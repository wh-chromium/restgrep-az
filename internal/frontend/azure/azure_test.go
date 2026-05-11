package azure

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"reflect"
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

func TestAzureFrontend_MapsToIntermediateResult(t *testing.T) {
	// Simulated Raw Output from Azure DevOps REST API
	mockRawAzureAPIResponse := `{
		"count": 2,
		"results": [
			{
				"fileName": "Controller.cs",
				"path": "/src/Controller.cs",
				"contentId": "azure-blob-sha-1",
				"matches": {
					"content": [
						{ "charOffset": 10, "length": 5 }
					]
				}
			},
			{
				"fileName": "NoMatch.cs",
				"path": "/src/NoMatch.cs",
				"contentId": "azure-blob-sha-2",
				"matches": {
					"content": []
				}
			}
		]
	}`

	b := New("org", "proj")
	b.HTTPClient = &http.Client{Transport: &mockTransport{responseBody: mockRawAzureAPIResponse, statusCode: 200}}
	b.TokenFetcher = func(ctx context.Context) (string, error) { return "token", nil }

	results, err := b.Search(context.Background(), "query", models.SearchOptions{})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Expected Translation to Intermediate Format
	expectedIntermediates := []models.IntermediateResult{
		{
			File:        "/src/Controller.cs",
			RemoteSHA:   "azure-blob-sha-1",
			CharOffset:  10,
			Length:      5,
			RawFragment: "[Match at char offset 10, length 5]",
			LineNumber:  1,
		},
		{
			File:        "/src/NoMatch.cs",
			RemoteSHA:   "azure-blob-sha-2",
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
