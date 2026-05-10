# `restgrep` Limitations & Constraints

`restgrep` is a powerful normalization layer, but it is constrained by the design of the remote search APIs it consumes.

## 1. Line Number Inaccuracy (GitHub Backends)

Neither the GitHub CLI nor the GitHub REST API provide precise character offsets or line numbers for search matches. They provide "text fragments."

- **Behavior**: When a local file is NOT available, GitHub results will report a placeholder line number (`1`).
- **Improvement**: If you have the repository checked out locally, `restgrep` will attempt to find the fragment within your local file to resolve the correct line number, though this can be ambiguous if the fragment appears multiple times.

## 2. Remote Context Constraints

Remote APIs do not return surrounding lines of code.

- **Standard Behavior**: Without a local repository, context flags (`-A`, `-B`, `-C`) will have no effect on the output.
- **Solution**: Always use `restgrep` inside a local checkout of the repository for full `grep` parity.

## 3. Rate Limiting

Remote search APIs (especially GitHub) are strictly rate-limited.

- **GitHub API**: Limits are typically ~10 requests per minute for code search.
- **Parallel Mode**: `restgrep`'s parallel execution consumes rate limits across multiple backends simultaneously.
- **Mitigation**: Use the `-m` (max-count) flag to reduce the amount of data requested and processed.

## 4. Path Filtering logic

Position path filtering (e.g., `restgrep pattern dir1`) is mapped to remote `path:` filters.

- **Azure DevOps**: Robust directory-level filtering.
- **GitHub**: Highly dependent on GitHub's indexing. GitHub may sometimes include results from paths that closely match the filter but are not exact directory sub-matches.

## 5. Regex vs. Glob

`restgrep` translates queries into **Glob** wildcards (`*`, `?`) for remote APIs. It does **not** support full PCRE or POSIX Regular Expressions. Passing complex regex characters (like `(` or `|`) will be treated as literals or may cause API errors.
