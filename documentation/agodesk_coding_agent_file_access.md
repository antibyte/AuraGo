# AgoDesk Coding Agent: File Access Implementation

AgoDesk can expose local file access to AuraGo through the existing `desktop.command` / `desktop.result` WebSocket transport. AgoDesk remains the authority for which folders are available and whether each folder permits reads, writes, or both.

## Contract

Advertise file support in `session.start` only when the user has enabled it locally:

```json
{
  "client_capabilities": ["remote.files.read", "remote.files.write"],
  "file_access": {
    "enabled": true,
    "max_read_bytes": 8388608,
    "max_write_bytes": 8388608,
    "roots": [
      {
        "root_id": "workspace",
        "label": "Workspace",
        "path_display": "~/Projects/AuraGo",
        "permissions": ["read", "write"]
      }
    ]
  }
}
```

AuraGo may then send:

- `file_list`: requires `remote.files.read`
- `file_read`: requires `remote.files.read`
- `file_write`: requires `remote.files.write`

Do not implement delete, rename, shell execution, advanced edits, or chunked file transfer for this v1 protocol.

## Required AgoDesk Changes

1. Add a local file-access settings model with stable `root_id`, display label, canonical absolute path, and permissions.
2. Include only enabled permissions in `session.start.client_capabilities`.
3. Include `file_access.roots` in `session.start`, using `path_display` for UI/debug display and keeping the canonical path local.
4. Handle `desktop.command.operation=file_list`, `file_read`, and `file_write`.
5. Return `desktop.result` with `ok=false` and a stable error code when access is disabled, denied, too large, or conflicts with a write guard.
6. Never log file contents. Audit operation, root id, relative path, byte count, status, and command id only.

## Path Validation

For every command:

1. Resolve `root_id` to the configured root. If absent, find the configured root that contains the requested absolute path.
2. Canonicalize the root and target path before access.
3. Reject `..` traversal, absolute paths under the wrong root, drive-prefix tricks, UNC escapes, and symlinks that resolve outside the root.
4. Check the requested permission after canonicalization:
   - `file_list` and `file_read` need `read`.
   - `file_write` needs `write`.
5. Apply the inline size limit before sending or accepting content.

## Atomic Writes

For `file_write`:

1. Validate parent directory access before creating anything.
2. Write to a temporary file in the target directory.
3. Flush/sync when supported by the platform.
4. Rename over the target atomically.
5. Clean up the temporary file on failure.
6. Return `FILE_CONFLICT` if AgoDesk implements an expected-hash or no-overwrite guard and the guard fails.

## Suggested Type Shape

```ts
type FileAccessRoot = {
  rootId: string;
  label: string;
  canonicalPath: string;
  pathDisplay: string;
  permissions: Array<"read" | "write">;
};

type FileCommandParams = {
  root_id?: string;
  path: string;
  recursive?: boolean;
  encoding?: "utf-8";
  content?: string;
};
```

## Acceptance Criteria

- AgoDesk does not advertise file capabilities when local file access is disabled.
- `file_read` and `file_list` work for read-enabled roots.
- `file_write` works only for write-enabled roots.
- Paths outside all configured roots are denied.
- Symlinks that escape a configured root are denied.
- Files above the negotiated inline size limit return `FILE_TOO_LARGE`.
- Error logs and audit logs never contain file contents.
- Older AuraGo servers that ignore `file_access` still keep chat, persona assets, and desktop control working.
