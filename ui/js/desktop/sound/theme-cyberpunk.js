// Cyberpunk: deep electronic pulses, metallic signals and tiny static clicks in a dark short plate with an echo.
// Key A3 (57), A Phrygian: 0 1 3 5 7 8 10. Subtle and refined: static stays faint, no harsh buzz.
(function () {
    'use strict';
    const echo = (time, feedback, mix) => ({ time, feedback, tone: 2600, mix, spread: 0.5 });
    const plate = (rt60, mix) => ({ size: 0.8, rt60, damping: 4500, predelay: 0.008, mix });
    self.DesktopSoundEngine.defineTheme({
        id: 'cyberpunk',
        key: 57,
        space: { size: 0.8, rt60: 0.5, damping: 4500, predelay: 0.008, mix: 0.2 },
        voices: {
            pulse: { model: 'analog', wave: 'saw', unison: 3, detune: 12, drift: 4, sub: 0, cutoff: 0.8, env: 5, envDecay: 0.1, q: 2, drive: 2, attack: 0.006, decay: 0.25, width: 0.6 },
            dull: { model: 'analog', wave: 'pulse', pw: 0.5, unison: 2, detune: 6, drift: 3, sub: 0, cutoff: 0.9, env: 3, envDecay: 0.08, q: 1.5, drive: 1.3, attack: 0.006, decay: 0.25, width: 0.3 },
            ping: { model: 'metal', kind: 'plate', t60: 0.7, damp: 0.3, beat: 2.5, hardness: 0.15, attack: 0.002, width: 0.5 },
            pipe: { model: 'metal', kind: 'pipe', t60: 0.5, damp: 0.8, beat: 1.2, hardness: 0.25, attack: 0.002, width: 0.5 },
            static: { model: 'noise', color: 'white', band: [3000, 8000], attack: 0.001, decay: 0.012, width: 0.9 },
            click: { model: 'click', material: 'relay', tone: 3600, ring: 0.012, noise: 0.7 },
            hiss: { model: 'noise', color: 'pink', band: [900, 5000], attack: 0.05, decay: 0.2 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'pulse', n: 0, t: 0, g: 0.9, pan: -0.1, decay: 0.7, cutoff: 0.8 }, { v: 'ping', n: 12, t: 0.2, g: 0.8, pan: -0.25, t60: 0.5 },
                    { v: 'ping', n: 13, t: 0.4, g: 0.85, pan: 0.1, t60: 0.55 }, { v: 'ping', n: 15, t: 0.6, g: 0.9, pan: -0.1, t60: 0.6 },
                    { v: 'pipe', n: 19, t: 0.84, g: 0.7, pan: 0.2, t60: 0.9 }, { v: 'pulse', n: 12, t: 0.84, g: 0.5, pan: 0, decay: 0.8 }],
                layers: [{ v: 'hiss', t: 0, g: 0.05 }, { v: 'static', t: 0.2, g: 0.06 }, { v: 'static', t: 0.6, g: 0.06 }],
                echo: echo(0.22, 0.38, 0.28),
                space: { size: 1.1, rt60: 1.3, damping: 4500, predelay: 0.02, mix: 0.3 }
            },
            'notify.message': {
                notes: [{ v: 'ping', n: 12, t: 0, g: 0.8, pan: -0.2, t60: 0.4 }, { v: 'ping', n: 13, t: 0.12, g: 0.85, pan: 0.15, t60: 0.4 },
                    { v: 'ping', n: 15, t: 0.24, g: 0.95, pan: 0, t60: 0.5 }, { v: 'pulse', n: 0, t: 0, g: 0.7, pan: 0, decay: 0.4 }],
                echo: echo(0.18, 0.3, 0.22),
                space: plate(0.7, 0.24)
            },
            'notify.info': {
                notes: [{ v: 'ping', n: 12, t: 0, g: 0.85, pan: -0.15, t60: 0.3 }, { v: 'ping', n: 17, t: 0.11, g: 1, pan: 0.15, t60: 0.4 }],
                echo: echo(0.16, 0.26, 0.2),
                space: plate(0.6, 0.22)
            },
            'notify.success': {
                notes: [{ v: 'ping', n: 3, t: 0, g: 0.8, pan: -0.2, t60: 0.2 }, { v: 'ping', n: 7, t: 0.1, g: 0.9, pan: 0.1, t60: 0.22 },
                    { v: 'ping', n: 12, t: 0.2, g: 1, pan: 0, t60: 0.35 }, { v: 'pulse', n: 0, t: 0.2, g: 0.6, pan: 0, decay: 0.3 }],
                echo: echo(0.14, 0.2, 0.16),
                space: plate(0.45, 0.2)
            },
            'notify.warning': {
                notes: [{ v: 'ping', n: 13, t: 0, g: 1, pan: -0.1, t60: 0.2 }, { v: 'ping', n: 13, t: 0.18, g: 0.85, pan: 0.1, t60: 0.25 }],
                layers: [{ v: 'static', t: 0, g: 0.05 }],
                echo: echo(0.16, 0.2, 0.14),
                space: plate(0.35, 0.16)
            },
            'notify.error': {
                notes: [{ v: 'dull', n: 12, t: 0, g: 1, pan: -0.1, attack: 0.01, decay: 0.28, cutoff: 0.8 }, { v: 'dull', n: 6, t: 0.16, g: 0.9, pan: 0.1, attack: 0.01, decay: 0.34, cutoff: 0.8 }],
                layers: [{ v: 'knock', t: 0, g: 0.4 }, { v: 'knock', t: 0.16, g: 0.35 }],
                echo: echo(0.16, 0.2, 0.14),
                space: plate(0.45, 0.16)
            },
            'dialog.open': {
                notes: [{ v: 'pulse', n: 5, t: 0, g: 0.85, pan: -0.15, decay: 0.18 }, { v: 'ping', n: 12, t: 0.08, g: 1, pan: 0.15, t60: 0.22 }],
                echo: echo(0.1, 0.15, 0.12),
                space: plate(0.3, 0.16)
            },
            'dialog.confirm': {
                notes: [{ v: 'pulse', n: 7, t: 0, g: 0.85, pan: -0.15, decay: 0.18 }, { v: 'ping', n: 15, t: 0.09, g: 1, pan: 0.15, t60: 0.25 }],
                echo: echo(0.1, 0.15, 0.12),
                space: plate(0.3, 0.16)
            },
            'dialog.cancel': {
                notes: [{ v: 'ping', n: 15, t: 0, g: 0.85, pan: 0.15, t60: 0.15 }, { v: 'pulse', n: 7, t: 0.09, g: 1, pan: -0.15, decay: 0.16 }],
                echo: echo(0.1, 0.12, 0.1),
                space: plate(0.25, 0.14)
            },
            'window.open': {
                notes: [{ v: 'pulse', n: 7, t: 0, g: 0.9, pan: -0.15, decay: 0.16 }, { v: 'ping', n: 12, t: 0.085, g: 1, pan: 0.2, t60: 0.2 }],
                layers: [{ v: 'static', t: 0, g: 0.06 }],
                echo: echo(0.1, 0.15, 0.12),
                space: plate(0.28, 0.16)
            },
            'window.close': {
                notes: [{ v: 'ping', n: 12, t: 0, g: 0.9, pan: 0.2, t60: 0.16 }, { v: 'pulse', n: 7, t: 0.085, g: 1, pan: -0.15, decay: 0.16 }],
                layers: [{ v: 'static', t: 0.085, g: 0.06 }],
                echo: echo(0.1, 0.12, 0.1),
                space: plate(0.22, 0.14)
            },
            'window.maximize': {
                notes: [{ v: 'pulse', n: 3, t: 0, g: 0.85, pan: -0.2, decay: 0.2 }, { v: 'ping', n: 15, t: 0.1, g: 1, pan: 0.2, t60: 0.2 }, { v: 'ping', n: 22, t: 0.18, g: 0.5, pan: 0.2, t60: 0.15 }],
                layers: [{ v: 'static', t: 0.1, g: 0.06 }],
                echo: echo(0.09, 0.12, 0.12),
                space: plate(0.25, 0.22)
            },
            'window.minimize': {
                notes: [{ v: 'ping', n: 10, t: 0, g: 0.9, pan: 0.15, t60: 0.1 }, { v: 'pulse', n: 3, t: 0.065, g: 1, pan: -0.15, decay: 0.12 }],
                space: plate(0.2, 0.12)
            },
            'window.restore': {
                notes: [{ v: 'pulse', n: 3, t: 0, g: 0.9, pan: -0.15, decay: 0.12 }, { v: 'ping', n: 10, t: 0.065, g: 1, pan: 0.15, t60: 0.1 }],
                space: plate(0.2, 0.12)
            },
            'window.snap': {
                layers: [{ v: 'click', t: 0, g: 1, tone: 3200, ring: 0.012 }, { v: 'static', t: 0, g: 0.05 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'dull', n: 3, t: 0, g: 0.9, pan: 0, attack: 0.007, decay: 0.12, cutoff: 0.8 }],
                layers: [{ v: 'knock', t: 0, g: 0.5 }, { v: 'click', t: 0, g: 0.7, tone: 900, ring: 0.02 }],
                space: { size: 0.5, rt60: 0.2, damping: 4000, predelay: 0.004, mix: 0.12 }
            },
            'file.drop': {
                notes: [{ v: 'dull', n: 3, t: 0, g: 0.8, pan: 0, attack: 0.007, decay: 0.18, cutoff: 0.8 }],
                layers: [{ v: 'knock', t: 0, g: 0.5 }],
                space: { size: 0.7, rt60: 0.35, damping: 4500, predelay: 0.006, mix: 0.18 }
            },
            'file.trash': {
                notes: [{ v: 'ping', n: 15, t: 0, g: 0.8, pan: 0.15, t60: 0.08 }, { v: 'ping', n: 10, t: 0.05, g: 0.9, pan: 0, t60: 0.08 },
                    { v: 'pulse', n: 5, t: 0.1, g: 1, pan: -0.15, decay: 0.14 }],
                layers: [{ v: 'hiss', t: 0, g: 0.1, attack: 0.02, decay: 0.12 }],
                space: plate(0.3, 0.14)
            },
            'file.delete': {
                notes: [{ v: 'ping', n: 8, t: 0, g: 0.85, pan: 0.1, t60: 0.15 }, { v: 'pulse', n: 0, t: 0.09, g: 1, pan: -0.1, decay: 0.3, cutoff: 0.9 }],
                layers: [{ v: 'knock', t: 0.09, g: 0.4 }, { v: 'static', t: 0, g: 0.05 }],
                echo: echo(0.1, 0.15, 0.12),
                space: plate(0.4, 0.18)
            },
            'space.switch': {
                notes: [{ v: 'pulse', n: 12, t: 0.03, g: 0.6, pan: 0, attack: 0.12, decay: 0.12, cutoff: 0.8 }],
                layers: [{ v: 'hiss', t: 0, g: 0.8, attack: 0.12, decay: 0.12, band: [900, 4000] }, { v: 'static', t: 0.15, g: 0.05 }],
                echo: echo(0.1, 0.12, 0.1),
                space: plate(0.25, 0.18)
            },
            'menu.open': {
                notes: [{ v: 'pulse', n: 7, t: 0, g: 0.9, pan: -0.1, decay: 0.07, dur: 0.085 }, { v: 'ping', n: 12, t: 0.035, g: 1, pan: 0.1, t60: 0.07, dur: 0.085 }],
                space: { size: 0.4, rt60: 0.1, damping: 5000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'ping', n: 12, t: 0, g: 0.9, pan: 0.1, t60: 0.07, dur: 0.085 }, { v: 'pulse', n: 7, t: 0.03, g: 1, pan: -0.1, decay: 0.07, dur: 0.085 }],
                space: { size: 0.4, rt60: 0.07, damping: 5000, predelay: 0.002, mix: 0.06 }
            },
            'ui.select': {
                layers: [{ v: 'click', t: 0, g: 1, tone: 2400, ring: 0.02 }, { v: 'static', t: 0, g: 0.04 }],
                space: { size: 0.4, rt60: 0.08, damping: 5500, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'click', t: 0, g: 0.9, tone: 2400, ring: 0.015 }, { v: 'click', t: 0.04, g: 1, tone: 3400, ring: 0.015 }, { v: 'hiss', t: 0, g: 0.05, attack: 0.005, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.1, damping: 5500, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.off': {
                layers: [{ v: 'click', t: 0, g: 0.9, tone: 3400, ring: 0.015 }, { v: 'click', t: 0.04, g: 1, tone: 2400, ring: 0.015 }, { v: 'hiss', t: 0, g: 0.05, attack: 0.005, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.1, damping: 5500, predelay: 0.002, mix: 0.08 }
            },
            'ui.tick': {
                layers: [{ v: 'click', t: 0, g: 1, tone: 3600, ring: 0.008, length: 0.0015 }],
                space: null
            }
        },
        fx: {}
    });
})();
