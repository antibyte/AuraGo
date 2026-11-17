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
            if (o.pan && c.createStereoPanner) { const p = c.createStereoPanner(); p.pan.value = Math.max(-1, Math.min(1, o.pan)); g.connect(p); p.connect(dest); }
            else g.connect(dest);
            n.start(t); n.stop(t + dur + .02);
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
            tail.connect(g);
            if (o.pan && c.createStereoPanner) { const p = c.createStereoPanner(); p.pan.value = Math.max(-1, Math.min(1, o.pan)); g.connect(p); p.connect(dest); }
            else g.connect(dest);
            src.start(t, tk.off, span);
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
// Electromechanical pinball chime bar: warm fundamental + metallic overtones up to 8 kHz + crisp striker click.
const CHIME = [[1, 1, 1.35, 'sine', 6800], [2, .28, .85, 'triangle', 6200], [2.756, .24, .52, 'sine', 7600], [4.12, .11, .28, 'sine', 8400]];
function bell(k, t, f, peak, d, len = 1.25, pan = 0) {
    k.take('tick', t, peak * .34, 1.25, d, { hp: 900, lp: 7500, pan });
    for (const [ratio, level, decay, type, lp] of CHIME) k.osc(type, f * ratio, 0, t, len * decay, peak * level, d, { attack: .002, lp, pan });
}
// Low body under a recorded hit: sine thump (<= 130 Hz) plus a dull noise knock. No ringing partials.
function thump(k, t, f, peak, d, len = .09) { k.osc('sine', f, f * .5, t, len, peak, d, { attack: .002 }); k.nz('brown', 'lowpass', 480, 220, .7, t, len * .65, peak * .5, d); }
// Pitched-down copy of the same role, for weight under the main strike.
const weight = (k, role, t, g, r, d, o = {}) => k.take(role, t, g, r * .62, d, { lp: 2400, ...o });

export const RECIPES = {
    // ---- flippers: crisp steel solenoid strike + tight EOS stop + cabinet thud ----
    flipperUp: R(8, .3, .1, (k, t, r, d) => {
        k.take('flipperUp', t, 1.28, r, d); weight(k, 'flipperUp', t, .38, r, d);
        k.take('impactMetal', t + .008, .34, r * 1.15, d, { hp: 450 });
        thump(k, t, 102 * r, .62, d, .08);
    }),
    flipperDown: R(5, .2, .05, (k, t, r, d) => {
        k.take('flipperDown', t, .72, r * 1.04, d, { lp: 4600 });
        k.take('impactRubber', t + .008, .38, r * .95, d);
        thump(k, t, 82 * r, .46, d, .065);
    }),
    // ---- pop bumper: instant (<6 ms) metal ring-rod snap + solid body thock + cabinet punch ----
    bumper: R(7, .28, .13, (k, t, r, d) => {
        k.take('bumper', t, .95, r, d); weight(k, 'bumper', t, .26, r, d, { lp: 1800 });
        k.take('clack', t, .46, r * 1.18, d, { hp: 550 });
        k.take('standup', t + .003, .28, r * 1.15, d);
        thump(k, t, 96 * r, .48, d, .068);
    }),
    sling: R(6, .24, .08, (k, t, r, d) => {
        k.take('sling', t, 1.85, r * 1.05, d); k.take('impactRubber', t, .68, r * .92, d); k.take('kickback', t, .45, r * 1.22, d, { lp: 3600 });
        thump(k, t, 112 * r, .88, d, .075);
        k.nz('pink', 'lowpass', 950, 260, .8, t, .022, .3, d);
    }),
    drop: R(5, .3, .12, (k, t, r, d) => { k.take('drop', t, 1, r, d); k.take('clack', t + .018, .28, r * .9, d, { lp: 3200 }); thump(k, t, 118 * r, .42, d, .055); }),
    bankReset: R(6, .6, .15, (k, t, r, d) => {
        for (let i = 0; i < 3; i++) {
            const s = t + i * .085 + Math.random() * .02;
            k.take('drop', s, .85, r * (.92 + i * .06), d); thump(k, s, 108, .45, d, .08);
        }
        thump(k, t, 88 * r, .52, d, .2);
    }),
    standup: R(4, .16, .06, (k, t, r, d) => { k.take('standup', t, .95, r * 1.08, d); k.take('impactRubber', t, .3, r, d); thump(k, t, 124 * r, .28, d, .045); }),
    // ---- switches ----
    spinnerTick: R(2, .08, .03, (k, t, r, d) => { k.take('tick', t, .84, r * 1.34, d, { hp: 750 }); }),
    rollover: R(2, .11, .06, (k, t, r, d) => { k.take('tick', t, .48, r * 1.15, d, { lp: 4400 }); k.take('impactWood', t, .18, r * 1.4, d, { lp: 3000 }); }),
    gate: R(2, .16, .08, (k, t, r, d) => { k.take('tick', t, .4, r * .92, d, { lp: 3400 }); k.take('tick', t + .035, .28, r * 1.12, d, { lp: 3800 }); }),
    // ---- holes and coils ----
    scoop: R(6, .5, .15, (k, t, r, d) => {
        k.take('scoop', t, 1, r * .94, d); thump(k, t, 94 * r, .62, d, .11);
        k.take('clack', t + .085, .42, r * .84, d, { lp: 3600 });
    }),
    eject: R(7, .3, .08, (k, t, r, d) => { k.take('eject', t, 1, r, d); thump(k, t, 116 * r, .76, d, .09); k.nz('pink', 'lowpass', 1200, 300, .8, t, .03, .32, d); }),
    kickback: R(8, .4, .12, (k, t, r, d) => { k.take('kickback', t, 1, r * 1.05, d); weight(k, 'kickback', t, .52, r, d); thump(k, t, 98 * r, .85, d, .14); }),
    knocker: R(9, 1.2, .35, (k, t, r, d) => {
        k.take('knocker', t, 1, r * .85, d); weight(k, 'knocker', t, .7, r * .9, d);
        k.osc('sine', 110 * r, 48 * r, t, .3, .85, d, { attack: .003 }); k.osc('sine', 58 * r, 45 * r, t, .5, .6, d, { attack: .004 });
        k.nz('brown', 'lowpass', 500, 0, .6, t, .05, .6, d);
    }),
    plungerRatchet: R(1, .06, .03, (k, t, r, d) => { k.take('tick', t, .75, r * 1.08, d, { hp: 450, lp: 5400 }); }),
    plungerRelease: R(8, .5, .12, (k, t, r, d) => {
        k.take('plungerRelease', t, 1, r, d); weight(k, 'plungerRelease', t, .52, r, d);
        thump(k, t, 98 * r, .82, d, .2);
        for (let i = 0; i < 3; i++) k.take('tick', t + .013 * i, .28, r * (.82 + i * .1), d, { lp: 3800 });
    }),
    magnetOn: R(4, .22, .1, (k, t, r, d) => { k.osc('sine', 80 * r, 58 * r, t, .14, .42, d, { lp: 400 }); k.nz('pink', 'lowpass', 680, 220, .9, t, .05, .28, d); }),
    clack: R(3, .12, .05, (k, t, r, d) => { k.take('clack', t, .88, r, d); }),
    // ---- ball impacts by material ----
    impactWood: R(1, .18, .06, (k, t, r, d) => { k.take('impactWood', t, .82, r * .95, d); thump(k, t, 126 * r, .22, d, .032); }),
    impactMetal: R(1, .2, .08, (k, t, r, d) => { k.take('impactMetal', t, .82, r, d); }),
    impactPlastic: R(1, .12, .05, (k, t, r, d) => { k.take('tick', t, .52, r * 1.08, d, { lp: 3600 }); k.nz('pink', 'bandpass', 1650 * r, 900 * r, 1.1, t, .014, .24, d); }),
    impactRubber: R(1, .15, .04, (k, t, r, d) => { k.take('impactRubber', t, .82, r, d); thump(k, t, 98 * r, .32, d, .04); }),
    impactFlipper: R(1, .12, .04, (k, t, r, d) => { k.take('impactRubber', t, .65, r * .84, d); thump(k, t, 90 * r, .28, d, .04); }),
    impactTarget: R(1, .15, .06, (k, t, r, d) => { k.take('impactTarget', t, .84, r, d); }),
    impactBall: R(2, .12, .06, (k, t, r, d) => { k.take('clack', t, .85, r * 1.28, d); }),
    // ---- drain, tilt ----
    drain: R(8, 1.25, .32, (k, t, r, d) => {
        k.take('drain', t, .95, r, d);
        k.take('impactWood', t + .065, .58, r * .88, d, { pan: -.18 });
        k.take('clack', t + .19, .46, r * .82, d, { lp: 3200, pan: .18 });
        k.nz('brown', 'lowpass', 380, 95, .9, t, .68, .42, d, { attack: .02 });
        k.osc('sine', 92 * r, 44 * r, t, .46, .42, d, { attack: .005 });
    }),
    tiltBuzzer: R(10, 1.4, .1, (k, t, r, d) => { k.buzz('square', 110 * r, 8, 1, 1100, t, 1.2, .52, d); }),
    tiltWarnBuzz: R(9, .5, .08, (k, t, r, d) => { k.buzz('square', 130 * r, 8, 1, 1100, t, .35, .42, d); }),
    // ---- cue accents (the music layer adds the melody stingers on top) ----
    bell: R(8, 1.5, .42, (k, t, r, d) => { bell(k, t, 784 * r, .28, d, 1.3); }),
    bellHigh: R(6, .95, .38, (k, t, r, d) => { bell(k, t, 1175 * r, .22, d, .8); }),
    jackpotHit: R(10, 2.1, .48, (k, t, r, d) => {
        k.take('bonus', t, .64, r * 1.05, d);
        k.take('jackpot', t, .12, r, d, { lp: 1500 });
        k.take('knocker', t, .44, r * .95, d, { lp: 3600 });
        k.take('sling', t, .45, r * 1.28, d, { hp: 900 });
        [587.33, 739.99, 880, 1108.73, 1318.51].forEach((f, i) => bell(k, t + i * .055, f * r, .19, d, 1.5, (i % 2 ? .4 : -.4) * (i < 4 ? 1 : 0)));
        k.osc('sine', 64 * r, 42 * r, t, .52, .52, d, { attack: .004 });
    }),
    surpriseFanfare: R(9, 1.85, .45, (k, t, r, d) => {
        k.take('bonus', t, .58, r * 1.16, d);
        k.take('sling', t, .4, r * 1.32, d, { hp: 900 });
        [659.25, 880, 1174.66, 1567.98].forEach((f, i) => bell(k, t + i * .06, f * r, .18, d, 1.3, i % 2 ? .42 : -.42));
        k.osc('sine', 78 * r, 48 * r, t, .4, .42, d, { attack: .004 });
    }),
    lockThunk: R(8, .8, .2, (k, t, r, d) => { k.take('lock', t, 1, r * .9, d); thump(k, t, 98 * r, .72, d, .28); }),
    riser: R(8, 1.4, .4, (k, t, r, d) => {
        k.nz('pink', 'bandpass', 280, 3400, 1.8, t, 1.1, .34, d, { attack: .88 });
        k.osc('sawtooth', 110 * r, 440 * r, t, 1.1, .18, d, { lp: 1650, attack: .88, pan: -.2 });
        k.osc('triangle', 165 * r, 660 * r, t, 1.1, .14, d, { lp: 2100, attack: .88, pan: .2 });
    }),
    descend: R(8, 1.6, .4, (k, t, r, d) => { k.osc('triangle', 392 * r, 98 * r, t, 1.3, .38, d, { attack: .01, lp: 1800 }); k.osc('sine', 110 * r, 52 * r, t, 1.4, .48, d, { attack: .02 }); }),
    tick: R(3, .08, .05, (k, t, r, d) => { k.take('tick', t, .65, r * 1.15, d, { lp: 5200 }); }),
    bonusTotal: R(8, 1.8, .42, (k, t, r, d) => { k.take('bonus', t, .72, r, d, { lp: 4200 }); bell(k, t + .05, 987.77 * r, .2, d, 1.3, -.25); bell(k, t + .11, 1174.66 * r, .2, d, 1.35, .25); })
};
export const RECIPE_NAMES = Object.freeze(Object.keys(RECIPES));
// Music dip (dB) under the hits that must cut through the bed.
for (const [name, db] of Object.entries({ flipperUp: -7, bumper: -6, sling: -7, kickback: -5, eject: -4, plungerRelease: -4, scoop: -4, knocker: -5, drop: -3, bankReset: -4, jackpotHit: -6, surpriseFanfare: -5 })) RECIPES[name].duck = db;
