// Black Gloss: "glow, not ding". Polished dark glass that blooms instead of being struck: slow attacks, no strike transient,
// notes sounding TOGETHER as dyads and triads (one swell), a faint soft bell as halo, a long dark hall.
// Key D-flat 5 (73), major scale 0 2 4 5 7 9 11. Register D-flat 4 to A-flat 5. Direction (open / close) is one small step of the
// upper voice arriving 30-40 ms after (or before) the lower one, or a second dyad that overlaps the first; there is no arpeggio.
(function () {
    'use strict';
    // A dyad: lower note `lo` and upper note `hi` of the same voice; `lag` delays the upper note (open) or, negative, the lower one (close).
    const dy = (v, lo, hi, t, g, o, lag) => {
        const l = lag || 0;
        return [Object.assign({ v, n: lo, t: l < 0 ? t - l : t, g: g * 0.85, pan: -0.2 }, o), Object.assign({ v, n: hi, t: l > 0 ? t + l : t, g, pan: 0.2 }, o)];
    };
    // A triad (root, middle, top) struck as one swell.
    const tri = (v, a, b, c, t, g, o) => [Object.assign({ v, n: a, t, g: g * 0.8, pan: -0.25 }, o), Object.assign({ v, n: b, t, g: g * 0.85, pan: 0.05 }, o),
        Object.assign({ v, n: c, t: t + 0.012, g, pan: 0.25 }, o)];
    self.DesktopSoundEngine.defineTheme({
        id: 'black_gloss',
        key: 73,
        space: { size: 1.3, rt60: 1.3, damping: 4200, predelay: 0.02, mix: 0.3 },
        voices: {
            glow: { model: 'glass', t60: 0.9, damp: 1.3, beat: 0.6, attack: 0.045, tilt: -1, width: 1, strike: 0, air: 0.05 },
            bloom: { model: 'glass', t60: 0.22, damp: 1.3, beat: 0.5, attack: 0.024, tilt: -1, width: 0.9, strike: 0, air: 0.04 },
            halo: { model: 'bell', kind: 'bell', t60: 1, index: 0.35, attack: 0.03, spread: 0.4 },
            low: { model: 'string', kind: 'muted', t60: 0.22, brightness: 0.16, pick: 0.12, body: 0.15 },
            tick: { model: 'click', material: 'glass', tone: 2600, ring: 0.03, noise: 0.55, q: 2.5 },
            air: { model: 'noise', color: 'pink', band: [500, 3000], attack: 0.07, decay: 0.22 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [...dy('glow', -12, -5, 0, 0.8, { t60: 1.1, attack: 0.05 }), ...dy('glow', -8, -3, 0.3, 0.85, { t60: 1.2, attack: 0.05 }),
                    ...tri('glow', -5, 0, 4, 0.6, 0.9, { t60: 1.2, attack: 0.05 }), ...tri('glow', -12, -8, 2, 0.9, 0.95, { t60: 1.2, attack: 0.05 }),
                    { v: 'halo', n: 7, t: 0.9, g: 0.35, pan: 0.1, t60: 1.2 }],
                layers: [{ v: 'air', t: 0, g: 0.05 }],
                space: { size: 1.5, rt60: 1.6, damping: 4000, predelay: 0.028, mix: 0.4 }
            },
            'notify.message': {
                notes: [...dy('glow', -17, -10, 0, 0.8, { t60: 0.5, attack: 0.05 }), ...dy('glow', -13, -8, 0.15, 0.9, { t60: 0.55, attack: 0.05 }), ...tri('glow', -10, -5, -1, 0.3, 1, { t60: 0.7, attack: 0.05 }),
                    { v: 'halo', n: -1, t: 0.3, g: 0.45, pan: 0.1, t60: 0.5 }],
                space: { size: 1.3, rt60: 0.9, damping: 2800, predelay: 0.024, mix: 0.34 }
            },
            'notify.info': {
                notes: [...dy('glow', -13, -6, 0, 0.85, { t60: 0.6, attack: 0.05 }), ...dy('glow', -10, -3, 0.16, 1, { t60: 0.8, attack: 0.05 }),
                    { v: 'halo', n: -3, t: 0.16, g: 0.6, pan: 0.15, t60: 0.5 }],
                space: { size: 1.3, rt60: 1.0, damping: 2800, predelay: 0.02, mix: 0.32 }
            },
            'notify.success': {
                notes: [...dy('glow', -10, -3, 0, 0.8, { t60: 0.55, attack: 0.03 }), ...dy('glow', -8, -1, 0.14, 0.9, { t60: 0.6, attack: 0.03 }),
                    ...tri('glow', -12, 0, 4, 0.28, 1, { t60: 0.65, attack: 0.03 }), { v: 'halo', n: 0, t: 0.28, g: 0.3, t60: 0.5 }],
                space: { size: 1.3, rt60: 0.85, damping: 4200, predelay: 0.018, mix: 0.32 }
            },
            'notify.warning': {
                notes: [...dy('glow', -5, 0, 0, 1, { t60: 0.45, tilt: -0.9, attack: 0.03 }), ...dy('glow', -5, 0, 0.24, 0.85, { t60: 0.55, tilt: -0.9, attack: 0.03 }),
                    { v: 'halo', n: -5, t: 0.24, g: 0.2, pan: 0.1, t60: 0.3 }],
                layers: [{ v: 'tick', t: 0, g: 0.3, tone: 2400 }],
                space: { size: 1.1, rt60: 0.9, damping: 3800, predelay: 0.012, mix: 0.28 }
            },
            'notify.error': {
                notes: [...dy('glow', -7, -1, 0, 1, { t60: 0.4, tilt: -1, damp: 1.2, attack: 0.03 }), ...dy('glow', -13, -7, 0.18, 0.95, { t60: 0.45, tilt: -1, damp: 1.2, attack: 0.03 }),
                    { v: 'low', n: -13, t: 0.18, g: 0.6, pan: 0.1, t60: 0.3 }],
                layers: [{ v: 'knock', t: 0, g: 0.35 }, { v: 'knock', t: 0.18, g: 0.4 }],
                space: { size: 1, rt60: 0.6, damping: 3600, predelay: 0.01, mix: 0.24 }
            },
            'dialog.open': {
                notes: [...dy('bloom', -10, -3, 0, 1, { t60: 0.25 }, 0.04), { v: 'halo', n: -3, t: 0.04, g: 0.25, pan: 0.15, t60: 0.25 }],
                space: { size: 1.1, rt60: 0.45, damping: 4500, predelay: 0.012, mix: 0.26 }
            },
            'dialog.confirm': {
                notes: [...dy('bloom', -5, 2, 0, 0.85, { t60: 0.22 }), ...dy('bloom', 0, 4, 0.12, 1, { t60: 0.28 }), { v: 'halo', n: 4, t: 0.13, g: 0.25, pan: 0.15, t60: 0.3 }],
                space: { size: 1.1, rt60: 0.5, damping: 4500, predelay: 0.012, mix: 0.26 }
            },
            'dialog.cancel': {
                notes: [...dy('bloom', 0, 4, 0, 0.85, { t60: 0.2 }), ...dy('bloom', -5, 2, 0.12, 1, { t60: 0.22 }), { v: 'halo', n: -5, t: 0.13, g: 0.2, pan: -0.15, t60: 0.25 }],
                space: { size: 1, rt60: 0.35, damping: 3800, predelay: 0.012, mix: 0.22 }
            },
            'window.open': {
                notes: dy('bloom', -5, 2, 0, 1, { t60: 0.26 }, 0.035),
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.12 }],
                space: { size: 1.1, rt60: 0.5, damping: 4800, predelay: 0.012, mix: 0.26 }
            },
            'window.close': {
                notes: dy('bloom', -5, 2, 0, 1, { t60: 0.22 }, -0.035),
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.12 }],
                space: { size: 1.1, rt60: 0.45, damping: 4200, predelay: 0.012, mix: 0.24 }
            },
            'window.maximize': {
                notes: [...dy('bloom', -12, 4, 0, 1, { t60: 0.3 }, 0.04), { v: 'halo', n: 4, t: 0.04, g: 0.25, pan: 0.2, t60: 0.3 }],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.12 }],
                space: { size: 1.3, rt60: 0.5, damping: 4800, predelay: 0.014, mix: 0.3 }
            },
            'window.minimize': {
                notes: dy('bloom', -10, -3, 0, 1, { t60: 0.18, attack: 0.016 }, -0.03),
                layers: [{ v: 'tick', t: 0, g: 0.2, tone: 2400 }],
                space: { size: 0.8, rt60: 0.35, damping: 4200, predelay: 0.008, mix: 0.18 }
            },
            'window.restore': {
                notes: dy('bloom', -10, -3, 0, 1, { t60: 0.2, attack: 0.016 }, 0.03),
                layers: [{ v: 'tick', t: 0, g: 0.2, tone: 2400 }],
                space: { size: 0.8, rt60: 0.35, damping: 4800, predelay: 0.008, mix: 0.18 }
            },
            'window.snap': {
                notes: [{ v: 'bloom', n: -8, t: 0.002, g: 0.3, t60: 0.08, dur: 0.07, attack: 0.008 }],
                layers: [{ v: 'tick', t: 0, g: 1, tone: 2400, ring: 0.02 }],
                space: { size: 0.4, rt60: 0.1, damping: 5000, predelay: 0.002, mix: 0.08 }
            },
            'window.deny': {
                notes: [{ v: 'low', n: -12, t: 0, g: 0.8, pan: 0, t60: 0.16, attack: 0.006 },
                    { v: 'glow', n: -12, t: 0, g: 0.8, pan: 0, t60: 0.2, tilt: -1, damp: 1.5, attack: 0.02 },
                    { v: 'halo', n: -12, t: 0, g: 0.2, pan: 0, t60: 0.12 }],
                layers: [{ v: 'knock', t: 0, g: 0.3 }],
                space: { size: 0.7, rt60: 0.3, damping: 3800, predelay: 0.004, mix: 0.14 }
            },
            'file.drop': {
                notes: [{ v: 'low', n: -7, t: 0, g: 0.9, pan: 0, t60: 0.24 }, { v: 'glow', n: -12, t: 0.02, g: 0.9, pan: -0.15, t60: 0.28, tilt: -1, attack: 0.025 },
                    { v: 'glow', n: -5, t: 0.02, g: 0.7, pan: 0.15, t60: 0.28, tilt: -1, attack: 0.025 }],
                layers: [{ v: 'knock', t: 0, g: 0.5 }],
                space: { size: 0.9, rt60: 0.45, damping: 3800, predelay: 0.006, mix: 0.22 }
            },
            'file.trash': {
                notes: [...dy('bloom', 0, 4, 0, 0.8, { t60: 0.16, attack: 0.012 }), ...dy('bloom', -5, -1, 0.06, 0.9, { t60: 0.16, attack: 0.012 }),
                    ...dy('bloom', -12, -8, 0.12, 1, { t60: 0.22, attack: 0.012 })],
                layers: [{ v: 'air', t: 0, g: 0.08, attack: 0.02, decay: 0.12 }],
                space: { size: 0.9, rt60: 0.3, damping: 4200, predelay: 0.006, mix: 0.2 }
            },
            'file.delete': {
                notes: [...dy('glow', -8, -3, 0, 0.85, { t60: 0.25, tilt: -0.9 }), { v: 'low', n: -12, t: 0.12, g: 1, pan: -0.1, t60: 0.3 },
                    ...dy('glow', -12, -5, 0.12, 0.6, { t60: 0.3, tilt: -1, attack: 0.03 })],
                layers: [{ v: 'knock', t: 0.12, g: 0.4 }],
                space: { size: 1, rt60: 0.4, damping: 3800, predelay: 0.008, mix: 0.24 }
            },
            'space.switch': {
                notes: tri('glow', -8, -3, 2, 0.05, 0.55, { t60: 0.28, attack: 0.05 }),
                layers: [{ v: 'air', t: 0, g: 0.9, attack: 0.15, decay: 0.2, band: [500, 3000] }],
                space: { size: 1.2, rt60: 0.45, damping: 4500, predelay: 0.01, mix: 0.28 }
            },
            'menu.open': {
                notes: dy('bloom', -8, -3, 0, 1, { t60: 0.1, attack: 0.012, air: 0, dur: 0.11 }, 0.035),
                layers: [{ v: 'tick', t: 0, g: 0.25, tone: 2400 }],
                space: { size: 0.6, rt60: 0.14, damping: 5000, predelay: 0.002, mix: 0.12 }
            },
            'menu.close': {
                notes: dy('bloom', -8, -3, 0, 1, { t60: 0.08, attack: 0.012, air: 0, dur: 0.1 }, -0.03),
                layers: [{ v: 'tick', t: 0, g: 0.25, tone: 2400 }],
                space: { size: 0.6, rt60: 0.05, damping: 5000, predelay: 0.002, mix: 0.07 }
            },
            'ui.select': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 2400, ring: 0.03 }],
                space: null
            },
            'ui.toggle.on': {
                notes: [{ v: 'bloom', n: -3, t: 0, g: 0.25, t60: 0.06, dur: 0.07, attack: 0.008, air: 0 }, { v: 'bloom', n: 4, t: 0.035, g: 0.3, t60: 0.06, dur: 0.07, attack: 0.008, air: 0 }],
                layers: [{ v: 'tick', t: 0, g: 0.9, tone: 2100, ring: 0.025 }, { v: 'tick', t: 0.035, g: 1, tone: 2800, ring: 0.025 }],
                space: null
            },
            'ui.toggle.off': {
                notes: [{ v: 'bloom', n: 4, t: 0, g: 0.25, t60: 0.06, dur: 0.07, attack: 0.008, air: 0 }, { v: 'bloom', n: -3, t: 0.035, g: 0.3, t60: 0.06, dur: 0.07, attack: 0.008, air: 0 }],
                layers: [{ v: 'tick', t: 0, g: 0.9, tone: 2800, ring: 0.025 }, { v: 'tick', t: 0.035, g: 1, tone: 2100, ring: 0.025 }],
                space: null
            },
            'ui.tick': {
                layers: [{ v: 'tick', t: 0, g: 1, tone: 3600, ring: 0.014, length: 0.0015 }],
                space: null
            }
        },
        fx: {}
    });
})();
