package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"smith/internal/source/model"
)

func TestResolveConfigFromFileAndOverrides(t *testing.T) {
	t.Setenv("SMITH_API_URL", "")
	t.Setenv("SMITH_OPERATOR_TOKEN", "")
	t.Setenv("SMITH_CONTEXT", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"dev","contexts":{"dev":{"server":"http://dev.local:8080","token":"dev-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resolved, err := resolveConfig(rootFlags{Config: cfgPath, Output: "json"})
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if resolved.Server != "http://dev.local:8080" {
		t.Fatalf("unexpected server: %s", resolved.Server)
	}
	if resolved.Token != "dev-token" {
		t.Fatalf("unexpected token: %s", resolved.Token)
	}
}

func TestResolveConfigEnvAndFlagPrecedence(t *testing.T) {
	t.Setenv("SMITH_API_URL", "http://env.local:8080")
	t.Setenv("SMITH_OPERATOR_TOKEN", "env-token")

	resolved, err := resolveConfig(rootFlags{Server: "http://flag.local:8080", Token: "flag-token", Output: "text"})
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if resolved.Server != "http://flag.local:8080" {
		t.Fatalf("unexpected server: %s", resolved.Server)
	}
	if resolved.Token != "flag-token" {
		t.Fatalf("unexpected token: %s", resolved.Token)
	}
}

func TestLoopCreateBatchArrayFile(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/loops" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "loops.json")
	content := `[{"title":"A","source_type":"github_issue","source_ref":"org/repo#1"}]`
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--server", srv.URL, "--output", "json", "loop", "create", "--batch", filePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if _, ok := received["loops"]; !ok {
		t.Fatalf("expected loops wrapper payload, got %#v", received)
	}
}

func TestLoopCreateWithEnvironmentImageFlags(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/loops" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "create",
		"--title", "Env", "--description", "Test", "--source-type", "interactive", "--source-ref", "terminal/session-01",
		"--env-image-ref", "ghcr.io/acme/replica:v2", "--env-image-pull-policy", "Always",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	rawEnv, ok := received["environment"].(map[string]any)
	if !ok {
		t.Fatalf("expected environment payload, got %#v", received)
	}
	rawImage, ok := rawEnv["container_image"].(map[string]any)
	if !ok {
		t.Fatalf("expected container_image payload, got %#v", rawEnv)
	}
	if rawImage["ref"] != "ghcr.io/acme/replica:v2" || rawImage["pull_policy"] != "Always" {
		t.Fatalf("unexpected container image payload: %#v", rawImage)
	}
}

func TestLoopCreateWithSkillFlags(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/loops" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "create",
		"--title", "Skill", "--description", "Test", "--source-type", "interactive", "--source-ref", "terminal/session-01",
		"--skill", "name=commit,source=local://skills/commit",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	skillsRaw, ok := received["skills"].([]any)
	if !ok || len(skillsRaw) != 1 {
		t.Fatalf("expected one skill in payload, got %#v", received["skills"])
	}
	skill, ok := skillsRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("expected skill map payload, got %#v", skillsRaw[0])
	}
	if skill["name"] != "commit" || skill["source"] != "local://skills/commit" {
		t.Fatalf("unexpected skill payload: %#v", skill)
	}
	if _, exists := skill["mount_path"]; exists {
		t.Fatalf("expected mount_path to be omitted when not provided: %#v", skill)
	}
}

func TestLoopCreateWithWorkspacePRDFile(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/loops" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	dir := t.TempDir()
	prdPath := filepath.Join(dir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(`{"stories":[{"id":"US-001","status":"open"}]}`), 0o600); err != nil {
		t.Fatalf("write prd file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "create",
		"--workspace-prd-file", prdPath, "--workspace-prompt", "refine PRD then build",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if received["source_type"] != "prompt" {
		t.Fatalf("expected source_type=prompt, got %#v", received["source_type"])
	}
	metadata, ok := received["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata map, got %#v", received["metadata"])
	}
	if metadata["workspace_prd_json"] == "" {
		t.Fatalf("expected workspace_prd_json to be set, got %#v", metadata)
	}
	if metadata["workspace_prd_path"] != ".agents/tasks/prd.json" {
		t.Fatalf("unexpected workspace_prd_path: %#v", metadata["workspace_prd_path"])
	}
}

func TestLoopCreateWithWorkspacePRDValidationErrorPrintsJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "prd failed readiness validation",
			"report": map[string]any{
				"valid":     false,
				"readiness": "fail",
				"errors": []map[string]any{{
					"code":       "prd_missing_quality_gates",
					"path":       "$.qualityGates",
					"message":    "at least one quality gate is required",
					"suggestion": "Add the commands required to verify PRD work.",
				}},
			},
		})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "create",
		"--workspace-prd-json", `{"version":1,"project":"Validation","overview":"Canonical PRD validation","qualityGates":[],"stories":[]}`,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code=1, got %d stderr=%s", code, stderr.String())
	}
	var body map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &body); err != nil {
		t.Fatalf("expected json validation output, got %v stdout=%s", err, stdout.String())
	}
	if body["error"] != "prd failed readiness validation" {
		t.Fatalf("unexpected output: %#v", body)
	}
}

func TestLoopCreateWithProviderID(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/loops" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "create",
		"--title", "Provider", "--description", "Test", "--source-type", "prompt", "--source-ref", "prompt:provider-test",
		"--provider-id", "CoDeX",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if got, _ := received["provider_id"].(string); got != "codex" {
		t.Fatalf("expected provider_id codex, got %#v", received["provider_id"])
	}
}

func TestLoopCreateRejectsInvalidWorkspacePRDJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--output", "json", "loop", "create",
		"--workspace-prd-json", `{"stories":`,
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code=2 for invalid json, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "must be valid json") {
		t.Fatalf("expected invalid json error, got stderr=%s", stderr.String())
	}
}

func TestPRDSubmitJSONSendsCanonicalPRD(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/ingress/prd" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer srv.Close()

	dir := t.TempDir()
	prdPath := filepath.Join(dir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(`{"version":1,"project":"Validation","overview":"Canonical PRD validation","qualityGates":["go test ./..."],"stories":[]}`), 0o600); err != nil {
		t.Fatalf("write prd file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--server", srv.URL, "--output", "json", "prd", "submit", "--file", prdPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if received["prd"] == nil {
		t.Fatalf("expected canonical prd payload, got %#v", received)
	}
	if received["tasks"] != nil {
		t.Fatalf("did not expect legacy tasks payload, got %#v", received)
	}
}

func TestLoopCreateEnvironmentSourceConflict(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--output", "json", "loop", "create",
		"--title", "Env", "--description", "Test", "--source-type", "interactive", "--source-ref", "terminal/session-01",
		"--env-image-ref", "ghcr.io/acme/replica:v2",
		"--env-docker-context", ".", "--env-dockerfile", "Dockerfile",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code=2 for source conflict, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "environment source conflict") {
		t.Fatalf("expected conflict error, got stderr=%s", stderr.String())
	}
}

func TestLoopGetBatchIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/loops/") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		id := strings.TrimPrefix(r.URL.Path, "/v1/loops/")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"state": map[string]any{
				"loop_id": id,
				"state":   "unresolved",
			},
		})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--server", srv.URL, "--output", "json", "loop", "get", "loop-a", "loop-b"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	results, ok := out["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("unexpected results: %#v", out)
	}
}

func TestLoopTraceBatchIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/loops/") || !strings.HasSuffix(r.URL.Path, "/trace") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/loops/"), "/trace")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"loop_id": id,
			"state": map[string]any{
				"loop_id": id,
				"state":   "synced",
			},
			"journal": []any{},
		})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--server", srv.URL, "--output", "json", "loop", "trace", "loop-a", "loop-b"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	results, ok := out["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("unexpected results: %#v", out)
	}
}

func TestLoopCancelBatchPostsOverride(t *testing.T) {
	var calls []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/control/override" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		calls = append(calls, payload)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "cancel",
		"--reason", "test-reason", "--actor", "test-actor", "loop-a", "loop-b",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	for _, payload := range calls {
		if payload["target_state"] != "cancelled" {
			t.Fatalf("expected target_state=cancelled, got %#v", payload)
		}
		if payload["reason"] != "test-reason" {
			t.Fatalf("expected reason, got %#v", payload)
		}
	}
}

func TestLoopDetachPostsControlDetach(t *testing.T) {
	var calls []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/control/detach") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		calls = append(calls, payload)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "detach",
		"--actor", "test-actor", "loop-a",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0]["actor"] != "test-actor" {
		t.Fatalf("expected actor=test-actor, got %#v", calls[0])
	}
}

func TestLoopCommandPostsControlCommand(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/control/command") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_ = json.NewEncoder(w).Encode(map[string]any{"accepted": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "command",
		"loop-a", "--actor", "test-actor", "--command", "pause",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if got["actor"] != "test-actor" || got["command"] != "pause" {
		t.Fatalf("unexpected payload: %#v", got)
	}
}

func TestLoopAttachFallsBackWhenEndpointMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/control/attach"):
			http.Error(w, "endpoint not found", http.StatusNotFound)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/journal"):
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"sequence": 1, "message": "entry"},
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/loops/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"state": map[string]any{"state": string(model.LoopStateCancelled)},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "attach",
		"--interval", (10 * time.Millisecond).String(), "--follow=false", "loop-a",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "not_supported") {
		t.Fatalf("expected attach fallback output, got %s", stdout.String())
	}
}

func TestPRDCreateTemplateToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prd.md")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--output", "json", "prd", "create", "Auth Flow", "--template", "feature", "--out", path,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(content), "## Goal") {
		t.Fatalf("expected feature template content, got: %s", string(content))
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out["template"] != "feature" {
		t.Fatalf("expected feature template metadata, got %#v", out)
	}
}

func TestPRDSubmitIncludesLoopIDsAndValidationErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/ingress/prd" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"item_index": 0, "loop_id": "loop-1", "status": "unresolved", "created": true},
				{"item_index": 1, "status": "error", "message": "task title is required", "source_ref": "prd:doc#x"},
			},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	prdPath := filepath.Join(dir, "prd.md")
	if err := os.WriteFile(prdPath, []byte("# Test PRD\n- [ ] One task"), 0o600); err != nil {
		t.Fatalf("write prd file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "prd", "submit", "--file", prdPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	loopIDs, ok := out["loop_ids"].([]any)
	if !ok || len(loopIDs) != 1 || loopIDs[0] != "loop-1" {
		t.Fatalf("expected loop_ids [loop-1], got %#v", out["loop_ids"])
	}
	errs, ok := out["validation_errors"].([]any)
	if !ok || len(errs) != 1 {
		t.Fatalf("expected one validation error, got %#v", out["validation_errors"])
	}
}
