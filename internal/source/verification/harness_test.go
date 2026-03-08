package verification

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestValidateHandoff(t *testing.T) {
	ok, _ := validateHandoff(Handoff{LoopID: "l", FinalDiffSummary: "d", ValidationState: "passed", NextSteps: "none"})
	if !ok {
		t.Fatal("expected valid handoff")
	}
	ok, _ = validateHandoff(Handoff{LoopID: "l"})
	if ok {
		t.Fatal("expected invalid handoff")
	}
}

func TestIsAmbiguous(t *testing.T) {
	if !isAmbiguous(PhaseState{CodeCommitted: true, StateCommitted: false, Compensated: false}) {
		t.Fatal("expected ambiguous state")
	}
	if isAmbiguous(PhaseState{CodeCommitted: true, StateCommitted: true, Compensated: false}) {
		t.Fatal("unexpected ambiguous state")
	}
}

func TestVerifyScenario(t *testing.T) {
	repoDir := t.TempDir()
	run(t, exec.Command("git", "-C", repoDir, "init", "-b", "main"))
	run(t, exec.Command("git", "-C", repoDir, "config", "user.name", "tester"))
	run(t, exec.Command("git", "-C", repoDir, "config", "user.email", "tester@example.com"))

	if err := os.MkdirAll(filepath.Join(repoDir, "service"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "service", "handler.txt"), []byte("handler=v2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	run(t, exec.Command("git", "-C", repoDir, "add", "."))
	run(t, exec.Command("git", "-C", repoDir, "commit", "-m", "scenario: single loop success"))
	run(t, exec.Command("git", "-C", repoDir, "checkout", "-b", "scenario/single-loop-success"))

	expectedPath := filepath.Join(t.TempDir(), "expected.json")
	spec := OutcomeSpec{Version: "v1", Scenarios: []ScenarioSpec{{
		ID:                          "single-loop-success",
		Branch:                      "scenario/single-loop-success",
		ExpectedState:               "synced",
		ExpectedFiles:               []string{"service/handler.txt"},
		ExpectedCommitSubjectPrefix: "scenario:",
	}}}
	payload, _ := json.Marshal(spec)
	if err := os.WriteFile(expectedPath, payload, 0o644); err != nil {
		t.Fatalf("write expected: %v", err)
	}

	report := Verify(context.Background(), VerifyInput{
		RepoPath:     repoDir,
		ExpectedPath: expectedPath,
		ScenarioID:   "single-loop-success",
	})
	if !report.Passed {
		t.Fatalf("expected report pass, got %+v", report)
	}
}

func run(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v: %s", err, string(out))
	}
}
