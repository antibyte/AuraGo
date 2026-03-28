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
| `move` | Move or rename | `destination` (string) |
| `stat` | Get file metadata | — |

All operations require `file_path` (relative to `workdir/`).

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
{"action": "filesystem", "operation": "move", "file_path": "old_name.txt", "destination": "new_name.txt"}
```

```json
{"action": "filesystem", "operation": "stat", "file_path": "somefile.pdf"}
```
