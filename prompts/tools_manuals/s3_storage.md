# Tool: `s3_storage`

## Purpose
Manage objects in S3-compatible storage services (AWS S3, MinIO, Wasabi, Backblaze B2). List, upload, download, delete, copy, and move objects across buckets.

## Prerequisites
- `s3.enabled: true` in config
- Store `s3_access_key` and `s3_secret_key` in the secrets vault
- For S3-compatible services (MinIO, Wasabi): set `s3.endpoint` and `s3.use_path_style: true`

## Parameters
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | ✅ | `list_buckets`, `list_objects`, `upload`, `download`, `delete`, `copy`, `move` |
| `bucket` | string | ❌ | Bucket name (uses config default if omitted) |
| `key` | string | ❌ | Object key (path within bucket) — required for upload/download/delete/copy/move |
| `local_path` | string | ❌ | Local file path — required for upload (source), optional for download (destination) |
| `prefix` | string | ❌ | Key prefix filter for list_objects (e.g. `backups/2025/`) |
| `destination_bucket` | string | ❌ | Target bucket for copy/move (defaults to source bucket) |
| `destination_key` | string | ❌ | Target key for copy/move |

## Operations

### `list_buckets`
List all accessible buckets. No additional parameters needed.

### `list_objects`
List objects in a bucket. Use `prefix` to filter by path.

### `upload`
Upload a local file to S3. Requires `key` and `local_path`.

### `download`
Download an S3 object to a local file. Requires `key`; `local_path` defaults to the key's filename.

### `delete`
Delete an object from S3. Requires `key`.

### `copy`
Copy an object within or across buckets. Requires `key`, `destination_bucket`, and `destination_key`.

### `move`
Move (copy + delete source) within or across buckets. Same params as copy.

## Configuration
```yaml
s3:
  enabled: true
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
