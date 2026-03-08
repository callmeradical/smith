package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"smith/internal/source/model"
)

type execRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

type commandRunner struct{}

var lookPath = exec.LookPath

func (commandRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func setupLoopEnvironment(ctx context.Context, env model.LoopEnvironment, workspace string, runner execRunner) (map[string]string, error) {
	if env.ResolvedMode != model.EnvironmentModeMise {
		return map[string]string{
			"environment_mode": env.ResolvedMode,
		}, nil
	}
	tools, sourceFile, err := resolveMiseTools(env, workspace)
	if err != nil {
		return nil, err
	}
	if len(tools) == 0 {
		return nil, errors.New("mise profile resolved with zero tools; define environment.mise.tools or provide a non-empty tool_versions_file")
	}

	if _, err := lookPath("mise"); err != nil {
		return nil, errors.New("mise binary not found in PATH; remediation: install mise in replica image or use a non-mise loop environment")
	}

	pairs := make([]string, 0, len(tools))
	orderedTools := sortedToolNames(tools)
	for _, tool := range orderedTools {
		version := tools[tool]
		pairs = append(pairs, tool+"="+version)
		output, runErr := runner.Run(ctx, workspace, "mise", "install", tool+"@"+version)
		if runErr != nil {
			return nil, fmt.Errorf(
				"mise install failed for %s@%s: %v. remediation: verify requested versions exist and network access allows tool download. output: %s",
				tool, version, runErr, strings.TrimSpace(string(output)),
			)
		}
	}
	meta := map[string]string{
		"environment_mode": "mise",
		"mise_tool_count":  strconv.Itoa(len(tools)),
		"mise_tools":       strings.Join(pairs, ","),
	}
	if sourceFile != "" {
		meta["mise_tool_versions_file"] = sourceFile
	}
	return meta, nil
}

func resolveMiseTools(env model.LoopEnvironment, workspace string) (map[string]string, string, error) {
	if env.Mise == nil {
		return nil, "", errors.New("environment mode is mise but environment.mise is missing")
	}
	tools := map[string]string{}
	sourceFile := strings.TrimSpace(env.Mise.ToolVersionsFile)
	if sourceFile != "" {
		path := sourceFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(workspace, path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("read mise tool_versions_file %q failed: %w", sourceFile, err)
		}
		fileTools, err := parseToolVersionsFile(string(content))
		if err != nil {
			return nil, "", fmt.Errorf("parse mise tool_versions_file %q failed: %w", sourceFile, err)
		}
		for name, version := range fileTools {
			tools[name] = version
		}
	}
	for name, version := range env.Mise.Tools {
		trimmedName := strings.TrimSpace(name)
		trimmedVersion := strings.TrimSpace(version)
		if trimmedName == "" || trimmedVersion == "" {
			return nil, "", errors.New("environment.mise.tools must only contain non-empty name/version values")
		}
		tools[trimmedName] = trimmedVersion
	}
	return tools, sourceFile, nil
}

func parseToolVersionsFile(raw string) (map[string]string, error) {
	tools := map[string]string{}
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.Fields(trimmed)
		if len(parts) < 2 {
			return nil, fmt.Errorf("line %d must contain \"tool version\"", i+1)
		}
		tool := strings.TrimSpace(parts[0])
		version := strings.TrimSpace(parts[1])
		if tool == "" || version == "" {
			return nil, fmt.Errorf("line %d has empty tool/version", i+1)
		}
		tools[tool] = version
	}
	return tools, nil
}

func sortedToolNames(tools map[string]string) []string {
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
