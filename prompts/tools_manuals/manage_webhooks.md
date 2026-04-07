# Webhooks (`manage_webhooks`, `webhook`)

Manage incoming webhook endpoints for AuraGo. Create, list, update, delete webhooks and view their request logs.

## Operations

| Operation | Description |
|-----------|-------------|
| `list` | List all configured incoming webhooks |
| `get` | Get details of a specific webhook by ID |
| `create` | Create a new incoming webhook endpoint |
| `update` | Update an existing webhook |
| `delete` | Delete a webhook by ID |
| `logs` | View recent request logs for a webhook |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | yes | One of: list, get, create, update, delete, logs |
| `id` | string | for get/update/delete/logs | Webhook ID |
| `name` | string | for create/update | Human-readable webhook name |
| `slug` | string | for create | URL slug for the webhook endpoint (e.g. `github-push`) |
| `enabled` | boolean | for create/update | Enable/disable the webhook |
| `token_id` | string | for create/update | Authentication token ID to associate |

## Examples

**List all webhooks:**
```json
{"action": "manage_webhooks", "action": "list"}
```

**Get a specific webhook:**
```json
{"action": "manage_webhooks", "action": "get", "id": "wh_12345"}
```

**Create a new webhook:**
```json
{"action": "manage_webhooks", "action": "create", "name": "GitHub Push Receiver", "slug": "github-push", "enabled": true, "token_id": "github-main"}
```

**Update a webhook:**
```json
{"action": "manage_webhooks", "action": "update", "id": "wh_12345", "name": "Updated Name", "enabled": false}
```

**Delete a webhook:**
```json
{"action": "manage_webhooks", "action": "delete", "id": "wh_12345"}
```

**View webhook logs:**
```json
{"action": "manage_webhooks", "action": "logs", "id": "wh_12345"}
```

## Configuration

```yaml
webhooks:
  enabled: true
  readonly: false  # Set true to block create/update/delete operations
  incoming:
    # Incoming webhooks are defined in data/webhooks.json
    # Use the create operation to add new webhooks
```

## Webhook URL Format

Incoming webhooks are accessible at:
```
https://your-aurago-host/api/webhooks/{slug}
```

The slug is the URL-friendly identifier you provide when creating the webhook.

## Request Logging

All incoming webhook requests are logged with:
- Timestamp
- HTTP method and headers
- Request body
- Response status

Logs can be retrieved using the `logs` operation.

## Notes

- **URL slugs**: Must be unique and URL-safe (lowercase, alphanumeric with hyphens)
- **Token authentication**: Webhooks can be associated with token IDs for authentication
- **Read-only mode**: When `webhooks.readonly: true`, create/update/delete operations are blocked
- **Data persistence**: Webhook configurations are stored in `data/webhooks.json`
