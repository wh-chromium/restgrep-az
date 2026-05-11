# `restgrep` Matching Behavior

This document explains how `restgrep` translates standard `grep` queries into remote API calls and how it ensures matching accuracy across different search engines.

## 1. Partial & Substring Matching

Standard `grep` matches any substring by default. Remote APIs often default to **tokenized/exact-word matching** for performance. `restgrep` bridges this gap using different strategies:

### Azure DevOps
- **Auto-Wildcarding**: If your query contains no wildcards (`*` or `?`), `restgrep` automatically wraps it (e.g., `*query*`). This forces Azure to perform a substring search.
- **Leading Wildcard Restriction**: Note that some Azure DevOps environments disable "Leading Wildcard" searches (starting a query with `*`). In such cases, the automatic `*query*` may return an error or zero results.

### GitHub (CLI & API)
- **Token-Based Search**: GitHub's engine is designed to find code symbols. It may not find arbitrary substrings that do not align with its internal tokenization rules.
- **Local Post-Filtering**: When GitHub returns a code fragment, `restgrep` performs a second **local substring match** on each line of the fragment. This ensures that the output strictly contains your query, even if the remote API returned a broader context.

## 2. Word Matching (`-w`)

The `-w` / `--word-regexp` flag is mapped to native "exact match" features in the remote APIs:

- **Azure DevOps**: Skips the auto-wildcarding step, leveraging Azure's native exact-word index.
- **GitHub**: Wraps the query in double quotes (e.g., `"query"`) to force the GitHub engine to treat it as a single, atomic token.
- **Mock/Chromium**: Uses Regex word boundaries (`\bquery\b`) to ensure 100% parity during testing.

## 3. Glob vs. Regex

It is important to note that `restgrep` backends primarily support **Glob-style wildcards**, whereas standard `grep` uses **Regular Expressions**.

| Feature | `grep` Syntax | `restgrep` Syntax |
| :--- | :--- | :--- |
| Multiple Characters | `.*` | `*` |
| Single Character | `.` | `?` |
| Literal Dot | `\.` | `.` |

*Note: Passing complex Regular Expressions to `restgrep` will likely result in literal character matches or API errors, as they are not currently translated into Glob syntax.*

## 4. Path Filtering

`restgrep` supports standard `grep` positional arguments for path filtering: `restgrep PATTERN [PATH...]`.

- **Azure DevOps**: Map to the `Path` filter array. Paths are automatically prefixed with `/` if missing.
- **GitHub (CLI/API)**: Appended to the query string as `path:PATH` qualifiers.
- **Multiple Paths**: If multiple paths are provided, `restgrep` searches in all of them (Logical OR behavior).

## 5. Result Sorting & Cache Efficiency

`restgrep` automatically **merges and sorts all results by filename** before performing local file enrichment.

- **Standard `grep` Parity**: Grouping matches by file is the standard behavior for code search tools.
- **Cache Optimization**: By sorting by filename, the Single-File MRU cache achieves **100% efficiency**. `restgrep` will open, read, and hash each unique file in your result set **exactly once**, regardless of how many matches are found in that file or how many backends returned it.

## 6. Summary Table

| Matching Feature | Azure DevOps | GitHub (CLI/API) |
| :--- | :--- | :--- |
| **Substring Match** | Supported (via auto `*`) | Best-effort (Remote token + Local filter) |
| **Wildcards (`*`, `?`)** | Supported natively | Supported (Limited by GitHub API) |
| **Case Insensitive (`-i`)**| Supported natively | Supported (Local filter on fragments) |
| **Whole Word (`-w`)** | Supported natively | Supported (via `"quotes"`) |
| **Path Filtering** | Supported (via `Path` filter) | Supported (via `path:` qualifier) |
| **Local Search** | Relaxed substring search | Relaxed substring search |
| **Regex Support** | No (Glob only) | No (Glob only) |

## 6. Relaxed Local Matching

When you have a local copy of a repository, `restgrep` tries to resolve the remote match to the actual current source code on your disk.

Unlike traditional tools that fail if the SHA1 hash doesn't match perfectly, `restgrep` uses a **Relaxed Matching** strategy:
1.  **Exact Match**: If the local file's SHA1 matches the remote index, `restgrep` uses the provided offset to extract the line perfectly.
2.  **Drift Recovery**: If the SHA1 doesn't match (i.e., you modified the file), `restgrep` performs a substring search for your **original query pattern** within the current local file.
3.  **Result**: If found, it corrects the line numbers and content dynamically and appends a `(relaxed match)` hint.

This ensures you can always see the real code even if your local repository is out of sync with the remote search index.
