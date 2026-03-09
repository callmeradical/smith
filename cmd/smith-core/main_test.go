package main

import (
	"context"
	"testing"
	"time"

	"smith/internal/source/gitpolicy"
	"smith/internal/source/journalpolicy"
	"smith/internal/source/model"
)

func TestResolveExecutionImageSelectionDefaults(t *testing.T) {
	cfg := config{replicaImage: "ghcr.io/smith/replica:v1", replicaPullPolicy: "IfNotPresent"}
	got, err := resolveExecutionImageSelection(context.Background(), nil, cfg, "loop-1")
	if err != nil {
		t.Fatalf("resolveExecutionImageSelection error: %v", err)
	}

	if got.Ref != "ghcr.io/smith/replica:v1" {
		t.Fatalf("ref: got %q", got.Ref)
	}
	if got.PullPolicy != "IfNotPresent" {
		t.Fatalf("pull policy: got %q", got.PullPolicy)
	}
	if got.Source != "core_default" {
		t.Fatalf("source: got %q", got.Source)
	}
	if got.Digest != "" {
		t.Fatalf("digest: got %q", got.Digest)
	}
}

func TestResolveExecutionImageSelectionFromLoopEnvironment(t *testing.T) {
	cfg := config{replicaImage: "ghcr.io/smith/replica:v1", replicaPullPolicy: "IfNotPresent"}
	anomaly := &model.Anomaly{
		Environment: model.LoopEnvironment{
			ContainerImage: &model.ContainerImageProfile{
				Ref:        "ghcr.io/custom/replica@sha256:deadbeef",
				PullPolicy: "Always",
			},
		},
	}
	got, err := resolveExecutionImageSelection(context.Background(), anomaly, cfg, "loop-1")
	if err != nil {
		t.Fatalf("resolveExecutionImageSelection error: %v", err)
	}

	if got.Ref != "ghcr.io/custom/replica@sha256:deadbeef" {
		t.Fatalf("ref: got %q", got.Ref)
	}
	if got.PullPolicy != "Always" {
		t.Fatalf("pull policy: got %q", got.PullPolicy)
	}
	if got.Source != "loop_environment_container_image" {
		t.Fatalf("source: got %q", got.Source)
	}
	if got.Digest != "sha256:deadbeef" {
		t.Fatalf("digest: got %q", got.Digest)
	}
}

func TestResolveExecutionImageSelectionOverrideUsesDefaultPullPolicyWhenUnset(t *testing.T) {
	cfg := config{replicaImage: "ghcr.io/smith/replica:v1", replicaPullPolicy: "IfNotPresent"}
	anomaly := &model.Anomaly{
		Environment: model.LoopEnvironment{
			ContainerImage: &model.ContainerImageProfile{Ref: "ghcr.io/custom/replica:v2"},
		},
	}
	got, err := resolveExecutionImageSelection(context.Background(), anomaly, cfg, "loop-1")
	if err != nil {
		t.Fatalf("resolveExecutionImageSelection error: %v", err)
	}
	if got.PullPolicy != "IfNotPresent" {
		t.Fatalf("pull policy: got %q", got.PullPolicy)
	}
}

func TestResolveExecutionImageSelectionDockerfileDisabled(t *testing.T) {
	cfg := config{
		replicaImage:      "ghcr.io/smith/replica:v1",
		replicaPullPolicy: "IfNotPresent",
		dockerfileBuild:   false,
	}
	anomaly := &model.Anomaly{
		Environment: model.LoopEnvironment{
			Dockerfile: &model.DockerfileProfile{
				ContextDir:     "workspace",
				DockerfilePath: "Dockerfile",
			},
		},
	}
	_, err := resolveExecutionImageSelection(context.Background(), anomaly, cfg, "loop-1")
	if err == nil {
		t.Fatal("expected dockerfile resolution error when build path disabled")
	}
}

func TestDockerfileBuildTagStable(t *testing.T) {
	profile := model.DockerfileProfile{
		ContextDir:     "workspace",
		DockerfilePath: "Dockerfile",
		BuildArgs: map[string]string{
			"GO_VERSION": "1.22",
			"TARGETOS":   "linux",
		},
	}
	a := dockerfileBuildTag("loop-abc", profile)
	b := dockerfileBuildTag("loop-abc", profile)
	if a != b {
		t.Fatalf("expected stable tag, got %q vs %q", a, b)
	}
}

func TestParseImageDigest(t *testing.T) {
	if got := parseImageDigest("ghcr.io/acme/replica:v1"); got != "" {
		t.Fatalf("expected empty digest, got %q", got)
	}
	if got := parseImageDigest("ghcr.io/acme/replica@sha256:abc123"); got != "sha256:abc123" {
		t.Fatalf("unexpected digest: %q", got)
	}
}

func TestGitContextForGitHubIssueFallback(t *testing.T) {
	anomaly := model.Anomaly{
		SourceType: "github_issue",
		SourceRef:  "acme/smith#42",
	}
	got := gitContextFor(anomaly)
	if got.Repository != "acme/smith" {
		t.Fatalf("repo: got %q", got.Repository)
	}
	if got.Branch != "main" {
		t.Fatalf("branch: got %q", got.Branch)
	}
	if got.CommitSHA != "unknown" {
		t.Fatalf("commit: got %q", got.CommitSHA)
	}
}

func TestHandoffConfigMapNameSanitizesAndBounds(t *testing.T) {
	got := handoffConfigMapName("LOOP/With Spaces.and_extra_chars___abcdefghijklmnopqrstuvwxyz")
	if got == "" {
		t.Fatal("expected non-empty configmap name")
	}
	if len(got) > 48 {
		t.Fatalf("expected bounded configmap name, got len=%d (%q)", len(got), got)
	}
	if got[:8] != "handoff-" {
		t.Fatalf("expected handoff prefix, got %q", got)
	}
}

func TestResolveSkillMountsFromAnomaly(t *testing.T) {
	readOnly := true
	anomaly := &model.Anomaly{
		ID: "loop-1",
		Skills: []model.LoopSkillMount{
			{
				Name:      "commit",
				Source:    "local://skills/commit",
				MountPath: "/smith/skills/commit",
				ReadOnly:  &readOnly,
			},
		},
	}
	mounts, names, err := resolveSkillMounts(anomaly)
	if err != nil {
		t.Fatalf("resolveSkillMounts error: %v", err)
	}
	if len(mounts) != 1 || len(names) != 1 {
		t.Fatalf("expected one skill mount, got mounts=%d names=%d", len(mounts), len(names))
	}
	if mounts[0].Source != "local://skills/commit" || mounts[0].MountPath != "/smith/skills/commit" || !mounts[0].ReadOnly {
		t.Fatalf("unexpected mount %+v", mounts[0])
	}
}

func TestSkillSourceConfigMapName(t *testing.T) {
	if got := skillSourceConfigMapName("local://skills/commit"); got != "skill-commit" {
		t.Fatalf("unexpected configmap name %q", got)
	}
	if got := skillSourceConfigMapName("http://example.com/skill"); got != "" {
		t.Fatalf("expected unsupported source to return empty name, got %q", got)
	}
}

func TestLoopInvocationMethodFor(t *testing.T) {
	if got := loopInvocationMethodFor(model.Anomaly{
		SourceType: "manual",
		Metadata: map[string]string{
			"invocation_method": "console_issue",
			"ingress_mode":      "prd",
		},
	}); got != "console_issue" {
		t.Fatalf("expected invocation_method override, got %q", got)
	}

	if got := loopInvocationMethodFor(model.Anomaly{
		SourceType: "github_issue",
		Metadata: map[string]string{
			"ingress_mode": "prd",
		},
	}); got != "prd" {
		t.Fatalf("expected ingress_mode fallback, got %q", got)
	}

	if got := loopInvocationMethodFor(model.Anomaly{SourceType: "github_issue"}); got != "github_issue" {
		t.Fatalf("expected source type fallback, got %q", got)
	}

	if got := loopInvocationMethodFor(model.Anomaly{}); got != "unknown" {
		t.Fatalf("expected unknown fallback, got %q", got)
	}
}

func TestLoopProviderFor(t *testing.T) {
	if got := loopProviderFor(model.Anomaly{
		ProviderID: "codex",
		Metadata: map[string]string{
			"workspace_provider": "claude",
			"workspace_agent":    "droid",
		},
	}); got != "codex" {
		t.Fatalf("expected provider_id precedence, got %q", got)
	}

	if got := loopProviderFor(model.Anomaly{
		Metadata: map[string]string{
			"workspace_provider": "claude",
			"workspace_agent":    "droid",
		},
	}); got != "claude" {
		t.Fatalf("expected workspace_provider fallback, got %q", got)
	}

	if got := loopProviderFor(model.Anomaly{
		Metadata: map[string]string{
			"workspace_agent": "droid",
		},
	}); got != "droid" {
		t.Fatalf("expected workspace_agent fallback, got %q", got)
	}

	if got := loopProviderFor(model.Anomaly{}); got != model.DefaultProviderID {
		t.Fatalf("expected default provider %q, got %q", model.DefaultProviderID, got)
	}
}

func TestLoadConfigGitPolicyDefaults(t *testing.T) {
	t.Setenv("SMITH_GIT_POLICY_CONFIG_ENABLED", "")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.gitPolicyConfig {
		t.Fatal("expected git policy config flag to default false")
	}
	def := gitpolicy.DefaultPolicy()
	if cfg.gitPolicy.BranchCleanup != def.BranchCleanup {
		t.Fatalf("expected default branch cleanup %q got %q", def.BranchCleanup, cfg.gitPolicy.BranchCleanup)
	}
	if cfg.gitPolicy.ConflictPolicy != def.ConflictPolicy {
		t.Fatalf("expected default conflict policy %q got %q", def.ConflictPolicy, cfg.gitPolicy.ConflictPolicy)
	}
}

func TestLoadConfigGitPolicyOverrides(t *testing.T) {
	t.Setenv("SMITH_GIT_POLICY_CONFIG_ENABLED", "true")
	t.Setenv("SMITH_GIT_POLICY_BRANCH_CLEANUP", "never")
	t.Setenv("SMITH_GIT_POLICY_CONFLICT_POLICY", "fail_fast")
	t.Setenv("SMITH_GIT_POLICY_DELETE_BRANCH_ON_MERGE", "false")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if !cfg.gitPolicyConfig {
		t.Fatal("expected git policy config flag true")
	}
	if cfg.gitPolicy.BranchCleanup != gitpolicy.BranchCleanupNever {
		t.Fatalf("unexpected branch cleanup %q", cfg.gitPolicy.BranchCleanup)
	}
	if cfg.gitPolicy.ConflictPolicy != gitpolicy.ConflictPolicyFailFast {
		t.Fatalf("unexpected conflict policy %q", cfg.gitPolicy.ConflictPolicy)
	}
	if cfg.gitPolicy.DeleteBranchOnMerge {
		t.Fatal("expected delete branch on merge false")
	}
}

func TestLoadConfigRejectsInvalidGitPolicyOverrides(t *testing.T) {
	t.Setenv("SMITH_GIT_POLICY_CONFIG_ENABLED", "true")
	t.Setenv("SMITH_GIT_POLICY_BRANCH_CLEANUP", "on_merge")
	t.Setenv("SMITH_GIT_POLICY_DELETE_BRANCH_ON_MERGE", "false")
	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected invalid git policy config error")
	}
}

func TestLoadConfigJournalPolicyDefaults(t *testing.T) {
	t.Setenv("SMITH_JOURNAL_POLICY_CONFIG_ENABLED", "")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.journalPolicyConfig {
		t.Fatal("expected journal policy config flag to default false")
	}
	def := journalpolicy.DefaultPolicy()
	if cfg.journalPolicy.RetentionMode != def.RetentionMode {
		t.Fatalf("expected default retention mode %q got %q", def.RetentionMode, cfg.journalPolicy.RetentionMode)
	}
	if cfg.journalPolicy.ArchiveMode != def.ArchiveMode {
		t.Fatalf("expected default archive mode %q got %q", def.ArchiveMode, cfg.journalPolicy.ArchiveMode)
	}
}

func TestLoadConfigJournalPolicyOverrides(t *testing.T) {
	t.Setenv("SMITH_JOURNAL_POLICY_CONFIG_ENABLED", "true")
	t.Setenv("SMITH_JOURNAL_RETENTION_MODE", "ttl")
	t.Setenv("SMITH_JOURNAL_RETENTION_TTL", "168h")
	t.Setenv("SMITH_JOURNAL_ARCHIVE_MODE", "s3")
	t.Setenv("SMITH_JOURNAL_ARCHIVE_BUCKET", "smith-journal-archive")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if !cfg.journalPolicyConfig {
		t.Fatal("expected journal policy config flag true")
	}
	if cfg.journalPolicy.RetentionMode != journalpolicy.RetentionTTL {
		t.Fatalf("unexpected retention mode %q", cfg.journalPolicy.RetentionMode)
	}
	if cfg.journalPolicy.RetentionTTL != 168*time.Hour {
		t.Fatalf("unexpected retention ttl %s", cfg.journalPolicy.RetentionTTL)
	}
	if cfg.journalPolicy.ArchiveMode != journalpolicy.ArchiveS3 {
		t.Fatalf("unexpected archive mode %q", cfg.journalPolicy.ArchiveMode)
	}
	if cfg.journalPolicy.ArchiveBucket != "smith-journal-archive" {
		t.Fatalf("unexpected archive bucket %q", cfg.journalPolicy.ArchiveBucket)
	}
}

func TestLoadConfigRejectsInvalidJournalPolicyOverrides(t *testing.T) {
	t.Setenv("SMITH_JOURNAL_POLICY_CONFIG_ENABLED", "true")
	t.Setenv("SMITH_JOURNAL_RETENTION_MODE", "ttl")
	t.Setenv("SMITH_JOURNAL_RETENTION_TTL", "")
	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected invalid journal policy config error")
	}
}
