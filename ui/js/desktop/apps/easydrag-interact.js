// EasyDrag canvas interaction: select, move (alignment guides, drop on wire), connect
// (magnetic ports), box select, pan and zoom (wheel, space, middle button, pinch),
// context menus, keyboard model and clipboard.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const DRAG_THRESHOLD = 4;
    const SNAP_RADIUS = 20;
    const GUIDE_RADIUS = 6;
    const CLIPBOARD_KEY = 'aurago.easydrag.clipboard';
    // The fragment kept in localStorage can hold typed parameters in plain text: it carries the time
    // it was copied and is dropped after CLIPBOARD_MAX_AGE_MS. One without that stamp (kept before the
    // stamp existed) counts as expired.
    const CLIPBOARD_MAX_AGE_MS = 24 * 60 * 60 * 1000;

    function create(ed, canvas, wires) {
        const core = ED.core;
        const G = ED.geometry;
        const { t } = ed;
        const bag = core.bag();
        const el = canvas.el;
        const overlay = el.querySelector('.ed-overlay');
        const selectBox = el.querySelector('.ed-select-box');
        const guides = core.el('<div class="ed-guides" aria-hidden="true"><div class="ed-guide ed-guide--v" hidden></div><div class="ed-guide ed-guide--h" hidden></div></div>');
        overlay.appendChild(guides);
        const guideV = guides.firstChild;
        const guideH = guides.lastChild;
        // connectHint shows beside the pointer why the port under it refuses the connection (U14). The
        // overlay is aria-hidden; the release announces the same words.
        const connectHint = core.el('<div class="ed-connect-hint" hidden></div>');
        overlay.appendChild(connectHint);
        connectHint.hidden = true;
        let hintTimer = 0;

        function showConnectHint(reason, clientX, clientY) {
            clearTimeout(hintTimer);
            hintTimer = 0;
            if (!reason) { connectHint.hidden = true; return; }
            const s = canvas.size();
            connectHint.textContent = core.tr(t, 'easydrag.ui.connect_' + reason, t('easydrag.ui.connect_port'));
            connectHint.style.transform = 'translate(' + Math.round(clientX - s.left + 14) + 'px,' + Math.round(clientY - s.top + 14) + 'px)';
            connectHint.hidden = false;
        }

        // A refused release keeps the reason on screen for 2.5 s.
        function fadeConnectHint() {
            clearTimeout(hintTimer);
            hintTimer = setTimeout(() => { hintTimer = 0; connectHint.hidden = true; }, 2500);
        }
        const pointers = new Map();
        let spaceDown = false;
        let lastPointer = null;
        let hoverTimer = 0;
        let pinch = null;
        let longPress = 0;
        let gesture = null; // the pan, drag, connect or box select that follows a pointer now

        const readonly = () => !!(ed.readonly || ed.runView);

        // ── announcements ───────────────────────────────────────────────────────

        const nameOf = id => { const n = ed.model.node(id); return n ? (n.label || n.type) : ''; };
        // tell says what an action did to steps in the live region: the name of one, the count of several.
        function tell(keyOne, keyMany, names) { if (names.length) canvas.announce(core.stepsText(t, keyOne, keyMany, names)); }
        // Arrow-key moves come in bursts: the live region hears about them once, 500 ms after the last
        // key press. A pointer drag never goes through here.
        let movedIds = [];
        const movedSoon = core.debounce(() => {
            tell('easydrag.ui.announce_moved_one', 'easydrag.ui.announce_moved_many', movedIds.map(nameOf).filter(Boolean));
            movedIds = [];
        }, 500);

        // ── selection ───────────────────────────────────────────────────────────

        function select(ids, opts) {
            if (ed.groups) ed.groups.clearActive();
            const next = opts && opts.toggle ? new Set(ed.selection) : new Set();
            (ids || []).forEach(id => {
                if (opts && opts.toggle && next.has(id)) next.delete(id); else next.add(id);
            });
            ed.selection.clear();
            next.forEach(id => ed.selection.add(id));
            ed.selectedEdge = null;
            ed.bus.emit('selection', ed.selection);
        }

        function selectAll() { select(ed.model.doc.nodes.map(n => n.id)); }

        // ── hit testing ─────────────────────────────────────────────────────────

        function portAt(clientX, clientY, side, except) {
            const p = canvas.clientToWorld(clientX, clientY);
            const radius = SNAP_RADIUS / ed.view.zoom;
            let best = null;
            ed.model.doc.nodes.forEach(n => {
                if (n.id === except || ed.groups && ed.groups.hidden(n.id)) return;
                const ports = side === 'in' ? ed.model.inputs(n) : ed.model.outputs(n);
                const outCount = ed.model.outputs(n).length;
                ports.forEach((port, idx) => {
                    const pt = G.portPoint(n.position, side, idx, outCount);
                    const d = Math.hypot(pt.x - p.x, pt.y - p.y);
                    if (d <= radius && (!best || d < best.d)) best = { node: n.id, port, point: pt, d };
                });
            });
            if (best) return best;
            const card = document.elementFromPoint(clientX, clientY);
            const nodeEl = card && card.closest && card.closest('.ed-node');
            if (nodeEl && nodeEl.dataset.nodeId !== except) {
                const n = ed.model.node(nodeEl.dataset.nodeId);
                const ports = n ? (side === 'in' ? ed.model.inputs(n) : ed.model.outputs(n)) : [];
                if (n && ports.length) return { node: n.id, port: ports[0], point: G.portPoint(n.position, side, 0, ed.model.outputs(n).length) };
            }
            return null;
        }

        // edgeNear finds the wire closest to worldPoint within reach. It runs on every drag frame, so it
        // looks nodes up in a Map and skips wires whose control-point box is out of reach.
        function edgeNear(worldPoint, exceptNode) {
            if (ed.renderStats) ed.renderStats.edgeNear++;
            const byId = new Map(ed.model.doc.nodes.map(n => [n.id, n]));
            const reach = 18 / ed.view.zoom;
            let best = null;
            ed.model.doc.edges.forEach(e => {
                if (e.source.node === exceptNode || e.target.node === exceptNode) return;
                const geo = wires.ends(e, byId); if (!geo) return;
                const { a, b } = geo;
                const box = G.wireBounds(a, b);
                if (worldPoint.x < box.x - reach || worldPoint.x > box.x + box.w + reach || worldPoint.y < box.y - reach || worldPoint.y > box.y + box.h + reach) return;
                const d = G.distanceToWire(worldPoint, a, b);
                if (d < reach && (!best || d < best.d)) best = { id: e.id, d };
            });
            return best;
        }

        // ── pointer handling ────────────────────────────────────────────────────

        function onPointerDown(event) {
            if (event.target.closest('.ed-zoom, .ed-minimap, .ed-empty, .ed-wire-actions, .ed-node-tools, .ed-port-tip')) return;
            el.focus({ preventScroll: true });
            pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
            // A second finger turns the gesture into a pinch; more fingers are ignored.
            if (pointers.size > 1) { if (pointers.size === 2) startPinch(); return; }
            const addBtn = event.target.closest('[data-ed-port-add]');
            if (addBtn && !readonly()) {
                event.preventDefault();
                event.stopPropagation();
                const card = addBtn.closest('.ed-node');
                const rect = addBtn.getBoundingClientRect();
                ed.bus.emit('quick-add', { x: rect.right + 8, y: rect.top, from: { node: card.dataset.nodeId, port: addBtn.dataset.edPortAdd } });
                return;
            }
            const portEl = event.target.closest('.ed-port');
            const nodeEl = event.target.closest('.ed-node');
            const groupHandle = event.target.closest('[data-ed-group-drag]');
            if (groupHandle && ed.groups && event.button === 0 && !spaceDown) {
                const id = groupHandle.dataset.edGroupDrag, item = ed.groups.group(id); if (!item) return;
                ed.groups.select(id, event.shiftKey || core.isMod(event) || item.members.every(member => ed.selection.has(member)));
                const first = ed.groups.movable(item.members)[0]; if (first) startNodeDrag(event, first, true); return;
            }
            const panGesture = event.button === 1 || spaceDown || (event.pointerType === 'touch' && !nodeEl);
            if (panGesture) { startPan(event, event.pointerType === 'touch' ? event.target.closest('.ed-edge') : null); return; }
            if (event.button !== 0) return;
            if (portEl && nodeEl && !readonly()) { startConnect(event, nodeEl.dataset.nodeId, portEl.dataset.edPort, portEl.dataset.edSide); return; }
            if (nodeEl) { startNodeDrag(event, nodeEl.dataset.nodeId); return; }
            if (!event.target.closest('.ed-edge')) startBoxSelect(event);
        }

        // capture makes a gesture follow its pointer (core.capturePointer: a cancel, lost capture or a
        // missed mouse release ends it as cancelled). The gesture's abort() cancels it early. Its end
        // is announced as "gesture-end" once onUp ran (the editor completes the emergency copy).
        function capture(event, onMove, onUp) {
            const g = core.capturePointer(el, event, onMove, (ev, cancelled) => {
                if (gesture === g) gesture = null;
                // An aborted gesture's finger stays down for the pinch; a missed release frees it.
                if (ev.type !== 'abort') pointers.delete(ev.pointerId);
                ed.dragging = false;
                onUp(ev, cancelled);
                ed.bus.emit('gesture-end');
            });
            gesture = g;
            return g;
        }

        // swallowNextClick eats the click a long press gives on release: the menu opened under the finger,
        // and the click would run the item there. It lasts for 800 ms at most.
        function swallowNextClick() {
            const eat = (ev) => { ev.preventDefault(); ev.stopPropagation(); off(); };
            const off = () => { clearTimeout(timer); document.removeEventListener('click', eat, true); };
            const timer = setTimeout(off, 800);
            document.addEventListener('click', eat, true);
        }

        // startPan moves the view with the pointer. A finger that lands on a wire and does not travel
        // (tapped) selects the wire: the pan captures the pointer, so the wire never gets its own click.
        function startPan(event, tapped) {
            event.preventDefault();
            const start = { x: event.clientX, y: event.clientY, view: Object.assign({}, ed.view) };
            let moved = false;
            // A finger shakes more than a mouse: a tap may travel 10 px.
            const slop = event.pointerType === 'touch' ? 10 : DRAG_THRESHOLD;
            el.classList.add('is-panning');
            capture(event, ev => {
                if (pinch) return;
                if (Math.hypot(ev.clientX - start.x, ev.clientY - start.y) >= slop) moved = true;
                canvas.setView({ x: start.view.x + ev.clientX - start.x, y: start.view.y + ev.clientY - start.y, zoom: start.view.zoom });
            }, (ev, cancelled) => {
                el.classList.remove('is-panning');
                if (tapped && !moved && !cancelled) wires.tapEdge(tapped.dataset.edgeId);
            });
        }

        // startPinch ends the first finger's pan, drag or box select (a drag moves back) and zooms
        // from the view as it is now.
        function startPinch() {
            if (gesture) gesture.abort();
            const pts = Array.from(pointers.values());
            pinch = { dist: Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y), view: Object.assign({}, ed.view) };
        }

        function onPointerMoveAny(event) {
            lastPointer = { x: event.clientX, y: event.clientY };
            if (pointers.has(event.pointerId)) pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
            if (pinch && pointers.size === 2) {
                const pts = Array.from(pointers.values());
                const dist = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y);
                const s = canvas.size();
                const center = { x: (pts[0].x + pts[1].x) / 2 - s.left, y: (pts[0].y + pts[1].y) / 2 - s.top };
                canvas.setView(G.zoomAt(pinch.view, pinch.view.zoom * dist / Math.max(pinch.dist, 1), center));
            }
        }

        function onPointerUpAny(event) {
            pointers.delete(event.pointerId);
            if (pointers.size < 2 && pinch) pinch = null;
        }

        function startNodeDrag(event, nodeId, selectionReady) {
            const additive = event.shiftKey || core.isMod(event);
            if (!selectionReady) { if (additive) select([nodeId], { toggle: true });
            else if (!ed.selection.has(nodeId)) select([nodeId]); }
            if (readonly() || additive) return;
            const ids = ed.groups ? ed.groups.movable(ed.selection) : Array.from(ed.selection);
            if (!ids.includes(nodeId)) return;
            const groupedGesture = !!(selectionReady || ed.groups && ed.groups.active);
            if (groupedGesture) ed.model.endCoalescing();
            const origin = new Map(ids.map(id => [id, Object.assign({}, ed.model.node(id).position)]));
            const start = { x: event.clientX, y: event.clientY };
            const primary = ed.model.node(nodeId);
            let moved = false;
            let applied = { x: 0, y: 0 };
            let dropEdge = null;
            if (event.pointerType === 'touch') {
                const groupId = selectionReady && ed.groups && ed.groups.memberOf(nodeId);
                longPress = setTimeout(() => { if (!moved) { if (groupId) { if (gesture) gesture.abort(); pointers.delete(event.pointerId); ed.groups.open(groupId); } else { swallowNextClick(); openNodeMenu(nodeId, start.x, start.y); } } }, 550);
            }
            capture(event, ev => {
                const dx = (ev.clientX - start.x) / ed.view.zoom;
                const dy = (ev.clientY - start.y) / ed.view.zoom;
                if (!moved && Math.hypot(ev.clientX - start.x, ev.clientY - start.y) < DRAG_THRESHOLD) return;
                if (!moved) { moved = true; clearTimeout(longPress); ed.dragging = true; el.classList.add('is-dragging'); wires.hideActions(); }
                const base = origin.get(nodeId);
                const snapped = alignTo(primary.id, { x: base.x + dx, y: base.y + dy }, ids);
                const want = { x: snapped.x - base.x, y: snapped.y - base.y };
                ed.model.moveNodes(ids, want.x - applied.x, want.y - applied.y);
                applied = want;
                dropEdge = null;
                if (ids.length === 1 && !ed.model.incoming(nodeId).length && !ed.model.outgoing(nodeId).length && ed.model.inputs(primary).length && ed.model.outputs(primary).length) {
                    const rect = canvas.nodeRect(nodeId);
                    const cy = rect.y + rect.h / 2;
                    const near = edgeNear({ x: rect.x + rect.w / 2, y: cy }, nodeId) ||
                        edgeNear({ x: rect.x + 16, y: cy }, nodeId) ||
                        edgeNear({ x: rect.x + rect.w - 16, y: cy }, nodeId);
                    dropEdge = near ? near.id : null;
                }
                wires.markDropTarget(dropEdge);
            }, (ev, cancelled) => {
                clearTimeout(longPress);
                el.classList.remove('is-dragging');
                hideGuides();
                wires.markDropTarget(null);
                if (!moved) return;
                if (cancelled) { ed.model.moveNodes(ids, -applied.x, -applied.y); if (groupedGesture) ed.model.endCoalescing(); return; }
                const n = ed.model.node(nodeId);
                const grid = { x: G.snap(n.position.x) - n.position.x, y: G.snap(n.position.y) - n.position.y };
                ed.model.moveNodes(ids, grid.x, grid.y);
                if (dropEdge) ed.model.insertOnEdge(dropEdge, nodeId, null);
                if (groupedGesture) ed.model.endCoalescing();
            });
        }

        // alignTo snaps the dragged card to the left edge or centre line of other cards and shows guides.
        function alignTo(id, pos, moving) {
            if (ed.renderStats) ed.renderStats.align++;
            const skip = new Set(moving);
            const h = G.nodeHeight(ed.model.outputs(ed.model.node(id)).length);
            let x = pos.x;
            let y = pos.y;
            let gx = null;
            let gy = null;
            ed.model.doc.nodes.forEach(n => {
                if (skip.has(n.id) || ed.groups && ed.groups.hidden(n.id)) return;
                const nh = G.nodeHeight(ed.model.outputs(n).length);
                if (gx === null && Math.abs(n.position.x - pos.x) < GUIDE_RADIUS / ed.view.zoom) { x = n.position.x; gx = x; }
                const cy = n.position.y + nh / 2;
                if (gy === null && Math.abs(cy - (pos.y + h / 2)) < GUIDE_RADIUS / ed.view.zoom) { y = cy - h / 2; gy = cy; }
            });
            showGuides(gx, gy);
            return { x, y };
        }

        function showGuides(gx, gy) {
            const v = ed.view;
            guideV.hidden = gx === null;
            guideH.hidden = gy === null;
            if (gx !== null) guideV.style.transform = 'translateX(' + (gx * v.zoom + v.x) + 'px)';
            if (gy !== null) guideH.style.transform = 'translateY(' + (gy * v.zoom + v.y) + 'px)';
        }

        function hideGuides() { guideV.hidden = true; guideH.hidden = true; }

        function startConnect(event, nodeId, port, side) {
            event.preventDefault();
            event.stopPropagation();
            let from = { node: nodeId, port };
            let reverse = side === 'in';
            // Pressing a wired input picks its wire up. A cancelled pick-up puts it back by undoing
            // the disconnect, as long as nothing else changed the document since (undoTop).
            let undoTop = false;
            let stopWatch = null;
            const existing = reverse ? ed.model.incoming(nodeId).filter(e => e.target.port === port).pop() : null;
            let anchor;
            let started = false;
            ed.dragging = true;
            el.classList.add('is-connecting');
            let target = null;
            let travel = 0;
            const originX = event.clientX;
            const originY = event.clientY;
            const update = ev => {
                travel = Math.max(travel, Math.hypot(ev.clientX - originX, ev.clientY - originY));
                if (!started) {
                    if (travel < DRAG_THRESHOLD) return;
                    started = true;
                    if (existing) {
                        ed.model.disconnect([existing.id]);
                        from = { node: existing.source.node, port: existing.source.port };
                        reverse = false;
                        undoTop = true;
                        stopWatch = ed.model.on(() => { undoTop = false; });
                    }
                    const anchorNode = ed.model.node(from.node);
                    if (!anchorNode) return;
                    const outs = ed.model.outputs(anchorNode);
                    anchor = reverse
                        ? G.portPoint(anchorNode.position, 'in', 0, outs.length)
                        : G.portPoint(anchorNode.position, 'out', Math.max(0, outs.indexOf(from.port)), outs.length);
                }
                if (!anchor) return;
                const p = canvas.clientToWorld(ev.clientX, ev.clientY);
                target = portAt(ev.clientX, ev.clientY, reverse ? 'out' : 'in', from.node);
                let state = '';
                if (target) {
                    const check = reverse ? ed.model.canConnect(target.node, target.port, from.node, from.port) : ed.model.canConnect(from.node, from.port, target.node, target.port);
                    state = check.ok ? 'snap' : 'invalid';
                    if (!check.ok) target = Object.assign({}, target, { invalid: check.reason });
                }
                showConnectHint(target && target.invalid ? target.invalid : '', ev.clientX, ev.clientY);
                const end = target && !target.invalid ? target.point : p;
                if (reverse) wires.preview(end, anchor, state); else wires.preview(anchor, end, state);
            };
            update(event);
            capture(event, update, (ev, cancelled) => {
                travel = Math.max(travel, Math.hypot(ev.clientX - originX, ev.clientY - originY));
                el.classList.remove('is-connecting');
                wires.preview(null, null);
                if (stopWatch) stopWatch();
                const refused = !cancelled && started && !!(target && target.invalid);
                if (refused) fadeConnectHint(); else showConnectHint('', 0, 0);
                // A tap never changes the document or its undo/redo history.
                if (!started) {
                    if (!cancelled) ed.bus.emit('connect-picker', { nodeId, port, side });
                    return;
                }
                if (cancelled) { if (undoTop) ed.model.undo(); return; }
                if (target && !target.invalid) {
                    if (reverse) ed.model.connect(target.node, target.port, from.node, from.port);
                    else ed.model.connect(from.node, from.port, target.node, target.port);
                    return;
                }
                if (target && target.invalid) { canvas.announce(core.tr(t, 'easydrag.ui.connect_' + target.invalid, t('easydrag.ui.connect_port'))); return; }
                if (!reverse) ed.bus.emit('quick-add', { x: ev.clientX, y: ev.clientY, from });
            });
        }

        function startBoxSelect(event) {
            const start = { x: event.clientX, y: event.clientY };
            const additive = event.shiftKey || core.isMod(event);
            const base = additive ? new Set(ed.selection) : new Set();
            let active = false;
            let lastHit = null;
            capture(event, ev => {
                if (!active && Math.hypot(ev.clientX - start.x, ev.clientY - start.y) < DRAG_THRESHOLD) return;
                active = true;
                const s = canvas.size();
                const a = { x: start.x - s.left, y: start.y - s.top };
                const b = { x: ev.clientX - s.left, y: ev.clientY - s.top };
                const box = G.normalizeRect(a, b);
                selectBox.hidden = false;
                selectBox.style.transform = 'translate(' + box.x + 'px,' + box.y + 'px)';
                selectBox.style.width = box.w + 'px';
                selectBox.style.height = box.h + 'px';
                const worldBox = G.normalizeRect(G.toWorld(ed.view, a), G.toWorld(ed.view, b));
                // Rects come from the nodes themselves; the selection changes only when the hit set does.
                const hit = ed.model.doc.nodes.filter(n => G.intersects(worldBox, ed.groups ? ed.groups.visualRect(n.id) : G.nodeRect(n.position, ed.model.outputs(n).length))).map(n => n.id);
                const key = hit.join(' ');
                if (key === lastHit) return;
                lastHit = key;
                select(Array.from(new Set([...base, ...hit])));
            }, () => {
                selectBox.hidden = true;
                if (!active) select(Array.from(base));
            });
        }

        // Pans and zooms change only ed.view: the editor stores the view per flow on this device
        // (aurago.easydrag.view.<id>), never in the document, so they neither save nor count as
        // unpublished changes.
        function onWheel(event) {
            event.preventDefault();
            const s = canvas.size();
            if (event.ctrlKey || event.metaKey) {
                const factor = Math.exp(-event.deltaY * (event.deltaMode === 1 ? 0.05 : 0.0025));
                canvas.setView(G.zoomAt(ed.view, ed.view.zoom * factor, { x: event.clientX - s.left, y: event.clientY - s.top }));
            } else {
                const dx = event.shiftKey ? event.deltaY : event.deltaX;
                const dy = event.shiftKey ? 0 : event.deltaY;
                const unit = event.deltaMode === 1 ? 16 : 1;
                canvas.setView({ x: ed.view.x - dx * unit, y: ed.view.y - dy * unit, zoom: ed.view.zoom });
            }
        }

        function onDoubleClick(event) {
            // Pointer capture retargets native click/dblclick to the canvas after a card press.
            // Only that retargeted case needs a fresh hit test; ordinary DOM targets stay intact.
            const hit = event.target === el ? document.elementFromPoint(event.clientX, event.clientY) : event.target;
            if (!hit || !el.contains(hit)) return;
            const target = hit;
            const group = target.closest('[data-ed-group]');
            if (group) { if (ed.groups && target.closest('[data-ed-group-drag]')) ed.groups.open(group.dataset.edGroup); return; }
            const nodeEl = target.closest('.ed-node');
            if (nodeEl && !target.closest('.ed-node-tools, .ed-port')) { ed.bus.emit('open-detail', { nodeId: nodeEl.dataset.nodeId }); return; }
            if (!nodeEl && !readonly() && target.closest('.ed-canvas') && !target.closest('.ed-zoom, .ed-minimap, .ed-empty, .ed-port-tip, .ed-edge, .ed-wire-actions')) {
                ed.bus.emit('quick-add', { x: event.clientX, y: event.clientY });
            }
        }

        // Clicks on empty canvas clear the selection through startBoxSelect; only card tools remain.
        function onClick(event) {
            const tool = event.target.closest('[data-ed-node-tool]');
            if (!tool) return;
            runNodeTool(tool.dataset.edNodeTool, tool.closest('.ed-node').dataset.nodeId);
        }

        // runNodeTool: a card tool selects its card (unless it is the only one selected) and runs the registry action
        // the card menu runs; run() checks the state again, so a tool that looks unavailable does nothing.
        function runNodeTool(tool, id) {
            const found = ED.actions.TOOLS.find(([name]) => name === tool);
            if (!found || !ED.actions.state(found[1], { host: ed, node: id }).enabled) return;
            if (!(ed.selection.size === 1 && ed.selection.has(id))) select([id]);
            ED.actions.run(found[1], { host: ed, node: id });
        }

        // ── commands that say what they did ─────────────────────────────────────

        // duplicateSteps copies steps next to themselves, selects the copies and announces them.
        function duplicateSteps(ids) {
            const added = ed.model.duplicate(ids);
            select(added);
            tell('easydrag.ui.announce_added_one', 'easydrag.ui.announce_added_many', added.map(nameOf));
        }

        // removeSteps deletes steps (one step hands its wires on, bridge) and announces them.
        function removeSteps(ids, quiet) {
            const names = ids.map(nameOf).filter(Boolean);
            ed.model.removeNodes(ids, { bridge: ids.length === 1 });
            select([]);
            if (quiet) return;
            tell('easydrag.ui.announce_deleted_one', 'easydrag.ui.announce_deleted_many', names);
            // The editor offers to take the deletion back (a toast with Undo, easydrag-history.js).
            if (names.length) ed.bus.emit('steps-deleted', { names });
        }

        // toggleSteps turns steps off, or on when all of them are off, and announces which.
        function toggleSteps(ids) {
            const list = ids.map(id => ed.model.node(id)).filter(Boolean);
            if (!list.length) return;
            const off = list.some(n => !n.settings.disabled);
            ed.model.toggleDisabled(list.map(n => n.id));
            const names = list.map(n => n.label || n.type);
            if (off) tell('easydrag.ui.announce_disabled_one', 'easydrag.ui.announce_disabled_many', names);
            else tell('easydrag.ui.announce_enabled_one', 'easydrag.ui.announce_enabled_many', names);
        }

        // undo and redo announce only when there was something to undo or redo.
        function undo() { if (!readonly() && ed.model.undo()) canvas.announce(t('easydrag.ui.announce_undo')); }
        function redo() { if (!readonly() && ed.model.redo()) canvas.announce(t('easydrag.ui.announce_redo')); }

        function onHover(event) {
            const nodeEl = event.target.closest && event.target.closest('.ed-node');
            clearTimeout(hoverTimer);
            const id = nodeEl ? nodeEl.dataset.nodeId : null;
            if (ed.dragging) return;
            hoverTimer = setTimeout(() => { wires.highlight(id); }, id ? 260 : 80);
        }

        // ── context menus ───────────────────────────────────────────────────────

        function openNodeMenu(id, x, y) {
            if (typeof ed.ctx.showContextMenu !== 'function') return;
            if (!ed.selection.has(id)) select([id]);
            ed.ctx.showContextMenu(x, y, ED.actions.items(['step.open', 'step.test', '-', 'edit.duplicate', 'edit.copy', 'edit.group', 'edit.extract', 'step.toggle', '-',
                'step.align_horizontal', 'step.align_vertical', '-', 'edit.delete'], { host: ed, node: id }));
        }

        // The canvas menu pastes and adds at the pointer (x and y are client coordinates).
        function openCanvasMenu(x, y) {
            if (typeof ed.ctx.showContextMenu !== 'function') return;
            ed.ctx.showContextMenu(x, y, ED.actions.items(['edit.add_step', 'edit.paste', 'edit.extract', '-', 'view.find_step', 'edit.auto_layout', 'edit.select_all', 'view.zoom_fit'], { host: ed, x, y }));
        }

        function onContextMenu(event) {
            if (event.target.closest('.ed-zoom, .ed-minimap, .ed-port-tip')) return;
            event.preventDefault();
            const group = event.target.closest('[data-ed-group]');
            if (group && ed.groups) { ed.groups.open(group.dataset.edGroup); return; }
            const nodeEl = event.target.closest('.ed-node');
            if (nodeEl) openNodeMenu(nodeEl.dataset.nodeId, event.clientX, event.clientY);
            else openCanvasMenu(event.clientX, event.clientY);
        }

        // ── clipboard ───────────────────────────────────────────────────────────

        function copySelection() {
            if (!ed.selection.size) return false;
            const frag = ed.model.fragment(Array.from(ed.selection));
            const masked = ED.credentials.mask(frag); // neither the system clipboard nor storage gets a typed credential
            const text = JSON.stringify(masked);
            core.storage.set(CLIPBOARD_KEY, { at: Date.now(), frag: masked });
            if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(text).catch(() => {});
            canvas.announce(t('easydrag.ui.copied', { count: frag.nodes.length }));
            return true;
        }

        async function readClipboard() {
            if (navigator.clipboard && navigator.clipboard.readText) {
                try {
                    const text = await navigator.clipboard.readText();
                    const parsed = JSON.parse(text);
                    if (parsed && parsed.easydrag === 1) return parsed;
                } catch (err) { /* permission denied or not EasyDrag JSON */ }
            }
            return storedFragment();
        }

        // storedFragment returns the fragment kept in localStorage while it is fresh, and removes an
        // expired, unstamped or unreadable one.
        function storedFragment() {
            const kept = core.storage.get(CLIPBOARD_KEY, null);
            if (!kept) return null;
            const at = Number(kept.at);
            const now = Date.now();
            if (kept.frag && Number.isFinite(at) && now - at <= CLIPBOARD_MAX_AGE_MS && at <= now + 60000) return kept.frag;
            core.storage.remove(CLIPBOARD_KEY);
            return null;
        }

        async function pasteAt(point) {
            if (readonly()) return;
            const frag = await readClipboard();
            // Reading the clipboard can wait on a permission prompt; the canvas may be read-only by then.
            if (!frag || readonly()) return;
            const at = point || (lastPointer ? canvas.clientToWorld(lastPointer.x, lastPointer.y) : centerWorld());
            const ids = ed.model.paste(frag, { x: G.snap(at.x), y: G.snap(at.y) });
            // paste returns no ids when it refuses the fragment (over 500 steps or 2000 wires) or
            // keeps none of its steps (unknown types); say so and keep the selection. The editor
            // shows "paste-refused" visibly as well.
            if (!ids.length) {
                canvas.announce(core.tr(t, 'easydrag.ui.paste_refused', t('easydrag.ui.error_generic')));
                ed.bus.emit('paste-refused');
                return;
            }
            select(ids);
            tell('easydrag.ui.announce_added_one', 'easydrag.ui.announce_added_many', ids.map(nameOf));
        }

        function centerWorld() {
            const s = canvas.size();
            return G.toWorld(ed.view, { x: s.w / 2, y: s.h / 2 });
        }

        // removeSelection deletes the selected wire or steps. A cut (quiet) leaves the announcement to
        // its copy: "Copied" already says where the steps went.
        function removeSelection(opts) {
            if (readonly()) return;
            if (ed.selectedEdge) { core.removeConnection(ed, ed.selectedEdge); ed.selectedEdge = null; return; }
            const ids = Array.from(ed.selection);
            if (!ids.length) return;
            removeSteps(ids, !!(opts && opts.quiet));
        }

        // ── keyboard ────────────────────────────────────────────────────────────

        function neighbour(id, key) {
            const n = ed.model.node(id);
            if (!n) return null;
            if (key === 'ArrowRight') { const e = ed.model.outgoing(id)[0]; return e ? e.target.node : null; }
            if (key === 'ArrowLeft') { const e = ed.model.incoming(id)[0]; return e ? e.source.node : null; }
            const column = ed.model.doc.nodes.filter(o => Math.abs(o.position.x - n.position.x) < G.NODE_W).sort((a, b) => a.position.y - b.position.y);
            const idx = column.findIndex(o => o.id === id);
            const next = column[key === 'ArrowDown' ? idx + 1 : idx - 1];
            return next ? next.id : null;
        }

        // handleKey processes the canvas keys that are no action (Space, Escape, the arrows, the active group's Delete
        // and Enter); the actions' shortcuts are ED.actions.key's. It returns true when the event was used.
        function handleKey(event) {
            const key = event.key;
            const target = event.target;
            // Typing, Tab and undo inside a field stay with that field, including IME.
            if (target && target !== el && core.isEditable(target)) return false;
            // Tab, Space and Enter on a button inside the canvas (zoom, node list, card tools) keep their
            // default: the button is pressed or focus moves on, so the canvas is no keyboard trap.
            if ((key === 'Tab' || key === ' ' || key === 'Enter') && target && target !== el && target.closest && target.closest('button, a, input, textarea, select')) return false;
            if (key === 'Tab') return false;
            const one = ed.selection.size === 1 ? Array.from(ed.selection)[0] : null;
            if (key === ' ' && !event.repeat) { spaceDown = true; el.classList.add('is-space'); return true; }
            if (core.isMod(event)) return false;
            if (ed.groups && ed.groups.active && (key === 'Delete' || key === 'Backspace' || key === 'Enter')) {
                if (key === 'Enter') ed.groups.open(ed.groups.active);
                else if (!event.repeat && !readonly() && (!ed.flushPendingEdits || ed.flushPendingEdits())) { ed.model.ungroup(ed.groups.active); select([]); }
                return true;
            }
            if (key === 'Escape') { select([]); wires.highlight(null); return true; }
            if (key.startsWith('Arrow')) {
                if (event.shiftKey && ed.selection.size && !readonly()) {
                    const d = event.altKey ? 1 : G.GRID;
                    const delta = { ArrowLeft: [-d, 0], ArrowRight: [d, 0], ArrowUp: [0, -d], ArrowDown: [0, d] }[key];
                    movedIds = ed.groups ? ed.groups.movable(ed.selection) : Array.from(ed.selection);
                    ed.model.moveNodes(movedIds, delta[0], delta[1]);
                    movedSoon();
                    return true;
                }
                const start = one || (ed.model.doc.nodes[0] && ed.model.doc.nodes[0].id);
                if (!start) return true;
                const next = one ? neighbour(one, key) : start;
                if (next) {
                    select([next]);
                    canvas.centerOn(next, { animate: true });
                    const n = ed.model.node(next);
                    canvas.announce((n.label || n.type) + ' · ' + t('easydrag.ui.node_focused'));
                }
                return true;
            }
            return false;
        }

        function handleKeyUp(event) {
            if (event.key === ' ') { spaceDown = false; el.classList.remove('is-space'); }
        }

        // ── actions ─────────────────────────────────────────────────────────────

        // The registry (easydrag-actions.js) holds every action's label, shortcut and reasons; the editing actions
        // are bound here. The context's node is the card a menu or tool belongs to, else the one selected step.
        const stepOf = c => (c && c.node) || (ed.selection.size === 1 ? Array.from(ed.selection)[0] : '');
        const noStep = c => (stepOf(c) ? '' : 'easydrag.ui.reason_no_selection');
        const grouped = () => !!(ed.groups && ed.groups.active);
        const bindAction = (id, impl) => ED.actions.bind(id, impl, ed);
        bindAction('edit.undo', { run: () => undo(), check: () => (ed.model.canUndo() ? '' : 'easydrag.ui.reason_nothing_to_undo') });
        bindAction('edit.redo', { run: () => redo(), check: () => (ed.model.canRedo() ? '' : 'easydrag.ui.reason_nothing_to_redo') });
        bindAction('edit.cut', { run: () => { if (copySelection()) removeSelection({ quiet: true }); } });
        bindAction('edit.copy', { run: () => copySelection() });
        bindAction('edit.paste', {
            run: c => pasteAt(c.x === undefined ? null : canvas.clientToWorld(c.x, c.y)),
            check: () => (navigator.clipboard && navigator.clipboard.readText) || storedFragment() ? '' : 'easydrag.ui.reason_nothing_to_paste'
        });
        bindAction('edit.duplicate', { run: () => duplicateSteps(Array.from(ed.selection)) });
        bindAction('edit.delete', { run: () => removeSelection(), accepts: () => !grouped(), check: c => (c.node || ed.selection.size || ed.selectedEdge ? '' : 'easydrag.ui.reason_no_selection') });
        bindAction('edit.select_all', { run: () => selectAll() });
        bindAction('edit.group', { run: () => ed.groups.createFromSelection(), show: () => !!ed.groups });
        bindAction('edit.add_step', { run: c => ed.bus.emit('quick-add', { x: c.x, y: c.y }) });
        bindAction('edit.auto_layout', { run: () => canvas.autoLayout() });
        bindAction('step.open', { run: c => ed.bus.emit('open-detail', { nodeId: stepOf(c) }), check: noStep, accepts: (event, where) => !grouped() && !where.control });
        bindAction('step.connect', { run: c => ed.bus.emit('connect-picker', { nodeId: stepOf(c) }), check: noStep });
        bindAction('step.toggle', {
            run: () => toggleSteps(Array.from(ed.selection)),
            label: c => { const n = ed.model.node(stepOf(c)); return n ? (n.settings.disabled ? 'easydrag.ui.node_enable' : 'easydrag.ui.node_disable') : ''; }
        });
        bindAction('step.test', { run: c => ed.bus.emit('node-test', { nodeId: stepOf(c) }), show: () => !ed.isBlock });
        bindAction('step.add_next', {
            run: c => {
                const id = stepOf(c), card = canvas.nodeEl(id), n = ed.model.node(id), outs = n ? ed.model.outputs(n) : [];
                if (!card || !outs.length) return;
                const rect = card.getBoundingClientRect();
                ed.bus.emit('quick-add', { x: rect.right + 8, y: rect.top, from: { node: id, port: outs[0] } });
            },
            check: c => { const n = ed.model.node(stepOf(c)); return n && ed.model.outputs(n).length ? '' : 'easydrag.ui.reason_no_output'; }
        });
        bindAction('step.align_horizontal', { run: () => canvas.alignSelection('horizontal'), show: () => ed.selection.size >= 2 });
        bindAction('step.align_vertical', { run: () => canvas.alignSelection('vertical'), show: () => ed.selection.size >= 2 });
        bindAction('view.zoom_in', { run: () => canvas.zoomBy(1.2) });
        bindAction('view.zoom_out', { run: () => canvas.zoomBy(1 / 1.2) });
        bindAction('view.zoom_fit', { run: () => canvas.fit({ animate: true }) });
        bindAction('view.zoom_reset', { run: () => { const s = canvas.size(); canvas.setView(G.zoomAt(ed.view, 1, { x: s.w / 2, y: s.h / 2 }), { animate: true }); } });
        // A card's tools are painted when its markup is built; a run, a dialog or quick-add changes which may act: every card repaints its own.
        function paintTools() {
            ed.model.doc.nodes.forEach(n => {
                const card = canvas.nodeEl(n.id);
                if (card) card.querySelectorAll('[data-ed-node-tool]').forEach(button => ED.actions.TOOLS.filter(([tool]) => tool === button.dataset.edNodeTool).forEach(([, action]) => ED.actions.paint(button, action, { host: ed, node: n.id }, { name: true })));
            });
        }
        bag.add(ed.bus.on('run', paintTools));

        // An expired fragment goes as soon as an editor opens, not only at the next paste.
        storedFragment();
        bag.listen(el, 'pointerdown', onPointerDown);
        bag.listen(el, 'pointermove', onPointerMoveAny);
        bag.listen(el, 'pointerup', onPointerUpAny);
        bag.listen(el, 'pointercancel', onPointerUpAny);
        bag.listen(el, 'lostpointercapture', onPointerUpAny);
        bag.listen(el, 'pointerover', onHover);
        bag.listen(el, 'pointerleave', () => { clearTimeout(hoverTimer); wires.highlight(null); });
        bag.listen(el, 'wheel', onWheel, { passive: false });
        bag.listen(el, 'dblclick', onDoubleClick);
        bag.listen(el, 'click', onClick);
        bag.listen(el, 'contextmenu', onContextMenu);
        bag.listen(el, 'focusout', () => { spaceDown = false; el.classList.remove('is-space'); });
        bag.add(() => { clearTimeout(hoverTimer); clearTimeout(longPress); clearTimeout(hintTimer); movedSoon.cancel(); });
        // Added last, so it runs first on dispose: a gesture still in progress is cancelled.
        bag.add(() => { if (gesture) gesture.abort(); });

        return {
            select, selectAll, paintTools, handleKey, handleKeyUp, copySelection, pasteAt, removeSelection, duplicateSteps, undo, redo, edgeNear,
            lastPointer: () => lastPointer,
            // abortGesture cancels a pan, drag, connect or box select in progress (a drag moves back).
            abortGesture() { if (gesture) gesture.abort(); },
            dispose() { bag.dispose(); guides.remove(); }
        };
    }

    ED.interact = { create };
})();
