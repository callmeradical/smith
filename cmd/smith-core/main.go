package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"smith/internal/source/core"
	"smith/internal/source/gitpolicy"
	"smith/internal/source/journalpolicy"
	"smith/internal/source/locking"
	"smith/internal/source/model"
	"smith/internal/source/replica"
	"smith/internal/source/store"
)

const (
	defaultPort             = 8081
	defaultShutdownTimeout  = 10 * time.Second
	defaultWorkspacePRDPath = ".agents/tasks/prd.json"
	defaultWorkspacePRDKey  = "prd.json"
)

type config struct {
	port                int
	etcdEndpoints       []string
	etcdDialTimeout     time.Duration
	namespace           string
	holderID            string
	replicaImage        string
	replicaPullPolicy   string
	workspaceSeedImage  string
	workspaceSeedPolicy string
	runtimeCredentials  string
	dockerfileRepo      string
	dockerfileBuild     bool
	gitPolicy           gitpolicy.Policy
	gitPolicyConfig     bool
	journalPolicy       journalpolicy.Policy
	journalPolicyConfig bool
	replicaSA           string
	defaultPolicy       model.LoopPolicy
}

type executionImageSelection struct {
	Ref        string
	PullPolicy string
	Source     string
	Digest     string
	BuildInfo  map[string]string
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
	if !anomalyFound {
		return fmt.Errorf("anomaly not found for loop %s", intent.LoopID)
	}
	var anomalyPtr *model.Anomaly
	anomalyPtr = &anomaly
	executionImage, err := resolveExecutionImageSelection(ctx, anomalyPtr, o.cfg, intent.LoopID)
	if err != nil {
		_, _ = o.store.PutStateFromCurrent(ctx, intent.LoopID, func(current model.StateRecord) (model.StateRecord, error) {
			if current.State != model.LoopStateUnresolved {
				return current, nil
			}
			current.State = model.LoopStateFlatline
			current.Reason = "execution-image-resolution-failed"
			return current, nil
		})
		_ = o.store.AppendJournal(ctx, model.JournalEntry{
			LoopID:        intent.LoopID,
			Phase:         "core",
			Level:         "error",
			ActorType:     "core",
			ActorID:       o.cfg.holderID,
			Message:       "failed to resolve execution image",
			CorrelationID: current.Record.CorrelationID,
			Metadata: map[string]string{
				"error": err.Error(),
			},
		})
		return err
	}
	resolvedSkillMounts, resolvedSkillNames, err := resolveSkillMounts(anomalyPtr)
	if err != nil {
		return err
	}

	jobName := replicaJobName(intent.LoopID)
	next := current.Record
	next.State = model.LoopStateRunning
	next.Attempt = current.Record.Attempt + 1
	next.WorkerJobName = jobName
	next.LockHolder = o.cfg.holderID
	next.Reason = "scheduled-by-core"
	if _, putErr := o.store.PutState(ctx, next, current.Revision); putErr != nil {
		return putErr
	}

	if err := o.createReplicaJob(ctx, intent.LoopID, jobName, next.CorrelationID, executionImage, anomaly, resolvedSkillMounts); err != nil {
		_, _ = o.store.PutStateFromCurrent(ctx, intent.LoopID, func(current model.StateRecord) (model.StateRecord, error) {
			if current.State != model.LoopStateRunning {
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
			"state":                       string(model.LoopStateRunning),
			"execution_image_ref":         executionImage.Ref,
			"execution_image_source":      executionImage.Source,
			"execution_image_digest":      executionImage.Digest,
			"execution_image_pull_policy": executionImage.PullPolicy,
			"skill_mount_count":           strconv.Itoa(len(resolvedSkillMounts)),
			"skill_mounts":                strings.Join(resolvedSkillNames, ","),
			"journal_retention_mode":      string(o.cfg.journalPolicy.RetentionMode),
			"journal_archive_mode":        string(o.cfg.journalPolicy.ArchiveMode),
		},
	})
	if len(executionImage.BuildInfo) > 0 {
		_ = o.store.AppendJournal(ctx, model.JournalEntry{
			LoopID:        intent.LoopID,
			Phase:         "core",
			Level:         "info",
			ActorType:     "core",
			ActorID:       o.cfg.holderID,
			Message:       "dockerfile build metadata",
			CorrelationID: next.CorrelationID,
			Metadata:      copyMap(executionImage.BuildInfo),
		})
	}
	return nil
}

func (o *orchestrator) createReplicaJob(ctx context.Context, loopID, jobName, correlationID string, executionImage executionImageSelection, anomaly model.Anomaly, skillMounts []replica.SkillMount) error {
	if err := o.ensureSkillSourcesExist(ctx, skillMounts); err != nil {
		return err
	}
	labels := map[string]string{
		"smith.io/job-name": jobName,
	}
	if project := projectLabelValue(anomaly); project != "" {
		labels["smith.io/project"] = project
	}
	request := replica.JobRequest{
		Namespace:                 o.cfg.namespace,
		EtcdEndpoints:             append([]string(nil), o.cfg.etcdEndpoints...),
		LoopID:                    loopID,
		CorrelationID:             correlationID,
		ProviderID:                loopProviderFor(anomaly),
		InvocationMethod:          loopInvocationMethodFor(anomaly),
		SourceType:                anomaly.SourceType,
		SourceRef:                 anomaly.SourceRef,
		JobName:                   jobName,
		Labels:                    labels,
		ServiceAccountName:        o.cfg.replicaSA,
		Image:                     executionImage.Ref,
		ImagePullPolicy:           executionImage.PullPolicy,
		WorkspaceSeedImage:        workspaceSeedImageFor(anomaly, o.cfg),
		WorkspaceSeedPullPolicy:   workspaceSeedPullPolicyFor(anomaly, o.cfg),
		Git:                       gitContextFor(anomaly),
		SkillMounts:               skillMounts,
		GitPolicy:                 gitPolicyPtr(o.cfg.gitPolicy, o.cfg.gitPolicyConfig),
		EnableGitPolicyConfig:     o.cfg.gitPolicyConfig,
		JournalPolicy:             journalPolicyPtr(o.cfg.journalPolicy, o.cfg.journalPolicyConfig),
		EnableJournalPolicyConfig: o.cfg.journalPolicyConfig,
		HandoffConfigMapName:      handoffConfigMapName(loopID),
		RuntimeCredentialsValue:   strings.TrimSpace(o.cfg.runtimeCredentials),
		BackoffLimit:              o.jobBackoff,
		ActiveDeadlineSeconds:     int64(o.cfg.defaultPolicy.Timeout.Seconds()),
		TTLSecondsAfterFinished:   o.jobTTL,
	}
	prdPayload, hasWorkspacePRD, err := workspacePRDPayload(anomaly.Metadata)
	if err != nil {
		return err
	}
	if hasWorkspacePRD {
		request.PRDConfigMapName = workspacePRDConfigMapName(loopID)
		request.PRDConfigMapKey = defaultWorkspacePRDKey
		request.WorkspacePRDPath = workspacePRDPathFor(anomaly.Metadata)
	}
	generator := replica.NewJobGenerator(kubeJobsAPI{
		kube:      o.kube,
		namespace: o.cfg.namespace,
	})
	if err := o.ensureHandoffConfigMap(ctx, loopID, request.HandoffConfigMapName); err != nil {
		return err
	}
	if hasWorkspacePRD {
		if err := o.ensureWorkspacePRDConfigMap(ctx, loopID, request.PRDConfigMapName, request.PRDConfigMapKey, prdPayload); err != nil {
			return err
		}
	}
	_, err = generator.Submit(ctx, request)
	return err
}

func (o *orchestrator) ensureHandoffConfigMap(ctx context.Context, loopID, configMapName string) error {
	name := strings.TrimSpace(configMapName)
	if name == "" {
		return errors.New("handoff configmap name is required")
	}
	payload, err := o.handoffConfigMapPayload(ctx, loopID)
	if err != nil {
		return err
	}
	client := o.kube.CoreV1().ConfigMaps(o.cfg.namespace)
	existing, err := client.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, createErr := client.Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: o.cfg.namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":      "smith",
					"app.kubernetes.io/component": "handoff",
				},
				Annotations: map[string]string{
					"smith.io/loop-id": loopID,
				},
			},
			Data: map[string]string{
				"latest.json": payload,
			},
		}, metav1.CreateOptions{})
		if createErr != nil {
			return fmt.Errorf("create handoff configmap %q: %w", name, createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get handoff configmap %q: %w", name, err)
	}
	if existing.Data != nil && existing.Data["latest.json"] == payload {
		return nil
	}
	next := existing.DeepCopy()
	if next.Data == nil {
		next.Data = map[string]string{}
	}
	next.Data["latest.json"] = payload
	if next.Annotations == nil {
		next.Annotations = map[string]string{}
	}
	next.Annotations["smith.io/loop-id"] = loopID
	if _, err := client.Update(ctx, next, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update handoff configmap %q: %w", name, err)
	}
	return nil
}

func (o *orchestrator) ensureWorkspacePRDConfigMap(ctx context.Context, loopID, configMapName, dataKey, payload string) error {
	name := strings.TrimSpace(configMapName)
	key := strings.TrimSpace(dataKey)
	body := strings.TrimSpace(payload)
	if name == "" {
		return errors.New("workspace prd configmap name is required")
	}
	if key == "" {
		return errors.New("workspace prd configmap key is required")
	}
	if body == "" {
		return errors.New("workspace prd payload is empty")
	}
	client := o.kube.CoreV1().ConfigMaps(o.cfg.namespace)
	existing, err := client.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, createErr := client.Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: o.cfg.namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":      "smith",
					"app.kubernetes.io/component": "workspace-prd",
				},
				Annotations: map[string]string{
					"smith.io/loop-id": loopID,
				},
			},
			Data: map[string]string{
				key: body,
			},
		}, metav1.CreateOptions{})
		if createErr != nil {
			return fmt.Errorf("create workspace prd configmap %q: %w", name, createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get workspace prd configmap %q: %w", name, err)
	}
	if existing.Data != nil && existing.Data[key] == body {
		return nil
	}
	next := existing.DeepCopy()
	if next.Data == nil {
		next.Data = map[string]string{}
	}
	next.Data[key] = body
	if next.Annotations == nil {
		next.Annotations = map[string]string{}
	}
	next.Annotations["smith.io/loop-id"] = loopID
	if _, err := client.Update(ctx, next, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update workspace prd configmap %q: %w", name, err)
	}
	return nil
}

func (o *orchestrator) handoffConfigMapPayload(ctx context.Context, loopID string) (string, error) {
	handoff, found, err := o.store.GetLatestHandoff(ctx, loopID)
	if err != nil {
		return "", fmt.Errorf("load latest handoff for %q: %w", loopID, err)
	}
	if found {
		out, err := json.Marshal(handoff)
		if err != nil {
			return "", fmt.Errorf("marshal latest handoff for %q: %w", loopID, err)
		}
		return string(out), nil
	}
	out, err := json.Marshal(map[string]string{"loop_id": loopID})
	if err != nil {
		return "", fmt.Errorf("marshal bootstrap handoff payload for %q: %w", loopID, err)
	}
	return string(out), nil
}

func (o *orchestrator) ensureSkillSourcesExist(ctx context.Context, mounts []replica.SkillMount) error {
	for _, mount := range mounts {
		configMapName := skillSourceConfigMapName(mount.Source)
		if configMapName == "" {
			return fmt.Errorf("invalid skill source %q", mount.Source)
		}
		_, err := o.kube.CoreV1().ConfigMaps(o.cfg.namespace).Get(ctx, configMapName, metav1.GetOptions{})
		if err == nil {
			continue
		}
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("skill %q source %q is not available (missing configmap %q)", mount.Name, mount.Source, configMapName)
		}
		return fmt.Errorf("failed to resolve skill %q source %q: %w", mount.Name, mount.Source, err)
	}
	return nil
}

func resolveExecutionImageSelection(ctx context.Context, anomaly *model.Anomaly, cfg config, loopID string) (executionImageSelection, error) {
	ref := strings.TrimSpace(cfg.replicaImage)
	pullPolicy := strings.TrimSpace(cfg.replicaPullPolicy)
	source := "core_default"

	if anomaly != nil && anomaly.Environment.Dockerfile != nil {
		return resolveDockerfileExecutionImage(ctx, cfg, *anomaly.Environment.Dockerfile, loopID)
	}
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
	}, nil
}

func workspaceSeedImageFor(anomaly model.Anomaly, cfg config) string {
	if image := strings.TrimSpace(anomaly.Metadata["workspace_seed_image"]); image != "" {
		return image
	}
	return strings.TrimSpace(cfg.workspaceSeedImage)
}

func workspaceSeedPullPolicyFor(anomaly model.Anomaly, cfg config) string {
	if policy := strings.TrimSpace(anomaly.Metadata["workspace_seed_pull_policy"]); policy != "" {
		return policy
	}
	return strings.TrimSpace(cfg.workspaceSeedPolicy)
}

func resolveDockerfileExecutionImage(ctx context.Context, cfg config, profile model.DockerfileProfile, loopID string) (executionImageSelection, error) {
	if err := validateDockerfileBuildInputs(profile); err != nil {
		return executionImageSelection{}, err
	}
	repo := strings.TrimSpace(cfg.dockerfileRepo)
	if repo == "" {
		repo = defaultDockerfileRepo(cfg.replicaImage)
	}
	tag := dockerfileBuildTag(loopID, profile)
	ref := fmt.Sprintf("%s:%s", repo, tag)
	pullPolicy := strings.TrimSpace(cfg.replicaPullPolicy)
	if pullPolicy == "" {
		pullPolicy = string(corev1.PullIfNotPresent)
	}
	buildInfo := map[string]string{
		"build_mode":         "dockerfile",
		"build_repo":         repo,
		"build_tag":          tag,
		"build_context_dir":  profile.ContextDir,
		"build_dockerfile":   profile.DockerfilePath,
		"build_target":       profile.Target,
		"build_cache_status": "unknown",
	}
	if cfg.dockerfileBuild {
		start := time.Now()
		cacheHit, err := runDockerfileBuild(ctx, ref, profile)
		buildInfo["build_duration_ms"] = strconv.FormatInt(time.Since(start).Milliseconds(), 10)
		if cacheHit {
			buildInfo["build_cache_status"] = "hit"
		} else {
			buildInfo["build_cache_status"] = "miss"
		}
		if err != nil {
			return executionImageSelection{}, err
		}
	} else {
		return executionImageSelection{}, errors.New("loop dockerfile build path is disabled; set SMITH_DOCKERFILE_BUILD_ENABLED=true")
	}
	return executionImageSelection{
		Ref:        ref,
		PullPolicy: pullPolicy,
		Source:     "loop_environment_dockerfile",
		Digest:     parseImageDigest(ref),
		BuildInfo:  buildInfo,
	}, nil
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

func validateDockerfileBuildInputs(profile model.DockerfileProfile) error {
	if strings.TrimSpace(profile.ContextDir) == "" {
		return errors.New("environment.dockerfile.context_dir is required")
	}
	if strings.TrimSpace(profile.DockerfilePath) == "" {
		return errors.New("environment.dockerfile.dockerfile_path is required")
	}
	if pathIsUnsafe(profile.ContextDir) {
		return fmt.Errorf("environment.dockerfile.context_dir is unsafe: %q", profile.ContextDir)
	}
	if pathIsUnsafe(profile.DockerfilePath) {
		return fmt.Errorf("environment.dockerfile.dockerfile_path is unsafe: %q", profile.DockerfilePath)
	}
	return nil
}

func pathIsUnsafe(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" {
		return true
	}
	if strings.HasPrefix(p, "/") {
		return true
	}
	parts := strings.Split(p, "/")
	for _, part := range parts {
		if part == ".." {
			return true
		}
	}
	return false
}

func runDockerfileBuild(ctx context.Context, ref string, profile model.DockerfileProfile) (bool, error) {
	args := []string{"build", "-f", profile.DockerfilePath, "-t", ref}
	if strings.TrimSpace(profile.Target) != "" {
		args = append(args, "--target", strings.TrimSpace(profile.Target))
	}
	keys := make([]string, 0, len(profile.BuildArgs))
	for key := range profile.BuildArgs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", key, profile.BuildArgs[key]))
	}
	args = append(args, profile.ContextDir)
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	cacheHit := strings.Contains(string(output), "CACHED")
	if err != nil {
		return cacheHit, fmt.Errorf("dockerfile build failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return cacheHit, nil
}

func dockerfileBuildTag(loopID string, profile model.DockerfileProfile) string {
	hashInput := strings.ToLower(strings.TrimSpace(loopID)) + "|" + strings.TrimSpace(profile.ContextDir) + "|" + strings.TrimSpace(profile.DockerfilePath) + "|" + strings.TrimSpace(profile.Target)
	keys := make([]string, 0, len(profile.BuildArgs))
	for key := range profile.BuildArgs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		hashInput += "|" + key + "=" + profile.BuildArgs[key]
	}
	digest := sha256.Sum256([]byte(hashInput))
	return "loop-" + hex.EncodeToString(digest[:6])
}

func defaultDockerfileRepo(replicaImage string) string {
	ref := strings.TrimSpace(replicaImage)
	if ref == "" {
		return "smith/replica-loop"
	}
	if idx := strings.IndexAny(ref, ":@"); idx >= 0 {
		ref = ref[:idx]
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "smith/replica-loop"
	}
	return ref + "-loop"
}

func gitContextFor(anomaly model.Anomaly) replica.GitContext {
	repo := strings.TrimSpace(anomaly.Metadata["github_repository"])
	if repo == "" && anomaly.SourceType == "github_issue" {
		ref := strings.TrimSpace(anomaly.SourceRef)
		if parts := strings.SplitN(ref, "#", 2); len(parts) > 0 {
			repo = strings.TrimSpace(parts[0])
		}
	}
	if repo == "" {
		repo = "unknown"
	}
	branch := strings.TrimSpace(anomaly.Metadata["git_branch"])
	if branch == "" {
		branch = "main"
	}
	commit := strings.TrimSpace(anomaly.Metadata["git_commit_sha"])
	if commit == "" {
		commit = "unknown"
	}
	return replica.GitContext{
		Repository: repo,
		Branch:     branch,
		CommitSHA:  commit,
	}
}

func projectLabelValue(anomaly model.Anomaly) string {
	if len(anomaly.Metadata) == 0 {
		return ""
	}
	for _, key := range []string{"project_id", "project_name", "github_repository"} {
		if value := strings.TrimSpace(anomaly.Metadata[key]); value != "" {
			return value
		}
	}
	return ""
}

func loopInvocationMethodFor(anomaly model.Anomaly) string {
	if method := strings.TrimSpace(anomaly.Metadata["invocation_method"]); method != "" {
		return method
	}
	if ingress := strings.TrimSpace(anomaly.Metadata["ingress_mode"]); ingress != "" {
		return ingress
	}
	if sourceType := strings.TrimSpace(anomaly.SourceType); sourceType != "" {
		return sourceType
	}
	return "unknown"
}

func loopProviderFor(anomaly model.Anomaly) string {
	if provider := strings.ToLower(strings.TrimSpace(anomaly.ProviderID)); provider != "" {
		return provider
	}
	if provider := strings.ToLower(strings.TrimSpace(anomaly.Metadata["workspace_provider"])); provider != "" {
		return provider
	}
	if provider := strings.ToLower(strings.TrimSpace(anomaly.Metadata["workspace_agent"])); provider != "" {
		return provider
	}
	return model.DefaultProviderID
}

func handoffConfigMapName(loopID string) string {
	base := strings.NewReplacer("/", "-", "_", "-", ".", "-", " ", "-").Replace(strings.ToLower(loopID))
	base = strings.Trim(base, "-")
	if base == "" {
		base = "loop"
	}
	if len(base) > 40 {
		base = base[:40]
	}
	return "handoff-" + base
}

func workspacePRDConfigMapName(loopID string) string {
	base := strings.NewReplacer("/", "-", "_", "-", ".", "-", " ", "-").Replace(strings.ToLower(loopID))
	base = strings.Trim(base, "-")
	if base == "" {
		base = "loop"
	}
	if len(base) > 44 {
		base = base[:44]
	}
	return "workspace-prd-" + base
}

func workspacePRDPayload(metadata map[string]string) (string, bool, error) {
	if len(metadata) == 0 {
		return "", false, nil
	}
	raw := strings.TrimSpace(metadata["workspace_prd_json"])
	if raw == "" {
		return "", false, nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", false, errors.New("workspace_prd_json is not valid json")
	}
	obj, ok := parsed.(map[string]any)
	if !ok {
		return "", false, errors.New("workspace_prd_json must be a json object")
	}
	stories, ok := obj["stories"].([]any)
	if !ok || len(stories) == 0 {
		return "", false, errors.New("workspace_prd_json must include a non-empty stories array")
	}
	normalized, err := json.Marshal(parsed)
	if err != nil {
		return "", false, fmt.Errorf("marshal workspace prd payload: %w", err)
	}
	return string(normalized), true, nil
}

func workspacePRDPathFor(metadata map[string]string) string {
	if len(metadata) == 0 {
		return defaultWorkspacePRDPath
	}
	candidate := strings.TrimSpace(strings.ReplaceAll(metadata["workspace_prd_path"], "\\", "/"))
	if candidate == "" {
		return defaultWorkspacePRDPath
	}
	if strings.HasPrefix(candidate, "/") {
		return defaultWorkspacePRDPath
	}
	cleaned := path.Clean(candidate)
	if cleaned == "." || cleaned == ".." {
		return defaultWorkspacePRDPath
	}
	if strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return defaultWorkspacePRDPath
	}
	return cleaned
}

func resolveSkillMounts(anomaly *model.Anomaly) ([]replica.SkillMount, []string, error) {
	if anomaly == nil || len(anomaly.Skills) == 0 {
		return nil, nil, nil
	}
	mounts := make([]replica.SkillMount, 0, len(anomaly.Skills))
	names := make([]string, 0, len(anomaly.Skills))
	for _, skill := range anomaly.Skills {
		name := strings.TrimSpace(skill.Name)
		source := strings.TrimSpace(skill.Source)
		mountPath := strings.TrimSpace(skill.MountPath)
		if name == "" || source == "" || mountPath == "" {
			return nil, nil, fmt.Errorf("invalid skill definition in anomaly %s", anomaly.ID)
		}
		readOnly := true
		if skill.ReadOnly != nil {
			readOnly = *skill.ReadOnly
		}
		mounts = append(mounts, replica.SkillMount{
			Name:      name,
			Source:    source,
			Version:   strings.TrimSpace(skill.Version),
			MountPath: mountPath,
			ReadOnly:  readOnly,
		})
		names = append(names, name)
	}
	return mounts, names, nil
}

func skillSourceConfigMapName(source string) string {
	trimmed := strings.TrimSpace(source)
	if !strings.HasPrefix(trimmed, "local://skills/") {
		return ""
	}
	name := strings.TrimPrefix(trimmed, "local://skills/")
	name = strings.NewReplacer("/", "-", "_", "-", ".", "-", " ", "-").Replace(strings.ToLower(name))
	name = strings.Trim(name, "-")
	if name == "" {
		return ""
	}
	if len(name) > 45 {
		name = name[:45]
	}
	return "skill-" + name
}

type kubeJobsAPI struct {
	kube      kubernetes.Interface
	namespace string
}

func (k kubeJobsAPI) CreateJob(ctx context.Context, job replica.JobManifest) error {
	k8sJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      job.Metadata.Name,
			Namespace: k.namespace,
			Labels:    copyMap(job.Metadata.Labels),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            int32Ptr(job.Spec.BackoffLimit),
			ActiveDeadlineSeconds:   int64Ptr(job.Spec.ActiveDeadlineSeconds),
			TTLSecondsAfterFinished: int32Ptr(job.Spec.TTLSecondsAfterFinished),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: copyMap(job.Spec.Template.Metadata.Labels)},
				Spec: corev1.PodSpec{
					ServiceAccountName: job.Spec.Template.Spec.ServiceAccountName,
					RestartPolicy:      corev1.RestartPolicy(job.Spec.Template.Spec.RestartPolicy),
					Volumes:            toK8sVolumes(job.Spec.Template.Spec.Volumes),
					InitContainers:     toK8sContainers(job.Spec.Template.Spec.InitContainers),
					Containers:         toK8sContainers(job.Spec.Template.Spec.Containers),
				},
			},
		},
	}
	_, err := k.kube.BatchV1().Jobs(k.namespace).Create(ctx, k8sJob, metav1.CreateOptions{})
	return err
}

func (k kubeJobsAPI) DeleteJob(ctx context.Context, namespace string, name string) error {
	targetNS := strings.TrimSpace(namespace)
	if targetNS == "" {
		targetNS = k.namespace
	}
	return k.kube.BatchV1().Jobs(targetNS).Delete(ctx, name, metav1.DeleteOptions{})
}

func toK8sVolumes(volumes []replica.Volume) []corev1.Volume {
	out := make([]corev1.Volume, 0, len(volumes))
	for _, v := range volumes {
		volume := corev1.Volume{Name: v.Name}
		if v.EmptyDir {
			volume.VolumeSource = corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}
			out = append(out, volume)
			continue
		}
		optional := v.Optional
		volume.VolumeSource = corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: v.ConfigMapName},
				Optional:             &optional,
			},
		}
		out = append(out, volume)
	}
	return out
}

func toK8sContainers(containers []replica.Container) []corev1.Container {
	out := make([]corev1.Container, 0, len(containers))
	for _, c := range containers {
		out = append(out, corev1.Container{
			Name:            c.Name,
			Image:           c.Image,
			ImagePullPolicy: corev1.PullPolicy(c.ImagePullPolicy),
			Command:         append([]string{}, c.Command...),
			Env:             toK8sEnv(c.Env),
			VolumeMounts:    toK8sVolumeMounts(c.VolumeMounts),
		})
	}
	return out
}

func toK8sEnv(in []replica.EnvVar) []corev1.EnvVar {
	out := make([]corev1.EnvVar, 0, len(in))
	for _, env := range in {
		item := corev1.EnvVar{Name: env.Name, Value: env.Value}
		if env.SecretKeyRef != nil {
			item.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: env.SecretKeyRef.Name},
					Key:                  env.SecretKeyRef.Key,
				},
			}
		}
		out = append(out, item)
	}
	return out
}

func toK8sVolumeMounts(in []replica.VolumeMount) []corev1.VolumeMount {
	out := make([]corev1.VolumeMount, 0, len(in))
	for _, mount := range in {
		out = append(out, corev1.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			ReadOnly:  mount.ReadOnly,
		})
	}
	return out
}

func copyMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func int32Ptr(v int32) *int32 { return &v }

func int64Ptr(v int64) *int64 { return &v }

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
		port:                envInt("SMITH_CORE_PORT", defaultPort),
		etcdEndpoints:       endpoints,
		etcdDialTimeout:     envDuration("SMITH_ETCD_DIAL_TIMEOUT", 5*time.Second),
		namespace:           envString("SMITH_NAMESPACE", "default"),
		holderID:            holderID,
		replicaImage:        envString("SMITH_REPLICA_IMAGE", "ghcr.io/smith/replica:v0.1.0"),
		replicaPullPolicy:   envString("SMITH_REPLICA_IMAGE_PULL_POLICY", string(corev1.PullIfNotPresent)),
		workspaceSeedImage:  strings.TrimSpace(os.Getenv("SMITH_WORKSPACE_SEED_IMAGE")),
		workspaceSeedPolicy: envString("SMITH_WORKSPACE_SEED_IMAGE_PULL_POLICY", string(corev1.PullIfNotPresent)),
		runtimeCredentials:  strings.TrimSpace(os.Getenv("SMITH_RUNTIME_CREDENTIALS")),
		dockerfileRepo:      strings.TrimSpace(os.Getenv("SMITH_DOCKERFILE_IMAGE_REPOSITORY")),
		dockerfileBuild:     envBool("SMITH_DOCKERFILE_BUILD_ENABLED", false),
		gitPolicy:           gitpolicy.DefaultPolicy(),
		gitPolicyConfig:     envBool("SMITH_GIT_POLICY_CONFIG_ENABLED", false),
		journalPolicy:       journalpolicy.DefaultPolicy(),
		journalPolicyConfig: envBool("SMITH_JOURNAL_POLICY_CONFIG_ENABLED", false),
		replicaSA:           envString("SMITH_REPLICA_TEMPLATE_SERVICE_ACCOUNT", "default"),
		defaultPolicy: model.LoopPolicy{
			MaxAttempts:      envInt("SMITH_LOOP_POLICY_MAX_ATTEMPTS", 3),
			BackoffInitial:   envDuration("SMITH_LOOP_POLICY_BACKOFF_INITIAL", 5*time.Second),
			BackoffMax:       envDuration("SMITH_LOOP_POLICY_BACKOFF_MAX", 2*time.Minute),
			Timeout:          envDuration("SMITH_LOOP_POLICY_TIMEOUT", 30*time.Minute),
			TerminateOnError: envBool("SMITH_LOOP_POLICY_TERMINATE_ON_ERROR", false),
		},
	}
	if cfg.gitPolicyConfig {
		cfg.gitPolicy.BranchCleanup = gitpolicy.BranchCleanupPolicy(envString("SMITH_GIT_POLICY_BRANCH_CLEANUP", string(cfg.gitPolicy.BranchCleanup)))
		cfg.gitPolicy.ConflictPolicy = gitpolicy.ConflictPolicy(envString("SMITH_GIT_POLICY_CONFLICT_POLICY", string(cfg.gitPolicy.ConflictPolicy)))
		cfg.gitPolicy.DeleteBranchOnMerge = envBool("SMITH_GIT_POLICY_DELETE_BRANCH_ON_MERGE", cfg.gitPolicy.DeleteBranchOnMerge)
	}
	if err := cfg.gitPolicy.Validate(); err != nil {
		return config{}, fmt.Errorf("invalid git policy configuration: %w", err)
	}
	if cfg.journalPolicyConfig {
		cfg.journalPolicy.RetentionMode = journalpolicy.RetentionMode(envString("SMITH_JOURNAL_RETENTION_MODE", string(cfg.journalPolicy.RetentionMode)))
		retentionTTLFallback := ""
		if cfg.journalPolicy.RetentionTTL > 0 {
			retentionTTLFallback = cfg.journalPolicy.RetentionTTL.String()
		}
		cfg.journalPolicy.RetentionTTL = envDurationAllowZero("SMITH_JOURNAL_RETENTION_TTL", retentionTTLFallback, cfg.journalPolicy.RetentionTTL)
		cfg.journalPolicy.ArchiveMode = journalpolicy.ArchiveMode(envString("SMITH_JOURNAL_ARCHIVE_MODE", string(cfg.journalPolicy.ArchiveMode)))
		cfg.journalPolicy.ArchiveBucket = strings.TrimSpace(os.Getenv("SMITH_JOURNAL_ARCHIVE_BUCKET"))
	}
	if err := cfg.journalPolicy.Validate(); err != nil {
		return config{}, fmt.Errorf("invalid journal policy configuration: %w", err)
	}
	return cfg, nil
}

func gitPolicyPtr(policy gitpolicy.Policy, enabled bool) *gitpolicy.Policy {
	if !enabled {
		return nil
	}
	p := policy
	return &p
}

func journalPolicyPtr(policy journalpolicy.Policy, enabled bool) *journalpolicy.Policy {
	if !enabled {
		return nil
	}
	p := policy
	return &p
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

func envDurationAllowZero(name, fallbackRaw string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		value = strings.TrimSpace(fallbackRaw)
	}
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
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
