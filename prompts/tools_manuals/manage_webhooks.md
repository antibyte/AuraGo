# Webhooks (`manage_webhooks`, `manage_outgoing_webhooks`, `call_webhook`)

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

Authenticate incoming requests with `Authorization: Bearer <token>` whenever the provider supports custom headers. The compatibility form `?token=<token>` remains available for restricted providers, but URLs can be recorded in access logs, browser history, and intermediary systems.

Incoming JSON is validated from its media type and isolated before it reaches the agent. Delivery is asynchronous. `webhooks.rate_limit` is a per-token token bucket measured in requests per minute; its capacity also defines the allowed burst.

## Outgoing Webhooks

`manage_outgoing_webhooks` lists, creates, updates, and deletes outgoing definitions. `call_webhook` invokes a configured definition. Allowed methods are `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, and `OPTIONS`; any other method is rejected before network access.

Outgoing URLs, custom body templates, and sensitive headers are Vault-only. List results return these values as `••••••••`; use that same mask with a stable webhook ID to preserve an existing value. Non-sensitive headers remain visible. Never place an outgoing secret directly in `config.yaml`.

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
- **Signature validation**: Header, algorithm (`sha256`, `sha1`, or `plain`), and Vault secret must be configured together; missing runtime secrets fail closed
- **Read-only mode**: When `webhooks.readonly: true`, create/update/delete operations are blocked
- **Data persistence**: Webhook configurations are stored in `data/webhooks.json`
