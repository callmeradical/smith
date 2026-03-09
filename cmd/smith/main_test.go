package main

import (
	"bytes"
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
		"--prd", "Add issue-driven PRD generation",
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
