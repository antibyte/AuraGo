// Crystal: glass chimes, a breath of air and a bright but soft hall. Key A5 (81), A major pentatonic.
// Frequent events (window, menu, touch) sit in the octave below the key; two- and three-note gestures read as an instrument.
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'crystal',
        key: 81,
        space: { size: 0.9, rt60: 0.85, damping: 7000, predelay: 0.012, mix: 0.24 },
        voices: {
            chime: { model: 'glass', t60: 0.9, beat: 0.55, attack: 0.004 },
            tick: { model: 'click', material: 'glass', tone: 5200, ring: 0.03 },
            air: { model: 'noise', color: 'pink', band: [2500, 7500], attack: 0.06, decay: 0.2 },
            knock: { model: 'noise', color: 'pink', band: [120, 900], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'chime', n: -5, t: 0, g: 0.8, pan: -0.25, t60: 0.8 }, { v: 'chime', n: 0, t: 0.17, g: 0.85, pan: 0.1, t60: 0.85 },
                    { v: 'chime', n: 4, t: 0.34, g: 0.9, pan: -0.1, t60: 0.9 }, { v: 'chime', n: 7, t: 0.52, g: 0.9, pan: 0.2, t60: 1 },
                    { v: 'chime', n: 12, t: 0.78, g: 0.85, pan: 0, t60: 1.3, attack: 0.006 }],
                layers: [{ v: 'air', t: 0, g: 0.05 }],
                space: { size: 1.2, rt60: 1.4, damping: 6500, predelay: 0.02, mix: 0.34 }
            },
            'notify.message': {
                notes: [{ v: 'chime', n: -5, t: 0, g: 0.8, pan: -0.2, t60: 0.7 }, { v: 'chime', n: 0, t: 0.11, g: 0.9, pan: 0.15, t60: 0.8 },
                    { v: 'chime', n: 4, t: 0.22, g: 1, pan: 0, t60: 0.95 }],
                space: { size: 1.05, rt60: 0.95, damping: 6500, predelay: 0.018, mix: 0.3 }
            },
            'notify.info': {
                notes: [{ v: 'chime', n: 0, t: 0, g: 0.85, pan: -0.15, t60: 0.6 }, { v: 'chime', n: 4, t: 0.1, g: 1, pan: 0.15, t60: 0.85 }],
                space: { size: 0.95, rt60: 0.9, damping: 6500, predelay: 0.014, mix: 0.27 }
            },
            'notify.success': {
                notes: [{ v: 'chime', n: -8, t: 0, g: 0.8, pan: -0.2, t60: 0.6 }, { v: 'chime', n: -3, t: 0.09, g: 0.9, pan: 0.1, t60: 0.7 },
                    { v: 'chime', n: 0, t: 0.18, g: 1, pan: 0, t60: 0.75 }],
                space: { size: 1, rt60: 0.8, damping: 6500, predelay: 0.014, mix: 0.28 }
            },
            'notify.warning': {
                notes: [{ v: 'chime', n: -10, t: 0, g: 1, pan: -0.1, t60: 0.55, damp: 0.9, attack: 0.006, air: 0.07 }, { v: 'chime', n: 2, t: 0, g: 0.3, pan: 0.1, t60: 0.4 },
                    { v: 'chime', n: -10, t: 0.17, g: 0.85, pan: 0.1, t60: 0.65, damp: 0.9, attack: 0.006, air: 0.07 }, { v: 'chime', n: 2, t: 0.17, g: 0.25, pan: -0.1, t60: 0.5 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.2 }],
                space: { size: 0.9, rt60: 0.8, damping: 5500, predelay: 0.01, mix: 0.26 }
            },
            'notify.error': {
                notes: [{ v: 'chime', n: -5, t: 0, g: 1, pan: -0.1, t60: 0.45, damp: 1, attack: 0.008, air: 0.07 },
                    { v: 'chime', n: -11, t: 0.15, g: 0.95, pan: 0.1, t60: 0.55, damp: 1, attack: 0.008, air: 0.07 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.2 }],
                space: { size: 0.8, rt60: 0.6, damping: 4500, predelay: 0.008, mix: 0.24 }
            },
            'dialog.open': {
                notes: [{ v: 'chime', n: -10, t: 0, g: 0.85, pan: -0.15, t60: 0.28 }, { v: 'chime', n: -5, t: 0.08, g: 1, pan: 0.15, t60: 0.32 }],
                space: { size: 0.8, rt60: 0.5, damping: 6500, predelay: 0.01, mix: 0.2 }
            },
            'dialog.confirm': {
                notes: [{ v: 'chime', n: -3, t: 0, g: 0.85, pan: -0.15, t60: 0.28 }, { v: 'chime', n: 4, t: 0.09, g: 1, pan: 0.15, t60: 0.36 }],
                space: { size: 0.8, rt60: 0.55, damping: 6500, predelay: 0.01, mix: 0.2 }
            },
            'dialog.cancel': {
                notes: [{ v: 'chime', n: 4, t: 0, g: 0.85, pan: 0.15, t60: 0.28 }, { v: 'chime', n: -3, t: 0.09, g: 1, pan: -0.15, t60: 0.3 }],
                space: { size: 0.8, rt60: 0.4, damping: 6500, predelay: 0.01, mix: 0.2 }
            },
            'window.open': {
                notes: [{ v: 'chime', n: -5, t: 0, g: 0.9, pan: -0.15, t60: 0.3 }, { v: 'chime', n: 0, t: 0.085, g: 1, pan: 0.2, t60: 0.3 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.12 }],
                space: { size: 0.8, rt60: 0.4, damping: 7000, predelay: 0.01, mix: 0.2 }
            },
            'window.close': {
                notes: [{ v: 'chime', n: 0, t: 0, g: 0.9, pan: 0.2, t60: 0.25 }, { v: 'chime', n: -5, t: 0.085, g: 1, pan: -0.15, t60: 0.25 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.12 }],
                space: { size: 0.8, rt60: 0.35, damping: 6000, predelay: 0.01, mix: 0.2 }
            },
            'window.maximize': {
                notes: [{ v: 'chime', n: -12, t: 0, g: 0.85, pan: -0.2, t60: 0.3 }, { v: 'chime', n: -3, t: 0.1, g: 1, pan: 0.2, t60: 0.34 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.12 }],
                space: { size: 1, rt60: 0.45, damping: 7000, predelay: 0.012, mix: 0.28 }
            },
            'window.minimize': {
                notes: [{ v: 'chime', n: -3, t: 0, g: 0.9, pan: 0.15, t60: 0.2 }, { v: 'chime', n: -10, t: 0.065, g: 1, pan: -0.15, t60: 0.22 }],
                space: { size: 0.7, rt60: 0.3, damping: 6000, predelay: 0.008, mix: 0.16 }
            },
            'window.restore': {
                notes: [{ v: 'chime', n: -10, t: 0, g: 0.9, pan: -0.15, t60: 0.2 }, { v: 'chime', n: -3, t: 0.065, g: 1, pan: 0.15, t60: 0.22 }],
                space: { size: 0.7, rt60: 0.3, damping: 6000, predelay: 0.008, mix: 0.16 }
            },
            'window.snap': {
                notes: [{ v: 'chime', n: -5, t: 0.002, g: 0.35, t60: 0.08, dur: 0.07, air: 0.08 }],
                layers: [{ v: 'tick', t: 0, g: 1, tone: 3000, ring: 0.025 }],
                space: { size: 0.4, rt60: 0.1, damping: 6500, predelay: 0.002, mix: 0.08 }
            },
            'window.deny': {
                notes: [{ v: 'chime', n: -7, t: 0.004, g: 0.35, pan: -0.1, t60: 0.14, damp: 1.1, attack: 0.006, air: 0.05 }, { v: 'chime', n: -7, t: 0.084, g: 0.4, pan: 0.1, t60: 0.2, damp: 1.1, attack: 0.006, air: 0.05 }],
                layers: [{ v: 'tick', material: 'felt', t: 0, g: 0.9, tone: 560, ring: 0.05, length: 0.008 }, { v: 'tick', material: 'felt', t: 0.08, g: 1, tone: 470, ring: 0.06, length: 0.008 }],
                space: { size: 0.45, rt60: 0.14, damping: 6500, predelay: 0.003, mix: 0.1 }
            },
            'file.drop': {
                notes: [{ v: 'chime', n: -5, t: 0.006, g: 0.45, pan: 0, t60: 0.22, damp: 1.1, attack: 0.006 }],
                layers: [{ v: 'tick', material: 'felt', t: 0, g: 1, tone: 480, ring: 0.07, length: 0.008 }],
                space: { size: 0.6, rt60: 0.3, damping: 6500, predelay: 0.005, mix: 0.16 }
            },
            'file.trash': {
                notes: [{ v: 'chime', n: 0, t: 0, g: 0.8, pan: 0.15, t60: 0.15 }, { v: 'chime', n: -5, t: 0.05, g: 0.9, pan: 0, t60: 0.15 },
                    { v: 'chime', n: -10, t: 0.1, g: 1, pan: -0.15, t60: 0.2 }],
                layers: [{ v: 'air', t: 0, g: 0.08, attack: 0.02, decay: 0.12 }],
                space: { size: 0.7, rt60: 0.4, damping: 5500, predelay: 0.006, mix: 0.16 }
            },
            'file.delete': {
                notes: [{ v: 'chime', n: -5, t: 0.004, g: 0.4, pan: 0.1, t60: 0.2, damp: 1.1, attack: 0.006 }, { v: 'chime', n: -12, t: 0.094, g: 0.5, pan: -0.1, t60: 0.26, damp: 1.1, attack: 0.006 }],
                layers: [{ v: 'tick', material: 'felt', t: 0, g: 0.8, tone: 650, ring: 0.05, length: 0.008 }, { v: 'tick', material: 'felt', t: 0.09, g: 1, tone: 440, ring: 0.07, length: 0.008 }],
                space: { size: 0.55, rt60: 0.25, damping: 6000, predelay: 0.004, mix: 0.12 }
            },
            'space.switch': {
                notes: [{ v: 'chime', n: 0, t: 0.05, g: 0.5, pan: 0, t60: 0.3, attack: 0.04 }],
                layers: [{ v: 'air', t: 0, g: 0.9, attack: 0.15, decay: 0.2, band: [1800, 6000] }],
                space: { size: 0.9, rt60: 0.45, damping: 6500, predelay: 0.01, mix: 0.2 }
            },
            'menu.open': {
                layers: [{ v: 'tick', t: 0, g: 0.9, tone: 2200, ring: 0.03 }, { v: 'tick', t: 0.03, g: 1, tone: 3000, ring: 0.03 }],
                space: { size: 0.45, rt60: 0.16, damping: 6500, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                layers: [{ v: 'tick', t: 0, g: 0.9, tone: 3000, ring: 0.03 }, { v: 'tick', t: 0.03, g: 1, tone: 2200, ring: 0.03 }],
                space: { size: 0.45, rt60: 0.12, damping: 6500, predelay: 0.002, mix: 0.08 }
            },
            'ui.select': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 3400, ring: 0.025 }],
                space: null
            },
            'ui.toggle.on': {
                layers: [{ v: 'tick', t: 0, g: 0.9, tone: 2400, ring: 0.03 }, { v: 'tick', t: 0.04, g: 1, tone: 3200, ring: 0.03 }],
                space: { size: 0.4, rt60: 0.1, damping: 6500, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.off': {
                layers: [{ v: 'tick', t: 0, g: 0.9, tone: 3200, ring: 0.03 }, { v: 'tick', t: 0.04, g: 1, tone: 2400, ring: 0.03 }],
                space: { size: 0.4, rt60: 0.1, damping: 6500, predelay: 0.002, mix: 0.08 }
            },
            'ui.tick': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 3000, ring: 0.018, length: 0.002 }],
                space: { size: 0.4, rt60: 0.06, damping: 6500, predelay: 0.002, mix: 0.06 }
            }
        },
        fx: {}
    });
})();
