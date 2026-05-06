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

## 4. Summary Table

| Matching Feature | Azure DevOps | GitHub (CLI/API) |
| :--- | :--- | :--- |
| **Substring Match** | Supported (via auto `*`) | Best-effort (Remote token + Local filter) |
| **Wildcards (`*`, `?`)** | Supported natively | Supported (Limited by GitHub API) |
| **Case Insensitive (`-i`)**| Supported natively | Supported (Local filter on fragments) |
| **Whole Word (`-w`)** | Supported natively | Supported (via `"quotes"`) |
| **Regex Support** | No (Glob only) | No (Glob only) |
