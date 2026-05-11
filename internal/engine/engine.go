package engine

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/wh-chromium/restgrep-az/internal/frontend"
	"github.com/wh-chromium/restgrep-az/internal/models"
	"github.com/wh-chromium/restgrep-az/internal/resolver"
)

type EngineFrontend struct {
	Frontend frontend.Frontend
	Resolver resolver.Resolver
	Limit    int
}

type Engine struct {
	frontends     []EngineFrontend
	out           io.Writer
	errOut        io.Writer
	executionMode string // "parallel" or "sequential"
}

func New(frontends []EngineFrontend, out io.Writer, errOut io.Writer, mode string) *Engine {
	if mode == "" {
		mode = "parallel"
	}
	return &Engine{frontends: frontends, out: out, errOut: errOut, executionMode: mode}
}

type frontendResultGroup struct {
	name     string
	results  []models.IntermediateResult
	resolver resolver.Resolver
	limit    int
	err      error
}

func (e *Engine) Run(ctx context.Context, query string, opts models.SearchOptions) error {
	var groups []frontendResultGroup

	if e.executionMode == "sequential" {
		for _, ef := range e.frontends {
			f := ef.Frontend
			currentOpts := opts
			if currentOpts.Limit <= 0 {
				currentOpts.Limit = ef.Limit
				if currentOpts.Limit <= 0 {
					currentOpts.Limit = 100
				}
			}

			results, err := f.Search(ctx, query, currentOpts)
			if err == nil {
				if len(results) > currentOpts.Limit {
					results = results[:currentOpts.Limit]
				}
				groups = append(groups, frontendResultGroup{
					name:     f.Name(),
					results:  results,
					resolver: ef.Resolver,
					limit:    currentOpts.Limit,
				})
				break
			}
			fmt.Fprintf(e.errOut, "[%s] Error: %v\n", f.Name(), err)
		}
	} else {
		// Parallel mode
		resultsChan := make(chan struct {
			index int
			group frontendResultGroup
		}, len(e.frontends))

		for i, ef := range e.frontends {
			go func(idx int, ef EngineFrontend) {
				f := ef.Frontend
				currentOpts := opts
				if currentOpts.Limit <= 0 {
					currentOpts.Limit = ef.Limit
					if currentOpts.Limit <= 0 {
						currentOpts.Limit = 100
					}
				}

				results, err := f.Search(ctx, query, currentOpts)
				if err != nil {
					resultsChan <- struct {
						index int
						group frontendResultGroup
					}{idx, frontendResultGroup{name: f.Name(), err: err}}
					return
				}

				if len(results) > currentOpts.Limit {
					results = results[:currentOpts.Limit]
				}
				resultsChan <- struct {
					index int
					group frontendResultGroup
				}{idx, frontendResultGroup{
					name:     f.Name(),
					results:  results,
					resolver: ef.Resolver,
					limit:    currentOpts.Limit,
				}}
			}(i, ef)
		}

		tempGroups := make([]frontendResultGroup, len(e.frontends))
		for i := 0; i < len(e.frontends); i++ {
			res := <-resultsChan
			tempGroups[res.index] = res.group
			if res.group.err != nil {
				fmt.Fprintf(e.errOut, "[%s] Error: %v\n", res.group.name, res.group.err)
			}
		}

		for _, g := range tempGroups {
			if g.err == nil && g.name != "" {
				groups = append(groups, g)
			}
		}
	}

	// 2. Map and Flatten for efficient resolution
	type resultPtr struct {
		ir           *models.IntermediateResult
		resolver     resolver.Resolver
		frontendName string
		groupIndex   int
		resultIndex  int
	}

	var allPtrs []resultPtr
	for gIdx, g := range groups {
		for rIdx := range g.results {
			allPtrs = append(allPtrs, resultPtr{
				ir:           &g.results[rIdx],
				resolver:     g.resolver,
				frontendName: g.name,
				groupIndex:   gIdx,
				resultIndex:  rIdx,
			})
		}
	}

	// Sort by filename for MRU efficiency if needed (though we call resolvers one by one now)
	sort.Slice(allPtrs, func(i, j int) bool {
		if allPtrs[i].ir.File != allPtrs[j].ir.File {
			return allPtrs[i].ir.File < allPtrs[j].ir.File
		}
		return allPtrs[i].ir.CharOffset < allPtrs[j].ir.CharOffset
	})

	// 3. Resolve all
	finalResults := make(map[*models.IntermediateResult]resolver.Result)
	for _, p := range allPtrs {
		res := p.resolver.Resolve(ctx, *p.ir, opts)
		finalResults[p.ir] = res
	}

	// 4. Output grouped by provider (Restore order)
	for _, g := range groups {
		if opts.Count {
			counts := make(map[string]int)
			for _, ir := range g.results {
				counts[ir.File]++
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
			for _, ir := range g.results {
				if !files[ir.File] {
					fmt.Fprintln(e.out, ir.File)
					files[ir.File] = true
				}
			}
		} else {
			var lastLineNum int
			var lastFile string
			seenLines := make(map[string]bool)

			for i := range g.results {
				ir := &g.results[i]
				res := finalResults[ir]

				if (opts.BeforeContext > 0 || opts.AfterContext > 0) && i > 0 {
					if res.File != lastFile || (res.Line > lastLineNum+1) {
						fmt.Fprintln(e.out, "--")
					}
				}

				if len(res.Lines) > 0 {
					for _, el := range res.Lines {
						sep := ":"
						if !el.Match {
							sep = "-"
						}
						
						var output string
						if opts.LineNumber {
							output = fmt.Sprintf("%s%s%d%s%s", res.File, sep, el.Number, sep, el.Text)
						} else {
							output = fmt.Sprintf("%s%s%s", res.File, sep, el.Text)
						}
						if el.Match && res.Message != "" {
							output += " " + res.Message
						}

						// Deduplicate
						if !seenLines[output] {
							fmt.Fprintln(e.out, output)
							seenLines[output] = true
						}
						lastLineNum = el.Number
					}
				} else {
					// Fallback to stub
					var output string
					if opts.LineNumber {
						output = fmt.Sprintf("%s:%d:%s", res.File, res.Line, res.Content)
					} else {
						output = fmt.Sprintf("%s:%s", res.File, res.Content)
					}
					if res.Message != "" {
						output += " " + res.Message
					}
					
					if !seenLines[output] {
						fmt.Fprintln(e.out, output)
						seenLines[output] = true
					}
					lastLineNum = res.Line
				}
				lastFile = res.File
			}
		}

		status := fmt.Sprintf("[%s] Showing %d results (limit: %d).", g.name, len(g.results), g.limit)
		if len(g.results) >= g.limit {
			status += " Limit reached, there might be more results."
		}
		fmt.Fprintln(e.out, status)
	}

	return nil
}
