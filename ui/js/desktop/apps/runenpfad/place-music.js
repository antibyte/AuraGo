/* Runenpfad place music: rich village hub & interior room scores, plus named voices and tune helpers.
   DOM-free and deterministic; delegates voice synthesis to RunenpfadMusicVoices when present. */
(function (root) {
    'use strict';

    const SCALES = {
        ionian: [0, 2, 4, 5, 7, 9, 11], mixolydian: [0, 2, 4, 5, 7, 9, 10], locrian: [0, 1, 3, 5, 6, 8, 10]
    };
    const NO_DRUMS = { kick: [], snare: [], hat: [] };

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

    function chordOf(th, bar) {
        return th.prog[Math.floor(bar / 4) % 2][bar % 4];
    }

    function figure(th, d, pat, base, type, vel, lo, hi) {
        return pat.map(function (n) { return note(n[0], n[1], fold(deg(th, d + base + n[2]), lo, hi), vel, type); });
    }

    // ---- village: G major warm pastoral folk dance (lute + harp + flute + cello + framedrum/tamb) ----
    const VILLAGE_LEAD = [
        [[0, 0.5, 0], [0.5, 0.5, 2], [1, 1, 4], [2, 0.5, 2], [2.5, 0.5, 1], [3, 1, 0]],
        [[0, 0.5, 4], [0.5, 0.5, 2], [1, 0.5, 0], [1.5, 0.5, 2], [2, 1, 4], [3, 1, 2]],
        [[0, 0.5, 0], [0.5, 0.5, 2], [1, 0.5, 4], [1.5, 0.5, 5], [2, 1, 4], [3, 0.5, 2], [3.5, 0.5, 1]]
    ];
    const VILLAGE_END = [[0, 1, 2], [1, 0.5, 1], [1.5, 0.5, 0], [2, 2, 0]];
    const VILLAGE_HALF = [[0, 0.5, 4], [0.5, 0.5, 2], [1, 1, 0], [2, 0.5, 2], [2.5, 0.5, 4], [3, 1, 6]];
    const VILLAGE_FLUTE = [[0.5, 0.75, 4], [1.5, 0.5, 5], [2.5, 0.75, 4], [3.25, 0.7, 7]];

    function composeVillage(th, layer, bar) {
        const d = chordOf(th, bar);
        const ph = bar % 4;
        const sec = Math.floor(bar / 4) % 2;
        const rest = ph === 3 ? (d === 0 ? VILLAGE_END : VILLAGE_HALF) : VILLAGE_LEAD[ph % 3];
        const out = [];
        if (layer === 'melody') return figure(th, d, rest, 7, sec ? 'fiddle' : 'lute', 0.9, th.root + 7, th.root + 24);
        if (layer === 'pad') return figure(th, d, VILLAGE_FLUTE, 14, 'flute', 0.85, th.root + 17, th.root + 31);
        if (layer === 'arp') {
            [0, 1, 2, 3].forEach(function (t, i) {
                [0, 2, 4].forEach(function (s, k) {
                    out.push(note(t + k * 0.05, 0.65, fold(deg(th, d + s), th.root - 2, th.root + 14), i % 2 ? 0.52 : 0.68, k === 2 ? 'harp' : 'lute'));
                });
            });
        } else if (layer === 'bass') {
            out.push(note(0, 1.4, fold(deg(th, d), th.root - 14, th.root - 3), 0.92, 'cello'));
            out.push(note(2, 1.4, fold(deg(th, d + 4), th.root - 14, th.root - 3), 0.82, 'cello'));
        } else if (layer === 'perc') {
            [0, 2.5].forEach(function (t, i) { out.push(note(t, 0.35, null, i ? 0.58 : 0.74, 'framedrum')); });
            [0.5, 1.5, 2.5, 3.5].forEach(function (t) { out.push(note(t, 0.09, null, 0.4, 'shaker')); });
            [1, 3].forEach(function (t) { out.push(note(t, 0.16, null, 0.58, 'tamb')); });
        }
        return out;
    }

    // ---- tavern: A major jovial hearth jig, keeping tight plucked voicings (pad <= 0.5, melody <= 1) ----
    const COZY_HOOK = [
        [[0, 0.5, 0], [0.5, 0.5, 2], [1, 0.5, 4], [1.5, 0.5, 7], [2, 0.5, 4], [2.5, 0.5, 2], [3, 0.5, 4], [3.5, 0.5, 2]],
        [[0, 0.5, 4], [0.5, 0.5, 2], [1, 0.5, 0], [1.5, 0.5, 2], [2, 0.5, 4], [2.5, 0.5, 7], [3, 0.9, 9]],
        [[0, 0.5, 0], [0.5, 0.5, 2], [1, 0.5, 4], [1.5, 0.5, 7], [2, 0.5, 4], [2.5, 0.5, 2], [3, 0.5, 4], [3.5, 0.5, 7]],
        [[0, 0.5, 4], [0.5, 0.5, 7], [1, 0.5, 4], [1.5, 0.5, 2], [2, 0.95, 0], [3.5, 0.5, 4]]
    ];
    const COZY_END = [[0, 0.5, 4], [0.5, 0.5, 7], [1, 0.5, 4], [1.5, 0.5, 2], [2, 0.95, 0]];

    function composeCozy(th, layer, bar) {
        const d = chordOf(th, bar);
        const sec = Math.floor(bar / 4) % 2;
        const out = [];
        if (layer === 'melody') return figure(th, d, bar % 8 === 7 ? COZY_END : COZY_HOOK[bar % 4], 7, sec ? 'fiddle' : 'lute', 0.92, th.root + 10, th.root + 27);
        if (layer === 'pad') {
            [0.5, 1.5, 2.5, 3.5].forEach(function (t) {
                [2, 4].forEach(function (s) { out.push(note(t, 0.25, fold(deg(th, d + s), th.root - 2, th.root + 12), 0.68, 'lute')); });
            });
        } else if (layer === 'arp') {
            [0, 1.5, 2, 3.5].forEach(function (t, i) {
                out.push(note(t, 0.45, fold(deg(th, d + [0, 4, 2, 7][i]), th.root + 10, th.root + 24), 0.55, 'harp'));
            });
        } else if (layer === 'bass') {
            [0, 1, 2, 3].forEach(function (t) { out.push(note(t, 0.5, fold(deg(th, d + (t % 2 ? 4 : 0)), th.root - 14, th.root - 3), 0.9, 'cello')); });
        } else if (layer === 'perc') {
            [0, 1, 2, 3].forEach(function (t) { out.push(note(t, 0.26, null, 0.72, 'tom')); });
            [1, 3].forEach(function (t) { out.push(note(t, 0.2, null, 0.54, 'framedrum')); });
            [0.5, 1.5, 2.5, 3.5].forEach(function (t) { out.push(note(t, 0.14, null, 0.75, 'tamb')); });
        }
        return out;
    }

    // ---- cave: E phrygian atmospheric cavern echoes (retains empty perc on bar 3, taiko on bar 5) ----
    function composeCave(th, layer, bar, seed) {
        const rng = rngFor(th, layer, bar, seed);
        const out = [];
        if (layer === 'pad') {
            out.push(note(0, 3.9, deg(th, 0) - 12, 0.92, 'growl'));
            if (bar % 4 === 3) out.push(note(1, 2.8, deg(th, 1) - 12, 0.62, 'choir'));
        } else if (layer === 'arp') {
            if (bar % 2 === 1) out.push(note(1.5, 1.4, deg(th, 4) + 12, 0.55, 'darkbell'));
        } else if (layer === 'melody') {
            const count = rng() < 0.25 ? 0 : (rng() < 0.65 ? 1 : 2);
            const slots = [0.5, 1.25, 2, 2.75, 3.25];
            const degs = [0, 1, 4, 7, 8];
            const pick = degs[Math.floor(rng() * degs.length)];
            for (let i = 0; i < count; i++) {
                const at = slots.splice(Math.floor(rng() * slots.length), 1)[0];
                out.push(note(at, 0.85, deg(th, pick + (i ? 1 : 0)) + 24, 0.64 + rng() * 0.28, 'celesta'));
            }
            out.sort(function (a, b) { return a.t - b.t; });
        } else if (layer === 'perc') {
            if (bar % 8 === 5) out.push(note(0, 0.6, null, 0.8, 'taiko'));
            if (bar % 8 === 2) out.push(note(2.5, 0.35, null, 0.55, 'tom'));
        }
        return out;
    }

    const SPECS = {
        village: { root: 55, scale: 'ionian', bpm: 104, prog: [[0, 3, 0, 4], [0, 3, 4, 0]], compose: composeVillage, perc: 0.65, vol: 1.35 },
        in_cozy: { root: 57, scale: 'ionian', bpm: 132, prog: [[0, 3, 0, 4], [0, 3, 4, 0]], compose: composeCozy, perc: 1.0, vol: 1.75 },
        in_cave: { root: 40, scale: 'phrygian', bpm: 54, prog: [[0, 0, 0, 0], [0, 0, 0, 0]], compose: composeCave, perc: 0.8, vol: 1.0 }
    };
    const IDS = Object.keys(SPECS);

    function register(Music, specs) {
        if (!Music || !Music.THEMES || !Music.SCALES) return 0;
        Object.keys(SCALES).forEach(function (name) { if (!Music.SCALES[name]) Music.SCALES[name] = SCALES[name]; });
        let made = 0;
        Object.keys(specs).forEach(function (id) {
            const s = specs[id];
            if (!Music.SCALES[s.scale]) return;
            Music.THEMES[id] = {
                id: id, biome: 'woodland', boss: false, root: s.root, scaleName: s.scale, scale: Music.SCALES[s.scale], bpm: s.bpm,
                prog: s.prog, arpInstr: s.arpInstr || 'lute', melInstr: s.melInstr || 'lute', arpGrid: s.arpGrid || [], drums: s.drums || NO_DRUMS,
                perc: s.perc || 0, vol: s.vol || 1, noFill: !!s.noFill, compose: s.compose || null
            };
            made++;
        });
        return made;
    }

    function namedVoice(name) {
        return function (h, f, t, dur, vel) {
            const Voices = root.RunenpfadMusicVoices;
            if (Voices && typeof Voices.buildVoice === 'function') {
                return Voices.buildVoice({
                    ctx: h.ctx,
                    keep: h.keep,
                    dsp: root.RunenpfadAudioDsp,
                    filt: h.filt,
                    noiseSrc: h.noiseSrc
                }, { type: name, vel: vel }, t, f, dur, h.g, 0);
            }
            const end = t + Math.max(0.08, dur);
            const isNoise = name === 'clock' || name === 'tamb';
            const src = h.keep(isNoise ? h.noiseSrc(t, 0.25) : h.osc('triangle', f || 220, t, 0));
            src.connect(h.g);
            h.envelope(h.g, t, Math.max(0.05, 0.16 * (vel || 0.7)), 0.01, end);
            src.stop(end + 0.02);
            return { end: end, main: src };
        };
    }

    const VOICES = {};
    ['lute', 'harp', 'flute', 'fiddle', 'plink', 'cello', 'brass', 'growl', 'metal', 'clock', 'tamb'].forEach(function (v) {
        VOICES[v] = namedVoice(v);
    });

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
        SPECS: SPECS, THEME_IDS: IDS, VOICES: VOICES, register: register,
        install: function (Music) { return register(Music, SPECS); }, themeFor: themeFor,
        tools: { rngFor: rngFor, deg: deg, fold: fold, note: note, chordOf: chordOf, figure: figure }
    };
})(typeof window !== 'undefined' ? window : globalThis);
