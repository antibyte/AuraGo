// Analog: warm detuned saw plucks with a sub, chorus and a short plate. Key D4 (62), D Dorian: 0 2 3 5 7 9 10.
// Frequent events (window, menu, touch) are short, quiet plucks and ticks; notices add chorus and a longer plate.
(function () {
    'use strict';
    const chorus = { rate: 0.55, depth: 0.003, delay: 0.015, mix: 0.3 };
    const plate = (rt60, mix) => ({ size: 0.8, rt60, damping: 8000, predelay: 0.008, mix });
    self.DesktopSoundEngine.defineTheme({
        id: 'analog',
        key: 62,
        space: { size: 0.8, rt60: 0.6, damping: 8000, predelay: 0.008, mix: 0.22 },
        voices: {
            pluck: { model: 'analog', wave: 'saw', unison: 5, detune: 20, drift: 4, sub: 0.3, cutoff: 0.9, env: 7, envDecay: 0.14, q: 2.2, drive: 1.5, attack: 0.004, decay: 0.28, width: 0.8 },
            blip: { model: 'analog', wave: 'saw', unison: 4, detune: 16, drift: 4, sub: 0.2, cutoff: 0.9, env: 5.5, envDecay: 0.08, q: 2, drive: 1.2, attack: 0.004, decay: 0.12, width: 0.7 },
            dull: { model: 'analog', wave: 'pulse', pw: 0.35, unison: 2, detune: 8, sub: 0, cutoff: 0.9, env: 2, envDecay: 0.05, q: 1, drive: 0.8, attack: 0.007, decay: 0.16, width: 0.3 },
            sweep: { model: 'analog', wave: 'saw', unison: 5, detune: 20, sub: 0.2, cutoff: 0.8, env: 4, envDecay: 0.3, q: 2, drive: 1, attack: 0.1, decay: 0.3, width: 0.7 },
            tick: { model: 'click', material: 'plastic', tone: 2400, ring: 0.02 },
            air: { model: 'noise', color: 'pink', band: [1500, 6000], attack: 0.05, decay: 0.2 },
            knock: { model: 'noise', color: 'pink', band: [120, 800], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'pluck', n: 7, t: 0, g: 0.8, pan: -0.25, decay: 0.5 }, { v: 'pluck', n: 12, t: 0.17, g: 0.85, pan: 0.1, decay: 0.55 },
                    { v: 'pluck', n: 15, t: 0.34, g: 0.9, pan: -0.1, decay: 0.6 }, { v: 'pluck', n: 19, t: 0.52, g: 0.9, pan: 0.2, decay: 0.7 },
                    { v: 'sweep', n: 12, t: 0.78, g: 0.55, pan: 0, decay: 0.8, attack: 0.12 }],
                layers: [{ v: 'air', t: 0, g: 0.05 }],
                chorus,
                space: { size: 1.1, rt60: 1.3, damping: 6500, predelay: 0.02, mix: 0.3 }
            },
            'notify.message': {
                notes: [{ v: 'pluck', n: 7, t: 0, g: 0.8, pan: -0.2, decay: 0.4 }, { v: 'pluck', n: 12, t: 0.11, g: 0.9, pan: 0.15, decay: 0.45 },
                    { v: 'pluck', n: 15, t: 0.22, g: 1, pan: 0, decay: 0.55 }],
                chorus,
                space: plate(0.8, 0.26)
            },
            'notify.info': {
                notes: [{ v: 'pluck', n: 12, t: 0, g: 0.85, pan: -0.15, decay: 0.3 }, { v: 'pluck', n: 17, t: 0.1, g: 1, pan: 0.15, decay: 0.4 }],
                chorus,
                space: plate(0.7, 0.24)
            },
            'notify.success': {
                notes: [{ v: 'pluck', n: 5, t: 0, g: 0.8, pan: -0.2, decay: 0.3 }, { v: 'pluck', n: 9, t: 0.09, g: 0.9, pan: 0.1, decay: 0.3 },
                    { v: 'pluck', n: 12, t: 0.18, g: 1, pan: 0, decay: 0.45 }],
                chorus,
                space: plate(0.7, 0.26)
            },
            'notify.warning': {
                notes: [{ v: 'dull', n: 10, t: 0, g: 1, pan: -0.1, decay: 0.2 }, { v: 'dull', n: 10, t: 0.17, g: 0.85, pan: 0.1, decay: 0.26 }],
                chorus,
                space: plate(0.6, 0.22)
            },
            'notify.error': {
                notes: [{ v: 'dull', n: 9, t: 0, g: 1, pan: -0.1, attack: 0.009, decay: 0.28 }, { v: 'dull', n: 3, t: 0.15, g: 0.9, pan: 0.1, attack: 0.009, decay: 0.34 }],
                layers: [{ v: 'knock', t: 0, g: 0.4 }, { v: 'knock', t: 0.15, g: 0.35 }],
                space: plate(0.5, 0.18)
            },
            'dialog.open': {
                notes: [{ v: 'pluck', n: 3, t: 0, g: 0.85, pan: -0.15, decay: 0.2 }, { v: 'pluck', n: 7, t: 0.08, g: 1, pan: 0.15, decay: 0.24 }],
                chorus,
                space: plate(0.45, 0.2)
            },
            'dialog.confirm': {
                notes: [{ v: 'pluck', n: 7, t: 0, g: 0.85, pan: -0.15, decay: 0.2 }, { v: 'pluck', n: 12, t: 0.09, g: 1, pan: 0.15, decay: 0.28 }],
                chorus,
                space: plate(0.5, 0.2)
            },
            'dialog.cancel': {
                notes: [{ v: 'pluck', n: 12, t: 0, g: 0.85, pan: 0.15, decay: 0.18 }, { v: 'pluck', n: 7, t: 0.09, g: 1, pan: -0.15, decay: 0.2 }],
                chorus,
                space: plate(0.35, 0.16)
            },
            'window.open': {
                notes: [{ v: 'blip', n: 7, t: 0, g: 0.9, pan: -0.15, decay: 0.2 }, { v: 'blip', n: 12, t: 0.085, g: 1, pan: 0.2, decay: 0.22 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.1 }],
                chorus,
                space: plate(0.35, 0.18)
            },
            'window.close': {
                notes: [{ v: 'blip', n: 12, t: 0, g: 0.9, pan: 0.2, decay: 0.18 }, { v: 'blip', n: 7, t: 0.085, g: 1, pan: -0.15, decay: 0.2 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.1 }],
                chorus,
                space: plate(0.3, 0.16)
            },
            'window.maximize': {
                notes: [{ v: 'blip', n: 0, t: 0, g: 0.85, pan: -0.2, decay: 0.22 }, { v: 'blip', n: 12, t: 0.1, g: 1, pan: 0.2, decay: 0.26 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.12 }],
                chorus,
                space: plate(0.45, 0.26)
            },
            'window.minimize': {
                notes: [{ v: 'blip', n: 10, t: 0, g: 0.9, pan: 0.15, decay: 0.13 }, { v: 'blip', n: 3, t: 0.065, g: 1, pan: -0.15, decay: 0.15 }],
                space: plate(0.25, 0.14)
            },
            'window.restore': {
                notes: [{ v: 'blip', n: 3, t: 0, g: 0.9, pan: -0.15, decay: 0.13 }, { v: 'blip', n: 10, t: 0.065, g: 1, pan: 0.15, decay: 0.15 }],
                space: plate(0.25, 0.14)
            },
            'window.snap': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 2800, ring: 0.015 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'dull', n: 0, t: 0, g: 0.9, pan: 0, attack: 0.007, decay: 0.12, cutoff: 0.8 }],
                layers: [{ v: 'knock', t: 0, g: 0.7, band: [250, 1000] }],
                space: { size: 0.5, rt60: 0.2, damping: 4500, predelay: 0.004, mix: 0.12 }
            },
            'file.drop': {
                notes: [{ v: 'dull', n: -2, t: 0, g: 0.9, pan: 0, attack: 0.007, decay: 0.2, cutoff: 0.9 }],
                layers: [{ v: 'knock', t: 0, g: 0.5, band: [250, 1000] }],
                space: { size: 0.7, rt60: 0.35, damping: 5000, predelay: 0.006, mix: 0.18 }
            },
            'file.trash': {
                notes: [{ v: 'blip', n: 12, t: 0, g: 0.8, pan: 0.15, decay: 0.1 }, { v: 'blip', n: 7, t: 0.05, g: 0.9, pan: 0, decay: 0.1 },
                    { v: 'blip', n: 3, t: 0.1, g: 1, pan: -0.15, decay: 0.14 }],
                layers: [{ v: 'air', t: 0, g: 0.1, attack: 0.02, decay: 0.12 }],
                space: plate(0.3, 0.14)
            },
            'file.delete': {
                notes: [{ v: 'blip', n: 5, t: 0, g: 0.85, pan: 0.1, decay: 0.18 }, { v: 'dull', n: -2, t: 0.09, g: 1, pan: -0.1, decay: 0.28 }],
                layers: [{ v: 'knock', t: 0.09, g: 0.4 }],
                space: plate(0.45, 0.18)
            },
            'space.switch': {
                notes: [{ v: 'sweep', n: 7, t: 0.03, g: 0.7, pan: 0, attack: 0.14, decay: 0.2 }],
                layers: [{ v: 'air', t: 0, g: 0.8, attack: 0.15, decay: 0.2, band: [1500, 5000] }],
                chorus,
                space: plate(0.4, 0.2)
            },
            'menu.open': {
                notes: [{ v: 'blip', n: 7, t: 0, g: 0.9, pan: -0.1, decay: 0.07, dur: 0.085 }, { v: 'blip', n: 12, t: 0.035, g: 1, pan: 0.1, decay: 0.07, dur: 0.085 }],
                space: { size: 0.4, rt60: 0.1, damping: 6000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'blip', n: 12, t: 0, g: 0.9, pan: 0.1, decay: 0.07, dur: 0.085 }, { v: 'blip', n: 7, t: 0.03, g: 1, pan: -0.1, decay: 0.07, dur: 0.085 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.select': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 1800, ring: 0.025, material: 'felt' }],
                space: { size: 0.4, rt60: 0.08, damping: 6500, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'tick', t: 0, g: 0.9, tone: 2200, ring: 0.018, material: 'felt' }, { v: 'tick', t: 0.04, g: 1, tone: 3000, ring: 0.018 }, { v: 'air', t: 0, g: 0.05, attack: 0.005, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.1, damping: 6500, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.off': {
                layers: [{ v: 'tick', t: 0, g: 0.9, tone: 3000, ring: 0.018 }, { v: 'tick', t: 0.04, g: 1, tone: 2200, ring: 0.018, material: 'felt' }, { v: 'air', t: 0, g: 0.05, attack: 0.005, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.1, damping: 6500, predelay: 0.002, mix: 0.08 }
            },
            'ui.tick': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 3400, ring: 0.01, length: 0.0015 }],
                space: null
            }
        },
        fx: {}
    });
})();
