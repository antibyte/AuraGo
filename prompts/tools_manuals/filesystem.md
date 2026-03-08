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