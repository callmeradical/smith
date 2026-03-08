package model

var allowedTransitions = map[LoopState]map[LoopState]struct{}{
	LoopStateUnresolved: {
		LoopStateOverwriting: {},
		LoopStateCancelled:   {},
	},
	LoopStateOverwriting: {
		LoopStateSynced:     {},
		LoopStateFlatline:   {},
		LoopStateUnresolved: {},
		LoopStateCancelled:  {},
	},
}

func IsValidTransition(from, to LoopState) bool {
	next, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	_, ok = next[to]
	return ok
}
