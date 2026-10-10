# Layerling — an open Desktop CAD editor

Use `layerling` for interactive 3D CAD in the user's open browser editor.
This tool needs an authenticated Desktop chat context, an enabled Layerling
integration, Desktop agent access, and `agent_access: read` or `write`.
It cannot start a headless editor or work after the browser disconnects.

1. Call `list_editors`. Match the exact `editor_id` supplied by the window chat
   context, or ask the user to select an editor when several are eligible.
   Never choose the most recently active window. IDs expire with connections.
2. Call `describe` with `arguments: "{\"operation\":\"create_shape\"}"` for the
   operation's pinned Layerling 1.57.0 parameter schema. Pass these parameters as
   the JSON string `arguments`, and the editor ID separately as `editor_id`.
3. Inspect with `get_scene` or `list_objects` before editing. Use real object IDs
   from that editor. Examples of editing operations are `create_shape`,
   `update_object`, `boolean_cut`, and `apply_edge_treatment`.
4. Inspect again and, when requested, save or export through Desktop paths.

Read access permits scene inspection, edge/error/reference/custom-shape lists,
print estimates and `capture_image`. Images are returned as bounded, expiring
artifact references, not base64 in the conversation. All other operations
require write access and a writable Desktop. Commands are sequential per editor.
An uncertain timeout or disconnection must never be retried automatically:
reconnect, inspect the scene and reconcile the user's intent first.

Desktop operations:

- `open_project`: `{"path":"Documents/Layerling/part.lyl"}`. Refuses unsaved changes.
- `save_project`: the same path shape. Saves the open project using its observed
  ETag; a different target is create-only. A conflict requires user resolution.
- `import_model`: `{"path":"Documents/Layerling/model.stl"}`; accepts STL, OBJ,
  3MF, STEP/STP and SVG through the existing importer.
- `export_model`: `{"path":"Documents/Layerling/part.step","format":"step"}`.
  Formats: `stl`, `obj`, `3mf`, `step`, `png`. The destination is create-only.

Do not place mesh/image payloads in messages. Use Desktop paths and file
operations for larger data. Arguments are bounded to 64 KiB and inline results
to 32 KiB. Request a narrower inspection if a result is too large. The upstream
HTTP MCP bridge, arbitrary remote URLs, direct printer control and automatic
skill installation are not part of this integration.
