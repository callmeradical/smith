package acceptance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"smith/internal/source/model"
	hassert "smith/internal/testharness/assertions"
	hfixture "smith/internal/testharness/fixture"
	hruntime "smith/internal/testharness/runtime"
)

func TestHarnessSmoke(t *testing.T) {
	ctx := hruntime.ContextWithTimeout(t, 2*time.Second)
	select {
	case <-ctx.Done():
		require.FailNow(t, "context canceled too early")
	default:
	}

	a := hfixture.NewLoopFixtureFromSeed("acceptance", 42)
	b := hfixture.NewLoopFixtureFromSeed("acceptance", 42)
	require.Equal(t, a, b, "expected deterministic fixture ids")
	hassert.RequireNonEmpty(t, "loop_id", a.LoopID)
	hassert.RequireNonEmpty(t, "correlation_id", a.CorrelationID)

	hassert.RequireLoopState(t, model.LoopStateSynced, model.LoopStateSynced)
	hassert.RequireJournalMessage(t, []model.JournalEntry{
		{LoopID: a.LoopID, Message: "replica execution completed"},
	}, "execution completed")
}
