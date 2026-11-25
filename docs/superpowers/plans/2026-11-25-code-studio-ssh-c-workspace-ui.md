# Plan C — Code Studio workspace UI

> For agentic workers: execute task by task, in order. Steps use `- [ ]`. Spec (decisions final): `docs/superpowers/specs/2026-11-25-code-studio-ssh-workspace-preview-design.md`, section 5, plus the API contract in section 2.

**Goal:** The front end for SSH workspaces in Code Studio:
- a workspace switcher at the top of the Explorer, with its menu
- a remote chip at the far left of the status bar
- a connect dialog with a host list, a "New SSH host" form, a folder browser and a host-key trust card
- tab sessions kept per workspace
- safe switching (Save all / Discard / Cancel)
- a disconnect banner with backoff reconnects
- capability gating for Git and LSP
- `user@host` in the window subtitle and the terminal tabs
- 72 new strings in all 16 languages

There is no Go change outside one browser test.

**Architecture**
1. `ui/js/desktop/apps/code-studio/workspace-model.js`: a pure UMD module, `window.CodeStudioWorkspaceModel`, with no DOM. It covers:
   - header and query injection
   - labels
   - session and active-workspace storage
   - error classification and backoff
   - remote path helpers and folder-listing normalisation
   - host search and sort
   - new-host validation

   It is tested by `workspace-model.test.cjs` (node:test + vm, the same pattern as `lsp-protocol.test.cjs`).
2. `ui/js/desktop/apps/code-studio/workspace.js`: a fragment of the shared Code Studio IIFE (`core.js` opens it, `shortcuts.js` closes it). It holds the switcher, chip, banner, menu, boot, switching, reconnect and connect dialog.
3. Integration into existing files. `core.js`:
   - `api()` adds `X-Code-Studio-Workspace`
   - `apiClient` gets the workspace endpoints
   - `saveState()` writes per-workspace sessions
   - `render()` boots the workspace
   - `renderStatus()` gets the chip
   - `shellMarkup()` gets the banner slot
   - download and upload URLs get `ws=`

   The other files:
   - `terminal.js`: socket URL and tab labels.
   - `lsp.js`: socket URL and the "not installed on host" text.
   - `sidebar.js`: the switcher in the Explorer head.
   - `git.js`: the "Git is not installed" empty state.
   - `shortcuts.js` and `command-palette.js`: commands.
   - `ui/css/code-studio.css`: styles.
4. `scripts/build-ui-bundles.js` gets 2 lines. Code Studio parts are listed by hand, not picked up automatically, and the pure module must be a standalone file for node tests. This is the worktree's own copy; the other session's uncommitted edits live in the main checkout. When merging to main the hunk is trivial: keep both sides.

**Tech stack:** vanilla JS fragments concatenated by the bundle script, CSS tokens `--cs-*` / `--vd-mat-*`, node:test, Go + rod for the one browser smoke.

---

## Conventions (repo rules)

- **Worktree:** `C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\cs-ssh-preview` (branch `feat/code-studio-ssh-preview`). Run every command from there.
- **Parallel plans:** plans B and D may work in the same worktree.
  - Commit only your own paths: `git add -- <new files>`, then `git commit -F <msgfile> -- <paths>`. The pathspec makes git commit only those paths.
  - Never use `git stash`.
- **Disposable folder** for message files and scripts: `C:\disposable\code-studio-plan-c\`. Never put them in the repo. Every commit message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Nothing is pushed.
- **Foreign edits:** before editing a shared file, run `git status --short -- <path>`. If it shows changes that are not yours, stop and report.
- **Bundle:** `npm run build:ui` regenerates every bundle; that is fine locally. Do not commit generated bundles until Task 15, which commits only `ui/js/desktop/bundles/code-studio.bundle.js`, and only after checking that no foreign uncommitted Code Studio sources exist.
- **GitNexus:** worktrees are not indexed, so use the main repo index. Before editing an existing function, run:
  `node C:\Users\Andi\.gitnexus\maintenance\AuraGo\graph\run.cjs impact "<symbol>" --direction upstream --repo C:\Users\Andi\Documents\repo\AuraGo`
  - Report the risk and warn on HIGH or CRITICAL.
  - `UNKNOWN` is unresolved: confirm the callers with Grep. Each task lists the callers it expects.
  - Before each commit, run `node C:\Users\Andi\.gitnexus\maintenance\AuraGo\graph\run.cjs detect-changes --scope all --repo C:\Users\Andi\Documents\repo\AuraGo`. `partial` or `truncated` means re-run.
- **Write tool and `\u`:** the Write tool decodes `\u` escapes. The code below has none and uses literal UTF-8 (`…`, `·`, `→`, `—`). Copy it as it stands.
- **Anchors decide, line numbers do not.** If an anchor is not found verbatim (for example because plan B or D touched it), put the code at the same logical place and say so in your report.
- **Browser tests:**
  - Run them in PowerShell, sandbox off, with `$env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:GOTMPDIR='D:\tmp\go-build'`.
  - If go reports `no such tool "vet"`, prefix `$env:GOTOOLCHAIN='go1.27.4';`.
  - Check for leftover headless rod Chromes first and ask the user before stopping any.
- **Lean tests:** the only new tests are the node test for the model and one browser smoke.

## Contract with plan A (assumed; adapt only `workspace-model.js` if A differs)

| Call | Request | What the UI reads |
|---|---|---|
| `GET /api/code-studio/workspaces` | – | `{workspaces:[{id, kind?, device_id, device_name, host, port, user\|username, root, label, last_used_at, connected?, capabilities?}]}` (a bare array is accepted too) |
| `POST /api/code-studio/workspaces` | `{device_id, root, trust_host_key?}` | the workspace or `{workspace}` |
| `DELETE /api/code-studio/workspaces/{id}` | – | any 2xx (204 ok) |
| `POST /api/code-studio/workspaces/{id}/connect` | `{trust_host_key?}` | `{workspace}` or `{capabilities}` |
| `GET /api/code-studio/remote/browse` | `?device_id=&path=&trust_host_key=` (no path = home) | `{path, entries:[{name, path?, type:'directory'\|'file', is_git?}]}` (`files` and `is_dir` accepted) |
| `GET /api/code-studio/remote/devices` | – | `{devices:[{id, name, host\|ip_address, port, username}]}` |
| `POST /api/code-studio/remote/devices` | `{name, host, port, username, auth, secret, passphrase?}` | `{device}` or the device |

**Errors.** Error bodies are JSON `{error, code}`. The UI handles these codes:
- `unknown_host_key` (409, with `host`, `key_type`, `fingerprint`)
- `host_key_mismatch` (409)
- `workspace_not_found` (404)
- `auth_failed`
- `not_directory`
- `not_found`

A broken SSH connection is recognised by any of:
- the codes `ssh_unavailable`, `ssh_unreachable`, `ssh_disconnected`, `unreachable` or `connection_lost`
- HTTP 502, 503 or 504
- a fetch network error

**Scoping.**
- Unscoped (no header, no `ws=`): `/workspaces*`, `/remote/*` and `/status`.
- Every other `/api/code-studio/*` request gets the header.
- Links and sockets get `?ws=<id>`. Plan A must accept `ws=` on HTTP endpoints too, because the desktop upload helper and download links cannot set headers.

## Hand-offs to plans B and D

- **What plan C owns:**
  - the `api()` header injection
  - the `terminal.js` and `lsp.js` socket URLs and the not-installed text
  - `saveState()`, the `render()` boot step, the status chip and the banner slot
  - the git gating
- **What plan B gets:** `POST /api/code-studio/preview` gets the header automatically.
- **Contract for plan D (and B):** `state.workspaceId`, `activeWorkspace()`, `workspaceIsRemote()`, `workspaceHostLabel()`, `workspaceHas(name)`, `workspaceURL(url)`, `workspaceSocketURL(path, instance)`.
- **Bug for plan D to fix:** `LSP_BRANCH_DIFF_COMMAND` in `lsp.js` hardcodes `git -C /workspace`, which is wrong on a remote host. D should drop `-C /workspace`; exec already runs in the workspace root.

## File map

| File | Change | Task |
|---|---|---|
| `ui/js/desktop/apps/code-studio/workspace-model.test.cjs` | Create | 1 |
| `ui/js/desktop/apps/code-studio/workspace-model.js` | Create | 2 |
| `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` | Modify (72 keys) | 3 |
| `C:\disposable\code-studio-plan-c\locales-c.mjs` | Create (not committed) | 3 |
| `ui/js/desktop/apps/code-studio/workspace.js` | Create, then append | 4, 6, 8, 9, 10 |
| `scripts/build-ui-bundles.js` | +2 lines | 4 |
| `ui/js/desktop/apps/code-studio/core.js` | Modify | 4, 6, 9, 13 |
| `ui/js/desktop/apps/code-studio/terminal.js` | Modify | 5, 9 |
| `ui/js/desktop/apps/code-studio/lsp.js` | Modify | 5, 12 |
| `ui/js/desktop/apps/code-studio/sidebar.js` | Modify | 6 |
| `ui/css/code-studio.css` | Append | 7, 11 |
| `ui/js/desktop/apps/code-studio/git.js` | Modify | 12 |
| `ui/js/desktop/apps/code-studio/shortcuts.js`, `command-palette.js` | Modify | 13 |
| `ui/desktop_code_studio_workspace_browser_test.go` | Create | 14 |
| `ui/js/desktop/bundles/code-studio.bundle.js` | Regenerate | 15 |

---

### Task 0: Preflight

- [ ] Run `git status --short` in the worktree. Note any foreign changes and do not touch them.
- [ ] Run `New-Item -ItemType Directory -Force C:\disposable\code-studio-plan-c`.

---

### Task 1: Model tests first (red)

**Files:** Create `ui/js/desktop/apps/code-studio/workspace-model.test.cjs`

- [ ] **Step 1:** Write the file with this complete content:

```js
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

// package.json has "type": "module": run the UMD source in a vm with a fake module, like lsp-protocol.test.cjs.
const mod = { exports: {} };
vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'workspace-model.js'), 'utf8'), { module: mod, window: {} });
const W = mod.exports;
const json = value => JSON.parse(JSON.stringify(value));

function memoryStorage() {
    const data = new Map();
    return { getItem: k => (data.has(k) ? data.get(k) : null), setItem: (k, v) => { data.set(k, String(v)); }, removeItem: k => { data.delete(k); } };
}

test('the header goes only on workspace-scoped Code Studio paths of a remote workspace', () => {
    const base = { credentials: 'same-origin', headers: { 'Content-Type': 'application/json' } };
    const scoped = W.withWorkspaceHeaders(base, 'ws-1', '/api/code-studio/files?path=%2Fworkspace');
    assert.equal(scoped.headers['X-Code-Studio-Workspace'], 'ws-1');
    assert.equal(scoped.headers['Content-Type'], 'application/json');
    assert.equal(base.headers['X-Code-Studio-Workspace'], undefined, 'caller options stay untouched');
    for (const p of ['/api/code-studio/workspaces', '/api/code-studio/workspaces/ws-1/connect', '/api/code-studio/remote/browse?device_id=d', '/api/code-studio/status', '/api/desktop/chat']) {
        assert.equal((W.withWorkspaceHeaders(base, 'ws-1', p).headers || {})['X-Code-Studio-Workspace'], undefined, p);
    }
    assert.equal(W.withWorkspaceHeaders(base, 'local', '/api/code-studio/files').headers['X-Code-Studio-Workspace'], undefined);
    assert.deepEqual(json(W.withWorkspaceHeaders({ method: 'POST' }, 'ws-1', '/api/code-studio/upload').headers), { 'X-Code-Studio-Workspace': 'ws-1' });
    assert.equal(W.withWorkspaceHeaders(base, 'bad id!', '/api/code-studio/files').headers['X-Code-Studio-Workspace'], undefined);
});

test('links and sockets carry ws=<id>', () => {
    assert.equal(W.withWorkspaceQuery('/api/code-studio/terminal', 'ws-1'), '/api/code-studio/terminal?ws=ws-1');
    assert.equal(W.withWorkspaceQuery('/api/code-studio/lsp?lang=go', 'ws-1'), '/api/code-studio/lsp?lang=go&ws=ws-1');
    assert.equal(W.withWorkspaceQuery('/api/code-studio/download?path=a#x', 'ws-1'), '/api/code-studio/download?path=a&ws=ws-1#x');
    assert.equal(W.withWorkspaceQuery('/api/code-studio/terminal?ws=ws-1', 'ws-1'), '/api/code-studio/terminal?ws=ws-1');
    assert.equal(W.withWorkspaceQuery('/api/code-studio/terminal', 'local'), '/api/code-studio/terminal');
    assert.equal(W.socketURL({ protocol: 'https:', host: 'aura:8443' }, '/api/code-studio/terminal', 'ws-1'), 'wss://aura:8443/api/code-studio/terminal?ws=ws-1');
    assert.equal(W.socketURL({ protocol: 'http:', host: 'aura' }, '/api/code-studio/lsp?lang=go', 'local'), 'ws://aura/api/code-studio/lsp?lang=go');
});

test('labels: device · ~/path, user@host, home shortcut', () => {
    const ws = W.normalizeWorkspace({ id: 'ws-1', device_id: 'd1', device_name: 'Pi', host: '10.0.0.5', port: 22, user: 'pi', root: '/home/pi/proj/' });
    assert.equal(ws.kind, 'ssh');
    assert.equal(ws.root, '/home/pi/proj');
    assert.equal(W.displayRoot(ws), '~/proj');
    assert.equal(W.workspaceLabel(ws), 'Pi · ~/proj');
    assert.equal(W.workspaceLabel(Object.assign({}, ws, { label: 'Website' })), 'Website');
    assert.equal(W.userAtHost(ws), 'pi@10.0.0.5');
    assert.equal(W.userAtHost(Object.assign({}, ws, { port: 2222 })), 'pi@10.0.0.5:2222');
    assert.equal(W.displayRoot(W.normalizeWorkspace({ id: 'r', user: 'root', root: '/root/x' })), '~/x');
    assert.equal(W.displayRoot(W.normalizeWorkspace({ id: 'o', user: 'pi', root: '/srv/www' })), '/srv/www');
    assert.equal(W.workspaceLabel(W.normalizeWorkspace({ id: 'h', host: 'box', user: 'u', root: '/home/u' })), 'box · ~');
    assert.equal(W.normalizeWorkspace({ id: 'local' }).kind, 'local');
    assert.equal(W.tildePath('/home/pi/a', '/home/pi'), '~/a');
    assert.equal(W.tildePath('/opt/a', '/home/pi'), '/opt/a');
});

test('workspace list: local first, newest next, duplicates dropped; merge keeps fields', () => {
    const list = W.normalizeWorkspaceList({ workspaces: [{ id: 'local' }, { id: 'a', host: 'h', root: '/a', last_used_at: '2026-11-01T10:00:00Z' }, { id: 'b', host: 'h', root: '/b', last_used_at: '2026-11-20T10:00:00Z' }, { id: 'a', host: 'x' }] });
    assert.deepEqual(list.map(w => w.id), ['local', 'b', 'a']);
    assert.deepEqual(W.normalizeWorkspaceList(null).map(w => w.id), ['local']);
    const merged = W.mergeWorkspace(list[1], { capabilities: { git: true } });
    assert.equal(merged.capabilities.git, true);
    assert.equal(merged.connected, true);
    assert.equal(merged.root, '/b');
});

test('sessions per workspace: sanitised, pruned, forgotten', () => {
    const storage = memoryStorage();
    assert.equal(W.SESSIONS_KEY, 'aurago.codeStudio.workspaceSessions.v1');
    assert.ok(W.writeSession(storage, 'ws-1', { openTabs: ['/workspace/a.go', '/etc/passwd', '/workspace/../x', 7], activeTabIndex: 5, recentFiles: ['/workspace/a.go'] }, 1000));
    W.writeSession(storage, 'local', { openTabs: ['/workspace/main.go'], activeTabIndex: 0 }, 2000);
    const sessions = W.readSessions(storage);
    assert.deepEqual(json(sessions['ws-1'].openTabs), ['/workspace/a.go']);
    assert.equal(sessions['ws-1'].activeTabIndex, 0);
    assert.deepEqual(json(sessions.local.openTabs), ['/workspace/main.go']);
    for (let i = 0; i < 40; i++) W.writeSession(storage, 'w' + i, { openTabs: [] }, 3000 + i);
    const pruned = W.readSessions(storage);
    assert.ok(Object.keys(pruned).length <= W.MAX_SESSIONS);
    assert.ok(pruned.local, 'local is never pruned');
    assert.ok(pruned.w39 && !pruned.w0);
    W.forgetSession(storage, 'w39');
    assert.equal(W.readSessions(storage).w39, undefined);
    storage.setItem(W.SESSIONS_KEY, '{broken');
    assert.deepEqual(json(W.readSessions(storage)), {});
});

test('active workspace per browser', () => {
    const storage = memoryStorage();
    assert.equal(W.readActive(storage), 'local');
    W.writeActive(storage, 'ws-1');
    assert.equal(storage.getItem('aurago.codeStudio.workspace.v1'), '{"id":"ws-1"}');
    assert.equal(W.readActive(storage), 'ws-1');
    storage.setItem(W.ACTIVE_KEY, '"<script>"');
    assert.equal(W.readActive(storage), 'local');
});

test('errors are classified; backoff grows to 60 s', () => {
    const e = (status, code) => Object.assign(new Error('x'), { status, code, body: code ? { code } : null });
    assert.equal(W.classifyError(e(409, 'unknown_host_key')), 'unknown_host_key');
    assert.equal(W.classifyError(e(409, 'host_key_mismatch')), 'host_key_mismatch');
    assert.equal(W.classifyError(e(404, 'workspace_not_found')), 'workspace_not_found');
    assert.equal(W.classifyError(e(400, 'auth_failed')), 'auth');
    assert.equal(W.classifyError(e(400, 'not_directory')), 'not_directory');
    assert.equal(W.classifyError(e(404)), 'not_found');
    assert.equal(W.classifyError(e(502)), 'unreachable');
    assert.equal(W.classifyError(e(500, 'ssh_unavailable')), 'unreachable');
    assert.equal(W.classifyError(new TypeError('Failed to fetch')), 'unreachable');
    assert.equal(W.classifyError(e(403)), 'other', 'a read-only desktop is not a sign-in failure');
    assert.deepEqual([0, 1, 2, 3, 4, 5, 9].map(W.backoffDelay), [2000, 5000, 10000, 20000, 30000, 60000, 60000]);
});

test('new host form validation', () => {
    const ok = W.validateHostForm({ name: 'Pi', host: '192.168.1.20', port: '22', username: 'pi', auth: 'password', secret: 'pw' });
    assert.equal(ok.valid, true);
    assert.deepEqual(json(ok.value), { name: 'Pi', host: '192.168.1.20', port: 22, username: 'pi', auth: 'password', secret: 'pw', passphrase: '' });
    const bad = W.validateHostForm({ name: '', host: 'ssh://pi@box', port: '70000', username: '', auth: 'key', secret: 'not a key' });
    assert.deepEqual(json(bad.errors), { name: 'required', host: 'host', port: 'port', username: 'required', secret: 'key' });
    const key = W.validateHostForm({ name: 'b', host: '[fe80::1]', port: '', username: 'u', auth: 'key', secret: '-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----', passphrase: 'pp' });
    assert.equal(key.valid, true);
    assert.equal(key.value.port, 22);
    assert.equal(key.value.host, 'fe80::1');
    assert.equal(key.value.passphrase, 'pp');
    assert.ok(key.value.secret.endsWith('-----END OPENSSH PRIVATE KEY-----\n'));
    assert.equal(W.validateHostForm({ name: 'x', host: 'h', username: 'u', auth: 'password', secret: '' }).errors.secret, 'required');
});

test('remote paths: normalise, parent, breadcrumbs, home', () => {
    assert.equal(W.normalizeRemotePath('/home//pi/./proj/../web/'), '/home/pi/web');
    assert.equal(W.normalizeRemotePath('/../..'), '/');
    assert.equal(W.normalizeRemotePath('src', '/home/pi'), '/home/pi/src');
    assert.equal(W.normalizeRemotePath(''), '/');
    assert.equal(W.normalizeRemotePath('~/x'), '~/x');
    assert.equal(W.expandHome('~/x', '/home/pi'), '/home/pi/x');
    assert.equal(W.expandHome('~', '/home/pi'), '/home/pi');
    assert.equal(W.expandHome('/opt', '/home/pi'), '/opt');
    assert.equal(W.remoteParent('/home/pi'), '/home');
    assert.equal(W.remoteParent('/home'), '/');
    assert.equal(W.remoteParent('/'), '/');
    assert.deepEqual(json(W.remoteBreadcrumbs('/home/pi')), [{ name: '/', path: '/' }, { name: 'home', path: '/home' }, { name: 'pi', path: '/home/pi' }]);
});

test('folder listing: folders, then dot folders, then files', () => {
    const listing = W.normalizeBrowse({ path: '/home/pi', entries: [{ name: 'notes.txt', type: 'file' }, { name: '.cache', type: 'directory' }, { name: 'proj', type: 'directory', is_git: true }, { name: 'Apps', path: '/home/pi/Apps', is_dir: true }, { name: '..', type: 'directory' }] });
    assert.equal(listing.path, '/home/pi');
    assert.deepEqual(json(listing.entries.map(e => [e.name, e.dir, e.git, e.path])), [['Apps', true, false, '/home/pi/Apps'], ['proj', true, true, '/home/pi/proj'], ['.cache', true, false, '/home/pi/.cache'], ['notes.txt', false, false, '/home/pi/notes.txt']]);
});

test('hosts: search, last used, recent folders, relative time', () => {
    const workspaces = W.normalizeWorkspaceList([{ id: 'a', device_id: 'd1', root: '/home/pi/a', last_used_at: '2026-11-20T00:00:00Z' }, { id: 'b', device_id: 'd1', root: '/srv/b', last_used_at: '2026-11-24T00:00:00Z' }]);
    const devices = W.normalizeDeviceList({ devices: [{ id: 'd2', name: 'NAS', ip_address: '10.0.0.9', username: 'admin' }, { id: 'd1', name: 'Pi', host: '10.0.0.5', port: 2222, username: 'pi' }, { name: 'no id' }] });
    assert.deepEqual(devices.map(d => d.id), ['d2', 'd1']);
    assert.equal(W.deviceAddress(devices[1]), 'pi@10.0.0.5:2222');
    assert.equal(W.deviceAddress(devices[0]), 'admin@10.0.0.9:22');
    assert.deepEqual(W.sortDevices(devices, workspaces).map(d => d.id), ['d1', 'd2']);
    assert.deepEqual(W.filterDevices(devices, 'pi@').map(d => d.id), ['d1']);
    assert.deepEqual(W.filterDevices(devices, ' nas ').map(d => d.id), ['d2']);
    assert.equal(W.filterDevices(devices, '').length, 2);
    assert.deepEqual(json(W.recentRoots(workspaces, 'd1')), ['/srv/b', '/home/pi/a']);
    assert.equal(W.deviceLastUsed(devices[1], workspaces), Date.parse('2026-11-24T00:00:00Z'));
    assert.deepEqual(json(W.relativeTime(Date.parse('2026-11-23T00:00:00Z'), Date.parse('2026-11-25T00:00:00Z'))), { value: -2, unit: 'day' });
    assert.equal(W.relativeTime(0, Date.now()), null);
});
```

- [ ] **Step 2:** Run `node --test ui/js/desktop/apps/code-studio/workspace-model.test.cjs`.
  Expected: it fails with `ENOENT ... workspace-model.js`.
- [ ] **Step 3:** No commit; Task 2 commits both files.

---

### Task 2: The pure model (green)

**Files:** Create `ui/js/desktop/apps/code-studio/workspace-model.js`

- [ ] **Step 1:** Write the file with this complete content:

```js
/* Code Studio workspaces (plan C): pure helpers without DOM access - request header and link/socket query injection,
   labels, per-workspace tab sessions and the active workspace in localStorage, error classification, reconnect backoff,
   remote paths, folder listings, host search and new-host validation. Runs in the code-studio bundle
   (window.CodeStudioWorkspaceModel) and under node (workspace-model.test.cjs). */
(function (root, factory) {
    'use strict';
    const api = factory();
    if (typeof module === 'object' && module && module.exports) module.exports = api;
    if (root) root.CodeStudioWorkspaceModel = api;
})(typeof window !== 'undefined' ? window : null, () => {
    'use strict';

    const LOCAL_ID = 'local';
    const HEADER = 'X-Code-Studio-Workspace';
    const SESSIONS_KEY = 'aurago.codeStudio.workspaceSessions.v1';
    const ACTIVE_KEY = 'aurago.codeStudio.workspace.v1';
    const MAX_SESSIONS = 24;
    const MAX_SESSION_PATHS = 20;
    const BACKOFF_MS = [2000, 5000, 10000, 20000, 30000, 60000];
    const ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/;
    // Workspace management and the container status are global; everything else under /api/code-studio/ is scoped.
    const UNSCOPED_PREFIXES = ['/api/code-studio/workspaces', '/api/code-studio/remote/', '/api/code-studio/status'];
    // Plan A's codes for a broken SSH connection (502/503/504 and fetch network errors count too).
    const CONNECTION_CODES = ['ssh_unavailable', 'ssh_unreachable', 'ssh_disconnected', 'unreachable', 'connection_lost'];
    const PRIVATE_KEY = /-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----/;
    const HOST_PATTERN = /^[A-Za-z0-9._:[\]-]+$/;

    function normalizeId(id) {
        const value = String(id == null ? '' : id).trim();
        return ID_PATTERN.test(value) ? value : LOCAL_ID;
    }
    function isRemote(id) { return normalizeId(id) !== LOCAL_ID; }
    function isScopedPath(path) {
        const value = String(path || '');
        return value.startsWith('/api/code-studio/') && !UNSCOPED_PREFIXES.some(prefix => value.startsWith(prefix));
    }
    // options.headers is a plain object (api() in core.js); FormData uploads come without headers.
    function withWorkspaceHeaders(options, id, path) {
        const result = Object.assign({}, options || {});
        if (!isRemote(id) || !isScopedPath(path)) return result;
        result.headers = Object.assign({}, result.headers || {}, { [HEADER]: normalizeId(id) });
        return result;
    }
    function withWorkspaceQuery(url, id) {
        const value = String(url || '');
        if (!isRemote(id)) return value;
        const hashAt = value.indexOf('#');
        const base = hashAt >= 0 ? value.slice(0, hashAt) : value;
        const hash = hashAt >= 0 ? value.slice(hashAt) : '';
        if (/[?&]ws=/.test(base)) return value;
        return base + (base.includes('?') ? '&' : '?') + 'ws=' + encodeURIComponent(normalizeId(id)) + hash;
    }
    function socketURL(loc, path, id) {
        return (loc && loc.protocol === 'https:' ? 'wss:' : 'ws:') + '//' + String(loc && loc.host || '') + withWorkspaceQuery(path, id);
    }
    function timeValue(value) {
        if (typeof value === 'number') return Number.isFinite(value) && value > 0 ? value : 0;
        const parsed = Date.parse(String(value || ''));
        return Number.isFinite(parsed) ? parsed : 0;
    }

    function localWorkspace() {
        return { id: LOCAL_ID, kind: 'local', deviceId: '', deviceName: '', host: '', port: 0, user: '', root: '/workspace', label: '', lastUsedAt: 0, connected: true, capabilities: null };
    }
    function normalizeWorkspace(raw) {
        const item = raw && raw.workspace && typeof raw.workspace === 'object' ? raw.workspace : (raw || {});
        const id = normalizeId(item.id);
        if (id === LOCAL_ID) return localWorkspace();
        return {
            id, kind: 'ssh',
            deviceId: String(item.device_id || ''),
            deviceName: String(item.device_name || item.device || ''),
            host: String(item.host || ''),
            port: Number(item.port) || 22,
            user: String(item.user || item.username || ''),
            root: normalizeRemotePath(item.root || '/'),
            label: String(item.label || '').trim(),
            lastUsedAt: timeValue(item.last_used_at),
            connected: item.connected === true,
            capabilities: item.capabilities && typeof item.capabilities === 'object' ? item.capabilities : null
        };
    }
    // current is a normalized workspace, payload a raw API answer ({workspace} or {capabilities}).
    function mergeWorkspace(current, payload) {
        const base = current && current.id ? current : localWorkspace();
        const raw = payload && payload.workspace && typeof payload.workspace === 'object' ? payload.workspace : payload;
        if (raw && raw.id != null && base.id !== LOCAL_ID && normalizeId(raw.id) === base.id) {
            const fresh = normalizeWorkspace(raw);
            return Object.assign({}, base, fresh, { capabilities: fresh.capabilities || base.capabilities, connected: true });
        }
        const caps = raw && raw.capabilities && typeof raw.capabilities === 'object' ? raw.capabilities : base.capabilities;
        return Object.assign({}, base, { capabilities: caps, connected: true });
    }
    function normalizeWorkspaceList(payload) {
        const list = Array.isArray(payload) ? payload : (payload && Array.isArray(payload.workspaces) ? payload.workspaces : []);
        const seen = new Set();
        const remote = [];
        list.forEach(raw => {
            const ws = normalizeWorkspace(raw);
            if (ws.id === LOCAL_ID || seen.has(ws.id)) return;
            seen.add(ws.id);
            remote.push(ws);
        });
        remote.sort((a, b) => (b.lastUsedAt - a.lastUsedAt) || a.id.localeCompare(b.id));
        return [localWorkspace()].concat(remote);
    }

    function homeOf(user) { return user === 'root' ? '/root' : (user ? '/home/' + user : ''); }
    function tildePath(path, home) {
        const value = String(path || '');
        if (!home || home === '/') return value;
        if (value === home) return '~';
        return value.startsWith(home + '/') ? '~' + value.slice(home.length) : value;
    }
    function displayRoot(ws) { return ws && ws.kind === 'ssh' ? tildePath(ws.root, homeOf(ws.user)) : '/workspace'; }
    function userAtHost(ws) {
        if (!ws || ws.kind === 'local') return '';
        return (ws.user ? ws.user + '@' : '') + (ws.host || '') + (ws.port && ws.port !== 22 ? ':' + ws.port : '');
    }
    function workspaceLabel(ws) {
        if (!ws || ws.kind !== 'ssh') return '';
        return ws.label || ((ws.deviceName || ws.host || ws.id) + ' · ' + displayRoot(ws));
    }

    function normalizeRemotePath(path, base) {
        let value = String(path == null ? '' : path).replace(/\\/g, '/').replace(/\0/g, '').trim();
        if (value === '~' || value.startsWith('~/')) {
            const rest = value.slice(2).split('/').filter(part => part && part !== '.').join('/');
            return rest ? '~/' + rest : '~';
        }
        if (!value.startsWith('/')) value = (base && String(base).startsWith('/') ? String(base) : '') + '/' + value;
        const parts = [];
        value.split('/').forEach(part => {
            if (!part || part === '.') return;
            if (part === '..') parts.pop();
            else parts.push(part);
        });
        return '/' + parts.join('/');
    }
    function expandHome(path, home) {
        const value = String(path || '').trim();
        if (!home || !(value === '~' || value.startsWith('~/'))) return value;
        return value === '~' ? home : home.replace(/\/+$/, '') + value.slice(1);
    }
    function remoteParent(path) {
        const value = normalizeRemotePath(path);
        if (value === '/' || value.startsWith('~')) return '/';
        const index = value.lastIndexOf('/');
        return index <= 0 ? '/' : value.slice(0, index);
    }
    function remoteBreadcrumbs(path) {
        const crumbs = [{ name: '/', path: '/' }];
        let current = '';
        normalizeRemotePath(path).split('/').filter(Boolean).forEach(part => { current += '/' + part; crumbs.push({ name: part, path: current }); });
        return crumbs;
    }
    function normalizeBrowse(payload) {
        const data = payload || {};
        const path = normalizeRemotePath(data.path || '/');
        const raw = Array.isArray(data.entries) ? data.entries : (Array.isArray(data.files) ? data.files : []);
        const entries = [];
        raw.forEach(item => {
            const name = String(item && item.name || '');
            if (!name || name === '.' || name === '..' || name.includes('/')) return;
            const dir = item.type === 'directory' || item.type === 'dir' || item.is_dir === true;
            entries.push({ name, path: item.path ? normalizeRemotePath(item.path) : normalizeRemotePath(name, path), dir, git: dir && item.is_git === true, hidden: name.startsWith('.') });
        });
        const rank = entry => (entry.dir ? (entry.hidden ? 1 : 0) : 2);
        entries.sort((a, b) => (rank(a) - rank(b)) || a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }));
        return { path, entries };
    }

    function readJSON(storage, key) {
        try {
            const raw = storage && storage.getItem(key);
            return raw ? JSON.parse(raw) : null;
        } catch (_) {
            return null;
        }
    }
    function sessionPaths(list) {
        const seen = new Set();
        const result = [];
        (Array.isArray(list) ? list : []).forEach(item => {
            const path = typeof item === 'string' ? item : (item && typeof item.path === 'string' ? item.path : '');
            if (!/^\/workspace\/./.test(path) || path.includes('\0') || path.split('/').some(part => part === '..' || part === '.') || seen.has(path)) return;
            seen.add(path);
            if (result.length < MAX_SESSION_PATHS) result.push(path);
        });
        return result;
    }
    function sanitizeSession(raw) {
        if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return null;
        const openTabs = sessionPaths(raw.openTabs);
        const index = Number(raw.activeTabIndex);
        return {
            openTabs,
            activeTabIndex: openTabs.length ? Math.min(Math.max(Number.isInteger(index) ? index : 0, 0), openTabs.length - 1) : -1,
            recentFiles: sessionPaths(raw.recentFiles),
            split: raw.split && typeof raw.split === 'object' && !Array.isArray(raw.split) ? raw.split : null,
            updatedAt: timeValue(raw.updatedAt)
        };
    }
    function readSessions(storage) {
        const data = readJSON(storage, SESSIONS_KEY);
        const sessions = {};
        if (!data || typeof data !== 'object' || Array.isArray(data)) return sessions;
        Object.keys(data).forEach(id => {
            if (normalizeId(id) !== id) return;
            const session = sanitizeSession(data[id]);
            if (session) sessions[id] = session;
        });
        return sessions;
    }
    function writeSessions(storage, sessions) {
        const keep = {};
        if (sessions[LOCAL_ID]) keep[LOCAL_ID] = sessions[LOCAL_ID];
        Object.keys(sessions).filter(id => id !== LOCAL_ID)
            .sort((a, b) => sessions[b].updatedAt - sessions[a].updatedAt)
            .slice(0, MAX_SESSIONS - (sessions[LOCAL_ID] ? 1 : 0))
            .forEach(id => { keep[id] = sessions[id]; });
        try {
            storage.setItem(SESSIONS_KEY, JSON.stringify(keep));
            return true;
        } catch (_) {
            return false;
        }
    }
    function writeSession(storage, id, session, now) {
        const sessions = readSessions(storage);
        const clean = sanitizeSession(Object.assign({}, session || {}, { updatedAt: Number(now) || Date.now() }));
        if (!clean) return false;
        sessions[normalizeId(id)] = clean;
        return writeSessions(storage, sessions);
    }
    function forgetSession(storage, id) {
        const sessions = readSessions(storage);
        if (!sessions[id]) return false;
        delete sessions[id];
        return writeSessions(storage, sessions);
    }
    function readActive(storage) {
        const data = readJSON(storage, ACTIVE_KEY);
        return normalizeId(data && typeof data === 'object' ? data.id : data);
    }
    function writeActive(storage, id) {
        try {
            storage.setItem(ACTIVE_KEY, JSON.stringify({ id: normalizeId(id) }));
            return true;
        } catch (_) {
            return false;
        }
    }

    function classifyError(err) {
        const status = Number(err && err.status) || 0;
        const code = String((err && (err.code || (err.body && err.body.code))) || '');
        if (code === 'unknown_host_key' || code === 'host_key_mismatch' || code === 'workspace_not_found') return code;
        if (code === 'auth_failed') return 'auth';
        if (code === 'not_directory') return 'not_directory';
        if (CONNECTION_CODES.includes(code) || status === 502 || status === 503 || status === 504) return 'unreachable';
        if (!status && err && err.name === 'TypeError') return 'unreachable';
        if (code === 'not_found' || status === 404) return 'not_found';
        return 'other';
    }
    function backoffDelay(attempt) {
        const index = Math.max(0, Math.floor(Number(attempt) || 0));
        return BACKOFF_MS[Math.min(index, BACKOFF_MS.length - 1)];
    }

    // Error values are keys of the form messages: required, host, port, key.
    function validateHostForm(values) {
        const v = values || {};
        const errors = {};
        const name = String(v.name || '').trim().slice(0, 80);
        const host = String(v.host || '').trim();
        const username = String(v.username || '').trim();
        const portText = String(v.port == null || v.port === '' ? '22' : v.port).trim();
        const auth = v.auth === 'key' ? 'key' : 'password';
        const secret = String(v.secret || '');
        if (!name) errors.name = 'required';
        if (!host) errors.host = 'required';
        else if (!HOST_PATTERN.test(host)) errors.host = 'host';
        if (!/^\d{1,5}$/.test(portText) || Number(portText) < 1 || Number(portText) > 65535) errors.port = 'port';
        if (!username) errors.username = 'required';
        if (!secret) errors.secret = 'required';
        else if (auth === 'key' && (!PRIVATE_KEY.test(secret) || secret.length > 65536)) errors.secret = 'key';
        const valid = !Object.keys(errors).length;
        return {
            valid,
            errors,
            value: valid ? { name, host: host.replace(/^\[|\]$/g, ''), port: Number(portText), username, auth, secret: auth === 'key' ? secret.trim() + '\n' : secret, passphrase: auth === 'key' ? String(v.passphrase || '') : '' } : null
        };
    }

    function normalizeDevice(raw) {
        const d = raw && raw.device && typeof raw.device === 'object' ? raw.device : (raw || {});
        const host = String(d.host || d.ip_address || '');
        return { id: String(d.id == null ? '' : d.id), name: String(d.name || host), host, port: Number(d.port) || 22, user: String(d.username || d.user || ''), lastUsedAt: timeValue(d.last_used_at) };
    }
    function normalizeDeviceList(payload) {
        const list = Array.isArray(payload) ? payload : (payload && Array.isArray(payload.devices) ? payload.devices : []);
        return list.map(normalizeDevice).filter(device => device.id);
    }
    function deviceAddress(device) {
        return device ? (device.user ? device.user + '@' : '') + device.host + ':' + (device.port || 22) : '';
    }
    function deviceLastUsed(device, workspaces) {
        let latest = device && device.lastUsedAt || 0;
        (workspaces || []).forEach(ws => { if (device && ws.deviceId === device.id && ws.lastUsedAt > latest) latest = ws.lastUsedAt; });
        return latest;
    }
    function sortDevices(devices, workspaces) {
        return (devices || []).slice().sort((a, b) => (deviceLastUsed(b, workspaces) - deviceLastUsed(a, workspaces)) || a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }));
    }
    function filterDevices(devices, query) {
        const q = String(query || '').trim().toLowerCase();
        if (!q) return (devices || []).slice();
        return (devices || []).filter(device => [device.name, device.host, device.user, deviceAddress(device)].some(value => String(value || '').toLowerCase().includes(q)));
    }
    function recentRoots(workspaces, deviceId) {
        const roots = [];
        (workspaces || []).filter(ws => ws.kind === 'ssh' && ws.deviceId === deviceId)
            .sort((a, b) => b.lastUsedAt - a.lastUsedAt)
            .forEach(ws => { if (!roots.includes(ws.root)) roots.push(ws.root); });
        return roots.slice(0, 5);
    }
    // {value, unit} for Intl.RelativeTimeFormat; null when never used.
    function relativeTime(ts, now) {
        const time = timeValue(ts);
        if (!time) return null;
        const seconds = (time - (Number(now) || Date.now())) / 1000;
        for (const [unit, size] of [['year', 31536000], ['month', 2592000], ['week', 604800], ['day', 86400], ['hour', 3600], ['minute', 60]]) {
            if (Math.abs(seconds) >= size) return { value: Math.round(seconds / size), unit };
        }
        return { value: 0, unit: 'minute' };
    }

    return {
        LOCAL_ID, HEADER, SESSIONS_KEY, ACTIVE_KEY, MAX_SESSIONS,
        normalizeId, isRemote, isScopedPath, withWorkspaceHeaders, withWorkspaceQuery, socketURL,
        localWorkspace, normalizeWorkspace, mergeWorkspace, normalizeWorkspaceList,
        displayRoot, tildePath, userAtHost, workspaceLabel,
        normalizeRemotePath, expandHome, remoteParent, remoteBreadcrumbs, normalizeBrowse,
        readSessions, writeSession, forgetSession, readActive, writeActive,
        classifyError, backoffDelay, validateHostForm,
        normalizeDevice, normalizeDeviceList, deviceAddress, deviceLastUsed, sortDevices, filterDevices, recentRoots, relativeTime
    };
});
```

- [ ] **Step 2:** Run `node --test ui/js/desktop/apps/code-studio/workspace-model.test.cjs`.
  Expected: `# pass 11`, `# fail 0`.
- [ ] **Step 3:** Run detect-changes.
- [ ] **Step 4:** Commit.
  1. Write `C:\disposable\code-studio-plan-c\msg-2.txt`:
     ```
     feat(code-studio): pure workspace model for SSH workspaces

     Header and ws= injection, labels, per-workspace tab sessions and the active
     workspace in localStorage, error classification, backoff, remote paths,
     folder listings, host search and new-host validation. Node tests.

     Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
     ```
  2. Run:
     `git add -- ui/js/desktop/apps/code-studio/workspace-model.js ui/js/desktop/apps/code-studio/workspace-model.test.cjs`
  3. Run:
     `git commit -F C:\disposable\code-studio-plan-c\msg-2.txt -- ui/js/desktop/apps/code-studio/workspace-model.js ui/js/desktop/apps/code-studio/workspace-model.test.cjs`

---

### Task 3: Strings for all 16 languages

**Files:**
- Create (not committed): `C:\disposable\code-studio-plan-c\locales-c.mjs`
- Modify: `ui/lang/desktop/*.json`

The locale files are flat JSON with 2-space indentation. The new lines go right after `"codeStudio.workspaceRoot":`, which is not the last key. Plans B and D should use another anchor to avoid adjacent-line clashes.

**English source for all 72 keys** (the `en` column of the script):

| Key | English |
|---|---|
| workspaceLocal | Local workspace |
| workspaceLocalSub | Container · /workspace |
| workspaceChipLocal | Local |
| workspaceChipRemote | SSH: {{name}} |
| workspaceMenu | Workspaces |
| workspaceRecent | Recent SSH folders |
| workspaceConnect | Connect to SSH Folder… |
| workspaceReconnect | Reconnect |
| workspaceDisconnect | Disconnect |
| workspaceRemove | Remove |
| workspaceRemoveConfirm | Remove {{workspace}} from the list? The files on the host stay untouched. |
| workspaceStatusConnected | Connected |
| workspaceStatusDisconnected | Not connected |
| workspaceStatusConnecting | Connecting… |
| workspaceStatusLost | Connection lost |
| workspaceStatusError | Connection failed |
| workspaceLost | Connection to {{host}} lost |
| workspaceRetryIn | Retrying in {{seconds}} s |
| workspaceReconnected | Reconnected to {{host}} |
| workspaceUseLocal | Use local workspace |
| workspaceReviewHostKey | Review host key… |
| workspaceDirtyTitle | Unsaved changes |
| workspaceDirtyPrompt | {{count}} files have unsaved changes. Save them before switching to {{workspace}}? |
| workspaceSaveFailedToast | {{file}} was not saved. {{error}} Your changes stay in the tab. |
| workspaceSwitchFailed | Could not open {{workspace}}: {{error}} |
| workspaceMissing | This SSH workspace no longer exists. Code Studio switched to the local workspace. |
| workspaceGitMissing | Git is not installed on {{host}} |
| workspaceLspNotInstalled | not installed on {{host}} |
| workspaceDialogTitle | Connect to SSH Folder |
| workspaceHosts | Hosts |
| workspaceSearchHosts | Search hosts |
| workspaceNoHosts | No SSH hosts in the inventory yet. |
| workspaceNoHostMatch | No host matches “{{query}}” |
| workspaceNewHost | New SSH Host |
| workspaceNeverUsed | Never used |
| workspacePickHost | Choose a host to see its folders. |
| workspaceHome | Home folder |
| workspaceUp | Parent folder |
| workspacePathLabel | Folder path |
| workspaceNoSubfolders | This folder has no subfolders. |
| workspaceKeyHint | Enter opens · → enters a folder · ← goes up |
| workspaceOpenFolder | Open Folder |
| workspaceTrustTitle | Unknown host key |
| workspaceTrustText | Code Studio has not connected to {{host}} before. Compare the fingerprint with the one the server shows before you trust it. |
| workspaceTrustKeyType | Key type |
| workspaceTrustFingerprint | Fingerprint |
| workspaceTrustContinue | Trust and Continue |
| workspaceMismatchTitle | Host key changed |
| workspaceErrorHostKeyMismatch | The host key of {{host}} has changed. Code Studio does not connect until known_hosts is fixed. |
| workspaceErrorAuth | Sign-in to {{host}} failed. Check the user name and the password or key. |
| workspaceErrorUnreachable | {{host}} cannot be reached. |
| workspaceErrorNotDirectory | {{path}} is not a folder. |
| workspaceErrorNotFound | {{path}} does not exist. |
| workspaceErrorOther | Something went wrong: {{error}} |
| workspaceChooseOtherHost | Choose Another Host |
| workspaceFormName | Name |
| workspaceFormHost | Host |
| workspaceFormPort | Port |
| workspaceFormUser | User |
| workspaceFormAuth | Sign in with |
| workspaceFormPassword | Password |
| workspaceFormKey | Private key |
| workspaceFormKeyFile | Load Key File… |
| workspaceFormPassphrase | Passphrase (optional) |
| workspaceFormVaultHint | The secret is stored in the AuraGo vault and never kept in the browser. |
| workspaceFormAdd | Add Host |
| workspaceFormRequired | Required |
| workspaceFormBadHost | Enter a host name or IP address, without user or scheme |
| workspaceFormBadPort | Port 1–65535 |
| workspaceFormBadKey | Paste a private key that starts with -----BEGIN |
| workspaceFormReadOnly | The desktop is read-only: new hosts cannot be added. |
| workspaceFormSaveFailed | Could not add the host: {{error}} |

The table and the `en` column of the script agree; the script is authoritative.

- [ ] **Step 1:** Run `git status --short -- ui/lang/desktop`.
  Expected: no output. If anything shows, stop and report.
- [ ] **Step 2:** Create `C:\disposable\code-studio-plan-c\locales-c.mjs`. Every array follows the order of `langs`.

```js
// Plan C: adds the codeStudio.workspace* strings to the 16 desktop locales, right after "codeStudio.workspaceRoot".
// Usage: node C:\disposable\code-studio-plan-c\locales-c.mjs C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\cs-ssh-preview
import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';

const repo = process.argv[2];
if (!repo) throw new Error('pass the worktree path');
const langs = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const anchorKey = '"codeStudio.workspaceRoot":';
//                         cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh
const S = {
workspaceLocal: ["Místní pracovní prostor","Lokalt arbejdsområde","Lokaler Arbeitsbereich","Τοπικός χώρος εργασίας","Local workspace","Espacio de trabajo local","Espace de travail local","लोकल वर्कस्पेस","Area di lavoro locale","ローカルワークスペース","Lokale werkruimte","Lokalt arbeidsområde","Lokalny obszar roboczy","Área de trabalho local","Lokal arbetsyta","本地工作区"],
workspaceLocalSub: ["Kontejner · /workspace","Container · /workspace","Container · /workspace","Κοντέινερ · /workspace","Container · /workspace","Contenedor · /workspace","Conteneur · /workspace","कंटेनर · /workspace","Container · /workspace","コンテナー · /workspace","Container · /workspace","Container · /workspace","Kontener · /workspace","Contêiner · /workspace","Container · /workspace","容器 · /workspace"],
workspaceChipLocal: ["Místní","Lokal","Lokal","Τοπικό","Local","Local","Local","लोकल","Locale","ローカル","Lokaal","Lokal","Lokalny","Local","Lokal","本地"],
workspaceChipRemote: ["SSH: {{name}}","SSH: {{name}}","SSH: {{name}}","SSH: {{name}}","SSH: {{name}}","SSH: {{name}}","SSH : {{name}}","SSH: {{name}}","SSH: {{name}}","SSH: {{name}}","SSH: {{name}}","SSH: {{name}}","SSH: {{name}}","SSH: {{name}}","SSH: {{name}}","SSH：{{name}}"],
workspaceMenu: ["Pracovní prostory","Arbejdsområder","Arbeitsbereiche","Χώροι εργασίας","Workspaces","Espacios de trabajo","Espaces de travail","वर्कस्पेस","Aree di lavoro","ワークスペース","Werkruimtes","Arbeidsområder","Obszary robocze","Áreas de trabalho","Arbetsytor","工作区"],
workspaceRecent: ["Nedávné složky SSH","Seneste SSH-mapper","Zuletzt genutzte SSH-Ordner","Πρόσφατοι φάκελοι SSH","Recent SSH folders","Carpetas SSH recientes","Dossiers SSH récents","हाल के SSH फ़ोल्डर","Cartelle SSH recenti","最近の SSH フォルダー","Recente SSH-mappen","Nylige SSH-mapper","Ostatnie foldery SSH","Pastas SSH recentes","Senaste SSH-mappar","最近的 SSH 文件夹"],
workspaceConnect: ["Připojit ke složce SSH…","Forbind til SSH-mappe…","Mit SSH-Ordner verbinden…","Σύνδεση σε φάκελο SSH…","Connect to SSH Folder…","Conectar a carpeta SSH…","Se connecter à un dossier SSH…","SSH फ़ोल्डर से कनेक्ट करें…","Connetti a cartella SSH…","SSH フォルダーに接続…","Verbinden met SSH-map…","Koble til SSH-mappe…","Połącz z folderem SSH…","Conectar a pasta SSH…","Anslut till SSH-mapp…","连接到 SSH 文件夹…"],
workspaceReconnect: ["Znovu připojit","Forbind igen","Neu verbinden","Επανασύνδεση","Reconnect","Reconectar","Reconnecter","फिर से कनेक्ट करें","Riconnetti","再接続","Opnieuw verbinden","Koble til igjen","Połącz ponownie","Reconectar","Anslut igen","重新连接"],
workspaceDisconnect: ["Odpojit","Afbryd forbindelse","Trennen","Αποσύνδεση","Disconnect","Desconectar","Déconnecter","डिस्कनेक्ट करें","Disconnetti","切断","Verbinding verbreken","Koble fra","Rozłącz","Desconectar","Koppla från","断开连接"],
workspaceRemove: ["Odebrat","Fjern","Entfernen","Αφαίρεση","Remove","Quitar","Retirer","हटाएँ","Rimuovi","削除","Verwijderen","Fjern","Usuń","Remover","Ta bort","移除"],
workspaceRemoveConfirm: ["Odebrat {{workspace}} ze seznamu? Soubory na hostiteli zůstanou beze změny.","Fjern {{workspace}} fra listen? Filerne på værten forbliver urørte.","{{workspace}} aus der Liste entfernen? Die Dateien auf dem Host bleiben unverändert.","Αφαίρεση του {{workspace}} από τη λίστα; Τα αρχεία στον κεντρικό υπολογιστή μένουν ανέγγιχτα.","Remove {{workspace}} from the list? The files on the host stay untouched.","¿Quitar {{workspace}} de la lista? Los archivos del host no se tocan.","Retirer {{workspace}} de la liste ? Les fichiers sur l'hôte restent intacts.","{{workspace}} को सूची से हटाएँ? होस्ट पर फ़ाइलें वैसी ही रहेंगी।","Rimuovere {{workspace}} dall'elenco? I file sull'host restano invariati.","{{workspace}} を一覧から削除しますか？ホスト上のファイルはそのままです。","{{workspace}} uit de lijst verwijderen? De bestanden op de host blijven ongewijzigd.","Fjerne {{workspace}} fra listen? Filene på verten forblir urørt.","Usunąć {{workspace}} z listy? Pliki na hoście pozostaną nietknięte.","Remover {{workspace}} da lista? Os arquivos no host não são alterados.","Ta bort {{workspace}} från listan? Filerna på värden lämnas orörda.","要从列表中移除 {{workspace}} 吗？主机上的文件不会改动。"],
workspaceStatusConnected: ["Připojeno","Forbundet","Verbunden","Συνδεδεμένο","Connected","Conectado","Connecté","कनेक्टेड","Connesso","接続済み","Verbonden","Tilkoblet","Połączono","Conectado","Ansluten","已连接"],
workspaceStatusDisconnected: ["Nepřipojeno","Ikke forbundet","Nicht verbunden","Μη συνδεδεμένο","Not connected","Sin conexión","Non connecté","कनेक्ट नहीं है","Non connesso","未接続","Niet verbonden","Ikke tilkoblet","Nie połączono","Não conectado","Inte ansluten","未连接"],
workspaceStatusConnecting: ["Připojování…","Forbinder…","Verbindung wird aufgebaut…","Σύνδεση…","Connecting…","Conectando…","Connexion…","कनेक्ट हो रहा है…","Connessione…","接続中…","Verbinden…","Kobler til…","Łączenie…","Conectando…","Ansluter…","正在连接…"],
workspaceStatusLost: ["Spojení ztraceno","Forbindelsen er tabt","Verbindung verloren","Η σύνδεση χάθηκε","Connection lost","Conexión perdida","Connexion perdue","कनेक्शन टूट गया","Connessione persa","接続が切れました","Verbinding verbroken","Tilkoblingen ble brutt","Utracono połączenie","Conexão perdida","Anslutningen bröts","连接已断开"],
workspaceStatusError: ["Připojení selhalo","Forbindelsen mislykkedes","Verbindung fehlgeschlagen","Η σύνδεση απέτυχε","Connection failed","Error de conexión","Échec de la connexion","कनेक्शन विफल","Connessione non riuscita","接続に失敗しました","Verbinding mislukt","Tilkoblingen mislyktes","Połączenie nie powiodło się","Falha na conexão","Anslutningen misslyckades","连接失败"],
workspaceLost: ["Spojení s {{host}} bylo ztraceno","Forbindelsen til {{host}} er tabt","Verbindung zu {{host}} verloren","Η σύνδεση με το {{host}} χάθηκε","Connection to {{host}} lost","Se perdió la conexión con {{host}}","Connexion à {{host}} perdue","{{host}} से कनेक्शन टूट गया","Connessione a {{host}} persa","{{host}} との接続が切れました","Verbinding met {{host}} verbroken","Tilkoblingen til {{host}} ble brutt","Utracono połączenie z {{host}}","Conexão com {{host}} perdida","Anslutningen till {{host}} bröts","与 {{host}} 的连接已断开"],
workspaceRetryIn: ["Nový pokus za {{seconds}} s","Prøver igen om {{seconds}} s","Neuer Versuch in {{seconds}} s","Νέα προσπάθεια σε {{seconds}} s","Retrying in {{seconds}} s","Reintentando en {{seconds}} s","Nouvel essai dans {{seconds}} s","{{seconds}} s में फिर कोशिश","Nuovo tentativo tra {{seconds}} s","{{seconds}} 秒後に再試行","Nieuwe poging over {{seconds}} s","Prøver igjen om {{seconds}} s","Ponowna próba za {{seconds}} s","Nova tentativa em {{seconds}} s","Försöker igen om {{seconds}} s","{{seconds}} 秒后重试"],
workspaceReconnected: ["Znovu připojeno k {{host}}","Forbundet igen til {{host}}","Wieder verbunden mit {{host}}","Επανασυνδέθηκε με το {{host}}","Reconnected to {{host}}","Reconectado con {{host}}","Reconnecté à {{host}}","{{host}} से फिर कनेक्ट हो गया","Riconnesso a {{host}}","{{host}} に再接続しました","Opnieuw verbonden met {{host}}","Koblet til {{host}} igjen","Ponownie połączono z {{host}}","Reconectado a {{host}}","Ansluten igen till {{host}}","已重新连接到 {{host}}"],
workspaceUseLocal: ["Použít místní pracovní prostor","Brug lokalt arbejdsområde","Lokalen Arbeitsbereich nutzen","Χρήση τοπικού χώρου εργασίας","Use local workspace","Usar espacio local","Utiliser l'espace local","लोकल वर्कस्पेस इस्तेमाल करें","Usa area di lavoro locale","ローカルワークスペースを使う","Lokale werkruimte gebruiken","Bruk lokalt arbeidsområde","Użyj lokalnego obszaru roboczego","Usar área de trabalho local","Använd lokal arbetsyta","使用本地工作区"],
workspaceReviewHostKey: ["Zkontrolovat klíč hostitele…","Gennemse værtsnøgle…","Host-Schlüssel prüfen…","Έλεγχος κλειδιού κεντρικού υπολογιστή…","Review host key…","Revisar clave de host…","Vérifier la clé d'hôte…","होस्ट कुंजी जाँचें…","Verifica chiave host…","ホストキーを確認…","Hostsleutel controleren…","Kontroller vertsnøkkel…","Sprawdź klucz hosta…","Verificar chave do host…","Granska värdnyckel…","检查主机密钥…"],
workspaceDirtyTitle: ["Neuložené změny","Ikke-gemte ændringer","Ungespeicherte Änderungen","Μη αποθηκευμένες αλλαγές","Unsaved changes","Cambios sin guardar","Modifications non enregistrées","बिना सहेजे बदलाव","Modifiche non salvate","未保存の変更","Niet-opgeslagen wijzigingen","Ulagrede endringer","Niezapisane zmiany","Alterações não salvas","Osparade ändringar","未保存的更改"],
workspaceDirtyPrompt: ["Počet souborů s neuloženými změnami: {{count}}. Uložit je před přepnutím na {{workspace}}?","{{count}} filer har ikke-gemte ændringer. Gem dem, før du skifter til {{workspace}}?","{{count}} Dateien haben ungespeicherte Änderungen. Vor dem Wechsel zu {{workspace}} speichern?","{{count}} αρχεία έχουν μη αποθηκευμένες αλλαγές. Αποθήκευση πριν από τη μετάβαση στο {{workspace}};","{{count}} files have unsaved changes. Save them before switching to {{workspace}}?","{{count}} archivos tienen cambios sin guardar. ¿Guardarlos antes de cambiar a {{workspace}}?","{{count}} fichiers ont des modifications non enregistrées. Les enregistrer avant de passer à {{workspace}} ?","{{count}} फ़ाइलों में बिना सहेजे बदलाव हैं। {{workspace}} पर जाने से पहले सहेजें?","{{count}} file hanno modifiche non salvate. Salvarli prima di passare a {{workspace}}?","{{count}} 個のファイルに未保存の変更があります。{{workspace}} に切り替える前に保存しますか？","{{count}} bestanden hebben niet-opgeslagen wijzigingen. Opslaan voordat je naar {{workspace}} wisselt?","{{count}} filer har ulagrede endringer. Lagre dem før du bytter til {{workspace}}?","Pliki z niezapisanymi zmianami: {{count}}. Zapisać je przed przełączeniem na {{workspace}}?","{{count}} arquivos têm alterações não salvas. Salvá-los antes de mudar para {{workspace}}?","{{count}} filer har osparade ändringar. Spara dem innan du byter till {{workspace}}?","{{count}} 个文件有未保存的更改。切换到 {{workspace}} 之前要保存吗？"],
workspaceSaveFailedToast: ["{{file}} nebyl uložen. {{error}} Změny zůstávají na kartě.","{{file}} blev ikke gemt. {{error}} Dine ændringer bliver i fanen.","{{file}} wurde nicht gespeichert. {{error}} Deine Änderungen bleiben im Tab.","Το {{file}} δεν αποθηκεύτηκε. {{error}} Οι αλλαγές σας παραμένουν στην καρτέλα.","{{file}} was not saved. {{error}} Your changes stay in the tab.","{{file}} no se guardó. {{error}} Tus cambios siguen en la pestaña.","{{file}} n'a pas été enregistré. {{error}} Vos modifications restent dans l'onglet.","{{file}} सहेजी नहीं गई। {{error}} आपके बदलाव टैब में बने रहेंगे।","{{file}} non è stato salvato. {{error}} Le modifiche restano nella scheda.","{{file}} は保存されませんでした。{{error}} 変更はタブに残っています。","{{file}} is niet opgeslagen. {{error}} Je wijzigingen blijven in het tabblad.","{{file}} ble ikke lagret. {{error}} Endringene dine blir værende i fanen.","Nie zapisano {{file}}. {{error}} Twoje zmiany pozostają na karcie.","{{file}} não foi salvo. {{error}} Suas alterações continuam na aba.","{{file}} sparades inte. {{error}} Dina ändringar finns kvar i fliken.","{{file}} 未保存。{{error}} 你的更改仍保留在标签页中。"],
workspaceSwitchFailed: ["{{workspace}} nelze otevřít: {{error}}","Kunne ikke åbne {{workspace}}: {{error}}","{{workspace}} konnte nicht geöffnet werden: {{error}}","Δεν ήταν δυνατό το άνοιγμα του {{workspace}}: {{error}}","Could not open {{workspace}}: {{error}}","No se pudo abrir {{workspace}}: {{error}}","Impossible d'ouvrir {{workspace}} : {{error}}","{{workspace}} नहीं खुल सका: {{error}}","Impossibile aprire {{workspace}}: {{error}}","{{workspace}} を開けませんでした: {{error}}","Kan {{workspace}} niet openen: {{error}}","Kunne ikke åpne {{workspace}}: {{error}}","Nie można otworzyć {{workspace}}: {{error}}","Não foi possível abrir {{workspace}}: {{error}}","Kunde inte öppna {{workspace}}: {{error}}","无法打开 {{workspace}}：{{error}}"],
workspaceMissing: ["Tento pracovní prostor SSH už neexistuje. Code Studio přepnul na místní pracovní prostor.","Dette SSH-arbejdsområde findes ikke længere. Code Studio skiftede til det lokale arbejdsområde.","Dieser SSH-Arbeitsbereich existiert nicht mehr. Code Studio hat zum lokalen Arbeitsbereich gewechselt.","Αυτός ο χώρος εργασίας SSH δεν υπάρχει πια. Το Code Studio μετέβη στον τοπικό χώρο εργασίας.","This SSH workspace no longer exists. Code Studio switched to the local workspace.","Este espacio de trabajo SSH ya no existe. Code Studio cambió al espacio local.","Cet espace de travail SSH n'existe plus. Code Studio est revenu à l'espace local.","यह SSH वर्कस्पेस अब मौजूद नहीं है। Code Studio लोकल वर्कस्पेस पर आ गया।","Questa area di lavoro SSH non esiste più. Code Studio è passato all'area locale.","この SSH ワークスペースはもう存在しません。Code Studio はローカルワークスペースに切り替えました。","Deze SSH-werkruimte bestaat niet meer. Code Studio is overgeschakeld naar de lokale werkruimte.","Dette SSH-arbeidsområdet finnes ikke lenger. Code Studio byttet til det lokale arbeidsområdet.","Ten obszar roboczy SSH już nie istnieje. Code Studio przełączył się na lokalny obszar roboczy.","Esta área de trabalho SSH não existe mais. O Code Studio voltou para a área local.","Den här SSH-arbetsytan finns inte längre. Code Studio bytte till den lokala arbetsytan.","此 SSH 工作区已不存在。Code Studio 已切换到本地工作区。"],
workspaceGitMissing: ["Git není na {{host}} nainstalován","Git er ikke installeret på {{host}}","Git ist auf {{host}} nicht installiert","Το Git δεν είναι εγκατεστημένο στο {{host}}","Git is not installed on {{host}}","Git no está instalado en {{host}}","Git n'est pas installé sur {{host}}","{{host}} पर Git इंस्टॉल नहीं है","Git non è installato su {{host}}","{{host}} に Git がインストールされていません","Git is niet geïnstalleerd op {{host}}","Git er ikke installert på {{host}}","Git nie jest zainstalowany na {{host}}","O Git não está instalado em {{host}}","Git är inte installerat på {{host}}","{{host}} 上未安装 Git"],
workspaceLspNotInstalled: ["není nainstalován na {{host}}","ikke installeret på {{host}}","nicht installiert auf {{host}}","δεν είναι εγκατεστημένο στο {{host}}","not installed on {{host}}","no instalado en {{host}}","non installé sur {{host}}","{{host}} पर इंस्टॉल नहीं है","non installato su {{host}}","{{host}} に未インストール","niet geïnstalleerd op {{host}}","ikke installert på {{host}}","nie zainstalowano na {{host}}","não instalado em {{host}}","inte installerad på {{host}}","未安装在 {{host}} 上"],
workspaceDialogTitle: ["Připojit ke složce SSH","Forbind til SSH-mappe","Mit SSH-Ordner verbinden","Σύνδεση σε φάκελο SSH","Connect to SSH Folder","Conectar a carpeta SSH","Se connecter à un dossier SSH","SSH फ़ोल्डर से कनेक्ट करें","Connetti a cartella SSH","SSH フォルダーに接続","Verbinden met SSH-map","Koble til SSH-mappe","Połącz z folderem SSH","Conectar a pasta SSH","Anslut till SSH-mapp","连接到 SSH 文件夹"],
workspaceHosts: ["Hostitelé","Værter","Hosts","Κεντρικοί υπολογιστές","Hosts","Hosts","Hôtes","होस्ट","Host","ホスト","Hosts","Verter","Hosty","Hosts","Värdar","主机"],
workspaceSearchHosts: ["Hledat hostitele","Søg efter værter","Hosts durchsuchen","Αναζήτηση κεντρικών υπολογιστών","Search hosts","Buscar hosts","Rechercher des hôtes","होस्ट खोजें","Cerca host","ホストを検索","Hosts zoeken","Søk etter verter","Szukaj hostów","Pesquisar hosts","Sök värdar","搜索主机"],
workspaceNoHosts: ["V inventáři zatím nejsou žádní hostitelé SSH.","Ingen SSH-værter i inventaret endnu.","Noch keine SSH-Hosts im Inventar.","Δεν υπάρχουν ακόμη κεντρικοί υπολογιστές SSH στο απόθεμα.","No SSH hosts in the inventory yet.","Aún no hay hosts SSH en el inventario.","Aucun hôte SSH dans l'inventaire pour l'instant.","इन्वेंटरी में अभी कोई SSH होस्ट नहीं है।","Nessun host SSH nell'inventario.","インベントリに SSH ホストがまだありません。","Nog geen SSH-hosts in de inventaris.","Ingen SSH-verter i inventaret ennå.","W inwentarzu nie ma jeszcze hostów SSH.","Ainda não há hosts SSH no inventário.","Inga SSH-värdar i inventariet än.","清单中还没有 SSH 主机。"],
workspaceNoHostMatch: ["Výrazu „{{query}}“ neodpovídá žádný hostitel","Ingen vært matcher »{{query}}«","Kein Host passt zu „{{query}}“","Κανένας κεντρικός υπολογιστής δεν ταιριάζει με «{{query}}»","No host matches “{{query}}”","Ningún host coincide con «{{query}}»","Aucun hôte ne correspond à « {{query}} »","“{{query}}” से कोई होस्ट मेल नहीं खाता","Nessun host corrisponde a «{{query}}»","「{{query}}」に一致するホストはありません","Geen host komt overeen met ‘{{query}}’","Ingen vert samsvarer med «{{query}}»","Żaden host nie pasuje do „{{query}}”","Nenhum host corresponde a “{{query}}”","Ingen värd matchar ”{{query}}”","没有与“{{query}}”匹配的主机"],
workspaceNewHost: ["Nový hostitel SSH","Ny SSH-vært","Neuer SSH-Host","Νέος κεντρικός υπολογιστής SSH","New SSH Host","Nuevo host SSH","Nouvel hôte SSH","नया SSH होस्ट","Nuovo host SSH","新しい SSH ホスト","Nieuwe SSH-host","Ny SSH-vert","Nowy host SSH","Novo host SSH","Ny SSH-värd","新建 SSH 主机"],
workspaceNeverUsed: ["Nikdy nepoužito","Aldrig brugt","Nie verwendet","Δεν χρησιμοποιήθηκε ποτέ","Never used","Nunca usado","Jamais utilisé","कभी इस्तेमाल नहीं हुआ","Mai usato","未使用","Nooit gebruikt","Aldri brukt","Nigdy nie używany","Nunca usado","Aldrig använd","从未使用"],
workspacePickHost: ["Vyberte hostitele a zobrazí se jeho složky.","Vælg en vært for at se dens mapper.","Wähle einen Host, um seine Ordner zu sehen.","Επιλέξτε έναν κεντρικό υπολογιστή για να δείτε τους φακέλους του.","Choose a host to see its folders.","Elige un host para ver sus carpetas.","Choisissez un hôte pour voir ses dossiers.","फ़ोल्डर देखने के लिए एक होस्ट चुनें।","Scegli un host per vederne le cartelle.","ホストを選ぶとフォルダーが表示されます。","Kies een host om de mappen te zien.","Velg en vert for å se mappene.","Wybierz host, aby zobaczyć jego foldery.","Escolha um host para ver as pastas dele.","Välj en värd för att se dess mappar.","选择一台主机以查看其文件夹。"],
workspaceHome: ["Domovská složka","Hjemmemappe","Home-Ordner","Αρχικός φάκελος","Home folder","Carpeta personal","Dossier personnel","होम फ़ोल्डर","Cartella home","ホームフォルダー","Thuismap","Hjemmemappe","Folder domowy","Pasta pessoal","Hemmapp","主文件夹"],
workspaceUp: ["Nadřazená složka","Overordnet mappe","Übergeordneter Ordner","Γονικός φάκελος","Parent folder","Carpeta superior","Dossier parent","ऊपर का फ़ोल्डर","Cartella superiore","親フォルダー","Bovenliggende map","Overordnet mappe","Folder nadrzędny","Pasta acima","Överordnad mapp","上级文件夹"],
workspacePathLabel: ["Cesta ke složce","Mappesti","Ordnerpfad","Διαδρομή φακέλου","Folder path","Ruta de la carpeta","Chemin du dossier","फ़ोल्डर पाथ","Percorso della cartella","フォルダーのパス","Mappad","Mappebane","Ścieżka folderu","Caminho da pasta","Mappsökväg","文件夹路径"],
workspaceNoSubfolders: ["Tato složka nemá podsložky.","Denne mappe har ingen undermapper.","Dieser Ordner hat keine Unterordner.","Αυτός ο φάκελος δεν έχει υποφακέλους.","This folder has no subfolders.","Esta carpeta no tiene subcarpetas.","Ce dossier n'a pas de sous-dossiers.","इस फ़ोल्डर में कोई सबफ़ोल्डर नहीं है।","Questa cartella non ha sottocartelle.","このフォルダーにはサブフォルダーがありません。","Deze map heeft geen submappen.","Denne mappen har ingen undermapper.","Ten folder nie ma podfolderów.","Esta pasta não tem subpastas.","Den här mappen har inga undermappar.","此文件夹没有子文件夹。"],
workspaceKeyHint: ["Enter otevře · → vstoupí do složky · ← o úroveň výš","Enter åbner · → går ind i mappen · ← går et niveau op","Enter öffnet · → in den Ordner · ← eine Ebene höher","Enter ανοίγει · → μπαίνει στον φάκελο · ← ένα επίπεδο πάνω","Enter opens · → enters a folder · ← goes up","Intro abre · → entra en una carpeta · ← sube un nivel","Entrée ouvre · → entre dans un dossier · ← remonte","Enter खोलता है · → फ़ोल्डर में जाता है · ← एक स्तर ऊपर","Invio apre · → entra nella cartella · ← sale di un livello","Enter で開く · → でフォルダーに入る · ← で上へ","Enter opent · → map in · ← een niveau omhoog","Enter åpner · → går inn i mappen · ← går et nivå opp","Enter otwiera · → wchodzi do folderu · ← poziom wyżej","Enter abre · → entra na pasta · ← sobe um nível","Enter öppnar · → går in i mappen · ← går upp en nivå","Enter 打开 · → 进入文件夹 · ← 返回上级"],
workspaceOpenFolder: ["Otevřít složku","Åbn mappe","Ordner öffnen","Άνοιγμα φακέλου","Open Folder","Abrir carpeta","Ouvrir le dossier","फ़ोल्डर खोलें","Apri cartella","フォルダーを開く","Map openen","Åpne mappe","Otwórz folder","Abrir pasta","Öppna mapp","打开文件夹"],
workspaceTrustTitle: ["Neznámý klíč hostitele","Ukendt værtsnøgle","Unbekannter Host-Schlüssel","Άγνωστο κλειδί κεντρικού υπολογιστή","Unknown host key","Clave de host desconocida","Clé d'hôte inconnue","अज्ञात होस्ट कुंजी","Chiave host sconosciuta","不明なホストキー","Onbekende hostsleutel","Ukjent vertsnøkkel","Nieznany klucz hosta","Chave de host desconhecida","Okänd värdnyckel","未知的主机密钥"],
workspaceTrustText: ["Code Studio se k {{host}} ještě nikdy nepřipojil. Než mu budete důvěřovat, porovnejte otisk s tím, který zobrazuje server.","Code Studio har aldrig forbundet til {{host}} før. Sammenlign fingeraftrykket med det, serveren viser, før du stoler på den.","Code Studio war noch nie mit {{host}} verbunden. Vergleiche den Fingerabdruck mit dem, den der Server anzeigt, bevor du ihm vertraust.","Το Code Studio δεν έχει συνδεθεί ποτέ στο {{host}}. Συγκρίνετε το αποτύπωμα με αυτό που δείχνει ο διακομιστής πριν τον εμπιστευτείτε.","Code Studio has not connected to {{host}} before. Compare the fingerprint with the one the server shows before you trust it.","Code Studio nunca se ha conectado a {{host}}. Compara la huella con la que muestra el servidor antes de confiar en él.","Code Studio ne s'est jamais connecté à {{host}}. Comparez l'empreinte avec celle affichée par le serveur avant de lui faire confiance.","Code Studio पहले कभी {{host}} से कनेक्ट नहीं हुआ। भरोसा करने से पहले फ़िंगरप्रिंट को सर्वर पर दिखने वाले फ़िंगरप्रिंट से मिलाएँ।","Code Studio non si è mai connesso a {{host}}. Confronta l'impronta con quella mostrata dal server prima di fidarti.","Code Studio はまだ {{host}} に接続したことがありません。信頼する前に、サーバーが表示するフィンガープリントと照合してください。","Code Studio heeft nog nooit verbinding gemaakt met {{host}}. Vergelijk de vingerafdruk met die van de server voordat je hem vertrouwt.","Code Studio har aldri koblet til {{host}} før. Sammenlign fingeravtrykket med det serveren viser før du stoler på den.","Code Studio nigdy wcześniej nie łączył się z {{host}}. Zanim mu zaufasz, porównaj odcisk z tym, który pokazuje serwer.","O Code Studio nunca se conectou a {{host}}. Compare a impressão digital com a que o servidor mostra antes de confiar nele.","Code Studio har aldrig anslutit till {{host}} tidigare. Jämför fingeravtrycket med det som servern visar innan du litar på den.","Code Studio 从未连接过 {{host}}。在信任之前，请将指纹与服务器显示的指纹进行比对。"],
workspaceTrustKeyType: ["Typ klíče","Nøgletype","Schlüsseltyp","Τύπος κλειδιού","Key type","Tipo de clave","Type de clé","कुंजी प्रकार","Tipo di chiave","キーの種類","Sleuteltype","Nøkkeltype","Typ klucza","Tipo de chave","Nyckeltyp","密钥类型"],
workspaceTrustFingerprint: ["Otisk","Fingeraftryk","Fingerabdruck","Αποτύπωμα","Fingerprint","Huella","Empreinte","फ़िंगरप्रिंट","Impronta","フィンガープリント","Vingerafdruk","Fingeravtrykk","Odcisk","Impressão digital","Fingeravtryck","指纹"],
workspaceTrustContinue: ["Důvěřovat a pokračovat","Stol på og fortsæt","Vertrauen und fortfahren","Εμπιστοσύνη και συνέχεια","Trust and Continue","Confiar y continuar","Faire confiance et continuer","भरोसा करें और जारी रखें","Fidati e continua","信頼して続行","Vertrouwen en doorgaan","Stol på og fortsett","Zaufaj i kontynuuj","Confiar e continuar","Lita på och fortsätt","信任并继续"],
workspaceMismatchTitle: ["Klíč hostitele se změnil","Værtsnøglen er ændret","Host-Schlüssel geändert","Το κλειδί του κεντρικού υπολογιστή άλλαξε","Host key changed","La clave de host cambió","Clé d'hôte modifiée","होस्ट कुंजी बदल गई","Chiave host cambiata","ホストキーが変更されました","Hostsleutel gewijzigd","Vertsnøkkelen er endret","Klucz hosta się zmienił","Chave do host alterada","Värdnyckeln har ändrats","主机密钥已更改"],
workspaceErrorHostKeyMismatch: ["Klíč hostitele {{host}} se změnil. Code Studio se nepřipojí, dokud nebude known_hosts opraven.","Værtsnøglen for {{host}} er ændret. Code Studio forbinder ikke, før known_hosts er rettet.","Der Host-Schlüssel von {{host}} hat sich geändert. Code Studio verbindet sich erst wieder, wenn known_hosts korrigiert ist.","Το κλειδί του κεντρικού υπολογιστή {{host}} άλλαξε. Το Code Studio δεν συνδέεται μέχρι να διορθωθεί το known_hosts.","The host key of {{host}} has changed. Code Studio does not connect until known_hosts is fixed.","La clave de host de {{host}} ha cambiado. Code Studio no se conectará hasta que se corrija known_hosts.","La clé d'hôte de {{host}} a changé. Code Studio ne se connecte plus tant que known_hosts n'est pas corrigé.","{{host}} की होस्ट कुंजी बदल गई है। known_hosts ठीक होने तक Code Studio कनेक्ट नहीं करेगा।","La chiave host di {{host}} è cambiata. Code Studio non si connette finché known_hosts non viene corretto.","{{host}} のホストキーが変わりました。known_hosts を修正するまで Code Studio は接続しません。","De hostsleutel van {{host}} is gewijzigd. Code Studio maakt pas weer verbinding als known_hosts is gecorrigeerd.","Vertsnøkkelen for {{host}} er endret. Code Studio kobler ikke til før known_hosts er rettet.","Klucz hosta {{host}} się zmienił. Code Studio nie połączy się, dopóki known_hosts nie zostanie poprawiony.","A chave do host {{host}} mudou. O Code Studio não se conecta até que o known_hosts seja corrigido.","Värdnyckeln för {{host}} har ändrats. Code Studio ansluter inte förrän known_hosts är rättad.","{{host}} 的主机密钥已更改。在修复 known_hosts 之前，Code Studio 不会连接。"],
workspaceErrorAuth: ["Přihlášení k {{host}} selhalo. Zkontrolujte uživatelské jméno a heslo nebo klíč.","Login på {{host}} mislykkedes. Kontrollér brugernavnet og adgangskoden eller nøglen.","Anmeldung bei {{host}} fehlgeschlagen. Prüfe Benutzername und Passwort oder Schlüssel.","Η σύνδεση στο {{host}} απέτυχε. Ελέγξτε το όνομα χρήστη και τον κωδικό ή το κλειδί.","Sign-in to {{host}} failed. Check the user name and the password or key.","Falló el inicio de sesión en {{host}}. Revisa el usuario y la contraseña o la clave.","Échec de la connexion à {{host}}. Vérifiez le nom d'utilisateur et le mot de passe ou la clé.","{{host}} पर साइन-इन विफल। उपयोगकर्ता नाम और पासवर्ड या कुंजी जाँचें।","Accesso a {{host}} non riuscito. Controlla nome utente e password o chiave.","{{host}} へのサインインに失敗しました。ユーザー名とパスワードまたはキーを確認してください。","Aanmelden bij {{host}} mislukt. Controleer de gebruikersnaam en het wachtwoord of de sleutel.","Innlogging på {{host}} mislyktes. Kontroller brukernavnet og passordet eller nøkkelen.","Logowanie do {{host}} nie powiodło się. Sprawdź nazwę użytkownika oraz hasło lub klucz.","Falha ao entrar em {{host}}. Verifique o usuário e a senha ou a chave.","Inloggningen på {{host}} misslyckades. Kontrollera användarnamnet och lösenordet eller nyckeln.","登录 {{host}} 失败。请检查用户名以及密码或密钥。"],
workspaceErrorUnreachable: ["{{host}} není dostupný.","{{host}} kan ikke nås.","{{host}} ist nicht erreichbar.","Το {{host}} δεν είναι προσβάσιμο.","{{host}} cannot be reached.","No se puede acceder a {{host}}.","{{host}} est injoignable.","{{host}} तक पहुँचा नहीं जा सकता।","{{host}} non è raggiungibile.","{{host}} に到達できません。","{{host}} is niet bereikbaar.","{{host}} kan ikke nås.","{{host}} jest nieosiągalny.","{{host}} não está acessível.","{{host}} går inte att nå.","无法访问 {{host}}。"],
workspaceErrorNotDirectory: ["{{path}} není složka.","{{path}} er ikke en mappe.","{{path}} ist kein Ordner.","Το {{path}} δεν είναι φάκελος.","{{path}} is not a folder.","{{path}} no es una carpeta.","{{path}} n'est pas un dossier.","{{path}} फ़ोल्डर नहीं है।","{{path}} non è una cartella.","{{path}} はフォルダーではありません。","{{path}} is geen map.","{{path}} er ikke en mappe.","{{path}} nie jest folderem.","{{path}} não é uma pasta.","{{path}} är inte en mapp.","{{path}} 不是文件夹。"],
workspaceErrorNotFound: ["{{path}} neexistuje.","{{path}} findes ikke.","{{path}} existiert nicht.","Το {{path}} δεν υπάρχει.","{{path}} does not exist.","{{path}} no existe.","{{path}} n'existe pas.","{{path}} मौजूद नहीं है।","{{path}} non esiste.","{{path}} は存在しません。","{{path}} bestaat niet.","{{path}} finnes ikke.","{{path}} nie istnieje.","{{path}} não existe.","{{path}} finns inte.","{{path}} 不存在。"],
workspaceErrorOther: ["Něco se pokazilo: {{error}}","Noget gik galt: {{error}}","Etwas ist schiefgelaufen: {{error}}","Κάτι πήγε στραβά: {{error}}","Something went wrong: {{error}}","Algo salió mal: {{error}}","Une erreur s'est produite : {{error}}","कुछ गड़बड़ हो गई: {{error}}","Qualcosa è andato storto: {{error}}","問題が発生しました: {{error}}","Er ging iets mis: {{error}}","Noe gikk galt: {{error}}","Coś poszło nie tak: {{error}}","Algo deu errado: {{error}}","Något gick fel: {{error}}","出现问题：{{error}}"],
workspaceChooseOtherHost: ["Vybrat jiného hostitele","Vælg en anden vært","Anderen Host wählen","Επιλογή άλλου κεντρικού υπολογιστή","Choose Another Host","Elegir otro host","Choisir un autre hôte","दूसरा होस्ट चुनें","Scegli un altro host","別のホストを選ぶ","Andere host kiezen","Velg en annen vert","Wybierz inny host","Escolher outro host","Välj en annan värd","选择其他主机"],
workspaceFormName: ["Název","Navn","Name","Όνομα","Name","Nombre","Nom","नाम","Nome","名前","Naam","Navn","Nazwa","Nome","Namn","名称"],
workspaceFormHost: ["Hostitel","Vært","Host","Κεντρικός υπολογιστής","Host","Host","Hôte","होस्ट","Host","ホスト","Host","Vert","Host","Host","Värd","主机"],
workspaceFormPort: ["Port","Port","Port","Θύρα","Port","Puerto","Port","पोर्ट","Porta","ポート","Poort","Port","Port","Porta","Port","端口"],
workspaceFormUser: ["Uživatel","Bruger","Benutzer","Χρήστης","User","Usuario","Utilisateur","उपयोगकर्ता","Utente","ユーザー","Gebruiker","Bruker","Użytkownik","Usuário","Användare","用户"],
workspaceFormAuth: ["Přihlásit pomocí","Log ind med","Anmelden mit","Σύνδεση με","Sign in with","Iniciar sesión con","Se connecter avec","इससे साइन इन करें","Accedi con","サインイン方法","Aanmelden met","Logg inn med","Zaloguj za pomocą","Entrar com","Logga in med","登录方式"],
workspaceFormPassword: ["Heslo","Adgangskode","Passwort","Κωδικός πρόσβασης","Password","Contraseña","Mot de passe","पासवर्ड","Password","パスワード","Wachtwoord","Passord","Hasło","Senha","Lösenord","密码"],
workspaceFormKey: ["Soukromý klíč","Privat nøgle","Privater Schlüssel","Ιδιωτικό κλειδί","Private key","Clave privada","Clé privée","प्राइवेट कुंजी","Chiave privata","秘密鍵","Privésleutel","Privat nøkkel","Klucz prywatny","Chave privada","Privat nyckel","私钥"],
workspaceFormKeyFile: ["Načíst soubor s klíčem…","Indlæs nøglefil…","Schlüsseldatei laden…","Φόρτωση αρχείου κλειδιού…","Load Key File…","Cargar archivo de clave…","Charger un fichier de clé…","कुंजी फ़ाइल लोड करें…","Carica file chiave…","鍵ファイルを読み込む…","Sleutelbestand laden…","Last inn nøkkelfil…","Wczytaj plik klucza…","Carregar arquivo de chave…","Läs in nyckelfil…","加载密钥文件…"],
workspaceFormPassphrase: ["Heslová fráze (volitelné)","Adgangssætning (valgfri)","Passphrase (optional)","Φράση πρόσβασης (προαιρετικά)","Passphrase (optional)","Frase de contraseña (opcional)","Phrase secrète (facultatif)","पासफ़्रेज़ (वैकल्पिक)","Passphrase (facoltativa)","パスフレーズ（任意）","Wachtwoordzin (optioneel)","Passfrase (valgfritt)","Hasło klucza (opcjonalnie)","Frase secreta (opcional)","Lösenfras (valfritt)","密码短语（可选）"],
workspaceFormVaultHint: ["Tajemství se uloží do trezoru AuraGo a v prohlížeči nikdy nezůstane.","Hemmeligheden gemmes i AuraGo-boksen og opbevares aldrig i browseren.","Das Geheimnis wird im AuraGo-Tresor gespeichert und nie im Browser behalten.","Το μυστικό αποθηκεύεται στο θησαυροφυλάκιο του AuraGo και δεν μένει ποτέ στο πρόγραμμα περιήγησης.","The secret is stored in the AuraGo vault and never kept in the browser.","El secreto se guarda en la bóveda de AuraGo y nunca se conserva en el navegador.","Le secret est stocké dans le coffre AuraGo et n'est jamais conservé dans le navigateur.","सीक्रेट AuraGo वॉल्ट में सहेजा जाता है और ब्राउज़र में कभी नहीं रहता।","Il segreto viene salvato nel vault di AuraGo e non resta mai nel browser.","シークレットは AuraGo の保管庫に保存され、ブラウザーには残りません。","Het geheim wordt in de AuraGo-kluis opgeslagen en nooit in de browser bewaard.","Hemmeligheten lagres i AuraGo-hvelvet og beholdes aldri i nettleseren.","Sekret jest przechowywany w sejfie AuraGo i nigdy nie zostaje w przeglądarce.","O segredo fica guardado no cofre do AuraGo e nunca é mantido no navegador.","Hemligheten sparas i AuraGos valv och behålls aldrig i webbläsaren.","密钥信息保存在 AuraGo 保险库中，绝不会保留在浏览器里。"],
workspaceFormAdd: ["Přidat hostitele","Tilføj vært","Host hinzufügen","Προσθήκη κεντρικού υπολογιστή","Add Host","Añadir host","Ajouter l'hôte","होस्ट जोड़ें","Aggiungi host","ホストを追加","Host toevoegen","Legg til vert","Dodaj host","Adicionar host","Lägg till värd","添加主机"],
workspaceFormRequired: ["Povinné","Påkrævet","Pflichtfeld","Υποχρεωτικό","Required","Obligatorio","Obligatoire","ज़रूरी","Obbligatorio","必須","Verplicht","Påkrevd","Wymagane","Obrigatório","Obligatoriskt","必填"],
workspaceFormBadHost: ["Zadejte název hostitele nebo IP adresu bez uživatele a schématu","Angiv et værtsnavn eller en IP-adresse uden bruger eller skema","Hostname oder IP-Adresse ohne Benutzer oder Schema eingeben","Πληκτρολογήστε όνομα κεντρικού υπολογιστή ή διεύθυνση IP, χωρίς χρήστη ή σχήμα","Enter a host name or IP address, without user or scheme","Escribe un nombre de host o una IP, sin usuario ni esquema","Saisissez un nom d'hôte ou une adresse IP, sans utilisateur ni schéma","बिना उपयोगकर्ता या स्कीम के होस्ट नाम या IP पता दर्ज करें","Inserisci un nome host o un indirizzo IP, senza utente né schema","ユーザーやスキームを含めずにホスト名か IP アドレスを入力してください","Voer een hostnaam of IP-adres in, zonder gebruiker of schema","Skriv inn et vertsnavn eller en IP-adresse uten bruker eller skjema","Podaj nazwę hosta lub adres IP, bez użytkownika i schematu","Informe um nome de host ou endereço IP, sem usuário nem esquema","Ange ett värdnamn eller en IP-adress utan användare eller schema","请输入主机名或 IP 地址，不要包含用户或协议"],
workspaceFormBadPort: ["Port 1–65535","Port 1–65535","Port 1–65535","Θύρα 1–65535","Port 1–65535","Puerto 1–65535","Port 1–65535","पोर्ट 1–65535","Porta 1–65535","ポート 1–65535","Poort 1–65535","Port 1–65535","Port 1–65535","Porta 1–65535","Port 1–65535","端口 1–65535"],
workspaceFormBadKey: ["Vložte soukromý klíč, který začíná -----BEGIN","Indsæt en privat nøgle, der starter med -----BEGIN","Füge einen privaten Schlüssel ein, der mit -----BEGIN beginnt","Επικολλήστε ιδιωτικό κλειδί που ξεκινά με -----BEGIN","Paste a private key that starts with -----BEGIN","Pega una clave privada que empiece por -----BEGIN","Collez une clé privée qui commence par -----BEGIN","-----BEGIN से शुरू होने वाली प्राइवेट कुंजी पेस्ट करें","Incolla una chiave privata che inizia con -----BEGIN","-----BEGIN で始まる秘密鍵を貼り付けてください","Plak een privésleutel die begint met -----BEGIN","Lim inn en privat nøkkel som begynner med -----BEGIN","Wklej klucz prywatny zaczynający się od -----BEGIN","Cole uma chave privada que comece com -----BEGIN","Klistra in en privat nyckel som börjar med -----BEGIN","请粘贴以 -----BEGIN 开头的私钥"],
workspaceFormReadOnly: ["Plocha je jen pro čtení: nové hostitele nelze přidat.","Skrivebordet er skrivebeskyttet: nye værter kan ikke tilføjes.","Der Desktop ist schreibgeschützt: Es können keine neuen Hosts angelegt werden.","Η επιφάνεια εργασίας είναι μόνο για ανάγνωση: δεν μπορούν να προστεθούν νέοι κεντρικοί υπολογιστές.","The desktop is read-only: new hosts cannot be added.","El escritorio es de solo lectura: no se pueden añadir hosts nuevos.","Le bureau est en lecture seule : impossible d'ajouter de nouveaux hôtes.","डेस्कटॉप केवल-पढ़ने के लिए है: नए होस्ट नहीं जोड़े जा सकते।","Il desktop è in sola lettura: non è possibile aggiungere nuovi host.","デスクトップは読み取り専用です。新しいホストは追加できません。","Het bureaublad is alleen-lezen: er kunnen geen nieuwe hosts worden toegevoegd.","Skrivebordet er skrivebeskyttet: nye verter kan ikke legges til.","Pulpit jest tylko do odczytu: nie można dodawać nowych hostów.","O desktop está em modo somente leitura: não é possível adicionar novos hosts.","Skrivbordet är skrivskyddat: nya värdar kan inte läggas till.","桌面为只读模式：无法添加新主机。"],
workspaceFormSaveFailed: ["Hostitele nelze přidat: {{error}}","Kunne ikke tilføje værten: {{error}}","Host konnte nicht hinzugefügt werden: {{error}}","Δεν ήταν δυνατή η προσθήκη του κεντρικού υπολογιστή: {{error}}","Could not add the host: {{error}}","No se pudo añadir el host: {{error}}","Impossible d'ajouter l'hôte : {{error}}","होस्ट नहीं जोड़ा जा सका: {{error}}","Impossibile aggiungere l'host: {{error}}","ホストを追加できませんでした: {{error}}","Kan de host niet toevoegen: {{error}}","Kunne ikke legge til verten: {{error}}","Nie można dodać hosta: {{error}}","Não foi possível adicionar o host: {{error}}","Kunde inte lägga till värden: {{error}}","无法添加主机：{{error}}"]
};

const placeholders = value => (value.match(/\{\{\w+\}\}/g) || []).sort().join(',');
const formalGerman = [' Sie ', ' Ihre ', ' Ihnen '];
for (const [key, values] of Object.entries(S)) {
    if (!Array.isArray(values) || values.length !== langs.length) throw new Error(`${key}: needs ${langs.length} translations`);
}
for (const [li, lang] of langs.entries()) {
    const file = path.join(repo, 'ui', 'lang', 'desktop', lang + '.json');
    const text = await readFile(file, 'utf8');
    const eol = text.includes('\r\n') ? '\r\n' : '\n';
    const existing = JSON.parse(text);
    const lines = text.split(eol);
    const index = lines.findIndex(line => line.trimStart().startsWith(anchorKey));
    if (index < 0) throw new Error(`${lang}: anchor not found`);
    if (!lines[index].trimEnd().endsWith(',')) throw new Error(`${lang}: anchor is the last entry`);
    const indent = lines[index].match(/^\s*/)[0];
    const added = [];
    for (const [short, values] of Object.entries(S)) {
        const key = 'codeStudio.' + short;
        const value = values[li];
        if (typeof value !== 'string' || !value.trim()) throw new Error(`${lang}: ${key} empty`);
        if (placeholders(value) !== placeholders(values[4])) throw new Error(`${lang}: ${key} placeholders differ from English`);
        if (lang === 'de' && formalGerman.some(word => (' ' + value + ' ').includes(word))) throw new Error(`de: ${key} uses formal address`);
        if (Object.prototype.hasOwnProperty.call(existing, key)) {
            if (existing[key] !== value) throw new Error(`${lang}: ${key} exists with another text`);
            continue;
        }
        added.push(`${indent}${JSON.stringify(key)}: ${JSON.stringify(value)},`);
    }
    if (added.length) {
        lines.splice(index + 1, 0, ...added);
        const next = lines.join(eol);
        const parsed = JSON.parse(next);
        for (const [short, values] of Object.entries(S)) if (parsed['codeStudio.' + short] !== values[li]) throw new Error(`${lang}: ${short} did not land`);
        await writeFile(file, next, 'utf8');
    }
    console.log(`${lang}: added ${added.length} keys`);
}
```

- [ ] **Step 3:** Run `node C:\disposable\code-studio-plan-c\locales-c.mjs C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\cs-ssh-preview`.
  Expected: 16 lines `<lang>: added 72 keys`. A second run prints `added 0 keys` for every file.
- [ ] **Step 4:** Run `go test ./ui/ -run '(?i)CodeStudioShellI18n' -count=1`.
  Expected: `ok`.
- [ ] **Step 5:** Run detect-changes.
- [ ] **Step 6:** Commit with the message `feat(code-studio): strings for SSH workspaces in 16 languages`. The body says: 72 codeStudio.workspace* strings, inserted after codeStudio.workspaceRoot. Commit all 16 `ui/lang/desktop/*.json` paths.

---

### Task 4: Foundation — `workspace.js` part A, bundle registration, core request and session hooks

**Files:**
- Create: `ui/js/desktop/apps/code-studio/workspace.js`
- Modify: `scripts/build-ui-bundles.js`, `ui/js/desktop/apps/code-studio/core.js`

- [ ] **Step 1: Impact.** Run impact for `api`, `saveState`, `restoreTabs`, `saveTab`, `downloadFile`, `uploadFile`, `showOperationError` and `createInstance`.
  - `api`: only `apiClient` in core.js.
  - `saveState`: many callers (core, panels, sidebar, settings); its behaviour stays compatible.
  - `restoreTabs`: only `render`.
  - `saveTab`: `saveTabWithActions`, `saveAllFiles`, the session action.

  Report the risk.
- [ ] **Step 2:** Create `workspace.js` with part A:

```js
    // Plan C (SSH workspaces): switcher, remote chip, connect dialog, per-workspace sessions, switching, disconnect
    // banner and capability gating. Pure helpers: workspace-model.js (window.CodeStudioWorkspaceModel). This fragment
    // shares the Code Studio IIFE and only declares functions and constants; nothing runs before render().
    // Contract for plans B/D: state.workspaceId, activeWorkspace(), workspaceIsRemote(), workspaceHostLabel(),
    // workspaceHas(name), workspaceURL(url), workspaceSocketURL(path, instance).

    const WS_MODEL = window.CodeStudioWorkspaceModel;
    const WS_KEY_FILE_LIMIT = 64 * 1024;
    const WS_GLYPHS = {
        laptop: '<rect x="3" y="3.5" width="10" height="7" rx="1.2"/><path d="M1.5 12.5h13"/>',
        server: '<rect x="2.5" y="2.5" width="11" height="4.5" rx="1.2"/><rect x="2.5" y="9" width="11" height="4.5" rx="1.2"/><path d="M5 4.75h.01M5 11.25h.01"/>',
        chevron: '<path d="M4.5 6.5 8 10l3.5-3.5"/>',
        up: '<path d="M8 13V3.5M4 7.5 8 3.5l4 4"/>',
        home: '<path d="M2.5 7.5 8 3l5.5 4.5V13a.5.5 0 0 1-.5.5H9.5V10h-3v3.5H3a.5.5 0 0 1-.5-.5z"/>',
        plus: '<path d="M8 3v10M3 8h10"/>',
        plug: '<path d="M6 2.5v3M10 2.5v3M4.5 5.5h7v2a3.5 3.5 0 0 1-7 0zM8 11v2.5"/>',
        unplug: '<path d="M2.5 2.5l11 11M10 2.5v3M8 11v2.5M4.5 5.5v2a3.5 3.5 0 0 0 3.5 3.5"/>',
        folder: '<path d="M2 4.5A1.5 1.5 0 0 1 3.5 3h2.6l1.4 1.5h5A1.5 1.5 0 0 1 14 6v5.5a1.5 1.5 0 0 1-1.5 1.5h-9A1.5 1.5 0 0 1 2 11.5z"/>',
        file: '<path d="M4 2.5h5l3 3v8H4z"/><path d="M9 2.5v3h3"/>',
        key: '<circle cx="5.5" cy="10.5" r="3"/><path d="M7.6 8.4 13.5 2.5M11 5l1.5 1.5"/>',
        warning: '<path d="M8 2.5 14 13H2z"/><path d="M8 6.5v3M8 11.25h.01"/>',
        x: '<path d="M4 4l8 8M12 4l-8 8"/>',
        check: '<path d="M3 8.5 6.5 12 13 4.5"/>',
        refresh: '<path d="M13 8a5 5 0 1 1-1.5-3.5M13 2.5v3h-3"/>'
    };

    function wsGlyph(name, size) {
        const px = Number(size || 14);
        return `<svg class="cs-ws-glyph" viewBox="0 0 16 16" width="${px}" height="${px}" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">${WS_GLYPHS[name] || ''}</svg>`;
    }

    function workspaceIsRemote(instance = state) { return !!instance && WS_MODEL.isRemote(instance.workspaceId); }
    function activeWorkspace(instance = state) { return (instance && instance.workspace) || WS_MODEL.localWorkspace(); }
    function workspaceHostLabel(ws = activeWorkspace()) { return WS_MODEL.userAtHost(ws); }
    function workspaceDisplayLabel(ws = activeWorkspace()) {
        return ws && ws.kind === 'ssh' ? WS_MODEL.workspaceLabel(ws) : tr('codeStudio.workspaceLocal', 'Local workspace');
    }
    function workspaceURL(url, instance = state || currentInstance()) {
        return WS_MODEL.withWorkspaceQuery(url, instance ? instance.workspaceId : WS_MODEL.LOCAL_ID);
    }
    function workspaceSocketURL(path, instance = state || currentInstance()) {
        return WS_MODEL.socketURL(location, path, instance ? instance.workspaceId : WS_MODEL.LOCAL_ID);
    }
    // Capabilities gate a feature only when the host reported them; unknown means available.
    function workspaceHas(name, instance = state) {
        if (!workspaceIsRemote(instance)) return true;
        const caps = activeWorkspace(instance).capabilities;
        return !caps || caps[name] !== false;
    }
    function workspaceChipState(instance = state) {
        if (!instance) return 'local';
        if (instance.workspaceSwitchTarget) return 'connecting';
        return workspaceIsRemote(instance) ? (instance.workspaceStatus || 'connected') : 'local';
    }
    function workspaceStatusText(status) {
        switch (status) {
            case 'connected': return tr('codeStudio.workspaceStatusConnected', 'Connected');
            case 'connecting': return tr('codeStudio.workspaceStatusConnecting', 'Connecting…');
            case 'lost': return tr('codeStudio.workspaceStatusLost', 'Connection lost');
            case 'hostkey': return tr('codeStudio.workspaceTrustTitle', 'Unknown host key');
            case 'error': return tr('codeStudio.workspaceStatusError', 'Connection failed');
            case 'disconnected': return tr('codeStudio.workspaceStatusDisconnected', 'Not connected');
            default: return tr('codeStudio.workspaceLocal', 'Local workspace');
        }
    }
    function workspaceDotState(ws) {
        if (state && ws.id === state.workspaceId) return workspaceChipState();
        return ws.connected ? 'connected' : 'disconnected';
    }
    function workspaceErrorText(kind, details) {
        const vars = Object.assign({ host: workspaceHostLabel(), path: '', error: '' }, details || {});
        switch (kind) {
            case 'host_key_mismatch': return tr('codeStudio.workspaceErrorHostKeyMismatch', 'The host key of {{host}} has changed. Code Studio does not connect until known_hosts is fixed.', vars);
            case 'auth': return tr('codeStudio.workspaceErrorAuth', 'Sign-in to {{host}} failed. Check the user name and the password or key.', vars);
            case 'unreachable': return tr('codeStudio.workspaceErrorUnreachable', '{{host}} cannot be reached.', vars);
            case 'not_directory': return tr('codeStudio.workspaceErrorNotDirectory', '{{path}} is not a folder.', vars);
            case 'not_found': return tr('codeStudio.workspaceErrorNotFound', '{{path}} does not exist.', vars);
            case 'workspace_not_found': return tr('codeStudio.workspaceMissing', 'This SSH workspace no longer exists. Code Studio switched to the local workspace.', vars);
            default: return tr('codeStudio.workspaceErrorOther', 'Something went wrong: {{error}}', vars);
        }
    }
    function workspaceFormErrorText(code) {
        if (code === 'host') return tr('codeStudio.workspaceFormBadHost', 'Enter a host name or IP address, without user or scheme');
        if (code === 'port') return tr('codeStudio.workspaceFormBadPort', 'Port 1–65535');
        if (code === 'key') return tr('codeStudio.workspaceFormBadKey', 'Paste a private key that starts with -----BEGIN');
        return tr('codeStudio.workspaceFormRequired', 'Required');
    }
    function workspaceLastUsedText(ts) {
        const rel = WS_MODEL.relativeTime(ts, Date.now());
        if (!rel) return tr('codeStudio.workspaceNeverUsed', 'Never used');
        try {
            return new Intl.RelativeTimeFormat(document.documentElement.lang || undefined, { numeric: 'auto' }).format(rel.value, rel.unit);
        } catch (_) {
            return new Date(ts).toLocaleDateString();
        }
    }

    // Called by api() in core.js for a failed scoped request of the given workspace.
    function workspaceNoteRequestError(instance, workspaceId, err) {
        if (!isLiveInstance(instance) || !WS_MODEL.isRemote(workspaceId) || instance.workspaceId !== workspaceId) return;
        const kind = WS_MODEL.classifyError(err);
        if (kind === 'unreachable') runWithInstance(instance, () => workspaceMarkLost(err));
        else if (kind === 'workspace_not_found') runWithInstance(instance, workspaceForgetMissing);
    }

    function workspaceApplySession(id) {
        const session = WS_MODEL.readSessions(localStorage)[id];
        if (!session) {
            // A new SSH workspace starts empty; local without a stored session keeps the tabs loadState read.
            if (WS_MODEL.isRemote(id)) {
                state.openTabs = [];
                state.activeTabIndex = -1;
                state.recentFiles = [];
                restoreSplitPreference(null);
            }
            return;
        }
        const paths = normalizePersistedPaths(session.openTabs);
        state.openTabs = paths.map(path => ({ path, content: '', modified: false, language: languageForPath(path), view: null }));
        state.activeTabIndex = paths.length ? Math.min(Math.max(session.activeTabIndex, 0), paths.length - 1) : -1;
        state.recentFiles = normalizePersistedPaths(session.recentFiles);
        restoreSplitPreference(session.split);
    }

    function workspaceSaveSession(session) {
        WS_MODEL.writeSession(localStorage, state.workspaceId, session, Date.now());
    }

    // The desktop window subtitle shows user@host:~/path (the Files app uses the same slot for its folder).
    function workspaceUpdateWindowTitle() {
        const win = state && state.root && typeof state.root.closest === 'function' ? state.root.closest('.vd-window') : null;
        const subtitle = win && win.querySelector('.vd-window-subtitle');
        if (!subtitle) return;
        const ws = activeWorkspace();
        subtitle.textContent = workspaceIsRemote() ? workspaceHostLabel(ws) + ':' + WS_MODEL.displayRoot(ws) : '';
        subtitle.title = workspaceIsRemote() ? workspaceHostLabel(ws) + ':' + ws.root : '';
    }
    function workspaceTerminalLabel(name) { return workspaceIsRemote() ? workspaceHostLabel() + ' · ' + name : name; }
    function workspaceTerminalBanner() {
        return workspaceIsRemote() ? ' (' + workspaceHostLabel() + ':' + WS_MODEL.displayRoot(activeWorkspace()) + ')' : '';
    }
    function workspaceToast(message) {
        const notify = state && state.context && state.context.notify;
        if (typeof notify === 'function') notify({ title: tr('codeStudio.title', 'Code Studio'), message });
        flashStatus(message, 6000);
    }
    // A failed save keeps the tab dirty (saveTab never clears modified on error); remote workspaces also get a toast.
    function workspaceSaveFailed(tab, err) {
        if (!workspaceIsRemote() || !tab) return;
        const kind = WS_MODEL.classifyError(err);
        const error = kind === 'unreachable' ? workspaceErrorText('unreachable') : (err && err.message || String(err || ''));
        workspaceToast(tr('codeStudio.workspaceSaveFailedToast', '{{file}} was not saved. {{error}} Your changes stay in the tab.', { file: baseName(tab.path), error }));
    }
```

- [ ] **Step 3:** In `scripts/build-ui-bundles.js`, insert two parts directly after the line `'ui/js/desktop/apps/code-studio/format.js',`:

```js
      // Plan C (SSH workspaces): the pure model (window.CodeStudioWorkspaceModel) before the fragment that uses it.
      'ui/js/desktop/apps/code-studio/workspace-model.js',
      'ui/js/desktop/apps/code-studio/workspace.js',
```

- [ ] **Step 4: Edits in `core.js`.**

  (a) In `createInstance`, insert these lines directly before `iconMarkup: context && typeof context.iconMarkup === 'function' ? ...`:

```js
            // Plan C: the active workspace ('local' or a saved SSH workspace) and its connection state.
            workspaceId: 'local',
            workspace: null,
            workspaces: [],
            workspaceStatus: 'local',
            workspaceError: null,
            workspaceWasLost: false,
            workspaceSwitching: false,
            workspaceSwitchTarget: '',
            workspaceRetryAttempt: 0,
            workspaceRetryAt: 0,
            workspaceRetryTimer: 0,
            workspaceRetryTick: 0,
            workspaceMenu: null,
            workspaceDialog: null,
```

  (b) Replace the whole `async function api(path, options) { ... }` with:

```js
    async function api(path, options) {
        const requestOptions = Object.assign({
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' }
        }, options || {});
        if (requestOptions.body instanceof FormData) delete requestOptions.headers;
        // Plan C: scoped requests name their workspace. The id is read when the request starts, so a request on its
        // way stays with the workspace it was started for (switching waits for pending saves).
        const instance = state || currentInstance();
        const workspaceId = instance && WS_MODEL.isScopedPath(path) ? instance.workspaceId : 'local';
        let response;
        try {
            response = await fetch(path, WS_MODEL.withWorkspaceHeaders(requestOptions, workspaceId, path));
        } catch (err) {
            workspaceNoteRequestError(instance, workspaceId, err);
            throw err;
        }
        if (!response.ok) {
            const text = await response.text().catch(() => '');
            let body = null;
            try { body = text ? JSON.parse(text) : null; } catch (_) { body = null; }
            const error = new Error((body && (body.error || body.message)) || text || response.statusText);
            error.status = response.status;
            if (body && typeof body === 'object') {
                error.code = body.code || '';
                error.body = body;
            }
            workspaceNoteRequestError(instance, workspaceId, error);
            throw error;
        }
        if (response.status === 204) return {};
        return response.json();
    }
```

  (c) In `apiClient`, insert these entries directly after the `gitBlame: path => api(...),` entry:

```js
        // Plan C: workspaces and the SSH folder picker (unscoped, see workspace-model.js).
        workspaces: () => api('/api/code-studio/workspaces'),
        createWorkspace: body => api('/api/code-studio/workspaces', { method: 'POST', body: JSON.stringify(body) }),
        deleteWorkspace: id => api('/api/code-studio/workspaces/' + encodeURIComponent(id), { method: 'DELETE' }),
        connectWorkspace: (id, trust) => api('/api/code-studio/workspaces/' + encodeURIComponent(id) + '/connect', {
            method: 'POST',
            body: JSON.stringify(trust ? { trust_host_key: trust } : {})
        }),
        remoteDevices: () => api('/api/code-studio/remote/devices'),
        createRemoteDevice: body => api('/api/code-studio/remote/devices', { method: 'POST', body: JSON.stringify(body) }),
        remoteBrowse: (deviceId, path, trust) => {
            const query = new URLSearchParams({ device_id: deviceId });
            if (path) query.set('path', path);
            if (trust) query.set('trust_host_key', trust);
            return api('/api/code-studio/remote/browse?' + query);
        },
```

  (d) Replace the whole `function saveState() { ... }` with:

```js
    function saveState() {
        const openPaths = normalizePersistedPaths(state.openTabs.filter(tab => !isVirtualTab(tab)));
        const recentPaths = normalizePersistedPaths(state.recentFiles);
        const activeIndex = openPaths.length ? Math.min(Math.max(Number.isInteger(state.activeTabIndex) ? state.activeTabIndex : 0, 0), openPaths.length - 1) : -1;
        const split = splitStatePayload(); // IDE package 1A
        // Plan C: the tabs belong to the active workspace; the legacy key keeps only the local workspace's tabs.
        workspaceSaveSession({ openTabs: openPaths, activeTabIndex: activeIndex, recentFiles: recentPaths, split });
        let legacy = null;
        if (workspaceIsRemote()) {
            try { legacy = JSON.parse(localStorage.getItem(STATE_KEY) || 'null') || {}; } catch (_) { legacy = {}; }
        }
        const payload = {
            openTabs: legacy ? (Array.isArray(legacy.openTabs) ? legacy.openTabs : []) : openPaths.map(path => ({ path })),
            activeTabIndex: legacy ? (Number.isInteger(legacy.activeTabIndex) ? legacy.activeTabIndex : -1) : activeIndex,
            sidebarVisible: state.sidebarVisible,
            sidebarWidth: clampDimension(state.sidebarWidth, MIN_SIDEBAR_WIDTH, MAX_SIDEBAR_WIDTH, 280),
            editorFontSize: state.editorFontSize,
            terminalVisible: state.panelVisible,
            panelVisible: state.panelVisible,
            activePanel: state.activePanel,
            sidebarView: state.sidebarView,
            terminalHeight: clampDimension(state.terminalHeight, MIN_TERMINAL_HEIGHT, MAX_TERMINAL_HEIGHT, 220),
            recentFiles: legacy ? (Array.isArray(legacy.recentFiles) ? legacy.recentFiles : []) : recentPaths,
            split: legacy ? (legacy.split || null) : split
        };
        localStorage.setItem(STATE_KEY, JSON.stringify(payload));
    }
```

  (e) In `restoreTabs`, replace
  `if (!savedPaths.length && !(target.context && target.context.path)) {`
  with
  `if (!savedPaths.length && !(target.context && target.context.path) && !workspaceIsRemote(target)) {`.
  The hello.* defaults only apply to the local workspace.

  (f) In `saveTab`, replace

```js
            try {
                await apiClient.writeFile(path, content);
            } catch (err) {
                if (isLiveInstance(target)) runWithInstance(target, () => showOperationError(err));
                return false;
            }
```

  with

```js
            try {
                await apiClient.writeFile(path, content);
            } catch (err) {
                // Plan C: the tab stays dirty; in an SSH workspace a toast says why.
                if (isLiveInstance(target)) runWithInstance(target, () => { showOperationError(err); workspaceSaveFailed(tab, err); });
                return false;
            }
```

  (g) In `downloadFile`, wrap both URLs:
  - `url: workspaceURL('/api/code-studio/download?path=' + encodeURIComponent(file.path))`
  - `window.location.href = workspaceURL('/api/code-studio/download?path=' + encodeURIComponent(file.path));`

  (h) In `uploadFile`, use `uploadURL: workspaceURL('/api/code-studio/upload')`.

  (i) Replace the body of `showOperationError` with:

```js
        const message = err && err.message ? err.message : String(err || '');
        // Plan C: in an SSH workspace the error is not a container error.
        renderStatus(workspaceIsRemote() ? workspaceErrorText('other', { error: message }) : tr('codeStudio.containerError', 'Container error: {error}', { error: message }));
        writeTerminalLine(message);
```

- [ ] **Step 5: Verify.**
  1. Run `npm run build:ui`, then `npm run build:ui -- --check`.
     Expected: `Checked ui/js/desktop/bundles/code-studio.bundle.js (24 parts)`, or more parts if plan B already landed, and no `Outdated` line.
  2. Run `go test ./ui/ -run '(?i)CodeStudio' -count=1`.
     Expected: `ok`; the browser tests skip without the env variable.
  3. Run `node --test ui/js/desktop/apps/code-studio/workspace-model.test.cjs`.
     Expected: 11 pass.

  `workspaceMarkLost` and `workspaceForgetMissing` arrive in Task 9. They are only reachable once a remote workspace is active, which Task 9 also enables.
- [ ] **Step 6:** Run detect-changes, then commit `workspace.js`, `scripts/build-ui-bundles.js` and `core.js` (not the bundle) with the message `feat(code-studio): workspace header on every request and per-workspace tab sessions`.

---

### Task 5: Sockets and terminal labels

**Files:** Modify `terminal.js` and `lsp.js`

- [ ] **Step 1: Impact.** Run impact for `renderTerminal`, `connectTerminalSession` and `lspOpenSocket`.
  Expected callers:
  - `renderTerminal`: many render paths.
  - `connectTerminalSession`: `connectTerminal`, `addTerminalSession`.
  - `lspOpenSocket`: `lspStart`, `lspSocketClosed`, `lspTouchConnection`.
- [ ] **Step 2: `terminal.js`.**
  - (a) In `renderTerminal`, change `<span>${esc(session.name || shellName(index))}</span>` to `<span>${esc(workspaceTerminalLabel(session.name || shellName(index)))}</span>`.
  - (b) In `renderTerminal`, change the fallback `<span>${esc(tr('codeStudio.terminal', 'Terminal'))}</span>` to `<span>${esc(workspaceTerminalLabel(tr('codeStudio.terminal', 'Terminal')))}</span>`.
  - (c) In `connectTerminalSession`, change the writeln line to:
    `term.writeln(tr('codeStudio.title', 'Code Studio') + ' - ' + shellName(index) + workspaceTerminalBanner());`
    The test marker `tr('codeStudio.title', 'Code Studio') + ' - ' + shellName(index)` stays intact.
  - (d) Replace these two lines

```js
            const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
            const ws = new WebSocket(protocol + '//' + location.host + '/api/code-studio/terminal');
```

    with

```js
            const ws = new WebSocket(workspaceSocketURL('/api/code-studio/terminal', instance)); // Plan C: ws=<id>
```

    This is safe because `instance` is declared above, in the same `try` block.

- [ ] **Step 3: `lsp.js`.** In `lspOpenSocket`, replace

```js
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        let socket;
        try {
            socket = new WebSocket(protocol + '//' + location.host + '/api/code-studio/lsp?lang=' + encodeURIComponent(conn.language));
```

  with

```js
        let socket;
        try {
            // Plan C: the bridge serves the active workspace; a switch shuts the sessions down and later opens new sockets.
            socket = new WebSocket(workspaceSocketURL('/api/code-studio/lsp?lang=' + encodeURIComponent(conn.language), instance));
```

- [ ] **Step 4:** Run `npm run build:ui`, then `go test ./ui/ -run '(?i)(CodeStudio|TerminalFrames|ShellI18n)' -count=1`.
  Expected: `ok`.
- [ ] **Step 5:** Run detect-changes, then commit `terminal.js` and `lsp.js` with the message `feat(code-studio): terminal and language server sockets carry the workspace`.

---

### Task 6: Status chip, Explorer switcher and banner (`workspace.js` part B)

**Files:** Append to `workspace.js`; modify `core.js` and `sidebar.js`

- [ ] **Step 1: Impact.** Run impact for `renderStatus`, `shellMarkup`, `sidebarHeadMarkup`, `wireSidebarHead` and `renderSidebar`.
  Expected callers:
  - `renderStatus`: many (HIGH fan-in; the change is additive).
  - `shellMarkup`: `render`, `ensureShellRoot`.
  - `sidebarHeadMarkup`: `renderSidebar`.
  - `wireSidebarHead`: `renderSidebar`, `renderOutline`.
- [ ] **Step 2:** Append part B to `workspace.js`:

```js
    // --- Chrome: status chip, Explorer switcher, banner -----------------------------------------------

    function workspaceMenuOpenFor(kind) {
        return !!(state && state.workspaceMenu && state.workspaceMenu.__anchorKind === kind);
    }

    function workspaceStatusChipMarkup() {
        const remote = workspaceIsRemote();
        const status = workspaceChipState();
        const ws = activeWorkspace();
        const text = remote ? tr('codeStudio.workspaceChipRemote', 'SSH: {{name}}', { name: ws.deviceName || ws.host }) : tr('codeStudio.workspaceChipLocal', 'Local');
        const title = remote ? workspaceDisplayLabel(ws) + ' — ' + workspaceHostLabel(ws) + '\n' + workspaceStatusText(status) : tr('codeStudio.workspaceLocalSub', 'Container · /workspace');
        const icon = status === 'connecting'
            ? '<span class="cs-ws-spinner" aria-hidden="true"></span>'
            : wsGlyph(['lost', 'error', 'hostkey'].includes(status) ? 'warning' : (remote ? 'server' : 'laptop'), 12);
        return `<span class="cs-status-item cs-status-remote" data-clickable data-status-remote data-remote-state="${esc(status)}" role="button" tabindex="0" aria-haspopup="menu" aria-expanded="${workspaceMenuOpenFor('chip')}" aria-label="${esc(text + ', ' + workspaceStatusText(status))}" title="${esc(title)}">${icon}<span class="cs-status-remote-label">${esc(text)}</span></span>`;
    }

    function wireWorkspaceChip(statusEl, refocus) {
        const chip = statusEl.querySelector('[data-status-remote]');
        if (!chip) return;
        chip.addEventListener('click', bind(() => openWorkspaceMenu(chip)));
        chip.addEventListener('keydown', bind(event => {
            if (!['Enter', ' ', 'ArrowUp'].includes(event.key)) return;
            event.preventDefault();
            openWorkspaceMenu(chip);
        }));
        if (refocus) chip.focus();
    }

    function workspaceSwitcherMarkup() {
        const ws = activeWorkspace();
        const remote = workspaceIsRemote();
        const status = workspaceChipState();
        const sub = remote ? workspaceHostLabel(ws) : tr('codeStudio.workspaceLocalSub', 'Container · /workspace');
        return `<button type="button" class="cs-ws-switcher" data-ws-switcher data-remote-state="${esc(status)}" aria-haspopup="menu" aria-expanded="${workspaceMenuOpenFor('switcher')}" title="${esc(tr('codeStudio.workspaceMenu', 'Workspaces'))}">
            <span class="cs-ws-switcher-icon">${status === 'connecting' ? '<span class="cs-ws-spinner" aria-hidden="true"></span>' : wsGlyph(remote ? 'server' : 'laptop', 16)}</span>
            <span class="cs-ws-switcher-text"><span class="cs-ws-switcher-label">${esc(workspaceDisplayLabel(ws))}</span><span class="cs-ws-switcher-sub">${esc(sub)}</span></span>
            ${remote ? `<span class="cs-ws-dot" data-ws-dot="${esc(workspaceDotState(ws))}" aria-hidden="true"></span>` : ''}
            <span class="cs-ws-switcher-chevron">${wsGlyph('chevron', 14)}</span>
        </button>`;
    }

    function wireWorkspaceSwitcher(root) {
        const button = root && root.querySelector('[data-ws-switcher]');
        if (!button || button._wsWired) return;
        button._wsWired = true;
        button.addEventListener('click', bind(() => openWorkspaceMenu(button)));
        button.addEventListener('keydown', bind(event => {
            if (event.key !== 'ArrowDown') return;
            event.preventDefault();
            openWorkspaceMenu(button);
        }));
    }

    function workspaceRenderSwitcher() {
        const current = shellPart('[data-ws-switcher]');
        if (!current) return;
        const hadFocus = document.activeElement === current;
        const holder = document.createElement('div');
        holder.innerHTML = workspaceSwitcherMarkup();
        const next = holder.firstElementChild;
        current.replaceWith(next);
        wireWorkspaceSwitcher(next.parentElement);
        if (hadFocus) next.focus();
    }

    function workspaceRootCrumbLabel() {
        if (!workspaceIsRemote()) return tr('codeStudio.workspaceRoot', 'Workspace');
        const root = activeWorkspace().root;
        return root === '/' ? '/' : root.split('/').filter(Boolean).pop();
    }

    function workspaceRenderBanner() {
        const banner = shellPart('[data-ws-banner]');
        if (!banner) return;
        const status = state.workspaceStatus;
        const show = workspaceIsRemote() && !state.workspaceSwitching
            && (['lost', 'hostkey', 'error'].includes(status) || (status === 'connecting' && state.workspaceWasLost));
        if (!show) {
            if (!banner.hidden) {
                banner.hidden = true;
                banner.innerHTML = '';
                banner.dataset.signature = '';
                delete banner.dataset.wsBannerState;
            }
            return;
        }
        const host = workspaceHostLabel();
        const error = state.workspaceError || {};
        let text;
        let actions;
        if (status === 'connecting') {
            text = tr('codeStudio.workspaceStatusConnecting', 'Connecting…') + ' ' + host;
            actions = [];
        } else if (status === 'lost') {
            text = tr('codeStudio.workspaceLost', 'Connection to {{host}} lost', { host });
            actions = ['reconnect', 'local'];
        } else if (status === 'hostkey') {
            text = tr('codeStudio.workspaceTrustTitle', 'Unknown host key') + ': ' + host;
            actions = ['review', 'local'];
        } else {
            text = workspaceErrorText(error.kind, { host, path: activeWorkspace().root, error: error.message || '' });
            actions = error.kind === 'host_key_mismatch' ? ['local'] : ['reconnect', 'local'];
        }
        const seconds = status === 'lost' && state.workspaceRetryAt ? Math.max(0, Math.ceil((state.workspaceRetryAt - Date.now()) / 1000)) : -1;
        const detail = seconds >= 0 ? tr('codeStudio.workspaceRetryIn', 'Retrying in {{seconds}} s', { seconds }) : '';
        const signature = [status, text, actions.join(',')].join('|');
        banner.hidden = false;
        banner.dataset.wsBannerState = status;
        if (banner.dataset.signature === signature) {
            // The countdown tick changes only the detail, so focus and screen readers stay calm.
            const countdown = banner.querySelector('[data-ws-banner-detail]');
            if (countdown) countdown.textContent = detail;
            return;
        }
        banner.dataset.signature = signature;
        banner.setAttribute('role', status === 'error' ? 'alert' : 'status');
        const labels = {
            reconnect: tr('codeStudio.workspaceReconnect', 'Reconnect'),
            local: tr('codeStudio.workspaceUseLocal', 'Use local workspace'),
            review: tr('codeStudio.workspaceReviewHostKey', 'Review host key…')
        };
        banner.innerHTML = `<span class="cs-ws-banner-icon">${status === 'connecting' ? '<span class="cs-ws-spinner" aria-hidden="true"></span>' : wsGlyph('warning', 16)}</span>
            <span class="cs-ws-banner-text"><strong>${esc(text)}</strong><span class="cs-ws-banner-detail" data-ws-banner-detail aria-hidden="true">${esc(detail)}</span></span>
            <span class="cs-ws-banner-actions">${actions.map(action => `<button type="button" class="cs-button${action === 'local' ? '' : ' primary'}" data-ws-banner-action="${action}">${esc(labels[action])}</button>`).join('')}</span>`;
        banner.querySelectorAll('[data-ws-banner-action]').forEach(button => button.addEventListener('click', bind(() => workspaceBannerAction(button.dataset.wsBannerAction))));
    }

    function workspaceBannerAction(action) {
        if (action === 'reconnect') workspaceReconnect();
        else if (action === 'local') switchWorkspace(WS_MODEL.LOCAL_ID);
        else if (action === 'review') {
            const ws = activeWorkspace();
            openConnectDialog({ deviceId: ws.deviceId, path: ws.root });
        }
    }

    function workspaceRenderChrome() {
        if (!state || !studioRoot()) return;
        workspaceRenderSwitcher();
        renderStatus();
        workspaceRenderBanner();
        workspaceUpdateWindowTitle();
    }
```

- [ ] **Step 3: `core.js`.**

  (a) In `shellMarkup`, directly after `<div class="code-studio-search" data-search hidden></div>`, add:

```html
            <div class="cs-ws-banner" data-ws-banner hidden></div>
```

  (b) In `renderStatus`, replace the opening block

```js
        const leftItems = [];
        leftItems.push(`<span class="cs-status-item cs-status-container" title="${esc(containerStatusLabel())}"><span class="cs-status-dot" data-status="${esc(state.containerStatus || 'unknown')}"></span>${esc(containerStatusLabel())}</span>`);
```

  with

```js
        const leftItems = [];
        const remote = workspaceIsRemote();
        const chipFocused = !!document.activeElement && statusEl.contains(document.activeElement) && document.activeElement.matches('[data-status-remote]');
        leftItems.push(workspaceStatusChipMarkup()); // Plan C: the workspace chip sits at the far left
        if (!remote) leftItems.push(`<span class="cs-status-item cs-status-container" title="${esc(containerStatusLabel())}"><span class="cs-status-dot" data-status="${esc(state.containerStatus || 'unknown')}"></span>${esc(containerStatusLabel())}</span>`);
```

  Then make the network and idle items local-only:
  - change `if (info.network === 'none') leftItems.push(` to `if (!remote && info.network === 'none') leftItems.push(`
  - change `const idleLabel = containerIdleLabel(info);` to `const idleLabel = remote ? '' : containerIdleLabel(info);`

  Directly after `statusEl.innerHTML = leftItems.join('') + '<span class="cs-status-spacer"></span>' + rightItems.join('');`, add:

```js
        wireWorkspaceChip(statusEl, chipFocused); // Plan C
```

- [ ] **Step 4: `sidebar.js`.**

  (a) In `sidebarHeadMarkup`, replace the `const crumbs = [...]` line with:

```js
        // Plan C: in an SSH workspace the root crumb shows the remote folder name; the switcher sits on top.
        const rootTitle = workspaceIsRemote() ? workspaceHostLabel() + ':' + activeWorkspace().root : WORKSPACE_ROOT;
        const crumbs = [`<button type="button" class="cs-sidebar-crumb${segments.length ? '' : ' current'}" data-sidebar-nav="${esc(WORKSPACE_ROOT)}" title="${esc(rootTitle)}">${iconMarkup('home', 'W', 'cs-sidebar-crumb-icon', 13)}<span>${esc(workspaceRootCrumbLabel())}</span></button>`];
```

  Then change `return \`<div class="cs-sidebar-head">` to `return \`<div class="cs-sidebar-head">${workspaceSwitcherMarkup()}`.

  (b) In `wireSidebarHead`, directly after `updatePolicyControls();`, add `wireWorkspaceSwitcher(sidebar); // Plan C`.

  (c) In `renderSidebar`, keep the switcher focused across re-renders. After `const focusedPath = focused ? focused.filePath : '';`, add:

```js
        const switcherFocused = !!document.activeElement && sidebar.contains(document.activeElement) && document.activeElement.matches('[data-ws-switcher]');
        const refocusSwitcher = () => { if (switcherFocused) { const button = sidebar.querySelector('[data-ws-switcher]'); if (button) button.focus(); } };
```

  Call `refocusSwitcher();` in two places:
  - after `wireSidebarResize();` inside the `if (errorMessage)` branch, before its `return`
  - after the final `wireSidebarResize();`

- [ ] **Step 5:** Run `npm run build:ui`, then `go test ./ui/ -run '(?i)CodeStudio' -count=1`.
  Expected: `ok`.
- [ ] **Step 6:** Run detect-changes, then commit `workspace.js`, `core.js` and `sidebar.js` with the message `feat(code-studio): workspace chip, Explorer switcher and connection banner`.

---

### Task 7: CSS for the chrome, menu and switch prompt

**Files:** Append to `ui/css/code-studio.css`. The menu and the modals mount on `document.body`; they reuse `.cs-context-menu` / `.cs-modal-backdrop`, which already carry the token scope.

- [ ] **Step 1:** Append:

```css
/* === Plan C: SSH workspaces — chrome ===================================================== */
.cs-ws-sr { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }
.cs-ws-glyph { flex: 0 0 auto; display: block; }
.cs-ws-spinner { flex: 0 0 auto; width: 12px; height: 12px; border: 2px solid currentColor; border-right-color: transparent; border-radius: 50%; animation: cs-spin 0.8s linear infinite; }

.cs-ws-switcher { display: flex; align-items: center; gap: 8px; width: 100%; min-height: 38px; margin: 0 0 4px; padding: 5px 8px; border: 1px solid var(--vd-mat-field-border, var(--cs-line)); border-radius: var(--vd-mat-field-radius, var(--cs-radius-md)); color: var(--cs-text); background: var(--vd-mat-field, var(--cs-panel-soft)); font: inherit; text-align: left; cursor: pointer; transition: border-color var(--cs-transition-fast), background var(--cs-transition-fast); }
.cs-ws-switcher:hover { background: var(--cs-hover); }
.cs-ws-switcher:focus-visible { outline: 2px solid var(--cs-accent-border); outline-offset: 1px; }
.cs-ws-switcher[aria-expanded="true"] { border-color: var(--cs-accent-border); }
.cs-ws-switcher-icon { display: inline-flex; color: var(--cs-muted); }
.cs-ws-switcher:is([data-remote-state="connected"], [data-remote-state="connecting"]) .cs-ws-switcher-icon { color: var(--cs-accent); }
.cs-ws-switcher:is([data-remote-state="lost"], [data-remote-state="hostkey"]) .cs-ws-switcher-icon { color: var(--cs-warning); }
.cs-ws-switcher[data-remote-state="error"] .cs-ws-switcher-icon { color: var(--cs-danger); }
.cs-ws-switcher-text { display: grid; flex: 1 1 auto; min-width: 0; line-height: 1.25; }
.cs-ws-switcher-label { overflow: hidden; font-size: 12px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.cs-ws-switcher-sub { overflow: hidden; color: var(--cs-muted); font: 11px var(--cs-mono-font); text-overflow: ellipsis; white-space: nowrap; }
.cs-ws-switcher-chevron { display: inline-flex; color: var(--cs-muted); transition: transform var(--cs-transition-fast); }
.cs-ws-switcher[aria-expanded="true"] .cs-ws-switcher-chevron { transform: rotate(180deg); }

.cs-ws-dot { flex: 0 0 auto; width: 8px; height: 8px; border-radius: 50%; background: var(--cs-muted); opacity: 0.6; }
.cs-ws-dot[data-ws-dot="connected"] { background: var(--cs-success); opacity: 1; }
.cs-ws-dot[data-ws-dot="connecting"] { background: var(--cs-accent); opacity: 1; animation: cs-pulse 1.2s ease-in-out infinite; }
.cs-ws-dot:is([data-ws-dot="lost"], [data-ws-dot="hostkey"]) { background: var(--cs-warning); opacity: 1; }
.cs-ws-dot[data-ws-dot="error"] { background: var(--cs-danger); opacity: 1; }

.code-studio-statusbar > span.cs-status-remote { flex: 0 0 auto; align-self: stretch; max-width: 22em; margin-left: -10px; padding: 0 10px; border-radius: 0; color: var(--cs-muted); }
.code-studio-statusbar > span.cs-status-remote:focus-visible { outline: 2px solid var(--cs-accent-border); outline-offset: -2px; }
.code-studio-statusbar > span.cs-status-remote:not([data-remote-state="local"]) { color: var(--cs-accent-contrast); background: var(--cs-accent); }
.code-studio-statusbar > span.cs-status-remote:not([data-remote-state="local"]):hover { filter: brightness(1.08); }
.code-studio-statusbar > span.cs-status-remote:is([data-remote-state="lost"], [data-remote-state="hostkey"]) { color: #1d1400; background: var(--cs-warning); }
.code-studio-statusbar > span.cs-status-remote[data-remote-state="error"] { color: #fff; background: var(--cs-danger); }
.cs-status-remote-label { overflow: hidden; font-weight: 600; text-overflow: ellipsis; }

.cs-ws-banner { display: flex; flex: 0 0 auto; align-items: center; gap: 10px; min-height: 36px; padding: 6px 12px; border-bottom: 1px solid var(--cs-line); color: var(--cs-text); background: rgba(245, 181, 68, 0.14); font-size: 12px; }
.cs-ws-banner[hidden] { display: none; }
.cs-ws-banner[data-ws-banner-state="error"] { background: var(--cs-danger-soft); }
.cs-ws-banner[data-ws-banner-state="connecting"] { background: var(--cs-accent-faint); }
.cs-ws-banner-icon { display: inline-flex; color: var(--cs-warning); }
.cs-ws-banner[data-ws-banner-state="error"] .cs-ws-banner-icon { color: var(--cs-danger); }
.cs-ws-banner[data-ws-banner-state="connecting"] .cs-ws-banner-icon { color: var(--cs-accent); }
.cs-ws-banner-text { display: flex; flex: 1 1 auto; flex-wrap: wrap; align-items: baseline; gap: 4px 10px; min-width: 0; }
.cs-ws-banner-detail { color: var(--cs-muted); }
.cs-ws-banner-actions { display: inline-flex; flex: 0 0 auto; gap: 6px; }
.cs-ws-banner-actions .cs-button { min-height: 26px; padding: 0 10px; }

.cs-context-menu.cs-ws-menu { min-width: 280px; max-width: min(440px, calc(100vw - 16px)); max-height: min(70vh, 520px); overflow-y: auto; gap: 1px; }
.cs-ws-menu-heading { padding: 6px 9px 3px; color: var(--vd-mat-label-color, var(--cs-muted)); font: 700 10.5px var(--vd-mat-label-font, var(--cs-ui-font)); letter-spacing: var(--vd-mat-label-tracking, 0.5px); text-transform: var(--vd-mat-label-transform, uppercase); }
.cs-ws-menu-row { display: flex; align-items: center; gap: 2px; border-radius: var(--ds-radius-sm, 6px); }
.cs-context-menu.cs-ws-menu button span { flex: 0 0 auto; }
.cs-context-menu.cs-ws-menu button.cs-ws-menu-item { display: flex; flex: 1 1 auto; align-items: center; gap: 9px; min-width: 0; height: auto; min-height: 32px; padding: 5px 9px; }
.cs-context-menu.cs-ws-menu button span.cs-ws-menu-text { display: grid; flex: 1 1 auto; min-width: 0; line-height: 1.3; white-space: normal; }
.cs-context-menu.cs-ws-menu button:focus-visible { outline: 2px solid var(--cs-accent-border); outline-offset: -2px; }
.cs-ws-menu-row.is-current .cs-ws-menu-item { background: var(--cs-accent-faint); }
.cs-ws-menu-icon { display: inline-flex; color: var(--cs-muted); }
.cs-ws-menu-row.is-current .cs-ws-menu-icon { color: var(--cs-accent); }
.cs-context-menu.cs-ws-menu .cs-ws-menu-label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cs-context-menu.cs-ws-menu .cs-ws-menu-sub { overflow: hidden; color: var(--cs-muted); font: 11px var(--cs-mono-font); text-overflow: ellipsis; white-space: nowrap; }
.cs-ws-menu-check { display: inline-flex; width: 14px; color: var(--cs-accent); }
.cs-context-menu.cs-ws-menu button.cs-ws-menu-remove { display: inline-flex; flex: 0 0 auto; align-items: center; justify-content: center; width: 26px; height: 26px; padding: 0; color: var(--cs-muted); opacity: 0; }
.cs-ws-menu-row:is(:hover, :focus-within) .cs-ws-menu-remove { opacity: 1; }
.cs-context-menu.cs-ws-menu button.cs-ws-menu-remove:hover { color: var(--cs-danger); background: var(--cs-danger-soft); }
@media (hover: none) { .cs-context-menu.cs-ws-menu button.cs-ws-menu-remove { width: 34px; height: 34px; opacity: 1; } }

.cs-ws-dialog-title { margin: 0; color: var(--cs-text); font-size: 15px; font-weight: 650; }
.cs-modal.cs-ws-dirty { width: min(440px, calc(100vw - 32px)); }
.cs-modal.cs-ws-dirty .cs-ws-dialog-title { margin-bottom: 8px; }
.cs-ws-git-missing strong { color: var(--cs-text); }

@media (prefers-reduced-motion: reduce) {
    .cs-ws-menu *, .cs-ws-spinner, .cs-ws-dot, .cs-ws-switcher, .cs-ws-switcher-chevron { animation: none !important; transition: none !important; }
}
```

- [ ] **Step 2:** Run `go test ./ui/ -run '(?i)CodeStudio' -count=1`.
  Expected: `ok`.
- [ ] **Step 3:** Commit `ui/css/code-studio.css` with the message `style(code-studio): workspace switcher, remote chip, banner and menu`.

---

### Task 8: The workspace menu (`workspace.js` part C)

**Files:** Append to `workspace.js`

- [ ] **Step 1:** Append:

```js
    // --- Workspace menu ------------------------------------------------------------------------

    async function workspaceLoadList(instance = state) {
        const target = instance;
        try {
            const payload = await apiClient.workspaces();
            if (!isLiveInstance(target)) return [];
            target.workspaces = WS_MODEL.normalizeWorkspaceList(payload);
            if (workspaceIsRemote(target)) {
                const fresh = target.workspaces.find(ws => ws.id === target.workspaceId);
                if (fresh) target.workspace = Object.assign({}, target.workspace, fresh, { capabilities: fresh.capabilities || (target.workspace && target.workspace.capabilities) || null });
            }
        } catch (_) {
            if (isLiveInstance(target) && !(target.workspaces || []).length) target.workspaces = [WS_MODEL.localWorkspace()];
        }
        return target.workspaces;
    }

    function closeWorkspaceMenu(instance = state, restoreFocus = false) {
        const menu = instance && instance.workspaceMenu;
        if (menu && typeof menu.__codeStudioCleanup === 'function') menu.__codeStudioCleanup(restoreFocus);
    }

    async function openWorkspaceMenu(anchor) {
        const instance = state;
        if (!isLiveInstance(instance) || !anchor) return;
        const kind = anchor.hasAttribute('data-status-remote') ? 'chip' : 'switcher';
        if (instance.workspaceMenu) {
            const same = instance.workspaceMenu.__anchorKind === kind;
            closeWorkspaceMenu(instance, same);
            if (same) return;
        }
        document.querySelectorAll('.cs-context-menu').forEach(other => {
            if (typeof other.__codeStudioCleanup === 'function') other.__codeStudioCleanup();
            else other.remove();
        });
        const menu = document.createElement('div');
        menu.className = 'cs-context-menu cs-ws-menu';
        menu.setAttribute('role', 'menu');
        menu.setAttribute('aria-label', tr('codeStudio.workspaceMenu', 'Workspaces'));
        menu.__anchorKind = kind;
        document.body.appendChild(menu);
        instance.workspaceMenu = menu;
        anchor.setAttribute('aria-expanded', 'true');
        let closed = false;
        let unregister = () => {};
        // The status bar and the sidebar re-render; the anchor is looked up again when the old element is gone.
        const currentAnchor = () => (anchor.isConnected ? anchor : (isLiveInstance(instance) ? runWithInstance(instance, () => shellPart(kind === 'chip' ? '[data-status-remote]' : '[data-ws-switcher]')) : null));
        const onPointer = event => {
            const owner = currentAnchor();
            if (!menu.contains(event.target) && !(owner && owner.contains(event.target))) cleanup(false);
        };
        const onResize = () => cleanup(false);
        const cleanup = restoreFocus => {
            if (closed) return;
            closed = true;
            unregister();
            document.removeEventListener('mousedown', onPointer, true);
            window.removeEventListener('resize', onResize);
            menu.remove();
            if (instance.workspaceMenu === menu) instance.workspaceMenu = null;
            const owner = currentAnchor();
            if (owner) owner.setAttribute('aria-expanded', 'false');
            if (restoreFocus && owner) owner.focus();
        };
        menu.__codeStudioCleanup = cleanup;
        unregister = registerDisposer(() => cleanup(false));
        menu.addEventListener('keydown', bindInstance(instance, event => workspaceMenuKeydown(event, menu, cleanup)));
        setTimeout(() => {
            if (closed) return;
            document.addEventListener('mousedown', onPointer, true);
            window.addEventListener('resize', onResize);
        }, 0);
        renderWorkspaceMenu(menu, cleanup, true);
        positionWorkspaceMenu(menu, anchor);
        await workspaceLoadList(instance);
        if (closed || !isLiveInstance(instance)) return;
        runWithInstance(instance, () => {
            renderWorkspaceMenu(menu, cleanup, menu.contains(document.activeElement));
            const owner = currentAnchor();
            if (owner) positionWorkspaceMenu(menu, owner);
        });
    }

    function positionWorkspaceMenu(menu, anchor) {
        const rect = anchor.getBoundingClientRect();
        menu.style.minWidth = Math.round(Math.max(280, Math.min(rect.width, 440))) + 'px';
        const box = menu.getBoundingClientRect();
        const below = rect.bottom + 4;
        const top = below + box.height <= window.innerHeight - 8 ? below : Math.max(8, rect.top - box.height - 4);
        menu.style.left = Math.round(Math.max(8, Math.min(rect.left, window.innerWidth - box.width - 8))) + 'px';
        menu.style.top = Math.round(top) + 'px';
    }

    function workspaceMenuItemMarkup(ws, currentId) {
        const checked = ws.id === currentId;
        const remote = ws.kind === 'ssh';
        const label = workspaceDisplayLabel(ws);
        const sub = remote ? workspaceHostLabel(ws) + ':' + WS_MODEL.displayRoot(ws) : tr('codeStudio.workspaceLocalSub', 'Container · /workspace');
        const dot = remote ? workspaceDotState(ws) : '';
        const aria = [label, sub, remote ? workspaceStatusText(dot) : ''].filter(Boolean).join(', ');
        const remove = remote
            ? `<button type="button" class="cs-ws-menu-remove" data-ws-remove="${esc(ws.id)}" tabindex="-1" aria-label="${esc(tr('codeStudio.workspaceRemove', 'Remove') + ': ' + label)}" title="${esc(tr('codeStudio.workspaceRemove', 'Remove'))}">${wsGlyph('x', 12)}</button>`
            : '';
        return `<div class="cs-ws-menu-row${checked ? ' is-current' : ''}" role="none">
            <button type="button" class="cs-ws-menu-item" role="menuitemradio" aria-checked="${checked}" tabindex="-1" data-ws-id="${esc(ws.id)}" aria-label="${esc(aria)}" title="${esc(remote ? workspaceHostLabel(ws) + ':' + ws.root : sub)}">
                <span class="cs-ws-menu-icon">${wsGlyph(remote ? 'server' : 'laptop', 15)}</span>
                <span class="cs-ws-menu-text"><span class="cs-ws-menu-label">${esc(label)}</span><span class="cs-ws-menu-sub">${esc(sub)}</span></span>
                ${remote ? `<span class="cs-ws-dot" data-ws-dot="${esc(dot)}" aria-hidden="true"></span>` : ''}
                <span class="cs-ws-menu-check" aria-hidden="true">${checked ? wsGlyph('check', 13) : ''}</span>
            </button>${remove}
        </div>`;
    }

    function workspaceMenuActionMarkup(action, glyph, label) {
        return `<button type="button" class="cs-ws-menu-item" role="menuitem" tabindex="-1" data-ws-action="${esc(action)}"><span class="cs-ws-menu-icon">${wsGlyph(glyph, 15)}</span><span class="cs-ws-menu-text"><span class="cs-ws-menu-label">${esc(label)}</span></span></button>`;
    }

    function renderWorkspaceMenu(menu, cleanup, focus) {
        const instance = state;
        const active = menu.contains(document.activeElement) ? document.activeElement : null;
        const focusedKey = active ? (active.dataset.wsId || active.dataset.wsAction || '') : '';
        const remotes = (instance.workspaces || []).filter(ws => ws.kind === 'ssh');
        const separator = '<div class="cs-context-separator" role="separator"></div>';
        const rows = [`<div class="cs-ws-menu-heading" role="presentation">${esc(tr('codeStudio.workspaceMenu', 'Workspaces'))}</div>`,
            workspaceMenuItemMarkup(WS_MODEL.localWorkspace(), instance.workspaceId)];
        if (remotes.length) {
            rows.push(separator, `<div class="cs-ws-menu-heading" role="presentation">${esc(tr('codeStudio.workspaceRecent', 'Recent SSH folders'))}</div>`);
            remotes.forEach(ws => rows.push(workspaceMenuItemMarkup(ws, instance.workspaceId)));
        }
        rows.push(separator, workspaceMenuActionMarkup('connect', 'plus', tr('codeStudio.workspaceConnect', 'Connect to SSH Folder…')));
        if (workspaceIsRemote()) {
            rows.push(workspaceMenuActionMarkup('reconnect', 'plug', tr('codeStudio.workspaceReconnect', 'Reconnect')));
            rows.push(workspaceMenuActionMarkup('disconnect', 'unplug', tr('codeStudio.workspaceDisconnect', 'Disconnect')));
        }
        menu.innerHTML = rows.join('');
        menu.querySelectorAll('[data-ws-id]').forEach(button => button.addEventListener('click', bindInstance(instance, () => { cleanup(true); switchWorkspace(button.dataset.wsId); })));
        menu.querySelectorAll('[data-ws-remove]').forEach(button => button.addEventListener('click', bindInstance(instance, event => { event.stopPropagation(); cleanup(true); workspaceRemove(button.dataset.wsRemove); })));
        menu.querySelectorAll('[data-ws-action]').forEach(button => button.addEventListener('click', bindInstance(instance, () => { cleanup(true); workspaceMenuAction(button.dataset.wsAction); })));
        if (!focus) return;
        const items = Array.from(menu.querySelectorAll('[data-ws-id], [data-ws-action]'));
        const target = (focusedKey && items.find(item => (item.dataset.wsId || item.dataset.wsAction) === focusedKey)) || menu.querySelector('[aria-checked="true"]') || items[0];
        if (target) target.focus();
    }

    function workspaceMenuKeydown(event, menu, cleanup) {
        const items = Array.from(menu.querySelectorAll('.cs-ws-menu-item:not([disabled])'));
        const active = document.activeElement;
        const current = active && active.classList.contains('cs-ws-menu-remove') ? active.parentElement.querySelector('.cs-ws-menu-item') : active;
        const index = items.indexOf(current);
        const remove = current && current.parentElement ? current.parentElement.querySelector('.cs-ws-menu-remove') : null;
        switch (event.key) {
            case 'ArrowDown':
            case 'ArrowUp': {
                event.preventDefault();
                const delta = event.key === 'ArrowDown' ? 1 : -1;
                const next = items[index < 0 ? 0 : (index + delta + items.length) % items.length];
                if (next) next.focus();
                break;
            }
            case 'Home': event.preventDefault(); if (items[0]) items[0].focus(); break;
            case 'End': event.preventDefault(); if (items.length) items[items.length - 1].focus(); break;
            case 'ArrowRight': if (remove) { event.preventDefault(); remove.focus(); } break;
            case 'ArrowLeft': if (active !== current && current) { event.preventDefault(); current.focus(); } break;
            case 'Delete':
            case 'Backspace': if (remove) { event.preventDefault(); remove.click(); } break;
            case 'Escape': event.preventDefault(); event.stopPropagation(); cleanup(true); break;
            case 'Tab': event.preventDefault(); cleanup(true); break;
            default: break;
        }
    }

    function workspaceMenuAction(action) {
        if (action === 'connect') openConnectDialog();
        else if (action === 'reconnect') workspaceReconnect();
        else if (action === 'disconnect') switchWorkspace(WS_MODEL.LOCAL_ID);
    }

    async function workspaceRemove(id) {
        const target = state;
        const ws = (target.workspaces || []).find(item => item.id === id);
        if (!ws) return;
        const confirmed = await confirmValue(tr('codeStudio.workspaceRemoveConfirm', 'Remove {{workspace}} from the list? The files on the host stay untouched.', { workspace: workspaceDisplayLabel(ws) }), { confirmLabel: tr('codeStudio.workspaceRemove', 'Remove'), confirmIcon: 'trash' });
        if (!confirmed || !isLiveInstance(target)) return;
        if (target.workspaceId === id && !await runWithInstance(target, () => switchWorkspace(WS_MODEL.LOCAL_ID))) return;
        try {
            await apiClient.deleteWorkspace(id);
        } catch (err) {
            if (!isLiveInstance(target)) return;
            if (err.status !== 404) {
                runWithInstance(target, () => workspaceToast(workspaceErrorText('other', { error: err.message })));
                return;
            }
        }
        if (!isLiveInstance(target)) return;
        WS_MODEL.forgetSession(localStorage, id);
        target.workspaces = (target.workspaces || []).filter(item => item.id !== id);
        runWithInstance(target, workspaceRenderChrome);
    }

    function openWorkspaceSwitcher() {
        const switcher = shellPart('[data-ws-switcher]');
        const anchor = switcher && state.sidebarVisible && switcher.getClientRects().length ? switcher : shellPart('[data-status-remote]');
        if (anchor) openWorkspaceMenu(anchor);
    }
```

- [ ] **Step 2:** Run `npm run build:ui`, then `go test ./ui/ -run '(?i)CodeStudio' -count=1`.
  Expected: `ok`.
- [ ] **Step 3:** Commit `workspace.js` with the message `feat(code-studio): workspace menu with recent SSH folders and keyboard navigation`.

---

### Task 9: Boot, switching, reconnect (`workspace.js` part D)

**Files:** Append to `workspace.js`; modify `core.js` (`render`) and `terminal.js` (`onclose`)

- [ ] **Step 1: Impact.** Run impact for `render` (callers: `window.CodeStudioApp.render`, the desktop router and `renderError` retry) and `connectTerminalSession`.
- [ ] **Step 2:** Append part D:

```js
    // --- Boot, connect, reconnect --------------------------------------------------------------

    async function workspaceBoot(instance) {
        const target = instance || state;
        if (!isLiveInstance(target)) return;
        addDisposer(target, () => workspaceClearRetry(target));
        const wanted = WS_MODEL.readActive(localStorage);
        await workspaceLoadList(target);
        if (!isLiveInstance(target)) return;
        const saved = wanted !== WS_MODEL.LOCAL_ID ? (target.workspaces || []).find(ws => ws.id === wanted) : null;
        runWithInstance(target, () => {
            if (saved) {
                target.workspaceId = saved.id;
                target.workspace = saved;
                target.workspaceStatus = 'connecting';
            } else if (wanted !== WS_MODEL.LOCAL_ID) {
                WS_MODEL.writeActive(localStorage, WS_MODEL.LOCAL_ID);
            }
            workspaceApplySession(target.workspaceId);
            renderTabs();
            workspaceRenderChrome();
        });
        if (saved) await workspaceConnectActive(target);
    }

    async function workspaceConnectActive(target, options = {}) {
        const id = target.workspaceId;
        if (!WS_MODEL.isRemote(id)) return true;
        runWithInstance(target, () => { target.workspaceStatus = 'connecting'; workspaceRenderChrome(); });
        try {
            const payload = await apiClient.connectWorkspace(id, options.trust || '');
            if (!isLiveInstance(target) || target.workspaceId !== id) return false;
            runWithInstance(target, () => {
                target.workspace = WS_MODEL.mergeWorkspace(target.workspace, payload);
                target.workspaceStatus = 'connected';
                target.workspaceError = null;
                workspaceRenderChrome();
                renderGitPanel();
            });
            return true;
        } catch (err) {
            if (isLiveInstance(target) && target.workspaceId === id) runWithInstance(target, () => workspaceConnectFailed(err));
            return false;
        }
    }

    function workspaceConnectFailed(err) {
        const instance = state;
        const kind = WS_MODEL.classifyError(err);
        if (kind === 'workspace_not_found') {
            workspaceForgetMissing();
            return;
        }
        instance.workspaceError = { kind, message: err && err.message ? err.message : String(err || '') };
        instance.workspaceWasLost = true;
        if (kind === 'unknown_host_key') {
            instance.workspaceStatus = 'hostkey';
            workspaceClearRetry(instance);
        } else if (kind === 'host_key_mismatch' || kind === 'auth') {
            instance.workspaceStatus = 'error';
            workspaceClearRetry(instance);
        } else {
            instance.workspaceStatus = 'lost';
            workspaceScheduleRetry();
        }
        workspaceRenderChrome();
    }

    function workspaceMarkLost(err) {
        const instance = state;
        if (instance.workspaceSwitching || ['lost', 'connecting', 'hostkey', 'error'].includes(instance.workspaceStatus)) return;
        workspaceConnectFailed(err);
    }

    function workspaceClearRetry(instance = state, keepAttempt = false) {
        if (!instance) return;
        if (instance.workspaceRetryTimer) clearTimeout(instance.workspaceRetryTimer);
        if (instance.workspaceRetryTick) clearInterval(instance.workspaceRetryTick);
        instance.workspaceRetryTimer = 0;
        instance.workspaceRetryTick = 0;
        instance.workspaceRetryAt = 0;
        if (!keepAttempt) instance.workspaceRetryAttempt = 0;
    }

    function workspaceScheduleRetry() {
        const instance = state;
        workspaceClearRetry(instance, true);
        const attempt = instance.workspaceRetryAttempt || 0;
        const delay = WS_MODEL.backoffDelay(attempt);
        instance.workspaceRetryAt = Date.now() + delay;
        instance.workspaceRetryTick = setInterval(bindInstance(instance, workspaceRenderBanner), 1000);
        instance.workspaceRetryTimer = setTimeout(bindInstance(instance, () => {
            workspaceClearRetry(instance, true);
            instance.workspaceRetryAttempt = attempt + 1;
            workspaceReconnect({ auto: true });
        }), delay);
    }

    async function workspaceReconnect(options = {}) {
        const target = state;
        if (!workspaceIsRemote(target) || target.workspaceStatus === 'connecting' || target.workspaceSwitching) return false;
        if (!options.auto) {
            workspaceClearRetry(target);
            target.workspaceWasLost = true; // a manual reconnect refreshes everything
        }
        const ok = await workspaceConnectActive(target, options);
        if (ok && isLiveInstance(target)) runWithInstance(target, workspaceAfterReconnect);
        return ok;
    }

    function workspaceAfterReconnect() {
        const instance = state;
        const wasLost = instance.workspaceWasLost;
        instance.workspaceWasLost = false;
        workspaceClearRetry(instance);
        workspaceRenderChrome();
        if (!wasLost) return;
        flashStatus(tr('codeStudio.workspaceReconnected', 'Reconnected to {{host}}', { host: workspaceHostLabel() }));
        refreshFiles(instance.currentPath);
        scheduleGitRefresh();
        const open = typeof WebSocket !== 'undefined' ? WebSocket.OPEN : 1;
        if (canWrite() && (instance.terminalSessions || []).every(session => !session.ws || session.ws.readyState !== open)) {
            closeTerminalSessionSockets(instance);
            renderTerminal();
            connectTerminal();
        }
        if (instance.lsp) {
            Object.keys(instance.lsp.connections).forEach(language => {
                const conn = instance.lsp.connections[language];
                if (conn.state === 'error' || conn.state === 'disconnected') lspRestart(language);
            });
        }
        instance.openTabs.forEach(tab => reloadCleanTabFromDisk(instance, tab));
    }

    // A remote shell that closes may mean the SSH connection broke: one connect call tells (and redials).
    function workspaceTerminalClosed() {
        const instance = state;
        if (!workspaceIsRemote(instance) || instance.workspaceStatus !== 'connected' || instance.workspaceSwitching) return;
        const id = instance.workspaceId;
        apiClient.connectWorkspace(id).then(payload => {
            if (isLiveInstance(instance) && instance.workspaceId === id) instance.workspace = WS_MODEL.mergeWorkspace(instance.workspace, payload);
        }, err => {
            if (isLiveInstance(instance) && instance.workspaceId === id) runWithInstance(instance, () => workspaceConnectFailed(err));
        });
    }

    function workspaceForgetMissing() {
        const instance = state;
        if (instance.workspaceForgetting) return;
        instance.workspaceForgetting = true;
        const id = instance.workspaceId;
        instance.workspaces = (instance.workspaces || []).filter(ws => ws.id !== id);
        workspaceToast(tr('codeStudio.workspaceMissing', 'This SSH workspace no longer exists. Code Studio switched to the local workspace.'));
        switchWorkspace(WS_MODEL.LOCAL_ID).then(ok => {
            if (ok) WS_MODEL.forgetSession(localStorage, id);
        }).finally(() => { instance.workspaceForgetting = false; });
    }

    // --- Switching -----------------------------------------------------------------------------

    function chooseWorkspaceSwitch(count, label) {
        return new Promise(resolve => {
            const instance = state;
            const overlay = document.createElement('div');
            overlay.className = 'cs-modal-backdrop';
            overlay.innerHTML = `<div class="cs-modal cs-ws-dirty" role="alertdialog" aria-modal="true" aria-labelledby="cs-ws-dirty-title" aria-describedby="cs-ws-dirty-text">
                <h2 class="cs-ws-dialog-title" id="cs-ws-dirty-title">${esc(tr('codeStudio.workspaceDirtyTitle', 'Unsaved changes'))}</h2>
                <p id="cs-ws-dirty-text">${esc(tr('codeStudio.workspaceDirtyPrompt', '{{count}} files have unsaved changes. Save them before switching to {{workspace}}?', { count, workspace: label }))}</p>
                <div class="cs-modal-actions">
                    <button type="button" class="cs-button" data-choice="cancel">${esc(tr('desktop.cancel', 'Cancel'))}</button>
                    <button type="button" class="cs-button danger" data-choice="discard">${esc(tr('codeStudio.discard', 'Discard'))}</button>
                    <button type="button" class="cs-button primary" data-choice="save"${canWrite(instance) ? '' : ' disabled'}>${esc(tr('codeStudio.saveAll', 'Save All'))}</button>
                </div>
            </div>`;
            document.body.appendChild(overlay);
            const release = trapDialogFocus(overlay);
            let settled = false;
            let unregister = () => {};
            const done = choice => {
                if (settled) return;
                settled = true;
                unregister();
                overlay.remove();
                release();
                resolve(choice);
            };
            runWithInstance(instance, () => { unregister = registerDisposer(() => done('cancel')); });
            overlay.querySelectorAll('[data-choice]').forEach(button => button.addEventListener('click', () => done(button.dataset.choice)));
            overlay.addEventListener('click', event => { if (event.target === overlay) done('cancel'); });
            overlay.addEventListener('keydown', event => {
                if (event.key !== 'Escape') return;
                event.preventDefault();
                event.stopPropagation();
                done('cancel');
            });
            (overlay.querySelector('[data-choice="save"]:not([disabled])') || overlay.querySelector('[data-choice="cancel"]')).focus();
        });
    }

    // Dirty tabs ask Save all / Discard / Cancel; pending saves finish before the workspace id changes.
    async function workspaceSettleTabs(target, next) {
        const dirty = target.openTabs.filter(tab => tab.modified && !isVirtualTab(tab));
        if (dirty.length) {
            const choice = await runWithInstance(target, () => chooseWorkspaceSwitch(dirty.length, workspaceDisplayLabel(next)));
            if (choice === 'cancel' || !isLiveInstance(target)) return false;
            if (choice === 'save' && !await runWithInstance(target, () => saveAllFiles())) {
                if (isLiveInstance(target)) runWithInstance(target, () => workspaceToast(tr('codeStudio.workspaceSwitchFailed', 'Could not open {{workspace}}: {{error}}', { workspace: workspaceDisplayLabel(next), error: tr('codeStudio.unsavedChanges', 'Unsaved changes') })));
                return false;
            }
        }
        await Promise.all(target.openTabs.map(tab => tab.pendingSave).filter(Boolean));
        return isLiveInstance(target) && !target.openTabs.some(tab => tab.pathMutation);
    }

    async function switchWorkspace(id, options = {}) {
        const target = state;
        if (!isLiveInstance(target) || target.workspaceSwitching) return false;
        const nextId = WS_MODEL.normalizeId(id);
        if (nextId === target.workspaceId) {
            // Same workspace (a host-key review ended in the dialog): take the fresh data and refresh.
            if (options.workspace && WS_MODEL.isRemote(nextId)) {
                target.workspace = Object.assign({}, target.workspace, options.workspace, { connected: true });
                target.workspaceStatus = 'connected';
                target.workspaceError = null;
                target.workspaceWasLost = true;
                runWithInstance(target, workspaceAfterReconnect);
            }
            return true;
        }
        let next = nextId === WS_MODEL.LOCAL_ID ? WS_MODEL.localWorkspace() : (options.workspace || (target.workspaces || []).find(ws => ws.id === nextId));
        if (!next) return false;
        target.workspaceSwitching = true;
        try {
            if (!await workspaceSettleTabs(target, next)) return false;
            if (next.kind === 'ssh' && !options.connected) {
                runWithInstance(target, () => { target.workspaceSwitchTarget = next.id; workspaceRenderChrome(); });
                try {
                    next = WS_MODEL.mergeWorkspace(next, await apiClient.connectWorkspace(next.id, options.trust || ''));
                } catch (err) {
                    if (!isLiveInstance(target)) return false;
                    const kind = WS_MODEL.classifyError(err);
                    const failed = next;
                    runWithInstance(target, () => {
                        if (kind === 'unknown_host_key') openConnectDialog({ deviceId: failed.deviceId, path: failed.root });
                        else workspaceToast(tr('codeStudio.workspaceSwitchFailed', 'Could not open {{workspace}}: {{error}}', { workspace: workspaceDisplayLabel(failed), error: workspaceErrorText(kind, { host: workspaceHostLabel(failed), path: failed.root, error: err.message || String(err) }) }));
                    });
                    return false;
                } finally {
                    if (isLiveInstance(target)) runWithInstance(target, () => { target.workspaceSwitchTarget = ''; workspaceRenderChrome(); });
                }
            }
            if (!isLiveInstance(target)) return false;
            runWithInstance(target, () => workspaceActivate(next));
            await runAsyncStep(target, () => refreshFiles(WORKSPACE_ROOT));
            if (!isLiveInstance(target)) return false;
            await runAsyncStep(target, restoreTabs);
            if (!isLiveInstance(target)) return false;
            runWithInstance(target, () => { connectTerminal(); scheduleGitRefresh(); });
            return true;
        } finally {
            if (isLiveInstance(target)) target.workspaceSwitching = false;
        }
    }

    // Tears the old workspace down (explorer, search, git, problems, terminals, LSP, tabs) and applies the new session.
    function workspaceActivate(next) {
        const instance = state;
        saveState(); // the leaving workspace's session
        closeWorkspaceMenu(instance);
        // Requests of the old workspace that are still in flight must not land in the new one.
        instance.treeRequest = instance.openRequest = instance.pathRequest = instance.gitRequest = Symbol('workspace');
        if (instance.agentAbortController) instance.agentAbortController.abort();
        closeTerminalSessionSockets(instance);
        lspShutdownInstance(instance);
        if (instance.findOpen) closeFindWidget();
        instance.openTabs.forEach(tab => destroyTabView(tab, true));
        instance.openTabs = [];
        instance.activeTabIndex = -1;
        resetSplitState();
        instance.files = [];
        instance.treeCache = {};
        instance.expandedDirs.clear();
        instance.currentPath = instance.selectedDir = WORKSPACE_ROOT;
        instance.searchResults = [];
        instance.searchPerformed = false;
        instance.searchTruncated = false;
        instance.gitIsRepo = null;
        instance.gitBranch = '';
        instance.gitChanges = [];
        instance.gitLog = [];
        instance.gitHistory = [];
        instance.gitCommitMessage = '';
        instance.outputLines = [];
        instance.workspaceId = next.id;
        instance.workspace = next.kind === 'ssh' ? next : null;
        instance.workspaceStatus = next.kind === 'ssh' ? 'connected' : 'local';
        instance.workspaceError = null;
        instance.workspaceWasLost = false;
        workspaceClearRetry(instance);
        WS_MODEL.writeActive(localStorage, next.id);
        workspaceApplySession(next.id);
        setProblems([]);
        renderTabs();
        renderBreadcrumbs();
        renderEditor();
        renderSearchPanel();
        renderGitPanel();
        renderTerminal();
        renderBottomPanel();
        renderActivityBar();
        renderWindowMenus();
        workspaceRenderChrome();
        saveState();
    }
```

- [ ] **Step 3: `core.js` `render()`.** Between the `setWindowBeforeClose` block and `await runAsyncStep(instance, () => refreshFiles(instance.currentPath));`, insert:

```js
            // Plan C: pick the workspace (local or this browser's last SSH workspace) and connect it.
            await runAsyncStep(instance, workspaceBoot);
            if (!isLiveInstance(instance)) return;
```

- [ ] **Step 4: `terminal.js`.** In `ws.onclose`, after the line `terminalSessionStatus(session, 'codeStudio.stopped');`, add:

```js
                workspaceTerminalClosed(); // Plan C: a closed remote shell may mean a broken SSH connection
```

  Disposed sessions null `onclose` first, so a switch or a window close never triggers this.
- [ ] **Step 5:** Run `npm run build:ui`, then `go test ./ui/ -run '(?i)CodeStudio' -count=1`.
  Expected: `ok`.
- [ ] **Step 6:** Run detect-changes, then commit `workspace.js`, `core.js` and `terminal.js` with the message `feat(code-studio): workspace boot, safe switching and backoff reconnects`.

---

### Task 10: The connect dialog (`workspace.js` part E)

**Files:** Append to `workspace.js`. This is one task with two steps and one commit, because the two halves reference each other.

- [ ] **Step 1:** Append the shell, the hosts pane and the new-host form:

```js
    // --- Connect dialog: shell, hosts, new host -------------------------------------------------

    const WSD_FOCUS_ATTRS = ['data-wsd-home', 'data-wsd-up', 'data-wsd-refresh', 'data-wsd-path', 'data-wsd-trust', 'data-wsd-trust-cancel', 'data-wsd-retry', 'data-wsd-other-host'];

    function wsdSkeleton(count) {
        return Array.from({ length: count }, () => '<div class="cs-wsd-skeleton" aria-hidden="true"></div>').join('');
    }

    function openConnectDialog(options = {}) {
        const instance = state;
        if (!isLiveInstance(instance)) return Promise.resolve(false);
        if (instance.workspaceDialog) {
            instance.workspaceDialog.focusFirst();
            return instance.workspaceDialog.promise;
        }
        closeWorkspaceMenu(instance);
        const dialog = {
            instance, devices: [], devicesLoading: true, devicesError: '', query: '', deviceId: '', form: null,
            home: {}, trusted: {}, browse: null, opening: false, openError: '',
            initial: { deviceId: String(options.deviceId || ''), path: String(options.path || '') }
        };
        const overlay = document.createElement('div');
        overlay.className = 'cs-modal-backdrop cs-wsd-backdrop';
        overlay.innerHTML = `<div class="cs-modal cs-wsd" role="dialog" aria-modal="true" aria-labelledby="cs-wsd-title">
            <header class="cs-wsd-head">
                <h2 class="cs-ws-dialog-title" id="cs-wsd-title">${esc(tr('codeStudio.workspaceDialogTitle', 'Connect to SSH Folder'))}</h2>
                <button type="button" class="cs-icon-button" data-wsd-close aria-label="${esc(tr('desktop.close', 'Close'))}" title="${esc(tr('desktop.close', 'Close'))}">${wsGlyph('x', 14)}</button>
            </header>
            <div class="cs-wsd-body">
                <section class="cs-wsd-hosts" aria-labelledby="cs-wsd-hosts-title">
                    <div class="cs-wsd-pane-title" id="cs-wsd-hosts-title">${esc(tr('codeStudio.workspaceHosts', 'Hosts'))}</div>
                    <div class="cs-wsd-hosts-main" data-wsd-hosts-main>
                        <input type="search" class="cs-wsd-search" data-wsd-search placeholder="${esc(tr('codeStudio.workspaceSearchHosts', 'Search hosts'))}" aria-label="${esc(tr('codeStudio.workspaceSearchHosts', 'Search hosts'))}" aria-controls="cs-wsd-hostlist" autocomplete="off" spellcheck="false">
                        <div class="cs-wsd-hostlist" id="cs-wsd-hostlist" role="listbox" aria-labelledby="cs-wsd-hosts-title" data-wsd-hostlist></div>
                        <button type="button" class="cs-button cs-wsd-new-host" data-wsd-new-host>${wsGlyph('plus', 14)}<span>${esc(tr('codeStudio.workspaceNewHost', 'New SSH Host'))}</span></button>
                    </div>
                    <div class="cs-wsd-form-slot" data-wsd-form-slot hidden></div>
                </section>
                <section class="cs-wsd-browser" aria-label="${esc(tr('codeStudio.workspacePathLabel', 'Folder path'))}" data-wsd-browser></section>
            </div>
            <footer class="cs-wsd-footer" data-wsd-footer></footer>
        </div>`;
        document.body.appendChild(overlay);
        const release = trapDialogFocus(overlay);
        let settled = false;
        let unregister = () => {};
        let resolvePromise = () => {};
        dialog.promise = new Promise(resolve => { resolvePromise = resolve; });
        dialog.overlay = overlay;
        dialog.close = result => {
            if (settled) return;
            settled = true;
            unregister();
            dialog.form = null; // drops any typed secret
            overlay.remove();
            release();
            if (instance.workspaceDialog === dialog) instance.workspaceDialog = null;
            resolvePromise(result);
        };
        dialog.isOpen = () => !settled && isLiveInstance(instance);
        dialog.focusFirst = () => {
            const search = overlay.querySelector('[data-wsd-search]');
            if (search && !search.closest('[hidden]')) search.focus();
        };
        instance.workspaceDialog = dialog;
        unregister = registerDisposer(() => dialog.close(false));
        wireConnectDialog(dialog);
        renderDialogHosts(dialog);
        renderDialogBrowser(dialog);
        renderDialogFooter(dialog);
        dialog.focusFirst();
        dialogLoadDevices(dialog);
        return dialog.promise;
    }

    function wireConnectDialog(dialog) {
        const overlay = dialog.overlay;
        const on = (type, handler) => overlay.addEventListener(type, bindInstance(dialog.instance, handler));
        on('mousedown', event => { dialog.backdropDown = event.target === overlay; });
        on('click', event => {
            if (event.target === overlay) {
                if (dialog.backdropDown) dialog.close(false);
                return;
            }
            const el = event.target.closest('[data-wsd-close],[data-wsd-cancel],[data-wsd-new-host],[data-wsd-host],[data-wsd-entry],[data-wsd-crumb],[data-wsd-home],[data-wsd-up],[data-wsd-refresh],[data-wsd-goto],[data-wsd-trust],[data-wsd-trust-cancel],[data-wsd-retry],[data-wsd-other-host],[data-wsd-open],[data-wsd-form-cancel],[data-wsd-key-file],[data-wsd-copy]');
            if (!el || el.disabled || el.getAttribute('aria-disabled') === 'true') return;
            const d = el.dataset;
            if ('wsdClose' in d || 'wsdCancel' in d) dialog.close(false);
            else if ('wsdNewHost' in d) dialogOpenForm(dialog);
            else if ('wsdHost' in d) dialogSelectDevice(dialog, d.wsdHost, {});
            else if ('wsdEntry' in d) dialogSelectEntry(dialog, d.wsdEntry, {});
            else if ('wsdCrumb' in d) dialogBrowse(dialog, d.wsdCrumb, {});
            else if ('wsdHome' in d) dialogBrowse(dialog, '', {});
            else if ('wsdUp' in d) dialogBrowse(dialog, WS_MODEL.remoteParent(dialog.browse.path), {});
            else if ('wsdRefresh' in d) dialogBrowse(dialog, dialog.browse.path, {});
            else if ('wsdGoto' in d) dialogBrowse(dialog, d.wsdGoto, {});
            else if ('wsdTrust' in d) dialogTrust(dialog);
            else if ('wsdTrustCancel' in d || 'wsdOtherHost' in d) dialogClearDevice(dialog);
            else if ('wsdRetry' in d) dialogRetry(dialog);
            else if ('wsdOpen' in d) dialogOpen(dialog, {});
            else if ('wsdFormCancel' in d) dialogCloseForm(dialog);
            else if ('wsdKeyFile' in d) { const input = overlay.querySelector('[data-wsd-key-input]'); if (input) input.click(); }
            else if ('wsdCopy' in d) copyTextToClipboard(d.wsdCopy);
        });
        on('dblclick', event => {
            const entry = event.target.closest('[data-wsd-entry][data-dir="true"]');
            if (entry) dialogBrowse(dialog, entry.dataset.wsdEntry, { focus: 'list' });
        });
        on('input', event => {
            if (event.target.matches('[data-wsd-search]')) {
                dialog.query = event.target.value;
                renderDialogHosts(dialog);
            } else if (event.target.closest('[data-wsd-form]')) {
                dialogFormInput(dialog, event.target);
            }
        });
        on('change', event => {
            if (event.target.matches('[data-wsd-auth]')) dialogFormAuth(dialog);
            else if (event.target.matches('[data-wsd-key-input]')) dialogLoadKeyFile(dialog, event.target);
        });
        on('submit', event => {
            event.preventDefault();
            if (event.target.matches('[data-wsd-pathform]')) dialogSubmitPath(dialog, event.target);
            else if (event.target.matches('[data-wsd-form]')) dialogSubmitForm(dialog, event.target);
        });
        on('keydown', event => dialogKeydown(dialog, event));
    }

    function dialogKeydown(dialog, event) {
        const target = event.target;
        if (event.key === 'Escape') {
            event.preventDefault();
            event.stopPropagation();
            if (dialog.form) dialogCloseForm(dialog);
            else dialog.close(false);
            return;
        }
        if (event.key === 'Enter' && (event.ctrlKey || event.metaKey) && !dialog.form) {
            event.preventDefault();
            dialogOpen(dialog, {});
            return;
        }
        if (target.closest('[data-wsd-hostlist]')) { dialogHostKeydown(dialog, event); return; }
        if (target.closest('[data-wsd-folders]')) { dialogFolderKeydown(dialog, event); return; }
        if (target.matches('[data-wsd-search]')) {
            const first = dialog.overlay.querySelector('[data-wsd-host]');
            if (event.key === 'ArrowDown' && first) { event.preventDefault(); first.focus(); }
            else if (event.key === 'Enter' && !event.isComposing) {
                event.preventDefault();
                if (first) dialogSelectDevice(dialog, first.dataset.wsdHost, { focus: 'list' });
            }
            return;
        }
        if (event.key !== 'Enter' || event.isComposing) return;
        if (target.closest('form, textarea, button, a, input, [role="option"]')) return;
        event.preventDefault();
        dialogOpen(dialog, {}); // Enter anywhere else opens the folder
    }

    function dialogHostKeydown(dialog, event) {
        const options = Array.from(dialog.overlay.querySelectorAll('[data-wsd-host]'));
        const index = options.indexOf(document.activeElement);
        let next = null;
        if (event.key === 'ArrowDown') next = options[Math.min(options.length - 1, index + 1)];
        else if (event.key === 'ArrowUp') {
            if (index <= 0) {
                event.preventDefault();
                dialog.overlay.querySelector('[data-wsd-search]').focus();
                return;
            }
            next = options[index - 1];
        } else if (event.key === 'Home') next = options[0];
        else if (event.key === 'End') next = options[options.length - 1];
        else if (['Enter', ' ', 'ArrowRight'].includes(event.key)) {
            if (index >= 0) {
                event.preventDefault();
                dialogSelectDevice(dialog, options[index].dataset.wsdHost, { focus: 'list' });
            }
            return;
        } else return;
        event.preventDefault();
        if (next) {
            options.forEach(option => { option.tabIndex = -1; });
            next.tabIndex = 0;
            next.focus();
        }
    }

    function dialogDevice(dialog) {
        return dialog.devices.find(device => device.id === dialog.deviceId) || null;
    }

    async function dialogLoadDevices(dialog) {
        dialog.devicesLoading = true;
        dialog.devicesError = '';
        renderDialogHosts(dialog);
        try {
            const [payload] = await Promise.all([apiClient.remoteDevices(), workspaceLoadList(dialog.instance)]);
            if (!dialog.isOpen()) return;
            dialog.devices = WS_MODEL.sortDevices(WS_MODEL.normalizeDeviceList(payload), dialog.instance.workspaces || []);
        } catch (err) {
            if (!dialog.isOpen()) return;
            dialog.devicesError = workspaceErrorText('other', { error: err.message || String(err) });
        }
        dialog.devicesLoading = false;
        runWithInstance(dialog.instance, () => {
            renderDialogHosts(dialog);
            const initial = dialog.initial.deviceId;
            if (initial && dialog.devices.some(device => device.id === initial)) dialogSelectDevice(dialog, initial, { path: dialog.initial.path });
        });
    }

    function renderDialogHosts(dialog) {
        const overlay = dialog.overlay;
        const list = overlay.querySelector('[data-wsd-hostlist]');
        overlay.querySelector('[data-wsd-hosts-main]').hidden = !!dialog.form;
        overlay.querySelector('[data-wsd-form-slot]').hidden = !dialog.form;
        const newHost = overlay.querySelector('[data-wsd-new-host]');
        newHost.disabled = !!dialog.instance.readOnly;
        newHost.title = dialog.instance.readOnly ? tr('codeStudio.workspaceFormReadOnly', 'The desktop is read-only: new hosts cannot be added.') : '';
        if (dialog.devicesLoading) {
            list.setAttribute('aria-busy', 'true');
            list.innerHTML = wsdSkeleton(3);
            return;
        }
        list.removeAttribute('aria-busy');
        if (dialog.devicesError) {
            list.innerHTML = `<div class="cs-wsd-empty" role="alert"><span>${esc(dialog.devicesError)}</span><button type="button" class="cs-button" data-wsd-retry>${esc(tr('desktop.retry', 'Retry'))}</button></div>`;
            return;
        }
        if (!dialog.devices.length) {
            list.innerHTML = `<div class="cs-wsd-empty">${esc(tr('codeStudio.workspaceNoHosts', 'No SSH hosts in the inventory yet.'))}</div>`;
            return;
        }
        const devices = WS_MODEL.filterDevices(dialog.devices, dialog.query);
        if (!devices.length) {
            list.innerHTML = `<div class="cs-wsd-empty" data-wsd-no-match>${esc(tr('codeStudio.workspaceNoHostMatch', 'No host matches “{{query}}”', { query: dialog.query.trim() }))}</div>`;
            return;
        }
        const hadFocus = list.contains(document.activeElement) ? document.activeElement.dataset.wsdHost : '';
        const focusId = devices.some(device => device.id === dialog.deviceId) ? dialog.deviceId : devices[0].id;
        list.innerHTML = devices.map(device => {
            const selected = device.id === dialog.deviceId;
            return `<div class="cs-wsd-host${selected ? ' is-selected' : ''}" role="option" data-wsd-host="${esc(device.id)}" aria-selected="${selected}" tabindex="${device.id === focusId ? 0 : -1}">
                <span class="cs-wsd-host-icon">${wsGlyph('server', 16)}</span>
                <span class="cs-wsd-host-text"><span class="cs-wsd-host-name">${esc(device.name)}</span><span class="cs-wsd-host-addr">${esc(WS_MODEL.deviceAddress(device))}</span></span>
                <span class="cs-wsd-host-used">${esc(workspaceLastUsedText(WS_MODEL.deviceLastUsed(device, dialog.instance.workspaces || [])))}</span>
            </div>`;
        }).join('');
        if (hadFocus) {
            const again = Array.from(list.querySelectorAll('[data-wsd-host]')).find(option => option.dataset.wsdHost === hadFocus);
            if (again) again.focus();
        }
    }

    function dialogSelectDevice(dialog, id, options) {
        if (!dialog.devices.some(device => device.id === id)) return;
        const changed = dialog.deviceId !== id;
        dialog.deviceId = id;
        dialog.openError = '';
        renderDialogHosts(dialog);
        if (!changed && dialog.browse && !dialog.browse.error && !dialog.browse.hostKey) {
            if (options.focus === 'list') dialogFocusFolders(dialog);
            return;
        }
        dialog.browse = null;
        dialogBrowse(dialog, options.path || '', { focus: options.focus });
    }

    function dialogClearDevice(dialog) {
        dialog.deviceId = '';
        dialog.browse = null;
        dialog.openError = '';
        renderDialogHosts(dialog);
        renderDialogBrowser(dialog);
        renderDialogFooter(dialog);
        const option = dialog.overlay.querySelector('[data-wsd-host][tabindex="0"]');
        if (option) option.focus();
        else dialog.focusFirst();
    }

    function dialogRetry(dialog) {
        if (dialog.devicesError) {
            dialogLoadDevices(dialog);
            return;
        }
        const retry = dialog.browse && dialog.browse.error && dialog.browse.error.retry;
        if (retry) retry('');
    }

    function dialogOpenForm(dialog) {
        if (dialog.instance.readOnly) return;
        dialog.form = { values: { name: '', host: '', port: '22', username: '', auth: 'password', secret: '', passphrase: '' }, errors: {}, busy: false, error: '' };
        renderDialogHosts(dialog);
        renderDialogForm(dialog);
        const name = dialog.overlay.querySelector('[data-wsd-form] [name="name"]');
        if (name) name.focus();
    }

    function dialogCloseForm(dialog) {
        dialog.form = null;
        dialog.overlay.querySelector('[data-wsd-form-slot]').innerHTML = '';
        renderDialogHosts(dialog);
        dialog.overlay.querySelector('[data-wsd-new-host]').focus();
    }

    function readDialogForm(formEl) {
        const data = new FormData(formEl);
        const get = name => String(data.get(name) || '');
        return { name: get('name'), host: get('host'), port: get('port'), username: get('username'), auth: get('auth') || 'password', secret: get('secret'), passphrase: get('passphrase') };
    }

    function renderDialogForm(dialog) {
        const slot = dialog.overlay.querySelector('[data-wsd-form-slot]');
        const form = dialog.form;
        if (!slot || !form) return;
        const v = form.values;
        const e = form.errors;
        const attrs = name => `id="cs-wsd-f-${name}" name="${name}" aria-invalid="${e[name] ? 'true' : 'false'}"${e[name] ? ` aria-describedby="cs-wsd-e-${name}"` : ''}`;
        const msg = name => (e[name] ? `<span class="cs-wsd-field-error" id="cs-wsd-e-${name}">${esc(workspaceFormErrorText(e[name]))}</span>` : '');
        const key = v.auth === 'key';
        slot.innerHTML = `<form class="cs-wsd-form" data-wsd-form novalidate aria-labelledby="cs-wsd-form-title">
            <div class="cs-wsd-pane-title" id="cs-wsd-form-title">${esc(tr('codeStudio.workspaceNewHost', 'New SSH Host'))}</div>
            <div class="cs-wsd-field"><label for="cs-wsd-f-name">${esc(tr('codeStudio.workspaceFormName', 'Name'))}</label><input ${attrs('name')} value="${esc(v.name)}" autocomplete="off" maxlength="80" placeholder="build-box">${msg('name')}</div>
            <div class="cs-wsd-row">
                <div class="cs-wsd-field"><label for="cs-wsd-f-host">${esc(tr('codeStudio.workspaceFormHost', 'Host'))}</label><input ${attrs('host')} value="${esc(v.host)}" autocomplete="off" spellcheck="false" placeholder="192.168.1.20">${msg('host')}</div>
                <div class="cs-wsd-field"><label for="cs-wsd-f-port">${esc(tr('codeStudio.workspaceFormPort', 'Port'))}</label><input ${attrs('port')} value="${esc(v.port)}" inputmode="numeric" autocomplete="off">${msg('port')}</div>
            </div>
            <div class="cs-wsd-field"><label for="cs-wsd-f-username">${esc(tr('codeStudio.workspaceFormUser', 'User'))}</label><input ${attrs('username')} value="${esc(v.username)}" autocomplete="off" spellcheck="false" placeholder="pi">${msg('username')}</div>
            <fieldset class="cs-wsd-auth"><legend>${esc(tr('codeStudio.workspaceFormAuth', 'Sign in with'))}</legend>
                <label><input type="radio" name="auth" value="password" data-wsd-auth${key ? '' : ' checked'}><span>${esc(tr('codeStudio.workspaceFormPassword', 'Password'))}</span></label>
                <label><input type="radio" name="auth" value="key" data-wsd-auth${key ? ' checked' : ''}><span>${esc(tr('codeStudio.workspaceFormKey', 'Private key'))}</span></label>
            </fieldset>
            ${key ? `<div class="cs-wsd-field"><label for="cs-wsd-f-secret">${esc(tr('codeStudio.workspaceFormKey', 'Private key'))}</label><textarea ${attrs('secret')} rows="4" spellcheck="false" autocomplete="off" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----">${esc(v.secret)}</textarea>${msg('secret')}
                    <button type="button" class="cs-button cs-wsd-keyfile" data-wsd-key-file>${esc(tr('codeStudio.workspaceFormKeyFile', 'Load Key File…'))}</button><input type="file" data-wsd-key-input hidden tabindex="-1"></div>
                <div class="cs-wsd-field"><label for="cs-wsd-f-passphrase">${esc(tr('codeStudio.workspaceFormPassphrase', 'Passphrase (optional)'))}</label><input type="password" ${attrs('passphrase')} value="${esc(v.passphrase)}" autocomplete="off"></div>`
            : `<div class="cs-wsd-field"><label for="cs-wsd-f-secret">${esc(tr('codeStudio.workspaceFormPassword', 'Password'))}</label><input type="password" ${attrs('secret')} value="${esc(v.secret)}" autocomplete="new-password">${msg('secret')}</div>`}
            <p class="cs-wsd-field-hint">${esc(tr('codeStudio.workspaceFormVaultHint', 'The secret is stored in the AuraGo vault and never kept in the browser.'))}</p>
            ${form.error ? `<div class="cs-wsd-open-error" role="alert">${wsGlyph('warning', 14)}<span>${esc(form.error)}</span></div>` : ''}
            <div class="cs-wsd-actions">
                <button type="button" class="cs-button" data-wsd-form-cancel>${esc(tr('desktop.cancel', 'Cancel'))}</button>
                <button type="submit" class="cs-button primary"${form.busy ? ' disabled aria-busy="true"' : ''}>${esc(tr('codeStudio.workspaceFormAdd', 'Add Host'))}</button>
            </div>
        </form>`;
    }

    function dialogFormInput(dialog, input) {
        if (!dialog.form || !input.name) return;
        dialog.form.values[input.name] = input.value;
        if (!dialog.form.errors[input.name]) return;
        delete dialog.form.errors[input.name];
        const field = input.closest('.cs-wsd-field');
        const error = field && field.querySelector('.cs-wsd-field-error');
        if (error) error.remove();
        input.setAttribute('aria-invalid', 'false');
        input.removeAttribute('aria-describedby');
    }

    function dialogFormAuth(dialog) {
        const formEl = dialog.overlay.querySelector('[data-wsd-form]');
        if (!dialog.form || !formEl) return;
        dialog.form.values = Object.assign(readDialogForm(formEl), { secret: '', passphrase: '' });
        delete dialog.form.errors.secret;
        renderDialogForm(dialog);
        const secret = dialog.overlay.querySelector('[data-wsd-form] [name="secret"]');
        if (secret) secret.focus();
    }

    function dialogLoadKeyFile(dialog, input) {
        const file = input.files && input.files[0];
        const formEl = dialog.overlay.querySelector('[data-wsd-form]');
        if (!file || !dialog.form || !formEl) return;
        if (file.size > WS_KEY_FILE_LIMIT) {
            dialog.form.values = readDialogForm(formEl);
            dialog.form.errors.secret = 'key';
            renderDialogForm(dialog);
            return;
        }
        file.text().then(bindInstance(dialog.instance, text => {
            const current = dialog.overlay.querySelector('[data-wsd-form]');
            if (!dialog.isOpen() || !dialog.form || !current) return;
            dialog.form.values = Object.assign(readDialogForm(current), { secret: text });
            delete dialog.form.errors.secret;
            renderDialogForm(dialog);
            const passphrase = dialog.overlay.querySelector('[data-wsd-form] [name="passphrase"]');
            if (passphrase) passphrase.focus();
        }));
    }

    async function dialogSubmitForm(dialog, formEl) {
        const form = dialog.form;
        if (!form || form.busy) return;
        form.values = readDialogForm(formEl);
        const check = WS_MODEL.validateHostForm(form.values);
        form.errors = check.errors;
        form.error = '';
        if (!check.valid) {
            renderDialogForm(dialog);
            const first = dialog.overlay.querySelector('[data-wsd-form] [aria-invalid="true"]');
            if (first) first.focus();
            return;
        }
        form.busy = true;
        renderDialogForm(dialog);
        try {
            const created = await apiClient.createRemoteDevice(check.value);
            if (!dialog.isOpen()) return;
            const device = WS_MODEL.normalizeDevice(created);
            if (!device.id) throw new Error('no device id');
            dialog.devices = [device].concat(dialog.devices.filter(item => item.id !== device.id));
            dialog.form = null; // drops the secret from memory
            runWithInstance(dialog.instance, () => {
                dialog.overlay.querySelector('[data-wsd-form-slot]').innerHTML = '';
                dialog.query = '';
                dialog.overlay.querySelector('[data-wsd-search]').value = '';
                dialogSelectDevice(dialog, device.id, { focus: 'list' });
            });
        } catch (err) {
            if (!dialog.isOpen() || dialog.form !== form) return;
            form.busy = false;
            form.error = tr('codeStudio.workspaceFormSaveFailed', 'Could not add the host: {{error}}', { error: err.message || String(err) });
            runWithInstance(dialog.instance, () => renderDialogForm(dialog));
        }
    }
```

- [ ] **Step 2:** Append the folder browser, the trust card and Open:

```js
    // --- Connect dialog: folder browser, host key, open ------------------------------------------

    async function dialogBrowse(dialog, rawPath, options = {}) {
        const deviceId = dialog.deviceId;
        if (!deviceId) return;
        const previous = dialog.browse && dialog.browse.deviceId === deviceId ? dialog.browse : null;
        const home = dialog.home[deviceId] || '';
        const path = rawPath ? WS_MODEL.normalizeRemotePath(WS_MODEL.expandHome(rawPath, home), previous && previous.path ? previous.path : (home || '/')) : '';
        // '~' before the first listing is the home folder, which the server lists when no path is sent.
        const requestPath = path.startsWith('~') ? '' : path;
        const token = Symbol('browse');
        dialog.browse = { deviceId, path: previous ? previous.path : '', entries: previous ? previous.entries : [], loading: true, error: null, hostKey: null, selected: '', token };
        dialog.openError = '';
        renderDialogBrowser(dialog);
        renderDialogFooter(dialog);
        const trust = options.trust || dialog.trusted[deviceId] || '';
        try {
            const listing = WS_MODEL.normalizeBrowse(await apiClient.remoteBrowse(deviceId, requestPath, trust));
            if (!dialog.isOpen() || dialog.browse.token !== token) return;
            if (!requestPath && !dialog.home[deviceId]) dialog.home[deviceId] = listing.path;
            if (options.trust) dialog.trusted[deviceId] = options.trust;
            Object.assign(dialog.browse, { path: listing.path, entries: listing.entries, loading: false });
        } catch (err) {
            if (!dialog.isOpen() || dialog.browse.token !== token) return;
            dialogBrowseFailed(dialog, err, path, fp => dialogBrowse(dialog, rawPath, Object.assign({}, options, { trust: fp || options.trust, focus: 'list' })));
        }
        runWithInstance(dialog.instance, () => {
            renderDialogBrowser(dialog);
            renderDialogFooter(dialog);
            const trustButton = dialog.overlay.querySelector('[data-wsd-trust]');
            if (dialog.browse.hostKey && trustButton) trustButton.focus();
            else if (options.focus === 'list') dialogFocusFolders(dialog);
        });
    }

    function dialogBrowseFailed(dialog, err, path, retry) {
        const kind = WS_MODEL.classifyError(err);
        const device = dialogDevice(dialog);
        const browse = dialog.browse;
        browse.loading = false;
        browse.entries = [];
        if (path) browse.path = path;
        if (kind === 'unknown_host_key') {
            const body = err.body || {};
            browse.hostKey = { host: String(body.host || (device && device.host) || ''), keyType: String(body.key_type || ''), fingerprint: String(body.fingerprint || ''), retry };
            return;
        }
        browse.error = {
            kind,
            text: workspaceErrorText(kind, { host: device ? WS_MODEL.deviceAddress(device) : '', path: path || browse.path, error: err.message || String(err) }),
            retry: kind === 'host_key_mismatch' ? null : retry
        };
    }

    function dialogTrust(dialog) {
        const key = dialog.browse && dialog.browse.hostKey;
        if (!key || !key.fingerprint) return;
        dialog.trusted[dialog.deviceId] = key.fingerprint;
        key.retry(key.fingerprint);
    }

    function renderDialogBrowser(dialog) {
        const pane = dialog.overlay.querySelector('[data-wsd-browser]');
        if (!pane) return;
        const active = pane.contains(document.activeElement) ? document.activeElement : null;
        const focusAttr = active ? WSD_FOCUS_ATTRS.find(attr => active.hasAttribute(attr)) : '';
        const focusList = !!active && !!active.closest('[data-wsd-folders]');
        const device = dialogDevice(dialog);
        const browse = dialog.browse;
        if (!device || !browse) {
            pane.innerHTML = `<div class="cs-wsd-placeholder">${wsGlyph('folder', 28)}<p>${esc(tr('codeStudio.workspacePickHost', 'Choose a host to see its folders.'))}</p></div>`;
        } else if (browse.hostKey) {
            const key = browse.hostKey;
            pane.innerHTML = `<div class="cs-wsd-card cs-wsd-trust" role="group" aria-labelledby="cs-wsd-trust-title">
                <span class="cs-wsd-card-icon">${wsGlyph('key', 20)}</span>
                <h3 class="cs-wsd-card-title" id="cs-wsd-trust-title">${esc(tr('codeStudio.workspaceTrustTitle', 'Unknown host key'))}</h3>
                <p>${esc(tr('codeStudio.workspaceTrustText', 'Code Studio has not connected to {{host}} before. Compare the fingerprint with the one the server shows before you trust it.', { host: key.host }))}</p>
                <dl class="cs-wsd-facts">
                    <dt>${esc(tr('codeStudio.workspaceFormHost', 'Host'))}</dt><dd><code>${esc(key.host)}</code></dd>
                    <dt>${esc(tr('codeStudio.workspaceTrustKeyType', 'Key type'))}</dt><dd><code>${esc(key.keyType)}</code></dd>
                    <dt>${esc(tr('codeStudio.workspaceTrustFingerprint', 'Fingerprint'))}</dt><dd><code>${esc(key.fingerprint)}</code><button type="button" class="cs-icon-button" data-wsd-copy="${esc(key.fingerprint)}" aria-label="${esc(tr('desktop.copy', 'Copy'))}" title="${esc(tr('desktop.copy', 'Copy'))}">${iconMarkup('copy', 'C', 'cs-icon-button-icon', 13)}</button></dd>
                </dl>
                <div class="cs-wsd-card-actions">
                    <button type="button" class="cs-button" data-wsd-trust-cancel>${esc(tr('desktop.cancel', 'Cancel'))}</button>
                    <button type="button" class="cs-button primary" data-wsd-trust>${esc(tr('codeStudio.workspaceTrustContinue', 'Trust and Continue'))}</button>
                </div>
            </div>`;
        } else if (browse.error && browse.error.kind === 'host_key_mismatch') {
            pane.innerHTML = `<div class="cs-wsd-card cs-wsd-danger" role="alert">
                <span class="cs-wsd-card-icon">${wsGlyph('warning', 20)}</span>
                <h3 class="cs-wsd-card-title">${esc(tr('codeStudio.workspaceMismatchTitle', 'Host key changed'))}</h3>
                <p>${esc(browse.error.text)}</p>
                <div class="cs-wsd-card-actions"><button type="button" class="cs-button" data-wsd-other-host>${esc(tr('codeStudio.workspaceChooseOtherHost', 'Choose Another Host'))}</button></div>
            </div>`;
        } else {
            pane.innerHTML = dialogListingMarkup(dialog, device, browse);
        }
        if (active && !pane.contains(document.activeElement)) {
            const target = (focusAttr && pane.querySelector('[' + focusAttr + ']:not([disabled])'))
                || (focusList && pane.querySelector('[data-wsd-folders] [tabindex="0"]'))
                || pane.querySelector('[data-wsd-trust], [data-wsd-folders] [tabindex="0"], [data-wsd-path], [data-wsd-other-host]');
            if (target) target.focus();
        }
    }

    function dialogListingMarkup(dialog, device, browse) {
        const home = dialog.home[device.id] || '';
        const crumbs = browse.path ? WS_MODEL.remoteBreadcrumbs(browse.path) : [];
        const crumbHtml = crumbs.map((crumb, index) => (index === crumbs.length - 1
            ? `<li><span class="cs-wsd-crumb is-current" aria-current="location">${esc(crumb.name)}</span></li>`
            : `<li><button type="button" class="cs-wsd-crumb" data-wsd-crumb="${esc(crumb.path)}">${esc(crumb.name)}</button></li>`)).join('');
        const recent = WS_MODEL.recentRoots(dialog.instance.workspaces || [], device.id).filter(root => root !== browse.path);
        const dirs = browse.entries.filter(entry => entry.dir);
        const files = browse.entries.filter(entry => !entry.dir);
        const focusPath = dirs.some(entry => entry.path === browse.selected) ? browse.selected : (dirs[0] && dirs[0].path);
        let rows;
        if (browse.error) {
            rows = `<div class="cs-wsd-card cs-wsd-error" role="alert">${wsGlyph('warning', 18)}<p>${esc(browse.error.text)}</p>${browse.error.retry ? `<button type="button" class="cs-button" data-wsd-retry>${esc(tr('desktop.retry', 'Retry'))}</button>` : ''}</div>`;
        } else if (browse.loading && !browse.entries.length) {
            rows = wsdSkeleton(6);
        } else {
            rows = dirs.map(entry => `<div class="cs-wsd-entry is-dir${entry.hidden ? ' is-hidden' : ''}${entry.path === browse.selected ? ' is-selected' : ''}" role="option" data-wsd-entry="${esc(entry.path)}" data-dir="true" data-name="${esc(entry.name)}" aria-selected="${entry.path === browse.selected}" tabindex="${entry.path === focusPath ? 0 : -1}" title="${esc(entry.path)}">${wsGlyph('folder', 15)}<span class="cs-wsd-entry-name">${esc(entry.name)}</span>${entry.git ? '<span class="cs-wsd-git" title="Git">git</span>' : ''}</div>`).join('')
                + files.map(entry => `<div class="cs-wsd-entry is-file" role="option" data-wsd-entry="${esc(entry.path)}" data-dir="false" aria-disabled="true" aria-selected="false" tabindex="-1">${wsGlyph('file', 15)}<span class="cs-wsd-entry-name">${esc(entry.name)}</span></div>`).join('')
                + (dirs.length ? '' : `<div class="cs-wsd-note">${esc(tr('codeStudio.workspaceNoSubfolders', 'This folder has no subfolders.'))}</div>`);
        }
        const pathLabel = tr('codeStudio.workspacePathLabel', 'Folder path');
        return `<div class="cs-wsd-toolbar">
                <button type="button" class="cs-icon-button" data-wsd-home aria-label="${esc(tr('codeStudio.workspaceHome', 'Home folder'))}" title="${esc(tr('codeStudio.workspaceHome', 'Home folder'))}">${wsGlyph('home', 15)}</button>
                <button type="button" class="cs-icon-button" data-wsd-up aria-label="${esc(tr('codeStudio.workspaceUp', 'Parent folder'))}" title="${esc(tr('codeStudio.workspaceUp', 'Parent folder'))}"${!browse.path || browse.path === '/' ? ' disabled' : ''}>${wsGlyph('up', 15)}</button>
                <nav class="cs-wsd-crumbs" aria-label="${esc(pathLabel)}"><ol>${crumbHtml}</ol></nav>
                <button type="button" class="cs-icon-button" data-wsd-refresh aria-label="${esc(tr('codeStudio.refresh', 'Refresh'))}" title="${esc(tr('codeStudio.refresh', 'Refresh'))}">${wsGlyph('refresh', 15)}</button>
            </div>
            <form class="cs-wsd-pathform" data-wsd-pathform>
                <label class="cs-ws-sr" for="cs-wsd-path">${esc(pathLabel)}</label>
                <input id="cs-wsd-path" class="cs-wsd-path" data-wsd-path value="${esc(browse.path)}" autocomplete="off" spellcheck="false" placeholder="/home/user/project">
            </form>
            ${recent.length ? `<div class="cs-wsd-recent"><span>${esc(tr('codeStudio.workspaceRecent', 'Recent SSH folders'))}</span>${recent.map(root => `<button type="button" class="cs-wsd-chip" data-wsd-goto="${esc(root)}" title="${esc(root)}">${esc(WS_MODEL.tildePath(root, home))}</button>`).join('')}</div>` : ''}
            <div class="cs-wsd-folders${browse.loading ? ' is-loading' : ''}" role="listbox" aria-label="${esc(browse.path || pathLabel)}" aria-busy="${browse.loading}" data-wsd-folders>${rows}</div>`;
    }

    function dialogFolderKeydown(dialog, event) {
        const browse = dialog.browse;
        if (!browse) return;
        const rows = Array.from(dialog.overlay.querySelectorAll('[data-wsd-entry][data-dir="true"]'));
        const index = rows.indexOf(document.activeElement);
        const focusRow = row => {
            if (!row) return;
            rows.forEach(item => { item.tabIndex = -1; });
            row.tabIndex = 0;
            row.focus();
            dialogSelectEntry(dialog, row.dataset.wsdEntry, { keepFocus: true });
        };
        switch (event.key) {
            case 'ArrowDown': event.preventDefault(); focusRow(rows[Math.min(rows.length - 1, index + 1)]); break;
            case 'ArrowUp': event.preventDefault(); focusRow(rows[Math.max(0, index - 1)]); break;
            case 'Home': event.preventDefault(); focusRow(rows[0]); break;
            case 'End': event.preventDefault(); focusRow(rows[rows.length - 1]); break;
            case 'ArrowRight':
                if (index >= 0) { event.preventDefault(); dialogBrowse(dialog, rows[index].dataset.wsdEntry, { focus: 'list' }); }
                break;
            case 'ArrowLeft':
            case 'Backspace':
                event.preventDefault();
                if (browse.path && browse.path !== '/') dialogBrowse(dialog, WS_MODEL.remoteParent(browse.path), { focus: 'list' });
                break;
            case 'Enter': event.preventDefault(); dialogOpen(dialog, {}); break;
            default:
                if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
                    const key = event.key.toLowerCase();
                    const hit = rows.slice(index + 1).concat(rows.slice(0, index + 1)).find(row => (row.dataset.name || '').toLowerCase().startsWith(key));
                    if (hit) { event.preventDefault(); focusRow(hit); }
                }
        }
    }

    // A click toggles the selection; the target is the selected folder or, without one, the listed folder.
    function dialogSelectEntry(dialog, path, options) {
        const browse = dialog.browse;
        const entry = browse && browse.entries.find(item => item.path === path);
        if (!entry || !entry.dir) return;
        browse.selected = options.keepFocus || browse.selected !== path ? path : '';
        dialog.overlay.querySelectorAll('[data-wsd-entry][data-dir="true"]').forEach(row => {
            const selected = row.dataset.wsdEntry === browse.selected;
            row.classList.toggle('is-selected', selected);
            row.setAttribute('aria-selected', selected ? 'true' : 'false');
            if (!options.keepFocus) row.tabIndex = row.dataset.wsdEntry === path ? 0 : -1;
        });
        dialog.openError = '';
        renderDialogFooter(dialog);
    }

    function dialogFocusFolders(dialog) {
        const target = dialog.overlay.querySelector('[data-wsd-folders] [tabindex="0"]') || dialog.overlay.querySelector('[data-wsd-path]');
        if (target) target.focus();
    }

    function dialogSubmitPath(dialog, formEl) {
        const value = formEl.querySelector('[data-wsd-path]').value.trim();
        if (value) dialogBrowse(dialog, value, { focus: 'list' });
    }

    function dialogTargetPath(dialog) {
        return dialog.browse ? (dialog.browse.selected || dialog.browse.path) : '';
    }

    function renderDialogFooter(dialog) {
        const footer = dialog.overlay.querySelector('[data-wsd-footer]');
        if (!footer) return;
        const device = dialogDevice(dialog);
        const browse = dialog.browse;
        const target = dialogTargetPath(dialog);
        const ready = !!device && !!browse && !browse.loading && !browse.hostKey && !browse.error && !!target && !dialog.opening;
        const preview = device && target ? WS_MODEL.userAtHost(device) + ':' + target : '';
        const focused = footer.contains(document.activeElement) ? (document.activeElement.hasAttribute('data-wsd-open') ? '[data-wsd-open]' : '[data-wsd-cancel]') : '';
        footer.innerHTML = `<code class="cs-wsd-target" data-wsd-target title="${esc(preview)}">${esc(preview || '—')}</code>
            <div class="cs-wsd-hint" aria-hidden="true">${esc(tr('codeStudio.workspaceKeyHint', 'Enter opens · → enters a folder · ← goes up'))}</div>
            ${dialog.openError ? `<div class="cs-wsd-open-error" role="alert">${wsGlyph('warning', 14)}<span>${esc(dialog.openError)}</span></div>` : ''}
            <div class="cs-wsd-actions">
                <button type="button" class="cs-button" data-wsd-cancel>${esc(tr('desktop.cancel', 'Cancel'))}</button>
                <button type="button" class="cs-button primary" data-wsd-open${ready ? '' : ' disabled'} aria-busy="${dialog.opening}">${dialog.opening ? '<span class="cs-ws-spinner" aria-hidden="true"></span>' : ''}<span>${esc(dialog.opening ? tr('codeStudio.workspaceStatusConnecting', 'Connecting…') : tr('codeStudio.workspaceOpenFolder', 'Open Folder'))}</span></button>
            </div>`;
        if (focused) {
            const el = footer.querySelector(focused);
            (el && !el.disabled ? el : footer.querySelector('[data-wsd-cancel]')).focus();
        }
    }

    async function dialogOpen(dialog, options) {
        const device = dialogDevice(dialog);
        const browse = dialog.browse;
        const root = dialogTargetPath(dialog);
        if (!device || !browse || !root || dialog.opening || browse.loading || browse.hostKey || browse.error) return;
        const instance = dialog.instance;
        dialog.opening = true;
        dialog.openError = '';
        renderDialogFooter(dialog);
        const trust = options.trust || dialog.trusted[device.id] || '';
        let workspace;
        try {
            const body = { device_id: device.id, root };
            if (trust) body.trust_host_key = trust;
            workspace = WS_MODEL.normalizeWorkspace(await apiClient.createWorkspace(body));
            if (!WS_MODEL.isRemote(workspace.id)) throw new Error('no workspace id');
        } catch (err) {
            if (!dialog.isOpen()) return;
            dialog.opening = false;
            const kind = WS_MODEL.classifyError(err);
            runWithInstance(instance, () => {
                if (kind === 'unknown_host_key') {
                    const b = err.body || {};
                    browse.hostKey = {
                        host: String(b.host || device.host), keyType: String(b.key_type || ''), fingerprint: String(b.fingerprint || ''),
                        retry: fp => {
                            browse.hostKey = null;
                            dialog.trusted[device.id] = fp;
                            renderDialogBrowser(dialog);
                            dialogOpen(dialog, { trust: fp });
                        }
                    };
                    renderDialogBrowser(dialog);
                    const trustButton = dialog.overlay.querySelector('[data-wsd-trust]');
                    if (trustButton) trustButton.focus();
                } else if (kind === 'host_key_mismatch') {
                    browse.error = { kind, text: workspaceErrorText(kind, { host: device.host }), retry: null };
                    renderDialogBrowser(dialog);
                } else {
                    dialog.openError = workspaceErrorText(kind, { host: WS_MODEL.deviceAddress(device), path: root, error: err.message || String(err) });
                }
                renderDialogFooter(dialog);
            });
            return;
        }
        if (!dialog.isOpen()) return;
        const list = (instance.workspaces || [WS_MODEL.localWorkspace()]).filter(ws => ws.id !== workspace.id);
        list.splice(1, 0, workspace);
        instance.workspaces = list;
        // The dialog stays open while the switch asks about dirty tabs; Cancel there keeps the dialog.
        const switched = await runWithInstance(instance, () => switchWorkspace(workspace.id, { workspace, connected: true }));
        if (!dialog.isOpen()) return;
        dialog.opening = false;
        if (switched) dialog.close(true);
        else runWithInstance(instance, () => renderDialogFooter(dialog));
    }
```

- [ ] **Step 3:** Run `npm run build:ui`, then `go test ./ui/ -run '(?i)CodeStudio' -count=1`.
  Expected: `ok`.
- [ ] **Step 4:** Commit `workspace.js` with the message `feat(code-studio): connect dialog with hosts, new-host form, folder browser and host-key trust`.

---

### Task 11: Dialog CSS

**Files:** Append to `ui/css/code-studio.css`

- [ ] **Step 1:** Append:

```css
/* === Plan C: SSH workspaces — connect dialog ============================================= */
.cs-modal.cs-wsd { display: grid; grid-template-rows: auto minmax(0, 1fr) auto; width: min(880px, calc(100vw - 32px)); height: min(600px, calc(100vh - 48px)); padding: 0; overflow: hidden; color: var(--cs-text); font: 12.5px var(--cs-ui-font); }
.cs-wsd-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 16px; border-bottom: 1px solid var(--cs-line); }
.cs-wsd-body { display: grid; grid-template-columns: minmax(220px, 300px) minmax(0, 1fr); min-height: 0; }
.cs-wsd-hosts { display: flex; flex-direction: column; gap: 8px; min-height: 0; padding: 12px; border-right: 1px solid var(--cs-line); background: var(--vd-mat-side, var(--cs-panel-soft)); }
.cs-wsd-hosts-main { display: flex; flex: 1 1 auto; flex-direction: column; gap: 8px; min-height: 0; }
.cs-wsd-hosts-main[hidden], .cs-wsd-form-slot[hidden] { display: none; }
.cs-wsd-form-slot { flex: 1 1 auto; min-height: 0; overflow-y: auto; }
.cs-wsd-pane-title { color: var(--vd-mat-label-color, var(--cs-muted)); font: 700 10.5px var(--vd-mat-label-font, var(--cs-ui-font)); letter-spacing: var(--vd-mat-label-tracking, 0.5px); text-transform: var(--vd-mat-label-transform, uppercase); }
.cs-modal input.cs-wsd-search, .cs-modal input.cs-wsd-path { box-sizing: border-box; width: 100%; height: 30px; }
.cs-modal :is(input.cs-wsd-search, input.cs-wsd-path, .cs-wsd-field input, .cs-wsd-field textarea):focus-visible { outline: 2px solid var(--cs-accent-border); outline-offset: 0; }
.cs-wsd-hostlist { display: flex; flex: 1 1 auto; flex-direction: column; gap: 2px; min-height: 0; overflow-y: auto; }
.cs-wsd-host { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; column-gap: 9px; align-items: center; padding: 7px 9px; border: 1px solid transparent; border-radius: var(--cs-radius-sm); cursor: pointer; }
.cs-wsd-host:hover { background: var(--cs-hover); }
.cs-wsd-host:focus-visible { outline: 2px solid var(--cs-accent-border); outline-offset: -2px; }
.cs-wsd-host.is-selected { border-color: var(--cs-accent-border); background: var(--cs-accent-soft); }
.cs-wsd-host-icon { display: inline-flex; color: var(--cs-muted); }
.cs-wsd-host.is-selected .cs-wsd-host-icon { color: var(--cs-accent); }
.cs-wsd-host-text { display: grid; min-width: 0; line-height: 1.3; }
.cs-wsd-host-name { overflow: hidden; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.cs-wsd-host-addr { overflow: hidden; color: var(--cs-muted); font: 11px var(--cs-mono-font); text-overflow: ellipsis; white-space: nowrap; }
.cs-wsd-host-used { align-self: start; color: var(--cs-muted); font-size: 10.5px; white-space: nowrap; }
.cs-wsd-new-host { justify-content: center; }
.cs-wsd-empty { display: grid; gap: 8px; justify-items: start; padding: 10px 4px; color: var(--cs-muted); }
.cs-wsd-skeleton { height: 40px; border-radius: var(--cs-radius-sm); background: linear-gradient(90deg, var(--cs-accent-faint) 0%, var(--cs-hover) 50%, var(--cs-accent-faint) 100%); background-size: 200% 100%; animation: cs-ws-shimmer 1.2s ease-in-out infinite; }
.cs-wsd-folders .cs-wsd-skeleton { flex: 0 0 auto; height: 28px; }
@keyframes cs-ws-shimmer { from { background-position: 100% 0; } to { background-position: -100% 0; } }

.cs-wsd-browser { display: flex; flex-direction: column; gap: 8px; min-width: 0; min-height: 0; padding: 12px; }
.cs-wsd-placeholder { display: grid; flex: 1 1 auto; place-items: center; align-content: center; gap: 10px; color: var(--cs-muted); text-align: center; }
.cs-modal .cs-wsd-placeholder p { color: var(--cs-muted); }
.cs-wsd-toolbar { display: flex; align-items: center; gap: 4px; min-width: 0; }
.cs-wsd-crumbs { flex: 1 1 auto; min-width: 0; overflow: hidden; }
.cs-wsd-crumbs ol { display: flex; align-items: center; gap: 2px; margin: 0; padding: 0; overflow-x: auto; list-style: none; scrollbar-width: none; }
.cs-wsd-crumbs li { display: inline-flex; flex: 0 0 auto; align-items: center; }
.cs-wsd-crumbs li + li::before { content: '/'; margin: 0 2px; color: var(--cs-muted); opacity: 0.6; }
.cs-wsd-crumb { padding: 2px 6px; border: 0; border-radius: var(--cs-radius-sm); color: var(--cs-muted); background: transparent; font: 12px var(--cs-mono-font); cursor: pointer; }
.cs-wsd-crumb:hover { color: var(--cs-text); background: var(--cs-hover); }
.cs-wsd-crumb:focus-visible { outline: 2px solid var(--cs-accent-border); outline-offset: 0; }
.cs-wsd-crumb.is-current { color: var(--cs-text); font-weight: 600; cursor: default; }
.cs-wsd-recent { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; color: var(--cs-muted); font-size: 11px; }
.cs-wsd-chip { padding: 2px 8px; border: 1px solid var(--cs-line); border-radius: 999px; color: var(--cs-text); background: transparent; font: 11px var(--cs-mono-font); cursor: pointer; }
.cs-wsd-chip:hover { border-color: var(--cs-accent-border); background: var(--cs-accent-faint); }
.cs-wsd-chip:focus-visible { outline: 2px solid var(--cs-accent-border); outline-offset: 1px; }
.cs-wsd-folders { display: flex; flex: 1 1 auto; flex-direction: column; gap: 1px; min-height: 0; padding: 4px; overflow-y: auto; border: 1px solid var(--cs-line); border-radius: var(--cs-radius-md); background: var(--cs-bg); }
.cs-wsd-folders.is-loading { opacity: 0.6; }
.cs-wsd-entry { display: flex; align-items: center; gap: 8px; min-height: 28px; padding: 3px 8px; border-radius: var(--cs-radius-sm); cursor: pointer; user-select: none; }
.cs-wsd-entry .cs-ws-glyph { color: var(--cs-muted); }
.cs-wsd-entry.is-dir .cs-ws-glyph { color: var(--cs-accent); }
.cs-wsd-entry:hover { background: var(--cs-hover); }
.cs-wsd-entry:focus-visible { outline: 2px solid var(--cs-accent-border); outline-offset: -2px; }
.cs-wsd-entry.is-selected { background: var(--cs-selection-strong); }
.cs-wsd-entry.is-hidden { opacity: 0.72; }
.cs-wsd-entry.is-file { color: var(--cs-muted); cursor: default; opacity: 0.55; }
.cs-wsd-entry-name { flex: 1 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cs-wsd-git { flex: 0 0 auto; padding: 0 6px; border: 1px solid var(--cs-accent-border); border-radius: 999px; color: var(--cs-accent); font: 600 10px/16px var(--cs-mono-font); }
.cs-wsd-note { padding: 10px 8px; color: var(--cs-muted); }

.cs-wsd-card { display: grid; align-content: start; gap: 10px; margin: auto 0; padding: 16px; border: 1px solid var(--cs-line); border-radius: var(--cs-radius-lg); background: var(--vd-mat-card, var(--cs-panel)); }
.cs-wsd-card-title { margin: 0; font-size: 14px; }
.cs-wsd-card-icon { display: inline-flex; color: var(--cs-warning); }
.cs-wsd-danger { border-color: var(--cs-danger); background: var(--cs-danger-soft); }
.cs-wsd-danger .cs-wsd-card-icon, .cs-wsd-error .cs-ws-glyph { color: var(--cs-danger); }
.cs-modal .cs-wsd-card p { color: var(--cs-text); line-height: 1.45; }
.cs-wsd-facts { display: grid; grid-template-columns: max-content minmax(0, 1fr); gap: 6px 12px; margin: 0; }
.cs-wsd-facts dt { color: var(--cs-muted); }
.cs-wsd-facts dd { display: flex; align-items: center; gap: 6px; min-width: 0; margin: 0; }
.cs-wsd-facts code { font: 12px var(--cs-mono-font); overflow-wrap: anywhere; user-select: all; }
.cs-wsd-card-actions { display: flex; justify-content: flex-end; gap: 8px; }
.cs-wsd-error { justify-items: start; margin: 8px; }

.cs-wsd-footer { display: grid; grid-template-columns: minmax(0, 1fr) auto; grid-template-areas: "target actions" "hint actions" "error error"; align-items: center; gap: 4px 12px; padding: 10px 16px; border-top: 1px solid var(--cs-line); }
.cs-wsd-target { grid-area: target; overflow: hidden; font: 12px var(--cs-mono-font); text-overflow: ellipsis; white-space: nowrap; }
.cs-wsd-hint { grid-area: hint; color: var(--cs-muted); font-size: 10.5px; }
.cs-wsd-open-error { grid-area: error; display: flex; align-items: center; gap: 6px; color: var(--cs-danger); }
.cs-wsd-footer .cs-wsd-actions { grid-area: actions; }
.cs-wsd-actions { display: flex; justify-content: flex-end; gap: 8px; }

.cs-wsd-form { display: grid; gap: 9px; }
.cs-wsd-row { display: grid; grid-template-columns: minmax(0, 1fr) 76px; gap: 8px; }
.cs-wsd-field { display: grid; gap: 4px; min-width: 0; }
.cs-modal .cs-wsd-field label { gap: 4px; font-size: 11.5px; }
.cs-modal .cs-wsd-field input { box-sizing: border-box; width: 100%; height: 30px; }
.cs-modal .cs-wsd-field textarea { box-sizing: border-box; width: 100%; min-height: 84px; padding: 6px 10px; border: 1px solid var(--vd-mat-field-border, var(--cs-border)); border-radius: var(--vd-mat-field-radius, var(--ds-radius-sm)); color: var(--cs-text); background: var(--vd-mat-field, var(--cs-panel-soft)); font: 11px var(--cs-mono-font); resize: vertical; }
.cs-modal .cs-wsd-field [aria-invalid="true"] { border-color: var(--cs-danger); }
.cs-wsd-field-error { color: var(--cs-danger); font-size: 11px; }
.cs-modal .cs-wsd-field-hint { margin: 0; color: var(--cs-muted); font-size: 11px; }
.cs-wsd-auth { display: flex; margin: 0; padding: 0; border: 0; }
.cs-wsd-auth legend { margin-bottom: 4px; font-size: 11.5px; font-weight: 600; }
.cs-modal .cs-wsd-auth label { display: inline-flex; flex: 1 1 0; }
.cs-wsd-auth input { position: absolute; opacity: 0; pointer-events: none; }
.cs-wsd-auth span { flex: 1 1 auto; padding: 5px 8px; border: 1px solid var(--cs-line); color: var(--cs-muted); font-weight: 500; text-align: center; cursor: pointer; }
.cs-wsd-auth label:first-of-type span { border-radius: var(--cs-radius-sm) 0 0 var(--cs-radius-sm); }
.cs-wsd-auth label:last-of-type span { border-left: 0; border-radius: 0 var(--cs-radius-sm) var(--cs-radius-sm) 0; }
.cs-wsd-auth input:checked + span { border-color: var(--cs-accent); color: var(--cs-accent-contrast); background: var(--cs-accent); }
.cs-wsd-auth input:focus-visible + span { outline: 2px solid var(--cs-accent-border); outline-offset: 1px; }
.cs-wsd-keyfile { justify-self: start; min-height: 26px; }

@media (max-width: 720px) {
    .cs-wsd-body { grid-template-columns: 1fr; grid-template-rows: minmax(140px, 38%) minmax(0, 1fr); }
    .cs-wsd-hosts { border-right: 0; border-bottom: 1px solid var(--cs-line); }
    .cs-wsd-hint { display: none; }
}
@media (prefers-reduced-motion: reduce) { .cs-wsd-skeleton { animation: none; } }
```

- [ ] **Step 2:** Run `go test ./ui/ -run '(?i)CodeStudio' -count=1`.
  Expected: `ok`.
- [ ] **Step 3:** Commit the CSS file with the message `style(code-studio): connect dialog, folder browser, trust card and host form`.

---

### Task 12: Capability gating — Git and LSP

**Files:** Modify `git.js` and `lsp.js`

- [ ] **Step 1: Impact.** Run impact for `renderGitPanel` (many callers) and `refreshGitStatus` (`scheduleGitRefresh`, toggle, the git actions). Also run it for `lspStateText` (only `lspStatusMarkup`).
- [ ] **Step 2: `git.js` `renderGitPanel`.** Directly after the `const head = \`...\`;` statement and before `if (state.gitIsRepo === false) {`, insert:

```js
        if (workspaceIsRemote() && !workspaceHas('git')) {
            // Plan C: the host reported no git; there is nothing to initialise either.
            panel.innerHTML = `${head}
                <div class="cs-git-init cs-ws-git-missing" data-ws-git-missing>
                    <p><strong>${esc(tr('codeStudio.workspaceGitMissing', 'Git is not installed on {{host}}', { host: workspaceHostLabel() }))}</strong></p>
                    <button type="button" class="cs-button" data-ws-git-reconnect>${esc(tr('codeStudio.workspaceReconnect', 'Reconnect'))}</button>
                </div>`;
            wireGitPanel(panel);
            const reconnect = panel.querySelector('[data-ws-git-reconnect]');
            if (reconnect) reconnect.addEventListener('click', bind(() => workspaceReconnect()));
            updatePolicyControls();
            return;
        }
```

- [ ] **Step 3: `git.js` `refreshGitStatus`.** Directly after `if (!isLiveInstance(target)) return;`, insert:

```js
        if (!workspaceHas('git', target)) {
            // Plan C: no git calls against a host without git.
            runWithInstance(target, () => {
                state.gitIsRepo = null;
                state.gitBranch = '';
                state.gitChanges = [];
                state.gitLog = [];
                renderGitPanel();
                renderActivityBar();
                renderStatus();
            });
            return;
        }
```

- [ ] **Step 4: `lsp.js` `lspStateText`.** Replace the `not-installed` case with:

```js
            case 'not-installed': return workspaceIsRemote()
                ? tr('codeStudio.workspaceLspNotInstalled', 'not installed on {{host}}', { host: workspaceHostLabel() }) // Plan C
                : tr('codeStudio.lspNotInstalled', 'Language server not installed — rebuild the Code Studio image');
```

  The bridge of plan A/D closes with 4404 when the capability is missing; the status then reads, for example, `gopls: not installed on pi@raspi`.
- [ ] **Step 5:** Run `npm run build:ui`, then `go test ./ui/ -run '(?i)CodeStudio' -count=1`.
  Expected: `ok`.
- [ ] **Step 6:** Run detect-changes, then commit `git.js` and `lsp.js` with the message `feat(code-studio): Git and LSP status follow the host's capabilities`.

---

### Task 13: Commands — palette, command table, File menu

**Files:** Modify `shortcuts.js`, `command-palette.js` and `core.js` (`renderWindowMenus`)

- [ ] **Step 1: Impact.** Run impact for `studioCommandTable` (`exposedCommand`), `getCommands` (palette render) and `renderWindowMenus` (many callers).
- [ ] **Step 2: `shortcuts.js`.** In `studioCommandTable`, insert after the line `// IDE package 1A`:

```js
            // Plan C: workspaces
            openConnectDialog: () => openConnectDialog(),
            openWorkspaceSwitcher,
            reconnectWorkspace: () => workspaceReconnect(),
            disconnectWorkspace: () => switchWorkspace('local'),
            switchWorkspace,
```

- [ ] **Step 3: `command-palette.js` `getCommands`.**
  1. After `const modified = ...;`, add:
     `const remote = !!state.workspaceId && state.workspaceId !== 'local';`
  2. Insert these entries after the `open-file` entry:

```js
            { id: 'workspace-connect', label: tr('codeStudio.workspaceMenu', 'Workspaces') + ': ' + tr('codeStudio.workspaceConnect', 'Connect to SSH Folder…'), icon: 'server', run: 'openConnectDialog' },
            { id: 'workspace-switch', label: tr('codeStudio.workspaceMenu', 'Workspaces') + '…', icon: 'folder-open', run: 'openWorkspaceSwitcher' },
            { id: 'workspace-reconnect', label: tr('codeStudio.workspaceMenu', 'Workspaces') + ': ' + tr('codeStudio.workspaceReconnect', 'Reconnect'), icon: 'refresh', run: 'reconnectWorkspace', disabled: !remote },
            { id: 'workspace-disconnect', label: tr('codeStudio.workspaceMenu', 'Workspaces') + ': ' + tr('codeStudio.workspaceDisconnect', 'Disconnect'), icon: 'x', run: 'disconnectWorkspace', disabled: !remote },
```

- [ ] **Step 4: `core.js` `renderWindowMenus`.** In the `file` menu, after the `open-file-dialog` item, insert:

```js
                    { id: 'workspace-connect', label: tr('codeStudio.workspaceConnect', 'Connect to SSH Folder…'), icon: 'server', action: bind(() => openConnectDialog()) },
                    ...(workspaceIsRemote() ? [{ id: 'workspace-disconnect', label: tr('codeStudio.workspaceDisconnect', 'Disconnect'), icon: 'x', action: bind(() => switchWorkspace('local')) }] : []),
```

- [ ] **Step 5:** Run `npm run build:ui`, then `go test ./ui/ -run '(?i)(CodeStudio|MobileLayout)' -count=1`.
  Expected: `ok`. The mobile layout test reads `command-palette.js`.
- [ ] **Step 6:** Run detect-changes, then commit the three files with the message `feat(code-studio): workspace commands in the palette and the File menu`.

---

### Task 14: One browser smoke against a stubbed workspace API

**Files:** Create `ui/desktop_code_studio_workspace_browser_test.go`

The plan A endpoints are stubbed page-side by wrapping `window.fetch`, so the shared Go fixture stays untouched. Files, terminal and git still go to the existing in-memory backend of `newCodeStudioBrowser`.

- [ ] **Step 1:** Create the file:

```go
package ui

import "testing"

// Plan C smoke: the workspace UI against a page-side stub of the plan A workspace API. The chip opens the menu, the
// connect dialog trusts an unknown host key, browses and opens a folder; afterwards scoped requests and the terminal
// socket carry the workspace, the subtitle and terminal tab show user@host, git is gated, a lost connection shows the
// banner and keeps a failed save dirty, and switching back with a dirty tab asks first.
func TestDesktopCodeStudioWorkspaceBrowser(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	errs := func() string { return page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str() }
	run := func(name, js string) {
		t.Helper()
		if _, err := page.Eval(js); err != nil {
			t.Fatalf("%s: %v; state: %s; browser errors: %s", name, err, page.MustEval(`()=>JSON.stringify({ws:csState()&&csState().workspaceId,status:csState()&&csState().workspaceStatus,requests:window.wsStub?wsStub.requests.slice(-8):[]})`).Str(), errs())
		}
	}

	run("boot and stub", `async()=>{
		await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);
		const stub=window.wsStub={offline:false,trusted:false,workspaces:[],requests:[],toasts:[],created:null};
		const json=(status,body)=>Promise.resolve(new Response(JSON.stringify(body),{status,headers:{'Content-Type':'application/json'}}));
		const real=window.fetch.bind(window);
		window.fetch=(input,init)=>{
			const url=String(input),opts=init||{},headers=opts.headers||{},ws=headers['X-Code-Studio-Workspace']||'';
			stub.requests.push({url,method:opts.method||'GET',ws});
			const path=url.split('?')[0],q=new URLSearchParams(url.split('?')[1]||'');
			if(path==='/api/code-studio/remote/devices')return json(200,{devices:[{id:'dev-1',name:'Build box',host:'10.0.0.5',port:22,username:'dev'}]});
			if(path==='/api/code-studio/remote/browse'){
				if(!stub.trusted&&q.get('trust_host_key')!=='SHA256:fixture')return json(409,{code:'unknown_host_key',error:'unknown host key',host:'10.0.0.5',key_type:'ssh-ed25519',fingerprint:'SHA256:fixture'});
				stub.trusted=true;
				const p=q.get('path')||'/home/dev';
				return json(200,{path:p,entries:p==='/home/dev'?[{name:'notes.txt',type:'file'},{name:'.cache',type:'directory'},{name:'proj',type:'directory',is_git:true}]:[{name:'src',type:'directory'}]});
			}
			if(path==='/api/code-studio/workspaces'&&opts.method==='POST'){
				const body=JSON.parse(opts.body);stub.created=body;
				const w={id:'ws-1',kind:'ssh',device_id:body.device_id,device_name:'Build box',host:'10.0.0.5',port:22,user:'dev',root:body.root,label:'',last_used_at:new Date().toISOString(),capabilities:{git:false,grep:true,lsp:{go:false,python:false,typescript:false,cpp:false}}};
				stub.workspaces=[w];return json(200,{workspace:w});
			}
			if(path==='/api/code-studio/workspaces')return json(200,{workspaces:[{id:'local',kind:'local'}].concat(stub.workspaces)});
			if(/^\/api\/code-studio\/workspaces\/[^/]+\/connect$/.test(path))return stub.offline?json(502,{code:'ssh_unavailable',error:'connection refused'}):json(200,{workspace:stub.workspaces[0]});
			if(stub.offline&&ws==='ws-1')return json(502,{code:'ssh_unavailable',error:'connection lost'});
			return real(input,init);
		};
		csState().context.notify=p=>{stub.toasts.push(p);};
		const chip=csRoot().querySelector('[data-status-remote]');
		if(!chip||chip.dataset.remoteState!=='local'||csRoot().querySelector('.code-studio-statusbar').firstElementChild!==chip)throw new Error('local chip is not first');
		if(!csRoot().querySelector('[data-ws-switcher]'))throw new Error('switcher missing');
		return true;
	}`)

	run("menu keyboard", `async()=>{
		csRoot().querySelector('[data-status-remote]').click();
		await csWait(()=>document.querySelector('.cs-ws-menu'),3000);
		if(document.activeElement.getAttribute('aria-checked')!=='true')throw new Error('current workspace not focused');
		csKey(document.activeElement,{key:'ArrowDown'});
		if(document.activeElement.dataset.wsAction!=='connect')throw new Error('ArrowDown did not reach connect');
		csKey(document.activeElement,{key:'Escape'});
		await csWait(()=>!document.querySelector('.cs-ws-menu'),2000);
		await csWait(()=>document.activeElement&&document.activeElement.matches('[data-status-remote]'),2000);
		return true;
	}`)

	run("connect dialog", `async()=>{
		csRoot().querySelector('[data-status-remote]').click();
		(await csWait(()=>document.querySelector('.cs-ws-menu [data-ws-action="connect"]'),3000)).click();
		const dialog=await csWait(()=>document.querySelector('.cs-wsd[role="dialog"][aria-modal="true"]'),3000);
		const host=await csWait(()=>dialog.querySelector('[data-wsd-host="dev-1"]'),5000);
		if(!host.textContent.includes('dev@10.0.0.5:22'))throw new Error('host address missing');
		if(!dialog.contains(document.activeElement))throw new Error('focus is not inside the dialog');
		const search=dialog.querySelector('[data-wsd-search]');
		search.value='zzz';search.dispatchEvent(new Event('input',{bubbles:true}));
		await csWait(()=>dialog.querySelector('[data-wsd-no-match]'),2000);
		search.value='';search.dispatchEvent(new Event('input',{bubbles:true}));
		(await csWait(()=>dialog.querySelector('[data-wsd-host="dev-1"]'),2000)).click();
		const trust=await csWait(()=>dialog.querySelector('[data-wsd-trust]'),5000);
		if(!dialog.querySelector('.cs-wsd-trust').textContent.includes('SHA256:fixture'))throw new Error('fingerprint missing');
		if(document.activeElement!==trust)throw new Error('trust button not focused');
		trust.click();
		const proj=await csWait(()=>dialog.querySelector('[data-wsd-entry="/home/dev/proj"]'),5000);
		const rows=[...dialog.querySelectorAll('[data-wsd-entry]')].map(r=>r.dataset.wsdEntry).join(',');
		if(rows!=='/home/dev/proj,/home/dev/.cache,/home/dev/notes.txt')throw new Error('order '+rows);
		if(!proj.querySelector('.cs-wsd-git'))throw new Error('git badge missing');
		if(dialog.querySelector('[data-wsd-entry="/home/dev/notes.txt"]').getAttribute('aria-disabled')!=='true')throw new Error('file not dimmed');
		proj.click();
		await csWait(()=>dialog.querySelector('[data-wsd-target]').textContent==='dev@10.0.0.5:/home/dev/proj',2000);
		csKey(dialog,{key:'Enter'});
		await csWait(()=>!document.querySelector('.cs-wsd')&&csState().workspaceId==='ws-1',8000);
		if(wsStub.created.root!=='/home/dev/proj'||wsStub.created.trust_host_key!=='SHA256:fixture')throw new Error('create body '+JSON.stringify(wsStub.created));
		return true;
	}`)

	run("remote chrome, requests and sockets", `async()=>{
		await csWait(()=>csRoot().querySelectorAll('.cs-tree-item').length>0&&wsStub.requests.some(r=>r.url.startsWith('/api/code-studio/files')&&r.ws==='ws-1'),8000);
		const chip=csRoot().querySelector('[data-status-remote]'),sw=csRoot().querySelector('[data-ws-switcher]');
		if(chip.dataset.remoteState!=='connected'||!chip.textContent.includes('Build box'))throw new Error('chip '+chip.outerHTML.slice(0,160));
		if(!sw.textContent.includes('Build box · ~/proj')||!sw.textContent.includes('dev@10.0.0.5'))throw new Error('switcher '+sw.textContent);
		if(csRoot().closest('.vd-window').querySelector('.vd-window-subtitle').textContent!=='dev@10.0.0.5:~/proj')throw new Error('subtitle');
		if(JSON.parse(localStorage.getItem('aurago.codeStudio.workspace.v1')).id!=='ws-1')throw new Error('active workspace not stored');
		if(wsStub.requests.some(r=>r.url.startsWith('/api/code-studio/workspaces')&&r.ws))throw new Error('workspace API must stay unscoped');
		const socket=await csWait(()=>csState().terminalSessions[0]&&csState().terminalSessions[0].ws,5000);
		if(!socket.url.includes('ws=ws-1'))throw new Error('terminal socket '+socket.url);
		if(!csRoot().querySelector('[data-terminal-tab="0"]').textContent.includes('dev@10.0.0.5'))throw new Error('terminal tab label');
		return true;
	}`)

	run("git gated by capabilities", `async()=>{
		csRoot().querySelector('[data-activity="git"]').click();
		const box=await csWait(()=>csRoot().querySelector('[data-ws-git-missing]'),5000);
		if(!box.textContent.includes('dev@10.0.0.5'))throw new Error('host missing in the git empty state');
		if(wsStub.requests.some(r=>r.url.startsWith('/api/code-studio/git/')&&r.ws==='ws-1'))throw new Error('git was called on a host without git');
		csRoot().querySelector('[data-activity="git"]').click();
		return true;
	}`)

	run("offline save keeps the tab dirty, banner reconnects", `async()=>{
		const id=csState().windowId;
		await CodeStudioApp.openFile('/workspace/main.go',true,id);
		await csWait(()=>csState().openTabs.length===1&&csState().openTabs[0].view,8000);
		csState().openTabs[0].view.dispatch({changes:{from:0,insert:'// remote edit\n'}});
		await csWait(()=>csState().openTabs[0].modified,2000);
		wsStub.offline=true;
		await CodeStudioApp.saveCurrentFile(id);
		await csWait(()=>{const b=csRoot().querySelector('[data-ws-banner]');return b&&!b.hidden&&b.dataset.wsBannerState==='lost';},5000);
		if(!csState().openTabs[0].modified)throw new Error('a failed save cleared the dirty flag');
		if(!wsStub.toasts.length)throw new Error('no toast for the failed save');
		if(csRoot().querySelector('[data-status-remote]').dataset.remoteState!=='lost')throw new Error('chip not lost');
		wsStub.offline=false;
		const button=csRoot().querySelector('[data-ws-banner-action="reconnect"]');
		if(button)button.click();
		await csWait(()=>csRoot().querySelector('[data-ws-banner]').hidden&&csRoot().querySelector('[data-status-remote]').dataset.remoteState==='connected',12000);
		return true;
	}`)

	run("dirty switch asks first", `async()=>{
		const ask=async()=>{csRoot().querySelector('[data-ws-switcher]').click();(await csWait(()=>document.querySelector('.cs-ws-menu [data-ws-id="local"]'),3000)).click();return csWait(()=>document.querySelector('.cs-ws-dirty'),3000);};
		(await ask()).querySelector('[data-choice="cancel"]').click();
		await csWait(()=>!document.querySelector('.cs-ws-dirty'),2000);
		if(csState().workspaceId!=='ws-1')throw new Error('cancel switched anyway');
		(await ask()).querySelector('[data-choice="discard"]').click();
		await csWait(()=>csState().workspaceId==='local'&&csRoot().querySelector('[data-status-remote]').dataset.remoteState==='local',8000);
		const before=wsStub.requests.length;
		await CodeStudioApp.refreshFiles('/workspace',csState().windowId);
		const files=wsStub.requests.slice(before).filter(r=>r.url.startsWith('/api/code-studio/files'));
		if(!files.length||files.some(r=>r.ws))throw new Error('local requests still carry a workspace');
		const sessions=JSON.parse(localStorage.getItem('aurago.codeStudio.workspaceSessions.v1'));
		if(!sessions['ws-1']||!sessions['ws-1'].openTabs.includes('/workspace/main.go'))throw new Error('ws-1 session not kept');
		if(csRoot().closest('.vd-window').querySelector('.vd-window-subtitle').textContent!=='')throw new Error('subtitle not cleared');
		if(fixtureErrors.length)throw new Error('browser errors '+JSON.stringify(fixtureErrors));
		return true;
	}`)
}
```

- [ ] **Step 2:** Run `npm run build:ui`. The test serves the working-tree bundle.
- [ ] **Step 3:** In PowerShell, sandbox off, after checking for leftover rod Chromes, run:
  `$env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:GOTMPDIR='D:\tmp\go-build'; go test ./ui/ -run '(?i)CodeStudioWorkspace' -count=1 -v`
  Expected: `--- PASS: TestDesktopCodeStudioWorkspaceBrowser` and `ok`.
  If it fails, fix the code, not the assertions, unless an assertion contradicts this plan. Report any change.
- [ ] **Step 4:** Commit the test file with the message `test(code-studio): browser smoke for SSH workspaces`.

---

### Task 15: Final verification and the bundle commit

- [ ] **Step 1:** Run `node --test ui/js/desktop/apps/code-studio/workspace-model.test.cjs`.
  Expected: 11 pass, 0 fail.
- [ ] **Step 2:** Run `npm run build:ui`, then `npm run build:ui -- --check`.
  Expected: no `Outdated` line.
- [ ] **Step 3:** Run `go test ./ui/ -run '(?i)CodeStudio' -count=1`.
  Expected: `ok` (static tests).
- [ ] **Step 4:** Run the browser regression set once (same env):
  `go test ./ui/ -run '(?i)^TestDesktopCodeStudio(Browser|LSPBrowser|WorkspaceBrowser)$' -count=1 -timeout 30m -v`
  Expected: three `--- PASS` lines.
- [ ] **Step 5:** Run `git status --short -- ui/js/desktop/apps/code-studio ui/css/code-studio.css ui/js/desktop/apps/code-studio.js`.
  - Expected: nothing uncommitted besides the bundle.
  - If plan B or D has uncommitted Code Studio sources, do not commit the bundle. Report it instead, so the last plan to finish commits it.
- [ ] **Step 6:** Run detect-changes. It must not report `partial` or `truncated`.
- [ ] **Step 7:** Commit only `ui/js/desktop/bundles/code-studio.bundle.js`, with the message `build(code-studio): bundle with the SSH workspace UI`. Do not commit other regenerated bundles.
- [ ] **Step 8: Report.**
  - Commits, test results and any anchor deviations.
  - Remaining assumptions about plan A (contract table above).
  - Acceptance on aurago-test is the user's.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| A save started before a switch lands in the new workspace | The header is read when the request starts, and `workspaceSettleTabs` awaits every `pendingSave` before `workspaceActivate` changes the id. In-flight tree, git and open requests are invalidated with a new `Symbol`. |
| Plan A answers in other shapes or uses other codes | All parsing is in `workspace-model.js`; the normalizers accept the common variants, and `CONNECTION_CODES` is one list. |
| A loss is only noticed on the next request | Remote terminal `onclose` triggers a connect probe; any scoped request failure marks the connection lost; the backoff runs 2, 5, 10, 20, 30, 60 s. |
| Focus loss on re-render (status bar, sidebar, dialog panes) | The chip, the switcher, the dialog panes and the footer restore focus by data attribute. |
| Merge clash with plans B and D in `build-ui-bundles.js`, the locales, `lsp.js`, `shortcuts.js` and `command-palette.js` | Distinct anchors (`workspaceRoot` for the locales); the owners are listed under hand-offs; conflicts are additive. |
| Multiple Code Studio windows | The desktop reuses one window per app; `api()` falls back to `currentInstance()` when `state` is unset after an `await`. |
