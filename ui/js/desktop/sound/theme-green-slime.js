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
            // the lead of every event: a small cluster of viscous bubbles whose pitch sags before it rises
            sq: { model: 'squelch', count: 2, gap: 0.05, spread: 2, t60: 0.07, dip: 6, rise: 0.5, drive: 2.2, wet: 0.08, wetCut: 900, attack: 0.01, width: 0.4 },
            // a dull blob: heavy thick, hardly any rise, only ever the quiet partner of a squelch
            blob: { model: 'drop', t60: 0.08, rise: 0.1, pitchSpread: 1, thick: 0.9, splash: 0, width: 0.4, attack: 0.008 },
            glob: { model: 'drop', t60: 0.1, rise: 0.4, pitchSpread: 2, thick: 0.4, splash: 0.1, width: 0.4, attack: 0.004 },
            squelch: { model: 'squelch', count: 4, gap: 0.035, spread: 4, t60: 0.09, dip: 4, rise: 0.7, drive: 2.2, wet: 0.05, wetCut: 1200 },
            knock: { model: 'click', material: 'felt', tone: 900, ring: 0.02, noise: 0.9, length: 0.008 },
            gel: { model: 'noise', color: 'pink', band: [300, 1200], attack: 0.02, decay: 0.14, width: 0.8 },
            smack: { model: 'noise', color: 'pink', band: [300, 1400], attack: 0.002, decay: 0.03, width: 0.6 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'sq', n: 5, t: 0, g: 0.8, pan: -0.25, count: 4, spread: 2 }, { v: 'sq', n: 8, t: 0.22, g: 0.85, pan: 0.15, count: 2 },
                    { v: 'blob', n: 11, t: 0.42, g: 0.35, pan: -0.1 }, { v: 'sq', n: 14, t: 0.46, g: 0.9, pan: 0.2, count: 3, dip: 6 },
                    { v: 'sq', n: 17, t: 0.82, g: 0.9, pan: 0, count: 5, t60: 0.1, dip: 6 }],
                layers: [{ v: 'gel', t: 0, g: 0.5, attack: 0.03, decay: 0.7 }],
                drive: 1,
                space: { size: 0.8, rt60: 0.6, damping: 2200, predelay: 0.01, mix: 0.26 }
            },
            'notify.message': {
                notes: [{ v: 'sq', n: 8, t: 0, g: 0.8, pan: -0.2, count: 2 }, { v: 'sq', n: 11, t: 0.13, g: 0.85, pan: 0.15, count: 2, dip: 6 },
                    { v: 'sq', n: 17, t: 0.27, g: 1, pan: 0, count: 3, dip: 6, t60: 0.09 }],
                layers: [{ v: 'gel', t: 0, g: 0.25, decay: 0.25 }],
                drive: 0.8,
                space: { size: 0.65, rt60: 0.4, damping: 2200, predelay: 0.006, mix: 0.22 }
            },
            'notify.info': {
                notes: [{ v: 'sq', n: 8, t: 0, g: 0.85, pan: -0.15, count: 2 }, { v: 'sq', n: 14, t: 0.12, g: 1, pan: 0.15, count: 2, dip: 6, t60: 0.08 },
                    { v: 'blob', n: 8, t: 0.12, g: 0.3, pan: 0.1 }],
                layers: [{ v: 'gel', t: 0.05, g: 0.2, decay: 0.2 }],
                space: { size: 0.6, rt60: 0.35, damping: 2200, predelay: 0.005, mix: 0.2 }
            },
            'notify.success': {
                notes: [{ v: 'sq', n: 8, t: 0, g: 0.8, pan: -0.2, count: 2 }, { v: 'sq', n: 11, t: 0.11, g: 0.9, pan: 0.1, count: 2, dip: 6 },
                    { v: 'sq', n: 17, t: 0.23, g: 1, pan: 0, count: 3, dip: 6 }],
                space: { size: 0.6, rt60: 0.35, damping: 2200, predelay: 0.005, mix: 0.2 }
            },
            'notify.warning': {
                notes: [{ v: 'sq', n: 8, t: 0, g: 1, pan: -0.1, count: 2, spread: 2, dip: 3 }, { v: 'sq', n: 8, t: 0.2, g: 0.85, pan: 0.1, count: 2, spread: 2, dip: 3 }],
                space: { size: 0.6, rt60: 0.3, damping: 2200, predelay: 0.005, mix: 0.2 }
            },
            'notify.error': {
                notes: [{ v: 'sq', n: 14, t: 0, g: 1, pan: -0.1, count: 3, spread: 2, dip: 3, rise: 0.3, attack: 0.012, t60: 0.1 },
                    { v: 'sq', n: 8, t: 0.17, g: 0.95, pan: 0.1, count: 3, spread: 2, dip: 3, rise: 0.2, attack: 0.012, t60: 0.12 }],
                layers: [{ v: 'gel', t: 0, g: 0.4, attack: 0.03, decay: 0.3 }],
                drive: 0.8,
                space: { size: 0.55, rt60: 0.3, damping: 2000, predelay: 0.005, mix: 0.18 }
            },
            'dialog.open': {
                notes: [{ v: 'sq', n: 11, t: 0, g: 0.85, pan: -0.15, count: 1, t60: 0.07 }, { v: 'sq', n: 17, t: 0.1, g: 1, pan: 0.15, count: 2, dip: 6, t60: 0.07 }],
                layers: [{ v: 'gel', t: 0, g: 0.18, decay: 0.15 }],
                space: { size: 0.55, rt60: 0.28, damping: 2200, predelay: 0.005, mix: 0.2 }
            },
            'dialog.confirm': {
                notes: [{ v: 'sq', n: 11, t: 0, g: 0.85, pan: -0.15, count: 2, t60: 0.07 }, { v: 'sq', n: 17, t: 0.11, g: 1, pan: 0.15, count: 2, dip: 6, t60: 0.07 },
                    { v: 'blob', n: 11, t: 0.11, g: 0.3, pan: 0.15 }],
                layers: [{ v: 'gel', t: 0, g: 0.18, decay: 0.18 }],
                space: { size: 0.55, rt60: 0.3, damping: 2200, predelay: 0.005, mix: 0.2 }
            },
            'dialog.cancel': {
                notes: [{ v: 'sq', n: 17, t: 0, g: 0.85, pan: 0.15, count: 2, t60: 0.06, rise: 0.3 }, { v: 'sq', n: 11, t: 0.11, g: 1, pan: -0.15, count: 2, t60: 0.07, dip: 6, rise: 0.25 }],
                space: { size: 0.5, rt60: 0.22, damping: 2000, predelay: 0.005, mix: 0.16 }
            },
            'window.open': {
                notes: [{ v: 'sq', n: 8, t: 0, g: 0.9, pan: -0.15, count: 1, t60: 0.06 }, { v: 'sq', n: 14, t: 0.1, g: 1, pan: 0.2, count: 2, t60: 0.055, dip: 6 }],
                layers: [{ v: 'gel', t: 0, g: 0.15, decay: 0.12 }],
                space: { size: 0.5, rt60: 0.22, damping: 2200, predelay: 0.004, mix: 0.18 }
            },
            'window.close': {
                notes: [{ v: 'sq', n: 14, t: 0, g: 0.9, pan: 0.2, count: 2, t60: 0.05, dip: 4, rise: 0.4 }, { v: 'sq', n: 8, t: 0.1, g: 1, pan: -0.15, count: 1, t60: 0.06, dip: 6 }],
                layers: [{ v: 'gel', t: 0, g: 0.15, decay: 0.12 }],
                space: { size: 0.5, rt60: 0.2, damping: 2000, predelay: 0.004, mix: 0.16 }
            },
            'window.maximize': {
                notes: [{ v: 'sq', n: 5, t: 0, g: 0.85, pan: -0.2, count: 1, t60: 0.07 }, { v: 'sq', n: 17, t: 0.12, g: 1, pan: 0.2, count: 3, t60: 0.07, dip: 6 }],
                layers: [{ v: 'gel', t: 0, g: 0.3, attack: 0.03, decay: 0.2 }],
                space: { size: 0.7, rt60: 0.32, damping: 2200, predelay: 0.007, mix: 0.24 }
            },
            'window.minimize': {
                notes: [{ v: 'sq', n: 14, t: 0, g: 0.9, pan: 0.15, count: 1, t60: 0.05 }, { v: 'sq', n: 8, t: 0.09, g: 1, pan: -0.15, count: 1, t60: 0.06, dip: 6 }],
                space: { size: 0.45, rt60: 0.2, damping: 2000, predelay: 0.003, mix: 0.14 }
            },
            'window.restore': {
                notes: [{ v: 'sq', n: 8, t: 0, g: 0.9, pan: -0.15, count: 1, t60: 0.05 }, { v: 'sq', n: 14, t: 0.09, g: 1, pan: 0.15, count: 1, t60: 0.06, dip: 6 }],
                space: { size: 0.45, rt60: 0.2, damping: 2000, predelay: 0.003, mix: 0.14 }
            },
            'window.snap': {
                notes: [{ v: 'sq', n: 14, t: 0, g: 1, pan: 0, count: 1, t60: 0.035, dip: 4, rise: 0.6 }],
                layers: [{ v: 'smack', t: 0, g: 0.2, decay: 0.02 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'sq', n: 12, t: 0, g: 0.8, pan: 0, count: 2, t60: 0.07, dip: 2, rise: 0.15, attack: 0.005 }],
                layers: [{ v: 'knock', t: 0, g: 0.6 }, { v: 'gel', t: 0, g: 0.3, attack: 0.01, decay: 0.08 }],
                space: { size: 0.45, rt60: 0.18, damping: 2000, predelay: 0.003, mix: 0.12 }
            },
            'file.drop': {
                notes: [{ v: 'sq', n: 8, t: 0, g: 0.9, pan: 0, count: 2, spread: 2, t60: 0.08, dip: 4, rise: 0.3, attack: 0.01 },
                    { v: 'blob', n: 14, t: 0.08, g: 0.4, pan: 0.2, t60: 0.06 }],
                layers: [{ v: 'knock', t: 0, g: 0.4 }, { v: 'gel', t: 0.03, g: 0.15, decay: 0.12 }],
                space: { size: 0.5, rt60: 0.25, damping: 2000, predelay: 0.004, mix: 0.18 }
            },
            'file.trash': {
                notes: [{ v: 'sq', n: 17, t: 0, g: 0.8, pan: 0.15, count: 1, t60: 0.05 }, { v: 'sq', n: 14, t: 0.09, g: 0.9, pan: 0, count: 1, t60: 0.06 },
                    { v: 'sq', n: 8, t: 0.18, g: 1, pan: -0.15, count: 3, t60: 0.06, dip: 6 }],
                layers: [{ v: 'gel', t: 0, g: 0.2, attack: 0.02, decay: 0.12 }],
                space: { size: 0.5, rt60: 0.25, damping: 2000, predelay: 0.004, mix: 0.16 }
            },
            'file.delete': {
                notes: [{ v: 'blob', n: 14, t: 0, g: 0.5, pan: 0.1, t60: 0.1 }, { v: 'sq', n: 8, t: 0.11, g: 1, pan: -0.1, count: 4, spread: 2, t60: 0.1, dip: 4, rise: 0.3 }],
                layers: [{ v: 'gel', t: 0.05, g: 0.35, attack: 0.04, decay: 0.2 }],
                drive: 0.8,
                space: { size: 0.6, rt60: 0.35, damping: 2000, predelay: 0.005, mix: 0.2 }
            },
            'space.switch': {
                notes: [{ v: 'sq', n: 8, t: 0.06, g: 0.5, pan: 0, count: 3, t60: 0.06, attack: 0.02 }],
                layers: [{ v: 'gel', t: 0, g: 0.9, attack: 0.14, decay: 0.2, band: [350, 1600] }],
                space: { size: 0.65, rt60: 0.3, damping: 2400, predelay: 0.006, mix: 0.2 }
            },
            'menu.open': {
                notes: [{ v: 'sq', n: 8, t: 0, g: 0.9, pan: -0.1, count: 1, t60: 0.035, dip: 4 }, { v: 'sq', n: 14, t: 0.055, g: 1, pan: 0.1, count: 1, t60: 0.035, dip: 6 }],
                layers: [{ v: 'gel', t: 0, g: 0.1, decay: 0.06 }],
                space: { size: 0.4, rt60: 0.1, damping: 2200, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'sq', n: 14, t: 0, g: 0.9, pan: 0.1, count: 1, t60: 0.03, dip: 4 }, { v: 'sq', n: 10, t: 0.05, g: 1, pan: -0.1, count: 1, t60: 0.03, dip: 4 }],
                space: { size: 0.4, rt60: 0.07, damping: 2200, predelay: 0.002, mix: 0.07 }
            },
            'ui.select': {
                notes: [{ v: 'sq', n: 17, t: 0, g: 0.8, pan: -0.1, count: 1, t60: 0.03, dip: 3, rise: 0.3, dur: 0.028 }, { v: 'sq', n: 14, t: 0.04, g: 1, pan: 0.1, count: 1, t60: 0.03, dip: 3, rise: 0.3, dur: 0.04 }],
                space: { size: 0.4, rt60: 0.05, damping: 2200, predelay: 0.002, mix: 0.07 }
            },
            'ui.toggle.on': {
                notes: [{ v: 'sq', n: 14, t: 0, g: 0.9, pan: -0.1, count: 1, t60: 0.03, dip: 4, dur: 0.028 }, { v: 'sq', n: 20, t: 0.04, g: 1, pan: 0.1, count: 1, t60: 0.03, dip: 4, dur: 0.028 }],
                space: { size: 0.4, rt60: 0.05, damping: 2200, predelay: 0.002, mix: 0.07 }
            },
            'ui.toggle.off': {
                notes: [{ v: 'sq', n: 20, t: 0, g: 0.9, pan: 0.1, count: 1, t60: 0.03, dip: 4, dur: 0.028 }, { v: 'sq', n: 14, t: 0.04, g: 1, pan: -0.1, count: 1, t60: 0.03, dip: 4, dur: 0.028 }],
                space: { size: 0.4, rt60: 0.05, damping: 2200, predelay: 0.002, mix: 0.07 }
            },
            'ui.tick': {
                notes: [{ v: 'sq', n: 20, t: 0, g: 0.8, pan: -0.15, count: 1, t60: 0.03, dip: 2, rise: 0.3, attack: 0.002, dur: 0.014 },
                    { v: 'sq', n: 17, t: 0.014, g: 1, pan: 0.15, count: 1, t60: 0.03, dip: 2, rise: 0.3, attack: 0.002, dur: 0.016 }],
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
