// c1d07 checks: start page, dialogs, editor screen and window shell. They run every EasyDrag
// module on the stub DOM of test-easydrag.mjs; that runner calls run(env) with its helpers
// (check, eq, guardAsync, miniDom, ...) and counts the failures.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const MODULES = ['easydrag-core.js', 'easydrag-template.js', 'easydrag-model.js', 'easydrag-geometry.js', 'easydrag-saver.js', 'easydrag-canvas.js',
    'easydrag-wires.js', 'easydrag-interact.js', 'easydrag-palette.js', 'easydrag-fields.js', 'easydrag-forms.js', 'easydrag-mapping.js',
    'easydrag-detail.js', 'easydrag-runs.js', 'easydrag-publish.js', 'easydrag-home.js', 'easydrag-dialogs.js', 'easydrag-editor.js', 'easydrag.js'];
const T1 = 'n_tttttttt';
const A = 'n_aaaaaaaa';
const B = 'n_bbbbbbbb';
const DRAFT_KEY = 'aurago.easydrag.draft.f1';

// flowDoc is a flow start -> alpha -> beta.
function flowDoc(name) {
    return {
        schema: 1, name: name || 'Flow',
        nodes: [
            { id: T1, key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} },
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

// chipState reads the editor's state chip: [colour, text, title, Publish disabled].
function chipState(editor) {
    const m = /<span class="ed-chip ed-chip--(\w+)"(?: title="([^"]*)")?>([^<]*)<\/span>/.exec(editor.el.querySelector('[data-ed-state]').html);
    return m ? [m[1], m[3], m[2] === undefined ? null : m[2], editor.el.querySelector('[data-ed-cmd="publish"]').disabled] : null;
}

export async function run(env) {
    const { apps, types, t, miniDom, eq, guardAsync, settle } = env;
    const catalogOf = () => ({ types, list: Array.from(types.values()), categories: [] });

    // sandbox loads every module and the window shell on miniDom, as the c1d04 and c1d06 harnesses
    // do: elements keep the markup they were given (el.html); value, checked, style, parentElement
    // and isConnected act like a browser's (connected = inside h.body). Document listeners land in
    // h.docListeners and h.fireDoc(type, init) dispatches to them. Timers and frames are recorded
    // and fired by hand; localStorage, ResizeObserver, matchMedia and EventSource are stubs. Every
    // core.modal dialog is listed in h.dialogs. The api records each request in h.requests and
    // answers with answer(req); undefined falls back to "no runs" and "no issues". ctx records
    // notifications (h.notes), confirmations (h.confirms, answered with h.confirmAnswer), window
    // menus (h.menus), cleared menus (h.cleared) and console.error lines (h.logged).
    function sandbox(answer) {
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
        Object.defineProperty(proto, 'parentElement', { configurable: true, get() { return this.parentNode; } });
        Object.defineProperty(proto, 'isConnected', {
            configurable: true,
            get() { let x = this; while (x.parentNode) x = x.parentNode; return x === body; }
        });
        proto.removeAttribute = function (name) { this.attrs.delete(name); };
        proto.setSelectionRange = function (start, end) { this.selectionStart = start; this.selectionEnd = end; };
        proto.scrollIntoView = function () {};
        proto.click = function () { this.fire('click'); };
        proto.blur = function () {};
        proto.select = function () {};
        const body = new dom.El('body', {});
        const docListeners = {};
        Object.assign(dom.document, {
            body,
            querySelector: sel => body.querySelector(sel),
            elementFromPoint: () => null,
            addEventListener(type, fn) { (docListeners[type] = docListeners[type] || []).push(fn); },
            removeEventListener(type, fn) { const list = docListeners[type] || []; const i = list.indexOf(fn); if (i >= 0) list.splice(i, 1); }
        });
        const timers = new Map();
        const frames = new Map();
        let nextId = 1;
        const store = new Map();
        const logged = [];
        class EventSource {
            constructor(url) { this.url = url; this.listeners = {}; this.closed = false; this.onerror = null; }
            addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
            close() { this.closed = true; }
        }
        const box = vm.createContext({
            window: { SYSTEM_LANG: 'en', location: { origin: 'https://aurago.test' }, open() {} }, navigator: { platform: 'Linux' }, crypto: webcrypto, document: dom.document,
            EventSource, URLSearchParams, URL,
            console: { log() {}, warn() {}, error: (...args) => { logged.push(args.map(String).join(' ')); } },
            localStorage: {
                get length() { return store.size; },
                key: i => Array.from(store.keys())[i] ?? null,
                getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); }
            },
            setTimeout: (fn, ms) => { const id = nextId++; timers.set(id, { fn, ms }); return id; },
            clearTimeout: id => { timers.delete(id); },
            requestAnimationFrame: fn => { const id = nextId++; frames.set(id, fn); return id; },
            cancelAnimationFrame: id => { frames.delete(id); },
            matchMedia: () => ({ matches: false }),
            CSS: { escape: value => String(value) },
            ResizeObserver: class { observe() {} disconnect() {} }
        });
        for (const file of MODULES) {
            const full = path.join(apps, file);
            vm.runInContext(fs.readFileSync(full, 'utf8'), box, { filename: full });
        }
        const ED = box.window.EasyDrag;
        const dialogs = [];
        const modal = ED.core.modal;
        ED.core.modal = (host, options) => { const d = modal(host, options); dialogs.push(d); return d; };
        const requests = [];
        const defaults = req => {
            if (req.method === 'GET' && /\/runs\?limit=1$/.test(req.url)) return { runs: [] };
            if (req.url === '/api/desktop/flows/validate') return { issues: [] };
            throw apiError('FLOW_NOT_FOUND');
        };
        const transport = (url, init) => {
            const req = { url, method: (init && init.method) || 'GET', body: init && init.body ? JSON.parse(init.body) : undefined };
            requests.push(req);
            return Promise.resolve().then(() => { const out = answer(req); return out === undefined ? defaults(req) : out; });
        };
        const h = {
            ED, dom, body, win: box.window, store, timers, frames, logged, docListeners, dialogs, requests, transport,
            api: ED.core.createApi(transport), catalog: catalogOf(),
            notes: [], confirms: [], confirmAnswer: false, menus: [], cleared: [], homes: [], opened: [],
            puts: () => requests.filter(r => r.method === 'PUT'),
            delays: () => Array.from(timers.values()).map(x => x.ms),
            pe: (id, x, y) => ({ pointerId: id, clientX: x, clientY: y, button: 0, buttons: 1, pointerType: 'mouse', shiftKey: false, ctrlKey: false, metaKey: false, altKey: false }),
            key: (k, target, extra) => Object.assign({ key: k, target, repeat: false, shiftKey: false, ctrlKey: false, metaKey: false, altKey: false }, extra),
            fireDoc(type, init) {
                const event = Object.assign({ type, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, stopPropagation() {} }, init);
                (docListeners[type] || []).slice().forEach(fn => fn(event));
                return event;
            },
            runTimers(ms) {
                for (const [id, entry] of Array.from(timers)) {
                    if (ms === undefined || entry.ms === ms) { timers.delete(id); entry.fn(); }
                }
            },
            flushFrames() {
                for (let i = 0; frames.size && i < 20; i++) {
                    const [id, fn] = frames.entries().next().value;
                    frames.delete(id);
                    fn();
                }
            }
        };
        h.ctx = {
            t, esc: ED.core.esc,
            notify: n => h.notes.push(n),
            confirmDialog: async (title, text) => { h.confirms.push(text); return h.confirmAnswer; },
            promptDialog: async () => null,
            setWindowMenus: (id, menus) => { h.menus = menus; },
            clearWindowMenus: id => { h.cleared.push(id); },
            showContextMenu: () => {},
            updateWindowContext: () => {},
            setWindowBeforeClose: () => {},
            isActive: () => true,
            openApp: () => {}
        };
        return h;
    }

    // openEditor creates the editor for flow f1 (draft revision 3) in h.body and runs its first frame.
    // opts: flow (fields over the default record), enabled.
    function openEditor(h, opts) {
        const o = opts || {};
        const flow = Object.assign({ id: 'f1', name: 'Flow', draft: flowDoc(), draft_revision: 3, published_draft_revision: 0, live: null, live_revision: 0 }, o.flow);
        const app = {
            ctx: h.ctx, t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, windowId: 'w1', readonly: false,
            openHome: x => h.homes.push(x || {}), openFlow: id => h.opened.push(id)
        };
        const editor = h.ED.editor.create(app, { flow, enabled: !!o.enabled, issues: [] }, {});
        h.body.appendChild(editor.el);
        h.flushFrames();
        return editor;
    }

    await guardAsync('c1d07 save states', async () => {
        const failures = {
            failed: () => apiError('FLOW_LOCKED'),
            offline: () => new TypeError('Failed to fetch'),
            invalid: () => Object.assign(new Error('bad'), { body: { error: 'bad', code: 'FLOW_INVALID', issues: [] } })
        };
        const rows = [];
        const kept = {};
        for (const name of Object.keys(failures)) {
            const saves = [];
            const h = sandbox(req => {
                if (req.method !== 'PUT') return undefined;
                const next = saves.length ? saves.shift() : failures[name]();
                if (next instanceof Error) throw next;
                return next;
            });
            const editor = openEditor(h);
            await settle();
            editor.ed.model.setFlow({ description: 'changed' });
            h.runTimers(1000);
            await settle();
            const chip = editor.el.querySelector('[data-ed-save]');
            const retry = chip.querySelector('[data-ed-cmd="save"]');
            const title = retry ? retry.getAttribute('title') : chip.title;
            const shown = [editor.ed.saver.state, chip.className, chip.html.includes(h.ED.core.ICONS.alert), title, !!retry, retry ? chip.title : null];
            // Ctrl+S tries again but never says "Saved" while the draft is not saved.
            const live = editor.el.querySelector('[data-ed-live]');
            live.textContent = '';
            const sent = h.puts().length;
            h.fireDoc('keydown', h.key('s', editor.el.querySelector('.ed-canvas'), { ctrlKey: true }));
            await settle();
            h.runTimers(30);
            const saveKey = [h.puts().length - sent, live.textContent];
            // Leaving asks first, and "stay" keeps the editor.
            const left = await editor.leave();
            rows.push([name, shown, saveKey, left, h.confirms.length]);
            kept[name] = { h, editor, saves };
        }
        eq('c1d07 failed, offline and invalid drafts show their state and error, Ctrl+S never says Saved and leaving asks', rows, [
            ['failed', ['failed', 'ed-foot-item ed-save ed-save--failed', true, 'error_flow_locked', true, ''], [1, ''], false, 1],
            ['offline', ['offline', 'ed-foot-item ed-save ed-save--offline', true, 'error_network', false, null], [1, ''], false, 1],
            ['invalid', ['invalid', 'ed-foot-item ed-save ed-save--invalid', true, '', false, null], [0, ''], false, 1]
        ]);
        const { h, editor, saves } = kept.failed;
        saves.push({ draft_revision: 4, issues: [] });
        const live = editor.el.querySelector('[data-ed-live]');
        live.textContent = '';
        editor.el.querySelector('[data-ed-save] [data-ed-cmd="save"]').fire('click');
        await settle();
        h.runTimers(30);
        eq('c1d07 the failed chip is a button that saves again and then says Saved',
            [editor.ed.saver.state, editor.ed.flow.draft_revision, live.textContent, !!editor.el.querySelector('[data-ed-save] [data-ed-cmd="save"]'), await editor.leave(), h.confirms.length],
            ['saved', 4, 'save_saved', false, true, 1]);
        eq('c1d07 the save state checks log no errors', Object.values(kept).flatMap(k => k.h.logged), []);
    });

    await guardAsync('c1d07 restore offer', async () => {
        const outcomes = [];
        for (const action of ['discard', 'restore', null, 'closed with the editor']) {
            const h = sandbox(() => undefined);
            h.store.set(DRAFT_KEY, JSON.stringify({ revision: 3, at: Date.now(), doc: flowDoc('Local') }));
            const editor = openEditor(h);
            await settle();
            const dialog = h.dialogs[0];
            const locked = !!dialog && !dialog.el.querySelector('[data-ed-action="close"]');
            const runsAsked = () => h.requests.filter(r => r.url.includes('/runs?')).length;
            const before = runsAsked();
            if (action === 'closed with the editor') { editor.dispose(); dialog.close(null); }
            else if (action === null) dialog.close(null);
            else dialog.el.querySelector('[data-ed-action="' + action + '"]').fire('click');
            await settle();
            outcomes.push([action, locked, before, h.store.has(DRAFT_KEY), editor.ed.model.doc.name, runsAsked(), h.logged.length]);
        }
        eq('c1d07 only an explicit discard drops the emergency copy; no answer keeps the copy and the draft', outcomes, [
            ['discard', true, 0, false, 'Flow', 1, 0],
            ['restore', true, 0, true, 'Local', 1, 0],
            [null, true, 0, true, 'Flow', 1, 0],
            ['closed with the editor', true, 0, true, 'Flow', 0, 0]
        ]);
    });

    await guardAsync('c1d07 save conflict', async () => {
        const saves = [];
        const gets = [];
        const h = sandbox(req => {
            if (req.method === 'PUT') { const next = saves.shift(); if (next instanceof Error) throw next; return next; }
            if (req.method === 'GET' && req.url === '/api/desktop/flows/f1') return gets.shift();
            return undefined;
        });
        const editor = openEditor(h);
        await settle();
        editor.ed.model.setFlow({ description: 'mine' });
        saves.push(apiError('FLOW_REVISION_CONFLICT'));
        h.runTimers(1000);
        await settle();
        const first = h.dialogs[0];
        const asked = [editor.ed.saver.state, !!first, !!(first && first.el.querySelector('[data-ed-action="close"]'))];
        first.close(null);
        await settle();
        const chip = editor.el.querySelector('[data-ed-save]');
        const fetched = () => h.requests.filter(r => r.method === 'GET' && r.url === '/api/desktop/flows/f1').length;
        eq('c1d07 a conflict closed without an answer neither keeps nor reloads: offline with the copy and a retry',
            [asked, editor.ed.saver.state, editor.ed.saver.error && editor.ed.saver.error.body.code, chip.title, h.store.has(DRAFT_KEY), editor.ed.model.doc.description, fetched(), h.delays().includes(5000)],
            [['conflict', true, false], 'offline', 'FLOW_REVISION_CONFLICT', 'error_flow_revision_conflict', true, 'mine', 0, true]);
        saves.push(apiError('FLOW_REVISION_CONFLICT'));
        h.runTimers(5000);
        await settle();
        gets.push({ flow: { draft_revision: 7, draft: flowDoc('Theirs') } });
        saves.push({ draft_revision: 8, issues: [] });
        h.dialogs[1].el.querySelector('[data-ed-action="keep"]').fire('click');
        await settle();
        const puts = h.puts();
        eq('c1d07 the retry asks again and an explicit keep overwrites on the server revision',
            [h.dialogs.length, puts.length, puts[2] && puts[2].body.base_revision, puts[2] && puts[2].body.doc.description, editor.ed.saver.state, h.store.has(DRAFT_KEY)],
            [2, 3, 7, 'mine', 'saved', false]);
        const reload = h.ED.dialogs.conflict(editor.ed);
        h.dialogs[2].el.querySelector('[data-ed-action="reload"]').fire('click');
        eq('c1d07 an explicit reload still answers reload', await reload, 'reload');
        eq('c1d07 the conflict checks log no errors', h.logged, []);
    });

    await guardAsync('c1d07 drag at dispose and leave', async () => {
        const results = [];
        for (const how of ['dispose', 'leave']) {
            const h = sandbox(req => (req.method === 'PUT' ? { draft_revision: 4, issues: [] } : undefined));
            const editor = openEditor(h);
            await settle();
            const canvasEl = editor.el.querySelector('.ed-canvas');
            editor.el.querySelector('[data-node-id="' + A + '"]').fire('pointerdown', h.pe(1, 300, 0));
            canvasEl.fire('pointermove', h.pe(1, 380, 40));
            const mid = Object.assign({}, editor.ed.model.node(A).position);
            const left = how === 'dispose' ? editor.dispose() : await editor.leave();
            await settle();
            const put = h.puts()[0];
            canvasEl.fire('pointermove', h.pe(1, 500, 80));
            results.push([how, mid.x !== 300 || mid.y !== 0, h.puts().length, put && put.body.doc.nodes.find(n => n.id === A).position, editor.ed.model.node(A).position, left, h.logged]);
        }
        eq('c1d07 a drag in progress is cancelled before the final save of dispose and leave', results, [
            ['dispose', true, 1, { x: 300, y: 0 }, { x: 300, y: 0 }, undefined, []],
            ['leave', true, 1, { x: 300, y: 0 }, { x: 300, y: 0 }, true, []]
        ]);
    });

    await guardAsync('c1d07 refused paste', async () => {
        const h = sandbox(() => undefined);
        const editor = openEditor(h);
        await settle();
        h.store.set('aurago.easydrag.clipboard', JSON.stringify({ easydrag: 1, nodes: Array.from({ length: 501 }, (_, i) => ({ id: 'n' + i, type: 'web.search' })), edges: [] }));
        const paste = h.menus.find(m => m.id === 'edit').items.find(i => i.id === 'paste');
        await paste.action();
        await settle();
        eq('c1d07 a refused paste is shown, not only announced', [editor.ed.model.doc.nodes.length, h.notes, h.logged], [3, [{ title: 'paste', message: 'paste_refused' }], []]);
    });

    await guardAsync('c1d07 shortcuts dialog and keys without a key', async () => {
        const h = sandbox(() => undefined);
        const editor = openEditor(h);
        await settle();
        const dialog = h.ED.dialogs.shortcuts(editor.ed);
        const html = dialog.el.parentNode.html;
        const kbds = html.match(/<kbd>[^<]*<\/kbd>/g) || [];
        eq('c1d07 the shortcuts dialog draws "+ / −" as one key and no empty key',
            [kbds.includes('<kbd>+ / −</kbd>'), kbds.includes('<kbd></kbd>'), kbds.includes('<kbd>Ctrl</kbd>'), kbds.includes('<kbd>S</kbd>')], [true, false, true, true]);
        dialog.close(null);
        h.runTimers(140);
        const event = h.fireDoc('keydown', { target: editor.el.querySelector('.ed-canvas'), key: undefined, ctrlKey: false, metaKey: false, shiftKey: false, altKey: false });
        eq('c1d07 a keydown without a key (autofill) is ignored without an error', [event.defaultPrevented, h.logged], [false, []]);
    });

    await guardAsync('c1d07 publish state', async () => {
        let liveRevision = 3;
        const record = rev => ({ id: 'f1', name: 'Flow', draft: flowDoc(), draft_revision: 3, published_draft_revision: 3, live: flowDoc(), live_revision: rev });
        const h = sandbox(req => {
            if (req.url === '/api/desktop/flows/f1/publish-preview') return { issues: [], effects: [], diff: {} };
            if (req.url === '/api/desktop/flows/f1/publish') return { flow: record(3), issues: [], partial: true, code: 'FLOW_PUBLISH_INCOMPLETE', error: 'server text' };
            if (req.method === 'GET' && req.url === '/api/desktop/flows/f1') return { flow: record(liveRevision), enabled: false, issues: [] };
            return undefined;
        });
        const editor = openEditor(h, { flow: { published_draft_revision: 2, live: flowDoc(), live_revision: 2 } });
        await settle();
        const states = [chipState(editor)];
        editor.el.querySelector('[data-ed-cmd="publish"]').fire('click');
        await settle();
        h.dialogs[h.dialogs.length - 1].el.querySelector('[data-ed-action="publish"]').fire('click');
        await settle();
        states.push(chipState(editor));
        const published = () => h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason: 'published' } });
        published();
        await settle();
        states.push(chipState(editor));
        liveRevision = 4;
        published();
        await settle();
        states.push(chipState(editor));
        eq('c1d07 a partial publish shows its own state and keeps Publish until a higher live revision arrives', states, [
            ['accent', 'state_changes', null, false],
            ['warn', 'state_publish_incomplete', 'error_flow_publish_incomplete', false],
            ['warn', 'state_publish_incomplete', 'error_flow_publish_incomplete', false],
            ['ok', 'state_published', null, false]
        ]);
        eq('c1d07 effectsConfirmed is a Set', Object.prototype.toString.call(editor.ed.effectsConfirmed), '[object Set]');
        eq('c1d07 the publish state checks log no errors', h.logged, []);
    });

    await guardAsync('c1d07 start page refreshes', async () => {
        let listAnswer = () => ({ flows: [{ id: 'f1', name: 'One', triggers: [], preview: [] }] });
        const h = sandbox(req => {
            if (req.url === '/api/desktop/flows') return listAnswer();
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
            return undefined;
        });
        const home = h.ED.home.create({ ctx: h.ctx, t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly: false, openFlow: id => h.opened.push(id) });
        h.body.appendChild(home.el);
        await settle();
        const grid = home.el.querySelector('.ed-flow-grid');
        const lists = () => h.requests.filter(r => r.url === '/api/desktop/flows').length;
        const changed = reason => h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason } });
        const initial = lists();
        changed('saved');
        changed('saved');
        changed('saved');
        await settle();
        const waiting = [lists(), h.delays()];
        h.runTimers(1500);
        await settle();
        const afterPause = lists();
        changed('saved');
        changed('published');
        await settle();
        eq('c1d07 saved changes refresh the flow list once after a pause, other changes at once',
            [initial, waiting, afterPause, lists(), h.delays()], [1, [1, [1500]], 2, 3, []]);
        listAnswer = () => { throw apiError('FLOWS_DISABLED'); };
        changed('enabled');
        await settle();
        const calm = [grid.html.includes('disabled_title'), grid.html.includes('ed-error'), h.delays(), lists()];
        await settle();
        const quiet = lists();
        changed('created');
        await settle();
        eq('c1d07 FLOWS_DISABLED shows a calm state and asks again only on the next change', [calm, quiet, lists()], [[true, false, [], 4], 4, 5]);
        const slow = deferred();
        listAnswer = () => slow.p;
        changed('created');
        await settle();
        listAnswer = () => ({ flows: [{ id: 'f2', name: 'Two', triggers: [], preview: [] }] });
        changed('created');
        await settle();
        slow.resolve({ flows: [{ id: 'f1', name: 'Old', triggers: [], preview: [] }] });
        await settle();
        eq('c1d07 only the newest flow list answer is shown', [grid.html.includes('data-ed-flow="f2"'), grid.html.includes('data-ed-flow="f1"')], [true, false]);
        changed('saved');
        home.dispose();
        eq('c1d07 dispose drops the refresh listener and a pending refresh', [(h.docListeners['aurago:flows-changed'] || []).length, h.delays()], [0, []]);
        eq('c1d07 the start page checks log no errors', h.logged, []);
    });

    await guardAsync('c1d07 window shell', async () => {
        const record = id => ({ flow: { id, name: 'F', draft: flowDoc(), draft_revision: 3, published_draft_revision: 0, live_revision: 0 }, enabled: false, issues: [] });
        const h = sandbox(req => {
            if (req.url.startsWith('/api/desktop/flows/node-types')) return { node_types: Array.from(types.values()), categories: [] };
            if (req.url === '/api/desktop/flows/f1' || req.url === '/api/desktop/flows/f2') return record(req.url.slice(-2));
            return undefined;
        });
        const container = h.body.appendChild(new h.dom.El('div', {}));
        const App = h.win.EasyDragApp;
        App.render(container, 'w1', Object.assign({}, h.ctx, { api: h.transport, flowId: 'f1' }));
        await settle();
        h.flushFrames();
        const first = container.querySelector('.ed-editor');
        App.open('w1', { flow_id: 'f2' });
        await settle();
        h.flushFrames();
        const inst = App._instances.get('w1');
        const second = inst && inst.screen && inst.screen.ed ? inst.screen.ed.flow.id : null;
        App.dispose('w1');
        eq('c1d07 the shell opens a flow, switches to another one and cleans up',
            [!!first, second, !!first && first.isConnected, container.children.length, h.cleared, App._instances.size], [true, 'f2', false, 0, ['w1'], 0]);
        eq('c1d07 the shell checks log no errors', h.logged, []);
        const d = sandbox(() => { throw apiError('FLOWS_DISABLED'); });
        const host = d.body.appendChild(new d.dom.El('div', {}));
        d.win.EasyDragApp.render(host, 'w2', Object.assign({}, d.ctx, { api: d.transport }));
        await settle();
        const box = host.querySelector('.ed-shell-error');
        const asked = d.requests.length;
        await settle();
        const quiet = [d.requests.length, d.delays()];
        box.querySelector('[data-ed-shell="retry"]').fire('click');
        await settle();
        eq('c1d07 FLOWS_DISABLED while loading shows the disabled state and asks again only on Retry',
            [!!box, !!(box && box.querySelector('[data-ed-shell="config"]')), asked, quiet, d.requests.length], [true, true, 1, [1, []], 2]);
    });
}
