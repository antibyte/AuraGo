## Tool: PDF Extractor

Extract plain text content from a PDF file. This is a built-in tool — no Python required.

When **summary mode** is enabled, the extracted text is automatically sent to a
separate summarisation model before being returned to you. In this mode you
**must** include the `search_query` parameter so the summariser knows which
information to extract from the document.

### Usage (normal mode — full text returned)

```json
{"action": "execute_skill", "skill": "pdf_extractor", "skill_args": {"filepath": "report.pdf"}}
```

### Usage (summary mode — only a focused summary is returned)

```json
{"action": "execute_skill", "skill": "pdf_extractor", "skill_args": {"filepath": "report.pdf", "search_query": "What are the financial projections for Q3?"}}
```

### Parameters
- `filepath` (string, required): Path to the PDF file (relative to workspace workdir, or absolute).
- `search_query` (string, optional but **required in summary mode**): Tell the summarisation model exactly what information you are looking for in the document. Be specific — e.g. "revenue figures and growth trends" rather than just "summary".

### Notes
- Image-based PDFs (scanned documents without OCR) cannot be extracted — the tool will report that no text was found.
- The file must be accessible within the project tree (path traversal outside the project is blocked).
