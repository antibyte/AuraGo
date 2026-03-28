## Tool: File Reader Advanced (`file_reader_advanced`)

Read files with fine-grained control: paginated line ranges, head/tail, line counting, and contextual search. Ideal for exploring large files without reading them entirely.

### Operations

| Operation | Description | Required Parameters |
|---|---|---|
| `read_lines` | Read a specific line range (1-based, inclusive) | `file_path`, `start_line`, `end_line` |
| `head` | Read the first N lines (default 20) | `file_path` |
| `tail` | Read the last N lines (default 20) | `file_path` |
| `count_lines` | Count total lines in a file | `file_path` |
| `search_context` | Find pattern matches with surrounding context lines | `file_path`, `pattern` |

All operations require `file_path` (alias `path`). Paths are resolved from `agent_workspace/workdir`, and project-root files are reachable via `../../`.

### Parameters

| Parameter | Description |
|---|---|
| `file_path` | File path relative to `agent_workspace/workdir` |
| `start_line` | First line to read (1-based, for `read_lines`) |
| `end_line` | Last line to read (inclusive, for `read_lines`) |
| `line_count` | Number of lines for `head`/`tail` (default 20); context lines for `search_context` (default 3) |
| `pattern` | Regex pattern for `search_context` |

### Key Behaviors

- **`read_lines`** returns lines with their line numbers. `start_line` and `end_line` are clamped to file bounds.
- **`head`** / **`tail`** default to 20 lines if `line_count` is not specified.
- **`count_lines`** returns the total number of lines and file size in bytes.
- **`search_context`** returns each match with `line_count` surrounding lines of context (default 3). Maximum 50 match groups.
- All paths are sandboxed to the workspace directory rooted at `agent_workspace/workdir`.
- For homepage projects, use the `homepage` tool instead of the generic file tools.

### Examples

```json
{"action": "file_reader_advanced", "operation": "read_lines", "file_path": "../../config.yaml", "start_line": 50, "end_line": 75}
```

```json
{"action": "file_reader_advanced", "operation": "head", "file_path": "server.log", "line_count": 50}
```

```json
{"action": "file_reader_advanced", "operation": "tail", "file_path": "server.log"}
```

```json
{"action": "file_reader_advanced", "operation": "count_lines", "file_path": "data.csv"}
```

```json
{"action": "file_reader_advanced", "operation": "search_context", "file_path": "../../cmd/aurago/main.go", "pattern": "func main", "line_count": 5}
```

### Tips

- Use `count_lines` first to understand file size before reading specific ranges.
- Combine `count_lines` → `read_lines` for paginated browsing of large files.
- Use `search_context` instead of `grep` when you need to see surrounding code, not just matching lines.
- `tail` is useful for checking recent log entries.
- Use `smart_file_read analyze` or `smart_file_read summarize` first when the file is very large and you need an overview before zooming into line ranges.
