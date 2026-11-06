// Stingers, interface sounds, horns, the ice-cream van jingle and the fanfares (own synthesis).
import {
    SR, TAU, mk, secs, rng, seedOf, filter, svf, Osc, add, addWrap, scale, sum, white, softClip, monoReverb, reverb, chorus, modal, pluck, loopify,
    thump, noiseBurst, whoosh, tone, sparkle, clicks, chime, SAW_H, SQUARE_H, mixVowel, formantVoice, fadeEdges, clamp, dbToGain, mtof
} from './kit.mjs';
import { mooVoice, WORLD_RECIPES } from './sfx-world.mjs';
import { airHorn } from './sfx-weapons.mjs';

const R = [];
const def = (group, variants, render, extra = {}) => R.push({ group, kind: 'sfx', variants, channels: 1, render, ...extra });
const defStereo = (group, variants, render) => def(group, variants, render, { channels: 2 });

// Brass-like voice: two detuned saws through an opening low-pass; mute = narrow wah band (sad trombone).
function brass(f, len, { attack = 0.04, vib = 0.004, vibHz = 5.2, bright = 6, release = 0.08, droop = 0, mute = false, level = 1 } = {}) {
    const n = secs(len), a = new Osc(0), b = new Osc(0.37), x = mk(n);
    for (let i = 0; i < n; i++) {
        const t = i / SR, fr = f * (1 - droop * Math.max(0, t / len - 0.6)) * (1 + vib * Math.min(1, t / 0.3) * Math.sin(TAU * vibHz * t));
        x[i] = a.saw(fr) + b.saw(fr * 1.004) * 0.8;
    }
    const open = svf(x, mute ? 'bp' : 'lp', i => { const t = i / SR, e = Math.min(1, t / (attack * 2.2)); return mute ? f * (2 + 3 * e + (t > len - 0.25 ? -1 : 0)) : f * (1.5 + bright * e * (1 - 0.35 * t / len)); }, mute ? 1.6 : 0.9);
    for (let i = 0; i < n; i++) { const t = i / SR; open[i] *= level * Math.min(1, t / attack) * Math.min(1, (len - t) / release) * (0.9 + 0.1 * Math.sin(TAU * 4.5 * t)); }
    return open;
}
const timpani = (f, len = 0.9) => sum(secs(len), [thump(len, f * 1.5, f, 0.03, len / 3.2, 1.2), scale(noiseBurst(rng(f | 0), 0.05, 0.01, { bp: 900, q: 1 }), 0.6)]);
const cymbal = (r, len = 1.6) => noiseBurst(r, len, len / 3.5, { hp: 5500 });
function spread(mono, tail = 0.0, wet = 0.18, size = 0.6) { const [l, r] = chorus(mono, { rate: 0.31, depth: 0.0025, base: 0.011, mix: 0.45 }); return reverb([l, r], { size, wet, tail }); }

// ---- stings ----
def('sting.perfect', 1, (v, r) => {
    const n = secs(1.3), o = mk(n);
    [[0, 784], [0.08, 988], [0.16, 1175], [0.25, 1568]].forEach(([at, f]) => add(o, chime(f, 0.8, 0.45), at, 0.8));
    add(o, sparkle(r, 0.8, 18, 3000, 9500, 0.2), 0.25, 0.3);
    return [monoReverb(o, { size: 0.4, wet: 0.1 })];
});
def('sting.pancake', 1, (v, r) => {
    const n = secs(1.3), o = sum(n, [scale(noiseBurst(r, 0.35, 0.07, { kind: 'pink', lp: 1800 }), 1.1), thump(0.3, 160, 60, 0.03, 0.08, 1.4)]);
    add(o, tone(0.75, t => 520 * Math.exp(-t / 0.4) + 110, { harmonics: SAW_H.slice(0, 6), env: t => Math.min(1, t / 0.02) * Math.exp(-t / 0.35) * 0.28, vib: 0.03, vibHz: 7 }), 0.1);
    add(o, tone(0.5, t => 180 + 300 * Math.exp(-t / 0.08) * Math.sin(TAU * 9 * t), { harmonics: [[1, 1], [2, 0.4], [3, 0.2]], env: t => Math.exp(-t / 0.18) * 0.3 }), 0.72);
    return [o];
});
def('sting.strike', 1, (v, r) => {
    const n = secs(1.8), o = mk(n);
    for (let k = 0; k < 80; k++) addWrap(o, modal(280 + 1100 * r(), [[1, 1, 0.05 + 0.05 * r()], [2.3, 0.3, 0.03]], secs(0.2)), secs(0.3 * Math.pow(r(), 1.4)), 0.15 + 0.45 * r());
    for (let k = 0; k < 14; k++) addWrap(o, modal(320 + 900 * r(), [[1, 1, 0.07]], secs(0.15)), secs(0.3 + 0.5 * r()), 0.2 * r());
    add(o, noiseBurst(r, 0.8, 0.3, { kind: 'brown', lp: 400 }), 0, 1.5);
    add(o, chime(1318, 1.0, 0.7), 0.5, 0.55); add(o, chime(1760, 0.9, 0.6), 0.62, 0.35);
    return [monoReverb(o, { size: 0.4, wet: 0.1 })];
});
def('sting.slingshot', 1, (v, r) => {
    const n = secs(1.1), stretch = tone(0.5, t => 80 + 130 * Math.pow(t / 0.5, 1.4), { harmonics: SAW_H.slice(0, 6), env: t => Math.min(1, t / 0.05) * 0.18, vib: 0.02, vibHz: 14 });
    const o = sum(n, [stretch]);
    add(o, pluck(246, secs(0.5), r, { decay: 0.999, bright: 0.9 }), 0.5, 0.8); add(o, whoosh(r, 0.45, 4200, 700, 0.9, 0.2), 0.5, 0.9); add(o, noiseBurst(r, 0.03, 0.005, { hp: 3000 }), 0.5, 0.9);
    return [o];
});
def('sting.moo', 1, (v, r) => [mooVoice(r, { len: 0.9, lo: 110, hi: 205, end: 120, shift: 1.05, rise: 0.22 })]);
def('sting.trick', 1, (v, r) => {
    const n = secs(0.8), o = mk(n);
    add(o, chime(880, 0.4, 0.3), 0, 0.8); add(o, chime(1319, 0.6, 0.4), 0.09, 0.9); add(o, sparkle(r, 0.5, 12, 3500, 9000, 0.12), 0.1, 0.3);
    return [o];
});
function turbo(level) {
    return (v, r) => {
        const d = [0.55, 0.75, 1.05][level - 1], n = secs(d + 0.2), x = svf(white(n, r), 'bp', i => 650 + (1600 + level * 700) * Math.min(1, i / (0.25 * SR)), 0.8), o = mk(n);
        for (let i = 0; i < n; i++) { const t = i / SR; o[i] = x[i] * Math.min(1, t / 0.02) * Math.exp(-t / (0.14 + 0.1 * level)) * 2.2; }
        const flame = svf(white(n, r), 'lp', 420 + 120 * level, 0.7); for (let i = 0; i < n; i++) o[i] += flame[i] * Math.exp(-i / SR / (0.2 + 0.1 * level)) * 1.4;
        const whistle = tone(d, t => 1800 * Math.pow(1.9 + 0.25 * level, t / d), { env: t => Math.min(1, t / 0.05) * Math.exp(-t / (0.2 + 0.08 * level)) * 0.07 * level });
        add(o, whistle, 0, 1); add(o, thump(0.2, 130 + 30 * level, 55, 0.02, 0.06 + 0.02 * level, 1.4), 0, 0.5 + 0.2 * level);
        for (let k = 0; k < level * 3; k++) add(o, noiseBurst(r, 0.025, 0.005, { bp: 1100 + 700 * r(), q: 2 }), 0.12 + 0.35 * r() * level / 3, 0.5);
        if (level === 3) add(o, thump(0.6, 70, 30, 0.1, 0.2, 1.6), 0.02, 0.9);
        return [softClip(o, 1 + level * 0.3)];
    };
}
def('turbo.1', 1, turbo(1)); def('turbo.2', 1, turbo(2)); def('turbo.3', 1, turbo(3));
def('turbo.tick', 3, (v, r) => [sum(secs(0.1), [scale(noiseBurst(r, 0.04, 0.004, { bp: 2800 + v * 500, q: 2 }), 1), tone(0.04, () => 2100 + v * 250, { env: t => Math.exp(-t / 0.008) * 0.5 })])]);
def('pad.boost', 1, (v, r) => {
    const n = secs(1.0), saw = tone(0.8, t => 180 * Math.pow(8, Math.pow(t / 0.8, 0.9)), { harmonics: SAW_H.slice(0, 10), env: t => Math.min(1, t / 0.04) * Math.exp(-t / 0.35) });
    return [softClip(sum(n, [scale(saw, 0.5), scale(whoosh(r, 0.8, 500, 5200, 0.8, 0.35), 0.8), add(mk(n), sparkle(r, 0.6, 12, 3000, 9000, 0.15), 0.1, 0.35), thump(0.15, 140, 70, 0.02, 0.04)]), 1.3)];
});
def('ramp.launch', 1, (v, r) => {
    const n = secs(1.1), spring = tone(0.7, t => 160 + 1100 * Math.pow(t / 0.7, 0.8) * (1 + 0.1 * Math.sin(TAU * 11 * t)), { harmonics: [[1, 1], [2, 0.4], [3, 0.2]], env: t => Math.min(1, t / 0.01) * Math.exp(-t / 0.45) * 0.5 });
    return [sum(n, [spring, scale(whoosh(r, 0.9, 400, 2800, 0.9, 0.4), 0.7), thump(0.2, 180, 70, 0.02, 0.05, 1.3)])];
});
def('land.soft', 2, (v, r) => [sum(secs(0.5), [thump(0.35, 130 - v * 20, 62, 0.025, 0.07, 1.3), scale(noiseBurst(r, 0.2, 0.05, { lp: 800 }), 0.8), add(mk(secs(0.5)), tone(0.2, t => 240 + 200 * Math.sin(TAU * 8 * t) * Math.exp(-t / 0.08), { harmonics: [[1, 1], [2, 0.3]], env: t => Math.exp(-t / 0.07) * 0.2 }), 0.07, 1)])]);
def('land.hard', 2, (v, r) => {
    const n = secs(0.9), o = sum(n, [thump(0.6, 110, 40, 0.04, 0.14, 1.6), scale(noiseBurst(r, 0.3, 0.06, { lp: 2800 }), 0.9), scale(noiseBurst(r, 0.05, 0.008, { hp: 1000 }), 0.8)]);
    add(o, tone(0.5, t => 280 + 160 * Math.sin(TAU * 17 * t) * Math.exp(-t / 0.2), { harmonics: SAW_H.slice(0, 5), env: t => Math.exp(-t / 0.18) * 0.12 }), 0.05);
    add(o, thump(0.3, 100, 50, 0.03, 0.08, 1.3), 0.27, 0.45); add(o, clicks(r, 0.4, 10, 2400, 3, 0.004), 0.1, 0.4);
    return [o];
});
def('roulette.tick', 3, (v, r) => [sum(secs(0.1), [modal(1300 + v * 280, [[1, 1, 0.02], [2.7, 0.4, 0.012]], secs(0.1)), scale(noiseBurst(r, 0.02, 0.003, { bp: 3200, q: 1.5 }), 0.5)])]);
def('roulette.done', 1, (v, r) => {
    const n = secs(1.1), o = mk(n);
    add(o, chime(1047, 0.7, 0.5), 0, 0.9); add(o, chime(1568, 0.9, 0.55), 0.13, 0.9); add(o, sparkle(r, 0.6, 14, 3000, 9000, 0.14), 0.14, 0.3);
    return [o];
});
def('bomb.tick', 2, (v, r) => [sum(secs(0.15), [modal(v ? 1000 : 1550, [[1, 1, 0.035], [2.4, 0.35, 0.02]], secs(0.14)), scale(noiseBurst(r, 0.015, 0.002, { hp: 3500 }), 0.5), thump(0.05, v ? 280 : 360, 150, 0.01, 0.02)])]);
def('bomb.blast', 1, (v, r) => {
    const big = WORLD_RECIPES.find(item => item.group === 'explosion.big').render(1, r), o = Float64Array.from(big[0]);
    add(o, tone(1.2, t => 2200 * Math.exp(-t / 0.55) + 260, { env: t => Math.sin(Math.PI * Math.min(1, t / 1.2)) ** 0.6 * 0.12 }), 0.35);
    return [softClip(o, 1.2)];
});

// ---- ice-cream van jingle: Greensleeves (traditional) in a music-box voice ----
const GREENSLEEVES = [[69, 2], [72, 4], [74, 2], [76, 3], [77, 1], [76, 2], [74, 4], [71, 2], [67, 3], [69, 1], [71, 2], [72, 4], [69, 2], [69, 3], [68, 1], [69, 2], [71, 4], [68, 2], [64, 6]];
function musicBox(r, notes, unit, transpose = 12) {
    const total = notes.reduce((a, [, d]) => a + d, 0) * unit + 1.4, n = secs(total), o = mk(n);
    let t = 0, k = 0;
    for (const [midi, d] of notes) {
        const f = mtof(midi + transpose), warble = 1 + 0.012 * Math.sin(k * 0.9), tine = modal(f, [[1, 1, 1.1], [6.27, 0.35, 0.35], [17.5, 0.12, 0.12], [2.01, 0.12, 0.5]], secs(1.5), { r, jitter: 0.001 });
        add(o, tine, t * warble, 0.5); add(o, noiseBurst(r, 0.012, 0.002, { hp: 4500 }), t * warble, 0.09);
        t += d * unit; k++;
    }
    const filtered = filter(filter(o, 'hp', 320, 0.7), 'lp', 5800, 0.8);
    const [l] = chorus(filtered, { rate: 0.7, depth: 0.0012, base: 0.006, mix: 0.4 });
    return monoReverb(l, { size: 0.35, wet: 0.1 });
}
def('jingle.softeis', 1, (v, r) => [musicBox(r, GREENSLEEVES, 0.17)]);

// ---- horns ----
const reedHonk = (r, f0, f1, len, nasal = true) => formantVoice(secs(len), i => f0 + (f1 - f0) * Math.min(1, i / SR / (len * 0.6)), i => nasal ? [[850, 6, 1], [1500, 8, 0.8], [2600, 10, 0.4]] : [[500, 7, 1], [1100, 8, 0.5]], r,
    { jitter: 0.01, shimmer: 0.04, breath: 0.06, tilt: 5200, pulse: 'square', amp: i => { const t = i / SR; return Math.min(1, t / 0.012) * Math.min(1, (len - t) / 0.06); } });
def('horn.vektor', 1, (v, r) => { const n = secs(0.55), o = mk(n); [0, 0.2].forEach(at => add(o, airHorn(r, [[440, 1], [554, 0.9]], 0.16), at)); return [o]; });
def('horn.kobalt', 1, (v, r) => [airHorn(r, [[420, 1], [500, 0.9]], 0.55)]);
def('horn.lumen', 1, (v, r) => [airHorn(r, [[311, 1], [370, 0.9], [466, 0.5]], 0.7)]);
def('horn.tundra', 1, (v, r) => { const n = secs(0.6), o = mk(n); [0, 0.24].forEach(at => add(o, airHorn(r, [[500, 1], [630, 0.9]], 0.2), at)); return [softClip(o, 1.6)]; });
def('horn.softeis', 1, (v, r) => [musicBox(r, GREENSLEEVES.slice(0, 6), 0.15)]);
def('horn.maehkoenig', 1, (v, r) => { const n = secs(0.6), x = reedHonk(r, 760, 560, 0.28, true), o = mk(n); add(o, x, 0, 1); add(o, reedHonk(r, 820, 600, 0.2, true), 0.32, 0.9); return [o]; });
def('horn.couch', 1, (v, r) => { const n = secs(0.8), o = mk(n); add(o, reedHonk(r, 300, 380, 0.22, false), 0, 0.8); add(o, reedHonk(r, 420, 400, 0.38, true), 0.27, 1); return [softClip(o, 1.3)]; });
def('horn.bertha', 1, (v, r) => [airHorn(r, [[147, 1], [185, 0.9], [220, 0.6]], 1.15, 1)]);

// ---- UI ----
def('ui.count', 1, (v, r) => [sum(secs(0.25), [tone(0.2, () => 523, { harmonics: [[1, 1], [2, 0.35], [3, 0.15]], env: t => Math.min(1, t / 0.006) * Math.exp(-t / 0.07) }), scale(noiseBurst(r, 0.01, 0.002, { hp: 3000 }), 0.2)])]);
def('ui.go', 1, (v, r) => {
    const n = secs(0.8);
    return [sum(n, [tone(0.6, t => 784 * (t < 0.07 ? 0.85 + 0.15 * t / 0.07 : 1), { harmonics: [[1, 1], [2, 0.4], [3, 0.2], [4, 0.1]], env: t => Math.min(1, t / 0.008) * Math.exp(-t / 0.25) }), tone(0.6, t => 1175 * (t < 0.07 ? 0.85 + 0.15 * t / 0.07 : 1), { harmonics: [[1, 1], [2, 0.3]], env: t => Math.min(1, t / 0.008) * Math.exp(-t / 0.2) * 0.7 }), scale(whoosh(r, 0.35, 800, 4500, 1, 0.3), 0.35)])];
});
def('ui.lap', 1, (v, r) => { const o = mk(secs(0.9)); add(o, chime(1047, 0.6, 0.35), 0, 0.9); add(o, chime(1319, 0.7, 0.4), 0.09, 0.8); return [o]; });
def('ui.bank', 1, (v, r) => {
    const o = mk(secs(1.0));
    add(o, clicks(r, 0.12, 7, 3000, 3, 0.003), 0, 0.5); add(o, chime(2093, 0.9, 0.5), 0.1, 0.8); add(o, chime(2637, 0.6, 0.4), 0.14, 0.5); add(o, sparkle(r, 0.5, 9, 4000, 9500, 0.1), 0.12, 0.25);
    return [o];
});
def('whoosh', 3, (v, r) => [whoosh(r, 0.5 + v * 0.1, [900, 600, 1400][v], [3600, 2400, 5200][v], 0.9, 0.4)]);
def('ufo.beam', 1, (v, r) => {
    const n = secs(2.0), main = mk(n);
    const a = tone(2.0, t => (t < 0.5 ? 300 + 500 * t / 0.5 : 800 - 300 * (t - 0.5) / 1.5) * (1 + 0.05 * Math.sin(TAU * 6 * t)), { harmonics: [[1, 1], [2.01, 0.3]], env: t => Math.min(1, t / 0.2) * Math.min(1, (2 - t) / 0.4) * 0.4 });
    const b = tone(2.0, t => 0.5 * (t < 0.5 ? 300 + 500 * t / 0.5 : 800 - 300 * (t - 0.5) / 1.5) * 1.5 * (1 + 0.03 * Math.sin(TAU * 4.5 * t + 1)), { harmonics: [[1, 1]], env: t => Math.min(1, t / 0.3) * Math.min(1, (2 - t) / 0.4) * 0.3 });
    for (let i = 0; i < n; i++) main[i] = a[i] * (0.65 + 0.35 * Math.sin(TAU * 23 * i / SR)) + b[i];
    return [monoReverb(sum(n, [main, add(mk(n), sparkle(r, 1.8, 24, 2500, 8000, 0.25), 0.1, 0.3)]), { size: 0.6, wet: 0.15 })];
});

// ---- stingers and fanfares (stereo) ----
defStereo('stinger.final_lap', 1, (v, r) => {
    const n = secs(2.6), o = mk(n);
    [[0, 392], [0.17, 392], [0.34, 523]].forEach(([at, f], k) => add(o, brass(f, k === 2 ? 1.5 : 0.17, { attack: 0.02, release: 0.1, level: 0.5 }), at + 0.1));
    [[0, 196], [0.17, 196], [0.34, 262]].forEach(([at, f], k) => add(o, brass(f, k === 2 ? 1.5 : 0.17, { attack: 0.025, release: 0.1, level: 0.45 }), at + 0.1));
    for (let k = 0; k < 8; k++) add(o, timpani(98, 0.5), 0.1 + k * 0.05, 0.1 + k * 0.05);
    add(o, timpani(98, 1.2), 0.46, 1); add(o, cymbal(r, 1.6), 0.45, 0.35);
    return spread(o, 0.5, 0.2);
});
defStereo('stinger.disco', 1, (v, r) => {
    const n = secs(2.2), o = mk(n);
    for (let k = 0; k < 4; k++) { add(o, thump(0.25, 120, 48, 0.03, 0.09, 1.4), k * 0.25, 0.9); add(o, noiseBurst(r, 0.06, 0.012, { hp: 8000 }), k * 0.25 + 0.125, 0.45); }
    [0.5, 0.75, 1.0].forEach((at, k) => { const st = mk(secs(0.3)); [220, 277, 330, 415].forEach(f => add(st, tone(0.3, () => f * (k === 2 ? 1.5 : 1), { harmonics: SAW_H.slice(0, 8) }), 0, 0.25)); const w = svf(st, 'lp', i => 500 * Math.pow(5, i / st.length), 2.5); for (let i = 0; i < w.length; i++) w[i] *= Math.exp(-i / (0.12 * SR)); add(o, w, at, 0.7); });
    add(o, sparkle(r, 1.4, 34, 2500, 9500, 0.25), 0.7, 0.4);
    return spread(o, 0.6, 0.2);
});
defStereo('fanfare.win', 1, (v, r) => {
    const n = secs(4.2), o = mk(n), melody = [[0, 0.18, 67], [0.2, 0.18, 72], [0.4, 0.18, 76], [0.6, 0.5, 79], [1.15, 0.2, 76], [1.4, 0.2, 79], [1.65, 1.9, 84]];
    for (const [at, len, m] of melody) { add(o, brass(mtof(m), len, { attack: 0.025, release: 0.1, level: 0.55 }), at + 0.1); add(o, brass(mtof(m - 12), len, { attack: 0.03, release: 0.1, level: 0.4 }), at + 0.1); }
    [[1.65, 76], [1.65, 72], [1.65, 67]].forEach(([at, m]) => add(o, brass(mtof(m), 1.9, { attack: 0.05, release: 0.2, level: 0.28 }), at + 0.1));
    add(o, timpani(130, 1.4), 0.1, 0.9); add(o, timpani(98, 1.4), 1.75, 1); add(o, cymbal(r, 2.2), 1.75, 0.35);
    add(o, sparkle(r, 2, 40, 3000, 9500, 0.3), 1.8, 0.28);
    return spread(o, 0.8, 0.22);
});
defStereo('fanfare.podium', 1, (v, r) => {
    const n = secs(3.2), o = mk(n);
    [[0, 0.22, 72], [0.25, 0.22, 76], [0.5, 0.22, 79], [0.75, 1.5, 84]].forEach(([at, len, m]) => { add(o, brass(mtof(m), len, { attack: 0.03, release: 0.12, level: 0.5, bright: 5 }), at + 0.1); add(o, brass(mtof(m - 12), len, { attack: 0.04, release: 0.12, level: 0.35, bright: 4 }), at + 0.1); });
    add(o, timpani(130, 1.0), 0.1, 0.8); add(o, timpani(98, 1.2), 0.85, 0.9); add(o, cymbal(r, 1.6), 0.85, 0.28); add(o, sparkle(r, 1.4, 22, 3000, 9000, 0.25), 0.9, 0.25);
    return spread(o, 0.7, 0.2);
});
defStereo('fanfare.lose', 1, (v, r) => {
    // sad trombone: four falling notes, the last one sagging with vibrato
    const n = secs(4.6), o = mk(n), notes = [[0, 0.5, 58], [0.62, 0.5, 57], [1.24, 0.5, 56], [1.86, 2.2, 55]];
    notes.forEach(([at, len, m], k) => add(o, brass(mtof(m), len, { attack: 0.07, release: 0.1, mute: true, level: 0.8, vib: k === 3 ? 0.014 : 0.004, vibHz: k === 3 ? 6 : 5, droop: k === 3 ? 0.07 : 0 }), at + 0.05));
    notes.forEach(([at, len, m], k) => add(o, brass(mtof(m), len, { attack: 0.07, release: 0.1, mute: false, bright: 2.5, level: 0.45, vib: k === 3 ? 0.014 : 0.004, vibHz: k === 3 ? 6 : 5, droop: k === 3 ? 0.07 : 0 }), at + 0.05));
    return spread(softClip(o, 1.2), 0.5, 0.14, 0.4);
});

export const COMIC_RECIPES = R;
void seedOf; void dbToGain; void fadeEdges; void clamp; void mixVowel; void loopify; void addWrap;
