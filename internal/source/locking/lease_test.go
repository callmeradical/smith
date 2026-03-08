package locking

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"smith/internal/source/model"
)

func TestAcquirePreventsSplitBrain(t *testing.T) {
	store := newInMemoryLeaseStore()
	manager := NewManager(store, 30*time.Second)
	manager.clock = fixedClock(time.Unix(100, 0))

	lockA, err := manager.Acquire(context.Background(), "loop-1", "core-a", 1)
	if err != nil {
		t.Fatalf("unexpected acquire error: %v", err)
	}
	if lockA.Holder != "core-a" {
		t.Fatalf("unexpected holder %q", lockA.Holder)
	}

	_, err = manager.Acquire(context.Background(), "loop-1", "core-b", 2)
	if err == nil {
		t.Fatal("expected lock held error")
	}
	if !errors.Is(err, ErrLockHeld) {
		t.Fatalf("expected ErrLockHeld, got %v", err)
	}
}

func TestAcquireStealsExpiredLease(t *testing.T) {
	store := newInMemoryLeaseStore()
	manager := NewManager(store, 30*time.Second)
	now := time.Unix(200, 0)
	manager.clock = fixedClock(now)

	if _, err := manager.Acquire(context.Background(), "loop-2", "core-a", 1); err != nil {
		t.Fatalf("unexpected acquire error: %v", err)
	}

	manager.clock = fixedClock(now.Add(31 * time.Second))
	lockB, err := manager.Acquire(context.Background(), "loop-2", "core-b", 2)
	if err != nil {
		t.Fatalf("expected steal after timeout, got %v", err)
	}
	if lockB.Holder != "core-b" {
		t.Fatalf("expected lock holder core-b, got %q", lockB.Holder)
	}
}

func TestRenewUpdatesHeartbeatForHolderOnly(t *testing.T) {
	store := newInMemoryLeaseStore()
	manager := NewManager(store, 30*time.Second)
	base := time.Unix(300, 0)
	manager.clock = fixedClock(base)

	if _, err := manager.Acquire(context.Background(), "loop-3", "core-a", 1); err != nil {
		t.Fatalf("unexpected acquire error: %v", err)
	}

	manager.clock = fixedClock(base.Add(5 * time.Second))
	lock, err := manager.Renew(context.Background(), "loop-3", "core-a", 22)
	if err != nil {
		t.Fatalf("unexpected renew error: %v", err)
	}
	if lock.LeaseID != 22 {
		t.Fatalf("expected lease id 22, got %d", lock.LeaseID)
	}
	if !lock.HeartbeatAt.Equal(base.Add(5 * time.Second)) {
		t.Fatalf("unexpected heartbeat %s", lock.HeartbeatAt)
	}

	if _, err := manager.Renew(context.Background(), "loop-3", "core-b", 23); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("expected ErrLockHeld for non-holder renew, got %v", err)
	}
}

func TestReleaseRequiresOwnership(t *testing.T) {
	store := newInMemoryLeaseStore()
	manager := NewManager(store, 30*time.Second)
	manager.clock = fixedClock(time.Unix(400, 0))

	if _, err := manager.Acquire(context.Background(), "loop-4", "core-a", 1); err != nil {
		t.Fatalf("unexpected acquire error: %v", err)
	}

	if err := manager.Release(context.Background(), "loop-4", "core-b"); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("expected ErrLockHeld, got %v", err)
	}
	if err := manager.Release(context.Background(), "loop-4", "core-a"); err != nil {
		t.Fatalf("unexpected release error: %v", err)
	}
}

type inMemoryLeaseStore struct {
	mu    sync.Mutex
	items map[string]leaseValue
}

type leaseValue struct {
	lock     model.LeaseLock
	revision int64
}

func newInMemoryLeaseStore() *inMemoryLeaseStore {
	return &inMemoryLeaseStore{
		items: map[string]leaseValue{},
	}
}

func (s *inMemoryLeaseStore) Read(_ context.Context, loopID string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.items[loopID]
	if !ok {
		return Record{Found: false}, nil
	}
	return Record{Found: true, Lock: v.lock, Revision: v.revision}, nil
}

func (s *inMemoryLeaseStore) PutIfRevision(_ context.Context, lock model.LeaseLock, expectedRevision int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.items[lock.LoopID]
	if expectedRevision == 0 {
		if ok {
			return false, nil
		}
		s.items[lock.LoopID] = leaseValue{lock: lock, revision: 1}
		return true, nil
	}

	if !ok || current.revision != expectedRevision {
		return false, nil
	}
	s.items[lock.LoopID] = leaseValue{lock: lock, revision: current.revision + 1}
	return true, nil
}

func (s *inMemoryLeaseStore) DeleteIfRevision(_ context.Context, loopID string, expectedRevision int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.items[loopID]
	if !ok || current.revision != expectedRevision {
		return false, nil
	}
	delete(s.items, loopID)
	return true, nil
}

func fixedClock(t time.Time) Clock {
	return func() time.Time { return t }
}
