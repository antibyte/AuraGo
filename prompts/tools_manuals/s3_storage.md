# Tool: `s3_storage`

## Purpose
Manage objects in S3-compatible storage services (AWS S3, MinIO, Wasabi, Backblaze B2, or managed RustFS). List, upload, download, delete, copy, and move objects across buckets.

## Prerequisites
- `s3.enabled: true` in config
- External provider: store `s3_access_key` and `s3_secret_key` in the secrets vault.
- Managed RustFS provider: complete administrator setup, select `s3.provider: rustfs`, and enable RustFS agent access. Credentials and the endpoint are managed privately; never ask for root credentials.
- For S3-compatible services (MinIO, Wasabi): set `s3.endpoint` and `s3.use_path_style: true`
- HTTP endpoints require `s3.insecure: true`; HTTPS is used by default.

## Parameters
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | ✅ | `list_buckets`, `list_objects`, `upload`, `download`, `delete`, `copy`, `move` |
| `bucket` | string | ❌ | Bucket name (uses config default if omitted) |
| `key` | string | ❌ | Object key (path within bucket) — required for upload/download/delete/copy/move |
| `local_path` | string | ❌ | Local file path — external uploads allow workspace/data files; managed uploads allow protected workspace/document files; downloads stay inside the workspace |
| `prefix` | string | ❌ | Key prefix filter for list_objects (e.g. `backups/2025/`) |
| `destination_bucket` | string | ❌ | Target bucket for copy/move (defaults to source bucket) |
| `destination_key` | string | ❌ | Target key for copy/move |

## Operations

### `list_buckets`
External S3 lists accessible buckets. Managed RustFS returns only configured permitted buckets and does not request the account-wide bucket list.

### `list_objects`
List objects in a bucket. Use `prefix` to filter by its literal object-key prefix. Managed RustFS permits only configured bucket/prefix pairs. An omitted prefix selects the single permitted prefix, or requires an explicit choice when several are configured. Managed results stop at 1,000 objects or the tool-output byte budget, mark `truncated`, and require a narrower prefix for more detail.

### `upload`
Upload a local file to S3. Requires `key` and `local_path`; external source files must be inside AuraGo's workspace or data directory. Managed RustFS sources must be admitted workspace/document files; configuration, credentials, databases and the data directory are refused. Write permission includes overwriting an existing key.

### `download`
Download an S3 object to a local workspace file. Requires `key`; `local_path` defaults to the key's filename inside the workspace. Requires `agent.allow_filesystem_write`. Publication is rooted and atomic; errors, oversized/truncated bodies and revoked access preserve an existing destination. The download byte limit follows `virtual_desktop.max_file_size_mb` (50 MiB when unset).

### `delete`
Delete an object from S3. Requires `key`.

### `copy`
Copy an object within or across buckets. Requires `key` and `destination_key`; `destination_bucket` defaults to the source bucket.

### `move`
Move (copy + delete source) within or across buckets. Same params as copy. Managed RustFS requires source Read and Delete plus destination Write. A successful copy followed by a failed delete reports partial failure and retains the source; do not treat that result as a completed move.

## Configuration
```yaml
s3:
  enabled: true
  provider: external  # omitted preserves existing external S3
  readonly: false
  endpoint: "https://s3.amazonaws.com"  # or http://minio.local:9000
  region: "us-east-1"
  bucket: "my-default-bucket"
  use_path_style: false  # true for MinIO / S3-compatible
  insecure: false         # true to allow HTTP endpoints
```

## Example Calls
```json
{ "operation": "list_buckets" }
{ "operation": "list_objects", "bucket": "backups", "prefix": "2025/01/" }
{ "operation": "upload", "bucket": "data", "key": "reports/analysis.pdf", "local_path": "agent_workspace/workdir/analysis.pdf" }
{ "operation": "download", "bucket": "data", "key": "config/settings.json", "local_path": "agent_workspace/workdir/settings.json" }
{ "operation": "delete", "bucket": "temp", "key": "old-file.txt" }
{ "operation": "copy", "key": "file.txt", "destination_bucket": "archive", "destination_key": "2025/file.txt" }
{ "operation": "move", "bucket": "inbox", "key": "new.csv", "destination_bucket": "processed", "destination_key": "data/new.csv" }
```

## Managed permissions

Managed Read, Write and Delete are separate grants. Copy requires source Read and destination Write. `s3.readonly` overrides every mutation grant. Empty allowed locations or prefix lists grant nothing; an explicitly configured empty prefix grants the whole named bucket. Prefixes and keys are case-sensitive literal strings, so `agent/` does not grant `agent2/`; `..` and encoded characters are ordinary key bytes rather than filesystem traversal. No bucket deletion, IAM administration or presigned-link operation is exposed.

Provider, target, policy and credential changes revoke active managed requests. Start a new request after an authorization change; never switch to external S3 or administrator credentials as a fallback. Switching provider preserves inactive settings and moves no objects.
