package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"smith/internal/source/ingress"
	"smith/internal/source/model"
)

type ingressLoop struct {
	LoopID      string
	SourceType  string
	SourceRef   string
	State       string
	CreatedFrom string
}

type ingressHarness struct {
	mu       sync.Mutex
	loops    map[string]ingressLoop
	nextID   int
	attaches []string
}

func newIngressHarness() *ingressHarness {
	return &ingressHarness{
		loops:  make(map[string]ingressLoop),
		nextID: 1,
	}
}

func (h *ingressHarness) newLoop(sourceType, sourceRef, createdFrom string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := fmt.Sprintf("loop-ingress-%03d", h.nextID)
	h.nextID++
	h.loops[id] = ingressLoop{
		LoopID:      id,
		SourceType:  sourceType,
		SourceRef:   sourceRef,
		State:       "synced",
		CreatedFrom: createdFrom,
	}
	if sourceType == "interactive" {
		h.attaches = append(h.attaches, id)
	}
	return id
}

func (h *ingressHarness) getLoop(id string) (ingressLoop, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	loop, ok := h.loops[id]
	return loop, ok
}

func (h *ingressHarness) attachCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.attaches)
}

func TestIngressModesLoopCreationAndExecution(t *testing.T) {
	h := newIngressHarness()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/loops":
			h.handleLoopCreate(t, w, r)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/v1/ingress/github/issues":
			h.handleGitHubIngress(t, w, r)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/v1/ingress/prd":
			h.handlePRDIngress(t, w, r)
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/loops/"):
			h.handleLoopGet(w, r)
			return
		default:
			http.Error(w, "unexpected route", http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Run("github ingress", func(t *testing.T) {
		dir := t.TempDir()
		issuesPath := filepath.Join(dir, "issues.json")
		payload := map[string]any{
			"issues": []map[string]any{{
				"id":     "1001",
				"title":  "Fix loop drift",
				"repo":   "acme/smith",
				"number": 12,
				"url":    "https://github.com/acme/smith/issues/12",
			}},
		}
		writeJSONFile(t, issuesPath, payload)

		out := runSmithctl(t, server.URL, "--output", "json", "loop", "ingest-github", "--file", issuesPath)
		loopID := mustGetIngressLoopID(t, out)
		assertLoopGet(t, server.URL, loopID, "github_issue", "acme/smith#12", "synced")
	})

	t.Run("prd ingress", func(t *testing.T) {
		dir := t.TempDir()
		prdPath := filepath.Join(dir, "prd.md")
		markdown := strings.Join([]string{
			"# Sample PRD",
			"",
			"## Overview",
			"",
			"Validate ingress before loop creation.",
			"",
			"## Quality Gates",
			"- go test ./...",
			"",
			"## Stories",
			"",
			"### US-001: Ship ingress e2e",
			"",
			"As an operator, I want validated PRD ingress.",
			"",
			"#### Acceptance Criteria",
			"- Ingress creates a loop draft.",
		}, "\n")
		if err := os.WriteFile(prdPath, []byte(markdown), 0o600); err != nil {
			t.Fatalf("write prd: %v", err)
		}

		out := runSmithctl(t, server.URL, "--output", "json", "prd", "submit", "--file", prdPath, "--source-ref", "docs/prd-ingress.md")
		loopID := mustGetIngressLoopID(t, out)
		assertLoopGet(t, server.URL, loopID, "prd_story", "docs/prd-ingress.md#US-001", "synced")
	})

	t.Run("interactive ingress", func(t *testing.T) {
		out := runSmithctl(t, server.URL, "--output", "json", "loop", "create",
			"--title", "Interactive triage",
			"--description", "Operator-driven loop",
			"--source-type", "interactive",
			"--source-ref", "terminal/session-01",
			"--idempotency-key", "interactive-session-01",
		)
		loopID := mustGetTopLevelLoopID(t, out)
		assertLoopGet(t, server.URL, loopID, "interactive", "terminal/session-01", "synced")
		if h.attachCount() != 1 {
			t.Fatalf("expected interactive attach tracking count 1, got %d", h.attachCount())
		}
	})

	t.Run("prd validation parity", func(t *testing.T) {
		dir := t.TempDir()
		prdPath := filepath.Join(dir, "invalid-prd.json")
		content := `{
			"version": 1,
			"project": "Validation",
			"overview": "Canonical PRD validation",
			"qualityGates": [],
			"stories": [
				{
					"id": "US-001",
					"title": "Oversized story",
					"status": "open",
					"description": "As an operator, I want a story that packs too much work into one iteration.",
					"acceptanceCriteria": ["one","two","three","four","five","six"]
				}
			]
		}`
		if err := os.WriteFile(prdPath, []byte(content), 0o600); err != nil {
			t.Fatalf("write prd: %v", err)
		}

		validateOut := runSmithValidate(t, prdPath)
		submitOut, stderr, code := runSmithctlWithExitCode(server.URL, "--output", "json", "prd", "submit", "--file", prdPath)
		if code != 1 {
			t.Fatalf("expected smithctl submit to fail, got code=%d stderr=%s stdout=%s", code, stderr, string(submitOut))
		}

		var cliReport map[string]any
		if err := json.Unmarshal(validateOut, &cliReport); err != nil {
			t.Fatalf("decode smith validate output: %v\n%s", err, string(validateOut))
		}
		var apiBody map[string]any
		if err := json.Unmarshal(submitOut, &apiBody); err != nil {
			t.Fatalf("decode smithctl submit output: %v\n%s", err, string(submitOut))
		}
		report, ok := apiBody["report"]
		if !ok {
			t.Fatalf("expected API rejection report in %s", string(submitOut))
		}
		if !reportsEqual(cliReport, report) {
			t.Fatalf("expected CLI validation and API rejection parity\ncli=%s\napi=%s", string(validateOut), string(submitOut))
		}
	})
}

func (h *ingressHarness) handleLoopCreate(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	sourceType, _ := req["source_type"].(string)
	sourceRef, _ := req["source_ref"].(string)
	if strings.TrimSpace(sourceType) == "" || strings.TrimSpace(sourceRef) == "" {
		http.Error(w, "source fields required", http.StatusBadRequest)
		return
	}
	id := h.newLoop(sourceType, sourceRef, "interactive")
	writeJSONResponse(t, w, http.StatusCreated, map[string]any{
		"loop_id":   id,
		"status":    "synced",
		"created":   true,
		"message":   "loop created",
		"ingress":   "interactive",
		"trace_ref": sourceRef,
	})
}

func (h *ingressHarness) handleGitHubIngress(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var req struct {
		Issues []struct {
			Repo   string `json:"repo"`
			Number int    `json:"number"`
		} `json:"issues"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if len(req.Issues) == 0 {
		http.Error(w, "issues required", http.StatusBadRequest)
		return
	}
	results := make([]map[string]any, 0, len(req.Issues))
	for i, issue := range req.Issues {
		sourceRef := fmt.Sprintf("%s#%d", strings.TrimSpace(issue.Repo), issue.Number)
		id := h.newLoop("github_issue", sourceRef, "github")
		results = append(results, map[string]any{
			"item_index": i,
			"loop_id":    id,
			"source_ref": sourceRef,
			"status":     "synced",
			"created":    true,
		})
	}
	writeJSONResponse(t, w, http.StatusOK, map[string]any{"results": results})
}

func (h *ingressHarness) handlePRDIngress(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var req struct {
		Format    string          `json:"format"`
		SourceRef string          `json:"source_ref"`
		Markdown  string          `json:"markdown"`
		PRD       json.RawMessage `json:"prd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	sourceRef := strings.TrimSpace(req.SourceRef)
	if sourceRef == "" {
		sourceRef = "prd:canonical"
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		if strings.TrimSpace(req.Markdown) != "" {
			format = "markdown"
		} else {
			format = "json"
		}
	}

	var (
		drafts []ingress.LoopDraft
		report model.PRDValidationReport
	)
	switch format {
	case "markdown", "md":
		prd, validation := model.ValidatePRDMarkdown([]byte(req.Markdown))
		report = validation
		if report.Valid {
			drafts = ingress.CanonicalPRDToDrafts(prd, sourceRef, nil)
		}
	case "json":
		prd, validation := model.ValidatePRDJSON(req.PRD)
		report = validation
		if report.Valid {
			drafts = ingress.CanonicalPRDToDrafts(prd, sourceRef, nil)
		}
	default:
		http.Error(w, "format must be markdown or json", http.StatusBadRequest)
		return
	}
	if !report.Valid {
		writeJSONResponse(t, w, http.StatusUnprocessableEntity, map[string]any{
			"error":  "prd failed readiness validation",
			"report": report,
		})
		return
	}
	results := make([]map[string]any, 0, len(drafts))
	for i, draft := range drafts {
		id := h.newLoop(draft.SourceType, draft.SourceRef, "prd")
		results = append(results, map[string]any{
			"item_index": i,
			"loop_id":    id,
			"source_ref": draft.SourceRef,
			"status":     "synced",
			"created":    true,
		})
	}
	writeJSONResponse(t, w, http.StatusOK, map[string]any{"results": results})
}

func (h *ingressHarness) handleLoopGet(w http.ResponseWriter, r *http.Request) {
	loopID := strings.TrimPrefix(r.URL.Path, "/v1/loops/")
	loop, ok := h.getLoop(loopID)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"state": map[string]any{
			"loop_id": loopID,
			"state":   loop.State,
		},
		"anomaly": map[string]any{
			"id":          loopID,
			"source_type": loop.SourceType,
			"source_ref":  loop.SourceRef,
		},
	})
}

func assertLoopGet(t *testing.T, serverURL, loopID, wantSourceType, wantSourceRef, wantState string) {
	t.Helper()
	out := runSmithctl(t, serverURL, "--output", "json", "loop", "get", loopID)
	var body map[string]any
	if err := json.Unmarshal(out, &body); err != nil {
		t.Fatalf("decode loop get response: %v\n%s", err, string(out))
	}
	state := asMap(t, body["state"], "state")
	anomaly := asMap(t, body["anomaly"], "anomaly")

	if got, _ := state["state"].(string); got != wantState {
		t.Fatalf("state mismatch: got=%q want=%q", got, wantState)
	}
	if got, _ := anomaly["source_type"].(string); got != wantSourceType {
		t.Fatalf("source_type mismatch: got=%q want=%q", got, wantSourceType)
	}
	if got, _ := anomaly["source_ref"].(string); got != wantSourceRef {
		t.Fatalf("source_ref mismatch: got=%q want=%q", got, wantSourceRef)
	}
}

func mustGetIngressLoopID(t *testing.T, output []byte) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(output, &body); err != nil {
		t.Fatalf("decode ingress response: %v\n%s", err, string(output))
	}
	resultsRaw, ok := body["results"].([]any)
	if !ok || len(resultsRaw) == 0 {
		t.Fatalf("expected non-empty results in ingress response: %s", string(output))
	}
	first := asMap(t, resultsRaw[0], "results[0]")
	id, _ := first["loop_id"].(string)
	if strings.TrimSpace(id) == "" {
		t.Fatalf("missing loop_id in ingress response: %s", string(output))
	}
	return id
}

func mustGetTopLevelLoopID(t *testing.T, output []byte) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(output, &body); err != nil {
		t.Fatalf("decode create response: %v\n%s", err, string(output))
	}
	id, _ := body["loop_id"].(string)
	if strings.TrimSpace(id) == "" {
		t.Fatalf("missing loop_id in create response: %s", string(output))
	}
	return id
}

func asMap(t *testing.T, value any, field string) map[string]any {
	t.Helper()
	m, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object", field)
	}
	return m
}

func writeJSONFile(t *testing.T, path string, payload map[string]any) {
	t.Helper()
	content, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write json file: %v", err)
	}
}

func writeJSONResponse(t *testing.T, w http.ResponseWriter, status int, payload map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func runSmithctl(t *testing.T, serverURL string, args ...string) []byte {
	t.Helper()
	stdout, stderr, code := runSmithctlWithExitCode(serverURL, args...)
	if code != 0 {
		fullArgs := append([]string{"run", "./cmd/smithctl", "--server", serverURL}, args...)
		t.Fatalf("smithctl failed: code=%d\nargs: %v\nstderr:\n%s\nstdout:\n%s", code, fullArgs, stderr, string(stdout))
	}
	return stdout
}

func runSmithctlWithExitCode(serverURL string, args ...string) ([]byte, string, int) {
	fullArgs := append([]string{"run", "./cmd/smithctl", "--server", serverURL}, args...)
	cmd := exec.Command("go", fullArgs...)
	cmd.Dir = filepath.Clean(filepath.Join(filepath.Dir(mustCallerFile()), "../../.."))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = 1
		}
	}
	return stdout.Bytes(), stderr.String(), code
}

func runSmithValidate(t *testing.T, path string) []byte {
	t.Helper()
	cmd := exec.Command("go", "run", "./cmd/smith", "--prd", "validate", path)
	cmd.Dir = filepath.Clean(filepath.Join(filepath.Dir(mustCallerFile()), "../../.."))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
			t.Fatalf("smith validate failed unexpectedly: %v\nstderr:\n%s\nstdout:\n%s", err, stderr.String(), stdout.String())
		}
	}
	return stdout.Bytes()
}

func reportsEqual(left any, right any) bool {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return bytes.Equal(leftJSON, rightJSON)
}

func mustCallerFile() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("unable to resolve caller")
	}
	return file
}
