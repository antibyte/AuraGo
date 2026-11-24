// Organic: a plucked nylon string, an intake of breath and a drop of water in a soft room.
// Key G3 (55), minor pentatonic 0 3 5 7 10. The lead is the string; the kalimba is only a quiet second colour.
// Gestures are rolled like a strummed pair (25-45 ms apart, the second note louder); a breath swell starts before the first note.
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'organic',
        key: 55,
        space: { size: 0.75, rt60: 0.45, damping: 5500, predelay: 0.01, mix: 0.2 },
        voices: {
            nyl: { model: 'string', kind: 'nylon', t60: 0.5, brightness: 0.3, pick: 0.12, body: 0.4, attack: 0.004 },
            harp: { model: 'string', kind: 'harp', t60: 0.6, brightness: 0.4, pick: 0.2, body: 0.25, attack: 0.002 },
            low: { model: 'string', kind: 'muted', t60: 0.22, brightness: 0.16, pick: 0.15, body: 0.4, attack: 0.004 },
            kal: { model: 'bar', kind: 'kalimba', t60: 0.5, hardness: 0.25, attack: 0.002, mallet: 0.3 },
            wdrop: { model: 'drop', rise: 0.5, t60: 0.08, attack: 0.003, splash: 0.04, width: 0.4 },
            felt: { model: 'click', material: 'felt', tone: 1000, ring: 0.014 },
            breath: { model: 'noise', color: 'pink', band: [400, 3000], attack: 0.06, decay: 0.08 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'nyl', n: 7, t: 0.08, g: 0.8, pan: -0.25, t60: 0.8 }, { v: 'nyl', n: 12, t: 0.26, g: 0.85, pan: 0.1, t60: 0.85 },
                    { v: 'nyl', n: 15, t: 0.44, g: 0.9, pan: -0.1, t60: 0.9 }, { v: 'nyl', n: 19, t: 0.62, g: 0.8, pan: 0.2, t60: 0.9 },
                    { v: 'harp', n: 24, t: 0.9, g: 0.8, pan: -0.1, t60: 1.2 }, { v: 'harp', n: 27, t: 0.94, g: 0.9, pan: 0.15, t60: 1.2 },
                    { v: 'kal', n: 24, t: 0.9, g: 0.25, pan: 0, t60: 1 }, { v: 'wdrop', n: 31, t: 1.15, g: 0.3, pan: 0.3 }],
                layers: [{ v: 'breath', t: 0, g: 0.06, attack: 0.08 }],
                space: { size: 1.1, rt60: 1.0, damping: 5000, predelay: 0.02, mix: 0.3 }
            },
            'notify.message': {
                notes: [{ v: 'nyl', n: 7, t: 0.07, g: 0.8, pan: -0.2, t60: 0.5 }, { v: 'nyl', n: 12, t: 0.2, g: 0.9, pan: 0.15, t60: 0.55 },
                    { v: 'nyl', n: 15, t: 0.34, g: 0.8, pan: -0.05, t60: 0.6 }, { v: 'harp', n: 19, t: 0.375, g: 1, pan: 0.1, t60: 0.7 },
                    { v: 'wdrop', n: 24, t: 0.5, g: 0.35, pan: 0.2 }],
                layers: [{ v: 'breath', t: 0, g: 0.05, attack: 0.07 }],
                space: { size: 0.9, rt60: 0.7, damping: 5200, predelay: 0.014, mix: 0.26 }
            },
            'notify.info': {
                notes: [{ v: 'nyl', n: 10, t: 0.06, g: 0.85, pan: -0.15, t60: 0.4 }, { v: 'nyl', n: 15, t: 0.2, g: 0.8, pan: 0.1, t60: 0.45 },
                    { v: 'harp', n: 19, t: 0.235, g: 1, pan: 0.15, t60: 0.55 }, { v: 'kal', n: 19, t: 0.235, g: 0.2, pan: 0.15, t60: 0.3 }],
                layers: [{ v: 'breath', t: 0, g: 0.05, attack: 0.06 }],
                space: { size: 0.8, rt60: 0.6, damping: 5200, predelay: 0.012, mix: 0.24 }
            },
            'notify.success': {
                notes: [{ v: 'nyl', n: 7, t: 0.06, g: 0.8, pan: -0.2, t60: 0.4 }, { v: 'nyl', n: 12, t: 0.18, g: 0.9, pan: 0.1, t60: 0.45 },
                    { v: 'nyl', n: 15, t: 0.3, g: 0.8, pan: -0.05, t60: 0.5 }, { v: 'harp', n: 22, t: 0.335, g: 1, pan: 0.1, t60: 0.6 }],
                layers: [{ v: 'breath', t: 0, g: 0.05, attack: 0.06 }, { v: 'felt', t: 0.06, g: 0.25 }],
                space: { size: 0.85, rt60: 0.65, damping: 5200, predelay: 0.012, mix: 0.24 }
            },
            'notify.warning': {
                notes: [{ v: 'nyl', n: 12, t: 0, g: 1, pan: -0.1, t60: 0.3, brightness: 0.22, attack: 0.006 }, { v: 'low', n: 12, t: 0, g: 0.35, attack: 0.007, pan: -0.1, t60: 0.2 },
                    { v: 'nyl', n: 12, t: 0.2, g: 0.85, pan: 0.1, t60: 0.4, brightness: 0.25, attack: 0.006 }, { v: 'low', n: 12, t: 0.2, g: 0.3, attack: 0.007, pan: 0.1, t60: 0.25 }],
                layers: [{ v: 'felt', t: 0, g: 0.25, tone: 1100 }, { v: 'felt', t: 0.2, g: 0.2, tone: 1100 }],
                space: { size: 0.75, rt60: 0.5, damping: 4800, predelay: 0.01, mix: 0.2 }
            },
            'notify.error': {
                notes: [{ v: 'nyl', n: 15, t: 0, g: 1, pan: -0.1, t60: 0.35, brightness: 0.22, attack: 0.008 },
                    { v: 'low', n: 15, t: 0, g: 0.3, pan: -0.1, t60: 0.16, attack: 0.008 },
                    { v: 'nyl', n: 9, t: 0.17, g: 1, pan: 0.1, t60: 0.45, brightness: 0.22, attack: 0.008 },
                    { v: 'low', n: 9, t: 0.17, g: 0.3, pan: 0.1, t60: 0.18, attack: 0.008 }],
                layers: [{ v: 'knock', t: 0, g: 0.08 }, { v: 'knock', t: 0.17, g: 0.08 }],
                space: { size: 0.65, rt60: 0.4, damping: 4500, predelay: 0.006, mix: 0.16 }
            },
            'dialog.open': {
                notes: [{ v: 'nyl', n: 5, t: 0.06, g: 0.8, pan: -0.15, t60: 0.3 }, { v: 'nyl', n: 12, t: 0.095, g: 1, pan: 0.15, t60: 0.34 }],
                layers: [{ v: 'breath', t: 0, g: 0.05, attack: 0.055 }],
                space: { size: 0.65, rt60: 0.4, damping: 5500, predelay: 0.008, mix: 0.18 }
            },
            'dialog.confirm': {
                notes: [{ v: 'nyl', n: 7, t: 0.06, g: 0.8, pan: -0.15, t60: 0.3 }, { v: 'harp', n: 15, t: 0.1, g: 1, pan: 0.15, t60: 0.4 },
                    { v: 'kal', n: 15, t: 0.1, g: 0.2, pan: 0.15, t60: 0.25 }],
                layers: [{ v: 'breath', t: 0, g: 0.05, attack: 0.055 }],
                space: { size: 0.65, rt60: 0.45, damping: 5500, predelay: 0.008, mix: 0.18 }
            },
            'dialog.cancel': {
                notes: [{ v: 'nyl', n: 15, t: 0.05, g: 0.8, pan: 0.15, t60: 0.26 }, { v: 'nyl', n: 7, t: 0.09, g: 1, pan: -0.15, t60: 0.3 },
                    { v: 'wdrop', n: 14, t: 0.16, g: 0.4, pan: -0.1 }],
                layers: [{ v: 'breath', t: 0, g: 0.04, attack: 0.05 }],
                space: { size: 0.65, rt60: 0.35, damping: 4800, predelay: 0.008, mix: 0.16 }
            },
            'window.open': {
                notes: [{ v: 'nyl', n: 7, t: 0.06, g: 0.8, pan: -0.15, t60: 0.28 }, { v: 'nyl', n: 12, t: 0.095, g: 1, pan: 0.2, t60: 0.32 }],
                layers: [{ v: 'breath', t: 0, g: 0.06, attack: 0.055 }],
                space: { size: 0.7, rt60: 0.35, damping: 5500, predelay: 0.008, mix: 0.18 }
            },
            'window.close': {
                notes: [{ v: 'nyl', n: 12, t: 0.05, g: 0.8, pan: 0.2, t60: 0.22 }, { v: 'nyl', n: 7, t: 0.085, g: 1, pan: -0.15, t60: 0.26 },
                    { v: 'wdrop', n: 17, t: 0.15, g: 0.3, pan: -0.1 }],
                layers: [{ v: 'breath', t: 0, g: 0.05, attack: 0.045 }],
                space: { size: 0.7, rt60: 0.3, damping: 5000, predelay: 0.008, mix: 0.16 }
            },
            'window.maximize': {
                notes: [{ v: 'nyl', n: 3, t: 0.07, g: 0.8, pan: -0.2, t60: 0.3 }, { v: 'nyl', n: 15, t: 0.11, g: 1, pan: 0.2, t60: 0.38 },
                    { v: 'harp', n: 19, t: 0.145, g: 0.5, pan: 0.2, t60: 0.4 }],
                layers: [{ v: 'breath', t: 0, g: 0.06, attack: 0.065 }],
                space: { size: 0.9, rt60: 0.45, damping: 5500, predelay: 0.01, mix: 0.24 }
            },
            'window.minimize': {
                notes: [{ v: 'nyl', n: 10, t: 0, g: 0.8, pan: 0.15, t60: 0.2 }, { v: 'nyl', n: 5, t: 0.04, g: 1, pan: -0.15, t60: 0.22 },
                    { v: 'wdrop', n: 12, t: 0.1, g: 0.35, pan: -0.1 }],
                layers: [{ v: 'felt', t: 0, g: 0.25 }],
                space: { size: 0.55, rt60: 0.22, damping: 5000, predelay: 0.006, mix: 0.14 }
            },
            'window.restore': {
                notes: [{ v: 'nyl', n: 5, t: 0.03, g: 0.8, pan: -0.15, t60: 0.2 }, { v: 'nyl', n: 10, t: 0.07, g: 1, pan: 0.15, t60: 0.24 }],
                layers: [{ v: 'breath', t: 0, g: 0.04, attack: 0.03, decay: 0.05 }, { v: 'felt', t: 0.03, g: 0.25 }],
                space: { size: 0.55, rt60: 0.22, damping: 5000, predelay: 0.006, mix: 0.14 }
            },
            'window.snap': {
                layers: [{ v: 'felt', t: 0, g: 1, tone: 1500, ring: 0.012 }],
                notes: [{ v: 'kal', n: 17, t: 0, g: 0.9, t60: 0.1, dur: 0.07 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'low', n: 5, t: 0, g: 1, pan: 0, t60: 0.22, body: 0.6 }, { v: 'low', n: 0, t: 0.01, g: 0.8, pan: 0, t60: 0.2, body: 0.6 },
                    { v: 'kal', n: 5, t: 0, g: 0.25, pan: 0, t60: 0.15, hardness: 0.15 }],
                layers: [{ v: 'knock', t: 0, g: 0.5 }],
                space: { size: 0.5, rt60: 0.22, damping: 4500, predelay: 0.004, mix: 0.1 }
            },
            'file.drop': {
                notes: [{ v: 'low', n: 5, t: 0, g: 1, pan: 0, t60: 0.26, body: 0.6, attack: 0.007 }, { v: 'nyl', n: 12, t: 0.02, g: 0.4, t60: 0.15 },
                    { v: 'wdrop', n: 15, t: 0.07, g: 0.55, pan: 0.1, t60: 0.1 }],
                layers: [{ v: 'knock', t: 0, g: 0.4 }],
                space: { size: 0.65, rt60: 0.3, damping: 5000, predelay: 0.006, mix: 0.16 }
            },
            'file.trash': {
                notes: [{ v: 'nyl', n: 15, t: 0.03, g: 0.8, pan: 0.15, t60: 0.18 }, { v: 'nyl', n: 10, t: 0.08, g: 0.9, pan: 0, t60: 0.18 },
                    { v: 'nyl', n: 5, t: 0.13, g: 1, pan: -0.15, t60: 0.24 }, { v: 'wdrop', n: 12, t: 0.2, g: 0.45, pan: -0.1 }],
                layers: [{ v: 'breath', t: 0, g: 0.07, attack: 0.025, decay: 0.05 }],
                space: { size: 0.6, rt60: 0.3, damping: 5000, predelay: 0.006, mix: 0.16 }
            },
            'file.delete': {
                notes: [{ v: 'nyl', n: 10, t: 0, g: 0.85, pan: 0.1, t60: 0.25 }, { v: 'low', n: 5, t: 0.09, g: 1, pan: -0.1, t60: 0.3, body: 0.6 },
                    { v: 'low', n: 0, t: 0.1, g: 0.8, pan: -0.1, t60: 0.28, body: 0.6 }],
                layers: [{ v: 'knock', t: 0.09, g: 0.4 }],
                space: { size: 0.75, rt60: 0.4, damping: 4800, predelay: 0.008, mix: 0.2 }
            },
            'space.switch': {
                notes: [{ v: 'wdrop', n: 24, t: 0.07, g: 0.6, pan: -0.2, count: 3, spread: 0.06, t60: 0.08 },
                    { v: 'harp', n: 12, t: 0.12, g: 0.3, pan: 0.1, t60: 0.4 }],
                layers: [{ v: 'breath', t: 0, g: 0.9, attack: 0.12, decay: 0.2, band: [400, 2800] }],
                space: { size: 0.85, rt60: 0.45, damping: 5200, predelay: 0.01, mix: 0.22 }
            },
            'menu.open': {
                notes: [{ v: 'nyl', n: 12, t: 0, g: 0.8, pan: -0.1, t60: 0.1, air: 0.08, dur: 0.1 }, { v: 'nyl', n: 19, t: 0.03, g: 1, pan: 0.1, t60: 0.1, air: 0.08, dur: 0.1 }],
                layers: [{ v: 'felt', t: 0, g: 0.25 }],
                space: { size: 0.4, rt60: 0.12, damping: 6000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'nyl', n: 19, t: 0, g: 0.8, pan: 0.1, t60: 0.1, air: 0.08, dur: 0.1 }, { v: 'nyl', n: 12, t: 0.03, g: 1, pan: -0.1, t60: 0.1, air: 0.08, dur: 0.1 }],
                layers: [{ v: 'felt', t: 0, g: 0.25 }],
                space: { size: 0.4, rt60: 0.11, damping: 6000, predelay: 0.002, mix: 0.1 }
            },
            'ui.select': {
                notes: [{ v: 'low', n: 17, t: 0, g: 0.6, t60: 0.06, dur: 0.07 }],
                layers: [{ v: 'felt', t: 0, g: 1, tone: 1100, ring: 0.014 }],
                space: { size: 0.4, rt60: 0.06, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'felt', t: 0, g: 0.9, tone: 900, ring: 0.012 }, { v: 'felt', t: 0.04, g: 1, tone: 1300, ring: 0.012 }],
                notes: [{ v: 'low', n: 12, t: 0, g: 0.5, t60: 0.05, dur: 0.05 }, { v: 'low', n: 19, t: 0.04, g: 0.6, t60: 0.05, dur: 0.05 }],
                space: { size: 0.4, rt60: 0.05, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.off': {
                layers: [{ v: 'felt', t: 0, g: 0.9, tone: 1300, ring: 0.012 }, { v: 'felt', t: 0.04, g: 1, tone: 900, ring: 0.012 }],
                notes: [{ v: 'low', n: 19, t: 0, g: 0.5, t60: 0.05, dur: 0.05 }, { v: 'low', n: 12, t: 0.04, g: 0.6, t60: 0.05, dur: 0.05 }],
                space: { size: 0.4, rt60: 0.05, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.tick': {
                layers: [{ v: 'felt', t: 0, g: 1, tone: 2400, ring: 0.008, length: 0.002 }],
                space: null
            }
        },
        fx: {}
    });
})();
