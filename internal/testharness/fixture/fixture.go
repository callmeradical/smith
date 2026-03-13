package fixture

import (
	"fmt"
	"math/rand"
	"strings"
)

// LoopFixture holds deterministic identifiers used by integration/acceptance flows.
type LoopFixture struct {
	LoopID        string
	CorrelationID string
}

// NewLoopFixtureFromSeed returns deterministic loop/correlation identifiers.
func NewLoopFixtureFromSeed(prefix string, seed int64) LoopFixture {
	normalized := strings.ToLower(strings.TrimSpace(prefix))
	if normalized == "" {
		normalized = "fixture"
	}
	rng := rand.New(rand.NewSource(seed))
	token := fmt.Sprintf("%08x", rng.Uint32())
	return LoopFixture{
		LoopID:        fmt.Sprintf("%s-loop-%s", normalized, token),
		CorrelationID: fmt.Sprintf("%s-corr-%s", normalized, token),
	}
}
