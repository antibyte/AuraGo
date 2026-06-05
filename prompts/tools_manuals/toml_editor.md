# toml_editor — TOML File Editor

Structured TOML file operations using dot-path notation. Read and modify TOML files (e.g., config files, Cargo.toml, pyproject.toml) while preserving structure.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `get` | Read a value at a TOML path | `file_path`, `json_path` or `toml_path` |
| `set` | Set/create a value at a TOML path | `file_path`, `json_path` or `toml_path`, `set_value` |
| `delete` | Remove a key at a TOML path | `file_path`, `json_path` or `toml_path` |
| `keys` | List keys at a TOML path (or root) | `file_path` (optional: `json_path` or `toml_path`) |
| `validate` | Check if file contains valid TOML | `file_path` |

## Path Syntax

Uses dot-path notation:
- `server.port` → `[server]\nport = 8080`
- `tool.poetry.dependencies` → Nested table lookup.

> **Note:** The tool accepts both `json_path` (for consistency across structured editors) and `toml_path` (TOML-specific alias).

## Key Behaviors

- **get**: Returns JSON representation of the TOML table/value.
- **set**: Updates the value. Creates tables if missing. 
- **delete**: Removes a key and writes the updated TOML back to disk.
- **keys**: Returns sorted table keys for the requested table or the document root.
- **validate**: Checks syntax correctness.
- **write operations**: `set` and `delete` require `agent.allow_filesystem_write`.

## Examples

```
# Read a config value
toml_editor(operation="get", file_path="config.toml", json_path="database.port")

# Set a nested value
toml_editor(operation="set", file_path="pyproject.toml", json_path="tool.poetry.version", set_value="1.2.0")

# Delete with the TOML-specific alias
toml_editor(operation="delete", file_path="Cargo.toml", toml_path="package.metadata.old")

# List root tables
toml_editor(operation="keys", file_path="Cargo.toml")

# Validate syntax
toml_editor(operation="validate", file_path="config.toml")
```

## Tips

- Use `toml_editor` instead of standard file writing to avoid accidentally causing syntax errors in TOML manifests.
- Accepts TOML-compatible `set_value` types (strings, numbers, booleans, arrays, and nested tables).
