package model

import (
	"testing"
)

func TestValidatePRDReportValidCanonicalDocument(t *testing.T) {
	prd := PRD{
		Version:  1,
		Project:  "Validation",
		Overview: "Canonical PRD validation",
		Gates:    []string{"go test ./...", "./scripts/validate-acceptance.sh"},
		Stories: []PRDStory{
			{
				ID:          "US-001",
				Title:       "Define route-aware validation contract",
				Status:      "in_progress",
				Description: "As a maintainer, I want shared validation for route and package changes.",
				AcceptanceCriteria: []string{
					"`smith --prd validate` returns machine-readable diagnostics for invalid input.",
					"Invalid story dependencies are rejected before ingress begins.",
				},
			},
			{
				ID:          "US-002",
				Title:       "Reuse validation contract in package workflows",
				Status:      "open",
				DependsOn:   []string{"US-001"},
				Description: "As a maintainer, I want route and package validations reused across CLI and API flows.",
				AcceptanceCriteria: []string{
					"CLI and API validation return the same diagnostic codes.",
					"Requests with missing package references fail with the shared validation report.",
				},
			},
		},
	}

	report := prd.ValidateReport()
	if !report.Valid {
		t.Fatalf("expected report to be valid, got %+v", report)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("expected zero blocking errors, got %+v", report.Errors)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("expected zero readiness warnings, got %+v", report.Warnings)
	}
	if report.Readiness != PRDReadinessPass {
		t.Fatalf("expected readiness %q, got %q", PRDReadinessPass, report.Readiness)
	}
}

func TestValidatePRDJSONMalformedDocument(t *testing.T) {
	prd, report := ValidatePRDJSON([]byte(`{"version":1,"project":"x",`))
	if prd != nil {
		t.Fatal("expected no parsed PRD for malformed JSON")
	}
	if report.Valid {
		t.Fatal("expected malformed JSON report to be invalid")
	}
	if len(report.Errors) != 1 {
		t.Fatalf("expected one parse diagnostic, got %+v", report.Errors)
	}
	if report.Errors[0].Code != PRDDiagnosticMalformedJSON {
		t.Fatalf("expected code %q, got %q", PRDDiagnosticMalformedJSON, report.Errors[0].Code)
	}
}

func TestValidatePRDReportDuplicateIDsAndUnknownDependency(t *testing.T) {
	prd := PRD{
		Version:  1,
		Project:  "Validation",
		Overview: "Canonical PRD validation",
		Gates:    []string{"go test ./..."},
		Stories: []PRDStory{
			{
				ID:                 "US-001",
				Title:              "First story",
				Status:             "open",
				Description:        "desc",
				AcceptanceCriteria: []string{"one"},
			},
			{
				ID:                 "US-001",
				Title:              "Duplicate story",
				Status:             "open",
				DependsOn:          []string{"US-999"},
				Description:        "desc",
				AcceptanceCriteria: []string{"one"},
			},
		},
	}

	report := prd.ValidateReport()
	if report.Valid {
		t.Fatal("expected invalid report")
	}
	assertDiagnosticCode(t, report.Errors, PRDDiagnosticDuplicateStoryID)
	assertDiagnosticCode(t, report.Errors, PRDDiagnosticUnknownStoryDependency)
}

func TestValidatePRDReportMissingProjectAndQualityGates(t *testing.T) {
	prd := PRD{
		Version:  1,
		Overview: "Canonical PRD validation",
		Stories: []PRDStory{
			{
				ID:                 "US-001",
				Title:              "First story",
				Status:             "open",
				Description:        "desc",
				AcceptanceCriteria: []string{"one"},
			},
		},
	}

	report := prd.ValidateReport()
	if report.Valid {
		t.Fatal("expected invalid report")
	}
	assertDiagnosticCode(t, report.Errors, PRDDiagnosticMissingProject)
	assertDiagnosticCode(t, report.Errors, PRDDiagnosticMissingQualityGates)
}

func TestValidatePRDReportRejectsNonCanonicalStatus(t *testing.T) {
	prd := PRD{
		Version:  1,
		Project:  "Validation",
		Overview: "Canonical PRD validation",
		Gates:    []string{"go test ./..."},
		Stories: []PRDStory{
			{
				ID:                 "US-001",
				Title:              "First story",
				Status:             "completed",
				Description:        "desc",
				AcceptanceCriteria: []string{"one"},
			},
		},
	}

	report := prd.ValidateReport()
	if report.Valid {
		t.Fatal("expected invalid report")
	}
	assertDiagnosticCode(t, report.Errors, PRDDiagnosticInvalidStoryStatus)
}

func TestValidatePRDReportWarnsOnOversizedStory(t *testing.T) {
	prd := PRD{
		Version:  1,
		Project:  "Validation",
		Overview: "Canonical PRD validation",
		Gates:    []string{"go test ./..."},
		Stories: []PRDStory{
			{
				ID:          "US-001",
				Title:       "Oversized story",
				Status:      "open",
				Description: "As an operator, I want a story that tries to pack too much work into one loop.",
				AcceptanceCriteria: []string{
					"Criterion one",
					"Criterion two",
					"Criterion three",
					"Criterion four",
					"Criterion five",
					"Criterion six",
				},
			},
		},
	}

	report := prd.ValidateReport()
	if !report.Valid {
		t.Fatalf("expected warning-only report to remain valid, got %+v", report)
	}
	assertDiagnosticCode(t, report.Warnings, PRDDiagnosticOversizedStory)
	if report.Readiness != PRDReadinessWarn {
		t.Fatalf("expected readiness %q, got %q", PRDReadinessWarn, report.Readiness)
	}
}

func TestValidatePRDReportWarnsOnWeakAcceptanceAndMissingNegativeCase(t *testing.T) {
	prd := PRD{
		Version:  1,
		Project:  "Validation",
		Overview: "Canonical PRD validation",
		Gates:    []string{"go test ./..."},
		Stories: []PRDStory{
			{
				ID:          "US-001",
				Title:       "Clarify acceptance criteria",
				Status:      "open",
				Description: "As a maintainer, I want acceptance criteria that are execution-ready.",
				AcceptanceCriteria: []string{
					"UI works as expected.",
				},
			},
		},
	}

	report := prd.ValidateReport()
	if !report.Valid {
		t.Fatalf("expected warning-only report to remain valid, got %+v", report)
	}
	assertDiagnosticCode(t, report.Warnings, PRDDiagnosticWeakAcceptance)
	assertDiagnosticCode(t, report.Warnings, PRDDiagnosticMissingNegativeCase)
}

func TestValidatePRDReportRejectsBundledSurfaceRewriteWithoutDependencies(t *testing.T) {
	prd := PRD{
		Version:  1,
		Project:  "Validation",
		Overview: "Canonical PRD validation",
		Gates:    []string{"go test ./..."},
		Stories: []PRDStory{
			{
				ID:          "US-001",
				Title:       "Rewrite CLI API and UI flows together",
				Status:      "open",
				Description: "As an operator, I want the CLI, API, and UI rewritten in one pass.",
				AcceptanceCriteria: []string{
					"CLI commands render the new output shape.",
					"API endpoints return the rewritten payloads.",
					"UI screens render the new workflow.",
					"Invalid requests are rejected with shared diagnostics.",
				},
			},
		},
	}

	report := prd.ValidateReport()
	if report.Valid {
		t.Fatal("expected invalid report")
	}
	diagnostic := findDiagnostic(report.Errors, PRDDiagnosticBundledStorySurfaces)
	if diagnostic.Code == "" {
		t.Fatalf("expected diagnostic %q in %+v", PRDDiagnosticBundledStorySurfaces, report.Errors)
	}
	if diagnostic.Suggestion == "" {
		t.Fatalf("expected bundled surface diagnostic to include a suggestion: %+v", diagnostic)
	}
}

func TestValidatePRDReportRejectsFutureStoryDependency(t *testing.T) {
	prd := PRD{
		Version:  1,
		Project:  "Validation",
		Overview: "Canonical PRD validation",
		Gates:    []string{"go test ./..."},
		Stories: []PRDStory{
			{
				ID:          "US-001",
				Title:       "Build validation entrypoint",
				Status:      "open",
				DependsOn:   []string{"US-002"},
				Description: "As a maintainer, I want the entrypoint implemented after the shared core.",
				AcceptanceCriteria: []string{
					"Validation commands call the shared package.",
					"Invalid configuration is rejected before execution.",
				},
			},
			{
				ID:          "US-002",
				Title:       "Build shared validation core",
				Status:      "open",
				Description: "As a maintainer, I want reusable readiness linting.",
				AcceptanceCriteria: []string{
					"Validation rules return stable diagnostic codes.",
					"Missing dependencies are rejected before ingress.",
				},
			},
		},
	}

	report := prd.ValidateReport()
	if report.Valid {
		t.Fatal("expected invalid report")
	}
	assertDiagnosticCode(t, report.Errors, PRDDiagnosticFutureStoryDependency)
}

func assertDiagnosticCode(t *testing.T, diagnostics []PRDValidationDiagnostic, want string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == want {
			return
		}
	}
	t.Fatalf("expected diagnostic code %q in %+v", want, diagnostics)
}
