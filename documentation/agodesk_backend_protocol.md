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

AuraGo accepts `desktop.result` WebSocket messages up to 16 MiB because screenshot results can include `data_base64`. All other control frames, including `local.agent.*`, are limited to 256 KiB. Clients must downscale or compress desktop screenshots when necessary. File payloads use a stricter v1 inline limit of 8 MiB or the smaller limit negotiated in `session.start.file_access`.

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
- `config.provider.catalog.list` / `config.provider.catalog.detail` / `config.provider.catalog`: provider catalog discovery backed by AuraGo's model catalog.
- `config.providers.list` / `config.providers`: list configured providers with safe secret and OAuth state only.
- `config.provider.get` / `config.provider`: load one configured provider for editing.
- `config.provider.upsert`: create or update one provider using explicit secret operations.
- `config.provider.delete`: remove a provider and its vault secrets.
- `config.provider.test` / `config.provider.test_result`: validate required provider settings and safe credential presence.
- `config.provider.oauth.start` / `config.provider.oauth.started`: start desktop-assisted OAuth with a local AgoDesk loopback redirect URI.
- `config.provider.oauth.complete` / `config.provider.oauth.status`: complete OAuth and return sanitized authorization status.
- `config.provider.oauth.revoke`: delete stored OAuth tokens and return sanitized authorization status.
- `local.agent.remote_tool` / `local.agent.remote_tool.result`: execute one of the narrow AuraGo memory/query adapters without starting the server agent.
- `local.agent.handoff`: hand the current request to the full AuraGo server agent. The normal `chat.response`, `chat.error`, plan, activity, media, and audio events are used for the result.
- `local.agent.turn`: synchronize one turn completed by the local agent into AuraGo history, activity, journal, and eligible memory analysis. This message is fire-and-forget.
- `local.agent.llm` / `local.agent.llm.result`: use one configured AuraGo provider without exporting its credentials and without adding server prompts or tools.
- `agent.activity`: optional server-to-client activity timeline events for agent turns and tool lifecycle updates.
- `desktop.command` / `desktop.result`: server-to-client command transport for screenshots, discovery, UI automation, browser CDP, permission requests, locally approved input/actions, locally approved file access, and locally enabled remote shell commands.

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
- `chat.agent_activity`: enables live `agent.activity` frames for transparent run/tool timelines.
- `chat.sessions`: enables shared AuraGo chat history, New Chat, and loading old conversations.
- `chat.cancel`: enables Stop for active AgoDesk agent turns.
- `chat.audio_events`: enables `chat.audio` frames for server-generated TTS playback.
- `chat.media_events`: enables `chat.media` frames for non-TTS images, audio/music, documents, videos, STL, links, and YouTube embeds.
- `chat.voice_output`: server-offered only when AuraGo TTS is configured; lets AgoDesk request server-side voice output with `chat.message.payload.voice_output=true`.
- `chat.voice_output_status`: enables AgoDesk to report the current chat speech-output state with `chat.voice_output.status`.
- `integrations.webhosts`: enables the Web Chat integrations drawer list over WebSocket.
- `system.warnings`: enables the Web Chat system warnings list and acknowledgement flow over WebSocket.
- `local.agent`: enables the local-agent backend messages documented below. It is negotiated only when the client offers it, remains available to read-only paired devices, and does not grant any permission outside the existing Memory, Provider, Vault, and Budget policies.
- `config.providers.read`: enables provider catalog, configured-provider list, provider detail, and OAuth status reads.
- `config.providers.write`: enables provider create, update, delete, and test commands. AuraGo offers this only when Web Config is enabled, the Vault is available, and the paired AgoDesk device is not read-only.
- `config.providers.oauth`: enables desktop-assisted OAuth start, complete, status, and revoke commands. AuraGo offers this only when Web Config is enabled, the Vault is available, and the paired AgoDesk device is not read-only.
- `remote.desktop.capture`: required for `desktop_screenshot`
- `remote.desktop.permission_request`: required for `desktop_permission_request`
- `remote.desktop.input`: required for `desktop_input`
- `remote.desktop.discovery`: required for `desktop_list_displays`, `desktop_list_windows`, `desktop_active_window`, and `desktop_host_info`
- `remote.desktop.ui_automation`: required for `desktop_ui_tree` and `desktop_ui_action`
- `remote.desktop.browser`: required for `desktop_browser_connect`, `desktop_browser_snapshot`, `desktop_browser_action`, and `desktop_browser_disconnect`
- `remote.files.read`: required for `file_list`, `file_read`, and `file_search`
- `remote.files.write`: required for `file_write` and `file_patch`
- `remote.shell.exec`: required for `shell_exec`; AgoDesk must advertise it only when remote shell is enabled in the local AgoDesk config, and AuraGo must offer/dispatch it only when its own `agent.allow_remote_shell` policy permits remote shell usage.
- `remote.shell.session`: required for `shell_session_start`, `shell_session_read`, `shell_session_input`, `shell_session_stop`, and `shell_session_list`; AgoDesk must advertise it only when persistent local shell sessions are enabled, and AuraGo offers it only when `agent.allow_remote_shell=true`.

If a client omits these capabilities, pairing, heartbeat, persona assets, and chat can still work, but remote commands return `UNSUPPORTED_CAPABILITY` immediately instead of waiting for a `desktop.result` timeout. A client that sends keepalives but does not advertise the desktop, file, or shell capabilities is connected, but only capable of the features it advertised.

## Local Agent Backend

The `local.agent` capability lets AgoDesk run a local tool loop while retaining AuraGo as the authority for configured providers, Vault credentials, memory, journal, activity, and full server-agent handoff.

All local-agent requests require:

- an accepted paired `session_id` matching the active WebSocket;
- a non-empty `request_id` generated by AgoDesk;
- `local.agent` in `session.accepted.advertised_capabilities`;
- a frame no larger than 256 KiB.

Collections in `local.agent.llm` and handoff transcripts are limited to 128 messages or tools. Roles, function names, JSON arguments, status values, and RFC3339 timestamps are validated. Unknown incoming message types such as the reserved `local.agent.activity` remain ignored in v1.

Only `local.agent.handoff` starts `ExecuteAgentLoop`. `local.agent.turn`, remote tools, and the LLM proxy never start the server agent. A normal `chat.message` keeps its existing behavior even while the client uses local-agent mode.

### Remote tools

Request:

```json
{
  "type": "local.agent.remote_tool",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "local-tool-1",
    "tool": "memory_search",
    "arguments": {
      "query": "What did we decide about backups?",
      "limit": 5
    }
  }
}
```

Supported tools:

- `memory_search`: requires string `query`; `limit` defaults to `5` and must be between `1` and `20`.
- `memory_get`: requires exactly one string `id` or `key`. Both names address the same AuraGo memory identifier.
- `query_aurago`: requires string `question` and accepts optional string `context`. AuraGo obtains normal memory context and performs exactly one direct, tool-free completion with the active AuraGo chat provider. The question, optional context, and memory context are wrapped as untrusted `external_data`.

Successful response:

```json
{
  "type": "local.agent.remote_tool.result",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "local-tool-1",
    "result": {
      "status": "success",
      "results": []
    }
  }
}
```

`query_aurago` returns:

```json
{
  "type": "local.agent.remote_tool.result",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "local-query-1",
    "result": {
      "text": "The backup retention was set to 30 days.",
      "sources": ["journal", "core_memory"]
    }
  }
}
```

Memory operations have a 30-second timeout. `query_aurago` has a 60-second timeout. Memory configuration and tool toggles remain authoritative; disabled memory returns `MEMORY_DISABLED`.

### Full server-agent handoff

```json
{
  "type": "local.agent.handoff",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "handoff-1",
    "user_message": "Finish the deployment and verify the service.",
    "transcript": [
      {"role": "user", "content": "Inspect the deployment files."},
      {"role": "assistant", "content": "The files are ready, but deployment needs server access."}
    ],
    "voice_output": false
  }
}
```

`transcript` is optional and accepts only `user` and `assistant` roles. AuraGo attaches it to the current request as untrusted context; it is never promoted to a system prompt and is not persisted as duplicate history. `user_message` is persisted as the normal user message.

The handoff uses the same conversation lock, cancellation registration, 30-minute timeout, activity, plan, media, audio, and response path as `chat.message`. Its result is the normal `chat.response` with `payload.request_id` equal to the handoff `request_id`. Reusing an accepted handoff ID does not start a second run.

### Turn synchronization

After a local turn finishes, AgoDesk sends:

```json
{
  "type": "local.agent.turn",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "turn-42",
    "user_message": "Check the README.",
    "assistant_message": "The README is current.",
    "status": "completed",
    "provider_id": "main",
    "model": "example-model",
    "client_timestamp": "2026-07-17T10:00:00Z",
    "tools": [
      {
        "tool": "workspace_search",
        "target": "README.md",
        "status": "completed"
      }
    ]
  }
}
```

Rules:

- `status` and every tool status must be `completed`, `failed`, or `cancelled`.
- `client_timestamp` must be RFC3339.
- Completed turns require both `user_message` and `assistant_message`; failed and cancelled turns may omit the assistant message.
- `tool`, `target`, and `status` are short summaries only. Never send file contents, shell output, environment variables, credentials, or full tool results.
- Ordinary local turns persist visible user and assistant history and update the shared session preview.
- A turn whose `request_id` matches a prior handoff synchronizes activity/journal data only; it does not duplicate the handoff conversation messages.
- Successful turns use AuraGo's existing auto-journal and memory-analysis flow. Failed and cancelled turns receive a status-aware activity journal entry and no long-term fact extraction.
- The message has no response. Errors are scrubbed and logged server-side. Duplicate `request_id` values on the same connection are ignored.

Activity rows use `source` and `channel` value `agodesk_local_agent`, preserve the validated client timestamp and provider/model hint, and store only bounded scrubbed tool summaries.

### LLM proxy

The LLM proxy lets the local agent use a configured AuraGo provider while provider secrets remain inside AuraGo:

```json
{
  "type": "local.agent.llm",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "llm-1",
    "provider_id": "main",
    "model": "requested-model",
    "messages": [
      {"role": "user", "content": "Read the requested file."},
      {
        "role": "assistant",
        "tool_calls": [
          {
            "id": "call-1",
            "name": "read_file",
            "arguments": {"path": "README.md"}
          }
        ]
      },
      {
        "role": "tool",
        "name": "read_file",
        "tool_call_id": "call-1",
        "content": "file contents supplied by AgoDesk"
      }
    ],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "read_file",
          "description": "Read one locally authorized file.",
          "parameters": {
            "type": "object",
            "properties": {"path": {"type": "string"}},
            "required": ["path"]
          }
        }
      }
    ]
  }
}
```

Allowed message roles are `system`, `user`, `assistant`, and `tool`. `name`, `tool_call_id`, and prior `tool_calls` are preserved so the client can continue multi-step local tool loops. Tool-call `arguments` and function `parameters` must be valid JSON; parameters must be a JSON object.

`provider_id` must exactly match an entry in AuraGo's configured provider list. AuraGo uses the requested `model`; only an empty model falls back to that provider's configured model. No server system prompt, agent prompt, native server tool, or automatic tool choice is added.

Response:

```json
{
  "type": "local.agent.llm.result",
  "payload": {
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "llm-1",
    "message": {
      "role": "assistant",
      "content": "",
      "tool_calls": [
        {
          "id": "call-2",
          "name": "read_file",
          "arguments": {"path": "next.txt"}
        }
      ]
    },
    "usage": {
      "prompt_tokens": 21,
      "completion_tokens": 7,
      "total_tokens": 28
    }
  }
}
```

Only the first assistant choice is returned. The request has a three-minute timeout. AuraGo checks the `chat` budget before the provider call and records successful usage in the `chat` category.

### Local-agent errors

Remote-tool and LLM-proxy failures always use their respective `.result` type, preserve `request_id`, omit `result`/`message`, and include:

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Safe client-facing message."
  }
}
```

Stable local-agent error codes are:

- `INVALID_REQUEST`
- `UNSUPPORTED_TOOL`
- `MEMORY_DISABLED`
- `PROVIDER_NOT_FOUND`
- `BUDGET_BLOCKED`
- `TIMEOUT`
- `UPSTREAM_ERROR`
- `INTERNAL_ERROR`

Existing transport policy errors such as `PAIRING_REQUIRED` and `UNSUPPORTED_CAPABILITY` may be returned before request execution. Provider response bodies, Vault values, API keys, and raw internal errors are never included.

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
- AuraGo caps `file_read` / `file_write` / `file_patch` inline command limits to 8 MiB or the smaller negotiated `max_read_bytes` / `max_write_bytes`, rejects known disabled or denied `root_id` cases for `file_list`, `file_read`, `file_search`, `file_write`, and `file_patch`, and still requires AgoDesk to enforce canonical path checks and permissions locally for every command.

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
- Store the remote-shell enablement flag, allowed working directories, shell choice, timeout, output limit, and approval policy locally in AgoDesk. AuraGo does not configure local shell access.
- Advertise `remote.shell.exec` only when local remote shell access is enabled. Execute `shell_exec` only after re-checking local config, capability negotiation, working-directory policy, timeout/output limits, and any required local user approval.
- Audit shell command id, cwd id/display, exit code, duration, timeout/truncation flags, and status. Do not log full command output, environment variables, secrets, or shell history.

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

When `chat.agent_activity` is negotiated, AuraGo can send transparent run and tool lifecycle events:

```json
{
  "type": "agent.activity",
  "payload": {
    "activity_id": "act-123",
    "parent_activity_id": "agent:req-1",
    "session_id": "agodesk:device-123",
    "conversation_id": "sess-abc",
    "request_id": "req-1",
    "kind": "shell",
    "phase": "started",
    "title": "remote_control_shell",
    "summary": "remote_control_shell started",
    "risk": "write"
  }
}
```

Run-level activities use `activity_id="agent:<request_id>"` and phases `started`, `completed`, `failed`, or `cancelled`. Tool activities use the stable AuraGo agent-action id and map action states to `queued`, `started`, `waiting_approval`, `completed`, `failed`, or `cancelled`. Summaries are deliberately short and scrubbed; AgoDesk must not expect full shell output, tool arguments, secrets, or file contents in these events.

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

## Provider Management And Desktop OAuth

AgoDesk can manage AuraGo providers over the primary WebSocket after `session.accepted`. This feature deliberately reuses the existing AgoDesk pairing, read-only device policy, capability negotiation, Vault-backed secret storage, and model catalog data.

AuraGo offers provider capabilities as follows:

- `config.providers.read` when `web_config.enabled` is true.
- `config.providers.write` only when `web_config.enabled` is true, the Vault is available, and the accepted AgoDesk device is not read-only.
- `config.providers.oauth` only when `web_config.enabled` is true, the Vault is available, and the accepted AgoDesk device is not read-only.

Every mutating command still checks these conditions server-side, even if the client sends an old or forged capability list.

### Safe provider shape

Configured provider responses never contain real secrets, access tokens, refresh tokens, auth codes, PKCE verifiers, or vault keys. They expose only safe state:

```json
{
  "id": "main",
  "name": "Main",
  "type": "openai",
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4o-mini",
  "account_id": "",
  "auth_type": "api_key",
  "oauth_auth_url": "",
  "oauth_token_url": "",
  "oauth_client_id": "",
  "oauth_scopes": "",
  "capabilities": {
    "auto": true,
    "tool_calling": false,
    "structured_outputs": false,
    "multimodal": false
  },
  "effective_capabilities": {
    "auto": true,
    "tool_calling": true,
    "structured_outputs": true,
    "multimodal": true,
    "source": "model"
  },
  "secrets": {
    "api_key": { "present": true },
    "oauth_client_secret": { "present": false }
  },
  "oauth": {
    "provider_id": "main",
    "configured": false,
    "authorized": false,
    "has_refresh_token": false,
    "missing_fields": ["auth_type"]
  },
  "references": [
    { "path": "llm.provider", "role": "primary_llm" },
    { "path": "image_generation.provider", "role": "image_generation" }
  ]
}
```

`references` identifies config slots currently pointing at the provider, such as `llm.provider`, `llm.helper_provider`, `vision.provider`, `whisper.provider`, `embeddings.provider`, `llm_guardian.provider`, `mission_preparation.provider`, `image_generation.provider`, `music_generation.provider`, `video_generation.provider`, and `a2a.llm.provider`.

### Catalog

Provider catalog list:

```json
{
  "type": "config.provider.catalog.list",
  "payload": {
    "session_id": "agodesk:device-123",
    "include_models": false
  }
}
```

Provider catalog detail:

```json
{
  "type": "config.provider.catalog.detail",
  "payload": {
    "session_id": "agodesk:device-123",
    "provider_id": "google",
    "include_models": true
  }
}
```

Both return `config.provider.catalog` with `metadata`, `providers`, and optional `models`. The provider objects mirror `/api/models/catalog` availability and include `oauth_setup` metadata from the bundled model catalog when available:

```json
{
  "type": "config.provider.catalog",
  "payload": {
    "session_id": "agodesk:device-123",
    "status": "ok",
    "enabled": true,
    "metadata": {
      "package_name": "oh-my-pi",
      "version": "..."
    },
    "providers": [
      {
        "id": "google",
        "aura_provider_type": "google",
        "name": "Google",
        "default_model": "gemini-2.5-flash",
        "oauth_provider": "google",
        "oauth_setup": {
          "flow": "authorization_code_pkce",
          "auth_url": "https://accounts.google.com/o/oauth2/v2/auth",
          "token_url": "https://oauth2.googleapis.com/token",
          "client_id": "optional-public-client-id",
          "scopes": ["openid", "profile", "email"],
          "callback_port": 8088,
          "callback_path": "/oauth/callback"
        },
        "available": false,
        "availability": "missing_credentials",
        "models_count": 8
      }
    ]
  }
}
```

AgoDesk must not hard-code Google, OpenRouter, or other provider-specific OAuth UI. Use `oauth_setup` to prefill setup URLs, scopes, endpoint fields, callback port/path hints, and labels.

### List and edit configured providers

List:

```json
{
  "type": "config.providers.list",
  "payload": {
    "session_id": "agodesk:device-123"
  }
}
```

Response:

```json
{
  "type": "config.providers",
  "payload": {
    "session_id": "agodesk:device-123",
    "status": "ok",
    "providers": []
  }
}
```

Load one provider:

```json
{
  "type": "config.provider.get",
  "payload": {
    "session_id": "agodesk:device-123",
    "provider_id": "main"
  }
}
```

Response is `config.provider` with one safe provider object.

Upsert uses explicit secret operations. Password/secret inputs in AgoDesk must be empty by default; never prefill masked values. Use `op:"keep"` unless the user entered a new value or explicitly cleared the field.

```json
{
  "type": "config.provider.upsert",
  "payload": {
    "session_id": "agodesk:device-123",
    "mode": "update",
    "provider": {
      "id": "main",
      "name": "Main",
      "type": "openai",
      "base_url": "https://api.openai.com/v1",
      "model": "gpt-4o-mini",
      "auth_type": "api_key"
    },
    "secrets": {
      "api_key": { "op": "set", "value": "new-secret" },
      "oauth_client_secret": { "op": "clear" }
    }
  }
}
```

Supported secret ops are:

- `keep`: preserve the current Vault value.
- `set`: write `value` to the Vault.
- `clear`: delete the Vault value.

For OAuth providers, static API keys are cleared and OAuth tokens are stored only under `oauth_<provider_id>` in AuraGo's Vault. OAuth client secrets, when required by the provider, are stored under `provider_<provider_id>_oauth_client_secret`.

Delete:

```json
{
  "type": "config.provider.delete",
  "payload": {
    "session_id": "agodesk:device-123",
    "provider_id": "spare",
    "force": false
  }
}
```

AuraGo rejects delete when the provider is still referenced unless `force:true` is sent. Successful delete returns `config.providers` and removes `provider_<id>_api_key`, `provider_<id>_oauth_client_secret`, and `oauth_<id>` from the Vault.

Test:

```json
{
  "type": "config.provider.test",
  "payload": {
    "session_id": "agodesk:device-123",
    "provider_id": "main"
  }
}
```

Response:

```json
{
  "type": "config.provider.test_result",
  "payload": {
    "session_id": "agodesk:device-123",
    "provider_id": "main",
    "status": "ok",
    "ok": true,
    "message": "Provider configuration looks usable."
  }
}
```

### Desktop-assisted OAuth

AgoDesk runs on the desktop, so the happy path must not require the user to copy/paste a callback URL into AuraGo. AgoDesk starts a local loopback HTTP listener, opens the authorization URL in the system browser or embedded WebView, catches the redirect, and sends the result back over the WebSocket.

Start:

```json
{
  "type": "config.provider.oauth.start",
  "payload": {
    "session_id": "agodesk:device-123",
    "provider_id": "google",
    "redirect_uri": "http://127.0.0.1:8088/oauth/callback"
  }
}
```

AuraGo validates that `redirect_uri` is HTTP loopback, stores a PKCE session with mode `agodesk_loopback`, and returns:

```json
{
  "type": "config.provider.oauth.started",
  "payload": {
    "session_id": "agodesk:device-123",
    "provider_id": "google",
    "auth_url": "https://accounts.google.com/o/oauth2/v2/auth?...",
    "mode": "agodesk_loopback",
    "oauth_state": "state-value",
    "expires_at": "2026-06-25T12:10:00Z",
    "fallback_modes": ["manual_paste"],
    "redirect_uri": "http://127.0.0.1:8088/oauth/callback"
  }
}
```

Complete with the full redirect URL:

```json
{
  "type": "config.provider.oauth.complete",
  "payload": {
    "session_id": "agodesk:device-123",
    "provider_id": "google",
    "redirect_url": "http://127.0.0.1:8088/oauth/callback?code=...&state=..."
  }
}
```

Or complete with parsed values:

```json
{
  "type": "config.provider.oauth.complete",
  "payload": {
    "session_id": "agodesk:device-123",
    "provider_id": "google",
    "redirect_uri": "http://127.0.0.1:8088/oauth/callback",
    "code": "...",
    "state": "..."
  }
}
```

AuraGo validates state, provider id, expiry, session mode, and redirect URI before exchanging the code. Tokens are stored only in the Vault. The response is sanitized `config.provider.oauth.status`:

```json
{
  "type": "config.provider.oauth.status",
  "payload": {
    "session_id": "agodesk:device-123",
    "provider_id": "google",
    "status": "ok",
    "configured": true,
    "authorized": true,
    "expired": false,
    "expiry": "2026-06-25T13:10:00Z",
    "has_refresh_token": true,
    "missing_fields": [],
    "redirect_uri": "http://127.0.0.1:8088/oauth/callback",
    "mode": "agodesk_loopback",
    "message": "Authorization successful."
  }
}
```

Status:

```json
{
  "type": "config.provider.oauth.status",
  "payload": {
    "session_id": "agodesk:device-123",
    "provider_id": "google",
    "redirect_uri": "http://127.0.0.1:8088/oauth/callback"
  }
}
```

Revoke:

```json
{
  "type": "config.provider.oauth.revoke",
  "payload": {
    "session_id": "agodesk:device-123",
    "provider_id": "google"
  }
}
```

AgoDesk must never log authorization callback query strings after redirect, authorization codes, tokens, client secrets, API keys, PKCE verifier values, Vault keys, or complete token-exchange errors that might include provider response bodies.

If provider login or consent is required by the upstream provider, AgoDesk may show the browser or WebView. The happy path still must not require a copy/paste step or an AuraGo-side manual callback action.

## Coding Agent Instruction: Provider Settings

When modifying the AgoDesk desktop client, implement provider settings against this WebSocket protocol.

1. Advertise `config.providers.read`, `config.providers.write`, and `config.providers.oauth` in `session.start.client_capabilities`.
2. After `session.accepted`, use `advertised_capabilities`; hide or disable provider UI actions that were not negotiated.
3. Build a Provider settings UI from `config.providers.list`. Show existing providers, active/reference badges, auth mode, OAuth status, model, base URL, and missing configuration warnings.
4. On edit, request `config.provider.get`. Never prefill password fields with masks.
5. Use `secret op=keep` unless the user enters a new secret or explicitly clears it.
6. Build "Add from catalog" with `config.provider.catalog.list`, then request `config.provider.catalog.detail` for the selected provider and prefill type, name, base URL, model, OAuth endpoints, scopes, and callback hints from AuraGo data.
7. Start a local loopback listener using `oauth_setup.callback_port` and `oauth_setup.callback_path` when present, otherwise choose a safe localhost fallback.
8. Send `config.provider.oauth.start` with the exact local `redirect_uri`, open `auth_url`, catch the callback, then send `config.provider.oauth.complete` with the full redirect URL or parsed `code` and `state`.
9. Close the loopback listener, refresh provider detail/status, and keep secrets out of logs, local state, telemetry, and crash reports.
10. Commit AgoDesk client changes separately from AuraGo backend changes and include protocol fixture tests.

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

### Patch file (`file_patch`)

Requires `remote.files.write`.

`file_patch` is the preferred precise edit primitive for AgoDesk file access. AuraGo forwards exact replacements and policy metadata; AgoDesk owns the local diff preview, user approval when required, hash validation, and atomic write.

```json
{
  "command_id": "cmd-patch-1",
  "operation": "file_patch",
  "params": {
    "root_id": "workspace",
    "path": "src/main.go",
    "expected_sha256": "b1946ac92492d2347c6235b4d2611184...",
    "dry_run": true,
    "patches": [
      {
        "old_text": "socket.connect();",
        "new_text": "await socket.connect();",
        "expected_occurrences": 1
      }
    ]
  }
}
```

Rules:

- `expected_sha256` is required. Return `FILE_HASH_MISMATCH` if the current file hash differs.
- `dry_run` defaults to `true`; only apply changes when AuraGo explicitly sends `dry_run:false`.
- `patches` is an ordered array of exact `{old_text,new_text,expected_occurrences}` replacements. Return `FILE_PATCH_MISMATCH` if an expected occurrence count does not match.
- Return structured mismatch details in `data` when safe, but do not include full file contents or secrets in `message`.

File command errors use `ok=false` and a stable error code in `error`, for example `FILE_ACCESS_DISABLED`, `FILE_ACCESS_DENIED`, `FILE_TOO_LARGE`, `FILE_CONFLICT`, `FILE_PATCH_MISMATCH`, or `FILE_HASH_MISMATCH`. Do not include file contents in error messages or logs.

Inline file payloads are limited to 8 MiB in v1 or the smaller value from `file_access.max_read_bytes` / `file_access.max_write_bytes`. Larger transfers should return `FILE_TOO_LARGE`; chunked transfer is reserved for a later protocol version.

## Remote Shell Access Metadata

AgoDesk owns local shell execution permissions. If local remote shell access is available, include `payload.shell_access` in `session.start` and advertise `remote.shell.exec` / `remote.shell.session` only when `shell_access.enabled=true` and the matching local shell mode is enabled:

```json
{
  "client_version": "agodesk-1.3.0",
  "client_capabilities": ["chat.full_response", "remote.shell.exec", "remote.shell.session"],
  "shell_access": {
    "enabled": true,
    "requires_approval": true,
    "default_cwd": "~",
    "allowed_cwds": [
      {
        "cwd_id": "workspace",
        "label": "Workspace",
        "path_display": "~/Projects/AuraGo"
      }
    ],
    "shells": ["powershell", "cmd"],
    "max_command_chars": 4000,
    "max_output_bytes": 1048576,
    "default_timeout_ms": 30000,
    "max_timeout_ms": 120000
  }
}
```

Rules:

- `shell_access` is optional for backward compatibility.
- `enabled=false` means AgoDesk must not advertise `remote.shell.exec` or `remote.shell.session`.
- AuraGo must not dispatch `shell_exec` or `shell_session_*` unless both sides allow it: AgoDesk advertises the matching shell capability, and AuraGo server policy has remote shell enabled (`agent.allow_remote_shell=true`).
- `requires_approval=true` means AgoDesk must show a local approval prompt before running each command. If the user denies or the prompt expires, return `SHELL_APPROVAL_DENIED`.
- `default_cwd` and `allowed_cwds.path_display` are UI/debug metadata. AuraGo must not treat them as authorization boundaries.
- `cwd_id` is stable for the local AgoDesk configuration and is used in later commands.
- AgoDesk must enforce canonical working-directory checks locally for every command because AuraGo never receives the real local paths.
- AgoDesk must enforce command length, timeout, and output limits locally even if AuraGo already clipped a request.
- Do not send or log environment variables, secrets, keychain paths, shell history, or full command output in normal logs. Audit command id, cwd id/display, duration, exit code, truncation, and status only.

## Remote Shell Commands

Shell commands reuse the existing `desktop.command` / `desktop.result` envelope pair. AgoDesk must execute them only when local remote shell access is enabled and the requested working directory resolves inside an allowed shell working directory. Shell execution is never read-only safe.

### Execute shell command (`shell_exec`)

Requires `remote.shell.exec`.

```json
{
  "command_id": "cmd-shell-1",
  "operation": "shell_exec",
  "params": {
    "command": "git status --short",
    "cwd_id": "workspace",
    "timeout_ms": 30000
  }
}
```

Successful result:

```json
{
  "command_id": "cmd-shell-1",
  "ok": true,
  "data": {
    "exit_code": 0,
    "stdout": " M src/main.ts\n",
    "stderr": "",
    "duration_ms": 145,
    "timed_out": false,
    "truncated": false,
    "cwd_display": "~/Projects/AuraGo",
    "shell": "powershell"
  }
}
```

`ok=true` means the command was launched and completed. Non-zero `exit_code` values are command results, not transport failures. Use `ok=false` for policy, validation, approval, spawn, timeout, or protocol errors:

```json
{
  "command_id": "cmd-shell-1",
  "ok": false,
  "error": "SHELL_ACCESS_DENIED",
  "message": "remote shell is disabled for this AgoDesk profile"
}
```

Stable shell error codes:

- `SHELL_ACCESS_DISABLED`: local AgoDesk shell access is off.
- `SHELL_ACCESS_DENIED`: capability, cwd, policy, or read-only state denies the request.
- `SHELL_APPROVAL_REQUIRED`: local approval is required but has not been granted yet.
- `SHELL_APPROVAL_DENIED`: the local user denied or ignored the command approval prompt.
- `SHELL_COMMAND_REJECTED`: command is empty, too long, blocked by local policy, or contains a locally forbidden pattern.
- `SHELL_TIMEOUT`: command exceeded the effective timeout and was terminated.
- `SHELL_OUTPUT_TOO_LARGE`: output exceeded the negotiated limit before truncation could produce a safe response.
- `SHELL_SPAWN_FAILED`: the configured shell process could not be started.

`shell_exec_stream` remains reserved for a later protocol version and is not available in v1.

### Persistent shell sessions (`shell_session_*`)

Requires `remote.shell.session`.

Use `shell_exec` for one-shot commands. Use shell sessions only for interactive or long-running processes where the agent needs to poll output or send input.

Start a session:

```json
{
  "command_id": "cmd-session-start-1",
  "operation": "shell_session_start",
  "params": {
    "command": "npm run dev",
    "cwd_id": "workspace",
    "initial_wait_ms": 1000
  }
}
```

Read output:

```json
{
  "command_id": "cmd-session-read-1",
  "operation": "shell_session_read",
  "params": {
    "session_id": "sh-abc",
    "offset": -2000,
    "limit": 2000,
    "wait_ms": 250
  }
}
```

Send input, stop, or list sessions:

```json
{"command_id":"cmd-session-input-1","operation":"shell_session_input","params":{"session_id":"sh-abc","input":"q"}}
{"command_id":"cmd-session-stop-1","operation":"shell_session_stop","params":{"session_id":"sh-abc"}}
{"command_id":"cmd-session-list-1","operation":"shell_session_list","params":{"limit":10}}
```

`initial_wait_ms` is only the initial read wait after process start. It is not a session lifetime. AgoDesk owns process storage, reconnect behavior, output buffering, approval, timeout, and cleanup. AuraGo does not persist session processes; after reconnect, it may only send `shell_session_list`, `shell_session_read`, `shell_session_input`, or `shell_session_stop` after normal capability and policy checks.

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
- `file_patch`
- `shell_exec`
- `shell_session_start`
- `shell_session_read`
- `shell_session_input`
- `shell_session_stop`
- `shell_session_list`

Read-only policy permits screenshot, permission status requests, discovery, UI tree reads, browser connect/snapshot/disconnect, file listing, file reading, and file search. It denies desktop input, `desktop_ui_action`, `desktop_browser_action`, file writing, `file_patch`, `shell_exec`, and every `shell_session_*` operation before dispatch.

`desktop_stream_start`, `desktop_stream_stop`, and `shell_exec_stream` remain reserved for a later backend version and are not available in v1.

For concrete client-side implementation checklists, see [`agodesk_coding_agent_file_access.md`](./agodesk_coding_agent_file_access.md) and [`agodesk_coding_agent_remote_shell.md`](./agodesk_coding_agent_remote_shell.md).
