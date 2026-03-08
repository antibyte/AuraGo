---
id: "koofr"
tags: ["system", "storage", "cloud"]
priority: 5
---
# internal_tool: koofr

Interact with the user's connected Koofr Cloud Storage account.
The Koofr tool allows you to read, write, and manage folders and files directly in their primary Koofr Vault. All paths MUST be absolute (starting with `/`).

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operation` | string | yes | `list`, `read`, `write`, `mkdir`, `delete`, `rename`, `copy` |
| `path` | string | yes | Absolute path inside Koofr (e.g. `/Documents/notes.txt` or `/`) |
| `destination` | string | no | Destination path for `rename`, `copy`, or `write` |
| `content` | string | no | Text content to upload when using `write` |

**Important Rules for Koofr:**
1. All paths must start with `/`. The root directory is `/`.
2. When performing `mkdir`, the `path` should be the full path of the new directory (e.g. `/NewFolder`).
3. For `write`, `destination` is the filename (e.g., `test.txt`) and `path` is the target directory (e.g., `/Backup`).
4. For `rename` and `copy`, `path` is the source and `destination` is the target.

### Examples

**List the root directory:**
```json
{"action": "koofr", "operation": "list", "path": "/"}
```

**Upload a new text file:**
```json
{"action": "koofr", "operation": "write", "path": "/Backup", "destination": "report.txt", "content": "This is a backup report."}
```

**Read a text file:**
```json
{"action": "koofr", "operation": "read", "path": "/Backup/report.txt"}
```

**Create a new folder:**
```json
{"action": "koofr", "operation": "mkdir", "path": "/MyNewFolder"}
```

**Delete a file:**
```json
{"action": "koofr", "operation": "delete", "path": "/Backup/report.txt"}
```
