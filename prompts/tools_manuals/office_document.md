# office_document

Use `office_document` for agent-safe Writer document work inside AuraGo's virtual desktop workspace. It reads and writes `.docx`, `.html`, `.md`, and `.txt` through AuraGo's Go Office backend, so the agent can create and edit documents without an open browser.

The tool requires `tools.office_document.enabled`, `virtual_desktop.enabled`, and `virtual_desktop.allow_agent_control`. Paths are jailed to `virtual_desktop.workspace_dir`; never include secrets or vault values in documents unless the user explicitly asked for that content.

## Operations

- `read`: use `path`; returns `entry`, `document`, and `office_version`.
- `write`: use `path` plus `content`/`title` or a full `document` object.
- `patch`: use `path`, optional seed `content`, `title`, `prepend_text`, `append_text`, and `replacements:[{find,replace}]`.
- `export`: use `path`, `output_path`, and `format` (`docx`, `html`, `md`, or `txt`).

## Examples

```json
{
  "operation": "write",
  "path": "Documents/briefing.docx",
  "title": "Briefing",
  "content": "Summary\nNext steps"
}
```

```json
{
  "operation": "patch",
  "path": "Documents/briefing.docx",
  "replacements": [{"find": "Next steps", "replace": "Action items"}],
  "append_text": "\nOwner: AuraGo"
}
```

Python skills should call this native tool through the Tool Bridge by listing `office_document` in `internal_tools`; do not import DOCX internals directly.
