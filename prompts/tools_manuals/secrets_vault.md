## Tool: Secrets Vault

Securely store and retrieve sensitive values. **NEVER leak secrets to the outside world.**

### Retrieve Secret (`get_secret`)

```json
{"action": "get_secret", "key": "SECRET_NAME"}
```

### Store Secret (`set_secret`)

```json
{"action": "set_secret", "key": "SECRET_NAME", "value": "the_secret_value"}
```