package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"smith/internal/source/model"
	"smith/internal/source/store"

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

func TestHandleLoopAttachRejectsNonRunningRuntime(t *testing.T) {
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		term: newTerminalSessionStore(),
		getStateFn: func(_ context.Context, loopID string) (store.LoopWithRevision, bool, error) {
			return store.LoopWithRevision{
				Record: model.StateRecord{
					LoopID:        loopID,
					State:         model.LoopStateOverwriting,
					WorkerJobName: "smith-replica-loop-pending-12345",
				},
			}, true, nil
		},
		runtimePods: &fakeRuntimePodReader{
			podsByJob: map[string][]corev1.Pod{
				"smith-replica-loop-pending-12345": {
					{
						ObjectMeta: metav1.ObjectMeta{Name: "smith-replica-loop-pending-12345-abc"},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "replica"}},
						},
						Status: corev1.PodStatus{Phase: corev1.PodPending},
					},
				},
			},
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-pending/control/attach", strings.NewReader(`{"actor":"alice","terminal":"console-pods"}`))
	s.handleLoopAttach(rec, req, "loop-pending")

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for non-running runtime pod, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["error"] != "runtime pod not running" {
		t.Fatalf("expected runtime pod not running error, got %q", body["error"])
	}
	if s.term.IsAttached("loop-pending", "alice") {
		t.Fatal("expected no terminal session to be created for non-running runtime pod")
	}
}

func TestHandleLoopAttachDetachIncludeRuntimeMetadata(t *testing.T) {
	var (
		audits   []store.AuditRecord
		journals []model.JournalEntry
	)
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		term: newTerminalSessionStore(),
		getStateFn: func(_ context.Context, loopID string) (store.LoopWithRevision, bool, error) {
			return store.LoopWithRevision{
				Record: model.StateRecord{
					LoopID:        loopID,
					State:         model.LoopStateOverwriting,
					WorkerJobName: "smith-replica-loop-running-12345",
					CorrelationID: "corr-attach-detach",
				},
			}, true, nil
		},
		appendAuditFn: func(_ context.Context, rec store.AuditRecord) error {
			audits = append(audits, rec)
			return nil
		},
		appendJournalFn: func(_ context.Context, entry model.JournalEntry) error {
			journals = append(journals, entry)
			return nil
		},
		runtimePods: &fakeRuntimePodReader{
			podsByJob: map[string][]corev1.Pod{
				"smith-replica-loop-running-12345": {
					{
						ObjectMeta: metav1.ObjectMeta{Name: "smith-replica-loop-running-12345-abc"},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "replica"}},
						},
						Status: corev1.PodStatus{Phase: corev1.PodRunning},
					},
				},
			},
		},
	}

	attachBody := strings.NewReader(`{"actor":"alice","terminal":"console-pods"}`)
	recAttach1 := httptest.NewRecorder()
	reqAttach1 := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-running/control/attach", attachBody)
	s.handleLoopAttach(recAttach1, reqAttach1, "loop-running")
	if recAttach1.Code != http.StatusOK {
		t.Fatalf("expected first attach success, got %d body=%s", recAttach1.Code, recAttach1.Body.String())
	}

	recAttach2 := httptest.NewRecorder()
	reqAttach2 := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-running/control/attach", strings.NewReader(`{"actor":"alice","terminal":"console-pods"}`))
	s.handleLoopAttach(recAttach2, reqAttach2, "loop-running")
	if recAttach2.Code != http.StatusOK {
		t.Fatalf("expected second attach success, got %d body=%s", recAttach2.Code, recAttach2.Body.String())
	}
	var attachResp map[string]any
	if err := json.NewDecoder(recAttach2.Body).Decode(&attachResp); err != nil {
		t.Fatalf("decode second attach response: %v", err)
	}
	if int(attachResp["attach_count"].(float64)) != 2 {
		t.Fatalf("expected actor attach_count to increment to 2, got %#v", attachResp["attach_count"])
	}

	recDetach := httptest.NewRecorder()
	reqDetach := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-running/control/detach", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopDetach(recDetach, reqDetach, "loop-running")
	if recDetach.Code != http.StatusOK {
		t.Fatalf("expected detach success, got %d body=%s", recDetach.Code, recDetach.Body.String())
	}
	if s.term.IsAttached("loop-running", "alice") {
		t.Fatal("expected actor to be detached")
	}

	if len(audits) != 3 {
		t.Fatalf("expected 3 audit records (2 attach + 1 detach), got %d", len(audits))
	}
	if len(journals) != 3 {
		t.Fatalf("expected 3 journal records (2 attach + 1 detach), got %d", len(journals))
	}

	lastAttachAudit := audits[1]
	if lastAttachAudit.Action != "attach-terminal" {
		t.Fatalf("expected attach-terminal action, got %q", lastAttachAudit.Action)
	}
	assertTerminalMetadata(t, lastAttachAudit.Metadata, "alice", "console-pods", "smith-system/smith-replica-loop-running-12345-abc:replica")
	if lastAttachAudit.Metadata["attach_count"] != "2" {
		t.Fatalf("expected attach_count=2 in audit metadata, got %q", lastAttachAudit.Metadata["attach_count"])
	}

	detachAudit := audits[2]
	if detachAudit.Action != "detach-terminal" {
		t.Fatalf("expected detach-terminal action, got %q", detachAudit.Action)
	}
	assertTerminalMetadata(t, detachAudit.Metadata, "alice", "console-pods", "smith-system/smith-replica-loop-running-12345-abc:replica")

	detachJournal := journals[2]
	if detachJournal.Message != "terminal detached" {
		t.Fatalf("expected detach journal message, got %q", detachJournal.Message)
	}
	assertTerminalMetadata(t, detachJournal.Metadata, "alice", "console-pods", "smith-system/smith-replica-loop-running-12345-abc:replica")
}

func TestHandleLoopDetachOnlyRemovesTargetActor(t *testing.T) {
	s := &server{
		term: newTerminalSessionStore(),
		getStateFn: func(_ context.Context, loopID string) (store.LoopWithRevision, bool, error) {
			return store.LoopWithRevision{
				Record: model.StateRecord{
					LoopID: loopID,
					State:  model.LoopStateOverwriting,
				},
			}, true, nil
		},
		appendAuditFn:   func(context.Context, store.AuditRecord) error { return nil },
		appendJournalFn: func(context.Context, model.JournalEntry) error { return nil },
	}
	runtime := loopRuntimeResponse{
		Namespace:     "smith-system",
		PodName:       "smith-replica-loop-actor-12345-abc",
		ContainerName: "replica",
		PodPhase:      string(corev1.PodRunning),
		Attachable:    true,
	}
	s.term.Attach("loop-actor", "alice", "console-pods", runtime)
	s.term.Attach("loop-actor", "bob", "console-pods", runtime)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-actor/control/detach", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopDetach(rec, req, "loop-actor")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected detach success for alice, got %d body=%s", rec.Code, rec.Body.String())
	}
	if s.term.IsAttached("loop-actor", "alice") {
		t.Fatal("expected alice to be detached")
	}
	if !s.term.IsAttached("loop-actor", "bob") {
		t.Fatal("expected bob to remain attached")
	}
}

func TestHandleLoopControlCommandExecutesAttachedRuntime(t *testing.T) {
	var (
		audits   []store.AuditRecord
		journals []model.JournalEntry
	)
	execRunner := &fakePodExecRunner{
		result: podExecResult{
			Stdout:   "hello\n",
			ExitCode: 0,
		},
	}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		getStateFn: func(_ context.Context, loopID string) (store.LoopWithRevision, bool, error) {
			return store.LoopWithRevision{
				Record: model.StateRecord{
					LoopID:        loopID,
					State:         model.LoopStateOverwriting,
					CorrelationID: "corr-command-success",
				},
			}, true, nil
		},
		appendAuditFn: func(_ context.Context, rec store.AuditRecord) error {
			audits = append(audits, rec)
			return nil
		},
		appendJournalFn: func(_ context.Context, entry model.JournalEntry) error {
			journals = append(journals, entry)
			return nil
		},
	}
	runtime := loopRuntimeResponse{
		Namespace:     "smith-system",
		PodName:       "smith-replica-loop-command-12345-abc",
		ContainerName: "replica",
		PodPhase:      string(corev1.PodRunning),
		Attachable:    true,
	}
	s.term.Attach("loop-command", "alice", "console-pods", runtime)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo hello"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected command success, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != 1 {
		t.Fatalf("expected exactly one pod exec call, got %d", execRunner.calls)
	}
	if execRunner.lastRequest.Command != "echo hello" {
		t.Fatalf("expected command payload echo hello, got %q", execRunner.lastRequest.Command)
	}
	if execRunner.lastRequest.Namespace != runtime.Namespace || execRunner.lastRequest.PodName != runtime.PodName || execRunner.lastRequest.ContainerName != runtime.ContainerName {
		t.Fatalf("unexpected runtime target: %+v", execRunner.lastRequest)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if delivered, ok := body["delivered"].(bool); !ok || !delivered {
		t.Fatalf("expected delivered=true, got %#v", body["delivered"])
	}
	if result, _ := body["result"].(string); result != "success" {
		t.Fatalf("expected result=success, got %#v", body["result"])
	}
	if int(body["exit_code"].(float64)) != 0 {
		t.Fatalf("expected exit_code=0, got %#v", body["exit_code"])
	}
	if stdout, _ := body["stdout"].(string); stdout != "hello\n" {
		t.Fatalf("expected stdout hello\\n, got %#v", body["stdout"])
	}

	foundHello := false
	for _, entry := range journals {
		if entry.Message == "hello" {
			foundHello = true
			break
		}
	}
	if !foundHello {
		t.Fatalf("expected journal output containing hello, got %+v", journals)
	}
	if len(audits) == 0 {
		t.Fatal("expected terminal command audit entry")
	}
	lastAudit := audits[len(audits)-1]
	if lastAudit.Action != "terminal-command" {
		t.Fatalf("expected terminal-command action, got %q", lastAudit.Action)
	}
	if lastAudit.Metadata["delivered"] != "true" {
		t.Fatalf("expected delivered audit metadata true, got %q", lastAudit.Metadata["delivered"])
	}
	if lastAudit.Metadata["exit_code"] != "0" {
		t.Fatalf("expected exit_code audit metadata 0, got %q", lastAudit.Metadata["exit_code"])
	}
}

func TestHandleLoopControlCommandRequiresAttach(t *testing.T) {
	execRunner := &fakePodExecRunner{}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		getStateFn: func(_ context.Context, loopID string) (store.LoopWithRevision, bool, error) {
			return store.LoopWithRevision{
				Record: model.StateRecord{
					LoopID: loopID,
					State:  model.LoopStateUnresolved,
				},
			}, true, nil
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo hello"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 when actor is not attached, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != 0 {
		t.Fatalf("expected no pod exec call without attachment, got %d", execRunner.calls)
	}
}

func TestHandleLoopControlCommandRejectsOversizedCommand(t *testing.T) {
	var audits []store.AuditRecord
	execRunner := &fakePodExecRunner{}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		getStateFn: func(_ context.Context, loopID string) (store.LoopWithRevision, bool, error) {
			return store.LoopWithRevision{
				Record: model.StateRecord{
					LoopID:        loopID,
					State:         model.LoopStateOverwriting,
					CorrelationID: "corr-command-rejected",
				},
			}, true, nil
		},
		appendAuditFn: func(_ context.Context, rec store.AuditRecord) error {
			audits = append(audits, rec)
			return nil
		},
	}
	runtime := loopRuntimeResponse{
		Namespace:     "smith-system",
		PodName:       "smith-replica-loop-command-99999-abc",
		ContainerName: "replica",
		PodPhase:      string(corev1.PodRunning),
		Attachable:    true,
	}
	s.term.Attach("loop-command", "alice", "console-pods", runtime)

	oversized := strings.Repeat("x", terminalCommandMaxSize+1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"`+oversized+`"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized command, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != 0 {
		t.Fatalf("expected no pod exec call for oversized command, got %d", execRunner.calls)
	}
	if len(audits) != 1 {
		t.Fatalf("expected one rejected audit entry, got %d", len(audits))
	}
	if audits[0].Action != "terminal-command-rejected" {
		t.Fatalf("expected terminal-command-rejected action, got %q", audits[0].Action)
	}
	if audits[0].Metadata["result"] != "rejected" {
		t.Fatalf("expected rejected metadata tag, got %q", audits[0].Metadata["result"])
	}
}

func assertTerminalMetadata(t *testing.T, metadata map[string]string, actor, terminal, runtimeRef string) {
	t.Helper()
	if metadata["actor"] != actor {
		t.Fatalf("expected metadata actor %q, got %q", actor, metadata["actor"])
	}
	if metadata["terminal"] != terminal {
		t.Fatalf("expected metadata terminal %q, got %q", terminal, metadata["terminal"])
	}
	if metadata["runtime_target_ref"] != runtimeRef {
		t.Fatalf("expected runtime_target_ref %q, got %q", runtimeRef, metadata["runtime_target_ref"])
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

type fakePodExecRunner struct {
	result      podExecResult
	err         error
	calls       int
	lastRequest podExecRequest
}

func (f *fakePodExecRunner) Execute(_ context.Context, req podExecRequest) (podExecResult, error) {
	f.calls++
	f.lastRequest = req
	return f.result, f.err
}
