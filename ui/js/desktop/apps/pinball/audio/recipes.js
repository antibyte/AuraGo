// AuraPinball sound recipes: every mechanical sound is a recorded CC0 take (samples.js) as its body, with a short low thump / noise knock under it.
// Imports cleanly in Node (no AudioContext access at load). Each recipe is run(k, t, rate, d) -> void, where
// k = kit (take / osc / nz / smp helpers), t = start time, rate = pitch factor (~1), d = destination node of the voice.
// RECIPES[name] = { run, priority, dur, wet } : priority decides who the 24-voice limiter drops, dur is the
// tail length in seconds, wet is the reverb send level.

import { TAKES } from './samples.js';

export function makeKit(engine) {
    let seed = 90210;
    const rnd = () => { seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0; return seed / 4294967296; };
    const env = (g, t, peak, a, dur) => { g.gain.setValueAtTime(0.0001, t); g.gain.linearRampToValueAtTime(peak, t + a); g.gain.exponentialRampToValueAtTime(0.0001, t + Math.max(dur, a + .002)); };
    function sweep(param, f0, f1, t, dur) { param.setValueAtTime(f0, t); if (f1 && f1 !== f0) param.exponentialRampToValueAtTime(f1, t + dur); }
    return {
        rnd,
        osc(type, f0, f1, t, dur, peak, dest, o = {}) {
            const c = engine.ctx, n = c.createOscillator(), g = c.createGain();
            n.type = type; sweep(n.frequency, f0, f1, t, dur); env(g, t, peak, o.attack ?? .001, dur);
            if (o.lp) { const f = c.createBiquadFilter(); f.type = 'lowpass'; f.frequency.value = o.lp; n.connect(f); f.connect(g); } else n.connect(g);
            g.connect(dest); n.start(t); n.stop(t + dur + .02);
            return n;
        },
        // Filtered noise burst: ftype = bandpass | lowpass | highpass, centre swept f0 -> f1.
        nz(kind, ftype, f0, f1, q, t, dur, peak, dest, o = {}) {
            const c = engine.ctx, src = c.createBufferSource(), f = c.createBiquadFilter(), g = c.createGain();
            src.buffer = engine.noise(kind); src.loop = true; f.type = ftype; f.Q.value = q; sweep(f.frequency, f0, f1, t, dur);
            env(g, t, peak, o.attack ?? .001, dur); src.connect(f); f.connect(g); g.connect(dest);
            src.start(t, rnd() * 1.5); src.stop(t + dur + .02);
        },
        smp(name, t, gain, rate, dest, o = {}) {
            const buf = engine.buffer(name);
            if (!buf) return false;
            const c = engine.ctx, src = c.createBufferSource(), g = c.createGain();
            src.buffer = buf; src.playbackRate.value = rate; g.gain.value = gain;
            if (o.lp) { const f = c.createBiquadFilter(); f.type = 'lowpass'; f.frequency.value = o.lp; src.connect(f); f.connect(g); } else src.connect(g);
            g.connect(dest); src.start(t);
            return true;
        },
        // One randomly chosen take of a mechanical role (samples.js): playbackRate +-4 %, gain +-1.5 dB, tail faded so a cut never clicks.
        // o.lp / o.hp: optional filters; o.pick: fixed take index; returns false when the sample is not loaded.
        take(role, t, gain, rate, dest, o = {}) {
            const list = TAKES[role];
            if (!list || !list.length) return false;
            const tk = list[o.pick ?? Math.floor(Math.random() * list.length)] || list[0], buf = engine.buffer(tk.s);
            if (!buf) return false;
            const c = engine.ctx, src = c.createBufferSource(), g = c.createGain();
            const r = Math.max(.25, rate * (1 + (Math.random() - .5) * .08)), level = gain * tk.g * Math.pow(10, (Math.random() - .5) * 3 / 20);
            const span = Math.max(.02, Math.min(tk.dur ?? buf.duration - tk.off, buf.duration - tk.off)), len = span / r, fade = Math.min(.03, len * .35);
            src.buffer = buf; src.playbackRate.value = r;
            g.gain.setValueAtTime(level, t); g.gain.setValueAtTime(level, t + len - fade); g.gain.linearRampToValueAtTime(0, t + len);
            let tail = src;
            if (o.hp) { const f = c.createBiquadFilter(); f.type = 'highpass'; f.frequency.value = o.hp; tail.connect(f); tail = f; }
            if (o.lp) { const f = c.createBiquadFilter(); f.type = 'lowpass'; f.frequency.value = o.lp; tail.connect(f); tail = f; }
            tail.connect(g); g.connect(dest); src.start(t, tk.off, span);
            return true;
        },
        // AM-modulated oscillator through a lowpass for buzzers and hums; returns {stop(t)}.
        buzz(type, freq, am, depth, lp, t, dur, peak, dest) {
            const c = engine.ctx, o = c.createOscillator(), f = c.createBiquadFilter(), g = c.createGain(), env2 = c.createGain(), l = c.createOscillator(), ld = c.createGain();
            o.type = type; o.frequency.value = freq; f.type = 'lowpass'; f.frequency.value = lp;
            g.gain.value = 1 - depth / 2; l.type = 'square'; l.frequency.value = am; ld.gain.value = depth / 2; l.connect(ld); ld.connect(g.gain);
            env2.gain.setValueAtTime(0.0001, t); env2.gain.linearRampToValueAtTime(peak, t + .01); env2.gain.setValueAtTime(peak, t + dur - .05); env2.gain.linearRampToValueAtTime(0.0001, t + dur);
            o.connect(f); f.connect(g); g.connect(env2); env2.connect(dest);
            o.start(t); l.start(t); o.stop(t + dur + .02); l.stop(t + dur + .02);
        }
    };
}


const R = (priority, dur, wet, run) => ({ priority, dur, wet, run });
const BELL = [[1, 1, 1.4], [2.76, .5, .9], [5.4, .25, .5], [8.93, .12, .3]]; // inharmonic partials: ratio, level, decay
function bell(k, t, f, peak, d, len = 1.4) { for (const [ratio, level, decay] of BELL) k.osc('sine', f * ratio, 0, t, len * decay, peak * level, d, { attack: .002 }); }
// Low body under a recorded hit: sine thump (<= 130 Hz) plus a dull noise knock. No ringing partials.
function thump(k, t, f, peak, d, len = .09) { k.osc('sine', f, f * .5, t, len, peak, d, { attack: .002 }); k.nz('brown', 'lowpass', 500, 250, .7, t, len * .6, peak * .5, d); }
// Pitched-down copy of the same role, for weight under the main strike.
const weight = (k, role, t, g, r, d, o = {}) => k.take(role, t, g, r * .62, d, { lp: 2600, ...o });

export const RECIPES = {
    // ---- flippers: coil strike (metal-on-metal take) + weight layer + low thump ----
    flipperUp: R(8, .3, .1, (k, t, r, d) => {
        k.take('flipperUp', t, 1.25, r, d); weight(k, 'flipperUp', t, .35, r, d);
        thump(k, t, 105 * r, .6, d, .08);
        k.nz('white', 'bandpass', 1400, 0, .8, t, .006, .3, d);
    }),
    flipperDown: R(5, .18, .05, (k, t, r, d) => {
        k.take('flipperDown', t, .55, r * 1.1, d, { lp: 5000 });
        thump(k, t, 85 * r, .4, d, .06);
    }),
    // ---- pop bumper thock, slingshot, targets ----
    bumper: R(7, .3, .15, (k, t, r, d) => {
        k.take('bumper', t, 1, r, d); weight(k, 'bumper', t, .25, r, d, { lp: 1800 });
        thump(k, t, 95 * r, .45, d, .07);
        k.buzz('square', 100 * r, 55, 1, 520, t, .035, .22, d); // coil buzz
    }),
    sling: R(6, .24, .08, (k, t, r, d) => {
        k.take('sling', t, 2, r * 1.05, d); k.take('impactRubber', t, .6, r * .9, d); k.take('kickback', t, .45, r * 1.25, d, { lp: 3500 });
        thump(k, t, 115 * r, .9, d, .07);
        k.nz('white', 'lowpass', 900, 0, .8, t, .02, .35, d);
    }),
    drop: R(5, .3, .12, (k, t, r, d) => { k.take('drop', t, 1, r, d); thump(k, t, 120 * r, .4, d, .05); }),
    bankReset: R(6, .6, .15, (k, t, r, d) => {
        for (let i = 0; i < 3; i++) {
            const s = t + i * .085 + Math.random() * .02;
            k.take('drop', s, .85, r * (.92 + i * .06), d); thump(k, s, 110, .45, d, .08);
        }
        thump(k, t, 90 * r, .5, d, .2);
    }),
    standup: R(4, .15, .06, (k, t, r, d) => { k.take('standup', t, .9, r * 1.1, d); thump(k, t, 130 * r, .25, d, .04); }),
    // ---- switches ----
    spinnerTick: R(2, .08, .02, (k, t, r, d) => { k.take('tick', t, .8, r * 1.35, d, { hp: 700 }); }),
    rollover: R(2, .1, .06, (k, t, r, d) => { k.take('tick', t, .4, r * 1.2, d, { lp: 4200 }); k.take('impactWood', t, .15, r * 1.5, d, { lp: 3000 }); }),
    gate: R(2, .16, .08, (k, t, r, d) => { k.take('tick', t, .35, r * .9, d, { lp: 3200 }); k.take('tick', t + .035, .25, r * 1.1, d, { lp: 3600 }); }),
    // ---- holes and coils ----
    scoop: R(6, .5, .15, (k, t, r, d) => {
        k.take('scoop', t, 1, r * .92, d); thump(k, t, 95 * r, .6, d, .1);
        k.take('clack', t + .09, .35, r * .8, d, { lp: 3500 });
    }),
    eject: R(7, .3, .08, (k, t, r, d) => { k.take('eject', t, 1, r, d); thump(k, t, 120 * r, .75, d, .09); k.nz('white', 'lowpass', 1200, 0, .8, t, .03, .35, d); }),
    kickback: R(8, .4, .12, (k, t, r, d) => { k.take('kickback', t, 1, r * 1.05, d); weight(k, 'kickback', t, .5, r, d); thump(k, t, 100 * r, .85, d, .14); }),
    knocker: R(9, 1.2, .35, (k, t, r, d) => {
        k.take('knocker', t, 1, r * .85, d); weight(k, 'knocker', t, .7, r * .9, d);
        k.osc('sine', 110 * r, 48 * r, t, .3, .85, d, { attack: .003 }); k.osc('sine', 58 * r, 45 * r, t, .5, .6, d, { attack: .004 });
        k.nz('brown', 'lowpass', 500, 0, .6, t, .05, .6, d);
    }),
    plungerRatchet: R(1, .05, .03, (k, t, r, d) => { k.take('tick', t, .7, r * 1.15, d, { hp: 500 }); }),
    plungerRelease: R(8, .5, .12, (k, t, r, d) => {
        k.take('plungerRelease', t, 1, r, d); weight(k, 'plungerRelease', t, .5, r, d);
        thump(k, t, 100 * r, .8, d, .2); // spring thud
        for (let i = 0; i < 3; i++) k.take('tick', t + .012 * i, .25, r * (.8 + i * .1), d, { lp: 3500 }); // coil chatter
    }),
    magnetOn: R(4, .2, .1, (k, t, r, d) => { k.osc('sine', 80 * r, 0, t, .1, .4, d); k.nz('white', 'lowpass', 700, 0, .8, t, .04, .3, d); }),
    clack: R(3, .12, .05, (k, t, r, d) => { k.take('clack', t, .85, r, d); }),
    // ---- ball impacts by material ----
    impactWood: R(1, .18, .06, (k, t, r, d) => { k.take('impactWood', t, .8, r * .95, d); thump(k, t, 130 * r, .2, d, .03); }),
    impactMetal: R(1, .2, .08, (k, t, r, d) => { k.take('impactMetal', t, .8, r, d); }),
    impactPlastic: R(1, .12, .05, (k, t, r, d) => { k.take('tick', t, .5, r * 1.1, d, { lp: 3500 }); k.nz('white', 'bandpass', 1800 * r, 0, .9, t, .01, .25, d); }),
    impactRubber: R(1, .15, .04, (k, t, r, d) => { k.take('impactRubber', t, .8, r, d); thump(k, t, 100 * r, .3, d, .04); }),
    impactFlipper: R(1, .12, .04, (k, t, r, d) => { k.take('impactRubber', t, .6, r * .8, d); thump(k, t, 90 * r, .25, d, .04); }),
    impactTarget: R(1, .15, .06, (k, t, r, d) => { k.take('impactTarget', t, .8, r, d); }),
    impactBall: R(2, .12, .06, (k, t, r, d) => { k.take('clack', t, .8, r * 1.3, d); }),
    // ---- drain, tilt ----
    drain: R(8, 1.2, .35, (k, t, r, d) => {
        k.take('drain', t, .9, r, d); k.nz('pink', 'bandpass', 1800, 200, 1.5, t, .9, .35, d, { attack: .05 });
        k.osc('sine', 300 * r, 60 * r, t, .8, .4, d, { attack: .02 });
    }),
    tiltBuzzer: R(10, 1.4, .1, (k, t, r, d) => { k.buzz('square', 110 * r, 8, 1, 1200, t, 1.2, .55, d); }),
    tiltWarnBuzz: R(9, .5, .08, (k, t, r, d) => { k.buzz('square', 130 * r, 8, 1, 1200, t, .35, .45, d); }),
    // ---- cue accents (the music layer adds the melody stingers on top) ----
    bell: R(8, 1.6, .45, (k, t, r, d) => { bell(k, t, 880 * r, .35, d); }),
    bellHigh: R(6, .9, .4, (k, t, r, d) => { bell(k, t, 1320 * r, .25, d, .8); }),
    jackpotHit: R(10, 2, .5, (k, t, r, d) => {
        k.take('jackpot', t, .8, r, d);
        [880, 1109, 1319].forEach((f, i) => bell(k, t + i * .06, f * r, .28, d, 1.8));
        k.osc('sine', 55 * r, 40 * r, t, .6, .8, d, { attack: .005 });
    }),
    lockThunk: R(8, .8, .2, (k, t, r, d) => { k.take('lock', t, 1, r * .9, d); thump(k, t, 100 * r, .7, d, .3); }),
    riser: R(8, 1.4, .4, (k, t, r, d) => {
        k.nz('pink', 'bandpass', 300, 4000, 2, t, 1.1, .35, d, { attack: .9 });
        k.osc('sawtooth', 110 * r, 440 * r, t, 1.1, .2, d, { lp: 1800, attack: .9 });
    }),
    descend: R(8, 1.6, .4, (k, t, r, d) => { k.osc('triangle', 440 * r, 110 * r, t, 1.3, .4, d, { attack: .01 }); k.osc('sine', 110 * r, 55 * r, t, 1.4, .5, d, { attack: .02 }); }),
    tick: R(3, .08, .05, (k, t, r, d) => { k.take('tick', t, .6, r * 1.2, d, { lp: 5000 }); }),
    bonusTotal: R(8, 1.8, .4, (k, t, r, d) => { k.take('bonus', t, .75, r, d); bell(k, t + .05, 1175 * r, .25, d, 1.4); })
};
export const RECIPE_NAMES = Object.freeze(Object.keys(RECIPES));
// Music dip (dB) under the hits that must cut through the bed.
for (const [name, db] of Object.entries({ flipperUp: -8, bumper: -6, sling: -8, kickback: -5, eject: -4, plungerRelease: -4, scoop: -4, knocker: -5, drop: -3, bankReset: -4 })) RECIPES[name].duck = db;
