package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRenderPRDMarkdownStableOutput(t *testing.T) {
	t.Parallel()

	prd := &PRD{
		Version:   1,
		Project:   "Smith PRD Validation",
		Overview:  "Normalize markdown PRDs into canonical JSON for downstream workflows.",
		Goals:     []string{"Make canonical PRD JSON deterministic.", "Preserve story ordering from markdown."},
		NonGoals:  []string{"Building a rich editor."},
		Success:   []string{"Imports preserve story IDs and quality gates."},
		Questions: []string{"Should export omit unsupported canonical sections?"},
		Rules:     []string{"Canonical JSON is the source of truth."},
		Gates:     []string{"go test ./...", "./scripts/validate-acceptance.sh"},
		Stories: []PRDStory{
			{
				ID:                 "US-001",
				Title:              "Define validation contract",
				Status:             "in_progress",
				Description:        "As a maintainer, I want validation diagnostics shared across entrypoints.",
				AcceptanceCriteria: []string{"Validation returns machine-readable diagnostics."},
			},
			{
				ID:                 "US-002",
				Title:              "Normalize markdown PRDs into canonical JSON",
				Status:             "open",
				DependsOn:          []string{"US-001"},
				Description:        "As a user, I want markdown normalized into canonical PRD JSON.",
				AcceptanceCriteria: []string{"Supported headings map into canonical JSON fields.", "Quality gates are preserved."},
			},
		},
	}

	markdown, report := prd.RenderMarkdown()
	if !report.Valid {
		t.Fatalf("expected valid report, got %+v", report)
	}

	expected := `# Smith PRD Validation

## Overview

Normalize markdown PRDs into canonical JSON for downstream workflows.

## Goals
- Make canonical PRD JSON deterministic.
- Preserve story ordering from markdown.

## Non-Goals
- Building a rich editor.

## Success Metrics
- Imports preserve story IDs and quality gates.

## Open Questions
- Should export omit unsupported canonical sections?

## Rules
- Canonical JSON is the source of truth.

## Quality Gates
- go test ./...
- ./scripts/validate-acceptance.sh

## Stories

### US-001: Define validation contract

As a maintainer, I want validation diagnostics shared across entrypoints.

#### Status

in_progress

#### Acceptance Criteria
- Validation returns machine-readable diagnostics.

### US-002: Normalize markdown PRDs into canonical JSON

As a user, I want markdown normalized into canonical PRD JSON.

#### Status

open

#### Depends On
- US-001

#### Acceptance Criteria
- Supported headings map into canonical JSON fields.
- Quality gates are preserved.`

	if markdown != expected {
		t.Fatalf("rendered markdown mismatch\nactual:\n%s\n\nexpected:\n%s", markdown, expected)
	}
}

func TestExportPRDJSONToMarkdownRoundTrip(t *testing.T) {
	t.Parallel()

	expected := &PRD{
		Version:   1,
		Project:   "Smith PRD Validation",
		Overview:  "Normalize markdown PRDs into canonical JSON for downstream workflows.",
		Goals:     []string{"Make canonical PRD JSON deterministic.", "Preserve story ordering from markdown."},
		NonGoals:  []string{"Building a rich editor."},
		Success:   []string{"Imports preserve story IDs and quality gates."},
		Questions: []string{"Should export omit unsupported canonical sections?"},
		Rules:     []string{"Canonical JSON is the source of truth."},
		Gates:     []string{"go test ./...", "./scripts/validate-acceptance.sh"},
		Stories: []PRDStory{
			{
				ID:                 "US-001",
				Title:              "Define validation contract",
				Status:             "in_progress",
				Description:        "As a maintainer, I want validation diagnostics shared across entrypoints.",
				AcceptanceCriteria: []string{"Validation returns machine-readable diagnostics."},
			},
			{
				ID:                 "US-002",
				Title:              "Normalize markdown PRDs into canonical JSON",
				Status:             "open",
				DependsOn:          []string{"US-001"},
				Description:        "As a user, I want markdown normalized into canonical PRD JSON.",
				AcceptanceCriteria: []string{"Supported headings map into canonical JSON fields.", "Quality gates are preserved."},
			},
		},
	}

	data, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("marshal prd: %v", err)
	}

	markdown, report := ExportPRDJSONToMarkdown(data)
	if !report.Valid {
		t.Fatalf("expected valid export report, got %+v", report)
	}

	actual, roundTripReport := ValidatePRDMarkdown([]byte(markdown))
	if !roundTripReport.Valid {
		t.Fatalf("expected round-trip markdown to validate, got %+v", roundTripReport)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("round-trip PRD mismatch\nactual: %#v\nexpected: %#v", actual, expected)
	}
}

func TestExportPRDJSONToMarkdownInvalidDocument(t *testing.T) {
	t.Parallel()

	expected := &PRD{
		Version:  1,
		Overview: "Canonical PRD validation",
		Stories: []PRDStory{
			{
				ID:                 "US-001",
				Title:              "Define validation contract",
				Status:             "open",
				Description:        "As a maintainer, I want shared validation.",
				AcceptanceCriteria: []string{"Validation report is shared."},
			},
		},
	}

	data, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("marshal prd: %v", err)
	}

	markdown, report := ExportPRDJSONToMarkdown(data)
	if markdown != "" {
		t.Fatalf("expected no markdown output for invalid document, got %q", markdown)
	}

	want := expected.ValidateReport()
	if !reflect.DeepEqual(report, want) {
		t.Fatalf("expected export diagnostics to match validation\nactual: %+v\nexpected: %+v", report, want)
	}
}

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
