// W1c Part D: the quick-add. D.2 checks its header text for every entry path, D.3 the suggestions,
// D.4 "Show all" and the scoped palette. It runs on the miniDom stub of test-easydrag.mjs, which calls
// run(env); alone this file only has to import. The palette and the model are the real modules; the
// canvas is a small stub (the quick-add reads clientToWorld, nodeRect and the canvas element).
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const MODULES = ['easydrag-core.js', 'easydrag-format.js', 'easydrag-template.js', 'easydrag-model.js', 'easydrag-geometry.js', 'easydrag-palette.js'];
const TRG = 'n_tttttttt';
const A = 'n_aaaaaaaa';
const B = 'n_bbbbbbbb';
const GAMMA = 'n_cccccccc';
const AFTER_KEY = 'aurago.easydrag.after';

// docOf is the flow start -> alpha -> beta, plus an unlabelled step gamma (it goes by its type's label).
const docOf = () => ({
    schema: 1, name: 'Quick',
    nodes: [
        { id: TRG, key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} },
        { id: A, key: 'alpha', type: 'web.search', label: 'Alpha', position: { x: 300, y: 0 }, params: {}, settings: {} },
        { id: B, key: 'beta', type: 'ai.step', label: 'Beta', position: { x: 600, y: 0 }, params: {}, settings: {} },
        { id: GAMMA, key: 'gamma', type: 'web.search', label: '', position: { x: 600, y: 300 }, params: {}, settings: {} }
    ],
    edges: [
        { id: 'e1', source: { node: TRG, port: 'out' }, target: { node: A, port: 'in' } },
        { id: 'e2', source: { node: A, port: 'out' }, target: { node: B, port: 'in' } }
    ]
});

const infoOf = (type, label, category, extra) => Object.assign({ type, version: 1, label, category, icon: 'search', inputs: ['in'], outputs: ['out'], params: [], availability: { state: 'available' } }, extra || {});
// The catalog the checks use: web.search and the manual trigger carry curated suggestions the way the
// server sends them (the trigger's list has six entries, one more than the quick-add shows).
const catalogOf = () => [
    infoOf('trigger.manual', 'Start', 'trigger', { trigger: true, inputs: [], suggest: ['ai.step', 'web.search', 'http.request', 'notify.telegram', 'logic.if', 'notify.email'] }),
    infoOf('ai.step', 'KI-Schritt', 'ai'),
    infoOf('web.search', 'Websuche', 'web', { suggest: ['notify.email', 'ai.step'] }),
    infoOf('http.request', 'HTTP-Anfrage', 'web'),
    infoOf('notify.telegram', 'Telegram', 'notify'),
    infoOf('notify.email', 'E-Mail', 'notify', { availability: { state: 'needs_setup', config_section: 'email' } }),
    infoOf('logic.if', 'Wenn', 'logic', { outputs: ['true', 'false'] }),
    infoOf('logic.stop', 'Stopp', 'logic', { outputs: [] })
];
const CATEGORIES = [{ id: 'trigger', label: 'Trigger' }, { id: 'ai', label: 'KI' }, { id: 'web', label: 'Web' }, { id: 'notify', label: 'Benachrichtigen' }, { id: 'logic', label: 'Logik' }];

// load runs the real palette, model and geometry on miniDom. h.open(req) opens the quick-add, h.types(quick)
// lists its entries, h.said holds the announcements, h.runTimers() fires the pending timeouts.
function load(env) {
    const { apps, miniDom, t } = env;
    const dom = miniDom();
    const proto = dom.El.prototype;
    // The search fields are read at render: an input answers with its typed value, else its attribute.
    Object.defineProperty(proto, 'value', {
        configurable: true,
        get() { return this.typed !== undefined ? this.typed : (this.attrs.get('value') || ''); },
        set(v) { this.typed = String(v); }
    });
    proto.scrollIntoView = function () {};
    proto.select = function () {};
    proto.removeAttribute = function (name) { this.attrs.delete(name); };
    const docListeners = {};
    Object.assign(dom.document, {
        addEventListener(type, fn) { (docListeners[type] = docListeners[type] || []).push(fn); },
        removeEventListener(type, fn) { const list = docListeners[type] || []; const i = list.indexOf(fn); if (i >= 0) list.splice(i, 1); }
    });
    const store = new Map();
    const timers = [];
    const logged = [];
    const box = vm.createContext({
        window: {}, navigator: { platform: 'Linux' }, crypto: webcrypto, document: dom.document,
        console: { log() {}, warn() {}, error: (...args) => { logged.push(args.map(String).join(' ')); } },
        localStorage: { getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); } },
        setTimeout: fn => { timers.push(fn); return timers.length; }, clearTimeout() {},
        requestAnimationFrame: fn => { fn(); return 1; },
        matchMedia: () => ({ matches: false }), CSS: { escape: value => String(value) }
    });
    for (const file of MODULES) vm.runInContext(fs.readFileSync(path.join(apps, file), 'utf8'), box, { filename: file });
    const ED = box.window.EasyDrag;
    const G = ED.geometry;
    const list = catalogOf();
    const types = new Map(list.map(info => [info.type, info]));
    const model = ED.model.create(docOf(), { types });
    const said = [];
    const root = new dom.El('div', { class: 'ed-editor' });
    const canvasEl = root.appendChild(new dom.El('div', { class: 'ed-canvas' }));
    const canvas = {
        el: canvasEl,
        nodeRect: id => { const n = model.node(id); return n ? G.nodeRect(n.position, 1) : null; },
        clientToWorld: (x, y) => ({ x, y }),
        centerOn() {}, size: () => ({ w: 1000, h: 600 })
    };
    const ed = {
        t, esc: ED.core.esc, windowId: 'w1', readonly: false, runView: null, ctx: { notify() {} },
        selection: new Set(), view: { x: 0, y: 0, zoom: 1 }, bus: ED.core.emitter(), catalog: { types, list, categories: CATEGORIES },
        model, root, announce: text => said.push(text), quickAdd: null
    };
    return {
        ED, ed, canvas, model, dom, root, store, said, logged,
        open: req => ED.palette.openQuickAdd(ed, canvas, req),
        types: quick => quick.el.querySelectorAll('[data-ed-index]').map(n => n.getAttribute('data-ed-type')),
        runTimers() { while (timers.length) timers.shift()(); }
    };
}

async function headerChecks(env) {
    const { eq, check, guardAsync } = env;

    await guardAsync('w1c D.2 quick-add header', async () => {
        const h = load(env);
        // head opens a quick-add and returns the dialog's accessible name and its header row text.
        const head = req => {
            const quick = h.open(req);
            const out = [quick.el.getAttribute('aria-label'), quick.el.querySelector('[data-ed-quick-head]').textContent, quick.el.children[0].attrs.has('data-ed-quick-head')];
            quick.close();
            return out;
        };
        const same = text => [text, text, true];
        eq('w1c D.2 a step\'s "+", a card tool, Ctrl+K and a wire pulled from a port head "Steps after: <step>"',
            [head({ x: 10, y: 10, from: { node: A, port: 'out' } }), head({ x: 10, y: 10, from: { node: TRG, port: 'out' } })],
            [same('quick_head_after:Alpha'), same('quick_head_after:Start')]);
        eq('w1c D.2 a step without a label goes by its type\'s label', head({ x: 10, y: 10, from: { node: GAMMA, port: 'out' } }), same('quick_head_after:Websuche'));
        eq('w1c D.2 Insert on a wire heads "Insert after: <the wire\'s source step>"', head({ x: 10, y: 10, edge: 'e2' }), same('quick_head_insert:Alpha'));
        eq('w1c D.2 the empty canvas and the canvas menu head "Add step", the trigger button "Choose a trigger"',
            [head({ x: 10, y: 10 }), head({ x: 10, y: 10, filter: 'trigger' })], [same('quick_title'), same('empty_action')]);
        eq('w1c D.2 a source that is gone falls back to the plain heading',
            [head({ x: 10, y: 10, from: { node: 'n_gone0000', port: 'out' } }), head({ x: 10, y: 10, edge: 'e_gone' })], [same('quick_title'), same('quick_title')]);

        // A label with markup stays text: the attribute is escaped, the row is set as text.
        h.model.setLabel(A, 'A<b>&"x');
        const quick = h.open({ x: 10, y: 10, from: { node: A, port: 'out' } });
        eq('w1c D.2 the step name is escaped in the accessible name and plain text in the row',
            [/[<>"]/.test(quick.el.getAttribute('aria-label')), quick.el.querySelector('[data-ed-quick-head]').textContent], [false, 'quick_head_after:A<b>&"x']);
        quick.close();
        eq('w1c D.2 the header checks log no errors', h.logged, []);
    });

    await guardAsync('w1c D.2 quick-add position uses the real height', async () => {
        const h = load(env);
        // The editor is 1000 x 300 px; a popover that is 200 px tall must fit below y = 280 at 92 px
        // (300 - 200 - 8), where the fixed 380 px of before pushed it above the editor.
        h.root.getBoundingClientRect = () => ({ left: 0, top: 0, right: 1000, bottom: 300, width: 1000, height: 300 });
        Object.defineProperty(h.dom.El.prototype, 'offsetHeight', { configurable: true, get() { return this.className.includes('ed-quick') ? 200 : 0; } });
        const quick = h.open({ x: 100, y: 280 });
        check('w1c D.2 position() clamps with the measured height', /,92px\)$/.test(quick.el.style.transform), quick.el.style.transform);
        quick.close();
        delete h.dom.El.prototype.offsetHeight;
        const fallback = h.open({ x: 100, y: 280 });
        check('w1c D.2 without a measured height the popover keeps the 380 px it had', /,-88px\)$/.test(fallback.el.style.transform), fallback.el.style.transform);
        fallback.close();
    });
}

export async function run(env) {
    await headerChecks(env);
}
