// HomeDash geometry (pure): arc, tick and needle paths, monotone (Fritsch-Carlson) sparklines, seven-segment digits.
// Angles: degrees from 12 o'clock, clockwise.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-geometry.js: load hd-core.js first');
    const clamp = E.core.clamp, n = v => String(Math.round(v * 1000) / 1000), rad = d => d * Math.PI / 180;
    const point = (cx, cy, r, deg) => [cx + r * Math.sin(rad(deg)), cy - r * Math.cos(rad(deg))];
    function arc(cx, cy, r, start, sweep) {
        const s = clamp(sweep, 0, 360);
        if (s >= 360) return arc(cx, cy, r, start, 180) + arc(cx, cy, r, start + 180, 180).replace(/^M[^A]*/, '');
        const [x0, y0] = point(cx, cy, r, start), [x1, y1] = point(cx, cy, r, start + s);
        return 'M' + n(x0) + ' ' + n(y0) + 'A' + n(r) + ' ' + n(r) + ' 0 ' + (s > 180 ? 1 : 0) + ' 1 ' + n(x1) + ' ' + n(y1);
    }
    function ticks(cx, cy, r1, r2, start, sweep, steps) { let d = '';
        for (let i = 0; i <= steps; i++) { const a = start + sweep * i / steps, [x0, y0] = point(cx, cy, r1, a), [x1, y1] = point(cx, cy, r2, a); d += 'M' + n(x0) + ' ' + n(y0) + 'L' + n(x1) + ' ' + n(y1); }
        return d; }
    function needle(cx, cy, length, tail, base, tip) {
        const pts = [[cx - tip / 2, cy - length], [cx + tip / 2, cy - length], [cx + base / 2, cy], [cx + base / 2, cy + tail], [cx - base / 2, cy + tail], [cx - base / 2, cy]];
        return 'M' + pts.map(p => n(p[0]) + ' ' + n(p[1])).join('L') + 'Z';
    }
    const fraction = (v, min, max) => (typeof v !== 'number' || !Number.isFinite(v) || !(max > min) ? 0 : clamp((v - min) / (max - min), 0, 1));
    const valueToAngle = (v, min, max, start, sweep) => start + fraction(v, min, max) * sweep;
    const needleChanged = (prev, next, min, max) => (typeof prev !== 'number' || typeof next !== 'number' ? prev !== next : Math.abs(next - prev) >= 0.0025 * (max - min));
    function minMaxLast(series) { const vals = (series || []).filter(v => typeof v === 'number' && Number.isFinite(v)); return vals.length ? { min: Math.min(...vals), max: Math.max(...vals), last: vals[vals.length - 1] } : null; }
    function monotone(pts) {
        const k = pts.length, d = [], m = new Array(k);
        for (let i = 0; i < k - 1; i++) d.push((pts[i + 1][1] - pts[i][1]) / (pts[i + 1][0] - pts[i][0]));
        m[0] = d[0]; m[k - 1] = d[k - 2];
        for (let i = 1; i < k - 1; i++) m[i] = d[i - 1] * d[i] <= 0 ? 0 : (d[i - 1] + d[i]) / 2;
        for (let i = 0; i < k - 1; i++) {
            if (d[i] === 0) { m[i] = 0; m[i + 1] = 0; continue; }
            const a = m[i] / d[i], b = m[i + 1] / d[i], s = a * a + b * b;
            if (s > 9) { const t = 3 / Math.sqrt(s); m[i] = t * a * d[i]; m[i + 1] = t * b * d[i]; }
        }
        let out = '';
        for (let i = 0; i < k - 1; i++) { const [x0, y0] = pts[i], [x1, y1] = pts[i + 1], dx = (x1 - x0) / 3;
            out += 'C' + n(x0 + dx) + ' ' + n(y0 + m[i] * dx) + ' ' + n(x1 - dx) + ' ' + n(y1 - m[i + 1] * dx) + ' ' + n(x1) + ' ' + n(y1); }
        return out;
    }
    function sparkline(values, opts) {
        const o = opts || {}, w = o.w || 100, h = o.h || 40, pad = o.pad === undefined ? 2 : o.pad, yPad = o.yPad === undefined ? 0.12 : o.yPad;
        const list = Array.isArray(values) ? values : [], st = minMaxLast(list);
        if (!st) return { line: '', area: '', last: null, min: null, max: null };
        const raw = st.max - st.min, padv = raw > 0 ? raw * yPad : Math.max(Math.abs(st.max) * 0.1, 1), lo = st.min - padv, hi = st.max + padv;
        const stepX = list.length > 1 ? (w - 2 * pad) / (list.length - 1) : 0, X = i => pad + i * stepX, Y = v => h - pad - (v - lo) / (hi - lo) * (h - 2 * pad);
        const segs = []; let cur = [];
        list.forEach((v, i) => { if (typeof v === 'number' && Number.isFinite(v)) cur.push([X(i), Y(v)]); else if (cur.length) { segs.push(cur); cur = []; } });
        if (cur.length) segs.push(cur);
        let line = '', area = '';
        for (const s of segs) { const head = 'M' + n(s[0][0]) + ' ' + n(s[0][1]), body = s.length > 1 ? monotone(s) : 'h0.001';
            line += head + body; area += head + body + 'L' + n(s[s.length - 1][0]) + ' ' + n(h) + 'L' + n(s[0][0]) + ' ' + n(h) + 'Z'; }
        const lp = segs[segs.length - 1][segs[segs.length - 1].length - 1];
        return { line, area, last: { x: Math.round(lp[0] * 1000) / 1000, y: Math.round(lp[1] * 1000) / 1000 }, min: lo, max: hi };
    }
    const SEGMENTS = Object.freeze({ '0': 'abcdef', '1': 'bc', '2': 'abdeg', '3': 'abcdg', '4': 'bcfg', '5': 'acdfg', '6': 'acdefg', '7': 'abc', '8': 'abcdefg', '9': 'abcdfg', '-': 'g', ' ': '', 'E': 'adefg', 'r': 'eg', 'o': 'cdeg', 'n': 'ceg', 'C': 'adef', 'F': 'aefg', 'H': 'bcefg', 'L': 'def', 'P': 'abefg' });
    const has = k => Object.prototype.hasOwnProperty.call(SEGMENTS, k);
    function sevenSeg(text) { const out = [];
        for (const ch of String(text == null ? '' : text)) {
            if (ch === '.' || ch === ',') { if (out.length && !out[out.length - 1].dp) out[out.length - 1].dp = true; else out.push({ char: ' ', segs: [], dp: true }); continue; }
            const key = has(ch) ? ch : has(ch.toUpperCase()) ? ch.toUpperCase() : ' '; out.push({ char: key, segs: SEGMENTS[key].split(''), dp: false });
        }
        return out; }
    const hexH = (x0, x1, y, t) => { const r = t / 2; return 'M' + n(x0) + ' ' + n(y) + 'L' + n(x0 + r) + ' ' + n(y - r) + 'L' + n(x1 - r) + ' ' + n(y - r) + 'L' + n(x1) + ' ' + n(y) + 'L' + n(x1 - r) + ' ' + n(y + r) + 'L' + n(x0 + r) + ' ' + n(y + r) + 'Z'; };
    const hexV = (y0, y1, x, t) => { const r = t / 2; return 'M' + n(x) + ' ' + n(y0) + 'L' + n(x + r) + ' ' + n(y0 + r) + 'L' + n(x + r) + ' ' + n(y1 - r) + 'L' + n(x) + ' ' + n(y1) + 'L' + n(x - r) + ' ' + n(y1 - r) + 'L' + n(x - r) + ' ' + n(y0 + r) + 'Z'; };
    function segmentPaths(w, h, t, gap) { const g = gap === undefined ? t * 0.25 : gap, L = t / 2, R = w - t / 2, T = t / 2, M = h / 2, B = h - t / 2;
        return { a: hexH(L + g, R - g, T, t), g: hexH(L + g, R - g, M, t), d: hexH(L + g, R - g, B, t), f: hexV(T + g, M - g, L, t), b: hexV(T + g, M - g, R, t),
            e: hexV(M + g, B - g, L, t), c: hexV(M + g, B - g, R, t), dp: 'M' + n(w + t * 0.4) + ' ' + n(B - t / 2) + 'h' + n(t) + 'v' + n(t) + 'h' + n(-t) + 'Z' }; }
    E.geometry = Object.freeze({ point, arc, ticks, needle, fraction, valueToAngle, needleChanged, minMaxLast, sparkline, SEGMENTS, sevenSeg, segmentPaths });
})();
