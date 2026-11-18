(function () {
    'use strict';

    // Canvas spectrum for the desktop music player.
    // The player owns the audio graph. This module only reads analyser frames
    // and paints its own canvas. Optional callbacks report loudness so the
    // shell can breathe with the music; they never receive audio samples.

    const MODES = ['bars', 'radial', 'wave'];
    const BAR_COUNT = 64;
    const PEAK_DECAY = 0.972;

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

    function shiftHue(r, g, b, degrees) {
        const hsl = rgbToHsl(r, g, b);
        if (hsl.s === 0) return `rgb(${Math.round(hsl.l * 255)},${Math.round(hsl.l * 255)},${Math.round(hsl.l * 255)})`;
        const h = (hsl.h + degrees / 360) % 1;
        const q = hsl.l < 0.5 ? hsl.l * (1 + hsl.s) : hsl.l + hsl.s - hsl.l * hsl.s;
        const p = 2 * hsl.l - q;
        return `rgb(${Math.round(hueToRgb(p, q, h + 1 / 3) * 255)},${Math.round(hueToRgb(p, q, h) * 255)},${Math.round(hueToRgb(p, q, h - 1 / 3) * 255)})`;
    }

    function css(rgb, alpha) {
        if (alpha == null || alpha >= 1) return rgb;
        // `rgb(r,g,b)` -> `rgba(r,g,b,a)` without reparsing the theme.
        if (rgb.charCodeAt(3) === 40) {
            return 'rgba(' + rgb.slice(4, -1) + ',' + alpha.toFixed(3) + ')';
        }
        return rgb;
    }

    function readPalette(root) {
        const styles = getComputedStyle(root || document.documentElement);
        const read = (name, fallback) => (styles.getPropertyValue(name) || '').trim() || fallback;
        const r = parseInt(read('--vd-accent-r', '39'), 10);
        const g = parseInt(read('--vd-accent-g', '199'), 10);
        const b = parseInt(read('--vd-accent-b', '166'), 10);
        const red = Number.isFinite(r) ? r : 39;
        const green = Number.isFinite(g) ? g : 199;
        const blue = Number.isFinite(b) ? b : 166;
        return {
            accent: read('--vd-accent', '#27c7a6'),
            accent2: shiftHue(red, green, blue, 42),
            muted: read('--vd-muted', '#a8bbd0'),
            r: red,
            g: green,
            b: blue
        };
    }

    function roundedBar(context, x, y, width, height, radius) {
        const r = Math.min(radius, width / 2, Math.max(0, height) / 2);
        context.beginPath();
        if (typeof context.roundRect === 'function') {
            context.roundRect(x, y, width, Math.max(0, height), r);
            return;
        }
        context.moveTo(x + r, y);
        context.lineTo(x + width - r, y);
        context.quadraticCurveTo(x + width, y, x + width, y + r);
        context.lineTo(x + width, y + height - r);
        context.quadraticCurveTo(x + width, y + height, x + width - r, y + height);
        context.lineTo(x + r, y + height);
        context.quadraticCurveTo(x, y + height, x, y + height - r);
        context.lineTo(x, y + r);
        context.quadraticCurveTo(x, y, x + r, y);
        context.closePath();
    }

    // Deterministic per-index phase so the idle stage feels organic but stable.
    function hashPhase(index) {
        let h = (index + 1) * 2654435761;
        h ^= h >>> 13;
        h = Math.imul(h, 2246822519);
        h ^= h >>> 16;
        return (h >>> 0) / 4294967295 * Math.PI * 2;
    }

    function create(canvas, options) {
        if (!canvas || typeof canvas.getContext !== 'function') return null;
        const context = canvas.getContext('2d');
        if (!context) return null;
        const opts = options || {};
        const mediaQuery = typeof matchMedia === 'function' ? matchMedia('(prefers-reduced-motion: reduce)') : null;

        const state = {
            mode: MODES.indexOf(opts.mode) >= 0 ? opts.mode : 'bars',
            playing: false,
            palette: opts.palette || readPalette(opts.root || canvas),
            coverLocked: false,
            levels: new Float32Array(BAR_COUNT),
            peaks: new Float32Array(BAR_COUNT),
            raw: new Float32Array(BAR_COUNT),
            frequency: null,
            waveform: null,
            width: 0,
            height: 0,
            ratio: 1,
            frame: 0,
            rotation: 0,
            bass: 0,
            energy: 0,
            smoothBass: 0,
            smoothEnergy: 0,
            sentBass: -1,
            reduced: !!(mediaQuery && mediaQuery.matches),
            destroyed: false,
            visible: true,
            cacheKey: '',
            cache: {},
            motes: [],
            handle: 0,
            resizeQueued: false
        };

        // A few slow dust motes give the stage depth without stealing focus.
        function seedMotes() {
            const count = state.reduced ? 0 : 34;
            state.motes = [];
            for (let i = 0; i < count; i++) {
                state.motes.push({
                    x: ((i * 0.61803398875) % 1),
                    y: ((i * 0.38196601125) % 1),
                    size: 0.8 + ((i * 37) % 10) / 10 * 1.4,
                    speed: 0.00012 + ((i * 13) % 10) / 10 * 0.00035,
                    drift: hashPhase(i),
                    depth: (i % 5) / 5
                });
            }
        }

        seedMotes();

        function onMotionChange() {
            state.reduced = !!(mediaQuery && mediaQuery.matches);
            seedMotes();
            state.cacheKey = '';
        }

        if (mediaQuery && typeof mediaQuery.addEventListener === 'function') {
            mediaQuery.addEventListener('change', onMotionChange);
        } else if (mediaQuery && typeof mediaQuery.addListener === 'function') {
            mediaQuery.addListener(onMotionChange);
        }

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
            // Gentle low-pass so the shell glow breathes instead of flickering.
            state.smoothBass = state.smoothBass * 0.82 + state.bass * 0.18;
            state.smoothEnergy = state.smoothEnergy * 0.86 + state.energy * 0.14;
            const out = clamp(state.smoothBass * 0.78 + state.smoothEnergy * 0.3, 0, 1);
            if (Math.abs(out - state.sentBass) < 0.012 && state.frame % 8 !== 0) return;
            state.sentBass = out;
            opts.onEnergy(out, state.smoothEnergy);
        }

        // Log-shaped bins, fast attack, slow release. Silence keeps a quiet breath
        // so the stage never collapses to an empty rectangle.
        function stageShape(index, count) {
            const across = count <= 1 ? 1 : Math.sin(Math.PI * index / (count - 1));
            return 0.42 + 0.58 * across;
        }

        function idleLevel(index, count) {
            if (state.reduced) return stageShape(index, count) * 0.34;
            const t = state.frame / 42;
            const phase = hashPhase(index);
            const ripple = 0.55 + 0.45 * Math.sin(t * 1.7 + phase);
            const slow = 0.75 + 0.25 * Math.sin(t * 0.6 + index * 0.12 + phase * 0.3);
            return clamp(stageShape(index, count) * ripple * slow * 0.62, 0, 1);
        }

        function readLevels() {
            const bars = state.levels;
            const peaks = state.peaks;
            const node = analyser();
            if (!node) {
                let energy = 0;
                for (let i = 0; i < bars.length; i++) {
                    const idle = idleLevel(i, bars.length);
                    bars[i] = bars[i] * 0.78 + idle * 0.22;
                    peaks[i] = Math.max(bars[i], peaks[i] * PEAK_DECAY);
                    energy += idle;
                }
                state.energy = energy / bars.length;
                state.bass = state.bass * 0.86 + 0.16 * 0.14;
                return false;
            }
            const size = node.frequencyBinCount || 1024;
            if (!state.frequency || state.frequency.length !== size) state.frequency = new Uint8Array(size);
            node.getByteFrequencyData(state.frequency);
            const bins = state.frequency;
            const usable = Math.max(8, Math.floor(size * 0.62));
            let energy = 0;
            let bass = 0;
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
                const average = (sum / span * 0.72 + peakBin * 0.28) / 255;
                const value = clamp(Math.pow(average, 1.08) * 1.15, 0, 1);
                energy += value;
                if (i < 8) bass += value;
                raw[i] = value;
            }
            const mean = energy / bars.length;
            const quiet = mean < 0.045;
            for (let i = 0; i < bars.length; i++) {
                const target = quiet ? idleLevel(i, bars.length) : raw[i];
                const previous = bars[i];
                bars[i] = target > previous
                    ? previous * 0.28 + target * 0.72
                    : previous * 0.84 + target * 0.16;
                peaks[i] = Math.max(bars[i], peaks[i] * PEAK_DECAY);
            }
            state.energy = mean;
            state.bass = state.bass * 0.62 + (bass / 8) * 0.38;
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
            return [p.accent, p.accent2, p.muted, state.width, state.height, state.mode].join('|');
        }

        // Gradients are rebuilt only when the palette, size or mode changes.
        // Allocating them per frame caused steady GC churn on long playlists.
        function ensureCache() {
            const key = paletteKey();
            if (key === state.cacheKey && state.cache.ready) return state.cache;
            const width = Math.max(1, state.width);
            const height = Math.max(1, state.height);
            const p = state.palette;
            const baseline = height * 0.78;
            const usable = height * 0.62;
            const barGradient = context.createLinearGradient(0, baseline - usable, 0, baseline);
            barGradient.addColorStop(0, p.accent2);
            barGradient.addColorStop(0.45, p.accent);
            barGradient.addColorStop(1, css(p.accent, 0.82));
            const mirrorGradient = context.createLinearGradient(0, baseline, 0, Math.min(height, baseline + usable * 0.5));
            mirrorGradient.addColorStop(0, css(p.accent, 0.32));
            mirrorGradient.addColorStop(1, css(p.accent, 0));
            const waveGradient = context.createLinearGradient(0, 0, width, 0);
            waveGradient.addColorStop(0, css(p.accent, 0.9));
            waveGradient.addColorStop(0.5, p.accent2);
            waveGradient.addColorStop(1, css(p.accent, 0.9));
            const washGradient = context.createLinearGradient(0, height * 0.5, 0, height);
            washGradient.addColorStop(0, css(p.accent2, 0.28));
            washGradient.addColorStop(1, css(p.accent2, 0));
            state.cache = { ready: true, barGradient, mirrorGradient, waveGradient, washGradient, baseline, usable };
            state.cacheKey = key;
            return state.cache;
        }

        function drawAtmosphere() {
            const width = state.width;
            const height = state.height;
            const lift = state.reduced ? 0 : clamp(state.smoothBass * 0.8 + state.smoothEnergy * 0.4, 0, 1);
            const cx = width * 0.5;
            const cy = height * 0.42;
            const radius = Math.min(width, height) * (0.36 + lift * 0.09);
            // Warm core glow.
            const glow = context.createRadialGradient(cx, cy, radius * 0.06, cx, cy, radius);
            glow.addColorStop(0, css(state.palette.accent2, 0.55));
            glow.addColorStop(0.45, css(state.palette.accent, 0.38));
            glow.addColorStop(1, 'rgba(0,0,0,0)');
            context.globalAlpha = 0.22 + lift * 0.24;
            context.fillStyle = glow;
            context.beginPath();
            context.arc(cx, cy, radius, 0, Math.PI * 2);
            context.fill();
            // Cool wide halo keeps large stages from looking flat.
            const halo = context.createRadialGradient(cx, cy, radius * 0.4, cx, cy, Math.max(width, height) * 0.62);
            halo.addColorStop(0, css(state.palette.accent, 0.10));
            halo.addColorStop(1, 'rgba(0,0,0,0)');
            context.globalAlpha = 0.5;
            context.fillStyle = halo;
            context.fillRect(0, 0, width, height);
            context.globalAlpha = 1;
            // Gentle top vignette for text legibility.
            const shade = context.createLinearGradient(0, 0, 0, height * 0.4);
            shade.addColorStop(0, 'rgba(0,0,0,0.22)');
            shade.addColorStop(1, 'rgba(0,0,0,0)');
            context.fillStyle = shade;
            context.fillRect(0, 0, width, Math.ceil(height * 0.4));
        }

        function drawMotes() {
            if (state.reduced || !state.motes.length) return;
            const width = state.width;
            const height = state.height;
            const lift = 0.25 + clamp(state.smoothEnergy * 1.4, 0, 1) * 0.75;
            context.save();
            for (let i = 0; i < state.motes.length; i++) {
                const mote = state.motes[i];
                const rise = (state.frame * mote.speed + mote.y) % 1;
                const x = ((mote.x + Math.sin(state.frame / 300 + mote.drift) * 0.012 * (1 + mote.depth)) + 1) % 1 * width;
                const y = (1 - rise) * height;
                const twinkle = 0.5 + 0.5 * Math.sin(state.frame / 36 + mote.drift * 3);
                context.globalAlpha = (0.05 + mote.depth * 0.10 + twinkle * 0.06) * lift;
                context.fillStyle = state.palette.accent2;
                const r = mote.size * (0.7 + mote.depth * 0.6);
                context.beginPath();
                context.arc(x, y, r, 0, Math.PI * 2);
                context.fill();
            }
            context.restore();
            context.globalAlpha = 1;
        }

        function drawBars() {
            const width = state.width;
            const height = state.height;
            const cache = ensureCache();
            const count = state.levels.length;
            const gap = width > 780 ? 3 : 2;
            const barWidth = Math.max(2, (width - gap * (count - 1)) / count);
            const baseline = cache.baseline;
            const usable = cache.usable;
            const pulse = state.reduced ? 0 : state.smoothBass;
            // Beat-reactive floor halo.
            context.globalAlpha = 0.12 + pulse * 0.20;
            context.fillStyle = state.palette.accent;
            context.beginPath();
            context.ellipse(width * 0.5, baseline, width * 0.44, 13 + pulse * 30, 0, 0, Math.PI * 2);
            context.fill();
            context.globalAlpha = 1;
            // Soft drop shadow under the bars, drawn once as a single shape.
            context.save();
            context.globalAlpha = 0.20;
            context.fillStyle = 'rgba(0,0,0,0.55)';
            for (let i = 0; i < count; i++) {
                const value = clamp(state.levels[i], 0, 1);
                const barHeight = Math.max(2, value * usable);
                const x = i * (barWidth + gap);
                roundedBar(context, x + 1, baseline - barHeight + 3, barWidth, barHeight, barWidth / 2);
                context.fill();
            }
            context.restore();
            // Main bars: one shared gradient, per-bar tip highlight.
            context.fillStyle = cache.barGradient;
            for (let i = 0; i < count; i++) {
                const value = clamp(state.levels[i], 0, 1);
                const barHeight = Math.max(2, value * usable);
                const x = i * (barWidth + gap);
                roundedBar(context, x, baseline - barHeight, barWidth, barHeight, barWidth / 2);
                context.fill();
            }
            // Tip lights: brighter where the music is louder.
            context.save();
            context.globalAlpha = 0.75;
            context.fillStyle = '#ffffff';
            for (let i = 0; i < count; i++) {
                const value = clamp(state.levels[i], 0, 1);
                if (value < 0.12) continue;
                const barHeight = Math.max(2, value * usable);
                const x = i * (barWidth + gap);
                const tipAlpha = 0.10 + value * 0.30;
                context.globalAlpha = tipAlpha;
                const tipH = Math.min(3, barHeight * 0.3);
                roundedBar(context, x + barWidth * 0.18, baseline - barHeight, barWidth * 0.64, tipH, tipH / 2);
                context.fill();
            }
            context.restore();
            // Fading mirror below the baseline.
            context.save();
            context.globalAlpha = 0.55;
            context.fillStyle = cache.mirrorGradient;
            for (let i = 0; i < count; i++) {
                const value = clamp(state.levels[i], 0, 1);
                const barHeight = Math.max(2, value * usable) * 0.38;
                const x = i * (barWidth + gap);
                roundedBar(context, x, baseline + 3, barWidth, barHeight, barWidth / 2);
                context.fill();
            }
            context.restore();
            // Peak caps with a calm hold-then-fall.
            context.fillStyle = state.palette.accent2;
            context.save();
            context.globalAlpha = 0.9;
            for (let i = 0; i < count; i++) {
                const peak = clamp(state.peaks[i], 0, 1);
                if (peak < 0.05) continue;
                const y = baseline - Math.max(2, peak * usable) - 4;
                roundedBar(context, i * (barWidth + gap), y, barWidth, 3, 1.5);
                context.fill();
            }
            context.restore();
            // Hairline floor.
            context.globalAlpha = 0.35;
            context.fillStyle = state.palette.accent;
            roundedBar(context, 0, baseline, width, 1.5, 0);
            context.fill();
            context.globalAlpha = 1;
        }

        function drawRadial() {
            const width = state.width;
            const height = state.height;
            const cx = width / 2;
            const cy = height * 0.42;
            const pulse = state.reduced ? 0 : state.smoothBass;
            const base = Math.max(28, Math.min(width, height) * 0.2);
            const radius = base * (1 + pulse * 0.07);
            const count = state.levels.length;
            const rotation = state.reduced ? 0 : state.rotation;
            // Soft core.
            const core = context.createRadialGradient(cx, cy, 0, cx, cy, radius * 1.08);
            core.addColorStop(0, css(state.palette.accent2, 0.75));
            core.addColorStop(0.55, css(state.palette.accent, 0.5));
            core.addColorStop(1, 'rgba(0,0,0,0)');
            context.globalAlpha = 0.30 + pulse * 0.22;
            context.fillStyle = core;
            context.beginPath();
            context.arc(cx, cy, radius * 1.08, 0, Math.PI * 2);
            context.fill();
            context.globalAlpha = 1;
            // Solid inner disc keeps the centre calm on bright covers.
            context.globalAlpha = 0.16;
            context.fillStyle = '#000000';
            context.beginPath();
            context.arc(cx, cy, radius * 0.74, 0, Math.PI * 2);
            context.fill();
            context.globalAlpha = 1;

            context.lineCap = 'round';
            context.lineWidth = Math.max(2.5, radius * 0.045);
            for (let pass = 0; pass < 2; pass++) {
                context.strokeStyle = pass === 0 ? css(state.palette.accent, 0.92) : state.palette.accent2;
                for (let i = 0; i < count; i++) {
                    const value = clamp(state.levels[i], 0, 1);
                    const angle = (i / count) * Math.PI * 2 - Math.PI / 2 + rotation * (pass === 0 ? 1 : -0.6);
                    const inner = radius * (pass === 0 ? 0.78 : 1.04);
                    const outer = inner + value * radius * (pass === 0 ? 0.74 : 0.40);
                    context.globalAlpha = pass === 0 ? 0.30 + value * 0.70 : 0.18 + value * 0.72;
                    context.beginPath();
                    context.moveTo(cx + Math.cos(angle) * inner, cy + Math.sin(angle) * inner);
                    context.lineTo(cx + Math.cos(angle) * outer, cy + Math.sin(angle) * outer);
                    context.stroke();
                }
            }
            context.globalAlpha = 1;

            // Faint tick ring for structure.
            context.save();
            context.strokeStyle = state.palette.muted;
            context.globalAlpha = 0.22;
            context.lineWidth = 1.25;
            context.beginPath();
            context.arc(cx, cy, radius * 0.78, 0, Math.PI * 2);
            context.stroke();
            context.globalAlpha = 0.14;
            context.lineWidth = 1;
            for (let i = 0; i < 60; i++) {
                const angle = (i / 60) * Math.PI * 2 + rotation * 0.3;
                const inner = radius * 1.18;
                const outer = inner + (i % 5 === 0 ? 5 : 2.5);
                context.beginPath();
                context.moveTo(cx + Math.cos(angle) * inner, cy + Math.sin(angle) * inner);
                context.lineTo(cx + Math.cos(angle) * outer, cy + Math.sin(angle) * outer);
                context.stroke();
            }
            context.restore();

            // Energy ring: conic when available, calm dashed arc otherwise.
            const ringRadius = radius * (1.58 + state.smoothEnergy * 0.10);
            if (typeof context.createConicGradient === 'function') {
                const ring = context.createConicGradient(rotation, cx, cy);
                ring.addColorStop(0, state.palette.accent);
                ring.addColorStop(0.5, state.palette.accent2);
                ring.addColorStop(1, state.palette.accent);
                context.strokeStyle = ring;
                context.globalAlpha = 0.55;
                context.lineWidth = 2;
                context.beginPath();
                context.arc(cx, cy, ringRadius, 0, Math.PI * 2);
                context.stroke();
            } else {
                context.strokeStyle = state.palette.accent;
                context.globalAlpha = 0.4;
                context.lineWidth = 2;
                context.setLineDash([14, 10]);
                context.lineDashOffset = state.reduced ? 0 : -state.rotation * radius;
                context.beginPath();
                context.arc(cx, cy, ringRadius, 0, Math.PI * 2);
                context.stroke();
                context.setLineDash([]);
            }
            context.globalAlpha = 1;
            // Orbiting sparks with short trails.
            context.fillStyle = state.palette.accent2;
            const sparks = 14;
            for (let i = 0; i < sparks; i++) {
                const dir = rotation * (1.4 + (i % 3) * 0.15) + (i / sparks) * Math.PI * 2;
                const orbit = radius * (1.26 + (i % 4) * 0.12 + state.levels[(i * 4) % count] * 0.22);
                const x = cx + Math.cos(dir) * orbit;
                const y = cy + Math.sin(dir) * orbit;
                const level = clamp(state.levels[(i * 3) % count], 0, 1);
                if (!state.reduced) {
                    context.globalAlpha = 0.18 + level * 0.25;
                    context.strokeStyle = state.palette.accent2;
                    context.lineWidth = 1.5;
                    context.beginPath();
                    context.arc(cx, cy, orbit, dir - 0.22, dir);
                    context.stroke();
                }
                context.globalAlpha = 0.25 + level * 0.75;
                context.fillStyle = state.palette.accent2;
                context.beginPath();
                context.arc(x, y, i % 3 === 0 ? 2.4 : 1.5, 0, Math.PI * 2);
                context.fill();
            }
            context.globalAlpha = 1;
        }

        function wavePoint(data, i, samples) {
            if (!data) {
                if (state.reduced) return Math.sin(i / 14) * 0.06;
                const drift = state.frame / 28;
                return Math.sin(i / 11 + drift) * 0.08 + Math.sin(i / 29 - drift * 0.6) * 0.04;
            }
            const index = Math.floor((i / samples) * (data.length - 1));
            const left = data[Math.max(0, index - 2)];
            const right = data[Math.min(data.length - 1, index + 2)];
            return ((data[index] * 2 + left + right) / 4 - 128) / 128;
        }

        function smoothTrace(path, points) {
            // Midpoint quadratic smoothing: silky without overshoot.
            path.moveTo(points[0][0], points[0][1]);
            for (let i = 1; i < points.length - 1; i++) {
                const xc = (points[i][0] + points[i + 1][0]) / 2;
                const yc = (points[i][1] + points[i + 1][1]) / 2;
                path.quadraticCurveTo(points[i][0], points[i][1], xc, yc);
            }
            path.lineTo(points[points.length - 1][0], points[points.length - 1][1]);
        }

        function drawWave() {
            const width = state.width;
            const height = state.height;
            const cache = ensureCache();
            const data = readWaveform();
            const middle = height * 0.56;
            const samples = Math.min(280, Math.max(120, Math.round(width / 4)));
            // Spectrum silhouette behind the waveform.
            context.beginPath();
            const silhouette = [];
            for (let i = 0; i < state.levels.length; i++) {
                const x = (i / (state.levels.length - 1)) * width;
                const y = middle - state.levels[i] * height * 0.26;
                silhouette.push([x, y]);
            }
            if (silhouette.length > 2) {
                context.moveTo(silhouette[0][0], middle + 8);
                context.lineTo(silhouette[0][0], silhouette[0][1]);
                smoothTrace(context, silhouette);
                context.lineTo(width, middle + 8);
                context.closePath();
            } else {
                context.lineTo(width, middle + 8);
                context.lineTo(0, middle + 8);
                context.closePath();
            }
            context.globalAlpha = 0.15 + state.smoothEnergy * 0.13;
            context.fillStyle = state.palette.accent;
            context.fill();
            context.globalAlpha = 1;

            const points = [];
            for (let i = 0; i <= samples; i++) {
                const x = (i / samples) * width;
                const y = middle + wavePoint(data, i, samples) * height * 0.32;
                points.push([x, y]);
            }
            // Wide soft under-glow, then a crisp core line.
            context.save();
            context.lineJoin = 'round';
            context.lineCap = 'round';
            context.beginPath();
            smoothTrace(context, points);
            context.strokeStyle = cache.waveGradient;
            context.globalAlpha = 0.22;
            context.lineWidth = Math.max(7, height * 0.02);
            context.stroke();
            context.beginPath();
            smoothTrace(context, points);
            context.globalAlpha = 1;
            context.lineWidth = Math.max(2, height * 0.008);
            context.stroke();
            context.restore();

            // Faint echo an octave below for depth.
            if (!state.reduced) {
                context.save();
                context.globalAlpha = 0.20;
                context.strokeStyle = cache.waveGradient;
                context.lineWidth = 1.25;
                context.beginPath();
                const echo = [];
                for (let i = 0; i <= samples; i += 2) {
                    const x = (i / samples) * width;
                    const y = middle + wavePoint(data, i, samples) * height * 0.16 + height * 0.045;
                    echo.push([x, y]);
                }
                if (echo.length > 1) smoothTrace(context, echo);
                context.stroke();
                context.restore();
            }

            // Centre hairline.
            context.save();
            context.globalAlpha = 0.28;
            context.fillStyle = state.palette.accent;
            context.fillRect(0, Math.round(middle), width, 1);
            context.restore();

            if (!state.reduced) {
                context.save();
                context.globalAlpha = 0.16;
                context.beginPath();
                smoothTrace(context, points);
                context.lineTo(width, height);
                context.lineTo(0, height);
                context.closePath();
                context.fillStyle = cache.washGradient;
                context.fill();
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
            if (!state.reduced) state.rotation += 0.0024 + state.smoothBass * 0.008;
            const active = readLevels();
            if (active) state.playing = true;
            context.clearRect(0, 0, state.width, state.height);
            drawAtmosphere();
            drawMotes();
            if (state.mode === 'radial') drawRadial();
            else if (state.mode === 'wave') drawWave();
            else drawBars();
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
            // requestAnimationFrame already throttles hidden tabs; keep the
            // analyser smoothing from freezing on a stale peak.
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
                state.mode = mode;
                state.cacheKey = '';
            },
            cycleMode() {
                state.mode = MODES[(MODES.indexOf(state.mode) + 1) % MODES.length];
                state.cacheKey = '';
                return state.mode;
            },
            setPalette(palette) {
                if (!palette) return;
                state.palette = Object.assign({}, state.palette, palette);
                state.cacheKey = '';
            },
            setCoverColor(r, g, b) {
                const red = clamp(Math.round(r), 0, 255);
                const green = clamp(Math.round(g), 0, 255);
                const blue = clamp(Math.round(b), 0, 255);
                state.coverLocked = true;
                state.palette = {
                    accent: `rgb(${red},${green},${blue})`,
                    accent2: shiftHue(red, green, blue, 36),
                    muted: state.palette.muted,
                    r: red,
                    g: green,
                    b: blue
                };
                state.cacheKey = '';
            },
            clearCoverColor() {
                state.coverLocked = false;
                state.palette = readPalette(opts.root || canvas);
                state.cacheKey = '';
            },
            refreshPalette() {
                if (state.coverLocked) return;
                state.palette = readPalette(opts.root || canvas);
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

    window.AudioPlayerVisualizer = Object.freeze({ create, MODES });
})();
