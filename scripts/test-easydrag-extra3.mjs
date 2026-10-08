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
    // h.docListeners (window ones in h.winListeners) and h.fireDoc(type, init) dispatches to them. Timers and frames are recorded
    // and fired by hand; localStorage, ResizeObserver, matchMedia and EventSource are stubs. Every
    // core.modal dialog is listed in h.dialogs. The api records each request in h.requests and
    // answers with answer(req); undefined falls back to "no runs" and "no issues". ctx records
    // notifications (h.notes), confirmations (h.confirms, answered with h.confirmAnswer), window
    // menus (h.menus), cleared menus (h.cleared) and console.error lines (h.logged).
    // platform is navigator.platform (default Linux; 'MacIntel' makes core.isMod read metaKey).
    function sandbox(answer, platform) {
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
        const winListeners = {};
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
            window: { SYSTEM_LANG: 'en', location: { origin: 'https://aurago.test' }, open() {}, addEventListener: (type, fn) => { (winListeners[type] = winListeners[type] || []).push(fn); },
                removeEventListener: (type, fn) => { const list = winListeners[type] || []; if (list.includes(fn)) list.splice(list.indexOf(fn), 1); } }, navigator: { platform: platform || 'Linux' }, crypto: webcrypto, document: dom.document,
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
            ED, dom, body, win: box.window, store, timers, frames, logged, docListeners, winListeners, dialogs, requests, transport,
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
    // opts: flow (fields over the default record), enabled, readonly (a read-only desktop).
    function openEditor(h, opts) {
        const o = opts || {};
        const flow = Object.assign({ id: 'f1', name: 'Flow', draft: flowDoc(), draft_revision: 3, published_draft_revision: 0, live: null, live_revision: 0 }, o.flow);
        const app = {
            ctx: h.ctx, t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, windowId: 'w1', readonly: !!o.readonly,
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

    // ── 1d-07 review (A): shell lifetime, window menus and keys ──

    await guardAsync('c1d07 review no screen after the window closed', async () => {
        const outcomes = [];
        for (const ending of ['loads', 'fails']) {
            const catalog = deferred();
            const h = sandbox(req => {
                if (req.url.startsWith('/api/desktop/flows/node-types')) return catalog.p;
                if (req.url === '/api/desktop/flows') return { flows: [] };
                if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
                return undefined;
            });
            const container = h.body.appendChild(new h.dom.El('div', {}));
            h.win.EasyDragApp.render(container, 'w1', Object.assign({}, h.ctx, { api: h.transport }));
            await settle();
            const root = container.children[0];
            h.win.EasyDragApp.dispose('w1');
            if (ending === 'loads') catalog.resolve({ node_types: Array.from(types.values()), categories: [] });
            else catalog.reject(apiError('FLOWS_DISABLED'));
            await settle();
            outcomes.push([ending, (h.docListeners['aurago:flows-changed'] || []).length, h.requests.filter(r => r.url === '/api/desktop/flows').length,
                root.children.filter(c => !c.className.includes('ed-loading')).length, h.logged]);
        }
        eq('c1d07 a catalog answer after dispose builds no start page or error card and leaves no flows-changed listener',
            outcomes, [['loads', 0, 0, 0, []], ['fails', 0, 0, 0, []]]);
        // A home navigation that waited for leave() stops when the editor closed meanwhile.
        const save = deferred();
        const e = sandbox(req => (req.method === 'PUT' ? save.p : undefined));
        const editor = openEditor(e);
        await settle();
        editor.ed.model.setFlow({ description: 'pending' });
        const going = e.menus.find(m => m.id === 'flow').items.find(i => i.id === 'home').action();
        await settle();
        editor.dispose();
        save.resolve({ draft_revision: 4, issues: [] });
        await going;
        await settle();
        eq('c1d07 goHome after a leave() that outlived the editor opens nothing', [e.homes.length, e.logged], [0, []]);
    });

    await guardAsync('c1d07 review window menu shortcuts', async () => {
        const h = sandbox(() => undefined);
        const editor = openEditor(h);
        await settle();
        const all = () => h.menus.flatMap(m => m.items);
        const item = id => all().find(i => i.id === id);
        eq('c1d10 only Ctrl+S, Ctrl+Enter and Ctrl+K are dispatched by the desktop; the other keys are shortcut hints and every label is a translation key',
            [all().filter(i => i.shortcut).map(i => i.id + '=' + i.shortcut), all().filter(i => i.shortcutHint).map(i => i.id + '=' + i.shortcutHint),
                all().filter(i => i.type !== 'separator' && (i.label || !i.labelKey)).map(i => i.id)],
            [['save=Ctrl+S', 'test=Ctrl+Enter', 'search=Ctrl+K'],
                ['undo=Ctrl+Z', 'redo=Ctrl+Shift+Z', 'cut=Ctrl+X', 'copy=Ctrl+C', 'paste=Ctrl+V', 'duplicate=Ctrl+D', 'delete-sel=Del', 'select-all=Ctrl+A',
                    'zoom-in=+', 'zoom-out=−', 'zoom-fit=Shift+1', 'keys=?'],
                []]);
        item('search').action();
        const search = editor.el.querySelector('.ed-palette-search');
        eq('c1d07 the Ctrl+K item focuses the palette search', h.dom.document.activeElement === search, true);
        // Edits wait while the detail view covers the canvas, and work again once it closes.
        item('select-all').action();
        editor.ed.bus.emit('open-detail', { nodeId: A });
        const under = [!!editor.ed.detail, ['undo', 'cut', 'copy', 'paste', 'duplicate', 'delete-sel', 'select-all'].map(id => item(id).disabled)];
        item('delete-sel').action();
        item('cut').action();
        const nodes = editor.ed.model.doc.nodes.length;
        h.ED.detail.close(editor.ed);
        eq('c1d07 the detail view disables the Edit items and their actions change nothing',
            [under, nodes, item('delete-sel').disabled], [[true, [true, true, true, true, true, true, true]], 3, false]);
        eq('c1d07 the menu checks log no errors', h.logged, []);
    });

    await guardAsync('c1d07 review menus under the restore offer', async () => {
        const h = sandbox(req => (req.method === 'PUT' ? { draft_revision: 4, issues: [] } : undefined));
        h.store.set(DRAFT_KEY, JSON.stringify({ revision: 3, at: Date.now(), doc: flowDoc('Local work') }));
        const editor = openEditor(h);
        await settle();
        const items = id => h.menus.find(m => m.id === id).items;
        const disabled = [
            ['undo', 'redo', 'cut', 'copy', 'paste', 'duplicate', 'delete-sel', 'select-all'].map(id => items('edit').find(i => i.id === id).disabled),
            ['save', 'test', 'run', 'publish', 'settings', 'delete'].map(id => items('flow').find(i => i.id === id).disabled)
        ];
        // Run every Edit action anyway, in menu order (select all, then delete): a stale menu or a raced click.
        for (const i of items('edit')) if (i.action) await i.action();
        items('edit').find(i => i.id === 'select-all').action();
        items('edit').find(i => i.id === 'delete-sel').action();
        await settle();
        h.runTimers(1000);
        await settle();
        const copy = JSON.parse(h.store.get(DRAFT_KEY));
        eq('c1d07 under the restore offer the menus are disabled and change neither the draft nor the copy',
            [disabled, editor.ed.model.doc.nodes.length, copy.doc.name, copy.doc.nodes.length, h.puts().length],
            [[[true, true, true, true, true, true, true, true], [true, true, true, true, true, true]], 3, 'Local work', 3, 0]);
        h.dialogs[0].el.querySelector('[data-ed-action="restore"]').fire('click');
        await settle();
        h.runTimers(140);
        eq('c1d07 after the answer the draft is the copy and the menus work again',
            [editor.ed.model.doc.name, items('edit').find(i => i.id === 'select-all').disabled, items('flow').find(i => i.id === 'save').disabled], ['Local work', false, false]);
        eq('c1d07 the restore menu checks log no errors', h.logged, []);
    });

    // ── 1d-07 review (B): start page, import and the minor items ──

    // hostile is markup that must stay text wherever a name or label appears.
    const hostile = '"><img src=x onerror=alert(1)><script>x</script>';
    // injected lists elements and on* attributes that only unescaped markup can create.
    const injected = root => {
        const out = [];
        const walk = node => {
            if (node.localName === 'img' || node.localName === 'script') out.push(node.localName);
            node.attrs.forEach((v, k) => { if (k.startsWith('on')) out.push(k); });
            node.children.forEach(walk);
        };
        walk(root);
        return out;
    };

    await guardAsync('c1d07 review start page states, import and cards', async () => {
        let listAnswer = () => ({ flows: [{ id: 'f1', name: 'One', triggers: [], preview: [] }] });
        const created = deferred();
        const h = sandbox(req => {
            if (req.url === '/api/desktop/flows' && req.method === 'GET') return listAnswer();
            if (req.url === '/api/desktop/flows' && req.method === 'POST') return created.p;
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [{ id: 't1', name: 'Tpl', description: 'd', categories: [] }] };
            if (req.url === '/api/desktop/flows/f1' && req.method === 'DELETE') return { status: 'deleted', used_by: [] };
            return undefined;
        });
        let menu = null;
        h.ctx.showContextMenu = (x, y, items) => { menu = items; };
        const home = h.ED.home.create({ ctx: h.ctx, t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly: false, openFlow: id => h.opened.push(id) });
        h.body.appendChild(home.el);
        await settle();
        const grid = home.el.querySelector('.ed-flow-grid');
        const search = home.el.querySelector('[data-ed-home-search]');
        const changed = reason => h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason } });
        const shown = () => [grid.html.includes('data-ed-flow="f1"'), grid.html.includes('disabled_title'), grid.html.includes('ed-error'),
            home.el.querySelector('.ed-template-grid').html.includes('data-ed-template="t1"'), !!search.disabled];
        const states = [shown()];
        listAnswer = () => { throw apiError('FLOWS_DISABLED'); };
        changed('enabled');
        await settle();
        home.el.querySelector('[data-ed-filter="active"]').fire('click');
        states.push(shown());
        listAnswer = () => { throw apiError('FLOW_INTERNAL'); };
        changed('enabled');
        await settle();
        home.el.querySelector('[data-ed-filter="all"]').fire('click');
        states.push(shown());
        listAnswer = () => ({ flows: [{ id: 'f1', name: 'One', triggers: [], preview: [] }] });
        changed('enabled');
        await settle();
        states.push(shown());
        eq('c1d07 a failed list load clears the cards, keeps its card on repaints and the templates, and pauses search until a load succeeds',
            states, [[true, false, false, true, false], [false, true, false, true, true], [false, false, true, true, true], [true, false, false, true, false]]);
        // Import: a file over the 4 MiB document limit is not read.
        let read = 0;
        const file = size => ({ size, text: async () => { read++; return '{"schema":1,"name":"X","nodes":[],"edges":[]}'; } });
        const input = home.el.querySelector('[data-ed-import-file]');
        input.fire('change', { target: { files: [file(4 * 1024 * 1024 + 1)], value: 'big.json' } });
        await settle();
        const posts = () => h.requests.filter(r => r.method === 'POST').length;
        const big = [read, posts(), h.notes.map(n => n.message)];
        input.fire('change', { target: { files: [file(1024)], value: 'small.json' } });
        await settle();
        eq('c1d07 an import over 4 MiB is refused unread; a small one is read and sent', [big, read, posts()], [[0, 0, ['error_flow_too_large']], 1, 1]);
        // While that create runs, a held Enter and a second click on a template create nothing more.
        const card = home.el.querySelector('[data-ed-template="t1"]');
        card.fire('keydown', { key: 'Enter', repeat: true });
        card.fire('click');
        card.fire('click');
        await settle();
        const during = posts();
        created.resolve({ flow: { id: 'f9' } });
        await settle();
        card.fire('click');
        await settle();
        eq('c1d07 one create at a time: repeats and clicks during a create send nothing, a later click creates again', [during, h.opened, posts()], [1, ['f9', 'f9'], 2]);
        // Deleting from the card menu drops the flow's emergency copy and viewport.
        h.store.set(DRAFT_KEY, JSON.stringify({ revision: 3, at: Date.now(), doc: flowDoc() }));
        h.store.set('aurago.easydrag.view.f1', JSON.stringify({ x: 1, y: 2, zoom: 1 }));
        h.confirmAnswer = true;
        home.el.querySelector('[data-ed-card-menu="f1"]').fire('click');
        await menu.find(i => i.label === 'home_delete').action();
        await settle();
        eq('c1d07 deleting a flow on the start page drops its emergency copy and viewport',
            [h.requests.some(r => r.method === 'DELETE'), h.store.has(DRAFT_KEY), h.store.has('aurago.easydrag.view.f1')], [true, false, false]);
        home.dispose();
        eq('c1d07 the start page review checks log no errors', h.logged, []);
    });

    await guardAsync('c1d07 review delete paths', async () => {
        const outcomes = [];
        for (const ending of ['fails', 'succeeds']) {
            const del = deferred();
            const h = sandbox(req => {
                if (req.method === 'DELETE') return del.p;
                if (req.method === 'PUT') return { draft_revision: 4, issues: [] };
                return undefined;
            });
            h.confirmAnswer = true;
            const editor = openEditor(h);
            await settle();
            h.runTimers(400);
            editor.ed.model.setFlow({ description: 'unsaved' });
            const had = [h.store.has(DRAFT_KEY), h.store.has('aurago.easydrag.view.f1')];
            const deleting = h.menus.find(m => m.id === 'flow').items.find(i => i.id === 'delete').action();
            await settle();
            // The server's broadcast of this very delete arrives before its answer.
            h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason: 'deleted' } });
            if (ending === 'fails') del.reject(apiError('FLOW_LOCKED')); else del.resolve({ status: 'deleted', used_by: [] });
            await deleting;
            await settle();
            if (ending === 'succeeds') editor.dispose(); // the shell disposes the editor for the start page
            h.runTimers(1000);
            await settle();
            outcomes.push([ending, had, h.store.has(DRAFT_KEY), h.store.has('aurago.easydrag.view.f1'), h.homes.length, h.puts().length,
                h.notes.map(n => n.message), h.logged]);
        }
        eq('c1d07 a failed delete keeps the saver; a done one drops copy and viewport, saves nothing and ignores its own broadcast', outcomes, [
            ['fails', [true, true], false, true, 0, 1, ['error_flow_locked'], []],
            ['succeeds', [true, true], false, false, 1, 0, [], []]
        ]);
    });

    await guardAsync('c1d07 review hostile names stay text', async () => {
        const h = sandbox(req => {
            if (req.url === '/api/desktop/flows' && req.method === 'GET') {
                return { flows: [{ id: 'f1' + hostile, name: hostile, description: hostile, triggers: [hostile], preview: [{ x: 0, y: 0, category: hostile }],
                    published: true, enabled: true, last_run: { status: hostile, started_at: hostile } }] };
            }
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [{ id: hostile, name: hostile, description: hostile, categories: [hostile] }] };
            return undefined;
        });
        const home = h.ED.home.create({ ctx: h.ctx, t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly: false, openFlow: () => {} });
        h.body.appendChild(home.el);
        await settle();
        const homeHits = injected(home.el);
        const cards = home.el.querySelectorAll('[data-ed-flow]').length;
        const doc = flowDoc(hostile);
        doc.nodes.forEach(n => { n.label = hostile + n.label; });
        const editor = openEditor(h, { flow: { draft: doc } });
        await settle();
        editor.ed.bus.emit('connect-picker', { nodeId: T1 });
        h.ED.dialogs.flowSettings(editor.ed);
        eq('c1d07 hostile flow, step, trigger, status and template names render as text on the start page, in the editor and its dialogs',
            [cards, homeHits, injected(editor.el), editor.el.querySelectorAll('.ed-modal').length], [1, [], [], 2]);
    });

    await guardAsync('c1d07 review editor minors', async () => {
        // View storage: written once a pan pauses, and on dispose.
        const h = sandbox(req => (req.method === 'PUT' ? { draft_revision: 4, issues: [] } : undefined));
        let beforeClose = 'unset';
        h.ctx.setWindowBeforeClose = (id, fn) => { beforeClose = fn; };
        const editor = openEditor(h);
        await settle();
        h.runTimers(400);
        let writes = 0;
        const set = h.store.set.bind(h.store);
        h.store.set = (k, v) => { if (String(k).startsWith('aurago.easydrag.view.')) writes++; return set(k, v); };
        const canvasEl = editor.el.querySelector('.ed-canvas');
        const wheel = () => canvasEl.fire('wheel', { deltaX: 0, deltaY: 4, deltaMode: 0, ctrlKey: false, metaKey: false, shiftKey: false, clientX: 500, clientY: 300, preventDefault() {} });
        for (let i = 0; i < 60; i++) wheel();
        const during = writes;
        h.runTimers(400);
        const paused = writes;
        // Keys: only from inside the editor (or the body), never from a content-editable target.
        const outside = h.body.appendChild(new h.dom.El('div', {}));
        const ctrlS = target => h.fireDoc('keydown', h.key('s', target, { ctrlKey: true }));
        editor.ed.model.setFlow({ description: 'keys' });
        ctrlS(outside);
        await settle();
        const fromOutside = h.puts().length;
        ctrlS(h.body);
        await settle();
        const fromBody = h.puts().length;
        h.menus.find(m => m.id === 'edit').items.find(i => i.id === 'select-all').action();
        const editable = canvasEl.appendChild(new h.dom.El('span', {}));
        editable.isContentEditable = true;
        h.fireDoc('keydown', h.key('Delete', editable));
        const kept = editor.ed.model.doc.nodes.length;
        eq('c1d07 the view is stored once a pan pauses; keys from outside the editor or a content-editable target do nothing',
            [during, paused, fromOutside, fromBody, kept], [0, 1, 0, 1, 3]);
        // Focus stays with the failed-save chip's button flow: after a retry it lands on the canvas.
        // Dispose: one module that throws does not keep the rest from being released; a pending view is stored.
        wheel();
        const close = h.ED.detail.close;
        h.ED.detail.close = () => { throw new Error('detail cleanup failed'); };
        editor.dispose();
        h.ED.detail.close = close;
        eq('c1d07 dispose survives a throwing step, releases listeners and the close guard, and stores a pending view',
            [writes, beforeClose, (h.docListeners.keydown || []).length, (h.docListeners['aurago:flows-changed'] || []).length, h.logged.length, h.logged[0] && h.logged[0].includes('detail cleanup failed')],
            [2, null, 0, 0, 1, true]);
    });

    await guardAsync('c1d07 review retry focus, run view restore and duplicate', async () => {
        const saves = [apiError('FLOW_LOCKED')];
        const h = sandbox(req => {
            if (req.method === 'PUT') { const next = saves.shift() || { draft_revision: 4, issues: [] }; if (next instanceof Error) throw next; return next; }
            if (req.url === '/api/desktop/flows/runs/r1?include=doc') return { run: { id: 'r1', status: 'success', mode: 'test', started_at: '2026-10-06T10:00:00Z', revision: 2 }, steps: [], doc: flowDoc('Run') };
            if (req.url === '/api/desktop/flows' && req.method === 'POST') return { flow: { id: 'f2' } };
            return undefined;
        });
        let menu = null;
        h.ctx.showContextMenu = (x, y, items) => { menu = items; };
        const editor = openEditor(h);
        await settle();
        editor.ed.model.setFlow({ description: 'x' });
        h.runTimers(1000);
        await settle();
        const retry = editor.el.querySelector('[data-ed-save] [data-ed-cmd="save"]');
        retry.focus();
        retry.fire('click');
        await settle();
        eq('c1d07 a retried save keeps keyboard focus in the editor (on the canvas)', [editor.ed.saver.state, h.dom.document.activeElement === editor.el.querySelector('.ed-canvas')], ['saved', true]);
        // A restore answered while the run view shows a stored run is still saved; duplicate copies the draft.
        const r = sandbox(req => {
            if (req.method === 'PUT') return { draft_revision: 4, issues: [] };
            if (req.url === '/api/desktop/flows/runs/r1?include=doc') return { run: { id: 'r1', status: 'success', mode: 'test', started_at: '2026-10-06T10:00:00Z', revision: 2 }, steps: [], doc: flowDoc('Run') };
            if (req.url === '/api/desktop/flows' && req.method === 'POST') return { flow: { id: 'f2' } };
            return undefined;
        });
        r.ctx.showContextMenu = (x, y, items) => { menu = items; };
        r.store.set(DRAFT_KEY, JSON.stringify({ revision: 3, at: Date.now(), doc: flowDoc('Local') }));
        const viewer = openEditor(r);
        await settle();
        viewer.showRun('r1');
        await settle();
        r.dialogs[0].el.querySelector('[data-ed-action="restore"]').fire('click');
        await settle();
        r.runTimers(1000);
        await settle();
        const put = r.puts()[0];
        viewer.el.querySelector('[data-ed-cmd="more"]').fire('click');
        await menu.find(i => i.label === 'home_duplicate').action();
        await settle();
        const post = r.requests.find(q => q.method === 'POST' && q.url === '/api/desktop/flows');
        eq('c1d07 a restore answered in the run view is saved, and duplicate copies the draft, not the run',
            [!!viewer.ed.runView, viewer.ed.model.doc.name, put && put.body.doc.name, post && post.body.import.name, r.opened], [true, 'Run', 'Local', 'copy_of:Local', ['f2']]);
        eq('c1d07 the retry and run view checks log no errors', [h.logged, r.logged], [[], []]);
    });

    // ── 1d-10: connect picker port names ──

    await guardAsync('c1d10 the connect picker names switch cases like the canvas', async () => {
        // Switch S (cases "Big" and an unnamed one) already sends case_1 to alpha, so alpha is
        // offered on case_2 and beta on case_1; start is upstream of S and not offered.
        const S = 'n_ssssssss';
        const doc = flowDoc();
        doc.nodes.push({ id: S, key: 'route', type: 'logic.switch', label: 'Route', position: { x: 300, y: 200 }, params: { cases: [{ label: 'Big' }, {}] }, settings: {} });
        doc.edges.push({ id: 'e3', source: { node: T1, port: 'out' }, target: { node: S, port: 'in' } });
        doc.edges.push({ id: 'e4', source: { node: S, port: 'case_1' }, target: { node: A, port: 'in' } });
        const h = sandbox(() => undefined);
        const editor = openEditor(h, { flow: { draft: doc } });
        await settle();
        editor.ed.bus.emit('connect-picker', { nodeId: S });
        const picker = editor.el.querySelector('.ed-modal--picker');
        const html = picker ? picker.parentNode.html : '';
        const chips = Array.from(html.matchAll(/data-ed-pick="([^"]+)"[\s\S]*?<span class="ed-chip ed-chip--muted">([^<]*)<\/span>/g)).map(m => m[1] + '=' + m[2]);
        const node = editor.ed.model.node(S);
        eq('c1d10 the picker shows a case by its label or number, the same text as the canvas port',
            [chips, ['case_1', 'case_2', 'default'].map(p => h.ED.canvas.portLabel(t, node, p)), h.logged],
            [[A + '=port_case:2', B + '=Big'], ['Big', 'port_case:2', 'port_default'], []]);
    });

    // ── 1d-10 review: real menu shortcuts on macOS and Windows ──

    // desktopMenus loads the desktop's window-menu dispatch (menus-and-routing.js and
    // shortcut-runtime.js) for window w1 with the editor's menus; every dispatched action is
    // recorded by item id in `dispatched`.
    function desktopMenus(menus, dispatched) {
        const coreDir = path.join(apps, '..', 'core');
        const routing = fs.readFileSync(path.join(coreDir, 'menus-and-routing.js'), 'utf8').replace(/\r\n?/g, '\n');
        const between = (a, b) => {
            const i = routing.indexOf(a);
            const j = routing.indexOf(b, i);
            if (i < 0 || j < 0) throw new Error('menus-and-routing.js misses ' + a);
            return routing.slice(i, j);
        };
        const box = vm.createContext({ closeWindowMenu() {}, state: { activeWindowId: 'w1', windowMenus: new Map() } });
        vm.runInContext([fs.readFileSync(path.join(coreDir, 'shortcut-runtime.js'), 'utf8'), between('function normalizeWindowMenuItems(', 'function normalizeWindowMenus('),
            between('function runWindowMenuAction(', 'function renderAppContent(')].join('\n') + '\nglobalThis.desktop = { normalizeWindowMenuItems, handleWindowMenuShortcut };', box);
        const actions = new Map();
        const rendered = menus.map(m => ({
            id: m.id,
            items: box.desktop.normalizeWindowMenuItems(m.items.map(i => (typeof i.action === 'function'
                ? Object.assign({}, i, { action: () => { dispatched.push(i.id); return i.action(); } }) : i)), m.id, actions, ['w1', m.id])
        }));
        box.state.windowMenus.set('w1', { renderedMenus: rendered, actions });
        return box.desktop;
    }

    await guardAsync('c1d10 the desktop runs Ctrl+S, Ctrl+Enter and Ctrl+K once on macOS (⌘) and on Windows (Ctrl)', async () => {
        const outcomes = [];
        for (const [platform, modifier] of [['MacIntel', 'metaKey'], ['Win32', 'ctrlKey']]) {
            const h = sandbox(req => (req.method === 'PUT' ? { draft_revision: 4, issues: [] } : undefined), platform);
            const editor = openEditor(h);
            await settle();
            const shortcuts = h.menus.flatMap(m => m.items).filter(i => i.shortcut).map(i => i.id + '=' + i.shortcut);
            const dispatched = [];
            const desktop = desktopMenus(h.menus, dispatched);
            // The editor's own handler: a key the desktop already ran (prevented) returns before
            // core.isMod is consulted, so the count says whether onKeyDown looked at it.
            let editorKeys = 0;
            const isMod = h.ED.core.isMod;
            h.ED.core.isMod = event => { editorKeys++; return isMod(event); };
            let saves = 0;
            const save = editor.ed.saver.save;
            editor.ed.saver.save = (...args) => { saves++; return save.apply(editor.ed.saver, args); };
            const search = editor.el.querySelector('.ed-palette-search');
            let focuses = 0;
            const focus = search.focus.bind(search);
            search.focus = () => { focuses++; focus(); };
            const canvasEl = editor.el.querySelector('.ed-canvas');
            // Ctrl+K first: Ctrl+Enter opens the test dialog, under which Ctrl+K waits.
            for (const key of ['k', 's', 'Enter']) {
                const event = {
                    type: 'keydown', key, code: key === 'Enter' ? 'Enter' : 'Key' + key.toUpperCase(), target: canvasEl, repeat: false,
                    ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, defaultPrevented: false,
                    preventDefault() { this.defaultPrevented = true; }, stopPropagation() {}
                };
                event[modifier] = true;
                desktop.handleWindowMenuShortcut(event); // the desktop's keydown listener runs first
                h.fireDoc('keydown', event); // then the editor's
            }
            await settle();
            outcomes.push([platform, shortcuts, dispatched, editorKeys, saves, focuses, h.logged]);
            h.ED.core.isMod = isMod;
            editor.dispose();
        }
        eq('c1d10 the menu keys are canonical, the desktop dispatches each one once and the editor does not run it again',
            outcomes, ['MacIntel', 'Win32'].map(p => [p, ['save=Ctrl+S', 'test=Ctrl+Enter', 'search=Ctrl+K'], ['search', 'save', 'test'], 0, 1, 1, []]));
    });

    await guardAsync('c1d10 mod+S never reaches the browser and saves nothing under a dialog or while Save is disabled', async () => {
        const answer = req => {
            if (req.method === 'PUT') return { draft_revision: 4, issues: [] };
            if (req.url === '/api/desktop/flows/runs/r1?include=doc') return { run: { id: 'r1', status: 'success', mode: 'test', started_at: '2026-10-06T10:00:00Z', revision: 2 }, steps: [], doc: flowDoc('Run') };
            return undefined;
        };
        // dialog: the shortcut list covers the canvas, Save itself is enabled; run view: Save is
        // disabled and no dialog is open; restore: the restore offer is a dialog and disables Save.
        const states = {
            dialog: (h, editor) => { h.ED.dialogs.shortcuts(editor.ed); },
            'run view': async (h, editor) => { editor.showRun('r1'); await settle(); },
            restore: () => {}
        };
        const outcomes = [];
        for (const [platform, modifier] of [['MacIntel', 'metaKey'], ['Win32', 'ctrlKey']]) {
            for (const [name, enter] of Object.entries(states)) {
                const h = sandbox(answer, platform);
                if (name === 'restore') h.store.set(DRAFT_KEY, JSON.stringify({ revision: 3, at: Date.now(), doc: flowDoc('Local') }));
                const editor = openEditor(h);
                await settle();
                await enter(h, editor);
                let saves = 0;
                const save = editor.ed.saver.save;
                editor.ed.saver.save = (...args) => { saves++; return save.apply(editor.ed.saver, args); };
                const dispatched = [];
                const desktop = desktopMenus(h.menus, dispatched);
                // Focus sits in the dialog when one is open, else on the canvas.
                const target = editor.el.querySelector('.ed-modal button') || editor.el.querySelector('.ed-canvas');
                const event = {
                    type: 'keydown', key: 's', code: 'KeyS', target, repeat: false,
                    ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, defaultPrevented: false,
                    preventDefault() { this.defaultPrevented = true; }, stopPropagation() {}
                };
                event[modifier] = true;
                desktop.handleWindowMenuShortcut(event);
                const seen = h.fireDoc('keydown', event);
                await settle();
                const saveItem = h.menus.flatMap(m => m.items).find(i => i.id === 'save');
                outcomes.push([platform, name, !!saveItem.disabled, !!editor.el.querySelector('.ed-modal-backdrop'), seen.defaultPrevented, saves, h.logged]);
                editor.dispose();
            }
        }
        eq('c1d10 under a dialog, in the run view and under the restore offer mod+S is prevented on macOS and Windows and saves nothing',
            outcomes, ['MacIntel', 'Win32'].flatMap(p => [[p, 'dialog', false, true, true, 0, []], [p, 'run view', true, false, true, 0, []], [p, 'restore', true, true, true, 0, []]]));
    });

    await guardAsync('c1d10 a malformed or unknown route opens the start page and the window context drops the notification ids', async () => {
        const h = sandbox(req => {
            if (req.url.startsWith('/api/desktop/flows/node-types')) return { node_types: Array.from(types.values()), categories: [] };
            if (req.url === '/api/desktop/flows') return { flows: [] };
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
            return undefined; // anything else: FLOW_NOT_FOUND
        });
        const patches = [];
        h.ctx.updateWindowContext = (id, patch) => { patches.push(JSON.stringify(patch)); };
        const screens = [];
        // ".." is refused by the client (FLOW_BAD_REQUEST); the notification's flow is unknown (FLOW_NOT_FOUND).
        for (const route of [{ flowId: '..' }, { flow_id: 'flow_aaaaaaaaaa', run_id: 'run_aaaaaaaaaaaa' }]) {
            const container = h.body.appendChild(new h.dom.El('div', {}));
            h.win.EasyDragApp.render(container, 'w1', Object.assign({}, h.ctx, { api: h.transport }, route));
            await settle();
            screens.push([!!container.querySelector('.ed-home'), !!container.querySelector('.ed-shell-error')]);
            h.win.EasyDragApp.dispose('w1');
        }
        openEditor(h);
        await settle();
        const cleared = JSON.stringify({ flowId: null, flow_id: null, run_id: null });
        eq('c1d10 both routes show the start page with a not-found notice; the start page and the editor clear flow_id and run_id',
            [screens, h.notes.map(n => n.message), patches.slice(0, 2), patches[patches.length - 1], h.logged],
            [[[true, false], [true, false]], ['error_flow_not_found', 'error_flow_not_found'], [cleared, cleared],
                JSON.stringify({ flowId: 'f1', flow_id: null, run_id: null }), []]);
    });

    // ── 1d-11: Mission Control's New flow launches EasyDrag with section "home" ──

    await guardAsync('c1d11 section home', async () => {
        const record = id => ({ flow: { id, name: 'F', draft: flowDoc(), draft_revision: 3, published_draft_revision: 0, live_revision: 0 }, enabled: false, issues: [] });
        const answerWith = put => req => {
            if (req.url.startsWith('/api/desktop/flows/node-types')) return { node_types: Array.from(types.values()), categories: [] };
            if (req.url === '/api/desktop/flows') return { flows: [] };
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
            if (req.method === 'GET' && (req.url === '/api/desktop/flows/f1' || req.url === '/api/desktop/flows/f2')) return record(req.url.slice(-2));
            if (req.method === 'PUT') return put();
            return undefined;
        };
        // stored is the context the shell keeps for a new window; updateWindowContext merges into it.
        async function windowWith(put, launch) {
            const h = sandbox(answerWith(put));
            const stored = Object.assign({}, launch);
            h.ctx.updateWindowContext = (id, patch) => { Object.assign(stored, patch); };
            const container = h.body.appendChild(new h.dom.El('div', {}));
            h.win.EasyDragApp.render(container, 'w1', Object.assign({}, h.ctx, { api: h.transport }, stored));
            await settle();
            h.flushFrames();
            return { h, stored, container, App: h.win.EasyDragApp };
        }
        const screenOf = w => {
            const inst = w.App._instances.get('w1');
            if (!inst || !inst.screen) return 'none';
            return inst.screen.ed ? 'editor:' + inst.screen.ed.flow.id : (w.container.querySelector('.ed-home') ? 'home' : 'other');
        };
        const lists = w => w.h.requests.filter(r => r.method === 'GET' && r.url === '/api/desktop/flows').length;

        // A pending change is saved before the start page shows; New flow gets the focus.
        const saved = await windowWith(() => ({ draft_revision: 4, issues: [] }), { flowId: 'f1' });
        const before = screenOf(saved);
        saved.App._instances.get('w1').screen.ed.model.setFlow({ description: 'pending' });
        saved.App.open('w1', { section: 'home' });
        await settle();
        const put = saved.h.puts()[0];
        eq('c1d11 open({section: "home"}) on an editor with a pending change saves it, then shows the start page with New flow focused',
            [before, put && put.body.doc.description, screenOf(saved), saved.h.dom.document.activeElement === saved.container.querySelector('.ed-home [data-ed-new]'), saved.h.confirms.length, saved.h.logged],
            ['editor:f1', 'pending', 'home', true, 0, []]);

        // A refused leave keeps the editor: the save fails, leave() asks and the answer is "stay".
        const refused = await windowWith(() => { throw apiError('FLOW_LOCKED'); }, { flowId: 'f1' });
        refused.App._instances.get('w1').screen.ed.model.setFlow({ description: 'pending' });
        refused.App.open('w1', { section: 'home' });
        await settle();
        eq('c1d11 a refused leave keeps the editor and shows no start page',
            [screenOf(refused), refused.h.confirms.length, !!refused.container.querySelector('.ed-home'), refused.h.logged], ['editor:f1', 1, false, []]);

        // Re-render safety: a new window drops section "home" from its stored context once read, a start
        // page that is shown already is kept, and a flow opened afterwards wins a later render.
        const launched = await windowWith(() => ({ draft_revision: 4, issues: [] }), { section: 'home' });
        const first = [screenOf(launched), launched.stored.section, lists(launched)];
        launched.App.open('w1', { section: 'home' });
        await settle();
        const again = [screenOf(launched), lists(launched)];
        launched.App.open('w1', { flowId: 'f2' });
        await settle();
        launched.h.flushFrames();
        const opened = [screenOf(launched), launched.stored.flowId];
        launched.App.render(launched.container, 'w1', Object.assign({}, launched.h.ctx, { api: launched.h.transport }, launched.stored));
        await settle();
        launched.h.flushFrames();
        eq('c1d11 section "home" is dropped from the stored context, a shown start page stays, and a re-render after opening a flow stays on that flow',
            [first, again, opened, screenOf(launched), launched.h.logged], [['home', null, 1], ['home', 1], ['editor:f2', 'f2'], 'editor:f2', []]);

        // Overlapping navigations on a failed save share one leave(): one dialog, and the newest navigation wins.
        const overlaps = [];
        for (const answer of [false, true]) {
            const w = await windowWith(() => { throw apiError('FLOW_LOCKED'); }, { flowId: 'f1' });
            w.h.confirmAnswer = answer;
            const editor = w.App._instances.get('w1').screen;
            editor.ed.model.setFlow({ description: 'pending' });
            w.App.open('w1', { section: 'home' });
            w.App.open('w1', { flowId: 'f2' });
            const viaMenu = editor.leave();
            await settle();
            await viaMenu;
            await settle();
            w.h.flushFrames();
            overlaps.push([answer, w.h.confirms.length, screenOf(w), w.h.logged]);
        }
        eq('c1d11 overlapping goHome, showFlow and leave() calls ask at most once; "stay" keeps the editor, "leave" opens the newest route',
            overlaps, [[false, 1, 'editor:f1', []], [true, 1, 'editor:f2', []]]);
    });

    // The FF2 and a1008 checks (test-easydrag-extra5.mjs, -extra6.mjs) build on this sandbox.
    return { sandbox, openEditor, desktopMenus };
}
