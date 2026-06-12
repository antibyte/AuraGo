# read_tool_output

Use `read_tool_output` when a compact tool result includes an `output_ref` and you need more detail without adding the entire raw output to context.

## Parameters

- `ref` (required): The `output_ref` from the compact result.
- `view`: `summary`, `head`, `tail`, `range`, `grep`, `jsonpath`, or `full`.
- `query`: Search text for `grep` or a JSONPath expression for `jsonpath`.
- `start_line`, `end_line`: 1-based line numbers for `range`.
- `max_lines`: Line limit for `head` or `tail`.
- `max_chars`: Character cap for any view. If omitted, output is capped to a compact default; very large requested caps are limited.
- `reason`: Optional note explaining why more output is needed.

Prefer focused views (`grep`, `range`, `jsonpath`, `head`, `tail`) before `full`.
