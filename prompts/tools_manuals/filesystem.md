## Tool: Filesystem Operations (`filesystem`)

Perform file system tasks. Your working directory is `agent_workspace/workdir`. The project root containing `documentation/` and `config.yaml` is two levels up (`../../`). 

### Operations

| Operation | Description | Extra Parameters |
|---|---|---|
| `list_dir` | List directory contents | — |
| `create_dir` | Create a directory | — |
| `read_file` | Read file contents | — |
| `write_file` | Write content to file | `content` (string) |
| `delete` | Delete a file or directory | — |
| `copy` | Copy a single file | `destination` (string) |
| `move` | Move or rename | `destination` (string) |
| `copy_batch` | Copy multiple files | `items` (array with `file_path` + `destination`) |
| `move_batch` | Move multiple files | `items` (array with `file_path` + `destination`) |
| `delete_batch` | Delete multiple files/directories | `items` (array with `file_path`) |
| `create_dir_batch` | Create multiple directories | `items` (array with `file_path`) |
| `stat` | Get file metadata | — |

Single-path operations require `file_path` (relative to `workdir/`).
Batch operations require `items`.

### Batch Behavior

- Batch operations return `success`, `partial`, or `error`.
- `partial` means some items succeeded and some failed.
- Each batch response includes a summary plus per-item results, so use it when you want to continue despite partial failures.

### Large Files

- `read_file` is a convenience read, not a large-file analysis tool.
- Large text files are truncated after a preview window.
- For logs, long code files, CSVs, or any file that does not fit comfortably in one prompt:
  use `smart_file_read` for `analyze`, `sample`, `structure`, or `summarize`.
  use `file_reader_advanced` for `head`, `tail`, `read_lines`, or `search_context`.

### Common Pitfalls

- Use the exact operation names `read_file` and `write_file`.
- Do not use shorthand names like `read` or `write` in tool calls.
- If you need to work on homepage projects, do not use `filesystem` at all. Use the `homepage` tool's own `read_file` / `write_file` operations so files end up in the homepage workspace.

### Examples

```json
{"action": "filesystem", "operation": "list_dir", "file_path": "."}
```

```json
{"action": "filesystem", "operation": "create_dir", "file_path": "my_project/data"}
```

```json
{"action": "filesystem", "operation": "read_file", "file_path": "notes.txt"}
```

```json
{"action": "smart_file_read", "operation": "analyze", "file_path": "server.log"}
```

```json
{"action": "filesystem", "operation": "write_file", "file_path": "output.txt", "content": "Hello World"}
```

```json
{"action": "filesystem", "operation": "delete", "file_path": "temp_file.txt"}
```

```json
{"action": "filesystem", "operation": "copy", "file_path": "notes.txt", "destination": "backup/notes.txt"}
```

```json
{"action": "filesystem", "operation": "move", "file_path": "old_name.txt", "destination": "new_name.txt"}
```

```json
{"action": "filesystem", "operation": "stat", "file_path": "somefile.pdf"}
```

```json
{"action": "filesystem", "operation": "copy_batch", "items": [{"file_path": "a.txt", "destination": "backup/a.txt"}, {"file_path": "b.txt", "destination": "backup/b.txt"}]}
```

```json
{"action": "filesystem", "operation": "delete_batch", "items": [{"file_path": "tmp/a.log"}, {"file_path": "tmp/b.log"}]}
```
