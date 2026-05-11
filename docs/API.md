# Restgrep API Documentation

`restgrep` is a normalization layer for remote code search APIs. It provides a standard `grep`-like interface that delegates queries to remote APIs, optimizing token usage and performance.

## Core Concepts

- **Backend**: A remote service providing search capabilities (e.g., Azure DevOps, GitHub).
- **Engine**: The core orchestrator that manages parallel/sequential execution, result merging, and local file enrichment.
- **Double-Sort Strategy**: To maximize cache efficiency, the engine sorts all collected results by **filename** to perform O(files) local resolution, then re-sorts them to match the **original provider order** for the user.
- **MRU Cache**: A single-file memory cache ensures each unique file is read and hashed only once per search.

## Settings Configuration (`restgrep.json`)

| Field | Type | Description |
| :--- | :--- | :--- |
| `backends` | Array | List of search backends to use (see below). |
| `execution_mode` | String | `parallel` (default) or `sequential`. |
| `backend_mode` | String | `naive`, `local` (default), or `try-diff-from-merge-base`. |
| `merge_base_branch` | String | The branch to use for drift recovery in `try-diff-from-merge-base` mode (e.g., `origin/main`). |

### Execution Modes

1.  **Parallel Mode (`parallel`)**:
    - Executes all backends simultaneously.
    - Merges results from all successful backends.
    - **Ignores failed backends** (errors are reported to `stderr`).

2.  **Sequential Fallback Mode (`sequential`)**:
    - Executes backends one-by-one in order.
    - **Stops after the first successful execution**.

---

### Resolver Modes

1.  **Naive Mode (`naive`)**:
    - Directly prints the remote match fragment returned by the API.
    - No local validation or line number correction.

2.  **Local Mode (`local`)**:
    - Smarter, fallback-based resolution.
    - Performs a substring search for the query pattern in your local working copy.
    - Resolves real line numbers even if the file has drifted.
    - Adds `(relaxed match)` if SHA1 doesn't match but pattern is found.

3.  **Merge-Base Diff Mode (`try-diff-from-merge-base`)**:
    - Complex drift recovery using `go-git`.
    - Assumes the remote search index matches a specific branch (e.g. `origin/main`).
    - Finds the file state at that branch, calculates its content, and diffs it against your local working copy.
    - Adjusts offsets and line numbers based on the delta between the remote indexed branch and your current branch.

---

## Supported Backends

### 1. Azure DevOps (`azure`)
- **API**: Uses Azure DevOps REST API v7.1.
- **Auth**: Dynamically fetches tokens via `az account get-access-token`.
- **Metadata**: Provides precise `charOffset`, allowing full local resolution and context.

### 2. GitHub CLI (`github`)
- **Mechanism**: Wraps the `gh search code` CLI command.
- **Auth**: Requires `gh auth login`.
- **Filtering**: Performs local substring filtering on fragments to ensure `grep` accuracy.

### 3. GitHub API (`github-api`)
- **Mechanism**: Calls the GitHub REST API directly via `gh api`.
- **Header**: Uses `vnd.github.v3.text-match+json` for rich match metadata.
- **Advantage**: Higher metadata consistency than the raw CLI backend.

### Backend Config Fields

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `type` | String | Yes | `azure`, `github`, or `github-api`. |
| `organization` | String | Azure only | Azure DevOps organization name. |
| `project` | String | Azure only | Azure DevOps project name. |
| `repo` | String | GH only | Target repository (e.g., `chromium/chromium`). |
| `limit` | Integer | No | Result limit for this backend (default: 100). |

---

## Sample Full Configuration

```json
{
  "execution_mode": "parallel",
  "merge_base_branch": "origin/main",
  "backends": [
    {
      "type": "azure",
      "organization": "Initech",
      "project": "CoverReportTemplates",
      "limit": 100,
      "backend_mode": "try-diff-from-merge-base"
    },
    {
      "type": "github-api",
      "repo": "chromium/chromium",
      "limit": 50,
      "backend_mode": "local"
    }
  ]
}
```
