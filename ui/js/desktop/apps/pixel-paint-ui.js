// Pixel panel "Atelier" (inspector tab) for the Art brush: medium grid with live stroke previews, the tool settings, paper,
// brush state (carried colour, wet / dry) and a mixing palette that uses the same paint engine. Logic of the tool itself is in
// pixel-paint.js (Pixel.art); this file only builds and wires the panel.
(function () {
    'use strict';
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};
    const Art = Pixel.art, toolUi = Pixel.toolUi;
    const lib = window.AuraPixelLib || null, paint = lib && lib.paint;
    if (!Art || !toolUi || !paint) {
        if (window.console) console.error('pixel-paint-ui.js needs pixel-paint.js');
        return;
    }
    const cfg = Art.cfg;
    const PW = 240, PH = 130;          // mixing palette backing size
    const PREVIEW_W = 88, PREVIEW_H = 44;
    // A pleasant colour per medium for when the primary colour is too light to show on paper.
    const MEDIUM_COLOR = {
        oil: [176, 52, 42], acrylic: [38, 96, 170], gouache: [200, 110, 40], knife: [60, 120, 90], blender: [150, 90, 130],
        charcoal: [38, 38, 42], pencil: [58, 58, 64], pastel: [190, 80, 120], crayon: [220, 90, 40], marker: [30, 130, 120],
        watercolor: [40, 100, 180], ink: [24, 24, 40]
    };
    const previewCache = Pixel.artPreviewCache || (Pixel.artPreviewCache = new Map());
    const settings = r => toolUi.settings(r, cfg);

    function previewColor(r, id) {
        const c = Pixel.hexToRgb(/^#[0-9a-f]{6}$/i.test(r.primaryColor) ? r.primaryColor : '#000000');
        return (c.r * 0.299 + c.g * 0.587 + c.b * 0.114) > 205 ? MEDIUM_COLOR[id] : [c.r, c.g, c.b];
    }
    function previewData(r, id) {
        const color = previewColor(r, id), key = id + '|' + r.artPaper.id + '|' + color.join(',');
        let data = previewCache.get(key);
        if (!data) {
            data = paint.previewStroke(id, PREVIEW_W, PREVIEW_H, { color, paper: paint.makePaper(r.artPaper.id) });
            previewCache.set(key, data);
            if (previewCache.size > 120) previewCache.delete(previewCache.keys().next().value);
        }
        return data;
    }

    // --- markup ---

    const label = (r, key) => r.esc(r.t(key));
    function buildPanel(r) {
        const s = settings(r);
        const media = paint.MEDIA.map(m => '<button type="button" class="pixel-art-medium" data-art-medium="' + m.id + '" aria-pressed="' + (m.id === s.medium ? 'true' : 'false') + '" title="' + label(r, m.labelKey) + '">' +
            '<canvas class="pixel-art-preview" width="' + PREVIEW_W + '" height="' + PREVIEW_H + '" aria-hidden="true"></canvas><span class="pixel-art-medium-name">' + label(r, m.labelKey) + '</span></button>').join('');
        const papers = paint.PAPERS.map(p => '<option value="' + p.id + '"' + (p.id === r.artPaper.id ? ' selected' : '') + '>' + label(r, p.labelKey) + '</option>').join('');
        const id = 'pixel-' + r.windowId + '-art';
        return '<div class="pixel-art-panel" data-art-panel>' +
            '<section class="pixel-art-sec"><h4 class="pixel-art-head">' + label(r, 'pixel.art_sec_medium') + '</h4><div class="pixel-art-media" role="group" aria-label="' + label(r, 'pixel.art_sec_medium') + '">' + media + '</div></section>' +
            '<section class="pixel-art-sec"><h4 class="pixel-art-head">' + label(r, 'pixel.art_sec_settings') + '</h4><div data-art-settings></div></section>' +
            '<section class="pixel-art-sec"><h4 class="pixel-art-head">' + label(r, 'pixel.art_sec_paper') + '</h4>' +
            '<label class="pixel-art-field" for="' + id + '-paper"><span class="pixel-opt-label">' + label(r, 'pixel.art_paper') + '</span><select class="pixel-select" id="' + id + '-paper" data-art-paper>' + papers + '</select></label>' +
            '<label class="pixel-art-field" for="' + id + '-scale"><span class="pixel-opt-label">' + label(r, 'pixel.art_grain_size') + '</span><input type="range" class="pixel-slider" id="' + id + '-scale" data-art-scale min="0.5" max="4" step="0.25" value="' + r.artPaper.scale + '"></label>' +
            '<canvas class="pixel-art-swatch" data-art-swatch width="240" height="48" role="img" aria-label="' + label(r, 'pixel.art_sec_paper') + '"></canvas>' +
            '<button type="button" class="pixel-btn" data-art-add-paper>' + label(r, 'pixel.art_add_paper') + '</button></section>' +
            '<section class="pixel-art-sec"><h4 class="pixel-art-head">' + label(r, 'pixel.art_sec_brush') + '</h4>' +
            '<div class="pixel-art-brushrow"><span class="pixel-art-carried" data-art-carried role="img" aria-label="' + label(r, 'pixel.art_carried') + '"></span>' +
            '<button type="button" class="pixel-btn" data-art-wash>' + label(r, 'pixel.art_wash') + '</button></div>' +
            '<div class="pixel-art-brushrow"><span class="pixel-art-state" data-art-state role="status"></span><button type="button" class="pixel-btn" data-art-dry>' + label(r, 'pixel.art_dry_now') + '</button></div></section>' +
            '<section class="pixel-art-sec"><h4 class="pixel-art-head">' + label(r, 'pixel.art_sec_palette') + '</h4>' +
            '<canvas class="pixel-art-palette" data-art-palette width="' + PW + '" height="' + PH + '" role="img" aria-label="' + label(r, 'pixel.art_sec_palette') + '"></canvas>' +
            '<div class="pixel-art-brushrow"><button type="button" class="pixel-btn" data-art-pick aria-pressed="false">' + label(r, 'pixel.art_pick') + '</button>' +
            '<button type="button" class="pixel-btn" data-art-clear>' + label(r, 'pixel.art_clear') + '</button></div>' +
            '<p class="pixel-draw-hint">' + label(r, 'pixel.art_palette_hint') + '</p></section></div>';
    }

    // --- previews (one per timeout slice, cached) ---

    function schedulePreviews(ui, r) {
        ui.token = (ui.token || 0) + 1;
        const token = ui.token, queue = Array.from(ui.el.querySelectorAll('[data-art-medium]'));
        const next = () => {
            if (ui.token !== token || r.state.disposed || !queue.length) return;
            const btn = queue.shift(), canvas = btn.querySelector('canvas'), id = btn.dataset.artMedium;
            if (canvas && id) {
                try { canvas.getContext('2d').putImageData(new ImageData(previewData(r, id), PREVIEW_W, PREVIEW_H), 0, 0); } catch (err) { if (window.console) console.error('pixel art preview', err); }
            }
            setTimeout(next, 0);
        };
        setTimeout(next, 0);
    }

    // --- sections ---

    function renderSettings(ui, r) {
        const host = ui.el.querySelector('[data-art-settings]'), s = settings(r);
        host.innerHTML = r.buildParamControls(cfg.schema, s, { idPrefix: 'pixel-' + r.windowId + '-p-' + cfg.id });
        const root = host.querySelector('[data-param-root]');
        ui.handle = root ? r.wireParamControls(root, cfg.schema, s, (key, value) => {
            ui.busy = true;
            try { toolUi.changed(r, cfg, key, value, 'panel'); } finally { ui.busy = false; }
        }) : null;
    }
    function paintPaperSwatch(ui, r) {
        const canvas = ui.el.querySelector('[data-art-swatch]');
        if (!canvas) return;
        const data = paint.renderPaper(paint.makePaper(r.artPaper.id), 240, 48, { scale: r.artPaper.scale, relief: 0.6 });
        canvas.getContext('2d').putImageData(new ImageData(data, 240, 48), 0, 0);
    }
    function refreshBrush(ui, r) {
        const held = r.artBrush, carried = held ? held.brush.carried() : null;
        const sw = ui.el.querySelector('[data-art-carried]');
        if (sw) sw.style.background = carried ? 'rgb(' + carried.join(',') + ')' : r.primaryColor;
        const wet = !!(r.artWet || r.artWetMapUsed);
        const state = ui.el.querySelector('[data-art-state]');
        if (state) state.textContent = r.t(wet ? 'pixel.art_state_wet' : 'pixel.art_state_dry');
        const dry = ui.el.querySelector('[data-art-dry]');
        if (dry) dry.disabled = !wet || !!r.strokeState;
    }
    function refreshMedia(ui, r) {
        const id = settings(r).medium;
        ui.el.querySelectorAll('[data-art-medium]').forEach(b => b.setAttribute('aria-pressed', b.dataset.artMedium === id ? 'true' : 'false'));
    }

    // --- mixing palette ---

    function paletteState(r) {
        if (!r.artPalette || r.artPalette.data.length !== PW * PH * 4) {
            const data = new Uint8ClampedArray(PW * PH * 4).fill(255);
            r.artPalette = { data, map: new Uint8Array(PW * PH), brush: null };
        }
        return r.artPalette;
    }
    function paletteBrush(r, pal) {
        const c = Pixel.hexToRgb(/^#[0-9a-f]{6}$/i.test(r.primaryColor) ? r.primaryColor : '#000000');
        if (!pal.brush) {
            pal.brush = paint.createBrush('oil', { size: 26, load: 0.9, mix: 0.9, grain: 0, paper: paint.makePaper('smooth'), color: [c.r, c.g, c.b], docW: PW, docH: PH });
        } else pal.brush.configure({ color: [c.r, c.g, c.b] });
        return pal.brush;
    }
    function drawPalette(ui) {
        const canvas = ui.el.querySelector('[data-art-palette]');
        if (canvas) canvas.getContext('2d').putImageData(new ImageData(ui.r.artPalette.data, PW, PH), 0, 0);
    }
    function wirePalette(ui, r) {
        const canvas = ui.el.querySelector('[data-art-palette]'), pal = paletteState(r);
        drawPalette(ui);
        if (!canvas) return;
        const at = e => {
            const box = canvas.getBoundingClientRect();
            return { x: (e.clientX - box.left) * PW / Math.max(1, box.width), y: (e.clientY - box.top) * PH / Math.max(1, box.height) };
        };
        const buf = { x: 0, y: 0, w: PW, h: PH, data: pal.data, sel: null, docW: PW, docH: PH, lockAlpha: false, wetMap: pal.map };
        let last = null, brush = null;
        const pick = p => {
            const x = Math.max(0, Math.min(PW - 1, Math.floor(p.x))), y = Math.max(0, Math.min(PH - 1, Math.floor(p.y))), i = (y * PW + x) * 4;
            const hex = r.rgbToHex(pal.data[i], pal.data[i + 1], pal.data[i + 2]);
            r.setPrimaryColor(hex);
            r.addRecentColor(hex);
        };
        const dabTo = p => {
            const step = Math.max(0.5, brush.spacing(26)), dist = Math.hypot(p.x - last.x, p.y - last.y);
            const n = Math.floor(dist / step);
            const ang = Math.atan2(p.y - last.y, p.x - last.x);
            for (let k = 1; k <= n; k++) {
                const t = k * step / dist;
                brush.dab(buf, last.x + (p.x - last.x) * t, last.y + (p.y - last.y) * t, 26, 1, ang, 1);
            }
            if (n > 0) { last = { x: last.x + (p.x - last.x) * n * step / dist, y: last.y + (p.y - last.y) * n * step / dist }; drawPalette(ui); }
        };
        canvas.addEventListener('pointerdown', e => {
            if (e.button) return;
            const p = at(e);
            if (ui.picking || e.altKey) { pick(p); return; }
            try { canvas.setPointerCapture(e.pointerId); } catch (_) { /* capture is optional */ }
            brush = paletteBrush(r, pal);
            brush.begin(true); // stays dirty so colours really mix
            brush.dab(buf, p.x, p.y, 26, 1, 0, 1);
            last = p;
            drawPalette(ui);
            e.preventDefault();
        });
        canvas.addEventListener('pointermove', e => { if (last && brush) dabTo(at(e)); });
        const stop = () => { last = null; };
        canvas.addEventListener('pointerup', stop);
        canvas.addEventListener('pointercancel', stop);
        const pickBtn = ui.el.querySelector('[data-art-pick]');
        if (pickBtn) pickBtn.addEventListener('click', () => { ui.picking = !ui.picking; pickBtn.setAttribute('aria-pressed', ui.picking ? 'true' : 'false'); });
        const clear = ui.el.querySelector('[data-art-clear]');
        if (clear) clear.addEventListener('click', () => { pal.data.fill(255); pal.map.fill(0); pal.brush = null; drawPalette(ui); });
    }

    // --- panel ---

    function refresh(r, key) {
        const ui = r.artUi;
        if (!ui || !ui.el) return;
        if (key === 'medium' || key === 'paper') { refreshMedia(ui, r); if (ui.handle && ui.handle.refresh) ui.handle.refresh(); }
        else if (!ui.busy && ui.handle && ui.handle.refresh) ui.handle.refresh();
        if (key === 'medium') refreshMedia(ui, r);
        refreshBrush(ui, r);
    }

    function mount(r, el) {
        const ui = r.artUi = { r, el, handle: null, busy: false, picking: false, token: 0 };
        r.artPanelRefresh = key => refresh(r, key);
        renderSettings(ui, r);
        paintPaperSwatch(ui, r);
        refreshBrush(ui, r);
        el.querySelectorAll('[data-art-medium]').forEach(b => b.addEventListener('click', () => {
            Art.setMedium(r, b.dataset.artMedium, 'panel');
            refreshMedia(ui, r);
            if (ui.handle && ui.handle.refresh) ui.handle.refresh();
            if (r.refreshBrushPanel) r.refreshBrushPanel();
        }));
        const paperSel = el.querySelector('[data-art-paper]'), scale = el.querySelector('[data-art-scale]');
        const paperChanged = () => {
            paintPaperSwatch(ui, r);
            schedulePreviews(ui, r);
            cfg.onChangeArt(r, 'paper', r.artPaper.id, settings(r));
        };
        if (paperSel) paperSel.addEventListener('change', () => { r.artPaper.id = paperSel.value; paperChanged(); });
        if (scale) scale.addEventListener('input', () => { r.artPaper.scale = Math.max(0.5, Math.min(4, Number(scale.value) || 1)); paintPaperSwatch(ui, r); cfg.onChangeArt(r, 'paper', r.artPaper.id, settings(r)); });
        const bind = (sel, fn) => { const b = el.querySelector(sel); if (b) b.addEventListener('click', fn); };
        bind('[data-art-add-paper]', () => Art.addPaperLayer(r));
        bind('[data-art-wash]', () => { const c = Pixel.getCommand('art.clean-brush'); if (c) c.run(r); });
        bind('[data-art-dry]', () => Art.dryNow(r));
        wirePalette(ui, r);
        schedulePreviews(ui, r);
        r.on('colors-changed', () => { if (!r.state.disposed) { schedulePreviews(ui, r); refreshBrush(ui, r); } });
    }

    Pixel.registerPanel({ id: 'art', labelKey: 'pixel.panel_art', fallback: 'Atelier', order: 36, dock: 'inspector',
        build: r => buildPanel(r),
        mount,
        onShow: r => { const ui = r.artUi; if (ui) { renderSettings(ui, r); refreshMedia(ui, r); refreshBrush(ui, r); schedulePreviews(ui, r); } }
    });
})();
