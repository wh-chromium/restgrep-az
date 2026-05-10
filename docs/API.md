# Restgrep API Documentation

`restgrep` is a normalization layer for various remote code search APIs (currently supporting Azure DevOps). It provides a standard `grep`-like interface that delegates queries to remote APIs, optimizing token usage for AI agents.

## Core Concepts

- **Backend**: A remote service providing search capabilities (e.g., Azure DevOps). Backends implement a common interface.
- **Engine**: The core component that parses generic `grep` arguments, translates them to backend-specific queries, and formats the output back to `grep`-like strings.
- **Configuration**: `restgrep` uses a configuration file (e.g., `restgrep.json`) to define project-specific backend settings.

## Extensibility

To add a new backend (e.g., GitHub CLI), implement the `backend.Backend` interface:

```go
type Backend interface {
	// Name returns the name of the backend.
	Name() string
	// Search executes a search using the provided query and options.
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
}
```

## Supported Backends

`restgrep` currently natively supports multiple code-search remote APIs:
- Azure DevOps (via `az` CLI token and direct REST API)
- GitHub (via `gh` CLI search command)
- GitHub API (direct Search API via `gh api` with text-match support)

### Azure DevOps Backend

The Azure DevOps backend interacts with the Azure DevOps REST API (v7.1).
It simulates authentication using an Access Token fetched typically via `az account get-access-token` (or `az rest`).
When `az rest` fails, it propagates the error.

### GitHub Backend

The GitHub backend natively executes the `gh search code` CLI command and parses the returned JSON payload (`--json path,textMatches`). It allows you to specify a repository constraint (e.g. `owner/repo`) via the settings configuration.

### GitHub API Backend

The GitHub API backend uses `gh api` to call the `/search/code` endpoint directly. It includes the `application/vnd.github.v3.text-match+json` header to retrieve detailed match fragments and indices.

### Settings Configuration

The settings file (`restgrep.json`) configures the backends and the execution strategy.

| Field | Type | Description |
| :--- | :--- | :--- |
| `backends` | Array | List of search backends to use. |
| `execution_mode` | String | `parallel` (default) or `sequential`. |
| `inexact_sha1_adjustment` | Boolean | If `true`, enables automatic Git-diff adjustment by default. |

#### Execution Modes

1.  **Parallel Mode (`parallel`)**:
    - Executes all backends simultaneously.
    - Waits for all backends to return results.
    - Merges results from all successful backends.
    - **Ignores failed backends** (e.g., if one API is down, you still get results from others).
    - **Optimization**: Performs local file enrichment (MRU cache) **after** all remote results are collected.

2.  **Sequential Fallback Mode (`sequential`)**:
    - Executes backends one-by-one in the order they are listed.
    - **Stops after the first successful execution** (even if results are empty).
    - Useful for prioritizing a fast internal API and falling back to a broader public API only if necessary.

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
      "type": "github",
      "repo": "wh-chromium/restgrep-az",
      "limit": 50
    },
    {
      "type": "github-api",
      "repo": "chromium/chromium",
      "limit": 20
    }
  ]
}
```
