// c1d14 checks: how the canvas restores a stored view (canvas.restoreView, used by the editor's
// placeView) and its readable fit. They run on the stub DOM of test-easydrag.mjs; that runner calls
// run(env) with its helpers and counts the failures.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const MODULES = ['easydrag-core.js', 'easydrag-template.js', 'easydrag-model.js', 'easydrag-geometry.js', 'easydrag-canvas.js'];
const T1 = 'n_tttttttt';

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
    const { apps, types, miniDom, eq, check, guardAsync } = env;

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
