package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"smith/internal/source/model"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIngressSummary(t *testing.T) {
	summary := ingressSummary([]ingressResult{
		{Status: "unresolved", Created: true},
		{Status: "error", Created: false},
		{Status: "unresolved", Created: false},
	})
	meta := summary["summary"].(map[string]int)
	if meta["requested"] != 3 || meta["created"] != 1 || meta["existing"] != 1 || meta["errors"] != 1 {
		t.Fatalf("unexpected summary: %#v", meta)
	}
}

func TestPresetCatalogSupportsCRUDAndPolicy(t *testing.T) {
	catalog := newPresetCatalog("team-default")
	if !catalog.Has("team-default") {
		t.Fatal("expected custom default preset to be present")
	}
	if !catalog.Has("standard") {
		t.Fatal("expected builtin standard preset to be present")
	}
	if err := catalog.Upsert("analytics"); err != nil {
		t.Fatalf("upsert preset: %v", err)
	}
	if !catalog.Has("analytics") {
		t.Fatal("expected analytics preset after upsert")
	}
	list := catalog.List()
	if len(list) < 2 {
		t.Fatalf("expected non-empty preset list, got %#v", list)
	}
	policy := catalog.Policy()
	if policy.DefaultPreset != "team-default" {
		t.Fatalf("unexpected default policy preset: %s", policy.DefaultPreset)
	}
	if _, ok := policy.AllowedPresets["analytics"]; !ok {
		t.Fatalf("expected analytics in allowed presets: %#v", policy.AllowedPresets)
	}
}

func TestMaskCredentialValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "short", in: "sk-12", want: "*****"},
		{name: "normal", in: "sk-test-123456", want: "sk-t******3456"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskCredentialValue(tc.in); got != tc.want {
				t.Fatalf("maskCredentialValue(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitLoopRouteSupportsSlashLoopIDs(t *testing.T) {
	tests := []struct {
		path       string
		wantLoopID string
		wantRoute  string
	}{
		{
			path:       "/v1/loops/alpha/feat-132",
			wantLoopID: "alpha/feat-132",
			wantRoute:  "",
		},
		{
			path:       "/v1/loops/alpha/feat-132/journal",
			wantLoopID: "alpha/feat-132",
			wantRoute:  "journal",
		},
		{
			path:       "/v1/loops/team/alpha/feat-132/control/attach",
			wantLoopID: "team/alpha/feat-132",
			wantRoute:  "control/attach",
		},
		{
			path:       "/v1/loops/team/alpha/feat-132/runtime",
			wantLoopID: "team/alpha/feat-132",
			wantRoute:  "runtime",
		},
		{
			path:       "/v1/loops/team/alpha/feat-132/journal/stream",
			wantLoopID: "team/alpha/feat-132",
			wantRoute:  "journal/stream",
		},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			loopID, route := splitLoopRoute(tc.path)
			if loopID != tc.wantLoopID || route != tc.wantRoute {
				t.Fatalf("splitLoopRoute(%q) = (%q, %q), want (%q, %q)", tc.path, loopID, route, tc.wantLoopID, tc.wantRoute)
			}
		})
	}
}

func TestResolveLoopRuntimeRunningPod(t *testing.T) {
	now := time.Now().UTC()
	reader := &fakeRuntimePodReader{
		podsByJob: map[string][]corev1.Pod{
			"smith-replica-loop-a-12345": {
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "smith-replica-loop-a-12345-abc",
						CreationTimestamp: metav1.NewTime(now),
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "replica"}},
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				},
			},
		},
	}
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		runtimePods: reader,
	}

	got := s.resolveLoopRuntime(context.Background(), "loop-a", model.StateRecord{
		LoopID:        "loop-a",
		State:         model.LoopStateOverwriting,
		WorkerJobName: "smith-replica-loop-a-12345",
	})

	if !got.Attachable {
		t.Fatalf("expected attachable true, got false with reason %q", got.Reason)
	}
	if got.Namespace != "smith-system" || got.PodName != "smith-replica-loop-a-12345-abc" || got.ContainerName != "replica" {
		t.Fatalf("unexpected runtime target: %+v", got)
	}
	if got.Reason != "" {
		t.Fatalf("expected empty reason for attachable runtime, got %q", got.Reason)
	}
}

func TestResolveLoopRuntimePendingPod(t *testing.T) {
	reader := &fakeRuntimePodReader{
		podsByJob: map[string][]corev1.Pod{
			"smith-replica-loop-b-12345": {
				{
					ObjectMeta: metav1.ObjectMeta{Name: "smith-replica-loop-b-12345-def"},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "replica"}},
					},
					Status: corev1.PodStatus{Phase: corev1.PodPending},
				},
			},
		},
	}
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		runtimePods: reader,
	}

	got := s.resolveLoopRuntime(context.Background(), "loop-b", model.StateRecord{
		LoopID:        "loop-b",
		State:         model.LoopStateUnresolved,
		WorkerJobName: "smith-replica-loop-b-12345",
	})

	if got.Attachable {
		t.Fatalf("expected attachable false for pending pod, got true")
	}
	if got.Reason != "runtime pod not running" {
		t.Fatalf("expected reason runtime pod not running, got %q", got.Reason)
	}
	if got.PodPhase != string(corev1.PodPending) {
		t.Fatalf("expected pod phase Pending, got %q", got.PodPhase)
	}
}

func TestResolveLoopRuntimeTerminalLoop(t *testing.T) {
	reader := &fakeRuntimePodReader{}
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		runtimePods: reader,
	}

	got := s.resolveLoopRuntime(context.Background(), "loop-c", model.StateRecord{
		LoopID: "loop-c",
		State:  model.LoopStateSynced,
	})

	if got.Attachable {
		t.Fatalf("expected attachable false for terminal loop, got true")
	}
	if got.Reason != "loop not active" {
		t.Fatalf("expected reason loop not active, got %q", got.Reason)
	}
	if reader.calls != 0 {
		t.Fatalf("expected no runtime pod lookup for terminal loop, got %d calls", reader.calls)
	}
}

func TestResolveLoopRuntimeMissingPod(t *testing.T) {
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		runtimePods: &fakeRuntimePodReader{podsByJob: map[string][]corev1.Pod{}},
	}

	got := s.resolveLoopRuntime(context.Background(), "loop-d", model.StateRecord{
		LoopID:        "loop-d",
		State:         model.LoopStateOverwriting,
		WorkerJobName: "smith-replica-loop-d-12345",
	})

	if got.Attachable {
		t.Fatalf("expected attachable false when pod is missing, got true")
	}
	if got.Reason != "runtime pod not found" {
		t.Fatalf("expected reason runtime pod not found, got %q", got.Reason)
	}
}

type fakeRuntimePodReader struct {
	podsByJob map[string][]corev1.Pod
	err       error
	calls     int
}

func (f *fakeRuntimePodReader) List(_ context.Context, _ string, opts metav1.ListOptions) (*corev1.PodList, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	jobName := selectorValue(opts.LabelSelector, "job-name")
	return &corev1.PodList{Items: f.podsByJob[jobName]}, nil
}

func selectorValue(selector, key string) string {
	key = strings.TrimSpace(key)
	for _, segment := range strings.Split(selector, ",") {
		left, right, ok := strings.Cut(segment, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(left) == key {
			return strings.TrimSpace(right)
		}
	}
	return ""
}
