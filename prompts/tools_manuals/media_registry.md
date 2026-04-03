# Media Registry Tool

Track all generated media (images, TTS audio, transcriptions) with metadata, tags, and descriptions.
Media is automatically registered when generated via `generate_image` or `tts`. Use this tool to search, describe, tag, and manage the registry.

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
{"action": "media_registry", "operation": "search", "query": "tts greeting", "media_type": "tts"}
```

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
- `tts` — Text-to-speech audio (auto-registered from `tts`)
- `audio` — Other audio files
- `music` — Music files
- `document` — Documents and PDFs (auto-registered from `document_creator`)

## Notes
- Items are auto-registered when created via `generate_image`, `tts`, or `document_creator`
- Use `update` to add descriptions and better tags after auto-registration
- `delete` is a soft-delete; items are hidden but not removed from the DB
- Tags are stored as JSON arrays; use `tag` operation with modes `add`, `remove`, or `set`

## ⚠️ MANDATORY REGISTRATION RULE
**You MUST NEVER save or copy files to `data/documents/`, `data/images/`, or any media directory without registering them in the media registry immediately after.**

The Media View in the Web UI only shows items that are registered. Files placed on disk without registration are invisible to the user and create orphaned clutter.

**Correct workflow for any document/file you create:**
1. Use `document_creator` → auto-registers on success ✅
2. Use `send_document` tool (not filesystem!) → auto-registers the file ✅
3. Use `filesystem` to write a file to a media directory? → immediately call `media_registry` `register` ✅

**Never do this:** Create a file with shell/Python/filesystem and skip registration → 📁 file is invisible in the UI. Or try calling `{"action": "filesystem", "operation": "send_document"}` which does not exist!
