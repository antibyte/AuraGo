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
- `chat.response.chunk`: reserved for streaming support; may carry optional metadata when streaming is enabled.
- `chat.plan_update`: active chat plan snapshot for clients that advertise `chat.plan_updates`.
- `chat.sessions.list` / `chat.sessions`: list AuraGo chat conversations available to AgoDesk.
- `chat.session.create` / `chat.session`: create or load a shared AuraGo chat conversation.
- `chat.session.load`: load a shared AuraGo chat conversation with visible messages.
- `chat.cancel` / `chat.cancelled`: stop the active agent turn for a conversation.
- `chat.audio`: server-generated TTS audio event for clients that negotiate `chat.audio_events`.
- `chat.media`: non-TTS chat artifacts for clients that negotiate `chat.media_events`.
- `chat.voice_output.status`: client status update and server acknowledgement for the same `speaker_mode` preference used by AuraGo Web Chat.
- `integrations.webhosts.list` / `integrations.webhosts`: list integrations with their own web UI, matching the Web Chat integrations drawer.
- `system.warnings.list` / `system.warnings`: list current system warnings shown in Web Chat.
- `system.warning.acknowledge`: acknowledge one warning or all warnings.
- `persona.assets.request`: client request for the currently active AuraGo persona's visual assets and prompt.
- `persona.assets`: server response with the active persona name, asset key, avatar image URL, icon URL, and persona prompt.
- `desktop.command` / `desktop.result`: server-to-client command transport for screenshots, discovery, UI automation, browser CDP, permission requests, locally approved input/actions, and locally approved file access.

## Client Capabilities

Clients must include `payload.client_capabilities` in `session.start`. AuraGo treats `session.accepted.capabilities` as the server-side feature list, and `session.start.client_capabilities` as the client's advertised feature list for that WebSocket session.

`session.accepted` also includes `advertised_capabilities`, the negotiated intersection of the client capabilities and AuraGo server policy. AgoDesk should use this field for UI registration and status display:

```json
{
  "session_id": "agodesk:dev-abc123",
  "device_id": "dev-abc123",
  "approved": true,
  "read_only": false,
  "capabilities": ["chat.full_response", "remote.desktop.capture"],
  "advertised_capabilities": [
    "chat.full_response",
    "remote.desktop.capture",
    "remote.desktop.discovery",
    "remote.desktop.ui_automation"
  ]
}
```

Desktop commands are dispatched only when the matching client capability is present:

- `chat.full_response`: required for server-initiated AgoChat messages.
- `chat.agent_metadata`: enables `metadata.agent_mood` on chat responses for voice-model tone selection.
- `chat.plan_updates`: enables live `chat.plan_update` frames and final `metadata.plan` snapshots.
- `chat.sessions`: enables shared AuraGo chat history, New Chat, and loading old conversations.
- `chat.cancel`: enables Stop for active AgoDesk agent turns.
- `chat.audio_events`: enables `chat.audio` frames for server-generated TTS playback.
- `chat.media_events`: enables `chat.media` frames for non-TTS images, audio/music, documents, videos, STL, links, and YouTube embeds.
- `chat.voice_output`: server-offered only when AuraGo TTS is configured; lets AgoDesk request server-side voice output with `chat.message.payload.voice_output=true`.
- `chat.voice_output_status`: enables AgoDesk to report the current chat speech-output state with `chat.voice_output.status`.
- `integrations.webhosts`: enables the Web Chat integrations drawer list over WebSocket.
- `system.warnings`: enables the Web Chat system warnings list and acknowledgement flow over WebSocket.
- `remote.desktop.capture`: required for `desktop_screenshot`
- `remote.desktop.permission_request`: required for `desktop_permission_request`
- `remote.desktop.input`: required for `desktop_input`
- `remote.desktop.discovery`: required for `desktop_list_displays`, `desktop_list_windows`, `desktop_active_window`, and `desktop_host_info`
- `remote.desktop.ui_automation`: required for `desktop_ui_tree` and `desktop_ui_action`
- `remote.desktop.browser`: required for `desktop_browser_connect`, `desktop_browser_snapshot`, `desktop_browser_action`, and `desktop_browser_disconnect`
- `remote.files.read`: required for `file_list`, `file_read`, and `file_search`
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
- AuraGo stores only sanitized session metadata from `file_access` and includes available `root_id`, display labels, permissions, and inline byte limits in the AgoDesk agent context.
- AuraGo caps `file_read` / `file_write` inline command limits to 8 MiB or the smaller negotiated `max_read_bytes` / `max_write_bytes`, rejects known disabled or denied `root_id` cases for `file_list`, `file_read`, `file_search`, and `file_write`, and still requires AgoDesk to enforce canonical path checks and permissions locally for every command.

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
- After `session.accepted`, store `advertised_capabilities`. If `chat.sessions` is negotiated, send `chat.sessions.list`, pick the last local `conversation_id`, or send `chat.session.create`.
- If `integrations.webhosts` is negotiated, send `integrations.webhosts.list` and render the returned webhost links.
- If `system.warnings` is negotiated, send `system.warnings.list` and refresh the local warning UI whenever `system.warnings` arrives.
- Send every `chat.message` with the accepted AgoDesk `session_id` and the active AuraGo `conversation_id`. Older clients may omit `conversation_id`; AuraGo then keeps the legacy transport-session behavior.
- Show Stop only while a request is active. Stop sends `chat.cancel` with `session_id`, `conversation_id`, and `request_id`, and must also stop local TTS/audio immediately.
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

## Agent Mood Metadata And Plan Updates

Interactive AgoDesk chat responses can include optional metadata for local voice rendering and plan display. AgoDesk must advertise the matching capabilities in `session.start.client_capabilities` before AuraGo sends the richer payloads.

When `chat.agent_metadata` is negotiated, `chat.response.payload.metadata.agent_mood` can include the current agent mood:

```json
{
  "metadata": {
    "source": "agodesk_chat",
    "agent_mood": {
      "mood": "focused",
      "primary_mood": "focused",
      "secondary_mood": "steady",
      "description": "I feel calm and ready to help.",
      "valence": 0.2,
      "arousal": 0.3,
      "confidence": 0.8,
      "recommended_response_style": "calm_and_precise",
      "source": "emotion_history",
      "timestamp": "2026-06-06 12:34:56"
    }
  }
}
```

AgoDesk should pass this object to its voice-model layer as style metadata. It must tolerate missing fields and ignore unknown fields.

When `chat.plan_updates` is negotiated, AuraGo can send:

```json
{
  "type": "chat.plan_update",
  "payload": {
    "session_id": "agodesk:device-123",
    "request_id": "req-1",
    "plan": null
  }
}
```

`plan` is either `null` or the same plan JSON shape used by AuraGo's web chat plan panel. `null` clears the local plan display. The final `chat.response.payload.metadata.plan` may include the latest snapshot for reconciliation.

For the concrete AgoDesk client implementation checklist, see [`agodesk_coding_agent_mood_plan.md`](./agodesk_coding_agent_mood_plan.md).

## Chat Sessions, Stop, And TTS

AgoDesk chat now uses the same AuraGo chat sessions as the Web UI. The WebSocket session id (`agodesk:<device>`) remains the transport/auth session. The active chat conversation is a shared AuraGo session id such as `sess-...`, carried as `conversation_id`.

### List sessions

```json
{
  "id": "sessions-1",
  "type": "chat.sessions.list",
  "timestamp": "2026-06-07T12:00:00Z",
  "payload": {
    "session_id": "agodesk:device-123",
    "limit": 20
  }
}
```

Response:

```json
{
  "type": "chat.sessions",
  "payload": {
    "session_id": "agodesk:device-123",
    "sessions": [
      {
        "id": "sess-abc",
        "preview": "Fix the dashboard widget",
        "created_at": "2026-06-07T10:00:00Z",
        "last_active_at": "2026-06-07T11:00:00Z",
        "message_count": 4
      }
    ]
  }
}
```

### Create or load a conversation

New Chat:

```json
{
  "type": "chat.session.create",
  "payload": {
    "session_id": "agodesk:device-123"
  }
}
```

Load history:

```json
{
  "type": "chat.session.load",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc"
  }
}
```

Both return `chat.session`. A load response includes visible, non-internal messages only:

```json
{
  "type": "chat.session",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "session": {
      "id": "sess-abc",
      "preview": "Fix the dashboard widget",
      "created_at": "2026-06-07T10:00:00Z",
      "last_active_at": "2026-06-07T11:00:00Z",
      "message_count": 4
    },
    "messages": [
      {
        "role": "user",
        "content": "Can you fix the widget?",
        "timestamp": "2026-06-07T10:00:00Z"
      }
    ]
  }
}
```

### Send a chat message

```json
{
  "type": "chat.message",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "text": "Continue here.",
    "role": "user",
    "voice_output": true
  }
}
```

`voice_output` is optional. Send it only when the local TTS mode wants AuraGo voice output and `chat.voice_output` was offered by the server.

Assistant responses, chunks, and plan updates echo `conversation_id` when a conversation is active:

```json
{
  "type": "chat.response",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "req-1",
    "text": "Done.",
    "role": "assistant",
    "metadata": {
      "source": "agodesk_chat"
    }
  }
}
```

### Stop active work

```json
{
  "type": "chat.cancel",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "req-1"
  }
}
```

AuraGo cancels the request context, interrupts the agent session for that `conversation_id`, and replies:

```json
{
  "type": "chat.cancelled",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "req-1",
    "status": "cancelled"
  }
}
```

If nothing is active, `status` is `not_active`. A wrong `session_id` returns `chat.error` with `SESSION_NOT_FOUND`.

### Voice output and audio events

TTS mode should default to `Auto` in AgoDesk:

- `Auto`: if `chat.voice_output` and `chat.audio_events` are negotiated, send `voice_output:true` and play `chat.audio`; otherwise use frontend/native TTS for the final assistant text.
- `AuraGo`: require `chat.voice_output` and `chat.audio_events`; otherwise show a quiet unavailable state and do not fall back silently.
- `Frontend`: speak the final assistant text locally.
- `Off`: do not request or play TTS.

Whenever the user changes speech output in AgoDesk, send the same preference state as the Web Chat speaker toggle:

```json
{
  "type": "chat.voice_output.status",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "speaker_mode": false,
    "mode": "off",
    "reason": "user_disabled"
  }
}
```

`speaker_mode` is canonical and maps to AuraGo's `/api/preferences` `speaker_mode`. `mode` is optional UI metadata; accepted values include `on` and `off`. AuraGo updates the shared voice-mode preference and acknowledges with the same message type:

```json
{
  "type": "chat.voice_output.status",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "speaker_mode": false,
    "mode": "off",
    "reason": "user_disabled",
    "status": "ok"
  }
}
```

When `chat.audio_events` is negotiated, AuraGo may emit:

```json
{
  "type": "chat.audio",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "req-1",
    "path": "/api/agodesk/tts/response.mp3",
    "title": "TTS Audio",
    "mime_type": "audio/mpeg",
    "filename": "response.mp3"
  }
}
```

Clients must queue audio in request order and resolve relative `path` values against the AuraGo origin. For AuraGo-generated TTS, AgoDesk should use the provided `/api/agodesk/tts/<filename>` path directly; it is limited by the server to cached TTS audio files and does not require a Web UI login cookie. Stop must clear this queue and cancel native/frontend speech immediately. Do not log shared keys, session tokens, or local TTS file paths. Render server text as sanitized Markdown or plain text, never raw HTML.

### Chat media events

When `chat.media_events` is negotiated, AuraGo emits non-TTS artifacts with `chat.media`. `chat.audio` remains reserved for AuraGo TTS only. Audio/music files produced by tools such as `send_audio` or `generate_music` are sent as `chat.media` with `kind:"audio"`.

```json
{
  "type": "chat.media",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "req-1",
    "kind": "document",
    "path": "/api/agodesk/media/documents/report.pdf?agodesk_exp=1780833600&agodesk_sig=...",
    "preview_url": "/api/agodesk/media/documents/report.pdf?inline=1&agodesk_exp=1780833600&agodesk_sig=...",
    "title": "Report",
    "mime_type": "application/pdf",
    "filename": "report.pdf",
    "format": "pdf",
    "open_mode": "inline"
  }
}
```

`kind` values include `image`, `audio`, `document`, `video`, `youtube_video`, `stl`, `live_stream`, and `link`. Local media paths are rewritten from protected Web Chat `/files/...` URLs to short-lived signed `/api/agodesk/media/<bucket>/<file>` URLs. The server only serves explicit media buckets (`images`, `generated_images`, `generated_videos`, `audio`, `documents`, and `downloads`), rejects traversal, and rejects missing, expired, or tampered signatures. Clients must use `path` and `preview_url` exactly as provided, including query parameters, and should request a fresh `chat.media` payload if an asset fetch returns `401`. YouTube payloads use `url`, `embed_url`, `video_id`, `title`, `provider:"youtube"`, and optional `start_seconds`; no media asset fetch is needed.

AgoDesk should render inline when practical, provide an "open in folder" or external-open action, and keep `open_mode` as a server suggestion rather than a hard requirement.

### Integration webhosts

When `integrations.webhosts` is negotiated, send:

```json
{
  "type": "integrations.webhosts.list",
  "payload": {
    "session_id": "agodesk:device-123"
  }
}
```

AuraGo replies with the same list the Web Chat integrations drawer uses:

```json
{
  "type": "integrations.webhosts",
  "payload": {
    "session_id": "agodesk:device-123",
    "status": "ok",
    "webhosts": [
      {
        "id": "virtual_desktop",
        "name": "Virtual Desktop",
        "description": "Browser-based virtual desktop",
        "status": "running",
        "url": "/desktop",
        "icon": "expand"
      }
    ]
  }
}
```

Resolve relative URLs against the AuraGo origin. Treat external URLs as normal browser/WebView targets.

### System warnings

When `system.warnings` is negotiated, send `system.warnings.list` with the accepted `session_id`. AuraGo replies with:

```json
{
  "type": "system.warnings",
  "payload": {
    "session_id": "agodesk:device-123",
    "warnings": [
      {
        "id": "warn-1",
        "severity": "warning",
        "title": "Test warning",
        "description": "Something needs attention",
        "category": "system",
        "timestamp": "2026-06-07T12:00:00Z",
        "acknowledged": false
      }
    ],
    "total": 1,
    "unacknowledged": 1
  }
}
```

To acknowledge, send `system.warning.acknowledge` with either `id` or `all:true`. AuraGo responds with a fresh `system.warnings` snapshot and also broadcasts snapshots to connected AgoDesk clients that negotiated `system.warnings` when warnings change.

For concrete AgoDesk client implementation checklists, see [`agodesk_coding_agent_chat_controls.md`](./agodesk_coding_agent_chat_controls.md) for Stop/New Chat/History/TTS and [`agodesk_coding_agent_media_integrations_warnings.md`](./agodesk_coding_agent_media_integrations_warnings.md) for media artifacts, integration webhosts, and system warnings.

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

Screenshots, discovery, UI tree reads, browser connect/snapshot/disconnect, and permission probes do not require local action approval. Input injection, UI actions, and browser actions require explicit local approval via the remote-control banner.

AuraGo accepts both legacy `ok` and current `success/status` result fields:

```json
{
  "command_id": "cmd-1",
  "success": true,
  "status": "ok",
  "data": {}
}
```

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

### Discovery (`remote.desktop.discovery`)

| Operation | Params | Result data |
|---|---|---|
| `desktop_list_displays` | `{}` | `{ "displays": [...] }` |
| `desktop_list_windows` | `{}` | `{ "windows": [...] }` |
| `desktop_active_window` | `{}` | active window object with id, title, process, bounds, and `display_id` |
| `desktop_host_info` | `{}` | host/platform metadata |

### UI automation (`remote.desktop.ui_automation`)

`desktop_ui_tree` returns a normalized accessibility tree for the active/root window or for a supplied `window_id`.

```json
{
  "command_id": "cmd-tree-1",
  "operation": "desktop_ui_tree",
  "params": { "window_id": "win-12345678" }
}
```

`desktop_ui_action` requires local approval and at least an `action` value. Common actions are `click`, `invoke`, `focus`, and `set_value`.

```json
{
  "command_id": "cmd-ui-1",
  "operation": "desktop_ui_action",
  "params": {
    "element_id": "elem-42",
    "action": "set_value",
    "value": "Hello from AuraGo"
  }
}
```

### Browser CDP (`remote.desktop.browser`)

Browser operations require `remote.desktop.browser`. `desktop_browser_action` requires local approval; connect, snapshot, and disconnect are read-only operations.

```json
{
  "command_id": "cmd-browser-1",
  "operation": "desktop_browser_connect",
  "params": { "endpoint": "http://127.0.0.1:9222" }
}
```

```json
{
  "command_id": "cmd-browser-2",
  "operation": "desktop_browser_action",
  "params": {
    "action": "fill",
    "selector": "#name",
    "value": "Ada"
  }
}
```

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

### Search files (`file_search`)

Requires `remote.files.read`.

Supported search operations are `grep`, `grep_recursive`, and `find`.

```json
{
  "command_id": "cmd-search-1",
  "operation": "file_search",
  "params": {
    "root_id": "workspace",
    "operation": "grep_recursive",
    "pattern": "TODO",
    "glob": "**/*.go",
    "output_mode": "content"
  }
}
```

Successful result:

```json
{
  "command_id": "cmd-search-1",
  "ok": true,
  "data": {
    "content": "{\"status\":\"success\",\"data\":[]}"
  }
}
```

AgoDesk should return AuraGo-compatible `FileSearchResult` JSON in `data.content`. Searches are scoped to the granted roots; AgoDesk enforces canonical path checks, index limits, result limits, file-size limits, pattern length limits, and sandbox post-filtering locally.

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
- `desktop_list_displays`
- `desktop_list_windows`
- `desktop_active_window`
- `desktop_host_info`
- `desktop_ui_tree`
- `desktop_ui_action`
- `desktop_browser_connect`
- `desktop_browser_snapshot`
- `desktop_browser_action`
- `desktop_browser_disconnect`
- `file_list`
- `file_read`
- `file_search`
- `file_write`

Read-only policy permits screenshot, permission status requests, discovery, UI tree reads, browser connect/snapshot/disconnect, file listing, file reading, and file search. It denies desktop input, `desktop_ui_action`, `desktop_browser_action`, and file writing before dispatch.

`desktop_stream_start` and `desktop_stream_stop` remain reserved for a later backend version and are not available in v1.

For a concrete client-side implementation checklist, see [`agodesk_coding_agent_file_access.md`](./agodesk_coding_agent_file_access.md).
