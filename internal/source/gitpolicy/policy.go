package gitpolicy

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidPolicy = errors.New("invalid git policy")
	fragmentReplacer = strings.NewReplacer("/", "-", "_", "-", " ", "-", ".", "-")
	fragmentRegex    = regexp.MustCompile(`[^a-z0-9\-]+`)
)

type MergeMethod string

const (
	MergeMethodSquash MergeMethod = "squash"
	MergeMethodRebase MergeMethod = "rebase"
	MergeMethodMerge  MergeMethod = "merge"
)

type BranchCleanupPolicy string

const (
	BranchCleanupOnMerge BranchCleanupPolicy = "on_merge"
	BranchCleanupNever   BranchCleanupPolicy = "never"
)

type ConflictPolicy string

const (
	ConflictPolicyManualReview ConflictPolicy = "manual_review"
	ConflictPolicyFailFast     ConflictPolicy = "fail_fast"
)

type Policy struct {
	BranchPrefix        string
	AllowCheckpointPush bool
	CheckpointPrefix    string
	FinalCommitType     string
	FinalCommitScope    string
	MergeMethod         MergeMethod
	DeleteBranchOnMerge bool
	BranchCleanup       BranchCleanupPolicy
	ConflictPolicy      ConflictPolicy
}

type BranchContext struct {
	LoopID        string
	Attempt       int
	CorrelationID string
}

type CommitContext struct {
	LoopID        string
	CorrelationID string
	Summary       string
}

type CommitMode string

const (
	CommitModeCheckpoint CommitMode = "checkpoint"
	CommitModeFinal      CommitMode = "final"
)

func DefaultPolicy() Policy {
	return Policy{
		BranchPrefix:        "smith/loop",
		AllowCheckpointPush: true,
		CheckpointPrefix:    "chore(loop-checkpoint)",
		FinalCommitType:     "feat",
		FinalCommitScope:    "loop",
		MergeMethod:         MergeMethodSquash,
		DeleteBranchOnMerge: true,
		BranchCleanup:       BranchCleanupOnMerge,
		ConflictPolicy:      ConflictPolicyManualReview,
	}
}

func (p Policy) Validate() error {
	if strings.TrimSpace(p.BranchPrefix) == "" {
		return fmt.Errorf("%w: branch prefix is required", ErrInvalidPolicy)
	}
	if strings.TrimSpace(p.CheckpointPrefix) == "" {
		return fmt.Errorf("%w: checkpoint prefix is required", ErrInvalidPolicy)
	}
	if strings.TrimSpace(p.FinalCommitType) == "" {
		return fmt.Errorf("%w: final commit type is required", ErrInvalidPolicy)
	}
	if strings.TrimSpace(p.FinalCommitScope) == "" {
		return fmt.Errorf("%w: final commit scope is required", ErrInvalidPolicy)
	}
	switch p.MergeMethod {
	case MergeMethodSquash, MergeMethodRebase, MergeMethodMerge:
	default:
		return fmt.Errorf("%w: unsupported merge method %q", ErrInvalidPolicy, p.MergeMethod)
	}
	switch p.BranchCleanup {
	case BranchCleanupOnMerge, BranchCleanupNever:
	default:
		return fmt.Errorf("%w: unsupported branch cleanup policy %q", ErrInvalidPolicy, p.BranchCleanup)
	}
	switch p.ConflictPolicy {
	case ConflictPolicyManualReview, ConflictPolicyFailFast:
	default:
		return fmt.Errorf("%w: unsupported conflict policy %q", ErrInvalidPolicy, p.ConflictPolicy)
	}
	if p.DeleteBranchOnMerge && p.BranchCleanup != BranchCleanupOnMerge {
		return fmt.Errorf("%w: delete branch on merge requires branch cleanup policy on_merge", ErrInvalidPolicy)
	}
	if !p.DeleteBranchOnMerge && p.BranchCleanup == BranchCleanupOnMerge {
		return fmt.Errorf("%w: branch cleanup policy on_merge requires delete branch on merge", ErrInvalidPolicy)
	}
	return nil
}

func BranchName(p Policy, ctx BranchContext) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	if ctx.LoopID == "" {
		return "", errors.New("loop id is required")
	}
	cleanLoop := sanitizeFragment(ctx.LoopID)
	if cleanLoop == "" {
		return "", errors.New("loop id cannot be sanitized to empty value")
	}
	return fmt.Sprintf("%s/%s/a%d", strings.TrimSuffix(p.BranchPrefix, "/"), cleanLoop, ctx.Attempt), nil
}

func CommitMessage(p Policy, mode CommitMode, ctx CommitContext) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	if ctx.LoopID == "" || ctx.CorrelationID == "" {
		return "", errors.New("loop id and correlation id are required")
	}
	if strings.TrimSpace(ctx.Summary) == "" {
		return "", errors.New("summary is required")
	}

	switch mode {
	case CommitModeCheckpoint:
		if !p.AllowCheckpointPush {
			return "", errors.New("checkpoint commits are disabled by policy")
		}
		return fmt.Sprintf("%s: %s [%s]", p.CheckpointPrefix, strings.TrimSpace(ctx.Summary), ctx.CorrelationID), nil
	case CommitModeFinal:
		header := fmt.Sprintf("%s(%s): %s", p.FinalCommitType, p.FinalCommitScope, strings.TrimSpace(ctx.Summary))
		body := fmt.Sprintf("\n\nLoop-ID: %s\nCorrelation-ID: %s", ctx.LoopID, ctx.CorrelationID)
		return header + body, nil
	default:
		return "", errors.New("unsupported commit mode")
	}
}

func IsCheckpointCommit(message string, p Policy) bool {
	return strings.HasPrefix(strings.TrimSpace(message), p.CheckpointPrefix+":")
}

func FinalizeBranchCommits(messages []string, p Policy) ([]string, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	var final []string
	for _, message := range messages {
		if !IsCheckpointCommit(message, p) {
			final = append(final, message)
		}
	}
	return final, nil
}

func sanitizeFragment(v string) string {
	lower := strings.ToLower(strings.TrimSpace(v))
	lower = fragmentReplacer.Replace(lower)
	lower = fragmentRegex.ReplaceAllString(lower, "")
	lower = strings.Trim(lower, "-")
	if len(lower) > 48 {
		lower = lower[:48]
	}
	return lower
}
