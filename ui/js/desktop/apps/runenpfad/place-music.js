/* Runenpfad place music: the village hub loop and the two interior loops, plus the extra instruments and tune helpers
   they (and story-music.js) use. DOM-free; composition is seeded and pure. */
(function (root) {
    'use strict';

    const SCALES = {
        ionian: [0, 2, 4, 5, 7, 9, 11], mixolydian: [0, 2, 4, 5, 7, 9, 10], locrian: [0, 1, 3, 5, 6, 8, 10]
    };
    const NO_DRUMS = { kick: [], snare: [], hat: [] };

    // ---- tune helpers (used by the compose functions here and in story-music.js) ----
    function rngOf(seed) {
        let a = seed >>> 0;
        return function () {
            a = (a + 0x6D2B79F5) >>> 0;
            let t = a;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    // Scale degree (any integer) -> midi note of the theme's scale.
    function deg(th, d) {
        return th.root + th.scale[((d % 7) + 7) % 7] + 12 * Math.floor(d / 7);
    }

    function fold(m, lo, hi) {
        let x = m;
        while (x > hi) x -= 12;
        while (x < lo) x += 12;
        return x;
    }

    function note(t, dur, midi, vel, type) {
        return { t: t, dur: dur, midi: midi, vel: vel, type: type };
    }

    function rngFor(th, layer, bar, seed) {
        return rngOf(root.RunenpfadMusic.hash32(th.id + '|' + layer + '|' + bar + '|' + seed));
    }

    // Chord degree of a bar from the theme's two 4-bar chord loops.
    function chordOf(th, bar) {
        return th.prog[Math.floor(bar / 4) % 2][bar % 4];
    }

    // A melody figure [[beat, beats, step], ...] over the bar's chord root; steps are scale steps above `base`.
    function figure(th, d, pat, base, type, vel, lo, hi) {
        return pat.map(function (n) { return note(n[0], n[1], fold(deg(th, d + base + n[2]), lo, hi), vel, type); });
    }

    // ---- village: G major, bouncy folk dance with a hopping flute and a clear V-I cadence ----
    const VILLAGE_LEAD = [
        [[0, 0.5, 0], [0.5, 0.5, 2], [1, 1, 4], [2, 0.5, 2], [2.5, 0.5, 1], [3, 1, 0]],
        [[0, 0.5, 4], [0.5, 0.5, 2], [1, 0.5, 0], [1.5, 0.5, 2], [2, 1, 4], [3, 1, 2]],
        [[0, 0.5, 0], [0.5, 0.5, 2], [1, 0.5, 4], [1.5, 0.5, 5], [2, 1, 4], [3, 0.5, 2], [3.5, 0.5, 1]]
    ];
    const VILLAGE_END = [[0, 1, 2], [1, 0.5, 1], [1.5, 0.5, 0], [2, 2, 0]]; // resolves onto the tonic
    const VILLAGE_HALF = [[0, 0.5, 4], [0.5, 0.5, 2], [1, 1, 0], [2, 0.5, 2], [2.5, 0.5, 4], [3, 1, 6]]; // rests on the dominant
    const VILLAGE_FLUTE = [[0.5, 0.25, 4], [1.25, 0.25, 2], [2.5, 0.25, 4], [3.25, 0.5, 7]];

    function composeVillage(th, layer, bar) {
        const d = chordOf(th, bar);
        const ph = bar % 4;
        const rest = ph === 3 ? (d === 0 ? VILLAGE_END : VILLAGE_HALF) : VILLAGE_LEAD[ph % 3];
        const out = [];
        if (layer === 'melody') return figure(th, d, rest, 7, 'lute', 0.9, th.root + 7, th.root + 24);
        if (layer === 'pad') return figure(th, d, VILLAGE_FLUTE, 14, 'flute', 0.85, th.root + 19, th.root + 31);
        if (layer === 'arp') {
            [0, 2].forEach(function (t) {
                [0, 2, 4].forEach(function (s, i) { out.push(note(t + i * 0.04, 0.5, fold(deg(th, d + s), th.root - 2, th.root + 12), 0.5, 'lute')); });
            });
        } else if (layer === 'bass') {
            out.push(note(0, 1, fold(deg(th, d), th.root - 14, th.root - 3), 0.9, 'string'));
            out.push(note(2, 1, fold(deg(th, d + 4), th.root - 14, th.root - 3), 0.8, 'string'));
        } else if (layer === 'perc') {
            [0, 2.5].forEach(function (t) { out.push(note(t, 0.25, null, 0.65, 'tom')); });
            [0.5, 1.5, 2.5, 3.5].forEach(function (t) { out.push(note(t, 0.1, null, 0.5, 'hat')); });
            [1, 3].forEach(function (t) { out.push(note(t, 0.1, null, 0.6, 'tamb')); });
        }
        return out;
    }

    // ---- tavern: A major, bright short plucks on a repeating hook, oom-pah strums, stomp, clap and tambourine ----
    const COZY_HOOK = [
        [[0, 0.5, 0], [0.5, 0.5, 2], [1, 0.5, 4], [1.5, 0.5, 7], [2, 0.5, 4], [2.5, 0.5, 2], [3, 0.5, 4], [3.5, 0.5, 2]],
        [[0, 0.5, 4], [0.5, 0.5, 2], [1, 0.5, 0], [1.5, 0.5, 2], [2, 0.5, 4], [2.5, 0.5, 7], [3, 1, 9]],
        [[0, 0.5, 0], [0.5, 0.5, 2], [1, 0.5, 4], [1.5, 0.5, 7], [2, 0.5, 4], [2.5, 0.5, 2], [3, 0.5, 4], [3.5, 0.5, 7]],
        [[0, 0.5, 4], [0.5, 0.5, 7], [1, 0.5, 4], [1.5, 0.5, 2], [2, 1, 0], [3.5, 0.5, 4]]
    ];
    const COZY_END = [[0, 0.5, 4], [0.5, 0.5, 7], [1, 0.5, 4], [1.5, 0.5, 2], [2, 2, 0]];

    function composeCozy(th, layer, bar) {
        const d = chordOf(th, bar);
        const out = [];
        if (layer === 'melody') return figure(th, d, bar % 8 === 7 ? COZY_END : COZY_HOOK[bar % 4], 7, 'pluck', 0.95, th.root + 10, th.root + 27);
        if (layer === 'pad') {
            [0.5, 1.5, 2.5, 3.5].forEach(function (t) {
                [2, 4].forEach(function (s) { out.push(note(t, 0.25, fold(deg(th, d + s), th.root - 2, th.root + 12), 0.65, 'lute')); });
            });
        } else if (layer === 'bass') {
            [0, 1, 2, 3].forEach(function (t) { out.push(note(t, 0.5, fold(deg(th, d + (t % 2 ? 4 : 0)), th.root - 14, th.root - 3), 0.9, 'string')); });
        } else if (layer === 'perc') {
            [0, 1, 2, 3].forEach(function (t) { out.push(note(t, 0.25, null, 0.7, 'tom')); });
            [1, 3].forEach(function (t) { out.push(note(t, 0.2, null, 0.55, 'snare')); });
            [0.5, 1.5, 2.5, 3.5].forEach(function (t) { out.push(note(t, 0.1, null, 0.75, 'tamb')); });
        }
        return out;
    }

    // ---- cave: low rough drone, sparse irregular plinks (minor second / tritone), an occasional low hit ----
    function composeCave(th, layer, bar, seed) {
        const rng = rngFor(th, layer, bar, seed);
        const out = [];
        if (layer === 'pad') {
            out.push(note(0, 4.2, th.root - 12, 1, 'growl'));
            if (bar % 4 === 3) out.push(note(1, 3, th.root - 11, 0.55, 'growl'));
        } else if (layer === 'melody') {
            const count = rng() < 0.3 ? 0 : (rng() < 0.65 ? 1 : 2);
            const slots = [0.5, 1.25, 2, 2.75, 3.25];
            const first = [24, 25, 30, 36, 37][Math.floor(rng() * 5)];
            for (let i = 0; i < count; i++) {
                const at = slots.splice(Math.floor(rng() * slots.length), 1)[0];
                out.push(note(at, 0.5, th.root + first + (i ? (rng() < 0.5 ? 1 : 6) : 0), 0.6 + rng() * 0.4, 'plink'));
            }
            out.sort(function (a, b) { return a.t - b.t; });
        } else if (layer === 'perc') {
            if (bar % 8 === 5) out.push(note(0, 0.4, null, 0.8, 'taiko'));
            if (bar % 8 === 2) out.push(note(2.5, 0.3, null, 0.55, 'tom'));
        }
        return out;
    }

    const SPECS = {
        village: { root: 55, scale: 'ionian', bpm: 104, prog: [[0, 3, 0, 4], [0, 3, 4, 0]], compose: composeVillage, perc: 0.6, vol: 1.4 },
        in_cozy: { root: 57, scale: 'ionian', bpm: 132, prog: [[0, 3, 0, 4], [0, 3, 4, 0]], compose: composeCozy, perc: 1, vol: 1.5 },
        in_cave: { root: 40, scale: 'phrygian', bpm: 54, prog: [[0, 0, 0, 0], [0, 0, 0, 0]], compose: composeCave, perc: 0.8, vol: 1 }
    };
    const IDS = Object.keys(SPECS);

    // Registers one non-boss theme per spec (shared with story-music.js through `register`). Specs without `compose`
    // use the stock composer with `arpInstr`, `melInstr`, `arpGrid` and `drums`.
    function register(Music, specs) {
        if (!Music || !Music.THEMES || !Music.SCALES) return 0;
        Object.keys(SCALES).forEach(function (name) { if (!Music.SCALES[name]) Music.SCALES[name] = SCALES[name]; });
        let made = 0;
        Object.keys(specs).forEach(function (id) {
            const s = specs[id];
            if (!Music.SCALES[s.scale]) return;
            Music.THEMES[id] = {
                id: id, biome: 'woodland', boss: false, root: s.root, scaleName: s.scale, scale: Music.SCALES[s.scale], bpm: s.bpm,
                prog: s.prog, arpInstr: s.arpInstr || 'pluck', melInstr: s.melInstr || 'pluck', arpGrid: s.arpGrid || [], drums: s.drums || NO_DRUMS,
                perc: s.perc || 0, vol: s.vol || 1, noFill: !!s.noFill, compose: s.compose || null
            };
            made++;
        });
        return made;
    }

    // ---- instruments: voice(h, freq, t, dur seconds, vel) -> { end, main }; main is stopped by music.js, extras here ----
    function amp(h, v) {
        const n = h.keep(h.ctx.createGain());
        n.gain.value = v;
        return n;
    }

    function vibrato(h, f, t, rate, depth, targets) {
        const lfo = h.keep(h.osc('sine', rate, t, 0));
        const lg = h.keep(h.ctx.createGain());
        lg.gain.setValueAtTime(0, t);
        lg.gain.linearRampToValueAtTime(f * depth, t + 0.35);
        lfo.connect(lg);
        targets.forEach(function (p) { lg.connect(p); });
        return lfo;
    }

    // Sustained voice envelope: attack, hold until the note ends, release.
    function sustain(h, t, dur, peak, atk, rel) {
        const end = t + dur + rel;
        h.g.gain.setValueAtTime(0.0001, t);
        h.g.gain.linearRampToValueAtTime(peak, t + atk);
        h.g.gain.setValueAtTime(peak, t + Math.max(atk + 0.01, dur - 0.05));
        h.g.gain.linearRampToValueAtTime(0.0001, end);
        return end;
    }

    // Short noise burst through a band: tambourine and clock clicks.
    function burst(h, t, ftype, freq, q, peak, len) {
        const n = h.keep(h.noiseSrc(t, 0.2));
        const f = h.keep(h.filt(ftype, freq, q));
        n.connect(f);
        f.connect(h.g);
        h.envelope(h.g, t, peak, 0.001, t + len);
        n.stop(t + len + 0.02);
        return n;
    }

    const VOICES = {
        // Plucked lute: soft triangle body, a bright saw octave that closes fast.
        lute: function (h, f, t, dur, vel) {
            const top = Math.min(5200, f * 8);
            const lp = h.keep(h.filt('lowpass', top, 0.9));
            lp.frequency.setValueAtTime(top, t);
            lp.frequency.exponentialRampToValueAtTime(Math.max(500, f * 1.4), t + 0.18);
            const a = h.keep(h.osc('triangle', f, t, 0));
            const b = h.keep(h.osc('sawtooth', f * 2, t, 4));
            const bg = amp(h, 0.14);
            a.connect(lp);
            b.connect(bg);
            bg.connect(lp);
            lp.connect(h.g);
            const end = t + Math.min(0.7, dur * 0.6 + 0.2);
            h.envelope(h.g, t, 0.21 * vel, 0.003, end);
            b.stop(end + 0.02);
            return { end: end, main: a };
        },
        // Harp: pure partials with a long ring.
        harp: function (h, f, t, dur, vel) {
            const a = h.keep(h.osc('sine', f, t, 0));
            const b = h.keep(h.osc('sine', f * 2, t, 0));
            const c = h.keep(h.osc('triangle', f * 3, t, 0));
            const bg = amp(h, 0.28);
            const cg = amp(h, 0.08);
            a.connect(h.g);
            b.connect(bg);
            bg.connect(h.g);
            c.connect(cg);
            cg.connect(h.g);
            const end = t + 1.7;
            h.envelope(h.g, t, 0.2 * vel, 0.004, end);
            [b, c].forEach(function (o) { o.stop(end + 0.02); });
            return { end: end, main: a };
        },
        // Flute: sine and a faint octave, slow vibrato, a little breath noise.
        flute: function (h, f, t, dur, vel) {
            const a = h.keep(h.osc('sine', f, t, 0));
            const b = h.keep(h.osc('triangle', f * 2, t, 0));
            const bg = amp(h, 0.1);
            const n = h.keep(h.noiseSrc(t, 0.3));
            const bp = h.keep(h.filt('bandpass', Math.min(5000, Math.max(1500, f * 3)), 1.2));
            const ng = amp(h, 0.06);
            a.connect(h.g);
            b.connect(bg);
            bg.connect(h.g);
            n.connect(bp);
            bp.connect(ng);
            ng.connect(h.g);
            const lfo = vibrato(h, f, t, 5, 0.004, [a.frequency]);
            const end = sustain(h, t, dur, 0.12 * vel, 0.03, 0.12);
            [b, lfo, n].forEach(function (o) { o.stop(end + 0.02); });
            return { end: end, main: a };
        },
        // Fiddle: two detuned saws through a band, bow noise at the attack, late vibrato.
        fiddle: function (h, f, t, dur, vel) {
            const hp = h.keep(h.filt('highpass', 280, 0.6));
            const lp = h.keep(h.filt('lowpass', 2400, 0.8));
            const a = h.keep(h.osc('sawtooth', f, t, -7));
            const b = h.keep(h.osc('sawtooth', f, t, 7));
            const n = h.keep(h.noiseSrc(t, 0.2));
            const nb = h.keep(h.filt('bandpass', 3000, 1.5));
            const ng = h.keep(h.ctx.createGain());
            ng.gain.setValueAtTime(0.05, t);
            ng.gain.exponentialRampToValueAtTime(0.0001, t + 0.08);
            a.connect(hp);
            b.connect(hp);
            hp.connect(lp);
            lp.connect(h.g);
            n.connect(nb);
            nb.connect(ng);
            ng.connect(h.g);
            const lfo = vibrato(h, f, t, 5.6, 0.006, [a.frequency, b.frequency]);
            const end = sustain(h, t, dur, 0.05 * vel, 0.05, 0.15);
            [b, lfo, n].forEach(function (o) { o.stop(end + 0.02); });
            return { end: end, main: a };
        },
        // Cave drip: a tiny rising sine blip with a short inharmonic overtone.
        plink: function (h, f, t, dur, vel) {
            const a = h.keep(h.osc('sine', f, t, 0));
            a.frequency.setValueAtTime(f * 0.9, t);
            a.frequency.exponentialRampToValueAtTime(f * 1.05, t + 0.04);
            const b = h.keep(h.osc('sine', f * 2.76, t, 0));
            const bg = amp(h, 0.12);
            a.connect(h.g);
            b.connect(bg);
            bg.connect(h.g);
            const end = t + 0.9;
            h.envelope(h.g, t, 0.26 * vel, 0.002, end);
            b.stop(end + 0.02);
            return { end: end, main: a };
        },
        // Low bowed string: saw with a sub octave under a slowly opening low-pass.
        cello: function (h, f, t, dur, vel) {
            const lp = h.keep(h.filt('lowpass', 700, 0.8));
            lp.frequency.setValueAtTime(400, t);
            lp.frequency.linearRampToValueAtTime(1100, t + Math.min(0.8, dur * 0.5));
            const a = h.keep(h.osc('sawtooth', f, t, 0));
            const b = h.keep(h.osc('triangle', f / 2, t, 0));
            a.connect(lp);
            b.connect(lp);
            lp.connect(h.g);
            const lfo = vibrato(h, f, t, 4.5, 0.003, [a.frequency]);
            const end = sustain(h, t, dur, 0.07 * vel, 0.3, 0.4);
            [b, lfo].forEach(function (o) { o.stop(end + 0.02); });
            return { end: end, main: a };
        },
        // Fanfare brass: two detuned saws and a square, the filter blooms open within 70 ms.
        brass: function (h, f, t, dur, vel) {
            const lp = h.keep(h.filt('lowpass', 900, 0.9));
            lp.frequency.setValueAtTime(700, t);
            lp.frequency.linearRampToValueAtTime(3800, t + 0.07);
            lp.frequency.linearRampToValueAtTime(2200, t + Math.max(0.2, dur));
            const a = h.keep(h.osc('sawtooth', f, t, -9));
            const b = h.keep(h.osc('sawtooth', f, t, 9));
            const c = h.keep(h.osc('square', f, t, 0));
            const cg = amp(h, 0.25);
            a.connect(lp);
            b.connect(lp);
            c.connect(cg);
            cg.connect(lp);
            lp.connect(h.g);
            const end = sustain(h, t, dur, 0.095 * vel, 0.03, 0.1);
            [b, c].forEach(function (o) { o.stop(end + 0.02); });
            return { end: end, main: a };
        },
        // Rough low drone: two saws 30 cents apart beat slowly under a low-pass.
        growl: function (h, f, t, dur, vel) {
            const lp = h.keep(h.filt('lowpass', 230, 0.9));
            const a = h.keep(h.osc('sawtooth', f, t, -30));
            const b = h.keep(h.osc('sawtooth', f, t, 30));
            a.connect(lp);
            b.connect(lp);
            lp.connect(h.g);
            const end = sustain(h, t, dur, 0.14 * vel, 0.5, 0.5);
            b.stop(end + 0.02);
            return { end: end, main: a };
        },
        // Cold bowed metal: inharmonic partials with a slow attack and a faint beat.
        metal: function (h, f, t, dur, vel) {
            const a = h.keep(h.osc('sine', f, t, 0));
            const b = h.keep(h.osc('sine', f, t, 7));
            const c = h.keep(h.osc('sine', f * 2.76, t, 0));
            const d = h.keep(h.osc('sine', f * 5.4, t, 0));
            const cg = amp(h, 0.5);
            const dg = amp(h, 0.22);
            a.connect(h.g);
            b.connect(h.g);
            c.connect(cg);
            cg.connect(h.g);
            d.connect(dg);
            dg.connect(h.g);
            const end = sustain(h, t, dur, 0.09 * vel, 0.6, 0.8);
            [b, c, d].forEach(function (o) { o.stop(end + 0.02); });
            return { end: end, main: a };
        },
        // Dry clock: a sharp click (high on the tick, lower on the tock).
        clock: function (h, f, t, dur, vel) {
            const n = burst(h, t, 'bandpass', vel > 0.8 ? 3800 : 2600, 5, 0.5 * vel, 0.05);
            const a = h.keep(h.osc('sine', vel > 0.8 ? 1900 : 1300, t, 0));
            const ag = amp(h, 0.4);
            a.connect(ag);
            ag.connect(h.g);
            a.stop(t + 0.04);
            return { end: t + 0.07, main: n };
        },
        // Tambourine: a short bright noise shake.
        tamb: function (h, f, t, dur, vel) {
            const n = burst(h, t, 'bandpass', 8000, 1, 0.2 * vel, 0.11);
            return { end: t + 0.14, main: n };
        }
    };

    // Theme id for the current place (null outside one). Interior rooms name their loop; the hub stage is the village.
    function themeFor(sim) {
        if (!sim) return null;
        if (sim.interior) {
            const room = sim.interior.room;
            const id = room && room.def && room.def.music;
            return IDS.indexOf(id) >= 0 ? id : 'in_cozy';
        }
        return sim.stage && sim.stage.hub ? 'village' : null;
    }

    register(root.RunenpfadMusic, SPECS);

    root.RunenpfadPlaceMusic = {
        SPECS: SPECS, THEME_IDS: IDS, VOICES: VOICES, register: register, install: function (Music) { return register(Music, SPECS); }, themeFor: themeFor,
        tools: { rngFor: rngFor, deg: deg, fold: fold, note: note, chordOf: chordOf, figure: figure }
    };
})(typeof window !== 'undefined' ? window : globalThis);
