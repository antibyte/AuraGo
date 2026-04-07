# JSON Editor (`json_editor`)

Structured JSON file operations using dot-path notation. Read, modify, and validate JSON files without loading or rewriting them manually.

## Operations

| Operation | Description |
|-----------|-------------|
| `get` | Read a value at a JSON path |
| `set` | Set/create a value at a JSON path |
| `delete` | Remove a key at a JSON path |
| `keys` | List keys at a JSON path (or root) |
| `validate` | Check if file contains valid JSON |
| `format` | Pretty-print/reformat the JSON file |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of: get, set, delete, keys, validate, format |
| `file_path` | string | yes | Path to the JSON file |
| `json_path` | string | for get, set, delete, keys | Dot-separated path (e.g., `server.port`, `users.0.name`) |
| `set_value` | string | for set | Value to set (any JSON type) |

## Path Syntax

Use dot-separated paths to navigate nested structures:
- `server.port` → `{"server": {"port": 8080}}`
- `users.0.name` → First element of `users` array, `name` field
- `database.credentials.password` → Deep nesting

## Key Behaviors

- **get**: Returns the raw JSON value at the path. Returns error if path not found.
- **set**: Creates intermediate objects if they don't exist. Creates the file if it doesn't exist. Always pretty-prints the result.
- **delete**: Removes the key-value pair. Fails if path doesn't exist.
- **keys**: Lists all keys at the given path. If no path given, lists root keys.
- **validate**: Returns `{status: ok, data: true/false}` — never errors on invalid JSON.
- **format**: Re-indents the file with 2-space indentation. Fails if JSON is invalid.

## Examples

**Read a config value:**
```json
{"action": "json_editor", "operation": "get", "file_path": "config.json", "json_path": "server.port"}
```

**Set a nested value (creates intermediaries):**
```json
{"action": "json_editor", "operation": "set", "file_path": "package.json", "json_path": "scripts.build", "set_value": "next build"}
```

**Delete a key:**
```json
{"action": "json_editor", "operation": "delete", "file_path": "tsconfig.json", "json_path": "compilerOptions.strict"}
```

**List root keys:**
```json
{"action": "json_editor", "operation": "keys", "file_path": "package.json"}
```

**Validate JSON syntax:**
```json
{"action": "json_editor", "operation": "validate", "file_path": "data.json"}
```

**Reformat messy JSON:**
```json
{"action": "json_editor", "operation": "format", "file_path": "data.json"}
```

## Notes

- Use `json_editor` instead of read_file + write_file for JSON config changes — it's safer and preserves formatting.
- For bulk reads, use `keys` first to discover the structure, then `get` for specific values.
- `set_value` accepts any JSON type: strings, numbers, booleans, arrays, objects, null.
- Works across all contexts: local workspace, remote devices, homepage containers, and SSH.
