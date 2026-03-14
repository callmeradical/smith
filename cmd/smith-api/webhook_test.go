package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smith/internal/source/store"
)

func TestHandleGitHubWebhook(t *testing.T) {
	ms := store.NewMemStore()
	s := &server{
		store:   ms,
		presets: newPresetCatalog("standard"),
	}

	payload := map[string]any{
		"action": "opened",
		"issue": map[string]any{
			"number":   123,
			"title":    "Test Issue",
			"body":     "Test Body",
			"html_url": "https://github.com/acme/smith/issues/123",
			"id":       9999,
		},
		"repository": map[string]any{
			"full_name": "acme/smith",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/github/issues", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issues")

	rr := httptest.NewRecorder()
	s.handleGitHubWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	loopID, _ := res["loop_id"].(string)
	if !strings.Contains(loopID, "acme-smith") {
		t.Fatalf("Expected loop_id to contain acme-smith, got %q", loopID)
	}

	// Verify it's in the store
	state, found, err := ms.GetState(req.Context(), loopID)
	if err != nil || !found {
		t.Fatalf("Loop not found in store: %v", err)
	}
	if state.Record.State != "unresolved" {
		t.Fatalf("Expected state unresolved, got %q", state.Record.State)
	}
}

func TestHandleGitHubWebhook_IgnoredEvents(t *testing.T) {
	s := &server{}

	tests := []struct {
		name   string
		event  string
		action string
		want   string
	}{
		{"wrong event", "push", "opened", "not-an-issue-event"},
		{"wrong action", "issues", "labeled", "not-an-opened-issue"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{"action": tc.action}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/github/issues", bytes.NewReader(body))
			req.Header.Set("X-GitHub-Event", tc.event)

			rr := httptest.NewRecorder()
			s.handleGitHubWebhook(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("Expected 200, got %d", rr.Code)
			}
			var res map[string]string
			json.Unmarshal(rr.Body.Bytes(), &res)
			if res["reason"] != tc.want {
				t.Fatalf("Expected reason %q, got %q", tc.want, res["reason"])
			}
		})
	}
}
