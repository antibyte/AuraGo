# Animated Persona Assets

## Purpose

Reviewed Rive payloads for future live voice UI integration.

## Ownership

- This folder owns twelve `.riv` files and `catalog.json`.
- Authoring sources, original PNGs, builders, demos and visual regression pages
  live in the sibling project `personas/`, with its current collection under
  `personas/personas/` relative to the parent repository directory.

## Local Contracts

- Catalog keys match AuraGo personality IDs, not the authoring folder names.
- Images are embedded in each RIVE 7.0 payload. Do not add duplicate atlases,
  demo audio, ZIP archives, or intermediate revisions to this folder.
- Keep the catalog revision, byte count and SHA-256 synchronized with each file.
- The paired local runtime is `ui/js/vendor/rive/`, pinned to
  `@rive-app/canvas@2.42.0`. Preserve its license and file hashes.
- State machine `VoicePersona` exposes `mode`, `viseme`, `mouthOpen`, `headTilt`.
  Control values and robot speech styles are documented in the catalog.
- These are prepared assets. Existing static avatars remain the active UI until
  a separate integration connects live voice events to the state machine.
- Preserve the original PNG fallback, including `custom` (no animated payload).

## Work Guidance

- Rebuild in the separate authoring project, then copy reviewed payloads here.
- Follow `documentation/animated-personas.md` for paths and runtime usage.

## Verification

- The authoring collection provides `verify_collection.py`, `check.html` and
  `alignment-check.html`; verify new animation revisions there before importing.
- `go list -json ./ui` must include all catalog payloads and local runtime files
  in `EmbedFiles`. `node scripts/build-ui-bundles.js --check` stays read-only.

## Child DOX Index

None.
