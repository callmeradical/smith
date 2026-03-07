package runtime

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// Enabled returns true when env var is explicitly set to "true".
func Enabled(name string) bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(name)), "true")
}

// RequireEnabled skips the test unless the env var is "true".
func RequireEnabled(t testing.TB, envVar string) {
	t.Helper()
	if !Enabled(envVar) {
		t.Skip("set " + envVar + "=true to run this test")
	}
}

// ContextWithTimeout creates a test-scoped context canceled on cleanup.
func ContextWithTimeout(t testing.TB, timeout time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	return ctx
}
