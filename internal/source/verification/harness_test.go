package verification

import (
	"testing"
)

func TestValidateHandoff(t *testing.T) {
	ok, _ := validateHandoff(Handoff{LoopID: "l", FinalDiffSummary: "d", ValidationState: "passed", NextSteps: "none"})
	if !ok {
		t.Fatal("expected valid handoff")
	}
	ok, _ = validateHandoff(Handoff{LoopID: "l"})
	if ok {
		t.Fatal("expected invalid handoff")
	}
}

func TestIsAmbiguous(t *testing.T) {
	if !isAmbiguous(PhaseState{CodeCommitted: true, StateCommitted: false, Compensated: false}) {
		t.Fatal("expected ambiguous state")
	}
	if isAmbiguous(PhaseState{CodeCommitted: true, StateCommitted: true, Compensated: false}) {
		t.Fatal("unexpected ambiguous state")
	}
}
