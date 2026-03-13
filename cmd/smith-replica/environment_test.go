package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"smith/internal/source/model"
)

func TestResolveMiseToolsFromFileAndInlineOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".tool-versions")
	content := "go 1.24.0\nnode 22.0.0\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write tool versions: %v", err)
	}
	env := model.LoopEnvironment{
		ResolvedMode: model.EnvironmentModeMise,
		Mise: &model.MiseEnvironment{
			ToolVersionsFile: ".tool-versions",
			Tools: map[string]string{
				"go": "1.25.0",
			},
		},
	}
	tools, source, err := resolveMiseTools(env, dir)
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	if source != ".tool-versions" {
		t.Fatalf("unexpected source: %s", source)
	}
	if tools["go"] != "1.25.0" {
		t.Fatalf("expected inline tool override, got %+v", tools)
	}
	if tools["node"] != "22.0.0" {
		t.Fatalf("expected node from file, got %+v", tools)
	}
}

func TestParseToolVersionsRejectsInvalidLine(t *testing.T) {
	_, err := parseToolVersionsFile("go\n")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestSetupLoopEnvironmentInstallsToolsDeterministically(t *testing.T) {
	runner := &fakeRunner{}
	env := model.LoopEnvironment{
		ResolvedMode: model.EnvironmentModeMise,
		Mise: &model.MiseEnvironment{
			Tools: map[string]string{
				"node": "22.0.0",
				"go":   "1.24.0",
			},
		},
	}

	originalLookPath := lookPath
	t.Cleanup(func() { lookPath = originalLookPath })
	lookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	meta, err := setupLoopEnvironment(context.Background(), env, t.TempDir(), runner)
	if err != nil {
		t.Fatalf("setup loop environment: %v", err)
	}
	if meta["mise_tool_count"] != "2" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 install calls, got %d", len(runner.calls))
	}
	if runner.calls[0] != "mise install go@1.24.0" || runner.calls[1] != "mise install node@22.0.0" {
		t.Fatalf("unexpected command order: %+v", runner.calls)
	}
}

func TestSetupLoopEnvironmentSurfacesRemediation(t *testing.T) {
	runner := &fakeRunner{
		err:    errors.New("exit status 1"),
		output: []byte("tool missing"),
	}
	env := model.LoopEnvironment{
		ResolvedMode: model.EnvironmentModeMise,
		Mise: &model.MiseEnvironment{
			Tools: map[string]string{
				"go": "0.0.0-bad",
			},
		},
	}

	originalLookPath := lookPath
	t.Cleanup(func() { lookPath = originalLookPath })
	lookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	_, err := setupLoopEnvironment(context.Background(), env, t.TempDir(), runner)
	if err == nil {
		t.Fatal("expected setup error")
	}
	if !strings.Contains(err.Error(), "remediation") {
		t.Fatalf("expected remediation guidance, got %v", err)
	}
}

func TestSetupLoopEnvironmentMissingMiseBinary(t *testing.T) {
	runner := &fakeRunner{}
	env := model.LoopEnvironment{
		ResolvedMode: model.EnvironmentModeMise,
		Mise: &model.MiseEnvironment{
			Tools: map[string]string{"go": "1.24.0"},
		},
	}
	originalLookPath := lookPath
	t.Cleanup(func() { lookPath = originalLookPath })
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	_, err := setupLoopEnvironment(context.Background(), env, t.TempDir(), runner)
	if err == nil {
		t.Fatal("expected missing binary error")
	}
	if !strings.Contains(err.Error(), "install mise") {
		t.Fatalf("expected remediation hint for install mise, got %v", err)
	}
}

type fakeRunner struct {
	calls  []string
	output []byte
	err    error
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, strings.Join(append([]string{name}, args...), " "))
	return f.output, f.err
}
