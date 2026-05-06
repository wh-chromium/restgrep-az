package azure

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wh-chromium/restgrep-az/internal/backend"
	"github.com/wh-chromium/restgrep-az/internal/engine"
)

func TestAzureSearch(t *testing.T) {
	mockResponse := `{
  "count": 1,
  "results": [
    {
      "fileName": "CodeSearchController.cs",
      "path": "/CodeSearchController.cs",
      "matches": {
        "content": [
          {
            "charOffset": 1187,
            "length": 20
          },
          {
            "charOffset": 1395,
            "length": 20
          },
          {
            "charOffset": 1686,
            "length": 20
          }
        ],
        "fileName": [
          {
            "charOffset": 0,
            "length": -1
          }
        ]
      },
      "collection": {
        "name": "DefaultCollection"
      },
      "project": {
        "name": "MyFirstProject",
        "id": "00000000-0000-0000-0000-000000000000"
      },
      "repository": {
        "name": "MyFirstProject",
        "id": "c1548045-29f6-4354-8114-55ef058be1a3",
        "type": "git"
      },
      "versions": [
        {
          "branchName": "master",
          "changeId": "47e1cc8877baea4b7bb33af803d6cc697914f88b"
        }
      ],
      "contentId": "004898f1ad91c9c2a0f492f2d1174468bc3c84ef"
    }
  ],
  "infoCode": 0
}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockResponse))
	}))
	defer ts.Close()

	b := New("fabrikam", "MyFirstProject")
	b.BaseURL = ts.URL
	b.TokenFetcher = func(ctx context.Context) (string, error) {
		return "test-token", nil
	}

	tests := []struct {
		name     string
		query    string
		opts     backend.SearchOptions
		expected string
	}{
		{
			name:  "Exact match, no flags",
			query: "CodeSearchController",
			opts:  backend.SearchOptions{},
			expected: `/CodeSearchController.cs:[Match at char offset 1187, length 20] (local file not found)
/CodeSearchController.cs:[Match at char offset 1395, length 20] (local file not found)
/CodeSearchController.cs:[Match at char offset 1686, length 20] (local file not found)
`,
		},
		{
			name:  "Partial string match (-i ignore case)",
			query: "codesearchcontroller",
			opts:  backend.SearchOptions{IgnoreCase: true},
			expected: `/CodeSearchController.cs:[Match at char offset 1187, length 20] (local file not found)
/CodeSearchController.cs:[Match at char offset 1395, length 20] (local file not found)
/CodeSearchController.cs:[Match at char offset 1686, length 20] (local file not found)
`,
		},
		{
			name:  "Line numbers flag (-n)",
			query: "CodeSearchController",
			opts:  backend.SearchOptions{LineNumber: true},
			expected: `/CodeSearchController.cs:1:[Match at char offset 1187, length 20] (local file not found)
/CodeSearchController.cs:1:[Match at char offset 1395, length 20] (local file not found)
/CodeSearchController.cs:1:[Match at char offset 1686, length 20] (local file not found)
`,
		},
		{
			name:  "Count flag (-c)",
			query: "CodeSearchController",
			opts:  backend.SearchOptions{Count: true},
			expected: `/CodeSearchController.cs:3
`,
		},
		{
			name:  "Files with matches flag (-l)",
			query: "CodeSearchController",
			opts:  backend.SearchOptions{FilesWithMatches: true},
			expected: `/CodeSearchController.cs
`,
		},
		{
			name:  "Multiple flags: -i -n",
			query: "codesearchcontroller",
			opts:  backend.SearchOptions{IgnoreCase: true, LineNumber: true},
			expected: `/CodeSearchController.cs:1:[Match at char offset 1187, length 20] (local file not found)
/CodeSearchController.cs:1:[Match at char offset 1395, length 20] (local file not found)
/CodeSearchController.cs:1:[Match at char offset 1686, length 20] (local file not found)
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			eb := engine.EngineBackend{
				Backend: b,
				Limit:   tt.opts.Limit,
			}
			eng := engine.New([]engine.EngineBackend{eb}, &buf)

			err := eng.Run(context.Background(), tt.query, tt.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			actual := buf.String()
			
			// Separate results from status lines
			lines := strings.Split(strings.TrimSpace(actual), "\n")
			var resultLines []string
			for _, line := range lines {
				if !(strings.HasPrefix(line, "[") && strings.Contains(line, "Showing")) {
					resultLines = append(resultLines, line)
				}
			}
			actualResults := strings.Join(resultLines, "\n")
			if len(resultLines) > 0 {
				actualResults += "\n"
			}

			if tt.opts.Count {
				actualLines := resultLines
				expectedLines := strings.Split(strings.TrimSpace(tt.expected), "\n")
				
				if len(actualLines) != len(expectedLines) {
					t.Errorf("expected %d lines, got %d. Output:\n%s", len(expectedLines), len(actualLines), actualResults)
				}
				
				for _, el := range expectedLines {
					found := false
					for _, al := range actualLines {
						if el == al {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected to find line %q in output:\n%s", el, actualResults)
					}
				}
			} else {
				if actualResults != tt.expected {
					t.Errorf("expected:\n%s\n\ngot:\n%s", tt.expected, actualResults)
				}
			}
		})
	}
}
