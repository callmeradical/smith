package model

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

const (
	CodexDefaultSkillMountRoot = "/smith/skills"
)

var skillNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

type SkillPolicy struct {
	AllowedSourcePrefixes []string
	AllowWritable         bool
}

type SkillNormalizationAudit struct {
	RequestedCount        int
	DefaultReadOnlyCount  int
	WritableCount         int
	WritableOverrideCount int
}

type LoopSkillMount struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	Version   string `json:"version,omitempty"`
	MountPath string `json:"mount_path,omitempty"`
	ReadOnly  *bool  `json:"read_only,omitempty"`
}

func DefaultSkillPolicy() SkillPolicy {
	return SkillPolicy{
		AllowedSourcePrefixes: []string{"local://skills/"},
		AllowWritable:         false,
	}
}

func NormalizeLoopSkills(skills []LoopSkillMount, providerID string) ([]LoopSkillMount, error) {
	normalized, _, err := NormalizeLoopSkillsWithPolicy(skills, providerID, DefaultSkillPolicy())
	return normalized, err
}

func NormalizeLoopSkillsWithPolicy(skills []LoopSkillMount, providerID string, policy SkillPolicy) ([]LoopSkillMount, SkillNormalizationAudit, error) {
	provider := strings.ToLower(strings.TrimSpace(providerID))
	if provider == "" {
		provider = DefaultProviderID
	}
	if provider != DefaultProviderID {
		return nil, SkillNormalizationAudit{}, fmt.Errorf("loop.skills unsupported for provider %q", provider)
	}
	if len(skills) == 0 {
		return nil, SkillNormalizationAudit{}, nil
	}
	normalizedPrefixes := normalizeSourcePrefixes(policy.AllowedSourcePrefixes)
	audit := SkillNormalizationAudit{RequestedCount: len(skills)}

	normalized := make([]LoopSkillMount, 0, len(skills))
	seenNames := map[string]struct{}{}
	for i, skill := range skills {
		name := strings.TrimSpace(skill.Name)
		if name == "" {
			return nil, SkillNormalizationAudit{}, fmt.Errorf("loop.skills[%d].name is required", i)
		}
		if !skillNamePattern.MatchString(name) {
			return nil, SkillNormalizationAudit{}, fmt.Errorf("loop.skills[%d].name %q is invalid (allowed: alphanumeric, ., _, -)", i, name)
		}
		canonicalName := strings.ToLower(name)
		if _, exists := seenNames[canonicalName]; exists {
			return nil, SkillNormalizationAudit{}, fmt.Errorf("loop.skills[%d].name %q is duplicated", i, name)
		}
		seenNames[canonicalName] = struct{}{}

		source := strings.TrimSpace(skill.Source)
		if source == "" {
			return nil, SkillNormalizationAudit{}, fmt.Errorf("loop.skills[%d].source is required", i)
		}
		if !sourceAllowed(source, normalizedPrefixes) {
			return nil, SkillNormalizationAudit{}, fmt.Errorf(
				"loop.skills[%d].source %q is not allowed (allowed prefixes: %s)",
				i,
				source,
				strings.Join(normalizedPrefixes, ", "),
			)
		}

		mountPath := strings.TrimSpace(skill.MountPath)
		if mountPath == "" {
			mountPath = path.Join(CodexDefaultSkillMountRoot, canonicalName)
		}
		if err := validateSkillMountPath(mountPath, i); err != nil {
			return nil, SkillNormalizationAudit{}, err
		}

		readOnly := true
		if skill.ReadOnly != nil {
			readOnly = *skill.ReadOnly
		}
		if skill.ReadOnly == nil {
			audit.DefaultReadOnlyCount++
		}
		if !readOnly {
			if !policy.AllowWritable {
				return nil, SkillNormalizationAudit{}, fmt.Errorf("loop.skills[%d].read_only=false is not allowed by policy", i)
			}
			audit.WritableCount++
			audit.WritableOverrideCount++
		}
		version := strings.TrimSpace(skill.Version)

		normalized = append(normalized, LoopSkillMount{
			Name:      name,
			Source:    source,
			Version:   version,
			MountPath: mountPath,
			ReadOnly:  boolPtr(readOnly),
		})
	}

	sort.Slice(normalized, func(i, j int) bool {
		return strings.ToLower(normalized[i].Name) < strings.ToLower(normalized[j].Name)
	})
	return normalized, audit, nil
}

func validateSkillMountPath(mountPath string, index int) error {
	if !strings.HasPrefix(mountPath, "/") {
		return fmt.Errorf("loop.skills[%d].mount_path must be an absolute path", index)
	}
	if strings.Contains(mountPath, "..") {
		return fmt.Errorf("loop.skills[%d].mount_path must not contain '..'", index)
	}
	cleaned := path.Clean(mountPath)
	if cleaned == "/" {
		return fmt.Errorf("loop.skills[%d].mount_path must not be root", index)
	}
	if cleaned != mountPath {
		return fmt.Errorf("loop.skills[%d].mount_path must be normalized", index)
	}
	return nil
}

func boolPtr(v bool) *bool {
	return &v
}

func normalizeSourcePrefixes(prefixes []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		trimmed := strings.TrimSpace(prefix)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return append([]string{}, DefaultSkillPolicy().AllowedSourcePrefixes...)
	}
	sort.Strings(out)
	return out
}

func sourceAllowed(source string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(source, prefix) {
			return true
		}
	}
	return false
}
