(function () {
    'use strict';
    const S = window.DesktopSoundSynth;
    window.DesktopSoundThemes.green_slime = {
        id: 'green_slime',
        events: S.composeDesktopEvents({ voice: 'slime', root: 53, rise: 5, fall: -7, color: 3,
            step: 0.082, gain: 0.24, tail: 0.15, filter: 980, resonance: 3.8, noiseCutoff: 680 })
    };
})();
