package replica

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"

	"smith/internal/source/gitpolicy"
	"smith/internal/source/journalpolicy"
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
	UserName   string
	UserEmail  string
}

type GitAuthProvider string

const (
	GitAuthProviderPAT       GitAuthProvider = "pat"
	GitAuthProviderGitHubApp GitAuthProvider = "github_app"
	GitAuthProviderSSH       GitAuthProvider = "ssh"
)

type GitHubAppAuth struct {
	AppID                string
	InstallationID       string
	PrivateKeySecretName string
	PrivateKeySecretKey  string
}

type GitAuthConfig struct {
	Provider            GitAuthProvider
	PATSecretName       string
	PATSecretKey        string
	GitHubApp           *GitHubAppAuth
	SSH                 *SSHAuth
	EnableGitHubAppAuth bool
	EnableSSHAuth       bool
}

type SSHAuth struct {
	PrivateKeySecretName string
	PrivateKeySecretKey  string
	KnownHostsSecretName string
	KnownHostsSecretKey  string
}

type JobRequest struct {
	Namespace                 string
	EtcdEndpoints             []string
	LoopID                    string
	CorrelationID             string
	ProviderID                string
	InvocationMethod          string
	SourceType                string
	SourceRef                 string
	JobName                   string
	Labels                    map[string]string
	ServiceAccountName        string
	Image                     string
	ImagePullPolicy           string
	WorkspaceSeedImage        string
	WorkspaceSeedPullPolicy   string
	Git                       GitContext
	SkillMounts               []SkillMount
	GitPolicy                 *gitpolicy.Policy
	EnableGitPolicyConfig     bool
	JournalPolicy             *journalpolicy.Policy
	EnableJournalPolicyConfig bool
	GitAuth                   *GitAuthConfig
	HandoffConfigMapName      string
	PRDConfigMapName          string
	PRDConfigMapKey           string
	WorkspacePRDPath          string
	RuntimeSecretName         string
	RuntimeCredentialsKey     string
	RuntimeCredentialsValue   string
	BackoffLimit              int32
	ActiveDeadlineSeconds     int64
	TTLSecondsAfterFinished   int32
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
	InitContainers     []Container
	Containers         []Container
}

type Volume struct {
	Name          string
	ConfigMapName string
	Optional      bool
	EmptyDir      bool
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

type SkillMount struct {
	Name      string
	Source    string
	Version   string
	MountPath string
	ReadOnly  bool
}

func NewJobGenerator(jobs JobsAPI) *JobGenerator {
	return &JobGenerator{jobs: jobs}
}

func BuildReplicaJob(req JobRequest) (JobManifest, error) {
	if strings.TrimSpace(req.PRDConfigMapName) != "" {
		if strings.TrimSpace(req.PRDConfigMapKey) == "" {
			req.PRDConfigMapKey = "prd.json"
		}
		if strings.TrimSpace(req.WorkspacePRDPath) == "" {
			req.WorkspacePRDPath = ".agents/tasks/prd.json"
		}
	}
	if err := validateRequest(req); err != nil {
		return JobManifest{}, err
	}
	if req.ImagePullPolicy == "" {
		req.ImagePullPolicy = "IfNotPresent"
	}

	jobName := strings.TrimSpace(req.JobName)
	if jobName == "" {
		jobName = fmt.Sprintf("smith-replica-%s", sanitizeName(req.LoopID))
	}
	labels := map[string]string{
		"app.kubernetes.io/name":      "smith-replica",
		"app.kubernetes.io/component": "replica",
		"smith.io/loop-id":            sanitizeKubernetesLabelValue(req.LoopID),
		"smith.io/correlation-id":     sanitizeKubernetesLabelValue(req.CorrelationID),
	}
	for key, raw := range req.Labels {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		labels[k] = sanitizeKubernetesLabelValue(raw)
	}

	env := []EnvVar{
		{Name: "STORY_ID", Value: req.LoopID},
		{Name: "SMITH_LOOP_ID", Value: req.LoopID},
		{Name: "SMITH_CORRELATION_ID", Value: req.CorrelationID},
		{Name: "SMITH_ETCD_ENDPOINTS", Value: strings.Join(sanitizeEtcdEndpoints(req.EtcdEndpoints), ",")},
		{Name: "SMITH_LOOP_PROVIDER", Value: strings.ToLower(strings.TrimSpace(req.ProviderID))},
		{Name: "SMITH_LOOP_INVOCATION_METHOD", Value: strings.TrimSpace(req.InvocationMethod)},
		{Name: "SMITH_LOOP_SOURCE_TYPE", Value: strings.TrimSpace(req.SourceType)},
		{Name: "SMITH_LOOP_SOURCE_REF", Value: strings.TrimSpace(req.SourceRef)},
		{Name: "SMITH_GIT_REPOSITORY", Value: req.Git.Repository},
		{Name: "SMITH_GIT_BRANCH", Value: req.Git.Branch},
		{Name: "SMITH_GIT_COMMIT_SHA", Value: req.Git.CommitSHA},
		{Name: "SMITH_GIT_USER_NAME", Value: req.Git.UserName},
		{Name: "SMITH_GIT_USER_EMAIL", Value: req.Git.UserEmail},
		{Name: "SMITH_HANDOFF_PATH", Value: "/smith/handoff/latest.json"},
	}
	if strings.TrimSpace(req.RuntimeCredentialsValue) != "" {
		env = append(env,
			EnvVar{Name: "SMITH_RUNTIME_CREDENTIALS", Value: req.RuntimeCredentialsValue},
			EnvVar{Name: "OPENAI_API_KEY", Value: req.RuntimeCredentialsValue},
		)
	} else if req.RuntimeSecretName != "" {
		env = append(env, EnvVar{
			Name: "SMITH_RUNTIME_CREDENTIALS",
			SecretKeyRef: &SecretKeyRef{
				Name: req.RuntimeSecretName,
				Key:  req.RuntimeCredentialsKey,
			},
		})
		env = append(env, EnvVar{
			Name: "OPENAI_API_KEY",
			SecretKeyRef: &SecretKeyRef{
				Name: req.RuntimeSecretName,
				Key:  req.RuntimeCredentialsKey,
			},
		})
	}
	if req.GitPolicy != nil {
		env = append(env,
			EnvVar{Name: "SMITH_GIT_POLICY_BRANCH_CLEANUP", Value: string(req.GitPolicy.BranchCleanup)},
			EnvVar{Name: "SMITH_GIT_POLICY_CONFLICT_POLICY", Value: string(req.GitPolicy.ConflictPolicy)},
			EnvVar{Name: "SMITH_GIT_POLICY_DELETE_BRANCH_ON_MERGE", Value: fmt.Sprintf("%t", req.GitPolicy.DeleteBranchOnMerge)},
		)
	}
	if req.JournalPolicy != nil {
		env = append(env,
			EnvVar{Name: "SMITH_JOURNAL_RETENTION_MODE", Value: string(req.JournalPolicy.RetentionMode)},
			EnvVar{Name: "SMITH_JOURNAL_RETENTION_TTL", Value: req.JournalPolicy.RetentionTTL.String()},
			EnvVar{Name: "SMITH_JOURNAL_ARCHIVE_MODE", Value: string(req.JournalPolicy.ArchiveMode)},
		)
		if strings.TrimSpace(req.JournalPolicy.ArchiveBucket) != "" {
			env = append(env, EnvVar{Name: "SMITH_JOURNAL_ARCHIVE_BUCKET", Value: req.JournalPolicy.ArchiveBucket})
		}
	}
	if req.GitAuth != nil {
		env = append(env, EnvVar{Name: "SMITH_GIT_AUTH_PROVIDER", Value: string(req.GitAuth.Provider)})
		switch req.GitAuth.Provider {
		case GitAuthProviderPAT:
			env = append(env, EnvVar{
				Name: "SMITH_GIT_PAT",
				SecretKeyRef: &SecretKeyRef{
					Name: req.GitAuth.PATSecretName,
					Key:  req.GitAuth.PATSecretKey,
				},
			})
		case GitAuthProviderGitHubApp:
			env = append(env,
				EnvVar{Name: "SMITH_GITHUB_APP_ID", Value: req.GitAuth.GitHubApp.AppID},
				EnvVar{Name: "SMITH_GITHUB_APP_INSTALLATION_ID", Value: req.GitAuth.GitHubApp.InstallationID},
				EnvVar{
					Name: "SMITH_GITHUB_APP_PRIVATE_KEY",
					SecretKeyRef: &SecretKeyRef{
						Name: req.GitAuth.GitHubApp.PrivateKeySecretName,
						Key:  req.GitAuth.GitHubApp.PrivateKeySecretKey,
					},
				},
			)
		case GitAuthProviderSSH:
			env = append(env,
				EnvVar{
					Name: "SMITH_GIT_SSH_PRIVATE_KEY",
					SecretKeyRef: &SecretKeyRef{
						Name: req.GitAuth.SSH.PrivateKeySecretName,
						Key:  req.GitAuth.SSH.PrivateKeySecretKey,
					},
				},
			)
			if strings.TrimSpace(req.GitAuth.SSH.KnownHostsSecretName) != "" && strings.TrimSpace(req.GitAuth.SSH.KnownHostsSecretKey) != "" {
				env = append(env, EnvVar{
					Name: "SMITH_GIT_SSH_KNOWN_HOSTS",
					SecretKeyRef: &SecretKeyRef{
						Name: req.GitAuth.SSH.KnownHostsSecretName,
						Key:  req.GitAuth.SSH.KnownHostsSecretKey,
					},
				})
			}
		}
	}

	volumes := []Volume{
		{
			Name:     "workspace",
			EmptyDir: true,
		},
		{
			Name:          "handoff",
			ConfigMapName: req.HandoffConfigMapName,
			Optional:      false,
		},
	}

	volumeMounts := []VolumeMount{
		{
			Name:      "workspace",
			MountPath: "/workspace",
			ReadOnly:  false,
		},
		{
			Name:      "handoff",
			MountPath: "/smith/handoff",
			ReadOnly:  true,
		},
	}
	hasPRDConfig := strings.TrimSpace(req.PRDConfigMapName) != ""
	if hasPRDConfig {
		volumes = append(volumes, Volume{
			Name:          "workspace-prd",
			ConfigMapName: req.PRDConfigMapName,
			Optional:      false,
		})
	}
	resolvedSkillNames := make([]string, 0, len(req.SkillMounts))
	for i, skill := range req.SkillMounts {
		volumeName := fmt.Sprintf("skill-%d-%s", i, sanitizeName(skill.Name))
		volumes = append(volumes, Volume{
			Name:          volumeName,
			ConfigMapName: skillConfigMapName(skill.Source),
			Optional:      false,
		})
		volumeMounts = append(volumeMounts, VolumeMount{
			Name:      volumeName,
			MountPath: skill.MountPath,
			ReadOnly:  skill.ReadOnly,
		})
		resolvedSkillNames = append(resolvedSkillNames, skill.Name)
	}
	if len(req.SkillMounts) > 0 {
		env = append(env,
			EnvVar{Name: "SMITH_SKILL_MOUNT_COUNT", Value: fmt.Sprintf("%d", len(req.SkillMounts))},
			EnvVar{Name: "SMITH_SKILL_MOUNTS", Value: strings.Join(resolvedSkillNames, ",")},
		)
	}
	initContainers := []Container{}
	if seedImage := strings.TrimSpace(req.WorkspaceSeedImage); seedImage != "" {
		pullPolicy := strings.TrimSpace(req.WorkspaceSeedPullPolicy)
		if pullPolicy == "" {
			pullPolicy = req.ImagePullPolicy
		}
		initContainers = append(initContainers, Container{
			Name:            "workspace-seed",
			Image:           seedImage,
			ImagePullPolicy: pullPolicy,
			Command: []string{
				"sh",
				"-lc",
				"set -eu; mkdir -p /workspace; if [ -d /seed ]; then cp -a /seed/. /workspace/; fi",
			},
			VolumeMounts: []VolumeMount{
				{
					Name:      "workspace",
					MountPath: "/workspace",
					ReadOnly:  false,
				},
			},
		})
	}
	if hasPRDConfig {
		targetPath := workspacePRDAbsolutePath(req.WorkspacePRDPath)
		targetDir := path.Dir(targetPath)
		sourcePath := "/smith/prd/" + strings.TrimSpace(req.PRDConfigMapKey)
		initContainers = append(initContainers, Container{
			Name:            "workspace-prd",
			Image:           req.Image,
			ImagePullPolicy: req.ImagePullPolicy,
			Command: []string{
				"sh",
				"-lc",
				"set -eu; mkdir -p " + shellQuote(targetDir) + "; cp " + shellQuote(sourcePath) + " " + shellQuote(targetPath),
			},
			VolumeMounts: []VolumeMount{
				{
					Name:      "workspace",
					MountPath: "/workspace",
					ReadOnly:  false,
				},
				{
					Name:      "workspace-prd",
					MountPath: "/smith/prd",
					ReadOnly:  true,
				},
			},
		})
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
					InitContainers:     initContainers,
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
	case len(sanitizeEtcdEndpoints(req.EtcdEndpoints)) == 0:
		return fmt.Errorf("%w: at least one etcd endpoint is required", ErrInvalidJobRequest)
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
	if strings.TrimSpace(req.PRDConfigMapName) != "" {
		if strings.TrimSpace(req.PRDConfigMapKey) == "" {
			return fmt.Errorf("%w: prd configmap key is required when prd configmap is set", ErrInvalidJobRequest)
		}
		if strings.Contains(strings.TrimSpace(req.PRDConfigMapKey), "/") {
			return fmt.Errorf("%w: prd configmap key must not contain '/'", ErrInvalidJobRequest)
		}
		if workspacePathUnsafe(req.WorkspacePRDPath) {
			return fmt.Errorf("%w: workspace prd path is unsafe", ErrInvalidJobRequest)
		}
	}

	if req.BackoffLimit < 0 || req.ActiveDeadlineSeconds <= 0 || req.TTLSecondsAfterFinished < 0 {
		return fmt.Errorf("%w: invalid retry/timeout settings", ErrInvalidJobRequest)
	}
	for _, skill := range req.SkillMounts {
		if strings.TrimSpace(skill.Name) == "" {
			return fmt.Errorf("%w: skill mount name is required", ErrInvalidJobRequest)
		}
		if strings.TrimSpace(skill.Source) == "" {
			return fmt.Errorf("%w: skill mount source is required", ErrInvalidJobRequest)
		}
		if !strings.HasPrefix(strings.TrimSpace(skill.Source), "local://skills/") {
			return fmt.Errorf("%w: unsupported skill mount source %q", ErrInvalidJobRequest, skill.Source)
		}
		if strings.TrimSpace(skill.MountPath) == "" || !strings.HasPrefix(skill.MountPath, "/") {
			return fmt.Errorf("%w: skill mount path must be absolute", ErrInvalidJobRequest)
		}
	}
	if req.RuntimeSecretName != "" && strings.TrimSpace(req.RuntimeCredentialsKey) == "" {
		return fmt.Errorf("%w: runtime credentials key is required when runtime secret is set", ErrInvalidJobRequest)
	}
	if req.GitPolicy != nil {
		if !req.EnableGitPolicyConfig {
			return fmt.Errorf("%w: git policy overrides are feature-flagged; set EnableGitPolicyConfig to true", ErrInvalidJobRequest)
		}
		if err := req.GitPolicy.Validate(); err != nil {
			return fmt.Errorf("%w: invalid git policy: %v", ErrInvalidJobRequest, err)
		}
	}
	if req.JournalPolicy != nil {
		if !req.EnableJournalPolicyConfig {
			return fmt.Errorf("%w: journal policy overrides are feature-flagged; set EnableJournalPolicyConfig to true", ErrInvalidJobRequest)
		}
		if err := req.JournalPolicy.Validate(); err != nil {
			return fmt.Errorf("%w: invalid journal policy: %v", ErrInvalidJobRequest, err)
		}
	}
	if req.GitAuth == nil {
		return nil
	}
	switch req.GitAuth.Provider {
	case GitAuthProviderPAT:
		if strings.TrimSpace(req.GitAuth.PATSecretName) == "" || strings.TrimSpace(req.GitAuth.PATSecretKey) == "" {
			return fmt.Errorf("%w: git auth provider pat requires pat secret name and key", ErrInvalidJobRequest)
		}
	case GitAuthProviderGitHubApp:
		if !req.GitAuth.EnableGitHubAppAuth {
			return fmt.Errorf("%w: github_app auth provider is feature-flagged; set EnableGitHubAppAuth to true", ErrInvalidJobRequest)
		}
		if req.GitAuth.GitHubApp == nil {
			return fmt.Errorf("%w: github_app auth provider requires github app config", ErrInvalidJobRequest)
		}
		if strings.TrimSpace(req.GitAuth.GitHubApp.AppID) == "" || strings.TrimSpace(req.GitAuth.GitHubApp.InstallationID) == "" {
			return fmt.Errorf("%w: github_app auth requires app id and installation id", ErrInvalidJobRequest)
		}
		if strings.TrimSpace(req.GitAuth.GitHubApp.PrivateKeySecretName) == "" || strings.TrimSpace(req.GitAuth.GitHubApp.PrivateKeySecretKey) == "" {
			return fmt.Errorf("%w: github_app auth requires private key secret name and key", ErrInvalidJobRequest)
		}
	case GitAuthProviderSSH:
		if !req.GitAuth.EnableSSHAuth {
			return fmt.Errorf("%w: ssh auth provider is feature-flagged; set EnableSSHAuth to true", ErrInvalidJobRequest)
		}
		if req.GitAuth.SSH == nil {
			return fmt.Errorf("%w: ssh auth provider requires ssh config", ErrInvalidJobRequest)
		}
		if strings.TrimSpace(req.GitAuth.SSH.PrivateKeySecretName) == "" || strings.TrimSpace(req.GitAuth.SSH.PrivateKeySecretKey) == "" {
			return fmt.Errorf("%w: ssh auth requires private key secret name and key", ErrInvalidJobRequest)
		}
	default:
		return fmt.Errorf("%w: unsupported git auth provider %q", ErrInvalidJobRequest, req.GitAuth.Provider)
	}
	return nil
}

func sanitizeEtcdEndpoints(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, raw := range in {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func skillConfigMapName(source string) string {
	trimmed := strings.TrimSpace(source)
	name := strings.TrimPrefix(trimmed, "local://skills/")
	name = sanitizeName(name)
	if name == "" {
		name = "unknown"
	}
	return "skill-" + name
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

func sanitizeKubernetesLabelValue(raw string) string {
	const maxLen = 63
	s := strings.TrimSpace(raw)
	if s == "" {
		return "unknown"
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}

	out := strings.Trim(b.String(), "-_.")
	if out == "" {
		out = "unknown"
	}
	if len(out) <= maxLen {
		return out
	}
	sum := sha1.Sum([]byte(out))
	suffix := hex.EncodeToString(sum[:])[:8]
	keep := maxLen - 1 - len(suffix)
	if keep < 1 {
		keep = 1
	}
	out = strings.Trim(out[:keep], "-_.")
	if out == "" {
		out = "x"
	}
	return out + "-" + suffix
}

func workspacePathUnsafe(raw string) bool {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if value == "" {
		return true
	}
	if strings.HasPrefix(value, "/") {
		return true
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." {
		return true
	}
	if strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return true
	}
	return false
}

func workspacePRDAbsolutePath(raw string) string {
	relative := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if relative == "" || workspacePathUnsafe(relative) {
		relative = ".agents/tasks/prd.json"
	}
	cleaned := path.Clean(relative)
	cleaned = strings.TrimPrefix(cleaned, "./")
	return path.Join("/workspace", cleaned)
}

func shellQuote(raw string) string {
	escaped := strings.ReplaceAll(raw, `'`, `'\''`)
	return "'" + escaped + "'"
}
