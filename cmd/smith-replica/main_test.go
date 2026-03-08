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
