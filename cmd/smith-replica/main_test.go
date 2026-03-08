package main

import "testing"

func TestExecutionImageMetadataFromEnv(t *testing.T) {
	t.Setenv("SMITH_EXECUTION_IMAGE_REF", "ghcr.io/acme/replica@sha256:abc")
	t.Setenv("SMITH_EXECUTION_IMAGE_SOURCE", "loop_environment_container_image")
	t.Setenv("SMITH_EXECUTION_IMAGE_DIGEST", "sha256:abc")
	t.Setenv("SMITH_EXECUTION_IMAGE_PULL_POLICY", "Always")

	metadata := executionImageMetadataFromEnv()
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
}

func TestExecutionImageMetadataFromEnvEmpty(t *testing.T) {
	t.Setenv("SMITH_EXECUTION_IMAGE_REF", "")
	t.Setenv("SMITH_EXECUTION_IMAGE_SOURCE", "")
	t.Setenv("SMITH_EXECUTION_IMAGE_DIGEST", "")
	t.Setenv("SMITH_EXECUTION_IMAGE_PULL_POLICY", "")

	if metadata := executionImageMetadataFromEnv(); metadata != nil {
		t.Fatalf("expected nil metadata, got %#v", metadata)
	}
}
