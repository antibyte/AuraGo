/* Runenpfad story music: three cutscene loops (dawn, dark, triumph). DOM-free; played through music.setOverride(). */
(function (root) {
    'use strict';

    const EIGHTHS = [0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5];
    const T = function () { return root.RunenpfadPlaceMusic.tools; };

    // ---- dark: low register only, cold metal and glass bells, minor seconds and tritones against a rough drone, a dry clock ----
    function composeDark(th, layer, bar, seed) {
        const t = T();
        const rng = t.rngFor(th, layer, bar, seed);
        const out = [];
        if (layer === 'pad') {
            out.push(t.note(0, 4.3, th.root, 1, 'growl'));
            if (bar % 8 === 5) out.push(t.note(2, 2, th.root + 1, 0.6, 'growl')); // minor second against the drone
            if (bar % 4 === 2) out.push(t.note(0, 4, th.root + 12, 0.7, 'metal'));
            if (bar % 4 === 3) out.push(t.note(1, 3, th.root + 18, 0.5, 'metal')); // tritone over the octave
        } else if (layer === 'melody') {
            if (rng() < 0.5) out.push(t.note([0.5, 1.5, 2.5][Math.floor(rng() * 3)], 2, th.root + 12 + [0, 1, 6, 13][Math.floor(rng() * 4)], 0.9, 'darkbell'));
            if (rng() < 0.2) out.push(t.note(3, 1, th.root + 24 + (rng() < 0.5 ? 1 : 6), 0.55, 'glass'));
        } else if (layer === 'perc') {
            [0, 1, 2, 3].forEach(function (b) { out.push(t.note(b, 0.1, null, b % 2 ? 0.6 : 1, 'clock')); });
            if (bar % 4 === 0) out.push(t.note(0, 0.4, null, 0.6, 'taiko'));
        }
        return out;
    }

    // ---- triumph: F major fanfare, dotted brass over fuller chords, timpani on the downbeats, snare rolls ----
    const FANFARE = [[0, 0.75, 0], [0.75, 0.25, 0], [1, 0.5, 3], [1.5, 0.5, 4]]; // pulse, rising fourth, rising fifth
    const LANDINGS = [[[2, 1, 7], [3, 1, 4]], [[2, 1.5, 7], [3.5, 0.5, 9]], [[2, 0.5, 4], [2.5, 0.5, 7], [3, 1, 9]], [[2, 1, 7], [3, 0.5, 9], [3.5, 0.5, 7]]];
    const FINAL = [[2, 2, 7]];

    function composeTriumph(th, layer, bar) {
        const t = T();
        const d = t.chordOf(th, bar);
        const ph = bar % 4;
        const last = bar % 8 === 7;
        const out = [];
        if (layer === 'melody') return t.figure(th, d, FANFARE.concat(last ? FINAL : LANDINGS[ph]), 7, 'brass', 1, th.root + 10, th.root + 26);
        if (layer === 'pad') {
            [[0, 1.5], [2, last ? 2 : 1]].forEach(function (c) {
                [0, 2, 4].forEach(function (s) { out.push(t.note(c[0], c[1], t.fold(t.deg(th, d + s), th.root - 2, th.root + 12), 0.55, 'horn')); });
            });
        } else if (layer === 'bass') {
            out.push(t.note(0, 1.4, t.fold(t.deg(th, d), th.root - 14, th.root - 3), 1, 'bass'));
            out.push(t.note(2, 1.4, t.fold(t.deg(th, d + 4), th.root - 14, th.root - 3), 0.9, 'bass'));
        } else if (layer === 'perc') {
            out.push(t.note(0, 0.4, null, 1, 'taiko'), t.note(2, 0.4, null, 0.9, 'taiko'));
            [1, 3].forEach(function (b) { out.push(t.note(b, 0.2, null, 0.75, 'snare')); });
            if (ph === 3) for (let i = 0; i < 8; i++) out.push(t.note(3 + i * 0.125, 0.1, null, 0.35 + i * 0.07, 'snare')); // roll into the next phrase
        }
        return out;
    }

    const SPECS = {
        // Dawn: G major, slow, harp arpeggios under a bell melody, no drums.
        story_dawn: { root: 55, scale: 'ionian', bpm: 66, prog: [[0, 4, 5, 3], [0, 3, 4, 0]], arpInstr: 'harp', melInstr: 'bell', arpGrid: EIGHTHS, vol: 0.9, noFill: true },
        // Dark: A locrian (no major third), 56 bpm, hand-written, see composeDark.
        story_dark: { root: 33, scale: 'locrian', bpm: 56, prog: [[0, 0, 0, 0], [0, 0, 0, 0]], compose: composeDark, perc: 1, vol: 1 },
        // Triumph: F major, 120 bpm, brass fanfare, the loudest of the story loops.
        story_triumph: { root: 53, scale: 'ionian', bpm: 120, prog: [[0, 3, 4, 0], [0, 3, 4, 0]], compose: composeTriumph, perc: 1.2, vol: 1.3 }
    };
    const IDS = Object.keys(SPECS);

    // Registers the story themes into RunenpfadMusic.THEMES (place-music.js adds the modes, voices and tune helpers they use).
    function install(Music) {
        const place = root.RunenpfadPlaceMusic;
        return place && place.register ? place.register(Music, SPECS) : 0;
    }

    install(root.RunenpfadMusic);

    root.RunenpfadStoryMusic = { SPECS: SPECS, THEME_IDS: IDS, install: install };
})(typeof window !== 'undefined' ? window : globalThis);
