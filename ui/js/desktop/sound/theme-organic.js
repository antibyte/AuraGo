// Organic: kalimba tines, a log drum, a nylon string, soft breath and a gentle water drop in a soft room.
// Key G3 (55), minor pentatonic 0 3 5 7 10. The key sits low, so melodies live one or two octaves above it (n 5..24).
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'organic',
        key: 55,
        space: { size: 0.75, rt60: 0.45, damping: 5500, predelay: 0.01, mix: 0.2 },
        voices: {
            kal: { model: 'bar', kind: 'kalimba', t60: 0.6, hardness: 0.3, attack: 0.002, mallet: 0.3 },
            log: { model: 'bar', kind: 'woodblock', t60: 0.14, hardness: 0.2, attack: 0.003, mallet: 0.35 },
            str: { model: 'string', kind: 'nylon', t60: 0.5, brightness: 0.3, pick: 0.2, body: 0.3 },
            low: { model: 'string', kind: 'muted', t60: 0.22, brightness: 0.16, pick: 0.15, body: 0.2 },
            wdrop: { model: 'drop', rise: 0.5, t60: 0.07, attack: 0.003, splash: 0.1, width: 0.4 },
            felt: { model: 'click', material: 'felt', tone: 1600, ring: 0.014 },
            breath: { model: 'noise', color: 'pink', band: [500, 3200], attack: 0.07, decay: 0.2 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'kal', n: 7, t: 0, g: 0.8, pan: -0.25, t60: 0.8 }, { v: 'kal', n: 12, t: 0.18, g: 0.85, pan: 0.1, t60: 0.85 },
                    { v: 'kal', n: 15, t: 0.36, g: 0.9, pan: -0.1, t60: 0.9 }, { v: 'str', n: 19, t: 0.54, g: 0.7, pan: 0.2, t60: 0.9 },
                    { v: 'kal', n: 19, t: 0.54, g: 0.8, pan: 0.2, t60: 1 }, { v: 'kal', n: 24, t: 0.8, g: 0.8, pan: 0, t60: 1.2 },
                    { v: 'wdrop', n: 31, t: 0.95, g: 0.3, pan: 0.3 }],
                layers: [{ v: 'breath', t: 0, g: 0.05 }],
                space: { size: 1.1, rt60: 1.0, damping: 5000, predelay: 0.02, mix: 0.3 }
            },
            'notify.message': {
                notes: [{ v: 'kal', n: 7, t: 0, g: 0.8, pan: -0.2, t60: 0.6 }, { v: 'kal', n: 12, t: 0.12, g: 0.9, pan: 0.15, t60: 0.65 },
                    { v: 'kal', n: 15, t: 0.24, g: 1, pan: 0, t60: 0.8 }, { v: 'str', n: 15, t: 0.24, g: 0.5, pan: 0, t60: 0.5 }],
                space: { size: 0.9, rt60: 0.7, damping: 5200, predelay: 0.014, mix: 0.26 }
            },
            'notify.info': {
                notes: [{ v: 'kal', n: 12, t: 0, g: 0.85, pan: -0.15, t60: 0.45 }, { v: 'kal', n: 17, t: 0.11, g: 1, pan: 0.15, t60: 0.6 },
                    { v: 'log', n: 17, t: 0.11, g: 0.4, pan: 0.15, t60: 0.06 }],
                layers: [{ v: 'felt', t: 0, g: 0.35 }],
                space: { size: 0.8, rt60: 0.6, damping: 5200, predelay: 0.012, mix: 0.24 }
            },
            'notify.success': {
                notes: [{ v: 'kal', n: 7, t: 0, g: 0.8, pan: -0.2, t60: 0.45 }, { v: 'kal', n: 10, t: 0.1, g: 0.9, pan: 0.1, t60: 0.5 },
                    { v: 'kal', n: 12, t: 0.2, g: 1, pan: 0, t60: 0.7 }, { v: 'wdrop', n: 24, t: 0.2, g: 0.3, pan: 0.2 }],
                layers: [{ v: 'felt', t: 0, g: 0.3 }],
                space: { size: 0.85, rt60: 0.65, damping: 5200, predelay: 0.012, mix: 0.24 }
            },
            'notify.warning': {
                notes: [{ v: 'kal', n: 10, t: 0, g: 1, pan: -0.1, t60: 0.3, hardness: 0.2 }, { v: 'log', n: 10, t: 0, g: 0.9, pan: -0.1, t60: 0.08 }, { v: 'low', n: 10, t: 0, g: 0.6, pan: -0.1, t60: 0.2 },
                    { v: 'kal', n: 10, t: 0.19, g: 0.8, pan: 0.1, t60: 0.4, hardness: 0.35 }, { v: 'log', n: 10, t: 0.19, g: 0.7, pan: 0.1, t60: 0.08 }, { v: 'low', n: 10, t: 0.19, g: 0.5, pan: 0.1, t60: 0.25 }],
                layers: [{ v: 'felt', t: 0, g: 0.5, tone: 1400 }, { v: 'felt', t: 0.19, g: 0.4, tone: 1400 }],
                space: { size: 0.75, rt60: 0.5, damping: 4800, predelay: 0.01, mix: 0.2 }
            },
            'notify.error': {
                notes: [{ v: 'kal', n: 15, t: 0, g: 1, pan: -0.1, t60: 0.35, hardness: 0.15, attack: 0.005 },
                    { v: 'low', n: 15, t: 0, g: 0.4, pan: -0.1, t60: 0.14, attack: 0.006 },
                    { v: 'kal', n: 9, t: 0.16, g: 1, pan: 0.1, t60: 0.45, hardness: 0.15, attack: 0.005 },
                    { v: 'low', n: 9, t: 0.16, g: 0.4, pan: 0.1, t60: 0.16, attack: 0.006 }],
                layers: [{ v: 'knock', t: 0, g: 0.15 }, { v: 'knock', t: 0.16, g: 0.15 }],
                space: { size: 0.65, rt60: 0.4, damping: 4500, predelay: 0.006, mix: 0.16 }
            },
            'dialog.open': {
                notes: [{ v: 'kal', n: 5, t: 0, g: 0.85, pan: -0.15, t60: 0.25 }, { v: 'kal', n: 12, t: 0.08, g: 1, pan: 0.15, t60: 0.3 },
                    { v: 'log', n: 12, t: 0.08, g: 0.35, pan: 0.15, t60: 0.05 }],
                space: { size: 0.65, rt60: 0.4, damping: 5500, predelay: 0.008, mix: 0.18 }
            },
            'dialog.confirm': {
                notes: [{ v: 'kal', n: 7, t: 0, g: 0.85, pan: -0.15, t60: 0.25 }, { v: 'kal', n: 15, t: 0.09, g: 1, pan: 0.15, t60: 0.32 },
                    { v: 'log', n: 15, t: 0.09, g: 0.35, pan: 0.15, t60: 0.05 }],
                layers: [{ v: 'felt', t: 0, g: 0.35 }],
                space: { size: 0.65, rt60: 0.45, damping: 5500, predelay: 0.008, mix: 0.18 }
            },
            'dialog.cancel': {
                notes: [{ v: 'kal', n: 15, t: 0, g: 0.85, pan: 0.15, t60: 0.22 }, { v: 'kal', n: 7, t: 0.09, g: 1, pan: -0.15, t60: 0.26 },
                    { v: 'log', n: 7, t: 0.09, g: 0.35, pan: -0.15, t60: 0.05 }],
                layers: [{ v: 'felt', t: 0, g: 0.35 }],
                space: { size: 0.65, rt60: 0.35, damping: 4800, predelay: 0.008, mix: 0.16 }
            },
            'window.open': {
                notes: [{ v: 'kal', n: 7, t: 0, g: 0.9, pan: -0.15, t60: 0.22 }, { v: 'kal', n: 12, t: 0.085, g: 1, pan: 0.2, t60: 0.26 },
                    { v: 'log', n: 12, t: 0.085, g: 0.3, pan: 0.2, t60: 0.05 }],
                layers: [{ v: 'breath', t: 0, g: 0.05, decay: 0.1 }],
                space: { size: 0.7, rt60: 0.35, damping: 5500, predelay: 0.008, mix: 0.18 }
            },
            'window.close': {
                notes: [{ v: 'kal', n: 12, t: 0, g: 0.9, pan: 0.2, t60: 0.2 }, { v: 'kal', n: 7, t: 0.085, g: 1, pan: -0.15, t60: 0.22 },
                    { v: 'log', n: 7, t: 0.085, g: 0.3, pan: -0.15, t60: 0.05 }],
                layers: [{ v: 'breath', t: 0, g: 0.05, decay: 0.1 }],
                space: { size: 0.7, rt60: 0.3, damping: 5000, predelay: 0.008, mix: 0.16 }
            },
            'window.maximize': {
                notes: [{ v: 'kal', n: 3, t: 0, g: 0.85, pan: -0.2, t60: 0.25 }, { v: 'kal', n: 15, t: 0.1, g: 1, pan: 0.2, t60: 0.32 },
                    { v: 'log', n: 15, t: 0.1, g: 0.3, pan: 0.2, t60: 0.05 }],
                layers: [{ v: 'breath', t: 0, g: 0.05, decay: 0.1 }],
                space: { size: 0.9, rt60: 0.45, damping: 5500, predelay: 0.01, mix: 0.24 }
            },
            'window.minimize': {
                notes: [{ v: 'kal', n: 10, t: 0, g: 0.9, pan: 0.15, t60: 0.15 }, { v: 'kal', n: 5, t: 0.065, g: 1, pan: -0.15, t60: 0.17 }],
                layers: [{ v: 'felt', t: 0, g: 0.3 }],
                space: { size: 0.55, rt60: 0.22, damping: 5000, predelay: 0.006, mix: 0.14 }
            },
            'window.restore': {
                notes: [{ v: 'kal', n: 5, t: 0, g: 0.9, pan: -0.15, t60: 0.15 }, { v: 'kal', n: 10, t: 0.065, g: 1, pan: 0.15, t60: 0.17 }],
                layers: [{ v: 'felt', t: 0, g: 0.3 }],
                space: { size: 0.55, rt60: 0.22, damping: 5000, predelay: 0.006, mix: 0.14 }
            },
            'window.snap': {
                layers: [{ v: 'felt', t: 0, g: 1, tone: 2200, ring: 0.012 }],
                notes: [{ v: 'kal', n: 17, t: 0, g: 0.9, t60: 0.1, dur: 0.07 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'log', n: 5, t: 0, g: 0.9, pan: 0, t60: 0.12 }, { v: 'low', n: 5, t: 0, g: 0.7, pan: 0, t60: 0.14 }],
                layers: [{ v: 'knock', t: 0, g: 0.5 }],
                space: { size: 0.5, rt60: 0.22, damping: 4500, predelay: 0.004, mix: 0.1 }
            },
            'file.drop': {
                notes: [{ v: 'low', n: 5, t: 0, g: 0.9, pan: 0, t60: 0.24 }, { v: 'log', n: 5, t: 0.02, g: 0.9, t60: 0.1 },
                    { v: 'kal', n: 12, t: 0.04, g: 0.5, t60: 0.2, hardness: 0.15 }],
                layers: [{ v: 'knock', t: 0, g: 0.35 }],
                space: { size: 0.65, rt60: 0.3, damping: 5000, predelay: 0.006, mix: 0.16 }
            },
            'file.trash': {
                notes: [{ v: 'kal', n: 15, t: 0, g: 0.8, pan: 0.15, t60: 0.14 }, { v: 'kal', n: 10, t: 0.05, g: 0.9, pan: 0, t60: 0.14 },
                    { v: 'kal', n: 5, t: 0.1, g: 1, pan: -0.15, t60: 0.2 }, { v: 'log', n: 5, t: 0.1, g: 0.4, pan: -0.15, t60: 0.06 }],
                layers: [{ v: 'breath', t: 0, g: 0.08, attack: 0.02, decay: 0.1 }],
                space: { size: 0.6, rt60: 0.3, damping: 5000, predelay: 0.006, mix: 0.16 }
            },
            'file.delete': {
                notes: [{ v: 'kal', n: 10, t: 0, g: 0.85, pan: 0.1, t60: 0.22 }, { v: 'low', n: 5, t: 0.09, g: 1, pan: -0.1, t60: 0.3 },
                    { v: 'log', n: 0, t: 0.09, g: 0.8, pan: -0.1, t60: 0.14 }],
                layers: [{ v: 'knock', t: 0.09, g: 0.4 }],
                space: { size: 0.75, rt60: 0.4, damping: 4800, predelay: 0.008, mix: 0.2 }
            },
            'space.switch': {
                notes: [{ v: 'wdrop', n: 24, t: 0.05, g: 0.6, pan: -0.2, count: 3, spread: 0.06, t60: 0.08 }, { v: 'kal', n: 12, t: 0.1, g: 0.35, pan: 0.1, t60: 0.3, hardness: 0.1 }],
                layers: [{ v: 'breath', t: 0, g: 0.9, attack: 0.12, decay: 0.2, band: [400, 2800] }],
                space: { size: 0.85, rt60: 0.45, damping: 5200, predelay: 0.01, mix: 0.22 }
            },
            'menu.open': {
                notes: [{ v: 'kal', n: 12, t: 0, g: 0.9, pan: -0.1, t60: 0.08, air: 0, dur: 0.09 }, { v: 'kal', n: 19, t: 0.035, g: 1, pan: 0.1, t60: 0.08, air: 0, dur: 0.09 }],
                layers: [{ v: 'felt', t: 0, g: 0.25 }],
                space: { size: 0.4, rt60: 0.12, damping: 6000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'kal', n: 19, t: 0, g: 0.9, pan: 0.1, t60: 0.08, air: 0, dur: 0.09 }, { v: 'kal', n: 12, t: 0.03, g: 1, pan: -0.1, t60: 0.08, air: 0, dur: 0.09 }],
                layers: [{ v: 'felt', t: 0, g: 0.25 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.select': {
                notes: [{ v: 'log', n: 17, t: 0, g: 0.4, t60: 0.04, dur: 0.06 }],
                layers: [{ v: 'felt', t: 0, g: 1, tone: 1800, ring: 0.014 }],
                space: { size: 0.4, rt60: 0.06, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'felt', t: 0, g: 0.9, tone: 1700, ring: 0.012 }, { v: 'felt', t: 0.04, g: 1, tone: 2600, ring: 0.012 }],
                notes: [{ v: 'log', n: 17, t: 0.04, g: 0.35, t60: 0.04, dur: 0.06 }],
                space: { size: 0.4, rt60: 0.06, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.off': {
                layers: [{ v: 'felt', t: 0, g: 0.9, tone: 2600, ring: 0.012 }, { v: 'felt', t: 0.04, g: 1, tone: 1700, ring: 0.012 }],
                notes: [{ v: 'log', n: 12, t: 0.04, g: 0.35, t60: 0.04, dur: 0.06 }],
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
