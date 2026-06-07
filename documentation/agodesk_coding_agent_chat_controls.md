# AgoDesk Coding Agent: Chat Controls, History, And TTS

Implement Stop, New Chat, History, and TTS against the AuraGo WebSocket protocol. Use shared AuraGo chat sessions (`sess-*`) for the visible conversation. Keep the AgoDesk WebSocket `session_id` (`agodesk:<device>`) only as the transport/auth session.

## Protocol Setup

1. Extend WebSocket models with tolerant parsing for unknown fields.
2. Add client capabilities:
   - `chat.sessions`
   - `chat.cancel`
   - `chat.audio_events`
   - `chat.voice_output`
   - `chat.voice_output_status`
3. After `session.accepted`, store `advertised_capabilities` as the negotiated feature set.
4. After `session.accepted`, send `persona.assets.request` as before.
5. If `chat.sessions` is negotiated, send `chat.sessions.list`, then select the last locally stored `conversation_id` or send `chat.session.create`.

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
3. Play only while the matching request/conversation is still active.
4. Stop clears queued audio, stops active audio, and cancels frontend/native speech.
5. Do not log local file paths or server audio URLs in normal logs.

## Acceptance Criteria

- AgoDesk stores negotiated capabilities after `session.accepted`.
- New Chat creates a `sess-*` conversation and focuses input.
- History lists sessions and loads old messages.
- Every new-client `chat.message` includes `session_id` and `conversation_id`.
- Stop sends `chat.cancel`, updates the UI immediately, and handles `chat.cancelled`.
- Speech-output changes send `chat.voice_output.status` with `speaker_mode`.
- Auto TTS uses AuraGo audio when available and frontend/native TTS otherwise.
- Older AuraGo servers without these capabilities still allow basic chat.
- Server text is sanitized Markdown/plain text only.
