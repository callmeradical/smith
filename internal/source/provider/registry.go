package provider

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	ProviderCodex     = "codex"
	DefaultProviderID = ProviderCodex
	DefaultCodexModel = "gpt-5-codex"
	CodexMiniModel    = "gpt-5-codex-mini"
)

var (
	ErrInvalidRegistration = errors.New("invalid provider registration")
	ErrUnknownProvider     = errors.New("unknown provider")
	ErrUnsupportedModel    = errors.New("unsupported provider model")
)

type Registry struct {
	defaultProviderID string
	registrations     map[string]Registration
}

func NewRegistry(defaultProviderID string) *Registry {
	return &Registry{
		defaultProviderID: normalize(defaultProviderID),
		registrations:     make(map[string]Registration),
	}
}

func NewDefaultRegistry() *Registry {
	registry := NewRegistry(DefaultProviderID)
	_ = registry.Register(NewCodexRegistration())
	return registry
}

func (r *Registry) Register(reg Registration) error {
	providerID := normalize(reg.ProviderID)
	if providerID == "" || reg.Adapter == nil {
		return ErrInvalidRegistration
	}

	models := make([]string, 0, len(reg.Models))
	for _, model := range reg.Models {
		normalized := normalize(model)
		if normalized == "" {
			continue
		}
		models = append(models, normalized)
	}
	if len(models) == 0 {
		return fmt.Errorf("%w: models are required", ErrInvalidRegistration)
	}

	defaultModel := normalize(reg.DefaultModel)
	if defaultModel == "" {
		defaultModel = models[0]
	}
	if !slices.Contains(models, defaultModel) {
		return fmt.Errorf("%w: default model %q missing from model catalog", ErrInvalidRegistration, defaultModel)
	}

	r.registrations[providerID] = Registration{
		ProviderID:   providerID,
		DefaultModel: defaultModel,
		Models:       models,
		Adapter:      reg.Adapter,
	}

	if r.defaultProviderID == "" {
		r.defaultProviderID = providerID
	}
	return nil
}

func (r *Registry) Resolve(providerID string, model string) (Selection, error) {
	providerID = normalize(providerID)
	if providerID == "" {
		providerID = r.defaultProviderID
	}

	reg, ok := r.registrations[providerID]
	if !ok {
		return Selection{}, fmt.Errorf("%w: %s", ErrUnknownProvider, providerID)
	}

	resolvedModel := normalize(model)
	if resolvedModel == "" {
		resolvedModel = reg.DefaultModel
	}
	if !slices.Contains(reg.Models, resolvedModel) {
		return Selection{}, fmt.Errorf("%w: provider=%s model=%s", ErrUnsupportedModel, providerID, resolvedModel)
	}

	if err := reg.Adapter.ValidateConfig(Config{Model: resolvedModel}); err != nil {
		return Selection{}, err
	}

	return Selection{
		ProviderID: providerID,
		Model:      resolvedModel,
		Adapter:    reg.Adapter,
	}, nil
}

func normalize(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}
