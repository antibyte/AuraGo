// Runenpfad composed multi-section biome & boss scores (8-to-16-bar thematic arrangements).
// Every pitched note is mapped through degMidi(theme, deg) so scale integrity holds strictly on all modes.
(function (root) {
    'use strict';

    function emitNotes(out, theme, rootMidi, degs, fn) {
        for (let i = 0; i < degs.length; i++) fn(degs[i], i, out, theme, rootMidi);
    }

    function addChord(out, h, th, lo, hi, pat, type, vel, dur, tOff) {
        for (let i = 0; i < pat.length; i++) {
            out.push({
                t: tOff || 0,
                dur: dur || 4,
                midi: h.fold(h.degMidi(th, pat[i]), lo, hi),
                vel: vel * (i === 0 ? 1 : 0.82),
                type: type
            });
        }
    }

    // ---- Calm / Exploration 16-bar suites (4 sections x 4 bars: A, B, A', C) ----

    const BIOME_ARRANGEMENTS = {
        woodland: {
            progs: [[0, 3, 0, 4], [5, 3, 4, 0], [0, 2, 3, 4], [5, 4, 3, 4]],
            bassInstr: 'cello',
            padInstr: 'string',
            arpInstr: 'harp',
            melodyA: [
                [[0, 1.5, 4, 0.85], [1.5, 0.5, 5, 0.72], [2, 1.0, 6, 0.88], [3, 0.9, 4, 0.75]],
                [[0, 1.0, 3, 0.84], [1.0, 1.0, 4, 0.80], [2, 1.8, 2, 0.86]],
                [[0, 1.0, 4, 0.86], [1.0, 0.5, 6, 0.78], [1.5, 1.0, 7, 0.90], [2.5, 1.3, 6, 0.82]],
                [[0, 1.5, 5, 0.85], [1.5, 0.5, 4, 0.74], [2, 1.8, 4, 0.82]]
            ],
            melodyB: [
                [[0, 1.0, 7, 0.90], [1.0, 1.0, 6, 0.82], [2, 1.0, 5, 0.80], [3, 0.9, 4, 0.76]],
                [[0, 1.5, 5, 0.88], [1.5, 0.5, 6, 0.75], [2, 1.8, 3, 0.84]],
                [[0, 0.75, 4, 0.84], [0.75, 0.75, 5, 0.80], [1.5, 1.0, 6, 0.86], [2.5, 1.3, 7, 0.92]],
                [[0, 2.2, 4, 0.86], [2.5, 1.3, 0, 0.74]]
            ],
            melInstrA: 'flute',
            melInstrB: 'fiddle',
            arpPat: [0, 2, 4, 7, 4, 2, 4, 5],
            perc: function (out, p) {
                [0, 2].forEach((t, i) => out.push({ t, dur: 0.35, midi: null, vel: i === 0 ? 0.78 : 0.62, type: 'framedrum' }));
                [0.5, 1.5, 2.5, 3.5].forEach((t) => out.push({ t, dur: 0.16, midi: null, vel: 0.42, type: 'tamb' }));
                if (p === 3) [3.0, 3.5].forEach((t) => out.push({ t, dur: 0.25, midi: null, vel: 0.52, type: 'tom' }));
            }
        },
        cavern: {
            progs: [[0, 1, 0, 6], [0, 3, 1, 0], [0, 5, 1, 4], [3, 1, 6, 0]],
            bassInstr: 'cello',
            padInstr: 'choir',
            arpInstr: 'celesta',
            melodyA: [
                [[0, 2.2, 4, 0.78], [2.5, 1.3, 5, 0.72]],
                [[0, 1.5, 3, 0.80], [1.75, 2.0, 1, 0.76]],
                [[0, 2.0, 4, 0.82], [2.0, 1.0, 2, 0.70], [3.0, 0.9, 1, 0.66]],
                [[0, 3.2, 0, 0.78]]
            ],
            melodyB: [
                [[0, 1.2, 7, 0.82], [1.5, 1.0, 5, 0.75], [2.75, 1.1, 4, 0.72]],
                [[0, 1.5, 3, 0.80], [2.0, 1.75, 5, 0.76]],
                [[0, 1.0, 6, 0.78], [1.5, 1.0, 4, 0.72], [2.5, 1.3, 1, 0.74]],
                [[0, 2.8, 0, 0.82]]
            ],
            melInstrA: 'darkbell',
            melInstrB: 'flute',
            arpGrid: [0, 1.25, 2.0, 3.25],
            arpPat: [0, 4, 2, 7],
            perc: function (out, p) {
                if (p % 2 === 0) out.push({ t: 0, dur: 0.7, midi: null, vel: 0.58, type: 'taiko' });
                out.push({ t: 2, dur: 0.08, midi: null, vel: 0.34, type: 'shaker' });
                if (p === 3) out.push({ t: 3.5, dur: 0.6, midi: null, vel: 0.44, type: 'taiko' });
            }
        },
        desert: {
            progs: [[0, 1, 0, 6], [0, 4, 1, 0], [0, 1, 3, 1], [4, 1, 6, 0]],
            bassInstr: 'pizz',
            padInstr: 'string',
            arpInstr: 'oud',
            melodyA: [
                [[0, 1.2, 4, 0.88], [1.25, 0.75, 5, 0.78], [2.0, 1.0, 4, 0.86], [3.0, 0.8, 2, 0.82]],
                [[0, 0.75, 3, 0.82], [0.75, 0.75, 2, 0.78], [1.5, 1.25, 1, 0.86], [3.0, 0.8, 0, 0.74]],
                [[0, 1.0, 2, 0.85], [1.0, 0.5, 4, 0.82], [1.5, 1.0, 5, 0.88], [2.5, 1.3, 4, 0.80]],
                [[0, 1.5, 1, 0.86], [1.75, 0.75, 2, 0.78], [2.5, 1.3, 0, 0.84]]
            ],
            melodyB: [
                [[0, 0.5, 4, 0.84], [0.5, 0.5, 5, 0.78], [1.0, 0.5, 6, 0.82], [1.5, 1.2, 4, 0.88], [3.0, 0.8, 2, 0.76]],
                [[0, 1.0, 3, 0.86], [1.0, 1.0, 1, 0.82], [2.0, 0.75, 2, 0.80], [2.75, 1.0, 4, 0.85]],
                [[0, 1.5, 7, 0.90], [1.5, 0.75, 6, 0.80], [2.25, 0.75, 5, 0.78], [3.0, 0.8, 4, 0.84]],
                [[0, 1.25, 2, 0.84], [1.5, 1.0, 1, 0.80], [2.5, 1.25, 0, 0.86]]
            ],
            melInstrA: 'ney',
            melInstrB: 'oud',
            arpPat: [0, 4, 2, 4, 5, 4, 2, 1],
            perc: function (out, p) {
                [0, 1.5].forEach((t, i) => out.push({ t, dur: 0.35, midi: null, vel: i === 0 ? 0.84 : 0.72, type: 'darbuka_doum' }));
                [1.0, 2.5, 3.0].forEach((t) => out.push({ t, dur: 0.14, midi: null, vel: 0.64, type: 'darbuka_tek' }));
                [0.5, 2.0, 3.5].forEach((t) => out.push({ t, dur: 0.18, midi: null, vel: 0.45, type: 'tamb' }));
                if (p === 3) out.push({ t: 3.75, dur: 0.12, midi: null, vel: 0.68, type: 'darbuka_tek' });
            }
        },
        glacier: {
            progs: [[0, 1, 0, 4], [5, 1, 4, 0], [0, 3, 4, 1], [5, 3, 1, 0]],
            bassInstr: 'horn',
            padInstr: 'choir',
            arpInstr: 'glass',
            melodyA: [
                [[0, 2.0, 4, 0.82], [2.0, 1.8, 6, 0.86]],
                [[0, 1.5, 3, 0.84], [1.75, 2.0, 4, 0.80]],
                [[0, 1.5, 7, 0.88], [1.75, 1.0, 6, 0.82], [2.75, 1.1, 4, 0.78]],
                [[0, 3.2, 2, 0.80]]
            ],
            melodyB: [
                [[0, 1.5, 6, 0.84], [1.5, 1.0, 7, 0.88], [2.5, 1.3, 4, 0.80]],
                [[0, 2.0, 3, 0.82], [2.25, 1.5, 1, 0.78]],
                [[0, 1.5, 4, 0.84], [1.5, 1.25, 6, 0.86], [2.75, 1.1, 8, 0.90]],
                [[0, 3.0, 4, 0.82]]
            ],
            melInstrA: 'flute',
            melInstrB: 'celesta',
            arpGrid: [0, 1, 2, 3],
            arpPat: [0, 2, 4, 7],
            perc: function (out, p) {
                out.push({ t: 0, dur: 0.4, midi: null, vel: 0.46, type: 'framedrum' });
                [1, 3].forEach((t) => out.push({ t, dur: 0.08, midi: null, vel: 0.32, type: 'shaker' }));
                if (p === 3) out.push({ t: 2.5, dur: 0.35, midi: null, vel: 0.36, type: 'hat' });
            }
        },
        citadel: {
            progs: [[0, 5, 3, 4], [0, 3, 4, 0], [0, 2, 5, 4], [3, 5, 4, 0]],
            bassInstr: 'cello',
            padInstr: 'organ',
            arpInstr: 'pizz',
            melodyA: [
                [[0, 1.5, 4, 0.88], [1.5, 0.5, 5, 0.80], [2.0, 1.0, 6, 0.90], [3.0, 0.9, 4, 0.82]],
                [[0, 1.0, 3, 0.85], [1.0, 1.0, 2, 0.80], [2.0, 1.8, 0, 0.86]],
                [[0, 1.0, 4, 0.88], [1.0, 1.0, 6, 0.86], [2.0, 1.0, 5, 0.82], [3.0, 0.9, 3, 0.78]],
                [[0, 2.0, 4, 0.90], [2.25, 1.5, 6, 0.84]]
            ],
            melodyB: [
                [[0, 0.75, 7, 0.90], [0.75, 0.75, 6, 0.84], [1.5, 1.0, 4, 0.86], [2.5, 1.3, 5, 0.82]],
                [[0, 1.0, 3, 0.86], [1.0, 0.5, 4, 0.80], [1.5, 1.25, 5, 0.86], [3.0, 0.8, 6, 0.88]],
                [[0, 1.0, 4, 0.88], [1.0, 1.0, 2, 0.82], [2.0, 1.0, 1, 0.78], [3.0, 0.9, 6, 0.84]],
                [[0, 2.8, 0, 0.90]]
            ],
            melInstrA: 'brass',
            melInstrB: 'fiddle',
            arpPat: [0, 2, 4, 2, 7, 4, 2, 4],
            perc: function (out, p) {
                [0, 2].forEach((t, i) => out.push({ t, dur: 0.45, midi: null, vel: i === 0 ? 0.85 : 0.72, type: 'taiko' }));
                [1, 3].forEach((t) => out.push({ t, dur: 0.2, midi: null, vel: 0.58, type: 'snare' }));
                if (p === 0) out.push({ t: 0, dur: 0.6, midi: null, vel: 0.42, type: 'anvil' });
                if (p === 3) [3.25, 3.5, 3.75].forEach((t) => out.push({ t, dur: 0.14, midi: null, vel: 0.52, type: 'snare' }));
            }
        }
    };

    const EIGHTHS = [0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5];
    const BOSS_MELODIES = {
        woodland: [
            [[0, 1.5, 0, 0.92], [1.5, 0.5, 1, 0.80], [2, 1.6, 2, 0.88]],
            [[0, 1.0, 4, 0.92], [1.0, 1.0, 3, 0.84], [2, 1.8, 1, 0.86]],
            [[0, 1.5, 2, 0.90], [1.5, 0.5, 4, 0.84], [2, 1.6, 5, 0.92]],
            [[0, 1.5, 4, 0.90], [2.0, 1.75, 1, 0.86]]
        ],
        cavern: [
            [[0, 2.2, 0, 0.88], [2.25, 1.5, 4, 0.82]],
            [[0, 1.8, 1, 0.90], [2.0, 1.8, 3, 0.82]],
            [[0, 2.0, 5, 0.88], [2.0, 1.6, 4, 0.84]],
            [[0, 1.8, 1, 0.92], [2.0, 1.8, 0, 0.86]]
        ],
        desert: [
            [[0, 1.6, 1, 0.90], [2.0, 1.6, 4, 0.86]],
            [[0, 1.4, 4, 0.88], [1.5, 0.5, 3, 0.80], [2.1, 1.6, 1, 0.86]],
            [[0, 1.5, 1, 0.90], [1.75, 0.75, 2, 0.82], [2.5, 1.3, 0, 0.86]],
            [[0, 1.6, 4, 0.88], [2.1, 1.6, 1, 0.90]]
        ],
        glacier: [
            [[0, 1.5, 4, 0.88], [1.75, 1.8, 7, 0.90]],
            [[0, 1.5, 6, 0.86], [2.0, 1.6, 4, 0.84]],
            [[0, 1.25, 5, 0.88], [1.5, 1.0, 6, 0.86], [2.75, 1.1, 4, 0.82]],
            [[0, 2.0, 1, 0.90], [2.25, 1.5, 0, 0.84]]
        ],
        citadel: [
            [[0, 2.0, 2, 0.92], [2.25, 1.5, 5, 0.88]],
            [[0, 1.8, 4, 0.90], [2.0, 1.75, 2, 0.86]],
            [[0, 1.5, 6, 0.92], [1.75, 0.75, 4, 0.84], [2.5, 1.3, 3, 0.86]],
            [[0, 2.0, 5, 0.90], [2.25, 1.5, 2, 0.86]]
        ]
    };

    function composeBiome(th, layer, bar, h) {
        const spec = BIOME_ARRANGEMENTS[th.biome];
        if (!spec) return null;
        const b16 = ((bar % 16) + 16) % 16;
        const sec = Math.floor(b16 / 4);
        const phr = b16 % 4;
        const d = spec.progs[sec][phr];
        const r = th.root;
        const out = [];

        if (layer === 'pad') {
            addChord(out, h, th, r - 2, r + 15, [d, d + 2, d + 4], spec.padInstr, 0.84, 3.9, 0);
            return out;
        }
        if (layer === 'bass') {
            const lo = h.fold(h.degMidi(th, d), r - 14, r - 3);
            const fi = h.fold(h.degMidi(th, d + 4), r - 14, r - 3);
            const mid = h.fold(h.degMidi(th, d + 2), r - 14, r - 3);
            if (phr % 2 === 0) {
                out.push({ t: 0, dur: 1.9, midi: lo, vel: 0.96, type: spec.bassInstr });
                out.push({ t: 2, dur: 1.0, midi: fi, vel: 0.82, type: spec.bassInstr });
                out.push({ t: 3, dur: 0.9, midi: mid, vel: 0.78, type: spec.bassInstr });
            } else {
                out.push({ t: 0, dur: 2.5, midi: lo, vel: 0.94, type: spec.bassInstr });
                out.push({ t: 2.5, dur: 0.5, midi: lo, vel: 0.72, type: spec.bassInstr });
                out.push({ t: 3, dur: 0.9, midi: fi, vel: 0.84, type: spec.bassInstr });
            }
            return out;
        }
        if (layer === 'arp') {
            const grid = spec.arpGrid || th.arpGrid || EIGHTHS;
            const pat = spec.arpPat;
            const gap = grid.length > 1 ? grid[1] - grid[0] : 1;
            grid.forEach((t, i) => {
                const deg = d + pat[i % pat.length];
                out.push({
                    t,
                    dur: Math.max(0.55, gap * 1.3),
                    midi: h.fold(h.degMidi(th, deg) + 12, r + 7, r + 24),
                    vel: (t % 1 === 0 ? 0.84 : 0.64),
                    type: spec.arpInstr
                });
            });
            return out;
        }
        if (layer === 'melody') {
            const isB = sec === 1 || sec === 3;
            const phrases = isB ? spec.melodyB : spec.melodyA;
            const instr = isB ? spec.melInstrB : spec.melInstrA;
            const notes = phrases[phr];
            emitNotes(out, th, r, notes, (n) => {
                out.push({
                    t: n[0],
                    dur: n[1],
                    midi: h.fold(h.degMidi(th, d + n[2]) + 12, r + 10, r + 28),
                    vel: n[3],
                    type: instr
                });
            });
            return out;
        }
        if (layer === 'perc') {
            spec.perc(out, phr);
            return out;
        }
        if (layer === 'boss') {
            return out;
        }
        return null;
    }

    function composeBoss(th, layer, bar, h) {
        const b8 = ((bar % 8) + 8) % 8;
        const sec = Math.floor(b8 / 4);
        const phr = b8 % 4;
        const d = th.prog[sec][phr];
        const r = th.root;
        const out = [];

        if (layer === 'pad') {
            addChord(out, h, th, r - 2, r + 14, [d, d + 2, d + 4], 'pad', 0.86, 3.8, 0);
            if (th.leadStyle === 'charmer') {
                out.push({ t: 0, dur: 3.8, midi: h.fold(h.degMidi(th, 1), r - 14, r - 3), vel: 0.78, type: 'choir' });
            }
            return out;
        }
        if (layer === 'bass') {
            const lo = h.fold(h.degMidi(th, d), r - 14, r - 3);
            const fi = h.fold(h.degMidi(th, d + 4), r - 14, r - 3);
            [[0, 1.35, 0.98, lo], [1.5, 0.45, 0.82, fi], [2, 1.2, 0.92, lo], [3.25, 0.65, 0.85, fi]].forEach((n) => {
                out.push({ t: n[0], dur: n[1], midi: n[3], vel: n[2], type: th.bassInstr || 'bass' });
            });
            return out;
        }
        if (layer === 'arp') {
            const grid = th.arpGrid || EIGHTHS;
            const gap = grid.length > 1 ? grid[1] - grid[0] : 0.5;
            grid.forEach((t, i) => {
                const deg = d + [0, 2, 4, 7][i % 4];
                out.push({
                    t,
                    dur: Math.max(0.45, gap * 1.2),
                    midi: h.fold(h.degMidi(th, deg) + 12, r + 7, r + 24),
                    vel: i % 2 === 0 ? 0.85 : 0.65,
                    type: th.arpInstr
                });
            });
            return out;
        }
        if (layer === 'melody') {
            const mList = BOSS_MELODIES[th.biome];
            const notes = mList ? mList[phr] : null;
            if (!notes) return null;
            const lo = th.biome === 'desert' ? r - 12 : (th.biome === 'citadel' ? r - 7 : r);
            const hi = th.biome === 'desert' ? r - 1 : (th.biome === 'citadel' ? r + 6 : r + 12);
            notes.forEach((n) => {
                out.push({
                    t: n[0],
                    dur: n[1],
                    midi: h.fold(h.degMidi(th, n[2] + (sec === 1 && phr === 2 ? 1 : 0)), lo, hi),
                    vel: n[3],
                    type: th.leadInstr || 'horn'
                });
            });
            return out;
        }
        if (layer === 'perc') {
            const dr = th.drums;
            dr.kick.forEach((t) => out.push({ t, dur: 0.3, midi: null, vel: 0.92, type: 'kick' }));
            dr.snare.forEach((t) => out.push({ t, dur: 0.2, midi: null, vel: 0.68, type: 'snare' }));
            dr.hat.forEach((t) => out.push({ t, dur: 0.12, midi: null, vel: 0.48, type: 'hat' }));
            (dr.tom || []).forEach((t, i) => out.push({ t, dur: 0.25, midi: null, vel: 0.75 + 0.1 * (i % 2), type: 'tom' }));
            (dr.taiko || []).forEach((t, i) => out.push({ t, dur: 0.42, midi: null, vel: 0.82 + 0.12 * (i % 2), type: 'taiko' }));
            (dr.tick || []).forEach((t, i) => out.push({ t, dur: 0.05, midi: null, vel: dr.tickQuiet ? 0.32 : (i % 4 === 0 ? 0.85 : 0.5), type: 'tick' }));
            (dr.clank || []).forEach((t) => out.push({ t, dur: 0.35, midi: null, vel: 0.78, type: 'clank' }));
            return out;
        }
        if (layer === 'boss') {
            // Retain exact 15-event structure on bar 0 (1 drone + 6 stabs + 8 ostinato) verified by sim tests.
            out.push({ t: 0, dur: 3.9, midi: h.fold(h.degMidi(th, 0), r - 24, r - 13), vel: th.droneVel || 1, type: 'drone' });
            [[0, [d, d + 1], 1], [1.5, [d, d + 4], 0.82], [2.75, [d + 1, d], 0.9]].forEach((st) => {
                st[1].forEach((deg) => {
                    out.push({ t: st[0], dur: 0.42, midi: h.fold(h.degMidi(th, deg), r - 12, r - 1), vel: st[2], type: 'stab' });
                });
            });
            const ostPat = [0, 0, 1, 0, 0, 4, 0, 1];
            if (phr === 3) out.push({ t: 0, dur: 3.9, midi: null, vel: 1, type: 'swell' });
            (th.ostGrid || EIGHTHS).forEach((t, i) => {
                const fast = (th.ostGrid || EIGHTHS).length > 8;
                const deg = d + ostPat[(fast ? i >> 1 : i) % 8];
                out.push({
                    t,
                    dur: fast ? 0.2 : 0.4,
                    midi: h.fold(h.degMidi(th, deg), th.ostLow ? r - 18 : r - 12, th.ostLow ? r - 6 : r + 2),
                    vel: i % 2 === 0 ? 0.98 : 0.72,
                    type: th.ostInstr || 'ostinato'
                });
            });
            return out;
        }
        return null;
    }

    root.RunenpfadMusicScores = {
        compose: function (theme, layer, bar, seed, helpers) {
            if (!theme || !helpers) return null;
            if (theme.boss && /^boss_/.test(theme.id)) return composeBoss(theme, layer, bar, helpers);
            if (!theme.boss && BIOME_ARRANGEMENTS[theme.biome]) return composeBiome(theme, layer, bar, helpers);
            return null;
        }
    };
})(typeof window !== 'undefined' ? window : globalThis);
