// Windows XP: electric-piano chords over a pad. Tines are played as CHORDS (thirds, sixths, a major seventh or added ninth now and then),
// not single-note lines; notices, dialogs and the start sound carry a pad under the same chord that enters a little before the tines.
// Key E-flat 4 (63), major scale; register E-flat 3 to E-flat 5, medium hall, unhurried timing (120-180 ms between chords).
// The motifs are this theme's own (chord steps inside the scale); they do not imitate any existing system sound.
(function () {
    'use strict';
    const hall = (rt60, mix) => ({ size: 0.95, rt60, damping: 5500, predelay: 0.012, mix });
    const keys = { rate: 0.7, depth: 0.003, delay: 0.014, mix: 0.25 };
    // A tine chord struck at `t`; `roll` > 0 strums upward (the notes `roll` seconds apart), < 0 strums downward.
    const ch = (ns, t, g, o, roll) => {
        const r = roll || 0, order = r < 0 ? ns.slice().reverse() : ns, step = Math.abs(r);
        return order.map((n, i) => Object.assign({ v: 'tine', n, t: Math.round((t + (r === 0 ? i * 0.004 : i * step)) * 1000) / 1000, g: g * (0.8 + 0.2 * (i + 1) / ns.length), pan: ((i % 3) - 1) * 0.18 }, o));
    };
    // The pad under a chord: the same pitches, entering `lead` seconds before the tines.
    const pad = (ns, t, g, o) => ns.map((n, i) => Object.assign({ v: 'pad', n, t, g, pan: ((i % 3) - 1) * 0.1 }, o));
    self.DesktopSoundEngine.defineTheme({
        id: 'windows_xp',
        key: 63,
        space: { size: 1, rt60: 0.8, damping: 5500, predelay: 0.014, mix: 0.24 },
        voices: {
            tine: { model: 'bell', kind: 'tine', t60: 0.5, index: 0.8, attack: 0.005, spread: 0.35 },
            pad: { model: 'pad', partials: 3, detune: 6, attack: 0.08, hold: 0.05, release: 0.35, cutoff: 2200, width: 0.8, shimmer: 0.3 },
            soft: { model: 'click', material: 'felt', tone: 1600, ring: 0.015 },
            air: { model: 'noise', color: 'pink', band: [1500, 6000], attack: 0.05, decay: 0.2 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [...pad([0, 4, 7], 0, 0.55, { attack: 0.3, hold: 0.55, release: 1.2 }), ...ch([0, 4, 7], 0.1, 0.8, { t60: 0.6 }, 0.012),
                    ...ch([5, 9, 12], 0.32, 0.85, { t60: 0.6 }, 0.012), ...ch([7, 11, 14], 0.54, 0.9, { t60: 0.6 }, 0.012),
                    ...ch([0, 4, 7, 11, 12], 0.8, 0.95, { t60: 1.1 }, 0.014)],
                layers: [{ v: 'air', t: 0, g: 0.04 }],
                chorus: keys,
                space: { size: 1.2, rt60: 1.4, damping: 5000, predelay: 0.02, mix: 0.32 }
            },
            'notify.message': {
                notes: [...pad([-3, 0, 4], 0, 0.45, { attack: 0.07, hold: 0.1, release: 0.5 }), ...ch([-3, 0, 4], 0.06, 0.85, { t60: 0.5 }),
                    ...ch([0, 4, 7, 11], 0.22, 1, { t60: 0.75 })],
                chorus: keys,
                space: hall(0.9, 0.26)
            },
            'notify.info': {
                notes: [...pad([0, 4], 0, 0.4, { attack: 0.06, hold: 0.05, release: 0.4 }), ...ch([0, 4], 0.05, 0.85, { t60: 0.4 }), ...ch([4, 9], 0.19, 1, { t60: 0.55 })],
                chorus: keys,
                space: hall(0.8, 0.24)
            },
            'notify.success': {
                notes: [...pad([0, 4, 7], 0, 0.45, { attack: 0.07, hold: 0.1, release: 0.5 }), ...ch([5, 9], 0.05, 0.8, { t60: 0.35 }),
                    ...ch([7, 11], 0.18, 0.9, { t60: 0.4 }), ...ch([0, 4, 7, 12], 0.32, 1, { t60: 0.6 })],
                chorus: keys,
                space: hall(0.8, 0.24)
            },
            'notify.warning': {
                notes: [...pad([0, 5, 7], 0, 0.35, { attack: 0.05, hold: 0.05, release: 0.3 }), ...ch([0, 5, 7], 0.05, 1, { t60: 0.35, index: 0.6 }),
                    ...ch([0, 5, 7], 0.23, 0.85, { t60: 0.45, index: 0.6 })],
                space: hall(0.6, 0.2)
            },
            'notify.error': {
                notes: [...ch([0, 6], 0, 1, { t60: 0.3, index: 0.5, attack: 0.009 }), ...ch([-6, 0], 0.16, 0.9, { t60: 0.4, index: 0.5, attack: 0.009 })],
                layers: [{ v: 'knock', t: 0, g: 0.35 }, { v: 'knock', t: 0.16, g: 0.35 }],
                space: hall(0.5, 0.16)
            },
            'dialog.open': {
                notes: [...pad([-3, 2], 0, 0.4, { attack: 0.05, hold: 0, release: 0.3 }), ...ch([-3, 2], 0.05, 0.9, { t60: 0.25 }, 0.015)],
                space: hall(0.4, 0.2)
            },
            'dialog.confirm': {
                notes: [...pad([0, 4], 0, 0.4, { attack: 0.05, hold: 0, release: 0.3 }), ...ch([0, 4], 0.05, 0.85, { t60: 0.22 }), ...ch([4, 7, 11], 0.18, 1, { t60: 0.32 })],
                space: hall(0.45, 0.2)
            },
            'dialog.cancel': {
                notes: [...pad([4, 7], 0, 0.35, { attack: 0.05, hold: 0, release: 0.25 }), ...ch([4, 7, 11], 0.05, 0.85, { t60: 0.2 }), ...ch([0, 4], 0.18, 1, { t60: 0.22 })],
                space: hall(0.3, 0.16)
            },
            'window.open': {
                notes: [{ v: 'pad', n: 0, t: 0, g: 0.9, attack: 0.05, hold: 0.02, release: 0.2 }, { v: 'pad', n: 7, t: 0, g: 0.9, attack: 0.05, hold: 0.02, release: 0.2 },
                    ...ch([0, 7], 0.04, 0.9, { t60: 0.25 }, 0.03)],
                layers: [{ v: 'soft', t: 0.04, g: 0.2 }],
                space: hall(0.4, 0.18)
            },
            'window.close': {
                notes: [{ v: 'pad', n: 0, t: 0, g: 0.4, attack: 0.05, hold: 0, release: 0.1 }, { v: 'pad', n: 7, t: 0, g: 0.4, attack: 0.05, hold: 0, release: 0.1 },
                    ...ch([0, 7], 0.04, 0.9, { t60: 0.2 }, -0.03)],
                layers: [{ v: 'soft', t: 0.07, g: 0.2 }],
                space: hall(0.3, 0.16)
            },
            'window.maximize': {
                notes: [{ v: 'pad', n: -5, t: 0, g: 0.5, attack: 0.06, hold: 0, release: 0.15 }, { v: 'pad', n: 7, t: 0, g: 0.5, attack: 0.06, hold: 0, release: 0.15 },
                    ...ch([-5, 4, 11], 0.05, 0.95, { t60: 0.28 }, 0.04)],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.12 }],
                space: hall(0.45, 0.26)
            },
            'window.minimize': {
                notes: ch([4, 9], 0.02, 1, { t60: 0.14 }, -0.025),
                space: hall(0.25, 0.14)
            },
            'window.restore': {
                notes: ch([4, 9], 0.02, 1, { t60: 0.15 }, 0.025),
                space: hall(0.25, 0.14)
            },
            'window.snap': {
                layers: [{ v: 'soft', t: 0, g: 1, material: 'plastic', tone: 3000, ring: 0.02 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'tine', n: -5, t: 0, g: 0.7, pan: 0, t60: 0.14, index: 0.4, attack: 0.007 }, { v: 'tine', n: -1, t: 0, g: 0.5, pan: 0, t60: 0.12, index: 0.4, attack: 0.007 }],
                layers: [{ v: 'soft', t: 0, g: 0.9, tone: 700, ring: 0.02 }, { v: 'knock', t: 0, g: 0.5 }],
                space: { size: 0.5, rt60: 0.2, damping: 4500, predelay: 0.004, mix: 0.12 }
            },
            'file.drop': {
                notes: ch([-3, 0], 0.004, 0.8, { t60: 0.2, index: 0.4, attack: 0.007 }),
                layers: [{ v: 'soft', t: 0, g: 0.8, tone: 900, ring: 0.025 }, { v: 'knock', t: 0, g: 0.5 }],
                space: { size: 0.7, rt60: 0.35, damping: 4500, predelay: 0.006, mix: 0.18 }
            },
            'file.trash': {
                notes: ch([0, 4, 7, 12], 0, 1, { t60: 0.1 }, -0.035),
                layers: [{ v: 'air', t: 0, g: 0.1, attack: 0.02, decay: 0.12 }],
                space: hall(0.3, 0.14)
            },
            'file.delete': {
                notes: [...ch([0, 4, 7], 0, 0.85, { t60: 0.18 }, -0.02), ...ch([-5, -1], 0.1, 1, { t60: 0.26, index: 0.6, attack: 0.006 })],
                layers: [{ v: 'knock', t: 0.1, g: 0.4 }],
                space: hall(0.45, 0.18)
            },
            'space.switch': {
                notes: [{ v: 'pad', n: 0, t: 0, g: 0.6, attack: 0.1, hold: 0, release: 0.2 }, { v: 'pad', n: 7, t: 0, g: 0.6, attack: 0.1, hold: 0, release: 0.2 },
                    ...ch([4, 12], 0.12, 0.4, { t60: 0.25 })],
                layers: [{ v: 'air', t: 0, g: 0.6, attack: 0.12, decay: 0.15, band: [1500, 5000] }],
                space: hall(0.35, 0.2)
            },
            'menu.open': {
                notes: [{ v: 'tine', n: 4, t: 0.002, g: 0.4, t60: 0.1, dur: 0.085, air: 0.03 }, { v: 'tine', n: 9, t: 0.035, g: 0.5, t60: 0.1, dur: 0.085, air: 0.03 }],
                layers: [{ v: 'soft', t: 0, g: 1, tone: 1500, ring: 0.02 }, { v: 'air', t: 0, g: 0.07, attack: 0.005, decay: 0.07 }],
                space: { size: 0.4, rt60: 0.1, damping: 5500, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'tine', n: 9, t: 0.002, g: 0.4, t60: 0.1, dur: 0.085, air: 0.03 }, { v: 'tine', n: 4, t: 0.03, g: 0.5, t60: 0.1, dur: 0.085, air: 0.03 }],
                layers: [{ v: 'soft', t: 0, g: 1, tone: 1500, ring: 0.02 }, { v: 'air', t: 0, g: 0.07, attack: 0.005, decay: 0.06 }],
                space: { size: 0.4, rt60: 0.07, damping: 5500, predelay: 0.002, mix: 0.06 }
            },
            'ui.select': {
                layers: [{ v: 'soft', t: 0, g: 1, tone: 1800, ring: 0.02 }],
                space: { size: 0.4, rt60: 0.08, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'soft', t: 0, g: 0.9, tone: 1500, ring: 0.018 }, { v: 'soft', t: 0.04, g: 1, tone: 2100, ring: 0.018 }, { v: 'air', t: 0, g: 0.05, attack: 0.005, decay: 0.05 }],
                notes: [{ v: 'tine', n: 4, t: 0.04, g: 0.3, t60: 0.06, dur: 0.07, air: 0 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.07 }
            },
            'ui.toggle.off': {
                layers: [{ v: 'soft', t: 0, g: 0.9, tone: 2100, ring: 0.018 }, { v: 'soft', t: 0.04, g: 1, tone: 1500, ring: 0.018 }, { v: 'air', t: 0, g: 0.05, attack: 0.005, decay: 0.05 }],
                notes: [{ v: 'tine', n: 0, t: 0.04, g: 0.3, t60: 0.06, dur: 0.07, air: 0 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.07 }
            },
            'ui.tick': {
                layers: [{ v: 'soft', t: 0, g: 1, tone: 2800, ring: 0.01, length: 0.0015 }],
                space: null
            }
        },
        fx: {}
    });
})();
