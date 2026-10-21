// Pixel module "smart" (WP7 intelligence): content-aware fill and the remove-object tool (PatchMatch in the worker, see
// pixel-lib-inpaint.js), the colour panel with picker, harmonies, saved swatches and palette extraction, and the smart
// document actions (trim, auto-crop, canvas size, fit canvas to layers).
(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    const SWATCH_KEY = 'pixel.swatches';
    const SWATCH_MAX = 48;
    const MAX_CANVAS_SIDE = 16384;
    const MAX_CANVAS_PIXELS = 120000000;
    // Base iteration count of the inpainting per quality step (see inpaint in pixel-lib-inpaint.js).
    const QUALITY = { fast: 3, balanced: 4, best: 7 };
    const hexPattern = /^#[0-9a-f]{6}$/i;

    const lib = () => window.AuraPixelLib || null;
    const clampNum = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

    function normalizeHex(text) {
        let v = String(text == null ? '' : text).trim().toLowerCase();
        if (!v.startsWith('#')) v = '#' + v;
        if (/^#[0-9a-f]{3}$/.test(v)) v = '#' + v[1] + v[1] + v[2] + v[2] + v[3] + v[3];
        return hexPattern.test(v) ? v : null;
    }

    function loadSwatches() {
        try {
            const list = JSON.parse(localStorage.getItem(SWATCH_KEY) || '[]');
            return Array.isArray(list) ? list.map(normalizeHex).filter(Boolean).slice(0, SWATCH_MAX) : [];
        } catch (_) {
            return [];
        }
    }

    function saveSwatches(list) {
        try { localStorage.setItem(SWATCH_KEY, JSON.stringify(list.slice(0, SWATCH_MAX))); } catch (_) { /* storage may be blocked */ }
    }

    // A bounding box that covers every part of boxes a and b (either may be null).
    function unionBox(a, b) {
        if (!a) return b;
        if (!b) return a;
        const x0 = Math.min(a.x, b.x), y0 = Math.min(a.y, b.y);
        return { x: x0, y: y0, w: Math.max(a.x + a.w, b.x + b.w) - x0, h: Math.max(a.y + a.h, b.y + b.h) - y0 };
    }

    // Content bounds of a canvas, read in bands so a huge canvas never needs one giant ImageData.
    function scanCanvasBounds(canvas, options) {
        const L = lib();
        const w = canvas.width, h = canvas.height;
        if (!L || !w || !h) return null;
        const ctx = canvas.getContext('2d', { willReadFrequently: true });
        const band = Math.max(1, Math.min(h, Math.floor(4000000 / w)));
        let box = null;
        for (let y = 0; y < h; y += band) {
            const rows = Math.min(band, h - y);
            const part = L.contentBounds({ width: w, height: rows, data: ctx.getImageData(0, y, w, rows).data }, options);
            if (part) box = unionBox(box, { x: part.x, y: part.y + y, w: part.w, h: part.h });
        }
        return box;
    }

    // Writes an image { width, height, data } into a canvas context at x, y (alpha included, nothing blended).
    function putImage(ctx, image, x, y) {
        if (typeof ImageData === 'function') {
            ctx.putImageData(new ImageData(image.data, image.width, image.height), x, y);
        } else {
            const target = ctx.createImageData(image.width, image.height);
            target.data.set(image.data);
            ctx.putImageData(target, x, y);
        }
    }

    const REMOVE_OBJECT_SVG = '<svg width="16" height="16" viewBox="0 0 16 16"><circle cx="6.5" cy="6.5" r="4.2" fill="none" stroke="currentColor" stroke-width="1.4" stroke-dasharray="2.2 1.6"/><path d="M9.8 9.8L14 14" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/><path d="M12.6 1.8l.55 1.45 1.45.55-1.45.55-.55 1.45-.55-1.45-1.45-.55 1.45-.55z" fill="currentColor"/></svg>';

    // --- content-aware fill dialog schema (rendered by the shared parameter controls) ---

    const FILL_SCHEMA = [
        { key: 'area', type: 'segmented', labelKey: 'pixel.caf_area', fallback: 'Sample area', default: 'auto', options: [
            { value: 'auto', labelKey: 'pixel.caf_area_auto', fallback: 'Auto' },
            { value: 'around', labelKey: 'pixel.caf_area_around', fallback: 'Around selection' }
        ] },
        { key: 'margin', type: 'range', labelKey: 'pixel.caf_margin', fallback: 'Margin', min: 16, max: 512, step: 8, default: 128, unit: 'px', showIf: v => v.area === 'around' },
        { key: 'grow', type: 'range', labelKey: 'pixel.caf_grow', fallback: 'Expand selection', min: 0, max: 24, step: 1, default: 0, unit: 'px' },
        { key: 'quality', type: 'segmented', labelKey: 'pixel.caf_quality', fallback: 'Quality', default: 'balanced', options: [
            { value: 'fast', labelKey: 'pixel.caf_quality_fast', fallback: 'Fast' },
            { value: 'balanced', labelKey: 'pixel.caf_quality_balanced', fallback: 'Balanced' },
            { value: 'best', labelKey: 'pixel.caf_quality_best', fallback: 'Best' }
        ] }
    ];

    // --- the remove-object tool (state lives on the window's runtime: r.smartState) ---

    function strokeBox(stroke, width, height) {
        let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
        stroke.points.forEach(p => {
            if (p.x < x0) x0 = p.x;
            if (p.y < y0) y0 = p.y;
            if (p.x > x1) x1 = p.x;
            if (p.y > y1) y1 = p.y;
        });
        const pad = Math.ceil(stroke.size / 2) + 6;
        const bx = Math.max(0, Math.floor(x0 - pad)), by = Math.max(0, Math.floor(y0 - pad));
        const ex = Math.min(width, Math.ceil(x1 + pad)), ey = Math.min(height, Math.ceil(y1 + pad));
        return ex > bx && ey > by ? { x: bx, y: by, w: ex - bx, h: ey - by } : null;
    }

    function tracePath(ctx, stroke, dx, dy) {
        ctx.lineCap = 'round';
        ctx.lineJoin = 'round';
        ctx.lineWidth = stroke.size;
        const pts = stroke.points;
        if (pts.length === 1) {
            ctx.beginPath();
            ctx.arc(pts[0].x - dx, pts[0].y - dy, stroke.size / 2, 0, Math.PI * 2);
            ctx.fill();
            return;
        }
        ctx.beginPath();
        ctx.moveTo(pts[0].x - dx, pts[0].y - dy);
        for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x - dx, pts[i].y - dy);
        ctx.stroke();
    }

    Pixel.registerTool({
        id: 'remove-object', group: 'retouch', order: 90, raster: true, cursor: 'none',
        labelKey: 'pixel.remove_object', fallback: 'Remove object', svg: REMOVE_OBJECT_SVG,
        options: r => {
            const size = r.smartState.removeSize;
            return `<div class="pixel-slider-row"><span class="pixel-slider-label">${r.esc(r.t('pixel.brush_size', 'Size'))}</span><input type="range" class="pixel-slider" data-ro-size min="4" max="200" value="${size}"><span class="pixel-slider-value" data-ro-size-val>${size}px</span></div>
                <p class="pixel-draw-hint">${r.esc(r.t('pixel.remove_object_hint', 'Paint over what should disappear, then release.'))}</p>`;
        },
        wireOptions: (r, bar) => {
            const slider = bar.querySelector('[data-ro-size]');
            const value = bar.querySelector('[data-ro-size-val]');
            if (!slider) return;
            slider.addEventListener('input', () => {
                r.smartState.removeSize = clampNum(Number(slider.value) || 28, 4, 200);
                if (value) value.textContent = r.smartState.removeSize + 'px';
                r.requestOverlay();
            });
        },
        onDown: (r, e, p) => {
            if (r.smartState.busy) {
                r.notify({ type: 'info', message: r.t('pixel.fill_busy', 'Another fill is still running.') });
                return;
            }
            r.smartState.stroke = { points: [{ x: p.x, y: p.y }], size: r.smartState.removeSize };
            r.requestOverlay();
        },
        onMove: (r, e, p) => {
            const stroke = r.smartState.stroke;
            r.cursorPos = { x: p.x, y: p.y };
            if (!stroke) return;
            const last = stroke.points[stroke.points.length - 1];
            if (Math.hypot(p.x - last.x, p.y - last.y) >= 1) stroke.points.push({ x: p.x, y: p.y });
            r.requestOverlay();
        },
        onHover: (r, e, p) => {
            r.cursorPos = { x: p.x, y: p.y };
            r.requestOverlay();
        },
        onUp: r => { r.finishRemoveObject(); },
        onKey: (r, e) => {
            if (e.key === 'Escape' && r.smartState.stroke) {
                r.smartState.stroke = null;
                r.requestOverlay();
                return true;
            }
            return false;
        },
        onDeactivate: r => { r.smartState.stroke = null; },
        overlay: (ctx, r) => {
            const s = r.smartState;
            if (s.stroke) {
                ctx.strokeStyle = ctx.fillStyle = 'rgba(255, 64, 64, 0.5)';
                tracePath(ctx, s.stroke, 0, 0);
            }
            const pos = r.cursorPos;
            if (!pos) return;
            const radius = Math.max(1, s.removeSize / 2);
            ctx.lineWidth = 2;
            ctx.strokeStyle = 'rgba(0, 0, 0, 0.7)';
            ctx.beginPath();
            ctx.arc(pos.x, pos.y, radius + 1, 0, Math.PI * 2);
            ctx.stroke();
            ctx.lineWidth = 1;
            ctx.strokeStyle = 'rgba(255, 255, 255, 0.9)';
            ctx.beginPath();
            ctx.arc(pos.x, pos.y, radius, 0, Math.PI * 2);
            ctx.stroke();
        }
    });

    // --- colour panel: markup ---

    const SWAP_ARROWS = '⇄';

    function harmonyLabel(r, id) {
        const labels = {
            complementary: ['pixel.harmony_complementary', 'Complementary'], analogous: ['pixel.harmony_analogous', 'Analogous'],
            triadic: ['pixel.harmony_triadic', 'Triadic'], 'split-complementary': ['pixel.harmony_split', 'Split-complementary'],
            tetradic: ['pixel.harmony_tetradic', 'Tetradic'], monochrome: ['pixel.harmony_monochrome', 'Monochrome shades']
        };
        const pair = labels[id] || ['pixel.harmony_' + id, id];
        return r.t(pair[0], pair[1]);
    }

    function swatchButton(r, attr, hex) {
        return `<button class="pixel-sc-swatch" type="button" ${attr}="${hex}" style="background:${hex}" title="${hex}" aria-label="${hex}"></button>`;
    }

    function buildColorsPanel(r) {
        const t = (key, fallback) => r.esc(r.t(key, fallback));
        const presets = r.PRESET_COLORS.map(c => `<button class="pixel-palette-swatch" type="button" data-palette-color="${c}" style="background:${c}" title="${c}"></button>`).join('');
        const num = (attr, label, min, max) => `<label class="pixel-sc-num"><span>${label}</span><input type="number" class="pixel-param-number" ${attr} min="${min}" max="${max}" step="1"></label>`;
        return `<div class="pixel-color-section pixel-sc">
            <div class="pixel-sc-top">
                <button class="pixel-sc-chip pixel-sc-chip-primary" type="button" data-sc-primary style="background:${r.primaryColor}" title="${t('pixel.color_primary', 'Primary color')}"></button>
                <button class="pixel-sc-chip pixel-sc-chip-secondary" type="button" data-sc-secondary style="background:${r.secondaryColor}" title="${t('pixel.color_secondary', 'Secondary color')}"></button>
                <button class="pixel-btn-icon" type="button" data-sc-swap title="${t('pixel.swap_colors', 'Swap colors')}">${SWAP_ARROWS}</button>
                <button class="pixel-btn-icon" type="button" data-sc-eyedropper title="${t('pixel.eyedropper', 'Eyedropper')}">${r.toolSvg('eyedropper')}</button>
            </div>
            <div class="pixel-sc-picker">
                <canvas class="pixel-sc-sv" data-sc-sv width="240" height="150" tabindex="0" role="slider" aria-label="${t('pixel.sc_saturation_value', 'Saturation and brightness')}"></canvas>
                <canvas class="pixel-sc-hue" data-sc-hue width="240" height="16" tabindex="0" role="slider" aria-label="${t('pixel.sc_hue', 'Hue')}"></canvas>
            </div>
            <div class="pixel-sc-fields">
                <input type="text" class="pixel-hex-input" data-hex-input data-sc-hex value="${r.primaryColor}" maxlength="7" spellcheck="false" title="${t('pixel.hex_value', 'Hex value')}">
                <div class="pixel-sc-row">${num('data-sc-r', 'R', 0, 255)}${num('data-sc-g', 'G', 0, 255)}${num('data-sc-b', 'B', 0, 255)}</div>
                <div class="pixel-sc-row">${num('data-sc-h', 'H', 0, 360)}${num('data-sc-s', 'S', 0, 100)}${num('data-sc-l', 'L', 0, 100)}</div>
                <div class="pixel-slider-row"><span class="pixel-slider-label">${t('pixel.sc_opacity', 'Opacity')}</span><input type="range" class="pixel-slider" data-sc-opacity min="1" max="100" value="${r.brushOpacity}"><span class="pixel-slider-value" data-sc-opacity-val>${r.brushOpacity}%</span></div>
            </div>
            <details class="pixel-sc-group" open><summary>${t('pixel.sc_harmonies', 'Harmonies')}</summary><div class="pixel-sc-harmonies" data-sc-harmonies></div></details>
            <details class="pixel-sc-group" open><summary>${t('pixel.sc_swatches', 'Swatches')}</summary>
                <div class="pixel-sc-grid" data-sc-swatches></div>
                <div class="pixel-sc-actions"><button class="pixel-btn" type="button" data-sc-add-swatch>${t('pixel.sc_add_swatch', 'Add current color')}</button></div>
            </details>
            <details class="pixel-sc-group" data-sc-palette-group><summary>${t('pixel.sc_palette', 'Palette from image')}</summary>
                <div class="pixel-sc-actions">
                    <select class="pixel-select pixel-sc-count" data-sc-count aria-label="${t('pixel.sc_palette_count', 'Number of colors')}">${[5, 6, 8, 10, 12, 16].map(n => `<option value="${n}"${n === 8 ? ' selected' : ''}>${n}</option>`).join('')}</select>
                    <button class="pixel-btn" type="button" data-sc-extract>${t('pixel.sc_extract', 'Extract')}</button>
                </div>
                <div class="pixel-sc-grid" data-sc-palette></div>
                <div class="pixel-sc-actions" data-sc-palette-actions hidden>
                    <button class="pixel-btn" type="button" data-sc-pal-swatches>${t('pixel.sc_pal_to_swatches', 'Add to swatches')}</button>
                    <button class="pixel-btn" type="button" data-sc-pal-gradient>${t('pixel.sc_pal_gradient', 'As gradient')}</button>
                    <button class="pixel-btn" type="button" data-sc-pal-layer>${t('pixel.sc_pal_layer', 'Palette layer')}</button>
                </div>
            </details>
            <details class="pixel-sc-group"><summary>${t('pixel.sc_presets', 'Presets')}</summary><div class="pixel-palette-grid">${presets}</div></details>
            <div class="pixel-recent-colors"><span class="pixel-label">${t('pixel.recent_colors', 'Recent colors')}</span><div class="pixel-recent-row" data-recent-colors></div></div>
        </div>`;
    }

    // --- colour panel: behaviour ---

    function mountColorsPanel(r, el) {
        const L = lib();
        const cs = { h: 0, s: 0, v: 1, hex: '#ffffff', palette: [], dragging: null };
        const q = selector => el.querySelector(selector);
        const svCanvas = q('[data-sc-sv]'), hueCanvas = q('[data-sc-hue]');
        const hexInput = q('[data-sc-hex]');
        const fields = { r: q('[data-sc-r]'), g: q('[data-sc-g]'), b: q('[data-sc-b]'), h: q('[data-sc-h]'), s: q('[data-sc-s]'), l: q('[data-sc-l]') };
        let redrawFrame = 0;

        function hexOf(h, s, v) {
            const rgb = L.hsvToRgb(h, s, v);
            return L.toHex(rgb[0], rgb[1], rgb[2]);
        }

        function drawSV() {
            if (!svCanvas) return;
            const ctx = svCanvas.getContext('2d');
            const w = svCanvas.width, h = svCanvas.height;
            ctx.fillStyle = hexOf(cs.h, 1, 1);
            ctx.fillRect(0, 0, w, h);
            let g = ctx.createLinearGradient(0, 0, w, 0);
            g.addColorStop(0, '#ffffff');
            g.addColorStop(1, 'rgba(255,255,255,0)');
            ctx.fillStyle = g;
            ctx.fillRect(0, 0, w, h);
            g = ctx.createLinearGradient(0, 0, 0, h);
            g.addColorStop(0, 'rgba(0,0,0,0)');
            g.addColorStop(1, '#000000');
            ctx.fillStyle = g;
            ctx.fillRect(0, 0, w, h);
            const x = cs.s * w, y = (1 - cs.v) * h;
            ctx.lineWidth = 3;
            ctx.strokeStyle = '#000000';
            ctx.beginPath();
            ctx.arc(x, y, 6, 0, Math.PI * 2);
            ctx.stroke();
            ctx.lineWidth = 1.5;
            ctx.strokeStyle = '#ffffff';
            ctx.beginPath();
            ctx.arc(x, y, 6, 0, Math.PI * 2);
            ctx.stroke();
        }

        function drawHue() {
            if (!hueCanvas) return;
            const ctx = hueCanvas.getContext('2d');
            const w = hueCanvas.width, h = hueCanvas.height;
            const g = ctx.createLinearGradient(0, 0, w, 0);
            for (let i = 0; i <= 6; i++) g.addColorStop(i / 6, hexOf(i * 60, 1, 1));
            ctx.fillStyle = g;
            ctx.fillRect(0, 0, w, h);
            const x = cs.h / 360 * w;
            ctx.lineWidth = 3;
            ctx.strokeStyle = '#000000';
            ctx.strokeRect(x - 3, 1, 6, h - 2);
            ctx.lineWidth = 1.5;
            ctx.strokeStyle = '#ffffff';
            ctx.strokeRect(x - 3, 1, 6, h - 2);
        }

        function redrawPicker() {
            if (redrawFrame) return;
            const run = () => { redrawFrame = 0; drawSV(); drawHue(); };
            redrawFrame = typeof requestAnimationFrame === 'function' ? requestAnimationFrame(run) : 0;
            if (!redrawFrame) run();
        }

        function paintInputs() {
            const rgb = L.parseColor(cs.hex) || [0, 0, 0];
            const hsl = L.rgbToHsl(rgb[0], rgb[1], rgb[2]);
            if (hexInput && document.activeElement !== hexInput) hexInput.value = cs.hex;
            const set = (key, value) => { if (fields[key] && document.activeElement !== fields[key]) fields[key].value = String(Math.round(value)); };
            set('r', rgb[0]); set('g', rgb[1]); set('b', rgb[2]);
            // Hue and saturation of a grey are not defined: keep what the picker shows.
            set('h', hsl[1] > 0 && hsl[2] > 0 && hsl[2] < 1 ? hsl[0] : cs.h);
            set('s', hsl[1] * 100); set('l', hsl[2] * 100);
            const primary = q('[data-sc-primary]');
            if (primary) primary.style.background = cs.hex;
        }

        function renderHarmonies() {
            const host = q('[data-sc-harmonies]');
            if (!host) return;
            host.innerHTML = L.colorHarmonies(cs.hex).map(row =>
                `<div class="pixel-sc-harmony"><span class="pixel-sc-harmony-name">${r.esc(harmonyLabel(r, row.id))}</span><div class="pixel-sc-grid pixel-sc-grid-inline">${row.colors.map(c => swatchButton(r, 'data-sc-color', c)).join('')}</div></div>`
            ).join('');
        }

        function renderSwatches() {
            const host = q('[data-sc-swatches]');
            if (!host) return;
            const list = loadSwatches();
            host.innerHTML = list.length
                ? list.map(c => swatchButton(r, 'data-sc-saved', c)).join('')
                : `<span class="pixel-draw-hint">${r.esc(r.t('pixel.sc_no_swatches', 'No saved swatches yet.'))}</span>`;
        }

        function renderPalette() {
            const host = q('[data-sc-palette]');
            const actions = q('[data-sc-palette-actions]');
            if (!host) return;
            host.innerHTML = cs.palette.map(c => swatchButton(r, 'data-sc-color', c.hex)).join('');
            if (actions) actions.hidden = !cs.palette.length;
        }

        // Makes hex the colour shown by the panel; the picker keeps its hue for greys and blacks.
        function showColor(hex, keepPicker) {
            const normal = normalizeHex(hex);
            if (!normal) return;
            const changed = normal !== cs.hex;
            if (!keepPicker) {
                const rgb = L.parseColor(normal);
                const hsv = L.rgbToHsv(rgb[0], rgb[1], rgb[2]);
                if (hsv[1] > 0 && hsv[2] > 0) cs.h = hsv[0];
                cs.s = hsv[1];
                cs.v = hsv[2];
            }
            cs.hex = normal;
            paintInputs();
            redrawPicker();
            if (changed) renderHarmonies();
        }

        // The user changed the colour in this panel.
        function commit(hex, remember) {
            const normal = normalizeHex(hex);
            if (!normal) return;
            cs.hex = normal;
            r.setPrimaryColor(normal);
            if (remember) r.addRecentColor(normal);
            paintInputs();
            redrawPicker();
            renderHarmonies();
        }

        function pickFromHSV(h, s, v, remember) {
            cs.h = clampNum(h, 0, 360);
            cs.s = clampNum(s, 0, 1);
            cs.v = clampNum(v, 0, 1);
            commit(hexOf(cs.h, cs.s, cs.v), remember);
        }

        function drag(canvas, apply) {
            if (!canvas) return;
            const at = e => {
                const rect = canvas.getBoundingClientRect();
                return { fx: clampNum((e.clientX - rect.left) / Math.max(1, rect.width), 0, 1), fy: clampNum((e.clientY - rect.top) / Math.max(1, rect.height), 0, 1) };
            };
            canvas.addEventListener('pointerdown', e => {
                if (e.button !== 0) return;
                cs.dragging = canvas;
                try { canvas.setPointerCapture(e.pointerId); } catch (_) { /* synthetic pointer */ }
                const p = at(e);
                apply(p.fx, p.fy, false);
                e.preventDefault();
            });
            canvas.addEventListener('pointermove', e => {
                if (cs.dragging !== canvas) return;
                const p = at(e);
                apply(p.fx, p.fy, false);
            });
            const finish = () => {
                if (cs.dragging !== canvas) return;
                cs.dragging = null;
                r.addRecentColor(cs.hex);
            };
            canvas.addEventListener('pointerup', finish);
            canvas.addEventListener('pointercancel', finish);
        }

        drag(svCanvas, (fx, fy, remember) => pickFromHSV(cs.h, fx, 1 - fy, remember));
        drag(hueCanvas, (fx, fy, remember) => pickFromHSV(fx * 360, cs.s || 0.0001, cs.v || 0.0001, remember));
        if (svCanvas) {
            svCanvas.addEventListener('keydown', e => {
                const step = e.shiftKey ? 0.05 : 0.01;
                const moves = { ArrowLeft: [-step, 0], ArrowRight: [step, 0], ArrowUp: [0, step], ArrowDown: [0, -step] };
                const move = moves[e.key];
                if (!move) return;
                e.preventDefault();
                pickFromHSV(cs.h, cs.s + move[0], cs.v + move[1], true);
            });
        }
        if (hueCanvas) {
            hueCanvas.addEventListener('keydown', e => {
                const step = e.shiftKey ? 10 : 1;
                if (e.key === 'ArrowLeft' || e.key === 'ArrowDown') { e.preventDefault(); pickFromHSV(cs.h - step, cs.s, cs.v, true); }
                if (e.key === 'ArrowRight' || e.key === 'ArrowUp') { e.preventDefault(); pickFromHSV(cs.h + step, cs.s, cs.v, true); }
            });
        }

        if (hexInput) {
            hexInput.addEventListener('input', () => {
                const hex = normalizeHex(hexInput.value);
                if (hex && hexInput.value.replace('#', '').length === 6) commit(hex, false);
            });
            hexInput.addEventListener('change', () => {
                const hex = normalizeHex(hexInput.value);
                if (hex) commit(hex, true);
                else hexInput.value = cs.hex;
            });
        }
        const readRGB = () => ['r', 'g', 'b'].map(key => clampNum(Math.round(Number(fields[key].value) || 0), 0, 255));
        ['r', 'g', 'b'].forEach(key => {
            if (!fields[key]) return;
            fields[key].addEventListener('input', () => { const c = readRGB(); commit(L.toHex(c[0], c[1], c[2]), false); });
            fields[key].addEventListener('change', () => { const c = readRGB(); commit(L.toHex(c[0], c[1], c[2]), true); paintInputs(); });
        });
        const readHSL = () => [clampNum(Number(fields.h.value) || 0, 0, 360), clampNum(Number(fields.s.value) || 0, 0, 100) / 100, clampNum(Number(fields.l.value) || 0, 0, 100) / 100];
        ['h', 's', 'l'].forEach(key => {
            if (!fields[key]) return;
            const apply = remember => {
                const hsl = readHSL();
                const rgb = L.hslToRgb(hsl[0], hsl[1], hsl[2]);
                const hex = L.toHex(rgb[0], rgb[1], rgb[2]);
                const hsv = L.rgbToHsv(rgb[0], rgb[1], rgb[2]);
                if (hsl[1] > 0 && hsl[2] > 0 && hsl[2] < 1) cs.h = hsl[0];
                cs.s = hsv[1];
                cs.v = hsv[2];
                commit(hex, remember);
            };
            fields[key].addEventListener('input', () => apply(false));
            fields[key].addEventListener('change', () => { apply(true); paintInputs(); });
        });

        const opacity = q('[data-sc-opacity]');
        const opacityValue = q('[data-sc-opacity-val]');
        if (opacity) {
            opacity.addEventListener('input', () => {
                r.brushOpacity = clampNum(Number(opacity.value) || 100, 1, 100);
                if (opacityValue) opacityValue.textContent = r.brushOpacity + '%';
                // The options bar of a brush shows the same value.
                const bar = r.optionsBar && r.optionsBar.querySelector ? r.optionsBar.querySelector('[data-brush-opacity]') : null;
                if (bar) {
                    bar.value = String(r.brushOpacity);
                    const label = r.optionsBar.querySelector('[data-brush-opacity-val]');
                    if (label) label.textContent = r.brushOpacity + '%';
                }
            });
        }

        // Clicks: any swatch sets the primary colour, a right click the secondary one.
        const colorOf = target => {
            const node = target && target.closest ? target.closest('[data-sc-color],[data-sc-saved],[data-palette-color],[data-recent-color]') : null;
            if (!node) return null;
            return { node, hex: node.dataset.scColor || node.dataset.scSaved || node.dataset.paletteColor || node.dataset.recentColor };
        };
        el.addEventListener('click', e => {
            const hit = colorOf(e.target);
            if (!hit) return;
            if (hit.node.dataset.scSaved && e.shiftKey) {
                saveSwatches(loadSwatches().filter(c => c !== normalizeHex(hit.hex)));
                renderSwatches();
                return;
            }
            commit(hit.hex, true);
        });
        el.addEventListener('contextmenu', e => {
            const hit = colorOf(e.target);
            if (!hit) return;
            e.preventDefault();
            e.stopPropagation();
            r.setSecondaryColor(hit.hex);
        });
        const click = (selector, handler) => { const node = q(selector); if (node) node.addEventListener('click', handler); };
        click('[data-sc-primary]', () => { const native = document.createElement('input'); native.type = 'color'; native.value = cs.hex; native.addEventListener('input', () => commit(native.value, false)); native.addEventListener('change', () => { r.addRecentColor(native.value); native.remove(); }); native.click(); });
        click('[data-sc-secondary]', () => { const native = document.createElement('input'); native.type = 'color'; native.value = r.secondaryColor; native.addEventListener('input', () => r.setSecondaryColor(native.value)); native.addEventListener('change', () => native.remove()); native.click(); });
        click('[data-sc-swap]', () => r.swapColors());
        click('[data-sc-eyedropper]', () => { if (r.activeTool !== 'eyedropper') r.setActiveTool('eyedropper'); });
        click('[data-sc-add-swatch]', () => r.addSwatch(r.primaryColor));
        const extract = () => {
            const count = Number((q('[data-sc-count]') || {}).value) || 8;
            cs.palette = r.extractDocumentPalette(count);
            renderPalette();
            const group = q('[data-sc-palette-group]');
            if (group) group.open = true;
        };
        click('[data-sc-extract]', extract);
        click('[data-sc-pal-swatches]', () => cs.palette.forEach(c => r.addSwatch(c.hex, true)));
        click('[data-sc-pal-gradient]', () => r.paletteToGradientLayer(cs.palette));
        click('[data-sc-pal-layer]', () => r.paletteToLayer(cs.palette));

        // The colour can change anywhere (eyedropper, rail swatches, swap, palette): follow it.
        r.on('colors-changed', data => {
            const swatch = q('[data-sc-secondary]');
            if (swatch && data && data.secondary) swatch.style.background = data.secondary;
            if (data && data.primary && data.primary !== cs.hex) showColor(data.primary, false);
        });
        r.on('swatches-changed', renderSwatches);

        r.renderRecentColors();
        showColor(r.primaryColor, false);
        renderSwatches();
        r.smartState.refreshColors = () => { showColor(r.primaryColor, false); renderSwatches(); };
        r.smartState.extractPalette = extract;
    }

    // --- install (per window) ---

    Pixel.registerModule('smart', function install(r) {
        const smart = {
            busy: false,
            cancel: null,
            fill: { area: 'auto', margin: 128, grow: 0, quality: 'balanced' },
            removeSize: 28,
            stroke: null,
            refreshColors: null,
            extractPalette: null
        };
        r.smartState = smart;
        const t = (key, fallback) => r.t(key, fallback);
        const info = message => r.notify({ type: 'info', message });
        const failure = message => r.notify({ type: 'error', message });

        // --- colour events: the setters announce the change so the colour panel can follow ---
        ['setPrimaryColor', 'setSecondaryColor'].forEach(name => {
            const original = r[name];
            if (typeof original !== 'function') return;
            r[name] = function colorSetter() {
                const result = original.apply(r, arguments);
                r.emit('colors-changed', { primary: r.primaryColor, secondary: r.secondaryColor });
                return result;
            };
        });

        // --- busy overlay with progress and cancel (the Escape key cancels too) ---
        function showProgress(label, onCancel) {
            const root = r.appEl || r.host;
            const el = document.createElement('div');
            el.className = 'pixel-smart-busy';
            el.setAttribute('role', 'status');
            el.innerHTML = `<span class="pixel-smart-busy-label">${r.esc(label)}</span><span class="pixel-smart-busy-bar"><i data-sb-fill></i></span><button type="button" class="pixel-btn" data-sb-cancel>${r.esc(t('pixel.cancel', 'Cancel'))}</button>`;
            root.appendChild(el);
            const fill = el.querySelector('[data-sb-fill]');
            const onKey = e => {
                if (e.key !== 'Escape') return;
                e.preventDefault();
                e.stopPropagation();
                onCancel();
            };
            document.addEventListener('keydown', onKey, true);
            el.querySelector('[data-sb-cancel]').addEventListener('click', onCancel);
            return {
                set(fraction) { if (fill) fill.style.width = Math.round(clampNum(fraction, 0, 1) * 100) + '%'; },
                done() { document.removeEventListener('keydown', onKey, true); el.remove(); }
            };
        }

        // --- inpainting ---

        // Fills a hole of the active layer from its surroundings. spec: { alpha (Uint8ClampedArray, box.w * box.h, 0..255),
        // box (document rect of alpha), label, margin ('auto' | px), patch, iterations, respectSelection }. Resolves true when
        // the result was committed (one history step).
        async function inpaintHole(spec) {
            const L = lib();
            const layer = r.activeLayer();
            if (!layer || !r.imgWidth || !L || typeof L.inpaintRegion !== 'function') return false;
            if (smart.busy) { info(t('pixel.fill_busy', 'Another fill is still running.')); return false; }
            if (!r.canPaintActiveLayer('content-aware-fill')) return false;
            r.commitPendingPreviews();
            const W = r.imgWidth, H = r.imgHeight, box = spec.box;
            const inMask = !!(r.editMaskMode && layer.maskCanvas);
            const sourceCanvas = inMask ? layer.maskCanvas : layer.canvas;
            const region = L.inpaintRegion(box, W, H, { margin: spec.margin, patch: spec.patch });
            if (!region) return false;
            const image = r.imageFromCanvasRegion(sourceCanvas, region);
            const mask = new Uint8ClampedArray(region.w * region.h);
            for (let y = 0; y < box.h; y++) {
                mask.set(spec.alpha.subarray(y * box.w, (y + 1) * box.w), (box.y - region.y + y) * region.w + (box.x - region.x));
            }
            const stamp = { layer, rev: inMask ? layer.maskRev : layer.rev, width: W, height: H, layers: r.layers };
            const controller = new AbortController();
            smart.busy = true;
            smart.cancel = () => controller.abort();
            const progress = showProgress(spec.label, () => controller.abort());
            r.setStatus(spec.label + '…');
            let result;
            try {
                result = await r.runJob('inpaint.fill', {
                    image, mask, patch: spec.patch || 7, iterations: spec.iterations || QUALITY.balanced, margin: spec.margin, seed: 1, inPlace: true
                }, { transfer: [image.data.buffer, mask.buffer], signal: controller.signal, key: 'pixel-inpaint', onProgress: f => progress.set(f) });
            } catch (err) {
                if (err && err.name === 'AbortError') info(t('pixel.fill_cancelled', 'Fill cancelled.'));
                else if (err && /no-sources/.test(err.message || '')) failure(t('pixel.fill_no_sources', 'There is nothing around the selection to copy from. Select a smaller area.'));
                else failure(t('pixel.fill_failed', 'The fill failed.') + (err && err.message ? ' ' + err.message : ''));
                return false;
            } finally {
                progress.done();
                smart.busy = false;
                smart.cancel = null;
                if (!r.state.disposed) r.updateStatus();
            }
            if (r.state.disposed) return false;
            const stale = r.layers !== stamp.layers || r.layers.indexOf(layer) < 0 || r.imgWidth !== stamp.width || r.imgHeight !== stamp.height ||
                (inMask ? layer.maskRev : layer.rev) !== stamp.rev || r.activeLayer() !== layer;
            if (stale) { info(t('pixel.fill_stale', 'The image changed while the fill was running, so the result was dropped.')); return false; }
            const out = r.copyCanvas(sourceCanvas);
            putImage(out.getContext('2d'), result.image, region.x, region.y);
            const extra = new Uint8ClampedArray(W * H);
            for (let y = 0; y < box.h; y++) extra.set(spec.alpha.subarray(y * box.w, (y + 1) * box.w), (box.y + y) * W + box.x);
            return r.commitLayerPixels(layer, out, { label: spec.label, respectSelection: spec.respectSelection !== false, mask: extra });
        }

        // Image data { width, height, data } of a region of a canvas.
        function imageFromCanvasRegion(canvas, region) {
            const data = canvas.getContext('2d', { willReadFrequently: true }).getImageData(region.x, region.y, region.w, region.h);
            return { width: region.w, height: region.h, data: data.data };
        }

        // Fills the selection of the active layer from the pixels around it. options (all optional, else the last values
        // of the dialog): { area: 'auto' | 'around', margin, grow, quality: 'fast' | 'balanced' | 'best' }.
        async function contentAwareFill(options) {
            const L = lib();
            if (!r.imgWidth || !L) return false;
            const sel = r.getSelectionMask();
            if (!sel) { info(t('pixel.fill_need_selection', 'Select the area to fill first.')); return false; }
            const o = Object.assign({}, smart.fill, options || {});
            const grow = clampNum(Math.round(Number(o.grow) || 0), 0, 40);
            const W = r.imgWidth, H = r.imgHeight;
            const x0 = Math.max(0, sel.x - grow), y0 = Math.max(0, sel.y - grow);
            const x1 = Math.min(W, sel.x + sel.w + grow), y1 = Math.min(H, sel.y + sel.h + grow);
            const box = { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
            let alpha = new Uint8ClampedArray(box.w * box.h);
            for (let y = 0; y < box.h; y++) {
                const from = (box.y + y) * W + box.x;
                alpha.set(sel.alpha.subarray(from, from + box.w), y * box.w);
            }
            if (grow > 0) alpha = L.morphAlpha(alpha, box.w, box.h, grow, 'dilate');
            return inpaintHole({
                alpha, box, label: t('pixel.content_aware_fill', 'Content-aware fill'),
                margin: o.area === 'around' ? clampNum(Number(o.margin) || 128, 16, 512) : 'auto',
                iterations: QUALITY[o.quality] || QUALITY.balanced, respectSelection: false
            });
        }

        function showContentAwareFillDialog() {
            if (!r.imgWidth) return;
            if (!r.selection) { info(t('pixel.fill_need_selection', 'Select the area to fill first.')); return; }
            const values = Object.assign(r.defaultParams(FILL_SCHEMA), smart.fill);
            const dlg = document.createElement('div');
            dlg.className = 'vd-modal-backdrop';
            dlg.innerHTML = `<form class="vd-modal pixel-smart-dialog" role="dialog" aria-modal="true">
                <div class="vd-modal-title">${r.esc(t('pixel.content_aware_fill', 'Content-aware fill'))}</div>
                <p class="pixel-draw-hint">${r.esc(t('pixel.caf_hint', 'Fills the selection with what the surroundings suggest.'))}</p>
                <div data-caf-params>${r.buildParamControls(FILL_SCHEMA, values, { idPrefix: 'pixel-caf' })}</div>
                <div class="vd-modal-actions">
                    <button type="button" class="vd-button" data-cancel>${r.esc(t('pixel.cancel', 'Cancel'))}</button>
                    <button type="submit" class="vd-button vd-button-primary">${r.esc(t('pixel.apply', 'Apply'))}</button>
                </div>
            </form>`;
            document.body.appendChild(dlg);
            const wire = r.wireParamControls(dlg.querySelector('[data-caf-params]'), FILL_SCHEMA, values, () => {});
            const close = () => { if (wire) wire.dispose(); dlg.remove(); };
            dlg.querySelector('form').addEventListener('submit', e => {
                e.preventDefault();
                smart.fill = { area: values.area, margin: values.margin, grow: values.grow, quality: values.quality };
                close();
                contentAwareFill(smart.fill);
            });
            dlg.querySelector('[data-cancel]').addEventListener('click', close);
            dlg.addEventListener('click', e => { if (e.target === dlg) close(); });
            dlg.addEventListener('keydown', e => { if (e.key === 'Escape') { e.stopPropagation(); close(); } });
            const first = dlg.querySelector('button, input');
            if (first && first.focus) first.focus();
        }

        // Mouse-up of the remove-object tool: the stroke becomes a mask (grown by 3 px) that is filled.
        function finishRemoveObject() {
            const stroke = smart.stroke;
            smart.stroke = null;
            r.requestOverlay();
            const L = lib();
            if (!stroke || !stroke.points.length || !L || !r.imgWidth) return Promise.resolve(false);
            const box = strokeBox(stroke, r.imgWidth, r.imgHeight);
            if (!box) return Promise.resolve(false);
            const canvas = r.makeCanvas(box.w, box.h);
            const ctx = canvas.getContext('2d', { willReadFrequently: true });
            ctx.strokeStyle = ctx.fillStyle = '#ffffff';
            tracePath(ctx, stroke, box.x, box.y);
            const data = ctx.getImageData(0, 0, box.w, box.h).data;
            let alpha = new Uint8ClampedArray(box.w * box.h);
            let any = false;
            for (let i = 0; i < alpha.length; i++) {
                if (data[i * 4 + 3] >= 128) { alpha[i] = 255; any = true; }
            }
            if (!any) return Promise.resolve(false);
            alpha = L.morphAlpha(alpha, box.w, box.h, 3, 'dilate');
            return inpaintHole({ alpha, box, label: t('pixel.remove_object', 'Remove object'), margin: 'auto', iterations: QUALITY.balanced, respectSelection: true });
        }

        // --- smart document actions ---

        function cropDocument(box, label) {
            r.transformDocument([1, 0, 0, 1, -box.x, -box.y], box.w, box.h);
            r.pushHistory(label);
            r.zoomFit();
        }

        function trimTransparent() {
            const L = lib();
            if (!r.imgWidth || !L) return false;
            r.commitPendingPreviews();
            const box = scanCanvasBounds(r.flattenDocument(), { mode: 'alpha', threshold: 0 });
            if (!box) { info(t('pixel.trim_empty', 'The image is completely transparent.')); return false; }
            if (box.w === r.imgWidth && box.h === r.imgHeight) { info(t('pixel.trim_nothing', 'There is nothing to trim.')); return false; }
            cropDocument(box, 'trim');
            return true;
        }

        // Crops to everything that differs from the background (the corner colour, or transparency).
        function autoCropToContent(tolerance) {
            const L = lib();
            if (!r.imgWidth || !L) return false;
            r.commitPendingPreviews();
            const flat = r.flattenDocument();
            const ctx = flat.getContext('2d', { willReadFrequently: true });
            const w = flat.width, h = flat.height;
            const corner = (x, y) => Array.from(ctx.getImageData(x, y, 1, 1).data);
            const tol = tolerance == null ? 12 : tolerance;
            const background = L.estimateBackground([corner(0, 0), corner(w - 1, 0), corner(0, h - 1), corner(w - 1, h - 1)], tol);
            const box = scanCanvasBounds(flat, background.transparent
                ? { mode: 'alpha', threshold: 8 }
                : { mode: 'color', background: background.color, tolerance: tol, threshold: 8 });
            if (!box) { info(t('pixel.autocrop_empty', 'No content found.')); return false; }
            if (box.w === w && box.h === h) { info(t('pixel.autocrop_nothing', 'The content already fills the image.')); return false; }
            cropDocument(box, 'auto-crop');
            return true;
        }

        // Crops the canvas to the union of the content of all layers (hidden ones included).
        function fitCanvasToLayers() {
            if (!r.imgWidth || !lib()) return false;
            r.commitPendingPreviews();
            let box = null;
            r.layers.forEach(layer => { box = unionBox(box, scanCanvasBounds(layer.canvas, { mode: 'alpha', threshold: 0 })); });
            if (!box) { info(t('pixel.trim_empty', 'The image is completely transparent.')); return false; }
            if (box.w === r.imgWidth && box.h === r.imgHeight) { info(t('pixel.fit_nothing', 'The canvas already fits the layers.')); return false; }
            cropDocument(box, 'fit-canvas');
            return true;
        }

        // New canvas size with the old content placed by an anchor ('tl' .. 'br', 'c' = centre). fill: 'transparent' |
        // 'primary' | 'secondary' | 'white' | 'black' | '#rrggbb' for the area that is added.
        function resizeCanvas(width, height, options) {
            const o = options || {};
            if (!r.imgWidth) return false;
            const nw = Math.round(Number(width)), nh = Math.round(Number(height));
            if (!(nw >= 1 && nh >= 1) || nw > MAX_CANVAS_SIDE || nh > MAX_CANVAS_SIDE || nw * nh > MAX_CANVAS_PIXELS) {
                failure(t('pixel.canvas_too_large', 'That canvas size is not possible.'));
                return false;
            }
            const w = r.imgWidth, h = r.imgHeight;
            if (nw === w && nh === h) return false;
            const anchor = String(o.anchor || 'c');
            const col = anchor.includes('l') ? 0 : (anchor.includes('r') ? 1 : 0.5);
            const row = anchor.includes('t') ? 0 : (anchor.includes('b') ? 1 : 0.5);
            const dx = Math.round((nw - w) * col), dy = Math.round((nh - h) * row);
            const fills = { primary: r.primaryColor, secondary: r.secondaryColor, white: '#ffffff', black: '#000000' };
            const color = o.fill && o.fill !== 'transparent' ? (fills[o.fill] || normalizeHex(o.fill)) : null;
            r.transformDocument([1, 0, 0, 1, dx, dy], nw, nh);
            if (color && (nw > w || nh > h)) {
                let target = r.layers[0];
                if (!target || target.sticker || target.lockPixels) {
                    target = r.createLayer({ name: t('pixel.layer_background', 'Background') });
                    r.layers.unshift(target);
                    r.activeLayerIdx = r.activeLayerIdx + 1;
                }
                const lctx = target.canvas.getContext('2d');
                lctx.save();
                lctx.fillStyle = color;
                lctx.beginPath();
                lctx.rect(0, 0, nw, nh);
                lctx.rect(dx, dy, w, h);
                lctx.fill('evenodd');
                lctx.restore();
                r.markLayerDirty(target);
                r.layersStructureChanged();
            }
            r.pushHistory('canvas size');
            r.zoomFit();
            return true;
        }

        const ANCHORS = [['tl', '↖'], ['t', '↑'], ['tr', '↗'], ['l', '←'], ['c', '•'], ['r', '→'], ['bl', '↙'], ['b', '↓'], ['br', '↘']];

        function showCanvasSizeDialog() {
            if (!r.imgWidth) return;
            const dlg = document.createElement('div');
            dlg.className = 'vd-modal-backdrop';
            const fillOption = (value, key, fallback) => `<option value="${value}">${r.esc(t(key, fallback))}</option>`;
            dlg.innerHTML = `<form class="vd-modal pixel-smart-dialog" role="dialog" aria-modal="true">
                <div class="vd-modal-title">${r.esc(t('pixel.canvas_size', 'Canvas size'))}</div>
                <div class="pixel-smart-form">
                    <label class="pixel-label">${r.esc(t('pixel.width', 'Width'))}</label>
                    <input class="vd-modal-input" type="number" data-cs-w value="${r.imgWidth}" min="1" max="${MAX_CANVAS_SIDE}">
                    <label class="pixel-label">${r.esc(t('pixel.height', 'Height'))}</label>
                    <input class="vd-modal-input" type="number" data-cs-h value="${r.imgHeight}" min="1" max="${MAX_CANVAS_SIDE}">
                    <label class="pixel-checkbox-row"><input type="checkbox" data-cs-lock> ${r.esc(t('pixel.lock_ratio', 'Keep proportions'))}</label>
                    <span class="pixel-label">${r.esc(t('pixel.canvas_anchor', 'Anchor'))}</span>
                    <div class="pixel-smart-anchor" role="radiogroup" aria-label="${r.esc(t('pixel.canvas_anchor', 'Anchor'))}">${ANCHORS.map(([code, glyph]) =>
                        `<button type="button" class="pixel-smart-anchor-btn${code === 'c' ? ' active' : ''}" role="radio" aria-checked="${code === 'c' ? 'true' : 'false'}" data-cs-anchor="${code}">${glyph}</button>`).join('')}</div>
                    <label class="pixel-label">${r.esc(t('pixel.canvas_fill', 'New area'))}</label>
                    <select class="pixel-select" data-cs-fill>${fillOption('transparent', 'pixel.canvas_fill_transparent', 'Transparent')}${fillOption('primary', 'pixel.canvas_fill_primary', 'Primary color')}${fillOption('secondary', 'pixel.canvas_fill_secondary', 'Secondary color')}${fillOption('white', 'pixel.canvas_fill_white', 'White')}${fillOption('black', 'pixel.canvas_fill_black', 'Black')}</select>
                </div>
                <div class="vd-modal-actions">
                    <button type="button" class="vd-button" data-cancel>${r.esc(t('pixel.cancel', 'Cancel'))}</button>
                    <button type="submit" class="vd-button vd-button-primary">${r.esc(t('pixel.apply', 'Apply'))}</button>
                </div>
            </form>`;
            document.body.appendChild(dlg);
            const wInput = dlg.querySelector('[data-cs-w]'), hInput = dlg.querySelector('[data-cs-h]');
            const lock = dlg.querySelector('[data-cs-lock]'), fill = dlg.querySelector('[data-cs-fill]');
            const ratio = r.imgWidth / r.imgHeight;
            let anchor = 'c';
            wInput.addEventListener('input', () => { if (lock.checked) hInput.value = String(Math.max(1, Math.round(wInput.value / ratio))); });
            hInput.addEventListener('input', () => { if (lock.checked) wInput.value = String(Math.max(1, Math.round(hInput.value * ratio))); });
            dlg.querySelectorAll('[data-cs-anchor]').forEach(btn => btn.addEventListener('click', () => {
                anchor = btn.dataset.csAnchor;
                dlg.querySelectorAll('[data-cs-anchor]').forEach(other => {
                    const on = other === btn;
                    other.classList.toggle('active', on);
                    other.setAttribute('aria-checked', on ? 'true' : 'false');
                });
            }));
            const close = () => dlg.remove();
            dlg.querySelector('form').addEventListener('submit', e => {
                e.preventDefault();
                const done = resizeCanvas(wInput.value, hInput.value, { anchor, fill: fill.value });
                if (done !== false || (Number(wInput.value) === r.imgWidth && Number(hInput.value) === r.imgHeight)) close();
            });
            dlg.querySelector('[data-cancel]').addEventListener('click', close);
            dlg.addEventListener('click', e => { if (e.target === dlg) close(); });
            dlg.addEventListener('keydown', e => { if (e.key === 'Escape') { e.stopPropagation(); close(); } });
            wInput.focus();
        }

        // --- colours: swatches and palettes ---

        function addSwatch(hex, quiet) {
            const normal = normalizeHex(hex);
            if (!normal) return false;
            const list = loadSwatches();
            if (list.includes(normal)) {
                if (!quiet) info(t('pixel.sc_swatch_exists', 'That color is already saved.'));
                return false;
            }
            list.unshift(normal);
            saveSwatches(list);
            r.emit('swatches-changed', { swatches: list.slice(0, SWATCH_MAX) });
            return true;
        }

        // Dominant colours of the document (the selection's bounding box when there is one), sorted by population.
        function extractDocumentPalette(count) {
            const L = lib();
            if (!r.imgWidth || !L || !r.canvas) return [];
            const sel = r.selection ? r.getSelectionMask() : null;
            const box = sel ? { x: sel.x, y: sel.y, w: sel.w, h: sel.h } : { x: 0, y: 0, w: r.imgWidth, h: r.imgHeight };
            const scale = Math.min(1, 256 / Math.max(box.w, box.h));
            const pw = Math.max(1, Math.round(box.w * scale)), ph = Math.max(1, Math.round(box.h * scale));
            const proxy = r.makeCanvas(pw, ph);
            const pctx = proxy.getContext('2d', { willReadFrequently: true });
            pctx.drawImage(r.canvas, box.x, box.y, box.w, box.h, 0, 0, pw, ph);
            const palette = L.extractPalette({ width: pw, height: ph, data: pctx.getImageData(0, 0, pw, ph).data }, count);
            if (!palette.length) info(t('pixel.sc_palette_empty', 'There are no visible colors to extract.'));
            return palette;
        }

        // A new layer above the active one, painted by paint(ctx, width, height).
        function addGeneratedLayer(name, paint) {
            if (!r.imgWidth) return false;
            if (r.layers.length >= r.MAX_LAYERS) { failure(t('pixel.max_layers', 'The layer limit is reached.')); return false; }
            r.commitPendingPreviews();
            const layer = r.createLayer({ name });
            paint(layer.canvas.getContext('2d'), r.imgWidth, r.imgHeight);
            r.layers.splice(r.activeLayerIdx + 1, 0, layer);
            r.activeLayerIdx = r.activeLayerIdx + 1;
            r.editMaskMode = false;
            r.layersStructureChanged();
            r.pushHistory(name);
            return true;
        }

        function lightness(c) {
            return lib().rgbToLab(c.r, c.g, c.b)[0];
        }

        // The palette as a horizontal gradient layer (dark to light). The stops stay on the runtime for other modules.
        function paletteToGradientLayer(palette) {
            const list = (palette || []).slice().sort((a, b) => lightness(a) - lightness(b));
            if (!list.length) return false;
            const stops = list.map((c, i) => ({ pos: list.length === 1 ? 0 : i / (list.length - 1), color: c.hex }));
            r.paletteGradient = stops;
            return addGeneratedLayer(t('pixel.sc_gradient_layer', 'Palette gradient'), (ctx, w, h) => {
                const g = ctx.createLinearGradient(0, 0, w, 0);
                stops.forEach(stop => g.addColorStop(stop.pos, stop.color));
                ctx.fillStyle = g;
                ctx.fillRect(0, 0, w, h);
            });
        }

        // The palette as a row of colour chips in a new layer.
        function paletteToLayer(palette) {
            const list = palette || [];
            if (!list.length) return false;
            return addGeneratedLayer(t('pixel.sc_palette_layer', 'Palette'), (ctx, w, h) => {
                const size = clampNum(Math.floor(Math.min(w / (list.length + 1), h / 3)), 8, 160);
                const gap = Math.max(2, Math.round(size / 8));
                const left = Math.round((w - list.length * size - (list.length - 1) * gap) / 2);
                list.forEach((c, i) => {
                    ctx.fillStyle = c.hex;
                    ctx.fillRect(Math.max(0, left) + i * (size + gap), Math.round(size / 2), size, size);
                });
            });
        }

        Object.assign(r, {
            contentAwareFill: Pixel.bindRuntime(r, contentAwareFill),
            showContentAwareFillDialog: Pixel.bindRuntime(r, showContentAwareFillDialog),
            finishRemoveObject: Pixel.bindRuntime(r, finishRemoveObject),
            imageFromCanvasRegion: Pixel.bindRuntime(r, imageFromCanvasRegion),
            cancelSmartJob: Pixel.bindRuntime(r, function cancelSmartJob() { if (smart.cancel) smart.cancel(); }),
            trimTransparent: Pixel.bindRuntime(r, trimTransparent),
            autoCropToContent: Pixel.bindRuntime(r, autoCropToContent),
            fitCanvasToLayers: Pixel.bindRuntime(r, fitCanvasToLayers),
            resizeCanvas: Pixel.bindRuntime(r, resizeCanvas),
            showCanvasSizeDialog: Pixel.bindRuntime(r, showCanvasSizeDialog),
            addSwatch: Pixel.bindRuntime(r, addSwatch),
            extractDocumentPalette: Pixel.bindRuntime(r, extractDocumentPalette),
            paletteToGradientLayer: Pixel.bindRuntime(r, paletteToGradientLayer),
            paletteToLayer: Pixel.bindRuntime(r, paletteToLayer)
        });
    });

    // --- registrations shared by every window ---

    // The colour panel replaces the built-in one (same id: order, tab label and dock are inherited).
    Pixel.registerPanel({
        id: 'colors',
        build: r => buildColorsPanel(r),
        mount: (r, el) => mountColorsPanel(r, el),
        onShow: r => { if (r.smartState && r.smartState.refreshColors) r.smartState.refreshColors(); }
    });

    const hasImage = r => !!r.imgWidth;

    Pixel.registerMenuItems('edit', [
        { id: 'content-aware-fill', labelKey: 'pixel.content_aware_fill_item', fallback: 'Content-aware fill…', icon: 'image', shortcut: 'Shift+Backspace',
            keywords: ['remove object', 'inpaint', 'heal', 'inhaltsbasiert', 'objekt entfernen'],
            action: r => r.showContentAwareFillDialog(), enabled: r => hasImage(r) && !!r.selection }
    ], { order: 50 });

    Pixel.registerMenuItems('image', [
        { id: 'trim', labelKey: 'pixel.trim', fallback: 'Trim transparent pixels', icon: 'scissors', keywords: ['crop', 'zuschneiden', 'transparent', 'rand'], action: r => r.trimTransparent(), enabled: hasImage },
        { id: 'auto-crop', labelKey: 'pixel.auto_crop', fallback: 'Auto-crop to content', icon: 'scissors', keywords: ['crop', 'zuschneiden', 'trim', 'background', 'hintergrund'], action: r => r.autoCropToContent(), enabled: hasImage },
        { id: 'canvas-size', labelKey: 'pixel.canvas_size_item', fallback: 'Canvas size…', icon: 'maximize', shortcut: 'Ctrl+Alt+C', keywords: ['arbeitsfläche', 'leinwand', 'extend', 'erweitern'], action: r => r.showCanvasSizeDialog(), enabled: hasImage },
        { id: 'fit-canvas-layers', labelKey: 'pixel.fit_canvas_layers', fallback: 'Fit canvas to layers', icon: 'maximize', keywords: ['crop', 'trim', 'ebenen'], action: r => r.fitCanvasToLayers(), enabled: hasImage }
    ], { order: 20 });

    Pixel.registerCommand({ id: 'color.extract-palette', labelKey: 'pixel.cmd_extract_palette', fallback: 'Extract palette from image', group: 'colors',
        keywords: ['palette', 'swatches', 'farbpalette', 'dominant colors'],
        run: r => { r.showPanel('colors'); if (r.smartState.extractPalette) r.smartState.extractPalette(); },
        enabled: hasImage });
    Pixel.registerCommand({ id: 'color.add-swatch', labelKey: 'pixel.cmd_add_swatch', fallback: 'Save primary color as swatch', group: 'colors',
        keywords: ['swatch', 'farbfeld', 'save color'], run: r => r.addSwatch(r.primaryColor) });
    Pixel.registerCommand({ id: 'color.swap', labelKey: 'pixel.swap_colors', fallback: 'Swap colors', group: 'colors',
        keywords: ['primary', 'secondary', 'vordergrund', 'hintergrund'], run: r => r.swapColors() });
})();
