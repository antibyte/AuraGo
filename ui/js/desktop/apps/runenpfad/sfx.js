/* Runenpfad synthesized sound recipes (global RunenpfadSfx). DOM-free and deterministic;
   upgrades melee, bow, bells, magic, explosions, ropes, and creature voices with RunenpfadAudioDsp physical models. */
(function (root) {
    'use strict';

    function builder(ac, noise, dest, t) {
        const nodes = [];
        const dsp = root.RunenpfadAudioDsp || null;
        let end = t + 0.05;
        let noiseAt = 0;
        const b = {
            ac: ac, noise: noise, dest: dest, dsp: dsp,
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
            sat: function (kind, to) {
                if (!dsp || typeof ac.createWaveShaper !== 'function') return to || dest;
                const ws = b.keep(ac.createWaveShaper());
                ws.curve = dsp.getCurve(ac, kind || 'warm');
                ws.connect(to || dest);
                return ws;
            },
            model: function (sampleId, t0, rate, peak, maxDur, opt) {
                if (!dsp) return false;
                const sample = dsp.getSample(ac, sampleId);
                if (!sample) return false;
                opt = opt || {};
                const src = b.keep(ac.createBufferSource());
                src.buffer = sample.buf;
                src.playbackRate.value = Math.max(0.25, Math.min(4.0, rate || 1));
                const dur = maxDur || (sample.buf.duration / src.playbackRate.value) || 0.6;
                const g = b.env(peak, t0, opt.atk || 0.002, dur, opt.to);
                if (opt.filter) {
                    const f = b.keep(ac.createBiquadFilter());
                    f.type = opt.filter;
                    f.frequency.value = opt.ff || 3000;
                    f.Q.value = opt.q || 0.8;
                    src.connect(f);
                    f.connect(g);
                } else {
                    src.connect(g);
                }
                src.start(t0);
                src.stop(t0 + dur + 0.03);
                return true;
            },
            o: function (type, f0, f1, t0, dur, peak, opt) {
                opt = opt || {};
                const os = b.keep(ac.createOscillator());
                os.type = type;
                os.frequency.setValueAtTime(f0, t0);
                if (f1 && f1 !== f0) os.frequency.exponentialRampToValueAtTime(Math.max(20, f1), t0 + dur);
                const g = b.env(peak, t0, opt.atk || 0.004, dur, opt.to);
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
            n: function (t0, dur, peak, opt) {
                opt = opt || {};
                const src = b.keep(ac.createBufferSource());
                src.buffer = (dsp && opt.color && dsp.getNoise(ac, opt.color)) || noise;
                src.loop = true;
                const f = b.keep(ac.createBiquadFilter());
                f.type = opt.type || 'bandpass';
                f.Q.value = opt.q || 0.8;
                f.frequency.setValueAtTime(opt.f0 || 1500, t0);
                if (opt.f1 && opt.f1 !== opt.f0) f.frequency.exponentialRampToValueAtTime(Math.max(30, opt.f1), t0 + dur);
                const g = b.env(peak, t0, opt.atk || 0.003, dur, opt.to);
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
            bell: function (f, t0, dur, peak) {
                if (dsp) {
                    const useDark = f < 420;
                    const ref = useDark ? 220 : 523.251;
                    if (b.model(useDark ? 'darkbell' : 'celesta', t0, f / ref, peak * 1.15, dur)) return;
                }
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
            grain: function (t0, dur, peak, targets, off) {
                const src = b.keep(ac.createBufferSource());
                src.buffer = (dsp && dsp.getNoise(ac, 'crackle')) || noise;
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
        // Tempered blade slash: crisp white-noise air crescent sweep + physical sword-ring steel harmonics.
        sword: wrap(function (b, t, g, sr, o) {
            const src = b.keep(b.ac.createBufferSource());
            src.buffer = b.noise;
            src.loop = true;
            const f = b.keep(b.ac.createBiquadFilter());
            f.type = 'bandpass';
            f.Q.value = 1.2;
            f.frequency.setValueAtTime(520 * sr, t);
            f.frequency.exponentialRampToValueAtTime(3800 * sr, t + 0.068);
            f.frequency.exponentialRampToValueAtTime(1550 * sr, t + 0.21);
            const env = b.keep(b.ac.createGain());
            env.gain.setValueAtTime(0.0001, t);
            env.gain.linearRampToValueAtTime(Math.max(0.0002, g * 0.58), t + 0.025);
            env.gain.linearRampToValueAtTime(Math.max(0.0002, g * 0.96), t + 0.07);
            env.gain.exponentialRampToValueAtTime(0.0001, t + 0.22);
            src.connect(f);
            f.connect(env);
            env.connect(b.dest);
            src.start(t, 0.11 + o.v * 0.3);
            src.stop(t + 0.24);
            b.touch(t + 0.24);
            b.model('sword_ring', t + 0.062, (1.28 + o.v * 0.06) * sr, g * 0.34, 0.22, { filter: 'highpass', ff: 1600 });
            [3600, 4750, 6100].forEach(function (hz, i) {
                b.o('sine', hz * sr * (0.97 + jit(o, i) * 0.06), 0, t + 0.068, 0.15, g * 0.20 / (1 + i * 0.38), { atk: 0.018 });
            });
        }),
        // Taut bowstring release: Karplus-Strong lute/gut pluck + crisp string snap + feathered arrow airflight.
        twang: wrap(function (b, t, g, sr, o) {
            const f = 196 * sr * (1 + o.v * 0.06);
            if (!b.model('lute', t, f / 261.6256, g * 0.78, 0.44)) {
                b.o('sawtooth', f * 1.03, f, t, 0.34, g * 0.34, { filter: 'lowpass', ff: 1900, q: 1.1 });
            }
            b.n(t, 0.022, g * 0.26, { color: 'crackle', type: 'highpass', f0: 2200 * sr });
            b.n(t + 0.015, 0.17, g * 0.44, { color: 'pink', f0: 1400 * sr, f1: 3900 * sr, q: 1.2, atk: 0.035 });
        }),
        // Dense weighted impact: taiko chest punch + crack transient + brown debris tail.
        impact: wrap(function (b, t, g, sr) {
            const out = b.sat('warm');
            b.model('taiko', t, 1.3 * sr, g * 0.48, 0.24, { to: out });
            b.n(t, 0.016, g * 0.8, { color: 'crackle', type: 'highpass', f0: 3200, q: 0.7, to: out });
            b.o('sine', 155 * sr, 46 * sr, t, 0.22, g * 0.78, { atk: 0.002, to: out });
            b.n(t + 0.004, 0.12, g * 0.34, { color: 'brown', type: 'lowpass', f0: 1400, f1: 240, q: 0.7, to: out });
        }),
        // Cinematic explosion: saturated sub shockwave + taiko body + deep brown rumble + sharp debris snap.
        boom: wrap(function (b, t, g, sr) {
            const out = b.sat('drive');
            b.o('sine', 88 * sr, 24 * sr, t, 0.9, g * 0.92, { atk: 0.006, to: out });
            b.model('taiko', t, 0.68 * sr, g * 0.65, 0.75, { to: out });
            b.n(t, 0.9, g * 0.72, { color: 'brown', type: 'lowpass', f0: 1350, f1: 95, q: 0.7, atk: 0.004, to: out });
            b.n(t, 0.035, g * 0.85, { color: 'crackle', type: 'highpass', f0: 2600, q: 0.7, to: out });
        }),
        // Celestial magic shimmer: harp & celesta harmonic cascade.
        shimmer: wrap(function (b, t, g, sr, o) {
            [1, 1.25, 1.5, 2, 2.5].forEach(function (m, i) {
                const hz = 660 * sr * m * (1 + (jit(o, i) - 0.5) * 0.008);
                if (!b.model(i % 2 ? 'glass' : 'celesta', t + i * 0.055, hz / 523.251, g * 0.34 / (1 + i * 0.2), 0.78)) {
                    b.bell(hz, t + i * 0.055, 0.7, g * 0.22 / (1 + i * 0.25));
                }
            });
        }),
        ice: wrap(function (b, t, g, sr, o) {
            b.model('glass', t, 1.4 * sr, g * 0.42, 0.65);
            const modes = [2300, 3700, 5100, 6800, 8900, 11000].map(function (f, i) {
                return b.reso(f * sr * (0.97 + jit(o, i) * 0.06), 25 + jit(o, i + 10) * 35, t, 0.08 + jit(o, i + 20) * 0.27, g * 8);
            });
            b.grain(t, 0.03, g * 0.85, null, 0.05);
            b.grain(t, 0.03, g * 1.1, modes, 0.2);
            b.n(t, 0.06, g * 0.38, { color: 'pink', type: 'lowpass', f0: 800, q: 0.7, atk: 0.004 });
            for (let i = 0; i < 24; i++) {
                const at = Math.pow(jit(o, i + 40), 1.8) * 0.42;
                const lvl = g * 0.72 * (1 - at / 0.6);
                const a1 = modes[Math.floor(jit(o, i + 80) * 6)];
                const tg = jit(o, i + 120) > 0.5 ? [a1, modes[Math.floor(jit(o, i + 160) * 6)]] : [a1];
                b.grain(t + 0.01 + at, 0.001 + jit(o, i + 200) * 0.004, lvl, tg, jit(o, i + 240) * 0.4);
            }
        }),
        crack: wrap(function (b, t, g) {
            const out = b.sat('drive');
            b.n(t, 0.06, g * 0.9, { color: 'crackle', type: 'highpass', f0: 1400, q: 0.8, to: out });
            b.n(t, 0.3, g * 0.5, { color: 'pink', f0: 3200, f1: 380, q: 0.8, to: out });
            b.n(t + 0.03, 0.58, g * 0.46, { color: 'brown', type: 'lowpass', f0: 340, q: 0.7, atk: 0.03, to: out });
        }),
        roar: wrap(function (b, t, g, sr, o) {
            const d = o.short ? 0.5 : 1.0;
            const out = b.sat('drive');
            const grr = b.o('sawtooth', 142 * sr, 54 * sr, t, d, g * 0.44, { filter: 'lowpass', ff: 860, q: 1, atk: 0.05, to: out });
            b.o('sawtooth', 146 * sr, 56 * sr, t, d, g * 0.26, { filter: 'lowpass', ff: 680, q: 1, atk: 0.05, to: out });
            b.lfo(grr.g.gain, 26, g * 0.12, t, d);
            b.n(t, d, g * 0.56, { color: 'brown', f0: 660 * sr, f1: 340 * sr, q: 4.5, atk: 0.05, to: out });
            b.n(t, d, g * 0.42, { color: 'pink', f0: 1320 * sr, f1: 760 * sr, q: 5.5, atk: 0.05, to: out });
        }),
        servo: wrap(function (b, t, g, sr, o) {
            const d = o.short ? 0.3 : 0.6;
            b.o('sawtooth', 380 * sr, 1150 * sr, t, d, g * 0.3, { filter: 'bandpass', ff: 1500, q: 4, atk: 0.05 });
            b.o('square', 95 * sr, 210 * sr, t, d, g * 0.14, { filter: 'lowpass', ff: 500, atk: 0.05 });
            b.n(t, d, g * 0.14, { color: 'pink', f0: 4000, q: 6, atk: 0.05 });
        }),
        rotor: wrap(function (b, t, g, sr, o) {
            const d = o.short ? 0.4 : 0.8;
            const r = b.o('square', 46 * sr, 58 * sr, t, d, g * 0.38, { filter: 'lowpass', ff: 420, atk: 0.1 });
            b.lfo(r.g.gain, 22, g * 0.16, t, d);
            b.o('triangle', 260 * sr, 380 * sr, t, d, g * 0.1, { atk: 0.12 });
            b.n(t, d, g * 0.12, { color: 'pink', f0: 1800, f1: 900, q: 1.5, atk: 0.1 });
        }),
        clank: wrap(function (b, t, g, sr) {
            const out = b.sat('warm');
            if (!b.model('anvil', t, sr * 0.9, g * 0.68, 0.48, { to: out })) {
                [310, 517, 830, 1210].forEach(function (f, i) {
                    b.o('square', f * sr, f * sr, t, 0.32 - i * 0.04, g * 0.13, { filter: 'bandpass', ff: f * sr * 1.5, q: 5 });
                });
            }
            b.n(t, 0.018, g * 0.52, { color: 'crackle', type: 'highpass', f0: 2800, to: out });
        }),
        clang: wrap(function (b, t, g, sr) {
            const out = b.sat('drive');
            if (!b.model('shield_clang', t, sr, g * 0.82, 0.54, { to: out })) {
                [[520, 0.4], [1330, 0.3], [2150, 0.2], [2900, 0.12]].forEach(function (p, i) {
                    b.o(i % 2 ? 'sine' : 'triangle', p[0] * sr, p[0] * sr * 0.995, t, 0.4 - i * 0.05, g * p[1]);
                });
            }
            b.n(t, 0.024, g * 0.55, { color: 'crackle', type: 'highpass', f0: 3000, to: out });
        }),
        ghost: wrap(function (b, t, g, sr) {
            const up = sr >= 1;
            b.n(t, 0.6, g * 0.5, { color: 'pink', f0: up ? 300 : 1500, f1: up ? 1500 : 300, q: 3, atk: 0.25 });
            const v = b.o('sine', (up ? 320 : 640), (up ? 640 : 320), t, 0.6, g * 0.18, { atk: 0.25 });
            b.lfo(v.osc.frequency, 6, 14, t, 0.6);
        }),
        ratchet: wrap(function (b, t, g, sr) {
            for (let i = 0; i < 8; i++) {
                const tt = t + 0.55 * (1 - Math.pow(1 - i / 7, 1.6));
                b.n(tt, 0.015, g * 0.48, { color: 'crackle', f0: (2800 + i * 140) * sr, q: 3 });
                b.o('square', (900 + i * 65) * sr, 0, tt, 0.018, g * 0.14);
            }
        }),
        puff: wrap(function (b, t, g, sr) {
            b.n(t, 0.2, g * 0.6, { color: 'pink', type: 'lowpass', f0: 1800, f1: 380, q: 0.8, atk: 0.01 });
            b.o('sine', 320 * sr, 140 * sr, t, 0.14, g * 0.4, { atk: 0.006 });
            b.n(t + 0.04, 0.1, g * 0.28, { f0: 900, q: 5 });
        }),
        screech: wrap(function (b, t, g, sr) {
            const s = b.o('sawtooth', 2600 * sr, 4300 * sr, t, 0.36, g * 0.3, { filter: 'bandpass', ff: 3500, q: 2.5, atk: 0.02 });
            b.lfo(s.osc.frequency, 38, 130, t, 0.36);
            b.o('square', 3900 * sr, 6100 * sr, t + 0.02, 0.3, g * 0.1, { filter: 'highpass', ff: 3000 });
        }),
        stomp: wrap(function (b, t, g, sr) {
            b.model('kick', t, sr, g * 0.55, 0.22);
            b.o('sine', 210 * sr, 70 * sr, t, 0.17, g * 0.72);
            b.n(t, 0.012, g * 0.35, { type: 'highpass', f0: 3000 });
        }),
        creak: wrap(function (b, t, g, sr, o) {
            const dur = 0.55;
            const master = b.env(g * 1.35, t, 0.04, dur);
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
        fireup: wrap(function (b, t, g, sr) {
            b.n(t, 0.52, g * 0.62, { color: 'pink', f0: 420 * sr, f1: 2800 * sr, q: 1.4, atk: 0.14 });
            b.model('harp', t + 0.04, 1.5 * sr, g * 0.42, 0.65);
        }),
        frostchime: wrap(function (b, t, g, sr) {
            [1, 1.25, 1.5, 2].forEach(function (m, i) { b.bell(1180 * sr * m, t + i * 0.07, 0.82, g * 0.24); });
        }),
        crackle: wrap(function (b, t, g, sr, o) {
            for (let i = 0; i < 8; i++) {
                const tt = t + i * 0.036 + jit(o, i) * 0.03;
                b.n(tt, 0.02, g * (0.3 + jit(o, i + 9) * 0.4), { color: 'crackle', type: 'highpass', f0: 2500 + jit(o, i + 3) * 3000, q: 1 });
            }
            b.o('sawtooth', 500 * sr, 130 * sr, t, 0.3, g * 0.15, { filter: 'bandpass', ff: 1400, q: 3 });
        }),
        hum: wrap(function (b, t, g, sr) {
            b.o('sine', 220 * sr, 220 * sr, t, 0.8, g * 0.3, { atk: 0.25 });
            b.o('sine', 225 * sr, 225 * sr, t, 0.8, g * 0.26, { atk: 0.25 });
            b.o('triangle', 440 * sr, 660 * sr, t, 0.6, g * 0.12, { atk: 0.25 });
        }),
        down: wrap(function (b, t, g, sr) {
            b.o('sine', 700 * sr, 200 * sr, t, 0.5, g * 0.3, { atk: 0.02 });
            b.o('triangle', 1050 * sr, 300 * sr, t, 0.45, g * 0.12, { atk: 0.02 });
        }),
        steam: wrap(function (b, t, g, sr) {
            b.n(t, 1.2, g * 0.72, { color: 'pink', type: 'highpass', f0: 1500 * sr, q: 0.6, atk: 0.08 });
            const r = b.n(t, 1.2, g * 0.55, { color: 'brown', type: 'lowpass', f0: 170, q: 0.8, atk: 0.1 });
            b.lfo(r.gain, 6, g * 0.2, t, 1.2);
        }),
        poof: wrap(function (b, t, g) {
            b.n(t, 0.26, g * 0.7, { color: 'pink', type: 'lowpass', f0: 2800, f1: 280, q: 0.7, atk: 0.01 });
        }),
        ping: wrap(function (b, t, g, sr) {
            if (!b.model('celesta', t, (1760 * sr) / 523.251, g * 0.55, 0.5)) {
                b.o('sine', 1760 * sr, 1760 * sr, t, 0.5, g * 0.4);
            }
        }),
        grind: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.9, g * 0.55, { color: 'brown', type: 'lowpass', f0: 520, f1: 240, q: 1.5, atk: 0.08 });
            const w = b.o('sawtooth', 58 * sr, 52 * sr, t, 0.9, g * 0.32, { filter: 'lowpass', ff: 210, atk: 0.08 });
            b.lfo(w.osc.frequency, 7, 3, t, 0.9);
            for (let i = 0; i < 5; i++) b.n(t + 0.12 * i + jit(o, i) * 0.04, 0.025, g * 0.35, { color: 'crackle', f0: 1200, q: 2 });
        }),
        hiss: wrap(function (b, t, g, sr) {
            b.n(t, 0.45, g * 0.4, { color: 'pink', f0: 5000 * sr, q: 0.5, atk: 0.15 });
        }),
        flame: wrap(function (b, t, g, sr, o) {
            const out = b.sat('warm');
            b.n(t, 0.88, g * 0.72, { color: 'brown', type: 'lowpass', f0: 1700, f1: 420, q: 0.8, atk: 0.04, to: out });
            b.o('sawtooth', 72 * sr, 44 * sr, t, 0.85, g * 0.3, { filter: 'lowpass', ff: 260, atk: 0.05, to: out });
            for (let i = 0; i < 4; i++) b.n(t + 0.1 + i * 0.12 + jit(o, i) * 0.05, 0.03, g * 0.32, { color: 'crackle', type: 'highpass', f0: 2500, to: out });
        }),
        charge: wrap(function (b, t, g, sr) {
            b.o('sawtooth', 140 * sr, 520 * sr, t, 0.55, g * 0.2, { filter: 'lowpass', ff: 1500, atk: 0.3 });
            b.o('sine', 70 * sr, 260 * sr, t, 0.55, g * 0.3, { atk: 0.3 });
        }),
        whistle: wrap(function (b, t, g, sr) {
            b.o('sine', 1800 * sr, 600 * sr, t, 0.4, g * 0.25, { atk: 0.05 });
            b.n(t, 0.4, g * 0.25, { color: 'pink', f0: 2400 * sr, f1: 900 * sr, q: 2, atk: 0.06 });
        }),
        dragonroar: wrap(function (b, t, g, sr, o) { dragonRoar(b, t, g, sr, o, false); }),
        dragonroar_frost: wrap(function (b, t, g, sr, o) { dragonRoar(b, t, g, sr, o, true); }),
        dragonwarn: wrap(function (b, t, g, sr) {
            b.o('sawtooth', 48 * sr, 96 * sr, t, 1.3, g * 0.32, { filter: 'lowpass', ff: 280, atk: 0.9 });
            b.o('sine', 38 * sr, 64 * sr, t, 1.3, g * 0.55, { atk: 0.8 });
            b.n(t, 1.3, g * 0.34, { color: 'brown', f0: 260 * sr, f1: 900 * sr, q: 2.5, atk: 0.9 });
            b.model('darkbell', t + 0.1, 0.5 * sr, g * 0.22, 1.1);
        }),
        dragonbreath: wrap(function (b, t, g, sr, o) {
            const d = 1.4;
            const out = b.sat('drive');
            b.n(t, d, g * 0.72, { color: 'pink', f0: 1500 * sr, f1: 680 * sr, q: 0.7, atk: 0.12, to: out });
            b.n(t, d, g * 0.5, { color: 'brown', type: 'lowpass', f0: 440, f1: 240, q: 0.8, atk: 0.15, to: out });
            b.o('sawtooth', 62 * sr, 44 * sr, t, d, g * 0.3, { filter: 'lowpass', ff: 220, atk: 0.15, to: out });
            for (let i = 0; i < 8; i++) {
                b.n(t + 0.12 + i * 0.14 + jit(o, i) * 0.06, 0.025, g * (0.24 + jit(o, i + 20) * 0.24), { color: 'crackle', type: 'highpass', f0: 2400 + jit(o, i + 40) * 2800, q: 1, to: out });
            }
        }),
        dragonbreath_frost: wrap(function (b, t, g, sr, o) {
            const d = 1.3;
            b.n(t, d, g * 0.7, { color: 'pink', type: 'highpass', f0: 3200 * sr, q: 0.6, atk: 0.1 });
            b.n(t, d, g * 0.42, { color: 'white', f0: 2200 * sr, f1: 1400 * sr, q: 1.5, atk: 0.12 });
            b.o('sawtooth', 90 * sr, 60 * sr, t, d, g * 0.2, { filter: 'lowpass', ff: 240, atk: 0.15 });
            for (let i = 0; i < 4; i++) b.model('glass', t + 0.12 + i * 0.26, 2.2 * sr * (1 + jit(o, i) * 0.35), g * 0.18, 0.55);
        }),
        dragonpass: wrap(function (b, t, g, sr) {
            for (let i = 0; i < 3; i++) {
                const tt = t + i * 0.55;
                const k = 1 - i * 0.18;
                b.n(tt, 0.7, g * 0.6 * k, { color: 'pink', f0: 180 * sr, f1: 520 * sr, q: 1.1, atk: 0.28 });
                b.model('taiko', tt + 0.24, 0.75 * sr, g * 0.68 * k, 0.48);
                b.o('sine', 75 * sr, 36 * sr, tt + 0.28, 0.4, g * 0.55 * k, { atk: 0.02 });
            }
        }),
        fwoosh: wrap(function (b, t, g, sr) {
            b.n(t, 0.34, g * 0.6, { color: 'pink', f0: 500 * sr, f1: 2200 * sr, q: 1, atk: 0.05 });
            b.o('sine', 260 * sr, 110 * sr, t, 0.1, g * 0.4, { atk: 0.004 });
            b.n(t, 0.015, g * 0.35, { type: 'highpass', f0: 3000 });
        }),
        // Rope grab: organic leather/hemp creak + taut physical string thwip.
        swinggrab: wrap(function (b, t, g, sr, o) {
            b.model('pizz', t, 0.85 * sr, g * 0.45, 0.25);
            const master = b.env(g * 1.15, t, 0.025, 0.38);
            const bp = b.keep(b.ac.createBiquadFilter());
            bp.type = 'bandpass';
            bp.frequency.setValueAtTime(720 * sr, t);
            bp.frequency.exponentialRampToValueAtTime(1180 * sr, t + 0.32);
            bp.Q.value = 5.5;
            bp.connect(master);
            for (let i = 0; i < 7; i++) b.grain(t + i * 0.04 + jit(o, i) * 0.01, 0.005, 0.42 + jit(o, i + 30) * 0.45, [bp], jit(o, i + 60) * 0.4);
            b.o('sine', 150 * sr, 82 * sr, t, 0.14, g * 0.65, { atk: 0.005 });
        }),
        swingwhoosh: wrap(function (b, t, g, sr) {
            b.n(t, 0.38, g * 0.78, { color: 'pink', f0: 360 * sr, f1: 1850 * sr, q: 1.1, atk: 0.09 });
            b.n(t + 0.05, 0.25, g * 0.26, { color: 'white', type: 'highpass', f0: 3000 * sr, q: 0.6, atk: 0.08 });
        }),
        glyph: wrap(function (b, t, g, sr) {
            const f = 523.25 * sr;
            if (!b.model('marimba', t, (f * 0.5) / 261.6256, g * 0.65, 0.72)) {
                b.o('sine', f, f, t, 0.85, g * 0.5, { atk: 0.003 });
            }
            b.model('celesta', t + 0.005, f / 523.251, g * 0.35, 0.65);
        }),
        resonance: wrap(function (b, t, g, sr, o) {
            if (Array.isArray(o.notes) && o.notes.length) {
                o.notes.forEach(function (f, i) {
                    const tt = t + i * 0.09;
                    b.bell(f, tt, i === o.notes.length - 1 ? 0.85 : 0.48, g * (i === o.notes.length - 1 ? 0.36 : 0.24));
                    b.model('harp', tt, f / 261.6256, g * 0.26, 0.7);
                });
                return;
            }
            const f = (o.freq || 523.25) * sr;
            b.model('harp', t, f / 261.6256, g * 0.45, 0.78);
            b.bell(f * 1.5, t + 0.02, 0.72, g * 0.24);
            b.bell(f * 2, t + 0.04, 0.9, g * 0.18);
        }),
        landing: wrap(function (b, t, g, sr, o) {
            const speed = Math.max(0, Math.min(1100, Number(o.speed) || 0));
            const weight = 0.58 + speed / 1100 * 0.5;
            b.o('sine', (150 - speed * 0.065) * sr, (48 - speed * 0.015) * sr, t, 0.24, g * weight, { atk: 0.002 });
            b.n(t, 0.032, g * weight * 0.42, { color: 'brown', type: 'lowpass', f0: 900, f1: 150, q: 0.7, atk: 0.003 });
            b.n(t, 0.012, g * 0.24, { type: 'highpass', f0: 2300, q: 0.7 });
        }),
        seqfail: wrap(function (b, t, g, sr) {
            b.bell(392 * sr, t, 0.7, g * 0.24);
            b.bell(415 * sr, t + 0.01, 0.7, g * 0.18);
            b.bell(311 * sr, t + 0.16, 0.8, g * 0.24);
            b.o('sine', 440 * sr, 200 * sr, t, 0.5, g * 0.16, { atk: 0.02 });
        }),
        // Heroic puzzle chime: flowing harp & celesta arpeggio into warm chord & octave glass shimmer.
        solved: wrap(function (b, t, g, sr, o) {
            const notes = [523.25, 659.25, 783.99, 1046.5, 1318.5];
            notes.forEach(function (f, i) {
                const tt = t + i * 0.095;
                b.bell(f * sr, tt, 0.72, g * 0.34);
                b.model('harp', tt, (f * 0.5 * sr) / 261.6256, g * 0.32, 0.72);
            });
            [523.25, 783.99, 1046.5, 1568].forEach(function (f, i) {
                b.bell(f * sr * (1 + (jit(o, i) - 0.5) * 0.004), t + 0.48, 0.9, g * 0.22);
                b.model('harp', t + 0.47 + i * 0.025, (f * 0.5 * sr) / 261.6256, g * 0.24, 0.88);
            });
            [1046.5, 1318.5, 1568, 2093].forEach(function (f, i) {
                b.model('glass', t + 0.5 + i * 0.02, (f * sr) / 523.251, g * 0.16, 0.92);
            });
            b.o('sine', 261.6 * sr, 261.6 * sr, t + 0.48, 0.75, g * 0.18, { atk: 0.01 });
        }),
        magicping: wrap(function (b, t, g, sr) {
            const up = sr >= 1;
            b.o('sine', (up ? 700 : 1500) * sr, (up ? 1500 : 600) * sr, t, 0.3, g * 0.28, { atk: 0.01 });
            b.bell((up ? 1568 : 988) * sr, t + (up ? 0.11 : 0.02), 0.55, g * 0.26);
        }),
        stairs: wrap(function (b, t, g, sr, o) {
            const up = sr >= 1;
            b.n(t, 0.9, g * 0.5, { color: 'brown', type: 'lowpass', f0: up ? 220 : 520, f1: up ? 520 : 220, q: 1.4, atk: 0.12 });
            const w = b.o('sawtooth', up ? 48 : 70, up ? 70 : 48, t, 0.9, g * 0.3, { filter: 'lowpass', ff: 180, atk: 0.1 });
            b.lfo(w.osc.frequency, 8, 3, t, 0.9);
            for (let i = 0; i < 4; i++) b.n(t + 0.16 * i + jit(o, i) * 0.05, 0.03, g * 0.3, { color: 'crackle', f0: up ? 900 + i * 150 : 1500 - i * 150, q: 2 });
            b.o('sine', 70, 40, up ? t : t + 0.8, 0.2, g * 0.4, { atk: 0.01 });
        }),
        ignite: wrap(function (b, t, g, sr, o) {
            const out = b.sat('warm');
            b.n(t, 0.52, g * 0.62, { color: 'pink', f0: 360 * sr, f1: 2400 * sr, q: 1.2, atk: 0.11, to: out });
            b.o('sawtooth', 120 * sr, 260 * sr, t, 0.4, g * 0.14, { filter: 'lowpass', ff: 900, atk: 0.1, to: out });
            for (let i = 0; i < 6; i++) b.n(t + 0.15 + i * 0.05 + jit(o, i) * 0.03, 0.02, g * (0.2 + jit(o, i + 9) * 0.3), { color: 'crackle', type: 'highpass', f0: (2500 + jit(o, i + 3) * 3000) * sr, q: 1, to: out });
            b.bell(880 * sr, t + 0.08, 0.5, g * 0.14);
        }),
        fizzle: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.7, g * 0.5, { color: 'pink', type: 'highpass', f0: 4200 * sr, q: 0.6, atk: 0.02 });
            b.o('sine', 600 * sr, 110 * sr, t, 0.6, g * 0.18, { atk: 0.01 });
            for (let i = 0; i < 5; i++) b.n(t + i * 0.1 + jit(o, i) * 0.05, 0.02, g * 0.3 * (1 - i / 6), { color: 'crackle', type: 'highpass', f0: 3000 + jit(o, i + 9) * 3000, q: 1 });
        }),
        chirp: wrap(function (b, t, g, sr) {
            [0, 0.11].forEach(function (dt) {
                b.o('sine', 700 * sr, 1500 * sr, t + dt, 0.1, g * 0.35, { atk: 0.008 });
                b.o('triangle', 1400 * sr, 2600 * sr, t + dt + 0.03, 0.07, g * 0.1, { atk: 0.006 });
            });
        }),
        popspark: wrap(function (b, t, g, sr, o) {
            b.o('sine', 420 * sr, 120 * sr, t, 0.09, g * 0.75, { atk: 0.002 });
            b.n(t, 0.02, g * 0.38, { type: 'highpass', f0: 3000 });
            for (let i = 0; i < 3; i++) b.bell(1800 * sr * (1 + i * 0.26 + (jit(o, i) - 0.5) * 0.02), t + 0.05 + i * 0.05, 0.35, g * 0.14);
        }),
        hollow: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.9, g * 0.6, { color: 'brown', type: 'lowpass', f0: 380, f1: 110, q: 1.2, atk: 0.1 });
            b.o('sine', 64 * sr, 38 * sr, t, 0.8, g * 0.7, { atk: 0.05 });
            b.o('sawtooth', 48 * sr, 40 * sr, t, 0.8, g * 0.18, { filter: 'lowpass', ff: 160, atk: 0.1 });
            for (let i = 0; i < 4; i++) b.n(t + 0.3 + i * 0.1 + jit(o, i) * 0.06, 0.02, g * 0.2, { color: 'crackle', f0: 900 + jit(o, i + 5) * 900, q: 2 });
        }),
        // Kept ultra-lean (4 nodes total) so rapid step bursts stay well below 150 nodes.
        softstep: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.065, g * 0.5, { color: 'pink', type: 'lowpass', f0: 820, f1: 280, q: 0.7, atk: 0.004 });
            b.o('sine', (110 + o.v * 30) * sr, 60 * sr, t, 0.065, g * 0.5, { atk: 0.004 });
        }),
        knock2: wrap(function (b, t, g, sr) {
            [0, 0.15].forEach(function (dt) {
                b.o('sine', 170 * sr, 90 * sr, t + dt, 0.1, g * 0.85, { atk: 0.003 });
                b.n(t + dt, 0.025, g * 0.45, { color: 'pink', f0: 700, q: 1.5 });
            });
        }),
        fanfare3: wrap(function (b, t, g, sr) {
            [523.25, 659.25, 783.99].forEach(function (f, i) {
                const tt = t + i * 0.095;
                b.bell(f * sr, tt, i === 2 ? 0.85 : 0.48, g * 0.34);
                b.model('harp', tt, (f * 0.5 * sr) / 261.6256, g * 0.28, i === 2 ? 0.75 : 0.4);
            });
        }),
        dryclick: wrap(function (b, t, g, sr) {
            b.n(t, 0.012, g * 0.8, { color: 'crackle', type: 'highpass', f0: 2500 });
            b.o('sine', 300 * sr, 150 * sr, t, 0.05, g * 0.3, { atk: 0.002 });
        }),
        squelch: wrap(function (b, t, g, sr) {
            b.n(t, 0.16, g * 0.7, { color: 'pink', f0: 900, f1: 250, q: 2.5, atk: 0.01 });
            b.o('sine', 230 * sr, 90 * sr, t, 0.14, g * 0.6, { atk: 0.005 });
            b.o('sawtooth', 160 * sr, 70 * sr, t, 0.12, g * 0.15, { filter: 'lowpass', ff: 600 });
        }),
        cutswell: wrap(function (b, t, g, sr) {
            const up = sr >= 1;
            b.n(t, 1.2, g * 0.38, { color: 'pink', type: 'lowpass', f0: up ? 200 : 700, f1: up ? 700 : 200, q: 0.8, atk: 0.5 });
            b.model('harp', t + 0.05, (up ? 220 : 196) / 261.6256, g * 0.32, 1.1);
            b.o('sine', up ? 110 : 150, up ? 170 : 90, t, 1.2, g * 0.32, { atk: 0.5 });
        }),
        cuttick: wrap(function (b, t, g, sr) {
            b.n(t, 0.02, g * 0.5, { color: 'pink', f0: 2200 * sr, q: 3 });
            b.o('sine', 900 * sr, 700 * sr, t, 0.03, g * 0.15, { atk: 0.002 });
        }),
        ray_board: wrap(function (b, t, g, sr, o) {
            const f = (o.freq || 440) * sr;
            const tone = b.o('sine', f * 0.92, f * 1.035, t, 0.5, g * 0.46, { atk: 0.15 });
            b.lfo(tone.osc.frequency, 3.6, f * 0.004, t, 0.5);
            b.n(t, 0.42, g * 0.065, { color: 'pink', f0: f * 1.8, f1: f * 3.8, q: 0.62, atk: 0.17 });
        }),
        ray_leap: wrap(function (b, t, g, sr, o) {
            const f = (o.freq || 440) * sr;
            const tone = b.o('sine', f * 0.86, f * 1.18, t, 0.38, g * 0.44, { atk: 0.09 });
            b.lfo(tone.osc.frequency, 5, f * 0.006, t, 0.38);
            b.n(t, 0.3, g * 0.068, { color: 'pink', f0: f * 1.7, f1: f * 4.2, q: 0.58, atk: 0.1 });
        }),
        ray_glint: wrap(function (b, t, g, sr, o) {
            const f = (o.freq || 440) * sr;
            b.model('celesta', t, (f * 1.5) / 523.251, g * 0.18, 0.45);
            b.n(t + 0.03, 0.38, g * 0.13, { color: 'pink', type: 'highpass', f0: f * 4.2, f1: f * 2.5, q: 0.55, atk: 0.15 });
        }),
        toad_bellow: wrap(function (b, t, g, sr) {
            const f = 104 * sr;
            b.o('sine', f * 0.9, f * 1.12, t, 0.58, g * 0.52, { atk: 0.2 });
            b.o('triangle', f * 1.9, f * 2.2, t + 0.025, 0.55, g * 0.2, { filter: 'lowpass', ff: 520 * sr, q: 1.3, atk: 0.18 });
            b.o('triangle', f * 3.1, f * 3.5, t + 0.055, 0.47, g * 0.14, { filter: 'bandpass', ff: 430 * sr, q: 3.2, atk: 0.14 });
            b.bell(f * 5.05, t + 0.1, 0.68, g * 0.18);
            b.n(t + 0.04, 0.4, g * 0.06, { color: 'brown', type: 'lowpass', f0: 420 * sr, f1: 300 * sr, q: 0.7, atk: 0.18 });
        }),
        toad_roll: wrap(function (b, t, g, sr) {
            const f = 82 * sr;
            b.o('sine', f * 1.4, f * 0.62, t, 0.8, g * 0.38, { atk: 0.16 });
            b.o('triangle', f * 2.3, f * 1.1, t + 0.04, 0.72, g * 0.14, { filter: 'lowpass', ff: 360 * sr, q: 0.8, atk: 0.13 });
            for (let i = 0; i < 5; i++) {
                const tt = t + i * 0.13;
                const pulse = (1 - i * 0.13) * g;
                b.o('sine', (145 - i * 12) * sr, (96 - i * 8) * sr, tt, 0.18, pulse * 0.26, { atk: 0.035 });
                b.n(tt, 0.16, pulse * 0.05, { color: 'brown', type: 'lowpass', f0: 420 * sr, f1: 180 * sr, q: 0.7, atk: 0.05 });
            }
        }),
        moth_flutter: wrap(function (b, t, g, sr, o) {
            for (let i = 0; i < 4; i++) {
                const tt = t + i * 0.075;
                const f = (1580 + i * 95 + jit(o, i) * 70) * sr;
                const level = g * (0.28 - i * 0.035);
                b.o('sine', f, f * (i % 2 ? 0.96 : 1.04), tt, 0.095, level, { atk: 0.018 });
                b.n(tt + 0.012, 0.048, level * 0.16, { color: 'pink', type: 'highpass', f0: 2600 * sr, f1: 3300 * sr, q: 0.65, atk: 0.02 });
            }
        }),
        moth_dive: wrap(function (b, t, g, sr) {
            b.n(t, 0.46, g * 0.48, { color: 'pink', type: 'bandpass', f0: 2300 * sr, f1: 520 * sr, q: 0.65, atk: 0.14 });
            b.n(t + 0.035, 0.38, g * 0.19, { color: 'white', type: 'highpass', f0: 3600 * sr, f1: 1700 * sr, q: 0.55, atk: 0.13 });
            b.o('sine', 1180 * sr, 280 * sr, t + 0.025, 0.48, g * 0.18, { atk: 0.12 });
        }),
        roller_turn: wrap(function (b, t, g, sr) {
            b.o('sawtooth', 168 * sr, 112 * sr, t, 0.37, g * 0.18, { filter: 'lowpass', ff: 520 * sr, q: 1.2, atk: 0.035 });
            [0, 0.095, 0.19].forEach(function (dt, i) {
                const f = (760 - i * 95) * sr;
                b.o('triangle', f, f * 0.9, t + dt, 0.065, g * (0.2 - i * 0.035), { filter: 'bandpass', ff: 1200 * sr, q: 1.4, atk: 0.009 });
                b.n(t + dt, 0.026, g * (0.13 - i * 0.02), { color: 'pink', type: 'lowpass', f0: 1800 * sr, f1: 900 * sr, q: 0.75, atk: 0.004 });
            });
            b.o('sine', 130 * sr, 72 * sr, t + 0.24, 0.2, g * 0.26, { atk: 0.008 });
        }),
        // Blade swing "fwhip": a 0.30 s air swish, band-passed noise (Q 1.4) whose centre goes 400 -> 2400 Hz at the amplitude peak (0.10 s) -> 700 Hz at the end,
        // then a 5 kHz low-pass. Amplitude rises linearly over 100 ms and decays smoothly to silence at 0.30 s. A quiet narrow 6 kHz noise band (Q 6, 30 ms attack,
        // 0.10 s, from the peak, about a sixth of the swish) is the blade edge. No sine partials, no click.
        sword2: wrap(function (b, t, g, sr) {
            const src = b.keep(b.ac.createBufferSource());
            src.buffer = (b.dsp && b.dsp.getNoise(b.ac, 'white')) || b.noise;
            src.loop = true;
            const f = b.keep(b.ac.createBiquadFilter());
            f.type = 'bandpass';
            f.Q.value = 1.4;
            f.frequency.setValueAtTime(400 * sr, t);
            f.frequency.exponentialRampToValueAtTime(2400 * sr, t + 0.1);
            f.frequency.exponentialRampToValueAtTime(700 * sr, t + 0.3);
            const lp = b.keep(b.ac.createBiquadFilter());
            lp.type = 'lowpass';
            lp.frequency.value = 5000;
            lp.Q.value = 0.7;
            const env = b.keep(b.ac.createGain());
            env.gain.setValueAtTime(0.0001, t);
            env.gain.linearRampToValueAtTime(Math.max(0.0002, g * 0.5), t + 0.1);
            env.gain.exponentialRampToValueAtTime(Math.max(0.0002, g * 0.25), t + 0.19);
            env.gain.exponentialRampToValueAtTime(0.0001, t + 0.3);
            env.connect(b.sat('warm'));
            b.touch(t + 0.34);
            src.connect(f);
            f.connect(lp);
            lp.connect(env);
            src.start(t, 0.11);
            src.stop(t + 0.32);
            b.n(t + 0.1, 0.1, g * 0.083, { color: 'white', f0: 6000 * sr, q: 6, atk: 0.03 });
        }),
        // Soft jump "hup": clean sine glide 300 -> 620 Hz over 0.16 s (8 ms attack) with a quieter triangle on the same glide, and a faint low-passed (800 Hz) noise puff.
        hop: wrap(function (b, t, g, sr) {
            b.o('sine', 300 * sr, 620 * sr, t, 0.16, g * 0.6, { atk: 0.008 });
            b.o('triangle', 300 * sr, 620 * sr, t, 0.16, g * 0.12, { atk: 0.008 });
            b.n(t, 0.1, g * 0.1, { color: 'pink', type: 'lowpass', f0: 800, q: 0.6, atk: 0.01 });
        }),
        // Skid scrape (0.18 s): band-passed pink noise sliding down, crackle grit, rubbery saw undertone, three grit ticks.
        skid: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.18, g * 0.55, { color: 'pink', f0: 1900 * sr, f1: 900 * sr, q: 1.6, atk: 0.012 });
            b.n(t, 0.14, g * 0.3, { color: 'crackle', type: 'highpass', f0: 2600 * sr, q: 0.8, atk: 0.01 });
            b.o('sawtooth', 190 * sr, 120 * sr, t, 0.16, g * 0.1, { filter: 'lowpass', ff: 600, atk: 0.01 });
            for (let i = 0; i < 3; i++) b.n(t + 0.03 + i * 0.045 + jit(o, i) * 0.015, 0.02, g * 0.25, { color: 'crackle', f0: (1500 + jit(o, i + 5) * 1200) * sr, q: 2 });
        }),
        // Ladder step: wooden tick (sine 210 -> 150 Hz) + noise knock + faint band-passed creak.
        climb: wrap(function (b, t, g, sr, o) {
            b.o('sine', 210 * sr, 150 * sr, t, 0.06, g * 0.55, { atk: 0.003 });
            b.n(t, 0.03, g * 0.35, { color: 'pink', f0: 1100 * sr, q: 2.5 });
            b.o('triangle', 520 * sr * (0.95 + jit(o, 0) * 0.1), 380 * sr, t + 0.015, 0.09, g * 0.1, { filter: 'bandpass', ff: 700, q: 3, atk: 0.015 });
        }),
        // Rope pull: narrow fibre creak (700 -> 1100 Hz noise, 0.11 s), three grit ticks, soft low thud.
        climb_rope: wrap(function (b, t, g, sr) {
            b.n(t, 0.11, g * 0.5, { color: 'pink', f0: 700 * sr, f1: 1100 * sr, q: 4.5, atk: 0.03 });
            for (let i = 0; i < 3; i++) b.n(t + i * 0.03, 0.012, g * 0.25, { color: 'crackle', f0: 1600 * sr, q: 2 });
            b.o('sine', 130 * sr, 95 * sr, t, 0.07, g * 0.3, { atk: 0.004 });
        }),
        // Heartbeat: two low sine thumps (70 -> 55 Hz and 62 -> 50 Hz, 0.11 s apart) with a short chest knock and brown noise tap.
        heartbeat: wrap(function (b, t, g, sr) {
            [[0, 70, 55, 1], [0.11, 62, 50, 0.8]].forEach(function (p) {
                b.o('sine', p[1] * sr, p[2] * sr, t + p[0], 0.16, g * 0.9 * p[3], { atk: 0.006 });
                b.o('sine', p[1] * 2.2 * sr, p[2] * 1.8 * sr, t + p[0], 0.05, g * 0.18 * p[3], { atk: 0.003 });
            });
            b.n(t, 0.04, g * 0.2, { color: 'brown', type: 'lowpass', f0: 260, f1: 120, q: 0.7, atk: 0.004 });
        }),
        // Revive: warm sine swell 196 -> 392 Hz over 0.7 s, soft triangle octave, rising pink-noise air, four celesta bells (C5 E5 G5 C6) 0.09 s apart.
        revive: wrap(function (b, t, g, sr) {
            b.o('sine', 196 * sr, 392 * sr, t, 0.7, g * 0.3, { atk: 0.25 });
            b.o('triangle', 392 * sr, 784 * sr, t + 0.05, 0.6, g * 0.08, { filter: 'lowpass', ff: 1800, atk: 0.25 });
            b.n(t, 0.7, g * 0.12, { color: 'pink', f0: 500 * sr, f1: 3200 * sr, q: 1.2, atk: 0.3 });
            [523.25, 659.25, 783.99, 1046.5].forEach(function (f, i) { b.bell(f * sr, t + 0.12 + i * 0.09, 0.4, g * 0.2); });
        }),
        // UI: soft wooden click (sine 560 -> 300 Hz) + noise tick + tiny 1.57 kHz chime.
        ui_select: wrap(function (b, t, g, sr) {
            b.o('sine', 560 * sr, 300 * sr, t, 0.04, g * 0.6, { atk: 0.002 });
            b.n(t, 0.018, g * 0.4, { color: 'pink', f0: 1400 * sr, q: 2.2 });
            b.o('sine', 1568 * sr, 0, t + 0.025, 0.2, g * 0.14, { atk: 0.004 });
        }),
        // UI: lower click (sine 380 -> 200 Hz) + dull tick + faint 784 Hz chime.
        ui_back: wrap(function (b, t, g, sr) {
            b.o('sine', 380 * sr, 200 * sr, t, 0.05, g * 0.6, { atk: 0.002 });
            b.n(t, 0.02, g * 0.35, { color: 'pink', f0: 800 * sr, q: 2 });
            b.o('sine', 784 * sr, 0, t + 0.03, 0.15, g * 0.1, { atk: 0.004 });
        }),
        // UI: dull double knock (sine 130 -> 75 Hz, 0.12 s apart) with low-passed brown ticks.
        ui_deny: wrap(function (b, t, g, sr) {
            [0, 0.12].forEach(function (dt) {
                b.o('sine', 130 * sr, 75 * sr, t + dt, 0.09, g * 0.8, { atk: 0.003 });
                b.n(t + dt, 0.03, g * 0.3, { color: 'brown', type: 'lowpass', f0: 500, q: 0.8 });
            });
        }),
        // UI: coin clink (coin model at 1.57 kHz, sine pair fallback) + celesta chime at 1318 Hz a beat later.
        ui_buy: wrap(function (b, t, g, sr) {
            const s = b.dsp && b.dsp.getSample(b.ac, 'coin');
            if (!b.model('coin', t, (1568 * sr) / (s ? s.refHz : 1318.51), g * 0.55, 0.4)) {
                [2400, 3300].forEach(function (hz, i) { b.o('sine', hz * sr, 0, t, 0.09, g * 0.2 / (1 + i * 0.5), { atk: 0.002 }); });
            }
            b.bell(1318.5 * sr, t + 0.07, 0.45, g * 0.22);
        }),
        // UI: short sweep (0.16 s); sr below 1 falls 900 -> 320 Hz (pause), otherwise rises 320 -> 900 Hz (resume); triangle octave + air.
        ui_sweep: wrap(function (b, t, g, sr) {
            const up = sr >= 1;
            b.o('sine', up ? 320 : 900, up ? 900 : 320, t, 0.16, g * 0.4, { atk: 0.02 });
            b.o('triangle', up ? 640 : 1800, up ? 1800 : 640, t, 0.14, g * 0.1, { atk: 0.02 });
            b.n(t, 0.16, g * 0.12, { color: 'pink', f0: up ? 600 : 2400, f1: up ? 2400 : 600, q: 1.1, atk: 0.03 });
        }),
        // UI: celesta-like star chime at 1046 Hz times sr (+ octave sparkle); ui_star_1..3 use sr 1, 1.26, 1.587 (major thirds).
        ui_star: wrap(function (b, t, g, sr) {
            b.bell(1046.5 * sr, t, 0.7, g * 0.42);
            b.bell(2093 * sr, t + 0.015, 0.45, g * 0.14);
            b.n(t, 0.015, g * 0.12, { color: 'crackle', type: 'highpass', f0: 5000 });
        }),
        // UI: three-note rising fanfare A4 - D5 - A5, 0.15 s apart, bell + triangle support, sparkle on top (about 0.7 s).
        ui_unlock: wrap(function (b, t, g, sr) {
            [440, 587.33, 880].forEach(function (f, i) {
                const tt = t + i * 0.15;
                const last = i === 2;
                b.bell(f * sr, tt, last ? 0.4 : 0.3, g * 0.34);
                b.o('triangle', f * sr, 0, tt, last ? 0.4 : 0.2, g * 0.1, { atk: 0.01 });
            });
            b.bell(1760 * sr, t + 0.32, 0.4, g * 0.14);
        })
    };

    // Rich organic dragon roar: driven throat growl + three vocal-cavity mid/presence formants
    // + controlled chest weight without transient peak stacking.
    function dragonRoar(b, t, g, sr, o, frost) {
        const d = 1.65;
        const hi = frost ? 1.24 : 1;
        const roarBus = b.sat('drive');
        const grr = b.o('sawtooth', 102 * sr * hi, 38 * sr * hi, t, d, g * 0.34, { filter: 'lowpass', ff: frost ? 1900 : 1650, q: 1.2, atk: 0.045, to: roarBus });
        b.o('sawtooth', 148 * sr * hi, 56 * sr * hi, t, d, g * 0.28, { filter: 'lowpass', ff: 1950, q: 1.0, atk: 0.055, to: roarBus });
        b.lfo(grr.g.gain, 26, g * 0.11, t, d);
        b.n(t, d, g * 0.56, { color: 'pink', f0: 540 * sr * hi, f1: 210 * sr * hi, q: 3.5, atk: 0.045, to: roarBus });
        b.n(t, d, g * 0.54, { color: 'pink', f0: 1120 * sr * hi, f1: 460 * sr * hi, q: 4.2, atk: 0.05, to: roarBus });
        b.n(t + 0.03, d * 0.85, g * 0.42, { color: 'crackle', f0: 2150 * sr * hi, f1: 880 * sr * hi, q: 3.0, atk: 0.045, to: roarBus });
        b.o('sine', 54 * sr, 26 * sr, t, d, g * 0.26, { atk: 0.06 });
        if (frost) {
            b.n(t + 0.1, 1.2, g * 0.34, { color: 'pink', type: 'highpass', f0: 4200, q: 0.6, atk: 0.2 });
            b.model('glass', t + 0.18, 2.4 * sr * (1 + jit(o, 1) * 0.1), g * 0.22, 0.9);
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
        skid: { sound: false, gain: 0.4, cooldown: 200, synth: 'skid', sg: 2.5 },
        climb_step: { sound: false, gain: 0.3, cooldown: 120, synth: 'climb', sg: 0.5, by: { key: 'kind', map: { ladder: { synth: 'climb' }, rope: { synth: 'climb_rope' }, '*': {} } } },
        heart_beat: { sound: false, gain: 0.4, cooldown: 300, synth: 'heartbeat', sg: 0.6 },
        revive: { sound: false, gain: 0.5, cooldown: 600, synth: 'revive', sg: 0.5 },
        gust_end: { sound: false, gain: 0.3, cooldown: 1500, synth: 'whoosh', sg: 1.0, sr: 0.55 },
        moon_step: { sound: 'step-stone', gain: 0.26, cooldown: 170, synth: 'landing', sg: 0.42, speed: 'impactSpeed', also: [{ synth: 'hollow', sg: 0.16, sr: 0.72, delay: 0.035 }] }
    };

    root.RunenpfadSfx = { SYNTHS: SYNTHS, CUES: CUES, builder: builder, resonanceNotes: resonanceNotes, cueTuning: cueTuning };
})(typeof window !== 'undefined' ? window : globalThis);
