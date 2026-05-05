# Manifest.build Integration

AuraGo can use Manifest.build as an OpenAI-compatible LLM gateway. The integration supports two modes:

- **Managed:** AuraGo starts a Manifest container plus a private PostgreSQL container through Docker.
- **External:** AuraGo connects to an existing hosted or self-hosted Manifest endpoint.

## Managed Mode

Managed mode uses:

- Manifest image: `manifestdotbuild/manifest:5`
- PostgreSQL image: `postgres:15-alpine`
- Manifest port: `2099`
- Postgres volume: `aurago_manifest_pgdata`
- Internal Docker network: `aurago_manifest`

Only the Manifest dashboard/API port is published. PostgreSQL is kept on the private Docker network and is not published to the host.

Required runtime secrets are stored only in the AuraGo vault:

- Manifest API key, usually starting with `mnfst_`
- PostgreSQL password
- Better Auth secret

Do not place these values in `config.yaml`.

## First Admin And API Key

Manifest requires the first admin account to be created in its own UI.

1. Enable Manifest in AuraGo config.
2. Save a PostgreSQL password and Better Auth secret in the vault.
3. Start the managed sidecars.
4. Open the Manifest dashboard URL from the status panel.
5. Create the first admin account.
6. Create or copy an API key from Manifest.
7. Save the `mnfst_...` API key in AuraGo's vault.
8. Add or select a provider with `type: manifest`.

AuraGo does not automate first-admin creation because that would depend on Manifest internals rather than the documented self-hosting contract.

## Provider Example

```yaml
providers:
  - id: manifest
    type: manifest
    name: "Manifest Gateway"
    model: "manifest/auto"

llm:
  provider: manifest
```

The API key belongs in the vault, not in YAML.

## External Mode

Use external mode when Manifest is already running somewhere else.

Set `manifest.external_base_url` to the OpenAI-compatible `/v1` URL, for example:

```yaml
manifest:
  enabled: true
  mode: external
  external_base_url: "https://manifest.example.com/v1"
```

## Tailscale Exposure

If tsnet is enabled, AuraGo can expose the managed Manifest dashboard/API through the embedded Tailscale node. This is optional and should be enabled only for trusted tailnets.

Do not expose Manifest publicly without authentication and HTTPS.

## Backup

Managed Manifest data lives in the PostgreSQL Docker volume. A simple backup can be created with:

```bash
docker exec aurago_manifest_postgres pg_dump -U manifest manifest > manifest-backup.sql
```

Restore and upgrade workflows should be tested before changing the major Manifest image tag.

## Upgrades

The default image is pinned to `manifestdotbuild/manifest:5`. Keep the major version pinned unless you intentionally want to test a major upgrade. Before upgrading:

- Back up the Postgres volume.
- Read Manifest release notes.
- Restart the managed sidecars after changing the image.

## Troubleshooting

Use `/api/manifest/status` or the Config UI status panel.

Common statuses:

- `disabled`: integration is off.
- `setup_required`: vault secrets or API key setup are missing.
- `stopped`: containers are not running.
- `starting`: Docker reports the container is starting or health is not confirmed yet.
- `running`: Manifest is reachable.
