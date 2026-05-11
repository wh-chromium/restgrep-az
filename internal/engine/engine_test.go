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
	"github.com/wh-chromium/restgrep-az/internal/frontend/localdiff"
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

func TestEngineMatrix(t *testing.T) {
	// 1. Setup local file for resolution
	content := "header\nTARGET_STRING\nfooter\n"
	tmpFile, _ := os.CreateTemp("", "engine_matrix_*.txt")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte(content))
	tmpFile.Close()
	fileName := tmpFile.Name()

	// Frontends (Simulated)
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
				ir := models.IntermediateResult{
					File:        fileName,
					RemoteSHA:   "mismatch-sha",
					CharOffset:  7, // TARGET_STRING
					RawFragment: "[Remote Fragment]",
					LineNumber:  1,
				}
				mf := &mockFrontend{name: fName, results: []models.IntermediateResult{ir}}
				ef := EngineFrontend{Frontend: mf, Resolver: rInfo.res, Limit: 10}

				eng := New([]EngineFrontend{ef}, &buf, &buf, "parallel")
				opts := models.SearchOptions{Query: "TARGET_STRING"}
				eng.Run(context.Background(), "TARGET_STRING", opts)

				output := buf.String()
				if rInfo.name == "naive" {
					if !strings.Contains(output, "[Remote Fragment]") {
						t.Errorf("Naive failed to use raw fragment")
					}
				} else {
					if !strings.Contains(output, "TARGET_STRING") || !strings.Contains(output, "(relaxed match)") {
						t.Errorf("Local relaxed failed to resolve. Output:\n%s", output)
					}
				}
			})
		}
	}
}

func TestLocalDiffAddFrontend(t *testing.T) {
	// 1. Setup a temporary git repository
	dir, err := os.MkdirTemp("", "restgrep_localdiff_test")
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
	filename := "test.txt"
	os.WriteFile(filename, []byte("line 1\n"), 0644)
	w, _ := repo.Worktree()
	w.Add(filename)
	commit, _ := w.Commit("base", &git.CommitOptions{
		Author: &object.Signature{Name: "T", Email: "t@e.com", When: time.Now()},
	})

	// origin/main
	ref := plumbing.NewHashReference("refs/remotes/origin/main", commit)
	repo.Storer.SetReference(ref)

	// 3. Add new lines in current branch (HEAD)
	os.WriteFile(filename, []byte("line 1\nNEW_FEATURE_PATTERN\n"), 0644)
	w.Add(filename)
	w.Commit("added feature", &git.CommitOptions{
		Author: &object.Signature{Name: "T", Email: "t@e.com", When: time.Now()},
	})

	// 4. Test the frontend
	ldf := localdiff.New("origin/main")
	opts := models.SearchOptions{MergeBaseBranch: "origin/main"}
	results, err := ldf.Search(context.Background(), "NEW_FEATURE", opts)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Errorf("Expected to find NEW_FEATURE in diff")
	}
	if results[0].RawFragment != "NEW_FEATURE_PATTERN" {
		t.Errorf("Expected fragment NEW_FEATURE_PATTERN, got %s", results[0].RawFragment)
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
