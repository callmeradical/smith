package store

import (
	"context"
	"encoding/json"

	clientv3 "go.etcd.io/etcd/client/v3"

	"smith/internal/source/locking"
	"smith/internal/source/model"
)

type EtcdLeaseStore struct {
	store *Store
}

func NewEtcdLeaseStore(store *Store) *EtcdLeaseStore {
	return &EtcdLeaseStore{store: store}
}

func (s *EtcdLeaseStore) Read(ctx context.Context, loopID string) (locking.Record, error) {
	key := model.LockKey(loopID)
	resp, err := s.store.Client().Get(ctx, key)
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

func (s *EtcdLeaseStore) PutIfRevision(ctx context.Context, lock model.LeaseLock, expectedRevision int64) (bool, error) {
	payload, err := json.Marshal(lock)
	if err != nil {
		return false, err
	}
	key := model.LockKey(lock.LoopID)
	cmp := clientv3.Compare(clientv3.ModRevision(key), "=", expectedRevision)
	if expectedRevision == 0 {
		cmp = clientv3.Compare(clientv3.Version(key), "=", 0)
	}
	resp, err := s.store.Client().Txn(ctx).
		If(cmp).
		Then(clientv3.OpPut(key, string(payload))).
		Commit()
	if err != nil {
		return false, err
	}
	return resp.Succeeded, nil
}

func (s *EtcdLeaseStore) DeleteIfRevision(ctx context.Context, loopID string, expectedRevision int64) (bool, error) {
	key := model.LockKey(loopID)
	resp, err := s.store.Client().Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision(key), "=", expectedRevision)).
		Then(clientv3.OpDelete(key)).
		Commit()
	if err != nil {
		return false, err
	}
	return resp.Succeeded, nil
}
