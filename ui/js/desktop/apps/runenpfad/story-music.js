/* Runenpfad story music: serene dawn, dread dark, and victorious triumph cutscene suites. DOM-free. */
(function (root) {
    'use strict';

    const EIGHTHS = [0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5];
    const T = function () { return root.RunenpfadPlaceMusic.tools; };

    // ---- dawn: warm G major pastoral sunrise over flowing harp & singing wooden flute ----
    const DAWN_FLUTE = [
        [[0, 1.5, 4], [1.5, 0.5, 5], [2, 1.8, 7]],
        [[0, 1.5, 6], [1.5, 0.5, 4], [2, 1.8, 2]],
        [[0, 1.0, 4], [1.0, 1.0, 5], [2, 1.8, 7]],
        [[0, 2.0, 9], [2.2, 1.6, 7]]
    ];

    function composeDawn(th, layer, bar) {
        const t = T();
        const d = t.chordOf(th, bar);
        const ph = bar % 4;
        const sec = Math.floor(bar / 4) % 2;
        const out = [];
        if (layer === 'pad') {
            [0, 2, 4].forEach(function (s, i) {
                out.push(t.note(0, 3.8, t.fold(t.deg(th, d + s), th.root - 2, th.root + 14), i === 0 ? 0.72 : 0.58, 'string'));
            });
        } else if (layer === 'bass') {
            out.push(t.note(0, 3.6, t.fold(t.deg(th, d), th.root - 14, th.root - 3), 0.78, 'cello'));
        } else if (layer === 'arp') {
            EIGHTHS.forEach(function (bt, i) {
                out.push(t.note(bt, 0.9, t.fold(t.deg(th, d + [0, 2, 4, 7][i % 4]) + 12, th.root + 7, th.root + 24), i % 2 ? 0.62 : 0.76, 'harp'));
            });
        } else if (layer === 'melody') {
            return t.figure(th, d, DAWN_FLUTE[ph], 7, sec ? 'celesta' : 'flute', 0.84, th.root + 12, th.root + 29);
        }
        return out;
    }

    // ---- dark: A locrian cold dread (keeps tritone/b2 tension, choir drone, glass/darkbell & ticking clock) ----
    function composeDark(th, layer, bar, seed) {
        const t = T();
        const rng = t.rngFor(th, layer, bar, seed);
        const out = [];
        if (layer === 'pad') {
            out.push(t.note(0, 3.9, t.deg(th, 0), 0.92, 'growl'));
            if (bar % 8 === 5) out.push(t.note(2, 1.8, t.deg(th, 1), 0.58, 'growl'));
            if (bar % 4 === 2) out.push(t.note(0, 3.8, t.deg(th, 0) + 12, 0.68, 'choir'));
            if (bar % 4 === 3) out.push(t.note(1, 2.8, t.deg(th, 4) + 12, 0.54, 'metal'));
        } else if (layer === 'melody') {
            if (rng() < 0.6) out.push(t.note([0.5, 1.5, 2.5][Math.floor(rng() * 3)], 1.4, t.deg(th, [0, 1, 4, 8][Math.floor(rng() * 4)]) + 12, 0.86, 'darkbell'));
            if (rng() < 0.35) out.push(t.note(3, 0.9, t.deg(th, rng() < 0.5 ? 1 : 4) + 24, 0.56, 'glass'));
        } else if (layer === 'perc') {
            [0, 1, 2, 3].forEach(function (b) { out.push(t.note(b, 0.1, null, b % 2 ? 0.6 : 0.95, 'clock')); });
            if (bar % 4 === 0) out.push(t.note(0, 0.6, null, 0.65, 'taiko'));
        }
        return out;
    }

    // ---- triumph: F major heroic orchestral fanfare with horns, brass, harp flourishes & snare/taiko ----
    const FANFARE = [[0, 0.75, 0], [0.75, 0.25, 0], [1, 0.5, 3], [1.5, 0.5, 4]];
    const LANDINGS = [[[2, 1, 7], [3, 1, 4]], [[2, 1.5, 7], [3.5, 0.5, 9]], [[2, 0.5, 4], [2.5, 0.5, 7], [3, 1, 9]], [[2, 1, 7], [3, 0.5, 9], [3.5, 0.5, 7]]];
    const FINAL = [[2, 1.9, 7]];

    function composeTriumph(th, layer, bar) {
        const t = T();
        const d = t.chordOf(th, bar);
        const ph = bar % 4;
        const last = bar % 8 === 7;
        const out = [];
        if (layer === 'melody') return t.figure(th, d, FANFARE.concat(last ? FINAL : LANDINGS[ph]), 7, 'brass', 0.96, th.root + 10, th.root + 26);
        if (layer === 'pad') {
            [[0, 1.6], [2, last ? 1.9 : 1.1]].forEach(function (c) {
                [0, 2, 4].forEach(function (s) { out.push(t.note(c[0], c[1], t.fold(t.deg(th, d + s), th.root - 2, th.root + 12), 0.74, 'horn')); });
            });
        } else if (layer === 'arp') {
            [0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5].forEach(function (bt, i) {
                out.push(t.note(bt, 0.75, t.fold(t.deg(th, d + [0, 2, 4, 7][i % 4]) + 12, th.root + 10, th.root + 26), i % 2 ? 0.56 : 0.72, 'harp'));
            });
        } else if (layer === 'bass') {
            out.push(t.note(0, 1.4, t.fold(t.deg(th, d), th.root - 14, th.root - 3), 0.96, 'cello'));
            out.push(t.note(2, 1.4, t.fold(t.deg(th, d + 4), th.root - 14, th.root - 3), 0.88, 'bass'));
        } else if (layer === 'perc') {
            out.push(t.note(0, 0.6, null, 0.95, 'taiko'), t.note(2, 0.55, null, 0.86, 'taiko'));
            [1, 3].forEach(function (b) { out.push(t.note(b, 0.22, null, 0.76, 'snare')); });
            if (ph === 3) for (let i = 0; i < 8; i++) out.push(t.note(3 + i * 0.125, 0.1, null, 0.35 + i * 0.07, 'snare'));
        }
        return out;
    }

    const SPECS = {
        story_dawn: { root: 55, scale: 'ionian', bpm: 66, prog: [[0, 4, 5, 3], [0, 3, 4, 0]], compose: composeDawn, noFill: true, vol: 0.9 },
        story_dark: { root: 33, scale: 'locrian', bpm: 56, prog: [[0, 0, 0, 0], [0, 0, 0, 0]], compose: composeDark, perc: 1, vol: 1.0 },
        story_triumph: { root: 53, scale: 'ionian', bpm: 120, prog: [[0, 3, 4, 0], [0, 3, 4, 0]], compose: composeTriumph, perc: 1.2, vol: 1.3 }
    };
    const IDS = Object.keys(SPECS);

    function install(Music) {
        const place = root.RunenpfadPlaceMusic;
        return place && place.register ? place.register(Music, SPECS) : 0;
    }

    install(root.RunenpfadMusic);

    root.RunenpfadStoryMusic = { SPECS: SPECS, THEME_IDS: IDS, install: install };
})(typeof window !== 'undefined' ? window : globalThis);
