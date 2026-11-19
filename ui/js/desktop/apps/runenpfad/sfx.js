/* Runenpfad synthesized sound recipes (global RunenpfadSfx). DOM-free and deterministic: variation comes from o.v only. */
(function (root) {
    'use strict';

    // Recipe signature: fn(ac, noise, dest, t, o) -> [anchorSource, nodesToDisconnect].
    // o = { gain, sr (pitch scale), v ([0,1) variation), short, count, freq }.
    function builder(ac, noise, dest, t) {
        const nodes = [];
        let end = t + 0.05;
        let noiseAt = 0;
        const b = {
            ac: ac, noise: noise, dest: dest,
            keep: function (n) { nodes.push(n); return n; },
            env: function (peak, t0, atk, dur, to) {
                const g = b.keep(ac.createGain());
                g.gain.setValueAtTime(0.0001, t0);
                g.gain.linearRampToValueAtTime(Math.max(0.0002, peak), t0 + atk);
                g.gain.exponentialRampToValueAtTime(0.0001, t0 + dur);
                g.connect(to || dest);
                end = Math.max(end, t0 + dur + 0.04);
                return g;
            },
            // Oscillator with an exponential pitch sweep, optional filter, and an exponential envelope.
            o: function (type, f0, f1, t0, dur, peak, opt) {
                opt = opt || {};
                const os = b.keep(ac.createOscillator());
                os.type = type;
                os.frequency.setValueAtTime(f0, t0);
                if (f1 && f1 !== f0) os.frequency.exponentialRampToValueAtTime(Math.max(20, f1), t0 + dur);
                const g = b.env(peak, t0, opt.atk || 0.004, dur);
                if (opt.filter) {
                    const f = b.keep(ac.createBiquadFilter());
                    f.type = opt.filter;
                    f.frequency.value = opt.ff || 1000;
                    f.Q.value = opt.q || 0.7;
                    os.connect(f);
                    f.connect(g);
                } else {
                    os.connect(g);
                }
                os.start(t0);
                os.stop(t0 + dur + 0.02);
                return { osc: os, g: g };
            },
            // Filtered noise burst with an optional filter sweep f0 -> f1.
            n: function (t0, dur, peak, opt) {
                opt = opt || {};
                const src = b.keep(ac.createBufferSource());
                src.buffer = noise;
                src.loop = true;
                const f = b.keep(ac.createBiquadFilter());
                f.type = opt.type || 'bandpass';
                f.Q.value = opt.q || 0.8;
                f.frequency.setValueAtTime(opt.f0 || 1500, t0);
                if (opt.f1 && opt.f1 !== opt.f0) f.frequency.exponentialRampToValueAtTime(Math.max(30, opt.f1), t0 + dur);
                const g = b.env(peak, t0, opt.atk || 0.003, dur);
                src.connect(f);
                f.connect(g);
                noiseAt = (noiseAt + 0.137) % 0.5;
                src.start(t0, noiseAt);
                src.stop(t0 + dur + 0.02);
                return g;
            },
            lfo: function (param, rate, depth, t0, dur) {
                const l = b.keep(ac.createOscillator());
                const lg = b.keep(ac.createGain());
                l.frequency.value = rate;
                lg.gain.value = depth;
                l.connect(lg);
                lg.connect(param);
                l.start(t0);
                l.stop(t0 + dur + 0.02);
            },
            // FM bell: sine carrier, inharmonic modulator with a decaying index.
            bell: function (f, t0, dur, peak) {
                const car = b.keep(ac.createOscillator());
                const mod = b.keep(ac.createOscillator());
                const mg = b.keep(ac.createGain());
                car.frequency.value = f;
                mod.frequency.value = f * 3.51;
                mg.gain.setValueAtTime(f * 2.4, t0);
                mg.gain.exponentialRampToValueAtTime(f * 0.05, t0 + dur * 0.8);
                mod.connect(mg);
                mg.connect(car.frequency);
                const g = b.env(peak, t0, 0.003, dur);
                car.connect(g);
                car.start(t0);
                mod.start(t0);
                car.stop(t0 + dur + 0.02);
                mod.stop(t0 + dur + 0.02);
            },
            // Short noise grain (1-6 ms) feeding the given nodes (or the output when none).
            grain: function (t0, dur, peak, targets, off) {
                const src = b.keep(ac.createBufferSource());
                src.buffer = noise;
                src.loop = true;
                const g = b.keep(ac.createGain());
                g.gain.setValueAtTime(Math.max(0.0002, peak), t0);
                g.gain.exponentialRampToValueAtTime(0.0001, t0 + dur);
                src.connect(g);
                (targets && targets.length ? targets : [dest]).forEach(function (n) { g.connect(n); });
                src.start(t0, off % 0.5);
                src.stop(t0 + dur + 0.005);
                end = Math.max(end, t0 + dur + 0.04);
            },
            // High-Q band-pass resonator that rings out into `to` (default dest).
            reso: function (f, q, t0, decay, gain, to) {
                const bp = b.keep(ac.createBiquadFilter());
                bp.type = 'bandpass';
                bp.frequency.value = f;
                bp.Q.value = q;
                const g = b.env(gain, t0, 0.002, decay, to);
                bp.connect(g);
                return bp;
            },
            touch: function (tEnd) { end = Math.max(end, tEnd + 0.04); },
            done: function () {
                const a = b.keep(ac.createOscillator());
                const z = b.keep(ac.createGain());
                z.gain.value = 0;
                a.connect(z);
                z.connect(dest);
                a.start(t);
                a.stop(end);
                return [a, nodes];
            }
        };
        return b;
    }

    // Deterministic pseudo-random in [0,1) from the cue variation and an index.
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

    const SYNTHS = {
        // Knight: fast blade swish (band-passed noise sweep 500 -> 3500 -> 1500 Hz) with a faint metallic shimmer.
        sword: wrap(function (b, t, g, sr, o) {
            const src = b.keep(b.ac.createBufferSource());
            src.buffer = b.noise;
            src.loop = true;
            const f = b.keep(b.ac.createBiquadFilter());
            f.type = 'bandpass';
            f.Q.value = 1.2;
            f.frequency.setValueAtTime(500 * sr, t);
            f.frequency.exponentialRampToValueAtTime(3500 * sr, t + 0.07);
            f.frequency.exponentialRampToValueAtTime(1500 * sr, t + 0.22);
            const env = b.keep(b.ac.createGain());
            env.gain.setValueAtTime(0.0001, t);
            env.gain.linearRampToValueAtTime(g * 0.55, t + 0.025);
            env.gain.linearRampToValueAtTime(g * 0.95, t + 0.07);
            env.gain.exponentialRampToValueAtTime(0.0001, t + 0.22);
            src.connect(f);
            f.connect(env);
            env.connect(b.dest);
            src.start(t, 0.11 + o.v * 0.3);
            src.stop(t + 0.24);
            b.touch(t + 0.24);
            [3600, 4700, 5900].forEach(function (hz, i) {
                b.o('sine', hz * sr * (0.97 + jit(o, i) * 0.06), 0, t + 0.07, 0.15, g * 0.19 / (1 + i * 0.4), { atk: 0.02 });
            });
        }),
        // Ranger: plucked string twang plus the arrow whoosh.
        twang: wrap(function (b, t, g, sr, o) {
            const f = 196 * sr * (1 + o.v * 0.08);
            b.o('sawtooth', f * 1.03, f, t, 0.34, g * 0.34, { filter: 'lowpass', ff: 1900, q: 1.1 });
            b.o('triangle', f * 2, f * 2, t, 0.2, g * 0.14);
            b.n(t + 0.01, 0.16, g * 0.4, { f0: 1200 * sr, f1: 3600 * sr, q: 1.2, atk: 0.04 });
        }),
        // Layered impact: sharp transient click + low body thump.
        impact: wrap(function (b, t, g, sr) {
            b.n(t, 0.014, g * 0.9, { type: 'highpass', f0: 3500, q: 0.7 });
            b.o('sine', 150 * sr, 48 * sr, t, 0.22, g * 0.9, { atk: 0.003 });
            b.n(t + 0.004, 0.1, g * 0.3, { type: 'lowpass', f0: 1500, f1: 300, q: 0.7 });
        }),
        // Explosion body: sub-bass drop + low rumble noise + crack.
        boom: wrap(function (b, t, g, sr) {
            b.o('sine', 85 * sr, 26 * sr, t, 0.85, g, { atk: 0.008 });
            b.o('sine', 55 * sr, 30 * sr, t + 0.08, 0.9, g * 0.45);
            b.n(t, 0.55, g * 0.65, { type: 'lowpass', f0: 1400, f1: 120, q: 0.7, atk: 0.004 });
            b.n(t, 0.03, g * 0.9, { type: 'highpass', f0: 2500, q: 0.7 });
        }),
        // Magic shimmer: a cluster of FM bells.
        shimmer: wrap(function (b, t, g, sr, o) {
            [1, 1.5, 2, 3, 4].forEach(function (m, i) {
                b.bell(740 * sr * m * (1 + (jit(o, i) - 0.5) * 0.01), t + i * 0.055, 0.7, g * 0.22 / (1 + i * 0.25));
            });
        }),
        // Glass shatter by modal synthesis: noise excites high-Q resonators; no oscillators involved.
        ice: wrap(function (b, t, g, sr, o) {
            const modes = [2300, 3700, 5100, 6800, 8900, 11000].map(function (f, i) {
                return b.reso(f * sr * (0.97 + jit(o, i) * 0.06), 25 + jit(o, i + 10) * 35, t, 0.08 + jit(o, i + 20) * 0.27, g * 9);
            });
            b.grain(t, 0.03, g * 0.9, null, 0.05);
            b.grain(t, 0.03, g * 1.2, modes, 0.2);
            b.n(t, 0.06, g * 0.4, { type: 'lowpass', f0: 800, q: 0.7, atk: 0.004 });
            for (let i = 0; i < 36; i++) {
                const at = Math.pow(jit(o, i + 40), 1.8) * 0.45;
                const lvl = g * 0.8 * (1 - at / 0.6);
                const a1 = modes[Math.floor(jit(o, i + 80) * 6)];
                const tg = jit(o, i + 120) > 0.5 ? [a1, modes[Math.floor(jit(o, i + 160) * 6)]] : [a1];
                b.grain(t + 0.01 + at, 0.001 + jit(o, i + 200) * 0.005, lvl, tg, jit(o, i + 240) * 0.4);
            }
        }),
        // Lightning: hard noise crack plus a low rumble.
        crack: wrap(function (b, t, g) {
            b.n(t, 0.06, g, { type: 'highpass', f0: 1200, q: 0.8 });
            b.n(t, 0.28, g * 0.5, { f0: 3200, f1: 400, q: 0.8 });
            b.n(t + 0.03, 0.55, g * 0.4, { type: 'lowpass', f0: 320, q: 0.7, atk: 0.04 });
        }),
        // Beast roar: two formant-filtered noise bands plus a falling sawtooth growl.
        roar: wrap(function (b, t, g, sr, o) {
            const d = o.short ? 0.45 : 0.95;
            const grr = b.o('sawtooth', 150 * sr, 58 * sr, t, d, g * 0.45, { filter: 'lowpass', ff: 900, q: 1, atk: 0.06 });
            b.o('sawtooth', 154 * sr, 60 * sr, t, d, g * 0.25, { filter: 'lowpass', ff: 700, q: 1, atk: 0.06 });
            b.lfo(grr.g.gain, 28, g * 0.12, t, d);
            b.n(t, d, g * 0.55, { f0: 720 * sr, f1: 420 * sr, q: 5, atk: 0.05 });
            b.n(t, d, g * 0.4, { f0: 1250 * sr, f1: 800 * sr, q: 6, atk: 0.05 });
        }),
        // Machine voices: servo whine, rotor thrum, metallic clank.
        servo: wrap(function (b, t, g, sr, o) {
            const d = o.short ? 0.3 : 0.6;
            b.o('sawtooth', 380 * sr, 1150 * sr, t, d, g * 0.3, { filter: 'bandpass', ff: 1500, q: 4, atk: 0.05 });
            b.o('square', 95 * sr, 210 * sr, t, d, g * 0.14, { filter: 'lowpass', ff: 500, atk: 0.05 });
            b.n(t, d, g * 0.14, { f0: 4000, q: 6, atk: 0.05 });
        }),
        rotor: wrap(function (b, t, g, sr, o) {
            const d = o.short ? 0.4 : 0.8;
            const r = b.o('square', 46 * sr, 58 * sr, t, d, g * 0.38, { filter: 'lowpass', ff: 420, atk: 0.1 });
            b.lfo(r.g.gain, 22, g * 0.16, t, d);
            b.o('triangle', 260 * sr, 380 * sr, t, d, g * 0.1, { atk: 0.12 });
            b.n(t, d, g * 0.12, { f0: 1800, f1: 900, q: 1.5, atk: 0.1 });
        }),
        clank: wrap(function (b, t, g, sr) {
            [310, 517, 830, 1210].forEach(function (f, i) {
                b.o('square', f * sr, f * sr, t, 0.32 - i * 0.04, g * 0.13, { filter: 'bandpass', ff: f * sr * 1.5, q: 5 });
            });
            b.n(t, 0.015, g * 0.6, { type: 'highpass', f0: 3000 });
        }),
        // Enemy block: metal clang with inharmonic partials.
        clang: wrap(function (b, t, g, sr) {
            [[520, 0.4], [1330, 0.3], [2150, 0.2], [2900, 0.12]].forEach(function (p, i) {
                b.o(i % 2 ? 'sine' : 'triangle', p[0] * sr, p[0] * sr * 0.995, t, 0.4 - i * 0.05, g * p[1]);
            });
            b.n(t, 0.02, g * 0.6, { type: 'highpass', f0: 3200 });
        }),
        // Ghostly whoosh: direction follows the pitch scale (>= 1 phases in, < 1 phases out).
        ghost: wrap(function (b, t, g, sr) {
            const up = sr >= 1;
            b.n(t, 0.6, g * 0.5, { f0: up ? 300 : 1500, f1: up ? 1500 : 300, q: 3, atk: 0.25 });
            const v = b.o('sine', (up ? 320 : 640), (up ? 640 : 320), t, 0.6, g * 0.18, { atk: 0.25 });
            b.lfo(v.osc.frequency, 6, 14, t, 0.6);
        }),
        // Clockwork ratchet: accelerating ticks.
        ratchet: wrap(function (b, t, g, sr) {
            for (let i = 0; i < 10; i++) {
                const tt = t + 0.6 * (1 - Math.pow(1 - i / 9, 1.6));
                b.n(tt, 0.014, g * 0.5, { f0: (2800 + i * 120) * sr, q: 3 });
                b.o('square', (900 + i * 60) * sr, 0, tt, 0.018, g * 0.14);
            }
        }),
        // Wet spore puff.
        puff: wrap(function (b, t, g, sr) {
            b.n(t, 0.2, g * 0.6, { type: 'lowpass', f0: 1800, f1: 400, q: 0.8, atk: 0.01 });
            b.o('sine', 320 * sr, 140 * sr, t, 0.14, g * 0.4, { atk: 0.006 });
            b.n(t + 0.04, 0.1, g * 0.28, { f0: 900, q: 5 });
        }),
        // Crystal bat screech.
        screech: wrap(function (b, t, g, sr) {
            const s = b.o('sawtooth', 2600 * sr, 4300 * sr, t, 0.36, g * 0.3, { filter: 'bandpass', ff: 3500, q: 2.5, atk: 0.02 });
            b.lfo(s.osc.frequency, 38, 130, t, 0.36);
            b.o('square', 3900 * sr, 6100 * sr, t + 0.02, 0.3, g * 0.1, { filter: 'highpass', ff: 3000 });
        }),
        // Stomp: springy thump.
        stomp: wrap(function (b, t, g, sr) {
            b.o('sine', 210 * sr, 70 * sr, t, 0.17, g * 0.9);
            b.o('sine', 280 * sr, 620 * sr, t + 0.02, 0.13, g * 0.24);
            b.n(t, 0.012, g * 0.4, { type: 'highpass', f0: 3000 });
        }),
        // Chest: stick-slip wood creak. A jittered 35-70 Hz impulse train with a gliding period rings three wooden resonances.
        creak: wrap(function (b, t, g, sr, o) {
            const dur = 0.6;
            const master = b.env(g * 1.4, t, 0.04, dur);
            const bank = [[380, 7], [900, 9], [1600, 6]].map(function (m, i) {
                const bp = b.keep(b.ac.createBiquadFilter());
                bp.type = 'bandpass';
                bp.frequency.value = m[0] * sr * (0.96 + jit(o, i) * 0.08);
                bp.Q.value = m[1];
                const bg = b.keep(b.ac.createGain());
                bg.gain.value = 4 - i;
                bp.connect(bg);
                bg.connect(master);
                return bp;
            });
            let tt = t;
            for (let i = 0; tt < t + dur; i++) {
                const rate = (35 + jit(o, i) * 35) * (1 + 0.5 * (tt - t) / dur);
                b.grain(tt, 0.004, 0.5 + jit(o, i + 50) * 0.5, bank, jit(o, i + 90) * 0.4);
                tt += 1 / rate;
            }
        }),
        // Rune pickups.
        fireup: wrap(function (b, t, g, sr) {
            b.n(t, 0.5, g * 0.6, { f0: 400 * sr, f1: 2600 * sr, q: 1.5, atk: 0.15 });
            b.o('sawtooth', 160 * sr, 480 * sr, t, 0.45, g * 0.16, { filter: 'lowpass', ff: 1400, atk: 0.12 });
        }),
        frostchime: wrap(function (b, t, g, sr) {
            [1, 1.25, 1.5, 2].forEach(function (m, i) { b.bell(1180 * sr * m, t + i * 0.07, 0.8, g * 0.2); });
        }),
        crackle: wrap(function (b, t, g, sr, o) {
            for (let i = 0; i < 9; i++) {
                const tt = t + i * 0.035 + jit(o, i) * 0.03;
                b.n(tt, 0.02, g * (0.3 + jit(o, i + 9) * 0.4), { type: 'highpass', f0: 2500 + jit(o, i + 3) * 3000, q: 1 });
            }
            b.o('sawtooth', 500 * sr, 130 * sr, t, 0.3, g * 0.15, { filter: 'bandpass', ff: 1400, q: 3 });
        }),
        hum: wrap(function (b, t, g, sr) {
            b.o('sine', 220 * sr, 220 * sr, t, 0.8, g * 0.3, { atk: 0.25 });
            b.o('sine', 225 * sr, 225 * sr, t, 0.8, g * 0.3, { atk: 0.25 });
            b.o('triangle', 440 * sr, 660 * sr, t, 0.6, g * 0.12, { atk: 0.25 });
        }),
        // Rune expire: soft down-sweep.
        down: wrap(function (b, t, g, sr) {
            b.o('sine', 700 * sr, 200 * sr, t, 0.5, g * 0.3, { atk: 0.02 });
            b.o('triangle', 1050 * sr, 300 * sr, t, 0.45, g * 0.12, { atk: 0.02 });
        }),
        // Geyser: sustained high-passed hiss (slow attack, long release) over a wobbling gurgle rumble.
        steam: wrap(function (b, t, g, sr) {
            b.n(t, 1.2, g * 0.75, { type: 'highpass', f0: 1500 * sr, q: 0.6, atk: 0.08 });
            const r = b.n(t, 1.2, g * 0.55, { type: 'lowpass', f0: 150, q: 0.8, atk: 0.1 });
            b.lfo(r.gain, 6, g * 0.2, t, 1.2);
        }),
        // Creature death: short noise poof (low-pass 3 kHz -> 300 Hz).
        poof: wrap(function (b, t, g) {
            b.n(t, 0.25, g * 0.7, { type: 'lowpass', f0: 3000, f1: 300, q: 0.7, atk: 0.01 });
        }),
        // Switch ping.
        ping: wrap(function (b, t, g, sr) {
            b.o('sine', 1760 * sr, 1760 * sr, t, 0.5, g * 0.4);
            b.o('sine', 2640 * sr, 2640 * sr, t, 0.35, g * 0.2);
            b.o('sine', 3520 * sr, 3520 * sr, t, 0.2, g * 0.1);
        }),
        // Gate: stone grind.
        grind: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.9, g * 0.5, { type: 'lowpass', f0: 500, f1: 250, q: 1.5, atk: 0.08 });
            const w = b.o('sawtooth', 58 * sr, 52 * sr, t, 0.9, g * 0.3, { filter: 'lowpass', ff: 200, atk: 0.08 });
            b.lfo(w.osc.frequency, 7, 3, t, 0.9);
            for (let i = 0; i < 5; i++) b.n(t + 0.12 * i + jit(o, i) * 0.04, 0.025, g * 0.35, { f0: 1200, q: 2 });
        }),
        // Flame jets.
        hiss: wrap(function (b, t, g, sr) {
            b.n(t, 0.45, g * 0.4, { f0: 5000 * sr, q: 0.5, atk: 0.15 });
        }),
        flame: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.85, g * 0.7, { type: 'lowpass', f0: 1800, f1: 500, q: 0.8, atk: 0.04 });
            b.o('sawtooth', 70 * sr, 45 * sr, t, 0.85, g * 0.3, { filter: 'lowpass', ff: 250, atk: 0.05 });
            for (let i = 0; i < 4; i++) b.n(t + 0.1 + i * 0.12 + jit(o, i) * 0.05, 0.03, g * 0.3, { type: 'highpass', f0: 2500 });
        }),
        // Boss telegraph helpers.
        charge: wrap(function (b, t, g, sr) {
            b.o('sawtooth', 140 * sr, 520 * sr, t, 0.55, g * 0.2, { filter: 'lowpass', ff: 1500, atk: 0.3 });
            b.o('sine', 70 * sr, 260 * sr, t, 0.55, g * 0.3, { atk: 0.3 });
        }),
        whistle: wrap(function (b, t, g, sr) {
            b.o('sine', 1800 * sr, 600 * sr, t, 0.4, g * 0.25, { atk: 0.05 });
            b.n(t, 0.4, g * 0.25, { f0: 2400 * sr, f1: 900 * sr, q: 2, atk: 0.06 });
        }),
        // ---- round 4: dragons ----
        // Huge beast roar (~1.6 s): vowel-like formant noise, falling detuned sawtooth growl with a tremolo, sub rumble.
        dragonroar: wrap(function (b, t, g, sr, o) { dragonRoar(b, t, g, sr, o, false); }),
        dragonroar_frost: wrap(function (b, t, g, sr, o) { dragonRoar(b, t, g, sr, o, true); }),
        // Ominous rising swell that sits under the roar.
        dragonwarn: wrap(function (b, t, g, sr) {
            const d = 1.3;
            b.o('sawtooth', 48 * sr, 96 * sr, t, d, g * 0.3, { filter: 'lowpass', ff: 260, atk: 0.9 });
            b.o('sine', 38 * sr, 64 * sr, t, d, g * 0.55, { atk: 0.8 });
            b.n(t, d, g * 0.32, { f0: 260 * sr, f1: 900 * sr, q: 2.5, atk: 0.9 });
            b.o('triangle', 110 * sr, 190 * sr, t + 0.2, d - 0.2, g * 0.1, { atk: 0.8 });
        }),
        // Sustained breath. Ember: band-passed fire roar + crackle + rumble. Frost: hissing icy blast with glassy sparkle.
        dragonbreath: wrap(function (b, t, g, sr, o) {
            const d = 1.4;
            b.n(t, d, g * 0.7, { f0: 1500 * sr, f1: 700 * sr, q: 0.7, atk: 0.12 });
            b.n(t, d, g * 0.45, { type: 'lowpass', f0: 420, f1: 260, q: 0.8, atk: 0.15 });
            b.o('sawtooth', 62 * sr, 44 * sr, t, d, g * 0.3, { filter: 'lowpass', ff: 200, atk: 0.15 });
            for (let i = 0; i < 12; i++) {
                b.n(t + 0.1 + i * 0.1 + jit(o, i) * 0.07, 0.025, g * (0.22 + jit(o, i + 20) * 0.25), { type: 'highpass', f0: 2200 + jit(o, i + 40) * 3000, q: 1 });
            }
        }),
        dragonbreath_frost: wrap(function (b, t, g, sr, o) {
            const d = 1.3;
            b.n(t, d, g * 0.7, { type: 'highpass', f0: 3200 * sr, q: 0.6, atk: 0.1 });
            b.n(t, d, g * 0.4, { f0: 2200 * sr, f1: 1400 * sr, q: 1.5, atk: 0.12 });
            b.o('sawtooth', 90 * sr, 60 * sr, t, d, g * 0.18, { filter: 'lowpass', ff: 220, atk: 0.15 });
            for (let i = 0; i < 4; i++) b.bell(2600 * sr * (1 + jit(o, i) * 0.6), t + 0.12 + i * 0.28, 0.6, g * 0.05);
        }),
        // Giant wing beats: three slow low thumps with swept air under each.
        dragonpass: wrap(function (b, t, g, sr) {
            for (let i = 0; i < 3; i++) {
                const tt = t + i * 0.55;
                const k = 1 - i * 0.18;
                b.n(tt, 0.7, g * 0.6 * k, { f0: 180 * sr, f1: 520 * sr, q: 1.1, atk: 0.28 });
                b.n(tt + 0.25, 0.4, g * 0.4 * k, { type: 'lowpass', f0: 500, f1: 150, q: 0.7, atk: 0.05 });
                b.o('sine', 75 * sr, 36 * sr, tt + 0.28, 0.4, g * 0.75 * k, { atk: 0.02 });
            }
        }),
        // Drakeling spit: small pop plus a fwoosh.
        fwoosh: wrap(function (b, t, g, sr) {
            b.n(t, 0.34, g * 0.6, { f0: 500 * sr, f1: 2200 * sr, q: 1, atk: 0.05 });
            b.o('sine', 260 * sr, 110 * sr, t, 0.1, g * 0.4, { atk: 0.004 });
            b.n(t, 0.015, g * 0.35, { type: 'highpass', f0: 3000 });
        }),
        // ---- round 4: swinging ropes ----
        // Rope creak/stretch (stick-slip grains through a narrow band) plus a soft grab thump.
        swinggrab: wrap(function (b, t, g, sr, o) {
            const master = b.env(g * 1.3, t, 0.03, 0.4);
            const bp = b.keep(b.ac.createBiquadFilter());
            bp.type = 'bandpass';
            bp.frequency.setValueAtTime(700 * sr, t);
            bp.frequency.exponentialRampToValueAtTime(1100 * sr, t + 0.35);
            bp.Q.value = 6;
            bp.connect(master);
            for (let i = 0; i < 10; i++) b.grain(t + i * 0.035 + jit(o, i) * 0.01, 0.005, 0.4 + jit(o, i + 30) * 0.5, [bp], jit(o, i + 60) * 0.4);
            b.o('sine', 150 * sr, 80 * sr, t, 0.14, g * 0.7, { atk: 0.006 });
            b.n(t, 0.08, g * 0.25, { type: 'lowpass', f0: 900, q: 0.7 });
        }),
        // Airy release whoosh; the cue scales pitch and gain with the launch speed.
        swingwhoosh: wrap(function (b, t, g, sr) {
            b.n(t, 0.38, g * 0.8, { f0: 350 * sr, f1: 1700 * sr, q: 1.1, atk: 0.1 });
            b.n(t + 0.05, 0.25, g * 0.25, { type: 'highpass', f0: 3000 * sr, q: 0.6, atk: 0.08 });
        }),
        // ---- round 4: puzzles ----
        // Kalimba/bell note of the glyph pentatonic: sr is the ratio to C5 (523 Hz).
        glyph: wrap(function (b, t, g, sr) {
            const f = 523.25 * sr;
            b.o('sine', f, f, t, 0.9, g * 0.5, { atk: 0.003 });
            b.o('triangle', f * 2, f * 2, t, 0.35, g * 0.14, { atk: 0.002 });
            b.o('sine', f * 5.4, f * 5.4, t, 0.1, g * 0.07, { atk: 0.001 });
            b.n(t, 0.012, g * 0.18, { type: 'highpass', f0: 4000 });
        }),
        // Runic resonance: a warm fundamental with a hollow fifth and octave tail.
        resonance: wrap(function (b, t, g, sr, o) {
            if (Array.isArray(o.notes) && o.notes.length) {
                o.notes.forEach(function (f, i) {
                    const tt = t + i * 0.09;
                    b.bell(f, tt, i === o.notes.length - 1 ? 0.78 : 0.45, g * (i === o.notes.length - 1 ? 0.24 : 0.16));
                    b.o('sine', f, f * 0.995, tt, i === o.notes.length - 1 ? 0.62 : 0.32, g * 0.12, { atk: 0.006 });
                });
                return;
            }
            const f = (o.freq || 523.25) * sr;
            b.o('sine', f, f * 0.995, t, 0.62, g * 0.42, { atk: 0.006 });
            b.bell(f * 1.5, t + 0.018, 0.72, g * 0.2);
            b.bell(f * 2, t + 0.035, 0.9, g * 0.16);
            b.n(t, 0.009, g * 0.08, { type: 'highpass', f0: 4200, q: 0.7 });
        }),
        // Grounded impact: higher incoming speed makes a louder, lower thump.
        landing: wrap(function (b, t, g, sr, o) {
            const speed = Math.max(0, Math.min(1100, Number(o.speed) || 0));
            const weight = 0.58 + speed / 1100 * 0.5;
            b.o('sine', (150 - speed * 0.065) * sr, (48 - speed * 0.015) * sr, t, 0.24, g * weight, { atk: 0.002 });
            b.n(t, 0.028, g * weight * 0.38, { type: 'lowpass', f0: 900, f1: 160, q: 0.7, atk: 0.003 });
            b.n(t, 0.01, g * 0.25, { type: 'highpass', f0: 2300, q: 0.7 });
        }),
        // Soft dissonant down-chime (minor second falling), never harsh.
        seqfail: wrap(function (b, t, g, sr) {
            b.bell(392 * sr, t, 0.7, g * 0.2);
            b.bell(415 * sr, t + 0.01, 0.7, g * 0.16);
            b.bell(311 * sr, t + 0.16, 0.8, g * 0.2);
            b.bell(294 * sr, t + 0.17, 0.8, g * 0.14);
            b.o('sine', 440 * sr, 200 * sr, t, 0.5, g * 0.12, { atk: 0.02 });
        }),
        // Puzzle solved fanfare: bright rising five-note arpeggio resolving into a held major chord (~1.2 s).
        solved: wrap(function (b, t, g, sr, o) {
            const notes = [523.25, 659.25, 783.99, 1046.5, 1318.5];
            notes.forEach(function (f, i) {
                const tt = t + i * 0.1;
                b.bell(f * sr, tt, 0.6, g * 0.3);
                b.o('triangle', f * sr, f * sr, tt, 0.35, g * 0.12, { atk: 0.004 });
            });
            [523.25, 783.99, 1046.5, 1568].forEach(function (f, i) {
                b.bell(f * sr * (1 + (jit(o, i) - 0.5) * 0.004), t + 0.5, 0.75, g * 0.2);
            });
            b.o('sine', 261.6 * sr, 261.6 * sr, t + 0.5, 0.7, g * 0.22, { atk: 0.01 });
            b.n(t + 0.5, 0.2, g * 0.1, { type: 'highpass', f0: 5000, q: 0.6, atk: 0.01 });
        }),
        // Magic ping that leaps up (sr >= 1) or drops down (sr < 1).
        magicping: wrap(function (b, t, g, sr) {
            const up = sr >= 1;
            b.o('sine', (up ? 700 : 1500) * sr, (up ? 1500 : 600) * sr, t, 0.32, g * 0.3, { atk: 0.01 });
            b.bell((up ? 1568 : 988) * sr, t + (up ? 0.12 : 0.02), 0.5, g * 0.22);
        }),
        // Stone grind that rises (sr >= 1) or sinks (sr < 1).
        stairs: wrap(function (b, t, g, sr, o) {
            const up = sr >= 1;
            b.n(t, 0.9, g * 0.5, { type: 'lowpass', f0: up ? 220 : 520, f1: up ? 520 : 220, q: 1.4, atk: 0.12 });
            const w = b.o('sawtooth', up ? 48 : 70, up ? 70 : 48, t, 0.9, g * 0.3, { filter: 'lowpass', ff: 180, atk: 0.1 });
            b.lfo(w.osc.frequency, 8, 3, t, 0.9);
            for (let i = 0; i < 5; i++) b.n(t + 0.15 * i + jit(o, i) * 0.05, 0.03, g * 0.3, { f0: up ? 900 + i * 150 : 1500 - i * 150, q: 2 });
            b.o('sine', 70, 40, up ? t : t + 0.8, 0.2, g * 0.4, { atk: 0.01 });
        }),
        // Brazier ignition: whoosh up into a crackle; sr lifts the pitch with the order.
        ignite: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.5, g * 0.6, { f0: 350 * sr, f1: 2200 * sr, q: 1.2, atk: 0.12 });
            b.o('sawtooth', 120 * sr, 260 * sr, t, 0.4, g * 0.14, { filter: 'lowpass', ff: 900, atk: 0.1 });
            for (let i = 0; i < 7; i++) b.n(t + 0.15 + i * 0.045 + jit(o, i) * 0.03, 0.02, g * (0.2 + jit(o, i + 9) * 0.3), { type: 'highpass', f0: (2500 + jit(o, i + 3) * 3000) * sr, q: 1 });
            b.bell(880 * sr, t + 0.08, 0.5, g * 0.1);
        }),
        // Failed ritual: hiss that fizzles out as the fire dies.
        fizzle: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.7, g * 0.5, { type: 'highpass', f0: 4200 * sr, q: 0.6, atk: 0.02 });
            b.o('sine', 600 * sr, 110 * sr, t, 0.6, g * 0.18, { atk: 0.01 });
            for (let i = 0; i < 6; i++) b.n(t + i * 0.09 + jit(o, i) * 0.05, 0.02, g * 0.3 * (1 - i / 7), { type: 'highpass', f0: 3000 + jit(o, i + 9) * 3000, q: 1 });
        }),
        // ---- places: interiors, caves and cutscene UI (no x: always centred) ----
        // Short rising warning chirp, played twice; sr sets the creature's pitch.
        chirp: wrap(function (b, t, g, sr) {
            [0, 0.11].forEach(function (dt) {
                b.o('sine', 700 * sr, 1500 * sr, t + dt, 0.1, g * 0.35, { atk: 0.008 });
                b.o('triangle', 1400 * sr, 2600 * sr, t + dt + 0.03, 0.07, g * 0.1, { atk: 0.006 });
            });
        }),
        // Creature pop followed by a little sparkle.
        popspark: wrap(function (b, t, g, sr, o) {
            b.o('sine', 420 * sr, 120 * sr, t, 0.09, g * 0.8, { atk: 0.002 });
            b.n(t, 0.02, g * 0.4, { type: 'highpass', f0: 3000 });
            for (let i = 0; i < 3; i++) b.bell(1800 * sr * (1 + i * 0.26 + (jit(o, i) - 0.5) * 0.02), t + 0.05 + i * 0.05, 0.35, g * 0.12);
        }),
        // Hollow stone rumble: low sine drop, dark noise bed and a few falling pebbles.
        hollow: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.9, g * 0.6, { type: 'lowpass', f0: 380, f1: 110, q: 1.2, atk: 0.1 });
            b.o('sine', 64 * sr, 38 * sr, t, 0.8, g * 0.7, { atk: 0.05 });
            b.o('sawtooth', 48 * sr, 40 * sr, t, 0.8, g * 0.18, { filter: 'lowpass', ff: 160, atk: 0.1 });
            for (let i = 0; i < 4; i++) b.n(t + 0.3 + i * 0.1 + jit(o, i) * 0.06, 0.02, g * 0.2, { f0: 900 + jit(o, i + 5) * 900, q: 2 });
        }),
        // Very soft footstep: a tiny low-passed thud.
        softstep: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.07, g * 0.5, { type: 'lowpass', f0: 900, f1: 300, q: 0.7, atk: 0.004 });
            b.o('sine', (110 + o.v * 30) * sr, 60 * sr, t, 0.07, g * 0.5, { atk: 0.004 });
        }),
        // Dull double knock on a locked door.
        knock2: wrap(function (b, t, g, sr) {
            [0, 0.15].forEach(function (dt) {
                b.o('sine', 170 * sr, 90 * sr, t + dt, 0.1, g * 0.9, { atk: 0.003 });
                b.n(t + dt, 0.025, g * 0.5, { f0: 700, q: 1.5 });
            });
        }),
        // Room cleared: three bright rising notes (C, E, G) with the last one held.
        fanfare3: wrap(function (b, t, g, sr) {
            [523.25, 659.25, 783.99].forEach(function (f, i) {
                const tt = t + i * 0.1;
                b.bell(f * sr, tt, i === 2 ? 0.8 : 0.45, g * 0.3);
                b.o('triangle', f * sr, f * sr, tt, i === 2 ? 0.5 : 0.25, g * 0.14, { atk: 0.004 });
            });
        }),
        // Empty chest: a dry click and a hollow tap.
        dryclick: wrap(function (b, t, g, sr) {
            b.n(t, 0.012, g * 0.8, { type: 'highpass', f0: 2500 });
            b.o('sine', 300 * sr, 150 * sr, t, 0.05, g * 0.3, { atk: 0.002 });
        }),
        // Creature hit: wet squelch with a soft thud.
        squelch: wrap(function (b, t, g, sr) {
            b.n(t, 0.16, g * 0.7, { f0: 900, f1: 250, q: 2.5, atk: 0.01 });
            b.o('sine', 230 * sr, 90 * sr, t, 0.14, g * 0.6, { atk: 0.005 });
            b.o('sawtooth', 160 * sr, 70 * sr, t, 0.12, g * 0.15, { filter: 'lowpass', ff: 600 });
        }),
        // Cutscene swell: low and rising (sr >= 1) or falling (sr < 1), soft and slow.
        cutswell: wrap(function (b, t, g, sr) {
            const up = sr >= 1;
            b.n(t, 1.2, g * 0.4, { type: 'lowpass', f0: up ? 200 : 700, f1: up ? 700 : 200, q: 0.8, atk: 0.5 });
            b.o('sine', up ? 110 : 150, up ? 170 : 90, t, 1.2, g * 0.35, { atk: 0.5 });
            b.o('triangle', up ? 220 : 300, up ? 340 : 180, t, 1.1, g * 0.1, { atk: 0.5 });
        }),
        // Cutscene page turn: a quiet tick.
        cuttick: wrap(function (b, t, g, sr) {
            b.n(t, 0.02, g * 0.5, { f0: 2200 * sr, q: 3 });
            b.o('sine', 900 * sr, 700 * sr, t, 0.03, g * 0.15, { atk: 0.002 });
        }),
        // Windray: a breathy, living lift with a separate soft harmonic glint.
        ray_board: wrap(function (b, t, g, sr, o) {
            const f = (o.freq || 440) * sr;
            const tone = b.o('sine', f * 0.92, f * 1.035, t, 0.5, g * 0.46, { atk: 0.15 });
            b.lfo(tone.osc.frequency, 3.6, f * 0.004, t, 0.5);
            b.n(t, 0.42, g * 0.06, { f0: f * 1.8, f1: f * 3.8, q: 0.62, atk: 0.17 });
        }),
        ray_leap: wrap(function (b, t, g, sr, o) {
            const f = (o.freq || 440) * sr;
            const tone = b.o('sine', f * 0.86, f * 1.18, t, 0.38, g * 0.44, { atk: 0.09 });
            b.lfo(tone.osc.frequency, 5, f * 0.006, t, 0.38);
            b.n(t, 0.3, g * 0.065, { f0: f * 1.7, f1: f * 4.2, q: 0.58, atk: 0.1 });
        }),
        ray_glint: wrap(function (b, t, g, sr, o) {
            const f = (o.freq || 440) * sr;
            b.o('sine', f * 1.5, f * 1.54, t, 0.4, g * 0.1, { atk: 0.1, filter: 'lowpass', ff: 2800, q: 0.7 });
            b.n(t + 0.03, 0.38, g * 0.13, { type: 'highpass', f0: f * 4.2, f1: f * 2.5, q: 0.55, atk: 0.15 });
        }),
        // Bell toad: a throaty, filtered call with a restrained metallic formant.
        toad_bellow: wrap(function (b, t, g, sr) {
            const f = 104 * sr;
            b.o('sine', f * 0.9, f * 1.12, t, 0.58, g * 0.52, { atk: 0.2 });
            b.o('triangle', f * 1.9, f * 2.2, t + 0.025, 0.55, g * 0.2, { filter: 'lowpass', ff: 520 * sr, q: 1.3, atk: 0.18 });
            b.o('triangle', f * 3.1, f * 3.5, t + 0.055, 0.47, g * 0.14, { filter: 'bandpass', ff: 430 * sr, q: 3.2, atk: 0.14 });
            b.bell(f * 5.05, t + 0.1, 0.68, g * 0.17);
            b.n(t + 0.04, 0.4, g * 0.055, { type: 'lowpass', f0: 420 * sr, f1: 300 * sr, q: 0.7, atk: 0.18 });
        }),
        toad_roll: wrap(function (b, t, g, sr) {
            const f = 82 * sr;
            b.o('sine', f * 1.4, f * 0.62, t, 0.8, g * 0.38, { atk: 0.16 });
            b.o('triangle', f * 2.3, f * 1.1, t + 0.04, 0.72, g * 0.14, { filter: 'lowpass', ff: 360 * sr, q: 0.8, atk: 0.13 });
            for (let i = 0; i < 5; i++) {
                const tt = t + i * 0.13;
                const pulse = (1 - i * 0.13) * g;
                b.o('sine', (145 - i * 12) * sr, (96 - i * 8) * sr, tt, 0.18, pulse * 0.26, { atk: 0.035 });
                b.n(tt, 0.16, pulse * 0.045, { type: 'lowpass', f0: 420 * sr, f1: 180 * sr, q: 0.7, atk: 0.05 });
            }
        }),
        // A lantern moth flutters in short, tapering wing-like pulses before diving.
        moth_flutter: wrap(function (b, t, g, sr, o) {
            for (let i = 0; i < 4; i++) {
                const tt = t + i * 0.075;
                const f = (1580 + i * 95 + jit(o, i) * 70) * sr;
                const level = g * (0.28 - i * 0.035);
                b.o('sine', f, f * (i % 2 ? 0.96 : 1.04), tt, 0.095, level, { atk: 0.018 });
                b.n(tt + 0.012, 0.048, level * 0.15, { type: 'highpass', f0: 2600 * sr, f1: 3300 * sr, q: 0.65, atk: 0.02 });
            }
        }),
        moth_dive: wrap(function (b, t, g, sr) {
            b.n(t, 0.46, g * 0.48, { type: 'bandpass', f0: 2300 * sr, f1: 520 * sr, q: 0.65, atk: 0.14 });
            b.n(t + 0.035, 0.38, g * 0.19, { type: 'highpass', f0: 3600 * sr, f1: 1700 * sr, q: 0.55, atk: 0.13 });
            b.o('sine', 1180 * sr, 280 * sr, t + 0.025, 0.48, g * 0.18, { atk: 0.12 });
        }),
        roller_turn: wrap(function (b, t, g, sr) {
            b.o('sawtooth', 168 * sr, 112 * sr, t, 0.37, g * 0.18, { filter: 'lowpass', ff: 520 * sr, q: 1.2, atk: 0.035 });
            [0, 0.095, 0.19].forEach(function (dt, i) {
                const f = (760 - i * 95) * sr;
                b.o('triangle', f, f * 0.9, t + dt, 0.065, g * (0.2 - i * 0.035), { filter: 'bandpass', ff: 1200 * sr, q: 1.4, atk: 0.009 });
                b.n(t + dt, 0.026, g * (0.13 - i * 0.02), { type: 'lowpass', f0: 1800 * sr, f1: 900 * sr, q: 0.75, atk: 0.004 });
            });
            b.o('sine', 130 * sr, 72 * sr, t + 0.24, 0.2, g * 0.26, { atk: 0.008 });
        })
    };

    function dragonRoar(b, t, g, sr, o, frost) {
        const d = 1.6;
        const hi = frost ? 1.25 : 1;
        const grr = b.o('sawtooth', 120 * sr * hi, 42 * sr * hi, t, d, g * 0.42, { filter: 'lowpass', ff: frost ? 1300 : 900, q: 1, atk: 0.12 });
        b.o('sawtooth', 124 * sr * hi, 44 * sr * hi, t, d, g * 0.24, { filter: 'lowpass', ff: 700, q: 1, atk: 0.14 });
        b.lfo(grr.g.gain, 24, g * 0.12, t, d);
        b.n(t, d, g * 0.55, { f0: 640 * sr * hi, f1: 300 * sr * hi, q: 5, atk: 0.1 });
        b.n(t, d, g * 0.4, { f0: 1150 * sr * hi, f1: 700 * sr * hi, q: 6, atk: 0.1 });
        b.n(t + 0.05, d * 0.8, g * 0.18, { f0: 2400 * sr * hi, f1: 1500 * sr * hi, q: 5, atk: 0.1 });
        b.o('sine', 52 * sr, 26 * sr, t, d, g * 0.6, { atk: 0.15 });
        if (frost) {
            b.n(t + 0.1, 1.2, g * 0.3, { type: 'highpass', f0: 4500, q: 0.6, atk: 0.2 });
            b.bell(2900 * sr * (1 + jit(o, 1) * 0.1), t + 0.2, 0.9, g * 0.05);
        }
    }

    function resonanceNotes(theme, index, phrase) {
        const scale = theme && theme.scale && theme.scale.length ? theme.scale : [0, 2, 4, 7, 9, 12, 14];
        const rootMidi = theme ? theme.root : 60;
        const degree = Math.max(0, Math.floor(Number(index) || 0));
        const note = function (step) {
            return 440 * Math.pow(2, (rootMidi + 12 + step - 69) / 12);
        };
        return {
            freq: note(scale[degree % scale.length] + 12 * Math.floor(degree / scale.length)),
            notes: phrase ? [0, 2, 4].map(function (step) { return note(scale[step]); }) : null
        };
    }

    function cueTuning(theme, cue, detail) {
        if (!cue) return null;
        const index = cue.scaleByIndex && detail && Number.isInteger(detail.index) ? detail.index : cue.themeIndex;
        return Number.isInteger(index) ? resonanceNotes(theme, index, !!cue.scalePhrase) : null;
    }

    const CUES = {
        moth_tell: { sound: false, gain: 0.42, cooldown: 700, synth: 'moth_flutter', sg: 0.3, sr: 1, also: [{ synth: 'shimmer', sg: 0.1, sr: 1.7, delay: 0.12 }] },
        moth_dive: { sound: false, gain: 0.48, cooldown: 320, synth: 'moth_dive', sg: 0.42, sr: 1, also: [{ synth: 'whistle', sg: 0.15, sr: 0.82, delay: 0.035 }] },
        toad_tell: { sound: false, gain: 0.48, cooldown: 850, synth: 'toad_bellow', sg: 0.58, sr: 0.9, also: [{ synth: 'clank', sg: 0.22, sr: 0.62, delay: 0.08 }] },
        toad_wave: { sound: false, gain: 0.58, cooldown: 450, synth: 'toad_roll', sg: 0.56, sr: 1, also: [{ synth: 'hollow', sg: 0.19, sr: 0.65, delay: 0.12 }] },
        roller_turn: { sound: false, gain: 0.44, cooldown: 420, synth: 'roller_turn', sg: 0.35, sr: 0.82, also: [{ synth: 'clank', sg: 0.18, sr: 0.7, delay: 0.14 }] },
        moon_step: { sound: 'step-stone', gain: 0.26, cooldown: 170, synth: 'landing', sg: 0.42, speed: 'impactSpeed', also: [{ synth: 'hollow', sg: 0.16, sr: 0.72, delay: 0.035 }] }
    };

    root.RunenpfadSfx = { SYNTHS: SYNTHS, CUES: CUES, builder: builder, resonanceNotes: resonanceNotes, cueTuning: cueTuning };
})(typeof window !== 'undefined' ? window : globalThis);
