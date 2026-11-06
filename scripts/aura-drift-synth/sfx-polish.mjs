// Polish-round stingers: feedback for near misses, drift chains, style banks, cloaking and sectors.
// Own synthesis, seeded, no clock and no Math.random. Every group has two or three variants so the
// bank can pick a fresh one on each play; the runtime also has a live Web Audio fallback recipe.
import {
    mk, secs, svf, add, scale, sum, white, softClip, monoReverb, modal, tone, noiseBurst, whoosh,
    sparkle, chime, thump, fadeEdges
} from './kit.mjs';

const R = [];
const def = (group, variants, render) => R.push({ group, kind: 'sfx', variants, channels: 1, render });

// A tight doppler pass: band-passed noise sweeps down while a whistle falls with it.
def('sting.near_miss', 3, (v, r) => {
    const dur = 0.38 + v * 0.07, n = secs(dur + 0.16), o = mk(n);
    const f0 = [1600, 1150, 2050][v], f1 = [330, 420, 540][v];
    add(o, whoosh(r, dur, f0, f1, 1.6, 0.3, { curve: 1.6 }), 0, 0.95);
    add(o, tone(dur, t => 2300 * Math.pow(f1 / 1500, t / dur), {
        env: t => Math.sin(Math.PI * Math.min(1, t / dur)) * 0.055
    }), 0.02, 0.55);
    add(o, noiseBurst(r, 0.05, 0.01, { hp: 4400 }), 0.035, 0.3);
    return [fadeEdges(softClip(o, 1.05))];
});

// Four ascending bells plus a dust of high sparkles; the multiplier picks the playback rate.
def('sting.style_bank', 2, (v, r) => {
    const n = secs(0.9), o = mk(n);
    const notes = v ? [659, 880, 1175, 1568] : [523, 784, 1046, 1318];
    notes.forEach((f, i) => add(o, chime(f, 0.55, 0.3), i * 0.065, 0.5 - i * 0.05));
    add(o, sparkle(r, 0.5, 14, 4200, 11000, 0.1), 0.12, 0.22);
    return [monoReverb(o, { size: 0.35, wet: 0.12 })];
});

// Short wooden marimba ticks: one per drift-chain link, pitch rises with the variant.
def('sting.drift_link', 3, (v, r) => {
    const n = secs(0.3), f = [420, 520, 640][v];
    const o = sum(n, [
        scale(modal(f, [[1, 1, 0.085], [2.7, 0.34, 0.05], [5.1, 0.15, 0.03]], n), 0.85),
        scale(noiseBurst(r, 0.026, 0.007, { bp: f * 4, q: 2 }), 0.45)
    ]);
    return [fadeEdges(o)];
});

// A shimmer that swallows the car: filtered noise sweeps up, a detuned sine wobbles, a bell fades in.
def('sting.cloak', 2, (v, r) => {
    const n = secs(1.05), o = mk(n);
    const sweep = svf(white(n, r), 'bp', i => 380 + (v ? 4400 : 2700) * Math.pow(i / n, 1.6), 2.2);
    for (let i = 0; i < n; i++) o[i] = sweep[i] * Math.sin(Math.PI * i / n) * 0.9;
    add(o, tone(0.85, t => (v ? 660 : 520) * (1 + 0.5 * t), {
        env: t => Math.sin(Math.PI * Math.min(1, t / 0.85)) * 0.065, vib: 0.02, vibHz: 11
    }), 0.05, 0.7);
    add(o, chime(v ? 1760 : 1318, 0.6, 0.35), 0.3, 0.22);
    return [monoReverb(o, { size: 0.5, wet: 0.2 })];
});

// Two-note sector split chime with a soft low thump.
def('sting.sector', 2, (v, r) => {
    const n = secs(0.72), o = mk(n), base = v ? 784 : 659;
    add(o, chime(base, 0.45, 0.3), 0, 0.7);
    add(o, chime(base * 1.5, 0.5, 0.34), 0.1, 0.55);
    add(o, thump(0.12, 180, 80, 0.01, 0.04, 1.1), 0, 0.3);
    return [monoReverb(o, { size: 0.3, wet: 0.1 })];
});

export const POLISH_RECIPES = R;
