package core

import (
	"context"
	"time"

	"smith/internal/source/model"
)

const defaultWatchRestartDelay = 250 * time.Millisecond

type StateSnapshot struct {
	LoopID   string
	State    model.LoopState
	Revision int64
}

type StateSource interface {
	ListStates(ctx context.Context) ([]StateSnapshot, error)
	WatchStates(ctx context.Context) <-chan StateSnapshot
}

type EventHandler interface {
	Handle(ctx context.Context, event UnresolvedEvent) error
}

type UnresolvedController struct {
	source       StateSource
	handler      EventHandler
	policy       model.LoopPolicy
	restartDelay time.Duration
	sleep        sleepFn
}

func NewUnresolvedController(source StateSource, handler EventHandler, policy model.LoopPolicy) *UnresolvedController {
	return &UnresolvedController{
		source:       source,
		handler:      handler,
		policy:       policy,
		restartDelay: defaultWatchRestartDelay,
		sleep:        sleepContext,
	}
}

func (c *UnresolvedController) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		states, err := c.source.ListStates(ctx)
		if err != nil {
			if err := c.sleep(ctx, c.restartDelay); err != nil {
				return nil
			}
			continue
		}
		for _, state := range states {
			c.handleState(ctx, state)
		}

		watchCh := c.source.WatchStates(ctx)
		for {
			select {
			case <-ctx.Done():
				return nil
			case state, ok := <-watchCh:
				if !ok {
					if err := c.sleep(ctx, c.restartDelay); err != nil {
						return nil
					}
					goto restart
				}
				c.handleState(ctx, state)
			}
		}

	restart:
	}
}

func (c *UnresolvedController) handleState(ctx context.Context, state StateSnapshot) {
	if state.State != model.LoopStateUnresolved {
		return
	}
	_ = c.handler.Handle(ctx, UnresolvedEvent{
		LoopID:   state.LoopID,
		State:    state.State,
		Revision: state.Revision,
		Policy:   c.policy,
	})
}
