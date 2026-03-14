package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"smith/internal/source/model"
)

func TestPRDAuthoringWorkflowEndToEnd(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(mustCallerFile()), "../../.."))
	validFixture := filepath.Join(repoRoot, "docs", "examples", "prd-authoring", "valid-prd.md")
	invalidFixture := filepath.Join(repoRoot, "docs", "examples", "prd-authoring", "invalid-prd.json")

	t.Run("markdown to canonical json to ingress", func(t *testing.T) {
		workDir := t.TempDir()
		jsonPath := filepath.Join(workDir, ".agents", "tasks", "prd.json")
		markdownOutPath := filepath.Join(workDir, "roundtrip.md")

		stdout, stderr, code := runSmithCLI(repoRoot, "--prd", "--from-markdown", validFixture, "--out", jsonPath)
		if code != 0 {
			t.Fatalf("markdown import failed: code=%d stderr=%s stdout=%s", code, stderr, string(stdout))
		}
		if !strings.Contains(string(stdout), "PRD JSON saved to "+jsonPath) {
			t.Fatalf("expected import success path, got stdout=%s", string(stdout))
		}

		importedJSON, err := os.ReadFile(jsonPath)
		if err != nil {
			t.Fatalf("read imported json: %v", err)
		}
		importedPRD, report := model.ValidatePRDJSON(importedJSON)
		if !report.Valid || report.Readiness != model.PRDReadinessPass {
			t.Fatalf("expected imported prd to validate cleanly, got %+v", report)
		}
		if len(importedPRD.Stories) != 2 {
			t.Fatalf("expected 2 stories, got %d", len(importedPRD.Stories))
		}

		stdout, stderr, code = runSmithCLI(repoRoot, "--prd", "validate", jsonPath)
		if code != 0 {
			t.Fatalf("validate failed unexpectedly: code=%d stderr=%s stdout=%s", code, stderr, string(stdout))
		}
		var validateReport model.PRDValidationReport
		if err := json.Unmarshal(stdout, &validateReport); err != nil {
			t.Fatalf("decode validate output: %v\n%s", err, string(stdout))
		}
		if !validateReport.Valid || validateReport.Readiness != model.PRDReadinessPass {
			t.Fatalf("expected readiness pass report, got %+v", validateReport)
		}

		stdout, stderr, code = runSmithCLI(repoRoot, "--prd", "--from-json", jsonPath, "--to-markdown", markdownOutPath)
		if code != 0 {
			t.Fatalf("markdown export failed: code=%d stderr=%s stdout=%s", code, stderr, string(stdout))
		}
		renderedMarkdown, err := os.ReadFile(markdownOutPath)
		if err != nil {
			t.Fatalf("read rendered markdown: %v", err)
		}
		roundTripPRD, roundTripReport := model.ValidatePRDMarkdown(renderedMarkdown)
		if !roundTripReport.Valid || roundTripReport.Readiness != model.PRDReadinessPass {
			t.Fatalf("expected round-trip markdown to validate, got %+v", roundTripReport)
		}
		if roundTripPRD.Project != importedPRD.Project {
			t.Fatalf("expected round-trip project %q, got %q", importedPRD.Project, roundTripPRD.Project)
		}
		if len(roundTripPRD.Gates) != len(importedPRD.Gates) {
			t.Fatalf("expected %d quality gates after round-trip, got %d", len(importedPRD.Gates), len(roundTripPRD.Gates))
		}

		h := newIngressHarness()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/v1/ingress/prd":
				h.handlePRDIngress(t, w, r)
				return
			case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/loops/"):
				h.handleLoopGet(w, r)
				return
			default:
				http.Error(w, "unexpected route", http.StatusNotFound)
			}
		}))
		defer server.Close()

		submitOut := runSmithctl(t, server.URL, "--output", "json", "prd", "submit", "--file", jsonPath, "--source-ref", ".agents/tasks/prd.json")
		loopID := mustGetIngressLoopID(t, submitOut)
		assertLoopGet(t, server.URL, loopID, "prd_story", ".agents/tasks/prd.json#US-001", "synced")
	})

	t.Run("invalid canonical prd stays blocked", func(t *testing.T) {
		validateOut := runSmithValidate(t, invalidFixture)
		var validateReport model.PRDValidationReport
		if err := json.Unmarshal(validateOut, &validateReport); err != nil {
			t.Fatalf("decode validate output: %v\n%s", err, string(validateOut))
		}
		if validateReport.Valid || validateReport.Readiness != model.PRDReadinessFail {
			t.Fatalf("expected readiness fail report, got %+v", validateReport)
		}
		if len(validateReport.Errors) != 2 {
			t.Fatalf("expected 2 blocking diagnostics, got %+v", validateReport.Errors)
		}
		if validateReport.Errors[0].Code != model.PRDDiagnosticMissingQualityGates {
			t.Fatalf("expected first diagnostic %q, got %+v", model.PRDDiagnosticMissingQualityGates, validateReport.Errors)
		}
		if validateReport.Errors[1].Code != model.PRDDiagnosticMissingStories {
			t.Fatalf("expected second diagnostic %q, got %+v", model.PRDDiagnosticMissingStories, validateReport.Errors)
		}

		h := newIngressHarness()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/v1/ingress/prd":
				h.handlePRDIngress(t, w, r)
				return
			default:
				http.Error(w, "unexpected route", http.StatusNotFound)
			}
		}))
		defer server.Close()

		submitOut, stderr, code := runSmithctlWithExitCode(server.URL, "--output", "json", "prd", "submit", "--file", invalidFixture)
		if code != 1 {
			t.Fatalf("expected smithctl submit to fail, got code=%d stderr=%s stdout=%s", code, stderr, string(submitOut))
		}

		var submitBody map[string]any
		if err := json.Unmarshal(submitOut, &submitBody); err != nil {
			t.Fatalf("decode smithctl submit output: %v\n%s", err, string(submitOut))
		}
		report, ok := submitBody["report"]
		if !ok {
			t.Fatalf("expected rejection report in submit output: %s", string(submitOut))
		}
		reportJSON, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("marshal rejection report: %v", err)
		}
		var submitReport model.PRDValidationReport
		if err := json.Unmarshal(reportJSON, &submitReport); err != nil {
			t.Fatalf("decode rejection report: %v\n%s", err, string(reportJSON))
		}
		if !reportsEqual(validateReport, submitReport) {
			t.Fatalf("expected ingress rejection to match smith validate report\nvalidate=%s\nsubmit=%s", string(validateOut), string(submitOut))
		}

		h.mu.Lock()
		defer h.mu.Unlock()
		if len(h.loops) != 0 {
			t.Fatalf("expected blocked ingress to create no loops, got %+v", h.loops)
		}
	})
}

func runSmithCLI(repoRoot string, args ...string) ([]byte, string, int) {
	fullArgs := append([]string{"run", "./cmd/smith"}, args...)
	cmd := exec.Command("go", fullArgs...)
	cmd.Dir = repoRoot
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = 1
		}
	}
	return stdout.Bytes(), stderr.String(), code
}
