# restgrep

A token-efficient normalization layer for Azure DevOps REST API and more.

## Installation

(Coming soon)

## Usage Examples

### Search like grep
```bash
restgrep "CodeSearchController"
```

Output:
```
[azure] /CodeSearchController.cs:0:Match at offset 1187, length 20
[azure] /CodeSearchController.cs:0:Match at offset 1395, length 20
```

### Search with max count
```bash
restgrep -m 1 "CodeSearchController"
```

### Search across multiple backends (Future)
When configured with both Azure and GitHub:
```bash
restgrep "pattern"
```
Output:
```
[azure] /src/main.cs:10:found pattern
[github] /pkg/lib.go:42:found pattern
```

## Configuration

Create a `restgrep.yaml`:
```yaml
azure:
  organization: "your-org"
  project: "your-project"
```

## Authentication

Requires `az` CLI to be logged in. `restgrep` will automatically grab the token using:
```bash
az account get-access-token --resource 499b84ac-1321-427f-aa17-267ca6975798
```
