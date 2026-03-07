package verification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type OutcomeSpec struct {
	Version   string         `json:"version"`
	Scenarios []ScenarioSpec `json:"scenarios"`
}

type ScenarioSpec struct {
	ID                          string   `json:"id"`
	Branch                      string   `json:"branch"`
	ExpectedState               string   `json:"expected_state"`
	ExpectedFiles               []string `json:"expected_files,omitempty"`
	ExpectedReason              string   `json:"expected_reason,omitempty"`
	ExpectedCommitSubjectPrefix string   `json:"expected_commit_subject_prefix,omitempty"`
}

type Handoff struct {
	LoopID           string `json:"loop_id"`
	FinalDiffSummary string `json:"final_diff_summary"`
	ValidationState  string `json:"validation_state"`
	NextSteps        string `json:"next_steps"`
}

type PhaseState struct {
	CodeCommitted  bool `json:"code_committed"`
	StateCommitted bool `json:"state_committed"`
	Compensated    bool `json:"compensated"`
}

type CheckResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Details string `json:"details,omitempty"`
}

type Report struct {
	ScenarioID   string        `json:"scenario_id"`
	Passed       bool          `json:"passed"`
	Ambiguous    bool          `json:"ambiguous_completion_state"`
	CommitSHA    string        `json:"commit_sha,omitempty"`
	Checks       []CheckResult `json:"checks"`
	Failures     []string      `json:"failures,omitempty"`
	MachineError string        `json:"machine_error,omitempty"`
}

type VerifyInput struct {
	RepoPath     string
	ExpectedPath string
	ScenarioID   string
	HandoffPath  string
	PhasePath    string
}

func Verify(ctx context.Context, in VerifyInput) Report {
	report := Report{ScenarioID: in.ScenarioID}
	if strings.TrimSpace(in.RepoPath) == "" || strings.TrimSpace(in.ExpectedPath) == "" || strings.TrimSpace(in.ScenarioID) == "" {
		report.MachineError = "repo_path, expected_path, and scenario_id are required"
		return report
	}

	spec, err := loadOutcomeSpec(in.ExpectedPath)
	if err != nil {
		report.MachineError = fmt.Sprintf("load expected outcomes: %v", err)
		return report
	}
	scenario, ok := findScenario(spec, in.ScenarioID)
	if !ok {
		report.MachineError = fmt.Sprintf("scenario not found: %s", in.ScenarioID)
		return report
	}

	branchCheck := checkBranchExists(ctx, in.RepoPath, scenario.Branch)
	report.Checks = append(report.Checks, branchCheck)
	if !branchCheck.Passed {
		report.Failures = append(report.Failures, branchCheck.Details)
		report.Passed = false
		return report
	}

	sha, author, email, subject, err := commitMetadata(ctx, in.RepoPath, scenario.Branch)
	if err != nil {
		report.Checks = append(report.Checks, CheckResult{Name: "commit_metadata", Passed: false, Details: err.Error()})
		report.Failures = append(report.Failures, err.Error())
		report.Passed = false
		return report
	}
	report.CommitSHA = sha

	commitCheck := CheckResult{Name: "commit_metadata", Passed: author != "" && email != "" && subject != "", Details: "author/email/subject present"}
	if !commitCheck.Passed {
		commitCheck.Details = "missing commit metadata"
		report.Failures = append(report.Failures, commitCheck.Details)
	}
	report.Checks = append(report.Checks, commitCheck)

	prefix := strings.TrimSpace(scenario.ExpectedCommitSubjectPrefix)
	if prefix != "" {
		prefixCheck := CheckResult{Name: "commit_subject_prefix", Passed: strings.HasPrefix(subject, prefix), Details: fmt.Sprintf("subject=%q expected_prefix=%q", subject, prefix)}
		if !prefixCheck.Passed {
			report.Failures = append(report.Failures, prefixCheck.Details)
		}
		report.Checks = append(report.Checks, prefixCheck)
	}

	for _, file := range scenario.ExpectedFiles {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		ok, details := fileExistsAtBranch(ctx, in.RepoPath, scenario.Branch, file)
		check := CheckResult{Name: "expected_file:" + file, Passed: ok, Details: details}
		report.Checks = append(report.Checks, check)
		if !check.Passed {
			report.Failures = append(report.Failures, check.Details)
		}
	}

	if strings.TrimSpace(in.HandoffPath) != "" {
		handoff, err := loadHandoff(in.HandoffPath)
		if err != nil {
			check := CheckResult{Name: "handoff_integrity", Passed: false, Details: err.Error()}
			report.Checks = append(report.Checks, check)
			report.Failures = append(report.Failures, check.Details)
		} else {
			ok, details := validateHandoff(handoff)
			check := CheckResult{Name: "handoff_integrity", Passed: ok, Details: details}
			report.Checks = append(report.Checks, check)
			if !ok {
				report.Failures = append(report.Failures, check.Details)
			}
		}
	}

	if strings.TrimSpace(in.PhasePath) != "" {
		phase, err := loadPhase(in.PhasePath)
		if err != nil {
			check := CheckResult{Name: "phase_state", Passed: false, Details: err.Error()}
			report.Checks = append(report.Checks, check)
			report.Failures = append(report.Failures, check.Details)
		} else {
			ambiguous := isAmbiguous(phase)
			report.Ambiguous = ambiguous
			check := CheckResult{Name: "ambiguous_completion_guard", Passed: !ambiguous, Details: "code_committed && !state_committed && !compensated must never happen"}
			report.Checks = append(report.Checks, check)
			if ambiguous {
				report.Failures = append(report.Failures, "ambiguous completion state detected")
			}
		}
	}

	report.Passed = len(report.Failures) == 0 && report.MachineError == ""
	return report
}

func loadOutcomeSpec(path string) (OutcomeSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return OutcomeSpec{}, err
	}
	var out OutcomeSpec
	if err := json.Unmarshal(data, &out); err != nil {
		return OutcomeSpec{}, err
	}
	return out, nil
}

func loadHandoff(path string) (Handoff, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Handoff{}, err
	}
	var out Handoff
	if err := json.Unmarshal(data, &out); err != nil {
		return Handoff{}, err
	}
	return out, nil
}

func loadPhase(path string) (PhaseState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PhaseState{}, err
	}
	var out PhaseState
	if err := json.Unmarshal(data, &out); err != nil {
		return PhaseState{}, err
	}
	return out, nil
}

func validateHandoff(h Handoff) (bool, string) {
	if strings.TrimSpace(h.LoopID) == "" {
		return false, "handoff.loop_id is required"
	}
	if strings.TrimSpace(h.FinalDiffSummary) == "" {
		return false, "handoff.final_diff_summary is required"
	}
	if strings.TrimSpace(h.ValidationState) == "" {
		return false, "handoff.validation_state is required"
	}
	if strings.TrimSpace(h.NextSteps) == "" {
		return false, "handoff.next_steps is required"
	}
	return true, "handoff required fields present"
}

func isAmbiguous(p PhaseState) bool {
	return p.CodeCommitted && !p.StateCommitted && !p.Compensated
}

func findScenario(spec OutcomeSpec, id string) (ScenarioSpec, bool) {
	for _, s := range spec.Scenarios {
		if strings.TrimSpace(s.ID) == strings.TrimSpace(id) {
			return s, true
		}
	}
	return ScenarioSpec{}, false
}

func checkBranchExists(ctx context.Context, repoPath, branch string) CheckResult {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--verify", branch)
	if err := cmd.Run(); err != nil {
		return CheckResult{Name: "branch_exists", Passed: false, Details: fmt.Sprintf("branch not found: %s", branch)}
	}
	return CheckResult{Name: "branch_exists", Passed: true, Details: branch}
}

func commitMetadata(ctx context.Context, repoPath, branch string) (sha, author, email, subject string, err error) {
	out, err := runGit(ctx, repoPath, "show", "-s", "--format=%H|%an|%ae|%s", branch)
	if err != nil {
		return "", "", "", "", err
	}
	parts := strings.Split(strings.TrimSpace(out), "|")
	if len(parts) != 4 {
		return "", "", "", "", errors.New("unexpected commit metadata format")
	}
	return parts[0], parts[1], parts[2], parts[3], nil
}

func fileExistsAtBranch(ctx context.Context, repoPath, branch, file string) (bool, string) {
	_, err := runGit(ctx, repoPath, "cat-file", "-e", fmt.Sprintf("%s:%s", branch, file))
	if err != nil {
		return false, fmt.Sprintf("missing file %s at branch %s", file, branch)
	}
	return true, fmt.Sprintf("found %s at branch %s", file, branch)
}

func runGit(ctx context.Context, repoPath string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoPath}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
