// EasyDrag canvas: viewport (one CSS transform), dot grid, node cards, zoom control,
// minimap, empty state and an accessible node list.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    // READABLE_ZOOM is the smallest zoom of an automatic fit; READABLE_PAD the room it leaves at the
    // trigger end. Below ZOOMED_OUT the cards show their labels only, in a larger size.
    const READABLE_ZOOM = 0.8;
    const READABLE_PAD = 48;
    const ZOOMED_OUT = 0.7;

    // portLabel names the output port of node n: a switch case by its label or number, any other port
    // by its translation. The connect picker uses it too.
    function portLabel(t, n, port) {
        if (!port.startsWith('case_')) return ED.core.tr(t, 'easydrag.ui.port_' + port, port);
        const idx = Number(port.slice(5)) - 1;
        const c = n && n.params && Array.isArray(n.params.cases) ? n.params.cases[idx] : null;
        return (c && c.label) || t('easydrag.ui.port_case', { n: idx + 1 });
    }

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
        const minimapNodes = minimap.querySelector('.ed-minimap-nodes');
        const listEl = el.querySelector('.ed-node-list');
        const liveEl = el.querySelector('[data-ed-live]');

        // Every timeout the canvas starts is tracked, so dispose can clear the ones still pending.
        const timers = new Set();
        let animTimer = 0;
        let announceTimer = 0;
        let miniDrag = null;
        // named holds the key and label each card was drawn with; a change to them can alter
        // the summaries of other cards that reference the node.
        const named = new Map();
        // keyIndex maps node keys to nodes for summaries; it is rebuilt when the model or its version
        // changes (a run view brings its own model, which starts at version 0 again).
        let keyIndex = new Map();
        let keyIndexModel = null;
        let keyIndexVersion = -1;
        // The minimap node layer is redrawn only when the model, minimapKey (document version,
        // selection, or a scale so small that cards need their minimum size) changes.
        let minimapKey = '';
        let minimapModel = null;
        let selectionTick = 0;

        // forgetCaches drops what was cached for the current document (on a reset).
        function forgetCaches() {
            keyIndexModel = null;
            keyIndexVersion = -1;
            minimapModel = null;
            minimapKey = '';
        }

        function later(fn, ms) {
            const id = setTimeout(() => { timers.delete(id); fn(); }, ms);
            timers.add(id);
            return id;
        }

        function cancel(id) { clearTimeout(id); timers.delete(id); }

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
            el.classList.toggle('is-zoomed-out', v.zoom < ZOOMED_OUT);
            zoomValue.textContent = Math.round(v.zoom * 100) + '%';
            minimapFrame.request();
        }

        // setView coerces the view: an x or y that is not a finite number becomes 0 and clampZoom
        // maps a bad zoom to 1, so a corrupt stored viewport or a NaN cannot reach the transforms.
        function setView(view, opts) {
            const v = view && typeof view === 'object' ? view : {};
            ed.view = { x: Number.isFinite(v.x) ? v.x : 0, y: Number.isFinite(v.y) ? v.y : 0, zoom: G.clampZoom(v.zoom) };
            el.classList.toggle('is-animating', !!(opts && opts.animate) && !matchMedia('(prefers-reduced-motion: reduce)').matches);
            if (opts && opts.animate) {
                cancel(animTimer);
                animTimer = later(() => el.classList.remove('is-animating'), 260);
            }
            viewFrame.request();
            ed.bus.emit('view', ed.view);
        }

        function allRects() { return ed.model.doc.nodes.map(n => G.nodeRect(n.position, outCount(n))); }

        // visibleArea is the part of the canvas that the run drawer (right) and a palette floating over
        // the canvas (left, narrow windows) leave free; with too little left, the whole canvas. The
        // drawer is measured by its width: it slides in with a transform, which a client rect follows.
        function visibleArea() {
            const s = size();
            const find = sel => (ed.root && typeof ed.root.querySelector === 'function' ? ed.root.querySelector(sel) : null);
            let left = 0;
            let right = 0;
            const drawer = find('.ed-drawer');
            if (drawer) right = clampCover(drawer.offsetWidth, s.w);
            const palette = find('.ed-palette:not(.is-collapsed)');
            if (palette && typeof getComputedStyle === 'function' && getComputedStyle(palette).position === 'absolute') {
                left = clampCover(palette.getBoundingClientRect().right - s.left, s.w);
            }
            if (s.w - left - right < 240) return { left: 0, w: s.w, h: s.h };
            return { left, w: s.w - left - right, h: s.h };
        }

        function clampCover(value, max) { return Number.isFinite(value) ? Math.min(max, Math.max(0, value)) : 0; }

        // fit shows the whole flow in the free part of the canvas. opts.readable (the automatic fits:
        // opening a flow, the run view) never goes below READABLE_ZOOM: a flow that does not fit then
        // starts at its trigger, READABLE_PAD from the left edge, and is centred vertically when its
        // height fits, else on the trigger's row. An explicit Fit (Shift+1, the button) shows it all.
        function fit(opts) {
            const area = visibleArea();
            const box = G.bounds(allRects());
            const v = G.fit(box, { w: area.w, h: area.h }, 72, 1);
            if (opts && opts.readable && box && v.zoom < READABLE_ZOOM) { setView(readable(box, area), opts); return; }
            setView({ x: v.x + area.left, y: v.y, zoom: v.zoom }, opts);
        }

        function readable(box, area) {
            const zoom = READABLE_ZOOM;
            const anchor = anchorRect() || box;
            const x = box.w * zoom <= area.w - 2 * READABLE_PAD
                ? area.left + (area.w - box.w * zoom) / 2 - box.x * zoom
                : area.left + READABLE_PAD - anchor.x * zoom;
            const y = box.h * zoom <= area.h - 2 * READABLE_PAD
                ? (area.h - box.h * zoom) / 2 - box.y * zoom
                : area.h / 2 - (anchor.y + anchor.h / 2) * zoom;
            return { x, y, zoom };
        }

        // anchorRect is the card of the leftmost enabled trigger, else of the leftmost step.
        function anchorRect() {
            const nodes = ed.model.doc.nodes;
            const triggers = nodes.filter(n => { const i = info(n); return i && i.trigger && !(n.settings && n.settings.disabled); });
            const pick = (triggers.length ? triggers : nodes).slice().sort((a, b) => a.position.x - b.position.x)[0];
            return pick ? G.nodeRect(pick.position, outCount(pick)) : null;
        }

        // center is the world point in the middle of the canvas and the zoom: stored that way, a view
        // suits any window size (setCenter places it again).
        function center() {
            const s = size();
            return { cx: (s.w / 2 - ed.view.x) / ed.view.zoom, cy: (s.h / 2 - ed.view.y) / ed.view.zoom, zoom: ed.view.zoom };
        }

        function setCenter(c, opts) {
            const s = size();
            const zoom = G.clampZoom(c && c.zoom);
            setView({ x: s.w / 2 - Number(c.cx) * zoom, y: s.h / 2 - Number(c.cy) * zoom, zoom }, opts);
        }

        // anyNodeVisible reports whether a card overlaps the free part of the canvas.
        function anyNodeVisible() {
            const a = visibleArea();
            const v = ed.view;
            return allRects().some(r => {
                const left = r.x * v.zoom + v.x;
                const top = r.y * v.zoom + v.y;
                return left + r.w * v.zoom > a.left && left < a.left + a.w && top + r.h * v.zoom > 0 && top < a.h;
            });
        }

        function zoomBy(factor, center) {
            const s = size();
            const point = center || { x: s.w / 2, y: s.h / 2 };
            setView(G.zoomAt(ed.view, ed.view.zoom * factor, point));
        }

        // centerOn puts a step in the middle of the free part of the canvas, at a readable zoom.
        function centerOn(id, opts) {
            const rect = nodeRect(id);
            if (!rect) return;
            const a = visibleArea();
            const zoom = Math.max(ed.view.zoom, READABLE_ZOOM);
            setView({ x: a.left + a.w / 2 - (rect.x + rect.w / 2) * zoom, y: a.h / 2 - (rect.y + rect.h / 2) * zoom, zoom }, opts);
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
                const src = byKey(root);
                const label = src ? src.label : root;
                const field = rest.replace(/\|.*$/, '').trim().replace(/^\./, '');
                return '‹' + label + (field ? ' › ' + field : '') + '›';
            });
            text = text.replace(/\s+/g, ' ').trim();
            return text.length > 56 ? text.slice(0, 55) + '…' : text;
        }

        // byKey looks a node up by key like model.byKey (the first node wins), without a scan per call.
        function byKey(key) {
            if (keyIndexModel !== ed.model || keyIndexVersion !== ed.model.version) {
                keyIndex = new Map();
                ed.model.doc.nodes.forEach(n => { if (!keyIndex.has(n.key)) keyIndex.set(n.key, n); });
                keyIndexModel = ed.model;
                keyIndexVersion = ed.model.version;
            }
            return keyIndex.get(key) || null;
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

        function cardMarkup(n, sum) {
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
                const label = portLabel(t, n, p);
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
            // Without a summary the second line names the type, or, while the step still carries the
            // type's own label, describes it: the card does not say the same thing twice.
            const typeLine = !i ? n.type : n.label && n.label !== i.label ? i.label : (i.description || '');
            return '<div class="ed-node-tile">' + core.icon(i ? i.icon : 'tool') + '</div>' +
                '<div class="ed-node-body"><div class="ed-node-label">' + esc(n.label || (i && i.label) || n.type) + '</div>' +
                (sum ? '<div class="ed-node-summary">' + esc(sum) + '</div>' : typeLine ? '<div class="ed-node-summary ed-node-summary--muted">' + esc(typeLine) + '</div>' : '') + '</div>' +
                '<div class="ed-node-badges">' + badges.join('') + '</div>' + inPorts + outPorts + tools +
                (status === 'error' && step ? '<div class="ed-node-error">' + esc(core.stepErrorText(t, step)) + '</div>' : '');
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
            if (ed.readonly || ed.runView) cls.push('is-readonly');
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
                    later(() => card.classList.remove('is-new'), 400);
                }
            }
            const i = info(n);
            card.className = classesFor(n) + (card.classList.contains('is-new') ? ' is-new' : '');
            card.dataset.cat = core.catOf(i);
            card.setAttribute('aria-label', n.label || n.type);
            card.style.transform = 'translate(' + n.position.x + 'px,' + n.position.y + 'px)';
            card.style.height = G.nodeHeight(outCount(n)) + 'px';
            named.set(n.id, n.key + '|' + n.label);
            const status = statusOf(n.id);
            const step = ed.run && ed.run.steps && ed.run.steps.get(n.id);
            // The summary shows the labels of referenced nodes, so it is part of the signature. The
            // selection is not: it only sets the is-selected class.
            const sum = summary(n, i);
            const sig = JSON.stringify([n.label, n.type, n.params, n.settings, status, step && step.duration_ms, step && step.error_code,
                i && i.availability, nodeIssues(n.id).length, !!ed.runView, ed.readonly, ed.model.outputs(n), sum]);
            if (signatures.get(n.id) !== sig) {
                signatures.set(n.id, sig);
                card.innerHTML = cardMarkup(n, sum);
            }
        }

        // relabelled reports whether a change added or removed one of these nodes or gave it a new key
        // or label; each can change what the summaries of other cards show.
        function relabelled(ids) {
            return (ids || []).some(id => {
                const n = ed.model.node(id);
                const was = named.get(id);
                return n ? was !== n.key + '|' + n.label : was !== undefined;
            });
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
            named.delete(id);
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

        // refreshSelection updates the card classes only: card markup does not show the selection.
        function refreshSelection() {
            ed.model.doc.nodes.forEach(n => {
                const card = nodeEls.get(n.id);
                if (card) card.className = classesFor(n) + (card.classList.contains('is-new') ? ' is-new' : '');
            });
            selectionTick++;
            minimapFrame.request();
        }

        // ── minimap ─────────────────────────────────────────────────────────────

        // drawMinimap draws the node layer in world coordinates and places it with a transform, so a
        // pan or zoom only moves the layer and the view rectangle.
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
            // Cards stay at least 2 px; that size only matters when the scale is tiny.
            const minSize = 2 / scale;
            const key = ed.model.version + ':' + selectionTick + ':' + (minSize > G.NODE_H ? minSize.toFixed(1) : '');
            if (minimapModel !== ed.model || key !== minimapKey) {
                minimapModel = ed.model;
                minimapKey = key;
                minimapNodes.innerHTML = ed.model.doc.nodes.map(n => {
                    const rc = G.nodeRect(n.position, outCount(n));
                    return '<rect x="' + rc.x + '" y="' + rc.y + '" width="' + Math.max(minSize, rc.w).toFixed(1) + '" height="' + Math.max(minSize, rc.h).toFixed(1) +
                        '" rx="12" data-cat="' + esc(core.catOf(info(n))) + '"' + (ed.selection.has(n.id) ? ' class="is-selected"' : '') + '></rect>';
                }).join('');
            }
            minimapNodes.setAttribute('transform', 'matrix(' + scale + ' 0 0 ' + scale + ' ' + ox.toFixed(1) + ' ' + oy.toFixed(1) + ')');
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
            if (miniDrag) miniDrag.detach();
            panMinimap(event);
            miniDrag = core.capturePointer(minimap, event, panMinimap, () => { miniDrag = null; });
        });
        bag.add(() => { if (miniDrag) miniDrag.detach(); });

        // ── accessible list and live region ─────────────────────────────────────

        function drawList() {
            // One pass over nodes and edges: the labels each node leads to, in edge order.
            const byId = new Map(ed.model.doc.nodes.map(n => [n.id, n]));
            const leadsTo = new Map();
            ed.model.doc.edges.forEach(e => {
                const other = byId.get(e.target.node);
                if (!other || !other.label) return;
                if (!leadsTo.has(e.source.node)) leadsTo.set(e.source.node, []);
                leadsTo.get(e.source.node).push(other.label);
            });
            listEl.innerHTML = ed.model.doc.nodes.map(n => {
                const targets = leadsTo.get(n.id) || [];
                const text = (n.label || n.type) + (targets.length ? ' → ' + targets.join(', ') : '');
                return '<li><button type="button" data-ed-focus-node="' + esc(n.id) + '">' + esc(text) + '</button></li>';
            }).join('');
        }

        // announce clears the live region and sets the text a moment later, so a repeated text is read
        // again; a newer text replaces one that is still waiting.
        function announce(text) {
            cancel(announceTimer);
            liveEl.textContent = '';
            announceTimer = later(() => { liveEl.textContent = text; }, 30);
        }

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
            if (change.kind === 'reset') forgetCaches();
            // A node that comes, goes or gets a new key or label can change the summaries of other cards:
            // check them all (signatures keep the markup of the unaffected ones).
            if (change.kind === 'reset' || change.kind === 'undo' || change.kind === 'redo' || change.meta || relabelled(change.nodes)) render();
            else renderNodes(change.nodes.concat(affectedByEdges(change.edges)));
        }));
        bag.add(ed.bus.on('selection', refreshSelection));
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
        bag.add(() => {
            viewFrame.cancel();
            minimapFrame.cancel();
            listUpdate.cancel();
            timers.forEach(id => clearTimeout(id));
            timers.clear();
        });

        return {
            el, world, wiresSvg, nodesHost,
            render, renderNodes, setView, fit, zoomBy, centerOn, center, setCenter, anyNodeVisible, size, clientToWorld, nodeRect, announce,
            nodeEl: id => nodeEls.get(id) || null,
            dispose() { bag.dispose(); }
        };
    }

    ED.canvas = { create, portLabel };
})();
