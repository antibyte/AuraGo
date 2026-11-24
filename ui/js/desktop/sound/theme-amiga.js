// Amiga: crisp 8-bit square and triangle notes with playful arpeggios, dry with a tiny slap echo.
// Key C5 (72), major scale: 0 2 4 5 7 9 11. Short notes, bits kept at 8 or above, duty near 0.5 or 0.25, nothing shrill.
(function () {
    'use strict';
    const slap = (mix, feedback = 0.15) => ({ time: 0.09, feedback, tone: 5000, mix, spread: 0.4 });
    const room = (rt60, mix) => ({ size: 0.4, rt60, damping: 6500, predelay: 0.002, mix });
    self.DesktopSoundEngine.defineTheme({
        id: 'amiga',
        key: 72,
        space: { size: 0.4, rt60: 0.12, damping: 6500, predelay: 0.002, mix: 0.1 },
        voices: {
            sq: { model: 'chip', wave: 'pulse', duty: 0.5, bits: 8, attack: 0.003, decay: 0.16 },
            lead: { model: 'chip', wave: 'pulse', duty: 0.25, bits: 8, attack: 0.003, decay: 0.2, vibrato: 14, vibratoRate: 6, vibratoDelay: 0.06 },
            tri: { model: 'chip', wave: 'tri', bits: 8, attack: 0.004, decay: 0.2 },
            arp: { model: 'chip', wave: 'pulse', duty: 0.25, arp: [4, 7], arpRate: 24, bits: 8, attack: 0.003, decay: 0.4, vibrato: 12, vibratoDelay: 0.1 },
            tick: { model: 'click', material: 'plastic', tone: 2400, ring: 0.015 },
            hat: { model: 'noise', color: 'white', band: [3000, 7000], attack: 0.001, decay: 0.02, width: 0.5 },
            air: { model: 'noise', color: 'pink', band: [1500, 5000], attack: 0.05, decay: 0.2 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'sq', n: 0, t: 0, g: 0.8, pan: -0.25, decay: 0.2 }, { v: 'sq', n: 4, t: 0.13, g: 0.85, pan: 0.1, decay: 0.2 },
                    { v: 'sq', n: 7, t: 0.26, g: 0.9, pan: -0.1, decay: 0.22 }, { v: 'lead', n: 12, t: 0.4, g: 0.85, pan: 0.2, decay: 0.3 },
                    { v: 'arp', n: 4, t: 0.62, g: 0.8, pan: 0, decay: 0.7, arp: [4, 7, 12], arpRate: 20 }],
                layers: [{ v: 'air', t: 0, g: 0.04 }],
                echo: slap(0.2, 0.25),
                space: room(0.3, 0.14)
            },
            'notify.message': {
                notes: [{ v: 'sq', n: 4, t: 0, g: 0.8, pan: -0.2, decay: 0.14 }, { v: 'sq', n: 7, t: 0.1, g: 0.9, pan: 0.15, decay: 0.16 },
                    { v: 'lead', n: 12, t: 0.2, g: 1, pan: 0, decay: 0.3 }],
                echo: slap(0.18),
                space: room(0.25, 0.12)
            },
            'notify.info': {
                notes: [{ v: 'sq', n: 4, t: 0, g: 0.85, pan: -0.15, decay: 0.12 }, { v: 'lead', n: 9, t: 0.09, g: 1, pan: 0.15, decay: 0.22 }],
                echo: slap(0.16),
                space: room(0.2, 0.12)
            },
            'notify.success': {
                notes: [{ v: 'sq', n: -8, t: 0, g: 0.8, pan: -0.2, decay: 0.12 }, { v: 'sq', n: -5, t: 0.08, g: 0.9, pan: 0.1, decay: 0.12 },
                    { v: 'lead', n: 0, t: 0.16, g: 1, pan: 0, decay: 0.3 }],
                echo: slap(0.16),
                space: room(0.2, 0.12)
            },
            'notify.warning': {
                notes: [{ v: 'lead', n: 5, t: 0, g: 1, pan: -0.1, decay: 0.12, vibrato: 0 }, { v: 'lead', n: 5, t: 0.17, g: 0.85, pan: 0.1, decay: 0.14, vibrato: 0 }],
                echo: slap(0.14),
                space: room(0.15, 0.1)
            },
            'notify.error': {
                notes: [{ v: 'tri', n: 6, t: 0, g: 1, pan: -0.1, attack: 0.008, decay: 0.2 }, { v: 'tri', n: 0, t: 0.15, g: 0.9, pan: 0.1, attack: 0.008, decay: 0.28 }],
                layers: [{ v: 'knock', t: 0, g: 0.35 }, { v: 'knock', t: 0.15, g: 0.35 }],
                echo: slap(0.12),
                space: room(0.2, 0.12)
            },
            'dialog.open': {
                notes: [{ v: 'sq', n: -5, t: 0, g: 0.85, pan: -0.15, decay: 0.1 }, { v: 'sq', n: -1, t: 0.07, g: 1, pan: 0.15, decay: 0.14 }],
                echo: slap(0.14),
                space: room(0.15, 0.1)
            },
            'dialog.confirm': {
                notes: [{ v: 'sq', n: 0, t: 0, g: 0.85, pan: -0.15, decay: 0.1 }, { v: 'lead', n: 7, t: 0.08, g: 1, pan: 0.15, decay: 0.2 }],
                echo: slap(0.14),
                space: room(0.18, 0.1)
            },
            'dialog.cancel': {
                notes: [{ v: 'sq', n: 7, t: 0, g: 0.85, pan: 0.15, decay: 0.1 }, { v: 'tri', n: 0, t: 0.08, g: 1, pan: -0.15, decay: 0.14 }],
                echo: slap(0.1),
                space: room(0.12, 0.08)
            },
            'window.open': {
                notes: [{ v: 'sq', n: 0, t: 0, g: 0.9, pan: -0.15, decay: 0.09 }, { v: 'sq', n: 4, t: 0.075, g: 1, pan: 0.2, decay: 0.13 }],
                echo: slap(0.12),
                space: room(0.12, 0.08)
            },
            'window.close': {
                notes: [{ v: 'sq', n: 4, t: 0, g: 0.9, pan: 0.2, decay: 0.08 }, { v: 'sq', n: 0, t: 0.075, g: 1, pan: -0.15, decay: 0.12 }],
                echo: slap(0.1),
                space: room(0.1, 0.07)
            },
            'window.maximize': {
                notes: [{ v: 'sq', n: -5, t: 0, g: 0.85, pan: -0.2, decay: 0.1 }, { v: 'sq', n: 7, t: 0.09, g: 1, pan: 0.2, decay: 0.15 }],
                echo: slap(0.14),
                space: room(0.2, 0.12)
            },
            'window.minimize': {
                notes: [{ v: 'sq', n: 5, t: 0, g: 0.9, pan: 0.15, decay: 0.06 }, { v: 'sq', n: -2, t: 0.055, g: 1, pan: -0.15, decay: 0.09 }],
                space: room(0.1, 0.06)
            },
            'window.restore': {
                notes: [{ v: 'sq', n: -2, t: 0, g: 0.9, pan: -0.15, decay: 0.06 }, { v: 'sq', n: 5, t: 0.055, g: 1, pan: 0.15, decay: 0.09 }],
                space: room(0.1, 0.06)
            },
            'window.snap': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 3400, ring: 0.02 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'tri', n: -12, t: 0, g: 0.9, pan: 0, attack: 0.006, decay: 0.12 }],
                layers: [{ v: 'knock', t: 0, g: 0.6 }],
                space: room(0.15, 0.1)
            },
            'file.drop': {
                notes: [{ v: 'tri', n: -9, t: 0, g: 0.9, pan: 0, attack: 0.006, decay: 0.18 }],
                layers: [{ v: 'knock', t: 0, g: 0.45 }],
                space: room(0.25, 0.12)
            },
            'file.trash': {
                notes: [{ v: 'sq', n: 9, t: 0, g: 0.8, pan: 0.15, decay: 0.05 }, { v: 'sq', n: 4, t: 0.05, g: 0.9, pan: 0, decay: 0.05 },
                    { v: 'sq', n: 0, t: 0.1, g: 1, pan: -0.15, decay: 0.08 }],
                layers: [{ v: 'hat', t: 0, g: 0.12 }],
                space: room(0.15, 0.1)
            },
            'file.delete': {
                notes: [{ v: 'sq', n: 0, t: 0, g: 0.85, pan: 0.1, decay: 0.1 }, { v: 'tri', n: -8, t: 0.09, g: 1, pan: -0.1, decay: 0.24 }],
                layers: [{ v: 'knock', t: 0.09, g: 0.35 }],
                echo: slap(0.12),
                space: room(0.2, 0.12)
            },
            'space.switch': {
                notes: [{ v: 'arp', n: 0, t: 0, g: 0.6, pan: 0, decay: 0.3, arp: [4, 7, 12], arpRate: 28, attack: 0.02 }],
                layers: [{ v: 'air', t: 0, g: 0.5, attack: 0.1, decay: 0.15, band: [1500, 5000] }],
                echo: slap(0.14),
                space: room(0.2, 0.12)
            },
            'menu.open': {
                notes: [{ v: 'sq', n: 0, t: 0, g: 0.9, pan: -0.1, decay: 0.05, dur: 0.085 }, { v: 'sq', n: 7, t: 0.035, g: 1, pan: 0.1, decay: 0.05, dur: 0.085 }],
                space: room(0.1, 0.08)
            },
            'menu.close': {
                notes: [{ v: 'sq', n: 7, t: 0, g: 0.9, pan: 0.1, decay: 0.05, dur: 0.085 }, { v: 'sq', n: 0, t: 0.03, g: 1, pan: -0.1, decay: 0.05, dur: 0.085 }],
                space: room(0.07, 0.06)
            },
            'ui.select': {
                notes: [{ v: 'sq', n: 7, t: 0, g: 0.8, pan: 0, decay: 0.04, dur: 0.07 }],
                layers: [{ v: 'tick', t: 0, g: 0.6, tone: 2000, ring: 0.015 }],
                space: room(0.08, 0.06)
            },
            'ui.toggle.on': {
                notes: [{ v: 'sq', n: 4, t: 0, g: 0.9, pan: -0.1, decay: 0.03, dur: 0.05 }, { v: 'sq', n: 11, t: 0.04, g: 1, pan: 0.1, decay: 0.03, dur: 0.06 }],
                space: room(0.08, 0.06)
            },
            'ui.toggle.off': {
                notes: [{ v: 'sq', n: 11, t: 0, g: 0.9, pan: 0.1, decay: 0.03, dur: 0.05 }, { v: 'sq', n: 4, t: 0.04, g: 1, pan: -0.1, decay: 0.03, dur: 0.06 }],
                space: room(0.08, 0.06)
            },
            'ui.tick': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 3000, ring: 0.008, length: 0.0015 }],
                space: null
            }
        },
        fx: {}
    });
})();
