(function () {
    'use strict';
    const L = DesktopSoundSynth.layer;
    const events = {
        'window.open': [
            L('tone', { midi: 84, duration: 0.55, attack: 0.004, decay: 0.12, sustain: 0.08, release: 0.35, gain: 0.42, reverb: 'long', partials: [{ ratio: 1, gain: 1, type: 'sine' }, { ratio: 2.01, gain: 0.35, type: 'triangle' }, { ratio: 3.02, gain: 0.12, type: 'sine' }] }),
            L('tone', { at: 0.09, midi: 88, duration: 0.5, attack: 0.003, decay: 0.1, sustain: 0.05, release: 0.4, gain: 0.32, reverb: 'long', partials: [{ ratio: 1, gain: 1, type: 'triangle' }, { ratio: 2.4, gain: 0.2, type: 'sine' }] })
        ],
        'window.close': [
            L('tone', { midi: 79, duration: 0.45, glide: DesktopSoundSynth.midiToHz(67), glideTime: 0.28, attack: 0.002, decay: 0.08, sustain: 0, release: 0.35, gain: 0.38, reverb: 'long', type: 'sine', partials: [{ ratio: 1, gain: 1 }, { ratio: 1.5, gain: 0.25, type: 'triangle' }] }),
            L('noise', { at: 0.12, duration: 0.18, gain: 0.06, filter: { type: 'highpass', freq: 3200, q: 0.8 }, reverb: 'short' })
        ],
        'window.minimize': [
            L('tone', { midi: 76, duration: 0.22, attack: 0.002, decay: 0.06, sustain: 0, release: 0.14, gain: 0.28, reverb: 'plate', partials: [{ ratio: 1, gain: 1, type: 'triangle' }] }),
            L('tone', { at: 0.05, midi: 72, duration: 0.18, attack: 0.001, decay: 0.05, sustain: 0, release: 0.12, gain: 0.2, reverb: 'short', partials: [{ ratio: 1, gain: 1, type: 'sine' }] })
        ],
        'window.restore': [
            L('tone', { midi: 72, duration: 0.28, glide: DesktopSoundSynth.midiToHz(76), glideTime: 0.16, attack: 0.003, decay: 0.07, sustain: 0.04, release: 0.18, gain: 0.3, reverb: 'plate', type: 'sine' }),
            L('noise', { duration: 0.08, gain: 0.04, filter: { type: 'bandpass', freq: 5200, q: 2 }, reverb: 'short' })
        ],
        'window.maximize': [
            L('arp', { notes: [72, 76, 79], step: 0.045, noteDur: 0.14, gain: 0.26, type: 'triangle', reverb: 'long' }),
            L('tone', { at: 0.14, midi: 84, duration: 0.35, attack: 0.004, decay: 0.1, sustain: 0.06, release: 0.25, gain: 0.22, reverb: 'long', partials: [{ ratio: 1, gain: 1, type: 'sine' }, { ratio: 2, gain: 0.3, type: 'triangle' }] })
        ],
        'window.snap': [
            L('tone', { midi: 81, duration: 0.12, attack: 0.001, decay: 0.04, sustain: 0, release: 0.07, gain: 0.34, reverb: 'short', partials: [{ ratio: 1, gain: 1, type: 'triangle' }, { ratio: 3, gain: 0.15, type: 'sine' }] }),
            L('noise', { at: 0.02, duration: 0.06, gain: 0.05, filter: { type: 'highpass', freq: 4000, q: 1.2 }, reverb: 'short' })
        ],
        'window.deny': [
            L('tone', { midi: 68, duration: 0.2, attack: 0.001, decay: 0.05, sustain: 0, release: 0.1, gain: 0.32, reverb: 'short', partials: [{ ratio: 1, gain: 1, type: 'square' }, { ratio: 1.02, gain: 0.4, type: 'sine' }] }),
            L('tone', { at: 0.07, midi: 65, duration: 0.18, attack: 0.001, decay: 0.04, sustain: 0, release: 0.08, gain: 0.28, reverb: 'short', type: 'triangle' })
        ],
        'notify.info': [
            L('arp', { notes: [79, 83, 86], step: 0.055, noteDur: 0.16, gain: 0.24, type: 'sine', reverb: 'long' }),
            L('tone', { at: 0.18, midi: 88, duration: 0.4, attack: 0.005, decay: 0.12, sustain: 0.05, release: 0.3, gain: 0.18, reverb: 'long', partials: [{ ratio: 1, gain: 1, type: 'triangle' }] })
        ],
        'notify.message': [
            L('tone', { midi: 76, duration: 0.32, attack: 0.003, decay: 0.08, sustain: 0.06, release: 0.22, gain: 0.3, reverb: 'plate', partials: [{ ratio: 1, gain: 1, type: 'sine' }, { ratio: 2.5, gain: 0.18, type: 'triangle' }] }),
            L('fm', { at: 0.06, midi: 81, modRatio: 3.5, index: 28, indexDecay: 0.08, duration: 0.2, gain: 0.12, reverb: 'short' })
        ],
        'notify.error': [
            L('tone', { midi: 62, duration: 0.28, attack: 0.001, decay: 0.06, sustain: 0, release: 0.15, gain: 0.36, reverb: 'short', partials: [{ ratio: 1, gain: 1, type: 'square' }, { ratio: 1.5, gain: 0.35, type: 'sawtooth' }] }),
            L('tone', { at: 0.09, midi: 58, duration: 0.24, attack: 0.001, decay: 0.05, sustain: 0, release: 0.12, gain: 0.3, reverb: 'short', type: 'triangle' }),
            L('noise', { at: 0.04, duration: 0.12, gain: 0.08, filter: { type: 'bandpass', freq: 900, q: 1.5 }, reverb: 'short' })
        ],
        'menu.open': [
            L('tone', { midi: 88, duration: 0.1, attack: 0.001, decay: 0.03, sustain: 0, release: 0.06, gain: 0.22, reverb: 'short', type: 'triangle' }),
            L('noise', { duration: 0.05, gain: 0.035, filter: { type: 'highpass', freq: 6000, q: 0.7 }, reverb: 'short' })
        ],
        'menu.close': [
            L('tone', { midi: 84, duration: 0.08, attack: 0.001, decay: 0.025, sustain: 0, release: 0.05, gain: 0.18, reverb: 'short', type: 'sine' })
        ],
        'space.switch': [
            L('sweep', { freq: DesktopSoundSynth.midiToHz(60), toFreq: DesktopSoundSynth.midiToHz(72), duration: 0.35, sweepTime: 0.32, type: 'sine', gain: 0.28, attack: 0.02, decay: 0.15, sustain: 0.1, release: 0.25, reverb: 'long' }),
            L('noise', { duration: 0.35, gain: 0.06, color: 'pink', filter: { type: 'bandpass', freq: 800, q: 0.6 }, sweep: { from: 400, to: 1800, time: 0.3 }, reverb: 'long' })
        ],
        'dialog.open': [
            L('tone', { midi: 74, duration: 0.2, attack: 0.004, decay: 0.06, sustain: 0.04, release: 0.14, gain: 0.26, reverb: 'plate', partials: [{ ratio: 1, gain: 1, type: 'sine' }, { ratio: 2, gain: 0.2, type: 'triangle' }] }),
            L('tone', { at: 0.05, midi: 77, duration: 0.18, attack: 0.003, decay: 0.05, sustain: 0, release: 0.12, gain: 0.2, reverb: 'short', type: 'triangle' })
        ],
        'dialog.confirm': [
            L('arp', { notes: [74, 77, 81], step: 0.04, noteDur: 0.1, gain: 0.22, type: 'sine', reverb: 'plate' })
        ],
        'dialog.cancel': [
            L('tone', { midi: 70, duration: 0.16, glide: DesktopSoundSynth.midiToHz(65), glideTime: 0.12, attack: 0.002, decay: 0.04, sustain: 0, release: 0.1, gain: 0.24, reverb: 'short', type: 'triangle' })
        ],
        'file.trash': [
            L('noise', { duration: 0.14, gain: 0.12, filter: { type: 'bandpass', freq: 1200, q: 1.2 }, reverb: 'short' }),
            L('tone', { at: 0.04, midi: 64, duration: 0.12, attack: 0.001, decay: 0.04, sustain: 0, release: 0.08, gain: 0.18, reverb: 'short', type: 'triangle' })
        ],
        'file.delete': [
            L('tone', { midi: 60, duration: 0.22, attack: 0.001, decay: 0.06, sustain: 0, release: 0.12, gain: 0.3, reverb: 'short', partials: [{ ratio: 1, gain: 1, type: 'square' }, { ratio: 0.5, gain: 0.2, type: 'sine' }] }),
            L('noise', { at: 0.03, duration: 0.1, gain: 0.1, filter: { type: 'lowpass', freq: 900, q: 0.8 }, reverb: 'short' })
        ],
        'file.drop': [
            L('pluck', { midi: 67, duration: 0.28, gain: 0.26, decay: 2.8, filterFreq: 900, reverb: 'plate' }),
            L('tone', { at: 0.05, midi: 72, duration: 0.15, attack: 0.002, decay: 0.05, sustain: 0, release: 0.1, gain: 0.16, reverb: 'short', type: 'sine' })
        ]
    };
    events._default = events['notify.info'];
    window.DesktopSoundThemes = window.DesktopSoundThemes || {};
    window.DesktopSoundThemes.crystal = { id: 'crystal', events };
})();
