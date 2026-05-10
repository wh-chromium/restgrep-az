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
		resultsChan := make(chan backendResultGroup, len(e.backends))
		for _, eb := range e.backends {
			go func(eb EngineBackend) {
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
					resultsChan <- backendResultGroup{name: b.Name(), err: err}
					return
				}

				if len(results) > currentOpts.Limit {
					results = results[:currentOpts.Limit]
				}
				resultsChan <- backendResultGroup{
					name:    b.Name(),
					results: results,
					limit:   currentOpts.Limit,
				}
			}(eb)
		}

		for i := 0; i < len(e.backends); i++ {
			group := <-resultsChan
			if group.err == nil {
				resultGroups = append(resultGroups, group)
			} else {
				fmt.Fprintf(e.errOut, "[%s] Error: %v\n", group.name, group.err)
			}
		}
	}

	// 1. Flatten all results
	var allResults []backend.SearchResult
	for _, group := range resultGroups {
		allResults = append(allResults, group.results...)
	}

	// 2. Sort by filename (ensures 100% cache efficiency)
	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].File != allResults[j].File {
			return allResults[i].File < allResults[j].File
		}
		return allResults[i].CharOffset < allResults[j].CharOffset
	})

	// 3. Process merged results (MRU Enrichment)
	var cachedFile string
	var cachedData []byte
	var cachedSHA string

	if opts.Count {
		counts := make(map[string]int)
		for _, r := range allResults {
			counts[r.File]++
		}
		// Print counts in sorted order
		var files []string
		for f := range counts {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			fmt.Fprintf(e.out, "%s:%d\n", f, counts[f])
		}
	} else if opts.FilesWithMatches {
		var lastFile string
		for _, r := range allResults {
			if r.File != lastFile {
				fmt.Fprintln(e.out, r.File)
				lastFile = r.File
			}
		}
	} else {
		for _, r := range allResults {
			content := r.Content
			line := r.Line

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
						content = fmt.Sprintf("%s (local file not found)", r.Content)
					}
				}

				if localPath == cachedFile {
					if cachedSHA == r.ContentId {
						content, line = getLineFromOffset(cachedData, r.CharOffset)
					} else {
						content = fmt.Sprintf("%s (local file mismatch)", r.Content)
					}
				} else if !strings.Contains(content, "local file not found") {
					content = fmt.Sprintf("%s (local file not found)", r.Content)
				}
			}

			if opts.LineNumber {
				fmt.Fprintf(e.out, "%s:%d:%s\n", r.File, line, content)
			} else {
				fmt.Fprintf(e.out, "%s:%s\n", r.File, content)
			}
		}
	}

	// 4. Final status reporting
	for _, group := range resultGroups {
		status := fmt.Sprintf("[%s] Showing %d results (limit: %d).", group.name, len(group.results), group.limit)
		if len(group.results) >= group.limit {
			status += " Limit reached, there might be more results."
		}
		fmt.Fprintln(e.out, status)
	}

	return nil
}



