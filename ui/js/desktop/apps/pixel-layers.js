// Pixel module "layers": the Layers dock (bottom panel "layers", replaces the built-in one) and the Layer window menu.
// Every operation lives in runtime.layerOps and goes through pixel-core (createLayer, markLayerDirty, commitLayerPixels,
// pushHistory); the dock, its context menu and the window menu only call those. DOM work happens in the panel mount.
(function () {
    'use strict';
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    const THUMB = { w: 44, h: 32 };
    const MASK_THUMB = { w: 32, h: 32 };
    const THUMB_INTERVAL = 100;
    const DRAG_THRESHOLD = 5;
    const SCROLL_EDGE = 28;
    const LOCKS = [
        { kind: 'pixels', prop: 'lockPixels', key: 'pixel.layer_lock_pixels', fb: 'Lock pixels' },
        { kind: 'alpha', prop: 'lockAlpha', key: 'pixel.layer_lock_alpha', fb: 'Lock transparency' },
        { kind: 'position', prop: 'lockPosition', key: 'pixel.layer_lock_position', fb: 'Lock position' }
    ];
    const BLEND_LABELS = {
        normal: 'Normal', multiply: 'Multiply', screen: 'Screen', overlay: 'Overlay', darken: 'Darken', lighten: 'Lighten',
        'color-dodge': 'Color dodge', 'color-burn': 'Color burn', 'hard-light': 'Hard light', 'soft-light': 'Soft light',
        difference: 'Difference', exclusion: 'Exclusion', hue: 'Hue', saturation: 'Saturation', color: 'Color', luminosity: 'Luminosity'
    };

    // --- pure helpers (also exported as Pixel.layerHelpers for the node tests) ---

    function clampNumber(v, lo, hi) {
        return v < lo ? lo : (v > hi ? hi : v);
    }

    function fmt(text, vars) {
        return String(text).replace(/\{(\w+)\}/g, (match, key) => (vars && vars[key] != null ? vars[key] : match));
    }

    // Largest rectangle with the aspect ratio of w x h that fits the box, centred in it.
    function fitRect(w, h, boxW, boxH) {
        if (!(w > 0) || !(h > 0)) return { x: 0, y: 0, w: boxW, h: boxH };
        const scale = Math.min(boxW / w, boxH / h);
        const dw = Math.max(1, Math.min(boxW, Math.round(w * scale)));
        const dh = Math.max(1, Math.min(boxH, Math.round(h * scale)));
        return { x: Math.floor((boxW - dw) / 2), y: Math.floor((boxH - dh) / 2), w: dw, h: dh };
    }

    // Drop position while dragging a row: number of other rows whose midpoint lies above the pointer.
    function dropSlot(mids, y) {
        let slot = 0;
        for (let i = 0; i < mids.length; i++) {
            if (mids[i] < y) slot++;
        }
        return slot;
    }

    // Bounding box of the pixels with alpha > 0 in RGBA data, or null for an empty layer.
    function contentBounds(data, w, h) {
        let minX = w, minY = h, maxX = -1, maxY = -1;
        for (let y = 0; y < h; y++) {
            const row = y * w * 4 + 3;
            let first = -1;
            for (let x = 0; x < w; x++) {
                if (data[row + x * 4]) { first = x; break; }
            }
            if (first < 0) continue;
            let last = first;
            for (let x = w - 1; x > first; x--) {
                if (data[row + x * 4]) { last = x; break; }
            }
            if (first < minX) minX = first;
            if (last > maxX) maxX = last;
            if (y < minY) minY = y;
            maxY = y;
        }
        return maxX < 0 ? null : { x: minX, y: minY, w: maxX - minX + 1, h: maxY - minY + 1 };
    }

    // Offset that moves the box b to the given alignment inside target ({ x, y, w, h }).
    function alignDelta(mode, b, target) {
        switch (mode) {
            case 'left': return { dx: target.x - b.x, dy: 0 };
            case 'hcenter': return { dx: Math.round(target.x + target.w / 2 - (b.x + b.w / 2)), dy: 0 };
            case 'right': return { dx: target.x + target.w - (b.x + b.w), dy: 0 };
            case 'top': return { dx: 0, dy: target.y - b.y };
            case 'vcenter': return { dx: 0, dy: Math.round(target.y + target.h / 2 - (b.y + b.h / 2)) };
            case 'bottom': return { dx: 0, dy: target.y + target.h - (b.y + b.h) };
            default: return { dx: 0, dy: 0 };
        }
    }

    // "Layer 2", "Layer 3", ...: the first number from `start` that no existing name uses.
    function uniqueName(base, taken, start) {
        let n = start || taken.length + 1;
        let name = base + ' ' + n;
        while (taken.includes(name)) name = base + ' ' + (++n);
        return name;
    }

    function fileSlug(text) {
        return String(text == null ? '' : text).replace(/[\\/:*?"<>|]+/g, '_').replace(/\s+/g, ' ').trim().slice(0, 60) || 'layer';
    }

    function blendLabelKey(mode) {
        return 'pixel.blend_' + String(mode).replace(/-/g, '_');
    }

    function blendFallback(mode) {
        return BLEND_LABELS[mode] || (String(mode).charAt(0).toUpperCase() + String(mode).slice(1));
    }

    Pixel.layerHelpers = { fitRect, dropSlot, contentBounds, alignDelta, uniqueName, fileSlug, fmt };

    // Translator with an English fallback that also works when the shell returns the key for unknown strings.
    function makeTranslator(r) {
        return function tr(key, fallback, vars) {
            let text = fallback;
            try {
                const out = r.t(key, fallback);
                if (out && out !== key) text = out;
            } catch (_) { /* keep the English text */ }
            return vars ? fmt(text, vars) : text;
        };
    }

    // --- icons (14px, currentColor) ---

    const svg = (inner, size) => `<svg width="${size || 14}" height="${size || 14}" viewBox="0 0 16 16" aria-hidden="true" focusable="false">${inner}</svg>`;
    const stroke = 'fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"';
    const ICONS = {
        eye: svg(`<path d="M1.5 8s2.4-4.5 6.5-4.5S14.5 8 14.5 8s-2.4 4.5-6.5 4.5S1.5 8 1.5 8z" ${stroke}/><circle cx="8" cy="8" r="2" fill="currentColor"/>`),
        eyeOff: svg(`<path d="M1.5 8s2.4-4.5 6.5-4.5S14.5 8 14.5 8s-2.4 4.5-6.5 4.5S1.5 8 1.5 8z" ${stroke} opacity="0.45"/><path d="M3 13L13 3" ${stroke}/>`),
        pixels: svg(`<path d="M11.5 2.5l2 2-7.5 7.5-3 1 1-3z" ${stroke}/>`),
        alpha: svg('<rect x="2" y="2" width="12" height="12" rx="1.5" fill="none" stroke="currentColor" stroke-width="1.3"/><rect x="2.7" y="2.7" width="5.3" height="5.3" fill="currentColor"/><rect x="8" y="8" width="5.3" height="5.3" fill="currentColor"/>'),
        position: svg(`<path d="M8 2v12M2 8h12M8 2L6 4M8 2l2 2M8 14l-2-2M8 14l2-2M2 8l2-2M2 8l2 2M14 8l-2-2M14 8l-2 2" ${stroke}/>`),
        clip: svg(`<path d="M3 3v6a2 2 0 002 2h7M9.5 8.5L12.5 11l-3 2.5" ${stroke}/>`),
        lock: svg('<rect x="3.5" y="7" width="9" height="6.5" rx="1.4" fill="currentColor"/><path d="M5.5 7V5.5a2.5 2.5 0 015 0V7" fill="none" stroke="currentColor" stroke-width="1.4"/>', 11),
        sticker: svg('<path d="M8 1.8l1.8 3.8 4.2.5-3.1 2.9.8 4.1L8 11.1 4.3 13.1l.8-4.1L2 6.1l4.2-.5z" fill="currentColor"/>', 11),
        plus: svg(`<rect x="2.5" y="2.5" width="11" height="11" rx="2" ${stroke}/><path d="M8 5.2v5.6M5.2 8h5.6" ${stroke}/>`),
        copy: svg(`<rect x="2.5" y="2.5" width="11" height="11" rx="2" ${stroke}/><rect x="5.2" y="5.2" width="5.6" height="5.6" rx="1" fill="none" stroke="currentColor" stroke-width="1.2" stroke-dasharray="1.6 1.4"/>`),
        duplicate: svg(`<rect x="2.5" y="5.5" width="8" height="8" rx="1.6" ${stroke}/><path d="M5.5 5.5v-1a2 2 0 012-2h4a2 2 0 012 2v4a2 2 0 01-2 2h-1" ${stroke}/>`),
        mask: svg('<rect x="2.5" y="2.5" width="11" height="11" rx="2" fill="none" stroke="currentColor" stroke-width="1.4"/><circle cx="8" cy="8" r="3" fill="currentColor"/>'),
        trash: svg(`<path d="M3 4.5h10M6.2 4.5V3h3.6v1.5M4.2 4.5l.6 8.5h6.4l.6-8.5M6.8 7v4M9.2 7v4" ${stroke}/>`),
        merge: svg(`<path d="M8 2v7M5 6.5L8 9.5l3-3M3 13h10" ${stroke}/>`),
        more: svg('<circle cx="3.5" cy="8" r="1.4" fill="currentColor"/><circle cx="8" cy="8" r="1.4" fill="currentColor"/><circle cx="12.5" cy="8" r="1.4" fill="currentColor"/>')
    };

    // --- layer operations ---

    function createOps(r) {
        const tr = makeTranslator(r);
        const say = (type, message) => { try { r.notify({ type, message }); } catch (_) { /* notifications are optional */ } };
        const hook = (name, ...args) => (typeof r[name] === 'function' ? r[name](...args) : undefined);
        const hasDoc = () => !!(r.imgWidth && r.imgHeight && r.layers.length);
        const pick = id => (id == null ? r.activeLayer() : r.layerById(id));
        const docSize = () => ({ w: r.imgWidth, h: r.imgHeight });
        const lockedNotice = () => say('info', tr('pixel.layer_locked', 'This layer is locked.'));
        const placeTitle = () => tr('pixel.layer_place_image', 'Place image as layer…').replace(/…$/, '');

        function canAdd() {
            if (!hasDoc()) return false;
            if (r.layers.length >= r.MAX_LAYERS) {
                say('error', tr('pixel.layer_max', 'Maximum number of layers reached ({n}).', { n: r.MAX_LAYERS }));
                return false;
            }
            return true;
        }

        // Structural change of the layer stack: repaint, refresh the dock and record one history entry.
        function finish(label, keepMaskEdit) {
            if (!keepMaskEdit) r.editMaskMode = false;
            r.layersStructureChanged();
            r.pushHistory(label);
        }

        function insertAt(layer, index) {
            r.layers.splice(index, 0, layer);
            r.activeLayerIdx = index;
        }

        function leaveMask(announce) {
            r.editMaskMode = false;
            if (announce) say('info', tr('pixel.mask_edit_off', 'Layer mask editing off'));
            r.refreshLayerPanel();
            hook('captureAdjustmentSource');
        }

        // --- selecting ---

        function select(id) {
            const index = r.layers.findIndex(l => l.id === id);
            if (index < 0) return false;
            if (index === r.activeLayerIdx) {
                if (r.editMaskMode) leaveMask(true);
                return true;
            }
            hook('resetFilterPreview');
            hook('commitPendingPreviews');
            hook('cancelStickerGesture');
            r.activeLayerIdx = index;
            if (r.layers[index].sticker && r.activeTool !== 'move') r.setActiveTool('move');
            if (r.editMaskMode) leaveMask(true);
            hook('captureAdjustmentSource');
            r.refreshLayerPanel();
            return true;
        }

        function selectRelative(delta) {
            const target = r.layers[r.activeLayerIdx + delta];
            return target ? select(target.id) : false;
        }

        // --- creating, copying, deleting ---

        function newLayer(opts) {
            const o = opts || {};
            if (!canAdd()) return null;
            hook('commitPendingPreviews');
            const name = o.name || uniqueName(tr('pixel.layer', 'Layer'), r.layers.map(l => l.name), r.layers.length + 1);
            const layer = r.createLayer({ name });
            if (o.fill) {
                const { w, h } = docSize();
                const lctx = layer.canvas.getContext('2d');
                lctx.fillStyle = o.fill;
                lctx.fillRect(0, 0, w, h);
                r.markLayerDirty(layer);
            }
            insertAt(layer, r.activeLayerIdx + 1);
            finish(o.label || tr('pixel.new_layer', 'Add Layer'));
            return layer;
        }

        function newFillLayer() {
            const color = r.primaryColor || '#000000';
            return newLayer({ fill: color, name: tr('pixel.layer_fill_name', 'Fill') + ' ' + color, label: tr('pixel.layer_new_fill', 'New fill layer') });
        }

        function duplicate(id) {
            const src = pick(id);
            if (!src || !canAdd()) return null;
            hook('commitPendingPreviews');
            hook('cancelStickerGesture');
            const copy = r.createLayer({
                name: tr('pixel.layer_copy_name', '{name} copy', { name: src.name }),
                visible: src.visible, opacity: src.opacity, blend: src.blend,
                lockPixels: src.lockPixels, lockAlpha: src.lockAlpha, lockPosition: src.lockPosition,
                maskEnabled: src.maskEnabled, clip: src.clip,
                maskCanvas: src.maskCanvas ? r.copyCanvas(src.maskCanvas) : null,
                source: src.sticker ? null : src.canvas,
                sticker: src.sticker ? { id: src.sticker.id, matrix: src.sticker.matrix.slice() } : null,
                stickerImage: src.stickerImage
            });
            insertAt(copy, r.layers.indexOf(src) + 1);
            if (copy.sticker) hook('repaintSticker', copy);
            finish(tr('pixel.duplicate_layer', 'Duplicate'));
            return copy;
        }

        // Clears everything outside the selection box and scales the alpha inside it by the selection mask.
        function restrictToSelection(ctx, sel, docW, docH, rectOnly) {
            if (sel.y > 0) ctx.clearRect(0, 0, docW, sel.y);
            if (sel.y + sel.h < docH) ctx.clearRect(0, sel.y + sel.h, docW, docH - sel.y - sel.h);
            if (sel.x > 0) ctx.clearRect(0, sel.y, sel.x, sel.h);
            if (sel.x + sel.w < docW) ctx.clearRect(sel.x + sel.w, sel.y, docW - sel.x - sel.w, sel.h);
            if (rectOnly) return;
            const image = ctx.getImageData(sel.x, sel.y, sel.w, sel.h);
            const d = image.data;
            for (let yy = 0; yy < sel.h; yy++) {
                let p = (sel.y + yy) * docW + sel.x, q = yy * sel.w * 4 + 3;
                for (let xx = 0; xx < sel.w; xx++, p++, q += 4) {
                    const a = sel.alpha[p];
                    if (a !== 255) d[q] = d[q] * a / 255;
                }
            }
            ctx.putImageData(image, sel.x, sel.y);
        }

        // Layer via copy (Ctrl+J): the selection, or the whole layer without one, as a new layer above. cut = remove it from the source.
        function viaCopy(opts) {
            const cut = !!(opts && opts.cut);
            const src = r.activeLayer();
            if (!src || !canAdd()) return null;
            if (src.sticker) return cut ? null : duplicate(src.id);
            let sel = null;
            if (r.selection) {
                sel = r.getSelectionMask();
                if (!sel) return null;
            } else if (cut) {
                say('info', tr('pixel.layer_need_selection', 'Make a selection first.'));
                return null;
            }
            r.editMaskMode = false;
            if (cut && !r.canPaintActiveLayer()) return null;
            hook('commitPendingPreviews');
            const { w, h } = docSize();
            const out = r.makeCanvas(w, h);
            const octx = out.getContext('2d');
            octx.drawImage(src.canvas, 0, 0);
            if (src.maskCanvas && src.maskEnabled) {
                octx.globalCompositeOperation = 'destination-in';
                octx.drawImage(src.maskCanvas, 0, 0);
                octx.globalCompositeOperation = 'source-over';
            }
            if (sel) restrictToSelection(octx, sel, w, h, r.selection.type === 'rect');
            const label = cut ? tr('pixel.layer_via_cut', 'Layer via cut') : tr('pixel.layer_via_copy', 'Layer via copy');
            const copy = r.createLayer({
                name: tr('pixel.layer_copy_name', '{name} copy', { name: src.name }),
                canvas: out, opacity: src.opacity, blend: src.blend
            });
            if (cut) r.eraseSelection();
            insertAt(copy, r.layers.indexOf(src) + 1);
            finish(label);
            return copy;
        }

        function remove(id) {
            const layer = pick(id);
            if (!layer) return false;
            if (r.layers.length <= 1) {
                say('info', tr('pixel.layer_last', 'The last layer cannot be deleted.'));
                return false;
            }
            const index = r.layers.indexOf(layer);
            const wasActive = index === r.activeLayerIdx;
            hook('cancelStickerGesture');
            if (wasActive) hook('discardPendingPreviews');
            else hook('commitPendingPreviews');
            const activeBefore = r.layers[r.activeLayerIdx];
            r.layers.splice(index, 1);
            let next;
            if (wasActive) next = Math.max(0, index - 1);
            else next = r.layers.indexOf(activeBefore);
            r.activeLayerIdx = clampNumber(next, 0, r.layers.length - 1);
            finish(tr('pixel.delete_layer', 'Delete Layer'), !wasActive && r.editMaskMode);
            return true;
        }

        // --- order ---

        function moveTo(id, to, label) {
            const from = r.layers.findIndex(l => l.id === id);
            const target = clampNumber(Math.round(to), 0, r.layers.length - 1);
            if (from < 0 || from === target) return false;
            const active = r.activeLayer();
            const [layer] = r.layers.splice(from, 1);
            r.layers.splice(target, 0, layer);
            r.activeLayerIdx = r.layers.indexOf(active);
            finish(label || tr('pixel.layer_move', 'Move layer'), true);
            return true;
        }

        function moveBy(delta, id) {
            const layer = pick(id);
            return layer ? moveTo(layer.id, r.layers.indexOf(layer) + delta) : false;
        }

        // --- properties ---

        const PROPS = ['name', 'visible', 'opacity', 'blend', 'lockPixels', 'lockAlpha', 'lockPosition', 'clip', 'maskEnabled'];

        function setProps(id, props, label, options) {
            const layer = pick(id);
            if (!layer) return false;
            const next = {};
            PROPS.forEach(key => {
                if (!props || !(key in props)) return;
                let value = props[key];
                if (key === 'opacity') {
                    value = Number(value);
                    if (!Number.isFinite(value)) return;
                    value = clampNumber(value, 0, 1);
                } else if (key === 'blend') {
                    if (!r.BLEND_MODES.includes(value)) return;
                } else if (key === 'name') {
                    value = String(value).trim();
                    if (!value) return;
                } else {
                    value = !!value;
                }
                if (layer[key] !== value) next[key] = value;
            });
            if (!Object.keys(next).length) return false;
            Object.assign(layer, next);
            r.compositeLayers();
            r.notifyLayersChanged();
            if (!options || options.history !== false) r.pushHistory(label || tr('pixel.layer_properties', 'Layer properties'));
            return true;
        }

        // Live opacity while a slider moves: no history entry (the dock records one when the slider is released).
        function setOpacity(id, value) {
            return setProps(id, { opacity: value }, '', { history: false });
        }

        function rename(id, name) {
            return setProps(id, { name }, tr('pixel.layer_rename', 'Rename layer'));
        }

        function toggleVisible(id) {
            const layer = pick(id);
            if (!layer) return false;
            return setProps(layer.id, { visible: !layer.visible }, layer.visible ? tr('pixel.layer_hist_hide', 'Hide layer') : tr('pixel.layer_hist_show', 'Show layer'));
        }

        // Alt+click on the eye: show only this layer; again: show everything.
        function solo(id) {
            const layer = pick(id);
            if (!layer) return false;
            const alone = layer.visible && r.layers.every(l => l === layer || !l.visible);
            r.layers.forEach(l => { l.visible = alone ? true : l === layer; });
            r.compositeLayers();
            r.notifyLayersChanged();
            r.pushHistory(alone ? tr('pixel.layer_hist_show_all', 'Show all layers') : tr('pixel.layer_hist_solo', 'Show only this layer'));
            return true;
        }

        function toggleLock(id, kind) {
            const layer = pick(id);
            const lock = LOCKS.find(entry => entry.kind === kind);
            if (!layer || !lock) return false;
            return setProps(layer.id, { [lock.prop]: !layer[lock.prop] }, tr(lock.key, lock.fb));
        }

        function toggleClip(id) {
            const layer = pick(id);
            if (!layer) return false;
            if (!layer.clip && r.layers.indexOf(layer) === 0) return false;
            return setProps(layer.id, { clip: !layer.clip }, tr('pixel.layer_clip_short', 'Clipping mask'));
        }

        // --- merging ---

        function mergeDown() {
            const index = r.activeLayerIdx;
            if (index <= 0 || r.layers.length < 2) {
                say('info', tr('pixel.layer_nothing_below', 'There is no layer below.'));
                return false;
            }
            const upper = r.layers[index];
            const lower = r.layers[index - 1];
            if (!upper.visible || !lower.visible) {
                say('info', tr('pixel.layer_merge_hidden', 'Show both layers to merge them.'));
                return false;
            }
            if (lower.lockPixels) { lockedNotice(); return false; }
            hook('commitPendingPreviews');
            hook('cancelStickerGesture');
            const { w, h } = docSize();
            // The lower layer's own blend and clip stay on the result; its mask and opacity are baked in.
            const base = Object.assign({}, lower, { blend: 'normal', clip: false, visible: true });
            const merged = r.makeCanvas(w, h);
            r.renderComposition(merged.getContext('2d'), [base, upper], { previews: false });
            Object.assign(lower, { canvas: merged, opacity: 1, visible: true, maskCanvas: null, maskEnabled: true, preview: null });
            delete lower.sticker;
            delete lower.stickerImage;
            lower.rev = r.nextRev();
            lower.maskRev = r.nextRev();
            r.layers.splice(index, 1);
            r.activeLayerIdx = index - 1;
            finish(tr('pixel.merge_layers', 'Merge Down'));
            return true;
        }

        function mergeVisible() {
            const visible = r.layers.filter(l => l.visible);
            if (visible.length < 2) {
                say('info', tr('pixel.layer_need_two', 'At least two visible layers are needed.'));
                return false;
            }
            hook('commitPendingPreviews');
            hook('cancelStickerGesture');
            const flat = r.flattenDocument();
            const merged = r.createLayer({ name: tr('pixel.layer_merged', 'Merged'), canvas: flat });
            const next = [];
            let placed = false;
            r.layers.forEach(layer => {
                if (!layer.visible) { next.push(layer); return; }
                if (!placed) { next.push(merged); placed = true; }
            });
            // In place: code that captured the array (async sticker loads) keeps pointing at the live layer stack.
            r.layers.splice(0, r.layers.length, ...next);
            r.activeLayerIdx = next.indexOf(merged);
            finish(tr('pixel.layer_merge_visible', 'Merge visible'));
            return true;
        }

        function flatten() {
            const only = r.layers.length === 1 ? r.layers[0] : null;
            const plain = only && !only.sticker && !only.maskCanvas && only.opacity === 1 && only.blend === 'normal' && only.visible;
            if (!r.layers.length || plain) return false;
            hook('commitPendingPreviews');
            hook('cancelStickerGesture');
            const flat = r.createLayer({ name: tr('pixel.layer_background', 'Background'), canvas: r.flattenDocument() });
            r.layers.splice(0, r.layers.length, flat);
            r.activeLayerIdx = 0;
            finish(tr('pixel.flatten', 'Flatten'));
            return true;
        }

        function rasterize(id) {
            const layer = pick(id);
            if (!layer || !layer.sticker) return false;
            select(layer.id);
            r.rasterizeSticker();
            return true;
        }

        // --- masks (the alpha channel of maskCanvas is the visibility) ---

        // mode: reveal | hide | selection | selection-hide
        function addMask(opts) {
            const mode = (opts && opts.mode) || 'reveal';
            const layer = r.activeLayer();
            if (!layer || !hasDoc()) return false;
            if (layer.maskCanvas) {
                say('info', tr('pixel.layer_has_mask', 'This layer already has a mask.'));
                return false;
            }
            const { w, h } = docSize();
            const wantsSelection = mode === 'selection' || mode === 'selection-hide';
            const sel = wantsSelection ? r.getSelectionMask() : null;
            if (wantsSelection && !sel) {
                say('info', tr('pixel.layer_need_selection', 'Make a selection first.'));
                return false;
            }
            hook('commitPendingPreviews');
            const mask = r.makeCanvas(w, h);
            const mctx = mask.getContext('2d');
            if (mode === 'reveal' || mode === 'selection-hide') {
                mctx.fillStyle = '#ffffff';
                mctx.fillRect(0, 0, w, h);
            }
            if (sel) {
                const part = r.makeCanvas(sel.w, sel.h);
                const pctx = part.getContext('2d');
                const image = pctx.createImageData(sel.w, sel.h);
                const d = image.data;
                for (let yy = 0; yy < sel.h; yy++) {
                    let p = (sel.y + yy) * w + sel.x, q = yy * sel.w * 4;
                    for (let xx = 0; xx < sel.w; xx++, p++, q += 4) {
                        d[q] = d[q + 1] = d[q + 2] = 255;
                        d[q + 3] = sel.alpha[p];
                    }
                }
                pctx.putImageData(image, 0, 0);
                if (mode === 'selection-hide') mctx.globalCompositeOperation = 'destination-out';
                mctx.drawImage(part, sel.x, sel.y);
                mctx.globalCompositeOperation = 'source-over';
            }
            layer.maskCanvas = mask;
            layer.maskEnabled = true;
            r.markLayerDirty(layer, { mask: true });
            r.editMaskMode = true;
            finish(tr('pixel.layer_add_mask', 'Add layer mask'), true);
            say('info', tr('pixel.mask_edit_on', 'Editing layer mask — paint white to reveal, eraser to hide.'));
            return true;
        }

        function requireMask() {
            const layer = r.activeLayer();
            if (!layer || !layer.maskCanvas) {
                say('info', tr('pixel.layer_no_mask', 'This layer has no mask.'));
                return null;
            }
            return layer;
        }

        function invertMask() {
            const layer = requireMask();
            if (!layer) return false;
            const { w, h } = docSize();
            const tmp = Pixel.acquireTempCanvas(w, h);
            const tctx = tmp.getContext('2d');
            tctx.fillStyle = '#ffffff';
            tctx.fillRect(0, 0, w, h);
            tctx.globalCompositeOperation = 'destination-out';
            tctx.drawImage(layer.maskCanvas, 0, 0);
            const mctx = layer.maskCanvas.getContext('2d');
            mctx.globalCompositeOperation = 'source-over';
            mctx.clearRect(0, 0, w, h);
            mctx.drawImage(tmp, 0, 0);
            Pixel.releaseTempCanvas(tmp);
            r.markLayerDirty(layer, { mask: true });
            r.compositeLayers();
            r.pushHistory(tr('pixel.layer_mask_invert', 'Invert mask'));
            return true;
        }

        function applyMask() {
            const layer = requireMask();
            if (!layer) return false;
            if (layer.lockPixels || layer.lockAlpha) { lockedNotice(); return false; }
            r.editMaskMode = false;
            if (!r.requireRasterLayer(layer)) return false;
            hook('commitPendingPreviews');
            const lctx = layer.canvas.getContext('2d');
            lctx.save();
            lctx.globalCompositeOperation = 'destination-in';
            lctx.drawImage(layer.maskCanvas, 0, 0);
            lctx.restore();
            layer.maskCanvas = null;
            layer.maskEnabled = true;
            r.markLayerDirty(layer);
            r.markLayerDirty(layer, { mask: true });
            finish(tr('pixel.layer_mask_apply', 'Apply mask'));
            return true;
        }

        function deleteMask() {
            const layer = requireMask();
            if (!layer) return false;
            layer.maskCanvas = null;
            layer.maskEnabled = true;
            r.markLayerDirty(layer, { mask: true });
            finish(tr('pixel.layer_mask_delete', 'Delete mask'));
            return true;
        }

        function toggleMask(id) {
            const layer = pick(id);
            if (!layer || !layer.maskCanvas) return false;
            return setProps(layer.id, { maskEnabled: !layer.maskEnabled }, layer.maskEnabled ? tr('pixel.layer_mask_disable', 'Disable mask') : tr('pixel.layer_mask_enable', 'Enable mask'));
        }

        // Mask thumbnail click: make the layer active and switch between painting its pixels and its mask.
        function editMask(id) {
            const layer = pick(id);
            if (!layer || !layer.maskCanvas) return false;
            if (r.layers.indexOf(layer) !== r.activeLayerIdx) select(layer.id);
            r.editMaskMode = !r.editMaskMode;
            say('info', r.editMaskMode ? tr('pixel.mask_edit_on', 'Editing layer mask — paint white to reveal, eraser to hide.') : tr('pixel.mask_edit_off', 'Layer mask editing off'));
            hook('captureAdjustmentSource');
            r.refreshLayerPanel();
            return true;
        }

        // --- selections from layers and masks ---

        function alphaOf(canvas) {
            const { w, h } = docSize();
            const data = canvas.getContext('2d').getImageData(0, 0, w, h).data;
            const alpha = new Uint8ClampedArray(w * h);
            for (let i = 0, q = 3; i < alpha.length; i++, q += 4) alpha[i] = data[q];
            return alpha;
        }

        // mode: replace | add | subtract | intersect
        function selectionFromLayer(id, mode) {
            const layer = pick(id);
            if (!layer || !hasDoc()) return false;
            const alpha = alphaOf(layer.canvas);
            if (layer.maskCanvas && layer.maskEnabled) {
                const mask = alphaOf(layer.maskCanvas);
                for (let i = 0; i < alpha.length; i++) alpha[i] = alpha[i] * mask[i] / 255;
            }
            const done = r.setSelectionMask(alpha, { mode: mode || 'replace' });
            if (!done && (!mode || mode === 'replace')) say('info', tr('pixel.layer_empty', 'The layer is empty.'));
            return done;
        }

        function selectionFromMask(id, mode) {
            const layer = pick(id);
            if (!layer || !layer.maskCanvas || !hasDoc()) return false;
            const done = r.setSelectionMask(alphaOf(layer.maskCanvas), { mode: mode || 'replace' });
            if (!done && (!mode || mode === 'replace')) say('info', tr('pixel.layer_mask_empty', 'The mask hides everything.'));
            return done;
        }

        // --- pixels ---

        // kind: primary | secondary | clear. Selection, locks and alpha lock apply (commitLayerPixels); in mask-edit mode the
        // primary colour reveals and the secondary colour or clear hides.
        function fill(kind) {
            const layer = r.activeLayer();
            if (!layer || !hasDoc()) return false;
            const { w, h } = docSize();
            const inMask = !!(r.editMaskMode && layer.maskCanvas);
            const src = Pixel.acquireTempCanvas(w, h);
            const sctx = src.getContext('2d');
            let label;
            if (kind === 'clear') {
                label = tr('pixel.layer_clear', 'Clear layer');
            } else {
                const primary = kind === 'primary';
                label = primary ? tr('pixel.layer_fill_primary', 'Fill with primary color') : tr('pixel.layer_fill_secondary', 'Fill with secondary color');
                if (!inMask || primary) {
                    sctx.fillStyle = inMask ? '#ffffff' : (primary ? r.primaryColor : r.secondaryColor);
                    sctx.fillRect(0, 0, w, h);
                }
            }
            const done = r.commitLayerPixels(layer, src, { label });
            Pixel.releaseTempCanvas(src);
            return done;
        }

        // Moves the layer content so that its bounding box aligns with the selection, or with the canvas without one.
        function align(mode) {
            const layer = r.activeLayer();
            if (!layer || !hasDoc()) return false;
            if (layer.lockPosition) { lockedNotice(); return false; }
            hook('commitPendingPreviews');
            const { w, h } = docSize();
            const bounds = contentBounds(layer.canvas.getContext('2d').getImageData(0, 0, w, h).data, w, h);
            if (!bounds) {
                say('info', tr('pixel.layer_empty', 'The layer is empty.'));
                return false;
            }
            const sel = r.selection ? r.getSelectionMask() : null;
            const target = sel ? { x: sel.x, y: sel.y, w: sel.w, h: sel.h } : { x: 0, y: 0, w, h };
            const delta = alignDelta(mode, bounds, target);
            if (!delta.dx && !delta.dy) return true;
            if (layer.sticker) {
                hook('cancelStickerGesture');
                layer.sticker.matrix[4] += delta.dx;
                layer.sticker.matrix[5] += delta.dy;
                hook('repaintSticker', layer);
            } else {
                const snap = Pixel.acquireTempCanvas(w, h);
                snap.getContext('2d').drawImage(layer.canvas, 0, 0);
                const lctx = layer.canvas.getContext('2d');
                lctx.clearRect(0, 0, w, h);
                lctx.drawImage(snap, delta.dx, delta.dy);
                Pixel.releaseTempCanvas(snap);
                r.markLayerDirty(layer);
            }
            r.compositeLayers();
            r.pushHistory(tr('pixel.layer_align', 'Align layer'));
            return true;
        }

        // --- files ---

        // The image becomes a new layer: centred, scaled down (never up) to fit the canvas.
        function placeImageElement(img, name) {
            if (!canAdd()) return null;
            const { w, h } = docSize();
            const sw = img.naturalWidth || img.width || 0;
            const sh = img.naturalHeight || img.height || 0;
            if (!sw || !sh) return null;
            hook('commitPendingPreviews');
            const scale = Math.min(1, w / sw, h / sh);
            const dw = Math.max(1, Math.round(sw * scale));
            const dh = Math.max(1, Math.round(sh * scale));
            const layer = r.createLayer({ name: name || tr('pixel.layer', 'Layer') });
            const lctx = layer.canvas.getContext('2d');
            lctx.imageSmoothingEnabled = true;
            lctx.imageSmoothingQuality = 'high';
            lctx.drawImage(img, Math.round((w - dw) / 2), Math.round((h - dh) / 2), dw, dh);
            r.markLayerDirty(layer);
            insertAt(layer, r.activeLayerIdx + 1);
            finish(placeTitle());
            return layer;
        }

        async function placeImagePath(path) {
            const revision = r.state.documentRevision;
            let url = '';
            try {
                const response = await fetch('/api/desktop/preview?path=' + encodeURIComponent(path), { credentials: 'same-origin', cache: 'no-store' });
                if (!response.ok) throw new Error('image');
                url = URL.createObjectURL(await response.blob());
                const img = await r.loadImage(url);
                if (r.state.disposed || revision !== r.state.documentRevision) return null;
                return placeImageElement(img, String(path).split('/').pop().replace(/\.[^.]+$/, ''));
            } catch (_) {
                if (!r.state.disposed) say('error', tr('pixel.layer_place_failed', 'The image could not be placed.'));
                return null;
            } finally {
                if (url) URL.revokeObjectURL(url);
            }
        }

        async function placeImage() {
            if (!hasDoc() || !r.ctx.openFileDialog || !canAdd()) return null;
            const result = await r.ctx.openFileDialog({
                title: placeTitle(), initialPath: 'Photos',
                filters: [{ label: r.t('desktop.file_dialog_images'), extensions: r.IMAGE_EXTS.filter(ext => ext !== 'ora') }]
            });
            if (!result || result.canceled || !result.path) return null;
            return placeImagePath(result.path);
        }

        // PNG download of the layer pixels (mask applied), document-sized so exported layers keep their position.
        function exportLayer(id) {
            const layer = pick(id);
            if (!layer || !hasDoc()) return false;
            const { w, h } = docSize();
            const out = r.makeCanvas(w, h);
            const octx = out.getContext('2d');
            octx.drawImage(layer.canvas, 0, 0);
            if (layer.maskCanvas && layer.maskEnabled) {
                octx.globalCompositeOperation = 'destination-in';
                octx.drawImage(layer.maskCanvas, 0, 0);
            }
            const link = document.createElement('a');
            link.download = fileSlug((r.fileName || 'image').replace(/\.[^.]+$/, '')) + '-' + fileSlug(layer.name) + '.png';
            const download = href => { link.href = href; link.click(); };
            out.toBlob(blob => {
                if (!blob) { download(out.toDataURL('image/png')); return; }
                const url = URL.createObjectURL(blob);
                download(url);
                setTimeout(() => URL.revokeObjectURL(url), 4000);
            }, 'image/png');
            return true;
        }

        // --- layer properties dialog ---

        function properties(id) {
            const layer = pick(id);
            if (!layer || !hasDoc()) return false;
            const esc = text => r.esc(text);
            const dlg = document.createElement('div');
            dlg.className = 'vd-modal-backdrop';
            const pct = Math.round(layer.opacity * 100);
            const options = r.BLEND_MODES.map(mode => `<option value="${esc(mode)}"${mode === layer.blend ? ' selected' : ''}>${esc(tr(blendLabelKey(mode), blendFallback(mode)))}</option>`).join('');
            const check = (prop, label) => `<label class="pixel-layer-dialog-check"><input type="checkbox" data-prop="${prop}"${layer[prop] ? ' checked' : ''}> ${esc(label)}</label>`;
            dlg.innerHTML = `<form class="vd-modal pixel-layer-dialog" role="dialog" aria-label="${esc(tr('pixel.layer_properties', 'Layer properties'))}">
                <div class="vd-modal-title">${esc(tr('pixel.layer_properties', 'Layer properties'))}</div>
                <div class="pixel-layer-dialog-grid">
                    <label class="pixel-layer-dialog-label">${esc(tr('pixel.layer_name', 'Name'))}</label>
                    <input class="vd-modal-input" type="text" maxlength="80" data-prop="name" value="${esc(layer.name)}">
                    <label class="pixel-layer-dialog-label">${esc(tr('pixel.layer_opacity', 'Opacity'))}</label>
                    <div class="pixel-layer-dialog-opacity"><input type="range" class="pixel-slider" min="0" max="100" data-prop="opacity" value="${pct}"><span data-prop-opacity-value>${pct}%</span></div>
                    <label class="pixel-layer-dialog-label">${esc(tr('pixel.layer_blend', 'Blend mode'))}</label>
                    <select class="pixel-select" data-prop="blend">${options}</select>
                </div>
                <div class="pixel-layer-dialog-checks">
                    ${LOCKS.map(lock => check(lock.prop, tr(lock.key, lock.fb))).join('')}
                    ${check('clip', tr('pixel.layer_clip_short', 'Clipping mask'))}
                </div>
                <div class="vd-modal-actions">
                    <button type="button" class="vd-button" data-cancel>${esc(tr('pixel.cancel', 'Cancel'))}</button>
                    <button type="submit" class="vd-button vd-button-primary">${esc(tr('pixel.apply', 'Apply'))}</button>
                </div>
            </form>`;
            document.body.appendChild(dlg);
            const field = prop => dlg.querySelector('[data-prop="' + prop + '"]');
            const slider = field('opacity');
            const shown = dlg.querySelector('[data-prop-opacity-value]');
            slider.addEventListener('input', () => { shown.textContent = slider.value + '%'; });
            dlg.querySelector('form').addEventListener('submit', e => {
                e.preventDefault();
                const props = { name: field('name').value, opacity: Number(slider.value) / 100, blend: field('blend').value };
                LOCKS.map(lock => lock.prop).concat('clip').forEach(prop => { props[prop] = field(prop).checked; });
                if (props.clip && r.layers.indexOf(layer) === 0) props.clip = false;
                dlg.remove();
                setProps(layer.id, props, tr('pixel.layer_properties', 'Layer properties'));
            });
            dlg.querySelector('[data-cancel]').addEventListener('click', () => dlg.remove());
            dlg.addEventListener('click', e => { if (e.target === dlg) dlg.remove(); });
            dlg.addEventListener('keydown', e => { if (e.key === 'Escape') { e.stopPropagation(); dlg.remove(); } });
            const nameField = field('name');
            if (nameField && nameField.focus) nameField.focus();
            return true;
        }

        return {
            select, selectRelative, newLayer, newFillLayer, duplicate, viaCopy, remove, moveTo, moveBy,
            toFront: id => { const layer = pick(id); return layer ? moveTo(layer.id, r.layers.length - 1, tr('pixel.layer_bring_front', 'Bring to front')) : false; },
            toBack: id => { const layer = pick(id); return layer ? moveTo(layer.id, 0, tr('pixel.layer_send_back', 'Send to back')) : false; },
            setProps, setOpacity, rename, toggleVisible, solo, toggleLock, toggleClip,
            mergeDown, mergeVisible, flatten, rasterize,
            addMask, invertMask, applyMask, deleteMask, toggleMask, editMask, selectionFromLayer, selectionFromMask,
            fill, align, placeImageElement, placeImagePath, placeImage, exportLayer, properties,
            leaveMask, hasDoc
        };
    }

    // --- Layer menu (window menu and the dock's context menu share this tree) ---

    function menuItems(r) {
        const L = r && r.layerOps;
        if (!L) return [];
        const item = (id, key, fallback, extra) => Object.assign({ id, labelKey: key, fallback, label: fallback }, extra);
        const active = () => r.activeLayer();
        const doc = () => !!(r.imgWidth && r.layers.length);
        const maskOn = () => { const l = active(); return !!(l && l.maskCanvas); };
        const noMask = () => { const l = active(); return !!(l && !l.maskCanvas); };
        const maskState = () => { const l = active(); return !!(l && l.maskCanvas && l.maskEnabled); };
        const sep = { type: 'separator' };
        const alignItem = (mode, key, fallback) => item('align-' + mode, key, fallback, { action: () => L.align(mode), enabled: doc });
        return [
            item('new-layer', 'pixel.new_layer', 'Add Layer', { icon: 'file-plus', shortcut: 'Ctrl+Shift+Alt+N', action: () => L.newLayer(), enabled: doc }),
            item('new-fill-layer', 'pixel.layer_new_fill', 'New fill layer', { action: () => L.newFillLayer(), enabled: doc }),
            item('via-copy', 'pixel.layer_via_copy', 'Layer via copy', { shortcut: 'Ctrl+J', keywords: ['copy', 'selection'], action: () => L.viaCopy(), enabled: doc }),
            item('via-cut', 'pixel.layer_via_cut', 'Layer via cut', { action: () => L.viaCopy({ cut: true }), enabled: () => doc() && !!r.selection }),
            item('duplicate', 'pixel.duplicate_layer', 'Duplicate', { action: () => L.duplicate(), enabled: doc }),
            item('delete', 'pixel.delete_layer', 'Delete Layer', { action: () => L.remove(), enabled: () => r.layers.length > 1 }),
            item('rename', 'pixel.layer_rename', 'Rename layer', { shortcut: 'F2', action: () => { if (r.layersPanel) r.layersPanel.startRename(); }, enabled: doc }),
            item('properties', 'pixel.layer_properties', 'Layer properties', { action: () => L.properties(), enabled: doc }),
            sep,
            item('merge-down', 'pixel.merge_layers', 'Merge Down', { shortcut: 'Ctrl+E', action: () => L.mergeDown(), enabled: () => r.activeLayerIdx > 0 }),
            item('merge-visible', 'pixel.layer_merge_visible', 'Merge visible', { shortcut: 'Ctrl+Shift+E', action: () => L.mergeVisible(), enabled: () => r.layers.filter(l => l.visible).length > 1 }),
            item('flatten', 'pixel.flatten', 'Flatten', { action: () => L.flatten(), enabled: doc }),
            item('rasterize', 'pixel.layer_rasterize', 'Rasterize sticker', { action: () => L.rasterize(), enabled: () => !!(active() && active().sticker) }),
            sep,
            item('mask', 'pixel.layer_mask', 'Layer mask', {
                items: [
                    item('mask-reveal', 'pixel.layer_mask_reveal', 'Add mask (reveal all)', { action: () => L.addMask({ mode: 'reveal' }), enabled: noMask }),
                    item('mask-hide', 'pixel.layer_mask_hide', 'Add mask (hide all)', { action: () => L.addMask({ mode: 'hide' }), enabled: noMask }),
                    item('mask-from-selection', 'pixel.layer_mask_from_selection', 'Add mask from selection', { action: () => L.addMask({ mode: 'selection' }), enabled: () => noMask() && !!r.selection }),
                    item('mask-hide-selection', 'pixel.layer_mask_hide_selection', 'Add mask hiding the selection', { action: () => L.addMask({ mode: 'selection-hide' }), enabled: () => noMask() && !!r.selection }),
                    sep,
                    item('mask-edit', 'pixel.layer_mask_edit', 'Edit mask', { action: () => L.editMask(), enabled: maskOn, checked: () => maskOn() && !!r.editMaskMode }),
                    item('mask-invert', 'pixel.layer_mask_invert', 'Invert mask', { action: () => L.invertMask(), enabled: maskOn }),
                    item('mask-apply', 'pixel.layer_mask_apply', 'Apply mask', { action: () => L.applyMask(), enabled: maskOn }),
                    item('mask-delete', 'pixel.layer_mask_delete', 'Delete mask', { action: () => L.deleteMask(), enabled: maskOn }),
                    item('mask-disable', 'pixel.layer_mask_disable', 'Disable mask', { action: () => L.toggleMask(), enabled: maskOn, hidden: () => maskOn() && !maskState() }),
                    item('mask-enable', 'pixel.layer_mask_enable', 'Enable mask', { action: () => L.toggleMask(), enabled: maskOn, hidden: () => !maskOn() || maskState() }),
                    item('mask-to-selection', 'pixel.layer_select_from_mask', 'Selection from mask', { action: () => L.selectionFromMask(), enabled: maskOn })
                ]
            }),
            item('select-from-layer', 'pixel.layer_select_from_layer', 'Selection from layer', { action: () => L.selectionFromLayer(), enabled: doc }),
            item('clip', 'pixel.layer_clip_short', 'Clipping mask', { shortcut: 'Ctrl+Alt+G', action: () => L.toggleClip(), enabled: () => r.activeLayerIdx > 0 || !!(active() && active().clip), checked: () => !!(active() && active().clip) }),
            sep,
            item('fill-primary', 'pixel.layer_fill_primary', 'Fill with primary color', { action: () => L.fill('primary'), enabled: doc }),
            item('fill-secondary', 'pixel.layer_fill_secondary', 'Fill with secondary color', { action: () => L.fill('secondary'), enabled: doc }),
            item('clear', 'pixel.layer_clear', 'Clear layer', { action: () => L.fill('clear'), enabled: doc }),
            sep,
            item('arrange', 'pixel.layer_arrange', 'Arrange', {
                items: [
                    item('bring-to-front', 'pixel.layer_bring_front', 'Bring to front', { shortcut: 'Ctrl+Shift+]', action: () => L.toFront(), enabled: () => r.activeLayerIdx < r.layers.length - 1 }),
                    item('bring-forward', 'pixel.layer_forward', 'Bring forward', { shortcut: 'Ctrl+]', shortcuts: ['Ctrl+Shift+ArrowUp'], action: () => L.moveBy(1), enabled: () => r.activeLayerIdx < r.layers.length - 1 }),
                    item('send-backward', 'pixel.layer_backward', 'Send backward', { shortcut: 'Ctrl+[', shortcuts: ['Ctrl+Shift+ArrowDown'], action: () => L.moveBy(-1), enabled: () => r.activeLayerIdx > 0 }),
                    item('send-to-back', 'pixel.layer_send_back', 'Send to back', { shortcut: 'Ctrl+Shift+[', action: () => L.toBack(), enabled: () => r.activeLayerIdx > 0 }),
                    sep,
                    item('select-above', 'pixel.layer_select_above', 'Select layer above', { shortcut: 'Alt+]', shortcuts: ['Alt+ArrowUp'], action: () => L.selectRelative(1), enabled: () => r.activeLayerIdx < r.layers.length - 1 }),
                    item('select-below', 'pixel.layer_select_below', 'Select layer below', { shortcut: 'Alt+[', shortcuts: ['Alt+ArrowDown'], action: () => L.selectRelative(-1), enabled: () => r.activeLayerIdx > 0 })
                ]
            }),
            item('align', 'pixel.layer_align', 'Align layer', {
                items: [
                    alignItem('left', 'pixel.layer_align_left', 'Align left'),
                    alignItem('hcenter', 'pixel.layer_align_hcenter', 'Align horizontal center'),
                    alignItem('right', 'pixel.layer_align_right', 'Align right'),
                    alignItem('top', 'pixel.layer_align_top', 'Align top'),
                    alignItem('vcenter', 'pixel.layer_align_vcenter', 'Align vertical center'),
                    alignItem('bottom', 'pixel.layer_align_bottom', 'Align bottom')
                ]
            }),
            sep,
            item('place-image', 'pixel.layer_place_image', 'Place image as layer…', { icon: 'folder-open', action: () => L.placeImage(), enabled: doc }),
            item('export-layer', 'pixel.layer_export', 'Export layer as PNG', { icon: 'download', action: () => L.exportLayer(), enabled: doc })
        ];
    }

    // --- the dock ---

    function buildPanelHTML(r) {
        const tr = makeTranslator(r);
        const esc = text => r.esc(text);
        const blendOptions = r.BLEND_MODES.map(mode => `<option value="${esc(mode)}">${esc(tr(blendLabelKey(mode), blendFallback(mode)))}</option>`).join('');
        const lockButtons = LOCKS.map(lock => {
            const label = esc(tr(lock.key, lock.fb));
            return `<button type="button" class="pixel-layers-toggle" data-layer-lock="${lock.kind}" title="${label}" aria-label="${label}" aria-pressed="false">${ICONS[lock.kind]}</button>`;
        }).join('');
        const clipLabel = esc(tr('pixel.layer_clip', 'Clipping mask (clip to the layer below)'));
        const footer = [
            ['new', 'plus', 'pixel.new_layer', 'Add Layer'],
            ['copy', 'copy', 'pixel.layer_via_copy', 'Layer via copy', 'Ctrl+J'],
            ['duplicate', 'duplicate', 'pixel.duplicate_layer', 'Duplicate'],
            ['mask', 'mask', 'pixel.layer_add_mask', 'Add layer mask'],
            ['delete', 'trash', 'pixel.delete_layer', 'Delete Layer'],
            ['merge', 'merge', 'pixel.merge_layers', 'Merge Down', 'Ctrl+E'],
            ['more', 'more', 'pixel.layer_more', 'More layer actions']
        ].map(([act, icon, key, fb, hint]) => {
            const label = esc(tr(key, fb) + (hint ? ' (' + hint + ')' : ''));
            return `<button type="button" class="pixel-layers-btn" data-layer-act="${act}" title="${label}" aria-label="${label}">${ICONS[icon]}</button>`;
        }).join('');
        return `<div class="pixel-layers-header" data-layers-header>
                <div class="pixel-layers-row">
                    <select class="pixel-select pixel-layers-blend" data-layer-blend title="${esc(tr('pixel.layer_blend', 'Blend mode'))}" aria-label="${esc(tr('pixel.layer_blend', 'Blend mode'))}">${blendOptions}</select>
                </div>
                <div class="pixel-layers-row">
                    <span class="pixel-layers-caption">${esc(tr('pixel.layer_opacity', 'Opacity'))}</span>
                    <input type="range" class="pixel-slider pixel-layers-opacity" data-layer-opacity-range min="0" max="100" step="1" value="100" aria-label="${esc(tr('pixel.layer_opacity', 'Opacity'))}">
                    <input type="number" class="pixel-layers-number" data-layer-opacity-number min="0" max="100" step="1" value="100" aria-label="${esc(tr('pixel.layer_opacity', 'Opacity'))}"><span class="pixel-layers-unit">%</span>
                </div>
                <div class="pixel-layers-row pixel-layers-locks">
                    ${lockButtons}
                    <span class="pixel-layers-gap"></span>
                    <button type="button" class="pixel-layers-toggle" data-layer-lock="clip" title="${clipLabel}" aria-label="${clipLabel}" aria-pressed="false">${ICONS.clip}</button>
                </div>
            </div>
            <div class="pixel-layer-list" data-layer-list tabindex="0" aria-label="${esc(tr('pixel.layers', 'Layers'))}">
                <div class="pixel-layer-rows" data-layer-rows role="listbox"></div>
                <div class="pixel-layer-drop" data-layer-drop hidden></div>
            </div>
            <div class="pixel-layer-actions pixel-layers-footer">${footer}</div>`;
    }

    function createPanel(r, section) {
        const ops = r.layerOps;
        const tr = makeTranslator(r);
        const esc = text => r.esc(text);
        const abort = new AbortController();
        const sig = { signal: abort.signal };
        const q = selector => section.querySelector(selector);
        const listEl = q('[data-layer-list]');
        const rowsEl = q('[data-layer-rows]');
        const dropEl = q('[data-layer-drop]');
        const blendSel = q('[data-layer-blend]');
        const rangeEl = q('[data-layer-opacity-range]');
        const numEl = q('[data-layer-opacity-number]');
        const lockBtns = Array.from(section.querySelectorAll('[data-layer-lock]'));
        const footerBtns = Array.from(section.querySelectorAll('[data-layer-act]'));
        const thumbs = new Map();
        const maskThumbs = new Map();
        let disposed = false, structKey = '', thumbTimer = 0, lastThumbs = 0, scrollRaf = 0;
        let renaming = null, drag = null, suppressClick = false, opacityEdit = null, menu = null, activeSeen = null, skipMore = false;

        const alive = () => !disposed && !r.state.disposed;
        const dpr = () => Math.min(2, Math.max(1, (typeof window.devicePixelRatio === 'number' && window.devicePixelRatio) || 1));
        const busy = () => !!(renaming || (drag && drag.active));
        const idOf = el => {
            const row = el && el.closest ? el.closest('[data-layer-id]') : null;
            return row ? Number(row.dataset.layerId) : null;
        };

        // --- rows ---

        function structureKey() {
            return JSON.stringify([r.imgWidth, r.imgHeight].concat(r.layers.map(l => [
                l.id, l.name, l.visible ? 1 : 0, l.blend, (l.lockPixels ? 1 : 0) + (l.lockAlpha ? 2 : 0) + (l.lockPosition ? 4 : 0),
                l.clip ? 1 : 0, l.maskCanvas ? (l.maskEnabled ? 2 : 1) : 0, l.sticker ? 1 : 0
            ])));
        }

        function badgesHTML(layer) {
            const out = [];
            const locks = LOCKS.filter(lock => layer[lock.prop]).map(lock => tr(lock.key, lock.fb));
            if (locks.length) out.push(`<span class="pixel-layer-badge" title="${esc(locks.join(', '))}">${ICONS.lock}</span>`);
            if (layer.clip) out.push(`<span class="pixel-layer-badge" title="${esc(tr('pixel.layer_clip', 'Clipping mask (clip to the layer below)'))}">${ICONS.clip}</span>`);
            if (layer.blend !== 'normal') {
                const name = tr(blendLabelKey(layer.blend), blendFallback(layer.blend));
                out.push(`<span class="pixel-layer-badge pixel-layer-badge-text" title="${esc(name)}">${esc(name)}</span>`);
            }
            if (layer.sticker) out.push(`<span class="pixel-layer-badge" title="${esc(tr('pixel.layer_badge_sticker', 'Sticker layer'))}">${ICONS.sticker}</span>`);
            return out.join('');
        }

        function rowHTML(layer, index, titles) {
            const ratio = dpr();
            const mask = layer.maskCanvas
                ? `<span class="pixel-layer-maskthumb${layer.maskEnabled ? '' : ' disabled'}" data-layer-mask-thumb title="${esc(titles.mask)}"><canvas class="pixel-layer-mask-canvas" data-layer-mask-canvas width="${Math.round(MASK_THUMB.w * ratio)}" height="${Math.round(MASK_THUMB.h * ratio)}"></canvas></span>`
                : '';
            const classes = 'pixel-layer-item' + (layer.visible ? '' : ' is-hidden') + (layer.clip ? ' pixel-layer-clipped' : '') + (layer.sticker ? ' is-sticker' : '');
            return `<div class="${classes}" role="option" aria-selected="false" data-layer-id="${layer.id}" data-layer-index="${index}">
                <button type="button" class="pixel-layer-vis${layer.visible ? '' : ' off'}" data-layer-eye title="${esc(titles.eye)}" aria-label="${esc(titles.eye)}" aria-pressed="${layer.visible ? 'true' : 'false'}">${layer.visible ? ICONS.eye : ICONS.eyeOff}</button>
                <span class="pixel-layer-thumb" data-layer-thumb-wrap title="${esc(titles.thumb)}"><canvas class="pixel-layer-thumb-canvas" data-layer-thumb width="${Math.round(THUMB.w * ratio)}" height="${Math.round(THUMB.h * ratio)}"></canvas></span>
                ${mask}
                <span class="pixel-layer-info"><span class="pixel-layer-name" data-layer-name title="${esc(titles.name)}">${esc(layer.name)}</span><span class="pixel-layer-badges">${badgesHTML(layer)}</span></span>
            </div>`;
        }

        // Rows, top layer first.
        function rowsHTML() {
            const titles = {
                eye: tr('pixel.layer_eye_hint', 'Show or hide (Alt+click: show only this layer)'),
                thumb: tr('pixel.layer_thumb_hint', 'Ctrl+click: selection from the layer (Shift: add, Alt: subtract)'),
                mask: tr('pixel.layer_mask_hint', 'Layer mask. Click: edit the mask. Shift+click: enable or disable it. Ctrl+click: selection from the mask'),
                name: tr('pixel.layer_rename_hint', 'Double-click to rename')
            };
            const out = [];
            for (let i = r.layers.length - 1; i >= 0; i--) out.push(rowHTML(r.layers[i], i, titles));
            return out.join('');
        }

        function rowEls() {
            return Array.from(rowsEl.querySelectorAll('[data-layer-id]'));
        }

        function updateActive() {
            const active = r.activeLayer();
            const activeId = active ? active.id : null;
            const maskEdit = !!(r.editMaskMode && active && active.maskCanvas);
            let activeRow = null;
            rowEls().forEach(row => {
                const on = Number(row.dataset.layerId) === activeId;
                row.classList.toggle('active', on);
                row.classList.toggle('editing-mask', on && maskEdit);
                row.setAttribute('aria-selected', on ? 'true' : 'false');
                if (on) activeRow = row;
            });
            if (activeId !== activeSeen) {
                activeSeen = activeId;
                if (activeRow && activeRow.scrollIntoView) activeRow.scrollIntoView({ block: 'nearest' });
            }
        }

        function rebuildRows() {
            rowsEl.innerHTML = rowsHTML();
            const ids = new Set(r.layers.map(l => l.id));
            [thumbs, maskThumbs].forEach(store => store.forEach((entry, id) => { if (!ids.has(id)) store.delete(id); }));
            updateActive();
            paintThumbs(true);
        }

        // --- thumbnails ---

        function paintChecker(ctx, f) {
            const size = 4;
            ctx.save();
            ctx.beginPath();
            ctx.rect(f.x, f.y, f.w, f.h);
            ctx.clip();
            ctx.fillStyle = '#ffffff';
            ctx.fillRect(f.x, f.y, f.w, f.h);
            ctx.fillStyle = '#c9c9c9';
            for (let y = 0; y * size < f.h; y++) {
                for (let x = y % 2; x * size < f.w; x += 2) ctx.fillRect(f.x + x * size, f.y + y * size, size, size);
            }
            ctx.restore();
        }

        function renderThumb(canvas, src, box, ratio, isMask) {
            const ctx = canvas.getContext('2d');
            ctx.setTransform(1, 0, 0, 1, 0, 0);
            ctx.clearRect(0, 0, canvas.width, canvas.height);
            const f = fitRect(r.imgWidth, r.imgHeight, box.w, box.h);
            const usable = !!(src && src.width && src.height);
            if (isMask) {
                // The mask alpha is the visibility: shown as white (visible) on black (hidden).
                ctx.scale(ratio, ratio);
                ctx.fillStyle = '#000000';
                ctx.fillRect(f.x, f.y, f.w, f.h);
                if (usable) {
                    const tmp = Pixel.acquireTempCanvas(canvas.width, canvas.height);
                    const tctx = tmp.getContext('2d');
                    tctx.scale(ratio, ratio);
                    tctx.fillStyle = '#ffffff';
                    tctx.fillRect(f.x, f.y, f.w, f.h);
                    tctx.globalCompositeOperation = 'destination-in';
                    tctx.imageSmoothingEnabled = true;
                    tctx.imageSmoothingQuality = 'high';
                    tctx.drawImage(src, 0, 0, src.width, src.height, f.x, f.y, f.w, f.h);
                    ctx.setTransform(1, 0, 0, 1, 0, 0);
                    ctx.drawImage(tmp, 0, 0);
                    Pixel.releaseTempCanvas(tmp);
                }
            } else {
                ctx.scale(ratio, ratio);
                paintChecker(ctx, f);
                if (usable) {
                    ctx.imageSmoothingEnabled = true;
                    ctx.imageSmoothingQuality = 'high';
                    ctx.drawImage(src, 0, 0, src.width, src.height, f.x, f.y, f.w, f.h);
                }
            }
            ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
            ctx.strokeStyle = 'rgba(0, 0, 0, 0.5)';
            ctx.lineWidth = 1;
            ctx.strokeRect(f.x + 0.5, f.y + 0.5, Math.max(0, f.w - 1), Math.max(0, f.h - 1));
        }

        // Cached small canvas per layer; redrawn only when the rev of the pixels (or the mask) changed.
        function thumbEntry(layer, isMask) {
            const store = isMask ? maskThumbs : thumbs;
            const box = isMask ? MASK_THUMB : THUMB;
            const ratio = dpr();
            const cw = Math.round(box.w * ratio), ch = Math.round(box.h * ratio);
            const key = (isMask ? layer.maskRev : layer.rev) + ':' + r.imgWidth + 'x' + r.imgHeight + ':' + ratio;
            let entry = store.get(layer.id);
            if (!entry) {
                entry = { canvas: r.makeCanvas(cw, ch), key: '' };
                store.set(layer.id, entry);
            }
            if (entry.canvas.width !== cw || entry.canvas.height !== ch) {
                entry.canvas.width = cw;
                entry.canvas.height = ch;
                entry.key = '';
            }
            if (entry.key !== key) {
                renderThumb(entry.canvas, isMask ? layer.maskCanvas : layer.canvas, box, ratio, isMask);
                entry.key = key;
            }
            return entry;
        }

        function paintInto(el, entry, force) {
            if (!force && el.dataset.thumbKey === entry.key) return;
            if (el.width !== entry.canvas.width || el.height !== entry.canvas.height) {
                el.width = entry.canvas.width;
                el.height = entry.canvas.height;
            }
            const ctx = el.getContext('2d');
            ctx.setTransform(1, 0, 0, 1, 0, 0);
            ctx.clearRect(0, 0, el.width, el.height);
            ctx.drawImage(entry.canvas, 0, 0);
            el.dataset.thumbKey = entry.key;
        }

        function paintThumbs(force) {
            lastThumbs = Date.now();
            rowEls().forEach(row => {
                const layer = r.layerById(Number(row.dataset.layerId));
                if (!layer) return;
                const canvas = row.querySelector('[data-layer-thumb]');
                if (canvas) paintInto(canvas, thumbEntry(layer, false), force);
                const maskCanvas = row.querySelector('[data-layer-mask-canvas]');
                if (maskCanvas && layer.maskCanvas) paintInto(maskCanvas, thumbEntry(layer, true), force);
            });
        }

        // A stroke bumps the rev on every frame; the thumbnails follow at most ten times per second.
        function scheduleThumbs() {
            if (thumbTimer || disposed) return;
            const wait = Math.max(0, THUMB_INTERVAL - (Date.now() - lastThumbs));
            thumbTimer = setTimeout(() => {
                thumbTimer = 0;
                if (alive() && !section.hidden) paintThumbs(false);
            }, wait);
        }

        // --- header and footer ---

        function syncHeader() {
            const layer = r.activeLayer();
            const off = !layer || !r.imgWidth;
            const focused = document.activeElement;
            [blendSel, rangeEl, numEl].forEach(el => { el.disabled = off; });
            lockBtns.forEach(btn => {
                const kind = btn.dataset.layerLock;
                let on = false, disabled = off;
                if (layer) {
                    if (kind === 'clip') {
                        on = !!layer.clip;
                        disabled = off || (!layer.clip && r.layers.indexOf(layer) === 0);
                    } else {
                        const lock = LOCKS.find(entry => entry.kind === kind);
                        on = !!(lock && layer[lock.prop]);
                    }
                }
                btn.disabled = disabled;
                btn.classList.toggle('active', on);
                btn.setAttribute('aria-pressed', on ? 'true' : 'false');
            });
            if (!layer) return;
            const pct = String(Math.round(layer.opacity * 100));
            if (focused !== blendSel) blendSel.value = layer.blend;
            if (focused !== rangeEl) rangeEl.value = pct;
            if (focused !== numEl) numEl.value = pct;
        }

        function syncFooter() {
            const layer = r.activeLayer();
            footerBtns.forEach(btn => {
                const act = btn.dataset.layerAct;
                let off = !layer || !r.imgWidth;
                if (act === 'delete') off = off || r.layers.length <= 1;
                if (act === 'merge') off = off || r.activeLayerIdx <= 0;
                if (act === 'mask') off = off || !!(layer && layer.maskCanvas);
                btn.disabled = off;
            });
        }

        function sync(force) {
            if (!alive()) return;
            if (section.hidden) return;
            syncHeader();
            syncFooter();
            if (!busy()) {
                const key = structureKey();
                if (force || key !== structKey) {
                    structKey = key;
                    rebuildRows();
                } else {
                    updateActive();
                }
            }
            scheduleThumbs();
        }

        // --- inline rename ---

        function startRename(id) {
            const layer = id == null ? r.activeLayer() : r.layerById(id);
            if (!layer || !alive()) return false;
            endRename(true);
            if (section.hidden) r.showPanel('layers');
            sync(true);
            const row = rowsEl.querySelector('[data-layer-id="' + layer.id + '"]');
            const nameEl = row ? row.querySelector('[data-layer-name]') : null;
            if (!nameEl) return false;
            const input = document.createElement('input');
            input.type = 'text';
            input.className = 'pixel-layer-rename';
            input.maxLength = 80;
            input.value = layer.name;
            input.setAttribute('aria-label', tr('pixel.layer_rename', 'Rename layer'));
            renaming = { id: layer.id, input, nameEl, done: false };
            nameEl.hidden = true;
            nameEl.after(input);
            input.addEventListener('keydown', e => {
                e.stopPropagation();
                if (e.key === 'Enter') { e.preventDefault(); endRename(true); }
                else if (e.key === 'Escape') { e.preventDefault(); endRename(false); }
            });
            input.addEventListener('blur', () => endRename(true));
            ['pointerdown', 'click', 'dblclick'].forEach(type => input.addEventListener(type, e => e.stopPropagation()));
            input.focus();
            input.select();
            return true;
        }

        function endRename(commit) {
            const state = renaming;
            if (!state || state.done) return;
            state.done = true;
            renaming = null;
            const value = state.input.value.trim();
            state.input.remove();
            state.nameEl.hidden = false;
            const layer = r.layerById(state.id);
            if (commit && value && layer && value !== layer.name) ops.rename(state.id, value);
            sync(true);
            if (listEl.focus) listEl.focus({ preventScroll: true });
        }

        // --- drag and drop ---

        function updateDrop(y) {
            const rows = rowEls().filter(row => row !== drag.row);
            const mids = rows.map(row => { const b = row.getBoundingClientRect(); return b.top + b.height / 2; });
            drag.slot = dropSlot(mids, y);
            let top = 0;
            if (rows.length) {
                top = drag.slot < rows.length ? rows[drag.slot].offsetTop : rows[rows.length - 1].offsetTop + rows[rows.length - 1].offsetHeight;
            }
            dropEl.style.top = Math.max(0, top - 1) + 'px';
            dropEl.hidden = false;
        }

        function scroller() {
            return section.closest ? section.closest('.pixel-dock-body') : null;
        }

        function runScroll() {
            if (scrollRaf || typeof requestAnimationFrame !== 'function') return;
            const step = () => {
                scrollRaf = 0;
                if (!drag || !drag.active || disposed) return;
                const box = scroller();
                if (box && drag.lastY != null) {
                    const rect = box.getBoundingClientRect();
                    let speed = 0;
                    if (drag.lastY < rect.top + SCROLL_EDGE) speed = -Math.ceil((rect.top + SCROLL_EDGE - drag.lastY) / 3);
                    else if (drag.lastY > rect.bottom - SCROLL_EDGE) speed = Math.ceil((drag.lastY - (rect.bottom - SCROLL_EDGE)) / 3);
                    if (speed) {
                        box.scrollTop += clampNumber(speed, -24, 24);
                        updateDrop(drag.lastY);
                    }
                }
                scrollRaf = requestAnimationFrame(step);
            };
            scrollRaf = requestAnimationFrame(step);
        }

        function dragKey(e) {
            if (e.key !== 'Escape') return;
            e.preventDefault();
            e.stopPropagation();
            endDrag(false);
        }

        function beginDrag(e) {
            drag.active = true;
            try { listEl.setPointerCapture(e.pointerId); } catch (_) { /* synthetic pointer */ }
            listEl.classList.add('is-dragging');
            drag.row.classList.add('dragging');
            document.addEventListener('keydown', dragKey, true);
            runScroll();
        }

        function endDrag(commit) {
            const state = drag;
            if (!state) return;
            drag = null;
            if (scrollRaf && typeof cancelAnimationFrame === 'function') cancelAnimationFrame(scrollRaf);
            scrollRaf = 0;
            try { listEl.releasePointerCapture(state.pointerId); } catch (_) { /* already released */ }
            if (!state.active) return;
            document.removeEventListener('keydown', dragKey, true);
            listEl.classList.remove('is-dragging');
            state.row.classList.remove('dragging');
            dropEl.hidden = true;
            // The click that follows the pointer release must not select a row.
            suppressClick = true;
            setTimeout(() => { suppressClick = false; }, 0);
            if (commit && state.slot >= 0) ops.moveTo(state.id, r.layers.length - 1 - state.slot);
            sync(true);
        }

        // --- context menu ---

        function closeMenu() {
            if (!menu) return;
            const state = menu;
            menu = null;
            state.el.remove();
            document.removeEventListener('pointerdown', state.onDown, true);
            document.removeEventListener('keydown', state.onKey, true);
            window.removeEventListener('blur', state.onClose);
            window.removeEventListener('resize', state.onClose);
        }

        function visibleItems(items) {
            const out = [];
            (items || []).forEach(entry => {
                if (!entry) return;
                if (entry.type === 'separator' || entry.separator) {
                    if (out.length && out[out.length - 1].type !== 'separator') out.push({ type: 'separator' });
                    return;
                }
                if (typeof entry.hidden === 'function' ? entry.hidden(r) : entry.hidden) return;
                out.push(entry.items ? Object.assign({}, entry, { items: visibleItems(entry.items) }) : entry);
            });
            while (out.length && out[out.length - 1].type === 'separator') out.pop();
            return out;
        }

        function menuHTML(items, path, actions) {
            return items.map((entry, i) => {
                if (entry.type === 'separator') return '<div class="vd-context-separator" role="separator"></div>';
                const key = (path ? path + '/' : '') + (entry.id || i);
                const label = esc(tr(entry.labelKey, entry.fallback || entry.label || entry.id));
                const off = typeof entry.enabled === 'function' ? !entry.enabled(r) : false;
                const mark = `<span class="vd-context-icon">${typeof entry.checked === 'function' && entry.checked(r) ? '✓' : ''}</span>`;
                if (entry.items) {
                    return `<div class="vd-context-submenu" role="none"><button type="button" class="vd-context-item" role="menuitem" aria-haspopup="true"${off ? ' disabled' : ''}>${mark}<span class="vd-context-label">${label}</span><span></span><span class="vd-context-arrow">›</span></button><div class="vd-context-submenu-popover" role="menu">${menuHTML(entry.items, key, actions)}</div></div>`;
                }
                actions.set(key, entry);
                const hint = entry.shortcut ? `<kbd class="vd-context-shortcut">${esc(entry.shortcut)}</kbd>` : '<span></span>';
                return `<button type="button" class="vd-context-item" role="menuitem" data-menu-action="${esc(key)}"${off ? ' disabled' : ''}>${mark}<span class="vd-context-label">${label}</span>${hint}<span></span></button>`;
            }).join('');
        }

        // Submenus open to the right by default; near the window edge they flip left or shift up.
        function placeSubmenu(sub) {
            const pop = sub.querySelector('.vd-context-submenu-popover');
            if (!pop) return;
            pop.style.left = '';
            pop.style.right = '';
            pop.style.top = '';
            const rect = pop.getBoundingClientRect();
            if (rect.right > window.innerWidth - 8) {
                pop.style.left = 'auto';
                pop.style.right = 'calc(100% + 6px)';
            }
            const overflow = pop.getBoundingClientRect().bottom - (window.innerHeight - 8);
            if (overflow > 0) pop.style.top = (-7 - overflow) + 'px';
        }

        // x/y: pointer position; anchor: rectangle of the button that opened the menu (the menu then opens above it).
        function openMenu(x, y, anchor) {
            closeMenu();
            if (!alive() || !r.layerOps) return;
            const actions = new Map();
            const items = visibleItems(menuItems(r));
            if (!items.length) return;
            const el = document.createElement('div');
            el.className = 'vd-context-menu vd-scroll pixel-layers-menu';
            el.setAttribute('role', 'menu');
            el.innerHTML = menuHTML(items, '', actions);
            document.body.appendChild(el);
            const limit = window.innerHeight - 16;
            if (el.scrollHeight > limit) { el.style.maxHeight = limit + 'px'; el.style.overflowY = 'auto'; }
            const box = el.getBoundingClientRect();
            let left = anchor ? anchor.left : x;
            let top = anchor ? anchor.top - box.height - 4 : y;
            if (anchor && top < 8) top = anchor.bottom + 4;
            el.style.left = Math.max(8, Math.min(left, window.innerWidth - box.width - 8)) + 'px';
            el.style.top = Math.max(8, Math.min(top, window.innerHeight - box.height - 8)) + 'px';
            const onClose = () => closeMenu();
            const onDown = e => {
                if (el.contains(e.target)) return;
                // A press on the button that opened the menu closes it; the click that follows must not reopen it.
                if (e.target.closest && e.target.closest('[data-layer-act="more"]')) {
                    skipMore = true;
                    setTimeout(() => { skipMore = false; }, 0);
                }
                closeMenu();
            };
            const onKey = e => { if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeMenu(); } };
            menu = { el, onDown, onKey, onClose };
            document.addEventListener('pointerdown', onDown, true);
            document.addEventListener('keydown', onKey, true);
            window.addEventListener('blur', onClose);
            window.addEventListener('resize', onClose);
            el.addEventListener('mouseover', e => {
                const sub = e.target.closest ? e.target.closest('.vd-context-submenu') : null;
                if (sub) placeSubmenu(sub);
            });
            el.addEventListener('click', e => {
                const button = e.target.closest ? e.target.closest('[data-menu-action]') : null;
                if (!button || button.disabled) return;
                const entry = actions.get(button.dataset.menuAction);
                closeMenu();
                if (entry && typeof entry.action === 'function') {
                    try { entry.action(r); } catch (err) { if (window.console) console.error('pixel layer menu', err); }
                }
            });
        }

        // --- events ---

        function modifierMode(e) {
            if (e.shiftKey && e.altKey) return 'intersect';
            if (e.shiftKey) return 'add';
            if (e.altKey) return 'subtract';
            return 'replace';
        }

        function onListClick(e) {
            if (suppressClick || renaming) return;
            const id = idOf(e.target);
            if (id == null) return;
            const target = e.target;
            if (target.closest('[data-layer-eye]')) {
                if (e.altKey) ops.solo(id);
                else ops.toggleVisible(id);
                return;
            }
            if (target.closest('[data-layer-mask-thumb]')) {
                if (e.ctrlKey || e.metaKey) ops.selectionFromMask(id, modifierMode(e));
                else if (e.shiftKey) ops.toggleMask(id);
                else ops.editMask(id);
                return;
            }
            if (target.closest('[data-layer-thumb-wrap]') && (e.ctrlKey || e.metaKey)) {
                ops.selectionFromLayer(id, modifierMode(e));
                return;
            }
            ops.select(id);
            if (listEl.focus) listEl.focus({ preventScroll: true });
        }

        function onListDblClick(e) {
            const id = idOf(e.target);
            if (id == null) return;
            if (e.target.closest('[data-layer-name]')) startRename(id);
            else if (e.target.closest('[data-layer-thumb-wrap]') && !(e.ctrlKey || e.metaKey)) ops.properties(id);
        }

        function onListContext(e) {
            e.preventDefault();
            e.stopPropagation();
            const id = idOf(e.target);
            if (id != null) ops.select(id);
            openMenu(e.clientX, e.clientY, null);
        }

        function onListKey(e) {
            if (renaming || (e.target.closest && e.target.closest('input, select, textarea'))) return;
            if (e.ctrlKey || e.metaKey || e.altKey) return;
            const consume = fn => { e.preventDefault(); e.stopPropagation(); fn(); };
            const active = r.activeLayer();
            switch (e.key) {
                case 'ArrowUp': consume(() => ops.selectRelative(1)); break;
                case 'ArrowDown': consume(() => ops.selectRelative(-1)); break;
                case 'Home': consume(() => ops.select(r.layers[r.layers.length - 1].id)); break;
                case 'End': consume(() => ops.select(r.layers[0].id)); break;
                case 'F2': consume(() => startRename()); break;
                // Only while the list itself has the focus; a click on the canvas moves the focus away from it.
                case 'Delete': consume(() => ops.remove()); break;
                case 'Enter': consume(() => ops.properties()); break;
                case ' ': if (active) consume(() => ops.toggleVisible(active.id)); break;
                case 'ContextMenu': consume(() => { const rect = listEl.getBoundingClientRect(); openMenu(rect.left + 24, rect.top + 24, null); }); break;
                default:
            }
        }

        function wire() {
            listEl.addEventListener('click', onListClick, sig);
            listEl.addEventListener('dblclick', onListDblClick, sig);
            listEl.addEventListener('contextmenu', onListContext, sig);
            listEl.addEventListener('keydown', onListKey, sig);
            listEl.addEventListener('pointerdown', e => {
                if (e.button !== 0 || renaming || r.layers.length < 2) return;
                if (e.target.closest('[data-layer-eye], [data-layer-mask-thumb], input')) return;
                const row = e.target.closest('[data-layer-id]');
                if (!row) return;
                drag = { id: Number(row.dataset.layerId), row, pointerId: e.pointerId, x: e.clientX, y: e.clientY, active: false, slot: -1, lastY: e.clientY };
            }, sig);
            listEl.addEventListener('pointermove', e => {
                if (!drag || e.pointerId !== drag.pointerId) return;
                if (!drag.active) {
                    if (Math.hypot(e.clientX - drag.x, e.clientY - drag.y) < DRAG_THRESHOLD) return;
                    beginDrag(e);
                }
                drag.lastY = e.clientY;
                updateDrop(e.clientY);
            }, sig);
            listEl.addEventListener('pointerup', e => { if (drag && e.pointerId === drag.pointerId) endDrag(true); }, sig);
            listEl.addEventListener('pointercancel', e => { if (drag && e.pointerId === drag.pointerId) endDrag(false); }, sig);
            listEl.addEventListener('lostpointercapture', e => { if (drag && drag.active && e.pointerId === drag.pointerId) endDrag(false); }, sig);

            blendSel.addEventListener('change', () => {
                const layer = r.activeLayer();
                if (!layer) return;
                const mode = blendSel.value;
                ops.setProps(layer.id, { blend: mode }, tr('pixel.layer_blend', 'Blend mode') + ': ' + tr(blendLabelKey(mode), blendFallback(mode)));
            }, sig);
            const liveOpacity = text => {
                const layer = r.activeLayer();
                const value = Number(text);
                if (!layer || text === '' || !Number.isFinite(value)) return;
                if (!opacityEdit || opacityEdit.id !== layer.id) opacityEdit = { id: layer.id, before: layer.opacity };
                const pct = clampNumber(Math.round(value), 0, 100);
                ops.setOpacity(layer.id, pct / 100);
                if (document.activeElement !== rangeEl) rangeEl.value = String(pct);
                if (document.activeElement !== numEl) numEl.value = String(pct);
            };
            const commitOpacity = () => {
                const edit = opacityEdit;
                opacityEdit = null;
                const layer = edit ? r.layerById(edit.id) : null;
                if (layer && layer.opacity !== edit.before) r.pushHistory(tr('pixel.layer_hist_opacity', 'Layer opacity'));
                sync();
            };
            rangeEl.addEventListener('input', () => liveOpacity(rangeEl.value), sig);
            rangeEl.addEventListener('change', commitOpacity, sig);
            numEl.addEventListener('input', () => liveOpacity(numEl.value), sig);
            numEl.addEventListener('change', commitOpacity, sig);
            lockBtns.forEach(btn => btn.addEventListener('click', () => {
                const kind = btn.dataset.layerLock;
                if (kind === 'clip') ops.toggleClip();
                else ops.toggleLock(null, kind);
            }, sig));
            footerBtns.forEach(btn => btn.addEventListener('click', () => {
                switch (btn.dataset.layerAct) {
                    case 'new': ops.newLayer(); break;
                    case 'copy': ops.viaCopy(); break;
                    case 'duplicate': ops.duplicate(); break;
                    case 'mask': ops.addMask({ mode: r.selection ? 'selection' : 'reveal' }); break;
                    case 'delete': ops.remove(); break;
                    case 'merge': ops.mergeDown(); break;
                    case 'more': if (!skipMore) openMenu(0, 0, btn.getBoundingClientRect()); break;
                    default:
                }
            }, sig));
        }

        wire();
        const offs = ['layers-changed', 'active-layer-changed', 'history-changed', 'tool-changed'].map(event => r.on(event, () => sync()));
        offs.push(r.on('document-changed', () => { structKey = ''; sync(true); }));
        offs.push(r.on('panel-changed', info => { if (info && info.id === 'layers') sync(true); }));

        const panel = {
            sync,
            rowsHTML,
            structureKey,
            startRename,
            openMenu,
            closeMenu,
            thumbnailEntry: thumbEntry,
            dispose() {
                if (disposed) return;
                closeMenu();
                endDrag(false);
                if (renaming) { renaming.done = true; renaming = null; }
                disposed = true;
                abort.abort();
                offs.forEach(off => off());
                if (thumbTimer) clearTimeout(thumbTimer);
                thumbTimer = 0;
                thumbs.clear();
                maskThumbs.clear();
            }
        };
        sync(true);
        return panel;
    }

    // --- registration ---

    Pixel.registerModule('layers', function install(r) {
        r.layerOps = createOps(r);
        r.layersPanel = null;
        // Everything that used to rebuild the built-in list (history restore, sticker add, mask toggles, ...) lands here.
        r.refreshLayerPanel = Pixel.bindRuntime(r, function refreshLayerPanel() {
            if (this.refreshStickerControls) this.refreshStickerControls();
            if (this.layersPanel) this.layersPanel.sync();
        });
        const baseDispose = r.disposeCore;
        r.disposeCore = Pixel.bindRuntime(r, function disposeCore() {
            if (this.layersPanel) this.layersPanel.dispose();
            this.layersPanel = null;
            return baseDispose.apply(this, arguments);
        });
    });

    Pixel.registerPanel({
        id: 'layers', labelKey: 'pixel.layers', fallback: 'Layers', order: 10, dock: 'bottom',
        build: r => buildPanelHTML(r),
        mount: (r, section) => { r.layersPanel = createPanel(r, section); },
        onShow: r => { if (r.layersPanel) r.layersPanel.sync(true); }
    });

    Pixel.registerMenuItems('layer', r => menuItems(r), { order: 0 });
})();
