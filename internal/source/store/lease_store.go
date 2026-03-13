package store

import (
	"context"

	"smith/internal/source/locking"
	"smith/internal/source/model"
)

// EtcdLeaseStore implements locking.LeaseStore using a StateStore.
type EtcdLeaseStore struct {
	store StateStore
}

func NewEtcdLeaseStore(store StateStore) *EtcdLeaseStore {
	return &EtcdLeaseStore{store: store}
}

func (s *EtcdLeaseStore) Read(ctx context.Context, loopID string) (locking.Record, error) {
	return s.store.ReadLock(ctx, loopID)
}

func (s *EtcdLeaseStore) PutIfRevision(ctx context.Context, lock model.LeaseLock, expectedRevision int64) (bool, error) {
	return s.store.PutLockIfRevision(ctx, lock, expectedRevision)
}

func (s *EtcdLeaseStore) DeleteIfRevision(ctx context.Context, loopID string, expectedRevision int64) (bool, error) {
	return s.store.DeleteLockIfRevision(ctx, loopID, expectedRevision)
}
