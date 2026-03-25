## Tool: XML Editor (`xml_editor`)

Read, modify, and validate XML files using XPath expressions. Supports element text editing, attribute manipulation, element creation/deletion, and formatting.

### Operations

| Operation | Description | Required Parameters |
|---|---|---|
| `get` | Read elements matching an XPath expression | `xpath` |
| `set_text` | Set the text content of matching elements | `xpath`, `set_value` (string) |
| `set_attribute` | Set an attribute on matching elements | `xpath`, `set_value` ({name, value}) |
| `add_element` | Add a child element to matching parents | `xpath`, `set_value` ({tag, text?, attributes?}) |
| `delete` | Remove elements matching the XPath | `xpath` |
| `validate` | Check if the file is valid XML | — |
| `format` | Pretty-print the XML file with 2-space indent | — |

All operations require `file_path` (relative to `workdir/`).

### XPath Syntax

Uses etree path syntax (simplified XPath):
- `//server` — find all `<server>` elements anywhere
- `./config/database` — find `<database>` under `<config>` root
- `//server[@name='main']` — find `<server>` with attribute `name="main"`
- `.//item` — find all `<item>` descendants

### Examples

```json
{"action": "xml_editor", "operation": "get", "file_path": "config.xml", "xpath": "//database/host"}
```

```json
{"action": "xml_editor", "operation": "set_text", "file_path": "config.xml", "xpath": "//database/port", "set_value": "5433"}
```

```json
{"action": "xml_editor", "operation": "set_attribute", "file_path": "pom.xml", "xpath": "//dependency[@groupId='org.example']", "set_value": {"name": "version", "value": "2.0.0"}}
```

```json
{"action": "xml_editor", "operation": "add_element", "file_path": "config.xml", "xpath": "//servers", "set_value": {"tag": "server", "text": "", "attributes": {"name": "backup", "host": "10.0.0.2"}}}
```

```json
{"action": "xml_editor", "operation": "delete", "file_path": "config.xml", "xpath": "//database/debug"}
```

```json
{"action": "xml_editor", "operation": "validate", "file_path": "data.xml"}
```

### Tips

- `get` returns element tag, text, attributes, and child tags for each match.
- `set_text` and `set_attribute` apply to **all** matching elements.
- `add_element` appends the new child to **each** matching parent.
- Use `validate` before editing to ensure the file is well-formed.
- All writes are atomic (temp file + rename) to prevent corruption.
