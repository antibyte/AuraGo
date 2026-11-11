// Pixel module "paint": the Art brush tool. It drives the DOM-free paint engines of pixel-lib-paint.js / pixel-lib-wet.js
// (oil, acrylic, gouache, knife, blender, charcoal, pencil, pastel, crayon, marker, watercolour, ink) on the stroke session of
// pixel-brush.js. Dry engines work on a cached pixel region (one getImageData per region, one putImageData per flush); the wet
// engines deposit into a wet layer that a 33 ms timer lets flow and dry, rendered over a copy of the layer. The Atelier panel
// lives in pixel-paint-ui.js. Pure helpers and the session logic are exposed as Pixel.art.
(function () {
    'use strict';
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};
    const Brush = Pixel.brush, toolUi = Pixel.toolUi;
    const lib = window.AuraPixelLib || null, paint = lib && lib.paint;
    if (!Brush || !toolUi || !paint || typeof paint.createWet !== 'function') {
        if (window.console) console.error('pixel-paint.js needs pixel-brush.js, pixel-lib-paint.js and pixel-lib-wet.js');
        return;
    }
    const { F, opt } = toolUi;
    const Art = Pixel.art = {};
    const MEDIA = paint.MEDIA;
    const now = () => (typeof performance !== 'undefined' && performance.now ? performance.now() : Date.now());
    const mediumOf = id => MEDIA.find(m => m.id === id) || MEDIA[0];
    Art.mediumOf = mediumOf;
    const engineOf = values => mediumOf(values && values.medium).engine;
    Art.engineOf = engineOf;

    const svg = body => '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">' + body + '</svg>';
    const pct = (key, labelKey, fallback, extra) => F(key, 'range', labelKey, fallback, Object.assign({ min: 0, max: 100, step: 1, unit: '%', default: 50 }, extra));

    // --- tool definition ---

    const schema = [
        F('medium', 'select', 'pixel.art_medium', 'Medium', { default: 'oil', options: MEDIA.map(m => opt(m.id, m.labelKey, m.id)) }),
        Object.assign({}, Brush.FIELDS.size, { default: mediumOf('oil').defaults.size }),
        pct('load', 'pixel.art_load', 'Paint load', { default: 80 }),
        pct('mix', 'pixel.art_mix', 'Mixing', { default: 60, showIf: v => engineOf(v) === 'bristle' }),
        pct('wetness', 'pixel.art_wetness', 'Water', { default: 0, showIf: v => engineOf(v) === 'wet' }),
        pct('grain', 'pixel.art_grain', 'Paper grain', { default: 50 }),
        Object.assign({}, Brush.FIELDS.flow, { default: 100 }),
        Object.assign({}, Brush.FIELDS.smoothing, { default: 15 }),
        Object.assign({}, Brush.FIELDS.pressure, { default: 'size' }),
        F('keepDirty', 'bool', 'pixel.art_keep_dirty', 'Do not wash the brush', { default: false, showIf: v => engineOf(v) === 'bristle' })
    ];
    const cfg = {
        id: 'art-brush', mode: 'effect', panel: true, pickColor: true,
        meta: { group: 'paint', order: 35, labelKey: 'pixel.art_brush', fallback: 'Art brush',
            svg: svg('<path d="M13.5 2.5c-3 .5-5.5 3-6.5 6l1.5 1.5c3-1 5.5-3.5 6-6.5z"/><path d="M7 8.5c-2 0-3 1.5-3 3 0 1-.5 1.7-1.5 2 2 .5 5-.3 5-3"/><path d="M12.8 12.2c.6.8 1.2 1.5 1.2 2.1a1.2 1.2 0 01-2.4 0c0-.6.6-1.3 1.2-2.1z"/>') },
        schema,
        defaults: {
            medium: 'oil', size: mediumOf('oil').defaults.size, load: 80, mix: 60, wetness: 0, grain: 50, flow: 100, smoothing: 15, pressure: 'size',
            keepDirty: false, angle: 0, angleFollow: true, spacing: 4, symmetry: 'off', scatter: 0, count: 1, sizeJitter: 0, angleJitter: 0, opacityJitter: 0
        },
        bar: ['medium', 'size', 'load', 'mix', 'wetness', 'grain']
    };
    Art.cfg = cfg;
    const settings = r => toolUi.settings(r, cfg);

    // --- brush and region cache ---

    const rgbOf = hex => { const c = Pixel.hexToRgb(/^#[0-9a-f]{6}$/i.test(hex) ? hex : '#000000'); return [c.r, c.g, c.b]; };
    const paperOf = r => paint.makePaper(r.artPaper.id);
    const configOf = (r, s, color) => ({
        load: s.load / 100, mix: s.mix / 100, wetness: s.wetness / 100, grain: s.grain / 100,
        paper: paperOf(r), paperScale: r.artPaper.scale, color: color || rgbOf(r.primaryColor)
    });
    Art.ensureBrush = function ensureBrush(r, s, w, h, color) {
        const key = s.medium + '|' + r.artPaper.id + '|' + w + 'x' + h, held = r.artBrush;
        const c = configOf(r, s, color);
        if (!held || held.key !== key) {
            r.artBrush = { key, brush: paint.createBrush(s.medium, Object.assign({ size: s.size, seed: (Math.random() * 0x7FFFFFFF) | 0, docW: w, docH: h }, c)) };
        } else held.brush.configure(c);
        return r.artBrush.brush;
    };
    // The shared path code reads s.spacing lazily, in percent of the size.
    Art.syncSpacing = function syncSpacing(r, s) {
        const held = r.artBrush;
        if (held && held.key.indexOf(s.medium + '|') === 0 && s.size > 0) s.spacing = Math.max(0.1, held.brush.spacing(s.size) / s.size * 100);
    };

    function ensureWetMap(r, layer, w, h) {
        const m = r.artWetMap;
        if (!m || m.layerId !== layer.id || m.w !== w || m.h !== h) r.artWetMap = { layerId: layer.id, w, h, map: new Uint8Array(w * h) };
        return r.artWetMap;
    }

    Art.commitRegion = function commitRegion(st) {
        const art = st.art, g = art && art.region;
        if (!g || !g.dirty) return false;
        st.S.write(g.rect, g.old, g.next);
        g.old.data.set(g.next.data);
        g.dirty = false;
        return true;
    };

    // The stroke session flushes through S.flush; the last region must be written before the history entry is made.
    Art.attachFlush = function attachFlush(st, renderFirst, after) {
        const flush0 = st.S.flush;
        st.S.flush = () => {
            if (renderFirst) renderFirst(); else Art.commitRegion(st);
            const out = flush0();
            if (after) after();
            return out;
        };
    };

    function stampDry(st, x, y, size, alpha, rot, pr) {
        const S = st.S, art = st.art, brush = art.brush, reach = brush.reach(size);
        const x0 = Math.max(0, Math.floor(x - reach)), y0 = Math.max(0, Math.floor(y - reach));
        const x1 = Math.min(S.w, Math.ceil(x + reach)), y1 = Math.min(S.h, Math.ceil(y + reach));
        if (x1 <= x0 || y1 <= y0) return;
        let g = art.region;
        if (!g || x0 < g.rect.x || y0 < g.rect.y || x1 > g.rect.x + g.rect.w || y1 > g.rect.y + g.rect.h) {
            Art.commitRegion(st);
            const grow = Math.max(96, reach);
            const rx0 = Math.max(0, Math.floor(x - reach - grow)), ry0 = Math.max(0, Math.floor(y - reach - grow));
            const rx1 = Math.min(S.w, Math.ceil(x + reach + grow)), ry1 = Math.min(S.h, Math.ceil(y + reach + grow));
            const rect = { x: rx0, y: ry0, w: rx1 - rx0, h: ry1 - ry0 };
            const old = S.read(rect), next = Brush.cloneImage(old), map = st.r.artWetMap;
            g = art.region = {
                rect, old, next, dirty: false,
                buf: { x: rect.x, y: rect.y, w: rect.w, h: rect.h, data: next.data, sel: null, docW: S.w, docH: S.h, lockAlpha: S.lockAlpha, wetMap: S.inMask || !map ? undefined : map.map }
            };
        }
        if (brush.dab(g.buf, x, y, size, pr, rot, alpha)) g.dirty = true;
    }

    function stampWet(st, x, y, size, alpha, rot, pr) {
        const ws = st.r.artWet;
        if (!ws) return;
        const d = st.art.brush.dab(ws.wet, x, y, size, pr, rot, alpha);
        if (d) st.S.mark(d.x0, d.y0, d.x1, d.y1);
    }
    cfg.stamp = function stamp(st, x, y, size, alpha, rot, pr) {
        if (!st.art) return;
        (st.art.wet ? stampWet : stampDry)(st, x, y, size, alpha, rot, pr);
    };

    // --- wet manager ---

    function dropWet(r) {
        const ws = r.artWet;
        if (!ws) return;
        if (ws.timer) clearInterval(ws.timer);
        ws.timer = 0;
        ws.wet.bake();
        r.artWet = null;
    }
    Art.dropWet = dropWet;

    // The layer revision the wet session last produced: a different one means someone else edited the layer.
    function touchLayer(r, ws, layer) {
        r.markLayerDirty(layer);
        ws.rev = layer.rev;
    }
    function foreignEdit(r, ws, layer) {
        return !layer || !layer.canvas || (!r.strokeState && layer.rev !== ws.rev);
    }

    // Renders the changed part of the wet layer (or, with whole, every un-baked paint area) over the dry base picture.
    function renderWet(r, whole) {
        const ws = r.artWet;
        if (!ws) return false;
        const layer = r.layerById(ws.layerId);
        if (foreignEdit(r, ws, layer)) { dropWet(r); return false; } // the foreign edit already holds the pixels
        const rect = whole ? ws.wet.paintRect() : ws.wet.dirtyRect();
        if (!rect) return false;
        const baseImg = ws.base.getContext('2d').getImageData(rect.x, rect.y, rect.w, rect.h);
        const out = new ImageData(new Uint8ClampedArray(rect.w * rect.h * 4), rect.w, rect.h), mask = r.selection ? r.getSelectionMask() : null;
        ws.wet.render(
            { x: rect.x, y: rect.y, w: rect.w, h: rect.h, data: out.data, sel: mask ? mask.alpha : null, docW: ws.docW, docH: ws.docH, lockAlpha: !!layer.lockAlpha },
            { x: rect.x, y: rect.y, w: rect.w, h: rect.h, data: baseImg.data });
        layer.canvas.getContext('2d').putImageData(out, rect.x, rect.y);
        ws.changed = true;
        return true;
    }
    Art.renderWet = renderWet;

    function tick(r, ws) {
        if (r.artWet !== ws) { clearInterval(ws.timer); return; }
        if (r.state && r.state.disposed) { dropWet(r); return; }
        if (ws.skip) { ws.skip = false; return; }
        const t0 = now(), layer = r.layerById(ws.layerId);
        if (foreignEdit(r, ws, layer)) { dropWet(r); return; }
        const still = ws.wet.step();
        if (renderWet(r)) {
            touchLayer(r, ws, layer);
            if (!r.strokeState) r.compositeLayers();
        }
        if (!still && !r.strokeState) finishWet(r);
        if (now() - t0 > 24) ws.skip = true; // never queue up when a tick was slow
    }

    // fast: simulate on for at most FAST_MS (Dry now, a new stroke over wet paint); what is still wet stays where it is.
    const FAST_MS = 40;
    function finishWet(r, o) {
        const ws = r.artWet;
        if (!ws) return false;
        if (o && o.fast) {
            const t0 = now();
            for (let i = 0; i < 600 && now() - t0 < FAST_MS; i++) if (!ws.wet.step()) break;
        }
        renderWet(r);
        if (r.artWet !== ws) return true; // a foreign edit ended the session
        const layer = r.layerById(ws.layerId), changed = ws.changed;
        dropWet(r);
        if (changed && layer) {
            touchLayer(r, ws, layer);
            r.compositeLayers();
            r.pushHistory('draw:' + cfg.id);
        }
        return true;
    }
    Art.finishWet = finishWet;

    function wetMismatch(ws, layer, w, h, paper, scale) {
        return ws.layerId !== layer.id || ws.docW !== w || ws.docH !== h || ws.paperId !== paper.id || ws.scale !== scale;
    }
    // Must the running wet session end before a stroke of this medium on this layer starts?
    Art.wetMustEnd = function wetMustEnd(r, s, layer) {
        const ws = r.artWet;
        if (!ws || !layer || !layer.canvas) return false;
        if (mediumOf(s.medium).engine !== 'wet') return true;
        return wetMismatch(ws, layer, layer.canvas.width, layer.canvas.height, paperOf(r), r.artPaper.scale);
    };
    Art.endWetBeforeStroke = function endWetBeforeStroke(r) {
        const layer = r.activeLayer();
        if (!r.artWet || !Art.wetMustEnd(r, settings(r), layer)) return;
        if (r.artWet.wet.hasPaint()) finishWet(r, { fast: true }); else dropWet(r);
    };

    function startWet(r, layer, s, S) {
        const paper = paperOf(r), scale = r.artPaper.scale;
        let ws = r.artWet;
        if (ws && wetMismatch(ws, layer, S.w, S.h, paper, scale)) {
            finishWet(r, { fast: true }); // safety net: onDown normally did this before the session opened
            ws = null;
        }
        if (!ws) {
            ws = r.artWet = {
                layerId: layer.id, docW: S.w, docH: S.h, paperId: paper.id, scale, changed: false, skip: false, timer: 0, rev: layer.rev,
                wet: paint.createWet({ docW: S.w, docH: S.h, paper, paperScale: scale }), base: r.copyCanvas(layer.canvas)
            };
            ws.timer = setInterval(() => tick(r, ws), 33);
        } else ws.rev = layer.rev; // opening the stroke session touched the layer itself
        return ws;
    }

    // Dry now: finish the wet session at once and forget which oil paint is still wet.
    Art.dryNow = function dryNow(r) {
        if (r.strokeState) return; // never in the middle of a stroke
        finishWet(r, { fast: true });
        if (r.artWetMap) r.artWetMap.map.fill(0);
        r.artWetMapUsed = false;
        if (r.artPanelRefresh) r.artPanelRefresh();
    };

    // --- stroke hooks ---

    cfg.begin = function begin(r, st) {
        const s = st.s, S = st.S, layer = r.activeLayer(), m = mediumOf(s.medium);
        if (!layer) return false;
        if (m.engine === 'wet') {
            if (S.inMask) {
                if (typeof r.notify === 'function') r.notify({ type: 'info', message: r.t('pixel.art_no_wet_on_mask') });
                return false;
            }
            startWet(r, layer, s, S);
        } else {
            if (r.artWet) { if (r.artWet.wet.hasPaint()) finishWet(r, { fast: true }); else dropWet(r); } // safety net, see onDown
            if (!S.inMask) ensureWetMap(r, layer, S.w, S.h);
        }
        const brush = Art.ensureBrush(r, s, S.w, S.h, rgbOf(st.color));
        brush.begin(!!s.keepDirty);
        Art.syncSpacing(r, s);
        st.art = { brush, region: null, wet: m.engine === 'wet' };
        const ws = st.art.wet ? r.artWet : null;
        Art.attachFlush(st, ws ? () => renderWet(r) : null, ws ? () => { const l = r.layerById(ws.layerId); if (l && r.artWet === ws) ws.rev = l.rev; } : null);
        return true;
    };
    cfg.done = function done(r, st) {
        const art = st.art;
        if (art && art.wet && r.artWet) r.artWet.changed = false; // the history entry of the stroke holds this picture
        if (art && !art.wet && mediumOf(st.s.medium).engine === 'bristle') r.artWetMapUsed = true;
        if (r.artPanelRefresh) r.artPanelRefresh();
    };
    // A wet stroke cannot be cancelled half-way: the pigment has already flowed past the stamped box. The session cancel restored
    // that box, so the whole wet picture is drawn again and the drying history entry records it.
    cfg.abort = function abort(r, st) {
        const ws = r.artWet;
        if (!st.art || !st.art.wet || !ws) return;
        const layer = r.layerById(ws.layerId);
        if (!layer || !layer.canvas) { dropWet(r); return; }
        ws.rev = layer.rev; // the cancel marked the layer itself
        if (renderWet(r, true)) {
            touchLayer(r, ws, layer);
            r.compositeLayers();
        }
        ws.changed = true;
    };

    // --- settings changes (bar, panel, commands) ---

    // Choosing a medium copies its defaults; other sliders never touch the medium.
    function applyMediumDefaults(s, id) {
        const d = mediumOf(id).defaults;
        Object.assign(s, { medium: id, size: d.size, load: d.load, mix: d.mix, wetness: d.wetness, grain: d.grain });
    }
    Art.applyMediumDefaults = applyMediumDefaults;
    Art.setMedium = function setMedium(r, id, source) {
        applyMediumDefaults(settings(r), id);
        toolUi.changed(r, cfg, 'medium', id, source);
    };
    // The medium select is part of the bar: its defaults change other bar fields, so the bar is refreshed after a bar change too.
    cfg.onChangeArt = function onChangeArt(r, key, value, s) {
        if (key === 'medium') applyMediumDefaults(s, value);
        const held = r.artBrush;
        if (held && held.key.indexOf(s.medium + '|') === 0) held.brush.configure(configOf(r, s));
        Art.syncSpacing(r, s);
        if (key === 'medium') {
            const h = (r.toolUiHandles || {})[cfg.id];
            if (h && h.refresh) h.refresh();
        }
        if (r.artPanelRefresh) r.artPanelRefresh(key);
    };

    cfg.tail = r => '<button type="button" class="pixel-btn pixel-opt-more" data-art-panel-open>' + r.esc(r.t('pixel.panel_art')) + '</button>';
    cfg.wireExtra = (r, bar) => {
        const more = bar.querySelector('[data-art-panel-open]');
        if (more) more.addEventListener('click', () => r.showPanel('art'));
    };
    Brush.defineStrokeTool(cfg);
    // defineStrokeTool installs its own onChange (runtime size sync); chain ours behind it.
    const defined = cfg.onChange;
    // A running wet session must end before a stroke session opens (its `before` snapshot is taken in onDown).
    const toolDef = Pixel.getTool(cfg.id), down0 = toolDef && toolDef.onDown;
    if (toolDef && down0) {
        toolDef.onDown = function onDown(r, e, p) {
            if (!e.button && !(p && p.alt)) Art.endWetBeforeStroke(r);
            return down0.call(this, r, e, p);
        };
    }
    cfg.onChange = (r, key, value, s) => { defined(r, key, value, s); cfg.onChangeArt(r, key, value, s); };

    // --- paper layer ---

    // Alpha 255 at the four corners and the centre: the layer hides everything below it.
    Art.layerIsOpaque = function layerIsOpaque(layer) {
        if (!layer || !layer.canvas) return false;
        const w = layer.canvas.width, h = layer.canvas.height;
        if (!w || !h) return false;
        try {
            const ctx = layer.canvas.getContext('2d');
            return [[0, 0], [w - 1, 0], [0, h - 1], [w - 1, h - 1], [w >> 1, h >> 1]].every(pt => ctx.getImageData(pt[0], pt[1], 1, 1).data[3] === 255);
        } catch (_) { return false; }
    };
    // Opaque active layer: paper above it plus a fresh painting layer on top (active). Otherwise: paper directly below it.
    Art.paperPlacement = function paperPlacement(index, opaque) {
        return opaque ? { paper: index + 1, paint: index + 2, active: 'paint' } : { paper: index, paint: -1, active: 'keep' };
    };
    Art.addPaperLayer = function addPaperLayer(r) {
        const w = r.imgWidth, h = r.imgHeight;
        if (!w || !h || !r.layers.length) return false;
        const active = r.activeLayer(), opaque = Art.layerIsOpaque(active);
        if (r.layers.length + (opaque ? 2 : 1) > r.MAX_LAYERS) { r.notify({ type: 'error', message: r.t('pixel.layer_max', { n: r.MAX_LAYERS }) }); return false; }
        if (r.commitPendingPreviews) r.commitPendingPreviews();
        const name = r.t('pixel.art_paper_layer');
        const data = paint.renderPaper(paperOf(r), w, h, { scale: r.artPaper.scale, relief: 0.6 });
        const layer = r.createLayer({ name });
        layer.canvas.getContext('2d').putImageData(new ImageData(data, w, h), 0, 0);
        r.markLayerDirty(layer);
        const place = Art.paperPlacement(Math.max(0, r.layers.indexOf(active)), opaque);
        r.layers.splice(place.paper, 0, layer);
        if (place.paint >= 0) {
            const fresh = r.createLayer({ name: r.t('pixel.art_paint_layer') });
            r.layers.splice(place.paint, 0, fresh);
            r.activeLayerIdx = r.layers.indexOf(fresh);
        } else r.activeLayerIdx = Math.max(0, r.layers.indexOf(active));
        r.editMaskMode = false;
        r.layersStructureChanged();
        r.pushHistory(name);
        return true;
    };

    // --- commands, menu ---

    const hasDoc = r => !!(r.imgWidth && r.layers && r.layers.length);
    Pixel.registerCommand({ id: 'art.dry-now', labelKey: 'pixel.cmd_art_dry_now', fallback: 'Dry now', group: 'paint',
        keywords: ['dry', 'wet', 'watercolor', 'oil', 'trocknen', 'nass', 'aquarell'], run: r => Art.dryNow(r), enabled: r => !r.strokeState && !!(r.artWet || r.artWetMap) });
    Pixel.registerCommand({ id: 'art.clean-brush', labelKey: 'pixel.cmd_art_clean_brush', fallback: 'Wash brush', group: 'paint',
        keywords: ['clean', 'wash', 'brush', 'pinsel', 'auswaschen', 'reinigen'],
        run: r => { r.artBrush = null; if (r.artPanelRefresh) r.artPanelRefresh(); }, enabled: hasDoc });
    Pixel.registerCommand({ id: 'art.paper-layer', labelKey: 'pixel.cmd_art_paper_layer', fallback: 'Add paper layer', group: 'paint',
        keywords: ['paper', 'canvas', 'texture', 'papier', 'leinwand', 'untergrund'], run: r => Art.addPaperLayer(r), enabled: hasDoc });
    Pixel.registerMenuItems('layer', [
        { id: 'art-paper-layer', labelKey: 'pixel.cmd_art_paper_layer', fallback: 'Add paper layer', label: 'Add paper layer',
            keywords: ['paper', 'canvas', 'papier', 'leinwand'], action: r => Art.addPaperLayer(r), enabled: hasDoc }
    ], { order: 1 });

    // --- per-window state ---

    Pixel.registerModule('paint', function install(r) {
        Object.assign(r, {
            artPaper: { id: 'cold_press', scale: 1 }, artBrush: null, artWet: null, artPalette: null,
            artWetMap: null, artWetMapUsed: false, artPanelRefresh: null
        });
        r.on('tool-changed', e => { if (!e || e.tool !== cfg.id) finishWet(r, { fast: true }); });
        r.on('active-layer-changed', () => finishWet(r, { fast: true }));
        r.on('document-changed', () => { dropWet(r); r.artWetMap = null; r.artBrush = null; });
        // Undo / redo / history jumps show the snapshot: the wet state is dropped without a render and without history.
        const restore0 = r.restoreHistory;
        if (typeof restore0 === 'function') r.restoreHistory = function restoreHistory() { dropWet(r); return restore0.apply(this, arguments); };
        const dispose0 = r.disposeCore;
        if (typeof dispose0 === 'function') r.disposeCore = function disposeCore() { dropWet(r); return dispose0.apply(this, arguments); };
    });
})();
