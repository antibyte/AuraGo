# Media Registry Tool

Track durable generated or uploaded media (images, videos, audio, music, documents) with metadata, tags, and descriptions.
Media is automatically registered when generated via durable media tools such as `generate_image`, `generate_video`, `send_video`, and `document_creator`. TTS audio is ephemeral cache output and is not kept as durable registry media.

## Prerequisites
- `media_registry.enabled: true` in config.yaml
- DB is auto-created at `sqlite.media_registry_path`

## Operations

### register — Manually register a media item
```json
{
  "action": "media_registry",
  "operation": "register",
  "media_type": "image",
  "filename": "sunset_photo.png",
  "file_path": "/workspace/images/sunset_photo.png",
  "description": "Sunset over the mountains, warm orange tones",
  "tags": ["sunset", "landscape", "mountains"]
}
```

### search — Search media by query (matches description, prompt, tags, filename)
```json
{"action": "media_registry", "operation": "search", "query": "sunset", "limit": 10}
{"action": "media_registry", "operation": "search", "query": "intro music", "media_type": "music"}
```
For `media_type: "image"`, results include both MediaRegistry entries and legacy ImageGallery entries. The `source_db` field tells whether the item came from `media_registry` or `image_gallery`.

For homepage projects, do not paste `web_path` directly into deployable source unless the page is only meant to run inside AuraGo. Copy or regenerate the image into the homepage project's `public/assets/...` directory, then reference the deployable project asset with a URL such as `/assets/hero.jpeg`. This keeps local previews, Netlify, and Vercel deployments from depending on AuraGo-only media routes.

### get — Get a single media item by ID
```json
{"action": "media_registry", "operation": "get", "id": 42}
```

### list — List media items with optional filters
```json
{"action": "media_registry", "operation": "list", "limit": 20}
{"action": "media_registry", "operation": "list", "media_type": "image", "limit": 10, "offset": 0}
```

### update — Update description or metadata of a media item
```json
{
  "action": "media_registry",
  "operation": "update",
  "id": 42,
  "description": "Updated description of the sunset image",
  "tags": ["sunset", "landscape", "golden-hour"]
}
```

### tag — Add, remove, or replace tags
```json
{"action": "media_registry", "operation": "tag", "id": 42, "tag_mode": "add", "tags": ["favorite"]}
{"action": "media_registry", "operation": "tag", "id": 42, "tag_mode": "remove", "tags": ["draft"]}
{"action": "media_registry", "operation": "tag", "id": 42, "tag_mode": "set", "tags": ["final", "published"]}
```

### delete — Soft-delete a media item
```json
{"action": "media_registry", "operation": "delete", "id": 42}
```

### stats — Get registry statistics
```json
{"action": "media_registry", "operation": "stats"}
```

## Media Types
- `image` — Generated images (auto-registered from `generate_image`)
- `audio` — Durable audio files
- `music` — Durable music files
- `video` — Videos (auto-registered from `generate_video` and `send_video`)
- `document` — Documents and PDFs (auto-registered from `document_creator`)

## Notes
- Items are auto-registered when created via `generate_image`, `generate_video`, `send_video`, or `document_creator`
- TTS output remains ephemeral and is cleaned up by the TTS cache lifecycle. Use `audio` or `music` for durable audio that should stay visible in the registry.
- Image searches are unified with the older ImageGallery database, so images visible in the Media View can also be discovered by the agent.
- Use `update` to add descriptions and better tags after auto-registration
- `delete` is a soft-delete; items are hidden but not removed from the DB
- Tags are stored as JSON arrays; use `tag` operation with modes `add`, `remove`, or `set`

## ⚠️ MANDATORY REGISTRATION RULE
**You MUST NEVER save or copy files to `data/documents/`, `data/images/`, or any media directory without registering them in the media registry immediately after.**

The Media View in the Web UI only shows items that are registered. Files placed on disk without registration are invisible to the user and create orphaned clutter.

**Correct workflow for any document/file you create:**
1. Use `document_creator` → auto-registers on success ✅
2. Use `send_document` tool (not filesystem!) → auto-registers the file ✅
3. Use `send_video` for video files that should appear in chat and media view ✅
4. Use `filesystem` to write a file to a media directory? → immediately call `media_registry` `register` ✅

**Never do this:** Create a file with shell/Python/filesystem and skip registration → 📁 file is invisible in the UI. Or try calling `{"action": "filesystem", "operation": "send_document"}` which does not exist!
