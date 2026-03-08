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

type skillLoop struct {
	LoopID  string
	Skills  []map[string]any
	Journal []map[string]any
}

type skillHarness struct {
	mu     sync.Mutex
	nextID int
	loops  map[string]skillLoop
}

func newSkillHarness() *skillHarness {
	return &skillHarness{
		nextID: 1,
		loops:  map[string]skillLoop{},
	}
}

func TestLoopSkillMountBehavior(t *testing.T) {
	h := newSkillHarness()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/loops":
			h.handleLoopCreate(t, w, r)
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/loops/") && strings.HasSuffix(r.URL.Path, "/journal"):
			h.handleLoopJournal(w, r)
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/loops/"):
			h.handleLoopGet(w, r)
			return
		default:
			http.Error(w, "unexpected route", http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Run("explicit mount path", func(t *testing.T) {
		out := runSmithctl(t, server.URL, "--output", "json", "loop", "create",
			"--title", "Skill explicit",
			"--description", "Skill test",
			"--source-type", "interactive",
			"--source-ref", "terminal/skill-explicit",
			"--skill", "name=commit,source=local://skills/commit,mount_path=/opt/skills/commit",
		)
		loopID := mustGetTopLevelLoopID(t, out)
		skills := getLoopSkills(t, server.URL, loopID)
		if got := asMap(t, skills[0], "skills[0]")["mount_path"]; got != "/opt/skills/commit" {
			t.Fatalf("expected explicit mount_path, got %#v", got)
		}
	})

	t.Run("default codex mount path", func(t *testing.T) {
		out := runSmithctl(t, server.URL, "--output", "json", "loop", "create",
			"--title", "Skill default",
			"--description", "Skill test",
			"--source-type", "interactive",
			"--source-ref", "terminal/skill-default",
			"--skill", "name=commit,source=local://skills/commit",
		)
		loopID := mustGetTopLevelLoopID(t, out)
		skills := getLoopSkills(t, server.URL, loopID)
		if got := asMap(t, skills[0], "skills[0]")["mount_path"]; got != "/smith/skills/commit" {
			t.Fatalf("expected default mount_path, got %#v", got)
		}
	})

	t.Run("missing skill failure", func(t *testing.T) {
		out, errOut, code := runSmithctlWithExitCode(server.URL,
			"--output", "json", "loop", "create",
			"--title", "Skill invalid",
			"--description", "Skill test",
			"--source-type", "interactive",
			"--source-ref", "terminal/skill-invalid",
			"--skill", "name=missing,source=local://unknown/missing",
		)
		if code == 0 {
			t.Fatalf("expected non-zero code for missing skill, got stdout=%s", string(out))
		}
		if !strings.Contains(errOut, "loop create failed") {
			t.Fatalf("expected loop create failed error, got %s", errOut)
		}
	})

	t.Run("journal metadata", func(t *testing.T) {
		out := runSmithctl(t, server.URL, "--output", "json", "loop", "create",
			"--title", "Skill journal",
			"--description", "Skill test",
			"--source-type", "interactive",
			"--source-ref", "terminal/skill-journal",
			"--skill", "name=commit,source=local://skills/commit",
		)
		loopID := mustGetTopLevelLoopID(t, out)
		logs := runSmithctl(t, server.URL, "--output", "json", "loop", "logs", loopID)
		var entries []map[string]any
		if err := json.Unmarshal(logs, &entries); err != nil {
			t.Fatalf("decode loop logs: %v\n%s", err, string(logs))
		}
		if len(entries) == 0 {
			t.Fatalf("expected journal entries, got %s", string(logs))
		}
		meta := asMap(t, entries[0]["metadata"], "metadata")
		if meta["skill_mount_count"] != "1" {
			t.Fatalf("expected skill_mount_count=1, got %#v", meta)
		}
		if meta["skill_mounts"] != "commit" {
			t.Fatalf("expected skill_mounts=commit, got %#v", meta)
		}
	})
}

func (h *skillHarness) handleLoopCreate(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	rawSkills, _ := req["skills"].([]any)
	skills := make([]map[string]any, 0, len(rawSkills))
	resolvedNames := make([]string, 0, len(rawSkills))
	for i, raw := range rawSkills {
		skill := asMap(t, raw, "skills")
		source, _ := skill["source"].(string)
		if !strings.HasPrefix(source, "local://skills/") {
			http.Error(w, "invalid skill source", http.StatusBadRequest)
			return
		}
		name, _ := skill["name"].(string)
		if strings.TrimSpace(name) == "" {
			http.Error(w, "invalid skill name", http.StatusBadRequest)
			return
		}
		if _, ok := skill["mount_path"]; !ok {
			skill["mount_path"] = "/smith/skills/" + strings.ToLower(strings.TrimSpace(name))
		}
		skills = append(skills, skill)
		resolvedNames = append(resolvedNames, strings.TrimSpace(name))
		_ = i
	}

	h.mu.Lock()
	loopID := fmt.Sprintf("loop-skill-%03d", h.nextID)
	h.nextID++
	h.loops[loopID] = skillLoop{
		LoopID: loopID,
		Skills: skills,
		Journal: []map[string]any{{
			"sequence": 1,
			"message":  "replica job scheduled",
			"metadata": map[string]any{
				"skill_mount_count": "1",
				"skill_mounts":      strings.Join(resolvedNames, ","),
			},
		}},
	}
	h.mu.Unlock()

	_ = json.NewEncoder(w).Encode(map[string]any{
		"loop_id": loopID,
		"status":  "synced",
		"created": true,
		"skills":  skills,
	})
}

func (h *skillHarness) handleLoopGet(w http.ResponseWriter, r *http.Request) {
	loopID := strings.TrimPrefix(r.URL.Path, "/v1/loops/")
	h.mu.Lock()
	loop, ok := h.loops[loopID]
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
		"anomaly": map[string]any{
			"id":     loopID,
			"skills": loop.Skills,
		},
	})
}

func (h *skillHarness) handleLoopJournal(w http.ResponseWriter, r *http.Request) {
	loopID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/loops/"), "/journal")
	h.mu.Lock()
	loop, ok := h.loops[loopID]
	h.mu.Unlock()
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(loop.Journal)
}

func getLoopSkills(t *testing.T, serverURL, loopID string) []any {
	t.Helper()
	out := runSmithctl(t, serverURL, "--output", "json", "loop", "get", loopID)
	var body map[string]any
	if err := json.Unmarshal(out, &body); err != nil {
		t.Fatalf("decode loop get response: %v\n%s", err, string(out))
	}
	anomaly := asMap(t, body["anomaly"], "anomaly")
	skills, ok := anomaly["skills"].([]any)
	if !ok {
		t.Fatalf("expected skills in anomaly, got %#v", anomaly)
	}
	return skills
}
