package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/restgrep-az/restgrep/internal/backend"
	"github.com/restgrep-az/restgrep/internal/backend/azure"
	"github.com/restgrep-az/restgrep/internal/backend/github"
	"github.com/restgrep-az/restgrep/internal/config"
	"github.com/restgrep-az/restgrep/internal/engine"
)

func main() {
	var opts backend.SearchOptions
	flag.BoolVar(&opts.IgnoreCase, "i", false, "Ignore case distinctions in patterns and input data")
	flag.BoolVar(&opts.IgnoreCase, "ignore-case", false, "Ignore case distinctions in patterns and input data")
	flag.BoolVar(&opts.LineNumber, "n", false, "Prefix each line of output with the 1-based line number")
	flag.BoolVar(&opts.LineNumber, "line-number", false, "Prefix each line of output with the 1-based line number")
	flag.BoolVar(&opts.Count, "c", false, "Suppress normal output; instead print a count of matching lines for each input file")
	flag.BoolVar(&opts.Count, "count", false, "Suppress normal output; instead print a count of matching lines for each input file")
	flag.BoolVar(&opts.FilesWithMatches, "l", false, "Suppress normal output; instead print the name of each input file")
	flag.BoolVar(&opts.FilesWithMatches, "files-with-matches", false, "Suppress normal output; instead print the name of each input file")
	flag.BoolVar(&opts.WordRegexp, "w", false, "Force PATTERN to match only whole words")
	flag.BoolVar(&opts.WordRegexp, "word-regexp", false, "Force PATTERN to match only whole words")
	flag.IntVar(&opts.Limit, "m", 0, "Stop after NUM matches")
	flag.IntVar(&opts.Limit, "max-count", 0, "Stop after NUM matches")

	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: restgrep [OPTION]... PATTERN")
		os.Exit(1)
	}

	query := args[0]

	// Load configuration
	cfg, err := config.Load("restgrep.json")
	if err != nil {
		if os.IsNotExist(err) {
			// Provide a default config if it doesn't exist to make it easier to test
			cfg = &config.Config{
				Backends: []config.BackendConfig{
					{Type: "azure", Organization: "fabrikam", Project: "MyFirstProject", Limit: 100},
				},
			}
		} else {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
	}

	var backends []engine.EngineBackend
	for _, bCfg := range cfg.Backends {
		limit := bCfg.Limit
		if limit <= 0 {
			limit = 100
		}
		var b backend.Backend
		switch bCfg.Type {
		case "azure":
			b = azure.New(bCfg.Organization, bCfg.Project)
		case "github":
			b = github.New(bCfg.Repo)
		default:
			fmt.Fprintf(os.Stderr, "Unknown backend type: %s\n", bCfg.Type)
			continue
		}
		backends = append(backends, engine.EngineBackend{
			Backend: b,
			Limit:   limit,
		})
	}

	eng := engine.New(backends, os.Stdout)

	ctx := context.Background()
	if err := eng.Run(ctx, query, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
		os.Exit(1)
	}
}
