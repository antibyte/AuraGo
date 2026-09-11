# Desktop Notes

Notes shares Autor's compact document header, save status, theme tokens, global Fruity menus and responsive side panels. The note itself flows continuously rather than using paper pages. All controls are localized in the 16 Desktop languages.

## Editing and files

Milkdown/Crepe and Kit are pinned to **7.22.1 (MIT)**. The Vanilla JavaScript host uses their public ProseMirror interfaces. Vendor JavaScript, CSS, fonts and license notices are local; rebuild with `npm run build:notes-vendor`, verify with `node scripts/build-notes-vendor.js --check`. CodeMirror is the existing shared local bundle. No paid components or additional service is required.

Write formatted text, headings, lists, tasks, tables, links, images and fenced code. Mermaid previews use the existing local renderer with strict security and sanitization. Source mode preserves unsupported Markdown, frontmatter and literal code. Notes above 200,000 characters use CodeMirror's virtualized source view; the file size ceiling is 2 MiB and the configured Desktop file limit also applies. This bounds rich layout without silently truncating a file.

Markdown files under `Documents/Notes` remain authoritative. Tags live in frontmatter; other fields are retained. Pins, sorting and the last note use the compatible `notes.meta.json` sidecar. Attachments use relative paths to `.attachments`. Moving a note updates actual Markdown destinations while leaving frontmatter and code examples untouched. HTML export embeds local image attachments; Markdown retains relative links. External links remain external. TXT omits Markdown formatting; print/PDF uses browser printing.

The library searches the same complete corpus used by the agent. Search is paginated and cancellable; no former 500-note index cap applies. It currently scans files directly. Add a derived FTS index only if measured corpus latency requires one.

## Saving and recovery

`/api/desktop/notes` uses existing authentication, CSRF and Desktop permission gates:

- GET reads a complete note and its ETag, or searches with q/folder/tag/offset/limit.
- POST creates a distinct new Markdown note.
- PUT requires `If-Match` or create-only `If-None-Match: *`.
- PATCH moves, trashes or restores a note with its source ETag. Trash paths preserve the original folder beneath `Trash/Notes/<unique id>/`.

Conditional writes and moves share the Desktop mutation lock. Existing user file-manager operations remain available. The app uses the shared OfficeSession queue: 800 ms inactivity, a five-second deadline, one pending request, revision-bound acknowledgement. An older response never marks newer input saved. Failed loads retain the existing note.

IndexedDB keeps the current recovery draft. Recovery is explicit. Close/open/new flush edits; after a failure, leaving is allowed only with an explicitly accepted, successfully stored draft. Conflicts support retry and a create-only copy. App disposal aborts requests and removes editors, listeners and timers. Reload warns while unsaved edits remain.

The right AI panel calls the existing tool-free Office assist route using explicit selection or current paragraph. A proposal never changes text automatically. Apply requires its original note and revision, and is one undoable action.

## Agent rights and isolation

`desktop_notes` exposes **list, search, read and create only**. The service rejects modifications to existing notes, attachments, metadata and Notes ancestors through non-user Desktop writes, moves, copies and deletes. The rule also covers notes created by agents. Native local file mutation paths check the protected roots, including symlink aliases and parent directories.

Local shell/Python/skill processes follow the selected execution policy, in Desktop Chat and every other channel. With `shell_sandbox.enabled: false`, permitted local tools run with the AuraGo process user's permissions even when Notes exist, on all platforms. Explicit `allow_unsafe_fallback` also permits this when the requested sandbox is unavailable. Tool permission gates still apply.

Native Notes/file tools retain their mutation guards independently of the sandbox setting. **Unisolated code can bypass those guards and change Notes through direct OS access.** The existing shell security hint explains this limit. With active Landlock, writable paths must not overlap Notes or their ancestors; enabled but unavailable isolation remains blocked unless unsafe fallback was explicitly allowed. Disabling Desktop does not remove native protection of a configured Notes directory. Kernel Landlock support and active isolation are separate facts.

This is not a host administrator security boundary: pre-existing unrestricted processes, privileged external integrations and direct OS access retain their own permissions. Restart to apply the new process policy. Do not mount protected Notes writable into independently managed execution containers or grant an agent host-admin access if this restriction must be enforced against those channels as well.

## Verification

- `go test ./internal/desktop ./internal/tools ./internal/sandbox`
- `go test ./internal/server -run 'TestDesktopNotes|TestDesktopOffice'`
- `go test ./ui`
- `node scripts/test-writer-session.mjs`
- `node scripts/build-ui-bundles.js --check`
- Chrome: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '^TestDesktopNotesAppBrowser$' -count=1`.
- Actual Desktop theme matrix: additionally set `AURAGO_NOTES_MATRIX=1` and run `TestDesktopAuroraBrowser`.

The browser suite covers rich/source editing, opaque frontmatter, format-aware search, relative link preservation, delayed saves, conflicts/copies, unavailable draft storage, readonly editing, stale/explicit AI proposals and a 420k-character source note. The shell matrix covers Standard/Fruity dark/light, both densities, 1920×1080, 1366×768 and touch 430×932, menus, focus and lifecycle. Build a matching resource set and binary and run `--check-assets` before installation.
