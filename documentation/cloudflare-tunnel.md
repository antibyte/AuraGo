# Cloudflare Tunnel

AuraGo manages cloudflared in Docker or as a native process. Enable the integration,
save its settings, then use **Start**, **Stop**, or **Restart** in Configuration.
These actions use saved settings and require administrator access; read-only mode
blocks them. Disabling the integration stops its connector synchronously. If Docker
cannot confirm termination, saving that revocation fails and the error remains visible.

## Authentication and routing

- **Token:** put the connector token in the Vault as `cloudflared_token`.
  Configure public hostnames and service origins in the Cloudflare dashboard.
  AuraGo runs `tunnel run` without `--url`. The local target selector controls the
  optional AuraGo loopback listener; it does not rewrite dashboard routing.
- **Named:** store credentials.json in the Vault as `cloudflared_credentials`,
  set `tunnel_name`, and configure `custom_ingress`. Each route needs a hostname
  or path expression and an HTTP(S) origin. Hostnames support a leading `*.`
  wildcard; paths use Go regular expressions. Service URLs must omit credentials,
  paths, query strings and fragments. AuraGo preserves route order and appends
  exactly one `http_status:404` rule. Old automatic Web UI/Homepage rules are no
  longer generated: add explicit public routes before starting a named tunnel.
- **Quick:** select an active registered Homepage project with built static output.
  Both integrations must be enabled. AuraGo copies an immutable snapshot up to
  64 MiB and allocates a private origin port; explicit ports and Web UI fallbacks
  are rejected. The HTTPS TryCloudflare URL is temporary. Docker needs a local
  engine socket or the verified AuraGo parent container network namespace.

For example, named routes in config.yaml:

```yaml
cloudflare_tunnel:
  enabled: true
  auth_method: named
  tunnel_name: home-lab
  custom_ingress:
    - hostname: site.example.com
      service: http://localhost:3000
    - hostname: ui.example.com
      service: http://localhost:18080
```

Configure the corresponding DNS routes in Cloudflare. Set `loopback_port: 18080`
if the second route should use AuraGo's optional loopback listener. Inside a
container, localhost means that container or its shared network namespace; native
host networking and Docker Desktop have different reachability requirements.

## Lifecycle and status

Running verified token/named Docker containers survive an AuraGo crash under
`unless-stopped` and are adopted at startup even when `auto_start` is false.
Stopped verified containers resume only when auto-start is enabled and writable;
manual actions remain available separately. AuraGo checks the reserved container
name, cloudflared image, command and ownership/auth labels. A foreign container is
reported as unverified and is never stopped or replaced. An orphaned quick container
is stopped because its private snapshot origin no longer exists.

Stop uses the original container ID and Docker engine, including after configuration
changes. Stop, start, restart, reconciliation and config revocation serialize.
Restart validates the chosen runtime and quick project before stopping; an
unconfirmed stop prevents another start. A quick listener stays disabled and bound
while termination is uncertain, preventing reuse of its port by another service.

Dashboard and both status APIs inspect the observed connector. Docker errors mean
**unknown**, not stopped. Inspect has a three-second timeout and does not hold the
state lock. A running connector or a captured URL does not prove that Cloudflare
can reach the public service.

Missing `auto_start`, `expose_web_ui` and `expose_homepage` values are false.
The reference template explicitly enables them; copying that template retains
those explicit values. Named routing uses `custom_ingress` independently of the
legacy exposure flags. Quick publication always uses its selected project.

## Ports and TLS

Ports must be integers: zero is the special default value, otherwise 1–65535.
Known conflicts with active HTTP, HTTPS, redirect, internal loopback, Homepage and
metrics listeners are rejected on load/save/start. Ports below 1024 are allowed but
may require OS privileges; unknown external listeners can still cause bind failures.

With `loopback_port: 0`, no extra Cloudflare listener is selected; HTTPS internal
self-calls may still use server.port. With `metrics_port: 0`, cloudflared gets
`--metrics localhost:0`: the OS chooses a loopback port. It does **not** disable
metrics. A positive value chooses a fixed loopback metrics port.

For a self-signed AuraGo HTTPS origin, prefer a plain HTTP loopback origin. Token
TLS auto-configuration needs `cloudflare_api_token`, an account ID and tunnel ID
or name. Missing prerequisites and lookup/GET/PUT/timeout failures add a sanitized
warning (`tls_prerequisites`, `tls_lookup`, `tls_get`, `tls_put`, `tls_timeout`,
`tls_origin`) to start/restart and subsequent status. They do not prevent the
connector from running, and do not prove public reachability. Configure loopback
HTTP or correct the origin manually when a warning remains.

Automatic `noTLSVerify` applies only to the exact local AuraGo HTTPS origin;
other services retain certificate verification. No global TLS exception is written.

## Installation and secrets

Managed Docker uses cloudflared **2026.10.0** pinned by manifest digest. Automatic
native installation supports Linux amd64/arm64 only and checks the fixed SHA-256
metadata from the [official release](https://github.com/cloudflare/cloudflared/releases/tag/2026.10.0).
Downloads are bounded to 100 MiB and installations serialize. A synced, closed
temporary binary replaces the old binary atomically; failures keep the old file.
Windows/macOS require a manually installed binary in AuraGo's bin directory or PATH.

Docker administrators can inspect container environment variables, including the
connector token, and read mounted named credentials. Vault storage protects secrets
at rest in AuraGo; it does not hide runtime secrets from Docker administrators.
Restrict engine access. AuraGo passes token credentials via environment rather than
process command-line arguments and registers secrets for output scrubbing.

## API

`POST /api/cloudflare-tunnel/start`, `/stop`, `/restart` use saved settings and the
existing administrator, CSRF and read-only policies. Existing response envelopes
are retained. Status adds `state_known`, `auth_method`, `warnings` and, when relevant,
`publication_disabled`; `running` is null when Docker state cannot be observed.

Local fixtures and browser checks cover lifecycle and UI behavior. A real Cloudflare
account, DNS/Access configuration and public origin reachability still need provider
acceptance; these checks do not deploy or restart a production instance.
