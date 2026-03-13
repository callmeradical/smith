package main

import (
	"context"
	"encoding/json"
	"errors"
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
		State:         model.LoopStateRunning,
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
		State:         model.LoopStateRunning,
		WorkerJobName: "smith-replica-loop-d-12345",
	})

	if got.Attachable {
		t.Fatalf("expected attachable false when pod is missing, got true")
	}
	if got.Reason != "runtime pod not found" {
		t.Fatalf("expected reason runtime pod not found, got %q", got.Reason)
	}
}

func TestResolveLoopRuntimeFallsBackToFirstContainer(t *testing.T) {
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		runtimePods: &fakeRuntimePodReader{
			podsByJob: map[string][]corev1.Pod{
				"smith-replica-loop-e-12345": {
					{
						ObjectMeta: metav1.ObjectMeta{Name: "smith-replica-loop-e-12345-abc"},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "worker"}},
						},
						Status: corev1.PodStatus{Phase: corev1.PodRunning},
					},
				},
			},
		},
	}

	got := s.resolveLoopRuntime(context.Background(), "loop-e", model.StateRecord{
		LoopID:        "loop-e",
		State:         model.LoopStateRunning,
		WorkerJobName: "smith-replica-loop-e-12345",
	})

	if !got.Attachable {
		t.Fatalf("expected attachable true when fallback container exists, got false with reason %q", got.Reason)
	}
	if got.ContainerName != "worker" {
		t.Fatalf("expected fallback container worker, got %q", got.ContainerName)
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
					State:         model.LoopStateRunning,
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

func TestHandleLoopAttachRejectsUnauthorizedBeforeRuntimeResolution(t *testing.T) {
	var stateCalls int
	var audits []store.AuditRecord
	runtimeReader := &fakeRuntimePodReader{}
	s := &server{
		cfg: config{
			operatorToken: "secret-token",
		},
		term: newTerminalSessionStore(),
		getStateFn: func(_ context.Context, _ string) (store.LoopWithRevision, bool, error) {
			stateCalls++
			return store.LoopWithRevision{}, false, nil
		},
		appendAuditFn: func(_ context.Context, rec store.AuditRecord) error {
			audits = append(audits, rec)
			return nil
		},
		runtimePods: runtimeReader,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-auth/control/attach", strings.NewReader(`{"actor":"alice","terminal":"console-pods"}`))
	s.handleLoopAttach(rec, req, "loop-auth")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized attach, got %d body=%s", rec.Code, rec.Body.String())
	}
	if stateCalls != 0 {
		t.Fatalf("expected no state resolution for unauthorized attach, got %d calls", stateCalls)
	}
	if runtimeReader.calls != 0 {
		t.Fatalf("expected no runtime resolution for unauthorized attach, got %d calls", runtimeReader.calls)
	}
	if len(audits) != 1 {
		t.Fatalf("expected one rejected attach audit entry, got %d", len(audits))
	}
	if audits[0].Action != "attach-terminal-rejected" {
		t.Fatalf("expected attach-terminal-rejected action, got %q", audits[0].Action)
	}
	if audits[0].Metadata["request_status"] != "rejected" {
		t.Fatalf("expected rejected status in audit metadata, got %q", audits[0].Metadata["request_status"])
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["code"] != terminalErrUnauthorized {
		t.Fatalf("expected unauthorized code %q, got %q", terminalErrUnauthorized, body["code"])
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
					State:         model.LoopStateRunning,
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
	if lastAttachAudit.Metadata["request_status"] != "accepted" {
		t.Fatalf("expected request_status=accepted in attach metadata, got %q", lastAttachAudit.Metadata["request_status"])
	}

	detachAudit := audits[2]
	if detachAudit.Action != "detach-terminal" {
		t.Fatalf("expected detach-terminal action, got %q", detachAudit.Action)
	}
	assertTerminalMetadata(t, detachAudit.Metadata, "alice", "console-pods", "smith-system/smith-replica-loop-running-12345-abc:replica")
	if detachAudit.Metadata["request_status"] != "accepted" {
		t.Fatalf("expected request_status=accepted in detach metadata, got %q", detachAudit.Metadata["request_status"])
	}

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
					State:  model.LoopStateRunning,
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

func TestHandleLoopDetachRejectsActorNotAttached(t *testing.T) {
	var audits []store.AuditRecord
	s := &server{
		term: newTerminalSessionStore(),
		getStateFn: func(_ context.Context, loopID string) (store.LoopWithRevision, bool, error) {
			return store.LoopWithRevision{
				Record: model.StateRecord{
					LoopID:        loopID,
					State:         model.LoopStateRunning,
					CorrelationID: "corr-detach-not-attached",
				},
			}, true, nil
		},
		appendAuditFn: func(_ context.Context, rec store.AuditRecord) error {
			audits = append(audits, rec)
			return nil
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-actor/control/detach", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopDetach(rec, req, "loop-actor")

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 when actor is not attached, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["error"] != "actor is not attached" {
		t.Fatalf("expected actor-not-attached error, got %q", body["error"])
	}
	if len(audits) != 1 {
		t.Fatalf("expected one detach rejection audit record, got %d", len(audits))
	}
	if audits[0].Action != "detach-terminal-rejected" {
		t.Fatalf("expected detach-terminal-rejected action, got %q", audits[0].Action)
	}
	if audits[0].Metadata["error_code"] != terminalErrNotAttached {
		t.Fatalf("expected detach rejection error_code %q, got %q", terminalErrNotAttached, audits[0].Metadata["error_code"])
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
					State:         model.LoopStateRunning,
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
	if lastAudit.Metadata["request_status"] != "accepted" {
		t.Fatalf("expected accepted request_status metadata, got %q", lastAudit.Metadata["request_status"])
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

func TestHandleLoopControlCommandRejectsInvalidJSON(t *testing.T) {
	var audits []store.AuditRecord
	execRunner := &fakePodExecRunner{}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		getStateFn: func(_ context.Context, loopID string) (store.LoopWithRevision, bool, error) {
			return store.LoopWithRevision{
				Record: model.StateRecord{
					LoopID:        loopID,
					State:         model.LoopStateRunning,
					CorrelationID: "corr-command-invalid-json",
				},
			}, true, nil
		},
		appendAuditFn: func(_ context.Context, rec store.AuditRecord) error {
			audits = append(audits, rec)
			return nil
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command"`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json payload, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != 0 {
		t.Fatalf("expected no pod exec call for invalid json, got %d", execRunner.calls)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["code"] != terminalErrInvalidJSON {
		t.Fatalf("expected invalid json code %q, got %q", terminalErrInvalidJSON, body["code"])
	}
	if len(audits) != 1 {
		t.Fatalf("expected one rejected audit entry, got %d", len(audits))
	}
	if audits[0].Metadata["error_code"] != terminalErrInvalidJSON {
		t.Fatalf("expected invalid-json audit error_code %q, got %q", terminalErrInvalidJSON, audits[0].Metadata["error_code"])
	}
}

func TestHandleLoopControlCommandRejectsRequiredCommand(t *testing.T) {
	var audits []store.AuditRecord
	execRunner := &fakePodExecRunner{}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		getStateFn: func(_ context.Context, loopID string) (store.LoopWithRevision, bool, error) {
			return store.LoopWithRevision{
				Record: model.StateRecord{
					LoopID:        loopID,
					State:         model.LoopStateRunning,
					CorrelationID: "corr-command-required",
				},
			}, true, nil
		},
		appendAuditFn: func(_ context.Context, rec store.AuditRecord) error {
			audits = append(audits, rec)
			return nil
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"   "}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when command is required, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != 0 {
		t.Fatalf("expected no pod exec call for missing command, got %d", execRunner.calls)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["code"] != terminalErrRequiredCmd {
		t.Fatalf("expected required-command code %q, got %q", terminalErrRequiredCmd, body["code"])
	}
	if len(audits) != 1 {
		t.Fatalf("expected one rejected audit entry, got %d", len(audits))
	}
	if audits[0].Metadata["error_code"] != terminalErrRequiredCmd {
		t.Fatalf("expected required-command audit error_code %q, got %q", terminalErrRequiredCmd, audits[0].Metadata["error_code"])
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
					State:         model.LoopStateRunning,
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
	if audits[0].Metadata["error_code"] != terminalErrTooLong {
		t.Fatalf("expected error_code %q, got %q", terminalErrTooLong, audits[0].Metadata["error_code"])
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["code"] != terminalErrTooLong {
		t.Fatalf("expected API error code %q, got %q", terminalErrTooLong, body["code"])
	}
}

func TestHandleLoopControlCommandRejectsUnauthorizedWithoutExec(t *testing.T) {
	var stateCalls int
	var audits []store.AuditRecord
	execRunner := &fakePodExecRunner{}
	s := &server{
		cfg: config{
			operatorToken: "secret-token",
		},
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		getStateFn: func(_ context.Context, _ string) (store.LoopWithRevision, bool, error) {
			stateCalls++
			return store.LoopWithRevision{}, false, nil
		},
		appendAuditFn: func(_ context.Context, rec store.AuditRecord) error {
			audits = append(audits, rec)
			return nil
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo hello"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized command, got %d body=%s", rec.Code, rec.Body.String())
	}
	if stateCalls != 0 {
		t.Fatalf("expected no state lookup for unauthorized command, got %d calls", stateCalls)
	}
	if execRunner.calls != 0 {
		t.Fatalf("expected no pod exec call for unauthorized command, got %d", execRunner.calls)
	}
	if len(audits) != 1 {
		t.Fatalf("expected one rejected audit entry, got %d", len(audits))
	}
	if audits[0].Metadata["error_code"] != terminalErrUnauthorized {
		t.Fatalf("expected unauthorized error_code %q, got %q", terminalErrUnauthorized, audits[0].Metadata["error_code"])
	}
}

func TestHandleLoopDetachRejectsUnauthorizedBeforeStateLookup(t *testing.T) {
	var stateCalls int
	var audits []store.AuditRecord
	s := &server{
		cfg: config{
			operatorToken: "secret-token",
		},
		term: newTerminalSessionStore(),
		getStateFn: func(_ context.Context, _ string) (store.LoopWithRevision, bool, error) {
			stateCalls++
			return store.LoopWithRevision{}, false, nil
		},
		appendAuditFn: func(_ context.Context, rec store.AuditRecord) error {
			audits = append(audits, rec)
			return nil
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-auth/control/detach", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopDetach(rec, req, "loop-auth")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized detach, got %d body=%s", rec.Code, rec.Body.String())
	}
	if stateCalls != 0 {
		t.Fatalf("expected no state lookup for unauthorized detach, got %d calls", stateCalls)
	}
	if len(audits) != 1 {
		t.Fatalf("expected one rejected detach audit entry, got %d", len(audits))
	}
	if audits[0].Action != "detach-terminal-rejected" {
		t.Fatalf("expected detach-terminal-rejected action, got %q", audits[0].Action)
	}
}

func TestHandleLoopControlCommandRateLimitPerSession(t *testing.T) {
	previousWindow := terminalCommandRateWindow
	terminalCommandRateWindow = 30 * time.Second
	t.Cleanup(func() {
		terminalCommandRateWindow = previousWindow
	})

	var audits []store.AuditRecord
	execRunner := &fakePodExecRunner{
		result: podExecResult{
			Stdout:   "ok\n",
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
					State:         model.LoopStateRunning,
					CorrelationID: "corr-command-throttle",
				},
			}, true, nil
		},
		appendAuditFn: func(_ context.Context, rec store.AuditRecord) error {
			audits = append(audits, rec)
			return nil
		},
		appendJournalFn: func(_ context.Context, _ model.JournalEntry) error { return nil },
	}
	runtime := loopRuntimeResponse{
		Namespace:     "smith-system",
		PodName:       "smith-replica-loop-command-12345-abc",
		ContainerName: "replica",
		PodPhase:      string(corev1.PodRunning),
		Attachable:    true,
	}
	s.term.Attach("loop-command", "alice", "console-pods", runtime)

	for i := 0; i < terminalCommandRateMax; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo ok"}`))
		s.handleLoopControlCommand(rec, req, "loop-command")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected command %d to pass within rate limit, got %d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo burst"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burst, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != terminalCommandRateMax {
		t.Fatalf("expected pod exec calls capped at %d, got %d", terminalCommandRateMax, execRunner.calls)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode throttled response: %v", err)
	}
	if body["code"] != terminalErrRateLimited {
		t.Fatalf("expected throttled code %q, got %q", terminalErrRateLimited, body["code"])
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header for throttled response")
	}
	if len(audits) < terminalCommandRateMax+1 {
		t.Fatalf("expected audit records for accepted and throttled commands, got %d", len(audits))
	}
	last := audits[len(audits)-1]
	if last.Action != "terminal-command-rejected" {
		t.Fatalf("expected terminal-command-rejected audit action, got %q", last.Action)
	}
	if last.Metadata["rejection_reason"] != "command rate limit exceeded" {
		t.Fatalf("expected throttle rejection reason, got %q", last.Metadata["rejection_reason"])
	}
	if last.Metadata["request_status"] != "rejected" {
		t.Fatalf("expected rejected request_status metadata, got %q", last.Metadata["request_status"])
	}
}

func TestHandleLoopControlCommandHandlesNonZeroExitResult(t *testing.T) {
	var (
		audits   []store.AuditRecord
		journals []model.JournalEntry
	)
	execRunner := &fakePodExecRunner{
		result: podExecResult{
			Stdout:   "ok\n",
			Stderr:   "oops\n",
			ExitCode: 17,
		},
	}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		getStateFn: func(_ context.Context, loopID string) (store.LoopWithRevision, bool, error) {
			return store.LoopWithRevision{
				Record: model.StateRecord{
					LoopID:        loopID,
					State:         model.LoopStateRunning,
					CorrelationID: "corr-command-nonzero",
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
		PodName:       "smith-replica-loop-command-nonzero-12345-abc",
		ContainerName: "replica",
		PodPhase:      string(corev1.PodRunning),
		Attachable:    true,
	}
	s.term.Attach("loop-command", "alice", "console-pods", runtime)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo ok"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected command response status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["result"] != "failed" {
		t.Fatalf("expected result=failed for non-zero exit, got %#v", body["result"])
	}
	if int(body["exit_code"].(float64)) != 17 {
		t.Fatalf("expected exit_code=17, got %#v", body["exit_code"])
	}
	if len(audits) == 0 {
		t.Fatal("expected terminal command audit entry")
	}
	lastAudit := audits[len(audits)-1]
	if lastAudit.Metadata["result"] != "failed" {
		t.Fatalf("expected failed audit result metadata, got %q", lastAudit.Metadata["result"])
	}
	if lastAudit.Metadata["stderr_bytes"] != "5" {
		t.Fatalf("expected stderr_bytes=5 in audit metadata, got %q", lastAudit.Metadata["stderr_bytes"])
	}

	foundStdout := false
	foundStderr := false
	for _, entry := range journals {
		if entry.Message == "ok" && entry.Metadata["stream"] == "stdout" {
			foundStdout = true
		}
		if entry.Message == "oops" && entry.Metadata["stream"] == "stderr" {
			foundStderr = true
		}
	}
	if !foundStdout || !foundStderr {
		t.Fatalf("expected stdout+stderr lines in journal entries, got %+v", journals)
	}
}

func TestHandleLoopControlCommandHandlesExecError(t *testing.T) {
	var audits []store.AuditRecord
	execRunner := &fakePodExecRunner{
		result: podExecResult{
			Stdout: "partial\n",
		},
		err: errors.New("runtime transport interrupted"),
	}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		getStateFn: func(_ context.Context, loopID string) (store.LoopWithRevision, bool, error) {
			return store.LoopWithRevision{
				Record: model.StateRecord{
					LoopID:        loopID,
					State:         model.LoopStateRunning,
					CorrelationID: "corr-command-exec-error",
				},
			}, true, nil
		},
		appendAuditFn: func(_ context.Context, rec store.AuditRecord) error {
			audits = append(audits, rec)
			return nil
		},
		appendJournalFn: func(_ context.Context, _ model.JournalEntry) error { return nil },
	}
	runtime := loopRuntimeResponse{
		Namespace:     "smith-system",
		PodName:       "smith-replica-loop-command-error-12345-abc",
		ContainerName: "replica",
		PodPhase:      string(corev1.PodRunning),
		Attachable:    true,
	}
	s.term.Attach("loop-command", "alice", "console-pods", runtime)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo ok"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected command response status 200 on exec error reporting path, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["result"] != "error" {
		t.Fatalf("expected result=error when exec runner returns error, got %#v", body["result"])
	}
	if int(body["exit_code"].(float64)) != -1 {
		t.Fatalf("expected exit_code=-1 on exec error, got %#v", body["exit_code"])
	}
	if body["error"] != "runtime transport interrupted" {
		t.Fatalf("expected exec error message in response, got %#v", body["error"])
	}

	if len(audits) == 0 {
		t.Fatal("expected terminal command audit entry")
	}
	lastAudit := audits[len(audits)-1]
	if lastAudit.Metadata["result"] != "error" {
		t.Fatalf("expected error result in audit metadata, got %q", lastAudit.Metadata["result"])
	}
	if lastAudit.Metadata["exec_error"] != "runtime transport interrupted" {
		t.Fatalf("expected exec_error metadata to be set, got %q", lastAudit.Metadata["exec_error"])
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
