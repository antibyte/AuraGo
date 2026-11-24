// Electronic Dreams: copper clicks, mellow lowpassed saw-pulse notes and a small amber celesta arpeggio under a tape echo.
// Key A3 (57), A minor seventh: 0 3 7 10 with passing notes 2 5 8. Slower and darker than the analog theme.
(function () {
    'use strict';
    const tape = (time, feedback, mix) => ({ time, feedback, tone: 3200, mix, spread: 0.5 });
    const plate = (rt60, mix) => ({ size: 0.9, rt60, damping: 6000, predelay: 0.012, mix });
    self.DesktopSoundEngine.defineTheme({
        id: 'electronic_dreams',
        key: 57,
        space: { size: 0.9, rt60: 0.7, damping: 6000, predelay: 0.012, mix: 0.22 },
        voices: {
            mellow: { model: 'analog', wave: 'saw', unison: 4, detune: 18, drift: 5, sub: 0, cutoff: 0.9, env: 6, envDecay: 0.12, q: 1.8, drive: 1.4, attack: 0.008, decay: 0.3, width: 0.7 },
            dull: { model: 'analog', wave: 'pulse', pw: 0.5, unison: 2, detune: 8, sub: 0, cutoff: 1, env: 3, envDecay: 0.09, q: 1.2, drive: 0.8, attack: 0.008, decay: 0.3, width: 0.4 },
            amber: { model: 'bell', kind: 'celesta', t60: 0.5, index: 0.8, attack: 0.004, spread: 0.3 },
            swell: { model: 'analog', wave: 'pulse', pw: 0.5, unison: 3, detune: 14, sub: 0, cutoff: 0.8, env: 2, envDecay: 0.3, q: 1.4, drive: 0.8, attack: 0.14, decay: 0.3, width: 0.6 },
            click: { model: 'click', material: 'copper', tone: 3000, ring: 0.03, noise: 0.5 },
            air: { model: 'noise', color: 'pink', band: [1200, 5000], attack: 0.05, decay: 0.2 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'mellow', n: 7, t: 0, g: 0.8, pan: -0.25, decay: 0.5 }, { v: 'mellow', n: 12, t: 0.2, g: 0.85, pan: 0.1, decay: 0.55 },
                    { v: 'amber', n: 15, t: 0.4, g: 0.8, pan: -0.1, t60: 0.6 }, { v: 'amber', n: 19, t: 0.6, g: 0.85, pan: 0.2, t60: 0.7 },
                    { v: 'mellow', n: 24, t: 0.84, g: 0.7, pan: 0, decay: 0.7 }, { v: 'amber', n: 24, t: 0.84, g: 0.6, pan: 0.1, t60: 0.9 }],
                layers: [{ v: 'air', t: 0, g: 0.05 }],
                echo: tape(0.22, 0.38, 0.3),
                space: { size: 1.1, rt60: 1.3, damping: 5500, predelay: 0.02, mix: 0.3 }
            },
            'notify.message': {
                notes: [{ v: 'mellow', n: 12, t: 0, g: 0.8, pan: -0.2, decay: 0.35 }, { v: 'mellow', n: 15, t: 0.13, g: 0.85, pan: 0.15, decay: 0.38 },
                    { v: 'mellow', n: 19, t: 0.26, g: 0.9, pan: 0, decay: 0.45 }, { v: 'amber', n: 19, t: 0.26, g: 0.5, pan: 0.1, t60: 0.5 }],
                echo: tape(0.16, 0.22, 0.2),
                space: plate(0.6, 0.22)
            },
            'notify.info': {
                notes: [{ v: 'mellow', n: 12, t: 0, g: 0.85, pan: -0.15, decay: 0.25 }, { v: 'amber', n: 19, t: 0.12, g: 1, pan: 0.15, t60: 0.4 }],
                echo: tape(0.18, 0.28, 0.22),
                space: plate(0.7, 0.22)
            },
            'notify.success': {
                notes: [{ v: 'mellow', n: 3, t: 0, g: 0.8, pan: -0.2, decay: 0.25 }, { v: 'mellow', n: 7, t: 0.11, g: 0.9, pan: 0.1, decay: 0.28 },
                    { v: 'mellow', n: 12, t: 0.22, g: 1, pan: 0, decay: 0.4 }, { v: 'amber', n: 12, t: 0.22, g: 0.5, pan: 0.1, t60: 0.5 }],
                echo: tape(0.16, 0.2, 0.18),
                space: plate(0.5, 0.2)
            },
            'notify.warning': {
                notes: [{ v: 'mellow', n: 10, t: 0, g: 1, pan: -0.1, decay: 0.2, cutoff: 0.9 }, { v: 'mellow', n: 10, t: 0.2, g: 0.85, pan: 0.1, decay: 0.26, cutoff: 0.9 }],
                echo: tape(0.18, 0.2, 0.16),
                space: plate(0.4, 0.18)
            },
            'notify.error': {
                notes: [{ v: 'mellow', n: 12, t: 0, g: 1, pan: -0.1, attack: 0.01, decay: 0.28, cutoff: 0.8 }, { v: 'mellow', n: 6, t: 0.16, g: 0.9, pan: 0.1, attack: 0.01, decay: 0.34, cutoff: 0.8 }],
                layers: [{ v: 'knock', t: 0, g: 0.4 }, { v: 'knock', t: 0.16, g: 0.35 }],
                echo: tape(0.16, 0.2, 0.16),
                space: plate(0.5, 0.18)
            },
            'dialog.open': {
                notes: [{ v: 'mellow', n: 7, t: 0, g: 0.85, pan: -0.15, decay: 0.2 }, { v: 'amber', n: 10, t: 0.08, g: 1, pan: 0.15, t60: 0.25 }],
                echo: tape(0.1, 0.15, 0.14),
                space: plate(0.3, 0.18)
            },
            'dialog.confirm': {
                notes: [{ v: 'mellow', n: 10, t: 0, g: 0.85, pan: -0.15, decay: 0.2 }, { v: 'amber', n: 15, t: 0.09, g: 1, pan: 0.15, t60: 0.3 }],
                echo: tape(0.1, 0.15, 0.14),
                space: plate(0.3, 0.18)
            },
            'dialog.cancel': {
                notes: [{ v: 'amber', n: 15, t: 0, g: 0.85, pan: 0.15, t60: 0.2 }, { v: 'mellow', n: 10, t: 0.09, g: 1, pan: -0.15, decay: 0.18 }],
                echo: tape(0.1, 0.15, 0.12),
                space: plate(0.35, 0.16)
            },
            'window.open': {
                notes: [{ v: 'mellow', n: 7, t: 0, g: 0.9, pan: -0.15, decay: 0.18 }, { v: 'mellow', n: 12, t: 0.085, g: 1, pan: 0.2, decay: 0.2 }],
                layers: [{ v: 'click', t: 0, g: 0.3 }],
                echo: tape(0.1, 0.18, 0.14),
                space: plate(0.3, 0.16)
            },
            'window.close': {
                notes: [{ v: 'mellow', n: 12, t: 0, g: 0.9, pan: 0.2, decay: 0.16 }, { v: 'mellow', n: 7, t: 0.085, g: 1, pan: -0.15, decay: 0.18 }],
                layers: [{ v: 'click', t: 0.085, g: 0.3 }],
                echo: tape(0.1, 0.15, 0.12),
                space: plate(0.25, 0.14)
            },
            'window.maximize': {
                notes: [{ v: 'mellow', n: 3, t: 0, g: 0.85, pan: -0.2, decay: 0.2 }, { v: 'mellow', n: 15, t: 0.1, g: 1, pan: 0.2, decay: 0.24 }, { v: 'amber', n: 22, t: 0.18, g: 0.5, pan: 0.2, t60: 0.2 }],
                echo: tape(0.09, 0.12, 0.12),
                space: plate(0.25, 0.22)
            },
            'window.minimize': {
                notes: [{ v: 'mellow', n: 10, t: 0, g: 0.9, pan: 0.15, decay: 0.12 }, { v: 'mellow', n: 3, t: 0.065, g: 1, pan: -0.15, decay: 0.14 }],
                space: plate(0.2, 0.12)
            },
            'window.restore': {
                notes: [{ v: 'mellow', n: 3, t: 0, g: 0.9, pan: -0.15, decay: 0.12 }, { v: 'mellow', n: 10, t: 0.065, g: 1, pan: 0.15, decay: 0.14 }],
                space: plate(0.2, 0.12)
            },
            'window.snap': {
                layers: [{ v: 'click', t: 0, g: 1, tone: 2800, ring: 0.015 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'dull', n: 3, t: 0, g: 0.9, pan: 0, attack: 0.007, decay: 0.12, cutoff: 0.8 }],
                layers: [{ v: 'knock', t: 0, g: 0.6 }],
                space: { size: 0.5, rt60: 0.2, damping: 4500, predelay: 0.004, mix: 0.12 }
            },
            'file.drop': {
                notes: [{ v: 'dull', n: 3, t: 0, g: 0.9, pan: 0, attack: 0.007, decay: 0.2, cutoff: 0.9 }],
                layers: [{ v: 'knock', t: 0, g: 0.5 }],
                space: { size: 0.7, rt60: 0.35, damping: 5000, predelay: 0.006, mix: 0.18 }
            },
            'file.trash': {
                notes: [{ v: 'mellow', n: 15, t: 0, g: 0.8, pan: 0.15, decay: 0.1 }, { v: 'mellow', n: 10, t: 0.05, g: 0.9, pan: 0, decay: 0.1 },
                    { v: 'mellow', n: 7, t: 0.1, g: 1, pan: -0.15, decay: 0.14 }],
                layers: [{ v: 'air', t: 0, g: 0.1, attack: 0.02, decay: 0.12 }],
                space: plate(0.3, 0.14)
            },
            'file.delete': {
                notes: [{ v: 'mellow', n: 7, t: 0, g: 0.85, pan: 0.1, decay: 0.18 }, { v: 'mellow', n: 0, t: 0.09, g: 1, pan: -0.1, decay: 0.3, cutoff: 0.9 }],
                layers: [{ v: 'knock', t: 0.09, g: 0.4 }],
                echo: tape(0.1, 0.15, 0.12),
                space: plate(0.45, 0.18)
            },
            'space.switch': {
                notes: [{ v: 'swell', n: 12, t: 0.03, g: 0.7, pan: 0, decay: 0.2 }],
                layers: [{ v: 'air', t: 0, g: 0.8, attack: 0.12, decay: 0.12, band: [1200, 4500] }],
                echo: tape(0.1, 0.12, 0.1),
                space: plate(0.25, 0.18)
            },
            'menu.open': {
                notes: [{ v: 'mellow', n: 7, t: 0, g: 0.9, pan: -0.1, decay: 0.07, dur: 0.085 }, { v: 'mellow', n: 12, t: 0.035, g: 1, pan: 0.1, decay: 0.07, dur: 0.085 }],
                space: { size: 0.4, rt60: 0.1, damping: 6000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'mellow', n: 12, t: 0, g: 0.9, pan: 0.1, decay: 0.07, dur: 0.085 }, { v: 'amber', n: 7, t: 0.03, g: 1, pan: -0.1, t60: 0.1, dur: 0.085 }],
                space: { size: 0.4, rt60: 0.1, damping: 6000, predelay: 0.002, mix: 0.1 }
            },
            'ui.select': {
                layers: [{ v: 'click', t: 0, g: 1, tone: 2200, ring: 0.025 }],
                space: { size: 0.4, rt60: 0.08, damping: 6500, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'click', t: 0, g: 0.9, tone: 2400, ring: 0.02 }, { v: 'click', t: 0.04, g: 1, tone: 3200, ring: 0.02 }, { v: 'air', t: 0, g: 0.05, attack: 0.005, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.1, damping: 6500, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.off': {
                layers: [{ v: 'click', t: 0, g: 0.9, tone: 3200, ring: 0.02 }, { v: 'click', t: 0.04, g: 1, tone: 2400, ring: 0.02 }, { v: 'air', t: 0, g: 0.05, attack: 0.005, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.1, damping: 6500, predelay: 0.002, mix: 0.08 }
            },
            'ui.tick': {
                layers: [{ v: 'click', t: 0, g: 1, tone: 3400, ring: 0.01, length: 0.0015 }],
                space: null
            }
        },
        fx: {}
    });
})();
