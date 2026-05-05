package backend

import (
	"context"
)

type SearchOptions struct {
	IgnoreCase       bool
	LineNumber       bool
	Count            bool
	FilesWithMatches bool
	WordRegexp       bool
}

type SearchResult struct {
	File       string
	Line       int
	Content    string
	ContentId  string
	CharOffset int
	Length     int
}

type Backend interface {
	Name() string
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
}
