# Secrets Vault (`secrets_vault`)

Securely store and retrieve sensitive values using AES-256-GCM encryption. **NEVER leak secrets to the outside world.** Vault is encrypted with the `AURAGO_MASTER_KEY` environment variable.

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `get` | Retrieve a secret value | `key` |
| `set` | Store a secret value | `key`, `value` |
| `delete` | Delete a secret | `key` |
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
- **Forbidden exports**: API keys, tokens, and passwords stored in vault are **forbidden to be exported to Python tools**. This prevents accidental exposure.
- **List behavior**: `list` returns only key names, never values — values can only be retrieved with `get`.
- **Key naming**: Use uppercase with underscores (e.g. `GITHUB_TOKEN`, `POSTGRES_PASSWORD`).
- **Deletion**: Once deleted, a secret cannot be recovered. Use `list` first to verify the key exists.