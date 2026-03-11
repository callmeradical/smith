package smithbridge

import (
	"context"
	"smith/internal/source/model"
)

type Bridge interface {
	GetLoop(ctx context.Context, loopID string) (*model.State, error)
	GetJournal(ctx context.Context, loopID string, limit int64) ([]model.JournalEntry, error)
	GetDocument(ctx context.Context, docID string) (*model.Document, error)
}
