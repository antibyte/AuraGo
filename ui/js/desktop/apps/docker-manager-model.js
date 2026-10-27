/* Docker Manager model: pure helpers, no DOM and no network. A classic script (window.DockerManagerModel) so that
   scripts/test-docker-manager-ui.mjs can run it in a Node vm. It owns the container status and problem derivation,
   the grouping by compose project, search and state filters, the multi-host merge, the host chip data, the host form
   rules and the error normalisation that the store and the views share. */
(function () {
    'use strict';

    const HOST_COLORS = ['blue', 'teal', 'purple', 'coral', 'pink', 'amber', 'green', 'gray'];
    const HOST_KINDS = ['ssh', 'tls', 'mtls', 'tcp'];
    const DEFAULT_PORTS = { tcp: 2375, tls: 2376, mtls: 2376, ssh: 22 };
    const STATE_FILTERS = ['all', 'running', 'stopped', 'problem'];
    const ACTIONS = ['start', 'stop', 'restart', 'pause', 'unpause', 'kill'];
    const REASONS = ['dns', 'refused', 'timeout', 'tls', 'auth', 'ssh_host_key', 'socket_missing', 'api_too_old', 'unknown'];
    const NEST_METHODS = ['docker_local', 'docker_remote', 'docker_ssh'];
    const PROTECTED_ACTIONS = ['stop', 'restart', 'pause', 'kill', 'remove'];
    const DEFAULT_STACKS_DIR = '/opt/stacks';
    const DEFAULT_SOCKET = '/var/run/docker.sock';
    const TONE_RANK = { bad: 3, warn: 2, ok: 1, idle: 0 };

    const text = value => (value == null ? '' : String(value));
    const lower = value => text(value).toLowerCase();
    const array = value => (Array.isArray(value) ? value : []);
    const colorOf = value => (HOST_COLORS.includes(lower(value)) ? lower(value) : 'gray');
    const compareText = (a, b) => text(a).localeCompare(text(b), undefined, { numeric: true, sensitivity: 'base' });

    // ---- containers ------------------------------------------------------------------------------------------------

    // Docker reports the exit code only inside the status text: "Exited (137) 3 minutes ago".
    function exitCode(container) {
        const match = /^exited \((-?\d+)\)/i.exec(text(container && container.status).trim());
        return match ? Number(match[1]) : null;
    }

    const containerId = container => text(container && (container.full_id || container.id));

    function containerName(container) {
        const names = array(container && container.names);
        return text(names[0] || (container && container.id) || '').replace(/^\//, '');
    }

    // Why a container is protected, or null. The server decides what is protected; the UI only explains it.
    function protectedReason(container) {
        const c = container || {};
        if (c.protected_owner) return { kind: 'owner', owner: text(c.protected_owner) };
        if (c.self) return { kind: 'self', owner: '' };
        if (c.docker_endpoint) return { kind: 'endpoint', owner: '' };
        if (c.shared_network) return { kind: 'network', owner: '' };
        return null;
    }
    const isProtected = container => !!protectedReason(container);
    const needsProtectedConfirm = (container, action) => isProtected(container) && PROTECTED_ACTIONS.includes(action);

    // The single reason a container counts as a problem: unhealthy, restarting, dead, or exited with a non-zero code.
    function problemOf(container) {
        const c = container || {};
        const state = lower(c.state);
        if (lower(c.health) === 'unhealthy' && state === 'running') return 'unhealthy';
        if (state === 'restarting') return 'restarting';
        if (state === 'dead') return 'dead';
        if (state === 'exited') {
            const code = exitCode(c);
            if (code !== null && code !== 0) return 'exited';
        }
        return '';
    }

    // tone: ok (green), warn (amber), bad (red), idle (grey). A tone is always shown together with the id as text.
    function statusOf(container) {
        const c = container || {};
        const state = lower(c.state), health = lower(c.health), problem = problemOf(c);
        if (state === 'running') {
            if (health === 'unhealthy') return { id: 'unhealthy', tone: 'warn', problem };
            if (health === 'starting') return { id: 'starting', tone: 'ok', problem };
            return { id: 'running', tone: 'ok', problem };
        }
        if (state === 'paused') return { id: 'paused', tone: 'idle', problem };
        if (state === 'restarting') return { id: 'restarting', tone: 'warn', problem };
        if (state === 'exited') {
            const code = exitCode(c);
            return { id: 'exited', tone: problem ? 'bad' : 'idle', code: code === null ? 0 : code, problem };
        }
        if (state === 'dead') return { id: 'dead', tone: 'bad', problem };
        if (state === 'created') return { id: 'created', tone: 'idle', problem };
        if (state === 'removing') return { id: 'removing', tone: 'idle', problem };
        return { id: 'unknown', tone: 'idle', problem };
    }

    function stateGroup(container) {
        const state = lower(container && container.state);
        if (state === 'running') return 'running';
        if (state === 'exited' || state === 'created' || state === 'dead') return 'stopped';
        return 'other';
    }

    function matchesFilter(container, filter) {
        switch (filter) {
            case 'running': return lower(container && container.state) === 'running';
            case 'stopped': return stateGroup(container) === 'stopped';
            case 'problem': return problemOf(container) !== '';
            default: return true;
        }
    }

    // Which buttons a row shows. Nothing in a read-only window; protected containers keep their buttons and ask first.
    function actionsFor(container, readonly) {
        if (readonly) return { toggle: '', restart: false, menu: [], remove: false };
        const state = lower(container && container.state);
        const out = { toggle: '', restart: false, menu: [], remove: state !== 'removing' };
        if (state === 'running') { out.toggle = 'stop'; out.restart = true; out.menu = ['pause', 'kill']; }
        else if (state === 'paused') { out.toggle = 'unpause'; out.menu = ['kill']; }
        else if (state === 'restarting') { out.toggle = 'stop'; out.menu = ['kill']; }
        else if (state === 'exited' || state === 'created') out.toggle = 'start';
        return out;
    }

    // ---- ports, images, dates ----------------------------------------------------------------------------------------

    // Docker lists a published port once per address family; the table shows it once, published ports first.
    function portItems(ports) {
        const seen = new Set(), items = [];
        for (const port of array(ports)) {
            const type = text(port.type || 'tcp'), published = Number(port.public) || 0, internal = Number(port.private) || 0;
            const label = (published ? published + '→' : '') + internal + '/' + type;
            if (seen.has(label)) continue;
            seen.add(label);
            items.push({ label, published, internal, type });
        }
        items.sort((a, b) => (b.published ? 1 : 0) - (a.published ? 1 : 0) || (a.published || a.internal) - (b.published || b.internal));
        return items;
    }

    function shortImage(image) {
        const value = text(image);
        if (value.startsWith('sha256:')) return value.slice(0, 19);
        return value.replace(/^docker\.io\/library\//, '').replace(/^docker\.io\//, '');
    }

    function formatCreated(seconds, lang) {
        const n = Number(seconds);
        if (!n) return '';
        const date = new Date(n * 1000);
        try { return date.toLocaleString(lang || undefined, { dateStyle: 'medium', timeStyle: 'short' }); } catch (error) { return date.toISOString(); }
    }

    function mountLabel(mount) {
        const m = mount || {};
        const from = text(m.name || m.source);
        return from ? from + ' → ' + text(m.destination) : text(m.destination);
    }

    // ---- rows, filters, groups --------------------------------------------------------------------------------------

    // One row per container of the hosts in scope ('all' or a host id). The key is stable across polls.
    function buildRows(hosts, byHost, scope) {
        const rows = [];
        for (const host of array(hosts)) {
            if (scope && scope !== 'all' && host.id !== scope) continue;
            const entry = (byHost || {})[host.id];
            for (const c of array(entry && entry.containers)) {
                rows.push({ key: host.id + '/' + containerId(c), hostId: host.id, hostName: text(host.name), hostColor: colorOf(host.color), c });
            }
        }
        return rows;
    }

    function haystack(row) {
        const c = row.c;
        const ports = array(c.ports).map(port => [port.public, port.private].filter(Boolean).join(' ')).join(' ');
        return lower([containerName(c), array(c.names).join(' '), c.image, c.id, c.full_id, c.compose_project, c.compose_service, c.state, row.hostName, ports].join(' '));
    }

    // Every word of the query must appear somewhere in the row (name, image, project, service, id, host, ports, state).
    function matchesQuery(row, query) {
        const terms = lower(query).split(/\s+/).filter(Boolean);
        if (!terms.length) return true;
        const hay = haystack(row);
        return terms.every(term => hay.includes(term));
    }

    function filterRows(rows, options) {
        const o = options || {};
        return array(rows).filter(row => matchesQuery(row, o.query) && matchesFilter(row.c, o.filter));
    }

    // The chip counts follow the search but not the state filter, so a chip always shows what it would give.
    function counts(rows, query) {
        const out = { all: 0, running: 0, stopped: 0, problem: 0 };
        for (const row of array(rows)) {
            if (!matchesQuery(row, query)) continue;
            out.all += 1;
            if (matchesFilter(row.c, 'running')) out.running += 1;
            if (matchesFilter(row.c, 'stopped')) out.stopped += 1;
            if (matchesFilter(row.c, 'problem')) out.problem += 1;
        }
        return out;
    }

    function groupCounts(rows) {
        let running = 0, problem = 0;
        for (const row of rows) {
            if (lower(row.c.state) === 'running') running += 1;
            if (problemOf(row.c)) problem += 1;
        }
        return { total: rows.length, running, problem };
    }

    // A project is green when every container runs, grey when none does, amber otherwise; a failed container makes it red.
    function groupTone(rows) {
        const tones = rows.map(row => statusOf(row.c).tone);
        if (tones.includes('bad')) return 'bad';
        if (tones.includes('warn')) return 'warn';
        const running = tones.filter(tone => tone === 'ok').length;
        if (running === rows.length) return 'ok';
        return running === 0 ? 'idle' : 'warn';
    }

    // Compose projects first (by name, then host order), then the containers without a project. In the multi-host view the
    // same project name on two hosts stays two groups; the single containers of all hosts share one group.
    function groupRows(rows, options) {
        const o = options || {};
        const multi = !!o.multiHost, hostOrder = array(o.hostOrder);
        const groups = new Map();
        for (const row of array(rows)) {
            const project = text(row.c.compose_project);
            const key = project ? 'p:' + (multi ? row.hostId + '|' : '') + project : 's:' + (multi ? '' : row.hostId);
            let group = groups.get(key);
            if (!group) {
                group = { key, kind: project ? 'project' : 'single', project, hostId: project ? row.hostId : '', hostName: project ? row.hostName : '', hostColor: project ? row.hostColor : '', rows: [] };
                groups.set(key, group);
            }
            group.rows.push(row);
        }
        const list = [...groups.values()];
        for (const group of list) {
            group.rows.sort((a, b) => compareText(a.c.compose_service || containerName(a.c), b.c.compose_service || containerName(b.c))
                || compareText(containerName(a.c), containerName(b.c)) || compareText(a.hostName, b.hostName));
            group.counts = groupCounts(group.rows);
            group.tone = group.kind === 'project' ? groupTone(group.rows) : 'idle';
        }
        const rank = id => { const index = hostOrder.indexOf(id); return index < 0 ? 999 : index; };
        list.sort((a, b) => (a.kind === b.kind ? 0 : a.kind === 'project' ? -1 : 1) || compareText(a.project, b.project) || rank(a.hostId) - rank(b.hostId));
        return list;
    }

    // A lone "single containers" group needs no header row.
    const showGroupHeaders = groups => !(array(groups).length === 1 && groups[0].kind === 'single');

    // The rows the keyboard can reach, in display order. A collapsed group hides its rows.
    function navItems(groups, collapsed) {
        const items = [], headers = showGroupHeaders(groups);
        for (const group of array(groups)) {
            if (headers) items.push({ type: 'group', key: 'g:' + group.key, group: group.key });
            if (headers && collapsed && collapsed.has(group.key)) continue;
            for (const row of group.rows) items.push({ type: 'row', key: row.key, group: group.key });
        }
        return items;
    }

    // ---- hosts -----------------------------------------------------------------------------------------------------

    const reasonCode = code => (REASONS.includes(code) ? code : 'unknown');

    // The server sends "0001-01-01T00:00:00Z" as checked_at for a host that was never tested. That means "not checked yet",
    // never a date, whatever else the status says.
    function normalizeStatus(status) {
        const s = status && typeof status === 'object' ? status : {};
        return text(s.checked_at).startsWith('0001-') ? Object.assign({}, s, { state: 'unknown', code: '' }) : s;
    }

    // The tone and the reason shown for a host. A failed container load beats an older "ok" from the host list.
    function hostHealth(host, entry) {
        const status = normalizeStatus(host && host.status);
        const failed = entry && entry.error;
        if (failed && failed.code === 'host_unreachable') return { tone: 'bad', state: 'error', reason: reasonCode(failed.reason || status.code), message: text(failed.message) };
        if (failed) return { tone: 'warn', state: 'error', reason: 'unknown', message: text(failed.message) };
        if (status.state === 'error') return { tone: 'bad', state: 'error', reason: reasonCode(status.code), message: text(status.message) };
        if (status.state === 'ok' || (entry && entry.loaded)) return { tone: 'ok', state: 'ok', reason: '', message: '' };
        return { tone: 'idle', state: 'unknown', reason: '', message: '' };
    }

    function hostAddress(host) {
        const h = host || {};
        const address = text(h.address);
        if (h.kind === 'local') return address;
        const port = Number(h.port) || DEFAULT_PORTS[h.kind] || 0;
        const bare = address.startsWith('[') || !address.includes(':') ? address : '[' + address + ']';
        return port ? bare + ':' + port : bare;
    }

    function hostCounts(hostId, byHost) {
        const list = array(byHost && byHost[hostId] && byHost[hostId].containers);
        return { running: list.filter(c => lower(c.state) === 'running').length, total: list.length };
    }

    function totalCounts(hosts, byHost) {
        const out = { running: 0, total: 0 };
        for (const host of array(hosts)) {
            const c = hostCounts(host.id, byHost);
            out.running += c.running;
            out.total += c.total;
        }
        return out;
    }

    const defaultPort = kind => DEFAULT_PORTS[kind] || 0;
    const nextColor = hosts => HOST_COLORS.find(color => !array(hosts).some(host => host.color === color)) || HOST_COLORS[array(hosts).length % HOST_COLORS.length];

    // ---- host form ---------------------------------------------------------------------------------------------------

    function emptyHostForm(hosts) {
        return {
            id: '', name: '', kind: 'ssh', originalKind: '', originalAddress: '', originalPort: '', originalSshUser: '',
            address: '', port: '', sshUser: '', sshSecret: '', tlsCA: '', tlsCert: '', tlsKey: '',
            clearSecret: false, stacksDir: DEFAULT_STACKS_DIR, socketPath: DEFAULT_SOCKET, color: nextColor(hosts), sort: 0, secretSet: false
        };
    }

    function hostToForm(host) {
        const h = host || {};
        return Object.assign(emptyHostForm([]), {
            id: text(h.id), name: text(h.name), kind: text(h.kind || 'ssh'), originalKind: text(h.kind), address: text(h.address), port: h.port ? String(h.port) : '',
            originalAddress: text(h.address), originalPort: h.port ? String(h.port) : '', originalSshUser: text(h.ssh_user),
            sshUser: text(h.ssh_user), stacksDir: text(h.stacks_dir) || DEFAULT_STACKS_DIR, socketPath: text(h.host_socket_path) || DEFAULT_SOCKET,
            color: colorOf(h.color), sort: Number(h.sort) || 0, secretSet: !!h.secret_set
        });
    }

    const secretFamily = kind => (kind === 'ssh' ? 'ssh' : kind === 'tls' || kind === 'mtls' ? 'tls' : '');

    // The server's comparison of an SSH endpoint: address trimmed, case-insensitive, without IPv6 brackets; an empty port
    // means 22; the user trimmed.
    const addressKey = value => lower(text(value).trim()).replace(/^\[|\]$/g, '');
    const sshPort = value => Number(text(value).trim()) || DEFAULT_PORTS.ssh;
    function sshEndpointChanged(form) {
        const f = form || {};
        return addressKey(f.address) !== addressKey(f.originalAddress) || sshPort(f.port) !== sshPort(f.originalPort)
            || text(f.sshUser).trim() !== text(f.originalSshUser).trim();
    }

    // True when an SSH host with a stored secret now points at another address, port or user: the server then refuses to
    // reuse the secret (invalid_host, field ssh_secret), so the form asks for it again up front.
    const sshSecretNeedsReentry = form => {
        const f = form || {};
        return !!f.id && !!f.secretSet && !f.clearSecret && text(f.kind) === 'ssh' && text(f.originalKind) === 'ssh' && sshEndpointChanged(f);
    };

    // A stored secret can stay only for the same kind of secret, only when it is not being removed, and an SSH secret only
    // for the same endpoint. TLS material keeps working whatever else is edited.
    function storedSecretUsable(form) {
        const f = form || {};
        if (!f.id || !f.secretSet || f.clearSecret || secretFamily(text(f.kind)) !== secretFamily(text(f.originalKind))) return false;
        return text(f.kind) !== 'ssh' || !sshEndpointChanged(f);
    }

    // Returns null or {field, code}; code names the docker_manager.v_<code> message. `hosts` is the current host list: names
    // are unique ignoring case and "Local" is reserved. The server validates again and has the last word.
    function validateHostForm(form, hosts) {
        const f = form || {};
        const name = text(f.name).trim(), kind = text(f.kind), address = text(f.address).trim(), port = text(f.port).trim();
        if (!name || [...name].length > 64) return { field: 'name', code: 'name' };
        if (lower(name) === 'local') return { field: 'name', code: 'name_reserved' };
        if (array(hosts).some(item => item.id !== f.id && lower(item.name) === lower(name))) return { field: 'name', code: 'name_taken' };
        if (!address) return { field: 'address', code: 'address' };
        if (/:\/\/|[\/\\\s]/.test(address)) return { field: 'address', code: 'address_scheme' };
        if (port !== '' && (!/^\d+$/.test(port) || Number(port) < 1 || Number(port) > 65535)) return { field: 'port', code: 'port' };
        if (kind === 'ssh') {
            if (!text(f.sshUser).trim()) return { field: 'sshUser', code: 'ssh_user' };
            if (text(f.sshSecret) === '' && !storedSecretUsable(f)) return { field: 'sshSecret', code: 'ssh_secret' };
        }
        // The server merges the TLS fields one by one, so an edit may send any subset; a new host needs the whole pair.
        if (kind === 'mtls' && !(text(f.tlsCert).trim() && text(f.tlsKey).trim()) && !storedSecretUsable(f)) return { field: 'tlsCert', code: 'tls_cert' };
        for (const [field, value] of [['stacksDir', f.stacksDir], ['socketPath', f.socketPath]]) {
            const path = text(value).trim();
            if (path && (!path.startsWith('/') || path.split('/').includes('..'))) return { field, code: 'path' };
        }
        return null;
    }

    // The HostWrite body. Secrets are sent only when typed; an empty field keeps what the server stores.
    function hostPayload(form) {
        const f = form || {}, kind = text(f.kind);
        const payload = {
            name: text(f.name).trim(), kind, address: text(f.address).trim(), port: Number(text(f.port).trim()) || 0,
            stacks_dir: text(f.stacksDir).trim(), host_socket_path: text(f.socketPath).trim(),
            color: colorOf(f.color), ssh_user: kind === 'ssh' ? text(f.sshUser).trim() : '', sort: Number(f.sort) || 0
        };
        if (kind === 'ssh' && text(f.sshSecret) !== '') payload.ssh_secret = text(f.sshSecret);
        // Only the PEM fields that were typed are sent; the server keeps the stored value of every field that is absent.
        const tls = {};
        for (const [key, value] of [['ca', f.tlsCA], ['cert', f.tlsCert], ['key', f.tlsKey]]) if (text(value).trim()) tls[key] = text(value).trim();
        if ((kind === 'tls' || kind === 'mtls') && Object.keys(tls).length) payload.tls = tls;
        if (f.clearSecret) payload.clear_secret = true;
        return payload;
    }

    // ---- errors ------------------------------------------------------------------------------------------------------

    // One shape for every failure: {code, message, reason, owner, field, status}. The desktop api() throws an Error that
    // carries the response body and status; a failed fetch carries neither.
    function normalizeError(error) {
        const e = error || {};
        const body = e.body && typeof e.body === 'object' ? e.body : {};
        const status = Number(e.status) || 0;
        if (e.name === 'AbortError' || e.code === 'aborted') return { code: 'aborted', message: '', reason: '', owner: '', field: '', status };
        let code = text(body.code || e.code);
        if (!code) code = status === 403 ? 'forbidden' : status === 0 ? 'network' : 'unknown';
        return { code, message: text(body.message || e.message), reason: text(body.reason), owner: text(body.owner), field: text(body.field), status };
    }

    window.DockerManagerModel = {
        HOST_COLORS, HOST_KINDS, DEFAULT_PORTS, STATE_FILTERS, ACTIONS, REASONS, NEST_METHODS, DEFAULT_STACKS_DIR, DEFAULT_SOCKET, TONE_RANK,
        colorOf, compareText, exitCode, containerId, containerName, protectedReason, isProtected, needsProtectedConfirm, problemOf, statusOf, stateGroup,
        matchesFilter, actionsFor, portItems, shortImage, formatCreated, mountLabel, buildRows, matchesQuery, filterRows, counts, groupRows,
        showGroupHeaders, navItems, reasonCode, normalizeStatus, hostHealth, hostAddress, hostCounts, totalCounts, defaultPort, nextColor, emptyHostForm,
        hostToForm, storedSecretUsable, sshSecretNeedsReentry, validateHostForm, hostPayload, normalizeError
    };
})();
