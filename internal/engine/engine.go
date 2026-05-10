package engine

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/wh-chromium/restgrep-az/internal/backend"
)

type EngineBackend struct {
	Backend backend.Backend
	Limit   int
}

type Engine struct {
	backends      []EngineBackend
	out           io.Writer
	errOut        io.Writer
	executionMode string // "parallel" or "sequential"
}

func New(backends []EngineBackend, out io.Writer, errOut io.Writer, mode string) *Engine {
	if mode == "" {
		mode = "parallel" // Default
	}
	return &Engine{backends: backends, out: out, errOut: errOut, executionMode: mode}
}

func getGitBlobSHA1(data []byte) string {
	h := sha1.New()
	h.Write([]byte(fmt.Sprintf("blob %d\x00", len(data))))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func getLineFromOffset(data []byte, charOffset int) (string, int) {
	line := 1
	lineStart := 0
	for i := 0; i < charOffset && i < len(data); i++ {
		if data[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	lineEnd := len(data)
	for i := charOffset; i < len(data); i++ {
		if data[i] == '\n' || data[i] == '\r' {
			lineEnd = i
			break
		}
	}
	return string(data[lineStart:lineEnd]), line
}

type backendResultGroup struct {
	name    string
	results []backend.SearchResult
	limit   int
	err     error
}

func (e *Engine) Run(ctx context.Context, query string, opts backend.SearchOptions) error {
	var resultGroups []backendResultGroup

	if e.executionMode == "sequential" {
		for _, eb := range e.backends {
			b := eb.Backend
			currentOpts := opts
			if currentOpts.Limit <= 0 {
				currentOpts.Limit = eb.Limit
				if currentOpts.Limit <= 0 {
					currentOpts.Limit = 100
				}
			}

			results, err := b.Search(ctx, query, currentOpts)
			if err == nil {
				if len(results) > currentOpts.Limit {
					results = results[:currentOpts.Limit]
				}
				resultGroups = append(resultGroups, backendResultGroup{
					name:    b.Name(),
					results: results,
					limit:   currentOpts.Limit,
				})
				break // Stop after first successful execution
			}
			fmt.Fprintf(e.errOut, "[%s] Error: %v\n", b.Name(), err)
		}
	} else {
		// Parallel mode
		resultGroups = make([]backendResultGroup, len(e.backends))
		resultsChan := make(chan struct {
			index int
			group backendResultGroup
		}, len(e.backends))

		for i, eb := range e.backends {
			go func(idx int, eb EngineBackend) {
				b := eb.Backend
				currentOpts := opts
				if currentOpts.Limit <= 0 {
					currentOpts.Limit = eb.Limit
					if currentOpts.Limit <= 0 {
						currentOpts.Limit = 100
					}
				}

				results, err := b.Search(ctx, query, currentOpts)
				if err != nil {
					resultsChan <- struct {
						index int
						group backendResultGroup
					}{idx, backendResultGroup{name: b.Name(), err: err}}
					return
				}

				if len(results) > currentOpts.Limit {
					results = results[:currentOpts.Limit]
				}
				resultsChan <- struct {
					index int
					group backendResultGroup
				}{idx, backendResultGroup{
					name:    b.Name(),
					results: results,
					limit:   currentOpts.Limit,
				}}
			}(i, eb)
		}

		for i := 0; i < len(e.backends); i++ {
			res := <-resultsChan
			resultGroups[res.index] = res.group
			if res.group.err != nil {
				fmt.Fprintf(e.errOut, "[%s] Error: %v\n", res.group.name, res.group.err)
			}
		}

		// Filter out failed backends for processing, but maintain order of successes
		var successfulGroups []backendResultGroup
		for _, g := range resultGroups {
			if g.err == nil && g.name != "" { // check name to skip uninitialized if any
				successfulGroups = append(successfulGroups, g)
			}
		}
		resultGroups = successfulGroups
	}

	if !opts.Count && !opts.FilesWithMatches {
		// 1. Prepare sortable slice of pointers to original results
		var allPointers []*backend.SearchResult
		for bIdx := range resultGroups {
			for rIdx := range resultGroups[bIdx].results {
				allPointers = append(allPointers, &resultGroups[bIdx].results[rIdx])
			}
		}

		// 2. Sort by filename for 100% cache efficiency during enrichment
		sort.Slice(allPointers, func(i, j int) bool {
			if allPointers[i].File != allPointers[j].File {
				return allPointers[i].File < allPointers[j].File
			}
			return allPointers[i].CharOffset < allPointers[j].CharOffset
		})

		// 3. Process enrichment (MRU Cache)
		var cachedFile string
		var cachedData []byte
		var cachedSHA string

		for _, r := range allPointers {
			if r.ContentId != "" {
				localPath := strings.TrimPrefix(r.File, "/")
				if localPath != cachedFile {
					data, err := os.ReadFile(localPath)
					if err == nil {
						cachedFile = localPath
						cachedData = data
						cachedSHA = getGitBlobSHA1(data)
					} else {
						cachedFile = ""
						cachedData = nil
						cachedSHA = ""
						r.Content = fmt.Sprintf("%s (local file not found)", r.Content)
					}
				}

				if localPath == cachedFile {
					if cachedSHA == r.ContentId {
						r.Content, r.Line = getLineFromOffset(cachedData, r.CharOffset)
					} else {
						r.Content = fmt.Sprintf("%s (local file mismatch)", r.Content)
					}
				} else if !strings.Contains(r.Content, "local file not found") {
					r.Content = fmt.Sprintf("%s (local file not found)", r.Content)
				}
			}
		}
	}

	// 4. Output grouped by provider
	for _, group := range resultGroups {
		if opts.Count {
			counts := make(map[string]int)
			for _, r := range group.results {
				counts[r.File]++
			}
			// Print counts for this provider in filename order
			var files []string
			for f := range counts {
				files = append(files, f)
			}
			sort.Strings(files)
			for _, f := range files {
				fmt.Fprintf(e.out, "%s:%d\n", f, counts[f])
			}
		} else if opts.FilesWithMatches {
			files := make(map[string]bool)
			for _, r := range group.results {
				if !files[r.File] {
					fmt.Fprintln(e.out, r.File)
					files[r.File] = true
				}
			}
		} else {
			for _, r := range group.results {
				if opts.LineNumber {
					fmt.Fprintf(e.out, "%s:%d:%s\n", r.File, r.Line, r.Content)
				} else {
					fmt.Fprintf(e.out, "%s:%s\n", r.File, r.Content)
				}
			}
		}

		// Per-provider status reporting
		status := fmt.Sprintf("[%s] Showing %d results (limit: %d).", group.name, len(group.results), group.limit)
		if len(group.results) >= group.limit {
			status += " Limit reached, there might be more results."
		}
		fmt.Fprintln(e.out, status)
	}

	return nil
}



