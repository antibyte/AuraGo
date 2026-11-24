// Voxel World: wooden taps, stone ticks and dull earth knocks. Key C4 (60), C major pentatonic in plain steps (0, 2, 4, 7, 9).
// Dry, short and percussive; the woodblock, the stone click and the earth knock are the whole instrument set.
// fx: windows break into blocks (fracture) and blocks land (impact), each in wood, stone and earth.
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'voxel_world',
        key: 60,
        space: { size: 0.4, rt60: 0.12, damping: 6000, predelay: 0.002, mix: 0.1 },
        voices: {
            block: { model: 'bar', kind: 'woodblock', t60: 0.14, damp: 0.7, hardness: 0.4, attack: 0.002, body: 0, mallet: 0.35 },
            stone: { model: 'click', material: 'stone', tone: 1500, ring: 0.012 },
            earth: { model: 'click', material: 'felt', tone: 480, ring: 0.045, noise: 0.6, q: 2, length: 0.008 },
            grit: { model: 'noise', color: 'pink', band: [400, 2200], attack: 0.004, decay: 0.05, q: 0.6 },
            wood: { model: 'debris', grain: 'wood' },
            rock: { model: 'debris', grain: 'stone' },
            dirt: { model: 'debris', grain: 'earth' }
        },
        events: {
            'session.start': {
                notes: [{ v: 'block', n: 0, t: 0, g: 0.85, pan: -0.25, t60: 0.2 }, { v: 'block', n: 2, t: 0.17, g: 0.9, pan: -0.1, t60: 0.22 },
                    { v: 'block', n: 4, t: 0.34, g: 0.95, pan: 0.1, t60: 0.24 }, { v: 'block', n: 7, t: 0.52, g: 0.95, pan: 0.2, t60: 0.28 },
                    { v: 'block', n: 9, t: 0.74, g: 1, pan: 0, t60: 0.5, hardness: 0.3 }],
                layers: [{ v: 'stone', t: 0, g: 0.5 }, { v: 'earth', t: 0.52, g: 0.4, tone: 400 }, { v: 'grit', t: 0.74, g: 0.25, decay: 0.12 }],
                space: { size: 0.8, rt60: 0.55, damping: 5500, predelay: 0.01, mix: 0.22 }
            },
            'notify.message': {
                notes: [{ v: 'block', n: 0, t: 0, g: 0.85, pan: -0.2, t60: 0.2 }, { v: 'block', n: 2, t: 0.12, g: 0.9, pan: 0.1, t60: 0.22 },
                    { v: 'block', n: 4, t: 0.24, g: 1, pan: 0, t60: 0.3 }],
                layers: [{ v: 'stone', t: 0, g: 0.4 }],
                space: { size: 0.6, rt60: 0.3, damping: 5500, predelay: 0.006, mix: 0.16 }
            },
            'notify.info': {
                notes: [{ v: 'block', n: 0, t: 0, g: 0.85, pan: -0.15, t60: 0.18 }, { v: 'block', n: 4, t: 0.11, g: 1, pan: 0.15, t60: 0.26 }],
                layers: [{ v: 'stone', t: 0, g: 0.4 }],
                space: { size: 0.55, rt60: 0.25, damping: 5500, predelay: 0.005, mix: 0.14 }
            },
            'notify.success': {
                notes: [{ v: 'block', n: -5, t: 0, g: 0.85, pan: -0.2, t60: 0.2 }, { v: 'block', n: -3, t: 0.1, g: 0.9, pan: 0.1, t60: 0.2 },
                    { v: 'block', n: 0, t: 0.2, g: 1, pan: 0, t60: 0.32 }],
                layers: [{ v: 'stone', t: 0.2, g: 0.4 }],
                space: { size: 0.55, rt60: 0.28, damping: 5500, predelay: 0.005, mix: 0.15 }
            },
            'notify.warning': {
                notes: [{ v: 'block', n: 4, t: 0, g: 1, pan: -0.1, t60: 0.16, hardness: 0.5 }, { v: 'block', n: 4, t: 0.18, g: 0.9, pan: 0.1, t60: 0.2, hardness: 0.5 }],
                layers: [{ v: 'stone', t: 0, g: 0.4, tone: 1300 }, { v: 'stone', t: 0.18, g: 0.35, tone: 1300 }],
                space: { size: 0.5, rt60: 0.22, damping: 5000, predelay: 0.005, mix: 0.14 }
            },
            'notify.error': {
                notes: [{ v: 'block', n: 4, t: 0, g: 0.9, pan: -0.1, t60: 0.2, hardness: 0.25, attack: 0.006 },
                    { v: 'block', n: -2, t: 0.15, g: 1, pan: 0.1, t60: 0.28, hardness: 0.25, attack: 0.008 }],
                layers: [{ v: 'earth', t: 0, g: 0.6 }, { v: 'earth', t: 0.15, g: 0.7, tone: 400 }, { v: 'grit', t: 0.15, g: 0.35, decay: 0.1 }],
                space: { size: 0.5, rt60: 0.25, damping: 4500, predelay: 0.004, mix: 0.14 }
            },
            'dialog.open': {
                notes: [{ v: 'block', n: -3, t: 0, g: 0.85, pan: -0.15, t60: 0.14 }, { v: 'block', n: 0, t: 0.08, g: 1, pan: 0.15, t60: 0.2 }],
                layers: [{ v: 'stone', t: 0, g: 0.35 }],
                space: { size: 0.5, rt60: 0.22, damping: 5500, predelay: 0.004, mix: 0.12 }
            },
            'dialog.confirm': {
                notes: [{ v: 'block', n: 0, t: 0, g: 0.85, pan: -0.15, t60: 0.14 }, { v: 'block', n: 4, t: 0.08, g: 1, pan: 0.15, t60: 0.22 }],
                layers: [{ v: 'stone', t: 0.08, g: 0.35 }],
                space: { size: 0.5, rt60: 0.22, damping: 5500, predelay: 0.004, mix: 0.12 }
            },
            'dialog.cancel': {
                notes: [{ v: 'block', n: 4, t: 0, g: 0.85, pan: 0.15, t60: 0.12 }, { v: 'block', n: 0, t: 0.08, g: 1, pan: -0.15, t60: 0.16 }],
                layers: [{ v: 'earth', t: 0.08, g: 0.4, tone: 520 }],
                space: { size: 0.45, rt60: 0.18, damping: 4800, predelay: 0.004, mix: 0.1 }
            },
            'window.open': {
                notes: [{ v: 'block', n: 0, t: 0, g: 0.9, pan: -0.15, t60: 0.12 }, { v: 'block', n: 4, t: 0.07, g: 1, pan: 0.2, t60: 0.16 }],
                layers: [{ v: 'stone', t: 0, g: 0.4 }],
                space: { size: 0.45, rt60: 0.2, damping: 5500, predelay: 0.004, mix: 0.12 }
            },
            'window.close': {
                notes: [{ v: 'block', n: 4, t: 0, g: 0.9, pan: 0.2, t60: 0.1 }, { v: 'block', n: 0, t: 0.07, g: 1, pan: -0.15, t60: 0.14 }],
                layers: [{ v: 'stone', t: 0.07, g: 0.4, tone: 1300 }],
                space: { size: 0.45, rt60: 0.18, damping: 5000, predelay: 0.004, mix: 0.1 }
            },
            'window.maximize': {
                notes: [{ v: 'block', n: -3, t: 0, g: 0.9, pan: -0.2, t60: 0.14 }, { v: 'block', n: 2, t: 0.09, g: 1, pan: 0.2, t60: 0.22 }],
                layers: [{ v: 'stone', t: 0, g: 0.4 }, { v: 'earth', t: 0, g: 0.35 }, { v: 'stone', t: 0.09, g: 0.35, tone: 1900 }],
                space: { size: 0.7, rt60: 0.35, damping: 5500, predelay: 0.008, mix: 0.18 }
            },
            'window.minimize': {
                notes: [{ v: 'block', n: 4, t: 0, g: 0.9, pan: 0.15, t60: 0.09 }, { v: 'block', n: 0, t: 0.055, g: 1, pan: -0.15, t60: 0.12 }],
                space: { size: 0.4, rt60: 0.14, damping: 5500, predelay: 0.003, mix: 0.1 }
            },
            'window.restore': {
                notes: [{ v: 'block', n: 0, t: 0, g: 0.9, pan: -0.15, t60: 0.09 }, { v: 'block', n: 4, t: 0.055, g: 1, pan: 0.15, t60: 0.12 }],
                space: { size: 0.4, rt60: 0.14, damping: 5500, predelay: 0.003, mix: 0.1 }
            },
            'window.snap': {
                notes: [{ v: 'block', n: 9, t: 0, g: 0.5, t60: 0.03, dur: 0.04, hardness: 0.3, mallet: 0.2 }],
                layers: [{ v: 'stone', t: 0, g: 1, tone: 1700, ring: 0.02, noise: 0.5 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'block', n: 0, t: 0, g: 0.8, pan: 0, t60: 0.1, hardness: 0.25, attack: 0.004 }],
                layers: [{ v: 'earth', t: 0, g: 0.9, tone: 450 }, { v: 'grit', t: 0, g: 0.3, decay: 0.05 }],
                space: { size: 0.4, rt60: 0.14, damping: 4500, predelay: 0.003, mix: 0.1 }
            },
            'file.drop': {
                notes: [{ v: 'block', n: -5, t: 0.01, g: 0.7, pan: 0, t60: 0.14, hardness: 0.2, attack: 0.004 }],
                layers: [{ v: 'earth', t: 0, g: 1, tone: 480, ring: 0.06 }, { v: 'wood', t: 0.02, g: 0.32, count: 4, spread: 0.06, decay: 0.1, size: 0.8, tone: 1.3 }],
                space: { size: 0.45, rt60: 0.2, damping: 4800, predelay: 0.004, mix: 0.12 }
            },
            'file.trash': {
                notes: [{ v: 'block', n: 7, t: 0, g: 0.8, pan: 0.15, t60: 0.08 }, { v: 'block', n: 4, t: 0.05, g: 0.9, pan: 0, t60: 0.1 },
                    { v: 'block', n: 0, t: 0.1, g: 1, pan: -0.15, t60: 0.14 }],
                layers: [{ v: 'dirt', t: 0.1, g: 0.3, count: 5, spread: 0.08, decay: 0.12, tone: 1.5 }],
                space: { size: 0.45, rt60: 0.18, damping: 5000, predelay: 0.004, mix: 0.12 }
            },
            'file.delete': {
                notes: [{ v: 'block', n: 4, t: 0, g: 0.8, pan: 0.1, t60: 0.1 }, { v: 'block', n: 0, t: 0.08, g: 1, pan: -0.1, t60: 0.2, hardness: 0.25, attack: 0.004 }],
                layers: [{ v: 'earth', t: 0.08, g: 0.8, tone: 400, ring: 0.07 }, { v: 'rock', t: 0.1, g: 0.35, count: 6, spread: 0.1, decay: 0.15 }],
                space: { size: 0.55, rt60: 0.28, damping: 4800, predelay: 0.006, mix: 0.15 }
            },
            'space.switch': {
                notes: [{ v: 'block', n: 2, t: 0.08, g: 0.5, pan: 0, t60: 0.14, hardness: 0.25 }],
                layers: [{ v: 'grit', t: 0, g: 0.9, attack: 0.1, decay: 0.2, band: [400, 2200] }, { v: 'dirt', t: 0.02, g: 0.3, count: 8, spread: 0.2, decay: 0.2 }],
                space: { size: 0.5, rt60: 0.25, damping: 5000, predelay: 0.005, mix: 0.14 }
            },
            'menu.open': {
                notes: [{ v: 'block', n: 0, t: 0, g: 0.9, pan: -0.1, t60: 0.05, dur: 0.07 }, { v: 'block', n: 4, t: 0.03, g: 1, pan: 0.1, t60: 0.05, dur: 0.08 }],
                space: { size: 0.4, rt60: 0.1, damping: 6000, predelay: 0.002, mix: 0.08 }
            },
            'menu.close': {
                notes: [{ v: 'block', n: 4, t: 0, g: 0.9, pan: 0.1, t60: 0.05, dur: 0.07 }, { v: 'block', n: 0, t: 0.03, g: 1, pan: -0.1, t60: 0.05, dur: 0.07 }],
                space: { size: 0.4, rt60: 0.07, damping: 6000, predelay: 0.002, mix: 0.06 }
            },
            'ui.select': {
                layers: [{ v: 'stone', t: 0, g: 1, tone: 1700, ring: 0.01 }, { v: 'grit', t: 0, g: 0.12, band: [400, 2000], decay: 0.03 }],
                space: { size: 0.4, rt60: 0.08, damping: 6000, predelay: 0.002, mix: 0.07 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'stone', t: 0, g: 0.9, tone: 1400, ring: 0.01 }, { v: 'stone', t: 0.04, g: 1, tone: 2100, ring: 0.01 }],
                space: null
            },
            'ui.toggle.off': {
                layers: [{ v: 'stone', t: 0, g: 0.9, tone: 2100, ring: 0.01 }, { v: 'stone', t: 0.04, g: 1, tone: 1400, ring: 0.01 }],
                space: null
            },
            'ui.tick': {
                layers: [{ v: 'stone', t: 0, g: 1, tone: 2600, ring: 0.006, length: 0.002 }],
                space: null
            }
        },
        fx: {
            'window.fracture': {
                wood: {
                    layers: [{ v: 'wood', t: 0, g: 0.6, count: 6, spread: 0.08, decay: 0.16, size: 0.9, tone: 0.8 },
                        { v: 'wood', t: 0.03, g: 0.8, count: 18, spread: 0.3, decay: 0.25, size: 0.4, tone: 0.8 },
                        { v: 'stone', t: 0, g: 0.3, tone: 1100, ring: 0.01 }, { v: 'stone', t: 0.05, g: 0.2, tone: 1300, ring: 0.01 }]
                },
                stone: {
                    layers: [{ v: 'rock', t: 0, g: 1, count: 4, spread: 0.05, decay: 0.12, size: 0.9, tone: 0.9 },
                        { v: 'rock', t: 0.03, g: 0.6, count: 14, spread: 0.3, decay: 0.2, size: 0.4 }]
                },
                earth: {
                    layers: [{ v: 'dirt', t: 0, g: 0.5, count: 4, spread: 0.06, decay: 0.15, size: 0.3, tone: 2 },
                        { v: 'dirt', t: 0.03, g: 0.3, count: 16, spread: 0.3, decay: 0.25, size: 0, tone: 2 },
                        { v: 'grit', t: 0.01, g: 0.8, decay: 0.2, band: [600, 3000] }]
                }
            },
            'window.impact': {
                wood: {
                    notes: [{ v: 'block', n: 2, t: 0, g: 1, t60: 0.1, hardness: 0.45, attack: 0.002 }, { v: 'block', n: 0, t: 0.012, g: 0.6, t60: 0.08, hardness: 0.3 }],
                    layers: [{ v: 'wood', t: 0.03, g: 0.25, count: 3, spread: 0.1, decay: 0.1, size: 0.5 }]
                },
                stone: {
                    layers: [{ v: 'stone', t: 0, g: 1, tone: 800, ring: 0.03, length: 0.008 }, { v: 'earth', t: 0, g: 0.4, tone: 400 },
                        { v: 'rock', t: 0.02, g: 0.5, count: 5, spread: 0.1, decay: 0.12, size: 0.6 }]
                },
                earth: {
                    layers: [{ v: 'earth', t: 0, g: 1, tone: 520, ring: 0.02, noise: 1, q: 0.8, length: 0.012 }, { v: 'grit', t: 0, g: 0.6, decay: 0.1, band: [450, 2000] },
                        { v: 'dirt', t: 0.02, g: 0.4, count: 6, spread: 0.12, decay: 0.15, size: 0, tone: 2 }]
                }
            }
        }
    });
})();
