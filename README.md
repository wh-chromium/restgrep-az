# restgrep

A token-efficient normalization layer for remote code search APIs (Azure DevOps and GitHub), providing a standard `grep`-like interface for AI agents and developers.

## Features

- **Multi-Backend**: Search across Azure DevOps and GitHub (CLI or API) simultaneously.
- **Grep Parity**: Supports standard flags like `-i`, `-n`, `-c`, `-l`, `-w`, and `-m`.
- **Local Resolution**: Automatically resolves remote match stubs to actual source lines by validating local files against Git blob SHA1 (`ContentId`).
- **Token Efficient**: Optimized for AI agents by providing concise, normalized output.

## Installation & Building

### From Source

Requires Go 1.26 or later.

```bash
# Clone the repository
git clone https://github.com/wh-chromium/restgrep-az.git
cd restgrep-az

# Build the executable
go build -o restgrep.exe cmd/restgrep/main.go
```

## Configuration

`restgrep` looks for a `restgrep.json` in the current directory.

```json
{
  "backends": [
    {
      "type": "azure",
      "organization": "your-org",
      "project": "your-project",
      "limit": 100
    },
    {
      "type": "github-api",
      "repo": "chromium/chromium",
      "limit": 10
    }
  ]
}
```

## Usage Examples

### Basic Substring Search
```bash
restgrep "WebContentsImpl"
```

### Exact Word Match
```bash
restgrep -w "WebContents"
```

### With Max Results (Limit)
```bash
restgrep -m 5 "NavigationHandle"
```

For more details, see [docs/EXAMPLES.md](docs/EXAMPLES.md).

## Authentication

- **Azure**: Requires `az` CLI to be logged in. `restgrep` fetches tokens dynamically.
- **GitHub**: Requires `gh` CLI to be logged in and authenticated.

## Documentation

- [API Specification](docs/API.md)
- [Matching Behavior & Wildcards](docs/MATCHING_BEHAVIOR.md)
- [Limitations](docs/LIMITATIONS.md)
- [Guide for AI Agents (Extending restgrep)](docs/FOR_AGENTS.md)

## License

BSD 3-Clause License. See [LICENSE](LICENSE) for details.
