package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigFromFileAndOverrides(t *testing.T) {
	t.Setenv("SMITH_API_URL", "")
	t.Setenv("SMITH_OPERATOR_TOKEN", "")
	t.Setenv("SMITH_CONTEXT", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"dev","contexts":{"dev":{"server":"http://dev.local:8080","token":"dev-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resolved, err := resolveConfig(rootFlags{Config: cfgPath, Output: "json"})
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if resolved.Server != "http://dev.local:8080" {
		t.Fatalf("unexpected server: %s", resolved.Server)
	}
	if resolved.Token != "dev-token" {
		t.Fatalf("unexpected token: %s", resolved.Token)
	}
}

func TestResolveConfigEnvAndFlagPrecedence(t *testing.T) {
	t.Setenv("SMITH_API_URL", "http://env.local:8080")
	t.Setenv("SMITH_OPERATOR_TOKEN", "env-token")

	resolved, err := resolveConfig(rootFlags{Server: "http://flag.local:8080", Token: "flag-token", Output: "text"})
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if resolved.Server != "http://flag.local:8080" {
		t.Fatalf("unexpected server: %s", resolved.Server)
	}
	if resolved.Token != "flag-token" {
		t.Fatalf("unexpected token: %s", resolved.Token)
	}
}
