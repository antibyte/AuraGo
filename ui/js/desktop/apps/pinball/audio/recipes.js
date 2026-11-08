// AuraPinball synth recipes: every mechanical sound is a short WebAudio patch layered over a CC0 sample.
// Imports cleanly in Node (no AudioContext access at load). Each recipe is run(k, t, rate, d) -> void, where
// k = kit (osc / nz / smp helpers), t = start time, rate = pitch factor (~1), d = destination node of the voice.
// RECIPES[name] = { run, priority, dur, wet } : priority decides who the 24-voice limiter drops, dur is the
// tail length in seconds, wet is the reverb send level.

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
function clunk(k, t, f, peak, d) { k.osc('sine', f, f * .5, t, .09, peak, d); k.nz('brown', 'lowpass', 900, 400, .7, t, .05, peak * .8, d); }

export const RECIPES = {
    // ---- flippers: coil thwack = 8 ms noise click + 110->60 Hz body thump + sample ----
    flipperUp: R(8, .25, .12, (k, t, r, d) => {
        k.nz('white', 'bandpass', 2200 * r, 0, 1.2, t, .008, .75, d);
        k.osc('sine', 110 * r, 60 * r, t, .09, .95, d, { attack: .002 });
        k.osc('square', 3600 * r, 0, t, .004, .12, d);
        k.osc('sawtooth', 100 * r, 70 * r, t, .05, .18, d, { lp: 500 });
        k.smp('flipper', t, .5, r, d);
    }),
    flipperDown: R(5, .15, .05, (k, t, r, d) => {
        k.nz('white', 'bandpass', 1500 * r, 0, 2, t, .006, .35, d);
        k.osc('sine', 85 * r, 50 * r, t, .05, .38, d, { attack: .002 });
        k.smp('flipper', t, .16, r * 1.25, d, { lp: 2500 });
    }),
    // ---- pop bumper "thock", slingshot, targets ----
    bumper: R(7, .35, .2, (k, t, r, d) => {
        k.smp('bumper', t, .8, r, d);
        k.osc('square', 70 * r, 55 * r, t, .03, .5, d, { lp: 400 });
        k.nz('white', 'bandpass', 3000, 0, 1.5, t, .014, .45, d);
        k.osc('sine', 1200 * r, 0, t, .12, .22, d); k.osc('sine', 1810 * r, 0, t, .08, .1, d);
    }),
    sling: R(6, .22, .1, (k, t, r, d) => {
        k.smp('sling', t, .75, r, d); k.smp('rubber', t, .4, r * 1.2, d);
        k.osc('sine', 140 * r, 70 * r, t, .05, .85, d, { attack: .002 });
        k.nz('white', 'lowpass', 900, 0, .8, t, .025, .5, d);
    }),
    drop: R(5, .3, .15, (k, t, r, d) => { k.smp('target', t, .8, r, d); k.osc('sine', 500 * r, 380 * r, t, .06, .5, d); k.nz('white', 'bandpass', 1800, 0, 2, t, .015, .35, d); }),
    bankReset: R(6, .6, .2, (k, t, r, d) => {
        for (let i = 0; i < 3; i++) { const s = t + i * .09; clunk(k, s, 230 * r, .6, d); k.smp('target', s, .45, r * .7, d); }
        k.osc('sawtooth', 120 * r, 110 * r, t, .3, .22, d, { lp: 600, attack: .01 });
    }),
    standup: R(4, .15, .08, (k, t, r, d) => { k.smp('target', t, .7, r * 1.15, d); k.osc('sine', 900 * r, 600 * r, t, .025, .3, d); k.nz('white', 'highpass', 4000, 0, .7, t, .005, .25, d); }),
    // ---- switches ----
    spinnerTick: R(2, .03, .02, (k, t, r, d) => { k.osc('sine', 3200 * r, 0, t, .004, .4, d); k.nz('white', 'highpass', 5000, 0, .7, t, .003, .18, d); }),
    rollover: R(2, .12, .1, (k, t, r, d) => { k.osc('triangle', 1760 * r, 1700 * r, t, .04, .22, d); k.nz('white', 'bandpass', 4000, 0, 3, t, .003, .2, d); }),
    gate: R(2, .2, .15, (k, t, r, d) => { k.osc('sine', 2800 * r, 0, t, .09, .25, d); k.osc('sine', 5600 * r, 0, t, .05, .08, d); }),
    // ---- holes and coils ----
    scoop: R(6, .6, .2, (k, t, r, d) => { k.smp('scoop', t, .9, r * .9, d); k.osc('sine', 90 * r, 60 * r, t, .1, .5, d); k.nz('white', 'bandpass', 2600, 0, 3, t + .05, .08, .15, d); }),
    eject: R(7, .3, .1, (k, t, r, d) => {
        k.osc('sine', 160 * r, 70 * r, t, .09, .85, d, { attack: .002 }); k.nz('white', 'lowpass', 1200, 0, .8, t, .04, .5, d);
        k.smp('clack', t, .5, r * .9, d); k.osc('sine', 600 * r, 300 * r, t, .12, .1, d);
    }),
    kickback: R(8, .4, .2, (k, t, r, d) => {
        k.smp('launch', t, .8, r * 1.1, d); k.osc('sine', 120 * r, 45 * r, t, .16, .9, d, { attack: .002 });
        k.nz('white', 'bandpass', 1500, 0, 1, t, .02, .6, d); k.osc('sine', 2000 * r, 0, t, .2, .1, d);
    }),
    knocker: R(9, 1.2, .4, (k, t, r, d) => {
        k.smp('knock', t, 1, r * .8, d); k.osc('sine', 120 * r, 50 * r, t, .25, 1, d, { attack: .003 });
        k.osc('sine', 60 * r, 45 * r, t, .5, .8, d, { attack: .004 }); k.nz('brown', 'lowpass', 500, 0, .6, t, .04, .6, d);
    }),
    plungerRatchet: R(1, .05, .04, (k, t, r, d) => { k.nz('white', 'bandpass', 2500 * r, 0, 2, t, .003, .5, d); k.osc('square', 800 * r, 0, t, .002, .15, d, { lp: 3000 }); }),
    plungerRelease: R(8, .5, .15, (k, t, r, d) => {
        k.smp('launch', t, .9, r, d); k.osc('sine', 180 * r, 90 * r, t, .32, .7, d, { attack: .002 });
        k.osc('sine', 360 * r, 180 * r, t, .2, .25, d); k.nz('brown', 'lowpass', 600, 0, .7, t, .05, .5, d);
    }),
    magnetOn: R(4, .2, .1, (k, t, r, d) => { k.osc('sine', 80 * r, 0, t, .1, .4, d); k.nz('white', 'lowpass', 700, 0, .8, t, .04, .3, d); }),
    clack: R(3, .15, .08, (k, t, r, d) => { k.smp('clack', t, .8, r, d); k.nz('white', 'bandpass', 3500 * r, 0, 2, t, .004, .4, d); }),
    // ---- ball impacts by material ----
    impactWood: R(1, .2, .08, (k, t, r, d) => { k.smp('target', t, .7, r * .95, d); k.osc('sine', 200 * r, 150 * r, t, .02, .3, d); }),
    impactMetal: R(1, .25, .1, (k, t, r, d) => { k.smp('clack', t, .8, r, d); k.osc('sine', 3000 * r, 0, t, .06, .12, d); }),
    impactPlastic: R(1, .15, .06, (k, t, r, d) => { k.nz('white', 'bandpass', 2200 * r, 0, 4, t, .012, .5, d); k.osc('sine', 600 * r, 0, t, .02, .25, d); k.smp('rubber', t, .3, r * 1.5, d); }),
    impactRubber: R(1, .15, .04, (k, t, r, d) => { k.smp('rubber', t, .8, r, d); k.osc('sine', 100 * r, 70 * r, t, .04, .3, d); }),
    impactFlipper: R(1, .12, .04, (k, t, r, d) => { k.smp('rubber', t, .6, r * .8, d); k.osc('sine', 90 * r, 60 * r, t, .04, .25, d); }),
    impactTarget: R(1, .2, .08, (k, t, r, d) => { k.smp('target', t, .7, r, d); k.osc('sine', 1400 * r, 0, t, .06, .12, d); }),
    impactBall: R(2, .15, .08, (k, t, r, d) => { k.smp('clack', t, .8, r * 1.3, d); k.osc('sine', 4000 * r, 0, t, .04, .15, d); }),
    // ---- drain, tilt ----
    drain: R(8, 1.2, .35, (k, t, r, d) => {
        k.smp('drain', t, .9, r, d); k.nz('pink', 'bandpass', 1800, 200, 1.5, t, .9, .35, d, { attack: .05 });
        k.osc('sine', 300 * r, 60 * r, t, .8, .4, d, { attack: .02 });
    }),
    tiltBuzzer: R(10, 1.4, .1, (k, t, r, d) => { k.buzz('square', 110 * r, 8, 1, 1200, t, 1.2, .55, d); }),
    tiltWarnBuzz: R(9, .5, .08, (k, t, r, d) => { k.buzz('square', 130 * r, 8, 1, 1200, t, .35, .45, d); }),
    // ---- cue accents (the music layer adds the melody stingers on top) ----
    bell: R(8, 1.6, .45, (k, t, r, d) => { bell(k, t, 880 * r, .35, d); }),
    bellHigh: R(6, .9, .4, (k, t, r, d) => { bell(k, t, 1320 * r, .25, d, .8); }),
    jackpotHit: R(10, 2, .5, (k, t, r, d) => {
        k.smp('jackpot', t, .8, r, d);
        [880, 1109, 1319].forEach((f, i) => bell(k, t + i * .06, f * r, .28, d, 1.8));
        k.osc('sine', 55 * r, 40 * r, t, .6, .8, d, { attack: .005 });
    }),
    lockThunk: R(8, .8, .25, (k, t, r, d) => { k.smp('lock', t, .85, r, d); k.osc('sine', 100 * r, 50 * r, t, .4, .7, d); k.nz('brown', 'lowpass', 400, 0, .6, t, .08, .5, d); }),
    riser: R(8, 1.4, .4, (k, t, r, d) => {
        k.nz('pink', 'bandpass', 300, 4000, 2, t, 1.1, .35, d, { attack: .9 });
        k.osc('sawtooth', 110 * r, 440 * r, t, 1.1, .2, d, { lp: 1800, attack: .9 });
    }),
    descend: R(8, 1.6, .4, (k, t, r, d) => { k.osc('triangle', 440 * r, 110 * r, t, 1.3, .4, d, { attack: .01 }); k.osc('sine', 110 * r, 55 * r, t, 1.4, .5, d, { attack: .02 }); }),
    tick: R(3, .08, .05, (k, t, r, d) => { k.osc('sine', 1800 * r, 0, t, .015, .4, d); k.nz('white', 'bandpass', 4200, 0, 3, t, .006, .2, d); }),
    bonusTotal: R(8, 1.8, .4, (k, t, r, d) => { k.smp('bonus', t, .75, r, d); bell(k, t + .05, 1175 * r, .25, d, 1.4); })
};
export const RECIPE_NAMES = Object.freeze(Object.keys(RECIPES));
