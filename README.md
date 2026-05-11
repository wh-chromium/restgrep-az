# restgrep

A token-efficient normalization layer for remote code search APIs (Azure DevOps and GitHub), providing a standard `grep`-like interface for AI agents and developers.

## Features

- **Multi-Backend**: Search across Azure DevOps, GitHub, and **Local Git Diffs** simultaneously.
- **Grep Parity**: Supports standard flags like `-i`, `-n`, `-c`, `-l`, `-w`, and `-m`.
- **Context Control**: Supports `-A`, `-B`, and `-C` using local resolution.
- **Dynamic Execution**: Choose between `parallel` (simultaneous) or `sequential` (fallback) search modes.
- **Relaxed Local Resolution**: Automatically resolves remote match stubs to actual source lines, with a robust fallback search if file versions have drifted.
- **High Performance**: Uses a Single-File MRU cache and global filename sorting to ensure each local file is read/hashed exactly once.

## Installation & Building

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
  "execution_mode": "parallel",
  "backends": [
    {
      "type": "azure",
      "organization": "your-org",
      "project": "your-project",
      "limit": 100
    },
    {
      "type": "local-diff-add",
      "merge_base_branch": "origin/main"
    }
  ]
}
```

## Usage Examples

### Basic Substring Search
```bash
restgrep "WebContentsImpl"
```

### Context Search (Surrounding Lines)
```bash
restgrep -C 2 "Omnibox"
```

### Path Filtering
```bash
restgrep "pattern" chrome/browser content/public
```

For more details, see [docs/EXAMPLES.md](docs/EXAMPLES.md).

## Documentation

- [API Specification & Configuration](docs/API.md)
- [Query Matching Behavior & Wildcards](docs/MATCHING_BEHAVIOR.md)
- [Path Filtering Examples](docs/EXAMPLES_PATHS.md)
- [Limitations](docs/LIMITATIONS.md)
- [Guide for AI Agents (Extending restgrep)](docs/FOR_AGENTS.md)

## License

BSD 3-Clause License. See [LICENSE](LICENSE) for details.
