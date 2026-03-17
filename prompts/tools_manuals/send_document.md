## Tool: Send Document (`send_document`)

Send a document to the user. In the Web UI it appears as a document card with an Open button (inline preview for PDF and text formats) and a Download button. Provide a local workspace path or a direct HTTPS URL to a document.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `path` | string | yes | Local workspace path (e.g. `report.pdf`) **or** a full HTTPS URL to a document |
| `title` | string | no | Optional title shown on the document card |

### Supported Formats

| Format | Preview |
|--------|---------|
| PDF | Inline in browser |
| TXT, MD | Inline in browser |
| CSV, JSON, XML | Inline in browser |
| DOCX, XLSX, PPTX | Download only |

### Workflow

1. Generate or obtain a document (via `execute_python`, `document_creator`, or skill).
2. Call `send_document` with the workspace-relative path or a URL.
3. Include the returned `web_path` in your final response text so references persist.

### Examples

```json
{"action": "send_document", "path": "report.pdf", "title": "Monthly Report"}
```

```json
{"action": "send_document", "path": "data.csv", "title": "Exported Data"}
```

### Notes

- Documents are copied to `data/documents/` and served at `/files/documents/...`
- URL documents are downloaded automatically — no pre-download needed
- PDF and plain-text files include a `preview_url` (`?inline=1`) that opens them in-browser
- **On error:** try at most **one alternative path**, then inform the user. Do **not** loop.
