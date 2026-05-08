# Restgrep: Path Filtering Examples

`restgrep` supports standard `grep` syntax for filtering searches to specific directories or files.

## Basic Usage

Search for a pattern within a specific subdirectory:

```bash
restgrep "Omnibox" chrome/browser
```

### GitHub API Backend
If using the `github-api` backend, this translates to:
`gh api "/search/code?q=Omnibox repo:owner/repo path:chrome/browser"`

### Azure DevOps Backend
If using the `azure` backend, this translates to a filter:
`"Path": ["/chrome/browser"]`

## Multiple Paths

You can specify multiple paths to search in multiple locations simultaneously (mirroring standard `grep` behavior):

```bash
restgrep "RenderFrameHost" content/browser content/public
```

## Mixing with Flags

Path filtering works seamlessly with all other flags:

```bash
restgrep -i -n "NavigationHandle" chrome/browser
```
