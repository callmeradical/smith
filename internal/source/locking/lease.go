package locking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"smith/internal/source/model"
)

var (
	ErrLockHeld      = errors.New("lock held by another holder")
	ErrLockLost      = errors.New("lock lost")
	ErrInvalidHolder = errors.New("invalid lock holder")
)

type Clock func() time.Time

type Record struct {
	Lock     model.LeaseLock
	Revision int64
	Found    bool
}

type LeaseStore interface {
	Read(ctx context.Context, loopID string) (Record, error)
	PutIfRevision(ctx context.Context, lock model.LeaseLock, expectedRevision int64) (bool, error)
	DeleteIfRevision(ctx context.Context, loopID string, expectedRevision int64) (bool, error)
}

type Manager struct {
	store        LeaseStore
	clock        Clock
	leaseTimeout time.Duration
}

func NewManager(store LeaseStore, leaseTimeout time.Duration) *Manager {
	return &Manager{
		store:        store,
		clock:        time.Now,
		leaseTimeout: leaseTimeout,
	}
}

func (m *Manager) Acquire(ctx context.Context, loopID, holder string, leaseID int64) (model.LeaseLock, error) {
	if holder == "" {
		return model.LeaseLock{}, ErrInvalidHolder
	}
	if loopID == "" {
		return model.LeaseLock{}, errors.New("loop id is required")
	}
	now := m.clock().UTC()

	record, err := m.store.Read(ctx, loopID)
	if err != nil {
		return model.LeaseLock{}, err
	}

	if !record.Found {
		lock := newLock(loopID, holder, leaseID, now)
		ok, putErr := m.store.PutIfRevision(ctx, lock, 0)
		if putErr != nil {
			return model.LeaseLock{}, putErr
		}
		if !ok {
			return model.LeaseLock{}, ErrLockHeld
		}
		return lock, nil
	}

	current := record.Lock
	if current.Holder == holder {
		current.HeartbeatAt = now
		current.LeaseID = leaseID
		ok, putErr := m.store.PutIfRevision(ctx, current, record.Revision)
		if putErr != nil {
			return model.LeaseLock{}, putErr
		}
		if !ok {
			return model.LeaseLock{}, ErrLockLost
		}
		return current, nil
	}

	if !isExpired(current.HeartbeatAt, now, m.leaseTimeout) {
		return model.LeaseLock{}, fmt.Errorf("%w: current holder=%s", ErrLockHeld, current.Holder)
	}

	stolen := newLock(loopID, holder, leaseID, now)
	ok, putErr := m.store.PutIfRevision(ctx, stolen, record.Revision)
	if putErr != nil {
		return model.LeaseLock{}, putErr
	}
	if !ok {
		return model.LeaseLock{}, ErrLockLost
	}
	return stolen, nil
}

func (m *Manager) Renew(ctx context.Context, loopID, holder string, leaseID int64) (model.LeaseLock, error) {
	record, err := m.store.Read(ctx, loopID)
	if err != nil {
		return model.LeaseLock{}, err
	}
	if !record.Found {
		return model.LeaseLock{}, ErrLockLost
	}
	if record.Lock.Holder != holder {
		return model.LeaseLock{}, ErrLockHeld
	}

	lock := record.Lock
	lock.LeaseID = leaseID
	lock.HeartbeatAt = m.clock().UTC()
	ok, putErr := m.store.PutIfRevision(ctx, lock, record.Revision)
	if putErr != nil {
		return model.LeaseLock{}, putErr
	}
	if !ok {
		return model.LeaseLock{}, ErrLockLost
	}
	return lock, nil
}

func (m *Manager) Release(ctx context.Context, loopID, holder string) error {
	record, err := m.store.Read(ctx, loopID)
	if err != nil {
		return err
	}
	if !record.Found {
		return nil
	}
	if record.Lock.Holder != holder {
		return ErrLockHeld
	}
	ok, delErr := m.store.DeleteIfRevision(ctx, loopID, record.Revision)
	if delErr != nil {
		return delErr
	}
	if !ok {
		return ErrLockLost
	}
	return nil
}

func isExpired(heartbeat, now time.Time, timeout time.Duration) bool {
	return !heartbeat.Add(timeout).After(now)
}

func newLock(loopID, holder string, leaseID int64, now time.Time) model.LeaseLock {
	return model.LeaseLock{
		LoopID:        loopID,
		Holder:        holder,
		LeaseID:       leaseID,
		AcquiredAt:    now,
		HeartbeatAt:   now,
		SchemaVersion: model.SchemaVersion,
	}
}
