// EasyDrag canvas: viewport (one CSS transform), dot grid, node cards, zoom control,
// minimap, empty state and an accessible node list.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    function create(ed) {
        const core = ED.core;
        const G = ED.geometry;
        const { t, esc } = ed;
        const bag = core.bag();
        const nodeEls = new Map();
        const signatures = new Map();

        const el = core.el(
            '<div class="ed-canvas" tabindex="0" role="application" aria-roledescription="' + esc(t('easydrag.ui.canvas_role')) + '" aria-label="' + esc(t('easydrag.ui.canvas_label')) + '">' +
            '<div class="ed-grid" aria-hidden="true"></div>' +
            '<div class="ed-world">' +
            '<svg class="ed-wires" overflow="visible" width="1" height="1" aria-hidden="true"><g class="ed-wires-layer"></g><g class="ed-wires-pills"></g><path class="ed-wire-preview" d="" hidden></path></svg>' +
            '<div class="ed-nodes"></div>' +
            '</div>' +
            '<div class="ed-overlay" aria-hidden="true"><div class="ed-select-box" hidden></div></div>' +
            '<div class="ed-empty" hidden><div class="ed-empty-card">' + core.icon('bolt', 'ed-empty-icon') +
            '<h3>' + esc(t('easydrag.ui.empty_title')) + '</h3><p>' + esc(t('easydrag.ui.empty_text')) + '</p>' +
            '<button type="button" class="ed-btn ed-btn--primary" data-ed-empty-add>' + core.icon('plus') + '<span>' + esc(t('easydrag.ui.empty_action')) + '</span></button></div></div>' +
            '<div class="ed-zoom" role="toolbar" aria-label="' + esc(t('easydrag.ui.zoom_label')) + '">' +
            '<button type="button" class="ed-icon-btn" data-ed-zoom="out" title="' + esc(t('easydrag.ui.zoom_out')) + '" aria-label="' + esc(t('easydrag.ui.zoom_out')) + '">' + core.icon('minus') + '</button>' +
            '<button type="button" class="ed-zoom-value" data-ed-zoom="reset" title="' + esc(t('easydrag.ui.zoom_reset')) + '">100%</button>' +
            '<button type="button" class="ed-icon-btn" data-ed-zoom="in" title="' + esc(t('easydrag.ui.zoom_in')) + '" aria-label="' + esc(t('easydrag.ui.zoom_in')) + '">' + core.icon('plus') + '</button>' +
            '<button type="button" class="ed-icon-btn" data-ed-zoom="fit" title="' + esc(t('easydrag.ui.zoom_fit')) + ' (' + esc(core.shortcut('Shift+1')) + ')" aria-label="' + esc(t('easydrag.ui.zoom_fit')) + '">' + core.icon('fit') + '</button>' +
            '</div>' +
            '<div class="ed-minimap" title="' + esc(t('easydrag.ui.minimap')) + '"><svg viewBox="0 0 200 130" preserveAspectRatio="xMidYMid meet"><g class="ed-minimap-nodes"></g><rect class="ed-minimap-view" rx="3"></rect></svg></div>' +
            '<ul class="ed-sr-only ed-node-list" aria-label="' + esc(t('easydrag.ui.node_list')) + '"></ul>' +
            '<div class="ed-sr-only" aria-live="polite" data-ed-live></div>' +
            '</div>');

        const grid = el.querySelector('.ed-grid');
        const world = el.querySelector('.ed-world');
        const nodesHost = el.querySelector('.ed-nodes');
        const wiresSvg = el.querySelector('.ed-wires');
        const emptyEl = el.querySelector('.ed-empty');
        const zoomValue = el.querySelector('.ed-zoom-value');
        const minimap = el.querySelector('.ed-minimap svg');
        const listEl = el.querySelector('.ed-node-list');
        const liveEl = el.querySelector('[data-ed-live]');

        const viewFrame = core.frame(applyView);
        const minimapFrame = core.frame(drawMinimap);
        const listUpdate = core.debounce(drawList, 400);

        function info(n) { return ed.model.info(n.type); }

        function outCount(n) { return ed.model.outputs(n).length; }

        function nodeRect(id) {
            const n = ed.model.node(id);
            return n ? G.nodeRect(n.position, outCount(n)) : null;
        }

        function size() {
            const rect = el.getBoundingClientRect();
            return { w: rect.width || 800, h: rect.height || 600, left: rect.left, top: rect.top };
        }

        function clientToWorld(clientX, clientY) {
            const s = size();
            return G.toWorld(ed.view, { x: clientX - s.left, y: clientY - s.top });
        }

        function applyView() {
            const v = ed.view;
            world.style.transform = 'translate(' + v.x + 'px,' + v.y + 'px) scale(' + v.zoom + ')';
            const step = 24 * v.zoom;
            grid.style.backgroundSize = step + 'px ' + step + 'px';
            grid.style.backgroundPosition = v.x + 'px ' + v.y + 'px';
            el.classList.toggle('is-zoomed-out', v.zoom < 0.55);
            zoomValue.textContent = Math.round(v.zoom * 100) + '%';
            minimapFrame.request();
        }

        // setView coerces the view: an x or y that is not a finite number becomes 0 and clampZoom
        // maps a bad zoom to 1, so a corrupt stored viewport or a NaN cannot reach the transforms.
        function setView(view, opts) {
            const v = view && typeof view === 'object' ? view : {};
            ed.view = { x: Number.isFinite(v.x) ? v.x : 0, y: Number.isFinite(v.y) ? v.y : 0, zoom: G.clampZoom(v.zoom) };
            el.classList.toggle('is-animating', !!(opts && opts.animate) && !matchMedia('(prefers-reduced-motion: reduce)').matches);
            if (opts && opts.animate) setTimeout(() => el.classList.remove('is-animating'), 260);
            viewFrame.request();
            ed.bus.emit('view', ed.view);
        }

        function allRects() { return ed.model.doc.nodes.map(n => G.nodeRect(n.position, outCount(n))); }

        function fit(opts) {
            setView(G.fit(G.bounds(allRects()), size(), 72, 1), opts);
        }

        function zoomBy(factor, center) {
            const s = size();
            const point = center || { x: s.w / 2, y: s.h / 2 };
            setView(G.zoomAt(ed.view, ed.view.zoom * factor, point));
        }

        function centerOn(id, opts) {
            const rect = nodeRect(id);
            if (!rect) return;
            const s = size();
            const zoom = Math.max(ed.view.zoom, 0.8);
            setView({ x: s.w / 2 - (rect.x + rect.w / 2) * zoom, y: s.h / 2 - (rect.y + rect.h / 2) * zoom, zoom }, opts);
        }

        // ── node cards ──────────────────────────────────────────────────────────

        function displayValue(n, i, name) {
            const spec = (i.params || []).find(p => p.name === name);
            let value = n.params[name];
            if (value === undefined && spec) value = spec.default;
            if (value === undefined || value === null || value === '') return '';
            if (spec && Array.isArray(spec.options)) {
                const opt = spec.options.find(o => String(o.value) === String(value));
                if (opt) return opt.label;
            }
            if (typeof value === 'object') return Array.isArray(value) ? t('easydrag.ui.summary_items', { count: value.length }) : '';
            let text = String(value).replace(/\{\{\s*([a-z][a-z0-9_]*)([^}]*)\}\}/g, (m, root, rest) => {
                const src = ed.model.byKey(root);
                const label = src ? src.label : root;
                const field = rest.replace(/\|.*$/, '').trim().replace(/^\./, '');
                return '‹' + label + (field ? ' › ' + field : '') + '›';
            });
            text = text.replace(/\s+/g, ' ').trim();
            return text.length > 56 ? text.slice(0, 55) + '…' : text;
        }

        function summary(n, i) {
            if (!i) return n.type;
            const text = String(i.summary || '').replace(/\{([a-z_]+)\}/g, (m, name) => displayValue(n, i, name)).trim();
            return text.replace(/^[·\s]+|[·\s]+$/g, '');
        }

        function statusOf(id) {
            const step = ed.run && ed.run.steps && ed.run.steps.get(id);
            return step ? step.status : '';
        }

        function nodeIssues(id) { return (ed.issues || []).filter(is => is.node_id === id && is.severity === 'error'); }

        function cardMarkup(n) {
            const i = info(n);
            const ports = ed.model.outputs(n);
            const h = G.nodeHeight(ports.length);
            const avail = i ? (i.availability || {}).state : 'missing';
            const status = statusOf(n.id);
            const step = ed.run && ed.run.steps && ed.run.steps.get(n.id);
            const badges = [];
            if (!i) badges.push('<span class="ed-badge ed-badge--warn" title="' + esc(t('easydrag.ui.badge_unknown')) + '">' + core.icon('alert') + '</span>');
            else if (avail === 'needs_setup') badges.push('<span class="ed-badge ed-badge--warn" title="' + esc(t('easydrag.ui.badge_needs_setup')) + '">' + core.icon('settings') + '</span>');
            else if (avail === 'blocked') badges.push('<span class="ed-badge ed-badge--warn" title="' + esc(t('easydrag.ui.badge_blocked')) + '">' + core.icon('lock') + '</span>');
            if (i && i.risky) badges.push('<span class="ed-badge ed-badge--risk" title="' + esc(t('easydrag.ui.badge_risky')) + '">' + core.icon('alert') + '</span>');
            if (nodeIssues(n.id).length) badges.push('<span class="ed-badge ed-badge--issue" title="' + esc(t('easydrag.ui.badge_incomplete')) + '"></span>');
            if (status) {
                const label = core.tr(t, 'easydrag.ui.status_' + status, status);
                const dur = step && step.duration_ms ? ' · ' + core.fmt.duration(step.duration_ms) : '';
                badges.push('<span class="ed-status ed-status--' + esc(status) + '" title="' + esc(label + dur) + '"><span class="ed-sr-only">' + esc(label) + '</span></span>');
            }
            const inPorts = ed.model.inputs(n).map(p => '<span class="ed-port ed-port--in" data-ed-port="' + esc(p) + '" data-ed-side="in" style="top:' + (h / 2) + 'px"></span>').join('');
            const outPorts = ports.map((p, idx) => {
                const y = ports.length > 1 ? G.PORT_TOP + idx * G.PORT_PITCH : h / 2;
                const showLabel = ports.length > 1 || p !== 'out';
                const label = p.startsWith('case_') ? portCaseLabel(n, p) : core.tr(t, 'easydrag.ui.port_' + p, p);
                return '<span class="ed-port ed-port--out" data-ed-port="' + esc(p) + '" data-ed-side="out" style="top:' + y + 'px">' +
                    (showLabel ? '<span class="ed-port-label">' + esc(label) + '</span>' : '') +
                    '<button type="button" class="ed-port-add" tabindex="-1" data-ed-port-add="' + esc(p) + '" aria-label="' + esc(t('easydrag.ui.port_add')) + '">' + core.icon('plus') + '</button></span>';
            }).join('');
            const ro = ed.readonly || ed.runView;
            const tools = ro ? '' : '<div class="ed-node-tools" role="toolbar">' +
                toolButton('test', 'flask', t('easydrag.ui.node_test')) +
                toolButton('disable', 'eye-off', n.settings.disabled ? t('easydrag.ui.node_enable') : t('easydrag.ui.node_disable')) +
                toolButton('duplicate', 'copy', t('easydrag.ui.node_duplicate')) +
                toolButton('delete', 'trash', t('easydrag.ui.node_delete')) + '</div>';
            const sum = summary(n, i);
            return '<div class="ed-node-tile">' + core.icon(i ? i.icon : 'tool') + '</div>' +
                '<div class="ed-node-body"><div class="ed-node-label">' + esc(n.label || (i && i.label) || n.type) + '</div>' +
                (sum ? '<div class="ed-node-summary">' + esc(sum) + '</div>' : '<div class="ed-node-summary ed-node-summary--muted">' + esc(i ? i.label : n.type) + '</div>') + '</div>' +
                '<div class="ed-node-badges">' + badges.join('') + '</div>' + inPorts + outPorts + tools +
                (status === 'error' && step ? '<div class="ed-node-error">' + esc(core.stepErrorText(t, step)) + '</div>' : '');
        }

        function portCaseLabel(n, port) {
            const idx = Number(port.slice(5)) - 1;
            const c = Array.isArray(n.params.cases) ? n.params.cases[idx] : null;
            return (c && c.label) || t('easydrag.ui.port_case', { n: idx + 1 });
        }

        function toolButton(id, iconName, label) {
            return '<button type="button" class="ed-node-tool" data-ed-node-tool="' + id + '" title="' + esc(label) + '" aria-label="' + esc(label) + '">' + core.icon(iconName) + '</button>';
        }

        function classesFor(n) {
            const i = info(n);
            const cls = ['ed-node'];
            if (i && i.trigger) cls.push('is-trigger');
            if (ed.selection.has(n.id)) cls.push('is-selected');
            if (n.settings.disabled) cls.push('is-disabled');
            if (!i || (i.availability && i.availability.state !== 'available')) cls.push('is-unavailable');
            const status = statusOf(n.id);
            if (status) cls.push('status-' + status);
            if (ed.runView) cls.push('is-readonly');
            return cls.join(' ');
        }

        function renderNode(n) {
            let card = nodeEls.get(n.id);
            if (!card) {
                card = document.createElement('div');
                card.dataset.nodeId = n.id;
                card.setAttribute('role', 'group');
                nodeEls.set(n.id, card);
                nodesHost.appendChild(card);
                if (!ed.initialRender) {
                    card.classList.add('is-new');
                    setTimeout(() => card.classList.remove('is-new'), 400);
                }
            }
            const i = info(n);
            card.className = classesFor(n) + (card.classList.contains('is-new') ? ' is-new' : '');
            card.dataset.cat = core.catOf(i);
            card.setAttribute('aria-label', n.label || n.type);
            card.style.transform = 'translate(' + n.position.x + 'px,' + n.position.y + 'px)';
            card.style.height = G.nodeHeight(outCount(n)) + 'px';
            const status = statusOf(n.id);
            const step = ed.run && ed.run.steps && ed.run.steps.get(n.id);
            const sig = JSON.stringify([n.label, n.type, n.params, n.settings, ed.selection.has(n.id), status, step && step.duration_ms, step && step.error_code,
                i && i.availability, nodeIssues(n.id).length, !!ed.runView, ed.readonly, ed.model.outputs(n)]);
            if (signatures.get(n.id) !== sig) {
                signatures.set(n.id, sig);
                card.innerHTML = cardMarkup(n);
            }
        }

        function renderNodes(ids) {
            (ids || []).forEach(id => {
                const n = ed.model.node(id);
                if (n) renderNode(n);
                else removeNode(id);
            });
            afterRender();
        }

        function removeNode(id) {
            const card = nodeEls.get(id);
            if (card) card.remove();
            nodeEls.delete(id);
            signatures.delete(id);
        }

        function render() {
            const present = new Set(ed.model.doc.nodes.map(n => n.id));
            Array.from(nodeEls.keys()).forEach(id => { if (!present.has(id)) removeNode(id); });
            ed.model.doc.nodes.forEach(renderNode);
            afterRender();
        }

        function afterRender() {
            emptyEl.hidden = ed.model.doc.nodes.length > 0 || !!ed.runView;
            minimapFrame.request();
            listUpdate();
        }

        function refreshClasses() {
            ed.model.doc.nodes.forEach(n => {
                const card = nodeEls.get(n.id);
                if (card) renderNode(n);
            });
        }

        // ── minimap ─────────────────────────────────────────────────────────────

        function drawMinimap() {
            const rects = allRects();
            const s = size();
            const viewRect = { x: -ed.view.x / ed.view.zoom, y: -ed.view.y / ed.view.zoom, w: s.w / ed.view.zoom, h: s.h / ed.view.zoom };
            const box = G.bounds(rects.concat([viewRect]));
            const scale = Math.min(200 / box.w, 130 / box.h);
            const ox = (200 - box.w * scale) / 2 - box.x * scale;
            const oy = (130 - box.h * scale) / 2 - box.y * scale;
            minimap.dataset.scale = scale;
            minimap.dataset.ox = ox;
            minimap.dataset.oy = oy;
            minimap.querySelector('.ed-minimap-nodes').innerHTML = ed.model.doc.nodes.map(n => {
                const rc = G.nodeRect(n.position, outCount(n));
                return '<rect x="' + (rc.x * scale + ox).toFixed(1) + '" y="' + (rc.y * scale + oy).toFixed(1) + '" width="' + Math.max(2, rc.w * scale).toFixed(1) +
                    '" height="' + Math.max(2, rc.h * scale).toFixed(1) + '" rx="1.5" data-cat="' + esc(core.catOf(info(n))) + '"' + (ed.selection.has(n.id) ? ' class="is-selected"' : '') + '></rect>';
            }).join('');
            const v = minimap.querySelector('.ed-minimap-view');
            v.setAttribute('x', (viewRect.x * scale + ox).toFixed(1));
            v.setAttribute('y', (viewRect.y * scale + oy).toFixed(1));
            v.setAttribute('width', (viewRect.w * scale).toFixed(1));
            v.setAttribute('height', (viewRect.h * scale).toFixed(1));
        }

        function minimapToWorld(event) {
            const rect = minimap.getBoundingClientRect();
            const sx = (event.clientX - rect.left) / rect.width * 200;
            const sy = (event.clientY - rect.top) / rect.height * 130;
            const scale = Number(minimap.dataset.scale) || 1;
            return { x: (sx - Number(minimap.dataset.ox)) / scale, y: (sy - Number(minimap.dataset.oy)) / scale };
        }

        function panMinimap(event) {
            const p = minimapToWorld(event);
            const s = size();
            setView({ x: s.w / 2 - p.x * ed.view.zoom, y: s.h / 2 - p.y * ed.view.zoom, zoom: ed.view.zoom });
        }

        bag.listen(minimap, 'pointerdown', (event) => {
            event.preventDefault();
            event.stopPropagation();
            minimap.setPointerCapture(event.pointerId);
            panMinimap(event);
            const move = ev => panMinimap(ev);
            const up = () => { minimap.removeEventListener('pointermove', move); minimap.removeEventListener('pointerup', up); };
            minimap.addEventListener('pointermove', move);
            minimap.addEventListener('pointerup', up);
        });

        // ── accessible list and live region ─────────────────────────────────────

        function drawList() {
            listEl.innerHTML = ed.model.doc.nodes.map(n => {
                const targets = ed.model.outgoing(n.id).map(e => { const other = ed.model.node(e.target.node); return other ? other.label : ''; }).filter(Boolean);
                const text = (n.label || n.type) + (targets.length ? ' → ' + targets.join(', ') : '');
                return '<li><button type="button" data-ed-focus-node="' + esc(n.id) + '">' + esc(text) + '</button></li>';
            }).join('');
        }

        function announce(text) { liveEl.textContent = ''; setTimeout(() => { liveEl.textContent = text; }, 30); }

        bag.listen(listEl, 'click', (event) => {
            const btn = event.target.closest('[data-ed-focus-node]');
            if (btn) ed.bus.emit('focus-node', btn.dataset.edFocusNode);
        });

        bag.listen(el.querySelector('.ed-zoom'), 'click', (event) => {
            const btn = event.target.closest('[data-ed-zoom]');
            if (!btn) return;
            const action = btn.dataset.edZoom;
            if (action === 'in') zoomBy(1.2);
            else if (action === 'out') zoomBy(1 / 1.2);
            else if (action === 'reset') { const s = size(); setView(G.zoomAt(ed.view, 1, { x: s.w / 2, y: s.h / 2 }), { animate: true }); }
            else fit({ animate: true });
        });

        bag.listen(el.querySelector('[data-ed-empty-add]'), 'click', () => {
            const s = size();
            ed.bus.emit('quick-add', { x: s.left + s.w / 2, y: s.top + s.h / 2, filter: 'trigger' });
        });

        bag.add(ed.bus.on('model', change => {
            if (change.kind === 'viewport') return;
            if (change.kind === 'reset' || change.kind === 'undo' || change.kind === 'redo' || change.meta) render();
            else renderNodes(change.nodes.concat(affectedByEdges(change.edges)));
        }));
        bag.add(ed.bus.on('selection', refreshClasses));
        bag.add(ed.bus.on('run', refreshClasses));
        bag.add(ed.bus.on('issues', refreshClasses));

        function affectedByEdges(edgeIds) {
            const ids = new Set();
            (edgeIds || []).forEach(id => {
                const e = ed.model.edge(id);
                if (e) { ids.add(e.source.node); ids.add(e.target.node); }
            });
            return Array.from(ids);
        }

        const resize = new ResizeObserver(() => { viewFrame.request(); });
        resize.observe(el);
        bag.add(() => resize.disconnect());
        bag.add(() => { viewFrame.cancel(); minimapFrame.cancel(); listUpdate.cancel(); });

        return {
            el, world, wiresSvg, nodesHost,
            render, renderNodes, setView, fit, zoomBy, centerOn, size, clientToWorld, nodeRect, announce,
            nodeEl: id => nodeEls.get(id) || null,
            dispose() { bag.dispose(); }
        };
    }

    ED.canvas = { create };
})();
