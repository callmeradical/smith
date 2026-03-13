package model

import (
	"fmt"
	"strings"
)

const (
	DefaultProviderID = "codex"
	DefaultModel      = "gpt-5-codex"
	CodexMiniModel    = "gpt-5-codex-mini"
)

func NormalizeProviderModel(providerID, model string) (string, string, error) {
	provider := strings.ToLower(strings.TrimSpace(providerID))
	if provider == "" {
		provider = DefaultProviderID
	}

	if provider != DefaultProviderID {
		return "", "", fmt.Errorf("unsupported provider_id %q", provider)
	}

	resolvedModel := strings.ToLower(strings.TrimSpace(model))
	if resolvedModel == "" {
		resolvedModel = DefaultModel
	}
	if resolvedModel == "" {
		return "", "", fmt.Errorf("model is required")
	}
	switch resolvedModel {
	case DefaultModel, CodexMiniModel:
	default:
		return "", "", fmt.Errorf("unsupported model %q for provider %q", resolvedModel, provider)
	}

	return provider, resolvedModel, nil
}

func (a *Anomaly) NormalizeProviderSelection() error {
	providerID, model, err := NormalizeProviderModel(a.ProviderID, a.Model)
	if err != nil {
		return err
	}
	a.ProviderID = providerID
	a.Model = model
	return nil
}
