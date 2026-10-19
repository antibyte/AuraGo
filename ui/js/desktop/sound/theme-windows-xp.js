(function () {
    'use strict';
    const S = window.DesktopSoundSynth;
    window.DesktopSoundThemes.windows_xp = {
        id: 'windows_xp',
        // Fresh friendly chimes with a bright upper partial and quick, rounded tails.
        events: S.composeDesktopEvents({ voice: 'chime', root: 65, rise: 2, fall: -3, color: 2,
            step: 0.064, gain: 0.22, tail: 0.13, filter: 4100 })
    };
})();
