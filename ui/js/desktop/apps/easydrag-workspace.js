// Device-local panel geometry; it never changes a flow document or its undo history.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const KEY = 'aurago.easydrag.workspace';

    function create(ed, canvas, palette) {
        const core = ED.core, bag = core.bag();
        const saved = core.storage.get(KEY, {}) || {};
        const prefs = {
            inspector: Number.isFinite(saved.inspector) ? saved.inspector : 400,
            palette: Number.isFinite(saved.palette) ? saved.palette : 280,
            outline: saved.outline === true
        };
        let drag = null, dead = false, layoutChosen = false;
        const controls = new Map();
        const widths = new Map();
        const frame = core.frame(() => { if (!dead) canvas.ensureSelected(); });
        const save = () => core.storage.set(KEY, prefs);

        function bounds(kind) {
            const width = ed.root.getBoundingClientRect().width;
            const outline = ed.outlineOpen && width > 900 ? 240 : 0;
            const library = palette.isOpen() && width >= 1180 ? prefs.palette : 0;
            return kind === 'inspector' ? { min: 320, max: Math.max(320, Math.min(800, width - 370 - outline - library)) }
                : { min: 220, max: Math.max(220, Math.min(380, width - (ed.detail ? prefs.inspector : 0) - 370 - outline)) };
        }

        function apply(kind, value) {
            const limit = bounds(kind);
            const width = Math.round(Math.max(limit.min, Math.min(limit.max, value)));
            if (widths.get(kind) !== width) { ed.root.style.setProperty('--ed-' + kind + '-width', width + 'px'); widths.set(kind, width); }
            const control = controls.get(kind);
            if (control) {
                control.setAttribute('aria-valuemin', String(limit.min));
                control.setAttribute('aria-valuemax', String(limit.max));
                control.setAttribute('aria-valuenow', String(width));
            }
            frame.request();
            return width;
        }

        function finish() {
            if (!drag) return;
            const old = drag;
            drag = null;
            if (old.control.hasPointerCapture && old.control.hasPointerCapture(old.id)) old.control.releasePointerCapture(old.id);
            ed.root.classList.remove('is-resizing-panel');
            save();
        }

        function resizeControl(host, kind) {
            if (controls.has(kind)) controls.get(kind).remove();
            const label = kind === 'inspector' ? ed.t('easydrag.ui.panel_resize_inspector') : ed.t('easydrag.ui.panel_resize_palette');
            const control = core.el('<div class="ed-panel-resize ed-panel-resize--' + kind + '" role="separator" tabindex="0" aria-orientation="vertical" aria-label="' + ed.esc(label) + '"></div>');
            controls.set(kind, control);
            host.appendChild(control);
            control.addEventListener('pointerdown', event => {
                if (event.button !== 0 || ed.root.getBoundingClientRect().width < 760) return;
                event.preventDefault();
                control.focus({ preventScroll: true });
                drag = { kind, id: event.pointerId, control, x: event.clientX, width: Number(control.getAttribute('aria-valuenow')) };
                control.setPointerCapture(event.pointerId);
                ed.root.classList.add('is-resizing-panel');
            });
            control.addEventListener('pointermove', event => {
                if (!drag || drag.control !== control || drag.id !== event.pointerId) return;
                prefs[kind] = apply(kind, drag.width + (event.clientX - drag.x) * (kind === 'inspector' ? -1 : 1));
            });
            ['pointerup', 'pointercancel', 'lostpointercapture'].forEach(name => control.addEventListener(name, finish));
            control.addEventListener('keydown', event => {
                if (event.isComposing || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
                event.preventDefault();
                const limit = bounds(kind), step = event.shiftKey ? 48 : 16;
                const delta = (event.key === 'ArrowRight' ? 1 : -1) * (kind === 'inspector' ? -1 : 1) * step;
                prefs[kind] = apply(kind, event.key === 'Home' ? limit.min : event.key === 'End' ? limit.max : Number(control.getAttribute('aria-valuenow')) + delta);
                save();
            });
            apply(kind, prefs[kind]);
        }

        resizeControl(palette.el, 'palette');
        const outlineButton = core.el('<button type="button" class="ed-icon-btn" aria-pressed="false" title="' + ed.esc(ed.t('easydrag.ui.node_list')) + '" aria-label="' + ed.esc(ed.t('easydrag.ui.node_list')) + '">' + core.icon('list') + '</button>');
        const focusButton = core.el('<button type="button" class="ed-icon-btn" title="' + ed.esc(ed.t('easydrag.ui.focus_selection')) + '" aria-label="' + ed.esc(ed.t('easydrag.ui.focus_selection')) + '">' + core.icon('fit') + '</button>');
        const rail = ed.root.querySelector('.ed-rail');
        rail.insertBefore(outlineButton, rail.querySelector('.ed-rail-spacer'));
        rail.insertBefore(focusButton, rail.querySelector('.ed-rail-spacer'));
        bag.listen(outlineButton, 'click', () => { palette.setOpen(false); canvas.setOutline(!ed.outlineOpen); });
        bag.listen(focusButton, 'click', () => canvas.focusSelection());
        bag.add(ed.bus.on('outline', open => { prefs.outline = open; outlineButton.setAttribute('aria-pressed', String(open)); save(); frame.request(); }));
        bag.add(ed.bus.on('detail-opened', host => {
            if (ed.root.getBoundingClientRect().width < 1500) palette.setOpen(false, true);
            if (ed.root.getBoundingClientRect().width < 1200 && ed.outlineOpen) canvas.setOutline(false);
            resizeControl(host, 'inspector'); frame.request();
        }));
        bag.add(ed.bus.on('detail-closed', () => { controls.delete('inspector'); frame.request(); }));
        bag.add(ed.bus.on('panel-size-menu', ({ kind, anchor }) => {
            if (!anchor || !['palette', 'inspector'].includes(kind) || typeof ed.ctx.showContextMenu !== 'function') return;
            const limit = bounds(kind), rect = anchor.getBoundingClientRect();
            const choices = Array.from(new Set([limit.min, kind === 'inspector' ? 400 : 280, Math.round((limit.min + limit.max) / 2), limit.max])).filter(width => width >= limit.min && width <= limit.max).sort((a, b) => a - b);
            ed.ctx.showContextMenu(rect.left, rect.bottom, choices.map(width => ({ label: width + ' px', action: () => { prefs[kind] = apply(kind, width); save(); anchor.focus({ preventScroll: true }); } })));
        }));
        const layout = core.frame(() => {
            if (dead) return;
            chooseLayout();
            apply('inspector', prefs.inspector);
            apply('palette', prefs.palette);
        });
        const observer = new ResizeObserver(() => layout.request());
        observer.observe(ed.root);
        observer.observe(canvas.el);
        const results = ED.testing && ED.testing.createResults(ed, canvas);
        function chooseLayout() {
            if (layoutChosen || palette.hasPreference() || !ed.model.doc.nodes.length) return;
            const box = ED.geometry.bounds(ed.model.doc.nodes.map(n => canvas.nodeRect(n.id)).filter(Boolean));
            const width = ed.root.getBoundingClientRect().width;
            if (!width) return;
            layoutChosen = true;
            if (box && box.w * 0.8 + 96 > width - prefs.palette - 48) { palette.setOpen(false, true); canvas.fit({ readable: true }); }
        }
        chooseLayout();
        if (prefs.outline) { palette.setOpen(false); canvas.setOutline(true); }
        return { dispose() { dead = true; finish(); frame.cancel(); layout.cancel(); observer.disconnect(); bag.dispose(); if (results) results.dispose(); controls.forEach(control => control.remove()); outlineButton.remove(); focusButton.remove(); } };
    }
    ED.workspace = { create };
})();
