(function (rootScope) {
    'use strict';

    // High-gloss 5-mode canvas visualizer engine for the native AuraGo music player.
    // The player owns the Web Audio graph. This module only reads analyser frames
    // and paints its own canvas. Optional callbacks report loudness so the shell
    // can breathe with the music; they never receive audio samples.

    const MODES = Object.freeze(['bars', 'radial', 'wave', 'nebula', 'horizon']);
    const BAR_COUNT = 64;
    const HORIZON_LAYERS = 12;
    const NEBULA_NODES = 96;
    const SHOCKWAVE_SLOTS = 5;
    const PEAK_HOLD_FRAMES = 10;

    function clamp(value, min, max) {
        return Math.max(min, Math.min(max, value));
    }

    function lerp(a, b, t) {
        return a + (b - a) * t;
    }

    function hueToRgb(p, q, t) {
        if (t < 0) t += 1;
        if (t > 1) t -= 1;
        if (t < 1 / 6) return p + (q - p) * 6 * t;
        if (t < 1 / 2) return q;
        if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6;
        return p;
    }

    function rgbToHsl(r, g, b) {
        const rf = clamp(r, 0, 255) / 255;
        const gf = clamp(g, 0, 255) / 255;
        const bf = clamp(b, 0, 255) / 255;
        const max = Math.max(rf, gf, bf);
        const min = Math.min(rf, gf, bf);
        const l = (max + min) / 2;
        if (max === min) return { h: 0, s: 0, l };
        const d = max - min;
        const s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
        let h;
        if (max === rf) h = (gf - bf) / d + (gf < bf ? 6 : 0);
        else if (max === gf) h = (bf - rf) / d + 2;
        else h = (rf - gf) / d + 4;
        return { h: h / 6, s, l };
    }

    function hslToRgbString(h, s, l) {
        const hue = ((h % 1) + 1) % 1;
        const sat = clamp(s, 0, 1);
        const lit = clamp(l, 0, 1);
        if (sat === 0) {
            const v = Math.round(lit * 255);
            return `rgb(${v},${v},${v})`;
        }
        const q = lit < 0.5 ? lit * (1 + sat) : lit + sat - lit * sat;
        const p = 2 * lit - q;
        const r = Math.round(hueToRgb(p, q, hue + 1 / 3) * 255);
        const g = Math.round(hueToRgb(p, q, hue) * 255);
        const b = Math.round(hueToRgb(p, q, hue - 1 / 3) * 255);
        return `rgb(${r},${g},${b})`;
    }

    function shiftHue(r, g, b, degrees, satBoost, lightOffset) {
        const hsl = rgbToHsl(r, g, b);
        if (hsl.s < 0.04) {
            const l = clamp(hsl.l + (lightOffset || 0), 0.12, 0.94);
            const v = Math.round(l * 255);
            return `rgb(${v},${v},${v})`;
        }
        const h = hsl.h + (degrees || 0) / 360;
        const s = clamp(hsl.s + (satBoost || 0), 0.18, 0.98);
        const l = clamp(hsl.l + (lightOffset || 0), 0.14, 0.88);
        return hslToRgbString(h, s, l);
    }

    function buildHarmonizedPalette(r, g, b, muted, isLight) {
        const red = clamp(Math.round(r), 0, 255);
        const green = clamp(Math.round(g), 0, 255);
        const blue = clamp(Math.round(b), 0, 255);
        const hsl = rgbToHsl(red, green, blue);
        const minL = isLight ? 0.34 : 0.46;
        const maxL = isLight ? 0.56 : 0.70;
        const baseS = hsl.s < 0.08 ? hsl.s : clamp(Math.max(hsl.s, 0.58), 0.25, 0.96);
        const baseL = clamp(hsl.l, minL, maxL);
        const accent = hslToRgbString(hsl.h, baseS, baseL);
        const accent2 = hslToRgbString(hsl.h + 38 / 360, clamp(baseS + 0.06, 0.25, 0.98), clamp(baseL + (isLight ? -0.02 : 0.06), 0.26, 0.78));
        const accent3 = hslToRgbString(hsl.h - 42 / 360, clamp(baseS + 0.04, 0.25, 0.96), clamp(baseL + (isLight ? -0.05 : 0.02), 0.24, 0.74));
        const hot = isLight
            ? hslToRgbString(hsl.h + 18 / 360, clamp(baseS + 0.12, 0.35, 1), 0.44)
            : hslToRgbString(hsl.h + 22 / 360, clamp(baseS * 0.78, 0.25, 0.95), 0.82);
        const deep = hslToRgbString(hsl.h - 14 / 360, clamp(baseS + 0.08, 0.25, 0.95), isLight ? 0.28 : 0.26);
        return {
            accent,
            accent2,
            accent3,
            hot,
            deep,
            muted: muted || (isLight ? '#4b5c73' : '#a8bbd0'),
            r: red,
            g: green,
            b: blue
        };
    }

    function css(color, alpha) {
        if (alpha == null || alpha >= 1) return color;
        const a = clamp(alpha, 0, 1).toFixed(3);
        if (typeof color === 'string') {
            if (color.startsWith('rgb(')) {
                return 'rgba(' + color.slice(4, -1) + ',' + a + ')';
            }
            if (color.startsWith('#') && (color.length === 7 || color.length === 4)) {
                let hex = color.slice(1);
                if (hex.length === 3) hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2];
                const n = parseInt(hex, 16);
                if (Number.isFinite(n)) {
                    return `rgba(${(n >> 16) & 255},${(n >> 8) & 255},${n & 255},${a})`;
                }
            }
        }
        return color;
    }

    function detectLightScheme(root) {
        if (typeof document !== 'undefined' && document.body && document.body.dataset) {
            if (document.body.dataset.scheme === 'light') return true;
            if (document.body.dataset.scheme === 'dark') return false;
        }
        try {
            if (typeof getComputedStyle === 'function' && root) {
                const bg = getComputedStyle(root).backgroundColor || '';
                const m = /^rgba?\((\d+),\s*(\d+),\s*(\d+)/i.exec(bg);
                if (m) {
                    const lum = (Number(m[1]) * 0.299 + Number(m[2]) * 0.587 + Number(m[3]) * 0.114) / 255;
                    return lum > 0.62;
                }
            }
        } catch (_) { /* ignore */ }
        return false;
    }

    function readPalette(root) {
        const isLight = detectLightScheme(root);
        let red = 39;
        let green = 199;
        let blue = 166;
        let muted = isLight ? '#4b5c73' : '#a8bbd0';
        try {
            const styles = getComputedStyle(root || document.documentElement);
            const read = (name, fallback) => (styles.getPropertyValue(name) || '').trim() || fallback;
            const r = parseInt(read('--vd-accent-r', '39'), 10);
            const g = parseInt(read('--vd-accent-g', '199'), 10);
            const b = parseInt(read('--vd-accent-b', '166'), 10);
            if (Number.isFinite(r)) red = r;
            if (Number.isFinite(g)) green = g;
            if (Number.isFinite(b)) blue = b;
            muted = read('--vd-muted', muted);
        } catch (_) { /* ignore in non-DOM envs */ }
        return buildHarmonizedPalette(red, green, blue, muted, isLight);
    }

    function roundedBar(context, x, y, width, height, radius) {
        const h = Math.max(0, height);
        const r = Math.min(radius, width / 2, h / 2);
        context.beginPath();
        if (typeof context.roundRect === 'function') {
            context.roundRect(x, y, width, h, r);
            return;
        }
        context.moveTo(x + r, y);
        context.lineTo(x + width - r, y);
        context.quadraticCurveTo(x + width, y, x + width, y + r);
        context.lineTo(x + width, y + h - r);
        context.quadraticCurveTo(x + width, y + h, x + width - r, y + h);
        context.lineTo(x + r, y + h);
        context.quadraticCurveTo(x, y + h, x, y + h - r);
        context.lineTo(x, y + r);
        context.quadraticCurveTo(x, y, x + r, y);
        context.closePath();
    }

    // Deterministic hash for stable harmonic offsets without per-frame RNG overhead.
    function hashPhase(index) {
        let h = (index + 1) * 2654435761;
        h ^= h >>> 13;
        h = Math.imul(h, 2246822519);
        h ^= h >>> 16;
        return (h >>> 0) / 4294967295 * Math.PI * 2;
    }

    function hashUnit(index) {
        let h = (index + 11) * 1597334677;
        h ^= h >>> 15;
        h = Math.imul(h, 3812015801);
        h ^= h >>> 13;
        return (h >>> 0) / 4294967295;
    }

    function smoothTrace(path, points) {
        if (!points.length) return;
        path.moveTo(points[0][0], points[0][1]);
        if (points.length === 1) return;
        for (let i = 1; i < points.length - 1; i++) {
            const xc = (points[i][0] + points[i + 1][0]) / 2;
            const yc = (points[i][1] + points[i + 1][1]) / 2;
            path.quadraticCurveTo(points[i][0], points[i][1], xc, yc);
        }
        path.lineTo(points[points.length - 1][0], points[points.length - 1][1]);
    }

    function smoothClosedLoop(path, points) {
        const n = points.length;
        if (n < 3) return;
        const startX = (points[n - 1][0] + points[0][0]) / 2;
        const startY = (points[n - 1][1] + points[0][1]) / 2;
        path.moveTo(startX, startY);
        for (let i = 0; i < n; i++) {
            const curr = points[i];
            const next = points[(i + 1) % n];
            const midX = (curr[0] + next[0]) / 2;
            const midY = (curr[1] + next[1]) / 2;
            path.quadraticCurveTo(curr[0], curr[1], midX, midY);
        }
        path.closePath();
    }

    function create(canvas, options) {
        if (!canvas || typeof canvas.getContext !== 'function') return null;
        const context = canvas.getContext('2d');
        if (!context) return null;
        const opts = options || {};
        const mediaQuery = typeof matchMedia === 'function' ? matchMedia('(prefers-reduced-motion: reduce)') : null;

        // The shell switches its animations off with data-animations="false".
        // Every other animated part of the desktop honours that switch, and so
        // does this stage: without it the player keeps a 60 fps canvas loop
        // running while the user asked for a calm desktop.
        function motionAllowed() {
            if (document.body && document.body.dataset.animations === 'false') return false;
            return !(mediaQuery && mediaQuery.matches);
        }

        const horizonHistory = [];
        for (let i = 0; i < HORIZON_LAYERS; i++) {
            horizonHistory.push(new Float32Array(BAR_COUNT));
        }

        const state = {
            mode: MODES.indexOf(opts.mode) >= 0 ? opts.mode : 'bars',
            playing: false,
            isLight: detectLightScheme(opts.root || canvas),
            palette: opts.palette || readPalette(opts.root || canvas),
            coverLocked: false,
            levels: new Float32Array(BAR_COUNT),
            peaks: new Float32Array(BAR_COUNT),
            peakHold: new Uint8Array(BAR_COUNT),
            peakVel: new Float32Array(BAR_COUNT),
            raw: new Float32Array(BAR_COUNT),
            horizonHistory,
            frequency: null,
            waveform: null,
            width: 0,
            height: 0,
            ratio: 1,
            frame: 0,
            rotation: 0,
            horizonPhase: 0,
            transition: 1,
            bass: 0,
            mid: 0,
            treble: 0,
            energy: 0,
            bassAvg: 0.14,
            beat: 0,
            beatCooldown: 0,
            smoothBass: 0,
            smoothMid: 0,
            smoothTreble: 0,
            smoothEnergy: 0,
            sentBass: -1,
            reduced: !motionAllowed(),
            destroyed: false,
            visible: true,
            cacheKey: '',
            cache: {},
            motes: [],
            nebulaStars: [],
            skyStars: [],
            shockwaves: [],
            handle: 0,
            resizeQueued: false
        };

        function seedPools() {
            const moteCount = state.reduced ? 0 : 40;
            state.motes = [];
            for (let i = 0; i < moteCount; i++) {
                state.motes.push({
                    x: (i * 0.61803398875) % 1,
                    y: (i * 0.38196601125) % 1,
                    size: 0.8 + ((i * 37) % 10) / 10 * 1.6,
                    speed: 0.00014 + ((i * 13) % 10) / 10 * 0.00038,
                    drift: hashPhase(i),
                    depth: (i % 5) / 5
                });
            }

            state.nebulaStars = [];
            for (let i = 0; i < NEBULA_NODES; i++) {
                const angle = hashPhase(i * 3 + 1);
                const radius = 0.14 + Math.sqrt(hashUnit(i * 7 + 3)) * 0.88;
                const z = 0.08 + hashUnit(i * 5 + 2) * 0.92;
                state.nebulaStars.push({
                    x: Math.cos(angle) * radius,
                    y: Math.sin(angle) * radius * 0.72,
                    z,
                    prevZ: z,
                    band: Math.floor(hashUnit(i * 11) * BAR_COUNT),
                    tone: i % 3,
                    size: 1.1 + hashUnit(i * 13) * 2.1,
                    twinkle: hashPhase(i * 17)
                });
            }

            state.skyStars = [];
            for (let i = 0; i < 42; i++) {
                state.skyStars.push({
                    x: hashUnit(i * 19 + 5),
                    y: 0.04 + hashUnit(i * 23 + 7) * 0.36,
                    size: 0.6 + hashUnit(i * 29) * 1.4,
                    phase: hashPhase(i * 31)
                });
            }

            state.shockwaves = [];
            for (let i = 0; i < SHOCKWAVE_SLOTS; i++) {
                state.shockwaves.push({ active: false, progress: 1, strength: 0 });
            }
        }

        function seedIdleHorizon() {
            for (let l = 0; l < HORIZON_LAYERS; l++) {
                const row = state.horizonHistory[l];
                for (let i = 0; i < BAR_COUNT; i++) {
                    const center = 1 - Math.abs((i / (BAR_COUNT - 1)) * 2 - 1);
                    const ridge = 0.22 + 0.34 * Math.sin(i * 0.28 + l * 0.55) * Math.sin(i * 0.11 - l * 0.3);
                    row[i] = clamp((0.12 + Math.abs(ridge)) * (0.45 + 0.55 * center), 0.05, 0.72);
                }
            }
        }

        seedPools();
        seedIdleHorizon();

        function emitShockwave(strength) {
            if (state.reduced) return;
            for (let i = 0; i < state.shockwaves.length; i++) {
                if (!state.shockwaves[i].active) {
                    state.shockwaves[i].active = true;
                    state.shockwaves[i].progress = 0;
                    state.shockwaves[i].strength = clamp(strength, 0.35, 1);
                    return;
                }
            }
        }

        function onMotionChange() {
            state.reduced = !motionAllowed();
            seedPools();
            state.transition = 1;
            state.cacheKey = '';
        }

        if (mediaQuery && typeof mediaQuery.addEventListener === 'function') {
            mediaQuery.addEventListener('change', onMotionChange);
        } else if (mediaQuery && typeof mediaQuery.addListener === 'function') {
            mediaQuery.addListener(onMotionChange);
        }

        const motionObserver = (typeof MutationObserver === 'function' && document.body)
            ? new MutationObserver(onMotionChange)
            : null;
        if (motionObserver) motionObserver.observe(document.body, { attributes: true, attributeFilter: ['data-animations'] });

        function resize() {
            if (state.destroyed) return;
            const ratio = Math.min(2, Math.max(1, window.devicePixelRatio || 1));
            const rect = canvas.getBoundingClientRect();
            const width = Math.max(1, Math.round(rect.width || canvas.clientWidth || 320));
            const height = Math.max(1, Math.round(rect.height || canvas.clientHeight || 180));
            state.ratio = ratio;
            state.width = width;
            state.height = height;
            canvas.width = Math.round(width * ratio);
            canvas.height = Math.round(height * ratio);
            context.setTransform(ratio, 0, 0, ratio, 0, 0);
            state.cacheKey = '';
        }

        function queueResize() {
            if (state.resizeQueued || state.destroyed) return;
            state.resizeQueued = true;
            window.requestAnimationFrame(() => {
                state.resizeQueued = false;
                resize();
            });
        }

        function analyser() {
            return typeof opts.getAnalyser === 'function' ? opts.getAnalyser() : null;
        }

        function publishEnergy() {
            if (typeof opts.onEnergy !== 'function') return;
            if (state.reduced) {
                if (state.sentBass !== 0) {
                    state.sentBass = 0;
                    opts.onEnergy(0, 0);
                }
                return;
            }
            state.smoothBass = state.smoothBass * 0.80 + state.bass * 0.20;
            state.smoothMid = state.smoothMid * 0.82 + state.mid * 0.18;
            state.smoothTreble = state.smoothTreble * 0.80 + state.treble * 0.20;
            state.smoothEnergy = state.smoothEnergy * 0.85 + state.energy * 0.15;
            const out = clamp(state.smoothBass * 0.78 + state.smoothEnergy * 0.30, 0, 1);
            if (Math.abs(out - state.sentBass) < 0.012 && state.frame % 8 !== 0) return;
            state.sentBass = out;
            opts.onEnergy(out, state.smoothEnergy);
        }

        function stageShape(index, count) {
            const u = count <= 1 ? 0.5 : index / (count - 1);
            const arch = Math.sin(Math.PI * u);
            const harmonic = 0.12 * Math.sin(u * Math.PI * 5);
            return clamp(0.36 + 0.54 * arch + harmonic, 0.2, 1);
        }

        function idleLevel(index, count) {
            if (state.reduced) return stageShape(index, count) * 0.40;
            const t = state.frame / 42;
            const phase = hashPhase(index);
            const waveA = 0.54 + 0.46 * Math.sin(t * 1.45 + index * 0.22 + phase * 0.35);
            const waveB = 0.76 + 0.24 * Math.cos(t * 0.72 - index * 0.14);
            return clamp(stageShape(index, count) * waveA * waveB * 0.64, 0.06, 0.82);
        }

        function pushHorizonRow() {
            const recycled = state.horizonHistory.pop();
            recycled.set(state.levels);
            state.horizonHistory.unshift(recycled);
        }

        function updatePeaks(i) {
            const value = state.levels[i];
            if (value >= state.peaks[i]) {
                state.peaks[i] = value;
                state.peakHold[i] = PEAK_HOLD_FRAMES;
                state.peakVel[i] = 0;
            } else if (state.peakHold[i] > 0) {
                state.peakHold[i] -= 1;
            } else {
                state.peakVel[i] += 0.0014;
                state.peaks[i] = Math.max(value, state.peaks[i] - state.peakVel[i]);
            }
        }

        function readLevels() {
            const bars = state.levels;
            const node = analyser();
            if (!node) {
                let energy = 0;
                let bass = 0;
                let mid = 0;
                let treble = 0;
                for (let i = 0; i < bars.length; i++) {
                    const idle = idleLevel(i, bars.length);
                    bars[i] = state.reduced ? idle : bars[i] * 0.78 + idle * 0.22;
                    updatePeaks(i);
                    energy += bars[i];
                    if (i < 12) bass += bars[i];
                    else if (i < 40) mid += bars[i];
                    else treble += bars[i];
                }
                state.energy = energy / bars.length;
                state.bass = state.reduced ? 0.18 : state.bass * 0.85 + (bass / 12) * 0.15;
                state.mid = state.reduced ? 0.22 : state.mid * 0.85 + (mid / 28) * 0.15;
                state.treble = state.reduced ? 0.16 : state.treble * 0.85 + (treble / 24) * 0.15;
                state.beat *= 0.88;
                if (!state.reduced && state.frame % 4 === 0) pushHorizonRow();
                return false;
            }

            const size = node.frequencyBinCount || 1024;
            if (!state.frequency || state.frequency.length !== size) state.frequency = new Uint8Array(size);
            node.getByteFrequencyData(state.frequency);
            const bins = state.frequency;
            const usable = Math.max(8, Math.floor(size * 0.65));
            let energy = 0;
            let bass = 0;
            let mid = 0;
            let treble = 0;
            const raw = state.raw;
            for (let i = 0; i < bars.length; i++) {
                const start = Math.floor(Math.pow(i / bars.length, 1.72) * usable);
                const end = Math.max(start + 1, Math.floor(Math.pow((i + 1) / bars.length, 1.72) * usable));
                let sum = 0;
                let peakBin = 0;
                for (let bin = start; bin < end && bin < size; bin++) {
                    sum += bins[bin];
                    if (bins[bin] > peakBin) peakBin = bins[bin];
                }
                const span = Math.max(1, Math.min(end, size) - start);
                const average = (sum / span * 0.70 + peakBin * 0.30) / 255;
                const value = clamp(Math.pow(average, 1.06) * 1.18, 0, 1);
                energy += value;
                if (i < 10) bass += value;
                else if (i < 38) mid += value;
                else treble += value;
                raw[i] = value;
            }
            const mean = energy / bars.length;
            const quiet = mean < 0.04;
            for (let i = 0; i < bars.length; i++) {
                const target = quiet ? idleLevel(i, bars.length) : raw[i];
                const previous = bars[i];
                bars[i] = state.reduced
                    ? target
                    : target > previous
                        ? previous * 0.25 + target * 0.75
                        : previous * 0.83 + target * 0.17;
                updatePeaks(i);
            }
            state.energy = mean;
            const bassLevel = quiet ? 0.18 : bass / 10;
            state.bass = state.bass * 0.56 + bassLevel * 0.44;
            state.mid = state.mid * 0.70 + (quiet ? 0.20 : mid / 28) * 0.30;
            state.treble = state.treble * 0.70 + (quiet ? 0.16 : treble / 26) * 0.30;

            // Adaptive beat onset detector.
            state.bassAvg = state.bassAvg * 0.94 + bassLevel * 0.06;
            if (state.beatCooldown > 0) state.beatCooldown -= 1;
            if (!quiet && bassLevel > state.bassAvg * 1.32 + 0.08 && bassLevel > 0.34 && state.beatCooldown === 0) {
                state.beat = clamp((bassLevel - state.bassAvg) * 2.2, 0.45, 1);
                state.beatCooldown = 7;
                emitShockwave(state.beat);
            } else {
                state.beat *= 0.87;
            }

            if (!state.reduced && state.frame % 3 === 0) pushHorizonRow();
            return mean > 0.02;
        }

        function readWaveform() {
            const node = analyser();
            if (!node) return null;
            const size = node.fftSize || 2048;
            if (!state.waveform || state.waveform.length !== size) state.waveform = new Uint8Array(size);
            node.getByteTimeDomainData(state.waveform);
            return state.waveform;
        }

        function paletteKey() {
            const p = state.palette;
            return [p.accent, p.accent2, p.accent3, p.hot, p.muted, state.isLight ? 'L' : 'D', state.width, state.height, state.mode].join('|');
        }

        function ensureCache() {
            const key = paletteKey();
            if (key === state.cacheKey && state.cache.ready) return state.cache;
            const width = Math.max(1, state.width);
            const height = Math.max(1, state.height);
            const p = state.palette;

            const baseline = height * 0.71;
            const usable = height * 0.55;
            const barGradient = context.createLinearGradient(0, baseline - usable, 0, baseline);
            barGradient.addColorStop(0, p.hot);
            barGradient.addColorStop(0.28, p.accent2);
            barGradient.addColorStop(0.68, p.accent);
            barGradient.addColorStop(1, css(p.deep, 0.92));

            const mirrorGradient = context.createLinearGradient(0, baseline, 0, Math.min(height, baseline + usable * 0.36));
            mirrorGradient.addColorStop(0, css(p.accent, state.isLight ? 0.24 : 0.34));
            mirrorGradient.addColorStop(1, css(p.accent, 0));

            const waveGradient = context.createLinearGradient(0, 0, width, 0);
            waveGradient.addColorStop(0, p.accent3);
            waveGradient.addColorStop(0.32, p.accent);
            waveGradient.addColorStop(0.68, p.accent2);
            waveGradient.addColorStop(1, p.hot);

            const waveSilkGradient = context.createLinearGradient(0, height * 0.16, 0, height * 0.78);
            waveSilkGradient.addColorStop(0, css(p.accent2, state.isLight ? 0.22 : 0.32));
            waveSilkGradient.addColorStop(0.5, css(p.accent, state.isLight ? 0.14 : 0.20));
            waveSilkGradient.addColorStop(1, css(p.accent3, 0));

            const sunCy = height * 0.34;
            const sunR = clamp(Math.min(width, height) * 0.20, 28, 128);
            const sunGradient = context.createLinearGradient(0, sunCy - sunR, 0, sunCy + sunR);
            sunGradient.addColorStop(0, p.hot);
            sunGradient.addColorStop(0.45, p.accent2);
            sunGradient.addColorStop(1, p.accent3);

            state.cache = {
                ready: true,
                barGradient,
                mirrorGradient,
                waveGradient,
                waveSilkGradient,
                sunGradient,
                baseline,
                usable,
                sunCy,
                sunR
            };
            state.cacheKey = key;
            return state.cache;
        }

        function blendOp() {
            return state.isLight ? 'source-over' : 'lighter';
        }

        function drawAtmosphere() {
            const width = state.width;
            const height = state.height;
            const lift = state.reduced ? 0 : clamp(state.smoothBass * 0.78 + state.smoothEnergy * 0.36 + state.beat * 0.25, 0, 1);
            const cx = width * 0.5;
            const cy = height * 0.41;
            const radius = Math.min(width, height) * (0.40 + lift * 0.11);

            const glow = context.createRadialGradient(cx, cy, radius * 0.05, cx, cy, radius);
            glow.addColorStop(0, css(state.palette.accent2, state.isLight ? 0.22 : 0.50));
            glow.addColorStop(0.48, css(state.palette.accent, state.isLight ? 0.14 : 0.30));
            glow.addColorStop(1, 'rgba(0,0,0,0)');
            context.globalAlpha = (0.20 + lift * 0.26) * (state.isLight ? 0.75 : 1);
            context.fillStyle = glow;
            context.beginPath();
            context.arc(cx, cy, radius, 0, Math.PI * 2);
            context.fill();

            const halo = context.createRadialGradient(cx, cy, radius * 0.35, cx, cy, Math.max(width, height) * 0.68);
            halo.addColorStop(0, css(state.palette.accent3, state.isLight ? 0.08 : 0.14));
            halo.addColorStop(1, 'rgba(0,0,0,0)');
            context.globalAlpha = 0.55;
            context.fillStyle = halo;
            context.fillRect(0, 0, width, height);
            context.globalAlpha = 1;

            if (!state.isLight) {
                const shade = context.createLinearGradient(0, 0, 0, height * 0.36);
                shade.addColorStop(0, 'rgba(0,0,0,0.24)');
                shade.addColorStop(1, 'rgba(0,0,0,0)');
                context.fillStyle = shade;
                context.fillRect(0, 0, width, Math.ceil(height * 0.36));
            }
        }

        function drawMotes() {
            if (state.reduced || !state.motes.length) return;
            const width = state.width;
            const height = state.height;
            const lift = 0.28 + clamp(state.smoothEnergy * 1.35 + state.beat * 0.3, 0, 1) * 0.72;
            context.save();
            for (let i = 0; i < state.motes.length; i++) {
                const mote = state.motes[i];
                const rise = (state.frame * mote.speed + mote.y) % 1;
                const x = ((mote.x + Math.sin(state.frame / 280 + mote.drift) * 0.014 * (1 + mote.depth)) + 1) % 1 * width;
                const y = (1 - rise) * height;
                const twinkle = 0.5 + 0.5 * Math.sin(state.frame / 32 + mote.drift * 3);
                context.globalAlpha = (0.06 + mote.depth * 0.11 + twinkle * 0.08) * lift;
                context.fillStyle = i % 2 === 0 ? state.palette.accent2 : state.palette.hot;
                const r = mote.size * (0.7 + mote.depth * 0.6);
                context.beginPath();
                context.arc(x, y, r, 0, Math.PI * 2);
                context.fill();
            }
            context.restore();
            context.globalAlpha = 1;
        }

        // Mode 1: Spectrum Pro — studio-grade acrylic harmonic columns, glass sheen, additive bloom & gravity peaks.
        function drawBars() {
            const width = state.width;
            const height = state.height;
            const cache = ensureCache();
            const count = state.levels.length;
            const marginX = clamp(width * 0.032, 8, 28);
            const spanW = Math.max(60, width - marginX * 2);
            const gap = spanW > 740 ? 3 : 2;
            const barWidth = Math.max(2, (spanW - gap * (count - 1)) / count);
            const baseline = cache.baseline;
            const usable = cache.usable;
            const pulse = state.reduced ? 0 : clamp(state.smoothBass + state.beat * 0.35, 0, 1);

            // Subtle studio dB guide gridlines in the background.
            context.save();
            context.strokeStyle = state.isLight ? 'rgba(20, 32, 48, 0.07)' : 'rgba(255, 255, 255, 0.05)';
            context.lineWidth = 1;
            for (let g = 1; g <= 4; g++) {
                const gy = Math.round(baseline - usable * (g / 4)) + 0.5;
                context.beginPath();
                context.moveTo(marginX, gy);
                context.lineTo(width - marginX, gy);
                context.stroke();
            }
            context.restore();

            // Beat-reactive volumetric stage floor puddle.
            context.save();
            const floorGlow = context.createRadialGradient(width * 0.5, baseline, 4, width * 0.5, baseline, spanW * 0.52);
            floorGlow.addColorStop(0, css(state.palette.hot, (state.isLight ? 0.18 : 0.34) * (0.35 + pulse * 0.65)));
            floorGlow.addColorStop(0.55, css(state.palette.accent, (state.isLight ? 0.10 : 0.20) * (0.35 + pulse * 0.65)));
            floorGlow.addColorStop(1, 'rgba(0,0,0,0)');
            context.fillStyle = floorGlow;
            context.beginPath();
            context.ellipse(width * 0.5, baseline, spanW * 0.52, 20 + pulse * 34, 0, 0, Math.PI * 2);
            context.fill();
            context.restore();

            // Soft drop shadow behind columns for 3D detachment.
            context.save();
            context.globalAlpha = state.isLight ? 0.12 : 0.28;
            context.fillStyle = '#04070d';
            for (let i = 0; i < count; i++) {
                const value = clamp(state.levels[i], 0, 1);
                const barH = Math.max(3, value * usable);
                const x = marginX + i * (barWidth + gap);
                roundedBar(context, x + 1.2, baseline - barH + 3, barWidth, barH, Math.min(4.5, barWidth * 0.45));
                context.fill();
            }
            context.restore();

            // Outer additive bloom behind energetic columns.
            if (!state.reduced) {
                context.save();
                context.globalCompositeOperation = blendOp();
                context.fillStyle = state.palette.accent2;
                for (let i = 0; i < count; i++) {
                    const value = clamp(state.levels[i], 0, 1);
                    if (value < 0.32) continue;
                    const barH = value * usable;
                    const x = marginX + i * (barWidth + gap);
                    context.globalAlpha = (value - 0.32) * (state.isLight ? 0.16 : 0.24);
                    roundedBar(context, x - 1.5, baseline - barH - 2, barWidth + 3, barH + 4, (barWidth + 3) * 0.45);
                    context.fill();
                }
                context.restore();
            }

            // Main Crystal Columns.
            context.fillStyle = cache.barGradient;
            for (let i = 0; i < count; i++) {
                const value = clamp(state.levels[i], 0, 1);
                const barH = Math.max(3, value * usable);
                const x = marginX + i * (barWidth + gap);
                roundedBar(context, x, baseline - barH, barWidth, barH, Math.min(4.5, barWidth * 0.45));
                context.fill();
            }

            // Glass tube left-edge specular sheen & crown lights.
            context.save();
            for (let i = 0; i < count; i++) {
                const value = clamp(state.levels[i], 0, 1);
                const barH = Math.max(3, value * usable);
                const x = marginX + i * (barWidth + gap);
                if (barWidth >= 4 && barH > 12) {
                    context.globalAlpha = state.isLight ? 0.28 : 0.20;
                    context.fillStyle = '#ffffff';
                    context.fillRect(x + 1, baseline - barH + 4, Math.max(1, barWidth * 0.18), barH - 6);
                }
                if (value >= 0.12) {
                    context.globalAlpha = (0.28 + value * 0.56);
                    context.fillStyle = state.isLight ? state.palette.hot : '#ffffff';
                    const capH = Math.min(3, barH * 0.26);
                    roundedBar(context, x + barWidth * 0.14, baseline - barH + 0.5, barWidth * 0.72, capH, capH / 2);
                    context.fill();
                }
            }
            context.restore();

            // Fading glossy mirror below the floor.
            context.save();
            context.globalAlpha = state.isLight ? 0.45 : 0.58;
            context.fillStyle = cache.mirrorGradient;
            for (let i = 0; i < count; i++) {
                const value = clamp(state.levels[i], 0, 1);
                const barH = Math.max(2, value * usable) * 0.34;
                const x = marginX + i * (barWidth + gap);
                roundedBar(context, x, baseline + 3, barWidth, barH, Math.min(4, barWidth * 0.4));
                context.fill();
            }
            context.restore();

            // Physics gravity peak caps with soft trailing halo.
            context.save();
            for (let i = 0; i < count; i++) {
                const peak = clamp(state.peaks[i], 0, 1);
                if (peak < 0.06) continue;
                const x = marginX + i * (barWidth + gap);
                const y = baseline - Math.max(3, peak * usable) - 6;
                context.globalAlpha = 0.22;
                context.fillStyle = state.palette.accent2;
                context.fillRect(x + barWidth * 0.2, y + 3, barWidth * 0.6, 5);
                context.globalAlpha = 0.94;
                context.fillStyle = state.palette.hot;
                roundedBar(context, x, y, barWidth, 3, 1.5);
                context.fill();
            }
            context.restore();

            // Crisp floor horizon hairline + center lens glint.
            context.save();
            const lineGrad = context.createLinearGradient(marginX, 0, width - marginX, 0);
            lineGrad.addColorStop(0, css(state.palette.accent3, 0.08));
            lineGrad.addColorStop(0.5, css(state.palette.hot, 0.72));
            lineGrad.addColorStop(1, css(state.palette.accent2, 0.08));
            context.fillStyle = lineGrad;
            context.fillRect(marginX, Math.round(baseline), spanW, 1.5);
            context.restore();
        }

        // Mode 2: Solaris Core — Concentric plasma aura, dual-layer rotating rays, waveform iris & beat shockwaves.
        function drawRadial() {
            const width = state.width;
            const height = state.height;
            const cx = width * 0.5;
            const cy = height * 0.405;
            const pulse = state.reduced ? 0 : clamp(state.smoothBass + state.beat * 0.42, 0, 1.2);
            const base = clamp(Math.min(width, height) * 0.19, 28, 128);
            const radius = base * (1 + pulse * 0.085);
            const count = state.levels.length;
            const rotation = state.reduced ? 0 : state.rotation;
            const waveform = readWaveform();

            // Expansive corona & rotating sweep.
            const core = context.createRadialGradient(cx, cy, radius * 0.1, cx, cy, radius * 1.45);
            core.addColorStop(0, css(state.palette.hot, state.isLight ? 0.28 : 0.68));
            core.addColorStop(0.45, css(state.palette.accent, state.isLight ? 0.18 : 0.42));
            core.addColorStop(0.78, css(state.palette.accent3, 0.12));
            core.addColorStop(1, 'rgba(0,0,0,0)');
            context.save();
            context.globalCompositeOperation = blendOp();
            context.globalAlpha = 0.42 + pulse * 0.24;
            context.fillStyle = core;
            context.beginPath();
            context.arc(cx, cy, radius * 1.45, 0, Math.PI * 2);
            context.fill();
            context.restore();

            // Closed smooth liquid-plasma outer aura envelope.
            const auraPoints = [];
            for (let i = 0; i < count; i++) {
                const mirrorIdx = i < count / 2 ? i * 2 : (count - 1 - i) * 2;
                const lvl = clamp(state.levels[mirrorIdx % count], 0, 1);
                const angle = (i / count) * Math.PI * 2 - Math.PI / 2 + rotation * 0.35;
                const r = radius * (1.02 + lvl * 0.66);
                auraPoints.push([cx + Math.cos(angle) * r, cy + Math.sin(angle) * r]);
            }
            context.save();
            context.beginPath();
            smoothClosedLoop(context, auraPoints);
            context.fillStyle = css(state.palette.accent2, state.isLight ? 0.12 : 0.16);
            context.fill();
            context.globalCompositeOperation = blendOp();
            context.strokeStyle = state.palette.accent2;
            context.globalAlpha = 0.28;
            context.lineWidth = Math.max(5, radius * 0.07);
            context.stroke();
            context.strokeStyle = state.palette.hot;
            context.globalAlpha = 0.85;
            context.lineWidth = 1.8;
            context.stroke();
            context.restore();

            // Dual-pass precision spectrum crown rays.
            context.save();
            context.lineCap = 'round';
            context.lineWidth = Math.max(2.2, radius * 0.042);
            for (let pass = 0; pass < 2; pass++) {
                context.strokeStyle = pass === 0 ? state.palette.accent : state.palette.accent3;
                for (let i = 0; i < count; i++) {
                    const value = clamp(state.levels[i], 0, 1);
                    const angle = (i / count) * Math.PI * 2 - Math.PI / 2 + rotation * (pass === 0 ? 1 : -0.65);
                    const inner = radius * (pass === 0 ? 0.78 : 1.06);
                    const outer = inner + value * radius * (pass === 0 ? 0.74 : 0.42);
                    context.globalAlpha = pass === 0 ? 0.34 + value * 0.66 : 0.20 + value * 0.62;
                    context.beginPath();
                    context.moveTo(cx + Math.cos(angle) * inner, cy + Math.sin(angle) * inner);
                    context.lineTo(cx + Math.cos(angle) * outer, cy + Math.sin(angle) * outer);
                    context.stroke();
                }
            }
            context.restore();

            // Frosted inner glass iris.
            context.save();
            context.fillStyle = state.isLight ? 'rgba(245, 248, 255, 0.56)' : 'rgba(8, 12, 20, 0.56)';
            context.beginPath();
            context.arc(cx, cy, radius * 0.74, 0, Math.PI * 2);
            context.fill();
            context.strokeStyle = css(state.palette.accent, 0.42);
            context.lineWidth = 1.5;
            context.stroke();

            // Inner circular oscilloscope waveform loop!
            const wfPoints = [];
            const wfSamples = 72;
            for (let i = 0; i < wfSamples; i++) {
                const angle = (i / wfSamples) * Math.PI * 2 + rotation * 0.6;
                let amp = 0;
                if (waveform) {
                    const idx = Math.floor((i / wfSamples) * (waveform.length - 1));
                    amp = (waveform[idx] - 128) / 128;
                } else {
                    const phase = state.reduced ? 0 : state.frame * 0.08;
                    amp = Math.sin(i * 0.35 + phase) * 0.18 + Math.cos(i * 0.7 - phase * 0.6) * 0.12;
                }
                const wfR = radius * (0.45 + clamp(amp, -0.45, 0.45) * 0.42);
                wfPoints.push([cx + Math.cos(angle) * wfR, cy + Math.sin(angle) * wfR]);
            }
            context.beginPath();
            smoothClosedLoop(context, wfPoints);
            context.strokeStyle = state.palette.hot;
            context.globalAlpha = 0.88;
            context.lineWidth = 1.75;
            context.stroke();

            // Precision Vernier tick ring.
            context.strokeStyle = state.palette.muted;
            context.globalAlpha = 0.24;
            context.lineWidth = 1;
            for (let i = 0; i < 60; i++) {
                const angle = (i / 60) * Math.PI * 2 + rotation * 0.3;
                const inner = radius * 1.18;
                const outer = inner + (i % 5 === 0 ? 6 : 2.5);
                context.beginPath();
                context.moveTo(cx + Math.cos(angle) * inner, cy + Math.sin(angle) * inner);
                context.lineTo(cx + Math.cos(angle) * outer, cy + Math.sin(angle) * outer);
                context.stroke();
            }
            context.restore();

            // Expanding beat shockwaves.
            context.save();
            context.globalCompositeOperation = blendOp();
            for (let i = 0; i < state.shockwaves.length; i++) {
                const sw = state.shockwaves[i];
                if (!sw.active) continue;
                const swR = radius * (1.05 + sw.progress * 1.15);
                context.globalAlpha = (1 - sw.progress) * 0.45 * sw.strength;
                context.strokeStyle = state.palette.hot;
                context.lineWidth = 2.2 - sw.progress * 1.4;
                context.beginPath();
                context.arc(cx, cy, swR, 0, Math.PI * 2);
                context.stroke();
            }
            context.restore();

            // Rotating outer conic energy ring & sparks.
            const ringRadius = radius * (1.62 + state.smoothEnergy * 0.12);
            context.save();
            if (typeof context.createConicGradient === 'function') {
                const ring = context.createConicGradient(rotation, cx, cy);
                ring.addColorStop(0, state.palette.accent);
                ring.addColorStop(0.35, state.palette.hot);
                ring.addColorStop(0.7, state.palette.accent3);
                ring.addColorStop(1, state.palette.accent);
                context.strokeStyle = ring;
                context.globalAlpha = 0.62;
                context.lineWidth = 2;
                context.beginPath();
                context.arc(cx, cy, ringRadius, 0, Math.PI * 2);
                context.stroke();
            } else {
                context.strokeStyle = state.palette.accent2;
                context.globalAlpha = 0.45;
                context.lineWidth = 2;
                context.beginPath();
                context.arc(cx, cy, ringRadius, 0, Math.PI * 2);
                context.stroke();
            }

            const sparks = 18;
            for (let i = 0; i < sparks; i++) {
                const dir = rotation * (1.35 + (i % 3) * 0.18) + (i / sparks) * Math.PI * 2;
                const level = clamp(state.levels[(i * 3) % count], 0, 1);
                const orbit = radius * (1.28 + (i % 4) * 0.12 + level * 0.24);
                const x = cx + Math.cos(dir) * orbit;
                const y = cy + Math.sin(dir) * orbit;
                if (!state.reduced) {
                    context.globalAlpha = 0.20 + level * 0.28;
                    context.strokeStyle = state.palette.accent2;
                    context.lineWidth = 1.6;
                    context.beginPath();
                    context.arc(cx, cy, orbit, dir - 0.24, dir);
                    context.stroke();
                }
                context.globalAlpha = 0.32 + level * 0.68;
                context.fillStyle = i % 2 === 0 ? state.palette.hot : state.palette.accent2;
                context.beginPath();
                context.arc(x, y, i % 3 === 0 ? 2.5 : 1.6, 0, Math.PI * 2);
                context.fill();
            }
            context.restore();
        }

        function wavePoint(data, i, samples, phaseOffset, freqBias) {
            const u = i / samples;
            const windowGain = Math.pow(Math.sin(Math.PI * u), 0.55);
            if (!data) {
                const drift = state.reduced ? 0.6 : state.frame / 26;
                const v = Math.sin(u * Math.PI * (4 + freqBias) + drift + phaseOffset) * 0.18
                    + Math.sin(u * Math.PI * (9 - freqBias) - drift * 0.65) * 0.08
                    + Math.cos(u * Math.PI * 15 + drift * 1.2) * 0.04;
                return v * windowGain;
            }
            const index = Math.floor(u * (data.length - 1));
            const left = data[Math.max(0, index - 2)];
            const right = data[Math.min(data.length - 1, index + 2)];
            const raw = ((data[index] * 2 + left + right) / 4 - 128) / 128;
            const shimmer = state.reduced ? 0 : Math.sin(u * Math.PI * 6 + state.frame * 0.06 + phaseOffset) * (0.04 * freqBias);
            return clamp((raw + shimmer) * windowGain, -1, 1);
        }

        // Mode 3: Aurora Silk — 3 layered chromatic silk curtains, glowing triple-pass traces & glossy reflection.
        function drawWave() {
            const width = state.width;
            const height = state.height;
            const cache = ensureCache();
            const data = readWaveform();
            const middle = height * 0.49;
            const samples = Math.min(260, Math.max(120, Math.round(width / 4)));
            const marginX = clamp(width * 0.024, 6, 20);
            const spanW = Math.max(40, width - marginX * 2);

            // Soft horizon calibration hairline.
            context.save();
            context.globalAlpha = state.isLight ? 0.16 : 0.24;
            context.fillStyle = state.palette.accent;
            context.fillRect(marginX, Math.round(middle), spanW, 1);
            context.restore();

            // Spectral mist silhouette behind the ribbons.
            context.save();
            const silhouette = [];
            for (let i = 0; i < state.levels.length; i++) {
                const x = marginX + (i / (state.levels.length - 1)) * spanW;
                const y = middle - state.levels[i] * height * 0.28;
                silhouette.push([x, y]);
            }
            if (silhouette.length > 2) {
                context.beginPath();
                context.moveTo(marginX, middle);
                smoothTrace(context, silhouette);
                context.lineTo(marginX + spanW, middle);
                context.closePath();
                context.globalAlpha = 0.22 + state.smoothEnergy * 0.16;
                context.fillStyle = cache.waveSilkGradient;
                context.fill();
            }
            context.restore();

            // Layer 1 & 2: Harmonized Silk Ribbons.
            const ribbons = [
                { phase: 2.1, amp: height * 0.20, shiftY: -height * 0.015, bias: 1.4, color: state.palette.accent3, alpha: 0.36, lw: 1.8 },
                { phase: 1.05, amp: height * 0.24, shiftY: height * 0.02, bias: 0.7, color: state.palette.accent2, alpha: 0.52, lw: 2.2 }
            ];
            for (let r = 0; r < ribbons.length; r++) {
                const cfg = ribbons[r];
                const pts = [];
                for (let i = 0; i <= samples; i += 2) {
                    const x = marginX + (i / samples) * spanW;
                    const y = middle + cfg.shiftY + wavePoint(data, i, samples, cfg.phase, cfg.bias) * cfg.amp;
                    pts.push([x, y]);
                }
                context.save();
                context.beginPath();
                smoothTrace(context, pts);
                context.strokeStyle = cfg.color;
                context.globalAlpha = cfg.alpha * 0.35;
                context.lineWidth = cfg.lw * 3.5;
                context.stroke();
                context.globalAlpha = cfg.alpha;
                context.lineWidth = cfg.lw;
                context.stroke();
                context.restore();
            }

            // Layer 3: Primary Prismatic Ribbon + Silk Curtain + Inverted Reflection.
            const points = [];
            const reflection = [];
            const peakSparks = [];
            for (let i = 0; i <= samples; i++) {
                const x = marginX + (i / samples) * spanW;
                const dy = wavePoint(data, i, samples, 0, 0) * height * 0.31;
                const y = middle + dy;
                points.push([x, y]);
                if (i % 2 === 0) reflection.push([x, middle + height * 0.10 - dy * 0.42]);
                if (i > 4 && i < samples - 4 && i % 18 === 0 && Math.abs(dy) > height * 0.09) {
                    peakSparks.push([x, y, clamp(Math.abs(dy) / (height * 0.31), 0.2, 1)]);
                }
            }

            context.save();
            // Translucent silk curtain fill down toward the floor.
            context.beginPath();
            smoothTrace(context, points);
            context.lineTo(marginX + spanW, middle + height * 0.28);
            context.lineTo(marginX, middle + height * 0.28);
            context.closePath();
            context.fillStyle = cache.waveSilkGradient;
            context.globalAlpha = state.isLight ? 0.55 : 0.72;
            context.fill();

            // Inverted liquid glass echo below baseline.
            if (!state.reduced && reflection.length > 2) {
                context.beginPath();
                smoothTrace(context, reflection);
                context.strokeStyle = cache.waveGradient;
                context.globalAlpha = state.isLight ? 0.14 : 0.20;
                context.lineWidth = 1.4;
                context.stroke();
            }

            // Triple-pass glowing neon ribbon trace.
            context.lineJoin = 'round';
            context.lineCap = 'round';
            context.globalCompositeOperation = blendOp();

            context.beginPath();
            smoothTrace(context, points);
            context.strokeStyle = cache.waveGradient;
            context.globalAlpha = state.isLight ? 0.18 : 0.26;
            context.lineWidth = Math.max(9, height * 0.028);
            context.stroke();

            context.beginPath();
            smoothTrace(context, points);
            context.globalAlpha = 0.88;
            context.lineWidth = Math.max(2.6, height * 0.009);
            context.stroke();

            context.beginPath();
            smoothTrace(context, points);
            context.strokeStyle = state.isLight ? state.palette.hot : '#ffffff';
            context.globalAlpha = state.isLight ? 0.45 : 0.65;
            context.lineWidth = 1.25;
            context.stroke();
            context.restore();

            // Treble crest shimmer beads.
            if (!state.reduced && peakSparks.length) {
                context.save();
                context.globalCompositeOperation = blendOp();
                for (let i = 0; i < peakSparks.length; i++) {
                    const [sx, sy, power] = peakSparks[i];
                    context.globalAlpha = (0.35 + power * 0.55) * (0.6 + state.smoothTreble * 0.6);
                    context.fillStyle = state.palette.hot;
                    context.beginPath();
                    context.arc(sx, sy, 2 + power * 2, 0, Math.PI * 2);
                    context.fill();
                }
                context.restore();
            }
        }

        // Mode 4: Nebula Pulse — 3D flying starfield, harmonic constellation filaments & volumetric clouds.
        function drawNebula() {
            const width = state.width;
            const height = state.height;
            const cx = width * 0.5;
            const cy = height * 0.415;
            const scale = Math.min(width, height) * 0.54;
            const driftT = state.reduced ? 0 : state.frame * 0.012;
            const energyBoost = state.reduced ? 0.18 : clamp(state.smoothEnergy + state.beat * 0.45, 0.08, 1.3);

            // Drifting twin volumetric nebula gas clouds.
            context.save();
            context.globalCompositeOperation = blendOp();
            const clouds = [
                { ox: Math.cos(driftT * 0.45) * width * 0.12, oy: Math.sin(driftT * 0.35) * height * 0.07, color: state.palette.accent3, r: scale * (0.72 + state.smoothBass * 0.18) },
                { ox: -Math.cos(driftT * 0.38) * width * 0.10, oy: Math.cos(driftT * 0.5) * height * 0.06, color: state.palette.accent2, r: scale * (0.64 + state.smoothMid * 0.16) }
            ];
            for (let c = 0; c < clouds.length; c++) {
                const cl = clouds[c];
                const grad = context.createRadialGradient(cx + cl.ox, cy + cl.oy, 4, cx + cl.ox, cy + cl.oy, cl.r);
                grad.addColorStop(0, css(cl.color, state.isLight ? 0.18 : 0.34));
                grad.addColorStop(0.55, css(state.palette.accent, state.isLight ? 0.08 : 0.14));
                grad.addColorStop(1, 'rgba(0,0,0,0)');
                context.fillStyle = grad;
                context.beginPath();
                context.arc(cx + cl.ox, cy + cl.oy, cl.r, 0, Math.PI * 2);
                context.fill();
            }
            context.restore();

            // Update 3D particles & collect foreground nodes for constellations.
            const speed = state.reduced ? 0 : 0.0022 + state.smoothEnergy * 0.010 + state.beat * 0.012;
            const swirl = state.reduced ? 0 : state.rotation * 0.42;
            const cosS = Math.cos(swirl);
            const sinS = Math.sin(swirl);
            const fg = [];
            const projected = [];

            for (let i = 0; i < state.nebulaStars.length; i++) {
                const star = state.nebulaStars[i];
                star.prevZ = star.z;
                if (!state.reduced) {
                    star.z -= speed * (0.75 + (i % 5) * 0.12);
                    if (star.z <= 0.07) {
                        star.z = 1.0;
                        star.prevZ = 1.0;
                    }
                }
                const rx = star.x * cosS - star.y * sinS;
                const ry = star.x * sinS + star.y * cosS;
                const sx = cx + (rx / star.z) * scale;
                const sy = cy + (ry / star.z) * scale;
                const px = cx + (rx / star.prevZ) * scale;
                const py = cy + (ry / star.prevZ) * scale;
                if (sx < -30 || sx > width + 30 || sy < -30 || sy > height + 30) continue;
                const depth = 1 - star.z;
                const lvl = clamp(state.levels[star.band], 0, 1);
                const node = { sx, sy, px, py, depth, lvl, star };
                projected.push(node);
                if (star.z < 0.68 && fg.length < 40) fg.push(node);
            }

            // Dynamic Constellation Network among foreground energy nodes.
            const maxDist = clamp(Math.min(width, height) * 0.31, 82, 195);
            const maxDist2 = maxDist * maxDist;
            context.save();
            context.globalCompositeOperation = blendOp();
            context.lineWidth = 1.15;
            for (let i = 0; i < fg.length; i++) {
                const a = fg[i];
                for (let j = i + 1; j < fg.length; j++) {
                    const b = fg[j];
                    const dx = b.sx - a.sx;
                    const dy = b.sy - a.sy;
                    const d2 = dx * dx + dy * dy;
                    if (d2 > maxDist2) continue;
                    const prox = 1 - Math.sqrt(d2) / maxDist;
                    const energy = clamp((a.lvl + b.lvl) * 0.5 + state.smoothMid * 0.5, 0.15, 1);
                    context.globalAlpha = prox * energy * (state.isLight ? 0.34 : 0.44);
                    context.strokeStyle = a.star.tone === 0 ? state.palette.accent : state.palette.accent2;
                    context.beginPath();
                    context.moveTo(a.sx, a.sy);
                    context.lineTo(b.sx, b.sy);
                    context.stroke();
                }
            }

            // Draw warp streaks, luminous bokeh halos & crisp cores.
            for (let i = 0; i < projected.length; i++) {
                const n = projected[i];
                const tone = n.star.tone === 0 ? state.palette.accent : (n.star.tone === 1 ? state.palette.accent2 : state.palette.hot);
                const radius = clamp(n.star.size * (0.45 + n.depth * 1.45 + n.lvl * 1.3), 0.7, 5.2);
                const alpha = clamp((0.18 + n.depth * 0.56 + n.lvl * 0.34) * (0.65 + energyBoost * 0.35), 0.08, 0.95);

                if (!state.reduced && n.depth > 0.28) {
                    context.globalAlpha = alpha * 0.42;
                    context.strokeStyle = tone;
                    context.lineWidth = Math.max(0.8, radius * 0.55);
                    context.beginPath();
                    context.moveTo(n.px, n.py);
                    context.lineTo(n.sx, n.sy);
                    context.stroke();
                }

                if (radius > 1.8) {
                    context.globalAlpha = alpha * 0.24;
                    context.fillStyle = tone;
                    context.beginPath();
                    context.arc(n.sx, n.sy, radius * 2.4, 0, Math.PI * 2);
                    context.fill();
                }

                context.globalAlpha = alpha;
                context.fillStyle = n.lvl > 0.55 ? state.palette.hot : tone;
                context.beginPath();
                context.arc(n.sx, n.sy, radius, 0, Math.PI * 2);
                context.fill();
            }

            // Expanding beat shockwaves & central resonance ring.
            const portalR = Math.min(width, height) * (0.11 + state.smoothBass * 0.05 + state.beat * 0.03);
            context.globalAlpha = 0.28 + state.smoothBass * 0.32;
            context.strokeStyle = state.palette.hot;
            context.lineWidth = 2;
            context.beginPath();
            context.arc(cx, cy, portalR, 0, Math.PI * 2);
            context.stroke();

            for (let i = 0; i < state.shockwaves.length; i++) {
                const sw = state.shockwaves[i];
                if (!sw.active) continue;
                const swR = portalR + sw.progress * Math.min(width, height) * 0.36;
                context.globalAlpha = (1 - sw.progress) * 0.40 * sw.strength;
                context.strokeStyle = state.palette.accent2;
                context.lineWidth = 2 - sw.progress * 1.2;
                context.beginPath();
                context.arc(cx, cy, swR, 0, Math.PI * 2);
                context.stroke();
            }
            context.restore();
        }

        // Mode 5: Cyber Horizon — Segmented holographic sun, 3D neon grid & 12-layer frequency mountain ridges.
        function drawHorizon() {
            const width = state.width;
            const height = state.height;
            const cache = ensureCache();
            const hy = height * 0.43;
            const cx = width * 0.5;
            const bottomY = height * 0.90;
            const sunCy = cache.sunCy;
            const sunR = cache.sunR * (1 + (state.reduced ? 0 : state.smoothBass * 0.06 + state.beat * 0.04));

            // 1. Twinkling Treble Starfield above horizon.
            context.save();
            for (let i = 0; i < state.skyStars.length; i++) {
                const s = state.skyStars[i];
                const twinkle = state.reduced ? 0.65 : 0.38 + 0.62 * Math.abs(Math.sin(state.frame * 0.03 + s.phase));
                context.globalAlpha = twinkle * (0.28 + state.smoothTreble * 0.65);
                context.fillStyle = i % 3 === 0 ? state.palette.hot : (state.isLight ? state.palette.accent3 : '#ffffff');
                context.beginPath();
                context.arc(s.x * width, s.y * height, s.size, 0, Math.PI * 2);
                context.fill();
            }
            context.restore();

            // 2. Luminous Synth Solar Disc with horizon scanline slices.
            context.save();
            const sunHalo = context.createRadialGradient(cx, sunCy, sunR * 0.4, cx, sunCy, sunR * 1.85);
            sunHalo.addColorStop(0, css(state.palette.hot, state.isLight ? 0.24 : 0.44));
            sunHalo.addColorStop(0.5, css(state.palette.accent2, state.isLight ? 0.12 : 0.22));
            sunHalo.addColorStop(1, 'rgba(0,0,0,0)');
            context.fillStyle = sunHalo;
            context.beginPath();
            context.arc(cx, sunCy, sunR * 1.85, 0, Math.PI * 2);
            context.fill();

            // Clip to the sky region above horizon and the circular sun disc, then draw horizontal bands skipping scanline slits.
            context.beginPath();
            context.rect(0, 0, width, hy + 2);
            context.clip();

            context.beginPath();
            context.arc(cx, sunCy, sunR, 0, Math.PI * 2);
            context.clip();

            context.globalAlpha = state.isLight ? 0.84 : 0.94;
            context.fillStyle = cache.sunGradient;
            const sunTop = sunCy - sunR;
            const sunBottom = Math.min(hy + 2, sunCy + sunR);
            let cursorY = sunTop;
            for (let slice = 0; slice < 5; slice++) {
                const sliceY = sunCy + sunR * (-0.08 + slice * 0.16);
                const sliceH = Math.max(2, sunR * (0.032 + slice * 0.015));
                if (sliceY > cursorY) {
                    const segBottom = Math.min(sliceY, sunBottom);
                    if (segBottom > cursorY) {
                        context.fillRect(cx - sunR - 1, cursorY, sunR * 2 + 2, segBottom - cursorY);
                    }
                }
                cursorY = sliceY + sliceH;
                if (cursorY >= sunBottom) break;
            }
            if (cursorY < sunBottom) {
                context.fillRect(cx - sunR - 1, cursorY, sunR * 2 + 2, sunBottom - cursorY);
            }
            context.restore();

            // 3. Approaching 3D Perspective Neon Floor Grid below horizon.
            context.save();
            const floorDepth = Math.max(40, height - hy);
            const gridColor = state.palette.accent;
            context.strokeStyle = gridColor;
            context.lineWidth = 1;

            // Perspective converging rays.
            const rays = 14;
            for (let r = -rays; r <= rays; r++) {
                const norm = r / rays;
                const bx = cx + norm * width * 1.45;
                const tx = cx + norm * width * 0.14;
                context.globalAlpha = (state.isLight ? 0.14 : 0.22) * (1 - Math.abs(norm) * 0.45);
                context.beginPath();
                context.moveTo(tx, hy);
                context.lineTo(bx, height);
                context.stroke();
            }

            // Approaching horizontal grid scanlines.
            const phase = state.reduced ? 0.4 : (state.horizonPhase % 1);
            for (let row = 0; row < 9; row++) {
                const t = (row + phase) / 9;
                const y = hy + Math.pow(t, 2.05) * floorDepth;
                context.globalAlpha = (0.08 + t * (state.isLight ? 0.18 : 0.28));
                context.beginPath();
                context.moveTo(0, y);
                context.lineTo(width, y);
                context.stroke();
            }

            // Horizon seam glow.
            context.globalAlpha = 0.72;
            context.strokeStyle = state.palette.hot;
            context.lineWidth = 1.6;
            context.beginPath();
            context.moveTo(0, hy);
            context.lineTo(width, hy);
            context.stroke();
            context.restore();

            // 4. 12-Layer 3D Cascading Perspective Frequency Mountain Ridges (back-to-front).
            const sampleStep = 2;
            const ridgeMargin = Math.max(12, width * 0.04);
            for (let layer = HORIZON_LAYERS - 1; layer >= 0; layer--) {
                const depthRatio = layer / (HORIZON_LAYERS - 1); // 1 = far back near horizon, 0 = front
                const nearness = 1 - depthRatio;
                const row = state.horizonHistory[layer];
                const baseY = lerp(hy + 6, bottomY, Math.pow(nearness, 1.32));
                const spreadHalf = lerp(width * 0.28, width * 0.5 - ridgeMargin, Math.pow(nearness, 0.75));
                const amp = height * lerp(0.11, 0.25, nearness);
                const pts = [];

                for (let i = 0; i < BAR_COUNT; i += sampleStep) {
                    const u = (i / (BAR_COUNT - 1)) * 2 - 1; // -1..1
                    const x = cx + u * spreadHalf;
                    // Arch slightly toward edges & let mid/high energy sculpt sharp peaks.
                    const envelope = 0.35 + 0.65 * Math.cos(u * Math.PI * 0.36);
                    const lvl = clamp(row[i], 0, 1);
                    const y = baseY - lvl * amp * envelope;
                    pts.push([x, y]);
                }
                if (pts.length < 3) continue;

                // Occlusion body fill + bright neon ridge stroke.
                context.save();
                context.beginPath();
                context.moveTo(pts[0][0], baseY + 14);
                context.lineTo(pts[0][0], pts[0][1]);
                smoothTrace(context, pts);
                context.lineTo(pts[pts.length - 1][0], baseY + 14);
                context.closePath();

                context.fillStyle = state.isLight
                    ? css(state.palette.deep, lerp(0.16, 0.42, nearness))
                    : `rgba(7, 11, 18, ${lerp(0.62, 0.86, nearness).toFixed(2)})`;
                context.fill();

                const crestColor = layer === 0
                    ? state.palette.hot
                    : layer < 4
                        ? state.palette.accent2
                        : layer < 8
                            ? state.palette.accent
                            : state.palette.accent3;

                // Additive halo on foreground ridges.
                context.beginPath();
                smoothTrace(context, pts);
                if (!state.reduced && nearness > 0.55) {
                    context.globalCompositeOperation = blendOp();
                    context.strokeStyle = crestColor;
                    context.globalAlpha = nearness * 0.26;
                    context.lineWidth = 6;
                    context.stroke();
                }
                context.globalCompositeOperation = 'source-over';
                context.strokeStyle = crestColor;
                context.globalAlpha = lerp(0.34, 0.96, nearness);
                context.lineWidth = lerp(1.1, 2.2, nearness);
                context.stroke();
                context.restore();
            }
        }

        function updateMotionPools() {
            if (state.reduced) return;
            state.rotation += 0.0028 + state.smoothBass * 0.009 + state.beat * 0.006;
            state.horizonPhase += 0.018 + state.smoothEnergy * 0.045;
            if (state.transition < 1) {
                state.transition = Math.min(1, state.transition + 0.08);
            }
            for (let i = 0; i < state.shockwaves.length; i++) {
                const sw = state.shockwaves[i];
                if (!sw.active) continue;
                sw.progress += 0.026 + state.smoothBass * 0.014;
                if (sw.progress >= 1) {
                    sw.active = false;
                    sw.progress = 1;
                }
            }
        }

        function renderCurrentMode() {
            context.clearRect(0, 0, state.width, state.height);
            drawAtmosphere();
            drawMotes();
            const alpha = state.reduced ? 1 : clamp(state.transition, 0.15, 1);
            if (alpha < 0.999) {
                context.save();
                context.globalAlpha = alpha;
            }
            if (state.mode === 'radial') drawRadial();
            else if (state.mode === 'wave') drawWave();
            else if (state.mode === 'nebula') drawNebula();
            else if (state.mode === 'horizon') drawHorizon();
            else drawBars();
            if (alpha < 0.999) {
                context.restore();
            }
        }

        function frame() {
            if (state.destroyed) return;
            if (!state.visible) {
                state.handle = window.requestAnimationFrame(frame);
                return;
            }
            state.frame += 1;
            updateMotionPools();
            const active = readLevels();
            if (active) state.playing = true;
            renderCurrentMode();
            publishEnergy();
            state.handle = window.requestAnimationFrame(frame);
        }

        function start() {
            if (state.destroyed || state.handle) return;
            resize();
            state.handle = window.requestAnimationFrame(frame);
        }

        function stop() {
            if (state.handle) window.cancelAnimationFrame(state.handle);
            state.handle = 0;
        }

        const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(() => queueResize()) : null;
        if (observer) observer.observe(canvas);
        window.addEventListener('resize', queueResize);

        const visibilityObserver = (typeof IntersectionObserver === 'function' && canvas)
            ? new IntersectionObserver(entries => {
                for (const entry of entries) {
                    state.visible = entry.isIntersecting !== false;
                }
            }, { threshold: 0.02 })
            : null;
        if (visibilityObserver) visibilityObserver.observe(canvas);

        function onVisibility() {
            if (document.hidden) {
                for (let i = 0; i < state.peaks.length; i++) state.peaks[i] *= 0.9;
            }
        }

        document.addEventListener('visibilitychange', onVisibility);

        resize();
        start();

        return {
            mode() { return state.mode; },
            setMode(mode) {
                if (MODES.indexOf(mode) < 0) return;
                if (state.mode !== mode) {
                    state.mode = mode;
                    state.transition = state.reduced ? 1 : 0.15;
                    state.cacheKey = '';
                    if (state.width > 1 && state.height > 1) renderCurrentMode();
                }
            },
            cycleMode() {
                const nextMode = MODES[(MODES.indexOf(state.mode) + 1) % MODES.length];
                state.mode = nextMode;
                state.transition = state.reduced ? 1 : 0.15;
                state.cacheKey = '';
                if (state.width > 1 && state.height > 1) renderCurrentMode();
                return state.mode;
            },
            setPalette(palette) {
                if (!palette) return;
                state.palette = Object.assign({}, state.palette, palette);
                state.cacheKey = '';
            },
            setCoverColor(r, g, b) {
                state.coverLocked = true;
                state.isLight = detectLightScheme(opts.root || canvas);
                state.palette = buildHarmonizedPalette(r, g, b, state.palette.muted, state.isLight);
                state.cacheKey = '';
            },
            clearCoverColor() {
                state.coverLocked = false;
                state.isLight = detectLightScheme(opts.root || canvas);
                state.palette = readPalette(opts.root || canvas);
                state.cacheKey = '';
            },
            refreshPalette() {
                state.isLight = detectLightScheme(opts.root || canvas);
                if (state.coverLocked) {
                    state.palette = buildHarmonizedPalette(state.palette.r, state.palette.g, state.palette.b, state.palette.muted, state.isLight);
                } else {
                    state.palette = readPalette(opts.root || canvas);
                }
                state.cacheKey = '';
            },
            resize,
            stop,
            start,
            destroy() {
                state.destroyed = true;
                stop();
                if (observer) observer.disconnect();
                if (visibilityObserver) visibilityObserver.disconnect();
                if (motionObserver) motionObserver.disconnect();
                window.removeEventListener('resize', queueResize);
                document.removeEventListener('visibilitychange', onVisibility);
                if (mediaQuery && typeof mediaQuery.removeEventListener === 'function') {
                    mediaQuery.removeEventListener('change', onMotionChange);
                } else if (mediaQuery && typeof mediaQuery.removeListener === 'function') {
                    mediaQuery.removeListener(onMotionChange);
                }
            }
        };
    }

    const api = Object.freeze({ create, MODES });
    if (typeof rootScope !== 'undefined' && rootScope) {
        rootScope.AudioPlayerVisualizer = api;
    }
    if (typeof module !== 'undefined' && module.exports) {
        module.exports = api;
    }
})(typeof window !== 'undefined' ? window : globalThis);
