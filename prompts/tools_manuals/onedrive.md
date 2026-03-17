---
id: "onedrive"
tags: ["system", "storage", "cloud", "microsoft"]
priority: 5
---
# internal_tool: onedrive

Interact with the user's connected Microsoft OneDrive cloud storage.
The OneDrive tool allows you to list, read, upload, search, delete, move, copy files and folders, check quota, and create share links. All paths MUST be absolute (starting with `/`).

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operation` | string | yes | `list`, `info`, `read`, `download`, `search`, `quota`, `upload`, `write`, `mkdir`, `delete`, `move`, `copy`, `share` |
| `path` | string | varies | Absolute path in OneDrive (e.g. `/Documents/notes.txt` or `/`). Required for most operations except `search` and `quota`. |
| `destination` | string | no | Destination path for `move` and `copy` operations. |
| `content` | string | no | Text content to upload (for `upload`/`write`) or search query (for `search`). |
| `max_results` | integer | no | Maximum number of items to return (default: 50 for `list`, 25 for `search`). |

**Important Rules for OneDrive:**
1. All paths must start with `/`. The root directory is `/`.
2. For `upload`/`write`, `path` is the full file path including filename (e.g. `/Documents/report.txt`). `write` is an alias for `upload`.
3. For `move` and `copy`, `path` is the source and `destination` is the full target path.
4. For `search`, pass the search query in the `content` parameter.
5. The `read`/`download` operations return text content (limited to 512 KB). For large or binary files, use `info` to get metadata instead.
6. The `share` operation creates an anonymous view-only sharing link.
7. Upload content is limited to 4 MB. Larger files cannot be uploaded via this tool.

### Examples

**List the root directory:**
```json
{"action": "onedrive", "operation": "list", "path": "/"}
```

**List a subfolder with limited results:**
```json
{"action": "onedrive", "operation": "list", "path": "/Documents", "max_results": 10}
```

**Read a text file:**
```json
{"action": "onedrive", "operation": "read", "path": "/Documents/notes.txt"}
```

**Get file/folder info:**
```json
{"action": "onedrive", "operation": "info", "path": "/Photos/vacation.jpg"}
```

**Search for files:**
```json
{"action": "onedrive", "operation": "search", "content": "quarterly report"}
```

**Check storage quota:**
```json
{"action": "onedrive", "operation": "quota"}
```

**Upload a new text file:**
```json
{"action": "onedrive", "operation": "upload", "path": "/Backup/report.txt", "content": "This is a backup report."}
```

**Create a new folder:**
```json
{"action": "onedrive", "operation": "mkdir", "path": "/NewFolder"}
```

**Delete a file:**
```json
{"action": "onedrive", "operation": "delete", "path": "/Backup/old-report.txt"}
```

**Move a file:**
```json
{"action": "onedrive", "operation": "move", "path": "/Documents/report.txt", "destination": "/Archive/report.txt"}
```

**Copy a file:**
```json
{"action": "onedrive", "operation": "copy", "path": "/Documents/report.txt", "destination": "/Backup/report-copy.txt"}
```

**Create a share link:**
```json
{"action": "onedrive", "operation": "share", "path": "/Documents/shared-doc.pdf"}
```
