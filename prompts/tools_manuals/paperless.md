# Paperless-ngx (`paperless`)

Interact with the Paperless-ngx document management system. Search, retrieve, and manage documents, tags, correspondents, and document types.

## Operations

| Operation | Description |
|-----------|-------------|
| `search` | Full-text search for documents |
| `get` | Get full metadata for a specific document |
| `download` | Download document text content |
| `upload` | Upload a new document |
| `update` | Update document metadata |
| `delete` | Delete a document |
| `list_tags` | List all available tags |
| `list_correspondents` | List all correspondents |
| `list_document_types` | List all document types |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `query` | string | for search | Full-text search query |
| `document_id` | string | for get/download/update/delete | Numeric document ID |
| `title` | string | no | Document title (for upload/update) |
| `tags` | string | no | Comma-separated tag names |
| `name` | string | no | Correspondent name (maps to Paperless-ngx "correspondent") |
| `category` | string | no | Document type name (maps to Paperless-ngx "document type") |
| `content` | string | for upload | Text content to upload as a document |
| `limit` | integer | no | Max results for search (default: 25) |

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

- **Tag format**: Tags in `upload` and `update` are comma-separated names (e.g., `"invoice,2025,important"`)
- **Correspondent**: The `name` parameter maps to Paperless-ngx's "correspondent" field
- **Document type**: The `category` parameter maps to Paperless-ngx's "document type" field
- **Discovery**: Use `list_tags`, `list_correspondents`, `list_document_types` to discover available filters before searching
