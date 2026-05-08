package githubapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/wh-chromium/restgrep-az/internal/backend"
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
	return "github-api"
}

type GHAPISearchResponse struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		Path        string `json:"path"`
		ContentId   string `json:"sha"` // Use blob SHA as ContentId
		TextMatches []struct {
			Fragment string `json:"fragment"`
			Matches  []struct {
				Text    string `json:"text"`
				Indices []int  `json:"indices"`
			} `json:"matches"`
		} `json:"text_matches"`
	} `json:"items"`
}

func (b *Backend) Search(ctx context.Context, query string, opts backend.SearchOptions) ([]backend.SearchResult, error) {
	q := query
	if opts.WordRegexp {
		q = fmt.Sprintf(`"%s"`, q)
	}

	for _, p := range opts.Paths {
		q = fmt.Sprintf("%s path:%s", q, p)
	}

	if b.Repo != "" {
		q = fmt.Sprintf("%s repo:%s", q, b.Repo)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100 // GitHub API search limit per page is 100
	}

	apiUrl := fmt.Sprintf("/search/code?q=%s&per_page=%d", url.QueryEscape(q), limit)

	cmd := exec.CommandContext(ctx, "gh", "api", "-H", "Accept: application/vnd.github.v3.text-match+json", apiUrl)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh api failed: %w, stderr: %s", err, stderr.String())
	}

	var searchResp GHAPISearchResponse
	if err := json.Unmarshal(stdout.Bytes(), &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse gh api JSON: %w (output: %s)", err, stdout.String())
	}

	var results []backend.SearchResult
	for _, item := range searchResp.Items {
		if len(item.TextMatches) > 0 {
			for _, tm := range item.TextMatches {
				lines := strings.Split(tm.Fragment, "\n")
				for _, line := range lines {
					matched := false
					lineToCheck := line
					queryToCheck := query
					if opts.IgnoreCase {
						lineToCheck = strings.ToLower(lineToCheck)
						queryToCheck = strings.ToLower(queryToCheck)
					}

					if strings.Contains(lineToCheck, queryToCheck) {
						matched = true
					}

					if matched {
						// Note: GitHub doesn't give precise offsets per line in the fragment easily
						// but it gives indices relative to the fragment.
						// For now, we follow the github backend pattern.
						results = append(results, backend.SearchResult{
							File:      item.Path,
							Line:      1,
							Content:   strings.TrimSpace(line),
							ContentId: item.ContentId,
						})
					}
				}
			}
		} else {
			results = append(results, backend.SearchResult{
				File:      item.Path,
				Line:      1,
				Content:   "[File match]",
				ContentId: item.ContentId,
			})
		}
	}

	return results, nil
}
