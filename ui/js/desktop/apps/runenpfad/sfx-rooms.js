/* Runenpfad room sound recipes (appends to RunenpfadSfx.SYNTHS and RunenpfadSfx.CUES): breakables, push blocks, plates,
   levers, gates, traps, braziers, cracked walls, keys, pages, the six guardians, dice and the task board.
   DOM-free and deterministic (seeded jitter only). Loads right after sfx.js; does nothing when RunenpfadSfx is missing. */
(function (root) {
    'use strict';

    const Sfx = root.RunenpfadSfx;
    if (!Sfx || typeof Sfx.builder !== 'function') return;
    const builder = Sfx.builder;

    function jit(o, i) {
        const x = Math.sin((o.v + 0.173) * 91.7 + i * 12.9898) * 43758.5453;
        return x - Math.floor(x);
    }

    function wrap(fn) {
        return function (ac, noise, dest, t, o) {
            const b = builder(ac, noise, dest, t);
            const oo = Object.assign({ gain: 0.4, sr: 1, v: 0 }, o);
            fn(b, t, oo.gain, oo.sr || 1, oo);
            return b.done();
        };
    }

    // n short band-passed grit ticks scattered over span seconds (debris, shards, splinters, chain links).
    function scatter(b, o, t, n, span, g, fLo, fHi, q, seed, curve) {
        for (let i = 0; i < n; i++) {
            const at = t + Math.pow(jit(o, i + seed), curve || 1.5) * span;
            const lvl = g * (0.45 + 0.55 * jit(o, i + seed + 40)) * (1 - (at - t) / (span * 1.5));
            b.n(at, 0.012 + jit(o, i + seed + 80) * 0.014, lvl, { color: 'crackle', type: 'bandpass', f0: fLo + jit(o, i + seed + 120) * (fHi - fLo), q: q });
        }
    }

    function coin(b, t, rate, peak, dur) {
        if (!b.model('coin', t, rate, peak, dur)) b.o('sine', 2400 * rate, 0, t, 0.09, peak * 0.4, { atk: 0.002 });
    }

    function paperFlip(b, t, g, sr) {
        b.n(t, 0.2, g * 0.5, { color: 'white', type: 'bandpass', f0: 2200 * sr, f1: 5200 * sr, q: 0.9, atk: 0.07 });
        b.n(t, 0.12, g * 0.2, { color: 'pink', type: 'highpass', f0: 3500, q: 0.5, atk: 0.03 });
        b.n(t + 0.19, 0.05, g * 0.38, { color: 'crackle', type: 'bandpass', f0: 2400 * sr, q: 1.4, atk: 0.004 });
        b.n(t + 0.19, 0.07, g * 0.25, { color: 'brown', type: 'lowpass', f0: 420, q: 0.7, atk: 0.003 });
    }

    function gate(shut) {
        // Heavy iron gate grinding in its track: a faint rumble bed, a 14 Hz ratchet, 26 irregular chain-rattle grains (2-5 kHz), two slowly drifting metal squeals; shut ends in a bright inharmonic clang.
        return wrap(function (b, t, g, sr, o) {
            const d = shut ? 0.7 : 0.95;
            const out = b.sat('warm');
            b.n(t, d, g * 0.16, { color: 'brown', type: 'lowpass', f0: shut ? 260 : 420, f1: shut ? 420 : 260, q: 1.2, atk: 0.08, to: out });
            b.o('sawtooth', 55 * sr, 50 * sr, t, d, g * 0.05, { filter: 'lowpass', ff: 200, atk: 0.1, to: out });
            b.n(t, d, g * 0.4, { color: 'pink', type: 'bandpass', f0: (shut ? 1550 : 950) * sr, f1: (shut ? 950 : 1600) * sr, q: 9, atk: 0.15, to: out });
            b.n(t + 0.1, d - 0.15, g * 0.25, { color: 'pink', type: 'bandpass', f0: (shut ? 1250 : 1150) * sr, f1: (shut ? 1000 : 1400) * sr, q: 11, atk: 0.2, to: out });
            for (let i = 0; i < 26; i++) {
                const at = t + 0.03 + jit(o, i) * (d - 0.1);
                b.n(at, 0.012 + jit(o, i + 30) * 0.015, g * (0.25 + jit(o, i + 60) * 0.45), { color: 'crackle', type: 'bandpass', f0: (2000 + jit(o, i + 90) * 3000) * sr, q: 3, to: out });
            }
            for (let i = 0; i < Math.floor(d * 14); i++) b.n(t + i / 14, 0.01, g * 0.3, { color: 'crackle', type: 'bandpass', f0: 3200 * sr, q: 4, to: out });
            if (shut) {
                const at = t + d - 0.2;
                [1850, 2630, 3470].forEach(function (f, i) { b.o('sine', f * sr * (1 + (jit(o, i + 5) - 0.5) * 0.02), 0, at, 0.25 - i * 0.04, g * 0.13, { atk: 0.001, to: out }); });
                b.model('anvil', at, 1.3 * sr, g * 0.35, 0.3, { to: out });
                b.n(at, 0.03, g * 0.8, { color: 'crackle', type: 'highpass', f0: 2500, to: out });
            } else {
                b.n(t + d - 0.1, 0.02, g * 0.5, { color: 'crackle', type: 'bandpass', f0: 2200, q: 3, to: out });
                b.n(t + d - 0.1, 0.12, g * 0.1, { color: 'brown', type: 'lowpass', f0: 400, f1: 200, q: 0.7, to: out });
            }
        });
    }

    // ---- guardian voices (short colour layers) and tells (rising build-ups, at least 0.6 s) ----
    const GUARDIAN_VOICES = {
        brood: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.28, g * 0.6, { color: 'pink', f0: 700 * sr, f1: 200 * sr, q: 2.5, atk: 0.02 });
            b.o('sine', 120 * sr, 58 * sr, t, 0.28, g * 0.6, { atk: 0.01 });
            [0.05, 0.13, 0.2].forEach(function (dt, i) { b.o('sine', (250 + i * 60) * sr, (520 + i * 80) * sr, t + dt, 0.06, g * 0.26, { atk: 0.006 }); });
        }),
        weaver: wrap(function (b, t, g, sr, o) {
            for (let i = 0; i < 6; i++) b.n(t + i * 0.035 + jit(o, i) * 0.012, 0.014, g * 0.5, { color: 'crackle', f0: (3000 + jit(o, i + 4) * 900) * sr, q: 4 });
            b.model('glass', t, 2.4 * sr, g * 0.28, 0.3);
            const s = b.o('sine', 3800 * sr, 5200 * sr, t, 0.22, g * 0.13, { atk: 0.03 });
            b.lfo(s.osc.frequency, 30, 90, t, 0.22);
        }),
        tombward: wrap(function (b, t, g, sr) {
            b.n(t, 0.5, g * 0.55, { color: 'brown', type: 'lowpass', f0: 380 * sr, f1: 160 * sr, q: 1.1, atk: 0.04 });
            b.o('sawtooth', 70 * sr, 48 * sr, t, 0.5, g * 0.3, { filter: 'lowpass', ff: 200, atk: 0.05 });
            b.model('darkbell', t, 0.5 * sr, g * 0.3, 0.8);
            b.n(t, 0.45, g * 0.22, { color: 'pink', f0: 1100 * sr, q: 5, atk: 0.1 });
        }),
        frostheart: wrap(function (b, t, g, sr) {
            b.n(t, 0.5, g * 0.45, { color: 'pink', f0: 1500 * sr, f1: 600 * sr, q: 3, atk: 0.12 });
            const v = b.o('sine', 520 * sr, 380 * sr, t, 0.5, g * 0.2, { atk: 0.12 });
            b.lfo(v.osc.frequency, 6, 12, t, 0.5);
            b.model('glass', t + 0.05, 2.0 * sr, g * 0.26, 0.6);
            b.n(t, 0.4, g * 0.2, { color: 'white', type: 'highpass', f0: 4200, q: 0.6, atk: 0.1 });
        }),
        boilerward: wrap(function (b, t, g, sr) {
            b.o('sawtooth', 90 * sr, 180 * sr, t, 0.4, g * 0.3, { filter: 'lowpass', ff: 520, atk: 0.06 });
            b.n(t, 0.4, g * 0.4, { color: 'pink', type: 'highpass', f0: 2000 * sr, q: 0.6, atk: 0.05 });
            b.model('anvil', t + 0.02, 0.8 * sr, g * 0.34, 0.4);
        }),
        counter: wrap(function (b, t, g, sr, o) {
            for (let i = 0; i < 4; i++) {
                const tt = t + i * 0.07;
                b.n(tt, 0.014, g * 0.45, { color: 'crackle', f0: 2400 * sr, q: 3 });
                b.o('sine', 1600 * sr, 0, tt, 0.012, g * 0.12, { atk: 0.001 });
            }
            b.model('darkbell', t, 0.7 * sr, g * 0.3, 0.7);
            b.o('sawtooth', 200 * sr, 400 * sr, t, 0.3, g * 0.12, { filter: 'bandpass', ff: 700, q: 3, atk: 0.04 });
        })
    };
    const GUARDIAN_TELLS = {
        brood: wrap(function (b, t, g, sr) {
            b.o('sine', 90 * sr, 170 * sr, t, 0.75, g * 0.55, { atk: 0.4 });
            b.n(t, 0.75, g * 0.4, { color: 'pink', type: 'lowpass', f0: 260 * sr, f1: 900 * sr, q: 1.6, atk: 0.35 });
            [0.12, 0.3, 0.46, 0.6].forEach(function (dt, i) { b.o('sine', (220 + i * 55) * sr, (440 + i * 90) * sr, t + dt, 0.07, g * 0.2, { atk: 0.008 }); });
        }),
        weaver: wrap(function (b, t, g, sr, o) {
            for (let i = 0; i < 9; i++) {
                const tt = t + 0.65 * (1 - Math.pow(1 - i / 8, 1.5));
                b.n(tt, 0.014, g * 0.5, { color: 'crackle', f0: (2800 + i * 150) * sr, q: 4 });
                b.o('square', (880 + i * 70) * sr, 0, tt, 0.014, g * 0.08);
            }
            b.o('sine', 2200 * sr, 3600 * sr, t, 0.7, g * 0.1, { atk: 0.4 });
        }),
        tombward: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.8, g * 0.5, { color: 'brown', type: 'lowpass', f0: 150 * sr, f1: 500 * sr, q: 1.3, atk: 0.45 });
            b.o('sawtooth', 45 * sr, 72 * sr, t, 0.8, g * 0.3, { filter: 'lowpass', ff: 180, atk: 0.45 });
            b.model('darkbell', t, 0.55 * sr, g * 0.28, 1.0);
            for (let i = 0; i < 4; i++) b.n(t + 0.1 + i * 0.15 + jit(o, i) * 0.05, 0.02, g * 0.25, { color: 'crackle', f0: 1200 + jit(o, i + 5) * 800, q: 2 });
        }),
        frostheart: wrap(function (b, t, g, sr) {
            b.n(t, 0.75, g * 0.4, { color: 'pink', f0: 300 * sr, f1: 1500 * sr, q: 3, atk: 0.3 });
            const v = b.o('sine', 320 * sr, 640 * sr, t, 0.75, g * 0.2, { atk: 0.3 });
            b.lfo(v.osc.frequency, 6, 14, t, 0.75);
            [1.6, 2.0, 2.5].forEach(function (r, i) { b.model('glass', t + 0.1 + i * 0.2, r * sr, g * 0.24, 0.5); });
            b.n(t + 0.2, 0.5, g * 0.2, { color: 'white', type: 'highpass', f0: 4200, q: 0.6, atk: 0.25 });
        }),
        boilerward: wrap(function (b, t, g, sr) {
            b.n(t, 0.75, g * 0.5, { color: 'pink', type: 'highpass', f0: 1500 * sr, q: 0.6, atk: 0.45 });
            b.o('sawtooth', 90 * sr, 240 * sr, t, 0.75, g * 0.28, { filter: 'lowpass', ff: 600, atk: 0.4 });
            [0, 0.22, 0.44].forEach(function (dt, i) { b.model('anvil', t + dt, (0.6 + i * 0.12) * sr, g * 0.2, 0.25); });
        }),
        counter: wrap(function (b, t, g, sr) {
            for (let i = 0; i < 10; i++) {
                const tt = t + 0.68 * (1 - Math.pow(1 - i / 9, 1.4));
                b.n(tt, 0.014, g * 0.42, { color: 'crackle', f0: 2400 * sr, q: 3 });
                b.o('sine', 1500 * sr, 0, tt, 0.012, g * 0.12, { atk: 0.001 });
            }
            b.model('darkbell', t, 0.7 * sr, g * 0.24, 0.9);
            b.o('sawtooth', 300 * sr, 900 * sr, t, 0.7, g * 0.12, { filter: 'bandpass', ff: 900, q: 3, atk: 0.4 });
        })
    };

    const SYNTHS = {
        // Clay pot smashing: a sharp broadband crack (white noise, 2-7 kHz), a mid body, 14 skittering shard grains of irregular timing, two faint inharmonic ring partials.
        rm_pot: wrap(function (b, t, g, sr, o) {
            const out = b.sat('warm');
            b.n(t, 0.09, g * 1.0, { color: 'white', type: 'bandpass', f0: 4200 * sr, q: 0.5, atk: 0.001, to: out });
            b.n(t, 0.012, g * 0.8, { color: 'crackle', type: 'highpass', f0: 3000, to: out });
            b.n(t + 0.004, 0.1, g * 0.35, { color: 'pink', type: 'bandpass', f0: 1200, q: 0.7, atk: 0.002, to: out });
            scatter(b, o, t + 0.03, 14, 0.3, g * 0.8, 1500 * sr, 7000 * sr, 3.5, 0, 1.3);
            b.o('sine', 3150 * sr, 0, t + 0.004, 0.1, g * 0.05, { atk: 0.001 });
            b.o('sine', 4870 * sr, 0, t + 0.004, 0.07, g * 0.04, { atk: 0.001 });
        }),
        // Wooden crate or barrel breaking: dull noise crack (300 Hz - 2.5 kHz), woody knocks made of band-passed noise bursts, splinter grains.
        rm_crate: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.14, g * 0.9, { color: 'pink', type: 'bandpass', f0: 900 * sr, q: 0.6, atk: 0.004 });
            b.n(t, 0.012, g * 0.2, { color: 'crackle', type: 'highpass', f0: 1800 });
            [0, 0.07, 0.13, 0.2].forEach(function (dt, i) {
                b.n(t + dt + jit(o, i + 5) * 0.02, 0.06, g * (0.7 - i * 0.12), { color: 'brown', type: 'bandpass', f0: (350 + jit(o, i) * 250) * sr, q: 2.5, atk: 0.002 });
            });
            scatter(b, o, t + 0.02, 9, 0.25, g * 0.55, 500 * sr, 2500 * sr, 4, 3, 1.4);
        }),
        // Stone dragged over stone: 0.6 s of band-passed brown and pink noise, amplitude-modulated irregularly at 27-43 Hz so it grinds and judders, a gritty 3 kHz band, grit ticks, a faint noise settle.
        rm_drag: wrap(function (b, t, g, sr, o) {
            const d = 0.6;
            const body = b.n(t, d, g * 0.9, { color: 'brown', type: 'bandpass', f0: 420 * sr, f1: 300 * sr, q: 0.9, atk: 0.05 });
            b.lfo(body.gain, 31, g * 0.5, t, d);
            b.lfo(body.gain, 43, g * 0.3, t, d);
            const mid = b.n(t, d, g * 0.5, { color: 'pink', type: 'bandpass', f0: 800, f1: 600, q: 1.1, atk: 0.06 });
            b.lfo(mid.gain, 27, g * 0.3, t, d);
            b.n(t, d, g * 0.2, { color: 'crackle', type: 'bandpass', f0: 3000, q: 1.5, atk: 0.06 });
            scatter(b, o, t + 0.03, 9, d - 0.15, g * 0.4, 900, 3500, 2, 10, 1);
            b.n(t + d - 0.06, 0.1, g * 0.08, { color: 'brown', type: 'lowpass', f0: 300, q: 0.7, atk: 0.01 });
        }),
        // Sliding on ice: hissing high noise band, a thin glassy whistle and a glass tick.
        rm_slide: wrap(function (b, t, g, sr) {
            b.n(t, 0.36, g * 0.55, { color: 'pink', type: 'highpass', f0: 2600 * sr, q: 0.6, atk: 0.06 });
            b.n(t, 0.36, g * 0.35, { color: 'white', f0: 3400 * sr, f1: 2000 * sr, q: 3, atk: 0.05 });
            b.o('sine', 3300 * sr, 2400 * sr, t + 0.02, 0.3, g * 0.07, { atk: 0.08 });
            b.model('glass', t + 0.04, 2.8 * sr, g * 0.14, 0.3);
        }),
        // Pressure plate: stone thud, metal click, dull scrape.
        rm_plate: wrap(function (b, t, g, sr) {
            b.o('sine', 140 * sr, 62 * sr, t, 0.12, g * 0.7, { atk: 0.002 });
            b.n(t, 0.014, g * 0.55, { color: 'crackle', type: 'highpass', f0: 2400, q: 0.8 });
            b.n(t, 0.07, g * 0.4, { color: 'brown', type: 'lowpass', f0: 700, f1: 240, q: 0.8, atk: 0.003 });
            b.model('anvil', t + 0.01, 0.5 * sr, g * 0.14, 0.2);
        }),
        // Lever: iron clunk with a body thud and three ratchet teeth.
        rm_lever: wrap(function (b, t, g, sr) {
            b.model('anvil', t, 0.62 * sr, g * 0.5, 0.42);
            b.o('sine', 120 * sr, 52 * sr, t, 0.18, g * 0.7, { atk: 0.003 });
            b.n(t, 0.016, g * 0.5, { color: 'crackle', type: 'highpass', f0: 2600 });
            for (let i = 0; i < 3; i++) {
                const tt = t + 0.05 + i * 0.045;
                b.n(tt, 0.012, g * 0.4, { color: 'crackle', f0: (1900 + i * 260) * sr, q: 3.5 });
                b.o('square', (520 + i * 60) * sr, 0, tt, 0.016, g * 0.1);
            }
            b.n(t + 0.02, 0.12, g * 0.25, { color: 'brown', type: 'lowpass', f0: 500, q: 0.8 });
        }),
        rm_gate_open: gate(false),
        rm_gate_shut: gate(true),
        // Spike plate warning: dry tick and a short rising metal scrape.
        rm_warn_spikes: wrap(function (b, t, g, sr) {
            b.n(t, 0.014, g * 0.7, { color: 'crackle', type: 'highpass', f0: 3200 });
            b.n(t + 0.02, 0.2, g * 0.8, { color: 'pink', f0: 1300 * sr, f1: 2600 * sr, q: 3, atk: 0.02 });
            b.o('triangle', 900 * sr, 1250 * sr, t + 0.02, 0.18, g * 0.2, { filter: 'bandpass', ff: 1500, q: 4, atk: 0.02 });
        }),
        // Steam vent warning: pipe knock, rising hiss and a low pressure tone.
        rm_warn_vent: wrap(function (b, t, g, sr) {
            b.o('sine', 190 * sr, 110 * sr, t, 0.08, g * 0.45, { atk: 0.003 });
            b.n(t, 0.42, g * 0.5, { color: 'pink', type: 'highpass', f0: 3600 * sr, q: 0.6, atk: 0.2 });
            b.o('sine', 92 * sr, 130 * sr, t, 0.42, g * 0.3, { atk: 0.2 });
        }),
        // Dart launcher warning: two cocking clicks and a tightening string.
        rm_warn_dart: wrap(function (b, t, g, sr) {
            [0, 0.07].forEach(function (dt) { b.n(t + dt, 0.012, g * 0.6, { color: 'crackle', f0: 2600 * sr, q: 4 }); });
            b.o('sine', 620 * sr, 1050 * sr, t + 0.04, 0.16, g * 0.12, { atk: 0.05 });
        }),
        // Metal spikes shooting out of stone: a fast upward band-passed noise shing (1.5 to 6 kHz in 75 ms), three inharmonic high partials ringing after it, a bright stop.
        rm_spike: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.075, g * 0.9, { color: 'white', type: 'bandpass', f0: 1500 * sr, f1: 6000 * sr, q: 1.8, atk: 0.004 });
            [2840, 4130, 5560].forEach(function (f, i) {
                b.o('sine', f * sr * (1 + (jit(o, i) - 0.5) * 0.02), 0, t + 0.07, 0.24 - i * 0.03, g * 0.12, { atk: 0.001 });
            });
            b.model('anvil', t + 0.07, 2.0 * sr, g * 0.1, 0.25);
            b.n(t + 0.075, 0.012, g * 0.5, { color: 'crackle', type: 'highpass', f0: 4000 });
        }),
        // Steam vent bursting: pressurised hiss, brown puff and a low thump.
        rm_vent: wrap(function (b, t, g, sr) {
            b.n(t, 0.5, g * 0.75, { color: 'pink', type: 'highpass', f0: 1500 * sr, q: 0.6, atk: 0.02 });
            b.n(t, 0.35, g * 0.5, { color: 'brown', type: 'lowpass', f0: 420, f1: 180, q: 0.8, atk: 0.01 });
            b.o('sine', 85 * sr, 48 * sr, t, 0.2, g * 0.4, { atk: 0.004 });
        }),
        // Dart shot: plucked string twang, crackle snap and a short air thwip.
        rm_dart: wrap(function (b, t, g, sr, o) {
            if (!b.model('lute', t, (330 * sr) / 261.6256, g * 0.9, 0.4)) b.o('sawtooth', 340 * sr, 320 * sr, t, 0.25, g * 0.2, { filter: 'lowpass', ff: 1900, q: 1.1 });
            b.n(t, 0.014, g * 0.3, { color: 'crackle', type: 'highpass', f0: 2600 });
            b.n(t + 0.01, 0.1, g * 0.5, { color: 'pink', f0: 1800 * sr, f1: 5200 * sr, q: 1.4, atk: 0.01 });
            b.o('sine', 760 * sr, 520 * sr, t, 0.12, g * 0.12, { atk: 0.004 });
        }),
        // Brazier catching fire: a soft low-passed fwoomp swelling over about 110 ms, a breathy bed, 24 sparse irregular crackles (1-6 kHz) over a second, a quiet bell for the order step far behind the flame.
        rm_ignite: wrap(function (b, t, g, sr, o) {
            const out = b.sat('warm');
            b.n(t, 0.5, g * 0.7, { color: 'brown', type: 'lowpass', f0: 1400, f1: 350, q: 0.7, atk: 0.11, to: out });
            b.n(t, 1.1, g * 0.18, { color: 'pink', type: 'bandpass', f0: 1800, f1: 900, q: 0.6, atk: 0.15, to: out });
            for (let i = 0; i < 24; i++) {
                const at = t + 0.1 + Math.pow(jit(o, i), 1.2) * 1.0;
                b.n(at, 0.012 + jit(o, i + 30) * 0.014, g * (0.2 + jit(o, i + 60) * 0.5) * (1 - (at - t) / 1.3), { color: 'crackle', type: 'bandpass', f0: 1000 + jit(o, i + 90) * 5000, q: 1.5, to: out });
            }
            b.bell(660 * sr, t + 0.12, 0.5, g * 0.08);
        }),
        // Brazier going out: a breathy hiss falling away with a low sigh.
        rm_snuff: wrap(function (b, t, g, sr) {
            b.n(t, 0.35, g * 0.5, { color: 'pink', type: 'lowpass', f0: 3000, f1: 260, q: 0.8, atk: 0.01 });
            b.n(t, 0.3, g * 0.35, { color: 'white', type: 'highpass', f0: 4200 * sr, q: 0.6, atk: 0.03 });
            b.o('sine', 300 * sr, 110 * sr, t, 0.2, g * 0.15, { atk: 0.01 });
        }),
        // Wrong order: two falling bells, a sinking tone, a stone rumble and fizzling crackles.
        rm_fail: wrap(function (b, t, g, sr, o) {
            b.bell(392 * sr, t, 0.7, g * 0.26);
            b.bell(311 * sr, t + 0.15, 0.8, g * 0.28);
            b.o('sine', 280 * sr, 90 * sr, t, 0.5, g * 0.22, { atk: 0.02 });
            b.n(t, 0.4, g * 0.35, { color: 'brown', type: 'lowpass', f0: 600, f1: 150, q: 1, atk: 0.02 });
            for (let i = 0; i < 5; i++) b.n(t + 0.05 + i * 0.07 + jit(o, i) * 0.03, 0.02, g * 0.25, { color: 'crackle', type: 'highpass', f0: 3000 + jit(o, i + 9) * 3000, q: 1 });
        }),
        // Puzzle solved: lydian harp and bell arpeggio into a glass chord, over a deep stone latch.
        rm_done: wrap(function (b, t, g, sr, o) {
            [523.25, 587.33, 739.99, 783.99, 987.77].forEach(function (f, i) {
                const tt = t + i * 0.085;
                b.bell(f * sr, tt, 0.6, g * 0.3);
                b.model('harp', tt, (f * 0.5 * sr) / 261.6256, g * 0.28, 0.6);
            });
            [523.25, 783.99, 1174.7].forEach(function (f, i) { b.model('glass', t + 0.45 + i * 0.02, (f * sr) / 523.251, g * 0.18, 0.8); });
            b.o('sine', 90 * sr, 55 * sr, t, 0.2, g * 0.4, { atk: 0.004 });
            b.n(t, 0.015, g * 0.4, { color: 'crackle', type: 'highpass', f0: 2400 });
        }),
        // A cracked wall taking a hit: dry chip, short thud and a few pebbles.
        rm_chip: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.015, g * 0.9, { color: 'crackle', type: 'highpass', f0: 1800 });
            b.o('sine', 150 * sr, 80 * sr, t, 0.09, g * 0.65, { atk: 0.002 });
            scatter(b, o, t + 0.02, 5, 0.15, g * 0.5, 1200, 3500, 3, 30, 1.3);
            b.n(t, 0.1, g * 0.3, { color: 'brown', type: 'lowpass', f0: 700, f1: 250, q: 0.8, atk: 0.004 });
        }),
        // A cracked wall collapsing: sub boom, long rubble roar, falling rocks and a dust hiss.
        rm_collapse: wrap(function (b, t, g, sr, o) {
            const out = b.sat('warm');
            b.o('sine', 90 * sr, 32 * sr, t, 0.55, g * 0.8, { atk: 0.004, to: out });
            b.n(t, 0.7, g * 0.7, { color: 'brown', type: 'lowpass', f0: 1600, f1: 180, q: 0.7, atk: 0.01, to: out });
            b.n(t, 0.03, g * 0.8, { color: 'crackle', type: 'highpass', f0: 2200, to: out });
            scatter(b, o, t + 0.03, 16, 0.6, g * 0.6, 900, 4200, 2.5, 0, 1.2);
            for (let i = 0; i < 3; i++) b.o('sine', 700 + jit(o, i + 50) * 800, 0, t + 0.12 + i * 0.13, 0.05, g * 0.12, { atk: 0.001 });
            b.n(t + 0.05, 0.5, g * 0.3, { color: 'pink', f0: 1200, f1: 400, q: 1.2, atk: 0.05, to: out });
        }),
        // Small key: coin-like clink, three ring partials and a second clink.
        rm_key: wrap(function (b, t, g, sr, o) {
            const s = b.dsp && b.dsp.getSample(b.ac, 'coin');
            b.model('coin', t, (2100 * sr) / (s ? s.refHz : 1318.51), g * 0.5, 0.35);
            [2480, 3350, 4720].forEach(function (f, i) {
                b.o('sine', f * sr * (1 + (jit(o, i) - 0.5) * 0.01), 0, t + 0.01 + i * 0.025, 0.2 - i * 0.04, g * 0.18 / (1 + i * 0.4), { atk: 0.001 });
            });
            b.model('coin', t + 0.07, (1500 * sr) / (s ? s.refHz : 1318.51), g * 0.3, 0.3);
            b.n(t, 0.01, g * 0.4, { color: 'crackle', type: 'highpass', f0: 4000 });
        }),
        // Lock: two ratchet clicks, the bolt clunking back and a tiny release ting.
        rm_unlock: wrap(function (b, t, g, sr) {
            [0, 0.09].forEach(function (dt) {
                b.n(t + dt, 0.012, g * 0.55, { color: 'crackle', f0: 2600 * sr, q: 3.5 });
                b.o('square', 1400 * sr, 0, t + dt, 0.012, g * 0.1);
            });
            b.o('sine', 100 * sr, 50 * sr, t + 0.19, 0.14, g * 0.7, { atk: 0.003 });
            b.model('anvil', t + 0.19, 0.7 * sr, g * 0.3, 0.3);
            b.n(t + 0.19, 0.014, g * 0.55, { color: 'crackle', type: 'highpass', f0: 2200 });
            b.bell(1568 * sr, t + 0.27, 0.4, g * 0.1);
        }),
        // Paper turning: soft broadband flip, a settling flap and a dull cover thump.
        rm_paper: wrap(function (b, t, g, sr) { paperFlip(b, t, g, sr); }),
        // Journal page found: paper flip followed by three rising celesta notes.
        rm_page: wrap(function (b, t, g, sr) {
            paperFlip(b, t, g * 0.7, sr);
            [659.25, 783.99, 1046.5].forEach(function (f, i) {
                b.bell(f * sr, t + 0.14 + i * 0.09, i === 2 ? 0.8 : 0.45, g * 0.32);
                b.model('harp', t + 0.14 + i * 0.09, (f * 0.5 * sr) / 261.6256, g * 0.22, 0.5);
            });
        }),
        // Task board: wooden knock, a pin tink and paper rustle.
        rm_board: wrap(function (b, t, g, sr, o) {
            b.o('sine', 200 * sr, 110 * sr, t, 0.06, g * 0.6, { atk: 0.002 });
            b.n(t, 0.03, g * 0.4, { color: 'pink', f0: 900 * sr, q: 2 });
            b.o('sine', 3300 * sr, 0, t + 0.04, 0.08, g * 0.1, { atk: 0.001 });
            paperFlip(b, t + 0.06, g * 0.55, sr * 0.9);
            scatter(b, o, t + 0.06, 3, 0.15, g * 0.2, 2500, 5000, 1.5, 40, 1);
        }),
        // Dice shaken in a leather cup, then tumbling on wood: cloth rustle, irregular muffled noise knocks, then two dice with 4 shrinking bounces each, out of sync.
        rm_dice: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.5, g * 0.1, { color: 'pink', type: 'bandpass', f0: 2500, q: 0.6, atk: 0.08 });
            b.n(t, 0.5, g * 0.12, { color: 'brown', type: 'bandpass', f0: 500, q: 0.8, atk: 0.05 });
            let pos = 0;
            for (let i = 0; i < 11; i++) {
                pos += 0.02 + jit(o, i) * 0.07;
                if (pos > 0.5) break;
                b.n(t + pos, 0.02, g * (0.3 + jit(o, i + 20) * 0.6), { color: 'pink', type: 'bandpass', f0: (600 + jit(o, i + 40) * 900) * sr, q: 2, atk: 0.002 });
            }
            [[0.56, 0.13], [0.6, 0.1]].forEach(function (die, d) {
                let at = t + die[0];
                let gap = die[1];
                for (let k = 0; k < 4; k++) {
                    const lvl = 0.9 - k * 0.2;
                    b.n(at, 0.03, g * lvl * 0.6, { color: 'pink', type: 'bandpass', f0: (1500 + jit(o, k + d * 8 + 60) * 1500) * sr, q: 2.5, atk: 0.002 });
                    b.n(at, 0.05, g * lvl * 0.15, { color: 'brown', type: 'bandpass', f0: 300, q: 2, atk: 0.003 });
                    at += gap * (0.9 + jit(o, k + d * 8 + 80) * 0.2);
                    gap *= 0.6;
                }
            });
        }),
        // Task done: wooden stamp thump with a paper tick, then a two-note bell.
        rm_stamp: wrap(function (b, t, g, sr) {
            b.o('sine', 130 * sr, 68 * sr, t, 0.16, g * 0.7, { atk: 0.003 });
            b.n(t, 0.02, g * 0.5, { color: 'crackle', f0: 1500, q: 1.2 });
            b.n(t + 0.01, 0.1, g * 0.3, { color: 'brown', type: 'lowpass', f0: 600, q: 0.8 });
            [659.25, 987.77].forEach(function (f, i) {
                b.bell(f * sr, t + 0.14 + i * 0.09, i ? 0.7 : 0.4, g * 0.32);
                b.model('harp', t + 0.14 + i * 0.09, (f * 0.5 * sr) / 261.6256, g * 0.22, 0.5);
            });
        }),
        // Reward claimed: a pouring run of coin clinks, a bell on top and a cloth rustle.
        rm_coins: wrap(function (b, t, g, sr, o) {
            [1.45, 1.7, 1.25, 1.9, 1.5, 2.1].forEach(function (r, i) {
                coin(b, t + i * 0.055 + jit(o, i) * 0.015, r * sr * 0.8, g * (0.42 - i * 0.03), 0.3);
            });
            b.bell(1318.5 * sr, t + 0.34, 0.5, g * 0.22);
            b.n(t, 0.2, g * 0.15, { color: 'brown', type: 'lowpass', f0: 900, q: 0.8, atk: 0.02 });
        })
    };
    Object.keys(GUARDIAN_VOICES).forEach(function (k) {
        SYNTHS['rm_v_' + k] = GUARDIAN_VOICES[k];
        SYNTHS['rm_t_' + k] = GUARDIAN_TELLS[k];
    });
    Object.keys(SYNTHS).forEach(function (k) { Sfx.SYNTHS[k] = SYNTHS[k]; });

    // ---- guardian cue tables ----
    // pitch of each guardian (multiplies every layer's sr), its signature move class and the moves it is known to use.
    const GUARDIANS = {
        brood: { pitch: 0.8, main: 'slam', moves: { slam: 'slam', spawn: 'summon', split: 'summon' } },
        weaver: { pitch: 1.3, main: 'shot', moves: { venom: 'shot', lunge: 'charge', web: 'web' } },
        tombward: { pitch: 0.7, main: 'slam', moves: { slam: 'slam', march: 'slam', raise: 'summon' } },
        frostheart: { pitch: 1.2, main: 'shot', moves: { ring: 'burst', triple: 'shot', blink: 'blink' } },
        boilerward: { pitch: 0.85, main: 'charge', moves: { charge: 'charge', cross: 'burst' } },
        counter: { pitch: 1, main: 'burst', moves: { clock: 'burst', beam: 'beam', summon: 'summon' } }
    };
    // Attack layers per move class, built from existing recipes; the guardian voice and pitch are added per guardian.
    const ATTACK_CLASSES = {
        slam: { synth: 'impact', sg: 0.9, sr: 0.7, also: [{ synth: 'boom', sg: 0.7, sr: 1.1 }] },
        shot: { synth: 'whoosh', sg: 0.7, sr: 1.4, also: [{ synth: 'puff', sg: 0.5, sr: 1.6, delay: 0.02 }, { synth: 'ghost', sg: 0.3, sr: 1.4 }] },
        charge: { synth: 'whoosh', sg: 0.8, sr: 0.5, also: [{ synth: 'stomp', sg: 0.7, sr: 0.7, delay: 0.05 }, { synth: 'stomp', sg: 0.5, sr: 0.8, delay: 0.2 }] },
        summon: { synth: 'shimmer', sg: 0.5, sr: 0.6, also: [{ synth: 'hum', sg: 0.4, sr: 0.6 }, { synth: 'ghost', sg: 0.3, sr: 0.8 }] },
        blink: { synth: 'magicping', sg: 0.55, sr: 1.8, also: [{ synth: 'ghost', sg: 0.4, sr: 1.3 }, { synth: 'poof', sg: 0.4, sr: 1, delay: 0.05 }] },
        beam: { synth: 'zap', sg: 0.8, sr: 0.6, also: [{ synth: 'hiss', sg: 0.5 }, { synth: 'hum', sg: 0.4, sr: 1.2 }] },
        web: { synth: 'whoosh', sg: 0.7, sr: 1.8, also: [{ synth: 'hiss', sg: 0.5, sr: 1.5 }, { synth: 'squelch', sg: 0.35, sr: 1.2, delay: 0.15 }] },
        burst: { synth: 'fwoosh', sg: 0.7, sr: 0.8, also: [{ synth: 'puff', sg: 0.6, sr: 0.6 }, { synth: 'crack', sg: 0.3, sr: 1.2 }] }
    };
    const CLASS_VOL = { slam: 0.3, shot: 3.6, charge: 1.2, summon: 1, blink: 1.8, beam: 1, web: 3.6, burst: 1 };
    const TELL_VOL = { brood: 0.35, weaver: 1.5, tombward: 0.7, frostheart: 0.9, boilerward: 0.8, counter: 1.2 };
    const TELL_SR = { slam: 0.85, shot: 1.15, charge: 0.9, summon: 1, blink: 1.3, beam: 1.2, web: 1.1, burst: 1 };

    function attackFor(kind, cls) {
        const g = GUARDIANS[kind];
        const c = ATTACK_CLASSES[cls];
        const vol = CLASS_VOL[cls];
        const also = c.also.map(function (a) { return Object.assign({}, a, { sg: a.sg * vol, sr: (a.sr || 1) * g.pitch }); });
        also.push({ synth: 'rm_v_' + kind, sg: 0.5 * vol, sr: 1 });
        return { synth: c.synth, sg: c.sg * vol, sr: c.sr * g.pitch, also: also };
    }
    function tellFor(kind, cls) {
        return { synth: 'rm_t_' + kind, sg: 0.6 * TELL_VOL[kind], sr: TELL_SR[cls] || 1 };
    }
    function patternMap(make) {
        const map = {};
        Object.keys(GUARDIANS).forEach(function (kind) {
            const g = GUARDIANS[kind];
            map[kind] = make(kind, g.main);
            Object.keys(g.moves).forEach(function (move) { map[kind + '_' + move] = make(kind, g.moves[move]); });
        });
        map['*'] = make('counter', 'burst');
        return map;
    }
    function kindMap(make) {
        const map = {};
        Object.keys(GUARDIANS).forEach(function (kind) { map[kind] = make(kind); });
        map['*'] = make('counter');
        return map;
    }
    const DEATHS = {
        brood: [{ synth: 'squelch', sg: 0.5, sr: 0.5, delay: 0.3 }, { synth: 'puff', sg: 0.6, sr: 0.5, delay: 0.5 }],
        weaver: [{ synth: 'ice', sg: 0.9, sr: 0.9, delay: 0.25 }, { synth: 'screech', sg: 0.3, sr: 0.7, delay: 0.1 }],
        tombward: [{ synth: 'hollow', sg: 0.6, sr: 0.7, delay: 0.3 }, { synth: 'rm_collapse', sg: 0.6, sr: 0.9, delay: 0.5 }],
        frostheart: [{ synth: 'ice', sg: 1, sr: 1.1, delay: 0.2 }, { synth: 'frostchime', sg: 0.5, sr: 0.9, delay: 0.7 }],
        boilerward: [{ synth: 'steam', sg: 0.6, sr: 0.9, delay: 0.2 }, { synth: 'clank', sg: 0.6, sr: 0.6, delay: 0.5 }],
        counter: [{ synth: 'ratchet', sg: 0.5, sr: 0.7, delay: 0.1 }, { synth: 'frostchime', sg: 0.4, sr: 0.6, delay: 0.7 }, { synth: 'clang', sg: 0.5, sr: 0.5, delay: 0.5 }]
    };

    const CUES = {
        in_break: { sound: false, gain: 0.5, cooldown: 60, synth: 'rm_pot', sg: 0.8, by: { key: 'kind', map: { pot: {}, crate: { synth: 'rm_crate', sr: 1 }, barrel: { synth: 'rm_crate', sr: 0.62 }, '*': {} } } },
        in_push: { sound: false, gain: 0.4, cooldown: 200, synth: 'rm_drag', sg: 0.7 },
        in_slide: { sound: false, gain: 0.3, cooldown: 250, synth: 'rm_slide', sg: 1.8 },
        in_plate: { sound: false, gain: 0.4, cooldown: 120, synth: 'rm_plate', sg: 0.7, by: { key: 'on', map: { true: { sr: 1 }, false: { sr: 1.5 }, '*': {} } } },
        in_lever: { sound: false, gain: 0.5, cooldown: 120, synth: 'rm_lever', sg: 0.7, by: { key: 'on', map: { true: { sr: 1 }, false: { sr: 0.8 }, '*': {} } } },
        in_gate: { sound: false, gain: 0.5, cooldown: 300, synth: 'rm_gate_open', sg: 0.7, by: { key: 'open', map: { true: { synth: 'rm_gate_open' }, false: { synth: 'rm_gate_shut' }, '*': {} } } },
        in_trap_warn: { sound: false, gain: 0.4, cooldown: 150, synth: 'rm_warn_spikes', sg: 0.7, by: { key: 'kind', map: { spikes: { synth: 'rm_warn_spikes', sg: 3.5 }, vent: { synth: 'rm_warn_vent', sg: 1 }, dart: { synth: 'rm_warn_dart', sg: 2.5 }, '*': {} } } },
        in_trap: { sound: false, gain: 0.5, cooldown: 100, synth: 'rm_spike', sg: 0.7, by: { key: 'kind', map: { spikes: { synth: 'rm_spike' }, vent: { synth: 'rm_vent' }, dart: { synth: 'rm_dart', sg: 1.4 }, '*': {} } } },
        in_brazier: { sound: false, gain: 0.4, cooldown: 120, synth: 'rm_ignite', sg: 0.7, scale: { key: 'step', base: 1, per: 0.12, min: 0.8, max: 2 }, by: { key: 'lit', map: { true: {}, false: { synth: 'rm_snuff', sg: 0.7 }, '*': {} } } },
        in_puzzle_fail: { sound: false, gain: 0.5, cooldown: 400, synth: 'rm_fail', sg: 0.7 },
        in_puzzle_done: { sound: false, gain: 0.6, cooldown: 500, synth: 'rm_done', sg: 0.7, also: [{ synth: 'shimmer', sg: 0.2, sr: 1.5, delay: 0.5 }] },
        in_crack: { sound: false, gain: 0.5, cooldown: 90, synth: 'rm_chip', sg: 0.7, by: { key: 'broken', map: { true: { synth: 'rm_collapse' }, false: { synth: 'rm_chip' }, '*': {} } } },
        in_key: { sound: false, gain: 0.5, cooldown: 140, synth: 'rm_key', sg: 0.7 },
        in_unlock: { sound: false, gain: 0.5, cooldown: 200, synth: 'rm_unlock', sg: 0.7 },
        in_page: { sound: false, gain: 0.6, cooldown: 300, synth: 'rm_page', sg: 0.7, by: { key: 'first', map: { true: {}, false: { synth: 'rm_paper', sg: 1.7 }, '*': {} } } },
        in_read: { sound: false, gain: 0.4, cooldown: 200, synth: 'rm_paper', sg: 1.7 },
        in_boss_tell: { sound: false, gain: 0.5, cooldown: 250, by: { key: 'pattern', prefix: true, map: patternMap(tellFor) } },
        in_boss_attack: { sound: false, gain: 0.6, cooldown: 120, by: { key: 'pattern', prefix: true, map: patternMap(attackFor) } },
        in_boss_phase: {
            sound: false, gain: 0.7, cooldown: 400,
            by: {
                key: 'kind',
                map: kindMap(function (kind) {
                    return { synth: 'boom', sg: 0.28, sr: 0.8, also: [{ synth: 'roar', sg: 0.14, sr: GUARDIANS[kind].pitch }, { synth: 'rm_v_' + kind, sg: 0.21 }, { synth: 'crack', sg: 0.1 }] };
                })
            }
        },
        in_boss_die: {
            sound: false, gain: 0.8, cooldown: 600,
            by: {
                key: 'kind',
                map: kindMap(function (kind) {
                    return {
                        synth: 'boom', sg: 0.36, sr: 0.8,
                        also: [{ synth: 'boom', sg: 0.28, sr: 0.6, delay: 0.2 }, { synth: 'rm_v_' + kind, sg: 0.32, delay: 0.05 }, { synth: 'shimmer', sg: 0.2, sr: 0.8, delay: 0.8 }].concat(DEATHS[kind].map(function (a) { return Object.assign({}, a, { sg: a.sg * 0.4 }); }))
                    };
                })
            }
        },
        dice_roll: { sound: false, gain: 0.5, cooldown: 300, synth: 'rm_dice', sg: 1.4 },
        dice_end: {
            sound: false, gain: 0.5, cooldown: 400,
            by: {
                key: 'result',
                map: {
                    win: { synth: 'fanfare3', sg: 0.7, sr: 1.19, also: [{ synth: 'ui_buy', sg: 0.4, delay: 0.2 }] },
                    lose: { synth: 'down', sg: 0.7, sr: 0.7, also: [{ synth: 'knock2', sg: 0.3, sr: 0.8, delay: 0.1 }] },
                    tie: { synth: 'ping', sg: 0.7, sr: 0.9, also: [{ synth: 'ping', sg: 0.5, sr: 0.9, delay: 0.12 }] },
                    '*': { synth: 'ping', sg: 0.7 }
                }
            }
        },
        bounty_done: { sound: false, gain: 0.6, cooldown: 400, synth: 'rm_stamp', sg: 0.7 },
        bounty_claim: { sound: false, gain: 0.6, cooldown: 400, synth: 'rm_coins', sg: 1.7 }
    };
    Object.keys(CUES).forEach(function (k) { Sfx.CUES[k] = CUES[k]; });

    // Exported for the audio test and for tuning: the guardian table (pitch, signature move class, known moves).
    Sfx.ROOM_GUARDIANS = GUARDIANS;
    // Shared helpers for sfx-deeps.js (recipe wrapper, grit scatter, seeded jitter).
    Sfx.ROOM_TOOLS = { wrap: wrap, scatter: scatter, jit: jit };
})(typeof window !== 'undefined' ? window : globalThis);
