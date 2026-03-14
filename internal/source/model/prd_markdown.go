package model

import (
	"regexp"
	"slices"
	"strings"
)

var (
	prdMarkdownHeadingPattern = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	prdMarkdownListPattern    = regexp.MustCompile(`^\s*(?:[-*+]\s+|\d+[.)]\s+)(.+?)\s*$`)
	prdMarkdownStoryPattern   = regexp.MustCompile(`(?i)\b(US-\d{3})\b(?:\s*[:\-]\s*(.+))?`)
	prdMarkdownDependencyID   = regexp.MustCompile(`US-\d{3}`)
)

func ParsePRDMarkdown(data []byte) *PRD {
	prd := &PRD{Version: 1}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	currentSection := ""
	currentStorySection := ""
	var currentStory *PRDStory
	inCodeFence := false

	flushStory := func() {
		if currentStory == nil {
			return
		}
		currentStory.Title = strings.TrimSpace(currentStory.Title)
		currentStory.Description = strings.TrimSpace(currentStory.Description)
		currentStory.Status = strings.TrimSpace(currentStory.Status)
		if currentStory.Status == "" {
			currentStory.Status = "open"
		}
		prd.Stories = append(prd.Stories, *currentStory)
		currentStory = nil
		currentStorySection = ""
	}

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, " \t")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeFence = !inCodeFence
			continue
		}
		if inCodeFence || trimmed == "" || isMarkdownNoise(trimmed) {
			continue
		}

		if heading := prdMarkdownHeadingPattern.FindStringSubmatch(trimmed); len(heading) == 3 {
			level := len(heading[1])
			title := cleanMarkdownText(heading[2])
			if id, storyTitle, ok := parseStoryHeading(title); ok {
				flushStory()
				currentSection = "stories"
				currentStory = &PRDStory{ID: id, Title: storyTitle, Status: "open"}
				continue
			}
			if currentStory != nil {
				if section, ok := mapStorySection(title); ok {
					currentStorySection = section
					continue
				}
				if level <= 2 {
					flushStory()
				}
			}
			if level == 1 && strings.TrimSpace(prd.Project) == "" {
				prd.Project = title
			}
			currentSection = mapDocumentSection(title)
			currentStorySection = ""
			continue
		}

		if item, ok := parseListItem(trimmed); ok {
			switch {
			case currentStory != nil:
				if key, _, labeled := parseLabeledValue(item); !labeled || !isStoryMetadataLabel(key) {
					if id, storyTitle, isStory := parseStoryHeading(item); currentStorySection == "" && isStory {
						flushStory()
						currentStory = &PRDStory{ID: id, Title: storyTitle, Status: "open"}
						currentSection = "stories"
						continue
					}
				}
				appendStoryListItem(currentStory, currentStorySection, item)
			case currentSection == "stories":
				if id, storyTitle, isStory := parseStoryHeading(item); isStory {
					flushStory()
					currentStory = &PRDStory{ID: id, Title: storyTitle, Status: "open"}
				}
			default:
				appendSectionItem(prd, currentSection, item)
			}
			continue
		}

		if currentStory != nil {
			appendStoryParagraph(currentStory, currentStorySection, trimmed)
			continue
		}

		if currentSection == "" || currentSection == "overview" {
			prd.Overview = appendParagraph(prd.Overview, cleanMarkdownText(trimmed))
			continue
		}
		appendSectionItem(prd, currentSection, trimmed)
	}

	flushStory()
	return prd
}

func ValidatePRDMarkdown(data []byte) (*PRD, PRDValidationReport) {
	prd := ParsePRDMarkdown(data)
	return prd, prd.ValidateReport()
}

func mapDocumentSection(raw string) string {
	switch normalizeHeading(raw) {
	case "project":
		return "project"
	case "overview", "product overview", "summary", "description":
		return "overview"
	case "goals", "goal", "objectives", "core objectives":
		return "goals"
	case "non goals", "non-goals", "nongoals":
		return "non-goals"
	case "success metrics", "metrics":
		return "success"
	case "open questions", "questions":
		return "questions"
	case "rules", "guardrails":
		return "rules"
	case "quality gates", "quality gate":
		return "gates"
	case "stories", "user stories", "ordered stories":
		return "stories"
	default:
		return ""
	}
}

func mapStorySection(raw string) (string, bool) {
	switch normalizeHeading(raw) {
	case "description", "story description":
		return "description", true
	case "acceptance criteria", "acceptance", "criteria":
		return "acceptance", true
	case "depends on", "dependencies", "dependency":
		return "dependencies", true
	case "status":
		return "status", true
	default:
		return "", false
	}
}

func appendSectionItem(prd *PRD, section, raw string) {
	item := cleanMarkdownText(raw)
	if item == "" {
		return
	}
	switch section {
	case "project":
		if prd.Project == "" {
			prd.Project = item
		}
	case "overview":
		prd.Overview = appendParagraph(prd.Overview, item)
	case "goals":
		prd.Goals = append(prd.Goals, item)
	case "non-goals":
		prd.NonGoals = append(prd.NonGoals, item)
	case "success":
		prd.Success = append(prd.Success, item)
	case "questions":
		prd.Questions = append(prd.Questions, item)
	case "rules":
		prd.Rules = append(prd.Rules, item)
	case "gates":
		prd.Gates = append(prd.Gates, item)
	default:
		prd.Overview = appendParagraph(prd.Overview, item)
	}
}

func appendStoryListItem(story *PRDStory, section, raw string) {
	item := cleanMarkdownText(raw)
	if item == "" {
		return
	}
	if key, value, ok := parseLabeledValue(item); ok {
		switch key {
		case "status":
			story.Status = normalizeStoryStatus(value)
			return
		case "depends on", "dependencies", "dependency":
			story.DependsOn = appendDependencies(story.DependsOn, value)
			return
		case "acceptance criteria", "acceptance", "criteria":
			story.AcceptanceCriteria = append(story.AcceptanceCriteria, cleanMarkdownText(value))
			return
		case "description", "story description":
			story.Description = appendParagraph(story.Description, cleanMarkdownText(value))
			return
		}
	}
	switch section {
	case "status":
		story.Status = normalizeStoryStatus(item)
	case "dependencies":
		story.DependsOn = appendDependencies(story.DependsOn, item)
	case "acceptance":
		story.AcceptanceCriteria = append(story.AcceptanceCriteria, item)
	default:
		story.AcceptanceCriteria = append(story.AcceptanceCriteria, item)
	}
}

func appendStoryParagraph(story *PRDStory, section, raw string) {
	text := cleanMarkdownText(raw)
	if text == "" {
		return
	}
	if key, value, ok := parseLabeledValue(text); ok {
		switch key {
		case "status":
			story.Status = normalizeStoryStatus(value)
			return
		case "depends on", "dependencies", "dependency":
			story.DependsOn = appendDependencies(story.DependsOn, value)
			return
		case "acceptance criteria", "acceptance", "criteria":
			story.AcceptanceCriteria = append(story.AcceptanceCriteria, cleanMarkdownText(value))
			return
		}
	}
	switch section {
	case "acceptance":
		story.AcceptanceCriteria = append(story.AcceptanceCriteria, text)
	case "dependencies":
		story.DependsOn = appendDependencies(story.DependsOn, text)
	case "status":
		story.Status = normalizeStoryStatus(text)
	default:
		story.Description = appendParagraph(story.Description, text)
	}
}

func appendDependencies(existing []string, raw string) []string {
	matches := prdMarkdownDependencyID.FindAllString(strings.ToUpper(raw), -1)
	for _, match := range matches {
		if !slices.Contains(existing, match) {
			existing = append(existing, match)
		}
	}
	return existing
}

func parseStoryHeading(raw string) (string, string, bool) {
	matches := prdMarkdownStoryPattern.FindStringSubmatch(raw)
	if len(matches) == 0 {
		return "", "", false
	}
	id := strings.ToUpper(matches[1])
	title := cleanMarkdownText(matches[2])
	if title == "" {
		trimmed := strings.TrimSpace(prdMarkdownStoryPattern.ReplaceAllString(raw, ""))
		title = cleanMarkdownText(strings.TrimLeft(trimmed, ":- "))
	}
	return id, title, true
}

func parseListItem(raw string) (string, bool) {
	matches := prdMarkdownListPattern.FindStringSubmatch(raw)
	if len(matches) != 2 {
		return "", false
	}
	return strings.TrimSpace(matches[1]), true
}

func parseLabeledValue(raw string) (string, string, bool) {
	before, after, ok := strings.Cut(raw, ":")
	if !ok {
		return "", "", false
	}
	key := normalizeHeading(before)
	value := strings.TrimSpace(after)
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}

func normalizeHeading(raw string) string {
	value := cleanMarkdownText(raw)
	value = strings.ToLower(value)
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, "0123456789. ")
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func cleanMarkdownText(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, "*`_")
	value = strings.TrimPrefix(value, "[ ] ")
	value = strings.TrimPrefix(value, "[x] ")
	value = strings.TrimPrefix(value, "[X] ")
	value = strings.TrimSpace(value)
	return value
}

func normalizeStoryStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "in progress":
		return "in_progress"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func appendParagraph(existing, raw string) string {
	text := cleanMarkdownText(raw)
	if text == "" {
		return existing
	}
	if strings.TrimSpace(existing) == "" {
		return text
	}
	return existing + "\n\n" + text
}

func isMarkdownNoise(raw string) bool {
	return strings.HasPrefix(raw, "---") || strings.HasPrefix(raw, "| ---")
}

func isStoryMetadataLabel(key string) bool {
	return key == "status" || key == "depends on" || key == "dependencies" || key == "dependency" || key == "acceptance criteria" || key == "acceptance" || key == "criteria" || key == "description" || key == "story description"
}
