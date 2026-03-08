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

	"smith/internal/source/model"
)

var ErrRevisionMismatch = errors.New("etcd revision mismatch")

type Event struct {
	LoopID   string
	State    model.StateRecord
	Revision int64
	HasState bool
	RawKey   string
	RawValue []byte
}

type AuditRecord struct {
	EventID       string            `json:"event_id"`
	Timestamp     time.Time         `json:"timestamp"`
	Actor         string            `json:"actor"`
	Action        string            `json:"action"`
	TargetLoopID  string            `json:"target_loop_id"`
	Reason        string            `json:"reason,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	SchemaVersion string            `json:"schema_version"`
}

type LoopWithRevision struct {
	Record   model.StateRecord
	Revision int64
}

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
