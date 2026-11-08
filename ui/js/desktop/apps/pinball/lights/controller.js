// Lamp, flasher and GI controller (plan sections 3.9 and 3.10). DOM-free and deterministic.
// Indexing: intensity[i] / modes[i] belong to layout.lamps[i]; its texel in `rgba` is i + 1 (texel 0 stays black).
// NOTE: the plan names both the method and the buffer "flash"; the Float32Array is exposed as `flashLevel`.
// flashLevel[j] belongs to layout.flashers[j]; giLevel[k] to layout.gi[k].
import { rngNext } from '../config.js';
import { SHOWS, SHOW_DEFAULTS, hueOf, hsvToRgb } from './shows.js';
import { FIELD_W, FIELD_H, FIELD_X0 } from './shapes.js';

export const TAU_RISE = 0.022;
export const TAU_FALL = 0.065;
const GI_TAU = 0.05;
const FLASH_HOLD = 0.04;
const MODE_NAMES = { off: 0, on: 1, slow: 2, fast: 3, pulse: 4, flicker: 5, dim: 6 };

export function parseHex(hex) {
    const m = /^#?([0-9a-f]{6})$/i.exec(String(hex || '').trim());
    if (!m) return [255, 255, 255];
    const n = parseInt(m[1], 16);
    return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function modeCode(m) { return typeof m === 'string' ? (MODE_NAMES[m] ?? 1) : (m | 0); }

// Lamp mode (plan 3.9 codes) -> target level at controller time `clock`; flick is a 0..1 sample for mode 5.
function modeLevel(mode, clock, flick) {
    switch (mode) {
    case 1: return 1;
    case 2: return (clock * 2) % 1 < 0.5 ? 1 : 0;
    case 3: return (clock * 6) % 1 < 0.5 ? 1 : 0;
    case 4: return 0.15 + 0.85 * (0.5 - 0.5 * Math.cos(clock * 1.25 * Math.PI * 2));
    case 5: return 0.3 + 0.7 * flick;
    case 6: return 0.3;
    default: return 0;
    }
}

export function createLamps(layout, opts = {}) {
    const defs = (layout && layout.lamps) || [], flashDefs = (layout && layout.flashers) || [], giDefs = (layout && layout.gi) || [];
    const n = defs.length, rng = { seed: (opts.seed >>> 0) || 0x5EED1 };
    const baseRgb = defs.map(d => parseHex(d.color)), rgb = baseRgb.map(c => c.slice());
    const modes = new Uint8Array(n), intensity = new Float32Array(n), flashBuf = new Float32Array(flashDefs.length);
    const giLevel = new Float32Array(giDefs.length).fill(1), rgba = new Uint8Array(1024);
    const flickHold = new Float32Array(n), flickVal = new Float32Array(n).fill(1);
    const flashAge = new Float32Array(flashDefs.length).fill(Infinity), flashDur = new Float32Array(flashDefs.length);
    const gi = giDefs.map(() => ({ mode: 1, left: 0, flickHold: 0, flickVal: 1 }));
    const tint = new Array(n).fill(null);
    let clock = 0, shows = [], nextShowId = 1;

    // Lamp views: playfield uv, group order and a global group-ordered sequence for `chase`.
    const groupOrder = [], groupCount = {};
    defs.forEach(d => { const g = d.group || d.id; if (!(g in groupCount)) { groupCount[g] = 0; groupOrder.push(g); } });
    const seqOf = defs.map((d, i) => ({ i, g: groupOrder.indexOf(d.group || d.id) }));
    seqOf.sort((a, b) => a.g - b.g || a.i - b.i);
    const views = defs.map((d, i) => ({
        index: i + 1, id: d.id, x: d.x, y: d.y, u: (d.x - FIELD_X0) / FIELD_W, v: d.y / FIELD_H, group: d.group || null,
        rgb: !!d.rgb, seq: 0, seqN: n
    }));
    seqOf.forEach((s, k) => { views[s.i].seq = k; });
    const idIndex = new Map(defs.map((d, i) => [d.id, i]));
    const flashIndex = new Map(flashDefs.map((d, i) => [d.id, i]));
    const giIndex = new Map(giDefs.map((d, i) => [d.id, i]));

    function setModes(src) { for (let i = 0; i < n; i++) modes[i] = src && i < src.length ? src[i] : 0; }

    function setColor(id, hex) {
        const i = idIndex.get(id);
        if (i === undefined) return;
        rgb[i] = parseHex(hex);
    }

    function show(name, o = {}) {
        const fn = SHOWS[name];
        if (!fn) return 0;
        const d = SHOW_DEFAULTS[name] || {};
        const s = {
            id: nextShowId++, name, fn, t: 0, duration: o.duration > 0 ? o.duration : (d.duration || 1),
            priority: o.priority || 0, replace: o.replace !== undefined ? !!o.replace : !!d.replace,
            loop: o.loop === true ? Infinity : (o.loop | 0) > 0 ? (o.loop | 0) : 1, color: o.color ? parseHex(o.color) : null,
            params: Object.assign({ seed: rng.seed }, o)
        };
        s.params.duration = s.duration;
        shows.push(s);
        shows.sort((a, b) => a.priority - b.priority || a.id - b.id);
        return s.id;
    }

    function stopShows(name) { shows = name ? shows.filter(s => s.name !== name) : []; }

    function flash(id, ms = 300) {
        const set = j => { flashAge[j] = 0; flashDur[j] = Math.max(0.05, ms / 1000); };
        if (id === 'all') flashDefs.forEach((_, j) => set(j));
        else { const j = flashIndex.get(id); if (j !== undefined) set(j); }
    }

    function giSet(id, mode, ms = 0) {
        const apply = k => { gi[k].mode = modeCode(mode); gi[k].left = ms > 0 ? ms / 1000 : 0; };
        if (id === 'all') giDefs.forEach((_, k) => apply(k));
        else { const k = giIndex.get(id); if (k !== undefined) apply(k); }
    }

    function approach(cur, target, dt, rise, fall) {
        const tau = target > cur ? rise : fall;
        return cur + (target - cur) * (1 - Math.exp(-dt / tau));
    }

    function update(dt) {
        if (!(dt > 0)) dt = 0;
        clock += dt;
        for (const s of shows) s.t += dt;
        shows = shows.filter(s => {
            if (s.t < s.duration) return true;
            if (--s.loop > 0) { s.t -= s.duration; return true; }
            return false;
        });
        for (let i = 0; i < n; i++) {
            let target;
            const mode = modes[i];
            if (mode === 5) {
                flickHold[i] -= dt;
                if (flickHold[i] <= 0) { flickVal[i] = rngNext(rng); flickHold[i] = 0.03 + rngNext(rng) * 0.06; }
                target = modeLevel(5, clock, flickVal[i]);
            } else target = modeLevel(mode, clock, 1);
            let tn = null;
            for (const s of shows) {
                const v = s.fn(views[i], s.t, s.params);
                if (s.replace) target = v; else if (v > target) target = v;
                if (s.name === 'rainbow' && defs[i].rgb) tn = hsvToRgb(hueOf(views[i], s.t, s.params));
                else if (s.color && v > 0.05) tn = s.color;
            }
            tint[i] = tn;
            intensity[i] = approach(intensity[i], target < 0 ? 0 : target > 1 ? 1 : target, dt, TAU_RISE, TAU_FALL);
        }
        for (let j = 0; j < flashDefs.length; j++) {
            flashAge[j] += dt;
            const t = flashAge[j], d = flashDur[j];
            if (t >= d) { flashBuf[j] = 0; continue; }
            const hold = Math.min(FLASH_HOLD, d * 0.3);
            if (t <= hold) flashBuf[j] = 1;
            else { const k = 1 - (t - hold) / (d - hold); flashBuf[j] = k * k; }
        }
        for (let k = 0; k < giDefs.length; k++) {
            const g = gi[k];
            if (g.left > 0) { g.left -= dt; if (g.left <= 0) { g.left = 0; g.mode = 1; } }
            if (g.mode === 5) {
                g.flickHold -= dt;
                if (g.flickHold <= 0) { g.flickVal = rngNext(rng); g.flickHold = 0.03 + rngNext(rng) * 0.06; }
            }
            giLevel[k] = approach(giLevel[k], modeLevel(g.mode, clock, g.flickVal), dt, GI_TAU, GI_TAU);
        }
        for (let i = 0; i < n && i < 255; i++) {
            const c = tint[i] || rgb[i], f = intensity[i], o = (i + 1) * 4;
            rgba[o] = Math.round(c[0] * f); rgba[o + 1] = Math.round(c[1] * f); rgba[o + 2] = Math.round(c[2] * f); rgba[o + 3] = 255;
        }
    }

    function inspect() {
        let lit = 0;
        for (let i = 0; i < n; i++) if (intensity[i] > 0.05) lit++;
        return { count: n, lit, shows: shows.map(s => s.name) };
    }

    rgba[3] = 255;
    return { setModes, setColor, show, stopShows, flash, gi: giSet, update, rgba, intensity, flashLevel: flashBuf, giLevel, inspect, index: idIndex, modes };
}

export default createLamps;
