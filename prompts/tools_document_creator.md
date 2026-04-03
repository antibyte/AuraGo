---
id: "tools_document_creator"
tags: ["conditional"]
priority: 20
conditions: ["document_creator_enabled"]
---
### Document Creator
| Tool | Purpose |
|---|---|
| `document_creator` | Create PDFs, convert documents, take screenshots — supports Maroto (built-in) and Gotenberg (Docker sidecar) backends |

**Operations:**
| Operation | Backend | Description |
|---|---|---|
| `create_pdf` | Both | Create a PDF from structured sections (title, text, tables, lists) |
| `url_to_pdf` | Gotenberg | Convert a web page URL to PDF |
| `html_to_pdf` | Gotenberg | Convert raw HTML content to PDF |
| `markdown_to_pdf` | Gotenberg | Convert Markdown content to PDF |
| `convert_document` | Gotenberg | Convert office documents (DOCX, XLSX, PPTX, ODT, etc.) to PDF |
| `merge_pdfs` | Gotenberg | Merge multiple existing PDF files into one |
| `screenshot_url` | Gotenberg | Take a screenshot of a web page |
| `screenshot_html` | Gotenberg | Take a screenshot of rendered HTML content |
| `health` | Gotenberg | Check Gotenberg service availability |

**Parameters:**
| Parameter | Required | Description |
|---|---|---|
| `operation` | ✅ | One of the operations above |
| `title` | ❌ | Document title (used by `create_pdf`) |
| `content` | ❌ | Text/HTML/Markdown content (for `html_to_pdf`, `markdown_to_pdf`, `screenshot_html`, or simple `create_pdf`) |
| `url` | ❌ | URL to convert/screenshot (for `url_to_pdf`, `screenshot_url`) |
| `filename` | ❌ | Custom output filename (auto-generated if omitted) |
| `paper_size` | ❌ | Page size: `A4` (default), `A3`, `A5`, `Letter`, `Legal`, `Tabloid` |
| `landscape` | ❌ | `true` for landscape orientation (default: portrait) |
| `sections` | ❌ | JSON array of sections for `create_pdf` (see below) |
| `source_files` | No | JSON array of file paths for `convert_document` and `merge_pdfs` |

**Sections format** (JSON array for `create_pdf`):
```json
[
  {"type": "text", "header": "Introduction", "body": "This is the first section."},
  {"type": "table", "header": "Data", "rows": [["Name", "Value"], ["CPU", "95%"], ["RAM", "4GB"]]},
  {"type": "list", "header": "Tasks", "body": "Item 1\nItem 2\nItem 3"}
]
```

**Backend behavior:**
- **Maroto** (built-in): No external dependencies. Supports `create_pdf` only. Good for simple reports and documents.
- **Gotenberg** (Docker sidecar): Requires Gotenberg container running. Supports all operations including office document conversion and web screenshots. Set backend to `gotenberg` in config to use.
- When backend is `maroto`, only `create_pdf` works. All other operations return an error suggesting to switch to Gotenberg.
- When backend is `gotenberg`, `create_pdf` generates HTML from sections and sends it to Gotenberg for rendering.

**Output:**
- Files are saved to the configured output directory (default: `data/documents/`)
- Documents are **automatically registered in the Media Registry**. You do NOT need to call `media_registry` manually when using this tool.
- Returns the file path and a web-accessible URL at `/files/documents/`

**IMPORTANT — Presenting the download link:**
When the tool returns a `web_path` (e.g. `/files/documents/report.pdf`), always present it to the user as a **clickable Markdown link**:
```
[📄 report.pdf](/files/documents/report.pdf)
```
Never output the `web_path` as bare text — it must be a Markdown link for the user to click it.
