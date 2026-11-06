// Weather-round sounds: thunder, wind gusts, wet-road spray and the cue when the sky changes.
// Own synthesis, seeded, no clock and no Math.random. The runtime keeps a live Web Audio fallback
// for every group, so a partially loaded pack still sounds like weather.
import {
    SR, mk, secs, filter, svf, add, white, brown, softClip, monoReverb,
    noiseBurst, whoosh, sparkle, chime, thump, fadeEdges
} from './kit.mjs';

const R = [];
const def = (group, variants, render) => R.push({ group, kind: 'sfx', variants, channels: 1, render });

// A close crack, then a long rolling body that swells and dies away in the reverb.
def('weather.thunder', 3, (v, r) => {
    const dur = 2.6 + v * 0.4, n = secs(dur + 0.4), o = mk(n);
    add(o, noiseBurst(r, 0.16, 0.035, { hp: 900 + v * 400 }), 0, 0.85);
    /* High-pass first: brown noise drifts in DC, which would waste headroom and click at the seam.
       A little band-passed white noise keeps the rumble audible on small speakers. */
    const body = filter(filter(brown(n, r), 'hp', 28), 'lp', 420 + v * 80);
    const grit = filter(white(n, r), 'bp', 780 + v * 180, 0.6), roll = mk(n);
    for (let i = 0; i < n; i++) {
        const t = i / SR;
        roll[i] = (body[i] + grit[i] * 0.28) * Math.pow(Math.min(1, t / 0.18), 1.5) *
            Math.exp(-Math.max(0, t - 0.2) / (0.75 + v * 0.25)) * 0.95;
    }
    add(o, roll, 0.03, 1);
    add(o, thump(0.9, 62 + v * 8, 34, 0.08, 0.3, 1.1), 0.05, 0.5);
    add(o, whoosh(r, dur * 0.7, 420, 90, 0.8, 0.3), 0.25, 0.35);
    return [monoReverb(fadeEdges(softClip(o, 1.1)), { size: 0.85, wet: 0.28 })];
});

// Air that rises, leans over and falls again: a band that opens with the gust.
def('weather.gust', 3, (v, r) => {
    const dur = 1.5 + v * 0.35, n = secs(dur + 0.15), o = mk(n);
    const air = svf(white(n, r), 'bp', i => 340 + v * 260 + 900 * Math.sin(Math.PI * Math.min(1, i / n)), 0.9);
    for (let i = 0; i < n; i++) o[i] = air[i] * Math.pow(Math.sin(Math.PI * Math.min(1, i / SR / dur)), 1.4) * 0.9;
    add(o, whoosh(r, dur, 1700 + v * 300, 260, 1.1, 0.45), 0, 0.4);
    add(o, noiseBurst(r, dur * 0.8, 0.25, { lp: 620 }), 0.08, 0.3);
    return [fadeEdges(softClip(o, 1.05))];
});

// A tyre through a puddle: short bright hiss, a few droplets and a soft low slap.
// The layers overlap in the first 20 ms, so they get real headroom before softClip: the shipped
// file then reads -1.3 dBFS at its native 44.1 kHz (no clipped samples). A 48 kHz resample can still
// overshoot the transient, so keep the sum below full scale rather than leaning on the limiter.
def('weather.splash', 3, (v, r) => {
    const n = secs(0.55), o = mk(n), hiss = svf(white(n, r), 'hp', 1200 + v * 350, 0.7);
    for (let i = 0; i < n; i++) o[i] = hiss[i] * Math.exp(-(i / SR) / 0.12) * 0.78;
    add(o, noiseBurst(r, 0.22, 0.05, { bp: 1900 + v * 400, q: 1.6 }), 0.01, 0.5);
    for (let k = 0; k < 7; k++) add(o, chime(700 + r() * 900 + v * 120, 0.12, 0.035), 0.02 + r() * 0.2, 0.08);
    add(o, thump(0.12, 180, 90, 0.01, 0.03), 0, 0.22);
    return [fadeEdges(softClip(o, 1.05))];
});

// One airy sweep tells the driver the sky changed: rising into rain, falling into clear.
def('weather.shift', 2, (v, r) => {
    const n = secs(0.9), o = mk(n);
    add(o, whoosh(r, 0.7, v ? 1200 : 700, v ? 280 : 1900, 1.3, 0.5), 0, 0.7);
    add(o, chime(v ? 523 : 392, 0.6, 0.35), 0.06, 0.4);
    add(o, sparkle(r, 0.5, 8, 3200, 8000, 0.1), 0.1, 0.2);
    return [monoReverb(fadeEdges(softClip(o, 1.05)), { size: 0.4, wet: 0.16 })];
});

export const WEATHER_RECIPES = R;
