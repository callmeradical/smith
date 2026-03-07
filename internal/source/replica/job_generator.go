package replica

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidJobRequest = errors.New("invalid replica job request")
	ErrSubmitFailed      = errors.New("replica job submit failed")
	ErrDeleteFailed      = errors.New("replica job delete failed")
)

type GitContext struct {
	Repository string
	Branch     string
	CommitSHA  string
}

type JobRequest struct {
	Namespace               string
	LoopID                  string
	CorrelationID           string
	ServiceAccountName      string
	Image                   string
	ImagePullPolicy         string
	Git                     GitContext
	HandoffConfigMapName    string
	RuntimeSecretName       string
	RuntimeCredentialsKey   string
	BackoffLimit            int32
	ActiveDeadlineSeconds   int64
	TTLSecondsAfterFinished int32
}

type JobManifest struct {
	APIVersion string
	Kind       string
	Metadata   ObjectMeta
	Spec       JobSpec
}

type ObjectMeta struct {
	Name      string
	Namespace string
	Labels    map[string]string
}

type JobSpec struct {
	BackoffLimit            int32
	ActiveDeadlineSeconds   int64
	TTLSecondsAfterFinished int32
	Template                PodTemplateSpec
}

type PodTemplateSpec struct {
	Metadata ObjectMeta
	Spec     PodSpec
}

type PodSpec struct {
	ServiceAccountName string
	RestartPolicy      string
	Volumes            []Volume
	Containers         []Container
}

type Volume struct {
	Name          string
	ConfigMapName string
}

type Container struct {
	Name            string
	Image           string
	ImagePullPolicy string
	Command         []string
	Env             []EnvVar
	VolumeMounts    []VolumeMount
}

type EnvVar struct {
	Name         string
	Value        string
	SecretKeyRef *SecretKeyRef
}

type SecretKeyRef struct {
	Name string
	Key  string
}

type VolumeMount struct {
	Name      string
	MountPath string
	ReadOnly  bool
}

type JobsAPI interface {
	CreateJob(ctx context.Context, job JobManifest) error
	DeleteJob(ctx context.Context, namespace string, name string) error
}

type JobGenerator struct {
	jobs JobsAPI
}

func NewJobGenerator(jobs JobsAPI) *JobGenerator {
	return &JobGenerator{jobs: jobs}
}

func BuildReplicaJob(req JobRequest) (JobManifest, error) {
	if err := validateRequest(req); err != nil {
		return JobManifest{}, err
	}
	if req.ImagePullPolicy == "" {
		req.ImagePullPolicy = "IfNotPresent"
	}

	jobName := fmt.Sprintf("smith-replica-%s", sanitizeName(req.LoopID))
	labels := map[string]string{
		"app.kubernetes.io/name":      "smith-replica",
		"app.kubernetes.io/component": "replica",
		"smith.io/loop-id":            req.LoopID,
		"smith.io/correlation-id":     req.CorrelationID,
	}

	env := []EnvVar{
		{Name: "STORY_ID", Value: req.LoopID},
		{Name: "SMITH_LOOP_ID", Value: req.LoopID},
		{Name: "SMITH_CORRELATION_ID", Value: req.CorrelationID},
		{Name: "SMITH_GIT_REPOSITORY", Value: req.Git.Repository},
		{Name: "SMITH_GIT_BRANCH", Value: req.Git.Branch},
		{Name: "SMITH_GIT_COMMIT_SHA", Value: req.Git.CommitSHA},
		{Name: "SMITH_HANDOFF_PATH", Value: "/smith/handoff/latest.json"},
	}
	if req.RuntimeSecretName != "" {
		env = append(env, EnvVar{
			Name: "SMITH_RUNTIME_CREDENTIALS",
			SecretKeyRef: &SecretKeyRef{
				Name: req.RuntimeSecretName,
				Key:  req.RuntimeCredentialsKey,
			},
		})
	}

	volumes := []Volume{
		{
			Name:          "handoff",
			ConfigMapName: req.HandoffConfigMapName,
		},
	}

	volumeMounts := []VolumeMount{
		{
			Name:      "handoff",
			MountPath: "/smith/handoff",
			ReadOnly:  true,
		},
	}

	return JobManifest{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Metadata: ObjectMeta{
			Name:      jobName,
			Namespace: req.Namespace,
			Labels:    labels,
		},
		Spec: JobSpec{
			BackoffLimit:            req.BackoffLimit,
			ActiveDeadlineSeconds:   req.ActiveDeadlineSeconds,
			TTLSecondsAfterFinished: req.TTLSecondsAfterFinished,
			Template: PodTemplateSpec{
				Metadata: ObjectMeta{
					Labels: labels,
				},
				Spec: PodSpec{
					ServiceAccountName: req.ServiceAccountName,
					RestartPolicy:      "Never",
					Volumes:            volumes,
					Containers: []Container{
						{
							Name:            "replica",
							Image:           req.Image,
							ImagePullPolicy: req.ImagePullPolicy,
							Command:         []string{"/bin/smith-replica"},
							Env:             env,
							VolumeMounts:    volumeMounts,
						},
					},
				},
			},
		},
	}, nil
}

func (g *JobGenerator) Submit(ctx context.Context, req JobRequest) (JobManifest, error) {
	job, err := BuildReplicaJob(req)
	if err != nil {
		return JobManifest{}, err
	}
	if err := g.jobs.CreateJob(ctx, job); err != nil {
		return JobManifest{}, fmt.Errorf("%w: %v", ErrSubmitFailed, err)
	}
	return job, nil
}

func (g *JobGenerator) Delete(ctx context.Context, namespace, name string) error {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: namespace and name are required", ErrInvalidJobRequest)
	}
	if err := g.jobs.DeleteJob(ctx, namespace, name); err != nil {
		return fmt.Errorf("%w: %v", ErrDeleteFailed, err)
	}
	return nil
}

func validateRequest(req JobRequest) error {
	switch {
	case strings.TrimSpace(req.Namespace) == "":
		return fmt.Errorf("%w: namespace is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.LoopID) == "":
		return fmt.Errorf("%w: loop id is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.CorrelationID) == "":
		return fmt.Errorf("%w: correlation id is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.ServiceAccountName) == "":
		return fmt.Errorf("%w: service account is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.Image) == "":
		return fmt.Errorf("%w: image is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.Git.Repository) == "":
		return fmt.Errorf("%w: git repository is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.Git.Branch) == "":
		return fmt.Errorf("%w: git branch is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.Git.CommitSHA) == "":
		return fmt.Errorf("%w: git commit sha is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.HandoffConfigMapName) == "":
		return fmt.Errorf("%w: handoff configmap is required", ErrInvalidJobRequest)
	}

	if req.BackoffLimit < 0 || req.ActiveDeadlineSeconds <= 0 || req.TTLSecondsAfterFinished < 0 {
		return fmt.Errorf("%w: invalid retry/timeout settings", ErrInvalidJobRequest)
	}
	if req.RuntimeSecretName != "" && strings.TrimSpace(req.RuntimeCredentialsKey) == "" {
		return fmt.Errorf("%w: runtime credentials key is required when runtime secret is set", ErrInvalidJobRequest)
	}
	return nil
}

func sanitizeName(loopID string) string {
	s := strings.ToLower(strings.TrimSpace(loopID))
	replacer := strings.NewReplacer(
		"/", "-",
		"_", "-",
		".", "-",
		" ", "-",
	)
	s = replacer.Replace(s)
	s = strings.Trim(s, "-")
	if s == "" {
		return "loop"
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}
