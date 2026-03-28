## Tool: File Editor (`file_editor`)

Precisely edit text files with targeted operations. Safer than `write_file` for modifications because it validates matches and operates atomically.

### Operations

| Operation | Description | Required Parameters |
|---|---|---|
| `str_replace` | Replace exact text (must match uniquely) | `old`, `new` |
| `str_replace_all` | Replace all occurrences | `old`, `new` |
| `insert_after` | Insert content after an anchor line | `marker`, `content` |
| `insert_before` | Insert content before an anchor line | `marker`, `content` |
| `append` | Append content to end of file | `content` |
| `prepend` | Prepend content to beginning of file | `content` |
| `delete_lines` | Delete a range of lines (1-based, inclusive) | `start_line`, `end_line` |

All operations require `file_path` (relative to `agent_workspace/workdir`). Project-root files are reachable via `../../`.

### Key Behaviors

- **`str_replace`** fails if the `old` text appears 0 or more than 1 times — provide enough context to make the match unique.
- **`insert_after` / `insert_before`** fail if the `marker` text appears on 0 or more than 1 lines.
- **`append`** creates the file if it doesn't exist.
- All writes are **atomic** (temp file + rename) to prevent data corruption.
- Do not use `file_editor` for homepage projects; use the `homepage` tool's own edit operations instead.

### Examples

```json
{"action": "file_editor", "operation": "str_replace", "file_path": "../../config.yaml", "old": "enabled: false", "new": "enabled: true"}
```

```json
{"action": "file_editor", "operation": "insert_after", "file_path": "requirements.txt", "marker": "flask==", "content": "redis==5.0.0"}
```

```json
{"action": "file_editor", "operation": "append", "file_path": "log.txt", "content": "2025-01-15: Task completed"}
```

```json
{"action": "file_editor", "operation": "delete_lines", "file_path": "data.csv", "start_line": 5, "end_line": 10}
```

### Tips

- To replace a multi-line block, include all lines in `old` with `\n` separators.
- Prefer `str_replace` over `write_file` for surgical edits — it's safer and preserves surrounding content.
- Use `insert_after` to add imports, config entries, or list items at a specific position.
