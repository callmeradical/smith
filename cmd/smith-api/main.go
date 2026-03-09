package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"smith/internal/source/ingress"
	"smith/internal/source/model"
	"smith/internal/source/provider"
	"smith/internal/source/store"

	clientv3 "go.etcd.io/etcd/client/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultPort            = 8080
	defaultShutdownTimeout = 10 * time.Second
	defaultRuntimeReason   = "runtime pod not found"
)

type config struct {
	port                  int
	etcdEndpoints         []string
	etcdDialTimeout       time.Duration
	operatorToken         string
	authStoreBackend      string
	authStorePath         string
	authStoreK8sNamespace string
	authStoreK8sSecret    string
	authStoreK8sKey       string
	defaultPreset         string
	skillPolicy           model.SkillPolicy
	runtimeNamespace      string
	runtimeContainerName  string
}

type server struct {
	cfg         config
	store       *store.Store
	auth        *provider.AuthManager
	projectCred provider.ProjectCredentialStore
	presets     *presetCatalog
	skillPolicy model.SkillPolicy
	term        *terminalSessionStore
	runtimePods runtimePodReader
}

type overrideRequest struct {
	LoopID      string          `json:"loop_id"`
	TargetState model.LoopState `json:"target_state"`
	Reason      string          `json:"reason"`
	Actor       string          `json:"actor"`
}

type costSummary struct {
	LoopID         string  `json:"loop_id"`
	EntryCount     int     `json:"entry_count"`
	TotalTokens    int64   `json:"total_tokens"`
	PromptTokens   int64   `json:"prompt_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
	LastActivityAt string  `json:"last_activity_at,omitempty"`
}

type authStartRequest struct {
	Actor string `json:"actor"`
}

type authCompleteRequest struct {
	Actor      string `json:"actor"`
	DeviceCode string `json:"device_code"`
}

type authAPIKeyRequest struct {
	Actor     string `json:"actor"`
	APIKey    string `json:"api_key"`
	AccountID string `json:"account_id"`
}

type projectCredentialUpsertRequest struct {
	Actor      string `json:"actor"`
	ProjectID  string `json:"project_id"`
	GitHubUser string `json:"github_user"`
	Credential string `json:"credential"`
}

type projectCredentialDeleteRequest struct {
	Actor     string `json:"actor"`
	ProjectID string `json:"project_id"`
}

type terminalAttachRequest struct {
	Actor    string `json:"actor"`
	Terminal string `json:"terminal"`
}

type terminalDetachRequest struct {
	Actor string `json:"actor"`
}

type terminalCommandRequest struct {
	Actor   string `json:"actor"`
	Command string `json:"command"`
}

type loopRuntimeResponse struct {
	LoopID        string `json:"loop_id"`
	Namespace     string `json:"namespace,omitempty"`
	PodName       string `json:"pod_name,omitempty"`
	ContainerName string `json:"container_name,omitempty"`
	PodPhase      string `json:"pod_phase,omitempty"`
	Attachable    bool   `json:"attachable"`
	Reason        string `json:"reason,omitempty"`
}

type loopDeleteRequest struct {
	Actor string `json:"actor"`
}

type loopCreateRequest struct {
	LoopID         string                 `json:"loop_id,omitempty"`
	IdempotencyKey string                 `json:"idempotency_key,omitempty"`
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	SourceType     string                 `json:"source_type"`
	SourceRef      string                 `json:"source_ref"`
	ProviderID     string                 `json:"provider_id,omitempty"`
	Model          string                 `json:"model,omitempty"`
	CorrelationID  string                 `json:"correlation_id,omitempty"`
	Metadata       map[string]string      `json:"metadata,omitempty"`
	Environment    *model.LoopEnvironment `json:"environment,omitempty"`
	Skills         []model.LoopSkillMount `json:"skills,omitempty"`
}

type loopBatchRequest struct {
	Loops []loopCreateRequest `json:"loops"`
}

type loopCreateResult struct {
	LoopID      string                 `json:"loop_id"`
	Status      string                 `json:"status"`
	Created     bool                   `json:"created"`
	Message     string                 `json:"message,omitempty"`
	Environment model.LoopEnvironment  `json:"environment"`
	Skills      []model.LoopSkillMount `json:"skills,omitempty"`
	HTTPCode    int                    `json:"http_code,omitempty"`
}

type githubIngressRequest struct {
	Issues   []ingress.GitHubIssue `json:"issues"`
	Metadata map[string]string     `json:"metadata,omitempty"`
}

type prdIngressRequest struct {
	Format    string            `json:"format,omitempty"`
	SourceRef string            `json:"source_ref,omitempty"`
	Markdown  string            `json:"markdown,omitempty"`
	Tasks     []ingress.PRDTask `json:"tasks,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type ingressResult struct {
	ItemIndex int    `json:"item_index"`
	LoopID    string `json:"loop_id,omitempty"`
	SourceRef string `json:"source_ref,omitempty"`
	Status    string `json:"status"`
	Created   bool   `json:"created"`
	Message   string `json:"message,omitempty"`
}

type loopTraceResponse struct {
	LoopID      string                   `json:"loop_id"`
	State       model.StateRecord        `json:"state"`
	Anomaly     *model.Anomaly           `json:"anomaly,omitempty"`
	Environment model.LoopEnvironment    `json:"environment,omitempty"`
	Journal     []model.JournalEntry     `json:"journal"`
	Handoffs    []model.Handoff          `json:"handoffs"`
	Overrides   []model.OperatorOverride `json:"overrides"`
	Audit       []store.AuditRecord      `json:"audit"`
}

type presetCatalog struct {
	mu            sync.RWMutex
	defaultPreset string
	presets       map[string]struct{}
}

type presetCreateRequest struct {
	Name string `json:"name"`
}

type terminalSessionStore struct {
	mu       sync.Mutex
	attached map[string]map[string]struct{}
}

type runtimePodReader interface {
	List(ctx context.Context, namespace string, opts metav1.ListOptions) (*corev1.PodList, error)
}

type kubeRuntimePodReader struct {
	kube kubernetes.Interface
}

func (k kubeRuntimePodReader) List(ctx context.Context, namespace string, opts metav1.ListOptions) (*corev1.PodList, error) {
	return k.kube.CoreV1().Pods(namespace).List(ctx, opts)
}

func newTerminalSessionStore() *terminalSessionStore {
	return &terminalSessionStore{
		attached: map[string]map[string]struct{}{},
	}
}

func (t *terminalSessionStore) Attach(loopID, actor string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.attached[loopID] == nil {
		t.attached[loopID] = map[string]struct{}{}
	}
	t.attached[loopID][actor] = struct{}{}
	return len(t.attached[loopID])
}

func (t *terminalSessionStore) Detach(loopID, actor string) (bool, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	actors, ok := t.attached[loopID]
	if !ok {
		return false, 0
	}
	if _, found := actors[actor]; !found {
		return false, len(actors)
	}
	delete(actors, actor)
	if len(actors) == 0 {
		delete(t.attached, loopID)
		return true, 0
	}
	return true, len(actors)
}

func (t *terminalSessionStore) IsAttached(loopID, actor string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	actors, ok := t.attached[loopID]
	if !ok {
		return false
	}
	_, found := actors[actor]
	return found
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("smith-api config error: %v", err)
	}

	es, err := store.New(ctx, cfg.etcdEndpoints, cfg.etcdDialTimeout)
	if err != nil {
		log.Fatalf("smith-api etcd init failed: %v", err)
	}
	defer func() { _ = es.Close() }()

	tokenStore, err := newTokenStore(ctx, cfg)
	if err != nil {
		log.Fatalf("smith-api auth store init failed: %v", err)
	}
	projectCredStore, ok := tokenStore.(provider.ProjectCredentialStore)
	if !ok {
		log.Fatalf("smith-api auth store does not support project credentials")
	}

	authManager := provider.NewAuthManager(
		provider.ProviderCodex,
		tokenStore,
		provider.NewMockDeviceAuthClient(),
		&auditBridge{store: es},
	)
	runtimePods, err := newRuntimePodReader()
	if err != nil {
		log.Printf("smith-api runtime pod lookup unavailable: %v", err)
	}

	s := &server{
		cfg:         cfg,
		store:       es,
		auth:        authManager,
		projectCred: projectCredStore,
		presets:     newPresetCatalog(cfg.defaultPreset),
		skillPolicy: cfg.skillPolicy,
		term:        newTerminalSessionStore(),
		runtimePods: runtimePods,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/readyz", s.handleReady)
	mux.HandleFunc("/v1/loops", s.handleLoops)
	mux.HandleFunc("/v1/loops/", s.handleLoopByID)
	mux.HandleFunc("/v1/environment/presets", s.handleEnvironmentPresets)
	mux.HandleFunc("/v1/environment/presets/", s.handleEnvironmentPresetByName)
	mux.HandleFunc("/v1/ingress/github/issues", s.handleIngressGitHubIssues)
	mux.HandleFunc("/v1/ingress/prd", s.handleIngressPRD)
	mux.HandleFunc("/v1/control/override", s.handleOverride)
	mux.HandleFunc("/v1/audit", s.handleAudit)
	mux.HandleFunc("/v1/reporting/cost", s.handleCost)
	mux.HandleFunc("/v1/auth/codex/connect/start", s.handleCodexAuthStart)
	mux.HandleFunc("/v1/auth/codex/connect/complete", s.handleCodexAuthComplete)
	mux.HandleFunc("/v1/auth/codex/connect/api-key", s.handleCodexAuthAPIKey)
	mux.HandleFunc("/v1/auth/codex/status", s.handleCodexAuthStatus)
	mux.HandleFunc("/v1/auth/codex/credential", s.handleCodexAuthCredential)
	mux.HandleFunc("/v1/auth/codex/disconnect", s.handleCodexAuthDisconnect)
	mux.HandleFunc("/v1/projects/credentials/github", s.handleProjectGitHubCredential)

	addr := fmt.Sprintf(":%d", cfg.port)
	httpServer := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("smith-api listening on %s", addr)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Printf("smith-api shutdown requested")
	case serveErr := <-errCh:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("smith-api failed: %v", serveErr)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("smith-api shutdown failed: %v", err)
	}
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *server) handleReady(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

func (s *server) handleLoops(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		states, err := s.store.ListStates(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, states)
	case http.MethodPost:
		s.handleLoopCreate(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleLoopCreate(w http.ResponseWriter, r *http.Request) {
	raw, err := ioReadAllLimit(r.Body, 1<<20)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var batch loopBatchRequest
	if err := json.Unmarshal(raw, &batch); err == nil && len(batch.Loops) > 0 {
		results := make([]loopCreateResult, 0, len(batch.Loops))
		for _, req := range batch.Loops {
			res := s.createOneLoop(r.Context(), req)
			results = append(results, res)
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": results})
		return
	}

	var single loopCreateRequest
	if err := json.Unmarshal(raw, &single); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	result := s.createOneLoop(r.Context(), single)
	code := http.StatusCreated
	if !result.Created {
		code = http.StatusOK
	}
	if result.HTTPCode != 0 {
		code = result.HTTPCode
	}
	writeJSON(w, code, result)
}

func (s *server) handleIngressGitHubIssues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req githubIngressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if len(req.Issues) == 0 {
		writeErr(w, http.StatusBadRequest, "at least one issue is required")
		return
	}
	results := make([]ingressResult, 0, len(req.Issues))
	for i, issue := range req.Issues {
		draft, err := ingress.GitHubIssueToDraft(issue)
		if err != nil {
			results = append(results, ingressResult{
				ItemIndex: i,
				Status:    "error",
				Message:   err.Error(),
			})
			continue
		}
		metadata := copyStringMap(req.Metadata)
		for k, v := range draft.Metadata {
			metadata[k] = v
		}
		res := s.createOneLoop(r.Context(), loopCreateRequest{
			IdempotencyKey: draft.IdempotencyKey,
			Title:          draft.Title,
			Description:    draft.Description,
			SourceType:     draft.SourceType,
			SourceRef:      draft.SourceRef,
			Metadata:       metadata,
		})
		results = append(results, ingressResult{
			ItemIndex: i,
			LoopID:    res.LoopID,
			SourceRef: draft.SourceRef,
			Status:    res.Status,
			Created:   res.Created,
			Message:   res.Message,
		})
	}
	writeJSON(w, http.StatusOK, ingressSummary(results))
}

func (s *server) handleIngressPRD(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req prdIngressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		if strings.TrimSpace(req.Markdown) != "" {
			format = "markdown"
		} else {
			format = "json"
		}
	}
	baseMetadata := copyStringMap(req.Metadata)

	var (
		drafts []ingress.LoopDraft
		errs   []ingress.ParseError
	)
	switch format {
	case "markdown", "md":
		drafts, errs = ingress.ParsePRDMarkdown(req.Markdown, req.SourceRef, baseMetadata)
	case "json", "structured":
		drafts, errs = ingress.PRDTasksToDrafts(req.Tasks, req.SourceRef, baseMetadata)
	default:
		writeErr(w, http.StatusBadRequest, "format must be markdown or json")
		return
	}

	results := make([]ingressResult, 0, len(drafts)+len(errs))
	for _, parseErr := range errs {
		results = append(results, ingressResult{
			ItemIndex: parseErr.ItemIndex,
			SourceRef: parseErr.SourceRef,
			Status:    "error",
			Message:   parseErr.Message,
		})
	}
	for i, draft := range drafts {
		res := s.createOneLoop(r.Context(), loopCreateRequest{
			IdempotencyKey: draft.IdempotencyKey,
			Title:          draft.Title,
			Description:    draft.Description,
			SourceType:     draft.SourceType,
			SourceRef:      draft.SourceRef,
			Metadata:       draft.Metadata,
		})
		results = append(results, ingressResult{
			ItemIndex: i,
			LoopID:    res.LoopID,
			SourceRef: draft.SourceRef,
			Status:    res.Status,
			Created:   res.Created,
			Message:   res.Message,
		})
	}
	writeJSON(w, http.StatusOK, ingressSummary(results))
}

func (s *server) createOneLoop(ctx context.Context, req loopCreateRequest) loopCreateResult {
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.SourceType = strings.TrimSpace(req.SourceType)
	req.SourceRef = strings.TrimSpace(req.SourceRef)
	req.CorrelationID = strings.TrimSpace(req.CorrelationID)
	if req.CorrelationID == "" {
		req.CorrelationID = fmt.Sprintf("corr-%d", time.Now().UTC().UnixNano())
	}
	if req.Title == "" || req.SourceType == "" || req.SourceRef == "" {
		return loopCreateResult{Status: "error", Message: "title, source_type, and source_ref are required", HTTPCode: http.StatusBadRequest}
	}

	reg := provider.NewDefaultRegistry()
	selection, err := reg.Resolve(req.ProviderID, req.Model)
	if err != nil {
		return loopCreateResult{Status: "error", Message: err.Error(), HTTPCode: http.StatusBadRequest}
	}
	environment, err := model.NormalizeLoopEnvironmentWithPolicy(req.Environment, s.presets.Policy())
	if err != nil {
		return loopCreateResult{Status: "error", Message: err.Error(), HTTPCode: http.StatusBadRequest}
	}
	skills, skillAudit, err := model.NormalizeLoopSkillsWithPolicy(req.Skills, selection.ProviderID, s.skillPolicy)
	if err != nil {
		return loopCreateResult{Status: "error", Message: err.Error(), HTTPCode: http.StatusBadRequest}
	}

	loopID := strings.TrimSpace(req.LoopID)
	if loopID == "" {
		loopID = deriveLoopID(req.IdempotencyKey, req.SourceType, req.SourceRef)
	}
	if existing, found, err := s.store.GetState(ctx, loopID); err == nil && found {
		stored, storedFound, _ := s.store.GetAnomaly(ctx, loopID)
		if !storedFound {
			stored.Environment = environment
		}
		return loopCreateResult{
			LoopID:      loopID,
			Status:      string(existing.Record.State),
			Created:     false,
			Message:     "existing loop returned via idempotency or explicit loop_id",
			Environment: stored.Environment,
			Skills:      stored.Skills,
			HTTPCode:    http.StatusOK,
		}
	}

	anomaly := model.Anomaly{
		ID:            loopID,
		Title:         req.Title,
		Description:   req.Description,
		SourceType:    req.SourceType,
		SourceRef:     req.SourceRef,
		ProviderID:    selection.ProviderID,
		Model:         selection.Model,
		Environment:   environment,
		Skills:        skills,
		Metadata:      withIdempotency(req.Metadata, req.IdempotencyKey),
		CorrelationID: req.CorrelationID,
		Policy: model.LoopPolicy{
			MaxAttempts:      3,
			BackoffInitial:   5 * time.Second,
			BackoffMax:       2 * time.Minute,
			Timeout:          30 * time.Minute,
			TerminateOnError: false,
		},
	}
	if err := s.store.PutAnomaly(ctx, anomaly); err != nil {
		return loopCreateResult{LoopID: loopID, Status: "error", Message: err.Error(), HTTPCode: http.StatusInternalServerError}
	}
	state := model.StateRecord{
		LoopID:        loopID,
		State:         model.LoopStateUnresolved,
		Attempt:       0,
		Reason:        "created-via-api",
		CorrelationID: req.CorrelationID,
	}
	if _, err := s.store.PutState(ctx, state, 0); err != nil && !errors.Is(err, store.ErrRevisionMismatch) {
		return loopCreateResult{LoopID: loopID, Status: "error", Message: err.Error(), HTTPCode: http.StatusInternalServerError}
	}
	_ = s.store.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "ingress",
		Level:         "info",
		ActorType:     "api",
		ActorID:       "smith-api",
		Message:       "loop created from ingress",
		CorrelationID: req.CorrelationID,
		Metadata: map[string]string{
			"source_type":                       req.SourceType,
			"source_ref":                        req.SourceRef,
			"environment_mode":                  environment.ResolvedMode,
			"skill_mount_count":                 strconv.Itoa(len(skills)),
			"skill_default_read_only_count":     strconv.Itoa(skillAudit.DefaultReadOnlyCount),
			"skill_writable_mount_count":        strconv.Itoa(skillAudit.WritableCount),
			"skill_writable_override_audit_cnt": strconv.Itoa(skillAudit.WritableOverrideCount),
		},
	})

	return loopCreateResult{
		LoopID:      loopID,
		Status:      string(model.LoopStateUnresolved),
		Created:     true,
		Environment: environment,
		Skills:      skills,
		HTTPCode:    http.StatusCreated,
	}
}

func (s *server) handleEnvironmentPresets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"default_preset": s.presets.Default(),
			"presets":        s.presets.List(),
		})
	case http.MethodPost:
		var req presetCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if err := s.presets.Upsert(req.Name); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"name": strings.ToLower(strings.TrimSpace(req.Name))})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleEnvironmentPresetByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/environment/presets/"))
	if name == "" {
		writeErr(w, http.StatusBadRequest, "preset name is required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !s.presets.Has(name) {
			writeErr(w, http.StatusNotFound, "preset not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"name": strings.ToLower(name)})
	case http.MethodPut:
		if err := s.presets.Upsert(name); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"name": strings.ToLower(name)})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleLoopByID(w http.ResponseWriter, r *http.Request) {
	loopID, route := splitLoopRoute(r.URL.Path)
	if strings.TrimSpace(loopID) == "" {
		writeErr(w, http.StatusBadRequest, "loop id is required")
		return
	}
	if route == "" {
		switch r.Method {
		case http.MethodGet:
			state, found, err := s.store.GetState(r.Context(), loopID)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !found {
				writeErr(w, http.StatusNotFound, "loop not found")
				return
			}
			anomaly, anomalyFound, err := s.store.GetAnomaly(r.Context(), loopID)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !anomalyFound {
				writeJSON(w, http.StatusOK, map[string]any{"state": state})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"state":       state,
				"anomaly":     anomaly,
				"environment": anomaly.Environment,
			})
			return
		case http.MethodDelete:
			if !s.authorized(r) {
				writeErr(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			state, found, err := s.store.GetState(r.Context(), loopID)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !found {
				writeErr(w, http.StatusNotFound, "loop not found")
				return
			}
			if isActiveLoopState(state.Record.State) {
				writeErr(w, http.StatusConflict, "cannot delete active loop")
				return
			}
			var req loopDeleteRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			actor := strings.TrimSpace(req.Actor)
			if actor == "" {
				actor = "operator"
			}
			if err := s.store.DeleteLoop(r.Context(), loopID); err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			_ = s.store.AppendAudit(r.Context(), store.AuditRecord{
				Actor:         actor,
				Action:        "delete-loop",
				TargetLoopID:  loopID,
				CorrelationID: state.Record.CorrelationID,
				Metadata: map[string]string{
					"final_state": string(state.Record.State),
				},
			})
			writeJSON(w, http.StatusOK, map[string]any{
				"loop_id": loopID,
				"status":  "deleted",
				"actor":   actor,
			})
			return
		default:
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
	}

	if route == "journal" {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		limit := int64(parseIntDefault(r.URL.Query().Get("limit"), 500))
		journal, err := s.store.ListJournal(r.Context(), loopID, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, journal)
		return
	}
	if route == "control/attach" {
		s.handleLoopAttach(w, r, loopID)
		return
	}
	if route == "control/detach" {
		s.handleLoopDetach(w, r, loopID)
		return
	}
	if route == "control/command" {
		s.handleLoopControlCommand(w, r, loopID)
		return
	}
	if route == "runtime" {
		s.handleLoopRuntime(w, r, loopID)
		return
	}
	if route == "handoffs" {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		limit := int64(parseIntDefault(r.URL.Query().Get("limit"), 100))
		handoffs, err := s.store.ListHandoffs(r.Context(), loopID, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, handoffs)
		return
	}
	if route == "overrides" {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		limit := int64(parseIntDefault(r.URL.Query().Get("limit"), 100))
		overrides, err := s.store.ListOverrides(r.Context(), loopID, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, overrides)
		return
	}
	if route == "trace" {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleLoopTrace(w, r, loopID)
		return
	}
	if route == "journal/stream" {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleJournalStream(w, r, loopID)
		return
	}

	writeErr(w, http.StatusNotFound, "endpoint not found")
}

func splitLoopRoute(path string) (loopID string, route string) {
	remainder := strings.TrimPrefix(path, "/v1/loops/")
	remainder = strings.TrimPrefix(remainder, "/")
	if remainder == "" {
		return "", ""
	}
	for _, suffix := range []string{
		"/journal/stream",
		"/control/attach",
		"/control/detach",
		"/control/command",
		"/runtime",
		"/handoffs",
		"/overrides",
		"/journal",
		"/trace",
	} {
		if strings.HasSuffix(remainder, suffix) {
			loopID = strings.TrimSuffix(remainder, suffix)
			loopID = strings.TrimSuffix(loopID, "/")
			return loopID, strings.TrimPrefix(suffix, "/")
		}
	}
	return remainder, ""
}

func (s *server) handleLoopRuntime(w http.ResponseWriter, r *http.Request, loopID string) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	state, found, err := s.store.GetState(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}
	writeJSON(w, http.StatusOK, s.resolveLoopRuntime(r.Context(), loopID, state.Record))
}

func (s *server) resolveLoopRuntime(ctx context.Context, loopID string, state model.StateRecord) loopRuntimeResponse {
	out := loopRuntimeResponse{
		LoopID:        loopID,
		Namespace:     runtimeNamespaceForConfig(s.cfg),
		ContainerName: runtimeContainerForConfig(s.cfg),
		Attachable:    false,
	}
	if !isActiveLoopState(state.State) {
		out.Reason = "loop not active"
		return out
	}
	if strings.TrimSpace(state.WorkerJobName) == "" || s.runtimePods == nil {
		out.Reason = defaultRuntimeReason
		return out
	}
	pod, found, err := s.findRuntimePod(ctx, out.Namespace, state.WorkerJobName)
	if err != nil || !found {
		out.Reason = defaultRuntimeReason
		return out
	}
	out.PodName = pod.Name
	out.PodPhase = string(pod.Status.Phase)

	containerName, ok := resolveRuntimeContainerName(pod, out.ContainerName)
	out.ContainerName = containerName
	if !ok {
		out.Reason = "runtime container not found"
		return out
	}
	if pod.Status.Phase != corev1.PodRunning {
		out.Reason = "runtime pod not running"
		return out
	}
	out.Attachable = true
	out.Reason = ""
	return out
}

func (s *server) findRuntimePod(ctx context.Context, namespace, workerJobName string) (corev1.Pod, bool, error) {
	if s.runtimePods == nil {
		return corev1.Pod{}, false, nil
	}
	list, err := s.runtimePods.List(ctx, namespace, metav1.ListOptions{
		LabelSelector: "job-name=" + strings.TrimSpace(workerJobName),
	})
	if err != nil {
		return corev1.Pod{}, false, err
	}
	if list == nil || len(list.Items) == 0 {
		return corev1.Pod{}, false, nil
	}
	best := list.Items[0]
	for i := 1; i < len(list.Items); i++ {
		if betterRuntimePod(list.Items[i], best) {
			best = list.Items[i]
		}
	}
	return best, true, nil
}

func betterRuntimePod(a, b corev1.Pod) bool {
	aScore := runtimePodScore(a.Status.Phase)
	bScore := runtimePodScore(b.Status.Phase)
	if aScore != bScore {
		return aScore > bScore
	}
	return a.CreationTimestamp.After(b.CreationTimestamp.Time)
}

func runtimePodScore(phase corev1.PodPhase) int {
	switch phase {
	case corev1.PodRunning:
		return 3
	case corev1.PodPending:
		return 2
	case corev1.PodUnknown:
		return 1
	default:
		return 0
	}
}

func resolveRuntimeContainerName(pod corev1.Pod, preferred string) (string, bool) {
	preferred = strings.TrimSpace(preferred)
	if preferred != "" {
		for _, container := range pod.Spec.Containers {
			if container.Name == preferred {
				return preferred, true
			}
		}
	}
	if len(pod.Spec.Containers) == 0 {
		return "", false
	}
	return pod.Spec.Containers[0].Name, true
}

func (s *server) handleLoopAttach(w http.ResponseWriter, r *http.Request, loopID string) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	state, found, err := s.store.GetState(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}
	if !isActiveLoopState(state.Record.State) {
		writeErr(w, http.StatusConflict, "loop is not active")
		return
	}

	var req terminalAttachRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "operator"
	}
	terminal := strings.TrimSpace(req.Terminal)
	if terminal == "" {
		terminal = "unknown"
	}
	attachCount := s.term.Attach(loopID, actor)

	_ = s.store.AppendAudit(r.Context(), store.AuditRecord{
		Actor:         actor,
		Action:        "attach-terminal",
		TargetLoopID:  loopID,
		CorrelationID: state.Record.CorrelationID,
		Metadata: map[string]string{
			"terminal":     terminal,
			"attach_count": strconv.Itoa(attachCount),
		},
	})
	_ = s.store.AppendJournal(r.Context(), model.JournalEntry{
		LoopID:        loopID,
		Phase:         "operator",
		Level:         "info",
		ActorType:     "operator",
		ActorID:       actor,
		Message:       "terminal attached",
		CorrelationID: state.Record.CorrelationID,
		Metadata: map[string]string{
			"terminal":     terminal,
			"attach_count": strconv.Itoa(attachCount),
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"loop_id":      loopID,
		"status":       "attached",
		"actor":        actor,
		"attach_count": attachCount,
	})
}

func (s *server) handleLoopDetach(w http.ResponseWriter, r *http.Request, loopID string) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	state, found, err := s.store.GetState(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}

	var req terminalDetachRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "operator"
	}

	detached, attachCount := s.term.Detach(loopID, actor)
	if !detached {
		writeErr(w, http.StatusConflict, "actor is not attached")
		return
	}

	_ = s.store.AppendAudit(r.Context(), store.AuditRecord{
		Actor:         actor,
		Action:        "detach-terminal",
		TargetLoopID:  loopID,
		CorrelationID: state.Record.CorrelationID,
		Metadata: map[string]string{
			"attach_count": strconv.Itoa(attachCount),
		},
	})
	_ = s.store.AppendJournal(r.Context(), model.JournalEntry{
		LoopID:        loopID,
		Phase:         "operator",
		Level:         "info",
		ActorType:     "operator",
		ActorID:       actor,
		Message:       "terminal detached",
		CorrelationID: state.Record.CorrelationID,
		Metadata: map[string]string{
			"attach_count": strconv.Itoa(attachCount),
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"loop_id":      loopID,
		"status":       "detached",
		"actor":        actor,
		"attach_count": attachCount,
	})
}

func (s *server) handleLoopControlCommand(w http.ResponseWriter, r *http.Request, loopID string) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	state, found, err := s.store.GetState(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}
	if !isActiveLoopState(state.Record.State) {
		writeErr(w, http.StatusConflict, "loop is not active")
		return
	}
	var req terminalCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "operator"
	}
	command := strings.TrimSpace(req.Command)
	if command == "" {
		writeErr(w, http.StatusBadRequest, "command is required")
		return
	}
	if !s.term.IsAttached(loopID, actor) {
		writeErr(w, http.StatusConflict, "actor must attach before issuing commands")
		return
	}
	_ = s.store.AppendAudit(r.Context(), store.AuditRecord{
		Actor:         actor,
		Action:        "terminal-command",
		TargetLoopID:  loopID,
		CorrelationID: state.Record.CorrelationID,
		Metadata: map[string]string{
			"command": command,
		},
	})
	_ = s.store.AppendJournal(r.Context(), model.JournalEntry{
		LoopID:        loopID,
		Phase:         "operator",
		Level:         "warn",
		ActorType:     "operator",
		ActorID:       actor,
		Message:       "terminal command issued",
		CorrelationID: state.Record.CorrelationID,
		Metadata: map[string]string{
			"command": command,
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"loop_id":   loopID,
		"status":    "accepted",
		"actor":     actor,
		"command":   command,
		"delivered": false,
	})
}

func isActiveLoopState(state model.LoopState) bool {
	switch state {
	case model.LoopStateUnresolved, model.LoopStateOverwriting:
		return true
	default:
		return false
	}
}

func (s *server) handleLoopTrace(w http.ResponseWriter, r *http.Request, loopID string) {
	limit := int64(parseIntDefault(r.URL.Query().Get("limit"), 500))
	state, found, err := s.store.GetState(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}

	out := loopTraceResponse{
		LoopID:    loopID,
		State:     state.Record,
		Journal:   []model.JournalEntry{},
		Handoffs:  []model.Handoff{},
		Overrides: []model.OperatorOverride{},
		Audit:     []store.AuditRecord{},
	}

	anomaly, anomalyFound, err := s.store.GetAnomaly(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if anomalyFound {
		out.Anomaly = &anomaly
		out.Environment = anomaly.Environment
	}

	out.Journal, err = s.store.ListJournal(r.Context(), loopID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Handoffs, err = s.store.ListHandoffs(r.Context(), loopID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Overrides, err = s.store.ListOverrides(r.Context(), loopID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Audit, err = s.store.ListAudit(r.Context(), loopID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleJournalStream(w http.ResponseWriter, r *http.Request, loopID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	sinceSeq := parseInt64Default(r.URL.Query().Get("since_seq"), 0)
	if sinceSeq < 0 {
		sinceSeq = 0
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(event string, payload any) error {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	_ = send("ready", map[string]any{"loop_id": loopID, "since_seq": sinceSeq})
	initial, rev, err := s.listJournalSinceWithRevision(r.Context(), loopID, sinceSeq)
	if err != nil {
		_ = send("error", map[string]string{"error": err.Error()})
		return
	}
	for _, entry := range initial {
		if err := send("entry", map[string]any{
			"entry":      entry,
			"emitted_at": time.Now().UTC().Format(time.RFC3339Nano),
		}); err != nil {
			return
		}
		sinceSeq = entry.Sequence
	}

	watchCh := s.store.Client().Watch(
		r.Context(),
		model.JournalPrefix(loopID)+"/",
		clientv3.WithPrefix(),
		clientv3.WithRev(rev+1),
	)

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			if _, err := fmt.Fprintf(w, ": keepalive %d\n\n", sinceSeq); err != nil {
				return
			}
			flusher.Flush()
		case resp, ok := <-watchCh:
			if !ok {
				return
			}
			if err := resp.Err(); err != nil {
				_ = send("error", map[string]string{"error": err.Error()})
				return
			}
			for _, event := range resp.Events {
				if event.Type != clientv3.EventTypePut || len(event.Kv.Value) == 0 {
					continue
				}
				var entry model.JournalEntry
				if err := json.Unmarshal(event.Kv.Value, &entry); err != nil {
					continue
				}
				if entry.Sequence <= sinceSeq {
					continue
				}
				if err := send("entry", map[string]any{
					"entry":      entry,
					"emitted_at": time.Now().UTC().Format(time.RFC3339Nano),
				}); err != nil {
					return
				}
				sinceSeq = entry.Sequence
			}
		}
	}
}

func (s *server) listJournalSinceWithRevision(ctx context.Context, loopID string, sinceSeq int64) ([]model.JournalEntry, int64, error) {
	resp, err := s.store.Client().Get(
		ctx,
		model.JournalPrefix(loopID)+"/",
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
	)
	if err != nil {
		return nil, 0, err
	}
	entries := make([]model.JournalEntry, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var entry model.JournalEntry
		if err := json.Unmarshal(kv.Value, &entry); err != nil {
			continue
		}
		if entry.Sequence <= sinceSeq {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, resp.Header.Revision, nil
}

func (s *server) handleOverride(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req overrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.LoopID = strings.TrimSpace(req.LoopID)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Actor = strings.TrimSpace(req.Actor)
	if req.Actor == "" {
		req.Actor = "operator"
	}
	if req.LoopID == "" || req.Reason == "" {
		writeErr(w, http.StatusBadRequest, "loop_id and reason are required")
		return
	}

	state, found, err := s.store.GetState(r.Context(), req.LoopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}
	if !model.IsValidTransition(state.Record.State, req.TargetState) {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid transition %s -> %s", state.Record.State, req.TargetState))
		return
	}

	next := state.Record
	next.State = req.TargetState
	next.Reason = req.Reason
	next.LockHolder = "operator-override"
	rev, err := s.store.PutState(r.Context(), next, state.Revision)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrRevisionMismatch) {
			status = http.StatusConflict
		}
		writeErr(w, status, err.Error())
		return
	}

	override := model.OperatorOverride{
		LoopID:        req.LoopID,
		Actor:         req.Actor,
		Action:        "override-state",
		TargetState:   req.TargetState,
		Reason:        req.Reason,
		CorrelationID: next.CorrelationID,
	}
	_ = s.store.AppendOverride(r.Context(), override)
	_ = s.store.AppendAudit(r.Context(), store.AuditRecord{
		Actor:         req.Actor,
		Action:        "override-state",
		TargetLoopID:  req.LoopID,
		Reason:        req.Reason,
		CorrelationID: next.CorrelationID,
		Metadata: map[string]string{
			"target_state": string(req.TargetState),
			"revision":     strconv.FormatInt(rev, 10),
		},
	})
	_ = s.store.AppendJournal(r.Context(), model.JournalEntry{
		LoopID:        req.LoopID,
		Phase:         "operator",
		Level:         "warn",
		ActorType:     "operator",
		ActorID:       req.Actor,
		Message:       "manual override applied",
		CorrelationID: next.CorrelationID,
		Metadata: map[string]string{
			"target_state": string(req.TargetState),
			"reason":       req.Reason,
		},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"loop_id":  req.LoopID,
		"state":    req.TargetState,
		"revision": rev,
	})
}

func (s *server) handleCost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	loopID := strings.TrimSpace(r.URL.Query().Get("loop_id"))
	if loopID == "" {
		writeErr(w, http.StatusBadRequest, "loop_id is required")
		return
	}
	entries, err := s.store.ListJournal(r.Context(), loopID, 0)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := costSummary{LoopID: loopID, EntryCount: len(entries)}
	for _, entry := range entries {
		if entry.Timestamp.After(parseRFC3339(out.LastActivityAt)) {
			out.LastActivityAt = entry.Timestamp.UTC().Format(time.RFC3339)
		}
		out.TotalTokens += parseInt64Default(entry.Metadata["token_total"], 0)
		out.PromptTokens += parseInt64Default(entry.Metadata["token_prompt"], 0)
		out.OutputTokens += parseInt64Default(entry.Metadata["token_output"], 0)
		out.TotalCostUSD += parseFloatDefault(entry.Metadata["cost_usd"], 0)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	loopID := strings.TrimSpace(r.URL.Query().Get("loop_id"))
	limit := int64(parseIntDefault(r.URL.Query().Get("limit"), 500))
	records, err := s.store.ListAudit(r.Context(), loopID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (s *server) handleCodexAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req authStartRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	session, err := s.auth.StartConnect(r.Context(), strings.TrimSpace(req.Actor))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *server) handleCodexAuthComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req authCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	token, err := s.auth.CompleteConnect(r.Context(), strings.TrimSpace(req.Actor), strings.TrimSpace(req.DeviceCode))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connected":       true,
		"expires_at":      token.ExpiresAt.UTC().Format(time.RFC3339),
		"account_id":      token.AccountID,
		"auth_method":     token.AuthMethod,
		"connected_at":    formatRFC3339OrEmpty(token.ConnectedAt),
		"last_refresh_at": formatRFC3339OrEmpty(token.LastRefreshAt),
	})
}

func (s *server) handleCodexAuthAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req authAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	token, err := s.auth.ConnectAPIKey(
		r.Context(),
		strings.TrimSpace(req.Actor),
		strings.TrimSpace(req.APIKey),
		strings.TrimSpace(req.AccountID),
	)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connected":       true,
		"expires_at":      token.ExpiresAt.UTC().Format(time.RFC3339),
		"account_id":      token.AccountID,
		"auth_method":     token.AuthMethod,
		"connected_at":    formatRFC3339OrEmpty(token.ConnectedAt),
		"last_refresh_at": formatRFC3339OrEmpty(token.LastRefreshAt),
	})
}

func (s *server) handleCodexAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	status, err := s.auth.Status(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{
		"connected": status.Connected,
		"provider":  provider.ProviderCodex,
	}
	if status.Connected {
		out["expires_at"] = status.ExpiresAt.UTC().Format(time.RFC3339)
		out["account_id"] = status.AccountID
		out["auth_method"] = status.AuthMethod
		out["connected_at"] = formatRFC3339OrEmpty(status.ConnectedAt)
		out["last_refresh_at"] = formatRFC3339OrEmpty(status.LastRefreshAt)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleCodexAuthCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	cred, err := s.auth.StoredCredential(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{
		"connected": cred.Connected,
		"provider":  provider.ProviderCodex,
	}
	if !cred.Connected {
		writeJSON(w, http.StatusOK, out)
		return
	}
	out["auth_method"] = cred.AuthMethod
	out["account_id"] = cred.AccountID
	if strings.EqualFold(cred.AuthMethod, "api_key") {
		out["api_key_masked"] = maskCredentialValue(cred.APIKey)
		reveal := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("reveal")), "true")
		if reveal {
			out["api_key"] = cred.APIKey
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleCodexAuthDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	actor := strings.TrimSpace(r.URL.Query().Get("actor"))
	if actor == "" {
		var req authStartRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		actor = strings.TrimSpace(req.Actor)
	}
	if err := s.auth.Disconnect(r.Context(), actor); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connected": false})
}

func (s *server) handleProjectGitHubCredential(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.projectCred == nil {
		writeErr(w, http.StatusInternalServerError, "project credential store unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
		if projectID == "" {
			writeErr(w, http.StatusBadRequest, "project_id is required")
			return
		}
		cred, found, err := s.projectCred.GetProjectCredential(r.Context(), projectID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out := map[string]any{
			"project_id":     projectID,
			"credential_set": found,
		}
		if found {
			out["github_user"] = strings.TrimSpace(cred.GitHubUser)
			out["credential_masked"] = maskCredentialValue(cred.PAT)
			out["updated_at"] = formatRFC3339OrEmpty(cred.UpdatedAt)
			if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("reveal")), "true") {
				out["credential"] = cred.PAT
			}
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var req projectCredentialUpsertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		projectID := strings.TrimSpace(req.ProjectID)
		if projectID == "" {
			writeErr(w, http.StatusBadRequest, "project_id is required")
			return
		}
		credential := strings.TrimSpace(req.Credential)
		if credential == "" {
			writeErr(w, http.StatusBadRequest, "credential is required")
			return
		}
		cred := provider.ProjectCredential{
			GitHubUser: strings.TrimSpace(req.GitHubUser),
			PAT:        credential,
			UpdatedAt:  time.Now().UTC(),
		}
		if err := s.projectCred.PutProjectCredential(r.Context(), projectID, cred); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"project_id":        projectID,
			"credential_set":    true,
			"github_user":       cred.GitHubUser,
			"credential_masked": maskCredentialValue(cred.PAT),
			"updated_at":        formatRFC3339OrEmpty(cred.UpdatedAt),
		})
	case http.MethodDelete:
		projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
		if projectID == "" {
			var req projectCredentialDeleteRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			projectID = strings.TrimSpace(req.ProjectID)
		}
		if projectID == "" {
			writeErr(w, http.StatusBadRequest, "project_id is required")
			return
		}
		if err := s.projectCred.DeleteProjectCredential(r.Context(), projectID); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"project_id":     projectID,
			"credential_set": false,
		})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) authorized(r *http.Request) bool {
	token := strings.TrimSpace(s.cfg.operatorToken)
	if token == "" {
		return true
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	return strings.TrimSpace(strings.TrimPrefix(auth, prefix)) == token
}

func newTokenStore(_ context.Context, cfg config) (provider.TokenStore, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.authStoreBackend))
	switch backend {
	case "", "file":
		return provider.NewFileTokenStore(cfg.authStorePath), nil
	case "kubernetes", "k8s":
		restConfig, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("kubernetes in-cluster config: %w", err)
		}
		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return nil, fmt.Errorf("kubernetes clientset: %w", err)
		}
		return provider.NewSecretTokenStore(
			clientset,
			cfg.authStoreK8sNamespace,
			cfg.authStoreK8sSecret,
			cfg.authStoreK8sKey,
		)
	default:
		return nil, fmt.Errorf("unsupported auth store backend %q", cfg.authStoreBackend)
	}
}

func newRuntimePodReader() (runtimePodReader, error) {
	client, err := kubeClient()
	if err != nil {
		return nil, err
	}
	return kubeRuntimePodReader{kube: client}, nil
}

func kubeClient() (*kubernetes.Clientset, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return kubernetes.NewForConfig(cfg)
	}
	kubeconfig := strings.TrimSpace(os.Getenv("KUBECONFIG"))
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			kubeconfig = home + "/.kube/config"
		}
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func loadConfig() (config, error) {
	endpoints := splitCSV(os.Getenv("SMITH_ETCD_ENDPOINTS"))
	if len(endpoints) == 0 {
		endpoints = []string{"http://127.0.0.1:2379"}
	}
	skillPolicy := model.DefaultSkillPolicy()
	if raw := splitCSV(os.Getenv("SMITH_SKILL_ALLOWED_SOURCES")); len(raw) > 0 {
		skillPolicy.AllowedSourcePrefixes = raw
	}
	skillPolicy.AllowWritable = envBool("SMITH_SKILL_ALLOW_WRITABLE", skillPolicy.AllowWritable)
	authStoreBackend := strings.ToLower(strings.TrimSpace(envString("SMITH_AUTH_STORE_BACKEND", "file")))
	authStoreK8sNamespace := strings.TrimSpace(envString("SMITH_AUTH_STORE_K8S_NAMESPACE", envString("POD_NAMESPACE", "default")))
	authStoreK8sSecret := strings.TrimSpace(envString("SMITH_AUTH_STORE_K8S_SECRET", "smith-auth-store"))
	authStoreK8sKey := strings.TrimSpace(envString("SMITH_AUTH_STORE_K8S_KEY", "tokens.json"))
	return config{
		port:                  envInt("SMITH_API_PORT", defaultPort),
		etcdEndpoints:         endpoints,
		etcdDialTimeout:       envDuration("SMITH_ETCD_DIAL_TIMEOUT", 5*time.Second),
		operatorToken:         strings.TrimSpace(os.Getenv("SMITH_OPERATOR_TOKEN")),
		authStoreBackend:      authStoreBackend,
		authStorePath:         envString("SMITH_AUTH_STORE_PATH", "/tmp/smith-auth/tokens.json"),
		authStoreK8sNamespace: authStoreK8sNamespace,
		authStoreK8sSecret:    authStoreK8sSecret,
		authStoreK8sKey:       authStoreK8sKey,
		defaultPreset:         strings.TrimSpace(os.Getenv("SMITH_DEFAULT_ENV_PRESET")),
		skillPolicy:           skillPolicy,
		runtimeNamespace:      strings.TrimSpace(envString("SMITH_RUNTIME_NAMESPACE", envString("SMITH_NAMESPACE", authStoreK8sNamespace))),
		runtimeContainerName:  strings.TrimSpace(envString("SMITH_RUNTIME_CONTAINER_NAME", "replica")),
	}, nil
}

func deriveLoopID(idempotencyKey, sourceType, sourceRef string) string {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		key = sourceType + ":" + sourceRef
	}
	key = strings.ToLower(strings.TrimSpace(key))
	replacer := strings.NewReplacer("/", "-", "_", "-", ".", "-", " ", "-", ":", "-")
	key = replacer.Replace(key)
	key = strings.Trim(key, "-")
	if key == "" {
		key = fmt.Sprintf("loop-%d", time.Now().UTC().UnixNano())
	}
	if len(key) > 48 {
		key = key[:48]
	}
	return "loop-" + key
}

func ingressSummary(results []ingressResult) map[string]any {
	created := 0
	existing := 0
	errorsCount := 0
	for _, res := range results {
		switch {
		case res.Status == "error":
			errorsCount++
		case res.Created:
			created++
		default:
			existing++
		}
	}
	return map[string]any{
		"results": results,
		"summary": map[string]int{
			"requested": len(results),
			"created":   created,
			"existing":  existing,
			"errors":    errorsCount,
		},
	}
}

func withIdempotency(metadata map[string]string, key string) map[string]string {
	out := map[string]string{}
	for k, v := range metadata {
		out[k] = v
	}
	if strings.TrimSpace(key) != "" {
		out["idempotency_key"] = strings.TrimSpace(key)
	}
	return out
}

func newPresetCatalog(defaultPreset string) *presetCatalog {
	policy := model.DefaultEnvironmentPolicy()
	presets := map[string]struct{}{}
	for name := range policy.AllowedPresets {
		presets[name] = struct{}{}
	}
	resolvedDefault := strings.ToLower(strings.TrimSpace(defaultPreset))
	if resolvedDefault == "" {
		resolvedDefault = policy.DefaultPreset
	}
	if _, ok := presets[resolvedDefault]; !ok {
		presets[resolvedDefault] = struct{}{}
	}
	return &presetCatalog{
		defaultPreset: resolvedDefault,
		presets:       presets,
	}
}

func (c *presetCatalog) Upsert(name string) error {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return errors.New("preset name is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.presets[normalized] = struct{}{}
	return nil
}

func (c *presetCatalog) Has(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.presets[normalized]
	return ok
}

func (c *presetCatalog) List() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, 0, len(c.presets))
	for name := range c.presets {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (c *presetCatalog) Default() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.defaultPreset
}

func (c *presetCatalog) Policy() model.EnvironmentPolicy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	allowed := map[string]struct{}{}
	for name := range c.presets {
		allowed[name] = struct{}{}
	}
	return model.EnvironmentPolicy{
		DefaultPreset:  c.defaultPreset,
		AllowedPresets: allowed,
	}
}

func copyStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func runtimeNamespaceForConfig(cfg config) string {
	namespace := strings.TrimSpace(cfg.runtimeNamespace)
	if namespace == "" {
		return "default"
	}
	return namespace
}

func runtimeContainerForConfig(cfg config) string {
	container := strings.TrimSpace(cfg.runtimeContainerName)
	if container == "" {
		return "replica"
	}
	return container
}

func envString(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func parseIntDefault(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

func parseInt64Default(raw string, fallback int64) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func parseFloatDefault(raw string, fallback float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return fallback
	}
	return v
}

func parseRFC3339(raw string) time.Time {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}

func formatRFC3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func maskCredentialValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return strings.Repeat("*", len(trimmed))
	}
	return trimmed[:4] + strings.Repeat("*", len(trimmed)-8) + trimmed[len(trimmed)-4:]
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func ioReadAllLimit(body io.Reader, max int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(body, max))
}

type auditBridge struct {
	store *store.Store
}

func (a *auditBridge) RecordAuthEvent(ctx context.Context, event provider.AuthEvent) error {
	if a == nil || a.store == nil {
		return nil
	}
	// Auth lifecycle calls should not block on audit availability.
	auditCtx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	return a.store.AppendAudit(auditCtx, store.AuditRecord{
		Actor:        event.Actor,
		Action:       "auth-" + event.Action,
		TargetLoopID: "",
		Metadata: map[string]string{
			"provider_id": event.ProviderID,
		},
	})
}
