package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/restgrep-az/restgrep/internal/backend"
)

type Backend struct {
	Repo string // e.g. "owner/repo"
}

func New(repo string) *Backend {
	return &Backend{
		Repo: repo,
	}
}

func (b *Backend) Name() string {
	return "github"
}

type GHSearchResult struct {
	Path        string `json:"path"`
	TextMatches []struct {
		Fragment string `json:"fragment"`
	} `json:"textMatches"`
}

func (b *Backend) Search(ctx context.Context, query string, opts backend.SearchOptions) ([]backend.SearchResult, error) {
	q := query
	// In GitHub code search, exact match can be wrapped in quotes
	if opts.WordRegexp {
		q = fmt.Sprintf(`"%s"`, q)
	}

	args := []string{"search", "code", q}
	if b.Repo != "" {
		args = append(args, "--repo", b.Repo)
	}
	args = append(args, "--json", "path,textMatches")

	cmd := exec.CommandContext(ctx, "gh", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh search code failed: %w, stderr: %s", err, stderr.String())
	}

	var ghResults []GHSearchResult
	if err := json.Unmarshal(stdout.Bytes(), &ghResults); err != nil {
		return nil, fmt.Errorf("failed to parse gh JSON: %w (output: %s)", err, stdout.String())
	}

	var results []backend.SearchResult
	for _, res := range ghResults {
		if len(res.TextMatches) > 0 {
			for _, match := range res.TextMatches {
				// The fragment often contains newlines with surrounding context.
				// We split them to represent line-by-line grep output.
				lines := strings.Split(match.Fragment, "\n")
				for _, line := range lines {
					// We only include lines that actually contain the query 
					// (case-insensitive for broad fallback) to strip irrelevant context lines GitHub adds
					lineToCheck := line
					queryToCheck := query
					if opts.IgnoreCase {
						lineToCheck = strings.ToLower(lineToCheck)
						queryToCheck = strings.ToLower(queryToCheck)
					}
					
					if strings.Contains(lineToCheck, queryToCheck) {
						results = append(results, backend.SearchResult{
							File:    res.Path,
							Line:    1, // GitHub textMatches do not natively provide clean line numbers without raw file parsing
							Content: strings.TrimSpace(line),
						})
					}
				}
			}
		} else {
			results = append(results, backend.SearchResult{
				File:    res.Path,
				Line:    1,
				Content: "[File match]",
			})
		}
	}

	return results, nil
}
