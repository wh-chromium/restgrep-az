# restgrep: Guide for AI Agents & Backend Developers

This document provides a technical specification for implementing new backends and understanding the `restgrep` engine's optimization strategies.

## Implementation Checklist

To add a new backend, follow these steps:

1.  **Define the Backend**: Create a new package under `internal/backend/` (e.g., `internal/backend/mycompany/`).
2.  **Implement the Interface**:
    ```go
    type Backend interface {
        Name() string
        Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
    }
    ```
3.  **Testable Execution**: Use the `Executor` interface pattern. This allows you to mock CLI commands (like `gh`) or API responses in unit tests without external dependencies.
4.  **Populate Resolution Metadata**: To enable local line/context extraction, your `SearchResult` **MUST** include:
    - `File`: Repository-relative path.
    - `ContentId`: The Git blob SHA1 of the file at the time of search.
    - `CharOffset`: The 0-based character offset of the match (if provided by API).
    - `Length`: The length of the match string.

## Engine Architecture: Double-Sort

The `restgrep` engine uses a two-pass sorting strategy to optimize local file accesses:

1.  **Phase 1 (Aggregation)**: Results are collected from backends (parallel or sequential).
2.  **Phase 2 (Sorting for Efficiency)**: Results are mapped to pointers and sorted globally by **File Path**.
3.  **Phase 3 (Enrichment)**: The enrichment process resolves matches. Grouping by file ensures sequential access pattern.
4.  **Phase 4 (Sorting for User)**: Enriched results are re-sorted back to their **original provider order** before being printed.

## Flag Mapping Guidelines

- **IgnoreCase (`-i`)**: Pass through to the API if supported; otherwise, filter locally using `strings.ToLower`.
- **WordRegexp (`-w`)**: Map to native exact-match features (e.g., `"quotes"` for GitHub).
- **Paths**: Handle multiple positional arguments by executing iterative queries if the remote API does not support `OR` logic for paths.


