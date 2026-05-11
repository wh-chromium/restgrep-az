package resolver

import (
	"context"
	"os"
	"testing"

	"github.com/wh-chromium/restgrep-az/internal/models"
)

func TestNaiveResolver(t *testing.T) {
	r := &NaiveResolver{}
	ir := models.IntermediateResult{
		File:        "test.txt",
		RawFragment: "hello world",
		LineNumber:  42,
	}

	res := r.Resolve(context.Background(), ir, models.SearchOptions{})

	if res.File != "test.txt" {
		t.Errorf("expected test.txt, got %s", res.File)
	}
	if res.Line != 42 {
		t.Errorf("expected 42, got %d", res.Line)
	}
	if res.Content != "hello world" {
		t.Errorf("expected hello world, got %s", res.Content)
	}
}

func TestLocalResolver(t *testing.T) {
	content := "line 1\nline 2\nTARGET\nline 4\n"
	tmpFile, _ := os.CreateTemp("", "resolver_test_*.txt")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte(content))
	tmpFile.Close()

	fileName := tmpFile.Name()
	sha := GetGitBlobSHA1([]byte(content))

	r := &LocalResolver{}

	t.Run("Exact SHA Match", func(t *testing.T) {
		ir := models.IntermediateResult{
			File:        fileName,
			RemoteSHA:   sha,
			CharOffset:  14, // "TARGET"
			RawFragment: "[Remote Stub]",
			LineNumber:  1,
		}

		opts := models.SearchOptions{Query: "TARGET"}
		res := r.Resolve(context.Background(), ir, opts)

		if res.Content != "TARGET" {
			t.Errorf("expected TARGET, got %s", res.Content)
		}
		if res.Line != 3 {
			t.Errorf("expected line 3, got %d", res.Line)
		}
		if res.Message != "" {
			t.Errorf("expected empty message, got %s", res.Message)
		}
	})

	t.Run("Relaxed Match (SHA Mismatch)", func(t *testing.T) {
		ir := models.IntermediateResult{
			File:        fileName,
			RemoteSHA:   "stale-sha", // Mismatch
			CharOffset:  99,          // Wrong offset
			RawFragment: "[Remote Stub]",
			LineNumber:  1,
		}

		opts := models.SearchOptions{Query: "TARGET"}
		res := r.Resolve(context.Background(), ir, opts)

		if res.Content != "TARGET" {
			t.Errorf("expected TARGET, got %s", res.Content)
		}
		if res.Line != 3 {
			t.Errorf("expected line 3, got %d", res.Line)
		}
		if res.Message != "(relaxed match)" {
			t.Errorf("expected relaxed match hint, got %s", res.Message)
		}
	})

	t.Run("Context Extraction", func(t *testing.T) {
		ir := models.IntermediateResult{
			File:        fileName,
			RemoteSHA:   sha,
			CharOffset:  14,
			RawFragment: "[Remote Stub]",
			LineNumber:  1,
		}

		opts := models.SearchOptions{Query: "TARGET", BeforeContext: 1, AfterContext: 1}
		res := r.Resolve(context.Background(), ir, opts)

		if len(res.Lines) != 3 {
			t.Fatalf("expected 3 context lines, got %d", len(res.Lines))
		}
		if res.Lines[0].Text != "line 2" || res.Lines[1].Text != "TARGET" || res.Lines[2].Text != "line 4" {
			t.Errorf("context extraction failed: %+v", res.Lines)
		}
	})
	
	t.Run("File Not Found", func(t *testing.T) {
		ir := models.IntermediateResult{
			File:        "does_not_exist.txt",
			RemoteSHA:   sha,
			CharOffset:  14,
			RawFragment: "[Remote Stub]",
			LineNumber:  42,
		}

		opts := models.SearchOptions{Query: "TARGET"}
		res := r.Resolve(context.Background(), ir, opts)

		if res.Content != "[Remote Stub]" {
			t.Errorf("expected stub fallback, got %s", res.Content)
		}
		if res.Message != "(local file not found)" {
			t.Errorf("expected not found message, got %s", res.Message)
		}
	})
}
