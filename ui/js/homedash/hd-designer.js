// HomeDash designer controller (M4 contract C4-C6, C14-C16; frontend 5). create(o) turns one open dashboard into an editor: it owns the working document, the
// history, the selection and focus, the edit chrome (E.editbar, conflict bar, drawers) and the gesture controller, and it saves through the realm lane o.lane.
// Every change is one exec(command): apply, history, lane.set, a synchronous render (the gestures rely on it), selection, announcement. Construction order
// (F1): the gesture controller exists before the first render() because sync() reads it. Keys: one root listener, preventDefault on every consumed key.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.commands || !E.save || !E.gestures || !E.editbar || !E.clipboard || !E.history || !E.sheet || !E.view) throw new Error('hd-designer.js: load hd-commands.js, hd-save.js, hd-gestures.js, hd-editbar.js and hd-clipboard.js first');
    const DRAWER_BELOW = 900, BPS = Object.freeze(['lg', 'md', 'sm']), NBSP = String.fromCodePoint(0xa0), FRAME = '[data-hd-wid]';
    const ACTIONS = Object.freeze(['undo', 'redo', 'cut', 'copy', 'paste', 'duplicate', 'delete', 'select_all', 'select_none', 'reset_layout', 'retry_save', 'resolve_conflict', 'show_draft', 'add_widget', 'focus_inspector', 'toggle_palette', 'toggle_inspector',
        'layout_lg', 'layout_md', 'layout_sm', 'page_prev', 'page_next']);
    function create(o) {
        if (!o || !o.lane || !o.root || !o.view) throw new Error('hd-designer.js: root, view and lane are required');
        const root = o.root, view = o.view, t = o.t, catalog = o.catalog, lane = o.lane, C = E.commands, life = E.core.bag(), bus = E.core.emitter(), history = E.history.create({ limit: 100 });
        const tm = o.timers || {}, setT = tm.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = tm.clearTimeout || (h => clearTimeout(h)), now = typeof tm.now === 'function' ? tm.now : () => Date.now();
        let doc = o.doc, pageId = view.pageId(), bp = 'lg', selection = [], focusId = null, conflict = null, drawers = false, panel = '', lastLive = '', destroyed = false, redoLabels = [], palette = null, inspector = null, gestures = null, D = null;
        let retryTimer = 0, lastPhase = 'saved', problem = false, autoOpened = false, resolving = false, minePending = false, ro = null;
        // Every sheet of the designer hands focus back inside the root when it closes: its opener may be gone, hidden by the command, or have been body.
        const sheet0 = E.sheet.create(root), after = v => { settleFocus(); return v; };
        const sheet = Object.freeze({ el: sheet0.el, isOpen: sheet0.isOpen, close: sheet0.close, destroy: sheet0.destroy, open: spec => sheet0.open(spec).then(after), confirm: spec => sheet0.confirm(spec).then(after) });
        const bar = E.editbar.create({ t, now, onCommand: (name, arg, ev) => command(name, arg, ev) });
        const page = () => doc.pages.find(p => p.id === pageId) || doc.pages[0], widget = id => page().widgets.find(w => w.id === id) || null;
        const constraints = id => { const w = widget(id); return w ? catalog.constraints(w) : {}; }, items = () => E.layout.effective(page().layouts, bp, constraints);
        const typeName = w => t('homedash.widget.' + w.type + '.name'), variantName = w => t('homedash.variant.' + w.type + '.' + w.variant), docEl = () => root.ownerDocument;
        function nameOf(w) { if (!w) return ''; const id = w.bindings && w.bindings.primary, v = id ? o.store.view(id) : null; return w.title || (v && v.name) || typeName(w); }
        function announce(text) { if (!text) return; const s = text === lastLive ? text + NBSP : text; lastLive = s; view.announce(s); }
        // Helpers of render()/sync()/exec() (all in the create() closure, before the first render). currentState() is the C4 state builder.
        const ANNOUNCE = Object.freeze({
            moved: 'homedash.designer.announce.moved', moved_many: 'homedash.designer.announce.moved_many',
            resized: 'homedash.designer.announce.resized', added: 'homedash.designer.announce.added',
            added_many: 'homedash.designer.announce.added_many', deleted: 'homedash.designer.announce.deleted',
            deleted_many: 'homedash.designer.announce.deleted_many',
            now_auto_md: 'homedash.bp.now_auto_md', now_auto_sm: 'homedash.bp.now_auto_sm'
        });
        function emit() { if (!destroyed) bus.emit('state', currentState()); }
        // A write that only repeats the current value still costs: setAttribute('tabindex') on the focused frame forces a synchronous layout (~1 per sync on the
        // 60-widget page), so sync() and the badge write an attribute only when it differs.
        function setIf(node, name, value) {
            const want = value === null || value === undefined || value === false ? null : value === true ? 'true' : String(value);
            if (node.getAttribute(name) !== want) E.core.setAttr(node, name, value);
        }
        function draftBadge(frame, on) {
            const old = frame.querySelector('.hd-draft-badge');
            if (on && !old) {
                const badge = E.core.el('span', { class: 'hd-draft-badge' });
                badge.appendChild(E.icons.icon('plug'));
                badge.appendChild(E.core.el('span', { class: 'hd-draft-text', text: t('homedash.designer.draft_state') }));
                frame.appendChild(badge);
            } else if (!on && old) old.remove();
            setIf(frame, 'data-hd-draft', on ? 'true' : null);
        }
        // One literal t() call per key: a ternary of keys inside t() would escape the key lint (R4).
        function limitText(limit) {
            return limit === 'edge' ? t('homedash.designer.announce.edge')
                : limit === 'min' ? t('homedash.designer.announce.limit_min')
                : limit === 'max' ? t('homedash.designer.announce.limit_max')
                : t('homedash.designer.announce.blocked');
        }
        function announceCmd(cmd) {
            const a = cmd && cmd.announce, key = a ? ANNOUNCE[a.key] : null; // literal map: no dynamic i18n key
            if (!key) return;
            const w = widget(a.id);
            announce(t(key, Object.assign({}, a, { name: w ? nameOf(w) : '' })));
        }
        function describe(w, item) {
            const i = item || { x: 0, y: 0, w: 1, h: 1 };
            const base = t('homedash.designer.frame_label', { type: typeName(w), name: nameOf(w), col: i.x + 1, row: i.y + 1, w: i.w, h: i.h });
            return E.doc.drafts(doc, catalog).has(w.id) ? base + ', ' + t('homedash.designer.frame_unbound') : base;
        }
        function describeLimit(kind, label) { return kind === 'drop_invalid' ? t('homedash.designer.announce.drop_invalid', { entity: label || '' }) : kind === 'page_full' ? t('homedash.designer.announce.limit_widgets', { max: C.LIMITS.pageWidgets }) : limitText(kind); }
        // ---- state, render, sync
        const pageElId = () => { const pg = view.page(); return pg ? pg.el.getAttribute('id') : null; };
        function currentState() {
            const pg = page(), st = lane.state(), peek = history.peek();
            return Object.freeze({ doc, pageId: pg.id, page: pg, bp, override: { md: Array.isArray(pg.layouts.md), sm: Array.isArray(pg.layouts.sm) }, selection: selection.slice(), primary: selection.length ? selection[selection.length - 1] : null, focusId,
                canUndo: history.canUndo(), canRedo: history.canRedo(), undoLabel: peek ? t(peek.label) : '', redoLabel: redoLabels.length ? t(redoLabels[redoLabels.length - 1]) : '', save: st, saveReason: st.error ? E.save.errorText(t, st.error) : '',
                drafts: E.doc.drafts(doc, catalog).size, conflict, drawers, panel, clipboard: !!(o.clipboard && o.clipboard.has()), gesture: gestures.active(), items: items() });
        }
        function barState(s) {
            return { canUndo: s.canUndo, canRedo: s.canRedo, undoLabel: s.undoLabel, redoLabel: s.redoLabel, pages: doc.pages.map((p, i) => ({ id: p.id, name: p.name || t('homedash.designer.page_n', { n: i + 1 }) })), pageId: s.pageId, panelId: pageElId(), bp, override: s.override,
                save: s.save, saveReason: s.saveReason, drafts: s.drafts, hasLook: !!(E.ddialogs && E.ddialogs.run), drawers, panel, hasPalette: !!shells.palette, hasInspector: !!shells.inspector, ids: { palette: shells.palette && shells.palette.getAttribute('id'), inspector: shells.inspector && shells.inspector.getAttribute('id') } };
        }
        function focusState() { const act = docEl().activeElement, inCanvas = !!(act && view.stage.contains(act)), w = inCanvas && typeof act.closest === 'function' ? act.closest(FRAME) : null; return { in: inCanvas, fid: w ? w.getAttribute('data-hd-wid') : null }; }
        function restoreFocus(fs, force) {   // never leave focus on body: the same frame, else the focused one, else the page
            if (!fs.in && !force) return; const act = docEl().activeElement; if (act && act !== root && root.contains(act) && !(force && !view.stage.contains(act))) return;
            const pg = view.page(), f = pg && ((fs.fid && pg.frame(fs.fid)) || (focusId && pg.frame(focusId)) || pg.el); if (f && typeof f.focus === 'function') f.focus({ preventScroll: true });
        }
        // Focus is never left on body or on a hidden control (a command can hide the control that ran it): the selected frame, else the first one, else the page, else Done.
        const usable = a => !!a && a !== docEl().body && a !== root && root.contains(a) && !(typeof a.closest === 'function' && a.closest('[hidden]'));
        const wasInside = () => { const a = docEl().activeElement; return !!a && a !== docEl().body && root.contains(a); };
        function settleFocus() {
            if (destroyed || usable(docEl().activeElement)) return; const pg = view.page(), order = pg ? C.readingOrder(items()) : [];
            const id = selection.slice().reverse().find(i => order.includes(i)) || (focusId && order.includes(focusId) ? focusId : null) || order[0] || null, f = pg ? (id && pg.frame(id)) || pg.el : null;
            if (f && typeof f.focus === 'function') f.focus({ preventScroll: true }); else bar.focus('done');
        }
        const guard = fn => { const had = wasInside(), r = fn(); if (had) settleFocus(); return r; };
        // keepBox: the stage did not change size (a gesture commit), so the page reuses its last measure instead of forcing a layout to read it again.
        function render(keepBox) {
            const fs = focusState(); view.update(doc, { normalised: true, keepBox: !!keepBox }); pageId = view.pageId(); sync(); restoreFocus(fs);
        }
        function sync() {
            const pg = view.page(); if (!pg) return; const order = C.readingOrder(items()), drafts = E.doc.drafts(doc, catalog);
            selection = selection.filter(id => order.includes(id));
            const anchor = (focusId && order.includes(focusId) && focusId) || selection[selection.length - 1] || order[0] || null;
            for (const id of order) { const f = pg.frame(id); if (!f) continue; setIf(f, 'aria-selected', selection.includes(id) ? 'true' : 'false'); setIf(f, 'tabindex', id === anchor ? '0' : '-1'); draftBadge(f, drafts.has(id)); }
            setIf(pg.el, 'tabindex', order.length ? '-1' : '0');
            const one = selection.length === 1 ? widget(selection[0]) : null;
            gestures.showSelection(one ? one.id : null, one ? (one.family ? t('homedash.designer.chip_family', { type: typeName(one), variant: variantName(one), family: t('homedash.family.' + one.family) }) : t('homedash.designer.chip', { type: typeName(one), variant: variantName(one) })) : '');
            emit();
        }
        const frameOfFocus = () => focusState().fid, focused = () => { const f = frameOfFocus() || focusId; return f && widget(f) ? f : null; };
        // The reads (focus, scrollIntoView) run before the writes of sync(): each read after a write is a forced layout (~220 ms on the 60-widget page under load).
        // focusId is set first, so the focusin handler finds nothing to sync.
        function focusFrame(id, scroll) {
            const pg = view.page(); if (!pg) return; if (id) focusId = id;
            const f = (id && pg.frame(id)) || pg.el; if (typeof f.focus === 'function') f.focus({ preventScroll: true }); if (scroll && f !== pg.el && typeof f.scrollIntoView === 'function') f.scrollIntoView({ block: 'nearest' });
            if (id) sync();
        }
        function sayCount(ids) {
            if (ids.length > 1) announce(t('homedash.designer.announce.selected', { count: ids.length }));
            else if (ids.length === 1) { const w = widget(ids[0]); if (w) announce(describe(w, items().find(i => i.id === ids[0]))); } else announce(t('homedash.designer.announce.selection_cleared'));
        }
        function select(ids, so) {
            const s = so || {}, order = C.readingOrder(items()); let list = [...new Set(ids || [])].filter(id => order.includes(id));
            if (s.primary && list.includes(s.primary)) list = list.filter(i => i !== s.primary).concat(s.primary);
            selection = list; if (s.focus && list.length) focusId = list[list.length - 1];
            if (s.focus && list.length) focusFrame(list[list.length - 1], true); else { sync(); if (s.focus) focusFrame(null, true); }   // focusFrame syncs once itself
            if (s.announce) sayCount(list);
        }
        // ---- commands
        function toastText(key) { return key === 'reload_agent' ? t('homedash.designer.reload_agent') : key === 'reload_other' ? t('homedash.designer.reload_other') : key === 'theirs_done' ? t('homedash.conflict.theirs_done') : ''; }
        function exec(cmd, xo) { const ok = execCommand(cmd, xo); if (ok) settleFocus(); return ok; }
        function execCommand(cmd, xo) {
            if (!cmd || destroyed) return false;
            if (cmd.refused) { announce(cmd.refused === 'last_page' ? t('homedash.designer.announce.last_page') : doc.pages.length >= C.LIMITS.pages && page().widgets.length < C.LIMITS.pageWidgets ? t('homedash.designer.announce.limit_pages', { max: C.LIMITS.pages }) : t('homedash.designer.announce.limit_widgets', { max: C.LIMITS.pageWidgets })); return false; }
            if (cmd.limit) { announce(limitText(cmd.limit)); return false; }
            const before = doc, a = cmd.announce, gone = a && a.key === 'deleted' ? before.pages.flatMap(p => p.widgets).find(w => w.id === a.id) : null; let next;
            try { next = E.doc.apply(doc, cmd.ops, 'after'); } catch (err) { console.warn('HomeDash: command failed', err); return false; }
            gestures.cancel(); doc = next; history.push(cmd); redoLabels = []; lane.set(doc);
            const fs = focusState(); render(xo && xo.keepBox); if (cmd.page && cmd.page !== pageId) setPage(cmd.page, { focus: fs.in }); if (cmd.select) select(cmd.select, { focus: true });
            const kind = cmd.meta && cmd.meta.kind, ids = (cmd.meta && cmd.meta.ids) || [], draft = ids.length === 1 && (kind === 'add' || kind === 'paste' || kind === 'duplicate') && E.doc.drafts(doc, catalog).has(ids[0]);
            if (xo && xo.quiet) return true;
            if (cmd.materialised) announce(cmd.materialised === 'md' ? t('homedash.bp.now_custom_md') : t('homedash.bp.now_custom_sm'));
            else if (draft) announce(t('homedash.designer.announce.added_draft', { name: nameOf(widget(ids[0])) }));
            else if (gone) announce(t('homedash.designer.announce.deleted', { name: nameOf(gone) }));
            else announceCmd(cmd);
            return true;
        }
        function undoRedo(dir) { const ok = undoRedoStep(dir); if (ok) settleFocus(); return ok; }
        function undoRedoStep(dir) {
            gestures.cancel(); let r = null;
            try { r = dir === 'undo' ? history.undo(doc) : history.redo(doc); } catch (err) { console.warn('HomeDash: history cleared', err); history.clear(); redoLabels = []; emit(); return false; }
            if (!r) { announce(dir === 'undo' ? t('homedash.designer.announce.nothing_undo') : t('homedash.designer.announce.nothing_redo')); return false; }
            if (dir === 'undo') redoLabels.push(r.command.label); else redoLabels.pop();
            doc = r.doc; lane.set(doc); render(); select(((r.command.meta && r.command.meta.ids) || []).filter(id => widget(id)), {});
            announce(dir === 'undo' ? t('homedash.designer.announce.undone', { label: t(r.command.label) }) : t('homedash.designer.announce.redone', { label: t(r.command.label) })); return true;
        }
        function replace(next, r) {
            const x = r || {}; gestures.cancel(); doc = next; if (x.clearHistory) { history.clear(); redoLabels = []; }
            problem = false; if (x.revision != null) lane.reset(next, x.revision); else lane.set(next);
            setConflict(null); render(); const text = toastText(x.toastKey); if (text) toast(text, 'info');
        }
        function bottomOf(ids) { const sel = page().layouts.lg.filter(i => ids.includes(i.id)), b = sel.length ? E.grid.bbox(sel) : null; return b ? b.y + b.h : null; }
        function addWidgets(entries, ao) {
            const a = ao || {}, y = bottomOf(selection), cmd = C.add(doc, { pageId, bp, entries, at: a.at, startY: y == null ? 0 : y, C: constraints, limitsOf: catalog.constraints, now: now() });
            if (!exec(cmd)) return []; const pg = view.page(), f = pg && pg.frame(cmd.meta.ids[0]); if (f && typeof f.scrollIntoView === 'function') f.scrollIntoView({ block: 'nearest' }); return cmd.meta.ids.slice();
        }
        const targets = () => (selection.length ? selection.slice() : focused() ? [focused()] : []);
        function copy(quiet) {
            const ids = targets(); if (!ids.length) return false;
            const text = E.clipboard.encode(E.doc.forSave(doc, { catalog }), pageId, ids); if (!text || !o.clipboard || !o.clipboard.write(text)) return false;
            if (!quiet) announce(ids.length === 1 ? t('homedash.designer.announce.copied_one') : t('homedash.designer.announce.copied_many', { count: ids.length })); emit(); return true;
        }
        function cut() {
            const ids = targets(); if (!ids.length || !copy(true)) return false; const cmd = C.remove(doc, { pageId, ids, label: 'homedash.cmd.cut', kind: 'cut', now: now() }); if (!cmd) return false; cmd.announce = null;
            if (!exec(cmd)) return false; announce(ids.length === 1 ? t('homedash.designer.announce.cut_one') : t('homedash.designer.announce.cut_many', { count: ids.length })); return true;
        }
        function paste() {
            const text = o.clipboard && o.clipboard.read(); if (!text) { announce(t('homedash.designer.announce.clipboard_empty')); return true; }
            const entries = E.clipboard.decode(text, { catalog, pages: new Set(doc.pages.map(p => p.id)) }); if (!entries.length) { announce(t('homedash.designer.announce.clipboard_bad')); return true; }
            const y = bottomOf(selection), end = items().reduce((m, i) => Math.max(m, i.y + i.h), 0);
            exec(C.paste(doc, { pageId, bp, entries, startY: y == null ? end : y, C: constraints, limitsOf: catalog.constraints, now: now() })); return true;
        }
        function duplicate() { const ids = targets(); if (!ids.length) return false; exec(C.duplicate(doc, { pageId, ids, C: constraints, limitsOf: catalog.constraints, now: now() })); return true; }
        function del() { const ids = targets(); if (!ids.length) return false; exec(C.remove(doc, { pageId, ids, now: now() })); return true; }
        function selectAll() { const order = C.readingOrder(items()), f = focused(); select(f ? order.filter(i => i !== f).concat(f) : order, { announce: true }); return order.length > 0; }
        function setVariant(id, v) { const w = widget(id); if (w) exec(C.variant(doc, { pageId, id, variant: v, limits: catalog.constraints({ type: w.type, variant: v }), now: now() })); }
        function nudge(dx, dy) {
            const ids = targets(); if (!ids.length) return false; if (!selection.length) select(ids, {});
            exec(C.nudge(doc, { pageId, bp, ids, dx, dy, C: constraints, now: now() })); return true;
        }
        function resizeStep(dw, dh) {
            const ids = targets(); if (!ids.length) return false; if (ids.length > 1) { announce(t('homedash.designer.announce.resize_single')); return true; } if (!selection.length) select(ids, {});
            exec(C.resizeStep(doc, { pageId, bp, id: ids[0], dw, dh, C: constraints, now: now() })); return true;
        }
        function tabMove(dir) {
            const order = C.readingOrder(items()), f = frameOfFocus(), i = f ? order.indexOf(f) : -1;
            if (i < 0) { if (dir > 0 && order.length && !f) { focusFrame(order[0], true); return true; } return false; }
            const j = i + dir; if (j < 0 || j >= order.length) return false; focusFrame(order[j], true); return true;
        }
        function stepPage(dir) { const ids = doc.pages.map(p => p.id); if (ids.length < 2) return false; setPage(ids[(ids.indexOf(pageId) + dir + ids.length) % ids.length], { focus: true }); return true; }
        function setPage(id, so) {
            if (!doc.pages.some(p => p.id === id) || id === pageId) return; const fs = focusState(); gestures.cancel(); view.select(id);
            const pg = page(); announce(t('homedash.designer.announce.page', { name: pg.name || id })); if (fs.in || (so && so.focus)) restoreFocus({ in: true, fid: null }, true);
        }
        function setBreakpoint(b) { if (!BPS.includes(b) || b === bp) return; gestures.cancel(); bp = b; view.setBreakpoint(b); sync(); }
        function showDraft() {
            const dr = E.doc.drafts(doc, catalog); for (const p of doc.pages) { const id = C.readingOrder(p.layouts.lg).find(i => dr.has(i)); if (!id) continue;
                if (p.id !== pageId) setPage(p.id, {}); select([id], { focus: true }); openPanel('inspector'); if (inspector && typeof inspector.setTab === 'function') inspector.setTab('data'); return true; }
            return false;
        }
        // ---- panels (drawers below DRAWER_BELOW)
        const shells = { palette: null, inspector: null }, scrim = E.core.el('div', { class: 'hd-drawer-scrim', 'aria-hidden': 'true', hidden: 'true' });
        function applyPanels() {
            E.core.setAttr(root, 'data-hd-drawers', drawers ? 'true' : 'false'); E.core.setAttr(root, 'data-hd-panel', panel || ''); E.core.setAttr(scrim, 'hidden', !(drawers && panel));
            for (const n of ['palette', 'inspector']) if (shells[n]) E.core.setAttr(shells[n], 'data-hd-drawer', !drawers || panel === n ? 'open' : 'closed');
        }
        function panelFocus(name) { const h = name === 'palette' ? palette : inspector; if (!h) return false; if (name === 'palette' && typeof h.focusSearch === 'function') h.focusSearch(); else if (typeof h.focusFirst === 'function') h.focusFirst(); return true; }
        function openPanel(name) { if (!shells[name]) return false; if (drawers) { panel = name; applyPanels(); emit(); } return panelFocus(name); }
        function closePanels() { if (!panel) return; const was = panel, act = docEl().activeElement, inside = !act || act === docEl().body || (shells[was] && shells[was].contains(act)); panel = ''; applyPanels(); emit(); if (inside) bar.focus('toggle_' + was); }
        function togglePanel(name) { if (!shells[name]) return false; if (drawers && panel === name) { closePanels(); return true; } return openPanel(name); }
        function measure() { const w = root.clientWidth, d = Number.isFinite(w) && w > 0 && w < DRAWER_BELOW; if (d === drawers) return; drawers = d; if (!d) panel = ''; else { const act = docEl().activeElement; panel = ['palette', 'inspector'].find(n => shells[n] && act && shells[n].contains(act)) || ''; }   // E2: the shell holding focus becomes the open drawer, so focus never lands on a hidden control
            applyPanels(); guard(() => emit()); }
        // ---- conflict, lane
        const cText = E.core.el('span', { class: 'hd-conflict-text' }), cbar = E.core.el('div', { class: 'hd-conflict-bar', role: 'alert', 'data-tone': 'warn', hidden: 'true' }, [E.icons.icon('alert'), cText,
            E.core.el('button', { type: 'button', class: 'hd-eb-btn', 'data-hd-cmd': 'resolve_conflict', text: t('homedash.conflict.resolve') })]);
        function setConflict(c) { conflict = c; if (!c) autoOpened = false; guard(() => { paintConflict(); emit(); }); }
        function paintConflict() {
            E.core.setAttr(cbar, 'hidden', !conflict); if (!conflict) return;
            const text = conflict.gone ? t('homedash.conflict.gone_title') : conflict.savedBy === 'agent' ? t('homedash.conflict.banner_agent') : t('homedash.conflict.banner'); if (cText.textContent !== text) cText.textContent = text;
        }
        function toast(text, tone) { view.toast(text, { tone: tone || 'info' }); }
        function tick() { retryTimer = 0; if (destroyed) return; bar.update(barState(currentState())); if (lane.state().phase === 'retrying') retryTimer = setT(tick, 1000); }
        function onLane(st) {
            if (destroyed) return; const ph = st.phase, was = lastPhase; lastPhase = ph;
            if (ph !== was) {   // conflict and gone are announced by the role=alert bar
                if (ph === 'retrying') { announce(t('homedash.save.announce_failed')); problem = true; }
                else if (ph === 'invalid') { announce(t('homedash.save.invalid', { reason: E.save.errorText(t, st.error) })); problem = true; }
                else if (ph === 'readonly') { announce(t('homedash.save.readonly')); problem = true; }
                else if (ph === 'conflict') { problem = true; if (!conflict) { conflict = { revision: st.currentRevision, savedBy: '', updatedAt: 0 }; openConflictOnce(); } }
                else if (ph === 'gone') { problem = true; if (!conflict || !conflict.gone) { conflict = { gone: true, revision: 0, savedBy: '', updatedAt: 0 }; openConflictOnce(); } }
                else if (ph === 'saved') { if (minePending) { minePending = false; toast(t('homedash.conflict.mine_done'), 'info'); } else if (problem) announce(t('homedash.save.announce_saved_again')); problem = false; }
            }
            if (ph === 'retrying' && !retryTimer) retryTimer = setT(tick, 1000);
            guard(() => { paintConflict(); emit(); });
        }
        function openConflictOnce() { if (!autoOpened) { autoOpened = true; resolveConflict(); } }
        async function resolveConflict() {
            if (!conflict || resolving || destroyed) return false; resolving = true;
            try {
                const c = await E.save.conflictSheet({ sheet, t, lang: o.lang, info: conflict }); if (destroyed) return false;
                if (c === 'theirs') await takeTheirs();
                else if (c === 'mine') { minePending = true; lane.overwrite(conflict.revision || lane.state().currentRevision); setConflict(null); }
                else if (c === 'copy') await saveCopy();
                return true;
            } finally { resolving = false; settleFocus(); }
        }
        async function fetchDoc() { const a = await o.api.dashboard(lane.id), d = a && a.dashboard; if (!d) throw new Error('no dashboard'); return d; }
        const normalise = d => E.doc.normalise(d, { sizeOf: catalog.defaultSize, constraints: catalog.constraints });
        async function takeTheirs() {
            try { const d = await fetchDoc(); if (!destroyed) replace(normalise(d), { revision: d.revision, clearHistory: true, toastKey: 'theirs_done' }); }
            catch (err) { announce(E.save.errorText(t, E.api.normalizeError(err))); }
        }
        async function saveCopy() {
            const suffix = t('homedash.conflict.copy_suffix'), name = Array.from(String(doc.name || '')).slice(0, Math.max(1, 80 - Array.from(suffix).length)).join('') + suffix;
            try { const a = await o.api.createDocument(Object.assign({}, E.doc.forSave(doc, { catalog }), { id: '', slug: '', name })), d = a && a.dashboard; if (d && d.id && typeof o.openCopy === 'function') o.openCopy(d.id, d.name || name); }
            catch (err) { announce(E.save.errorText(t, E.api.normalizeError(err))); }
        }
        // C7: a newer server revision of the open dashboard. Deferred while a PUT is open; our own save (revision <= lane.revision()) is ignored.
        async function reconcile(info) {
            if (destroyed || !info) return; if (lane.state().saving) { await lane.settle(0); if (destroyed) return; }
            if (info.gone) { gestures.cancel(); setConflict({ gone: true, revision: 0, savedBy: info.savedBy || '', updatedAt: info.updatedAt || 0 }); if (lane.state().dirty) lane.flush(); openConflictOnce(); return; }
            const rev = Number(info.revision); if (!(rev > lane.revision())) return; gestures.cancel();
            if (lane.state().dirty) { setConflict({ revision: rev, savedBy: info.savedBy || '', updatedAt: info.updatedAt || 0 }); lane.conflict(rev); return; }
            try { const d = await fetchDoc(); if (!destroyed && !lane.state().dirty) replace(normalise(d), { revision: d.revision, clearHistory: true, toastKey: info.savedBy === 'agent' ? 'reload_agent' : 'reload_other' }); } catch (err) { announce(E.save.errorText(t, E.api.normalizeError(err))); }
        }
        async function prepareExit() {
            gestures.cancel(); history.seal(); await lane.flush(); if (destroyed) return 'keep';
            const dr = E.doc.drafts(doc, catalog);
            if (dr.size) { const c = await E.save.draftsSheet({ sheet, t, count: dr.size }); if (c !== 'leave') { showDraft(); return 'keep'; } }
            const st = lane.state(); if (st.phase === 'saved' && !st.dirty) return 'leave';
            const c = await E.save.leaveSheet({ sheet, t, phase: st.phase, reason: st.error ? E.save.errorText(t, st.error) : '', mode: 'exit' }); return c === 'leave' ? 'leave' : c === 'discard' ? 'discard' : 'keep';
        }
        // ---- context menu, more menu, actions
        function registered(w) { const spec = catalog.widget(w.type), def = E.widgets && E.widgets.get ? E.widgets.get(w.type) : null; return (spec && spec.variants ? spec.variants.map(v => v.id) : []).filter(v => !def || def.variants.includes(v)); }
        const keep = fn => () => { const r = fn(); settleFocus(); return r; };
        function frameItems() {
            const one = selection.length === 1 ? widget(selection[0]) : null, list = one ? registered(one) : [];
            return [{ id: 'edit', label: t('homedash.ctx.edit'), icon: 'edit', shortcut: 'Enter', disabled: !inspector, run: keep(() => run('focus_inspector')) },
                { id: 'duplicate', label: t('homedash.ctx.duplicate'), icon: 'duplicate', shortcut: 'Ctrl+D', run: keep(() => run('duplicate')) }, { separator: true },
                { id: 'copy', label: t('homedash.ctx.copy'), icon: 'copy', shortcut: 'Ctrl+C', run: keep(() => run('copy')) }, { id: 'cut', label: t('homedash.ctx.cut'), icon: 'cut', shortcut: 'Ctrl+X', run: keep(() => run('cut')) }, { separator: true },
                { id: 'variant', label: t('homedash.ctx.variant'), icon: 'variant', disabled: !one || list.length < 2, items: one ? list.map(v => ({ id: 'variant_' + v, label: t('homedash.variant.' + one.type + '.' + v), icon: v === one.variant ? 'check' : undefined, checked: v === one.variant, run: keep(() => setVariant(one.id, v)) })) : [] },
                { separator: true }, { id: 'delete', label: t('homedash.ctx.delete'), icon: 'delete', shortcut: 'Del', danger: true, run: keep(() => run('delete')) }];
        }
        function contextMenu(ev) {
            const tg = ev && ev.target; if (!tg || typeof tg.closest !== 'function') return null;
            const at = () => { if (ev.clientX || ev.clientY) return { x: ev.clientX, y: ev.clientY }; const r = tg.getBoundingClientRect(); return { x: r.left + 8, y: r.top + 8 }; };
            const tab = tg.closest('.hd-ptab'); if (tab) { const items = E.ddialogs && E.ddialogs.pageMenu ? E.ddialogs.pageMenu(D, tab.getAttribute('data-page')) : null; return items && items.length ? Object.assign(at(), { items }) : null; }
            if (tg.closest('.hd-editbar') || tg.closest('.hd-palette') || tg.closest('.hd-inspector') || tg.closest('.hd-sheet-layer') || !tg.closest('.hd-stage')) return null;
            const fr = tg.closest(FRAME);
            if (fr) { const id = fr.getAttribute('data-hd-wid'); if (!selection.includes(id)) select([id], { focus: true }); return Object.assign(at(), { items: frameItems() }); }
            return Object.assign(at(), { items: [{ id: 'paste', label: t('homedash.ctx.paste'), icon: 'paste', shortcut: 'Ctrl+V', disabled: !(o.clipboard && o.clipboard.has()), run: keep(() => run('paste')) },
                { id: 'add_widget', label: t('homedash.ctx.add_widget'), icon: 'add_widget', disabled: !palette, run: keep(() => run('add_widget')) }] });
        }
        function moreItems() {
            const dd = !!E.ddialogs, ask = a => () => { o.request(a); }, out = [];
            if (dd) out.push({ id: 'rename', label: t('homedash.menu.rename'), icon: 'rename', run: ask('rename') });
            out.push({ id: 'duplicate_dashboard', label: t('homedash.menu.duplicate_dashboard'), icon: 'duplicate', run: ask('duplicate_dashboard') });
            if (dd) out.push({ id: 'import', label: t('homedash.menu.import'), run: ask('import') }, { id: 'export', label: t('homedash.menu.export'), run: ask('export') });
            out.push({ separator: true }, { id: 'delete_dashboard', label: t('homedash.menu.delete'), icon: 'delete', danger: true, run: ask('delete_dashboard') }); return out;
        }
        function run(action, arg) { const r = runAction(action, arg); settleFocus(); return r; }
        function runAction(action, arg) {
            switch (action) {
                case 'undo': case 'redo': return undoRedo(action);
                case 'copy': return copy(); case 'cut': return cut(); case 'paste': return paste(); case 'duplicate': return duplicate(); case 'delete': return del(); case 'select_all': return selectAll();
                case 'select_none': select([]); announce(t('homedash.designer.announce.selection_cleared')); return true;
                case 'reset_layout': return exec(C.resetLayout(doc, { pageId, bp, now: now() }));
                case 'retry_save': lane.retryNow(); return true; case 'resolve_conflict': resolveConflict(); return !!conflict; case 'show_draft': return showDraft();
                case 'add_widget': return openPanel('palette'); case 'focus_inspector': return openPanel('inspector'); case 'toggle_palette': return togglePanel('palette'); case 'toggle_inspector': return togglePanel('inspector');
                case 'layout_lg': case 'layout_md': case 'layout_sm': setBreakpoint(action.slice(7)); return true; case 'page_prev': return stepPage(-1); case 'page_next': return stepPage(1);
                default: return false;
            }
        }
        function command(name, arg, ev) {
            if (name === 'more') { const b = bar.el.querySelector('.hd-eb-more') || (ev && ev.target), r = b && typeof b.getBoundingClientRect === 'function' ? b.getBoundingClientRect() : { left: 0, bottom: 0 }; if (o.ui && typeof o.ui.menu === 'function') o.ui.menu(r.left, r.bottom, moreItems()); return; }
            if (name === 'page') { setPage(arg, { focus: false }); return; }
            if (!run(name, arg) && !ACTIONS.includes(name) && typeof o.request === 'function') o.request(name, arg);
        }
        // ---- keys (C14)
        function escapeLadder() {
            if (gestures.active()) { gestures.cancel(); announce(t('homedash.designer.announce.cancelled')); return true; }
            if (panel) { closePanels(); return true; }
            if (selection.length) { select([]); announce(t('homedash.designer.announce.selection_cleared')); return true; }
            if (typeof o.request === 'function') o.request('exit_edit'); return true;
        }
        function handle(k) {
            const f = focused();
            switch (k.action) {
                case 'undo': case 'redo': undoRedo(k.action); return true;
                case 'page_prev': return stepPage(-1); case 'page_next': return stepPage(1); case 'escape': return escapeLadder();
                case 'copy': return copy() || true; case 'cut': return cut() || true; case 'paste': return paste(); case 'duplicate': return duplicate(); case 'select_all': return selectAll();
                case 'toggle_focused': if (!f) return false; select(selection.includes(f) ? selection.filter(i => i !== f) : selection.concat(f), { focus: true }); return true;
                case 'select_focused': if (!f || !frameOfFocus()) return false; select([f], { focus: true }); return true;
                case 'nudge': return nudge(k.dx, k.dy); case 'resize_step': return resizeStep(k.dw, k.dh);
                case 'focus_dir': { if (f) { const to = C.spatial(items(), f, k.dir); if (to) focusFrame(to, true); } return true; }
                case 'delete': return del(); case 'focus_inspector': return inspector ? openPanel('inspector') : false;
                case 'focus_next': return tabMove(1); case 'focus_prev': return tabMove(-1);
                default: return false;
            }
        }
        function onKey(ev) {
            if (ev.defaultPrevented || destroyed) return; const tg = ev.target, closest = s => (tg && typeof tg.closest === 'function' ? tg.closest(s) : null);
            if (closest('.hd-sheet-layer')) return;
            const k = C.keymap(ev, { canvas: !!closest('.hd-stage'), typing: E.core.isTyping(tg) }); if (k && handle(k)) ev.preventDefault();
        }
        // ---- setup (F1): root attributes, chrome, view mode, then the gesture controller, then everything that reads it
        function teardown() {   // the destroy of D, also the rollback of a failed setup (the parts that exist so far are torn down)
            if (destroyed) return; if (gestures) gestures.cancel(); const pg = view.page(), order = pg ? C.readingOrder(items()) : []; destroyed = true; clearT(retryTimer); if (ro) ro.disconnect();
            for (const n of [palette, inspector]) if (n && typeof n.destroy === 'function') n.destroy();
            if (gestures) gestures.destroy(); bar.destroy(); sheet.destroy(); life.dispose(); for (const n of [cbar, scrim, shells.palette, shells.inspector]) if (n) n.remove();
            if (pg) for (const id of order) { const f = pg.frame(id); if (f) draftBadge(f, false); }
            for (const a of ['data-hd-drawers', 'data-hd-panel', 'data-hd-has-palette', 'data-hd-has-inspector', 'data-hd-gesture']) root.removeAttribute(a);
            view.setMode('view'); view.setDescribe(null); view.setEmptyAction(null); view.setBreakpoint(null); E.core.setAttr(root, 'data-hd-mode', 'view'); history.clear(); view.announce(t('homedash.designer.exit_announce'));
        }
        try {
        E.core.setAttr(root, 'data-hd-mode', 'design'); root.insertBefore(bar.el, root.children[0] || null); root.insertBefore(cbar, root.children[1] || null);
        for (const [name, cls] of [['palette', 'hd-palette'], ['inspector', 'hd-inspector']]) {
            if (!E[name]) continue; const label = name === 'palette' ? t('homedash.designer.palette') : t('homedash.designer.inspector');
            shells[name] = E.core.el('div', { class: cls, role: 'region', id: E.core.uid('hd-' + name), 'aria-label': label, 'data-hd-drawer': 'open' }); E.core.setAttr(root, 'data-hd-has-' + name, 'true');
            if (name === 'palette') root.insertBefore(shells[name], view.stage); else { const kids = Array.from(root.children); root.insertBefore(shells[name], kids[kids.indexOf(view.stage) + 1] || null); }
        }
        root.appendChild(scrim); view.setMode('design');
        gestures = E.gestures.create({ root, stage: view.stage, win: o.win || docEl().defaultView || docEl(), timers: tm, raf: o.raf, caf: o.caf, hold: on => lane.hold(on), announce, describeLimit, select: (ids, so) => select(ids, so),
            ctx: () => ({ page: view.page(), doc, pageId, bp, selection, items: items(), cols: E.layout.COLS[bp], compact: page().compact !== false, C: constraints, catalog, readonly: false, widget }),
            commit(kind, p) {
                if (kind === 'move' || kind === 'resize') {
                    const mine = p.next.filter(i => (kind === 'move' ? p.ids : [p.id]).includes(i.id)), box = mine.length ? E.grid.bbox(mine) : { x: 0, y: 0, w: 1, h: 1 };
                    exec(C.layout(doc, { pageId, bp: p.bp, next: p.next, C: constraints, ids: kind === 'move' ? p.ids : [p.id], kind, now: now(), label: kind === 'resize' ? 'homedash.cmd.resize' : 'homedash.cmd.move',
                        announce: kind === 'resize' ? { key: 'resized', w: box.w, h: box.h } : p.ids.length === 1 ? { key: 'moved', col: box.x + 1, row: box.y + 1 } : { key: 'moved_many', count: p.ids.length } }), { keepBox: true });
                } else if (kind === 'marquee') { select(p.ids, { announce: true }); }
                else if (kind === 'add') addWidgets(p.entries, { at: p.at, source: 'drop' });
                else if (kind === 'rebind') { if (exec(C.bind(doc, { pageId, id: p.id, binding: p.binding, entity: p.entity && p.entity.id, now: now() }), { quiet: true })) announce(t('homedash.designer.announce.rebound', { name: nameOf(widget(p.id)), entity: (p.entity && (p.entity.name || p.entity.id)) || '' })); }
            } });
        D = Object.freeze({
            state: currentState, on: bus.on, off: bus.off, doc: () => doc, exec, undo: () => undoRedo('undo'), redo: () => undoRedo('redo'), seal: () => history.seal(), replace, select, focusFrame, setPage, setBreakpoint, addWidgets, constraints, widget,
            slot: name => (name === 'palette' || name === 'inspector' ? shells[name] : bar.slot(name)), openPanel, closePanels, announce, toast, sheet: () => sheet, gestures, t, lang: o.lang, catalog, store: o.store, api: o.api, run, handles: a => ACTIONS.includes(a), contextMenu,
            reconcile, prepareExit, flush: ms => lane.flush({ keepalive: ms === 0 }),
            destroy: teardown
        });
        life.add(bus.on('state', s => { guard(() => bar.update(barState(s))); if (typeof o.onState === 'function') o.onState(); }));
        life.on(root, 'keydown', onKey); life.on(scrim, 'click', () => closePanels());
        life.on(view.stage, 'focusin', ev => { const w = ev.target && typeof ev.target.closest === 'function' ? ev.target.closest(FRAME) : null, id = w ? w.getAttribute('data-hd-wid') : null; if (id && id !== focusId) { focusId = id; sync(); } });
        life.add(lane.on('state', onLane)); life.add(view.on('page', () => { const np = view.pageId(); if (np !== pageId) { pageId = np; selection = []; focusId = null; } gestures.reset(); sync(); bus.emit('page', pageId); }));
        const RO = o.ResizeObserver || (typeof ResizeObserver === 'function' ? ResizeObserver : null); if (RO) { ro = new RO(() => measure()); ro.observe(root); } measure();
        if (E.palette && shells.palette) palette = E.palette.create({ host: shells.palette, designer: D, t, lang: o.lang, catalog, store: o.store, api: o.api });
        if (E.inspector && shells.inspector) inspector = E.inspector.create({ host: shells.inspector, designer: D, t, lang: o.lang, catalog, store: o.store, api: o.api });
        view.setDescribe(describe); view.setEmptyAction({ cmd: 'add_widget', label: t('homedash.designer.add_widget') }); view.setBreakpoint('lg'); applyPanels(); paintConflict(); render();
        if (o.widgetId && widget(o.widgetId)) select([o.widgetId], { focus: true }); else { const first = C.readingOrder(items())[0]; focusFrame(first || null, false); }
        announce(t('homedash.designer.enter_announce')); onLane(lane.state());
        if (E.ddialogs && typeof E.ddialogs.enhanceTabs === 'function') E.ddialogs.enhanceTabs(D, bar.slot('pages'));
        } catch (err) { teardown(); throw err; }
        return D;
    }
    E.designer = Object.freeze({ create, ACTIONS, DRAWER_BELOW });
})();
