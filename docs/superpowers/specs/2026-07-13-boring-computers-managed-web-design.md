# Managed Boring Computers Web UI

## Goal

When `virtual_computers.enabled` is true, AuraGo installs and operates the complete self-hosted Boring Computers stack: the `boringd` control plane and the upstream `apps/web` management application. The right-hand chat integrations drawer shows a **Boring Computers** entry only while the integration is enabled. Opening the entry loads the authenticated management application at `/boring-computers/`.

The user must not need to install Node.js, build the web application, create services, expose ports, or manage SSH tunnels manually.

## Non-goals

- Do not replace or fork the upstream Boring Computers user experience inside AuraGo.
- Do not expose `boringd`, its bearer token, or the management service directly to the network.
- Do not make the public `boringcomputers.com` showcase the management target.
- Do not require Docker build access or a user-managed container.
- Do not remove AuraGo's existing Virtual Computers desktop application or API.

## Architecture

AuraGo extends the existing Virtual Computers setup manager. The selected control-plane host continues to run `boringd` and additionally runs the upstream `apps/web` application as a systemd service.

The managed services use fixed loopback endpoints:

- `boringd`: the configured private URL, normally `http://127.0.0.1:18080`
- Boring Computers web application: `http://127.0.0.1:18081`

The web application is built from an AuraGo-pinned, reviewed upstream revision. AuraGo applies a small, deterministic compatibility overlay during installation so the upstream application:

1. uses `/boring-computers` as its public base path;
2. sends REST and WebSocket traffic through `/boring-computers/boring`;
3. supports the token-injecting proxy in the production preview service as well as during development; and
4. never embeds `BORING_TOKEN` or a private `boringd` URL in browser assets.

The compatibility overlay is version-coupled to the pinned upstream revision. Setup must fail with an actionable compatibility error if the expected upstream files no longer match instead of applying an unsafe partial patch.

AuraGo exposes `/boring-computers/` through its existing authenticated HTTP server. A reverse proxy forwards HTTP and WebSocket upgrades to the loopback management endpoint. The public browser never connects directly to the management port or `boringd`.

## Installation and Lifecycle

The current idempotent setup and repair flow remains the single entry point. It performs these additional steps after the existing host preflight succeeds:

1. Install the pinned Node.js/npm runtime required by the reviewed upstream revision.
2. Clone or update the Boring Computers source to AuraGo's pinned revision.
3. Apply and verify the AuraGo compatibility overlay.
4. Install locked npm dependencies with the upstream lockfile.
5. Build `apps/web` with an empty `PUBLIC_BORING_URL` so the browser uses the private proxy path.
6. Write a root-readable-only environment file containing `BORING_URL` and `BORING_TOKEN`.
7. Install and enable `boring-web.service`, bound to `127.0.0.1:18081` with a strict port requirement.
8. Restart the service and verify both `boringd` and the management page.

Installation is staged. A failed dependency install, build, overlay validation, service start, or health check leaves the last known-good management build and service definition in place. Temporary build directories and logs are removed before setup returns.

`repair` repeats the same idempotent process. AuraGo updates the pinned upstream revision only when AuraGo itself ships a reviewed revision change; it does not follow upstream `main` implicitly.

## Local and SSH-host Modes

In `local_host` mode, AuraGo proxies directly to `127.0.0.1:18081` after verifying that the local management service is reachable.

In `ssh_host` mode, AuraGo manages a second SSH tunnel in addition to the existing private `boringd` tunnel. The management tunnel maps an AuraGo-local loopback endpoint to the remote host's `127.0.0.1:18081`. Tunnel creation is lazy and concurrency-safe, reuses a healthy tunnel, and replaces stale tunnels. Closing or replacing one Virtual Computers tunnel must clean up both channels.

The remote web service talks to the remote `boringd` over loopback, so the token remains confined to the remote service environment. AuraGo still keeps its own copy in the Vault for API operations and setup.

## HTTP, Authentication, and Security

All `/boring-computers/` requests pass through AuraGo's normal authentication and security middleware. The management proxy must:

- preserve the `/boring-computers` browser-facing base path while rewriting the upstream request path correctly;
- support WebSocket upgrades for terminal, VNC, and agent channels;
- reject access when `virtual_computers.enabled` is false;
- return `503 Service Unavailable` with a safe message when the service or SSH tunnel is unavailable;
- avoid returning private hostnames, loopback URLs, tokens, service environment values, or raw SSH errors;
- preserve same-origin browser behavior and secure cookie handling; and
- add no permissive cross-origin policy.

The install log redactor must cover every new environment assignment that can contain `BORING_TOKEN` or provider credentials. The token remains forbidden from Python tools and browser-visible configuration responses under the existing Virtual Computers Vault contract.

## Setup Status and Health

Setup status distinguishes the two managed components:

- `control_plane`: configured and healthy state for `boringd`
- `management`: installed, reachable, and healthy state for `apps/web`

The existing top-level `configured` and `healthy` values remain backward compatible. `healthy` is true only when both required components are healthy after managed setup. New fields are additive.

The connection test checks both components and reports which one failed. It does not make an LLM request and does not expose secrets. A management-page health check validates an HTTP success response and expected application marker, not merely an open TCP port.

## Chat Drawer Behavior

`integrationWebhostsForRequest` adds this entry when and only when `cfg.VirtualComputers.Enabled` is true:

- ID: `boring_computers`
- Name: `Boring Computers`
- Description: `Managed virtual computer control center`
- URL: `/boring-computers/`
- Icon: the existing terminal/computer-compatible chat icon
- Status: `running` when the management endpoint is healthy, otherwise `starting`

The entry remains available while enabled even if setup is incomplete, allowing the user to reach a safe service-unavailable page and then use setup or repair. Disabling the integration removes the entry and makes the management route unavailable immediately.

The existing webhost response remains additive and compatible for Chat and AgoDesk consumers. No new translation key is required for the brand name. If user-facing setup or error text is added to the configuration UI, all 15 supported language files must be updated.

## Failure Handling

- Unsupported hosts fail preflight before package or service changes.
- A port collision on `18081` is reported explicitly; AuraGo does not kill an unrelated process.
- A dependency or build failure returns a redacted summary and preserves the prior working deployment.
- A service health failure reports separate control-plane and management results.
- SSH tunnel failures close partially opened listeners and do not leave goroutines or ports behind.
- Upstream compatibility drift fails closed and instructs the user to update AuraGo or retry repair after a supported release.

## Testing

Implementation follows test-first development. Required coverage includes:

1. Drawer API includes Boring Computers only when `virtual_computers.enabled` is true.
2. Drawer metadata and `/boring-computers/` URL remain stable for Chat and AgoDesk consumers.
3. Disabled integrations receive no management proxy access.
4. HTTP proxy routing preserves the base path and does not leak the token.
5. WebSocket proxying works for an authenticated request.
6. Local mode reaches the loopback management endpoint.
7. SSH mode creates, reuses, replaces, and cleans up both tunnels.
8. Install scripts install Node/npm, use locked dependencies, build the pinned source, create a restricted environment file, and configure `boring-web.service` on both supported architectures.
9. Install and setup logs redact all managed secrets.
10. Setup status remains backward compatible while reporting both component states.
11. Repair is idempotent and a failed staged build preserves the active deployment.
12. Relevant server, configuration, Virtual Computers, UI regression, race-sensitive, and full Go test suites pass.

Before the implementation commit, GitNexus `detect_changes` must confirm that only the expected Virtual Computers setup, proxy, integration webhost, status, and test flows are affected.
