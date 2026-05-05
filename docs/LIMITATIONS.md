# `restgrep` Limitations

This document outlines the known limitations of `restgrep` that stem from the design of the underlying remote APIs it consumes.

## 1. Line Number Inaccuracy (GitHub Backend)

The `gh search code` command and its underlying GitHub REST API **do not** provide line numbers in their search results. The API returns a "fragment" of text containing the match, but not its position within the file.

As a result, when using the `github` backend, `restgrep` will always report a placeholder line number (e.g., `1`), even when the `-n` flag is used.

**Example Output:**
```text
content/public/browser/web_contents.h:1:class WebContents;
```
*(The line number `1` is a placeholder and may not be accurate.)*

In contrast, the `azure` backend can provide accurate line numbers if the file is resolved locally via SHA1 validation, as it gets a `charOffset` from the API.

## 2. Lack of Surrounding File Context (Remote-Only)

Neither the Azure DevOps nor the GitHub search APIs return the full file content in their initial search responses. This is an intentional design choice to keep the API payloads small and fast.

-   **Azure DevOps:** Provides a character offset (`charOffset`) which allows `restgrep` to calculate the line and content **if and only if** the file is available locally and its SHA1 hash matches.
-   **GitHub:** Provides a small text fragment, but no positional data.

If a file is not present on your local disk, `restgrep` cannot display the full, real line of code and will instead show the metadata provided by the backend (e.g., `[Match at char offset...]` for Azure). To get full `grep` parity in these cases, the file would need to be checked out locally.
