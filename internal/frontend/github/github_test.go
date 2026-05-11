package github

import (
	"context"
	"testing"

	"github.com/wh-chromium/restgrep-az/internal/models"
)

type mockExecutor struct {
	stdout []byte
	stderr []byte
	err    error
}

func (m *mockExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	return m.stdout, m.stderr, m.err
}

func TestGithubFrontend_Search(t *testing.T) {
	mockResponse := `[
		{
			"path": "test.go",
			"sha": "dummy-sha",
			"textMatches": [
				{
					"fragment": "func match() {}\nfunc noMatch() {}"
				}
			]
		},
		{
			"path": "nomatch.go",
			"sha": "dummy-sha-2",
			"textMatches": []
		}
	]`

	b := New("repo")
	b.Executor = &mockExecutor{stdout: []byte(mockResponse)}

	t.Run("Substring Match", func(t *testing.T) {
		results, err := b.Search(context.Background(), "match", models.SearchOptions{})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 2 { // One line matches 'match', plus the noMatch file
			t.Fatalf("expected 2 results, got %d", len(results))
		}

		if results[0].File != "test.go" || results[0].RawFragment != "func match() {}" {
			t.Errorf("Unexpected first result: %+v", results[0])
		}
		if results[1].File != "nomatch.go" || results[1].RawFragment != "[File match]" {
			t.Errorf("Unexpected second result: %+v", results[1])
		}
	})

	t.Run("Word Match", func(t *testing.T) {
		// "match" will match "func match() {}" but not "func noMatch() {}"
		results, err := b.Search(context.Background(), "match", models.SearchOptions{WordRegexp: true})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 2 { // "match" line and the nomatch file fallback
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].RawFragment != "func match() {}" {
			t.Errorf("Unexpected first result: %+v", results[0])
		}
	})

	t.Run("Case Insensitive", func(t *testing.T) {
		results, err := b.Search(context.Background(), "MATCH", models.SearchOptions{IgnoreCase: true})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Should match both "match" and "noMatch" fragments from the first file, plus the second file
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}
	})
}
