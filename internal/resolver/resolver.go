package resolver

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/wh-chromium/restgrep-az/internal/models"
)

type Result struct {
	File    string
	Line    int
	Content string
	Message string // e.g. "local file mismatch"
}

type Resolver interface {
	Resolve(ctx context.Context, ir models.IntermediateResult, debug bool) Result
}

// 1. Naive Resolver
type NaiveResolver struct{}

func (n *NaiveResolver) Resolve(ctx context.Context, ir models.IntermediateResult, debug bool) Result {
	if debug {
		fmt.Printf("[DEBUG][resolver] Naive resolution for %s\n", ir.File)
	}
	return Result{
		File:    ir.File,
		Line:    ir.LineNumber,
		Content: ir.RawFragment,
	}
}

// 2. Local No Diff Resolver
type LocalNoDiffResolver struct{}

func (l *LocalNoDiffResolver) Resolve(ctx context.Context, ir models.IntermediateResult, debug bool) Result {
	if debug {
		fmt.Printf("[DEBUG][resolver] LocalNoDiff resolution for %s\n", ir.File)
	}
	
	if ir.RemoteSHA == "" {
		return Result{File: ir.File, Line: ir.LineNumber, Content: ir.RawFragment, Message: "(no SHA provided)"}
	}

	localPath := strings.TrimPrefix(ir.File, "/")
	data, err := os.ReadFile(localPath)
	if err != nil {
		if debug {
			fmt.Printf("[DEBUG][resolver] Local file not found: %s\n", localPath)
		}
		return Result{File: ir.File, Line: ir.LineNumber, Content: ir.RawFragment, Message: "(local file not found)"}
	}

	sha := GetGitBlobSHA1(data)
	if sha == ir.RemoteSHA {
		if debug {
			fmt.Printf("[DEBUG][resolver] SHA1 matched for %s\n", ir.File)
		}
		if ir.CharOffset >= 0 {
			content, line := GetLineFromOffset(data, ir.CharOffset)
			return Result{File: ir.File, Line: line, Content: content}
		}
		// Fallback if offset is missing but SHA matches
		return Result{File: ir.File, Line: ir.LineNumber, Content: ir.RawFragment}
	}

	if debug {
		fmt.Printf("[DEBUG][resolver] SHA1 mismatch for %s (local: %s, remote: %s)\n", ir.File, sha, ir.RemoteSHA)
	}
	return Result{File: ir.File, Line: ir.LineNumber, Content: ir.RawFragment, Message: "(local file mismatch)"}
}

// 3. Local With Diff Resolver
type LocalWithDiffResolver struct{}

func (l *LocalWithDiffResolver) Resolve(ctx context.Context, ir models.IntermediateResult, debug bool) Result {
	if debug {
		fmt.Printf("[DEBUG][resolver] LocalWithDiff resolution for %s\n", ir.File)
	}

	if ir.RemoteSHA == "" {
		return Result{File: ir.File, Line: ir.LineNumber, Content: ir.RawFragment, Message: "(no SHA provided)"}
	}

	localPath := strings.TrimPrefix(ir.File, "/")
	data, err := os.ReadFile(localPath)
	if err != nil {
		return Result{File: ir.File, Line: ir.LineNumber, Content: ir.RawFragment, Message: "(local file not found)"}
	}

	sha := GetGitBlobSHA1(data)
	if sha == ir.RemoteSHA {
		if debug {
			fmt.Printf("[DEBUG][resolver] SHA1 matched for %s\n", ir.File)
		}
		if ir.CharOffset >= 0 {
			content, line := GetLineFromOffset(data, ir.CharOffset)
			return Result{File: ir.File, Line: line, Content: content}
		}
		return Result{File: ir.File, Line: ir.LineNumber, Content: ir.RawFragment}
	}

	// SHA Mismatch: Try Diff Adjustment
	if debug {
		fmt.Printf("[DEBUG][resolver] SHA1 mismatch, attempting diff adjustment for %s\n", ir.File)
	}
	
	newOffset, ok := resolveInexactOffset(ir.RemoteSHA, data, ir.CharOffset)
	if ok {
		if debug {
			fmt.Printf("[DEBUG][resolver] Diff adjustment successful for %s\n", ir.File)
		}
		content, line := GetLineFromOffset(data, newOffset)
		return Result{File: ir.File, Line: line, Content: content}
	}

	if debug {
		fmt.Printf("[DEBUG][resolver] Diff adjustment failed for %s\n", ir.File)
	}
	return Result{File: ir.File, Line: ir.LineNumber, Content: ir.RawFragment, Message: "(local file mismatch - adjustment failed)"}
}

// Helpers (moved from engine)

func GetGitBlobSHA1(data []byte) string {
	h := sha1.New()
	h.Write([]byte(fmt.Sprintf("blob %d\x00", len(data))))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func GetLineFromOffset(data []byte, charOffset int) (string, int) {
	if charOffset < 0 || charOffset >= len(data) {
		return "", 1
	}
	line := 1
	lineStart := 0
	for i := 0; i < charOffset && i < len(data); i++ {
		if data[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	lineEnd := len(data)
	for i := charOffset; i < len(data); i++ {
		if data[i] == '\n' || data[i] == '\r' {
			lineEnd = i
			break
		}
	}
	return string(data[lineStart:lineEnd]), line
}

func resolveInexactOffset(remoteSHA string, localData []byte, remoteCharOffset int) (int, bool) {
	if remoteCharOffset < 0 {
		return 0, false
	}
	repo, err := git.PlainOpen(".")
	if err != nil {
		return 0, false
	}

	hash := plumbing.NewHash(remoteSHA)
	blob, err := repo.BlobObject(hash)
	if err != nil {
		return 0, false
	}

	reader, err := blob.Reader()
	if err != nil {
		return 0, false
	}
	defer reader.Close()
	remoteData, err := io.ReadAll(reader)
	if err != nil {
		return 0, false
	}

	remoteLineContent, _ := GetLineFromOffset(remoteData, remoteCharOffset)
	if remoteLineContent == "" {
		return 0, false
	}
	
	lines := strings.Split(string(localData), "\n")
	for i, line := range lines {
		if strings.Contains(line, remoteLineContent) {
			offset := 0
			for j := 0; j < i; j++ {
				offset += len(lines[j]) + 1
			}
			return offset, true
		}
	}

	return 0, false
}
