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

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | string | **Required.** Operation to perform |
| `file_path` | string | Input PDF file path |
| `output_file` | string | Output file/directory (auto-generated if omitted) |
| `source_files` | string | JSON array of PDF paths for merge |
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
