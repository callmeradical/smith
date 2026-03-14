package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	api "smith/pkg/api/v1"
)

func TestClient_CreateLoop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/loops" {
			t.Errorf("expected /v1/loops, got %s", r.URL.Path)
		}

		res := api.LoopCreateResult{
			LoopID:  "loop-123",
			Status:  "unresolved",
			Created: true,
		}
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	req := api.LoopCreateRequest{
		Title: "Test Loop",
	}

	res, err := c.CreateLoop(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateLoop failed: %v", err)
	}

	if res.LoopID != "loop-123" {
		t.Errorf("expected loop-123, got %s", res.LoopID)
	}
}

func TestClient_GetLoop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := api.LoopResponse{
			State: api.State{
				LoopID: "loop-123",
				State:  "running",
			},
		}
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	res, err := c.GetLoop(context.Background(), "loop-123")
	if err != nil {
		t.Fatalf("GetLoop failed: %v", err)
	}

	if res.State.LoopID != "loop-123" {
		t.Errorf("expected loop-123, got %s", res.State.LoopID)
	}
}
