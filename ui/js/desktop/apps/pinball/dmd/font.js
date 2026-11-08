// AuraPinball DMD bitmap digit fonts (l 10x16, m 6x10, s 4x6; glyphs 0-9 , . and space). DOM-free.
// Glyphs are generated once from stroke skeletons so the three sizes stay visually consistent.
const SKELETON = {
    '0': [[[0.2, 0], [0.8, 0], [1, 0.15], [1, 0.85], [0.8, 1], [0.2, 1], [0, 0.85], [0, 0.15], [0.2, 0]]],
    '1': [[[0.15, 0.22], [0.55, 0], [0.55, 1]], [[0.1, 1], [1, 1]]],
    '2': [[[0, 0.15], [0.2, 0], [0.8, 0], [1, 0.15], [1, 0.4], [0, 1], [1, 1]]],
    '3': [[[0, 0.1], [0.2, 0], [0.8, 0], [1, 0.12], [1, 0.38], [0.8, 0.5], [0.35, 0.5]], [[0.8, 0.5], [1, 0.62], [1, 0.88], [0.8, 1], [0.2, 1], [0, 0.9]]],
    '4': [[[0.75, 1], [0.75, 0], [0, 0.7], [1, 0.7]]],
    '5': [[[1, 0], [0.1, 0], [0, 0.45], [0.7, 0.4], [1, 0.55], [1, 0.85], [0.8, 1], [0.2, 1], [0, 0.9]]],
    '6': [[[0.9, 0.05], [0.6, 0], [0.2, 0], [0, 0.2], [0, 0.85], [0.2, 1], [0.8, 1], [1, 0.85], [1, 0.6], [0.8, 0.45], [0.2, 0.45], [0, 0.6]]],
    '7': [[[0, 0], [1, 0], [0.35, 1]]],
    '8': [[[0.5, 0.5], [0.15, 0.4], [0.1, 0.12], [0.3, 0], [0.7, 0], [0.9, 0.12], [0.85, 0.4], [0.5, 0.5], [0.1, 0.62], [0, 0.85], [0.2, 1], [0.8, 1], [1, 0.85], [0.9, 0.62], [0.5, 0.5]]]
};
SKELETON['9'] = SKELETON['6'].map(poly => poly.map(([x, y]) => [1 - x, 1 - y]));

const SPECS = Object.freeze({
    l: { w: 10, h: 16, gw: 9, thick: 2 },
    m: { w: 6, h: 10, gw: 5, thick: 1 },
    s: { w: 4, h: 6, gw: 3, thick: 1 }
});

function segmentDistance(px, py, ax, ay, bx, by) {
    const dx = bx - ax, dy = by - ay, len2 = dx * dx + dy * dy;
    let u = len2 > 0 ? ((px - ax) * dx + (py - ay) * dy) / len2 : 0;
    u = Math.max(0, Math.min(1, u));
    return Math.hypot(px - (ax + u * dx), py - (ay + u * dy));
}

function strokeGlyph(polys, spec) {
    const { gw, h, thick } = spec, half = thick / 2;
    const bits = new Uint8Array(gw * h);
    const map = ([x, y]) => [half + x * (gw - thick), half + y * (h - thick)];
    const lines = polys.map(poly => poly.map(map));
    for (let y = 0; y < h; y++) for (let x = 0; x < gw; x++) {
        let lit = false;
        for (const line of lines) {
            for (let i = 0; i + 1 < line.length && !lit; i++) {
                lit = segmentDistance(x + 0.5, y + 0.5, line[i][0], line[i][1], line[i + 1][0], line[i + 1][1]) <= half + 1e-9;
            }
            if (lit) break;
        }
        if (lit) bits[y * gw + x] = 1;
    }
    return bits;
}

function dotGlyph(spec, comma) {
    const t = Math.max(1, Math.round(spec.thick)), tail = comma ? Math.max(1, Math.round(spec.h / 8)) : 0;
    const rows = t + tail;
    return { gw: t, rows, yOff: spec.h - t, bits: new Uint8Array(t * rows).fill(1), adv: t + 1 };
}

function build(key) {
    const spec = SPECS[key], glyphs = {};
    for (const ch of '0123456789') glyphs[ch] = { gw: spec.gw, rows: spec.h, yOff: 0, bits: strokeGlyph(SKELETON[ch], spec), adv: spec.w };
    glyphs['.'] = dotGlyph(spec, false);
    glyphs[','] = dotGlyph(spec, true);
    glyphs[' '] = { gw: 0, rows: 0, yOff: 0, bits: new Uint8Array(0), adv: Math.max(2, spec.w >> 1) };
    return { key, w: spec.w, h: spec.h, glyphs };
}

export const FONTS = Object.freeze({ l: build('l'), m: build('m'), s: build('s') });

export function formatNumber(n, { sep = ',' } = {}) {
    const v = Math.max(0, Math.floor(Number.isFinite(n) ? n : 0));
    const digits = String(v);
    if (!sep) return digits;
    return digits.replace(/\B(?=(\d{3})+(?!\d))/g, sep);
}

const stringCache = new Map();

// Renders a digit string into a 0/1 bitmap {w, h, bits}. Unknown characters are skipped.
export function renderString(fontKey, str) {
    const key = fontKey + '|' + str;
    const hit = stringCache.get(key);
    if (hit) return hit;
    const font = FONTS[fontKey] || FONTS.m;
    const chars = Array.from(String(str)).filter(ch => font.glyphs[ch]);
    const w = Math.max(1, chars.reduce((sum, ch) => sum + font.glyphs[ch].adv, 0) - 1);
    const h = Math.max(font.h, ...chars.map(ch => font.glyphs[ch].yOff + font.glyphs[ch].rows));
    const bits = new Uint8Array(w * h);
    let x = 0;
    for (const ch of chars) {
        const g = font.glyphs[ch];
        for (let gy = 0; gy < g.rows; gy++) for (let gx = 0; gx < g.gw; gx++) {
            if (g.bits[gy * g.gw + gx] && x + gx < w) bits[(g.yOff + gy) * w + x + gx] = 1;
        }
        x += g.adv;
    }
    const result = { w, h, bits };
    if (stringCache.size > 256) stringCache.clear();
    stringCache.set(key, result);
    return result;
}

export function measureString(fontKey, str) { return renderString(fontKey, str).w; }
