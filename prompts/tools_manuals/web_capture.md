# Tool: `web_capture`

## Purpose
Take a screenshot (PNG) or render a PDF of any web page using an embedded headless Chromium browser. This tool does **not** require Gotenberg or any external Docker sidecar — it uses the same go-rod browser engine already used by the web scraper.

## When to Use
- Save a visual snapshot of a web page for documentation or reference.
- Render a web-based dashboard or report as a PDF.
- Capture proof of a web page's current state.
- Generate a PDF from an online article, invoice, or HTML page.
- Preferred over `document_creator` (url_to_pdf / screenshot_url) when Gotenberg is not running.

## Parameters
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | ✅ | `"screenshot"` (PNG image) or `"pdf"` (PDF document) |
| `url` | string | ✅ | Full HTTP/HTTPS URL to capture |
| `selector` | string | ❌ | CSS selector to wait for before capture (ensures dynamic content is loaded) |
| `full_page` | boolean | ❌ | Capture the full scrollable page (screenshot only; default: `false`) |
| `output_dir` | string | ❌ | Directory where file is saved (default: `agent_workspace/workdir`) |

## Output
JSON with these fields:
- `status` — `"success"` or `"error"`
- `operation` — echo of the requested operation
- `file` — absolute path to the saved file
- `message` — human-readable confirmation or error description

## Behaviour Notes
- Output filenames are auto-generated from the hostname and a timestamp (e.g. `screenshot_example.com_20260115_143022.png`).
- The tool waits for `WaitLoad` (network idle) before capture. Use `selector` for SPA pages that render content after the initial load event.
- `full_page` only applies to `screenshot`; for `pdf` the browser always renders the full document.
- Requires Chromium to be installed on the host or available for auto-download by go-rod.

## Example Calls
```json
{ "operation": "screenshot", "url": "https://grafana.example.com/d/myboard", "selector": ".panel-container", "full_page": true }
{ "operation": "pdf", "url": "https://invoice.example.com/inv/12345", "output_dir": "data/documents" }
```
