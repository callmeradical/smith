package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"smith/internal/source/model"
)

func TestExecutionImageMetadataFromEnv(t *testing.T) {
	t.Setenv("SMITH_EXECUTION_IMAGE_REF", "ghcr.io/acme/replica@sha256:abc")
	t.Setenv("SMITH_EXECUTION_IMAGE_SOURCE", "loop_environment_container_image")
	t.Setenv("SMITH_EXECUTION_IMAGE_DIGEST", "sha256:abc")
	t.Setenv("SMITH_EXECUTION_IMAGE_PULL_POLICY", "Always")
	t.Setenv("SMITH_LOOP_PROVIDER", "codex")
	t.Setenv("SMITH_LOOP_INVOCATION_METHOD", "github_issue")
	t.Setenv("SMITH_LOOP_SOURCE_TYPE", "github_issue")
	t.Setenv("SMITH_LOOP_SOURCE_REF", "acme/repo#22")
	t.Setenv("SMITH_JOURNAL_RETENTION_MODE", "ttl")
	t.Setenv("SMITH_JOURNAL_RETENTION_TTL", "168h")
	t.Setenv("SMITH_JOURNAL_ARCHIVE_MODE", "s3")
	t.Setenv("SMITH_JOURNAL_ARCHIVE_BUCKET", "smith-journal-archive")

	metadata := runtimeMetadataFromEnv()
	if metadata["execution_image_ref"] != "ghcr.io/acme/replica@sha256:abc" {
		t.Fatalf("unexpected execution_image_ref: %q", metadata["execution_image_ref"])
	}
	if metadata["execution_image_source"] != "loop_environment_container_image" {
		t.Fatalf("unexpected execution_image_source: %q", metadata["execution_image_source"])
	}
	if metadata["execution_image_digest"] != "sha256:abc" {
		t.Fatalf("unexpected execution_image_digest: %q", metadata["execution_image_digest"])
	}
	if metadata["execution_image_pull_policy"] != "Always" {
		t.Fatalf("unexpected execution_image_pull_policy: %q", metadata["execution_image_pull_policy"])
	}
	if metadata["loop_invocation_method"] != "github_issue" {
		t.Fatalf("unexpected loop_invocation_method: %q", metadata["loop_invocation_method"])
	}
	if metadata["loop_provider"] != "codex" {
		t.Fatalf("unexpected loop_provider: %q", metadata["loop_provider"])
	}
	if metadata["loop_source_type"] != "github_issue" {
		t.Fatalf("unexpected loop_source_type: %q", metadata["loop_source_type"])
	}
	if metadata["loop_source_ref"] != "acme/repo#22" {
		t.Fatalf("unexpected loop_source_ref: %q", metadata["loop_source_ref"])
	}
	if metadata["journal_retention_mode"] != "ttl" {
		t.Fatalf("unexpected journal_retention_mode: %q", metadata["journal_retention_mode"])
	}
	if metadata["journal_retention_ttl"] != "168h" {
		t.Fatalf("unexpected journal_retention_ttl: %q", metadata["journal_retention_ttl"])
	}
	if metadata["journal_archive_mode"] != "s3" {
		t.Fatalf("unexpected journal_archive_mode: %q", metadata["journal_archive_mode"])
	}
	if metadata["journal_archive_bucket"] != "smith-journal-archive" {
		t.Fatalf("unexpected journal_archive_bucket: %q", metadata["journal_archive_bucket"])
	}
}

func TestExecutionImageMetadataFromEnvEmpty(t *testing.T) {
	t.Setenv("SMITH_EXECUTION_IMAGE_REF", "")
	t.Setenv("SMITH_EXECUTION_IMAGE_SOURCE", "")
	t.Setenv("SMITH_EXECUTION_IMAGE_DIGEST", "")
	t.Setenv("SMITH_EXECUTION_IMAGE_PULL_POLICY", "")
	t.Setenv("SMITH_LOOP_PROVIDER", "")
	t.Setenv("SMITH_LOOP_INVOCATION_METHOD", "")
	t.Setenv("SMITH_LOOP_SOURCE_TYPE", "")
	t.Setenv("SMITH_LOOP_SOURCE_REF", "")
	t.Setenv("SMITH_JOURNAL_RETENTION_MODE", "")
	t.Setenv("SMITH_JOURNAL_RETENTION_TTL", "")
	t.Setenv("SMITH_JOURNAL_ARCHIVE_MODE", "")
	t.Setenv("SMITH_JOURNAL_ARCHIVE_BUCKET", "")

	if metadata := runtimeMetadataFromEnv(); metadata != nil {
		t.Fatalf("expected nil metadata, got %#v", metadata)
	}
}

func TestReadHandoffFileMissingIsNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	parsed, err := readHandoffFile(path)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if parsed != nil {
		t.Fatalf("expected nil parsed handoff, got %#v", parsed)
	}
}

func TestReadHandoffFileParsesJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "handoff.json")
	if err := os.WriteFile(path, []byte(`{"loop_id":"loop-abc"}`), 0o644); err != nil {
		t.Fatalf("write handoff file: %v", err)
	}
	parsed, err := readHandoffFile(path)
	if err != nil {
		t.Fatalf("readHandoffFile error: %v", err)
	}
	if parsed == nil || parsed.LoopID != "loop-abc" {
		t.Fatalf("unexpected parsed handoff: %#v", parsed)
	}
}

func TestLoadLoopExecutionConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("SMITH_LOOP_PROVIDER", "")
	t.Setenv("SMITH_LOOP_INVOCATION_METHOD", "")
	t.Setenv("SMITH_LOOP_SOURCE_TYPE", "")
	t.Setenv("SMITH_LOOP_SOURCE_REF", "")
	t.Setenv("SMITH_AGENT_CLI_CMD", "")
	t.Setenv("SMITH_AGENT_CLI_CMD_CODEX", "")
	t.Setenv("SMITH_CODEX_CLI_CMD", "")
	t.Setenv("SMITH_LOOP_PRD_PATH", "")
	t.Setenv("SMITH_LOOP_PRD_STORY_COUNT", "")
	t.Setenv("SMITH_ISSUE_PRD_INTERACTIVE", "")
	t.Setenv("SMITH_ISSUE_PRD_INTERACTIVE_WAIT", "")
	t.Setenv("SMITH_ISSUE_PRD_INTERACTIVE_POLL", "")
	t.Setenv("SMITH_ISSUE_WORKFLOW_ENABLED", "")
	t.Setenv("SMITH_LOOP_MAX_ITERATIONS", "")
	t.Setenv("SMITH_LOOP_ITERATION_WAIT", "")

	cfg := loadLoopExecutionConfigFromEnv()
	if cfg.ProviderID != "codex" {
		t.Fatalf("expected default provider codex, got %q", cfg.ProviderID)
	}
	if cfg.InvocationMethod != "unknown" {
		t.Fatalf("expected invocation method unknown, got %q", cfg.InvocationMethod)
	}
	if cfg.SourceType != "" || cfg.SourceRef != "" {
		t.Fatalf("expected empty source fields, got type=%q ref=%q", cfg.SourceType, cfg.SourceRef)
	}
	if cfg.MaxIterations != defaultLoopMaxIterations {
		t.Fatalf("expected default max iterations %d, got %d", defaultLoopMaxIterations, cfg.MaxIterations)
	}
	if cfg.IterationWait != defaultLoopIterationWait {
		t.Fatalf("expected default iteration wait %s, got %s", defaultLoopIterationWait, cfg.IterationWait)
	}
	if cfg.CodexCommand != defaultCodexCLICommand {
		t.Fatalf("expected default codex command %q, got %q", defaultCodexCLICommand, cfg.CodexCommand)
	}
	if cfg.PRDPath != defaultPRDPath {
		t.Fatalf("expected default prd path %q, got %q", defaultPRDPath, cfg.PRDPath)
	}
	if cfg.PRDStoryCount != defaultPRDStoryCount {
		t.Fatalf("expected default PRD story count %d, got %d", defaultPRDStoryCount, cfg.PRDStoryCount)
	}
	if !cfg.InteractivePRD {
		t.Fatal("expected interactive PRD default true")
	}
	if cfg.InteractivePRDWait != defaultInteractiveWait {
		t.Fatalf("expected default interactive wait %s, got %s", defaultInteractiveWait, cfg.InteractivePRDWait)
	}
	if cfg.InteractivePRDPoll != defaultInteractivePoll {
		t.Fatalf("expected default interactive poll %s, got %s", defaultInteractivePoll, cfg.InteractivePRDPoll)
	}
	if !cfg.IssueWorkflowEnabled {
		t.Fatal("expected issue workflow enabled by default")
	}
}

func TestLoadLoopExecutionConfigFromEnvOverrides(t *testing.T) {
	t.Setenv("SMITH_LOOP_PROVIDER", "codex")
	t.Setenv("SMITH_LOOP_INVOCATION_METHOD", "github_issue")
	t.Setenv("SMITH_LOOP_SOURCE_TYPE", "github_issue")
	t.Setenv("SMITH_LOOP_SOURCE_REF", "acme/repo#77")
	t.Setenv("SMITH_AGENT_CLI_CMD", "custom-agent exec -")
	t.Setenv("SMITH_AGENT_CLI_CMD_CODEX", "")
	t.Setenv("SMITH_CODEX_CLI_CMD", "")
	t.Setenv("SMITH_LOOP_PRD_PATH", "tmp/prd.custom.json")
	t.Setenv("SMITH_LOOP_PRD_STORY_COUNT", "7")
	t.Setenv("SMITH_ISSUE_PRD_INTERACTIVE", "false")
	t.Setenv("SMITH_ISSUE_PRD_INTERACTIVE_WAIT", "3m")
	t.Setenv("SMITH_ISSUE_PRD_INTERACTIVE_POLL", "1500ms")
	t.Setenv("SMITH_ISSUE_WORKFLOW_ENABLED", "false")
	t.Setenv("SMITH_LOOP_MAX_ITERATIONS", "42")
	t.Setenv("SMITH_LOOP_ITERATION_WAIT", "1500ms")

	cfg := loadLoopExecutionConfigFromEnv()
	if cfg.ProviderID != "codex" {
		t.Fatalf("expected provider codex, got %q", cfg.ProviderID)
	}
	if cfg.InvocationMethod != "github_issue" {
		t.Fatalf("expected invocation method github_issue, got %q", cfg.InvocationMethod)
	}
	if cfg.MaxIterations != 42 {
		t.Fatalf("expected max iterations 42, got %d", cfg.MaxIterations)
	}
	if cfg.IterationWait != 1500*time.Millisecond {
		t.Fatalf("expected iteration wait 1500ms, got %s", cfg.IterationWait)
	}
	if cfg.SourceType != "github_issue" || cfg.SourceRef != "acme/repo#77" {
		t.Fatalf("unexpected source fields type=%q ref=%q", cfg.SourceType, cfg.SourceRef)
	}
	if cfg.CodexCommand != "custom-agent exec -" {
		t.Fatalf("unexpected codex command: %q", cfg.CodexCommand)
	}
	if cfg.PRDPath != "tmp/prd.custom.json" {
		t.Fatalf("unexpected prd path: %q", cfg.PRDPath)
	}
	if cfg.PRDStoryCount != 7 {
		t.Fatalf("unexpected prd story count: %d", cfg.PRDStoryCount)
	}
	if cfg.InteractivePRD {
		t.Fatal("expected interactive PRD false")
	}
	if cfg.InteractivePRDWait != 3*time.Minute {
		t.Fatalf("unexpected interactive wait: %s", cfg.InteractivePRDWait)
	}
	if cfg.InteractivePRDPoll != 1500*time.Millisecond {
		t.Fatalf("unexpected interactive poll: %s", cfg.InteractivePRDPoll)
	}
        if cfg.IssueWorkflowEnabled {
                t.Fatal("expected issue workflow disabled")
        }
}

func TestLoadLoopExecutionConfigFromEnvStage(t *testing.T) {
        t.Setenv("SMITH_LOOP_STAGE", "prd")
        cfg := loadLoopExecutionConfigFromEnv()
        if cfg.Stage != "prd" {
                t.Fatalf("expected stage prd, got %q", cfg.Stage)
        }
}

func TestLoadLoopExecutionConfigFromEnvRejectsInvalidValues(t *testing.T) {

	t.Setenv("SMITH_LOOP_PROVIDER", "")
	t.Setenv("SMITH_AGENT_CLI_CMD", "")
	t.Setenv("SMITH_AGENT_CLI_CMD_CODEX", "")
	t.Setenv("SMITH_CODEX_CLI_CMD", "")
	t.Setenv("SMITH_LOOP_INVOCATION_METHOD", "")
	t.Setenv("SMITH_LOOP_MAX_ITERATIONS", "-5")
	t.Setenv("SMITH_LOOP_ITERATION_WAIT", "not-a-duration")
	t.Setenv("SMITH_LOOP_PRD_STORY_COUNT", "-3")
	t.Setenv("SMITH_ISSUE_PRD_INTERACTIVE_WAIT", "bad")
	t.Setenv("SMITH_ISSUE_PRD_INTERACTIVE_POLL", "bad")
	t.Setenv("SMITH_ISSUE_PRD_INTERACTIVE", "invalid")

	cfg := loadLoopExecutionConfigFromEnv()
	if cfg.MaxIterations != defaultLoopMaxIterations {
		t.Fatalf("expected fallback max iterations %d, got %d", defaultLoopMaxIterations, cfg.MaxIterations)
	}
	if cfg.IterationWait != defaultLoopIterationWait {
		t.Fatalf("expected fallback iteration wait %s, got %s", defaultLoopIterationWait, cfg.IterationWait)
	}
	if cfg.InteractivePRDWait != defaultInteractiveWait {
		t.Fatalf("expected fallback interactive wait %s, got %s", defaultInteractiveWait, cfg.InteractivePRDWait)
	}
	if cfg.InteractivePRDPoll != defaultInteractivePoll {
		t.Fatalf("expected fallback interactive poll %s, got %s", defaultInteractivePoll, cfg.InteractivePRDPoll)
	}
	if cfg.InteractivePRD != defaultInteractivePRD {
		t.Fatalf("expected fallback interactive bool %t, got %t", defaultInteractivePRD, cfg.InteractivePRD)
	}
	if cfg.PRDStoryCount != defaultPRDStoryCount {
		t.Fatalf("expected fallback PRD story count %d, got %d", defaultPRDStoryCount, cfg.PRDStoryCount)
	}
}

func TestLoadLoopExecutionConfigFromEnvAllowsZeroInteractiveWait(t *testing.T) {
	t.Setenv("SMITH_LOOP_PROVIDER", "codex")
	t.Setenv("SMITH_AGENT_CLI_CMD", "")
	t.Setenv("SMITH_AGENT_CLI_CMD_CODEX", "")
	t.Setenv("SMITH_CODEX_CLI_CMD", "")
	t.Setenv("SMITH_LOOP_INVOCATION_METHOD", "github_issue")
	t.Setenv("SMITH_ISSUE_PRD_INTERACTIVE_WAIT", "0s")

	cfg := loadLoopExecutionConfigFromEnv()
	if cfg.InteractivePRDWait != 0 {
		t.Fatalf("expected interactive wait 0, got %s", cfg.InteractivePRDWait)
	}
}

func TestLoadLoopExecutionConfigFromEnvUsesMethodProfile(t *testing.T) {
	t.Setenv("SMITH_LOOP_PROVIDER", "codex")
	t.Setenv("SMITH_AGENT_CLI_CMD", "")
	t.Setenv("SMITH_AGENT_CLI_CMD_CODEX", "")
	t.Setenv("SMITH_CODEX_CLI_CMD", "")
	t.Setenv("SMITH_LOOP_INVOCATION_METHOD", "manual")
	t.Setenv("SMITH_LOOP_MAX_ITERATIONS", "")
	t.Setenv("SMITH_LOOP_ITERATION_WAIT", "")

	cfg := loadLoopExecutionConfigFromEnv()
	if cfg.InvocationMethod != "manual" {
		t.Fatalf("expected invocation method manual, got %q", cfg.InvocationMethod)
	}
	if cfg.MaxIterations != 120 {
		t.Fatalf("expected manual max iterations 120, got %d", cfg.MaxIterations)
	}
	if cfg.IterationWait != 2*time.Second {
		t.Fatalf("expected manual iteration wait 2s, got %s", cfg.IterationWait)
	}
}

func TestLoadLoopExecutionConfigFromEnvUsesProviderSpecificCommand(t *testing.T) {
	t.Setenv("SMITH_LOOP_PROVIDER", "claude")
	t.Setenv("SMITH_LOOP_INVOCATION_METHOD", "prompt")
	t.Setenv("SMITH_AGENT_CLI_CMD", "")
	t.Setenv("SMITH_AGENT_CLI_CMD_CLAUDE", "claude -p")
	t.Setenv("SMITH_CODEX_CLI_CMD", "")

	cfg := loadLoopExecutionConfigFromEnv()
	if cfg.ProviderID != "claude" {
		t.Fatalf("expected provider claude, got %q", cfg.ProviderID)
	}
	if cfg.CodexCommand != "claude -p" {
		t.Fatalf("expected provider command claude -p, got %q", cfg.CodexCommand)
	}
}

func TestLoadLoopExecutionConfigFromEnvUsesGlobalAgentCommandOverride(t *testing.T) {
	t.Setenv("SMITH_LOOP_PROVIDER", "codex")
	t.Setenv("SMITH_LOOP_INVOCATION_METHOD", "prompt")
	t.Setenv("SMITH_AGENT_CLI_CMD", "custom-agent --stream")
	t.Setenv("SMITH_AGENT_CLI_CMD_CODEX", "codex exec --ignored")
	t.Setenv("SMITH_CODEX_CLI_CMD", "codex exec --legacy")

	cfg := loadLoopExecutionConfigFromEnv()
	if cfg.CodexCommand != "custom-agent --stream" {
		t.Fatalf("expected global agent command override, got %q", cfg.CodexCommand)
	}
}

func TestInteractivePRDCommandHint(t *testing.T) {
	promptPath := "/workspace/.smith/prompts/loop-prd.md"
	prdPath := "/workspace/.agents/tasks/prd.json"
	if got := interactivePRDCommandHint(promptPath, prdPath, 6, "codex exec --yolo --skip-git-repo-check -"); got != "smith --prompt '/workspace/.smith/prompts/loop-prd.md' --out '/workspace/.agents/tasks/prd.json' --stories 6 --agent-cmd 'codex exec --yolo --skip-git-repo-check -'" {
		t.Fatalf("unexpected smith interactive hint: %q", got)
	}
	if got := interactivePRDCommandHint(promptPath, prdPath, 0, ""); got != "smith --prompt '/workspace/.smith/prompts/loop-prd.md' --out '/workspace/.agents/tasks/prd.json'" {
		t.Fatalf("unexpected minimal hint: %q", got)
	}
}

func TestLoopExecutionMetadata(t *testing.T) {
	cfg := loopExecutionConfig{
		ProviderID:         "codex",
		InvocationMethod:   "github_issue",
		SourceType:         "github_issue",
		SourceRef:          "acme/repo#22",
		MaxIterations:      7,
		IterationWait:      3 * time.Second,
		CodexCommand:       "codex exec -",
		PRDPath:            ".agents/tasks/prd.json",
		PRDStoryCount:      8,
		InteractivePRD:     true,
		InteractivePRDWait: 4 * time.Minute,
		InteractivePRDPoll: 2 * time.Second,
	}
	metadata := loopExecutionMetadata(cfg)
	if metadata["loop_provider"] != "codex" {
		t.Fatalf("unexpected loop_provider: %q", metadata["loop_provider"])
	}
	if metadata["loop_invocation_method"] != "github_issue" {
		t.Fatalf("unexpected loop_invocation_method: %q", metadata["loop_invocation_method"])
	}
	if metadata["loop_max_iterations"] != "7" {
		t.Fatalf("unexpected loop_max_iterations: %q", metadata["loop_max_iterations"])
	}
	if metadata["loop_iteration_wait"] != "3s" {
		t.Fatalf("unexpected loop_iteration_wait: %q", metadata["loop_iteration_wait"])
	}
	if metadata["loop_source_type"] != "github_issue" {
		t.Fatalf("unexpected loop_source_type: %q", metadata["loop_source_type"])
	}
	if metadata["loop_source_ref"] != "acme/repo#22" {
		t.Fatalf("unexpected loop_source_ref: %q", metadata["loop_source_ref"])
	}
	if metadata["loop_codex_command"] != "codex exec -" {
		t.Fatalf("unexpected loop_codex_command: %q", metadata["loop_codex_command"])
	}
	if metadata["loop_agent_command"] != "codex exec -" {
		t.Fatalf("unexpected loop_agent_command: %q", metadata["loop_agent_command"])
	}
	if metadata["loop_prd_path"] != ".agents/tasks/prd.json" {
		t.Fatalf("unexpected loop_prd_path: %q", metadata["loop_prd_path"])
	}
	if metadata["loop_prd_story_count"] != "8" {
		t.Fatalf("unexpected loop_prd_story_count: %q", metadata["loop_prd_story_count"])
	}
	if metadata["loop_prd_interactive"] != "true" {
		t.Fatalf("unexpected loop_prd_interactive: %q", metadata["loop_prd_interactive"])
	}
}

func TestExpectedPRDStoryCount(t *testing.T) {
	if got := expectedPRDStoryCount(6, model.Anomaly{}); got != 6 {
		t.Fatalf("expected default story count 6, got %d", got)
	}
	if got := expectedPRDStoryCount(6, model.Anomaly{Metadata: map[string]string{"prd_story_count": "4"}}); got != 4 {
		t.Fatalf("expected metadata override story count 4, got %d", got)
	}
	if got := expectedPRDStoryCount(0, model.Anomaly{}); got != defaultPRDStoryCount {
		t.Fatalf("expected fallback story count %d, got %d", defaultPRDStoryCount, got)
	}
}

func TestReadPRDStoryCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prd.json")
	if err := os.WriteFile(path, []byte(`{"stories":[{"id":"US-1"},{"id":"US-2"}]}`), 0o644); err != nil {
		t.Fatalf("write prd file: %v", err)
	}
	count, err := readPRDStoryCount(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected story count 2, got %d", count)
	}
}

func TestReadPRDProgress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prd.json")
	if err := os.WriteFile(path, []byte(`{"stories":[{"id":"US-1","status":"open"},{"id":"US-2","status":"in_progress"},{"id":"US-3","status":"done"}]}`), 0o644); err != nil {
		t.Fatalf("write prd file: %v", err)
	}
	progress, err := readPRDProgress(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progress.Total != 3 || progress.Open != 1 || progress.InProgress != 1 || progress.Done != 1 {
		t.Fatalf("unexpected progress: %#v", progress)
	}
}

func TestMaterializePRDFromMetadata(t *testing.T) {
	dir := t.TempDir()
	prdPath := filepath.Join(dir, ".agents", "tasks", "prd.json")
	written, err := materializePRDFromMetadata(prdPath, map[string]string{
		"workspace_prd_json": `{"stories":[{"id":"US-001","status":"open"}]}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !written {
		t.Fatal("expected PRD materialization to write file")
	}
	content, err := os.ReadFile(prdPath)
	if err != nil {
		t.Fatalf("read prd file: %v", err)
	}
	if !strings.Contains(string(content), `"stories"`) {
		t.Fatalf("unexpected prd content: %s", string(content))
	}
}

func TestMaterializePRDFromMetadataRejectsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	prdPath := filepath.Join(dir, ".agents", "tasks", "prd.json")
	if _, err := materializePRDFromMetadata(prdPath, map[string]string{
		"workspace_prd_json": `{"stories":`,
	}); err == nil {
		t.Fatal("expected invalid json error")
	}
}

func TestResolveIssuePRDPath(t *testing.T) {
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, ".agents", "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	expectedPath := filepath.Join(tasksDir, "prd.json")

	if _, _, err := resolveIssuePRDPath(expectedPath); !os.IsNotExist(err) {
		t.Fatalf("expected not-exist before creating files, got %v", err)
	}

	altPath := filepath.Join(tasksDir, "prd-alpha.json")
	if err := os.WriteFile(altPath, []byte(`{"stories":[]}`), 0o644); err != nil {
		t.Fatalf("write alternate prd: %v", err)
	}
	resolved, source, err := resolveIssuePRDPath(expectedPath)
	if err != nil {
		t.Fatalf("resolve single prd failed: %v", err)
	}
	if resolved != altPath || source != "single_json_in_tasks" {
		t.Fatalf("unexpected resolution %q source %q", resolved, source)
	}

	secondPath := filepath.Join(tasksDir, "prd-beta.json")
	if err := os.WriteFile(secondPath, []byte(`{"stories":[]}`), 0o644); err != nil {
		t.Fatalf("write second prd: %v", err)
	}
	if _, _, err := resolveIssuePRDPath(expectedPath); err == nil || !strings.Contains(err.Error(), "multiple PRD files found") {
		t.Fatalf("expected multiple-files error, got %v", err)
	}
}

func TestBuildIssuePRDPromptIncludesStoryCount(t *testing.T) {
	prompt := buildIssuePRDPrompt(model.Anomaly{
		SourceType:    "github_issue",
		SourceRef:     "acme/repo#7",
		Title:         "Sample issue",
		Description:   "Sample description",
		CorrelationID: "corr-1",
		Metadata: map[string]string{
			"github_issue_context_json": `{"issue":{"number":7},"comments":[{"id":1}]}`,
		},
	}, "/workspace/.agents/tasks/prd.json", 9)
	if !strings.Contains(prompt, "Include exactly 9 user stories in the stories array.") {
		t.Fatalf("expected prompt to include story count requirement, got: %s", prompt)
	}
	if !strings.Contains(prompt, "GitHub Issue Full Context (JSON):") {
		t.Fatalf("expected full issue context in prompt, got: %s", prompt)
	}
}

func TestShouldRunIssueWorkflow(t *testing.T) {
	cfg := loopExecutionConfig{
		InvocationMethod:     "github_issue",
		IssueWorkflowEnabled: true,
	}
	if !shouldRunIssueWorkflow(cfg, model.Anomaly{}) {
		t.Fatal("expected github_issue invocation to run issue workflow")
	}
	cfg = loopExecutionConfig{
		InvocationMethod:     "manual",
		SourceType:           "manual",
		IssueWorkflowEnabled: true,
	}
	if shouldRunIssueWorkflow(cfg, model.Anomaly{}) {
		t.Fatal("did not expect manual invocation to run issue workflow")
	}
	cfg = loopExecutionConfig{
		InvocationMethod:     "manual",
		IssueWorkflowEnabled: true,
	}
	if !shouldRunIssueWorkflow(cfg, model.Anomaly{SourceType: "github_issue"}) {
		t.Fatal("expected github source type fallback to run issue workflow")
	}
	if !shouldRunIssueWorkflow(cfg, model.Anomaly{Metadata: map[string]string{"workspace_prompt": "add an export dashboard"}}) {
		t.Fatal("expected workspace prompt metadata to run issue workflow")
	}
	cfg = loopExecutionConfig{
		InvocationMethod:     "prompt",
		IssueWorkflowEnabled: true,
	}
	if !shouldRunIssueWorkflow(cfg, model.Anomaly{}) {
		t.Fatal("expected prompt invocation to run issue workflow")
	}
	cfg = loopExecutionConfig{
		InvocationMethod:     "github_issue",
		IssueWorkflowEnabled: false,
	}
	if shouldRunIssueWorkflow(cfg, model.Anomaly{}) {
		t.Fatal("did not expect disabled issue workflow to run")
	}
}

func TestMergeMetadata(t *testing.T) {
	merged := mergeMetadata(
		map[string]string{"a": "1", "b": "2"},
		map[string]string{"b": "override", "c": "3"},
	)
	if merged["a"] != "1" || merged["b"] != "override" || merged["c"] != "3" {
		t.Fatalf("unexpected merged metadata: %#v", merged)
	}
}
