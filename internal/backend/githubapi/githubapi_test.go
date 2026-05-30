package githubapi

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

func TestGithubAPIFrontend_MapsToIntermediateResult(t *testing.T) {
	// Simulated Raw Output from 'gh api /search/code' with text-match header
	mockRawGHAPIResponse := `{
		"total_count": 2,
		"items": [
			{
				"path": "test.go",
				"sha": "github-api-blob-sha-1",
				"text_matches": [
					{
						"fragment": "func match() {}\nfunc noMatch() {}"
					}
				]
			},
			{
				"path": "nomatch.go",
				"sha": "github-api-blob-sha-2",
				"text_matches": []
			}
		]
	}`

	b := New("repo")
	b.Executor = &mockExecutor{stdout: []byte(mockRawGHAPIResponse)}

	results, err := b.Search(context.Background(), "match", models.SearchOptions{})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Expected Translation to Intermediate Format
	expectedIntermediates := []models.IntermediateResult{
		{
			File:        "test.go",
			RemoteSHA:   "github-api-blob-sha-1",
			CharOffset:  -1, 
			Length:      0,
			RawFragment: "func match() {}",
			LineNumber:  1,
		},
		{
			File:        "nomatch.go",
			RemoteSHA:   "github-api-blob-sha-2",
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
