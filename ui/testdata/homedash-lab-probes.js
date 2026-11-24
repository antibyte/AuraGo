// HomeDash control lab probes (test fixture, never shipped). hdLabProbes.run() measures the rendered panel (contract C6).
// Plain page script, no imports: the colour maths mirror ui/js/homedash/hd-colour.js on purpose.
(function () {
    'use strict';
    const TEXT_MIN = 4.5, GRAPHIC_MIN = 3, HIT_MIN = 44, EPS = 0.5, WHITE = [255, 255, 255, 1];
    let ctx = null;
    // Colour strings come from getComputedStyle: rgb()/rgba() are parsed exactly; color(srgb ...), oklch() and anything else go through
    // a 1x1 canvas that normalises them to sRGB (an invalid string leaves the transparent default, so it reads as alpha 0).
    const RGB = /^rgba?\(\s*([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)(?:\s*[,/]\s*([\d.]+)(%?))?\s*\)$/;
    function rgba(value) {
        const m = RGB.exec(String(value || '').trim());
        if (m) return [Number(m[1]), Number(m[2]), Number(m[3]), m[4] === undefined ? 1 : m[5] ? Number(m[4]) / 100 : Number(m[4])];
        if (!ctx) { const c = document.createElement('canvas'); c.width = c.height = 1; ctx = c.getContext('2d', { willReadFrequently: true }); }
        ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = 'rgba(0, 0, 0, 0)'; ctx.fillStyle = String(value || ''); ctx.fillRect(0, 0, 1, 1);
        const d = ctx.getImageData(0, 0, 1, 1).data; return [d[0], d[1], d[2], d[3] / 255];
    }
    function over(t, b) { const a = t[3] + b[3] * (1 - t[3]); if (!a) return [0, 0, 0, 0]; return [0, 1, 2].map(i => (t[i] * t[3] + b[i] * b[3] * (1 - t[3])) / a).concat([a]); }
    function lum(c) { const f = v => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); }; return 0.2126 * f(c[0]) + 0.7152 * f(c[1]) + 0.0722 * f(c[2]); }
    function ratio(a, b) { const x = lum(a), y = lum(b); return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05); }
    const show = v => (Math.floor(v * 100) / 100).toFixed(2);
    const dims = b => show(b.width) + 'x' + show(b.height);
    function label(el) { const cell = el.closest('[data-hd-lab-control]'), where = cell ? cell.getAttribute('data-hd-lab-control') + ' ' + cell.getAttribute('data-hd-lab-state') : 'panel';
        return where + ' .' + String(el.getAttribute('class') || el.tagName.toLowerCase()).split(/\s+/)[0]; }
    const exempt = el => !!el.closest('[aria-disabled="true"], [disabled], [data-hd-status="unavailable"], [data-hd-disabled="true"]');
    function backgroundOf(el) { const layers = [];
        for (let n = el; n && n.nodeType === 1; n = n.parentElement) { const cs = getComputedStyle(n); if (cs.backgroundImage && cs.backgroundImage !== 'none') return null;
            const c = rgba(cs.backgroundColor); if (c[3] > 0) { layers.push(c); if (c[3] >= 1) break; } }
        let bg = WHITE; for (let i = layers.length - 1; i >= 0; i--) bg = over(layers[i], bg); return bg; }
    function opacityOf(el) { let o = 1; for (let n = el; n && n.nodeType === 1; n = n.parentElement) { const v = Number(getComputedStyle(n).opacity); o *= Number.isFinite(v) ? v : 1; } return o; }
    const PALETTE = ['--hd-canvas', '--hd-surface-solid', '--hd-sunken', '--hd-ink', '--hd-ink-muted', '--hd-ink-faint', '--hd-accent', '--hd-on', '--hd-on-ink', '--hd-ink-on-accent', '--hd-selected', '--hd-selected-ink', '--hd-btn', '--hd-btn-ink', '--hd-hairline', '--hd-focus', '--hd-ok', '--hd-warn', '--hd-alert', '--hd-info', '--hd-ok-ink', '--hd-warn-ink', '--hd-alert-ink', '--hd-info-ink'];
    const S = '--hd-surface-solid', C = '--hd-canvas';
    const PAIRS = [['--hd-ink', S, 4.5, 't'], ['--hd-ink-muted', S, 4.5, 't'], ['--hd-ink', C, 4.5, 't'], ['--hd-ink-muted', C, 4.5, 't'], ['--hd-on-ink', '--hd-on', 4.5, 't'], ['--hd-ink-on-accent', '--hd-accent', 4.5, 't'],
        ['--hd-selected-ink', '--hd-selected', 4.5, 't'], ['--hd-btn-ink', '--hd-btn', 4.5, 't'], ['--hd-ok-ink', S, 4.5, 't'], ['--hd-warn-ink', S, 4.5, 't'], ['--hd-alert-ink', S, 4.5, 't'], ['--hd-info-ink', S, 4.5, 't'],
        ['--hd-accent', S, 3, 'g'], ['--hd-focus', S, 3, 'g'], ['--hd-ok', S, 3, 'g'], ['--hd-warn', S, 3, 'g'], ['--hd-alert', S, 3, 'g']];
    function tokens(root, palette, r) {
        const probe = document.createElement('span'); probe.className = 'hd-lab-probe'; root.appendChild(probe);
        const rootStyle = getComputedStyle(root);
        // An unset custom property reads as an empty string; only a non-empty value is resolved. The colour comes from a probe element.
        const read = t => { if (!rootStyle.getPropertyValue(t).trim()) return null; probe.style.setProperty('color', 'var(' + t + ')'); return rgba(getComputedStyle(probe).color); };
        try {
            for (const t of PALETTE) { r.resolution.checked++; if (!read(t)) r.resolution.failures.push('token ' + t + ' does not resolve'); }
            if (palette === 'theme') { r.resolution.checked++; const a = read(S), b = read('--vd-mat-card-color'); if (!a || !b || a.slice(0, 3).some((v, i) => Math.abs(v - b[i]) > 1)) r.resolution.failures.push('bridge: --hd-surface-solid does not follow --vd-mat-card-color'); }
            const canvasBg = over(read(C) || WHITE, WHITE);
            for (const [fg, bg, min, kind] of PAIRS) { const f = read(fg), b = read(bg); if (!f || !b) continue;
                const base = bg === C ? canvasBg : over(b, canvasBg), v = ratio(over(f, base), base); r.contrast.checked++;
                if (v < min) { const msg = 'token ' + fg + ' on ' + bg + ' ' + show(v) + ' < ' + min; if (kind === 'g' && palette === 'theme') r.contrast.warnings.push(msg + ' (theme colours, reported only)'); else r.contrast.failures.push(msg); } }
        } finally { probe.remove(); }
    }
    function text(root, r) { const owners = new Set(), walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
        for (let n = walker.nextNode(); n; n = walker.nextNode()) if (n.textContent.trim() && n.parentElement) owners.add(n.parentElement);
        for (const el of owners) { const cs = getComputedStyle(el), rect = el.getBoundingClientRect();
            if (cs.visibility === 'hidden' || !rect.width || !rect.height || el.closest('[hidden]') || exempt(el)) continue;
            const bg = backgroundOf(el); if (!bg) { r.contrast.unmeasured++; continue; }
            // SVG text is painted with fill, not color.
            const paint = el instanceof SVGElement ? cs.fill : cs.color; if (!paint || paint === 'none') continue;
            if (/^url\(/.test(paint)) { r.contrast.unmeasured++; continue; }
            const fg = rgba(paint); fg[3] *= opacityOf(el); const min = el.closest('[aria-hidden="true"]') ? GRAPHIC_MIN : TEXT_MIN, v = ratio(over(fg, bg), bg); r.contrast.checked++;
            if (v < min) r.contrast.failures.push('text ' + label(el) + ' ' + JSON.stringify(el.textContent.trim().slice(0, 24)) + ' ' + show(v) + ' < ' + min); } }
    const GRAPHICS = '.hd-dial-arc, .hd-gauge-arc, .hd-gauge-needle-shape, .hd-spark-line, .hd-slider-fill, .hd-bar-fill, .hd-seg7-seg[data-lit="true"], .hd-toggle-thumb, .hd-lamp-dot';
    function graphics(root, r, extraSelectors) { const sel = [GRAPHICS].concat(extraSelectors || []).join(', ');
        for (const el of root.querySelectorAll(sel)) { const rect = el.getBoundingClientRect(); if (exempt(el) || !rect.width || !rect.height) continue;
            const cs = getComputedStyle(el), svg = el instanceof SVGElement; let paint = null, opacity = 1;
            if (cs.stroke && cs.stroke !== 'none') { if (/^url\(/.test(cs.stroke)) { r.contrast.unmeasured++; continue; } const s = rgba(cs.stroke); if (s[3] > 0) { paint = s; opacity = Number(cs.strokeOpacity); } }
            if (!paint && svg && cs.fill && cs.fill !== 'none') { if (/^url\(/.test(cs.fill)) { r.contrast.unmeasured++; continue; } const f = rgba(cs.fill); if (f[3] > 0) { paint = f; opacity = Number(cs.fillOpacity); } }
            if (!paint && !svg) { const b = rgba(cs.backgroundColor); if (b[3] > 0) paint = b; }
            if (!paint) continue; paint[3] *= opacityOf(el) * (Number.isFinite(opacity) ? opacity : 1);
            const host = svg ? (el.ownerSVGElement ? el.ownerSVGElement.parentElement : el.parentElement) : el.parentElement, bg = host ? backgroundOf(host) : null; if (!bg) { r.contrast.unmeasured++; continue; }
            const v = ratio(over(paint, bg), bg); r.contrast.checked++; if (v < GRAPHIC_MIN) r.contrast.failures.push('graphic ' + label(el) + ' ' + show(v) + ' < 3'); } }
    const INTERACTIVE = ['button', '[role="slider"]', '[role="switch"]', '[role="radio"]', '[role="spinbutton"]', '[tabindex="0"]'].map(s => '.hd-lab-stage ' + s).join(', ');
    function hit(root, r) { for (const el of root.querySelectorAll(INTERACTIVE)) { const b = el.getBoundingClientRect(); if ((!b.width && !b.height) || getComputedStyle(el).visibility === 'hidden') continue; r.hit.checked++;
        if (b.width < HIT_MIN - EPS || b.height < HIT_MIN - EPS) r.hit.failures.push('hit ' + label(el) + ' ' + dims(b) + ' < 44x44'); } }
    function shift(root, entries, states, r) { const groups = new Map();
        for (const cell of root.querySelectorAll('[data-hd-lab-control]')) { const t = cell.querySelector('.hd-lab-stage > *'); if (!t) continue; const b = t.getBoundingClientRect(), k = cell.getAttribute('data-hd-lab-control');
            if (!groups.has(k)) groups.set(k, []); groups.get(k).push({ s: cell.getAttribute('data-hd-lab-state'), w: b.width, h: b.height }); }
        for (const [k, list] of groups) { r.shift.checked++; for (const it of list.slice(1)) if (Math.abs(it.w - list[0].w) > EPS || Math.abs(it.h - list[0].h) > EPS) r.shift.failures.push('shift ' + k + ' ' + list[0].s + ' ' + show(list[0].w) + 'x' + show(list[0].h) + ' vs ' + it.s + ' ' + show(it.w) + 'x' + show(it.h)); }
        // Live check: set() through every state on the first instance of each control; the instance is restored to its own state afterwards.
        const seen = new Set(); for (const e of entries) { if (seen.has(e.control)) continue; seen.add(e.control); const b0 = e.el.getBoundingClientRect();
            try { for (const s of states) { e.apply(s); const b = e.el.getBoundingClientRect(); if (Math.abs(b.width - b0.width) > EPS || Math.abs(b.height - b0.height) > EPS) r.shift.failures.push('live shift ' + e.control + ' ' + e.state + ' -> ' + s + ' ' + dims(b0) + ' vs ' + dims(b)); } }
            catch (err) { r.shift.failures.push('live shift ' + e.control + ' threw: ' + String((err && err.message) || err)); }
            finally { try { e.apply(e.state); } catch (err) { r.shift.failures.push('live shift ' + e.control + ' could not restore ' + e.state + ': ' + String((err && err.message) || err)); } }
            r.shift.checked++; } }
    async function animations(motion, r) { if (motion === 'full') return; await new Promise(res => requestAnimationFrame(() => requestAnimationFrame(res))); await new Promise(res => setTimeout(res, 300)); r.animations.checked = true;
        for (const a of document.getAnimations()) if (a.playState === 'running' || a.pending) { const t = a.effect && a.effect.target; r.animations.running.push((t ? label(t) : '?') + ' ' + (a.animationName || a.transitionProperty || 'animation')); }
        for (const x of r.animations.running) r.animations.failures.push('still running with motion=' + motion + ': ' + x); }
    function backdrop(root, r) { let n = 0; for (const el of [root].concat(Array.from(root.querySelectorAll('*')))) { const cs = getComputedStyle(el), v = cs.backdropFilter || cs.webkitBackdropFilter; if (v && v !== 'none') n++; }
        r.backdrop.count = n; r.backdrop.limit = root.querySelectorAll('.hd-frame').length + 1; if (n > r.backdrop.limit) r.backdrop.failures.push('backdrop-filter on ' + n + ' elements, limit ' + r.backdrop.limit); }
    async function fonts(r) { for (const face of ['600 16px "HomeDash Barlow Condensed"', '16px "HomeDash Share Tech Mono"']) { r.resolution.checked++;
        try { const list = await document.fonts.load(face, 'Ag0'); if (!list.length || list.some(f => f.status !== 'loaded')) r.resolution.failures.push('font ' + face + ' did not load'); }
        catch (err) { r.resolution.failures.push('font ' + face + ' failed: ' + String((err && err.message) || err)); } } }
    async function run(o) {
        const r = { contrast: { checked: 0, failures: [], warnings: [], unmeasured: 0 }, hit: { checked: 0, failures: [] }, shift: { checked: 0, failures: [] }, animations: { checked: false, running: [], failures: [] }, backdrop: { count: 0, limit: 0, failures: [] }, resolution: { checked: 0, failures: [] } };
        tokens(o.root, o.palette, r); text(o.root, r); graphics(o.root, r, o.graphics); hit(o.root, r); shift(o.root, (o.entries || []).filter(e => !e.entry), o.states || [], r); backdrop(o.root, r); await fonts(r); await animations(o.motion, r);
        return r;
    }
    window.hdLabProbes = Object.freeze({ run, ratio, over, rgba });
})();
