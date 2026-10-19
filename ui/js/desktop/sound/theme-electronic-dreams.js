(function () {
    'use strict';
    const S = window.DesktopSoundSynth;
    window.DesktopSoundThemes.electronic_dreams = {
        id: 'electronic_dreams',
        events: S.composeDesktopEvents({ voice: 'relay', root: 59, rise: 3, fall: -4, color: 2, interval: 5,
            step: 0.072, gain: 0.23, tail: 0.09, modRatio: 1.55, index: 21, filter: 1750, signal: 980 })
    };
})();
