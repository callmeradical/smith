package model

import (
	"strings"
	"testing"
)

func TestNormalizeLoopSkillsAppliesCodexDefaults(t *testing.T) {
	skills, err := NormalizeLoopSkills([]LoopSkillMount{{
		Name:   "commit",
		Source: "local://skills/commit",
	}}, "codex")
	if err != nil {
		t.Fatalf("NormalizeLoopSkills error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].MountPath != "/smith/skills/commit" {
		t.Fatalf("unexpected mount path: %q", skills[0].MountPath)
	}
	if skills[0].ReadOnly == nil || !*skills[0].ReadOnly {
		t.Fatalf("expected read_only=true by default")
	}
}

func TestNormalizeLoopSkillsRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name        string
		skills      []LoopSkillMount
		errContains string
	}{
		{name: "missing name", skills: []LoopSkillMount{{Source: "x"}}, errContains: "loop.skills[0].name is required"},
		{name: "missing source", skills: []LoopSkillMount{{Name: "x"}}, errContains: "loop.skills[0].source is required"},
		{name: "relative path", skills: []LoopSkillMount{{Name: "x", Source: "s", MountPath: "tmp/skills"}}, errContains: "loop.skills[0].mount_path must be an absolute path"},
		{name: "traversal", skills: []LoopSkillMount{{Name: "x", Source: "s", MountPath: "/tmp/../skills"}}, errContains: "loop.skills[0].mount_path must not contain '..'"},
		{name: "root", skills: []LoopSkillMount{{Name: "x", Source: "s", MountPath: "/"}}, errContains: "loop.skills[0].mount_path must not be root"},
		{name: "dup", skills: []LoopSkillMount{{Name: "commit", Source: "a"}, {Name: "COMMIT", Source: "b"}}, errContains: "loop.skills[1].name \"COMMIT\" is duplicated"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeLoopSkills(tc.skills, "codex")
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.errContains)
			}
		})
	}
}

func TestNormalizeLoopSkillsRejectsUnsupportedProvider(t *testing.T) {
	_, err := NormalizeLoopSkills([]LoopSkillMount{{Name: "commit", Source: "local://x"}}, "other")
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}
