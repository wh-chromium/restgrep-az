package engine

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/wh-chromium/restgrep-az/internal/backend"
)

type mockStrategyBackend struct {
	name    string
	results []backend.SearchResult
	fail    bool
}

func (m *mockStrategyBackend) Name() string { return m.name }
func (m *mockStrategyBackend) Search(ctx context.Context, query string, opts backend.SearchOptions) ([]backend.SearchResult, error) {
	if m.fail {
		return nil, errors.New("forced failure")
	}
	return m.results, nil
}

func TestEngineExecutionStrategies(t *testing.T) {
	b1 := &mockStrategyBackend{name: "b1", results: []backend.SearchResult{{File: "f1", Content: "c1"}}}
	b2 := &mockStrategyBackend{name: "b2", results: []backend.SearchResult{{File: "f2", Content: "c2"}}}
	bFail := &mockStrategyBackend{name: "fail", fail: true}

	t.Run("Parallel Mode: Aggregates all successes, ignores failures", func(t *testing.T) {
		var buf bytes.Buffer
		backends := []EngineBackend{
			{Backend: b1, Limit: 10},
			{Backend: bFail, Limit: 10},
			{Backend: b2, Limit: 10},
		}
		eng := New(backends, &buf, &buf, "parallel")

		err := eng.Run(context.Background(), "query", backend.SearchOptions{})
		if err != nil {
			t.Fatalf("Parallel mode should not return error on partial failure: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "f1:c1") || !strings.Contains(output, "f2:c2") {
			t.Errorf("Parallel mode failed to aggregate results: %s", output)
		}
		if !strings.Contains(output, "[b1]") || !strings.Contains(output, "[b2]") {
			t.Errorf("Parallel mode status lines missing: %s", output)
		}
		if !strings.Contains(output, "[fail] Error: forced failure") {
			t.Errorf("Parallel mode did not report error: %s", output)
		}
	})

	t.Run("Sequential Mode: Stops after first success", func(t *testing.T) {
		var buf bytes.Buffer
		// Order: Fail -> Success (b1) -> Success (b2)
		// Should only show results from b1.
		backends := []EngineBackend{
			{Backend: bFail, Limit: 10},
			{Backend: b1, Limit: 10},
			{Backend: b2, Limit: 10},
		}
		eng := New(backends, &buf, &buf, "sequential")

		err := eng.Run(context.Background(), "query", backend.SearchOptions{})
		if err != nil {
			t.Fatalf("Sequential fallback failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "f1:c1") {
			t.Errorf("Sequential mode missed first success: %s", output)
		}
		if strings.Contains(output, "f2:c2") {
			t.Errorf("Sequential mode failed to stop after first success: %s", output)
		}
		if !strings.Contains(output, "[b1]") || strings.Contains(output, "[b2]") {
			t.Errorf("Sequential mode status lines incorrect: %s", output)
		}
		if !strings.Contains(output, "[fail] Error: forced failure") {
			t.Errorf("Sequential mode did not report error: %s", output)
		}
	})

	t.Run("Parallel Mode: Grouping by Provider", func(t *testing.T) {
		var buf bytes.Buffer
		// Both backends find matches in the same file "shared.txt"
		b1 := &mockStrategyBackend{name: "azure", results: []backend.SearchResult{{File: "shared.txt", ContentId: "sha", CharOffset: 0, Content: "azure-hit"}}}
		b2 := &mockStrategyBackend{name: "github", results: []backend.SearchResult{{File: "shared.txt", ContentId: "sha", CharOffset: 0, Content: "github-hit"}}}
		
		backends := []EngineBackend{
			{Backend: b1, Limit: 10},
			{Backend: b2, Limit: 10},
		}
		eng := New(backends, &buf, &buf, "parallel")

		// Create the local file so enrichment happens
		content := "local content\n"
		os.WriteFile("shared.txt", []byte(content), 0644)
		defer os.Remove("shared.txt")

		err := eng.Run(context.Background(), "query", backend.SearchOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		output := buf.String()
		// Verify azure results come before github results because of backendIndex sort
		azureIdx := strings.Index(output, "[azure]")
		githubIdx := strings.Index(output, "[github]")
		
		if azureIdx > githubIdx {
			t.Errorf("Results not grouped by provider correctly. Azure should come before GitHub. Output:\n%s", output)
		}
	})
}
