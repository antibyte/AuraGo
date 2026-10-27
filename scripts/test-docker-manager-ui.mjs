// Docker Manager desktop app: Node tests for the pure model and the store (with a fake api). Later tasks append the load
// contract of the five modules, the stylesheet rules and the translation keys. Run: node scripts/test-docker-manager-ui.mjs
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const files = ['docker-manager-model.js', 'docker-manager-store.js'];
const read = file => fs.readFileSync(path.join(root, file), 'utf8');
const plain = value => JSON.parse(JSON.stringify(value));
// The modules run in their own vm realm, so arrays and objects they return are compared as plain JSON.
const eq = (actual, expected, message) => assert.deepStrictEqual(plain(actual), plain(expected), message);

// ---- the pure modules load without a DOM (the vm has no document) and stay under 1100 lines ------------------------------
for (const file of files) {
    const lines = read('ui/js/desktop/apps/' + file).split('\n').length;
    assert.ok(lines < 1100, `${file} has ${lines} lines, want fewer than 1100`);
}
const window = {};
vm.runInNewContext(files.map(file => read('ui/js/desktop/apps/' + file)).join('\n'), {
    window, console, setTimeout, clearTimeout, AbortController, Intl, Map, Set, JSON, Math, Date, URL, performance
});
const model = window.DockerManagerModel, Store = window.DockerManagerStore;
assert.ok(model && Store, 'the model and the store publish their global');

// ---- model fixtures -------------------------------------------------------------------------------------------------
const container = (name, extra = {}) => ({
    id: name.slice(0, 12).padEnd(12, '0'), full_id: name.padEnd(64, '0'), names: [name], image: 'nginx:1.27', image_id: 'sha256:aaa', state: 'running', status: 'Up 3 hours',
    created: 1760000000, ports: [], mounts: [], networks: ['bridge'], ...extra
});
const hostA = { id: 'local', name: 'Local', kind: 'local', address: 'unix:///var/run/docker.sock', color: 'blue', builtin: true, status: { state: 'ok' } };
const hostB = { id: 'h_nas', name: 'NAS', kind: 'ssh', address: '192.0.2.20', port: 0, color: 'teal', status: { state: 'unknown' } };

// ---- status and problems ----------------------------------------------------------------------------------------------
eq(plain(model.statusOf(container('a'))), { id: 'running', tone: 'ok', problem: '' });
eq(plain(model.statusOf(container('a', { health: 'unhealthy' }))), { id: 'unhealthy', tone: 'warn', problem: 'unhealthy' });
assert.equal(model.statusOf(container('a', { health: 'starting' })).id, 'starting');
assert.equal(model.problemOf(container('a', { health: 'starting' })), '', 'a starting health check is not a problem');
assert.equal(model.statusOf(container('a', { state: 'restarting', status: 'Restarting (1) 5 seconds ago' })).tone, 'warn');
assert.equal(model.problemOf(container('a', { state: 'restarting' })), 'restarting');
assert.equal(model.problemOf(container('a', { state: 'exited', status: 'Exited (1) 2 minutes ago' })), 'exited');
assert.equal(model.statusOf(container('a', { state: 'exited', status: 'Exited (137) 2 minutes ago' })).code, 137);
assert.equal(model.statusOf(container('a', { state: 'exited', status: 'Exited (137) 2 minutes ago' })).tone, 'bad');
assert.equal(model.problemOf(container('a', { state: 'exited', status: 'Exited (0) 2 minutes ago' })), '', 'a clean exit is no problem');
assert.equal(model.statusOf(container('a', { state: 'exited', status: 'Exited (0) 2 minutes ago' })).tone, 'idle');
assert.equal(model.problemOf(container('a', { state: 'dead' })), 'dead');
assert.equal(model.problemOf(container('a', { state: 'created' })), '');
assert.equal(model.statusOf(container('a', { state: 'paused' })).tone, 'idle');
assert.equal(model.exitCode({ status: 'Up 2 hours' }), null);

// ---- filters, search, counts ------------------------------------------------------------------------------------------
const rows = model.buildRows([hostA, hostB], {
    local: { containers: [
        container('web', { compose_project: 'shop', compose_service: 'web', ports: [{ ip: '0.0.0.0', private: 80, public: 8080, type: 'tcp' }, { ip: '::', private: 80, public: 8080, type: 'tcp' }] }),
        container('db', { compose_project: 'shop', compose_service: 'db', image: 'postgres:16', health: 'unhealthy' }),
        container('worker', { compose_project: 'shop', compose_service: 'worker', state: 'exited', status: 'Exited (2) 1 hour ago' }),
        container('tools', { state: 'exited', status: 'Exited (0) 1 hour ago' }),
        container('aurago', { self: true, protected_owner: 'AuraGo' })
    ] },
    h_nas: { containers: [
        container('web', { compose_project: 'shop', compose_service: 'web', image: 'nginx:1.26' }),
        container('media', { compose_project: 'media', compose_service: 'plex', image: 'plex:latest' }),
        container('loose')
    ] }
}, 'all');
assert.equal(rows.length, 8);
assert.equal(new Set(rows.map(row => row.key)).size, 8, 'row keys are unique across hosts, even for equal container names');
assert.equal(model.buildRows([hostA, hostB], { local: { containers: [container('x')] }, h_nas: { containers: [container('y')] } }, 'h_nas').length, 1, 'a host scope shows only that host');
assert.equal(model.buildRows([hostA], {}, 'all').length, 0, 'a host without loaded containers contributes no rows');
eq(plain(model.counts(rows, '')), { all: 8, running: 6, stopped: 2, problem: 2 });
eq(plain(model.counts(rows, 'shop')), { all: 4, running: 3, stopped: 1, problem: 2 });
eq(model.filterRows(rows, { filter: 'problem' }).map(row => model.containerName(row.c)), ['db', 'worker']);
eq(model.filterRows(rows, { filter: 'stopped' }).map(row => model.containerName(row.c)), ['worker', 'tools']);
assert.equal(model.filterRows(rows, { filter: 'running' }).length, 6);
assert.equal(model.filterRows(rows, { filter: 'all', query: '' }).length, 8);
eq(model.filterRows(rows, { query: 'postgres' }).map(row => model.containerName(row.c)), ['db'], 'search matches the image');
assert.equal(model.filterRows(rows, { query: '8080' }).length, 1, 'search matches a published port');
assert.equal(model.filterRows(rows, { query: 'nas web' }).length, 1, 'every search word must match, the host name counts');
assert.equal(model.filterRows(rows, { query: 'SHOP db' }).length, 1, 'search ignores case');
assert.equal(model.filterRows(rows, { query: 'nothing-like-this' }).length, 0);
assert.equal(model.filterRows(rows, { query: 'plex', filter: 'stopped' }).length, 0, 'search and state filter combine');

// ---- grouping by compose project --------------------------------------------------------------------------------------
const groups = model.groupRows(rows, { multiHost: true, hostOrder: ['local', 'h_nas'] });
eq(groups.map(group => group.key), ['p:h_nas|media', 'p:local|shop', 'p:h_nas|shop', 's:'], 'projects by name then host order, single containers last');
assert.equal(groups[1].hostName, 'Local');
eq(groups[1].rows.map(row => model.containerName(row.c)), ['db', 'web', 'worker'], 'rows sort by service');
eq(plain(groups[1].counts), { total: 3, running: 2, problem: 2 });
assert.equal(groups[1].tone, 'bad', 'a failed container makes the project red');
assert.equal(groups[0].tone, 'ok');
assert.equal(groups[3].kind, 'single');
eq(groups[3].rows.map(row => model.containerName(row.c)), ['aurago', 'loose', 'tools'], 'single containers of all hosts share one group');
assert.equal(model.groupRows(model.buildRows([hostA, hostB], { local: { containers: [container('a', { compose_project: 'p' })] }, h_nas: { containers: [container('a', { compose_project: 'p' })] } }, 'all'), { multiHost: true }).length, 2, 'the same project on two hosts stays two groups');
const single = model.groupRows(model.buildRows([hostA], { local: { containers: [container('a'), container('b')] } }, 'local'), {});
assert.equal(single.length, 1);
assert.equal(model.showGroupHeaders(single), false, 'one group of single containers needs no header');
eq(model.navItems(single, new Set()).map(item => item.type), ['row', 'row']);
assert.equal(model.showGroupHeaders(groups), true);
const collapsedKeys = model.navItems(groups, new Set(['p:local|shop'])).map(item => item.key);
assert.ok(collapsedKeys.includes('g:p:local|shop'), 'a collapsed group keeps its header');
assert.ok(!collapsedKeys.includes('local/' + 'db'.padEnd(64, '0')), 'a collapsed group hides its rows');
assert.ok(model.navItems(groups, new Set()).some(item => item.key === 'local/' + 'db'.padEnd(64, '0')), 'an open group lists its rows');
const partial = model.groupRows(model.buildRows([hostA], { local: { containers: [container('a', { compose_project: 'p' }), container('b', { compose_project: 'p', state: 'exited', status: 'Exited (0) 1 day ago' })] } }, 'local'), {});
assert.equal(partial[0].tone, 'warn', 'a project with a stopped service is partial');

// ---- hosts: health, address, counts -----------------------------------------------------------------------------------
assert.equal(model.hostHealth(hostA, { loaded: true }).tone, 'ok');
assert.equal(model.hostHealth(hostB, undefined).state, 'unknown');
const never = { id: 'h_new', kind: 'tcp', status: { state: 'error', code: 'refused', checked_at: '0001-01-01T00:00:00Z' } };
assert.equal(model.hostHealth(never, undefined).state, 'unknown', 'a zero checked_at means not checked yet, whatever else the status says');
assert.equal(model.hostHealth(never, undefined).tone, 'idle');
assert.equal(model.hostHealth({ status: { state: 'ok', checked_at: '2026-10-26T10:00:00Z' } }, undefined).state, 'ok', 'a real checked_at is trusted');
assert.equal(model.hostHealth(hostB, { loaded: false, error: { code: 'host_unreachable', reason: 'refused', message: 'connect refused' } }).reason, 'refused');
assert.equal(model.hostHealth(hostB, { loaded: true, error: { code: 'host_unreachable', reason: 'weird' } }).reason, 'unknown', 'an unknown reason code falls back');
assert.equal(model.hostHealth({ status: { state: 'error', code: 'tls' } }, undefined).reason, 'tls');
assert.equal(model.hostHealth(hostB, { loaded: true, error: { code: 'docker_error', message: 'boom' } }).tone, 'warn');
assert.equal(model.hostAddress(hostB), '192.0.2.20:22');
assert.equal(model.hostAddress({ kind: 'tls', address: '2001:db8::1', port: 0 }), '[2001:db8::1]:2376');
assert.equal(model.hostAddress({ kind: 'tcp', address: 'docker.lan', port: 4243 }), 'docker.lan:4243');
assert.equal(model.hostAddress(hostA), 'unix:///var/run/docker.sock');
eq(plain(model.totalCounts([hostA, hostB], { local: { containers: [container('a'), container('b', { state: 'exited' })] }, h_nas: { containers: [container('c')] } })), { running: 2, total: 3 });
assert.equal(model.nextColor([{ color: 'blue' }, { color: 'teal' }]), 'purple');
assert.equal(model.colorOf('Teal'), 'teal');
assert.equal(model.colorOf('chartreuse'), 'gray');

// ---- ports, images, actions -------------------------------------------------------------------------------------------
eq(model.portItems(rows[0].c.ports).map(item => item.label), ['8080→80/tcp'], 'IPv4 and IPv6 bindings of one port show once');
eq(model.portItems([{ private: 9000, type: 'tcp' }, { private: 80, public: 80, type: 'tcp' }]).map(item => item.label), ['80→80/tcp', '9000/tcp'], 'published ports come first');
assert.equal(model.shortImage('docker.io/library/nginx:1.27'), 'nginx:1.27');
assert.equal(model.shortImage('ghcr.io/org/app:v1'), 'ghcr.io/org/app:v1');
assert.equal(model.shortImage('sha256:0123456789abcdef0123456789abcdef'), 'sha256:0123456789ab');
eq(plain(model.actionsFor(container('a'), false)), { toggle: 'stop', restart: true, menu: ['pause', 'kill'], remove: true });
eq(plain(model.actionsFor(container('a', { state: 'exited' }), false)), { toggle: 'start', restart: false, menu: [], remove: true });
eq(plain(model.actionsFor(container('a', { state: 'paused' }), false)), { toggle: 'unpause', restart: false, menu: ['kill'], remove: true });
eq(plain(model.actionsFor(container('a'), true)), { toggle: '', restart: false, menu: [], remove: false }, 'read-only hides every mutating action');
assert.equal(model.needsProtectedConfirm(rows[4].c, 'stop'), true);
assert.equal(model.needsProtectedConfirm(rows[4].c, 'start'), false);
assert.equal(model.needsProtectedConfirm(rows[0].c, 'stop'), false);
eq(plain(model.protectedReason({ protected_owner: 'Invasion' })), { kind: 'owner', owner: 'Invasion' });
assert.equal(model.protectedReason(container('a')), null);

// ---- host form ---------------------------------------------------------------------------------------------------------
const form = overrides => Object.assign(model.emptyHostForm([]), { name: 'NAS', address: '192.0.2.20', sshUser: 'root', sshSecret: 'secret' }, overrides);
assert.equal(model.validateHostForm(form({})), null);
assert.equal(model.validateHostForm(form({ name: '  ' })).code, 'name');
assert.equal(model.validateHostForm(form({ name: 'x'.repeat(65) })).code, 'name');
assert.equal(model.validateHostForm(form({ address: '' })).code, 'address');
assert.equal(model.validateHostForm(form({ address: 'tcp://192.0.2.20:2375' })).code, 'address_scheme');
assert.equal(model.validateHostForm(form({ address: 'host/path' })).code, 'address_scheme');
assert.equal(model.validateHostForm(form({ address: '[2001:db8::1]' })), null, 'a bracketed IPv6 address is fine');
assert.equal(model.validateHostForm(form({ port: '70000' })).code, 'port');
assert.equal(model.validateHostForm(form({ port: 'abc' })).code, 'port');
assert.equal(model.validateHostForm(form({ port: '2222' })), null);
assert.equal(model.validateHostForm(form({ sshUser: '' })).code, 'ssh_user');
assert.equal(model.validateHostForm(form({ sshSecret: '' })).code, 'ssh_secret');
assert.equal(model.validateHostForm(form({ id: 'h_1', originalKind: 'ssh', secretSet: true, sshSecret: '' })), null, 'editing keeps the stored secret');
assert.equal(model.validateHostForm(form({ id: 'h_1', originalKind: 'tls', secretSet: true, sshSecret: '' })).code, 'ssh_secret', 'a stored TLS bundle is no SSH secret');
assert.equal(model.validateHostForm(form({ id: 'h_1', originalKind: 'ssh', secretSet: true, clearSecret: true, sshSecret: '' })).code, 'ssh_secret', 'removing the secret needs a new one');
assert.equal(model.validateHostForm(form({ kind: 'tcp', sshUser: '', sshSecret: '' })), null, 'plain TCP has no credentials');
assert.equal(model.validateHostForm(form({ kind: 'tls', sshUser: '', sshSecret: '' })), null, 'server TLS may use the system roots');
assert.equal(model.validateHostForm(form({ kind: 'mtls', sshUser: '', sshSecret: '' })).code, 'tls_cert');
assert.equal(model.validateHostForm(form({ kind: 'mtls', tlsCert: 'C', tlsKey: '' })).code, 'tls_cert', 'a new mutual-TLS host needs the pair');
assert.equal(model.validateHostForm(form({ id: 'h_1', originalKind: 'mtls', secretSet: true, kind: 'mtls', tlsCert: 'C', tlsKey: '' })), null, 'an edit may replace one PEM field, the server merges them');
assert.equal(model.validateHostForm(form({ name: 'local' })).code, 'name_reserved');
assert.equal(model.validateHostForm(form({ name: ' LOCAL ' })).code, 'name_reserved');
assert.equal(model.validateHostForm(form({ name: 'nas' }), [{ id: 'h_x', name: 'NAS' }]).code, 'name_taken', 'names are unique ignoring case');
assert.equal(model.validateHostForm(form({ id: 'h_x', originalKind: 'ssh', secretSet: true, sshSecret: '' }), [{ id: 'h_x', name: 'NAS' }]), null, 'a host may keep its own name');
assert.equal(model.validateHostForm(form({ kind: 'mtls', tlsCert: 'C', tlsKey: 'K' })), null);
assert.equal(model.validateHostForm(form({ stacksDir: 'stacks' })).code, 'path');
assert.equal(model.validateHostForm(form({ socketPath: '/var/../run/docker.sock' })).code, 'path');
eq(plain(model.hostPayload(form({ port: '2222', color: 'coral' }))), {
    name: 'NAS', kind: 'ssh', address: '192.0.2.20', port: 2222, stacks_dir: '/opt/stacks', host_socket_path: '/var/run/docker.sock', color: 'coral', ssh_user: 'root', sort: 0, ssh_secret: 'secret'
});
const keep = model.hostPayload(form({ id: 'h_1', originalKind: 'ssh', secretSet: true, sshSecret: '' }));
assert.ok(!('ssh_secret' in keep) && !('tls' in keep) && !('clear_secret' in keep), 'an empty secret field keeps the stored secret');
eq(plain(model.hostPayload(form({ kind: 'tls', tlsCA: 'CA' })).tls), { ca: 'CA' }, 'only the typed PEM fields are sent');
eq(plain(model.hostPayload(form({ kind: 'mtls', tlsCert: 'C', tlsKey: 'K' })).tls), { cert: 'C', key: 'K' });
assert.ok(!('tls' in model.hostPayload(form({ kind: 'tls' }))), 'no PEM typed, no tls object');
assert.equal(model.hostPayload(form({ kind: 'tcp', sshUser: 'root', sshSecret: 'x' })).ssh_user, '', 'only SSH sends a user');
assert.ok(!('ssh_secret' in model.hostPayload(form({ kind: 'tcp', sshSecret: 'x' }))), 'only SSH sends an SSH secret');
assert.equal(model.hostPayload(form({ clearSecret: true })).clear_secret, true);
eq([model.hostPayload(form({ stacksDir: '', socketPath: '' })).stacks_dir, model.hostPayload(form({ stacksDir: '', socketPath: '' })).host_socket_path], ['', ''], 'an empty folder or socket is sent empty: the server keeps the stored value on edit and applies the default on create');
const edited = model.hostToForm({ id: 'h_1', name: 'NAS', kind: 'tls', address: 'nas', port: 2376, ssh_user: '', stacks_dir: '/srv/stacks', color: 'amber', secret_set: true, sort: 3 });
eq([edited.originalKind, edited.port, edited.stacksDir, edited.secretSet, edited.sort], ['tls', '2376', '/srv/stacks', true, 3]);

// ---- errors ------------------------------------------------------------------------------------------------------------
assert.equal(model.normalizeError(Object.assign(new Error('x'), { status: 502, body: { status: 'error', code: 'host_unreachable', reason: 'dns', message: 'lookup failed' } })).reason, 'dns');
assert.equal(model.normalizeError(Object.assign(new Error('x'), { status: 409, body: { code: 'container_protected_confirmation_required', owner: 'AuraGo' } })).owner, 'AuraGo');
assert.equal(model.normalizeError(Object.assign(new Error('x'), { status: 400, body: { code: 'invalid_host', field: 'address', message: 'bad' } })).field, 'address');
assert.equal(model.normalizeError(new TypeError('Failed to fetch')).code, 'network');
assert.equal(model.normalizeError(Object.assign(new Error('HTTP 403'), { status: 403, body: {} })).code, 'forbidden');
assert.equal(model.normalizeError(Object.assign(new Error('HTTP 500'), { status: 500, body: {} })).code, 'unknown');
assert.equal(model.normalizeError({ name: 'AbortError' }).code, 'aborted');

// ---- store with a fake api -----------------------------------------------------------------------------------------------
const httpError = (status, body) => Object.assign(new Error(body.message || 'HTTP ' + status), { status, body: { status: 'error', ...body } });
function fakeApi(routes) {
    const calls = [];
    const api = (url, init = {}) => {
        const method = init.method || 'GET';
        const key = method + ' ' + url.replace(/\?.*$/, '');
        calls.push({ method, url, key, body: init.body ? JSON.parse(init.body) : undefined });
        const handler = routes[key];
        if (handler === undefined) return Promise.reject(httpError(404, { code: 'not_found', message: 'no route ' + key }));
        return new Promise((resolve, reject) => {
            const abort = () => reject(Object.assign(new Error('aborted'), { name: 'AbortError' }));
            if (init.signal?.aborted) return abort();
            init.signal?.addEventListener('abort', abort, { once: true });
            Promise.resolve().then(() => (typeof handler === 'function' ? handler(url, init) : handler)).then(resolve, reject);
        });
    };
    api.calls = calls;
    api.count = key => calls.filter(call => call.key === key).length;
    return api;
}
const tick = () => new Promise(resolve => setTimeout(resolve, 0));
const B = '/api/docker-manager';

{
    // refresh loads the host list, then one container list per host; an unreachable host keeps the others working
    let nasUp = false;
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { status: 'ok', hosts: [hostA, hostB] },
        ['GET ' + B + '/hosts/local/containers']: { status: 'ok', containers: [container('web')] },
        ['GET ' + B + '/hosts/h_nas/containers']: () => { if (!nasUp) throw httpError(502, { code: 'host_unreachable', reason: 'timeout', message: 'dial timeout' }); return { status: 'ok', containers: [container('media')] }; }
    });
    const store = Store.create({ api });
    await store.refresh();
    assert.equal(store.state.hosts.length, 2);
    assert.equal(store.state.byHost.local.containers.length, 1);
    assert.equal(store.state.byHost.h_nas.error.code, 'host_unreachable');
    assert.equal(store.state.byHost.h_nas.error.reason, 'timeout');
    assert.equal(store.state.byHost.h_nas.loaded, false);
    assert.equal(model.hostHealth(store.state.hosts[1], store.state.byHost.h_nas).tone, 'bad');
    nasUp = true;
    await store.loadContainers('h_nas');
    assert.equal(store.state.byHost.h_nas.error, null, 'a recovered host clears its error');
    nasUp = false;
    await store.loadContainers('h_nas');
    assert.equal(store.state.byHost.h_nas.containers.length, 1, 'a failed refresh keeps the last containers of that host');
    assert.equal(store.state.byHost.h_nas.error.code, 'host_unreachable');
    store.setScope('h_nas');
    assert.equal(store.state.scope, 'h_nas');
    store.setScope('does-not-exist');
    assert.equal(store.state.scope, 'all', 'an unknown host id falls back to all hosts');
    store.dispose();
}

{
    // one request in flight per key
    let release;
    const gate = new Promise(resolve => { release = resolve; });
    const api = fakeApi({ ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/containers']: () => gate.then(() => ({ containers: [container('slow')] })) });
    const store = Store.create({ api });
    await store.loadHosts();
    const first = store.loadContainers('local'), second = store.loadContainers('local');
    assert.equal(first, second, 'the second call joins the request in flight');
    assert.equal(api.count('GET ' + B + '/hosts/local/containers'), 1);
    assert.equal(store.pending(), true);
    release();
    await first;
    assert.equal(store.pending(), false);
    assert.equal(store.state.byHost.local.containers[0].names[0], 'slow');
    store.dispose();
}

{
    // actions: POST, then a fresh list; busy blocks a double click; read_only hides the mutating actions
    let listCalls = 0;
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] },
        ['GET ' + B + '/hosts/local/containers']: () => ({ containers: [container('web', { state: ++listCalls > 1 ? 'exited' : 'running' })] }),
        ['POST ' + B + '/hosts/local/containers/abc/stop']: {},
        ['POST ' + B + '/hosts/local/containers/abc/kill']: () => { throw httpError(403, { code: 'read_only', message: 'read only' }); },
        ['DELETE ' + B + '/hosts/local/containers/abc']: {}
    });
    const store = Store.create({ api });
    await store.refresh();
    const pendingStop = store.act('local', 'abc', 'stop', {});
    const doubleClick = await store.act('local', 'abc', 'stop', {});
    assert.equal(doubleClick.ok, false);
    assert.equal(doubleClick.error.code, 'busy');
    assert.equal(store.state.busy.size, 1);
    assert.equal((await pendingStop).ok, true);
    assert.equal(store.state.busy.size, 0);
    assert.equal(store.state.byHost.local.containers[0].state, 'exited', 'the list is reloaded after an action');
    assert.equal(store.canMutate(), true);
    const killed = await store.act('local', 'abc', 'kill', {});
    eq([killed.ok, killed.error.code], [false, 'read_only']);
    assert.equal(store.state.dockerReadonly, true);
    assert.equal(store.canMutate(), false);
    assert.equal(store.canEditHosts(), true, 'docker.readonly does not stop host management');
    await store.remove('local', 'abc', { force: true, confirmProtected: true });
    assert.equal(api.calls.find(call => call.method === 'DELETE').url, B + '/hosts/local/containers/abc?force=1&confirm=protected');
    await store.act('local', 'abc', 'restart').catch(() => {});
    assert.equal(api.calls.filter(call => call.method === 'POST').pop().url, B + '/hosts/local/containers/abc/restart', 'actions carry no confirmation, the server never asks for one');
    store.dispose();
}

{
    // docker disabled; the desktop read-only policy
    const api = fakeApi({ ['GET ' + B + '/hosts']: () => { throw httpError(503, { code: 'docker_disabled', message: 'off' }); } });
    const store = Store.create({ api, readonly: true });
    await store.refresh();
    assert.equal(store.state.disabled, true);
    assert.equal(store.state.hostsFailed, null);
    assert.equal(store.canMutate(), false);
    assert.equal(store.canEditHosts(), false);
    store.dispose();
}

{
    // polling is gated by visibility and never overlaps
    const api = fakeApi({ ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/containers']: { containers: [] } });
    const store = Store.create({ api });
    assert.equal(store.pollOnce(), false, 'a hidden window does not poll');
    store.setVisible(true);
    await tick(); await tick(); await tick();
    const initial = api.count('GET ' + B + '/hosts/local/containers');
    assert.ok(initial >= 1);
    const hostCalls = api.count('GET ' + B + '/hosts');
    assert.equal(store.pollOnce(), true);
    await tick(); await tick();
    assert.equal(api.count('GET ' + B + '/hosts/local/containers'), initial + 1);
    assert.equal(api.count('GET ' + B + '/hosts'), hostCalls + 1, 'the host list is polled with the containers, the server refreshes stale statuses behind it');
    store.setVisible(false);
    assert.equal(store.pollOnce(), false);
    store.dispose();
    assert.equal(store.pollOnce(), false);
}

{
    // the browser giving up on a slow host reads as the host being unreachable (timeout), not as a generic failure
    const api = fakeApi({ ['GET ' + B + '/hosts']: { hosts: [hostB] }, ['GET ' + B + '/hosts/h_nas/containers']: () => new Promise(() => {}) });
    const store = Store.create({ api, requestTimeoutMs: 30 });
    await store.loadHosts();
    await store.loadContainers('h_nas');
    eq([store.state.byHost.h_nas.error.code, store.state.byHost.h_nas.error.reason], ['host_unreachable', 'timeout']);
    assert.equal(model.hostHealth(store.state.hosts[0], store.state.byHost.h_nas).reason, 'timeout');
    store.dispose();
}

{
    // hiding the window aborts a slow poll without recording an error
    const api = fakeApi({ ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/containers']: () => new Promise(() => {}) });
    const store = Store.create({ api });
    await store.loadHosts();
    store.setVisible(true);
    await tick();
    store.setVisible(false);
    await tick(); await tick();
    assert.equal(store.state.byHost.local, undefined, 'an aborted poll records nothing');
    store.dispose();
}

{
    // detail drawer: logs and inspect on demand, stale answers dropped
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] },
        ['GET ' + B + '/hosts/local/containers']: { containers: [container('web')] },
        ['GET ' + B + '/hosts/local/containers/' + 'web'.padEnd(64, '0') + '/logs']: (url) => ({ logs: 'line from ' + url.split('tail=')[1] }),
        ['GET ' + B + '/hosts/local/containers/' + 'web'.padEnd(64, '0') + '/inspect']: { inspect: { Id: 'x' } }
    });
    const store = Store.create({ api });
    await store.refresh();
    const row = model.buildRows(store.state.hosts, store.state.byHost, 'all')[0];
    store.openDetail(row);
    assert.equal(store.state.detail.tab, 'overview');
    assert.equal(api.count('GET ' + B + '/hosts/local/containers/' + 'web'.padEnd(64, '0') + '/logs'), 0, 'logs load only when the tab opens');
    store.setDetailTab('logs');
    await tick(); await tick();
    assert.equal(store.state.detail.logs.status, 'ok');
    assert.equal(store.state.detail.logs.text, 'line from 200');
    await store.loadLogs(500);
    assert.equal(store.state.detail.logs.text, 'line from 500');
    await store.loadLogs(7);
    assert.equal(store.state.detail.logs.text, 'line from 500', 'an unsupported tail keeps the current one');
    store.setDetailTab('inspect');
    await tick(); await tick();
    eq(plain(store.state.detail.inspect.data), { Id: 'x' });
    const stale = store.loadLogs(100);
    store.closeDetail();
    await stale;
    assert.equal(store.state.detail, null, 'closing the drawer drops a late answer');
    store.dispose();
}

{
    // host management
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] },
        ['POST ' + B + '/hosts']: (url, init) => ({ host: { id: 'h_new', name: JSON.parse(init.body).name, kind: 'tcp', address: 'a', status: { state: 'unknown' } } }),
        ['GET ' + B + '/hosts/h_new/containers']: { containers: [] },
        ['POST ' + B + '/hosts/test']: { host_status: { state: 'ok', engine_version: '27.1.0', api_version: '1.46' } },
        ['DELETE ' + B + '/hosts/h_new']: {},
        ['GET ' + B + '/nests']: { nests: [{ id: 'n1', name: 'Nest', deploy_method: 'docker_ssh' }] },
        ['POST ' + B + '/hosts/import-nest/n1']: { host: { id: 'h_n1', name: 'Nest', kind: 'ssh', address: 'n', status: { state: 'unknown' } } },
        ['POST ' + B + '/hosts/h_new/test']: { host: { id: 'h_new', name: 'edge', kind: 'tcp', address: 'a', status: { state: 'ok' } } }
    });
    const store = Store.create({ api });
    await store.loadHosts();
    assert.equal((await store.testDraft({ name: 'x' })).engine_version, '27.1.0');
    assert.ok(!('id' in api.calls.find(call => call.key === 'POST ' + B + '/hosts/test').body), 'a new host is tested without an id');
    await store.testDraft({ name: 'x' }, 'h_new');
    assert.equal(api.calls.filter(call => call.key === 'POST ' + B + '/hosts/test').pop().body.id, 'h_new', 'an edited host is tested with its id so the stored secrets are reused');
    const created = await store.saveHost('', { name: 'edge', kind: 'tcp' });
    assert.equal(created.id, 'h_new');
    assert.equal(store.state.hosts.length, 2);
    assert.equal((await store.testHost('h_new')).status.state, 'ok');
    assert.equal(store.state.hosts.find(host => host.id === 'h_new').status.state, 'ok');
    assert.equal((await store.loadNests())[0].id, 'n1');
    assert.equal((await store.importNest('n1')).id, 'h_n1');
    store.setScope('h_new');
    await store.deleteHost('h_new');
    assert.equal(store.state.scope, 'all', 'deleting the scoped host falls back to all hosts');
    assert.ok(!store.state.hosts.some(host => host.id === 'h_new'));
    await assert.rejects(store.saveHost('local', { name: 'x' }), error => error.dm.code === 'not_found', 'a failed save rejects with the normalized info');
    store.dispose();
}

console.log('docker-manager ui tests: model and store ok');
