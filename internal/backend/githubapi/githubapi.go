package githubapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"

	"github.com/wh-chromium/restgrep-az/internal/backend"
)

type Executor interface {
	Execute(ctx context.Context, name string, args ...string) ([]byte, []byte, error)
}

type RealExecutor struct{}

func (e *RealExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

type Backend struct {
	Repo     string // e.g. "owner/repo"
	Executor Executor
}

func New(repo string) *Backend {
	return &Backend{
		Repo:     repo,
		Executor: &RealExecutor{},
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
	paths := opts.Paths
	if len(paths) == 0 {
		paths = []string{""}
	}

	var wordRe *regexp.Regexp
	if opts.WordRegexp {
		pattern := `\b` + regexp.QuoteMeta(query) + `\b`
		if opts.IgnoreCase {
			pattern = `(?i)\b` + regexp.QuoteMeta(query) + `\b`
		}
		var err error
		wordRe, err = regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid word-regexp: %w", err)
		}
	}

	var allResults []backend.SearchResult
	for _, p := range paths {
		q := query
		if opts.WordRegexp {
			q = fmt.Sprintf(`"%s"`, q)
		}

		if p != "" {
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

		stdout, stderr, err := b.Executor.Execute(ctx, "gh", "api", "-H", "Accept: application/vnd.github.v3.text-match+json", apiUrl)
		if err != nil {
			return nil, fmt.Errorf("gh api failed for path %q: %w, stderr: %s", p, err, string(stderr))
		}

		var searchResp GHAPISearchResponse
		if err := json.Unmarshal(stdout, &searchResp); err != nil {
			return nil, fmt.Errorf("failed to parse gh api JSON: %w (output: %s)", err, string(stdout))
		}

		for _, item := range searchResp.Items {
			if len(item.TextMatches) > 0 {
				for _, tm := range item.TextMatches {
					lines := strings.Split(tm.Fragment, "\n")
					for _, line := range lines {
						matched := false
						if opts.WordRegexp {
							matched = wordRe.MatchString(line)
						} else {
							lineToCheck := line
							queryToCheck := query
							if opts.IgnoreCase {
								lineToCheck = strings.ToLower(lineToCheck)
								queryToCheck = strings.ToLower(queryToCheck)
							}

							if strings.Contains(lineToCheck, queryToCheck) {
								matched = true
							}
						}

						if matched {
							allResults = append(allResults, backend.SearchResult{
								File:      item.Path,
								Line:      1,
								Content:   strings.TrimSpace(line),
								ContentId: item.ContentId,
							})
						}
					}
				}
			} else {
				allResults = append(allResults, backend.SearchResult{
					File:      item.Path,
					Line:      1,
					Content:   "[File match]",
					ContentId: item.ContentId,
				})
			}
		}
		
		if opts.Limit > 0 && len(allResults) >= opts.Limit {
			break
		}
	}

	return allResults, nil
}
