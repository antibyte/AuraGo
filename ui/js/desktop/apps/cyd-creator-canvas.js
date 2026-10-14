// CYD Creator designer canvas: arrange toolbar (zoom, snap, grid, align, distribute, z-order,
// lock/hide/delete), the device frame sized from the board profile and orientation, the scaled CYD
// screen with every visible widget, and the unscaled overlay (selection, handles, guides, marquee,
// drop target, issue markers). Pointer and keyboard handling live in cyd-creator-interact.js.
// Layout values are untrusted (see cyd-creator-render.js): widget visuals come only from
// P.render.widget, the screen colour only through P.render.colors, and widget boxes reach CSS as
// finite integers with their size clamped like the renderer clamps it.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const BEZEL = 14;
    // PAD must match the padding of .cydc-canvas-stage in the designer CSS section (plan 4b Task 11).
    const PAD = 24;
    const LABEL_H = 30;
    const MAX_COORD = 4096; // maxCoordinate, internal/cydcreator/limits.go
    const FLASH_MS = 1300;
    const SELECTION_SPEAK_MS = 400;
    const ALIGN = ['left', 'hcenter', 'right', 'top', 'vcenter', 'bottom'];
    const ZORDER = ['front', 'forward', 'backward', 'back'];

    // fitZoom is the largest integer zoom (1..4) at which the scaled screen fits into a viewport of
    // vw × vh CSS px next to the unscaled stage padding, device bezel and device label.
    function fitZoom(size, vw, vh) {
        return P.geometry.fitZoom(size, { w: vw - 2 * (BEZEL + PAD), h: vh - 2 * (BEZEL + PAD) - LABEL_H }, 0);
    }

    function mount(d, host) {
        const { h, dom, icon } = P.render;
        const M = P.model;
        const G = P.geometry;
        const t = d.t;
        const bag = P.util.bag();
        const refs = {};
        const tools = new Map(); // command → toolbar button
        const hintId = P.util.uid('cc_hint');
        const tool = (cmd, name, key, extra) => h('button', Object.assign({ type: 'button', class: 'cydc-tool', 'data-cc-cmd': cmd, title: t(key), 'aria-label': t(key), ref: el => { tools.set(cmd, el); } }, extra || {}), icon(name));
        const ref = key => el => { refs[key] = el; };

        const root = dom(h('div', { class: 'cydc-canvas', tabindex: '0', role: 'group', 'aria-roledescription': t('cyd_creator.designer.canvas_role'), 'aria-label': t('cyd_creator.designer.canvas_label'), 'aria-describedby': hintId, ref: ref('root') },
            h('div', { class: 'cydc-canvas-bar', role: 'toolbar', 'aria-label': t('cyd_creator.designer.toolbar_label'), ref: ref('bar') },
                h('div', { class: 'cydc-tool-group' },
                    tool('zoom-out', 'zoom-out', 'cyd_creator.designer.zoom_out'),
                    h('span', { class: 'cydc-zoom-label', ref: ref('zoom') }),
                    tool('zoom-in', 'zoom-in', 'cyd_creator.designer.zoom_in'),
                    tool('zoom-fit', 'fit', 'cyd_creator.designer.zoom_fit')),
                h('div', { class: 'cydc-tool-group' },
                    h('label', { class: 'cydc-snap' }, h('span', {}, t('cyd_creator.designer.snap')),
                        h('select', { 'data-cc-cmd': 'snap', ref: ref('snap') }, G.SNAP_STEPS.map(s => h('option', { value: String(s) }, s ? t('cyd_creator.designer.snap_px', { px: s }) : t('cyd_creator.designer.snap_off'))))),
                    tool('grid', 'grid', 'cyd_creator.designer.grid', { 'aria-pressed': 'false' })),
                h('div', { class: 'cydc-tool-group' },
                    ALIGN.map(m => tool('align-' + m, 'align-' + m, 'cyd_creator.designer.align_' + m)),
                    tool('distribute-h', 'distribute-h', 'cyd_creator.designer.distribute_h'),
                    tool('distribute-v', 'distribute-v', 'cyd_creator.designer.distribute_v')),
                h('div', { class: 'cydc-tool-group' }, ZORDER.map(z => tool('z-' + z, 'z-' + z, 'cyd_creator.designer.z_' + z))),
                h('div', { class: 'cydc-tool-group' },
                    tool('lock', 'lock', 'cyd_creator.designer.lock'),
                    tool('hide', 'eye-off', 'cyd_creator.designer.hide'),
                    tool('delete', 'trash', 'cyd_creator.designer.delete'))),
            h('div', { class: 'cydc-canvas-viewport', ref: ref('viewport') },
                h('div', { class: 'cydc-canvas-stage' },
                    h('div', { class: 'cydc-device-frame', ref: ref('frame') },
                        h('div', { class: 'cydc-screen-host', ref: ref('host') },
                            h('div', { class: 'cydc-screen', 'aria-hidden': 'true', ref: ref('screen') }),
                            h('div', { class: 'cydc-overlay', 'aria-hidden': 'true', ref: ref('overlay') },
                                h('div', { class: 'cydc-overlay-sel', ref: ref('sel') }),
                                h('div', { class: 'cydc-guides', ref: ref('guides') }),
                                h('div', { class: 'cydc-marquee', hidden: true, ref: ref('marquee') }),
                                h('div', { class: 'cydc-drop', hidden: true, ref: ref('drop') })))),
                    h('div', { class: 'cydc-device-label', ref: ref('label') }))),
            h('p', { class: 'cydc-sr-only', id: hintId }, t('cyd_creator.designer.canvas_hint')),
            h('div', { class: 'cydc-sr-only', 'aria-live': 'polite', ref: ref('live') })));
        host.appendChild(root);

        // The elements of the current render by widget id (ids are only checked for duplicates, so
        // they never go into a selector): the widget element, its preview node and a container's slot.
        const els = new Map();
        const views = new Map();
        const slots = new Map();
        // Widgets flashing after a remote change; a render within the flash keeps the class.
        const flashing = new Set();
        // One timer per purpose (flash, live region, selection message, first fit).
        const timers = {};
        function later(key, fn, ms) { clearTimeout(timers[key]); timers[key] = setTimeout(() => { delete timers[key]; fn(); }, ms); }
        function cancel(key) { clearTimeout(timers[key]); delete timers[key]; }
        bag.add(() => Object.keys(timers).forEach(cancel));

        const z = () => d.zoom || 1;
        function px(v) { return Math.round(v) + 'px'; }
        function place(el, r, scale) {
            const s = scale || 1;
            el.style.left = px(r.x * s); el.style.top = px(r.y * s);
            el.style.width = px(r.w * s); el.style.height = px(r.h * s);
        }
        function num(v, min, max, fallback) { return typeof v === 'number' && Number.isFinite(v) ? Math.max(min, Math.min(max, Math.round(v))) : fallback; }
        // boxOf is the widget with the box the canvas draws: finite integer position, size 1..MAX_SIDE.
        function boxOf(w) { return Object.assign({}, w, { x: num(w.x, -MAX_COORD, MAX_COORD, 0), y: num(w.y, -MAX_COORD, MAX_COORD, 0), w: num(w.w, 1, G.MAX_SIDE, 1), h: num(w.h, 1, G.MAX_SIDE, 1) }); }
        // renderOpts: the active tab of a tabview, and for statusbars the screen title and the
        // project settings (Wi-Fi, clock); P.render checks their types itself.
        function renderOpts(w) {
            const screen = d.screen();
            return { activeTab: d.activeTab(w.id), screenTitle: screen ? screen.title : '', settings: d.project().settings };
        }
        // screenBackground mirrors the device: the screen's bg_color and bg_opacity overrides when
        // P.render.colors accepts them, else the theme background.
        function screenBackground(theme, style) {
            const bg = (theme.tokens || {}).bg || '';
            const c = P.render.colors(theme, style);
            return c.bg || (c.alpha < 1 ? P.render.rgba(bg, c.alpha, bg) : bg);
        }
        function widgetEl(id) { return els.get(id) || null; }

        // ── rendering ───────────────────────────────────────────────────────
        function renderWidget(w, theme, parent) {
            const spec = d.spec(w.type);
            const b = boxOf(w);
            const el = document.createElement('div');
            el.className = 'cydc-w' + (w.locked ? ' is-locked' : '');
            el.dataset.ccWidgetId = w.id;
            place(el, b);
            const issues = d.issuesFor(w);
            if (issues.length) {
                el.classList.add(issues.some(i => i.severity === 'error') ? 'has-error' : 'has-warning');
                el.title = issues.map(d.issueText).join('\n');
            }
            const view = P.render.widget(spec, w, theme, d.inst.assetUrl, renderOpts(w));
            el.appendChild(view);
            // A duplicate id (ID_DUPLICATE issue) maps to the first widget, as M.findWidget does.
            const first = !els.has(w.id);
            if (first) { els.set(w.id, el); views.set(w.id, view); }
            if (flashing.has(w.id)) el.classList.add('cydc-flash');
            if (spec && spec.container) {
                const slot = document.createElement('div');
                slot.className = 'cydc-slot';
                place(slot, M.contentBox(b, spec));
                el.appendChild(slot);
                if (first) slots.set(w.id, slot);
                renderList(w.children || [], slot, theme, w);
            }
            if (parent && parent.type === 'tabview') el.dataset.ccPage = String(w.tab || 0);
            return el;
        }
        function renderList(list, container, theme, parent) {
            list.forEach(w => {
                if (w.hidden) return;
                if (parent && parent.type === 'tabview' && (w.tab || 0) !== d.activeTab(parent.id)) return;
                container.appendChild(renderWidget(w, theme, parent));
            });
        }

        function render() {
            const theme = d.theme();
            const tk = theme.tokens || {};
            const size = d.screenSize();
            const scale = z();
            refs.host.style.width = px(size.w * scale);
            refs.host.style.height = px(size.h * scale);
            const scr = refs.screen;
            scr.style.width = px(size.w);
            scr.style.height = px(size.h);
            scr.style.transform = 'scale(' + scale + ')';
            scr.style.fontFamily = P.render.fontFamily(theme.font);
            scr.style.fontSize = (theme.font_size || 16) + 'px';
            scr.style.color = tk.text || '';
            els.clear(); views.clear(); slots.clear();
            scr.replaceChildren();
            root.classList.toggle('is-readonly', d.readonly());
            root.classList.toggle('is-system', d.isSystem());
            const screen = d.isSystem() ? null : d.screen();
            if (screen) {
                scr.dataset.ccScreenId = screen.id;
                scr.style.background = screenBackground(theme, screen.style);
                renderList(screen.widgets || [], scr, theme, null);
            } else {
                delete scr.dataset.ccScreenId;
                scr.style.background = tk.bg || '';
                if (d.isSystem()) scr.appendChild(dom(P.render.wifiSetup(theme, size, t)));
            }
            const board = d.board();
            const status = board ? String(board.status || '') : '';
            refs.label.replaceChildren(dom(h('span', {}, board ? board.label : String(d.project().board || ''))),
                board ? dom(h('span', { class: 'cydc-tag', 'data-tone': status === 'verified' ? 'ok' : null }, t.or('cyd_creator.board.status_' + status, status))) : document.createTextNode(''),
                dom(h('span', { class: 'cydc-device-size' }, size.w + ' × ' + size.h + ' · ' + t('cyd_creator.orientation.' + (d.project().orientation || 'landscape')))));
            renderOverlay();
            syncBar();
        }

        // renderOverlay draws selection boxes, resize handles and issue dots. preview moves
        // the boxes of ids by delta (device px) or replaces one box by rect during a gesture.
        function renderOverlay(preview) {
            const scale = z();
            const ov = refs.overlay;
            ov.classList.toggle('has-grid', d.showGrid && !d.isSystem());
            ov.style.setProperty('--cc-grid', ((d.snap || 8) * scale) + 'px');
            const layer = refs.sel;
            layer.replaceChildren();
            if (d.isSystem()) return;
            const p = d.project();
            const env = d.env();
            const screen = d.screen();
            if (screen) {
                M.walk({ screens: [screen] }, w => {
                    const issues = d.issuesFor(w);
                    if (!issues.length || !els.has(w.id)) return true;
                    const r = M.absRect(p, w.id, env);
                    const dot = document.createElement('div');
                    dot.className = 'cydc-issue-dot' + (issues.some(i => i.severity === 'error') ? ' is-error' : ' is-warning');
                    dot.style.left = px((r.x + r.w) * scale - 5);
                    dot.style.top = px(r.y * scale - 5);
                    layer.appendChild(dot);
                    return true;
                });
            }
            const single = d.selection.length === 1;
            d.selection.forEach(id => {
                const f = d.find(id);
                let r = M.absRect(p, id, env);
                if (!f || !r) return;
                if (preview && preview.ids && preview.ids.has(id)) r = G.translate(r, preview.delta.x, preview.delta.y);
                if (preview && preview.rect && preview.rect.id === id) r = preview.rect;
                const box = document.createElement('div');
                box.className = 'cydc-sel' + (f.widget.locked ? ' is-locked' : '') + (els.has(id) ? '' : ' is-hidden');
                place(box, r, scale);
                if (single && !f.widget.locked && !d.readonly()) {
                    G.HANDLES.forEach(hd => {
                        const handle = document.createElement('div');
                        handle.className = 'cydc-handle cydc-handle-' + hd;
                        handle.dataset.ccHandle = hd;
                        box.appendChild(handle);
                    });
                }
                layer.appendChild(box);
            });
        }

        // setDisabled moves the focus to the canvas before a focused tool is disabled: the page would
        // get it, and the next Delete key would reach the desktop (it deletes a selected file icon).
        function setDisabled(b, off) {
            if (off && !b.disabled && document.activeElement === b) root.focus({ preventScroll: true });
            b.disabled = off;
        }
        function syncBar() {
            const n = d.selection.length;
            const ro = d.readonly() || d.isSystem();
            refs.zoom.textContent = z() + '×';
            refs.snap.value = String(d.snap);
            tools.get('grid').setAttribute('aria-pressed', String(d.showGrid));
            const need = { 'distribute-h': 3, 'distribute-v': 3 };
            tools.forEach((b, cmd) => {
                if (/^(align|distribute|z-|lock|hide|delete)/.test(cmd)) setDisabled(b, ro || n < (need[cmd] || 1));
            });
            setDisabled(tools.get('zoom-out'), z() <= G.ZOOM_MIN);
            setDisabled(tools.get('zoom-in'), z() >= G.ZOOM_MAX);
        }

        function fit(auto) {
            const vp = refs.viewport;
            // Code mode hides the canvas (0×0): keep the zoom until it is shown again.
            if (!vp.clientWidth) return;
            d.setZoom(fitZoom(d.screenSize(), vp.clientWidth, vp.clientHeight), !!auto);
        }

        // ── gesture previews (called by interact) ──────────────────────────
        function previewMove(ids, delta) {
            ids.forEach(id => {
                const el = els.get(id);
                const f = d.find(id);
                if (el && f) { const b = boxOf(f.widget); el.style.left = px(b.x + delta.x); el.style.top = px(b.y + delta.y); }
            });
            renderOverlay({ ids: new Set(ids), delta });
        }
        function previewResize(id, abs) {
            const f = d.find(id);
            const el = els.get(id);
            const old = views.get(id);
            if (!f || !el || !old) return;
            const o = M.absOrigin(d.project(), id, d.env());
            const w = boxOf(Object.assign({}, f.widget, { x: abs.x - o.x, y: abs.y - o.y, w: abs.w, h: abs.h }));
            const spec = d.spec(w.type);
            place(el, w);
            const view = P.render.widget(spec, w, d.theme(), d.inst.assetUrl, renderOpts(w));
            el.replaceChild(view, old);
            views.set(id, view);
            const slot = slots.get(id);
            if (slot) place(slot, M.contentBox(w, spec));
            renderOverlay({ rect: Object.assign({ id }, abs) });
        }
        function showGuides(guides) {
            const scale = z();
            refs.guides.replaceChildren();
            (guides && guides.x || []).forEach(x => { const g = document.createElement('div'); g.className = 'cydc-guide cydc-guide-v'; g.style.left = px(x * scale); refs.guides.appendChild(g); });
            (guides && guides.y || []).forEach(y => { const g = document.createElement('div'); g.className = 'cydc-guide cydc-guide-h'; g.style.top = px(y * scale); refs.guides.appendChild(g); });
        }
        function showRect(el, r) { el.hidden = !r; if (r) place(el, r, z()); }

        function flash(ids) {
            ids.forEach(id => {
                flashing.add(id);
                const el = els.get(id);
                if (!el) return;
                el.classList.remove('cydc-flash');
                void el.offsetWidth; // restarts the animation
                el.classList.add('cydc-flash');
            });
            if (!flashing.size) return;
            later('flash', () => {
                flashing.forEach(id => { const el = els.get(id); if (el) el.classList.remove('cydc-flash'); });
                flashing.clear();
            }, FLASH_MS);
        }
        function changedIds(before, after) {
            const index = project => { const m = new Map(); if (project) M.walk(project, w => { const copy = Object.assign({}, w); delete copy.children; m.set(w.id, JSON.stringify(copy)); }); return m; };
            const a = index(before);
            const out = [];
            index(after).forEach((json, id) => { if (a.get(id) !== json) out.push(id); });
            return out;
        }

        // ── live region ─────────────────────────────────────────────────────
        function speak(text) {
            refs.live.textContent = '';
            later('speak', () => { refs.live.textContent = text; }, 30);
        }
        // announce speaks a command's message; it replaces a pending selection message.
        function announce(text) { cancel('selection'); speak(text); }
        // Selection changes are spoken once the selection settles (clicks, box select, keys).
        let spoken = '';
        function selectionText() {
            const ids = d.selection;
            if (!ids.length) return t.or('cyd_creator.designer.selection_none', 'Selection cleared');
            if (ids.length > 1) return t.or('cyd_creator.designer.selection_many', '{count} widgets selected', { count: ids.length });
            const f = d.find(ids[0]);
            const name = f ? (f.widget.name || d.label(f.widget.type)) : '';
            return t.or('cyd_creator.designer.selection_one', '{name} selected', { name });
        }
        function selectionChanged() {
            const key = d.selection.join(' ');
            if (key === spoken) return;
            spoken = key;
            later('selection', () => speak(selectionText()), SELECTION_SPEAK_MS);
        }

        // ── wiring ──────────────────────────────────────────────────────────
        bag.on(refs.bar, 'click', event => {
            const b = event.target.closest('button[data-cc-cmd]');
            if (!b || b.disabled) return;
            const cmd = b.dataset.ccCmd;
            const c = d.commands;
            if (cmd === 'zoom-in') d.setZoom(z() + 1);
            else if (cmd === 'zoom-out') d.setZoom(z() - 1);
            else if (cmd === 'zoom-fit') fit(true);
            else if (cmd === 'grid') d.setGrid(!d.showGrid);
            else if (cmd.startsWith('align-')) c.align(cmd.slice(6));
            else if (cmd === 'distribute-h') c.distribute('h');
            else if (cmd === 'distribute-v') c.distribute('v');
            else if (cmd.startsWith('z-')) c.zorder(cmd.slice(2));
            else if (cmd === 'lock') c.toggle('locked');
            else if (cmd === 'hide') c.toggle('hidden');
            else if (cmd === 'delete') c.remove();
        });
        bag.on(refs.snap, 'change', () => d.setSnap(Number(refs.snap.value)));
        bag.add(d.bus.on('project', info => {
            // A change that switched the screen was rendered by the 'screen' event before it.
            if (!(info && info.screenChanged)) render();
            if (info && info.origin === 'remote') flash(changedIds(info.before, d.project()));
        }));
        bag.add(d.bus.on('selection', () => { renderOverlay(); syncBar(); selectionChanged(); }));
        bag.add(d.bus.on('screen', () => { render(); refs.viewport.scrollTop = 0; }));
        bag.add(d.bus.on('view', render));
        bag.add(d.bus.on('issues', render));
        bag.add(d.bus.on('lock', render));
        if (typeof ResizeObserver === 'function') {
            const ro = new ResizeObserver(() => { if (d.zoomAuto) fit(true); });
            ro.observe(refs.viewport);
            bag.add(() => ro.disconnect());
        }
        render();
        later('fit', () => { if (d.zoomAuto) fit(true); }, 0);

        return {
            root, viewport: refs.viewport, host: refs.host, screenEl: refs.screen, overlay: refs.overlay,
            render, renderOverlay, fit, widgetEl, previewMove, previewResize, showGuides, flash, announce,
            showMarquee: r => showRect(refs.marquee, r),
            showDrop: r => showRect(refs.drop, r),
            hostRect: () => refs.host.getBoundingClientRect(),
            clientToWorld: (x, y) => G.toWorld({ x, y }, refs.host.getBoundingClientRect(), z()),
            scrollTo(id) { const el = els.get(id); if (el && el.scrollIntoView) el.scrollIntoView({ block: 'nearest', inline: 'nearest' }); },
            dispose() { bag.dispose(); els.clear(); views.clear(); slots.clear(); flashing.clear(); root.remove(); }
        };
    }

    P.designerCanvas = { mount, fitZoom, BEZEL, PAD, LABEL_H };
})();
