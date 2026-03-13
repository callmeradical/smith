package model

import (
	"fmt"
	"sort"
	"strings"
)

const (
	EnvironmentModePreset         = "preset"
	EnvironmentModeMise           = "mise"
	EnvironmentModeContainerImage = "container_image"
	EnvironmentModeDockerfile     = "dockerfile"

	DefaultEnvironmentPreset = "standard"
)

var allowedEnvironmentPresets = map[string]struct{}{
	"standard":    {},
	"secure":      {},
	"performance": {},
	"minimal":     {},
}

type EnvironmentPolicy struct {
	DefaultPreset  string
	AllowedPresets map[string]struct{}
}

type LoopEnvironment struct {
	Preset         string                 `json:"preset,omitempty"`
	Mise           *MiseEnvironment       `json:"mise,omitempty"`
	ContainerImage *ContainerImageProfile `json:"container_image,omitempty"`
	Dockerfile     *DockerfileProfile     `json:"dockerfile,omitempty"`
	Env            map[string]string      `json:"env,omitempty"`
	ResolvedMode   string                 `json:"resolved_mode,omitempty"`
}

type MiseEnvironment struct {
	ToolVersionsFile string            `json:"tool_versions_file,omitempty"`
	Tools            map[string]string `json:"tools,omitempty"`
}

type ContainerImageProfile struct {
	Ref        string `json:"ref"`
	PullPolicy string `json:"pull_policy,omitempty"`
}

type DockerfileProfile struct {
	ContextDir     string            `json:"context_dir,omitempty"`
	DockerfilePath string            `json:"dockerfile_path,omitempty"`
	Target         string            `json:"target,omitempty"`
	BuildArgs      map[string]string `json:"build_args,omitempty"`
}

func NormalizeLoopEnvironment(in *LoopEnvironment) (LoopEnvironment, error) {
	return NormalizeLoopEnvironmentWithPolicy(in, DefaultEnvironmentPolicy())
}

func DefaultEnvironmentPolicy() EnvironmentPolicy {
	return EnvironmentPolicy{
		DefaultPreset:  DefaultEnvironmentPreset,
		AllowedPresets: clonePresetMap(allowedEnvironmentPresets),
	}
}

func NormalizeLoopEnvironmentWithPolicy(in *LoopEnvironment, policy EnvironmentPolicy) (LoopEnvironment, error) {
	defaultPreset := strings.ToLower(strings.TrimSpace(policy.DefaultPreset))
	if defaultPreset == "" {
		defaultPreset = DefaultEnvironmentPreset
	}
	allowedPresets := clonePresetMap(policy.AllowedPresets)
	if len(allowedPresets) == 0 {
		allowedPresets = clonePresetMap(allowedEnvironmentPresets)
	}
	if _, ok := allowedPresets[defaultPreset]; !ok {
		return LoopEnvironment{}, fmt.Errorf("invalid environment default preset %q (allowed: %s)", defaultPreset, strings.Join(sortedKeys(allowedPresets), ", "))
	}
	if in == nil {
		return LoopEnvironment{Preset: defaultPreset, ResolvedMode: EnvironmentModePreset}, nil
	}
	env := LoopEnvironment{
		Preset:         strings.ToLower(strings.TrimSpace(in.Preset)),
		Mise:           cloneMise(in.Mise),
		ContainerImage: cloneContainerImage(in.ContainerImage),
		Dockerfile:     cloneDockerfile(in.Dockerfile),
		Env:            cloneMap(in.Env),
	}
	if env.Preset == "" {
		env.Preset = defaultPreset
	}
	if _, ok := allowedPresets[env.Preset]; !ok {
		return LoopEnvironment{}, fmt.Errorf("invalid environment preset %q (allowed: %s)", env.Preset, strings.Join(sortedKeys(allowedPresets), ", "))
	}
	for key := range env.Env {
		if strings.TrimSpace(key) == "" {
			return LoopEnvironment{}, fmt.Errorf("environment env keys must be non-empty")
		}
	}

	mode := EnvironmentModePreset
	sourceCount := 0
	if env.Mise != nil {
		sourceCount++
		mode = EnvironmentModeMise
		if err := validateMise(*env.Mise); err != nil {
			return LoopEnvironment{}, err
		}
	}
	if env.ContainerImage != nil {
		sourceCount++
		mode = EnvironmentModeContainerImage
		if err := validateContainerImage(*env.ContainerImage); err != nil {
			return LoopEnvironment{}, err
		}
	}
	if env.Dockerfile != nil {
		sourceCount++
		mode = EnvironmentModeDockerfile
		if err := validateDockerfile(*env.Dockerfile); err != nil {
			return LoopEnvironment{}, err
		}
	}

	if sourceCount > 1 {
		return LoopEnvironment{}, fmt.Errorf("environment source conflict: specify only one of mise, container_image, or dockerfile (precedence is dockerfile > container_image > mise > preset)")
	}
	env.ResolvedMode = mode
	return env, nil
}

func clonePresetMap(in map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for k := range in {
		out[k] = struct{}{}
	}
	return out
}

func validateMise(mise MiseEnvironment) error {
	file := strings.TrimSpace(mise.ToolVersionsFile)
	if file == "" && len(mise.Tools) == 0 {
		return fmt.Errorf("environment.mise requires tool_versions_file or tools")
	}
	for name := range mise.Tools {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("environment.mise.tools keys must be non-empty")
		}
	}
	return nil
}

func validateContainerImage(image ContainerImageProfile) error {
	image.Ref = strings.TrimSpace(image.Ref)
	if image.Ref == "" {
		return fmt.Errorf("environment.container_image.ref is required")
	}
	policy := strings.TrimSpace(image.PullPolicy)
	if policy == "" {
		return nil
	}
	switch policy {
	case "Always", "IfNotPresent", "Never":
		return nil
	default:
		return fmt.Errorf("environment.container_image.pull_policy must be one of Always, IfNotPresent, Never")
	}
}

func validateDockerfile(profile DockerfileProfile) error {
	if strings.TrimSpace(profile.ContextDir) == "" {
		return fmt.Errorf("environment.dockerfile.context_dir is required")
	}
	if strings.TrimSpace(profile.DockerfilePath) == "" {
		return fmt.Errorf("environment.dockerfile.dockerfile_path is required")
	}
	for key := range profile.BuildArgs {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("environment.dockerfile.build_args keys must be non-empty")
		}
	}
	return nil
}

func cloneMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneMise(in *MiseEnvironment) *MiseEnvironment {
	if in == nil {
		return nil
	}
	return &MiseEnvironment{
		ToolVersionsFile: strings.TrimSpace(in.ToolVersionsFile),
		Tools:            cloneMap(in.Tools),
	}
}

func cloneContainerImage(in *ContainerImageProfile) *ContainerImageProfile {
	if in == nil {
		return nil
	}
	out := &ContainerImageProfile{
		Ref:        strings.TrimSpace(in.Ref),
		PullPolicy: strings.TrimSpace(in.PullPolicy),
	}
	if out.PullPolicy == "" {
		out.PullPolicy = "IfNotPresent"
	}
	return out
}

func cloneDockerfile(in *DockerfileProfile) *DockerfileProfile {
	if in == nil {
		return nil
	}
	return &DockerfileProfile{
		ContextDir:     strings.TrimSpace(in.ContextDir),
		DockerfilePath: strings.TrimSpace(in.DockerfilePath),
		Target:         strings.TrimSpace(in.Target),
		BuildArgs:      cloneMap(in.BuildArgs),
	}
}

func sortedKeys(in map[string]struct{}) []string {
	out := make([]string, 0, len(in))
	for k := range in {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
