package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPRDModeBuildsPromptAndWritesOutput(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, ".agents", "tasks", "prd.json")
	capturePath := filepath.Join(dir, "prompt.txt")
	agentPath := filepath.Join(dir, "fake-agent.sh")

	t.Setenv("SMITH_TEST_OUT", outPath)
	t.Setenv("SMITH_TEST_CAPTURE", capturePath)

	script := strings.Join([]string{
		"#!/bin/sh",
		"set -eu",
		"cat > \"$SMITH_TEST_CAPTURE\"",
		"mkdir -p \"$(dirname \"$SMITH_TEST_OUT\")\"",
		"printf '%s\\n' '{\"stories\":[{\"id\":\"US-001\",\"status\":\"open\"}]}' > \"$SMITH_TEST_OUT\"",
	}, "\n")
	if err := os.WriteFile(agentPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agent: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--prd",
		"Add issue-driven PRD generation",
		"--out", outPath,
		"--stories", "7",
		"--agent-cmd", agentPath,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}

	promptBytes, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatalf("read captured prompt: %v", err)
	}
	prompt := string(promptBytes)
	if !strings.Contains(prompt, "Use the $prd skill") {
		t.Fatalf("expected prd skill instruction in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "Include exactly 7 user stories") {
		t.Fatalf("expected story count instruction in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "Add issue-driven PRD generation") {
		t.Fatalf("expected request in prompt, got: %s", prompt)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected output prd file: %v", err)
	}
	if !strings.Contains(stdout.String(), "PRD JSON saved to") {
		t.Fatalf("expected success output path, got stdout=%s", stdout.String())
	}
}

func TestRunPromptModeUsesPromptFile(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, ".agents", "tasks", "prd.json")
	capturePath := filepath.Join(dir, "prompt.txt")
	promptPath := filepath.Join(dir, "input-prompt.md")
	agentPath := filepath.Join(dir, "fake-agent.sh")

	t.Setenv("SMITH_TEST_OUT", outPath)
	t.Setenv("SMITH_TEST_CAPTURE", capturePath)

	if err := os.WriteFile(promptPath, []byte("custom prompt body\n"), 0o644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}
	script := strings.Join([]string{
		"#!/bin/sh",
		"set -eu",
		"cat \"$1\" > \"$SMITH_TEST_CAPTURE\"",
		"mkdir -p \"$(dirname \"$SMITH_TEST_OUT\")\"",
		"printf '%s\\n' '{\"stories\":[]}' > \"$SMITH_TEST_OUT\"",
	}, "\n")
	if err := os.WriteFile(agentPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agent: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--prompt", promptPath,
		"--out", outPath,
		"--agent-cmd", agentPath + " {prompt}",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}

	promptBytes, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatalf("read captured prompt: %v", err)
	}
	if string(promptBytes) != "custom prompt body\n" {
		t.Fatalf("unexpected prompt content: %q", string(promptBytes))
	}
}

func TestRunImportMarkdownUsesDefaultOutputPath(t *testing.T) {
	dir := t.TempDir()
	markdownPath := filepath.Join(dir, "feature.md")
	markdown := strings.Join([]string{
		"# Smith PRD Validation",
		"",
		"## Overview",
		"",
		"Normalize markdown PRDs into canonical JSON for downstream workflows.",
		"",
		"## Quality Gates",
		"- go test ./...",
		"",
		"## Stories",
		"",
		"### US-001: Define validation contract",
		"",
		"As a maintainer, I want shared validation.",
		"",
		"#### Acceptance Criteria",
		"- Validation returns diagnostics.",
	}, "\n")
	if err := os.WriteFile(markdownPath, []byte(markdown), 0o644); err != nil {
		t.Fatalf("write markdown fixture: %v", err)
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})

	var stdout, stderr bytes.Buffer
	code := run([]string{"--prd", "--from-markdown", markdownPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}

	outPath := filepath.Join(dir, ".agents", "tasks", "prd.json")
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected default output prd file: %v", err)
	}
	if !strings.Contains(stdout.String(), "PRD JSON saved to") || !strings.Contains(stdout.String(), outPath) {
		t.Fatalf("expected success output path, got stdout=%s", stdout.String())
	}
}

func TestRunExportMarkdownSuccess(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "prd.json")
	markdownPath := filepath.Join(dir, "prd.md")
	prd := `{
  "version": 1,
  "project": "Smith PRD Validation",
  "overview": "Canonical PRD validation",
  "qualityGates": ["go test ./..."],
  "stories": [
    {
      "id": "US-001",
      "title": "Define validation contract",
      "status": "open",
      "description": "As a maintainer, I want shared validation.",
      "acceptanceCriteria": ["Validation report is shared."]
    }
  ]
}`
	if err := os.WriteFile(jsonPath, []byte(prd), 0o644); err != nil {
		t.Fatalf("write prd json: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--prd", "--from-json", jsonPath, "--to-markdown", markdownPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}

	rendered, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read markdown output: %v", err)
	}
	if !strings.Contains(string(rendered), "# Smith PRD Validation") {
		t.Fatalf("expected rendered markdown, got %s", string(rendered))
	}
	if !strings.Contains(stdout.String(), "PRD markdown saved to") || !strings.Contains(stdout.String(), markdownPath) {
		t.Fatalf("expected success output path, got stdout=%s", stdout.String())
	}
}

func TestRunValidateInvalidPRDReturnsDiagnostics(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "prd.json")
	prd := `{
  "version": 1,
  "project": "Smith PRD Validation",
  "overview": "Canonical PRD validation",
  "qualityGates": [],
  "stories": []
}`
	if err := os.WriteFile(jsonPath, []byte(prd), 0o644); err != nil {
		t.Fatalf("write prd json: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--prd", "validate", jsonPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected validation failure code=1, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}

	var report struct {
		Valid  bool `json:"valid"`
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("expected machine-readable JSON diagnostics: %v\nstdout=%s", err, stdout.String())
	}
	if report.Valid {
		t.Fatalf("expected invalid report, got %+v", report)
	}
	if len(report.Errors) == 0 {
		t.Fatalf("expected diagnostics, got %+v", report)
	}
}

func TestRunValidateWarningOnlyPRDReturnsZero(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "prd.json")
	prd := `{
  "version": 1,
  "project": "Smith PRD Validation",
  "overview": "Canonical PRD validation",
  "qualityGates": ["go test ./..."],
  "stories": [
    {
      "id": "US-001",
      "title": "Clarify readiness warnings",
      "status": "open",
      "description": "As a maintainer, I want warning-only lint results preserved.",
      "acceptanceCriteria": ["UI works as expected."]
    }
  ]
}`
	if err := os.WriteFile(jsonPath, []byte(prd), 0o644); err != nil {
		t.Fatalf("write prd json: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--prd", "validate", jsonPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected validation warning code=0, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}

	var report struct {
		Valid     bool   `json:"valid"`
		Readiness string `json:"readiness"`
		Warnings  []struct {
			Code string `json:"code"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("expected machine-readable JSON diagnostics: %v\nstdout=%s", err, stdout.String())
	}
	if !report.Valid {
		t.Fatalf("expected warning-only report to be valid, got %+v", report)
	}
	if report.Readiness != "warn" {
		t.Fatalf("expected readiness warn, got %+v", report)
	}
	if len(report.Warnings) == 0 {
		t.Fatalf("expected warning diagnostics, got %+v", report)
	}
}

func TestRunRejectsInvalidExportArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--prd", "--to-markdown", "prd.md"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code=2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--to-markdown requires --from-json") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunImportRequiresPRDModeFlag(t *testing.T) {
	dir := t.TempDir()
	markdownPath := filepath.Join(dir, "feature.md")
	if err := os.WriteFile(markdownPath, []byte("# Example\n"), 0o644); err != nil {
		t.Fatalf("write markdown fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--from-markdown", markdownPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code=2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "supports PRD mode only") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunValidateRequiresPRDModeFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"validate"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code=2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "supports PRD mode only") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunRequiresPRDModeFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"just", "text"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code=2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "supports PRD mode only") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}
