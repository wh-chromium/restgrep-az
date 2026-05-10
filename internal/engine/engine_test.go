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
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/wh-chromium/restgrep-az/internal/backend"
)

func TestEngineInexactSHA1Adjustment(t *testing.T) {
	// 1. Setup a temporary git repository
	dir, err := os.MkdirTemp("", "restgrep_git_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Commit a file
	filename := "file.txt"
	content := "header\nMATCH_LINE\nfooter\n"
	os.WriteFile(filename, []byte(content), 0644)

	w, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add(filename); err != nil {
		t.Fatal(err)
	}
	commit, err := w.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get the SHA of the blob
	commitObj, err := repo.CommitObject(commit)
	if err != nil {
		t.Fatal(err)
	}
	file, err := commitObj.File(filename)
	if err != nil {
		t.Fatal(err)
	}
	remoteSHA := file.Hash.String()

	// "MATCH_LINE" is at offset 7 in the original file
	remoteOffset := 7

	// 3. Modify the file locally (add 2 lines at the top)
	// Now MATCH_LINE is at offset 21 (Assuming 7 chars per line + \n)
	newContent := "new line 1\nnew line 2\nheader\nMATCH_LINE\nfooter\n"
	os.WriteFile(filename, []byte(newContent), 0644)

	mockB := &mockGenericBackend{
		name: "git-test",
		results: []backend.SearchResult{
			{
				File:       filename,
				ContentId:  remoteSHA,
				CharOffset: remoteOffset,
				Content:    "MATCH_LINE",
			},
		},
	}

	t.Run("Adjustment Enabled", func(t *testing.T) {
		var buf bytes.Buffer
		eb := EngineBackend{Backend: mockB, Limit: 10}
		eng := New([]EngineBackend{eb}, &buf, &buf, "parallel")

		opts := backend.SearchOptions{InexactSHA1Adjustment: true, LineNumber: true}
		if err := eng.Run(context.Background(), "MATCH_LINE", opts); err != nil {
			t.Fatal(err)
		}

		output := buf.String()
		// Correct new line number should be 4 (10*2 + 7? No, just look at the string)
		// new line 1 (1)
		// new line 2 (2)
		// header (3)
		// MATCH_LINE (4)
		if !strings.Contains(output, "file.txt:4:MATCH_LINE") {
			t.Errorf("expected adjusted output to contain correct line 4, got:\n%s", output)
		}
	})

	t.Run("Adjustment Disabled", func(t *testing.T) {
		var buf bytes.Buffer
		eb := EngineBackend{Backend: mockB, Limit: 10}
		eng := New([]EngineBackend{eb}, &buf, &buf, "parallel")

		opts := backend.SearchOptions{InexactSHA1Adjustment: false}
		if err := eng.Run(context.Background(), "MATCH_LINE", opts); err != nil {
			t.Fatal(err)
		}

		output := buf.String()
		if !strings.Contains(output, "local file mismatch") {
			t.Errorf("expected local file mismatch warning, got:\n%s", output)
		}
	})
}

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

func TestEngineContextExtraction(t *testing.T) {
	// 1. Create a temporary file with several lines
	content := "line 1\nline 2\nline 3\nMATCH\nline 5\nline 6\nline 7\n"
	tmpFile, _ := os.CreateTemp("", "restgrep_ctx_*.txt")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte(content))
	tmpFile.Close()

	sha := getGitBlobSHA1([]byte(content))
	fileName := tmpFile.Name()

	// "MATCH" is at char offset 21 (6+1 + 6+1 + 6+1)
	matchOffset := 21

	mockB := &mockGenericBackend{
		name: "test-ctx",
		results: []backend.SearchResult{
			{
				File:       fileName,
				ContentId:  sha,
				CharOffset: matchOffset,
				Content:    "MATCH",
			},
		},
	}

	tests := []struct {
		name     string
		opts     backend.SearchOptions
		expected string
	}{
		{
			name: "After Context (-A 1)",
			opts: backend.SearchOptions{AfterContext: 1},
			expected: fmt.Sprintf("%s:MATCH\n%s-line 5\n", fileName, fileName),
		},
		{
			name: "Before Context (-B 1)",
			opts: backend.SearchOptions{BeforeContext: 1},
			expected: fmt.Sprintf("%s-line 3\n%s:MATCH\n", fileName, fileName),
		},
		{
			name: "Full Context (-C 1)",
			opts: backend.SearchOptions{BeforeContext: 1, AfterContext: 1},
			expected: fmt.Sprintf("%s-line 3\n%s:MATCH\n%s-line 5\n", fileName, fileName, fileName),
		},
		{
			name: "Full Context with line numbers (-C 1 -n)",
			opts: backend.SearchOptions{BeforeContext: 1, AfterContext: 1, LineNumber: true},
			expected: fmt.Sprintf("%s-3-line 3\n%s:4:MATCH\n%s-5-line 5\n", fileName, fileName, fileName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			eb := EngineBackend{Backend: mockB, Limit: 10}
			eng := New([]EngineBackend{eb}, &buf, &buf, "parallel")

			if err := eng.Run(context.Background(), "MATCH", tt.opts); err != nil {
				t.Fatalf("Run failed: %v", err)
			}

			// Extract results from output (ignoring status line)
			output := buf.String()
			lines := strings.Split(strings.TrimSpace(output), "\n")
			var resultLines []string
			for _, line := range lines {
				if !strings.Contains(line, "Showing") {
					resultLines = append(resultLines, line)
				}
			}
			actualResults := strings.Join(resultLines, "\n")
			if len(resultLines) > 0 {
				actualResults += "\n"
			}

			if actualResults != tt.expected {
				t.Errorf("expected:\n%q\ngot:\n%q", tt.expected, actualResults)
			}
		})
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
