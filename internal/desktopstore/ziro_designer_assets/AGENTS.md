# Ziro Designer image

## Purpose

Build the optional offline Ziro Designer Store image and its document bridge.

## Ownership

This directory owns the pinned build, static server and GPL browser adapter.
The parent Store owns lifecycle/publication. Desktop `ziro-files.js` and
`ziro-designer.js` own Shared access, UI, conflicts and window lifetime.

## Local Contracts

- Pin upstream revision `7b4597005e4801d4fb19661fcac6fc3380547cf2` and image tag
  `ziro-7b45970-1` together with the catalog and Docker publication workflow.
- `adapt.mjs` fails when a pinned source seam changes. Do not loosen it without
  inspecting the new upstream. Preserve all editor flush callbacks; explicit
  Shared snapshots must propagate serialization failures and include binary and
  newly generated files. Open-file state alone can be stale.
- The image serves static assets as `node`, port 8080, without writable project
  mounts, secrets or Docker access. Cloud, AI, telemetry and external library
  fallbacks stay disabled; enforce self-only runtime connections through CSP.
- Shared mutations travel only through the parent Desktop API. The child accepts
  a MessagePort only from the declared parent origin/window and document challenge.
  Navigation/close must revoke it. Never expose a general filesystem bridge.
- Each draft ID includes canonical Shared path and stable desktop window key;
  equal project names and two windows must not share a draft record.
- Embedded routing and global last-session restore stay disabled: an async
  upstream restore can otherwise replace the host's already opened project.
- Embedded live-sync channels use that same window-specific identity. Never
  broadcast by KiCad basename: different Shared projects can have identical
  filenames, and separate windows must retain explicit ETag conflict handling.
- Keep background editor warmup disabled in embedded mode: its placeholder
  projects can replace the active project's native library tables.
- Reload project library tables after mounting the schematic's memory filesystem;
  reload symbol tables again at index lookup so a later native project reload
  cannot leave the chooser with stale tables.
- The bridge keeps a complete memory mount below the editors' temporary mounts;
  remounts must not make library files disappear. Mirror changes byte-for-byte
  and revoke its listener/mount on project replacement or disposal. Native reads
  inside the selected project's root use the bridge's current byte cache, so a
  partial temporary mount cannot hide a custom library. Other paths use wx.
- Embedded frames receive fresh props and remount at project boundaries; clear
  obsolete flush callbacks before opening. Snapshot drains React updates and
  flushes schematic, PCB and project-owned symbol/footprint library edits.
  An editor with remaining unsaved work must fail the snapshot, never report
  Shared success. Unbound libraries require the editor's Save As first.
- Prefer exact relative file paths when applying editor changes; hierarchical
  sheets in different subfolders may share a basename. The pinned loader and
  hierarchy walker are adapted to retain those paths and resolve child references
  relative to their parent sheet, without escaping the project.
- Ship GPL/NOTICE and the corresponding adapted source archive in the image.

## Work Guidance

Keep the adapter small and inspectable. The upstream UI remains English.
Project folders and file deletions belong to the File Manager; Shared Save is
explicit. Do not activate an external service to fill missing offline libraries.

## Verification

- `node scripts/test-ziro.mjs` and `node scripts/test-ziro-bridge.mjs` at repo root.
- Build the Docker image, verify UID 1000 and `/health`, then run the real-browser
  acceptance documented in `documentation/ziro-designer.md`.
- Typecheck and production build run inside Docker. Verify both architectures
  separately from GHCR publication and installed-app acceptance.

## Child DOX Index

None.
