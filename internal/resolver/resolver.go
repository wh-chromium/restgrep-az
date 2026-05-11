package resolver

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/wh-chromium/restgrep-az/internal/models"
)

type Result struct {
	File    string
	Line    int
	Content string
	Message string // e.g. "local file mismatch"
	Lines   []models.EnrichedLine
}

type Resolver interface {
	Resolve(ctx context.Context, ir models.IntermediateResult, opts models.SearchOptions) Result
}

// 1. Naive Resolver: Directly uses what the API returned.
type NaiveResolver struct{}

func (n *NaiveResolver) Resolve(ctx context.Context, ir models.IntermediateResult, opts models.SearchOptions) Result {
	if opts.Debug {
		fmt.Printf("[DEBUG][resolver] Naive resolution for %s\n", ir.File)
	}
	return Result{
		File:    ir.File,
		Line:    ir.LineNumber,
		Content: ir.RawFragment,
	}
}

// 2. Local Resolver: Tries to find the code in a local file if it exists.
type LocalResolver struct{}

func (l *LocalResolver) Resolve(ctx context.Context, ir models.IntermediateResult, opts models.SearchOptions) Result {
	if opts.Debug {
		fmt.Printf("[DEBUG][resolver] Local resolution for %s\n", ir.File)
	}

	localPath := strings.TrimPrefix(ir.File, "/")
	data, err := os.ReadFile(localPath)
	if err != nil {
		if opts.Debug {
			fmt.Printf("[DEBUG][resolver] Local file not found: %s\n", localPath)
		}
		return Result{File: ir.File, Line: ir.LineNumber, Content: ir.RawFragment, Message: "(local file not found)"}
	}

	// Strategy: If we have an offset, check there first.
	// But since we are "relaxed", we will also search the file for the pattern.
	
	// Try to find the line in the current local data.
	q := opts.Query
	if q == "" {
		// Fallback to fragment if query is somehow missing from options
		q = ir.RawFragment
	}

	foundOffset := -1
	
	// If WordRegexp is active, use regex for exact word search
	if opts.WordRegexp {
		pattern := `\b` + regexp.QuoteMeta(q) + `\b`
		if opts.IgnoreCase {
			pattern = `(?i)\b` + regexp.QuoteMeta(q) + `\b`
		}
		re, err := regexp.Compile(pattern)
		if err == nil {
			loc := re.FindIndex(data)
			if loc != nil {
				foundOffset = loc[0]
			}
		}
	} else {
		// Simple substring search
		searchData := string(data)
		searchTerm := q
		if opts.IgnoreCase {
			searchData = strings.ToLower(searchData)
			searchTerm = strings.ToLower(searchTerm)
		}
		foundOffset = strings.Index(searchData, searchTerm)
	}

	if foundOffset >= 0 {
		content, line := GetLineFromOffset(data, foundOffset)
		
		// If SHA1 matches, it's a "High Confidence" local resolution.
		// If it doesn't match, we still found the code, so we print it but maybe add a hint.
		message := ""
		if ir.RemoteSHA != "" {
			localSHA := GetGitBlobSHA1(data)
			if localSHA != ir.RemoteSHA {
				message = "(relaxed match)"
			}
		}
		
		var lines []models.EnrichedLine
		if opts.BeforeContext > 0 || opts.AfterContext > 0 {
			lines = GetLinesWithContext(data, foundOffset, opts.BeforeContext, opts.AfterContext)
		}

		return Result{File: ir.File, Line: line, Content: content, Message: message, Lines: lines}
	}

	if opts.Debug {
		fmt.Printf("[DEBUG][resolver] Query pattern not found in local file: %s\n", localPath)
	}
	return Result{File: ir.File, Line: ir.LineNumber, Content: ir.RawFragment, Message: "(local file out of sync)"}
}

// Helpers

func GetLinesWithContext(data []byte, charOffset int, before, after int) []models.EnrichedLine {
	// 1. Find the match line start and number
	matchLine := 1
	lineStart := 0
	for i := 0; i < charOffset && i < len(data); i++ {
		if data[i] == '\n' {
			matchLine++
			lineStart = i + 1
		}
	}

	// Helper to get line end
	getLineEnd := func(start int) int {
		for i := start; i < len(data); i++ {
			if data[i] == '\n' || data[i] == '\r' {
				return i
			}
		}
		return len(data)
	}

	// 2. Collect match line
	matchEnd := getLineEnd(lineStart)
	lines := []models.EnrichedLine{{Text: string(data[lineStart:matchEnd]), Number: matchLine, Match: true}}

	// 3. Collect "before" context
	currStart := lineStart
	for i := 0; i < before; i++ {
		if currStart <= 0 {
			break
		}
		// Look for start of previous line
		searchIdx := currStart - 2
		if searchIdx < 0 {
			break
		}
		prevStart := 0
		for j := searchIdx; j >= 0; j-- {
			if data[j] == '\n' {
				prevStart = j + 1
				break
			}
		}
		prevEnd := getLineEnd(prevStart)
		lines = append([]models.EnrichedLine{{Text: string(data[prevStart:prevEnd]), Number: matchLine - (i + 1), Match: false}}, lines...)
		currStart = prevStart
	}

	// 4. Collect "after" context
	currEnd := matchEnd
	for i := 0; i < after; i++ {
		if currEnd >= len(data) {
			break
		}
		nextStart := currEnd + 1
		if nextStart >= len(data) {
			break
		}
		nextEnd := getLineEnd(nextStart)
		lines = append(lines, models.EnrichedLine{Text: string(data[nextStart:nextEnd]), Number: matchLine + (i + 1), Match: false})
		currEnd = nextEnd
	}

	return lines
}

func GetGitBlobSHA1(data []byte) string {
	// Re-calculating for the "relaxed match" hint
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
