# Tool: `archive`

## Purpose
Create, extract, or list ZIP and TAR.GZ/TGZ archives. Uses Go's standard library — no external dependencies.

## When to Use
- Bundle multiple files into a single archive for transfer or backup.
- Extract received archives to inspect or process their contents.
- List archive contents without extracting to check what's inside.

## Parameters
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | ✅ | `create`, `extract`, or `list` |
| `path` | string | ✅ | Path to the archive file (target for create, source for extract/list) |
| `destination` | string | ❌ | Target directory (extraction dest or source dir for create) |
| `source_files` | string | ❌ | JSON array of specific file paths to include (create only; alternative to destination) |
| `format` | string | ❌ | `zip` or `tar.gz` (create only; extract/list auto-detect from extension) |

## Operations

### `create`
Builds an archive from the files in `destination` directory or from explicit `source_files` list.

### `extract`
Extracts the archive at `path` into the `destination` directory. Path traversal attacks are blocked.

### `list`
Lists all entries in the archive without extracting.

## Output
JSON with: `status`, `message`, `files` (array of filenames), `total` (count).

## Security
- Extraction enforces path traversal protection — zip slip attacks are blocked.
- Total uncompressed data is capped at 1 GB per extraction.
- Write operations require `allow_filesystem_write` to be enabled.

## Example Calls
```json
{ "operation": "list", "path": "backup.zip" }
{ "operation": "extract", "path": "data.tar.gz", "destination": "output/" }
{ "operation": "create", "path": "bundle.zip", "destination": "my_project/", "format": "zip" }
{ "operation": "create", "path": "selected.tar.gz", "source_files": "[\"file1.txt\", \"file2.txt\"]" }
```
