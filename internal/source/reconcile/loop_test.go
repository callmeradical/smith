package reconcile

import (
	"context"
	"testing"

	"smith/internal/source/model"
)

func TestReconcileCorrectsUnresolvedWhenRuntimeActive(t *testing.T) {
	state := &fakeStateWriter{}
	runtime := &fakeRuntime{}
	metrics := &fakeMetrics{}
	loop := NewLoop(state, runtime, metrics)

	result, err := loop.ReconcileOne(context.Background(), StateSnapshot{
		LoopID:      "loop-1",
		State:       model.LoopStateUnresolved,
		Attempt:     0,
		MaxAttempts: 3,
	}, RuntimeSnapshot{JobName: "job-1", Phase: RuntimeRunning})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Corrected || result.Action != "state->running" {
		t.Fatalf("unexpected result %+v", result)
	}
	if state.to != model.LoopStateRunning {
		t.Fatalf("expected overwrite transition, got %q", state.to)
	}
	if metrics.counts[MetricDriftCorrected] != 1 {
		t.Fatalf("expected corrected metric increment, got %+v", metrics.counts)
	}
}

func TestReconcileEscalatesStaleMissingRuntime(t *testing.T) {
	state := &fakeStateWriter{}
	runtime := &fakeRuntime{}
	metrics := &fakeMetrics{}
	loop := NewLoop(state, runtime, metrics)

	result, err := loop.ReconcileOne(context.Background(), StateSnapshot{
		LoopID:      "loop-2",
		State:       model.LoopStateRunning,
		Attempt:     1,
		MaxAttempts: 3,
		IsStale:     true,
	}, RuntimeSnapshot{JobName: "", Phase: RuntimeMissing})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Escalated || result.Action != "state->flatline" {
		t.Fatalf("unexpected result %+v", result)
	}
	if state.to != model.LoopStateFlatline {
		t.Fatalf("expected flatline transition, got %q", state.to)
	}
	if metrics.counts[MetricDriftEscalated] != 1 {
		t.Fatalf("expected escalated metric increment, got %+v", metrics.counts)
	}
}

func TestReconcileRetriesFailedRuntimeWhenAttemptsRemain(t *testing.T) {
	state := &fakeStateWriter{}
	runtime := &fakeRuntime{}
	metrics := &fakeMetrics{}
	loop := NewLoop(state, runtime, metrics)

	result, err := loop.ReconcileOne(context.Background(), StateSnapshot{
		LoopID:      "loop-3",
		State:       model.LoopStateRunning,
		Attempt:     0,
		MaxAttempts: 3,
	}, RuntimeSnapshot{JobName: "job-3", Phase: RuntimeFailed})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Corrected || result.Action != "state->unresolved" {
		t.Fatalf("unexpected result %+v", result)
	}
	if state.to != model.LoopStateUnresolved {
		t.Fatalf("expected unresolved transition, got %q", state.to)
	}
}

func TestReconcileDeletesZombieRuntimeForTerminalState(t *testing.T) {
	state := &fakeStateWriter{}
	runtime := &fakeRuntime{}
	metrics := &fakeMetrics{}
	loop := NewLoop(state, runtime, metrics)

	result, err := loop.ReconcileOne(context.Background(), StateSnapshot{
		LoopID:      "loop-4",
		State:       model.LoopStateSynced,
		Attempt:     1,
		MaxAttempts: 3,
	}, RuntimeSnapshot{JobName: "job-zombie", Phase: RuntimeRunning})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Corrected || result.Action != "delete-zombie-job" {
		t.Fatalf("unexpected result %+v", result)
	}
	if runtime.deletedJob != "job-zombie" {
		t.Fatalf("expected delete on zombie job, got %q", runtime.deletedJob)
	}
}

func TestReconcileNoopWhenStateAndRuntimeAligned(t *testing.T) {
	state := &fakeStateWriter{}
	runtime := &fakeRuntime{}
	metrics := &fakeMetrics{}
	loop := NewLoop(state, runtime, metrics)

	result, err := loop.ReconcileOne(context.Background(), StateSnapshot{
		LoopID:      "loop-5",
		State:       model.LoopStateRunning,
		Attempt:     1,
		MaxAttempts: 3,
	}, RuntimeSnapshot{JobName: "job-5", Phase: RuntimeRunning})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "no-op" || result.DriftDetected {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(metrics.counts) != 1 || metrics.counts[MetricReconcileRuns] != 1 {
		t.Fatalf("unexpected metrics %+v", metrics.counts)
	}
}

type fakeStateWriter struct {
	loopID string
	to     model.LoopState
	reason string
}

func (f *fakeStateWriter) Transition(_ context.Context, loopID string, to model.LoopState, reason string) error {
	f.loopID = loopID
	f.to = to
	f.reason = reason
	return nil
}

type fakeRuntime struct {
	loopID     string
	deletedJob string
	reason     string
}

func (f *fakeRuntime) Delete(_ context.Context, loopID string, jobName string, reason string) error {
	f.loopID = loopID
	f.deletedJob = jobName
	f.reason = reason
	return nil
}

type fakeMetrics struct {
	counts map[string]int
}

func (f *fakeMetrics) Inc(name string) {
	if f.counts == nil {
		f.counts = map[string]int{}
	}
	f.counts[name]++
}
