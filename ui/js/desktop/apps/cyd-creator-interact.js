// CYD Creator designer interaction: select (click, Shift/Ctrl toggle, box select), move with grid
// snapping and alignment guides, reparent on drop into a panel or the visible tabview page, 8-handle
// resize, drag from the palette, tab switching on tabview headers and the keyboard model (nudge,
// delete, clipboard, duplicate, undo/redo, zoom). Gestures preview in the DOM and commit one
// layout change on release; a cancelled gesture re-renders the stored layout.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const THRESHOLD = 3;
    const GUIDE_PX = 6;
    const PAGE_KEYS = new Set(['Enter', 'Delete', 'Backspace', 'F2']);

    // capture follows one pointer on target (pattern: easydrag-core capturePointer). onEnd(event,
    // cancelled) runs once: pointerup (not cancelled), pointercancel, lost capture, a mouse move with
    // no button down, or abort() (cancelled). Every end releases the pointer capture.
    function capture(target, event, onMove, onEnd) {
        const id = event.pointerId;
        const types = ['pointerup', 'pointercancel', 'lostpointercapture'];
        let done = false;
        const move = ev => {
            if (ev.pointerId !== id) return;
            if (ev.pointerType === 'mouse' && ev.buttons === 0) finish(ev, true);
            else onMove(ev);
        };
        const end = ev => { if (ev.pointerId === id) finish(ev, ev.type !== 'pointerup'); };
        function finish(ev, cancelled) {
            if (done) return;
            done = true;
            target.removeEventListener('pointermove', move);
            types.forEach(type => target.removeEventListener(type, end));
            try { if (target.hasPointerCapture && target.hasPointerCapture(id)) target.releasePointerCapture(id); } catch (err) { /* released */ }
            onEnd(ev, cancelled);
        }
        try { target.setPointerCapture(id); } catch (err) { /* synthetic pointer */ }
        target.addEventListener('pointermove', move);
        types.forEach(type => target.addEventListener(type, end));
        return { abort() { finish({ type: 'abort', pointerId: id }, true); } };
    }

    function isField(el) { return !!(el && el.closest && el.closest('input, textarea, select, [contenteditable]:not([contenteditable="false"])')); }

    function attach(d, canvas) {
        const M = P.model;
        const G = P.geometry;
        const t = d.t;
        const bag = P.util.bag();
        let gesture = null;
        const locked = () => d.readonly() || d.isSystem();
        const radius = () => Math.max(2, GUIDE_PX / (d.zoom || 1));
        const far = (a, ev) => Math.hypot(ev.clientX - a.x, ev.clientY - a.y) >= THRESHOLD;
        // world is a pointer's unrounded device position: measured against the screen host, so a
        // viewport scrolled during a gesture keeps the widget under the pointer.
        function world(ev) {
            const r = canvas.hostRect();
            const z = d.zoom || 1;
            return { x: (ev.clientX - r.left) / z, y: (ev.clientY - r.top) / z };
        }
        // worldDelta is the whole device-pixel distance between two world positions.
        const worldDelta = (a, b) => ({ x: Math.round(b.x - a.x), y: Math.round(b.y - a.y) });

        function visibleIn(parent, w) {
            return !w.hidden && (!parent || parent.type !== 'tabview' || (w.tab || 0) === d.activeTab(parent.id));
        }
        // siblingRects lists the absolute rects of the visible siblings a moving widget snaps to.
        function siblingRects(found, exclude) {
            const p = d.project();
            const env = d.env();
            const screen = d.screen();
            const list = found.parent ? (found.parent.children || []) : ((screen && screen.widgets) || []);
            return list.filter(w => visibleIn(found.parent, w) && !exclude.has(w.id)).map(w => M.absRect(p, w.id, env));
        }
        // containerAt returns the top-most visible container whose content box holds point (screen
        // coordinates), skipping the widgets in exclude and their subtrees.
        function containerAt(point, exclude) {
            const screen = d.screen();
            if (!screen) return null;
            let best = null;
            (function visit(list, parent, ox, oy) {
                list.forEach(w => {
                    if (exclude.has(w.id) || !visibleIn(parent, w)) return;
                    const spec = d.spec(w.type);
                    if (!spec || !spec.container) return;
                    const box = M.contentBox(w, spec);
                    const r = { x: ox + w.x + box.x, y: oy + w.y + box.y, w: box.w, h: box.h };
                    if (!G.contains(r, point)) return;
                    best = { widget: w, rect: r, tab: w.type === 'tabview' ? d.activeTab(w.id) : undefined };
                    visit(w.children || [], w, r.x, r.y);
                });
            })(screen.widgets || [], null, 0, 0);
            return best;
        }

        // ── move ────────────────────────────────────────────────────────────
        function startMove(event, id) {
            const p = d.project();
            const env = d.env();
            const ids = M.topLevel(p, d.selection).filter(x => { const f = d.find(x); return f && !f.widget.locked; });
            if (!ids.length) return;
            const found = d.find(ids.includes(id) ? id : ids[0]);
            const moving = new Set(ids);
            const abs0 = G.bounds(ids.map(x => M.absRect(p, x, env)));
            const frame = d.frameOf(found);
            const targets = G.guideTargets(siblingRects(found, moving), frame);
            const parentId = found.parent ? found.parent.id : '';
            const start = { x: event.clientX, y: event.clientY };
            const from = world(event);
            let moved = false;
            let delta = { x: 0, y: 0 };
            let drop = null;
            gesture = capture(canvas.viewport, event, ev => {
                if (!moved && !far(start, ev)) return;
                if (!moved) { moved = true; canvas.root.classList.add('is-dragging'); }
                const step = worldDelta(from, world(ev));
                const raw = G.translate(abs0, step.x, step.y);
                const snapped = ev.altKey ? Object.assign({ guides: { x: [], y: [] } }, raw) : G.snapMove(raw, targets, { step: d.snap, radius: radius(), origin: frame });
                delta = { x: snapped.x - abs0.x, y: snapped.y - abs0.y };
                canvas.previewMove(ids, delta);
                canvas.showGuides(snapped.guides);
                drop = containerAt(canvas.clientToWorld(ev.clientX, ev.clientY), moving);
                canvas.showDrop(drop && drop.widget.id !== parentId ? drop.rect : null);
            }, (ev, cancelled) => {
                gesture = null;
                canvas.root.classList.remove('is-dragging');
                canvas.showGuides(null);
                canvas.showDrop(null);
                if (!moved) return;
                if (cancelled) { canvas.render(); return; }
                commitMove(ids, delta, drop, parentId);
            });
        }

        // commitMove saves a move as one undo step. Dropped where the pressed widget already lives
        // (parentId), every widget moves in place, so a selection spanning parents keeps them;
        // dropped elsewhere, every widget goes into the drop target (or the screen root).
        function commitMove(ids, delta, drop, parentId) {
            const p = d.project();
            const env = d.env();
            const screen = d.screen();
            const dropId = drop ? drop.widget.id : '';
            const inPlace = dropId === parentId;
            // Layout ops take integers only (the server rejects anything else): every coordinate is rounded.
            const ops = ids.map(id => {
                const f = d.find(id);
                if (inPlace || (f.parent ? f.parent.id : '') === dropId) return { op: 'move_widget', id, x: Math.round(f.widget.x + delta.x), y: Math.round(f.widget.y + delta.y) };
                const abs = G.translate(M.absRect(p, id, env), delta.x, delta.y);
                const origin = drop ? drop.rect : { x: 0, y: 0 };
                const op = { op: 'move_widget', id, screen: screen.id, x: Math.round(abs.x - origin.x), y: Math.round(abs.y - origin.y) };
                if (drop) { op.parent = dropId; if (drop.widget.type === 'tabview') op.tab = drop.tab; }
                return op;
            });
            const reparent = ops.some(op => op.screen);
            // A refused change (read-only, an impossible op) leaves the stored layout: drop the preview.
            if (!d.commands.run(reparent ? 'reparent' : 'move', ops)) { canvas.render(); return; }
            if (reparent) d.announce(drop ? t('cyd_creator.designer.moved_into', { name: drop.widget.name || d.label(drop.widget.type) }) : t('cyd_creator.designer.moved_to_screen'));
        }

        // ── resize ──────────────────────────────────────────────────────────
        function startResize(event, handle) {
            const id = d.selection.length === 1 ? d.selection[0] : '';
            const found = id ? d.find(id) : null;
            if (!found || found.widget.locked) return;
            // Not stopped: the window manager still sees the press and raises the window.
            event.preventDefault();
            const abs0 = M.absRect(d.project(), id, d.env());
            const frame = d.frameOf(found);
            const targets = G.guideTargets(siblingRects(found, new Set([id])), frame);
            const start = { x: event.clientX, y: event.clientY };
            const from = world(event);
            let last = abs0;
            let moved = false;
            canvas.root.classList.add('is-resizing');
            gesture = capture(canvas.viewport, event, ev => {
                // A click on a handle is no resize: the edges move only after a real drag.
                if (!moved && !far(start, ev)) return;
                moved = true;
                const step = worldDelta(from, world(ev));
                let r = G.resizeRect(abs0, handle, step.x, step.y, { keepRatio: ev.shiftKey });
                if (!ev.altKey && !ev.shiftKey) {
                    const s = G.snapResize(r, handle, targets, { step: d.snap, radius: radius(), origin: frame });
                    canvas.showGuides(s.guides);
                    r = { x: s.x, y: s.y, w: s.w, h: s.h };
                } else canvas.showGuides(null);
                last = r;
                canvas.previewResize(id, r);
            }, (ev, cancelled) => {
                gesture = null;
                canvas.root.classList.remove('is-resizing');
                canvas.showGuides(null);
                if (!moved) return;
                if (cancelled) { canvas.render(); return; }
                const o = M.absOrigin(d.project(), id, d.env());
                const set = { x: Math.round(last.x - o.x), y: Math.round(last.y - o.y), w: Math.round(last.w), h: Math.round(last.h) };
                if (!d.commands.run('resize', [{ op: 'update_widget', id, set }])) canvas.render();
            });
        }

        // ── box select ──────────────────────────────────────────────────────
        function startBox(event) {
            const additive = event.shiftKey || event.ctrlKey || event.metaKey;
            const base = additive ? d.selection.slice() : [];
            const start = { x: event.clientX, y: event.clientY };
            const from = canvas.clientToWorld(event.clientX, event.clientY);
            let active = false;
            gesture = capture(canvas.viewport, event, ev => {
                if (!active && !far(start, ev)) return;
                active = true;
                const box = G.normalizeRect(from, canvas.clientToWorld(ev.clientX, ev.clientY));
                canvas.showMarquee(box);
                const screen = d.screen();
                const items = ((screen && screen.widgets) || []).filter(w => !w.hidden).map(w => ({ id: w.id, rect: { x: w.x, y: w.y, w: w.w, h: w.h } }));
                const hits = G.inBox(items, box);
                d.select(base.concat(hits.filter(id => !base.includes(id))));
            }, () => {
                gesture = null;
                canvas.showMarquee(null);
                if (!active && !additive) d.select([]);
            });
        }

        function onPointerDown(event) {
            if (event.button !== 0 || gesture) return;
            const target = event.target;
            const vp = canvas.viewport;
            // A press on the viewport's own scrollbars scrolls; it neither selects nor clears.
            if (target === vp && (event.offsetX >= vp.clientWidth || event.offsetY >= vp.clientHeight)) return;
            canvas.root.focus({ preventScroll: true });
            const handle = target.closest('[data-cc-handle]');
            if (handle) { if (!locked()) startResize(event, handle.dataset.ccHandle); return; }
            if (d.isSystem()) return;
            const wEl = target.closest('[data-cc-widget-id]');
            const tab = target.closest('[data-cc-tab]');
            if (wEl && tab && wEl.contains(tab)) {
                const id = wEl.dataset.ccWidgetId;
                d.setTab(id, Number(tab.dataset.ccTab) || 0);
                d.select([id]);
                event.preventDefault();
                return;
            }
            if (wEl) {
                const id = wEl.dataset.ccWidgetId;
                event.preventDefault();
                if (event.shiftKey || event.ctrlKey || event.metaKey) { d.select([id], { toggle: true }); return; }
                if (!d.selection.includes(id)) d.select([id]);
                if (!locked()) startMove(event, id);
                return;
            }
            event.preventDefault();
            startBox(event);
        }

        // ── palette drag ────────────────────────────────────────────────────
        // ghostFor renders the new widget (catalog defaults from P.model.newWidget) at the zoom.
        function ghostFor(spec, sample) {
            const scale = d.zoom || 1;
            const theme = d.theme();
            const screen = d.screen();
            const ghost = document.createElement('div');
            ghost.className = 'cc-drag-ghost';
            ghost.style.width = sample.w * scale + 'px';
            ghost.style.height = sample.h * scale + 'px';
            const inner = document.createElement('div');
            inner.className = 'cc-drag-ghost-inner';
            inner.style.width = sample.w + 'px';
            inner.style.height = sample.h + 'px';
            inner.style.transform = 'scale(' + scale + ')';
            inner.style.fontFamily = P.render.fontFamily(theme.font);
            inner.style.fontSize = (theme.font_size || 16) + 'px';
            const opts = { activeTab: 0, screenTitle: screen ? screen.title : '', settings: d.project().settings };
            inner.appendChild(P.render.widget(spec, Object.assign({}, sample, { x: 0, y: 0 }), theme, d.inst.assetUrl, opts));
            ghost.appendChild(inner);
            return ghost;
        }

        // startPaletteDrag follows a pointer pressed on a palette entry: dropping on the visible
        // screen adds the widget there (into the container under the pointer); a press without
        // movement adds it in the middle of the screen. The widget itself is built by the add
        // command (P.model.newWidget). The pointer is captured on the canvas viewport, so a palette
        // that re-renders during the drag does not end it. The canvas takes the focus at the press:
        // browsers that do not focus a pressed button (Safari, Firefox on macOS) would leave it
        // elsewhere, and Escape must reach gestureKey to cancel the drag.
        function startPaletteDrag(type, event) {
            const spec = d.spec(type);
            if (!spec || locked() || event.button !== 0 || gesture) return;
            event.preventDefault();
            canvas.root.focus({ preventScroll: true });
            const start = { x: event.clientX, y: event.clientY };
            const sample = M.newWidget(d.project(), spec, { x: 0, y: 0 }, d.screenSize());
            let moved = false;
            let ghost = null;
            let point = null;
            let drop = null;
            gesture = capture(canvas.viewport, event, ev => {
                if (!moved && !far(start, ev)) return;
                if (!moved) { moved = true; ghost = ghostFor(spec, sample); document.body.appendChild(ghost); canvas.root.classList.add('is-dropping'); }
                const scale = d.zoom || 1;
                ghost.style.transform = 'translate(' + Math.round(ev.clientX - sample.w * scale / 2) + 'px,' + Math.round(ev.clientY - sample.h * scale / 2) + 'px)';
                // Only the part of the screen the viewport shows (it clips a zoomed screen) takes drops.
                const r = canvas.hostRect();
                const v = canvas.viewport.getBoundingClientRect();
                const inside = ev.clientX >= Math.max(r.left, v.left) && ev.clientX < Math.min(r.right, v.right) && ev.clientY >= Math.max(r.top, v.top) && ev.clientY < Math.min(r.bottom, v.bottom);
                ghost.classList.toggle('is-outside', !inside);
                point = inside ? canvas.clientToWorld(ev.clientX, ev.clientY) : null;
                drop = point ? containerAt(point, new Set()) : null;
                canvas.showDrop(drop ? drop.rect : null);
            }, (ev, cancelled) => {
                gesture = null;
                if (ghost) ghost.remove();
                canvas.root.classList.remove('is-dropping');
                canvas.showDrop(null);
                if (cancelled) return;
                if (!moved) { d.commands.addAtCenter(type); canvas.root.focus({ preventScroll: true }); return; }
                if (point) { d.commands.addWidget(type, point, drop ? { id: drop.widget.id, tab: drop.tab } : null); canvas.root.focus({ preventScroll: true }); }
            });
        }

        // ── keyboard ────────────────────────────────────────────────────────
        // shortcut maps a key to the designer command it runs, or null for the keys the designer
        // leaves alone: Ctrl/Meta+S (the shell saves), Alt combinations (the desktop), Tab, typing.
        // Ctrl/Meta shortcuts take no Alt and Shift only for redo (Ctrl+Shift+Z).
        function shortcut(event) {
            const key = event.key;
            const c = d.commands;
            const lower = key.length === 1 ? key.toLowerCase() : key;
            if (event.ctrlKey || event.metaKey) {
                if (event.altKey) return null;
                if (lower === 'z') return event.shiftKey ? c.redo : c.undo;
                if (event.shiftKey) return null;
                return { y: c.redo, c: c.copy, x: c.cut, v: c.paste, d: c.duplicate, a: c.selectAll }[lower] || null;
            }
            if (event.altKey) return null;
            if (key === 'Delete' || key === 'Backspace') return c.remove;
            // The desktop opens (Enter) or renames (F2) its selected icons from these keys; on the
            // canvas they do nothing.
            if ((key === 'Enter' || key === 'F2') && event.target === canvas.root) return () => {};
            if (key === 'Escape') return d.selection.length ? () => d.select([]) : null;
            // Arrows nudge 1 px (Shift: 8 px); the store joins a burst into one undo step.
            const delta = G.nudgeDelta(key, event.shiftKey);
            if (delta) return d.selection.length || gesture ? () => c.nudge(delta.x, delta.y) : null;
            if (key === '+' || key === '=') return () => d.setZoom((d.zoom || 1) + 1);
            if (key === '-') return () => d.setZoom((d.zoom || 1) - 1);
            if (key === '0') return () => canvas.fit(true);
            return null;
        }

        // gestureKey handles a key while a gesture runs (false otherwise): Escape cancels it, the
        // designer's other keys are used without running (so neither the gesture's preview nor the
        // desktop sees them), anything else stays native. The palette forwards its keys here.
        function gestureKey(event) {
            if (!gesture || event.defaultPrevented || typeof event.key !== 'string') return false;
            if (event.key === 'Escape') { gesture.abort(); return true; }
            return !isField(event.target) && !!shortcut(event);
        }

        // handleKey runs a designer shortcut; it returns true when the key was used. It gets only the
        // keys of the canvas (its listener) and of the designer panels that pass theirs on, never
        // those of the code editor, dialogs or other windows. Form fields and Space/Enter on buttons
        // keep their native behaviour; nothing runs while the canvas is hidden (Code mode).
        function handleKey(event) {
            if (event.defaultPrevented || typeof event.key !== 'string') return false;
            if (gesture) return gestureKey(event);
            if (isField(event.target) || !canvas.viewport.clientWidth) return false;
            const target = event.target;
            if ((event.key === ' ' || event.key === 'Enter') && target && target.closest && target.closest('button')) return false;
            const run = shortcut(event);
            if (!run) return false;
            run();
            return true;
        }

        bag.on(canvas.viewport, 'pointerdown', onPointerDown);
        // A used key stops here, so desktop-wide shortcuts (Delete on a selected desktop icon) do
        // not run as well; keys the designer does not use (Ctrl+S, Tab) bubble on. Enter, Delete,
        // Backspace and F2 never leave the canvas, its toolbar included (as in the palette and the
        // screens panel): the desktop opens, deletes or renames its selected icons from them. A
        // toolbar button keeps its native Enter click (no preventDefault); form fields (the snap
        // select) keep theirs and the desktop ignores keys from fields.
        bag.on(canvas.root, 'keydown', event => {
            if (handleKey(event)) { event.preventDefault(); event.stopPropagation(); return; }
            if (PAGE_KEYS.has(event.key) && !isField(event.target)) event.stopPropagation();
        });
        bag.on(window, 'blur', () => { if (gesture) gesture.abort(); });
        bag.add(d.bus.on('project', info => { if (gesture && info && info.origin !== 'local') gesture.abort(); }));
        bag.add(d.bus.on('screen', () => { if (gesture) gesture.abort(); }));
        // Dispose cancels a gesture in progress (a drag snaps back, a palette ghost disappears).
        bag.add(() => { if (gesture) gesture.abort(); });

        return {
            startPaletteDrag, handleKey, gestureKey, containerAt,
            abort() { if (gesture) gesture.abort(); },
            dispose() { bag.dispose(); }
        };
    }

    P.designerInteract = { attach, capture };
})();
