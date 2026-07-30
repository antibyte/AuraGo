# Secrets Vault (`secrets_vault`)

Securely store and retrieve model-created values using AES-256-GCM encryption. **NEVER leak secrets to the outside world.** Vault is encrypted with the `AURAGO_MASTER_KEY` environment variable. User-provided, UI, system, and legacy values remain hidden from the agent.

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `get` | Retrieve a model-created value; protected values return presence only | `key` |
| `set` | Store a model-created value | `key`, `value` |
| `delete` | Delete a model-created value | `key` |
| `list` | List all secret keys (values never exposed) | — |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of: `get`, `set`, `delete`, `list` |
| `key` | string | for get/set/delete | Secret key name (e.g. `GITHUB_TOKEN`, `MYSQL_PASSWORD`) |
| `value` | string | for set | Secret value to store |

## Examples

**Retrieve a secret:**
```json
{"action": "secrets_vault", "operation": "get", "key": "GITHUB_TOKEN"}
```

**Store a secret:**
```json
{"action": "secrets_vault", "operation": "set", "key": "MY_API_KEY", "value": "secret123"}
```

**Delete a secret:**
```json
{"action": "secrets_vault", "operation": "delete", "key": "OLD_TOKEN"}
```

**List all secret keys:**
```json
{"action": "secrets_vault", "operation": "list"}
```

## Configuration

No config.yaml changes needed. Vault is enabled by default when `AURAGO_MASTER_KEY` is set.

```yaml
# Vault is auto-enabled when AURAGO_MASTER_KEY environment variable is set
# No explicit config required
```

## Notes

- **Security**: Secrets are encrypted at rest with AES-256-GCM. The master key never leaves the server.
- **User input**: Use `request_vault_secret` instead of asking the user to paste a value into chat. The secure-dialog value is never shown to the agent.
- **Provenance protection**: Only values created by the model through `secrets_vault set` can be read, replaced, deleted, or exported by agent paths. UI, modal, system, and unclassified legacy values expose only key and presence.
- **List behavior**: `list` returns only key names, never values.
- **Key naming**: Use uppercase with underscores (e.g. `GITHUB_TOKEN`, `POSTGRES_PASSWORD`).
- **Deletion**: Once deleted, a secret cannot be recovered. Use `list` first to verify the key exists.
