# OneDrive (`onedrive`)

Interact with Microsoft OneDrive cloud storage. List, read, upload, search, delete, move, copy files and folders, check quota, and create share links.

## Operations

| Operation | Description |
|-----------|-------------|
| `list` | List files in a directory |
| `info` | Get file/folder metadata |
| `read` | Read text file content (max 512 KB) |
| `download` | Download file content |
| `search` | Search for files |
| `quota` | Check storage quota |
| `upload` | Upload a text file (`write` is alias) |
| `mkdir` | Create a new folder |
| `delete` | Delete a file or folder |
| `move` | Move a file or folder |
| `copy` | Copy a file or folder |
| `share` | Create anonymous view-only sharing link |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `path` | string | for most operations | Absolute path in OneDrive (e.g., `/Documents/notes.txt`) |
| `destination` | string | for move, copy | Destination path |
| `content` | string | for upload, search | File content or search query |
| `max_results` | integer | no | Max items to return (default: 50 for list, 25 for search) |

## Examples

**List root directory:**
```json
{"action": "onedrive", "operation": "list", "path": "/"}
```

**Read a text file:**
```json
{"action": "onedrive", "operation": "read", "path": "/Documents/notes.txt"}
```

**Search for files:**
```json
{"action": "onedrive", "operation": "search", "content": "quarterly report"}
```

**Check storage quota:**
```json
{"action": "onedrive", "operation": "quota"}
```

**Upload a file:**
```json
{"action": "onedrive", "operation": "upload", "path": "/Backup/report.txt", "content": "This is a backup report."}
```

**Move a file:**
```json
{"action": "onedrive", "operation": "move", "path": "/Documents/report.txt", "destination": "/Archive/report.txt"}
```

**Create a share link:**
```json
{"action": "onedrive", "operation": "share", "path": "/Documents/shared-doc.pdf"}
```

## Notes

- **Paths**: All paths must start with `/`. Root is `/`
- **File size limits**: Upload content limited to 4 MB; read/download limited to 512 KB for text
- **Share links**: Create anonymous view-only sharing links
- **Large files**: For binary files larger than 512 KB, use `info` to get metadata instead of `read`
