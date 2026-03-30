# Cheatsheet Tool (`cheatsheet`)

Manage reusable workflow instructions (cheat sheets) that describe step-by-step procedures.

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `list` | List all cheat sheets | — |
| `get` | View a cheat sheet | `id` or `name` |
| `create` | Create a new cheat sheet | `name`, `content` |
| `update` | Update an existing cheat sheet | `id`, `name`, `content`, `active` |
| `delete` | Delete a cheat sheet | `id` |
| `attach` | Attach a text file to a cheat sheet | `id`, `filename`, `content`, `source` |
| `detach` | Remove an attachment from a cheat sheet | `id`, `attachment_id` |

## Examples

```json
{"action": "cheatsheet", "operation": "list"}
```

```json
{"action": "cheatsheet", "operation": "create", "name": "Deploy Docker Stack", "content": "# Deploy\n1. Pull latest images\n2. Run docker-compose up -d\n3. Check logs"}
```

```json
{"action": "cheatsheet", "operation": "get", "id": "deploy-docker-stack"}
```

```json
{"action": "cheatsheet", "operation": "update", "id": "1", "active": false}
```

```json
{"action": "cheatsheet", "operation": "attach", "id": "1", "filename": "notes.md", "content": "# Notes\nSome extra context."}
```

```json
{"action": "cheatsheet", "operation": "detach", "id": "1", "attachment_id": "abc-123"}
```
