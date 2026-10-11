# read_tool_output

Use `read_tool_output` when a tool result was archived because it was too large ("Full output archived", with an `output_ref` and a view of the output) and the part you need is not in that view. The archive message names the exact call for the rest: copy its `ref` and its `offset` or `start_line`/`end_line`.

## Parameters

- `ref` (required): the `output_ref` value of the archived result, for example `toolout_3f2a…`. The name `output_ref` is accepted too.
- `view`: `full` (default), `grep`, `jsonpath`, `range`, `head`, `tail`, or `summary`.
- `offset`: byte position to continue from. Every page that stops before the end returns `next_offset`; pass it as `offset` for the next page.
- `query`: the search text for `grep`, or a JSONPath expression such as `$.items[0].name` for `jsonpath`.
- `start_line`, `end_line`: 1-based line numbers for `range`.
- `max_lines`: line count for `head` or `tail`.
- `max_chars` (alias `limit`): a smaller page size. Every page fits the inline output budget anyway.
- `reason`: optional note explaining why more output is needed.

## Choosing a view

- A search target (a name, ID, error text): `grep`. Matching lines come back whole; in a long single-line output (minified JSON or HTML) each match comes with about 200 bytes of context and its byte position.
- A field of a JSON result: `jsonpath`, for example `$.containers[3].status`. It reads the tool's own JSON.
- Omitted log lines: `range` with the line numbers from the archive message. `tail` returns the end of the output.
- The rest of a long output: `full` with `offset` set to the archive message's offset, then `next_offset` until it is absent.

Views and offsets refer to the tool's own text, without the "Tool Output:" prefix or the external-data escaping.
