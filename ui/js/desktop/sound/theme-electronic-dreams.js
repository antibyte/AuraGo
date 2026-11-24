// Electronic Dreams: an amber signal arpeggio on tape. A soft tine bell runs fast up (or down) the minor seventh chord
// 0 3 7 10 (A minor, key A3 = 57), copper clicks mark the first note and a dark tape echo repeats the figure quietly.
// A slow, low, mellow synth bed sits under notices only. Touch sounds are copper clicks.
(function () {
    'use strict';
    const tape = (time, feedback, mix) => ({ time, feedback, tone: 2800, mix, spread: 0.5 });
    const plate = (rt60, mix) => ({ size: 0.9, rt60, damping: 5500, predelay: 0.012, mix });
    const room = { size: 0.4, rt60: 0.1, damping: 6000, predelay: 0.002, mix: 0.1 };
    // An arpeggio: notes n at a fixed step, panned left to right.
    const arp = (v, ns, step, g, extra) => ns.map((n, i) => Object.assign({ v, n, t: i * step, g: g * (0.8 + 0.2 * (i + 1) / ns.length), pan: -0.25 + 0.5 * i / Math.max(1, ns.length - 1) }, extra));
    self.DesktopSoundEngine.defineTheme({
        id: 'electronic_dreams',
        key: 57,
        space: { size: 0.9, rt60: 0.7, damping: 5500, predelay: 0.012, mix: 0.22 },
        voices: {
            amber: { model: 'bell', kind: 'tine', t60: 0.32, index: 0.4, attack: 0.006, spread: 0.3 },
            amberLow: { model: 'bell', kind: 'celesta', t60: 0.3, index: 0.3, attack: 0.006, spread: 0.2 },
            bed: { model: 'analog', wave: 'saw', unison: 3, detune: 12, drift: 5, sub: 0, cutoff: 1.1, env: 0.8, envDecay: 0.4, q: 0.8, drive: 0.5, attack: 0.12, decay: 0.5, width: 0.7 },
            dull: { model: 'analog', wave: 'pulse', pw: 0.5, unison: 2, detune: 8, sub: 0, cutoff: 1, env: 2, envDecay: 0.09, q: 1, drive: 0.6, attack: 0.008, decay: 0.3, width: 0.4 },
            click: { model: 'click', material: 'copper', tone: 3000, ring: 0.03, noise: 0.5 },
            air: { model: 'noise', color: 'pink', band: [1200, 5000], attack: 0.05, decay: 0.2 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'bed', n: -5, t: 0, g: 0.9, pan: -0.1, attack: 0.25, decay: 1.4 }, { v: 'bed', n: 0, t: 0.1, g: 0.7, pan: 0.1, attack: 0.3, decay: 1.4 }]
                    .concat(arp('amber', [12, 15, 19, 22], 0.07, 0.8, { t60: 0.5 }))
                    .concat(arp('amber', [19, 22, 24, 27], 0.07, 0.8, { t60: 0.9 }).map(x => Object.assign(x, { t: x.t + 0.5 }))),
                layers: [{ v: 'click', t: 0, g: 0.3 }, { v: 'air', t: 0, g: 0.05 }],
                echo: tape(0.24, 0.3, 0.3),
                space: { size: 1.1, rt60: 1.3, damping: 5000, predelay: 0.02, mix: 0.3 }
            },
            'notify.message': {
                notes: [{ v: 'bed', n: 0, t: 0, g: 0.8, pan: 0, attack: 0.15, decay: 0.7 }].concat(arp('amber', [12, 15, 19, 22], 0.065, 1, { t60: 0.4 })),
                layers: [{ v: 'click', t: 0, g: 0.3 }],
                echo: tape(0.2, 0.28, 0.24),
                space: plate(0.6, 0.22)
            },
            'notify.info': {
                notes: [{ v: 'bed', n: 3, t: 0, g: 0.7, pan: 0, attack: 0.12, decay: 0.5 }].concat(arp('amber', [15, 19, 22], 0.07, 1, { t60: 0.35 })),
                layers: [{ v: 'click', t: 0, g: 0.3 }],
                echo: tape(0.18, 0.26, 0.2),
                space: plate(0.6, 0.2)
            },
            'notify.success': {
                notes: [{ v: 'bed', n: 7, t: 0, g: 0.7, pan: 0, attack: 0.12, decay: 0.5 }].concat(arp('amber', [7, 12, 15, 19], 0.06, 1, { t60: 0.35 })),
                layers: [{ v: 'click', t: 0, g: 0.3 }],
                echo: tape(0.16, 0.22, 0.18),
                space: plate(0.5, 0.2)
            },
            'notify.warning': {
                notes: [{ v: 'bed', n: 3, t: 0, g: 0.8, pan: 0, attack: 0.1, decay: 0.45 }, { v: 'amber', n: 10, t: 0, g: 1, pan: -0.15, t60: 0.3 }, { v: 'amber', n: 10, t: 0.2, g: 0.85, pan: 0.15, t60: 0.3 }],
                layers: [{ v: 'click', t: 0, g: 0.3 }],
                echo: tape(0.2, 0.2, 0.14),
                space: plate(0.4, 0.18)
            },
            'notify.error': {
                notes: [{ v: 'bed', n: -2, t: 0, g: 0.9, pan: 0, attack: 0.04, decay: 0.45, cutoff: 0.9 }, { v: 'amberLow', n: 12, t: 0, g: 1, pan: -0.1, t60: 0.3 }, { v: 'amberLow', n: 6, t: 0.16, g: 0.9, pan: 0.1, t60: 0.35 }],
                layers: [{ v: 'knock', t: 0, g: 0.4 }, { v: 'knock', t: 0.16, g: 0.35 }],
                echo: tape(0.18, 0.2, 0.14),
                space: plate(0.45, 0.18)
            },
            'dialog.open': {
                notes: arp('amber', [12, 15, 19], 0.06, 1, { t60: 0.14 }),
                layers: [{ v: 'click', t: 0, g: 0.3 }],
                echo: tape(0.14, 0.03, 0.12),
                space: plate(0.2, 0.16)
            },
            'dialog.confirm': {
                notes: arp('amber', [12, 15, 19, 24], 0.055, 1, { t60: 0.18 }),
                layers: [{ v: 'click', t: 0, g: 0.3 }],
                echo: tape(0.16, 0.03, 0.12),
                space: plate(0.22, 0.16)
            },
            'dialog.cancel': {
                notes: arp('amber', [24, 19, 15, 12], 0.055, 1, { t60: 0.1 }),
                layers: [{ v: 'click', t: 0.165, g: 0.3 }],
                echo: tape(0.14, 0, 0.08),
                space: plate(0.15, 0.12)
            },
            'window.open': {
                notes: arp('amber', [12, 15, 19], 0.06, 1, { t60: 0.12 }),
                layers: [{ v: 'click', t: 0, g: 0.3 }],
                echo: tape(0.14, 0, 0.08),
                space: plate(0.18, 0.14)
            },
            'window.close': {
                notes: arp('amber', [19, 15, 12], 0.06, 1, { t60: 0.18 }),
                layers: [{ v: 'click', t: 0.12, g: 0.3 }],
                space: plate(0.22, 0.14)
            },
            'window.maximize': {
                notes: arp('amber', [7, 12, 15, 19], 0.06, 1, { t60: 0.12 }),
                layers: [{ v: 'click', t: 0, g: 0.3 }],
                echo: tape(0.14, 0, 0.1),
                space: plate(0.2, 0.2)
            },
            'window.minimize': {
                notes: arp('amber', [22, 19, 15], 0.05, 1, { t60: 0.12 }),
                space: plate(0.2, 0.12)
            },
            'window.restore': {
                notes: arp('amber', [15, 19, 22], 0.05, 1, { t60: 0.12 }),
                space: plate(0.2, 0.12)
            },
            'window.snap': {
                layers: [{ v: 'click', t: 0, g: 1, tone: 2800, ring: 0.015 }],
                space: room
            },
            'window.deny': {
                notes: [{ v: 'dull', n: 3, t: 0, g: 0.9, pan: 0, attack: 0.007, decay: 0.12, cutoff: 0.8 }],
                layers: [{ v: 'click', t: 0, g: 0.8, tone: 1400, ring: 0.04 }, { v: 'click', t: 0.05, g: 0.6, tone: 1100, ring: 0.04 }, { v: 'knock', t: 0, g: 0.5 }],
                space: { size: 0.5, rt60: 0.2, damping: 4500, predelay: 0.004, mix: 0.12 }
            },
            'file.drop': {
                notes: [{ v: 'amberLow', n: 3, t: 0.02, g: 0.9, pan: 0, t60: 0.28, index: 0.2 }, { v: 'dull', n: 3, t: 0, g: 0.6, pan: 0, attack: 0.007, decay: 0.12, cutoff: 0.8 }],
                layers: [{ v: 'click', t: 0, g: 0.9, tone: 1600, ring: 0.04 }, { v: 'knock', t: 0, g: 0.4 }],
                space: { size: 0.7, rt60: 0.35, damping: 5000, predelay: 0.006, mix: 0.18 }
            },
            'file.trash': {
                notes: arp('amber', [22, 19, 15], 0.045, 1, { t60: 0.12 }),
                layers: [{ v: 'air', t: 0, g: 0.1, attack: 0.02, decay: 0.12 }, { v: 'click', t: 0, g: 0.3 }],
                space: plate(0.3, 0.14)
            },
            'file.delete': {
                notes: [{ v: 'amber', n: 10, t: 0, g: 0.85, pan: 0.1, t60: 0.15 }, { v: 'amberLow', n: 3, t: 0.07, g: 1, pan: -0.1, t60: 0.15 }, { v: 'amberLow', n: 0, t: 0.14, g: 1, pan: 0, t60: 0.18 }],
                layers: [{ v: 'knock', t: 0.14, g: 0.4 }],
                echo: tape(0.14, 0, 0.1),
                space: plate(0.25, 0.16)
            },
            'space.switch': {
                notes: [{ v: 'bed', n: 7, t: 0.03, g: 0.7, pan: 0, attack: 0.12, decay: 0.2 }],
                layers: [{ v: 'air', t: 0, g: 0.8, attack: 0.12, decay: 0.12, band: [1200, 4500] }, { v: 'click', t: 0.18, g: 0.3 }],
                echo: tape(0.16, 0.08, 0.08),
                space: plate(0.18, 0.16)
            },
            'menu.open': {
                notes: [{ v: 'amber', n: 19, t: 0.0, g: 0.9, pan: -0.1, t60: 0.07, dur: 0.085 }, { v: 'amber', n: 24, t: 0.035, g: 1, pan: 0.1, t60: 0.07, dur: 0.085 }],
                layers: [{ v: 'click', t: 0, g: 0.5 }],
                space: room
            },
            'menu.close': {
                notes: [{ v: 'amber', n: 24, t: 0, g: 0.9, pan: 0.1, t60: 0.07, dur: 0.085 }, { v: 'amber', n: 19, t: 0.03, g: 1, pan: -0.1, t60: 0.07, dur: 0.085 }],
                layers: [{ v: 'click', t: 0.03, g: 0.5 }],
                space: room
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
