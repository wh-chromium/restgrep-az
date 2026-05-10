package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/wh-chromium/restgrep-az/internal/models"
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
	return "github"
}

type GHSearchResult struct {
	Path        string `json:"path"`
	SHA         string `json:"sha"`
	TextMatches []struct {
		Fragment string `json:"fragment"`
	} `json:"textMatches"`
}

func (b *Backend) Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.IntermediateResult, error) {
	paths := opts.Paths
	if len(paths) == 0 {
		paths = []string{""} // Root/all
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

	var allResults []models.IntermediateResult
	for _, p := range paths {
		q := query
		if opts.WordRegexp {
			q = fmt.Sprintf(`"%s"`, q)
		}

		if p != "" {
			q = fmt.Sprintf("%s path:%s", q, p)
		}

		limit := opts.Limit
		if limit <= 0 {
			limit = 100
		}

		args := []string{"search", "code", q, "--limit", fmt.Sprintf("%d", limit)}
		if b.Repo != "" {
			args = append(args, "--repo", b.Repo)
		}
		args = append(args, "--json", "path,sha,textMatches")

		if opts.Debug {
			fmt.Printf("[DEBUG][github] Executing: gh %s\n", strings.Join(args, " "))
		}

		stdout, stderr, err := b.Executor.Execute(ctx, "gh", args...)
		if err != nil {
			return nil, fmt.Errorf("gh search code failed for path %q: %w, stderr: %s", p, err, string(stderr))
		}

		if opts.Debug {
			fmt.Printf("[DEBUG][github] Raw Output: %s\n", string(stdout))
		}

		var ghResults []GHSearchResult
		if err := json.Unmarshal(stdout, &ghResults); err != nil {
			return nil, fmt.Errorf("failed to parse gh JSON: %w (output: %s)", err, string(stdout))
		}

		for _, res := range ghResults {
			if len(res.TextMatches) > 0 {
				for _, match := range res.TextMatches {
					lines := strings.Split(match.Fragment, "\n")
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
							matched = strings.Contains(lineToCheck, queryToCheck)
						}
						
						if matched {
							ir := models.IntermediateResult{
								File:        res.Path,
								RemoteSHA:   res.SHA,
								CharOffset:  -1,
								RawFragment: strings.TrimSpace(line),
								LineNumber:  1,
							}
							allResults = append(allResults, ir)
							if opts.Debug {
								fmt.Printf("[DEBUG][github] Translated Intermediate: %+v\n", ir)
							}
						}
					}
				}
			} else {
				ir := models.IntermediateResult{
					File:        res.Path,
					RemoteSHA:   res.SHA,
					CharOffset:  -1,
					RawFragment: "[File match]",
					LineNumber:  1,
				}
				allResults = append(allResults, ir)
				if opts.Debug {
					fmt.Printf("[DEBUG][github] Translated Intermediate: %+v\n", ir)
				}
			}
		}
		
		if opts.Limit > 0 && len(allResults) >= opts.Limit {
			break
		}
	}

	return allResults, nil
}
