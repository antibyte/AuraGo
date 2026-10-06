// EasyDrag wires: one SVG layer under the cards, hover actions (insert, delete), path
// highlight, item-count pills after runs, flow animation and the connection preview.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const SVG = 'http://www.w3.org/2000/svg';

    function create(ed, canvas) {
        const core = ED.core;
        const G = ED.geometry;
        const { t, esc } = ed;
        const bag = core.bag();
        const layer = canvas.wiresSvg.querySelector('.ed-wires-layer');
        const pills = canvas.wiresSvg.querySelector('.ed-wires-pills');
        const previewPath = canvas.wiresSvg.querySelector('.ed-wire-preview');
        const groups = new Map();
        let hoverEdge = null;
        let hideTimer = 0;
        let pathNode = null;
        let pathSet = new Set();
        let dropTarget = null;

        const actions = core.el('<div class="ed-wire-actions" hidden>' +
            '<button type="button" class="ed-wire-btn" data-ed-wire="insert" title="' + esc(t('easydrag.ui.wire_insert')) + '" aria-label="' + esc(t('easydrag.ui.wire_insert')) + '">' + core.icon('plus') + '</button>' +
            '<button type="button" class="ed-wire-btn ed-wire-btn--danger" data-ed-wire="delete" title="' + esc(t('easydrag.ui.wire_delete')) + '" aria-label="' + esc(t('easydrag.ui.wire_delete')) + '">' + core.icon('x') + '</button></div>');
        canvas.world.appendChild(actions);

        function ends(e) {
            const src = ed.model.node(e.source.node);
            const dst = ed.model.node(e.target.node);
            if (!src || !dst) return null;
            const outs = ed.model.outputs(src);
            const idx = Math.max(0, outs.indexOf(e.source.port));
            return {
                a: G.portPoint(src.position, 'out', idx, outs.length),
                b: G.portPoint(dst.position, 'in', 0, ed.model.outputs(dst).length),
                src, dst
            };
        }

        function edgeState(e) {
            const run = ed.run && ed.run.edges && ed.run.edges.get(e.id);
            return run ? run.state : '';
        }

        function drawEdge(e) {
            const geo = ends(e);
            let g = groups.get(e.id);
            if (!geo) { if (g) { g.remove(); groups.delete(e.id); } return; }
            if (!g) {
                g = document.createElementNS(SVG, 'g');
                g.setAttribute('class', 'ed-edge');
                g.dataset.edgeId = e.id;
                const hit = document.createElementNS(SVG, 'path');
                hit.setAttribute('class', 'ed-wire-hit');
                const line = document.createElementNS(SVG, 'path');
                line.setAttribute('class', 'ed-wire');
                g.appendChild(hit);
                g.appendChild(line);
                layer.appendChild(g);
                groups.set(e.id, g);
            }
            const d = G.wirePath(geo.a, geo.b);
            g.firstChild.setAttribute('d', d);
            g.lastChild.setAttribute('d', d);
            const state = edgeState(e);
            const cls = ['ed-edge'];
            if (state) cls.push('is-' + state);
            if (e.source.port === 'error') cls.push('is-error-port');
            if (geo.src.settings.disabled || geo.dst.settings.disabled) cls.push('is-disabled');
            if (ed.selectedEdge === e.id) cls.push('is-selected');
            if (dropTarget === e.id) cls.push('is-drop-target');
            if (pathNode && pathSet.has(e.source.node) && pathSet.has(e.target.node)) cls.push('is-path');
            g.setAttribute('class', cls.join(' '));
            g.dataset.cat = core.catOf(ed.model.info(geo.src.type));
        }

        function render() {
            const present = new Set(ed.model.doc.edges.map(e => e.id));
            Array.from(groups.keys()).forEach(id => { if (!present.has(id)) { groups.get(id).remove(); groups.delete(id); } });
            ed.model.doc.edges.forEach(drawEdge);
            drawPills();
            if (hoverEdge && !present.has(hoverEdge)) hideActions(true);
        }

        function renderFor(nodeIds) {
            const set = new Set(nodeIds || []);
            ed.model.doc.edges.forEach(e => { if (set.has(e.source.node) || set.has(e.target.node)) drawEdge(e); });
            drawPills();
            if (hoverEdge) placeActions(hoverEdge);
        }

        function drawPills() {
            pills.innerHTML = '';
            if (!ed.run || !ed.run.edges) return;
            ed.model.doc.edges.forEach(e => {
                const info = ed.run.edges.get(e.id);
                if (!info || !info.count) return;
                const geo = ends(e);
                if (!geo) return;
                const mid = G.wireMidpoint(geo.a, geo.b);
                const text = String(info.count);
                const w = 14 + text.length * 7;
                const g = document.createElementNS(SVG, 'g');
                g.setAttribute('class', 'ed-pill');
                g.setAttribute('transform', 'translate(' + (mid.x - w / 2).toFixed(1) + ',' + (mid.y - 9).toFixed(1) + ')');
                g.innerHTML = '<rect width="' + w + '" height="18" rx="9"></rect><text x="' + (w / 2) + '" y="13" text-anchor="middle">' + esc(text) + '</text>';
                pills.appendChild(g);
            });
        }

        // ── hover actions ───────────────────────────────────────────────────────

        function placeActions(edgeId) {
            const e = ed.model.edge(edgeId);
            const geo = e && ends(e);
            if (!geo) { hideActions(true); return; }
            const mid = G.wireMidpoint(geo.a, geo.b);
            actions.style.transform = 'translate(' + mid.x + 'px,' + mid.y + 'px) translate(-50%,-50%) scale(' + (1 / Math.max(ed.view.zoom, 0.6)) + ')';
        }

        function showActions(edgeId) {
            if (ed.readonly || ed.runView) return;
            clearTimeout(hideTimer);
            hoverEdge = edgeId;
            ed.hoverEdge = edgeId;
            placeActions(edgeId);
            actions.hidden = false;
            groups.forEach((g, id) => g.classList.toggle('is-hover', id === edgeId));
        }

        function hideActions(now) {
            clearTimeout(hideTimer);
            const hide = () => {
                actions.hidden = true;
                hoverEdge = null;
                ed.hoverEdge = null;
                groups.forEach(g => g.classList.remove('is-hover'));
            };
            if (now) hide(); else hideTimer = setTimeout(hide, 220);
        }

        bag.listen(layer, 'pointerover', (event) => {
            const g = event.target.closest('.ed-edge');
            if (g && !ed.dragging) showActions(g.dataset.edgeId);
        });
        bag.listen(layer, 'pointerout', (event) => {
            if (!event.relatedTarget || !event.relatedTarget.closest || !event.relatedTarget.closest('.ed-wire-actions, .ed-edge')) hideActions(false);
        });
        bag.listen(actions, 'pointerenter', () => clearTimeout(hideTimer));
        bag.listen(actions, 'pointerleave', () => hideActions(false));
        bag.listen(actions, 'pointerdown', event => event.stopPropagation());
        bag.listen(actions, 'click', (event) => {
            const btn = event.target.closest('[data-ed-wire]');
            if (!btn || !hoverEdge) return;
            const edgeId = hoverEdge;
            if (btn.dataset.edWire === 'delete') {
                ed.model.disconnect([edgeId]);
                hideActions(true);
                return;
            }
            const rect = btn.getBoundingClientRect();
            ed.bus.emit('quick-add', { x: rect.left + rect.width / 2, y: rect.bottom + 6, edge: edgeId });
            hideActions(true);
        });
        bag.listen(layer, 'click', (event) => {
            const g = event.target.closest('.ed-edge');
            if (!g) return;
            event.stopPropagation();
            ed.selectedEdge = g.dataset.edgeId;
            ed.selection.clear();
            ed.bus.emit('selection', ed.selection);
            render();
        });

        // highlight marks the whole path through a node (ancestors and descendants).
        function highlight(nodeId) {
            pathNode = nodeId || null;
            pathSet = new Set();
            if (pathNode && ed.model.node(pathNode)) {
                pathSet = new Set([pathNode, ...ed.model.upstream(pathNode), ...ed.model.downstream(pathNode)]);
            }
            canvas.el.classList.toggle('has-path', !!pathNode);
            ed.model.doc.edges.forEach(drawEdge);
        }

        // markDropTarget marks the wire a dragged card would be inserted into (null clears it). It looks
        // the wire up by id in groups: edge ids come from documents and never go into a selector.
        function markDropTarget(edgeId) {
            const next = edgeId || null;
            if (next === dropTarget) return;
            const prev = dropTarget && groups.get(dropTarget);
            if (prev) prev.classList.remove('is-drop-target');
            dropTarget = next;
            const g = next && groups.get(next);
            if (g) g.classList.add('is-drop-target');
        }

        // preview draws the wire that follows the pointer while connecting.
        function preview(a, b, state) {
            if (!a || !b) { previewPath.hidden = true; return; }
            previewPath.hidden = false;
            previewPath.setAttribute('d', G.wirePath(a, b));
            previewPath.setAttribute('class', 'ed-wire-preview' + (state ? ' is-' + state : ''));
        }

        bag.add(ed.bus.on('model', change => {
            if (change.kind === 'viewport') return;
            if (change.structural || change.kind !== 'change' || change.meta) render();
            else renderFor(change.nodes);
        }));
        bag.add(ed.bus.on('run', render));
        bag.add(ed.bus.on('selection', () => { if (ed.selection.size) { ed.selectedEdge = null; } render(); }));
        bag.add(ed.bus.on('view', () => { if (hoverEdge) placeActions(hoverEdge); }));

        return {
            render, renderFor, highlight, preview, markDropTarget,
            hideActions: () => hideActions(true),
            dispose() { clearTimeout(hideTimer); bag.dispose(); actions.remove(); }
        };
    }

    ED.wires = { create };
})();
