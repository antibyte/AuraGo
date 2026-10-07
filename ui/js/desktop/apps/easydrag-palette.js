// EasyDrag palette: collapsible node catalogue with search and drag to canvas, plus the
// quick-add popover (Tab, wire dropped into empty space, "+" on ports and wires).
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const RECENT_KEY = 'aurago.easydrag.recent';
    const OPEN_KEY = 'aurago.easydrag.palette.open';

    function search(list, query) {
        const q = String(query || '').trim().toLowerCase();
        if (!q) return list.slice();
        return list.map(info => {
            const label = String(info.label || '').toLowerCase();
            const desc = String(info.description || '').toLowerCase();
            let score = 0;
            if (label.startsWith(q)) score = 3;
            else if (label.includes(q)) score = 2;
            else if (desc.includes(q) || info.type.includes(q)) score = 1;
            return { info, score };
        }).filter(x => x.score > 0).sort((a, b) => b.score - a.score || a.info.label.localeCompare(b.info.label)).map(x => x.info);
    }

    function availabilityBadge(core, t, esc, info) {
        const state = (info.availability || {}).state;
        if (state === 'needs_setup') return '<span class="ed-chip ed-chip--warn" role="link" data-ed-setup="' + esc((info.availability || {}).config_section || 'tools') + '" title="' + esc(t('easydrag.ui.palette_needs_setup')) + '">' + esc(t('easydrag.ui.palette_setup')) + '</span>';
        if (state === 'blocked') return '<span class="ed-chip ed-chip--warn" title="' + esc(t('easydrag.ui.badge_blocked')) + '">' + core.icon('lock') + '</span>';
        if (info.risky) return '<span class="ed-chip ed-chip--risk" title="' + esc(t('easydrag.ui.badge_risky')) + '">' + core.icon('alert') + '</span>';
        return '';
    }

    function itemMarkup(core, t, esc, info, tag) {
        const unavailable = (info.availability || {}).state !== 'available';
        return '<' + tag + ' class="ed-palette-item' + (unavailable ? ' is-unavailable' : '') + '" data-ed-type="' + esc(info.type) + '" data-cat="' + esc(core.catOf(info)) + '"' +
            (tag === 'li' ? ' role="option"' : ' role="button" tabindex="0"') + ' title="' + esc(info.description || info.label) + '">' +
            '<span class="ed-palette-tile">' + core.icon(info.icon) + '</span>' +
            '<span class="ed-palette-text"><span class="ed-palette-name">' + esc(info.label) + '</span>' +
            (info.description ? '<span class="ed-palette-desc">' + esc(info.description) + '</span>' : '') + '</span>' +
            availabilityBadge(core, t, esc, info) + '</' + tag + '>';
    }

    function rememberType(type) {
        const core = ED.core;
        const list = core.storage.get(RECENT_KEY, []).filter(x => x !== type);
        list.unshift(type);
        core.storage.set(RECENT_KEY, list.slice(0, 6));
    }

    // place adds a node of type at a world position, optionally connected or inserted.
    function place(ed, type, at, opts) {
        const G = ED.geometry;
        const info = ed.catalog.types.get(type);
        if (!info || ed.readonly || ed.runView) return null;
        rememberType(type);
        const pos = { x: G.snap(at.x), y: G.snap(at.y) };
        let id;
        if (opts && opts.edge && ed.model.edge(opts.edge) && (info.inputs || []).length) {
            id = ed.model.insertOnEdge(opts.edge, type, pos);
        } else if (opts && opts.from && !info.trigger) {
            id = ed.model.addNode(type, pos, null, opts.from);
        } else {
            id = ed.model.addNode(type, pos);
        }
        if (id) {
            ed.selection.clear();
            ed.selection.add(id);
            ed.bus.emit('selection', ed.selection);
        }
        return id;
    }

    function createPanel(ed, canvas) {
        const core = ED.core;
        const G = ED.geometry;
        const { t, esc } = ed;
        const bag = core.bag();
        const collapsed = new Set(core.storage.get('aurago.easydrag.palette.collapsed', ['tool']));
        const el = core.el('<aside class="ed-palette" aria-label="' + esc(t('easydrag.ui.palette_title')) + '">' +
            '<div class="ed-palette-head"><div class="ed-search">' + core.icon('search') +
            '<input type="search" class="ed-palette-search" placeholder="' + esc(t('easydrag.ui.palette_search')) + '" aria-label="' + esc(t('easydrag.ui.palette_search')) + '" enterkeyhint="search" inputmode="search"></div>' +
            '<button type="button" class="ed-icon-btn" data-ed-palette-close title="' + esc(t('easydrag.ui.palette_hide')) + '" aria-label="' + esc(t('easydrag.ui.palette_hide')) + '">' + core.icon('chevron-left') + '</button></div>' +
            '<div class="ed-palette-list"></div>' +
            '<p class="ed-palette-hint">' + esc(t('easydrag.ui.palette_hint')) + '</p></aside>');
        const list = el.querySelector('.ed-palette-list');
        const input = el.querySelector('.ed-palette-search');

        function groups() {
            const out = [];
            const curated = ed.catalog.categories.filter(c => !String(c.id).startsWith('tool:'));
            curated.forEach(c => out.push({ id: c.id, label: c.label, items: ed.catalog.list.filter(i => i.category === c.id) }));
            const tools = ed.catalog.list.filter(i => String(i.category).startsWith('tool:'));
            if (tools.length) out.push({ id: 'tool', label: t('easydrag.ui.palette_more_tools'), items: tools, sub: ed.catalog.categories.filter(c => String(c.id).startsWith('tool:')) });
            return out.filter(g => g.items.length);
        }

        function render() {
            const query = input.value;
            if (query.trim()) {
                const found = search(ed.catalog.list, query);
                list.innerHTML = found.length
                    ? '<div class="ed-palette-items">' + found.map(i => itemMarkup(core, t, esc, i, 'div')).join('') + '</div>'
                    : '<p class="ed-palette-empty">' + esc(t('easydrag.ui.palette_no_results')) + '</p>';
                return;
            }
            list.innerHTML = groups().map(g => {
                const open = !collapsed.has(g.id);
                const body = g.sub
                    ? g.sub.map(sc => {
                        const items = g.items.filter(i => i.category === sc.id);
                        return items.length ? '<h4 class="ed-palette-sub">' + esc(sc.label) + '</h4>' + items.map(i => itemMarkup(core, t, esc, i, 'div')).join('') : '';
                    }).join('')
                    : g.items.map(i => itemMarkup(core, t, esc, i, 'div')).join('');
                return '<section class="ed-palette-cat" data-cat="' + esc(g.id) + '">' +
                    '<button type="button" class="ed-palette-cat-head" aria-expanded="' + open + '" data-ed-cat="' + esc(g.id) + '">' +
                    '<span class="ed-palette-cat-dot"></span><span>' + esc(g.label) + '</span><span class="ed-count">' + g.items.length + '</span>' + core.icon('chevron-down', 'ed-chevron') + '</button>' +
                    '<div class="ed-palette-items"' + (open ? '' : ' hidden') + '>' + body + '</div></section>';
            }).join('');
        }

        // setOpen shows or hides the panel; transient leaves the stored choice as it is.
        function setOpen(open, transient) {
            el.classList.toggle('is-collapsed', !open);
            ed.root.classList.toggle('has-palette', open);
            if (!transient) core.storage.set(OPEN_KEY, open);
            ed.bus.emit('palette', open);
        }

        // ── drag from palette ───────────────────────────────────────────────────

        let drag = null; // the drag in progress (core.capturePointer handle)

        // startDrag follows one pointer from a palette item. A release adds the node (a click
        // appends it, a drop places it); a cancel (pointercancel, lost capture, a release
        // missed outside the window, dispose) only cleans up. The pointer is captured on the
        // list, which outlives a re-render of its items (search, catalog refresh).
        function startDrag(event, type) {
            if (drag || ed.readonly || ed.runView) return;
            const info = ed.catalog.types.get(type);
            const start = { x: event.clientX, y: event.clientY };
            let ghost = null;
            let overEdge = null;
            const move = ev => {
                if (!ghost && Math.hypot(ev.clientX - start.x, ev.clientY - start.y) < 5) return;
                if (!ghost) {
                    ghost = core.el('<div class="ed-drag-ghost" data-cat="' + esc(core.catOf(info)) + '"><span class="ed-palette-tile">' + core.icon(info.icon) + '</span><span>' + esc(info.label) + '</span></div>');
                    ed.root.appendChild(ghost);
                    ed.dragging = true;
                }
                const host = ed.root.getBoundingClientRect();
                ghost.style.transform = 'translate(' + (ev.clientX - host.left + 8) + 'px,' + (ev.clientY - host.top + 8) + 'px)';
                const overCanvas = !!(document.elementFromPoint(ev.clientX, ev.clientY) || { closest: () => null }).closest('.ed-canvas');
                ghost.classList.toggle('is-over-canvas', overCanvas);
                el.querySelectorAll('.is-drop-target').forEach(n => n.classList.remove('is-drop-target'));
                overEdge = null;
                if (overCanvas && (info.inputs || []).length && (info.outputs || []).length) {
                    const target = document.elementFromPoint(ev.clientX, ev.clientY).closest('.ed-edge');
                    canvas.el.querySelectorAll('.ed-edge.is-drop-target').forEach(g => g.classList.remove('is-drop-target'));
                    if (target) { overEdge = target.dataset.edgeId; target.classList.add('is-drop-target'); }
                }
            };
            const up = (ev, cancelled) => {
                drag = null;
                ed.dragging = false;
                canvas.el.querySelectorAll('.ed-edge.is-drop-target').forEach(g => g.classList.remove('is-drop-target'));
                if (cancelled) { if (ghost) ghost.remove(); return; }
                if (!ghost) { addByClick(type); return; }
                ghost.remove();
                const hit = document.elementFromPoint(ev.clientX, ev.clientY);
                if (!hit || !hit.closest('.ed-canvas')) return;
                const p = canvas.clientToWorld(ev.clientX, ev.clientY);
                place(ed, type, { x: p.x - G.NODE_W / 2, y: p.y - G.NODE_H / 2 }, { edge: overEdge });
                closeOverlay();
            };
            drag = core.capturePointer(list, event, move, up);
        }

        // addByClick appends after the single selected node, or places the node in free space.
        function addByClick(type) {
            const info = ed.catalog.types.get(type);
            const sel = ed.selection.size === 1 ? ed.model.node(Array.from(ed.selection)[0]) : null;
            if (sel && !info.trigger && ed.model.outputs(sel).length) {
                const rects = ed.model.doc.nodes.map(n => canvas.nodeRect(n.id));
                const at = G.freeSpot({ x: sel.position.x + G.NODE_W + 96, y: sel.position.y }, rects);
                const id = place(ed, type, at, { from: { node: sel.id, port: ed.model.outputs(sel)[0] } });
                closeOverlay();
                if (id) canvas.centerOn(id, { animate: true });
                return;
            }
            const s = canvas.size();
            const center = G.toWorld(ed.view, { x: s.w / 2, y: s.h / 2 });
            const rects = ed.model.doc.nodes.map(n => canvas.nodeRect(n.id));
            place(ed, type, G.freeSpot({ x: center.x - G.NODE_W / 2, y: center.y - G.NODE_H / 2 }, rects));
            closeOverlay();
        }

        // ── floating panel (narrow windows) ──────────────────────────────────────

        // isOverlay: the panel floats over the canvas (narrow windows) instead of beside it.
        function isOverlay() { return !!el.isConnected && typeof getComputedStyle === 'function' && getComputedStyle(el).position === 'absolute'; }
        function isOpen() { return !el.classList.contains('is-collapsed'); }

        // A floating panel covers the flow: it closes once a step was added and when the canvas is
        // pressed. Closing it this way leaves the stored choice as it is.
        function closeOverlay() { if (isOpen() && isOverlay()) setOpen(false, true); }

        // syncLayout follows the window width: becoming a floating panel closes it, becoming a docked
        // one again restores the stored choice. The editor runs it before its first view.
        let overlay = null;
        function syncLayout() {
            if (!el.isConnected) return;
            const now = isOverlay();
            if (now === overlay) return;
            const was = overlay;
            overlay = now;
            if (now) closeOverlay();
            else if (was) setOpen(!!core.storage.get(OPEN_KEY, true), true);
        }

        // afterMove runs fn once the panel stopped moving (its margin transition), at the latest after
        // 400 ms: the canvas beside a docked panel has its final width by then.
        function afterMove(fn) {
            let done = false;
            let timer = 0;
            const end = event => { if (!event || (event.target === el && event.propertyName === 'margin-left')) finish(); };
            function finish() {
                if (done) return;
                done = true;
                clearTimeout(timer);
                el.removeEventListener('transitionend', end);
                fn();
            }
            el.addEventListener('transitionend', end);
            timer = setTimeout(finish, 400);
        }
        if (typeof ResizeObserver === 'function' && ed.root) {
            const watch = new ResizeObserver(() => syncLayout());
            watch.observe(ed.root);
            bag.add(() => watch.disconnect());
        }
        bag.listen(canvas.el, 'pointerdown', closeOverlay);

        bag.listen(input, 'input', core.debounce(render, 80));
        bag.listen(list, 'click', (event) => {
            // The setup chip opens on click: a popup opened on pointerdown is blocked on tablets.
            const setup = event.target.closest('[data-ed-setup]');
            if (setup) { event.preventDefault(); window.open('/config#' + setup.dataset.edSetup, '_blank', 'noopener'); return; }
            const head = event.target.closest('[data-ed-cat]');
            if (!head) return;
            const id = head.dataset.edCat;
            if (collapsed.has(id)) collapsed.delete(id); else collapsed.add(id);
            core.storage.set('aurago.easydrag.palette.collapsed', Array.from(collapsed));
            render();
        });
        bag.listen(list, 'pointerdown', (event) => {
            if (event.target.closest('[data-ed-setup]')) return;
            const item = event.target.closest('[data-ed-type]');
            if (!item || event.button !== 0) return;
            event.preventDefault();
            startDrag(event, item.dataset.edType);
        });
        bag.listen(list, 'keydown', (event) => {
            const item = event.target.closest('[data-ed-type]');
            if (item && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); addByClick(item.dataset.edType); }
        });
        bag.listen(el.querySelector('[data-ed-palette-close]'), 'click', () => setOpen(false));

        render();
        setOpen(core.storage.get(OPEN_KEY, true));

        return {
            el, render, setOpen, isOpen, isOverlay, syncLayout, afterMove,
            focusSearch() { setOpen(true); input.focus(); input.select(); },
            dispose() { if (drag) drag.abort(); bag.dispose(); }
        };
    }

    // openQuickAdd shows the quick-add popover at client point (x, y).
    // req: {x, y, from?: {node, port}, edge?: edgeId, filter?: "trigger"}
    function openQuickAdd(ed, canvas, req) {
        const core = ED.core;
        const G = ED.geometry;
        const { t, esc } = ed;
        if (ed.readonly || ed.runView) return null;
        closeQuickAdd(ed);
        let candidates = ed.catalog.list.slice();
        if (req.filter === 'trigger') candidates = candidates.filter(i => i.trigger);
        if (req.from || req.edge) candidates = candidates.filter(i => !i.trigger && (i.inputs || []).length);
        if (req.edge) candidates = candidates.filter(i => (i.outputs || []).length);
        const recent = core.storage.get(RECENT_KEY, []).map(type => candidates.find(i => i.type === type)).filter(Boolean);
        const el = core.el('<div class="ed-quick" role="dialog" aria-label="' + esc(t('easydrag.ui.quick_title')) + '">' +
            '<div class="ed-search">' + core.icon('search') + '<input type="text" class="ed-quick-search" role="combobox" aria-expanded="true" aria-autocomplete="list" placeholder="' +
            esc(req.filter === 'trigger' ? t('easydrag.ui.quick_search_trigger') : t('easydrag.ui.quick_search')) + '" enterkeyhint="go" autocomplete="off"></div>' +
            '<ul class="ed-quick-list" role="listbox"></ul></div>');
        ed.root.appendChild(el);
        const input = el.querySelector('input');
        const listEl = el.querySelector('ul');
        const listId = 'ed-quick-list-' + ed.windowId;
        listEl.id = listId;
        input.setAttribute('aria-controls', listId);
        let items = [];
        let active = 0;

        function render() {
            const q = input.value;
            if (q.trim()) items = search(candidates, q);
            else {
                const rest = candidates.filter(i => !recent.includes(i));
                items = recent.concat(rest);
            }
            items = items.slice(0, 60);
            active = Math.min(active, Math.max(0, items.length - 1));
            if (!items.length) { listEl.innerHTML = '<li class="ed-quick-empty" role="presentation">' + esc(t('easydrag.ui.palette_no_results')) + '</li>'; return; }
            let lastCat = '';
            listEl.innerHTML = items.map((info, idx) => {
                let head = '';
                const cat = !q.trim() && idx < recent.length ? 'recent' : (q.trim() ? '' : info.category);
                if (cat && cat !== lastCat) {
                    const label = cat === 'recent' ? t('easydrag.ui.quick_recent') : ((ed.catalog.categories.find(c => c.id === cat) || {}).label || cat);
                    head = '<li class="ed-quick-group" role="presentation">' + esc(label) + '</li>';
                    lastCat = cat;
                }
                return head + itemMarkup(core, t, esc, info, 'li').replace('<li class="ed-palette-item', '<li id="' + listId + '-' + idx + '" aria-selected="' + (idx === active) + '" data-ed-index="' + idx + '" class="ed-palette-item' + (idx === active ? ' is-active' : ''));
            }).join('');
            input.setAttribute('aria-activedescendant', listId + '-' + active);
            const activeEl = listEl.querySelector('.is-active');
            if (activeEl) activeEl.scrollIntoView({ block: 'nearest' });
        }

        function position() {
            const host = ed.root.getBoundingClientRect();
            const w = 320;
            const h = 380;
            const x = core.clamp(req.x - host.left, 8, host.width - w - 8);
            const y = core.clamp(req.y - host.top, 8, host.height - h - 8);
            el.style.transform = 'translate(' + x + 'px,' + y + 'px)';
        }

        function choose(idx) {
            const info = items[idx];
            if (!info) return;
            let at;
            if (req.from) {
                const src = ed.model.node(req.from.node);
                const outs = ed.model.outputs(src);
                const port = G.portPoint(src.position, 'out', Math.max(0, outs.indexOf(req.from.port)), outs.length);
                const dropped = canvas.clientToWorld(req.x, req.y);
                at = dropped.x > port.x + 40 ? { x: dropped.x, y: dropped.y - G.NODE_H / 2 } : { x: src.position.x + G.NODE_W + 96, y: port.y - G.NODE_H / 2 };
            } else if (req.edge) {
                const e = ed.model.edge(req.edge);
                const src = ed.model.node(e.source.node);
                const dst = ed.model.node(e.target.node);
                at = { x: (src.position.x + dst.position.x) / 2, y: (src.position.y + dst.position.y) / 2 + 24 };
            } else {
                const p = canvas.clientToWorld(req.x, req.y);
                at = { x: p.x - G.NODE_W / 2, y: p.y - G.NODE_H / 2 };
            }
            const rects = ed.model.doc.nodes.map(n => canvas.nodeRect(n.id));
            const spot = req.edge ? at : G.freeSpot(at, rects);
            const id = place(ed, info.type, spot, { from: req.from, edge: req.edge });
            close();
            if (id) canvas.el.focus({ preventScroll: true });
        }

        let closed = false;

        function close() {
            if (closed) return;
            closed = true;
            el.remove();
            document.removeEventListener('pointerdown', outside, true);
            if (ed.quickAdd && ed.quickAdd.el === el) ed.quickAdd = null;
        }

        function outside(event) { if (!el.contains(event.target)) close(); }

        input.addEventListener('input', () => { active = 0; render(); });
        input.addEventListener('keydown', (event) => {
            // Keys of an IME composition belong to the text; they stay off the canvas too.
            if (event.isComposing || event.keyCode === 229) { event.stopPropagation(); return; }
            if (event.key === 'ArrowDown') { event.preventDefault(); active = Math.min(items.length - 1, active + 1); render(); }
            else if (event.key === 'ArrowUp') { event.preventDefault(); active = Math.max(0, active - 1); render(); }
            else if (event.key === 'Enter') { event.preventDefault(); choose(active); }
            else if (event.key === 'Escape' || event.key === 'Tab') { event.preventDefault(); close(); canvas.el.focus({ preventScroll: true }); }
            event.stopPropagation();
        });
        listEl.addEventListener('pointerdown', (event) => {
            const item = event.target.closest('[data-ed-index]');
            if (!item) return;
            event.preventDefault();
            choose(Number(item.dataset.edIndex));
        });
        // Listen for outside presses from the next task on (not the press that opened the popover),
        // unless it was closed by then.
        setTimeout(() => { if (!closed) document.addEventListener('pointerdown', outside, true); }, 0);
        position();
        render();
        input.focus({ preventScroll: true });
        requestAnimationFrame(() => el.classList.add('is-open'));
        ed.quickAdd = { el, close };
        return ed.quickAdd;
    }

    function closeQuickAdd(ed) {
        if (ed.quickAdd) ed.quickAdd.close();
    }

    ED.palette = { createPanel, openQuickAdd, closeQuickAdd, search, place };
})();
