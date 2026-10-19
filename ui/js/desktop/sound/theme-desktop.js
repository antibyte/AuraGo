(function () {
    'use strict';
    const S = window.DesktopSoundSynth;
    const L = S.layer;
    // Original sonic identities. Each uses its own tuning, rhythm, instrument and texture.
    // Short navigation gestures, longer confirmations, descending warnings; never a looping bed.
    const profiles = {
        amiga: { root: 72, step: 0.045, voice: 'chip', gain: 0.20, tail: 0.06, chord: [0, 4, 7], filter: 3900 },
        cyberpunk: { root: 57, step: 0.065, voice: 'pulse', gain: 0.22, tail: 0.11, chord: [0, 7, 14], filter: 2100 },
        black_gloss: { root: 65, step: 0.080, voice: 'glass', gain: 0.20, tail: 0.22, chord: [0, 7, 12], filter: 3200 },
        crazy: { root: 69, step: 0.050, voice: 'bubble', gain: 0.20, tail: 0.13, chord: [0, 6, 11, 16], filter: 2600 },
        lollipop: { root: 77, step: 0.075, voice: 'celesta', gain: 0.18, tail: 0.18, chord: [0, 4, 9], filter: 3500 },
        organic: { root: 62, step: 0.095, voice: 'wood', gain: 0.22, tail: 0.20, chord: [0, 5, 9], filter: 1600 }
    };
    function note(p, midi, at, quiet) {
        const base = { midi, at, gain: p.gain * (quiet ? 0.65 : 1), duration: 0.20 + p.tail,
            attack: 0.004, decay: 0.07, sustain: 0, release: p.tail,
            filter: { type: 'lowpass', freq: p.filter, q: 0.7 } };
        switch (p.voice) {
            case 'chip': return L('tone', Object.assign(base, { type: 'square', duration: 0.12, decay: 0.035, partials: [{ ratio: 1, gain: 0.7, type: 'square' }, { ratio: 0.5, gain: 0.25, type: 'triangle' }] }));
            case 'pulse': return L('fm', Object.assign(base, { modRatio: 2, index: 90, indexDecay: 0.08, reverb: 'short' }));
            case 'glass': return L('tone', Object.assign(base, { reverb: 'plate', partials: [{ ratio: 1, gain: 1 }, { ratio: 2.76, gain: 0.14 }, { ratio: 4.07, gain: 0.05 }] }));
            case 'bubble': return L('tone', Object.assign(base, { type: 'sine', glide: S.midiToHz(midi + 7), glideTime: 0.09, reverb: 'short' }));
            case 'celesta': return L('fm', Object.assign(base, { modRatio: 3, index: 24, indexDecay: 0.05, reverb: 'plate' }));
            default: return L('tone', Object.assign(base, { reverb: 'short', partials: [{ ratio: 1, gain: 1 }, { ratio: 2.03, gain: 0.22, type: 'triangle' }, { ratio: 3.97, gain: 0.07 }] }));
        }
    }
    Object.entries(profiles).forEach(([id, p]) => {
        const c = p.chord;
        const scores = {
            'window.open': [0, c[1], c[2]], 'window.close': [c[1], 0, -12],
            'window.minimize': [0, -5], 'window.restore': [-5, 0],
            'window.maximize': [0, c[1], c[2], 12], 'window.snap': [7], 'window.deny': [-7, -8],
            'notify.info': [c[1], c[2], 12], 'notify.message': [0, 12], 'notify.error': [-12, -13, -17],
            'menu.open': [12], 'menu.close': [7], 'space.switch': [-12, 0, c[1]],
            'dialog.open': [0, c[1]], 'dialog.confirm': [0, c[2], 12], 'dialog.cancel': [0, -5],
            'file.trash': [-5, -12], 'file.delete': [-12, -19], 'file.drop': [-12, 0]
        };
        const events = {};
        Object.entries(scores).forEach(([event, notes]) => {
            const quiet = event.startsWith('menu.') || event === 'window.snap';
            events[event] = notes.map((n, i) => note(p, p.root + n, i * p.step, quiet));
            if (id === 'cyberpunk' && !quiet) events[event].push(L('noise', { duration: 0.045, gain: 0.025, filter: { type: 'highpass', freq: 3800 } }));
            if (id === 'organic') events[event].push(L('noise', { duration: 0.12, gain: 0.022, color: 'pink', filter: { type: 'bandpass', freq: 720, q: 0.7 }, reverb: 'short' }));
            if (id === 'crazy' && event === 'notify.info') events[event].push(note(p, p.root + c[3], 0.22, true));
        });
        events._default = events['notify.info'];
        window.DesktopSoundThemes[id] = { id, events };
    });
})();
