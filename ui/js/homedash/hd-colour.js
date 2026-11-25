// HomeDash colour (pure): sRGB/OKLCH conversion, WCAG contrast and the custom palette solver (visual part 1.5).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-colour.js: load hd-core.js first');
    const clamp = E.core.clamp;
    const PALETTE_TOKENS = Object.freeze(['canvas', 'canvas-2', 'surface-solid', 'surface-raised', 'sunken', 'field', 'ink', 'ink-muted', 'ink-faint', 'ink-on-accent', 'accent', 'on', 'on-ink', 'selected', 'selected-ink', 'btn', 'btn-ink', 'hairline', 'focus', 'ok', 'warn', 'alert', 'info', 'heat', 'cool', 'edge-tint', 'shadow-ink']);
    const NEUTRAL = Object.freeze({
        dark: { ok: '#4AD29C', warn: '#F2B84B', alert: '#FF7A72', info: '#5AA9FF', heat: '#FF9A57', cool: '#5CC8FF', 'edge-tint': '#FFFFFF', 'shadow-ink': '#000000' },
        light: { ok: '#1A7D52', warn: '#9A6200', alert: '#C4352B', info: '#1F63C7', heat: '#D9622B', cool: '#1E8FCB', 'edge-tint': '#FFFFFF', 'shadow-ink': '#1B2333' }
    });
    function hexToRgb(hex) { const m = /^#?([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(String(hex || '').trim()); if (!m) return null;
        const h = m[1].length === 3 ? m[1].replace(/./g, c => c + c) : m[1]; return [0, 2, 4].map(i => parseInt(h.slice(i, i + 2), 16)); }
    const rgbToHex = rgb => '#' + rgb.map(v => clamp(Math.round(v), 0, 255).toString(16).padStart(2, '0')).join('').toUpperCase();
    const toLinear = c => (c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4));
    const toGamma = c => (c <= 0.0031308 ? 12.92 * c : 1.055 * Math.pow(c, 1 / 2.4) - 0.055);
    function rgbToOklch(rgb) {
        const [r, g, b] = rgb.map(v => toLinear(v / 255));
        const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b), m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b), s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
        const L = 0.2104542553 * l + 0.7936177850 * m - 0.0040720468 * s, A = 1.9779984951 * l - 2.4285922050 * m + 0.4505937099 * s, B = 0.0259040371 * l + 0.7827717662 * m - 0.8086757660 * s;
        const C = Math.sqrt(A * A + B * B); let H = Math.atan2(B, A) * 180 / Math.PI; if (H < 0) H += 360;
        return [L, C, C < 1e-6 ? 0 : H];
    }
    function oklchToLinear([L, C, H]) {
        const a = C * Math.cos(H * Math.PI / 180), b = C * Math.sin(H * Math.PI / 180);
        const l = Math.pow(L + 0.3963377774 * a + 0.2158037573 * b, 3), m = Math.pow(L - 0.1055613458 * a - 0.0638541728 * b, 3), s = Math.pow(L - 0.0894841775 * a - 1.2914855480 * b, 3);
        return [4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s, -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s, -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s];
    }
    const inGamut = lin => lin.every(v => v >= -1e-4 && v <= 1 + 1e-4);
    function oklchToRgb(lch) {
        const L = clamp(lch[0], 0, 1), H = lch[2]; let C = Math.max(0, lch[1]);
        if (!inGamut(oklchToLinear([L, C, H]))) { let lo = 0, hi = C; for (let i = 0; i < 24; i++) { const mid = (lo + hi) / 2; if (inGamut(oklchToLinear([L, mid, H]))) lo = mid; else hi = mid; } C = lo; }
        return oklchToLinear([L, C, H]).map(v => toGamma(clamp(v, 0, 1)) * 255);
    }
    const oklchToHex = lch => rgbToHex(oklchToRgb(lch));
    function luminance(rgb) { const f = v => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); }; return 0.2126 * f(rgb[0]) + 0.7152 * f(rgb[1]) + 0.0722 * f(rgb[2]); }
    function contrast(a, b) { const x = luminance(typeof a === 'string' ? hexToRgb(a) : a), y = luminance(typeof b === 'string' ? hexToRgb(b) : b); return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05); }
    function stepUntil(lch, step, ok) { const cur = lch.slice(); let hex = oklchToHex(cur);
        for (let i = 0; i < 60 && !ok(hex); i++) { cur[0] = clamp(cur[0] + step, 0, 1); hex = oklchToHex(cur); } return hex; }
    function solvePalette(input) {
        const scheme = input && input.scheme === 'light' ? 'light' : 'dark', dark = scheme === 'dark', away = dark ? 1 : -1;
        const hue = (((Number(input && input.surface_hue) || 0) % 360) + 360) % 360, surfL = dark ? 0.22 : 0.985;
        const accentRgb = hexToRgb(input && input.accent) || hexToRgb(dark ? '#7AA7FF' : '#2F6FE0');
        const tone = (L, C) => oklchToHex([clamp(L, 0, 1), C === undefined ? 0.012 : C, hue]), t = {};
        // 1: surfaces; canvas and sunken are darker than the surface in both schemes
        t['surface-solid'] = tone(surfL); t.canvas = tone(surfL - 0.04); t['canvas-2'] = tone(surfL - (dark ? 0.01 : 0.07));
        t.sunken = tone(surfL - 0.07); t.field = t.sunken; t['surface-raised'] = dark ? tone(surfL + 0.03) : t['surface-solid'];
        // 2: ink, then muted walked toward the surface while 5.0:1 (surface) and 4.5:1 (canvas) hold
        const inkL = dark ? 0.96 : 0.18; t.ink = tone(inkL, 0.01);
        let mutedL = inkL, muted = t.ink;
        for (let i = 0; i < 100; i++) { const L = mutedL - away * 0.01, c = tone(L, 0.01); if (contrast(c, t['surface-solid']) < 5 || contrast(c, t.canvas) < 4.5) break; mutedL = L; muted = c; }
        t['ink-muted'] = muted; t['ink-faint'] = tone((mutedL + surfL) / 2, 0.01); t.hairline = tone(surfL + away * 0.12, 0.01);
        // 3: on = accent, on-ink = black or white, on stepped to 4.5:1
        const accentLch = rgbToOklch(accentRgb), onInk = contrast(accentRgb, [255, 255, 255]) >= contrast(accentRgb, [0, 0, 0]) ? '#FFFFFF' : '#000000';
        t['on-ink'] = onInk; t.on = stepUntil(accentLch, onInk === '#FFFFFF' ? -0.02 : 0.02, h => contrast(h, onInk) >= 4.5);
        // 4: accent as a graphic, 3:1 on the surface
        t.accent = stepUntil(accentLch, away * 0.02, h => contrast(h, t['surface-solid']) >= 3);
        t['ink-on-accent'] = contrast(t.accent, '#FFFFFF') >= contrast(t.accent, '#000000') ? '#FFFFFF' : '#000000'; t.focus = t.accent;
        t.selected = oklchToHex([clamp(surfL + away * 0.08, 0, 1), Math.min(accentLch[1], 0.06), accentLch[2]]); t['selected-ink'] = t.ink;
        t.btn = tone(surfL + away * 0.05); t['btn-ink'] = t.ink;
        // 5: status colours from the neutral set, stepped to 4.5:1 on the surface
        for (const n of ['ok', 'warn', 'alert', 'info']) t[n] = stepUntil(rgbToOklch(hexToRgb(NEUTRAL[scheme][n])), away * 0.02, h => contrast(h, t['surface-solid']) >= 4.5);
        for (const n of ['heat', 'cool', 'edge-tint', 'shadow-ink']) t[n] = NEUTRAL[scheme][n];
        return { scheme, tokens: t };
    }
    // HUD ink for the Instrument family: a light, readable tint of the accent on the near-black panel (#090c10). Monochrome accents
    // (OKLCH chroma < 0.04) become white phosphor. Otherwise lightness starts at 0.72 and rises in 0.02 steps (cap 0.97) until the
    // contrast is 4.5:1; chroma drops in 0.01 steps while a step leaves the sRGB gamut. Pure; returns lowercase #rrggbb.
    const HUD_PANEL = '#090c10', HUD_MONO = '#e8edf2';
    function hudFor(hex) {
        const rgb = hexToRgb(hex); if (!rgb) return HUD_MONO;
        const lch = rgbToOklch(rgb); if (lch[1] < 0.04) return HUD_MONO;
        let L = Math.max(lch[0], 0.72), out = HUD_MONO;
        for (;;) {
            let C = lch[1]; while (C > 0 && !inGamut(oklchToLinear([L, C, lch[2]]))) C = Math.max(0, Math.round((C - 0.01) * 1e6) / 1e6);
            out = rgbToHex(oklchToRgb([L, C, lch[2]])).toLowerCase();
            if (contrast(out, HUD_PANEL) >= 4.5 || L >= 0.97) return out;
            L = Math.min(0.97, Math.round((L + 0.02) * 1e6) / 1e6);
        }
    }
    // Contrast-safe graphic colour: `hex` is returned unchanged (lowercase #rrggbb) when it already reaches `min` against every background in
    // `against` (one hex or an array). Otherwise OKLCH lightness moves in 0.01 steps toward whichever extreme (black or white) gives the better
    // worst-case contrast, hue and chroma kept (chroma shrinks only where the step leaves the sRGB gamut, inside oklchToRgb). If no lightness
    // reaches `min`, the best extreme is returned. Unparseable input comes back unchanged. Pure.
    function ensureContrast(hex, against, min) {
        const rgb = hexToRgb(hex), bgs = (Array.isArray(against) ? against : [against]).map(hexToRgb).filter(Boolean);
        if (!rgb || !bgs.length) return rgb ? rgbToHex(rgb).toLowerCase() : String(hex);
        const worst = h => Math.min(...bgs.map(b => contrast(h, b))), lch = rgbToOklch(rgb), start = rgbToHex(rgb).toLowerCase();
        if (worst(start) >= min) return start;
        const toward = dir => oklchToHex([dir, lch[1], lch[2]]).toLowerCase(), dark = toward(0), light = toward(1);
        const dir = worst(dark) >= worst(light) ? -1 : 1, extreme = dir < 0 ? dark : light;
        let L = lch[0];
        for (let i = 0; i < 100; i++) {
            L = clamp(Math.round((L + dir * 0.01) * 1e6) / 1e6, 0, 1);
            const out = oklchToHex([L, lch[1], lch[2]]).toLowerCase();
            if (worst(out) >= min) return out;
            if (L <= 0 || L >= 1) break;
        }
        return extreme;
    }
    const applyPalette = (el, tokens) => E.core.setVars(el, Object.fromEntries(PALETTE_TOKENS.map(n => ['--hd-' + n, tokens[n]])));
    const clearPalette = el => E.core.setVars(el, Object.fromEntries(PALETTE_TOKENS.map(n => ['--hd-' + n, null])));
    function hsvToRgb(h, s, v) { const f = n => { const k = (n + h / 60) % 6; return (v - v * s * Math.max(0, Math.min(k, 4 - k, 1))) * 255; }; return [f(5), f(3), f(1)]; }
    E.colour = Object.freeze({ PALETTE_TOKENS, NEUTRAL, hexToRgb, rgbToHex, rgbToOklch, oklchToRgb, oklchToHex, luminance, contrast, solvePalette, applyPalette, clearPalette, hsvToRgb, hudFor, ensureContrast });
})();
