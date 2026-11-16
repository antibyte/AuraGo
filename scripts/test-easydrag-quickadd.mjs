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
    await suggestionChecks(env);
    await scopeChecks(env);
    await reviewChecks(env);
}
// suggestionChecks: the entries the quick-add offers first (D.3). The catalog of load() lists curated
// suggestions for the manual trigger (six, one over the limit) and for web.search (the e-mail step first,
// which is not set up).
async function suggestionChecks(env) {
    const { eq, check, guardAsync } = env;
    const FROM_ALPHA = { x: 10, y: 10, from: { node: A, port: 'out' } };

    await guardAsync('w1c D.3 quick-add suggestions', async () => {
        const h = load(env);
        const marked = quick => quick.el.querySelectorAll('[data-ed-suggested]').map(n => n.getAttribute('data-ed-type'));
        const groups = quick => quick.el.querySelectorAll('[data-ed-quick-suggested]').length;
        const listed = (req, history) => {
            if (history !== undefined) h.store.set(AFTER_KEY, JSON.stringify(history)); else h.store.delete(AFTER_KEY);
            const quick = h.open(req);
            const out = { all: h.types(quick), marked: marked(quick), groups: groups(quick), greyed: quick.el.querySelectorAll('.is-unavailable').map(n => n.getAttribute('data-ed-type')) };
            quick.close();
            return out;
        };

        const curated = listed(FROM_ALPHA);
        eq('w1c D.3 without a history the curated list of the source comes first, a type that is not set up greyed, the rest follows without it',
            [curated.all, curated.marked, curated.groups, curated.greyed],
            [['notify.email', 'ai.step', 'web.search', 'http.request', 'notify.telegram', 'logic.if', 'logic.stop'], ['notify.email', 'ai.step'], 1, ['notify.email']]);

        const history = listed(FROM_ALPHA, { 'web.search': { 'notify.telegram': 3, 'logic.if': 1, 'ai.step': 1 } });
        eq('w1c D.3 the own history comes before the curated list, highest count first, and no type appears twice',
            [history.all, history.marked], [['notify.telegram', 'logic.if', 'ai.step', 'notify.email', 'web.search', 'http.request', 'logic.stop'], ['notify.telegram', 'logic.if', 'ai.step', 'notify.email']]);

        const capped = listed({ x: 10, y: 10, from: { node: TRG, port: 'out' } });
        eq('w1c D.3 at most five are suggested; the sixth curated type is listed with the rest',
            [capped.all, capped.marked.length], [['ai.step', 'web.search', 'http.request', 'notify.telegram', 'logic.if', 'notify.email', 'logic.stop'], 5]);

        const mixed = listed({ x: 10, y: 10, from: { node: TRG, port: 'out' } }, { 'trigger.manual': { 'trigger.manual': 9, 'gone.type': 5, 'logic.if': 2 } });
        eq('w1c D.3 history entries that are triggers, gone from the catalog or not candidates are skipped',
            mixed.marked, ['logic.if', 'ai.step', 'web.search', 'http.request', 'notify.telegram']);

        const damaged = ['x', '[1,2]', 'null', '{"web.search":5}', '{"web.search":{"ai.step":"many","gone":-2}}'].map(raw => {
            h.store.set(AFTER_KEY, raw);
            const quick = h.open(FROM_ALPHA);
            const out = marked(quick);
            quick.close();
            return out;
        });
        eq('w1c D.3 a damaged history changes nothing', damaged, damaged.map(() => ['notify.email', 'ai.step']));

        h.store.delete(AFTER_KEY);
        h.store.set('aurago.easydrag.recent', JSON.stringify(['logic.stop', 'ai.step']));
        const recent = listed(FROM_ALPHA);
        eq('w1c D.3 the recently used steps follow the suggestions, without the ones already suggested',
            recent.all, ['notify.email', 'ai.step', 'logic.stop', 'web.search', 'http.request', 'notify.telegram', 'logic.if']);
        h.store.delete('aurago.easydrag.recent');

        const wire = listed({ x: 10, y: 10, edge: 'e2' });
        eq('w1c D.3 Insert on a wire suggests after the wire\'s source step, only steps that have an output',
            [wire.marked, wire.all.includes('logic.stop')], [['notify.email', 'ai.step'], false]);

        const bare = listed({ x: 10, y: 10 });
        const trigger = listed({ x: 10, y: 10, filter: 'trigger' });
        eq('w1c D.3 the empty canvas and the trigger button have no source and no suggestions',
            [bare.groups, bare.marked, trigger.groups, trigger.all], [0, [], 0, ['trigger.manual']]);

        const quick = h.open(FROM_ALPHA);
        const search = quick.el.querySelector('.ed-quick-search');
        search.value = 'Telegram';
        search.fire('input');
        eq('w1c D.3 a search lists matches only, without a suggestions group',
            [h.types(quick), quick.el.querySelectorAll('[data-ed-quick-suggested]').length, marked(quick)], [['notify.telegram'], 0, []]);
        quick.close();
        eq('w1c D.3 the suggestion checks log no errors', h.logged, []);
    });

    await guardAsync('w1c D.3 history of what was added after what', async () => {
        const h = load(env);
        const book = () => JSON.parse(h.store.get(AFTER_KEY) || 'null');
        const pick = (req, type) => {
            const quick = h.open(req);
            const search = quick.el.querySelector('.ed-quick-search');
            search.value = type;
            search.fire('input');
            search.fire('keydown', { key: 'Enter' });
        };
        pick({ x: 10, y: 10, from: { node: A, port: 'out' } }, 'Telegram');
        pick({ x: 10, y: 10, from: { node: A, port: 'out' } }, 'Telegram');
        pick({ x: 10, y: 10, from: { node: A, port: 'out' } }, 'HTTP');
        eq('w1c D.3 a step added after a source is counted under the source\'s type, highest first',
            book(), { 'web.search': { 'notify.telegram': 2, 'http.request': 1 } });

        pick({ x: 10, y: 10, edge: 'e1' }, 'Wenn');
        eq('w1c D.3 a step inserted on a wire counts after the wire\'s source step',
            book()['trigger.manual'], { 'logic.if': 1 });

        const before = JSON.stringify(book());
        pick({ x: 10, y: 10 }, 'Wenn');
        h.ED.palette.place(h.ed, 'logic.stop', { x: 900, y: 0 }, { edge: 'e2' });
        h.ED.palette.place(h.ed, 'trigger.manual', { x: 900, y: 200 }, { from: { node: A, port: 'out' } });
        eq('w1c D.3 a step added on the empty canvas, beside a wire it could not cut, or a trigger counts nothing', JSON.stringify(book()), before);

        for (let i = 0; i < 3; i++) pick({ x: 10, y: 10, from: { node: B, port: 'out' } }, 'KI-Schritt');
        eq('w1c D.3 the history is per source type', Object.keys(book()), ['web.search', 'trigger.manual', 'ai.step']);
        check('w1c D.3 the book keeps only objects of counts', Object.values(book()).every(row => Object.values(row).every(n => Number.isInteger(n) && n > 0)));
        eq('w1c D.3 the history checks log no errors', h.logged, []);
    });
}
// scopeChecks: "Show all" and the scoped palette (D.4). panelOf adds the real palette panel to load()'s editor.
function panelOf(h) {
    const panel = h.ED.palette.createPanel(h.ed, h.canvas);
    h.root.appendChild(panel.el);
    const banner = panel.el.querySelector('[data-ed-palette-scope]');
    return {
        panel, banner,
        search: panel.el.querySelector('.ed-palette-search'),
        item: type => panel.el.querySelectorAll('[data-ed-type]').find(n => n.getAttribute('data-ed-type') === type),
        scoped: () => !banner.attrs.has('hidden'),
        text: () => banner.querySelector('.ed-palette-scope-text').textContent,
        // choose presses Enter on a palette entry, as a keyboard user does; a click ends in the same function.
        choose(type) { this.item(type).fire('keydown', { key: 'Enter' }); },
        // showAll opens the quick-add for req and clicks its "Show all" link.
        showAll(req) {
            const quick = h.open(req);
            quick.el.querySelector('[data-ed-quick-all]').fire('click');
            return quick;
        }
    };
}

async function scopeChecks(env) {
    const { eq, check, guardAsync } = env;
    const FROM_ALPHA = { x: 10, y: 10, from: { node: A, port: 'out' } };
    const added = h => h.model.doc.nodes.filter(n => ![TRG, A, B, GAMMA].includes(n.id));

    await guardAsync('w1c D.4 Show all hands the source to the palette', async () => {
        const h = load(env);
        const p = panelOf(h);
        const quick = p.showAll(FROM_ALPHA);
        eq('w1c D.4 the link closes the quick-add and focuses the palette search, not the canvas',
            [h.ed.quickAdd, h.root.querySelectorAll('.ed-quick').length, h.dom.document.activeElement === p.search, h.dom.document.activeElement === h.canvas.el], [null, 0, true, false]);
        eq('w1c D.4 the palette shows "Adds after <step>" with a Cancel button', [p.scoped(), p.text(), !!p.banner.querySelector('[data-ed-palette-scope-cancel]')], [true, 'palette_scope:Alpha', true]);
        check('w1c D.4 the quick-add is gone for good (no handler left that would close it twice)', quick.el.parentNode === null);

        p.choose('http.request');
        const first = added(h)[0];
        eq('w1c D.4 the next step chosen in the palette goes after the source and ends the scope',
            [added(h).length, first.type, h.model.incoming(first.id).map(e => e.source.node), p.scoped(), h.model.node(Array.from(h.ed.selection)[0]).id === first.id], [1, 'http.request', [A], false, true]);
        h.ed.selection.clear();
        p.choose('notify.telegram');
        const second = added(h)[1];
        eq('w1c D.4 the step after that is a plain palette add again (no selection, no source)', [added(h).length, h.model.incoming(second.id).length], [2, 0]);
        eq('w1c D.4 the scoped add counts in the history like a quick-add', JSON.parse(h.store.get(AFTER_KEY)), { 'web.search': { 'http.request': 1 } });
        eq('w1c D.4 the scope checks log no errors', h.logged, []);
    });

    await guardAsync('w1c D.4 the palette places the step where the quick-add would', async () => {
        const req = { x: 900, y: 120, from: { node: A, port: 'out' } };
        const viaQuick = load(env);
        const q = viaQuick.open(req);
        const search = q.el.querySelector('.ed-quick-search');
        search.value = 'HTTP';
        search.fire('input');
        search.fire('keydown', { key: 'Enter' });
        const viaPalette = load(env);
        const p = panelOf(viaPalette);
        p.showAll(req);
        p.choose('http.request');
        const a = added(viaQuick)[0].position;
        const b = added(viaPalette)[0].position;
        eq('w1c D.4 a wire pulled from a port and dropped far away puts the step at the drop point either way', [a, b.x === a.x && b.y === a.y, a.x > 600], [a, true, true]);

        const wireQuick = load(env);
        const wq = wireQuick.open({ x: 10, y: 10, edge: 'e2' });
        const wqSearch = wq.el.querySelector('.ed-quick-search');
        wqSearch.value = 'HTTP';
        wqSearch.fire('input');
        wqSearch.fire('keydown', { key: 'Enter' });
        const wirePalette = load(env);
        const wp = panelOf(wirePalette);
        wp.showAll({ x: 10, y: 10, edge: 'e2' });
        eq('w1c D.4 the scope of an inserted wire says the wire\'s source step', wp.text(), 'palette_scope:Alpha');
        wp.choose('http.request');
        const ends = h => { const n = added(h)[0]; return [h.model.incoming(B).map(e => e.source.node)[0] === n.id, h.model.incoming(n.id).map(e => e.source.node), h.model.doc.edges.some(e => e.id === 'e2'), n.position]; };
        eq('w1c D.4 a wire scope inserts between the two steps exactly as the quick-add does', [ends(wirePalette), JSON.stringify(ends(wirePalette)) === JSON.stringify(ends(wireQuick))], [[true, [A], false, ends(wirePalette)[3]], true]);
    });

    await guardAsync('w1c D.4 ending the scope', async () => {
        const h = load(env);
        const p = panelOf(h);
        p.showAll(FROM_ALPHA);
        p.banner.querySelector('[data-ed-palette-scope-cancel]').fire('click');
        eq('w1c D.4 Cancel ends the scope and leaves the focus in the search', [p.scoped(), h.dom.document.activeElement === p.search], [false, true]);
        p.choose('http.request');
        eq('w1c D.4 a step chosen after Cancel is not connected', [added(h).length, h.model.incoming(added(h)[0].id).length], [1, 0]);

        p.showAll(FROM_ALPHA);
        const unscoped = p.panel.el.fire('keydown', { key: 'Escape' });
        eq('w1c D.4 Escape ends the scope, is taken (the editor does not see it) and puts the focus on the canvas',
            [p.scoped(), unscoped.stopped, unscoped.defaultPrevented, h.dom.document.activeElement === h.canvas.el], [false, true, true, true]);
        const plain = p.panel.el.fire('keydown', { key: 'Escape' });
        eq('w1c D.4 without a scope Escape is left to the editor', [plain.stopped, plain.defaultPrevented], [false, false]);

        p.showAll(FROM_ALPHA);
        p.panel.setOpen(false);
        eq('w1c D.4 hiding the palette ends the scope', p.scoped(), false);

        p.showAll(FROM_ALPHA);
        h.model.removeNodes([A]);
        const count = h.model.doc.nodes.length;
        p.choose('notify.telegram');
        eq('w1c D.4 a source that is gone by then adds nothing, says so and ends the scope',
            [h.model.doc.nodes.length === count, h.said.includes('quick_target_gone'), p.scoped()], [true, true, false]);
        eq('w1c D.4 the ending checks log no errors', h.logged, []);
    });

    await guardAsync('w1c D.4 Enter in the scoped search', async () => {
        const h = load(env);
        const p = panelOf(h);
        const type = text => { p.search.value = text; p.search.fire('input'); h.runTimers(); };
        type('Telegram');
        p.search.fire('keydown', { key: 'Enter' });
        eq('w1c D.4 without a scope Enter in the search adds nothing, as before', added(h).length, 0);
        p.showAll(FROM_ALPHA);
        p.search.value = '';
        p.search.fire('keydown', { key: 'Enter' });
        eq('w1c D.4 with a scope but no query Enter adds nothing', [added(h).length, p.scoped()], [0, true]);
        type('Telegram');
        p.search.fire('keydown', { key: 'Enter' });
        const step = added(h)[0];
        eq('w1c D.4 with a scope Enter adds the first match after the source', [step.type, h.model.incoming(step.id).map(e => e.source.node), p.scoped()], ['notify.telegram', [A], false]);
    });

    await guardAsync('w1c D.4 entries without a source and guards', async () => {
        const h = load(env);
        const p = panelOf(h);
        p.showAll({ x: 10, y: 10 });
        eq('w1c D.4 from the empty canvas Show all opens the palette with the search focused and no scope', [p.scoped(), h.dom.document.activeElement === p.search, h.ed.quickAdd], [false, true, null]);
        p.showAll({ x: 10, y: 10, filter: 'trigger' });
        eq('w1c D.4 the trigger button\'s link does the same', [p.scoped(), h.dom.document.activeElement === p.search], [false, true]);

        h.ed.readonly = true;
        h.ed.bus.emit('palette-scope', { from: { node: A, port: 'out' }, edge: undefined, name: 'Alpha', world: () => ({ x: 0, y: 0 }) });
        eq('w1c D.4 a read-only editor takes no scope', p.scoped(), false);
        h.ed.readonly = false;

        p.panel.dispose();
        h.ed.bus.emit('palette-scope', { from: { node: A, port: 'out' }, name: 'Alpha', world: () => ({ x: 0, y: 0 }) });
        eq('w1c D.4 a disposed palette no longer listens', p.scoped(), false);
        eq('w1c D.4 the guard checks log no errors', h.logged, []);
    });

    await guardAsync('w1c D.4 keyboard path of the quick-add', async () => {
        const h = load(env);
        const p = panelOf(h);
        const quick = h.open(FROM_ALPHA);
        const search = quick.el.querySelector('.ed-quick-search');
        const link = quick.el.querySelector('[data-ed-quick-all]');
        const tab = search.fire('keydown', { key: 'Tab' });
        eq('w1c D.4 Tab in the search moves to Show all and keeps the quick-add open', [h.dom.document.activeElement === link, !!h.ed.quickAdd, tab.defaultPrevented], [true, true, true]);
        link.fire('keydown', { key: 'Tab', shiftKey: true });
        eq('w1c D.4 Shift+Tab on the link goes back to the search', [h.dom.document.activeElement === search, !!h.ed.quickAdd], [true, true]);
        search.fire('keydown', { key: 'Tab' });
        link.fire('keydown', { key: 'Tab' });
        eq('w1c D.4 Tab on the link closes the quick-add and gives the focus back to the canvas', [h.ed.quickAdd, h.dom.document.activeElement === h.canvas.el], [null, true]);
        const again = h.open(FROM_ALPHA);
        again.el.querySelector('.ed-quick-search').fire('keydown', { key: 'Tab', shiftKey: true });
        eq('w1c D.4 Shift+Tab in the search and Escape close it as before', [h.ed.quickAdd, p.scoped()], [null, false]);
        const third = h.open(FROM_ALPHA);
        third.el.querySelector('[data-ed-quick-all]').fire('keydown', { key: 'Escape' });
        eq('w1c D.4 Escape on the link closes it too', h.ed.quickAdd, null);
        eq('w1c D.4 the keyboard checks log no errors', h.logged, []);
    });
}

// reviewChecks: the review fixes of Part D (history keeps learning, scoped Enter only adds steps that fit, focus goes back).
async function reviewChecks(env) {
    const { eq, check, guardAsync } = env;
    const FROM_ALPHA = { x: 10, y: 10, from: { node: A, port: 'out' } };
    const added = h => h.model.doc.nodes.filter(n => ![TRG, A, B, GAMMA].includes(n.id));

    await guardAsync('w1c D.R history keeps learning', async () => {
        const h = load(env);
        const row = {};
        for (let i = 1; i <= 8; i++) row['old.type' + i] = 3;
        h.store.set(AFTER_KEY, JSON.stringify({ 'web.search': row }));
        const addNew = () => h.ED.palette.place(h.ed, 'http.request', { x: 900, y: 100 }, { from: { node: A, port: 'out' } });
        addNew();
        let book = JSON.parse(h.store.get(AFTER_KEY))['web.search'];
        eq('w1c D.R after 8 types a 9th is kept and the least used goes', [Object.keys(book).length, book['http.request']], [8, 1]);
        addNew(); addNew(); addNew();
        book = JSON.parse(h.store.get(AFTER_KEY))['web.search'];
        const top = Object.entries(book).sort((a, b) => b[1] - a[1])[0];
        eq('w1c D.R after three more adds it ranks first', [top[0], top[1]], ['http.request', 4]);

        const many = {};
        for (let i = 0; i < 40; i++) many['s' + i] = { 'ai.step': 1 };
        h.store.set(AFTER_KEY, JSON.stringify(many));
        addNew();
        const keys = Object.keys(JSON.parse(h.store.get(AFTER_KEY)));
        eq('w1c D.R the 41st source evicts the one written longest ago', [keys.length, keys.includes('s0'), keys.includes('s1'), keys[keys.length - 1]], [40, false, true, 'web.search']);
    });

    await guardAsync('w1c D.R scoped Enter and clicks only add steps that fit', async () => {
        const h = load(env);
        const p = panelOf(h);
        p.showAll(FROM_ALPHA);
        p.search.value = 'Start';
        p.search.fire('keydown', { key: 'Enter' });
        eq('w1c D.R Enter skips a trigger match and keeps the scope', [added(h).length, p.scoped()], [0, true]);
        p.choose('trigger.manual');
        eq('w1c D.R a click on a trigger keeps the scope and adds nothing', [added(h).length, p.scoped(), h.said.includes('connect_port')], [0, true, true]);
        p.search.value = 'Telegram';
        p.search.fire('keydown', { key: 'Enter' });
        eq('w1c D.R Enter right after typing (no render yet) adds the fitting match', [added(h).map(n => n.type), p.scoped()], [['notify.telegram'], false]);

        const w = load(env);
        const q = panelOf(w);
        q.showAll({ x: 10, y: 10, edge: 'e2' });
        q.search.value = 'Stopp';
        q.search.fire('keydown', { key: 'Enter' });
        eq('w1c D.R under a wire scope a step without outputs is not added', [added(w).length, q.scoped()], [0, true]);
        eq('w1c D.R the review checks log no errors', [h.logged, w.logged], [[], []]);
    });

    await guardAsync('w1c D.R focus returns to the inspector', async () => {
        const h = load(env);
        const p = panelOf(h);
        let called = 0;
        p.showAll(Object.assign({ restoreFocus: () => { called++; } }, FROM_ALPHA));
        p.choose('http.request');
        eq('w1c D.R restoreFocus runs after a scoped add', called, 1);
        p.showAll(Object.assign({ restoreFocus: () => { called++; } }, FROM_ALPHA));
        p.panel.el.fire('keydown', { key: 'Escape' });
        eq('w1c D.R restoreFocus runs on Escape instead of the canvas', [called, h.dom.document.activeElement === h.canvas.el], [2, false]);
        p.showAll(FROM_ALPHA);
        p.panel.el.fire('keydown', { key: 'Escape', isComposing: true });
        check('w1c D.R Escape during composition keeps the scope', p.scoped());
    });
}
