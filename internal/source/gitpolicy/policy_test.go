package gitpolicy

import (
	"strings"
	"testing"
)

func TestDefaultPolicyValidates(t *testing.T) {
	p := DefaultPolicy()
	if err := p.Validate(); err != nil {
		t.Fatalf("default policy should validate: %v", err)
	}
}

func TestBranchNameIncludesLoopAttempt(t *testing.T) {
	p := DefaultPolicy()
	branch, err := BranchName(p, BranchContext{
		LoopID:  "TD-123/alpha",
		Attempt: 2,
	})
	if err != nil {
		t.Fatalf("branch name failed: %v", err)
	}
	if branch != "smith/loop/td-123-alpha/a2" {
		t.Fatalf("unexpected branch name %q", branch)
	}
}

func TestCommitMessageModes(t *testing.T) {
	p := DefaultPolicy()
	ctx := CommitContext{
		LoopID:        "td-123",
		CorrelationID: "corr-123",
		Summary:       "apply schema migration",
	}

	checkpoint, err := CommitMessage(p, CommitModeCheckpoint, ctx)
	if err != nil {
		t.Fatalf("checkpoint commit failed: %v", err)
	}
	if !strings.HasPrefix(checkpoint, "chore(loop-checkpoint):") {
		t.Fatalf("unexpected checkpoint commit %q", checkpoint)
	}

	final, err := CommitMessage(p, CommitModeFinal, ctx)
	if err != nil {
		t.Fatalf("final commit failed: %v", err)
	}
	if !strings.HasPrefix(final, "feat(loop): apply schema migration") {
		t.Fatalf("unexpected final header %q", final)
	}
	if !strings.Contains(final, "Loop-ID: td-123") || !strings.Contains(final, "Correlation-ID: corr-123") {
		t.Fatalf("expected traceability trailers in final commit: %q", final)
	}
}

func TestFinalizeBranchCommitsDropsCheckpointNoise(t *testing.T) {
	p := DefaultPolicy()
	messages := []string{
		"chore(loop-checkpoint): draft 1 [corr-1]",
		"fix(loop): improve retry behavior",
		"chore(loop-checkpoint): draft 2 [corr-1]",
	}

	filtered, err := FinalizeBranchCommits(messages, p)
	if err != nil {
		t.Fatalf("finalize failed: %v", err)
	}
	if len(filtered) != 1 || filtered[0] != "fix(loop): improve retry behavior" {
		t.Fatalf("unexpected filtered commits %+v", filtered)
	}
}

func TestValidateRejectsInvalidMergeMethod(t *testing.T) {
	p := DefaultPolicy()
	p.MergeMethod = "octopus"
	if err := p.Validate(); err == nil {
		t.Fatal("expected invalid merge method error")
	}
}

func TestValidateRejectsInvalidBranchCleanupPolicy(t *testing.T) {
	p := DefaultPolicy()
	p.BranchCleanup = "after_archive"
	if err := p.Validate(); err == nil {
		t.Fatal("expected invalid branch cleanup policy error")
	}
}

func TestValidateRejectsInvalidConflictPolicy(t *testing.T) {
	p := DefaultPolicy()
	p.ConflictPolicy = "favor_ours"
	if err := p.Validate(); err == nil {
		t.Fatal("expected invalid conflict policy error")
	}
}

func TestValidateRejectsCleanupDeleteMismatch(t *testing.T) {
	p := DefaultPolicy()
	p.BranchCleanup = BranchCleanupNever
	p.DeleteBranchOnMerge = true
	if err := p.Validate(); err == nil {
		t.Fatal("expected cleanup/delete mismatch error")
	}
}
