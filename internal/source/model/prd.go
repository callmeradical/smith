package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type PRD struct {
	Version   int         `json:"version"`
	Project   string      `json:"project"`
	Overview  string      `json:"overview"`
	Goals     []string    `json:"goals"`
	NonGoals  []string    `json:"nonGoals"`
	Success   []string    `json:"successMetrics"`
	Questions []string    `json:"openQuestions"`
	Stack     PRDStack    `json:"stack"`
	Routes    []PRDRoute  `json:"routes"`
	UI        string      `json:"uiNotes"`
	DataModel []PRDEntity `json:"dataModel"`
	Import    string      `json:"importFormat"`
	Rules     []string    `json:"rules"`
	Gates     []string    `json:"qualityGates"`
	Stories   []PRDStory  `json:"stories"`
}

type PRDStack struct {
	Framework string `json:"framework"`
	Hosting   string `json:"hosting"`
	Database  string `json:"database"`
	Auth      string `json:"auth"`
}

type PRDRoute struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
}

type PRDEntity struct {
	Name   string     `json:"name"`
	Fields []PRDField `json:"fields"`
}

type PRDField struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Note string `json:"note,omitempty"`
}

type PRDStory struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Status             string   `json:"status"` // always "open" initially
	DependsOn          []string `json:"dependsOn,omitempty"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
}

type PRDValidationReport struct {
	Valid     bool                      `json:"valid"`
	Errors    []PRDValidationDiagnostic `json:"errors,omitempty"`
	Warnings  []PRDValidationDiagnostic `json:"warnings,omitempty"`
	Readiness string                    `json:"readiness"`
}

type PRDValidationDiagnostic struct {
	Code       string `json:"code"`
	Path       string `json:"path"`
	StoryID    string `json:"storyId,omitempty"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

const (
	PRDReadinessPass = "pass"
	PRDReadinessWarn = "warn"
	PRDReadinessFail = "fail"

	PRDDiagnosticMalformedJSON           = "prd_malformed_json"
	PRDDiagnosticInvalidVersion          = "prd_invalid_version"
	PRDDiagnosticMissingProject          = "prd_missing_project"
	PRDDiagnosticMissingOverview         = "prd_missing_overview"
	PRDDiagnosticMissingQualityGates     = "prd_missing_quality_gates"
	PRDDiagnosticEmptyQualityGate        = "prd_empty_quality_gate"
	PRDDiagnosticMissingStories          = "prd_missing_stories"
	PRDDiagnosticNonCanonicalStoryID     = "prd_non_canonical_story_id"
	PRDDiagnosticDuplicateStoryID        = "prd_duplicate_story_id"
	PRDDiagnosticMissingStoryTitle       = "prd_missing_story_title"
	PRDDiagnosticMissingStoryDescription = "prd_missing_story_description"
	PRDDiagnosticMissingAcceptance       = "prd_missing_acceptance_criteria"
	PRDDiagnosticInvalidStoryStatus      = "prd_invalid_story_status"
	PRDDiagnosticMultipleInProgress      = "prd_multiple_in_progress_stories"
	PRDDiagnosticUnknownStoryDependency  = "prd_unknown_story_dependency"
	PRDDiagnosticOversizedStory          = "prd_oversized_story"
)

var (
	prdStoryIDPattern  = regexp.MustCompile(`^US-\d{3}$`)
	prdAllowedStatuses = []string{"open", "in_progress", "done"}
)

const (
	maxPRDStoryAcceptanceCriteria = 5
	maxPRDStoryCharacters         = 900
)

func (p *PRD) Validate() error {
	report := p.ValidateReport()
	if report.Valid {
		return nil
	}
	return errors.New(report.Errors[0].Message)
}

func ParsePRDJSON(data []byte) (*PRD, error) {
	var prd PRD
	if err := json.Unmarshal(data, &prd); err != nil {
		return nil, err
	}
	return &prd, nil
}

func ValidatePRDJSON(data []byte) (*PRD, PRDValidationReport) {
	prd, err := ParsePRDJSON(data)
	if err != nil {
		return nil, PRDValidationReport{
			Valid: false,
			Errors: []PRDValidationDiagnostic{
				{
					Code:       PRDDiagnosticMalformedJSON,
					Path:       "$",
					Message:    fmt.Sprintf("PRD JSON could not be parsed: %v", err),
					Suggestion: "Fix the JSON syntax and try validation again.",
				},
			},
			Readiness: PRDReadinessFail,
		}
	}
	return prd, prd.ValidateReport()
}

func (p *PRD) ValidateReport() PRDValidationReport {
	report := PRDValidationReport{
		Errors:   make([]PRDValidationDiagnostic, 0),
		Warnings: make([]PRDValidationDiagnostic, 0),
	}

	if p.Version <= 0 {
		report.Errors = append(report.Errors, PRDValidationDiagnostic{
			Code:       PRDDiagnosticInvalidVersion,
			Path:       "$.version",
			Message:    "version must be a positive integer",
			Suggestion: "Set version to the current canonical schema version.",
		})
	}
	if strings.TrimSpace(p.Project) == "" {
		report.Errors = append(report.Errors, PRDValidationDiagnostic{
			Code:       PRDDiagnosticMissingProject,
			Path:       "$.project",
			Message:    "project name is required",
			Suggestion: "Add a non-empty project name.",
		})
	}
	if strings.TrimSpace(p.Overview) == "" {
		report.Errors = append(report.Errors, PRDValidationDiagnostic{
			Code:       PRDDiagnosticMissingOverview,
			Path:       "$.overview",
			Message:    "overview is required",
			Suggestion: "Add a short overview describing the PRD scope.",
		})
	}
	if len(p.Gates) == 0 {
		report.Errors = append(report.Errors, PRDValidationDiagnostic{
			Code:       PRDDiagnosticMissingQualityGates,
			Path:       "$.qualityGates",
			Message:    "at least one quality gate is required",
			Suggestion: "Add the commands required to verify PRD work.",
		})
	}
	for i, gate := range p.Gates {
		if strings.TrimSpace(gate) != "" {
			continue
		}
		report.Errors = append(report.Errors, PRDValidationDiagnostic{
			Code:       PRDDiagnosticEmptyQualityGate,
			Path:       fmt.Sprintf("$.qualityGates[%d]", i),
			Message:    "quality gate commands must be non-empty strings",
			Suggestion: "Replace the empty value with an executable verification command.",
		})
	}
	if len(p.Stories) == 0 {
		report.Errors = append(report.Errors, PRDValidationDiagnostic{
			Code:       PRDDiagnosticMissingStories,
			Path:       "$.stories",
			Message:    "at least one story is required",
			Suggestion: "Add one or more canonical US-### stories.",
		})
	}

	storyIDs := make(map[string]int, len(p.Stories))
	inProgressStories := make([]string, 0, 1)
	for i, story := range p.Stories {
		storyPath := fmt.Sprintf("$.stories[%d]", i)
		storyID := strings.TrimSpace(story.ID)
		expectedID := fmt.Sprintf("US-%03d", i+1)
		if storyID == "" || !prdStoryIDPattern.MatchString(storyID) || storyID != expectedID {
			report.Errors = append(report.Errors, PRDValidationDiagnostic{
				Code:       PRDDiagnosticNonCanonicalStoryID,
				Path:       storyPath + ".id",
				StoryID:    storyID,
				Message:    fmt.Sprintf("story id must be canonical and sequential; expected %q", expectedID),
				Suggestion: fmt.Sprintf("Rename this story id to %q.", expectedID),
			})
		}
		if firstIndex, exists := storyIDs[storyID]; storyID != "" && exists {
			report.Errors = append(report.Errors, PRDValidationDiagnostic{
				Code:       PRDDiagnosticDuplicateStoryID,
				Path:       storyPath + ".id",
				StoryID:    storyID,
				Message:    fmt.Sprintf("story id %q duplicates $.stories[%d].id", storyID, firstIndex),
				Suggestion: "Assign each story a unique canonical id.",
			})
		} else if storyID != "" {
			storyIDs[storyID] = i
		}
		if strings.TrimSpace(story.Title) == "" {
			report.Errors = append(report.Errors, PRDValidationDiagnostic{
				Code:       PRDDiagnosticMissingStoryTitle,
				Path:       storyPath + ".title",
				StoryID:    storyID,
				Message:    "story title is required",
				Suggestion: "Add a concise story title.",
			})
		}
		if strings.TrimSpace(story.Description) == "" {
			report.Errors = append(report.Errors, PRDValidationDiagnostic{
				Code:       PRDDiagnosticMissingStoryDescription,
				Path:       storyPath + ".description",
				StoryID:    storyID,
				Message:    "story description is required",
				Suggestion: "Describe the user and desired outcome for this story.",
			})
		}
		if len(story.AcceptanceCriteria) == 0 {
			report.Errors = append(report.Errors, PRDValidationDiagnostic{
				Code:       PRDDiagnosticMissingAcceptance,
				Path:       storyPath + ".acceptanceCriteria",
				StoryID:    storyID,
				Message:    "at least one acceptance criterion is required",
				Suggestion: "Add concrete acceptance criteria for this story.",
			})
		}
		storySize := len(strings.TrimSpace(story.Title)) + len(strings.TrimSpace(story.Description))
		for _, criterion := range story.AcceptanceCriteria {
			storySize += len(strings.TrimSpace(criterion))
		}
		if len(story.AcceptanceCriteria) > maxPRDStoryAcceptanceCriteria || storySize > maxPRDStoryCharacters {
			report.Errors = append(report.Errors, PRDValidationDiagnostic{
				Code:    PRDDiagnosticOversizedStory,
				Path:    storyPath,
				StoryID: storyID,
				Message: "story is too large for a single Ralph iteration",
				Suggestion: fmt.Sprintf(
					"Split the story into smaller stories with at most %d acceptance criteria and tighter scope.",
					maxPRDStoryAcceptanceCriteria,
				),
			})
		}
		status := strings.TrimSpace(story.Status)
		if !slices.Contains(prdAllowedStatuses, status) {
			report.Errors = append(report.Errors, PRDValidationDiagnostic{
				Code:       PRDDiagnosticInvalidStoryStatus,
				Path:       storyPath + ".status",
				StoryID:    storyID,
				Message:    fmt.Sprintf("story status %q is not canonical", status),
				Suggestion: "Use one of: open, in_progress, done.",
			})
		}
		if status == "in_progress" {
			inProgressStories = append(inProgressStories, storyID)
		}
	}

	if len(inProgressStories) > 1 {
		report.Errors = append(report.Errors, PRDValidationDiagnostic{
			Code:       PRDDiagnosticMultipleInProgress,
			Path:       "$.stories",
			Message:    "only one story may be in_progress at a time",
			Suggestion: "Mark the remaining active stories as open or done.",
		})
	}

	for i, story := range p.Stories {
		storyPath := fmt.Sprintf("$.stories[%d]", i)
		storyID := strings.TrimSpace(story.ID)
		for depIndex, dependency := range story.DependsOn {
			depID := strings.TrimSpace(dependency)
			if _, ok := storyIDs[depID]; ok {
				continue
			}
			report.Errors = append(report.Errors, PRDValidationDiagnostic{
				Code:       PRDDiagnosticUnknownStoryDependency,
				Path:       fmt.Sprintf("%s.dependsOn[%d]", storyPath, depIndex),
				StoryID:    storyID,
				Message:    fmt.Sprintf("story dependency %q does not match any known story id", depID),
				Suggestion: "Reference an existing canonical story id.",
			})
		}
	}

	report.Valid = len(report.Errors) == 0
	switch {
	case len(report.Errors) > 0:
		report.Readiness = PRDReadinessFail
	case len(report.Warnings) > 0:
		report.Readiness = PRDReadinessWarn
	default:
		report.Readiness = PRDReadinessPass
	}
	return report
}
