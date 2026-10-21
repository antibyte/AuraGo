(function () {
    'use strict';
    const Pixel = window.AuraPixelApp;
    const normalized = m => m.map(value => Math.round(value * 1e10) / 1e10);
    const point = (m, x, y) => ({ x: m[0] * x + m[2] * y + m[4], y: m[1] * x + m[3] * y + m[5] });
    const centre = m => point(m, 512, 512);
    const multiply = (a, b) => [a[0]*b[0]+a[2]*b[1], a[1]*b[0]+a[3]*b[1], a[0]*b[2]+a[2]*b[3], a[1]*b[2]+a[3]*b[3], a[0]*b[4]+a[2]*b[5]+a[4], a[1]*b[4]+a[3]*b[5]+a[5]];
    const width = m => Math.hypot(m[0], m[1]) * 1024;
    Pixel.installStickers = function installStickers(r) {
        let picker, frame, controls, gesture = null;
        const abort = new AbortController(), loads = new Set();
        const tr = key => r.t('stickers.' + key);
        const active = () => r.layers[r.activeLayerIdx];
        function paint(layer) {
            if (!layer || !layer.sticker || !layer.stickerImage) return;
            layer.canvas.width = r.imgWidth; layer.canvas.height = r.imgHeight;
            const ctx = layer.canvas.getContext('2d', { willReadFrequently: true });
            ctx.save(); ctx.imageSmoothingQuality = 'high';
            ctx.setTransform(...layer.sticker.matrix);
            ctx.drawImage(layer.stickerImage, 0, 0);
            ctx.restore();
            r.markLayerDirty(layer);
        }
        function cancel() {
            if (!gesture) return;
            const old = gesture; gesture = null;
            old.layer.sticker.matrix = old.matrix;
            paint(old.layer); r.compositeLayers();
            if (frame && frame.hasPointerCapture(old.pointer)) frame.releasePointerCapture(old.pointer);
        }
        function refresh() {
            if (!frame || r.state.disposed) return;
            picker?.setDisabled(!r.imgWidth || !r.imgHeight);
            r.host.querySelector('[data-sticker-hint]').textContent = r.imgWidth ? tr('sessionHint') : tr('openImage');
            const layer = active(), sticker = layer && layer.sticker;
            controls.hidden = !sticker;
            frame.replaceChildren();
            frame.toggleAttribute('hidden', !sticker || !layer.visible || !!r.cropState || (r.activeTool && r.activeTool !== 'move'));
            if (!sticker) return;
            const m = sticker.matrix, c = centre(m);
            const current = { x: c.x, y: c.y, size: width(m), angle: Math.atan2(m[1], m[0]) * 180 / Math.PI };
            Object.entries(current).forEach(([key, value]) => {
                const input = controls.querySelector('[data-sticker-field="' + key + '"]');
                if (input !== document.activeElement) input.value = String(Math.round(value * 10) / 10);
            });
            if (frame.hasAttribute('hidden')) return;
            frame.setAttribute('viewBox', '0 0 ' + r.imgWidth + ' ' + r.imgHeight);
            const points = [[0, 0], [1024, 0], [1024, 1024], [0, 1024]].map(([x, y]) => point(m, x, y));
            const outline = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
            outline.setAttribute('points', points.map(p => p.x + ',' + p.y).join(' '));
            outline.dataset.stickerHandle = 'move'; outline.setAttribute('vector-effect', 'non-scaling-stroke'); frame.append(outline);
            const ratio = r.imgWidth / Math.max(1, r.canvas.getBoundingClientRect().width);
            const rotation = point(m, 512, -100);
            points.concat([rotation]).forEach((p, index) => {
                const handle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
                handle.setAttribute('cx', p.x); handle.setAttribute('cy', p.y); handle.setAttribute('r', 6 * ratio);
                handle.setAttribute('vector-effect', 'non-scaling-stroke');
                handle.dataset.stickerHandle = index === 4 ? 'rotate' : 'scale'; frame.append(handle);
            });
        }
        function changed(layer, matrix) {
            if (!matrix.every(Number.isFinite) || width(matrix) < 1 || width(matrix) > 32768) return;
            const next = normalized(matrix);
            if (next.every((value, i) => value === layer.sticker.matrix[i])) return false;
            layer.sticker.matrix = next; paint(layer); r.compositeLayers(); return true;
        }
        function transformAt(m, scale, angle, dx, dy) {
            const c = centre(m);
            const radians = (angle || 0) * Math.PI / 180, cos = Math.cos(radians) * scale, sin = Math.sin(radians) * scale;
            const next = multiply([cos, sin, -sin, cos, 0, 0], m);
            next[4] = c.x + (dx || 0) - 512 * (next[0] + next[2]);
            next[5] = c.y + (dy || 0) - 512 * (next[1] + next[3]);
            return next;
        }
        Object.assign(r, {
            isStickerLayer: () => !!(active() && active().sticker),
            requireRasterLayer(layer) {
                const target = layer || active();
                if (!target?.sticker) return true;
                // Masks may be painted on sticker layers; only the sticker pixels are protected.
                if (r.editMaskMode && target.maskCanvas && target === active()) return true;
                r.notify({ type: 'info', message: tr('rasterizeHint') });
                r.showPanel('stickers'); refresh(); return false;
            },
            repaintSticker: paint,
            refreshStickerControls: refresh,
            cancelStickerGesture: cancel,
            transformStickerLayers(matrix) {
                r.layers.forEach(layer => {
                    if (layer.sticker) {
                        layer.sticker.matrix = normalized(multiply(matrix, layer.sticker.matrix));
                        paint(layer);
                    }
                });
            },
            async addSticker(id) {
                if (!r.imgWidth || !r.imgHeight) { r.notify({ type: 'info', message: tr('openImage') }); return; }
                if (r.layers.length >= r.MAX_LAYERS) { r.notify({ type: 'error', message: r.t('pixel.max_layers') }); return; }
                const revision = r.state.documentRevision, layers = r.layers, w = r.imgWidth, h = r.imgHeight;
                const controller = new AbortController(); loads.add(controller);
                try {
                    const image = await window.AuraStickerLibrary.image(id, controller.signal);
                    if (r.state.disposed || revision !== r.state.documentRevision || layers !== r.layers || w !== r.imgWidth || h !== r.imgHeight) return;
                    if (r.layers.length >= r.MAX_LAYERS) { r.notify({ type: 'error', message: r.t('pixel.max_layers') }); return; }
                    r.resetFilterPreview(); r.commitPendingPreviews(); r.deselect(); r.editMaskMode = false;
                    const side = .24 * Math.min(w, h);
                    const layer = r.createLayer({ name: tr('name.' + id), stickerImage: image,
                        sticker: { id, matrix: normalized([side / 1024, 0, 0, side / 1024, (w - side) / 2, (h - side) / 2]) } });
                    paint(layer); r.layers.push(layer); r.activeLayerIdx = r.layers.length - 1;
                    if (r.activeTool !== 'move') r.setActiveTool('move');
                    r.compositeLayers(); r.refreshLayerPanel(); r.pushHistory(tr('add'));
                } finally { loads.delete(controller); }
            },
            rasterizeSticker() {
                const layer = active(); if (!layer?.sticker) return;
                cancel(); paint(layer); delete layer.sticker; delete layer.stickerImage;
                // The canvas now holds raster pixels; bump the rev so the history snapshots them instead of sharing.
                r.markLayerDirty(layer);
                r.captureAdjustmentSource();
                r.pushHistory(tr('rasterize')); r.refreshLayerPanel(); refresh();
            },
            showStickers() {
                if (!picker) picker = window.AuraStickerLibrary.mount(r.host.querySelector('[data-pixel-stickers]'), { t: r.t, onSelect: r.addSticker });
                picker.setActive(true); picker.setDisabled(!r.imgWidth || !r.imgHeight);
                const hint = r.host.querySelector('[data-sticker-hint]');
                hint.textContent = r.imgWidth ? tr('sessionHint') : tr('openImage');
                refresh();
            },
            resetStickerDocument() {
                cancel(); loads.forEach(controller => controller.abort()); loads.clear();
                r.state.documentRevision = (r.state.documentRevision || 0) + 1;
            },
            mountStickers() {
                const tab = document.createElement('button'); tab.type = 'button'; tab.className = 'pixel-tab';
                tab.dataset.panel = 'stickers'; tab.textContent = tr('title'); r.host.querySelector('.pixel-tabs').append(tab);
                const host = document.createElement('div'); host.className = 'pixel-panel-section'; host.dataset.section = 'stickers'; host.hidden = true;
                const hint = document.createElement('p'); hint.dataset.stickerHint = ''; hint.textContent = tr('openImage'); host.append(hint);
                const library = document.createElement('div'); library.dataset.pixelStickers = ''; host.append(library);
                r.host.querySelector('[data-section="transform"]').parentElement.append(host);
                controls = document.createElement('fieldset'); controls.className = 'pixel-sticker-controls'; controls.hidden = true;
                const legend = document.createElement('legend'); legend.textContent = tr('transform'); controls.append(legend);
                for (const key of ['x', 'y', 'size', 'angle']) {
                    const label = document.createElement('label'); label.textContent = tr(key);
                    const input = document.createElement('input'); input.type = 'number'; input.step = '.1'; input.dataset.stickerField = key;
                    if (key === 'size') { input.min = '1'; input.max = '32768'; }
                    label.append(input); controls.append(label);
                }
                const rasterize = document.createElement('button'); rasterize.type = 'button'; rasterize.className = 'pixel-btn'; rasterize.textContent = tr('rasterize');
                rasterize.addEventListener('click', r.rasterizeSticker, { signal: abort.signal }); controls.append(rasterize); host.prepend(controls);
                controls.addEventListener('change', event => {
                    const input = event.target.closest('[data-sticker-field]'), layer = active();
                    if (!input || !layer?.sticker || !Number.isFinite(input.valueAsNumber)) return;
                    cancel();
                    const m = layer.sticker.matrix, c = centre(m), key = input.dataset.stickerField, v = input.valueAsNumber;
                    if (key === 'size' && (v < 1 || v > 32768)) { input.value = String(width(m)); return; }
                    const next = transformAt(m, key === 'size' ? v / width(m) : 1, key === 'angle' ? v - Math.atan2(m[1], m[0]) * 180 / Math.PI : 0, key === 'x' ? v - c.x : 0, key === 'y' ? v - c.y : 0);
                    if (changed(layer, next)) r.pushHistory(tr('transform'));
                    refresh();
                }, { signal: abort.signal });
                frame = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
                frame.classList.add('pixel-sticker-frame'); frame.setAttribute('aria-hidden', 'true'); frame.setAttribute('hidden', '');
                r.canvasWrap.append(frame);
                r.canvasWrap.addEventListener('pointerdown', event => {
                    const handle = event.target.closest('[data-sticker-handle]');
                    if (event.button !== 0 || gesture || r.spaceHeld || r.cropState || r.compareMode ||
                        (r.activeTool && r.activeTool !== 'move') || (r.selection && !handle)) return;
                    const start = r.canvasCoords(event);
                    let index = r.activeLayerIdx;
                    const mode = handle?.dataset.stickerHandle || 'move';
                    if (mode === 'move') {
                        index = -1;
                        if (start.x < 0 || start.y < 0 || start.x >= r.imgWidth || start.y >= r.imgHeight) return;
                        // Pick rendered pixels so rotation, transparency, masks and stacking match the canvas.
                        for (let i = r.layers.length - 1; i >= 0; i--) {
                            const candidate = r.layers[i];
                            if (!candidate.visible || candidate.opacity <= 0 || !candidate.canvas) continue;
                            if (!candidate.canvas.getContext('2d').getImageData(start.x, start.y, 1, 1).data[3]) continue;
                            if (candidate.maskCanvas && candidate.maskEnabled && !candidate.maskCanvas.getContext('2d').getImageData(start.x, start.y, 1, 1).data[3]) continue;
                            index = i; break;
                        }
                    }
                    const layer = r.layers[index];
                    if (!layer?.sticker) return;
                    event.preventDefault(); event.stopPropagation(); cancel();
                    if (index !== r.activeLayerIdx) {
                        r.resetFilterPreview(); r.commitPendingPreviews(); r.activeLayerIdx = index;
                        if (r.editMaskMode) { r.editMaskMode = false; r.notify({ type: 'info', message: r.t('pixel.mask_edit_off') }); }
                        r.captureAdjustmentSource(); r.refreshLayerPanel(); refresh();
                    }
                    gesture = { layer, matrix: layer.sticker.matrix.slice(), start, handle: mode, pointer: event.pointerId };
                    frame.setPointerCapture(event.pointerId);
                }, { signal: abort.signal });
                frame.addEventListener('pointermove', event => {
                    if (!gesture || gesture.pointer !== event.pointerId) return;
                    const g = gesture, p = r.canvasCoords(event), c = centre(g.matrix);
                    let next;
                    if (g.handle === 'move') next = transformAt(g.matrix, 1, 0, p.x - g.start.x, p.y - g.start.y);
                    else if (g.handle === 'scale') next = transformAt(g.matrix, Math.max(.001, Math.hypot(p.x - c.x, p.y - c.y) / Math.max(1, Math.hypot(g.start.x - c.x, g.start.y - c.y))), 0);
                    else next = transformAt(g.matrix, 1, (Math.atan2(p.y - c.y, p.x - c.x) - Math.atan2(g.start.y - c.y, g.start.x - c.x)) * 180 / Math.PI);
                    changed(g.layer, next);
                }, { signal: abort.signal });
                frame.addEventListener('pointerup', event => {
                    if (!gesture || gesture.pointer !== event.pointerId) return;
                    const g = gesture; gesture = null;
                    if (frame.hasPointerCapture(event.pointerId)) frame.releasePointerCapture(event.pointerId);
                    if (g.matrix.some((value, i) => value !== g.layer.sticker.matrix[i])) r.pushHistory(tr('transform'));
                }, { signal: abort.signal });
                for (const name of ['pointercancel', 'lostpointercapture']) frame.addEventListener(name, cancel, { signal: abort.signal });
                window.addEventListener('blur', cancel, { signal: abort.signal });
                window.addEventListener('keydown', event => { if (event.key === 'Escape' && gesture) { event.preventDefault(); event.stopPropagation(); cancel(); } }, { capture: true, signal: abort.signal });
                window.addEventListener('resize', refresh, { signal: abort.signal });
                // Handles and fields follow the layers; refreshing on every composite rebuilt DOM on each mouse move.
                const syncControls = () => { const layer = active(); if ((layer && layer.sticker) || !controls.hidden) refresh(); };
                r.on('layers-changed', syncControls);
                r.on('active-layer-changed', refresh);
                r.on('document-changed', refresh);
            },
            disposeStickers() { r.resetStickerDocument(); abort.abort(); picker?.dispose(); frame?.remove(); controls?.remove(); }
        });
    };
})();
