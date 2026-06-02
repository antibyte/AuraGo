# AgoDesk Coding Agent: Persona Prompt Implementation

AuraGo now includes the active persona prompt in the existing `persona.assets` WebSocket response. Implement this in AgoDesk by extending the current persona asset handling, not by adding a second request.

## Contract

After `session.accepted`, AgoDesk already sends:

```json
{
  "type": "persona.assets.request",
  "payload": {
    "session_id": "agodesk:dev:abc123"
  }
}
```

AuraGo responds with:

```json
{
  "type": "persona.assets",
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

`persona_prompt` is the markdown body of the active AuraGo persona file. AuraGo removes YAML front matter before sending it. The field is always present in new AuraGo versions, but AgoDesk should still tolerate it being absent or empty when talking to older servers.

## Required AgoDesk Changes

1. Extend the local `persona.assets` payload model with `persona_prompt` as an optional string.
2. When handling `persona.assets`, cache `persona_prompt` together with `persona`, `icon_key`, `avatar_image_url`, `icon_url`, and `asset_version`.
3. Resolve only the image URLs against the AuraGo origin. Do not treat `persona_prompt` as a URL.
4. Apply `persona_prompt` to AgoDesk's local persona/agent instruction layer wherever AgoDesk previously used a hard-coded persona prompt.
5. Refresh the cached prompt whenever AgoDesk reconnects, observes a persona change, or receives a different `asset_version`.
6. If `persona_prompt` is absent or empty, keep AgoDesk functional by falling back to its neutral local default, but re-request after the next accepted session.
7. Render the prompt only as plain text or sanitized markdown in debugging/settings UIs. Never inject it as raw HTML.

## Suggested Data Shape

```ts
type PersonaAssetsPayload = {
  session_id: string;
  persona: string;
  icon_key: string;
  avatar_image_url: string;
  icon_url: string;
  persona_prompt?: string;
  asset_version: string;
};

type CachedPersonaAssets = {
  sessionId: string;
  persona: string;
  iconKey: string;
  avatarImageUrl: string;
  iconUrl: string;
  personaPrompt: string;
  assetVersion: string;
  receivedAt: string;
};
```

## Acceptance Criteria

- AgoDesk requests `persona.assets` after every accepted session.
- The cached persona prompt changes when AuraGo's active persona changes.
- Chat/avatar UI still uses `icon_url` and `avatar_image_url` exactly as before.
- Local AgoDesk agent behavior uses the received `persona_prompt` instead of a hard-coded prompt.
- Older AuraGo servers without `persona_prompt` do not crash the client.
