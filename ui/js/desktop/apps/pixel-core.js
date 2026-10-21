(function () {
    'use strict';

    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    // Layers, masks, history snapshots and caches use plain canvases, never the temp pool: releasing a pooled canvas
    // clears it, which would wipe pixels that other history entries still share.
    function makeCanvas(width, height) {
        const canvas = document.createElement('canvas');
        canvas.width = Math.max(0, Math.round(width) || 0);
        canvas.height = Math.max(0, Math.round(height) || 0);
        canvas.getContext('2d', { willReadFrequently: true });
        return canvas;
    }

    function copyCanvas(source) {
        const copy = makeCanvas(source.width, source.height);
        if (source.width && source.height) copy.getContext('2d').drawImage(source, 0, 0);
        return copy;
    }

    function isDrawable(canvas) {
        return !!(canvas && canvas.width > 0 && canvas.height > 0);
    }

    // --- composition ---

    function maskActive(layer) {
        return !!(layer.maskCanvas && layer.maskEnabled && isDrawable(layer.maskCanvas));
    }

    function layerSource(layer, previews) {
        return previews && layer.preview ? layer.preview : layer.canvas;
    }

    // A clip layer is clipped to the nearest layer below it that is not a clip layer itself.
    function clipBase(layers, index) {
        for (let j = index - 1; j >= 0; j--) {
            if (!layers[j].clip) return layers[j];
        }
        return null;
    }

    function drawLayer(ctx, layers, index, previews) {
        const layer = layers[index];
        if (!layer || !layer.visible || !(layer.opacity > 0)) return;
        const source = layerSource(layer, previews);
        if (!isDrawable(source)) return;
        const masked = maskActive(layer);
        const base = layer.clip ? clipBase(layers, index) : null;
        if (base && (!base.visible || !(base.opacity > 0))) return;
        const op = Pixel.blendOp(layer.blend);
        if (!masked && !base) {
            ctx.globalAlpha = layer.opacity;
            ctx.globalCompositeOperation = op;
            ctx.drawImage(source, 0, 0);
            ctx.globalAlpha = 1;
            ctx.globalCompositeOperation = 'source-over';
            return;
        }
        const w = ctx.canvas.width, h = ctx.canvas.height;
        const tmp = Pixel.acquireTempCanvas(w, h);
        const tctx = tmp.getContext('2d');
        tctx.drawImage(source, 0, 0);
        if (masked) {
            tctx.globalCompositeOperation = 'destination-in';
            tctx.drawImage(layer.maskCanvas, 0, 0);
        }
        if (base) {
            const shape = Pixel.acquireTempCanvas(w, h);
            const sctx = shape.getContext('2d');
            const baseSource = layerSource(base, previews);
            if (isDrawable(baseSource)) {
                sctx.globalAlpha = base.opacity;
                sctx.drawImage(baseSource, 0, 0);
                sctx.globalAlpha = 1;
                if (maskActive(base)) {
                    sctx.globalCompositeOperation = 'destination-in';
                    sctx.drawImage(base.maskCanvas, 0, 0);
                }
            }
            tctx.globalCompositeOperation = 'destination-in';
            tctx.drawImage(shape, 0, 0);
            Pixel.releaseTempCanvas(shape);
        }
        ctx.globalAlpha = layer.opacity;
        ctx.globalCompositeOperation = op;
        ctx.drawImage(tmp, 0, 0);
        ctx.globalAlpha = 1;
        ctx.globalCompositeOperation = 'source-over';
        Pixel.releaseTempCanvas(tmp);
    }

    function renderRange(ctx, layers, from, to, previews) {
        for (let i = from; i < to; i++) drawLayer(ctx, layers, i, previews);
    }

    function countVisible(layers, from, to) {
        let count = 0;
        for (let i = from; i < to; i++) {
            if (layers[i].visible && layers[i].opacity > 0 && isDrawable(layers[i].canvas)) count++;
        }
        return count;
    }

    // Layers above the active one can live in a cache only when they do not depend on what is below them:
    // normal blend and no clipping, so (A over (B over C)) == ((A over B) over C).
    function aboveIsSimple(layers, from, to) {
        for (let i = from; i < to; i++) {
            const layer = layers[i];
            if (!layer.visible || !(layer.opacity > 0)) continue;
            if (layer.clip || Pixel.blendOp(layer.blend) !== 'source-over') return false;
        }
        return true;
    }

    function rangeKey(layers, from, to, w, h, previews) {
        let key = w + 'x' + h;
        for (let i = from; i < to; i++) {
            const l = layers[i];
            key += '|' + l.id + ':' + l.rev + ':' + (l.maskCanvas ? l.maskRev : 0) + ':' + (l.visible ? 1 : 0) + ':' + l.opacity + ':' +
                l.blend + ':' + (l.clip ? 1 : 0) + ':' + (l.maskEnabled ? 1 : 0) + ':' + (previews && l.preview ? (l.previewRev || 1) : 0);
        }
        return key;
    }

    function ensureCache(caches, slot, key, w, h, render) {
        let entry = caches[slot];
        if (!entry) entry = caches[slot] = { key: '', canvas: makeCanvas(w, h) };
        if (entry.canvas.width !== w || entry.canvas.height !== h) {
            entry.canvas.width = w;
            entry.canvas.height = h;
            entry.key = '';
        }
        if (entry.key !== key) {
            const cctx = entry.canvas.getContext('2d');
            cctx.clearRect(0, 0, w, h);
            render(cctx);
            entry.key = key;
        }
        return entry.canvas;
    }

    // --- history ---

    function layerMeta(layer) {
        const meta = {
            id: layer.id, name: layer.name, visible: layer.visible, opacity: layer.opacity, blend: layer.blend,
            lockPixels: layer.lockPixels, lockAlpha: layer.lockAlpha, lockPosition: layer.lockPosition,
            maskEnabled: layer.maskEnabled, clip: layer.clip, rev: layer.rev, maskRev: layer.maskRev, hasMask: !!layer.maskCanvas
        };
        if (layer.sticker) {
            meta.sticker = { id: layer.sticker.id, matrix: layer.sticker.matrix.slice() };
            meta.stickerImage = layer.stickerImage;
        }
        return meta;
    }

    function historyBytes(history) {
        const seen = new Set();
        let total = 0;
        history.forEach(entry => {
            entry.layers.forEach(item => {
                [item.snap, item.maskSnap].forEach(snap => {
                    if (snap && !seen.has(snap)) {
                        seen.add(snap);
                        total += snap.canvas.width * snap.canvas.height * 4;
                    }
                });
            });
            const alpha = entry.selection && entry.selection.alpha;
            if (alpha && !seen.has(alpha)) {
                seen.add(alpha);
                total += alpha.length;
            }
        });
        return total;
    }

    // --- selection masks (document-sized Uint8ClampedArray, 0..255) ---

    function maskBounds(alpha, w, h) {
        let minX = w, minY = h, maxX = -1, maxY = -1;
        for (let y = 0; y < h; y++) {
            const row = y * w;
            let first = -1;
            for (let x = 0; x < w; x++) {
                if (alpha[row + x]) { first = x; break; }
            }
            if (first < 0) continue;
            let last = first;
            for (let x = w - 1; x > first; x--) {
                if (alpha[row + x]) { last = x; break; }
            }
            if (first < minX) minX = first;
            if (last > maxX) maxX = last;
            if (y < minY) minY = y;
            maxY = y;
        }
        return maxX < 0 ? null : { x: minX, y: minY, w: maxX - minX + 1, h: maxY - minY + 1 };
    }

    // Contour pixels (alpha > 127 with a neighbour <= 127) for the marching ants.
    function maskBorder(alpha, w, h, b) {
        const border = [];
        for (let y = b.y; y < b.y + b.h; y++) {
            for (let x = b.x; x < b.x + b.w; x++) {
                const p = y * w + x;
                if (alpha[p] <= 127) continue;
                if ((x > 0 && alpha[p - 1] <= 127) || (x < w - 1 && alpha[p + 1] <= 127) ||
                    (y > 0 && alpha[p - w] <= 127) || (y < h - 1 && alpha[p + w] <= 127)) {
                    border.push(p);
                }
            }
        }
        return border;
    }

    function combineAlpha(current, next, mode) {
        const out = new Uint8ClampedArray(current.length);
        if (mode === 'add') {
            for (let i = 0; i < out.length; i++) out[i] = current[i] > next[i] ? current[i] : next[i];
        } else if (mode === 'subtract') {
            for (let i = 0; i < out.length; i++) out[i] = current[i] * (255 - next[i]) / 255;
        } else {
            for (let i = 0; i < out.length; i++) out[i] = current[i] < next[i] ? current[i] : next[i];
        }
        return out;
    }

    function rasterizeSelection(sel, docW, docH) {
        const rx = Math.round(Math.min(sel.x, sel.x + sel.w)), ry = Math.round(Math.min(sel.y, sel.y + sel.h));
        const rw = Math.round(Math.abs(sel.w)), rh = Math.round(Math.abs(sel.h));
        const x0 = Math.max(0, rx), y0 = Math.max(0, ry);
        const x1 = Math.min(docW, rx + rw), y1 = Math.min(docH, ry + rh);
        if (x1 <= x0 || y1 <= y0) return null;
        const bw = x1 - x0, bh = y1 - y0;
        const alpha = new Uint8ClampedArray(docW * docH);
        if (sel.type === 'ellipse') {
            const tmp = Pixel.acquireTempCanvas(bw, bh);
            const tctx = tmp.getContext('2d');
            tctx.fillStyle = '#ffffff';
            tctx.beginPath();
            tctx.ellipse(rx + rw / 2 - x0, ry + rh / 2 - y0, rw / 2, rh / 2, 0, 0, Math.PI * 2);
            tctx.fill();
            const d = tctx.getImageData(0, 0, bw, bh).data;
            Pixel.releaseTempCanvas(tmp);
            for (let y = 0; y < bh; y++) {
                let p = (y0 + y) * docW + x0, q = y * bw * 4 + 3;
                for (let x = 0; x < bw; x++, p++, q += 4) alpha[p] = d[q];
            }
        } else {
            for (let y = y0; y < y1; y++) alpha.fill(255, y * docW + x0, y * docW + x1);
        }
        return { alpha, x: x0, y: y0, w: bw, h: bh };
    }

    // --- writing pixels through an alpha mask ---

    function alphaCanvas(alpha, docW, box) {
        const canvas = Pixel.acquireTempCanvas(box.w, box.h);
        const actx = canvas.getContext('2d');
        const image = actx.createImageData(box.w, box.h);
        const d = image.data;
        for (let y = 0; y < box.h; y++) {
            let p = (box.y + y) * docW + box.x, q = y * box.w * 4 + 3;
            for (let x = 0; x < box.w; x++, p++, q += 4) d[q] = alpha[p];
        }
        actx.putImageData(image, 0, 0);
        return canvas;
    }

    // dst = dst * (1 - a) + src * a, evaluated on the GPU in premultiplied space.
    function blendThroughAlpha(ctx, source, alpha, docW, box) {
        const mask = alphaCanvas(alpha, docW, box);
        const part = Pixel.acquireTempCanvas(box.w, box.h);
        const pctx = part.getContext('2d');
        pctx.drawImage(source, box.x, box.y, box.w, box.h, 0, 0, box.w, box.h);
        pctx.globalCompositeOperation = 'destination-in';
        pctx.drawImage(mask, 0, 0);
        ctx.save();
        ctx.globalCompositeOperation = 'destination-out';
        ctx.drawImage(mask, box.x, box.y);
        ctx.globalCompositeOperation = 'lighter';
        ctx.drawImage(part, box.x, box.y);
        ctx.restore();
        Pixel.releaseTempCanvas(part);
        Pixel.releaseTempCanvas(mask);
    }

    function replaceRegion(ctx, source, box) {
        ctx.clearRect(box.x, box.y, box.w, box.h);
        ctx.drawImage(source, box.x, box.y, box.w, box.h, box.x, box.y, box.w, box.h);
    }

    // Alpha lock: colour changes, the layer's own alpha channel is kept.
    function commitKeepingAlpha(ctx, source, alpha, docW, box) {
        const work = Pixel.acquireTempCanvas(box.w, box.h);
        const wctx = work.getContext('2d');
        wctx.drawImage(ctx.canvas, box.x, box.y, box.w, box.h, 0, 0, box.w, box.h);
        wctx.translate(-box.x, -box.y);
        if (alpha) blendThroughAlpha(wctx, source, alpha, docW, box);
        else replaceRegion(wctx, source, box);
        wctx.setTransform(1, 0, 0, 1, 0, 0);
        const next = wctx.getImageData(0, 0, box.w, box.h);
        const old = ctx.getImageData(box.x, box.y, box.w, box.h);
        const n = next.data, o = old.data;
        for (let i = 0; i < n.length; i += 4) {
            const oa = o[i + 3];
            if (oa === 0) { n[i] = n[i + 1] = n[i + 2] = n[i + 3] = 0; continue; }
            if (n[i + 3] === 0) { n[i] = o[i]; n[i + 1] = o[i + 1]; n[i + 2] = o[i + 2]; }
            n[i + 3] = oa;
        }
        ctx.putImageData(next, box.x, box.y);
        Pixel.releaseTempCanvas(work);
    }

    // --- registries ---
    // Shared by every Pixel window. Feature modules call these at load time (before any window renders), so a new
    // module only has to be listed in the loader: no shell file needs an edit. Registering an existing id replaces the
    // earlier definition; fields left out (order, dock, group, svg, shortcut, ...) are inherited from the replaced one.
    const registry = { version: 0, seq: 0, modules: [], panels: new Map(), tools: new Map(), commands: new Map(), menus: [] };
    const PANEL_INHERIT = ['labelKey', 'fallback', 'order', 'dock', 'icon'];
    const TOOL_INHERIT = ['group', 'order', 'svg', 'shortcut', 'labelKey', 'fallback', 'cursor', 'raster'];
    const MENU_LABELS = {
        file: ['desktop.menu_file', 'File'], edit: ['desktop.menu_edit', 'Edit'], image: ['pixel.menu_image', 'Image'],
        layer: ['pixel.menu_layer', 'Layer'], select: ['pixel.menu_select', 'Select'], filter: ['pixel.menu_filter', 'Filter'],
        generate: ['pixel.menu_generate', 'Generate'], view: ['desktop.menu_view', 'View'], ai: ['pixel.ai_generate', 'AI']
    };

    function inheritFields(next, prev, fields) {
        if (!prev) return next;
        fields.forEach(key => { if (next[key] === undefined) next[key] = prev[key]; });
        return next;
    }

    function bump() {
        registry.version++;
        return ++registry.seq;
    }

    // install(runtime) runs once per window, after the built-in installers, in registration (= load) order.
    Pixel.registerModule = function registerModule(id, install) {
        if (!id || typeof install !== 'function') return;
        const entry = { id: String(id), install };
        const index = registry.modules.findIndex(m => m.id === entry.id);
        if (index >= 0) registry.modules[index] = entry;
        else registry.modules.push(entry);
        bump();
    };
    Pixel.listModules = function listModules() {
        return registry.modules.slice();
    };

    // def: { id, labelKey, fallback, order, dock: 'inspector'|'bottom', icon?, build(r) -> inner html, mount(r, sectionEl),
    //        onShow(r, previousId), onHide(r) }
    Pixel.registerPanel = function registerPanel(def) {
        if (!def || !def.id) return null;
        const prev = registry.panels.get(def.id);
        const next = inheritFields(Object.assign({}, def), prev, PANEL_INHERIT);
        next.dock = next.dock === 'bottom' ? 'bottom' : 'inspector';
        if (typeof next.order !== 'number') next.order = 1000;
        next.seq = prev ? prev.seq : bump();
        if (prev) bump();
        registry.panels.set(next.id, next);
        return next;
    };
    Pixel.getPanel = function getPanel(id) {
        return registry.panels.get(id) || null;
    };
    Pixel.listPanels = function listPanels(dock) {
        return Array.from(registry.panels.values())
            .filter(def => !dock || def.dock === dock)
            .sort((a, b) => a.order - b.order || a.seq - b.seq);
    };

    // def: { id, group, order, svg, shortcut, labelKey, fallback, raster, cursor: string|fn(r), options(r) -> html,
    //        wireOptions(r, barEl), onDown(r, e, p), onMove(r, e, p), onUp(r, e, p), onHover(r, e, p), onKey(r, e),
    //        onActivate(r), onDeactivate(r), overlay(ctx, r) }
    // A tool with any pointer handler owns its events; one without keeps the legacy code path of pixel-events.js.
    Pixel.registerTool = function registerTool(def) {
        if (!def || !def.id) return null;
        const prev = registry.tools.get(def.id);
        const next = inheritFields(Object.assign({}, def), prev, TOOL_INHERIT);
        if (!next.group) next.group = 'paint';
        if (typeof next.order !== 'number') next.order = 1000;
        next.seq = prev ? prev.seq : bump();
        if (prev) bump();
        registry.tools.set(next.id, next);
        return next;
    };
    Pixel.getTool = function getTool(id) {
        return registry.tools.get(id) || null;
    };
    Pixel.listTools = function listTools(group) {
        const rank = g => { const i = Pixel.TOOL_GROUP_ORDER.indexOf(g); return i < 0 ? Pixel.TOOL_GROUP_ORDER.length : i; };
        return Array.from(registry.tools.values())
            .filter(def => !group || def.group === group)
            .sort((a, b) => (rank(a.group) - rank(b.group)) || (a.group < b.group ? -1 : (a.group > b.group ? 1 : 0)) || a.order - b.order || a.seq - b.seq);
    };
    Pixel.toolGroups = function toolGroups() {
        const groups = [];
        Pixel.listTools().forEach(def => {
            let entry = groups[groups.length - 1];
            if (!entry || entry.group !== def.group) groups.push(entry = { group: def.group, tools: [] });
            entry.tools.push(def);
        });
        return groups;
    };
    Pixel.toolHasHandlers = function toolHasHandlers(def) {
        return !!(def && (typeof def.onDown === 'function' || typeof def.onMove === 'function' || typeof def.onUp === 'function' || typeof def.onHover === 'function'));
    };

    // items: array of window-menu items, or a function (r) -> array evaluated per window. Item actions get the runtime:
    // { id, labelKey, icon, shortcut, action(r), enabled(r), checked(r), items: [...] }. An item whose id already exists in
    // the same menu replaces it. Chunks of one menu are ordered by options.order (default 100), then by registration.
    Pixel.registerMenuItems = function registerMenuItems(menuId, items, options) {
        if (!menuId || !items) return;
        const order = options && typeof options.order === 'number' ? options.order : 100;
        registry.menus.push({ menuId: String(menuId), items, order, seq: bump() });
    };
    Pixel.listMenuItems = function listMenuItems(menuId, runtime) {
        return composeMenuItems(menuId, runtime);
    };

    // def: { id, labelKey, fallback, keywords: [], group, shortcut: string|string[], run(r), enabled(r) }
    Pixel.registerCommand = function registerCommand(def) {
        if (!def || !def.id || typeof def.run !== 'function') return null;
        const next = Object.assign({}, def);
        next.seq = bump();
        registry.commands.set(next.id, next);
        return next;
    };
    Pixel.getCommand = function getCommand(id) {
        return registry.commands.get(id) || null;
    };

    function trimSeparators(list) {
        const out = [];
        list.forEach(item => {
            const sep = item.type === 'separator' || item.separator;
            if (sep && (!out.length || out[out.length - 1].type === 'separator')) return;
            out.push(sep ? { type: 'separator' } : item);
        });
        while (out.length && out[out.length - 1].type === 'separator') out.pop();
        return out;
    }

    function composeMenuItems(menuId, runtime) {
        const chunks = registry.menus.filter(chunk => chunk.menuId === menuId).sort((a, b) => a.order - b.order || a.seq - b.seq);
        const out = [];
        chunks.forEach(chunk => {
            let list = typeof chunk.items === 'function' ? chunk.items(runtime) : chunk.items;
            list = (Array.isArray(list) ? list : []).filter(Boolean);
            if (!list.length) return;
            if (out.length) out.push({ type: 'separator' });
            list.forEach(item => {
                const index = item.id && item.type !== 'separator' ? out.findIndex(existing => existing.id === item.id) : -1;
                if (index >= 0) out[index] = item;
                else out.push(item);
            });
        });
        return trimSeparators(out);
    }

    function bindMenuItem(item, runtime) {
        if (item.type === 'separator' || item.separator) return { type: 'separator' };
        const bound = Object.assign({}, item);
        const children = item.items || item.children;
        if (children) {
            bound.items = trimSeparators((typeof children === 'function' ? children(runtime) : children).filter(Boolean)).map(child => bindMenuItem(child, runtime));
            delete bound.children;
        } else if (typeof item.action === 'function') {
            bound.action = () => item.action(runtime);
        }
        if (typeof item.enabled === 'function') bound.disabled = () => !item.enabled(runtime);
        else if (typeof item.disabled === 'function') bound.disabled = () => !!item.disabled(runtime);
        if (typeof item.checked === 'function') bound.checked = () => !!item.checked(runtime);
        if (typeof item.hidden === 'function') bound.hidden = !!item.hidden(runtime);
        return bound;
    }

    // The window menus in the fixed top-level order (file, edit, image, layer, select, filter, generate, view, ai);
    // menus without items are left out. Unknown menu ids registered by modules follow the known ones.
    Pixel.composeMenus = function composeMenus(runtime) {
        const known = Pixel.MENU_ORDER.slice();
        registry.menus.forEach(chunk => { if (!known.includes(chunk.menuId)) known.push(chunk.menuId); });
        return known.map(id => {
            const items = composeMenuItems(id, runtime).map(item => bindMenuItem(item, runtime));
            if (!items.length) return null;
            const label = MENU_LABELS[id] || [null, id];
            return { id, labelKey: label[0] || '', label: label[1], items };
        }).filter(Boolean);
    };

    function walkMenuCommands(menuId, items, prefix, out, runtime) {
        items.forEach(item => {
            if (!item || item.type === 'separator' || item.separator) return;
            const children = item.items || item.children;
            const id = prefix + (item.id || item.labelKey || '');
            if (children) {
                walkMenuCommands(menuId, trimSeparators((typeof children === 'function' ? children(runtime) : children).filter(Boolean)), id + '/', out, runtime);
                return;
            }
            if (typeof item.action !== 'function' || !item.id) return;
            out.push({
                id: 'menu:' + menuId + ':' + id,
                labelKey: item.labelKey, fallback: item.fallback || item.label || item.id,
                group: menuId, keywords: item.keywords || [],
                shortcut: [item.shortcut, item.shortcutHint].concat(item.shortcuts || []).filter(Boolean),
                run: r => item.action(r),
                enabled: typeof item.enabled === 'function' ? item.enabled : (typeof item.disabled === 'function' ? r => !item.disabled(r) : (item.disabled ? () => false : null)),
                source: 'menu'
            });
        });
    }

    // Every command the window knows: explicit ones, plus one per menu item, tool ("Tool: ...") and panel ("Show ...").
    // Entries: { id, label, group, keywords, shortcuts: [canonical], shortcut: display, run(r), enabled(r), source }.
    Pixel.listCommands = function listCommands(runtime) {
        const t = runtime && runtime.t ? runtime.t : (key, fallback) => fallback || key;
        const raw = [];
        Pixel.MENU_ORDER.concat(registry.menus.map(chunk => chunk.menuId)).filter((id, i, all) => all.indexOf(id) === i).forEach(menuId => {
            walkMenuCommands(menuId, composeMenuItems(menuId, runtime), '', raw, runtime);
        });
        Pixel.listTools().forEach(def => {
            raw.push({
                id: 'tool:' + def.id, source: 'tool', group: 'tools',
                label: t('pixel.cmd_tool_prefix', 'Tool') + ': ' + (def.labelKey ? t(def.labelKey, def.fallback || def.id) : t('pixel.' + String(def.id).replace(/-/g, '_'), def.fallback || def.id)),
                shortcut: def.shortcut || Pixel.TOOL_SHORTCUTS[def.id] || '', keywords: [def.id, def.group],
                run: r => r.setActiveTool(def.id)
            });
        });
        Pixel.listPanels().forEach(def => {
            raw.push({
                id: 'panel:' + def.id, source: 'panel', group: 'panels',
                label: t('pixel.cmd_show_prefix', 'Show') + ' ' + t(def.labelKey || ('pixel.' + def.id), def.fallback || def.id),
                keywords: [def.id], run: r => r.showPanel(def.id)
            });
        });
        registry.commands.forEach(def => raw.push(Object.assign({ source: 'command' }, def)));
        return raw.map(cmd => {
            const list = [].concat(cmd.shortcut || []).filter(Boolean);
            return {
                id: cmd.id,
                label: cmd.label || t(cmd.labelKey || '', cmd.fallback || cmd.id),
                group: cmd.group || '', keywords: cmd.keywords || [],
                shortcut: list[0] || '',
                shortcuts: list.map(Pixel.normalizeShortcut).filter(Boolean),
                run: cmd.run, enabled: typeof cmd.enabled === 'function' ? cmd.enabled : null,
                source: cmd.source
            };
        });
    };

    // --- jobs: pool of two workers with a main-thread fallback ---

    function abortError() {
        const err = new Error('Aborted');
        err.name = 'AbortError';
        return err;
    }

    function createJobHost(getLib) {
        const maxWorkers = 2;
        const workers = [];
        const queue = [];
        const keyed = new Map();
        let nextId = 0, proven = false, broken = false, closed = false, syncJob = null;

        const workerUsable = () => !broken && typeof Worker === 'function';

        function workerURL() {
            const path = '/js/desktop/apps/pixel-worker.js';
            if (window.AuraLazyAssets && typeof window.AuraLazyAssets.versionedURL === 'function') return window.AuraLazyAssets.versionedURL(path);
            return path + '?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev');
        }

        function settle(job, ok, value) {
            if (job.done) return;
            job.done = true;
            if (job.key && keyed.get(job.key) === job) keyed.delete(job.key);
            if (job.signal && job.abortListener) job.signal.removeEventListener('abort', job.abortListener);
            const index = queue.indexOf(job);
            if (index >= 0) queue.splice(index, 1);
            if (ok) job.resolve(value);
            else job.reject(value);
        }

        function dropWorker(slot) {
            const index = workers.indexOf(slot);
            if (index >= 0) workers.splice(index, 1);
            slot.worker.onmessage = slot.worker.onerror = null;
            try { slot.worker.terminate(); } catch (_) { /* already gone */ }
        }

        function cancel(job) {
            if (job.done) return;
            job.cancelled = true;
            // A running worker cannot be interrupted: it is terminated and a fresh one is created on demand.
            if (job.slot) dropWorker(job.slot);
            job.slot = null;
            settle(job, false, abortError());
            pump();
        }

        function failOver() {
            // Workers that never reported ready cannot be used here (blocked, missing file, ...): run everything on this thread.
            broken = true;
            const orphans = workers.slice().map(slot => slot.job).filter(Boolean);
            workers.slice().forEach(dropWorker);
            orphans.forEach(job => { job.slot = null; });
            queue.unshift(...orphans.filter(job => !job.done));
        }

        function spawn() {
            let worker;
            try {
                worker = new Worker(workerURL());
            } catch (_) {
                broken = true;
                return null;
            }
            const slot = { worker, job: null };
            worker.onmessage = event => {
                const data = event.data || {};
                if (data.ready) { proven = true; return; }
                const job = slot.job;
                if (!job || data.id !== job.id) return;
                if (data.progress !== undefined) {
                    if (job.onProgress) { try { job.onProgress(data.progress); } catch (_) { /* caller bug must not stop the job */ } }
                    return;
                }
                slot.job = null;
                job.slot = null;
                if (data.error !== undefined) settle(job, false, new Error(data.error));
                else settle(job, true, data.result);
                pump();
            };
            worker.onerror = event => {
                if (event && event.preventDefault) event.preventDefault();
                const job = slot.job;
                if (!proven) {
                    failOver();
                } else {
                    dropWorker(slot);
                    if (job) { job.slot = null; settle(job, false, new Error((event && event.message) || 'Pixel worker failed')); }
                }
                pump();
            };
            workers.push(slot);
            return slot;
        }

        function start(slot, job) {
            slot.job = job;
            job.slot = slot;
            // Buffers are only handed over once a worker has proven itself, so a failed start can still run the job locally.
            try {
                slot.worker.postMessage({ id: job.id, op: job.op, args: job.args }, proven ? job.transfer : []);
            } catch (err) {
                slot.job = null;
                job.slot = null;
                settle(job, false, err);
            }
        }

        function runSync(job) {
            syncJob = job;
            setTimeout(() => {
                const finish = () => { syncJob = null; pump(); };
                if (job.done) { finish(); return; }
                const lib = getLib();
                if (!lib) { settle(job, false, new Error('Pixel library is not loaded')); finish(); return; }
                let res;
                try {
                    res = lib.handleMessage({ id: job.id, op: job.op, args: job.args }, {
                        progress: fraction => { if (!job.done && job.onProgress) { try { job.onProgress(fraction); } catch (_) { /* caller bug */ } } },
                        cancelled: () => job.cancelled || !!(job.signal && job.signal.aborted)
                    });
                } catch (err) {
                    res = { id: job.id, error: err && err.message ? err.message : String(err) };
                }
                Promise.resolve(res).then(out => {
                    if (out && out.error !== undefined) settle(job, false, new Error(out.error));
                    else settle(job, true, out ? out.result : undefined);
                    finish();
                });
            }, 0);
        }

        function pump() {
            if (closed) return;
            while (queue.length) {
                if (workerUsable()) {
                    let slot = workers.find(candidate => !candidate.job);
                    if (!slot && workers.length < maxWorkers) {
                        slot = spawn();
                        if (!slot) continue;
                    }
                    if (!slot) return;
                    start(slot, queue.shift());
                } else {
                    if (syncJob) return;
                    runSync(queue.shift());
                }
            }
        }

        return {
            submit(op, args, options) {
                const o = options || {};
                if (closed || (o.signal && o.signal.aborted)) return Promise.reject(abortError());
                return new Promise((resolve, reject) => {
                    const job = {
                        id: ++nextId, op, args, transfer: Array.isArray(o.transfer) ? o.transfer : [], key: o.key || null,
                        signal: o.signal || null, onProgress: typeof o.onProgress === 'function' ? o.onProgress : null,
                        resolve, reject, done: false, slot: null, cancelled: false, abortListener: null
                    };
                    if (job.key) {
                        const previous = keyed.get(job.key);
                        if (previous) cancel(previous);
                        keyed.set(job.key, job);
                    }
                    if (job.signal) {
                        job.abortListener = () => cancel(job);
                        job.signal.addEventListener('abort', job.abortListener, { once: true });
                    }
                    queue.push(job);
                    pump();
                });
            },
            info() {
                return { workers: workers.length, queued: queue.length, usingWorkers: workerUsable(), proven };
            },
            dispose() {
                closed = true;
                const jobs = queue.slice().concat(workers.map(slot => slot.job).filter(Boolean), syncJob ? [syncJob] : []);
                workers.slice().forEach(dropWorker);
                jobs.forEach(job => { job.slot = null; settle(job, false, abortError()); });
            }
        };
    }

    Pixel.makeCanvas = makeCanvas;
    Pixel.copyCanvas = copyCanvas;

    Pixel.installCore = function installCore(runtime) {
        const listeners = Object.create(null);
        const caches = { below: null, above: null, flat: null };
        const overlays = new Map();
        let layersFrame = 0;
        let overlayFrame = 0;
        let lastActiveLayerId = null;
        let selectionCache = null;
        let disposed = false;
        let jobHost = null;
        let commandCache = null;
        let menuTimer = 0;

        Object.assign(runtime, {
            nextLayerId: 0,
            revSeq: 0,
            selectionRev: 0,
            makeCanvas,
            copyCanvas,

            // --- event bus ---
            on: Pixel.bindRuntime(runtime, function on(event, fn) {
                if (typeof fn !== 'function') return () => {};
                (listeners[event] = listeners[event] || new Set()).add(fn);
                return () => { if (listeners[event]) listeners[event].delete(fn); };
            }),
            emit: Pixel.bindRuntime(runtime, function emit(event, data) {
                const set = listeners[event];
                if (!set || !set.size) return;
                Array.from(set).forEach(fn => {
                    try { fn(data); } catch (err) { if (window.console) console.error('pixel ' + event, err); }
                });
            }),
            // layers-changed is coalesced to one event per animation frame.
            notifyLayersChanged: Pixel.bindRuntime(runtime, function notifyLayersChanged() {
                if (disposed || layersFrame) return;
                if (typeof requestAnimationFrame !== 'function') { this.emit('layers-changed'); return; }
                layersFrame = requestAnimationFrame(() => {
                    layersFrame = 0;
                    if (disposed) return;
                    this.syncActiveLayer();
                    this.emit('layers-changed');
                });
            }),
            // active-layer-changed fires when the identity of the active layer changes, not on every index write.
            syncActiveLayer: Pixel.bindRuntime(runtime, function syncActiveLayer() {
                const layer = this.activeLayer();
                if (!layer || layer.id === lastActiveLayerId) return;
                lastActiveLayerId = layer.id;
                this.emit('active-layer-changed', { id: layer.id, index: this.activeLayerIdx });
            }),
            disposeCore: Pixel.bindRuntime(runtime, function disposeCore() {
                disposed = true;
                if (layersFrame && typeof cancelAnimationFrame === 'function') cancelAnimationFrame(layersFrame);
                layersFrame = 0;
                if (overlayFrame && typeof cancelAnimationFrame === 'function') cancelAnimationFrame(overlayFrame);
                overlayFrame = 0;
                if (menuTimer) clearTimeout(menuTimer);
                menuTimer = 0;
                if (jobHost) jobHost.dispose();
                jobHost = null;
                overlays.clear();
                commandCache = null;
                Object.keys(listeners).forEach(key => delete listeners[key]);
                caches.below = caches.above = caches.flat = null;
                selectionCache = null;
            }),

            // --- overlay compositor ---
            // The overlay canvas is document-sized and sits above the display canvas. Every overlay is a draw function;
            // requestOverlay() repaints all of them in z order on the next animation frame (clear, then draw each).
            registerOverlay: Pixel.bindRuntime(runtime, function registerOverlay(id, draw, z) {
                if (!id || typeof draw !== 'function') return () => {};
                const previous = overlays.get(id);
                overlays.set(id, { id, draw, z: typeof z === 'number' ? z : 50, seq: previous ? previous.seq : overlays.size });
                this.requestOverlay();
                return () => this.unregisterOverlay(id);
            }),
            unregisterOverlay: Pixel.bindRuntime(runtime, function unregisterOverlay(id) {
                if (overlays.delete(id)) this.requestOverlay();
            }),
            requestOverlay: Pixel.bindRuntime(runtime, function requestOverlay() {
                if (disposed || overlayFrame) return;
                if (typeof requestAnimationFrame !== 'function') { this.paintOverlays(); return; }
                overlayFrame = requestAnimationFrame(() => {
                    overlayFrame = 0;
                    if (!disposed) this.paintOverlays();
                });
            }),
            paintOverlays: Pixel.bindRuntime(runtime, function paintOverlays() {
                const ctx = this.olCtx, canvas = this.overlayCanvas;
                if (!ctx || !canvas) return;
                ctx.setTransform(1, 0, 0, 1, 0, 0);
                ctx.globalAlpha = 1;
                ctx.globalCompositeOperation = 'source-over';
                ctx.clearRect(0, 0, canvas.width, canvas.height);
                Array.from(overlays.values()).sort((a, b) => a.z - b.z || a.seq - b.seq).forEach(entry => {
                    ctx.save();
                    try { entry.draw(ctx, this); } catch (err) { if (window.console) console.error('pixel overlay ' + entry.id, err); }
                    ctx.restore();
                });
            }),
            listOverlays: Pixel.bindRuntime(runtime, function listOverlays() {
                return Array.from(overlays.values()).sort((a, b) => a.z - b.z || a.seq - b.seq).map(entry => entry.id);
            }),

            // --- jobs (worker pool with main-thread fallback) ---
            // runJob(op, args, { transfer = [], signal, key, onProgress }) -> Promise of the op result. Jobs with the same key
            // replace each other (the older one rejects with an AbortError), so previews can fire on every slider move.
            runJob: Pixel.bindRuntime(runtime, function runJob(op, args, options) {
                if (disposed) return Promise.reject(abortError());
                if (!jobHost) jobHost = createJobHost(() => window.AuraPixelLib || null);
                return jobHost.submit(op, args || {}, options);
            }),
            jobInfo: Pixel.bindRuntime(runtime, function jobInfo() {
                return jobHost ? jobHost.info() : { workers: 0, queued: 0, usingWorkers: typeof Worker === 'function', proven: false };
            }),
            // { width, height, data: Uint8ClampedArray RGBA } of a canvas; the data is a copy.
            imageFromCanvas: Pixel.bindRuntime(runtime, function imageFromCanvas(canvas) {
                const w = canvas.width, h = canvas.height;
                if (!w || !h) return { width: w, height: h, data: new Uint8ClampedArray(0) };
                return { width: w, height: h, data: canvas.getContext('2d', { willReadFrequently: true }).getImageData(0, 0, w, h).data };
            }),
            canvasFromImage: Pixel.bindRuntime(runtime, function canvasFromImage(img) {
                const canvas = makeCanvas(img.width, img.height);
                if (!canvas.width || !canvas.height) return canvas;
                const ctx = canvas.getContext('2d');
                if (typeof ImageData === 'function') {
                    ctx.putImageData(new ImageData(img.data, img.width, img.height), 0, 0);
                } else {
                    const target = ctx.createImageData(img.width, img.height);
                    target.data.set(img.data);
                    ctx.putImageData(target, 0, 0);
                }
                return canvas;
            }),

            // --- commands ---
            listCommands: Pixel.bindRuntime(runtime, function listCommands() {
                return Pixel.listCommands(this);
            }),
            runCommand: Pixel.bindRuntime(runtime, function runCommand(id) {
                const cmd = this.listCommands().find(entry => entry.id === id);
                if (!cmd || (cmd.enabled && cmd.enabled(this) === false)) return false;
                cmd.run(this);
                return true;
            }),
            // Commands bound to a canonical shortcut ("ctrl+shift+z"), most specific registration last.
            commandsForShortcut: Pixel.bindRuntime(runtime, function commandsForShortcut(canonical) {
                if (!commandCache || commandCache.version !== registry.version) {
                    const table = new Map();
                    // Menu items first, then tools and panels, explicit commands last: later entries win on conflicts.
                    const order = { menu: 0, tool: 1, panel: 2, command: 3 };
                    this.listCommands().sort((a, b) => (order[a.source] || 0) - (order[b.source] || 0)).forEach(cmd => {
                        cmd.shortcuts.forEach(key => {
                            if (!table.has(key)) table.set(key, []);
                            table.get(key).push(cmd);
                        });
                    });
                    commandCache = { version: registry.version, table };
                }
                return commandCache.table.get(canonical) || [];
            }),
            // Runs the newest enabled command for a keyboard event; returns the command or null.
            runShortcut: Pixel.bindRuntime(runtime, function runShortcut(e) {
                const list = this.commandsForShortcut(Pixel.shortcutFromEvent(e));
                for (let i = list.length - 1; i >= 0; i--) {
                    const cmd = list[i];
                    if (cmd.enabled && cmd.enabled(this) === false) continue;
                    e.preventDefault();
                    cmd.run(this);
                    return cmd;
                }
                return null;
            }),
            // Window menus are built from the registry; enabled()/checked() states are read when they are (re)built.
            rebuildMenus: Pixel.bindRuntime(runtime, function rebuildMenus() {
                if (disposed || typeof this.ctx.setWindowMenus !== 'function') return;
                const menus = Pixel.composeMenus(this);
                const dynamic = items => items.some(item => typeof item.disabled === 'function' || typeof item.checked === 'function' || (item.items && dynamic(item.items)));
                this.menusDynamic = menus.some(menu => dynamic(menu.items));
                this.ctx.setWindowMenus(this.windowId, menus);
            }),
            // Coalesced rebuild for state changes (selection, history, layers); a menu open at that moment closes.
            refreshMenus: Pixel.bindRuntime(runtime, function refreshMenus() {
                if (disposed || menuTimer) return;
                menuTimer = setTimeout(() => {
                    menuTimer = 0;
                    if (!disposed) this.rebuildMenus();
                }, 150);
            }),

            // --- which Pixel window gets keyboard shortcuts ---
            // The window with focus inside it, else the one that saw the latest pointer activity. Two windows never both react.
            isActiveWindow: Pixel.bindRuntime(runtime, function isActiveWindow() {
                if (disposed || !this.host) return false;
                const rects = this.host.getClientRects ? this.host.getClientRects() : null;
                if (rects && rects.length === 0) return false;
                const focused = document.activeElement;
                if (focused && focused !== document.body) {
                    if (this.host.contains(focused)) return true;
                    if (focused.closest && focused.closest('.pixel-app')) return false;
                }
                return Pixel.activeInstance === this.state;
            }),
            markActiveWindow: Pixel.bindRuntime(runtime, function markActiveWindow() {
                this.state.lastActive = Date.now();
                Pixel.activeInstance = this.state;
            }),

            // --- layers ---
            nextRev: Pixel.bindRuntime(runtime, function nextRev() {
                return ++this.revSeq;
            }),
            createLayer: Pixel.bindRuntime(runtime, function createLayer(opts) {
                const o = opts || {};
                const w = o.width != null ? o.width : this.imgWidth;
                const h = o.height != null ? o.height : this.imgHeight;
                const canvas = o.canvas || makeCanvas(w, h);
                let id = o.id;
                if (!id) id = ++this.nextLayerId;
                else if (id > this.nextLayerId) this.nextLayerId = id;
                const layer = {
                    id,
                    name: o.name != null ? o.name : '',
                    canvas,
                    visible: o.visible !== false,
                    opacity: typeof o.opacity === 'number' ? o.opacity : 1,
                    blend: Pixel.BLEND_MODES.includes(o.blend) ? o.blend : 'normal',
                    lockPixels: !!o.lockPixels,
                    lockAlpha: !!o.lockAlpha,
                    lockPosition: !!o.lockPosition,
                    maskCanvas: o.maskCanvas || null,
                    maskEnabled: o.maskEnabled !== false,
                    clip: !!o.clip,
                    rev: o.rev || this.nextRev(),
                    maskRev: o.maskRev || this.nextRev(),
                    preview: null
                };
                if (o.sticker) {
                    layer.sticker = o.sticker;
                    layer.stickerImage = o.stickerImage;
                }
                if (o.source && isDrawable(canvas)) canvas.getContext('2d').drawImage(o.source, 0, 0);
                return layer;
            }),
            activeLayer: Pixel.bindRuntime(runtime, function activeLayer() {
                return this.layers[this.activeLayerIdx] || null;
            }),
            layerById: Pixel.bindRuntime(runtime, function layerById(id) {
                return this.layers.find(layer => layer.id === id) || null;
            }),
            layerIndexById: Pixel.bindRuntime(runtime, function layerIndexById(id) {
                return this.layers.findIndex(layer => layer.id === id);
            }),
            // Cheap; called on every stroke move. Revs key the composite caches and the history snapshots.
            markLayerDirty: Pixel.bindRuntime(runtime, function markLayerDirty(layer = this.activeLayer(), { mask = false } = {}) {
                if (!layer) return;
                if (mask) layer.maskRev = this.nextRev();
                else layer.rev = this.nextRev();
                this.notifyLayersChanged();
            }),
            invalidateCompositeCaches: Pixel.bindRuntime(runtime, function invalidateCompositeCaches() {
                caches.below = caches.above = caches.flat = null;
            }),
            // Write access to the active layer (or its mask in mask-edit mode); marks it dirty.
            getActiveCtx: Pixel.bindRuntime(runtime, function getActiveCtx() {
                const layer = this.activeLayer();
                if (!layer) return null;
                const inMask = !!(this.editMaskMode && layer.maskCanvas);
                // A pending adjust/filter preview would cover whatever is painted, so bake it in first.
                if (!inMask && layer.preview && this.commitPendingPreviews) this.commitPendingPreviews();
                this.markLayerDirty(layer, { mask: inMask });
                return (inMask ? layer.maskCanvas : layer.canvas).getContext('2d');
            }),
            getActiveReadCtx: Pixel.bindRuntime(runtime, function getActiveReadCtx() {
                const layer = this.activeLayer();
                if (!layer) return null;
                return (this.editMaskMode && layer.maskCanvas ? layer.maskCanvas : layer.canvas).getContext('2d');
            }),
            // Gate for tools that paint: sticker layers need rasterizing, locked layers refuse pixels.
            canPaintActiveLayer: Pixel.bindRuntime(runtime, function canPaintActiveLayer(tool) {
                const layer = this.activeLayer();
                if (!layer || !this.requireRasterLayer(layer)) return false;
                const inMask = !!(this.editMaskMode && layer.maskCanvas);
                if (!inMask && (layer.lockPixels || (layer.lockAlpha && tool === 'eraser'))) {
                    this.notify({ type: 'info', message: this.t('pixel.layer_locked', 'This layer is locked.') });
                    return false;
                }
                return true;
            }),

            // --- composition ---
            flattenDocument: Pixel.bindRuntime(runtime, function flattenDocument(target) {
                const w = this.imgWidth || this.canvas.width, h = this.imgHeight || this.canvas.height;
                const out = target || makeCanvas(w, h);
                if (out.width !== w || out.height !== h) { out.width = w; out.height = h; }
                this.renderComposition(out.getContext('2d'), this.layers, { previews: false });
                return out;
            }),
            renderComposition: Pixel.bindRuntime(runtime, function renderComposition(ctx, layers = this.layers, { previews = true } = {}) {
                ctx.setTransform(1, 0, 0, 1, 0, 0);
                ctx.globalAlpha = 1;
                ctx.globalCompositeOperation = 'source-over';
                ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);
                renderRange(ctx, layers, 0, layers.length, previews);
            }),
            compositeLayers: Pixel.bindRuntime(runtime, function compositeLayers() {
                const ctx = this.cctx;
                if (!ctx || !this.canvas || !this.canvas.width || !this.canvas.height) return;
                const layers = this.layers, count = layers.length;
                const w = this.canvas.width, h = this.canvas.height;
                ctx.setTransform(1, 0, 0, 1, 0, 0);
                ctx.globalAlpha = 1;
                ctx.globalCompositeOperation = 'source-over';
                ctx.clearRect(0, 0, w, h);
                const active = Math.max(0, Math.min(this.activeLayerIdx, count - 1));
                let belowCanvas = null, aboveCanvas = null, from = 0, to = count;
                if (active >= 2 && countVisible(layers, 0, active) >= 2) {
                    belowCanvas = ensureCache(caches, 'below', rangeKey(layers, 0, active, w, h, true), w, h, c => renderRange(c, layers, 0, active, true));
                    from = active;
                }
                if (active < count - 2 && !layers[active].clip && aboveIsSimple(layers, active + 1, count) && countVisible(layers, active + 1, count) >= 2) {
                    aboveCanvas = ensureCache(caches, 'above', rangeKey(layers, active + 1, count, w, h, true), w, h, c => renderRange(c, layers, active + 1, count, true));
                    to = active + 1;
                }
                if (!belowCanvas) caches.below = null;
                if (!aboveCanvas) caches.above = null;
                if (belowCanvas) ctx.drawImage(belowCanvas, 0, 0);
                renderRange(ctx, layers, from, to, true);
                if (aboveCanvas) ctx.drawImage(aboveCanvas, 0, 0);
            }),
            // Colour of the composite at a point. The display canvas is used while it shows no preview.
            sampleComposite: Pixel.bindRuntime(runtime, function sampleComposite(x, y) {
                const w = this.canvas.width, h = this.canvas.height;
                if (x < 0 || y < 0 || x >= w || y >= h) return [0, 0, 0, 0];
                if (!this.compareMode && !this.layers.some(layer => layer.preview)) {
                    caches.flat = null;
                    return this.cctx.getImageData(x, y, 1, 1).data;
                }
                const layers = this.layers;
                const flat = ensureCache(caches, 'flat', rangeKey(layers, 0, layers.length, w, h, false), w, h, c => renderRange(c, layers, 0, layers.length, false));
                return flat.getContext('2d').getImageData(x, y, 1, 1).data;
            }),
            // Non-destructive preview: the canvas replaces the layer's pixels on screen until it is committed or cleared.
            // Redrawing the same preview canvas in place needs another call so caches see the change.
            setLayerPreview: Pixel.bindRuntime(runtime, function setLayerPreview(layer, canvas) {
                if (!layer) return;
                layer.preview = canvas || null;
                layer.previewRev = canvas ? this.nextRev() : 0;
                this.compositeLayers();
                this.notifyLayersChanged();
            }),

            // --- history (structural sharing) ---
            pushHistory: Pixel.bindRuntime(runtime, function pushHistory(label) {
                const w = this.canvas.width, h = this.canvas.height;
                if (!w || !h) return;
                const previous = this.history[this.historyIdx] || null;
                const earlier = new Map();
                if (previous && previous.width === w && previous.height === h) {
                    previous.layers.forEach(item => earlier.set(item.meta.id, item));
                }
                const layers = this.layers.map(l => {
                    const old = earlier.get(l.id);
                    let snap = null, maskSnap = null;
                    if (!l.sticker && isDrawable(l.canvas)) {
                        if (old && old.snap && old.snap.rev === l.rev) snap = old.snap;
                        else snap = { id: l.id, rev: l.rev, canvas: copyCanvas(l.canvas) };
                    }
                    if (l.maskCanvas && isDrawable(l.maskCanvas)) {
                        if (old && old.maskSnap && old.maskSnap.maskRev === l.maskRev) maskSnap = old.maskSnap;
                        else maskSnap = { id: l.id, maskRev: l.maskRev, canvas: copyCanvas(l.maskCanvas) };
                    }
                    return { meta: layerMeta(l), snap, maskSnap };
                });
                if (this.historyIdx < this.history.length - 1) this.history = this.history.slice(0, this.historyIdx + 1);
                const active = this.activeLayer();
                this.history.push({
                    label: label || '', width: w, height: h, activeLayerId: active ? active.id : 0,
                    selection: this.selection || null, layers
                });
                while (this.history.length > this.MAX_HISTORY) this.history.shift();
                while (this.history.length > 2 && historyBytes(this.history) > Pixel.HISTORY_BYTE_BUDGET) this.history.shift();
                this.historyIdx = this.history.length - 1;
                this.isDirty = true;
                this.updateStatus();
                if (this.refreshHistoryPanel) this.refreshHistoryPanel();
                this.emit('history-changed');
            }),
            restoreHistory: Pixel.bindRuntime(runtime, function restoreHistory(entry) {
                this.cancelStickerGesture();
                this.resetFilterPreview();
                if (this.compareMode) this.exitCompareMode({ preservePreview: false });
                this.resetAdjustmentControls();
                this.adjustPreview = null;
                const w = entry.width, h = entry.height;
                const sizeChanged = w !== this.canvas.width || h !== this.canvas.height;
                if (sizeChanged) {
                    this.canvas.width = w;
                    this.canvas.height = h;
                    this.imgWidth = w;
                    this.imgHeight = h;
                    this.overlayCanvas.width = w;
                    this.overlayCanvas.height = h;
                } else {
                    this.clearOverlay();
                }
                const current = new Map(this.layers.map(l => [l.id, l]));
                this.layers = entry.layers.map(item => {
                    const meta = item.meta;
                    const old = current.get(meta.id);
                    let canvas;
                    if (!meta.sticker && old && !old.sticker && !sizeChanged && isDrawable(old.canvas) && old.rev === meta.rev) {
                        canvas = old.canvas;
                    } else {
                        canvas = makeCanvas(w, h);
                        if (item.snap) canvas.getContext('2d').drawImage(item.snap.canvas, 0, 0);
                    }
                    let maskCanvas = null;
                    if (meta.hasMask) {
                        if (old && old.maskCanvas && !sizeChanged && old.maskRev === meta.maskRev) {
                            maskCanvas = old.maskCanvas;
                        } else {
                            maskCanvas = makeCanvas(w, h);
                            const mctx = maskCanvas.getContext('2d');
                            if (item.maskSnap) {
                                mctx.drawImage(item.maskSnap.canvas, 0, 0);
                            } else {
                                mctx.fillStyle = '#ffffff';
                                mctx.fillRect(0, 0, w, h);
                            }
                        }
                    }
                    return this.createLayer({
                        id: meta.id, name: meta.name, visible: meta.visible, opacity: meta.opacity, blend: meta.blend,
                        lockPixels: meta.lockPixels, lockAlpha: meta.lockAlpha, lockPosition: meta.lockPosition,
                        maskEnabled: meta.maskEnabled, clip: meta.clip, rev: meta.rev, maskRev: meta.maskRev,
                        canvas, maskCanvas,
                        sticker: meta.sticker ? { id: meta.sticker.id, matrix: meta.sticker.matrix.slice() } : null,
                        stickerImage: meta.stickerImage
                    });
                });
                this.layers.forEach(layer => { if (layer.sticker) this.repaintSticker(layer); });
                let index = this.layers.findIndex(layer => layer.id === entry.activeLayerId);
                if (index < 0) index = Math.max(0, Math.min(this.activeLayerIdx, this.layers.length - 1));
                this.activeLayerIdx = index;
                const layer = this.activeLayer();
                if (this.editMaskMode && !(layer && layer.maskCanvas)) this.editMaskMode = false;
                this.selImageData = null;
                this.moveSelFloating = false;
                this.selection = entry.selection || null;
                if (this.selection) {
                    this.startMarchingAnts();
                } else {
                    this.stopMarchingAnts();
                    this.clearOverlay();
                }
                this.captureAdjustmentSource();
                this.compositeLayers();
                this.refreshLayerPanel();
                this.applyZoom();
                this.syncActiveLayer();
                this.notifyLayersChanged();
                if (sizeChanged) this.emit('document-changed', { width: w, height: h, reason: 'history' });
            }),

            // --- selection ---
            // Document-sized alpha for the current selection (rect/ellipse/lasso rasterised once per selection change).
            getSelectionMask: Pixel.bindRuntime(runtime, function getSelectionMask() {
                const sel = this.selection;
                const w = this.canvas ? this.canvas.width : 0, h = this.canvas ? this.canvas.height : 0;
                if (!sel || !w || !h) return null;
                if (selectionCache && selectionCache.sel === sel && selectionCache.rev === this.selectionRev && selectionCache.w === w && selectionCache.h === h) {
                    return selectionCache.result;
                }
                let result = null;
                if (sel.type === 'mask') {
                    if (sel.alpha && sel.alpha.length === w * h) result = { alpha: sel.alpha, x: sel.x, y: sel.y, w: sel.w, h: sel.h };
                } else {
                    result = rasterizeSelection(sel, w, h);
                }
                selectionCache = { sel, rev: this.selectionRev, w, h, result };
                return result;
            }),
            // mode: replace | add | subtract | intersect. The array is adopted as is; callers must not change it afterwards.
            setSelectionMask: Pixel.bindRuntime(runtime, function setSelectionMask(alpha, options) {
                const w = this.canvas.width, h = this.canvas.height;
                if (!w || !h || !alpha || alpha.length !== w * h) return false;
                const mode = (options && options.mode) || 'replace';
                let out = alpha instanceof Uint8ClampedArray ? alpha : Uint8ClampedArray.from(alpha);
                if (mode === 'add' || mode === 'subtract' || mode === 'intersect') {
                    const current = this.getSelectionMask();
                    if (current) out = combineAlpha(current.alpha, out, mode);
                    else if (mode !== 'add') out = new Uint8ClampedArray(w * h);
                }
                const bounds = maskBounds(out, w, h);
                if (!bounds) {
                    this.deselect();
                    return false;
                }
                this.selection = { type: 'mask', x: bounds.x, y: bounds.y, w: bounds.w, h: bounds.h, alpha: out, border: maskBorder(out, w, h, bounds) };
                this.startMarchingAnts();
                return true;
            }),
            invertSelection: Pixel.bindRuntime(runtime, function invertSelection() {
                const current = this.getSelectionMask();
                if (!current) return false;
                const inverted = new Uint8ClampedArray(current.alpha.length);
                for (let i = 0; i < inverted.length; i++) inverted[i] = 255 - current.alpha[i];
                return this.setSelectionMask(inverted, { mode: 'replace' });
            }),

            // --- committing pixels ---
            // source: document-sized canvas or ImageData. opts: { label, respectSelection = true, mask = null, history = true };
            // mask is an extra document-sized alpha (Uint8ClampedArray or { alpha }) multiplied with the selection.
            commitLayerPixels: Pixel.bindRuntime(runtime, function commitLayerPixels(layer, source, opts) {
                const o = opts || {};
                layer = layer || this.activeLayer();
                if (!layer || !source || !this.canvas.width) return false;
                if (!this.requireRasterLayer(layer)) return false;
                const inMask = !!(this.editMaskMode && layer === this.activeLayer() && layer.maskCanvas);
                if (layer.lockPixels && !inMask) {
                    this.notify({ type: 'info', message: this.t('pixel.layer_locked', 'This layer is locked.') });
                    return false;
                }
                const w = this.canvas.width, h = this.canvas.height;
                const target = inMask ? layer.maskCanvas : layer.canvas;
                const ctx = target.getContext('2d');
                let src = source, ownSource = null;
                if (typeof ImageData !== 'undefined' && source instanceof ImageData) {
                    ownSource = Pixel.acquireTempCanvas(source.width, source.height);
                    ownSource.getContext('2d').putImageData(source, 0, 0);
                    src = ownSource;
                }
                const selection = o.respectSelection === false ? null : this.getSelectionMask();
                const extra = o.mask ? (o.mask.alpha || o.mask) : null;
                if (o.respectSelection !== false && this.selection && !selection) {
                    if (ownSource) Pixel.releaseTempCanvas(ownSource);
                    return false;
                }
                let alpha = null, box = { x: 0, y: 0, w, h };
                if (selection && extra) {
                    alpha = new Uint8ClampedArray(w * h);
                    for (let y = selection.y; y < selection.y + selection.h; y++) {
                        for (let p = y * w + selection.x, end = p + selection.w; p < end; p++) alpha[p] = selection.alpha[p] * extra[p] / 255;
                    }
                    box = { x: selection.x, y: selection.y, w: selection.w, h: selection.h };
                } else if (selection) {
                    alpha = selection.alpha;
                    box = { x: selection.x, y: selection.y, w: selection.w, h: selection.h };
                } else if (extra) {
                    alpha = extra;
                    box = maskBounds(extra, w, h);
                    if (!box) {
                        if (ownSource) Pixel.releaseTempCanvas(ownSource);
                        return false;
                    }
                }
                const opaque = !alpha || (selection && !extra && this.selection.type === 'rect');
                if (layer.lockAlpha && !inMask) commitKeepingAlpha(ctx, src, opaque ? null : alpha, w, box);
                else if (opaque) replaceRegion(ctx, src, box);
                else blendThroughAlpha(ctx, src, alpha, w, box);
                if (ownSource) Pixel.releaseTempCanvas(ownSource);
                this.markLayerDirty(layer, { mask: inMask });
                this.compositeLayers();
                if (o.history !== false) this.pushHistory(o.label || 'edit');
                return true;
            })
        });

        // Resizing the overlay canvas clears it, and the outline of a selection changes with it: repaint.
        runtime.on('document-changed', () => runtime.requestOverlay());
        runtime.on('selection-changed', () => runtime.requestOverlay());
        // Menu items may depend on the selection, history or layers (enabled/checked); rebuild only when one does.
        const menuEvents = ['selection-changed', 'history-changed', 'active-layer-changed', 'document-changed'];
        menuEvents.forEach(event => runtime.on(event, () => { if (runtime.menusDynamic) runtime.refreshMenus(); }));
    };
})();
