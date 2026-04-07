# Document Creator (`document_creator`)

Create PDF documents, convert files to PDF, merge PDFs, and take screenshots. Backend is configured in settings: `maroto` (built-in, create_pdf only) or `gotenberg` (Docker sidecar, all operations).

## Operations

| Operation | Description |
|-----------|-------------|
| `create_pdf` | Create structured PDF document from sections |
| `url_to_pdf` | Capture webpage as PDF |
| `html_to_pdf` | Render HTML content as PDF |
| `markdown_to_pdf` | Render Markdown as PDF |
| `convert_document` | Convert Office files to PDF via LibreOffice |
| `merge_pdfs` | Combine multiple PDFs into one |
| `screenshot_url` | Capture webpage as image |
| `screenshot_html` | Render HTML to image |
| `health` | Check Gotenberg status |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `title` | string | for create_pdf | Document title |
| `content` | string | for html_to_pdf, screenshot_html, markdown_to_pdf | HTML/Markdown/text content |
| `url` | string | for url_to_pdf, screenshot_url | URL to capture |
| `filename` | string | no | Output filename without extension (auto-generated if omitted) |
| `paper_size` | string | no | Paper size: A4, A3, A5, Letter, Legal, Tabloid |
| `landscape` | boolean | no | Landscape orientation (default: false) |
| `sections` | string | for create_pdf | JSON array of sections |
| `source_files` | string | for merge_pdfs, convert_document | JSON array of file paths |

## Sections Format (create_pdf)

Each section is a JSON object with `type`, `header`, and `body`:
```json
[
  {"type": "text", "header": "Introduction", "body": "Document content here..."},
  {"type": "table", "header": "Data", "rows": [["Col1", "Col2"], ["Val1", "Val2"]]},
  {"type": "list", "header": "Items", "body": "Item 1\nItem 2\nItem 3"}
]
```

## Examples

**Create a PDF from Markdown:**
```json
{"action": "document_creator", "operation": "markdown_to_pdf", "title": "My Document", "content": "# Hello\n\nThis is a **bold** statement."}
```

**Capture a webpage as PDF:**
```json
{"action": "document_creator", "operation": "url_to_pdf", "url": "https://example.com", "filename": "example-page"}
```

**Merge multiple PDFs:**
```json
{"action": "document_creator", "operation": "merge_pdfs", "source_files": "[\"doc1.pdf\", \"doc2.pdf\"]", "filename": "combined"}
```

**Take a screenshot of a webpage:**
```json
{"action": "document_creator", "operation": "screenshot_url", "url": "https://example.com", "filename": "screenshot"}
```

**Check Gotenberg status:**
```json
{"action": "document_creator", "operation": "health"}
```

## Configuration

```yaml
# Backend selection in settings:
# - maroto: built-in, create_pdf only
# - gotenberg: Docker sidecar, all operations
```

## Notes

- **PDF creation**: Use `create_pdf` for structured documents with sections, `markdown_to_pdf` for simple Markdown content
- **Web capture**: `url_to_pdf` and `screenshot_url` capture webpages at their current state
- **Gotenberg**: For full functionality (web capture, Office conversion), enable the Gotenberg Docker sidecar
- **Output**: Generated files are saved to the media registry and can be accessed via the gallery