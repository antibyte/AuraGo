## Tool: Smart File Read (`smart_file_read`)

Inspect large files intelligently without dumping the whole file into the prompt. Use this when `filesystem.read_file` would be too blunt or when you first need an overview before targeted reads.

### Operations

| Operation | Description | Required Parameters |
|---|---|---|
| `analyze` | Return file metadata, size, line count, MIME/type hints, and recommended next steps | `file_path` |
| `sample` | Return a representative sample of the file | `file_path` |
| `structure` | Detect likely structure such as JSON, XML, CSV, or plain text | `file_path` |
| `summarize` | Generate a focused summary using the summary LLM path | `file_path` |

### Parameters

| Parameter | Description |
|---|---|
| `file_path` | File path relative to `agent_workspace/workdir` (use `../../` for project-root files) |
| `query` | Optional focus question for `summarize` |
| `line_count` | Number of lines per sample section for `sample` (default 20) |
| `sampling_strategy` | `head`, `tail`, `distributed`, or `semantic` |
| `max_tokens` | Approximate token budget for `sample` / `summarize` |

### Behavior

- `distributed` sampling reads representative sections from the beginning, middle, and end of a text file.
- `semantic` currently falls back to `distributed`.
- `summarize` reads the full file only when it fits comfortably; otherwise it summarizes representative samples.
- Homepage projects should still use the `homepage` tool instead of the generic file tools.
- For binary files, use a specialized tool instead:
  - images → `analyze_image`
  - PDFs → `pdf_extractor`
  - unknown/binary → `detect_file_type`

### Examples

```json
{"action": "smart_file_read", "operation": "analyze", "file_path": "server.log"}
```

```json
{"action": "smart_file_read", "operation": "sample", "file_path": "server.log", "sampling_strategy": "distributed", "line_count": 25}
```

```json
{"action": "smart_file_read", "operation": "structure", "file_path": "../../documentation/report.csv"}
```

```json
{"action": "smart_file_read", "operation": "summarize", "file_path": "logs/app.log", "query": "Find the most likely root cause of the repeated 500 errors", "max_tokens": 1800}
```

### Tips

- Use `analyze` first if you do not yet know how large or structured the file is.
- After `sample` or `summarize`, switch to `file_reader_advanced read_lines` or `search_context` for precise follow-up reads.
- Use `structure` before editing large JSON/XML/CSV files so you know what you are dealing with.
