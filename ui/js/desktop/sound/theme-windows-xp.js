// Windows XP: bright electric-piano chimes, a soft pad swell and soft clicks in a medium hall. Key E-flat 4 (63), major scale.
// The motifs are this theme's own (rising and falling leaps inside the scale); they do not imitate any existing system sound.
(function () {
    'use strict';
    const hall = (rt60, mix) => ({ size: 0.95, rt60, damping: 6000, predelay: 0.012, mix });
    self.DesktopSoundEngine.defineTheme({
        id: 'windows_xp',
        key: 63,
        space: { size: 1, rt60: 0.8, damping: 6000, predelay: 0.014, mix: 0.24 },
        voices: {
            tine: { model: 'bell', kind: 'tine', t60: 0.5, index: 1, attack: 0.004, spread: 0.3 },
            pad: { model: 'pad', partials: 4, detune: 8, attack: 0.15, hold: 0.1, release: 0.5, cutoff: 2500, width: 0.7, shimmer: 0.3 },
            soft: { model: 'click', material: 'felt', tone: 1800, ring: 0.015 },
            air: { model: 'noise', color: 'pink', band: [1500, 6000], attack: 0.05, decay: 0.2 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'pad', n: 4, t: 0, g: 0.6, pan: 0, attack: 0.3, hold: 0.4, release: 1.2 }, { v: 'tine', n: 9, t: 0.12, g: 0.85, pan: -0.2, t60: 0.6 },
                    { v: 'tine', n: 4, t: 0.3, g: 0.8, pan: 0.15, t60: 0.6 }, { v: 'tine', n: 12, t: 0.48, g: 0.9, pan: -0.1, t60: 0.7 },
                    { v: 'tine', n: 16, t: 0.72, g: 0.9, pan: 0.2, t60: 1.1 }],
                layers: [{ v: 'air', t: 0, g: 0.04 }],
                space: { size: 1.2, rt60: 1.4, damping: 5500, predelay: 0.02, mix: 0.32 }
            },
            'notify.message': {
                notes: [{ v: 'tine', n: 9, t: 0, g: 0.85, pan: -0.2, t60: 0.5 }, { v: 'tine', n: 4, t: 0.13, g: 0.8, pan: 0.15, t60: 0.5 },
                    { v: 'tine', n: 12, t: 0.26, g: 1, pan: 0, t60: 0.7 }],
                space: hall(0.9, 0.26)
            },
            'notify.info': {
                notes: [{ v: 'tine', n: 4, t: 0, g: 0.85, pan: -0.15, t60: 0.35 }, { v: 'tine', n: 9, t: 0.11, g: 1, pan: 0.15, t60: 0.5 }],
                space: hall(0.8, 0.24)
            },
            'notify.success': {
                notes: [{ v: 'tine', n: 4, t: 0, g: 0.8, pan: -0.2, t60: 0.3 }, { v: 'tine', n: 7, t: 0.09, g: 0.9, pan: 0.1, t60: 0.3 },
                    { v: 'tine', n: 12, t: 0.18, g: 1, pan: 0, t60: 0.55 }],
                space: hall(0.8, 0.24)
            },
            'notify.warning': {
                notes: [{ v: 'tine', n: 5, t: 0, g: 1, pan: -0.1, t60: 0.3, index: 0.7 }, { v: 'tine', n: 5, t: 0.18, g: 0.85, pan: 0.1, t60: 0.4, index: 0.7 }],
                space: hall(0.6, 0.2)
            },
            'notify.error': {
                notes: [{ v: 'tine', n: 12, t: 0, g: 1, pan: -0.1, t60: 0.3, index: 0.5, attack: 0.009 }, { v: 'tine', n: 6, t: 0.15, g: 0.9, pan: 0.1, t60: 0.35, index: 0.5, attack: 0.009 }],
                layers: [{ v: 'knock', t: 0, g: 0.35 }, { v: 'knock', t: 0.15, g: 0.35 }],
                space: hall(0.5, 0.16)
            },
            'dialog.open': {
                notes: [{ v: 'tine', n: 2, t: 0, g: 0.85, pan: -0.15, t60: 0.2 }, { v: 'tine', n: 7, t: 0.08, g: 1, pan: 0.15, t60: 0.28 }],
                space: hall(0.4, 0.2)
            },
            'dialog.confirm': {
                notes: [{ v: 'tine', n: 4, t: 0, g: 0.85, pan: -0.15, t60: 0.22 }, { v: 'tine', n: 11, t: 0.09, g: 1, pan: 0.15, t60: 0.32 }],
                space: hall(0.45, 0.2)
            },
            'dialog.cancel': {
                notes: [{ v: 'tine', n: 11, t: 0, g: 0.85, pan: 0.15, t60: 0.2 }, { v: 'tine', n: 4, t: 0.09, g: 1, pan: -0.15, t60: 0.22 }],
                space: hall(0.3, 0.16)
            },
            'window.open': {
                notes: [{ v: 'tine', n: 7, t: 0, g: 0.9, pan: -0.15, t60: 0.22 }, { v: 'tine', n: 12, t: 0.085, g: 1, pan: 0.2, t60: 0.28 }],
                layers: [{ v: 'soft', t: 0, g: 0.3 }],
                space: hall(0.4, 0.18)
            },
            'window.close': {
                notes: [{ v: 'tine', n: 12, t: 0, g: 0.9, pan: 0.2, t60: 0.2 }, { v: 'tine', n: 7, t: 0.085, g: 1, pan: -0.15, t60: 0.22 }],
                layers: [{ v: 'soft', t: 0.085, g: 0.3 }],
                space: hall(0.3, 0.16)
            },
            'window.maximize': {
                notes: [{ v: 'tine', n: 4, t: 0, g: 0.85, pan: -0.2, t60: 0.25 }, { v: 'tine', n: 16, t: 0.1, g: 1, pan: 0.2, t60: 0.3 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.12 }],
                space: hall(0.45, 0.26)
            },
            'window.minimize': {
                notes: [{ v: 'tine', n: 9, t: 0, g: 0.9, pan: 0.15, t60: 0.14 }, { v: 'tine', n: 2, t: 0.065, g: 1, pan: -0.15, t60: 0.16 }],
                space: hall(0.25, 0.14)
            },
            'window.restore': {
                notes: [{ v: 'tine', n: 2, t: 0, g: 0.9, pan: -0.15, t60: 0.14 }, { v: 'tine', n: 9, t: 0.065, g: 1, pan: 0.15, t60: 0.16 }],
                space: hall(0.25, 0.14)
            },
            'window.snap': {
                layers: [{ v: 'soft', t: 0, g: 1, material: 'plastic', tone: 3400, ring: 0.02 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'tine', n: 0, t: 0, g: 0.7, pan: 0, t60: 0.14, index: 0.4, attack: 0.007 }],
                layers: [{ v: 'soft', t: 0, g: 0.9, tone: 700, ring: 0.02 }, { v: 'knock', t: 0, g: 0.5 }],
                space: { size: 0.5, rt60: 0.2, damping: 4500, predelay: 0.004, mix: 0.12 }
            },
            'file.drop': {
                notes: [{ v: 'tine', n: -3, t: 0, g: 0.8, pan: 0, t60: 0.2, index: 0.4, attack: 0.007 }],
                layers: [{ v: 'soft', t: 0, g: 0.8, tone: 800, ring: 0.025 }, { v: 'knock', t: 0, g: 0.5 }],
                space: { size: 0.7, rt60: 0.35, damping: 5000, predelay: 0.006, mix: 0.18 }
            },
            'file.trash': {
                notes: [{ v: 'tine', n: 12, t: 0, g: 0.8, pan: 0.15, t60: 0.1 }, { v: 'tine', n: 7, t: 0.05, g: 0.9, pan: 0, t60: 0.1 },
                    { v: 'tine', n: 2, t: 0.1, g: 1, pan: -0.15, t60: 0.14 }],
                layers: [{ v: 'air', t: 0, g: 0.1, attack: 0.02, decay: 0.12 }],
                space: hall(0.3, 0.14)
            },
            'file.delete': {
                notes: [{ v: 'tine', n: 5, t: 0, g: 0.85, pan: 0.1, t60: 0.2 }, { v: 'tine', n: -2, t: 0.09, g: 1, pan: -0.1, t60: 0.28, index: 0.6, attack: 0.006 }],
                layers: [{ v: 'knock', t: 0.09, g: 0.4 }],
                space: hall(0.45, 0.18)
            },
            'space.switch': {
                notes: [{ v: 'pad', n: 4, t: 0, g: 0.9, pan: 0, attack: 0.12, hold: 0, release: 0.25 }, { v: 'tine', n: 12, t: 0.12, g: 0.4, pan: 0.1, t60: 0.25 }],
                layers: [{ v: 'air', t: 0, g: 0.6, attack: 0.12, decay: 0.15, band: [1500, 5000] }],
                space: hall(0.35, 0.2)
            },
            'menu.open': {
                notes: [{ v: 'tine', n: 7, t: 0, g: 0.9, pan: -0.1, t60: 0.1, dur: 0.085, air: 0 }, { v: 'tine', n: 12, t: 0.035, g: 1, pan: 0.1, t60: 0.1, dur: 0.085, air: 0 }],
                space: { size: 0.4, rt60: 0.1, damping: 6000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'tine', n: 12, t: 0, g: 0.9, pan: 0.1, t60: 0.1, dur: 0.085, air: 0 }, { v: 'tine', n: 7, t: 0.03, g: 1, pan: -0.1, t60: 0.1, dur: 0.085, air: 0 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.select': {
                layers: [{ v: 'soft', t: 0, g: 1, tone: 2200, ring: 0.02 }],
                space: { size: 0.4, rt60: 0.08, damping: 6500, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'soft', t: 0, g: 0.9, tone: 2400, ring: 0.018 }, { v: 'soft', t: 0.04, g: 1, tone: 3200, ring: 0.018 }, { v: 'air', t: 0, g: 0.05, attack: 0.005, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.1, damping: 6500, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.off': {
                layers: [{ v: 'soft', t: 0, g: 0.9, tone: 3200, ring: 0.018 }, { v: 'soft', t: 0.04, g: 1, tone: 2400, ring: 0.018 }, { v: 'air', t: 0, g: 0.05, attack: 0.005, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.1, damping: 6500, predelay: 0.002, mix: 0.08 }
            },
            'ui.tick': {
                layers: [{ v: 'soft', t: 0, g: 1, tone: 3400, ring: 0.01, length: 0.0015 }],
                space: null
            }
        },
        fx: {}
    });
})();
