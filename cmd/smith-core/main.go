package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"smith/internal/source/core"
	"smith/internal/source/locking"
	"smith/internal/source/model"
	"smith/internal/source/store"
)

const (
	defaultPort            = 8081
	defaultShutdownTimeout = 10 * time.Second
)

type config struct {
	port              int
	etcdEndpoints     []string
	etcdDialTimeout   time.Duration
	namespace         string
	holderID          string
	replicaImage      string
	replicaPullPolicy string
	replicaSA         string
	defaultPolicy     model.LoopPolicy
}

type executionImageSelection struct {
	Ref        string
	PullPolicy string
	Source     string
	Digest     string
}

type orchestrator struct {
	store      *store.Store
	locks      *locking.Manager
	kube       kubernetes.Interface
	cfg        config
	jobTTL     int32
	jobBackoff int32
}

type intentQueue struct {
	orch *orchestrator
}

func (q *intentQueue) Enqueue(ctx context.Context, intent core.ExecutionIntent) error {
	return q.orch.HandleIntent(ctx, intent)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("smith-core config error: %v", err)
	}

	es, err := store.New(ctx, cfg.etcdEndpoints, cfg.etcdDialTimeout)
	if err != nil {
		log.Fatalf("smith-core etcd init failed: %v", err)
	}
	defer func() { _ = es.Close() }()

	kube, err := kubeClient()
	if err != nil {
		log.Fatalf("smith-core kube init failed: %v", err)
	}

	orch := &orchestrator{
		store:      es,
		locks:      locking.NewManager(store.NewEtcdLeaseStore(es), 30*time.Second),
		kube:       kube,
		cfg:        cfg,
		jobTTL:     3600,
		jobBackoff: 0,
	}

	watcher := core.NewUnresolvedWatcher(&intentQueue{orch: orch})
	controller := core.NewUnresolvedController(coreStateSource{store: es}, watcher, cfg.defaultPolicy)
	go func() {
		if runErr := controller.Run(ctx); runErr != nil {
			log.Printf("unresolved controller exited with error: %v", runErr)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	addr := fmt.Sprintf(":%d", cfg.port)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("smith-core listening on %s", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Printf("smith-core shutdown requested")
	case serveErr := <-errCh:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("smith-core failed: %v", serveErr)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("smith-core shutdown failed: %v", err)
	}
}

type coreStateSource struct {
	store *store.Store
}

func (s coreStateSource) ListStates(ctx context.Context) ([]core.StateSnapshot, error) {
	states, err := s.store.ListStates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]core.StateSnapshot, 0, len(states))
	for _, state := range states {
		out = append(out, core.StateSnapshot{
			LoopID:   state.Record.LoopID,
			State:    state.Record.State,
			Revision: state.Revision,
		})
	}
	return out, nil
}

func (s coreStateSource) WatchStates(ctx context.Context) <-chan core.StateSnapshot {
	storeEvents := s.store.WatchState(ctx)
	out := make(chan core.StateSnapshot)
	go func() {
		defer close(out)
		for event := range storeEvents {
			select {
			case <-ctx.Done():
				return
			case out <- core.StateSnapshot{
				LoopID:   event.LoopID,
				State:    event.State.State,
				Revision: event.Revision,
			}:
			}
		}
	}()
	return out
}

func (o *orchestrator) HandleIntent(ctx context.Context, intent core.ExecutionIntent) error {
	leaseID := time.Now().UTC().UnixNano()
	_, err := o.locks.Acquire(ctx, intent.LoopID, o.cfg.holderID, leaseID)
	if err != nil {
		return err
	}
	defer func() {
		if releaseErr := o.locks.Release(context.Background(), intent.LoopID, o.cfg.holderID); releaseErr != nil {
			log.Printf("lock release failed loop=%s: %v", intent.LoopID, releaseErr)
		}
	}()

	current, found, err := o.store.GetState(ctx, intent.LoopID)
	if err != nil {
		return err
	}
	if !found || current.Record.State != model.LoopStateUnresolved {
		return nil
	}
	anomaly, anomalyFound, err := o.store.GetAnomaly(ctx, intent.LoopID)
	if err != nil {
		return err
	}
	var anomalyPtr *model.Anomaly
	if anomalyFound {
		anomalyPtr = &anomaly
	}
	executionImage := resolveExecutionImageSelection(anomalyPtr, o.cfg)

	jobName := replicaJobName(intent.LoopID)
	next := current.Record
	next.State = model.LoopStateOverwriting
	next.Attempt = current.Record.Attempt + 1
	next.WorkerJobName = jobName
	next.LockHolder = o.cfg.holderID
	next.Reason = "scheduled-by-core"
	if _, putErr := o.store.PutState(ctx, next, current.Revision); putErr != nil {
		return putErr
	}

	if err := o.createReplicaJob(ctx, intent.LoopID, jobName, next.CorrelationID, executionImage); err != nil {
		_, _ = o.store.PutStateFromCurrent(ctx, intent.LoopID, func(current model.StateRecord) (model.StateRecord, error) {
			if current.State != model.LoopStateOverwriting {
				return current, nil
			}
			current.State = model.LoopStateFlatline
			current.Reason = "replica-job-create-failed"
			return current, nil
		})
		_ = o.store.AppendJournal(ctx, model.JournalEntry{
			LoopID:        intent.LoopID,
			Phase:         "core",
			Level:         "error",
			ActorType:     "core",
			ActorID:       o.cfg.holderID,
			Message:       "failed to create replica job",
			CorrelationID: next.CorrelationID,
			Metadata: map[string]string{
				"job_name": jobName,
				"error":    err.Error(),
			},
		})
		return err
	}

	_ = o.store.AppendJournal(ctx, model.JournalEntry{
		LoopID:        intent.LoopID,
		Phase:         "core",
		Level:         "info",
		ActorType:     "core",
		ActorID:       o.cfg.holderID,
		Message:       "replica job scheduled",
		CorrelationID: next.CorrelationID,
		Metadata: map[string]string{
			"job_name":                    jobName,
			"state":                       string(model.LoopStateOverwriting),
			"execution_image_ref":         executionImage.Ref,
			"execution_image_source":      executionImage.Source,
			"execution_image_digest":      executionImage.Digest,
			"execution_image_pull_policy": executionImage.PullPolicy,
		},
	})
	return nil
}

func (o *orchestrator) createReplicaJob(ctx context.Context, loopID, jobName, correlationID string, executionImage executionImageSelection) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: o.cfg.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "smith-replica",
				"app.kubernetes.io/component": "replica",
				"smith.io/loop-id":            loopID,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &o.jobBackoff,
			TTLSecondsAfterFinished: &o.jobTTL,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      "smith-replica",
						"app.kubernetes.io/component": "replica",
						"smith.io/loop-id":            loopID,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: o.cfg.replicaSA,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "replica",
							Image:           executionImage.Ref,
							ImagePullPolicy: corev1.PullPolicy(executionImage.PullPolicy),
							Env: []corev1.EnvVar{
								{Name: "SMITH_LOOP_ID", Value: loopID},
								{Name: "SMITH_CORRELATION_ID", Value: correlationID},
								{Name: "SMITH_ETCD_ENDPOINTS", Value: strings.Join(o.cfg.etcdEndpoints, ",")},
								{Name: "SMITH_EXECUTION_IMAGE_REF", Value: executionImage.Ref},
								{Name: "SMITH_EXECUTION_IMAGE_SOURCE", Value: executionImage.Source},
								{Name: "SMITH_EXECUTION_IMAGE_DIGEST", Value: executionImage.Digest},
								{Name: "SMITH_EXECUTION_IMAGE_PULL_POLICY", Value: executionImage.PullPolicy},
							},
						},
					},
				},
			},
		},
	}

	_, err := o.kube.BatchV1().Jobs(o.cfg.namespace).Create(ctx, job, metav1.CreateOptions{})
	return err
}

func resolveExecutionImageSelection(anomaly *model.Anomaly, cfg config) executionImageSelection {
	ref := strings.TrimSpace(cfg.replicaImage)
	pullPolicy := strings.TrimSpace(cfg.replicaPullPolicy)
	source := "core_default"

	if anomaly != nil && anomaly.Environment.ContainerImage != nil && strings.TrimSpace(anomaly.Environment.ContainerImage.Ref) != "" {
		ref = strings.TrimSpace(anomaly.Environment.ContainerImage.Ref)
		source = "loop_environment_container_image"
		if strings.TrimSpace(anomaly.Environment.ContainerImage.PullPolicy) != "" {
			pullPolicy = strings.TrimSpace(anomaly.Environment.ContainerImage.PullPolicy)
		}
	}
	if pullPolicy == "" {
		pullPolicy = string(corev1.PullIfNotPresent)
	}
	return executionImageSelection{
		Ref:        ref,
		PullPolicy: pullPolicy,
		Source:     source,
		Digest:     parseImageDigest(ref),
	}
}

func parseImageDigest(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	parts := strings.SplitN(ref, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func loadConfig() (config, error) {
	endpoints := splitCSV(os.Getenv("SMITH_ETCD_ENDPOINTS"))
	if len(endpoints) == 0 {
		endpoints = []string{"http://127.0.0.1:2379"}
	}

	holderID := strings.TrimSpace(os.Getenv("SMITH_CORE_HOLDER_ID"))
	if holderID == "" {
		host, _ := os.Hostname()
		holderID = "smith-core"
		if host != "" {
			holderID = "smith-core-" + host
		}
	}

	cfg := config{
		port:              envInt("SMITH_CORE_PORT", defaultPort),
		etcdEndpoints:     endpoints,
		etcdDialTimeout:   envDuration("SMITH_ETCD_DIAL_TIMEOUT", 5*time.Second),
		namespace:         envString("SMITH_NAMESPACE", "default"),
		holderID:          holderID,
		replicaImage:      envString("SMITH_REPLICA_IMAGE", "ghcr.io/smith/replica:v0.1.0"),
		replicaPullPolicy: envString("SMITH_REPLICA_IMAGE_PULL_POLICY", string(corev1.PullIfNotPresent)),
		replicaSA:         envString("SMITH_REPLICA_TEMPLATE_SERVICE_ACCOUNT", "default"),
		defaultPolicy: model.LoopPolicy{
			MaxAttempts:      envInt("SMITH_LOOP_POLICY_MAX_ATTEMPTS", 3),
			BackoffInitial:   envDuration("SMITH_LOOP_POLICY_BACKOFF_INITIAL", 5*time.Second),
			BackoffMax:       envDuration("SMITH_LOOP_POLICY_BACKOFF_MAX", 2*time.Minute),
			Timeout:          envDuration("SMITH_LOOP_POLICY_TIMEOUT", 30*time.Minute),
			TerminateOnError: envBool("SMITH_LOOP_POLICY_TERMINATE_ON_ERROR", false),
		},
	}
	return cfg, nil
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

func replicaJobName(loopID string) string {
	s := strings.NewReplacer("/", "-", "_", "-", ".", "-", " ", "-").Replace(strings.ToLower(loopID))
	s = strings.Trim(s, "-")
	if s == "" {
		s = "loop"
	}
	if len(s) > 32 {
		s = s[:32]
	}
	return fmt.Sprintf("smith-replica-%s-%d", s, time.Now().UTC().Unix()%100000)
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

func envString(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return b
}

func envInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
