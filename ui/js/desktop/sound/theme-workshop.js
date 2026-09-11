(function () {
    'use strict';
    const L = DesktopSoundSynth.layer;
    const events = {
        'window.open': [
            L('fm', { midi: 58, modRatio: 4.5, index: 55, indexDecay: 0.15, duration: 0.28, gain: 0.28, carrierType: 'sawtooth', filter: { type: 'bandpass', freq: 900, q: 1.2 }, reverb: 'short' }),
            L('noise', { at: 0.02, duration: 0.22, gain: 0.1, filter: { type: 'bandpass', freq: 1400, q: 0.8 }, sweep: { from: 800, to: 2400, time: 0.2 }, reverb: 'short' }),
            L('tone', { at: 0.12, midi: 72, duration: 0.08, attack: 0.001, decay: 0.02, sustain: 0, release: 0.05, gain: 0.22, type: 'square', filter: { type: 'highpass', freq: 1800, q: 2 }, reverb: 'short' })
        ],
        'window.close': [
            L('noise', { duration: 0.18, gain: 0.12, filter: { type: 'highpass', freq: 3200, q: 1.5 }, decayPower: 2.5, reverb: 'short' }),
            L('tone', { at: 0.04, midi: 64, duration: 0.1, gain: 0.2, type: 'square', filter: { type: 'bandpass', freq: 2200, q: 2.5 }, reverb: 'short' })
        ],
        'window.minimize': [
            L('fm', { midi: 52, modRatio: 3, index: 40, duration: 0.14, gain: 0.24, reverb: 'short' }),
            L('noise', { duration: 0.06, gain: 0.08, filter: { type: 'bandpass', freq: 1800, q: 2 }, reverb: 'short' })
        ],
        'window.restore': [
            L('noise', { duration: 0.1, gain: 0.1, sweep: { from: 600, to: 1800, time: 0.08 }, filter: { type: 'bandpass', freq: 1200, q: 1.2 }, reverb: 'short' }),
            L('tone', { at: 0.05, midi: 67, duration: 0.08, gain: 0.2, type: 'square', reverb: 'short' })
        ],
        'window.maximize': [
            L('fm', { midi: 55, modRatio: 5, index: 60, indexDecay: 0.18, duration: 0.24, gain: 0.26, reverb: 'short' }),
            L('tone', { at: 0.1, midi: 79, duration: 0.12, gain: 0.22, type: 'square', filter: { type: 'bandpass', freq: 2800, q: 2 }, reverb: 'short' })
        ],
        'window.snap': [
            L('tone', { midi: 84, duration: 0.05, gain: 0.32, type: 'square', filter: { type: 'bandpass', freq: 3200, q: 3 }, reverb: 'short' }),
            L('noise', { at: 0.01, duration: 0.04, gain: 0.1, filter: { type: 'highpass', freq: 4000, q: 1.5 }, reverb: 'short' })
        ],
        'window.deny': [
            L('tone', { midi: 50, duration: 0.08, gain: 0.34, type: 'square', filter: { type: 'bandpass', freq: 900, q: 2.5 }, reverb: 'short' }),
            L('tone', { at: 0.06, midi: 50, duration: 0.08, gain: 0.3, type: 'square', filter: { type: 'bandpass', freq: 850, q: 2.5 }, reverb: 'short' }),
            L('noise', { at: 0.03, duration: 0.06, gain: 0.08, filter: { type: 'lowpass', freq: 1200, q: 1 }, reverb: 'short' })
        ],
        'notify.info': [
            L('tone', { midi: 79, duration: 0.35, attack: 0.002, decay: 0.1, sustain: 0.05, release: 0.25, gain: 0.28, type: 'triangle', partials: [{ ratio: 1, gain: 1 }, { ratio: 2.8, gain: 0.35, type: 'sine' }, { ratio: 4.2, gain: 0.15, type: 'sine' }], reverb: 'short' }),
            L('noise', { at: 0.02, duration: 0.05, gain: 0.06, filter: { type: 'highpass', freq: 5000, q: 1 }, reverb: 'short' })
        ],
        'notify.message': [
            L('fm', { midi: 67, modRatio: 3.2, index: 42, duration: 0.22, gain: 0.24, reverb: 'short' }),
            L('tone', { at: 0.08, midi: 74, duration: 0.15, gain: 0.18, type: 'square', filter: { type: 'bandpass', freq: 2400, q: 2 }, reverb: 'short' })
        ],
        'notify.error': [
            L('tone', { midi: 45, duration: 0.1, gain: 0.36, type: 'square', filter: { type: 'bandpass', freq: 700, q: 2.5 }, reverb: 'short' }),
            L('tone', { at: 0.07, midi: 45, duration: 0.1, gain: 0.32, type: 'square', filter: { type: 'bandpass', freq: 650, q: 2.5 }, reverb: 'short' }),
            L('noise', { at: 0.04, duration: 0.12, gain: 0.12, filter: { type: 'lowpass', freq: 800, q: 1.2 }, reverb: 'short' })
        ],
        'menu.open': [
            L('tone', { midi: 90, duration: 0.04, gain: 0.24, type: 'square', filter: { type: 'highpass', freq: 2500, q: 2 }, reverb: 'short' }),
            L('noise', { duration: 0.03, gain: 0.06, filter: { type: 'bandpass', freq: 3500, q: 2.5 }, reverb: 'short' })
        ],
        'menu.close': [
            L('tone', { midi: 78, duration: 0.035, gain: 0.18, type: 'square', reverb: 'short' })
        ],
        'space.switch': [
            L('fm', { midi: 48, modRatio: 6, index: 70, indexDecay: 0.25, duration: 0.35, gain: 0.22, reverb: 'short' }),
            L('noise', { duration: 0.3, gain: 0.1, sweep: { from: 400, to: 2200, time: 0.28 }, filter: { type: 'bandpass', freq: 1000, q: 0.9 }, reverb: 'short' })
        ],
        'dialog.open': [
            L('tone', { midi: 62, duration: 0.12, gain: 0.22, type: 'square', filter: { type: 'bandpass', freq: 1600, q: 1.8 }, reverb: 'short' }),
            L('fm', { at: 0.04, midi: 65, modRatio: 2.5, index: 30, duration: 0.1, gain: 0.16, reverb: 'short' })
        ],
        'dialog.confirm': [
            L('tone', { midi: 67, duration: 0.1, gain: 0.24, type: 'square', filter: { type: 'bandpass', freq: 2000, q: 2 }, reverb: 'short' }),
            L('tone', { at: 0.05, midi: 72, duration: 0.08, gain: 0.2, type: 'triangle', reverb: 'short' })
        ],
        'dialog.cancel': [
            L('noise', { duration: 0.08, gain: 0.1, filter: { type: 'lowpass', freq: 900, q: 1 }, reverb: 'short' })
        ],
        'file.trash': [
            L('noise', { duration: 0.1, gain: 0.14, filter: { type: 'bandpass', freq: 1300, q: 1.5 }, reverb: 'short' }),
            L('tone', { at: 0.03, midi: 55, duration: 0.08, gain: 0.2, type: 'square', reverb: 'short' })
        ],
        'file.delete': [
            L('tone', { midi: 48, duration: 0.14, gain: 0.3, type: 'sawtooth', filter: { type: 'lowpass', freq: 600, q: 2 }, reverb: 'short' }),
            L('noise', { at: 0.05, duration: 0.1, gain: 0.12, filter: { type: 'bandpass', freq: 800, q: 1.2 }, reverb: 'short' })
        ],
        'file.drop': [
            L('fm', { midi: 60, modRatio: 3.8, index: 48, duration: 0.16, gain: 0.24, reverb: 'short' }),
            L('tone', { at: 0.04, midi: 65, duration: 0.08, gain: 0.18, type: 'square', reverb: 'short' })
        ]
    };
    events._default = events['notify.info'];
    window.DesktopSoundThemes = window.DesktopSoundThemes || {};
    window.DesktopSoundThemes.workshop = { id: 'workshop', events };
})();
