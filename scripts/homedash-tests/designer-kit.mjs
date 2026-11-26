// Shared test kit for the M4 part-2 DOM modules (palette, inspector, look, dialogs): boots the engine up to a file with the fake DOM, the golden catalog,
// a fake store/api and a fake designer D that applies the executed commands to its document. Not a test file (no `.test.mjs` suffix).
import { loadEngine, APP_ORDER, read, plain } from './harness.mjs';
import { FakeElement } from './dom.mjs';
export const CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json'));
export const FILES = upto => APP_ORDER.slice(0, APP_ORDER.indexOf(upto) + 1);
export const boot = upto => { const { E, window, document } = loadEngine(FILES(upto), { dom: true }); return { E, window, document, cat: E.catalog.create(CATALOG) }; };
export { FakeElement };
// Dispatches on `node` itself with `target` set: the fake DOM has no bubbling, so delegated listeners are fed directly.
export const fire = (node, type, extra, target) => { const ev = Object.assign({ type, target: target || node, preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; } }, extra); node.dispatchEvent(ev); return ev; };
// fire() returns the event, so a test reads ev.defaultPrevented and ev.stopped.
export const flush = (ms = 30) => new Promise(r => setTimeout(r, ms));
export function fakeStore(views) {
    const set = new Set(), calls = [], v = views || {}, released = [];
    return { views: v, calls, released, listeners: set, view: id => v[id] || null, on(type, fn) { set.add(fn); return () => set.delete(fn); }, emit(ids) { for (const fn of [...set]) fn(ids); },
        watch() { return { set(ids) { calls.push(ids.slice()); }, setActive() {}, release() { released.push(true); } }; }, docs: { get: () => null } };
}
export function fakeApi(entities) {
    const log = [], list = entities || [];
    return { log, entities: async p => { log.push(p); const rows = list.filter(e => (!p.kind || e.kind === p.kind) && (!p.q || (e.name + e.id).toLowerCase().includes(String(p.q).toLowerCase()))); return { entities: rows.slice(p.offset || 0, (p.offset || 0) + p.limit), total: rows.length, sources: [{ source: 'ha' }, { source: 'system' }] }; },
        entity: async id => { log.push({ entity: id }); const e = list.find(x => x.id === id); if (!e) { const err = new Error('nf'); err.status = 404; err.code = 'entity_not_found'; throw err; } return { entity: e }; } };
}
// A designer double: state() is rebuilt from the document after every exec; on('state') fires; commands are applied with E.doc.apply and recorded.
export function fakeD(E, cat, doc0, o) {
    const em = E.core.emitter(), x = o || {}, execs = [], calls = [], t = x.t || (k => k); let doc = doc0, sel = x.selection || [], pageId = x.pageId || doc0.pages[0].id;
    const state = () => { const page = doc.pages.find(p => p.id === pageId) || doc.pages[0]; pageId = page.id; return { doc, pageId, page, bp: 'lg', selection: sel, primary: sel[sel.length - 1] || null, focusId: x.focusId || null, drawers: !!x.drawers, items: page.layouts.lg }; };
    const D = { t, lang: 'en', catalog: cat, execs, calls, state, doc: () => doc, on: (type, fn) => em.on(type, fn), constraints: id => cat.constraints(doc.pages.find(p => p.id === pageId).widgets.find(w => w.id === id)),
        exec(cmd) { execs.push(cmd); if (!cmd || cmd.refused || cmd.limit) return false; doc = E.doc.apply(doc, cmd.ops); em.emit('state', state()); return true; },
        select(ids) { sel = ids; em.emit('state', state()); }, setDoc(d) { doc = d; em.emit('state', state()); },
        seal() { calls.push('seal'); }, announce(text) { calls.push('announce:' + text); }, closePanels() { calls.push('closePanels'); }, focusFrame(id) { calls.push('focusFrame:' + id); }, addWidgets(entries, opts) { calls.push({ addWidgets: plain(entries), opts }); return []; },
        gestures: { dragFrom(ev, payload) { calls.push({ dragFrom: payload.kind, label: payload.label }); D.lastDrag = payload; } }, sheet: () => x.sheet || null };
    return D;
}
export const docOf = (E, cat, widgets, layout, extra) => E.doc.normalise(Object.assign({ name: 'Test', pages: [{ id: 'main', name: 'Main', widgets, layouts: { lg: layout } }, { id: 'p2', name: 'Second', widgets: [], layouts: { lg: [] } }] }, extra), { sizeOf: cat.defaultSize, constraints: cat.constraints });
