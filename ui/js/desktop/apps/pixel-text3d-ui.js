// Pixel inspector panel "3D Text" (id: 'text3d', dock: 'inspector').
// Provides text & font controls, interactive 3D trackball & rotation sliders, 12 warp modes with visual icons,
// 18 live procedural texture swatches, style/angle presets, lighting & drop shadow controls, and Place/Cancel actions.
(function () {
    'use strict';

    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};
    const lib = () => window.AuraPixelLib && window.AuraPixelLib.text3d;
    const api = () => Pixel.text3d;

    const WARP_SVGS = {
        none: '<path d="M3 10h14"/>',
        arc: '<path d="M3 13Q10 3 17 13"/>',
        arch: '<path d="M3 12Q10 4 17 12M3 15Q10 7 17 15" opacity=".75"/>',
        wave: '<path d="M2 10Q6 4 10 10T18 10"/>',
        flag: '<path d="M3 7Q7 3 11 7T17 7v6Q13 9 9 13T3 13z"/>',
        bulge: '<path d="M3 7Q10 2 17 7M3 13Q10 18 17 13"/>',
        cylinder: '<ellipse cx="10" cy="6" rx="7" ry="2.2"/><path d="M3 6v8c0 1.2 3.1 2.2 7 2.2s7-1 7-2.2V6"/>',
        twist: '<path d="M3 7c4 0 5 6 7 6s3-6 7-6M3 13c4 0 5-6 7-6s3 6 7 6"/>',
        bridge: '<path d="M3 6h14M3 15Q10 9 17 15"/>',
        wedge: '<path d="M3 8l14-3v10L3 12z"/>',
        rise: '<path d="M3 14L17 6"/>',
        dome: '<path d="M3 13A7 7 0 0117 13"/><ellipse cx="10" cy="13" rx="7" ry="2" opacity=".6"/>'
    };

    function sliderRow(r, key, labelKey, min, max, step, val, unit) {
        const suffix = unit || '';
        const disp = step && step < 1 ? Number(val).toFixed(2) : Math.round(Number(val));
        return '<div class="pixel-slider-row">' +
            '<span class="pixel-slider-label">' + r.esc(r.t(labelKey)) + '</span>' +
            '<input type="range" class="pixel-slider" data-t3d-range="' + key + '" min="' + min + '" max="' + max + '" step="' + (step || 1) + '" value="' + val + '">' +
            '<span class="pixel-slider-value" data-t3d-val="' + key + '" data-t3d-unit="' + r.esc(suffix) + '">' + disp + r.esc(suffix) + '</span>' +
            '</div>';
    }

    function buildPanelHTML(r) {
        const T = lib(), M = api();
        if (!T || !M) return '';
        const st = M.stateOf(r), p = st.params;
        const presets = T.STYLE_PRESETS.map(pr =>
            '<button type="button" class="pixel-chip" data-t3d-preset="' + r.esc(pr.id) + '">' + r.esc(r.t(pr.labelKey)) + '</button>'
        ).join('');
        const fonts = T.FONTS.map(f =>
            '<option value="' + r.esc(f.id) + '"' + (f.id === p.font ? ' selected' : '') + ' style="font-family:' + r.esc(f.stack) + '">' + r.esc(f.name) + '</option>'
        ).join('');
        const angles = T.ANGLE_PRESETS.map(a =>
            '<button type="button" class="pixel-chip" data-t3d-angle="' + r.esc(a.id) + '">' + r.esc(r.t(a.labelKey)) + '</button>'
        ).join('');
        const bevels = T.BEVELS.map(b =>
            '<button type="button" class="pixel-param-seg-btn' + (b.id === p.bevelStyle ? ' active' : '') + '" data-t3d-bevel="' + r.esc(b.id) + '" aria-pressed="' + (b.id === p.bevelStyle ? 'true' : 'false') + '">' + r.esc(r.t(b.labelKey)) + '</button>'
        ).join('');
        const warps = T.WARPS.map(w =>
            '<button type="button" class="pixel-t3d-warp-btn' + (w.id === p.warp ? ' active' : '') + '" data-t3d-warp="' + r.esc(w.id) + '" aria-pressed="' + (w.id === p.warp ? 'true' : 'false') + '" title="' + r.esc(r.t(w.labelKey)) + '">' +
            '<svg class="pixel-t3d-warp-svg" width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + (WARP_SVGS[w.id] || '') + '</svg>' +
            '<span class="pixel-t3d-warp-name">' + r.esc(r.t(w.labelKey)) + '</span></button>'
        ).join('');
        const textures = T.TEXTURES.map(tx =>
            '<button type="button" class="pixel-t3d-tex-card' + (tx.id === p.texture ? ' active' : '') + '" data-t3d-tex="' + r.esc(tx.id) + '" aria-pressed="' + (tx.id === p.texture ? 'true' : 'false') + '" title="' + r.esc(r.t(tx.labelKey)) + '">' +
            '<canvas class="pixel-t3d-tex-swatch" width="44" height="44" data-t3d-swatch="' + r.esc(tx.id) + '" aria-hidden="true"></canvas>' +
            '<span class="pixel-t3d-tex-name">' + r.esc(r.t(tx.labelKey)) + '</span></button>'
        ).join('');

        return '<div class="pixel-t3d-panel" data-t3d-panel>' +
            '<section class="pixel-t3d-sec">' +
                '<div class="pixel-t3d-sec-head">' + r.esc(r.t('pixel.text3d_sec_presets')) + '</div>' +
                '<div class="pixel-filter-chips" data-t3d-presets>' + presets + '</div>' +
            '</section>' +
            '<section class="pixel-t3d-sec">' +
                '<div class="pixel-t3d-sec-head">' + r.esc(r.t('pixel.text3d_sec_text')) + '</div>' +
                '<textarea class="pixel-t3d-textarea" data-t3d-text rows="2" maxlength="240" spellcheck="false" placeholder="' + r.esc(r.t('pixel.text3d_placeholder')) + '" aria-label="' + r.esc(r.t('pixel.text3d_text')) + '">' + r.esc(p.text) + '</textarea>' +
                '<div class="pixel-t3d-font-row">' +
                    '<select class="pixel-select" data-t3d-font aria-label="' + r.esc(r.t('pixel.font_family')) + '">' + fonts + '</select>' +
                    '<button type="button" class="pixel-btn pixel-t3d-style-btn' + (p.bold ? ' active' : '') + '" data-t3d-toggle="bold" aria-pressed="' + (p.bold ? 'true' : 'false') + '" title="' + r.esc(r.t('pixel.text3d_bold')) + '">B</button>' +
                    '<button type="button" class="pixel-btn pixel-t3d-style-btn' + (p.italic ? ' active' : '') + '" data-t3d-toggle="italic" aria-pressed="' + (p.italic ? 'true' : 'false') + '" title="' + r.esc(r.t('pixel.text3d_italic')) + '"><em>I</em></button>' +
                '</div>' +
                '<div class="pixel-param-segmented" role="group" aria-label="' + r.esc(r.t('pixel.text3d_align')) + '">' +
                    ['left', 'center', 'right'].map(a =>
                        '<button type="button" class="pixel-param-seg-btn' + (p.align === a ? ' active' : '') + '" data-t3d-align="' + a + '" aria-pressed="' + (p.align === a ? 'true' : 'false') + '">' + r.esc(r.t('pixel.text3d_align_' + a)) + '</button>'
                    ).join('') +
                '</div>' +
                sliderRow(r, 'fontSize', 'pixel.font_size', 16, 600, 1, p.fontSize, 'px') +
                sliderRow(r, 'letterSpacing', 'pixel.text3d_spacing', -15, 60, 1, p.letterSpacing, '') +
                sliderRow(r, 'lineHeight', 'pixel.text3d_line_height', 0.85, 2.0, 0.05, p.lineHeight, '') +
            '</section>' +
            '<section class="pixel-t3d-sec">' +
                '<div class="pixel-t3d-sec-head">' + r.esc(r.t('pixel.text3d_sec_space')) + '</div>' +
                '<div class="pixel-t3d-orbit-row">' +
                    '<canvas class="pixel-t3d-trackball" data-t3d-trackball width="108" height="108" role="img" aria-label="' + r.esc(r.t('pixel.text3d_trackball')) + '" title="' + r.esc(r.t('pixel.text3d_trackball_hint')) + '"></canvas>' +
                    '<div class="pixel-t3d-orbit-side">' +
                        '<div class="pixel-t3d-readout" data-t3d-readout></div>' +
                        '<div class="pixel-filter-chips" data-t3d-angles>' + angles + '</div>' +
                    '</div>' +
                '</div>' +
                sliderRow(r, 'yaw', 'pixel.text3d_yaw', -180, 180, 1, p.yaw, '°') +
                sliderRow(r, 'pitch', 'pixel.text3d_pitch', -85, 85, 1, p.pitch, '°') +
                sliderRow(r, 'roll', 'pixel.text3d_roll', -180, 180, 1, p.roll, '°') +
                sliderRow(r, 'perspective', 'pixel.text3d_perspective', 0, 100, 1, p.perspective, '%') +
                sliderRow(r, 'depth', 'pixel.text3d_depth', 0, 100, 1, p.depth, '') +
                sliderRow(r, 'bevel', 'pixel.text3d_bevel', 0, 100, 1, p.bevel, '%') +
                '<div class="pixel-param-segmented" role="group" aria-label="' + r.esc(r.t('pixel.text3d_bevel_style')) + '">' + bevels + '</div>' +
                '<div class="pixel-t3d-pos-row">' +
                    '<div class="pixel-t3d-pos-sliders">' +
                        sliderRow(r, 'posX', 'pixel.text3d_pos_x', 0, 100, 1, Math.round(p.posX * 100), '%') +
                        sliderRow(r, 'posY', 'pixel.text3d_pos_y', 0, 100, 1, Math.round(p.posY * 100), '%') +
                    '</div>' +
                    '<button type="button" class="pixel-btn pixel-t3d-center-btn" data-t3d-center title="' + r.esc(r.t('pixel.text3d_center')) + '">' + r.esc(r.t('pixel.text3d_center')) + '</button>' +
                '</div>' +
            '</section>' +
            '<section class="pixel-t3d-sec">' +
                '<div class="pixel-t3d-sec-head">' + r.esc(r.t('pixel.text3d_sec_warp')) + '</div>' +
                '<div class="pixel-t3d-warp-grid" role="group" aria-label="' + r.esc(r.t('pixel.text3d_warp')) + '">' + warps + '</div>' +
                '<div class="pixel-t3d-warp-opts" data-t3d-warp-opts' + (p.warp === 'none' ? ' hidden' : '') + '>' +
                    sliderRow(r, 'warpStrength', 'pixel.text3d_warp_strength', -100, 100, 1, p.warpStrength, '%') +
                    '<div data-t3d-warp-freq-wrap' + (T.warpById(p.warp).hasFreq ? '' : ' hidden') + '>' +
                        sliderRow(r, 'warpFreq', 'pixel.text3d_warp_freq', 0.5, 5, 0.25, p.warpFreq, '') +
                    '</div>' +
                '</div>' +
            '</section>' +
            '<section class="pixel-t3d-sec">' +
                '<div class="pixel-t3d-sec-head">' + r.esc(r.t('pixel.text3d_sec_texture')) + '</div>' +
                '<div class="pixel-t3d-tex-grid" role="group" aria-label="' + r.esc(r.t('pixel.text3d_sec_texture')) + '">' + textures + '</div>' +
                '<div class="pixel-t3d-colors">' +
                    '<label class="pixel-t3d-color-field"><span>' + r.esc(r.t('pixel.text3d_color')) + '</span><input type="color" class="pixel-t3d-color" data-t3d-color="color" value="' + r.esc(p.color) + '"></label>' +
                    '<label class="pixel-t3d-color-field"><span>' + r.esc(r.t('pixel.text3d_side_color')) + '</span><input type="color" class="pixel-t3d-color" data-t3d-color="sideColor" value="' + r.esc(p.sideColor) + '"></label>' +
                '</div>' +
                '<label class="pixel-checkbox-row"><input type="checkbox" data-t3d-check="sideMatchTexture"' + (p.sideMatchTexture ? ' checked' : '') + '> ' + r.esc(r.t('pixel.text3d_side_match')) + '</label>' +
                sliderRow(r, 'texScale', 'pixel.text3d_tex_scale', 25, 300, 5, p.texScale, '%') +
                sliderRow(r, 'texRotate', 'pixel.text3d_tex_rotate', 0, 360, 1, p.texRotate, '°') +
            '</section>' +
            '<details class="pixel-t3d-details">' +
                '<summary class="pixel-t3d-summary">' + r.esc(r.t('pixel.text3d_sec_light')) + '</summary>' +
                '<div class="pixel-t3d-details-body">' +
                    sliderRow(r, 'lightAzimuth', 'pixel.text3d_light_angle', -180, 180, 1, p.lightAzimuth, '°') +
                    sliderRow(r, 'lightElevation', 'pixel.text3d_light_height', 5, 88, 1, p.lightElevation, '°') +
                    sliderRow(r, 'gloss', 'pixel.text3d_gloss', 0, 100, 1, p.gloss, '%') +
                    sliderRow(r, 'metalness', 'pixel.text3d_metalness', 0, 100, 1, p.metalness, '%') +
                    sliderRow(r, 'bumpStrength', 'pixel.text3d_bump', 0, 100, 1, p.bumpStrength, '%') +
                    '<label class="pixel-checkbox-row"><input type="checkbox" data-t3d-check="shadow"' + (p.shadow ? ' checked' : '') + '> ' + r.esc(r.t('pixel.text3d_shadow')) + '</label>' +
                    '<div data-t3d-shadow-opts' + (p.shadow ? '' : ' hidden') + '>' +
                        sliderRow(r, 'shadowOpacity', 'pixel.text3d_shadow_opacity', 0, 100, 1, p.shadowOpacity, '%') +
                        sliderRow(r, 'shadowBlur', 'pixel.text3d_shadow_blur', 0, 100, 1, p.shadowBlur, '%') +
                    '</div>' +
                '</div>' +
            '</details>' +
            '<div class="pixel-t3d-actions">' +
                '<button type="button" class="pixel-btn pixel-btn-primary" data-t3d-apply>' + r.esc(r.t('pixel.text3d_place')) + '</button>' +
                '<button type="button" class="pixel-btn" data-t3d-reset>' + r.esc(r.t('pixel.reset')) + '</button>' +
                '<button type="button" class="pixel-btn" data-t3d-cancel>' + r.esc(r.t('pixel.cancel')) + '</button>' +
            '</div>' +
            '<p class="pixel-draw-hint">' + r.esc(r.t('pixel.text3d_canvas_hint')) + '</p>' +
        '</div>';
    }

    // --- interactive mini 3D trackball widget ---

    function drawTrackball(canvas, p) {
        if (!canvas || !lib()) return;
        const ctx = canvas.getContext('2d');
        if (!ctx) return;
        const w = canvas.width, h = canvas.height, cx = w / 2, cy = h / 2, rad = Math.min(w, h) * 0.41;
        ctx.clearRect(0, 0, w, h);

        // Sphere backdrop
        const grad = ctx.createRadialGradient(cx - rad * 0.32, cy - rad * 0.32, rad * 0.12, cx, cy, rad);
        grad.addColorStop(0, 'rgba(130, 195, 255, 0.26)');
        grad.addColorStop(0.7, 'rgba(38, 62, 104, 0.24)');
        grad.addColorStop(1, 'rgba(12, 18, 32, 0.45)');
        ctx.beginPath();
        ctx.arc(cx, cy, rad, 0, Math.PI * 2);
        ctx.fillStyle = grad;
        ctx.fill();
        ctx.strokeStyle = 'rgba(110, 195, 255, 0.55)';
        ctx.lineWidth = 1.25;
        ctx.stroke();

        const T = lib();
        const R = T.rotationMatrix(p.pitch, p.yaw, p.roll);
        const proj = (x, y, z) => {
            const v = T.mulMat3Vec(R, x, y, z);
            return { x: cx + v[0] * rad, y: cy - v[1] * rad, z: v[2] };
        };

        // Equator & meridian rings
        const ring = (pts, color) => {
            ctx.beginPath();
            for (let i = 0; i < pts.length; i++) (i ? ctx.lineTo(pts[i].x, pts[i].y) : ctx.moveTo(pts[i].x, pts[i].y));
            ctx.closePath();
            ctx.strokeStyle = color;
            ctx.lineWidth = 1;
            ctx.stroke();
        };
        const eq = [], mer = [];
        for (let i = 0; i < 24; i++) {
            const a = (i / 24) * Math.PI * 2;
            eq.push(proj(Math.cos(a), 0, Math.sin(a)));
            mer.push(proj(0, Math.cos(a), Math.sin(a)));
        }
        ring(eq, 'rgba(130, 210, 255, 0.28)');
        ring(mer, 'rgba(130, 210, 255, 0.22)');

        // Mini tilted 3D text plaque inside the sphere
        const plaque = [proj(-0.62, 0.28, 0.14), proj(0.62, 0.28, 0.14), proj(0.62, -0.28, 0.14), proj(-0.62, -0.28, 0.14)];
        const backP = [proj(-0.62, 0.28, -0.14), proj(0.62, 0.28, -0.14), proj(0.62, -0.28, -0.14), proj(-0.62, -0.28, -0.14)];
        ctx.beginPath();
        for (let i = 0; i < 4; i++) { ctx.moveTo(backP[i].x, backP[i].y); ctx.lineTo(plaque[i].x, plaque[i].y); }
        ctx.strokeStyle = 'rgba(255, 200, 90, 0.45)';
        ctx.lineWidth = 1.2;
        ctx.stroke();
        ctx.beginPath();
        plaque.forEach((pt, i) => (i ? ctx.lineTo(pt.x, pt.y) : ctx.moveTo(pt.x, pt.y)));
        ctx.closePath();
        ctx.fillStyle = 'rgba(86, 212, 255, 0.34)';
        ctx.fill();
        ctx.strokeStyle = '#56d4ff';
        ctx.lineWidth = 1.4;
        ctx.stroke();

        // Front normal dot
        const tip = proj(0, 0, 0.92);
        ctx.beginPath();
        ctx.moveTo(cx, cy); ctx.lineTo(tip.x, tip.y);
        ctx.strokeStyle = tip.z >= 0 ? '#ffd166' : 'rgba(255, 209, 102, 0.4)';
        ctx.lineWidth = 1.5;
        ctx.stroke();
        ctx.beginPath();
        ctx.arc(tip.x, tip.y, 4, 0, Math.PI * 2);
        ctx.fillStyle = tip.z >= 0 ? '#ffd166' : '#7b8aa8';
        ctx.fill();
    }

    function paintSwatches(el, st) {
        const M = api();
        if (!M || typeof ImageData !== 'function') return;
        el.querySelectorAll('[data-t3d-swatch]').forEach(cv => {
            const id = cv.dataset.t3dSwatch;
            const img = M.getSwatchData(id, 44, st.params);
            const ctx = cv.getContext('2d');
            if (ctx && img && img.data) {
                try { ctx.putImageData(new ImageData(img.data, img.width, img.height), 0, 0); } catch (_) { /* fake DOM */ }
            }
        });
    }

    function syncPanel(el, r, st) {
        const T = lib();
        if (!el || !T) return;
        const p = st.params;
        const txt = el.querySelector('[data-t3d-text]');
        if (txt && document.activeElement !== txt) txt.value = p.text;
        const font = el.querySelector('[data-t3d-font]');
        if (font) font.value = p.font;
        ['bold', 'italic'].forEach(k => {
            const b = el.querySelector('[data-t3d-toggle="' + k + '"]');
            if (b) { b.classList.toggle('active', !!p[k]); b.setAttribute('aria-pressed', p[k] ? 'true' : 'false'); }
        });
        el.querySelectorAll('[data-t3d-align]').forEach(b => {
            const on = b.dataset.t3dAlign === p.align;
            b.classList.toggle('active', on); b.setAttribute('aria-pressed', on ? 'true' : 'false');
        });
        el.querySelectorAll('[data-t3d-bevel]').forEach(b => {
            const on = b.dataset.t3dBevel === p.bevelStyle;
            b.classList.toggle('active', on); b.setAttribute('aria-pressed', on ? 'true' : 'false');
        });
        el.querySelectorAll('[data-t3d-warp]').forEach(b => {
            const on = b.dataset.t3dWarp === p.warp;
            b.classList.toggle('active', on); b.setAttribute('aria-pressed', on ? 'true' : 'false');
        });
        el.querySelectorAll('[data-t3d-tex]').forEach(b => {
            const on = b.dataset.t3dTex === p.texture;
            b.classList.toggle('active', on); b.setAttribute('aria-pressed', on ? 'true' : 'false');
        });
        const warpOpts = el.querySelector('[data-t3d-warp-opts]');
        if (warpOpts) warpOpts.hidden = p.warp === 'none';
        const freqWrap = el.querySelector('[data-t3d-warp-freq-wrap]');
        if (freqWrap) freqWrap.hidden = !T.warpById(p.warp).hasFreq;
        const shadowOpts = el.querySelector('[data-t3d-shadow-opts]');
        if (shadowOpts) shadowOpts.hidden = !p.shadow;

        el.querySelectorAll('[data-t3d-range]').forEach(inp => {
            const k = inp.dataset.t3dRange;
            const raw = (k === 'posX' || k === 'posY') ? Math.round(p[k] * 100) : p[k];
            inp.value = String(raw);
            const valEl = el.querySelector('[data-t3d-val="' + k + '"]');
            if (valEl) {
                const step = Number(inp.step) || 1;
                const disp = step < 1 ? Number(raw).toFixed(2) : Math.round(Number(raw));
                valEl.textContent = disp + (valEl.dataset.t3dUnit || '');
            }
        });
        ['color', 'sideColor'].forEach(k => {
            const inp = el.querySelector('[data-t3d-color="' + k + '"]');
            if (inp) inp.value = p[k];
        });
        ['sideMatchTexture', 'shadow'].forEach(k => {
            const inp = el.querySelector('[data-t3d-check="' + k + '"]');
            if (inp) inp.checked = !!p[k];
        });
        const readout = el.querySelector('[data-t3d-readout]');
        if (readout) {
            readout.textContent = 'X ' + Math.round(p.pitch) + '° · Y ' + Math.round(p.yaw) + '° · Z ' + Math.round(p.roll) + '°';
        }
        drawTrackball(el.querySelector('[data-t3d-trackball]'), p);
        const applyBtn = el.querySelector('[data-t3d-apply]');
        if (applyBtn) applyBtn.disabled = !!(r.ctx && r.ctx.readonly) || !String(p.text || '').trim();
    }

    function wireTrackball(el, r, st) {
        const canvas = el.querySelector('[data-t3d-trackball]');
        if (!canvas) return;
        const M = api();
        let drag = null;
        canvas.addEventListener('pointerdown', e => {
            if (e.button) return;
            try { canvas.setPointerCapture(e.pointerId); } catch (_) { /* optional */ }
            drag = { x0: e.clientX, y0: e.clientY, yaw0: st.params.yaw, pitch0: st.params.pitch };
            e.preventDefault();
        });
        canvas.addEventListener('pointermove', e => {
            if (!drag) return;
            const yaw = ((drag.yaw0 + (e.clientX - drag.x0) * 1.45 + 540) % 360) - 180;
            const pitch = Math.max(-85, Math.min(85, drag.pitch0 + (e.clientY - drag.y0) * 1.15));
            M.updateParams(r, st, { yaw: Math.round(yaw), pitch: Math.round(pitch) }, { dragging: true });
        });
        const stop = () => {
            if (!drag) return;
            drag = null;
            M.scheduleRender(r, st, { dragging: false });
        };
        canvas.addEventListener('pointerup', stop);
        canvas.addEventListener('pointercancel', stop);
        canvas.addEventListener('dblclick', () => M.updateParams(r, st, { pitch: 0, yaw: 0, roll: 0 }));
    }

    function mount(r, el) {
        const T = lib(), M = api();
        if (!T || !M) return;
        const st = M.stateOf(r);
        st.panelSync = () => syncPanel(el, r, st);

        const stopKeys = e => { e.stopPropagation(); };
        const txt = el.querySelector('[data-t3d-text]');
        if (txt) {
            txt.addEventListener('keydown', stopKeys);
            txt.addEventListener('keyup', stopKeys);
            txt.addEventListener('keypress', stopKeys);
            txt.addEventListener('input', () => M.updateParams(r, st, { text: txt.value }, { fromPanel: true }));
        }
        const font = el.querySelector('[data-t3d-font]');
        if (font) font.addEventListener('change', () => M.updateParams(r, st, { font: font.value }, { fromPanel: true }));

        el.addEventListener('click', e => {
            const t = e.target;
            if (!t || !t.closest) return;
            const preset = t.closest('[data-t3d-preset]');
            if (preset) {
                const pr = T.STYLE_PRESETS.find(item => item.id === preset.dataset.t3dPreset);
                if (pr) {
                    const patch = Object.assign({}, pr.values);
                    const txDef = T.textureById(patch.texture || st.params.texture);
                    patch.metalness = Math.round(txDef.metalness * 100);
                    patch.gloss = Math.round(txDef.gloss * 100);
                    patch.bumpStrength = Math.round(txDef.bump * 100);
                    M.updateParams(r, st, patch);
                    paintSwatches(el, st);
                }
                return;
            }
            const angle = t.closest('[data-t3d-angle]');
            if (angle) {
                const a = T.ANGLE_PRESETS.find(item => item.id === angle.dataset.t3dAngle);
                if (a) M.updateParams(r, st, { pitch: a.pitch, yaw: a.yaw, roll: a.roll, perspective: a.perspective });
                return;
            }
            const tog = t.closest('[data-t3d-toggle]');
            if (tog) {
                const k = tog.dataset.t3dToggle;
                M.updateParams(r, st, { [k]: !st.params[k] });
                return;
            }
            const align = t.closest('[data-t3d-align]');
            if (align) { M.updateParams(r, st, { align: align.dataset.t3dAlign }); return; }
            const bevel = t.closest('[data-t3d-bevel]');
            if (bevel) { M.updateParams(r, st, { bevelStyle: bevel.dataset.t3dBevel }); return; }
            const warp = t.closest('[data-t3d-warp]');
            if (warp) { M.updateParams(r, st, { warp: warp.dataset.t3dWarp }); return; }
            const tex = t.closest('[data-t3d-tex]');
            if (tex) {
                const txDef = T.textureById(tex.dataset.t3dTex);
                M.updateParams(r, st, {
                    texture: txDef.id,
                    metalness: Math.round(txDef.metalness * 100),
                    gloss: Math.round(txDef.gloss * 100),
                    bumpStrength: Math.round(txDef.bump * 100),
                    sideColor: txDef.side || st.params.sideColor
                });
                return;
            }
            if (t.closest('[data-t3d-center]')) { M.updateParams(r, st, { posX: 0.5, posY: 0.5 }); return; }
            if (t.closest('[data-t3d-apply]')) { M.commitSession(r, st); return; }
            if (t.closest('[data-t3d-cancel]')) { M.cancelSession(r, st); return; }
            if (t.closest('[data-t3d-reset]')) {
                const fresh = T.defaultParams({ text: st.params.text });
                M.updateParams(r, st, fresh);
                paintSwatches(el, st);
            }
        });

        el.addEventListener('input', e => {
            const t = e.target;
            if (!t || !t.matches) return;
            if (t.matches('[data-t3d-range]')) {
                const k = t.dataset.t3dRange;
                const val = Number(t.value);
                const next = (k === 'posX' || k === 'posY') ? val / 100 : val;
                const valEl = el.querySelector('[data-t3d-val="' + k + '"]');
                if (valEl) {
                    const step = Number(t.step) || 1;
                    valEl.textContent = (step < 1 ? val.toFixed(2) : Math.round(val)) + (valEl.dataset.t3dUnit || '');
                }
                M.updateParams(r, st, { [k]: next }, { fromPanel: true, dragging: true });
                if (k === 'yaw' || k === 'pitch' || k === 'roll') {
                    const readout = el.querySelector('[data-t3d-readout]');
                    if (readout) readout.textContent = 'X ' + Math.round(st.params.pitch) + '° · Y ' + Math.round(st.params.yaw) + '° · Z ' + Math.round(st.params.roll) + '°';
                    drawTrackball(el.querySelector('[data-t3d-trackball]'), st.params);
                }
            } else if (t.matches('[data-t3d-color]')) {
                const k = t.dataset.t3dColor;
                M.updateParams(r, st, { [k]: t.value }, { fromPanel: true });
                paintSwatches(el, st);
            }
        });

        el.addEventListener('change', e => {
            const t = e.target;
            if (!t || !t.matches) return;
            if (t.matches('[data-t3d-range]')) {
                M.scheduleRender(r, st, { dragging: false });
            } else if (t.matches('[data-t3d-check]')) {
                const k = t.dataset.t3dCheck;
                M.updateParams(r, st, { [k]: !!t.checked });
            }
        });

        wireTrackball(el, r, st);
        paintSwatches(el, st);
        syncPanel(el, r, st);
    }

    Pixel.registerPanel({
        id: 'text3d',
        labelKey: 'pixel.text3d_panel',
        fallback: '3D Text',
        order: 45,
        dock: 'inspector',
        build: r => buildPanelHTML(r),
        mount: (r, el) => mount(r, el),
        onShow: r => {
            const M = api();
            if (!M) return;
            const st = M.stateOf(r);
            if (r.activeTool !== 'text-3d') r.setActiveTool('text-3d');
            else M.activateSession(r, st, { keepSize: true });
        },
        onHide: r => {
            const M = api();
            if (!M) return;
            const st = M.stateOf(r);
            if (st.active) M.cancelSession(r, st);
        }
    });
})();
