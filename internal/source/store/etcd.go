package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"smith/internal/source/locking"
	"smith/internal/source/model"
)

type Store struct {
	cli *clientv3.Client
}

func New(ctx context.Context, endpoints []string, dialTimeout time.Duration) (*Store, error) {
	clean := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		ep = strings.TrimSpace(ep)
		if ep != "" {
			clean = append(clean, ep)
		}
	}
	if len(clean) == 0 {
		return nil, errors.New("at least one etcd endpoint is required")
	}
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	cli, err := clientv3.New(clientv3.Config{
		Context:     ctx,
		Endpoints:   clean,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &Store{cli: cli}, nil
}

func (s *Store) Close() error {
	if s == nil || s.cli == nil {
		return nil
	}
	return s.cli.Close()
}

func (s *Store) Client() *clientv3.Client {
	return s.cli
}

func (s *Store) ListStates(ctx context.Context) ([]LoopWithRevision, error) {
	resp, err := s.cli.Get(ctx, model.PrefixState+"/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	out := make([]LoopWithRevision, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		rec, err := model.DecodeStateRecord(kv.Value)
		if err != nil {
			continue
		}
		out = append(out, LoopWithRevision{Record: rec, Revision: kv.ModRevision})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Record.UpdatedAt.Before(out[j].Record.UpdatedAt)
	})
	return out, nil
}

func (s *Store) PutAnomaly(ctx context.Context, anomaly model.Anomaly) error {
	if strings.TrimSpace(anomaly.ID) == "" {
		return errors.New("anomaly id is required")
	}
	anomaly.SchemaVersion = model.SchemaVersion
	now := time.Now().UTC()
	if anomaly.CreatedAt.IsZero() {
		anomaly.CreatedAt = now
	}
	anomaly.UpdatedAt = now
	payload, err := json.Marshal(anomaly)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, model.AnomalyKey(anomaly.ID), string(payload))
	return err
}

func (s *Store) GetAnomaly(ctx context.Context, loopID string) (model.Anomaly, bool, error) {
	resp, err := s.cli.Get(ctx, model.AnomalyKey(loopID))
	if err != nil {
		return model.Anomaly{}, false, err
	}
	if len(resp.Kvs) == 0 {
		return model.Anomaly{}, false, nil
	}
	var anomaly model.Anomaly
	if err := json.Unmarshal(resp.Kvs[0].Value, &anomaly); err != nil {
		return model.Anomaly{}, false, err
	}
	return anomaly, true, nil
}

func (s *Store) GetState(ctx context.Context, loopID string) (LoopWithRevision, bool, error) {
	key := model.StateKey(loopID)
	resp, err := s.cli.Get(ctx, key)
	if err != nil {
		return LoopWithRevision{}, false, err
	}
	if len(resp.Kvs) == 0 {
		return LoopWithRevision{}, false, nil
	}
	rec, err := model.DecodeStateRecord(resp.Kvs[0].Value)
	if err != nil {
		return LoopWithRevision{}, false, err
	}
	return LoopWithRevision{Record: rec, Revision: resp.Kvs[0].ModRevision}, true, nil
}

func (s *Store) PutState(ctx context.Context, rec model.StateRecord, expectedRevision int64) (int64, error) {
	if strings.TrimSpace(rec.LoopID) == "" {
		return 0, errors.New("loop id is required")
	}
	rec.SchemaVersion = model.SchemaVersion
	rec.UpdatedAt = time.Now().UTC()
	payload, err := json.Marshal(rec)
	if err != nil {
		return 0, err
	}

	key := model.StateKey(rec.LoopID)
	cmp := clientv3.Compare(clientv3.ModRevision(key), "=", expectedRevision)
	if expectedRevision == 0 {
		cmp = clientv3.Compare(clientv3.Version(key), "=", 0)
	}

	txnResp, err := s.cli.Txn(ctx).
		If(cmp).
		Then(clientv3.OpPut(key, string(payload))).
		Else(clientv3.OpGet(key)).
		Commit()
	if err != nil {
		return 0, err
	}
	if !txnResp.Succeeded {
		return 0, ErrRevisionMismatch
	}

	getResp, err := s.cli.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	if len(getResp.Kvs) == 0 {
		return 0, errors.New("state write succeeded but value missing")
	}
	return getResp.Kvs[0].ModRevision, nil
}

func (s *Store) PutStateFromCurrent(ctx context.Context, loopID string, mutate func(current model.StateRecord) (model.StateRecord, error)) (LoopWithRevision, error) {
	current, found, err := s.GetState(ctx, loopID)
	if err != nil {
		return LoopWithRevision{}, err
	}
	if !found {
		return LoopWithRevision{}, fmt.Errorf("state not found for loop %s", loopID)
	}
	next, err := mutate(current.Record)
	if err != nil {
		return LoopWithRevision{}, err
	}
	rev, err := s.PutState(ctx, next, current.Revision)
	if err != nil {
		return LoopWithRevision{}, err
	}
	next.ObservedRevision = rev
	return LoopWithRevision{Record: next, Revision: rev}, nil
}

func (s *Store) DeleteLoop(ctx context.Context, loopID string) error {
	id := strings.TrimSpace(loopID)
	if id == "" {
		return errors.New("loop id is required")
	}
	ops := []clientv3.Op{
		clientv3.OpDelete(model.StateKey(id)),
		clientv3.OpDelete(model.AnomalyKey(id)),
		clientv3.OpDelete(model.LockKey(id)),
		clientv3.OpDelete(model.JournalPrefix(id)+"/", clientv3.WithPrefix()),
		clientv3.OpDelete(model.HandoffPrefix(id)+"/", clientv3.WithPrefix()),
		clientv3.OpDelete(model.OverridePrefix(id)+"/", clientv3.WithPrefix()),
	}
	_, err := s.cli.Txn(ctx).Then(ops...).Commit()
	return err
}

func (s *Store) NextSequence(ctx context.Context, prefix string) (int64, error) {
	resp, err := s.cli.Get(ctx, prefix+"/", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend), clientv3.WithLimit(1))
	if err != nil {
		return 0, err
	}
	if len(resp.Kvs) == 0 {
		return 1, nil
	}
	base := path.Base(string(resp.Kvs[0].Key))
	var seq int64
	_, scanErr := fmt.Sscanf(base, "%d", &seq)
	if scanErr != nil {
		return 1, nil
	}
	return seq + 1, nil
}

func (s *Store) AppendJournal(ctx context.Context, entry model.JournalEntry) error {
	entry.SchemaVersion = model.SchemaVersion
	entry.Timestamp = time.Now().UTC()
	if entry.Sequence == 0 {
		seq, err := s.NextSequence(ctx, model.JournalPrefix(entry.LoopID))
		if err != nil {
			return err
		}
		entry.Sequence = seq
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, model.JournalKey(entry.LoopID, entry.Sequence), string(payload))
	return err
}

func (s *Store) PutDocument(ctx context.Context, doc model.Document) error {
	if strings.TrimSpace(doc.ID) == "" {
		return errors.New("document id is required")
	}
	doc.SchemaVersion = model.SchemaVersion
	now := time.Now().UTC()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now
	payload, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, model.DocumentKey(doc.ID), string(payload))
	return err
}

func (s *Store) GetDocument(ctx context.Context, docID string) (model.Document, bool, error) {
	resp, err := s.cli.Get(ctx, model.DocumentKey(docID))
	if err != nil {
		return model.Document{}, false, err
	}
	if len(resp.Kvs) == 0 {
		return model.Document{}, false, nil
	}
	var doc model.Document
	if err := json.Unmarshal(resp.Kvs[0].Value, &doc); err != nil {
		return model.Document{}, false, err
	}
	return doc, true, nil
}

func (s *Store) ListDocuments(ctx context.Context) ([]model.Document, error) {
	resp, err := s.cli.Get(ctx, model.PrefixDocuments+"/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	out := make([]model.Document, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var doc model.Document
		if err := json.Unmarshal(kv.Value, &doc); err != nil {
			continue
		}
		out = append(out, doc)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s *Store) DeleteDocument(ctx context.Context, docID string) error {
	if strings.TrimSpace(docID) == "" {
		return errors.New("document id is required")
	}
	_, err := s.cli.Delete(ctx, model.DocumentKey(docID))
	return err
}

func (s *Store) ListJournal(ctx context.Context, loopID string, limit int64) ([]model.JournalEntry, error) {
	opts := []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend)}
	if limit > 0 {
		opts = append(opts, clientv3.WithLimit(limit))
	}
	resp, err := s.cli.Get(ctx, model.JournalPrefix(loopID)+"/", opts...)
	if err != nil {
		return nil, err
	}
	entries := make([]model.JournalEntry, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var entry model.JournalEntry
		if unmarshalErr := json.Unmarshal(kv.Value, &entry); unmarshalErr != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *Store) AppendHandoff(ctx context.Context, handoff model.Handoff) error {
	handoff.SchemaVersion = model.SchemaVersion
	handoff.Timestamp = time.Now().UTC()
	if handoff.Sequence == 0 {
		seq, err := s.NextSequence(ctx, model.HandoffPrefix(handoff.LoopID))
		if err != nil {
			return err
		}
		handoff.Sequence = seq
	}
	payload, err := json.Marshal(handoff)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, model.HandoffKey(handoff.LoopID, handoff.Sequence), string(payload))
	return err
}

func (s *Store) GetLatestHandoff(ctx context.Context, loopID string) (model.Handoff, bool, error) {
	loopID = strings.TrimSpace(loopID)
	if loopID == "" {
		return model.Handoff{}, false, errors.New("loop id is required")
	}
	resp, err := s.cli.Get(
		ctx,
		model.HandoffPrefix(loopID)+"/",
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend),
		clientv3.WithLimit(1),
	)
	if err != nil {
		return model.Handoff{}, false, err
	}
	if len(resp.Kvs) == 0 {
		return model.Handoff{}, false, nil
	}
	var handoff model.Handoff
	if err := json.Unmarshal(resp.Kvs[0].Value, &handoff); err != nil {
		return model.Handoff{}, false, err
	}
	return handoff, true, nil
}

func (s *Store) ListHandoffs(ctx context.Context, loopID string, limit int64) ([]model.Handoff, error) {
	opts := []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend)}
	if limit > 0 {
		opts = append(opts, clientv3.WithLimit(limit))
	}
	resp, err := s.cli.Get(ctx, model.HandoffPrefix(loopID)+"/", opts...)
	if err != nil {
		return nil, err
	}
	out := make([]model.Handoff, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var handoff model.Handoff
		if err := json.Unmarshal(kv.Value, &handoff); err != nil {
			continue
		}
		out = append(out, handoff)
	}
	return out, nil
}

func (s *Store) AppendOverride(ctx context.Context, override model.OperatorOverride) error {
	override.SchemaVersion = model.SchemaVersion
	override.Timestamp = time.Now().UTC()
	if override.Sequence == 0 {
		seq, err := s.NextSequence(ctx, model.OverridePrefix(override.LoopID))
		if err != nil {
			return err
		}
		override.Sequence = seq
	}
	payload, err := json.Marshal(override)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, model.OverrideKey(override.LoopID, override.Sequence), string(payload))
	return err
}

func (s *Store) ListOverrides(ctx context.Context, loopID string, limit int64) ([]model.OperatorOverride, error) {
	opts := []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend)}
	if limit > 0 {
		opts = append(opts, clientv3.WithLimit(limit))
	}
	resp, err := s.cli.Get(ctx, model.OverridePrefix(loopID)+"/", opts...)
	if err != nil {
		return nil, err
	}
	out := make([]model.OperatorOverride, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var override model.OperatorOverride
		if err := json.Unmarshal(kv.Value, &override); err != nil {
			continue
		}
		out = append(out, override)
	}
	return out, nil
}

func (s *Store) AppendAudit(ctx context.Context, rec AuditRecord) error {
	if rec.EventID == "" {
		rec.EventID = fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	rec.SchemaVersion = model.SchemaVersion
	rec.Timestamp = time.Now().UTC()
	key := fmt.Sprintf("%s/%04d/%02d/%02d/%s", model.PrefixAudit, rec.Timestamp.Year(), rec.Timestamp.Month(), rec.Timestamp.Day(), rec.EventID)
	payload, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, key, string(payload))
	return err
}

func (s *Store) ListAudit(ctx context.Context, loopID string, limit int64) ([]AuditRecord, error) {
	opts := []clientv3.OpOption{
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend),
	}
	resp, err := s.cli.Get(ctx, model.PrefixAudit+"/", opts...)
	if err != nil {
		return nil, err
	}
	records := make([]AuditRecord, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var rec AuditRecord
		if err := json.Unmarshal(kv.Value, &rec); err != nil {
			continue
		}
		if loopID != "" && rec.TargetLoopID != loopID {
			continue
		}
		records = append(records, rec)
		if limit > 0 && int64(len(records)) >= limit {
			break
		}
	}
	return records, nil
}

func (s *Store) ListJournalSinceWithRevision(ctx context.Context, loopID string, sinceSeq int64) ([]model.JournalEntry, int64, error) {
	resp, err := s.cli.Get(
		ctx,
		model.JournalPrefix(loopID)+"/",
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
	)
	if err != nil {
		return nil, 0, err
	}
	entries := make([]model.JournalEntry, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var entry model.JournalEntry
		if err := json.Unmarshal(kv.Value, &entry); err != nil {
			continue
		}
		if entry.Sequence <= sinceSeq {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, resp.Header.Revision, nil
}

func (s *Store) WatchJournal(ctx context.Context, loopID string) <-chan model.JournalEntry {
	return s.WatchJournalWithRev(ctx, loopID, 0)
}

func (s *Store) WatchJournalWithRev(ctx context.Context, loopID string, rev int64) <-chan model.JournalEntry {
	out := make(chan model.JournalEntry)
	prefix := model.JournalPrefix(loopID) + "/"
	opts := []clientv3.OpOption{clientv3.WithPrefix()}
	if rev > 0 {
		opts = append(opts, clientv3.WithRev(rev))
	}
	watchCh := s.cli.Watch(ctx, prefix, opts...)
	go func() {
		defer close(out)
		for watchResp := range watchCh {
			if watchResp.Err() != nil {
				continue
			}
			for _, event := range watchResp.Events {
				if event.Type != clientv3.EventTypePut || len(event.Kv.Value) == 0 {
					continue
				}
				var entry model.JournalEntry
				if err := json.Unmarshal(event.Kv.Value, &entry); err != nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- entry:
				}
			}
		}
	}()
	return out
}

func (s *Store) WatchState(ctx context.Context) <-chan Event {
	out := make(chan Event)
	watchCh := s.cli.Watch(ctx, model.PrefixState+"/", clientv3.WithPrefix())
	go func() {
		defer close(out)
		for watchResp := range watchCh {
			if watchResp.Err() != nil {
				continue
			}
			for _, event := range watchResp.Events {
				if event.Type != clientv3.EventTypePut || len(event.Kv.Value) == 0 {
					continue
				}
				rec, err := model.DecodeStateRecord(event.Kv.Value)
				if err != nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- Event{
					LoopID:   rec.LoopID,
					State:    rec,
					Revision: event.Kv.ModRevision,
					HasState: true,
					RawKey:   string(event.Kv.Key),
					RawValue: event.Kv.Value,
				}:
				}
			}
		}
	}()
	return out
}

func (s *Store) WatchDocuments(ctx context.Context) <-chan model.Document {
	out := make(chan model.Document)
	watchCh := s.cli.Watch(ctx, model.PrefixDocuments+"/", clientv3.WithPrefix())
	go func() {
		defer close(out)
		for watchResp := range watchCh {
			if watchResp.Err() != nil {
				continue
			}
			for _, event := range watchResp.Events {
				if event.Type != clientv3.EventTypePut || len(event.Kv.Value) == 0 {
					continue
				}
				var doc model.Document
				if err := json.Unmarshal(event.Kv.Value, &doc); err != nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- doc:
				}
			}
		}
	}()
	return out
}

func (s *Store) WatchAudit(ctx context.Context) <-chan AuditRecord {
	out := make(chan AuditRecord)
	watchCh := s.cli.Watch(ctx, model.PrefixAudit+"/", clientv3.WithPrefix())
	go func() {
		defer close(out)
		for watchResp := range watchCh {
			if watchResp.Err() != nil {
				continue
			}
			for _, event := range watchResp.Events {
				if event.Type != clientv3.EventTypePut || len(event.Kv.Value) == 0 {
					continue
				}
				var rec AuditRecord
				if err := json.Unmarshal(event.Kv.Value, &rec); err != nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- rec:
				}
			}
		}
	}()
	return out
}

// RecordPhase implements completion.PhaseStore
func (s *Store) RecordPhase(ctx context.Context, record model.JournalEntry) error {
	return s.AppendJournal(ctx, record)
}

// SetStateSynced implements completion.PhaseStore
func (s *Store) SetStateSynced(ctx context.Context, loopID string, commitSHA string) error {
	_, err := s.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		current.State = model.LoopStateSynced
		current.Reason = fmt.Sprintf("completion-saga-succeeded: commit=%s", commitSHA)
		return current, nil
	})
	return err
}

// SetStateUnresolved implements completion.PhaseStore
func (s *Store) SetStateUnresolved(ctx context.Context, loopID string, reason string) error {
	_, err := s.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		current.State = model.LoopStateUnresolved
		current.Reason = reason
		return current, nil
	})
	return err
}

func (s *Store) ReadLock(ctx context.Context, loopID string) (locking.Record, error) {
	key := model.LockKey(loopID)
	resp, err := s.cli.Get(ctx, key)
	if err != nil {
		return locking.Record{}, err
	}
	if len(resp.Kvs) == 0 {
		return locking.Record{Found: false}, nil
	}

	var lock model.LeaseLock
	if err := json.Unmarshal(resp.Kvs[0].Value, &lock); err != nil {
		return locking.Record{}, err
	}
	return locking.Record{Lock: lock, Revision: resp.Kvs[0].ModRevision, Found: true}, nil
}

func (s *Store) PutLockIfRevision(ctx context.Context, lock model.LeaseLock, expectedRevision int64) (bool, error) {
	payload, err := json.Marshal(lock)
	if err != nil {
		return false, err
	}
	key := model.LockKey(lock.LoopID)
	cmp := clientv3.Compare(clientv3.ModRevision(key), "=", expectedRevision)
	if expectedRevision == 0 {
		cmp = clientv3.Compare(clientv3.Version(key), "=", 0)
	}
	resp, err := s.cli.Txn(ctx).
		If(cmp).
		Then(clientv3.OpPut(key, string(payload))).
		Commit()
	if err != nil {
		return false, err
	}
	return resp.Succeeded, nil
}

func (s *Store) DeleteLockIfRevision(ctx context.Context, loopID string, expectedRevision int64) (bool, error) {
	key := model.LockKey(loopID)
	resp, err := s.cli.Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision(key), "=", expectedRevision)).
		Then(clientv3.OpDelete(key)).
		Commit()
	if err != nil {
		return false, err
	}
	return resp.Succeeded, nil
}
