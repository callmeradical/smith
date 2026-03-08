package model

import (
	"context"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

type memKV struct {
	mu       sync.RWMutex
	data     map[string][]byte
	watchers map[string][]chan WatchEvent
}

func newMemKV() *memKV {
	return &memKV{data: map[string][]byte{}, watchers: map[string][]chan WatchEvent{}}
}

func (m *memKV) Put(_ context.Context, key string, value []byte) error {
	m.mu.Lock()
	m.data[key] = append([]byte(nil), value...)
	var toNotify []chan WatchEvent
	for prefix, chans := range m.watchers {
		if strings.HasPrefix(key, prefix) {
			toNotify = append(toNotify, chans...)
		}
	}
	m.mu.Unlock()

	event := WatchEvent{Key: key, Value: append([]byte(nil), value...)}
	for _, ch := range toNotify {
		select {
		case ch <- event:
		default:
		}
	}
	return nil
}

func (m *memKV) Get(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.data[key]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), value...), true, nil
}

func (m *memKV) ListPrefix(_ context.Context, prefix string) ([]KVPair, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]KVPair, 0)
	for key, value := range m.data {
		if strings.HasPrefix(key, prefix) {
			out = append(out, KVPair{Key: key, Value: append([]byte(nil), value...)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (m *memKV) WatchPrefix(ctx context.Context, prefix string) <-chan WatchEvent {
	ch := make(chan WatchEvent, 8)
	m.mu.Lock()
	m.watchers[prefix] = append(m.watchers[prefix], ch)
	m.mu.Unlock()
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch
}

func TestAnomalyStateJournalAndHandoffCRUD(t *testing.T) {
	ctx := context.Background()
	kv := newMemKV()

	if err := PutAnomaly(ctx, kv, Anomaly{ID: "loop-1", Title: "t"}); err != nil {
		t.Fatalf("put anomaly: %v", err)
	}
	anomaly, found, err := GetAnomaly(ctx, kv, "loop-1")
	if err != nil || !found {
		t.Fatalf("get anomaly found=%v err=%v", found, err)
	}
	if anomaly.SchemaVersion != SchemaVersion {
		t.Fatalf("expected anomaly schema %s got %s", SchemaVersion, anomaly.SchemaVersion)
	}

	if err := PutState(ctx, kv, StateRecord{LoopID: "loop-1", State: LoopStateUnresolved, CorrelationID: "corr"}); err != nil {
		t.Fatalf("put state: %v", err)
	}
	state, found, err := GetState(ctx, kv, "loop-1")
	if err != nil || !found {
		t.Fatalf("get state found=%v err=%v", found, err)
	}
	if state.State != LoopStateUnresolved {
		t.Fatalf("expected unresolved got %s", state.State)
	}

	if err := AppendJournal(ctx, kv, JournalEntry{LoopID: "loop-1", Message: "one"}); err != nil {
		t.Fatalf("append journal 1: %v", err)
	}
	if err := AppendJournal(ctx, kv, JournalEntry{LoopID: "loop-1", Message: "two"}); err != nil {
		t.Fatalf("append journal 2: %v", err)
	}
	journal, err := ListJournal(ctx, kv, "loop-1")
	if err != nil {
		t.Fatalf("list journal: %v", err)
	}
	if len(journal) != 2 || journal[0].Sequence != 1 || journal[1].Sequence != 2 {
		t.Fatalf("unexpected journal sequence: %+v", journal)
	}

	if err := AppendHandoff(ctx, kv, Handoff{LoopID: "loop-1", ValidationState: "passed"}); err != nil {
		t.Fatalf("append handoff: %v", err)
	}
	pairs, err := kv.ListPrefix(ctx, HandoffPrefix("loop-1")+"/")
	if err != nil {
		t.Fatalf("list handoffs: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("expected 1 handoff got %d", len(pairs))
	}
}

func TestWatchStates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kv := newMemKV()
	ch := WatchStates(ctx, kv)

	if err := PutState(ctx, kv, StateRecord{LoopID: "loop-watch", State: LoopStateUnresolved, CorrelationID: "c"}); err != nil {
		t.Fatalf("put state: %v", err)
	}

	select {
	case got, ok := <-ch:
		if !ok {
			t.Fatal("watch channel closed unexpectedly")
		}
		if got.LoopID != "loop-watch" || got.State.State != LoopStateUnresolved {
			t.Fatalf("unexpected watch event: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch event")
	}
}
