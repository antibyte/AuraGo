// Lollipop: "music box". Very short, bright celesta notes in quick ornaments (grace notes, three- to five-note runs and turns with
// 35-55 ms between notes), a glockenspiel doubling the top note an octave up, candy pops as punctuation on the last note, a tiny bright room.
// Key B-flat 5 (82), major scale 0 2 4 5 7 9 11. Frequent events sit in the octave below the key; no long tails.
(function () {
    'use strict';
    // A quick run on the music-box tines: `ns` one after the other `gap` seconds apart, rising in level; the last note rings a little
    // longer (`ring`) and is doubled by a soft celesta chime.
    const run = (v, ns, t, gap, g, ring, o) => {
        const out = [];
        ns.forEach((n, i) => {
            const last = i === ns.length - 1, at = Math.round((t + i * gap) * 1000) / 1000, pan = i % 2 ? 0.2 : -0.2;
            const tine = Object.assign({ v: v === 'cel' ? 'box' : v, n, t: at, g: g * (last ? 1 : 0.6 + 0.3 * i / ns.length), pan, t60: last ? ring : 0.06 }, o);
            if (v === 'cel') delete tine.index; // `index` belongs to the celesta only
            out.push(tine);
            if (last && v === 'cel') out.push(Object.assign({ v, n, t: at, g: g * 0.5, pan, t60: ring }, o));
        });
        return out;
    };
    // Doubling of the last note: glockenspiel an octave up, quiet, plus an optional candy pop on or just after it.
    const top = (n, t, g, pop) => [{ v: 'glock', n: n + 12, t, g, pan: 0.15, t60: 0.2 }].concat(pop ? [{ v: 'pop', n, t: t + 0.006, g: pop, pan: 0.1 }] : []);
    self.DesktopSoundEngine.defineTheme({
        id: 'lollipop',
        key: 82,
        space: { size: 0.5, rt60: 0.3, damping: 9000, predelay: 0.004, mix: 0.18 },
        voices: {
            mini: { model: 'bell', kind: 'celesta', t60: 0.08, index: 0.8, attack: 0.003, spread: 0.3 },
            box: { model: 'bar', kind: 'kalimba', t60: 0.12, hardness: 0.35, attack: 0.002, mallet: 0.25, body: 0, spread: 2 },
            cel: { model: 'bell', kind: 'celesta', t60: 0.15, index: 0.9, attack: 0.002, spread: 0.3 },
            glock: { model: 'bar', kind: 'glockenspiel', t60: 0.25, hardness: 0.15, attack: 0.002, mallet: 0.2 },
            pop: { model: 'drop', rise: 0.7, t60: 0.05, attack: 0.003, splash: 0.15, width: 0.3 },
            low: { model: 'string', kind: 'muted', t60: 0.2, brightness: 0.2, pick: 0.15, body: 0.15 },
            tick: { model: 'click', material: 'plastic', tone: 2600, ring: 0.012, noise: 0.6, q: 2.5 },
            air: { model: 'noise', color: 'pink', band: [1800, 7000], attack: 0.05, decay: 0.18 },
            knock: { model: 'noise', color: 'pink', band: [250, 1000], attack: 0.001, decay: 0.03 }
        },
        events: {
            'session.start': {
                notes: [...run('cel', [-12, -7, -3, 0, 4], 0, 0.11, 0.8, 0.3), ...run('cel', [-5, -1, -3, 0, 2], 0.62, 0.05, 0.85, 0.3),
                    ...run('cel', [0, 2, 0, -1, 4, 7], 1.02, 0.05, 0.95, 0.9), ...top(7, 1.27, 0.35, 0.35)],
                layers: [{ v: 'air', t: 0, g: 0.05 }],
                space: { size: 0.8, rt60: 0.75, damping: 8500, predelay: 0.008, mix: 0.26 }
            },
            'notify.message': {
                notes: [...run('cel', [-5, -3, 0, -1, 4], 0, 0.045, 0.9, 0.4), ...top(4, 0.18, 0.3, 0.35)],
                space: { size: 0.6, rt60: 0.45, damping: 9000, predelay: 0.006, mix: 0.2 }
            },
            'notify.info': {
                notes: [...run('cel', [-5, -1, 2], 0, 0.045, 0.9, 0.3), ...top(2, 0.09, 0.3, 0.3)],
                space: { size: 0.55, rt60: 0.4, damping: 9000, predelay: 0.005, mix: 0.18 }
            },
            'notify.success': {
                notes: [...run('cel', [-12, -7, -3, 0], 0, 0.05, 0.9, 0.4), ...top(0, 0.15, 0.3, 0.35)],
                space: { size: 0.6, rt60: 0.45, damping: 9000, predelay: 0.006, mix: 0.2 }
            },
            'notify.warning': {
                notes: [{ v: 'cel', n: -9, t: 0, g: 0.4, t60: 0.08, index: 0.6, attack: 0.005 }, ...run('cel', [-7, -7], 0.04, 0.16, 1, 0.22, { index: 0.6, attack: 0.006 }),
                    { v: 'glock', n: -7, t: 0.2, g: 0.2, t60: 0.15 }],
                layers: [{ v: 'tick', t: 0, g: 0.3, tone: 1800 }],
                space: { size: 0.5, rt60: 0.35, damping: 7500, predelay: 0.005, mix: 0.18 }
            },
            'notify.error': {
                notes: [{ v: 'cel', n: -12, t: 0, g: 1, pan: -0.1, t60: 0.22, index: 0.5, attack: 0.009 },
                    { v: 'cel', n: -18, t: 0.15, g: 0.9, pan: 0.1, t60: 0.3, index: 0.5, attack: 0.01 },
                    { v: 'low', n: -18, t: 0.15, g: 0.7, pan: 0.1, t60: 0.28 }],
                layers: [{ v: 'knock', t: 0, g: 0.4 }, { v: 'knock', t: 0.15, g: 0.4 }],
                space: { size: 0.5, rt60: 0.35, damping: 6000, predelay: 0.006, mix: 0.16 }
            },
            'dialog.open': {
                notes: [{ v: 'cel', n: -14, t: 0, g: 0.4, t60: 0.06, pan: -0.2 }, ...run('cel', [-12, -7], 0.035, 0.045, 1, 0.2)],
                space: { size: 0.5, rt60: 0.3, damping: 9000, predelay: 0.005, mix: 0.18 }
            },
            'dialog.confirm': {
                notes: [...run('cel', [-8, -4, -1], 0, 0.045, 1, 0.26), ...top(-1, 0.09, 0.25, 0.3)],
                space: { size: 0.55, rt60: 0.35, damping: 9000, predelay: 0.005, mix: 0.18 }
            },
            'dialog.cancel': {
                notes: [...run('cel', [-1, -4, -8], 0, 0.045, 1, 0.22)],
                space: { size: 0.5, rt60: 0.3, damping: 7500, predelay: 0.005, mix: 0.16 }
            },
            'window.open': {
                notes: run('cel', [-12, -8, -5], 0, 0.04, 1, 0.2),
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.1 }],
                space: { size: 0.5, rt60: 0.3, damping: 9000, predelay: 0.005, mix: 0.18 }
            },
            'window.close': {
                notes: run('cel', [-5, -8, -12], 0, 0.04, 1, 0.18),
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.1 }],
                space: { size: 0.5, rt60: 0.25, damping: 7500, predelay: 0.005, mix: 0.16 }
            },
            'window.maximize': {
                notes: [...run('cel', [-17, -10, -5], 0, 0.05, 1, 0.24), ...top(-5, 0.1, 0.22, 0.28)],
                layers: [{ v: 'air', t: 0, g: 0.05, decay: 0.1 }],
                space: { size: 0.7, rt60: 0.4, damping: 9000, predelay: 0.008, mix: 0.24 }
            },
            'window.minimize': {
                notes: run('cel', [-8, -12, -15], 0, 0.035, 1, 0.14),
                layers: [{ v: 'tick', t: 0, g: 0.2, tone: 2200 }],
                space: { size: 0.45, rt60: 0.2, damping: 7500, predelay: 0.004, mix: 0.14 }
            },
            'window.restore': {
                notes: run('cel', [-15, -12, -8], 0, 0.035, 1, 0.16),
                layers: [{ v: 'tick', t: 0, g: 0.2, tone: 2200 }],
                space: { size: 0.45, rt60: 0.2, damping: 7500, predelay: 0.004, mix: 0.14 }
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
                notes: [{ v: 'low', n: -17, t: 0, g: 0.6, pan: 0, t60: 0.2 }, { v: 'pop', n: -10, t: 0.004, g: 0.8, pan: -0.1, t60: 0.07 }, { v: 'pop', n: -3, t: 0.06, g: 0.5, pan: 0.15, t60: 0.05 }],
                layers: [{ v: 'knock', t: 0, g: 0.3 }],
                space: { size: 0.5, rt60: 0.25, damping: 6500, predelay: 0.006, mix: 0.16 }
            },
            'file.trash': {
                notes: [...run('cel', [-3, -7, -12, -17], 0, 0.04, 1, 0.16), { v: 'pop', n: -17, t: 0.126, g: 0.3 }],
                layers: [{ v: 'air', t: 0, g: 0.08, attack: 0.02, decay: 0.1 }],
                space: { size: 0.5, rt60: 0.25, damping: 7000, predelay: 0.006, mix: 0.16 }
            },
            'file.delete': {
                notes: [...run('cel', [-8, -12], 0, 0.05, 0.9, 0.2, { index: 0.7 }), { v: 'low', n: -17, t: 0.09, g: 1, pan: -0.1, t60: 0.28 },
                    { v: 'cel', n: -17, t: 0.09, g: 0.4, pan: -0.1, t60: 0.24, index: 0.6, attack: 0.007 }],
                layers: [{ v: 'knock', t: 0.09, g: 0.4 }],
                space: { size: 0.55, rt60: 0.35, damping: 6000, predelay: 0.008, mix: 0.18 }
            },
            'space.switch': {
                notes: run('cel', [-12, -9, -5, -2, 0], 0.05, 0.05, 0.5, 0.25, { attack: 0.01 }),
                layers: [{ v: 'air', t: 0, g: 0.8, attack: 0.12, decay: 0.2, band: [1500, 6500] }],
                space: { size: 0.6, rt60: 0.35, damping: 8500, predelay: 0.008, mix: 0.2 }
            },
            'menu.open': {
                notes: run('mini', [-12, -8, -5], 0, 0.035, 1, 0.07, { air: 0, dur: 0.1 }),
                layers: [{ v: 'tick', t: 0, g: 0.2, tone: 2200 }, { v: 'air', t: 0, g: 0.06, decay: 0.08 }],
                space: { size: 0.4, rt60: 0.12, damping: 8000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: run('mini', [-5, -8, -12], 0, 0.03, 1, 0.06, { air: 0, dur: 0.08 }),
                layers: [{ v: 'tick', t: 0, g: 0.2, tone: 2200 }, { v: 'air', t: 0, g: 0.06, decay: 0.07 }],
                space: { size: 0.4, rt60: 0.05, damping: 8000, predelay: 0.002, mix: 0.04 }
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
