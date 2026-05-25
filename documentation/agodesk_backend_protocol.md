# agodesk Backend Protocol

This document is the backend contract for the agodesk desktop client.

## WebSocket Endpoint

- URL: `/api/agodesk/ws`
- Transport: WebSocket text frames with JSON envelopes.
- Auth: the route bypasses browser session auth, but the socket performs its own pairing handshake.
- Development fallback: unauthenticated chat is only allowed for loopback clients that connect with `?insecure_loopback=1`.

Every frame uses this envelope:

```json
{
  "id": "uuid-or-client-message-id",
  "type": "message.type",
  "timestamp": "2026-05-24T12:00:00Z",
  "payload": {}
}
```

## Connection Flow

1. Client connects to `/api/agodesk/ws`.
2. Server immediately sends `system.connected`.
3. Production clients send `session.start` with either a one-time `pairing_token` or a stored `device_id` plus `shared_key_proof`.
4. Server replies with `session.accepted` or `chat.error`.
5. Chat is accepted only after pairing, except explicit loopback development mode.

## Message Types

- `system.connected`: server greeting with `protocol_version`, temporary `session_id` (not for chat), auth flags, and capabilities.
- `system.ping` / `system.pong`: keepalive.
- `session.start`: client pairing or reconnect request.
- `session.accepted`: server approval. Fresh pairing includes `shared_key` once so the client can store it securely. The returned `session_id` replaces the temporary ID from `system.connected` and must be used in every subsequent `chat.message`.
- `chat.message`: user prompt.
- `chat.response`: full assistant response with `request_id`.
- `chat.error`: machine-readable error.
- `chat.response.chunk`: reserved for streaming support.
- `desktop.command` / `desktop.result`: server-to-client desktop command transport for screenshots, permission requests, and locally approved input.

## Pairing

Fresh pairing:

- The user creates a Remote Control enrollment token in AuraGo.
- agodesk sends it as `payload.pairing_token` in `session.start`.
- AuraGo creates a RemoteHub device tagged `agodesk` and `desktop-client`.
- The enrollment token is the approval step for agodesk pairing; there is no separate manual approval action in Remote Control.
- AuraGo stores the generated shared key in the Vault under `remote_shared_key_<device_id>`.
- `session.accepted.shared_key` is returned only on fresh pairing.

Reconnect:

- agodesk sends `device_id` and `shared_key_proof`.
- `shared_key_proof` is an object with `nonce`, `timestamp`, and `hmac` (hex HMAC-SHA256).
- The proof is an HMAC-SHA256 over the `session.start` envelope id, device id, nonce, and proof timestamp.
- AuraGo verifies the proof with the Vault-stored shared key.
- Reconnect is allowed for paired devices in `approved`, `connected`, or `offline` status. `offline` only means no socket is currently connected.
- Reconnect responses never echo the shared key.

Example reconnect payload:

```json
{
  "device_id": "dev-abc123",
  "shared_key_proof": {
    "nonce": "uuid",
    "timestamp": "2026-05-24T12:00:00.000Z",
    "hmac": "hex-hmac-sha256"
  }
}
```

Proof format:

```text
material =
  "agodesk.v1" +
  "\nsession.start\n" +
  envelope_id +
  "\n" +
  device_id +
  "\n" +
  nonce +
  "\n" +
  timestamp
hmac = hex(HMAC_SHA256(shared_key_bytes, material))
```

`shared_key_bytes`: valid hex string is decoded; otherwise the raw UTF-8 string is used as key material.

## Desktop Client Requirements

- Store `device_id` persistently.
- Store `shared_key` in OS keychain or secure Tauri storage when available.
- Never print the shared key in logs or UI.
- Send `session.start` automatically after `system.connected` when paired.
- Block chat input until `session.accepted` in production mode.
- Implement native Tauri commands for desktop control:
  - `collect_host_info()`
  - `list_displays()`
  - `list_windows()`
  - `capture_screen({ display_id?, window_id?, format, quality })`
  - `control_permission_status()`
  - `inject_input(event)` only during an approved local control session.
  - `set_input_approval(approved)` / `reset_desktop_session()`
- Display a visible local remote-control banner with approve, deny, and stop controls before allowing input injection.

## Desktop Control

Screenshots do not require user approval. Input injection requires explicit local approval via the remote-control banner.

### Screenshot request params (`desktop_screenshot`)

Capture a specific monitor in multi-monitor setups with `display_id` from `list_displays()`:

```json
{
  "display_id": "display-1",
  "format": "png",
  "quality": 80
}
```

Capture a single window:

```json
{
  "window_id": "win-12345678",
  "format": "jpeg",
  "quality": 85
}
```

Omit `display_id` to capture the primary monitor.

### Screenshot result (`desktop.result.data`)

```json
{
  "source": "display",
  "display_id": "display-0",
  "format": "png",
  "width": 1920,
  "height": 1080,
  "scale_factor": 1.0,
  "mime": "image/png",
  "data_base64": "..."
}
```

Window captures set `"source": "window"` and include `window_id`.

### Input events (`desktop_input`)

`params.kind` values:

| kind | payload |
|---|---|
| `mouse_move` | `{ "x": 100, "y": 200, "absolute": true }` |
| `mouse_click` | `{ "x": 100, "y": 200, "button": "left", "action": "click" }` |
| `key_down` / `key_up` | `{ "key": "enter" }` or `{ "code": 65 }` |
| `text` | `{ "text": "Hello" }` |

Input is blocked until the user approves remote control locally.

## RemoteHub Operations

The existing RemoteHub command protocol supports these agodesk-capable operations in this backend version:

- `desktop_screenshot`
- `desktop_permission_request`
- `desktop_input`

Read-only policy permits screenshot and permission status requests. It denies desktop input.

`desktop_stream_start` and `desktop_stream_stop` remain reserved for a later backend version and are not available in v1.
