## Tool: File Editor (`file_editor`)

Precisely edit text files with targeted operations. Safer than `write_file` for modifications because it validates matches and operates atomically.

### Operations

| Operation | Description | Required Parameters |
|---|---|---|
| `str_replace` | Replace exact text (must match uniquely) | `old`, `new` |
| `str_replace_all` | Replace all occurrences | `old`, `new` |
| `str_replace_regex` | Replace using a Go regex (supports capture groups $1, $2…) | `old` (regex), `new` |
| `str_replace_glob` | Replace literal text across all files matching a glob | `file_path` (glob), `old`, `new` |
| `insert_after` | Insert content after an anchor line | `marker`, `content` |
| `insert_before` | Insert content before an anchor line | `marker`, `content` |
| `append` | Append content to end of file | `content` |
| `prepend` | Prepend content to beginning of file | `content` |
| `delete_lines` | Delete a range of lines (1-based, inclusive) | `start_line`, `end_line` |
| `hashline_replace` | Replace text anchored to a hashed line | `old`, `new`, `anchor_line`, `anchor_hash` |
| `hashline_insert_after` | Insert content after a hashed anchor line | `marker`, `content`, `anchor_line`, `anchor_hash` |
| `hashline_insert_before` | Insert content before a hashed anchor line | `marker`, `content`, `anchor_line`, `anchor_hash` |
| `hashline_delete` | Delete a line range anchored to a hashed line inside that range | `start_line`, `end_line`, `anchor_line`, `anchor_hash` |

All operations require `file_path` (relative to `agent_workspace/workdir`). Project-root files are reachable via `../../`.

### Key Behaviors

- **`str_replace`** fails if the `old` text appears 0 or more than 1 times — provide enough context to make the match unique.
- **`str_replace_regex`** uses Go regex syntax. `old` is the pattern; `new` is the replacement (supports `$1`, `$2` capture groups). Fails if pattern matches 0 times.
- **`str_replace_glob`** replaces literal `old` with `new` in every file matching the glob. `file_path` is the glob (e.g. `"../../src/*.go"`). Reports count per file. Does NOT require unique matches. Skips files over 10 MB. Note: Go's stdlib glob does not support `**` — use explicit paths or `*.ext` patterns.
- **`insert_after` / `insert_before`** fail if the `marker` text appears on 0 or more than 1 lines.
- **`append`** creates the file if it doesn't exist.
- **Hashline operations** require a recent `filesystem` `read_file` call with `include_hashes: true`; if the anchor content changed, they fail with `STALE CONTEXT` and you must re-read.
- All writes are **atomic** (temp file + rename) to prevent data corruption.
- Do not use `file_editor` for homepage projects; use `homepage_file` edit operations instead.
- Do not use `file_editor` for Virtual Desktop files such as `Apps/...` or `Widgets/...`; those live in `virtual_desktop.workspace_dir`, not `agent_workspace/workdir`. Use `virtual_desktop_files`, `virtual_desktop_apps`, or `virtual_desktop_widgets` instead.

### Hashline Mode

Hashline mode protects edits from stale context by validating that the anchor line's content has not changed since you read it.

**Key principle:** The hash is computed from the **line content only**, not the line number. This means:
- If you insert/delete lines **above** a target line, the target's **line number shifts**, but its **content hash stays valid**.
- You can perform **multiple edits in the same file without re-reading**, as long as you adjust `anchor_line` for shifted lines and the target content has not changed.
- You only need to re-read if the **content of the target anchor line itself changed**.

**Workflow:**

1. Read the file with hashes:

```json
{"action": "filesystem", "operation": "read_file", "file_path": "../../internal/tools/example.go", "include_hashes": true}
```

2. Use the emitted `LINE#HASH:CONTENT` prefix as the edit anchor:

```text
42#a1b2c3d4:func main() {
```

`42` is the line number. `a1b2c3d4` is an 8-character hash of the line content only.

3. Edit with a hashline operation:

```json
{"action": "file_editor", "operation": "hashline_replace", "file_path": "../../internal/tools/example.go", "old": "func main() {", "new": "func main() error {", "anchor_line": 42, "anchor_hash": "a1b2c3d4"}
```

**Rules:**

- Always provide `anchor_line` and `anchor_hash` from the hashline read.
- `hashline_replace` replaces an `old` match that **starts on the validated anchor line**. Matches elsewhere in the file do not matter.
- `hashline_insert_after` and `hashline_insert_before` insert relative to the validated anchor line; `marker` must appear on that line.
- `hashline_delete` requires `anchor_line` to be inside the deleted range.
- **Multi-edit without re-read is possible.** If you insert 2 lines above line 10, a later untouched line 15 becomes line 17 — use `anchor_line: 17` but keep the **original `anchor_hash`** from the read.
- On `STALE CONTEXT`, the anchor line's content changed. Re-read the file with `include_hashes: true` and retry.

**Multi-Edit Example (no re-read needed):**

```json
// Step 1: Read with hashes
{"action": "filesystem", "operation": "read_file", "file_path": "main.go", "include_hashes": true}
// → 1#aaa111:package main
// → 3#ccc333:func main() {

// Step 2: Insert after line 1
{"action": "file_editor", "operation": "hashline_insert_after", "file_path": "main.go", "marker": "package main", "content": "import \"fmt\"", "anchor_line": 1, "anchor_hash": "aaa111"}

// Step 3: Replace line 4 (was line 3 before insert) — same original hash!
{"action": "file_editor", "operation": "hashline_replace", "file_path": "main.go", "old": "func main() {", "new": "func main() error {", "anchor_line": 4, "anchor_hash": "ccc333"}
```

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

```json
{"action": "file_editor", "operation": "str_replace_regex", "file_path": "../../config.yaml", "old": "version: \\d+", "new": "version: 42"}
```

```json
{"action": "file_editor", "operation": "str_replace_glob", "file_path": "../../internal/tools/*.go", "old": "oldFunctionName", "new": "newFunctionName"}
```

```json
{"action": "file_editor", "operation": "hashline_delete", "file_path": "data.csv", "start_line": 5, "end_line": 10, "anchor_line": 5, "anchor_hash": "a1b2c3d4"}
```

### Tips

- To replace a multi-line block, include all lines in `old` with `\n` separators.
- Prefer `str_replace` over `write_file` for surgical edits — it's safer and preserves surrounding content.
- Use `insert_after` to add imports, config entries, or list items at a specific position.
