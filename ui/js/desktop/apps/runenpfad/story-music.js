/* Runenpfad story music: three cutscene loops (dawn, dark, triumph). DOM-free; played through music.setOverride(). */
(function (root) {
    'use strict';

    const EIGHTHS = [0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5];
    const QUARTERS = [0, 1, 2, 3];

    // Instruments come from music.js (bell, darkbell, horn, pluck, string, tick) and place-music.js (harp).
    const SPECS = {
        // Dawn: G major, slow, harp arpeggios under a bell melody, no drums.
        story_dawn: { root: 55, scale: 'ionian', bpm: 66, prog: [[0, 4, 5, 3], [0, 3, 4, 0]], arpInstr: 'harp', melInstr: 'bell', arpGrid: EIGHTHS, drums: { kick: [], snare: [], hat: [] }, vol: 0.9, noFill: true },
        // Dark: A phrygian (no major third), very slow, a low drone and a ticking clock on every beat.
        story_dark: { root: 45, scale: 'phrygian', bpm: 56, prog: [[0, 0, 1, 0], [0, 5, 1, 0]], arpInstr: 'tick', melInstr: 'darkbell', arpGrid: [], drums: { kick: [], snare: [], hat: [], tick: QUARTERS }, perc: 0.85, vol: 0.9, noFill: true },
        // Triumph: F major, broad horn lead over a steady march (kick on 1 and 3, snare on 2 and 4).
        story_triumph: { root: 53, scale: 'ionian', bpm: 108, prog: [[0, 3, 4, 0], [0, 5, 3, 4]], arpInstr: 'string', melInstr: 'horn', arpGrid: QUARTERS, drums: { kick: [0, 2], snare: [1, 3], hat: [], taiko: [0] }, perc: 0.7, vol: 1 }
    };
    const IDS = Object.keys(SPECS);

    // Registers the story themes into RunenpfadMusic.THEMES (place-music.js adds the major modes they use).
    function install(Music) {
        const place = root.RunenpfadPlaceMusic;
        return place && place.register ? place.register(Music, SPECS) : 0;
    }

    install(root.RunenpfadMusic);

    root.RunenpfadStoryMusic = { SPECS: SPECS, THEME_IDS: IDS, install: install };
})(typeof window !== 'undefined' ? window : globalThis);
