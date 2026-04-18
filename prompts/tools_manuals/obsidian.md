# Obsidian Tool

Interact with an Obsidian vault using the Local REST API plugin.

## Operations

### Reading
- `health` — Check API connectivity and version
- `list_files` — List files/folders (optional: `directory`)
- `read_note` — Read a note (required: `path`; optional: `target_type`, `target` for sub-document)
- `document_map` — Get heading/block structure of a note (required: `path`)
- `list_tags` — List all tags with usage counts
- `list_commands` — List all available Obsidian commands

### Searching
- `search` — Full-text search (required: `query`; optional: `context_length`)
- `search_dataview` — Dataview DQL query (required: `query`)

### Writing
- `create_note` — Create a new note (required: `path`, `content`)
- `update_note` — Replace entire note content (required: `path`, `content`)
- `patch_note` — Append/prepend/replace content (required: `path`, `content`; optional: `target_type`, `target`, `patch_op`)
- `delete_note` — Delete a note (required: `path`; needs `allow_destructive`)

### Periodic Notes
- `daily_note` / `periodic_note` — Read or append to periodic notes (optional: `period` = daily|weekly|monthly|quarterly|yearly, `content`)

### Commands
- `execute_command` — Execute an Obsidian command (required: `command_id`)
- `open_in_obsidian` — Get URI to open a note in the Obsidian app (required: `path`)

## Sub-Document Targeting

Use `target_type` and `target` to read or patch specific sections:
- `target_type: "heading"`, `target: "My Heading"` — Target a specific heading
- `target_type: "block"`, `target: "block-id"` — Target a specific block
- `target_type: "frontmatter"`, `target: "field"` — Target frontmatter field

## Patch Operations

The `patch_op` parameter controls how content is applied:
- `append` (default) — Add content after existing
- `prepend` — Add content before existing
- `replace` — Replace target content entirely

## Permission Model

- **Enabled**: All read operations available
- **Read Only**: Write/patch/create/execute operations blocked
- **Allow Destructive**: Required for `delete_note`

## Configuration

Set in config.yaml under `obsidian:` section. API key stored in vault as `obsidian_api_key`.
Requires the Obsidian Local REST API plugin (v3.6.1+).
