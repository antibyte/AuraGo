// Shared building blocks for the one-shot sound recipes: thumps, noise bursts, whooshes, chimes, voices, crowds.
import {
    SR, TAU, mk, secs, rng, filter, svf, Osc, add, addWrap, expDecay, envelope, mul, scale, sum, peak, white, pink, brown, formantVoice, clamp,
    softClip, monoReverb, echo, fadeEdges, modal, pluck, lerp, loopify
} from './dsp.mjs';

export * from './dsp.mjs';

export const VOWELS = {
    a: [[800, 9, 1], [1150, 12, 0.6], [2900, 14, 0.25]], o: [[450, 8, 1], [800, 10, 0.6], [2830, 14, 0.15]],
    u: [[325, 8, 1], [700, 10, 0.5], [2700, 14, 0.1]], e: [[400, 9, 1], [2000, 12, 0.5], [2800, 14, 0.3]],
    i: [[270, 8, 1], [2300, 12, 0.4], [3000, 14, 0.3]], aw: [[570, 8, 1], [840, 10, 0.7], [2410, 14, 0.2]],
    ae: [[660, 8, 1], [1700, 12, 0.6], [2400, 14, 0.25]]
};
export const mixVowel = (a, b, t, shift = 1) => VOWELS[a].map((fa, k) => [lerp(fa[0], VOWELS[b][k][0], t) * shift, lerp(fa[1], VOWELS[b][k][1], t), lerp(fa[2], VOWELS[b][k][2], t)]);

// Sine with exponential pitch fall and amplitude decay (kicks, thuds, booms).
export function thump(dur, f0, f1, tauPitch, tauAmp, drive = 1) {
    const n = secs(dur), o = mk(n); let ph = 0;
    for (let i = 0; i < n; i++) {
        const t = i / SR, f = f1 + (f0 - f1) * Math.exp(-t / tauPitch);
        ph += TAU * f / SR; o[i] = Math.sin(ph) * Math.exp(-t / tauAmp);
    }
    return drive === 1 ? o : softClip(o, drive);
}
// Decaying noise burst, optionally band-limited.
export function noiseBurst(r, dur, tau, { lp, hp, bp, q = 1, kind = 'white', attack = 0.0008 } = {}) {
    const n = secs(dur); let x = kind === 'pink' ? pink(n, r) : kind === 'brown' ? brown(n, r) : white(n, r);
    if (bp) x = filter(x, 'bp', bp, q); if (lp) x = filter(x, 'lp', lp); if (hp) x = filter(x, 'hp', hp);
    const e = expDecay(n, tau), a = Math.round(attack * SR);
    for (let i = 0; i < n; i++) x[i] *= e[i] * (i < a ? i / a : 1);
    return x;
}
// Band-passed noise whose centre sweeps f0 -> f1 with a bell-shaped level (whoosh, wind-ups, passes).
export function whoosh(r, dur, f0, f1, q = 1.2, peakAt = 0.5, { curve = 1 } = {}) {
    const n = secs(dur), x = svf(white(n, r), 'bp', i => f0 * Math.pow(f1 / f0, Math.pow(i / n, curve)), q), o = mk(n);
    for (let i = 0; i < n; i++) {
        const t = i / n, w = t < peakAt ? Math.sin(Math.PI / 2 * t / peakAt) : Math.cos(Math.PI / 2 * (t - peakAt) / (1 - peakAt));
        o[i] = x[i] * w * w * 2.2;
    }
    return o;
}
// Oscillator with a frequency function of time (seconds). harmonics: [[mult, amp], ...] for additive waveforms.
export function tone(dur, freqFn, { harmonics = [[1, 1]], env = null, vib = 0, vibHz = 5.5, phase = 0 } = {}) {
    const n = secs(dur), o = mk(n); let ph = phase;
    for (let i = 0; i < n; i++) {
        const t = i / SR, f = freqFn(t) * (1 + vib * Math.sin(TAU * vibHz * t));
        ph += TAU * f / SR; let v = 0;
        for (const [m, a] of harmonics) if (f * m < SR * 0.45) v += a * Math.sin(ph * m);
        o[i] = v * (env ? env(t) : 1);
    }
    return o;
}
export const SAW_H = Array.from({ length: 14 }, (_, k) => [k + 1, 1 / (k + 1)]);
export const SQUARE_H = Array.from({ length: 8 }, (_, k) => [2 * k + 1, 1 / (2 * k + 1)]);
export const adsr = (a, d, s, rel, hold) => t => t < a ? t / a : t < a + d ? 1 - (1 - s) * (t - a) / d : t < hold ? s : t < hold + rel ? s * (1 - (t - hold) / rel) : 0;
export const pulseEnv = (a, tau) => t => (t < a ? t / a : Math.exp(-(t - a) / tau));

// Bell / glass / marimba like partials.
export const BELL = [[1, 1, 1], [2.76, 0.6, 0.55], [5.4, 0.35, 0.35], [8.93, 0.2, 0.25], [13.3, 0.1, 0.15]];
export function chime(f, dur, tau = 0.5, partials = BELL) {
    const n = secs(dur), o = modal(f, partials.map(([m, a, d]) => [m, a, d * tau * 2]), n);
    return fadeEdges(o, 0.001, 0.01);
}
// Short random sparkles (tiny high chimes) over a duration.
export function sparkle(r, dur, count, fLo, fHi, tau = 0.12) {
    const n = secs(dur), o = mk(n);
    for (let k = 0; k < count; k++) {
        const f = fLo * Math.pow(fHi / fLo, r()), g = 0.25 + 0.75 * r(), at = Math.floor(r() * n * 0.9);
        const c = modal(f, [[1, 1, tau], [2.4, 0.3, tau * 0.6]], secs(tau * 1.6));
        addWrap(o, c, at, g);
    }
    return o;
}
export function clicks(r, dur, count, f, q = 3, tau = 0.004, spread = 1) {
    const n = secs(dur), o = mk(n);
    for (let k = 0; k < count; k++) {
        const c = noiseBurst(r, tau * 8, tau, { bp: f * (0.7 + 0.6 * r()), q });
        addWrap(o, c, Math.floor(Math.pow(r(), spread) * (n - c.length)), 0.4 + 0.6 * r());
    }
    return o;
}
// Convert a per-voice mono render into a crowd (many formant voices).
export function crowd(r, dur, voices, { f0Lo = 110, f0Hi = 260, vowel = ['a', 'a'], glide = 1, onset = 0.6, length = [0.8, 1.6], breath = 0.12, claps = 0 } = {}) {
    const n = secs(dur), out = mk(n);
    for (let v = 0; v < voices; v++) {
        const at = r() * onset, len = Math.min(dur - at, length[0] + (length[1] - length[0]) * r()), m = secs(len);
        const base = f0Lo + (f0Hi - f0Lo) * r(), shift = 0.88 + 0.28 * r(), wob = 3 + 4 * r(), g = 0.5 + 0.5 * r(), ph = r() * 6;
        const x = formantVoice(m, i => base * (1 + 0.08 * Math.sin(TAU * wob * i / SR + ph)) * (1 + (glide - 1) * (i / m)), i => mixVowel(vowel[0], vowel[1], i / m, shift), r,
            { jitter: 0.012, shimmer: 0.1, breath, amp: i => Math.sin(Math.PI * Math.min(1, i / m)) ** 0.7 });
        add(out, x, at, g * 0.1);
    }
    for (let k = 0; k < claps; k++) {
        const c = noiseBurst(r, 0.05, 0.008, { bp: 1500 + 1500 * r(), q: 0.9 });
        add(out, c, r() * (dur - 0.1), 0.12 * r());
    }
    return out;
}
