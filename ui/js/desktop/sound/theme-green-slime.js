// Green Slime: thick gel bubbles, viscous squelches and a wet soft knock in a damp, short room. Key F3 (53), minor third steps
// (0 3 6 9). Pitched notes stay at n >= -5 (the models need 130 Hz and more). Frequent events are short, soft and low in level.
// The theme also owns the window effects slime.pop (a bubble bursting) and slime.snap (a gel strand snapping).
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'green_slime',
        key: 53,
        space: { size: 0.55, rt60: 0.25, damping: 2500, predelay: 0.004, mix: 0.18 },
        voices: {
            glob: { model: 'drop', t60: 0.1, rise: 0.4, pitchSpread: 2, thick: 0.4, splash: 0.1, width: 0.4, attack: 0.004 },
            squelch: { model: 'squelch', count: 4, gap: 0.035, spread: 4, t60: 0.09, dip: 4, rise: 0.7, drive: 2.2, wet: 0.05, wetCut: 1200 },
            knock: { model: 'click', material: 'felt', tone: 900, ring: 0.02, noise: 0.9, length: 0.008 },
            gel: { model: 'noise', color: 'pink', band: [350, 1400], attack: 0.05, decay: 0.12, width: 0.8 },
            thwip: { model: 'noise', color: 'pink', band: [700, 3500], attack: 0.003, decay: 0.05, q: 1.2, width: 0.7 },
            smack: { model: 'noise', color: 'pink', band: [300, 1400], attack: 0.002, decay: 0.03, width: 0.6 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'squelch', n: 3, t: 0, g: 0.8, pan: -0.25, count: 4, spread: 2 }, { v: 'glob', n: 6, t: 0.2, g: 0.85, pan: 0.15 },
                    { v: 'glob', n: 9, t: 0.4, g: 0.9, pan: -0.1, count: 2, spread: 0.05 }, { v: 'glob', n: 12, t: 0.6, g: 0.9, pan: 0.2, count: 2, spread: 0.06 },
                    { v: 'squelch', n: 15, t: 0.85, g: 0.85, pan: 0, count: 5, t60: 0.1 }],
                layers: [{ v: 'gel', t: 0, g: 0.5, attack: 0.2, decay: 0.6 }],
                drive: 1,
                space: { size: 0.9, rt60: 0.7, damping: 2800, predelay: 0.012, mix: 0.28 }
            },
            'notify.message': {
                notes: [{ v: 'squelch', n: 6, t: 0, g: 0.8, pan: -0.2, count: 2, spread: 2 }, { v: 'squelch', n: 9, t: 0.13, g: 0.85, pan: 0.15, count: 3, spread: 3 },
                    { v: 'squelch', n: 15, t: 0.28, g: 1, pan: 0, count: 4, spread: 3 }],
                layers: [{ v: 'gel', t: 0, g: 0.25, decay: 0.25 }],
                drive: 0.8,
                space: { size: 0.75, rt60: 0.45, damping: 2800, predelay: 0.008, mix: 0.24 }
            },
            'notify.info': {
                notes: [{ v: 'glob', n: 6, t: 0, g: 0.85, pan: -0.15 }, { v: 'squelch', n: 12, t: 0.11, g: 1, pan: 0.15, count: 3, spread: 3, t60: 0.08 }],
                space: { size: 0.7, rt60: 0.4, damping: 2800, predelay: 0.006, mix: 0.22 }
            },
            'notify.success': {
                notes: [{ v: 'glob', n: 6, t: 0, g: 0.8, pan: -0.2 }, { v: 'glob', n: 9, t: 0.1, g: 0.9, pan: 0.1 },
                    { v: 'squelch', n: 12, t: 0.2, g: 1, pan: 0, count: 3 }],
                space: { size: 0.7, rt60: 0.4, damping: 2800, predelay: 0.006, mix: 0.22 }
            },
            'notify.warning': {
                notes: [{ v: 'squelch', n: 6, t: 0, g: 1, pan: -0.1, count: 2, spread: 2, dip: 2 }, { v: 'squelch', n: 6, t: 0.19, g: 0.85, pan: 0.1, count: 2, spread: 2, dip: 2 }],
                space: { size: 0.65, rt60: 0.35, damping: 2500, predelay: 0.006, mix: 0.2 }
            },
            'notify.error': {
                notes: [{ v: 'squelch', n: 12, t: 0, g: 1, pan: -0.1, count: 3, spread: 2, dip: 2, rise: 0.3, attack: 0.012, t60: 0.1 },
                    { v: 'squelch', n: 6, t: 0.17, g: 0.95, pan: 0.1, count: 3, spread: 2, dip: 2, rise: 0.2, attack: 0.012, t60: 0.12 }],
                layers: [{ v: 'gel', t: 0, g: 0.4, attack: 0.03, decay: 0.3 }],
                drive: 0.8,
                space: { size: 0.6, rt60: 0.35, damping: 2200, predelay: 0.005, mix: 0.18 }
            },
            'dialog.open': {
                notes: [{ v: 'glob', n: 9, t: 0, g: 0.85, pan: -0.15, t60: 0.07 }, { v: 'glob', n: 15, t: 0.08, g: 1, pan: 0.15, t60: 0.08 }],
                space: { size: 0.6, rt60: 0.3, damping: 2800, predelay: 0.006, mix: 0.2 }
            },
            'dialog.confirm': {
                notes: [{ v: 'glob', n: 12, t: 0, g: 0.85, pan: -0.15, t60: 0.07 }, { v: 'squelch', n: 18, t: 0.09, g: 1, pan: 0.15, count: 2, spread: 2, t60: 0.06 }],
                space: { size: 0.6, rt60: 0.35, damping: 2800, predelay: 0.006, mix: 0.2 }
            },
            'dialog.cancel': {
                notes: [{ v: 'squelch', n: 18, t: 0, g: 0.85, pan: 0.15, count: 2, spread: 2, t60: 0.05, rise: 0.3 }, { v: 'glob', n: 12, t: 0.09, g: 1, pan: -0.15, t60: 0.07, rise: 0.2 }],
                space: { size: 0.55, rt60: 0.25, damping: 2400, predelay: 0.006, mix: 0.16 }
            },
            'window.open': {
                notes: [{ v: 'glob', n: 9, t: 0, g: 0.9, pan: -0.15, t60: 0.06 }, { v: 'squelch', n: 15, t: 0.07, g: 1, pan: 0.2, count: 2, spread: 2, t60: 0.05 }],
                layers: [{ v: 'smack', t: 0, g: 0.08, decay: 0.05 }],
                space: { size: 0.55, rt60: 0.25, damping: 2800, predelay: 0.004, mix: 0.18 }
            },
            'window.close': {
                notes: [{ v: 'squelch', n: 15, t: 0, g: 0.9, pan: 0.2, count: 2, spread: 2, t60: 0.045, rise: 0.4 }, { v: 'glob', n: 9, t: 0.07, g: 1, pan: -0.15, t60: 0.06 }],
                layers: [{ v: 'smack', t: 0, g: 0.08, decay: 0.05 }],
                space: { size: 0.55, rt60: 0.2, damping: 2400, predelay: 0.004, mix: 0.16 }
            },
            'window.maximize': {
                notes: [{ v: 'glob', n: 6, t: 0, g: 0.85, pan: -0.2, t60: 0.07 }, { v: 'squelch', n: 18, t: 0.1, g: 1, pan: 0.2, count: 3, t60: 0.07 }],
                layers: [{ v: 'gel', t: 0, g: 0.3, attack: 0.06, decay: 0.2 }],
                space: { size: 0.8, rt60: 0.35, damping: 2800, predelay: 0.008, mix: 0.26 }
            },
            'window.minimize': {
                notes: [{ v: 'glob', n: 15, t: 0, g: 0.9, pan: 0.15, t60: 0.05 }, { v: 'glob', n: 9, t: 0.06, g: 1, pan: -0.15, t60: 0.06 }],
                space: { size: 0.5, rt60: 0.2, damping: 2400, predelay: 0.003, mix: 0.14 }
            },
            'window.restore': {
                notes: [{ v: 'glob', n: 9, t: 0, g: 0.9, pan: -0.15, t60: 0.05 }, { v: 'glob', n: 15, t: 0.06, g: 1, pan: 0.15, t60: 0.06 }],
                space: { size: 0.5, rt60: 0.2, damping: 2400, predelay: 0.003, mix: 0.14 }
            },
            'window.snap': {
                notes: [{ v: 'glob', n: 15, t: 0, g: 1, pan: 0, t60: 0.03, rise: 0.5, pitchSpread: 0, splash: 0.2 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'glob', n: 6, t: 0, g: 0.8, pan: 0, t60: 0.07, rise: 0.15, thick: 0.5, splash: 0.2, attack: 0.005 }],
                layers: [{ v: 'knock', t: 0, g: 0.6 }, { v: 'gel', t: 0, g: 0.3, attack: 0.01, decay: 0.08 }],
                space: { size: 0.45, rt60: 0.18, damping: 2200, predelay: 0.003, mix: 0.12 }
            },
            'file.drop': {
                notes: [{ v: 'squelch', n: 9, t: 0, g: 0.9, pan: 0, count: 2, spread: 2, t60: 0.08, dip: 3, rise: 0.3, attack: 0.01 }, { v: 'glob', n: 15, t: 0.07, g: 0.4, pan: 0.2, t60: 0.04 }],
                layers: [{ v: 'knock', t: 0, g: 0.4 }],
                space: { size: 0.55, rt60: 0.28, damping: 2400, predelay: 0.004, mix: 0.18 }
            },
            'file.trash': {
                notes: [{ v: 'glob', n: 18, t: 0, g: 0.8, pan: 0.15, t60: 0.05 }, { v: 'glob', n: 15, t: 0.05, g: 0.9, pan: 0, t60: 0.06 },
                    { v: 'squelch', n: 9, t: 0.1, g: 1, pan: -0.15, count: 3, t60: 0.06 }],
                layers: [{ v: 'gel', t: 0, g: 0.2, attack: 0.02, decay: 0.12 }],
                space: { size: 0.55, rt60: 0.28, damping: 2400, predelay: 0.004, mix: 0.16 }
            },
            'file.delete': {
                notes: [{ v: 'glob', n: 12, t: 0, g: 0.85, pan: 0.1, t60: 0.1, thick: 0.5 }, { v: 'squelch', n: 9, t: 0.1, g: 1, pan: -0.1, count: 4, spread: 2, t60: 0.1, dip: 3, rise: 0.3 }],
                layers: [{ v: 'gel', t: 0.05, g: 0.35, attack: 0.04, decay: 0.2 }],
                drive: 0.8,
                space: { size: 0.65, rt60: 0.38, damping: 2200, predelay: 0.005, mix: 0.2 }
            },
            'space.switch': {
                notes: [{ v: 'squelch', n: 6, t: 0.06, g: 0.5, pan: 0, count: 3, t60: 0.06, attack: 0.02 }],
                layers: [{ v: 'gel', t: 0, g: 0.9, attack: 0.14, decay: 0.2, band: [350, 1600] }],
                space: { size: 0.7, rt60: 0.3, damping: 2600, predelay: 0.006, mix: 0.2 }
            },
            'menu.open': {
                notes: [{ v: 'glob', n: 9, t: 0, g: 0.9, pan: -0.1, t60: 0.03, splash: 0.1 }, { v: 'glob', n: 15, t: 0.035, g: 1, pan: 0.1, t60: 0.03, splash: 0.1 }],
                space: { size: 0.4, rt60: 0.1, damping: 2800, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'glob', n: 15, t: 0, g: 0.9, pan: 0.1, t60: 0.025, splash: 0.1 }, { v: 'glob', n: 9, t: 0.03, g: 1, pan: -0.1, t60: 0.025, splash: 0.1 }],
                space: { size: 0.4, rt60: 0.07, damping: 2800, predelay: 0.002, mix: 0.07 }
            },
            'ui.select': {
                notes: [{ v: 'glob', n: 15, t: 0, g: 0.8, pan: -0.1, t60: 0.02, rise: 0.3, dur: 0.035 }, { v: 'glob', n: 12, t: 0.03, g: 1, pan: 0.1, t60: 0.02, rise: 0.3, dur: 0.04 }],
                space: { size: 0.4, rt60: 0.08, damping: 2800, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.on': {
                notes: [{ v: 'glob', n: 12, t: 0, g: 0.9, pan: -0.1, t60: 0.02, dur: 0.035 }, { v: 'glob', n: 18, t: 0.035, g: 1, pan: 0.1, t60: 0.02, dur: 0.035 }],
                space: { size: 0.4, rt60: 0.08, damping: 2800, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.off': {
                notes: [{ v: 'glob', n: 18, t: 0, g: 0.9, pan: 0.1, t60: 0.02, dur: 0.035 }, { v: 'glob', n: 12, t: 0.035, g: 1, pan: -0.1, t60: 0.02, dur: 0.035 }],
                space: { size: 0.4, rt60: 0.08, damping: 2800, predelay: 0.002, mix: 0.08 }
            },
            'ui.tick': {
                notes: [{ v: 'glob', n: 18, t: 0, g: 0.8, pan: -0.15, t60: 0.02, rise: 0.3, thick: 0.4, attack: 0.001, dur: 0.025 },
                    { v: 'glob', n: 15, t: 0.012, g: 1, pan: 0.15, t60: 0.02, rise: 0.3, thick: 0.4, attack: 0.001, dur: 0.028 }],
                space: null
            }
        },
        fx: {
            'slime.pop': {
                small: {
                    notes: [{ v: 'glob', n: 18, t: 0, g: 1, pan: 0, t60: 0.03, rise: 1.2, thick: 0.4, splash: 0.4, pitchSpread: 0, dur: 0.12 }],
                    layers: [{ v: 'smack', t: 0, g: 0.15, decay: 0.03 }]
                },
                medium: {
                    notes: [{ v: 'glob', n: 12, t: 0, g: 1, pan: 0, t60: 0.04, rise: 1, thick: 0.5, splash: 0.4, pitchSpread: 0, dur: 0.15 }],
                    layers: [{ v: 'smack', t: 0, g: 0.15, decay: 0.04 }]
                },
                large: {
                    notes: [{ v: 'glob', n: 6, t: 0, g: 1, pan: 0, t60: 0.05, rise: 0.8, thick: 0.6, splash: 0.4, pitchSpread: 0, dur: 0.17 }],
                    layers: [{ v: 'smack', t: 0, g: 0.15, decay: 0.05, band: [200, 1000] }]
                }
            },
            'slime.snap': {
                small: {
                    notes: [{ v: 'squelch', n: 15, t: 0.008, g: 0.8, pan: 0, count: 2, gap: 0.025, t60: 0.04, dip: 3, rise: 1, drive: 1.5, wet: 0.1, wetCut: 1800, dur: 0.15 }],
                    layers: [{ v: 'smack', t: 0, g: 0.5, band: [800, 3500], decay: 0.02 }]
                },
                medium: {
                    notes: [{ v: 'squelch', n: 9, t: 0.01, g: 0.8, pan: 0, count: 2, gap: 0.03, t60: 0.05, dip: 3, rise: 1, drive: 1.5, wet: 0.12, wetCut: 1800, dur: 0.19 }],
                    layers: [{ v: 'smack', t: 0, g: 0.5, band: [600, 2800], decay: 0.025 }]
                },
                large: {
                    notes: [{ v: 'squelch', n: 6, t: 0.012, g: 0.8, pan: 0, count: 2, gap: 0.035, t60: 0.055, dip: 3, rise: 1, drive: 1.5, wet: 0.14, wetCut: 1800, dur: 0.22 }],
                    layers: [{ v: 'smack', t: 0, g: 0.5, band: [400, 2200], decay: 0.03 }]
                }
            }
        }
    });
})();
