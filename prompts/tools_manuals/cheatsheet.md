# Cheatsheet Tool (`cheatsheet`)

Manage reusable workflow instructions (cheat sheets) that describe step-by-step procedures.

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `list` | List all cheat sheets | — |
| `get` | View a cheat sheet | `id` or `name` |
| `create` | Create a new cheat sheet | `name`, `content`, optional `abstract`, `tags` |
| `update` | Update an existing cheat sheet | `id`, optional `name`, `content`, `abstract`, `tags`, `active`, `delete_locked` |
| `delete` | Delete a cheat sheet | `id` |
| `attach` | Attach a text file to a cheat sheet | `id`, `filename`, `content`, `source` |
| `detach` | Remove an attachment from a cheat sheet | `id`, `attachment_id` |

## Field Notes

- `tags` is an array of short strings. Send `[]` on `update` to clear all tags.
- On `update`, sending `content: ""` clears content and `abstract: ""` clears the abstract.
- `active: false` deactivates a sheet and removes it from semantic retrieval.
- `delete_locked: true` prevents deletion until it is set back to false.
- Cheat sheet content is limited to 100,000 characters.
- Attachments must be `.txt` or `.md`; total attachment text per sheet is limited to 25,000 characters.
- `attach.source` may be `upload` or `knowledge`; if omitted, `upload` is used.

## Examples

```json
{"action": "cheatsheet", "operation": "list"}
```

```json
{"action": "cheatsheet", "operation": "create", "name": "Deploy Docker Stack", "content": "# Deploy\n1. Pull latest images\n2. Run docker-compose up -d\n3. Check logs", "tags": ["docker", "deploy"]}
```

```json
{"action": "cheatsheet", "operation": "get", "id": "deploy-docker-stack"}
```

```json
{"action": "cheatsheet", "operation": "update", "id": "cs_123", "active": false, "tags": []}
```

```json
{"action": "cheatsheet", "operation": "attach", "id": "cs_123", "filename": "notes.md", "content": "# Notes\nSome extra context.", "source": "upload"}
```

```json
{"action": "cheatsheet", "operation": "detach", "id": "cs_123", "attachment_id": "att_456"}
```
