// Crazy: bouncing bubbles, boing glides and a toy xylophone in a short, springy room. Key E4 (64).
// Notes are drawn from the whole-tone scale (0 2 4 6 8 10) and Lydian (0 2 4 6 7 9 11): the surprise sits in the leaps and
// the glides, not in loudness or length. Frequent events stay short and quiet.
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'crazy',
        key: 64,
        space: { size: 0.55, rt60: 0.3, damping: 6000, predelay: 0.004, mix: 0.18 },
        voices: {
            boing: { model: 'blip', glide: -5, glideTime: 0.045, boing: 0.35, boingRate: 9, harm: 0.25, attack: 0.004, decay: 0.14 },
            pop: { model: 'drop', t60: 0.05, rise: 1, pitchSpread: 2, splash: 0.3, width: 0.5 },
            toy: { model: 'bar', kind: 'xylophone', t60: 0.28, hardness: 0.25, mallet: 0.2, attack: 0.002 },
            bell: { model: 'bar', kind: 'glockenspiel', t60: 0.7, hardness: 0.2, mallet: 0.15, attack: 0.002 },
            thump: { model: 'bar', kind: 'marimba', t60: 0.22, hardness: 0.2, body: 0, mallet: 0.25, attack: 0.004 },
            tick: { model: 'click', material: 'plastic', tone: 2600, ring: 0.02, length: 0.003 },
            puff: { model: 'noise', color: 'pink', band: [900, 4500], attack: 0.01, decay: 0.07, width: 0.9 },
            knock: { model: 'noise', color: 'pink', band: [250, 900], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'toy', n: 0, t: 0, g: 0.85, pan: -0.25 }, { v: 'toy', n: 4, t: 0.13, g: 0.9, pan: 0.15 },
                    { v: 'boing', n: 8, t: 0.26, g: 0.8, pan: -0.1, glide: -7, decay: 0.2 }, { v: 'toy', n: 11, t: 0.43, g: 0.9, pan: 0.2 },
                    { v: 'bell', n: 16, t: 0.62, g: 0.9, pan: 0, t60: 1.1 }, { v: 'pop', n: 12, t: 0.84, g: 0.6, pan: 0.15, count: 3, spread: 0.06, t60: 0.07 }],
                layers: [{ v: 'puff', t: 0.6, g: 0.1, attack: 0.04, decay: 0.3 }],
                echo: { time: 0.17, feedback: 0.3, tone: 4500, mix: 0.14, spread: 0.6 },
                space: { size: 0.9, rt60: 0.8, damping: 5500, predelay: 0.012, mix: 0.28 }
            },
            'notify.message': {
                notes: [{ v: 'pop', n: 4, t: 0, g: 0.85, pan: -0.2, t60: 0.07, splash: 0.4 }, { v: 'boing', n: 10, t: 0.1, g: 0.85, pan: 0.15, glide: -6, decay: 0.16 },
                    { v: 'pop', n: 7, t: 0.22, g: 1, pan: 0, t60: 0.1, count: 3, spread: 0.05, splash: 0.4 }],
                echo: { time: 0.12, feedback: 0.25, tone: 4500, mix: 0.12, spread: 0.6 },
                space: { size: 0.75, rt60: 0.5, damping: 5500, predelay: 0.008, mix: 0.24 }
            },
            'notify.info': {
                notes: [{ v: 'pop', n: 2, t: 0, g: 0.85, pan: -0.15, t60: 0.06 }, { v: 'pop', n: 8, t: 0.1, g: 1, pan: 0.15, t60: 0.08, count: 2, spread: 0.05 }],
                space: { size: 0.7, rt60: 0.45, damping: 5500, predelay: 0.006, mix: 0.22 }
            },
            'notify.success': {
                notes: [{ v: 'pop', n: 4, t: 0, g: 0.8, pan: -0.2, t60: 0.06 }, { v: 'boing', n: 8, t: 0.09, g: 0.85, pan: 0.1, glide: -3, decay: 0.12 },
                    { v: 'pop', n: 12, t: 0.18, g: 1, pan: 0, t60: 0.1, count: 2, spread: 0.05 }],
                space: { size: 0.7, rt60: 0.45, damping: 5500, predelay: 0.006, mix: 0.22 }
            },
            'notify.warning': {
                notes: [{ v: 'pop', n: 6, t: 0, g: 1, pan: -0.1, t60: 0.08, rise: 0.3, attack: 0.005 }, { v: 'pop', n: 6, t: 0.17, g: 0.85, pan: 0.1, t60: 0.1, rise: 0.3, attack: 0.005 }],
                space: { size: 0.65, rt60: 0.4, damping: 5000, predelay: 0.006, mix: 0.2 }
            },
            'notify.error': {
                notes: [{ v: 'pop', n: 4, t: 0, g: 1, pan: -0.1, t60: 0.12, attack: 0.008, rise: 0.15, count: 2, spread: 0.05, splash: 0.4 },
                    { v: 'pop', n: -2, t: 0.15, g: 0.95, pan: 0.1, t60: 0.15, attack: 0.008, rise: 0.15, count: 2, spread: 0.06, splash: 0.4 }],
                layers: [{ v: 'puff', t: 0, g: 0.3, attack: 0.04, decay: 0.35, band: [250, 1400] }],
                space: { size: 0.6, rt60: 0.4, damping: 4500, predelay: 0.005, mix: 0.18 }
            },
            'dialog.open': {
                notes: [{ v: 'pop', n: 0, t: 0, g: 0.85, pan: -0.15, t60: 0.05 }, { v: 'boing', n: 7, t: 0.08, g: 1, pan: 0.15, glide: -4, decay: 0.1 }],
                space: { size: 0.6, rt60: 0.35, damping: 5500, predelay: 0.006, mix: 0.2 }
            },
            'dialog.confirm': {
                notes: [{ v: 'pop', n: 4, t: 0, g: 0.85, pan: -0.15, t60: 0.05 }, { v: 'pop', n: 10, t: 0.09, g: 1, pan: 0.15, t60: 0.07, count: 2, spread: 0.04 }],
                space: { size: 0.6, rt60: 0.4, damping: 5500, predelay: 0.006, mix: 0.2 }
            },
            'dialog.cancel': {
                notes: [{ v: 'pop', n: 10, t: 0, g: 0.85, pan: 0.15, t60: 0.05, rise: 0.5 }, { v: 'pop', n: 4, t: 0.09, g: 1, pan: -0.15, t60: 0.06, rise: 0.5 }],
                space: { size: 0.6, rt60: 0.3, damping: 4800, predelay: 0.006, mix: 0.16 }
            },
            'window.open': {
                notes: [{ v: 'boing', n: 2, t: 0, g: 0.9, pan: -0.15, glide: -5, decay: 0.1 }, { v: 'pop', n: 8, t: 0.075, g: 1, pan: 0.2, t60: 0.05 }],
                space: { size: 0.55, rt60: 0.3, damping: 6000, predelay: 0.004, mix: 0.18 }
            },
            'window.close': {
                notes: [{ v: 'pop', n: 8, t: 0, g: 0.9, pan: 0.2, t60: 0.045, rise: 0.5 }, { v: 'boing', n: 2, t: 0.07, g: 1, pan: -0.15, glide: 5, decay: 0.1 }],
                space: { size: 0.55, rt60: 0.25, damping: 5500, predelay: 0.004, mix: 0.16 }
            },
            'window.maximize': {
                notes: [{ v: 'boing', n: 0, t: 0, g: 0.85, pan: -0.2, glide: -7, decay: 0.12 }, { v: 'pop', n: 12, t: 0.1, g: 1, pan: 0.2, t60: 0.06, count: 2, spread: 0.04 }],
                echo: { time: 0.09, feedback: 0.25, tone: 4500, mix: 0.1, spread: 0.6 },
                space: { size: 0.8, rt60: 0.4, damping: 5500, predelay: 0.008, mix: 0.26 }
            },
            'window.minimize': {
                notes: [{ v: 'pop', n: 8, t: 0, g: 0.9, pan: 0.15, t60: 0.04, rise: 0.5 }, { v: 'boing', n: 2, t: 0.06, g: 1, pan: -0.15, glide: 4, decay: 0.08, boing: 0.2 }],
                space: { size: 0.5, rt60: 0.22, damping: 5500, predelay: 0.003, mix: 0.14 }
            },
            'window.restore': {
                notes: [{ v: 'boing', n: 2, t: 0, g: 0.9, pan: -0.15, glide: -4, decay: 0.08, boing: 0.2 }, { v: 'pop', n: 8, t: 0.06, g: 1, pan: 0.15, t60: 0.04 }],
                space: { size: 0.5, rt60: 0.22, damping: 5500, predelay: 0.003, mix: 0.14 }
            },
            'window.snap': {
                notes: [{ v: 'pop', n: 12, t: 0, g: 1, pan: 0, t60: 0.03, rise: 0.4, splash: 0.15, pitchSpread: 0 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'pop', n: 0, t: 0, g: 0.8, pan: 0, t60: 0.06, rise: 0.1, splash: 0.5, attack: 0.005 }, { v: 'pop', n: -5, t: 0.045, g: 0.8, pan: 0, t60: 0.07, rise: 0.1, attack: 0.005 }],
                layers: [{ v: 'puff', t: 0, g: 0.3, attack: 0.02, decay: 0.12, band: [250, 1100] }],
                space: { size: 0.45, rt60: 0.2, damping: 4500, predelay: 0.003, mix: 0.12 }
            },
            'file.drop': {
                notes: [{ v: 'pop', n: -3, t: 0, g: 0.9, pan: 0, t60: 0.1, rise: 0.3, splash: 0.5, attack: 0.005 }, { v: 'pop', n: 4, t: 0.06, g: 0.45, pan: 0.2, t60: 0.04, rise: 0.4 }],
                layers: [{ v: 'puff', t: 0.01, g: 0.15, decay: 0.12 }],
                space: { size: 0.55, rt60: 0.3, damping: 5000, predelay: 0.004, mix: 0.18 }
            },
            'file.trash': {
                notes: [{ v: 'toy', n: 12, t: 0, g: 0.8, pan: 0.15, t60: 0.12 }, { v: 'toy', n: 8, t: 0.05, g: 0.9, pan: 0, t60: 0.12 },
                    { v: 'toy', n: 4, t: 0.1, g: 1, pan: -0.15, t60: 0.16 }],
                layers: [{ v: 'puff', t: 0, g: 0.1, decay: 0.12 }],
                space: { size: 0.55, rt60: 0.3, damping: 5000, predelay: 0.004, mix: 0.16 }
            },
            'file.delete': {
                notes: [{ v: 'thump', n: -2, t: 0, g: 0.85, pan: 0.1, t60: 0.2, attack: 0.005 }, { v: 'boing', n: -8, t: 0.09, g: 0.9, pan: -0.1, glide: 6, decay: 0.2, boing: 0.2 }],
                layers: [{ v: 'puff', t: 0.09, g: 0.12, attack: 0.02, decay: 0.15 }],
                space: { size: 0.65, rt60: 0.4, damping: 4800, predelay: 0.005, mix: 0.2 }
            },
            'space.switch': {
                notes: [{ v: 'boing', n: 6, t: 0.08, g: 0.45, pan: 0, glide: -6, glideTime: 0.1, decay: 0.2, attack: 0.04, boing: 0.2 }],
                layers: [{ v: 'puff', t: 0, g: 0.9, attack: 0.12, decay: 0.2, band: [500, 3500] }],
                space: { size: 0.7, rt60: 0.35, damping: 5500, predelay: 0.006, mix: 0.2 }
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
                notes: [{ v: 'pop', n: 6, t: 0, g: 0.8, pan: -0.1, t60: 0.02, rise: 0.8, attack: 0.001, dur: 0.03 }],
                layers: [{ v: 'tick', t: 0, g: 0.5, tone: 2200, ring: 0.012 }],
                space: { size: 0.4, rt60: 0.08, damping: 5500, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.on': {
                notes: [{ v: 'pop', n: 6, t: 0, g: 0.9, pan: -0.1, t60: 0.02, rise: 0.8, dur: 0.03 }, { v: 'pop', n: 10, t: 0.035, g: 1, pan: 0.1, t60: 0.02, rise: 0.8, dur: 0.03 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.07 }
            },
            'ui.toggle.off': {
                notes: [{ v: 'pop', n: 10, t: 0, g: 0.9, pan: 0.1, t60: 0.02, rise: 0.8, dur: 0.03 }, { v: 'pop', n: 6, t: 0.035, g: 1, pan: -0.1, t60: 0.02, rise: 0.8, dur: 0.03 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.07 }
            },
            'ui.tick': {
                notes: [{ v: 'pop', n: 16, t: 0, g: 0.8, pan: -0.15, t60: 0.02, rise: 0.3, splash: 0.2, attack: 0.001, dur: 0.025 }, { v: 'pop', n: 19, t: 0.012, g: 1, pan: 0.15, t60: 0.02, rise: 0.3, splash: 0.2, attack: 0.001, dur: 0.028 }],
                space: null
            }
        },
        fx: {}
    });
})();
