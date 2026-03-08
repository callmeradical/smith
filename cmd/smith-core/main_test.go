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
