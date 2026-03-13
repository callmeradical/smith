package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"smith/internal/source/locking"
	"smith/internal/source/model"
)

type MemStore struct {
	mu        sync.RWMutex
	states    map[string]LoopWithRevision
	anomalies map[string]model.Anomaly
	docs      map[string]model.Document
	journal   map[string][]model.JournalEntry
	handoffs  map[string][]model.Handoff
	overrides map[string][]model.OperatorOverride
	audit     []AuditRecord
	locks     map[string]entry
	revision  int64

	stateWatchers    []chan Event
	docWatchers      []chan model.Document
	journalWatchers  map[string][]chan model.JournalEntry
	auditWatchers    []chan AuditRecord
}

func NewMemStore() *MemStore {
	return &MemStore{
		states:          make(map[string]LoopWithRevision),
		anomalies:       make(map[string]model.Anomaly),
		docs:            make(map[string]model.Document),
		journal:         make(map[string][]model.JournalEntry),
		handoffs:        make(map[string][]model.Handoff),
		overrides:       make(map[string][]model.OperatorOverride),
		locks:           make(map[string]entry),
		journalWatchers: make(map[string][]chan model.JournalEntry),
	}
}

func (m *MemStore) Close() error {
	return nil
}

func (m *MemStore) ListStates(ctx context.Context) ([]LoopWithRevision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]LoopWithRevision, 0, len(m.states))
	for _, s := range m.states {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Record.UpdatedAt.Before(out[j].Record.UpdatedAt)
	})
	return out, nil
}

func (m *MemStore) GetState(ctx context.Context, loopID string) (LoopWithRevision, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.states[loopID]
	return s, ok, nil
}

func (m *MemStore) PutState(ctx context.Context, rec model.StateRecord, expectedRevision int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.states[rec.LoopID]
	if (expectedRevision == 0 && ok) || (expectedRevision != 0 && current.Revision != expectedRevision) {
		return 0, ErrRevisionMismatch
	}

	m.revision++
	rec.UpdatedAt = time.Now().UTC()
	m.states[rec.LoopID] = LoopWithRevision{Record: rec, Revision: m.revision}

	event := Event{
		LoopID:   rec.LoopID,
		State:    rec,
		Revision: m.revision,
		HasState: true,
	}
	for _, w := range m.stateWatchers {
		select {
		case w <- event:
		default:
		}
	}
	return m.revision, nil
}

func (m *MemStore) PutStateFromCurrent(ctx context.Context, loopID string, mutate func(current model.StateRecord) (model.StateRecord, error)) (LoopWithRevision, error) {
	current, found, err := m.GetState(ctx, loopID)
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
	rev, err := m.PutState(ctx, next, current.Revision)
	if err != nil {
		return LoopWithRevision{}, err
	}
	next.ObservedRevision = rev
	return LoopWithRevision{Record: next, Revision: rev}, nil
}

func (m *MemStore) DeleteLoop(ctx context.Context, loopID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, loopID)
	delete(m.anomalies, loopID)
	delete(m.locks, loopID)
	delete(m.journal, loopID)
	delete(m.handoffs, loopID)
	delete(m.overrides, loopID)
	return nil
}

func (m *MemStore) WatchState(ctx context.Context) <-chan Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan Event, 100)
	m.stateWatchers = append(m.stateWatchers, ch)
	return ch
}

func (m *MemStore) PutAnomaly(ctx context.Context, anomaly model.Anomaly) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	anomaly.UpdatedAt = time.Now().UTC()
	m.anomalies[anomaly.ID] = anomaly
	return nil
}

func (m *MemStore) GetAnomaly(ctx context.Context, loopID string) (model.Anomaly, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.anomalies[loopID]
	return a, ok, nil
}

func (m *MemStore) PutDocument(ctx context.Context, doc model.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc.UpdatedAt = time.Now().UTC()
	m.docs[doc.ID] = doc
	for _, w := range m.docWatchers {
		select {
		case w <- doc:
		default:
		}
	}
	return nil
}

func (m *MemStore) GetDocument(ctx context.Context, docID string) (model.Document, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.docs[docID]
	return d, ok, nil
}

func (m *MemStore) ListDocuments(ctx context.Context) ([]model.Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.Document, 0, len(m.docs))
	for _, d := range m.docs {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (m *MemStore) DeleteDocument(ctx context.Context, docID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.docs, docID)
	return nil
}

func (m *MemStore) WatchDocuments(ctx context.Context) <-chan model.Document {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan model.Document, 100)
	m.docWatchers = append(m.docWatchers, ch)
	return ch
}

func (m *MemStore) NextSequence(ctx context.Context, prefix string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// This is a bit simplified for MemStore
	return int64(time.Now().UnixNano()), nil
}

func (m *MemStore) AppendJournal(ctx context.Context, entry model.JournalEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry.Timestamp = time.Now().UTC()
	if entry.Sequence == 0 {
		entry.Sequence = int64(len(m.journal[entry.LoopID]) + 1)
	}
	m.journal[entry.LoopID] = append(m.journal[entry.LoopID], entry)
	for _, w := range m.journalWatchers[entry.LoopID] {
		select {
		case w <- entry:
		default:
		}
	}
	return nil
}

func (m *MemStore) ListJournal(ctx context.Context, loopID string, limit int64) ([]model.JournalEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j := m.journal[loopID]
	if limit > 0 && int64(len(j)) > limit {
		j = j[int64(len(j))-limit:]
	}
	return j, nil
}

func (m *MemStore) ListJournalSinceWithRevision(ctx context.Context, loopID string, sinceSeq int64) ([]model.JournalEntry, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j := m.journal[loopID]
	out := make([]model.JournalEntry, 0)
	for _, e := range j {
		if e.Sequence > sinceSeq {
			out = append(out, e)
		}
	}
	return out, m.revision, nil
}

func (m *MemStore) WatchJournal(ctx context.Context, loopID string) <-chan model.JournalEntry {
	return m.WatchJournalWithRev(ctx, loopID, 0)
}

func (m *MemStore) WatchJournalWithRev(ctx context.Context, loopID string, rev int64) <-chan model.JournalEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan model.JournalEntry, 100)
	m.journalWatchers[loopID] = append(m.journalWatchers[loopID], ch)
	return ch
}

func (m *MemStore) AppendHandoff(ctx context.Context, handoff model.Handoff) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	handoff.Timestamp = time.Now().UTC()
	m.handoffs[handoff.LoopID] = append(m.handoffs[handoff.LoopID], handoff)
	return nil
}

func (m *MemStore) GetLatestHandoff(ctx context.Context, loopID string) (model.Handoff, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h := m.handoffs[loopID]
	if len(h) == 0 {
		return model.Handoff{}, false, nil
	}
	return h[len(h)-1], true, nil
}

func (m *MemStore) ListHandoffs(ctx context.Context, loopID string, limit int64) ([]model.Handoff, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h := m.handoffs[loopID]
	if limit > 0 && int64(len(h)) > limit {
		h = h[int64(len(h))-limit:]
	}
	return h, nil
}

func (m *MemStore) AppendOverride(ctx context.Context, override model.OperatorOverride) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	override.Timestamp = time.Now().UTC()
	m.overrides[override.LoopID] = append(m.overrides[override.LoopID], override)
	return nil
}

func (m *MemStore) ListOverrides(ctx context.Context, loopID string, limit int64) ([]model.OperatorOverride, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	o := m.overrides[loopID]
	if limit > 0 && int64(len(o)) > limit {
		o = o[int64(len(o))-limit:]
	}
	return o, nil
}

func (m *MemStore) AppendAudit(ctx context.Context, rec AuditRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec.Timestamp = time.Now().UTC()
	m.audit = append(m.audit, rec)
	for _, w := range m.auditWatchers {
		select {
		case w <- rec:
		default:
		}
	}
	return nil
}

func (m *MemStore) ListAudit(ctx context.Context, loopID string, limit int64) ([]AuditRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AuditRecord, 0)
	for i := len(m.audit) - 1; i >= 0; i-- {
		if loopID == "" || m.audit[i].TargetLoopID == loopID {
			out = append(out, m.audit[i])
		}
		if limit > 0 && int64(len(out)) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MemStore) WatchAudit(ctx context.Context) <-chan AuditRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan AuditRecord, 100)
	m.auditWatchers = append(m.auditWatchers, ch)
	return ch
}

func (m *MemStore) RecordPhase(ctx context.Context, record model.JournalEntry) error {
	return m.AppendJournal(ctx, record)
}

func (m *MemStore) SetStateSynced(ctx context.Context, loopID string, commitSHA string) error {
	_, err := m.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		current.State = model.LoopStateSynced
		current.Reason = fmt.Sprintf("completion-saga-succeeded: commit=%s", commitSHA)
		return current, nil
	})
	return err
}

func (m *MemStore) SetStateUnresolved(ctx context.Context, loopID string, reason string) error {
	_, err := m.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		current.State = model.LoopStateUnresolved
		current.Reason = reason
		return current, nil
	})
	return err
}

func (m *MemStore) ReadLock(ctx context.Context, loopID string) (locking.Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	l, ok := m.locks[loopID]
	if !ok {
		return locking.Record{Found: false}, nil
	}
	return locking.Record{Found: true, Lock: l.lock, Revision: l.revision}, nil
}

func (m *MemStore) PutLockIfRevision(ctx context.Context, lock model.LeaseLock, expectedRevision int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.locks[lock.LoopID]
	if !ok {
		if expectedRevision != 0 {
			return false, nil
		}
		m.revision++
		m.locks[lock.LoopID] = entry{lock: lock, revision: m.revision}
		return true, nil
	}
	if current.revision != expectedRevision {
		return false, nil
	}
	m.revision++
	m.locks[lock.LoopID] = entry{lock: lock, revision: m.revision}
	return true, nil
}

func (m *MemStore) DeleteLockIfRevision(ctx context.Context, loopID string, expectedRevision int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.locks[loopID]
	if !ok {
		return false, nil
	}
	if current.revision != expectedRevision {
		return false, nil
	}
	delete(m.locks, loopID)
	return true, nil
}

type entry struct {
	lock     model.LeaseLock
	revision int64
}
