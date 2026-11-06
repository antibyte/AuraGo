// Weapon sounds: readable at a glance and a little silly (air horn, rubber duck, boing, slide-whistle cow).
import {
    SR, TAU, mk, secs, filter, svf, add, addWrap, scale, sum, white, pink, brown, formantVoice, clamp, softClip, monoReverb, echo, modal, pluck,
    loopify, loopHz, thump, noiseBurst, whoosh, tone, sparkle, clicks, mixVowel, SAW_H, SQUARE_H, chime, pulseEnv, expDecay, fadeEdges
} from './kit.mjs';
import { mooVoice } from './sfx-world.mjs';

const R = [];
const def = (group, variants, render, extra = {}) => R.push({ group, kind: 'sfx', variants, channels: 1, render, ...extra });

// A raspy, noise-driven "wak": falling pitch, turbulent source (high jitter and breath), nasal formants and a rough amplitude flutter.
export function duckQuack(r, pitch = 1, len = 0.24) {
    const n = secs(len), f0 = i => ((480 - 340) * Math.exp(-i / SR / 0.08) + 340) * pitch;
    const voice = formantVoice(n, f0, i => { const t = i / n; return [[(1000 - 250 * t) * Math.sqrt(pitch), 5, 1], [2100, 7, 0.9], [3100, 9, 0.4]]; }, r,
        { jitter: 0.2, shimmer: 0.12, breath: 1.2, tilt: 5000, pulse: 'saw', amp: i => { const t = i / n; return Math.min(1, t / 0.02) * Math.pow(Math.min(1, (1 - t) / 0.3), 0.9) * (0.3 + 0.7 * Math.sin(TAU * 45 * i / SR)); } });
    const squeak = tone(0.05, t => (2300 - 700 * t / 0.05) * pitch, { harmonics: [[1, 1], [2, 0.3]], env: t => Math.sin(Math.PI * Math.min(1, t / 0.05)) * 0.1 });
    return add(voice, squeak, 0, 1);
}
const quackScaled = (r, pitch, len, gain) => scale(duckQuack(r, pitch, len), gain);

function airHorn(r, notes, len, sink = 0.9) {
    const n = secs(len), out = mk(n);
    notes.forEach(([f, g], k) => {
        const x = tone(len, t => f * (t < 0.045 ? 0.88 + 0.12 * (t / 0.045) : 1) * (1 + 0.003 * Math.sin(TAU * (5.5 + k) * t)), { harmonics: SAW_H.concat(Array.from({ length: 6 }, (_, q) => [15 + q, 0.04])), phase: k });
        add(out, x, 0, g);
    });
    const bell = svf(out, 'bp', 1150, 0.7), bell2 = svf(out, 'bp', 2400, 1.2);
    for (let i = 0; i < n; i++) {
        const t = i / SR, env = Math.min(1, t / 0.018) * (t > len - 0.16 ? Math.max(0, (len - t) / 0.16) : 1) * (1 - 0.1 * sink * (t / len)) * (t < 0.07 ? 1 : 0.5 + 0.5 * Math.exp(-(t - 0.07) / 0.08));
        out[i] = (out[i] * 0.35 + bell[i] * 1.1 + bell2[i] * 0.5) * env;
    }
    const rasp = filter(white(n, r), 'bp', 1800, 0.8); for (let i = 0; i < n; i++) out[i] += rasp[i] * 0.05 * Math.min(1, i / 1500);
    return softClip(out, 1.15);
}

def('weapon.oil', 2, (v, r) => {
    const n = secs(0.95), o = mk(n);
    for (let k = 0; k < 8; k++) {
        const f0 = 260 + 380 * r() + v * 40, len = 0.045 + 0.05 * r(), g = 0.35 + 0.65 * r();
        add(o, tone(len, t => f0 * (1 + 1.1 * t / len), { env: t => Math.exp(-t / (len * 0.45)) * Math.min(1, t / 0.003) }), 0.05 + 0.5 * r() * r(), g);
    }
    return [sum(n, [o, thump(0.3, 220, 80, 0.03, 0.07, 1.4), scale(noiseBurst(r, 0.7, 0.18, { lp: 1300, kind: 'pink' }), 0.8)])];
});
def('weapon.mine_drop', 2, (v, r) => {
    const n = secs(0.95), o = sum(n, [thump(0.25, 190, 70, 0.02, 0.06, 1.3), scale(modal(520 + v * 70, [[1, 1, 0.25], [2.4, 0.5, 0.15], [4.2, 0.2, 0.1]], secs(0.4)), 0.5), scale(noiseBurst(r, 0.06, 0.01, { bp: 2000, q: 1.2 }), 0.5)]);
    add(o, thump(0.12, 160, 90, 0.015, 0.04), 0.17, 0.4);
    [[0.42, 1300], [0.56, 1750]].forEach(([at, f]) => add(o, tone(0.07, () => f, { harmonics: SQUARE_H.slice(0, 4), env: t => Math.min(1, t / 0.004) * Math.min(1, (0.07 - t) / 0.01) * 0.28 }), at));
    return [o];
});
def('weapon.emp_charge', 1, (v, r) => {
    const n = secs(0.8), x = tone(0.8, t => 160 * Math.pow(15, Math.pow(t / 0.8, 1.2)) * (1 + 0.05 * Math.sin(TAU * 58 * t)), { harmonics: SAW_H.slice(0, 8), env: t => Math.min(1, t / 0.08) * (0.3 + 0.7 * t / 0.8) });
    const zap = mk(n); for (let k = 0; k < 40; k++) { const at = 0.8 * Math.pow(r(), 0.5); addWrap(zap, noiseBurst(r, 0.012, 0.002, { hp: 3000 }), secs(at), 0.5 * (0.2 + at)); }
    return [softClip(sum(n, [scale(x, 0.5), zap, scale(filter(white(n, r), 'bp', 3000, 3), 0.04)]), 1.4)];
});
def('weapon.emp_pulse', 1, (v, r) => {
    const n = secs(1.3), sweep = tone(0.7, t => 3400 * Math.exp(-t / 0.18) + 150, { harmonics: SQUARE_H, env: t => Math.exp(-t / 0.2) });
    const zap = mk(n); for (let k = 0; k < 60; k++) addWrap(zap, noiseBurst(r, 0.02, 0.003, { hp: 2200 }), secs(0.6 * Math.pow(r(), 1.8)), 0.6 * r());
    return [softClip(sum(n, [thump(0.9, 100, 38, 0.06, 0.25, 1.5), scale(sweep, 0.5), zap, scale(noiseBurst(r, 0.3, 0.07, { hp: 800 }), 0.6)]), 1.5)];
});
def('weapon.rocket_fire', 2, (v, r) => {
    const n = secs(1.5), hiss = svf(white(n, r), 'bp', i => 700 + 3200 * Math.min(1, i / (0.7 * SR)), 0.7), x = mk(n);
    for (let i = 0; i < n; i++) { const t = i / SR; x[i] = hiss[i] * Math.min(1, t / 0.05) * Math.exp(-t / 0.65) * 2.4; }
    const rumble = noiseBurst(r, 1.4, 0.5, { kind: 'brown', lp: 260 }); for (let i = 0; i < rumble.length; i++) rumble[i] *= 3;
    return [softClip(sum(n, [x, rumble, thump(0.3, 150, 60, 0.04, 0.09, 1.3), scale(noiseBurst(r, 0.06, 0.01, { hp: 1500 }), 0.7)]), 1.4)];
});
def('weapon.rocket_lock', 1, (v, r) => {
    const n = secs(1.3), o = mk(n); let t = 0, gap = 0.26;
    while (t < 1.0) { add(o, tone(0.045, () => 1900, { harmonics: SQUARE_H.slice(0, 3), env: x => Math.min(1, x / 0.004) * Math.min(1, (0.045 - x) / 0.008) * 0.5 }), t); t += gap; gap = Math.max(0.06, gap * 0.8); }
    add(o, tone(0.28, () => 1900, { harmonics: SQUARE_H.slice(0, 3), env: x => Math.min(1, x / 0.004) * Math.min(1, (0.28 - x) / 0.02) * 0.5 }), t + 0.02);
    return [o];
});
def('weapon.mg', 3, (v, r) => {
    const n = secs(0.2);
    return [sum(n, [scale(noiseBurst(r, 0.12, 0.016, { bp: 2000 + v * 400, q: 0.9 }), 1), thump(0.14, 260 - v * 20, 120, 0.012, 0.03, 1.3), scale(tone(0.05, t => 620 + 550 * t / 0.05, { env: t => Math.exp(-t / 0.02) * 0.4 }), 1)])];
});
def('weapon.shield_on', 1, (v, r) => {
    const n = secs(0.9), g = tone(0.5, t => 480 * Math.pow(3.2, t / 0.5), { harmonics: [[1, 1], [2, 0.25], [3, 0.08]], env: t => Math.min(1, t / 0.03) * Math.exp(-t / 0.28) });
    return [monoReverb(sum(n, [scale(g, 0.6), add(mk(n), sparkle(r, 0.6, 14, 1800, 7000, 0.18), 0.12, 0.4), scale(whoosh(r, 0.5, 600, 3200, 1.2, 0.6), 0.3)]), { size: 0.5, wet: 0.14 })];
});
def('weapon.shield_hit', 2, (v, r) => {
    const n = secs(0.6), boing = tone(0.35, t => 300 + 700 * Math.exp(-t / 0.05) + v * 60, { harmonics: [[1, 1], [2, 0.4], [3, 0.15]], env: t => Math.exp(-t / 0.1) });
    return [sum(n, [scale(boing, 0.6), scale(chime(1900 + v * 300, 0.5, 0.5), 0.5), thump(0.1, 200, 110, 0.015, 0.03, 1.2), scale(noiseBurst(r, 0.04, 0.008, { hp: 3000 }), 0.4)])];
});
def('weapon.duck_launch', 1, (v, r) => {
    const n = secs(1.3), o = mk(n);
    [[0, 1.0], [0.13, 1.12], [0.26, 0.94], [0.4, 1.2]].forEach(([at, p]) => add(o, quackScaled(r, p, 0.2, 1), at));
    const flaps = mk(n); for (let k = 0; k < 18; k++) addWrap(flaps, noiseBurst(r, 0.05, 0.012, { bp: 1100, q: 0.7 }), secs(0.05 + k * 0.055), 0.25);
    return [sum(n, [o, flaps, scale(whoosh(r, 0.9, 500, 2500, 0.9, 0.4), 0.3)])];
});
def('weapon.duck_quack', 3, (v, r) => [monoReverb(duckQuack(r, [1.05, 1.17, 1.32][v], 0.26 + v * 0.02), { size: 0.2, wet: 0.05, tail: 0.1 })]);
def('weapon.rail_charge', 1, (v, r) => {
    const n = secs(0.7), x = tone(0.7, t => 90 * Math.pow(60, Math.pow(t / 0.7, 1.3)), { harmonics: SAW_H.slice(0, 7), env: t => Math.min(1, t / 0.05) * (0.25 + 0.75 * t / 0.7) });
    const hum = tone(0.7, t => 50 + 40 * t / 0.7, { harmonics: [[1, 1], [2, 0.5]], env: () => 0.35 });
    return [softClip(sum(n, [scale(x, 0.45), hum, scale(clicks(r, 0.7, 36, 5000, 2, 0.002, 0.6), 0.45)]), 1.3)];
});
def('weapon.rail_fire', 1, (v, r) => {
    const n = secs(1.8), pew = tone(0.5, t => 7000 * Math.exp(-t / 0.07) + 140, { harmonics: SQUARE_H, env: t => Math.exp(-t / 0.16) });
    const x = sum(n, [scale(pew, 0.6), scale(noiseBurst(r, 0.2, 0.02, { hp: 1500 }), 1.1), thump(0.7, 90, 32, 0.08, 0.2, 1.5), scale(noiseBurst(r, 0.8, 0.2, { kind: 'pink', lp: 900 }), 0.5)]);
    return [monoReverb(echo(softClip(x, 1.6), { time: 0.07, feedback: 0.45, mix: 0.4, damp: 2400, tail: 0.5 }), { size: 0.6, wet: 0.14 })];
}, {});
def('weapon.flame_loop', 1, (v, r) => {
    const n = secs(2.6), bodyN = svf(pink(n, r), 'bp', 320, 0.3), mid = svf(white(n, r), 'bp', 1100, 0.5), hiss = filter(white(n, r), 'hp', 3000), flick = mk(n);
    let a = 0.5; for (let i = 0; i < n; i++) { if ((i & 31) === 0) a += (r() - a) * 0.45; flick[i] = a; }
    const x = mk(n); for (let i = 0; i < n; i++) x[i] = bodyN[i] * (0.3 + flick[i]) * 3.4 + mid[i] * flick[i] * 0.9 + hiss[i] * 0.05 * flick[i];
    const crackle = mk(n); for (let k = 0; k < 140; k++) addWrap(crackle, noiseBurst(r, 0.014, 0.0025, { bp: 1500 + 3500 * r(), q: 2 }), secs(2.5 * r()), 0.9 * r());
    return [loopify(softClip(sum(n, [x, crackle]), 1.4), 0.6)];
}, { loop: true });
def('weapon.frost_throw', 1, (v, r) => {
    const n = secs(0.9), g = tone(0.6, t => 1500 * Math.pow(2.6, t / 0.6), { harmonics: [[1, 1], [2.01, 0.4], [3.02, 0.1]], env: t => Math.sin(Math.PI * Math.min(1, t / 0.6)) ** 1.5 * 0.4 });
    return [sum(n, [scale(whoosh(r, 0.7, 1200, 5000, 1, 0.45), 0.6), g, add(mk(n), sparkle(r, 0.6, 16, 3000, 9000, 0.12), 0.1, 0.35), thump(0.1, 180, 100, 0.015, 0.03)])];
});
def('weapon.frost_burst', 1, (v, r) => {
    const n = secs(1.5), pings = mk(n);
    for (let k = 0; k < 46; k++) addWrap(pings, modal(2200 + 6800 * r(), [[1, 1, 0.1 + 0.25 * r()], [2.3, 0.3, 0.08]], secs(0.45)), secs(0.4 * Math.pow(r(), 1.5)), 0.12 + 0.3 * r());
    const crunch = mk(n); for (let k = 0; k < 28; k++) addWrap(crunch, noiseBurst(r, 0.03, 0.006, { hp: 2600 }), secs(0.02 + 0.35 * r()), 0.5 * r());
    return [monoReverb(sum(n, [pings, crunch, scale(noiseBurst(r, 0.12, 0.02, { hp: 1800 }), 0.8), thump(0.2, 150, 60, 0.03, 0.06), scale(chime(1568, 1.2, 0.6), 0.4)]), { size: 0.5, wet: 0.12 })];
});
def('weapon.horn_blast', 2, (v, r) => [airHorn(r, v === 0 ? [[415, 1], [523, 0.9], [622, 0.7]] : [[370, 1], [466, 0.85], [554, 0.7]], 1.15)]);
def('weapon.hole_loop', 1, (v, r) => {
    const n = secs(2.6), L = n - secs(0.6), lf = loopHz(L, 1), x = mk(n);
    const swirl = svf(white(n, r), 'bp', i => 650 + 500 * Math.sin(TAU * lf * i / SR), 9);
    const sub = tone(2.6, t => 46 + 2 * Math.sin(TAU * lf * t), { harmonics: [[1, 1], [2.004, 0.5], [3, 0.2]], env: () => 0.5 });
    for (let i = 0; i < n; i++) x[i] = swirl[i] * 1.3 * (0.6 + 0.4 * Math.sin(TAU * lf * i / SR + 1.2)) + sub[i];
    [0.1, 1.3].forEach(at => add(x, tone(1.1, t => 900 * Math.exp(-t / 0.55) + 90, { harmonics: [[1, 1], [2, 0.3]], env: t => Math.sin(Math.PI * Math.min(1, t / 1.1)) * 0.2 }), at));
    return [loopify(x, 0.6)];
}, { loop: true });
def('weapon.disco_on', 1, (v, r) => {
    const n = secs(1.5), o = mk(n), chord = [165, 196, 247, 294, 392];
    [0, 0.19].forEach((at, k) => {
        const len = 0.22, st = mk(secs(len));
        chord.forEach(f => add(st, tone(len, () => f * 2, { harmonics: SAW_H.slice(0, 9) }), 0, 0.3));
        const wah = svf(st, 'lp', i => (k ? 2600 : 450) * Math.pow(k ? 0.25 : 5.5, i / st.length), 3);
        for (let i = 0; i < wah.length; i++) wah[i] *= Math.min(1, i / 200) * Math.exp(-i / (0.09 * SR));
        add(o, wah, at, 0.9);
    });
    add(o, noiseBurst(r, 0.2, 0.05, { hp: 7000 }), 0.38, 0.4);
    add(o, sparkle(r, 1.0, 22, 2500, 9500, 0.2), 0.4, 0.4);
    return [monoReverb(softClip(o, 1.3), { size: 0.45, wet: 0.12 })];
});
def('weapon.glove', 2, (v, r) => {
    const n = secs(1.0), f0 = 190 + v * 25;
    const boing = tone(0.95, t => f0 * (1 + 1.6 * Math.exp(-t / 0.28) * Math.sin(TAU * (9 - 5 * t) * t + 0.9) * 0.55 + 0.9 * Math.exp(-t / 0.07)), { harmonics: [[1, 1], [2, 0.45], [3, 0.22], [4, 0.1]], env: t => Math.min(1, t / 0.004) * Math.exp(-t / 0.28) });
    return [softClip(sum(n, [boing, scale(noiseBurst(r, 0.05, 0.008, { bp: 1200, q: 0.8 }), 0.7), thump(0.12, 200, 90, 0.015, 0.03, 1.2)]), 1.3)];
});
def('weapon.harpoon_fire', 1, (v, r) => {
    const n = secs(0.9), string = pluck(98, secs(0.8), r, { decay: 0.9985, bright: 0.8, pick: 0.2 });
    return [sum(n, [scale(string, 0.9), scale(noiseBurst(r, 0.07, 0.01, { hp: 2500 }), 0.9), scale(tone(0.4, t => 2600 - 1500 * t / 0.4, { env: t => Math.exp(-t / 0.2) * 0.08 }), 1), thump(0.12, 150, 80, 0.02, 0.04)])];
});
def('weapon.harpoon_clank', 2, (v, r) => {
    const n = secs(0.9), o = mk(n);
    add(o, modal(440 + v * 90, [[1, 1, 0.4], [2.3, 0.7, 0.3], [3.9, 0.5, 0.25], [6.1, 0.3, 0.15], [9.2, 0.15, 0.1]], secs(0.7)), 0, 0.9);
    add(o, noiseBurst(r, 0.04, 0.006, { hp: 2500 }), 0, 0.8);
    for (let k = 0; k < 10; k++) add(o, modal(1600 + 2400 * r(), [[1, 1, 0.07], [2.6, 0.4, 0.04]], secs(0.12)), 0.12 + 0.5 * r(), 0.3 * r());
    return [o];
});
def('weapon.cow_launch', 1, (v, r) => {
    const n = secs(1.9), whistle = tone(1.1, t => 380 * Math.pow(7, Math.pow(t / 1.1, 0.85)) * (1 + 0.012 * Math.sin(TAU * 7 * t) + 0.03 * Math.sin(TAU * 2.2 * t)), { harmonics: [[1, 1], [2, 0.08]], env: t => Math.min(1, t / 0.03) * (t > 0.95 ? Math.max(0, (1.1 - t) / 0.15) : 1) * 0.5 });
    const breath = scale(svf(white(secs(1.1), r), 'bp', i => 380 * Math.pow(7, Math.pow(i / SR / 1.1, 0.85)) * 1.5, 2), 0.14);
    const o = sum(n, [scale(thump(0.3, 120, 45, 0.04, 0.08, 1.2), 0.5), scale(noiseBurst(r, 0.1, 0.02, { lp: 2600 }), 0.4)]);
    add(o, whistle, 0.05, 1.5); add(o, breath, 0.05, 1);
    add(o, mooVoice(r, { len: 0.55, lo: 160, hi: 230, end: 140, shift: 1.1 }), 0.7, 0.3);
    return [monoReverb(o, { size: 0.4, wet: 0.08 })];
});
def('weapon.cow_land', 1, (v, r) => {
    const n = secs(1.5), o = sum(n, [thump(1.1, 80, 30, 0.07, 0.24, 1.6), scale(noiseBurst(r, 0.6, 0.12, { kind: 'pink', lp: 900 }), 0.9), scale(noiseBurst(r, 0.1, 0.02, { hp: 900 }), 0.7), scale(tone(0.4, t => 400 * Math.exp(-t / 0.15) + 90, { env: t => Math.exp(-t / 0.12) * 0.4 }), 1)]);
    add(o, mooVoice(r, { len: 0.5, lo: 180, hi: 250, end: 170, shift: 1.12 }), 0.38, 0.5);
    return [monoReverb(softClip(o, 1.3), { size: 0.4, wet: 0.1 })];
});
def('weapon.cloak_on', 1, (v, r) => {
    const n = secs(1.0), x = tone(0.8, t => 2100 * Math.exp(-t / 0.35) + 350, { harmonics: [[1, 1], [2.01, 0.3], [3.02, 0.1]], env: t => Math.sin(Math.PI * Math.min(1, t / 0.8)) ** 0.7 * (0.7 + 0.3 * Math.sin(TAU * 13 * t)) * 0.4 });
    return [monoReverb(sum(n, [x, scale(whoosh(r, 0.8, 4200, 600, 1.4, 0.3), 0.5), add(mk(n), sparkle(r, 0.6, 12, 2500, 8000, 0.15), 0.1, 0.3)]), { size: 0.55, wet: 0.14 })];
});
def('weapon.overdrive_on', 1, (v, r) => {
    const n = secs(1.3), rev = tone(1.1, t => 75 * Math.pow(6, Math.pow(t / 1.1, 0.8)), { harmonics: SAW_H.slice(0, 10), env: t => Math.min(1, t / 0.05) * (t > 0.95 ? Math.max(0, (1.1 - t) / 0.15) : 1) });
    const whistleT = tone(1.1, t => 1300 * Math.pow(3, t / 1.1), { env: t => Math.min(1, t / 0.4) * 0.12 });
    const fire = svf(white(n, r), 'bp', i => 400 + 1800 * Math.min(1, i / SR), 0.5);
    for (let i = 0; i < n; i++) fire[i] *= Math.min(1, i / 4000) * Math.exp(-Math.max(0, i / SR - 0.5) / 0.35) * 1.6;
    return [softClip(sum(n, [scale(rev, 0.5), whistleT, fire, thump(0.25, 140, 60, 0.03, 0.08)]), 1.7)];
});

export const WEAPON_RECIPES = R;
export { airHorn };
