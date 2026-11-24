// Lollipop: sweet celesta chimes, glockenspiel sparkle and light candy-like pops in a small bright room.
// Key B-flat 5 (82), major scale 0 2 4 5 7 9 11. The frequent events sit in the octave below the key and keep soft attacks.
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'lollipop',
        key: 82,
        space: { size: 0.6, rt60: 0.45, damping: 8000, predelay: 0.008, mix: 0.2 },
        voices: {
            cel: { model: 'bell', kind: 'celesta', t60: 0.5, index: 0.9, attack: 0.004, spread: 0.3 },
            glock: { model: 'bar', kind: 'glockenspiel', t60: 0.5, hardness: 0.2, attack: 0.003, mallet: 0.25 },
            pop: { model: 'drop', rise: 0.7, t60: 0.05, attack: 0.003, splash: 0.15, width: 0.3 },
            low: { model: 'string', kind: 'muted', t60: 0.2, brightness: 0.2, pick: 0.15, body: 0.15 },
            tick: { model: 'click', material: 'plastic', tone: 2600, ring: 0.012, noise: 0.6, q: 2.5 },
            air: { model: 'noise', color: 'pink', band: [1800, 7000], attack: 0.05, decay: 0.18 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'cel', n: -12, t: 0, g: 0.8, pan: -0.25, t60: 0.7 }, { v: 'cel', n: -7, t: 0.16, g: 0.85, pan: 0.1, t60: 0.75 },
                    { v: 'cel', n: -3, t: 0.32, g: 0.9, pan: -0.1, t60: 0.8 }, { v: 'cel', n: 0, t: 0.48, g: 0.9, pan: 0.2, t60: 0.9 },
                    { v: 'glock', n: 0, t: 0.48, g: 0.4, pan: 0.2, t60: 0.8 }, { v: 'cel', n: 7, t: 0.72, g: 0.85, pan: 0, t60: 1.2, attack: 0.006 },
                    { v: 'pop', n: 7, t: 0.72, g: 0.3, pan: 0.15 }],
                layers: [{ v: 'air', t: 0, g: 0.05 }],
                space: { size: 1, rt60: 1.0, damping: 7500, predelay: 0.014, mix: 0.3 }
            },
            'notify.message': {
                notes: [{ v: 'cel', n: -7, t: 0, g: 0.8, pan: -0.2, t60: 0.5 }, { v: 'cel', n: -3, t: 0.1, g: 0.9, pan: 0.15, t60: 0.55 },
                    { v: 'cel', n: 0, t: 0.2, g: 1, pan: 0, t60: 0.7 }, { v: 'glock', n: 0, t: 0.2, g: 0.35, pan: 0, t60: 0.5 }],
                space: { size: 0.8, rt60: 0.7, damping: 7500, predelay: 0.012, mix: 0.26 }
            },
            'notify.info': {
                notes: [{ v: 'cel', n: -5, t: 0, g: 0.85, pan: -0.15, t60: 0.4 }, { v: 'cel', n: -1, t: 0.09, g: 1, pan: 0.15, t60: 0.55 },
                    { v: 'glock', n: -1, t: 0.09, g: 0.3, pan: 0.15, t60: 0.35 }],
                space: { size: 0.75, rt60: 0.6, damping: 7500, predelay: 0.01, mix: 0.24 }
            },
            'notify.success': {
                notes: [{ v: 'cel', n: -12, t: 0, g: 0.8, pan: -0.2, t60: 0.4 }, { v: 'cel', n: -7, t: 0.09, g: 0.9, pan: 0.1, t60: 0.45 },
                    { v: 'cel', n: -3, t: 0.18, g: 0.95, pan: -0.1, t60: 0.5 }, { v: 'cel', n: 0, t: 0.27, g: 1, pan: 0, t60: 0.65 },
                    { v: 'pop', n: 0, t: 0.27, g: 0.3 }],
                space: { size: 0.8, rt60: 0.6, damping: 7500, predelay: 0.01, mix: 0.24 }
            },
            'notify.warning': {
                notes: [{ v: 'cel', n: -7, t: 0, g: 1, pan: -0.1, t60: 0.3, index: 0.6, attack: 0.007 }, { v: 'glock', n: -7, t: 0, g: 0.3, pan: -0.1, t60: 0.2 },
                    { v: 'cel', n: -7, t: 0.17, g: 0.85, pan: 0.1, t60: 0.4, index: 0.6, attack: 0.007 }, { v: 'glock', n: -7, t: 0.17, g: 0.25, pan: 0.1, t60: 0.2 }],
                layers: [{ v: 'tick', t: 0, g: 0.3, tone: 1800 }],
                space: { size: 0.7, rt60: 0.5, damping: 6500, predelay: 0.008, mix: 0.2 }
            },
            'notify.error': {
                notes: [{ v: 'cel', n: -12, t: 0, g: 1, pan: -0.1, t60: 0.28, index: 0.5, attack: 0.009 },
                    { v: 'cel', n: -18, t: 0.15, g: 0.9, pan: 0.1, t60: 0.34, index: 0.5, attack: 0.01 },
                    { v: 'low', n: -18, t: 0.15, g: 0.7, pan: 0.1, t60: 0.28 }],
                layers: [{ v: 'knock', t: 0, g: 0.4 }, { v: 'knock', t: 0.15, g: 0.4 }],
                space: { size: 0.6, rt60: 0.4, damping: 5500, predelay: 0.006, mix: 0.18 }
            },
            'dialog.open': {
                notes: [{ v: 'cel', n: -12, t: 0, g: 0.85, pan: -0.15, t60: 0.22 }, { v: 'cel', n: -7, t: 0.08, g: 1, pan: 0.15, t60: 0.28 },
                    { v: 'glock', n: -7, t: 0.08, g: 0.3, pan: 0.15, t60: 0.2 }],
                space: { size: 0.65, rt60: 0.4, damping: 7500, predelay: 0.008, mix: 0.2 }
            },
            'dialog.confirm': {
                notes: [{ v: 'cel', n: -8, t: 0, g: 0.85, pan: -0.15, t60: 0.22 }, { v: 'cel', n: -1, t: 0.09, g: 1, pan: 0.15, t60: 0.32 },
                    { v: 'pop', n: -1, t: 0.09, g: 0.3, pan: 0.15 }],
                space: { size: 0.65, rt60: 0.45, damping: 7500, predelay: 0.008, mix: 0.2 }
            },
            'dialog.cancel': {
                notes: [{ v: 'cel', n: -1, t: 0, g: 0.85, pan: 0.15, t60: 0.2 }, { v: 'cel', n: -8, t: 0.09, g: 1, pan: -0.15, t60: 0.24 },
                    { v: 'glock', n: -8, t: 0.09, g: 0.3, pan: -0.15, t60: 0.18 }],
                space: { size: 0.65, rt60: 0.35, damping: 6500, predelay: 0.008, mix: 0.18 }
            },
            'window.open': {
                notes: [{ v: 'cel', n: -12, t: 0, g: 0.9, pan: -0.15, t60: 0.22 }, { v: 'cel', n: -7, t: 0.08, g: 1, pan: 0.2, t60: 0.24 },
                    { v: 'glock', n: -7, t: 0.08, g: 0.25, pan: 0.2, t60: 0.16 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.1 }],
                space: { size: 0.65, rt60: 0.35, damping: 7500, predelay: 0.008, mix: 0.2 }
            },
            'window.close': {
                notes: [{ v: 'cel', n: -7, t: 0, g: 0.9, pan: 0.2, t60: 0.2 }, { v: 'cel', n: -12, t: 0.08, g: 1, pan: -0.15, t60: 0.22 },
                    { v: 'glock', n: -12, t: 0.08, g: 0.25, pan: -0.15, t60: 0.15 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.1 }],
                space: { size: 0.65, rt60: 0.3, damping: 6500, predelay: 0.008, mix: 0.18 }
            },
            'window.maximize': {
                notes: [{ v: 'cel', n: -17, t: 0, g: 0.85, pan: -0.2, t60: 0.24 }, { v: 'cel', n: -8, t: 0.1, g: 1, pan: 0.2, t60: 0.3 },
                    { v: 'pop', n: -8, t: 0.1, g: 0.3, pan: 0.2 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.1 }],
                space: { size: 0.85, rt60: 0.4, damping: 7500, predelay: 0.01, mix: 0.24 }
            },
            'window.minimize': {
                notes: [{ v: 'cel', n: -8, t: 0, g: 0.9, pan: 0.15, t60: 0.16 }, { v: 'cel', n: -15, t: 0.065, g: 1, pan: -0.15, t60: 0.18 }],
                layers: [{ v: 'tick', t: 0, g: 0.2, tone: 2200 }],
                space: { size: 0.55, rt60: 0.25, damping: 6500, predelay: 0.006, mix: 0.16 }
            },
            'window.restore': {
                notes: [{ v: 'cel', n: -15, t: 0, g: 0.9, pan: -0.15, t60: 0.16 }, { v: 'cel', n: -8, t: 0.065, g: 1, pan: 0.15, t60: 0.18 }],
                layers: [{ v: 'tick', t: 0, g: 0.2, tone: 2200 }],
                space: { size: 0.55, rt60: 0.25, damping: 6500, predelay: 0.006, mix: 0.16 }
            },
            'window.snap': {
                layers: [{ v: 'tick', t: 0, g: 0.8, tone: 2800, ring: 0.014 }],
                notes: [{ v: 'glock', n: -7, t: 0, g: 0.9, t60: 0.1, dur: 0.07 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'low', n: -17, t: 0, g: 0.9, pan: 0, t60: 0.16 }, { v: 'cel', n: -17, t: 0, g: 0.5, pan: 0, t60: 0.15, index: 0.4, attack: 0.007 }],
                layers: [{ v: 'knock', t: 0, g: 0.4 }],
                space: { size: 0.5, rt60: 0.22, damping: 4800, predelay: 0.004, mix: 0.1 }
            },
            'file.drop': {
                notes: [{ v: 'low', n: -17, t: 0, g: 0.9, pan: 0, t60: 0.22 }, { v: 'glock', n: -12, t: 0.03, g: 0.8, t60: 0.15 }, { v: 'cel', n: -17, t: 0, g: 0.5, t60: 0.2, index: 0.4, attack: 0.007 }],
                layers: [{ v: 'knock', t: 0, g: 0.3 }],
                space: { size: 0.6, rt60: 0.3, damping: 5500, predelay: 0.006, mix: 0.16 }
            },
            'file.trash': {
                notes: [{ v: 'cel', n: -5, t: 0, g: 0.8, pan: 0.15, t60: 0.14 }, { v: 'cel', n: -12, t: 0.05, g: 0.9, pan: 0, t60: 0.14 },
                    { v: 'cel', n: -17, t: 0.1, g: 1, pan: -0.15, t60: 0.18 }, { v: 'pop', n: -17, t: 0.1, g: 0.3 }],
                layers: [{ v: 'air', t: 0, g: 0.08, attack: 0.02, decay: 0.1 }],
                space: { size: 0.6, rt60: 0.3, damping: 6000, predelay: 0.006, mix: 0.16 }
            },
            'file.delete': {
                notes: [{ v: 'cel', n: -12, t: 0, g: 0.85, pan: 0.1, t60: 0.24, index: 0.7 }, { v: 'low', n: -17, t: 0.09, g: 1, pan: -0.1, t60: 0.28 },
                    { v: 'cel', n: -17, t: 0.09, g: 0.4, pan: -0.1, t60: 0.26, index: 0.6, attack: 0.007 }],
                layers: [{ v: 'knock', t: 0.09, g: 0.4 }],
                space: { size: 0.7, rt60: 0.4, damping: 5500, predelay: 0.008, mix: 0.18 }
            },
            'space.switch': {
                notes: [{ v: 'cel', n: -7, t: 0.05, g: 0.5, pan: 0, t60: 0.3, attack: 0.02 }, { v: 'glock', n: 0, t: 0.15, g: 0.3, pan: 0.2, t60: 0.3 }],
                layers: [{ v: 'air', t: 0, g: 0.8, attack: 0.12, decay: 0.2, band: [1500, 6500] }],
                space: { size: 0.75, rt60: 0.4, damping: 7500, predelay: 0.008, mix: 0.2 }
            },
            'menu.open': {
                notes: [{ v: 'glock', n: -12, t: 0, g: 0.9, pan: -0.1, t60: 0.07, air: 0, dur: 0.09 }, { v: 'glock', n: -5, t: 0.035, g: 1, pan: 0.1, t60: 0.07, air: 0, dur: 0.09 }],
                layers: [{ v: 'tick', t: 0, g: 0.2, tone: 2200 }],
                space: { size: 0.4, rt60: 0.12, damping: 7000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'glock', n: -5, t: 0, g: 0.9, pan: 0.1, t60: 0.07, air: 0, dur: 0.09 }, { v: 'glock', n: -12, t: 0.03, g: 1, pan: -0.1, t60: 0.07, air: 0, dur: 0.09 }],
                layers: [{ v: 'tick', t: 0, g: 0.2, tone: 2200 }],
                space: { size: 0.4, rt60: 0.07, damping: 7000, predelay: 0.002, mix: 0.06 }
            },
            'ui.select': {
                notes: [{ v: 'pop', n: -7, t: 0, g: 0.5, t60: 0.03, dur: 0.06 }],
                layers: [{ v: 'tick', t: 0, g: 1, tone: 2400, ring: 0.012 }],
                space: { size: 0.4, rt60: 0.06, damping: 7000, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'tick', t: 0, g: 0.9, tone: 2200, ring: 0.012 }, { v: 'tick', t: 0.035, g: 1, tone: 3000, ring: 0.012 }],
                notes: [{ v: 'glock', n: -7, t: 0.035, g: 0.5, t60: 0.06, dur: 0.06, air: 0 }],
                space: { size: 0.4, rt60: 0.06, damping: 7000, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.off': {
                layers: [{ v: 'tick', t: 0, g: 0.9, tone: 3000, ring: 0.012 }, { v: 'tick', t: 0.035, g: 1, tone: 2200, ring: 0.012 }],
                notes: [{ v: 'glock', n: -12, t: 0.035, g: 0.5, t60: 0.06, dur: 0.06, air: 0 }],
                space: { size: 0.4, rt60: 0.06, damping: 7000, predelay: 0.002, mix: 0.06 }
            },
            'ui.tick': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 3400, ring: 0.008, length: 0.002 }],
                space: null
            }
        },
        fx: {}
    });
})();
