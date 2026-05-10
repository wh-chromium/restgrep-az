// Copyright 2026 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/wh-chromium/restgrep-az/internal/backend"
)

// MockChromiumBackend simulates a remote code search API for a large codebase like Chromium.
type MockChromiumBackend struct {
	files map[string][]string
}

func NewMockChromiumBackend() *MockChromiumBackend {
	return &MockChromiumBackend{
		files: map[string][]string{
			"src/content/browser/web_contents/web_contents_impl.cc": {
				"#include \"content/browser/web_contents/web_contents_impl.h\"",
				"#include \"chrome/browser/ui/omnibox/omnibox_edit_model.h\"",
				"",
				"namespace content {",
				"",
				"WebContentsImpl::WebContentsImpl(BrowserContext* browser_context) {",
				"  // Initialize WebContents and Omnibox references",
				"  InitRenderViewHost();",
				"}",
				"",
				"void WebContentsImpl::InitRenderViewHost() {",
				"  // RenderViewHost initialization logic",
				"}",
				"",
				"void WebContentsImpl::NavigateToURL(const GURL& url) {",
				"  // Navigation logic here",
				"}",
				"",
				"}  // namespace content",
			},
			"src/base/strings/string_util.cc": {
				"#include \"base/strings/string_util.h\"",
				"",
				"namespace base {",
				"",
				"bool StartsWith(StringPiece text, StringPiece prefix, CompareCase case_sensitivity) {",
				"  // StartsWith prefix logic",
				"  return true;",
				"}",
				"",
				"bool EndsWith(StringPiece text, StringPiece suffix, CompareCase case_sensitivity) {",
				"  // EndsWith suffix logic",
				"  return true;",
				"}",
				"",
				"}  // namespace base",
			},
			"src/net/http/http_request_info.h": {
				"#ifndef NET_HTTP_HTTP_REQUEST_INFO_H_",
				"#define NET_HTTP_HTTP_REQUEST_INFO_H_",
				"",
				"namespace net {",
				"",
				"struct HttpRequestInfo {",
				"  HttpRequestInfo();",
				"  ~HttpRequestInfo();",
				"",
				"  std::string url;",
				"  std::string method;",
				"};",
				"",
				"}  // namespace net",
				"",
				"#endif  // NET_HTTP_HTTP_REQUEST_INFO_H_",
			},
		},
	}
}

func (m *MockChromiumBackend) Name() string {
	return "mock-chromium"
}

func (m *MockChromiumBackend) Search(ctx context.Context, query string, opts backend.SearchOptions) ([]backend.SearchResult, error) {
	var results []backend.SearchResult
	q := query
	if opts.IgnoreCase {
		q = strings.ToLower(q)
	}

	var wordRe *regexp.Regexp
	if opts.WordRegexp {
		pattern := `\b` + regexp.QuoteMeta(q) + `\b`
		if opts.IgnoreCase {
			pattern = `(?i)\b` + regexp.QuoteMeta(q) + `\b`
		}
		wordRe = regexp.MustCompile(pattern)
	}

	// We sort the map keys manually if we needed deterministic output across different files,
	// but Engine groups or prints them as they come. For reliable testing, 
	// we will iterate in a fixed order.
	fileNames := []string{
		"src/base/strings/string_util.cc",
		"src/content/browser/web_contents/web_contents_impl.cc",
		"src/net/http/http_request_info.h",
	}

	for _, file := range fileNames {
		// Path filtering
		if len(opts.Paths) > 0 {
			matchPath := false
			for _, p := range opts.Paths {
				if strings.HasPrefix(file, p) {
					matchPath = true
					break
				}
			}
			if !matchPath {
				continue
			}
		}

		lines := m.files[file]
		for i, line := range lines {
			matched := false
			if opts.WordRegexp {
				matched = wordRe.MatchString(line)
			} else {
				lineToCheck := line
				if opts.IgnoreCase {
					lineToCheck = strings.ToLower(lineToCheck)
				}
				matched = strings.Contains(lineToCheck, q)
			}

			if matched {
				results = append(results, backend.SearchResult{
					File:    file,
					Line:    i + 1,
					Content: line,
				})
			}
		}
	}

	return results, nil
}

func TestEngineLocalResolutionWithCache(t *testing.T) {
	// 1. Create a temporary file
	content := "line one\nline two\nline three\n"
	tmpFile, err := os.CreateTemp("", "restgrep_test_*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", tmpFile)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	sha := getGitBlobSHA1([]byte(content))
	fileName := tmpFile.Name()

	// 2. Create a mock backend that returns 2 matches for this file
	mockB := &mockGenericBackend{
		name: "test-cache",
		results: []backend.SearchResult{
			{
				File:       fileName,
				ContentId:  sha,
				CharOffset: 0, // "line one"
				Content:    "[Stub 1]",
			},
			{
				File:       fileName,
				ContentId:  sha,
				CharOffset: 9, // "line two"
				Content:    "[Stub 2]",
			},
		},
	}

	var buf bytes.Buffer
	eb := EngineBackend{Backend: mockB, Limit: 10}
	eng := New([]EngineBackend{eb}, &buf, &buf, "parallel")

	// 3. Run engine
	if err := eng.Run(context.Background(), "unused", backend.SearchOptions{}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	actual := buf.String()
	expected := fmt.Sprintf("%s:line one\n%s:line two\n[test-cache] Showing 2 results (limit: 10).\n", fileName, fileName)
	
	if actual != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, actual)
	}
}

type mockGenericBackend struct {
	name    string
	results []backend.SearchResult
}

func (m *mockGenericBackend) Name() string { return m.name }
func (m *mockGenericBackend) Search(ctx context.Context, query string, opts backend.SearchOptions) ([]backend.SearchResult, error) {
	return m.results, nil
}


func TestEngineChromiumSimulations(t *testing.T) {
	mockBackend := NewMockChromiumBackend()

	tests := []struct {
		name     string
		query    string
		opts     backend.SearchOptions
		expected string
	}{
		{
			name:  "Exact match, no flags",
			query: "WebContentsImpl::WebContentsImpl",
			opts:  backend.SearchOptions{},
			expected: `src/content/browser/web_contents/web_contents_impl.cc:WebContentsImpl::WebContentsImpl(BrowserContext* browser_context) {
`,
		},
		{
			name:  "Prefix match (StartsWith), no flags",
			query: "StartsWith",
			opts:  backend.SearchOptions{},
			expected: `src/base/strings/string_util.cc:bool StartsWith(StringPiece text, StringPiece prefix, CompareCase case_sensitivity) {
src/base/strings/string_util.cc:  // StartsWith prefix logic
`,
		},
		{
			name:  "Suffix match (suffix logic), no flags",
			query: "suffix logic",
			opts:  backend.SearchOptions{},
			expected: `src/base/strings/string_util.cc:  // EndsWith suffix logic
`,
		},
		{
			name:  "Partial string match (-i ignore case)",
			query: "INITRENDErviewHOST",
			opts:  backend.SearchOptions{IgnoreCase: true},
			expected: `src/content/browser/web_contents/web_contents_impl.cc:  InitRenderViewHost();
src/content/browser/web_contents/web_contents_impl.cc:void WebContentsImpl::InitRenderViewHost() {
`,
		},
		{
			name:  "Line numbers flag (-n)",
			query: "HttpRequestInfo",
			opts:  backend.SearchOptions{LineNumber: true},
			expected: `src/net/http/http_request_info.h:6:struct HttpRequestInfo {
src/net/http/http_request_info.h:7:  HttpRequestInfo();
src/net/http/http_request_info.h:8:  ~HttpRequestInfo();
`,
		},
		{
			name:  "Count flag (-c) case insensitive",
			query: "string",
			opts:  backend.SearchOptions{Count: true, IgnoreCase: true},
			// map iteration in engine.go for -c is non-deterministic, so we will handle that in a custom check if needed,
			// or we can sort it. Since Engine map iteration is non-deterministic, we will sort the output lines in the test verification.
			expected: `src/base/strings/string_util.cc:3
src/net/http/http_request_info.h:2
`,
		},
		{
			name:  "Files with matches flag (-l)",
			query: "namespace",
			opts:  backend.SearchOptions{FilesWithMatches: true},
			// Same, map iteration for duplicate prevention might not change the order since it prints as it encounters them.
			// Since our mock backend returns files in a stable order, -l will print in that stable order.
			expected: `src/base/strings/string_util.cc
src/content/browser/web_contents/web_contents_impl.cc
src/net/http/http_request_info.h
`,
		},
		{
			name:  "Multiple flags: -i -n",
			query: "bool",
			opts:  backend.SearchOptions{IgnoreCase: true, LineNumber: true},
			expected: `src/base/strings/string_util.cc:5:bool StartsWith(StringPiece text, StringPiece prefix, CompareCase case_sensitivity) {
src/base/strings/string_util.cc:10:bool EndsWith(StringPiece text, StringPiece suffix, CompareCase case_sensitivity) {
`,
		},
		{
			name:  "Word match (-w)",
			query: "WebContents",
			opts:  backend.SearchOptions{WordRegexp: true},
			expected: `src/content/browser/web_contents/web_contents_impl.cc:  // Initialize WebContents and Omnibox references
`,
		},
		{
			name:  "Chromium: Omnibox substring",
			query: "Omnibox",
			opts:  backend.SearchOptions{},
			expected: `src/content/browser/web_contents/web_contents_impl.cc:  // Initialize WebContents and Omnibox references
`,
		},
		{
			name:  "Path filtering: src/base",
			query: "namespace",
			opts:  backend.SearchOptions{Paths: []string{"src/base"}},
			expected: `src/base/strings/string_util.cc:namespace base {
src/base/strings/string_util.cc:}  // namespace base
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			eb := EngineBackend{
				Backend: mockBackend,
				Limit:   tt.opts.Limit,
			}
			eng := New([]EngineBackend{eb}, &buf, &buf, "parallel")

			err := eng.Run(context.Background(), tt.query, tt.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			actual := buf.String()
			
			// Separate results from status lines
			lines := strings.Split(strings.TrimSpace(actual), "\n")
			var resultLines []string
			var statusLines []string
			for _, line := range lines {
				if strings.HasPrefix(line, "[") && strings.Contains(line, "Showing") {
					statusLines = append(statusLines, line)
				} else {
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
				
				// Ensure both have same number of lines
				if len(actualLines) != len(expectedLines) {
					t.Errorf("expected %d lines, got %d. Output:\n%s", len(expectedLines), len(actualLines), actual)
				}
				
				// Check that each expected line exists in actual
				for _, el := range expectedLines {
					found := false
					for _, al := range actualLines {
						if el == al {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected to find line %q in output:\n%s", el, actual)
					}
				}
			} else {
				if actualResults != tt.expected {
					t.Errorf("expected:\n%s\n\ngot:\n%s", tt.expected, actualResults)
				}
			}
			
			// Verify we have at least one status line
			if len(statusLines) == 0 {
				t.Errorf("expected at least one status line, got none")
			}
		})
	}
}
