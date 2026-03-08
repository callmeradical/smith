package main

import (
	"testing"

	"smith/internal/source/model"
)

func TestResolveExecutionImageSelectionDefaults(t *testing.T) {
	cfg := config{replicaImage: "ghcr.io/smith/replica:v1", replicaPullPolicy: "IfNotPresent"}
	got := resolveExecutionImageSelection(nil, cfg)

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
	got := resolveExecutionImageSelection(anomaly, cfg)

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
	got := resolveExecutionImageSelection(anomaly, cfg)
	if got.PullPolicy != "IfNotPresent" {
		t.Fatalf("pull policy: got %q", got.PullPolicy)
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
