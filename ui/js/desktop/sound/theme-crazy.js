// Crazy: springy boings and a soft toy xylophone in a short, slapping room. Key E4 (64).
// Notes are drawn from the whole-tone scale (0 2 4 6 8 10) and Lydian (0 2 4 6 7 9 11): the surprise sits in the leaps (tritones,
// whole steps) and the glides, not in loudness or length. A bubble pop is only an occasional accent, never a cluster.
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'crazy',
        key: 64,
        space: { size: 0.5, rt60: 0.25, damping: 6000, predelay: 0.004, mix: 0.16 },
        voices: {
            boing: { model: 'blip', glide: -5, glideTime: 0.045, boing: 0.35, boingRate: 9, harm: 0.25, attack: 0.004, decay: 0.14 },
            pop: { model: 'drop', t60: 0.05, rise: 1, pitchSpread: 2, splash: 0.2, width: 0.5 },
            toy: { model: 'bar', kind: 'xylophone', t60: 0.24, hardness: 0.12, mallet: 0.4, attack: 0.003 },
            bell: { model: 'bar', kind: 'glockenspiel', t60: 0.7, hardness: 0.12, mallet: 0.35, attack: 0.003 },
            thump: { model: 'bar', kind: 'marimba', t60: 0.22, hardness: 0.2, body: 0, mallet: 0.25, attack: 0.004 },
            tick: { model: 'click', material: 'plastic', tone: 2600, ring: 0.02, length: 0.003 },
            knock: { model: 'click', material: 'felt', tone: 420, ring: 0.03, length: 0.006 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'toy', n: 0, t: 0, g: 0.85, pan: -0.25 }, { v: 'toy', n: 4, t: 0.13, g: 0.9, pan: 0.15 },
                    { v: 'boing', n: 8, t: 0.26, g: 0.8, pan: -0.1, glide: -7, decay: 0.2 }, { v: 'toy', n: 14, t: 0.43, g: 0.9, pan: 0.2 },
                    { v: 'bell', n: 18, t: 0.62, g: 0.9, pan: 0, t60: 1.1 }, { v: 'pop', n: 12, t: 0.84, g: 0.5, pan: 0.15, t60: 0.07 }],
                echo: { time: 0.17, feedback: 0.3, tone: 4500, mix: 0.14, spread: 0.6 },
                space: { size: 0.9, rt60: 0.8, damping: 5500, predelay: 0.012, mix: 0.28 }
            },
            'notify.message': {
                notes: [{ v: 'boing', n: 4, t: 0, g: 0.85, pan: -0.2, glide: -5, decay: 0.15 }, { v: 'toy', n: 10, t: 0.11, g: 0.9, pan: 0.15 },
                    { v: 'toy', n: 16, t: 0.2, g: 1, pan: 0, t60: 0.35 }],
                echo: { time: 0.08, feedback: 0.15, tone: 4500, mix: 0.1, spread: 0.6 },
                space: { size: 0.6, rt60: 0.35, damping: 5500, predelay: 0.006, mix: 0.2 }
            },
            'notify.info': {
                notes: [{ v: 'boing', n: 2, t: 0, g: 0.85, pan: -0.15, glide: -4, decay: 0.12 }, { v: 'toy', n: 9, t: 0.1, g: 1, pan: 0.15, t60: 0.3 }],
                echo: { time: 0.08, feedback: 0.15, tone: 4500, mix: 0.1, spread: 0.6 },
                space: { size: 0.55, rt60: 0.3, damping: 5500, predelay: 0.005, mix: 0.18 }
            },
            'notify.success': {
                notes: [{ v: 'toy', n: 4, t: 0, g: 0.8, pan: -0.2 }, { v: 'toy', n: 8, t: 0.09, g: 0.9, pan: 0.1 },
                    { v: 'boing', n: 14, t: 0.18, g: 1, pan: 0, glide: -3, decay: 0.16 }],
                echo: { time: 0.08, feedback: 0.15, tone: 4500, mix: 0.1, spread: 0.6 },
                space: { size: 0.6, rt60: 0.35, damping: 5500, predelay: 0.006, mix: 0.2 }
            },
            'notify.warning': {
                notes: [{ v: 'boing', n: 7, t: 0, g: 1, pan: -0.1, glide: -3, boing: 0.45, decay: 0.12 },
                    { v: 'boing', n: 7, t: 0.17, g: 0.8, pan: 0.1, glide: -3, boing: 0.45, decay: 0.14 }],
                echo: { time: 0.08, feedback: 0.15, tone: 4500, mix: 0.08, spread: 0.6 },
                space: { size: 0.55, rt60: 0.3, damping: 5000, predelay: 0.006, mix: 0.18 }
            },
            'notify.error': {
                notes: [{ v: 'boing', n: 6, t: 0, g: 1, pan: -0.1, glide: 2, boing: 0.45, boingRate: 7, decay: 0.14 },
                    { v: 'boing', n: 0, t: 0.17, g: 0.95, pan: 0.1, glide: 4, glideTime: 0.08, boing: 0.45, boingRate: 7, decay: 0.22 }],
                echo: { time: 0.08, feedback: 0.15, tone: 4000, mix: 0.1, spread: 0.6 },
                space: { size: 0.55, rt60: 0.3, damping: 5000, predelay: 0.005, mix: 0.16 }
            },
            'dialog.open': {
                notes: [{ v: 'boing', n: 0, t: 0, g: 0.9, pan: -0.15, glide: -5, decay: 0.1 }, { v: 'toy', n: 7, t: 0.08, g: 1, pan: 0.15, t60: 0.22 }],
                echo: { time: 0.07, feedback: 0.12, tone: 4500, mix: 0.09, spread: 0.6 },
                space: { size: 0.5, rt60: 0.28, damping: 5500, predelay: 0.005, mix: 0.18 }
            },
            'dialog.confirm': {
                notes: [{ v: 'toy', n: 4, t: 0, g: 0.85, pan: -0.15, t60: 0.2 }, { v: 'boing', n: 10, t: 0.09, g: 1, pan: 0.15, glide: -5, decay: 0.14 }, { v: 'bell', n: 16, t: 0.17, g: 0.6, pan: 0, t60: 0.4 }],
                echo: { time: 0.08, feedback: 0.15, tone: 4500, mix: 0.1, spread: 0.6 },
                space: { size: 0.5, rt60: 0.3, damping: 5500, predelay: 0.005, mix: 0.18 }
            },
            'dialog.cancel': {
                notes: [{ v: 'boing', n: 10, t: 0, g: 0.9, pan: 0.15, glide: 4, decay: 0.1 }, { v: 'toy', n: 4, t: 0.09, g: 1, pan: -0.15, t60: 0.16 }],
                echo: { time: 0.07, feedback: 0.12, tone: 4500, mix: 0.08, spread: 0.6 },
                space: { size: 0.5, rt60: 0.25, damping: 4800, predelay: 0.005, mix: 0.15 }
            },
            'window.open': {
                notes: [{ v: 'boing', n: 2, t: 0, g: 0.9, pan: -0.15, glide: -6, decay: 0.12 }, { v: 'toy', n: 8, t: 0.075, g: 1, pan: 0.2, t60: 0.2 }],
                space: { size: 0.5, rt60: 0.25, damping: 6000, predelay: 0.004, mix: 0.16 }
            },
            'window.close': {
                notes: [{ v: 'toy', n: 8, t: 0, g: 0.9, pan: 0.2, t60: 0.14 }, { v: 'boing', n: 2, t: 0.07, g: 1, pan: -0.15, glide: 6, decay: 0.12 }],
                space: { size: 0.5, rt60: 0.22, damping: 5500, predelay: 0.004, mix: 0.14 }
            },
            'window.maximize': {
                notes: [{ v: 'boing', n: 0, t: 0, g: 0.85, pan: -0.2, glide: -8, decay: 0.14 }, { v: 'toy', n: 12, t: 0.1, g: 1, pan: 0.2, t60: 0.3 }],
                echo: { time: 0.09, feedback: 0.18, tone: 4500, mix: 0.1, spread: 0.6 },
                space: { size: 0.7, rt60: 0.38, damping: 5500, predelay: 0.008, mix: 0.24 }
            },
            'window.minimize': {
                notes: [{ v: 'toy', n: 8, t: 0, g: 0.9, pan: 0.15, t60: 0.12 }, { v: 'boing', n: 2, t: 0.06, g: 1, pan: -0.15, glide: 5, decay: 0.09, boing: 0.2 }],
                space: { size: 0.45, rt60: 0.2, damping: 5500, predelay: 0.003, mix: 0.13 }
            },
            'window.restore': {
                notes: [{ v: 'boing', n: 2, t: 0, g: 0.9, pan: -0.15, glide: -5, decay: 0.09, boing: 0.2 }, { v: 'toy', n: 8, t: 0.06, g: 1, pan: 0.15, t60: 0.12 }],
                space: { size: 0.45, rt60: 0.2, damping: 5500, predelay: 0.003, mix: 0.13 }
            },
            'window.snap': {
                notes: [{ v: 'toy', n: 14, t: 0, g: 0.9, pan: -0.1, t60: 0.06, dur: 0.05 }, { v: 'boing', n: 20, t: 0.025, g: 0.9, pan: 0.1, glide: -3, boing: 0.15, decay: 0.05, dur: 0.05 }],
                layers: [{ v: 'tick', t: 0, g: 0.4, tone: 2000, ring: 0.012 }],
                space: { size: 0.4, rt60: 0.08, damping: 6000, predelay: 0.002, mix: 0.08 }
            },
            'window.deny': {
                notes: [{ v: 'boing', n: -7, t: 0, g: 0.9, pan: 0, glide: 5, glideTime: 0.07, boing: 0.2, decay: 0.16 }],
                layers: [{ v: 'knock', t: 0, g: 0.5 }],
                space: { size: 0.45, rt60: 0.18, damping: 4800, predelay: 0.003, mix: 0.12 }
            },
            'file.drop': {
                notes: [{ v: 'boing', n: 6, t: 0, g: 0.95, pan: -0.05, glide: 7, boing: 0.3, decay: 0.13 },
                    { v: 'boing', n: 2, t: 0.085, g: 0.5, pan: 0.15, glide: 4, boing: 0.2, decay: 0.09 }, { v: 'toy', n: -2, t: 0.12, g: 0.5, pan: 0, t60: 0.14 }],
                echo: { time: 0.08, feedback: 0.12, tone: 4500, mix: 0.08, spread: 0.6 },
                space: { size: 0.5, rt60: 0.25, damping: 5000, predelay: 0.004, mix: 0.16 }
            },
            'file.trash': {
                notes: [{ v: 'toy', n: 12, t: 0, g: 0.8, pan: 0.15, t60: 0.12 }, { v: 'toy', n: 8, t: 0.05, g: 0.9, pan: 0, t60: 0.12 },
                    { v: 'toy', n: 4, t: 0.1, g: 1, pan: -0.15, t60: 0.16 }],
                space: { size: 0.5, rt60: 0.25, damping: 5000, predelay: 0.004, mix: 0.15 }
            },
            'file.delete': {
                notes: [{ v: 'thump', n: -2, t: 0, g: 0.85, pan: 0.1, t60: 0.2, attack: 0.005 }, { v: 'boing', n: -8, t: 0.09, g: 0.9, pan: -0.1, glide: 6, decay: 0.2, boing: 0.2 }],
                space: { size: 0.6, rt60: 0.35, damping: 4800, predelay: 0.005, mix: 0.18 }
            },
            'space.switch': {
                notes: [{ v: 'boing', n: 6, t: 0.0, g: 0.8, pan: -0.2, glide: -8, glideTime: 0.12, decay: 0.26, attack: 0.03, boing: 0.15 },
                    { v: 'toy', n: 12, t: 0.14, g: 0.6, pan: 0.2, t60: 0.3 }],
                echo: { time: 0.09, feedback: 0.15, tone: 4500, mix: 0.1, spread: 0.6 },
                space: { size: 0.6, rt60: 0.32, damping: 5500, predelay: 0.006, mix: 0.2 }
            },
            'menu.open': {
                notes: [{ v: 'toy', n: 2, t: 0, g: 0.9, pan: -0.1, t60: 0.1, dur: 0.085 }, { v: 'toy', n: 8, t: 0.035, g: 1, pan: 0.1, t60: 0.1, dur: 0.085 }],
                space: { size: 0.4, rt60: 0.1, damping: 6000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'toy', n: 8, t: 0, g: 0.9, pan: 0.1, t60: 0.1, dur: 0.085 }, { v: 'toy', n: 2, t: 0.03, g: 1, pan: -0.1, t60: 0.1, dur: 0.085 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.select': {
                notes: [{ v: 'boing', n: 14, t: 0, g: 0.8, pan: -0.1, glide: -3, boing: 0.15, attack: 0.002, decay: 0.045, dur: 0.05 }],
                layers: [{ v: 'tick', t: 0, g: 0.5, tone: 2200, ring: 0.012 }],
                space: { size: 0.4, rt60: 0.08, damping: 5500, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.on': {
                notes: [{ v: 'boing', n: 10, t: 0, g: 0.9, pan: -0.1, glide: -3, boing: 0.1, attack: 0.002, decay: 0.04, dur: 0.04 },
                    { v: 'boing', n: 14, t: 0.035, g: 1, pan: 0.1, glide: -3, boing: 0.1, attack: 0.002, decay: 0.04, dur: 0.04 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.07 }
            },
            'ui.toggle.off': {
                notes: [{ v: 'boing', n: 14, t: 0, g: 0.9, pan: 0.1, glide: 3, boing: 0.1, attack: 0.002, decay: 0.04, dur: 0.04 },
                    { v: 'boing', n: 10, t: 0.035, g: 1, pan: -0.1, glide: 3, boing: 0.1, attack: 0.002, decay: 0.04, dur: 0.04 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.07 }
            },
            'ui.tick': {
                notes: [{ v: 'toy', n: 18, t: 0, g: 0.8, pan: -0.15, t60: 0.03, attack: 0.001, dur: 0.02 }, { v: 'toy', n: 22, t: 0.014, g: 1, pan: 0.15, t60: 0.03, attack: 0.001, dur: 0.02 }],
                space: null
            }
        },
        fx: {}
    });
})();
