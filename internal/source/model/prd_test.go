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
				ID:                 "US-001",
				Title:              "Define validation contract",
				Status:             "in_progress",
				Description:        "As a maintainer, I want shared validation.",
				AcceptanceCriteria: []string{"Validation report is shared."},
			},
			{
				ID:                 "US-002",
				Title:              "Reuse validation contract",
				Status:             "open",
				DependsOn:          []string{"US-001"},
				Description:        "As a maintainer, I want API and CLI parity.",
				AcceptanceCriteria: []string{"CLI and API share the same rules."},
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

func TestValidatePRDReportRejectsOversizedStory(t *testing.T) {
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
	if report.Valid {
		t.Fatal("expected invalid report")
	}
	assertDiagnosticCode(t, report.Errors, PRDDiagnosticOversizedStory)
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
