package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecutionImageMetadataFromEnv(t *testing.T) {
	t.Setenv("SMITH_EXECUTION_IMAGE_REF", "ghcr.io/acme/replica@sha256:abc")
	t.Setenv("SMITH_EXECUTION_IMAGE_SOURCE", "loop_environment_container_image")
	t.Setenv("SMITH_EXECUTION_IMAGE_DIGEST", "sha256:abc")
	t.Setenv("SMITH_EXECUTION_IMAGE_PULL_POLICY", "Always")

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
}

func TestExecutionImageMetadataFromEnvEmpty(t *testing.T) {
	t.Setenv("SMITH_EXECUTION_IMAGE_REF", "")
	t.Setenv("SMITH_EXECUTION_IMAGE_SOURCE", "")
	t.Setenv("SMITH_EXECUTION_IMAGE_DIGEST", "")
	t.Setenv("SMITH_EXECUTION_IMAGE_PULL_POLICY", "")

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
