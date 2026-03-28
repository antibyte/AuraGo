## Tool: File Search (`file_search`)

Search for text patterns inside files or find files by name within the workspace. Results are returned as structured JSON.

### Operations

| Operation | Description | Required Parameters |
|---|---|---|
| `grep` | Search a single file for a regex pattern | `pattern`, `file_path` |
| `grep_recursive` | Search files matching a glob for a regex pattern | `pattern`, `glob` |
| `find` | Find files by name matching a glob pattern | `glob` |

### Parameters

| Parameter | Description |
|---|---|
| `pattern` | Regex pattern to search for (grep/grep_recursive) |
| `file_path` | File path relative to `agent_workspace/workdir` (alias `path`) |
| `glob` | One or more glob patterns. Match against relative paths for recursive operations. Separate multiple globs with commas, e.g. `**/*.go,**/*.md`. |
| `output_mode` | Optional. `count` returns only match count instead of full results |

### Key Behaviors

- **`grep`** returns line numbers and matching content for each hit.
- **`grep_recursive`** searches across all files matching the glob(s). Globs are evaluated against the relative path. Skips `.git/`, `node_modules/`, `__pycache__/`, `venv/`, and files larger than 10 MB.
- **`find`** returns file paths matching the glob pattern (max 1000 results).
- All paths are sandboxed to `agent_workspace/workdir`, with project-root files reachable via `../../`.
- Maximum 500 matches for grep operations.

### Examples

```json
{"action": "file_search", "operation": "grep", "pattern": "TODO|FIXME", "file_path": "../../cmd/aurago/main.go"}
```

```json
{"action": "file_search", "operation": "grep_recursive", "pattern": "func Test", "glob": "**/*_test.go"}
```

```json
{"action": "file_search", "operation": "grep_recursive", "pattern": "password", "glob": "**/*.yaml,**/*.yml", "output_mode": "count"}
```

```json
{"action": "file_search", "operation": "find", "glob": "**/*.json"}
```

### Tips

- Use `grep` for targeted searches in a known file; use `grep_recursive` to search across many files.
- Use `output_mode: "count"` when you only need to know how many matches exist.
- Patterns use Go regex syntax (similar to PCRE without lookaheads).
