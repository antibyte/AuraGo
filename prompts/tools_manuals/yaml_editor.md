# yaml_editor — YAML File Editor

Structured YAML file operations using dot-path notation. Read, modify, and validate YAML files while preserving comments and structure.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `get` | Read a value at a YAML path | `file_path`, `json_path` |
| `set` | Set/create a value at a YAML path | `file_path`, `json_path`, `set_value` |
| `delete` | Remove a key at a YAML path | `file_path`, `json_path` |
| `keys` | List keys at a YAML path (or root) | `file_path` (optional: `json_path`) |
| `validate` | Check if file contains valid YAML | `file_path` |

## Path Syntax

Uses the same dot-path notation as json_editor:
- `server.port` → `server:\n  port: 8080`
- `database.host` → Nested YAML mapping
- `logging.level` → Any depth

> **Note:** The parameter is named `json_path` for consistency across editors.

## Key Behaviors

- **get**: Returns the value at the path as JSON. Returns error if path not found.
- **set**: Creates intermediate mappings if they don't exist. Creates the file if it doesn't exist. Preserves existing YAML comments via Node tree editing.
- **delete**: Removes the key-value pair. Fails if path doesn't exist.
- **keys**: Lists all keys at the given path. If no path given, lists root keys. Only works on mapping nodes.
- **validate**: Returns success/error — checks if the YAML can be parsed.

## Examples

```
# Read a config value
yaml_editor(operation="get", file_path="config.yaml", json_path="server.port")

# Set a value (preserves comments)
yaml_editor(operation="set", file_path="docker-compose.yml", json_path="services.web.ports", set_value=["8080:80"])

# Delete a key
yaml_editor(operation="delete", file_path="config.yaml", json_path="deprecated_setting")

# List root keys
yaml_editor(operation="keys", file_path="config.yaml")

# Validate YAML syntax
yaml_editor(operation="validate", file_path="config.yaml")
```

## Tips

- Use `yaml_editor` instead of read_file + write_file for YAML config changes — it preserves comments and structure.
- `set_value` accepts any type: strings, numbers, booleans, arrays, objects.
- For config.yaml changes, always use yaml_editor to avoid accidentally breaking the file structure.
- Works across all contexts: local workspace, remote devices (Fernsteuerung), homepage containers, and SSH.
