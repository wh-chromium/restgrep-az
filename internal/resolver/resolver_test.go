package resolver

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/wh-chromium/restgrep-az/internal/models"
)

func TestNaiveResolver_MapsIntermediateToFinal(t *testing.T) {
	r := &NaiveResolver{}
	ir := models.IntermediateResult{
		File:        "test.txt",
		RawFragment: "hello world",
		LineNumber:  42,
	}

	res := r.Resolve(context.Background(), ir, models.SearchOptions{})

	expected := Result{
		File:    "test.txt",
		Line:    42,
		Content: "hello world",
	}

	if !reflect.DeepEqual(res, expected) {
		t.Errorf("Naive mapping failed.\nExpected: %+v\nGot:      %+v", expected, res)
	}
}

func TestLocalResolver_MapsIntermediateToFinal(t *testing.T) {
	content := "line 1\nline 2\nTARGET\nline 4\n"
	tmpFile, _ := os.CreateTemp("", "resolver_test_*.txt")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte(content))
	tmpFile.Close()

	fileName := tmpFile.Name()
	sha := GetGitBlobSHA1([]byte(content))

	r := &LocalResolver{}

	t.Run("1. Exact SHA Match -> Resolves real line number and content", func(t *testing.T) {
		ir := models.IntermediateResult{
			File:        fileName,
			RemoteSHA:   sha,
			CharOffset:  14, // "TARGET"
			RawFragment: "[Remote Stub]",
			LineNumber:  1,
		}

		opts := models.SearchOptions{Query: "TARGET"}
		res := r.Resolve(context.Background(), ir, opts)

		expected := Result{
			File:    fileName,
			Line:    3, // It calculated the real line!
			Content: "TARGET",
			Message: "", 
		}

		if !reflect.DeepEqual(res, expected) {
			t.Errorf("Exact match failed.\nExpected: %+v\nGot:      %+v", expected, res)
		}
	})

	t.Run("2. Relaxed Match (SHA Mismatch) -> Resolves via local search and adds hint", func(t *testing.T) {
		ir := models.IntermediateResult{
			File:        fileName,
			RemoteSHA:   "stale-sha", // Mismatch!
			CharOffset:  99,          // Wrong offset!
			RawFragment: "[Remote Stub]",
			LineNumber:  1,
		}

		opts := models.SearchOptions{Query: "TARGET"}
		res := r.Resolve(context.Background(), ir, opts)

		expected := Result{
			File:    fileName,
			Line:    3,
			Content: "TARGET",
			Message: "(relaxed match)", // It warns you but still finds it
		}

		if !reflect.DeepEqual(res, expected) {
			t.Errorf("Relaxed match failed.\nExpected: %+v\nGot:      %+v", expected, res)
		}
	})

	t.Run("3. File Not Found -> Falls back to remote stub with warning", func(t *testing.T) {
		ir := models.IntermediateResult{
			File:        "does_not_exist.txt",
			RemoteSHA:   sha,
			CharOffset:  14,
			RawFragment: "[Remote Stub]",
			LineNumber:  42,
		}

		opts := models.SearchOptions{Query: "TARGET"}
		res := r.Resolve(context.Background(), ir, opts)

		expected := Result{
			File:    "does_not_exist.txt",
			Line:    42,
			Content: "[Remote Stub]",
			Message: "(local file not found)",
		}

		if !reflect.DeepEqual(res, expected) {
			t.Errorf("File not found fallback failed.\nExpected: %+v\nGot:      %+v", expected, res)
		}
	})
}
