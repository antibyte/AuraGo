## Tool: Workspace Search (`workspace_search`)

Fast resident search over `directories.workspace_dir`. Use it before broad file walks when you need to find files, grep indexed text, inspect recent files, rescan the index, or check search status.

### Scope and Safety

- Searches only the configured agent workspace directory.
- Does not persist file content. Only file access metadata is stored in `data/workspace_search.db`.
- Skips common dependency/cache folders, `.env`, database/vault files, binary files, large files beyond `workspace_search.max_file_size_mb`, and symlink escapes.
- The index is built asynchronously. If it is still building, use `file_search` as a compatibility fallback.

### Operations

| Operation | Purpose |
|-----------|---------|
| `find` | Fuzzy path search, boosted by frecency |
| `grep` | Plain or regex search over indexed text lines |
| `glob` | Return files matching a glob, e.g. `**/*.go` |
| `recent` | Return recently accessed workspace files |
| `rescan` | Force a full workspace rescan |
| `status` | Show index readiness, counts, root, and excludes |

### Parameters

| Parameter | Type | Notes |
|-----------|------|-------|
| `operation` | string | Required. `find`, `grep`, `glob`, `recent`, `rescan`, or `status` |
| `query` | string | Search text for `find` and `grep` |
| `pattern` | string | Alias for `query` for compatibility |
| `glob` | string | Optional file filter such as `**/*.go`; required for `glob` if `query` is empty |
| `mode` | string | `plain` or `regex` for grep; `fuzzy_path` for find |
| `output_mode` | string | `content` or `count` for grep |
| `case_sensitive` | boolean | Default false |
| `limit` | integer | Defaults to `workspace_search.max_results`, capped at 1000 |

### Examples

```json
{"action": "workspace_search", "operation": "status"}
```

```json
{"action": "workspace_search", "operation": "find", "query": "invoice parser", "limit": 20}
```

```json
{"action": "workspace_search", "operation": "grep", "query": "TODO|FIXME", "mode": "regex", "glob": "**/*.go"}
```

```json
{"action": "workspace_search", "operation": "grep", "query": "password", "glob": "**/*.yaml,**/*.yml", "output_mode": "count"}
```

```json
{"action": "workspace_search", "operation": "recent", "limit": 10}
```
