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
const deferred = () => { let resolve, reject; const p = new Promise((a, b) => { resolve = a; reject = b; }); return { p, resolve, reject }; };

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
    // announcements in h.announced, console.error lines in h.logged. opts: types (the catalog),
    // doc (the flow, default flowDoc()). The page's origin is https://aurago.test.
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
            window: { SYSTEM_LANG: 'en', location: { origin: 'https://aurago.test' } }, navigator: { platform: 'Linux' }, crypto: webcrypto, document: dom.document, EventSource, URLSearchParams, URL,
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
        const model = ED.model.create(o.doc || flowDoc(), { types: catalogTypes });
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
            ED, ed, model, bus, dom, sources, requests, notes, announced, logged, timers, store, win: box.window, docListeners,
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
        let stored = 'running';
        const h = harness(req => {
            if (req.url === '/api/desktop/flows/runs/r1') { fetches++; return { run: { id: 'r1', status: stored, mode: 'test', started_at: '2026-10-06T10:00:00Z' }, steps: [] }; }
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
        // Refused (429 FLOW_RUN_LIMIT) or broken streams back off; never a tight loop. The fifth
        // failure in a row asks the server about the run (still running: keep reconnecting).
        const delays = [];
        for (let i = 0; i < 6; i++) { h.last().fail(); await settle(); delays.push(h.timer()); h.runTimers(); }
        eq('c1d06 failed streams retry after 1.5 s, doubling up to 15 s', [delays, h.sources.length, h.last().url, fetches], [[1500, 3000, 6000, 12000, 15000, 15000], 9, '/api/desktop/flows/runs/r1/events?after=5', 1]);
        h.last().emit('event', { seq: 6, type: 'step_started', node_id: B });
        h.last().fail();
        const reset = h.timer();
        h.runTimers();
        eq('c1d06 a received event resets the backoff and moves the start point', [reset, h.last().url], [1500, '/api/desktop/flows/runs/r1/events?after=6']);
        stored = 'success';
        h.last().emit('end', {});
        await settle();
        eq('c1d06 end loads the stored result', [fetches, h.ed.run.status, h.announced, h.timers.size], [2, 'success', ['run_done'], 0]);
        // A stream that breaks after run_finished but before "end" loads the result too.
        runs.attach('r1', { mode: 'test' });
        h.last().emit('event', { seq: 9, type: 'run_finished', run: { status: 'success', duration_ms: 5 } });
        h.last().fail();
        await settle();
        eq('c1d06 a stream lost after run_finished loads the result instead of retrying', [fetches, h.timers.size, h.announced.length], [3, 0, 2]);
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
        // runIt starts the run; the next dialog opens once it is gone (one run at a time).
        async function runIt(dialog) {
            dialog.el.querySelector('[data-ed-action="run"]').fire('click');
            await settle();
            runs.clearRun();
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

    // ── review fixes: security and data integrity ──

    const outputRun = output => ({ id: 'r1', status: 'success', mode: 'test', steps: new Map([[A, { node_id: A, status: 'success', output }]]), record: null });

    await guardAsync('c1d06 review hostile output', async () => {
        const h = harness(() => { throw apiError('FLOW_NOT_FOUND'); });
        eq('c1d06 file links allow only same-origin paths under /files/',
            ['javascript:alert(1)', '//evil.example/files/x.png', 'https://evil.example/files/x.png', '/files/../x', '/files/%2e%2e/x', '/\\evil.example/files/x', 'data:image/png;base64,AA', '/api/x', '', null,
                '/files/a b.pdf', 'https://aurago.test/files/ok.png', '/files/doc.pdf?v=2#p'].map(h.ED.detail.safeFileUrl),
            ['', '', '', '', '', '', '', '', '', '', '/files/a%20b.pdf', '/files/ok.png', '/files/doc.pdf?v=2']);
        // Step output is untrusted (web pages, webhooks, model text): file objects can be forged.
        h.ed.run = outputRun({
            a: { $type: 'file', name: 'x.pdf', mime: 'application/pdf', web_path: 'javascript:alert(document.domain)//' },
            b: { $type: 'file', name: 'p.png', mime: 'image/png', web_path: 'https://tracker.example/p.png' },
            c: { $type: 'file', name: 'q.mp3', mime: 'audio/mpeg', web_path: '//evil.example/files/q.mp3' },
            d: { $type: 'file', name: 'r.png', mime: 'image/png', web_path: '/files/../etc/r.png' },
            ok: { $type: 'file', name: 'ok.png', mime: 'image/png', web_path: '/files/ok.png' },
            pdf: { $type: 'file', name: 'doc.pdf', mime: 'application/pdf', web_path: '/files/doc.pdf' }
        });
        h.ED.detail.open(h.ed, A);
        await settle();
        const out = h.ed.root.querySelector('.ed-output');
        eq('c1d06 forged file objects get no links, media or frames; without pdf.js a PDF keeps its link only',
            [out.querySelectorAll('a').map(a => a.getAttribute('href')), out.querySelectorAll('img').map(i => i.getAttribute('src')), out.querySelectorAll('audio, iframe, canvas').length, out.querySelectorAll('.ed-file-card').length],
            [['/files/ok.png', '/files/doc.pdf'], ['/files/ok.png'], 0, 6]);
        eq('c1d06 the hostile output checks log no errors', h.logged, []);
    });

    await guardAsync('c1d06 review deep output', async () => {
        const h = harness(() => { throw apiError('FLOW_NOT_FOUND'); });
        let list = 1;
        let obj = 'x';
        for (let i = 0; i < 10000; i++) { list = [list]; obj = { a: obj, file: { $type: 'file', name: 'f' + i, web_path: '/files/f' + i } }; }
        h.ed.run = outputRun({ list, obj });
        const detail = h.ED.detail.open(h.ed, A);
        const tree = h.ed.root.querySelector('.ed-output').html;
        const cards = h.ed.root.querySelectorAll('.ed-file-card').length;
        h.ed.root.querySelector('[data-ed-view="json"]').fire('click');
        const json = h.ed.root.querySelector('.ed-output').html;
        eq('c1d06 output nested 10000 levels deep renders, cut at 24 levels',
            [!!detail, h.ed.detail === detail, (tree.match(/<details/g) || []).length < 100, tree.includes('ed-json-more'), cards < 30, json.includes('…')], [true, true, true, true, true, true]);
        h.ED.detail.close(h.ed);
        h.runTimers();
        // A render that throws closes the overlay instead of leaving it stuck.
        const render = h.ED.forms.render;
        h.ED.forms.render = () => { throw new Error('boom'); };
        const failed = h.ED.detail.open(h.ed, B);
        h.ED.forms.render = render;
        h.runTimers();
        eq('c1d06 a detail view that fails to render closes and leaves no overlay',
            [failed, h.ed.detail, h.ed.root.querySelectorAll('.ed-detail-backdrop').length, h.logged.length], [null, null, 0, 1]);
    });

    await guardAsync('c1d06 review notes', async () => {
        const h = harness(() => { throw apiError('FLOW_NOT_FOUND'); });
        h.ED.detail.open(h.ed, A);
        const root = h.ed.root;
        const note = () => root.querySelector('[data-pane="note"] textarea');
        root.querySelector('[data-ed-pane="note"]').fire('click');
        note().value = 'for alpha';
        note().fire('input');
        root.querySelector('[data-ed-nav="1"]').fire('click');
        const afterNav = [h.ed.detail.nodeId(), h.model.node(A).settings.notes, h.model.node(B).settings.notes];
        note().value = 'for beta';
        note().fire('input');
        h.ED.detail.close(h.ed);
        h.runTimers();
        eq('c1d06 a note typed before moving on or closing is saved on its own node',
            [afterNav, h.model.node(A).settings.notes, h.model.node(B).settings.notes], [[B, 'for alpha', undefined], 'for alpha', 'for beta']);
    });

    await guardAsync('c1d06 review finish race', async () => {
        const late = deferred();
        const slow = deferred();
        let r3Fetches = 0;
        const h = harness(req => {
            if (req.url === '/api/desktop/flows/runs/r1') return late.p;
            if (req.url === '/api/desktop/flows/runs/r3/cancel') throw apiError('FLOW_RUN_FINISHED');
            if (req.url === '/api/desktop/flows/runs/r3') return ++r3Fetches === 1 ? slow.p : { run: { id: 'r3', status: 'cancelled', mode: 'test', started_at: '2026-10-06T10:00:00Z' }, steps: [] };
            throw apiError('FLOW_RUN_NOT_FOUND');
        });
        const runs = h.ED.runs.create(h.ed, h.canvas);
        runs.attach('r1', { mode: 'test' });
        h.last().emit('end', {});
        await settle();
        runs.attach('r2', { mode: 'test' });
        h.last().emit('snapshot', { run: { id: 'r2', status: 'running' }, steps: [] });
        late.resolve({ run: { id: 'r1', status: 'error', mode: 'test', started_at: '2026-10-06T10:00:00Z' }, steps: [{ node_id: A, status: 'error', error_code: 'X' }] });
        await settle();
        eq('c1d06 an older run\'s late result leaves the current run alone',
            [h.ed.run.id, h.ed.run.status, h.ed.run.record && h.ed.run.record.id, h.ed.run.steps.has(A), h.ed.lastRunData, h.announced, runs.isRunning()], ['r2', 'running', 'r2', false, null, [], true]);
        // "end" and a cancel answered FLOW_RUN_FINISHED both finish r3: one result, one announcement.
        runs.attach('r3', { mode: 'test' });
        h.last().emit('snapshot', { run: { id: 'r3', status: 'running' }, steps: [] });
        h.last().emit('end', {});
        await runs.cancel();
        slow.resolve({ run: { id: 'r3', status: 'cancelled', mode: 'test', started_at: '2026-10-06T10:00:00Z' }, steps: [] });
        await settle();
        eq('c1d06 a run finishes once when "end" races a finished cancel', [h.ed.run.status, h.announced, h.notes], ['cancelled', ['run_failed'], []]);
        eq('c1d06 the finish race checks log no errors', h.logged, []);
    });

    await guardAsync('c1d06 review one run at a time', async () => {
        const h = harness(req => {
            if (/\/test-data\//.test(req.url)) return { data: {} };
            if (req.url === '/api/desktop/flows/f1/test' || req.url === '/api/desktop/flows/f1/run') return { run_id: 'r9' };
            throw apiError('FLOW_RUN_NOT_FOUND');
        });
        const runs = h.ED.runs.create(h.ed, h.canvas);
        const posts = () => h.requests.filter(r => r.method === 'POST').length;
        runs.attach('r1', { mode: 'test' });
        h.last().emit('snapshot', { run: { id: 'r1', status: 'running' }, steps: [] });
        h.ED.detail.open(h.ed, A);
        const button = () => h.ed.root.querySelector('[data-ed-detail-test]');
        const whileRunning = [await runs.startTest({ onlyNode: A, quick: true }), await runs.startTest({}), posts(), button().disabled];
        await runs.runLive();
        h.last().emit('event', { seq: 1, type: 'run_finished', run: { status: 'success' } });
        const afterRun = [button().disabled, posts(), h.ed.run.id];
        h.ED.detail.close(h.ed);
        // A second start while the first one's dialog is open is ignored.
        const first = runs.startTest({});
        const second = await runs.startTest({});
        const dialog = await first;
        dialog.el.querySelector('[data-ed-action="cancel"]').fire('click');
        await settle();
        const again = await runs.startTest({});
        eq('c1d06 no test or live run starts while a run or a test dialog is active',
            [whileRunning, afterRun, second, !!dialog, !!again], [[undefined, undefined, 0, true], [false, 0, 'r1'], undefined, true, true]);
    });

    await guardAsync('c1d06 review effects scope', async () => {
        const fxTypes = new Map(types);
        fxTypes.set('web.search', Object.assign({}, types.get('web.search'), { effects: ['sends_message'] }));
        fxTypes.set('files.delete', { type: 'files.delete', label: 'Delete', inputs: ['in'], outputs: ['out'], params: [], effects: ['deletes'] });
        const doc = flowDoc();
        doc.nodes[3].type = 'files.delete';
        const h = harness(req => {
            if (/\/test-data\//.test(req.url)) return { data: {} };
            if (req.url === '/api/desktop/flows/f1/test') return { run_id: 'r' + h.requests.length };
            throw apiError('FLOW_RUN_NOT_FOUND');
        }, { types: fxTypes, doc });
        const runs = h.ED.runs.create(h.ed, h.canvas);
        const html = d => d.el.parentNode.html;
        const tests = () => h.requests.filter(r => r.url === '/api/desktop/flows/f1/test').length;
        // Testing one step asks for its own effect only; "remember" stores that effect as a list.
        const one = await runs.startTest({ onlyNode: A });
        const oneHtml = html(one);
        one.body.querySelector('[data-ed-effects-skip]').checked = true;
        one.el.querySelector('[data-ed-action="run"]').fire('click');
        await settle();
        runs.clearRun();
        const stored = h.store.get('aurago.easydrag.effects-ok.f1');
        const quickStep = await runs.startTest({ onlyNode: A, quick: true });
        runs.clearRun();
        // The whole flow has another effect: it still asks (also for a quick test), for that one only.
        const all = await runs.startTest({ quick: true });
        const allHtml = all ? html(all) : '';
        all.el.querySelector('[data-ed-action="cancel"]').fire('click');
        await settle();
        eq('c1d06 confirming one step\'s effects covers those effects only',
            [oneHtml.includes('effect_sends_message'), oneHtml.includes('effect_deletes'), stored, quickStep, tests(), !!all, allHtml.includes('effect_deletes'), allHtml.includes('effect_sends_message')],
            [true, false, '["sends_message"]', undefined, 2, true, true, false]);
        // An older stored `true` (one switch for the whole flow) confirms nothing.
        h.store.set('aurago.easydrag.effects-ok.f1', 'true');
        h.ed.effectsConfirmed = false;
        const old = await runs.startTest({ onlyNode: A, quick: true });
        eq('c1d06 an old flow-wide confirmation asks again', [!!old, old ? html(old).includes('effect_sends_message') : false], [true, true]);
        eq('c1d06 the effects checks log no errors', h.logged, []);
    });

    // ── review fixes: performance and leaks ──

    // counting wraps the innerHTML setter of node and counts its rebuilds.
    function counting(node) {
        const set = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(node), 'innerHTML').set;
        const counter = { n: 0 };
        Object.defineProperty(node, 'innerHTML', { configurable: true, set(v) { counter.n++; set.call(this, v); } });
        return counter;
    }

    await guardAsync('c1d06 review run events', async () => {
        const h = harness(() => { throw apiError('FLOW_NOT_FOUND'); });
        const rows = Array.from({ length: 200 }, (_, i) => Object.fromEntries(Array.from({ length: 40 }, (_, j) => ['f' + j, 'v' + i + '_' + j])));
        h.ed.run = outputRun({ items: rows });
        h.ed.run.status = 'running';
        h.ed.lastRunData = { runId: 'r0', label: 'Last run', roots: { alpha: { items: [] } } };
        h.ED.detail.open(h.ed, A);
        const out = h.ed.root.querySelector('.ed-output');
        const lis = (out.html.match(/<li/g) || []).length;
        const outputs = counting(out);
        const trees = counting(h.ed.root.querySelector('.ed-tree'));
        // Events of other steps change neither this step nor the run data.
        for (let i = 0; i < 20; i++) {
            h.ed.run.steps.set(B, { node_id: B, status: 'running' });
            h.bus.emit('run', h.ed.run);
        }
        const quiet = [outputs.n, trees.n];
        out.scrollTop = 120;
        h.ed.run.steps.set(A, { node_id: A, status: 'success', output: { items: rows.slice(0, 2) } });
        h.bus.emit('run', h.ed.run);
        const newStep = [outputs.n, out.scrollTop];
        h.ed.lastRunData = { runId: 'r1', label: 'Last run', roots: { alpha: { items: [1] } } };
        h.bus.emit('last-run', {});
        eq('c1d06 run events redraw the output only for a new step of the node, the input only for new run data',
            [lis <= 1700, quiet, newStep, trees.n], [true, [0, 0], [1, 120], 1]);
        h.ED.detail.close(h.ed);
        eq('c1d06 the run event checks log no errors', h.logged, []);
    });

    await guardAsync('c1d06 review pdf thumbnails', async () => {
        const h = harness(() => { throw apiError('FLOW_NOT_FOUND'); });
        h.dom.El.prototype.getContext = () => ({});
        h.dom.El.prototype.toDataURL = () => 'data:image/png;base64,UERG';
        const pdf = { created: 0, destroyed: 0 };
        h.win.pdfjsLib = {
            getDocument: ({ url }) => {
                pdf.created++;
                const page = { getViewport: () => ({ width: 10, height: 20 }), render: () => ({ promise: Promise.resolve() }) };
                return { promise: url.includes('broken') ? Promise.reject(new Error('bad pdf')) : Promise.resolve({ getPage: async () => page }), destroy: () => { pdf.destroyed++; return Promise.resolve(); } };
            }
        };
        h.ed.run = outputRun({ doc: { $type: 'file', name: 'doc.pdf', mime: 'application/pdf', web_path: '/files/doc.pdf' }, bad: { $type: 'file', name: 'b.pdf', mime: 'application/pdf', web_path: '/files/broken.pdf' } });
        h.ED.detail.open(h.ed, A);
        await settle();
        const thumbs = () => h.ed.root.querySelectorAll('img[data-pdf]').map(i => [i.getAttribute('data-pdf'), i.src, i.hidden]);
        const first = [thumbs(), pdf.created, pdf.destroyed];
        h.ed.root.querySelector('[data-ed-view="table"]').fire('click');
        h.ed.root.querySelector('[data-ed-view="tree"]').fire('click');
        await settle();
        eq('c1d06 pdf.js renders each thumbnail once and always destroys its loading task; a failed one is retried',
            [first, [thumbs(), pdf.created, pdf.destroyed]],
            [[[['/files/doc.pdf', 'data:image/png;base64,UERG', false]], 2, 2], [[['/files/doc.pdf', 'data:image/png;base64,UERG', false]], 3, 3]]);
        h.ED.detail.close(h.ed);
        eq('c1d06 the thumbnail checks log no errors', h.logged, []);
    });

    // ── review fixes: run view data and minors ──

    const finalRun = (id, status) => ({ run: { id, status, mode: 'test', started_at: '2026-10-06T10:00:00Z' }, steps: [] });

    await guardAsync('c1d06 review run view', async () => {
        const h = harness(() => { throw apiError('FLOW_RUN_NOT_FOUND'); });
        const runs = h.ED.runs.create(h.ed, h.canvas);
        h.ed.lastRunData = { runId: 'r9', label: 'Editor run', roots: { alpha: { from: 'editor-run' } } };
        runs.attach('r1', { mode: 'live' });
        const live = h.last();
        live.emit('snapshot', { run: { id: 'r1', status: 'running' }, steps: [] });
        // The editor's enterRunView sets ed.runView, then calls applyRunView.
        const detail = { run: { id: 'r0', status: 'success', mode: 'test', started_at: '2026-10-06T09:00:00Z' }, steps: [{ node_id: A, node_key: 'alpha', status: 'success', output: { from: 'viewed-run' } }], doc: flowDoc() };
        h.ed.runView = { run: detail.run, doc: detail.doc };
        runs.applyRunView(detail);
        const inView = [live.closed, h.ed.run.id, runs.isRunning(), h.ed.runView.data.roots.alpha];
        h.ED.detail.open(h.ed, B);
        const tree = h.ed.root.querySelector('.ed-tree').html;
        const focus = h.dom.document.activeElement;
        h.ED.detail.close(h.ed);
        // The editor's exitRunView clears ed.runView, then calls clearRun: the live run comes back.
        h.ed.runView = null;
        runs.clearRun();
        eq('c1d06 the run view shows its own run\'s data and parks a live run that clearRun attaches again',
            [inView, tree.includes('viewed-run'), tree.includes('editor-run'), !!(focus && focus.getAttribute('data-ed-detail-close') !== null), h.ed.run.id, h.last() !== live, h.last().url, runs.isRunning()],
            [[true, 'r0', false, { from: 'viewed-run' }], true, false, true, 'r1', true, '/api/desktop/flows/runs/r1/events', true]);
        eq('c1d06 the run view checks log no errors', h.logged, []);
    });

    await guardAsync('c1d06 review lost streams', async () => {
        const h = harness(req => {
            if (req.url === '/api/desktop/flows/runs/r1') throw Object.assign(apiError('FLOW_RUN_NOT_FOUND'), { status: 404 });
            if (req.url === '/api/desktop/flows/runs/r2') throw new Error('network');
            throw apiError('FLOW_RUN_NOT_FOUND');
        });
        const runs = h.ED.runs.create(h.ed, h.canvas);
        // failFive fails the stream five times, reconnecting after each of the first four.
        const failFive = async () => { for (let i = 0; i < 5; i++) { if (i) h.runTimers(); h.last().fail(); await settle(); } };
        runs.attach('r1', { mode: 'test' });
        await failFive();
        const gone = [h.sources.length, h.timers.size, h.last().closed, runs.isRunning(), h.ed.run.stale, h.notes];
        runs.attach('r2', { mode: 'test' });
        await failFive();
        eq('c1d06 a stream failing five times asks for the run: unknown stops and says so, a network error keeps retrying',
            [gone, h.timers.size, runs.isRunning(), h.notes.length],
            [[5, 0, true, false, true, [{ title: 'run_stream_lost', message: 'error_flow_run_not_found', type: 'error' }]], 1, true, 1]);
    });

    await guardAsync('c1d06 review test dialog', async () => {
        const doc = flowDoc();
        doc.nodes[1].label = '';
        doc.nodes[2].label = '';
        const h = harness(req => {
            if (/\/test-data\//.test(req.url)) return { data: {} };
            if (req.url === '/api/desktop/flows/runs/r1') return { run: { id: 'r1', status: 'error', mode: 'test', started_at: '2026-10-06T10:00:00Z' }, steps: [{ node_id: A, status: 'error', error_code: 'X' }] };
            throw apiError('FLOW_RUN_NOT_FOUND');
        }, { doc });
        const runs = h.ED.runs.create(h.ed, h.canvas);
        // M1: a draft that does not save is not tested in its last saved version.
        h.ed.saver = { flush: async () => false };
        const refused = await runs.startTest({});
        h.ed.saver = { flush: async () => true };
        const dialog = await runs.startTest({});
        const html = dialog.el.parentNode.html;
        dialog.el.querySelector('[data-ed-action="cancel"]').fire('click');
        await settle();
        // M6: unlabelled steps read as their type.
        runs.attach('r1', { mode: 'test' });
        h.last().emit('end', {});
        await settle();
        eq('c1d06 the test dialog refuses an unsaved draft, names unlabelled triggers and alerts errors; failures name unlabelled steps',
            [refused, h.notes, html.includes('>trigger.manual</option>'), html.includes('class="ed-error" role="alert"'), h.announced],
            [undefined, [{ title: 'test_title', message: 'test_unsaved', type: 'error' }], true, true, ['run_failed_at:web.search']]);
    });

    await guardAsync('c1d06 review publish dialog and issues', async () => {
        const preview = deferred();
        let previews = 0;
        const h = harness(req => {
            if (req.url === '/api/desktop/flows/f1/publish-preview') { previews++; return previews === 1 ? preview.p : { issues: [], effects: [], diff: {} }; }
            throw apiError('FLOW_NOT_FOUND');
        });
        const pub = h.ED.publish.create(h.ed);
        // M2: clicks while the dialog loads or is open do nothing.
        const one = pub.openDialog();
        const two = await pub.openDialog();
        preview.resolve({ issues: [], effects: [], diff: {} });
        const dialog = await one;
        const three = await pub.openDialog();
        dialog.el.querySelector('[data-ed-action="cancel"]').fire('click');
        await settle();
        const four = await pub.openDialog();
        const opens = [two, !!dialog, three, !!four, previews];
        four.el.querySelector('[data-ed-action="cancel"]').fire('click');
        await settle();
        // M5: Escape closes the issues popover and gives focus back to its anchor.
        const anchor = h.ed.root.appendChild(h.ED.core.el('<button type="button">issues</button>'));
        anchor.focus();
        h.ed.issues = [{ severity: 'error', code: 'X', node_id: A }];
        pub.openIssues(anchor);
        const pop = h.ed.root.querySelector('.ed-issues-popover');
        const focusedInside = pop.contains(h.dom.document.activeElement);
        pop.querySelector('button').fire('keydown', { key: 'Escape' });
        h.runTimers();
        eq('c1d06 the publish dialog opens once; Escape closes the issues popover and returns focus',
            [opens, focusedInside, h.ed.root.querySelector('.ed-issues-popover'), h.dom.document.activeElement === anchor, (h.docListeners.pointerdown || []).length],
            [[undefined, true, undefined, true, 2], true, null, true, 0]);
    });

    await guardAsync('c1d06 review drawer', async () => {
        const slow = deferred();
        let entered = [];
        const h = harness(req => {
            if (req.url === '/api/desktop/flows/f1/runs?limit=50') return slow.p;
            if (req.url === '/api/desktop/flows/f1/runs?limit=50&status=error') return { runs: [{ id: 'r_new', status: 'error', mode: 'test', started_at: '2026-10-06T10:00:00Z' }] };
            if (req.url === '/api/desktop/flows/runs/r_new?include=doc') return Object.assign(finalRun('r_new', 'error'), { doc: flowDoc() });
            throw apiError('FLOW_RUN_NOT_FOUND');
        });
        h.ed.enterRunView = d => entered.push(d.run.id);
        const runs = h.ED.runs.create(h.ed, h.canvas);
        const opener = h.ed.root.appendChild(h.ED.core.el('<button type="button">runs</button>'));
        opener.focus();
        runs.toggleDrawer(true);
        const drawer = h.ed.root.querySelector('.ed-drawer');
        const focusIn = h.dom.document.activeElement && h.dom.document.activeElement.getAttribute('data-ed-runs-filter');
        // M4: the answer to an older filter does not overwrite the newer one.
        drawer.querySelector('[data-ed-runs-filter="errors"]').fire('click');
        await settle();
        slow.resolve({ runs: [{ id: 'r_old', status: 'success', mode: 'live', started_at: '2026-10-06T09:00:00Z' }] });
        await settle();
        const listed = drawer.querySelector('.ed-runs-list').html;
        // M4: a double click on a run fetches it once.
        const row = drawer.querySelector('[data-ed-run="r_new"]');
        row.fire('click');
        row.fire('click');
        await settle();
        const fetches = h.requests.filter(r => r.url === '/api/desktop/flows/runs/r_new?include=doc').length;
        drawer.querySelector('[data-ed-drawer-close]').focus();
        drawer.querySelector('[data-ed-drawer-close]').fire('click');
        eq('c1d06 the drawer takes and returns focus, shows the latest filter and opens a run once',
            [focusIn, listed.includes('r_new'), listed.includes('r_old'), fetches, entered, h.dom.document.activeElement === opener, runs.drawerOpen()],
            ['all', true, false, 1, ['r_new'], true, false]);
    });

    await guardAsync('c1d06 review detail keyboard and names', async () => {
        const h = harness(() => { throw apiError('FLOW_NOT_FOUND'); });
        h.ED.detail.open(h.ed, A);
        const root = h.ed.root;
        const dialog = root.querySelector('.ed-detail');
        const active = () => h.dom.document.activeElement;
        root.querySelector('[data-ed-pane="settings"]').fire('click');
        const keyInput = root.querySelector('[data-pane="settings"] input.ed-code');
        const keyLabel = keyInput.closest('.ed-field').querySelector('label');
        const onError = root.querySelector('[data-pane="settings"] [role="radiogroup"]');
        root.querySelector('[data-ed-pane="note"]').fire('click');
        const names = [keyLabel.getAttribute('for') === keyInput.getAttribute('id') && !!keyInput.getAttribute('id'), onError.getAttribute('aria-label'), root.querySelector('[data-pane="note"] textarea').getAttribute('aria-label')];
        // Arrows in a tab group move within it (and select), not to another node.
        const settingsTab = root.querySelector('[data-ed-pane="settings"]');
        settingsTab.focus();
        settingsTab.fire('keydown', { key: 'ArrowRight' });
        const toNote = [active().getAttribute('data-ed-pane'), root.querySelector('[data-ed-pane="note"]').getAttribute('aria-selected'), h.ed.detail.nodeId()];
        active().fire('keydown', { key: 'ArrowRight' });
        const wrapped = active().getAttribute('data-ed-pane');
        // Tab and Shift+Tab stay inside the dialog.
        const first = root.querySelector('[data-ed-nav="-1"]');
        first.focus();
        const back = first.fire('keydown', { key: 'Tab', shiftKey: true });
        const last = active();
        const forth = last.fire('keydown', { key: 'Tab', shiftKey: false });
        eq('c1d06 the detail view names its fields, keeps arrows in tab groups and traps Tab',
            [names, toNote, wrapped, back.defaultPrevented, last !== first && dialog.contains(last), forth.defaultPrevented, active() === first],
            [[true, 'settings_on_error', 'detail_note'], ['note', 'true', A], 'params', true, true, true, true]);
        h.ED.detail.close(h.ed);
        eq('c1d06 the keyboard checks log no errors', h.logged, []);
    });

    await guardAsync('c1d06 review dispose', async () => {
        const validate = deferred();
        const flush = deferred();
        const stored = deferred();
        const h = harness(req => {
            if (req.url === '/api/desktop/flows/validate') return validate.p;
            if (req.url === '/api/desktop/flows/runs/r5?include=doc') return stored.p;
            if (/\/test-data\//.test(req.url)) return { data: {} };
            throw apiError('FLOW_NOT_FOUND');
        });
        const entered = [];
        const issues = [];
        h.ed.enterRunView = d => entered.push(d.run.id);
        h.bus.on('issues', list => issues.push(list));
        h.ed.saver = { flush: () => flush.p };
        const pub = h.ED.publish.create(h.ed);
        const runs = h.ED.runs.create(h.ed, h.canvas);
        pub.refreshIssues();
        h.runTimers();
        const testing = runs.startTest({});
        const viewing = runs.openRunView('r5');
        pub.dispose();
        runs.dispose();
        validate.resolve({ issues: [{ code: 'X', severity: 'error' }] });
        flush.resolve(true);
        stored.resolve(Object.assign(finalRun('r5', 'success'), { doc: flowDoc() }));
        const results = [await testing, await viewing];
        await settle();
        eq('c1d06 nothing lands after dispose: validation, test dialog, run view',
            [results, issues, h.ed.issues, entered, h.notes, h.requests.filter(r => /\/test-data\//.test(r.url)).length, h.sources.length], [[undefined, undefined], [], [], [], [], 0, 0]);
    });
}
