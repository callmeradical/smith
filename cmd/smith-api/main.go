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
	"strconv"
	"strings"
	"syscall"
	"time"

	"smith/internal/source/ingress"
	"smith/internal/source/model"
	"smith/internal/source/provider"
	"smith/internal/source/store"
)

const (
	defaultPort            = 8080
	defaultShutdownTimeout = 10 * time.Second
)

type config struct {
	port            int
	etcdEndpoints   []string
	etcdDialTimeout time.Duration
	operatorToken   string
	authStorePath   string
}

type server struct {
	cfg   config
	store *store.Store
	auth  *provider.AuthManager
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

	authManager := provider.NewAuthManager(
		provider.ProviderCodex,
		provider.NewFileTokenStore(cfg.authStorePath),
		provider.NewMockDeviceAuthClient(),
		&auditBridge{store: es},
	)

	s := &server{cfg: cfg, store: es, auth: authManager}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/readyz", s.handleReady)
	mux.HandleFunc("/v1/loops", s.handleLoops)
	mux.HandleFunc("/v1/loops/", s.handleLoopByID)
	mux.HandleFunc("/v1/ingress/github/issues", s.handleIngressGitHubIssues)
	mux.HandleFunc("/v1/ingress/prd", s.handleIngressPRD)
	mux.HandleFunc("/v1/control/override", s.handleOverride)
	mux.HandleFunc("/v1/reporting/cost", s.handleCost)
	mux.HandleFunc("/v1/auth/codex/connect/start", s.handleCodexAuthStart)
	mux.HandleFunc("/v1/auth/codex/connect/complete", s.handleCodexAuthComplete)
	mux.HandleFunc("/v1/auth/codex/status", s.handleCodexAuthStatus)
	mux.HandleFunc("/v1/auth/codex/disconnect", s.handleCodexAuthDisconnect)

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
	environment, err := model.NormalizeLoopEnvironment(req.Environment)
	if err != nil {
		return loopCreateResult{Status: "error", Message: err.Error(), HTTPCode: http.StatusBadRequest}
	}
	skills, err := model.NormalizeLoopSkills(req.Skills, selection.ProviderID)
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
			"source_type":       req.SourceType,
			"source_ref":        req.SourceRef,
			"environment_mode":  environment.ResolvedMode,
			"skill_mount_count": strconv.Itoa(len(skills)),
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

func (s *server) handleLoopByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/loops/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeErr(w, http.StatusBadRequest, "loop id is required")
		return
	}
	loopID := parts[0]

	if len(parts) == 1 {
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
	}

	if len(parts) == 2 && parts[1] == "journal" {
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

	writeErr(w, http.StatusNotFound, "endpoint not found")
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
		"connected":  true,
		"expires_at": token.ExpiresAt.UTC().Format(time.RFC3339),
		"account_id": token.AccountID,
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
	connected, expiresAt, err := s.auth.Status(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connected":  connected,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
		"provider":   provider.ProviderCodex,
	})
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
	if err := s.auth.Disconnect(r.Context(), actor); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connected": false})
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

func loadConfig() (config, error) {
	endpoints := splitCSV(os.Getenv("SMITH_ETCD_ENDPOINTS"))
	if len(endpoints) == 0 {
		endpoints = []string{"http://127.0.0.1:2379"}
	}
	return config{
		port:            envInt("SMITH_API_PORT", defaultPort),
		etcdEndpoints:   endpoints,
		etcdDialTimeout: envDuration("SMITH_ETCD_DIAL_TIMEOUT", 5*time.Second),
		operatorToken:   strings.TrimSpace(os.Getenv("SMITH_OPERATOR_TOKEN")),
		authStorePath:   envString("SMITH_AUTH_STORE_PATH", "/tmp/smith-auth/tokens.json"),
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

func copyStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
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
	return a.store.AppendAudit(ctx, store.AuditRecord{
		Actor:        event.Actor,
		Action:       "auth-" + event.Action,
		TargetLoopID: "",
		Metadata: map[string]string{
			"provider_id": event.ProviderID,
		},
	})
}
