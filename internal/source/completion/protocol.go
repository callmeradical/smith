package completion

import (
	"context"
	"errors"
	"fmt"
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

type CommitRequest struct {
	LoopID        string
	CorrelationID string
	FinalDiff     string
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
	RecordPhase(ctx context.Context, record PhaseRecord) error
	SetStateSynced(ctx context.Context, loopID string, commitSHA string) error
	SetStateUnresolved(ctx context.Context, loopID string, reason string) error
}

type GitWriter interface {
	CommitAndPush(ctx context.Context, loopID string, finalDiff string) (string, error)
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
	}); err != nil {
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
	}); err != nil {
		return CommitResult{}, err
	}

	if err := p.store.SetStateSynced(ctx, req.LoopID, commitSHA); err != nil {
		_ = p.store.RecordPhase(ctx, PhaseRecord{
			LoopID:      req.LoopID,
			Phase:       PhaseCompensationNeed,
			CommitSHA:   commitSHA,
			Description: "state sync failed after code commit",
		})

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
		})
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
	}); err != nil {
		return CommitResult{}, err
	}

	return CommitResult{
		Outcome:   OutcomeSynced,
		CommitSHA: commitSHA,
	}, nil
}
