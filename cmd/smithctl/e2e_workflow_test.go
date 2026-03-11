package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoopRuntime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/loops/loop-123/runtime" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"loop_id":    "loop-123",
			"pod_name":   "smith-replica-123",
			"namespace":  "smith-system",
			"attachable": true,
		})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--server", srv.URL, "--output", "json", "loop", "runtime", "loop-123"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out["pod_name"] != "smith-replica-123" || out["attachable"] != true {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestLoopCost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/reporting/cost" || r.URL.Query().Get("loop_id") != "loop-123" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"loop_id":        "loop-123",
			"provider_id":    "codex",
			"model":          "gpt-4",
			"total_tokens":   1500,
			"total_cost_usd": 0.045,
		})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--server", srv.URL, "--output", "json", "loop", "cost", "loop-123"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out["total_tokens"] != float64(1500) || out["total_cost_usd"] != 0.045 || out["provider_id"] != "codex" || out["model"] != "gpt-4" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestLoopCreateFromGitHubIssueID(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/ingress/github/issues" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	// We use repo#number format to avoid inferCurrentRepo which might fail in some environments
	code := run([]string{"--server", srv.URL, "--output", "json", "loop", "create", "--from-github", "org/repo#456"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	issues, ok := received["issues"].([]any)
	if !ok || len(issues) != 1 {
		t.Fatalf("expected one issue, got %#v", received)
	}
	issue := issues[0].(map[string]any)
	if issue["repository"] != "org/repo" || issue["number"] != float64(456) {
		t.Fatalf("unexpected issue payload: %#v", issue)
	}
}
