// EasyDrag browser fixture: an in-memory /api/desktop/flows backend that answers with the server's
// shapes (internal/server/flows_handlers*.go), a run engine with a scripted EventSource (snapshot,
// events with seq and id, end, resync) and a small /api/missions/v2 stub for Mission Control.
// Loaded after aurora-fixture.js by TestDesktopEasyDragBrowser; it wraps whatever window.fetch is.
// A flows request that no route here takes is pushed into fixtureErrors, so the test fails instead
// of passing on aurora-fixture's catch-all answer.
(function () {
    'use strict';

    const previousFetch = window.fetch.bind(window);
    const JSON_HEADERS = { 'Content-Type': 'application/json; charset=utf-8', 'Cache-Control': 'no-store' };
    const reply = (data, status, headers) => new Response(JSON.stringify(data) + '\n', { status: status || 200, headers: Object.assign({}, JSON_HEADERS, headers || {}) });
    const fail = (status, code, error, headers) => reply({ error: error || code, code }, status, headers);
    const iso = offset => new Date(Date.now() + (offset || 0)).toISOString();
    // ZERO_TIME is how Go encodes the zero time.Time (published_at of an unpublished flow).
    const ZERO_TIME = '0001-01-01T00:00:00Z';
    const clone = v => JSON.parse(JSON.stringify(v));
    const wait = ms => new Promise(r => setTimeout(r, ms));
    const ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/;
    const SECRET_PATTERN = /^[a-z0-9_]{1,40}$/;
    const SECRET_MAX_BYTES = 4 << 10;
    const SECRET_WRITES_PER_MINUTE = 30;
    let idSeq = 0;
    const nextID = prefix => prefix + String(++idSeq).padStart(prefix === 'flow_' ? 10 : 12, 'a');

    // unrouted records a request the fixture has no route for; the test fails on fixtureErrors.
    function unrouted(method, path) {
        const message = 'easydrag-fixture: no route for ' + method + ' ' + path;
        if (Array.isArray(window.fixtureErrors)) window.fixtureErrors.push(message);
        return fail(404, 'FLOW_NOT_FOUND', message);
    }

    const P = (name, kind, label, extra) => Object.assign({ name, kind, label }, extra || {});
    const T = (type, category, icon, label, extra) => Object.assign({
        type, version: 1, category, icon, label, inputs: ['in'], outputs: ['out'], params: [], availability: { state: 'available' }
    }, extra || {});

    const catalog = {
        categories: [
            { id: 'trigger', label: 'Auslöser' }, { id: 'logic', label: 'Logik' }, { id: 'ai', label: 'KI' }, { id: 'web', label: 'Web' },
            { id: 'documents', label: 'Dokumente & Dateien' }, { id: 'notify', label: 'Benachrichtigungen' }, { id: 'smart_home', label: 'Smart Home' },
            { id: 'tool:files', label: 'Dateien' }
        ],
        node_types: [
            T('trigger.manual', 'trigger', 'hand-click', 'Manuell starten', { trigger: true, inputs: [], description: 'Startet den Flow per Klick.',
                params: [P('data', 'json', 'Beispieldaten')], output_fields: [{ name: 'data', type: 'object' }], sample: { data: { thema: 'KI' } } }),
            // The real schedule trigger (scheduleParams in internal/flows/catalog_schedule.go): a mode
            // and the values that mode shows.
            T('trigger.schedule', 'trigger', 'clock', 'Zeitplan', { trigger: true, inputs: [], summary: '{mode}', description: 'Startet zu festen Zeiten, zum Beispiel werktags um 7:00.',
                params: [
                    P('mode', 'select', 'Wiederholung', { default: 'daily', options: [
                        { value: 'interval_minutes', label: 'Alle paar Minuten' }, { value: 'interval_hours', label: 'Alle paar Stunden' },
                        { value: 'daily', label: 'Täglich' }, { value: 'weekdays', label: 'Werktags (Mo–Fr)' }, { value: 'weekly', label: 'Wöchentlich' },
                        { value: 'monthly', label: 'Monatlich' }, { value: 'cron', label: 'Cron-Ausdruck' }] }),
                    P('minutes', 'number', 'Minuten', { required: true, default: 15, visible_if: { param: 'mode', equals: ['interval_minutes'] } }),
                    P('hours', 'number', 'Stunden', { required: true, default: 1, visible_if: { param: 'mode', equals: ['interval_hours'] } }),
                    P('time', 'text', 'Uhrzeit', { required: true, default: '07:00', visible_if: { param: 'mode', equals: ['daily', 'weekdays', 'weekly', 'monthly'] } }),
                    P('weekdays', 'multiselect', 'Wochentage', { required: true, visible_if: { param: 'mode', equals: ['weekly'] }, options: [
                        { value: 'mon', label: 'Mo' }, { value: 'tue', label: 'Di' }, { value: 'wed', label: 'Mi' }, { value: 'thu', label: 'Do' },
                        { value: 'fri', label: 'Fr' }, { value: 'sat', label: 'Sa' }, { value: 'sun', label: 'So' }] }),
                    P('day', 'number', 'Tag im Monat', { required: true, default: 1, visible_if: { param: 'mode', equals: ['monthly'] } }),
                    P('cron', 'cron', 'Cron-Ausdruck', { required: true, visible_if: { param: 'mode', equals: ['cron'] } })
                ], output_fields: [{ name: 'fired_at', type: 'text' }] }),
            T('web.search', 'web', 'search', 'Websuche', { summary: '{query}', description: 'Sucht im Web.', primary_input: 'query',
                params: [P('query', 'text', 'Suchbegriff', { required: true, templatable: true }), P('count', 'number', 'Anzahl', { default: 5 })],
                output_fields: [{ name: 'results', type: 'list', primary: true }, { name: 'count', type: 'number' }] }),
            // The real http.request names no effect in the catalog: its default method is GET. A
            // POST sends (httpRequestEffects), which only the publish preview sees (effects()).
            T('http.request', 'web', 'world-www', 'HTTP-Anfrage', { summary: '{method} {url}', description: 'Ruft eine Web-Adresse auf.', primary_input: 'url',
                params: [
                    P('method', 'select', 'Methode', { default: 'GET', options: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map(m => ({ value: m, label: m })) }),
                    P('url', 'text', 'Adresse', { required: true, templatable: true }),
                    P('auth_secret', 'secret_ref', 'Geheimnis')
                ], output_fields: [{ name: 'body', type: 'text', primary: true }, { name: 'status', type: 'number' }] }),
            T('ai.prompt', 'ai', 'sparkles', 'KI-Schritt', { description: 'Lässt die KI einen Text erzeugen.', primary_input: 'prompt',
                params: [P('prompt', 'textarea', 'Anweisung', { required: true, templatable: true })],
                output_fields: [{ name: 'text', type: 'text', primary: true }, { name: 'tokens', type: 'object' }] }),
            T('documents.pdf', 'documents', 'file-type-pdf', 'PDF erstellen', { description: 'Erstellt ein PDF im Workspace.', effects: ['writes_files'], primary_input: 'content',
                params: [P('title', 'text', 'Titel', { templatable: true }), P('content', 'textarea', 'Inhalt', { required: true, templatable: true })],
                output_fields: [{ name: 'file', type: 'file', primary: true }] }),
            T('notify.telegram', 'notify', 'brand-telegram', 'Telegram senden', { description: 'Schickt eine Nachricht an Telegram.', effects: ['sends_message'], primary_input: 'message',
                params: [P('message', 'textarea', 'Nachricht', { required: true, templatable: true, sensitive_sink: true }), P('file', 'file', 'Datei', { templatable: true })],
                output_fields: [{ name: 'message_id', type: 'number' }] }),
            T('logic.if', 'logic', 'arrows-split', 'Wenn / dann', { description: 'Verzweigt nach einer Bedingung.', outputs: ['true', 'false'],
                params: [P('condition', 'condition_group', 'Bedingung', { required: true })] }),
            T('smart_home.service', 'smart_home', 'home', 'Home Assistant', { description: 'Schaltet Geräte.', effects: ['controls_devices'], risky: true,
                availability: { state: 'needs_setup', config_section: 'home_assistant' },
                params: [P('entity', 'select', 'Gerät', { required: true, options_source: 'ha_entities' })] }),
            T('tool.file_reader', 'tool:files', 'tool', 'Datei lesen', { description: 'Liest eine Datei aus dem Workspace.', tool: 'file_reader',
                params: [P('path', 'file', 'Datei', { required: true, templatable: true })], output_fields: [{ name: 'content', type: 'text', primary: true }] })
        ]
    };
    const types = new Map(catalog.node_types.map(i => [i.type, i]));
    const CATALOG_ETAG = 'W/"easydrag-fixture-1"';
    const templates = [
        { id: 'ai_news_pdf_telegram', name: 'KI-Nachrichten als PDF', description: 'Sucht jeden Morgen Nachrichten, fasst sie zusammen und schickt ein PDF.', categories: ['trigger', 'web', 'ai', 'documents', 'notify'] },
        { id: 'daily_digest', name: 'Tagesrückblick', description: 'Fasst den Tag am Abend zusammen.', categories: ['trigger', 'ai', 'notify'] }
    ];

    const node = (id, key, type, label, x, y, params) => ({ id, key, type, type_version: 1, label, position: { x, y }, params: params || {}, settings: { retry: {} } });
    const edge = (id, a, b, port) => ({ id, source: { node: a, port: port || 'out' }, target: { node: b, port: 'in' } });
    const FLOW_SETTINGS = { concurrency: 'queue', max_run_seconds: 1800, notify_on_error: 'desktop' };
    function briefing() {
        return {
            schema: 1, kind: 'flow', name: 'Morgenbriefing', description: 'KI-Nachrichten als PDF', settings: clone(FLOW_SETTINGS),
            nodes: [
                node('n_zeitplan', 'zeitplan', 'trigger.schedule', 'Werktags 7 Uhr', 0, 120, { mode: 'weekdays', time: '07:00' }),
                node('n_suche', 'suche', 'web.search', 'Nachrichten suchen', 320, 120, { query: 'KI Nachrichten heute', count: 5 }),
                node('n_ki', 'ki', 'ai.prompt', 'Zusammenfassen', 640, 120, { prompt: 'Fasse diese Schlagzeilen zusammen: {{suche.results | pluck("title") | join(", ")}}' }),
                node('n_pdf', 'pdf', 'documents.pdf', 'PDF erstellen', 960, 120, { title: 'Briefing', content: '{{ki.text}}' })
            ],
            edges: [edge('e_1', 'n_zeitplan', 'n_suche'), edge('e_2', 'n_suche', 'n_ki'), edge('e_3', 'n_ki', 'n_pdf')]
        };
    }
    function emptyFlow(name) {
        return { schema: 1, kind: 'flow', name: name || 'Neuer Flow', nodes: [], edges: [], settings: clone(FLOW_SETTINGS) };
    }

    // state is what the test reads and switches:
    //   disabled        every flows route answers 503 FLOWS_DISABLED (the capability went off)
    //   failSaves       answers {status, code, error} the next draft saves get, in order (for
    //                   example 500 FLOW_INTERNAL, or 403 FLOW_PERMISSION_DENIED "flows are read-only")
    //   publishPartial  a publish answers 200 {partial: true, code: publishPartial, …}
    //   resyncAfter     a run stream sends "resync" after this many live events (once), as the bus
    //                   does when it drops a slow client
    //   failType        steps of this node type fail, and their run with them
    //   failPreview     the publish preview answers 500 FLOW_INTERNAL
    //   delay           milliseconds each step of a run takes
    const state = {
        flows: new Map(), runs: new Map(), saves: [], requests: [], testData: {}, secrets: ['wetter_api'], secretWrites: [], streams: [],
        delay: 140, disabled: false, failSaves: [], publishPartial: '', resyncAfter: 0, failType: '', failPreview: false,
        options: { ha_entities: { options: [{ value: 'light.kueche', label: 'Küche' }, { value: 'light.flur', label: 'Flur' }], truncated: true } }
    };

    // broadcast sends flows_changed through the desktop's event handler, as the server's
    // broadcastFlowsChanged reaches the shell over the WebSocket.
    function broadcast(flowId, reason) {
        setTimeout(() => {
            const payload = { flow_id: flowId, reason };
            const shell = window.aurora && typeof window.aurora.handleDesktopEvent === 'function' ? window.aurora.handleDesktopEvent : null;
            if (shell) shell({ type: 'flows_changed', payload });
            else document.dispatchEvent(new CustomEvent('aurago:flows-changed', { detail: payload }));
        }, 0);
    }

    function addFlow(doc, published, enabled) {
        const id = nextID('flow_');
        const draft = Object.assign(clone(doc), { id });
        const rec = {
            id, kind: 'flow', name: draft.name, description: draft.description || '', mission_id: 'mission_' + id,
            draft, draft_revision: 1, live_revision: published ? 1 : 0,
            published_draft_revision: published ? 1 : 0, published_at: published ? iso(-86400000) : ZERO_TIME, created_at: iso(-172800000), updated_at: iso(-3600000)
        };
        if (published) rec.live = clone(draft);
        state.flows.set(id, { rec, enabled: !!enabled });
        return rec;
    }
    const seeded = addFlow(briefing(), true, true);

    function triggerNodes(doc) { return doc.nodes.filter(n => (types.get(n.type) || {}).trigger && !(n.settings || {}).disabled); }
    function triggersOf(doc) { return triggerNodes(doc).map(n => n.type); }
    // header is a run record as the run list and the start page get it: without trigger_data.
    function header(run) { const out = clone(run); delete out.trigger_data; return out; }
    function lastRun(id) {
        const runs = Array.from(state.runs.values()).filter(r => r.run.flow_id === id && r.run.mode === 'live').map(r => r.run);
        runs.sort((a, b) => b.started_at.localeCompare(a.started_at));
        return runs[0] ? header(runs[0]) : null;
    }
    function summary(entry) {
        const r = entry.rec;
        const out = {
            id: r.id, name: r.name, mission_id: r.mission_id, enabled: entry.enabled,
            published: !!r.live, has_unpublished_changes: !r.live || r.draft_revision !== r.published_draft_revision,
            draft_revision: r.draft_revision, live_revision: r.live_revision, updated_at: r.updated_at,
            triggers: triggersOf(r.draft), preview: r.draft.nodes.map(n => ({ x: n.position.x, y: n.position.y, category: (types.get(n.type) || {}).category || 'unknown' }))
        };
        if (r.description) out.description = r.description;
        const last = lastRun(r.id);
        if (last) out.last_run = last;
        return out;
    }

    // paramValue is a param's value, or its default when the node does not set it.
    function paramValue(info, params, name) {
        if (params && Object.prototype.hasOwnProperty.call(params, name)) return params[name];
        const spec = info.params.find(p => p.name === name);
        return spec ? spec.default : undefined;
    }

    // paramVisible mirrors paramVisible in internal/flows/validate.go: a hidden param is not required.
    function paramVisible(info, params, spec) {
        if (!spec.visible_if) return true;
        const value = paramValue(info, params, spec.visible_if.param);
        return spec.visible_if.equals.includes(String(value == null ? '' : value));
    }

    function validate(doc, mode) {
        const issues = [];
        const severity = mode === 'publish' ? 'error' : 'warning';
        if (!triggersOf(doc).length) issues.push({ code: 'FLOW_NO_TRIGGER', severity, message: 'the flow has no enabled trigger' });
        doc.nodes.forEach(n => {
            const info = types.get(n.type);
            if (!info) { issues.push({ code: 'NODE_TYPE_UNKNOWN', severity: 'error', node_id: n.id, message: 'unknown node type' }); return; }
            info.params.filter(p => p.required && paramVisible(info, n.params, p)).forEach(p => {
                const v = paramValue(info, n.params, p.name);
                if (v === undefined || v === null || v === '' || (Array.isArray(v) && !v.length)) {
                    issues.push({ code: 'PARAM_REQUIRED', severity, node_id: n.id, param: p.name, message: p.name + ' is required' });
                }
            });
            if (n.type === 'notify.telegram' && /\{\{\s*trigger/.test(JSON.stringify(n.params))) {
                issues.push({ code: 'UNTRUSTED_DATA_TO_SINK', severity: 'warning', node_id: n.id, param: 'message', message: 'untrusted data reaches a message' });
            }
        });
        return issues;
    }
    const hasErrors = issues => issues.some(i => i.severity === 'error');

    // effectsOf mirrors the definitions' EffectsOf: the effects of a node's real parameters. An
    // HTTP request sends unless its method is missing, empty or GET (httpRequestEffects); the
    // other fixture types have fixed effects.
    function effectsOf(n) {
        if (n.type === 'http.request') {
            const method = (n.params || {}).method;
            if (method === undefined || method === null) return [];
            if (typeof method === 'string' && (!method.trim() || method.trim().toUpperCase() === 'GET')) return [];
            return ['sends_message'];
        }
        return (types.get(n.type) || {}).effects || [];
    }

    // effects is CollectEffects: the effects of the real parameters of every enabled node.
    function effects(doc) {
        const map = new Map();
        doc.nodes.forEach(n => {
            if ((n.settings || {}).disabled) return;
            effectsOf(n).forEach(e => { if (!map.has(e)) map.set(e, []); map.get(e).push(n.id); });
        });
        return Array.from(map.entries()).map(([effect, node_ids]) => ({ effect, node_ids }));
    }

    // diff counts like flows.DiffFlows: nodes by id (moving one is no change), edges by their ports.
    function diff(live, draft) {
        if (!live) return { first_publish: true, added_nodes: draft.nodes.length, removed_nodes: 0, changed_nodes: 0, changed_edges: 0 };
        const strip = n => JSON.stringify(Object.assign({}, n, { position: null }));
        const before = new Map(live.nodes.map(n => [n.id, n]));
        const after = new Map(draft.nodes.map(n => [n.id, n]));
        const ports = d => new Set(d.edges.map(e => e.source.node + ':' + e.source.port + '>' + e.target.node + ':' + e.target.port));
        const a = ports(live);
        const b = ports(draft);
        return {
            first_publish: false,
            added_nodes: draft.nodes.filter(n => !before.has(n.id)).length,
            removed_nodes: live.nodes.filter(n => !after.has(n.id)).length,
            changed_nodes: draft.nodes.filter(n => before.has(n.id) && strip(before.get(n.id)) !== strip(n)).length,
            changed_edges: Array.from(a).filter(x => !b.has(x)).length + Array.from(b).filter(x => !a.has(x)).length
        };
    }

    function outputFor(n) {
        switch (n.type) {
        case 'web.search': return { results: [1, 2, 3].map(i => ({ title: 'Schlagzeile ' + i, url: 'https://news.example/' + i, snippet: 'Kurzer Anriss ' + i })), count: 3 };
        case 'ai.prompt': return { text: 'Heute: drei wichtige KI-Meldungen, kurz und verständlich zusammengefasst.', tokens: { input: 182, output: 64 }, model: 'fixture' };
        case 'documents.pdf': return { file: { $type: 'file', name: 'briefing.pdf', path: 'Documents/EasyDrag/briefing.pdf', size: 18234 } };
        case 'notify.telegram': return { message_id: 42 };
        default: return (types.get(n.type) || {}).trigger ? { data: { thema: 'KI' }, fired_at: iso() } : { ok: true };
        }
    }

    // plan orders the nodes reachable from the trigger (breadth first); the false and error ports
    // are not taken.
    function plan(doc, triggerID, onlyNode) {
        if (onlyNode) return [doc.nodes.find(n => n.id === onlyNode)].filter(Boolean);
        const order = [];
        const seen = new Set();
        const queue = [triggerID];
        while (queue.length) {
            const id = queue.shift();
            if (seen.has(id)) continue;
            seen.add(id);
            const n = doc.nodes.find(x => x.id === id);
            if (!n || (n.settings || {}).disabled) continue;
            order.push(n);
            doc.edges.filter(e => e.source.node === id && e.source.port !== 'false' && e.source.port !== 'error').forEach(e => queue.push(e.target.node));
        }
        return order;
    }

    function sampleOf(entry, nodeId) {
        const stored = state.testData[entry.rec.id + '/' + nodeId];
        if (stored) return clone(stored);
        const n = entry.rec.draft.nodes.find(x => x.id === nodeId);
        const sample = n && (types.get(n.type) || {}).sample;
        return sample && sample.data ? clone(sample.data) : {};
    }

    // ── run engine: runs go on whether a stream watches them or not, as on the server ──

    function emitRunEvent(rec, type, extra) {
        const ev = Object.assign({ seq: rec.events.length + 1, run_id: rec.run.id, type }, extra, { time: iso() });
        rec.events.push(ev);
        Array.from(rec.listeners).forEach(fn => fn(ev));
    }

    function endRun(rec, status, failure) {
        rec.run.status = status;
        rec.run.finished_at = iso();
        rec.run.duration_ms = Math.max(1, Date.parse(rec.run.finished_at) - Date.parse(rec.run.started_at));
        const summary = { status, duration_ms: rec.run.duration_ms };
        if (failure) {
            rec.run.error_code = summary.error_code = failure.error_code;
            rec.run.error_message = summary.error_message = failure.error_message;
        }
        emitRunEvent(rec, 'run_finished', { run: summary });
        rec.done = true;
        Array.from(rec.listeners).forEach(fn => fn(null));
        rec.listeners.clear();
        // Only live runs reach Mission Control (Service.onRunFinished skips test runs).
        if (rec.run.mode === 'live') broadcast(rec.run.flow_id, 'run_finished');
    }

    async function execute(rec) {
        await wait(30);
        if (rec.cancel) { endRun(rec, 'cancelled'); return; }
        rec.run.status = 'running';
        emitRunEvent(rec, 'run_started', { run: { status: 'running' } });
        for (const n of rec.order) {
            if (rec.cancel) break;
            emitRunEvent(rec, 'step_started', { node_id: n.id });
            const started = iso();
            await wait(state.delay);
            if (rec.cancel) break;
            if (state.failType && n.type === state.failType) {
                const failure = { error_code: 'FLOW_NOTIFY_FAILED', error_message: 'Bad Request: chat not found' };
                const step = Object.assign({ node_id: n.id, node_key: n.key, attempt: 1, status: 'error', started_at: started, finished_at: iso(), duration_ms: state.delay }, failure);
                rec.steps.push(step);
                emitRunEvent(rec, 'step_finished', { node_id: n.id, step });
                endRun(rec, 'error', failure);
                return;
            }
            const out = outputFor(n);
            const step = {
                node_id: n.id, node_key: n.key, attempt: 1, status: 'success', started_at: started, finished_at: iso(), duration_ms: state.delay,
                output: out, item_count: Array.isArray(out.results) ? out.results.length : 1, ports: n.type === 'logic.if' ? ['true'] : ['out']
            };
            rec.steps.push(step);
            emitRunEvent(rec, 'step_finished', { node_id: n.id, step });
        }
        endRun(rec, rec.cancel ? 'cancelled' : 'success');
    }

    function startRun(entry, mode, body) {
        const doc = clone(mode === 'live' ? entry.rec.live : entry.rec.draft);
        const triggers = triggerNodes(doc);
        const trigger = triggers.find(n => n.id === body.trigger_node) || triggers.find(n => n.type === 'trigger.manual') || triggers[0];
        if (!trigger) return null;
        const id = nextID('run_');
        const data = body.trigger_data || sampleOf(entry, trigger.id);
        if (mode === 'test' && body.remember_data && body.trigger_data) state.testData[entry.rec.id + '/' + trigger.id] = clone(body.trigger_data);
        const run = {
            id, flow_id: entry.rec.id, revision: mode === 'live' ? entry.rec.live_revision : entry.rec.draft_revision, mode, trigger_node: trigger.id,
            trigger_type: mode === 'test' ? 'test' : 'manual', trigger_data: data, status: 'queued', started_at: iso(), duration_ms: 0
        };
        const rec = { run, steps: [], doc, order: plan(doc, trigger.id, body.only_node), events: [], listeners: new Set(), done: false, cancel: false };
        state.runs.set(id, rec);
        execute(rec);
        return id;
    }

    // waitingRun adds a live run of the published flow that a trigger started and that waits for a
    // slot: it never starts by itself, and a cancel ends it at once (the runner's finishUnstarted).
    function waitingRun(flowId) {
        const entry = state.flows.get(flowId || seeded.id);
        const doc = clone(entry.rec.live);
        const trigger = triggerNodes(doc)[0];
        const id = nextID('run_');
        const run = {
            id, flow_id: entry.rec.id, revision: entry.rec.live_revision, mode: 'live', trigger_node: trigger.id,
            trigger_type: 'schedule', trigger_data: {}, status: 'waiting', started_at: iso(), duration_ms: 0
        };
        state.runs.set(id, { run, steps: [], doc, order: [], events: [], listeners: new Set(), done: false, cancel: false, unstarted: true });
        return id;
    }

    // ── scripted EventSource for GET /api/desktop/flows/runs/{run}/events (streamFlowRunEvents) ──
    class FixtureEventSource {
        constructor(url) {
            this.url = String(url);
            this.listeners = {};
            this.closed = false;
            this.readyState = 0;
            this.onerror = null;
            this.onmessage = null;
            this.unsubscribe = null;
            state.streams.push(this);
            setTimeout(() => this.open(), 10);
        }
        addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
        removeEventListener(type, fn) { this.listeners[type] = (this.listeners[type] || []).filter(f => f !== fn); }
        close() {
            this.closed = true;
            this.readyState = 2;
            if (this.unsubscribe) { this.unsubscribe(); this.unsubscribe = null; }
        }
        dispatch(type, data, id) {
            if (this.closed) return;
            const event = { type, data: JSON.stringify(data), lastEventId: id ? String(id) : '' };
            (this.listeners[type] || []).slice().forEach(fn => fn(event));
        }
        fail() {
            this.readyState = 2;
            if (typeof this.onerror === 'function') this.onerror({ type: 'error' });
        }
        open() {
            if (this.closed) return;
            const u = new URL(this.url, location.origin);
            const match = /^\/api\/desktop\/flows\/runs\/([^/]+)\/events$/.exec(u.pathname);
            if (!match) { unrouted('GET', u.pathname + u.search); this.fail(); return; }
            const rec = state.runs.get(decodeURIComponent(match[1]));
            // The server answers 503 or 404 with JSON, which an EventSource reports as an error.
            if (state.disabled || !rec) { this.fail(); return; }
            this.readyState = 1;
            const asked = Number(u.searchParams.get('after'));
            const after = Number.isInteger(asked) && asked > 0 && asked <= (1 << 20) ? asked : 0;
            let last = after;
            let finished = false;
            let live = 0;
            const send = ev => {
                last = Math.max(last, ev.seq);
                if (ev.type === 'run_finished') finished = true;
                this.dispatch('event', ev, ev.seq);
            };
            // snapshot first (the stored RunDetail, without an id), then the backlog after the start point.
            this.dispatch('snapshot', { run: clone(rec.run), steps: clone(rec.steps) });
            rec.events.filter(ev => ev.seq > after).forEach(send);
            if (rec.done) { this.dispatch('end', {}); this.close(); return; }
            const listener = ev => {
                if (this.closed) return;
                if (ev === null) { this.dispatch('end', {}); this.close(); return; }
                send(clone(ev));
                live++;
                if (state.resyncAfter && live >= state.resyncAfter && !finished) {
                    state.resyncAfter = 0;
                    this.dispatch('resync', { after: last });
                    this.close();
                }
            };
            rec.listeners.add(listener);
            this.unsubscribe = () => rec.listeners.delete(listener);
        }
    }
    FixtureEventSource.CONNECTING = 0;
    FixtureEventSource.OPEN = 1;
    FixtureEventSource.CLOSED = 2;
    window.EventSource = FixtureEventSource;

    // ── routes (handleFlows) ──

    function collection(method, body) {
        if (method === 'GET') return reply({ flows: Array.from(state.flows.values()).map(summary) });
        if (method !== 'POST') return null;
        let doc;
        if (body.import) {
            if (body.import.schema !== 1) return fail(400, 'FLOW_BAD_REQUEST', 'unsupported flow schema');
            doc = clone(body.import);
        } else if (body.template) {
            if (!templates.some(tp => tp.id === body.template)) return fail(400, 'FLOW_BAD_REQUEST', 'unknown template');
            doc = Object.assign(briefing(), { name: 'Vorlage: Morgenbriefing' });
        } else {
            doc = emptyFlow(body.name);
        }
        if (body.name) doc.name = body.name;
        const rec = addFlow(doc, false, false);
        broadcast(rec.id, 'created');
        return reply({ flow: rec }, 201);
    }

    function secrets(method, rest, body) {
        if (!rest.length) return method === 'GET' ? reply({ secrets: state.secrets.slice().sort() }) : null;
        const name = rest[0];
        if (rest.length !== 1 || !SECRET_PATTERN.test(name)) return fail(400, 'FLOW_BAD_REQUEST', 'secret names use a-z, 0-9 and _ (at most 40 characters)');
        if (method !== 'PUT' && method !== 'DELETE') return null;
        const now = Date.now();
        state.secretWrites = state.secretWrites.filter(at => now - at < 60000);
        if (state.secretWrites.length >= SECRET_WRITES_PER_MINUTE) return fail(429, 'FLOW_RATE_LIMITED', 'too many flow secret changes; try again in a minute', { 'Retry-After': '60' });
        state.secretWrites.push(now);
        if (method === 'PUT') {
            const value = typeof body.value === 'string' ? body.value : '';
            if (!value.trim()) return fail(400, 'FLOW_BAD_REQUEST', 'the secret value is empty');
            if (new TextEncoder().encode(value).length > SECRET_MAX_BYTES) return fail(413, 'FLOW_TOO_LARGE', 'the secret value is larger than 4 KiB');
            if (!state.secrets.includes(name)) state.secrets.push(name);
            return reply({ status: 'saved' });
        }
        state.secrets = state.secrets.filter(s => s !== name);
        const usedBy = Array.from(state.flows.values()).filter(e => e.rec.live && JSON.stringify(e.rec.live.nodes).includes('"' + name + '"')).map(e => e.rec.name).sort();
        return reply({ status: 'deleted', used_by: usedBy });
    }

    function runRoute(method, rest) {
        if (!rest.length || !ID_PATTERN.test(rest[0])) return fail(404, 'FLOW_RUN_NOT_FOUND', 'run not found');
        const rec = state.runs.get(rest[0]);
        if (rest.length === 1 && method === 'GET') {
            if (!rec) return fail(404, 'FLOW_RUN_NOT_FOUND', 'run not found');
            return { detail: true, rec };
        }
        if (rest.length === 2 && rest[1] === 'cancel' && method === 'POST') {
            if (!rec) return fail(404, 'FLOW_RUN_NOT_FOUND', 'run not found');
            if (rec.done) return fail(409, 'FLOW_RUN_FINISHED', 'the run has already finished');
            rec.cancel = true;
            if (rec.unstarted) endRun(rec, 'cancelled', { error_code: 'FLOW_CANCELLED', error_message: 'the run was cancelled before it started' });
            return reply({ cancelled: true }, 202);
        }
        return null;
    }

    function nodeTypes(method, rest, headers) {
        if (method !== 'GET') return null;
        if (!rest.length) {
            if (headers.get('If-None-Match') === CATALOG_ETAG) return new Response(null, { status: 304, headers: { ETag: CATALOG_ETAG, 'Cache-Control': 'private, no-cache' } });
            return reply(catalog, 200, { ETag: CATALOG_ETAG, 'Cache-Control': 'private, no-cache' });
        }
        if (rest.length !== 3 || rest[1] !== 'options') return null;
        const info = types.get(rest[0]);
        if (!info) return fail(404, 'FLOW_NOT_FOUND', 'unknown node type');
        const spec = info.params.find(p => p.name === rest[2]);
        const list = spec && spec.options_source && state.options[spec.options_source];
        if (!list) return fail(404, 'FLOW_NOT_FOUND', 'the parameter has no dynamic options');
        const out = { options: clone(list.options) };
        if (list.truncated) out.truncated = true;
        return reply(out);
    }

    function flowRoute(method, id, rest, body, query) {
        if (!ID_PATTERN.test(id)) return fail(404, 'FLOW_NOT_FOUND', 'flow not found');
        const action = rest[0] || '';
        const segments = { '': 0, 'publish-preview': 1, publish: 1, enabled: 1, export: 1, test: 1, run: 1, runs: 1, 'test-data': 2 };
        if (!(action in segments) || rest.length !== segments[action]) return null;
        const entry = state.flows.get(id);
        if (!entry) return fail(404, 'FLOW_NOT_FOUND', 'flow not found');
        const rec = entry.rec;
        if (!action) {
            if (method === 'GET') return reply({ flow: rec, enabled: entry.enabled, issues: validate(rec.draft, 'draft') });
            if (method === 'DELETE') { state.flows.delete(rec.id); broadcast(rec.id, 'deleted'); return reply({ status: 'deleted' }); }
            if (method !== 'PUT') return null;
            const injected = state.failSaves.shift();
            if (injected) return fail(injected.status, injected.code, injected.error);
            if (!body.doc || body.doc.schema !== 1) return fail(400, 'FLOW_BAD_REQUEST', 'unsupported flow schema');
            // SaveDraft refuses a draft with error-severity issues (draft rules) before the revision check.
            const draftIssues = validate(body.doc, 'draft');
            if (hasErrors(draftIssues)) return reply({ error: 'the flow is not valid', code: 'FLOW_INVALID', issues: draftIssues }, 422);
            if (body.base_revision !== rec.draft_revision) return fail(409, 'FLOW_REVISION_CONFLICT', 'the draft was changed elsewhere');
            rec.draft = Object.assign(clone(body.doc), { id: rec.id });
            rec.name = rec.draft.name;
            rec.description = rec.draft.description || '';
            rec.draft_revision += 1;
            rec.updated_at = iso();
            state.saves.push(clone(rec.draft));
            broadcast(rec.id, 'saved');
            return reply({ draft_revision: rec.draft_revision, issues: validate(rec.draft, 'draft') });
        }
        if (action === 'publish-preview' && method === 'GET') {
            if (state.failPreview) return fail(500, 'FLOW_INTERNAL', 'the flow service failed; see the server log');
            const issues = validate(rec.draft, 'publish');
            return reply({ issues, effects: effects(rec.draft), diff: diff(rec.live || null, rec.draft), can_publish: !hasErrors(issues) });
        }
        if (action === 'publish' && method === 'POST') {
            if (body.base_revision !== rec.draft_revision) return fail(409, 'FLOW_REVISION_CONFLICT', 'the draft was changed elsewhere');
            const issues = validate(rec.draft, 'publish');
            if (hasErrors(issues)) return reply({ error: 'the flow cannot be published', code: 'FLOW_INVALID', issues }, 422);
            // Publishing the published draft revision again changes nothing in the store (the heal path).
            if (!rec.live || rec.published_draft_revision !== rec.draft_revision) {
                rec.live = clone(rec.draft);
                rec.live_revision += 1;
                rec.published_draft_revision = rec.draft_revision;
                rec.published_at = iso();
            }
            broadcast(rec.id, 'published');
            if (state.publishPartial) {
                const code = state.publishPartial;
                const error = code === 'FLOW_MISSION_MISSING'
                    ? "Published, but the flow's Mission Control entry is missing. Export the flow, delete it and import it again."
                    : 'Published, but not every part could be updated (Mission Control or timers). Publish again to finish.';
                return reply({ flow: rec, issues, partial: true, code, error });
            }
            return reply({ flow: rec, issues });
        }
        if (action === 'enabled' && method === 'POST') {
            if (body.enabled && !rec.live) return fail(409, 'FLOW_NOT_PUBLISHED', 'the flow is not published');
            entry.enabled = !!body.enabled;
            broadcast(rec.id, 'enabled');
            return reply({ enabled: entry.enabled });
        }
        if (action === 'export' && method === 'GET') return reply(rec.draft, 200, { 'Content-Disposition': 'attachment; filename="flow.easydrag.json"' });
        if ((action === 'test' || action === 'run') && method === 'POST') {
            if (action === 'run' && !rec.live) return fail(409, 'FLOW_NOT_PUBLISHED', 'the flow is not published');
            // RunNow refuses a paused flow (ErrFlowDisabled); test runs stay allowed.
            if (action === 'run' && !entry.enabled) return fail(409, 'FLOW_DISABLED', 'the flow is paused; switch it on first');
            if (action === 'test') {
                const issues = validate(rec.draft, 'draft');
                if (hasErrors(issues)) return reply({ error: 'the draft has errors', code: 'FLOW_INVALID', issues }, 422);
                if (body.only_node && !rec.draft.nodes.some(n => n.id === body.only_node)) {
                    return reply({ error: 'the node to test does not exist', code: 'FLOW_INVALID', issues: [{ code: 'FLOW_NODE_NOT_FOUND', severity: 'error', message: 'the node to test does not exist' }] }, 422);
                }
            }
            const runID = startRun(entry, action === 'test' ? 'test' : 'live', body);
            if (!runID) return fail(409, 'FLOW_NO_TRIGGER', 'the flow has no trigger');
            return reply({ run_id: runID, status: 'started' }, 202);
        }
        if (action === 'runs' && method === 'GET') {
            const mode = query.get('mode') || '';
            const status = query.get('status') || '';
            const limitText = query.get('limit') || '';
            if (mode && !['test', 'live', 'agent', 'call'].includes(mode)) return fail(400, 'FLOW_BAD_REQUEST', 'mode must be test, live, agent or call');
            if (status && !['queued', 'running', 'waiting', 'success', 'error', 'cancelled'].includes(status)) return fail(400, 'FLOW_BAD_REQUEST', 'status must be queued, running, waiting, success, error or cancelled');
            if (limitText && !/^\d+$/.test(limitText)) return fail(400, 'FLOW_BAD_REQUEST', 'limit must be a non-negative whole number');
            const limit = Math.min(Number(limitText) || 50, 200);
            const runs = Array.from(state.runs.values()).map(r => r.run).filter(r => r.flow_id === rec.id && (!mode || r.mode === mode) && (!status || r.status === status));
            runs.sort((a, b) => b.started_at.localeCompare(a.started_at));
            return reply({ runs: runs.slice(0, limit).map(header) });
        }
        if (action === 'test-data') {
            const nodeId = rest[1];
            if (!/^[A-Za-z0-9_-]{1,64}$/.test(nodeId)) return fail(400, 'FLOW_BAD_REQUEST', 'the node id is not valid');
            if (method !== 'GET' && method !== 'PUT') return null;
            // Only an enabled trigger of the draft has sample data.
            if (!triggerNodes(rec.draft).some(n => n.id === nodeId)) return fail(409, 'FLOW_NO_TRIGGER', 'the node is not an enabled trigger of the draft');
            if (method === 'PUT') {
                state.testData[rec.id + '/' + nodeId] = clone(body.data || {});
                return reply({ data: clone(body.data || {}) });
            }
            return reply({ data: sampleOf(entry, nodeId) });
        }
        return null;
    }

    async function route(method, u, body, headers) {
        if (state.disabled) return fail(503, 'FLOWS_DISABLED', 'EasyDrag flows are disabled');
        const parts = u.pathname.replace(/^\/api\/desktop\/flows\/?/, '').split('/').filter(Boolean).map(decodeURIComponent);
        let answer = null;
        if (!parts.length) answer = collection(method, body);
        else if (parts[0] === 'secrets') answer = secrets(method, parts.slice(1), body);
        else if (parts[0] === 'runs') answer = runRoute(method, parts.slice(1));
        else if (parts[0] === 'node-types') answer = nodeTypes(method, parts.slice(1), headers);
        else if (parts[0] === 'templates' && parts.length === 1) answer = method === 'GET' ? reply({ templates: clone(templates) }) : null;
        else if (parts[0] === 'validate' && parts.length === 1) {
            if (method === 'POST') {
                if (body.mode && body.mode !== 'draft' && body.mode !== 'publish') answer = fail(400, 'FLOW_BAD_REQUEST', 'mode must be "draft" or "publish"');
                else if (!body.doc || body.doc.schema !== 1) answer = fail(400, 'FLOW_BAD_REQUEST', 'unsupported flow schema');
                else { const issues = validate(body.doc, body.mode || 'draft'); answer = reply({ valid: !hasErrors(issues), issues }); }
            }
        } else answer = flowRoute(method, parts[0], parts.slice(1), body, u.searchParams);
        if (answer && answer.detail) {
            const out = { run: clone(answer.rec.run), steps: clone(answer.rec.steps) };
            if (u.searchParams.get('include') === 'doc') out.doc = clone(answer.rec.doc);
            return reply(out);
        }
        return answer || unrouted(method, u.pathname + u.search);
    }

    // scheduleCron mirrors flows.ScheduleToCron for the modes of the schedule trigger.
    function scheduleCron(params) {
        const p = params || {};
        const [hour, minute] = String(p.time || '07:00').split(':').map(Number);
        const days = { mon: 1, tue: 2, wed: 3, thu: 4, fri: 5, sat: 6, sun: 0 };
        switch (p.mode || 'daily') {
        case 'interval_minutes': return '*/' + (p.minutes || 15) + ' * * * *';
        case 'interval_hours': return '0 */' + (p.hours || 1) + ' * * *';
        case 'weekdays': return minute + ' ' + hour + ' * * 1-5';
        case 'weekly': return minute + ' ' + hour + ' * * ' + (p.weekdays || []).map(d => days[d]).sort((a, b) => a - b).join(',');
        case 'monthly': return minute + ' ' + hour + ' ' + (p.day || 1) + ' * *';
        case 'cron': return String(p.cron || '');
        default: return minute + ' ' + hour + ' * * *';
        }
    }

    // nextRun is the next local run of a daily or weekdays schedule; other modes get a time 18 h ahead.
    function nextRun(params) {
        const p = params || {};
        if (p.mode !== 'weekdays' && p.mode !== 'daily') return iso(18 * 3600000);
        const [hour, minute] = String(p.time || '07:00').split(':').map(Number);
        const at = new Date();
        at.setHours(hour, minute, 0, 0);
        if (at <= new Date()) at.setDate(at.getDate() + 1);
        while (p.mode === 'weekdays' && (at.getDay() === 0 || at.getDay() === 6)) at.setDate(at.getDate() + 1);
        return at.toISOString();
    }

    // ── /api/missions/v2: the flow missions Mission Control lists (tools.MissionV2) ──
    function flowMission(entry) {
        const r = entry.rec;
        const m = {
            id: r.mission_id, name: r.name, prompt: '', execution_type: 'flow', schedule: '', trigger_type: '', priority: 'medium',
            enabled: entry.enabled, status: 'idle', last_run: ZERO_TIME, last_result: '', last_output: '', run_count: 0,
            created_at: r.created_at, locked: false, runner_type: 'local', flow_id: r.id
        };
        if (r.live) {
            m.flow_published = true;
            m.flow_triggers = triggerNodes(r.live).map(n => n.type === 'trigger.schedule'
                ? { node_id: n.id, trigger_type: 'schedule', schedule: scheduleCron(n.params) }
                : { node_id: n.id, trigger_type: 'manual' });
            // missionPayload adds next_run for an armed timer: a switched-on flow with a schedule.
            const schedule = triggerNodes(r.live).find(n => n.type === 'trigger.schedule');
            if (entry.enabled && schedule) m.next_run = nextRun(schedule.params);
        }
        return m;
    }

    function missions(method, u) {
        if (method === 'GET' && u.pathname === '/api/missions/v2') return reply({ missions: Array.from(state.flows.values()).map(flowMission), queue: { items: [], running: '' } });
        if (method === 'GET' && u.pathname === '/api/missions/v2/history') return reply({ entries: [], total: 0, limit: Number(u.searchParams.get('limit')) || 25, offset: 0 });
        return unrouted(method, u.pathname + u.search);
    }

    const MISSIONS_VIA_DESKTOP = '/api/desktop/integrations/missions/v2';
    window.fetch = async (url, options) => {
        const opts = options || {};
        const u = new URL(String(url), location.origin);
        const method = String(opts.method || 'GET').toUpperCase();
        // Mission Control reaches the mission API through the Desktop integrations route, which the
        // server maps to /api/missions/v2/….
        if (u.pathname === MISSIONS_VIA_DESKTOP || u.pathname.startsWith(MISSIONS_VIA_DESKTOP + '/')) u.pathname = '/api/missions/v2' + u.pathname.slice(MISSIONS_VIA_DESKTOP.length);
        if (u.pathname === '/api/missions/v2' || u.pathname.startsWith('/api/missions/v2/')) return missions(method, u);
        if (u.pathname !== '/api/desktop/flows' && !u.pathname.startsWith('/api/desktop/flows/')) return previousFetch(url, opts);
        state.requests.push(method + ' ' + u.pathname + u.search);
        let body = {};
        try { body = opts.body ? JSON.parse(opts.body) : {}; } catch (err) { return fail(400, 'FLOW_BAD_REQUEST', 'the request body is not valid JSON'); }
        return route(method, u, body || {}, new Headers(opts.headers || {}));
    };

    // The desktop's t() (js/shared/shared-core.js) fills {{name}} and {name}; aurora-fixture's
    // translator only fills {{name}}, and EasyDrag's strings use {name}. Once the fixture is ready,
    // t() works like the production one.
    window.fixtureReady = window.fixtureReady.then(() => {
        const translate = (key, args) => {
            let value = (window.I18N && window.I18N[key]) || key;
            Object.entries(args || {}).forEach(([k, v]) => { value = value.replaceAll('{{' + k + '}}', v).replaceAll('{' + k + '}', v); });
            return value;
        };
        window.t = translate;
        if (window.i18n) window.i18n.t = translate;
    });

    window.edFixture = {
        state, seededID: seeded.id, catalog,
        lastSave: () => state.saves[state.saves.length - 1] || null,
        runs: () => Array.from(state.runs.values()).map(r => r.run),
        flow: id => state.flows.get(id || seeded.id),
        // addDraft adds an unpublished flow with the briefing's steps (Mission Control's second row).
        addDraft: name => addFlow(Object.assign(briefing(), { name }), false, false).id,
        // addWaitingRun adds a live run that waits for a slot (see waitingRun) and returns its id.
        addWaitingRun: flowId => waitingRun(flowId),
        // editor returns the editor state of the first EasyDrag window (null on the start page).
        editor() {
            const inst = window.EasyDragApp && Array.from(window.EasyDragApp._instances.values())[0];
            return inst && inst.screen && inst.screen.ed ? inst.screen.ed : null;
        }
    };
})();
