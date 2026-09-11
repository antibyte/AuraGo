(function () {
    'use strict';
    const L = DesktopSoundSynth.layer;
    const hz = DesktopSoundSynth.midiToHz;
    const events = {
        'window.open': [
            L('sweep', { freq: hz(48), toFreq: hz(72), duration: 0.32, sweepTime: 0.28, type: 'sawtooth', gain: 0.32, attack: 0.008, decay: 0.12, sustain: 0.08, release: 0.2, filter: { type: 'lowpass', freq: 2200, q: 1.2 }, reverb: 'plate' }),
            L('fm', { at: 0.08, midi: 76, modRatio: 2, index: 45, indexDecay: 0.12, duration: 0.22, gain: 0.18, filter: { type: 'lowpass', freq: 4000, q: 0.8 }, reverb: 'short' })
        ],
        'window.close': [
            L('sweep', { freq: hz(72), toFreq: hz(52), duration: 0.24, sweepTime: 0.22, type: 'square', gain: 0.28, attack: 0.002, decay: 0.08, sustain: 0, release: 0.15, filter: { type: 'lowpass', freq: 1800, q: 1.5 }, reverb: 'short' }),
            L('noise', { duration: 0.08, gain: 0.06, filter: { type: 'highpass', freq: 2500, q: 1 }, reverb: 'short' })
        ],
        'window.minimize': [
            L('tone', { midi: 64, duration: 0.14, attack: 0.002, decay: 0.05, sustain: 0, release: 0.08, gain: 0.26, type: 'sawtooth', filter: { type: 'lowpass', freq: 1200, q: 2 }, reverb: 'short' })
        ],
        'window.restore': [
            L('sweep', { freq: hz(55), toFreq: hz(67), duration: 0.18, sweepTime: 0.16, type: 'triangle', gain: 0.24, reverb: 'plate' })
        ],
        'window.maximize': [
            L('arp', { notes: [60, 64, 67, 72], step: 0.04, noteDur: 0.1, gain: 0.24, type: 'sawtooth', filter: { type: 'lowpass', freq: 2600, q: 1 }, reverb: 'plate' }),
            L('fm', { at: 0.16, midi: 79, modRatio: 3, index: 35, duration: 0.18, gain: 0.16, reverb: 'short' })
        ],
        'window.snap': [
            L('tone', { midi: 78, duration: 0.08, attack: 0.001, decay: 0.03, sustain: 0, release: 0.05, gain: 0.3, type: 'square', filter: { type: 'bandpass', freq: 1800, q: 2.5 }, reverb: 'short' })
        ],
        'window.deny': [
            L('tone', { midi: 55, duration: 0.16, attack: 0.001, decay: 0.04, sustain: 0, release: 0.08, gain: 0.32, type: 'sawtooth', detune: -8, filter: { type: 'lowpass', freq: 900, q: 2 }, reverb: 'short' }),
            L('tone', { at: 0.06, midi: 52, duration: 0.14, gain: 0.28, type: 'square', filter: { type: 'lowpass', freq: 700, q: 1.5 }, reverb: 'short' })
        ],
        'notify.info': [
            L('arp', { notes: [72, 76, 79, 84], step: 0.045, noteDur: 0.11, gain: 0.22, type: 'sawtooth', filter: { type: 'lowpass', freq: 3200, q: 0.9 }, reverb: 'plate' }),
            L('fm', { at: 0.2, midi: 84, modRatio: 2.5, index: 30, duration: 0.2, gain: 0.14, reverb: 'short' })
        ],
        'notify.message': [
            L('fm', { midi: 69, modRatio: 2.2, index: 38, indexDecay: 0.1, duration: 0.28, gain: 0.26, filter: { type: 'lowpass', freq: 2800, q: 1.2 }, reverb: 'plate' }),
            L('tone', { at: 0.1, midi: 76, duration: 0.15, gain: 0.14, type: 'triangle', reverb: 'short' })
        ],
        'notify.error': [
            L('tone', { midi: 45, duration: 0.22, gain: 0.34, type: 'sawtooth', detune: -12, filter: { type: 'lowpass', freq: 800, q: 2.5 }, reverb: 'short' }),
            L('noise', { at: 0.04, duration: 0.14, gain: 0.12, filter: { type: 'bandpass', freq: 600, q: 1.8 }, reverb: 'short' }),
            L('tone', { at: 0.1, midi: 42, duration: 0.18, gain: 0.28, type: 'square', filter: { type: 'lowpass', freq: 500, q: 2 }, reverb: 'short' })
        ],
        'menu.open': [
            L('tone', { midi: 88, duration: 0.06, gain: 0.2, type: 'square', filter: { type: 'highpass', freq: 2000, q: 1 }, reverb: 'short' })
        ],
        'menu.close': [
            L('tone', { midi: 80, duration: 0.05, gain: 0.16, type: 'triangle', filter: { type: 'lowpass', freq: 3000, q: 0.8 }, reverb: 'short' })
        ],
        'space.switch': [
            L('sweep', { freq: hz(52), toFreq: hz(64), duration: 0.4, sweepTime: 0.38, type: 'sawtooth', gain: 0.26, attack: 0.03, decay: 0.2, sustain: 0.12, release: 0.25, filter: { type: 'lowpass', freq: 1600, q: 0.7 }, reverb: 'long' }),
            L('noise', { duration: 0.35, gain: 0.07, color: 'pink', filter: { type: 'bandpass', freq: 700, q: 0.5 }, reverb: 'long' })
        ],
        'dialog.open': [
            L('fm', { midi: 65, modRatio: 2, index: 32, duration: 0.2, gain: 0.24, reverb: 'plate' }),
            L('tone', { at: 0.06, midi: 69, duration: 0.12, gain: 0.16, type: 'triangle', reverb: 'short' })
        ],
        'dialog.confirm': [
            L('arp', { notes: [65, 69, 72], step: 0.038, noteDur: 0.09, gain: 0.22, type: 'sawtooth', reverb: 'plate' })
        ],
        'dialog.cancel': [
            L('sweep', { freq: hz(67), toFreq: hz(55), duration: 0.14, sweepTime: 0.12, type: 'triangle', gain: 0.22, reverb: 'short' })
        ],
        'file.trash': [
            L('noise', { duration: 0.12, gain: 0.14, filter: { type: 'bandpass', freq: 1100, q: 1.4 }, reverb: 'short' }),
            L('tone', { at: 0.03, midi: 58, duration: 0.1, gain: 0.2, type: 'square', filter: { type: 'lowpass', freq: 900, q: 1.5 }, reverb: 'short' })
        ],
        'file.delete': [
            L('tone', { midi: 50, duration: 0.18, gain: 0.3, type: 'sawtooth', filter: { type: 'lowpass', freq: 700, q: 2 }, reverb: 'short' }),
            L('noise', { at: 0.04, duration: 0.1, gain: 0.1, filter: { type: 'lowpass', freq: 500, q: 1 }, reverb: 'short' })
        ],
        'file.drop': [
            L('fm', { midi: 62, modRatio: 1.8, index: 28, duration: 0.2, gain: 0.24, reverb: 'plate' }),
            L('tone', { at: 0.05, midi: 67, duration: 0.12, gain: 0.16, type: 'triangle', reverb: 'short' })
        ]
    };
    events._default = events['notify.info'];
    window.DesktopSoundThemes = window.DesktopSoundThemes || {};
    window.DesktopSoundThemes.analog = { id: 'analog', events };
})();
