// CYD Creator designer entry: P.designer.create(inst) owns the designer state (selection, active
// screen, active tab per tabview, zoom/snap/grid, issues) and the editing commands. Every layout
// change goes through inst.layoutStore.change with ops from P.model; the shell (plan 4a) saves them.
// Canvas, interact, palette, screens and inspector are mounted into the shell's containers.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const VIEW_KEY = 'aurago.cyd-creator.designer.view';
    const SYSTEM_WIFI = '__wifi_setup__';
    let clipboard = null; // shared by every designer window of this document

    function emitter() {
        const map = new Map();
        return {
            on(type, fn) { if (!map.has(type)) map.set(type, new Set()); map.get(type).add(fn); return () => map.get(type).delete(fn); },
            emit(type, data) { Array.from(map.get(type) || []).forEach(fn => { try { fn(data); } catch (err) { console.error('cyd-creator designer', type, err); } }); }
        };
    }
    function readView() { try { return JSON.parse(localStorage.getItem(VIEW_KEY) || '{}') || {}; } catch (err) { return {}; } }
    function writeView(v) { try { localStorage.setItem(VIEW_KEY, JSON.stringify(v)); } catch (err) { /* storage blocked */ } }

    function create(inst) {
        const M = P.model;
        const G = P.geometry;
        const t = inst.t;
        const store = inst.layoutStore;
        const bag = P.util.bag();
        const view = readView();
        const parts = {};
        const d = {
            inst, t, store, bag, bus: emitter(), SYSTEM_WIFI,
            specs: M.specMap(inst.caps.widgets),
            selection: [], screenId: '', tabs: new Map(),
            zoom: 0, zoomAuto: true,
            snap: G.snapStep(view.snap == null ? 8 : view.snap), showGrid: view.grid !== false,
            previewTheme: '', issues: [], inspectorEditing: false, disposed: false
        };
        d.project = () => store.get();
        d.readonly = () => !!(inst.readonly || (typeof inst.isLocked === 'function' && inst.isLocked()));
        d.theme = () => {
            const themes = inst.caps.themes || [];
            const id = d.previewTheme || d.project().theme;
            return themes.find(x => x.id === id) || themes[0] || { tokens: {} };
        };
        d.board = () => (inst.caps.boards || []).find(b => b.id === d.project().board) || null;
        d.screenSize = () => G.screenSize(d.board(), d.project().orientation);
        d.env = () => ({ specs: d.specs, theme: d.theme() });
        d.spec = type => d.specs.get(type) || null;
        d.isSystem = () => d.screenId === SYSTEM_WIFI;
        d.screen = () => { const f = M.findScreen(d.project(), d.screenId); return f ? f.screen : null; };
        d.find = id => M.findWidget(d.project(), id);
        d.activeTab = id => d.tabs.get(id) || 0;
        d.announce = text => { if (parts.canvas) parts.canvas.announce(text); };
        // A widget type's name: translated, else the catalog's type id.
        d.label = type => t.or('cyd_creator.widget.' + type, type);
        d.issueText = issue => { const key = 'cyd_creator.issue.' + issue.code; const s = t(key); return s && s !== key ? s : issue.message || issue.code; };
        d.issuesFor = w => d.issues.filter(i => i.widget && (i.widget === w.id || (w.name && i.widget === w.name)));
        d.issuesForScreen = s => d.issues.filter(i => i.screen && (i.screen === s.id || i.screen === s.name));
        d.inScreen = id => { const f = d.find(id); return !!f && !d.isSystem() && f.screen.id === d.screenId; };

        function pickScreen() {
            const p = d.project();
            if (d.isSystem() && p.settings && p.settings.wifi !== 'off') return;
            if (!M.findScreen(p, d.screenId)) {
                const start = M.findScreen(p, p.start_screen);
                d.screenId = start ? start.screen.id : ((p.screens || [])[0] || {}).id || '';
            }
        }
        pickScreen();

        // ── selection and view ──────────────────────────────────────────────
        d.select = (ids, opts) => {
            const o = opts || {};
            let next = o.toggle || o.add ? d.selection.slice() : [];
            (ids || []).forEach(id => {
                const i = next.indexOf(id);
                if (o.toggle && i >= 0) next.splice(i, 1); else if (i < 0) next.push(id);
            });
            next = next.filter(d.inScreen);
            if (next.join(' ') === d.selection.join(' ')) return;
            d.selection = next;
            d.bus.emit('selection', next);
        };
        d.setScreen = id => {
            const p = d.project();
            if (id === SYSTEM_WIFI ? !(p.settings && p.settings.wifi !== 'off') : !M.findScreen(p, id)) return;
            if (id === d.screenId) return;
            d.screenId = id === SYSTEM_WIFI ? id : M.findScreen(p, id).screen.id;
            d.selection = [];
            d.bus.emit('screen', d.screenId);
            d.bus.emit('selection', d.selection);
        };
        d.setTab = (id, tab) => { d.tabs.set(id, tab); d.bus.emit('view'); };
        // setZoom re-renders only on a change (the canvas fits again on every resize tick).
        d.setZoom = (z, auto) => {
            const next = G.clampZoom(z);
            if (next === d.zoom && !!auto === d.zoomAuto) return;
            d.zoom = next;
            d.zoomAuto = !!auto;
            d.bus.emit('view');
        };
        d.setSnap = s => { d.snap = G.snapStep(s); writeView({ snap: d.snap, grid: d.showGrid }); d.bus.emit('view'); };
        d.setGrid = on => { d.showGrid = !!on; writeView({ snap: d.snap, grid: d.showGrid }); d.bus.emit('view'); };
        d.setPreviewTheme = id => { if (d.previewTheme === (id || '')) return; d.previewTheme = id || ''; d.bus.emit('view'); };

        // ── commands ────────────────────────────────────────────────────────
        // opCtx is what P.model.apply needs besides the draft: catalog, screen size, theme ids.
        d.opCtx = () => ({ specs: d.specs, screenSize: d.screenSize(), themes: (inst.caps.themes || []).map(x => x.id) });
        function opError() {
            const text = t('cyd_creator.designer.op_failed');
            d.announce(text);
            if (typeof inst.notify === 'function') inst.notify('error', text);
        }
        // transact runs fn(draft, apply) as one undo step; apply(ops) applies layout ops to the draft.
        // Changes are never gated by validation (the server reports issues after saving); only
        // structurally impossible ops fail. Returns fn's result, or null when read-only or failed.
        function transact(label, fn, coalesce) {
            if (d.readonly()) return null;
            try {
                return store.change(label, draft => fn(draft, ops => M.apply(draft, ops, d.opCtx())), coalesce ? { coalesce } : undefined);
            } catch (err) {
                if (err && err.code === 'CYD_INVALID_LAYOUT') { opError(); return null; }
                throw err;
            }
        }
        // run applies ops as one undo step and returns the op results (null when read-only or failed).
        function run(label, ops, coalesce) {
            if (!ops.length) return null;
            return transact(label, (draft, apply) => apply(ops), coalesce);
        }
        const unlocked = ids => ids.filter(id => { const f = d.find(id); return f && !f.widget.locked; });
        const sel = () => M.topLevel(d.project(), d.selection);
        function frameOf(found) {
            if (!found || !found.parent) { const s = d.screenSize(); return { x: 0, y: 0, w: s.w, h: s.h }; }
            const o = M.absRect(d.project(), found.parent.id, d.env());
            const box = M.contentBox(found.parent, d.spec(found.parent.type));
            return { x: o.x + box.x, y: o.y + box.y, w: box.w, h: box.h };
        }
        d.frameOf = frameOf;

        const commands = {
            run, transact,
            // addWidget places type with its centre at point (screen coordinates) inside target
            // ({id, tab} container or null = screen root), snapped to the grid.
            addWidget(type, point, target) {
                const spec = d.spec(type);
                const screen = d.screen();
                if (!spec || !screen || d.readonly()) return null;
                const size = d.screenSize();
                const p = d.project();
                const tw = target && target.id ? d.find(target.id) : null;
                const frame = tw ? (() => { const o = M.absRect(p, tw.widget.id, d.env()); const b = M.contentBox(tw.widget, d.spec(tw.widget.type)); return { x: o.x + b.x, y: o.y + b.y, w: b.w, h: b.h }; })() : { x: 0, y: 0, w: size.w, h: size.h };
                const w = M.newWidget(p, spec, { x: 0, y: 0 }, size);
                if (type !== 'statusbar' || tw) {
                    const at = point || { x: frame.x + frame.w / 2, y: frame.y + frame.h / 2 };
                    const raw = { x: Math.round(at.x - w.w / 2), y: Math.round(at.y - w.h / 2), w: w.w, h: w.h };
                    const inside = G.clampInto(raw, frame);
                    w.x = G.snapValue(inside.x - frame.x, d.snap);
                    w.y = G.snapValue(inside.y - frame.y, d.snap);
                }
                const op = { op: 'add_widget', screen: screen.id, widget: w };
                if (tw) { op.parent = tw.widget.id; if (tw.widget.type === 'tabview') op.tab = target.tab || 0; }
                const res = run('add', [op]);
                if (!res) return null;
                d.select([res[0].id]);
                d.announce(t('cyd_creator.designer.added', { name: d.label(type) }));
                return res[0].id;
            },
            addAtCenter(type) { return commands.addWidget(type, null, null); },
            remove() {
                const ids = unlocked(sel());
                if (!ids.length) return;
                if (run('delete', ids.map(id => ({ op: 'delete_widget', id })))) {
                    d.select([]);
                    d.announce(t('cyd_creator.designer.deleted', { count: ids.length }));
                }
            },
            duplicate() {
                const p = d.project();
                const names = M.usedNames(p);
                const ops = sel().map(id => {
                    const f = d.find(id);
                    const copy = M.cloneSubtree(p, f.widget, names);
                    copy.x += 8; copy.y += 8;
                    delete copy.locked;
                    const op = { op: 'add_widget', screen: f.screen.id, widget: copy };
                    if (f.parent) { op.parent = f.parent.id; if (f.parent.type === 'tabview') op.tab = f.widget.tab || 0; }
                    return op;
                });
                const res = run('duplicate', ops);
                if (res) d.select(res.map(r => r.id));
            },
            copy() {
                const ids = sel();
                if (!ids.length) return false;
                const p = d.project();
                clipboard = { count: 0, items: ids.map(id => ({ widget: P.util.deepClone(d.find(id).widget), abs: M.absRect(p, id, d.env()) })) };
                d.announce(t('cyd_creator.designer.copied', { count: ids.length }));
                return true;
            },
            cut() { if (commands.copy()) commands.remove(); },
            paste() {
                const screen = d.screen();
                if (!clipboard || !screen || d.readonly()) return;
                clipboard.count++;
                const p = d.project();
                const names = M.usedNames(p);
                const offset = 8 * clipboard.count;
                const ops = clipboard.items.map(item => {
                    const copy = M.cloneSubtree(p, item.widget, names);
                    delete copy.tab;
                    copy.x = item.abs.x + offset; copy.y = item.abs.y + offset;
                    return { op: 'add_widget', screen: screen.id, widget: copy };
                });
                const res = run('paste', ops);
                if (res) d.select(res.map(r => r.id));
            },
            nudge(dx, dy) {
                const ids = unlocked(sel());
                if (!ids.length) return;
                const ops = ids.map(id => { const w = d.find(id).widget; return { op: 'move_widget', id, x: w.x + dx, y: w.y + dy }; });
                run('nudge', ops, 'nudge:' + ids.join(','));
            },
            // place moves widgets to absolute rects (screen coordinates) inside their parents.
            place(rects) {
                const p = d.project();
                const ops = rects.map(r => { const o = M.absOrigin(p, r.id, d.env()); return { op: 'move_widget', id: r.id, x: r.x - o.x, y: r.y - o.y }; });
                return run('arrange', ops);
            },
            align(mode) {
                const ids = unlocked(sel());
                if (!ids.length) return;
                const p = d.project();
                const rects = ids.map(id => M.absRect(p, id, d.env()));
                const out = G.align(rects, mode, ids.length === 1 ? frameOf(d.find(ids[0])) : null);
                commands.place(out.map((r, i) => Object.assign({ id: ids[i] }, r)));
            },
            distribute(axis) {
                const ids = unlocked(sel());
                if (ids.length < 3) return;
                const p = d.project();
                const out = G.distribute(ids.map(id => M.absRect(p, id, d.env())), axis);
                commands.place(out.map((r, i) => Object.assign({ id: ids[i] }, r)));
            },
            zorder(dir) {
                const ops = sel().map(id => {
                    const f = d.find(id);
                    const last = f.list.length - 1;
                    const index = dir === 'front' ? last : dir === 'back' ? 0 : dir === 'forward' ? Math.min(last, f.index + 1) : Math.max(0, f.index - 1);
                    return { op: 'reorder_widget', id, index };
                });
                run('zorder', ops);
            },
            toggle(key) {
                const ids = sel();
                if (!ids.length) return;
                const all = ids.every(id => d.find(id).widget[key]);
                run(key, ids.map(id => ({ op: 'update_widget', id, set: { [key]: !all } })));
            },
            update(id, set, coalesce) { return run('edit', [{ op: 'update_widget', id, set }], coalesce); },
            undo() { if (!d.readonly()) store.undo(); },
            redo() { if (!d.readonly()) store.redo(); },
            selectAll() {
                const screen = d.screen();
                if (screen) d.select((screen.widgets || []).filter(w => !w.hidden).map(w => w.id));
            }
        };
        d.commands = commands;

        // ── store, bus and shell events ─────────────────────────────────────
        // A store change emits 'project' ({before, label, origin, screenChanged}). When it removed
        // the active screen, 'screen' comes first and 'project' carries screenChanged: true, so a
        // part that renders on both renders once.
        let previous = d.project();
        bag.add(store.subscribe((project, info) => {
            const before = previous;
            previous = project;
            const screenBefore = d.screenId;
            pickScreen();
            const kept = d.selection.filter(d.inScreen);
            if (kept.length !== d.selection.length) { d.selection = kept; d.bus.emit('selection', kept); }
            const screenChanged = screenBefore !== d.screenId;
            if (screenChanged) d.bus.emit('screen', d.screenId);
            d.bus.emit('project', Object.assign({ before, screenChanged }, info));
        }));
        const bus = inst.bus;
        // listen subscribes to the shell bus; dispose uses on()'s unsubscribe or off(type, fn).
        function listen(type, fn) {
            if (!bus || typeof bus.on !== 'function') return;
            const off = bus.on(type, fn);
            bag.add(() => { if (typeof off === 'function') off(); else if (typeof bus.off === 'function') bus.off(type, fn); });
        }
        // The shell sends the issues after every save, reload and upload, usually unchanged: an
        // identical list re-renders nothing (a re-render would reset a name being typed in the
        // inspector and make gesture previews flicker).
        listen('layout:issues', list => {
            const next = Array.isArray(list) ? list : [];
            if (JSON.stringify(next) === JSON.stringify(d.issues)) return;
            d.issues = next;
            d.bus.emit('issues', d.issues);
        });
        // Locked by an agent job or read-only (the desktop policy can change while the window is
        // open): a running gesture ends, and canvas, palette, screens and inspector re-render
        // disabled (or enabled again).
        listen('lock:changed', () => { if (parts.interact) parts.interact.abort(); d.bus.emit('lock'); });
        listen('readonly:changed', () => { if (parts.interact) parts.interact.abort(); d.bus.emit('lock'); });
        // Code mode hides the canvas: a running gesture ends there.
        listen('mode:changed', mode => { if (mode === 'code' && parts.interact) parts.interact.abort(); });
        listen('designer:reveal', target => api.reveal(target || {}));

        function mount(key, factory) {
            if (parts[key]) { parts[key].dispose(); delete parts[key]; }
            parts[key] = factory();
            return parts[key];
        }

        const api = {
            mountCanvas(el) {
                if (parts.interact) { parts.interact.dispose(); delete parts.interact; }
                const canvas = mount('canvas', () => P.designerCanvas.mount(d, el));
                d.canvas = canvas;
                parts.interact = P.designerInteract.attach(d, canvas);
                d.interact = parts.interact;
            },
            mountPalette(el) { mount('palette', () => P.designerPalette.mount(d, el)); },
            mountScreens(el) { mount('screens', () => P.designerScreens.mount(d, el)); },
            mountInspector(el) { mount('inspector', () => P.designerInspector.mount(d, el)); },
            // reveal shows a screen and/or selects a widget (by id or name), opening the tab pages
            // on its path, scrolling it into view and flashing it.
            reveal(target) {
                const o = target || {};
                const p = d.project();
                if (o.widget) {
                    const f = M.findWidget(p, o.widget, o.screen || undefined) || M.findWidget(p, o.widget);
                    if (f) {
                        d.setScreen(f.screen.id);
                        const chain = f.ancestors.concat([f.widget]);
                        chain.forEach((a, i) => { if (a.type === 'tabview' && chain[i + 1]) d.tabs.set(a.id, chain[i + 1].tab || 0); });
                        d.bus.emit('view');
                        d.select([f.widget.id]);
                        if (parts.canvas) { parts.canvas.scrollTo(f.widget.id); parts.canvas.flash([f.widget.id]); }
                        return;
                    }
                }
                if (o.screen) { const s = M.findScreen(p, o.screen); if (s) d.setScreen(s.screen.id); }
            },
            dispose() {
                if (d.disposed) return;
                d.disposed = true;
                ['interact', 'inspector', 'screens', 'palette', 'canvas'].forEach(k => { if (parts[k]) { parts[k].dispose(); delete parts[k]; } });
                bag.dispose();
            },
            // state is for tests and the shell's diagnostics; treat it as read-only.
            get state() { return d; }
        };
        return api;
    }

    P.designer = { create, SYSTEM_WIFI };
})();
