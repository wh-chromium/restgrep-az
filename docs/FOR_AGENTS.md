# restgrep: Guide for AI Agents

This document provides a technical specification for implementing new backends in `restgrep`. Use this to add support for proprietary or internal search APIs.

## Implementation Checklist

To add a new backend, follow these steps:

1.  **Define the Backend**: Create a new package under `internal/backend/` (e.g., `internal/backend/mycompany/`).
2.  **Implement the Interface**: Your struct must satisfy the `backend.Backend` interface:
    ```go
    type Backend interface {
        Name() string
        Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
    }
    ```
3.  **Map Flags to API**:
    - **IgnoreCase (`-i`)**: If the remote API supports case-insensitivity, pass it through. If not, filter the results locally.
    - **WordRegexp (`-w`)**: Ensure the query matches only whole words. For GitHub, this means wrapping in `"quotes"`. For Azure, it's the default. For others, use regex `\b`.
4.  **Populate Local Resolution Metadata**: To enable real-time local file resolution (replacing remote stubs with actual code), your `SearchResult` **MUST** include:
    - `File`: The repository-relative path (e.g., `src/main.go`).
    - `ContentId`: The Git blob SHA1 of the file at the time of search.
    - `CharOffset`: The 0-based character offset of the match.
    - `Length`: The length of the match string.

## Rigorous Testing

`restgrep` provides a formal contract validation suite. You **SHOULD** use this in your backend tests to ensure parity with `grep` behavior.

**Example Test:**
```go
func TestMyBackend(t *testing.T) {
    b := NewMyBackend(...)
    cases := []backend.ContractTestCase{
        {
            Name: "Substring Search",
            Query: "search",
            Options: backend.SearchOptions{},
            ExpectedFiles: []string{"src/search_service.go"},
        },
        {
            Name: "Word Match Boundary",
            Query: "WebContents",
            Options: backend.SearchOptions{WordRegexp: true},
            ForbiddenStrings: []string{"WebContentsImpl"}, // Should not match partials
        },
    }
    backend.VerifyBackendContract(t, b, cases)
}
```

## Integration

Finally, update `cmd/restgrep/main.go` and `internal/config/config.go` to include your new backend type in the JSON settings loader.
