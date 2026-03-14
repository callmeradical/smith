package completion

import (
	"context"
	"errors"
	"testing"

	"smith/internal/source/model"
)

func TestExecuteSuccess(t *testing.T) {
        store := &fakeStore{}
        git := &fakeGit{commitSHA: "abc123"}
        p := NewProtocol(store, git)

        result, err := p.Execute(context.Background(), CommitRequest{
                LoopID:        "loop-1",
                CorrelationID: "corr-1",
                FinalDiff:     "diff",
        })
        if err != nil {
                t.Fatalf("unexpected error: %v", err)
        }
        if result.Outcome != OutcomeSynced {
                t.Fatalf("expected synced outcome, got %q", result.Outcome)
        }
        if result.CommitSHA != "abc123" {
                t.Fatalf("unexpected commit sha %q", result.CommitSHA)
        }
        assertHasPhase(t, store.phases, PhasePrepared)
        assertHasPhase(t, store.phases, PhaseCodeCommitted)
        assertHasPhase(t, store.phases, PhaseStateCommitted)
}

func TestExecuteSuccessWithPullRequest(t *testing.T) {
        store := &fakeStore{}
        git := &fakeGit{commitSHA: "abc123", prURL: "https://github.com/pr/1"}
        p := NewProtocol(store, git)

        result, err := p.Execute(context.Background(), CommitRequest{
                LoopID:        "loop-pr",
                CorrelationID: "corr-pr",
                FinalDiff:     "diff",
                PullRequest:   true,
                PRTitle:       "Title",
                PRBody:        "Body",
        })
        if err != nil {
                t.Fatalf("unexpected error: %v", err)
        }
        if git.prCalls != 1 {
                t.Fatalf("expected one PR call, got %d", git.prCalls)
        }
        if result.Outcome != OutcomeSynced {
                t.Fatalf("expected synced outcome, got %q", result.Outcome)
        }
        assertHasPhase(t, store.phases, PhasePrepared)
        assertHasPhase(t, store.phases, PhaseCodeCommitted)
        assertHasPhase(t, store.phases, PhaseStateCommitted)

        // Verify PR URL journaled
        foundPR := false
        for _, journal := range store.phases {
                if journal.Metadata != nil && journal.Metadata["pr_url"] == "https://github.com/pr/1" {
                        foundPR = true
                        break
                }
        }
        if !foundPR {
                t.Fatal("PR URL not found in journal")
        }
}

func TestExecuteCommitFailureIsRetryable(t *testing.T) {

	store := &fakeStore{}
	git := &fakeGit{commitErr: errors.New("push failed")}
	p := NewProtocol(store, git)

	result, err := p.Execute(context.Background(), CommitRequest{
		LoopID:        "loop-2",
		CorrelationID: "corr-2",
		FinalDiff:     "diff",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrCodeCommitFailed) {
		t.Fatalf("expected ErrCodeCommitFailed, got %v", err)
	}
	if result.Outcome != OutcomeRetryable {
		t.Fatalf("expected retryable, got %q", result.Outcome)
	}
	if store.unresolvedReason != "commit-push-failed" {
		t.Fatalf("expected unresolved reason set, got %q", store.unresolvedReason)
	}
}

func TestExecuteSyncFailureCompensates(t *testing.T) {
	store := &fakeStore{syncErr: errors.New("etcd unavailable")}
	git := &fakeGit{commitSHA: "def456"}
	p := NewProtocol(store, git)

	result, err := p.Execute(context.Background(), CommitRequest{
		LoopID:        "loop-3",
		CorrelationID: "corr-3",
		FinalDiff:     "diff",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrStateFinalizeFailed) {
		t.Fatalf("expected ErrStateFinalizeFailed, got %v", err)
	}
	if result.Outcome != OutcomeRetryable {
		t.Fatalf("expected retryable outcome, got %q", result.Outcome)
	}
	if git.revertCalls != 1 {
		t.Fatalf("expected one revert call, got %d", git.revertCalls)
	}
	assertHasPhase(t, store.phases, PhaseCompensationNeed)
	assertHasPhase(t, store.phases, PhaseCompensated)
	if store.unresolvedReason != "compensated-after-sync-failure" {
		t.Fatalf("unexpected unresolved reason %q", store.unresolvedReason)
	}
}

func TestExecuteSyncFailureAndRevertFailureSignalsCompensationRequired(t *testing.T) {
	store := &fakeStore{syncErr: errors.New("cas mismatch")}
	git := &fakeGit{
		commitSHA: "xyz789",
		revertErr: errors.New("revert failed"),
	}
	p := NewProtocol(store, git)

	result, err := p.Execute(context.Background(), CommitRequest{
		LoopID:        "loop-4",
		CorrelationID: "corr-4",
		FinalDiff:     "diff",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrAmbiguousTerminalGuard) {
		t.Fatalf("expected ErrAmbiguousTerminalGuard, got %v", err)
	}
	if result.Outcome != OutcomeCompensationRequired {
		t.Fatalf("expected compensation required outcome, got %q", result.Outcome)
	}
	assertHasPhase(t, store.phases, PhaseCompensationNeed)
	if hasPhase(store.phases, PhaseStateCommitted) {
		t.Fatal("state committed phase should not be set on sync failure")
	}
}

func TestExecuteRequiresLoopID(t *testing.T) {
	store := &fakeStore{}
	git := &fakeGit{commitSHA: "abc123"}
	p := NewProtocol(store, git)

	_, err := p.Execute(context.Background(), CommitRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func assertHasPhase(t *testing.T, phases []model.JournalEntry, phase Phase) {
	t.Helper()
	if !hasPhase(phases, phase) {
		t.Fatalf("expected phase %q in %+v", phase, phases)
	}
}

func hasPhase(phases []model.JournalEntry, phase Phase) bool {
	for _, record := range phases {
		if record.Metadata != nil && record.Metadata["completion_phase"] == string(phase) {
			return true
		}
	}
	return false
}

type fakeStore struct {
	phases           []model.JournalEntry
	syncErr          error
	unresolvedReason string
}

func (f *fakeStore) RecordPhase(_ context.Context, record model.JournalEntry) error {
	f.phases = append(f.phases, record)
	return nil
}

func (f *fakeStore) SetStateSynced(_ context.Context, _ string, _ string) error {
	return f.syncErr
}

func (f *fakeStore) SetStateUnresolved(_ context.Context, _ string, reason string) error {
        f.unresolvedReason = reason
        return nil
}

func (f *fakeStore) AppendJournal(_ context.Context, entry model.JournalEntry) error {
        f.phases = append(f.phases, entry)
        return nil
}

type fakeGit struct {

        commitSHA   string
        commitErr   error
        prURL       string
        prErr       error
        prCalls     int
        revertErr   error
        revertCalls int
}

func (f *fakeGit) CommitAndPush(_ context.Context, _ string, _ string) (string, error) {
        if f.commitErr != nil {
                return "", f.commitErr
        }
        return f.commitSHA, nil
}

func (f *fakeGit) CreatePullRequest(_ context.Context, _ string, _ string, _ string, _ string) (string, error) {
        f.prCalls++
        if f.prErr != nil {
                return "", f.prErr
        }
        return f.prURL, nil
}

func (f *fakeGit) Revert(_ context.Context, _ string, _ string) error {

	f.revertCalls++
	return f.revertErr
}
