# PrintGuard in the Desktop Software Store

PrintGuard keeps its original dashboard, camera video, printer configuration and
automatic failure responses. AuraGo manages the container and optionally connects
the existing MCP client. Installing the Store entry does not enable agent access.

## Prerequisites and setup

1. Enable Desktop and Docker writes. Use a local Docker daemon running Linux
   containers on amd64 or arm64. Remote daemons are outside this integration.
2. Enable AuraGo login protection and open AuraGo over HTTPS at its configured
   primary DNS name (`server.https.domain`, or `server.host` behind a trusted
   HTTPS reverse proxy).
3. Configure `server.preview_domain` on a **different registrable site**, with
   wildcard DNS and a trusted wildcard TLS certificate routed to AuraGo. For
   example, `aura.example.org` and `*.apps.example.net`. The setup probes the
   generated app host over HTTPS without sending an AuraGo session or token.
   This hostname must also be reachable from the AuraGo server (use split DNS
   where a public address cannot loop back through the router).
4. Install PrintGuard from the System category. First installation uses CPU.
   When AuraGo runs in Docker, select a user-defined network already attached to
   AuraGo with external camera/network access. One suitable network is selected
   automatically; multiple networks require a choice. Internal networks (including
   Docker control), the default `bridge`, `host` and `none` are not choices. The
   bundled `tcp://docker-proxy:2375` endpoint is supported inside AuraGo's Compose
   deployment, in addition to local sockets/loopback endpoints.
5. Open PrintGuard and configure printers, network cameras, monitors and failure
   responses in its own Settings. Pulling camera streams needs no input port.

All Store, Desktop, start-menu and Launchpad opens use the isolated HTTPS preview
gateway, including when the global preview preference is off. There is no direct
LAN or Tailscale fallback for this app. Only container port 8000 is published,
on a dynamically allocated **127.0.0.1** host port. RTSP 8554, RTMP 1935 and the
internal MediaMTX ports are not published. Upstream Host, Origin and framing
checks remain active; `PRINTGUARD_ORIGINS` names the exact AuraGo and app origins.

## Acceleration and lifecycle

Use **Set up** in the Store to change the acceleration profile:

| Profile | Platforms | Extra requirement |
| --- | --- | --- |
| CPU | Linux amd64, arm64 | None |
| Intel | Linux amd64 | Supported Intel GPU and `/dev/dri` on the Docker host |
| NVIDIA | Linux amd64 | NVIDIA drivers, NVIDIA Container Toolkit and its Docker runtime |

The Intel profile grants only `/dev/dri:/dev/dri:rwm`; NVIDIA requests the NVIDIA
driver with `gpu,compute,utility` capabilities. No profile enables privileged
mode, arbitrary host devices or USB passthrough. AMD is not supported by v1.
Setup displays the selected profile and `stats.inference_device` from the live
API; a reported CPU device with a selected GPU profile is marked as a fallback.
Once PrintGuard requires API authentication, reading the compute device needs a
valid saved token; before any token exists its private read-only API provides it.

Install, start and replacement wait up to five minutes for the image's Docker
health check (`/api/health`) and the web port. Updates and profile changes use the
Store's parked-container replacement and rollback, preserving the previous image,
configuration revision and device grants. Desired settings remain visibly pending
after a failed replacement. Rollback restores the runtime, not a previous copy of
the data: back up the stopped `/data` volume before future version upgrades.

Stop, update, restart and profile changes interrupt monitoring. Printing continues
unless PrintGuard or the operator separately sends a printer command. Normal
uninstall retains the named data volume and saved settings. **Delete data** also
removes that volume and the AuraGo Vault settings.

The release references are pinned to the v2.6.3 OCI index digests, resolved
anonymously on 2026-10-23:

| Image in `ghcr.io/oliverbravery/` | SHA-256 |
| --- | --- |
| `printguard:2.6.3` | `c70168c1703c29a447d19917d3296ef7093ebb8d2fbe267879e8cecc284a11a3` |
| `printguard:2.6.3-intel` | `afa75b109c030c291cf006334f4ee754698aefd02c116b2d714b96d00e7c2192` |
| `printguard:2.6.3-nvidia` | `c7cdf6c6a33982b0f7f8500273f8140f401d11b68a159279b3d4d9e149e7300f` |

Store Update reapplies the reviewed pin for the selected profile; it does not
follow upstream `latest`. A later release requires reviewing and updating all
profile pins together. The packaged upstream icon and GPL license/notice are in
`ui/img/desktop/store/printguard.*`.

## Optional agent access

Enable both `agent.allow_mcp` and `mcp.enabled`. In PrintGuard Settings → API,
generate a token with the desired `read` or `control` scope. Save it through the
Store setup dialog and choose:

- **Off** (default): no managed agent connection.
- **Status and camera images**: `get_state`, `list_printers`, `get_printer`,
  `list_monitors`, `get_monitor`, `list_cameras`, `get_camera`,
  `get_camera_frame`, `recent_events`.
- **Status and printer control**: the same reads plus `control_printer`, which
  supports only pause, resume and cancel. Desktop read-only blocks control.

Saving revalidates the running service. **Test saved connection** tests only
saved settings and never sends a printer command. Activation requires anonymous
`GET /api/v1/state` to return 401, an authenticated read to succeed, and discovery
of every required MCP tool. Upstream control tokens also grant heating and print
start, but AuraGo's enforced discovery/execution allowlist excludes those tools.

The managed name is `printguard-store`, using Streamable HTTP at `/mcp/` over the
private local connection. A custom connection with that name is preserved and
reported as a conflict. Token or mode changes revoke the old grant immediately.
Lifecycle operations close the old connection before touching the container;
after confirmed start the saved grant is tested and restored. A fresh AuraGo
process must verify again. User-managed MCP connections are never replaced.

Commands require the user's request. Automatic failure detection and responses
remain in PrintGuard. If a command times out or loses its transport, its outcome
is uncertain: query `get_printer`/`get_state`, report that uncertainty and never
automatically replay the command.

The API token is write-only in AuraGo and stored only in its encrypted Vault.
Blank preserves it; the explicit removal checkbox deletes it and turns access
off, even if gateway prerequisites have become unavailable. PrintGuard owns its
printer passwords, camera logins and token hashes in `/data`; those credentials
are not moved into AuraGo and the volume is not encrypted by AuraGo.

Admin API: `GET/PUT /api/desktop/store/apps/printguard/config` and
`POST /api/desktop/store/apps/printguard/test`. GET contains flags, prerequisites,
desired/active profile and compute status, never the token. PUT accepts
`acceleration`, `docker_network`, `agent_mode`, `api_token`, `clear_api_token`;
origins and GPU grants are generated by the server.

## Verification boundaries

Run the Store, MCP and gateway tests, `TestPrintGuard*` in server/UI, and
`node scripts/build-ui-bundles.js --check`. Browser setup coverage is opt-in with
`AURAGO_RUN_BROWSER_SMOKE=1`, using synthetic APIs, real modules/styles and both
themes at narrow/wide sizes. These tests do not establish physical device or
external DNS/TLS acceptance.

`TestPrintGuardLiveCPU` is opt-in with `AURAGO_PRINTGUARD_LIVE_URL` set to an
explicit `http://127.0.0.1:<port>` origin of an isolated, empty reviewed image.
It refuses a hub containing printers/cameras or existing API tokens, creates
temporary read/control test tokens and exercises the real MCP handshake and a
state read. Remove its dedicated container and test volume afterward; never
point this test at an operator's hub. It sends no printer command.

Separately accept CPU amd64 and arm64, Intel and NVIDIA on suitable hosts; check
the actual compute device, video, WebSockets, upload, session expiry and multiple
windows through the deployed HTTPS gateway. Real pause/resume/cancel tests require
an explicit operator request and a suitable test print.

Upstream references: [API and MCP](https://github.com/oliverbravery/PrintGuard/blob/v2.6.3/docs/api.md),
[deployment](https://github.com/oliverbravery/PrintGuard/blob/v2.6.3/docs/deployment.md),
[hardware](https://github.com/oliverbravery/PrintGuard/blob/v2.6.3/docs/hardware.md).
