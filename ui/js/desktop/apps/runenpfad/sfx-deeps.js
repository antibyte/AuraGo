/* Runenpfad "deeps" sound recipes (appends to RunenpfadSfx.SYNTHS and RunenpfadSfx.CUES): Brandur the ember wyrm (tells, attacks,
   phase, death, roar), the cave impacts, emberlings and drakelings, the tablets, the bell, the forge and the cooking pot.
   Everything is built from filtered noise and physical models, no plain oscillator beeps. DOM-free and deterministic
   (seeded jitter only). Loads after sfx-rooms.js; does nothing when RunenpfadSfx or its builder is missing. */
(function (root) {
    'use strict';

    const Sfx = root.RunenpfadSfx;
    if (!Sfx || typeof Sfx.builder !== 'function' || !Sfx.ROOM_TOOLS) return;
    const wrap = Sfx.ROOM_TOOLS.wrap;
    const scatter = Sfx.ROOM_TOOLS.scatter;
    const jit = Sfx.ROOM_TOOLS.jit;

    // Brandur's roar body: driven throat growl (two sawtooth voices with a 18 Hz flutter), three breathy vocal-tract formants
    // sweeping down, a high breath hiss and a sub weight. Deeper, wider and breathier than the existing dragon roar.
    function bigRoar(b, t, g, sr, o, d, drop) {
        const bus = b.sat('drive');
        const k = drop || 1;
        const grr = b.o('sawtooth', 74 * sr, 27 * sr * k, t, d, g * 0.36, { filter: 'lowpass', ff: 1250, q: 1.2, atk: 0.1, to: bus });
        b.o('sawtooth', 111 * sr, 41 * sr * k, t + 0.02, d, g * 0.26, { filter: 'lowpass', ff: 1500, q: 1.0, atk: 0.12, to: bus });
        b.lfo(grr.g.gain, 18, g * 0.12, t, d);
        b.n(t, d, g * 0.58, { color: 'pink', f0: 420 * sr, f1: 170 * sr * k, q: 3.2, atk: 0.1, to: bus });
        b.n(t, d, g * 0.5, { color: 'pink', f0: 900 * sr, f1: 360 * sr * k, q: 4, atk: 0.12, to: bus });
        b.n(t + 0.04, d * 0.85, g * 0.38, { color: 'crackle', f0: 1700 * sr, f1: 700 * sr * k, q: 2.6, atk: 0.1, to: bus });
        b.n(t, d * 0.9, g * 0.22, { color: 'white', type: 'highpass', f0: 3000, q: 0.5, atk: 0.2, to: bus });
        b.o('sine', 46 * sr, 22 * sr, t, d, g * 0.3, { atk: 0.12 });
    }

    // Heavy low thump with falling rubble: sub drop, brown body, a dull crack and grit grains.
    function thump(b, t, g, sr, o, grit) {
        const out = b.sat('warm');
        b.o('sine', 84 * sr, 28 * sr, t, 0.42, g * 0.9, { atk: 0.004, to: out });
        b.n(t, 0.5, g * 0.6, { color: 'brown', type: 'lowpass', f0: 520, f1: 110, q: 0.8, atk: 0.006, to: out });
        b.n(t, 0.03, g * 0.5, { color: 'crackle', type: 'bandpass', f0: 1500, q: 1.2, to: out });
        b.n(t + 0.02, 0.45, g * 0.22, { color: 'pink', type: 'bandpass', f0: 380, f1: 160, q: 1.2, atk: 0.02, to: out });
        scatter(b, o, t + 0.05, grit || 14, 0.7, g * 0.55, 800 * sr, 4200 * sr, 2.8, 5, 1.25);
    }

    // Wing beats: n slow airy pulses (band-passed brown plus pink) with a body thump each.
    function beats(b, t, g, sr, o, n, gap, lvl, rise) {
        for (let i = 0; i < n; i++) {
            const at = t + i * gap * (1 - (rise || 0) * i / Math.max(1, n));
            const lv = lvl * (1 - i * 0.08);
            b.n(at, 0.34, g * 0.7 * lv, { color: 'brown', type: 'bandpass', f0: 200 * sr, f1: 640 * sr, q: 0.7, atk: 0.07 });
            b.n(at, 0.3, g * 0.34 * lv, { color: 'pink', type: 'bandpass', f0: 700 * sr, f1: 1700 * sr, q: 0.9, atk: 0.08 });
            b.o('sine', 64 * sr, 38 * sr, at + 0.06, 0.2, g * 0.3 * lv, { atk: 0.02 });
        }
    }

    // One continuous noise source with a sustained envelope (fast attack, flat sustain, smooth release) and an optional filter sweep.
    // The builder's own noise envelope decays at once, which would turn a hiss into a burst.
    function sustained(b, t, d, peak, color, type, f0, f1, q, atk, rel) {
        const ac = b.ac;
        const src = b.keep(ac.createBufferSource());
        src.buffer = (b.dsp && b.dsp.getNoise(ac, color)) || b.noise;
        src.loop = true;
        const f = b.keep(ac.createBiquadFilter());
        f.type = type;
        f.Q.value = q;
        f.frequency.setValueAtTime(f0, t);
        f.frequency.linearRampToValueAtTime(f1, t + d);
        const g = b.keep(ac.createGain());
        g.gain.setValueAtTime(0.0001, t);
        g.gain.linearRampToValueAtTime(peak, t + atk);
        g.gain.setValueAtTime(peak, t + d - rel);
        g.gain.linearRampToValueAtTime(0.0001, t + d);
        src.connect(f);
        f.connect(g);
        g.connect(b.dest);
        src.start(t, 0.3);
        src.stop(t + d + 0.02);
        b.touch(t + d);
    }

    // Steam vent: one continuous pressurised hiss (pink noise high-passed from 1.8 up to 3.2 kHz, 0.08 s attack, sustain, 0.4 s release, no
    // tremolo), a short brown puff at the onset, and at most two deep lava blurps well after the hiss peak.
    function steamBlurps(b, t, g, sr, o, d, n, hiss) {
        sustained(b, t, d, g * 0.9 * hiss, 'pink', 'highpass', 1800 * sr, 3200 * sr, 0.6, 0.08, 0.4);
        sustained(b, t, d, g * 0.3 * hiss, 'white', 'highpass', 4000, 6000, 0.5, 0.1, 0.4);
        b.n(t, 0.15, g * 0.5, { color: 'brown', type: 'lowpass', f0: 600, f1: 200, q: 0.8, atk: 0.005 });
        for (let i = 0; i < Math.min(2, n); i++) {
            const at = t + 0.55 + i * 0.4;
            const f = (90 + jit(o, i + 20) * 40) * sr;
            b.o('sine', f, f * 1.8, at, 0.16, g * 0.35, { atk: 0.015 });
        }
    }

    // Rising build-up helper: a band of noise climbing from f0 to f1 over the tell plus a sub tone.
    function build(b, t, g, d, f0, f1, q, sub0, sub1, color) {
        b.n(t, d, g * 0.55, { color: color || 'pink', f0: f0, f1: f1, q: q, atk: d * 0.6 });
        b.o('sine', sub0, sub1, t, d, g * 0.35, { atk: d * 0.6 });
    }

    const SYNTHS = {
        // Brandur's full roar: huge, deep and breathy, with a second lower layer a hair later for width and rubble raining off the ceiling.
        dp_roar: wrap(function (b, t, g, sr, o) {
            bigRoar(b, t, g, sr, o, 2.3, 1);
            bigRoar(b, t + 0.14, g * 0.5, sr * 0.7, o, 2.0, 0.9);
            scatter(b, o, t + 0.4, 10, 1.6, g * 0.3, 900, 3800, 2.4, 0, 1.1);
        }),
        // Short growl that colours other cues as Brandur's voice: a low throat burst with breath.
        dp_v_brandur: wrap(function (b, t, g, sr, o) {
            bigRoar(b, t, g * 0.7, sr * 1.05, o, 0.7, 0.8);
        }),
        // Roar tell: a deep chest inhale, a creaking throat and a rising growl before the roar.
        dp_t_roar: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.85, g * 0.5, { color: 'brown', type: 'bandpass', f0: 160 * sr, f1: 800 * sr, q: 1.4, atk: 0.5 });
            b.o('sawtooth', 40 * sr, 78 * sr, t, 0.85, g * 0.3, { filter: 'lowpass', ff: 260, atk: 0.55 });
            b.n(t + 0.1, 0.7, g * 0.2, { color: 'pink', type: 'highpass', f0: 2200, f1: 4200, q: 0.6, atk: 0.5 });
            for (let i = 0; i < 5; i++) b.n(t + 0.1 + i * 0.13 + jit(o, i) * 0.04, 0.03, g * 0.25, { color: 'crackle', type: 'bandpass', f0: 500 + jit(o, i + 5) * 300, q: 3 });
        }),
        // Gas-jet breath: a roaring pink and brown jet whose band climbs from 700 Hz to 2.6 kHz, a 13 Hz flicker, a hissing top, an ignition whomp and 22 crackles.
        dp_a_breath: wrap(function (b, t, g, sr, o) {
            const d = 1.35;
            const out = b.sat('warm');
            const jet = b.n(t, d, g * 0.75, { color: 'pink', f0: 700 * sr, f1: 2600 * sr, q: 0.9, atk: 0.09, to: out });
            b.lfo(jet.gain, 13, g * 0.18, t, d);
            b.n(t, d, g * 0.6, { color: 'brown', type: 'lowpass', f0: 420 * sr, f1: 300 * sr, q: 0.8, atk: 0.1, to: out });
            b.n(t + 0.05, d * 0.9, g * 0.28, { color: 'white', type: 'highpass', f0: 3600, q: 0.6, atk: 0.15, to: out });
            b.o('sine', 76 * sr, 40 * sr, t, 0.35, g * 0.5, { atk: 0.01, to: out });
            for (let i = 0; i < 22; i++) {
                const at = t + 0.08 + Math.pow(jit(o, i), 1.1) * (d - 0.2);
                b.n(at, 0.012 + jit(o, i + 30) * 0.014, g * (0.18 + jit(o, i + 60) * 0.4), { color: 'crackle', type: 'bandpass', f0: 1200 + jit(o, i + 90) * 5000, q: 1.6, to: out });
            }
        }),
        // Breath tell: a sucked-in rising band (300 Hz to 1.9 kHz), a glowing hum, embers ticking faster and faster.
        dp_t_breath: wrap(function (b, t, g, sr, o) {
            build(b, t, g, 0.85, 300 * sr, 1900 * sr, 4, 60 * sr, 120 * sr, 'pink');
            b.n(t + 0.2, 0.6, g * 0.18, { color: 'white', type: 'highpass', f0: 4000, q: 0.6, atk: 0.4 });
            for (let i = 0; i < 12; i++) {
                const at = t + 0.85 * (1 - Math.pow(1 - i / 11, 1.6));
                b.n(at, 0.014, g * (0.2 + i * 0.025), { color: 'crackle', type: 'bandpass', f0: 1800 + jit(o, i) * 3000, q: 2 });
            }
        }),
        // Tail sweep: a creak in the tail, a rising whip of air, then a dry crack on top of a low thud with a few chips of stone.
        dp_a_tail: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.3, g * 0.8, { color: 'pink', f0: 450 * sr, f1: 3200 * sr, q: 1.2, atk: 0.12 });
            b.n(t, 0.35, g * 0.35, { color: 'brown', type: 'lowpass', f0: 300, f1: 160, q: 0.8, atk: 0.1 });
            const at = t + 0.28;
            b.n(at, 0.03, g * 0.9, { color: 'crackle', type: 'highpass', f0: 2400 });
            b.o('sine', 96 * sr, 40 * sr, at, 0.25, g * 0.7, { atk: 0.003 });
            b.n(at, 0.3, g * 0.4, { color: 'brown', type: 'lowpass', f0: 600, f1: 140, q: 0.8, atk: 0.004 });
            scatter(b, o, at + 0.02, 7, 0.4, g * 0.45, 1200, 4800, 3, 8, 1.3);
        }),
        dp_t_tail: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.75, g * 0.4, { color: 'brown', type: 'bandpass', f0: 140 * sr, f1: 320 * sr, q: 2.5, atk: 0.3 });
            b.o('sawtooth', 60 * sr, 120 * sr, t, 0.75, g * 0.12, { filter: 'bandpass', ff: 220, q: 4, atk: 0.4 });
            b.n(t + 0.2, 0.55, g * 0.45, { color: 'pink', type: 'bandpass', f0: 500 * sr, f1: 1500 * sr, q: 1.5, atk: 0.4 });
            for (let i = 0; i < 4; i++) b.n(t + 0.05 + i * 0.15 + jit(o, i) * 0.05, 0.05, g * 0.22, { color: 'brown', type: 'bandpass', f0: 250 + jit(o, i + 9) * 150, q: 4 });
        }),
        // Stomp attack and cave landing: the heavy thump with rubble (the landing is deeper with more grit and a dust hiss).
        dp_a_stomp: wrap(function (b, t, g, sr, o) { thump(b, t, g, sr, o, 14); }),
        dp_land: wrap(function (b, t, g, sr, o) {
            thump(b, t, g, sr * 0.8, o, 24);
            b.n(t + 0.05, 0.9, g * 0.3, { color: 'pink', type: 'highpass', f0: 1800, f1: 900, q: 0.6, atk: 0.1 });
            b.model('anvil', t, 0.28, g * 0.2, 0.5);
        }),
        // Stomp tell: a rising floor rumble with pebbles dancing off the ground faster and faster.
        dp_t_stomp: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.8, g * 0.7, { color: 'brown', type: 'lowpass', f0: 90 * sr, f1: 340 * sr, q: 1.1, atk: 0.55 });
            b.o('sine', 38 * sr, 66 * sr, t, 0.8, g * 0.5, { atk: 0.55 });
            for (let i = 0; i < 14; i++) {
                const at = t + 0.8 * (1 - Math.pow(1 - i / 13, 1.5));
                b.n(at, 0.02, g * (0.15 + i * 0.02), { color: 'crackle', type: 'bandpass', f0: 900 + jit(o, i) * 2400, q: 3 });
            }
        }),
        // Takeoff: four heavy wing beats that speed up, dust blown out sideways, a low whoomp under each.
        dp_a_takeoff: wrap(function (b, t, g, sr, o) {
            beats(b, t, g, sr, o, 4, 0.26, 1, 0.3);
            b.n(t, 1.1, g * 0.3, { color: 'pink', type: 'bandpass', f0: 900, f1: 2600, q: 0.7, atk: 0.4 });
            scatter(b, o, t + 0.1, 8, 0.9, g * 0.25, 1500, 5000, 2, 15, 1);
        }),
        dp_t_takeoff: wrap(function (b, t, g, sr, o) {
            beats(b, t + 0.05, g * 0.6, sr, o, 2, 0.32, 0.8, 0);
            b.n(t, 0.8, g * 0.35, { color: 'brown', type: 'lowpass', f0: 120 * sr, f1: 320 * sr, q: 1, atk: 0.5 });
            b.o('sawtooth', 45 * sr, 80 * sr, t, 0.8, g * 0.15, { filter: 'lowpass', ff: 220, atk: 0.5 });
        }),
        // Wing buffet: a huge airy whoosh in three washes, the band falling away as it passes (pink 300 to 1.4 kHz, white top, brown push).
        dp_a_buffet: wrap(function (b, t, g, sr, o) {
            [0, 0.3, 0.6].forEach(function (dt, i) {
                const lv = 1 - i * 0.25;
                b.n(t + dt, 0.55, g * 0.8 * lv, { color: 'pink', f0: 280 * sr, f1: 1300 * sr, q: 0.7, atk: 0.12 });
                b.n(t + dt, 0.5, g * 0.4 * lv, { color: 'white', type: 'highpass', f0: 2400, f1: 1200, q: 0.5, atk: 0.14 });
                b.n(t + dt, 0.4, g * 0.6 * lv, { color: 'brown', type: 'lowpass', f0: 230, f1: 120, q: 0.8, atk: 0.1 });
                b.o('sine', 58 * sr, 36 * sr, t + dt + 0.1, 0.25, g * 0.3 * lv, { atk: 0.03 });
            });
        }),
        dp_t_buffet: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.8, g * 0.5, { color: 'pink', f0: 500 * sr, f1: 2400 * sr, q: 1.6, atk: 0.55 });
            b.n(t, 0.8, g * 0.3, { color: 'white', type: 'highpass', f0: 3000, q: 0.5, atk: 0.55 });
            beats(b, t + 0.1, g * 0.4, sr, o, 2, 0.34, 0.6, 0);
        }),
        // Vents: a strong steam hiss with four deep lava blurps.
        dp_a_vents: wrap(function (b, t, g, sr, o) { steamBlurps(b, t, g, sr, o, 1.3, 2, 1); }),
        dp_t_vents: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.8, g * 0.5, { color: 'pink', type: 'highpass', f0: 1400 * sr, f1: 3600 * sr, q: 0.6, atk: 0.55 });
            b.o('sine', 70 * sr, 130 * sr, t, 0.8, g * 0.3, { atk: 0.5 });
            for (let i = 0; i < 4; i++) {
                const at = t + 0.1 + i * 0.17 + jit(o, i) * 0.04;
                b.o('sine', (140 + jit(o, i + 3) * 120) * sr, 380 * sr, at, 0.07, g * 0.35, { atk: 0.006 });
                b.n(at + 0.01, 0.06, g * 0.25, { color: 'brown', type: 'bandpass', f0: 240, q: 3 });
            }
        }),
        // Phase change: the roar with a boom under it.
        dp_phase: wrap(function (b, t, g, sr, o) {
            bigRoar(b, t, g, sr, o, 1.5, 0.9);
            b.o('sine', 70 * sr, 26 * sr, t, 0.6, g * 0.5, { atk: 0.004 });
            scatter(b, o, t + 0.1, 8, 0.9, g * 0.3, 900, 4000, 2.5, 0, 1.2);
        }),
        // Death: a long falling roar fading into a wheeze, the ground booming twice, rubble pouring down, embers dying away.
        dp_death: wrap(function (b, t, g, sr, o) {
            bigRoar(b, t, g, sr * 0.85, o, 3.0, 0.5);
            [0.5, 1.6].forEach(function (dt, i) {
                b.o('sine', 70 * sr, 24 * sr, t + dt, 0.6, g * (0.7 - i * 0.2), { atk: 0.005 });
                b.n(t + dt, 0.7, g * 0.4, { color: 'brown', type: 'lowpass', f0: 600, f1: 100, q: 0.8, atk: 0.006 });
            });
            scatter(b, o, t + 0.5, 22, 2.2, g * 0.4, 700, 4200, 2.4, 0, 1);
            b.n(t + 2.2, 1.0, g * 0.18, { color: 'pink', type: 'lowpass', f0: 2600, f1: 300, q: 0.8, atk: 0.1 });
            for (let i = 0; i < 10; i++) b.n(t + 2.0 + i * 0.12 + jit(o, i) * 0.06, 0.014, g * 0.15, { color: 'crackle', type: 'bandpass', f0: 2000 + jit(o, i + 9) * 3000, q: 2 });
        }),
        // The sleeper stirring: a vast rumble swelling out of silence, stone shifting, a low breathy growl and a long sigh.
        dp_wake: wrap(function (b, t, g, sr, o) {
            b.n(t, 2.3, g * 0.7, { color: 'brown', type: 'lowpass', f0: 70 * sr, f1: 220 * sr, q: 1, atk: 1.0 });
            b.o('sine', 28 * sr, 52 * sr, t, 2.3, g * 0.5, { atk: 1.0 });
            scatter(b, o, t + 0.3, 14, 1.9, g * 0.35, 800, 3500, 2.2, 0, 1);
            const grr = b.o('sawtooth', 52 * sr, 36 * sr, t + 0.9, 1.4, g * 0.3, { filter: 'lowpass', ff: 380, q: 1.1, atk: 0.5 });
            b.lfo(grr.g.gain, 14, g * 0.1, t + 0.9, 1.4);
            b.n(t + 1.5, 0.9, g * 0.3, { color: 'pink', type: 'bandpass', f0: 600 * sr, f1: 250 * sr, q: 1.5, atk: 0.3 });
        }),
        // A hit on the scales while he is awake: a dull plated clank (an anvil model well below pitch), a glint of high ring, two pebbles.
        dp_scale: wrap(function (b, t, g, sr, o) {
            b.model('anvil', t, 0.9 * sr, g * 0.5, 0.35);
            b.n(t, 0.014, g * 0.7, { color: 'crackle', type: 'highpass', f0: 2600 });
            b.o('sine', 2960 * sr, 0, t + 0.004, 0.14, g * 0.07, { atk: 0.001 });
            b.n(t, 0.08, g * 0.35, { color: 'brown', type: 'lowpass', f0: 500, f1: 200, q: 0.8, atk: 0.003 });
            scatter(b, o, t + 0.03, 3, 0.1, g * 0.3, 2000, 5000, 3, 12, 1);
        }),
        // Drowsy on: a purring rumble (brown noise and a low saw, both chopped by a 21 Hz flutter) that breathes slowly.
        dp_drowsy: wrap(function (b, t, g, sr, o) {
            const d = 1.8;
            const purr = b.n(t, d, g * 0.6, { color: 'brown', type: 'lowpass', f0: 150 * sr, f1: 110 * sr, q: 1.2, atk: 0.5 });
            b.lfo(purr.gain, 21, g * 0.3, t, d);
            const s = b.o('sawtooth', 52 * sr, 46 * sr, t, d, g * 0.25, { filter: 'lowpass', ff: 170, q: 1.0, atk: 0.5 });
            b.lfo(s.g.gain, 19, g * 0.12, t, d);
            b.n(t + 0.3, d - 0.4, g * 0.1, { color: 'pink', type: 'bandpass', f0: 380, f1: 260, q: 1.5, atk: 0.6 });
        }),
        // Drowsy off: a short snort of breath and a startled low rumble.
        dp_drowsy_end: wrap(function (b, t, g, sr) {
            b.n(t, 0.3, g * 0.6, { color: 'pink', f0: 1000 * sr, f1: 320 * sr, q: 1.6, atk: 0.03 });
            b.n(t, 0.5, g * 0.5, { color: 'brown', type: 'lowpass', f0: 300, f1: 110, q: 0.9, atk: 0.04 });
            b.o('sine', 70 * sr, 46 * sr, t + 0.05, 0.4, g * 0.35, { atk: 0.05 });
        }),
        // Falling stalactite: the stone cracking into the floor (sharp noise crack, dull thud, 12 bright shards skittering), no whistle.
        dp_rock: wrap(function (b, t, g, sr, o) {
            const out = b.sat('warm');
            b.n(t, 0.05, g * 0.9, { color: 'white', type: 'bandpass', f0: 2600 * sr, q: 0.6, atk: 0.001, to: out });
            b.n(t, 0.012, g * 0.8, { color: 'crackle', type: 'highpass', f0: 2800, to: out });
            b.o('sine', 120 * sr, 46 * sr, t, 0.22, g * 0.8, { atk: 0.003, to: out });
            b.n(t, 0.35, g * 0.5, { color: 'brown', type: 'lowpass', f0: 800, f1: 160, q: 0.8, atk: 0.004, to: out });
            scatter(b, o, t + 0.03, 12, 0.5, g * 0.6, 1800 * sr, 7000 * sr, 3.5, 0, 1.2);
        }),
        // Fire rain: a fine hissing patter of embers (30 tiny bright ticks over 1.3 s) over a soft sizzle.
        dp_embers: wrap(function (b, t, g, sr, o) {
            b.n(t, 1.3, g * 0.28, { color: 'pink', type: 'highpass', f0: 3500, q: 0.6, atk: 0.15 });
            b.n(t, 1.2, g * 0.15, { color: 'brown', type: 'lowpass', f0: 450, f1: 200, q: 0.8, atk: 0.2 });
            for (let i = 0; i < 30; i++) {
                const at = t + Math.pow(jit(o, i), 1.2) * 1.2;
                b.n(at, 0.01 + jit(o, i + 30) * 0.012, g * (0.25 + jit(o, i + 60) * 0.55), { color: 'crackle', type: 'bandpass', f0: 2500 + jit(o, i + 90) * 5000, q: 2.2 });
            }
        }),
        // Pillar ignite: a soft blue whomp (low swell, no crackle) with a glassy shimmer on top and a quiet bell.
        dp_pillar: wrap(function (b, t, g, sr, o) {
            const out = b.sat('warm');
            b.o('sine', 100 * sr, 58 * sr, t, 0.5, g * 0.6, { atk: 0.09, to: out });
            b.n(t, 0.6, g * 0.55, { color: 'brown', type: 'lowpass', f0: 1000, f1: 260, q: 0.7, atk: 0.1, to: out });
            b.n(t + 0.05, 0.9, g * 0.12, { color: 'pink', type: 'bandpass', f0: 2600, f1: 1500, q: 0.8, atk: 0.2, to: out });
            [2.2, 3.1, 4.4].forEach(function (r, i) { b.model('glass', t + 0.08 + i * 0.06, r * sr, g * (0.2 - i * 0.04), 0.9); });
            b.bell(880 * sr, t + 0.12, 0.8, g * 0.07);
        }),
        // Emberling tell: a hot sizzle, a tiny rising "tsss" and six sparks ticking faster.
        dp_ember_tell: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.4, g * 0.45, { color: 'white', type: 'highpass', f0: 3200 * sr, f1: 6200 * sr, q: 0.6, atk: 0.22 });
            for (let i = 0; i < 6; i++) b.n(t + 0.4 * (1 - Math.pow(1 - i / 5, 1.5)), 0.012, g * 0.5, { color: 'crackle', type: 'bandpass', f0: 3000 + jit(o, i) * 3000, q: 2 });
            b.n(t, 0.35, g * 0.2, { color: 'brown', type: 'lowpass', f0: 300, q: 0.8, atk: 0.2 });
        }),
        // Emberling hop: a sizzling spit of sparks, a soft thud of landing and a quick hiss.
        dp_ember_hop: wrap(function (b, t, g, sr, o) {
            b.n(t, 0.25, g * 0.5, { color: 'pink', f0: 1500 * sr, f1: 4000 * sr, q: 1.2, atk: 0.02 });
            b.o('sine', 210 * sr, 120 * sr, t + 0.02, 0.1, g * 0.35, { atk: 0.004 });
            b.n(t + 0.16, 0.1, g * 0.4, { color: 'brown', type: 'lowpass', f0: 500, f1: 180, q: 0.8, atk: 0.004 });
            scatter(b, o, t, 8, 0.3, g * 0.45, 2500, 7000, 2, 20, 1.1);
        }),
        // Drakeling tell: a thin rising screech, a bright vocal formant swept from 1 to 2.2 kHz, a hint of rasp.
        dp_drake_tell: wrap(function (b, t, g, sr, o) {
            const s = b.o('sawtooth', 780 * sr, 1500 * sr, t, 0.5, g * 0.12, { filter: 'bandpass', ff: 1800, q: 3, atk: 0.2 });
            b.lfo(s.osc.frequency, 9, 40, t, 0.5);
            b.n(t, 0.5, g * 0.45, { color: 'pink', f0: 1000 * sr, f1: 2300 * sr, q: 4.5, atk: 0.25 });
            b.n(t + 0.1, 0.35, g * 0.2, { color: 'crackle', type: 'highpass', f0: 3600, q: 0.8, atk: 0.2 });
        }),
        // Drakeling attack: a short sharp screech, then a wet spit of fire (a hiss with crackles).
        dp_drake_attack: wrap(function (b, t, g, sr, o) {
            b.o('sawtooth', 1500 * sr, 900 * sr, t, 0.22, g * 0.1, { filter: 'bandpass', ff: 2200, q: 3, atk: 0.01 });
            b.n(t, 0.2, g * 0.5, { color: 'pink', f0: 2600 * sr, f1: 1200 * sr, q: 4, atk: 0.01 });
            b.n(t + 0.1, 0.3, g * 0.55, { color: 'pink', f0: 900 * sr, f1: 2800 * sr, q: 1.2, atk: 0.02 });
            b.n(t + 0.1, 0.25, g * 0.25, { color: 'white', type: 'highpass', f0: 4000, q: 0.6, atk: 0.03 });
            scatter(b, o, t + 0.1, 8, 0.3, g * 0.4, 1500, 6000, 2, 30, 1);
        }),
        // Tablet: a stone slab scraping over stone (brown band chopped at 30 and 41 Hz, gritty 3 kHz) then a soft celesta chime.
        dp_tablet: wrap(function (b, t, g, sr, o) {
            const body = b.n(t, 0.38, g * 0.6, { color: 'brown', type: 'bandpass', f0: 380, f1: 300, q: 0.9, atk: 0.04 });
            b.lfo(body.gain, 30, g * 0.3, t, 0.38);
            b.lfo(body.gain, 41, g * 0.2, t, 0.38);
            b.n(t, 0.35, g * 0.15, { color: 'crackle', type: 'bandpass', f0: 3000, q: 1.5, atk: 0.05 });
            scatter(b, o, t + 0.03, 5, 0.3, g * 0.25, 900, 3500, 2, 10, 1);
            b.n(t + 0.34, 0.1, g * 0.3, { color: 'brown', type: 'lowpass', f0: 400, f1: 200, q: 0.7, atk: 0.004 });
            [783.99, 1174.7].forEach(function (f, i) {
                b.bell(f * sr, t + 0.4 + i * 0.11, 0.9, g * 0.3);
                b.model('harp', t + 0.4 + i * 0.11, (f * 0.5 * sr) / 261.6256, g * 0.15, 0.6);
            });
        }),
        // Cracked bell: a split clang (a dark bell model with a sour second partial beating against it), 22 buzzing rattle ticks and a dull anvil clunk. No sustain.
        dp_bell_cracked: wrap(function (b, t, g, sr, o) {
            const out = b.sat('warm');
            b.model('darkbell', t, 0.95 * sr, g * 0.7, 0.8, { to: out });
            b.o('sine', 523 * sr, 0, t, 0.5, g * 0.12, { atk: 0.002, to: out });
            b.o('sine', 547 * sr, 0, t, 0.4, g * 0.12, { atk: 0.002, to: out });
            b.model('anvil', t, 0.55 * sr, g * 0.35, 0.3, { to: out });
            b.n(t, 0.02, g * 0.8, { color: 'crackle', type: 'highpass', f0: 2200, to: out });
            for (let i = 0; i < 22; i++) {
                const at = t + 0.03 + Math.pow(jit(o, i), 1.4) * 0.8;
                b.n(at, 0.014 + jit(o, i + 30) * 0.01, g * (0.2 + jit(o, i + 60) * 0.4) * (1 - (at - t) / 1.1), { color: 'crackle', type: 'bandpass', f0: 1500 + jit(o, i + 90) * 2200, q: 4, to: out });
            }
        }),
        // Mended bell: a big bronze bell struck low (about 170 Hz). Inharmonic partials as in a church bell (hum an octave below, prime,
        // minor-third tierce, fifth, nominal, upper partials), each slightly detuned for beating, the higher ones dying first over about 5 s;
        // a sharp metallic clapper transient and two quiet room echoes. No plucked or synthetic tone.
        dp_bell_toll: wrap(function (b, t, g, sr, o) {
            const f = 170 * sr;
            const P = [[0.5, 1.0, 5.5], [1.0, 0.8, 5.0], [1.2, 0.7, 4.0], [1.5, 0.4, 3.0], [2.0, 0.6, 3.5], [2.5, 0.25, 2.2], [3.0, 0.3, 1.8], [4.0, 0.2, 1.2], [5.4, 0.12, 0.8]];
            [0, 0.085, 0.17].forEach(function (dly, k) {
                const lv = k === 0 ? 1 : 0.16 / k;
                P.forEach(function (p, i) {
                    const det = 1 + (jit(o, i + k * 11) - 0.5) * 0.006;
                    b.o('sine', f * p[0] * det, 0, t + dly, p[2] * (k ? 0.7 : 1), g * 0.3 * p[1] * lv, { atk: 0.002 });
                    if (k === 0 && i > 0 && i < 6) b.o('sine', f * p[0] * (det + 0.0035), 0, t, p[2] * 0.8, g * 0.12 * p[1], { atk: 0.002 });
                });
            });
            b.model('darkbell', t, 0.75 * sr, g * 0.35, 3.0);
            b.n(t, 0.03, g * 0.9, { color: 'crackle', type: 'highpass', f0: 2200 });
            b.n(t, 0.12, g * 0.5, { color: 'white', type: 'bandpass', f0: 3400, q: 1.2, atk: 0.001 });
            b.model('anvil', t, 0.5 * sr, g * 0.2, 0.4);
        }),
        // Slain: the last breath (a long falling wheeze), a final deep boom and cinders crackling out.
        dp_slain: wrap(function (b, t, g, sr, o) {
            b.n(t, 1.8, g * 0.5, { color: 'pink', type: 'lowpass', f0: 1800, f1: 140, q: 1.0, atk: 0.05 });
            b.o('sine', 64 * sr, 22 * sr, t + 0.2, 1.0, g * 0.7, { atk: 0.01 });
            b.n(t + 0.2, 1.1, g * 0.5, { color: 'brown', type: 'lowpass', f0: 500, f1: 90, q: 0.8, atk: 0.01 });
            scatter(b, o, t + 0.25, 14, 1.4, g * 0.35, 900, 4200, 2.4, 0, 1.1);
            for (let i = 0; i < 12; i++) b.n(t + 0.5 + i * 0.16 + jit(o, i) * 0.08, 0.014, g * 0.18, { color: 'crackle', type: 'bandpass', f0: 1800 + jit(o, i + 9) * 3000, q: 2 });
        }),
        // Sealed: a deep stone latch and a warm glass chord rising over a breathing hum, ending in two soft bells.
        dp_sealed: wrap(function (b, t, g, sr, o) {
            b.o('sine', 80 * sr, 40 * sr, t, 0.5, g * 0.7, { atk: 0.006 });
            b.n(t, 0.5, g * 0.4, { color: 'brown', type: 'lowpass', f0: 500, f1: 120, q: 0.8, atk: 0.006 });
            b.n(t + 0.2, 1.8, g * 0.3, { color: 'pink', type: 'bandpass', f0: 500, f1: 900, q: 2, atk: 0.8 });
            [261.6, 392, 523.3, 784].forEach(function (f, i) { b.model('glass', t + 0.3 + i * 0.12, (f * sr) / 523.251, g * 0.22, 1.6); });
            [523.25, 783.99].forEach(function (f, i) { b.bell(f * sr, t + 1.0 + i * 0.2, 1.4, g * 0.2); });
        }),
        // Brandur's blessing: a warm glass chord blooming over a soft swell of air, one high bell.
        dp_blessing: wrap(function (b, t, g, sr, o) {
            b.n(t, 1.1, g * 0.3, { color: 'pink', type: 'bandpass', f0: 500, f1: 1100, q: 1.5, atk: 0.5 });
            [392, 523.3, 659.3, 1046.5].forEach(function (f, i) { b.model('glass', t + i * 0.07, (f * sr) / 523.251, g * 0.26, 1.4); });
            b.bell(1568 * sr, t + 0.3, 1.0, g * 0.12);
        }),
        // Forge strike, good: a clean anvil ring (the anvil model at two pitches, three inharmonic ring partials), a bright spark hiss.
        dp_strike_good: wrap(function (b, t, g, sr, o) {
            b.model('anvil', t, 1.0 * sr, g * 0.8, 1.0);
            b.model('anvil', t + 0.01, 1.5 * sr, g * 0.28, 0.6);
            [1850, 2630, 3470].forEach(function (f, i) { b.o('sine', f * sr * (1 + (jit(o, i) - 0.5) * 0.01), 0, t, 0.6 - i * 0.12, g * 0.1, { atk: 0.001 }); });
            b.n(t, 0.02, g * 0.8, { color: 'crackle', type: 'highpass', f0: 2500 });
            b.o('sine', 110 * sr, 60 * sr, t, 0.12, g * 0.4, { atk: 0.003 });
            scatter(b, o, t + 0.02, 7, 0.3, g * 0.3, 3500, 8000, 2, 50, 1.2);
        }),
        // Forge strike, bad: a dull clank (the anvil model far below pitch through a 900 Hz low pass, no ring), a thud and a flat knock.
        dp_strike_bad: wrap(function (b, t, g, sr) {
            b.model('anvil', t, 0.45 * sr, g * 0.7, 0.25, { filter: 'lowpass', ff: 900, q: 0.7 });
            b.n(t, 0.05, g * 0.5, { color: 'brown', type: 'bandpass', f0: 600 * sr, q: 1.2, atk: 0.002 });
            b.o('sine', 95 * sr, 50 * sr, t, 0.14, g * 0.6, { atk: 0.003 });
            b.n(t, 0.1, g * 0.3, { color: 'brown', type: 'lowpass', f0: 500, f1: 200, q: 0.8, atk: 0.003 });
        }),
        // Cooking: a pan sizzle (pink highpass hiss, 28 crackles) with a few soft bubble pops rising out of the stew.
        dp_cook: wrap(function (b, t, g, sr, o) {
            b.n(t, 1.2, g * 0.4, { color: 'pink', type: 'highpass', f0: 3200, q: 0.6, atk: 0.12 });
            b.n(t, 1.1, g * 0.2, { color: 'brown', type: 'lowpass', f0: 500, f1: 300, q: 0.8, atk: 0.2 });
            for (let i = 0; i < 28; i++) {
                const at = t + 0.05 + jit(o, i) * 1.0;
                b.n(at, 0.01 + jit(o, i + 30) * 0.012, g * (0.15 + jit(o, i + 60) * 0.4), { color: 'crackle', type: 'bandpass', f0: 1500 + jit(o, i + 90) * 5000, q: 1.8 });
            }
            for (let i = 0; i < 6; i++) {
                const at = t + 0.2 + i * 0.15 + jit(o, i + 5) * 0.08;
                const f = (170 + jit(o, i + 50) * 200) * sr;
                b.o('sine', f, f * 2.1, at, 0.06, g * 0.3, { atk: 0.006 });
            }
        }),
        // Craft done, ok: a bright finishing ting (two bells and a coin sparkle) over a soft anvil tap.
        dp_done_ok: wrap(function (b, t, g, sr, o) {
            b.model('anvil', t, 1.2 * sr, g * 0.25, 0.3);
            [659.25, 987.77, 1318.5].forEach(function (f, i) {
                b.bell(f * sr, t + 0.04 + i * 0.08, 0.8, g * 0.3);
                b.model('harp', t + 0.04 + i * 0.08, (f * 0.5 * sr) / 261.6256, g * 0.18, 0.5);
            });
            b.model('coin', t + 0.2, 1.6 * sr, g * 0.2, 0.3);
        }),
        // Craft done, failed: a dull sag (a falling low tone, a wooden thud and a flat tick).
        dp_done_fail: wrap(function (b, t, g, sr) {
            b.o('sine', 260 * sr, 110 * sr, t, 0.35, g * 0.4, { atk: 0.01 });
            b.n(t, 0.2, g * 0.5, { color: 'brown', type: 'lowpass', f0: 600, f1: 200, q: 0.8, atk: 0.004 });
            b.n(t, 0.014, g * 0.4, { color: 'crackle', type: 'bandpass', f0: 1200, q: 2 });
        })
    };
    Object.keys(SYNTHS).forEach(function (k) { Sfx.SYNTHS[k] = SYNTHS[k]; });

    // ---- guardian table: Brandur's pitch, signature move and the seven moves he uses ----
    const MOVES = ['breath', 'tail', 'stomp', 'takeoff', 'buffet', 'roar', 'vents'];
    if (Sfx.ROOM_GUARDIANS) Sfx.ROOM_GUARDIANS.brandur = { pitch: 0.5, main: 'breath', moves: { breath: 'breath', tail: 'tail', stomp: 'stomp', takeoff: 'takeoff', buffet: 'buffet', roar: 'roar', vents: 'vents' } };

    // Per move: [tell synth, tell sg, tell sr], [attack synth, attack sg, attack sr, extra layers].
    const TELLS = {
        breath: { synth: 'dp_t_breath', sg: 0.5 }, tail: { synth: 'dp_t_tail', sg: 1.2 }, stomp: { synth: 'dp_t_stomp', sg: 0.45 },
        takeoff: { synth: 'dp_t_takeoff', sg: 0.65 }, buffet: { synth: 'dp_t_buffet', sg: 0.7 }, roar: { synth: 'dp_t_roar', sg: 0.6 }, vents: { synth: 'dp_t_vents', sg: 0.5 }
    };
    const ATTACKS = {
        breath: { synth: 'dp_a_breath', sg: 0.8, also: [{ synth: 'dp_v_brandur', sg: 0.3 }] },
        tail: { synth: 'dp_a_tail', sg: 0.8, also: [{ synth: 'dp_v_brandur', sg: 0.2, delay: 0.1 }] },
        stomp: { synth: 'dp_a_stomp', sg: 0.9 },
        takeoff: { synth: 'dp_a_takeoff', sg: 0.8, also: [{ synth: 'dp_v_brandur', sg: 0.25 }] },
        buffet: { synth: 'dp_a_buffet', sg: 0.8 },
        roar: { synth: 'dp_roar', sg: 0.7 },
        vents: { synth: 'dp_a_vents', sg: 0.8 }
    };
    function addPatterns(type, table) {
        const cue = Sfx.CUES[type];
        if (!cue || !cue.by || !cue.by.map) return;
        MOVES.forEach(function (m) { cue.by.map['brandur_' + m] = Object.assign({}, table[m]); });
        cue.by.map.brandur = Object.assign({}, table[MOVES[0]]);
    }
    addPatterns('in_boss_tell', TELLS);
    addPatterns('in_boss_attack', ATTACKS);
    if (Sfx.CUES.in_boss_phase) Sfx.CUES.in_boss_phase.by.map.brandur = { synth: 'dp_phase', sg: 0.6, sr: 1, also: [] };
    if (Sfx.CUES.in_boss_die) Sfx.CUES.in_boss_die.by.map.brandur = { synth: 'dp_death', sg: 0.7, sr: 1, also: [{ synth: 'rm_collapse', sg: 0.2, sr: 0.8, delay: 0.9 }] };

    // ---- mob voices (merged into the in_mob_tell and in_mob_attack kind tables by audio.js) ----
    Sfx.MOB_VOICES = {
        tell: { emberling: { synth: 'dp_ember_tell', sg: 0.5, sr: 1 }, drakeling: { synth: 'dp_drake_tell', sg: 1.0, sr: 1 } },
        attack: { emberling: { synth: 'dp_ember_hop', sg: 0.55, sr: 1 }, drakeling: { synth: 'dp_drake_attack', sg: 0.55, sr: 1 } }
    };

    // ---- cues for the new events (every one carries a TRIM key in audio-mix.js) ----
    const CUES = {
        in_tablet: { sound: false, gain: 0.5, cooldown: 300, synth: 'dp_tablet', sg: 0.7, by: { key: 'first', map: { true: { also: [{ synth: 'shimmer', sg: 0.18, sr: 1.4, delay: 0.5 }] }, false: { sr: 0.9 }, '*': {} } } },
        dr_bell: { sound: false, gain: 0.7, cooldown: 600, synth: 'dp_bell_toll', sg: 0.8, by: { key: 'mended', map: { true: { synth: 'dp_bell_toll' }, false: { synth: 'dp_bell_cracked' }, '*': { synth: 'dp_bell_cracked' } } } },
        dr_outcome: { sound: false, gain: 0.8, cooldown: 1000, synth: 'dp_slain', sg: 0.8, by: { key: 'fate', map: { slain: { synth: 'dp_slain' }, sealed: { synth: 'dp_sealed' }, '*': { synth: 'dp_sealed' } } } },
        dr_blessing: { sound: false, gain: 0.6, cooldown: 800, synth: 'dp_blessing', sg: 0.8 },
        dr_wake: { sound: false, gain: 0.8, cooldown: 1500, synth: 'dp_wake', sg: 0.8, also: [{ synth: 'dp_roar', sg: 0.3, sr: 0.8, delay: 1.4 }] },
        dr_scale: { sound: false, gain: 0.5, cooldown: 120, synth: 'dp_scale', sg: 0.8 },
        dr_drowsy: { sound: false, gain: 0.5, cooldown: 600, synth: 'dp_drowsy', sg: 0.8, by: { key: 'on', map: { true: { synth: 'dp_drowsy' }, false: { synth: 'dp_drowsy_end' }, '*': { synth: 'dp_drowsy_end' } } } },
        dr_impact: { sound: false, gain: 0.7, cooldown: 120, synth: 'dp_rock', sg: 0.8, by: { key: 'kind', map: { stalactite: { synth: 'dp_rock' }, rain: { synth: 'dp_embers', sg: 1.4, cooldown: 400 }, land: { synth: 'dp_land' }, vent: { synth: 'dp_a_vents', sr: 1.15, sg: 0.6 }, '*': { synth: 'dp_rock' } } } },
        craft_cook: { sound: false, gain: 0.5, cooldown: 500, synth: 'dp_cook', sg: 0.8 },
        craft_strike: { sound: false, gain: 0.6, cooldown: 120, synth: 'dp_strike_good', sg: 0.8, by: { key: 'quality', map: { good: { synth: 'dp_strike_good' }, bad: { synth: 'dp_strike_bad' }, '*': { synth: 'dp_strike_bad' } } } },
        craft_done: { sound: false, gain: 0.6, cooldown: 400, synth: 'dp_done_ok', sg: 0.8, by: { key: 'ok', map: { true: { synth: 'dp_done_ok' }, false: { synth: 'dp_done_fail' }, '*': { synth: 'dp_done_ok' } } } }
    };
    Object.keys(CUES).forEach(function (k) { Sfx.CUES[k] = CUES[k]; });

    Sfx.DEEP_MOVES = MOVES;
})(typeof window !== 'undefined' ? window : globalThis);
