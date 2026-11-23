# Code Studio IDE package 2: code intelligence via LSP

Status: design approved by the user on 2026-11-23. Spec under review; no plan and no code yet.

## Context

Code Studio is the desktop IDE app.

- Frontend: `ui/js/desktop/apps/code-studio/*.js`, concatenated into `ui/js/desktop/bundles/code-studio.bundle.js` by `npm run build:ui`. The file list lives in `scripts/build-ui-bundles.js`.
- CSS: `ui/css/code-studio.css`.
- CodeMirror 6 vendor bundle: `ui/js/vendor/codemirror-bundle.esm.js`, built by `npm run build:codemirror` from `scripts/codemirror-entry.js`. The export list also lives in `scripts/build-codemirror.js`. The bundle already exports `autocompletion`, `linter`, `lintGutter` and `setDiagnostics`.
- Backend: `internal/server/code_studio_handlers.go`.
- Terminal: WebSocket `handleTerminal` in `internal/server/code_studio_handlers.go`. The terminal bridge (`serveCodeStudioTerminal`, `runLineTerminal`) is in `internal/server/code_studio_terminal.go`. Non-TTY exec with stdin and Docker stream demux in `internal/server/code_studio_exec_stdin.go`. Terminal sessions end leftover processes by an env nonce (`AURAGO_TERMINAL_SESSION`) through a `/proc` scan, see `dockerContainerTerminalSession.End` in `internal/server/containers_terminal.go`.
- Container `aurago-code-studio`, defined in `internal/desktop/code_container.go`:
  - image `ghcr.io/antibyte/aurago-code-studio:latest`
  - user `developer`, workspace `/workspace`
  - network `none` unless `virtual_desktop.code_studio.allow_network` is set
  - 4096 MB memory, 1024 pids
  - idle shutdown unless held (`HoldTerminal` in `internal/desktop/code_container_lifecycle.go`)
- Image: `deploy/docker/Dockerfile.code-studio` (`node:22-bookworm-slim`, Go 1.27.4, Python 3, gcc, ripgrep, global npm `typescript`, `tsx`, `pnpm`, `yarn`). It is published only by `.github/workflows/docker-publish.yml`, on a `v*` tag push or a manual `workflow_dispatch`.
- Problems panel: `ui/js/desktop/apps/code-studio/problems.js`. One list (`state.problems`, `setProblems`, `addProblems`, `applyDiagnosticsToOpenTabs`) is fed by cheap client checks (sources `syntax` and `json`) and by a container project check.
- i18n: keys `codeStudio.*` in the 16 files `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`.
- Package 1 (editor, Git workbench, project-wide replace) landed on 2026-11-23, commits `40186a819..eaba3ba40`.

This is package 2 of 4. Package 1 is the editor and Git work. Package 3 (run and debug) and package 4 (git remote) come later and are not part of this spec.

## Goal

Give the Code Studio editor code intelligence for Go, TypeScript/JavaScript, Python and C/C++:

- hover (types and docs)
- go to definition
- find references
- rename across files
- live diagnostics while typing
- completion, signature help and quick fixes (code actions)

The language servers run inside the `aurago-code-studio` container. The browser talks to them through a Go WebSocket bridge.

## Decisions the user made (do not re-ask)

1. Image: the agent changes `deploy/docker/Dockerfile.code-studio` and builds and tests the image locally with Docker Desktop under a local tag (for example `aurago-code-studio:lsp-dev`). The user triggers the publish workflow later. Nothing is pushed.
2. Language servers: gopls, typescript-language-server, pyright and clangd. All four.
3. Extras in scope: completion, signature help and quick fixes.
4. Rename and multi-file edits: a preview dialog lists files and change counts, then applies the edits.
   - Open tabs are edited in the buffer. The change is unsaved and undoable.
   - Closed files are written straight to disk through the existing file API.
5. Architecture A: the official `@codemirror/lsp-client` (6.3.x, MIT, by the CodeMirror author) runs in the browser. A thin Go WebSocket-to-stdio pipe runs per language server.
   - Rejected B: a hand-written JS LSP client (too much code).
   - Rejected C: a Go-side LSP client with REST endpoints (heaviest, extra latency).
6. Lean tests: Go bridge tests, one Node unit test, one browser test, and a manual check with the local image. Implementation lands on local `main`. Nothing is pushed.

## 1. Image

File: `deploy/docker/Dockerfile.code-studio`. All versions are pinned in `ARG` lines.

- gopls: `go install golang.org/x/tools/gopls@<pinned>` with the image's Go toolchain, binary in `/usr/local/bin`. The build cache and module cache are removed afterwards.
- typescript-language-server: `npm install -g typescript-language-server@<pinned>`, on top of the global `typescript` that the image already has.
- pyright: `npm install -g pyright@<pinned>`. It provides `pyright-langserver`.
- clangd: the Debian bookworm `clangd` apt package (clangd 14). This is the only option that works for both amd64 and arm64, because upstream clangd releases have no arm64 Linux build.
  - Started with `--background-index=false`, so it never writes `.cache/clangd` into the user's project.
- Expected image growth: about 200 to 250 MB.
- Pinned versions (latest stable on 2026-11-23):
  - gopls `v0.24.0`. It needs Go 1.27, which matches the image.
  - typescript-language-server `6.1.0`. It requires Node >= 22.22.2, so the plan must make sure the `node:22-bookworm-slim` base resolves to at least 22.22.2, or pin a base tag that does.
  - pyright `1.1.414`.
- Old images without the servers keep working. The bridge reports "not installed" and the editor works as before.

## 2. Go bridge

New file: `internal/server/code_studio_lsp.go`.

Route: `GET /api/code-studio/lsp?lang=go|typescript|python|cpp`, a WebSocket. It is registered in `registerCodeStudioRoutes`.

### Gate chain

The same gate chain as `handleTerminal`, in this order:

1. `requirePermission(desktopScopeWrite)`
2. `rejectReadOnly`
3. `admitDockerTerminal`
4. `isAuthenticated`
5. `desktop.SameHostWebSocketOrigin`
6. GET only
7. `codeContainer(ctx, true)`
8. `HoldTerminal`

An unknown `lang` returns HTTP 400 before the upgrade.

### Fixed command table

The page only names a language. It can never choose the command.

| `lang` | command |
| --- | --- |
| `go` | `gopls` (stdio) |
| `typescript` | `typescript-language-server --stdio` |
| `python` | `pyright-langserver --stdio` |
| `cpp` | `clangd --background-index=false` |

Security note: anyone with write scope can already open a shell in the container. Arbitrary LSP messages from the page (for example gopls commands that run `go test`) need no extra trust. The container stays the boundary.

### Process

- Non-TTY docker exec with `AttachStdin`, `AttachStdout` and `AttachStderr`.
- User `developer`, working directory `/workspace`.
- Env: `AURAGO_LSP_SESSION=<nonce>` and `GOTOOLCHAIN=local`.
  - `GOTOOLCHAIN=local` makes gopls never try to download a toolchain while the network is off.

### Framing

- Docker stream: demultiplex the 8-byte headers.
- Server to page: stdout goes through an LSP `Content-Length` reader. Each message body becomes one WebSocket text frame.
- stderr is discarded. Only a short, capped tail is logged at debug level.
- Page to server: each text frame must be valid JSON (`json.Valid`). The server adds a `Content-Length` header and writes the frame to stdin.

### Limits

- Page to server frame: at most the configured max file size plus 64 KiB. A `didOpen` carries a whole file.
- Server to page message: at most 32 MiB.
- LSP header block: at most 8 KiB.
- At most 8 LSP sessions server-wide.
- Ping and pong as in the terminal.

### Close codes

- 4404: language server not installed. Checked with `command -v <binary>` right after the upgrade.
- 4429: too many sessions.
- 4500: start failed.
- 1011: the server process exited.

### Cleanup

On socket close:

1. Close the process stdin.
2. Wait up to 2 s for the process to exit.
3. End leftover processes with the nonce through a `/proc` scan, the same pattern as the terminal `End`. Send TERM, then KILL.

### Writes

Rename and quick fixes never write through the bridge. They use the existing file API, `PUT /api/code-studio/file`.

## 3. Browser client and UI

### Vendor bundle

- Add `@codemirror/lsp-client` `6.3.1` to `package.json`. It depends on `marked` `^15` (not the newer marked 18).
- Export the needed pieces from `scripts/codemirror-entry.js` and `scripts/build-codemirror.js`: `LSPClient`, `Workspace`, `hoverTooltips`, `serverCompletion`, `signatureHelp`, `jumpToDefinition` and the others the modules use.
- Rebuild with `npm run build:codemirror`.
- No Internet assets.
- Hover HTML goes through an allowlist sanitizer, passed to the client as the `sanitizeHTML` option: an existing desktop helper if one fits, otherwise a small one in `lsp.js`.

### New modules

Both are added to the code-studio bundle list in `scripts/build-ui-bundles.js`.

- `ui/js/desktop/apps/code-studio/lsp.js`: connection manager, WebSocket transport, the Code Studio `Workspace` subclass, and diagnostics.
- `ui/js/desktop/apps/code-studio/lsp-actions.js`: references, rename, quick fixes, the preview dialog and the WorkspaceEdit applier.

### Connections

- One `LSPClient` per language per Code Studio window.
- A client starts lazily, when the first file of its language opens.
- Language mapping:
  - `.go` to go
  - `.ts .tsx .js .jsx .mjs .cjs` to typescript
  - `.py` to python
  - `.c .cc .cpp .cxx .h .hpp` to cpp
- URIs are the container paths (`file:///workspace/...`). There is no path translation.
- Root: `rootUri` and the single workspace folder are `file:///workspace`. gopls finds nested modules on its own. typescript-language-server finds the nearest `tsconfig.json` per file. clangd walks up to the nearest `compile_commands.json`. pyright reads a `pyrightconfig.json` or `pyproject.toml` at `/workspace` only, so subprojects get defaults.
- Idle release: after 10 minutes without edits or requests in that language, the browser closes that language's socket. This releases the container hold, so idle shutdown still works. The next keystroke or request reconnects.

### Workspace subclass

- Maps lsp-client files to Code Studio tabs.
- Split views of one file share one LSP document.
- `displayFile` handles cross-file jumps. It opens the tab, or activates it if it is already open.

### Features and keys

- Hover tooltip (types and docs).
- Completion items appear in the existing autocomplete popup.
- Signature help on `(` and `,`, and on Ctrl+Shift+Space.
- Go to definition: F12, and Ctrl+Click (Cmd+Click on macOS) when the file's server is ready. While a server is ready, extra cursors move to Alt+Click, as in VS Code. Without a ready server, Ctrl+Click keeps adding cursors as today.
- Find references: Shift+F12. Results go to a new **References** tab in the bottom panel, grouped by file with line previews. Clicking a result opens the location. This replaces the library's own reference panel.
- Rename: F2 in the editor. F2 in the file tree keeps renaming files.
  - A small input collects the new name, then `textDocument/rename` runs.
  - Then the preview dialog shows the changes. Then the edits are applied.
- Quick fixes:
  - A lightbulb in the gutter on lines that have LSP diagnostics.
  - Ctrl+. opens a menu of code actions (`textDocument/codeAction` with the line's diagnostics).
  - A quick fix whose edit stays in the current file applies directly, as one undoable step.
  - A quick fix that touches other files goes through the preview dialog.
  - Code action commands go to `workspace/executeCommand`.
  - A server-sent `workspace/applyEdit` uses the same applier.

### Diagnostics

- A handler for `textDocument/publishDiagnostics` converts diagnostics to Code Studio problems. The source is `gopls`, `typescript`, `pyright` or `clangd`.
- `replaceProblemsForFile(file, source, problems)` is a new function in `problems.js`. It replaces that file's problems for that one source. The Problems panel, squiggles and status counters stay one pipeline.
- When a language's server is ready, the client-side `syntax` check is skipped for that language, so there are no duplicates.
- After Code Studio's own writes, the client sends `didSave` and `workspace/didChangeWatchedFiles`. Writes are: save, rename apply, replace, and git discard or switch.

### Status bar

- One item for the active file's server: starting, ready, not installed, or error.
- Clicking the item restarts that server.

### Settings

- One switch in the settings dialog: "Code intelligence (LSP)". Default on.
- Stored in the existing localStorage key `aurago.codeStudio.settings.v1`.

### Read-only desktop

LSP stays off. The bridge rejects the connection, with the same rules as the terminal.

### Strings

New strings go into all 16 `ui/lang/desktop/*.json` files, under `codeStudio.*`.

## Error handling

- 4404: status "Language server not installed — rebuild the Code Studio image". No reconnect.
- 4429: status "Too many language servers". Retry when the tab is focused again.
- Any other close (server crash, container stopped or idle):
  - status "Disconnected"
  - reconnect with backoff of 1, 2, 5 and 10 s (5 tries)
  - after the 5 tries, status "Error — click to restart"
  - after a reconnect, open documents are sent again with `didOpen`, and the stale LSP problems of that language are cleared
- Request timeouts:
  - 3 s for hover and completion
  - 15 s for references and rename
  - on timeout, a short notice. The editor stays usable.
- Partial write failures while applying an edit: the preview dialog lists the failed files. The other files stay applied.

## Testing (lean)

- Go: `internal/server/code_studio_lsp_test.go`, with a fake exec stream. Covers:
  - framing in both directions (split headers, several messages in one chunk)
  - size limits
  - invalid JSON
  - unknown `lang` returns 400
  - read-only returns 403
  - missing binary returns 4404
  - session cap returns 4429
  - cleanup on close
- Node unit test: a `*.test.cjs` file next to the module, like `ui/js/desktop/apps/notes-index.test.cjs`. Covers:
  - WorkspaceEdit to per-file change list
  - publishDiagnostics to problems
- One browser test in the `ui` package, in the existing Code Studio browser harness (for example `ui/desktop_code_studio_browser_test.go`). It uses a scripted fake LSP WebSocket. The test:
  - opens a `.go` file
  - checks that a diagnostic appears in the Problems panel
  - checks that the hover tooltip shows
  - checks that F12 jumps to the definition
- Manual: with the locally built image, each of the four servers answers `initialize` and a hover on a small sample project through the real bridge.

## Delivery and plans

Plans go in `docs/superpowers/plans/`. That folder is git-ignored, so the plan files are force-added.

- `2026-11-23-code-studio-pkg2a-image-bridge.md`: image and Go bridge.
- `2026-11-23-code-studio-pkg2b-client-core.md`: vendor bundle, `lsp.js`, diagnostics, hover, completion, signature help, definition, status bar and setting.
- `2026-11-23-code-studio-pkg2c-actions.md`: references, rename, quick fixes, preview and applier, locales.

Order: 2a and 2b can run in parallel. 2c needs 2b.

Commit rules:

- Each task is committed to local `main` through a temporary index. Other sessions share the checkout.
- Nothing is pushed. The user publishes the image.

## Out of scope

- Rust (rust-analyzer), Java and PHP servers.
- LSP formatting. The existing Format Document stays as it is.
- Workspace symbol search.
- Inlay hints.
- Semantic tokens.
- Call hierarchy.
- Sharing one language server between several windows.
- Exposing LSP to the AuraGo agent.
- A server-side config switch.
