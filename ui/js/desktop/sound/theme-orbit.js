(function () {
    'use strict';
    const S = window.DesktopSoundSynth;
    const L = S.layer;

    // Orbit sounds like a quiet space station: soft synth whooshes, sonar pings, airlock hiss and small HUD
    // blips. Everything is built from sines and band-limited noise with 3 to 30 ms attacks and rounded tails,
    // so a window that opens or closes forty times a day never clicks or bites. Nothing sits above about
    // 4 kHz, and the sparkle of a dissolving window is a handful of faint grains, not a shimmer.
    // The synth peak-normalises every buffer, so this file only balances the layers inside one sound. The key
    // is A: the pings are A5, C#6 and E6, and every shell sound moves between notes of that chord and its
    // neighbours. No layer uses the synth's reverb pool, because its convolver is shared by all layers of one
    // render and would smear a layer into the envelope of its neighbour; the echo of a chime is a second,
    // quieter and darker tone instead.

    // A rounded sine blip with an optional pitch glide: attack, a short fall to `sustain` and an exponential
    // release. The oscillator is kept running for attack + decay, and the synth adds the release to that, so
    // it never stops while the envelope is still audible.
    function blip(freq, at, o) {
        o = o || {};
        const attack = o.attack || 0.004;
        const decay = o.decay || 0.03;
        return L('tone', {
            freq, glide: o.to, glideTime: o.glideTime || 0.04, at, duration: attack + decay,
            gain: o.gain == null ? 0.3 : o.gain, attack, decay,
            sustain: o.sustain == null ? 0.2 : o.sustain, release: o.release || 0.08,
            type: 'sine', partials: o.partials, pan: o.pan,
            filter: { type: 'lowpass', freq: o.cutoff || 3200, q: 0.5 }
        });
    }

    // A pitched swoosh: a sine that glides from one note to another.
    function glide(from, to, at, time, o) {
        o = o || {};
        return blip(from, at, Object.assign({ to, glideTime: time, attack: 0.012, decay: time * 0.8,
            sustain: 0.1, release: 0.06, cutoff: 2600 }, o));
    }

    // A whoosh: noise through a band-pass filter whose centre sweeps from one frequency to another. The level
    // rises over `attack` seconds and falls over `decay` seconds, so a rising whoosh peaks late and a falling
    // one early.
    function whoosh(from, to, at, dur, o) {
        o = o || {};
        return L('noise', {
            at, duration: dur, gain: o.gain == null ? 0.4 : o.gain,
            attack: o.attack || dur * 0.45, decay: o.decay || dur * 0.55, release: 0.02,
            color: 'pink', decayPower: 0.3, filter: { type: 'bandpass', q: o.q || 1.1 },
            sweep: { from, to, time: dur }
        });
    }

    // A soft puff of air (an airlock seal, a latch) through a low-pass filter.
    function puff(at, dur, cutoff, gain) {
        return L('noise', {
            at, duration: dur, gain, attack: 0.006, decay: dur * 0.9, release: 0.02,
            color: 'pink', decayPower: 1.6, filter: { type: 'lowpass', freq: cutoff, q: 0.6 }
        });
    }

    // Faint grains of the dissolving window: very short sines on notes of the key, alternating left and right.
    const GRAINS = [
        { at: 0.095, note: 2217, gain: 0.085, pan: -0.5 }, { at: 0.128, note: 1760, gain: 0.08, pan: 0.45 },
        { at: 0.152, note: 2637, gain: 0.066, pan: -0.3 }, { at: 0.190, note: 1976, gain: 0.06, pan: 0.5 },
        { at: 0.214, note: 1480, gain: 0.05, pan: -0.45 }, { at: 0.247, note: 1760, gain: 0.04, pan: 0.3 },
        { at: 0.280, note: 1319, gain: 0.03, pan: -0.2 }
    ];

    function sparkle() {
        return GRAINS.map(g => blip(g.note, g.at, { gain: g.gain, pan: g.pan, attack: 0.003, decay: 0.012,
            sustain: 0.3, release: 0.03, cutoff: 4000 }));
    }

    // A sonar-ish chime of two sine notes with a body of a quiet second partial and a darker, quieter echo of
    // the last note.
    const BELL = [{ ratio: 1, gain: 1 }, { ratio: 2, gain: 0.16, type: 'triangle' }];

    function chime(first, second, gap, echoAfter, o) {
        o = o || {};
        return [
            blip(first, 0, { gain: 0.3, partials: BELL, decay: 0.07, sustain: 0.12, release: o.release || 0.24, cutoff: 3600 }),
            blip(second, gap, { gain: 0.34, partials: BELL, decay: 0.08, sustain: 0.12, release: (o.release || 0.24) + 0.08, cutoff: 3600 }),
            blip(second, gap + echoAfter, { gain: 0.12, decay: 0.06, sustain: 0.1, release: o.echoRelease || 0.22, cutoff: 1900 })
        ];
    }

    // A low, muted pair of blips: it informs without scolding.
    const MUTED = [{ ratio: 1, gain: 1 }, { ratio: 2, gain: 0.26, type: 'triangle' }];

    function lowBlips(first, second, gap, gain, release) {
        return [
            blip(first, 0, { gain, partials: MUTED, decay: 0.03, sustain: 0.18, release, cutoff: 700 }),
            blip(second, gap, { gain: gain * 0.9, partials: MUTED, decay: 0.03, sustain: 0.18, release, cutoff: 650 })
        ];
    }

    const events = {
        // Materialise: a whoosh that builds, a soft chirp rising an octave and a faint FM glint on the arrival.
        'window.open': [
            whoosh(280, 2300, 0, 0.22, { gain: 0.5, attack: 0.15, decay: 0.07, q: 1.0 }),
            glide(392, 880, 0.03, 0.15, { gain: 0.3, attack: 0.03, decay: 0.12, release: 0.07 }),
            L('fm', { midi: 88, at: 0.12, duration: 0.1, gain: 0.13, attack: 0.01, decay: 0.07, sustain: 0, release: 0.08,
                modRatio: 2, index: 120, indexDecay: 0.14, filter: { type: 'lowpass', freq: 3800, q: 0.5 } })
        ],
        // Dissolve: a falling whoosh, a sinking chirp and a tail of faint grains.
        'window.close': [
            whoosh(2400, 260, 0, 0.26, { gain: 0.42, attack: 0.05, decay: 0.21, q: 1.0 }),
            glide(740, 330, 0, 0.22, { gain: 0.26, attack: 0.01, decay: 0.19, release: 0.06 })
        ].concat(sparkle()),
        'window.minimize': [
            whoosh(1900, 520, 0, 0.15, { gain: 0.4, attack: 0.03, decay: 0.12, q: 1.2 }),
            glide(587, 294, 0, 0.14, { gain: 0.28, attack: 0.008, release: 0.05 })
        ],
        'window.restore': [
            whoosh(500, 2100, 0, 0.16, { gain: 0.38, attack: 0.11, decay: 0.05, q: 1.2 }),
            glide(294, 587, 0, 0.15, { gain: 0.26, attack: 0.02, release: 0.05 }),
            blip(1319, 0.12, { gain: 0.1, release: 0.07 })
        ],
        'window.maximize': [
            whoosh(220, 1700, 0, 0.22, { gain: 0.42, attack: 0.15, decay: 0.07, q: 0.9 }),
            glide(220, 440, 0, 0.2, { gain: 0.32, attack: 0.02, decay: 0.17, release: 0.06 }),
            blip(880, 0.17, { gain: 0.14, release: 0.1 }),
            blip(1319, 0.2, { gain: 0.09, release: 0.12 })
        ],
        // An airlock seal: a soft thump, a puff of air and a tiny latch.
        'window.snap': [
            glide(190, 120, 0, 0.06, { gain: 0.42, attack: 0.003, decay: 0.04, release: 0.06, cutoff: 900 }),
            puff(0, 0.06, 1700, 0.22),
            blip(1760, 0.045, { gain: 0.07, release: 0.06 })
        ],
        'window.deny': lowBlips(196, 165, 0.1, 0.42, 0.06),
        'notify.info': chime(659, 880, 0.12, 0.17),
        'notify.message': chime(988, 1319, 0.09, 0.14, { release: 0.2, echoRelease: 0.18 }),
        'notify.error': lowBlips(220, 185, 0.11, 0.46, 0.09),
        // The tiny HUD blips of menus: a short glide up to open and down to close.
        'menu.open': [blip(880, 0, { to: 1175, glideTime: 0.035, gain: 0.3, decay: 0.02, sustain: 0.15, release: 0.05 })],
        'menu.close': [blip(1047, 0, { to: 698, glideTime: 0.035, gain: 0.26, decay: 0.02, sustain: 0.15, release: 0.05 })],
        'space.switch': [
            L('noise', { duration: 0.15, gain: 0.34, attack: 0.08, decay: 0.07, release: 0.02, color: 'pink', decayPower: 0.3,
                filter: { type: 'bandpass', q: 2.2 }, sweep: { from: 900, to: 1500, time: 0.15 } }),
            blip(659, 0.07, { gain: 0.14, release: 0.07 }),
            blip(880, 0.12, { gain: 0.16, release: 0.1 })
        ],
        'dialog.open': [
            blip(880, 0, { gain: 0.26, release: 0.08 }),
            blip(1319, 0.05, { gain: 0.24, release: 0.12 }),
            puff(0, 0.05, 1500, 0.1)
        ],
        'dialog.confirm': [
            blip(880, 0, { gain: 0.26, partials: BELL, release: 0.18, cutoff: 3600 }),
            blip(1319, 0.07, { gain: 0.32, partials: BELL, release: 0.28, cutoff: 3600 }),
            blip(1319, 0.24, { gain: 0.11, release: 0.18, cutoff: 1900 })
        ],
        'dialog.cancel': [
            blip(1047, 0, { gain: 0.24, release: 0.08, cutoff: 2400 }),
            blip(784, 0.06, { gain: 0.22, release: 0.1, cutoff: 2400 })
        ],
        'file.trash': [
            whoosh(1300, 600, 0, 0.16, { gain: 0.38, attack: 0.015, decay: 0.145, q: 1.0 }),
            glide(300, 170, 0, 0.1, { gain: 0.25, attack: 0.006, release: 0.06 })
        ],
        'file.delete': [
            whoosh(2000, 500, 0, 0.26, { gain: 0.45, attack: 0.02, decay: 0.24, q: 0.9 }),
            glide(220, 110, 0, 0.14, { gain: 0.36, attack: 0.006, release: 0.07, cutoff: 900 }),
            blip(165, 0.17, { gain: 0.2, release: 0.08, cutoff: 800 })
        ],
        'file.drop': [
            puff(0, 0.07, 1300, 0.25),
            glide(260, 190, 0, 0.06, { gain: 0.35, attack: 0.004, decay: 0.04, release: 0.05, cutoff: 900 }),
            blip(1319, 0.05, { gain: 0.09, release: 0.08 })
        ]
    };
    events._default = events['notify.info'];

    // Theme FX: orbit.ping is not a shell event. The Orbit desktop effect asks for it by name through
    // sound-runtime.js when the desktop is clicked and passes a variant for how far the pulse travels. A
    // sonar ping is a sine with a 6 ms attack that decays exponentially over 420 to 700 ms, an echo 180 ms
    // later that is 9 dB quieter and a little darker, and a faint noise click at the start. The runtime plays
    // it at 0.38 of a window sound, at most once per 350 ms and three times per 2 s.
    const PING = {
        small: { freq: 1320, release: 0.42 },
        medium: { freq: 1100, release: 0.56 },
        large: { freq: 880, release: 0.7 }
    };
    const ECHO_AT = 0.18;
    const ECHO_GAIN = Math.pow(10, -9 / 20);

    function ping(s) {
        const ring = { duration: 0.01, attack: 0.006, decay: 0.001, sustain: 1, release: s.release, type: 'sine' };
        return [
            L('noise', { duration: 0.004, gain: 0.05, attack: 0.001, decay: 0.004, color: 'white', decayPower: 1,
                filter: { type: 'bandpass', freq: 2400, q: 0.9 } }),
            L('tone', Object.assign({ freq: s.freq, gain: 0.5, filter: { type: 'lowpass', freq: s.freq * 2.5, q: 0.5 } }, ring)),
            L('tone', Object.assign({ freq: s.freq, at: ECHO_AT, gain: 0.5 * ECHO_GAIN,
                filter: { type: 'lowpass', freq: s.freq * 1.6, q: 0.5 } }, ring))
        ];
    }

    const specialFx = { 'orbit.ping': {} };
    Object.keys(PING).forEach(size => { specialFx['orbit.ping'][size] = ping(PING[size]); });

    window.DesktopSoundThemes = window.DesktopSoundThemes || {};
    window.DesktopSoundThemes.orbit = { id: 'orbit', events, specialFx };
})();
