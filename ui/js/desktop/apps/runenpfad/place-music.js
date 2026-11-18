/* Runenpfad place music: the village hub loop and the two interior loops, plus the extra instruments they use. DOM-free. */
(function (root) {
    'use strict';

    const SCALES = { ionian: [0, 2, 4, 5, 7, 9, 11], mixolydian: [0, 2, 4, 5, 7, 9, 10] };
    const NO_DRUMS = { kick: [], snare: [], hat: [] };

    // Fields not listed here fall back to the composer defaults in music.js. `perc` is the drum layer level while no
    // enemy is near (the biome themes keep it silent), `vol` the deck level, `noFill` drops the snare roll of every 4th bar.
    const SPECS = {
        // Peaceful village: G major, moderate tempo, plucked lute lead, a soft flute line on the off beats, shaker and hand drum.
        village: { root: 55, scale: 'ionian', bpm: 100, prog: [[0, 3, 0, 4], [0, 5, 3, 4]], arpInstr: 'flute', melInstr: 'lute', arpGrid: [0.5, 2.5], drums: { kick: [], snare: [], hat: [0.5, 1.5, 2.5, 3.5], tom: [0, 2.5] }, perc: 0.4, vol: 0.9, noFill: true },
        // Tavern jig: A mixolydian, lilting 3+3+2 plucked strings under a fiddle lead; quieter, foot-stomp percussion only.
        in_cozy: { root: 57, scale: 'mixolydian', bpm: 138, prog: [[0, 6, 0, 3], [0, 3, 6, 0]], arpInstr: 'lute', melInstr: 'fiddle', arpGrid: [0, 0.75, 1.5, 2, 2.75, 3.5], drums: { kick: [], snare: [], hat: [], tom: [0, 2], tick: [1, 3], tickQuiet: true }, perc: 0.35, vol: 0.75, noFill: true },
        // Cave: E minor (no major third), very slow cello motif, distant drips and a slow heartbeat.
        in_cave: { root: 52, scale: 'minor', bpm: 54, prog: [[0, 0, 5, 0], [0, 3, 0, 6]], arpInstr: 'plink', melInstr: 'cello', arpGrid: [1.25, 3.5], drums: { kick: [0, 0.5], snare: [], hat: [] }, perc: 0.3, vol: 0.85, noFill: true }
    };
    const IDS = Object.keys(SPECS);

    // Registers one non-boss theme per spec (shared with story-music.js through `register`).
    function register(Music, specs) {
        if (!Music || !Music.THEMES || !Music.SCALES) return 0;
        Object.keys(SCALES).forEach(function (name) { if (!Music.SCALES[name]) Music.SCALES[name] = SCALES[name]; });
        let made = 0;
        Object.keys(specs).forEach(function (id) {
            const s = specs[id];
            if (!Music.SCALES[s.scale]) return;
            Music.THEMES[id] = {
                id: id, biome: 'woodland', boss: false, root: s.root, scaleName: s.scale, scale: Music.SCALES[s.scale], bpm: s.bpm,
                prog: s.prog, arpInstr: s.arpInstr, melInstr: s.melInstr, arpGrid: s.arpGrid, drums: s.drums || NO_DRUMS,
                perc: s.perc || 0, vol: s.vol || 1, noFill: !!s.noFill
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

    const VOICES = {
        // Plucked lute: soft triangle body, a bright saw octave that closes fast.
        lute: function (h, f, t, dur, vel) {
            const top = Math.min(5200, f * 8);
            const lp = h.keep(h.filt('lowpass', top, 0.9));
            lp.frequency.setValueAtTime(top, t);
            lp.frequency.exponentialRampToValueAtTime(Math.max(500, f * 1.4), t + 0.22);
            const a = h.keep(h.osc('triangle', f, t, 0));
            const b = h.keep(h.osc('sawtooth', f * 2, t, 4));
            const bg = amp(h, 0.1);
            a.connect(lp);
            b.connect(bg);
            bg.connect(lp);
            lp.connect(h.g);
            const end = t + Math.min(1, dur * 0.8 + 0.3);
            h.envelope(h.g, t, 0.19 * vel, 0.003, end);
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
            const end = sustain(h, t, dur, 0.1 * vel, 0.09, 0.18);
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
            h.envelope(h.g, t, 0.2 * vel, 0.002, end);
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

    root.RunenpfadPlaceMusic = { SPECS: SPECS, THEME_IDS: IDS, VOICES: VOICES, register: register, install: function (Music) { return register(Music, SPECS); }, themeFor: themeFor };
})(typeof window !== 'undefined' ? window : globalThis);
