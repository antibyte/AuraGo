# Koofr (`koofr`)

Interact with Koofr Cloud Storage. Read, write, and manage folders and files in the primary Koofr Vault.

## Operations

| Operation | Description |
|-----------|-------------|
| `list` | List files in a directory |
| `read` | Read text file content |
| `download` | Download a file into the local workspace |
| `write` | Upload a non-empty text file from the `content` parameter |
| `upload` | Upload an existing local file, including binary files such as generated images |
| `mkdir` | Create a new folder |
| `delete` | Delete a file or folder |
| `rename` | Rename a file or folder |
| `copy` | Copy a file or folder inside Koofr |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `path` | string | yes | Absolute path inside Koofr (e.g., `/Documents/notes.txt`) |
| `destination` | string | for rename, copy, write, upload, download | Target path or filename |
| `content` | string | for write | Non-empty text content to upload |
| `local_path` | string | for upload | Existing local file path to upload |

## Examples

**List root directory:**
```json
{"action": "koofr", "operation": "list", "path": "/"}
```

**Upload a file:**
```json
{"action": "koofr", "operation": "write", "path": "/Backup", "destination": "report.txt", "content": "This is a backup report."}
```

**Upload an existing generated image or other local binary file:**
```json
{"action": "koofr", "operation": "upload", "path": "/Backup", "destination": "funny_cat_car.jpeg", "local_path": "workdir/funny_cat_car.jpeg"}
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
- **write**: `destination` is the filename, `path` is the target directory, and `content` must be non-empty. Use `upload` for existing local files.
- **upload**: `local_path` is the existing local source file, `path` is the Koofr target directory, and `destination` is the remote filename. The source file must exist and must not be 0 bytes.
- **read**: Only for text files. For audio, images, PDFs, or other binary files use `download`
- **download**: `destination` is a local workspace path such as `workdir/song.mp3`
- **rename/copy**: `path` is source, `destination` is a target path inside Koofr
