// Workshop: relay clicks, small servo glides, a pneumatic breath and a little metal ping. Key E4 (64), fifths and fourths only (0, 5, 7).
// Clicks and hiss carry the frequent events; the metal ping is the only tonal colour. Tight workshop room.
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'workshop',
        key: 64,
        space: { size: 0.55, rt60: 0.35, damping: 5500, predelay: 0.006, mix: 0.2 },
        voices: {
            ping: { model: 'metal', kind: 'pipe', t60: 0.5, damp: 0.35, beat: 1.5, hardness: 0.55, attack: 0.002, width: 0.5, spread: 2 },
            servo: { model: 'blip', glide: -5, glideTime: 0.05, harm: 0.25, attack: 0.006, decay: 0.12 },
            relay: { model: 'click', material: 'relay', tone: 2600, ring: 0.018 },
            knock: { model: 'click', material: 'felt', tone: 520, ring: 0.03, noise: 0.5, q: 1.2, length: 0.008 },
            hiss: { model: 'noise', color: 'pink', band: [2200, 7000], attack: 0.025, decay: 0.12, q: 0.6 },
            puff: { model: 'noise', color: 'pink', band: [900, 4200], attack: 0.01, decay: 0.08, q: 0.6 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'servo', n: -12, t: 0, g: 0.5, pan: -0.2, glide: 7, glideTime: 0.14, decay: 0.3, attack: 0.02 },
                    { v: 'ping', n: -5, t: 0.22, g: 0.8, pan: -0.2, t60: 0.6 }, { v: 'ping', n: 0, t: 0.42, g: 0.85, pan: 0.1, t60: 0.7 },
                    { v: 'ping', n: 7, t: 0.62, g: 0.9, pan: -0.05, t60: 0.8 }, { v: 'ping', n: 12, t: 0.9, g: 0.9, pan: 0.15, t60: 1.2 }],
                layers: [{ v: 'puff', t: 0, g: 0.5, attack: 0.1, decay: 0.3 }, { v: 'relay', t: 0.2, g: 0.5 }, { v: 'relay', t: 0.4, g: 0.5, tone: 2900 },
                    { v: 'relay', t: 0.6, g: 0.5, tone: 3200 }, { v: 'hiss', t: 1.05, g: 0.35, attack: 0.05, decay: 0.5 }],
                space: { size: 0.9, rt60: 0.9, damping: 5000, predelay: 0.012, mix: 0.3 }
            },
            'notify.message': {
                notes: [{ v: 'ping', n: 0, t: 0.03, g: 0.8, pan: -0.2, t60: 0.4 }, { v: 'ping', n: 5, t: 0.17, g: 0.9, pan: 0.15, t60: 0.45 },
                    { v: 'ping', n: 12, t: 0.31, g: 1, pan: 0, t60: 0.6 }],
                layers: [{ v: 'relay', t: 0, g: 0.6 }, { v: 'relay', t: 0.14, g: 0.5, tone: 2900 }, { v: 'puff', t: 0.3, g: 0.3, decay: 0.15 }],
                space: { size: 0.75, rt60: 0.6, damping: 5000, predelay: 0.01, mix: 0.26 }
            },
            'notify.info': {
                notes: [{ v: 'ping', n: 0, t: 0.03, g: 0.85, pan: -0.15, t60: 0.35 }, { v: 'ping', n: 7, t: 0.15, g: 1, pan: 0.15, t60: 0.5 }],
                layers: [{ v: 'relay', t: 0, g: 0.6 }],
                space: { size: 0.7, rt60: 0.5, damping: 5000, predelay: 0.008, mix: 0.24 }
            },
            'notify.success': {
                notes: [{ v: 'ping', n: -5, t: 0.03, g: 0.8, pan: -0.2, t60: 0.35 }, { v: 'ping', n: 0, t: 0.13, g: 0.9, pan: 0.1, t60: 0.4 },
                    { v: 'ping', n: 12, t: 0.23, g: 1, pan: 0, t60: 0.55 }],
                layers: [{ v: 'relay', t: 0, g: 0.6 }, { v: 'relay', t: 0.22, g: 0.5, tone: 3100 }],
                space: { size: 0.7, rt60: 0.5, damping: 5000, predelay: 0.008, mix: 0.24 }
            },
            'notify.warning': {
                notes: [{ v: 'ping', n: 5, t: 0.03, g: 1, pan: -0.1, t60: 0.28, attack: 0.004 }, { v: 'ping', n: 5, t: 0.22, g: 0.85, pan: 0.1, t60: 0.34, attack: 0.004 }],
                layers: [{ v: 'relay', t: 0, g: 0.6 }, { v: 'relay', t: 0.19, g: 0.5 }, { v: 'puff', t: 0.2, g: 0.25, decay: 0.12 }],
                space: { size: 0.6, rt60: 0.45, damping: 4800, predelay: 0.008, mix: 0.22 }
            },
            'notify.error': {
                notes: [{ v: 'ping', n: 0, t: 0.02, g: 0.9, pan: -0.1, t60: 0.3, damp: 1, hardness: 0.3, attack: 0.008 },
                    { v: 'ping', n: -6, t: 0.19, g: 0.9, pan: 0.1, t60: 0.34, damp: 1, hardness: 0.3, attack: 0.01 }],
                layers: [{ v: 'knock', t: 0, g: 0.8 }, { v: 'knock', t: 0.17, g: 0.7, tone: 420 }, { v: 'puff', t: 0.17, g: 0.35, decay: 0.2 }],
                space: { size: 0.55, rt60: 0.4, damping: 4000, predelay: 0.006, mix: 0.2 }
            },
            'dialog.open': {
                notes: [{ v: 'ping', n: -5, t: 0.02, g: 0.8, pan: -0.15, t60: 0.2 }, { v: 'ping', n: 0, t: 0.1, g: 1, pan: 0.15, t60: 0.26 }],
                layers: [{ v: 'relay', t: 0, g: 0.6 }],
                space: { size: 0.6, rt60: 0.4, damping: 5000, predelay: 0.006, mix: 0.2 }
            },
            'dialog.confirm': {
                notes: [{ v: 'ping', n: 0, t: 0.02, g: 0.8, pan: -0.15, t60: 0.2 }, { v: 'ping', n: 7, t: 0.1, g: 1, pan: 0.15, t60: 0.3 }],
                layers: [{ v: 'relay', t: 0, g: 0.6 }],
                space: { size: 0.6, rt60: 0.42, damping: 5000, predelay: 0.006, mix: 0.2 }
            },
            'dialog.cancel': {
                notes: [{ v: 'ping', n: 7, t: 0.02, g: 0.8, pan: 0.15, t60: 0.15 }, { v: 'ping', n: 0, t: 0.1, g: 1, pan: -0.15, t60: 0.2 }],
                layers: [{ v: 'relay', t: 0, g: 0.5, tone: 2300 }],
                space: { size: 0.5, rt60: 0.3, damping: 4500, predelay: 0.006, mix: 0.16 }
            },
            'window.open': {
                notes: [{ v: 'servo', n: 0, t: 0, g: 0.55, pan: -0.1, glide: -7, glideTime: 0.04, decay: 0.09, attack: 0.008 },
                    { v: 'ping', n: 5, t: 0.06, g: 0.9, pan: 0.15, t60: 0.22 }],
                layers: [{ v: 'relay', t: 0, g: 0.6 }, { v: 'puff', t: 0.02, g: 0.2, decay: 0.07 }],
                space: { size: 0.55, rt60: 0.35, damping: 5500, predelay: 0.006, mix: 0.18 }
            },
            'window.close': {
                notes: [{ v: 'ping', n: 5, t: 0, g: 0.9, pan: 0.15, t60: 0.16 },
                    { v: 'servo', n: 0, t: 0.055, g: 0.55, pan: -0.1, glide: 7, glideTime: 0.04, decay: 0.09, attack: 0.008 }],
                layers: [{ v: 'relay', t: 0.06, g: 0.6, tone: 2300 }, { v: 'puff', t: 0.03, g: 0.2, decay: 0.07 }],
                space: { size: 0.5, rt60: 0.3, damping: 5000, predelay: 0.006, mix: 0.16 }
            },
            'window.maximize': {
                notes: [{ v: 'servo', n: 0, t: 0, g: 0.55, pan: -0.15, glide: -7, glideTime: 0.06, decay: 0.12, attack: 0.008 },
                    { v: 'ping', n: 7, t: 0.08, g: 1, pan: 0.2, t60: 0.3 }],
                layers: [{ v: 'relay', t: 0, g: 0.6 }, { v: 'hiss', t: 0.02, g: 0.3, decay: 0.12 }],
                space: { size: 0.75, rt60: 0.45, damping: 5500, predelay: 0.01, mix: 0.24 }
            },
            'window.minimize': {
                notes: [{ v: 'ping', n: 5, t: 0.01, g: 0.9, pan: 0.15, t60: 0.12 },
                    { v: 'servo', n: 0, t: 0.05, g: 0.5, pan: -0.1, glide: 7, glideTime: 0.03, decay: 0.07, attack: 0.006 }],
                layers: [{ v: 'relay', t: 0, g: 0.5, tone: 2300 }],
                space: { size: 0.5, rt60: 0.25, damping: 5000, predelay: 0.005, mix: 0.14 }
            },
            'window.restore': {
                notes: [{ v: 'servo', n: 0, t: 0, g: 0.5, pan: -0.1, glide: -7, glideTime: 0.03, decay: 0.07, attack: 0.006 },
                    { v: 'ping', n: 5, t: 0.045, g: 0.9, pan: 0.15, t60: 0.14 }],
                layers: [{ v: 'relay', t: 0, g: 0.5, tone: 2900 }],
                space: { size: 0.5, rt60: 0.25, damping: 5000, predelay: 0.005, mix: 0.14 }
            },
            'window.snap': {
                layers: [{ v: 'relay', t: 0, g: 1, tone: 3000, double: 0.01 }],
                space: null
            },
            'window.deny': {
                layers: [{ v: 'knock', t: 0, g: 1, tone: 520, ring: 0.05, q: 2.5 }, { v: 'puff', t: 0, g: 0.3, band: [300, 1200], decay: 0.06 }],
                notes: [{ v: 'servo', n: -5, t: 0, g: 0.5, pan: 0, glide: 2, glideTime: 0.05, decay: 0.1, attack: 0.008 }],
                space: { size: 0.45, rt60: 0.22, damping: 4500, predelay: 0.004, mix: 0.12 }
            },
            'file.drop': {
                notes: [{ v: 'ping', n: -5, t: 0.02, g: 0.7, pan: 0, t60: 0.2, damp: 1.4, hardness: 0.1, attack: 0.004 }],
                layers: [{ v: 'knock', t: 0, g: 0.9, tone: 700, q: 2.5 }, { v: 'puff', t: 0.01, g: 0.3, band: [500, 2500], decay: 0.08 }],
                space: { size: 0.55, rt60: 0.32, damping: 4500, predelay: 0.005, mix: 0.16 }
            },
            'file.trash': {
                notes: [{ v: 'servo', n: 7, t: 0, g: 0.6, pan: 0.1, glide: 8, glideTime: 0.05, decay: 0.12, attack: 0.006 },
                    { v: 'ping', n: -5, t: 0.09, g: 0.8, pan: -0.1, t60: 0.15, damp: 1.2 }],
                layers: [{ v: 'hiss', t: 0, g: 0.4, decay: 0.12, sweep: -1.5 }, { v: 'relay', t: 0.09, g: 0.6, tone: 2000 }],
                space: { size: 0.55, rt60: 0.32, damping: 4500, predelay: 0.005, mix: 0.16 }
            },
            'file.delete': {
                notes: [{ v: 'ping', n: -5, t: 0.1, g: 0.9, pan: -0.1, t60: 0.3, damp: 1, hardness: 0.2, attack: 0.006 }],
                layers: [{ v: 'hiss', t: 0, g: 0.5, decay: 0.16, sweep: -2 }, { v: 'relay', t: 0.02, g: 0.6, tone: 2100 }, { v: 'knock', t: 0.1, g: 0.7, tone: 600, q: 2.5 }],
                space: { size: 0.7, rt60: 0.42, damping: 4500, predelay: 0.008, mix: 0.2 }
            },
            'space.switch': {
                notes: [{ v: 'servo', n: 0, t: 0.03, g: 0.4, pan: 0, glide: -7, glideTime: 0.12, decay: 0.25, attack: 0.05 }],
                layers: [{ v: 'hiss', t: 0, g: 0.9, attack: 0.12, decay: 0.2, sweep: 1 }, { v: 'relay', t: 0.3, g: 0.5, tone: 2400 }],
                space: { size: 0.6, rt60: 0.4, damping: 5000, predelay: 0.008, mix: 0.2 }
            },
            'menu.open': {
                layers: [{ v: 'relay', t: 0, g: 0.8, tone: 2200, length: 0.002 }, { v: 'relay', t: 0.03, g: 1, tone: 3000, length: 0.002 }],
                space: { size: 0.4, rt60: 0.1, damping: 6500, predelay: 0.002, mix: 0.08 }
            },
            'menu.close': {
                layers: [{ v: 'relay', t: 0, g: 0.8, tone: 3000, length: 0.002 }, { v: 'relay', t: 0.03, g: 1, tone: 2200, length: 0.002 }],
                space: { size: 0.4, rt60: 0.08, damping: 6500, predelay: 0.002, mix: 0.06 }
            },
            'ui.select': {
                layers: [{ v: 'relay', t: 0, g: 1, tone: 2400, ring: 0.02 }, { v: 'puff', t: 0, g: 0.15, decay: 0.04 }],
                space: { size: 0.4, rt60: 0.1, damping: 6500, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'relay', t: 0, g: 0.9, tone: 2200, ring: 0.015 }, { v: 'relay', t: 0.04, g: 1, tone: 3200, ring: 0.015 }],
                space: null
            },
            'ui.toggle.off': {
                layers: [{ v: 'relay', t: 0, g: 0.9, tone: 3200, ring: 0.015 }, { v: 'relay', t: 0.04, g: 1, tone: 2200, ring: 0.015 }],
                space: null
            },
            'ui.tick': {
                layers: [{ v: 'relay', t: 0, g: 1, tone: 3600, ring: 0.01, length: 0.0015 }],
                space: null
            }
        },
        fx: {}
    });
})();
