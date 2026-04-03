# text_diff — Text and File Differences

Compare text files or strings to see exact insertions and deletions in unified diff format.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `compare_files` | Compare two files on disk | `file_path_a`, `file_path_b` |
| `compare_text` | Compare two text strings | `text_a`, `text_b` |

## Key Behaviors

- Returns standard unified diff format (like `git diff`).
- Useful for verifying what changed after a script ran or checking two versions of a config.
- Automatically handles line endings (CRLF vs LF) to prevent noisy diffs.

## Examples

```
# Compare two files
text_diff(operation="compare_files", file_path_a="config.yaml", file_path_b="config.yaml.bak")

# Compare before/after text
text_diff(operation="compare_text", text_a="old text content", text_b="new text content")
```

## Tips
- Keep outputs small by only diffing relevant files.
- Excellent for creating patches or verifying that a `replace_string` operation worked as intended.