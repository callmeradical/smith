package v1

import (
	"encoding/json"
	"time"

	"smith/internal/source/model"
)

// LoopState represents the current phase of a loop
type LoopState string

const (
	LoopStateUnresolved LoopState = "unresolved"
	LoopStateRunning    LoopState = "running"
	LoopStateSynced     LoopState = "synced"
	LoopStateFlatline   LoopState = "flatline"
	LoopStateCancelled  LoopState = "cancelled"
)

// LoopEnvironment represents the execution environment for a loop
type LoopEnvironment struct {
	Preset         string                 `json:"preset,omitempty"`
	Mise           *MiseEnvironment       `json:"mise,omitempty"`
	ContainerImage *ContainerImageProfile `json:"container_image,omitempty"`
	Dockerfile     *DockerfileProfile     `json:"dockerfile,omitempty"`
	Env            map[string]string      `json:"env,omitempty"`
	ResolvedMode   string                 `json:"resolved_mode,omitempty"`

	// Deprecated: Use ContainerImage instead
	ImageRef string `json:"image_ref,omitempty"`
	// Deprecated: Use ContainerImage instead
	ImagePullPolicy string `json:"image_pull_policy,omitempty"`
}

type MiseEnvironment struct {
	ToolVersionsFile string            `json:"tool_versions_file,omitempty"`
	Tools            map[string]string `json:"tools,omitempty"`
}

type ContainerImageProfile struct {
	Ref        string `json:"ref"`
	PullPolicy string `json:"pull_policy,omitempty"`
}

type DockerfileProfile struct {
	ContextDir     string            `json:"context_dir,omitempty"`
	DockerfilePath string            `json:"dockerfile_path,omitempty"`
	Target         string            `json:"target,omitempty"`
	BuildArgs      map[string]string `json:"build_args,omitempty"`
}

// LoopSkillMount represents a skill attached to a loop
type LoopSkillMount struct {
	Name      string            `json:"name"`
	Source    string            `json:"source"`
	Version   string            `json:"version,omitempty"`
	MountPath string            `json:"mount_path,omitempty"`
	ReadOnly  *bool             `json:"read_only,omitempty"`
	Config    map[string]string `json:"config,omitempty"`
}

// LoopPolicy defines the execution policy for a loop
type LoopPolicy struct {
	MaxAttempts      int           `json:"max_attempts"`
	BackoffInitial   time.Duration `json:"backoff_initial"`
	BackoffMax       time.Duration `json:"backoff_max"`
	Timeout          time.Duration `json:"timeout"`
	TerminateOnError bool          `json:"terminate_on_error"`
}

// Anomaly represents a task or issue that Smith should resolve
type Anomaly struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	SourceType    string            `json:"source_type"`
	SourceRef     string            `json:"source_ref"`
	ProviderID    string            `json:"provider_id"`
	Model         string            `json:"model"`
	Environment   LoopEnvironment   `json:"environment"`
	Skills        []LoopSkillMount  `json:"skills,omitempty"`
	Policy        LoopPolicy        `json:"policy"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	CorrelationID string            `json:"correlation_id"`
	SchemaVersion string            `json:"schema_version"`
}

// State represents the runtime state of a loop
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

// LoopWithRevision represents a loop state with its etcd revision
type LoopWithRevision struct {
	Record   State `json:"record"`
	Revision int64 `json:"revision"`
}

// JournalEntry represents a single log entry for a loop
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

// Handoff represents the result of a loop execution
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

// OperatorOverride represents an operator-initiated state change
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

// AuditRecord represents a security-relevant event
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

// Document represents a high-level requirement or PRD
type Document struct {
	ID            string            `json:"id"`
	ProjectID     string            `json:"project_id"`
	Title         string            `json:"title"`
	Content       string            `json:"content"`
	Format        string            `json:"format"`
	SourceType    string            `json:"source_type"`
	SourceRef     string            `json:"source_ref,omitempty"`
	Status        string            `json:"status"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CorrelationID string            `json:"correlation_id"`
	SchemaVersion string            `json:"schema_version"`
}

// GitHubIssue represents a GitHub issue source
type GitHubIssue struct {
	ID             string            `json:"id,omitempty"`
	Repository     string            `json:"repository"`
	Number         int               `json:"number"`
	Title          string            `json:"title"`
	Body           string            `json:"body,omitempty"`
	URL            string            `json:"url,omitempty"`
	Labels         []string          `json:"labels,omitempty"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// PRDTask represents a task extracted from a PRD
type PRDTask struct {
	ID          string            `json:"id,omitempty"`
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Section     string            `json:"section,omitempty"`
	SourceRef   string            `json:"source_ref,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Request/Response types for the API

type OverrideRequest struct {
	LoopID      string    `json:"loop_id"`
	TargetState LoopState `json:"target_state"`
	Reason      string    `json:"reason"`
	Actor       string    `json:"actor"`
}

type OverrideResponse struct {
	LoopID   string `json:"loop_id"`
	Status   string `json:"status"`
	State    State  `json:"state"`
	Revision int64  `json:"revision"`
}

type CostSummary struct {
	LoopID         string  `json:"loop_id"`
	ProviderID     string  `json:"provider_id,omitempty"`
	Model          string  `json:"model,omitempty"`
	EntryCount     int     `json:"entry_count"`
	TotalTokens    int64   `json:"total_tokens"`
	PromptTokens   int64   `json:"prompt_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
	LastActivityAt string  `json:"last_activity_at,omitempty"`
}

type AuthStartRequest struct {
	Actor string `json:"actor"`
}

type AuthCompleteRequest struct {
	Actor      string `json:"actor"`
	DeviceCode string `json:"device_code"`
}

type AuthAPIKeyRequest struct {
	Actor     string `json:"actor"`
	APIKey    string `json:"api_key"`
	AccountID string `json:"account_id"`
}

type ProjectCredentialUpsertRequest struct {
	Actor      string `json:"actor"`
	ProjectID  string `json:"project_id"`
	GitHubUser string `json:"github_user"`
	Credential string `json:"credential"`
}

type ProjectCredentialDeleteRequest struct {
	Actor     string `json:"actor"`
	ProjectID string `json:"project_id"`
}

type TerminalAttachRequest struct {
	Actor    string `json:"actor"`
	Terminal string `json:"terminal"`
}

type TerminalDetachRequest struct {
	Actor string `json:"actor"`
}

type TerminalCommandRequest struct {
	Actor   string `json:"actor"`
	Command string `json:"command"`
}

type LoopRuntimeResponse struct {
	LoopID        string `json:"loop_id"`
	Namespace     string `json:"namespace,omitempty"`
	PodName       string `json:"pod_name,omitempty"`
	ContainerName string `json:"container_name,omitempty"`
	PodPhase      string `json:"pod_phase,omitempty"`
	Attachable    bool   `json:"attachable"`
	Reason        string `json:"reason,omitempty"`
}

type LoopResponse struct {
	State       State            `json:"state"`
	Anomaly     *Anomaly         `json:"anomaly,omitempty"`
	Environment *LoopEnvironment `json:"environment,omitempty"`
}

type LoopDeleteRequest struct {
	Actor string `json:"actor"`
}

type DocumentRequest struct {
	ID         string            `json:"id,omitempty"`
	ProjectID  string            `json:"project_id"`
	Title      string            `json:"title"`
	Content    string            `json:"content"`
	Format     string            `json:"format"`
	SourceType string            `json:"source_type"`
	SourceRef  string            `json:"source_ref,omitempty"`
	Status     string            `json:"status,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type DocumentBuildRequest struct {
	Actor string `json:"actor"`
}

type LoopCreateRequest struct {
	LoopID         string            `json:"loop_id,omitempty"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	Title          string            `json:"title"`
	Description    string            `json:"description"`
	SourceType     string            `json:"source_type"`
	SourceRef      string            `json:"source_ref"`
	ProviderID     string            `json:"provider_id,omitempty"`
	Model          string            `json:"model,omitempty"`
	CorrelationID  string            `json:"correlation_id,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Environment    *LoopEnvironment  `json:"environment,omitempty"`
	Skills         []LoopSkillMount  `json:"skills,omitempty"`
}

type LoopBatchRequest struct {
	Loops []LoopCreateRequest `json:"loops"`
}

type LoopCreateResult struct {
	LoopID           string                     `json:"loop_id"`
	Status           string                     `json:"status"`
	Created          bool                       `json:"created"`
	Message          string                     `json:"message,omitempty"`
	Environment      LoopEnvironment            `json:"environment"`
	Skills           []LoopSkillMount           `json:"skills,omitempty"`
	ValidationReport *model.PRDValidationReport `json:"validation_report,omitempty"`
	HTTPCode         int                        `json:"http_code,omitempty"`
}

type GitHubIngressRequest struct {
	Issues   []GitHubIssue     `json:"issues"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type PRDIngressRequest struct {
	Format    string            `json:"format,omitempty"`
	SourceRef string            `json:"source_ref,omitempty"`
	Markdown  string            `json:"markdown,omitempty"`
	PRD       json.RawMessage   `json:"prd,omitempty"`
	Tasks     []PRDTask         `json:"tasks,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type IngressResult struct {
	ItemIndex int    `json:"item_index"`
	LoopID    string `json:"loop_id,omitempty"`
	SourceRef string `json:"source_ref,omitempty"`
	Status    string `json:"status"`
	Created   bool   `json:"created"`
	Message   string `json:"message,omitempty"`
}

type IngressSummary struct {
	Results []IngressResult `json:"results"`
	Summary struct {
		Created  int `json:"created"`
		Existing int `json:"existing"`
		Errors   int `json:"errors"`
	} `json:"summary"`
}

type LoopTraceResponse struct {
	LoopID      string             `json:"loop_id"`
	State       State              `json:"state"`
	Anomaly     *Anomaly           `json:"anomaly,omitempty"`
	Environment LoopEnvironment    `json:"environment,omitempty"`
	Journal     []JournalEntry     `json:"journal"`
	Handoffs    []Handoff          `json:"handoffs"`
	Overrides   []OperatorOverride `json:"overrides"`
	Audit       []AuditRecord      `json:"audit"`
}

type PresetCreateRequest struct {
	Name string `json:"name"`
}
