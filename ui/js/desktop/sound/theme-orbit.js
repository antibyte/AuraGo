// Orbit: sonar pings with a soft echo, drifting whooshes, airlock hiss and a slow pad. Key E4 (64), E Lydian (0, 2, 4, 6, 7, 9, 11).
// Deep space: the long tail belongs to the notices and the start sound; frequent events stay short with at most one faint echo.
// fx: 'orbit.ping' is a sonar ping in three sizes (larger = lower and longer), each with its own echo.
(function () {
    'use strict';
    self.DesktopSoundEngine.defineTheme({
        id: 'orbit',
        key: 64,
        space: { size: 1.4, rt60: 1.5, damping: 5000, predelay: 0.02, mix: 0.26 },
        voices: {
            ping: { model: 'blip', harm: 0.5, attack: 0.006, decay: 0.28, air: 0.1, glide: 2, glideTime: 0.025 },
            ring: { model: 'metal', kind: 'ping', t60: 0.3, damp: 0.8, beat: 0.8, hardness: 0.15, attack: 0.004, width: 0.6, spread: 2, air: 0.06 },
            deep: { model: 'blip', harm: 0.4, attack: 0.012, decay: 0.3, glide: 2, glideTime: 0.08, air: 0.08 },
            pad: { model: 'pad', partials: 4, detune: 9, attack: 0.25, hold: 0.1, release: 0.9, cutoff: 1800, width: 0.8, shimmer: 0.4 },
            whoosh: { model: 'noise', color: 'pink', band: [300, 3200], sweep: 1.2, q: 0.6, attack: 0.12, decay: 0.2, width: 1 },
            soft: { model: 'bell', kind: 'tine', t60: 0.7, index: 0.5, attack: 0.008, spread: 0.3 },
            cel: { model: 'bell', kind: 'celesta', t60: 0.6, index: 0.5, attack: 0.008, spread: 0.3 },
            mellow: { model: 'analog', wave: 'saw', unison: 3, detune: 12, drift: 4, sub: 0, cutoff: 0.7, env: 2.5, envDecay: 0.25, q: 1, drive: 1, attack: 0.015, decay: 0.4, width: 0.7 },
            swell: { model: 'analog', wave: 'tri', unison: 3, detune: 9, drift: 4, sub: 0, cutoff: 0.8, env: 1.5, envDecay: 0.3, q: 0.8, drive: 0.4, attack: 0.045, decay: 0.3, release: 0.12, width: 0.8 },
            sonar: { model: 'blip', harm: 0.35, attack: 0.008, decay: 0.4, air: 0.1, glide: 3, glideTime: 0.045 },
            breath: { model: 'noise', color: 'pink', band: [250, 2600], sweep: 1, q: 0.5, attack: 0.16, decay: 0.12, width: 1 },
            knock: { model: 'bar', kind: 'woodblock', t60: 0.12, hardness: 0.1, mallet: 0.2, attack: 0.002 },
            hiss: { model: 'noise', color: 'pink', band: [2500, 8000], q: 0.5, attack: 0.02, decay: 0.12, width: 1 },
            dock: { model: 'click', material: 'plastic', tone: 2800, ring: 0.02, noise: 0.75, q: 1.5 }
        },
        events: {
            'session.start': {
                notes: [{ v: 'pad', n: -5, t: 0, g: 0.4, pan: 0, attack: 0.4, release: 1.0 }, { v: 'pad', n: 0, t: 0.05, g: 0.35, pan: 0, attack: 0.45, release: 1.0 },
                    { v: 'swell', n: 4, t: 0.25, g: 0.9, pan: -0.25, unison: 5, detune: 14 }, { v: 'swell', n: 7, t: 0.47, g: 0.9, pan: 0.2, unison: 5, detune: 14 },
                    { v: 'swell', n: 11, t: 0.69, g: 0.95, pan: -0.1, unison: 5, detune: 14 }, { v: 'swell', n: 16, t: 0.91, g: 1, pan: 0.1, unison: 5, detune: 14, decay: 0.5 }, { v: 'cel', n: 16, t: 0.93, g: 0.4, pan: 0.1, t60: 0.6 },
                    { v: 'sonar', n: 23, t: 1.2, g: 0.8, pan: 0.15, decay: 0.4 }],
                layers: [{ v: 'breath', t: 0, g: 0.9, attack: 0.3, decay: 0.3 }, { v: 'breath', t: 0.8, g: 0.5, attack: 0.25, decay: 0.4, band: [600, 4500], sweep: -1 }],
                chorus: { rate: 0.4, depth: 0.003, delay: 0.015, mix: 0.35 },
                echo: { time: 0.3, feedback: 0.2, tone: 3000, mix: 0.36, spread: 0.7 },
                space: { size: 1.8, rt60: 0.6, damping: 4500, predelay: 0.03, mix: 0.45 }
            },
            'notify.message': {
                notes: [{ v: 'pad', n: 4, t: 0, g: 0.6, attack: 0.15, hold: 0.05, release: 0.5 },
                    { v: 'swell', n: 4, t: 0.06, g: 0.9, pan: -0.2 }, { v: 'swell', n: 6, t: 0.24, g: 0.9, pan: 0.15 },
                    { v: 'swell', n: 11, t: 0.42, g: 1, pan: 0, decay: 0.35 },
                    { v: 'sonar', n: 18, t: 0.56, g: 0.8, pan: 0.2, decay: 0.22 }],
                layers: [{ v: 'breath', t: 0, g: 0.5 }],
                echo: { time: 0.15, feedback: 0.16, tone: 3000, mix: 0.26, spread: 0.6 },
                space: { size: 1.4, rt60: 0.22, damping: 4500, predelay: 0.02, mix: 0.32 }
            },
            'notify.info': {
                notes: [{ v: 'pad', n: 0, t: 0, g: 0.6, attack: 0.12, hold: 0, release: 0.4 },
                    { v: 'swell', n: 0, t: 0.08, g: 0.9, pan: -0.15 }, { v: 'swell', n: 7, t: 0.27, g: 1, pan: 0.15, decay: 0.3 }],
                layers: [{ v: 'breath', t: 0, g: 0.35, attack: 0.1 }],
                echo: { time: 0.2, feedback: 0.1, tone: 3000, mix: 0.16, spread: 0.5 },
                space: { size: 1.3, rt60: 0.2, damping: 4500, predelay: 0.018, mix: 0.3 }
            },
            'notify.success': {
                notes: [{ v: 'pad', n: 0, t: 0, g: 0.6, attack: 0.12, hold: 0, release: 0.35 },
                    { v: 'swell', n: 0, t: 0.08, g: 0.85, pan: -0.2 }, { v: 'swell', n: 7, t: 0.28, g: 0.9, pan: 0.1 },
                    { v: 'swell', n: 12, t: 0.46, g: 1, pan: 0, decay: 0.3 }],
                layers: [{ v: 'breath', t: 0, g: 0.3, attack: 0.1 }],
                echo: { time: 0.2, feedback: 0.08, tone: 3000, mix: 0.14, spread: 0.5 },
                space: { size: 1.3, rt60: 0.1, damping: 4500, predelay: 0.018, mix: 0.3 }
            },
            'notify.warning': {
                notes: [{ v: 'pad', n: -3, t: 0, g: 0.5, attack: 0.06, hold: 0, release: 0.4 },
                    { v: 'mellow', n: 9, t: 0, g: 1, pan: -0.1, decay: 0.3 }, { v: 'soft', n: 9, t: 0, g: 0.6, pan: -0.1 },
                    { v: 'mellow', n: 9, t: 0.24, g: 0.9, pan: 0.1, decay: 0.4 }, { v: 'soft', n: 9, t: 0.24, g: 0.55, pan: 0.1, t60: 0.7 }],
                layers: [{ v: 'dock', t: 0, g: 0.1, tone: 1500, material: 'felt' }],
                echo: { time: 0.24, feedback: 0.12, tone: 2800, mix: 0.16, spread: 0.4 },
                space: { size: 1.1, rt60: 0.24, damping: 4000, predelay: 0.012, mix: 0.28 }
            },
            'notify.error': {
                notes: [{ v: 'pad', n: -6, t: 0, g: 0.5, attack: 0.06, hold: 0, release: 0.4 },
                    { v: 'mellow', n: 6, t: 0, g: 1, pan: -0.1, decay: 0.3 }, { v: 'soft', n: 6, t: 0, g: 0.55, pan: -0.1 },
                    { v: 'mellow', n: 0, t: 0.22, g: 1, pan: 0.1, decay: 0.35, glide: -1 }, { v: 'soft', n: 0, t: 0.22, g: 0.55, pan: 0.1, t60: 0.45 }],
                layers: [{ v: 'dock', t: 0.22, g: 0.15, tone: 900, material: 'felt' }],
                echo: { time: 0.26, feedback: 0.12, tone: 2500, mix: 0.16, spread: 0.4 },
                space: { size: 1.1, rt60: 0.16, damping: 3800, predelay: 0.01, mix: 0.28 }
            },
            'dialog.open': {
                notes: [{ v: 'pad', n: 0, t: 0, g: 0.4, attack: 0.05, hold: 0, release: 0.3 },
                    { v: 'soft', n: 0, t: 0, g: 0.85, pan: -0.15, t60: 0.3 }, { v: 'mellow', n: 0, t: 0, g: 0.4, pan: -0.15, decay: 0.15 },
                    { v: 'cel', n: 7, t: 0.09, g: 1, pan: 0.15, t60: 0.35 }, { v: 'mellow', n: 7, t: 0.09, g: 0.4, pan: 0.15, decay: 0.2 }],
                echo: { time: 0.12, feedback: 0.1, tone: 3000, mix: 0.12, spread: 0.4 },
                space: { size: 1.1, rt60: 0.16, damping: 4500, predelay: 0.01, mix: 0.26 }
            },
            'dialog.confirm': {
                notes: [{ v: 'pad', n: 4, t: 0, g: 0.3, attack: 0.05, hold: 0, release: 0.3 },
                    { v: 'ping', n: 4, t: 0, g: 0.85, pan: -0.15, decay: 0.12 }, { v: 'ring', n: 4, t: 0, g: 0.34, pan: -0.15, t60: 0.13 }, { v: 'ping', n: 11, t: 0.08, g: 1, pan: 0.15, decay: 0.2 }, { v: 'ring', n: 11, t: 0.08, g: 0.40, pan: 0.15, t60: 0.22 }],
                layers: [{ v: 'hiss', t: 0, g: 0.3, attack: 0.04, decay: 0.15 }],
                echo: { time: 0.13, feedback: 0.14, tone: 3500, mix: 0.18, spread: 0.4 },
                space: { size: 0.9, rt60: 0.14, damping: 5000, predelay: 0.01, mix: 0.2 }
            },
            'dialog.cancel': {
                notes: [{ v: 'pad', n: -1, t: 0, g: 0.25, attack: 0.05, hold: 0, release: 0.25 },
                    { v: 'ping', n: 11, t: 0, g: 0.85, pan: 0.15, decay: 0.1 }, { v: 'ring', n: 11, t: 0, g: 0.34, pan: 0.15, t60: 0.11 }, { v: 'ping', n: 4, t: 0.08, g: 1, pan: -0.15, decay: 0.14 }, { v: 'ring', n: 4, t: 0.08, g: 0.40, pan: -0.15, t60: 0.15 }],
                echo: { time: 0.1, feedback: 0.10, tone: 3000, mix: 0.12, spread: 0.4 },
                space: { size: 0.8, rt60: 0.21, damping: 4500, predelay: 0.008, mix: 0.16 }
            },
            'window.open': {
                notes: [{ v: 'ping', n: 2, t: 0, g: 0.85, pan: -0.15, decay: 0.1 }, { v: 'ring', n: 2, t: 0, g: 0.34, pan: -0.15, t60: 0.11 }, { v: 'ping', n: 9, t: 0.07, g: 1, pan: 0.2, decay: 0.14 }, { v: 'ring', n: 9, t: 0.07, g: 0.40, pan: 0.2, t60: 0.15 }],
                layers: [{ v: 'whoosh', t: 0, g: 0.3, attack: 0.05, decay: 0.08, band: [500, 3500] }],
                echo: { time: 0.11, feedback: 0.10, tone: 3500, mix: 0.15, spread: 0.4 },
                space: { size: 0.8, rt60: 0.21, damping: 5000, predelay: 0.008, mix: 0.18 }
            },
            'window.close': {
                notes: [{ v: 'ping', n: 9, t: 0, g: 0.85, pan: 0.2, decay: 0.08 }, { v: 'ring', n: 9, t: 0, g: 0.34, pan: 0.2, t60: 0.09 }, { v: 'ping', n: 2, t: 0.07, g: 1, pan: -0.15, decay: 0.12 }, { v: 'ring', n: 2, t: 0.07, g: 0.40, pan: -0.15, t60: 0.13 }],
                layers: [{ v: 'whoosh', t: 0, g: 0.3, attack: 0.04, decay: 0.08, band: [500, 3500], sweep: -1.2 }],
                echo: { time: 0.1, feedback: 0.08, tone: 3000, mix: 0.12, spread: 0.4 },
                space: { size: 0.7, rt60: 0.17, damping: 4500, predelay: 0.006, mix: 0.15 }
            },
            'window.maximize': {
                notes: [{ v: 'ping', n: 0, t: 0, g: 0.85, pan: -0.2, decay: 0.14 }, { v: 'ring', n: 0, t: 0, g: 0.34, pan: -0.2, t60: 0.14 }, { v: 'ping', n: 11, t: 0.09, g: 1, pan: 0.2, decay: 0.15 }, { v: 'ring', n: 11, t: 0.09, g: 0.40, pan: 0.2, t60: 0.14 }],
                layers: [{ v: 'whoosh', t: 0, g: 0.4, attack: 0.05, decay: 0.06, band: [400, 4000] }],
                echo: { time: 0.1, feedback: 0.03, tone: 3500, mix: 0.12, spread: 0.5 },
                space: { size: 0.8, rt60: 0.10, damping: 5000, predelay: 0.012, mix: 0.24 }
            },
            'window.minimize': {
                notes: [{ v: 'ping', n: 9, t: 0, g: 0.85, pan: 0.15, decay: 0.07 }, { v: 'ring', n: 9, t: 0, g: 0.34, pan: 0.15, t60: 0.08 }, { v: 'ping', n: 2, t: 0.055, g: 1, pan: -0.15, decay: 0.1 }, { v: 'ring', n: 2, t: 0.055, g: 0.40, pan: -0.15, t60: 0.11 }],
                echo: { time: 0.08, feedback: 0.07, tone: 3000, mix: 0.1, spread: 0.3 },
                space: { size: 0.6, rt60: 0.14, damping: 4500, predelay: 0.005, mix: 0.12 }
            },
            'window.restore': {
                notes: [{ v: 'ping', n: 2, t: 0, g: 0.85, pan: -0.15, decay: 0.07 }, { v: 'ring', n: 2, t: 0, g: 0.34, pan: -0.15, t60: 0.08 }, { v: 'ping', n: 9, t: 0.055, g: 1, pan: 0.15, decay: 0.1 }, { v: 'ring', n: 9, t: 0.055, g: 0.40, pan: 0.15, t60: 0.11 }],
                echo: { time: 0.08, feedback: 0.07, tone: 3000, mix: 0.1, spread: 0.3 },
                space: { size: 0.6, rt60: 0.14, damping: 4500, predelay: 0.005, mix: 0.12 }
            },
            'window.snap': {
                layers: [{ v: 'dock', t: 0, g: 1, tone: 3000, ring: 0.015, noise: 0.4, q: 2.5 }, { v: 'hiss', t: 0, g: 0.12, decay: 0.03, attack: 0.003 }],
                space: null
            },
            'window.deny': {
                notes: [{ v: 'knock', n: -5, t: 0.012, g: 0.9, pan: 0 }, { v: 'mellow', n: -5, t: 0.012, g: 0.5, pan: 0, decay: 0.12 }, { v: 'cel', n: -2, t: 0.06, g: 0.4, pan: 0.1, t60: 0.2 }],
                layers: [{ v: 'dock', t: 0, g: 0.8, tone: 500, material: 'felt', ring: 0.04 }],
                space: { size: 0.7, rt60: 0.16, damping: 4000, predelay: 0.004, mix: 0.2 }
            },
            'file.drop': {
                notes: [{ v: 'deep', n: -5, t: 0.01, g: 0.8, pan: 0, decay: 0.2, attack: 0.01, glide: 0, harm: 0.4 }, { v: 'ring', n: -5, t: 0.01, g: 0.36, pan: 0, t60: 0.20 }],
                layers: [{ v: 'dock', t: 0, g: 0.9, tone: 700, material: 'felt', ring: 0.05 }, { v: 'hiss', t: 0.02, g: 0.15, decay: 0.08 }],
                echo: { time: 0.14, feedback: 0.07, tone: 2500, mix: 0.12, spread: 0.3 },
                space: { size: 0.7, rt60: 0.10, damping: 4000, predelay: 0.006, mix: 0.16 }
            },
            'file.trash': {
                notes: [{ v: 'ping', n: 11, t: 0, g: 0.8, pan: 0.15, decay: 0.08 }, { v: 'ring', n: 11, t: 0, g: 0.32, pan: 0.15, t60: 0.09 }, { v: 'ping', n: 6, t: 0.05, g: 0.9, pan: 0, decay: 0.08 }, { v: 'ring', n: 6, t: 0.05, g: 0.36, pan: 0, t60: 0.09 },
                    { v: 'ping', n: 2, t: 0.1, g: 1, pan: -0.15, decay: 0.1 }, { v: 'ring', n: 2, t: 0.1, g: 0.40, pan: -0.15, t60: 0.11 }],
                layers: [{ v: 'whoosh', t: 0, g: 0.45, band: [400, 3000], sweep: -2, attack: 0.03, decay: 0.1 }],
                echo: { time: 0.12, feedback: 0.07, tone: 3000, mix: 0.12, spread: 0.3 },
                space: { size: 0.7, rt60: 0.07, damping: 4500, predelay: 0.006, mix: 0.15 }
            },
            'file.delete': {
                notes: [{ v: 'ping', n: 7, t: 0, g: 0.8, pan: 0.1, decay: 0.12 }, { v: 'ring', n: 7, t: 0, g: 0.32, pan: 0.1, t60: 0.13 }, { v: 'deep', n: -5, t: 0.1, g: 1, pan: -0.1, decay: 0.18, glide: 0, attack: 0.01, harm: 0.4 }, { v: 'ring', n: -5, t: 0.1, g: 0.45, pan: -0.1, t60: 0.18 }],
                layers: [{ v: 'whoosh', t: 0, g: 0.5, band: [300, 2500], sweep: -2, attack: 0.04, decay: 0.12 }, { v: 'dock', t: 0.1, g: 0.5, tone: 700, material: 'felt', ring: 0.05 }],
                echo: { time: 0.16, feedback: 0.04, tone: 2500, mix: 0.15, spread: 0.4 },
                space: { size: 0.9, rt60: 0.06, damping: 4000, predelay: 0.008, mix: 0.2 }
            },
            'space.switch': {
                notes: [{ v: 'pad', n: 0, t: 0.02, g: 0.8, pan: 0, attack: 0.12, hold: 0, release: 0.25 }],
                layers: [{ v: 'whoosh', t: 0, g: 0.9, attack: 0.15, decay: 0.2, band: [250, 3000], sweep: 1.5 }],
                space: { size: 1, rt60: 0.28, damping: 4500, predelay: 0.01, mix: 0.2 }
            },
            'menu.open': {
                notes: [{ v: 'ping', n: 4, t: 0, g: 0.85, pan: -0.1, decay: 0.05, dur: 0.08 }, { v: 'ring', n: 4, t: 0, g: 0.34, pan: -0.1, t60: 0.06 }, { v: 'ping', n: 11, t: 0.035, g: 1, pan: 0.1, decay: 0.05, dur: 0.09 }, { v: 'ring', n: 11, t: 0.035, g: 0.40, pan: 0.1, t60: 0.06 }],
                space: { size: 0.5, rt60: 0.07, damping: 5000, predelay: 0.002, mix: 0.1 }
            },
            'menu.close': {
                notes: [{ v: 'ping', n: 11, t: 0, g: 0.85, pan: 0.1, decay: 0.05, dur: 0.07 }, { v: 'ring', n: 11, t: 0, g: 0.34, pan: 0.1, t60: 0.06 }, { v: 'ping', n: 4, t: 0.03, g: 1, pan: -0.1, decay: 0.05, dur: 0.07 }, { v: 'ring', n: 4, t: 0.03, g: 0.40, pan: -0.1, t60: 0.06 }],
                space: { size: 0.5, rt60: 0.05, damping: 5000, predelay: 0.002, mix: 0.07 }
            },
            'ui.select': {
                layers: [{ v: 'dock', t: 0, g: 1, tone: 2600, ring: 0.02 }, { v: 'hiss', t: 0, g: 0.1, decay: 0.04 }],
                space: { size: 0.4, rt60: 0.06, damping: 6000, predelay: 0.002, mix: 0.08 }
            },
            'ui.toggle.on': {
                layers: [{ v: 'hiss', t: 0, g: 0.1, attack: 0.005, decay: 0.05 }, { v: 'dock', t: 0, g: 0.9, tone: 2400, ring: 0.015 }, { v: 'dock', t: 0.04, g: 1, tone: 3400, ring: 0.015 }],
                space: null
            },
            'ui.toggle.off': {
                layers: [{ v: 'dock', t: 0, g: 0.9, tone: 3400, ring: 0.015 }, { v: 'dock', t: 0.04, g: 1, tone: 2400, ring: 0.015 }, { v: 'hiss', t: 0.04, g: 0.05, attack: 0.005, decay: 0.04 }],
                space: { size: 0.4, rt60: 0.06, damping: 6000, predelay: 0.002, mix: 0.08 }
            },
            'ui.tick': {
                layers: [{ v: 'dock', t: 0, g: 1, tone: 3800, ring: 0.01, length: 0.002 }],
                space: null
            }
        },
        fx: {
            'orbit.ping': {
                small: {
                    notes: [{ v: 'ping', n: 12, t: 0, g: 1, decay: 0.15, attack: 0.005 }],
                    echo: { time: 0.12, feedback: 0.18, tone: 4000, mix: 0.4, spread: 0.5 }
                },
                medium: {
                    notes: [{ v: 'ping', n: 7, t: 0, g: 1, decay: 0.25, attack: 0.006 }],
                    echo: { time: 0.17, feedback: 0.18, tone: 3500, mix: 0.4, spread: 0.6 }
                },
                large: {
                    notes: [{ v: 'ping', n: 0, t: 0, g: 1, decay: 0.2, attack: 0.008 }],
                    echo: { time: 0.2, feedback: 0.12, tone: 3000, mix: 0.4, spread: 0.7 }
                }
            }
        }
    });
})();
