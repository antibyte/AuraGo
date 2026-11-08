// Light shows: SHOWS[name](lamp, t, params) -> 0..1. DOM-free and deterministic. `lamp` is the controller's lamp view
// ({index, x, y, u, v, seq, seqN, group, rgb, ...}); t is seconds since the show started. Params: x, y (origin),
// duration (s), width/band/tail, speed, seed, color (tint, handled by the controller).
import { FIELD_W, FIELD_H, FIELD_X0 } from './shapes.js';

const TAU = Math.PI * 2;
const clamp01 = v => (v < 0 ? 0 : v > 1 ? 1 : v);
const dur = p => (p && p.duration > 0 ? p.duration : 1);
const MAX_R = Math.hypot(FIELD_W, FIELD_H) * 0.8;

export const SHOW_DEFAULTS = Object.freeze({
    sweepUp: { duration: 0.8 }, sweepDown: { duration: 0.8 }, sweepLeft: { duration: 0.8 }, sweepRight: { duration: 0.8 },
    radial: { duration: 1.2 }, ring: { duration: 1.2 }, chase: { duration: 1.0 }, strobe: { duration: 0.8 },
    sparkle: { duration: 2.0 }, spiral: { duration: 1.6 }, breath: { duration: 3.2 }, wipeDiagonal: { duration: 1.0 },
    rainbow: { duration: 3.0 }, blackout: { duration: 1.0, replace: true }
});

function ux(lamp) { return lamp.u !== undefined ? lamp.u : (lamp.x - FIELD_X0) / FIELD_W; }
function vy(lamp) { return lamp.v !== undefined ? lamp.v : lamp.y / FIELD_H; }

// A bright front travelling over coordinate c in 0..1 with a trailing glow; rises as the front passes.
function sweep(c, t, p) {
    const w = p.width > 0 ? p.width : 0.35, f = (t / dur(p)) * (1 + w) - w * 0.15;
    const d = f - c;
    return d < 0 ? 0 : clamp01(1 - d / w);
}

function origin(lamp, p) {
    const ox = p.x !== undefined ? p.x : 0, oy = p.y !== undefined ? p.y : 0.5;
    return Math.hypot(lamp.x - ox, lamp.y - oy);
}

function hash(a, b, seed) {
    let h = Math.imul(a + 1, 0x9E3779B1) ^ Math.imul(b + 7, 0x85EBCA6B) ^ Math.imul((seed | 0) + 13, 0xC2B2AE35);
    h ^= h >>> 15; h = Math.imul(h, 0x2C1B3C6D); h ^= h >>> 12; h = Math.imul(h, 0x297A2D39); h ^= h >>> 15;
    return (h >>> 0) / 4294967296;
}

// Hue 0..1 for the rainbow show (the controller tints rgb lamps with it).
export function hueOf(lamp, t, p = {}) {
    const h = ux(lamp) * 0.6 + vy(lamp) * 0.4 - t * (p.speed || 0.5);
    return h - Math.floor(h);
}

export function hsvToRgb(h, s = 1, v = 1) {
    const i = Math.floor(h * 6) % 6, f = h * 6 - Math.floor(h * 6), p = v * (1 - s), q = v * (1 - f * s), r = v * (1 - (1 - f) * s);
    const c = [[v, r, p], [q, v, p], [p, v, r], [p, q, v], [r, p, v], [v, p, q]][i];
    return [Math.round(c[0] * 255), Math.round(c[1] * 255), Math.round(c[2] * 255)];
}

export const SHOWS = {
    sweepUp: (l, t, p = {}) => sweep(vy(l), t, p),
    sweepDown: (l, t, p = {}) => sweep(1 - vy(l), t, p),
    sweepLeft: (l, t, p = {}) => sweep(1 - ux(l), t, p),
    sweepRight: (l, t, p = {}) => sweep(ux(l), t, p),
    wipeDiagonal: (l, t, p = {}) => sweep((ux(l) + vy(l)) / 2, t, p),
    radial(l, t, p = {}) {
        const w = p.width > 0 ? p.width : 0.3, f = (t / dur(p)) * (1 + w) * MAX_R, d = f - origin(l, p);
        return d < 0 ? 0 : clamp01(1 - d / (w * MAX_R));
    },
    ring(l, t, p = {}) {
        const band = p.band > 0 ? p.band : 0.07, f = (t / dur(p)) * (MAX_R + band);
        return clamp01(1 - Math.abs(origin(l, p) - f) / band);
    },
    chase(l, t, p = {}) {
        const n = l.seqN || 1, tail = p.tail > 0 ? p.tail : Math.max(3, n * 0.15), head = (t / dur(p)) * (n + tail);
        const d = head - (l.seq || 0);
        return d < 0 ? 0 : clamp01(1 - d / tail);
    },
    strobe(l, t) { const f = t * 12; return f - Math.floor(f) < 0.5 ? 1 : 0; },
    sparkle(l, t, p = {}) {
        const step = Math.floor(t * (p.rate || 14));
        const a = hash(l.index || 0, step, p.seed), b = hash(l.index || 0, step - 1, p.seed);
        return clamp01((a > 0.72 ? 1 : 0) * 0.9 + (b > 0.72 ? 0.25 : 0));
    },
    spiral(l, t, p = {}) {
        const ox = p.x !== undefined ? p.x : 0, oy = p.y !== undefined ? p.y : 0.5;
        const ang = Math.atan2(l.y - oy, l.x - ox) / TAU, r = Math.hypot(l.x - ox, l.y - oy) / MAX_R;
        const ph = ang + r * 2.2 - t * 1.4, wave = ph - Math.floor(ph);
        return clamp01(1 - wave * 2.2) * clamp01(t * 6);
    },
    breath(l, t, p = {}) { const per = p.period > 0 ? p.period : 1.6; return 0.5 - 0.5 * Math.cos((TAU * t) / per); },
    rainbow(l, t, p = {}) {
        if (l.rgb) return 1;
        return 0.5 + 0.5 * Math.sin(TAU * hueOf(l, t, p) * 2);
    },
    blackout: () => 0
};

export default SHOWS;
