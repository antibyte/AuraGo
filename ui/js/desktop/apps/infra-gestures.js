// Infra gestures: pointer, wheel and keyboard behaviour of the canvas and the
// drag source for the library. It plans every layout change as before/after
// intents (drop slot, drop parent, region growth) and hands them to the store
// as commands; the canvas only shows temporary positions while a gesture runs.
// Module routing: infra.js binds onRootKeydown on the app root and calls
// attach() once; the library calls startLibraryDrag, the inspector and the
// library call placeAtCenter, the inspector calls arrange.
(function () {
    'use strict';

    const THRESHOLD = 4, LIBRARY_THRESHOLD = 5, PAN_THRESHOLD = 3;
    const ARROWS = { ArrowLeft: 'left', ArrowRight: 'right', ArrowUp: 'up', ArrowDown: 'down' };
    // frame coalesces many requests into one requestAnimationFrame callback.
    const frame = fn => {
        let id = 0;
        return {
            request() { if (!id) id = requestAnimationFrame(() => { id = 0; fn(); }); },
            cancel() { if (id) cancelAnimationFrame(id); id = 0; },
        };
    };
    // capturePointer follows one pointer on target. onEnd(event, cancelled) runs once: on pointerup
    // (not cancelled), or on pointercancel, lost capture or a mouse move with no button down, which
    // means the release was missed (cancelled). abort() ends it as cancelled; detach() ends it silently.
    function capturePointer(target, event, onMove, onEnd) {
        const id = event.pointerId;
        const types = ['pointerup', 'pointercancel', 'lostpointercapture'];
        let done = false;
        const move = ev => {
            if (ev.pointerId !== id) return;
            if (ev.pointerType === 'mouse' && ev.buttons === 0) finish(ev, true);
            else onMove(ev);
        };
        const end = ev => {
            if (ev.pointerId !== id) return;
            // A mouse that loses capture with its button still down (the browser dropped it, or the
            // capturing element was re-rendered) keeps going: pointerup or a move without buttons ends it.
            if (ev.type === 'lostpointercapture' && ev.pointerType === 'mouse' && ev.buttons !== 0) return;
            finish(ev, ev.type !== 'pointerup');
        };
        function detach() {
            done = true;
            target.removeEventListener('pointermove', move);
            types.forEach(type => target.removeEventListener(type, end));
        }
        function finish(ev, cancelled) {
            if (done) return;
            detach();
            onEnd(ev, cancelled);
        }
        try { target.setPointerCapture(id); } catch (err) { /* the pointer is already released */ }
        target.addEventListener('pointermove', move);
        types.forEach(type => target.addEventListener(type, end));
        return {
            abort() { finish({ type: 'abort', pointerId: id, clientX: event.clientX, clientY: event.clientY }, true); },
            detach
        };
    }
    const isTyping = target => !!target?.closest?.('input, select, textarea, [contenteditable=""], [contenteditable="true"]');
    const isMod = event => event.ctrlKey || event.metaKey;
    const readingOrder = cards => cards.slice().sort((a, b) => a.y - b.y || a.x - b.x);

    function create(app) {
        const { store, state, t, model, root } = app;
        const canvas = app.canvas, viewport = canvas.viewport;
        const touches = new Map();
        let gesture = null, moveMode = null, pinch = null, spaceDown = false, spaceUsed = false, suppressClick = false, suppressTimer = 0, attached = false;

        const nameOf = id => store.nodeById(id)?.name || id;
        const commands = () => window.InfraAppStore.commands;
        const movedLabel = count => (count === 1 ? t('desktop.infra_moved_one') : t('desktop.infra_moved').replace('{count}', count));
        function holdClick() {
            suppressClick = true;
            clearTimeout(suppressTimer);
            suppressTimer = setTimeout(() => { suppressClick = false; }, 0);
        }
        function begin(event, onMove, onEnd) {
            const cap = capturePointer(viewport, event, onMove, (ev, cancelled) => { gesture = null; onEnd(ev, cancelled); });
            gesture = cap;
            return cap;
        }
        // A captured pointer retargets the click that follows to the capturing element (the
        // viewport), so clicks and double clicks look for the item under the pointer instead.
        function clickTarget(event) {
            const target = event.target;
            if (!(target instanceof Element) || target !== viewport || event.detail === 0) return target instanceof Element ? target : null;
            const hit = document.elementFromPoint(event.clientX, event.clientY);
            return hit && viewport.contains(hit) ? hit : target;
        }

        // ---- drop planning ----------------------------------------------------------------
        // planDrop turns wanted card centres into before/after intents: optional
        // overlap nudge, drop parent (Map view, server support) and region growth.
        function planDrop(view, wants, { nudge = true } = {}) {
            const scene = canvas.sceneData();
            const parentAware = view === 'atlas' && store.features.layoutParent;
            const moving = new Set(wants.keys());
            const obstacles = scene.cards.filter(card => !moving.has(card.id)).map(model.cardRect);
            const before = new Map(), after = new Map(), moves = [];
            const placementOf = id => after.get(id) || store.effectivePlacement(id, view);
            const ids = new Set(scene.regionRects.keys());
            let hitRegion = '';
            for (const [id, want] of wants) {
                const pos = nudge ? model.freeSlot(want, obstacles) : { x: want.x, y: want.y };
                obstacles.push(model.cardRect(pos));
                const current = store.effectiveParent(id, view);
                const parent = view === 'atlas' ? model.dropParent(current, pos, scene.regionRects, scene.regionDepth) : {};
                if (parent.parent_id) hitRegion = parent.parent_id;
                const parentChange = parentAware && 'parent_id' in parent ? parent : {};
                after.set(id, { x: Math.round(pos.x), y: Math.round(pos.y), pinned: true, ...parentChange });
                before.set(id, store.currentIntent(id, view, 'parent_id' in parentChange));
                moves.push({ id, from: current, to: 'parent_id' in parentChange ? parentChange.parent_id : current });
                const target = 'parent_id' in parentChange ? parentChange.parent_id : current;
                if (!parentAware || !target || !ids.has(target)) continue;
                for (const [regionId, rect] of model.growRegions(target, model.cardRect(pos), { ids, parentOf: rid => store.effectiveParent(rid, view), placementOf })) {
                    if (!before.has(regionId)) before.set(regionId, store.currentIntent(regionId, view, false));
                    after.set(regionId, { ...(after.get(regionId) || {}), x: Math.round(rect.x), y: Math.round(rect.y), w: Math.round(rect.w), h: Math.round(rect.h), pinned: true });
                }
            }
            return { before, after, moves, hitRegion, parentAware };
        }
        const unchanged = plan => [...plan.after].every(([id, next]) => { const prev = plan.before.get(id); return !!prev && Object.keys(next).every(key => prev[key] === next[key]); });
        function commitPlan(view, plan, kind) {
            if (kind !== 'place' && unchanged(plan)) return;
            const name = id => store.nodeById(id)?.name || id;
            const changed = plan.moves.filter(move => move.to !== move.from);
            const label = kind === 'place'
                ? (plan.moves.length === 1 ? t('desktop.infra_placed').replace('{name}', name(plan.moves[0].id)) : t('desktop.infra_placed_many').replace('{count}', plan.moves.length))
                : changed.length === 1 && plan.moves.length === 1
                    ? (changed[0].to ? t('desktop.infra_moved_into').replace('{name}', name(changed[0].id)).replace('{place}', name(changed[0].to)) : t('desktop.infra_moved_out').replace('{name}', name(changed[0].id)))
                    : movedLabel(plan.moves.length);
            const { commands: factory } = window.InfraAppStore;
            const command = kind === 'place' ? factory.placeFromLibrary(view, plan.after, label)
                : changed.length ? factory.reparent(view, plan.before, plan.after, label) : factory.moveNodes(view, plan.before, plan.after, label);
            void store.dispatch(command);
            app.announce(label);
            if (plan.hitRegion && !plan.parentAware) store.notify(t('desktop.infra_parent_unsupported'), 'info');
        }
        // subtree collects a region with every drawn descendant region and card.
        function subtree(regionId, view, scene) {
            const regions = new Set([regionId]), cards = new Set(), queue = [regionId];
            while (queue.length) {
                const parent = queue.shift();
                for (const region of scene.regions) if (!regions.has(region.id) && store.effectiveParent(region.id, view) === parent) { regions.add(region.id); queue.push(region.id); }
                for (const card of scene.cards) if (store.effectiveParent(card.id, view) === parent) cards.add(card.id);
            }
            return { regions, cards };
        }
        // planRegionMove shifts a region with everything inside it by delta (world px).
        function planRegionMove(regionId, delta, view, scene, family) {
            const before = new Map(), after = new Map();
            for (const id of [...family.regions, ...family.cards]) {
                const cur = store.currentIntent(id, view, false);
                if (!cur) continue;
                before.set(id, cur);
                after.set(id, { ...cur, x: cur.x + delta.x, y: cur.y + delta.y, pinned: true });
            }
            const rect = scene.regionRects.get(regionId);
            const moved = { x: rect.x + delta.x, y: rect.y + delta.y, w: rect.w, h: rect.h };
            let changed = false;
            if (store.features.layoutParent && rect) {
                const result = model.dropParent(store.effectiveParent(regionId, view), { x: moved.x + 20, y: moved.y + 18 }, scene.regionRects, scene.regionDepth, family.regions);
                if ('parent_id' in result) {
                    changed = true;
                    after.set(regionId, { ...after.get(regionId), parent_id: result.parent_id });
                    before.set(regionId, store.currentIntent(regionId, view, true));
                    const ids = new Set(scene.regionRects.keys());
                    const placementOf = id => after.get(id) || store.effectivePlacement(id, view);
                    for (const [id, grown] of result.parent_id ? model.growRegions(result.parent_id, moved, { ids, parentOf: rid => store.effectiveParent(rid, view), placementOf }) : []) {
                        if (!before.has(id)) before.set(id, store.currentIntent(id, view, false));
                        after.set(id, { ...(after.get(id) || {}), x: Math.round(grown.x), y: Math.round(grown.y), w: Math.round(grown.w), h: Math.round(grown.h), pinned: true });
                    }
                }
            }
            return { before, after, changed, to: changed ? after.get(regionId).parent_id : store.effectiveParent(regionId, view) };
        }
        // commitRegionDelta saves a place moved by delta together with everything inside it.
        function commitRegionDelta(regionId, delta, view, scene, family) {
            const plan = planRegionMove(regionId, delta, view, scene, family);
            const label = plan.changed
                ? (plan.to ? t('desktop.infra_moved_into').replace('{name}', nameOf(regionId)).replace('{place}', nameOf(plan.to)) : t('desktop.infra_moved_out').replace('{name}', nameOf(regionId)))
                : movedLabel(plan.after.size);
            void store.dispatch(plan.changed ? commands().reparent(view, plan.before, plan.after, label) : commands().moveNodes(view, plan.before, plan.after, label));
            app.announce(label);
        }
        // resizeRect applies a handle drag to the start rectangle: edges snap to the
        // grid and the minimum size holds with the opposite edge fixed.
        function resizeRect(handle, start, dx, dy) {
            let left = start.x, top = start.y, right = start.x + start.w, bottom = start.y + start.h;
            if (handle.includes('w')) left = model.snap(left + dx);
            if (handle.includes('e')) right = model.snap(right + dx);
            if (handle.includes('n')) top = model.snap(top + dy);
            if (handle.includes('s')) bottom = model.snap(bottom + dy);
            if (right - left < model.REGION_MIN_W) { if (handle.includes('w')) left = right - model.REGION_MIN_W; else right = left + model.REGION_MIN_W; }
            if (bottom - top < model.REGION_MIN_H) { if (handle.includes('n')) top = bottom - model.REGION_MIN_H; else bottom = top + model.REGION_MIN_H; }
            return { x: left, y: top, w: right - left, h: bottom - top };
        }

        // ---- gestures on the canvas ------------------------------------------------------------
        function startPan(event, clearOnClick) {
            const origin = { ...canvas.getView() }, sx = event.clientX, sy = event.clientY;
            let moved = false;
            if (spaceDown) spaceUsed = true;
            begin(event, ev => {
                const dx = ev.clientX - sx, dy = ev.clientY - sy;
                if (!moved && Math.hypot(dx, dy) < PAN_THRESHOLD) return;
                moved = true;
                viewport.classList.add('is-panning');
                canvas.setView({ x: origin.x + dx, y: origin.y + dy, zoom: origin.zoom });
            }, (ev, cancelled) => {
                viewport.classList.remove('is-panning');
                if (moved) holdClick();
                else if (clearOnClick && !cancelled) store.clearSelection();
            });
        }
        function startMarquee(event) {
            const rect = viewport.getBoundingClientRect(), base = isMod(event) ? new Set(state.selection) : new Set();
            const sx = event.clientX, sy = event.clientY;
            let started = false, lastKey = null;
            const update = ev => {
                const a = { x: sx - rect.left, y: sy - rect.top }, b = { x: ev.clientX - rect.left, y: ev.clientY - rect.top };
                canvas.showMarquee(model.normalizeRect(a, b));
                const view = canvas.getView();
                const box = model.normalizeRect(model.toWorld(view, a), model.toWorld(view, b));
                const hits = canvas.sceneData().cards.filter(card => model.intersects(model.cardRect(card), box)).map(card => card.id);
                const next = [...base, ...hits.filter(id => !base.has(id))], key = next.join('\x1f');
                if (key === lastKey) return;
                lastKey = key;
                store.select(next);
            };
            begin(event, ev => {
                if (!started && Math.hypot(ev.clientX - sx, ev.clientY - sy) < THRESHOLD) return;
                started = true;
                update(ev);
            }, (ev, cancelled) => {
                canvas.showMarquee(null);
                if (!started) return;
                if (cancelled) store.select([...base]);
                holdClick();
            });
        }
        function startCardDrag(event, id) {
            const view = app.mapView(), scene = canvas.sceneData(), zoom = canvas.getView().zoom, sx = event.clientX, sy = event.clientY;
            let started = false, ids = [], positions = new Map();
            const origin = new Map();
            const compute = ev => {
                const o = origin.get(id);
                const dx = model.snap(o.x + (ev.clientX - sx) / zoom) - o.x, dy = model.snap(o.y + (ev.clientY - sy) / zoom) - o.y;
                positions = new Map(ids.map(cid => [cid, { x: origin.get(cid).x + dx, y: origin.get(cid).y + dy }]));
            };
            const paint = frame(() => {
                if (!started) return;
                canvas.moveTemp(positions);
                canvas.setDropTarget(view === 'atlas' ? model.regionAt(positions.get(id), scene.regionRects, scene.regionDepth) : '');
            });
            begin(event, ev => {
                if (!started) {
                    if (Math.hypot(ev.clientX - sx, ev.clientY - sy) < THRESHOLD) return;
                    if (!state.selection.has(id)) store.select([id]);
                    ids = [...state.selection].filter(cid => scene.cardById.has(cid));
                    if (!ids.includes(id)) ids.push(id);
                    for (const cid of ids) { const card = scene.cardById.get(cid); origin.set(cid, { x: card.x, y: card.y }); }
                    store.beginInteraction();
                    root.classList.add('is-dragging');
                    started = true;
                }
                compute(ev);
                paint.request();
            }, (ev, cancelled) => {
                paint.cancel();
                if (!started) return;
                if (!cancelled) {
                    compute(ev);
                    commitPlan(view, planDrop(view, positions, { nudge: true }), 'move');
                    canvas.renderNow();
                } else app.announce(t('desktop.infra_move_cancelled'));
                canvas.clearTemp(); canvas.setDropTarget('');
                root.classList.remove('is-dragging');
                store.endInteraction();
                holdClick();
            });
        }
        function startRegionMove(event, regionId) {
            const view = app.mapView(), scene = canvas.sceneData(), rect = scene.regionRects.get(regionId);
            if (!rect) return;
            const zoom = canvas.getView().zoom, sx = event.clientX, sy = event.clientY, family = subtree(regionId, view, scene);
            let started = false, delta = { x: 0, y: 0 };
            const compute = ev => { delta = { x: model.snap(rect.x + (ev.clientX - sx) / zoom) - rect.x, y: model.snap(rect.y + (ev.clientY - sy) / zoom) - rect.y }; };
            const paint = frame(() => {
                if (!started) return;
                const cards = new Map([...family.cards].map(cid => { const card = scene.cardById.get(cid); return [cid, { x: card.x + delta.x, y: card.y + delta.y }]; }));
                const regions = new Map([...family.regions].map(rid => { const r = scene.regionRects.get(rid); return [rid, { x: r.x + delta.x, y: r.y + delta.y, w: r.w, h: r.h }]; }));
                canvas.moveTemp(cards, regions);
            });
            begin(event, ev => {
                if (!started) {
                    if (Math.hypot(ev.clientX - sx, ev.clientY - sy) < THRESHOLD) return;
                    if (!state.selection.has(regionId)) store.select([regionId]);
                    store.beginInteraction();
                    root.classList.add('is-dragging');
                    started = true;
                }
                compute(ev);
                paint.request();
            }, (ev, cancelled) => {
                paint.cancel();
                if (!started) return;
                if (!cancelled) {
                    compute(ev);
                    if (delta.x || delta.y) {
                        commitRegionDelta(regionId, delta, view, scene, family);
                        canvas.renderNow();
                    }
                } else app.announce(t('desktop.infra_move_cancelled'));
                canvas.clearTemp();
                root.classList.remove('is-dragging');
                store.endInteraction();
                holdClick();
            });
        }
        function startResize(event, handleEl) {
            const id = handleEl.closest('[data-region]')?.dataset.region, handle = handleEl.dataset.resize, view = app.mapView();
            if (!id || !handle || view !== 'atlas' || !store.features.layoutParent) return;
            const scene = canvas.sceneData(), start = scene.regionRects.get(id);
            if (!start) return;
            const zoom = canvas.getView().zoom, sx = event.clientX, sy = event.clientY;
            let started = false, rect = start;
            const compute = ev => { rect = resizeRect(handle, start, (ev.clientX - sx) / zoom, (ev.clientY - sy) / zoom); };
            begin(event, ev => {
                if (!started) {
                    if (Math.hypot(ev.clientX - sx, ev.clientY - sy) < THRESHOLD) return;
                    store.beginInteraction();
                    started = true;
                }
                compute(ev);
                canvas.moveTemp(new Map(), new Map([[id, rect]]));
            }, (ev, cancelled) => {
                if (!started) return;
                if (!cancelled) {
                    compute(ev);
                    const before = store.currentIntent(id, view, false);
                    if (before && (rect.x !== before.x || rect.y !== before.y || rect.w !== before.w || rect.h !== before.h)) {
                        void store.dispatch(commands().resizePlace(view, new Map([[id, before]]), new Map([[id, { x: rect.x, y: rect.y, w: rect.w, h: rect.h, pinned: true }]]), ''));
                        canvas.renderNow();
                    }
                }
                canvas.clearTemp();
                store.endInteraction();
                holdClick();
            });
        }
        function startConnect(event, fromId) {
            const sx = event.clientX, sy = event.clientY;
            let started = false;
            const targetAt = ev => {
                const hit = document.elementFromPoint(ev.clientX, ev.clientY)?.closest('[data-node]');
                const id = hit && viewport.contains(hit) ? hit.dataset.node : '';
                return id && id !== fromId ? id : '';
            };
            begin(event, ev => {
                if (!started && Math.hypot(ev.clientX - sx, ev.clientY - sy) < THRESHOLD) return;
                started = true;
                const targetId = targetAt(ev), card = targetId ? canvas.sceneData().cardById.get(targetId) : null;
                canvas.setConnectTarget(targetId);
                canvas.showEdgePreview(fromId, card ? { x: card.x, y: card.y } : canvas.clientToWorld(ev.clientX, ev.clientY));
            }, (ev, cancelled) => {
                const targetId = started && !cancelled ? targetAt(ev) : '';
                canvas.setConnectTarget('');
                canvas.showEdgePreview(fromId, null);
                if (!started) return;
                holdClick();
                if (targetId) app.dialogs.openConnectionPopover({ from: fromId, to: targetId, clientX: ev.clientX, clientY: ev.clientY });
            });
        }

        // ---- pointer routing ----------------------------------------------------------------------
        function startPinch() {
            gesture?.abort();
            const [a, b] = [...touches.values()], rect = viewport.getBoundingClientRect();
            const mid = { x: (a.x + b.x) / 2 - rect.left, y: (a.y + b.y) / 2 - rect.top };
            const view = { ...canvas.getView() };
            pinch = { view, dist: Math.hypot(a.x - b.x, a.y - b.y) || 1, world: model.toWorld(view, mid) };
        }
        function onPointerDown(event) {
            if (event.button === 2) return; // a context menu press changes nothing
            if (event.pointerType === 'touch') {
                touches.set(event.pointerId, { x: event.clientX, y: event.clientY });
                if (touches.size === 2) { startPinch(); return; }
                if (touches.size > 2) return;
            }
            if (gesture && event.pointerType !== 'touch') gesture.abort(); // a release outside the canvas never reached it
            if (pinch || gesture || !(event.target instanceof Element)) return;
            if (moveMode) cancelMoveMode(); // only Enter keeps a keyboard move
            if (event.button !== 0 && event.button !== 1) return;
            if (event.button === 1 || spaceDown || state.tool === 'hand') { startPan(event, false); return; }
            const target = event.target;
            const card = target.closest('[data-node]'), header = target.closest('[data-region-header]'), handle = target.closest('[data-resize]'), edge = target.closest('[data-edge]');
            if (target.closest('[data-toggle-host-services], [data-hosted-service], [data-guest-more], [data-guest-shelf]')) return;
            if (!card && !header && !handle && !edge) {
                if (event.shiftKey) startMarquee(event); else startPan(event, true);
                return;
            }
            if (!store.canWrite()) return;
            if (card && (target.closest('[data-connect-handle]') || state.tool === 'connect')) { startConnect(event, card.dataset.node); return; }
            if (handle) { startResize(event, handle); return; }
            if (header) { startRegionMove(event, header.dataset.regionHeader); return; }
            if (card && !event.shiftKey && !isMod(event)) startCardDrag(event, card.dataset.node);
        }
        function onPointerMove(event) {
            if (!touches.has(event.pointerId)) return;
            touches.set(event.pointerId, { x: event.clientX, y: event.clientY });
            if (!pinch || touches.size < 2) return;
            const [a, b] = [...touches.values()], rect = viewport.getBoundingClientRect();
            const zoom = model.clampZoom(pinch.view.zoom * (Math.hypot(a.x - b.x, a.y - b.y) / pinch.dist));
            const mid = { x: (a.x + b.x) / 2 - rect.left, y: (a.y + b.y) / 2 - rect.top };
            canvas.setView({ x: mid.x - pinch.world.x * zoom, y: mid.y - pinch.world.y * zoom, zoom });
        }
        function onPointerEnd(event) {
            touches.delete(event.pointerId);
            if (pinch && touches.size < 2) pinch = null;
        }
        function onWheel(event) {
            event.preventDefault();
            const rect = viewport.getBoundingClientRect();
            if (isMod(event)) {
                canvas.zoomBy(Math.exp(-event.deltaY * (event.deltaMode === 1 ? 0.05 : 0.0025)), { x: event.clientX - rect.left, y: event.clientY - rect.top });
                return;
            }
            const unit = event.deltaMode === 1 ? 16 : 1;
            let dx = event.deltaX * unit, dy = event.deltaY * unit;
            if (event.shiftKey) [dx, dy] = [dy, dx];
            const view = canvas.getView();
            canvas.setView({ x: view.x - dx, y: view.y - dy, zoom: view.zoom });
        }

        // ---- clicks -------------------------------------------------------------------------------------
        const selectWith = (id, event) => store.select([id], { toggle: event.shiftKey || isMod(event) });
        function onClick(event) {
            if (suppressClick) { suppressClick = false; return; }
            const target = clickTarget(event);
            if (!target) return;
            const badge = target.closest('[data-toggle-host-services]');
            if (badge) { store.toggleHost(badge.dataset.toggleHostServices); return; }
            const chip = target.closest('[data-hosted-service]');
            if (chip) { store.select([chip.dataset.hostedService]); return; }
            const more = target.closest('[data-guest-more]');
            if (more) { app.library?.reveal(more.dataset.guestMore); return; }
            const card = target.closest('[data-node]');
            if (card) { selectWith(card.dataset.node, event); return; }
            const header = target.closest('[data-region-header]');
            if (header) { selectWith(header.dataset.regionHeader, event); return; }
            const edge = target.closest('[data-edge]');
            if (edge) store.selectEdge(edge.dataset.edge);
        }
        function onDblClick(event) {
            const card = clickTarget(event)?.closest('[data-node]');
            if (!card) return;
            store.select([card.dataset.node]);
            app.inspector?.focusName();
        }

        // ---- keyboard on the canvas ------------------------------------------------------------------------
        const focusedItem = () => {
            const active = document.activeElement;
            if (!active || !viewport.contains(active)) return null;
            const card = active.closest('[data-node]'), header = active.closest('[data-region-header]');
            return card ? { id: card.dataset.node, card: true } : header ? { id: header.dataset.regionHeader, card: false } : null;
        };
        function moveFocus(direction) {
            const scene = canvas.sceneData(), focused = focusedItem();
            let from = null;
            if (focused?.card) from = scene.cardById.get(focused.id);
            else if (focused) { const rect = scene.regionRects.get(focused.id); if (rect) from = { id: focused.id, x: rect.x + rect.w / 2, y: rect.y + rect.h / 2 }; }
            else from = scene.cardById.get(state.selected) || readingOrder(scene.cards)[0];
            if (!from) return;
            const next = focused ? model.spatialNeighbour(from, scene.cards, direction) : from.id;
            const el = next ? canvas.cardEl(next) : null;
            if (!el) return;
            canvas.setRovingCard(next);
            el.focus({ preventScroll: true });
            canvas.ensureVisible(next);
            app.announce(el.getAttribute('aria-label') || nameOf(next));
        }
        function onKeyDown(event) {
            if (isTyping(event.target)) return;
            if (moveMode) { onMoveKey(event); return; }
            const key = event.key, mod = isMod(event);
            const onItem = event.target === viewport || !!event.target.closest?.('[data-node], [data-region-header]');
            if (mod && !event.altKey && (key === 'a' || key === 'A')) {
                event.preventDefault();
                const filtersOn = model.filtersActive(state.filters);
                store.select(canvas.sceneData().cards.filter(card => !filtersOn || [card.id, ...card.guests].some(id => { const node = store.nodeById(id); return !!node && store.matches(node); })).map(card => card.id));
                return;
            }
            if (mod || event.altKey) return;
            if (ARROWS[key]) { event.preventDefault(); moveFocus(ARROWS[key]); return; }
            if (key === 'Escape') {
                if (gesture) { gesture.abort(); event.preventDefault(); } else store.clearSelection();
                return;
            }
            if (key === ' ' && onItem) { event.preventDefault(); if (!event.repeat) { spaceDown = true; spaceUsed = false; } return; }
            if (key === 'Enter' && onItem) {
                const item = focusedItem();
                if (item) { event.preventDefault(); selectWith(item.id, event); }
                return;
            }
            switch (key) {
                case 'm': case 'M': {
                    if (!store.canWrite()) { app.announce(t('desktop.infra_error_infra_readonly')); break; }
                    const ids = moveTargets(canvas.sceneData());
                    if (ids.length && enterMoveMode(ids)) event.preventDefault();
                    else app.announce(t('desktop.infra_move_nothing'));
                    break;
                }
                case 'Delete': case 'Backspace': {
                    if (!store.canWrite()) break;
                    const ids = state.selection.size ? [...state.selection] : focusedItem() ? [focusedItem().id] : [];
                    if (!ids.length) break;
                    event.preventDefault();
                    if (ids.length === 1) void app.dialogs.deleteNode(ids[0]); else void app.dialogs.deleteNodes(ids);
                    break;
                }
                case 'f': case 'F': canvas.fit({ animate: true }); break;
                case '+': case '=': canvas.zoomBy(1.2); break;
                case '-': canvas.zoomBy(1 / 1.2); break;
                case '0': canvas.zoomTo(1); break;
                case 'v': case 'V': store.setTool('select'); break;
                case 'h': case 'H': store.setTool('hand'); break;
                case 'c': case 'C': if (store.canWrite()) store.setTool('connect'); break;
                default: break;
            }
        }
        function onKeyUp(event) {
            if (event.key !== ' ') return;
            const used = spaceUsed;
            spaceDown = false; spaceUsed = false;
            if (used || moveMode || isTyping(event.target)) return;
            const item = focusedItem();
            if (!item) return;
            event.preventDefault();
            holdClick();
            store.select([item.id], { toggle: true });
        }
        function onFocusIn(event) {
            const target = event.target;
            if (target === viewport) {
                const scene = canvas.sceneData();
                const id = scene.cardById.has(state.selected) ? state.selected : readingOrder(scene.cards)[0]?.id;
                if (!id) return;
                canvas.setRovingCard(id);
                canvas.cardEl(id)?.focus({ preventScroll: true });
                return;
            }
            const item = target.closest?.('[data-node]') || target.closest?.('[data-region-header]');
            if (item) canvas.setRovingCard(item.dataset.node || item.dataset.regionHeader);
        }
        function onFocusOut(event) {
            if (viewport.contains(event.relatedTarget)) return;
            spaceDown = false;
            cancelMoveMode(); // losing focus is not a decision to save
        }

        // ---- move mode ----------------------------------------------------------------------------------------
        // moveTargets: the selected assets, else the one selected place, else the focused asset or place.
        function moveTargets(scene) {
            const selected = [...state.selection], focused = focusedItem();
            const cards = selected.filter(id => scene.cardById.has(id));
            if (cards.length) return cards;
            const places = selected.filter(id => scene.regionRects.has(id));
            if (places.length === 1) return places;
            if (places.length > 1 && focused && !focused.card && places.includes(focused.id)) return [focused.id];
            return focused && (focused.card ? scene.cardById.has(focused.id) : scene.regionRects.has(focused.id)) ? [focused.id] : [];
        }
        const canResizePlace = () => moveMode?.kind === 'region' && moveMode.view === 'atlas' && store.features.layoutParent;
        // placeRect is the place as the move mode shows it: moved, and grown or shrunk at the bottom right corner.
        function placeRect(rect) {
            return { x: rect.x + moveMode.dx, y: rect.y + moveMode.dy, w: rect.w + moveMode.dw, h: rect.h + moveMode.dh };
        }
        function showMoveTemp() {
            const scene = canvas.sceneData();
            if (moveMode.kind === 'region') {
                const { dx, dy, family } = moveMode;
                const cards = new Map([...family.cards].map(cid => { const card = scene.cardById.get(cid); return [cid, { x: card.x + dx, y: card.y + dy }]; }));
                const regions = new Map([...family.regions].map(rid => { const r = scene.regionRects.get(rid); return [rid, { x: r.x + dx, y: r.y + dy, w: r.w, h: r.h }]; }));
                regions.set(moveMode.regionId, placeRect(scene.regionRects.get(moveMode.regionId)));
                canvas.moveTemp(cards, regions);
                return;
            }
            const positions = new Map(moveMode.ids.map(id => [id, { x: moveMode.origin.get(id).x + moveMode.dx, y: moveMode.origin.get(id).y + moveMode.dy }]));
            canvas.moveTemp(positions);
            if (moveMode.view === 'atlas') canvas.setDropTarget(model.regionAt(positions.get(moveMode.ids[0]), scene.regionRects, scene.regionDepth));
        }
        // enterMoveMode starts the keyboard move of assets, or of one place with everything inside it.
        function enterMoveMode(ids) {
            if (!store.canWrite() || moveMode || gesture) return false;
            const scene = canvas.sceneData(), view = app.mapView(), cardIds = ids.filter(id => scene.cardById.has(id));
            let focusTarget = null;
            if (cardIds.length) {
                moveMode = { view, kind: 'cards', ids: cardIds, origin: new Map(cardIds.map(id => [id, { x: scene.cardById.get(id).x, y: scene.cardById.get(id).y }])), dx: 0, dy: 0 };
                focusTarget = canvas.cardEl(cardIds[0]);
            } else if (ids.length === 1 && scene.regionRects.has(ids[0])) {
                moveMode = { view, kind: 'region', ids: [], regionId: ids[0], family: subtree(ids[0], view, scene), dx: 0, dy: 0, dw: 0, dh: 0, resizing: false };
                focusTarget = canvas.regionEl(ids[0])?.querySelector('[data-region-header]');
            } else return false;
            store.beginInteraction();
            app.announce(t(moveMode.kind === 'region' ? 'desktop.infra_move_mode_place' : 'desktop.infra_move_mode'));
            if (focusTarget && document.activeElement !== focusTarget) focusTarget.focus({ preventScroll: true });
            return true;
        }
        function finishMoveMode(commit) {
            if (!moveMode) return;
            const mode = moveMode;
            moveMode = null;
            if (commit && mode.kind === 'region') {
                const scene = canvas.sceneData(), start = scene.regionRects.get(mode.regionId);
                if (start && (mode.dx || mode.dy)) commitRegionDelta(mode.regionId, { x: mode.dx, y: mode.dy }, mode.view, scene, mode.family);
                if (start && (mode.dw || mode.dh)) {
                    // Saved after the move, so the stored place already sits where it was moved to.
                    const end = { x: start.x + mode.dx, y: start.y + mode.dy, w: start.w + mode.dw, h: start.h + mode.dh };
                    const before = store.currentIntent(mode.regionId, mode.view, false);
                    if (before) void store.dispatch(commands().resizePlace(mode.view, new Map([[mode.regionId, before]]), new Map([[mode.regionId, { x: end.x, y: end.y, w: end.w, h: end.h, pinned: true }]]), ''));
                }
                canvas.renderNow();
            } else if (commit && (mode.dx || mode.dy)) {
                const wants = new Map(mode.ids.map(id => [id, { x: mode.origin.get(id).x + mode.dx, y: mode.origin.get(id).y + mode.dy }]));
                commitPlan(mode.view, planDrop(mode.view, wants, { nudge: false }), 'move');
                canvas.renderNow();
            }
            canvas.clearTemp(); canvas.setDropTarget('');
            store.endInteraction();
        }
        // cancelMoveMode puts everything back and says so. Only Enter keeps a keyboard move.
        function cancelMoveMode() {
            if (!moveMode) return;
            finishMoveMode(false);
            app.announce(t('desktop.infra_move_cancelled'));
        }
        // R switches a place between moving and resizing; the arrows then do what the mode says.
        function onMoveKey(event) {
            if ((event.key === 'r' || event.key === 'R') && !event.ctrlKey && !event.metaKey && !event.altKey) {
                if (!canResizePlace()) return;
                event.preventDefault();
                moveMode.resizing = !moveMode.resizing;
                app.announce(t(moveMode.resizing ? 'desktop.infra_resize_mode_on' : 'desktop.infra_move_mode_place'));
                return;
            }
            const resizing = moveMode.kind === 'region' && moveMode.resizing === true;
            const step = resizing ? 24 : event.shiftKey ? 8 : 24;
            const delta = { ArrowLeft: [-step, 0], ArrowRight: [step, 0], ArrowUp: [0, -step], ArrowDown: [0, step] }[event.key];
            if (delta) {
                event.preventDefault();
                if (!resizing) { moveMode.dx += delta[0]; moveMode.dy += delta[1]; }
                else if (canResizePlace()) {
                    // Exactly one step per press, never below the minimum. The clamped size is what is kept,
                    // so the arrow back is not swallowed by the minimum.
                    const rect = canvas.sceneData().regionRects.get(moveMode.regionId);
                    const size = { w: Math.max(model.REGION_MIN_W, rect.w + moveMode.dw + delta[0]), h: Math.max(model.REGION_MIN_H, rect.h + moveMode.dh + delta[1]) };
                    moveMode.dw = size.w - rect.w; moveMode.dh = size.h - rect.h;
                    app.announce(model.fill(t('desktop.infra_place_size'), { w: size.w, h: size.h }));
                } else return;
                showMoveTemp();
                return;
            }
            if (event.key === 'Enter') { event.preventDefault(); finishMoveMode(true); }
            else if (event.key === 'Escape') { event.preventDefault(); cancelMoveMode(); }
            else if (event.key === 'Tab') cancelMoveMode(); // focus is about to leave; nothing was saved
        }

        // ---- library drag ----------------------------------------------------------------------------------------
        // startLibraryDrag drags rows of the library (mode 'place' for To place, 'move' for assets on the
        // map) onto the map or onto a place row; items/'discovery' belong to the import sheet.
        function startLibraryDrag(event, ids, { mode = 'place', items = null, onDragged = () => app.library?.suppressNextClick() } = {}) {
            if (!store.canWrite() || moveMode || pinch || !ids?.length) return;
            gesture?.abort(); // a press that never saw its release
            const target = event.currentTarget || app.library?.el;
            if (!target) return;
            const sx = event.clientX, sy = event.clientY, touch = event.pointerType === 'touch', view = app.mapView();
            let started = false, ghost = null, last = null;
            // Discovered guests join their host; the map position under the pointer means nothing to them.
            const guestsOnly = mode === 'discovery' && !!items?.length && items.every(item => item.guest);
            const probe = ev => {
                const hit = document.elementFromPoint(ev.clientX, ev.clientY), scene = canvas.sceneData();
                const over = !!hit && viewport.contains(hit), world = canvas.clientToWorld(ev.clientX, ev.clientY);
                // A place row of the library takes the drop before the map does.
                const row = mode !== 'discovery' ? hit?.closest?.('.infra-library-row') : null;
                const dropId = row && row.hasAttribute('data-library-drop') && app.library.el.contains(row) ? row.dataset.libraryDrop : '';
                return { over, onLibrary: !!hit && target.contains(hit), world, placeTarget: dropId && !ids.includes(dropId) ? dropId : '', region: over && view === 'atlas' ? model.regionAt(world, scene.regionRects, scene.regionDepth) : '' };
            };
            const paint = frame(() => {
                if (!ghost || !last) return;
                const info = probe(last), onMap = info.over && !info.placeTarget;
                app.library?.setDropTarget(info.placeTarget);
                canvas.setDropTarget(info.placeTarget || guestsOnly ? '' : info.region);
                const label = info.placeTarget ? model.fill(t('desktop.infra_drop_into'), { name: nameOf(info.placeTarget) })
                    : onMap ? (guestsOnly ? t('desktop.infra_drop_guest') : info.region ? model.fill(t('desktop.infra_drop_into'), { name: nameOf(info.region) }) : t('desktop.infra_drop_on_map')) : '';
                const invalid = !info.placeTarget && !info.over && !info.onLibrary;
                ghost.move(last.clientX, last.clientY, { scale: onMap ? canvas.getView().zoom : 1, label, invalid });
                root.classList.toggle('is-drag-invalid', invalid);
            });
            const cap = capturePointer(target, event, ev => {
                if (!started) {
                    const dx = ev.clientX - sx, dy = ev.clientY - sy;
                    if (Math.hypot(dx, dy) < LIBRARY_THRESHOLD) return;
                    if (touch && Math.abs(dy) > Math.abs(dx)) { cap.detach(); gesture = null; return; }
                    started = true;
                    ghost = canvas.createGhost(ids, items);
                    store.beginInteraction();
                    root.classList.add('is-dragging');
                }
                last = ev;
                paint.request();
            }, (ev, cancelled) => {
                gesture = null;
                paint.cancel();
                if (!started) return;
                const info = !cancelled ? probe(ev) : null;
                ghost?.remove();
                canvas.setDropTarget(''); app.library?.setDropTarget('');
                root.classList.remove('is-dragging', 'is-drag-invalid');
                if (info?.placeTarget) void finishLibraryDrop(ids, { mode, placeTarget: info.placeTarget });
                else if (info?.over) void finishLibraryDrop(ids, { mode, world: info.world, region: info.region });
                store.endInteraction();
                onDragged();
            });
            gesture = cap;
        }
        // finishLibraryDrop commits a library drag that ended over the map or on a place row.
        function finishLibraryDrop(ids, { mode = 'place', placeTarget = '', world = null, region = '' } = {}) {
            const view = app.mapView();
            if (mode === 'discovery') return world && app.importer ? app.importer.dropOnCanvas(ids, world, view === 'atlas' ? region : '') : Promise.resolve(false);
            if (placeTarget) return moveIntoPlace(ids, placeTarget);
            if (!world) return Promise.resolve(false);
            const points = ids.length === 1 ? [world] : model.compactGrid(ids.length, world);
            commitPlan(view, planDrop(view, new Map(ids.map((id, index) => [id, points[index]])), { nudge: true }), mode === 'move' ? 'move' : 'place');
            store.select(ids);
            canvas.renderNow();
            return Promise.resolve(true);
        }
        // positionsAt returns free card centres around a world point.
        function positionsAt(count, world) {
            const obstacles = canvas.sceneData().cards.map(model.cardRect);
            const points = count === 1 ? [world] : model.compactGrid(count, world);
            return points.map(point => { const pos = model.freeSlot(point, obstacles); obstacles.push(model.cardRect(pos)); return { x: Math.round(pos.x), y: Math.round(pos.y) }; });
        }
        // moveIntoPlace puts assets into a place ('' = out of every place): placed assets with a drawn
        // target move inside it in one layout save, the rest only change parent_id (node update).
        function moveIntoPlace(ids, placeId) {
            if (!store.canWrite()) return Promise.resolve(false);
            const view = app.mapView(), scene = canvas.sceneData(), parentOf = id => store.effectiveParent(id, view);
            const valid = ids.filter(id => store.nodeById(id) && id !== placeId && !(placeId && model.isDescendant(placeId, id, parentOf)) && parentOf(id) !== placeId);
            if (!valid.length) return Promise.resolve(false);
            const target = placeId ? scene.regionRects.get(placeId) : null, layoutIds = [], parentIds = [];
            for (const id of valid) {
                const node = store.nodeById(id);
                if (view === 'atlas' && store.features.layoutParent && scene.cardById.has(id) && !model.isContainer(node.kind) && (!placeId || target)) layoutIds.push(id); else parentIds.push(id);
            }
            const label = placeId ? model.countText(valid.length, t('desktop.infra_moved_to_place_one'), t('desktop.infra_moved_to_place'), { place: nameOf(placeId) }) : model.countText(valid.length, t('desktop.infra_moved_out_of_place_one'), t('desktop.infra_moved_out_of_place'));
            const work = [];
            if (layoutIds.length) {
                const moving = new Set(layoutIds), obstacles = scene.cards.filter(card => !moving.has(card.id)).map(model.cardRect);
                const first = scene.cardById.get(layoutIds[0]), from = scene.regionRects.get(parentOf(layoutIds[0]));
                const centre = target ? { x: target.x + target.w / 2, y: target.y + target.h / 2 } : from ? { x: from.x + from.w + model.REGION_PAD + model.CARD_W / 2, y: first.y } : { x: first.x, y: first.y };
                const wants = model.compactGrid(layoutIds.length, centre), before = new Map(), after = new Map(), regionIds = new Set(scene.regionRects.keys());
                const placementOf = id => after.get(id) || store.effectivePlacement(id, view);
                layoutIds.forEach((id, index) => {
                    const pos = model.freeSlot(wants[index], obstacles);
                    obstacles.push(model.cardRect(pos));
                    before.set(id, store.currentIntent(id, view, true));
                    after.set(id, { x: Math.round(pos.x), y: Math.round(pos.y), pinned: true, parent_id: placeId });
                    if (!placeId) return;
                    for (const [regionId, rect] of model.growRegions(placeId, model.cardRect(pos), { ids: regionIds, parentOf, placementOf })) {
                        if (!before.has(regionId)) before.set(regionId, store.currentIntent(regionId, view, false));
                        after.set(regionId, { ...(after.get(regionId) || {}), x: Math.round(rect.x), y: Math.round(rect.y), w: Math.round(rect.w), h: Math.round(rect.h), pinned: true });
                    }
                });
                work.push(store.dispatch(commands().reparent(view, before, after, label)));
            }
            if (parentIds.length) work.push(store.dispatch(commands().setParent(parentIds.map(id => ({ id, before: store.nodeById(id).parent_id || '', after: placeId })), label)));
            app.announce(label);
            return Promise.all(work).then(results => results.every(Boolean)).catch(error => store.writeFailed(error).then(() => false));
        }
        // removeFromMap unplaces assets in the current map view (undoable).
        function removeFromMap(ids) {
            const view = app.mapView(), before = new Map();
            for (const id of ids) { const intent = store.currentIntent(id, view, false); if (intent) before.set(id, intent); }
            if (!before.size || !store.canWrite() || !store.features.unplace) return Promise.resolve(false);
            const label = model.countText(before.size, t('desktop.infra_removed_from_map_one'), t('desktop.infra_removed_from_map'));
            app.announce(label);
            return store.dispatch(commands().unplace(view, before, label));
        }

        // ---- placing and arranging ---------------------------------------------------------------------------------
        function placeAtCenter(ids, { moveMode: enter = false } = {}) {
            if (!store.canWrite() || !ids?.length) return;
            app.ensureMapView();
            canvas.renderNow();
            const view = app.mapView(), size = canvas.size(), scene = canvas.sceneData();
            const fallback = scene.bounds ? { x: scene.bounds.x + scene.bounds.w / 2, y: scene.bounds.y + scene.bounds.h / 2 } : { x: 0, y: 0 };
            const centre = size.w > 0 && size.h > 0 ? model.toWorld(canvas.getView(), { x: size.w / 2, y: size.h / 2 }) : fallback;
            const spots = model.compactGrid(ids.length, centre);
            commitPlan(view, planDrop(view, new Map(ids.map((id, index) => [id, spots[index]])), { nudge: true }), 'place');
            store.select(ids);
            canvas.renderNow();
            canvas.setRovingCard(ids[0]);
            canvas.cardEl(ids[0])?.focus({ preventScroll: true });
            if (enter) enterMoveMode(ids);
        }
        function arrange() {
            if (!store.canWrite()) return;
            const view = app.mapView(), scene = canvas.sceneData();
            const selected = [...state.selection].filter(id => scene.cardById.has(id));
            const targets = selected.length ? selected : scene.cards.filter(card => !scene.regionRects.has(store.effectiveParent(card.id, view))).map(card => card.id);
            const targetSet = new Set(targets);
            const nodes = targets.map(id => store.nodeById(id)).filter(Boolean);
            const obstacles = scene.cards.filter(card => !targetSet.has(card.id)).map(card => ({ x: card.x, y: card.y }));
            const positions = view === 'topology' ? model.topologyPositions(nodes, obstacles) : model.gridPositions(nodes, obstacles, state.nodes.length);
            if (!positions.size) return;
            const before = new Map(), after = new Map();
            for (const [id, position] of positions) {
                const current = store.currentIntent(id, view, false);
                if (!current) continue;
                before.set(id, current);
                after.set(id, { x: Math.round(position.x), y: Math.round(position.y), pinned: position.pinned === true });
            }
            if (!after.size) return;
            const label = t('desktop.infra_arranged');
            void store.dispatch(commands().moveNodes(view, before, after, label));
            app.announce(label);
        }

        // ---- shortcuts on the whole window --------------------------------------------------------------------------
        function onRootKeydown(event) {
            if (event.key === 'Escape' && gesture) { event.preventDefault(); gesture.abort(); return; }
            if (app.q('[data-dialog]')?.open) return;
            const mod = isMod(event), key = String(event.key || '').toLowerCase();
            if (mod && key === 's') { event.preventDefault(); app.inspector?.saveDraft(); return; }
            if (mod && key === 'k') {
                const search = app.q('[data-search]');
                if (search) { event.preventDefault(); search.focus(); search.select?.(); }
                return;
            }
            if (mod && !event.altKey && (key === 'z' || key === 'y') && !isTyping(event.target)) {
                event.preventDefault();
                const view = app.mapView(), redo = key === 'y' || event.shiftKey;
                void (redo ? store.redo(view) : store.undo(view)).then(ok => { if (ok) app.announce(redo ? t('desktop.infra_redone') : t('desktop.infra_undone')); });
                return;
            }
            if (event.key === 'Escape' && event.target.closest?.('[data-inspector]') && !isTyping(event.target)) { event.preventDefault(); app.inspector?.close(); }
        }
        function cancel() {
            gesture?.abort();
            finishMoveMode(false);
            pinch = null; touches.clear();
            spaceDown = false;
        }

        const listeners = [
            ['pointerdown', onPointerDown], ['pointermove', onPointerMove], ['pointerup', onPointerEnd], ['pointercancel', onPointerEnd], ['lostpointercapture', onPointerEnd],
            ['click', onClick], ['dblclick', onDblClick], ['keydown', onKeyDown], ['keyup', onKeyUp], ['focusin', onFocusIn], ['focusout', onFocusOut],
        ];
        function attach() {
            if (attached) return;
            attached = true;
            for (const [type, handler] of listeners) viewport.addEventListener(type, handler);
            viewport.addEventListener('wheel', onWheel, { passive: false });
        }
        function dispose() {
            cancel();
            clearTimeout(suppressTimer);
            if (attached) {
                for (const [type, handler] of listeners) viewport.removeEventListener(type, handler);
                viewport.removeEventListener('wheel', onWheel);
            }
            attached = false;
        }

        return { attach, cancel, startLibraryDrag, finishLibraryDrop, moveIntoPlace, removeFromMap, positionsAt, placeAtCenter, enterMoveMode, arrange, onRootKeydown, dispose };
    }

    window.InfraAppGestures = { create };
})();
