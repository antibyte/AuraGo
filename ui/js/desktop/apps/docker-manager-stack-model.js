/* Docker Manager stack model: pure helpers for the Stacks section, no DOM and no network. A classic script
   (window.DockerManagerStackModel) so that scripts/test-docker-manager-ui.mjs can run it in a Node vm. It owns the stack
   status and tone, the rows and groups of the stack list, the compose lint helpers, the line diff of the conflict dialog,
   the command palette ranking, the random .env value and the reducer that folds an operation's stream events into one
   entry. Everything that needs the host list, the translator or the clock takes it as an argument. */
(function () {
    'use strict';

    const SECTIONS = ['stacks', 'containers'];
    const STACK_TABS = ['overview', 'compose', 'env', 'logs', 'terminal'];
    const PROJECT_TABS = ['overview', 'logs'];
    const STACK_STATUSES = ['running', 'partial', 'stopped', 'error', 'draft'];
    const STACK_TONE = { running: 'ok', partial: 'warn', error: 'bad', stopped: 'idle', draft: 'idle' };
    const OP_KINDS = ['prepare', 'config', 'up', 'down', 'stop', 'restart', 'pull', 'update', 'delete'];
    const OP_STATES = ['queued', 'running', 'health_wait', 'succeeded', 'failed', 'interrupted'];
    const ACTIVE_STATES = ['queued', 'running', 'health_wait'];
    const OP_TONE = { queued: 'idle', running: 'info', health_wait: 'info', succeeded: 'ok', failed: 'bad', interrupted: 'warn' };
    const SERVICE_STATES = ['running', 'exited', 'created', 'restarting', 'paused', 'dead', 'missing'];
    const LINT_CODES = ['yaml_syntax', 'not_a_mapping', 'no_services', 'service_no_image', 'invalid_port', 'port_duplicate', 'port_conflict',
        'variable_missing', 'image_latest', 'image_no_tag', 'privileged', 'docker_socket', 'network_mode_host', 'restart_missing', 'cap_add', 'env_syntax'];
    const SEVERITY_RANK = { error: 3, warning: 2, hint: 1 };
    const VOLUME_KINDS = ['named', 'bind', 'relative', 'anonymous', 'tmpfs'];
    const CONVERT_WARNINGS = ['flag_unsupported', 'flag_ignored', 'flag_value_invalid'];
    const NAME_PATTERN = /^[a-z0-9][a-z0-9_-]{0,62}$/;
    const OUTPUT_CAP = 64 * 1024;
    const DIFF_CELL_LIMIT = 4000000;
    const ENV_NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;
    const ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';

    const text = value => (value == null ? '' : String(value));
    const lower = value => text(value).toLowerCase();
    const array = value => (Array.isArray(value) ? value : []);
    const objects = value => array(value).filter(item => item && typeof item === 'object');
    const compareText = (a, b) => text(a).localeCompare(text(b), undefined, { numeric: true, sensitivity: 'base' });

    // ---- names -------------------------------------------------------------------------------------------------------

    // Mirrors dockerstacks.ValidateName: ^[a-z0-9][a-z0-9_-]{0,62}$. `taken` lists the stack names already on the host.
    function validateName(name, taken) {
        const value = text(name);
        if (!value) return { code: 'empty' };
        if (!NAME_PATTERN.test(value)) return { code: 'invalid' };
        if (array(taken).includes(value)) return { code: 'taken' };
        return null;
    }

    // A folder-safe suggestion from a service name or an image: lower case, other characters become "-".
    function sanitizeName(value) {
        const cleaned = lower(value).replace(/[^a-z0-9_-]+/g, '-').replace(/^[^a-z0-9]+/, '').replace(/-+$/, '');
        return cleaned.slice(0, 63);
    }

    // The first key under `services:` of a pasted compose file, or ''.
    function firstServiceName(compose) {
        const match = /^services:[ \t]*(?:#.*)?\r?\n(?:[ \t]*(?:#.*)?\r?\n)*[ \t]+([A-Za-z0-9][A-Za-z0-9_.-]*)[ \t]*:/m.exec(text(compose));
        return match ? sanitizeName(match[1]) : '';
    }

    // ---- stack status, rows and groups -------------------------------------------------------------------------------

    const stackStatusId = stack => (STACK_STATUSES.includes(stack && stack.status) ? stack.status : 'stopped');
    const stackTone = stack => STACK_TONE[stackStatusId(stack)];

    function stackRows(hosts, byHost, scope) {
        const rows = [];
        for (const host of objects(hosts)) {
            if (scope && scope !== 'all' && host.id !== scope) continue;
            const listing = byHost && byHost[host.id] && byHost[host.id].listing;
            for (const stack of objects(listing && listing.stacks)) {
                rows.push({ key: host.id + '/' + stack.name, hostId: host.id, hostName: text(host.name), hostColor: host.color, stack });
            }
        }
        return rows;
    }

    function externalRows(hosts, byHost, scope) {
        const rows = [];
        for (const host of objects(hosts)) {
            if (scope && scope !== 'all' && host.id !== scope) continue;
            const listing = byHost && byHost[host.id] && byHost[host.id].listing;
            for (const project of objects(listing && listing.external)) {
                rows.push({ key: 'x:' + host.id + '/' + project.name, hostId: host.id, hostName: text(host.name), hostColor: host.color, project });
            }
        }
        return rows;
    }

    // Containers without a compose project (helper containers included: the Containers view shows them with a lock).
    const singleContainerRows = containerRows => objects(containerRows).filter(row => row.c && !text(row.c.compose_project));

    function stackHaystack(row) {
        const s = row.stack || row.project || {};
        const ports = objects(s.ports).map(port => port.public_port).filter(Boolean).join(' ');
        return lower([s.name, row.hostName, s.status, s.working_dir, s.dir, ports].join(' '));
    }

    function matchesStackQuery(row, query) {
        const terms = lower(query).split(/\s+/).filter(Boolean);
        if (!terms.length) return true;
        const hay = stackHaystack(row);
        return terms.every(term => hay.includes(term));
    }

    const byNameThenHost = hostOrder => (a, b) => {
        const nameA = (a.stack || a.project).name, nameB = (b.stack || b.project).name;
        const rank = id => { const index = array(hostOrder).indexOf(id); return index < 0 ? 999 : index; };
        return compareText(nameA, nameB) || rank(a.hostId) - rank(b.hostId);
    };

    // The three groups of the stack list, filtered by the search words. `containerRows` are the rows of model.buildRows.
    function stackGroups(hosts, byHost, containerRows, options) {
        const o = options || {}, order = objects(hosts).map(host => host.id);
        const sorter = byNameThenHost(order);
        const stacks = stackRows(hosts, byHost, o.scope).filter(row => matchesStackQuery(row, o.query)).sort(sorter);
        const external = externalRows(hosts, byHost, o.scope).filter(row => matchesStackQuery(row, o.query)).sort(sorter);
        const singles = singleContainerRows(containerRows).filter(row => {
            const c = row.c;
            const hay = lower([c.names && c.names[0], c.image, c.id, row.hostName, c.state].join(' '));
            return lower(o.query).split(/\s+/).filter(Boolean).every(term => hay.includes(term));
        });
        singles.sort((a, b) => compareText(a.c.names && a.c.names[0], b.c.names && b.c.names[0]) || compareText(a.hostName, b.hostName));
        return { stacks, external, singles };
    }

    // What one host contributes to the stack list: ok | loading | not_prepared | api_too_old | unreachable | unavailable | error.
    function hostStacksState(entry) {
        if (!entry) return 'loading';
        const code = entry.error && entry.error.code;
        if (code === 'host_not_prepared') return 'not_prepared';
        if (code === 'api_too_old') return 'api_too_old';
        if (code === 'host_unreachable') return 'unreachable';
        if (code === 'stacks_unavailable' || code === 'stacks_dir_unavailable') return 'unavailable';
        if (entry.error && !entry.loaded) return 'error';
        return entry.loaded ? 'ok' : 'loading';
    }

    function stackCounts(hosts, byHost, scope) {
        const rows = stackRows(hosts, byHost, scope), external = externalRows(hosts, byHost, scope);
        return { stacks: rows.length, external: external.length, total: rows.length + external.length };
    }

    // The accent button of a stack: Deploy while nothing (or only part) runs, Update for a running stack.
    function primaryAction(stack) {
        const status = stackStatusId(stack);
        return status === 'running' ? 'update' : 'deploy';
    }

    // Which operations the detail header and the palette offer for a status. `deploy` runs kind up.
    function stackActions(stack) {
        const status = stackStatusId(stack), up = status === 'running' || status === 'partial';
        return { primary: primaryAction(stack), restart: up, stop: up, pull: status !== 'draft', down: status !== 'draft', remove: true };
    }

    const OP_FOR_ACTION = { deploy: 'up', update: 'update', restart: 'restart', stop: 'stop', pull: 'pull', down: 'down' };

    // ---- services, ports, time ---------------------------------------------------------------------------------------

    function serviceTone(service) {
        const s = service || {}, state = lower(s.state), health = lower(s.health);
        if (state === 'running') return health === 'unhealthy' ? 'warn' : 'ok';
        if (state === 'restarting') return 'warn';
        if (state === 'dead') return 'bad';
        if (state === 'exited') return Number(s.exit_code) > 0 ? 'bad' : 'idle';
        return 'idle';
    }
    const serviceStateId = service => (SERVICE_STATES.includes(lower(service && service.state)) ? lower(service.state) : 'missing');

    const isLoopback = ip => ip === '127.0.0.1' || ip === '::1' || /^127\./.test(ip);

    // The address a browser can use for a host: the page's own host name for the local engine, else the host's address
    // without port and brackets.
    function linkAddress(host, pageHost) {
        const h = host || {};
        if (h.kind === 'local') return text(pageHost);
        let address = text(h.address).trim().replace(/^\[|\]$/g, '');
        if (/^[^:]+:\d+$/.test(address)) address = address.slice(0, address.lastIndexOf(':'));
        return address;
    }

    // One entry per published port (IPv4 and IPv6 bindings of the same port show once). `url` is empty for UDP and for
    // ports bound to the loopback of the engine host, which a browser elsewhere cannot reach.
    function portLinks(ports, address) {
        const seen = new Set(), out = [];
        const bare = text(address).includes(':') && !text(address).startsWith('[') ? '[' + address + ']' : text(address);
        for (const port of objects(ports)) {
            const published = Number(port.public_port) || 0;
            if (!published) continue;
            const protocol = text(port.protocol || 'tcp'), id = published + '/' + protocol + '/' + text(port.service);
            if (seen.has(id)) continue;
            seen.add(id);
            const reachable = protocol === 'tcp' && !isLoopback(text(port.ip)) && bare !== '';
            out.push({ key: id, label: String(published), service: text(port.service), protocol, container: Number(port.private_port) || 0, url: reachable ? 'http://' + bare + ':' + published : '' });
        }
        return out.sort((a, b) => Number(a.label) - Number(b.label));
    }

    // {unit, n}: the largest whole unit of the age of `createdSeconds` at `nowMs`; null for a draft (no containers).
    function age(createdSeconds, nowMs) {
        const created = Number(createdSeconds);
        if (!created) return null;
        const seconds = Math.max(0, Math.floor((Number(nowMs) || 0) / 1000 - created));
        if (seconds >= 86400) return { unit: 'days', n: Math.floor(seconds / 86400) };
        if (seconds >= 3600) return { unit: 'hours', n: Math.floor(seconds / 3600) };
        if (seconds >= 60) return { unit: 'minutes', n: Math.floor(seconds / 60) };
        return { unit: 'seconds', n: seconds };
    }

    const pad = n => String(n).padStart(2, '0');
    // 1536 -> "1.5 KB"; a negative or unknown size is ''.
    function formatBytes(bytes) {
        let value = Number(bytes);
        if (!(value >= 0)) return '';
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        let unit = 0;
        while (value >= 1024 && unit < units.length - 1) { value /= 1024; unit += 1; }
        return (unit === 0 || value >= 10 ? String(Math.round(value)) : value.toFixed(1)) + ' ' + units[unit];
    }

    // m:ss, or h:mm:ss from one hour on.
    function clock(seconds) {
        const s = Math.max(0, Math.floor(Number(seconds) || 0));
        return s >= 3600 ? Math.floor(s / 3600) + ':' + pad(Math.floor(s / 60) % 60) + ':' + pad(s % 60) : Math.floor(s / 60) + ':' + pad(s % 60);
    }
    function elapsed(op, nowMs) {
        const o = op || {}, start = Date.parse(o.started_at || o.created_at || '');
        if (!Number.isFinite(start)) return '';
        const end = o.finished_at ? Date.parse(o.finished_at) : Number(nowMs) || Date.now();
        return clock(((Number.isFinite(end) ? end : start) - start) / 1000);
    }

    // ---- compose lint ------------------------------------------------------------------------------------------------

    // The file of a diagnostic is decided by its code: env_syntax belongs to .env (the server also sets params.file to ".env"),
    // everything else to the compose file.
    const diagnosticFile = d => (d && d.code === 'env_syntax' ? 'env' : 'compose');
    const severityOf = d => (SEVERITY_RANK[d && d.severity] ? d.severity : 'hint');

    function sortDiagnostics(list) {
        return objects(list).slice().sort((a, b) => (Number(a.line) || 0) - (Number(b.line) || 0) || SEVERITY_RANK[severityOf(b)] - SEVERITY_RANK[severityOf(a)]);
    }
    function splitDiagnostics(list) {
        const out = { compose: [], env: [] };
        for (const d of sortDiagnostics(list)) out[diagnosticFile(d)].push(d);
        return out;
    }
    function lintCounts(list) {
        const out = { error: 0, warning: 0, hint: 0 };
        for (const d of objects(list)) out[severityOf(d)] += 1;
        return out;
    }
    function worstSeverity(list) {
        const counts = lintCounts(list);
        return counts.error ? 'error' : counts.warning ? 'warning' : counts.hint ? 'hint' : '';
    }

    // The localized sentence of a diagnostic. Unknown codes and texts whose placeholders the server did not fill keep the
    // English message of the server.
    function diagnosticText(d, t) {
        if (!d) return '';
        if (!LINT_CODES.includes(d.code)) return text(d.message || d.code);
        const params = Object.assign(d.service ? { service: d.service } : {}, d.params || {});
        const out = t('lint_' + d.code, params);
        return /\{\{\w+\}\}/.test(out) ? text(d.message || out) : out;
    }

    // Offsets of every line start of `source`, for mapping diagnostics onto a plain textarea.
    function lineIndex(source) {
        const value = text(source), starts = [0];
        for (let i = 0; i < value.length; i += 1) if (value.charCodeAt(i) === 10) starts.push(i + 1);
        return { source: value, starts };
    }
    // {from, to} of a 1-based line (0 or out of range = the first or last line); a column starts the range there, else it
    // starts at the first non-blank character.
    function lineRange(index, line, column) {
        const count = index.starts.length, n = Math.min(Math.max(Number(line) || 1, 1), count);
        const from = index.starts[n - 1];
        let to = n < count ? index.starts[n] - 1 : index.source.length;
        if (index.source.charCodeAt(to - 1) === 13) to -= 1;
        const rowText = index.source.slice(from, to);
        const offset = Number(column) > 0 ? Number(column) - 1 : rowText.length - rowText.trimStart().length;
        return { line: n, from: Math.min(from + offset, to), to };
    }

    // The compose-side facts the new-stack preview shows.
    function storageGroups(preview) {
        const out = { named: [], bind: [], relative: [], anonymous: [], tmpfs: [] };
        for (const service of objects(preview && preview.services)) {
            for (const volume of objects(service.volumes)) {
                if (out[volume.kind]) out[volume.kind].push({ service: service.name, source: text(volume.source), target: text(volume.target), readOnly: !!volume.read_only });
            }
        }
        return out;
    }
    const randomFixes = list => [...new Set(objects(list).filter(d => d.fix && d.fix.kind === 'env_random' && ENV_NAME.test(text(d.fix.variable))).map(d => d.fix.variable))];

    // ---- .env --------------------------------------------------------------------------------------------------------

    const escapeRe = value => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    function envHas(envText, name) {
        return ENV_NAME.test(text(name)) && new RegExp('^[ \\t]*(?:export[ \\t]+)?' + escapeRe(text(name)) + '[ \\t]*=', 'm').test(text(envText));
    }
    // Adds NAME=value on its own line; an existing definition is never overwritten.
    function appendEnvVariable(envText, name, value) {
        const base = text(envText);
        if (!ENV_NAME.test(text(name)) || envHas(base, name)) return base;
        return base + (base === '' || base.endsWith('\n') ? '' : '\n') + name + '=' + text(value) + '\n';
    }
    // A random value of letters and digits (safe unquoted in an .env file). `source` is a crypto-like object.
    function randomValue(length, source) {
        const wanted = Math.max(1, Math.min(Number(length) || 32, 128)), out = [];
        while (out.length < wanted) {
            const bytes = source.getRandomValues(new Uint8Array(wanted * 2));
            for (const byte of bytes) {
                if (byte < 248 && out.length < wanted) out.push(ALPHABET[byte % 62]);
            }
        }
        return out.join('');
    }

    // ---- dropped files -----------------------------------------------------------------------------------------------

    // 'compose' for compose.yaml, docker-compose.yml and any other .yml/.yaml; 'env' for .env and *.env; else ''.
    function classifyDroppedFile(name) {
        const file = lower(name).split(/[\\/]/).pop();
        if (file === '.env' || file.endsWith('.env')) return 'env';
        if (/\.ya?ml$/.test(file)) return 'compose';
        return '';
    }

    // ---- line diff ---------------------------------------------------------------------------------------------------

    function splitLines(value) {
        const source = text(value).replace(/\r\n/g, '\n');
        if (source === '') return [];
        const lines = source.split('\n');
        if (lines[lines.length - 1] === '') lines.pop();
        return lines;
    }

    // Rows {type: same|add|del, text, oldNo, newNo}. A common start and end are cut off first; the middle uses a longest
    // common subsequence, or a plain "all removed, all added" when it would be too large to compute.
    function lineDiff(before, after) {
        const a = splitLines(before), b = splitLines(after);
        let start = 0;
        while (start < a.length && start < b.length && a[start] === b[start]) start += 1;
        let endA = a.length, endB = b.length;
        while (endA > start && endB > start && a[endA - 1] === b[endB - 1]) { endA -= 1; endB -= 1; }
        const rows = [];
        for (let i = 0; i < start; i += 1) rows.push({ type: 'same', text: a[i], oldNo: i + 1, newNo: i + 1 });
        const midA = a.slice(start, endA), midB = b.slice(start, endB), n = midA.length, m = midB.length;
        let oldNo = start + 1, newNo = start + 1;
        const del = line => rows.push({ type: 'del', text: line, oldNo: oldNo++, newNo: 0 });
        const add = line => rows.push({ type: 'add', text: line, oldNo: 0, newNo: newNo++ });
        if (n * m > DIFF_CELL_LIMIT) {
            midA.forEach(del);
            midB.forEach(add);
        } else {
            const table = Array.from({ length: n + 1 }, () => new Uint32Array(m + 1));
            for (let i = n - 1; i >= 0; i -= 1) for (let j = m - 1; j >= 0; j -= 1) {
                table[i][j] = midA[i] === midB[j] ? table[i + 1][j + 1] + 1 : Math.max(table[i + 1][j], table[i][j + 1]);
            }
            let i = 0, j = 0;
            while (i < n && j < m) {
                if (midA[i] === midB[j]) { rows.push({ type: 'same', text: midA[i], oldNo: oldNo++, newNo: newNo++ }); i += 1; j += 1; }
                else if (table[i + 1][j] >= table[i][j + 1]) { del(midA[i]); i += 1; }
                else { add(midB[j]); j += 1; }
            }
            while (i < n) { del(midA[i]); i += 1; }
            while (j < m) { add(midB[j]); j += 1; }
        }
        for (let k = endA; k < a.length; k += 1) rows.push({ type: 'same', text: a[k], oldNo: oldNo++, newNo: newNo++ });
        return rows;
    }

    const diffChanged = rows => objects(rows).some(row => row.type !== 'same');

    // Long unchanged runs become one {type: 'skip', count} row; `context` lines stay around every change.
    function collapseDiff(rows, context) {
        const keep = new Array(rows.length).fill(false), c = Math.max(0, context == null ? 3 : context);
        rows.forEach((row, index) => {
            if (row.type === 'same') return;
            for (let k = Math.max(0, index - c); k <= Math.min(rows.length - 1, index + c); k += 1) keep[k] = true;
        });
        const out = [];
        let skipped = 0;
        rows.forEach((row, index) => {
            if (keep[index]) {
                if (skipped) { out.push({ type: 'skip', count: skipped }); skipped = 0; }
                out.push(row);
            } else skipped += 1;
        });
        if (skipped) out.push({ type: 'skip', count: skipped });
        return out;
    }

    // ---- command palette ---------------------------------------------------------------------------------------------

    // Every word must appear in the label (a hit at the start of a word scores more); a word that is not a substring may
    // still match as a subsequence of at least two letters. 0 means no match.
    function fuzzyScore(label, query) {
        const words = lower(query).split(/\s+/).filter(Boolean), hay = lower(label);
        if (!words.length) return 1;
        let score = 0;
        for (const word of words) {
            const at = hay.indexOf(word);
            if (at >= 0) { score += at === 0 ? 3 : /[\s/_-]/.test(hay[at - 1]) ? 2 : 1; continue; }
            let k = 0;
            for (const ch of hay) if (ch === word[k]) k += 1;
            if (word.length >= 2 && k === word.length) score += 0.5; else return 0;
        }
        return score;
    }

    function rankItems(items, query, limit) {
        const ranked = array(items).map((item, order) => ({ item, order, score: fuzzyScore(item.label, query) })).filter(entry => entry.score > 0);
        ranked.sort((a, b) => b.score - a.score || a.order - b.order);
        return ranked.slice(0, limit || 50).map(entry => entry.item);
    }

    // Segments {text, hit} of a label with the query words marked (substring hits only).
    function highlight(label, query) {
        const value = text(label), hay = lower(value), marks = new Array(value.length).fill(false);
        for (const word of lower(query).split(/\s+/).filter(Boolean)) {
            const at = hay.indexOf(word);
            if (at >= 0) for (let k = at; k < at + word.length; k += 1) marks[k] = true;
        }
        const out = [];
        for (let k = 0; k < value.length; k += 1) {
            const last = out[out.length - 1];
            if (last && last.hit === marks[k]) last.text += value[k]; else out.push({ text: value[k], hit: marks[k] });
        }
        return out;
    }

    // The commands for the current data. `label(action, params)` returns the translated label. Own stacks come first. With
    // `logs: true` every stack also gets a Logs command; the container commands (Logs for every container, Terminal for the
    // running ones unless `canTerminal` is false) come last and only when `containers` (the rows of model.buildRows) is given.
    function paletteItems(data) {
        const d = data || {}, label = d.label, items = [];
        const add = (id, action, params, extra) => items.push(Object.assign({ id, action, label: label(action, params || {}) }, extra));
        for (const row of objects(d.stackRows)) {
            const name = d.multi ? row.stack.name + ' (' + row.hostName + ')' : row.stack.name, base = { stackKey: row.key, hostId: row.hostId };
            const params = { stack: name, name };
            add('open:' + row.key, 'open', params, base);
            if (d.logs) add('logs:' + row.key, 'logs', params, base);
            if (!d.canMutate) continue;
            const a = stackActions(row.stack);
            add('deploy:' + row.key, a.primary, params, base);
            if (stackStatusId(row.stack) === 'partial') add('update:' + row.key, 'update', params, base);
            if (a.restart) add('restart:' + row.key, 'restart', params, base);
            if (a.stop) add('stop:' + row.key, 'stop', params, base);
        }
        for (const host of objects(d.hosts)) {
            const params = { host: text(host.name) };
            if (d.canMutate) add('new:' + host.id, 'new_stack', params, { hostId: host.id });
            if (d.canMutate && d.hostStates && d.hostStates[host.id] === 'not_prepared') add('prepare:' + host.id, 'prepare', params, { hostId: host.id });
            if (d.scope !== host.id) add('switch:' + host.id, 'switch_host', params, { hostId: host.id });
        }
        if (d.scope !== 'all' && objects(d.hosts).length > 1) add('switch:all', 'switch_all', {}, { hostId: 'all' });
        add('refresh', 'refresh', {});
        for (const row of objects(d.containers)) {
            const c = row.c || {}, own = array(c.names)[0] || c.id, name = text(d.multi ? own + ' (' + row.hostName + ')' : own).replace(/^\//, '');
            const extra = { rowKey: row.key, hostId: row.hostId };
            items.push(Object.assign({ id: 'clogs:' + row.key, action: 'logs_container', label: label('logs', { name }) }, extra));
            if (d.canTerminal !== false && d.canMutate && lower(c.state) === 'running' && !c.aurago_helper) items.push(Object.assign({ id: 'term:' + row.key, action: 'terminal', label: label('terminal', { name }) }, extra));
        }
        return items;
    }

    // ---- operations: folding stream events into one entry ------------------------------------------------------------

    const isActiveState = state => ACTIVE_STATES.includes(state);
    const isActiveOp = op => isActiveState(op && op.state);

    function capOutput(value) {
        const source = text(value);
        if (source.length <= OUTPUT_CAP) return source;
        const cut = source.slice(source.length - OUTPUT_CAP), newline = cut.indexOf('\n');
        return newline >= 0 && newline < 512 ? cut.slice(newline + 1) : cut;
    }

    function newOpEntry(op, now) {
        const o = Object.assign({}, op);
        return { id: text(o.id), op: o, output: capOutput(o.output), progress: [], stream: 'idle', local: Number(now) || 0, shown: false };
    }

    // A fresh snapshot (first connect or reconnect) replaces the output and the progress seen so far.
    function applySnapshot(entry, op) {
        entry.op = Object.assign({}, op);
        entry.output = capOutput(op && op.output);
        entry.progress = [];
        return entry;
    }

    function mergeProgress(list, item) {
        if (!item || !item.id) return;
        let current = list.find(entry => entry.id === item.id);
        if (!current) { current = { id: text(item.id), parent: '', status: '', text: '', current: 0, total: 0, percent: 0 }; list.push(current); }
        for (const key of ['parent', 'status', 'text']) if (item[key] !== undefined) current[key] = text(item[key]);
        for (const key of ['current', 'total', 'percent']) if (item[key] !== undefined) current[key] = Number(item[key]) || 0;
        if (current.total > 0) current.percent = Math.max(0, Math.min(100, (current.current / current.total) * 100));
    }

    // Folds one stream event {type, text, progress, operation} into the entry. Returns the entry.
    function applyOpEvent(entry, event) {
        const e = event || {};
        if (e.type === 'output') entry.output = capOutput(entry.output + text(e.text));
        else if (e.type === 'progress') mergeProgress(entry.progress, e.progress);
        else if (e.type === 'state' && e.operation) entry.op = Object.assign({}, entry.op, e.operation, { output: entry.op.output });
        else if (e.type === 'done' && e.operation) {
            entry.op = Object.assign({}, e.operation);
            if (e.operation.output) entry.output = capOutput(e.operation.output);
        }
        return entry;
    }

    // Per-image bars: progress items that carry a total, in the order they were first seen.
    const progressBars = entry => objects(entry && entry.progress).filter(item => item.total > 0);

    // Newest first in, newest first out: every active entry stays, finished ones only up to `keep`.
    function trimEntries(entries, keep) {
        let finished = 0;
        return array(entries).filter(entry => isActiveOp(entry.op) || finished++ < keep);
    }

    function parseJSON(data) {
        try { const value = JSON.parse(data); return value && typeof value === 'object' ? value : null; } catch (error) { return null; }
    }

    // A draft is dirty when its text differs from the file as last loaded or saved.
    const draftDirty = (draft, file) => !!draft && text(draft.text) !== text(file && file.content);

    window.DockerManagerStackModel = {
        SECTIONS, STACK_TABS, PROJECT_TABS, STACK_STATUSES, STACK_TONE, OP_KINDS, OP_STATES, ACTIVE_STATES, OP_TONE, SERVICE_STATES, LINT_CODES, VOLUME_KINDS, CONVERT_WARNINGS,
        OUTPUT_CAP, OP_FOR_ACTION,
        validateName, sanitizeName, firstServiceName, stackStatusId, stackTone, stackRows, externalRows, singleContainerRows, matchesStackQuery, stackGroups,
        hostStacksState, stackCounts, primaryAction, stackActions, serviceTone, serviceStateId, linkAddress, portLinks, age, formatBytes, clock, elapsed,
        diagnosticFile, severityOf, sortDiagnostics, splitDiagnostics, lintCounts, worstSeverity, diagnosticText, lineIndex, lineRange, storageGroups, randomFixes,
        envHas, appendEnvVariable, randomValue, classifyDroppedFile, lineDiff, diffChanged, collapseDiff, fuzzyScore, rankItems, highlight, paletteItems,
        isActiveState, isActiveOp, capOutput, newOpEntry, applySnapshot, applyOpEvent, progressBars, trimEntries, parseJSON, draftDirty
    };
})();
