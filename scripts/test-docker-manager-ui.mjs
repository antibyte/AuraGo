// Docker Manager desktop app: Node tests for the pure model and the store (with a fake api). Later tasks append the load
// contract of the five modules, the stylesheet rules and the translation keys. Run: node scripts/test-docker-manager-ui.mjs
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const files = ['docker-manager-model.js', 'docker-manager-stack-model.js', 'docker-manager-charts.js', 'docker-manager-store.js', 'docker-manager-live.js', 'docker-manager-logs.js', 'docker-manager-terminal.js'];
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
// an SSH host as the edit form loads it: stored secret, nothing typed, the endpoint it was saved with
const storedSsh = overrides => form({ id: 'h_1', originalKind: 'ssh', originalAddress: '192.0.2.20', originalPort: '', originalSshUser: 'root', secretSet: true, sshSecret: '', ...overrides });
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
assert.equal(model.validateHostForm(storedSsh({})), null, 'editing keeps the stored secret');
assert.equal(model.validateHostForm(form({ id: 'h_1', originalKind: 'tls', secretSet: true, sshSecret: '' })).code, 'ssh_secret', 'a stored TLS bundle is no SSH secret');
assert.equal(model.validateHostForm(storedSsh({ clearSecret: true })).code, 'ssh_secret', 'removing the secret needs a new one');
assert.equal(model.validateHostForm(form({ kind: 'tcp', sshUser: '', sshSecret: '' })), null, 'plain TCP has no credentials');
assert.equal(model.validateHostForm(form({ kind: 'tls', sshUser: '', sshSecret: '' })), null, 'server TLS may use the system roots');
assert.equal(model.validateHostForm(form({ kind: 'mtls', sshUser: '', sshSecret: '' })).code, 'tls_cert');
assert.equal(model.validateHostForm(form({ kind: 'mtls', tlsCert: 'C', tlsKey: '' })).code, 'tls_cert', 'a new mutual-TLS host needs the pair');
assert.equal(model.validateHostForm(form({ id: 'h_1', originalKind: 'mtls', secretSet: true, kind: 'mtls', tlsCert: 'C', tlsKey: '' })), null, 'an edit may replace one PEM field, the server merges them');
assert.equal(model.validateHostForm(form({ name: 'local' })).code, 'name_reserved');
assert.equal(model.validateHostForm(form({ name: ' LOCAL ' })).code, 'name_reserved');
assert.equal(model.validateHostForm(form({ name: 'nas' }), [{ id: 'h_x', name: 'NAS' }]).code, 'name_taken', 'names are unique ignoring case');
assert.equal(model.validateHostForm(storedSsh({ id: 'h_x' }), [{ id: 'h_x', name: 'NAS' }]), null, 'a host may keep its own name');
assert.equal(model.validateHostForm(form({ kind: 'mtls', tlsCert: 'C', tlsKey: 'K' })), null);
assert.equal(model.validateHostForm(form({ stacksDir: 'stacks' })).code, 'path');
assert.equal(model.validateHostForm(form({ socketPath: '/var/../run/docker.sock' })).code, 'path');
for (const bad of ['/opt/my stacks', '/opt/a:b', '/opt/a,b', '/opt/a=b', '/opt/"x"', "/opt/it's", '/opt/a`b', '/opt/a\\b', '/opt/a\x1fb']) {
    eq(model.validateHostForm(form({ stacksDir: bad })), { field: 'stacksDir', code: 'path' }, 'the server rejects ' + JSON.stringify(bad) + ' in a path, so does the form');
}
assert.equal(model.validateHostForm(form({ stacksDir: '/srv/compose-stacks/prod_1', socketPath: '/run/user/1000/docker.sock' })), null);
{
    const usable = overrides => model.storedSecretUsable(storedSsh(overrides));
    assert.equal(usable({}), true, 'the same endpoint keeps the stored SSH secret');
    assert.equal(usable({ address: '  192.0.2.20 ' }), true, 'the address is compared trimmed');
    assert.equal(usable({ address: '192.0.2.21' }), false, 'another address needs the secret again');
    assert.equal(usable({ originalAddress: 'NAS.lan', address: ' nas.LAN ' }), true, 'the address is compared ignoring case');
    assert.equal(usable({ originalAddress: '2001:db8::1', address: '[2001:db8::1]' }), true, 'IPv6 brackets do not count as a change');
    assert.equal(usable({ port: '22' }), true, 'an empty port and 22 are the same SSH port');
    assert.equal(usable({ originalPort: '22', port: '' }), true, 'also the other way round');
    assert.equal(usable({ port: '2222' }), false, 'another port needs the secret again');
    assert.equal(usable({ originalPort: '2222', port: '2222' }), true);
    assert.equal(usable({ sshUser: 'admin' }), false, 'another user needs the secret again');
    assert.equal(usable({ sshUser: ' root ' }), true, 'the user is compared trimmed');
    assert.equal(usable({ kind: 'tls', originalKind: 'tls' }), true, 'TLS material is not tied to the SSH endpoint');
    assert.equal(model.storedSecretUsable(form({ id: 'h_1', kind: 'ssh', originalKind: 'tls', originalAddress: '192.0.2.20', originalSshUser: 'root', secretSet: true })), false, 'a stored TLS bundle is no SSH secret');
    assert.equal(model.storedSecretUsable(form({ id: 'h_1', kind: 'tcp', originalKind: 'ssh', originalAddress: '192.0.2.20', secretSet: true })), false, 'another kind of secret cannot stay');
    const tlsHost = { id: 'h_t', kind: 'tls', originalKind: 'tls', originalAddress: 'old.lan', originalPort: '2376', secretSet: true, address: 'new.lan', port: '4000', sshUser: '' };
    assert.equal(model.storedSecretUsable(form(tlsHost)), true, 'a TLS host keeps its stored material when its address or port changes');
    assert.equal(model.storedSecretUsable(form({ ...tlsHost, kind: 'mtls', originalKind: 'mtls' })), true);
    assert.equal(model.validateHostForm(storedSsh({ address: '192.0.2.21' })).code, 'ssh_secret', 'saving or testing with a changed address and no secret is refused up front');
    assert.equal(model.validateHostForm(storedSsh({ sshUser: 'admin' })).code, 'ssh_secret');
    assert.equal(model.validateHostForm(storedSsh({ port: '2222', sshSecret: 'typed' })), null, 'a typed secret is enough after a change');
    assert.equal(model.sshSecretNeedsReentry(storedSsh({ port: '2222' })), true);
    assert.equal(model.sshSecretNeedsReentry(storedSsh({})), false);
    assert.equal(model.sshSecretNeedsReentry(storedSsh({ port: '2222', clearSecret: true })), false, 'removing the secret is its own explanation');
    assert.equal(model.sshSecretNeedsReentry(storedSsh({ port: '2222', kind: 'tcp' })), false, 'only an SSH form asks again');
    const loaded = model.hostToForm({ id: 'h_1', name: 'NAS', kind: 'ssh', address: 'nas.lan', port: 2222, ssh_user: 'root', secret_set: true });
    eq([loaded.originalAddress, loaded.originalPort, loaded.originalSshUser], ['nas.lan', '2222', 'root']);
    assert.equal(model.storedSecretUsable(loaded), true, 'a loaded host form starts with its secret usable');
    assert.equal(model.storedSecretUsable({ ...loaded, address: 'other.lan' }), false);
}
eq(plain(model.hostPayload(form({ port: '2222', color: 'coral' }))), {
    name: 'NAS', kind: 'ssh', address: '192.0.2.20', port: 2222, stacks_dir: '/opt/stacks', host_socket_path: '/var/run/docker.sock', color: 'coral', ssh_user: 'root', sort: 0, ssh_secret: 'secret'
});
const keep = model.hostPayload(storedSsh({}));
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
    // docker.readonly comes with every host list: it hides the actions before the first refusal and clears when lifted
    let readonly = true;
    const api = fakeApi({ ['GET ' + B + '/hosts']: () => ({ hosts: [hostA], docker_readonly: readonly }) });
    const store = Store.create({ api });
    assert.equal(store.canMutate(), true, 'nothing is known before the first list');
    await store.loadHosts();
    assert.equal(store.state.dockerReadonly, true);
    assert.equal(store.canMutate(), false, 'the actions are hidden without a prior refusal');
    assert.equal(store.canEditHosts(), true, 'docker.readonly does not stop host management');
    readonly = false;
    await store.loadHosts();
    assert.equal(store.state.dockerReadonly, false, 'lifting docker.readonly clears the flag');
    assert.equal(store.canMutate(), true);
    const old = Store.create({ api: fakeApi({ ['GET ' + B + '/hosts']: { hosts: [hostA] } }) });
    old.state.dockerReadonly = true;
    await old.loadHosts();
    assert.equal(old.state.dockerReadonly, true, 'an answer without the field leaves what a 403 taught');
    old.dispose();
    store.dispose();
}

{
    // the 403 path still works, and the next list may clear it again
    let readonly = false, listed = 0;
    const api = fakeApi({
        ['GET ' + B + '/hosts']: () => { listed += 1; return { hosts: [hostA], docker_readonly: readonly }; },
        ['GET ' + B + '/hosts/local/containers']: { containers: [container('web')] },
        ['POST ' + B + '/hosts/local/containers/abc/stop']: () => { throw httpError(403, { code: 'read_only', message: 'read only' }); }
    });
    const store = Store.create({ api });
    await store.refresh();
    assert.equal(store.canMutate(), true);
    readonly = true;
    const stopped = await store.act('local', 'abc', 'stop');
    eq([stopped.ok, stopped.error.code, store.state.dockerReadonly], [false, 'read_only', true]);
    readonly = false;
    await store.loadHosts();
    assert.equal(store.state.dockerReadonly, false);
    assert.ok(listed >= 2);
    store.dispose();
}

{
    // the desktop policy event sets the desktop read-only flag from "readonly" only; "enabled" is the Virtual Desktop's own switch
    const listeners = {};
    const document = { hidden: false, addEventListener: (name, fn) => { listeners[name] = fn; }, removeEventListener: name => { delete listeners[name]; } };
    const sandbox = { window: {}, document, console, setTimeout, clearTimeout, AbortController, Intl, Map, Set, JSON, Math, Date, URL, performance };
    vm.runInNewContext(files.map(file => read('ui/js/desktop/apps/' + file)).join('\n'), sandbox);
    const api = fakeApi({ ['GET ' + B + '/hosts']: { hosts: [] } });
    const store = sandbox.window.DockerManagerStore.create({ api });
    store.attach({});
    const policy = detail => listeners['aurago:desktop-policy']({ detail });
    policy({ enabled: false });
    assert.equal(store.state.readonly, false, 'enabled: false does not make the app read-only');
    policy({ readonly: true });
    assert.equal(store.state.readonly, true);
    policy({ enabled: true });
    assert.equal(store.state.readonly, true, 'enabled: true does not lift read-only');
    policy({ readonly: false, enabled: false });
    assert.equal(store.state.readonly, false);
    policy({});
    assert.equal(store.state.readonly, false, 'an event without the flag changes nothing');
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
    // detail drawer: inspect on demand, stale answers dropped; the log tab is a follow panel and fetches nothing here
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] },
        ['GET ' + B + '/hosts/local/containers']: { containers: [container('web')] },
        ['GET ' + B + '/hosts/local/containers/' + 'web'.padEnd(64, '0') + '/inspect']: { inspect: { Id: 'x' } }
    });
    const store = Store.create({ api });
    await store.refresh();
    const row = model.buildRows(store.state.hosts, store.state.byHost, 'all')[0];
    store.openDetail(row);
    assert.equal(store.state.detail.tab, 'overview');
    store.setDetailTab('logs');
    store.setDetailTab('terminal');
    await tick(); await tick();
    eq([store.state.detail.tab, api.calls.filter(call => /inspect|logs|terminal/.test(call.url)).length], ['terminal', 0], 'the log and terminal tabs fetch nothing through the store');
    store.setDetailTab('inspect');
    await tick(); await tick();
    eq(plain(store.state.detail.inspect.data), { Id: 'x' });
    const stale = store.loadInspect();
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

{
    // a draft the server cannot test (a rejected field, missing credentials) is a 400 that rejects with the field, as a save does
    const api = fakeApi({
        ['POST ' + B + '/hosts/test']: () => Promise.reject(httpError(400, { code: 'invalid_host', field: 'ssh_secret', message: 'an SSH password or private key is required' }))
    });
    const store = Store.create({ api });
    await assert.rejects(store.testDraft({ name: 'x', kind: 'ssh' }), error => error.dm.code === 'invalid_host' && error.dm.field === 'ssh_secret' && error.dm.status === 400);
    store.dispose();
}

// ---- model: helper containers and error data --------------------------------------------------------------------------
eq(plain(model.protectedReason({ aurago_helper: 'runner' })), { kind: 'helper', owner: 'runner' }, 'a Docker Manager helper container is protected');
eq(plain(model.protectedReason({ aurago_helper: 'io', protected_owner: 'docker_manager' })), { kind: 'helper', owner: 'io' }, 'the server also names docker_manager as the owner of a helper: the helper text wins');
assert.equal(model.normalizeError(Object.assign(new Error('x'), { status: 409, body: { code: 'revision_conflict', file: 'compose', current: { revision: 'abc' } } })).data.current.revision, 'abc', 'the answer body travels with the error');

// ---- stack model: names, status, rows, groups --------------------------------------------------------------------------
const sm = window.DockerManagerStackModel;
assert.ok(sm, 'the stack model publishes its global');
{
    eq(sm.validateName('immich'), null);
    eq(sm.validateName(''), { code: 'empty' });
    eq(sm.validateName('Immich'), { code: 'invalid' });
    eq(sm.validateName('-x'), { code: 'invalid' });
    eq(sm.validateName('a'.repeat(63)), null);
    eq(sm.validateName('a'.repeat(64)), { code: 'invalid' });
    eq(sm.validateName('shop', ['shop', 'media']), { code: 'taken' });
    assert.equal(sm.sanitizeName('My App v2!'), 'my-app-v2');
    assert.equal(sm.sanitizeName('--nginx'), 'nginx');
    assert.equal(sm.firstServiceName('services:\n  immich-server:\n    image: x\n  db:\n    image: y\n'), 'immich-server');
    assert.equal(sm.firstServiceName('version: "3"\nservices:\n  # first\n\n  Web:\n    image: x'), 'web');
    assert.equal(sm.firstServiceName('image: x'), '');

    const stack = (name, extra = {}) => ({ name, host_id: 'local', dir: '/opt/stacks/' + name, compose_file: 'compose.yaml', has_env: false, status: 'running', services: { total: 2, running: 2 }, ports: [], container_ids: [], created: 1760000000, ...extra });
    const listing = (stacks, extra = {}) => ({ host_id: 'local', stacks_dir: '/opt/stacks', stacks, external: [], ignored: [], ...extra });
    const stacksByHost = {
        local: { loaded: true, listing: listing([stack('shop'), stack('blog', { status: 'draft', services: { total: 0, running: 0 } })], { external: [{ name: 'legacy', working_dir: '/srv/legacy', status: 'partial', services: { total: 2, running: 1 }, ports: [], container_ids: [] }] }) },
        h_nas: { loaded: true, listing: listing([stack('media', { host_id: 'h_nas', ports: [{ service: 'plex', public_port: 32400, private_port: 32400, protocol: 'tcp' }] })]) }
    };
    const hs = [hostA, hostB];
    eq(sm.stackRows(hs, stacksByHost, 'all').map(row => row.key), ['local/shop', 'local/blog', 'h_nas/media']);
    eq(sm.stackRows(hs, stacksByHost, 'h_nas').map(row => row.key), ['h_nas/media'], 'a host scope shows only that host');
    eq(sm.externalRows(hs, stacksByHost, 'all').map(row => row.key), ['x:local/legacy']);
    const containerRows = model.buildRows(hs, { local: { containers: [container('shop-web', { compose_project: 'shop' }), container('tools')] }, h_nas: { containers: [container('loose')] } }, 'all');
    const groups = sm.stackGroups(hs, stacksByHost, containerRows, { scope: 'all', query: '' });
    eq(groups.stacks.map(row => row.stack.name), ['blog', 'media', 'shop'], 'own stacks sort by name across hosts');
    eq(groups.singles.map(row => model.containerName(row.c)), ['loose', 'tools'], 'only containers without a compose project are single');
    eq(sm.stackGroups(hs, stacksByHost, containerRows, { scope: 'all', query: 'nas plex' }).stacks.length, 0, 'the search words must all match (plex is a service, not a stack field)');
    eq(sm.stackGroups(hs, stacksByHost, containerRows, { scope: 'all', query: '32400' }).stacks.map(row => row.stack.name), ['media'], 'a published port matches');
    eq(sm.stackGroups(hs, stacksByHost, containerRows, { scope: 'all', query: 'legacy' }).external.length, 1);
    eq(plain(sm.stackCounts(hs, stacksByHost, 'all')), { stacks: 3, external: 1, total: 4 });
    assert.equal(sm.stackTone({ status: 'running' }), 'ok');
    assert.equal(sm.stackTone({ status: 'partial' }), 'warn');
    assert.equal(sm.stackTone({ status: 'error' }), 'bad');
    assert.equal(sm.stackTone({ status: 'draft' }), 'idle');
    assert.equal(sm.stackStatusId({ status: 'weird' }), 'stopped', 'an unknown status reads as stopped');
    assert.equal(sm.hostStacksState(undefined), 'loading');
    assert.equal(sm.hostStacksState({ loaded: true, error: null }), 'ok');
    assert.equal(sm.hostStacksState({ error: { code: 'host_not_prepared' } }), 'not_prepared');
    assert.equal(sm.hostStacksState({ error: { code: 'api_too_old' } }), 'api_too_old');
    assert.equal(sm.hostStacksState({ loaded: true, error: { code: 'host_unreachable' } }), 'unreachable');
    assert.equal(sm.hostStacksState({ error: { code: 'stacks_dir_unavailable' } }), 'unavailable');
    assert.equal(sm.hostStacksState({ error: { code: 'docker_error' }, loaded: false }), 'error');
    eq(plain(sm.stackActions({ status: 'running' })), { primary: 'update', restart: true, stop: true, pull: true, down: true, remove: true });
    eq(plain(sm.stackActions({ status: 'draft' })), { primary: 'deploy', restart: false, stop: false, pull: false, down: false, remove: true });
    assert.equal(sm.stackActions({ status: 'stopped' }).primary, 'deploy');
    assert.equal(sm.serviceTone({ state: 'exited', exit_code: 1 }), 'bad');
    assert.equal(sm.serviceTone({ state: 'exited', exit_code: 0 }), 'idle');
    assert.equal(sm.serviceTone({ state: 'running', health: 'unhealthy' }), 'warn');
    assert.equal(sm.serviceStateId({ state: 'gone' }), 'missing');
}

// ---- stack model: port links, time, bytes ------------------------------------------------------------------------------
{
    const ports = [
        { service: 'web', ip: '0.0.0.0', public_port: 8080, private_port: 80, protocol: 'tcp' },
        { service: 'web', ip: '::', public_port: 8080, private_port: 80, protocol: 'tcp' },
        { service: 'dns', ip: '', public_port: 53, private_port: 53, protocol: 'udp' },
        { service: 'admin', ip: '127.0.0.1', public_port: 9000, private_port: 9000, protocol: 'tcp' },
        { service: 'db', private_port: 5432, protocol: 'tcp' }
    ];
    eq(sm.portLinks(ports, '192.0.2.20').map(link => [link.label, link.url]), [['53', ''], ['8080', 'http://192.0.2.20:8080'], ['9000', '']], 'IPv4 and IPv6 bindings show once; UDP and loopback-only ports are not links; unpublished ports are skipped');
    assert.equal(sm.portLinks([ports[0]], '2001:db8::1')[0].url, 'http://[2001:db8::1]:8080', 'an IPv6 address is bracketed');
    assert.equal(sm.portLinks([ports[0]], '')[0].url, '', 'no address, no link');
    assert.equal(sm.linkAddress({ kind: 'local', address: 'unix:///var/run/docker.sock' }, 'desk.lan'), 'desk.lan', 'the local engine is reached under the name of the page');
    assert.equal(sm.linkAddress({ kind: 'ssh', address: '[2001:db8::1]' }, 'desk.lan'), '2001:db8::1');
    assert.equal(sm.linkAddress({ kind: 'tcp', address: 'docker.lan:2375' }, 'desk.lan'), 'docker.lan');
    const now = 1760000000 * 1000 + 3 * 86400 * 1000 + 5000;
    eq(sm.age(1760000000, now), { unit: 'days', n: 3 });
    eq(sm.age(1760000000, 1760000000 * 1000 + 2 * 3600 * 1000), { unit: 'hours', n: 2 });
    eq(sm.age(1760000000, 1760000000 * 1000 + 90 * 1000), { unit: 'minutes', n: 1 });
    eq(sm.age(1760000000, 1760000000 * 1000 + 5000), { unit: 'seconds', n: 5 });
    assert.equal(sm.age(0, now), null, 'a draft has no age');
    assert.equal(sm.clock(75), '1:15');
    assert.equal(sm.clock(3725), '1:02:05');
    assert.equal(sm.elapsed({ started_at: '2026-10-27T10:00:00Z', finished_at: '2026-10-27T10:01:05Z' }, 0), '1:05');
    assert.equal(sm.elapsed({ created_at: '2026-10-27T10:00:00Z' }, Date.parse('2026-10-27T10:00:09Z')), '0:09', 'a running operation counts from created_at');
    assert.equal(sm.elapsed({}, 0), '');
    eq([sm.formatBytes(0), sm.formatBytes(1536), sm.formatBytes(15 * 1024 * 1024), sm.formatBytes(-1), sm.formatBytes(3 * 1024 ** 3)], ['0 B', '1.5 KB', '15 MB', '', '3.0 GB']);
}

// ---- stack model: lint, .env, dropped files ----------------------------------------------------------------------------
{
    const diags = [
        { line: 5, severity: 'hint', code: 'image_latest', message: 'English', service: 'web' },
        { line: 2, severity: 'error', code: 'service_no_image', message: 'English', service: 'db' },
        { line: 2, severity: 'warning', code: 'variable_missing', message: 'English', params: { variable: 'DB_PASSWORD' }, fix: { kind: 'env_random', variable: 'DB_PASSWORD' } },
        { line: 3, severity: 'warning', code: 'env_syntax', message: 'English', params: { file: '.env', detail: 'expected KEY=value' } },
        { line: 7, severity: 'warning', code: 'brand_new_code', message: 'Only the server knows this' }
    ];
    eq(sm.sortDiagnostics(diags).map(d => d.code), ['service_no_image', 'variable_missing', 'env_syntax', 'image_latest', 'brand_new_code'], 'by line, then severity');
    eq(sm.splitDiagnostics(diags).env.map(d => d.code), ['env_syntax'], 'env_syntax belongs to the .env editor');
    eq(sm.splitDiagnostics(diags).compose.length, 4);
    eq(plain(sm.lintCounts(diags)), { error: 1, warning: 3, hint: 1 });
    assert.equal(sm.worstSeverity(diags), 'error');
    assert.equal(sm.worstSeverity([]), '');
    const t = (key, params) => key.replace('lint_', 'L:') + (params ? JSON.stringify(params) : '');
    assert.equal(sm.diagnosticText(diags[1], t), 'L:service_no_image{"service":"db"}', 'the service of the diagnostic fills {{service}}');
    assert.equal(sm.diagnosticText(diags[2], t), 'L:variable_missing{"variable":"DB_PASSWORD"}', 'the params of the diagnostic fill the text');
    assert.equal(sm.diagnosticText(diags[4], t), 'Only the server knows this', 'an unknown code keeps the server message');
    assert.equal(sm.diagnosticText(diags[0], key => 'Pin the tag of {{service}}'), 'English', 'a text whose placeholder stayed open falls back to the server message');
    eq(sm.randomFixes(diags), ['DB_PASSWORD']);
    eq(sm.randomFixes([{ fix: { kind: 'env_random', variable: 'bad name' } }]), [], 'a variable name that is no valid env name is never offered');
    const index = sm.lineIndex('a: 1\n  b: 2\r\n\nlast');
    eq(plain(sm.lineRange(index, 1)), { line: 1, from: 0, to: 4 });
    eq(plain(sm.lineRange(index, 2)), { line: 2, from: 7, to: 11 }, 'the range starts at the first non-blank character and ends before the line break');
    eq(plain(sm.lineRange(index, 2, 4)), { line: 2, from: 8, to: 11 }, 'a column moves the start');
    eq(plain(sm.lineRange(index, 99)), { line: 4, from: 14, to: 18 }, 'a line beyond the end is the last line');
    eq(plain(sm.lineRange(index, 0)), { line: 1, from: 0, to: 4 }, 'line 0 (the whole file) is the first line');
    assert.equal(sm.envHas('A=1\nexport DB_PASSWORD = x\n', 'DB_PASSWORD'), true);
    assert.equal(sm.envHas('#DB_PASSWORD=1\n', 'DB_PASSWORD'), false, 'a comment defines nothing');
    assert.equal(sm.appendEnvVariable('A=1', 'B', 'x'), 'A=1\nB=x\n');
    assert.equal(sm.appendEnvVariable('', 'B', 'x'), 'B=x\n');
    assert.equal(sm.appendEnvVariable('B=old\n', 'B', 'x'), 'B=old\n', 'an existing variable is never overwritten');
    assert.equal(sm.appendEnvVariable('A=1\n', 'bad name', 'x'), 'A=1\n');
    const fake = { getRandomValues: bytes => { for (let i = 0; i < bytes.length; i += 1) bytes[i] = (i * 37 + 11) % 256; return bytes; } };
    const value = sm.randomValue(32, fake);
    assert.match(value, /^[A-Za-z0-9]{32}$/);
    assert.equal(sm.randomValue(200, fake).length, 128, 'the length is capped');
    eq([sm.classifyDroppedFile('compose.yaml'), sm.classifyDroppedFile('docker-compose.yml'), sm.classifyDroppedFile('C:\\x\\immich.YML'), sm.classifyDroppedFile('.env'), sm.classifyDroppedFile('prod.env'), sm.classifyDroppedFile('notes.txt')], ['compose', 'compose', 'compose', 'env', 'env', '']);
    const preview = { services: [{ name: 'db', volumes: [{ kind: 'named', source: 'pg', target: '/var/lib/postgresql' }, { kind: 'relative', source: './data', target: '/data', read_only: true }] }, { name: 'web', volumes: [{ kind: 'tmpfs', target: '/tmp' }] }] };
    eq(plain(sm.storageGroups(preview)), { named: [{ service: 'db', source: 'pg', target: '/var/lib/postgresql', readOnly: false }], bind: [], relative: [{ service: 'db', source: './data', target: '/data', readOnly: true }], anonymous: [], tmpfs: [{ service: 'web', source: '', target: '/tmp', readOnly: false }] });
}

// ---- stack model: line diff ----------------------------------------------------------------------------------------------
{
    const rows = sm.lineDiff('a\nb\nc\nd\n', 'a\nB\nc\nd\ne\n');
    eq(rows.map(row => row.type + ':' + row.text), ['same:a', 'del:b', 'add:B', 'same:c', 'same:d', 'add:e']);
    eq(rows.map(row => [row.oldNo, row.newNo]), [[1, 1], [2, 0], [0, 2], [3, 3], [4, 4], [0, 5]]);
    assert.equal(sm.diffChanged(rows), true);
    assert.equal(sm.diffChanged(sm.lineDiff('x\ny', 'x\ny\n')), false, 'a final line break alone is no difference');
    assert.equal(sm.diffChanged(sm.lineDiff('', '')), false);
    eq(sm.lineDiff('', 'a').map(row => row.type), ['add']);
    const long = Array.from({ length: 30 }, (unused, index) => 'line' + index).join('\n');
    const collapsed = sm.collapseDiff(sm.lineDiff(long, long.replace('line15', 'LINE15')), 2);
    eq(collapsed.map(row => row.type), ['skip', 'same', 'same', 'del', 'add', 'same', 'same', 'skip']);
    assert.equal(collapsed[0].count, 13);
    const big = Array.from({ length: 2500 }, (unused, index) => 'a' + index).join('\n'), other = Array.from({ length: 2500 }, (unused, index) => 'b' + index).join('\n');
    assert.equal(sm.lineDiff(big, other).length, 5000, 'a huge middle falls back to "all removed, all added" instead of a slow table');
}

// ---- stack model: palette --------------------------------------------------------------------------------------------------
{
    assert.ok(sm.fuzzyScore('Restart immich', 'rest imm') > sm.fuzzyScore('Deploy my-immich', 'imm'), 'a hit at the start of a word ranks higher');
    assert.equal(sm.fuzzyScore('Restart immich', 'zzz'), 0);
    assert.ok(sm.fuzzyScore('Restart immich', 'rsti') > 0, 'a subsequence of two or more letters matches');
    assert.equal(sm.fuzzyScore('Restart immich', 'r z'), 0, 'every word must match');
    assert.equal(sm.fuzzyScore('anything', ''), 1);
    const items = [{ label: 'Deploy shop' }, { label: 'Open shop' }, { label: 'Restart shopping-list' }, { label: 'Prepare NAS' }];
    eq(sm.rankItems(items, 'shop').map(item => item.label), ['Deploy shop', 'Open shop', 'Restart shopping-list'], 'equal scores keep the original order');
    eq(sm.rankItems(items, 'open sh').map(item => item.label), ['Open shop']);
    assert.equal(sm.rankItems(items, '', 2).length, 2, 'the limit applies');
    eq(sm.highlight('Restart immich', 'imm'), [{ text: 'Restart ', hit: false }, { text: 'imm', hit: true }, { text: 'ich', hit: false }]);
    const hs2 = [{ id: 'local', name: 'Local' }, { id: 'h_nas', name: 'NAS' }];
    const row = (hostId, name, status) => ({ key: hostId + '/' + name, hostId, hostName: hostId === 'local' ? 'Local' : 'NAS', stack: { name, status } });
    const label = (action, params) => action + ':' + (params.stack || params.host || '');
    const data = { hosts: hs2, stackRows: [row('local', 'shop', 'running'), row('h_nas', 'blog', 'stopped'), row('local', 'wip', 'partial')], multi: true, hostStates: { local: 'ok', h_nas: 'not_prepared' }, scope: 'all', canMutate: true, label };
    eq(sm.paletteItems(data).map(item => item.id), [
        'open:local/shop', 'deploy:local/shop', 'restart:local/shop', 'stop:local/shop', 'open:h_nas/blog', 'deploy:h_nas/blog',
        'open:local/wip', 'deploy:local/wip', 'update:local/wip', 'restart:local/wip', 'stop:local/wip',
        'new:local', 'switch:local', 'new:h_nas', 'prepare:h_nas', 'switch:h_nas', 'refresh'
    ], 'actions follow the status; an unprepared host offers Prepare');
    assert.equal(sm.paletteItems(data)[1].action, 'update', 'a running stack offers Update, not Deploy');
    assert.equal(sm.paletteItems(data)[0].label, 'open:shop (Local)', 'several hosts: the host is part of the name');
    eq(sm.paletteItems(Object.assign({}, data, { canMutate: false })).map(item => item.action), ['open', 'open', 'open', 'switch_host', 'switch_host', 'refresh'], 'a read-only window gets no mutating command');
    const inside = sm.paletteItems(Object.assign({}, data, { scope: 'h_nas', stackRows: [], multi: false })).map(item => item.id);
    eq(inside.slice(-2), ['switch:all', 'refresh'], 'inside one host there is a way back to all hosts');
    assert.ok(inside.includes('switch:local') && !inside.includes('switch:h_nas'), 'and a way to the other host, but not to the one already shown');
}

// ---- stack model: folding the stream of an operation -------------------------------------------------------------------
{
    const op = { id: 'op_1', host_id: 'local', stack: 'shop', kind: 'up', state: 'running', output: 'hello\n', created_at: '2026-10-27T10:00:00Z' };
    const entry = sm.newOpEntry(op, 1000);
    assert.equal(entry.output, 'hello\n');
    sm.applyOpEvent(entry, { type: 'output', text: 'world\n' });
    assert.equal(entry.output, 'hello\nworld\n');
    sm.applyOpEvent(entry, { type: 'progress', progress: { id: 'db Pulling', status: 'Downloading', current: 50, total: 200 } });
    sm.applyOpEvent(entry, { type: 'progress', progress: { id: 'web Pulling', status: 'Waiting' } });
    sm.applyOpEvent(entry, { type: 'progress', progress: { id: 'db Pulling', current: 100 } });
    eq(sm.progressBars(entry).map(item => [item.id, item.percent]), [['db Pulling', 50]], 'only items with a total get a bar; a later event updates its item');
    assert.equal(entry.progress.length, 2);
    sm.applyOpEvent(entry, { type: 'state', operation: { id: 'op_1', state: 'health_wait' } });
    assert.equal(entry.op.state, 'health_wait');
    assert.equal(entry.op.stack, 'shop', 'a state event updates, it does not replace');
    assert.equal(entry.output, 'hello\nworld\n', 'a state event carries no output');
    assert.equal(sm.isActiveOp(entry.op), true);
    sm.applyOpEvent(entry, { type: 'done', operation: { id: 'op_1', state: 'failed', output: 'full output\n', error: 'boom', error_line: 4 } });
    assert.equal(sm.isActiveOp(entry.op), false);
    assert.equal(entry.output, 'full output\n', 'done carries the whole output');
    sm.applySnapshot(entry, { id: 'op_1', state: 'running', output: 'again\n' });
    eq([entry.output, entry.progress.length], ['again\n', 0], 'a snapshot replaces the output and the progress seen so far');
    const huge = sm.capOutput('x'.repeat(70 * 1024) + '\nlast line');
    assert.ok(huge.length <= sm.OUTPUT_CAP && huge.endsWith('last line'), 'the output keeps its tail');
    const e = n => ({ op: { state: n } });
    eq(sm.trimEntries([e('running'), e('succeeded'), e('failed'), e('queued'), e('succeeded')], 1).map(x => x.op.state), ['running', 'succeeded', 'queued'], 'active entries stay, finished ones are cut to the newest few');
    eq(plain(sm.parseJSON('{"a":1}')), { a: 1 });
    assert.equal(sm.parseJSON('not json'), null);
    assert.equal(sm.draftDirty({ text: 'a' }, { content: 'a' }), false);
    assert.equal(sm.draftDirty({ text: 'a' }, { content: 'b' }), true);
}

// ---- store: stacks ---------------------------------------------------------------------------------------------------------
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
class FakeEventSource {
    static all = [];
    constructor(url) { this.url = url; this.listeners = {}; this.closed = false; FakeEventSource.all.push(this); }
    addEventListener(type, fn) { (this.listeners[type] ||= []).push(fn); }
    close() { this.closed = true; }
    emit(type, data) { for (const fn of this.listeners[type] || []) fn({ data: typeof data === 'string' ? data : JSON.stringify(data) }); }
    fail() { if (this.onerror) this.onerror(); }
}
const stackFixture = (name, extra = {}) => ({ name, host_id: 'local', dir: '/opt/stacks/' + name, compose_file: 'compose.yaml', has_env: false, status: 'running', services: { total: 1, running: 1 }, ports: [], container_ids: [], created: 1760000000, ...extra });
const listingBody = (stacks, extra = {}) => ({ status: 'ok', host_id: 'local', stacks_dir: '/opt/stacks', stacks, external: [], ignored: [], scanned_at: '2026-10-27T10:00:00Z', cached: false, ...extra });
const detailFixture = (extra = {}) => ({
    ...stackFixture('shop'), service_list: [{ name: 'web', image: 'nginx:1.27', state: 'running', ports: [] }],
    compose: { name: 'compose.yaml', content: 'services:\n  web:\n    image: nginx:1.27\n', revision: 'r1', exists: true }, env: { name: '.env', content: '', revision: '', exists: false }, ...extra
});
const opFixture = (extra = {}) => ({ id: 'op_a1', host_id: 'local', stack: 'shop', kind: 'up', options: {}, state: 'queued', output: '', created_at: '2026-10-27T10:00:00Z', ...extra });

{
    // listing: a host that is not prepared, a host with an old API and an unreachable host keep their own state; a refresh asks the server to rescan
    let mode = 'ok';
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA, hostB] },
        ['GET ' + B + '/hosts/local/stacks']: () => {
            if (mode === 'unprepared') throw httpError(409, { code: 'host_not_prepared', message: 'prepare first', helpers: { runner_image: 'docker:29.9.2-cli', runner_image_present: false, io_container: 'missing' } });
            if (mode === 'down') throw httpError(502, { code: 'host_unreachable', reason: 'timeout', message: 'dial timeout' });
            return listingBody([stackFixture('shop'), stackFixture('blog', { status: 'draft' })]);
        },
        ['GET ' + B + '/hosts/h_nas/stacks']: () => { throw httpError(409, { code: 'api_too_old', message: 'old engine', api_version: '1.38' }); }
    });
    const store = Store.create({ api });
    assert.equal(store.state.section, 'stacks', 'the Stacks section is the default');
    await store.loadHosts();
    await Promise.all([store.loadStacks('local'), store.loadStacks('h_nas')]);
    eq(store.state.stacks.local.listing.stacks.map(stack => stack.name), ['shop', 'blog']);
    assert.equal(store.state.stacks.local.loaded, true);
    eq([store.state.stacks.h_nas.error.code, store.state.stacks.h_nas.error.data.api_version, store.state.stacks.h_nas.listing], ['api_too_old', '1.38', null]);
    assert.equal(sm.hostStacksState(store.state.stacks.h_nas), 'api_too_old');
    mode = 'down';
    await store.loadStacks('local');
    eq([store.state.stacks.local.error.reason, store.state.stacks.local.listing.stacks.length], ['timeout', 2], 'an unreachable host keeps the stacks it showed last');
    mode = 'unprepared';
    await store.loadStacks('local');
    eq([store.state.stacks.local.listing, store.state.stacks.local.error.data.helpers.runner_image_present], [null, false], 'a host that answers "not prepared" shows nothing stale, and the answer names what is missing');
    mode = 'ok';
    await store.loadStacks('local', true);
    assert.equal(api.calls.filter(call => call.key === 'GET ' + B + '/hosts/local/stacks').pop().url, B + '/hosts/local/stacks?refresh=1');
    assert.equal(store.state.stacks.local.error, null, 'a recovered host clears its error');
    store.dispose();
}

{
    // the Stacks section loads the listings with the containers; the Containers section leaves the lister alone
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/containers']: { containers: [] }, ['GET ' + B + '/hosts/local/stacks']: listingBody([]),
        ['GET ' + B + '/operations']: { operations: [opFixture({ state: 'running' })] }, ['GET ' + B + '/operations/op_a1/stream']: {}
    });
    const store = Store.create({ api, section: 'containers', live: false, eventSource: url => new FakeEventSource(url) });
    FakeEventSource.all = [];
    store.setVisible(true);
    await tick(); await tick(); await tick(); await tick();
    assert.equal(api.count('GET ' + B + '/hosts/local/stacks'), 0, 'the Containers section does not list stacks');
    assert.equal(api.calls.find(call => call.key === 'GET ' + B + '/operations').url, B + '/operations?active=1', 'opening the window asks for the operations that run on the engine');
    assert.equal(store.state.ops.length, 1, 'a running operation found on the engine joins the drawer');
    assert.equal(FakeEventSource.all.length, 1, 'and its stream is followed');
    store.setSection('stacks');
    await tick(); await tick();
    assert.equal(api.count('GET ' + B + '/hosts/local/stacks'), 1, 'switching to the Stacks section lists the stacks');
    store.setSection('nonsense');
    assert.equal(store.state.section, 'stacks');
    assert.equal(store.pollOnce(), true);
    await tick(); await tick();
    assert.equal(api.count('GET ' + B + '/hosts/local/stacks'), 2, 'the poll lists the stacks again');
    store.dispose();
    assert.equal(FakeEventSource.all[0].closed, true, 'closing the window closes the streams');
}

{
    // the open stack: loaded, saved with the revision it is based on, a revision conflict names the host's version
    let putCalls = 0;
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/stacks']: listingBody([stackFixture('shop')]),
        ['GET ' + B + '/hosts/local/stacks/shop']: () => ({ stack: detailFixture() }),
        ['PUT ' + B + '/hosts/local/stacks/shop/files']: () => {
            putCalls += 1;
            if (putCalls === 2) throw httpError(409, { code: 'revision_conflict', message: 'changed', file: 'compose', current: { name: 'compose.yaml', content: 'host text', revision: 'r9', exists: true } });
            return { revisions: putCalls === 1 ? { compose: 'r2', env: 'e1' } : { compose: 'r10' } };
        },
        ['GET ' + B + '/hosts/local/stacks/ghost']: () => { throw httpError(404, { code: 'stack_not_found', message: 'no such stack' }); }
    });
    const store = Store.create({ api });
    await store.loadHosts();
    store.openStack('local', 'shop', 'compose');
    assert.equal(store.state.stack.status, 'loading');
    await tick(); await tick();
    eq([store.state.stack.status, store.state.stack.tab, store.state.stack.detail.compose.revision], ['ok', 'compose', 'r1']);
    store.setStackTab('env');
    store.setStackTab('bogus');
    assert.equal(store.state.stack.tab, 'env');
    const revisions = await store.saveFiles('local', 'shop', { compose: { content: 'new', revision: 'r1' }, env: { content: 'A=1\n', revision: '' } });
    eq(plain(revisions), { compose: 'r2', env: 'e1' });
    assert.deepStrictEqual(plain(api.calls.find(call => call.method === 'PUT').body), { compose: { content: 'new', revision: 'r1' }, env: { content: 'A=1\n', revision: '' } });
    eq([store.state.stack.detail.compose.content, store.state.stack.detail.compose.revision, store.state.stack.detail.env.exists, store.state.stack.detail.has_env], ['new', 'r2', true, true], 'a saved file becomes the loaded one');
    await assert.rejects(store.saveFiles('local', 'shop', { compose: { content: 'again', revision: 'r1' } }), error => error.dm.code === 'revision_conflict' && error.dm.data.file === 'compose' && error.dm.data.current.revision === 'r9' && error.dm.data.current.content === 'host text');
    assert.equal(store.state.stack.detail.compose.revision, 'r2', 'a refused save changes nothing');
    store.openStack('local', 'ghost');
    await tick(); await tick();
    eq([store.state.stack.status, store.state.stack.error.code], ['error', 'stack_not_found']);
    const late = store.loadStack();
    store.closeStack();
    await late;
    assert.equal(store.state.stack, null, 'closing the pane drops a late answer');
    store.dispose();
}

{
    // an operation: started, followed through its stream, finished; the listing and the open stack are reloaded
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/containers']: { containers: [] }, ['GET ' + B + '/hosts/local/stacks']: listingBody([stackFixture('shop')]),
        ['GET ' + B + '/hosts/local/stacks/shop']: () => ({ stack: detailFixture() }),
        ['POST ' + B + '/hosts/local/stacks/shop/ops']: () => ({ operation: opFixture() }),
        ['GET ' + B + '/operations']: () => ({ operations: [opFixture({ state: 'health_wait' })] })
    });
    FakeEventSource.all = [];
    const store = Store.create({ api, eventSource: url => new FakeEventSource(url) });
    await store.loadHosts();
    store.openStack('local', 'shop');
    await tick(); await tick();
    const finishedIds = [];
    store.onOperationDone(entry => finishedIds.push(entry.id + ':' + entry.op.state));
    const started = await store.startOp('local', 'shop', 'up');
    assert.equal(started.ok, true);
    assert.deepStrictEqual(plain(api.calls.find(call => call.method === 'POST').body), { kind: 'up' }, 'volumes is sent only when asked for');
    await store.startOp('local', 'shop', 'down', { volumes: true });
    assert.deepStrictEqual(plain(api.calls.filter(call => call.method === 'POST').pop().body), { kind: 'down', volumes: true });
    const stream = FakeEventSource.all[0];
    assert.equal(stream.url, B + '/operations/op_a1/stream');
    assert.equal(FakeEventSource.all.length, 1, 'one stream per operation: the second start joined the same id');
    const entry = store.state.ops[0];
    stream.emit('snapshot', opFixture({ state: 'running', output: 'pulling\n' }));
    stream.emit('output', { type: 'output', seq: 1, text: 'more\n' });
    stream.emit('progress', { type: 'progress', seq: 2, progress: { id: 'web', current: 1, total: 4 } });
    stream.emit('state', { type: 'state', seq: 3, operation: opFixture({ state: 'health_wait' }) });
    eq([entry.op.state, entry.output, entry.stream, sm.progressBars(entry)[0].percent], ['health_wait', 'pulling\nmore\n', 'live', 25]);
    assert.equal(store.activeOps().length, 1);
    stream.emit('output', 'this is not json');
    assert.equal(entry.output, 'pulling\nmore\n', 'a broken event is ignored');
    await store.loadActiveOperations();
    eq([entry.output, entry.stream, sm.progressBars(entry).map(bar => bar.percent), entry.op.state, store.state.ops.length], ['pulling\nmore\n', 'live', [25], 'health_wait', 1], 'the list of active operations carries no output: it never wipes the output and the progress the stream delivered');
    const listBefore = api.count('GET ' + B + '/hosts/local/stacks'), detailBefore = api.count('GET ' + B + '/hosts/local/stacks/shop');
    stream.emit('done', opFixture({ state: 'succeeded', output: 'final\n', ports: [{ service: 'web', public_port: 8080, private_port: 80, protocol: 'tcp' }] }));
    await tick(); await tick();
    eq([stream.closed, entry.stream, entry.op.state, entry.output, finishedIds], [true, 'closed', 'succeeded', 'final\n', ['op_a1:succeeded']], 'done closes the stream and tells the listeners once');
    assert.equal(store.activeOps().length, 0);
    assert.ok(api.count('GET ' + B + '/hosts/local/stacks') > listBefore, 'the listing is refreshed after the operation');
    assert.equal(api.calls.filter(call => call.key === 'GET ' + B + '/hosts/local/stacks').pop().url, B + '/hosts/local/stacks?refresh=1');
    assert.ok(api.count('GET ' + B + '/hosts/local/stacks/shop') > detailBefore, 'and so is the open stack');
    store.dispose();
}

{
    // a busy stack attaches to the running operation; a broken stream is read again and followed again with a growing delay
    let state = 'running';
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/containers']: { containers: [] }, ['GET ' + B + '/hosts/local/stacks']: listingBody([stackFixture('shop')]),
        ['POST ' + B + '/hosts/local/stacks/shop/ops']: () => { throw httpError(409, { code: 'operation_in_progress', message: 'busy', operation_id: 'op_old', kind: 'update' }); },
        ['GET ' + B + '/operations/op_old']: () => ({ operation: opFixture({ id: 'op_old', kind: 'update', state, output: 'seen so far\n' }) })
    });
    FakeEventSource.all = [];
    const store = Store.create({ api, eventSource: url => new FakeEventSource(url), reconnectMs: [10, 20] });
    await store.loadHosts();
    const busy = await store.startOp('local', 'shop', 'up');
    eq([busy.ok, busy.busy, busy.error.code], [false, 'op_old', 'operation_in_progress']);
    await sleep(120);
    eq([store.state.ops.length, store.state.ops[0].id, FakeEventSource.all.length], [1, 'op_old', 1], 'the running operation is attached and followed');
    const first = FakeEventSource.all[0];
    first.emit('snapshot', opFixture({ id: 'op_old', kind: 'update', state: 'running', output: 'seen so far\n' }));
    first.fail();
    assert.equal(first.closed, true);
    assert.equal(store.state.ops[0].stream, 'retry');
    await sleep(150);
    assert.equal(FakeEventSource.all.length, 2, 'the operation is read again and the stream reopened');
    const second = FakeEventSource.all[1];
    state = 'succeeded';
    second.fail();
    await sleep(150);
    eq([FakeEventSource.all.length, store.state.ops[0].op.state, store.state.ops[0].stream], [2, 'succeeded', 'closed'], 'an operation that ended while the stream was down is finished without a new stream');
    store.dispose();
}

{
    // an operation named by the listing is picked up; create with deploy, delete, prepare, convert, lint, volumes, project actions
    let lintCalls = 0, deployFails = false;
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/containers']: { containers: [] },
        ['GET ' + B + '/hosts/local/stacks']: listingBody([stackFixture('shop', { operation: { id: 'op_x', kind: 'update', state: 'running' } })]),
        ['GET ' + B + '/operations/op_x']: () => ({ operation: opFixture({ id: 'op_x', kind: 'update', state: 'running' }) }),
        ['POST ' + B + '/hosts/local/stacks']: () => (deployFails
            ? { stack: detailFixture({ name: 'blog3' }), deploy_error: { code: 'host_not_prepared', message: 'prepare first' } }
            : { stack: detailFixture({ name: 'blog' }), operation: opFixture({ id: 'op_c1', stack: 'blog' }) }),
        ['DELETE ' + B + '/hosts/local/stacks/shop']: () => ({ operation: opFixture({ id: 'op_d1', kind: 'delete' }) }),
        ['POST ' + B + '/hosts/local/prepare']: () => ({ operation: opFixture({ id: 'op_p1', stack: '', kind: 'prepare' }) }),
        ['POST ' + B + '/convert/docker-run']: () => ({ compose: 'services:\n  web:\n    image: nginx\n', service: 'web', warnings: [{ code: 'flag_ignored', flag: '--rm', message: 'x' }] }),
        ['POST ' + B + '/hosts/local/lint']: () => {
            lintCalls += 1;
            if (lintCalls === 1) return new Promise(() => {});
            return { diagnostics: [{ line: 1, severity: 'hint', code: 'image_latest', message: 'x' }], preview: { services: [], named_volumes: [], variables: [] } };
        },
        ['GET ' + B + '/hosts/local/stacks/shop/volumes']: { volumes: [{ name: 'shop_db', driver: 'local', size: 1048576 }] },
        ['POST ' + B + '/hosts/local/projects/legacy/restart']: {},
        ['GET ' + B + '/hosts/local/helpers']: { helpers: { runner_image: 'docker:29.9.2-cli', prepared: true } },
        ['GET ' + B + '/operations']: { operations: [opFixture()] }, ['GET ' + B + '/operations/op_a1']: { operation: opFixture({ state: 'succeeded' }) }
    });
    FakeEventSource.all = [];
    const store = Store.create({ api, eventSource: url => new FakeEventSource(url) });
    await store.loadHosts();
    await store.loadStacks('local');
    await tick(); await tick();
    eq([store.state.ops.map(entry => entry.id), FakeEventSource.all.length], [['op_x'], 1], 'the operation the listing names joins the drawer');
    const created = await store.createStack('local', { name: 'blog', compose: 'services: {}\n', env: '', deploy: true });
    assert.deepStrictEqual(plain(api.calls.find(call => call.key === 'POST ' + B + '/hosts/local/stacks').body), { name: 'blog', compose: 'services: {}\n', deploy: true }, 'no .env is sent for an empty one');
    eq([created.stack.name, store.state.ops[0].id], ['blog', 'op_c1'], 'a deploying create follows its operation');
    deployFails = true;
    const opsBefore = store.state.ops.length;
    const refused = await store.createStack('local', { name: 'blog3', compose: 'x', deploy: true });
    eq([refused.stack.name, refused.deploy_error.code, refused.operation, store.state.ops.length], ['blog3', 'host_not_prepared', undefined, opsBefore], 'a deploy that could not start answers with deploy_error and no operation: the files are written and nothing is followed');
    deployFails = false;
    await store.createStack('local', { name: 'blog2', compose: 'x', env: 'A=1\n' });
    assert.deepStrictEqual(plain(api.calls.filter(call => call.key === 'POST ' + B + '/hosts/local/stacks').pop().body), { name: 'blog2', compose: 'x', env: 'A=1\n' });
    assert.equal((await store.deleteStack('local', 'shop', true)).ok, true);
    assert.equal(api.calls.find(call => call.method === 'DELETE').url, B + '/hosts/local/stacks/shop?volumes=1');
    assert.equal((await store.prepareHost('local')).entry.op.kind, 'prepare');
    const converted = await store.convertDockerRun('docker run nginx');
    eq([converted.service, converted.warnings[0].flag], ['web', '--rm']);
    assert.equal(api.calls.find(call => call.key === 'POST ' + B + '/convert/docker-run').body.command, 'docker run nginx');
    const stale = store.lint('local', { compose: 'a' }, 'editor');
    const fresh = await store.lint('local', { compose: 'b' }, 'editor');
    await assert.rejects(stale, error => error.dm.code === 'aborted');
    assert.equal(fresh.diagnostics[0].code, 'image_latest');
    eq(plain(await store.loadVolumes('local', 'shop')), [{ name: 'shop_db', driver: 'local', size: 1048576 }]);
    await store.projectAction('local', 'legacy', 'restart');
    assert.equal((await store.loadHelpers('local')).prepared, true);
    eq((await store.loadOperations({ host: 'local', stack: 'shop', active: true, limit: 5 })).length, 1);
    assert.equal(api.calls.filter(call => call.method === 'GET' && call.key === 'GET ' + B + '/operations').pop().url, B + '/operations?host=local&stack=shop&active=1&limit=5');
    assert.equal((await store.loadOperation('op_a1')).state, 'succeeded');
    store.dispose();
}

// ---- model: null entries in the container data --------------------------------------------------------------------------
assert.equal(model.portItems([null, undefined, { private: 80, public: 8080, type: 'tcp' }, 'x']).length, 1, 'a null port entry is skipped, not dereferenced');
assert.equal(model.buildRows([hostA], { local: { containers: [null, container('ok')] } }, 'all').length, 1, 'a null container is skipped');
assert.equal(model.filterRows(model.buildRows([hostA], { local: { containers: [container('web', { ports: [null, { private: 80, public: 8080 }] })] } }, 'all'), { query: '8080' }).length, 1, 'the search survives a null port entry');
eq(plain(model.totalCounts([hostA], { local: { containers: [null, container('a')] } })), { running: 1, total: 1 });

// ---- store: recovery and failed actions -----------------------------------------------------------------------------------
{
    // Docker switched on again: the poll asks for the host list and the app recovers without a manual refresh
    let docker = false;
    const api = fakeApi({
        ['GET ' + B + '/hosts']: () => { if (!docker) throw httpError(503, { code: 'docker_disabled', message: 'off' }); return { hosts: [hostA] }; },
        ['GET ' + B + '/hosts/local/containers']: { containers: [] }, ['GET ' + B + '/hosts/local/stacks']: listingBody([])
    });
    const store = Store.create({ api });
    store.setVisible(true);
    await tick(); await tick(); await tick();
    assert.equal(store.state.disabled, true);
    docker = true;
    assert.equal(store.pollOnce(), true, 'a disabled app keeps polling');
    await tick(); await tick(); await tick(); await tick();
    eq([store.state.disabled, store.state.hosts.length, store.state.byHost.local.loaded], [false, 1, true]);
    store.dispose();
}

{
    // a failed container action reloads the list at once
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/containers']: { containers: [container('web')] },
        ['POST ' + B + '/hosts/local/containers/abc/stop']: () => { throw httpError(500, { code: 'docker_error', message: 'boom' }); }
    });
    const store = Store.create({ api, section: 'containers' });
    await store.refresh();
    const before = api.count('GET ' + B + '/hosts/local/containers');
    const result = await store.act('local', 'abc', 'stop');
    await tick();
    eq([result.ok, result.error.code, api.count('GET ' + B + '/hosts/local/containers') - before], [false, 'docker_error', 1], 'the list is reloaded after a failed action, not 5 s later');
    store.dispose();
}

// ---- charts: pure SVG and text helpers -----------------------------------------------------------------------------------
{
    const charts = window.DockerManagerCharts;
    assert.ok(charts, 'the charts module publishes its global');
    eq([charts.bytes(0, 'en'), charts.bytes(1023, 'en'), charts.bytes(1536, 'en'), charts.bytes(10 * 1048576, 'en'), charts.bytes(2147483648, 'en')], ['0 B', '1023 B', '1.5 KB', '10 MB', '2.0 GB']);
    eq([charts.bytes(-1), charts.bytes(NaN), charts.bytes(undefined), charts.bytes('x')], ['–', '–', '–', '–'], 'unknown sizes are an en dash');
    eq([charts.percent(0.01, 'en'), charts.percent(3.24, 'en'), charts.percent(42.4, 'en'), charts.percent(250, 'en'), charts.percent(-1), charts.percent(null)], ['0%', '3.2%', '42%', '250%', '–', '–'], 'a percent keeps one decimal below 10, -1 is unknown');
    eq(charts.series([{ cpu: 1 }, { cpu: -1 }, null, { cpu: 3 }], 'cpu'), [1, null, null, 3]);
    assert.equal(charts.rate([{ t: 1000, net_rx: 100 }, { t: 3000, net_rx: 300 }], 'net_rx'), 100, 'bytes per second between the last two points');
    assert.equal(charts.rate([{ t: 1000, net_rx: 500 }, { t: 3000, net_rx: 20 }], 'net_rx'), 0, 'a counter reset is no negative rate');
    assert.equal(charts.rate([{ t: 1000, net_rx: 1 }], 'net_rx'), null, 'one point has no rate');

    const flat = charts.sparkline([1, 2, 3], { width: 60, height: 20, label: 'CPU' });
    assert.ok(flat.startsWith('<svg class="dm-spark"') && flat.includes('role="img"') && flat.includes('aria-label="CPU"') && flat.includes('class="dm-spark-line"') && flat.includes('class="dm-spark-area"'), 'a series draws a line and an area');
    assert.ok(charts.sparkline([1, 2]).includes('aria-hidden="true"') && !charts.sparkline([1, 2]).includes('role="img"'), 'without a label the sparkline is decorative');
    assert.ok(charts.sparkline([], {}).includes('is-empty') && charts.sparkline([null, -1], {}).includes('dm-spark-base'), 'no known value draws only the baseline');
    assert.equal((charts.sparkline([1, 2, null, 3, 4], {}).match(/M/g) || []).length, 2 + 2, 'a gap splits the line (and its area) in two');
    assert.ok(charts.sparkline([5], {}).includes('M') && !charts.sparkline([5], {}).includes('dm-spark-area'), 'a single point is a dot, not an area');
    assert.ok(charts.sparkline([1], { label: 'a"b<c' }).includes('aria-label="a&quot;b&lt;c"'), 'the label is escaped');
    // right-aligned in the window: the last point sits at the right edge, the first of three in a 60-slot window near the right
    const aligned = charts.sparkline([0, 10], { width: 100, height: 10, max: 10, window: 60 }).match(/class="dm-spark-line" d="M([\d.]+) /);
    assert.ok(aligned && Number(aligned[1]) > 95, 'a short series hugs the right edge of the window, got ' + (aligned && aligned[1]));
    // values above max are clamped to the top
    assert.ok(charts.sparkline([500], { height: 20, max: 100 }).includes(' 1.5'), 'a value above max is drawn at the top');

    const meter = charts.gauge(42, 100, { label: 'CPU', unit: 'percent', lang: 'en' });
    assert.ok(meter.includes('data-level="ok"') && meter.includes('role="meter"') && meter.includes('aria-valuenow="42"') && meter.includes('aria-valuetext="42%"') && meter.includes('width="42"'), 'a percent gauge fills to its share');
    assert.ok(charts.gauge(80, 100, { label: 'x', unit: 'percent' }).includes('data-level="warn"') && charts.gauge(95, 100, { label: 'x', unit: 'percent' }).includes('data-level="bad"'), 'warn from 75 percent, bad from 90');
    const memory = charts.gauge(1073741824, 4294967296, { label: 'RAM', unit: 'bytes', lang: 'en' });
    assert.ok(memory.includes('1.0 GB / 4.0 GB') && memory.includes('width="25"'), 'a bytes gauge shows used and total');
    const disk = charts.gauge(5368709120, 0, { label: 'Disk', unit: 'bytes', lang: 'en' });
    assert.ok(disk.includes('data-level="plain"') && !disk.includes('<svg') && !disk.includes('role="meter"') && disk.includes('5.0 GB'), 'without a ceiling there is no bar, only the value');
    assert.ok(charts.gauge(-1, 100, { label: 'CPU', unit: 'percent' }).includes('data-level="none"'), 'an unknown value has no level');
    assert.ok(charts.gauge(1, 100, { label: '<b>', unit: 'percent' }).includes('&lt;b&gt;') && !charts.gauge(1, 100, { label: '<b>', unit: 'percent' }).includes('<b>'), 'labels are escaped');
    assert.ok(charts.gauge(3, 10, { label: 'x', text: '3 of 10 shown' }).includes('3 of 10 shown'), 'text replaces the generated value text');

    assert.equal(charts.loadCell([]), '', 'no points, no cell');
    const cell = charts.loadCell([{ t: 1, cpu: 5, mem: 1048576 }, { t: 2, cpu: 12.4, mem: 2097152 }], { lang: 'en' });
    assert.ok(cell.includes('>12%<') && cell.includes('>2.0 MB<') && (cell.match(/<svg/g) || []).length === 2, 'the cell has CPU and memory, each with a sparkline');
    assert.ok(charts.loadCell([{ t: 1, cpu: -1, mem: 5 }], { lang: 'en' }).includes('>–<'), 'an unknown CPU shows a dash');
    const young = charts.loadCell([{ t: 1, cpu: 1, mem: 1 }, { t: 2, cpu: 2, mem: 2 }, { t: 3, cpu: 3, mem: 3 }], { lang: 'en' }).match(/class="dm-spark-line" d="M([\d.]+) /);
    assert.ok(young && Number(young[1]) < 45, 'a series of three points fills the sparkline instead of hugging the right edge, first x = ' + (young && young[1]));
    assert.ok(Number(charts.loadCell(Array.from({ length: 60 }, (_, i) => ({ t: i, cpu: 1, mem: 1 })), { lang: 'en' }).match(/class="dm-spark-line" d="M([\d.]+) /)[1]) < 3, 'a full ring spans the whole width');
}

// ---- live stream client: fake EventSource, fake api, fake timers ---------------------------------------------------------
function fakeTimers() {
    let clock = 0, seq = 0;
    const jobs = new Map();
    return {
        setTimeout: (fn, ms) => { const id = ++seq; jobs.set(id, { fn, at: clock + ms }); return id; },
        clearTimeout: id => { jobs.delete(id); },
        // Runs every timer due within `ms`, in order; a timer may arm the next one inside the window.
        advance(ms) {
            const end = clock + ms;
            for (;;) {
                const next = [...jobs].filter(([, job]) => job.at <= end).sort((a, b) => a[1].at - b[1].at)[0];
                if (!next) break;
                clock = next[1].at;
                jobs.delete(next[0]);
                next[1].fn();
            }
            clock = end;
        },
        now: () => clock,
        pending: () => jobs.size
    };
}
class FakeSource {
    constructor(url, list) { this.url = url; this.handlers = {}; this.closed = false; list.push(this); }
    addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
    close() { this.closed = true; }
    emit(type, data) { for (const fn of this.handlers[type] || []) fn({ data: typeof data === 'string' ? data : JSON.stringify(data) }); }
    fail() { this.onerror?.(new Error('boom')); }
}
function liveHarness(extra = {}) {
    const timers = fakeTimers(), sources = [], puts = [], log = [];
    let putResult = () => ({ status: 'ok', accepted: { hosts: 1, stats: 0, host_stats: 0 }, rejected: [] });
    const api = async (url, init) => {
        puts.push({ url, method: init.method, body: JSON.parse(init.body) });
        return putResult(init);
    };
    const live = window.DockerManagerLive.create({
        api, eventSource: url => new FakeSource(url, sources), now: timers.now, setTimeout: timers.setTimeout, clearTimeout: timers.clearTimeout,
        onConnection: (up, info) => log.push(['connection', up, !!(info && info.resumed)]), onStatus: (host, status) => log.push(['status', host, status.state]),
        onEvent: ev => log.push(['event', ev.action]), onResync: ev => log.push(['resync', ev.host, ev.reason]), onOperation: (host, op) => log.push(['operation', host, op.id]),
        onStats: (host, id, ring) => log.push(['stats', host, id, ring.length]), onHostStats: (host, data) => log.push(['host_stats', host, data.cpu]),
        ...extra
    });
    return { live, timers, sources, puts, log, setPut: fn => { putResult = fn; } };
}
const hello = (source, id = 's'.repeat(32)) => source.emit('hello', { stream_id: id, heartbeat_ms: 15000, limits: { stats_ids: 50, hosts: 20 } });
const point = (t, cpu = 1, mem = 100) => ({ t, cpu, mem, mem_limit: 1000, net_rx: 0, net_tx: 0, blk_r: 0, blk_w: 0, pids: 1 });

{
    // hello names the stream; a wish made before it is sent at once, one request
    const h = liveHarness();
    h.live.start();
    eq(h.sources.map(s => s.url), [B + '/stream']);
    h.live.setWant({ hosts: ['local'], stats: { local: ['abcdef0123456789zz'] }, hostStats: ['local'] });
    h.timers.advance(1000);
    assert.equal(h.puts.length, 0, 'nothing is sent before the stream has a name');
    hello(h.sources[0]);
    await tick();
    assert.equal(h.live.connected(), true);
    assert.equal(h.live.streamId(), 's'.repeat(32));
    eq(h.puts.map(p => [p.method, p.url, p.body]), [['PUT', B + '/stream/subscriptions', { stream_id: 's'.repeat(32), hosts: ['local'], stats: { local: ['abcdef012345'] }, host_stats: ['local'] }]], 'container ids are cut to the 12-character short id');
    eq(h.log.filter(entry => entry[0] === 'connection'), [['connection', true, false]]);
    // changes are collected for 250 ms and sent once; the same wish again sends nothing
    h.live.setWant({ hosts: ['local', 'nas'] });
    h.live.setWant({ hosts: ['local', 'nas'], stats: { nas: ['n1'] } });
    h.timers.advance(249);
    await tick();
    assert.equal(h.puts.length, 1, 'still debouncing');
    h.timers.advance(2);
    await tick();
    assert.equal(h.puts.length, 2);
    eq(h.puts[1].body, { stream_id: 's'.repeat(32), hosts: ['local', 'nas'], stats: { nas: ['n1'] }, host_stats: [] });
    h.live.setWant({ hosts: ['nas', 'local'], stats: { nas: ['n1'] } });
    h.timers.advance(1000);
    await tick();
    assert.equal(h.puts.length, 2, 'the same wish in another order is no change');
    h.live.stop();
    assert.equal(h.sources[0].closed, true);
    assert.equal(h.timers.pending(), 0, 'stop clears every timer');
}

{
    // limits: 50 containers per host and 20 hosts; duplicates and blanks go; the rest keep their order
    const h = liveHarness();
    h.live.start();
    hello(h.sources[0]);
    const ids = Array.from({ length: 60 }, (_, i) => 'c' + String(i).padStart(11, '0'));
    const hosts = Array.from({ length: 25 }, (_, i) => 'h' + i);
    h.live.setWant({ hosts: hosts.concat(['h1', '']), stats: { h0: ids.concat(ids[0]), empty: [] }, hostStats: ['h0', 'h0'] });
    h.timers.advance(300);
    await tick();
    eq([h.puts[0].body.hosts.length, h.puts[0].body.stats.h0.length, h.puts[0].body.stats.h0[0], Object.keys(h.puts[0].body.stats), h.puts[0].body.host_stats], [20, 50, ids[0], ['h0'], ['h0']]);
    h.live.stop();
}

{
    // messages: status, event, resync and operation go to the callbacks; the status of a host is kept
    const h = liveHarness();
    h.live.start();
    hello(h.sources[0]);
    const s = h.sources[0];
    s.emit('host_status', { host: 'local', state: 'live', reason: '', t: 5 });
    s.emit('event', { host: 'local', type: 'container', action: 'die', id: 'x'.repeat(64), short_id: 'x'.repeat(12), name: 'web-1', project: '', service: '', t: 6 });
    s.emit('resync', { host: 'local', reason: 'reconnect' });
    s.emit('operation', { host: 'local', operation: { id: 'op_1', state: 'running' } });
    s.emit('host_stats', { host: 'local', t: 9, cpu: 12.5, ncpu: 8, mem_used: 1, mem_total: 2, running: 3, sampled: 3, partial: false });
    s.emit('event', '{not json');
    eq(h.log.slice(1), [['status', 'local', 'live'], ['event', 'die'], ['resync', 'local', 'reconnect'], ['operation', 'local', 'op_1'], ['host_stats', 'local', 12.5]]);
    eq([h.live.status('local').state, h.live.status('nas'), h.live.hostStats('local').cpu, h.live.hostStats('nas')], ['live', null, 12.5, null]);
    h.live.stop();
}

{
    // stats: a ring of 60 points per wanted container; old or repeated points are ignored; history replaces; gone clears
    const h = liveHarness();
    h.live.start();
    hello(h.sources[0]);
    const s = h.sources[0];
    h.live.setWant({ hosts: ['local'], stats: { local: ['aaaaaaaaaaaa'] } });
    s.emit('stats', { host: 'local', id: 'bbbbbbbbbbbb', point: point(1) });
    assert.equal(h.live.ring('local', 'bbbbbbbbbbbb').length, 0, 'a container nobody asked for is ignored');
    for (let t = 1; t <= 70; t += 1) s.emit('stats', { host: 'local', id: 'aaaaaaaaaaaa', point: point(t) });
    s.emit('stats', { host: 'local', id: 'aaaaaaaaaaaa', point: point(70) });
    s.emit('stats', { host: 'local', id: 'aaaaaaaaaaaa', point: point(3) });
    const ring = h.live.ring('local', 'aaaaaaaaaaaa');
    eq([ring.length, ring[0].t, ring[59].t], [60, 11, 70], 'sixty points, the oldest dropped, a repeated or older point ignored');
    s.emit('stats_history', { host: 'local', id: 'aaaaaaaaaaaa', points: [point(100), point(101), point(102)] });
    eq(h.live.ring('local', 'aaaaaaaaaaaa').map(p => p.t), [100, 101, 102], 'the history replaces the ring');
    s.emit('stats_gone', { host: 'local', id: 'aaaaaaaaaaaa' });
    assert.equal(h.live.ring('local', 'aaaaaaaaaaaa').length, 0);
    eq(h.log.filter(entry => entry[0] === 'stats').slice(-1), [['stats', 'local', 'aaaaaaaaaaaa', 0]], 'gone is reported as an empty ring');
    // dropping a container from the wish frees its ring
    s.emit('stats_history', { host: 'local', id: 'aaaaaaaaaaaa', points: [point(1)] });
    h.live.setWant({ hosts: ['local'] });
    assert.equal(h.live.ring('local', 'aaaaaaaaaaaa').length, 0);
    h.live.stop();
}

{
    // a broken stream is closed and opened again after 1, 2, 4 s; the wish is sent again on the new stream; hello resets the delay
    const h = liveHarness();
    h.live.start();
    hello(h.sources[0], 'a'.repeat(32));
    h.live.setWant({ hosts: ['local'] });
    h.timers.advance(300);
    await tick();
    h.sources[0].emit('host_status', { host: 'local', state: 'live' });
    h.sources[0].fail();
    assert.equal(h.sources[0].closed, true, 'the browser retry is not used');
    eq([h.live.connected(), h.live.failed(), h.live.status('local'), h.sources.length], [false, true, null, 1]);
    h.timers.advance(999);
    assert.equal(h.sources.length, 1);
    h.timers.advance(2);
    assert.equal(h.sources.length, 2, 'first retry after 1 s');
    h.sources[1].fail();
    h.timers.advance(1999);
    assert.equal(h.sources.length, 2);
    h.timers.advance(2);
    assert.equal(h.sources.length, 3, 'second retry after 2 s');
    hello(h.sources[2], 'b'.repeat(32));
    await tick();
    eq(h.puts.map(p => p.body.stream_id), ['a'.repeat(32), 'b'.repeat(32)], 'the wish is sent again to the new stream');
    eq(h.log.filter(entry => entry[0] === 'connection'), [['connection', true, false], ['connection', false, false], ['connection', true, true]], 'one report per change; the second hello is a resume');
    h.sources[2].fail();
    h.timers.advance(1001);
    assert.equal(h.sources.length, 4, 'after a hello the delay starts at 1 s again');
    h.live.stop();
}

{
    // the backoff stops at 30 s
    const h = liveHarness();
    h.live.start();
    for (let i = 0; i < 9; i += 1) { h.sources[h.sources.length - 1].fail(); h.timers.advance(31000); }
    assert.equal(h.sources.length, 10);
    h.sources[9].fail();
    h.timers.advance(29999);
    assert.equal(h.sources.length, 10);
    h.timers.advance(2);
    assert.equal(h.sources.length, 11, 'never longer than 30 s');
    h.live.stop();
}

{
    // unknown stream (404 stream_not_found) and a dropped slow client both reconnect; rejected hosts are kept
    const h = liveHarness();
    h.live.start();
    hello(h.sources[0]);
    h.setPut(() => { throw Object.assign(new Error('gone'), { status: 404, body: { code: 'stream_not_found' } }); });
    h.live.setWant({ hosts: ['local'] });
    h.timers.advance(300);
    await tick();
    assert.equal(h.sources[0].closed, true, 'an unknown stream is replaced');
    h.timers.advance(1001);
    assert.equal(h.sources.length, 2);
    h.setPut(() => ({ status: 'ok', accepted: { hosts: 1, stats: 0, host_stats: 0 }, rejected: [{ host: 'ghost', reason: 'unknown_host' }] }));
    hello(h.sources[1]);
    await tick();
    eq(h.live.rejected(), [{ host: 'ghost', reason: 'unknown_host' }]);
    h.sources[1].emit('dropped', { reason: 'slow_client' });
    eq([h.live.connected(), h.live.failed(), h.sources[1].closed], [false, true, true]);
    h.live.stop();
}

{
    // another request error is tried again a few times, then left alone until the wish changes
    const h = liveHarness();
    h.live.start();
    hello(h.sources[0]);
    let calls = 0;
    h.setPut(() => { calls += 1; throw Object.assign(new Error('x'), { status: 500, body: { code: 'internal_error' } }); });
    h.live.setWant({ hosts: ['local'] });
    for (let i = 0; i < 10; i += 1) { h.timers.advance(3500); await tick(); }
    assert.equal(calls, 4, 'one request and three retries');
    assert.equal(h.live.connected(), true, 'a failed subscription does not close the stream');
    h.live.stop();
}

{
    // watchdog: EventSource shows no heartbeat comments, so a silent stream is asked whether the server still has it
    const h = liveHarness();
    h.live.start();
    hello(h.sources[0]);
    await tick();
    h.timers.advance(44000);
    await tick();
    eq([h.puts.length, h.live.connected()], [0, true], 'nothing before three heartbeats (45 s) have passed');
    h.sources[0].emit('event', { host: 'local', type: 'container', action: 'start' });
    h.timers.advance(44000);
    await tick();
    eq([h.puts.length, h.live.connected()], [0, true], 'every message starts the window again');
    h.timers.advance(1001);
    await tick();
    eq([h.puts.length, h.puts[0].body.stream_id, h.live.connected()], [1, 's'.repeat(32), true], 'a silent stream is asked whether the server still knows it, and an answer keeps it');
    h.timers.advance(45001);
    await tick();
    assert.equal(h.puts.length, 2, 'and the next window starts');
    // the server lost the stream: the probe is answered 404 and the stream is reopened with the usual backoff
    h.setPut(() => { throw Object.assign(new Error('gone'), { status: 404, body: { code: 'stream_not_found' } }); });
    h.timers.advance(45001);
    await tick();
    eq([h.live.connected(), h.live.failed()], [false, true]);
    h.timers.advance(1001);
    assert.equal(h.sources.length, 2);
    // an answer that never comes counts as lost after 10 s
    h.setPut(init => new Promise((resolve, reject) => init.signal.addEventListener('abort', () => reject(Object.assign(new Error('aborted'), { name: 'AbortError' })))));
    hello(h.sources[1], 'c'.repeat(32));
    await tick();
    h.timers.advance(45001);
    await tick();
    assert.equal(h.live.connected(), true, 'still waiting for the answer');
    h.timers.advance(10001);
    await tick();
    eq([h.live.connected(), h.live.failed()], [false, true], 'no answer within 10 s');
    h.live.stop();
    assert.equal(h.timers.pending(), 0, 'stop clears the watchdog');
    // the window follows the heartbeat of the server: three times, at least 45 s
    const slow = liveHarness();
    slow.live.start();
    slow.sources[0].emit('hello', { stream_id: 'd'.repeat(32), heartbeat_ms: 30000, limits: {} });
    await tick();
    slow.timers.advance(89000);
    await tick();
    assert.equal(slow.puts.length, 0);
    slow.timers.advance(1001);
    await tick();
    assert.equal(slow.puts.length, 1);
    slow.live.stop();
}

{
    // an empty wish is not sent at all, and neither is a stop that never started
    const h = liveHarness();
    h.live.stop();
    h.live.start();
    h.live.start();
    assert.equal(h.sources.length, 1, 'start twice opens one stream');
    hello(h.sources[0]);
    h.live.setWant({});
    h.timers.advance(1000);
    await tick();
    assert.equal(h.puts.length, 0);
    h.live.stop();
    h.sources[0].emit('host_status', { host: 'local', state: 'live' });
    assert.equal(h.live.status('local'), null, 'a stopped client ignores late messages');
}

// ---- model and stack model: terminal state, tabs, palette commands for logs and terminals ---------------------------------
{
    const web = container('web'), stopped = container('old', { state: 'exited', status: 'Exited (0) 1 hour ago' }), helper = container('aurago-dm-op-1', { aurago_helper: 'runner', protected_owner: 'docker_manager' });
    eq([model.terminalState(web), model.terminalState(stopped), model.terminalState(helper), model.terminalState(null)], ['ok', 'not_running', 'helper', 'not_running'], 'a shell needs a running container that is no helper');
    eq(sm.STACK_TABS, ['overview', 'compose', 'env', 'logs', 'terminal']);
    eq(sm.PROJECT_TABS, ['overview', 'logs']);
    eq([model.reasonCode('host_not_found'), model.reasonCode('weird'), model.REASONS.includes('host_not_found')], ['host_not_found', 'unknown', true], 'a host deleted while it was watched has its own reason');
    const hs = [{ id: 'local', name: 'Local' }];
    const stackRows = [{ key: 'local/shop', hostId: 'local', hostName: 'Local', stack: { name: 'shop', status: 'running' } }];
    const containers = model.buildRows(hs, { local: { containers: [web, stopped, helper] } }, 'all');
    const label = (action, params) => action + ':' + (params.name || params.stack || params.host || '');
    const data = { hosts: hs, stackRows, containers, logs: true, canTerminal: true, canMutate: true, multi: false, hostStates: { local: 'ok' }, scope: 'local', label };
    const items = sm.paletteItems(data);
    eq(items.map(item => item.id), [
        'open:local/shop', 'logs:local/shop', 'deploy:local/shop', 'restart:local/shop', 'stop:local/shop', 'new:local', 'refresh',
        'clogs:local/' + 'web'.padEnd(64, '0'), 'term:local/' + 'web'.padEnd(64, '0'), 'clogs:local/' + 'old'.padEnd(64, '0'), 'clogs:local/' + 'aurago-dm-op-1'.padEnd(64, '0')
    ], 'Logs for every stack and container, Terminal only for running containers that are no helpers');
    eq(items.slice(7).map(item => [item.action, item.label, item.hostId]), [['logs_container', 'logs:web', 'local'], ['terminal', 'terminal:web', 'local'], ['logs_container', 'logs:old', 'local'], ['logs_container', 'logs:aurago-dm-op-1', 'local']]);
    eq(items[1].label, 'logs:shop');
    eq(sm.paletteItems(Object.assign({}, data, { canMutate: false })).filter(item => item.action === 'terminal'), [], 'a read-only window gets no Terminal command');
    eq(sm.paletteItems(Object.assign({}, data, { containers: undefined, logs: false })).map(item => item.action), ['open', 'update', 'restart', 'stop', 'new_stack', 'refresh'], 'without the new data the palette is what it was');
    eq(sm.paletteItems(Object.assign({}, data, { multi: true })).filter(item => item.action === 'logs_container')[0].label, 'logs:web (Local)', 'several hosts: the host is part of the name');
}

// ---- store: live stream -----------------------------------------------------------------------------------------------------
{
    const sources = [], timers = fakeTimers();
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA, hostB] },
        ['GET ' + B + '/hosts/local/containers']: { containers: [container('web')] },
        ['GET ' + B + '/hosts/h_nas/containers']: { containers: [container('media')] },
        ['GET ' + B + '/hosts/local/stacks']: listingBody([stackFixture('shop')]),
        ['GET ' + B + '/hosts/h_nas/stacks']: listingBody([]),
        ['GET ' + B + '/hosts/local/stacks/shop']: { stack: detailFixture() },
        ['GET ' + B + '/operations']: { operations: [] },
        ['GET ' + B + '/operations/op_a1/stream']: {},
        ['PUT ' + B + '/stream/subscriptions']: { status: 'ok', accepted: { hosts: 1, stats: 0, host_stats: 0 }, rejected: [] }
    });
    const store = Store.create({ api, section: 'stacks', eventSource: url => new FakeSource(url, sources), setTimeout: timers.setTimeout, clearTimeout: timers.clearTimeout, reloadMs: 300 });
    const n = key => api.count(key), C = 'GET ' + B + '/hosts/local/containers', S = 'GET ' + B + '/hosts/local/stacks', H = 'GET ' + B + '/hosts';
    assert.equal(store.liveStatus('local'), 'off', 'no stream before the window is visible');
    store.setVisible(true);
    for (let i = 0; i < 8; i += 1) await tick();
    eq(sources.map(s => s.url), [B + '/stream'], 'a visible window opens the stream');
    hello(sources[0]);
    sources[0].emit('host_status', { host: 'local', state: 'live', reason: '', t: 1 });
    sources[0].emit('host_status', { host: 'h_nas', state: 'connecting', reason: '', t: 1 });
    eq([store.liveStatus('local'), store.liveStatus('h_nas'), store.state.live.connected], ['live', 'connecting', true]);
    // the hosts in scope are subscribed (events and status); no gauges while all hosts are shown
    timers.advance(300);
    await tick();
    eq(api.calls.filter(call => call.method === 'PUT').pop().body, { stream_id: 's'.repeat(32), hosts: ['local', 'h_nas'], stats: {}, host_stats: [] });

    // a live host is read again only after about a minute; any other host on every tick
    const containersBefore = n(C), nasBefore = n('GET ' + B + '/hosts/h_nas/containers');
    store.pollOnce();
    await tick(); await tick();
    eq([n(C) - containersBefore, n('GET ' + B + '/hosts/h_nas/containers') - nasBefore], [0, 1], 'a live host is skipped by the 5 s poll, a connecting host is not');
    store.state.byHost.local.at = Date.now() - 58000;
    store.state.stacks.local.at = Date.now() - 58000;
    store.pollOnce();
    await tick(); await tick();
    eq([n(C) - containersBefore, n(S) > 1], [1, true], 'the safety poll of a live host runs once its data is a minute old');

    // events: one reload per host and 300 ms however many events come; other types and other hosts are ignored
    const reloadBase = n(C), stacksBase = n(S);
    const die = (extra = {}) => ({ host: 'local', type: 'container', action: 'die', id: 'x'.repeat(64), short_id: 'x'.repeat(12), name: 'web-1', project: '', service: '', t: 5, ...extra });
    sources[0].emit('event', die()); sources[0].emit('event', die({ action: 'start' })); sources[0].emit('event', die({ action: 'health_status', health: 'healthy' }));
    sources[0].emit('event', die({ type: 'image', action: 'pull' }));
    sources[0].emit('event', die({ host: 'elsewhere' }));
    timers.advance(299);
    await tick();
    assert.equal(n(C), reloadBase, 'nothing before the delay is over');
    timers.advance(2);
    await tick(); await tick();
    eq([n(C) - reloadBase, n(S) - stacksBase], [1, 1], 'three container events make one reload of the containers and one of the stacks');
    store.setSection('containers');
    await tick(); await tick();
    const switchedC = n(C), stacksFixed = n(S);
    sources[0].emit('event', die());
    timers.advance(400);
    await tick(); await tick();
    eq([n(C) - switchedC, n(S) - stacksFixed], [1, 0], 'the Containers section does not reload the stacks');
    store.setSection('stacks');
    await tick(); await tick();

    // an event of the open stack's project reloads that stack; a resync reloads it whatever the project
    store.openStack('local', 'shop', 'overview');
    await tick(); await tick();
    const stackBase = n('GET ' + B + '/hosts/local/stacks/shop');
    sources[0].emit('event', die({ project: 'other' }));
    timers.advance(400);
    await tick(); await tick();
    assert.equal(n('GET ' + B + '/hosts/local/stacks/shop') - stackBase, 0, 'an event of another project leaves the open stack alone');
    sources[0].emit('event', die({ project: 'shop' }));
    timers.advance(400);
    await tick(); await tick();
    assert.equal(n('GET ' + B + '/hosts/local/stacks/shop') - stackBase, 1);
    sources[0].emit('resync', { host: 'local', reason: 'reconnect' });
    timers.advance(400);
    await tick(); await tick();
    assert.equal(n('GET ' + B + '/hosts/local/stacks/shop') - stackBase, 2, 'a resync reloads the open stack too');
    store.closeStack();

    // operation messages join the drawer and are followed through their own stream
    sources[0].emit('operation', { host: 'local', operation: opFixture({ state: 'running' }) });
    eq([store.state.ops.length, store.state.ops[0].op.state, sources.filter(s => s.url.endsWith('/operations/op_a1/stream')).length], [1, 'running', 1]);

    // stats and host stats reach their own listeners and never repaint the app
    let repaints = 0;
    store.subscribe(() => { repaints += 1; });
    const seen = [];
    const off1 = store.onStats((host, id, points) => seen.push(['stats', host, id, points.length]));
    const off2 = store.onHostStats((host, data) => seen.push(['host', host, data.cpu]));
    store.setLiveStats({ local: ['abcdef012345ffff'], elsewhere: ['zzz'] });
    timers.advance(300);
    await tick();
    eq(api.calls.filter(call => call.method === 'PUT').pop().body.stats, { local: ['abcdef012345'] }, 'only containers of hosts in scope are subscribed, cut to the short id');
    repaints = 0;
    sources[0].emit('stats', { host: 'local', id: 'abcdef012345', point: point(1, 4) });
    sources[0].emit('host_stats', { host: 'local', t: 1, cpu: 7, ncpu: 2, mem_used: 1, mem_total: 2, running: 1, sampled: 1, partial: false });
    eq(seen, [['stats', 'local', 'abcdef012345', 1], ['host', 'local', 7]]);
    assert.equal(repaints, 0, 'stats do not repaint the app');
    eq([store.liveRing('local', 'abcdef012345').length, store.liveHostStats('local').cpu], [1, 7]);
    off1(); off2();
    sources[0].emit('stats', { host: 'local', id: 'abcdef012345', point: point(2, 5) });
    assert.equal(seen.length, 2, 'a removed listener hears nothing');

    // gauges are asked for under a single host
    store.setScope('local');
    timers.advance(300);
    await tick();
    eq(api.calls.filter(call => call.method === 'PUT').pop().body.host_stats, ['local']);
    store.setScope('all');
    await tick(); await tick();

    // paused host: the 5 s poll is back and the status says so
    sources[0].emit('host_status', { host: 'local', state: 'paused', reason: '', t: 9 });
    assert.equal(store.liveStatus('local'), 'paused');
    const pausedBase = n(C);
    store.pollOnce();
    await tick(); await tick();
    assert.equal(n(C) - pausedBase, 1, 'a paused host is polled on every tick again');
    sources[0].emit('host_status', { host: 'local', state: 'live', reason: '', t: 10 });

    // the stream breaks: every host is paused, and the reconnect reads everything once
    sources[0].fail();
    eq([store.state.live.failed, store.liveStatus('local'), store.liveStatus('h_nas')], [true, 'paused', 'paused']);
    const downBase = n(C);
    timers.advance(1001);
    assert.equal(sources.filter(s => s.url === B + '/stream').length, 2);
    hello(sources[sources.length - 1], 'r'.repeat(32));
    await tick(); await tick();
    eq([store.state.live.failed, store.state.live.connected, n(C) - downBase >= 1], [false, true, true], 'the reconnect reloads what may have changed meanwhile');

    // hiding the window ends the stream
    store.setVisible(false);
    assert.equal(sources[sources.length - 1].closed, true);
    eq([store.liveStatus('local'), store.state.live.connected], ['off', false]);
    store.dispose();
}

{
    // no live client: without EventSource nothing is opened and the 5 s poll stays what it was
    const api = fakeApi({ ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/containers']: { containers: [] }, ['GET ' + B + '/hosts/local/stacks']: listingBody([]), ['GET ' + B + '/operations']: { operations: [] } });
    const store = Store.create({ api, eventSource: null, live: false });
    store.setVisible(true);
    for (let i = 0; i < 6; i += 1) await tick();
    assert.equal(store.liveStatus('local'), 'off');
    const before = api.count('GET ' + B + '/hosts/local/containers');
    store.pollOnce();
    await tick(); await tick();
    assert.equal(api.count('GET ' + B + '/hosts/local/containers') - before, 1);
    store.setLiveStats({ local: ['a'] });
    assert.deepEqual(plain(store.liveRing('local', 'a')), []);
    store.dispose();
}

// ---- store: disk usage, external project, terminal diagnosis --------------------------------------------------------------
{
    let mode = 'ok';
    const df = { images: { count: 3, active: 2, size: 1000, reclaimable: 100 }, containers: { count: 2, active: 1, size: 10, reclaimable: 0 }, volumes: { count: 1, active: 1, size: 500, reclaimable: 0 }, build_cache: { count: 0, active: 0, size: 0, reclaimable: 0 }, total: 1510 };
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA, hostB] },
        ['GET ' + B + '/hosts/local/containers']: { containers: [] }, ['GET ' + B + '/hosts/h_nas/containers']: { containers: [] },
        ['GET ' + B + '/hosts/local/stacks']: listingBody([]), ['GET ' + B + '/hosts/h_nas/stacks']: listingBody([]), ['GET ' + B + '/operations']: { operations: [] },
        ['GET ' + B + '/hosts/local/df']: () => { if (mode === 'fail') throw httpError(502, { code: 'host_unreachable', reason: 'timeout', message: 'slow' }); return { status: 'ok', df, fetched_at: '2026-10-28T10:00:00Z', cached: mode === 'cached' }; },
        ['GET ' + B + '/hosts/local/containers/ruview/terminal']: () => { throw httpError(409, { code: 'container_protected_confirmation_required', owner: 'ruview', message: 'confirm' }); },
        ['GET ' + B + '/hosts/local/containers/ro/terminal']: () => { throw httpError(403, { code: 'read_only', message: 'read only' }); },
        ['GET ' + B + '/hosts/local/containers/odd/terminal']: {}
    });
    const store = Store.create({ api, scope: 'local', section: 'containers', live: false });
    await store.refresh();
    const D = 'GET ' + B + '/hosts/local/df';
    eq([api.count(D), store.state.df.local.status, store.state.df.local.df.total, store.state.df.local.cached], [1, 'ok', 1510, false], 'a single host scope reads its disk usage once');
    await store.refresh();
    assert.equal(api.count(D), 1, 'a fresh reading is not read again');
    mode = 'cached';
    await store.loadDf('local', { refresh: true });
    eq([api.calls.filter(call => call.key === D).pop().url, store.state.df.local.cached], [B + '/hosts/local/df?refresh=1', true], 'the gauge refresh asks the server to read again');
    mode = 'fail';
    await store.loadDf('local', { refresh: true });
    eq([store.state.df.local.status, store.state.df.local.df.total, store.state.df.local.error.code, store.state.df.local.busy], ['ok', 1510, 'host_unreachable', false], 'a failed reading keeps the last numbers and records the error');
    store.setScope('all');
    const allBase = api.count(D);
    await store.refresh();
    assert.equal(api.count(D), allBase, 'all hosts have no gauge and read no disk usage');
    store.dispose();

    // the external project pane and the stack pane exclude each other; tabs are checked
    const store2 = Store.create({ api, live: false, scope: 'all' });
    await store2.refresh();
    store2.openProject('local', 'legacy', 'logs');
    eq([store2.state.project.key, store2.state.project.tab], ['x:local/legacy', 'logs']);
    store2.setProjectTab('compose');
    assert.equal(store2.state.project.tab, 'logs', 'an unknown tab is ignored');
    store2.openStack('local', 'shop', 'overview');
    eq([store2.state.project, store2.state.stack.key], [null, 'local/shop'], 'opening a stack closes the project pane');
    store2.openProject('local', 'legacy');
    eq([store2.state.stack, store2.state.project.tab], [null, 'overview'], 'opening a project closes the stack pane');
    store2.closeProject();
    assert.equal(store2.state.project, null);
    store2.dispose();

    // a refused terminal handshake is explained by a plain GET
    const store3 = Store.create({ api, live: false });
    const protectedInfo = await store3.diagnoseTerminal('local', 'ruview');
    eq([protectedInfo.code, protectedInfo.data.owner], ['container_protected_confirmation_required', 'ruview']);
    const readonlyInfo = await store3.diagnoseTerminal('local', 'ro');
    eq([readonlyInfo.code, store3.state.dockerReadonly], ['read_only', true], 'a read_only answer switches the app to read-only');
    assert.equal(await store3.diagnoseTerminal('local', 'odd'), null, 'an answer without an error is no diagnosis');
    store3.dispose();
}

// ---- logs: the model of the follow panel ------------------------------------------------------------------------------------
{
    const Logs = window.DockerManagerLogs;
    assert.ok(Logs && typeof Logs.create === 'function', 'the logs module publishes its global');
    const ESC = String.fromCharCode(27);
    eq([Logs.cleanText(ESC + '[31mred' + ESC + '[0m text'), Logs.cleanText('50%\r75%\r100%'), Logs.cleanText('line\r'), Logs.cleanText(null)], ['red text', '100%', 'line', ''], 'colour codes go, a progress bar keeps its last state');
    const stamp = (n, extra = {}) => ({ t: '2026-10-28T07:33:' + String(n).padStart(2, '0') + '.123456789Z', s: 'out', text: 'line ' + n, trunc: false, ...extra });

    {
        const m = Logs.createModel({ max: 5 });
        const first = m.append([stamp(1), stamp(2, { s: 'err' }), stamp(3)]);
        eq([first.added.length, first.shown, first.dropped.length, m.count()], [3, 3, 0, 3]);
        assert.equal(m.all()[1].s, 'err', 'the stderr flag is kept');
        const second = m.append([stamp(4), stamp(5), stamp(6), stamp(7)]);
        eq([second.dropped.map(e => e.text), m.all().map(e => e.text)], [['line 1', 'line 2'], ['line 3', 'line 4', 'line 5', 'line 6', 'line 7']], 'the ring keeps the newest lines and reports what fell off');
        assert.equal(m.lastTime(), stamp(7).t, 'since is the time of the newest line');
        m.setQuery('  LINE 5 ');
        eq(m.visible().map(e => e.text), ['line 5'], 'the filter ignores case and surrounding blanks');
        eq([m.append([stamp(8)]).shown, m.visible().length], [0, 1], 'a new line that does not match the filter is not shown');
        m.setQuery('');
        m.clear();
        eq([m.count(), m.lastTime()], [0, stamp(8).t], 'clear empties the screen but remembers where the stream stood');
        m.reset();
        eq([m.count(), m.lastTime()], [0, ''], 'reset forgets the resume point too');
        assert.equal(Logs.createModel().max, 5000, 'the ring holds 5000 lines');
    }
    {
        // services: the filter, the names seen so far, a text filter that also matches the service
        const m = Logs.createModel();
        m.append([stamp(1, { svc: 'web', ctr: 'aaaaaaaaaaaa' }), stamp(2, { svc: 'db', ctr: 'bbbbbbbbbbbb', text: 'ready' }), stamp(3, { svc: 'web', ctr: 'aaaaaaaaaaaa' })]);
        eq(m.services(), ['web', 'db']);
        assert.equal(m.toggleService('db'), false);
        eq(m.visible().map(e => e.svc), ['web', 'web'], 'a hidden service is not shown');
        assert.equal(m.serviceShown('db'), false);
        assert.equal(m.toggleService('db'), true);
        m.setQuery('db');
        eq(m.visible().map(e => e.text), ['ready'], 'the filter matches the service name');
        m.setQuery('');
        eq(m.text().split('\n'), ['[web] line 1', '[db] ready', '[web] line 3'], 'copy gives the visible lines with the service');
        assert.ok(m.text({ timestamps: true }).startsWith(stamp(1).t + ' [web] line 1'), 'and the timestamps when they are on');
        m.addService('cache');
        eq(m.services(), ['web', 'db', 'cache']);
    }
    {
        // follow: pausing counts the lines that arrive; following again clears the counter
        const m = Logs.createModel();
        assert.equal(m.isFollowing(), true);
        m.setFollow(false);
        m.addUnseen(3);
        m.addUnseen(2);
        eq([m.isFollowing(), m.unseen()], [false, 5]);
        m.setFollow(true);
        eq([m.isFollowing(), m.unseen()], [true, 0]);
    }
    {
        // a reconnect repeats the newest second: lines already seen are skipped, new ones are kept, and the filter ends with the first new line
        const m = Logs.createModel();
        m.append([stamp(1), stamp(2), stamp(2, { text: 'second at the same time' })]);
        m.beginResume();
        const r = m.append([stamp(1), stamp(2), stamp(2, { text: 'second at the same time' }), stamp(2, { text: 'new at the same time' }), stamp(3)]);
        eq(r.added.map(e => e.text), ['new at the same time', 'line 3'], 'older lines and exact repeats are skipped');
        eq(m.all().length, 5);
        m.beginResume();
        eq(m.append([stamp(3, { text: 'late' })]).added.length, 1, 'a line of the same second that was not seen before is kept');
        // lines without a parseable time cannot be told apart: nothing to resume from
        const plain = Logs.createModel();
        plain.append([{ t: '', text: 'x' }]);
        plain.beginResume();
        eq([plain.lastTime(), plain.append([{ t: '', text: 'x' }]).added.length], ['', 1]);
    }
    {
        // notices: every not_followed notice of the server is one line with all the services, each once; attached and detached name the service
        let list = [];
        list = Logs.mergeNotice(list, { code: 'not_followed', message: 'history of db failed', container: 'abcdef012345' });
        list = Logs.mergeNotice(list, { code: 'attached', message: 'started', services: ['web'], container: '111111111111' });
        list = Logs.mergeNotice(list, { code: 'not_followed', message: 'no free slot', services: ['cache', 'worker'] });
        list = Logs.mergeNotice(list, { code: 'not_followed', message: 'again', services: ['cache'], container: 'abcdef012345' });
        eq(list.map(item => item.code), ['not_followed', 'attached'], 'all not_followed notices are one entry, at the place of the first');
        eq(list[0], { code: 'not_followed', message: 'history of db failed', services: ['abcdef012345', 'cache', 'worker'] });
        for (const code of ['detached', 'attached', 'truncated_history', 'detached']) list = Logs.mergeNotice(list, { code, message: code });
        eq(list.length, 4, 'at most four notices are kept');
        eq([Logs.noticeName({ services: ['web'], container: 'abc' }), Logs.noticeName({ container: 'abc' }), Logs.noticeName({})], ['web', 'abc', ''], 'the service first, else the container');
        // a failed stream request ends the retries for any 4xx answer and for the codes that mean "not now"; a 5xx, a timeout and the network are tried again
        eq([{ code: 'too_many_streams', status: 429 }, { code: 'weird', status: 404 }, { code: 'bad_request', status: 400 }, { code: 'docker_disabled', status: 503 }, { code: 'docker_hosts_unavailable', status: 503 },
            { code: 'docker_error', status: 502 }, { code: 'timeout', status: 0 }, { code: 'network', status: 0 }, null].map(Logs.isFinalDiagnosis), [true, true, true, true, true, false, false, false, false]);
    }
    {
        eq(Logs.followUrl({ kind: 'container', hostId: 'h 1', ref: 'abc' }, { tail: 500 }), '/api/docker-manager/hosts/h%201/containers/abc/logs?follow=1&tail=500');
        eq(Logs.followUrl({ kind: 'container', hostId: 'local', ref: 'abc' }, {}), '/api/docker-manager/hosts/local/containers/abc/logs?follow=1&tail=200', 'a container starts with 200 lines');
        eq(Logs.followUrl({ kind: 'stack', hostId: 'local', name: 'shop' }, { tail: 7 }), '/api/docker-manager/hosts/local/stacks/shop/logs?follow=1&tail=100', 'a stack starts with 100 lines per container, an unsupported tail falls back');
        eq(Logs.followUrl({ kind: 'project', hostId: 'local', name: 'legacy' }, { tail: 1000 }), '/api/docker-manager/hosts/local/projects/legacy/logs?follow=1&tail=1000');
        eq(Logs.followUrl({ kind: 'container', hostId: 'local', ref: 'abc' }, { since: '2026-10-28T07:33:01.5Z', tail: 100 }), '/api/docker-manager/hosts/local/containers/abc/logs?follow=1&tail=5000&since=2026-10-28T07%3A33%3A01.5Z', 'a reconnect asks for everything since the newest line');
        eq(Logs.TAILS, [100, 200, 500, 1000, 5000]);
        const hues = new Set(['web', 'db', 'cache', 'worker', 'proxy', 'x'].map(Logs.hueOf));
        assert.ok([...hues].every(h => h >= 0 && h < 6) && Logs.hueOf('web') === Logs.hueOf('web'), 'a service always gets the same one of six hues');
    }
}

// ---- terminal: the pure helpers of the panel, and the generic diagnosis of the store -----------------------------------------
{
    const Term = window.DockerManagerTerminal;
    assert.ok(Term && typeof Term.create === 'function', 'the terminal module publishes its global');
    eq(Term.socketUrl({ protocol: 'http:', host: 'desk.lan:8443' }, 'local', 'abc', false), 'ws://desk.lan:8443/api/docker-manager/hosts/local/containers/abc/terminal');
    eq(Term.socketUrl({ protocol: 'https:', host: 'desk.lan' }, 'h 1', 'a/b', true), 'wss://desk.lan/api/docker-manager/hosts/h%201/containers/a%2Fb/terminal?confirm=protected', 'https uses wss, the confirmation is a query, parts are escaped');

    const web = container('web'), stopped = container('old', { state: 'exited', status: 'Exited (0) 1 hour ago' });
    const helper = container('aurago-dm-op-1', { aurago_helper: 'runner', protected_owner: 'docker_manager' }), lock = container('ruview', { protected_owner: 'ruview' });
    eq(plain(Term.targetOf(web)), { ref: web.full_id, label: 'web', c: web, state: 'ok', protected: false }, 'a target names the full id of its container');
    eq([Term.targetOf(stopped).state, Term.targetOf(helper).state, Term.targetOf(lock).protected, Term.targetOf(web, 'service').label], ['not_running', 'helper', true, 'service']);
    const detail = { service_list: [{ name: 'web', state: 'running', container_id: web.id }, { name: 'db', state: 'exited', container_id: stopped.id }, { name: 'gone', state: 'running', container_id: 'ffffffffffff' }, { name: 'bare', state: 'running' }, null] };
    eq(Term.stackTargets(detail, [web, stopped]).map(item => [item.label, item.ref, item.state]), [['web', web.full_id, 'ok']], 'a stack offers its running services whose container the list knows');
    const cache = container('cache');
    eq(Term.stackTargets({ service_list: [{ name: 'cache', state: 'running', container_id: cache.full_id }] }, [web, cache]).map(item => [item.label, item.ref]), [['cache', cache.full_id]], 'a 64-character container id of the stack matches the 12-character id of the list');
    eq(Term.stackTargets(null, null), []);
    eq([Term.diagnosePath('local', 'abc', false), Term.diagnosePath('h 1', 'a/b', true)], ['/hosts/local/containers/abc/terminal', '/hosts/h%201/containers/a%2Fb/terminal?confirm=protected'], 'after the confirmation the diagnosis carries it, so a later 429 or 502 is not read as the protection');
    eq([Term.blockedBy(Term.targetOf(web), false), Term.blockedBy(Term.targetOf(web), true), Term.blockedBy(null, false), Term.blockedBy(Term.targetOf(helper), false), Term.blockedBy(Term.targetOf(stopped), false)],
        ['', 'term_readonly', 'term_none_running', 'term_helper', 'term_not_running'], 'read-only wins, then a missing target, a helper, a stopped container');
}
{
    // diagnose is the generic form of diagnoseTerminal: any path below /api/docker-manager, the error info or null
    const api = fakeApi({
        ['GET ' + B + '/hosts/local/stacks/ghost/logs']: () => { throw httpError(404, { code: 'stack_not_found', message: 'no such stack' }); },
        ['GET ' + B + '/hosts/local/containers/web/logs']: () => { throw httpError(429, { code: 'too_many_streams', message: 'full' }); },
        ['GET ' + B + '/hosts/local/containers/ok/logs']: {}
    });
    const store = Store.create({ api, live: false });
    eq((await store.diagnose('/hosts/local/stacks/ghost/logs?follow=1&tail=100')).code, 'stack_not_found');
    eq((await store.diagnose('/hosts/local/containers/web/logs?follow=1&tail=200')).code, 'too_many_streams');
    assert.equal(await store.diagnose('/hosts/local/containers/ok/logs?follow=1'), null, 'a plain answer is no diagnosis');
    assert.equal(api.calls[0].url, B + '/hosts/local/stacks/ghost/logs?follow=1&tail=100', 'the query is kept');
    store.dispose();
}

console.log('docker-manager ui tests: model and store ok');

// ---- stylesheet rules (same contract as the Infra guard: tokens only, gated motion) --------------------------------------
{
    const css = read('ui/css/desktop-app-docker-manager.css').replace(/\/\*[\s\S]*?\*\//g, '');
    const stripMixes = value => value.replace(/color-mix\((?:[^()]|\([^()]*\))*\)/g, 'color-mix()');
    const NAMED = /(^|[\s,(])(white|black|red|green|blue|gray|grey|silver|orange|yellow|purple|pink|navy|teal|maroon|olive|lime|aqua|fuchsia)(?=$|[\s,)!;])/i;
    const COLOR_PROPS = /^(color|background|background-color|border|border-(top|right|bottom|left)|border(-(top|right|bottom|left))?-color|outline|outline-color|fill|stroke|box-shadow|text-shadow|caret-color|accent-color|text-decoration-color|column-rule)$/;
    const MOTION_PROPS = /^(transition|transition-property|transition-duration|animation|animation-name|animation-duration)$/;
    // leafRules: every rule that holds declarations, with the at-rules around it.
    const leafRules = text => {
        const rules = [], stack = [];
        let buffer = '';
        for (const ch of text) {
            if (ch === '{') { stack.push(buffer.trim()); buffer = ''; }
            else if (ch === '}') { const head = stack.pop() || ''; if (buffer.trim()) rules.push({ selector: head, body: buffer.trim(), context: [...stack] }); buffer = ''; }
            else buffer += ch;
        }
        return rules;
    };
    const splitSelectors = selector => {
        const parts = [];
        let depth = 0, current = '';
        for (const ch of selector) {
            if (ch === '(') depth += 1;
            if (ch === ')') depth -= 1;
            if (ch === ',' && depth === 0) { parts.push(current.trim()); current = ''; } else current += ch;
        }
        if (current.trim()) parts.push(current.trim());
        return parts;
    };
    const outside = stripMixes(css);
    eq(outside.match(/#[0-9a-fA-F]{3,8}\b/g) || [], [], 'the stylesheet uses hex colours outside color-mix()');
    eq(outside.match(/\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(\s*[\d.]/g) || [], [], 'the stylesheet uses literal colour functions instead of tokens');
    assert.ok(!/var\(--vd-font[,)]/.test(css), 'use --vd-font-ui, not --vd-font');
    assert.ok(!/\[data-theme[=\]]/.test(css), 'rules must not key on a theme id; use the --vd-theme-* tokens');
    assert.ok(!/(^|[^a-z-])cc-[a-z]/.test(css), 'the cc- class prefix is reserved by cookie-banner blockers; this app uses dm-');
    const named = [], labels = [], ungated = [];
    for (const rule of leafRules(css).filter(item => !item.context.some(head => head.startsWith('@keyframes')))) {
        for (const [, prop, rawValue] of rule.body.matchAll(/([a-z-]+)\s*:\s*([^;]+)/g)) {
            const name = prop.toLowerCase(), value = rawValue.trim();
            if (name.startsWith('--')) continue;
            if (COLOR_PROPS.test(name) && NAMED.test(stripMixes(value))) named.push(rule.selector + ' { ' + name + ': ' + value + ' }');
            if (name === 'text-transform' && /uppercase/i.test(value)) labels.push(rule.selector + ' { text-transform: uppercase }');
            // Logs, raw JSON and technical details are code; every other label keeps the UI font.
            if ((name === 'font' || name === 'font-family') && /monospace/i.test(value) && !/\.dm-(log|pre)\b/.test(rule.selector)) labels.push(rule.selector + ' { ' + name + ': ' + value + ' }');
            if (MOTION_PROPS.test(name) && !/^(none|0m?s)\b/i.test(value)) {
                const gated = rule.context.some(head => /prefers-reduced-motion\s*:\s*no-preference/.test(head)) && splitSelectors(rule.selector).every(part => part.includes(':not([data-animations="false"])'));
                if (!gated) ungated.push(rule.selector + ' { ' + name + ' }');
            }
        }
    }
    eq(named, [], 'the stylesheet uses named colours instead of tokens');
    eq(labels, [], 'the stylesheet has uppercase labels or monospace outside the log and technical-detail surfaces');
    eq(ungated, [], 'motion must sit inside @media (prefers-reduced-motion: no-preference) with body:not([data-animations="false"])');
}

// ---- load contract: every module, each under 1100 lines, none touches the DOM at load, each publishes its global ----------
const uiFiles = ['docker-manager-views.js', 'docker-manager-stacks.js', 'docker-manager-editor.js', 'docker-manager-ops.js', 'docker-manager-logs.js', 'docker-manager-terminal.js', 'docker-manager-panes.js', 'docker-manager-palette.js', 'docker-manager-dialogs.js', 'docker-manager-newstack.js', 'docker-manager.js'];
const full = {};
{
    const all = [...files, ...uiFiles];
    for (const file of all) {
        const lines = read('ui/js/desktop/apps/' + file).split('\n').length;
        assert.ok(lines < 1100, `${file} has ${lines} lines, want fewer than 1100`);
    }
    vm.runInNewContext(all.map(file => read('ui/js/desktop/apps/' + file)).join('\n'), {
        window: full, console, setTimeout, clearTimeout, AbortController, Intl, Map, Set, JSON, Math, Date, URL, performance
    });
    assert.ok(full.DockerManagerCharts && full.DockerManagerLive && full.DockerManagerLogs && full.DockerManagerTerminal && full.DockerManagerPanes, 'the five live modules publish their global');
    assert.ok(full.DockerManagerModel && full.DockerManagerStackModel && full.DockerManagerStore && full.DockerManagerViews && full.DockerManagerStacks && full.DockerManagerEditor && full.DockerManagerOps && full.DockerManagerPalette && full.DockerManagerDialogs && full.DockerManagerNewStack && full.DockerManagerApp, 'all 11 modules publish their global');
    assert.equal(typeof full.DockerManagerApp.render, 'function');
    assert.equal(typeof full.DockerManagerApp.dispose, 'function');
    assert.equal(typeof full.DockerManagerViews.create, 'function');
    assert.equal(typeof full.DockerManagerDialogs.create, 'function');
}

// ---- the stack builders: the rail names the sections, the list groups the rows, user data never reaches the markup unescaped -----
{
    const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    const t = (key, params) => key + (params ? JSON.stringify(params) : '');
    const sm2 = full.DockerManagerStackModel, model2 = full.DockerManagerModel;
    const views = full.DockerManagerViews.create({ t, esc, model: model2, stackModel: sm2, lang: 'en', uid: 'x' });
    const stacks = full.DockerManagerStacks.create({ t, esc, model: model2, stackModel: sm2, views, uid: 'x' });
    const evil = '<img src=x onerror=alert(1)>';
    const stack = (name, extra = {}) => ({ name, host_id: 'local', dir: '/opt/stacks/' + name, compose_file: 'compose.yaml', has_env: false, status: 'running', services: { total: 2, running: 2 }, ports: [{ service: 'web', public_port: 8080, private_port: 80, protocol: 'tcp' }], container_ids: [], created: 1760000000, ...extra });
    const hosts = [hostA, hostB];
    const stacksByHost = {
        local: { loaded: true, listing: { stacks: [stack(evil), stack('blog', { status: 'draft', services: { total: 0, running: 0 }, ports: [] })], external: [{ name: 'legacy', working_dir: '/srv/<b>', status: 'partial', services: { total: 2, running: 1 }, ports: [], container_ids: [] }], ignored: [{ name: evil }] } },
        h_nas: { loaded: false, error: { code: 'host_not_prepared', data: { helpers: { runner_image: 'docker:29.9.2-cli', runner_image_present: false, io_container: 'missing' } } } }
    };
    const byHost = { local: { containers: [container('tools'), container('aurago-dm-op-1', { aurago_helper: 'runner' })], loaded: true }, h_nas: { containers: [], loaded: true } };
    const rows = model2.buildRows(hosts, byHost, 'all');
    const detail = {
        ...stack('shop'), service_list: [{ name: '<b>web</b>', image: 'nginx:1.27', state: 'running', health: 'healthy', ports: [{ service: 'web', ip: '0.0.0.0', public_port: 8080, private_port: 80, protocol: 'tcp' }] }, { name: 'db', image: 'postgres:16', state: 'exited', exit_code: 1, ports: [] }],
        compose: { name: 'compose.yaml', content: 'services: {}', revision: 'r1', exists: true }, env: { name: '.env', content: '', revision: '', exists: false }
    };
    const snap = {
        state: { scope: 'all', stacks: stacksByHost, byHost, hostsLoaded: true, detail: null, stack: { key: 'local/shop', hostId: 'local', name: 'shop', tab: 'overview', status: 'ok', detail, error: null }, busy: new Set(), ops: [] },
        hosts, scopeHosts: hosts, multi: true, canMutate: true, section: 'stacks', query: '', hostStates: { local: 'ok', h_nas: 'not_prepared' }, prepareActive: {}, stackOps: {}, dirty: { compose: true },
        pageHost: 'desk.lan', now: 1760000000000 + 86400000, sgroups: sm2.stackGroups(hosts, stacksByHost, rows, { scope: 'all', query: '' }), stackCounts: sm2.stackCounts(hosts, stacksByHost, 'all')
    };
    const list = stacks.list(snap);
    assert.ok(!list.includes('<img') && !list.includes('<b>') && list.includes('&lt;img'), 'names, folders and ignored folders are escaped');
    assert.ok(list.indexOf('data-sgroup="stacks"') < list.indexOf('data-sgroup="external"') && list.indexOf('data-sgroup="external"') < list.indexOf('data-sgroup="singles"'), 'own stacks, external projects, single containers');
    assert.ok(list.includes('data-action="prepare-host"') && list.includes('data-host-id="h_nas"') && list.includes('docker:29.9.2-cli') && list.includes('prepare_image_missing'), 'an unprepared host offers Prepare host and names what is missing');
    assert.ok(list.includes('data-action="project-action"') && list.includes('data-act="restart"'), 'an external project can be started, stopped and restarted');
    assert.ok(list.includes('class="dm-lock"'), 'a helper container shows its lock');
    assert.ok(!stacks.list(Object.assign({}, snap, { canMutate: false })).includes('prepare-host'), 'a read-only window offers no Prepare host');
    const rail = views.rail(snap);
    assert.ok(rail.includes('data-section="stacks"') && /data-section="stacks"[^>]*aria-current="page"/.test(rail) && !/data-section="containers"[^>]*aria-current/.test(rail), 'the rail marks the current section');
    const head = stacks.head(snap), overview = stacks.overview(snap);
    assert.ok(head.includes('data-op="update"') && head.includes('data-op="restart"') && head.includes('class="dm-dirty"'), 'a running stack offers Update, and an unsaved compose tab is marked');
    assert.ok(stacks.head(Object.assign({}, snap, { stackOps: { 'local/shop': { op: { kind: 'up', state: 'running' } } } })).includes('data-op="update"') && /data-op="update"[^>]*disabled/.test(stacks.head(Object.assign({}, snap, { stackOps: { 'local/shop': { op: { kind: 'up', state: 'running' } } } }))), 'a stack with a running operation cannot start another');
    assert.ok(!stacks.head(Object.assign({}, snap, { canMutate: false })).includes('data-op='), 'a read-only window shows no operation buttons');
    assert.ok(overview.includes('href="http://desk.lan:8080"') && overview.includes('rel="noopener noreferrer"') && !overview.includes('<b>web'), 'ports are links to the page host name, services are escaped');
    const shell = stacks.shell(snap.state.stack);
    assert.ok(shell.includes('data-ed-surface="compose"') && shell.includes('data-ed-surface="env"') && shell.includes('data-action="save-deploy"'), 'the shell holds the two editors and their toolbars');
    assert.ok(stacks.problem(Object.assign({}, snap, { state: Object.assign({}, snap.state, { stack: { key: 'local/gone', name: 'gone', status: 'error', error: { code: 'stack_not_found' } } }) })).includes('stack_gone'), 'a vanished stack says so');
}

// ---- live views: load cells, host gauges, paused banner, drawer parts, follow-panel mounts, the external project pane -------
{
    const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    const t = (key, params) => key + (params ? JSON.stringify(params) : '');
    const sm2 = full.DockerManagerStackModel, model2 = full.DockerManagerModel, charts = full.DockerManagerCharts;
    const views = full.DockerManagerViews.create({ t, esc, model: model2, stackModel: sm2, charts, lang: 'en', uid: 'x' });
    const stacks = full.DockerManagerStacks.create({ t, esc, model: model2, stackModel: sm2, views, uid: 'x' });
    const hosts = [hostA, hostB];
    const web = container('web', { compose_project: 'shop', compose_service: 'web' }), idle = container('old', { state: 'exited', status: 'Exited (0) 1 hour ago' });
    const byHost = { local: { containers: [web, idle, container('legacy-app', { compose_project: 'legacy', compose_service: 'app', compose_working_dir: '/srv/legacy' }), container('legacy-db', { compose_project: 'legacy', compose_service: 'db', compose_working_dir: '/srv/other' })], loaded: true }, h_nas: { containers: [], loaded: true } };
    const external = [{ name: 'legacy', working_dir: '/srv/legacy', status: 'partial', services: { total: 2, running: 1 }, ports: [], container_ids: [] }];
    const stacksByHost = { local: { loaded: true, listing: { stacks: [], external, ignored: [] } } };
    const rows = model2.buildRows(hosts, byHost, 'all');
    const snapBase = {
        state: { scope: 'all', stacks: stacksByHost, byHost, hostsLoaded: true, detail: null, stack: null, project: null, busy: new Set(), ops: [], df: {} },
        hosts, scopeHosts: hosts, rows, multi: true, canMutate: true, headers: false, collapsed: new Set(), groups: [], section: 'containers',
        liveStatus: { local: 'live', h_nas: 'connecting' }, hostStats: {}, hostStatsFor: {}, pageHost: 'desk.lan', now: 1760000000000
    };

    // the table: running containers get an empty load placeholder (filled in place), stopped ones an empty cell, group rows span six columns
    const groups = model2.groupRows(rows, { multiHost: true, hostOrder: ['local', 'h_nas'] });
    const body = views.tbody(Object.assign({}, snapBase, { groups, headers: true }));
    assert.ok(body.includes('class="dm-cell-load" data-load="local/' + web.id + '"></td>'), 'a running container has a load placeholder keyed by host and short id');
    assert.equal((body.match(/data-load=/g) || []).length, 3, 'only the running containers have one');
    assert.ok(body.includes('colspan="6"') && !body.includes('colspan="5"'), 'group rows span all six columns');
    assert.ok(!/<td[^>]*class="dm-cell-load"[^>]*>[^<]+</.test(body), 'the markup of the table never holds numbers: they are patched in place');

    // the drawer: top and body apart; the follow tabs have no body
    const drawerSnap = tab => Object.assign({}, snapBase, { state: Object.assign({}, snapBase.state, { detail: { key: rows[0].key, hostId: 'local', id: web.full_id, tab, inspect: { status: 'idle', data: null, error: null } } }) });
    const row0 = rows[0];
    const overviewParts = views.drawer(drawerSnap('overview'), row0);
    assert.ok(overviewParts.top.includes('data-tab="terminal"') && overviewParts.top.includes('aria-controls="x-tabpanel-terminal"') && overviewParts.body.includes('id="x-tabpanel-overview"') && overviewParts.live === '', 'four tabs, the Overview body');
    const logsParts = views.drawer(drawerSnap('logs'), row0), termParts = views.drawer(drawerSnap('terminal'), row0);
    eq([logsParts.body, logsParts.live, termParts.body, termParts.live], ['', 'logs', '', 'terminal'], 'the follow tabs are persistent panels, not part of the repainted body');
    assert.ok(/data-tab="logs"[^>]*aria-selected="true"/.test(logsParts.top) && /data-tab="overview"[^>]*aria-selected="false"/.test(logsParts.top), 'the open tab is the selected one');
    const hostile = views.drawer(drawerSnap('"><img src=x onerror=alert(1)>'), row0);
    assert.ok(!hostile.body.includes('<img') && hostile.body.includes('data-tab-body="&quot;&gt;&lt;img'), 'the tab name never reaches an attribute unescaped');
    assert.ok(hostile.body.includes('id="x-tabpanel-overview"'), 'an unknown tab shows the overview');
    assert.deepStrictEqual([...full.DockerManagerViews.TABS], ['overview', 'logs', 'terminal', 'inspect']);
    assert.equal(views.errorText({ code: 'too_many_streams' }), 'error_too_many_streams{"version":"","command":""}', 'the new codes have their own sentences');
    assert.ok(views.errorText({ code: 'helper_container' }).startsWith('error_helper_container') && views.errorText({ code: 'project_not_found' }).startsWith('error_project_not_found'));
    eq(['forbidden_origin', 'docker_hosts_unavailable', 'docker_disabled'].map(code => views.errorText({ code }).startsWith('error_' + code)), [true, true, true], 'a refused origin, unavailable hosts and a switched-off Docker have sentences');
    assert.equal(views.errorText({ code: 'host_unreachable', message: 'dial tcp 10.0.0.2:2375: connect: refused' }), 'reason_unknown dial tcp 10.0.0.2:2375: connect: refused', 'a host_unreachable without a classified reason (an ended log stream) names the cause');
    assert.equal(views.errorText({ code: 'host_unreachable', reason: 'refused', message: 'dial tcp x' }), 'reason_refused', 'a classified reason is the whole sentence, as before');

    // the rail gauges: one host only
    assert.equal(views.railGauges(snapBase), '', 'no gauges under All hosts');
    const gauged = (stateExtra, extra = {}) => Object.assign({}, snapBase, { state: Object.assign({}, snapBase.state, { scope: 'local' }, stateExtra) }, extra);
    const stats = { host: 'local', t: 1, cpu: 42.5, ncpu: 8, mem_used: 1073741824, mem_total: 4294967296, running: 3, sampled: 3, partial: false };
    const df = { images: { size: 1000 }, containers: { size: 10 }, volumes: { size: 500 }, build_cache: { size: 0 }, total: 1510 };
    const full1 = views.railGauges(gauged({ df: { local: { status: 'ok', df, busy: false } } }, { hostStats: { local: stats } }));
    assert.ok(full1.includes('>gauge_cpu<') && full1.includes('>43%<') && full1.includes('1.0 GB / 4.0 GB') && full1.includes('1.5 KB'), 'CPU as a percent, memory used of total, Docker disk in bytes');
    assert.ok(full1.includes('data-action="refresh-df"') && full1.includes('data-host-id="local"') && !full1.includes('gauge_partial') && !full1.includes('gauge_paused'), 'a refresh button and no hint while everything is fine');
    assert.ok(views.railGauges(gauged({ df: { local: { status: 'ok', df, busy: true } } }, { hostStats: { local: stats } })).includes('disabled aria-busy="true"'), 'the refresh button is busy while the disk usage is read');
    assert.ok(views.railGauges(gauged({}, { hostStats: { local: Object.assign({}, stats, { partial: true, sampled: 100, running: 140 }) } })).includes('gauge_partial{&quot;sampled&quot;:100,&quot;running&quot;:140}'), 'a partial sample says how many containers it covers');
    const waiting = views.railGauges(gauged({}));
    assert.ok(waiting.includes('gauge_waiting') && waiting.includes('data-level="none"'), 'before the first sample the gauges are empty and say so');
    assert.ok(views.railGauges(gauged({}, { hostStats: { local: stats }, liveStatus: { local: 'paused' } })).includes('gauge_paused'), 'paused live data is named');
    assert.ok(views.railGauges(gauged({ df: { local: { status: 'error', df: null, error: { code: 'host_unreachable' }, busy: false } } }, { hostStats: { local: stats } })).includes('gauge_failed'), 'a failed disk reading is named');
    assert.ok(!views.railGauges(gauged({ scope: 'h_gone' })).length, 'a host that is not in the list has no gauges');

    // the banner for paused live updates: not for a live host, not for one that is unreachable (its own banner says that)
    const bannerSnap = (liveStatus, nasEntry) => Object.assign({}, snapBase, { liveStatus, state: Object.assign({}, snapBase.state, { byHost: Object.assign({}, byHost, { h_nas: nasEntry || byHost.h_nas }) }) });
    const paused = views.banners(bannerSnap({ local: 'live', h_nas: 'paused' }));
    assert.ok(paused.includes('data-live-banner="h_nas"') && paused.includes('live_paused{&quot;host&quot;:&quot;NAS&quot;}') && !paused.includes('data-live-banner="local"') && paused.includes('role="status"'), 'a paused host gets a polite banner');
    assert.ok(!views.banners(bannerSnap({ local: 'live', h_nas: 'paused' }, { containers: [], loaded: false, error: { code: 'host_unreachable', reason: 'timeout' } })).includes('data-live-banner'), 'an unreachable host keeps only its own banner');
    assert.ok(!views.banners(bannerSnap({ local: 'connecting', h_nas: 'off' })).includes('data-live-banner'), 'no banner while the stream is still coming up');

    // stack pane: five tabs, mounts for the follow panels, load placeholders on running services
    const detail = {
        name: 'shop', host_id: 'local', dir: '/opt/stacks/shop', compose_file: 'compose.yaml', has_env: false, status: 'running', services: { total: 2, running: 1 }, ports: [], created: 1760000000,
        service_list: [{ name: 'web', image: 'nginx', state: 'running', container_id: web.id, ports: [] }, { name: 'db', image: 'postgres', state: 'exited', exit_code: 1, container_id: idle.id, ports: [] }]
    };
    const stackSnap = tab => Object.assign({}, snapBase, { section: 'stacks', dirty: {}, canMutate: true, stackOps: {}, state: Object.assign({}, snapBase.state, { stack: { key: 'local/shop', hostId: 'local', name: 'shop', tab, status: 'ok', detail, error: null } }) });
    const head = stacks.head(stackSnap('logs'));
    assert.ok(['overview', 'compose', 'env', 'logs', 'terminal'].every(tab => head.includes('data-action="stack-tab" data-tab="' + tab + '"')) && /data-tab="logs"[^>]*aria-selected="true"/.test(head), 'five stack tabs');
    const shell = stacks.shell(stackSnap('overview').state.stack);
    assert.ok(shell.includes('data-panel="logs"') && shell.includes('data-mount="logs"') && shell.includes('data-panel="terminal"') && shell.includes('data-mount="terminal"') && /data-panel="logs"[^>]*hidden/.test(shell), 'the shell holds a mount per follow panel, hidden until its tab opens');
    const overview = stacks.overview(stackSnap('overview'));
    assert.ok(overview.includes('data-load="local/' + web.id + '"') && (overview.match(/data-load=/g) || []).length === 1, 'only the running service has a load placeholder');
    const withFullId = Object.assign({}, stackSnap('overview'), { state: Object.assign({}, stackSnap('overview').state, { stack: Object.assign({}, stackSnap('overview').state.stack, { detail: Object.assign({}, detail, { service_list: [{ name: 'web', image: 'nginx', state: 'running', container_id: web.full_id, ports: [] }] }) }) }) });
    const fullOverview = stacks.overview(withFullId);
    assert.ok(fullOverview.includes('data-load="local/' + web.id + '"') && fullOverview.includes('data-container-id="' + web.id + '"') && !fullOverview.includes(web.full_id), 'a service container that comes with its 64-character id is keyed by the 12-character id of the stream and of the container list');

    // the external project pane
    const projectSnap = tab => Object.assign({}, snapBase, { section: 'stacks', canMutate: true, state: Object.assign({}, snapBase.state, { project: { key: 'x:local/legacy', hostId: 'local', name: 'legacy', tab } }) });
    const phead = stacks.projectHead(projectSnap('overview'));
    assert.ok(phead.includes('data-action="project-tab" data-tab="overview"') && phead.includes('data-action="project-tab" data-tab="logs"') && !phead.includes('data-tab="compose"') && phead.includes('data-act="restart"') && phead.includes('data-action="close-project"') && phead.includes('title="/srv/legacy"'), 'two tabs, start/stop/restart, close, the working directory');
    assert.ok(!stacks.projectHead(Object.assign({}, projectSnap('overview'), { canMutate: false })).includes('data-act='), 'a read-only window shows no project actions');
    const pshell = stacks.projectShell({ key: 'x:local/legacy', tab: 'logs' });
    assert.ok(/data-panel="overview"[^>]*hidden/.test(pshell) && !/data-panel="logs"[^>]*hidden/.test(pshell) && pshell.includes('data-mount="logs"'), 'the project shell shows the open tab');
    const poverview = stacks.projectOverview(projectSnap('overview'));
    assert.ok(poverview.includes('legacy-app') && !poverview.includes('legacy-db') && poverview.includes('data-action="show-project"') && (poverview.match(/data-load=/g) || []).length === 1, 'the containers of this project and working directory only, with a load placeholder and a way to the Containers section');
    const gone = Object.assign({}, projectSnap('overview'), { state: Object.assign({}, projectSnap('overview').state, { stacks: {}, byHost: { local: { containers: [], loaded: true } } }) });
    assert.ok(stacks.projectHead(gone).includes('project_gone') && stacks.projectOverview(gone).includes('project_gone'), 'a project that left the listing says so');
    const list = stacks.list(Object.assign({}, snapBase, { section: 'stacks', hostStates: { local: 'ok', h_nas: 'ok' }, prepareActive: {}, sgroups: sm2.stackGroups(hosts, stacksByHost, rows, { scope: 'all', query: '' }), stackCounts: { total: 1 } }));
    assert.ok(list.includes('data-action="open-project" data-host-id="local" data-project="legacy"') && !list.includes('data-action="show-project"'), 'an external row opens the project pane');
}

// ---- panes: follow panels follow their tab, load cells are patched in place, the visible ones are subscribed -------------
{
    // a tiny DOM: just enough for the panes module (querySelector(All), dataset, hidden, getBoundingClientRect, getClientRects)
    const makeEl = (props = {}) => {
        const el = Object.assign({ hidden: false, dataset: {}, children: [], innerHTML: '', title: '', rect: { top: 0, bottom: 20 }, shown: true }, props);
        el.getBoundingClientRect = () => el.rect;
        el.getClientRects = () => (el.shown ? [el.rect] : []);
        el.querySelector = selector => { const match = /\[data-(live-panel|mount)="(\w+)"\]/.exec(selector); return match ? (el.mounts && el.mounts[match[2]]) || null : null; };
        el.querySelectorAll = selector => (selector === '[data-load]' ? el.cells || [] : []);
        return el;
    };
    const calls = [];
    const makePanel = kind => { const panel = { kind, active: false, disposed: false, updates: 0, focused: 0, setActive(v) { panel.active = v; calls.push([kind, 'active', v]); }, update() { panel.updates += 1; }, focus() { panel.focused += 1; }, dispose() { panel.disposed = true; calls.push([kind, 'dispose']); } }; return panel; };
    const opened = { logs: [], terminal: [] };
    const logsApi = { open: (el, target) => { const p = makePanel('logs'); p.target = target; p.el = el; opened.logs.push(p); return p; } };
    const termApi = { open: (el, config) => { const p = makePanel('terminal'); p.config = config; p.el = el; opened.terminal.push(p); return p; } };
    const webRow = { key: 'local/' + 'web'.padEnd(64, '0'), c: container('web') };
    const store = { state: { stack: null, byHost: { local: { containers: [webRow.c] } } }, canMutate: () => true, liveRing: (host, id) => (id === 'good' ? [{ t: 1, cpu: 5, mem: 1048576, net_rx: 0, net_tx: 0 }, { t: 2001, cpu: 7, mem: 2097152, net_rx: 2048, net_tx: 0 }] : []) };
    const panes = full.DockerManagerPanes.create({ t: (key, params) => key + JSON.stringify(params || {}), model: model, charts: full.DockerManagerCharts, logs: logsApi, terminal: termApi, store, findRow: key => (key === webRow.key ? webRow : null), lang: 'en' });
    const live = makeEl({ mounts: { logs: makeEl(), terminal: makeEl() } });

    panes.drawer(live, { key: webRow.key, hostId: 'local', id: webRow.c.full_id, tab: 'overview' }, false);
    eq([opened.logs.length, opened.terminal.length], [0, 0], 'nothing is built before its tab is shown');
    panes.drawer(live, { key: webRow.key, hostId: 'local', id: webRow.c.full_id, tab: 'logs' }, false);
    eq([opened.logs.length, opened.logs[0].active, live.mounts.logs.hidden, live.mounts.terminal.hidden], [1, true, false, true], 'the logs panel is built and follows');
    eq(plain(opened.logs[0].target), { kind: 'container', hostId: 'local', ref: webRow.c.full_id, label: 'web', services: undefined }, 'it follows the container by its full id');
    panes.drawer(live, { key: webRow.key, hostId: 'local', id: webRow.c.full_id, tab: 'terminal' }, true);
    eq([opened.logs[0].active, opened.terminal.length, opened.terminal[0].active, opened.terminal[0].focused, opened.terminal[0].config.autofocus], [false, 1, true, 1, true], 'switching tabs stops one and starts the other, the panels stay');
    eq(plain(opened.terminal[0].config.targets().map(item => [item.ref, item.state])), [[webRow.c.full_id, 'ok']], 'the terminal asks the current container list');
    assert.equal(opened.terminal[0].config.readonly(), false);
    panes.drawer(live, { key: webRow.key, hostId: 'local', id: webRow.c.full_id, tab: 'logs' }, false);
    assert.equal(opened.logs.length, 1, 'coming back reuses the panel');
    panes.drawer(live, { key: 'local/other', hostId: 'local', id: 'x', tab: 'logs' }, false);
    eq([opened.logs[0].disposed, opened.terminal[0].disposed], [true, true], 'another container or a closed drawer shuts the panels down');
    panes.drawer(live, null, false);

    // the stack pane: the panel is built on its tab, services reach the chips, a replaced mount builds again, a covered pane follows nothing
    const shellEl = makeEl({ mounts: { logs: makeEl(), terminal: makeEl() } });
    const stackState = tab => ({ key: 'local/shop', hostId: 'local', name: 'shop', tab, status: 'ok', detail: { service_list: [{ name: 'web', state: 'running', container_id: webRow.c.id }, { name: 'db', state: 'exited', container_id: 'ffffffffffff' }] } });
    store.state.stack = stackState('logs');
    const before = opened.logs.length;
    panes.stack(shellEl, store.state.stack, true, false);
    const stackLogs = opened.logs[before];
    eq([stackLogs.target.kind, stackLogs.target.name, stackLogs.target.services, stackLogs.active], ['stack', 'shop', ['web', 'db'], true]);
    panes.stack(shellEl, store.state.stack, false, false);
    assert.equal(stackLogs.active, false, 'a covered pane follows nothing');
    shellEl.mounts.logs = makeEl();
    panes.stack(shellEl, store.state.stack, true, false);
    eq([stackLogs.disposed, opened.logs.length - before], [true, 2], 'a mount that was replaced gets a new panel');
    store.state.stack = stackState('terminal');
    panes.stack(shellEl, store.state.stack, true, false);
    eq(plain(opened.terminal[opened.terminal.length - 1].config.targets().map(item => item.label)), ['web'], 'the stack terminal offers its running services');
    panes.stack(shellEl, null, true, false);
    assert.equal(opened.terminal[opened.terminal.length - 1].disposed, true, 'no stack, no panels');

    // the project pane
    const projectEl = makeEl({ mounts: { logs: makeEl() } });
    panes.project(projectEl, { key: 'x:local/legacy', hostId: 'local', name: 'legacy', tab: 'logs' }, true);
    const projectLogs = opened.logs[opened.logs.length - 1];
    eq([projectLogs.target.kind, projectLogs.target.name, projectLogs.active], ['project', 'legacy', true]);
    panes.project(projectEl, null, true);
    assert.equal(projectLogs.disposed, true);

    // load cells: filled from the ring, only the asked ones, nothing again for an unchanged ring
    const cell = (key, extra = {}) => makeEl({ dataset: { load: key }, ...extra });
    const good = cell('local/good'), other = cell('local/none');
    const table = makeEl({ cells: [good, other] });
    panes.patchLoads(table, new Set(['local/good']));
    assert.ok(good.innerHTML.includes('dm-load-line') && good.innerHTML.includes('>7.0%<') && good.title.startsWith('stat_title') && good.title.includes('"rx":"1.0 KB"'), 'a cell shows CPU and memory and names the network rate in its title');
    eq([other.innerHTML, other.dataset.sig], ['', undefined], 'a cell that was not asked for stays as it is');
    good.innerHTML = 'marker';
    panes.patchLoads(table);
    assert.equal(good.innerHTML, 'marker', 'an unchanged ring is not painted again');
    store.liveRing = () => [];
    panes.patchLoads(table);
    eq([good.innerHTML, other.innerHTML], ['', ''], 'a container without points shows no cell');

    // which containers are on screen: rendered, inside the viewport, at most 50 per host, once each
    const viewport = makeEl({ rect: { top: 100, bottom: 300 } });
    const many = Array.from({ length: 60 }, (_, i) => cell('local/c' + String(i).padStart(11, '0'), { rect: { top: 150, bottom: 170 } }));
    const entries = [
        { root: makeEl({ cells: [cell('local/in', { rect: { top: 120, bottom: 140 } }), cell('local/above', { rect: { top: 10, bottom: 30 } }), cell('local/below', { rect: { top: 400, bottom: 420 } }), cell('local/hidden', { shown: false }), cell('h_nas/n1', { rect: { top: 200, bottom: 220 } }), cell('local/in', { rect: { top: 130, bottom: 150 } })] }), viewport },
        { root: makeEl({ cells: many }), viewport }
    ];
    const visible = panes.visibleLoads(entries);
    eq([visible['h_nas'], visible.local.slice(0, 2), visible.local.length], [['n1'], ['in', 'c00000000000'], 50], 'rendered, inside the viewport, once each, at most 50 per host');
    eq(panes.visibleLoads([{ root: null, viewport }]), {});
    panes.dispose();
}

// ---- terminal panel: read-only ends an open shell; a hidden tab keeps it; closing the pane, another container and the window end it ----
{
    // A tiny DOM (just the elements the terminal module looks up), a fake xterm and a fake WebSocket, in a vm of their own that has a `document`.
    const fakeMount = () => {
        const parts = {}, node = () => ({ hidden: false, textContent: '', innerHTML: '', value: '', dataset: {}, clientWidth: 0, clientHeight: 0, style: {}, append() {}, remove() {}, focus() {}, addEventListener() {}, querySelector: () => ({ textContent: '' }) });
        return { hidden: false, parts, replaceChildren() {}, querySelector(selector) { const key = /data-tp-[a-z-]+/.exec(selector)[0]; return parts[key] || (parts[key] = node()); } };
    };
    const harness = () => {
        const win = {}, sockets = [], terms = [], ro = { on: false };
        win.Terminal = class {
            constructor() { this.cols = 80; this.rows = 24; terms.push(this); }
            loadAddon() {} open() {} attachCustomKeyEventHandler() {} reset() {} write() {} focus() {}
            onData(fn) { this.data = fn; }
            onBinary(fn) { this.binary = fn; }
            dispose() { this.disposed = true; }
        };
        vm.runInNewContext(['docker-manager-model.js', 'docker-manager-terminal.js', 'docker-manager-panes.js'].map(file => read('ui/js/desktop/apps/' + file)).join('\n'), {
            window: win, console, Map, Set, JSON, Math, Date, URL, AbortController, setTimeout, clearTimeout, TextEncoder,
            document: { createElement: () => ({ style: {}, className: '', remove() {}, getContext: () => null }) },
            getComputedStyle: () => ({ fontFamily: 'sans-serif', color: 'rgb(0, 0, 0)' })
        });
        const openSocket = url => {
            const socket = { url, readyState: 0, sent: [], closed: false };
            socket.send = data => { socket.sent.push(typeof data === 'string' ? data : 'bin:' + Buffer.from(data).toString()); };
            socket.close = () => { socket.readyState = 3; socket.closed = true; };
            socket.open = () => { socket.readyState = 1; socket.onopen(); };
            sockets.push(socket);
            return socket;
        };
        const terminal = win.DockerManagerTerminal.create({
            t: key => key, esc: String, views: { errorText: info => info.message }, diagnose: async () => null, confirmProtected: async () => true,
            socket: openSocket, requestFrame: fn => fn(), location: { protocol: 'http:', host: 'desk.lan' }
        });
        return { win, Term: win.DockerManagerTerminal, terminal, sockets, terms, ro };
    };
    const END = '{"type":"end"}';
    const web = container('web'), nas = container('nas-web');

    // read-only switched on while a shell is open: the input stops at once, the next update() ends the session with the end frame
    {
        const h = harness(), el = fakeMount();
        const panel = h.terminal.open(el, { hostId: 'local', targets: () => [h.Term.targetOf(web)], readonly: () => h.ro.on });
        panel.setActive(true);
        h.sockets[0].open();
        h.terms[0].data('ls');
        eq([panel.phase(), h.sockets[0].sent], ['connected', ['bin:ls']], 'a shell opens and takes input');
        h.ro.on = true;
        h.terms[0].data('rm');
        eq(h.sockets[0].sent, ['bin:ls'], 'input is dropped as soon as read-only is on, even before the panel noticed');
        panel.update();
        eq([panel.phase(), h.sockets[0].sent, h.sockets[0].closed, el.parts['data-tp-msg'].textContent, el.parts['data-tp-screen'].hidden], ['idle', ['bin:ls', END], true, 'term_readonly', true], 'read-only ends the open shell and says why');
        h.terms[0].data('x');
        eq(h.sockets[0].sent, ['bin:ls', END], 'nothing is sent after the end');
        h.ro.on = false;
        panel.update();
        eq([el.parts['data-tp-msg'].textContent, el.parts['data-tp-start'].hidden, h.sockets.length], ['', false, 1], 'lifting read-only offers the shell again, without opening one by itself');
        panel.dispose();

        // a handshake that is still open is closed too (there is no shell to end yet)
        const el2 = fakeMount(), panel2 = h.terminal.open(el2, { hostId: 'local', targets: () => [h.Term.targetOf(web)], readonly: () => h.ro.on });
        panel2.setActive(true);
        const pending = h.sockets[1];
        eq([panel2.phase(), pending.readyState], ['connecting', 0]);
        h.ro.on = true;
        panel2.update();
        eq([panel2.phase(), pending.closed, pending.sent, el2.parts['data-tp-msg'].textContent], ['idle', true, [], 'term_readonly'], 'read-only also cancels a connecting shell');
        panel2.dispose();
    }

    // the panes keep a shell while its pane exists (other tab, covered, minimised) and end it when the pane or the window closes
    {
        const setup = () => {
            const h = harness(), rows = [{ key: 'local/' + web.full_id, c: web }, { key: 'h_nas/' + nas.full_id, c: nas }];
            const store = { state: { stack: null, byHost: {} }, canMutate: () => !h.ro.on };
            const logs = { open: () => ({ setActive() {}, update() {}, focus() {}, dispose() {} }) };
            const panes = h.win.DockerManagerPanes.create({ t: key => key, model: h.win.DockerManagerModel, charts: {}, logs, terminal: h.terminal, store, findRow: key => rows.find(row => row.key === key) || null, lang: 'en' });
            const mount = fakeMount(), live = { querySelector: selector => (selector.includes('"terminal"') ? mount : null) };
            const detail = (index, tab) => ({ key: rows[index].key, hostId: index ? 'h_nas' : 'local', id: rows[index].c.full_id, tab });
            panes.drawer(live, detail(0, 'terminal'), false);
            h.sockets[0].open();
            return { h, panes, live, detail };
        };
        {
            const { h, panes, live, detail } = setup();
            for (const tab of ['', 'overview', 'logs', 'terminal']) panes.drawer(live, detail(0, tab), false);
            eq([h.sockets.length, h.sockets[0].sent, h.sockets[0].closed], [1, [], false], 'another tab, a covered drawer or a hidden window leave the shell open');
            panes.drawer(live, null, false);
            eq([h.sockets[0].sent, h.sockets[0].closed, h.terms[0].disposed], [[END], true, true], 'closing the pane ends the shell');
        }
        {
            const { h, panes, live, detail } = setup();
            panes.drawer(live, detail(1, 'terminal'), false);
            eq([h.sockets[0].sent, h.sockets[0].closed, h.sockets.length, h.sockets[1].url.includes('/hosts/h_nas/')], [[END], true, 2, true], 'another container on another host ends the shell and opens its own');
        }
        {
            const { h, panes } = setup();
            panes.dispose();
            eq([h.sockets[0].sent, h.sockets[0].closed], [[END], true], 'closing the window ends the shell');
        }
        {
            const { h, panes, live, detail } = setup();
            h.ro.on = true;
            panes.drawer(live, detail(0, 'terminal'), false);
            eq([h.sockets[0].sent, h.sockets[0].closed], [[END], true], 'a repaint after read-only was switched on reaches the panel and ends the shell');
        }
    }
}

// ---- the lint scheduler: one request per quiet period, the newest answer wins, aborted requests are silent ------------------------
{
    const { createLinter, LINT_DELAY_MS } = full.DockerManagerEditor;
    assert.equal(LINT_DELAY_MS, 400, 'the editors lint 400 ms after the last edit');
    const delivered = [];
    let calls = 0;
    const linter = createLinter(() => { calls += 1; const mine = calls; return new Promise(resolve => setTimeout(() => resolve({ n: mine }), mine === 1 ? 150 : 5)); }, (result, error) => delivered.push(error ? 'error' : result.n), 20);
    linter.schedule();
    linter.schedule();
    await sleep(70);
    assert.equal(calls, 1, 'two edits in a row make one request');
    linter.schedule();
    await sleep(300);
    eq(delivered, [2], 'the answer of a request that a newer one replaced is dropped');
    const failures = [];
    const failing = createLinter(() => Promise.reject(Object.assign(new Error('x'), { dm: { code: failures.length ? 'aborted' : 'timeout' } })), (result, error) => failures.push(error ? error.dm.code : 'result'), 5);
    await failing.flush();
    await failing.flush();
    eq(failures, ['timeout'], 'a failure is delivered, an aborted request is silent');
    const cancelled = [];
    const quiet = createLinter(() => Promise.resolve({ n: 1 }), result => cancelled.push(result), 5);
    quiet.schedule();
    quiet.cancel();
    await sleep(80);
    quiet.dispose();
    eq(cancelled, [], 'a cancelled lint never runs');
}

// ---- the workbench: a save started for one stack never writes into the drafts of another or deploys it -------------------------------
{
    // The smallest DOM the editor needs to mount its plain text area (CodeMirror never loads here).
    const element = () => ({
        className: '', id: '', hidden: false, dataset: {}, children: [], textContent: '', innerHTML: '', value: '', readOnly: false, spellcheck: false, listeners: {},
        append(...nodes) { this.children.push(...nodes); }, replaceChildren(...nodes) { this.children = nodes; }, setAttribute() {},
        addEventListener(type, handler) { (this.listeners[type] = this.listeners[type] || []).push(handler); }
    });
    const editorWindow = {};
    vm.runInNewContext(read('ui/js/desktop/apps/docker-manager-editor.js'), { window: editorWindow, document: { createElement: element }, console, setTimeout, clearTimeout, AbortController, Map, Set, JSON, Math, Date, Promise });
    const t2 = key => key, never = () => new Promise(() => {});
    const sm2 = full.DockerManagerStackModel;
    const factory = editorWindow.DockerManagerEditor.create({ t: t2, esc: String, stackModel: sm2, views: { errorText: () => 'error', dot: () => '' }, uid: 'x', load: never });
    const stackOf = (name, content) => ({ key: 'local/' + name, hostId: 'local', name, detail: { compose: { content, revision: 'r-' + name, exists: true }, env: { content: '', revision: '', exists: false } } });
    const store = { state: { stack: null }, lint: () => Promise.resolve({ diagnostics: [] }), saves: [], saveFiles(hostId, name, payload) { return new Promise(resolve => this.saves.push({ hostId, name, payload, resolve })); } };
    const deployed = [], toasts = [];
    const bench = factory.createWorkbench({ store, dialogs: {}, toast: (message, tone) => toasts.push(tone), announce() {}, canMutate: () => true, onChange() {}, onDeploy: key => deployed.push(key) });
    const surface = element(), pane = { querySelector: selector => (selector === '[data-ed-surface="compose"]' ? surface : null), querySelectorAll: () => [] };
    const openStack = stack => { store.state.stack = stack; bench.open(pane, stack); bench.showTab('compose'); };
    const edit = text => {
        const area = surface.children[0].children[0].children[0];
        area.value = text;
        for (const handler of area.listeners.input) handler({});
    };

    openStack(stackOf('a', 'a: 1'));
    edit('a: 2');
    assert.equal(bench.anyDirty(), true, 'an edited draft is dirty');
    const own = bench.save(['compose'], true);
    store.saves[0].resolve({ compose: 'r-a2' });
    assert.equal(await own, true, 'a save of the open stack succeeds');
    eq(deployed, ['local/a'], 'a save with deploy hands the key of the stack it saved to onDeploy');
    eq([store.saves[0].name, store.saves[0].payload.compose.revision], ['a', 'r-a'], 'the save went to stack a with the revision its draft was based on');

    edit('a: 3');
    const stale = bench.save(['compose'], true);
    openStack(stackOf('b', 'b: 1'));
    toasts.length = 0;
    store.saves[1].resolve({ compose: 'r-a3' });
    assert.equal(await stale, false, 'a save that finished after another stack was opened reports failure');
    eq(deployed, ['local/a'], 'and deploys nothing');
    eq(bench.dirtyState(), { compose: false, env: false }, 'the drafts of stack b were not written into');
    eq(toasts, [], 'and nothing is reported as saved');
    edit('b: 2');
    const second = bench.save(['compose'], false);
    store.saves[2].resolve({ compose: 'r-b2' });
    assert.equal(await second, true, 'the workbench saves for the new stack once the stale save is over');
    eq([store.saves[2].name, store.saves[2].payload.compose.revision], ['b', 'r-b'], 'the next save goes to stack b');
    bench.close();
}

// ---- the progress drawer: a status line, entries with bars, output without colour codes, everything escaped --------------------
{
    const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    const t = (key, params) => key + (params ? JSON.stringify(params) : '');
    const sm2 = full.DockerManagerStackModel, model2 = full.DockerManagerModel;
    const views = full.DockerManagerViews.create({ t, esc, model: model2, stackModel: sm2, lang: 'en', uid: 'x' });
    const store2 = { state: { ops: [] } };
    const ops = full.DockerManagerOps.create({ t, esc, model: model2, stackModel: sm2, views, store: store2, toast: () => {}, announce: () => {}, uid: 'x' });
    const snap = { state: store2.state, hosts: [hostA], multi: false, pageHost: 'desk.lan', now: Date.parse('2026-10-27T10:00:30Z') };
    assert.ok(ops.html(snap).includes('ops_idle'), 'nothing running: the status line says so');
    const entry = sm2.newOpEntry({ id: 'op_1', host_id: 'local', stack: '<b>shop</b>', kind: 'up', state: 'running', created_at: '2026-10-27T10:00:00Z', output: '\x1b[32mhello <i>\x1b[0m\n' }, 0);
    sm2.applyOpEvent(entry, { type: 'progress', progress: { id: 'db layer', current: 1, total: 4 } });
    store2.state.ops = [entry];
    const closed = ops.html(snap);
    assert.ok(/aria-expanded="false"/.test(closed) && /dm-ops-body" id="x-ops-body" hidden/.test(closed), 'collapsed: only the status line shows');
    ops.setOpen(true);
    const html = ops.html(snap);
    assert.ok(!html.includes('<b>shop') && html.includes('&lt;b&gt;shop'), 'the stack name is escaped');
    assert.ok(!html.includes('\x1b') && html.includes('hello &lt;i&gt;'), 'terminal colour codes are removed and the output is escaped');
    assert.ok(html.includes('<progress') && html.includes('value="25"') && html.includes('db layer'), 'a bar per image that reported a total');
    assert.ok(html.includes('data-elapsed="op_1"') && html.includes('0:30'), 'the elapsed time counts from the start');
    assert.equal(ops.tick({ querySelectorAll: () => [] }), true, 'the ticker keeps going while an operation runs');
    entry.op.state = 'failed';
    entry.op.error = 'line 3: bad';
    entry.op.error_line = 3;
    const failed = ops.html(snap);
    assert.ok(failed.includes('data-action="op-goto-line"') && failed.includes('line 3: bad') && failed.includes('data-op-output'), 'a failed operation shows its error, a way to the line and its output');
    assert.equal(ops.tick({ querySelectorAll: () => [] }), false, 'the ticker stops when nothing runs');
}

// ---- the owner of a 409 protected-confirmation answer gets the translated sentence, never the server's message -----------------
{
    const t = (key, params) => key + (params ? JSON.stringify(params) : '');
    const views = full.DockerManagerViews.create({ t, esc: String, model: full.DockerManagerModel, lang: 'en', uid: 'x' });
    eq(['self', 'docker-endpoint', 'shared-network', 'unverified', 'docker_manager', 'ruview', ''].map(views.protectedLabelText),
        ['protected_self', 'protected_endpoint', 'protected_network', 'protected_unverified', 'protected_helper', 'protected_owner{"owner":"ruview"}', ''], 'a helper container of the Stacks feature names itself, not an owner');
}

// ---- error texts: the codes of the host and operation routes, and the deploy that could not start -------------------------------
{
    const t = (key, params) => key + (params ? JSON.stringify(params) : '');
    const views = full.DockerManagerViews.create({ t, esc: String, model: full.DockerManagerModel, lang: 'en', uid: 'x' });
    for (const code of ['invalid_operation', 'operation_not_found', 'docker_error', 'host_not_found', 'invalid_action', 'internal_error', 'bad_request', 'project_conflict']) {
        assert.ok(views.errorText({ code, message: 'English' }).startsWith('error_' + code), code + ' has its own text');
    }
    assert.ok(views.errorText({ code: 'project_conflict', message: 'runs from /srv/shop' }).endsWith(' runs from /srv/shop'), 'the message of a project conflict names the other folder');
    assert.ok(views.errorText({ code: 'docker_error', message: 'cause' }).endsWith(' cause'), 'the message of a Docker error names the cause');
    assert.ok(!views.errorText({ code: 'internal_error', message: 'The deployment could not be started' }).includes('deployment'), 'the code decides, the English message of a deploy_error does not show');
    assert.equal(views.errorText({ code: 'brand_new', message: 'English' }), 'error_unknown English', 'an unknown code shows the generic sentence and the message');
}

// ---- translations: every key the code can ask for exists in all sixteen locales with the same placeholders -------------------
{
    const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
    const locales = Object.fromEntries(LANGS.map(lang => [lang, JSON.parse(read('ui/lang/desktop/' + lang + '.json'))]));
    const en = locales.en;
    const prefix = 'docker_manager.';
    const used = new Set();
    for (const file of uiFiles) {
        const source = read('ui/js/desktop/apps/' + file);
        for (const match of source.matchAll(/(?<![\w.])t\(\s*'([a-zA-Z0-9_]+)'/g)) used.add(prefix + match[1]);
        for (const match of source.matchAll(/\bdt\(\s*'(desktop\.[a-z_]+)'/g)) used.add(match[1]);
    }
    // Keys the code builds from a prefix, taken from the lists that drive them. The prefix-only matches above are removed.
    for (const key of [...used]) if (key.endsWith('_')) used.delete(key);
    const families = {
        status_: ['running', 'paused', 'restarting', 'exited_code', 'created', 'dead', 'removing', 'unhealthy', 'starting', 'unknown'],
        action_: [...model.ACTIONS, 'remove', 'logs', 'inspect', 'more', 'open_details', 'done', 'failed', 'deploy', 'update', 'pull', 'down', 'down_volumes', 'delete'],
        reason_: [...model.REASONS],
        error_: ['read_only', 'container_not_found', 'invalid_host', 'network', 'timeout', 'forbidden', 'unknown', 'host_not_prepared', 'api_too_old', 'stacks_unavailable', 'stack_name_invalid', 'stack_not_found', 'stack_exists',
            'operation_in_progress', 'file_too_large', 'runner_image_missing', 'stacks_dir_unavailable', 'convert_failed', 'revision_conflict',
            'invalid_operation', 'operation_not_found', 'docker_error', 'host_not_found', 'invalid_action', 'internal_error', 'bad_request', 'project_conflict',
            'stream_not_found', 'too_many_streams', 'websocket_required', 'container_not_running', 'helper_container', 'project_not_found',
            'forbidden_origin', 'docker_hosts_unavailable', 'docker_disabled'],
        filter_: [...model.STATE_FILTERS],
        kind_: ['local', ...model.HOST_KINDS],
        color_: [...model.HOST_COLORS],
        col_: ['status', 'name', 'image', 'ports', 'load', 'actions'],
        host_: ['ok', 'error', 'unknown'],
        protected_: ['owner', 'self', 'endpoint', 'network', 'unverified', 'helper'],
        tab_: [...full.DockerManagerViews.TABS],
        nest_method_: [...model.NEST_METHODS],
        v_: ['name', 'name_taken', 'name_reserved', 'address', 'address_scheme', 'port', 'ssh_user', 'ssh_secret', 'tls_cert', 'path'],
        stack_status_: [...sm.STACK_STATUSES],
        stab_: [...sm.STACK_TABS],
        age_: ['days', 'hours', 'minutes', 'seconds'],
        svc_state_: [...sm.SERVICE_STATES],
        svc_health_: ['healthy', 'unhealthy', 'starting', 'none'],
        op_kind_: [...sm.OP_KINDS],
        lint_: [...sm.LINT_CODES, 'sev_error', 'sev_warning', 'sev_hint'],
        op_state_: [...sm.OP_STATES],
        palette_: ['open', 'deploy', 'update', 'restart', 'stop', 'new_stack', 'prepare', 'switch_host', 'switch_all', 'refresh', 'logs', 'terminal'],
        sv_name_: ['empty', 'invalid', 'taken'],
        convert_: [...sm.CONVERT_WARNINGS],
        preview_kind_: [...sm.VOLUME_KINDS],
        preview_var_: ['set', 'default', 'required', 'missing'],
        logs_state_: ['idle', 'connecting', 'live', 'reconnecting', 'ended', 'error'],
        logs_notice_: ['not_followed', 'attached', 'detached', 'truncated_history'],
        term_status_: ['idle', 'connecting', 'connected', 'closed', 'error']
    };
    // Keys passed to t() through a conditional expression, which the literal scan above cannot see.
    for (const name of ['prepare_image_ok', 'prepare_image_missing', 'prepare_io_ok', 'prepare_io_mismatch', 'prepare_io_missing', 'ns_deployed', 'ns_saved', 'term_readonly', 'term_helper', 'term_not_running', 'term_none_running']) used.add(prefix + name);
    for (const [family, names] of Object.entries(families)) for (const name of names) used.add(prefix + family + name);
    used.add('desktop.app_docker_manager');
    assert.ok(used.size > 150, 'the key scan found only ' + used.size + ' keys');
    eq([...used].filter(key => !(typeof en[key] === 'string' && en[key].trim() !== '')).sort(), [], 'en.json is missing keys the code asks for');
    const tokens = value => [...String(value).matchAll(/\{\{(\w+)\}\}/g)].map(match => match[1]).sort().join(',');
    assert.ok(/anonymous/.test(en['docker_manager.down_volumes_body']), 'down --volumes also removes the anonymous volumes of the containers, and the copy must say so');
    const own = Object.keys(en).filter(key => key.startsWith(prefix));
    eq(own.filter(key => !used.has(key)).sort(), [], 'en.json has keys that no code asks for');
    for (const lang of LANGS) {
        const ownKeys = Object.keys(locales[lang]).filter(key => key.startsWith(prefix));
        eq(ownKeys.sort(), [...own].sort(), lang + '.json has a different docker_manager key set');
        for (const key of [...own, 'desktop.app_docker_manager']) {
            assert.ok(String(locales[lang][key] || '').trim() !== '', lang + '.json has an empty ' + key);
            assert.equal(tokens(locales[lang][key]), tokens(en[key]), lang + '.json changes the placeholders of ' + key);
        }
    }
}

console.log('docker-manager ui tests: stylesheet, load contract and translations ok');
