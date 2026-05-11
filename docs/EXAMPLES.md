# Restgrep Usage Examples

`restgrep` provides a powerful CLI experience similar to `grep`, but executes searches remotely across Azure DevOps, GitHub, and local Git repositories simultaneously.

## Usage Syntax

```bash
restgrep [OPTIONS] PATTERN [PATH...]
```

## Basic Search

Search for a string across all configured backends:

```bash
restgrep "CodeSearchController"
```

### Local File Resolution (The Magic of `restgrep`)
If you run `restgrep` inside your local repository, it performs Git SHA1 validation to extract the *exact* real source line and context, even if the remote index only gives a metadata stub.

**Example Output (Resolved Locally):**
```text
src/controllers/CodeSearchController.cs:12:public class CodeSearchController : Controller {
```

**Example Output (File Not Found Locally):**
If the file isn't checked out, `restgrep` elegantly falls back to the remote stub:
```text
/CodeSearchController.cs:[Match at char offset 1187, length 20] (local file not found)
```

## Path Filtering

`restgrep` supports standard `grep` positional arguments for path filtering. 

Search for a pattern within a specific subdirectory:
```bash
restgrep "Omnibox" chrome/browser
```

You can specify multiple paths (searches in all of them):
```bash
restgrep "RenderFrameHost" content/browser content/public
```

## Supported `grep` Flags

`restgrep` translates these flags natively to the remote APIs where possible, or processes them locally.

### Context Control (`-A`, `-B`, `-C`)
Show surrounding lines of code (requires the file to be present locally).
```bash
restgrep -C 1 "CodeSearchController"
```
**Output:**
```text
src/controllers/CodeSearchController.cs-11-using System;
src/controllers/CodeSearchController.cs:12:public class CodeSearchController : Controller {
src/controllers/CodeSearchController.cs-13-    private readonly ISearchService _searchService;
```

### Case-Insensitive Search (`-i`)
```bash
restgrep -i "codesearchcontroller"
```

### Exact Word Matching (`-w`)
Matches only whole words, skipping partial substring matches (e.g., skips `BaseController` when searching for `Base`).
```bash
restgrep -w "Base"
```

### Showing Match Counts (`-c`)
Returns the number of matches found per file.
```bash
restgrep -c "Controller"
```
**Output:**
```text
/CodeSearchController.cs:3
/AuthController.cs:5
```

### Showing Only Filenames (`-l`)
Prints the names of files containing matches, without the matched content.
```bash
restgrep -l "factory"
```

### Limiting Results (`-m`)
Limit the maximum number of results retrieved per backend. Crucial for saving API tokens and processing time.
```bash
restgrep -m 5 "Factory"
```

---

## Further Reading
- [Chromium Real-World Search Examples](EXAMPLES_CHROMIUM.md)
- [Query Matching & Wildcard Behavior](MATCHING_BEHAVIOR.md)
