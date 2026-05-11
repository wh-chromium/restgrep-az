package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/wh-chromium/restgrep-az/internal/config"
	"github.com/wh-chromium/restgrep-az/internal/engine"
	"github.com/wh-chromium/restgrep-az/internal/frontend/azure"
	"github.com/wh-chromium/restgrep-az/internal/frontend/github"
	"github.com/wh-chromium/restgrep-az/internal/frontend/githubapi"
	"github.com/wh-chromium/restgrep-az/internal/frontend/localdiff"
	"github.com/wh-chromium/restgrep-az/internal/models"
	"github.com/wh-chromium/restgrep-az/internal/resolver"
)

func main() {
	// 1. Load configuration first to use as defaults
	cfg, err := config.Load("restgrep.json")
	if err != nil {
		if os.IsNotExist(err) {
			cfg = &config.Config{
				Backends: []config.BackendConfig{
					{Type: "azure", Organization: "fabrikam", Project: "MyFirstProject", Limit: 100},
				},
				ExecutionMode: "parallel",
			}
		} else {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
	}

	// 2. Define and parse flags
	var opts models.SearchOptions
	flag.BoolVar(&opts.IgnoreCase, "i", false, "Ignore case distinctions")
	flag.BoolVar(&opts.LineNumber, "n", false, "Prefix each line with 1-based line number")
	flag.BoolVar(&opts.Count, "c", false, "Print a count of matching lines for each file")
	flag.BoolVar(&opts.FilesWithMatches, "l", false, "Print only names of files with matches")
	flag.BoolVar(&opts.WordRegexp, "w", false, "Force PATTERN to match only whole words")
	flag.IntVar(&opts.Limit, "m", 0, "Stop after NUM matches")
	flag.IntVar(&opts.AfterContext, "A", 0, "Print NUM lines of trailing context")
	flag.IntVar(&opts.BeforeContext, "B", 0, "Print NUM lines of leading context")
	
	var contextLines int
	flag.IntVar(&contextLines, "C", 0, "Print NUM lines of surrounding context")
	flag.BoolVar(&opts.Debug, "debug", false, "Show detailed pipeline logs")

	flag.Parse()

	if contextLines > 0 {
		if opts.AfterContext == 0 { opts.AfterContext = contextLines }
		if opts.BeforeContext == 0 { opts.BeforeContext = contextLines }
	}

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: restgrep [OPTION]... PATTERN [PATH...]")
		os.Exit(1)
	}

	query := args[0]
	opts.Query = query
	opts.Paths = args[1:]
	opts.MergeBaseBranch = cfg.MergeBaseBranch

	// 3. Instantiate Frontends and Resolvers
	var eFrontends []engine.EngineFrontend
	for _, bCfg := range cfg.Backends {
		limit := bCfg.Limit
		if limit <= 0 { limit = 100 }

		var f engine.EngineFrontend
		f.Limit = limit

		// Frontend type
		switch bCfg.Type {
		case "azure":
			f.Frontend = azure.New(bCfg.Organization, bCfg.Project)
		case "github":
			githubBackend := github.New(bCfg.Repo)
			githubBackend.Executor = &github.RealExecutor{}
			f.Frontend = githubBackend
		case "github-api":
			githubAPIBackend := githubapi.New(bCfg.Repo)
			githubAPIBackend.Executor = &githubapi.RealExecutor{}
			f.Frontend = githubAPIBackend
		case "local-diff-add":
			f.Frontend = localdiff.New(bCfg.MergeBaseBranch)
		default:
			fmt.Fprintf(os.Stderr, "Unknown frontend type: %s\n", bCfg.Type)
			continue
		}

		// Resolver mode
		mode := bCfg.BackendMode
		if mode == "" {
			mode = cfg.BackendMode
		}
		
		// Map branch if specific for this backend
		resolverOpts := opts
		if bCfg.MergeBaseBranch != "" {
			resolverOpts.MergeBaseBranch = bCfg.MergeBaseBranch
		}

		if mode == "" {
			mode = string(models.ModeLocal)
		}

		switch models.ResolverMode(mode) {
		case models.ModeNaive:
			f.Resolver = &resolver.NaiveResolver{}
		case models.ModeLocal:
			f.Resolver = &resolver.LocalResolver{}
		default:
			f.Resolver = &resolver.LocalResolver{}
		}

		eFrontends = append(eFrontends, f)
	}

	eng := engine.New(eFrontends, os.Stdout, os.Stderr, cfg.ExecutionMode)
	if err := eng.Run(context.Background(), query, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
		os.Exit(1)
	}
}
