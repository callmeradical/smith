package e2e

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"smith/internal/source/locking"
	"smith/internal/source/model"
)

type memLeaseStore struct {
	mu      sync.Mutex
	rev     int64
	records map[string]entry
}

type entry struct {
	lock     model.LeaseLock
	revision int64
}

func newMemLeaseStore() *memLeaseStore {
	return &memLeaseStore{records: map[string]entry{}}
}

func (m *memLeaseStore) Read(_ context.Context, loopID string) (locking.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[loopID]
	if !ok {
		return locking.Record{Found: false}, nil
	}
	return locking.Record{Found: true, Lock: rec.lock, Revision: rec.revision}, nil
}

func (m *memLeaseStore) PutIfRevision(_ context.Context, lock model.LeaseLock, expectedRevision int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.records[lock.LoopID]
	if !ok {
		if expectedRevision != 0 {
			return false, nil
		}
		m.rev++
		m.records[lock.LoopID] = entry{lock: lock, revision: m.rev}
		return true, nil
	}
	if current.revision != expectedRevision {
		return false, nil
	}
	m.rev++
	m.records[lock.LoopID] = entry{lock: lock, revision: m.rev}
	return true, nil
}

func (m *memLeaseStore) DeleteIfRevision(_ context.Context, loopID string, expectedRevision int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.records[loopID]
	if !ok {
		return false, nil
	}
	if current.revision != expectedRevision {
		return false, nil
	}
	delete(m.records, loopID)
	return true, nil
}

func TestConcurrentLoopsIsolation(t *testing.T) {
	store := newMemLeaseStore()
	mgr := locking.NewManager(store, 5*time.Minute)

	loops := []string{"concurrent-safe-a", "concurrent-safe-b", "single-loop-success"}
	acquiredBy := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, loopID := range loops {
		for i := 0; i < 20; i++ {
			wg.Add(1)
			holder := fmt.Sprintf("worker-%s-%d", loopID, i)
			go func(loopID, holder string, leaseID int64) {
				defer wg.Done()
				lock, err := mgr.Acquire(context.Background(), loopID, holder, leaseID)
				if err != nil {
					return
				}
				mu.Lock()
				if _, exists := acquiredBy[loopID]; !exists {
					acquiredBy[loopID] = lock.Holder
				}
				mu.Unlock()
			}(loopID, holder, int64(i+1))
		}
	}
	wg.Wait()

	for _, loopID := range loops {
		holder := acquiredBy[loopID]
		require.NotEmpty(t, holder, "expected one holder for loop %s", loopID)
		require.NoError(t, mgr.Release(context.Background(), loopID, holder), "release lock for %s", loopID)
		_, err := mgr.Acquire(context.Background(), loopID, holder+"-next", 999)
		require.NoError(t, err, "expected second acquire after release for %s", loopID)
	}
}
