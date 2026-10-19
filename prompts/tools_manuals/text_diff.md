# text_diff — Text and File Differences

Compare two files or text strings and return a unified diff. File paths obey the configured workspace restrictions. This tool reads inputs and does not apply patches.

## Operations

| Operation | Required fields |
| --- | --- |
| `diff_files` | `file1`, `file2` |
| `diff_strings` | `text1`, `text2` |

## Examples

```json
{"action":"text_diff","operation":"diff_files","file1":"config.yaml","file2":"config.yaml.bak"}
```

```json
{"action":"text_diff","operation":"diff_strings","text1":"old text content","text2":"new text content"}
```

Keep inputs focused on the relevant change. Inspect the returned diff before deciding whether another write is needed. Do not use `compare_files`, `compare_text`, `file_path_a`, `file_path_b`, `text_a`, or `text_b`; those names are not part of this tool's schema.
