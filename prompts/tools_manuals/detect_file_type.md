# Tool: `detect_file_type`

## Purpose
Identify the **true** file type of one or more files using magic-byte analysis (reads the file header, ignores the file extension). Returns MIME type, canonical extension, and type group for each file.

## When to Use
- A file has the wrong or missing extension and you need to know its actual format.
- Scanning a directory to categorise media, documents, or binary files.
- Verifying that an uploaded or downloaded file is what it claims to be.
- Pre-processing files for conversion, extraction, or analysis.

## Parameters
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | ✅ | Absolute or relative path to a file or directory |
| `recursive` | boolean | ❌ | Recurse into sub-directories when `path` is a directory (default: `false`) |

## Output
JSON with these fields:
- `status` — `"success"` or `"error"`
- `total` — total number of files processed
- `errors` — number of files that could not be read
- `files` — array of entries:
  - `path` — path to the file
  - `mime` — detected MIME type (e.g. `"image/jpeg"`, `"video/mp4"`)
  - `extension` — canonical extension without dot (e.g. `"jpg"`, `"mp4"`)
  - `group` — broad type category (e.g. `"image"`, `"video"`, `"audio"`, `"application"`)
  - `error` — error message for this file (only present on failure)

## Behaviour Notes
- Files whose type cannot be detected get `mime: "application/octet-stream"` and `group: "unknown"`.
- Only the first 261 bytes of each file are read (sufficient for all matchers).
- Symlinks are followed.
- This tool is read-only and always enabled.

## Example Calls
```json
{ "path": "agent_workspace/workdir/download.bin" }
{ "path": "/data/uploads", "recursive": true }
{ "path": "agent_workspace/workdir/suspicious_file.jpg" }
```
