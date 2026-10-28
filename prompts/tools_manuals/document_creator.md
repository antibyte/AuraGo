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
| `title` | string | no | Optional document title, used only by create_pdf |
| `content` | string | for html_to_pdf, screenshot_html, markdown_to_pdf | HTML/Markdown/text content |
| `url` | string | for url_to_pdf, screenshot_url | URL to capture |
| `filename` | string | no | Output filename without extension (auto-generated if omitted) |
| `paper_size` | string | no | Paper size: A4, A3, A5, Letter, Legal, Tabloid |
| `landscape` | boolean | no | Landscape orientation (default: false) |
| `sections` | string | no | JSON array of sections for create_pdf; nonempty sections take precedence over content |
| `source_files` | string | for merge_pdfs, convert_document | JSON array of file paths |
| `block_remote_content` | boolean | no | Adds an offline restriction for supplied HTML/Markdown. Default: false; the admin policy still blocks remote resources by default and cannot be relaxed by a tool call. See below |

## Sections Format (create_pdf)

Each section is a JSON object with `type`, `header`, and `body`:
```json
[
  {"type": "text", "header": "Introduction", "body": "Document content here..."},
  {"type": "table", "header": "Data", "rows": [["Col1", "Col2"], ["Val1", "Val2"]]},
  {"type": "list", "header": "Items", "body": "Item 1\nItem 2\nItem 3"}
]
```

Both backends validate sections before rendering. A table with no rows is valid;
otherwise its first row must contain 1–12 columns and every row must have the same
width. Invalid input returns a JSON error without submitting a render. The title
and each section header appear once; an empty body does not repeat the header.
With no sections (including `[]`), `create_pdf` uses `content`.

## Rendering untrusted HTML (`block_remote_content`)

Supplied HTML/Markdown stays offline unless an administrator enables
`tools.document_creator.allow_remote_resources`. Setting `block_remote_content:
true` always keeps it offline, including after that admin grant:
- Remote images, stylesheets, fonts, frames and fetches are not loaded. Embed images and fonts as `data:` URLs; inline CSS (`<style>`, `style=""`) works.
- Scripts do not run.
- Meta refresh tags and connection hints (`<link rel="preconnect">`, `dns-prefetch`, `prefetch`, `prerender`, `preload`) are neutralized.
- `url_to_pdf`, `screenshot_url` and `convert_document` cannot be protected this way and return an error when the flag is set.

The admin grant applies only to `html_to_pdf`, `markdown_to_pdf`, `screenshot_html`
and Gotenberg `create_pdf`. Public URL captures remain available. Every Gotenberg
conversion first verifies version 8.38.0+ within major version 8 and its runtime
network filters via the private `/debug` route; failed or incomplete verification
prevents upload. Private targets, redirects and subresources remain blocked even
when public resources are enabled. Flows retain their stricter offline policy.

## Examples

**Create a PDF from Markdown:**
```json
{"action": "document_creator", "operation": "markdown_to_pdf", "content": "# My Document\n\nThis is a **bold** statement."}
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
tools:
  document_creator:
    enabled: false
    backend: maroto
    output_dir: data/documents
    allow_remote_resources: false
    gotenberg:
      url: "" # native loopback / Docker service default
      timeout: 120
```

## Notes

- **PDF creation**: Use `create_pdf` for structured documents with sections, `markdown_to_pdf` for simple Markdown content
- **Web capture**: `url_to_pdf` and `screenshot_url` capture webpages at their current state
- **Gotenberg**: For full functionality (web capture, Office conversion), enable the Gotenberg Docker sidecar
- **Sources**: `convert_document` requires exactly one regular file; `merge_pdfs` requires at least two. Maximum 20 files, 32 MiB per file and 64 MiB total, checked before staging and while reading. Workspace protections still apply.
- **Output**: Existing documents are preserved: `report.pdf`, `report-1.pdf`, …, `report-999.pdf`. The response contains the actual filename/path. Unicode is supported; control characters, NTFS streams and reserved device names are rejected. PDF/image responses are limited to 10 MiB.
- **Cancellation**: Copies and rendering honor the caller context. Maroto's internal generation cannot be interrupted; a canceled result is discarded.
- **Gallery**: Agent dispatch registers successful files when the media registry is available. Direct backend calls return paths without registration.
- **Operations**: See `documentation/document-creator.md` for the pinned sidecar profile, startup migration and saved-settings diagnostics. Document Creator has no read-only setting.
