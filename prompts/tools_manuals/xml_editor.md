# XML Editor (`xml_editor`)

Read, modify, and validate XML files using XPath expressions. Supports element text editing, attribute manipulation, element creation/deletion, and formatting.

## Operations

| Operation | Description |
|-----------|-------------|
| `get` | Read elements matching an XPath expression |
| `set_text` | Set the text content of matching elements |
| `set_attribute` | Set an attribute on matching elements |
| `add_element` | Add a child element to matching parents |
| `delete` | Remove elements matching the XPath |
| `validate` | Check if the file is valid XML |
| `format` | Pretty-print the XML file with 2-space indent |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `file_path` | string | yes | Path to the XML file (relative to workdir/) |
| `xpath` | string | for get, set_text, set_attribute, add_element, delete | XPath expression |
| `set_value` | string/object | for set_text, set_attribute, add_element | Value to set |

## XPath Syntax

Uses etree path syntax (simplified XPath):
- `//server` — find all `<server>` elements anywhere
- `./config/database` — find `<database>` under `<config>` root
- `//server[@name='main']` — find `<server>` with attribute `name="main"`
- `.//item` — find all `<item>` descendants

## Examples

**Read elements:**
```json
{"action": "xml_editor", "operation": "get", "file_path": "config.xml", "xpath": "//database/host"}
```

**Set text content:**
```json
{"action": "xml_editor", "operation": "set_text", "file_path": "config.xml", "xpath": "//database/port", "set_value": "5433"}
```

**Set an attribute:**
```json
{"action": "xml_editor", "operation": "set_attribute", "file_path": "pom.xml", "xpath": "//dependency[@groupId='org.example']", "set_value": {"name": "version", "value": "2.0.0"}}
```

**Add a new element:**
```json
{"action": "xml_editor", "operation": "add_element", "file_path": "config.xml", "xpath": "//servers", "set_value": {"tag": "server", "text": "", "attributes": {"name": "backup", "host": "10.0.0.2"}}}
```

**Delete elements:**
```json
{"action": "xml_editor", "operation": "delete", "file_path": "config.xml", "xpath": "//database/debug"}
```

**Validate XML:**
```json
{"action": "xml_editor", "operation": "validate", "file_path": "data.xml"}
```

## Notes

- `get` returns element tag, text, attributes, and child tags for each match
- `set_text` and `set_attribute` apply to **all** matching elements
- `add_element` appends the new child to **each** matching parent
- All writes are atomic (temp file + rename) to prevent corruption
- Use `validate` before editing to ensure the file is well-formed
