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
| `token_id` | string | for create/update | Webhook token ID to bind to this endpoint. Incoming calls must use this exact token. |

## Examples

**List all webhooks:**
```json
{"action": "manage_webhooks", "operation": "list"}
```

**Get a specific webhook:**
```json
{"action": "manage_webhooks", "operation": "get", "id": "wh_12345"}
```

**Create a new webhook:**
```json
{"action": "manage_webhooks", "operation": "create", "name": "GitHub Push Receiver", "slug": "github-push", "enabled": true, "token_id": "tok_12345"}
```

**Update a webhook:**
```json
{"action": "manage_webhooks", "operation": "update", "id": "wh_12345", "name": "Updated Name", "enabled": false}
```

**Delete a webhook:**
```json
{"action": "manage_webhooks", "operation": "delete", "id": "wh_12345"}
```

**View webhook logs:**
```json
{"action": "manage_webhooks", "operation": "logs", "id": "wh_12345"}
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
https://your-aurago-host/webhook/{slug}
```

The slug is the URL-friendly identifier you provide when creating the webhook.

## Request Logging

All incoming webhook requests are logged with:
- Timestamp
- Webhook ID/name
- Response status
- Source IP
- Payload size
- Delivery success and error text, if any

Logs can be retrieved using the `logs` operation.

## Notes

- **URL slugs**: Must be unique and URL-safe (lowercase, alphanumeric with hyphens)
- **Token authentication**: Webhooks are bound to the configured `token_id`; other webhook-scoped tokens are rejected
- **Read-only mode**: When `webhooks.readonly: true`, create/update/delete operations are blocked
- **Data persistence**: Webhook configurations are stored in `data/webhooks.json`
