(function () {
    'use strict';
    const L = DesktopSoundSynth.layer;
    const events = {
        'window.open': [
            L('pluck', { midi: 67, duration: 0.32, gain: 0.38, decay: 3.2, filterFreq: 1400, reverb: 'plate' }),
            L('noise', { at: 0.01, duration: 0.025, gain: 0.08, filter: { type: 'bandpass', freq: 2800, q: 3 }, reverb: 'short' }),
            L('pluck', { at: 0.08, midi: 72, duration: 0.28, gain: 0.28, decay: 2.8, filterFreq: 1600, reverb: 'plate' })
        ],
        'window.close': [
            L('noise', { duration: 0.08, gain: 0.14, filter: { type: 'lowpass', freq: 420, q: 0.9 }, reverb: 'short' }),
            L('pluck', { at: 0.02, midi: 55, duration: 0.2, gain: 0.32, decay: 4, filterFreq: 600, reverb: 'short' })
        ],
        'window.minimize': [
            L('pluck', { midi: 60, duration: 0.18, gain: 0.3, decay: 3.5, filterFreq: 800, reverb: 'short' }),
            L('noise', { duration: 0.04, gain: 0.06, filter: { type: 'bandpass', freq: 1800, q: 2.5 }, reverb: 'short' })
        ],
        'window.restore': [
            L('pluck', { midi: 58, duration: 0.22, gain: 0.28, decay: 3, filterFreq: 1000, reverb: 'plate' }),
            L('pluck', { at: 0.06, midi: 64, duration: 0.2, gain: 0.24, decay: 2.6, filterFreq: 1200, reverb: 'short' })
        ],
        'window.maximize': [
            L('pluck', { midi: 64, duration: 0.26, gain: 0.34, decay: 2.8, filterFreq: 1300, reverb: 'plate' }),
            L('arp', { at: 0.05, notes: [64, 67, 71], step: 0.05, noteDur: 0.12, gain: 0.22, reverb: 'plate' })
        ],
        'window.snap': [
            L('noise', { duration: 0.05, gain: 0.12, filter: { type: 'bandpass', freq: 2200, q: 2.8 }, reverb: 'short' }),
            L('pluck', { at: 0.015, midi: 69, duration: 0.1, gain: 0.26, decay: 4, filterFreq: 1800, reverb: 'short' })
        ],
        'window.deny': [
            L('pluck', { midi: 48, duration: 0.16, gain: 0.34, decay: 5, filterFreq: 500, reverb: 'short' }),
            L('noise', { at: 0.04, duration: 0.06, gain: 0.1, filter: { type: 'lowpass', freq: 600, q: 1 }, reverb: 'short' })
        ],
        'notify.info': [
            L('arp', { notes: [67, 71, 74, 79], step: 0.055, noteDur: 0.14, gain: 0.26, reverb: 'plate' }),
            L('noise', { at: 0.02, duration: 0.03, gain: 0.05, filter: { type: 'bandpass', freq: 3200, q: 2 }, reverb: 'short' })
        ],
        'notify.message': [
            L('pluck', { midi: 69, duration: 0.24, gain: 0.3, decay: 3, filterFreq: 1100, reverb: 'plate' }),
            L('pluck', { at: 0.07, midi: 72, duration: 0.2, gain: 0.22, decay: 2.8, filterFreq: 1300, reverb: 'short' })
        ],
        'notify.error': [
            L('pluck', { midi: 45, duration: 0.22, gain: 0.36, decay: 4.5, filterFreq: 450, reverb: 'short' }),
            L('noise', { at: 0.05, duration: 0.1, gain: 0.12, filter: { type: 'lowpass', freq: 700, q: 1.2 }, reverb: 'short' }),
            L('pluck', { at: 0.1, midi: 42, duration: 0.18, gain: 0.28, decay: 5, filterFreq: 380, reverb: 'short' })
        ],
        'menu.open': [
            L('noise', { duration: 0.035, gain: 0.1, filter: { type: 'bandpass', freq: 2600, q: 3.5 }, reverb: 'short' }),
            L('pluck', { at: 0.01, midi: 76, duration: 0.08, gain: 0.2, decay: 4, filterFreq: 2000, reverb: 'short' })
        ],
        'menu.close': [
            L('noise', { duration: 0.03, gain: 0.08, filter: { type: 'bandpass', freq: 2000, q: 2.5 }, reverb: 'short' })
        ],
        'space.switch': [
            L('noise', { duration: 0.28, gain: 0.08, color: 'pink', filter: { type: 'lowpass', freq: 900, q: 0.5 }, sweep: { from: 600, to: 1400, time: 0.25 }, reverb: 'plate' }),
            L('pluck', { at: 0.12, midi: 62, duration: 0.25, gain: 0.22, decay: 3, filterFreq: 800, reverb: 'plate' })
        ],
        'dialog.open': [
            L('pluck', { midi: 65, duration: 0.18, gain: 0.28, decay: 3, filterFreq: 1000, reverb: 'plate' }),
            L('noise', { at: 0.02, duration: 0.025, gain: 0.06, filter: { type: 'bandpass', freq: 2400, q: 2 }, reverb: 'short' })
        ],
        'dialog.confirm': [
            L('arp', { notes: [65, 69, 72], step: 0.045, noteDur: 0.11, gain: 0.24, reverb: 'plate' })
        ],
        'dialog.cancel': [
            L('pluck', { midi: 58, duration: 0.14, gain: 0.26, decay: 4, filterFreq: 650, reverb: 'short' })
        ],
        'file.trash': [
            L('noise', { duration: 0.16, gain: 0.14, color: 'pink', filter: { type: 'bandpass', freq: 900, q: 1.1 }, reverb: 'short' }),
            L('noise', { at: 0.05, duration: 0.08, gain: 0.08, filter: { type: 'highpass', freq: 1800, q: 1.5 }, reverb: 'short' })
        ],
        'file.delete': [
            L('noise', { duration: 0.12, gain: 0.16, filter: { type: 'lowpass', freq: 550, q: 0.8 }, reverb: 'short' }),
            L('pluck', { at: 0.04, midi: 50, duration: 0.15, gain: 0.28, decay: 4.5, filterFreq: 500, reverb: 'short' })
        ],
        'file.drop': [
            L('pluck', { midi: 62, duration: 0.22, gain: 0.3, decay: 3.2, filterFreq: 950, reverb: 'plate' }),
            L('noise', { at: 0.03, duration: 0.05, gain: 0.07, filter: { type: 'bandpass', freq: 1500, q: 1.8 }, reverb: 'short' })
        ]
    };
    events._default = events['notify.info'];
    window.DesktopSoundThemes = window.DesktopSoundThemes || {};
    window.DesktopSoundThemes.wood = { id: 'wood', events };
})();
