package store

import (
	"context"
	"errors"
	"time"

	"smith/internal/source/locking"
	"smith/internal/source/model"
)

var ErrRevisionMismatch = errors.New("etcd revision mismatch")

type Event struct {
	LoopID   string
	State    model.StateRecord
	Revision int64
	HasState bool
	RawKey   string
	RawValue []byte
}

type AuditRecord struct {
	EventID       string            `json:"event_id"`
	Timestamp     time.Time         `json:"timestamp"`
	Actor         string            `json:"actor"`
	Action        string            `json:"action"`
	TargetLoopID  string            `json:"target_loop_id"`
	Reason        string            `json:"reason,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	SchemaVersion string            `json:"schema_version"`
}

type LoopWithRevision struct {
	Record   model.StateRecord
	Revision int64
}

type StateStore interface {
	Close() error

	// States
	ListStates(ctx context.Context) ([]LoopWithRevision, error)
	GetState(ctx context.Context, loopID string) (LoopWithRevision, bool, error)
	PutState(ctx context.Context, rec model.StateRecord, expectedRevision int64) (int64, error)
	PutStateFromCurrent(ctx context.Context, loopID string, mutate func(current model.StateRecord) (model.StateRecord, error)) (LoopWithRevision, error)
	DeleteLoop(ctx context.Context, loopID string) error
	WatchState(ctx context.Context) <-chan Event

	// Anomalies
	PutAnomaly(ctx context.Context, anomaly model.Anomaly) error
	GetAnomaly(ctx context.Context, loopID string) (model.Anomaly, bool, error)

	// Documents
	PutDocument(ctx context.Context, doc model.Document) error
	GetDocument(ctx context.Context, docID string) (model.Document, bool, error)
	ListDocuments(ctx context.Context) ([]model.Document, error)
	DeleteDocument(ctx context.Context, docID string) error
	WatchDocuments(ctx context.Context) <-chan model.Document

	// Journal
	AppendJournal(ctx context.Context, entry model.JournalEntry) error
	ListJournal(ctx context.Context, loopID string, limit int64) ([]model.JournalEntry, error)
	ListJournalSinceWithRevision(ctx context.Context, loopID string, sinceSeq int64) ([]model.JournalEntry, int64, error)
	WatchJournal(ctx context.Context, loopID string) <-chan model.JournalEntry
	WatchJournalWithRev(ctx context.Context, loopID string, rev int64) <-chan model.JournalEntry

	// Handoffs
	AppendHandoff(ctx context.Context, handoff model.Handoff) error
	GetLatestHandoff(ctx context.Context, loopID string) (model.Handoff, bool, error)
	ListHandoffs(ctx context.Context, loopID string, limit int64) ([]model.Handoff, error)

	// Overrides
	AppendOverride(ctx context.Context, override model.OperatorOverride) error
	ListOverrides(ctx context.Context, loopID string, limit int64) ([]model.OperatorOverride, error)

	// Audit
	AppendAudit(ctx context.Context, rec AuditRecord) error
	ListAudit(ctx context.Context, loopID string, limit int64) ([]AuditRecord, error)
	WatchAudit(ctx context.Context) <-chan AuditRecord

	// Sequences
	NextSequence(ctx context.Context, prefix string) (int64, error)

	// Completion / Saga support
	RecordPhase(ctx context.Context, record model.JournalEntry) error
	SetStateSynced(ctx context.Context, loopID string, commitSHA string) error
	SetStateUnresolved(ctx context.Context, loopID string, reason string) error

	// Locking
	ReadLock(ctx context.Context, loopID string) (locking.Record, error)
	PutLockIfRevision(ctx context.Context, lock model.LeaseLock, expectedRevision int64) (bool, error)
	DeleteLockIfRevision(ctx context.Context, loopID string, expectedRevision int64) (bool, error)
}
