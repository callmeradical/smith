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

type LoopSkillMount struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	Version   string `json:"version,omitempty"`
	MountPath string `json:"mount_path,omitempty"`
	ReadOnly  *bool  `json:"read_only,omitempty"`
}

func NormalizeLoopSkills(skills []LoopSkillMount, providerID string) ([]LoopSkillMount, error) {
	provider := strings.ToLower(strings.TrimSpace(providerID))
	if provider == "" {
		provider = DefaultProviderID
	}
	if provider != DefaultProviderID {
		return nil, fmt.Errorf("loop.skills unsupported for provider %q", provider)
	}
	if len(skills) == 0 {
		return nil, nil
	}

	normalized := make([]LoopSkillMount, 0, len(skills))
	seenNames := map[string]struct{}{}
	for i, skill := range skills {
		name := strings.TrimSpace(skill.Name)
		if name == "" {
			return nil, fmt.Errorf("loop.skills[%d].name is required", i)
		}
		if !skillNamePattern.MatchString(name) {
			return nil, fmt.Errorf("loop.skills[%d].name %q is invalid (allowed: alphanumeric, ., _, -)", i, name)
		}
		canonicalName := strings.ToLower(name)
		if _, exists := seenNames[canonicalName]; exists {
			return nil, fmt.Errorf("loop.skills[%d].name %q is duplicated", i, name)
		}
		seenNames[canonicalName] = struct{}{}

		source := strings.TrimSpace(skill.Source)
		if source == "" {
			return nil, fmt.Errorf("loop.skills[%d].source is required", i)
		}

		mountPath := strings.TrimSpace(skill.MountPath)
		if mountPath == "" {
			mountPath = path.Join(CodexDefaultSkillMountRoot, canonicalName)
		}
		if err := validateSkillMountPath(mountPath, i); err != nil {
			return nil, err
		}

		readOnly := true
		if skill.ReadOnly != nil {
			readOnly = *skill.ReadOnly
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
	return normalized, nil
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
