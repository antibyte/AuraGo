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

- `system.connected`: server greeting with `protocol_version`, temporary `session_id`, auth flags, and capabilities.
- `system.ping` / `system.pong`: keepalive.
- `session.start`: client pairing or reconnect request.
- `session.accepted`: server approval. Fresh pairing includes `shared_key` once so the client can store it securely.
- `chat.message`: user prompt.
- `chat.response`: full assistant response with `request_id`.
- `chat.error`: machine-readable error.
- `chat.response.chunk`: reserved for streaming support.
- `desktop.command` / `desktop.result`: reserved for desktop capture and input commands.

## Pairing

Fresh pairing:

- The user creates a Remote Control enrollment token in AuraGo.
- agodesk sends it as `payload.pairing_token` in `session.start`.
- AuraGo creates a RemoteHub device tagged `agodesk` and `desktop-client`.
- AuraGo stores the generated shared key in the Vault under `remote_shared_key_<device_id>`.
- `session.accepted.shared_key` is returned only on fresh pairing.

Reconnect:

- agodesk sends `device_id` and `shared_key_proof`.
- The proof is an HMAC-SHA256 over the `session.start` envelope id, device id, nonce, and proof timestamp.
- AuraGo verifies the proof with the Vault-stored shared key.
- Reconnect responses never echo the shared key.

## Desktop Client Requirements

- Store `device_id` persistently.
- Store `shared_key` in OS keychain or secure Tauri storage when available.
- Never print the shared key in logs or UI.
- Send `session.start` automatically after `system.connected` when paired.
- Block chat input until `session.accepted` in production mode.
- Implement native Tauri commands for future desktop control:
  - `collect_host_info()`
  - `list_displays()`
  - `capture_screen({ display_id, format, quality })`
  - `control_permission_status()`
  - `inject_input(event)` only during an approved local control session.
- Display a visible local remote-control banner with approve, deny, and stop controls before allowing input injection.

## Planned Remote Operations

The existing RemoteHub command protocol reserves these operations for agodesk-capable clients:

- `desktop_screenshot`
- `desktop_stream_start`
- `desktop_stream_stop`
- `desktop_permission_request`
- `desktop_input`

Read-only policy permits screenshot, stream start/stop, and permission status requests. It denies desktop input.
