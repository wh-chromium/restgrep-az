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

type enrichedLine struct {
	text   string
	number int
	match  bool
}

func getLinesWithContext(data []byte, charOffset int, before, after int) []enrichedLine {
	// 1. Find the match line start and number
	matchLine := 1
	lineStart := 0
	for i := 0; i < charOffset && i < len(data); i++ {
		if data[i] == '\n' {
			matchLine++
			lineStart = i + 1
		}
	}

	// Helper to get line end
	getLineEnd := func(start int) int {
		for i := start; i < len(data); i++ {
			if data[i] == '\n' || data[i] == '\r' {
				return i
			}
		}
		return len(data)
	}

	// 2. Collect match line
	matchEnd := getLineEnd(lineStart)
	lines := []enrichedLine{{text: string(data[lineStart:matchEnd]), number: matchLine, match: true}}

	// 3. Collect "before" context
	currStart := lineStart
	for i := 0; i < before; i++ {
		if currStart <= 0 {
			break
		}
		// Look for start of previous line
		searchIdx := currStart - 2
		if searchIdx < 0 {
			break
		}
		prevStart := 0
		for j := searchIdx; j >= 0; j-- {
			if data[j] == '\n' {
				prevStart = j + 1
				break
			}
		}
		prevEnd := getLineEnd(prevStart)
		lines = append([]enrichedLine{{text: string(data[prevStart:prevEnd]), number: matchLine - (i + 1), match: false}}, lines...)
		currStart = prevStart
	}

	// 4. Collect "after" context
	currEnd := matchEnd
	for i := 0; i < after; i++ {
		if currEnd >= len(data) {
			break
		}
		nextStart := currEnd + 1
		if nextStart >= len(data) {
			break
		}
		nextEnd := getLineEnd(nextStart)
		lines = append(lines, enrichedLine{text: string(data[nextStart:nextEnd]), number: matchLine + (i + 1), match: false})
		currEnd = nextEnd
	}

	return lines
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

	type searchResultEnriched struct {
		file      string
		lines     []enrichedLine
		content   string // fallback if not local
		line      int    // fallback if not local
		contentId string
	}

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

	// 3. Result storage with context
	enrichedResults := make(map[*backend.SearchResult]searchResultEnriched)

	if !opts.Count && !opts.FilesWithMatches {
		var cachedFile string
		var cachedData []byte
		var cachedSHA string

		for _, r := range allPointers {
			enr := searchResultEnriched{file: r.File, content: r.Content, line: r.Line, contentId: r.ContentId}
			
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
						enr.content = fmt.Sprintf("%s (local file not found)", r.Content)
					}
				}

				if localPath == cachedFile {
					if cachedSHA == r.ContentId {
						enr.lines = getLinesWithContext(cachedData, r.CharOffset, opts.BeforeContext, opts.AfterContext)
					} else {
						enr.content = fmt.Sprintf("%s (local file mismatch)", r.Content)
					}
				} else if !strings.Contains(enr.content, "local file not found") {
					enr.content = fmt.Sprintf("%s (local file not found)", r.Content)
				}
			}
			enrichedResults[r] = enr
		}
	}

	// 4. Output grouped by provider
	for _, group := range resultGroups {
		if opts.Count {
			counts := make(map[string]int)
			for _, r := range group.results {
				counts[r.File]++
			}
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
			var lastLineNum int
			var lastFile string
			for i := range group.results {
				enr := enrichedResults[&group.results[i]]
				
				// Standard grep behavior: print -- separator if there is a gap between matches in the same file
				// or between different files if context is enabled.
				if (opts.BeforeContext > 0 || opts.AfterContext > 0) && i > 0 {
					if enr.file != lastFile || (len(enr.lines) > 0 && enr.lines[0].number > lastLineNum+1) {
						fmt.Fprintln(e.out, "--")
					}
				}

				if len(enr.lines) > 0 {
					for _, el := range enr.lines {
						sep := ":"
						if !el.match {
							sep = "-"
						}
						if opts.LineNumber {
							fmt.Fprintf(e.out, "%s%s%d%s%s\n", enr.file, sep, el.number, sep, el.text)
						} else {
							fmt.Fprintf(e.out, "%s%s%s\n", enr.file, sep, el.text)
						}
						lastLineNum = el.number
					}
				} else {
					// Fallback to stub
					if opts.LineNumber {
						fmt.Fprintf(e.out, "%s:%d:%s\n", enr.file, enr.line, enr.content)
					} else {
						fmt.Fprintf(e.out, "%s:%s\n", enr.file, enr.content)
					}
					lastLineNum = enr.line
				}
				lastFile = enr.file
			}
		}

		// Status reporting
		status := fmt.Sprintf("[%s] Showing %d results (limit: %d).", group.name, len(group.results), group.limit)
		if len(group.results) >= group.limit {
			status += " Limit reached, there might be more results."
		}
		fmt.Fprintln(e.out, status)
	}

	return nil
}



