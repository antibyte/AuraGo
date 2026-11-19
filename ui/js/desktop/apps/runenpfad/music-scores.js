// Runenpfad composed multi-section biome & boss scores: 25 distinct stage suites (5 per biome),
// biome-tailored dramatic action layers, and 8-bar world boss suites with Phase-2 overdrive.
// Every pitched note passes through degMidi(theme, deg) so scale integrity holds strictly across all modes.
(function (root) {
    'use strict';

    const EIGHTHS = [0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5];
    const QUARTERS = [0, 1, 2, 3];
    const SYNC_GRID = [0, 0.75, 1.5, 2, 2.75, 3.5];
    const SPARSE_GRID = [0, 1.5, 2, 3.25];
    const TRIPLET_GRID = [0, 0.67, 1.33, 2, 2.67, 3.33];

    function addChord(out, h, th, lo, hi, pat, type, vel, dur, tOff) {
        for (let i = 0; i < pat.length; i++) {
            out.push({
                t: tOff || 0,
                dur: dur || 3.8,
                midi: h.fold(h.degMidi(th, pat[i]), lo, hi),
                vel: vel * (i === 0 ? 1 : 0.8),
                type: type
            });
        }
    }

    function emitPad(out, h, th, d, r, instr, style, phr) {
        const tri = [d, d + 2, d + 4];
        if (style === 'choral') {
            addChord(out, h, th, r - 2, r + 15, [d, d + 2, d + 4, d + 7], instr, 0.78, 3.9, 0);
        } else if (style === 'swell') {
            addChord(out, h, th, r - 2, r + 14, tri, instr, 0.80, 3.5, 0.25);
        } else if (style === 'broken') {
            tri.forEach(function (deg, i) {
                out.push({ t: i * 1.0, dur: Math.max(0.9, 3.7 - i), midi: h.fold(h.degMidi(th, deg), r - 2, r + 15), vel: 0.78 - i * 0.05, type: instr });
            });
            out.push({ t: 3.0, dur: 0.9, midi: h.fold(h.degMidi(th, d + 4), r - 2, r + 15), vel: 0.7, type: instr });
        } else if (style === 'pulsed') {
            [0, 1.5, 2.5].forEach(function (t, j) {
                addChord(out, h, th, r - 2, r + 14, [d, d + 2, d + 4], instr, 0.76 - j * 0.08, j === 1 ? 0.9 : 1.1, t);
            });
        } else {
            addChord(out, h, th, r - 2, r + 15, phr === 3 ? [d, d + 2, d + 4, d + 6] : tri, instr, 0.82, 3.85, 0);
        }
    }

    function emitBass(out, h, th, d, r, instr, style, phr) {
        const lo = h.fold(h.degMidi(th, d), r - 14, r - 3);
        const fi = h.fold(h.degMidi(th, d + 4), r - 14, r - 3);
        const mi = h.fold(h.degMidi(th, d + 2), r - 14, r - 3);
        const hi = h.fold(h.degMidi(th, d + 6), r - 14, r - 3);
        if (style === 'walking') {
            const notes = phr % 2 === 0 ? [lo, mi, fi, hi] : [lo, fi, mi, lo];
            notes.forEach(function (m, i) { out.push({ t: i, dur: 0.9, midi: m, vel: i === 0 ? 0.92 : 0.78, type: instr }); });
        } else if (style === 'pulse') {
            [0, 0.75, 1.5, 2, 3, 3.5].forEach(function (t, i) {
                out.push({ t: t, dur: 0.45, midi: i % 3 === 1 ? fi : lo, vel: i % 2 === 0 ? 0.9 : 0.72, type: instr });
            });
        } else if (style === 'pedal') {
            const ped = h.fold(h.degMidi(th, 0), r - 14, r - 3);
            out.push({ t: 0, dur: 3.7, midi: phr === 3 ? fi : ped, vel: 0.9, type: instr });
        } else if (style === 'march') {
            [[0, 1.1, 0.95, lo], [1.5, 0.4, 0.74, fi], [2, 1.1, 0.88, lo], [3, 0.8, 0.8, fi]].forEach(function (n) {
                out.push({ t: n[0], dur: n[1], midi: n[3], vel: n[2], type: instr });
            });
        } else if (style === 'syncopated') {
            [[0, 0.7, 0.92, lo], [0.75, 0.7, 0.78, mi], [1.5, 0.9, 0.86, fi], [2.5, 0.7, 0.8, lo], [3.25, 0.65, 0.84, hi]].forEach(function (n) {
                out.push({ t: n[0], dur: n[1], midi: n[3], vel: n[2], type: instr });
            });
        } else if (phr % 2 === 0) {
            out.push({ t: 0, dur: 1.9, midi: lo, vel: 0.94, type: instr });
            out.push({ t: 2, dur: 1.0, midi: fi, vel: 0.8, type: instr });
            out.push({ t: 3, dur: 0.9, midi: mi, vel: 0.76, type: instr });
        } else {
            out.push({ t: 0, dur: 2.4, midi: lo, vel: 0.92, type: instr });
            out.push({ t: 2.5, dur: 0.45, midi: lo, vel: 0.7, type: instr });
            out.push({ t: 3, dur: 0.9, midi: fi, vel: 0.82, type: instr });
        }
    }

    // Light atmospheric exploration percussion so calm stages stay open and combat surges contrast sharply.
    function emitCalmPerc(out, style, phr, sec) {
        if (style === 'wood_gentle') {
            out.push({ t: 0, dur: 0.35, midi: null, vel: 0.54, type: 'framedrum' });
            if (phr % 2 === 1) out.push({ t: 2, dur: 0.32, midi: null, vel: 0.44, type: 'framedrum' });
            [0.5, 2.5].forEach(function (t) { out.push({ t: t, dur: 0.14, midi: null, vel: 0.3, type: 'tamb' }); });
        } else if (style === 'wood_folk') {
            [0, 2].forEach(function (t, i) { out.push({ t: t, dur: 0.3, midi: null, vel: i ? 0.48 : 0.62, type: 'framedrum' }); });
            [1, 3].forEach(function (t) { out.push({ t: t, dur: 0.14, midi: null, vel: 0.38, type: 'tamb' }); });
            if (phr === 3) out.push({ t: 3.5, dur: 0.2, midi: null, vel: 0.42, type: 'tom' });
        } else if (style === 'wood_canopy') {
            [0, 1.5, 2.5].forEach(function (t, i) { out.push({ t: t, dur: 0.18, midi: null, vel: i ? 0.34 : 0.5, type: 'tom' }); });
            [0.5, 2.0, 3.5].forEach(function (t) { out.push({ t: t, dur: 0.08, midi: null, vel: 0.28, type: 'shaker' }); });
        } else if (style === 'mystic_sparse') {
            if (phr % 2 === 0) out.push({ t: 0, dur: 0.65, midi: null, vel: 0.4, type: 'taiko' });
            if (phr === 3) out.push({ t: 2, dur: 0.12, midi: null, vel: 0.3, type: 'tamb' });
        } else if (style === 'march_light') {
            [0, 2].forEach(function (t, i) { out.push({ t: t, dur: 0.42, midi: null, vel: i ? 0.48 : 0.62, type: 'taiko' }); });
            if (phr === 3) [3, 3.5].forEach(function (t) { out.push({ t: t, dur: 0.16, midi: null, vel: 0.38, type: 'snare' }); });
        } else if (style === 'cav_tick') {
            [0, 1, 2, 3].forEach(function (t, i) { out.push({ t: t, dur: 0.06, midi: null, vel: i === 0 ? 0.55 : 0.32, type: 'clock' }); });
            if (phr % 2 === 1) out.push({ t: 2.5, dur: 0.26, midi: null, vel: 0.42, type: 'tom' });
        } else if (style === 'cav_menace') {
            out.push({ t: 0, dur: 0.45, midi: null, vel: 0.56, type: 'taiko' });
            if (phr === 0 && sec % 2 === 0) out.push({ t: 2, dur: 0.5, midi: null, vel: 0.36, type: 'anvil' });
            out.push({ t: 3, dur: 0.22, midi: null, vel: 0.4, type: 'tom' });
        } else if (style === 'darbuka_calm') {
            [0, 1.5].forEach(function (t, i) { out.push({ t: t, dur: 0.32, midi: null, vel: i ? 0.48 : 0.62, type: 'darbuka_doum' }); });
            [1, 3].forEach(function (t) { out.push({ t: t, dur: 0.12, midi: null, vel: 0.42, type: 'darbuka_tek' }); });
        } else if (style === 'darbuka_drive') {
            [0, 1.5, 2.75].forEach(function (t, i) { out.push({ t: t, dur: 0.28, midi: null, vel: i ? 0.52 : 0.64, type: 'darbuka_doum' }); });
            [0.5, 1.0, 2.0, 3.5].forEach(function (t) { out.push({ t: t, dur: 0.12, midi: null, vel: 0.44, type: 'darbuka_tek' }); });
        } else if (style === 'glacier_soft') {
            out.push({ t: 0, dur: 0.35, midi: null, vel: 0.4, type: 'framedrum' });
            [1, 3].forEach(function (t) { out.push({ t: t, dur: 0.08, midi: null, vel: 0.28, type: 'shaker' }); });
        } else if (style === 'clockwork') {
            [0, 2].forEach(function (t) { out.push({ t: t, dur: 0.06, midi: null, vel: 0.48, type: 'clock' }); });
            [0.5, 1.5, 2.5, 3.5].forEach(function (t) { out.push({ t: t, dur: 0.05, midi: null, vel: 0.26, type: 'tick' }); });
            if (phr === 3) out.push({ t: 3, dur: 0.16, midi: null, vel: 0.4, type: 'snare' });
        } else {
            out.push({ t: 0, dur: 0.45, midi: null, vel: 0.62, type: 'taiko' });
            [1.5, 3].forEach(function (t) { out.push({ t: t, dur: 0.18, midi: null, vel: 0.4, type: 'snare' }); });
            if (phr === 0) out.push({ t: 0, dur: 0.5, midi: null, vel: 0.32, type: 'anvil' });
        }
    }

    // ---- 25 Distinct Hand-Composed Stage Suites (5 per biome) ----
    const BIOME_SUITES = {
        woodland: [
            { // Level 1: Sunlit Meadow (D Dorian, classic lyrical flute/fiddle quest)
                bpmMul: 1.00, progs: [[0, 3, 0, 4], [5, 3, 4, 0], [0, 2, 3, 4], [5, 4, 3, 0]],
                padInstr: 'string', padStyle: 'sustain', bassInstr: 'cello', bassStyle: 'lyrical',
                arpInstr: 'harp', arpGrid: EIGHTHS, arpPat: [0, 2, 4, 7, 4, 2, 4, 5],
                melInstrA: 'flute', melInstrB: 'fiddle', percStyle: 'wood_gentle',
                melA: [
                    [[0, 1.5, 4, 0.85], [1.5, 0.5, 5, 0.72], [2, 1.0, 6, 0.88], [3, 0.9, 4, 0.75]],
                    [[0, 1.0, 3, 0.84], [1.0, 1.0, 4, 0.80], [2, 1.8, 2, 0.86]],
                    [[0, 1.0, 4, 0.86], [1.0, 0.5, 6, 0.78], [1.5, 1.0, 7, 0.90], [2.5, 1.3, 6, 0.82]],
                    [[0, 1.5, 5, 0.85], [1.5, 0.5, 4, 0.74], [2, 1.8, 4, 0.82]]
                ],
                melB: [
                    [[0, 1.0, 7, 0.90], [1.0, 1.0, 6, 0.82], [2, 1.0, 5, 0.80], [3, 0.9, 4, 0.76]],
                    [[0, 1.5, 5, 0.88], [1.5, 0.5, 6, 0.75], [2, 1.8, 3, 0.84]],
                    [[0, 0.75, 4, 0.84], [0.75, 0.75, 5, 0.80], [1.5, 1.0, 6, 0.86], [2.5, 1.3, 7, 0.92]],
                    [[0, 2.2, 4, 0.86], [2.5, 1.3, 0, 0.74]]
                ]
            },
            { // Level 2: Ancient Grove (Witch's camp, rustic fiddle dance)
                bpmMul: 1.06, progs: [[0, 6, 3, 4], [0, 5, 6, 0], [3, 4, 5, 2], [0, 6, 4, 0]],
                padInstr: 'horn', padStyle: 'swell', bassInstr: 'pizz', bassStyle: 'walking',
                arpInstr: 'lute', arpGrid: TRIPLET_GRID, arpPat: [0, 4, 2, 5, 4, 2],
                melInstrA: 'fiddle', melInstrB: 'flute', percStyle: 'wood_folk',
                melA: [
                    [[0, 0.5, 2, 0.86], [0.5, 0.5, 4, 0.8], [1, 1.0, 7, 0.9], [2, 0.75, 5, 0.82], [2.75, 1.0, 4, 0.78]],
                    [[0, 0.75, 6, 0.88], [0.75, 0.75, 5, 0.8], [1.5, 1.0, 4, 0.84], [2.5, 1.2, 2, 0.8]],
                    [[0, 1.0, 4, 0.86], [1, 0.5, 2, 0.78], [1.5, 1.0, 5, 0.88], [2.5, 0.75, 4, 0.82], [3.25, 0.6, 3, 0.76]],
                    [[0, 1.5, 2, 0.85], [1.5, 0.5, 1, 0.74], [2, 1.7, 0, 0.88]]
                ],
                melB: [
                    [[0, 0.5, 7, 0.9], [0.5, 0.5, 6, 0.82], [1, 0.5, 4, 0.8], [1.5, 1.0, 2, 0.86], [2.5, 1.2, 5, 0.84]],
                    [[0, 1.0, 3, 0.86], [1, 0.5, 4, 0.8], [1.5, 0.5, 5, 0.82], [2, 1.7, 7, 0.9]],
                    [[0, 0.75, 6, 0.88], [0.75, 0.75, 4, 0.82], [1.5, 1.0, 5, 0.86], [2.5, 1.3, 6, 0.88]],
                    [[0, 1.25, 7, 0.9], [1.5, 1.0, 4, 0.84], [2.5, 1.3, 2, 0.8]]
                ]
            },
            { // Level 3: High Canopy (agile swings, bouncy marimba & syncopated flute)
                bpmMul: 1.11, progs: [[0, 2, 5, 4], [3, 0, 2, 4], [5, 2, 3, 6], [0, 3, 4, 0]],
                padInstr: 'string', padStyle: 'broken', bassInstr: 'pizz', bassStyle: 'syncopated',
                arpInstr: 'lute', arpGrid: SYNC_GRID, arpPat: [0, 4, 7, 4, 2, 5],
                melInstrA: 'flute', melInstrB: 'marimba', percStyle: 'wood_canopy',
                melA: [
                    [[0, 0.5, 7, 0.9], [0.75, 0.75, 5, 0.82], [1.5, 0.5, 6, 0.84], [2.25, 0.75, 4, 0.8], [3, 0.8, 7, 0.88]],
                    [[0, 0.75, 8, 0.9], [0.75, 0.75, 7, 0.84], [1.5, 1.0, 5, 0.86], [2.75, 1.0, 4, 0.8]],
                    [[0, 0.5, 4, 0.84], [0.5, 0.5, 5, 0.8], [1, 0.75, 6, 0.86], [2, 0.75, 7, 0.9], [2.75, 1.0, 9, 0.92]],
                    [[0, 1.0, 7, 0.88], [1.25, 0.75, 5, 0.8], [2.25, 1.5, 4, 0.84]]
                ],
                melB: [
                    [[0, 0.5, 2, 0.82], [0.5, 0.5, 4, 0.8], [1, 0.5, 6, 0.84], [1.5, 0.5, 7, 0.88], [2.25, 1.4, 5, 0.84]],
                    [[0, 0.75, 6, 0.86], [1, 0.75, 4, 0.82], [2, 0.75, 2, 0.8], [3, 0.8, 4, 0.84]],
                    [[0, 1.0, 5, 0.86], [1.25, 0.75, 7, 0.9], [2, 1.0, 6, 0.84], [3, 0.9, 4, 0.82]],
                    [[0, 1.5, 2, 0.84], [1.75, 0.75, 4, 0.8], [2.5, 1.3, 0, 0.86]]
                ]
            },
            { // Level 4: Lantern Hollow (twilight mystery, choir/horn/celesta)
                bpmMul: 0.90, progs: [[0, 1, 5, 3], [0, 6, 4, 0], [3, 1, 0, 5], [6, 4, 1, 0]],
                padInstr: 'choir', padStyle: 'choral', bassInstr: 'cello', bassStyle: 'pedal',
                arpInstr: 'harp', arpGrid: SPARSE_GRID, arpPat: [0, 4, 7, 5],
                melInstrA: 'horn', melInstrB: 'celesta', percStyle: 'mystic_sparse',
                melA: [
                    [[0, 2.2, 4, 0.82], [2.5, 1.3, 2, 0.74]],
                    [[0, 1.75, 5, 0.84], [2.0, 1.8, 4, 0.78]],
                    [[0, 1.5, 1, 0.80], [1.75, 1.0, 2, 0.76], [2.75, 1.1, 4, 0.82]],
                    [[0, 3.2, 0, 0.80]]
                ],
                melB: [
                    [[0, 1.5, 7, 0.84], [1.75, 1.0, 6, 0.78], [2.75, 1.1, 4, 0.76]],
                    [[0, 1.75, 3, 0.82], [2.0, 1.8, 5, 0.80]],
                    [[0, 1.25, 4, 0.82], [1.5, 1.25, 2, 0.76], [2.75, 1.1, 1, 0.78]],
                    [[0, 2.8, 0, 0.84]]
                ]
            },
            { // Level 5: Boar Approach (noble war-horn & brass prelude)
                bpmMul: 1.05, progs: [[0, 4, 5, 3], [0, 6, 4, 0], [0, 3, 5, 4], [6, 4, 3, 0]],
                padInstr: 'string', padStyle: 'pulsed', bassInstr: 'cello', bassStyle: 'march',
                arpInstr: 'lute', arpGrid: EIGHTHS, arpPat: [0, 4, 2, 7, 4, 2, 0, 4],
                melInstrA: 'horn', melInstrB: 'brass', percStyle: 'march_light',
                melA: [
                    [[0, 1.5, 0, 0.88], [1.5, 0.5, 2, 0.78], [2, 1.7, 4, 0.9]],
                    [[0, 1.0, 5, 0.88], [1.0, 0.75, 4, 0.82], [1.75, 2.0, 2, 0.86]],
                    [[0, 1.25, 4, 0.88], [1.5, 0.5, 6, 0.82], [2.0, 1.7, 7, 0.92]],
                    [[0, 1.5, 6, 0.88], [1.75, 1.8, 4, 0.86]]
                ],
                melB: [
                    [[0, 1.0, 7, 0.92], [1.0, 0.5, 6, 0.84], [1.5, 1.25, 5, 0.86], [2.75, 1.0, 4, 0.84]],
                    [[0, 1.5, 3, 0.88], [1.5, 0.5, 4, 0.82], [2, 1.7, 5, 0.88]],
                    [[0, 1.0, 6, 0.90], [1.25, 0.75, 7, 0.88], [2.0, 1.7, 4, 0.9]],
                    [[0, 2.2, 0, 0.9], [2.5, 1.2, 4, 0.82]]
                ]
            }
        ],
        cavern: [
            { // Level 6: Crystal Entry (resonant darkbell & choir)
                bpmMul: 0.98, progs: [[0, 1, 0, 6], [0, 3, 1, 0], [0, 5, 1, 4], [3, 1, 6, 0]],
                padInstr: 'choir', padStyle: 'choral', bassInstr: 'cello', bassStyle: 'pedal',
                arpInstr: 'celesta', arpGrid: SPARSE_GRID, arpPat: [0, 4, 2, 7],
                melInstrA: 'darkbell', melInstrB: 'flute', percStyle: 'mystic_sparse',
                melA: [
                    [[0, 2.2, 4, 0.78], [2.5, 1.3, 5, 0.72]],
                    [[0, 1.5, 3, 0.80], [1.75, 2.0, 1, 0.76]],
                    [[0, 2.0, 4, 0.82], [2.0, 1.0, 2, 0.70], [3.0, 0.9, 1, 0.66]],
                    [[0, 3.2, 0, 0.78]]
                ],
                melB: [
                    [[0, 1.2, 7, 0.82], [1.5, 1.0, 5, 0.75], [2.75, 1.1, 4, 0.72]],
                    [[0, 1.5, 3, 0.80], [2.0, 1.75, 5, 0.76]],
                    [[0, 1.0, 6, 0.78], [1.5, 1.0, 4, 0.72], [2.5, 1.3, 1, 0.74]],
                    [[0, 2.8, 0, 0.82]]
                ]
            },
            { // Level 7: Pendulum Chasm (taut clockwork fiddle/marimba)
                bpmMul: 1.09, progs: [[0, 3, 1, 6], [0, 4, 1, 0], [5, 1, 3, 4], [0, 1, 6, 0]],
                padInstr: 'string', padStyle: 'pulsed', bassInstr: 'cello', bassStyle: 'walking',
                arpInstr: 'pizz', arpGrid: EIGHTHS, arpPat: [0, 4, 1, 4, 2, 4, 5, 4],
                melInstrA: 'fiddle', melInstrB: 'marimba', percStyle: 'cav_tick',
                melA: [
                    [[0, 0.5, 4, 0.86], [0.5, 0.5, 5, 0.78], [1.0, 1.0, 3, 0.84], [2.0, 0.75, 1, 0.82], [2.75, 1.0, 0, 0.8]],
                    [[0, 0.75, 1, 0.84], [0.75, 0.75, 3, 0.8], [1.5, 1.0, 4, 0.88], [2.75, 1.0, 6, 0.82]],
                    [[0, 0.5, 5, 0.86], [0.5, 0.5, 4, 0.8], [1.0, 1.0, 1, 0.84], [2.25, 1.4, 3, 0.82]],
                    [[0, 1.25, 1, 0.84], [1.5, 0.75, 2, 0.78], [2.5, 1.3, 0, 0.86]]
                ],
                melB: [
                    [[0, 0.5, 7, 0.88], [0.5, 0.5, 5, 0.82], [1.0, 0.75, 4, 0.84], [2.0, 0.75, 3, 0.8], [3.0, 0.8, 1, 0.82]],
                    [[0, 1.0, 4, 0.86], [1.25, 0.75, 5, 0.82], [2.25, 1.5, 6, 0.86]],
                    [[0, 0.75, 7, 0.88], [0.75, 0.75, 6, 0.82], [1.5, 1.0, 4, 0.86], [2.5, 1.3, 1, 0.84]],
                    [[0, 1.75, 4, 0.86], [2.0, 1.75, 0, 0.84]]
                ]
            },
            { // Level 8: Dragon Gorge (ominous cavernous horn & organ)
                bpmMul: 0.94, progs: [[0, 1, 5, 4], [0, 6, 1, 0], [3, 1, 5, 6], [4, 1, 6, 0]],
                padInstr: 'organ', padStyle: 'swell', bassInstr: 'cello', bassStyle: 'march',
                arpInstr: 'pizz', arpGrid: QUARTERS, arpPat: [0, 1, 4, 5],
                melInstrA: 'horn', melInstrB: 'darkbell', percStyle: 'cav_menace',
                melA: [
                    [[0, 1.75, 0, 0.88], [2.0, 1.75, 1, 0.84]],
                    [[0, 1.5, 4, 0.9], [1.75, 0.75, 3, 0.82], [2.5, 1.3, 1, 0.86]],
                    [[0, 2.0, 5, 0.88], [2.25, 1.5, 4, 0.84]],
                    [[0, 1.75, 1, 0.86], [2.0, 1.8, 0, 0.88]]
                ],
                melB: [
                    [[0, 1.5, 7, 0.9], [1.75, 1.0, 5, 0.84], [2.75, 1.1, 4, 0.82]],
                    [[0, 1.75, 6, 0.88], [2.0, 1.7, 3, 0.84]],
                    [[0, 1.25, 4, 0.86], [1.5, 1.0, 5, 0.84], [2.5, 1.3, 7, 0.9]],
                    [[0, 2.5, 4, 0.88], [2.75, 1.1, 0, 0.82]]
                ]
            },
            { // Level 9: Deep Crystal Grotto (glass & flute serene underground oasis)
                bpmMul: 0.92, progs: [[0, 3, 5, 6], [0, 5, 3, 0], [6, 3, 1, 0], [3, 5, 6, 0]],
                padInstr: 'choir', padStyle: 'sustain', bassInstr: 'cello', bassStyle: 'lyrical',
                arpInstr: 'glass', arpGrid: SYNC_GRID, arpPat: [0, 4, 7, 5, 2, 4],
                melInstrA: 'flute', melInstrB: 'celesta', percStyle: 'glacier_soft',
                melA: [
                    [[0, 1.5, 4, 0.82], [1.75, 1.0, 6, 0.8], [2.75, 1.1, 7, 0.86]],
                    [[0, 2.0, 5, 0.84], [2.25, 1.5, 3, 0.8]],
                    [[0, 1.25, 2, 0.8], [1.5, 1.25, 4, 0.82], [2.75, 1.1, 5, 0.84]],
                    [[0, 3.0, 4, 0.82]]
                ],
                melB: [
                    [[0, 1.5, 7, 0.86], [1.5, 1.0, 8, 0.88], [2.5, 1.3, 7, 0.82]],
                    [[0, 1.75, 5, 0.84], [2.0, 1.7, 6, 0.8]],
                    [[0, 1.0, 4, 0.82], [1.25, 1.0, 3, 0.78], [2.5, 1.3, 1, 0.8]],
                    [[0, 2.8, 0, 0.84]]
                ]
            },
            { // Level 10: Goblin Warren (tense reed & pizz before Goblin King)
                bpmMul: 1.08, progs: [[0, 1, 6, 1], [0, 4, 1, 0], [5, 1, 4, 6], [1, 6, 4, 0]],
                padInstr: 'string', padStyle: 'broken', bassInstr: 'pizz', bassStyle: 'syncopated',
                arpInstr: 'lute', arpGrid: EIGHTHS, arpPat: [0, 1, 4, 1, 5, 4, 1, 0],
                melInstrA: 'reed', melInstrB: 'fiddle', percStyle: 'cav_menace',
                melA: [
                    [[0, 1.0, 1, 0.86], [1.0, 0.5, 3, 0.8], [1.5, 1.0, 4, 0.88], [2.75, 1.0, 5, 0.84]],
                    [[0, 0.75, 4, 0.86], [0.75, 0.75, 1, 0.82], [1.5, 1.25, 0, 0.84], [3.0, 0.8, 3, 0.8]],
                    [[0, 1.0, 5, 0.88], [1.25, 0.75, 6, 0.84], [2.0, 1.0, 4, 0.86], [3.0, 0.8, 1, 0.82]],
                    [[0, 1.5, 3, 0.86], [1.75, 1.8, 1, 0.88]]
                ],
                melB: [
                    [[0, 0.75, 4, 0.88], [0.75, 0.75, 5, 0.84], [1.5, 1.0, 7, 0.9], [2.75, 1.0, 5, 0.84]],
                    [[0, 1.0, 6, 0.88], [1.25, 0.75, 4, 0.84], [2.25, 1.4, 3, 0.82]],
                    [[0, 0.75, 1, 0.86], [0.75, 0.75, 3, 0.82], [1.5, 1.0, 4, 0.88], [2.75, 1.0, 1, 0.84]],
                    [[0, 1.8, 0, 0.9], [2.25, 1.4, 4, 0.82]]
                ]
            }
        ],
        desert: [
            { // Level 11: Sun Dunes (E Phrygian Dominant ney & oud desert caravan)
                bpmMul: 1.00, progs: [[0, 1, 0, 6], [0, 4, 1, 0], [0, 1, 3, 1], [4, 1, 6, 0]],
                padInstr: 'string', padStyle: 'sustain', bassInstr: 'pizz', bassStyle: 'pulse',
                arpInstr: 'oud', arpGrid: EIGHTHS, arpPat: [0, 4, 2, 4, 5, 4, 2, 1],
                melInstrA: 'ney', melInstrB: 'oud', percStyle: 'darbuka_calm',
                melA: [
                    [[0, 1.2, 4, 0.88], [1.25, 0.75, 5, 0.78], [2.0, 1.0, 4, 0.86], [3.0, 0.8, 2, 0.82]],
                    [[0, 0.75, 3, 0.82], [0.75, 0.75, 2, 0.78], [1.5, 1.25, 1, 0.86], [3.0, 0.8, 0, 0.74]],
                    [[0, 1.0, 2, 0.85], [1.0, 0.5, 4, 0.82], [1.5, 1.0, 5, 0.88], [2.5, 1.3, 4, 0.80]],
                    [[0, 1.5, 1, 0.86], [1.75, 0.75, 2, 0.78], [2.5, 1.3, 0, 0.84]]
                ],
                melB: [
                    [[0, 0.5, 4, 0.84], [0.5, 0.5, 5, 0.78], [1.0, 0.5, 6, 0.82], [1.5, 1.2, 4, 0.88], [3.0, 0.8, 2, 0.76]],
                    [[0, 1.0, 3, 0.86], [1.0, 1.0, 1, 0.82], [2.0, 0.75, 2, 0.80], [2.75, 1.0, 4, 0.85]],
                    [[0, 1.5, 7, 0.90], [1.5, 0.75, 6, 0.80], [2.25, 0.75, 5, 0.78], [3.0, 0.8, 4, 0.84]],
                    [[0, 1.25, 2, 0.84], [1.5, 1.0, 1, 0.80], [2.5, 1.25, 0, 0.86]]
                ]
            },
            { // Level 12: Mirage Oasis (warm harp/marimba shimmer under desert stars)
                bpmMul: 0.93, progs: [[0, 3, 1, 0], [5, 1, 4, 0], [0, 6, 3, 1], [4, 3, 1, 0]],
                padInstr: 'string', padStyle: 'swell', bassInstr: 'cello', bassStyle: 'lyrical',
                arpInstr: 'harp', arpGrid: TRIPLET_GRID, arpPat: [0, 2, 4, 7, 4, 2],
                melInstrA: 'flute', melInstrB: 'marimba', percStyle: 'wood_gentle',
                melA: [
                    [[0, 1.5, 2, 0.84], [1.75, 0.75, 4, 0.8], [2.5, 1.3, 5, 0.86]],
                    [[0, 1.25, 4, 0.84], [1.5, 1.0, 3, 0.8], [2.5, 1.3, 2, 0.82]],
                    [[0, 1.0, 6, 0.86], [1.25, 0.75, 5, 0.8], [2.0, 1.7, 4, 0.84]],
                    [[0, 1.5, 2, 0.84], [1.75, 0.75, 1, 0.78], [2.5, 1.3, 0, 0.84]]
                ],
                melB: [
                    [[0, 0.75, 7, 0.88], [1.0, 0.75, 6, 0.82], [2.0, 0.75, 4, 0.84], [3.0, 0.8, 2, 0.8]],
                    [[0, 1.25, 3, 0.84], [1.5, 1.0, 5, 0.86], [2.5, 1.3, 4, 0.82]],
                    [[0, 1.0, 2, 0.82], [1.25, 0.75, 4, 0.84], [2.25, 1.5, 1, 0.84]],
                    [[0, 2.2, 0, 0.86], [2.5, 1.2, 4, 0.78]]
                ]
            },
            { // Level 13: Sandworks & Rituals (propulsive reed & syncopated darbuka)
                bpmMul: 1.08, progs: [[0, 4, 1, 4], [0, 1, 6, 0], [3, 4, 1, 0], [1, 4, 6, 0]],
                padInstr: 'organ', padStyle: 'pulsed', bassInstr: 'pizz', bassStyle: 'syncopated',
                arpInstr: 'oud', arpGrid: SYNC_GRID, arpPat: [0, 1, 4, 5, 4, 2],
                melInstrA: 'reed', melInstrB: 'ney', percStyle: 'darbuka_drive',
                melA: [
                    [[0, 0.75, 1, 0.88], [0.75, 0.75, 2, 0.82], [1.5, 1.0, 4, 0.9], [2.75, 1.0, 5, 0.84]],
                    [[0, 0.5, 4, 0.86], [0.5, 0.5, 3, 0.8], [1.0, 1.0, 2, 0.84], [2.25, 1.4, 1, 0.86]],
                    [[0, 1.0, 5, 0.88], [1.25, 0.75, 4, 0.84], [2.0, 1.0, 2, 0.82], [3.0, 0.8, 4, 0.86]],
                    [[0, 1.25, 1, 0.88], [1.5, 0.75, 0, 0.82], [2.5, 1.3, 4, 0.86]]
                ],
                melB: [
                    [[0, 0.75, 7, 0.9], [0.75, 0.75, 5, 0.84], [1.5, 1.0, 4, 0.88], [2.75, 1.0, 2, 0.82]],
                    [[0, 1.0, 3, 0.86], [1.25, 0.75, 4, 0.84], [2.0, 1.7, 6, 0.88]],
                    [[0, 0.5, 5, 0.86], [0.5, 0.5, 4, 0.82], [1.0, 1.0, 2, 0.84], [2.25, 1.5, 1, 0.86]],
                    [[0, 1.75, 2, 0.86], [2.0, 1.75, 0, 0.88]]
                ]
            },
            { // Level 14: Sun Tomb (solemn pharaonic choir, horn & darkbell)
                bpmMul: 0.88, progs: [[0, 1, 6, 0], [0, 3, 1, 0], [4, 1, 5, 6], [1, 6, 1, 0]],
                padInstr: 'choir', padStyle: 'choral', bassInstr: 'cello', bassStyle: 'pedal',
                arpInstr: 'harp', arpGrid: SPARSE_GRID, arpPat: [0, 4, 1, 7],
                melInstrA: 'horn', melInstrB: 'darkbell', percStyle: 'mystic_sparse',
                melA: [
                    [[0, 2.0, 4, 0.86], [2.25, 1.5, 1, 0.82]],
                    [[0, 1.75, 2, 0.84], [2.0, 1.8, 0, 0.82]],
                    [[0, 1.5, 5, 0.86], [1.75, 1.0, 4, 0.8], [2.75, 1.1, 2, 0.78]],
                    [[0, 3.1, 1, 0.84]]
                ],
                melB: [
                    [[0, 1.75, 7, 0.88], [2.0, 1.7, 5, 0.82]],
                    [[0, 1.5, 4, 0.86], [1.75, 1.0, 2, 0.8], [2.75, 1.1, 1, 0.82]],
                    [[0, 2.0, 3, 0.84], [2.25, 1.5, 4, 0.86]],
                    [[0, 2.9, 0, 0.86]]
                ]
            },
            { // Level 15: Orb Approach (heroic brass & ney sandstorm build)
                bpmMul: 1.07, progs: [[0, 1, 4, 1], [0, 6, 4, 0], [0, 3, 4, 1], [6, 4, 1, 0]],
                padInstr: 'string', padStyle: 'pulsed', bassInstr: 'cello', bassStyle: 'march',
                arpInstr: 'oud', arpGrid: EIGHTHS, arpPat: [0, 2, 4, 5, 4, 2, 1, 4],
                melInstrA: 'brass', melInstrB: 'ney', percStyle: 'darbuka_drive',
                melA: [
                    [[0, 1.5, 0, 0.9], [1.5, 0.5, 1, 0.82], [2.0, 1.7, 4, 0.92]],
                    [[0, 1.0, 5, 0.88], [1.25, 0.75, 4, 0.84], [2.0, 1.7, 2, 0.86]],
                    [[0, 1.25, 4, 0.9], [1.5, 0.5, 6, 0.84], [2.0, 1.7, 7, 0.92]],
                    [[0, 1.5, 5, 0.88], [1.75, 1.8, 1, 0.86]]
                ],
                melB: [
                    [[0, 0.75, 7, 0.9], [0.75, 0.75, 5, 0.84], [1.5, 1.25, 4, 0.88], [2.75, 1.0, 2, 0.84]],
                    [[0, 1.25, 1, 0.88], [1.5, 0.75, 2, 0.84], [2.25, 1.5, 4, 0.9]],
                    [[0, 1.0, 5, 0.9], [1.25, 0.75, 4, 0.84], [2.0, 1.7, 1, 0.88]],
                    [[0, 2.0, 0, 0.9], [2.25, 1.5, 4, 0.84]]
                ]
            }
        ],
        glacier: [
            { // Level 16: Aurora Slopes (F# Lydian flute, celesta & glass shimmer)
                bpmMul: 1.00, progs: [[0, 1, 0, 4], [5, 1, 4, 0], [0, 3, 4, 1], [5, 3, 1, 0]],
                padInstr: 'choir', padStyle: 'choral', bassInstr: 'horn', bassStyle: 'pedal',
                arpInstr: 'glass', arpGrid: QUARTERS, arpPat: [0, 2, 4, 7],
                melInstrA: 'flute', melInstrB: 'celesta', percStyle: 'glacier_soft',
                melA: [
                    [[0, 2.0, 4, 0.82], [2.0, 1.8, 6, 0.86]],
                    [[0, 1.5, 3, 0.84], [1.75, 2.0, 4, 0.80]],
                    [[0, 1.5, 7, 0.88], [1.75, 1.0, 6, 0.82], [2.75, 1.1, 4, 0.78]],
                    [[0, 3.2, 2, 0.80]]
                ],
                melB: [
                    [[0, 1.5, 6, 0.84], [1.5, 1.0, 7, 0.88], [2.5, 1.3, 4, 0.80]],
                    [[0, 2.0, 3, 0.82], [2.25, 1.5, 1, 0.78]],
                    [[0, 1.5, 4, 0.84], [1.5, 1.25, 6, 0.86], [2.75, 1.1, 8, 0.90]],
                    [[0, 3.0, 4, 0.82]]
                ]
            },
            { // Level 17: Frozen Pines (brisk Nordic fiddle & harp trek)
                bpmMul: 1.10, progs: [[0, 4, 5, 1], [0, 2, 4, 0], [5, 4, 3, 1], [0, 1, 4, 0]],
                padInstr: 'string', padStyle: 'broken', bassInstr: 'cello', bassStyle: 'walking',
                arpInstr: 'harp', arpGrid: EIGHTHS, arpPat: [0, 4, 2, 7, 4, 2, 3, 4],
                melInstrA: 'fiddle', melInstrB: 'flute', percStyle: 'wood_folk',
                melA: [
                    [[0, 0.75, 4, 0.86], [0.75, 0.75, 3, 0.8], [1.5, 1.0, 4, 0.88], [2.75, 1.0, 7, 0.86]],
                    [[0, 1.0, 6, 0.86], [1.0, 0.5, 5, 0.8], [1.5, 1.0, 4, 0.84], [2.75, 1.0, 2, 0.8]],
                    [[0, 0.75, 3, 0.84], [0.75, 0.75, 4, 0.82], [1.5, 1.25, 6, 0.88], [3.0, 0.8, 4, 0.8]],
                    [[0, 1.5, 2, 0.84], [1.75, 1.8, 0, 0.86]]
                ],
                melB: [
                    [[0, 0.75, 7, 0.88], [0.75, 0.75, 6, 0.82], [1.5, 1.0, 4, 0.86], [2.75, 1.0, 3, 0.82]],
                    [[0, 1.0, 4, 0.86], [1.25, 0.75, 6, 0.84], [2.25, 1.5, 7, 0.9]],
                    [[0, 1.0, 8, 0.9], [1.25, 0.75, 7, 0.84], [2.0, 1.7, 4, 0.86]],
                    [[0, 1.5, 3, 0.84], [1.75, 1.8, 0, 0.86]]
                ]
            },
            { // Level 18: Frost Chasm (sweeping alpine horns & soaring strings)
                bpmMul: 0.96, progs: [[0, 3, 4, 5], [1, 4, 0, 5], [3, 1, 4, 6], [5, 4, 1, 0]],
                padInstr: 'choir', padStyle: 'swell', bassInstr: 'cello', bassStyle: 'march',
                arpInstr: 'glass', arpGrid: TRIPLET_GRID, arpPat: [0, 3, 4, 7, 4, 3],
                melInstrA: 'horn', melInstrB: 'string', percStyle: 'march_light',
                melA: [
                    [[0, 1.75, 4, 0.88], [2.0, 1.75, 7, 0.9]],
                    [[0, 1.5, 6, 0.86], [1.75, 0.75, 4, 0.82], [2.5, 1.3, 3, 0.84]],
                    [[0, 1.5, 2, 0.84], [1.75, 0.75, 4, 0.84], [2.5, 1.3, 6, 0.88]],
                    [[0, 2.2, 4, 0.86], [2.5, 1.3, 0, 0.8]]
                ],
                melB: [
                    [[0, 1.5, 7, 0.9], [1.75, 1.0, 8, 0.88], [2.75, 1.1, 6, 0.84]],
                    [[0, 1.75, 4, 0.86], [2.0, 1.7, 3, 0.82]],
                    [[0, 1.5, 1, 0.84], [1.75, 0.75, 3, 0.82], [2.5, 1.3, 4, 0.88]],
                    [[0, 2.8, 0, 0.86]]
                ]
            },
            { // Level 19: Crystal Vault (music-box celesta & marimba sparkle)
                bpmMul: 1.04, progs: [[0, 2, 3, 4], [5, 2, 1, 0], [0, 6, 4, 2], [3, 1, 4, 0]],
                padInstr: 'string', padStyle: 'sustain', bassInstr: 'pizz', bassStyle: 'lyrical',
                arpInstr: 'harp', arpGrid: SYNC_GRID, arpPat: [0, 4, 7, 3, 4, 2],
                melInstrA: 'celesta', melInstrB: 'marimba', percStyle: 'glacier_soft',
                melA: [
                    [[0, 0.75, 7, 0.86], [0.75, 0.75, 6, 0.8], [1.5, 1.0, 4, 0.84], [2.75, 1.0, 3, 0.8]],
                    [[0, 1.0, 4, 0.84], [1.25, 0.75, 7, 0.88], [2.25, 1.5, 6, 0.82]],
                    [[0, 0.75, 4, 0.84], [1.0, 0.75, 3, 0.8], [2.0, 0.75, 2, 0.82], [3.0, 0.8, 4, 0.84]],
                    [[0, 1.5, 1, 0.82], [1.75, 1.8, 0, 0.84]]
                ],
                melB: [
                    [[0, 0.75, 4, 0.84], [0.75, 0.75, 6, 0.82], [1.5, 1.0, 7, 0.88], [2.75, 1.0, 9, 0.9]],
                    [[0, 1.25, 7, 0.86], [1.5, 1.0, 6, 0.82], [2.5, 1.3, 4, 0.82]],
                    [[0, 1.0, 3, 0.84], [1.25, 0.75, 4, 0.84], [2.25, 1.5, 2, 0.82]],
                    [[0, 2.2, 4, 0.84], [2.5, 1.2, 0, 0.8]]
                ]
            },
            { // Level 20: Sentinel Peak (icy fortress brass & horn ascent)
                bpmMul: 1.09, progs: [[0, 1, 4, 5], [0, 3, 4, 0], [5, 3, 1, 4], [1, 4, 3, 0]],
                padInstr: 'choir', padStyle: 'pulsed', bassInstr: 'cello', bassStyle: 'march',
                arpInstr: 'pizz', arpGrid: EIGHTHS, arpPat: [0, 4, 3, 7, 4, 3, 1, 4],
                melInstrA: 'brass', melInstrB: 'horn', percStyle: 'march_light',
                melA: [
                    [[0, 1.25, 0, 0.9], [1.5, 0.5, 3, 0.82], [2.0, 1.7, 4, 0.92]],
                    [[0, 1.0, 6, 0.88], [1.25, 0.75, 4, 0.84], [2.0, 1.7, 3, 0.84]],
                    [[0, 1.25, 4, 0.9], [1.5, 0.5, 6, 0.84], [2.0, 1.7, 7, 0.94]],
                    [[0, 1.5, 6, 0.88], [1.75, 1.8, 4, 0.86]]
                ],
                melB: [
                    [[0, 1.0, 7, 0.92], [1.25, 0.75, 8, 0.9], [2.0, 1.7, 6, 0.86]],
                    [[0, 1.25, 4, 0.88], [1.5, 0.75, 3, 0.82], [2.25, 1.5, 1, 0.84]],
                    [[0, 1.0, 3, 0.86], [1.25, 0.75, 4, 0.86], [2.0, 1.7, 7, 0.92]],
                    [[0, 2.0, 4, 0.88], [2.25, 1.5, 0, 0.86]]
                ]
            }
        ],
        citadel: [
            { // Level 21: Outer Foundry (C Harmonic Minor brass/fiddle industrial march)
                bpmMul: 1.00, progs: [[0, 5, 3, 4], [0, 3, 4, 0], [0, 2, 5, 4], [3, 5, 4, 0]],
                padInstr: 'organ', padStyle: 'pulsed', bassInstr: 'cello', bassStyle: 'march',
                arpInstr: 'pizz', arpGrid: EIGHTHS, arpPat: [0, 2, 4, 2, 7, 4, 2, 4],
                melInstrA: 'brass', melInstrB: 'fiddle', percStyle: 'citadel_heavy',
                melA: [
                    [[0, 1.5, 4, 0.88], [1.5, 0.5, 5, 0.80], [2.0, 1.0, 6, 0.90], [3.0, 0.9, 4, 0.82]],
                    [[0, 1.0, 3, 0.85], [1.0, 1.0, 2, 0.80], [2.0, 1.8, 0, 0.86]],
                    [[0, 1.0, 4, 0.88], [1.0, 1.0, 6, 0.86], [2.0, 1.0, 5, 0.82], [3.0, 0.9, 3, 0.78]],
                    [[0, 2.0, 4, 0.90], [2.25, 1.5, 6, 0.84]]
                ],
                melB: [
                    [[0, 0.75, 7, 0.90], [0.75, 0.75, 6, 0.84], [1.5, 1.0, 4, 0.86], [2.5, 1.3, 5, 0.82]],
                    [[0, 1.0, 3, 0.86], [1.0, 0.5, 4, 0.80], [1.5, 1.25, 5, 0.86], [3.0, 0.8, 6, 0.88]],
                    [[0, 1.0, 4, 0.88], [1.0, 1.0, 2, 0.82], [2.0, 1.0, 1, 0.78], [3.0, 0.9, 6, 0.84]],
                    [[0, 2.8, 0, 0.90]]
                ]
            },
            { // Level 22: Clockwork Works (syncopated reed & fiddle machine)
                bpmMul: 1.07, progs: [[0, 3, 6, 4], [0, 5, 4, 0], [3, 6, 2, 4], [5, 3, 4, 0]],
                padInstr: 'organ', padStyle: 'broken', bassInstr: 'pizz', bassStyle: 'syncopated',
                arpInstr: 'lute', arpGrid: SYNC_GRID, arpPat: [0, 4, 6, 4, 2, 4],
                melInstrA: 'fiddle', melInstrB: 'reed', percStyle: 'clockwork',
                melA: [
                    [[0, 0.5, 7, 0.88], [0.5, 0.5, 6, 0.82], [1.0, 1.0, 4, 0.86], [2.25, 0.75, 2, 0.82], [3.0, 0.8, 4, 0.84]],
                    [[0, 0.75, 3, 0.86], [0.75, 0.75, 4, 0.82], [1.5, 1.0, 6, 0.88], [2.75, 1.0, 5, 0.84]],
                    [[0, 0.5, 4, 0.86], [0.5, 0.5, 2, 0.8], [1.0, 1.0, 0, 0.86], [2.25, 1.5, 6, 0.84]],
                    [[0, 1.25, 4, 0.88], [1.5, 0.75, 3, 0.82], [2.5, 1.3, 0, 0.88]]
                ],
                melB: [
                    [[0, 0.75, 6, 0.88], [0.75, 0.75, 7, 0.9], [1.5, 1.0, 9, 0.9], [2.75, 1.0, 7, 0.84]],
                    [[0, 1.0, 5, 0.86], [1.25, 0.75, 4, 0.82], [2.25, 1.5, 2, 0.84]],
                    [[0, 0.75, 3, 0.84], [0.75, 0.75, 5, 0.82], [1.5, 1.0, 6, 0.88], [2.75, 1.0, 4, 0.86]],
                    [[0, 1.75, 2, 0.84], [2.0, 1.75, 0, 0.88]]
                ]
            },
            { // Level 23: Ember Furnace (heavy fuzzbass furnace & horn/brass roar)
                bpmMul: 0.96, progs: [[0, 1, 5, 4], [0, 4, 6, 0], [3, 5, 1, 4], [5, 1, 4, 0]],
                padInstr: 'organ', padStyle: 'swell', bassInstr: 'fuzzbass', bassStyle: 'pulse',
                arpInstr: 'pizz', arpGrid: QUARTERS, arpPat: [0, 4, 5, 4],
                melInstrA: 'horn', melInstrB: 'brass', percStyle: 'cav_menace',
                melA: [
                    [[0, 1.75, 0, 0.9], [2.0, 1.75, 5, 0.88]],
                    [[0, 1.5, 4, 0.9], [1.75, 0.75, 3, 0.82], [2.5, 1.3, 2, 0.84]],
                    [[0, 1.5, 6, 0.9], [1.75, 0.75, 5, 0.84], [2.5, 1.3, 4, 0.88]],
                    [[0, 2.0, 2, 0.86], [2.25, 1.5, 0, 0.88]]
                ],
                melB: [
                    [[0, 1.5, 7, 0.92], [1.75, 1.0, 6, 0.86], [2.75, 1.1, 4, 0.84]],
                    [[0, 1.75, 5, 0.88], [2.0, 1.7, 3, 0.84]],
                    [[0, 1.25, 4, 0.88], [1.5, 1.0, 6, 0.86], [2.5, 1.3, 7, 0.92]],
                    [[0, 2.5, 4, 0.9], [2.75, 1.1, 0, 0.86]]
                ]
            },
            { // Level 24: High Gothic Spire (tragic choir, fiddle & darkbell)
                bpmMul: 0.91, progs: [[0, 6, 5, 4], [0, 3, 6, 0], [5, 3, 2, 4], [6, 5, 4, 0]],
                padInstr: 'choir', padStyle: 'choral', bassInstr: 'cello', bassStyle: 'walking',
                arpInstr: 'celesta', arpGrid: SPARSE_GRID, arpPat: [0, 4, 6, 7],
                melInstrA: 'fiddle', melInstrB: 'darkbell', percStyle: 'mystic_sparse',
                melA: [
                    [[0, 1.5, 6, 0.86], [1.75, 1.0, 4, 0.82], [2.75, 1.1, 5, 0.84]],
                    [[0, 1.75, 3, 0.86], [2.0, 1.75, 2, 0.82]],
                    [[0, 1.5, 4, 0.86], [1.75, 1.0, 6, 0.88], [2.75, 1.1, 7, 0.9]],
                    [[0, 3.0, 4, 0.84]]
                ],
                melB: [
                    [[0, 1.5, 7, 0.88], [1.75, 1.0, 6, 0.84], [2.75, 1.1, 5, 0.82]],
                    [[0, 1.75, 4, 0.86], [2.0, 1.7, 3, 0.82]],
                    [[0, 1.5, 2, 0.84], [1.75, 1.0, 4, 0.84], [2.75, 1.1, 6, 0.88]],
                    [[0, 2.9, 0, 0.88]]
                ]
            },
            { // Level 25: Final Ascent (grand orchestral brass, choir & harp climax)
                bpmMul: 1.06, progs: [[0, 5, 6, 4], [0, 3, 5, 0], [6, 5, 3, 4], [0, 6, 4, 0]],
                padInstr: 'choir', padStyle: 'swell', bassInstr: 'cello', bassStyle: 'march',
                arpInstr: 'harp', arpGrid: EIGHTHS, arpPat: [0, 4, 6, 7, 6, 4, 2, 4],
                melInstrA: 'brass', melInstrB: 'horn', percStyle: 'citadel_heavy',
                melA: [
                    [[0, 1.5, 0, 0.92], [1.5, 0.5, 4, 0.86], [2.0, 1.7, 7, 0.94]],
                    [[0, 1.25, 6, 0.9], [1.5, 0.5, 5, 0.84], [2.0, 1.7, 4, 0.9]],
                    [[0, 1.0, 5, 0.88], [1.25, 0.75, 6, 0.88], [2.0, 1.7, 7, 0.94]],
                    [[0, 1.5, 6, 0.9], [1.75, 1.8, 4, 0.88]]
                ],
                melB: [
                    [[0, 1.0, 7, 0.92], [1.0, 0.5, 8, 0.88], [1.5, 1.25, 6, 0.88], [2.75, 1.0, 4, 0.86]],
                    [[0, 1.5, 3, 0.88], [1.5, 0.5, 4, 0.84], [2.0, 1.7, 6, 0.92]],
                    [[0, 1.25, 7, 0.92], [1.5, 0.75, 6, 0.86], [2.25, 1.5, 4, 0.9]],
                    [[0, 2.2, 0, 0.92], [2.5, 1.3, 4, 0.86]]
                ]
            }
        ]
    };

    const BIOME_DRAMA = {
        woodland: { ost: 'ostinato', stab: 'horn', snare: 'snare' },
        cavern: { ost: 'ostinato', stab: 'brass', snare: 'anvil' },
        desert: { ost: 'oud', stab: 'horn', snare: 'darbuka_tek' },
        glacier: { ost: 'ostinato', stab: 'horn', snare: 'snare' },
        citadel: { ost: 'fuzzbass', stab: 'brass', snare: 'anvil' }
    };

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

    function suiteFor(theme, seed) {
        const list = theme && BIOME_SUITES[theme.biome];
        if (!list || !list.length) return null;
        return list[((seed >>> 0) % list.length)];
    }

    function composeBiome(th, layer, bar, seed, h) {
        const spec = suiteFor(th, seed);
        if (!spec) return null;
        const b16 = ((bar % 16) + 16) % 16;
        const sec = Math.floor(b16 / 4);
        const phr = b16 % 4;
        const d = spec.progs[sec][phr];
        const r = th.root;
        const out = [];

        if (layer === 'pad') {
            emitPad(out, h, th, d, r, spec.padInstr, spec.padStyle, phr);
            return out;
        }
        if (layer === 'bass') {
            emitBass(out, h, th, d, r, spec.bassInstr, spec.bassStyle, phr);
            return out;
        }
        if (layer === 'arp') {
            const grid = spec.arpGrid || th.arpGrid || EIGHTHS;
            const pat = spec.arpPat;
            const gap = grid.length > 1 ? grid[1] - grid[0] : 1;
            grid.forEach(function (t, i) {
                const deg = d + pat[(i + sec) % pat.length];
                out.push({
                    t: t,
                    dur: Math.max(0.42, gap * 1.25),
                    midi: h.fold(h.degMidi(th, deg) + 12, r + 7, r + 24),
                    vel: t % 1 === 0 ? 0.82 : 0.62,
                    type: spec.arpInstr
                });
            });
            return out;
        }
        if (layer === 'melody') {
            const isB = sec === 1 || sec === 3;
            const phrases = isB ? spec.melB : spec.melA;
            const instr = isB ? spec.melInstrB : spec.melInstrA;
            const notes = phrases[phr];
            for (let i = 0; i < notes.length; i++) {
                const n = notes[i];
                out.push({
                    t: n[0],
                    dur: n[1],
                    midi: h.fold(h.degMidi(th, d + n[2]) + 12, r + 10, r + 28),
                    vel: n[3],
                    type: instr
                });
            }
            return out;
        }
        if (layer === 'perc') {
            emitCalmPerc(out, spec.percStyle, phr, sec);
            return out;
        }
        if (layer === 'boss') {
            return out;
        }
        if (layer === 'drama') {
            const dr = BIOME_DRAMA[th.biome] || BIOME_DRAMA.woodland;
            const ostPat = [0, 0, 4, 0, 2, 4, 1, 0];
            EIGHTHS.forEach(function (t, i) {
                const deg = d + ostPat[i];
                out.push({
                    t: t,
                    dur: 0.42,
                    midi: h.fold(h.degMidi(th, deg), r - 12, r + 2),
                    vel: i % 2 === 0 ? 0.94 : 0.72,
                    type: dr.ost
                });
            });
            [[0, [d, d + 4], 0.94], [1.5, [d, d + 2], 0.82], [2.75, [d + 4, d + 7], 0.88]].forEach(function (st) {
                st[1].forEach(function (deg) {
                    out.push({ t: st[0], dur: 0.48, midi: h.fold(h.degMidi(th, deg), r - 4, r + 10), vel: st[2], type: dr.stab });
                });
            });
            [0, 1.5, 2.5].forEach(function (t, i) { out.push({ t: t, dur: 0.55, midi: null, vel: i === 0 ? 0.92 : 0.78, type: 'taiko' }); });
            [1, 3].forEach(function (t) { out.push({ t: t, dur: 0.22, midi: null, vel: 0.82, type: dr.snare }); });
            [0.5, 1.5, 2.5, 3.5].forEach(function (t) { out.push({ t: t, dur: 0.12, midi: null, vel: 0.56, type: 'hat' }); });
            if (phr === 3) {
                out.push({ t: 0, dur: 3.9, midi: null, vel: 0.92, type: 'swell' });
                [3.25, 3.5, 3.75].forEach(function (t) { out.push({ t: t, dur: 0.16, midi: null, vel: 0.78, type: 'tom' }); });
            }
            return out;
        }
        return null;
    }

    function composeBoss(th, layer, bar, seed, h) {
        const b8 = ((bar % 8) + 8) % 8;
        const sec = Math.floor(b8 / 4);
        const phr = b8 % 4;
        const d = th.prog[sec][phr];
        const r = th.root;
        const sVar = ((seed >>> 0) % 2);
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
            const notes = sVar ? [[0, 1.1, 0.98, lo], [1.25, 0.55, 0.82, fi], [2, 1.2, 0.92, lo], [3.25, 0.65, 0.86, fi]]
                : [[0, 1.35, 0.98, lo], [1.5, 0.45, 0.82, fi], [2, 1.2, 0.92, lo], [3.25, 0.65, 0.85, fi]];
            notes.forEach(function (n) { out.push({ t: n[0], dur: n[1], midi: n[3], vel: n[2], type: th.bassInstr || 'bass' }); });
            return out;
        }
        if (layer === 'arp') {
            const grid = th.arpGrid || EIGHTHS;
            const gap = grid.length > 1 ? grid[1] - grid[0] : 0.5;
            const pat = sVar ? [0, 4, 2, 7] : [0, 2, 4, 7];
            grid.forEach(function (t, i) {
                const deg = d + pat[i % 4];
                out.push({ t: t, dur: Math.max(0.45, gap * 1.2), midi: h.fold(h.degMidi(th, deg) + 12, r + 7, r + 24), vel: i % 2 === 0 ? 0.85 : 0.65, type: th.arpInstr });
            });
            return out;
        }
        if (layer === 'melody') {
            const mList = BOSS_MELODIES[th.biome];
            const notes = mList ? mList[phr] : null;
            if (!notes) return null;
            const lo = th.biome === 'desert' ? r - 12 : (th.biome === 'citadel' ? r - 7 : r);
            const hi = th.biome === 'desert' ? r - 1 : (th.biome === 'citadel' ? r + 6 : r + 12);
            notes.forEach(function (n) {
                out.push({ t: n[0], dur: n[1], midi: h.fold(h.degMidi(th, n[2] + (sec === 1 && phr === 2 ? 1 : 0)), lo, hi), vel: n[3], type: th.leadInstr || 'horn' });
            });
            return out;
        }
        if (layer === 'perc') {
            const dr = th.drums;
            dr.kick.forEach(function (t) { out.push({ t: t, dur: 0.3, midi: null, vel: 0.92, type: 'kick' }); });
            dr.snare.forEach(function (t) { out.push({ t: t, dur: 0.2, midi: null, vel: 0.68, type: 'snare' }); });
            dr.hat.forEach(function (t) { out.push({ t: t, dur: 0.12, midi: null, vel: 0.48, type: 'hat' }); });
            (dr.tom || []).forEach(function (t, i) { out.push({ t: t, dur: 0.25, midi: null, vel: 0.75 + 0.1 * (i % 2), type: 'tom' }); });
            (dr.taiko || []).forEach(function (t, i) { out.push({ t: t, dur: 0.42, midi: null, vel: 0.82 + 0.12 * (i % 2), type: 'taiko' }); });
            (dr.tick || []).forEach(function (t, i) { out.push({ t: t, dur: 0.05, midi: null, vel: dr.tickQuiet ? 0.32 : (i % 4 === 0 ? 0.85 : 0.5), type: 'tick' }); });
            (dr.clank || []).forEach(function (t) { out.push({ t: t, dur: 0.35, midi: null, vel: 0.78, type: 'clank' }); });
            return out;
        }
        if (layer === 'boss') {
            // Retain exact 15-event structure on bar 0 (1 drone + 6 stabs + 8 ostinato) verified by sim tests.
            out.push({ t: 0, dur: 3.9, midi: h.fold(h.degMidi(th, 0), r - 24, r - 13), vel: th.droneVel || 1, type: 'drone' });
            [[0, [d, d + 1], 1], [1.5, [d, d + 4], 0.82], [2.75, [d + 1, d], 0.9]].forEach(function (st) {
                st[1].forEach(function (deg) {
                    out.push({ t: st[0], dur: 0.42, midi: h.fold(h.degMidi(th, deg), r - 12, r - 1), vel: st[2], type: 'stab' });
                });
            });
            const ostPat = [0, 0, 1, 0, 0, 4, 0, 1];
            if (phr === 3) out.push({ t: 0, dur: 3.9, midi: null, vel: 1, type: 'swell' });
            (th.ostGrid || EIGHTHS).forEach(function (t, i) {
                const fast = (th.ostGrid || EIGHTHS).length > 8;
                const deg = d + ostPat[(fast ? i >> 1 : i) % 8];
                out.push({
                    t: t, dur: fast ? 0.2 : 0.4,
                    midi: h.fold(h.degMidi(th, deg), th.ostLow ? r - 18 : r - 12, th.ostLow ? r - 6 : r + 2),
                    vel: i % 2 === 0 ? 0.98 : 0.72,
                    type: th.ostInstr || 'ostinato'
                });
            });
            return out;
        }
        if (layer === 'drama') {
            const stabType = th.biome === 'glacier' ? 'glass' : (th.biome === 'cavern' ? 'darkbell' : 'brass');
            [[0.5, d + 4, 0.86], [1.25, d + 1, 0.8], [2.25, d + 7, 0.9], [3.5, d + 4, 0.84]].forEach(function (n) {
                out.push({ t: n[0], dur: 0.36, midi: h.fold(h.degMidi(th, n[1]), r + 4, r + 20), vel: n[2], type: stabType });
            });
            [0, 0.75, 1.5, 2.25, 3].forEach(function (t, i) { out.push({ t: t, dur: 0.45, midi: null, vel: i % 2 === 0 ? 0.95 : 0.8, type: 'taiko' }); });
            [1, 2.5, 3.5].forEach(function (t) { out.push({ t: t, dur: 0.2, midi: null, vel: 0.82, type: 'snare' }); });
            if (phr % 2 === 0) out.push({ t: 0, dur: 0.48, midi: null, vel: 0.62, type: 'anvil' });
            EIGHTHS.forEach(function (t) { out.push({ t: t, dur: 0.1, midi: null, vel: 0.54, type: 'hat' }); });
            if (phr === 3) out.push({ t: 0, dur: 3.9, midi: null, vel: 1.0, type: 'swell' });
            return out;
        }
        return null;
    }

    root.RunenpfadMusicScores = {
        stageTempoMul: function (theme, seed) {
            const spec = !theme || theme.boss ? null : suiteFor(theme, seed);
            return spec ? (spec.bpmMul || 1.0) : 1.0;
        },
        compose: function (theme, layer, bar, seed, helpers) {
            if (!theme || !helpers) return null;
            if (theme.boss && /^boss_/.test(theme.id)) return composeBoss(theme, layer, bar, seed, helpers);
            if (!theme.boss && BIOME_SUITES[theme.biome]) return composeBiome(theme, layer, bar, seed, helpers);
            return null;
        }
    };
})(typeof window !== 'undefined' ? window : globalThis);
