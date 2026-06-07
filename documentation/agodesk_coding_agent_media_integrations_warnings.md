# AgoDesk Coding Agent: Media, Integrations, And Warnings

Implement the AgoDesk client work for AuraGo chat media artifacts, integration webhosts, and system warnings. This instruction assumes Stop, New Chat, History, and TTS from `agodesk_coding_agent_chat_controls.md` already exist.

## Protocol Setup

1. Keep tolerant parsing for unknown WebSocket message types and fields.
2. Add client capabilities:
   - `chat.media_events`
   - `integrations.webhosts`
   - `system.warnings`
3. After `session.accepted`, store `advertised_capabilities` as the negotiated feature set.
4. If `integrations.webhosts` is negotiated, send `integrations.webhosts.list` with the accepted `session_id`.
5. If `system.warnings` is negotiated, send `system.warnings.list` with the accepted `session_id`.
6. Render `chat.media` only when `chat.media_events` is negotiated.

## Local State

Extend the existing chat state with:

```ts
type AgoDeskMediaState = {
  mediaByConversation: Map<string, ChatMediaItem[]>;
  integrationWebhosts: WebhostIntegration[];
  systemWarnings: SystemWarning[];
  warningTotal: number;
  warningUnacknowledged: number;
};
```

Persist only UI preferences. Do not persist server media paths, warning payloads, shared keys, tokens, or local file paths in logs or debug dumps.

## Media Events

Handle `chat.media` as non-TTS chat artifacts. `chat.audio` remains AuraGo TTS only; tool audio and generated music arrive as `chat.media` with `kind:"audio"`.

Supported `kind` values:

- `image`: render inline and offer an open-folder/file action.
- `audio`: play in a local audio player or queue; use title/filename metadata and offer an open-folder/file action.
- `document`: render `preview_url` inline when possible; otherwise show a file action.
- `video` and `live_stream`: embed a player when WebView support allows it.
- `youtube_video`: prefer `embed_url`; if WebView or CSP blocks embedding, open `url` externally.
- `stl`: render inline only if the client has a viewer; otherwise show a file action.
- `link`: open externally or in AgoDesk's integration/WebView surface.

Asset rules:

1. Resolve relative `path` and `preview_url` values against the AuraGo origin.
2. Use `/api/agodesk/media/...` paths exactly as provided, including `agodesk_exp`, `agodesk_sig`, and any preview parameters.
3. Treat these paths as short-lived signed URLs. Do not persist them beyond the active chat/history render cache; on HTTP `401`, reload the conversation or request fresh media metadata.
4. Do not rewrite these paths back to `/files/...`; `/files/...` requires a Web UI login cookie.
5. Stop should stop active audio/video playback for the current request.
6. Render titles, captions, filenames, and descriptions as text or sanitized Markdown only.

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

- AgoDesk advertises and stores `chat.media_events`, `integrations.webhosts`, and `system.warnings` when supported.
- `chat.media` renders images, documents, audio/music, video, STL, links, and YouTube without requiring Web UI cookies.
- `chat.audio` still handles only AuraGo TTS.
- Integrations show the same webhost list as the Web Chat integrations drawer.
- System warnings show the same warning list/counts as Web Chat and can acknowledge one or all warnings.
- Older AuraGo servers without these capabilities still allow existing chat controls.
- Server-provided text is sanitized Markdown/plain text only.
