package model

import "time"

const SchemaVersion = SchemaVersionV1

type LoopState string

const (
	LoopStateUnresolved  LoopState = "unresolved"
	LoopStateOverwriting LoopState = "overwriting"
	LoopStateSynced      LoopState = "synced"
	LoopStateFlatline    LoopState = "flatline"
	LoopStateCancelled   LoopState = "cancelled"
)

type Anomaly struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	SourceType    string            `json:"source_type"`
	SourceRef     string            `json:"source_ref"`
	ProviderID    string            `json:"provider_id"`
	Model         string            `json:"model"`
	Policy        LoopPolicy        `json:"policy"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	CorrelationID string            `json:"correlation_id"`
	SchemaVersion string            `json:"schema_version"`
}

type LoopPolicy struct {
	MaxAttempts      int           `json:"max_attempts"`
	BackoffInitial   time.Duration `json:"backoff_initial"`
	BackoffMax       time.Duration `json:"backoff_max"`
	Timeout          time.Duration `json:"timeout"`
	TerminateOnError bool          `json:"terminate_on_error"`
}

type State struct {
	LoopID           string     `json:"loop_id"`
	State            LoopState  `json:"state"`
	Attempt          int        `json:"attempt"`
	Reason           string     `json:"reason,omitempty"`
	WorkerJobName    string     `json:"worker_job_name,omitempty"`
	LockHolder       string     `json:"lock_holder,omitempty"`
	ObservedRevision int64      `json:"observed_revision"`
	UpdatedAt        time.Time  `json:"updated_at"`
	LastHeartbeatAt  *time.Time `json:"last_heartbeat_at,omitempty"`
	CorrelationID    string     `json:"correlation_id"`
	SchemaVersion    string     `json:"schema_version"`
}

type StateRecord = State

type JournalEntry struct {
	LoopID        string            `json:"loop_id"`
	Sequence      int64             `json:"sequence"`
	Timestamp     time.Time         `json:"timestamp"`
	Phase         string            `json:"phase"`
	Level         string            `json:"level"`
	ActorType     string            `json:"actor_type"`
	ActorID       string            `json:"actor_id"`
	Message       string            `json:"message"`
	Command       string            `json:"command,omitempty"`
	ExitCode      *int              `json:"exit_code,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CorrelationID string            `json:"correlation_id"`
	SchemaVersion string            `json:"schema_version"`
}

type Handoff struct {
	LoopID            string            `json:"loop_id"`
	Sequence          int64             `json:"sequence"`
	Timestamp         time.Time         `json:"timestamp"`
	FinalDiffSummary  string            `json:"final_diff_summary"`
	ValidationState   string            `json:"validation_state"`
	ValidationDetails string            `json:"validation_details,omitempty"`
	NextSteps         string            `json:"next_steps"`
	ArtifactRefs      []string          `json:"artifact_refs,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	CorrelationID     string            `json:"correlation_id"`
	SchemaVersion     string            `json:"schema_version"`
}

type LeaseLock struct {
	LoopID        string    `json:"loop_id"`
	Holder        string    `json:"holder"`
	LeaseID       int64     `json:"lease_id"`
	AcquiredAt    time.Time `json:"acquired_at"`
	HeartbeatAt   time.Time `json:"heartbeat_at"`
	SchemaVersion string    `json:"schema_version"`
}

type OperatorOverride struct {
	LoopID        string    `json:"loop_id"`
	Sequence      int64     `json:"sequence"`
	Timestamp     time.Time `json:"timestamp"`
	Actor         string    `json:"actor"`
	Action        string    `json:"action"`
	TargetState   LoopState `json:"target_state"`
	Reason        string    `json:"reason"`
	CorrelationID string    `json:"correlation_id"`
	SchemaVersion string    `json:"schema_version"`
}
