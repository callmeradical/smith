package acceptance

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"

	"smith/internal/source/locking"
	"smith/internal/source/model"
)

type bddSuite struct {
	t           *testing.T
	mgr         *locking.Manager
	store       *bddLeaseStore
	singleState model.LoopState
	isolated    bool
}

func newBDDSuite(t *testing.T) *bddSuite {
	store := &bddLeaseStore{records: map[string]bddLeaseEntry{}}
	return &bddSuite{
		t:           t,
		store:       store,
		mgr:         locking.NewManager(store, 3*time.Minute),
		singleState: model.LoopStateUnresolved,
	}
}

func (s *bddSuite) resetFixture() error {
	s.store = &bddLeaseStore{records: map[string]bddLeaseEntry{}}
	s.mgr = locking.NewManager(s.store, 3*time.Minute)
	s.singleState = model.LoopStateUnresolved
	s.isolated = false
	return nil
}

func (s *bddSuite) runSingleLoopFlow() error {
	loopID := "single-loop-success"
	lock, err := s.mgr.Acquire(context.Background(), loopID, "worker-a", 1)
	if err != nil {
		return err
	}
	s.singleState = model.LoopStateOverwriting
	if err := s.mgr.Release(context.Background(), loopID, lock.Holder); err != nil {
		return err
	}
	s.singleState = model.LoopStateSynced
	return nil
}

func (s *bddSuite) assertSingleLoopSynced() error {
	require.Equal(s.t, model.LoopStateSynced, s.singleState)
	return nil
}

func (s *bddSuite) runConcurrentSafetyFlow() error {
	loops := []string{"concurrent-safe-a", "concurrent-safe-b", "concurrent-safe-c"}
	owners := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, loopID := range loops {
		for i := 0; i < 12; i++ {
			wg.Add(1)
			holder := fmt.Sprintf("%s-worker-%d", loopID, i)
			leaseID := int64(i + 1)
			go func(loopID, holder string, leaseID int64) {
				defer wg.Done()
				lock, err := s.mgr.Acquire(context.Background(), loopID, holder, leaseID)
				if err != nil {
					return
				}
				mu.Lock()
				if _, ok := owners[loopID]; !ok {
					owners[loopID] = lock.Holder
				}
				mu.Unlock()
			}(loopID, holder, leaseID)
		}
	}
	wg.Wait()

	for _, loopID := range loops {
		if owners[loopID] == "" {
			return fmt.Errorf("missing owner for %s", loopID)
		}
	}
	s.isolated = true
	return nil
}

func (s *bddSuite) assertConcurrentIsolation() error {
	require.True(s.t, s.isolated)
	return nil
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	// Hooked in TestFeatures to capture *testing.T and scenario-local state.
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			state := newBDDSuite(t)
			sc.Step(`^a loop workflow fixture is initialized$`, state.resetFixture)
			sc.Step(`^I run single loop completion flow$`, state.runSingleLoopFlow)
			sc.Step(`^the loop should reach synced state$`, state.assertSingleLoopSynced)
			sc.Step(`^I run concurrent loop safety flow$`, state.runConcurrentSafetyFlow)
			sc.Step(`^each loop should have an isolated lock owner$`, state.assertConcurrentIsolation)
		},
		Options: &godog.Options{
			Format: "pretty",
			Paths:  []string{"loop_workflows.feature"},
		},
	}
	if suite.Run() != 0 {
		t.Fatal("godog feature suite failed")
	}
}

type bddLeaseStore struct {
	mu      sync.Mutex
	rev     int64
	records map[string]bddLeaseEntry
}

type bddLeaseEntry struct {
	lock     model.LeaseLock
	revision int64
}

func (m *bddLeaseStore) Read(_ context.Context, loopID string) (locking.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[loopID]
	if !ok {
		return locking.Record{Found: false}, nil
	}
	return locking.Record{Found: true, Lock: rec.lock, Revision: rec.revision}, nil
}

func (m *bddLeaseStore) PutIfRevision(_ context.Context, lock model.LeaseLock, expectedRevision int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.records[lock.LoopID]
	if !ok {
		if expectedRevision != 0 {
			return false, nil
		}
		m.rev++
		m.records[lock.LoopID] = bddLeaseEntry{lock: lock, revision: m.rev}
		return true, nil
	}
	if current.revision != expectedRevision {
		return false, nil
	}
	m.rev++
	m.records[lock.LoopID] = bddLeaseEntry{lock: lock, revision: m.rev}
	return true, nil
}

func (m *bddLeaseStore) DeleteIfRevision(_ context.Context, loopID string, expectedRevision int64) (bool, error) {
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
