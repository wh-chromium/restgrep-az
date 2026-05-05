package backend

import (
	"context"
	"strings"
	"testing"
)

// ContractTestCase defines a standard test case for any backend implementation.
type ContractTestCase struct {
	Name             string
	Query            string
	Options          SearchOptions
	ExpectedFiles    []string // Files that must be present in results
	ForbiddenStrings []string // Content that must NOT be present (useful for -w or case tests)
}

// VerifyBackendContract provides a rigorous verification suite for Backend implementations.
// This ensures that new backends (including proprietary ones) adhere to the restgrep standard.
func VerifyBackendContract(t *testing.T, b Backend, cases []ContractTestCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			results, err := b.Search(context.Background(), tc.Query, tc.Options)
			if err != nil {
				t.Fatalf("Backend %s failed Search: %v", b.Name(), err)
			}

			// Verify expected files are present
			for _, expFile := range tc.ExpectedFiles {
				found := false
				for _, res := range results {
					if res.File == expFile {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected file %q not found in results", expFile)
				}
			}

			// Verify forbidden strings are NOT present
			for _, forbidden := range tc.ForbiddenStrings {
				for _, res := range results {
					content := res.Content
					if tc.Options.IgnoreCase {
						content = strings.ToLower(content)
						forbidden = strings.ToLower(forbidden)
					}
					if strings.Contains(content, forbidden) {
						t.Errorf("Forbidden string %q found in result content: %q", forbidden, res.Content)
					}
				}
			}
		})
	}
}
