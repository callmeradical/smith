package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type ingressLoopRecord struct {
	LoopID     string
	State      string
	SourceType string
	SourceRef  string
	Metadata   map[string]string
}

type ingressMockServer struct {
	mu          sync.Mutex
	loops       map[string]ingressLoopRecord
	attachCalls map[string]int
	nextID      int
}

func newIngressMockServer() *ingressMockServer {
	return &ingressMockServer{
		loops:       map[string]ingressLoopRecord{},
		attachCalls: map[string]int{},
		nextID:      1,
	}
}

func (m *ingressMockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/ingress/github/issues":
		m.handleGitHubIngress(w, r)
		return
	case r.Method == http.MethodPost && r.URL.Path == "/v1/ingress/prd":
		m.handlePRDIngress(w, r)
		return
	case r.Method == http.MethodPost && r.URL.Path == "/v1/loops":
		m.handleLoopCreate(w, r)
		return
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/loops/"):
		m.handleLoopGet(w, r)
		return
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/control/attach"):
		m.handleAttach(w, r)
		return
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (m *ingressMockServer) handleGitHubIngress(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	issues, _ := payload["issues"].([]any)

	results := make([]map[string]any, 0, len(issues))
	for idx, item := range issues {
		issue, _ := item.(map[string]any)
		repo, _ := issue["repository"].(string)
		number := int(asFloat(issue["number"]))
		sourceRef := fmt.Sprintf("%s#%d", strings.TrimSpace(repo), number)
		loopID := fmt.Sprintf("loop-gh-%d", number)
		if number <= 0 {
			loopID = fmt.Sprintf("loop-gh-%d", idx+1)
		}

		m.mu.Lock()
		m.loops[loopID] = ingressLoopRecord{
			LoopID:     loopID,
			State:      "synced",
			SourceType: "github_issue",
			SourceRef:  sourceRef,
			Metadata: map[string]string{
				"ingress_mode": "github_issue",
			},
		}
		m.mu.Unlock()

		results = append(results, map[string]any{
			"item_index": idx,
			"loop_id":    loopID,
			"source_ref": sourceRef,
			"status":     "synced",
			"created":    true,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"summary": map[string]any{
			"requested": len(results),
			"created":   len(results),
			"existing":  0,
			"errors":    0,
		},
	})
}

func (m *ingressMockServer) handlePRDIngress(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	sourceRoot, _ := payload["source_ref"].(string)
	sourceRoot = strings.TrimSpace(sourceRoot)
	if sourceRoot == "" {
		sourceRoot = "prd:adhoc"
	}

	m.mu.Lock()
	loopID := fmt.Sprintf("loop-prd-%d", m.nextID)
	m.nextID++
	sourceRef := sourceRoot + "#general-task-1"
	m.loops[loopID] = ingressLoopRecord{
		LoopID:     loopID,
		State:      "synced",
		SourceType: "prd_task",
		SourceRef:  sourceRef,
		Metadata: map[string]string{
			"ingress_mode": "prd",
			"prd_source":   sourceRoot,
		},
	}
	m.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"results": []map[string]any{
			{
				"item_index": 0,
				"loop_id":    loopID,
				"source_ref": sourceRef,
				"status":     "synced",
				"created":    true,
			},
		},
		"summary": map[string]any{
			"requested": 1,
			"created":   1,
			"existing":  0,
			"errors":    0,
		},
	})
}

func (m *ingressMockServer) handleLoopCreate(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	sourceType, _ := payload["source_type"].(string)
	sourceRef, _ := payload["source_ref"].(string)

	m.mu.Lock()
	loopID := fmt.Sprintf("loop-interactive-%d", m.nextID)
	m.nextID++
	m.loops[loopID] = ingressLoopRecord{
		LoopID:     loopID,
		State:      "synced",
		SourceType: strings.TrimSpace(sourceType),
		SourceRef:  strings.TrimSpace(sourceRef),
		Metadata: map[string]string{
			"ingress_mode": "interactive",
		},
	}
	m.mu.Unlock()

	writeJSON(w, http.StatusCreated, map[string]any{
		"loop_id":  loopID,
		"status":   "synced",
		"created":  true,
		"message":  "loop created",
		"httpCode": http.StatusCreated,
	})
}

func (m *ingressMockServer) handleLoopGet(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/loops/"), "/")
	if len(parts) != 1 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	loopID := parts[0]

	m.mu.Lock()
	record, ok := m.loops[loopID]
	m.mu.Unlock()
	if !ok {
		http.Error(w, "loop not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"state": map[string]any{
			"loop_id": loopID,
			"state":   record.State,
		},
		"anomaly": map[string]any{
			"loop_id":     loopID,
			"source_type": record.SourceType,
			"source_ref":  record.SourceRef,
			"metadata":    record.Metadata,
		},
	})
}

func (m *ingressMockServer) handleAttach(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/loops/"), "/")
	if len(parts) != 3 || parts[1] != "control" || parts[2] != "attach" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	loopID := parts[0]

	m.mu.Lock()
	_, ok := m.loops[loopID]
	if ok {
		m.attachCalls[loopID]++
	}
	m.mu.Unlock()
	if !ok {
		http.Error(w, "loop not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"loop_id":    loopID,
		"status":     "attached",
		"session_id": "session-" + loopID,
	})
}

func TestIngressGitHubModeE2E(t *testing.T) {
	mock := newIngressMockServer()
	srv := httptest.NewServer(mock)
	defer srv.Close()

	dir := t.TempDir()
	payloadPath := filepath.Join(dir, "issues.json")
	require.NoError(t, os.WriteFile(payloadPath, []byte(`[
	{"repository":"org/repo","number":123,"title":"Fix flaky lock"}
]`), 0o600))

	stdout, stderr, err := runSmithctl(t,
		"--server", srv.URL, "--output", "json",
		"loop", "create", "--from-github", payloadPath,
	)
	require.NoError(t, err, "stderr=%s", stderr)
	var createOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &createOut), "stdout=%s", stdout)

	results := mustSlice(t, createOut["results"])
	require.Len(t, results, 1)
	loopID, _ := mustMap(t, results[0])["loop_id"].(string)
	require.NotEmpty(t, loopID)

	stdout, stderr, err = runSmithctl(t,
		"--server", srv.URL, "--output", "json",
		"loop", "get", loopID,
	)
	require.NoError(t, err, "stderr=%s", stderr)
	var getOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &getOut), "stdout=%s", stdout)

	state := mustMap(t, getOut["state"])
	require.Equal(t, "synced", state["state"])
	anomaly := mustMap(t, getOut["anomaly"])
	require.Equal(t, "github_issue", anomaly["source_type"])
	require.Equal(t, "org/repo#123", anomaly["source_ref"])
}

func TestIngressPRDModeE2E(t *testing.T) {
	mock := newIngressMockServer()
	srv := httptest.NewServer(mock)
	defer srv.Close()

	dir := t.TempDir()
	prdPath := filepath.Join(dir, "prd.md")
	require.NoError(t, os.WriteFile(prdPath, []byte("# PRD\n\n## Tasks\n- [ ] Build ingress flow"), 0o600))

	stdout, stderr, err := runSmithctl(t,
		"--server", srv.URL, "--output", "json",
		"loop", "create", "--from-prd", prdPath, "--source-ref", "prd:docs/prd-ingress.md",
	)
	require.NoError(t, err, "stderr=%s", stderr)
	var createOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &createOut), "stdout=%s", stdout)

	results := mustSlice(t, createOut["results"])
	require.Len(t, results, 1)
	loopID, _ := mustMap(t, results[0])["loop_id"].(string)
	require.NotEmpty(t, loopID)

	stdout, stderr, err = runSmithctl(t,
		"--server", srv.URL, "--output", "json",
		"loop", "get", loopID,
	)
	require.NoError(t, err, "stderr=%s", stderr)
	var getOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &getOut), "stdout=%s", stdout)

	state := mustMap(t, getOut["state"])
	require.Equal(t, "synced", state["state"])
	anomaly := mustMap(t, getOut["anomaly"])
	require.Equal(t, "prd_task", anomaly["source_type"])
	sourceRef, _ := anomaly["source_ref"].(string)
	require.True(t, strings.HasPrefix(sourceRef, "prd:docs/prd-ingress.md#"), "source_ref=%s", sourceRef)
}

func TestIngressInteractiveAttachModeE2E(t *testing.T) {
	mock := newIngressMockServer()
	srv := httptest.NewServer(mock)
	defer srv.Close()

	stdout, stderr, err := runSmithctl(t,
		"--server", srv.URL, "--output", "json",
		"loop", "create",
		"--title", "Interactive diagnostics",
		"--description", "attach to live terminal",
		"--source-type", "interactive_terminal",
		"--source-ref", "terminal:operator-session",
		"--idempotency-key", "interactive-1",
	)
	require.NoError(t, err, "stderr=%s", stderr)
	var createOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &createOut), "stdout=%s", stdout)
	loopID, _ := createOut["loop_id"].(string)
	require.NotEmpty(t, loopID)

	stdout, stderr, err = runSmithctl(t,
		"--server", srv.URL, "--output", "json",
		"loop", "attach", "--follow=false", loopID,
	)
	require.NoError(t, err, "stderr=%s", stderr)
	var attachOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &attachOut), "stdout=%s", stdout)
	results := mustSlice(t, attachOut["results"])
	require.Len(t, results, 1)
	result := mustMap(t, results[0])
	require.Equal(t, "attached", result["status"])

	mock.mu.Lock()
	require.Equal(t, 1, mock.attachCalls[loopID])
	mock.mu.Unlock()

	stdout, stderr, err = runSmithctl(t,
		"--server", srv.URL, "--output", "json",
		"loop", "get", loopID,
	)
	require.NoError(t, err, "stderr=%s", stderr)
	var getOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &getOut), "stdout=%s", stdout)

	state := mustMap(t, getOut["state"])
	require.Equal(t, "synced", state["state"])
	anomaly := mustMap(t, getOut["anomaly"])
	require.Equal(t, "interactive_terminal", anomaly["source_type"])
	require.Equal(t, "terminal:operator-session", anomaly["source_ref"])
}

func runSmithctl(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	repoRoot := rootDirFromTestFile(t)
	cmdArgs := append([]string{"run", "./cmd/smithctl"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", string(out), err
	}
	return string(out), "", nil
}

func rootDirFromTestFile(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "failed to resolve caller path")
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func asFloat(v any) float64 {
	switch typed := v.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case json.Number:
		num, _ := strconv.ParseFloat(typed.String(), 64)
		return num
	default:
		return 0
	}
}

func mustMap(t *testing.T, value any) map[string]any {
	t.Helper()
	out, ok := value.(map[string]any)
	require.True(t, ok, "expected map, got %T", value)
	return out
}

func mustSlice(t *testing.T, value any) []any {
	t.Helper()
	out, ok := value.([]any)
	require.True(t, ok, "expected slice, got %T", value)
	return out
}
