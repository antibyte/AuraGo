# AgoDesk Coding Agent: Chat Controls, History, And TTS

Implement Stop, New Chat, History, and TTS against the AuraGo WebSocket protocol. Use shared AuraGo chat sessions (`sess-*`) for the visible conversation. Keep the AgoDesk WebSocket `session_id` (`agodesk:<device>`) only as the transport/auth session.

## Protocol Setup

1. Extend WebSocket models with tolerant parsing for unknown fields.
2. Add client capabilities:
   - `chat.sessions`
   - `chat.cancel`
   - `chat.audio_events`
   - `chat.media_events`
   - `chat.voice_output`
   - `chat.voice_output_status`
   - `integrations.webhosts`
   - `system.warnings`
3. After `session.accepted`, store `advertised_capabilities` as the negotiated feature set.
4. After `session.accepted`, send `persona.assets.request` as before.
5. If `chat.sessions` is negotiated, send `chat.sessions.list`, then select the last locally stored `conversation_id` or send `chat.session.create`.
6. If `integrations.webhosts` is negotiated, send `integrations.webhosts.list`.
7. If `system.warnings` is negotiated, send `system.warnings.list`.

## Chat State

Track this local state:

```ts
type AgoDeskChatState = {
  transportSessionId: string;
  activeConversationId?: string;
  activeRequestId?: string;
  requestInFlight: boolean;
  negotiatedCapabilities: Set<string>;
  sessions: ChatSessionSummary[];
  messagesByConversation: Map<string, ChatMessage[]>;
  mediaByConversation: Map<string, ChatMediaItem[]>;
  integrationWebhosts: WebhostIntegration[];
  systemWarnings: SystemWarning[];
  ttsMode: "auto" | "aurago" | "frontend" | "off";
  speakerMode: boolean;
};
```

Persist only the last active `conversation_id`, TTS mode, and UI preferences locally. Do not store shared keys, tokens, or local TTS file paths in logs or debug dumps.

## New Chat

1. User clicks New Chat.
2. Send `chat.session.create` with the accepted `session_id`.
3. On `chat.session`, set `activeConversationId`, clear the chat transcript, refresh the history list, and focus the input.
4. Store the new `conversation_id` locally.

## History

1. Render `chat.sessions` as a history list with preview, relative/absolute time, and message count.
2. On click, send `chat.session.load` with `session_id` and `conversation_id`.
3. On `chat.session`, replace the visible transcript with returned messages. The server returns only non-internal messages.
4. Sanitize Markdown/text. Never render server text as raw HTML.

## Sending Messages

Every `chat.message` must include:

```json
{
  "session_id": "agodesk:device-123",
  "conversation_id": "sess-abc",
  "text": "Hello",
  "role": "user"
}
```

If no `conversation_id` exists and `chat.sessions` is negotiated, create a session before sending. Store `request_id` from the envelope id as `activeRequestId` until the final response, cancellation, or error.

## Stop Button

1. Show and enable Stop only while `requestInFlight` is true.
2. On click, immediately stop local audio playback and frontend/native TTS.
3. Send `chat.cancel`:

```json
{
  "session_id": "agodesk:device-123",
  "conversation_id": "sess-abc",
  "request_id": "req-1"
}
```

4. Mark the local assistant output as stopped without waiting for the server.
5. On `chat.cancelled`, clear `requestInFlight` and `activeRequestId`.
6. If the server returns `SESSION_NOT_FOUND`, reset pairing/session state and require reconnect.

## TTS

Expose TTS mode with default `Auto`:

- `Auto`: if both `chat.voice_output` and `chat.audio_events` are negotiated, send `voice_output:true` and play `chat.audio`. If no server audio arrives for the final assistant response, use frontend/native TTS.
- `AuraGo`: send `voice_output:true` only when both capabilities are negotiated. If unavailable, show an unavailable state and keep silent.
- `Frontend`: do not send `voice_output`; speak the final assistant text locally.
- `Off`: do not request or play TTS.

When the user changes AgoDesk speech output, immediately notify AuraGo with the same preference used by Web Chat:

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

Implementation rules:

1. Treat `speaker_mode` as canonical. It maps directly to AuraGo Web Chat `/api/preferences` `speaker_mode`.
2. Send `speaker_mode:true` for enabled speech output and `speaker_mode:false` when speech output is disabled.
3. Set `mode` to `on` or `off` for clarity. Do not invent provider-specific mode names in this protocol field.
4. On `chat.voice_output.status` acknowledgement with `status:"ok"`, keep local state as-is. If the ack differs, update local state to the acknowledged `speaker_mode`.
5. When switching to `Off`, stop active audio, clear queued audio, cancel frontend/native speech, and send `chat.voice_output.status` before the next `chat.message`.
6. New `chat.message` requests should only include `voice_output:true` when `speakerMode` is true and the selected TTS mode needs AuraGo audio.

Audio handling:

1. Queue `chat.audio` by `request_id`.
2. Resolve relative `path` values against the AuraGo origin.
3. Use AuraGo-provided `/api/agodesk/tts/<filename>` paths directly for server TTS; do not rewrite them to `/tts/` because `/tts/` follows Web UI session auth.
4. Play only while the matching request/conversation is still active.
5. Stop clears queued audio, stops active audio, and cancels frontend/native speech.
6. Do not log local file paths or server audio URLs in normal logs.

## Media Events

Handle `chat.media` only when `chat.media_events` is negotiated. `chat.audio` is AuraGo TTS only; tool audio and generated music arrive as `chat.media` with `kind:"audio"`.

Supported `kind` values:

- `image`: render inline and offer an open-folder/file action.
- `audio`: play in a local audio player or queue; use title/filename metadata and offer an open-folder/file action.
- `document`: render `preview_url` inline when possible; otherwise show a file action.
- `video` and `live_stream`: embed a player when WebView support allows it.
- `youtube_video`: prefer `embed_url`; if WebView or CSP blocks embedding, open `url` externally.
- `stl`: render inline only if the client has a viewer; otherwise show a file action.
- `link`: open externally or in AgoDesk's integration/webview surface.

Asset rules:

1. Resolve relative `path` and `preview_url` values against the AuraGo origin.
2. Use `/api/agodesk/media/...` paths exactly as provided.
3. Do not rewrite these paths back to `/files/...`; `/files/...` requires a Web UI login cookie.
4. Stop should stop active audio/video playback for the current request.
5. Render titles, captions, and descriptions as text/sanitized Markdown only.

## Integration Webhosts

On `integrations.webhosts`, replace the local integrations list with `payload.webhosts`. Each item has `id`, `name`, `description`, `status`, `url`, and `icon`.

Render the same conceptual drawer/list as Web Chat:

- Show running/starting status.
- Resolve relative URLs against the AuraGo origin.
- Open integrations in an embedded WebView when possible; offer external-open fallback.
- Refresh the list after reconnect and when the user opens the integrations surface.

## System Warnings

On `system.warnings`, replace the local warnings snapshot with `payload.warnings` and update badge counts from `total` and `unacknowledged`.

Warning UI rules:

1. Render severity, title, description, category, timestamp, and acknowledged state.
2. Acknowledge one warning with `system.warning.acknowledge` and `id`.
3. Acknowledge all warnings with `system.warning.acknowledge` and `all:true`.
4. Treat any incoming `system.warnings` as authoritative, including broadcasts caused by Web Chat or another AgoDesk client.
5. Never render warning descriptions as raw HTML.

## Acceptance Criteria

- AgoDesk stores negotiated capabilities after `session.accepted`.
- New Chat creates a `sess-*` conversation and focuses input.
- History lists sessions and loads old messages.
- Every new-client `chat.message` includes `session_id` and `conversation_id`.
- Stop sends `chat.cancel`, updates the UI immediately, and handles `chat.cancelled`.
- Speech-output changes send `chat.voice_output.status` with `speaker_mode`.
- Auto TTS uses AuraGo audio when available and frontend/native TTS otherwise.
- `chat.media` renders images, documents, audio/music, video, STL, links, and YouTube without requiring Web UI cookies.
- Integrations show the same webhost list as the Web Chat integrations drawer.
- System warnings show the same warning list/counts as Web Chat and can acknowledge one or all warnings.
- Older AuraGo servers without these capabilities still allow basic chat.
- Server text is sanitized Markdown/plain text only.
