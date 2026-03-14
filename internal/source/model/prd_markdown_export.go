package model

import (
	"strings"
)

// RenderMarkdown renders the parser-supported canonical PRD sections in a
// stable markdown layout. Invalid PRDs return the same validation diagnostics
// and no markdown output.
func (p *PRD) RenderMarkdown() (string, PRDValidationReport) {
	if p == nil {
		report := PRDValidationReport{
			Valid: false,
			Errors: []PRDValidationDiagnostic{
				{
					Code:       PRDDiagnosticMalformedJSON,
					Path:       "$",
					Message:    "PRD document is required",
					Suggestion: "Provide a canonical PRD document before exporting markdown.",
				},
			},
			Readiness: PRDReadinessFail,
		}
		return "", report
	}

	report := p.ValidateReport()
	if !report.Valid {
		return "", report
	}

	var sections []string
	sections = append(sections, "# "+strings.TrimSpace(p.Project))
	sections = append(sections, "## Overview\n\n"+strings.TrimSpace(p.Overview))
	sections = appendOptionalListSection(sections, "Goals", p.Goals)
	sections = appendOptionalListSection(sections, "Non-Goals", p.NonGoals)
	sections = appendOptionalListSection(sections, "Success Metrics", p.Success)
	sections = appendOptionalListSection(sections, "Open Questions", p.Questions)
	sections = appendOptionalListSection(sections, "Rules", p.Rules)
	sections = appendOptionalListSection(sections, "Quality Gates", p.Gates)
	sections = append(sections, renderStoriesSection(p.Stories))

	return strings.Join(sections, "\n\n"), report
}

func ExportPRDJSONToMarkdown(data []byte) (string, PRDValidationReport) {
	prd, report := ValidatePRDJSON(data)
	if !report.Valid {
		return "", report
	}
	return prd.RenderMarkdown()
}

func appendOptionalListSection(sections []string, title string, items []string) []string {
	if len(items) == 0 {
		return sections
	}

	lines := make([]string, 0, len(items)+1)
	lines = append(lines, "## "+title)
	for _, item := range items {
		lines = append(lines, "- "+strings.TrimSpace(item))
	}
	return append(sections, strings.Join(lines, "\n"))
}

func renderStoriesSection(stories []PRDStory) string {
	storySections := make([]string, 0, len(stories)+1)
	storySections = append(storySections, "## Stories")
	for _, story := range stories {
		storySections = append(storySections, renderStory(story))
	}
	return strings.Join(storySections, "\n\n")
}

func renderStory(story PRDStory) string {
	parts := []string{
		"### " + strings.TrimSpace(story.ID) + ": " + strings.TrimSpace(story.Title),
		strings.TrimSpace(story.Description),
		"#### Status\n\n" + strings.TrimSpace(story.Status),
	}

	if len(story.DependsOn) > 0 {
		dependencies := make([]string, 0, len(story.DependsOn)+1)
		dependencies = append(dependencies, "#### Depends On")
		for _, dependency := range story.DependsOn {
			dependencies = append(dependencies, "- "+strings.TrimSpace(dependency))
		}
		parts = append(parts, strings.Join(dependencies, "\n"))
	}

	acceptance := make([]string, 0, len(story.AcceptanceCriteria)+1)
	acceptance = append(acceptance, "#### Acceptance Criteria")
	for _, criterion := range story.AcceptanceCriteria {
		acceptance = append(acceptance, "- "+strings.TrimSpace(criterion))
	}
	parts = append(parts, strings.Join(acceptance, "\n"))

	return strings.Join(parts, "\n\n")
}
