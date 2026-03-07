package model

import "testing"

func TestNormalizeLoopEnvironmentDefaults(t *testing.T) {
	env, err := NormalizeLoopEnvironment(nil)
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if env.Preset != DefaultEnvironmentPreset {
		t.Fatalf("unexpected preset: %s", env.Preset)
	}
	if env.ResolvedMode != EnvironmentModePreset {
		t.Fatalf("unexpected mode: %s", env.ResolvedMode)
	}
}

func TestNormalizeLoopEnvironmentContainerImage(t *testing.T) {
	env, err := NormalizeLoopEnvironment(&LoopEnvironment{ContainerImage: &ContainerImageProfile{Ref: "ghcr.io/acme/smith-replica:v1"}})
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if env.ResolvedMode != EnvironmentModeContainerImage {
		t.Fatalf("unexpected mode: %s", env.ResolvedMode)
	}
	if env.ContainerImage == nil || env.ContainerImage.PullPolicy != "IfNotPresent" {
		t.Fatalf("expected default pull policy")
	}
}

func TestNormalizeLoopEnvironmentRejectsConflicts(t *testing.T) {
	_, err := NormalizeLoopEnvironment(&LoopEnvironment{
		Mise:           &MiseEnvironment{ToolVersionsFile: ".tool-versions"},
		ContainerImage: &ContainerImageProfile{Ref: "ghcr.io/acme/smith:v1"},
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestNormalizeLoopEnvironmentRejectsInvalidDockerfile(t *testing.T) {
	_, err := NormalizeLoopEnvironment(&LoopEnvironment{Dockerfile: &DockerfileProfile{ContextDir: "."}})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestNormalizeLoopEnvironmentWithPolicyDefaultPreset(t *testing.T) {
	policy := EnvironmentPolicy{
		DefaultPreset: "team-default",
		AllowedPresets: map[string]struct{}{
			"team-default": {},
			"standard":     {},
		},
	}
	env, err := NormalizeLoopEnvironmentWithPolicy(nil, policy)
	if err != nil {
		t.Fatalf("normalize with policy: %v", err)
	}
	if env.Preset != "team-default" {
		t.Fatalf("expected default preset team-default, got %s", env.Preset)
	}
}

func TestNormalizeLoopEnvironmentWithPolicyRejectsUnknownPreset(t *testing.T) {
	policy := EnvironmentPolicy{
		DefaultPreset: "team-default",
		AllowedPresets: map[string]struct{}{
			"team-default": {},
		},
	}
	_, err := NormalizeLoopEnvironmentWithPolicy(&LoopEnvironment{Preset: "standard"}, policy)
	if err == nil {
		t.Fatal("expected unknown preset error")
	}
}
