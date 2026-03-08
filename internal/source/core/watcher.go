package core

import (
	"context"
	"errors"
	"sync"
	"time"

	"smith/internal/source/model"
)

const (
	defaultRetryMaxAttempts = 3
	defaultBackoffInitial   = 100 * time.Millisecond
	defaultBackoffMax       = 2 * time.Second
)

var ErrRetryExhausted = errors.New("retry attempts exhausted")

type ExecutionIntent struct {
	LoopID   string
	Revision int64
}

type UnresolvedEvent struct {
	LoopID   string
	State    model.LoopState
	Revision int64
	Policy   model.LoopPolicy
}

type IntentQueue interface {
	Enqueue(ctx context.Context, intent ExecutionIntent) error
}

type sleepFn func(context.Context, time.Duration) error

type loopRecord struct {
	revision int64
	inFlight bool
	enqueued bool
}

type UnresolvedWatcher struct {
	queue IntentQueue
	sleep sleepFn
	mu    sync.Mutex
	loops map[string]loopRecord
}

func NewUnresolvedWatcher(queue IntentQueue) *UnresolvedWatcher {
	return &UnresolvedWatcher{
		queue: queue,
		sleep: sleepContext,
		loops: make(map[string]loopRecord),
	}
}

func (w *UnresolvedWatcher) Handle(ctx context.Context, event UnresolvedEvent) error {
	if event.State != model.LoopStateUnresolved {
		return nil
	}

	if !w.tryAcquire(event.LoopID, event.Revision) {
		return nil
	}

	err := w.enqueueWithRetry(ctx, event)
	w.finishAttempt(event.LoopID, event.Revision, err == nil)
	return err
}

func (w *UnresolvedWatcher) tryAcquire(loopID string, revision int64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	record, ok := w.loops[loopID]
	if ok {
		if revision < record.revision {
			return false
		}
		if revision == record.revision && (record.inFlight || record.enqueued) {
			return false
		}
	}

	w.loops[loopID] = loopRecord{
		revision: revision,
		inFlight: true,
	}
	return true
}

func (w *UnresolvedWatcher) finishAttempt(loopID string, revision int64, enqueued bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	record, ok := w.loops[loopID]
	if !ok || record.revision != revision {
		return
	}

	record.inFlight = false
	record.enqueued = enqueued
	w.loops[loopID] = record
}

func (w *UnresolvedWatcher) enqueueWithRetry(ctx context.Context, event UnresolvedEvent) error {
	policy := retryPolicyFor(event.Policy)
	intent := ExecutionIntent{
		LoopID:   event.LoopID,
		Revision: event.Revision,
	}

	var lastErr error
	for attempt := 1; attempt <= policy.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := w.queue.Enqueue(ctx, intent)
		if err == nil {
			return nil
		}

		lastErr = err
		if attempt == policy.maxAttempts {
			break
		}

		backoff := policy.backoffForAttempt(attempt)
		if sleepErr := w.sleep(ctx, backoff); sleepErr != nil {
			return sleepErr
		}
	}

	return errors.Join(ErrRetryExhausted, lastErr)
}

type retryPolicy struct {
	maxAttempts int
	initial     time.Duration
	max         time.Duration
}

func retryPolicyFor(policy model.LoopPolicy) retryPolicy {
	out := retryPolicy{
		maxAttempts: policy.MaxAttempts,
		initial:     policy.BackoffInitial,
		max:         policy.BackoffMax,
	}

	if out.maxAttempts <= 0 {
		out.maxAttempts = defaultRetryMaxAttempts
	}
	if out.initial <= 0 {
		out.initial = defaultBackoffInitial
	}
	if out.max <= 0 {
		out.max = defaultBackoffMax
	}
	if out.max < out.initial {
		out.max = out.initial
	}

	return out
}

func (r retryPolicy) backoffForAttempt(attempt int) time.Duration {
	multiplier := 1 << (attempt - 1)
	backoff := r.initial * time.Duration(multiplier)
	if backoff > r.max {
		return r.max
	}
	return backoff
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
