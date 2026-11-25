# Code Studio: SSH workspaces + HTML preview — design

Date: 2026-11-25. Branch `feat/code-studio-ssh-preview` (worktree `.claude/worktrees/cs-ssh-preview`).

User request (German): Code Studio should be able to open a folder on an SSH host as its workspace, and offer an
HTML preview inside the window or in a new browser tab, "with perfect UI and UX".

Decisions taken with the user:
- Credentials: pick an SSH device from the inventory (Quick Connect devices, secrets in the vault, known_hosts);
  a short "New SSH host" form inside Code Studio creates a new inventory device when it is missing.
- The Code Studio agent (AI chat) works fully remote in an SSH workspace (read, write, patch, run commands).
- LSP in an SSH workspace runs the language servers on the remote host over SSH when they are installed there;
  otherwise the status bar says clearly which server is missing.

## 1. Today

Every Code Studio operation is a command run in the `aurago-code-studio` container through
`codeStudioDockerAPI` (`Exec`, `PutArchive`) with the workspace at `/workspace`
(`internal/server/code_studio_handlers.go`, `code_studio_files.go`, `code_studio_paths.go`, `code_studio_terminal.go`,
`code_studio_git_workbench.go`, `code_studio_exec_stdin.go`, `code_studio_lsp.go`). Paths from the browser are
sanitised to `/workspace/...` by `sanitizeCodeStudioPath`. The UI (`ui/js/desktop/apps/code-studio/*.js`,
`ui/js/desktop/apps/code-studio.js`) talks to `/api/code-studio/*` and knows almost nothing about `/workspace`
except LSP URIs.

## 2. Workspace model

- A workspace is either `local` (the container, unchanged behaviour, default) or a saved SSH workspace.
- Saved SSH workspaces are persisted server-side (desktop data store; plan decides the table/file):
  `{id (random, url-safe), device_id, root (absolute remote dir), label, created_at, last_used_at}`.
- **Virtual paths stay `/workspace/...` for every workspace.** The browser, sanitising, git pathspecs, search
  results and LSP URIs keep using `/workspace/<rel>`; only the SSH backend maps `/workspace/<rel>` to
  `<root>/<rel>` and back. This keeps UI churn minimal and every existing path check valid.
- Each HTTP request names its workspace with header `X-Code-Studio-Workspace: <id>`; WebSockets (terminal, LSP) and
  plain links use query `ws=<id>`. Missing/`local` = local. Unknown id = 404 `workspace_not_found`.

### API
- `GET /api/code-studio/workspaces` → local + saved SSH workspaces (device name, host, user, root, label,
  last_used_at, capabilities if cached).
- `POST /api/code-studio/workspaces` `{device_id, root, label?, trust_host_key?}` → connects, checks that `root`
  is a directory, probes capabilities, saves (or updates the existing entry for the same device+root) and returns
  the workspace. Unknown host key → `409 {code:"unknown_host_key", host, key_type, fingerprint}`; the UI asks
  and repeats with `trust_host_key=<fingerprint>` (written to known_hosts only if the fingerprint still matches).
  Changed host key → `409 {code:"host_key_mismatch"}`, no override from Code Studio.
- `DELETE /api/code-studio/workspaces/{id}` → forget (closes pooled connection).
- `POST /api/code-studio/workspaces/{id}/connect` → (re)connect + refresh capabilities; same 409 contract.
- `GET /api/code-studio/remote/browse?device_id=&path=` → folder picker listing over SFTP (dirs first, files
  shown dimmed, `is_git` flag for dirs containing `.git`), default path = remote home (`sftp.Getwd`). Same 409
  host-key contract.
- `GET /api/code-studio/remote/devices` → inventory devices usable for SSH (protocol ssh or empty), no secrets.
- `POST /api/code-studio/remote/devices` `{name, host, port, username, auth:"password"|"key", secret, passphrase?}`
  → creates the inventory device with the secret stored in the vault (reuse the inventory/vault helpers used by
  `/api/devices`; do not edit `device_handlers.go`, another session is changing it).

### Capabilities
Probed at connect (one SSH exec): `uname -sm`, `$SHELL`, and `command -v` for git, grep, gofmt, gopls,
pyright-langserver, typescript-language-server, clangd, python3. Returned as
`{os, arch, shell, git, grep, formatters:{...}, lsp:{go,python,typescript,cpp: bool}}`. The UI gates features
from it (Git view empty state "Git is not installed on <host>", LSP status item).

## 3. Backend abstraction

New interface (file `code_studio_backend.go`), chosen per request by `h.backendFor(r)`:

```
List(ctx, vdir) ([]codeStudioFileEntry, error)
Probe(ctx, vpath) (codeStudioFileProbe, error)
Read(ctx, vpath, max int64) ([]byte, error)
Write(ctx, vpath string, content []byte, createOnly bool) error   // errCodeStudioExists → 409
Move(ctx, from, to string) error
Delete(ctx, vpath string) error
Mkdir(ctx, vpath string) error
Search(ctx, opts codeStudioSearchOptions) ([]codeStudioSearchResult, bool, error)
Exec(ctx, argv []string, vcwd string, stdin []byte, timeout time.Duration) (codeStudioExecResult, error)
Git(ctx, timeout, args ...string) (codeStudioExecResult, error)   // git -C <root> --literal-pathspecs ...
Terminal / LSP: attach functions returning a stdio+resize handle for the WS bridges
Info() codeStudioWorkspaceInfo                                     // id, kind, label, root, capabilities
```

- `localBackend` wraps today's container code unchanged (same scripts, same `codeStudioDockerAPI`, so the
  existing fakes and tests keep working). Handlers are refactored to call the backend instead of `h.docker`.
- `sshBackend`: pooled `*ssh.Client` per device (keepalive 30 s, idle close 10 min, transparent redial once on a
  broken connection), one SFTP client per pooled connection.
  - Files over SFTP. Write = temp sibling `.aurago-save-<rand>` + `posix-rename@openssh.com` (fallback remove +
    rename), mode of the existing file kept, createOnly via `O_EXCL`. Move creates the destination parent.
    Delete recursive. Lexical confinement to `root` plus `RealPath` check for symlinks; this is a UX guard, not a
    security boundary (the same user has a full shell in the terminal) — document that in code.
  - Search: SSH exec of `grep -rnIZ` (with `-F/-E`, `-i`, `-w`, `--include`, `--exclude`, `--exclude-dir`,
    `-m`) under the mapped dir, output parsed and mapped back to virtual paths, same 200-result cap and 16 MiB
    output cap. If grep is missing: clear error.
  - Exec/Git: `cd <mapped cwd> && exec <shell-quoted argv>` over an SSH session, stdin supported, 4 MiB output
    cap, timeout closes the session (signal KILL first).
  - Terminal: PTY session, `cd <root> && exec "${SHELL:-/bin/sh}" -l`, resize via `WindowChange`. Same WS
    message protocol as the local terminal.
  - LSP: language-server command run over SSH stdio when the capability is present; the bridge rewrites
    `file:///workspace` ↔ `file://<root>` in both directions (string-value aware, not blind byte replace).
- Read-only desktop, permission scopes and size limits apply exactly as for local.
- Credentials never reach the browser; host keys use the existing known_hosts flow (`resolveSSHAccess`,
  `buildSSHConfig` in `internal/desktop/ssh_proxy.go` — export small helpers if needed, do not touch the
  uncommitted Quick Connect files of the other session: `internal/desktop/remote_files*.go`, `sftp_handlers.go`,
  `device_handlers.go`, `device_quick_connect.go`, `server_routes.go`, `inventory.go`).

## 4. HTML preview

### Serving
- `POST /api/code-studio/preview` `{path}` (+ workspace header) → `{url, token, expires_at}` where
  `url = /code-studio-preview/<token>/<rel path>`.
- Token = HMAC-signed, stateless (pattern of the desktop embed tokens in `auth.go`): workspace id, user/session
  binding if available, expiry 12 h. The `/code-studio-preview/` prefix is exempt from cookie auth (cookies are
  `SameSite=Strict` and the sandboxed page has an opaque origin, so the token is the capability). Read-only,
  confined to the workspace.
- `GET /code-studio-preview/<token>/<rel>` reads through the workspace backend; directory → `index.html`;
  MIME by extension; size cap 25 MiB; styled 404. Headers: `Content-Security-Policy: sandbox allow-scripts
  allow-forms allow-modals allow-popups allow-downloads` (no `allow-same-origin`: the page can never read
  AuraGo cookies, storage or API responses), `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`,
  `Cache-Control: no-store`.
- Live reload: for `text/html` responses the server injects
  `<script src="/code-studio-preview/<token>/__aurago/live.js"></script>` before `</body>` (or at the end).
  `live.js` long-polls `__aurago/version?since=N` (25 s) and reloads; when every changed path is `.css` it
  swaps the stylesheet `href`s with a cache buster instead of reloading. It also forwards `console.error`,
  `window.onerror` and unhandled rejections to `parent` via `postMessage` (shape `{auragoPreview:1, kind, text}`).
- Version counter per workspace, bumped (with the changed virtual paths) by every Code Studio write, move,
  delete, upload, LSP edit and agent write.

### UI
- Entry points: eye button in the editor title area for `.html/.htm/.svg`; Explorer context menu "Open Preview"
  and "Open in Browser Tab"; command palette "Preview: Open to the Side", "Preview: Open in Browser Tab";
  shortcut Ctrl+Shift+V toggles the side preview for the active HTML file.
- Side preview pane right of the editor group, resizable splitter (width persisted), toolbar:
  reload, address field with the workspace-relative path (editable, Enter navigates inside the workspace),
  viewport switch (Responsive / Tablet 768 / Mobile 375 with centred device frame), console badge with error
  count (click shows the list, clear button), "Open in new tab", close. Follow mode: preview follows the active
  HTML tab (pin button to keep a file). Iframe uses the same sandbox flags as the CSP. Saving reloads (live.js);
  unsaved edits are not previewed (status hint "Showing saved version" while the tab is dirty).
- New tab: `window.open(url, '_blank', 'noopener')`; it keeps live reload.
- Theme-aware styling with the Code Studio tokens, keyboard reachable, focus rings, aria labels, empty states
  ("Open an HTML file to preview it"), loading shimmer, error card when the file is gone.

## 5. Workspace UI/UX

- **Workspace switcher** at the top of the Explorer: icon (laptop = local, server = SSH) + label
  (`label` or `device · ~/path`), chevron. Menu: Local workspace; recent SSH workspaces (status dot
  connected/disconnected, host, path, remove ×); "Connect to SSH folder…"; when remote: "Reconnect", "Disconnect".
- **Status bar remote chip** at the far left (accent background when remote): "SSH: <device>"; click opens the
  switcher menu. Connecting → spinner; lost → warning colour.
- **Connect dialog** (one modal, two panes): left = host list (search box, inventory SSH devices with name,
  user@host:port, last used), "+ New SSH host" opens an inline form (name, host, port 22, user, password or
  private key paste/file, passphrase) with validation; right = folder browser for the selected host: breadcrumb +
  editable path field, Home/Up buttons, folder list (git badge), "New folder", recent paths for that host.
  Footer: chosen path preview, "Open folder" primary button (Enter). Host-key trust appears as an inline card
  (host, key type, SHA256 fingerprint, "Trust and continue" / "Cancel"). Errors inline (auth failed, unreachable,
  not a directory) with retry.
- **Switching**: open tabs are kept per workspace (localStorage `aurago.codeStudio.workspaceSessions.v1`), the
  active workspace per browser in `aurago.codeStudio.workspace.v1`; switching with dirty tabs asks
  Save all / Discard / Cancel. Explorer, search, git, problems, terminals and LSP restart for the new workspace.
  Window title and terminal tab titles show `user@host`.
- **Disconnect handling**: banner "Connection to <host> lost" with Reconnect, auto-retry with backoff; saves while
  offline fail with a toast and keep the tab dirty; nothing is silently dropped.
- Locales: all new strings in every `ui/lang/desktop/*.json` (16 languages).

## 6. Agent in an SSH workspace

- New agent tool `code_studio_workspace` (operations: list_files, read_file, read_file_excerpt, search,
  write_file, patch_file (exact text replace, unique match), run_command (timeout ≤ 120 s)) with `workspace_id`
  and workspace-relative `path`. Uses the same backend, honours read-only, desktop agent control and permission
  rules like `virtual_desktop_files`. Writes bump the preview version and emit a desktop event
  `code_studio_file_changed {workspace_id, path}`; Code Studio re-reads clean open tabs of that workspace.
- The Code Studio chat context sends `workspace_id`; when it is an SSH workspace the prompt says: the workspace is
  the remote folder `<label>` on `<host>`, use `code_studio_workspace` with that id for all file and command work,
  never `virtual_desktop_files` / `open_in_app` for these paths.

## 7. Plans

- **A** backend: workspace store + API, backend interface, local refactor, SSH backend (files, search, exec, git,
  terminal), devices/browse endpoints, host-key flow, capabilities. Go only.
- **B** HTML preview: token + route + live reload + version bumps (Go) and the preview pane UI.
- **C** workspace UI: switcher, status chip, connect dialog, folder browser, new-host form, per-workspace sessions,
  disconnect banner, feature gating, locales.
- **D** LSP over SSH + agent tool + prompt + file-changed refresh.

Order: A first; then B, C and D can run in parallel on disjoint files (B's Go part only needs the backend
interface). Testing stays lean: unit tests with fakes, an SSH backend test against an in-process
`golang.org/x/crypto/ssh` + `pkg/sftp` server, one browser smoke for preview and connect dialog; acceptance on
aurago-test by the user.
