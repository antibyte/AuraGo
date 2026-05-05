# Manifest.build Integration

Manifest.build is available as an optional OpenAI-compatible gateway provider in AuraGo.

Use the Manifest provider when:

- The selected provider has `type: manifest`.
- The user wants requests routed through their self-hosted or hosted Manifest gateway.
- `/api/manifest/status` reports the integration as running or configured externally.

Do not assume Manifest is ready just because the config section exists. Check status first when the user asks to diagnose or use the managed gateway.

Useful status endpoint:

```http
GET /api/manifest/status
```

Important fields:

- `status`: `disabled`, `setup_required`, `stopped`, `starting`, `running`, or `unknown`.
- `url`: dashboard/API URL for the user.
- `provider_base_url`: OpenAI-compatible `/v1` endpoint used by AuraGo.
- `admin_setup_required`: true when the user still needs to create the first Manifest admin or save an API key.
- `message`: setup or health detail.

The agent cannot safely create the first Manifest admin account or API key unless the required credentials/API are already available through documented interfaces. Guide the user through the Manifest UI instead.

Secrets are vault-only:

- Manifest API key, usually `mnfst_...`
- Managed Postgres password
- Better Auth secret

Never write these values to `config.yaml`, documentation, logs, reports, or generated tools.
