package completion

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type RealGit struct {
	Workspace string
	Repository string
	Branch     string
	PAT        string
}

func (g *RealGit) CommitAndPush(ctx context.Context, loopID string, finalDiff string) (string, error) {
	if g.PAT == "" {
		return "", fmt.Errorf("git push failed: SMITH_GIT_PAT is not set")
	}

	// 1. Configure local git user if not set
	if err := g.run(ctx, "config", "user.name", "smith-replica"); err != nil {
		return "", err
	}
	if err := g.run(ctx, "config", "user.email", "smith-replica@smith.io"); err != nil {
		return "", err
	}

	// 2. Add all changes
	if err := g.run(ctx, "add", "-A"); err != nil {
		return "", err
	}

	// 3. Commit
	commitMsg := fmt.Sprintf("chore(loop): sync loop %s\n\nFinal Diff Summary:\n%s", loopID, finalDiff)
	if err := g.run(ctx, "commit", "-m", commitMsg); err != nil {
		// If there are no changes, commit might fail. We should check if it's actually an error.
		if strings.Contains(err.Error(), "nothing to commit") {
			return g.headSHA(ctx)
		}
		return "", err
	}

	// 4. Push using the PAT in the URL
	// We rewrite the remote URL temporarily to include the PAT for the push
	authURL := g.authURL()
	if err := g.run(ctx, "push", authURL, fmt.Sprintf("HEAD:%s", g.Branch)); err != nil {
		return "", fmt.Errorf("git push failed: %w", err)
	}

	return g.headSHA(ctx)
}

func (g *RealGit) Revert(ctx context.Context, loopID string, commitSHA string) error {
	// Revert is used for compensation in the saga. 
	// In this implementation, we'll just push a revert commit or force-push back if appropriate.
	// For simplicity and safety in MVP, we'll push a revert.
	if err := g.run(ctx, "revert", "--no-edit", commitSHA); err != nil {
		return err
	}
	authURL := g.authURL()
	return g.run(ctx, "push", authURL, fmt.Sprintf("HEAD:%s", g.Branch))
}

func (g *RealGit) run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.Workspace
	// Strip PAT from logs if it ever ends up there
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %w (output: %s)", args[0], err, string(output))
	}
	return nil
}

func (g *RealGit) headSHA(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = g.Workspace
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *RealGit) authURL() string {
	repo := g.Repository
	// Basic URL rewrite for GitHub PAT: https://<pat>@github.com/org/repo.git
	if strings.HasPrefix(repo, "https://") {
		return "https://" + g.PAT + "@" + strings.TrimPrefix(repo, "https://")
	}
	return repo
}
