package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type envHarness struct {
	mu     sync.Mutex
	nextID int
	loops  map[string]map[string]any
}

func newEnvHarness() *envHarness {
	return &envHarness{
		nextID: 1,
		loops:  map[string]map[string]any{},
	}
}

func TestLoopEnvironmentModes(t *testing.T) {
	h := newEnvHarness()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/loops":
			h.handleLoopCreate(t, w, r)
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/loops/"):
			h.handleLoopGet(w, r)
			return
		default:
			http.Error(w, "unexpected route", http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Run("preset default", func(t *testing.T) {
		out := runSmithctl(t, server.URL, "--output", "json", "loop", "create",
			"--title", "Default env",
			"--description", "No env flags",
			"--source-type", "interactive",
			"--source-ref", "terminal/default-env",
		)
		loopID := mustGetTopLevelLoopID(t, out)
		assertEnvironmentMode(t, server.URL, loopID, "preset")
	})

	t.Run("mise mode", func(t *testing.T) {
		out := runSmithctl(t, server.URL, "--output", "json", "loop", "create",
			"--title", "Mise env",
			"--description", "Toolchain pin",
			"--source-type", "interactive",
			"--source-ref", "terminal/mise-env",
			"--env-mise-file", ".tool-versions",
			"--env-tool", "go=1.22.0",
		)
		loopID := mustGetTopLevelLoopID(t, out)
		assertEnvironmentMode(t, server.URL, loopID, "mise")
	})

	t.Run("container image mode", func(t *testing.T) {
		out := runSmithctl(t, server.URL, "--output", "json", "loop", "create",
			"--title", "Image env",
			"--description", "Image override",
			"--source-type", "interactive",
			"--source-ref", "terminal/image-env",
			"--env-image-ref", "ghcr.io/acme/replica:v2",
			"--env-image-pull-policy", "Always",
		)
		loopID := mustGetTopLevelLoopID(t, out)
		assertEnvironmentMode(t, server.URL, loopID, "container_image")
	})

	t.Run("dockerfile mode", func(t *testing.T) {
		out := runSmithctl(t, server.URL, "--output", "json", "loop", "create",
			"--title", "Dockerfile env",
			"--description", "Build override",
			"--source-type", "interactive",
			"--source-ref", "terminal/docker-env",
			"--env-docker-context", ".",
			"--env-dockerfile", "Dockerfile",
			"--env-docker-target", "runtime",
			"--env-build-arg", "GO_VERSION=1.22",
		)
		loopID := mustGetTopLevelLoopID(t, out)
		assertEnvironmentMode(t, server.URL, loopID, "dockerfile")
	})
}

func (h *envHarness) handleLoopCreate(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	environment, _ := req["environment"].(map[string]any)
	if environment == nil {
		environment = map[string]any{}
	}
	if _, ok := environment["preset"]; !ok {
		environment["preset"] = "standard"
	}
	mode := "preset"
	if _, ok := environment["mise"]; ok {
		mode = "mise"
	}
	if _, ok := environment["container_image"]; ok {
		mode = "container_image"
	}
	if _, ok := environment["dockerfile"]; ok {
		mode = "dockerfile"
	}
	environment["resolved_mode"] = mode

	h.mu.Lock()
	id := fmt.Sprintf("loop-env-%03d", h.nextID)
	h.nextID++
	h.loops[id] = environment
	h.mu.Unlock()

	_ = json.NewEncoder(w).Encode(map[string]any{
		"loop_id":     id,
		"status":      "synced",
		"created":     true,
		"environment": environment,
		"http_code":   201,
		"message":     "loop created",
	})
}

func (h *envHarness) handleLoopGet(w http.ResponseWriter, r *http.Request) {
	loopID := strings.TrimPrefix(r.URL.Path, "/v1/loops/")
	h.mu.Lock()
	environment, ok := h.loops[loopID]
	h.mu.Unlock()
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"state": map[string]any{
			"loop_id": loopID,
			"state":   "synced",
		},
		"environment": environment,
	})
}

func assertEnvironmentMode(t *testing.T, serverURL, loopID, wantMode string) {
	t.Helper()
	out := runSmithctl(t, serverURL, "--output", "json", "loop", "get", loopID)
	var body map[string]any
	if err := json.Unmarshal(out, &body); err != nil {
		t.Fatalf("decode loop get response: %v\n%s", err, string(out))
	}
	environment := asMap(t, body["environment"], "environment")
	if got, _ := environment["resolved_mode"].(string); got != wantMode {
		t.Fatalf("resolved_mode mismatch: got=%q want=%q", got, wantMode)
	}
}
