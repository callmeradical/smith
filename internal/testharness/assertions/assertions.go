package assertions

import (
	"strings"
	"testing"

	"smith/internal/source/model"
)

// RequireLoopState fails the test when got != want.
func RequireLoopState(t testing.TB, got, want model.LoopState) {
	t.Helper()
	if got != want {
		t.Fatalf("unexpected loop state: got=%s want=%s", got, want)
	}
}

// RequireNonEmpty fails the test when value is blank.
func RequireNonEmpty(t testing.TB, field, value string) {
	t.Helper()
	if strings.TrimSpace(value) == "" {
		t.Fatalf("expected non-empty value for %s", field)
	}
}

// RequireJournalMessage finds a journal message containing substring.
func RequireJournalMessage(t testing.TB, entries []model.JournalEntry, contains string) {
	t.Helper()
	for _, entry := range entries {
		if strings.Contains(entry.Message, contains) {
			return
		}
	}
	t.Fatalf("expected journal message containing %q", contains)
}
