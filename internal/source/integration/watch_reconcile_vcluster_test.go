//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"smith/internal/source/core"
	"smith/internal/source/model"
	"smith/internal/source/reconcile"
	"smith/internal/source/store"
	hassert "smith/internal/testharness/assertions"
	hfixture "smith/internal/testharness/fixture"
	hruntime "smith/internal/testharness/runtime"
)

func TestVClusterWatchReconcile(t *testing.T) {
	hruntime.RequireEnabled(t, "SMITH_IT_ENABLE")

	ctx := hruntime.ContextWithTimeout(t, 2*time.Minute)
	etcdEndpoints := splitCSV(os.Getenv("SMITH_IT_ETCD_ENDPOINTS"))
	if len(etcdEndpoints) == 0 {
		t.Fatal("SMITH_IT_ETCD_ENDPOINTS is required")
	}

	es, err := store.New(ctx, etcdEndpoints, 5*time.Second)
	require.NoError(t, err, "connect etcd")
	defer func() { _ = es.Close() }()

	kube, err := kubeClient()
	require.NoError(t, err, "kube client")

	namespace := envDefault("SMITH_IT_NAMESPACE", "smith-system")
	fixture := hfixture.NewLoopFixtureFromSeed("integration", time.Now().UTC().UnixNano())
	loopID := fixture.LoopID
	correlationID := fixture.CorrelationID
	hassert.RequireNonEmpty(t, "loop_id", loopID)
	hassert.RequireNonEmpty(t, "correlation_id", correlationID)

	watchCtx, cancelWatch := context.WithCancel(ctx)
	defer cancelWatch()
	events := es.WatchState(watchCtx)

	initial := model.StateRecord{
		LoopID:        loopID,
		State:         model.LoopStateUnresolved,
		Attempt:       0,
		CorrelationID: correlationID,
	}
	rev, err := es.PutState(ctx, initial, 0)
	require.NoError(t, err, "put initial state")

	waitForStateEvent(t, events, loopID, model.LoopStateUnresolved)

	q := &countingQueue{}
	watcher := core.NewUnresolvedWatcher(q)
	require.NoError(t, watcher.Handle(ctx, core.UnresolvedEvent{LoopID: loopID, State: model.LoopStateUnresolved, Revision: rev}), "handle unresolved event 1")
	require.NoError(t, watcher.Handle(ctx, core.UnresolvedEvent{LoopID: loopID, State: model.LoopStateUnresolved, Revision: rev}), "handle unresolved event duplicate")
	require.Equal(t, 1, q.count, "expected queue count 1 for duplicate event")

	stateAfterRunning, err := es.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		if current.State != model.LoopStateUnresolved {
			return current, fmt.Errorf("unexpected state %s", current.State)
		}
		current.State = model.LoopStateRunning
		current.Reason = "integration-progress"
		current.Attempt = 1
		return current, nil
	})
	require.NoError(t, err, "transition to running")
	hassert.RequireLoopState(t, stateAfterRunning.Record.State, model.LoopStateRunning)

	next := stateAfterRunning.Record
	next.State = model.LoopStateSynced
	next.Reason = "integration-complete"
	_, err = es.PutState(ctx, next, stateAfterRunning.Revision)
	require.NoError(t, err, "transition to synced")

	jobName := fmt.Sprintf("it-zombie-%d", time.Now().UTC().Unix()%100000)
	ensureNamespace(t, ctx, kube, namespace)
	_, err = kube.BatchV1().Jobs(namespace).Create(ctx, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:    "pause",
						Image:   "busybox:1.36",
						Command: []string{"sh", "-c", "sleep 300"},
					}},
				},
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err, "create zombie job")

	rLoop := reconcile.NewLoop(
		&etcdStateWriter{store: es},
		&kubeJobRuntime{kube: kube, namespace: namespace},
		noopMetrics{},
	)
	res, err := rLoop.ReconcileOne(ctx,
		reconcile.StateSnapshot{LoopID: loopID, State: model.LoopStateSynced, Attempt: 1, MaxAttempts: 3, IsStale: false},
		reconcile.RuntimeSnapshot{JobName: jobName, Phase: reconcile.RuntimeRunning},
	)
	require.NoError(t, err, "reconcile zombie delete")
	require.True(t, res.Corrected, "expected corrected reconcile result")
	require.Equal(t, "delete-zombie-job", res.Action)

	_, err = kube.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	require.Error(t, err)
	require.True(t, k8serrors.IsNotFound(err), "expected job to be deleted, err=%v", err)
}

func waitForStateEvent(t *testing.T, events <-chan store.Event, loopID string, expected model.LoopState) {
	t.Helper()
	timeout := time.NewTimer(15 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("watch channel closed")
			}
			if ev.LoopID == loopID && ev.State.State == expected {
				return
			}
		case <-timeout.C:
			t.Fatalf("timed out waiting for state event loop=%s state=%s", loopID, expected)
		}
	}
}

func ensureNamespace(t *testing.T, ctx context.Context, kube kubernetes.Interface, namespace string) {
	t.Helper()
	_, err := kube.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		return
	}
	if !k8serrors.IsNotFound(err) {
		t.Fatalf("get namespace: %v", err)
	}
	_, err = kube.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		t.Fatalf("create namespace: %v", err)
	}
}

func kubeClient() (*kubernetes.Clientset, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return kubernetes.NewForConfig(cfg)
	}
	kubeconfig := strings.TrimSpace(os.Getenv("KUBECONFIG"))
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			kubeconfig = home + "/.kube/config"
		}
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

type countingQueue struct{ count int }

func (q *countingQueue) Enqueue(_ context.Context, _ core.ExecutionIntent) error {
	q.count++
	return nil
}

type etcdStateWriter struct{ store *store.Store }

func (w *etcdStateWriter) Transition(ctx context.Context, loopID string, to model.LoopState, reason string) error {
	_, err := w.store.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		if current.State == to {
			return current, nil
		}
		if !model.IsValidTransition(current.State, to) {
			return current, errors.New("invalid transition")
		}
		current.State = to
		current.Reason = reason
		return current, nil
	})
	return err
}

type kubeJobRuntime struct {
	kube      kubernetes.Interface
	namespace string
}

func (r *kubeJobRuntime) Delete(ctx context.Context, _ string, jobName string, _ string) error {
	propagation := metav1.DeletePropagationBackground
	return r.kube.BatchV1().Jobs(r.namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &propagation})
}

type noopMetrics struct{}

func (noopMetrics) Inc(string) {}
