package reconcile

import (
	"context"
	"fmt"

	"smith/internal/source/model"
)

const (
	MetricReconcileRuns  = "smith_reconcile_runs_total"
	MetricDriftDetected  = "smith_reconcile_drift_detected_total"
	MetricDriftCorrected = "smith_reconcile_drift_corrected_total"
	MetricDriftEscalated = "smith_reconcile_drift_escalated_total"
)

type RuntimePhase string

const (
	RuntimeMissing   RuntimePhase = "missing"
	RuntimePending   RuntimePhase = "pending"
	RuntimeRunning   RuntimePhase = "running"
	RuntimeSucceeded RuntimePhase = "succeeded"
	RuntimeFailed    RuntimePhase = "failed"
)

type StateSnapshot struct {
	LoopID      string
	State       model.LoopState
	Attempt     int
	MaxAttempts int
	IsStale     bool
}

type RuntimeSnapshot struct {
	JobName string
	Phase   RuntimePhase
}

type StateWriter interface {
	Transition(ctx context.Context, loopID string, to model.LoopState, reason string) error
}

type JobRuntime interface {
	Delete(ctx context.Context, loopID string, jobName string, reason string) error
}

type MetricsSink interface {
	Inc(name string)
}

type Result struct {
	DriftDetected bool
	Corrected     bool
	Escalated     bool
	Action        string
}

type Loop struct {
	state   StateWriter
	runtime JobRuntime
	metrics MetricsSink
}

func NewLoop(state StateWriter, runtime JobRuntime, metrics MetricsSink) *Loop {
	return &Loop{
		state:   state,
		runtime: runtime,
		metrics: metrics,
	}
}

func (l *Loop) ReconcileOne(ctx context.Context, state StateSnapshot, runtime RuntimeSnapshot) (Result, error) {
	l.metrics.Inc(MetricReconcileRuns)

	if state.LoopID == "" {
		return Result{}, fmt.Errorf("loop id is required")
	}

	switch {
	case state.State == model.LoopStateUnresolved && (runtime.Phase == RuntimePending || runtime.Phase == RuntimeRunning):
		l.metrics.Inc(MetricDriftDetected)
		if err := l.state.Transition(ctx, state.LoopID, model.LoopStateRunning, "runtime-active-detected"); err != nil {
			return Result{}, err
		}
		l.metrics.Inc(MetricDriftCorrected)
		return Result{DriftDetected: true, Corrected: true, Action: "state->running"}, nil

	case state.State == model.LoopStateRunning && runtime.Phase == RuntimeMissing:
		l.metrics.Inc(MetricDriftDetected)
		if state.IsStale {
			if err := l.state.Transition(ctx, state.LoopID, model.LoopStateFlatline, "runtime-missing-stale"); err != nil {
				return Result{}, err
			}
			l.metrics.Inc(MetricDriftEscalated)
			return Result{DriftDetected: true, Escalated: true, Action: "state->flatline"}, nil
		}
		if err := l.state.Transition(ctx, state.LoopID, model.LoopStateUnresolved, "runtime-missing-retry"); err != nil {
			return Result{}, err
		}
		l.metrics.Inc(MetricDriftCorrected)
		return Result{DriftDetected: true, Corrected: true, Action: "state->unresolved"}, nil

	case state.State == model.LoopStateRunning && runtime.Phase == RuntimeFailed:
		l.metrics.Inc(MetricDriftDetected)
		if state.Attempt+1 >= state.MaxAttempts {
			if err := l.state.Transition(ctx, state.LoopID, model.LoopStateFlatline, "runtime-failed-max-attempts"); err != nil {
				return Result{}, err
			}
			l.metrics.Inc(MetricDriftEscalated)
			return Result{DriftDetected: true, Escalated: true, Action: "state->flatline"}, nil
		}
		if err := l.state.Transition(ctx, state.LoopID, model.LoopStateUnresolved, "runtime-failed-retry"); err != nil {
			return Result{}, err
		}
		l.metrics.Inc(MetricDriftCorrected)
		return Result{DriftDetected: true, Corrected: true, Action: "state->unresolved"}, nil

	case isTerminal(state.State) && (runtime.Phase == RuntimePending || runtime.Phase == RuntimeRunning):
		l.metrics.Inc(MetricDriftDetected)
		if err := l.runtime.Delete(ctx, state.LoopID, runtime.JobName, "zombie-runtime-after-terminal-state"); err != nil {
			return Result{}, err
		}
		l.metrics.Inc(MetricDriftCorrected)
		return Result{DriftDetected: true, Corrected: true, Action: "delete-zombie-job"}, nil
	}

	return Result{Action: "no-op"}, nil
}

func isTerminal(state model.LoopState) bool {
	switch state {
	case model.LoopStateSynced, model.LoopStateFlatline, model.LoopStateCancelled:
		return true
	default:
		return false
	}
}
