// c1d14 checks: how the canvas restores a stored view (canvas.restoreView, used by the editor's
// placeView) and its readable fit. c1d15b checks: the effects a step test confirms, stopping runs
// from the drawer and the run view, deleting a flow secret and the start page's lock card. They
// run on the stub DOM of test-easydrag.mjs; that runner calls run(env) with its helpers and
// counts the failures.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const MODULES = ['easydrag-core.js', 'easydrag-template.js', 'easydrag-model.js', 'easydrag-geometry.js', 'easydrag-canvas.js'];
const UI_MODULES = ['easydrag-core.js', 'easydrag-template.js', 'easydrag-model.js', 'easydrag-fields.js', 'easydrag-forms.js', 'easydrag-runs.js', 'easydrag-home.js'];
const T1 = 'n_tttttttt';
const T2 = 'n_uuuuuuuu';
const TG = 'n_telegram';
const TG2 = 'n_telegra2';
const PDF = 'n_pdfpdfpd';
const DEL = 'n_deletexx';

// effectsDoc: start -> telegram -> pdf -> delete, and a second trigger feeding pdf through its
// own Telegram step.
function effectsDoc() {
    const n = (id, key, type, label) => ({ id, key, type, label, position: { x: 0, y: 0 }, params: {}, settings: {} });
    const e = (id, a, b) => ({ id, source: { node: a, port: 'out' }, target: { node: b, port: 'in' } });
    return {
        schema: 1, name: 'Effects',
        nodes: [n(T1, 'start', 'trigger.manual', 'Start'), n(T2, 'second', 'trigger.manual', 'Second'), n(TG, 'telegram', 'notify.telegram', 'Tell'),
            n(TG2, 'telegram_2', 'notify.telegram', 'Tell again'), n(PDF, 'pdf', 'documents.pdf', 'Document'), n(DEL, 'cleanup', 'files.delete', 'Clean up')],
        edges: [e('e1', T1, TG), e('e2', TG, PDF), e('e3', PDF, DEL), e('e4', T2, TG2), e('e5', TG2, PDF)]
    };
}

const apiError = code => Object.assign(new Error('text ' + code), { body: { error: 'text ' + code, code } });
const deferred = () => { let resolve, reject; const p = new Promise((a, b) => { resolve = a; reject = b; }); return { p, resolve, reject }; };

// flowDoc is a trigger and two steps in a row, 832 px wide.
function flowDoc() {
    return {
        schema: 1, name: 'Row',
        nodes: [
            { id: T1, key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} },
            { id: 'n_aaaaaaaa', key: 'alpha', type: 'web.search', label: 'Alpha', position: { x: 300, y: 0 }, params: { query: 'x' }, settings: {} },
            { id: 'n_bbbbbbbb', key: 'beta', type: 'web.search', label: 'Beta', position: { x: 600, y: 0 }, params: { query: 'y' }, settings: {} }
        ],
        edges: []
    };
}

export async function run(env) {
    const { apps, types, t, miniDom, eq, check, guardAsync, settle } = env;

    // uiHarness loads UI_MODULES on miniDom, where elements keep the markup they were given
    // (el.html) and value, checked and select values act like a browser's. The api is
    // core.createApi over a transport that records each request in h.requests and answers with
    // answer({url, method, body}) (a throw rejects). Notifications land in h.notes, console.error
    // lines in h.logged; localStorage is h.store. opts: types (the catalog), doc (the flow).
    function uiHarness(answer, opts) {
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
        proto.removeAttribute = function (name) { this.attrs.delete(name); };
        proto.click = function () { this.fire('click'); };
        // An element is connected when the editor root is among its ancestors (the root itself
        // hangs in core.el's template); getElementById searches that root, as the page would.
        let root = null;
        Object.defineProperty(proto, 'isConnected', { configurable: true, get() { for (let x = this; x; x = x.parentNode) if (x === root) return true; return false; } });
        Object.assign(dom.document, { addEventListener() {}, removeEventListener() {}, getElementById: id => (root ? root.querySelector('[id="' + id + '"]') : null) });
        const store = new Map();
        const logged = [];
        const timers = new Map();
        let nextId = 1;
        const opened = [];
        // clock.now (when set) is Date.now() in the sandbox.
        const clock = { now: null };
        const ClockDate = class extends Date {};
        ClockDate.now = () => (clock.now === null ? Date.now() : clock.now);
        const box = vm.createContext({
            window: { SYSTEM_LANG: 'en', open: (...args) => { opened.push(args); } }, navigator: { platform: 'Linux' }, crypto: webcrypto, document: dom.document, Date: ClockDate,
            URLSearchParams, URL, EventSource: class { addEventListener() {} close() {} },
            console: { log() {}, warn() {}, error: (...args) => { logged.push(args.map(String).join(' ')); } },
            localStorage: { getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); } },
            setTimeout: (fn, ms) => { const id = nextId++; timers.set(id, { fn, ms }); return id; },
            clearTimeout: id => { timers.delete(id); },
            requestAnimationFrame: () => 0, cancelAnimationFrame() {}, CSS: { escape: value => String(value) }
        });
        for (const file of UI_MODULES) {
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
        const model = ED.model.create(o.doc || effectsDoc(), { types: catalogTypes });
        const notes = [];
        const ed = {
            ctx: { notify: n => notes.push(n) }, t, esc: ED.core.esc, api: ED.core.createApi(transport), catalog: { types: catalogTypes }, windowId: 'w1',
            readonly: false, root: ED.core.el('<div class="ed-editor"></div>'), flow: { id: 'f1', draft_revision: 3, published_draft_revision: 3 },
            model, saver: null, selection: new Set(), run: null, runView: null, issues: [], lastRunData: null, effectsConfirmed: new Set(), bus: ED.core.emitter()
        };
        root = ed.root;
        const open = () => ed.root.querySelectorAll('.ed-modal-backdrop').filter(m => !m.classList.contains('is-closing'));
        return {
            ED, ed, dom, requests, notes, logged, store, timers, opened, clock,
            canvas: { announce() {} },
            top: () => open()[open().length - 1] || null,
            runTimers(ms) { for (const [id, entry] of Array.from(timers)) { if (ms !== undefined && entry.ms > ms) continue; timers.delete(id); entry.fn(); } }
        };
    }

    // ── 1d-15b A: a step test confirms the effects of every step that runs ──
    await guardAsync('c1d15b step test effects', async () => {
        const fxTypes = new Map(types);
        fxTypes.set('notify.telegram', Object.assign({}, types.get('notify.telegram'), { effects: ['sends_message'] }));
        fxTypes.set('documents.pdf', Object.assign({}, types.get('documents.pdf'), { effects: ['writes_files'] }));
        fxTypes.set('files.delete', { type: 'files.delete', label: 'Delete', inputs: ['in'], outputs: ['out'], params: [], effects: ['deletes'] });
        // preview is the publish preview's answer (the effects of the real settings); a function
        // that throws makes it fail.
        const harness = (doc, preview) => uiHarness(req => {
            if (/\/test-data\//.test(req.url)) return { data: {} };
            if (req.url === '/api/desktop/flows/f1/test') return { run_id: 'r1' };
            if (req.url === '/api/desktop/flows/f1/publish-preview') return typeof preview === 'function' ? preview() : (preview || { issues: [], effects: [], diff: {} });
            throw apiError('FLOW_RUN_NOT_FOUND');
        }, { types: fxTypes, doc });
        const effectKeys = dialog => dialog.body.querySelectorAll('[data-ed-test-effects] [data-ed-effect]').map(li => li.getAttribute('data-ed-effect')).sort();

        // The real case: start -> telegram -> pdf. Testing the pdf step sends the message too.
        const h = harness();
        const runs = h.ED.runs.create(h.ed, h.canvas);
        const step = await runs.startTest({ onlyNode: PDF });
        const asked = [!!step, effectKeys(step), h.ED.runs.effects(h.ed, PDF, T1).get('sends_message')];
        // The other trigger reaches pdf through its own Telegram step: choosing it lists that one.
        const select = step.body.querySelector('[data-ed-test-trigger]');
        select.querySelectorAll('option').forEach(op => { if (op.attrs.get('value') === T2) op.attrs.set('selected', ''); else op.attrs.delete('selected'); });
        step.body.querySelector('[data-ed-effects-skip]').checked = true;
        select.fire('change');
        await settle();
        const other = [effectKeys(step), h.ED.runs.effects(h.ed, PDF, T2).get('sends_message'), step.body.querySelector('[data-ed-effects-skip]').checked];
        step.el.querySelector('[data-ed-action="run"]').fire('click');
        await settle();
        const posted = h.requests.filter(r => r.url === '/api/desktop/flows/f1/test').map(r => [r.body.trigger_node, r.body.only_node]);
        eq('c1d15b a step test asks for the effects of the steps before it (start -> telegram -> pdf asks sends_message)',
            [asked, other, posted, h.store.get('aurago.easydrag.effects-ok.f1')],
            [[true, ['sends_message', 'writes_files'], ['Tell']], [['sends_message', 'writes_files'], ['Tell again'], true], [[T2, PDF]], '["sends_message","writes_files"]']);

        // Steps after the tested one, disabled steps and the steps behind them do not run.
        const off = effectsDoc();
        off.nodes.find(n => n.id === TG).settings = { disabled: true };
        const d = harness(off);
        const scoped = [T1, T2].map(id => Array.from(d.ED.runs.effects(d.ed, PDF, id).keys()).sort());
        const quick = await d.ED.runs.create(d.ed, d.canvas).startTest({ onlyNode: PDF, quick: true, triggerNode: T1 });
        const whole = Array.from(d.ED.runs.effects(d.ed).keys()).sort();
        eq('c1d15b a disabled step stops what only it feeds; later steps and other triggers do not count; the whole flow counts every enabled step',
            [scoped, quick, d.requests.filter(r => r.url === '/api/desktop/flows/f1/test').map(r => r.body.trigger_node), whole],
            [[[], ['sends_message', 'writes_files']], undefined, [T1], ['deletes', 'sends_message', 'writes_files']]);

        // The real settings count: an HTTP request with POST sends, though its catalog entry
        // (default GET) names no effect. Only the steps that run count with their real effects.
        fxTypes.set('http.request', { type: 'http.request', label: 'HTTP', inputs: ['in'], outputs: ['out'], params: [{ name: 'method', kind: 'select' }] });
        const web = effectsDoc();
        web.nodes.find(n => n.id === TG).type = 'http.request';
        web.nodes.find(n => n.id === TG).params = { method: 'POST' };
        web.nodes.find(n => n.id === TG2).type = 'http.request';
        const realPreview = { issues: [], diff: {}, effects: [{ effect: 'sends_message', node_ids: [TG] }, { effect: 'writes_files', node_ids: [PDF] }, { effect: 'deletes', node_ids: [DEL] }] };
        const w = harness(web, realPreview);
        const post = await w.ED.runs.create(w.ed, w.canvas).startTest({ onlyNode: PDF, quick: true, triggerNode: T1 });
        const postKeys = post ? effectKeys(post) : null;
        const postNodes = w.ED.runs.effects(w.ed, PDF, T1, new Map([[TG, ['sends_message']]])).get('sends_message');
        const catalogOnly = Array.from(w.ED.runs.effects(w.ed, PDF, T1).keys());
        eq('c1d15b an HTTP POST step in the run scope asks through the preview; real effects of steps that do not run are left out',
            [!!post, postKeys, postNodes, catalogOnly, w.requests.filter(r => r.url === '/api/desktop/flows/f1/test').length],
            [true, ['sends_message', 'writes_files'], ['Tell'], ['writes_files'], 0]);
        if (post) post.el.querySelector('[data-ed-action="cancel"]').fire('click');

        // A preview that fails: the catalog effects are listed with a hint, and even a quick test
        // whose catalog effects are all confirmed opens the dialog instead of running silently.
        const lost = harness(web, () => { throw apiError('FLOW_INTERNAL'); });
        lost.store.set('aurago.easydrag.effects-ok.f1', '["writes_files"]');
        const unchecked = await lost.ED.runs.create(lost.ed, lost.canvas).startTest({ onlyNode: PDF, quick: true, triggerNode: T1 });
        eq('c1d15b without the preview the dialog says the effects could not be fully checked and never runs silently',
            [!!unchecked, unchecked && !!unchecked.body.querySelector('[data-ed-effects-unchecked]'), unchecked && effectKeys(unchecked), lost.requests.filter(r => r.url === '/api/desktop/flows/f1/test').length,
                !!post && !post.body.querySelector('[data-ed-effects-unchecked]')],
            [true, true, [], 0, true]);

        // An edit that lands after the preview (here: while it loads) is checked again: Run then
        // shows the new list and waits for a second confirmation; a quick test opens the dialog.
        const editedHarness = () => {
            let calls = 0;
            let e = null;
            const doc = effectsDoc();
            Object.assign(doc.nodes.find(n => n.id === TG), { type: 'http.request', params: { method: 'GET' } });
            e = harness(doc, () => {
                calls++;
                if (calls === 1) { e.ed.model.setParam(TG, 'method', 'POST'); return { issues: [], diff: {}, effects: [{ effect: 'writes_files', node_ids: [PDF] }] }; }
                return { issues: [], diff: {}, effects: [{ effect: 'sends_message', node_ids: [TG] }, { effect: 'writes_files', node_ids: [PDF] }] };
            });
            e.flushes = 0;
            e.ed.saver = { flush: () => { e.flushes++; return Promise.resolve(true); } };
            e.previews = () => e.requests.filter(r => r.url === '/api/desktop/flows/f1/publish-preview').length;
            e.tests = () => e.requests.filter(r => r.url === '/api/desktop/flows/f1/test').length;
            return e;
        };
        const ea = editedHarness();
        const edited = await ea.ED.runs.create(ea.ed, ea.canvas).startTest({ onlyNode: PDF, triggerNode: T1 });
        const beforeRun = effectKeys(edited);
        edited.el.querySelector('[data-ed-action="run"]').fire('click');
        await settle();
        const afterFirst = [effectKeys(edited), ea.tests(), ea.previews(), ea.flushes, !!ea.top()];
        edited.el.querySelector('[data-ed-action="run"]').fire('click');
        await settle();
        const eb = editedHarness();
        eb.store.set('aurago.easydrag.effects-ok.f1', '["writes_files"]');
        const quickEdited = await eb.ED.runs.create(eb.ed, eb.canvas).startTest({ onlyNode: PDF, quick: true, triggerNode: T1 });
        eq('c1d15b an edit after the preview is checked again: Run shows the new list first, a quick test opens the dialog',
            [beforeRun, afterFirst, ea.tests(), !!quickEdited, quickEdited && effectKeys(quickEdited), eb.tests(), eb.previews(), eb.flushes],
            [['writes_files'], [['sends_message', 'writes_files'], 0, 2, 2, true], 1, true, ['sends_message'], 0, 2, 2]);

        // A running step whose effects hook failed (the catalog marks it risky without effects)
        // makes the effects unchecked; such a step that does not run changes nothing.
        fxTypes.set('tool.broken', { type: 'tool.broken', label: 'Broken', inputs: ['in'], outputs: ['out'], params: [], risky: true });
        const broken = effectsDoc();
        broken.nodes.find(n => n.id === DEL).type = 'tool.broken';
        const bh = harness(broken);
        bh.store.set('aurago.easydrag.effects-ok.f1', '["sends_message","writes_files"]');
        const bruns = bh.ED.runs.create(bh.ed, bh.canvas);
        const before = await bruns.startTest({ onlyNode: PDF, quick: true, triggerNode: T1 });
        bruns.clearRun();
        const crash = await bruns.startTest({ onlyNode: DEL, quick: true, triggerNode: T1 });
        eq('c1d15b a running step whose effects hook failed shows the unchecked hint and is never run quietly',
            [before, bh.requests.filter(r => r.url === '/api/desktop/flows/f1/test').length, !!crash, crash && !!crash.body.querySelector('[data-ed-effects-unchecked]'),
                bh.ED.runs.unknownEffects(bh.ed, PDF, T1), bh.ED.runs.unknownEffects(bh.ed, DEL, T1)],
            [undefined, 1, true, true, false, true]);
        eq('c1d15b the step test effect checks log no errors', [h.logged, d.logged, w.logged, lost.logged, ea.logged, eb.logged, bh.logged], [[], [], [], [], [], [], []]);
    });

    // ── 1d-15b B: runs that have not ended can be stopped from the drawer and the run view ──
    await guardAsync('c1d15b stop runs', async () => {
        const at = '2026-10-06T10:00:00Z';
        const run = (id, status, mode) => ({ id, status, mode, started_at: at, trigger_node: T1 });
        let listed = [run('r_wait', 'waiting', 'live'), run('r_done', 'success', 'live'), run('r_test', 'running', 'test')];
        const cancels = new Map([['r_wait', () => ({ cancelled: true })], ['r_test', () => ({ cancelled: true })],
            ['r_gone', () => { throw apiError('FLOW_RUN_FINISHED'); }], ['r_lost', () => { throw apiError('FLOW_RUN_NOT_FOUND'); }], ['r_view', () => ({ cancelled: true })]]);
        const h = uiHarness(req => {
            if (req.url === '/api/desktop/flows/f1/runs?limit=50') return { runs: listed };
            const cancel = /^\/api\/desktop\/flows\/runs\/([a-z_]+)\/cancel$/.exec(req.url);
            if (cancel && req.method === 'POST') return cancels.get(cancel[1])();
            if (req.url === '/api/desktop/flows/runs/r_view') return { run: run('r_view', 'cancelled', 'live'), steps: [] };
            throw apiError('FLOW_RUN_NOT_FOUND');
        });
        const runs = h.ED.runs.create(h.ed, h.canvas);
        const drawer = () => h.ed.root.querySelector('.ed-drawer');
        const stopOf = id => drawer().querySelector('[data-ed-run-stop="' + id + '"]');
        const posted = () => h.requests.filter(r => r.url.endsWith('/cancel')).map(r => r.url.split('/')[5]);
        const lists = () => h.requests.filter(r => r.url === '/api/desktop/flows/f1/runs?limit=50').length;
        runs.toggleDrawer(true);
        await settle();
        const label = stopOf('r_wait') && stopOf('r_wait').getAttribute('aria-label');
        const shown = [!!stopOf('r_wait'), !!stopOf('r_done'), !!stopOf('r_test'), String(label).startsWith('run_stop_label:') && label.length > 'run_stop_label:'.length,
            stopOf('r_wait').localName, stopOf('r_wait').parentNode.className];
        // A live run asks first: keeping it posts nothing.
        stopOf('r_wait').focus();
        stopOf('r_wait').fire('click');
        await settle();
        const asked = [!!h.top(), h.top() && h.top().querySelector('.ed-modal-title') !== null, posted()];
        h.top().querySelector('[data-ed-action="keep"]').fire('click');
        await settle();
        const kept = [h.top(), posted()];
        // Stop: the cancel is posted, the list comes again and the focus stays on the run's row.
        const before = lists();
        listed = [run('r_wait', 'cancelled', 'live'), run('r_done', 'success', 'live'), run('r_test', 'running', 'test')];
        stopOf('r_wait').focus();
        stopOf('r_wait').fire('click');
        await settle();
        h.top().querySelector('[data-ed-action="stop"]').fire('click');
        await settle();
        const stopped = [posted(), lists() - before, !!stopOf('r_wait'), h.dom.document.activeElement && h.dom.document.activeElement.getAttribute('data-ed-run'), h.notes];
        eq('c1d15b a waiting live run shows Stop in the drawer; Stop asks first and then posts the cancel and refreshes the list',
            [shown, asked, kept, stopped],
            [[true, false, true, true, 'button', 'ed-run-item'], [true, true, []], [null, []], [['r_wait'], 1, false, 'r_wait', []]]);
        // A test run stops without a question; a run that ended meanwhile (409) refreshes quietly;
        // an unknown run says so.
        stopOf('r_test').fire('click');
        await settle();
        const testStop = [h.top(), posted()];
        listed = [run('r_gone', 'running', 'live'), run('r_lost', 'queued', 'live')];
        runs.toggleDrawer(false);
        runs.toggleDrawer(true);
        await settle();
        const listsBefore = lists();
        stopOf('r_gone').fire('click');
        await settle();
        h.top().querySelector('[data-ed-action="stop"]').fire('click');
        await settle();
        const gone = [posted().slice(-1), lists() - listsBefore, h.notes.slice()];
        stopOf('r_lost').fire('click');
        await settle();
        h.top().querySelector('[data-ed-action="stop"]').fire('click');
        await settle();
        eq('c1d15b a test run stops without asking; 409 FLOW_RUN_FINISHED refreshes quietly; other errors are shown',
            [testStop, gone, h.notes.map(n => [n.title, n.message, n.type])],
            [[null, ['r_wait', 'r_test']], [['r_gone'], 1, []], [['run_cancel', 'error_flow_run_not_found', 'error']]]);
        // The run view: Stop posts the cancel and shows the stored state of the viewed run.
        h.ed.runView = { run: run('r_view', 'waiting', 'live'), doc: effectsDoc() };
        runs.applyRunView({ run: h.ed.runView.run, steps: [] });
        const viewed = runs.stopRun(h.ed.runView.run);
        await settle();
        h.top().querySelector('[data-ed-action="stop"]').fire('click');
        await settle();
        eq('c1d15b Stop in the run view posts the cancel and refreshes the viewed run',
            [await viewed, posted().slice(-1), h.ed.runView.run.status, h.ed.run.status, !!h.ed.run.view], [true, ['r_view'], 'cancelled', 'cancelled', true]);
        // A read-only desktop offers no Stop.
        h.ed.readonly = true;
        listed = [run('r_wait', 'waiting', 'live')];
        runs.toggleDrawer(false);
        runs.toggleDrawer(true);
        await settle();
        eq('c1d15b a read-only desktop shows no Stop and stops nothing', [!!stopOf('r_wait'), await runs.stopRun(listed[0])], [false, false]);
        eq('c1d15b the stop checks log no errors', h.logged, []);
    });

    // ── 1d-15b review: stops of runs no stream shows, out-of-order answers, focus and names ──
    await guardAsync('c1d15b stop follow-ups', async () => {
        const at = '2026-10-06T10:00:00Z';
        const run = (id, status, mode) => ({ id, status, mode, started_at: at, trigger_node: T1 });
        let listed = [run('r_t', 'running', 'test'), run('r_a', 'waiting', 'agent')];
        let stored = 'running';
        const views = [];
        const h = uiHarness(req => {
            if (req.url === '/api/desktop/flows/f1/runs?limit=50') return { runs: listed };
            if (req.url === '/api/desktop/flows/runs/r_t/cancel') return { cancelled: true };
            if (req.url === '/api/desktop/flows/runs/r_t') return { run: run('r_t', stored, 'test'), steps: [] };
            if (req.url === '/api/desktop/flows/runs/r_s/cancel') return { cancelled: true };
            if (req.url === '/api/desktop/flows/runs/r_s') return { run: run('r_s', 'running', 'test'), steps: [] };
            if (req.url === '/api/desktop/flows/runs/r_v') { const d = deferred(); views.push(d); return d.p; }
            throw apiError('FLOW_RUN_NOT_FOUND');
        });
        const runs = h.ED.runs.create(h.ed, h.canvas);
        const drawer = () => h.ed.root.querySelector('.ed-drawer');
        const stopOf = id => drawer().querySelector('[data-ed-run-stop="' + id + '"]');
        const lists = () => h.requests.filter(r => r.url === '/api/desktop/flows/f1/runs?limit=50').length;
        const fetches = () => h.requests.filter(r => r.url === '/api/desktop/flows/runs/r_t').length;
        const delays = () => Array.from(h.timers.values()).map(x => x.ms).filter(ms => ms >= 1000);
        runs.toggleDrawer(true);
        await settle();
        // #8: the name carries the seconds; #7: an agent run asks too.
        const label = stopOf('r_a').getAttribute('aria-label');
        stopOf('r_a').fire('click');
        await settle();
        const asked = !!h.top();
        h.top().querySelector('[data-ed-action="keep"]').fire('click');
        await settle();
        // #9: a reload keeps the focus on the Stop of a run that goes on.
        stopOf('r_a').focus();
        runs.refreshShown();
        await settle();
        const kept = h.dom.document.activeElement === stopOf('r_a');
        // #2: a test run that streams elsewhere is asked for after 1, 2 and 4 s until it ended.
        const listsBefore = lists();
        stopOf('r_t').fire('click');
        await settle();
        const first = [delays(), fetches(), lists() - listsBefore];
        h.runTimers(1000);
        await settle();
        const second = [delays(), fetches()];
        stored = 'cancelled';
        listed = [run('r_t', 'cancelled', 'test'), run('r_a', 'waiting', 'agent')];
        h.runTimers(2000);
        await settle();
        eq('c1d15b a stop asks for any run but a test, names the run to the second, keeps the focus on a Stop that stays, and follows a run no stream shows',
            [label === 'run_stop_label:' + h.ED.core.fmt.dateTime(at, true), h.ED.core.fmt.dateTime(at, true) !== h.ED.core.fmt.dateTime(at), asked, kept, first, second, [delays(), fetches(), lists() - listsBefore, !!stopOf('r_t')]],
            [true, true, true, true, [[1000], 0, 1], [[2000], 1], [[], 2, 2, false]]);
        // #4: an older answer for the viewed run never replaces a newer one.
        h.ed.runView = { run: run('r_v', 'running', 'live'), doc: effectsDoc() };
        runs.applyRunView({ run: h.ed.runView.run, steps: [] });
        runs.refreshShown();
        runs.refreshShown();
        await settle();
        views[1].resolve({ run: run('r_v', 'cancelled', 'live'), steps: [] });
        await settle();
        views[0].resolve({ run: run('r_v', 'running', 'live'), steps: [] });
        await settle();
        eq('c1d15b an older answer for the viewed run never takes back its end', [views.length, h.ed.runView.run.status, h.ed.run.status], [2, 'cancelled', 'cancelled']);
        h.ed.runView = null;
        runs.clearRun();
        // #3: a run that keeps going is asked for after 1, 2, 4, 8 and 16 s, then no more.
        listed = [run('r_s', 'running', 'test')];
        runs.refreshShown();
        await settle();
        const slowDelays = [];
        h.timers.clear();
        stopOf('r_s').fire('click');
        await settle();
        for (let i = 0; i < 6; i++) { const next = delays(); slowDelays.push(next[0] || 0); h.runTimers(16000); await settle(); }
        eq('c1d15b a stopped run that keeps going is followed for up to 31 s', slowDelays, [1000, 2000, 4000, 8000, 16000, 0]);
        // #4: run_finished refreshes once per burst, and at the latest a second after it began.
        h.timers.clear();
        const lastRuns = () => h.requests.filter(r => r.url === '/api/desktop/flows/f1/runs?limit=1').length;
        const lastBefore = lastRuns();
        const waits = [];
        for (const now of [10000, 10200, 10500, 10900]) {
            h.clock.now = now;
            runs.runFinished();
            waits.push(Array.from(h.timers.values()).map(x => x.ms));
        }
        h.runTimers(250);
        await settle();
        eq('c1d15b a burst of run ends refreshes once, a second after it began at the latest', [waits, lastRuns() - lastBefore], [[[250], [250], [250], [100]], 1]);
        // dispose drops a pending refresh and a pending follow-up.
        h.clock.now = null;
        runs.runFinished();
        stored = 'running';
        listed = [run('r_t', 'running', 'test')];
        runs.refreshShown();
        await settle();
        stopOf('r_t').fire('click');
        await settle();
        const pending = Array.from(h.timers.values()).map(x => x.ms).sort((a, b) => a - b);
        runs.dispose();
        eq('c1d15b dispose cancels the run_finished refresh and the stop follow-up', [pending, Array.from(h.timers.values()).map(x => x.ms)], [[250, 1000], []]);
        eq('c1d15b the stop follow-up checks log no errors', h.logged, []);
    });

    // ── 1d-15b C: a flow secret can be deleted from the secret field ──
    await guardAsync('c1d15b delete secrets', async () => {
        const h = uiHarness(() => { throw apiError('FLOW_NOT_FOUND'); });
        const F = h.ED.fields;
        const host = h.ed.root;
        let names = ['api_key', 'other'];
        let lists = 0;
        const deleted = [];
        let deleteAnswer = () => Promise.resolve({ status: 'deleted', used_by: ['Alpha', 'Beta', 'Gamma', 'Delta', 'Epsilon'] });
        const api = {
            secrets: () => { lists++; return Promise.resolve({ secrets: names.slice() }); },
            deleteSecret: name => { deleted.push(name); names = names.filter(n => n !== name); return deleteAnswer(name); }
        };
        const notes = [];
        const picked = [];
        const env = { t, esc: h.ED.core.esc, readonly: false, root: host, api, secretCache: {}, notify: n => notes.push(n) };
        const picker = host.appendChild(F.secretRef(env, 'api_key', v => picked.push(v === undefined ? 'cleared' : v), 'sec-a'));
        await settle();
        const button = picker.querySelector('[data-ed-secret-delete]');
        const options = () => picker.querySelectorAll('option').map(o => o.getAttribute('value'));
        const ready = [!!button, button.disabled, button.getAttribute('aria-label')];
        // Cancel posts nothing.
        button.fire('click');
        const dialog = h.top();
        const text = dialog && dialog.html.includes('secret_delete_text:api_key');
        dialog.querySelector('[data-ed-action="keep"]').fire('click');
        await settle();
        const kept = [deleted.slice(), picked.slice(), lists];
        // Delete: posted, the names come again, the field is cleared and a warning names three flows.
        button.fire('click');
        h.top().querySelector('[data-ed-action="delete"]').fire('click');
        await settle();
        eq('c1d15b Delete asks first, then deletes the secret, refreshes the names, clears the field and warns with used_by',
            [ready, text, kept, deleted, lists, options(), picked, button.disabled, h.dom.document.activeElement === picker.querySelector('select'), notes],
            [[true, false, 'secret_delete'], true, [[], [], 1], ['api_key'], 2, ['', 'other'], ['cleared'], true, true,
                [{ title: 'secret_deleted:api_key', message: 'secret_still_used:names_more:Alpha, Beta, Gamma,2', type: 'warning' }]]);
        // No user left: no warning. A 429 keeps the dialog open and holds Delete for Retry-After.
        deleteAnswer = () => Promise.resolve({ status: 'deleted', used_by: [] });
        const other = F.secretRef(env, 'other', () => {}, 'sec-b');
        await settle();
        deleteAnswer = () => Promise.reject(Object.assign(new Error('slow'), { status: 429, retryAfter: 30, body: { error: 'slow', code: 'FLOW_RATE_LIMITED' } }));
        other.querySelector('[data-ed-secret-delete]').fire('click');
        const limited = h.top();
        limited.querySelector('[data-ed-action="delete"]').fire('click');
        await settle();
        const action = limited.querySelector('[data-ed-action="delete"]');
        const held = [h.top() === limited, limited.querySelector('.ed-error').textContent, (h.runTimers(0), action.disabled)];
        h.runTimers(30000);
        const released = action.disabled;
        deleteAnswer = () => Promise.resolve({ status: 'deleted', used_by: [] });
        action.fire('click');
        await settle();
        eq('c1d15b a 429 holds Delete for Retry-After; a delete without users shows no warning',
            [held, released, h.top(), deleted.slice(1), notes.length],
            [[true, 'error_flow_rate_limited secret_retry:30', true], false, null, ['other', 'other'], 1]);
        const readonly = F.secretRef(Object.assign({}, env, { readonly: true }), 'other', () => {}, 'sec-c');
        eq('c1d15b a read-only field has no Delete', !!readonly.querySelector('[data-ed-secret-delete]'), false);

        // The real chain: the change rebuilds the form (forms refresh, the focus is in the dialog),
        // so the focus goes to the new list, found by its id, not to the detached one.
        let vault = ['api_key', 'other'];
        const fh = uiHarness(req => {
            if (req.url === '/api/desktop/flows/secrets') return { secrets: vault.slice() };
            if (req.url === '/api/desktop/flows/secrets/api_key' && req.method === 'DELETE') { vault = vault.filter(n => n !== 'api_key'); return { status: 'deleted', used_by: ['Alpha', 'Beta'] }; }
            throw apiError('FLOW_NOT_FOUND');
        });
        const info = { type: 'http.request', params: [{ name: 'auth_secret', kind: 'secret_ref', label: 'Secret' }] };
        let node = { id: 'n_http', type: 'http.request', params: { auth_secret: 'api_key' }, settings: {} };
        let form = null;
        form = fh.ED.forms.render({
            ed: fh.ed, node, info, roots: {}, upstream: [], issues: [],
            onChange: (name, value) => {
                const params = Object.assign({}, node.params);
                if (value === undefined) delete params[name]; else params[name] = value;
                node = Object.assign({}, node, { params });
                form.refresh(node);
            }
        });
        fh.ed.root.appendChild(form.el);
        await settle();
        const before = form.el.querySelector('select');
        form.el.querySelector('[data-ed-secret-delete]').fire('click');
        fh.top().querySelector('[data-ed-action="delete"]').fire('click');
        await settle();
        const after = form.el.querySelector('select');
        eq('c1d15b after a delete that rebuilds the form, the new list has the focus and the field is empty',
            [after !== before, before.isConnected, fh.dom.document.activeElement === after, after.getAttribute('id'), 'auth_secret' in node.params, after.querySelectorAll('option').map(o => o.getAttribute('value')),
                fh.notes.map(n => n.message)],
            [true, false, true, 'ed-p-w1-n_http-auth_secret', false, ['', 'other'], ['secret_still_used:Alpha and Beta']]);

        // A chosen secret the Vault no longer has is marked; a list that cannot load marks nothing.
        const gone = host.appendChild(F.secretRef(Object.assign({}, env, { secretCache: {} }), 'old_key', () => {}, 'sec-e'));
        await settle();
        const goneOption = gone.querySelectorAll('option').find(o => o.getAttribute('value') === 'old_key');
        eq('c1d15b a chosen secret that no longer exists is marked as missing',
            [!!goneOption && goneOption.attrs.has('selected'), gone.querySelector('select').getAttribute('data-error'), gone.querySelector('select').html.includes('secret_missing:old_key'),
                picker.querySelector('select').html.includes('secret_missing')],
            [true, 'error_flow_secret_unavailable', true, false]);
        eq('c1d15b the secret delete checks log no errors', [h.logged, fh.logged], [[], []]);
    });

    // ── 1d-15b D: the start page's lock card offers the window card's actions ──
    await guardAsync('c1d15b start page lock card', async () => {
        let listAnswer = () => { throw apiError('FLOWS_DISABLED'); };
        const h = uiHarness(req => {
            if (req.url === '/api/desktop/flows') return listAnswer();
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
            throw apiError('FLOW_NOT_FOUND');
        });
        const app = { ctx: { notify() {} }, t, esc: h.ED.core.esc, api: h.ed.api, readonly: false, catalog: { types }, openFlow() {}, openHome() {} };
        const home = h.ED.home.create(app);
        h.ed.root.appendChild(home.el);
        await settle();
        const grid = home.el.querySelector('.ed-flow-grid');
        const lists = () => h.requests.filter(r => r.url === '/api/desktop/flows').length;
        const off = [!!grid.querySelector('.ed-home-empty [data-ed-home-settings]'), !!grid.querySelector('.ed-home-empty [data-ed-home-retry]'), grid.html.includes('ed-error')];
        grid.querySelector('[data-ed-home-settings]').fire('click');
        const first = grid.querySelector('[data-ed-home-retry]');
        first.focus();
        first.fire('click');
        await settle();
        const again = grid.querySelector('[data-ed-home-retry]');
        const retried = [h.opened, lists(), again !== first && h.dom.document.activeElement === again];
        listAnswer = () => { throw apiError('FLOW_INTERNAL'); };
        grid.querySelector('[data-ed-home-retry]').fire('click');
        await settle();
        const other = [!!grid.querySelector('[data-ed-home-settings]'), !!grid.querySelector('[data-ed-home-retry]'), grid.html.includes('ed-error'), h.dom.document.activeElement === grid.querySelector('[data-ed-home-retry]')];
        // Flows back: the focus goes to the first card.
        listAnswer = () => ({ flows: [{ id: 'f9', name: 'Nine', triggers: [], preview: [] }] });
        grid.querySelector('[data-ed-home-retry]').fire('click');
        await settle();
        const back = h.dom.document.activeElement === grid.querySelector('[data-ed-flow="f9"]');
        // A control outside the start page keeps the focus.
        listAnswer = () => { throw apiError('FLOWS_DISABLED'); };
        await home.reload();
        const outside = h.ed.root.appendChild(h.ED.core.el('<button type="button">elsewhere</button>'));
        outside.focus();
        grid.querySelector('[data-ed-home-retry]').fire('click');
        await settle();
        const kept = h.dom.document.activeElement === outside;
        eq('c1d15b the start page lock card offers Open settings and Try again; other errors offer Try again only; Try again keeps the focus on the grid',
            [off, retried, other, back, kept], [[true, true, false], [[['/config#flows', '_blank', 'noopener']], 2, true], [false, true, true, true], true, true]);
        home.dispose();
        eq('c1d15b the lock card checks log no errors', h.logged, []);
    });

    // canvasFor runs the canvas on the stub DOM; resize(w, h) sets the size its element reports.
    function canvasFor() {
        const dom = miniDom();
        const proto = dom.El.prototype;
        Object.defineProperty(proto, 'style', {
            configurable: true,
            get() { return this.styles; },
            set(value) { this.styles = Object.assign(value, { setProperty(name, v) { this[name] = String(v); } }); }
        });
        Object.assign(dom.document, { addEventListener() {}, removeEventListener() {} });
        const box = vm.createContext({
            window: {}, navigator: { platform: 'Linux' }, crypto: webcrypto, document: dom.document, console,
            localStorage: { getItem: () => null, setItem() {}, removeItem() {} },
            setTimeout: () => 0, clearTimeout() {}, requestAnimationFrame: () => 0, cancelAnimationFrame() {},
            matchMedia: () => ({ matches: false }), CSS: { escape: value => String(value) },
            ResizeObserver: class { observe() {} disconnect() {} }
        });
        for (const file of MODULES) {
            const full = path.join(apps, file);
            vm.runInContext(fs.readFileSync(full, 'utf8'), box, { filename: full });
        }
        const ED = box.window.EasyDrag;
        const bus = ED.core.emitter();
        const model = ED.model.create(flowDoc(), { types });
        const ed = {
            ctx: {}, t: k => k, esc: ED.core.esc, catalog: { types }, readonly: false, model, bus, initialRender: true, root: null,
            selection: new Set(), selectedEdge: null, view: { x: 0, y: 0, zoom: 1 }, run: null, issues: []
        };
        const canvas = ED.canvas.create(ed);
        let size = { width: 1000, height: 600 };
        canvas.el.getBoundingClientRect = () => ({ left: 0, top: 0, right: size.width, bottom: size.height, width: size.width, height: size.height });
        return { ed, canvas, resize(w, h) { size = { width: w, height: h }; } };
    }
    const round = v => ({ x: Math.round(v.x * 100) / 100, y: Math.round(v.y * 100) / 100, zoom: Math.round(v.zoom * 1000) / 1000 });

    await guardAsync('c1d14 stored views', async () => {
        const h = canvasFor();
        h.canvas.setView({ x: 40, y: 120, zoom: 0.9 });
        const before = round(h.ed.view);
        // A view stored in the older form is screen offsets of another window size: not used.
        const old = h.canvas.restoreView({ x: -500, y: 0, zoom: 1 });
        eq('c1d14 a stored view in the old {x, y, zoom} form is ignored', [old, round(h.ed.view)], [false, before]);

        // The stored centre puts the same world point in the middle at any window size.
        const stored = h.canvas.center();
        h.resize(1400, 900);
        const restored = h.canvas.restoreView(stored);
        const again = h.canvas.center();
        eq('c1d14 a stored centre restores the same view in a larger window',
            [restored, Math.round(again.cx), Math.round(again.cy), again.zoom], [true, Math.round(stored.cx), Math.round(stored.cy), stored.zoom]);
        eq('c1d14 the restored view keeps the world point in the middle', round(h.ed.view), round({ x: 700 - stored.cx * 0.9, y: 450 - stored.cy * 0.9, zoom: 0.9 }));

        // A view that shows no step is not kept: the editor fits instead, readable on a narrow canvas.
        h.resize(390, 700);
        const far = h.canvas.restoreView({ cx: 50000, cy: 50000, zoom: 1 });
        check('c1d14 a stored view that shows no step is refused', far === false && !h.canvas.anyNodeVisible());
        h.canvas.fit({ readable: true });
        eq('c1d14 the fallback is the readable fit from the trigger', round(h.ed.view), round({ x: 48, y: 350 - 36 * 0.8, zoom: 0.8 }));
        check('c1d14 the readable fit shows a step', h.canvas.anyNodeVisible());

        // An explicit fit shows everything, below the readable zoom when it must.
        h.canvas.fit();
        check('c1d14 an explicit fit goes below the readable zoom on a narrow canvas', h.ed.view.zoom < 0.8, String(h.ed.view.zoom));
    });
}
