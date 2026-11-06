// World sounds: impacts, scrapes, tyres, explosions, props, crowd, animals and biome ambience (own synthesis).
import {
    SR, TAU, mk, secs, filter, svf, add, addWrap, expDecay, mul, scale, sum, peak, white, pink, brown, formantVoice, clamp, softClip, monoReverb,
    fadeEdges, modal, loopify, loopHz, thump, noiseBurst, whoosh, tone, sparkle, clicks, crowd, mixVowel, SAW_H, SQUARE_H, echo, lerp
} from './kit.mjs';

const R = [];
const def = (group, variants, render, extra = {}) => R.push({ group, kind: 'sfx', variants, channels: 1, render, ...extra });
const slowRand = (r, hz, count = 5) => { const p = Array.from({ length: count }, () => [hz * (0.5 + r()), r() * TAU, 0.4 + 0.6 * r()]); const tot = p.reduce((a, b) => a + b[2], 0); return t => p.reduce((a, [f, ph, g]) => a + g * Math.sin(TAU * f * t + ph), 0) / tot; };

// ---- impacts ----
def('impact.light', 3, (v, r) => {
    const n = secs(0.3), o = sum(n, [scale(noiseBurst(r, 0.12, 0.022, { lp: 1800 + v * 300 }), 1), scale(modal(300 + v * 70, [[1, 1, 0.05], [2.1, 0.5, 0.035], [3.3, 0.3, 0.025]], n), 0.8), scale(thump(0.11, 155 + v * 15, 68, 0.015, 0.03), 0.45)]);
    for (let k = 0; k < 4; k++) add(o, noiseBurst(r, 0.02, 0.005, { bp: 900 + 600 * r(), q: 3 }), 0.025 + 0.06 * k + 0.02 * r(), 0.3 / (k + 1));
    return [o];
});
def('impact.heavy', 3, (v, r) => {
    const n = secs(1.1), raw = noiseBurst(r, 0.6, 0.11, { lp: 3600 }), crunch = mk(raw.length);
    let amp = 1; for (let i = 0; i < raw.length; i++) { if (i % 260 === 0) amp = 0.25 + 0.75 * r(); crunch[i] = raw[i] * amp; }
    const parts = [scale(crunch, 1.3), scale(thump(0.5, 95, 45, 0.05, 0.12, 1.2), 0.45), scale(noiseBurst(r, 0.05, 0.008, { hp: 900 }), 0.8)];
    parts.push(scale(modal(180 + v * 30, [[1, 0.5, 0.25], [1.6, 0.4, 0.2], [2.7, 0.3, 0.18], [4.4, 0.2, 0.12], [6.8, 0.12, 0.1]], n), 0.35));
    const debris = mk(n);
    for (let k = 0; k < 46; k++) { const at = 0.04 + 0.8 * Math.pow(r(), 1.7); addWrap(debris, r() < 0.6 ? noiseBurst(r, 0.025, 0.004, { bp: 1200 + 3800 * r(), q: 3 }) : modal(1500 + 2500 * r(), [[1, 1, 0.05], [2.7, 0.3, 0.03]], secs(0.12)), secs(at), 0.5 * r() * (1 - at)); }
    parts.push(debris);
    return [filter(softClip(sum(n, parts), 1.3), 'lp', 8000)];
});
def('impact.metal', 3, (v, r) => {
    const n = secs(1.1), base = 330 + v * 85, ratios = [1, 2.32, 4.25, 6.63, 9.38, 12.5];
    const plate = modal(base, ratios.map((m, k) => [m, 1 / (1 + k * 0.45), 0.5 - k * 0.055]), n, { jitter: 0.015, r });
    const hit = sum(n, [plate, scale(noiseBurst(r, 0.05, 0.006, { hp: 2200 }), 0.8), scale(thump(0.12, base * 0.7, base * 0.4, 0.02, 0.03), 0.6)]);
    add(hit, scale(modal(base * 1.13, ratios.map((m, k) => [m, 1 / (1 + k * 0.5), 0.35 - k * 0.04]), secs(0.7), { r }), 0.5), 0.028);
    return [hit];
});
def('scrape.metal', 2, (v, r) => {
    const n = secs(2.2), noise = white(n, r), grit = mk(n); let amp = 0, tgt = 0;
    const rate = 85 + v * 40, step = Math.round(SR / rate);
    for (let i = 0; i < n; i++) { if (i % step === 0) tgt = 0.25 + 0.75 * r(); amp += (tgt - amp) * 0.15; grit[i] = noise[i] * amp; }
    const lfo = slowRand(r, 0.5), banded = sum(n, [1, 2.1, 3.6].map((m, k) => svf(grit, 'bp', i => (1250 + v * 150) * m * (1 + 0.12 * lfo(i / SR)), 7 + k * 2)));
    return [loopify(softClip(banded, 1.0), 0.4)];
}, { loop: true });
def('tyre.squeal', 2, (v, r) => {
    const n = secs(2.4), L = n - secs(0.4), base = 880 + v * 230, f1 = loopHz(L, 1), f2 = loopHz(L, 7), f3 = loopHz(L, 38);
    const tn = mk(L); let ph = 0, ph2 = 0;
    for (let i = 0; i < L; i++) {
        const t = i / SR, f = base * (1 + 0.07 * Math.sin(TAU * f1 * t) + 0.018 * Math.sin(TAU * f2 * t + 1));
        ph += TAU * f / SR; ph2 += TAU * f * 1.0045 / SR;
        let w = 0; for (let h = 1; h <= 6; h++) w += Math.sin(ph * h) / h + 0.6 * Math.sin(ph2 * h + 0.7) / h;
        tn[i] = w * (0.7 + 0.3 * Math.sin(TAU * f3 * t)) * (0.85 + 0.15 * Math.sin(TAU * loopHz(L, 2.5) * t));
    }
    const nz = loopify(svf(white(n, r), 'bp', base * 1.7, 5), 0.4), fric = loopify(filter(white(n, r), 'hp', 2500), 0.4);
    const mix = sum(L, [scale(tn, 0.5), scale(nz, 1.4), scale(fric, 0.28)]);
    return [filter(softClip(mix, 1.1), 'peak', 2400, 1.2, 4)];
}, { loop: true });

// ---- explosions ----
def('explosion.small', 2, (v, r) => {
    const n = secs(1.6), nz = white(n, r);
    const crack = svf(nz, 'lp', i => 650 + 9000 * Math.exp(-i / (0.045 * SR)), 0.9); for (let i = 0; i < n; i++) crack[i] *= Math.exp(-i / (0.1 * SR));
    const rumble = noiseBurst(r, 1.5, 0.45, { kind: 'brown', lp: 300 }); for (let i = 0; i < rumble.length; i++) rumble[i] *= 3;
    const debris = mk(n); for (let k = 0; k < 24; k++) addWrap(debris, noiseBurst(r, 0.02, 0.004, { bp: 1800 + 3500 * r(), q: 2 }), secs(0.1 + 0.8 * r() * r()), 0.4 * r());
    const x = sum(n, [scale(crack, 1.1), thump(1.4, 120 - v * 15, 38, 0.1, 0.32, 1.5), rumble, debris]);
    return [monoReverb(softClip(x, 1.7), { size: 0.45, wet: 0.12, tail: 0.3 })];
});
def('explosion.big', 2, (v, r) => {
    const n = secs(3.2), nz = white(n, r);
    const crack = svf(nz, 'lp', i => 500 + 11000 * Math.exp(-i / (0.07 * SR)), 0.9); for (let i = 0; i < n; i++) crack[i] *= Math.exp(-i / (0.16 * SR));
    const roar = svf(white(n, r), 'lp', i => 2400 * Math.exp(-i / (0.5 * SR)) + 180, 0.8); for (let i = 0; i < n; i++) roar[i] *= Math.exp(-i / (0.45 * SR));
    const rumble = noiseBurst(r, 3, 1.1, { kind: 'brown', lp: 220 }); for (let i = 0; i < rumble.length; i++) rumble[i] *= 4.5;
    const wob = loopHz(n, 5.5); for (let i = 0; i < n && i < rumble.length; i++) rumble[i] *= 0.75 + 0.25 * Math.sin(TAU * wob * i / SR);
    const debris = mk(n);
    for (let k = 0; k < 70; k++) addWrap(debris, noiseBurst(r, 0.04, 0.006, { bp: 1200 + 4500 * r(), q: 2.5 }), secs(0.25 + 2.3 * r() * r()), 0.5 * r());
    for (let k = 0; k < 9; k++) addWrap(debris, modal(900 + 1900 * r(), [[1, 1, 0.12], [2.7, 0.4, 0.07]], secs(0.2)), secs(0.4 + 2 * r()), 0.12 * r());
    let x = sum(n, [scale(crack, 1.2), scale(roar, 0.9), thump(2.6, 80 - v * 10, 27, 0.18, 0.7, 1.5), rumble, debris]);
    x = softClip(x, 1.8);
    const refl = mk(n); add(refl, filter(x, 'lp', 700), 0.19, 0.45); add(refl, filter(x, 'lp', 450), 0.43, 0.28); add(refl, filter(x, 'lp', 300), 0.78, 0.18);
    return [monoReverb(sum(n, [x, refl]), { size: 0.78, wet: 0.2, tail: 0.5 })];
});

// ---- crates and props ----
def('crate.break', 3, (v, r) => {
    const n = secs(0.7), o = mk(n);
    for (let k = 0; k < 44; k++) addWrap(o, noiseBurst(r, 0.03, 0.005, { bp: 700 + 3600 * r(), q: 3 }), secs(0.03 + 0.42 * Math.pow(r(), 1.3)), 0.2 + 0.6 * r());
    for (let k = 0; k < 8; k++) addWrap(o, noiseBurst(r, 0.04, 0.008, { bp: 1300 + 700 * r(), q: 2 }), secs(0.01 + 0.2 * r()), 0.5 + 0.5 * r());
    for (let k = 0; k < 6; k++) addWrap(o, modal(250 + 500 * r(), [[1, 1, 0.012], [2.4, 0.4, 0.008]], secs(0.08)), secs(0.2 * r()), 0.3);
    return [sum(n, [o, scale(noiseBurst(r, 0.04, 0.006, { hp: 2500 }), 0.5)])];
});
def('prop.hay', 2, (v, r) => {
    const n = secs(0.7), rustle = mk(n); for (let k = 0; k < 40; k++) addWrap(rustle, noiseBurst(r, 0.03, 0.01, { bp: 3200 + 2000 * r(), q: 1 }), secs(0.02 + 0.45 * r() * r()), 0.2 * r());
    return [sum(n, [noiseBurst(r, 0.5, 0.07, { kind: 'pink', lp: 900 + v * 150 }), scale(rustle, 1), scale(thump(0.2, 130, 70, 0.04, 0.05), 0.5)])];
});
def('prop.cone', 3, (v, r) => {
    const n = secs(0.8), o = mk(n), f = 400 + v * 45;
    [[0, 1, 1], [0.15, 0.55, 1.07], [0.25, 0.32, 1.12], [0.31, 0.18, 1.16]].forEach(([at, g, p]) => add(o, sum(secs(0.3), [modal(f * p, [[1, 1, 0.1], [1.9, 0.5, 0.07], [3.1, 0.25, 0.05]], secs(0.3)), scale(noiseBurst(r, 0.04, 0.008, { bp: 1500, q: 1 }), 0.6), thump(0.12, f * 0.6, f * 0.3, 0.01, 0.025)]), at, g));
    return [o];
});
def('prop.barrel', 2, (v, r) => {
    const n = secs(0.9);
    return [sum(n, [thump(0.5, 150 - v * 15, 78, 0.03, 0.16, 1.3), scale(modal(380 + v * 50, [[1, 1, 0.3], [2.1, 0.5, 0.22], [3.3, 0.3, 0.15]], n), 0.5), scale(noiseBurst(r, 0.1, 0.015, { bp: 1100, q: 1 }), 0.7)])];
});
def('prop.tyres', 2, (v, r) => {
    const n = secs(0.55), sq = tone(0.3, t => 750 + 600 * t / 0.3 + v * 90, { harmonics: SAW_H.slice(0, 5), env: t => Math.sin(Math.PI * Math.min(1, t / 0.3)) * 0.25 });
    return [sum(n, [thump(0.4, 95, 55, 0.04, 0.12, 1.2), scale(noiseBurst(r, 0.2, 0.04, { lp: 900 }), 0.9), add(mk(n), filter(sq, 'bp', 1100, 1.5), 0.08, 0.7)])];
});

// ---- crowd ----
def('crowd.cheer', 2, (v, r) => {
    const dur = 2.8, n = secs(dur), voices = crowd(r, dur, 40, { f0Lo: 150, f0Hi: 420, vowel: ['u', 'a'], glide: 1.12, onset: 0.9, length: [0.5, 1.3], breath: 0.06, claps: 36 });
    const p = Math.max(1e-9, peak(voices)), roar = filter(white(n, r), 'bp', 700, 0.5), x = mk(n);
    for (let i = 0; i < n; i++) { const t = i / SR; x[i] = (voices[i] / p * 0.9 + roar[i] * 0.07 * Math.min(1, t / 0.5)) * Math.min(1, t / 0.12) * Math.min(1, (dur - t) / 0.8); }
    return [monoReverb(x, { size: 0.5, wet: 0.12 })];
});
def('crowd.aww', 2, (v, r) => {
    const dur = 2.3, n = secs(dur), voices = crowd(r, dur, 24, { f0Lo: 190, f0Hi: 340, vowel: ['o', 'aw'], glide: 0.74, onset: 0.3, length: [1.0, 1.6], breath: 0.15, claps: 0 }), p = Math.max(1e-9, peak(voices));
    for (let i = 0; i < n; i++) { const t = i / SR; voices[i] = voices[i] / p * 0.9 * Math.min(1, t / 0.15) * Math.min(1, (dur - t) / 0.6); }
    return [monoReverb(voices, { size: 0.45, wet: 0.1 })];
});
// ---- animals ----
export function mooVoice(r, { len = 1.4, lo = 95, hi = 150, end = 80, shift = 1, rise = 0.28 }) {
    const n = secs(len);
    const f0 = i => { const t = i / SR; const up = t < rise ? lo + (hi - lo) * Math.pow(t / rise, 0.8) : hi + (end - hi) * Math.pow((t - rise) / (len - rise), 1.3); return up * (1 + 0.012 * Math.sin(TAU * 5.4 * t)); };
    const x = formantVoice(n, f0, i => { const t = i / n; return mixVowel('o', 'u', Math.min(1, t * 1.4), shift).map(([f, q, g]) => [f * (t < 0.1 ? 0.7 + 3 * t : 1), q, g]); }, r,
        { jitter: 0.014, shimmer: 0.06, breath: 0.07, tilt: 1800, amp: i => { const t = i / n; return Math.min(1, t / 0.07) * Math.pow(Math.min(1, (1 - t) / 0.25), 0.8) * (0.85 + 0.15 * Math.sin(TAU * 5 * t * len)); } });
    return x;
}
def('animal.cow', 2, (v, r) => [monoReverb(mooVoice(r, { len: 1.5 + v * 0.2, lo: 80 + v * 14, hi: 138 + v * 10, end: 70 + v * 9, shift: 1 - v * 0.06 }), { size: 0.3, wet: 0.08 })]);
def('animal.sheep', 2, (v, r) => {
    const len = 0.95 + v * 0.1, n = secs(len), f0b = 400 + v * 70, trem = 28 + v * 4;
    const x = formantVoice(n, i => { const t = i / n; return f0b * (1 + 0.12 * (1 - t) - 0.1 * t) * (1 + 0.04 * Math.sin(TAU * trem * i / SR)); },
        i => [[900, 5, 1], [1500 - 150 * (i / n), 6, 0.8], [2600, 8, 0.4]], r,
        { jitter: 0.09, shimmer: 0.25, breath: 0.4, tilt: 3400, pulse: 'saw', amp: i => { const t = i / n; return Math.min(1, t / 0.05) * Math.min(1, (1 - t) / 0.2) * (0.6 + 0.4 * Math.abs(Math.sin(Math.PI * trem * i / SR))); } });
    return [x];
});
def('animal.penguin', 2, (v, r) => {
    const out = mk(secs(1.5)), calls = [[0, 0.32, 230], [0.38, 0.3, 245], [0.75, 0.62, 215]];
    for (const [at, len, f] of calls) {
        const n = secs(len), x = formantVoice(n, i => (f + v * 25) * (1 + 0.28 * (i / n) - (at > 0.7 ? 0.35 * (i / n) ** 2 : 0)), i => [[650, 7, 1], [1150, 9, 0.8], [2300, 12, 0.5], [3200, 14, 0.2]], r,
            { jitter: 0.04, shimmer: 0.15, breath: 0.12, tilt: 4200, pulse: 'square', amp: i => { const t = i / n; return Math.min(1, t / 0.04) * Math.min(1, (1 - t) / 0.2) * (0.6 + 0.4 * Math.sin(TAU * 54 * i / SR)); } });
        add(out, x, at, 1);
    }
    return [monoReverb(out, { size: 0.35, wet: 0.1 })];
});
def('animal.llama', 2, (v, r) => {
    const len = 1.25, n = secs(len), x = formantVoice(n, i => { const t = i / n; return (170 + v * 24) * (0.85 + 0.45 * Math.sin(Math.PI * Math.min(1, t * 1.15)) ** 0.8 + 0.12 * t) * (1 + 0.02 * Math.sin(TAU * 6 * t * len)); },
        i => { const t = i / n; return mixVowel('o', 'ae', t, 1).map(([f, q, g], k) => [k === 0 ? f * 0.85 : f, q, g]); }, r,
        { jitter: 0.03, shimmer: 0.12, breath: 0.18, tilt: 2400, amp: i => { const t = i / n; return Math.min(1, t / 0.12) * Math.min(1, (1 - t) / 0.3); } });
    const hum = tone(len, t => 170 + v * 24, { harmonics: [[1, 0.5], [2, 0.3]], env: t => Math.min(1, t / 0.2) * Math.min(1, (len - t) / 0.3) * 0.15 });
    return [sum(n, [x, hum])];
});
def('animal.crab', 2, (v, r) => {
    const n = secs(1.0), o = mk(n);
    let t = 0.02; for (let g = 0; g < 7; g++) {
        const cnt = 2 + Math.floor(r() * 3);
        for (let k = 0; k < cnt; k++) { const f = 2400 + 3200 * r(); addWrap(o, sum(secs(0.05), [noiseBurst(r, 0.02, 0.0018, { bp: f, q: 5 }), modal(900 + 900 * r(), [[1, 0.8, 0.014], [2.3, 0.3, 0.008]], secs(0.04))]), secs(t), 0.5 + 0.5 * r()); t += 0.03 + 0.02 * r(); }
        t += 0.06 + 0.07 * r(); if (t > 0.9) break;
    }
    return [o];
});
// ---- ambience (seamless loops) ----
const finishLoop = (x, xf) => [loopify(x, xf)];
def('amb.wind', 1, (v, r) => {
    const n = secs(7), lfo = slowRand(r, 0.35), gust = slowRand(r, 0.28), p = pink(n, r), x = svf(p, 'bp', i => 1000 + 450 * lfo(i / SR), 0.8);
    const whistle = svf(white(n, r), 'bp', i => 1750 + 320 * lfo(i / SR * 1.9), 26), howl = svf(white(n, r), 'bp', i => 780 + 120 * lfo(i / SR * 0.7 + 4), 30);
    for (let i = 0; i < n; i++) { const g = 0.35 + 0.65 * Math.max(0, 0.5 + 0.5 * gust(i / SR)); x[i] = x[i] * g * 1.5 + (whistle[i] * 0.9 + howl[i] * 0.7) * g * g; }
    return finishLoop(x, 1);
}, { loop: true });
def('amb.sea', 1, (v, r) => {
    const n = secs(9.5), body = svf(pink(n, r), 'lp', 1400, 0.7), foam = filter(white(n, r), 'hp', 3000), sub = svf(brown(n, r), 'lp', 140, 0.7), x = mk(n), period = 4.0;
    for (let i = 0; i < n; i++) {
        const t = i / SR, ph = (t % period) / period, swell = Math.pow(Math.sin(Math.PI * Math.min(1, ph * 1.15)), 2.2), crest = Math.pow(Math.max(0, Math.sin(Math.PI * Math.min(1, Math.max(0, ph * 1.15 - 0.15)))), 4);
        x[i] = body[i] * (0.12 + 0.88 * swell) * 1.8 + foam[i] * crest * 0.35 + sub[i] * swell * 3;
    }
    return finishLoop(x, 1.5);
}, { loop: true });
export function birdCall(r, kind) {
    if (kind === 0) { const o = mk(secs(0.4)); [0, 0.14].forEach(at => add(o, tone(0.1, t => 3300 + 1100 * Math.sin(Math.PI * t / 0.1) ** 0.6, { harmonics: [[1, 1], [2, 0.18]], env: t => Math.sin(Math.PI * Math.min(1, t / 0.1)) * 0.7 }), at)); return o; }
    if (kind === 1) return tone(0.55, t => 2700 + 80 * Math.sin(TAU * 38 * t), { harmonics: [[1, 1], [2, 0.12]], env: t => Math.sin(Math.PI * Math.min(1, t / 0.55)) * (0.5 + 0.5 * Math.sin(TAU * 17 * t)) ** 2 * 0.6 });
    if (kind === 2) return tone(0.6, t => 1900 + 700 * Math.min(1, t / 0.4) - 150 * Math.max(0, t - 0.4) / 0.2, { harmonics: [[1, 1], [2, 0.3], [3, 0.06]], env: t => Math.sin(Math.PI * Math.min(1, t / 0.6)) * 0.7, vib: 0.01, vibHz: 9 });
    return tone(0.3, t => 4200 - 2300 * (t / 0.3) ** 0.7, { harmonics: [[1, 1], [2, 0.1]], env: t => Math.sin(Math.PI * Math.min(1, t / 0.3)) * 0.6 });
}
def('amb.forest', 1, (v, r) => {
    const n = secs(10), leaves = filter(filter(pink(n, r), 'hp', 900), 'lp', 5200), gust = slowRand(r, 0.15), x = mk(n);
    for (let i = 0; i < n; i++) x[i] = leaves[i] * (0.05 + 0.04 * gust(i / SR)) * 2;
    for (let k = 0; k < 11; k++) add(x, birdCall(r, k % 4), 0.8 + 7.4 * r(), 0.45 + 0.4 * r());
    return [monoReverb(loopify(x, 1.5), { size: 0.5, wet: 0.1 })];
}, { loop: true });
def('amb.machinery', 1, (v, r) => {
    const n = secs(6), x = mk(n);
    for (let i = 0; i < n; i++) { const t = i / SR; x[i] = 0.5 * Math.sin(TAU * 50 * t) + 0.33 * Math.sin(TAU * 100.3 * t) + 0.2 * Math.sin(TAU * 150.1 * t) + 0.1 * Math.sin(TAU * 200 * t); }
    const whirr = svf(white(n, r), 'bp', 1800, 2), thumps = mk(n);
    for (let i = 0; i < n; i++) whirr[i] *= 0.12 + 0.1 * Math.sin(TAU * 11 * i / SR);
    for (let k = 0; k * 0.5 < 5.6; k++) { add(thumps, thump(0.2, 90, 50, 0.025, 0.06, 1.2), k * 0.5, 0.7); add(thumps, noiseBurst(r, 0.03, 0.006, { bp: 2400, q: 2 }), k * 0.5, 0.25); }
    add(thumps, modal(520, [[1, 1, 0.5], [2.6, 0.5, 0.3], [4.1, 0.3, 0.2]], secs(0.6)), 1.7, 0.18); add(thumps, modal(380, [[1, 1, 0.5], [2.4, 0.5, 0.3]], secs(0.6)), 4.2, 0.15);
    return finishLoop(sum(n, [scale(x, 0.5), scale(whirr, 1.6), scale(thumps, 0.18)]), 0.5);
}, { loop: true });
def('amb.city', 1, (v, r) => {
    const n = secs(9), rumble = svf(brown(n, r), 'lp', 240, 0.7), mid = svf(pink(n, r), 'bp', 700, 0.5), lfo = slowRand(r, 0.2), x = mk(n);
    for (let i = 0; i < n; i++) x[i] = rumble[i] * 2 * (0.7 + 0.3 * lfo(i / SR)) + mid[i] * (0.05 + 0.03 * lfo(i / SR + 3));
    [1.2, 3.9, 6.4].forEach((at, k) => add(x, scale(whoosh(r, 1.9, 350 + 120 * k, 1100 - 150 * k, 0.9, 0.5), 1.1), at));
    [[2.6, 410], [7.5, 470]].forEach(([at, f]) => { add(x, tone(0.35, () => f, { harmonics: SAW_H.slice(0, 6), env: t => Math.min(1, t / 0.02) * Math.min(1, (0.35 - t) / 0.05) * 0.22 }), at); add(x, tone(0.35, () => f * 1.26, { harmonics: SAW_H.slice(0, 6), env: t => Math.min(1, t / 0.02) * Math.min(1, (0.35 - t) / 0.05) * 0.08 }), at); });
    const mur = crowd(r, 8, 9, { f0Lo: 100, f0Hi: 220, vowel: ['o', 'e'], onset: 6, length: [0.4, 0.9], breath: 0.3 }); add(x, filter(mur, 'lp', 1800), 0, 1.2);
    return [monoReverb(loopify(x, 1.5), { size: 0.4, wet: 0.08 })];
}, { loop: true });
def('amb.snow', 1, (v, r) => {
    const n = secs(8), hush = svf(pink(n, r), 'lp', 1500, 0.6), air = svf(white(n, r), 'bp', 5600, 0.6), lfo = slowRand(r, 0.1), x = mk(n);
    for (let i = 0; i < n; i++) { const g = 0.5 + 0.5 * lfo(i / SR); x[i] = hush[i] * (0.25 + 0.3 * g) * 1.4 + air[i] * 0.04 * g; }
    for (let k = 0; k < 4; k++) add(x, tone(0.4, t => 640 + 420 * t / 0.4 * (k % 2 ? 1 : -1), { harmonics: SAW_H.slice(0, 5), env: t => Math.sin(Math.PI * Math.min(1, t / 0.4)) ** 2 * 0.05 }), 0.9 + 1.7 * k + r());
    for (let k = 0; k < 22; k++) add(x, noiseBurst(r, 0.02, 0.003, { hp: 3500 }), 0.5 + 6.5 * r(), 0.07 * r());
    return [monoReverb(loopify(x, 1), { size: 0.55, wet: 0.12 })];
}, { loop: true });
def('amb.desert', 1, (v, r) => {
    const n = secs(8), body = svf(pink(n, r), 'bp', 420, 0.5), sand = filter(white(n, r), 'hp', 5200), lfo = slowRand(r, 0.13), x = mk(n);
    for (let i = 0; i < n; i++) { const g = 0.4 + 0.6 * (0.5 + 0.5 * lfo(i / SR)); x[i] = body[i] * g * 2 + sand[i] * 0.05 * g * g; }
    const cic = mk(n); let ph = 0;
    for (let i = 0; i < n; i++) { const t = i / SR, gate = Math.max(0, Math.sin(TAU * 0.62 * t - 0.5)) ** 0.5, am = 0.5 + 0.5 * Math.sin(TAU * 46 * t); ph += TAU * (4700 + 60 * Math.sin(TAU * 3 * t)) / SR; cic[i] = Math.sin(ph) * am * gate * 0.05; }
    add(x, cic, 0, 1);
    return [monoReverb(loopify(x, 1.2), { size: 0.35, wet: 0.06 })];
}, { loop: true });

export const WORLD_RECIPES = R;
