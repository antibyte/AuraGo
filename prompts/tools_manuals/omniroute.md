# OmniRoute Integration

OmniRoute is available as an optional OpenAI-compatible gateway provider with provider type `omniroute`.

## Configuration

Enable it under `omniroute:`. In managed mode AuraGo starts one Docker sidecar using `diegosouzapw/omniroute:3.8.39`, publishes port `20128`, and persists `/app/data` in the named Docker volume `aurago_omniroute_data`.

Use external mode when an existing OmniRoute instance is already running. In that case `omniroute.external_base_url` must point at the OpenAI-compatible `/v1` endpoint.

## Provider Setup

Create a provider entry with:

```yaml
providers:
  - id: omniroute
    type: omniroute
    name: "OmniRoute Gateway"
    model: auto
```

Do not manually type a provider `base_url` for managed OmniRoute. AuraGo resolves it from the `omniroute:` settings.

## Vault Secrets

These secrets are vault-only and must not be written to `config.yaml`:

- `omniroute_api_key`: API key AuraGo uses for OpenAI-compatible requests to OmniRoute.
- `omniroute_initial_password`: initial OmniRoute admin password required before the first managed start.
- `omniroute_jwt_secret`: generated backend session secret.
- `omniroute_api_key_secret`: generated backend API-key secret.
- `omniroute_ws_bridge_secret`: generated websocket bridge secret.

AuraGo can auto-generate the three backend secrets, but the user must provide the initial admin password before starting managed OmniRoute for the first time.

## API Controls

The Config UI and REST API expose:

- `GET /api/omniroute/status`
- `POST /api/omniroute/test`
- `POST /api/omniroute/start`
- `POST /api/omniroute/stop`

Managed start/stop affect only the OmniRoute sidecar container. The persistent Docker volume is not deleted.

## Safety

OmniRoute remains a local gateway endpoint and is not routed through Cloudflare AI Gateway. API keys and generated secrets are forbidden from Python tool export.
