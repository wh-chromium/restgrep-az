package engine

import (
	"bytes"
	"context"
	"errors"
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
		eng := New(backends, &buf, "parallel")

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
		eng := New(backends, &buf, "sequential")

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
	})
}
