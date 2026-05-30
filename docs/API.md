# Restgrep API Documentation

`restgrep` is a normalization layer for remote code search APIs. It provides a standard `grep`-like interface that delegates queries to remote APIs, optimizing token usage and performance.

## Core Concepts

- **Frontend**: A remote service client (e.g., Azure DevOps, GitHub, Local Diff).
- **Resolver**: The logic that finalizes an intermediate match (e.g., Naive or Local resolution).
- **Engine**: The core orchestrator that manages parallel/sequential execution, result merging, and local file enrichment.
- **Double-Sort Strategy**: The engine sorts all collected results by **filename** to perform efficient sequential local resolution, then re-sorts them to match the **original provider order** for the user.

## Settings Configuration (`restgrep.json`)

| Field | Type | Description |
| :--- | :--- | :--- |
| `backends` | Array | List of search frontends to use. |
| `execution_mode` | String | `parallel` (default) or `sequential`. |
| `backend_mode` | String | `naive` or `local` (default). |
| `merge_base_branch` | String | Global default branch for `local-diff-add`. |

### Execution Modes

1.  **Parallel Mode (`parallel`)**:
    - Executes all backends simultaneously.
    - Merges results from all successful backends.
    - **Ignores failed backends** (errors are reported to `stderr`).

2.  **Sequential Fallback Mode (`sequential`)**:
    - Executes backends one-by-one in order.
    - **Stops after the first successful execution**.

### Resolver Modes

1.  **Naive Mode (`naive`)**:
    - Directly prints the remote match fragment returned by the API.
    - No local validation or line number correction.

2.  **Local Mode (`local`)**:
    - Smarter, fallback-based resolution.
    - Performs a substring search for the query pattern in your local working copy.
    - Resolves real line numbers even if the file has drifted.
    - Adds `(relaxed match)` if SHA1 doesn't match but pattern is found.

---

## Supported Frontends

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

### 4. Local Diff Add (`local-diff-add`)
- **Mechanism**: Uses `go-git` to find all newly added lines since the `merge-base` of your current branch and a target branch (e.g. `origin/main`).
- **Use Case**: Find patterns only in your recent local changes before committing.

### Frontend Config Fields

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `type` | String | Yes | `azure`, `github`, `github-api`, or `local-diff-add`. |
| `organization` | String | Azure only | Azure DevOps organization name. |
| `project` | String | Azure only | Azure DevOps project name. |
| `repo` | String | GH only | Target repository (e.g., `chromium/chromium`). |
| `limit` | Integer | No | Result limit for this backend (default: 100). |
| `backend_mode` | String | No | Override resolver mode for this frontend. |
| `merge_base_branch` | String | No | Branch for `local-diff-add` (default: `origin/main`). |

---

## Sample Full Configuration

```json
{
  "execution_mode": "parallel",
  "merge_base_branch": "origin/main",
  "backends": [
    {
      "type": "local-diff-add",
      "merge_base_branch": "origin/main"
    },
    {
      "type": "azure",
      "organization": "Initech",
      "project": "CoverReportTemplates",
      "limit": 100,
      "backend_mode": "local"
    },
    {
      "type": "github-api",
      "repo": "chromium/chromium",
      "limit": 50,
      "backend_mode": "naive"
    }
  ]
}
```
