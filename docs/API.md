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
| `inexact_sha1_adjustment` | Boolean | If `true`, enables automatic Git-diff adjustment if local files have drifted. |

### Execution Modes

1.  **Parallel Mode (`parallel`)**:
    - Executes all backends simultaneously.
    - Merges results from all successful backends.
    - **Ignores failed backends** (errors are reported to `stderr`).

2.  **Sequential Fallback Mode (`sequential`)**:
    - Executes backends one-by-one in order.
    - **Stops after the first successful execution**.

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
  "inexact_sha1_adjustment": true,
  "backends": [
    {
      "type": "azure",
      "organization": "Initech",
      "project": "CoverReportTemplates",
      "limit": 100
    },
    {
      "type": "github-api",
      "repo": "chromium/chromium",
      "limit": 50
    }
  ]
}
```
