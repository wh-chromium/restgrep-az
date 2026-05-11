package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/wh-chromium/restgrep-az/internal/models"
	"github.com/wh-chromium/restgrep-az/internal/resolver"
)

type mockFrontend struct {
	name    string
	results []models.IntermediateResult
}

func (m *mockFrontend) Name() string { return m.name }
func (m *mockFrontend) Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.IntermediateResult, error) {
	return m.results, nil
}

func TestEngineMergeBaseDiff(t *testing.T) {
	// 1. Setup a temporary git repository
	dir, err := os.MkdirTemp("", "restgrep_mergebase_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Commit a file to 'main'
	filename := "drift.txt"
	content := "line A\nTARGET\nline C\n"
	os.WriteFile(filename, []byte(content), 0644)

	w, _ := repo.Worktree()
	w.Add(filename)
	commit, _ := w.Commit("initial on main", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "t@e.com", When: time.Now()},
	})

	// Create a branch reference 'origin/main' pointing to this commit
	refName := plumbing.ReferenceName("refs/remotes/origin/main")
	ref := plumbing.NewHashReference(refName, commit)
	repo.Storer.SetReference(ref)

	// TARGET is at offset 7 in main
	remoteOffset := 7

	// 3. Diverge locally: Add lines at top
	newContent := "TOP 1\nTOP 2\nline A\nTARGET\nline C\n"
	os.WriteFile(filename, []byte(newContent), 0644)

	mockB := &mockFrontend{
		name: "git-mergebase",
		results: []models.IntermediateResult{
			{
				File:       filename,
				RemoteSHA:  "indexed-sha", // Doesn't matter for this mode as much as branch state
				CharOffset: remoteOffset,
				RawFragment: "TARGET",
				LineNumber: 1,
			},
		},
	}

	t.Run("Merge-Base Adjustment", func(t *testing.T) {
		var buf bytes.Buffer
		ef := EngineFrontend{
			Frontend: mockB,
			Resolver: &resolver.DiffMergeBaseResolver{},
			Limit:    10,
		}
		eng := New([]EngineFrontend{ef}, &buf, &buf, "parallel")

		opts := models.SearchOptions{
			MergeBaseBranch: "origin/main",
			LineNumber:      true,
		}
		if err := eng.Run(context.Background(), "TARGET", opts); err != nil {
			t.Fatal(err)
		}

		output := buf.String()
		// New line number should be 4
		if !strings.Contains(output, "drift.txt:4:TARGET") {
			t.Errorf("expected adjusted output to contain correct line 4, got:\n%s", output)
		}
		if !strings.Contains(output, "(adjusted from origin/main)") {
			t.Errorf("expected adjustment hint")
		}
	})
}

func TestEngine3x2Matrix(t *testing.T) {
	// 1. Setup local file
	content := "header\nMATCH_HERE\nfooter\n"
	tmpFile, _ := os.CreateTemp("", "engine_relaxed_*.txt")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte(content))
	tmpFile.Close()
	fileName := tmpFile.Name()

	// Frontends
	frontends := []string{"azure", "github", "githubapi"}
	// Resolvers
	resolvers := []struct {
		name string
		res  resolver.Resolver
	}{
		{"naive", &resolver.NaiveResolver{}},
		{"local", &resolver.LocalResolver{}},
	}

	for _, fName := range frontends {
		for _, rInfo := range resolvers {
			t.Run(fmt.Sprintf("%s_%s", fName, rInfo.name), func(t *testing.T) {
				var buf bytes.Buffer
				
				// Simulate a "stale" remote result (offset is wrong, SHA is wrong)
				ir := models.IntermediateResult{
					File:        fileName,
					RemoteSHA:   "stale-sha",
					CharOffset:  999, // Way off
					RawFragment: "[Remote Stub]",
					LineNumber:  1,
				}

				mf := &mockFrontend{name: fName, results: []models.IntermediateResult{ir}}
				ef := EngineFrontend{
					Frontend: mf,
					Resolver: rInfo.res,
					Limit:    10,
				}

				eng := New([]EngineFrontend{ef}, &buf, &buf, "parallel")
				opts := models.SearchOptions{Query: "MATCH_HERE"}
				eng.Run(context.Background(), "MATCH_HERE", opts)

				output := buf.String()
				
				if rInfo.name == "naive" {
					if !strings.Contains(output, "[Remote Stub]") {
						t.Errorf("Naive failed to use raw fragment")
					}
				} else {
					// Local (relaxed) should find the string even if offset/SHA were wrong
					if !strings.Contains(output, "MATCH_HERE") || strings.Contains(output, "[Remote Stub]") {
						t.Errorf("Local relaxed resolver failed to find pattern. Output:\n%s", output)
					}
					if !strings.Contains(output, "(relaxed match)") {
						t.Errorf("Expected relaxed match warning")
					}
				}
			})
		}
	}
}

func TestEngineExecutionStrategies(t *testing.T) {
	b1 := &mockFrontend{name: "b1", results: []models.IntermediateResult{{File: "f1", RawFragment: "c1"}}}
	b2 := &mockFrontend{name: "b2", results: []models.IntermediateResult{{File: "f2", RawFragment: "c2"}}}

	t.Run("Parallel Mode", func(t *testing.T) {
		var buf bytes.Buffer
		backends := []EngineFrontend{
			{Frontend: b1, Resolver: &resolver.NaiveResolver{}, Limit: 10},
			{Frontend: b2, Resolver: &resolver.NaiveResolver{}, Limit: 10},
		}
		eng := New(backends, &buf, &buf, "parallel")
		eng.Run(context.Background(), "q", models.SearchOptions{})
		if !strings.Contains(buf.String(), "f1") || !strings.Contains(buf.String(), "f2") {
			t.Errorf("Parallel failed to aggregate successes")
		}
	})

	t.Run("Sequential Mode", func(t *testing.T) {
		var buf bytes.Buffer
		backends := []EngineFrontend{
			{Frontend: b1, Resolver: &resolver.NaiveResolver{}, Limit: 10},
			{Frontend: b2, Resolver: &resolver.NaiveResolver{}, Limit: 10},
		}
		eng := New(backends, &buf, &buf, "sequential")
		eng.Run(context.Background(), "q", models.SearchOptions{})
		if !strings.Contains(buf.String(), "f1") || strings.Contains(buf.String(), "f2") {
			t.Errorf("Sequential failed to stop after first success. Output: %s", buf.String())
		}
	})
}
