---
id: "paperless"
tags: ["system", "documents", "dms"]
priority: 5
---
# internal_tool: paperless

Interact with the user's Paperless-ngx document management system.
Search, retrieve, and manage documents, tags, correspondents, and document types.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `operation` | string | yes | `search`, `get`, `download`, `upload`, `update`, `delete`, `list_tags`, `list_correspondents`, `list_document_types` |
| `query` | string | for search | Full-text search query |
| `document_id` | string | for get/download/update/delete | Numeric document ID |
| `title` | string | no | Document title (for upload/update) |
| `tags` | string | no | Comma-separated tag names (for search filter or upload/update) |
| `name` | string | no | Correspondent name (for search filter or upload/update) |
| `category` | string | no | Document type name (for search filter or upload/update) |
| `content` | string | for upload | Text content to upload as a new document |
| `limit` | int | no | Max results for search (default: 25) |

**Important Rules:**
1. Use `search` to find documents by text content, tags, correspondents, or document types.
2. Use `get` to retrieve full metadata for a specific document (includes content preview).
3. Use `download` to get a document's full text content.
4. Use `list_tags`, `list_correspondents`, `list_document_types` to discover available categories before filtering.
5. Tags in `upload` and `update` are comma-separated names (e.g. `"invoice,2025,important"`).
6. The `name` parameter maps to the Paperless-ngx "correspondent" field.
7. The `category` parameter maps to the Paperless-ngx "document type" field.

### Examples

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

**Download document text content:**
```json
{"action": "paperless", "operation": "download", "document_id": "42"}
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

**List all correspondents:**
```json
{"action": "paperless", "operation": "list_correspondents"}
```

**List all document types:**
```json
{"action": "paperless", "operation": "list_document_types"}
```
