package github

import (
	"context"
	"reflect"
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

func TestGithubFrontend_MapsToIntermediateResult(t *testing.T) {
	// Simulated Raw Output from 'gh search code' CLI
	mockRawGHCLIResponse := `[
		{
			"path": "test.go",
			"sha": "github-blob-sha-1",
			"textMatches": [
				{
					"fragment": "func match() {}\nfunc noMatch() {}"
				}
			]
		},
		{
			"path": "nomatch.go",
			"sha": "github-blob-sha-2",
			"textMatches": []
		}
	]`

	b := New("repo")
	b.Executor = &mockExecutor{stdout: []byte(mockRawGHCLIResponse)}

	// We search for "match" (which is in the first file's fragment)
	results, err := b.Search(context.Background(), "match", models.SearchOptions{})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Expected Translation to Intermediate Format
	expectedIntermediates := []models.IntermediateResult{
		{
			File:        "test.go",
			RemoteSHA:   "github-blob-sha-1",
			CharOffset:  -1, // CLI doesn't provide precise offsets
			Length:      0,
			RawFragment: "func match() {}", // Notice it was post-filtered from the multi-line fragment
			LineNumber:  1,
		},
		{
			File:        "nomatch.go",
			RemoteSHA:   "github-blob-sha-2",
			CharOffset:  -1,
			Length:      0,
			RawFragment: "[File match]",
			LineNumber:  1,
		},
	}

	if len(results) != len(expectedIntermediates) {
		t.Fatalf("Expected %d intermediate results, got %d", len(expectedIntermediates), len(results))
	}

	for i, expected := range expectedIntermediates {
		if !reflect.DeepEqual(results[i], expected) {
			t.Errorf("Result %d mapping failed.\nExpected: %+v\nGot:      %+v", i, expected, results[i])
		}
	}
}
