package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/wh-chromium/restgrep-az/internal/models"
	"github.com/wh-chromium/restgrep-az/internal/resolver"
)

// mockFrontend implements the Frontend interface for testing.
type mockFrontend struct {
	name    string
	results []models.IntermediateResult
}

func (m *mockFrontend) Name() string { return m.name }
func (m *mockFrontend) Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.IntermediateResult, error) {
	return m.results, nil
}

func TestEngine3x3Matrix(t *testing.T) {
	// Setup test data
	content := "line 1\nline 2\nMATCH\n"
	tmpFile, _ := os.CreateTemp("", "3x3_test_*.txt")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte(content))
	tmpFile.Close()

	sha := resolver.GetGitBlobSHA1([]byte(content))
	fileName := tmpFile.Name()

	// Frontends
	frontends := []string{"azure", "github", "githubapi"}
	// Resolvers
	resolvers := []struct {
		name string
		res  resolver.Resolver
	}{
		{"naive", &resolver.NaiveResolver{}},
		{"local", &resolver.LocalNoDiffResolver{}},
		{"git-diff", &resolver.LocalWithDiffResolver{}},
	}

	for _, fName := range frontends {
		for _, resInfo := range resolvers {
			t.Run(fmt.Sprintf("%s_%s", fName, resInfo.name), func(t *testing.T) {
				var buf bytes.Buffer
				
				// Create mock results
				ir := models.IntermediateResult{
					File:        fileName,
					RemoteSHA:   sha,
					CharOffset:  14, // "MATCH"
					RawFragment: "[Remote Fragment]",
					LineNumber:  1,
				}
				
				mf := &mockFrontend{name: fName, results: []models.IntermediateResult{ir}}
				ef := EngineFrontend{
					Frontend: mf,
					Resolver: resInfo.res,
					Limit:    10,
				}

				eng := New([]EngineFrontend{ef}, &buf, &buf, "parallel")
				err := eng.Run(context.Background(), "MATCH", models.SearchOptions{})
				if err != nil {
					t.Fatalf("Run failed: %v", err)
				}

				output := buf.String()
				
				// Expectations
				switch resInfo.name {
				case "naive":
					if !strings.Contains(output, "[Remote Fragment]") {
						t.Errorf("Naive resolver failed to use raw fragment")
					}
				case "local", "git-diff":
					if !strings.Contains(output, "MATCH") || strings.Contains(output, "[Remote Fragment]") {
						t.Errorf("%s resolver failed to resolve local content. Output:\n%s", resInfo.name, output)
					}
				}
			})
		}
	}
}

func TestEngineExecutionStrategies(t *testing.T) {
	// Re-implement simplified strategy tests
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
			t.Errorf("Parallel failed to aggregate")
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
			t.Errorf("Sequential failed to stop after first success")
		}
	})
}
