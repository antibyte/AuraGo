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
        const pointers = new Map();
        let spaceDown = false;
        let lastPointer = null;
        let hoverTimer = 0;
        let pinch = null;
        let longPress = 0;
        let gesture = null; // the pan, drag, connect or box select that follows a pointer now

        const readonly = () => !!(ed.readonly || ed.runView);

        // ── selection ───────────────────────────────────────────────────────────

        function select(ids, opts) {
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
                if (n.id === except) return;
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
            const byId = new Map(ed.model.doc.nodes.map(n => [n.id, n]));
            const reach = 18 / ed.view.zoom;
            let best = null;
            ed.model.doc.edges.forEach(e => {
                if (e.source.node === exceptNode || e.target.node === exceptNode) return;
                const src = byId.get(e.source.node);
                const dst = byId.get(e.target.node);
                if (!src || !dst) return;
                const outs = ed.model.outputs(src);
                const a = G.portPoint(src.position, 'out', Math.max(0, outs.indexOf(e.source.port)), outs.length);
                const b = G.portPoint(dst.position, 'in', 0, ed.model.outputs(dst).length);
                const box = G.wireBounds(a, b);
                if (worldPoint.x < box.x - reach || worldPoint.x > box.x + box.w + reach || worldPoint.y < box.y - reach || worldPoint.y > box.y + box.h + reach) return;
                const d = G.distanceToWire(worldPoint, a, b);
                if (d < reach && (!best || d < best.d)) best = { id: e.id, d };
            });
            return best;
        }

        // ── pointer handling ────────────────────────────────────────────────────

        function onPointerDown(event) {
            if (event.target.closest('.ed-zoom, .ed-minimap, .ed-empty, .ed-wire-actions, .ed-node-tools')) return;
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
            const panGesture = event.button === 1 || spaceDown || (event.pointerType === 'touch' && !nodeEl);
            if (panGesture) { startPan(event); return; }
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

        function startPan(event) {
            event.preventDefault();
            const start = { x: event.clientX, y: event.clientY, view: Object.assign({}, ed.view) };
            el.classList.add('is-panning');
            capture(event, ev => {
                if (pinch) return;
                canvas.setView({ x: start.view.x + ev.clientX - start.x, y: start.view.y + ev.clientY - start.y, zoom: start.view.zoom });
            }, () => { el.classList.remove('is-panning'); });
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

        function startNodeDrag(event, nodeId) {
            const additive = event.shiftKey || core.isMod(event);
            if (additive) select([nodeId], { toggle: true });
            else if (!ed.selection.has(nodeId)) select([nodeId]);
            if (readonly() || additive) return;
            const ids = Array.from(ed.selection);
            const origin = new Map(ids.map(id => [id, Object.assign({}, ed.model.node(id).position)]));
            const start = { x: event.clientX, y: event.clientY };
            const primary = ed.model.node(nodeId);
            let moved = false;
            let applied = { x: 0, y: 0 };
            let dropEdge = null;
            if (event.pointerType === 'touch') {
                longPress = setTimeout(() => { if (!moved) openNodeMenu(nodeId, start.x, start.y); }, 550);
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
                if (ids.length === 1 && !ed.model.incoming(nodeId).length && !ed.model.outgoing(nodeId).length && ed.model.inputs(primary).length) {
                    const rect = canvas.nodeRect(nodeId);
                    const near = edgeNear({ x: rect.x + rect.w / 2, y: rect.y + rect.h / 2 }, nodeId);
                    dropEdge = near ? near.id : null;
                }
                wires.markDropTarget(dropEdge);
            }, (ev, cancelled) => {
                clearTimeout(longPress);
                el.classList.remove('is-dragging');
                hideGuides();
                wires.markDropTarget(null);
                if (!moved) return;
                if (cancelled) { ed.model.moveNodes(ids, -applied.x, -applied.y); return; }
                const n = ed.model.node(nodeId);
                const grid = { x: G.snap(n.position.x) - n.position.x, y: G.snap(n.position.y) - n.position.y };
                ed.model.moveNodes(ids, grid.x, grid.y);
                if (dropEdge) ed.model.insertOnEdge(dropEdge, nodeId, null);
            });
        }

        // alignTo snaps the dragged card to the left edge or centre line of other cards and shows guides.
        function alignTo(id, pos, moving) {
            const skip = new Set(moving);
            const h = G.nodeHeight(ed.model.outputs(ed.model.node(id)).length);
            let x = pos.x;
            let y = pos.y;
            let gx = null;
            let gy = null;
            ed.model.doc.nodes.forEach(n => {
                if (skip.has(n.id)) return;
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
            if (reverse) {
                const existing = ed.model.incoming(nodeId).filter(e => e.target.port === port).pop();
                if (existing) {
                    ed.model.disconnect([existing.id]);
                    from = { node: existing.source.node, port: existing.source.port };
                    reverse = false;
                    undoTop = true;
                    stopWatch = ed.model.on(() => { undoTop = false; });
                }
            }
            const anchorNode = ed.model.node(from.node);
            const outs = ed.model.outputs(anchorNode);
            const anchor = reverse
                ? G.portPoint(anchorNode.position, 'in', 0, outs.length)
                : G.portPoint(anchorNode.position, 'out', Math.max(0, outs.indexOf(from.port)), outs.length);
            ed.dragging = true;
            el.classList.add('is-connecting');
            let target = null;
            const update = ev => {
                const p = canvas.clientToWorld(ev.clientX, ev.clientY);
                target = portAt(ev.clientX, ev.clientY, reverse ? 'out' : 'in', from.node);
                let state = '';
                if (target) {
                    const check = reverse ? ed.model.canConnect(target.node, target.port, from.node, from.port) : ed.model.canConnect(from.node, from.port, target.node, target.port);
                    state = check.ok ? 'snap' : 'invalid';
                    if (!check.ok) target = Object.assign({}, target, { invalid: check.reason });
                }
                const end = target && !target.invalid ? target.point : p;
                if (reverse) wires.preview(end, anchor, state); else wires.preview(anchor, end, state);
            };
            update(event);
            capture(event, update, (ev, cancelled) => {
                el.classList.remove('is-connecting');
                wires.preview(null, null);
                if (stopWatch) stopWatch();
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
                const hit = ed.model.doc.nodes.filter(n => G.intersects(worldBox, G.nodeRect(n.position, ed.model.outputs(n).length))).map(n => n.id);
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
            const nodeEl = event.target.closest('.ed-node');
            if (nodeEl && !event.target.closest('.ed-node-tools, .ed-port')) { ed.bus.emit('open-detail', { nodeId: nodeEl.dataset.nodeId }); return; }
            if (!nodeEl && !readonly() && event.target.closest('.ed-canvas') && !event.target.closest('.ed-zoom, .ed-minimap, .ed-empty')) {
                ed.bus.emit('quick-add', { x: event.clientX, y: event.clientY });
            }
        }

        // Clicks on empty canvas clear the selection through startBoxSelect; only card tools remain.
        function onClick(event) {
            const tool = event.target.closest('[data-ed-node-tool]');
            if (!tool) return;
            runNodeTool(tool.dataset.edNodeTool, tool.closest('.ed-node').dataset.nodeId);
        }

        function runNodeTool(action, id) {
            if (readonly()) return;
            if (action === 'test') ed.bus.emit('node-test', { nodeId: id });
            else if (action === 'disable') ed.model.toggleDisabled([id]);
            else if (action === 'duplicate') select(ed.model.duplicate([id]));
            else if (action === 'delete') { ed.model.removeNodes([id], { bridge: true }); select([]); }
        }

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
            const n = ed.model.node(id);
            const ro = readonly();
            ed.ctx.showContextMenu(x, y, [
                { icon: 'edit', label: t('easydrag.ui.menu_open_node'), action: () => ed.bus.emit('open-detail', { nodeId: id }) },
                { icon: 'play', label: t('easydrag.ui.node_test'), disabled: ro, action: () => ed.bus.emit('node-test', { nodeId: id }) },
                { separator: true },
                { icon: 'copy', label: t('easydrag.ui.node_duplicate'), disabled: ro, action: () => select(ed.model.duplicate(Array.from(ed.selection))) },
                { icon: 'copy', label: t('easydrag.ui.menu_copy'), action: () => copySelection() },
                { icon: 'pause', label: n && n.settings.disabled ? t('easydrag.ui.node_enable') : t('easydrag.ui.node_disable'), disabled: ro, action: () => ed.model.toggleDisabled(Array.from(ed.selection)) },
                { separator: true },
                { icon: 'trash', label: t('easydrag.ui.node_delete'), disabled: ro, action: () => removeSelection() }
            ]);
        }

        function openCanvasMenu(x, y) {
            if (typeof ed.ctx.showContextMenu !== 'function') return;
            const ro = readonly();
            ed.ctx.showContextMenu(x, y, [
                { icon: 'plus', label: t('easydrag.ui.menu_add_node'), disabled: ro, action: () => ed.bus.emit('quick-add', { x, y }) },
                { icon: 'copy', label: t('easydrag.ui.menu_paste'), disabled: ro, action: () => pasteAt(canvas.clientToWorld(x, y)) },
                { separator: true },
                { icon: 'list', label: t('easydrag.ui.menu_select_all'), action: selectAll },
                { icon: 'refresh', label: t('easydrag.ui.zoom_fit'), action: () => canvas.fit({ animate: true }) }
            ]);
        }

        function onContextMenu(event) {
            if (event.target.closest('.ed-zoom, .ed-minimap')) return;
            event.preventDefault();
            const nodeEl = event.target.closest('.ed-node');
            if (nodeEl) openNodeMenu(nodeEl.dataset.nodeId, event.clientX, event.clientY);
            else openCanvasMenu(event.clientX, event.clientY);
        }

        // ── clipboard ───────────────────────────────────────────────────────────

        function copySelection() {
            if (!ed.selection.size) return false;
            const frag = ed.model.fragment(Array.from(ed.selection));
            const text = JSON.stringify(frag);
            core.storage.set(CLIPBOARD_KEY, frag);
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
            return core.storage.get(CLIPBOARD_KEY, null);
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
        }

        function centerWorld() {
            const s = canvas.size();
            return G.toWorld(ed.view, { x: s.w / 2, y: s.h / 2 });
        }

        function removeSelection() {
            if (readonly()) return;
            if (ed.selectedEdge) { ed.model.disconnect([ed.selectedEdge]); ed.selectedEdge = null; return; }
            const ids = Array.from(ed.selection);
            if (!ids.length) return;
            ed.model.removeNodes(ids, { bridge: ids.length === 1 });
            select([]);
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

        // handleKey processes canvas shortcuts; it returns true when the event was used.
        function handleKey(event) {
            const key = event.key;
            const target = event.target;
            // Tab, Space and Enter on a button inside the canvas (zoom, node list, card tools) keep their
            // default: the button is pressed or focus moves on, so the canvas is no keyboard trap.
            if ((key === 'Tab' || key === ' ' || key === 'Enter') && target && target !== el && target.closest && target.closest('button, a, input')) return false;
            const mod = core.isMod(event);
            const one = ed.selection.size === 1 ? Array.from(ed.selection)[0] : null;
            if (key === ' ' && !event.repeat) { spaceDown = true; el.classList.add('is-space'); return true; }
            if (mod && key.toLowerCase() === 'z' && !event.shiftKey) { if (!readonly()) ed.model.undo(); return true; }
            if (mod && (key.toLowerCase() === 'y' || (key.toLowerCase() === 'z' && event.shiftKey))) { if (!readonly()) ed.model.redo(); return true; }
            if (mod && key.toLowerCase() === 'a') { selectAll(); return true; }
            if (mod && key.toLowerCase() === 'c') return copySelection();
            if (mod && key.toLowerCase() === 'x') { if (copySelection()) removeSelection(); return true; }
            if (mod && key.toLowerCase() === 'v') { pasteAt(null); return true; }
            if (mod && key.toLowerCase() === 'd') { if (!readonly() && ed.selection.size) select(ed.model.duplicate(Array.from(ed.selection))); return true; }
            if (mod && key === '0') { const s = canvas.size(); canvas.setView(G.zoomAt(ed.view, 1, { x: s.w / 2, y: s.h / 2 }), { animate: true }); return true; }
            if (mod) return false;
            if (key === 'Delete' || key === 'Backspace') { removeSelection(); return true; }
            if (key === 'Escape') { select([]); wires.highlight(null); return true; }
            if (key === 'Tab') {
                // Shift+Tab, and Tab on a read-only canvas, move focus on as usual.
                if (event.shiftKey || readonly()) return false;
                const src = one && ed.model.node(one);
                const outs = src ? ed.model.outputs(src) : [];
                if (src && outs.length) {
                    // Tab on a selected step appends the next step after it.
                    const r = canvas.nodeEl(one).getBoundingClientRect();
                    ed.bus.emit('quick-add', { x: r.right + 24, y: r.top, from: { node: one, port: outs[0] } });
                    return true;
                }
                const p = lastPointer && el.matches(':hover') ? lastPointer : null;
                const s = canvas.size();
                ed.bus.emit('quick-add', p ? { x: p.x, y: p.y } : { x: s.left + s.w / 2, y: s.top + s.h / 2 });
                return true;
            }
            if (key === '!' || (key === '1' && event.shiftKey)) { canvas.fit({ animate: true }); return true; }
            if (key === '+' || key === '=') { canvas.zoomBy(1.2); return true; }
            if (key === '-') { canvas.zoomBy(1 / 1.2); return true; }
            if ((key === 'd' || key === 'D') && ed.selection.size) { if (!readonly()) ed.model.toggleDisabled(Array.from(ed.selection)); return true; }
            if (key === 'Enter' && one) { ed.bus.emit('open-detail', { nodeId: one }); return true; }
            if ((key === 'c' || key === 'C') && one && !readonly()) { ed.bus.emit('connect-picker', { nodeId: one }); return true; }
            if (key.startsWith('Arrow')) {
                if (event.shiftKey && ed.selection.size && !readonly()) {
                    const d = event.altKey ? 1 : G.GRID;
                    const delta = { ArrowLeft: [-d, 0], ArrowRight: [d, 0], ArrowUp: [0, -d], ArrowDown: [0, d] }[key];
                    ed.model.moveNodes(Array.from(ed.selection), delta[0], delta[1]);
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
        bag.add(() => { clearTimeout(hoverTimer); clearTimeout(longPress); });
        // Added last, so it runs first on dispose: a gesture still in progress is cancelled.
        bag.add(() => { if (gesture) gesture.abort(); });

        return {
            select, selectAll, handleKey, handleKeyUp, copySelection, pasteAt, removeSelection,
            lastPointer: () => lastPointer,
            // abortGesture cancels a pan, drag, connect or box select in progress (a drag moves back).
            abortGesture() { if (gesture) gesture.abort(); },
            dispose() { bag.dispose(); guides.remove(); }
        };
    }

    ED.interact = { create };
})();
