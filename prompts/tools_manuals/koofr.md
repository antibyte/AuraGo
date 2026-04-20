# Koofr (`koofr`)

Interact with Koofr Cloud Storage. Read, write, and manage folders and files in the primary Koofr Vault.

## Operations

| Operation | Description |
|-----------|-------------|
| `list` | List files in a directory |
| `read` | Read text file content |
| `download` | Download a file into the local workspace |
| `write` | Upload a text file |
| `mkdir` | Create a new folder |
| `delete` | Delete a file or folder |
| `rename` | Rename a file or folder |
| `copy` | Copy a file or folder inside Koofr |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `path` | string | yes | Absolute path inside Koofr (e.g., `/Documents/notes.txt`) |
| `destination` | string | for rename, copy, write, download | Target path or filename |
| `content` | string | for write | Text content to upload |

## Examples

**List root directory:**
```json
{"action": "koofr", "operation": "list", "path": "/"}
```

**Upload a file:**
```json
{"action": "koofr", "operation": "write", "path": "/Backup", "destination": "report.txt", "content": "This is a backup report."}
```

**Read a file:**
```json
{"action": "koofr", "operation": "read", "path": "/Backup/report.txt"}
```

**Download a binary file to the workspace:**
```json
{"action": "koofr", "operation": "download", "path": "/suno/song.mp3", "destination": "workdir/song.mp3"}
```

**Create a folder:**
```json
{"action": "koofr", "operation": "mkdir", "path": "/MyNewFolder"}
```

**Delete a file:**
```json
{"action": "koofr", "operation": "delete", "path": "/Backup/report.txt"}
```

## Notes

- **Paths**: All paths must start with `/`. Root is `/`
- **mkdir**: The `path` should be the full path of the new directory
- **write**: `destination` is the filename, `path` is the target directory
- **read**: Only for text files. For audio, images, PDFs, or other binary files use `download`
- **download**: `destination` is a local workspace path such as `workdir/song.mp3`
- **rename/copy**: `path` is source, `destination` is a target path inside Koofr
