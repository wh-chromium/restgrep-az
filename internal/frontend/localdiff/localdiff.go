package localdiff

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/wh-chromium/restgrep-az/internal/models"
)

type Backend struct {
	MergeBaseBranch string
}

func New(branch string) *Backend {
	return &Backend{MergeBaseBranch: branch}
}

func (b *Backend) Name() string {
	return "local-diff-add"
}

func (b *Backend) Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.IntermediateResult, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return nil, fmt.Errorf("failed to open git repo: %w", err)
	}

	branch := b.MergeBaseBranch
	if branch == "" {
		branch = opts.MergeBaseBranch
	}
	if branch == "" {
		branch = "origin/main"
	}

	// 1. Get HEAD
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}
	headCommit, _ := repo.CommitObject(head.Hash())

	// 2. Get Target Branch
	ref, err := repo.Reference(plumbing.ReferenceName("refs/remotes/"+branch), true)
	if err != nil {
		ref, _ = repo.Reference(plumbing.ReferenceName("refs/heads/"+branch), true)
	}
	if ref == nil {
		return nil, fmt.Errorf("branch %s not found", branch)
	}
	targetCommit, _ := repo.CommitObject(ref.Hash())

	// 3. Find Merge Base
	bases, err := headCommit.MergeBase(targetCommit)
	if err != nil || len(bases) == 0 {
		return nil, fmt.Errorf("failed to find merge base between HEAD and %s", branch)
	}
	mergeBase := bases[0]

	if opts.Debug {
		fmt.Printf("[DEBUG][local-diff-add] Merge base: %s\n", mergeBase.Hash)
	}

	// 4. Get Diff between Merge Base and Current HEAD (or worktree?)
	// Let's compare mergeBase tree vs HEAD tree for simplicity first, 
	// then maybe worktree.
	baseTree, _ := mergeBase.Tree()
	headTree, _ := headCommit.Tree()
	
	patch, err := baseTree.Patch(headTree)
	if err != nil {
		return nil, fmt.Errorf("failed to compute patch: %w", err)
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

	var results []models.IntermediateResult
	
	// 5. Iterate through file patches
	for _, fp := range patch.FilePatches() {
		from, to := fp.Files()
		var targetFile string
		if to != nil {
			targetFile = to.Path()
		} else if from != nil {
			targetFile = from.Path()
		}

		// Filter by path if requested
		if len(opts.Paths) > 0 {
			match := false
			for _, p := range opts.Paths {
				if strings.HasPrefix(targetFile, p) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		for _, hunk := range fp.Chunks() {
			if hunk.Type() == 1 { // 1 is Add in diffmatchpatch/go-git
				lines := strings.Split(hunk.Content(), "\n")
				for _, line := range lines {
					if line == "" {
						continue
					}
					
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
						results = append(results, models.IntermediateResult{
							File:        targetFile,
							RawFragment: strings.TrimSpace(line),
							CharOffset:  -1, // Offset in a patch is not a file offset
							LineNumber:  1,  // Line number in a patch is relative
						})
					}
				}
			}
		}
	}

	return results, nil
}
