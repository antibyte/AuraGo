(function () {
    'use strict';

    // Canvas spectrum renderer for the desktop music player.
    // The player owns the audio graph; this module only reads analyser frames
    // and paints them. It never touches the DOM outside of its own canvas.

    const MODES = ['bars', 'radial', 'wave'];
    const BAR_COUNT = 56;
    const PEAK_DECAY = 0.94;

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

    // Second accent colour derived from the theme accent so every desktop
    // theme gets a matching gradient without extra theme tokens.
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
        return {
            accent: read('--vd-accent', '#27c7a6'),
            accent2: shiftHue(Number.isFinite(r) ? r : 39, Number.isFinite(g) ? g : 199, Number.isFinite(b) ? b : 166, 42),
            muted: read('--vd-muted', '#a8bbd0')
        };
    }

    function roundedBar(context, x, y, width, height, radius) {
        const r = Math.min(radius, width / 2, Math.max(0, height) / 2);
        if (typeof context.roundRect === 'function') {
            context.beginPath();
            context.roundRect(x, y, width, Math.max(0, height), r);
            return;
        }
        context.beginPath();
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
            levels: new Float32Array(BAR_COUNT),
            peaks: new Float32Array(BAR_COUNT),
            frequency: null,
            waveform: null,
            width: 0,
            height: 0,
            ratio: 1,
            frame: 0,
            rotation: 0,
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

        // Log-ish mapping of the linear FFT bins onto the visible bars.
        function readLevels() {
            const node = analyser();
            const bars = state.levels;
            const peaks = state.peaks;
            if (!node) {
                for (let i = 0; i < bars.length; i++) {
                    const idle = 0.04 + 0.02 * Math.sin(state.frame / 42 + i * 0.4);
                    bars[i] = idle;
                    peaks[i] = Math.max(peaks[i] * PEAK_DECAY, idle);
                }
                return false;
            }
            const size = node.frequencyBinCount || 1024;
            if (!state.frequency || state.frequency.length !== size) state.frequency = new Uint8Array(size);
            node.getByteFrequencyData(state.frequency);
            const bins = state.frequency;
            const usable = Math.max(8, Math.floor(size * 0.72));
            let energy = 0;
            for (let i = 0; i < bars.length; i++) {
                const start = Math.floor(Math.pow(i / bars.length, 1.9) * usable);
                const end = Math.max(start + 1, Math.floor(Math.pow((i + 1) / bars.length, 1.9) * usable));
                let sum = 0;
                for (let bin = start; bin < end && bin < size; bin++) sum += bins[bin];
                const average = sum / Math.max(1, end - start) / 255;
                const value = clamp(average * 1.55 + average * average * 0.9, 0, 1);
                energy += value;
                bars[i] = bars[i] * 0.55 + value * 0.45;
                peaks[i] = Math.max(bars[i], peaks[i] * PEAK_DECAY);
            }
            return energy / bars.length > 0.015;
        }

        function readWaveform() {
            const node = analyser();
            if (!node) return null;
            const size = node.fftSize || 2048;
            if (!state.waveform || state.waveform.length !== size) state.waveform = new Uint8Array(size);
            node.getByteTimeDomainData(state.waveform);
            return state.waveform;
        }

        function drawBars() {
            const width = state.width;
            const height = state.height;
            const gap = width > 640 ? 4 : 2;
            const barWidth = Math.max(1.5, (width - gap * (state.levels.length - 1)) / state.levels.length);
            const baseline = height * 0.92;
            const usable = baseline * 0.86;
            const gradient = context.createLinearGradient(0, 0, 0, baseline);
            gradient.addColorStop(0, state.palette.accent2);
            gradient.addColorStop(1, state.palette.accent);
            context.fillStyle = gradient;
            for (let i = 0; i < state.levels.length; i++) {
                const value = clamp(state.levels[i], 0, 1);
                const barHeight = Math.max(2, value * usable);
                const x = i * (barWidth + gap);
                roundedBar(context, x, baseline - barHeight, barWidth, barHeight, barWidth / 2);
                context.fill();
            }
            context.fillStyle = state.palette.accent2;
            for (let i = 0; i < state.peaks.length; i++) {
                const peak = clamp(state.peaks[i], 0, 1);
                if (peak < 0.04) continue;
                const y = baseline - Math.max(2, peak * usable) - 2;
                roundedBar(context, i * (barWidth + gap), y, barWidth, 2, 1);
                context.fill();
            }
        }

        function drawRadial() {
            const width = state.width;
            const height = state.height;
            const cx = width / 2;
            const cy = height / 2;
            const radius = Math.max(24, Math.min(width, height) * 0.34);
            const count = state.levels.length;
            const rotation = state.reduced ? 0 : state.rotation;
            context.lineCap = 'round';
            context.lineWidth = Math.max(2, radius * 0.055);
            for (let i = 0; i < count; i++) {
                const value = clamp(state.levels[i], 0, 1);
                const angle = (i / count) * Math.PI * 2 - Math.PI / 2 + rotation;
                const inner = radius;
                const outer = radius + value * radius * 0.85;
                const gradient = context.createLinearGradient(
                    cx + Math.cos(angle) * inner, cy + Math.sin(angle) * inner,
                    cx + Math.cos(angle) * outer, cy + Math.sin(angle) * outer
                );
                gradient.addColorStop(0, state.palette.accent);
                gradient.addColorStop(1, state.palette.accent2);
                context.strokeStyle = gradient;
                context.beginPath();
                context.moveTo(cx + Math.cos(angle) * inner, cy + Math.sin(angle) * inner);
                context.lineTo(cx + Math.cos(angle) * outer, cy + Math.sin(angle) * outer);
                context.stroke();
            }
            const core = context.createRadialGradient(cx, cy, 0, cx, cy, radius);
            core.addColorStop(0, state.palette.accent2);
            core.addColorStop(1, 'rgba(0,0,0,0)');
            context.globalAlpha = 0.35;
            context.fillStyle = core;
            context.beginPath();
            context.arc(cx, cy, radius, 0, Math.PI * 2);
            context.fill();
            context.globalAlpha = 1;
            context.strokeStyle = state.palette.muted;
            context.globalAlpha = 0.35;
            context.lineWidth = 1;
            context.beginPath();
            context.arc(cx, cy, radius, 0, Math.PI * 2);
            context.stroke();
            context.globalAlpha = 1;
        }

        function drawWave() {
            const width = state.width;
            const height = state.height;
            const data = readWaveform();
            const middle = height / 2;
            const gradient = context.createLinearGradient(0, 0, width, 0);
            gradient.addColorStop(0, state.palette.accent);
            gradient.addColorStop(0.5, state.palette.accent2);
            gradient.addColorStop(1, state.palette.accent);
            context.strokeStyle = gradient;
            context.lineWidth = Math.max(2, height * 0.012);
            context.lineJoin = 'round';
            context.beginPath();
            const samples = 220;
            if (data) {
                for (let i = 0; i <= samples; i++) {
                    const index = Math.floor((i / samples) * (data.length - 1));
                    const value = (data[index] - 128) / 128;
                    const x = (i / samples) * width;
                    const y = middle + value * height * 0.36;
                    if (i === 0) context.moveTo(x, y);
                    else context.lineTo(x, y);
                }
            } else {
                for (let i = 0; i <= samples; i++) {
                    const x = (i / samples) * width;
                    const y = middle + Math.sin(i / 9 + state.frame / 30) * height * 0.05;
                    if (i === 0) context.moveTo(x, y);
                    else context.lineTo(x, y);
                }
            }
            context.stroke();
            if (!state.reduced) {
                context.globalAlpha = 0.22;
                context.lineTo(width, middle);
                context.lineTo(0, middle);
                context.closePath();
                context.fillStyle = gradient;
                context.fill();
                context.globalAlpha = 1;
            }
        }

        function frame() {
            if (state.destroyed) return;
            state.frame += 1;
            state.rotation += 0.0022;
            const active = readLevels();
            if (active) state.playing = true;
            context.clearRect(0, 0, state.width, state.height);
            if (state.mode === 'radial') drawRadial();
            else if (state.mode === 'wave') drawWave();
            else drawBars();
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
            refreshPalette() {
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
