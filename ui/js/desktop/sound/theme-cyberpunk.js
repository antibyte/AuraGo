// Cyberpunk: dark pulse, metal ping, stutter. A low driven pulse is triggered two or three times in a row like a glitch
// (the last repeat moves one semitone or a minor third to show the direction), one small metal ping sits high above it
// on notices and confirms, a faint static tick marks closings. Key A3 (57), A Phrygian: 0 1 3 5 7 8 10.
// The pulse lives around D3-C4 (the note floor is 130 Hz). Subtle and refined: static stays faint, no harsh buzz.
(function () {
    'use strict';
    const LIFT = 7; // the stutter pulses sit a fifth above the key: shorter, lower notes leak too much energy below 90 Hz
    const echo = (time, feedback, mix) => ({ time, feedback, tone: 2000, mix, spread: 0.5 });
    const plate = (rt60, mix) => ({ size: 0.8, rt60, damping: 3800, predelay: 0.008, mix });
    const room = { size: 0.4, rt60: 0.1, damping: 5000, predelay: 0.002, mix: 0.1 };
    // A stutter: the pulse `v` is triggered `count` times `step` s apart, each repeat quieter; the last one moves to `last`.
    const stut = (v, n, t0, last, count, step, g, extra) => {
        const out = [];
        for (let i = 0; i < count; i++) {
            const final = i === count - 1;
            out.push(Object.assign({ v, n: (final ? last : n) + LIFT, t: t0 + i * step, g: g * (1 - 0.3 * i), pan: i % 2 ? 0.15 : -0.15 }, final ? { decay: 0.16 } : {}, extra));
        }
        return out;
    };
    self.DesktopSoundEngine.defineTheme({
        id: 'cyberpunk',
        key: 57,
        space: { size: 0.8, rt60: 0.5, damping: 3800, predelay: 0.008, mix: 0.2 },
        voices: {
            pulse: { model: 'analog', wave: 'saw', pw: 0.5, unison: 3, detune: 10, drift: 3, sub: 0, cutoff: 1.4, env: 3, envDecay: 0.05, q: 1.8, drive: 2, attack: 0.003, decay: 0.09, width: 0.5 },
            dull: { model: 'analog', wave: 'pulse', pw: 0.5, unison: 2, detune: 6, drift: 3, sub: 0, cutoff: 1, env: 2, envDecay: 0.08, q: 1.2, drive: 1.3, attack: 0.006, decay: 0.25, width: 0.3 },
            ping: { model: 'metal', kind: 'plate', t60: 0.3, damp: 0.5, beat: 2, hardness: 0.1, strike: 0, attack: 0.003, width: 0.5 },
            static: { model: 'noise', color: 'white', band: [3000, 8000], attack: 0.001, decay: 0.012, width: 0.9 },
            relay: { model: 'click', material: 'relay', tone: 2600, ring: 0.014, noise: 0.6 },
            hiss: { model: 'noise', color: 'pink', band: [900, 5000], attack: 0.05, decay: 0.2 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: stut('pulse', -9, 0, -9, 3, 0.035, 1, { decay: 0.5 }).concat(stut('pulse', -4, 0.3, -2, 3, 0.035, 1, { decay: 0.3 }), stut('pulse', 0, 0.6, 3, 3, 0.035, 1, { decay: 0.3 }),
                    [{ v: 'ping', n: 24, t: 0.66, g: 0.8, pan: -0.2, t60: 0.6 }, { v: 'ping', n: 27, t: 0.9, g: 0.7, pan: 0.2, t60: 0.9 }, { v: 'pulse', n: 2, t: 0.9, g: 0.7, pan: 0, decay: 0.8 }]),
                layers: [{ v: 'hiss', t: 0, g: 0.05 }, { v: 'static', t: 0.3, g: 0.05 }, { v: 'static', t: 0.6, g: 0.05 }],
                echo: echo(0.22, 0.3, 0.26),
                space: { size: 1.1, rt60: 1.3, damping: 3800, predelay: 0.02, mix: 0.3 }
            },
            'notify.message': {
                notes: stut('pulse', -5, 0, -4, 3, 0.03, 1).concat(stut('pulse', -4, 0.14, 0, 3, 0.03, 1), [{ v: 'ping', n: 24, t: 0.26, g: 0.8, pan: 0.1, t60: 0.45 }]),
                echo: echo(0.2, 0.15, 0.22),
                space: plate(0.6, 0.22)
            },
            'notify.info': {
                notes: stut('pulse', -2, 0, 1, 3, 0.03, 1).concat([{ v: 'ping', n: 24, t: 0.1, g: 0.8, pan: 0.1, t60: 0.4 }]),
                echo: echo(0.18, 0.15, 0.2),
                space: plate(0.5, 0.2)
            },
            'notify.success': {
                notes: stut('pulse', -5, 0, -2, 3, 0.03, 1).concat(stut('pulse', 0, 0.13, 3, 2, 0.03, 1), [{ v: 'ping', n: 27, t: 0.17, g: 0.8, pan: 0.1, t60: 0.4 }]),
                echo: echo(0.16, 0.12, 0.16),
                space: plate(0.45, 0.2)
            },
            'notify.warning': {
                notes: stut('pulse', -4, 0, -4, 2, 0.035, 1, { attack: 0.007, decay: 0.12 }).concat(stut('pulse', -4, 0.2, -4, 2, 0.035, 0.85, { attack: 0.007, decay: 0.12 }), [{ v: 'ping', n: 25, t: 0, g: 0.5, pan: 0.1, t60: 0.2 }]),
                layers: [{ v: 'static', t: 0.2, g: 0.04 }],
                echo: echo(0.2, 0.12, 0.14),
                space: plate(0.35, 0.16)
            },
            'notify.error': {
                notes: stut('dull', 3, 0, 3, 2, 0.035, 1, { attack: 0.01, decay: 0.2, cutoff: 0.8 }).concat(stut('dull', -3, 0.17, -3, 2, 0.035, 0.9, { attack: 0.01, decay: 0.3, cutoff: 0.8 })),
                layers: [{ v: 'knock', t: 0, g: 0.4 }, { v: 'knock', t: 0.17, g: 0.35 }],
                echo: echo(0.18, 0.12, 0.14),
                space: plate(0.45, 0.16)
            },
            'dialog.open': {
                notes: stut('pulse', -5, 0, -2, 3, 0.03, 1).concat([{ v: 'ping', n: 24, t: 0.09, g: 0.5, pan: 0.15, t60: 0.15 }]),
                echo: echo(0.14, 0.05, 0.12),
                space: plate(0.2, 0.14)
            },
            'dialog.confirm': {
                notes: stut('pulse', -2, 0, 3, 3, 0.04, 1).concat([{ v: 'ping', n: 28, t: 0.1, g: 0.7, pan: 0.15, t60: 0.25 }]),
                echo: echo(0.16, 0.05, 0.12),
                space: plate(0.22, 0.14)
            },
            'dialog.cancel': {
                notes: [{ v: 'ping', n: 24, t: 0, g: 0.4, pan: 0.15, t60: 0.1 }].concat(stut('pulse', 3, 0.02, -2, 3, 0.03, 1)),
                layers: [{ v: 'static', t: 0.08, g: 0.04 }],
                space: plate(0.15, 0.12)
            },
            'window.open': {
                notes: stut('pulse', -5, 0, -2, 3, 0.04, 1).concat([{ v: 'ping', n: 24, t: 0.08, g: 0.35, pan: 0.15, t60: 0.14 }]),
                layers: [{ v: 'relay', t: 0, g: 0.3 }],
                echo: echo(0.12, 0, 0.16),
                space: plate(0.15, 0.12)
            },
            'window.close': {
                notes: stut('pulse', -2, 0, -5, 3, 0.03, 1),
                layers: [{ v: 'static', t: 0.06, g: 0.04 }],
                space: plate(0.15, 0.12)
            },
            'window.maximize': {
                notes: stut('pulse', -7, 0, -4, 3, 0.03, 1).concat(stut('pulse', 0, 0.12, 3, 2, 0.03, 1)),
                layers: [{ v: 'relay', t: 0, g: 0.3 }],
                space: plate(0.18, 0.16)
            },
            'window.minimize': {
                notes: stut('pulse', 0, 0, -4, 2, 0.03, 1, { decay: 0.07 }),
                layers: [{ v: 'static', t: 0.04, g: 0.04 }],
                space: plate(0.15, 0.1)
            },
            'window.restore': {
                notes: stut('pulse', -4, 0, 0, 2, 0.03, 1, { decay: 0.07 }),
                space: plate(0.15, 0.1)
            },
            'window.snap': {
                layers: [{ v: 'relay', t: 0, g: 1, tone: 2800, ring: 0.012 }, { v: 'relay', t: 0.025, g: 0.5, tone: 2000, ring: 0.01 }],
                space: room
            },
            'window.deny': {
                notes: stut('dull', -2, 0, -2, 2, 0.04, 0.9, { attack: 0.007, decay: 0.1, cutoff: 0.8 }),
                layers: [{ v: 'knock', t: 0, g: 0.5 }, { v: 'relay', t: 0, g: 0.7, tone: 900, ring: 0.02 }],
                space: { size: 0.5, rt60: 0.2, damping: 3500, predelay: 0.004, mix: 0.12 }
            },
            'file.drop': {
                notes: stut('pulse', 5, 0, 0, 3, 0.045, 1, { decay: 0.12 }).concat([{ v: 'ping', n: 12, t: 0.09, g: 0.6, pan: 0.1, t60: 0.2 }]),
                layers: [{ v: 'knock', t: 0, g: 0.3, band: [300, 1200] }, { v: 'relay', t: 0, g: 0.5, tone: 1800 }],
                space: { size: 0.7, rt60: 0.3, damping: 3800, predelay: 0.006, mix: 0.16 }
            },
            'file.trash': {
                notes: stut('pulse', 3, 0, -4, 3, 0.04, 1, { decay: 0.07 }),
                layers: [{ v: 'hiss', t: 0, g: 0.1, attack: 0.02, decay: 0.12 }, { v: 'static', t: 0.08, g: 0.04 }],
                space: plate(0.25, 0.14)
            },
            'file.delete': {
                notes: stut('pulse', 1, 0, -5, 3, 0.04, 1, { decay: 0.1 }).concat([{ v: 'dull', n: 0, t: 0.12, g: 0.8, pan: 0, decay: 0.28, cutoff: 1 }]),
                layers: [{ v: 'knock', t: 0.12, g: 0.4 }, { v: 'static', t: 0.12, g: 0.04 }],
                echo: echo(0.14, 0.05, 0.1),
                space: plate(0.3, 0.16)
            },
            'space.switch': {
                notes: [{ v: 'pulse', n: 3, t: 0.03, g: 0.6, pan: 0, attack: 0.12, decay: 0.12, cutoff: 0.8 }, { v: 'ping', n: 24, t: 0.2, g: 0.4, pan: 0.1, t60: 0.15 }],
                layers: [{ v: 'hiss', t: 0, g: 0.8, attack: 0.12, decay: 0.12, band: [900, 4000] }, { v: 'static', t: 0.15, g: 0.04 }],
                echo: echo(0.14, 0.05, 0.08),
                space: plate(0.2, 0.16)
            },
            'menu.open': {
                notes: stut('pulse', -2, 0, 1, 2, 0.03, 0.8, { decay: 0.05, dur: 0.07 }),
                layers: [{ v: 'relay', t: 0, g: 0.7, tone: 2400 }, { v: 'relay', t: 0.03, g: 0.8, tone: 3200 }],
                space: room
            },
            'menu.close': {
                notes: stut('pulse', 1, 0, -2, 2, 0.03, 0.8, { decay: 0.07, dur: 0.085 }),
                layers: [{ v: 'relay', t: 0, g: 0.7, tone: 3200 }, { v: 'relay', t: 0.03, g: 0.8, tone: 2400 }, { v: 'static', t: 0.03, g: 0.04 }],
                space: { size: 0.4, rt60: 0.07, damping: 5000, predelay: 0.002, mix: 0.06 }
            },
            'ui.select': {
                layers: [{ v: 'relay', t: 0, g: 1, tone: 2400, ring: 0.02 }, { v: 'static', t: 0, g: 0.04 }],
                space: { size: 0.4, rt60: 0.08, damping: 5000, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'relay', t: 0, g: 0.9, tone: 2200, ring: 0.015 }, { v: 'relay', t: 0.04, g: 1, tone: 3400, ring: 0.015 }, { v: 'hiss', t: 0, g: 0.05, attack: 0.005, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.1, damping: 5000, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.off': {
                layers: [{ v: 'relay', t: 0, g: 0.9, tone: 3400, ring: 0.015 }, { v: 'relay', t: 0.04, g: 1, tone: 2200, ring: 0.015 }, { v: 'hiss', t: 0, g: 0.05, attack: 0.005, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.1, damping: 5000, predelay: 0.002, mix: 0.08 }
            },
            'ui.tick': {
                layers: [{ v: 'relay', t: 0, g: 1, tone: 3600, ring: 0.008, length: 0.0015 }],
                space: null
            }
        },
        fx: {}
    });
})();
