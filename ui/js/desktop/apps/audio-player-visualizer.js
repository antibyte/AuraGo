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

    function create(canvas, options) {
        if (!canvas || typeof canvas.getContext !== 'function') return null;
        const context = canvas.getContext('2d');
        if (!context) return null;
        const opts = options || {};
        const reduced = typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

        const state = {
            mode: MODES.indexOf(opts.mode) >= 0 ? opts.mode : 'bars',
            playing: false,
            palette: opts.palette || readPalette(opts.root || canvas),
            coverLocked: false,
            levels: new Float32Array(BAR_COUNT),
            peaks: new Float32Array(BAR_COUNT),
            frequency: null,
            waveform: null,
            width: 0,
            height: 0,
            ratio: 1,
            frame: 0,
            rotation: 0,
            bass: 0,
            energy: 0,
            sentBass: -1,
            reduced,
            destroyed: false
        };

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
            if (Math.abs(state.bass - state.sentBass) < 0.025 && state.frame % 8 !== 0) return;
            state.sentBass = state.bass;
            opts.onEnergy(state.bass, state.energy);
        }

        // Log-shaped bins, fast attack, slow release. Silence keeps a quiet breath
        // so the stage never collapses to an empty rectangle.
        function stageShape(index, count) {
            const across = count <= 1 ? 1 : Math.sin(Math.PI * index / (count - 1));
            return 0.42 + 0.58 * across;
        }

        function idleLevel(index, count) {
            const drift = state.reduced ? 0 : state.frame / 16;
            const ripple = 0.55 + 0.45 * Math.sin(drift + index * 0.55);
            const slow = 0.75 + 0.25 * Math.sin(drift * 0.35 + index * 0.12);
            return clamp(stageShape(index, count) * ripple * slow * 0.72, 0, 1);
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
            if (!state.raw || state.raw.length !== bars.length) state.raw = new Float32Array(bars.length);
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

        function drawAtmosphere() {
            const width = state.width;
            const height = state.height;
            const lift = state.reduced ? 0 : state.bass;
            const cx = width * 0.5;
            const cy = height * 0.4;
            const radius = Math.min(width, height) * (0.34 + lift * 0.08);
            const glow = context.createRadialGradient(cx, cy, radius * 0.08, cx, cy, radius);
            glow.addColorStop(0, state.palette.accent2);
            glow.addColorStop(0.45, state.palette.accent);
            glow.addColorStop(1, 'rgba(0,0,0,0)');
            context.globalAlpha = 0.16 + lift * 0.22;
            context.fillStyle = glow;
            context.beginPath();
            context.arc(cx, cy, radius, 0, Math.PI * 2);
            context.fill();
            context.globalAlpha = 1;
        }

        function drawBars() {
            const width = state.width;
            const height = state.height;
            const count = state.levels.length;
            const gap = width > 780 ? 3 : 2;
            const barWidth = Math.max(2, (width - gap * (count - 1)) / count);
            const baseline = height * 0.78;
            const usable = height * 0.62;
            context.globalAlpha = 0.14 + state.bass * 0.2;
            context.fillStyle = state.palette.accent;
            context.beginPath();
            context.ellipse(width * 0.5, baseline, width * 0.46, 16 + state.bass * 28, 0, 0, Math.PI * 2);
            context.fill();
            context.globalAlpha = 1;
            const gradient = context.createLinearGradient(0, baseline - usable, 0, baseline);
            gradient.addColorStop(0, state.palette.accent2);
            gradient.addColorStop(0.45, state.palette.accent);
            gradient.addColorStop(1, state.palette.accent);
            context.fillStyle = gradient;
            for (let i = 0; i < count; i++) {
                const value = clamp(state.levels[i], 0, 1);
                const barHeight = Math.max(2, value * usable);
                const x = i * (barWidth + gap);
                roundedBar(context, x, baseline - barHeight, barWidth, barHeight, barWidth / 2);
                context.fill();
            }
            context.save();
            context.globalAlpha = 0.34;
            for (let i = 0; i < count; i++) {
                const value = clamp(state.levels[i], 0, 1);
                const barHeight = Math.max(2, value * usable) * 0.42;
                const x = i * (barWidth + gap);
                roundedBar(context, x, baseline + 3, barWidth, barHeight, barWidth / 2);
                context.fill();
            }
            context.restore();
            context.fillStyle = state.palette.accent2;
            for (let i = 0; i < count; i++) {
                const peak = clamp(state.peaks[i], 0, 1);
                if (peak < 0.05) continue;
                const y = baseline - Math.max(2, peak * usable) - 4;
                roundedBar(context, i * (barWidth + gap), y, barWidth, 3, 1.5);
                context.fill();
            }
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
            const cy = height * 0.4;
            const base = Math.max(28, Math.min(width, height) * 0.2);
            const radius = base * (1 + state.bass * 0.06);
            const count = state.levels.length;
            const rotation = state.reduced ? 0 : state.rotation;
            const core = context.createRadialGradient(cx, cy, 0, cx, cy, radius * 1.05);
            core.addColorStop(0, state.palette.accent2);
            core.addColorStop(0.55, state.palette.accent);
            core.addColorStop(1, 'rgba(0,0,0,0)');
            context.globalAlpha = 0.28 + state.bass * 0.2;
            context.fillStyle = core;
            context.beginPath();
            context.arc(cx, cy, radius * 1.05, 0, Math.PI * 2);
            context.fill();
            context.globalAlpha = 1;

            context.lineCap = 'round';
            context.lineWidth = Math.max(2.5, radius * 0.045);
            for (let pass = 0; pass < 2; pass++) {
                context.strokeStyle = pass === 0 ? state.palette.accent : state.palette.accent2;
                for (let i = 0; i < count; i++) {
                    const value = clamp(state.levels[i], 0, 1);
                    const angle = (i / count) * Math.PI * 2 - Math.PI / 2 + rotation;
                    const inner = radius * (pass === 0 ? 0.78 : 1.02);
                    const outer = inner + value * radius * (pass === 0 ? 0.72 : 0.38);
                    context.globalAlpha = pass === 0 ? 0.35 + value * 0.65 : 0.2 + value * 0.7;
                    context.beginPath();
                    context.moveTo(cx + Math.cos(angle) * inner, cy + Math.sin(angle) * inner);
                    context.lineTo(cx + Math.cos(angle) * outer, cy + Math.sin(angle) * outer);
                    context.stroke();
                }
            }
            context.globalAlpha = 1;

            context.strokeStyle = state.palette.muted;
            context.globalAlpha = 0.28;
            context.lineWidth = 1.25;
            context.beginPath();
            context.arc(cx, cy, radius * 0.78, 0, Math.PI * 2);
            context.stroke();
            if (typeof context.createConicGradient === 'function') {
                const ring = context.createConicGradient(rotation, cx, cy);
                ring.addColorStop(0, state.palette.accent);
                ring.addColorStop(0.5, state.palette.accent2);
                ring.addColorStop(1, state.palette.accent);
                context.strokeStyle = ring;
                context.globalAlpha = 0.55;
                context.lineWidth = 2;
                context.beginPath();
                context.arc(cx, cy, radius * (1.55 + state.energy * 0.08), 0, Math.PI * 2);
                context.stroke();
            }
            context.globalAlpha = 0.85;
            context.fillStyle = state.palette.accent2;
            const sparks = 14;
            for (let i = 0; i < sparks; i++) {
                const angle = rotation * (1.4 + (i % 3) * 0.15) + (i / sparks) * Math.PI * 2;
                const orbit = radius * (1.25 + (i % 4) * 0.12 + state.levels[i * 4] * 0.2);
                const x = cx + Math.cos(angle) * orbit;
                const y = cy + Math.sin(angle) * orbit;
                context.globalAlpha = 0.25 + state.levels[(i * 3) % count] * 0.75;
                context.beginPath();
                context.arc(x, y, i % 3 === 0 ? 2.4 : 1.5, 0, Math.PI * 2);
                context.fill();
            }
            context.globalAlpha = 1;
        }

        function wavePoint(data, i, samples) {
            if (!data) {
                const drift = state.reduced ? 0 : state.frame / 28;
                return Math.sin(i / 11 + drift) * 0.08 + Math.sin(i / 29 - drift * 0.6) * 0.04;
            }
            const index = Math.floor((i / samples) * (data.length - 1));
            const left = data[Math.max(0, index - 2)];
            const right = data[Math.min(data.length - 1, index + 2)];
            return ((data[index] * 2 + left + right) / 4 - 128) / 128;
        }

        function drawWave() {
            const width = state.width;
            const height = state.height;
            const data = readWaveform();
            const middle = height * 0.56;
            const samples = Math.min(280, Math.max(120, Math.round(width / 4)));
            const gradient = context.createLinearGradient(0, 0, width, 0);
            gradient.addColorStop(0, state.palette.accent);
            gradient.addColorStop(0.5, state.palette.accent2);
            gradient.addColorStop(1, state.palette.accent);

            context.beginPath();
            for (let i = 0; i < state.levels.length; i++) {
                const x = (i / (state.levels.length - 1)) * width;
                const y = middle - state.levels[i] * height * 0.28;
                if (i === 0) context.moveTo(x, y);
                else context.lineTo(x, y);
            }
            context.lineTo(width, middle + 8);
            context.lineTo(0, middle + 8);
            context.closePath();
            context.globalAlpha = 0.16 + state.energy * 0.12;
            context.fillStyle = state.palette.accent;
            context.fill();
            context.globalAlpha = 1;

            const trace = () => {
                context.beginPath();
                for (let i = 0; i <= samples; i++) {
                    const x = (i / samples) * width;
                    const y = middle + wavePoint(data, i, samples) * height * 0.34;
                    if (i === 0) context.moveTo(x, y);
                    else context.lineTo(x, y);
                }
            };
            trace();
            context.strokeStyle = gradient;
            context.lineJoin = 'round';
            context.lineCap = 'round';
            context.globalAlpha = 0.2;
            context.lineWidth = Math.max(7, height * 0.02);
            context.stroke();
            context.globalAlpha = 1;
            context.lineWidth = Math.max(2, height * 0.008);
            context.stroke();

            if (!state.reduced) {
                context.globalAlpha = 0.16;
                context.lineTo(width, height);
                context.lineTo(0, height);
                context.closePath();
                const wash = context.createLinearGradient(0, middle, 0, height);
                wash.addColorStop(0, state.palette.accent2);
                wash.addColorStop(1, 'rgba(0,0,0,0)');
                context.fillStyle = wash;
                context.fill();
                context.globalAlpha = 1;
            }
        }

        function frame() {
            if (state.destroyed) return;
            state.frame += 1;
            if (!state.reduced) state.rotation += 0.0024 + state.bass * 0.008;
            const active = readLevels();
            if (active) state.playing = true;
            context.clearRect(0, 0, state.width, state.height);
            drawAtmosphere();
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

        const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(() => resize()) : null;
        if (observer) observer.observe(canvas);
        window.addEventListener('resize', resize);

        resize();
        start();

        return {
            mode() { return state.mode; },
            setMode(mode) {
                if (MODES.indexOf(mode) < 0) return;
                state.mode = mode;
            },
            cycleMode() {
                state.mode = MODES[(MODES.indexOf(state.mode) + 1) % MODES.length];
                return state.mode;
            },
            setPalette(palette) {
                if (!palette) return;
                state.palette = Object.assign({}, state.palette, palette);
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
            },
            clearCoverColor() {
                state.coverLocked = false;
                state.palette = readPalette(opts.root || canvas);
            },
            refreshPalette() {
                if (state.coverLocked) return;
                state.palette = readPalette(opts.root || canvas);
            },
            resize,
            stop,
            start,
            destroy() {
                state.destroyed = true;
                stop();
                if (observer) observer.disconnect();
                window.removeEventListener('resize', resize);
            }
        };
    }

    window.AudioPlayerVisualizer = Object.freeze({ create, MODES });
})();
