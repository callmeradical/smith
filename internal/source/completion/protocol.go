package completion

import (
	"context"
	"errors"
	"fmt"

	"smith/internal/source/model"
)

var (
	ErrCodeCommitFailed       = errors.New("code commit failed")
	ErrStateFinalizeFailed    = errors.New("state finalize failed")
	ErrCompensationFailed     = errors.New("compensation failed")
	ErrAmbiguousTerminalGuard = errors.New("ambiguous terminal prevented")
)

type Phase string

const (
	PhasePrepared         Phase = "prepared"
	PhaseCodeCommitted    Phase = "code_committed"
	PhaseStateCommitted   Phase = "state_committed"
	PhaseCompensated      Phase = "compensated"
	PhaseCompensationNeed Phase = "compensation_needed"
)

type PhaseRecord struct {
	LoopID      string
	Phase       Phase
	CommitSHA   string
	Description string
}

func (r PhaseRecord) ToJournal() model.JournalEntry {
	return model.JournalEntry{
		LoopID:    r.LoopID,
		Phase:     "completion",
		Level:     "info",
		ActorType: "replica",
		ActorID:   "smith-replica",
		Message:   fmt.Sprintf("phase: %s - %s", r.Phase, r.Description),
		Metadata: map[string]string{
			"completion_phase": string(r.Phase),
			"commit_sha":       r.CommitSHA,
		},
	}
}

type CommitRequest struct {
        LoopID        string
        CorrelationID string
        FinalDiff     string
        PullRequest   bool
        PRTitle       string
        PRBody        string
}

type Outcome string
const (
	OutcomeSynced               Outcome = "synced"
	OutcomeRetryable            Outcome = "retryable"
	OutcomeCompensationRequired Outcome = "compensation_required"
)

type CommitResult struct {
	Outcome   Outcome
	CommitSHA string
}

type PhaseStore interface {
        RecordPhase(ctx context.Context, record model.JournalEntry) error
        SetStateSynced(ctx context.Context, loopID string, commitSHA string) error
        SetStateUnresolved(ctx context.Context, loopID string, reason string) error
        AppendJournal(ctx context.Context, entry model.JournalEntry) error
}
type GitWriter interface {
        CommitAndPush(ctx context.Context, loopID string, finalDiff string) (string, error)
        CreatePullRequest(ctx context.Context, loopID string, commitSHA string, title string, body string) (string, error)
        Revert(ctx context.Context, loopID string, commitSHA string) error
}

type Protocol struct {
        store PhaseStore
        git   GitWriter
}
func NewProtocol(store PhaseStore, git GitWriter) *Protocol {
	return &Protocol{
		store: store,
		git:   git,
	}
}

func (p *Protocol) Execute(ctx context.Context, req CommitRequest) (CommitResult, error) {
	if req.LoopID == "" {
		return CommitResult{}, errors.New("loop id is required")
	}

	if err := p.store.RecordPhase(ctx, PhaseRecord{
		LoopID:      req.LoopID,
		Phase:       PhasePrepared,
		Description: "completion protocol prepared",
	}.ToJournal()); err != nil {
		return CommitResult{}, err
	}

	commitSHA, err := p.git.CommitAndPush(ctx, req.LoopID, req.FinalDiff)
	if err != nil {
		_ = p.store.SetStateUnresolved(ctx, req.LoopID, "commit-push-failed")
		return CommitResult{
			Outcome: OutcomeRetryable,
		}, fmt.Errorf("%w: %v", ErrCodeCommitFailed, err)
	}

	if err := p.store.RecordPhase(ctx, PhaseRecord{
	        LoopID:      req.LoopID,
	        Phase:       PhaseCodeCommitted,
	        CommitSHA:   commitSHA,
	        Description: "code commit pushed",
	}.ToJournal()); err != nil {
	        return CommitResult{}, err
	}

	if req.PullRequest {
	        prURL, err := p.git.CreatePullRequest(ctx, req.LoopID, commitSHA, req.PRTitle, req.PRBody)
	        if err != nil {
	                _ = p.store.AppendJournal(ctx, model.JournalEntry{
	                        LoopID:        req.LoopID,
	                        Phase:         "completion",
	                        Level:         "warn",
	                        ActorType:     "replica",
	                        ActorID:       "smith-replica",
	                        Message:       "pull request creation failed; proceeding with commit only",
	                        CorrelationID: req.CorrelationID,
	                        Metadata: map[string]string{
	                                "error": err.Error(),
	                        },
	                })
	        } else {
	                _ = p.store.AppendJournal(ctx, model.JournalEntry{
	                        LoopID:        req.LoopID,
	                        Phase:         "completion",
	                        Level:         "info",
	                        ActorType:     "replica",
	                        ActorID:       "smith-replica",
	                        Message:       "pull request created: " + prURL,
	                        CorrelationID: req.CorrelationID,
	                        Metadata: map[string]string{
	                                "pr_url": prURL,
	                        },
	                })
	        }
	}

	if err := p.store.SetStateSynced(ctx, req.LoopID, commitSHA); err != nil {

		_ = p.store.RecordPhase(ctx, PhaseRecord{
			LoopID:      req.LoopID,
			Phase:       PhaseCompensationNeed,
			CommitSHA:   commitSHA,
			Description: "state sync failed after code commit",
		}.ToJournal())

		if revertErr := p.git.Revert(ctx, req.LoopID, commitSHA); revertErr != nil {
			return CommitResult{
					Outcome:   OutcomeCompensationRequired,
					CommitSHA: commitSHA,
				}, errors.Join(
					fmt.Errorf("%w: %v", ErrStateFinalizeFailed, err),
					fmt.Errorf("%w: %v", ErrCompensationFailed, revertErr),
					ErrAmbiguousTerminalGuard,
				)
		}

		_ = p.store.RecordPhase(ctx, PhaseRecord{
			LoopID:      req.LoopID,
			Phase:       PhaseCompensated,
			CommitSHA:   commitSHA,
			Description: "commit reverted after state sync failure",
		}.ToJournal())
		_ = p.store.SetStateUnresolved(ctx, req.LoopID, "compensated-after-sync-failure")

		return CommitResult{
			Outcome:   OutcomeRetryable,
			CommitSHA: commitSHA,
		}, fmt.Errorf("%w: %v", ErrStateFinalizeFailed, err)
	}

	if err := p.store.RecordPhase(ctx, PhaseRecord{
		LoopID:      req.LoopID,
		Phase:       PhaseStateCommitted,
		CommitSHA:   commitSHA,
		Description: "state transitioned to synced",
	}.ToJournal()); err != nil {
		return CommitResult{}, err
	}

	return CommitResult{
		Outcome:   OutcomeSynced,
		CommitSHA: commitSHA,
	}, nil
}
