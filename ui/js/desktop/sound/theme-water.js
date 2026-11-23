// Water: clear drops and small drop clusters in a wet, damped cave, with a low-passed wash. Key G4 (67), G major pentatonic.
// Frequent events (window, menu, touch) are two quick drops; only session.start uses the full cave.
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'water',
        key: 67,
        space: { size: 1, rt60: 0.8, damping: 4500, predelay: 0.012, mix: 0.3 },
        voices: {
            drop: { model: 'drop', t60: 0.07, rise: 0.6, pitchSpread: 2, splash: 0.2, width: 0.5 },
            low: { model: 'drop', t60: 0.1, rise: 0.3, pitchSpread: 1, thick: 0.25, splash: 0.15, width: 0.3 },
            wash: { model: 'noise', color: 'pink', band: [180, 1500], attack: 0.1, decay: 0.3, width: 0.9 },
            knock: { model: 'noise', color: 'pink', band: [120, 700], attack: 0.001, decay: 0.03 },
            trickle: { model: 'noise', color: 'pink', band: [600, 3500], attack: 0.01, decay: 0.08 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'drop', n: 0, t: 0, g: 0.8, pan: -0.25, t60: 0.1 }, { v: 'drop', n: 4, t: 0.2, g: 0.85, pan: 0.15, t60: 0.1 },
                    { v: 'drop', n: 9, t: 0.4, g: 0.9, pan: -0.1, t60: 0.11, count: 2, spread: 0.05 },
                    { v: 'drop', n: 12, t: 0.62, g: 0.9, pan: 0.2, t60: 0.12, count: 2, spread: 0.06 },
                    { v: 'drop', n: 16, t: 0.9, g: 0.85, pan: 0, t60: 0.16, count: 3, spread: 0.07, splash: 0.3 }],
                layers: [{ v: 'wash', t: 0, g: 0.3, attack: 0.25, decay: 0.7 }],
                space: { size: 1.3, rt60: 1.5, damping: 4000, predelay: 0.02, mix: 0.38 }
            },
            'notify.message': {
                notes: [{ v: 'drop', n: 0, t: 0, g: 0.8, pan: -0.2, t60: 0.08 }, { v: 'drop', n: 4, t: 0.12, g: 0.9, pan: 0.15, t60: 0.09 },
                    { v: 'drop', n: 9, t: 0.24, g: 1, pan: 0, t60: 0.11, count: 2, spread: 0.05 }],
                layers: [{ v: 'wash', t: 0, g: 0.15, decay: 0.3 }],
                space: { size: 1.1, rt60: 0.95, damping: 4000, predelay: 0.015, mix: 0.33 }
            },
            'notify.info': {
                notes: [{ v: 'drop', n: 4, t: 0, g: 0.85, pan: -0.15, t60: 0.08 }, { v: 'drop', n: 9, t: 0.11, g: 1, pan: 0.15, t60: 0.1 }],
                space: { size: 1, rt60: 0.85, damping: 4200, predelay: 0.012, mix: 0.3 }
            },
            'notify.success': {
                notes: [{ v: 'drop', n: 4, t: 0, g: 0.8, pan: -0.2, t60: 0.08, rise: 0.9 }, { v: 'drop', n: 7, t: 0.1, g: 0.9, pan: 0.1, t60: 0.09, rise: 0.9 },
                    { v: 'drop', n: 12, t: 0.2, g: 1, pan: 0, t60: 0.12, count: 3, spread: 0.04, pitchSpread: 3 }],
                layers: [{ v: 'trickle', t: 0.18, g: 0.12, decay: 0.2 }],
                space: { size: 1, rt60: 0.8, damping: 4200, predelay: 0.012, mix: 0.3 }
            },
            'notify.warning': {
                notes: [{ v: 'drop', n: 7, t: 0, g: 1, pan: -0.1, t60: 0.09, rise: 0.2, attack: 0.005 },
                    { v: 'drop', n: 7, t: 0.17, g: 0.85, pan: 0.1, t60: 0.11, rise: 0.2, attack: 0.005 }],
                layers: [{ v: 'wash', t: 0, g: 0.12, decay: 0.2 }],
                space: { size: 0.9, rt60: 0.75, damping: 3800, predelay: 0.01, mix: 0.28 }
            },
            'notify.error': {
                notes: [{ v: 'drop', n: 4, t: 0, g: 1, pan: -0.1, t60: 0.12, attack: 0.008, rise: 0.15, count: 2, spread: 0.05, splash: 0.4 },
                    { v: 'drop', n: -2, t: 0.15, g: 0.95, pan: 0.1, t60: 0.15, attack: 0.008, rise: 0.15, count: 2, spread: 0.06, splash: 0.4 }],
                layers: [{ v: 'wash', t: 0, g: 0.35, attack: 0.04, decay: 0.35, band: [250, 1400] }],
                space: { size: 0.8, rt60: 0.55, damping: 3200, predelay: 0.008, mix: 0.24 }
            },
            'dialog.open': {
                notes: [{ v: 'drop', n: 0, t: 0, g: 0.85, pan: -0.15, t60: 0.06 }, { v: 'drop', n: 7, t: 0.08, g: 1, pan: 0.15, t60: 0.07 }],
                space: { size: 0.8, rt60: 0.45, damping: 4200, predelay: 0.01, mix: 0.24 }
            },
            'dialog.confirm': {
                notes: [{ v: 'drop', n: 4, t: 0, g: 0.85, pan: -0.15, t60: 0.06 }, { v: 'drop', n: 9, t: 0.09, g: 1, pan: 0.15, t60: 0.08 }],
                space: { size: 0.85, rt60: 0.5, damping: 4200, predelay: 0.01, mix: 0.25 }
            },
            'dialog.cancel': {
                notes: [{ v: 'drop', n: 9, t: 0, g: 0.85, pan: 0.15, t60: 0.05, rise: 0.3 }, { v: 'drop', n: 4, t: 0.09, g: 1, pan: -0.15, t60: 0.06, rise: 0.3 }],
                space: { size: 0.8, rt60: 0.35, damping: 3600, predelay: 0.01, mix: 0.2 }
            },
            'window.open': {
                notes: [{ v: 'drop', n: 0, t: 0, g: 0.9, pan: -0.15, t60: 0.05 }, { v: 'drop', n: 4, t: 0.075, g: 1, pan: 0.2, t60: 0.06 }],
                layers: [{ v: 'trickle', t: 0, g: 0.06, decay: 0.07 }],
                space: { size: 0.8, rt60: 0.35, damping: 4200, predelay: 0.01, mix: 0.22 }
            },
            'window.close': {
                notes: [{ v: 'drop', n: 4, t: 0, g: 0.9, pan: 0.2, t60: 0.05 }, { v: 'drop', n: 0, t: 0.075, g: 1, pan: -0.15, t60: 0.05 }],
                layers: [{ v: 'trickle', t: 0, g: 0.06, decay: 0.07 }],
                space: { size: 0.8, rt60: 0.3, damping: 3800, predelay: 0.01, mix: 0.2 }
            },
            'window.maximize': {
                notes: [{ v: 'drop', n: 0, t: 0, g: 0.85, pan: -0.2, t60: 0.06 }, { v: 'drop', n: 9, t: 0.09, g: 1, pan: 0.2, t60: 0.08, count: 2, spread: 0.04 }],
                layers: [{ v: 'wash', t: 0, g: 0.12, attack: 0.08, decay: 0.2 }],
                space: { size: 1, rt60: 0.42, damping: 4200, predelay: 0.012, mix: 0.3 }
            },
            'window.minimize': {
                notes: [{ v: 'drop', n: 7, t: 0, g: 0.9, pan: 0.15, t60: 0.04 }, { v: 'drop', n: 2, t: 0.06, g: 1, pan: -0.15, t60: 0.05 }],
                space: { size: 0.7, rt60: 0.25, damping: 3800, predelay: 0.008, mix: 0.18 }
            },
            'window.restore': {
                notes: [{ v: 'drop', n: 2, t: 0, g: 0.9, pan: -0.15, t60: 0.04 }, { v: 'drop', n: 7, t: 0.06, g: 1, pan: 0.15, t60: 0.05 }],
                space: { size: 0.7, rt60: 0.25, damping: 3800, predelay: 0.008, mix: 0.18 }
            },
            'window.snap': {
                notes: [{ v: 'drop', n: 12, t: 0, g: 1, pan: 0, t60: 0.03, rise: 0.4, splash: 0.15, pitchSpread: 0 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'drop', n: 0, t: 0, g: 0.8, pan: 0, t60: 0.06, rise: 0.1, splash: 0.5, attack: 0.005 }, { v: 'drop', n: -5, t: 0.045, g: 0.8, pan: 0, t60: 0.07, rise: 0.1, attack: 0.005 }],
                layers: [{ v: 'wash', t: 0, g: 0.3, attack: 0.02, decay: 0.12, band: [250, 1100] }],
                space: { size: 0.5, rt60: 0.22, damping: 3500, predelay: 0.004, mix: 0.14 }
            },
            'file.drop': {
                notes: [{ v: 'low', n: -3, t: 0, g: 0.9, pan: 0, t60: 0.1, rise: 0.4, splash: 0.5, thick: 0 }, { v: 'drop', n: 4, t: 0.07, g: 0.45, pan: 0.2, t60: 0.04 }],
                layers: [{ v: 'trickle', t: 0.01, g: 0.12, decay: 0.12 }],
                space: { size: 0.7, rt60: 0.38, damping: 3800, predelay: 0.006, mix: 0.2 }
            },
            'file.trash': {
                notes: [{ v: 'drop', n: 12, t: 0, g: 0.8, pan: 0.15, t60: 0.04 }, { v: 'drop', n: 7, t: 0.05, g: 0.9, pan: 0, t60: 0.05 },
                    { v: 'drop', n: 2, t: 0.1, g: 1, pan: -0.15, t60: 0.07 }],
                layers: [{ v: 'trickle', t: 0, g: 0.1, decay: 0.12 }],
                space: { size: 0.7, rt60: 0.35, damping: 3800, predelay: 0.006, mix: 0.18 }
            },
            'file.delete': {
                notes: [{ v: 'low', n: 2, t: 0, g: 0.85, pan: 0.1, t60: 0.1, rise: 0.2 }, { v: 'low', n: -5, t: 0.1, g: 1, pan: -0.1, t60: 0.13, rise: 0.15, thick: 0.4 }],
                layers: [{ v: 'wash', t: 0.05, g: 0.2, attack: 0.05, decay: 0.2, band: [150, 900] }],
                space: { size: 0.85, rt60: 0.5, damping: 3500, predelay: 0.008, mix: 0.24 }
            },
            'space.switch': {
                notes: [{ v: 'drop', n: 12, t: 0.15, g: 0.5, pan: 0, t60: 0.06 }],
                layers: [{ v: 'wash', t: 0, g: 0.9, attack: 0.14, decay: 0.2, band: [400, 2200] }],
                space: { size: 0.9, rt60: 0.4, damping: 4000, predelay: 0.01, mix: 0.22 }
            },
            'menu.open': {
                notes: [{ v: 'drop', n: 4, t: 0, g: 0.9, pan: -0.1, t60: 0.025, splash: 0.1 }, { v: 'drop', n: 9, t: 0.035, g: 1, pan: 0.1, t60: 0.025, splash: 0.1 }],
                space: { size: 0.4, rt60: 0.1, damping: 5000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'drop', n: 9, t: 0, g: 0.9, pan: 0.1, t60: 0.02, splash: 0.1 }, { v: 'drop', n: 4, t: 0.03, g: 1, pan: -0.1, t60: 0.02, splash: 0.1 }],
                space: { size: 0.4, rt60: 0.07, damping: 5000, predelay: 0.002, mix: 0.07 }
            },
            'ui.select': {
                notes: [{ v: 'drop', n: 9, t: 0, g: 0.8, pan: -0.1, t60: 0.02, rise: 0.3, splash: 0.1 }, { v: 'drop', n: 7, t: 0.03, g: 1, pan: 0.1, t60: 0.02, rise: 0.3 }],
                space: { size: 0.4, rt60: 0.08, damping: 5000, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.on': {
                notes: [{ v: 'drop', n: 7, t: 0, g: 0.9, pan: -0.1, t60: 0.02 }, { v: 'drop', n: 12, t: 0.035, g: 1, pan: 0.1, t60: 0.02 }],
                layers: [{ v: 'trickle', t: 0, g: 0.05, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.08, damping: 5000, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.off': {
                notes: [{ v: 'drop', n: 12, t: 0, g: 0.9, pan: 0.1, t60: 0.02 }, { v: 'drop', n: 7, t: 0.035, g: 1, pan: -0.1, t60: 0.02 }],
                layers: [{ v: 'trickle', t: 0, g: 0.05, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.08, damping: 5000, predelay: 0.002, mix: 0.08 }
            },
            'ui.tick': {
                notes: [{ v: 'drop', n: 16, t: 0, g: 0.8, pan: -0.15, t60: 0.02, rise: 0.3, splash: 0.2, attack: 0.001, dur: 0.025 }, { v: 'drop', n: 19, t: 0.012, g: 1, pan: 0.15, t60: 0.02, rise: 0.3, splash: 0.2, attack: 0.001, dur: 0.028 }],
                space: null
            }
        },
        fx: {}
    });
})();
