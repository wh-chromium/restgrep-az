package azure

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/wh-chromium/restgrep-az/internal/models"
)

type mockTransport struct {
	responseBody string
	statusCode   int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(m.responseBody)),
		Header:     make(http.Header),
	}, nil
}

func TestAzureFrontend_Search(t *testing.T) {
	mockResponse := `{
		"count": 1,
		"results": [
			{
				"fileName": "Controller.cs",
				"path": "/src/Controller.cs",
				"contentId": "dummy-sha",
				"matches": {
					"content": [
						{ "charOffset": 10, "length": 5 }
					]
				}
			},
			{
				"fileName": "NoMatch.cs",
				"path": "/src/NoMatch.cs",
				"contentId": "dummy-sha-2",
				"matches": {
					"content": []
				}
			}
		]
	}`

	b := New("org", "proj")
	b.HTTPClient = &http.Client{Transport: &mockTransport{responseBody: mockResponse, statusCode: 200}}
	b.TokenFetcher = func(ctx context.Context) (string, error) { return "token", nil }

	results, err := b.Search(context.Background(), "query", models.SearchOptions{})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// 1. Check match with content
	if results[0].File != "/src/Controller.cs" {
		t.Errorf("got %s", results[0].File)
	}
	if results[0].RemoteSHA != "dummy-sha" {
		t.Errorf("got %s", results[0].RemoteSHA)
	}
	if results[0].CharOffset != 10 {
		t.Errorf("got %d", results[0].CharOffset)
	}
	if results[0].Length != 5 {
		t.Errorf("got %d", results[0].Length)
	}
	if results[0].RawFragment != "[Match at char offset 10, length 5]" {
		t.Errorf("got %s", results[0].RawFragment)
	}

	// 2. Check file match (no content matches)
	if results[1].File != "/src/NoMatch.cs" {
		t.Errorf("got %s", results[1].File)
	}
	if results[1].CharOffset != -1 {
		t.Errorf("got %d", results[1].CharOffset)
	}
	if results[1].RawFragment != "[File match]" {
		t.Errorf("got %s", results[1].RawFragment)
	}
}
