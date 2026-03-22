---
tool: pdf_operations
version: 1
tags: ["always"]
---

# PDF Operations Tool

Manipulate PDF files locally without external services.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `merge` | Combine multiple PDFs into one | `source_files` (JSON array), `output_file` |
| `split` | Split PDF into individual pages or at specific page numbers | `file_path`, optionally `pages`, `output_file` (directory) |
| `watermark` | Add diagonal semi-transparent text watermark | `file_path`, `watermark_text` |
| `compress` | Optimize/reduce PDF file size | `file_path` |
| `encrypt` | Password-protect with AES-256 | `file_path`, `password` |
| `decrypt` | Remove password protection | `file_path`, `password` |
| `metadata` | Read page count, properties, keywords | `file_path` |
| `page_count` | Get number of pages | `file_path` |
| `form_fields` | List all form fields in a PDF | `file_path` |
| `fill_form` | Fill PDF form fields with values | `file_path`, `source_files` (JSON object) |
| `export_form` | Export form data to JSON file | `file_path` |
| `reset_form` | Reset all form fields to defaults | `file_path` |
| `lock_form` | Lock all form fields (read-only) | `file_path` |

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | string | **Required.** Operation to perform |
| `file_path` | string | Input PDF file path |
| `output_file` | string | Output file/directory (auto-generated if omitted) |
| `source_files` | string | JSON array of PDF paths for merge, or JSON object `{"field":"value"}` for fill_form |
| `pages` | string | Comma-separated page numbers for split |
| `watermark_text` | string | Text for watermark overlay |
| `password` | string | Password for encrypt/decrypt |

## Examples

Merge two PDFs:
```json
{"operation": "merge", "source_files": "[\"report_q1.pdf\",\"report_q2.pdf\"]", "output_file": "combined_report.pdf"}
```

Split into individual pages:
```json
{"operation": "split", "file_path": "document.pdf", "output_file": "./pages/"}
```

Add watermark:
```json
{"operation": "watermark", "file_path": "contract.pdf", "watermark_text": "CONFIDENTIAL"}
```

Compress a large PDF:
```json
{"operation": "compress", "file_path": "large_scan.pdf"}
```

Encrypt with password:
```json
{"operation": "encrypt", "file_path": "sensitive.pdf", "password": "secure123"}
```

Get metadata:
```json
{"operation": "metadata", "file_path": "document.pdf"}
```

List form fields:
```json
{"operation": "form_fields", "file_path": "application.pdf"}
```

Fill a PDF form:
```json
{"operation": "fill_form", "file_path": "application.pdf", "source_files": "{\"first_name\":\"John\",\"last_name\":\"Doe\",\"email\":\"john@example.com\"}", "output_file": "filled_application.pdf"}
```

Export form data to JSON:
```json
{"operation": "export_form", "file_path": "filled_form.pdf", "output_file": "form_data.json"}
```

Lock form fields (make read-only):
```json
{"operation": "lock_form", "file_path": "filled_form.pdf"}
```
