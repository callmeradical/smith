package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParsePRDMarkdownFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		markdownFile string
		expectedFile string
	}{
		{
			name:         "well formed markdown",
			markdownFile: "well_formed.md",
			expectedFile: "well_formed.expected.json",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			markdown := readFixture(t, tc.markdownFile)
			expected := loadExpectedPRD(t, tc.expectedFile)

			actual := ParsePRDMarkdown(markdown)
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("parsed PRD mismatch\nactual: %#v\nexpected: %#v", actual, expected)
			}

			report := actual.ValidateReport()
			if !report.Valid {
				t.Fatalf("expected parsed markdown to validate, got %+v", report)
			}
		})
	}
}

func TestParsePRDMarkdownPartialStructure(t *testing.T) {
	t.Parallel()

	prd, report := ValidatePRDMarkdown(readFixture(t, "partial.md"))
	if !report.Valid {
		t.Fatalf("expected partial markdown fixture to validate, got %+v", report)
	}
	if len(prd.Goals) != 1 || prd.Goals[0] != "Capture goals when present." {
		t.Fatalf("expected goals to be imported, got %#v", prd.Goals)
	}
	if len(prd.Gates) != 1 || prd.Gates[0] != "make ci-local-act" {
		t.Fatalf("expected quality gates to be imported, got %#v", prd.Gates)
	}
	if len(prd.Stories) != 2 {
		t.Fatalf("expected 2 stories, got %d", len(prd.Stories))
	}
	if prd.Stories[1].DependsOn[0] != "US-001" {
		t.Fatalf("expected dependency to be preserved, got %#v", prd.Stories[1].DependsOn)
	}
}

func TestValidatePRDMarkdownMalformedFixture(t *testing.T) {
	t.Parallel()

	_, report := ValidatePRDMarkdown(readFixture(t, "malformed.md"))
	if report.Valid {
		t.Fatalf("expected malformed markdown fixture to fail validation")
	}
	assertDiagnosticCode(t, report.Errors, PRDDiagnosticMissingQualityGates)
	assertDiagnosticCode(t, report.Errors, PRDDiagnosticMissingStories)

	for _, code := range []string{PRDDiagnosticMissingQualityGates, PRDDiagnosticMissingStories} {
		diagnostic := findDiagnostic(report.Errors, code)
		if diagnostic.Suggestion == "" {
			t.Fatalf("expected diagnostic %q to include a suggestion", code)
		}
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "prd_markdown", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func loadExpectedPRD(t *testing.T, name string) *PRD {
	t.Helper()
	data := readFixture(t, name)
	var prd PRD
	if err := json.Unmarshal(data, &prd); err != nil {
		t.Fatalf("unmarshal expected prd %s: %v", name, err)
	}
	return &prd
}

func findDiagnostic(diagnostics []PRDValidationDiagnostic, code string) PRDValidationDiagnostic {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return diagnostic
		}
	}
	return PRDValidationDiagnostic{}
}
