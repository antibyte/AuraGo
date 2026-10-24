# Paperless-ngx (`paperless`)

Interact with the Paperless-ngx document management system. Search, retrieve, and manage documents, tags, correspondents, and document types.

## Operations

| Operation | Description |
|-----------|-------------|
| `search` | Full-text search for documents |
| `get` | Get metadata for a specific document. OCR text is clipped at 8000 characters and the message reports the cut. |
| `download` | Read OCR text, truncated at 8000 characters |
| `upload` | Upload text and return a task id while processing continues |
| `update` | Replace title, correspondent, or document type. Add tags without removing existing ones. Tags that are already present are not written again. |
| `delete` | Delete a document |
| `list_tags` | List all available tags |
| `list_correspondents` | List all correspondents |
| `list_document_types` | List all document types |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `query` | string | for search | Full-text search query. Search does not use `content`. |
| `document_id` | string | for get/download/update/delete | Positive document ID. A JSON number is accepted. |
| `title` | string | no | Document title. Upload sets it. Update replaces it. |
| `tags` | string | no | Comma-separated exact tag names or positive IDs. An array is accepted and joined. Search requires every tag. Update adds tags and keeps the current ones. |
| `name` / `correspondent` | string | no | Exact correspondent name or positive ID. `correspondent` wins when both are set. |
| `category` / `document_type` | string | no | Exact document type name or positive ID. `document_type` wins when both are set. |
| `content` | string | for upload | Text uploaded as a `.txt` file. A success result includes `task_id` while processing is still running. |
| `limit` | integer | no | Max search results. Default 25, capped at 50. A numeric string is accepted. |

## Examples

**Search for invoices:**
```json
{"action": "paperless", "operation": "search", "query": "invoice electricity"}
```

**Search with filters:**
```json
{"action": "paperless", "operation": "search", "query": "2025", "tags": "invoice", "name": "Stadtwerke"}
```

**Get document metadata:**
```json
{"action": "paperless", "operation": "get", "document_id": "42"}
```

**Upload a new document:**
```json
{"action": "paperless", "operation": "upload", "title": "Meeting Notes March 2026", "content": "Discussed Q1 results...", "tags": "meeting,notes", "name": "Company"}
```

**Update document metadata:**
```json
{"action": "paperless", "operation": "update", "document_id": "42", "title": "Corrected Title", "tags": "invoice,corrected"}
```

**Delete a document:**
```json
{"action": "paperless", "operation": "delete", "document_id": "42"}
```

**List all tags:**
```json
{"action": "paperless", "operation": "list_tags"}
```

## Notes

- **Tag format**: Tags are comma-separated exact names or positive IDs (for example `"invoice,2025"`). Search matches every tag. Upload stores those IDs. Update adds them to the document's current tags. An empty tag list changes nothing. When every requested tag is already present, update does not write. When the document response has no tag list, update leaves the tags unchanged.
- **Correspondent**: `name` and `correspondent` are the same field. A partial name such as `Stadt` does not match `Stadtwerke`.
- **Document type**: `category` and `document_type` are the same field. Each value is one name, not a comma-separated list.
- **Upload**: Only the text in `content` is uploaded, as a `.txt` file. The result is a processing task id, not a searchable document yet.
- **Get and download**: The returned text is the OCR content. When it is longer than 8000 characters, the message reports the returned length and the original length.
- **Read-only**: `paperless_ngx.readonly` blocks upload, update, and delete.
- **Credentials**: The API token is Vault key `paperless_ngx_api_token` and applies when that secret is saved. The URL and toggles apply when the configuration is saved.
- **Discovery**: Use `list_tags`, `list_correspondents`, and `list_document_types` before filtering. A list that hits the page cap says how many rows are shown out of the reported total.
