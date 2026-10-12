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
    const LABEL_H = 30;
    const MAX_COORD = 4096; // maxCoordinate, internal/cydcreator/limits.go
    const ALIGN = ['left', 'hcenter', 'right', 'top', 'vcenter', 'bottom'];
    const ZORDER = ['front', 'forward', 'backward', 'back'];

    function mount(d, host) {
        const { h, dom, icon } = P.render;
        const M = P.model;
        const G = P.geometry;
        const t = d.t;
        const bag = P.util.bag();
        const refs = {};
        const hintId = P.util.uid('cc_hint');
        const tool = (cmd, name, key, extra) => h('button', Object.assign({ type: 'button', class: 'cc-tool', 'data-cc-cmd': cmd, title: t(key), 'aria-label': t(key) }, extra || {}), icon(name));
        const ref = key => el => { refs[key] = el; };

        const root = dom(h('div', { class: 'cc-canvas', tabindex: '0', role: 'group', 'aria-roledescription': t('cyd_creator.designer.canvas_role'), 'aria-label': t('cyd_creator.designer.canvas_label'), 'aria-describedby': hintId, ref: ref('root') },
            h('div', { class: 'cc-canvas-bar', role: 'toolbar', 'aria-label': t('cyd_creator.designer.toolbar_label'), ref: ref('bar') },
                h('div', { class: 'cc-tool-group' },
                    tool('zoom-out', 'zoom-out', 'cyd_creator.designer.zoom_out'),
                    h('span', { class: 'cc-zoom-label', ref: ref('zoom') }),
                    tool('zoom-in', 'zoom-in', 'cyd_creator.designer.zoom_in'),
                    tool('zoom-fit', 'fit', 'cyd_creator.designer.zoom_fit')),
                h('div', { class: 'cc-tool-group' },
                    h('label', { class: 'cc-snap' }, h('span', {}, t('cyd_creator.designer.snap')),
                        h('select', { 'data-cc-cmd': 'snap', ref: ref('snap') }, G.SNAP_STEPS.map(s => h('option', { value: String(s) }, s ? t('cyd_creator.designer.snap_px', { px: s }) : t('cyd_creator.designer.snap_off'))))),
                    tool('grid', 'grid', 'cyd_creator.designer.grid', { 'aria-pressed': 'false' })),
                h('div', { class: 'cc-tool-group' },
                    ALIGN.map(m => tool('align-' + m, 'align-' + m, 'cyd_creator.designer.align_' + m)),
                    tool('distribute-h', 'distribute-h', 'cyd_creator.designer.distribute_h'),
                    tool('distribute-v', 'distribute-v', 'cyd_creator.designer.distribute_v')),
                h('div', { class: 'cc-tool-group' }, ZORDER.map(z => tool('z-' + z, 'z-' + z, 'cyd_creator.designer.z_' + z))),
                h('div', { class: 'cc-tool-group' },
                    tool('lock', 'lock', 'cyd_creator.designer.lock'),
                    tool('hide', 'eye-off', 'cyd_creator.designer.hide'),
                    tool('delete', 'trash', 'cyd_creator.designer.delete'))),
            h('div', { class: 'cc-canvas-viewport', ref: ref('viewport') },
                h('div', { class: 'cc-canvas-stage' },
                    h('div', { class: 'cc-device-frame', ref: ref('frame') },
                        h('div', { class: 'cc-screen-host', ref: ref('host') },
                            h('div', { class: 'cc-screen', 'aria-hidden': 'true', ref: ref('screen') }),
                            h('div', { class: 'cc-overlay', 'aria-hidden': 'true', ref: ref('overlay') },
                                h('div', { class: 'cc-overlay-sel', ref: ref('sel') }),
                                h('div', { class: 'cc-guides', ref: ref('guides') }),
                                h('div', { class: 'cc-marquee', hidden: true, ref: ref('marquee') }),
                                h('div', { class: 'cc-drop', hidden: true, ref: ref('drop') })))),
                    h('div', { class: 'cc-device-label', ref: ref('label') }))),
            h('p', { class: 'cc-sr-only', id: hintId }, t('cyd_creator.designer.canvas_hint')),
            h('div', { class: 'cc-sr-only', 'aria-live': 'polite', ref: ref('live') })));
        host.appendChild(root);

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
        function widgetEl(id) { return refs.screen.querySelector('[data-cc-widget-id="' + String(id).replace(/["\\]/g, '') + '"]'); }

        // ── rendering ───────────────────────────────────────────────────────
        function renderWidget(w, theme, parent) {
            const spec = d.spec(w.type);
            const b = boxOf(w);
            const el = document.createElement('div');
            el.className = 'cc-w' + (w.locked ? ' is-locked' : '');
            el.dataset.ccWidgetId = w.id;
            place(el, b);
            const issues = d.issuesFor(w);
            if (issues.length) {
                el.classList.add(issues.some(i => i.severity === 'error') ? 'has-error' : 'has-warning');
                el.title = issues.map(d.issueText).join('\n');
            }
            el.appendChild(P.render.widget(spec, w, theme, d.inst.assetUrl, renderOpts(w)));
            if (spec && spec.container) {
                const slot = document.createElement('div');
                slot.className = 'cc-slot';
                place(slot, M.contentBox(b, spec));
                el.appendChild(slot);
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
            scr.replaceChildren();
            root.classList.toggle('is-readonly', d.readonly());
            root.classList.toggle('is-system', d.isSystem());
            if (d.isSystem()) {
                delete scr.dataset.ccScreenId;
                scr.style.background = tk.bg || '';
                scr.appendChild(dom(P.render.wifiSetup(theme, size, t)));
            } else {
                const screen = d.screen();
                if (screen) {
                    scr.dataset.ccScreenId = screen.id;
                    // The screen's bg_color override only when it is a colour the server accepts.
                    scr.style.background = P.render.colors(theme, { bg_color: screen.style && screen.style.bg_color }).bg || tk.bg || '';
                    renderList(screen.widgets || [], scr, theme, null);
                }
            }
            const board = d.board();
            refs.label.replaceChildren(dom(h('span', {}, board ? board.label : d.project().board)),
                board ? dom(h('span', { class: 'cc-badge cc-badge-' + board.status }, t('cyd_creator.board.status_' + board.status))) : document.createTextNode(''),
                dom(h('span', { class: 'cc-device-size' }, size.w + ' × ' + size.h + ' · ' + t('cyd_creator.orientation.' + (d.project().orientation || 'landscape')))));
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
                    if (!issues.length || !widgetEl(w.id)) return true;
                    const r = M.absRect(p, w.id, env);
                    const dot = document.createElement('div');
                    dot.className = 'cc-issue-dot' + (issues.some(i => i.severity === 'error') ? ' is-error' : ' is-warning');
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
                box.className = 'cc-sel' + (f.widget.locked ? ' is-locked' : '') + (widgetEl(id) ? '' : ' is-hidden');
                place(box, r, scale);
                if (single && !f.widget.locked && !d.readonly()) {
                    G.HANDLES.forEach(hd => {
                        const handle = document.createElement('div');
                        handle.className = 'cc-handle cc-handle-' + hd;
                        handle.dataset.ccHandle = hd;
                        box.appendChild(handle);
                    });
                }
                layer.appendChild(box);
            });
        }

        function syncBar() {
            const n = d.selection.length;
            const ro = d.readonly() || d.isSystem();
            refs.zoom.textContent = z() + '×';
            refs.snap.value = String(d.snap);
            const grid = refs.bar.querySelector('[data-cc-cmd="grid"]');
            grid.setAttribute('aria-pressed', String(d.showGrid));
            const need = { 'distribute-h': 3, 'distribute-v': 3 };
            refs.bar.querySelectorAll('[data-cc-cmd]').forEach(b => {
                const cmd = b.dataset.ccCmd;
                if (/^(align|distribute|z-|lock|hide|delete)/.test(cmd)) b.disabled = ro || n < (need[cmd] || 1);
            });
            refs.bar.querySelector('[data-cc-cmd="zoom-out"]').disabled = z() <= G.ZOOM_MIN;
            refs.bar.querySelector('[data-cc-cmd="zoom-in"]').disabled = z() >= G.ZOOM_MAX;
        }

        function fit(auto) {
            const vp = refs.viewport;
            // Code mode hides the canvas (0×0): keep the zoom until it is shown again.
            if (!vp.clientWidth) return;
            const size = d.screenSize();
            const zoom = G.fitZoom({ w: size.w + BEZEL * 2, h: size.h + BEZEL * 2 + LABEL_H }, { w: vp.clientWidth, h: vp.clientHeight }, 12);
            d.setZoom(zoom, !!auto);
        }

        // ── gesture previews (called by interact) ──────────────────────────
        function previewMove(ids, delta) {
            ids.forEach(id => {
                const el = widgetEl(id);
                const f = d.find(id);
                if (el && f) { const b = boxOf(f.widget); el.style.left = px(b.x + delta.x); el.style.top = px(b.y + delta.y); }
            });
            renderOverlay({ ids: new Set(ids), delta });
        }
        function previewResize(id, abs) {
            const f = d.find(id);
            const el = widgetEl(id);
            if (!f || !el) return;
            const o = M.absOrigin(d.project(), id, d.env());
            const w = boxOf(Object.assign({}, f.widget, { x: abs.x - o.x, y: abs.y - o.y, w: abs.w, h: abs.h }));
            const spec = d.spec(w.type);
            place(el, w);
            el.replaceChild(P.render.widget(spec, w, d.theme(), d.inst.assetUrl, renderOpts(w)), el.firstChild);
            const slot = el.querySelector(':scope > .cc-slot');
            if (slot) place(slot, M.contentBox(w, spec));
            renderOverlay({ rect: Object.assign({ id }, abs) });
        }
        function showGuides(guides) {
            const scale = z();
            refs.guides.replaceChildren();
            (guides && guides.x || []).forEach(x => { const g = document.createElement('div'); g.className = 'cc-guide cc-guide-v'; g.style.left = px(x * scale); refs.guides.appendChild(g); });
            (guides && guides.y || []).forEach(y => { const g = document.createElement('div'); g.className = 'cc-guide cc-guide-h'; g.style.top = px(y * scale); refs.guides.appendChild(g); });
        }
        function showRect(el, r) { el.hidden = !r; if (r) place(el, r, z()); }

        function flash(ids) {
            ids.forEach(id => {
                const el = widgetEl(id);
                if (!el) return;
                el.classList.remove('cc-flash');
                void el.offsetWidth;
                el.classList.add('cc-flash');
                bag.timeout(() => el.classList.remove('cc-flash'), 1300);
            });
        }
        function changedIds(before, after) {
            const index = project => { const m = new Map(); if (project) M.walk(project, w => { const copy = Object.assign({}, w); delete copy.children; m.set(w.id, JSON.stringify(copy)); }); return m; };
            const a = index(before);
            const out = [];
            index(after).forEach((json, id) => { if (a.get(id) !== json) out.push(id); });
            return out;
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
            render();
            if (info && info.origin === 'remote') flash(changedIds(info.before, d.project()));
        }));
        bag.add(d.bus.on('selection', () => { renderOverlay(); syncBar(); }));
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
        bag.timeout(() => { if (d.zoomAuto) fit(true); }, 0);

        return {
            root, viewport: refs.viewport, host: refs.host, screenEl: refs.screen, overlay: refs.overlay,
            render, renderOverlay, fit, widgetEl, previewMove, previewResize, showGuides, flash,
            showMarquee: r => showRect(refs.marquee, r),
            showDrop: r => showRect(refs.drop, r),
            hostRect: () => refs.host.getBoundingClientRect(),
            clientToWorld: (x, y) => G.toWorld({ x, y }, refs.host.getBoundingClientRect(), z()),
            scrollTo(id) { const el = widgetEl(id); if (el && el.scrollIntoView) el.scrollIntoView({ block: 'nearest', inline: 'nearest' }); },
            announce(text) { refs.live.textContent = ''; bag.timeout(() => { refs.live.textContent = text; }, 30); },
            dispose() { bag.dispose(); root.remove(); }
        };
    }

    P.designerCanvas = { mount, BEZEL };
})();
