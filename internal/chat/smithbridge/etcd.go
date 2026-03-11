package smithbridge

import (
	"context"
	"fmt"
	"smith/internal/source/model"
	"smith/internal/source/store"
)

type EtcdBridge struct {
	store *store.Store
}

func NewEtcdBridge(s *store.Store) *EtcdBridge {
	return &EtcdBridge{store: s}
}

func (b *EtcdBridge) GetLoop(ctx context.Context, loopID string) (*model.State, error) {
	l, found, err := b.store.GetState(ctx, loopID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("loop not found")
	}
	return &l.Record, nil
}

func (b *EtcdBridge) GetJournal(ctx context.Context, loopID string, limit int64) ([]model.JournalEntry, error) {
	return b.store.ListJournal(ctx, loopID, limit)
}

func (b *EtcdBridge) GetDocument(ctx context.Context, docID string) (*model.Document, error) {
	d, found, err := b.store.GetDocument(ctx, docID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("document not found")
	}
	return &d, nil
}
