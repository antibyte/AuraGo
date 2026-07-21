# go2rtc Integration

AuraGo can manage [go2rtc](https://github.com/AlexxIT/go2rtc) as an optional Docker sidecar for secure camera viewing and snapshots. The integration is disabled by default and manages only its own container.

## Security model

- The reviewed default image is `alexxit/go2rtc:1.9.14@sha256:675c318b23c06fd862a61d262240c9a63436b4050d177ffc68a32710d9e05bae`.
- The go2rtc API and Web UI are never bound directly to the LAN. Native AuraGo binds API port 1984 only to `127.0.0.1`; Docker deployments use an internal network alias.
- Camera source URLs and the internal go2rtc API password are stored only in AuraGo's encrypted Vault. Generated `data/go2rtc/go2rtc.yaml` contains no camera sources or plaintext password.
- Sources are injected through go2rtc's runtime stream API after startup. Only `rtsp`, `rtsps`, `rtspx`, `http`, `https`, and `onvif` network URLs are accepted.
- The container runs as a non-root user with a read-only root filesystem, no capabilities, `no-new-privileges`, a restricted `/tmp`, and CPU, memory, and PID limits.
- AuraGo's proxy removes caller cookies and authorization headers, applies its own internal Basic authentication, and restricts media access to configured, enabled stream IDs.
- Credential-bearing ONVIF SOAP requests use a direct private-network transport and never honor `HTTP_PROXY` or `HTTPS_PROXY` from the environment.
- Upstream go2rtc logging is disabled because producer warnings can contain runtime source URLs. AuraGo exposes sanitized lifecycle and stream status instead.
- Snapshot memory is bounded to 16 entries and 64 MiB. Stored snapshots retain at most 1,000 files or 2 GiB and remove the oldest files first.

## Proxy and WebRTC

The stable viewer contract is `/api/go2rtc/viewer/{stream_id}`. MSE, HLS, MP4, MJPEG, WebSocket, and optional WHEP traffic pass through `/api/go2rtc/proxy/`.

WebRTC media normally flows directly between browser and go2rtc and cannot be carried completely through an HTTP reverse proxy. Direct LAN WebRTC is therefore disabled by default. Enabling it requires a concrete private LAN bind/candidate IP and publishes port 8555 over both TCP and UDP. RTSP port 8554 is never published on the host.

The built-in Virtual Desktop app **Network Cameras** uses this same-origin viewer and proxy boundary without exposing camera URLs or go2rtc credentials to the browser. Open it from the launcher or directly with `/desktop?app=network-cameras`.

## Virtual Desktop app and camera setup

The **Network Cameras** app combines a five-second snapshot grid with one selected live view. Its optional live-grid mode opens at most four viewers at once; hidden, minimized, and closed windows stop polling and tear down their media frames.

Administrators can enable the managed sidecar, discover ONVIF cameras, add a known local ONVIF address, or enter a supported stream URL. Other users with `go2rtc.view` see enabled cameras only. Camera management remains administrator-only.

Automatic ONVIF discovery is capability gated. AuraGo sends one bounded WS-Discovery probe per suitable private IPv4 interface only when `Runtime.BroadcastOK` is available. A normal Docker bridge does not provide that capability, so the app offers manual IP and stream-URL setup without granting host networking or extra privileges. Discovery candidates and credential-bearing setup tokens are random, memory-only, limited, single-use, and expire after five minutes. A setup token is reserved during publication, released after any pre-publication failure, and consumed only after the desired configuration has been published. SOAP requests are limited to the responding private host, 20 seconds, and 1 MiB.

The app consumes these AuraGo-owned endpoints:

- `GET /api/go2rtc/app/state` for sanitized capability and stream telemetry
- `GET /api/go2rtc/thumbnail/{stream_id}.jpg` for non-persistent JPEG tiles with private caching and ETags
- `POST /api/go2rtc/setup/enable`, `/api/go2rtc/discovery`, and `/api/go2rtc/discovery/profiles` for administrator setup
- `POST /api/go2rtc/streams` plus `PATCH` or `DELETE /api/go2rtc/streams/{stream_id}` for administrator-managed streams

The raw upstream `api/onvif` endpoint is always blocked, including when the optional administrator Web UI is enabled. It is never used for discovery or credentials.

### Mutation and recovery contract

Camera create, update, delete, and integration-enable requests publish a fully loaded and validated candidate configuration before replacing the active YAML. Vault changes are rolled back if validation or publication fails. Once published, the YAML/Vault state remains AuraGo's desired state even if Docker or go2rtc is temporarily unavailable; the background manager retries reconciliation.

- Complete create returns HTTP `201`; complete update, delete, and enable return HTTP `200`.
- A published change whose runtime reconciliation failed returns HTTP `202` with `status: "degraded"`, `saved: true`, and `runtime_reconciled: false`. The response includes only the safe saved stream state or remaining stream count.
- Invalid input returns `400`, a missing stream `404`, a duplicate ID `409`, and an unmet integration prerequisite `412`. Config or Vault publication failures return `500` and `saved: false`.

Before enabling the integration, AuraGo checks the live Docker contract with bounded requests. Container and image listing must be readable, network listing must also be readable when AuraGo itself runs in Docker, and a POST to start a cryptographically random nonexistent sentinel container must return `404`. The probe never creates a container. Missing capabilities are reported with `docker_unreachable`, `docker_containers_denied`, `docker_images_denied`, `docker_networks_denied`, or `docker_post_denied`.

## Agent operations

When the integration, agent access, and authenticated API are all available, AuraGo advertises a read-only `go2rtc` tool with:

- `status`
- `list_streams`
- `stream_status`
- `snapshot`
- `analyze_snapshot`
- `show_live_stream`

The agent accepts stable stream IDs only and cannot start or stop the service or modify stream configuration. Snapshot analysis uses AuraGo's existing Vision provider and budget accounting.

## Configuration

Configure the integration under **Config → Network & Remote → go2rtc Cameras**, or use the administrator onboarding in **Network Cameras**. Connection tests, container controls, snapshots, and viewers intentionally use only the last saved configuration.

The optional administrator Web UI is a read-only subset of the original go2rtc interface. Configuration, logs, publishing, process control, and stream mutation routes remain blocked.

## License

go2rtc is Copyright AlexxIT and contributors and is distributed under the MIT License. AuraGo does not modify or rebuild the upstream image; it pulls the pinned published image at runtime. Review the upstream [license](https://github.com/AlexxIT/go2rtc/blob/v1.9.14/LICENSE) and [release notes](https://github.com/AlexxIT/go2rtc/releases/tag/v1.9.14) before overriding the pinned image.
