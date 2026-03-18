---
id: "tools_s3"
tags: ["conditional"]
priority: 31
conditions: ["s3_enabled"]
---
### S3-Compatible Cloud Storage
| Tool | Purpose |
|---|---|
| `s3_storage` | Manage objects in S3-compatible storage (AWS S3, MinIO, Wasabi, B2): list_buckets, list_objects, upload, download, delete, copy, move |

Credentials must be stored in the secrets vault as `s3_access_key` and `s3_secret_key`.
