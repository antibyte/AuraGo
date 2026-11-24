// Black Gloss: polished dark glass tones with soft, spacious tails and a soft bell as the second colour.
// Key D-flat 5 (73), major scale 0 2 4 5 7 9 11; held notes only on 0 2 4 7, the other degrees pass quickly.
// Darker, slower and rounder than crystal: tilted glass, 10-14 ms attacks, a smooth hall.
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'black_gloss',
        key: 73,
        space: { size: 1.1, rt60: 1.0, damping: 5200, predelay: 0.018, mix: 0.26 },
        voices: {
            glow: { model: 'glass', t60: 0.9, damp: 0.9, beat: 0.4, attack: 0.009, tilt: -0.4, width: 0.8, strike: 0.12, air: 0.05 },
            bell: { model: 'bell', kind: 'bell', t60: 0.8, index: 0.6, attack: 0.006, spread: 0.3 },
            low: { model: 'string', kind: 'muted', t60: 0.22, brightness: 0.16, pick: 0.12, body: 0.15 },
            tick: { model: 'click', material: 'glass', tone: 3000, ring: 0.03, noise: 0.55, q: 2.5 },
            air: { model: 'noise', color: 'pink', band: [700, 4200], attack: 0.07, decay: 0.22 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'glow', n: -5, t: 0, g: 0.8, pan: -0.25, t60: 0.9 }, { v: 'glow', n: 0, t: 0.2, g: 0.85, pan: 0.1, t60: 1 },
                    { v: 'glow', n: 4, t: 0.4, g: 0.9, pan: -0.1, t60: 1.1 }, { v: 'bell', n: 7, t: 0.62, g: 0.5, pan: 0.2, t60: 1 },
                    { v: 'glow', n: 7, t: 0.62, g: 0.9, pan: 0.2, t60: 1.2 }, { v: 'glow', n: 12, t: 0.9, g: 0.85, pan: 0, t60: 1.3, attack: 0.016 }],
                layers: [{ v: 'air', t: 0, g: 0.05 }],
                space: { size: 1.4, rt60: 1.5, damping: 5000, predelay: 0.025, mix: 0.34 }
            },
            'notify.message': {
                notes: [{ v: 'glow', n: -5, t: 0, g: 0.8, pan: -0.2, t60: 0.7 }, { v: 'glow', n: 0, t: 0.13, g: 0.9, pan: 0.15, t60: 0.8 },
                    { v: 'glow', n: 4, t: 0.26, g: 1, pan: 0, t60: 0.95 }, { v: 'bell', n: 4, t: 0.26, g: 0.35, pan: 0, t60: 0.6 }],
                space: { size: 1.1, rt60: 0.95, damping: 5200, predelay: 0.02, mix: 0.28 }
            },
            'notify.info': {
                notes: [{ v: 'glow', n: 0, t: 0, g: 0.85, pan: -0.15, t60: 0.5 }, { v: 'glow', n: 4, t: 0.12, g: 1, pan: 0.15, t60: 0.75 },
                    { v: 'bell', n: 4, t: 0.12, g: 0.35, pan: 0.15, t60: 0.5 }],
                space: { size: 1, rt60: 0.85, damping: 5200, predelay: 0.016, mix: 0.26 }
            },
            'notify.success': {
                notes: [{ v: 'glow', n: -8, t: 0, g: 0.8, pan: -0.2, t60: 0.5 }, { v: 'glow', n: -5, t: 0.1, g: 0.9, pan: 0.1, t60: 0.55 },
                    { v: 'glow', n: 0, t: 0.2, g: 1, pan: 0, t60: 0.7 }, { v: 'bell', n: 0, t: 0.2, g: 0.35, pan: 0, t60: 0.55 }],
                space: { size: 1, rt60: 0.8, damping: 5200, predelay: 0.014, mix: 0.26 }
            },
            'notify.warning': {
                notes: [{ v: 'glow', n: -5, t: 0, g: 1, pan: -0.1, t60: 0.4, tilt: -0.9, attack: 0.014 }, { v: 'bell', n: -5, t: 0, g: 0.3, pan: -0.1, t60: 0.3 },
                    { v: 'glow', n: -5, t: 0.2, g: 0.85, pan: 0.1, t60: 0.5, tilt: -0.9, attack: 0.014 }, { v: 'bell', n: -5, t: 0.2, g: 0.25, pan: 0.1, t60: 0.3 }],
                layers: [{ v: 'tick', t: 0, g: 0.3, tone: 2800 }],
                space: { size: 0.9, rt60: 0.8, damping: 4800, predelay: 0.01, mix: 0.22 }
            },
            'notify.error': {
                notes: [{ v: 'glow', n: -7, t: 0, g: 1, pan: -0.1, t60: 0.35, tilt: -1, damp: 1.2, attack: 0.012 },
                    { v: 'glow', n: -13, t: 0.16, g: 0.9, pan: 0.1, t60: 0.4, tilt: -1, damp: 1.2, attack: 0.014 },
                    { v: 'low', n: -13, t: 0.16, g: 0.5, pan: 0.1, t60: 0.3 }],
                layers: [{ v: 'knock', t: 0, g: 0.4 }, { v: 'knock', t: 0.16, g: 0.4 }],
                space: { size: 0.8, rt60: 0.5, damping: 4200, predelay: 0.008, mix: 0.2 }
            },
            'dialog.open': {
                notes: [{ v: 'glow', n: -8, t: 0, g: 0.85, pan: -0.15, t60: 0.3 }, { v: 'glow', n: -3, t: 0.09, g: 1, pan: 0.15, t60: 0.34 },
                    { v: 'bell', n: -3, t: 0.09, g: 0.3, pan: 0.15, t60: 0.2 }],
                space: { size: 0.9, rt60: 0.5, damping: 5500, predelay: 0.01, mix: 0.2 }
            },
            'dialog.confirm': {
                notes: [{ v: 'glow', n: -5, t: 0, g: 0.85, pan: -0.15, t60: 0.3 }, { v: 'glow', n: 4, t: 0.1, g: 1, pan: 0.15, t60: 0.38 },
                    { v: 'bell', n: 4, t: 0.1, g: 0.3, pan: 0.15, t60: 0.2 }],
                space: { size: 0.9, rt60: 0.55, damping: 5500, predelay: 0.01, mix: 0.2 }
            },
            'dialog.cancel': {
                notes: [{ v: 'glow', n: 4, t: 0, g: 0.85, pan: 0.15, t60: 0.28 }, { v: 'glow', n: -5, t: 0.1, g: 1, pan: -0.15, t60: 0.3 },
                    { v: 'bell', n: -5, t: 0.1, g: 0.25, pan: -0.15, t60: 0.18 }],
                space: { size: 0.9, rt60: 0.4, damping: 4800, predelay: 0.01, mix: 0.18 }
            },
            'window.open': {
                notes: [{ v: 'glow', n: -5, t: 0, g: 0.9, pan: -0.15, t60: 0.28, attack: 0.01 }, { v: 'glow', n: 0, t: 0.09, g: 1, pan: 0.2, t60: 0.3, attack: 0.01 },
                    { v: 'bell', n: 0, t: 0.09, g: 0.25, pan: 0.2, t60: 0.18 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.12 }],
                space: { size: 0.9, rt60: 0.4, damping: 5800, predelay: 0.01, mix: 0.2 }
            },
            'window.close': {
                notes: [{ v: 'glow', n: 0, t: 0, g: 0.9, pan: 0.2, t60: 0.24, attack: 0.01 }, { v: 'glow', n: -5, t: 0.09, g: 1, pan: -0.15, t60: 0.26, attack: 0.01 },
                    { v: 'bell', n: -5, t: 0.09, g: 0.25, pan: -0.15, t60: 0.16 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.12 }],
                space: { size: 0.9, rt60: 0.35, damping: 5000, predelay: 0.01, mix: 0.2 }
            },
            'window.maximize': {
                notes: [{ v: 'glow', n: -12, t: 0, g: 0.85, pan: -0.2, t60: 0.3 }, { v: 'glow', n: -3, t: 0.11, g: 1, pan: 0.2, t60: 0.34 },
                    { v: 'bell', n: -3, t: 0.11, g: 0.3, pan: 0.2, t60: 0.2 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.12 }],
                space: { size: 1.1, rt60: 0.5, damping: 5800, predelay: 0.012, mix: 0.26 }
            },
            'window.minimize': {
                notes: [{ v: 'glow', n: -3, t: 0, g: 0.9, pan: 0.15, t60: 0.18, attack: 0.008 }, { v: 'glow', n: -8, t: 0.07, g: 1, pan: -0.15, t60: 0.2, attack: 0.008 }],
                layers: [{ v: 'tick', t: 0, g: 0.25, tone: 2800 }],
                space: { size: 0.7, rt60: 0.3, damping: 5000, predelay: 0.008, mix: 0.16 }
            },
            'window.restore': {
                notes: [{ v: 'glow', n: -8, t: 0, g: 0.9, pan: -0.15, t60: 0.18, attack: 0.008 }, { v: 'glow', n: -3, t: 0.07, g: 1, pan: 0.15, t60: 0.2, attack: 0.008 }],
                layers: [{ v: 'tick', t: 0, g: 0.25, tone: 2800 }],
                space: { size: 0.7, rt60: 0.3, damping: 5000, predelay: 0.008, mix: 0.16 }
            },
            'window.snap': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 3000, ring: 0.02 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'low', n: -12, t: 0, g: 0.8, pan: 0, t60: 0.16, attack: 0.006 },
                    { v: 'glow', n: -12, t: 0, g: 0.8, pan: 0, t60: 0.2, tilt: -1, damp: 1.5, attack: 0.012 },
                    { v: 'bell', n: -12, t: 0, g: 0.25, pan: 0, t60: 0.12 }],
                layers: [{ v: 'knock', t: 0, g: 0.3 }],
                space: { size: 0.6, rt60: 0.25, damping: 4200, predelay: 0.004, mix: 0.12 }
            },
            'file.drop': {
                notes: [{ v: 'low', n: -7, t: 0, g: 0.9, pan: 0, t60: 0.24 }, { v: 'glow', n: -5, t: 0.02, g: 0.9, pan: 0, t60: 0.2, tilt: -1, attack: 0.01 }],
                layers: [{ v: 'knock', t: 0, g: 0.5 }],
                space: { size: 0.8, rt60: 0.4, damping: 4500, predelay: 0.006, mix: 0.18 }
            },
            'file.trash': {
                notes: [{ v: 'glow', n: 0, t: 0, g: 0.8, pan: 0.15, t60: 0.16, attack: 0.008 }, { v: 'glow', n: -5, t: 0.055, g: 0.9, pan: 0, t60: 0.16, attack: 0.008 },
                    { v: 'glow', n: -8, t: 0.11, g: 1, pan: -0.15, t60: 0.22, attack: 0.008 }],
                layers: [{ v: 'air', t: 0, g: 0.08, attack: 0.02, decay: 0.12 }],
                space: { size: 0.8, rt60: 0.4, damping: 5000, predelay: 0.006, mix: 0.16 }
            },
            'file.delete': {
                notes: [{ v: 'glow', n: -8, t: 0, g: 0.85, pan: 0.1, t60: 0.3, tilt: -0.9 }, { v: 'low', n: -12, t: 0.1, g: 1, pan: -0.1, t60: 0.3 },
                    { v: 'glow', n: -12, t: 0.1, g: 0.5, pan: -0.1, t60: 0.34, tilt: -1, attack: 0.012 }],
                layers: [{ v: 'knock', t: 0.1, g: 0.4 }],
                space: { size: 0.9, rt60: 0.5, damping: 4500, predelay: 0.008, mix: 0.2 }
            },
            'space.switch': {
                notes: [{ v: 'glow', n: 0, t: 0.05, g: 0.5, pan: 0, t60: 0.3, attack: 0.05 }],
                layers: [{ v: 'air', t: 0, g: 0.9, attack: 0.15, decay: 0.2, band: [600, 3500] }],
                space: { size: 1, rt60: 0.5, damping: 5500, predelay: 0.01, mix: 0.22 }
            },
            'menu.open': {
                notes: [{ v: 'glow', n: -8, t: 0, g: 0.9, pan: -0.1, t60: 0.1, air: 0, dur: 0.09, attack: 0.005 }, { v: 'glow', n: -3, t: 0.035, g: 1, pan: 0.1, t60: 0.1, air: 0, dur: 0.09, attack: 0.005 }],
                layers: [{ v: 'tick', t: 0, g: 0.25, tone: 2600 }],
                space: { size: 0.4, rt60: 0.12, damping: 6000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'glow', n: -3, t: 0, g: 0.9, pan: 0.1, t60: 0.1, air: 0, dur: 0.09, attack: 0.005 }, { v: 'glow', n: -8, t: 0.03, g: 1, pan: -0.1, t60: 0.1, air: 0, dur: 0.09, attack: 0.005 }],
                layers: [{ v: 'tick', t: 0, g: 0.25, tone: 2600 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.select': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 2800, ring: 0.03 }],
                space: null
            },
            'ui.toggle.on': {
                notes: [{ v: 'glow', n: -3, t: 0, g: 0.25, t60: 0.06, dur: 0.07, attack: 0.004, air: 0 }, { v: 'glow', n: 4, t: 0.035, g: 0.3, t60: 0.06, dur: 0.07, attack: 0.004, air: 0 }],
                layers: [{ v: 'tick', t: 0, g: 0.9, tone: 2300, ring: 0.025 }, { v: 'tick', t: 0.035, g: 1, tone: 3100, ring: 0.025 }],
                space: null
            },
            'ui.toggle.off': {
                notes: [{ v: 'glow', n: 4, t: 0, g: 0.25, t60: 0.06, dur: 0.07, attack: 0.004, air: 0 }, { v: 'glow', n: -3, t: 0.035, g: 0.3, t60: 0.06, dur: 0.07, attack: 0.004, air: 0 }],
                layers: [{ v: 'tick', t: 0, g: 0.9, tone: 3100, ring: 0.025 }, { v: 'tick', t: 0.035, g: 1, tone: 2300, ring: 0.025 }],
                space: null
            },
            'ui.tick': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 4200, ring: 0.014, length: 0.0015 }],
                space: null
            }
        },
        fx: {}
    });
})();
