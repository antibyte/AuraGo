// AuraPinball DMD text rasteriser. Canvas fillText when a canvas exists, otherwise a deterministic block fallback.
// DOM access is guarded so the module imports under Node. Bitmaps hold levels 0..3.
import { FONT_STACK } from '../config.js';

export const SIZES = Object.freeze({ s: 9, m: 13, l: 20, xl: 30 });
export const LARGE_MIN_PX = 13;
export const CACHE_LIMIT = 64;
const MAX_WIDTH = 2048;

// Alpha 0..1 to a DMD level: <0.2 -> 0, <0.45 -> 1, <0.7 -> 2, otherwise 3.
export function quantize(alpha) {
    return alpha < 0.2 ? 0 : alpha < 0.45 ? 1 : alpha < 0.7 ? 2 : 3;
}

const LARGE_RANGES = [[0x0900, 0x097F], [0x1100, 0x11FF], [0x2E80, 0x9FFF], [0xA960, 0xA97F], [0xAC00, 0xD7FF], [0xF900, 0xFAFF], [0xFF00, 0xFFEF], [0x20000, 0x3FFFF]];

// CJK, Hangul and Devanagari glyphs turn to mush below 13 px.
export function needsLargeGlyphs(str) {
    for (const ch of String(str ?? '')) {
        const cp = ch.codePointAt(0);
        if (cp < 0x0900) continue;
        for (const [lo, hi] of LARGE_RANGES) if (cp >= lo && cp <= hi) return true;
    }
    return false;
}

function defaultCanvasFactory() {
    try {
        if (typeof OffscreenCanvas === 'function') return new OffscreenCanvas(8, 8);
        if (typeof document !== 'undefined' && document.createElement) return document.createElement('canvas');
    } catch { /* no canvas */ }
    return null;
}

export function createRaster(options = {}) {
    const cache = new Map();
    let canvas = null, ctx = null, tried = options.canvas === false;

    function context() {
        if (tried) return ctx;
        tried = true;
        try {
            canvas = (options.canvasFactory || defaultCanvasFactory)();
            ctx = canvas && canvas.getContext ? canvas.getContext('2d', { willReadFrequently: true }) : null;
        } catch { ctx = null; }
        return ctx;
    }

    function fallback(str, px) {
        const chars = Array.from(str), cell = 0.6 * px;
        const w = Math.max(1, Math.round(cell * chars.length)), h = Math.max(1, Math.ceil(px * 1.3));
        const data = new Uint8Array(w * h), top = Math.round(px * 0.22), bottom = Math.max(top + 1, Math.round(px * 0.95));
        chars.forEach((ch, i) => {
            if (/\s/.test(ch)) return;
            const x0 = Math.round(i * cell), x1 = Math.max(x0 + 1, Math.round((i + 1) * cell) - 1);
            for (let y = top; y < bottom && y < h; y++) for (let x = x0; x < x1 && x < w; x++) {
                const edge = y === top || y === bottom - 1 || x === x0 || x === x1 - 1;
                data[y * w + x] = edge || px < 13 ? 3 : 2;
            }
        });
        return { w, h, data, canvas: false };
    }

    function render(str, px) {
        const c = context();
        if (!c) return fallback(str, px);
        try {
            c.font = 'bold ' + px + 'px ' + FONT_STACK;
            const w = Math.min(MAX_WIDTH, Math.max(1, Math.ceil(c.measureText(str).width) + 2)), h = Math.ceil(px * 1.3);
            canvas.width = w; canvas.height = h;
            c.font = 'bold ' + px + 'px ' + FONT_STACK;
            c.clearRect(0, 0, w, h);
            c.fillStyle = '#fff';
            c.textBaseline = 'alphabetic';
            c.fillText(str, 1, Math.round(px * 0.95));
            const img = c.getImageData(0, 0, w, h).data, data = new Uint8Array(w * h);
            for (let i = 0; i < data.length; i++) data[i] = quantize(img[i * 4 + 3] / 255);
            return { w, h, data, canvas: true };
        } catch { return fallback(str, px); }
    }

    // Bitmap of one string at a pixel size (LRU cached).
    function bitmap(str, px) {
        const text = String(str);
        const key = px + '|' + text;
        const hit = cache.get(key);
        if (hit) { cache.delete(key); cache.set(key, hit); return hit; }
        const made = render(text, px);
        cache.set(key, made);
        while (cache.size > CACHE_LIMIT) cache.delete(cache.keys().next().value);
        return made;
    }

    // Shrinks the pixel size until the string fits maxWidth; returns the smallest tried bitmap otherwise.
    function fit(str, px, maxWidth, minPx) {
        let size = px, bm = bitmap(str, size);
        if (!(maxWidth > 0) || bm.w <= maxWidth || px <= minPx) return { ...bm, px: size };
        size = Math.max(minPx, Math.min(px - 1, Math.floor(px * maxWidth / bm.w)));
        bm = bitmap(str, size);
        while (bm.w > maxWidth && size > minPx) { size--; bm = bitmap(str, size); }
        return { ...bm, px: size };
    }

    return { bitmap, fit, hasCanvas: () => !!context(), size: () => cache.size, limit: CACHE_LIMIT };
}
