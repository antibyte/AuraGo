// c1d04 checks: canvas, wires and interact; c1d05 checks: palette, fields, forms and mapping. Both
// run on the stub DOM of test-easydrag.mjs. That runner calls run(env) with its helpers (check, eq,
// guardAsync, miniDom, ...); failures are counted there.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const MODULES = ['easydrag-core.js', 'easydrag-template.js', 'easydrag-model.js', 'easydrag-geometry.js', 'easydrag-canvas.js', 'easydrag-wires.js', 'easydrag-interact.js',
    'easydrag-palette.js', 'easydrag-fields.js', 'easydrag-forms.js', 'easydrag-mapping.js'];
const A = 'n_aaaaaaaa';
const B = 'n_bbbbbbbb';
const C = 'n_cccccccc';

// twoNodes is a flow alpha -> beta whose beta step references alpha; more adds nodes and edges.
function twoNodes(edgeId, more) {
    const extra = more || {};
    return {
        schema: 1, name: 'Two',
        nodes: [
            { id: A, key: 'alpha', type: 'web.search', label: 'Alpha', position: { x: 0, y: 0 }, params: { query: 'x' }, settings: {} },
            { id: B, key: 'beta', type: 'web.search', label: 'Beta', position: { x: 600, y: 0 }, params: { query: '{{alpha.results}}' }, settings: {} }
        ].concat(extra.nodes || []),
        edges: [{ id: edgeId || 'e1', source: { node: A, port: 'out' }, target: { node: B, port: 'in' } }].concat(extra.edges || [])
    };
}

export async function run(env) {
    const { apps, types, t, miniDom, check, eq, guardAsync, settle } = env;
    // A summary makes card markup show the labels of referenced steps.
    const canvasTypes = new Map(types);
    canvasTypes.set('web.search', Object.assign({}, types.get('web.search'), { summary: '{query}' }));

    // harness runs the canvas modules in a sandbox. Timeouts and animation frames are recorded and fired
    // by hand; ResizeObserver, localStorage and navigator are stubs; model changes reach the bus as the
    // editor forwards them. Every element keeps the markup it was given (el.html); counts.cards counts
    // card markup rebuilds and counts.minimapNodes rebuilds of the minimap node layer. Interact's
    // announcements land in h.announced, console.error lines in h.logged.
    function harness(doc) {
        const dom = miniDom();
        const proto = dom.El.prototype;
        const setHTML = Object.getOwnPropertyDescriptor(proto, 'innerHTML').set;
        const counts = { cards: 0, minimapNodes: 0 };
        Object.defineProperty(proto, 'innerHTML', {
            configurable: true,
            set(value) {
                this.html = String(value);
                if (this.attrs.has('data-node-id')) counts.cards++;
                if (this.className === 'ed-minimap-nodes') counts.minimapNodes++;
                setHTML.call(this, value);
            }
        });
        dom.document.elementFromPoint = () => null;
        // Element and document methods the 1d-05 modules use; document listeners land in
        // dom.document.listeners.
        proto.removeAttribute = function (name) { this.attrs.delete(name); };
        proto.setSelectionRange = function (start, end) { this.selectionStart = start; this.selectionEnd = end; };
        proto.scrollIntoView = function () {};
        // The constructor's "this.style = {}" goes through this setter, which adds setProperty.
        Object.defineProperty(proto, 'style', {
            configurable: true,
            get() { return this.styles; },
            set(value) { this.styles = Object.assign(value, { setProperty(name, v) { this[name] = String(v); } }); }
        });
        const docListeners = {};
        Object.assign(dom.document, {
            listeners: docListeners,
            addEventListener(type, fn) { (docListeners[type] = docListeners[type] || []).push(fn); },
            removeEventListener(type, fn) { const list = docListeners[type] || []; const i = list.indexOf(fn); if (i >= 0) list.splice(i, 1); }
        });
        const timers = new Map();
        const frames = new Map();
        let nextId = 1;
        const observers = { observed: 0, disconnected: 0 };
        const store = new Map();
        const logged = [];
        const navigator = { platform: 'Linux' };
        const box = vm.createContext({
            window: {}, navigator, crypto: webcrypto, document: dom.document,
            console: { log() {}, warn() {}, error: (...args) => { logged.push(args.map(String).join(' ')); } },
            localStorage: { getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); } },
            setTimeout: (fn, ms) => { const id = nextId++; timers.set(id, { fn, ms }); return id; },
            clearTimeout: id => { timers.delete(id); },
            requestAnimationFrame: fn => { const id = nextId++; frames.set(id, fn); return id; },
            cancelAnimationFrame: id => { frames.delete(id); },
            matchMedia: () => ({ matches: false }),
            CSS: { escape: value => String(value) },
            ResizeObserver: class { observe() { observers.observed++; } disconnect() { observers.disconnected++; } }
        });
        for (const file of MODULES) {
            const full = path.join(apps, file);
            vm.runInContext(fs.readFileSync(full, 'utf8'), box, { filename: full });
        }
        const ED = box.window.EasyDrag;
        const bus = ED.core.emitter();
        const quickAdds = [];
        const details = [];
        let selections = 0;
        bus.on('quick-add', p => quickAdds.push(p));
        bus.on('open-detail', p => details.push(p));
        bus.on('selection', () => { selections++; });
        // subscriptions holds the bus listeners the modules have not removed yet.
        const subscriptions = new Set();
        const on = bus.on;
        bus.on = (type, fn) => {
            const token = { type };
            subscriptions.add(token);
            const off = on(type, fn);
            return () => { subscriptions.delete(token); off(); };
        };
        const model = ED.model.create(doc || { schema: 1, name: 'Canvas', nodes: [], edges: [] }, { types: canvasTypes });
        model.on(change => bus.emit('model', change));
        const ed = {
            ctx: {}, t, esc: ED.core.esc, catalog: { types: canvasTypes }, readonly: false, model, bus, initialRender: true,
            selection: new Set(), selectedEdge: null, view: { x: 0, y: 0, zoom: 1 }, run: null, issues: []
        };
        const canvas = ED.canvas.create(ed);
        const wires = ED.wires.create(ed, canvas);
        const announced = [];
        const announce = canvas.announce;
        const interact = ED.interact.create(ed, Object.assign({}, canvas, { announce: text => { announced.push(text); announce(text); } }), wires);
        const h = {
            ED, ed, canvas, wires, interact, model, bus, el: canvas.el, dom, store, navigator, announced, logged, counts, timers, frames,
            observers, subscriptions, quickAdds, details, selections: () => selections,
            card: id => canvas.nodeEl(id),
            pos: id => Object.assign({}, model.node(id).position),
            pe: (id, x, y, extra) => Object.assign({ pointerId: id, clientX: x, clientY: y, button: 0, buttons: 1, pointerType: 'mouse', shiftKey: false, ctrlKey: false, metaKey: false, altKey: false }, extra),
            key: (k, target, extra) => Object.assign({ key: k, target: target || canvas.el, repeat: false, shiftKey: false, ctrlKey: false, metaKey: false, altKey: false }, extra),
            flushFrames() {
                for (let i = 0; frames.size && i < 20; i++) {
                    const [id, fn] = frames.entries().next().value;
                    frames.delete(id);
                    fn();
                }
            },
            runTimers(ms) {
                for (const [id, entry] of Array.from(timers)) {
                    if (ms === undefined || entry.ms === ms) { timers.delete(id); entry.fn(); }
                }
            }
        };
        // The editor draws the document once, then later cards are new ones.
        canvas.render();
        wires.render();
        h.flushFrames();
        ed.initialRender = false;
        return h;
    }

    await guardAsync('c1d04 viewport coercion', async () => {
        const h = harness();
        const grid = h.el.querySelector('.ed-grid');
        const mini = h.el.querySelector('.ed-minimap-view');
        const seen = [];
        const drawn = [];
        // A corrupt stored viewport (aurago.easydrag.view.<id>) or a NaN must not reach the transforms.
        for (const view of [JSON.parse('{"x":"a","zoom":1}'), { x: NaN, y: Infinity, zoom: NaN }, { x: '12', y: null, zoom: '2' }, null, 'oops']) {
            h.canvas.setView(view);
            h.flushFrames();
            seen.push([h.ed.view, h.canvas.world.style.transform]);
            drawn.push(grid.style.backgroundSize, grid.style.backgroundPosition, ['x', 'y', 'width', 'height'].map(a => mini.getAttribute(a)));
        }
        const zero = [{ x: 0, y: 0, zoom: 1 }, 'translate(0px,0px) scale(1)'];
        eq('c1d04 setView turns an x, y or zoom that is not a finite number into 0, 0 and 1', seen, [zero, zero, zero, zero, zero]);
        check('c1d04 grid and minimap get no NaN from a bad view', !/NaN/.test(JSON.stringify(drawn)) && mini.getAttribute('width') !== null, JSON.stringify(drawn.slice(0, 3)));
        h.canvas.setView({ x: 12.5, y: -40, zoom: 5 });
        h.flushFrames();
        eq('c1d04 setView keeps finite coordinates and clamps the zoom', [h.ed.view, h.canvas.world.style.transform], [{ x: 12.5, y: -40, zoom: 2 }, 'translate(12.5px,-40px) scale(2)']);
        eq('c1d04 the viewport checks log no errors', h.logged, []);
    });

    await guardAsync('c1d04 paste refusal', async () => {
        const h = harness();
        const KEY = 'aurago.easydrag.clipboard';
        const kept = h.model.addNode('web.search', { x: 0, y: 0 });
        h.interact.select([kept]);
        const tooMany = { easydrag: 1, nodes: Array.from({ length: 501 }, (_, i) => ({ id: 'n' + i, type: 'web.search' })), edges: [] };
        h.store.set(KEY, JSON.stringify(tooMany));
        await h.interact.pasteAt({ x: 0, y: 0 });
        eq('c1d04 a refused fragment is announced and keeps the selection', [h.model.doc.nodes.length, h.announced, Array.from(h.ed.selection)], [1, ['paste_refused'], [kept]]);
        h.store.set(KEY, JSON.stringify({ easydrag: 1, nodes: [{ id: 'a', type: 'evil.type' }], edges: [] }));
        await h.interact.pasteAt({ x: 0, y: 0 });
        eq('c1d04 a fragment without a known step is announced too', [h.model.doc.nodes.length, h.announced.length, Array.from(h.ed.selection)], [1, 2, [kept]]);
        h.store.set(KEY, JSON.stringify({ easydrag: 1, nodes: [{ id: 'a', key: 'k', type: 'web.search', position: { x: 0, y: 0 } }, { id: 'b', key: 'l', type: 'web.search', position: { x: 300, y: 0 } }],
            edges: [{ source: { node: 'a', port: 'out' }, target: { node: 'b', port: 'in' } }] }));
        await h.interact.pasteAt({ x: 100, y: 200 });
        const pasted = Array.from(h.ed.selection);
        eq('c1d04 a good fragment is pasted, drawn and selected without an announcement',
            [h.model.doc.nodes.length, pasted.length, h.model.incoming(pasted[1]).length, h.announced.length, !!h.card(pasted[0]), h.canvas.wiresSvg.querySelectorAll('.ed-edge').length],
            [3, 2, 1, 2, true, 1]);
        // readClipboard keeps the easydrag marker check: other JSON on the system clipboard is not pasted.
        h.store.delete(KEY);
        h.navigator.clipboard = { readText: async () => JSON.stringify({ nodes: [{ id: 'x', type: 'web.search' }], edges: [] }) };
        await h.interact.pasteAt({ x: 0, y: 0 });
        eq('c1d04 clipboard JSON without the easydrag marker is not pasted', [h.model.doc.nodes.length, h.announced.length], [3, 2]);
        h.navigator.clipboard = { readText: async () => { h.ed.readonly = true; return JSON.stringify({ easydrag: 1, nodes: [{ id: 'x', type: 'web.search' }], edges: [] }); } };
        await h.interact.pasteAt({ x: 0, y: 0 });
        eq('c1d04 a canvas that turned read-only while the clipboard was read pastes nothing', [h.model.doc.nodes.length, h.announced.length], [3, 2]);
        eq('c1d04 the paste checks log no errors', h.logged, []);
    });

    await guardAsync('c1d04 keyboard', async () => {
        // WCAG 2.1.2: Tab and Shift+Tab must be able to leave the canvas, and its buttons keep Space and Enter.
        const h = harness(twoNodes());
        const { interact, el, key } = h;
        h.runTimers(400); // draws the screen-reader node list
        const zoomIn = el.querySelector('[data-ed-zoom="in"]');
        const listButton = el.querySelector('[data-ed-focus-node]');
        interact.select([A]);
        const onButtons = [key('Tab', el, { shiftKey: true }), key('Tab', zoomIn), key('Tab', listButton), key(' ', zoomIn), key('Enter', zoomIn), key('Enter', listButton)].map(ev => interact.handleKey(ev));
        eq('c1d04 Shift+Tab, and Tab, Space and Enter on canvas buttons keep their default',
            [onButtons, h.quickAdds.length, h.details.length, el.classList.contains('is-space')], [[false, false, false, false, false, false], 0, 0, false]);
        h.ed.readonly = true;
        eq('c1d04 Tab and Shift+Tab on a read-only canvas move focus on', [interact.handleKey(key('Tab')), interact.handleKey(key('Tab', el, { shiftKey: true })), h.quickAdds.length], [false, false, 0]);
        h.ed.readonly = false;
        const afterStep = interact.handleKey(key('Tab'));
        interact.select([]);
        const inMiddle = interact.handleKey(key('Tab'));
        eq('c1d04 Tab on the canvas still opens quick-add, after the selected step or in the middle',
            [afterStep, inMiddle, h.quickAdds.map(q => q.from || { x: q.x, y: q.y })], [true, true, [{ node: A, port: 'out' }, { x: 400, y: 300 }]]);
        interact.select([A]);
        eq('c1d04 Space and Enter on the canvas keep their shortcuts', [interact.handleKey(key(' ')), el.classList.contains('is-space'), interact.handleKey(key('Enter')), h.details.length], [true, true, true, 1]);
        interact.handleKeyUp(key(' '));
        eq('c1d04 the keyboard checks log no errors', h.logged, []);
    });

    await guardAsync('c1d04 pinch', async () => {
        const touch = { pointerType: 'touch' };
        // The reviewer's sequence: the first finger starts a pan, the second one a pinch, then both move apart in turn.
        const h = harness(twoNodes());
        const grid = h.el.querySelector('.ed-grid');
        grid.fire('pointerdown', h.pe(11, 100, 100, touch));
        grid.fire('pointerdown', h.pe(12, 300, 100, touch));
        const zooms = [];
        for (let i = 1; i <= 4; i++) {
            h.el.fire('pointermove', h.pe(12, 300 + i * 40, 100, touch));
            zooms.push(Number(h.ed.view.zoom.toFixed(3)));
            h.el.fire('pointermove', h.pe(11, 100 - i * 40, 100, touch));
            zooms.push(Number(h.ed.view.zoom.toFixed(3)));
        }
        eq('c1d04 a pinch that starts as a touch pan zooms with every finger move', zooms, [1.2, 1.4, 1.6, 1.8, 2, 2, 2, 2]);
        h.el.fire('pointerup', h.pe(11, 0, 0, touch));
        h.el.fire('pointerup', h.pe(12, 0, 0, touch));
        // A pinch that starts as a node drag moves the card back and then zooms.
        const d = harness(twoNodes());
        const origin = d.pos(A);
        d.card(A).fire('pointerdown', d.pe(21, 10, 10, touch));
        d.el.fire('pointermove', d.pe(21, 70, 10, touch));
        const dragged = d.pos(A).x;
        d.el.querySelector('.ed-grid').fire('pointerdown', d.pe(22, 300, 300, touch));
        const afterSecond = [d.pos(A), d.ed.dragging, Array.from(d.timers.values()).some(x => x.ms === 550)];
        d.el.fire('pointermove', d.pe(22, 500, 300, touch));
        eq('c1d04 a second finger ends a node drag (the card moves back) and the pinch zooms',
            [dragged, afterSecond, d.pos(A), d.ed.view.zoom > 1], [60, [origin, false, false], origin, true]);
        // lostpointercapture frees the finger, so the next one pans instead of pinching with a ghost.
        const l = harness(twoNodes());
        const lgrid = l.el.querySelector('.ed-grid');
        lgrid.fire('pointerdown', l.pe(31, 100, 100, touch));
        l.el.fire('lostpointercapture', l.pe(31, 100, 100, touch));
        lgrid.fire('pointerdown', l.pe(32, 200, 200, touch));
        l.el.fire('pointermove', l.pe(32, 250, 200, touch));
        eq('c1d04 lost capture frees the pointer: the next finger pans', l.ed.view, { x: 50, y: 0, zoom: 1 });
        eq('c1d04 the pinch checks log no errors', [h.logged, d.logged, l.logged], [[], [], []]);
    });

    await guardAsync('c1d04 ended gestures', async () => {
        const h = harness(twoNodes());
        const origin = h.pos(B);
        const moveListeners = () => (h.el.listeners.pointermove || []).length;
        const idle = moveListeners();
        // A mouse move with no button down means the release was missed: the drag is cancelled.
        h.card(B).fire('pointerdown', h.pe(1, 600, 0));
        h.el.fire('pointermove', h.pe(1, 660, 0));
        const moved = h.pos(B).x;
        h.el.fire('pointermove', h.pe(1, 760, 0, { buttons: 0 }));
        eq('c1d04 a mouse move with no button down cancels a drag whose release was missed',
            [moved, h.pos(B), h.ed.dragging, moveListeners() === idle], [660, origin, false, true]);
        h.card(B).fire('pointerdown', h.pe(2, 600, 0));
        h.el.fire('pointermove', h.pe(2, 660, 0));
        h.el.fire('lostpointercapture', h.pe(2, 660, 0));
        h.el.fire('pointermove', h.pe(2, 700, 0));
        eq('c1d04 lost pointer capture cancels a drag', [h.pos(B), h.ed.dragging, moveListeners() === idle], [origin, false, true]);
        // The minimap follows its pointer through the same helper.
        const mini = h.el.querySelector('.ed-minimap svg');
        mini.getBoundingClientRect = () => ({ left: 0, top: 0, right: 200, bottom: 130, width: 200, height: 130 });
        const miniMoves = () => (mini.listeners.pointermove || []).length;
        mini.fire('pointerdown', h.pe(41, 50, 50, { pointerType: 'touch' }));
        mini.fire('pointercancel', h.pe(41, 50, 50, { pointerType: 'touch' }));
        const parked = JSON.stringify(h.ed.view);
        mini.fire('pointermove', h.pe(41, 150, 100, { pointerType: 'touch' }));
        eq('c1d04 a cancelled minimap drag removes its listeners and ignores later moves', [miniMoves(), JSON.stringify(h.ed.view) === parked], [0, true]);
        mini.setPointerCapture = () => { throw new Error('InvalidPointerId'); };
        mini.fire('pointerdown', h.pe(42, 50, 50));
        const pressed = JSON.stringify(h.ed.view);
        mini.fire('pointermove', h.pe(43, 150, 100));
        const otherPointer = JSON.stringify(h.ed.view) === pressed;
        mini.fire('pointermove', h.pe(42, 150, 100));
        const ownPointer = JSON.stringify(h.ed.view) !== pressed;
        mini.fire('lostpointercapture', h.pe(42, 150, 100));
        eq('c1d04 the minimap survives a failing setPointerCapture, follows only its pointer and stops on lost capture',
            [otherPointer, ownPointer, miniMoves()], [true, true, 0]);
        eq('c1d04 the gesture checks log no errors', h.logged, []);
    });

    await guardAsync('c1d04 wires', async () => {
        // An edge id that would break a CSS selector: the drop target is found through the wire groups.
        const odd = 'e"] .x';
        const h = harness(twoNodes(odd, { nodes: [{ id: C, key: 'gamma', type: 'web.search', label: 'Gamma', position: { x: 0, y: 400 }, params: {}, settings: {} }] }));
        const group = () => h.canvas.wiresSvg.querySelectorAll('.ed-edge').find(g => g.getAttribute('data-edge-id') === odd);
        h.card(C).fire('pointerdown', h.pe(1, 10, 410));
        h.el.fire('pointermove', h.pe(1, 310, 10));
        const marked = !!group() && group().classList.contains('is-drop-target');
        h.el.fire('pointerup', h.pe(1, 310, 10));
        eq('c1d04 dropping a card on a wire whose id is no valid selector marks the wire and inserts the card',
            [marked, h.model.incoming(C).map(e => e.source.node), h.model.incoming(B).map(e => e.source.node), h.canvas.wiresSvg.querySelectorAll('.ed-edge').filter(g => g.classList.contains('is-drop-target')).length],
            [true, [A], [C], 0]);
        // A cancelled pick-up of a wired input puts the wire back; released on empty canvas it stays off.
        const p = harness(twoNodes());
        const before = JSON.stringify(p.model.doc);
        const inPort = () => p.card(B).querySelector('.ed-port--in');
        inPort().fire('pointerdown', p.pe(51, 600, 36));
        const lifted = p.model.doc.edges.length;
        p.el.fire('pointermove', p.pe(51, 500, 100));
        p.el.fire('pointercancel', p.pe(51, 500, 100));
        eq('c1d04 a cancelled pick-up of a wired input puts the wire back', [lifted, JSON.stringify(p.model.doc) === before], [0, true]);
        inPort().fire('pointerdown', p.pe(52, 600, 36));
        p.el.fire('pointermove', p.pe(52, 900, 500));
        p.el.fire('pointerup', p.pe(52, 900, 500));
        eq('c1d04 a pick-up released on empty canvas keeps the wire off and offers quick-add from its source',
            [p.model.doc.edges.length, p.quickAdds.map(q => q.from)], [0, [{ node: A, port: 'out' }]]);
        // When the document changed during the pick-up, a cancel must not undo that change.
        const q = harness(twoNodes());
        q.card(B).querySelector('.ed-port--in').fire('pointerdown', q.pe(53, 600, 36));
        q.model.setLabel(A, 'Other');
        q.el.fire('pointercancel', q.pe(53, 600, 36));
        eq('c1d04 a cancelled pick-up after another change undoes nothing', [q.model.doc.edges.length, q.model.node(A).label], [0, 'Other']);
        eq('c1d04 the wire checks log no errors', [h.logged, p.logged, q.logged], [[], [], []]);
    });

    await guardAsync('c1d04 read-only canvas', async () => {
        const h = harness(twoNodes());
        h.ed.readonly = true;
        // A card drawn before the switch still has its tools: they must do nothing.
        h.card(A).querySelector('[data-ed-node-tool="delete"]').fire('click');
        h.card(A).querySelector('[data-ed-node-tool="test"]').fire('click');
        h.canvas.render(); // the editor redraws after switching the mode
        h.card(A).querySelector('[data-ed-port-add]').fire('pointerdown', h.pe(1, 232, 36));
        h.el.fire('pointerup', h.pe(1, 232, 36));
        eq('c1d04 a read-only canvas marks its cards and ignores card tools and port +',
            [h.card(A).classList.contains('is-readonly'), !!h.card(A).querySelector('[data-ed-node-tool]'), h.model.doc.nodes.length, h.quickAdds.length], [true, false, 2, 0]);
        eq('c1d04 the read-only checks log no errors', h.logged, []);
    });

    await guardAsync('c1d04 card summaries', async () => {
        const h = harness(twoNodes());
        const beta = () => h.card(B).html;
        const drawn = beta().includes('Alpha');
        h.model.setLabel(A, 'Gamma');
        const relabelled = [beta().includes('Gamma'), beta().includes('Alpha')];
        h.model.removeNodes([A]);
        eq('c1d04 a card summary follows the label of the step it references',
            [drawn, relabelled, beta().includes('Gamma'), beta().includes('alpha')], [true, [true, false], false, true]);
        eq('c1d04 the summary checks log no errors', h.logged, []);
    });

    await guardAsync('c1d04 announce and dispose', async () => {
        const h = harness(twoNodes());
        const live = h.el.querySelector('[data-ed-live]');
        h.canvas.announce('one');
        h.canvas.announce('two');
        const waiting = Array.from(h.timers.values()).filter(x => x.ms === 30).length;
        h.runTimers(30);
        eq('c1d04 announce keeps one timer and says the latest text', [waiting, live.textContent], [1, 'two']);
        // Leave work pending: an animated view change, an announcement, a new card, frames and a drag.
        h.canvas.setView({ x: 5, y: 5, zoom: 1 }, { animate: true });
        h.canvas.announce('three');
        h.model.addNode('web.search', { x: 0, y: 300 });
        const origin = h.pos(B);
        h.card(B).fire('pointerdown', h.pe(61, 600, 0));
        h.el.fire('pointermove', h.pe(61, 660, 0));
        const busy = [h.timers.size > 0, h.frames.size > 0, h.subscriptions.size];
        const elements = [];
        const walk = node => { elements.push(node); node.children.forEach(walk); };
        walk(h.el);
        h.interact.dispose();
        h.wires.dispose();
        h.canvas.dispose();
        const listening = elements.filter(node => Object.values(node.listeners).some(list => list.length)).map(node => node.className || node.localName);
        eq('c1d04 dispose cancels a drag and leaves no listeners, timeouts, frames or bus subscriptions',
            [busy, h.pos(B), listening, h.timers.size, h.frames.size, h.subscriptions.size, h.observers], [[true, true, 8], origin, [], 0, 0, 0, { observed: 1, disconnected: 1 }]);
        eq('c1d04 the dispose checks log no errors', h.logged, []);
    });

    // ── performance: work that must not happen on selection, drag and pan frames ──

    await guardAsync('c1d04 selection cost', async () => {
        const h = harness(twoNodes());
        const rebuilt = h.counts.cards;
        h.interact.handleKey(h.key('a', h.el, { ctrlKey: true }));
        const marked = [h.card(A).classList.contains('is-selected'), h.card(B).classList.contains('is-selected')];
        h.interact.handleKey(h.key('Escape'));
        eq('c1d04 Ctrl+A and Escape rebuild no card markup', [h.counts.cards - rebuilt, marked, h.card(A).classList.contains('is-selected')], [0, [true, true], false]);
        const wire = h.canvas.wiresSvg.querySelector('.ed-edge');
        wire.firstChild.fire('click');
        const picked = [wire.classList.contains('is-selected'), h.ed.selectedEdge];
        h.interact.select([A]);
        eq('c1d04 a wire click marks the wire and selecting a step unmarks it', [picked, wire.classList.contains('is-selected'), h.ed.selectedEdge], [[true, 'e1'], false, null]);
        eq('c1d04 the selection checks log no errors', h.logged, []);
    });

    await guardAsync('c1d04 run pills', async () => {
        const h = harness(twoNodes());
        h.ed.run = { steps: new Map(), edges: new Map([['e1', { state: 'done', count: 3 }]]) };
        h.bus.emit('run');
        const layer = h.canvas.wiresSvg.querySelector('.ed-wires-pills');
        const pill = layer.children[0];
        const placed = pill && pill.getAttribute('transform');
        h.card(A).fire('pointerdown', h.pe(1, 10, 10));
        h.el.fire('pointermove', h.pe(1, 10, 110));
        h.el.fire('pointermove', h.pe(1, 10, 160));
        const during = [layer.children.length, layer.children[0] === pill, pill.getAttribute('transform') !== placed];
        h.el.fire('pointerup', h.pe(1, 10, 160));
        h.bus.emit('run');
        eq('c1d04 drag frames only move the run pills; a run event rebuilds them', [during, layer.children[0] !== pill, layer.children.length], [[1, true, true], true, 1]);
        eq('c1d04 the pill checks log no errors', h.logged, []);
    });

    await guardAsync('c1d04 frame work', async () => {
        const h = harness(twoNodes());
        const G = h.ED.geometry;
        // highlight: hovering fires it for every element of a card; the same node redraws nothing.
        let paths = 0;
        const wirePath = G.wirePath;
        G.wirePath = (a, b) => { paths++; return wirePath(a, b); };
        h.wires.highlight(A);
        const first = paths;
        h.wires.highlight(A);
        const again = paths - first;
        h.model.setLabel(B, 'Changed');
        const edited = paths;
        h.wires.highlight(A);
        G.wirePath = wirePath;
        eq('c1d04 highlighting the same node again redraws no wire until the document changes', [first, again, paths - edited, h.el.classList.contains('has-path')], [1, 0, 1, true]);
        // box select: the selection changes only when the hit set does.
        const grid = h.el.querySelector('.ed-grid');
        const s0 = h.selections();
        grid.fire('pointerdown', h.pe(2, -50, -50));
        h.el.fire('pointermove', h.pe(2, 100, 100));
        h.el.fire('pointermove', h.pe(2, 120, 100));
        h.el.fire('pointermove', h.pe(2, 140, 110));
        const one = [h.selections() - s0, Array.from(h.ed.selection)];
        h.el.fire('pointermove', h.pe(2, 700, 100));
        h.el.fire('pointerup', h.pe(2, 700, 100));
        eq('c1d04 box select emits a selection only when the hit set changes', [one, h.selections() - s0, Array.from(h.ed.selection)], [[1, [A]], 2, [A, B]]);
        // minimap: the node layer sits in world coordinates; a pan or zoom only moves it.
        h.flushFrames();
        const nodes = h.el.querySelector('.ed-minimap-nodes');
        const view = h.el.querySelector('.ed-minimap-view');
        const drawn = h.counts.minimapNodes;
        const x0 = view.getAttribute('x');
        h.canvas.setView({ x: -300, y: 40, zoom: 1 });
        h.flushFrames();
        h.canvas.setView({ x: 200, y: -90, zoom: 0.5 });
        h.flushFrames();
        const panned = [h.counts.minimapNodes - drawn, view.getAttribute('x') !== x0, /^matrix\(/.test(nodes.getAttribute('transform') || ''), nodes.children[1].getAttribute('x')];
        h.interact.select([A]);
        h.flushFrames();
        eq('c1d04 a pan or zoom moves the minimap layer without redrawing it; a selection redraws it once',
            [panned, h.counts.minimapNodes - drawn, nodes.html.includes('is-selected')], [[0, true, true, '600'], 1, true]);
        // A click on the minimap still centres the view on the point under it.
        const mini = h.el.querySelector('.ed-minimap svg');
        mini.getBoundingClientRect = () => ({ left: 0, top: 0, right: 200, bottom: 130, width: 200, height: 130 });
        const scale = Number(mini.dataset.scale);
        mini.fire('pointerdown', h.pe(3, 716 * scale + Number(mini.dataset.ox), 36 * scale + Number(mini.dataset.oy)));
        mini.fire('pointerup', h.pe(3, 0, 0));
        check('c1d04 a minimap click centres the view on the step under it', Math.abs(h.ed.view.x - (400 - 716 * h.ed.view.zoom)) < 0.01 && Math.abs(h.ed.view.y - (300 - 36 * h.ed.view.zoom)) < 0.01, JSON.stringify(h.ed.view));
        // edgeNear skips wires by their control-point box: the curve never leaves it.
        const inside = [[{ x: 0, y: 0 }, { x: 300, y: 100 }], [{ x: 500, y: 0 }, { x: 100, y: 200 }], [{ x: 0, y: 0 }, { x: 10, y: -50 }]].every(([a, b]) => {
            const box = G.wireBounds(a, b);
            return Array.from({ length: 41 }, (_, i) => G.bezierPoint(a, b, i / 40)).every(p => p.x >= box.x - 1e-9 && p.x <= box.x + box.w + 1e-9 && p.y >= box.y - 1e-9 && p.y <= box.y + box.h + 1e-9);
        });
        check('c1d04 wireBounds holds the whole wire, backwards wires included', inside);
        // The node list is built in one pass over nodes and edges.
        h.runTimers(400);
        check('c1d04 the node list names where each step leads', h.el.querySelector('.ed-node-list').html.includes('Alpha ' + String.fromCharCode(0x2192) + ' Changed'));
        eq('c1d04 the frame checks log no errors', h.logged, []);
    });

    await guardAsync('c1d04 model swap', async () => {
        // Entering a run view swaps in a new model that starts at version 0 like the untouched draft:
        // caches keyed on the version alone would keep showing the draft.
        const h = harness(twoNodes('e1', { nodes: [{ id: C, key: 'gamma', type: 'web.search', label: 'Gamma', position: { x: 0, y: 300 }, params: {}, settings: {} }] }));
        const nodes = h.el.querySelector('.ed-minimap-nodes');
        const G = h.ED.geometry;
        h.wires.highlight(A);
        const draft = [h.model.version, nodes.children.length, h.card(B).html.includes('Alpha')];
        const run = h.ED.model.create({ schema: 1, name: 'Run', nodes: [
            { id: A, key: 'alpha', type: 'web.search', label: 'Renamed in run', position: { x: 0, y: 0 }, params: {}, settings: {} },
            { id: B, key: 'beta', type: 'web.search', label: 'Beta', position: { x: 600, y: 0 }, params: { query: '{{alpha.results}}' }, settings: {} }
        ], edges: [{ id: 'e1', source: { node: A, port: 'out' }, target: { node: B, port: 'in' } }] }, { types: h.ed.catalog.types });
        h.ed.model = run;
        h.ed.selection = new Set();
        h.ed.runView = { run: {}, doc: {} };
        h.bus.emit('model', { kind: 'reset', nodes: [], edges: [], meta: true, structural: true });
        h.flushFrames();
        let paths = 0;
        const wirePath = G.wirePath;
        G.wirePath = (a, b) => { paths++; return wirePath(a, b); };
        h.wires.highlight(A);
        G.wirePath = wirePath;
        eq('c1d04 a swapped-in model at the same version redraws the minimap, summaries and path',
            [draft, run.version, nodes.children.length, h.card(B).html.includes('Renamed in run'), h.card(B).html.includes('Alpha'), paths], [[0, 3, true], 0, 2, true, false, 1]);
        eq('c1d04 the swap checks log no errors', h.logged, []);
    });

    // ── c1d05: palette, fields, forms and mapping ──

    // paletteHarness adds a palette panel to the canvas harness: a catalog list with categories, the
    // editor root holding canvas and panel, and elementFromPoint returning h.at.
    function paletteHarness() {
        const h = harness(twoNodes());
        // The palette reads its search box at render: inputs get a value (the attribute until set).
        Object.defineProperty(h.dom.El.prototype, 'value', {
            configurable: true,
            get() { return this.typed !== undefined ? this.typed : (this.attrs.get('value') || ''); },
            set(v) { this.typed = String(v); }
        });
        const list =Array.from(canvasTypes.values()).map(info => Object.assign({ category: info.trigger ? 'trigger' : 'web', availability: { state: 'available' } }, info));
        Object.assign(h.ed.catalog, { list, categories: [{ id: 'trigger', label: 'Trigger' }, { id: 'web', label: 'Web' }] });
        h.ed.root = h.ED.core.el('<div class="ed-editor"></div>');
        h.ed.root.appendChild(h.el);
        h.ed.windowId = 'w1';
        h.at = null;
        h.dom.document.elementFromPoint = () => h.at;
        h.panel = h.ED.palette.createPanel(h.ed, h.canvas);
        h.ed.root.appendChild(h.panel.el);
        h.item = type => h.panel.el.querySelectorAll('[data-ed-type]').find(n => n.getAttribute('data-ed-type') === type);
        return h;
    }

    // formHarness renders the parameter form of one node of info. opts: params (the node's), answers
    // (functions answering the options requests in turn), readonly. Changes land in h.changes, options
    // requests in h.calls; h.field(name) is the .ed-field of a param.
    function formHarness(info, opts) {
        const o = opts || {};
        const h = harness();
        Object.defineProperty(h.dom.El.prototype, 'tagName', { configurable: true, get() { return String(this.localName).toUpperCase(); } });
        const answers = (o.answers || []).slice();
        h.changes = [];
        h.calls = [];
        h.ed.api = { options: (type, param) => { h.calls.push(type + '|' + param); const next = answers.shift(); return next ? next() : Promise.resolve({ options: [] }); } };
        h.ed.root = h.ED.core.el('<div class="ed-editor"></div>');
        h.ed.windowId = 'w1';
        h.ed.readonly = !!o.readonly;
        h.node = { id: A, key: 'alpha', type: info.type, label: 'Alpha', position: { x: 0, y: 0 }, params: o.params || {}, settings: {} };
        h.form = h.ED.forms.render({ ed: h.ed, node: h.node, info, upstream: [], roots: {}, issues: [], onChange: (name, value) => h.changes.push([name, value]) });
        h.field = name => h.form.el.querySelectorAll('.ed-field').find(f => f.getAttribute('data-param') === name);
        return h;
    }

    await guardAsync('c1d05 palette drag', async () => {
        const h = paletteHarness();
        const G = h.ED.geometry;
        const nodes = () => h.model.doc.nodes.length;
        const ghost = () => !!h.ed.root.querySelector('.ed-drag-ghost');
        const listening = item => ['pointermove', 'pointerup', 'pointercancel', 'lostpointercapture'].reduce((n, type) => n + (item.listeners[type] || []).length, 0);
        const item = h.item('web.search');
        // A touch the browser turns into a scroll of the palette is cancelled: a later release adds nothing.
        item.fire('pointerdown', h.pe(1, 10, 10, { pointerType: 'touch' }));
        item.fire('pointercancel', h.pe(1, 10, 10, { pointerType: 'touch' }));
        item.fire('pointerup', h.pe(1, 10, 10, { pointerType: 'touch' }));
        const cancelled = [nodes(), listening(item)];
        // A release missed outside the window: the next mouse move without a button ends the drag.
        item.fire('pointerdown', h.pe(2, 10, 10));
        item.fire('pointermove', h.pe(2, 200, 200));
        const dragging = [ghost(), h.ed.dragging];
        item.fire('pointermove', h.pe(2, 220, 220, { buttons: 0 }));
        item.fire('pointerup', h.pe(2, 220, 220));
        eq('c1d05 a cancelled or missed palette drag adds nothing and leaves no ghost or listeners',
            [cancelled, dragging, nodes(), ghost(), h.ed.dragging, listening(item)], [[2, 0], [true, true], 2, false, false, 0]);
        // A click still adds the step; a drop on the canvas places it under the pointer.
        item.fire('pointerdown', h.pe(3, 10, 10));
        item.fire('pointerup', h.pe(3, 10, 10));
        const clicked = [nodes(), h.ed.selection.size];
        h.at = h.el;
        item.fire('pointerdown', h.pe(4, 10, 10));
        item.fire('pointermove', h.pe(4, 300, 200));
        item.fire('pointerup', h.pe(4, 300, 200));
        const dropped = h.model.node(Array.from(h.ed.selection)[0]);
        eq('c1d05 a palette click adds a step and a drop on the canvas places one there',
            [clicked, nodes(), dropped && dropped.position, ghost()], [[3, 1], 4, { x: G.snap(300 - G.NODE_W / 2), y: G.snap(200 - G.NODE_H / 2) }, false]);
        // Closing the editor during a drag ends it.
        item.fire('pointerdown', h.pe(5, 10, 10));
        item.fire('pointermove', h.pe(5, 200, 200));
        const before = [ghost(), h.ed.dragging];
        h.panel.dispose();
        eq('c1d05 dispose ends a palette drag', [before, ghost(), h.ed.dragging, listening(item), nodes()], [[true, true], false, false, 0, 4]);
        eq('c1d05 the palette checks log no errors', h.logged, []);
    });

    await guardAsync('c1d05 sample lookups', async () => {
        const h = harness();
        const F = h.ED.fields;
        const roots = { x: { a: 1 }, list: [{ p: 1 }, { q: 2 }] };
        // fields.sampleAt walks paths through template.resolvePath, like the engine's stepInto.
        eq('c1d05 sample lookups read own fields only and count negative indexes from the end',
            ['x.constructor', 'constructor', 'x.__proto__', 'list[-1]', 'list[-3]', 'x.a', 'x..'].map(p => F.sampleAt({ roots }, p)),
            [null, null, null, { q: 2 }, null, 1, null]);
        const env = {
            t, esc: h.ED.core.esc, readonly: false, roots, change() {},
            upstream: [{ key: 'list', label: 'List', cat: 'web', fields: [{ name: 'items', type: 'list' }] }, { key: 'x', label: 'X', cat: 'web', fields: ['a', 'b'] }]
        };
        const field = F.templateField(env, '', {});
        const input = field.el.querySelector('.ed-tpl-input');
        const suggest = text => {
            input.value = text;
            input.selectionStart = text.length;
            input.fire('input');
            const list = field.el.querySelector('.ed-suggest');
            return list ? list.items.map(it => it.text) : [];
        };
        field.focus();
        eq('c1d05 autocomplete offers the fields of the sample at a path and output fields given as names or objects',
            [suggest('{{list[-1].'), suggest('{{list.'), suggest('{{x.'), suggest('{{x.constructor.')], [['list[-1].q'], ['list.items'], ['x.a', 'x.b'], []]);
        eq('c1d05 the sample checks log no errors', h.logged, []);
    });

    await guardAsync('c1d05 mapping references', async () => {
        const h = harness();
        const { pathJoin } = h.ED.mapping;
        const keys = ['plain', 'two words', 'a"b', 'back\\slash', 'line\nbreak', 'tab\there', '0', '$type', '}}', 'gr' + String.fromCharCode(0xfc) + 'n'];
        const value = {};
        keys.forEach((k, i) => { value[k] = i; });
        // A reference from the input tree must read back its own field through the shared parser and resolver.
        eq('c1d05 every input tree reference reads back its own field',
            keys.map(k => h.ED.template.evaluate('{{' + pathJoin('x', k) + '}}', { x: value })), keys.map((k, i) => i));
        eq('c1d05 indexes and plain names stay short', [pathJoin('x', 2), pathJoin('x', 'name'), pathJoin('x', 'a"b'), pathJoin('x', 'a\\b')], ['x[2]', 'x.name', 'x["a\\"b"]', 'x["a\\\\b"]']);
        eq('c1d05 the mapping checks log no errors', h.logged, []);
    });

    await guardAsync('c1d05 option lists', async () => {
        const failing = () => Promise.reject(Object.assign(new Error('down'), { body: { error: 'down', code: 'FLOW_OPTIONS_UNAVAILABLE' } }));
        const missions = () => Promise.resolve({ options: [{ value: 'm1', label: 'One' }, { value: 'm2', label: 'Two' }] });
        const info = { type: 'mission.run', params: [{ name: 'mission', kind: 'select', label: 'Mission', options_source: 'missions', required: true }] };
        const h = formHarness(info, { params: { mission: 'm2' }, answers: [failing, missions] });
        const select = () => h.field('mission').querySelector('select');
        const values = () => select().querySelectorAll('option').map(n => n.getAttribute('value') + (n.attrs.has('selected') ? '*' : ''));
        await settle();
        const failed = [select().getAttribute('data-error'), values()];
        h.form.refresh(h.node);
        await settle();
        const loaded = values();
        h.form.refresh(h.node);
        await settle();
        eq('c1d05 a failed options request is not cached: the next build asks again and reuses the answer',
            [failed, loaded, values(), h.calls], [['error_flow_options_unavailable', ['m2*']], ['m1', 'm2*'], ['m1', 'm2*'], ['mission.run|mission', 'mission.run|mission']]);
        // Home Assistant entities are cut at 2000; the combo box says so, also when it is rebuilt from the cache.
        const haInfo = { type: 'home.state', params: [{ name: 'entity', kind: 'select', label: 'Entity', options_source: 'ha_entities' }] };
        const cut = formHarness(haInfo, { answers: [() => Promise.resolve({ options: [{ value: 'light.a', label: 'A', hint: 'on' }], truncated: true })] });
        const whole = formHarness(haInfo, { answers: [() => Promise.resolve({ options: [{ value: 'light.a', label: 'A' }] })] });
        await settle();
        const hints = f => f.field('entity').querySelectorAll('.ed-combo .ed-options-truncated').length;
        const shown = [hints(cut), cut.field('entity').querySelectorAll('datalist option').length, hints(whole)];
        cut.form.refresh(cut.node);
        eq('c1d05 a cut Home Assistant list shows one hint under the combo box, a whole list none',
            [shown, hints(cut), cut.calls.length], [[1, 1, 0], 1, 1]);
        eq('c1d05 the option checks log no errors', [h.logged, cut.logged, whole.logged], [[], [], []]);
    });

    await guardAsync('c1d05 plain-text hint', async () => {
        const hinted = f => f.form.el.querySelectorAll('.ed-field').filter(n => n.querySelector('.ed-field-plaintext')).map(n => n.getAttribute('data-param'));
        const ldap = formHarness({ type: 'tool.ldap', params: [{ name: 'base_dn', kind: 'text', label: 'Base DN', templatable: true },
            { name: 'changes', kind: 'json', label: 'Changes', templatable: true }, { name: 'entry_attributes', kind: 'json', label: 'Entry attributes', templatable: true }] });
        const hooks = formHarness({ type: 'tool.manage_outgoing_webhooks', params: [{ name: 'headers', kind: 'keyvalue', label: 'Headers', templatable: true },
            { name: 'parameters', kind: 'json', label: 'Parameters', templatable: true }] });
        const other = formHarness({ type: 'tool.other', params: [{ name: 'headers', kind: 'keyvalue', label: 'Headers', templatable: true }] });
        eq('c1d05 generic tool params saved as plain text carry a hint, others none', [hinted(ldap), hinted(hooks), hinted(other)], [['changes', 'entry_attributes'], ['headers'], []]);
        eq('c1d05 the hint checks log no errors', [ldap.logged, hooks.logged, other.logged], [[], [], []]);
    });

    await guardAsync('c1d05 drops', async () => {
        const blank = () => ({ left: '', op: 'eq', right: '', type: 'auto' });
        const info = { type: 'logic.if', params: [{ name: 'condition', kind: 'condition_group', label: 'Condition' }] };
        const transfer = ref => ({ types: ['application/x-easydrag-ref'], getData: type => (type === 'application/x-easydrag-ref' ? ref : '') });
        const h = formHarness(info, { params: { condition: { match: 'all', rows: [blank(), blank()] } } });
        const lefts = () => h.form.el.querySelectorAll('.ed-cond-left .ed-tpl');
        lefts()[1].fire('drop', { dataTransfer: transfer('alpha.results') });
        // A drop on the param outside its template fields still goes into the first one.
        h.field('condition').fire('drop', { dataTransfer: transfer('alpha.count') });
        eq('c1d05 a drop goes into the template field under the pointer',
            h.changes.map(([name, value]) => [name, value.rows.map(r => r.left)]), [['condition', ['', '{{alpha.results}}']], ['condition', ['{{alpha.count}}', '{{alpha.results}}']]]);
        const ro = formHarness(info, { params: { condition: { match: 'all', rows: [blank(), blank()] } }, readonly: true });
        ro.form.el.querySelectorAll('.ed-cond-left .ed-tpl')[1].fire('drop', { dataTransfer: transfer('alpha.results') });
        ro.form.insert('condition', 'alpha.results');
        eq('c1d05 a read-only form takes no drop or insert', ro.changes, []);
        eq('c1d05 the drop checks log no errors', [h.logged, ro.logged], [[], []]);
    });

    // ── c1d05 review: template field and secrets ──

    // tplHarness is one template field. opts: field (its options), roots, upstream, readonly;
    // saved values land in h.changes. h.type sets the text with the caret at its end.
    function tplHarness(opts, value) {
        const o = opts || {};
        const h = harness();
        h.changes = [];
        const env = { t, esc: h.ED.core.esc, readonly: !!o.readonly, change: v => h.changes.push(v), roots: o.roots || {}, upstream: o.upstream || [] };
        h.field = h.ED.fields.templateField(env, value || '', o.field || {});
        h.view = h.field.el.querySelector('.ed-tpl-view');
        h.input = h.field.el.querySelector('.ed-tpl-input');
        h.list = () => h.field.el.querySelector('.ed-suggest');
        h.offered = () => (h.list() ? h.list().items.map(i => i.text) : null);
        h.type = text => { h.input.value = text; h.input.selectionStart = h.input.selectionEnd = text.length; h.input.fire('input'); };
        h.press = (key, extra) => h.input.fire('keydown', Object.assign({ key }, extra));
        h.editing = () => h.field.el.classList.contains('is-editing');
        h.onView = () => h.dom.document.activeElement === h.view;
        return h;
    }
    const sampleRoots = { alpha: { results: [1], meta: { n: 1 } }, http: { headers: { 'content-type': 'text/html' } } };
    const sampleUpstream = [{ key: 'alpha', label: 'Alpha', cat: 'web', fields: ['results'] }, { key: 'http', label: 'HTTP', cat: 'web', fields: ['headers'] }];

    await guardAsync('c1d05 review autocomplete', async () => {
        const h = tplHarness({ roots: sampleRoots, upstream: sampleUpstream });
        h.field.focus();
        h.type('{{alp');
        const offered = h.offered();
        h.press('Enter');
        const step = [h.input.value, h.input.selectionStart, h.offered()];
        h.press('ArrowDown');
        h.press('Enter');
        const object = [h.input.value, h.offered()];
        h.press('Enter');
        const leaf = [h.input.value, h.offered(), h.editing()];
        const tab = h.press('Tab').defaultPrevented;
        h.press('Enter');
        eq('c1d05 autocomplete goes on from a step or object to its fields and closes after a value; then Tab leaves and Enter saves',
            [offered, step, object, leaf, tab, h.changes, h.editing(), h.onView()],
            [['alpha'], ['{{alpha.}}', 8, ['alpha.results', 'alpha.meta']], ['{{alpha.meta.}}', ['alpha.meta.n']], ['{{alpha.meta.n}}', null, true], false, ['{{alpha.meta.n}}'], false, true]);
        h.field.focus();
        h.type('{{alpha');
        const same = h.offered();
        h.type('{{alpha | up');
        const filter = h.offered();
        h.press('Enter');
        eq('c1d05 autocomplete offers nothing that repeats the text and closes after a filter', [same, filter, h.input.value, h.offered()], [null, ['alpha | upper'], '{{alpha | upper}}', null]);
        eq('c1d05 the autocomplete checks log no errors', h.logged, []);
    });

    await guardAsync('c1d05 review template field accessibility', async () => {
        const h = tplHarness({ roots: sampleRoots, upstream: sampleUpstream, field: { labelledBy: 'lbl-1', id: 'f-1' } });
        const named = [h.view.getAttribute('aria-labelledby'), h.input.getAttribute('aria-labelledby'), h.view.getAttribute('aria-readonly'), h.input.getAttribute('role'), h.input.getAttribute('aria-expanded')];
        const ro = tplHarness({ readonly: true, field: { label: 'Query' } });
        h.field.focus();
        h.type('{{alpha.');
        const listId = h.list().getAttribute('id');
        const options = h.list().querySelectorAll('li');
        const open = [h.input.getAttribute('aria-expanded'), h.input.getAttribute('aria-controls') === listId, h.input.getAttribute('aria-activedescendant') === options[0].getAttribute('id'),
            options.map(li => li.getAttribute('role') + ':' + li.getAttribute('aria-selected'))];
        h.press('ArrowDown');
        const moved = [h.input.getAttribute('aria-activedescendant') === options[1].getAttribute('id'), options.map(li => li.getAttribute('aria-selected'))];
        h.press('Escape');
        const closed = [h.input.getAttribute('aria-expanded'), h.input.getAttribute('aria-activedescendant'), h.input.getAttribute('aria-controls'), h.editing()];
        h.type('changed');
        h.press('Escape');
        eq('c1d05 a template field is named, read-only only when it is, and its suggestion list is a listbox the input points at',
            [named, [ro.view.getAttribute('aria-label'), ro.view.getAttribute('aria-readonly')], open, moved, closed],
            [['lbl-1', 'lbl-1', null, 'combobox', 'false'], ['Query', 'true'], ['true', true, true, ['option:true', 'option:false']], [true, ['false', 'true']], ['false', null, null, true]]);
        eq('c1d05 Escape in a single line undoes the edit and puts focus on the chips', [h.editing(), h.changes, h.onView(), h.input.value], [false, [], true, '']);
        // M6: Escape in a multi-line text saves it like leaving the field; Enter stays a new line.
        const m = tplHarness({ field: { multiline: true, label: 'Text' } }, 'old');
        m.field.focus();
        m.type('new text');
        const newline = m.press('Enter').defaultPrevented;
        m.press('Escape');
        eq('c1d05 Escape in a multi-line field saves the text and puts focus on the chips',
            [newline, m.changes, m.editing(), m.onView(), m.input.getAttribute('role'), m.view.getAttribute('aria-multiline')], [false, ['new text'], false, true, null, 'true']);
        eq('c1d05 the accessibility checks log no errors', [h.logged, ro.logged, m.logged], [[], [], []]);
    });

    await guardAsync('c1d05 review references in autocomplete', async () => {
        const h = tplHarness({ roots: sampleRoots, upstream: sampleUpstream });
        h.field.focus();
        h.type('{{http.headers.');
        const offered = h.offered();
        h.press('Enter');
        eq('c1d05 autocomplete quotes a field name that is no identifier, so the reference parses and resolves',
            [offered, h.ED.template.parseExpr(offered[0]).path, h.ED.template.evaluate(h.input.value, sampleRoots), typeof h.ED.template.pathJoin],
            [['http.headers["content-type"]'], ['headers', 'content-type'], 'text/html', 'function']);
        eq('c1d05 the reference checks log no errors', h.logged, []);
    });

    await guardAsync('c1d05 review IME composition', async () => {
        const h = tplHarness();
        h.field.focus();
        h.type('abc');
        const composing = [h.press('Enter', { isComposing: true }).defaultPrevented, h.press('Enter', { keyCode: 229 }).defaultPrevented, h.press('Escape', { isComposing: true }).defaultPrevented];
        const kept = [h.editing(), h.changes];
        const added = [];
        const tags = h.ED.fields.tags({ t, esc: h.ED.core.esc, readonly: false }, ['a'], v => added.push(v));
        const tagInput = tags.querySelector('.ed-tags-input');
        tagInput.value = 'b';
        tagInput.fire('keydown', { key: 'Enter', isComposing: true });
        const whileComposing = added.length;
        tagInput.fire('keydown', { key: 'Enter' });
        eq('c1d05 keys of an IME composition neither save a template field nor add a tag',
            [composing, kept, whileComposing, added], [[false, false, false], [true, []], 0, [['a', 'b']]]);
        eq('c1d05 the IME checks log no errors', h.logged, []);
    });

    await guardAsync('c1d05 review secrets', async () => {
        const h = harness();
        const F = h.ED.fields;
        const host = h.ED.core.el('<div class="ed-editor"></div>');
        const saved = [];
        let lists = 0;
        let listAnswer = () => Promise.resolve({ secrets: ['api_key'] });
        let saveAnswer = () => Promise.resolve({ status: 'saved' });
        const api = { secrets: () => { lists++; return listAnswer(); }, saveSecret: (name, value) => { saved.push([name, value]); return saveAnswer(); } };
        const picked = [];
        const env = { t, esc: h.ED.core.esc, readonly: false, root: host, api, secretCache: {} };
        const picker = F.secretRef(env, 'api_key', v => picked.push(v), 'sec-a');
        F.secretRef(env, undefined, v => picked.push(v), 'sec-b');
        await settle();
        const shared = [lists, picker.querySelectorAll('option').map(o => o.getAttribute('value'))];
        const open = () => host.querySelectorAll('.ed-modal-backdrop').filter(o => !o.classList.contains('is-closing'));
        const top = () => open()[open().length - 1];
        const fill = (dialog, name, value) => { dialog.querySelector('[data-ed-secret-name]').value = name; dialog.querySelector('[data-ed-secret-value]').value = value; };
        const act = async (id, dialog) => { (dialog || top()).querySelector('[data-ed-action="' + id + '"]').fire('click'); await settle(); };
        const error = dialog => dialog.querySelector('.ed-error').textContent;
        // 7: a taken name is saved only after the replace confirmation.
        picker.querySelector('[data-ed-secret-new]').fire('click');
        const dialog = top();
        fill(dialog, 'api_key', 'v2');
        await act('save', dialog);
        const taken = [error(dialog), open().length, saved.length];
        await act('keep');
        const kept = [open().length, top() === dialog, saved.length];
        await act('save', dialog);
        await act('replace');
        eq('c1d05 a new secret with a taken name asks before it replaces the value; pickers of a form share one list request',
            [shared, taken, kept, saved, picked, open().length, lists], [[1, ['', 'api_key']], ['secret_name_taken:api_key', 2, 0], [1, true, 0], [['api_key', 'v2']], ['api_key'], 0, 2]);
        // 8: the server's checks run before sending; a 429 holds Save for Retry-After seconds.
        picker.querySelector('[data-ed-secret-new]').fire('click');
        const second = top();
        const e = String.fromCharCode(0xe9); // 2 bytes in UTF-8
        const tries = [];
        for (const [name, value] of [['Bad Name', 'x'], ['fresh', '  \n '], ['fresh', 'x'.repeat(4097)], ['fresh', e.repeat(2049)]]) {
            fill(second, name, value);
            await act('save', second);
            tries.push(error(second));
        }
        const sentBefore = saved.length;
        saveAnswer = () => Promise.reject(Object.assign(new Error('slow'), { status: 429, retryAfter: 60, body: { error: 'slow', code: 'FLOW_RATE_LIMITED' } }));
        fill(second, 'fresh', e.repeat(2048));
        await act('save', second);
        const save = second.querySelector('[data-ed-action="save"]');
        const limited = [saved.length - sentBefore, error(second), save.disabled];
        h.runTimers(0);
        const held = save.disabled;
        h.runTimers(60000);
        eq('c1d05 the secret dialog checks the name, a blank value and 4096 bytes before sending and holds Save after a 429',
            [tries, limited, held, save.disabled], [['secret_invalid', 'secret_empty', 'secret_too_large', 'secret_too_large'], [1, 'error_flow_rate_limited secret_retry:60', false], true, false]);
        // 8: a secret list that cannot be loaded shows a hint; the next picker asks again.
        listAnswer = () => Promise.reject(Object.assign(new Error('vault'), { body: { error: 'vault', code: 'FLOWS_DISABLED' } }));
        const failingEnv = { t, esc: h.ED.core.esc, readonly: false, root: host, api, secretCache: {} };
        const failing = F.secretRef(failingEnv, 'old', () => {}, 'sec-c');
        await settle();
        const failed = [failing.querySelector('.ed-secret-unavailable').hidden, failing.querySelector('select').getAttribute('data-error'), failing.querySelectorAll('option').map(o => o.getAttribute('value'))];
        failing.querySelector('[data-ed-secret-new]').fire('click');
        const third = top();
        fill(third, 'brand_new', 'value');
        const sentNow = saved.length;
        await act('save', third);
        const blocked = [error(third), saved.length - sentNow];
        await act('cancel', third);
        listAnswer = () => Promise.resolve({ secrets: ['old'] });
        const listsBefore = lists;
        const again = F.secretRef(failingEnv, 'old', () => {}, 'sec-d');
        await settle();
        eq('c1d05 a secret list that cannot be loaded shows a hint, blocks a new secret and is asked for again',
            [failed, blocked, again.querySelector('.ed-secret-unavailable').hidden, lists - listsBefore], [[false, 'error_flows_disabled', ['', 'old']], ['secrets_unavailable', 0], true, 1]);
        eq('c1d05 the secret checks log no errors', h.logged, []);
    });

    await guardAsync('c1d05 review malformed params and labels', async () => {
        const h = harness();
        const F = h.ED.fields;
        const env = { t, esc: h.ED.core.esc, readonly: false, roots: {}, change() {}, upstream: [{ key: 'alpha', label: 'Alpha', cat: 'x" onclick="evil', fields: [] }] };
        const cond = F.conditionGroup(env, { match: 'any', rows: 'oops' }, () => {});
        const cases = F.cases(env, [null, 5, { label: 'x' }, ['y']], () => {});
        const list = F.fieldList(env, {}, { not: 'a list' }, () => {});
        const kv = F.keyValue(env, ['a', 'b'], () => {});
        const tags = F.tags(env, [{}, 'a', null, 3], () => {});
        eq('c1d05 malformed rows, cases, field lists, pairs and tags become valid lists',
            [cond.querySelectorAll('.ed-cond-row').length, cases.querySelectorAll('.ed-case').length, list.querySelectorAll('.ed-fieldlist-row').length, kv.querySelectorAll('.ed-kv-row').length, tags.querySelectorAll('.ed-tag').length],
            [1, 1, 0, 0, 2]);
        const row = cond.querySelector('.ed-cond-row');
        const pair = F.keyValue(env, { k: 'v' }, () => {}).querySelector('.ed-kv-row');
        eq('c1d05 every condition, case, pair and tag control has a name; remove buttons name their row',
            [row.querySelector('.ed-cond-op select').getAttribute('aria-label'), row.querySelector('.ed-cond-type select').getAttribute('aria-label'),
                row.querySelector('[data-ed-cond-remove]').getAttribute('aria-label'), row.querySelector('.ed-cond-left .ed-tpl-view').getAttribute('aria-label'),
                cond.querySelector('.ed-seg').getAttribute('aria-label'), cases.querySelector('.ed-case-label').getAttribute('aria-label'),
                pair.querySelector('input').getAttribute('aria-label'), pair.querySelector('.ed-tpl-view').getAttribute('aria-label'), pair.querySelector('[data-ed-kv-remove]').getAttribute('aria-label'),
                tags.querySelector('.ed-tags-input').getAttribute('aria-label'), tags.querySelector('[data-ed-tag-remove]').getAttribute('aria-label')],
            ['cond_op', 'cond_type', 'remove_row:1', 'cond_left', 'cond_match', 'case_label', 'kv_key', 'kv_value', 'remove_row:1', 'tags_add', 'remove_tag:a']);
        const chip = F.templateField(env, '{{alpha.x}}', {}).el.querySelector('.ed-tpl-view').html;
        check('c1d05 a chip escapes the category of its step', chip.includes('cat-x&quot; onclick=&quot;evil') && !chip.includes('cat-x" onclick'), chip);
        eq('c1d05 the param checks log no errors', h.logged, []);
    });
}
