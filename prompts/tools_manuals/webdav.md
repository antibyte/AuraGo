# WebDAV Tool

Access files on a WebDAV-compatible cloud storage (Nextcloud, ownCloud, Synology, Box, etc.).

## Prerequisites
- `webdav.enabled: true` in config.yaml
- `webdav.url` set to the WebDAV endpoint (e.g. `https://cloud.example.com/remote.php/dav/files/user/`)
- either:
  - `webdav.auth_type: basic` with `webdav.username` and a password/app password stored in the Vault
  - or `webdav.auth_type: bearer` with a bearer token stored in the Vault

## Operations

### list — List directory contents
```json
{"action": "webdav", "operation": "list", "path": "/"}
{"action": "webdav", "operation": "list", "path": "Documents/Projects"}
```

### read — Download and display a text file
```json
{"action": "webdav", "operation": "read", "path": "Documents/notes.txt"}
```

### write — Upload/create a file
```json
{"action": "webdav", "operation": "write", "path": "Documents/report.md", "content": "# Monthly Report\n\nContent here..."}
```

### mkdir — Create a directory
```json
{"action": "webdav", "operation": "mkdir", "path": "Documents/NewFolder"}
```

### delete — Delete a file or directory
```json
{"action": "webdav", "operation": "delete", "path": "Documents/old_file.txt"}
```

### move — Move or rename a file/directory
```json
{"action": "webdav", "operation": "move", "path": "Documents/draft.md", "destination": "Documents/final.md"}
```

### info — Get metadata for a single file/directory
```json
{"action": "webdav", "operation": "info", "path": "Documents/report.pdf"}
```

## Important Notes
- All `path` values are relative to the configured `webdav.url` base
- Authentication is configured globally in the WebDAV section; the agent does not pass credentials manually per tool call
- `read` output is truncated to ~8000 chars for large files
- `write` creates a new file or overwrites an existing one
- `move` will fail if the destination already exists (no overwrite by default)
- `delete` on a directory removes it recursively
