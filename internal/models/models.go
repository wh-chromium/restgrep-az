package models

// IntermediateResult is the normalized format produced by all frontends.
type IntermediateResult struct {
	File        string
	RemoteSHA   string
	CharOffset  int    // -1 if not provided
	Length      int    // -1 if not provided
	RawFragment string // The raw snippet or line returned by the API
	LineNumber  int    // Backend's best guess (often 1)
}

// ResolverMode defines how an intermediate result should be finalized.
type ResolverMode string

const (
	ModeNaive         ResolverMode = "naive"
	ModeLocal         ResolverMode = "local"
)

// SearchOptions is the shared configuration for a search run.
type SearchOptions struct {
	IgnoreCase       bool
	LineNumber       bool
	Count            bool
	FilesWithMatches bool
	WordRegexp       bool
	Limit            int
	Paths            []string
	AfterContext     int
	BeforeContext    int
	Debug            bool
	Query            string // The original query pattern
	MergeBaseBranch  string // e.g. "origin/main"
}
