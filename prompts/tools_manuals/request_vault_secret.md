# request_vault_secret

Use `request_vault_secret` when an interactive user must provide an API key,
token, password, or other secret. The client opens a secure masked dialog and
stores the value directly in AuraGo's encrypted Vault. The value is never
returned to you.

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `prompt` | yes | Plain-text explanation shown to the user. Maximum 2,000 characters. |
| `vault_key` | yes | Named reference matching `[A-Z0-9_]{1,64}`. Lowercase input is normalized to uppercase. |
| `replace` | no | Replace an existing value. Defaults to `true`. |

The prefixes `provider_`, `oauth_`, `remote_shared_key_`, and AuraGo's internal
metadata namespace are reserved.

## Results

The result is intentionally value-free:

```json
{"status":"stored","vault_key":"SERVICE_API_KEY","present":true}
```

Cancellation returns `{"status":"cancelled"}`. Timeout, an invalid key,
unavailable client support, and Vault write failures return only a sanitized
error code.

## Security rules

- Never ask the user to paste a secret into chat when this tool is available.
- Never infer, request, repeat, log, remember, or claim to have seen the value.
- A key created through this dialog is user-provided. `secrets_vault get`,
  Python, sandbox, custom-tool, and skill export paths cannot reveal it.
- Refer to the stored credential by `vault_key` or use a server-side
  integration that resolves the key directly.
