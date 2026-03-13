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
		{name: "relative path", skills: []LoopSkillMount{{Name: "x", Source: "local://skills/x", MountPath: "tmp/skills"}}, errContains: "loop.skills[0].mount_path must be an absolute path"},
		{name: "traversal", skills: []LoopSkillMount{{Name: "x", Source: "local://skills/x", MountPath: "/tmp/../skills"}}, errContains: "loop.skills[0].mount_path must not contain '..'"},
		{name: "root", skills: []LoopSkillMount{{Name: "x", Source: "local://skills/x", MountPath: "/"}}, errContains: "loop.skills[0].mount_path must not be root"},
		{name: "dup", skills: []LoopSkillMount{{Name: "commit", Source: "local://skills/a"}, {Name: "COMMIT", Source: "local://skills/b"}}, errContains: "loop.skills[1].name \"COMMIT\" is duplicated"},
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

func TestNormalizeLoopSkillsWithPolicyRejectsDisallowedSource(t *testing.T) {
	_, _, err := NormalizeLoopSkillsWithPolicy([]LoopSkillMount{{
		Name:   "commit",
		Source: "https://example.com/skills/commit",
	}}, "codex", SkillPolicy{
		AllowedSourcePrefixes: []string{"local://skills/"},
	})
	if err == nil {
		t.Fatal("expected source allowlist error")
	}
	if !strings.Contains(err.Error(), "is not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeLoopSkillsWithPolicyRejectsWritableWithoutPolicy(t *testing.T) {
	readOnly := false
	_, _, err := NormalizeLoopSkillsWithPolicy([]LoopSkillMount{{
		Name:     "commit",
		Source:   "local://skills/commit",
		ReadOnly: &readOnly,
	}}, "codex", SkillPolicy{
		AllowedSourcePrefixes: []string{"local://skills/"},
		AllowWritable:         false,
	})
	if err == nil {
		t.Fatal("expected writable policy error")
	}
	if !strings.Contains(err.Error(), "read_only=false is not allowed by policy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeLoopSkillsWithPolicyAuditsDefaultsAndOverrides(t *testing.T) {
	readOnly := false
	skills, audit, err := NormalizeLoopSkillsWithPolicy([]LoopSkillMount{
		{
			Name:   "commit",
			Source: "local://skills/commit",
		},
		{
			Name:     "lint",
			Source:   "local://skills/lint",
			ReadOnly: &readOnly,
		},
	}, "codex", SkillPolicy{
		AllowedSourcePrefixes: []string{"local://skills/"},
		AllowWritable:         true,
	})
	if err != nil {
		t.Fatalf("NormalizeLoopSkillsWithPolicy error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	if audit.RequestedCount != 2 {
		t.Fatalf("expected requested_count=2, got %d", audit.RequestedCount)
	}
	if audit.DefaultReadOnlyCount != 1 {
		t.Fatalf("expected default_read_only_count=1, got %d", audit.DefaultReadOnlyCount)
	}
	if audit.WritableCount != 1 {
		t.Fatalf("expected writable_count=1, got %d", audit.WritableCount)
	}
	if audit.WritableOverrideCount != 1 {
		t.Fatalf("expected writable_override_count=1, got %d", audit.WritableOverrideCount)
	}
}
