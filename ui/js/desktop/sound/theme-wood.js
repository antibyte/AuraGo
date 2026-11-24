// Wood: warm rosewood marimba taps, felt clicks and a soft small room. Key F4 (65), pentatonic 0 2 4 7 9.
// Frequent events (window, menu, touch) sit around and above the key; two- and three-note gestures read as an instrument.
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'wood',
        key: 65,
        space: { size: 0.55, rt60: 0.3, damping: 6000, predelay: 0.006, mix: 0.16 },
        voices: {
            mar: { model: 'bar', kind: 'marimba', t60: 0.4, damp: 1.2, hardness: 0.3, attack: 0.002, mallet: 0.3, body: 0.2 },
            tap: { model: 'bar', kind: 'woodblock', t60: 0.09, hardness: 0.25, attack: 0.002, mallet: 0.3 },
            low: { model: 'string', kind: 'muted', t60: 0.2, brightness: 0.18, pick: 0.15, body: 0.15 },
            felt: { model: 'click', material: 'felt', tone: 1500, ring: 0.014 },
            air: { model: 'noise', color: 'pink', band: [900, 4500], attack: 0.03, decay: 0.12 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'mar', n: -5, t: 0, g: 0.8, pan: -0.25, t60: 0.6 }, { v: 'mar', n: 0, t: 0.18, g: 0.85, pan: 0.1, t60: 0.65 },
                    { v: 'mar', n: 4, t: 0.36, g: 0.9, pan: -0.1, t60: 0.7 }, { v: 'mar', n: 7, t: 0.54, g: 0.9, pan: 0.2, t60: 0.8 },
                    { v: 'mar', n: 12, t: 0.8, g: 0.85, pan: 0, t60: 1.1 }],
                layers: [{ v: 'air', t: 0, g: 0.05 }],
                space: { size: 0.9, rt60: 0.9, damping: 5500, predelay: 0.015, mix: 0.28 }
            },
            'notify.message': {
                notes: [{ v: 'mar', n: -5, t: 0, g: 0.8, pan: -0.2, t60: 0.5 }, { v: 'mar', n: 0, t: 0.12, g: 0.9, pan: 0.15, t60: 0.55 },
                    { v: 'mar', n: 4, t: 0.24, g: 1, pan: 0, t60: 0.7 },
                    { v: 'tap', n: 4, t: 0.24, g: 0.4, pan: 0, t60: 0.05 }],
                layers: [{ v: 'felt', t: 0, g: 0.4, tone: 1500 }],
                space: { size: 0.8, rt60: 0.6, damping: 5500, predelay: 0.012, mix: 0.24 }
            },
            'notify.info': {
                notes: [{ v: 'mar', n: 0, t: 0, g: 0.85, pan: -0.15, t60: 0.3 }, { v: 'mar', n: 4, t: 0.11, g: 1, pan: 0.15, t60: 0.4, hardness: 0.4 },
                    { v: 'tap', n: 4, t: 0.11, g: 0.4, pan: 0.15, t60: 0.05 }],
                layers: [{ v: 'felt', t: 0, g: 0.4, tone: 1500 }],
                space: { size: 0.7, rt60: 0.5, damping: 5500, predelay: 0.01, mix: 0.2 }
            },
            'notify.success': {
                notes: [{ v: 'mar', n: -8, t: 0, g: 0.8, pan: -0.2, t60: 0.4 }, { v: 'mar', n: -3, t: 0.1, g: 0.9, pan: 0.1, t60: 0.45 },
                    { v: 'mar', n: 0, t: 0.2, g: 1, pan: 0, t60: 0.6 }, { v: 'tap', n: 0, t: 0.2, g: 0.4, pan: 0, t60: 0.05 }],
                layers: [{ v: 'felt', t: 0, g: 0.4, tone: 1500 }],
                space: { size: 0.75, rt60: 0.55, damping: 5500, predelay: 0.01, mix: 0.22 }
            },
            'notify.warning': {
                notes: [{ v: 'mar', n: -3, t: 0, g: 1, pan: -0.1, t60: 0.25, hardness: 0.2, attack: 0.004 }, { v: 'tap', n: -3, t: 0, g: 0.6, pan: -0.1, t60: 0.06 },
                    { v: 'mar', n: -3, t: 0.18, g: 0.8, pan: 0.1, t60: 0.35, hardness: 0.35, attack: 0.004 }, { v: 'tap', n: -3, t: 0.18, g: 0.5, pan: 0.1, t60: 0.06 }],
                layers: [{ v: 'felt', t: 0, g: 0.5, tone: 1400 }, { v: 'felt', t: 0.18, g: 0.4, tone: 1400 }],
                space: { size: 0.7, rt60: 0.5, damping: 5000, predelay: 0.01, mix: 0.2 }
            },
            'notify.error': {
                notes: [{ v: 'mar', n: 0, t: 0, g: 1, pan: -0.1, t60: 0.3, hardness: 0.2, attack: 0.005 },
                    { v: 'low', n: -6, t: 0, g: 0.4, pan: -0.1, t60: 0.14, attack: 0.006 },
                    { v: 'mar', n: -6, t: 0.15, g: 1, pan: 0.1, t60: 0.38, hardness: 0.2, attack: 0.005 },
                    { v: 'low', n: -12, t: 0.15, g: 0.4, pan: 0.1, t60: 0.16, attack: 0.006 }],
                layers: [{ v: 'knock', t: 0, g: 0.15 }, { v: 'knock', t: 0.15, g: 0.15 }],
                space: { size: 0.6, rt60: 0.4, damping: 4500, predelay: 0.006, mix: 0.16 }
            },
            'dialog.open': {
                notes: [{ v: 'mar', n: -5, t: 0, g: 0.85, pan: -0.15, t60: 0.2 }, { v: 'mar', n: 0, t: 0.08, g: 1, pan: 0.15, t60: 0.26 }],
                space: { size: 0.6, rt60: 0.35, damping: 5500, predelay: 0.008, mix: 0.16 }
            },
            'dialog.confirm': {
                notes: [{ v: 'mar', n: 0, t: 0, g: 0.85, pan: -0.15, t60: 0.22 }, { v: 'mar', n: 7, t: 0.09, g: 1, pan: 0.15, t60: 0.3 }, { v: 'tap', n: 7, t: 0.09, g: 0.4, pan: 0.15, t60: 0.05 }],
                layers: [{ v: 'felt', t: 0, g: 0.4, tone: 1500 }],
                space: { size: 0.6, rt60: 0.4, damping: 5500, predelay: 0.008, mix: 0.16 }
            },
            'dialog.cancel': {
                notes: [{ v: 'mar', n: 7, t: 0, g: 0.85, pan: 0.15, t60: 0.2 }, { v: 'mar', n: 0, t: 0.09, g: 1, pan: -0.15, t60: 0.22 }, { v: 'tap', n: 0, t: 0.09, g: 0.4, pan: -0.15, t60: 0.05 }],
                layers: [{ v: 'felt', t: 0, g: 0.4, tone: 1500 }],
                space: { size: 0.6, rt60: 0.3, damping: 4800, predelay: 0.008, mix: 0.14 }
            },
            'window.open': {
                notes: [{ v: 'mar', n: -3, t: 0, g: 0.9, pan: -0.15, t60: 0.2 }, { v: 'mar', n: 4, t: 0.085, g: 1, pan: 0.2, t60: 0.24 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.1 }],
                space: { size: 0.6, rt60: 0.3, damping: 5500, predelay: 0.008, mix: 0.16 }
            },
            'window.close': {
                notes: [{ v: 'mar', n: 4, t: 0, g: 0.9, pan: 0.2, t60: 0.18 }, { v: 'mar', n: -3, t: 0.085, g: 1, pan: -0.15, t60: 0.2 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.1 }],
                space: { size: 0.6, rt60: 0.25, damping: 5000, predelay: 0.008, mix: 0.14 }
            },
            'window.maximize': {
                notes: [{ v: 'mar', n: -8, t: 0, g: 0.85, pan: -0.2, t60: 0.22 }, { v: 'mar', n: 4, t: 0.1, g: 1, pan: 0.2, t60: 0.3 }, { v: 'tap', n: 4, t: 0.1, g: 0.4, pan: 0.2, t60: 0.05 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.1 }],
                space: { size: 0.8, rt60: 0.4, damping: 5500, predelay: 0.01, mix: 0.2 }
            },
            'window.minimize': {
                notes: [{ v: 'mar', n: 2, t: 0, g: 0.9, pan: 0.15, t60: 0.14 }, { v: 'mar', n: -5, t: 0.065, g: 1, pan: -0.15, t60: 0.16 }],
                space: { size: 0.5, rt60: 0.2, damping: 5000, predelay: 0.006, mix: 0.12 }
            },
            'window.restore': {
                notes: [{ v: 'mar', n: -5, t: 0, g: 0.9, pan: -0.15, t60: 0.14 }, { v: 'mar', n: 2, t: 0.065, g: 1, pan: 0.15, t60: 0.16 }],
                space: { size: 0.5, rt60: 0.2, damping: 5000, predelay: 0.006, mix: 0.12 }
            },
            'window.snap': {
                layers: [{ v: 'felt', t: 0, g: 1, tone: 2200, ring: 0.012 }],
                notes: [{ v: 'tap', n: 7, t: 0, g: 0.5, t60: 0.05 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'low', n: -7, t: 0, g: 0.8, pan: 0, t60: 0.16, attack: 0.006 },
                    { v: 'mar', n: -7, t: 0, g: 0.9, pan: 0, t60: 0.2, hardness: 0.15, attack: 0.005, damp: 2 }],
                layers: [{ v: 'knock', t: 0, g: 0.3 }],
                space: { size: 0.5, rt60: 0.22, damping: 4500, predelay: 0.004, mix: 0.1 }
            },
            'file.drop': {
                notes: [{ v: 'low', n: -7, t: 0, g: 0.9, pan: 0, t60: 0.22 }, { v: 'tap', n: -3, t: 0.03, g: 0.9, t60: 0.08 }],
                layers: [{ v: 'knock', t: 0, g: 0.4 }],
                space: { size: 0.6, rt60: 0.3, damping: 5000, predelay: 0.006, mix: 0.14 }
            },
            'file.trash': {
                notes: [{ v: 'mar', n: 4, t: 0, g: 0.8, pan: 0.15, t60: 0.12 }, { v: 'mar', n: -3, t: 0.05, g: 0.9, pan: 0, t60: 0.12 },
                    { v: 'mar', n: -8, t: 0.1, g: 1, pan: -0.15, t60: 0.16 }],
                layers: [{ v: 'air', t: 0, g: 0.08, attack: 0.02, decay: 0.1 }],
                space: { size: 0.55, rt60: 0.25, damping: 5000, predelay: 0.006, mix: 0.14 }
            },
            'file.delete': {
                notes: [{ v: 'mar', n: -5, t: 0, g: 0.85, pan: 0.1, t60: 0.2 }, { v: 'low', n: -12, t: 0.09, g: 1, pan: -0.1, t60: 0.26 }],
                layers: [{ v: 'knock', t: 0.09, g: 0.4 }],
                space: { size: 0.65, rt60: 0.35, damping: 4800, predelay: 0.008, mix: 0.16 }
            },
            'space.switch': {
                notes: [{ v: 'mar', n: 0, t: 0.05, g: 0.5, pan: 0, t60: 0.25, attack: 0.02 }],
                layers: [{ v: 'air', t: 0, g: 0.8, attack: 0.12, decay: 0.2, band: [700, 3500] }],
                space: { size: 0.7, rt60: 0.35, damping: 5500, predelay: 0.008, mix: 0.18 }
            },
            'menu.open': {
                notes: [{ v: 'mar', n: 0, t: 0, g: 0.9, pan: -0.1, t60: 0.08, air: 0, dur: 0.09 }, { v: 'mar', n: 7, t: 0.035, g: 1, pan: 0.1, t60: 0.08, air: 0, dur: 0.09 }],
                space: { size: 0.4, rt60: 0.1, damping: 6000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'mar', n: 7, t: 0, g: 0.9, pan: 0.1, t60: 0.08, air: 0, dur: 0.09 }, { v: 'mar', n: 0, t: 0.03, g: 1, pan: -0.1, t60: 0.08, air: 0, dur: 0.09 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.select': {
                layers: [{ v: 'felt', t: 0, g: 1, tone: 1800, ring: 0.014 }],
                notes: [{ v: 'tap', n: 4, t: 0, g: 0.4, t60: 0.04, dur: 0.06 }],
                space: { size: 0.4, rt60: 0.06, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'felt', t: 0, g: 0.9, tone: 1700, ring: 0.012 }, { v: 'felt', t: 0.04, g: 1, tone: 2600, ring: 0.012 }],
                space: { size: 0.4, rt60: 0.06, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.off': {
                layers: [{ v: 'felt', t: 0, g: 0.9, tone: 2600, ring: 0.012 }, { v: 'felt', t: 0.04, g: 1, tone: 1700, ring: 0.012 }],
                space: { size: 0.4, rt60: 0.06, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.tick': {
                layers: [{ v: 'felt', t: 0, g: 1, tone: 3000, ring: 0.008, length: 0.002 }],
                space: null
            }
        },
        fx: {}
    });
})();
