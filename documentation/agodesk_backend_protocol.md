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

AuraGo accepts AgoDesk WebSocket messages up to 16 MiB. Desktop screenshot results include `data_base64` inside `desktop.result`, so clients must allow outgoing screenshot frames at least this large or downscale/compress before replying. File payloads use a stricter v1 inline limit of 8 MiB or the smaller limit negotiated in `session.start.file_access`.

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
- `chat.response`: full assistant response with `request_id`; may also be server-initiated when `metadata.server_push=true`.
- `chat.error`: machine-readable error.
- `chat.response.chunk`: reserved for streaming support.
- `persona.assets.request`: client request for the currently active AuraGo persona's visual assets and prompt.
- `persona.assets`: server response with the active persona name, asset key, avatar image URL, icon URL, and persona prompt.
- `desktop.command` / `desktop.result`: server-to-client command transport for screenshots, permission requests, locally approved input, and locally approved file access.

## Client Capabilities

Clients must include `payload.client_capabilities` in `session.start`. AuraGo treats `session.accepted.capabilities` as the server-side feature list, and `session.start.client_capabilities` as the client's advertised feature list for that WebSocket session.

Desktop commands are dispatched only when the matching client capability is present:

- `chat.full_response`: required for server-initiated AgoChat messages.
- `remote.desktop.capture`: required for `desktop_screenshot`
- `remote.desktop.permission_request`: required for `desktop_permission_request`
- `remote.desktop.input`: required for `desktop_input`
- `remote.files.read`: required for `file_list` and `file_read`
- `remote.files.write`: required for `file_write`

If a client omits these capabilities, pairing, heartbeat, persona assets, and chat can still work, but remote commands return `UNSUPPORTED_CAPABILITY` immediately instead of waiting for a `desktop.result` timeout. A client that sends keepalives but does not advertise the desktop or file capabilities is connected, but only capable of the features it advertised.

## File Access Metadata

AgoDesk owns local file permissions. If local file access is available, include `payload.file_access` in `session.start`:

```json
{
  "client_version": "agodesk-1.2.0",
  "client_capabilities": ["chat.full_response", "remote.files.read", "remote.files.write"],
  "file_access": {
    "enabled": true,
    "max_read_bytes": 8388608,
    "max_write_bytes": 8388608,
    "roots": [
      {
        "root_id": "workspace",
        "label": "Workspace",
        "path_display": "~/Projects/AuraGo",
        "permissions": ["read", "write"]
      }
    ]
  },
  "host": {
    "hostname": "AGODESK",
    "os": "windows",
    "arch": "amd64"
  }
}
```

Rules:

- `file_access` is optional for backward compatibility.
- `enabled=false` means AgoDesk should not advertise `remote.files.read` or `remote.files.write`.
- `root_id` is stable for the local AgoDesk configuration and is used in later commands.
- `path_display` is UI/debug metadata. AuraGo must not treat it as an authorization boundary.
- AuraGo may display/cache this metadata, but AgoDesk must enforce canonical path checks and permissions locally for every command.

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
- After `session.accepted`, send `persona.assets.request` and cache the returned `persona.assets` values for chat/avatar UI. Re-request after reconnect or when the server sends/your UI observes a persona change.
- Implement native Tauri commands for desktop control:
  - `collect_host_info()`
  - `list_displays()`
  - `list_windows()`
  - `capture_screen({ display_id?, window_id?, format, quality })`
  - `control_permission_status()`
  - `inject_input(event)` only during an approved local control session.
  - `set_input_approval(approved)` / `reset_desktop_session()`
- Display a visible local remote-control banner with approve, deny, and stop controls before allowing input injection.
- Store file-access roots and per-root read/write permissions locally in AgoDesk. AuraGo does not configure or enforce these roots.
- Canonicalize every requested file path before access, reject traversal and symlink escapes, enforce per-root permissions, and use atomic writes.

## Server-Initiated AgoChat Messages

AuraGo can proactively send a text message to a connected AgoDesk client from autonomous sessions such as missions, heartbeat, planner notifications, or maintenance.

The backend emits a normal `chat.response` envelope without a preceding client `chat.message`. Clients must display these as assistant messages when `payload.metadata.server_push` is `true`:

```json
{
  "id": "server-generated-id",
  "type": "chat.response",
  "timestamp": "2026-05-31T17:00:00Z",
  "payload": {
    "session_id": "agodesk:device-123",
    "request_id": "chat-push-1",
    "text": "Mission finished successfully.",
    "role": "assistant",
    "metadata": {
      "source": "aurago_agent",
      "server_push": true
    }
  }
}
```

Coding agents should use the AuraGo `send_agodesk_chat` tool for proactive AgoChat messages. Use the `device_id` shown in the system prompt's `REACHABLE CHAT CHANNELS` section or discover connected clients through `remote_control` `list_devices`.

## Active Persona Assets

The desktop client can ask AuraGo which avatar, icon, and prompt should represent the currently active persona. This is intended for chat headers, tray overlays, notifications, local UI copy, and any local agent behavior that should mirror the web chat persona.

Request after the WebSocket session is accepted:

```json
{
  "id": "persona-assets-1",
  "type": "persona.assets.request",
  "timestamp": "2026-05-24T12:00:00Z",
  "payload": {
    "session_id": "agodesk:dev:abc123"
  }
}
```

Response:

```json
{
  "id": "server-generated-id",
  "type": "persona.assets",
  "timestamp": "2026-05-24T12:00:00Z",
  "payload": {
    "session_id": "agodesk:dev:abc123",
    "persona": "punk",
    "icon_key": "punk",
    "avatar_image_url": "/img/personas/punk.png?v=20260502-persona-refresh",
    "icon_url": "/img/persona-icons/punk.png?v=20260502-persona-refresh",
    "persona_prompt": "# Core Personality: Punk\n\nDirect, raw, no sugarcoating...",
    "asset_version": "20260502-persona-refresh"
  }
}
```

Rules:

- `session_id` is required and must match the accepted AgoDesk session.
- `persona` is the configured active persona name.
- `icon_key` is the asset key the UI should use. Built-in personas use their own key; custom or unknown personas return `custom`.
- `avatar_image_url` is the larger persona portrait from `/img/personas/`.
- `icon_url` is the small chat/avatar icon from `/img/persona-icons/`.
- `persona_prompt` is the active persona markdown body with YAML front matter removed. It may contain headings and multiple paragraphs. It can be an empty string when the active persona file cannot be found.
- URLs are same-origin relative paths. Prefix them with the connected AuraGo origin in native clients.
- Treat these URLs as UI assets, not user-provided content. Do not execute or parse returned image data.
- Treat `persona_prompt` as server-provided instructions only for AgoDesk's local agent/persona layer. Do not render it as trusted HTML; display as plain text or markdown through the same sanitizer used for server-authored documentation.

## Coding Agent Instruction: Persona Assets

When modifying the AgoDesk desktop client, use the persona asset protocol instead of hard-coding persona images or persona behavior text.

1. Wait for `session.accepted`.
2. Send `persona.assets.request` with the accepted `session_id`. Go clients using `internal/agodesk` should call `agodesk.NewPersonaAssetsRequest(sessionID)` instead of hand-building the envelope.
3. On `persona.assets`, resolve `avatar_image_url` and `icon_url` against the AuraGo server origin and store `persona_prompt` alongside the image URLs.
4. Use `icon_url` for compact chat bubbles, tray/status indicators, and 32px controls.
5. Use `avatar_image_url` for larger profile cards, welcome panels, or previews.
6. Use `persona_prompt` as the current local persona instruction for AgoDesk-controlled UI/agent behavior. Replace the previous cached prompt atomically when a new `persona.assets` response arrives.
7. Store `asset_version` with the cached URLs and prompt; refresh the cache after reconnect, after an explicit persona-change event, or when `asset_version` changes.
8. If the request returns `chat.error` with `PAIRING_REQUIRED` or `SESSION_NOT_FOUND`, do not guess a persona. Retry only after a fresh `session.accepted`.

For a concrete client-side implementation checklist, see [`agodesk_coding_agent_persona_prompt.md`](./agodesk_coding_agent_persona_prompt.md).

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

## File Access Commands

File commands reuse the existing `desktop.command` / `desktop.result` envelope pair. AgoDesk must execute them only when local file access is enabled and the path resolves inside a configured root with the required permission.

### List files (`file_list`)

Requires `remote.files.read`.

```json
{
  "command_id": "cmd-list-1",
  "operation": "file_list",
  "params": {
    "root_id": "workspace",
    "path": "src",
    "recursive": false
  }
}
```

Successful result:

```json
{
  "command_id": "cmd-list-1",
  "ok": true,
  "data": {
    "files": [
      {
        "name": "main.go",
        "path": "src/main.go",
        "type": "file",
        "size": 1234,
        "modified_at": "2026-06-03T12:00:00Z"
      }
    ]
  }
}
```

### Read file (`file_read`)

Requires `remote.files.read`.

```json
{
  "command_id": "cmd-read-1",
  "operation": "file_read",
  "params": {
    "root_id": "workspace",
    "path": "src/main.go",
    "encoding": "utf-8"
  }
}
```

Successful result:

```json
{
  "command_id": "cmd-read-1",
  "ok": true,
  "data": {
    "content": "package main\n",
    "encoding": "utf-8",
    "bytes": 13,
    "truncated": false
  }
}
```

### Write file (`file_write`)

Requires `remote.files.write`.

```json
{
  "command_id": "cmd-write-1",
  "operation": "file_write",
  "params": {
    "root_id": "workspace",
    "path": "src/main.go",
    "content": "package main\n"
  }
}
```

Successful result:

```json
{
  "command_id": "cmd-write-1",
  "ok": true,
  "data": {
    "bytes": 13
  }
}
```

If `root_id` is present, `path` is relative to that root. If `root_id` is omitted, AgoDesk may accept an absolute path only when the canonical path resolves inside a configured root.

File command errors use `ok=false` and a stable error code in `error`, for example `FILE_ACCESS_DISABLED`, `FILE_ACCESS_DENIED`, `FILE_TOO_LARGE`, or `FILE_CONFLICT`. Do not include file contents in error messages or logs.

Inline file payloads are limited to 8 MiB in v1 or the smaller value from `file_access.max_read_bytes` / `file_access.max_write_bytes`. Larger transfers should return `FILE_TOO_LARGE`; chunked transfer is reserved for a later protocol version.

## RemoteHub Operations

The existing RemoteHub command protocol supports these agodesk-capable operations in this backend version:

- `desktop_screenshot`
- `desktop_permission_request`
- `desktop_input`
- `file_list`
- `file_read`
- `file_write`

Read-only policy permits screenshot, permission status requests, file listing, and file reading. It denies desktop input and file writing.

`desktop_stream_start` and `desktop_stream_stop` remain reserved for a later backend version and are not available in v1.

For a concrete client-side implementation checklist, see [`agodesk_coding_agent_file_access.md`](./agodesk_coding_agent_file_access.md).
