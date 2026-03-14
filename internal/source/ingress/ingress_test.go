package ingress

import (
	"testing"

	"smith/internal/source/model"
)

func TestGitHubIssueToDraft(t *testing.T) {
	draft, err := GitHubIssueToDraft(GitHubIssue{
		Repository: "acme/smith",
		Number:     123,
		Title:      "Fix lock drift",
		Body:       "reconcile stale jobs",
		URL:        "https://github.com/acme/smith/issues/123",
		Labels:     []string{"p0", "bug"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if draft.SourceRef != "acme/smith#123" {
		t.Fatalf("unexpected source ref: %s", draft.SourceRef)
	}
	if draft.SourceType != "github_issue" {
		t.Fatalf("unexpected source type: %s", draft.SourceType)
	}
	if draft.IdempotencyKey != "github:acme/smith#123" {
		t.Fatalf("unexpected idempotency key: %s", draft.IdempotencyKey)
	}
	if got := draft.Metadata["github_issue_url"]; got == "" {
		t.Fatalf("expected github_issue_url metadata")
	}
}

func TestParsePRDMarkdown(t *testing.T) {
	doc := `# Smith MVP

## Intake
- [ ] Add GitHub ingestion endpoint
- [ ] Add PRD ingestion endpoint

## CLI
1. Scaffold smithctl resources
`
	drafts, errs := ParsePRDMarkdown(doc, "docs/prd1.md", map[string]string{"env": "test"})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %#v", errs)
	}
	if len(drafts) != 3 {
		t.Fatalf("expected 3 drafts, got %d", len(drafts))
	}
	if drafts[0].Metadata["prd_source_ref"] != "docs/prd1.md" {
		t.Fatalf("missing source mapping")
	}
	if drafts[0].SourceType != "prd_task" {
		t.Fatalf("unexpected source type: %s", drafts[0].SourceType)
	}
}

func TestPRDTasksToDraftsValidation(t *testing.T) {
	drafts, errs := PRDTasksToDrafts([]PRDTask{{Title: "ok"}, {Title: ""}}, "docs/prd.md", nil)
	if len(drafts) != 1 {
		t.Fatalf("expected 1 draft, got %d", len(drafts))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].ItemIndex != 1 {
		t.Fatalf("unexpected item index: %d", errs[0].ItemIndex)
	}
}

func TestCanonicalPRDToDrafts(t *testing.T) {
	drafts := CanonicalPRDToDrafts(&model.PRD{
		Stories: []model.PRDStory{
			{
				ID:          "US-001",
				Title:       "Define validation contract",
				Status:      "open",
				DependsOn:   []string{"US-000"},
				Description: "As a maintainer, I want shared validation.",
			},
		},
	}, "docs/prd.json", map[string]string{"env": "test"})
	if len(drafts) != 1 {
		t.Fatalf("expected 1 draft, got %d", len(drafts))
	}
	if drafts[0].SourceType != "prd_story" {
		t.Fatalf("unexpected source type: %s", drafts[0].SourceType)
	}
	if drafts[0].SourceRef != "docs/prd.json#US-001" {
		t.Fatalf("unexpected source ref: %s", drafts[0].SourceRef)
	}
	if drafts[0].Metadata["prd_story_id"] != "US-001" {
		t.Fatalf("expected prd_story_id metadata, got %#v", drafts[0].Metadata)
	}
}
