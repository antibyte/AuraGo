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
// vm results come from another realm: compare through JSON so strict deepEqual ignores prototypes.
const deepEq = (a, b, m) => assert.deepEqual(json(a), json(b), m);

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
    deepEq(json(W.withWorkspaceHeaders({ method: 'POST' }, 'ws-1', '/api/code-studio/upload').headers), { 'X-Code-Studio-Workspace': 'ws-1' });
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
    deepEq(list.map(w => w.id), ['local', 'b', 'a']);
    deepEq(W.normalizeWorkspaceList(null).map(w => w.id), ['local']);
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
    deepEq(json(sessions['ws-1'].openTabs), ['/workspace/a.go']);
    assert.equal(sessions['ws-1'].activeTabIndex, 0);
    deepEq(json(sessions.local.openTabs), ['/workspace/main.go']);
    for (let i = 0; i < 40; i++) W.writeSession(storage, 'w' + i, { openTabs: [] }, 3000 + i);
    const pruned = W.readSessions(storage);
    assert.ok(Object.keys(pruned).length <= W.MAX_SESSIONS);
    assert.ok(pruned.local, 'local is never pruned');
    assert.ok(pruned.w39 && !pruned.w0);
    W.forgetSession(storage, 'w39');
    assert.equal(W.readSessions(storage).w39, undefined);
    storage.setItem(W.SESSIONS_KEY, '{broken');
    deepEq(json(W.readSessions(storage)), {});
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
    deepEq([0, 1, 2, 3, 4, 5, 9].map(W.backoffDelay), [2000, 5000, 10000, 20000, 30000, 60000, 60000]);
});

test('new host form validation', () => {
    const ok = W.validateHostForm({ name: 'Pi', host: '192.168.1.20', port: '22', username: 'pi', auth: 'password', secret: 'pw' });
    assert.equal(ok.valid, true);
    deepEq(json(ok.value), { name: 'Pi', host: '192.168.1.20', port: 22, username: 'pi', auth: 'password', secret: 'pw', passphrase: '' });
    const bad = W.validateHostForm({ name: '', host: 'ssh://pi@box', port: '70000', username: '', auth: 'key', secret: 'not a key' });
    deepEq(json(bad.errors), { name: 'required', host: 'host', port: 'port', username: 'required', secret: 'key' });
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
    deepEq(json(W.remoteBreadcrumbs('/home/pi')), [{ name: '/', path: '/' }, { name: 'home', path: '/home' }, { name: 'pi', path: '/home/pi' }]);
});

test('folder listing: folders, then dot folders, then files', () => {
    const listing = W.normalizeBrowse({ path: '/home/pi', entries: [{ name: 'notes.txt', type: 'file' }, { name: '.cache', type: 'directory' }, { name: 'proj', type: 'directory', is_git: true }, { name: 'Apps', path: '/home/pi/Apps', is_dir: true }, { name: '..', type: 'directory' }] });
    assert.equal(listing.path, '/home/pi');
    deepEq(json(listing.entries.map(e => [e.name, e.dir, e.git, e.path])), [['Apps', true, false, '/home/pi/Apps'], ['proj', true, true, '/home/pi/proj'], ['.cache', true, false, '/home/pi/.cache'], ['notes.txt', false, false, '/home/pi/notes.txt']]);
});

test('hosts: search, last used, recent folders, relative time', () => {
    const workspaces = W.normalizeWorkspaceList([{ id: 'a', device_id: 'd1', root: '/home/pi/a', last_used_at: '2026-11-20T00:00:00Z' }, { id: 'b', device_id: 'd1', root: '/srv/b', last_used_at: '2026-11-24T00:00:00Z' }]);
    const devices = W.normalizeDeviceList({ devices: [{ id: 'd2', name: 'NAS', ip_address: '10.0.0.9', username: 'admin' }, { id: 'd1', name: 'Pi', host: '10.0.0.5', port: 2222, username: 'pi' }, { name: 'no id' }] });
    deepEq(devices.map(d => d.id), ['d2', 'd1']);
    assert.equal(W.deviceAddress(devices[1]), 'pi@10.0.0.5:2222');
    assert.equal(W.deviceAddress(devices[0]), 'admin@10.0.0.9:22');
    deepEq(W.sortDevices(devices, workspaces).map(d => d.id), ['d1', 'd2']);
    deepEq(W.filterDevices(devices, 'pi@').map(d => d.id), ['d1']);
    deepEq(W.filterDevices(devices, ' nas ').map(d => d.id), ['d2']);
    assert.equal(W.filterDevices(devices, '').length, 2);
    deepEq(json(W.recentRoots(workspaces, 'd1')), ['/srv/b', '/home/pi/a']);
    assert.equal(W.deviceLastUsed(devices[1], workspaces), Date.parse('2026-11-24T00:00:00Z'));
    deepEq(json(W.relativeTime(Date.parse('2026-11-23T00:00:00Z'), Date.parse('2026-11-25T00:00:00Z'))), { value: -2, unit: 'day' });
    assert.equal(W.relativeTime(0, Date.now()), null);
});
