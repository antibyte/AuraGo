/* Runenpfad deeps music: the mine (a slower, sparser cave with wooden creaks and pick strikes as percussion) and the ember wyrm's
   lair (a dark boss loop with a drama layer). Registered through RunenpfadPlaceMusic.register; DOM-free and deterministic.
   Loads after story-music.js; does nothing when RunenpfadPlaceMusic or RunenpfadMusic is missing. */
(function (root) {
    'use strict';

    const PM = root.RunenpfadPlaceMusic;
    if (!PM || !PM.tools || typeof PM.register !== 'function' || !root.RunenpfadMusic) return;
    const T = PM.tools;
    const note = T.note;
    const deg = T.deg;
    const fold = T.fold;

    // ---- mine: D minor, 50 bpm. A low drone every other bar, one sparse plucked lantern note, a far bell, and a timber
    // percussion bed: pick strikes on stone ('clock'), a tick of grit, a long wooden creak ('swell') and a soft knock ('tom'). ----
    function composeMine(th, layer, bar, seed) {
        const rng = T.rngFor(th, layer, bar, seed);
        const out = [];
        if (layer === 'pad') {
            if (bar % 2 === 0) out.push(note(0, 3.9, deg(th, 0) - 12, 0.78, 'growl'));
            if (bar % 8 === 4) out.push(note(1, 2.8, deg(th, 4) - 12, 0.5, 'choir'));
        } else if (layer === 'bass') {
            if (bar % 4 === 0) out.push(note(0, 3.6, fold(deg(th, 0), th.root - 14, th.root - 3), 0.6, 'cello'));
        } else if (layer === 'arp') {
            if (bar % 4 === 2) out.push(note(1.5, 1.4, deg(th, 4) + 12, 0.5, 'darkbell'));
            if (bar % 8 === 5) out.push(note(0.5, 0.9, deg(th, 2) + 24, 0.4, 'glass'));
        } else if (layer === 'melody') {
            const count = rng() < 0.4 ? 0 : 1;
            const slots = [0.5, 1.25, 2, 2.75];
            const degs = [0, 2, 4, 6, 7];
            const pick = degs[Math.floor(rng() * degs.length)];
            if (count) out.push(note(slots[Math.floor(rng() * slots.length)], 0.9, deg(th, pick) + 12, 0.5 + rng() * 0.22, 'lute'));
        } else if (layer === 'perc') {
            if (bar % 2 === 0) {
                out.push(note(1, 0.06, null, 0.52, 'clock'));
                out.push(note(2.5, 0.06, null, 0.36, 'clock'));
            }
            if (bar % 4 === 1) out.push(note(3.25, 0.05, null, 0.34, 'tick'));
            if (bar % 4 === 3) out.push(note(0.5, 1.6, null, 0.4, 'swell'));
            if (bar % 8 === 6) out.push(note(0, 0.3, null, 0.5, 'tom'));
        } else if (layer === 'drama') {
            [0, 0, 1, 0, 0, 4, 0, 1].forEach(function (d, i) {
                out.push(note(i * 0.5, 0.42, fold(deg(th, d), th.root - 12, th.root + 2), i % 2 === 0 ? 0.86 : 0.68, 'cello'));
            });
            [0, 2].forEach(function (t, i) { out.push(note(t, 0.4, null, i === 0 ? 0.85 : 0.65, 'tom')); });
            [1, 3].forEach(function (t) { out.push(note(t, 0.06, null, 0.55, 'clock')); });
        }
        return out;
    }

    // ---- lair: A phrygian, 84 bpm. A growling drone with a flat-two lean, a driving cello ostinato, tolling dark bells, a low
    // brass call in the second half of each phrase and a heavy taiko pulse; the drama layer is the fight (fast cello, taiko every beat). ----
    const LAIR_OSTINATO = [0, 0, 1, 0, 0, 4, 0, 1];
    const LAIR_CALL = [[[0, 1.5, 0], [1.5, 0.5, 1], [2, 1, 0], [3, 1, -2]], [[0, 1, 4], [1, 0.5, 3], [1.5, 0.5, 1], [2, 2, 0]]];

    function composeLair(th, layer, bar, seed) {
        const rng = T.rngFor(th, layer, bar, seed);
        const d = T.chordOf(th, bar);
        const out = [];
        if (layer === 'pad') {
            out.push(note(0, 3.9, deg(th, d) - 12, 0.9, 'growl'));
            if (bar % 4 === 2) out.push(note(0, 3.8, deg(th, d) + 12, 0.62, 'choir'));
            if (bar % 4 === 3) out.push(note(1, 2.8, deg(th, d + 1) + 12, 0.5, 'metal'));
        } else if (layer === 'bass') {
            LAIR_OSTINATO.forEach(function (o, i) {
                out.push(note(i * 0.5, 0.45, fold(deg(th, d + o), th.root - 14, th.root - 2), i % 2 === 0 ? 0.84 : 0.66, 'cello'));
            });
        } else if (layer === 'arp') {
            if (bar % 2 === 0) out.push(note(0, 1.6, fold(deg(th, d), th.root + 10, th.root + 22), 0.62, 'darkbell'));
            if (bar % 4 === 3) out.push(note(2.5, 0.9, fold(deg(th, d + 1), th.root + 22, th.root + 34), 0.46, 'glass'));
        } else if (layer === 'melody') {
            if (bar % 4 >= 2) return T.figure(th, d, LAIR_CALL[bar % 2], 0, 'brass', 0.8, th.root + 2, th.root + 19);
            if (rng() < 0.25) out.push(note(3, 0.9, deg(th, 1) + 12, 0.5, 'darkbell'));
        } else if (layer === 'perc') {
            [0, 2.5].forEach(function (t, i) { out.push(note(t, 0.5, null, i === 0 ? 0.92 : 0.78, 'taiko')); });
            [1.5, 3.25].forEach(function (t) { out.push(note(t, 0.25, null, 0.66, 'tom')); });
            [1, 3].forEach(function (t) { out.push(note(t, 0.06, null, 0.5, 'clock')); });
        } else if (layer === 'drama') {
            [0, 1, 0, 4, 0, 1, 5, 4].forEach(function (n, i) {
                out.push(note(i * 0.5, 0.4, fold(deg(th, n), th.root - 12, th.root + 2), i % 2 === 0 ? 0.92 : 0.72, 'cello'));
            });
            [0, 1, 2, 3].forEach(function (t, i) { out.push(note(t, 0.5, null, i === 0 ? 0.95 : 0.78, i % 2 ? 'tom' : 'taiko')); });
            [0.5, 1.5, 2.5, 3.5].forEach(function (t) { out.push(note(t, 0.06, null, 0.45, 'clock')); });
            if (bar % 4 === 3) [2.5, 2.75, 3, 3.25, 3.5, 3.75].forEach(function (t, i) { out.push(note(t, 0.2, null, 0.6 + i * 0.06, 'tom')); });
        }
        return out;
    }

    const SPECS = {
        in_mine: { root: 38, scale: 'minor', bpm: 50, prog: [[0, 0, 0, 0], [0, 0, 5, 0]], compose: composeMine, perc: 0.8, vol: 1.0 },
        in_lair: { root: 33, scale: 'phrygian', bpm: 84, prog: [[0, 1, 0, 5], [0, 1, 6, 0]], compose: composeLair, perc: 0.9, vol: 1.0 }
    };
    const IDS = Object.keys(SPECS);

    function install(Music) { return PM.register(Music, SPECS); }
    install(root.RunenpfadMusic);

    root.RunenpfadDeepsMusic = { SPECS: SPECS, THEME_IDS: IDS, install: install };
})(typeof window !== 'undefined' ? window : globalThis);
