(function () {
    'use strict';
    const L = DesktopSoundSynth.layer;
    const events = {
        'window.open': [
            L('tone', { midi: 76, duration: 0.12, attack: 0.002, decay: 0.04, sustain: 0, release: 0.08, gain: 0.22, type: 'sine', filter: { type: 'lowpass', freq: 1800, q: 0.8 }, reverb: 'long' }),
            L('tone', { at: 0.07, midi: 79, duration: 0.14, attack: 0.002, decay: 0.05, sustain: 0, release: 0.1, gain: 0.2, type: 'sine', filter: { type: 'lowpass', freq: 1600, q: 0.7 }, reverb: 'long' }),
            L('tone', { at: 0.14, midi: 83, duration: 0.16, attack: 0.003, decay: 0.06, sustain: 0, release: 0.12, gain: 0.18, type: 'triangle', filter: { type: 'lowpass', freq: 1400, q: 0.6 }, reverb: 'long' }),
            L('noise', { duration: 0.2, gain: 0.05, color: 'pink', filter: { type: 'lowpass', freq: 900, q: 0.5 }, reverb: 'long' })
        ],
        'window.close': [
            L('pluck', { midi: 67, duration: 0.35, gain: 0.28, decay: 2.5, filterFreq: 700, reverb: 'long' }),
            L('noise', { duration: 0.25, gain: 0.06, color: 'pink', filter: { type: 'lowpass', freq: 600, q: 0.4 }, reverb: 'long' })
        ],
        'window.minimize': [
            L('tone', { midi: 72, duration: 0.1, attack: 0.002, decay: 0.03, sustain: 0, release: 0.07, gain: 0.2, type: 'sine', filter: { type: 'lowpass', freq: 1200, q: 0.6 }, reverb: 'plate' }),
            L('pluck', { at: 0.04, midi: 69, duration: 0.12, gain: 0.16, decay: 2.2, filterFreq: 800, reverb: 'short' })
        ],
        'window.restore': [
            L('pluck', { midi: 64, duration: 0.2, gain: 0.24, decay: 2.8, filterFreq: 850, reverb: 'plate' }),
            L('tone', { at: 0.06, midi: 67, duration: 0.12, gain: 0.16, type: 'sine', filter: { type: 'lowpass', freq: 1000, q: 0.5 }, reverb: 'short' })
        ],
        'window.maximize': [
            L('arp', { notes: [67, 71, 74, 79], step: 0.06, noteDur: 0.14, gain: 0.2, type: 'sine', filter: { type: 'lowpass', freq: 1500, q: 0.6 }, reverb: 'long' }),
            L('noise', { at: 0.1, duration: 0.2, gain: 0.05, color: 'pink', filter: { type: 'lowpass', freq: 800, q: 0.4 }, reverb: 'long' })
        ],
        'window.snap': [
            L('pluck', { midi: 74, duration: 0.1, gain: 0.26, decay: 3, filterFreq: 1100, reverb: 'short' }),
            L('tone', { at: 0.03, midi: 77, duration: 0.08, gain: 0.14, type: 'sine', filter: { type: 'lowpass', freq: 1400, q: 0.7 }, reverb: 'short' })
        ],
        'window.deny': [
            L('tone', { midi: 55, duration: 0.2, attack: 0.003, decay: 0.06, sustain: 0, release: 0.14, gain: 0.28, type: 'sine', filter: { type: 'lowpass', freq: 500, q: 0.8 }, reverb: 'short' }),
            L('tone', { at: 0.08, midi: 52, duration: 0.18, gain: 0.24, type: 'triangle', filter: { type: 'lowpass', freq: 450, q: 0.7 }, reverb: 'short' })
        ],
        'notify.info': [
            L('arp', { notes: [74, 77, 81], step: 0.065, noteDur: 0.15, gain: 0.2, type: 'sine', filter: { type: 'lowpass', freq: 1600, q: 0.6 }, reverb: 'long' }),
            L('noise', { duration: 0.15, gain: 0.04, color: 'pink', filter: { type: 'lowpass', freq: 700, q: 0.5 }, reverb: 'long' })
        ],
        'notify.message': [
            L('pluck', { midi: 72, duration: 0.28, gain: 0.26, decay: 2.6, filterFreq: 950, reverb: 'long' }),
            L('tone', { at: 0.08, midi: 76, duration: 0.12, gain: 0.14, type: 'sine', reverb: 'plate' })
        ],
        'notify.error': [
            L('tone', { midi: 48, duration: 0.25, attack: 0.004, decay: 0.08, sustain: 0, release: 0.18, gain: 0.3, type: 'sine', filter: { type: 'lowpass', freq: 400, q: 0.9 }, reverb: 'short' }),
            L('tone', { at: 0.06, midi: 45, duration: 0.2, gain: 0.26, type: 'triangle', filter: { type: 'lowpass', freq: 380, q: 0.8 }, reverb: 'short' }),
            L('noise', { at: 0.04, duration: 0.15, gain: 0.06, color: 'pink', filter: { type: 'lowpass', freq: 350, q: 0.6 }, reverb: 'short' })
        ],
        'menu.open': [
            L('pluck', { midi: 79, duration: 0.08, gain: 0.18, decay: 3.5, filterFreq: 1400, reverb: 'short' }),
            L('tone', { at: 0.02, midi: 83, duration: 0.06, gain: 0.12, type: 'sine', reverb: 'short' })
        ],
        'menu.close': [
            L('pluck', { midi: 76, duration: 0.06, gain: 0.14, decay: 4, filterFreq: 1200, reverb: 'short' })
        ],
        'space.switch': [
            L('noise', { duration: 0.4, gain: 0.08, color: 'pink', filter: { type: 'lowpass', freq: 700, q: 0.4 }, sweep: { from: 500, to: 1200, time: 0.35 }, reverb: 'long' }),
            L('tone', { at: 0.15, midi: 60, duration: 0.3, attack: 0.02, decay: 0.15, sustain: 0.08, release: 0.2, gain: 0.18, type: 'sine', filter: { type: 'lowpass', freq: 900, q: 0.5 }, reverb: 'long' })
        ],
        'dialog.open': [
            L('pluck', { midi: 65, duration: 0.18, gain: 0.22, decay: 2.8, filterFreq: 800, reverb: 'plate' }),
            L('tone', { at: 0.05, midi: 69, duration: 0.1, gain: 0.14, type: 'sine', reverb: 'short' })
        ],
        'dialog.confirm': [
            L('arp', { notes: [65, 69, 72], step: 0.055, noteDur: 0.12, gain: 0.18, type: 'sine', filter: { type: 'lowpass', freq: 1200, q: 0.6 }, reverb: 'plate' })
        ],
        'dialog.cancel': [
            L('pluck', { midi: 58, duration: 0.14, gain: 0.2, decay: 3.2, filterFreq: 650, reverb: 'short' })
        ],
        'file.trash': [
            L('noise', { duration: 0.14, gain: 0.1, color: 'pink', filter: { type: 'bandpass', freq: 700, q: 0.8 }, reverb: 'short' }),
            L('pluck', { at: 0.04, midi: 60, duration: 0.1, gain: 0.16, decay: 3, filterFreq: 700, reverb: 'short' })
        ],
        'file.delete': [
            L('tone', { midi: 50, duration: 0.22, gain: 0.26, type: 'sine', filter: { type: 'lowpass', freq: 450, q: 0.9 }, reverb: 'short' }),
            L('noise', { at: 0.05, duration: 0.12, gain: 0.08, color: 'pink', filter: { type: 'lowpass', freq: 400, q: 0.7 }, reverb: 'short' })
        ],
        'file.drop': [
            L('pluck', { midi: 62, duration: 0.25, gain: 0.28, decay: 2.4, filterFreq: 750, reverb: 'long' }),
            L('tone', { at: 0.05, midi: 65, duration: 0.1, attack: 0.002, decay: 0.04, sustain: 0, release: 0.08, gain: 0.14, type: 'sine', filter: { type: 'lowpass', freq: 1000, q: 0.6 }, reverb: 'plate' })
        ]
    };
    events._default = events['notify.info'];
    window.DesktopSoundThemes = window.DesktopSoundThemes || {};
    window.DesktopSoundThemes.water = { id: 'water', events };
})();
