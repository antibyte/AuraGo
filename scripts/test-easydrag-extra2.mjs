// c1d06 checks: detail view, test runs and publishing. They run on the stub DOM of
// test-easydrag.mjs; that runner calls run(env) with its helpers (check, eq, guardAsync,
// miniDom, ...) and counts the failures.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const MODULES = ['easydrag-core.js', 'easydrag-template.js', 'easydrag-model.js', 'easydrag-fields.js', 'easydrag-forms.js', 'easydrag-mapping.js',
    'easydrag-detail.js', 'easydrag-runs.js', 'easydrag-publish.js'];
const T1 = 'n_tttttttt';
const T2 = 'n_uuuuuuuu';
const A = 'n_aaaaaaaa';
const B = 'n_bbbbbbbb';

// flowDoc is a flow with two manual triggers; the first one feeds alpha -> beta.
function flowDoc() {
    return {
        schema: 1, name: 'Flow',
        nodes: [
            { id: T1, key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} },
            { id: T2, key: 'start_2', type: 'trigger.manual', label: 'Second', position: { x: 0, y: 200 }, params: {}, settings: {} },
            { id: A, key: 'alpha', type: 'web.search', label: 'Alpha', position: { x: 300, y: 0 }, params: { query: 'x' }, settings: {} },
            { id: B, key: 'beta', type: 'web.search', label: 'Beta', position: { x: 600, y: 0 }, params: { query: '' }, settings: {} }
        ],
        edges: [
            { id: 'e1', source: { node: T1, port: 'out' }, target: { node: A, port: 'in' } },
            { id: 'e2', source: { node: A, port: 'out' }, target: { node: B, port: 'in' } }
        ]
    };
}

const apiError = code => Object.assign(new Error('text ' + code), { body: { error: 'text ' + code, code } });

// textareaValue is what a browser shows in the test dialog's textarea: its markup, unescaped.
function textareaValue(html) {
    const m = /data-ed-test-data>([\s\S]*?)<\/textarea>/.exec(html);
    const entities = { '&amp;': '&', '&lt;': '<', '&gt;': '>', '&quot;': '"', '&#39;': "'" };
    return m ? m[1].replace(/&(?:amp|lt|gt|quot|#39);/g, e => entities[e]) : null;
}

export async function run(env) {
    const { apps, types, t, miniDom, eq, guardAsync, settle } = env;

    // harness loads the modules into a sandbox with miniDom (every element keeps the markup it was
    // given in el.html; value, checked and select values act like a browser's), recorded timers and
    // frames, a stub localStorage and a stub EventSource: h.sources lists the streams, es.emit(type,
    // data) delivers a message and es.fail() an error, both only while the stream is open. The api
    // is core.createApi over a transport that records each request in h.requests and answers with
    // answer({url, method, body}) (a throw rejects). Notifications land in h.notes, canvas
    // announcements in h.announced, console.error lines in h.logged.
    function harness(answer, opts) {
        const o = opts || {};
        const dom = miniDom();
        const proto = dom.El.prototype;
        const setHTML = Object.getOwnPropertyDescriptor(proto, 'innerHTML').set;
        Object.defineProperty(proto, 'innerHTML', { configurable: true, set(value) { this.html = String(value); setHTML.call(this, value); } });
        Object.defineProperty(proto, 'value', {
            configurable: true,
            get() {
                if (this.typed !== undefined) return this.typed;
                if (this.localName === 'select') {
                    const options = this.querySelectorAll('option');
                    const chosen = options.find(op => op.attrs.has('selected')) || options[0];
                    return chosen ? chosen.attrs.get('value') : '';
                }
                return this.attrs.get('value') || '';
            },
            set(v) { this.typed = String(v); }
        });
        Object.defineProperty(proto, 'checked', {
            configurable: true,
            get() { return this.ticked !== undefined ? this.ticked : this.attrs.has('checked'); },
            set(v) { this.ticked = !!v; }
        });
        Object.defineProperty(proto, 'tagName', { configurable: true, get() { return String(this.localName).toUpperCase(); } });
        Object.defineProperty(proto, 'style', {
            configurable: true,
            get() { return this.styles; },
            set(value) { this.styles = Object.assign(value, { setProperty(name, v) { this[name] = String(v); } }); }
        });
        proto.removeAttribute = function (name) { this.attrs.delete(name); };
        proto.setSelectionRange = function (start, end) { this.selectionStart = start; this.selectionEnd = end; };
        proto.scrollIntoView = function () {};
        proto.click = function () { this.fire('click'); };
        const docListeners = {};
        Object.assign(dom.document, {
            addEventListener(type, fn) { (docListeners[type] = docListeners[type] || []).push(fn); },
            removeEventListener(type, fn) { const list = docListeners[type] || []; const i = list.indexOf(fn); if (i >= 0) list.splice(i, 1); }
        });
        const timers = new Map();
        const frames = new Map();
        let nextId = 1;
        const store = new Map();
        const logged = [];
        const sources = [];
        class EventSource {
            constructor(url) { this.url = url; this.listeners = {}; this.closed = false; this.onerror = null; sources.push(this); }
            addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
            close() { this.closed = true; }
            emit(type, data) { if (!this.closed) (this.listeners[type] || []).slice().forEach(fn => fn({ type, data: JSON.stringify(data) })); }
            fail() { if (!this.closed && this.onerror) this.onerror({ type: 'error' }); }
        }
        const box = vm.createContext({
            window: { SYSTEM_LANG: 'en' }, navigator: { platform: 'Linux' }, crypto: webcrypto, document: dom.document, EventSource, URLSearchParams,
            console: { log() {}, warn() {}, error: (...args) => { logged.push(args.map(String).join(' ')); } },
            localStorage: { getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); } },
            setTimeout: (fn, ms) => { const id = nextId++; timers.set(id, { fn, ms }); return id; },
            clearTimeout: id => { timers.delete(id); },
            requestAnimationFrame: fn => { const id = nextId++; frames.set(id, fn); return id; },
            cancelAnimationFrame: id => { frames.delete(id); },
            CSS: { escape: value => String(value) },
            ResizeObserver: class { observe() {} disconnect() {} }
        });
        for (const file of MODULES) {
            const full = path.join(apps, file);
            vm.runInContext(fs.readFileSync(full, 'utf8'), box, { filename: full });
        }
        const ED = box.window.EasyDrag;
        const requests = [];
        const transport = (url, init) => {
            const req = { url, method: (init && init.method) || 'GET', body: init && init.body ? JSON.parse(init.body) : undefined };
            requests.push(req);
            return Promise.resolve().then(() => answer(req));
        };
        const catalogTypes = o.types || types;
        const model = ED.model.create(flowDoc(), { types: catalogTypes });
        const bus = ED.core.emitter();
        model.on(change => bus.emit('model', change));
        const notes = [];
        const announced = [];
        const ed = {
            ctx: { notify: n => notes.push(n), confirmDialog: async () => false }, t, esc: ED.core.esc, api: ED.core.createApi(transport),
            catalog: { types: catalogTypes }, windowId: 'w1', readonly: false, root: ED.core.el('<div class="ed-editor"></div>'),
            flow: { id: 'f1', draft_revision: 3, published_draft_revision: 0 }, flowEnabled: false, model, saver: null,
            selection: new Set(), run: null, runView: null, issues: [], lastRunData: null, detail: null, effectsConfirmed: false, bus
        };
        return {
            ED, ed, model, bus, dom, sources, requests, notes, announced, logged, timers, store,
            canvas: { announce: text => announced.push(text) },
            // timer returns the delay of the one pending timer (all delays when there are more).
            timer() { const list = Array.from(timers.values()).map(x => x.ms); return list.length === 1 ? list[0] : list; },
            runTimers() { for (const [id, entry] of Array.from(timers)) { timers.delete(id); entry.fn(); } },
            last: () => sources[sources.length - 1]
        };
    }

    await guardAsync('c1d06 partial publish', async () => {
        const answers = [];
        const h = harness(req => {
            if (req.url === '/api/desktop/flows/f1/publish') return answers.shift();
            if (req.url === '/api/desktop/flows/f1/publish-preview') return { issues: [], effects: [], diff: {} };
            if (req.url === '/api/desktop/flows/f1/enabled') return { enabled: req.body.enabled };
            throw apiError('FLOW_NOT_FOUND');
        });
        const published = [];
        h.bus.on('published', flow => published.push(flow.published_draft_revision));
        const pub = h.ED.publish.create(h.ed);
        const live = rev => ({ id: 'f1', draft_revision: 3, published_draft_revision: rev, live: {} });
        const enables = () => h.requests.filter(r => r.url.endsWith('/enabled')).length;
        answers.push({ flow: live(3), issues: [], partial: true, code: 'FLOW_PUBLISH_INCOMPLETE', error: 'Published, but not every part could be updated.' });
        const ok = await pub.publish(true);
        eq('c1d06 an incomplete publish is live and flagged, warns and does not activate',
            [ok, h.ed.flow.published_draft_revision, published, h.ed.publishIncomplete, enables(), h.notes],
            [true, 3, [3], 'FLOW_PUBLISH_INCOMPLETE', 0, [{ title: 'publish_partial', message: 'error_flow_publish_incomplete', type: 'warning', duration: 12000 }]]);
        // The dialog still offers Publish and says why; publishing the same revision again finishes it.
        const dialog = await pub.openDialog();
        const html = dialog.el.parentNode.html;
        const action = dialog.el.querySelector('[data-ed-action="publish"]');
        eq('c1d06 after an incomplete publish the dialog explains it and offers Publish',
            [html.includes('error_flow_publish_incomplete'), !!action, h.requests.filter(r => r.url.endsWith('/publish')).map(r => r.body.base_revision)], [true, true, [3]]);
        answers.push({ flow: live(3), issues: [] });
        dialog.el.querySelector('[data-ed-activate]').checked = true;
        action.fire('click');
        await settle();
        eq('c1d06 a full publish clears the flag and activates',
            [h.ed.publishIncomplete, enables(), h.ed.flowEnabled, h.notes.slice(1).map(n => n.message)], ['', 1, true, ['publish_done:Flow']]);
        // A missing Mission Control entry: publishing again does not help, so nothing suggests it.
        answers.push({ flow: live(3), issues: [], partial: true, code: 'FLOW_MISSION_MISSING', error: 'Export the flow, delete it and import it again.' });
        h.ed.flowEnabled = false;
        await pub.publish(true);
        const missing = await pub.openDialog();
        const missingHtml = missing.el.parentNode.html;
        eq('c1d06 a missing mission entry is flagged with its own text and no retry hint',
            [h.ed.publishIncomplete, h.notes[h.notes.length - 1].message, enables(), missingHtml.includes('error_flow_mission_missing'), missingHtml.includes('error_flow_publish_incomplete'),
                !!missing.el.querySelector('[data-ed-action="publish"]')],
            ['FLOW_MISSION_MISSING', 'error_flow_mission_missing', 1, true, false, true]);
        // A t without the key returns the key, as the desktop's t does.
        const bare = h.ED.publish.create(Object.assign({}, h.ed, { t: key => key }));
        eq('c1d06 an unknown partial code shows the server text', [bare.partialText('FLOW_SOMETHING_NEW', 'server says'), pub.partialText('FLOW_MISSION_MISSING')], ['server says', 'error_flow_mission_missing']);
        eq('c1d06 the publish checks log no errors', h.logged, []);
    });

    await guardAsync('c1d06 run stream', async () => {
        let fetches = 0;
        const h = harness(req => {
            if (req.url === '/api/desktop/flows/runs/r1') { fetches++; return { run: { id: 'r1', status: 'success', mode: 'test', started_at: '2026-10-06T10:00:00Z' }, steps: [] }; }
            throw apiError('FLOW_RUN_NOT_FOUND');
        });
        const runs = h.ED.runs.create(h.ed, h.canvas);
        runs.attach('r1', { mode: 'test' });
        const first = h.sources[0];
        first.emit('snapshot', { run: { id: 'r1', status: 'running' }, steps: [] });
        first.emit('event', { seq: 3, type: 'step_started', node_id: A });
        first.emit('resync', { after: 5 });
        await settle();
        eq('c1d06 resync reconnects at once after the larger seq and does not finish',
            [first.url, first.closed, h.sources.length, h.last().url, h.timers.size, fetches, h.ed.run.status],
            ['/api/desktop/flows/runs/r1/events', true, 2, '/api/desktop/flows/runs/r1/events?after=5', 0, 0, 'running']);
        h.last().emit('resync', { after: 2 });
        eq('c1d06 resync never moves the start point back', h.last().url, '/api/desktop/flows/runs/r1/events?after=5');
        // Refused (429 FLOW_RUN_LIMIT) or broken streams back off; never a tight loop.
        const delays = [];
        for (let i = 0; i < 6; i++) { h.last().fail(); delays.push(h.timer()); h.runTimers(); }
        eq('c1d06 failed streams retry after 1.5 s, doubling up to 15 s', [delays, h.sources.length, h.last().url], [[1500, 3000, 6000, 12000, 15000, 15000], 9, '/api/desktop/flows/runs/r1/events?after=5']);
        h.last().emit('event', { seq: 6, type: 'step_started', node_id: B });
        h.last().fail();
        const reset = h.timer();
        h.runTimers();
        eq('c1d06 a received event resets the backoff and moves the start point', [reset, h.last().url], [1500, '/api/desktop/flows/runs/r1/events?after=6']);
        h.last().emit('end', {});
        await settle();
        eq('c1d06 end loads the stored result', [fetches, h.ed.run.status, h.announced, h.timers.size], [1, 'success', ['run_done'], 0]);
        // A stream that breaks after run_finished but before "end" loads the result too.
        runs.attach('r1', { mode: 'test' });
        h.last().emit('event', { seq: 9, type: 'run_finished', run: { status: 'success', duration_ms: 5 } });
        h.last().fail();
        await settle();
        eq('c1d06 a stream lost after run_finished loads the result instead of retrying', [fetches, h.timers.size, h.announced.length], [2, 0, 2]);
        eq('c1d06 the stream checks log no errors', h.logged, []);
    });

    await guardAsync('c1d06 cancel', async () => {
        let stored = 'running';
        const h = harness(req => {
            if (req.url === '/api/desktop/flows/runs/r1/cancel' || req.url === '/api/desktop/flows/runs/r3/cancel') throw apiError('FLOW_RUN_FINISHED');
            if (req.url === '/api/desktop/flows/runs/r2/cancel') throw apiError('FLOW_RUN_NOT_FOUND');
            if (req.url === '/api/desktop/flows/runs/r1') return { run: { id: 'r1', status: 'cancelled', mode: 'test', started_at: '2026-10-06T10:00:00Z' }, steps: [] };
            if (req.url === '/api/desktop/flows/runs/r3') return { run: { id: 'r3', status: stored, mode: 'test', started_at: '2026-10-06T10:00:00Z' }, steps: [] };
            throw apiError('FLOW_RUN_NOT_FOUND');
        });
        const runs = h.ED.runs.create(h.ed, h.canvas);
        runs.attach('r1', { mode: 'test' });
        h.last().emit('snapshot', { run: { id: 'r1', status: 'running' }, steps: [] });
        await runs.cancel();
        await settle();
        eq('c1d06 a cancel of a run that already ended shows its result without an error',
            [h.ed.run.status, h.notes, h.sources[0].closed, runs.isRunning(), h.requests.filter(r => r.url === '/api/desktop/flows/runs/r1').length], ['cancelled', [], true, false, 1]);
        // While the stored run is not final yet, the stream stays and brings the end.
        runs.attach('r3', { mode: 'test' });
        const r3 = h.last();
        r3.emit('snapshot', { run: { id: 'r3', status: 'running' }, steps: [] });
        await runs.cancel();
        await settle();
        eq('c1d06 a cancel answered FLOW_RUN_FINISHED keeps the stream while the stored run is not final', [r3.closed, h.ed.run.status, h.notes], [false, 'running', []]);
        runs.attach('r2', { mode: 'test' });
        h.last().emit('snapshot', { run: { id: 'r2', status: 'running' }, steps: [] });
        await runs.cancel();
        eq('c1d06 a cancel of an unknown run keeps the error', h.notes.map(n => [n.message, n.type]), [['error_flow_run_not_found', 'error']]);
        eq('c1d06 the cancel checks log no errors', h.logged, []);
    });

    await guardAsync('c1d06 test data', async () => {
        const samples = { [T1]: { name: 'Ada', token: '[redacted]' }, [T2]: { other: 1 } };
        let runId = 0;
        const h = harness(req => {
            const m = /^\/api\/desktop\/flows\/f1\/test-data\/(.+)$/.exec(req.url);
            if (m) return { data: samples[m[1]] };
            if (req.url === '/api/desktop/flows/f1/test') return { run_id: 'r' + (++runId) };
            throw apiError('FLOW_RUN_NOT_FOUND');
        });
        const runs = h.ED.runs.create(h.ed, h.canvas);
        const tests = () => h.requests.filter(r => r.url === '/api/desktop/flows/f1/test').map(r => r.body);
        // open shows the dialog and gives its textarea the value a browser would.
        async function open() {
            const dialog = await runs.startTest({});
            const area = dialog.body.querySelector('[data-ed-test-data]');
            area.value = textareaValue(dialog.el.parentNode.html);
            return { dialog, area, select: dialog.body.querySelector('[data-ed-test-trigger]') };
        }
        async function runIt(dialog) {
            dialog.el.querySelector('[data-ed-action="run"]').fire('click');
            await settle();
        }
        const a = await open();
        const shown = a.area.value;
        await runIt(a.dialog);
        const b = await open();
        b.area.value = '{"name": "Grace"}';
        await runIt(b.dialog);
        const c = await open();
        c.area.value = JSON.stringify({ name: 'Linus', token: '[redacted]' });
        await runIt(c.dialog);
        eq('c1d06 unchanged test data runs with the stored sample; edits are remembered unless they hold [redacted]',
            [shown, tests(), h.notes],
            [JSON.stringify(samples[T1], null, 2), [
                { trigger_node: T1, remember_data: false },
                { trigger_node: T1, remember_data: true, trigger_data: { name: 'Grace' } },
                { trigger_node: T1, remember_data: false, trigger_data: { name: 'Linus', token: '[redacted]' } }
            ], [{ title: 'test_title', message: 'test_data_redacted' }]]);
        // Another trigger shows its own sample; unchanged, it runs with that trigger's stored one.
        const d = await open();
        d.select.value = T2;
        d.select.fire('change');
        await settle();
        const switched = d.area.value;
        await runIt(d.dialog);
        const e = await open();
        e.area.value = '{"mine": true}';
        e.select.value = T1;
        e.select.fire('change');
        await settle();
        eq('c1d06 picking another trigger loads its sample unless the text was edited',
            [switched, tests()[3], e.area.value], [JSON.stringify(samples[T2], null, 2), { trigger_node: T2, remember_data: false }, '{"mine": true}']);
        eq('c1d06 the test data checks log no errors', h.logged, []);
    });

    await guardAsync('c1d06 detail run view', async () => {
        const detailTypes = new Map(types);
        detailTypes.set('web.search', Object.assign({}, types.get('web.search'), { params: [{ name: 'query', kind: 'text', label: 'Query', templatable: true }] }));
        const h = harness(() => { throw apiError('FLOW_NOT_FOUND'); }, { types: detailTypes });
        h.ed.lastRunData = { runId: 'r1', label: 'Last run', roots: { alpha: { results: [{ title: 'A' }] } } };
        const rows = () => h.ed.root.querySelectorAll('.ed-tree-row');
        const alphaRow = () => rows().find(r => r.getAttribute('data-ref') === 'alpha.results');
        // Editing: rows drag, and a click inserts into the first template field.
        h.ED.detail.open(h.ed, B);
        const editDrag = rows().map(r => r.getAttribute('draggable'));
        alphaRow().fire('click');
        const inserted = h.model.node(B).params.query;
        h.ED.detail.close(h.ed);
        h.runTimers();
        // Run view: the same tree shows the run's data but inserts nothing.
        h.ed.runView = { run: { id: 'r1' }, doc: flowDoc() };
        h.ed.run = { id: 'r1', status: 'success', mode: 'test', steps: new Map(), record: null };
        h.ED.detail.open(h.ed, B);
        const version = h.model.version;
        alphaRow().fire('click');
        alphaRow().fire('keydown', { key: 'Enter' });
        eq('c1d06 the run view input tree is neither draggable nor inserts',
            [editDrag.length > 0 && editDrag.every(v => v === 'true'), inserted, rows().length > 0 && rows().every(r => r.getAttribute('draggable') === 'false'), h.model.version === version, h.model.node(B).params.query],
            [true, '{{alpha.results}}', true, true, '{{alpha.results}}']);
        h.ED.detail.close(h.ed);
        eq('c1d06 the detail checks log no errors', h.logged, []);
    });
}
