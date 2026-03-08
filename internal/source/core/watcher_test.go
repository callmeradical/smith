package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"smith/internal/source/model"
)

func TestHandleIgnoresNonUnresolved(t *testing.T) {
	q := &fakeQueue{}
	w := NewUnresolvedWatcher(q)
	w.sleep = noSleep

	err := w.Handle(context.Background(), UnresolvedEvent{
		LoopID:   "loop-1",
		State:    model.LoopStateSynced,
		Revision: 1,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if q.calls != 0 {
		t.Fatalf("expected no enqueue calls, got %d", q.calls)
	}
}

func TestHandleDedupesSameRevision(t *testing.T) {
	q := &fakeQueue{}
	w := NewUnresolvedWatcher(q)
	w.sleep = noSleep

	event := UnresolvedEvent{
		LoopID:   "loop-1",
		State:    model.LoopStateUnresolved,
		Revision: 42,
	}
	if err := w.Handle(context.Background(), event); err != nil {
		t.Fatalf("first handle failed: %v", err)
	}
	if err := w.Handle(context.Background(), event); err != nil {
		t.Fatalf("second handle failed: %v", err)
	}
	if q.calls != 1 {
		t.Fatalf("expected exactly one enqueue call, got %d", q.calls)
	}
}

func TestHandleIgnoresStaleRevision(t *testing.T) {
	q := &fakeQueue{}
	w := NewUnresolvedWatcher(q)
	w.sleep = noSleep

	newest := UnresolvedEvent{
		LoopID:   "loop-1",
		State:    model.LoopStateUnresolved,
		Revision: 10,
	}
	stale := UnresolvedEvent{
		LoopID:   "loop-1",
		State:    model.LoopStateUnresolved,
		Revision: 9,
	}

	if err := w.Handle(context.Background(), newest); err != nil {
		t.Fatalf("newest handle failed: %v", err)
	}
	if err := w.Handle(context.Background(), stale); err != nil {
		t.Fatalf("stale handle failed: %v", err)
	}
	if q.calls != 1 {
		t.Fatalf("expected one enqueue call, got %d", q.calls)
	}
}

func TestHandleRetriesWithExponentialBackoff(t *testing.T) {
	q := &fakeQueue{
		results: []error{
			errors.New("transient-1"),
			errors.New("transient-2"),
			nil,
		},
	}
	w := NewUnresolvedWatcher(q)
	var sleeps []time.Duration
	w.sleep = func(_ context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		return nil
	}

	err := w.Handle(context.Background(), UnresolvedEvent{
		LoopID:   "loop-2",
		State:    model.LoopStateUnresolved,
		Revision: 5,
		Policy: model.LoopPolicy{
			MaxAttempts:    3,
			BackoffInitial: 10 * time.Millisecond,
			BackoffMax:     25 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if q.calls != 3 {
		t.Fatalf("expected three enqueue calls, got %d", q.calls)
	}
	if len(sleeps) != 2 {
		t.Fatalf("expected two backoff sleeps, got %d", len(sleeps))
	}
	if sleeps[0] != 10*time.Millisecond || sleeps[1] != 20*time.Millisecond {
		t.Fatalf("unexpected backoff sequence: %v", sleeps)
	}
}

func TestHandleReturnsRetryExhaustedAndAllowsFutureRetry(t *testing.T) {
	q := &fakeQueue{
		results: []error{
			errors.New("always-fail"),
			errors.New("always-fail"),
		},
	}
	w := NewUnresolvedWatcher(q)
	w.sleep = noSleep

	event := UnresolvedEvent{
		LoopID:   "loop-3",
		State:    model.LoopStateUnresolved,
		Revision: 7,
		Policy: model.LoopPolicy{
			MaxAttempts:    2,
			BackoffInitial: time.Millisecond,
			BackoffMax:     time.Millisecond,
		},
	}

	err := w.Handle(context.Background(), event)
	if err == nil {
		t.Fatal("expected retry exhaustion error")
	}
	if !errors.Is(err, ErrRetryExhausted) {
		t.Fatalf("expected ErrRetryExhausted, got %v", err)
	}

	q.results = []error{nil}
	if retryErr := w.Handle(context.Background(), event); retryErr != nil {
		t.Fatalf("expected subsequent retry to succeed, got %v", retryErr)
	}
	if q.calls != 3 {
		t.Fatalf("expected total of three enqueue calls, got %d", q.calls)
	}
}

func TestHandleDedupesConcurrentDuplicateEvents(t *testing.T) {
	q := &fakeQueue{
		waitCh: make(chan struct{}),
	}
	w := NewUnresolvedWatcher(q)
	w.sleep = noSleep

	event := UnresolvedEvent{
		LoopID:   "loop-4",
		State:    model.LoopStateUnresolved,
		Revision: 88,
	}

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		done <- w.Handle(context.Background(), event)
	}()
	<-started

	time.Sleep(20 * time.Millisecond)
	if err := w.Handle(context.Background(), event); err != nil {
		t.Fatalf("duplicate concurrent event should be deduped without error: %v", err)
	}

	close(q.waitCh)
	if err := <-done; err != nil {
		t.Fatalf("first event failed: %v", err)
	}
	if q.calls != 1 {
		t.Fatalf("expected one enqueue call with concurrent duplicates, got %d", q.calls)
	}
}

func noSleep(_ context.Context, _ time.Duration) error {
	return nil
}

type fakeQueue struct {
	mu      sync.Mutex
	calls   int
	results []error
	waitCh  chan struct{}
}

func (f *fakeQueue) Enqueue(_ context.Context, _ ExecutionIntent) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls++

	if f.waitCh != nil {
		waitCh := f.waitCh
		f.mu.Unlock()
		<-waitCh
		f.mu.Lock()
	}

	if len(f.results) == 0 {
		return nil
	}

	err := f.results[0]
	f.results = f.results[1:]
	return err
}
