# Restgrep Examples

`restgrep` provides a CLI experience similar to `grep`, but executes searches remotely across Azure DevOps and GitHub.

## Quick Links

- [Azure DevOps Specific Examples](EXAMPLES_AZURE.md)
- [Chromium / GitHub Specific Examples](EXAMPLES_CHROMIUM.md)
- [Query Matching Behavior & Wildcards](MATCHING_BEHAVIOR.md)

## Supported `grep` flags

`restgrep` currently supports these flags by translating them into backend-specific filters where applicable, or by processing the output.

- `-i`, `--ignore-case`: Ignore case distinctions.
- `-n`, `--line-number`: Prefix each line of output with the 1-based line number.
- `-l`, `--files-with-matches`: Print only the name of each input file.
- `-c`, `--count`: Print a count of matching lines for each input file.
- `-w`, `--word-regexp`: Force PATTERN to match only whole words.
- `-m`, `--max-count`: Stop after NUM matches (defaults to 100 or as configured in `restgrep.json`).

## Configuration (`restgrep.json`)

Configure your search targets:

```json
{
  "backends": [
    {
      "type": "azure",
      "organization": "fabrikam",
      "project": "MyFirstProject"
    },
    {
      "type": "github",
      "repo": "chromium/chromium"
    }
  ]
}
```
