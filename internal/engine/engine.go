package engine

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/restgrep-az/restgrep/internal/backend"
)

type Engine struct {
	backends []backend.Backend
	out      io.Writer
}

func New(backends []backend.Backend, out io.Writer) *Engine {
	return &Engine{backends: backends, out: out}
}

func getGitBlobSHA1(data []byte) string {
	h := sha1.New()
	h.Write([]byte(fmt.Sprintf("blob %d\x00", len(data))))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func getLineFromOffset(data []byte, charOffset int) (string, int) {
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

func (e *Engine) Run(ctx context.Context, query string, opts backend.SearchOptions) error {
	for _, b := range e.backends {
		results, err := b.Search(ctx, query, opts)
		if err != nil {
			return fmt.Errorf("backend %s failed: %w", b.Name(), err)
		}
		
		if opts.Count {
			// group by file
			counts := make(map[string]int)
			for _, r := range results {
				counts[r.File]++
			}
			for file, count := range counts {
				fmt.Fprintf(e.out, "%s:%d\n", file, count)
			}
			continue
		}

		if opts.FilesWithMatches {
			files := make(map[string]bool)
			for _, r := range results {
				if !files[r.File] {
					fmt.Fprintln(e.out, r.File)
					files[r.File] = true
				}
			}
			continue
		}

		for _, r := range results {
			content := r.Content
			line := r.Line

			if r.ContentId != "" {
				localPath := strings.TrimPrefix(r.File, "/")
				data, err := os.ReadFile(localPath)
				if err == nil {
					sha := getGitBlobSHA1(data)
					if sha == r.ContentId {
						content, line = getLineFromOffset(data, r.CharOffset)
					} else {
						content = fmt.Sprintf("%s (local file mismatch)", r.Content)
					}
				} else {
					content = fmt.Sprintf("%s (local file not found)", r.Content)
				}
			}

			if opts.LineNumber {
				fmt.Fprintf(e.out, "%s:%d:%s\n", r.File, line, content)
			} else {
				fmt.Fprintf(e.out, "%s:%s\n", r.File, content)
			}
		}
	}
	return nil
}


