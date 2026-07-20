# go2rtc Integration

AuraGo can manage [go2rtc](https://github.com/AlexxIT/go2rtc) as an optional Docker sidecar for secure camera viewing and snapshots. The integration is disabled by default and manages only its own container.

## Security model

- The reviewed default image is `alexxit/go2rtc:1.9.14@sha256:675c318b23c06fd862a61d262240c9a63436b4050d177ffc68a32710d9e05bae`.
- The go2rtc API and Web UI are never bound directly to the LAN. Native AuraGo binds API port 1984 only to `127.0.0.1`; Docker deployments use an internal network alias.
- Camera source URLs and the internal go2rtc API password are stored only in AuraGo's encrypted Vault. Generated `data/go2rtc/go2rtc.yaml` contains no camera sources or plaintext password.
- Sources are injected through go2rtc's runtime stream API after startup. Only `rtsp`, `rtsps`, `rtspx`, `http`, `https`, and `onvif` network URLs are accepted.
- The container runs as a non-root user with a read-only root filesystem, no capabilities, `no-new-privileges`, a restricted `/tmp`, and CPU, memory, and PID limits.
- AuraGo's proxy removes caller cookies and authorization headers, applies its own internal Basic authentication, and restricts media access to configured, enabled stream IDs.

## Proxy and WebRTC

The stable viewer contract is `/api/go2rtc/viewer/{stream_id}`. MSE, HLS, MP4, MJPEG, WebSocket, and optional WHEP traffic pass through `/api/go2rtc/proxy/`.

WebRTC media normally flows directly between browser and go2rtc and cannot be carried completely through an HTTP reverse proxy. Direct LAN WebRTC is therefore disabled by default. Enabling it requires a concrete private LAN bind/candidate IP and publishes port 8555 over both TCP and UDP. RTSP port 8554 is never published on the host.

This same-origin viewer and proxy boundary is intended for future use by AuraGo's Virtual Desktop without exposing camera URLs or go2rtc credentials to the desktop app.

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

Configure the integration under **Config → Network & Remote → go2rtc Cameras**. Connection tests, container controls, snapshots, and viewers intentionally use only the last saved configuration.

The optional administrator Web UI is a read-only subset of the original go2rtc interface. Configuration, logs, publishing, process control, and stream mutation routes remain blocked.

## License

go2rtc is Copyright AlexxIT and contributors and is distributed under the MIT License. AuraGo does not modify or rebuild the upstream image; it pulls the pinned published image at runtime. Review the upstream [license](https://github.com/AlexxIT/go2rtc/blob/v1.9.14/LICENSE) and [release notes](https://github.com/AlexxIT/go2rtc/releases/tag/v1.9.14) before overriding the pinned image.
