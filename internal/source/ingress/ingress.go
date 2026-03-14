package ingress

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	api "smith/pkg/api/v1"
)

type LoopDraft struct {
	ID             string
	IdempotencyKey string
	Title          string
	Description    string
	SourceType     string
	SourceRef      string
	Metadata       map[string]string
}

// Remove redundant internal types
type ParseError struct {
	ItemIndex int
	Message   string
	SourceRef string
}

var (
	headingRegex      = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*$`)
	unorderedTask     = regexp.MustCompile(`^\s*[-*]\s+(?:\[[ xX]\]\s*)?(.+?)\s*$`)
	orderedTask       = regexp.MustCompile(`^\s*\d+[.)]\s+(.+?)\s*$`)
	nonSlugCharacters = regexp.MustCompile(`[^a-z0-9-]+`)
)

func GitHubIssueToDraft(issue api.GitHubIssue) (LoopDraft, error) {
	repo := strings.TrimSpace(issue.Repository)
	title := strings.TrimSpace(issue.Title)
	if repo == "" {
		return LoopDraft{}, fmt.Errorf("repository is required")
	}
	if issue.Number <= 0 {
		return LoopDraft{}, fmt.Errorf("number must be > 0")
	}
	if title == "" {
		return LoopDraft{}, fmt.Errorf("title is required")
	}
	sourceRef := fmt.Sprintf("%s#%d", repo, issue.Number)
	idempotency := strings.TrimSpace(issue.IdempotencyKey)
	if idempotency == "" {
		idempotency = "github:" + sourceRef
	}
	metadata := copyMetadata(issue.Metadata)
	metadata["ingress_mode"] = "github_issue"
	metadata["github_repository"] = repo
	metadata["github_issue_number"] = strconv.Itoa(issue.Number)
	if strings.TrimSpace(issue.ID) != "" {
		metadata["github_issue_id"] = strings.TrimSpace(issue.ID)
	}
	if strings.TrimSpace(issue.URL) != "" {
		metadata["github_issue_url"] = strings.TrimSpace(issue.URL)
	}
	if len(issue.Labels) > 0 {
		metadata["github_labels"] = strings.Join(issue.Labels, ",")
	}
	return LoopDraft{
		IdempotencyKey: idempotency,
		Title:          title,
		Description:    strings.TrimSpace(issue.Body),
		SourceType:     "github_issue",
		SourceRef:      sourceRef,
		Metadata:       metadata,
	}, nil
}

func PRDTasksToDrafts(tasks []api.PRDTask, sourceRef string, baseMetadata map[string]string) ([]LoopDraft, []ParseError) {
	trimmedSource := strings.TrimSpace(sourceRef)
	if trimmedSource == "" {
		trimmedSource = "prd:adhoc"
	}
	out := make([]LoopDraft, 0, len(tasks))
	errs := make([]ParseError, 0)
	for i, task := range tasks {
		title := strings.TrimSpace(task.Title)
		if title == "" {
			errs = append(errs, ParseError{ItemIndex: i, Message: "task title is required"})
			continue
		}
		section := strings.TrimSpace(task.Section)
		if section == "" {
			section = "General"
		}
		taskSourceRef := strings.TrimSpace(task.SourceRef)
		if taskSourceRef == "" {
			taskSourceRef = fmt.Sprintf("%s#%s-task-%d", trimmedSource, slug(section), i+1)
		}
		description := strings.TrimSpace(task.Description)
		if description == "" {
			description = fmt.Sprintf("PRD task from section %q", section)
		}
		metadata := copyMetadata(baseMetadata)
		for k, v := range task.Metadata {
			metadata[k] = v
		}
		metadata["ingress_mode"] = "prd"
		metadata["prd_source_ref"] = trimmedSource
		metadata["prd_section"] = section
		if strings.TrimSpace(task.ID) != "" {
			metadata["prd_task_id"] = strings.TrimSpace(task.ID)
		}
		out = append(out, LoopDraft{
			ID:             strings.TrimSpace(task.ID),
			IdempotencyKey: "prd:" + taskSourceRef,
			Title:          title,
			Description:    description,
			SourceType:     "prd_task",
			SourceRef:      taskSourceRef,
			Metadata:       metadata,
		})
	}
	return out, errs
}

func ParsePRDMarkdown(markdown, sourceRef string, baseMetadata map[string]string) ([]LoopDraft, []ParseError) {
	if strings.TrimSpace(markdown) == "" {
		return nil, []ParseError{{ItemIndex: -1, Message: "markdown document is empty"}}
	}
	trimmedSource := strings.TrimSpace(sourceRef)
	if trimmedSource == "" {
		trimmedSource = "prd:markdown"
	}
	lines := strings.Split(markdown, "\n")
	section := "General"
	tasks := make([]api.PRDTask, 0)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if m := headingRegex.FindStringSubmatch(trimmed); len(m) == 2 {
			section = strings.TrimSpace(m[1])
			continue
		}
		taskText := ""
		if m := unorderedTask.FindStringSubmatch(trimmed); len(m) == 2 {
			taskText = strings.TrimSpace(m[1])
		} else if m := orderedTask.FindStringSubmatch(trimmed); len(m) == 2 {
			taskText = strings.TrimSpace(m[1])
		}
		if taskText == "" {
			continue
		}
		tasks = append(tasks, api.PRDTask{
			Title:       taskText,
			Description: fmt.Sprintf("Extracted from PRD line %d", i+1),
			Section:     section,
			Metadata: map[string]string{
				"prd_line": strconv.Itoa(i + 1),
			},
		})
	}
	if len(tasks) == 0 {
		// Fallback: treat the whole document as a single task if no list items found
		title := "Build from PRD"
		// Try to find the first heading or the first non-empty line as title
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if m := headingRegex.FindStringSubmatch(trimmed); len(m) == 2 {
					title = strings.TrimSpace(m[1])
				} else {
					title = trimmed
					if len(title) > 60 {
						title = title[:57] + "..."
					}
				}
				break
			}
		}
		tasks = append(tasks, api.PRDTask{
			Title:       title,
			Description: markdown,
			Section:     "General",
		})
	}

	return PRDTasksToDrafts(tasks, trimmedSource, baseMetadata)
}

func copyMetadata(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func slug(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, " ", "-")
	value = nonSlugCharacters.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "section"
	}
	return value
}
