(function () {
    'use strict';
    const S = window.DesktopSoundSynth;
    const L = S.layer;

    // Theme FX are not shell events. The Green Slime desktop effect asks for them by name through
    // sound-runtime.js: slime.pop when a bubble bursts and slime.snap when a pulled strand parts. Each has a
    // small, medium and large variant. The synth peak-normalises every buffer, so this file only sets the
    // balance inside one sound; the runtime plays them at about 40% of a window sound and rate-limits them.
    // No layer uses the synth's reverb pool, because its convolver is shared by all layers of one render and
    // would smear a layer into the envelope of its neighbour. Every layer is low-passed or band-limited.

    // Bubble burst: a sine blop whose pitch glides upward (a smaller bubble rings higher), a very short
    // band-passed tick of the film tearing, and a faint low-passed tail of wet air.
    const POP = {
        small: { from: 520, to: 1100, glide: 0.030, attack: 0.003, decay: 0.016, sustain: 0.20, release: 0.050,
            click: 3800, tick: 0.002, wet: 1100, tail: 0.060 },
        medium: { from: 360, to: 760, glide: 0.042, attack: 0.004, decay: 0.022, sustain: 0.22, release: 0.075,
            click: 3000, tick: 0.004, wet: 800, tail: 0.080 },
        large: { from: 240, to: 520, glide: 0.058, attack: 0.005, decay: 0.030, sustain: 0.25, release: 0.105,
            click: 2300, tick: 0.006, wet: 600, tail: 0.100 }
    };

    function pop(s) {
        return [
            L('noise', { duration: s.tick, gain: 0.09, attack: 0.0005, decay: s.tick, color: 'white', decayPower: 1,
                filter: { type: 'bandpass', freq: s.click, q: 0.9 } }),
            L('tone', { at: 0.002, freq: s.from, glide: s.to, glideTime: s.glide, duration: s.attack + s.decay,
                gain: 0.5, attack: s.attack, decay: s.decay, sustain: s.sustain, release: s.release, type: 'sine',
                filter: { type: 'lowpass', freq: s.to * 1.8, q: 0.5 } }),
            L('noise', { at: 0.01, duration: s.tail, gain: 0.06, attack: 0.006, decay: s.tail, color: 'pink', decayPower: 1.6,
                filter: { type: 'lowpass', freq: s.wet, q: 0.6 } })
        ];
    }

    // Strand parting: a wet "thwip" (band-passed noise whose centre sweeps down) and, a moment later, the
    // soft low plop of the base recoiling into the gel. The plop gets a quiet octave above it so that small
    // speakers, which drop everything below about 200 Hz, still carry it.
    const SNAP = {
        small: { from: 3400, to: 1100, q: 1.4, sweep: 0.025, plopAt: 0.036, plopFrom: 240, plopTo: 160, lowpass: 520, tail: 0.060 },
        medium: { from: 3000, to: 900, q: 1.3, sweep: 0.032, plopAt: 0.040, plopFrom: 205, plopTo: 138, lowpass: 440, tail: 0.075 },
        large: { from: 2600, to: 750, q: 1.2, sweep: 0.040, plopAt: 0.046, plopFrom: 180, plopTo: 120, lowpass: 380, tail: 0.090 }
    };

    function snap(s) {
        const plop = { at: s.plopAt, glideTime: 0.060, duration: 0.036, attack: 0.006, decay: 0.030, sustain: 0.25, release: 0.070, type: 'sine' };
        return [
            L('noise', { duration: s.sweep + 0.004, gain: 0.7, attack: 0.002, decay: s.sweep + 0.004, color: 'white', decayPower: 1,
                filter: { type: 'bandpass', freq: s.from, q: s.q }, sweep: { from: s.from, to: s.to, time: s.sweep } }),
            L('tone', Object.assign({ freq: s.plopFrom, glide: s.plopTo, gain: 0.5,
                filter: { type: 'lowpass', freq: s.lowpass, q: 0.5 } }, plop)),
            L('tone', Object.assign({ freq: s.plopFrom * 2, glide: s.plopTo * 2, gain: 0.16,
                filter: { type: 'lowpass', freq: s.lowpass * 2, q: 0.5 } }, plop)),
            L('noise', { at: s.plopAt + 0.004, duration: s.tail, gain: 0.04, attack: 0.008, decay: s.tail, color: 'pink', decayPower: 1.6,
                filter: { type: 'lowpass', freq: s.lowpass * 1.2, q: 0.6 } })
        ];
    }

    const specialFx = { 'slime.pop': {}, 'slime.snap': {} };
    Object.keys(POP).forEach(size => {
        specialFx['slime.pop'][size] = pop(POP[size]);
        specialFx['slime.snap'][size] = snap(SNAP[size]);
    });

    window.DesktopSoundThemes.green_slime = {
        id: 'green_slime',
        events: S.composeDesktopEvents({ voice: 'slime', root: 53, rise: 5, fall: -7, color: 3,
            step: 0.082, gain: 0.24, tail: 0.15, filter: 980, resonance: 3.8, noiseCutoff: 680 }),
        specialFx
    };
})();
