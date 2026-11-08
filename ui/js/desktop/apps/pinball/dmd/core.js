// AuraPinball DMD core: 192x48 frame (levels 0..3), draw API, scene queue with priorities and transitions. DOM-free.
import { rngNext } from '../config.js';
import { SIZES, LARGE_MIN_PX, needsLargeGlyphs } from './raster.js';
import { formatNumber, renderString } from './font.js';
import { PLAY_SCENES } from './scenes-play.js';
import { EVENT_SCENES } from './scenes-events.js';

export const DMD_W = 192;
export const DMD_H = 48;
export const QUEUE_MAX = 6;
export const STEP = 1 / 30;
export const TRANSITION_S = Object.freeze({ cut: 0, wipeLeft: 0.3, wipeRight: 0.3, wipeUp: 0.3, dissolve: 0.35, zoom: 0.4 });
const KEY_RE = /^[a-z][a-z0-9]*(?:_[a-z0-9]+)+$/;

// Deterministic dissolve threshold per pixel.
const NOISE = (() => {
    const holder = { seed: 0x5eed }, out = new Float32Array(DMD_W * DMD_H);
    for (let i = 0; i < out.length; i++) out[i] = rngNext(holder);
    return out;
})();

const ease = p => p * p * (3 - 2 * p);

export function createDmd({ raster, tr, locale, scenes, strict = false } = {}) {
    const N = DMD_W * DMD_H;
    const frame = new Uint8Array(N), next = new Uint8Array(N), layer = new Uint8Array(N), from = new Uint8Array(N);
    const table = { ...PLAY_SCENES, ...EVENT_SCENES, ...(scenes || {}) };
    let version = 0, acc = 0, dirty = true, trans = null, lastText = '', texts = [];
    let base = { scene: 'attract', params: {}, mem: {}, t: 0 };
    let active = null;
    const queue = [];

    function resolve(input, vars) {
        const raw = String(input ?? '');
        let out = raw;
        if (KEY_RE.test(raw)) {
            let got = null;
            try { got = typeof tr === 'function' ? tr(raw, vars) : null; } catch { got = null; }
            out = got && got !== raw && !got.startsWith('pinball.') ? String(got) : raw.replace(/^[a-z0-9]+_/, '').replace(/_/g, ' ').toUpperCase();
        }
        return vars ? out.replace(/\{(\w+)\}/g, (m, k) => (k in vars ? String(vars[k]) : m)) : out;
    }

    const d = { W: DMD_W, H: DMD_H, t: 0, dt: STEP, mem: {}, locale };
    let buf = layer;

    d.pixel = (x, y, level = 3) => {
        x = Math.round(x); y = Math.round(y);
        if (x >= 0 && y >= 0 && x < DMD_W && y < DMD_H) buf[y * DMD_W + x] = level;
    };
    d.get = (x, y) => (x >= 0 && y >= 0 && x < DMD_W && y < DMD_H ? buf[y * DMD_W + x] : 0);
    d.clear = (level = 0) => { buf.fill(level); };
    d.rect = (x, y, w, h, level = 3, fill = true) => {
        x = Math.round(x); y = Math.round(y); w = Math.round(w); h = Math.round(h);
        for (let j = 0; j < h; j++) for (let i = 0; i < w; i++) {
            if (fill || i === 0 || j === 0 || i === w - 1 || j === h - 1) d.pixel(x + i, y + j, level);
        }
    };
    d.line = (x0, y0, x1, y1, level = 3) => {
        x0 = Math.round(x0); y0 = Math.round(y0); x1 = Math.round(x1); y1 = Math.round(y1);
        const dx = Math.abs(x1 - x0), dy = -Math.abs(y1 - y0), sx = x0 < x1 ? 1 : -1, sy = y0 < y1 ? 1 : -1;
        let err = dx + dy;
        for (let guard = 0; guard < 600; guard++) {
            d.pixel(x0, y0, level);
            if (x0 === x1 && y0 === y1) break;
            const e2 = 2 * err;
            if (e2 >= dy) { err += dy; x0 += sx; }
            if (e2 <= dx) { err += dx; y0 += sy; }
        }
    };
    d.circle = (cx, cy, r, level = 3, fill = false) => {
        const rr = Math.ceil(r);
        for (let j = -rr; j <= rr; j++) for (let i = -rr; i <= rr; i++) {
            const dist = Math.hypot(i, j);
            if (fill ? dist <= r + 0.3 : Math.abs(dist - r) <= 0.55) d.pixel(cx + i, cy + j, level);
        }
    };
    // Vector sprite: array of [x, y] points; null starts a new stroke. fill=true scan-fills the closed outline.
    d.sprite = (points, x = 0, y = 0, scale = 1, rot = 0, level = 3, fill = false) => {
        const cos = Math.cos(rot), sin = Math.sin(rot), strokes = [[]];
        for (const p of points || []) {
            if (!p) { if (strokes[strokes.length - 1].length) strokes.push([]); continue; }
            strokes[strokes.length - 1].push([x + (p[0] * cos - p[1] * sin) * scale, y + (p[0] * sin + p[1] * cos) * scale]);
        }
        if (fill) {
            for (let row = 0; row < DMD_H; row++) {
                const xs = [], yc = row + 0.5;
                for (const s of strokes) for (let i = 0; i < s.length; i++) {
                    const a = s[i], b = s[(i + 1) % s.length];
                    if ((a[1] <= yc) !== (b[1] <= yc)) xs.push(a[0] + (yc - a[1]) * (b[0] - a[0]) / (b[1] - a[1]));
                }
                xs.sort((p, q) => p - q);
                for (let i = 0; i + 1 < xs.length; i += 2) for (let c = Math.round(xs[i]); c < Math.round(xs[i + 1]); c++) d.pixel(c, row, level);
            }
        }
        for (const s of strokes) for (let i = 0; i + 1 < s.length; i++) d.line(s[i][0], s[i][1], s[i + 1][0], s[i + 1][1], level);
    };
    d.sparkle = (n, seed = 1, region = null, level = 0) => {
        const holder = { seed: seed >>> 0 }, r = region || { x: 0, y: 0, w: DMD_W, h: DMD_H };
        for (let i = 0; i < n; i++) {
            const x = r.x + Math.floor(rngNext(holder) * r.w), y = r.y + Math.floor(rngNext(holder) * r.h);
            const lv = level || 1 + Math.floor(rngNext(holder) * 3), star = rngNext(holder) < 0.25;
            d.pixel(x, y, lv);
            if (star) { d.pixel(x - 1, y, 2); d.pixel(x + 1, y, 2); d.pixel(x, y - 1, 2); d.pixel(x, y + 1, 2); d.pixel(x, y, 3); }
        }
    };
    d.progress = (x, y, w, h, frac, level = 3) => {
        d.rect(x, y, w, h, level, false);
        const inner = Math.round((w - 2) * Math.max(0, Math.min(1, frac)));
        if (inner > 0) d.rect(x + 1, y + 1, inner, h - 2, level, true);
    };
    d.invert = (x = 0, y = 0, w = DMD_W, h = DMD_H) => {
        for (let j = Math.max(0, y); j < Math.min(DMD_H, y + h); j++) for (let i = Math.max(0, x); i < Math.min(DMD_W, x + w); i++) buf[j * DMD_W + i] = 3 - buf[j * DMD_W + i];
    };

    function blit(data, w, h, x0, y0, level, clipL, clipR) {
        for (let j = 0; j < h; j++) for (let i = 0; i < w; i++) {
            const v = data[j * w + i];
            if (!v) continue;
            const px = x0 + i;
            if (px < clipL || px >= clipR) continue;
            d.pixel(px, y0 + j, Math.min(v, level));
        }
    }

    d.resolve = resolve;
    // text(strOrKey, x, y, {size, align, valign, level, maxWidth, vars}); y is the top of the line box.
    d.text = (input, x, y, o = {}) => {
        const str = resolve(input, o.vars);
        lastText = str; texts.push(str);
        if (!str || !raster) return { w: 0, h: 0 };
        const large = needsLargeGlyphs(str), level = o.level ?? 3;
        const px = Math.max(SIZES[o.size] || SIZES.m, large ? LARGE_MIN_PX : 0), minPx = large ? LARGE_MIN_PX : Math.min(px, 8);
        const bm = o.maxWidth ? raster.fit(str, px, o.maxWidth, minPx) : { ...raster.bitmap(str, px), px };
        const align = o.align || 'left';
        const boxW = o.maxWidth && bm.w > o.maxWidth ? o.maxWidth : bm.w;
        const left = align === 'center' ? Math.round(x - boxW / 2) : align === 'right' ? Math.round(x - boxW) : Math.round(x);
        const top = Math.round(o.valign === 'middle' ? y - bm.px * 0.585 : y);
        if (o.maxWidth && bm.w > o.maxWidth) { // marquee
            const gap = 20, shift = Math.floor(d.t * 30) % (bm.w + gap);
            blit(bm.data, bm.w, bm.h, left - shift, top, level, left, left + boxW);
            blit(bm.data, bm.w, bm.h, left - shift + bm.w + gap, top, level, left, left + boxW);
        } else blit(bm.data, bm.w, bm.h, left, top, level, 0, DMD_W);
        return { w: boxW, h: bm.h, px: bm.px };
    };
    // number(n, x, y, {font: 'l'|'m'|'s', align, level, sep}) with thousands separators.
    d.number = (n, x, y, o = {}) => {
        const s = renderString(o.font || 'm', formatNumber(n, { sep: o.sep === undefined ? ',' : o.sep }).padStart(o.pad || 0, '0')), level = o.level ?? 3;
        const left = o.align === 'center' ? Math.round(x - s.w / 2) : o.align === 'right' ? Math.round(x - s.w) : Math.round(x);
        for (let j = 0; j < s.h; j++) for (let i = 0; i < s.w; i++) if (s.bits[j * s.w + i]) d.pixel(left + i, y + j, level);
        return { w: s.w, h: s.h };
    };

    // ----- scene control -----
    const durationOf = kind => TRANSITION_S[kind] ?? 0;

    function startTransition(kind) {
        const dur = durationOf(kind);
        from.set(frame);
        trans = dur > 0 ? { kind, t: 0, dur } : null;
    }

    function makeEntry(name, params, opts) {
        const def = table[name];
        const ms = Number.isFinite(opts.ms) && opts.ms > 0 ? opts.ms : (def.ms ?? Infinity);
        return {
            name, def, params: params || {}, ms, mem: {}, t: 0,
            priority: Number.isFinite(opts.priority) ? opts.priority : (def.priority ?? 5),
            transition: opts.transition || def.transition || 'cut'
        };
    }

    function activate(entry) {
        active = entry;
        startTransition(entry.transition);
        dirty = true;
    }

    function finishActive() {
        const following = queue.shift();
        if (following) { active = following; startTransition(following.transition); }
        else { active = null; startTransition('dissolve'); }
        dirty = true;
    }

    function enqueue(entry) {
        let at = queue.length;
        while (at > 0 && queue[at - 1].priority < entry.priority) at--;
        queue.splice(at, 0, entry);
        if (queue.length > QUEUE_MAX) queue.pop();
        return queue.includes(entry);
    }

    function play(name, params, opts = {}) {
        if (!table[name]) return false;
        const entry = makeEntry(name, params, opts);
        if (!active) { activate(entry); return true; }
        if (entry.def.live && active.name === name && entry.priority >= active.priority) {
            active.params = entry.params; active.priority = entry.priority;
            if (opts.ms) { active.ms = entry.ms; active.t = 0; }
            dirty = true;
            return true;
        }
        if (entry.priority > active.priority) {
            if (entry.priority >= 9) queue.length = 0;
            activate(entry);
            return true;
        }
        return enqueue(entry);
    }

    function setBase(scene, params, quiet = false) {
        if (!table[scene]) return false;
        if (base.scene === scene) {
            base.params = params || {};
            if (!quiet) dirty = true;
        } else {
            base = { scene, params: params || {}, mem: {}, t: 0 };
            if (!active) startTransition(table[scene].transition || 'dissolve');
            dirty = true;
        }
        return true;
    }

    function clear() {
        queue.length = 0;
        if (active) { active = null; startTransition('dissolve'); }
        dirty = true;
    }

    function composite(out, kind, p) {
        const q = ease(p);
        if (kind === 'wipeLeft' || kind === 'wipeRight' || kind === 'wipeUp') {
            const edge = Math.floor((kind === 'wipeUp' ? DMD_H : DMD_W) * p);
            for (let y = 0; y < DMD_H; y++) for (let x = 0; x < DMD_W; x++) {
                const i = y * DMD_W + x;
                const fresh = kind === 'wipeLeft' ? x < edge : kind === 'wipeRight' ? x >= DMD_W - edge : y >= DMD_H - edge;
                out[i] = fresh ? layer[i] : from[i];
            }
        } else if (kind === 'dissolve') {
            for (let i = 0; i < N; i++) out[i] = NOISE[i] < p ? layer[i] : from[i];
        } else if (kind === 'zoom') {
            const s = 0.12 + 0.88 * q;
            for (let y = 0; y < DMD_H; y++) for (let x = 0; x < DMD_W; x++) {
                const sx = Math.round(DMD_W / 2 + (x - DMD_W / 2) / s), sy = Math.round(DMD_H / 2 + (y - DMD_H / 2) / s);
                const i = y * DMD_W + x;
                out[i] = sx >= 0 && sy >= 0 && sx < DMD_W && sy < DMD_H ? layer[sy * DMD_W + sx] : from[i];
            }
        } else out.set(layer);
    }

    function render(step) {
        const cur = active || base;
        buf = layer; buf.fill(0);
        d.t = cur.t; d.dt = step; d.mem = cur.mem;
        texts = []; lastText = '';
        try { table[cur.name || cur.scene].draw(d, cur.params, cur.t); } catch (error) { if (strict) throw error; /* a broken scene must not stop the display */ }
        if (trans) composite(next, trans.kind, Math.min(1, trans.t / trans.dur)); else next.set(layer);
        let changed = false;
        for (let i = 0; i < N; i++) if (next[i] !== frame[i]) { changed = true; break; }
        if (changed) { frame.set(next); version++; }
        return changed;
    }

    // `state` is the rules dmdState: {scene, params} for the base scene; params update live every tick.
    function update(dt, state) {
        if (state && typeof state === 'object' && state.scene) setBase(state.scene, state.params, true);
        acc += Number.isFinite(dt) ? Math.max(0, dt) : 0;
        if (!dirty && acc < STEP) return false;
        const step = acc; acc = 0; dirty = false;
        base.t += step;
        if (active) {
            active.t += step;
            if (active.t * 1000 >= active.ms) finishActive();
        }
        if (trans) { trans.t += step; if (trans.t >= trans.dur) trans = null; }
        return render(step);
    }

    function inspect() {
        const cur = active || base;
        return {
            base: { scene: base.scene, params: base.params },
            scene: active ? active.name : null,
            queue: queue.map(e => e.name),
            text: lastText,
            texts: texts.slice(),
            transition: trans ? { kind: trans.kind, progress: Math.min(1, trans.t / trans.dur) } : null,
            t: cur.t
        };
    }

    return {
        width: DMD_W, height: DMD_H, frame,
        get version() { return version; },
        setBase, play, update, clear, inspect,
        scenes: table
    };
}
