# Restgrep: Azure DevOps Examples (Simulated)

This file documents examples and patterns specifically for the Azure DevOps backend. 

*Note: The outputs below are **simulated** based on the Azure DevOps REST API v7.1 specifications.*

## Basic Usage

Search for a string in the configured Azure DevOps project:

```bash
restgrep "CodeSearchController"
```

### Remote Stub Output (Local file NOT found)

If the file is not checked out locally, `restgrep` provides the remote match metadata:

**Output:**
```text
/CodeSearchController.cs:[Match at char offset 1187, length 20] (local file not found)
```

### Local File Resolution (SHA1 Validation)

If you have the repository checked out locally, `restgrep` will automatically resolve the match to the actual source line by validating the Git blob SHA1 (`contentId`).

**If local file exists and SHA1 matches:**
```bash
restgrep "CodeSearchController"
```
**Output:**
```text
src/controllers/CodeSearchController.cs:12:public class CodeSearchController : Controller {
```

## Supported Flags (Simulated)

### Case-Insensitive Search (`-i`)

```bash
restgrep -i "codesearchcontroller"
```
**Output:**
```text
/CodeSearchController.cs:[Match at char offset 0, length 20] (local file not found)
```

### Showing Match Counts per File (`-c`)

```bash
restgrep -c "Controller"
```
**Output:**
```text
/CodeSearchController.cs:3
/AuthController.cs:5
/BaseController.cs:1
```

### Showing Only Filenames (`-l`)

```bash
restgrep -l "factory"
```
**Output:**
```text
/src/factories/ControllerFactory.cs
/src/factories/RepositoryFactory.cs
```

### Word Matching (`-w`)

Matches only whole words, avoiding substring partials.

```bash
restgrep -w "Base"
```
**Output:**
```text
/src/core/Base.cs:[Match at char offset 45, length 4] (local file not found)
```
*(Note: This skips matches like `BaseController` or `Database`).*

### Showing Line Numbers (`-n`)

When resolved locally, line numbers are accurate. For remote stubs, Azure often defaults to `1` or provides character offsets.

```bash
restgrep -n "CodeSearchController"
```
**Output:**
```text
/CodeSearchController.cs:1:[Match at char offset 1187, length 20] (local file not found)
```
