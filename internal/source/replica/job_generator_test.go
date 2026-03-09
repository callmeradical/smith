package replica

import (
	"context"
	"errors"
	"testing"
	"time"

	"smith/internal/source/gitpolicy"
	"smith/internal/source/journalpolicy"
)

func TestBuildReplicaJobIncludesRequiredContext(t *testing.T) {
	req := validRequest()

	job, err := BuildReplicaJob(req)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if job.Kind != "Job" {
		t.Fatalf("expected Job kind, got %q", job.Kind)
	}
	if job.Metadata.Namespace != "smith-system" {
		t.Fatalf("expected namespace smith-system, got %q", job.Metadata.Namespace)
	}
	if job.Metadata.Name != "smith-replica-loop-123" {
		t.Fatalf("unexpected generated job name %q", job.Metadata.Name)
	}
	if job.Spec.Template.Spec.ServiceAccountName != "smith-replica" {
		t.Fatalf("unexpected service account: %q", job.Spec.Template.Spec.ServiceAccountName)
	}
	if len(job.Spec.Template.Spec.Volumes) == 0 || job.Spec.Template.Spec.Volumes[0].ConfigMapName != "handoff-loop-123" {
		t.Fatalf("expected handoff volume wired from configmap, got %+v", job.Spec.Template.Spec.Volumes)
	}

	env := map[string]EnvVar{}
	for _, item := range job.Spec.Template.Spec.Containers[0].Env {
		env[item.Name] = item
	}

	required := []string{
		"STORY_ID",
		"SMITH_LOOP_ID",
		"SMITH_CORRELATION_ID",
		"SMITH_GIT_REPOSITORY",
		"SMITH_GIT_BRANCH",
		"SMITH_GIT_COMMIT_SHA",
		"SMITH_HANDOFF_PATH",
		"SMITH_SKILL_MOUNT_COUNT",
		"SMITH_SKILL_MOUNTS",
	}
	for _, key := range required {
		if _, ok := env[key]; !ok {
			t.Fatalf("missing required env %s", key)
		}
	}
	if env["STORY_ID"].Value != "loop-123" {
		t.Fatalf("expected STORY_ID=loop-123, got %q", env["STORY_ID"].Value)
	}
	secretEnv, ok := env["SMITH_RUNTIME_CREDENTIALS"]
	if !ok || secretEnv.SecretKeyRef == nil {
		t.Fatalf("expected SMITH_RUNTIME_CREDENTIALS secret key ref, got %+v", secretEnv)
	}
	if secretEnv.SecretKeyRef.Name != "smith-runtime" || secretEnv.SecretKeyRef.Key != "runtime_credentials" {
		t.Fatalf("unexpected secret ref %+v", secretEnv.SecretKeyRef)
	}
	if env["SMITH_SKILL_MOUNT_COUNT"].Value != "2" {
		t.Fatalf("expected SMITH_SKILL_MOUNT_COUNT=2, got %q", env["SMITH_SKILL_MOUNT_COUNT"].Value)
	}
	if env["SMITH_SKILL_MOUNTS"].Value != "commit,lint" {
		t.Fatalf("expected resolved skill names, got %q", env["SMITH_SKILL_MOUNTS"].Value)
	}
	volumes := map[string]Volume{}
	for _, v := range job.Spec.Template.Spec.Volumes {
		volumes[v.Name] = v
	}
	if volumes["skill-0-commit"].ConfigMapName != "skill-commit" {
		t.Fatalf("unexpected configmap for commit skill: %+v", volumes["skill-0-commit"])
	}
	if volumes["skill-1-lint"].ConfigMapName != "skill-lint" {
		t.Fatalf("unexpected configmap for lint skill: %+v", volumes["skill-1-lint"])
	}
	mounts := map[string]VolumeMount{}
	for _, m := range job.Spec.Template.Spec.Containers[0].VolumeMounts {
		mounts[m.Name] = m
	}
	if mounts["skill-0-commit"].MountPath != "/smith/skills/commit" || !mounts["skill-0-commit"].ReadOnly {
		t.Fatalf("unexpected commit mount: %+v", mounts["skill-0-commit"])
	}
	if mounts["skill-1-lint"].MountPath != "/smith/skills/lint" || mounts["skill-1-lint"].ReadOnly {
		t.Fatalf("unexpected lint mount: %+v", mounts["skill-1-lint"])
	}
}

func TestBuildReplicaJobSanitizesLabelsAndRespectsCustomJobName(t *testing.T) {
	req := validRequest()
	req.LoopID = "alpha/feat/issue-132-prd"
	req.CorrelationID = "corr/123"
	req.JobName = "smith-replica-alpha-feat-issue-132-99999"
	req.Labels = map[string]string{
		"smith.io/project": "pod-visualizer/demo",
		"smith.io/custom":  "!!!",
	}

	job, err := BuildReplicaJob(req)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if job.Metadata.Name != req.JobName {
		t.Fatalf("expected custom job name %q, got %q", req.JobName, job.Metadata.Name)
	}
	if got := job.Metadata.Labels["smith.io/loop-id"]; got != "alpha-feat-issue-132-prd" {
		t.Fatalf("unexpected loop label %q", got)
	}
	if got := job.Metadata.Labels["smith.io/correlation-id"]; got != "corr-123" {
		t.Fatalf("unexpected correlation label %q", got)
	}
	if got := job.Metadata.Labels["smith.io/project"]; got != "pod-visualizer-demo" {
		t.Fatalf("unexpected project label %q", got)
	}
	if got := job.Metadata.Labels["smith.io/custom"]; got != "unknown" {
		t.Fatalf("unexpected custom label %q", got)
	}
}

func TestBuildReplicaJobWithGitPolicyOverrides(t *testing.T) {
	req := validRequest()
	policy := gitpolicy.DefaultPolicy()
	policy.BranchCleanup = gitpolicy.BranchCleanupNever
	policy.DeleteBranchOnMerge = false
	policy.ConflictPolicy = gitpolicy.ConflictPolicyFailFast
	req.GitPolicy = &policy
	req.EnableGitPolicyConfig = true

	job, err := BuildReplicaJob(req)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	env := map[string]EnvVar{}
	for _, item := range job.Spec.Template.Spec.Containers[0].Env {
		env[item.Name] = item
	}
	if env["SMITH_GIT_POLICY_BRANCH_CLEANUP"].Value != "never" {
		t.Fatalf("unexpected branch cleanup env: %+v", env["SMITH_GIT_POLICY_BRANCH_CLEANUP"])
	}
	if env["SMITH_GIT_POLICY_CONFLICT_POLICY"].Value != "fail_fast" {
		t.Fatalf("unexpected conflict policy env: %+v", env["SMITH_GIT_POLICY_CONFLICT_POLICY"])
	}
	if env["SMITH_GIT_POLICY_DELETE_BRANCH_ON_MERGE"].Value != "false" {
		t.Fatalf("unexpected delete policy env: %+v", env["SMITH_GIT_POLICY_DELETE_BRANCH_ON_MERGE"])
	}
}

func TestBuildReplicaJobRejectsGitPolicyOverrideWhenFeatureDisabled(t *testing.T) {
	req := validRequest()
	policy := gitpolicy.DefaultPolicy()
	req.GitPolicy = &policy
	req.EnableGitPolicyConfig = false
	_, err := BuildReplicaJob(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrInvalidJobRequest) {
		t.Fatalf("expected ErrInvalidJobRequest, got %v", err)
	}
}

func TestBuildReplicaJobWithJournalPolicyOverrides(t *testing.T) {
	req := validRequest()
	policy := journalpolicy.Policy{
		RetentionMode: journalpolicy.RetentionTTL,
		RetentionTTL:  14 * 24 * time.Hour,
		ArchiveMode:   journalpolicy.ArchiveS3,
		ArchiveBucket: "smith-journal-archive",
	}
	req.JournalPolicy = &policy
	req.EnableJournalPolicyConfig = true

	job, err := BuildReplicaJob(req)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	env := map[string]EnvVar{}
	for _, item := range job.Spec.Template.Spec.Containers[0].Env {
		env[item.Name] = item
	}
	if env["SMITH_JOURNAL_RETENTION_MODE"].Value != "ttl" {
		t.Fatalf("unexpected retention mode env: %+v", env["SMITH_JOURNAL_RETENTION_MODE"])
	}
	if env["SMITH_JOURNAL_RETENTION_TTL"].Value != "336h0m0s" {
		t.Fatalf("unexpected retention ttl env: %+v", env["SMITH_JOURNAL_RETENTION_TTL"])
	}
	if env["SMITH_JOURNAL_ARCHIVE_MODE"].Value != "s3" {
		t.Fatalf("unexpected archive mode env: %+v", env["SMITH_JOURNAL_ARCHIVE_MODE"])
	}
	if env["SMITH_JOURNAL_ARCHIVE_BUCKET"].Value != "smith-journal-archive" {
		t.Fatalf("unexpected archive bucket env: %+v", env["SMITH_JOURNAL_ARCHIVE_BUCKET"])
	}
}

func TestBuildReplicaJobRejectsJournalPolicyOverrideWhenFeatureDisabled(t *testing.T) {
	req := validRequest()
	policy := journalpolicy.Policy{
		RetentionMode: journalpolicy.RetentionTTL,
		RetentionTTL:  time.Hour,
		ArchiveMode:   journalpolicy.ArchiveNone,
	}
	req.JournalPolicy = &policy
	req.EnableJournalPolicyConfig = false
	_, err := BuildReplicaJob(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrInvalidJobRequest) {
		t.Fatalf("expected ErrInvalidJobRequest, got %v", err)
	}
}

func TestBuildReplicaJobWithGitHubAppAuth(t *testing.T) {
	req := validRequest()
	req.GitAuth = &GitAuthConfig{
		Provider:            GitAuthProviderGitHubApp,
		EnableGitHubAppAuth: true,
		GitHubApp: &GitHubAppAuth{
			AppID:                "12345",
			InstallationID:       "67890",
			PrivateKeySecretName: "smith-github-app",
			PrivateKeySecretKey:  "private_key",
		},
	}

	job, err := BuildReplicaJob(req)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	env := map[string]EnvVar{}
	for _, item := range job.Spec.Template.Spec.Containers[0].Env {
		env[item.Name] = item
	}
	if env["SMITH_GIT_AUTH_PROVIDER"].Value != string(GitAuthProviderGitHubApp) {
		t.Fatalf("unexpected auth provider env: %+v", env["SMITH_GIT_AUTH_PROVIDER"])
	}
	if env["SMITH_GITHUB_APP_ID"].Value != "12345" {
		t.Fatalf("unexpected app id env: %+v", env["SMITH_GITHUB_APP_ID"])
	}
	if env["SMITH_GITHUB_APP_PRIVATE_KEY"].SecretKeyRef == nil {
		t.Fatalf("expected github app private key secret ref")
	}
}

func TestBuildReplicaJobRejectsGitHubAppWhenFeatureDisabled(t *testing.T) {
	req := validRequest()
	req.GitAuth = &GitAuthConfig{
		Provider: GitAuthProviderGitHubApp,
		GitHubApp: &GitHubAppAuth{
			AppID:                "12345",
			InstallationID:       "67890",
			PrivateKeySecretName: "smith-github-app",
			PrivateKeySecretKey:  "private_key",
		},
	}
	_, err := BuildReplicaJob(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrInvalidJobRequest) {
		t.Fatalf("expected ErrInvalidJobRequest, got %v", err)
	}
}

func TestBuildReplicaJobWithSSHAuth(t *testing.T) {
	req := validRequest()
	req.GitAuth = &GitAuthConfig{
		Provider:      GitAuthProviderSSH,
		EnableSSHAuth: true,
		SSH: &SSHAuth{
			PrivateKeySecretName: "smith-git-ssh",
			PrivateKeySecretKey:  "id_ed25519",
			KnownHostsSecretName: "smith-git-ssh",
			KnownHostsSecretKey:  "known_hosts",
		},
	}

	job, err := BuildReplicaJob(req)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	env := map[string]EnvVar{}
	for _, item := range job.Spec.Template.Spec.Containers[0].Env {
		env[item.Name] = item
	}
	if env["SMITH_GIT_AUTH_PROVIDER"].Value != string(GitAuthProviderSSH) {
		t.Fatalf("unexpected auth provider env: %+v", env["SMITH_GIT_AUTH_PROVIDER"])
	}
	if env["SMITH_GIT_SSH_PRIVATE_KEY"].SecretKeyRef == nil {
		t.Fatalf("expected SMITH_GIT_SSH_PRIVATE_KEY secret ref")
	}
	if env["SMITH_GIT_SSH_KNOWN_HOSTS"].SecretKeyRef == nil {
		t.Fatalf("expected SMITH_GIT_SSH_KNOWN_HOSTS secret ref")
	}
}

func TestBuildReplicaJobRejectsSSHWhenFeatureDisabled(t *testing.T) {
	req := validRequest()
	req.GitAuth = &GitAuthConfig{
		Provider: GitAuthProviderSSH,
		SSH: &SSHAuth{
			PrivateKeySecretName: "smith-git-ssh",
			PrivateKeySecretKey:  "id_ed25519",
		},
	}
	_, err := BuildReplicaJob(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrInvalidJobRequest) {
		t.Fatalf("expected ErrInvalidJobRequest, got %v", err)
	}
}

func TestBuildReplicaJobValidation(t *testing.T) {
	req := validRequest()
	req.Git.Repository = ""

	_, err := BuildReplicaJob(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrInvalidJobRequest) {
		t.Fatalf("expected ErrInvalidJobRequest, got %v", err)
	}

	req = validRequest()
	req.SkillMounts = append(req.SkillMounts, SkillMount{Source: "local://skills/no-name", MountPath: "/smith/skills/no-name", ReadOnly: true})
	_, err = BuildReplicaJob(req)
	if err == nil {
		t.Fatal("expected validation error for missing skill name")
	}
	if !errors.Is(err, ErrInvalidJobRequest) {
		t.Fatalf("expected ErrInvalidJobRequest, got %v", err)
	}
}

func TestSubmitSurfacesClientFailure(t *testing.T) {
	client := &fakeJobsAPI{
		createErr: errors.New("k8s unavailable"),
	}
	generator := NewJobGenerator(client)

	_, err := generator.Submit(context.Background(), validRequest())
	if err == nil {
		t.Fatal("expected submit failure")
	}
	if !errors.Is(err, ErrSubmitFailed) {
		t.Fatalf("expected ErrSubmitFailed, got %v", err)
	}
}

func TestDeleteSurfacesClientFailure(t *testing.T) {
	client := &fakeJobsAPI{
		deleteErr: errors.New("delete denied"),
	}
	generator := NewJobGenerator(client)

	err := generator.Delete(context.Background(), "smith-system", "smith-replica-loop-123")
	if err == nil {
		t.Fatal("expected delete failure")
	}
	if !errors.Is(err, ErrDeleteFailed) {
		t.Fatalf("expected ErrDeleteFailed, got %v", err)
	}
}

func TestSubmitAndDeleteSuccess(t *testing.T) {
	client := &fakeJobsAPI{}
	generator := NewJobGenerator(client)

	job, err := generator.Submit(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("unexpected submit error: %v", err)
	}
	if client.created.Metadata.Name != job.Metadata.Name {
		t.Fatalf("expected created job %q, got %q", job.Metadata.Name, client.created.Metadata.Name)
	}

	if err := generator.Delete(context.Background(), "smith-system", job.Metadata.Name); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
	if client.deletedNamespace != "smith-system" || client.deletedName != job.Metadata.Name {
		t.Fatalf("unexpected delete target %s/%s", client.deletedNamespace, client.deletedName)
	}
}

func validRequest() JobRequest {
	return JobRequest{
		Namespace:          "smith-system",
		LoopID:             "loop-123",
		CorrelationID:      "corr-123",
		ServiceAccountName: "smith-replica",
		Image:              "ghcr.io/smith/replica:latest",
		ImagePullPolicy:    "IfNotPresent",
		Git:                GitContext{Repository: "https://github.com/acme/repo.git", Branch: "main", CommitSHA: "abc1234"},
		GitAuth: &GitAuthConfig{
			Provider:      GitAuthProviderPAT,
			PATSecretName: "smith-git-pat",
			PATSecretKey:  "token",
		},
		SkillMounts: []SkillMount{
			{
				Name:      "commit",
				Source:    "local://skills/commit",
				MountPath: "/smith/skills/commit",
				ReadOnly:  true,
			},
			{
				Name:      "lint",
				Source:    "local://skills/lint",
				MountPath: "/smith/skills/lint",
				ReadOnly:  false,
			},
		},
		HandoffConfigMapName:    "handoff-loop-123",
		RuntimeSecretName:       "smith-runtime",
		RuntimeCredentialsKey:   "runtime_credentials",
		BackoffLimit:            2,
		ActiveDeadlineSeconds:   1800,
		TTLSecondsAfterFinished: 3600,
	}
}

type fakeJobsAPI struct {
	created          JobManifest
	deletedNamespace string
	deletedName      string
	createErr        error
	deleteErr        error
}

func (f *fakeJobsAPI) CreateJob(_ context.Context, job JobManifest) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = job
	return nil
}

func (f *fakeJobsAPI) DeleteJob(_ context.Context, namespace, name string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedNamespace = namespace
	f.deletedName = name
	return nil
}
