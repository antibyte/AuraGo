/* Live Speech FX - audio-reactive background canvas for the desktop Live Speech
 * app. Renders aurora blobs tinted by session state, a microphone-reactive
 * circular waveform ring, and an output-reactive orb with an optional spectrum
 * ring. Reads mic RMS from the runtime `level` event and output levels through
 * the optional adapter `getOutputLevel()` / `getOutputSpectrum()` taps. */
(function () {
    'use strict';

    const TAU = Math.PI * 2;
    const SEGMENTS = 64;
    const SPECTRUM_BARS = 48;
    const MAX_PARTICLES = 90;
    const DPR_CAP = 1.5;

    // State hue overrides; -1 means "use the desktop accent hue".
    const STATE_HUES = {
        idle: -1,
        connecting: 38,
        listening: -1,
        speaking: 222,
        executing: 26,
        parked: 212,
        reconnecting: 38,
        error: 4
    };

    function clamp01(value) {
        return value < 0 ? 0 : value > 1 ? 1 : value;
    }

    function rgbHue(r, g, b) {
        const max = Math.max(r, g, b);
        const min = Math.min(r, g, b);
        if (max === min) return 168;
        const d = max - min;
        let hue;
        if (max === r) hue = ((g - b) / d) % 6;
        else if (max === g) hue = (b - r) / d + 2;
        else hue = (r - g) / d + 4;
        hue *= 60;
        return hue < 0 ? hue + 360 : hue;
    }

    function accentHueFor(el) {
        try {
            const raw = getComputedStyle(el).getPropertyValue('--vd-accent').trim();
            const match = /^#([0-9a-f]{6})$/i.exec(raw);
            if (match) {
                const value = parseInt(match[1], 16);
                return Math.round(rgbHue((value >> 16) & 255, (value >> 8) & 255, value & 255));
            }
        } catch (_) { }
        return 168;
    }

    function surfaceColorFor(el) {
        try {
            const raw = getComputedStyle(el).getPropertyValue('--vd-surface').trim();
            if (raw) return raw;
        } catch (_) { }
        return '#121920';
    }

    function create(options) {
        const canvas = options && options.canvas;
        const runtime = options && options.runtime;
        const ctx = canvas && canvas.getContext ? canvas.getContext('2d') : null;
        if (!canvas || !ctx || !runtime) {
            return { setEnabled: function () { }, dispose: function () { } };
        }

        const reducedMotion = typeof window.matchMedia === 'function' &&
            window.matchMedia('(prefers-reduced-motion: reduce)').matches;

        const accentHue = accentHueFor(canvas);
        const surfaceColor = surfaceColorFor(canvas);

        let disposed = false;
        let enabled = options.enabled !== false;
        let raf = 0;
        let running = false;
        let inView = true;
        let width = 0;
        let height = 0;
        let lastTime = 0;
        let clock = 0;

        let state = runtime.state || 'idle';
        let muted = !!runtime.muted;
        let speechNow = false;
        let prevSpeech = false;
        let inTarget = 0;
        let inLevel = 0;
        let outLevel = 0;
        let errorFlash = 0;

        const segLevels = new Float32Array(SEGMENTS);
        const spectrum = new Uint8Array(SPECTRUM_BARS);
        const specSmooth = new Float32Array(SPECTRUM_BARS);
        const particles = [];
        for (let i = 0; i < MAX_PARTICLES; i += 1) {
            particles.push({ x: 0, y: 0, vx: 0, vy: 0, life: 0, maxLife: 1, size: 1 });
        }
        const blobs = [
            { ang: 0.4, dist: 0.30, speed: 0.11, size: 0.52, hueShift: 0, phase: 0.0 },
            { ang: 2.5, dist: 0.38, speed: -0.08, size: 0.62, hueShift: 24, phase: 1.7 },
            { ang: 4.2, dist: 0.26, speed: 0.14, size: 0.45, hueShift: -18, phase: 3.1 },
            { ang: 5.6, dist: 0.44, speed: -0.05, size: 0.70, hueShift: 40, phase: 4.6 }
        ];

        function hueFor(current) {
            const mapped = STATE_HUES[current];
            return mapped == null || mapped === -1 ? accentHue : mapped;
        }

        function readOutputLevel() {
            const adapter = runtime.adapter;
            if (!adapter) return null;
            if (typeof adapter.getOutputLevel === 'function') return clamp01(adapter.getOutputLevel());
            const player = adapter.player;
            if (player && typeof player.getOutputLevel === 'function') return clamp01(player.getOutputLevel());
            return null;
        }

        function readOutputSpectrum() {
            const adapter = runtime.adapter;
            if (!adapter) return false;
            if (typeof adapter.getOutputSpectrum === 'function') return adapter.getOutputSpectrum(spectrum) === true;
            const player = adapter.player;
            if (player && typeof player.getOutputSpectrum === 'function') return player.getOutputSpectrum(spectrum) === true;
            return false;
        }

        function spawnParticle(cx, cy, radius, hue) {
            for (let i = 0; i < particles.length; i += 1) {
                const p = particles[i];
                if (p.life > 0) continue;
                const ang = Math.random() * TAU;
                const speed = 24 + Math.random() * 60;
                p.x = cx + Math.cos(ang) * radius;
                p.y = cy + Math.sin(ang) * radius;
                p.vx = Math.cos(ang) * speed;
                p.vy = Math.sin(ang) * speed - 12;
                p.maxLife = 0.6 + Math.random() * 0.6;
                p.life = p.maxLife;
                p.size = 1 + Math.random() * 1.8;
                p.hue = hue;
                return;
            }
        }

        function resize() {
            const rect = canvas.getBoundingClientRect();
            const dpr = Math.min(DPR_CAP, window.devicePixelRatio || 1);
            width = Math.max(1, rect.width);
            height = Math.max(1, rect.height);
            canvas.width = Math.round(width * dpr);
            canvas.height = Math.round(height * dpr);
            ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
            if (reducedMotion) drawStatic();
        }

        function drawBackground(hue, energy) {
            ctx.fillStyle = surfaceColor;
            ctx.fillRect(0, 0, width, height);
            const cx = width / 2;
            const cy = height * 0.44;
            const base = Math.min(width, height);
            ctx.globalCompositeOperation = 'lighter';
            for (let i = 0; i < blobs.length; i += 1) {
                const blob = blobs[i];
                const bx = cx + Math.cos(blob.ang + blob.phase) * blob.dist * width * 0.5;
                const by = cy + Math.sin(blob.ang * 0.9 + blob.phase) * blob.dist * height * 0.5;
                const radius = Math.max(20, blob.size * base * 0.75);
                const alpha = 0.09 + energy * 0.16;
                const gradient = ctx.createRadialGradient(bx, by, 0, bx, by, radius);
                gradient.addColorStop(0, 'hsla(' + (hue + blob.hueShift) + ',72%,56%,' + alpha + ')');
                gradient.addColorStop(1, 'hsla(' + (hue + blob.hueShift) + ',72%,56%,0)');
                ctx.fillStyle = gradient;
                ctx.fillRect(bx - radius, by - radius, radius * 2, radius * 2);
            }
            ctx.globalCompositeOperation = 'source-over';
        }

        function drawMicRing(cx, cy, radius, hue) {
            const lineWidth = Math.max(1.6, radius * 0.016);
            ctx.lineWidth = lineWidth;
            ctx.lineCap = 'round';
            for (let i = 0; i < SEGMENTS; i += 1) {
                const level = segLevels[i];
                if (level < 0.012) continue;
                const ang = (i / SEGMENTS) * TAU - Math.PI / 2;
                const inner = radius + 6;
                const outer = inner + 4 + level * radius * 0.38;
                const cos = Math.cos(ang);
                const sin = Math.sin(ang);
                ctx.strokeStyle = 'hsla(' + hue + ',78%,62%,' + (0.18 + level * 0.72) + ')';
                ctx.beginPath();
                ctx.moveTo(cx + cos * inner, cy + sin * inner);
                ctx.lineTo(cx + cos * outer, cy + sin * outer);
                ctx.stroke();
            }
        }

        function drawOrb(cx, cy, baseRadius, hue, level, active) {
            const breath = active ? 0 : 0.035 * Math.sin(clock * 1.6);
            const radius = baseRadius * (1 + breath + level * 0.55);
            if (radius < 2) return;
            const glow = ctx.createRadialGradient(cx, cy, 0, cx, cy, radius * 2.4);
            const sat = active ? 82 : 48;
            const light = active ? 62 : 46;
            glow.addColorStop(0, 'hsla(' + hue + ',' + sat + '%,' + (light + 14) + '%,' + (0.55 + level * 0.4) + ')');
            glow.addColorStop(0.45, 'hsla(' + hue + ',' + sat + '%,' + light + '%,' + (0.22 + level * 0.3) + ')');
            glow.addColorStop(1, 'hsla(' + hue + ',' + sat + '%,' + light + '%,0)');
            ctx.fillStyle = glow;
            ctx.beginPath();
            ctx.arc(cx, cy, radius * 2.4, 0, TAU);
            ctx.fill();
            ctx.fillStyle = 'hsla(' + hue + ',' + Math.min(90, sat + 8) + '%,' + (light + 18) + '%,' + (0.5 + level * 0.45) + ')';
            ctx.beginPath();
            ctx.arc(cx, cy, radius, 0, TAU);
            ctx.fill();
        }

        function drawSpectrumRing(cx, cy, radius, hue) {
            ctx.lineWidth = Math.max(1.4, radius * 0.05);
            ctx.lineCap = 'round';
            for (let i = 0; i < SPECTRUM_BARS; i += 1) {
                const value = specSmooth[i] / 255;
                if (value < 0.03) continue;
                const ang = (i / SPECTRUM_BARS) * TAU - Math.PI / 2;
                const inner = radius + 10;
                const outer = inner + 3 + value * radius * 0.6;
                const cos = Math.cos(ang);
                const sin = Math.sin(ang);
                ctx.strokeStyle = 'hsla(' + hue + ',80%,66%,' + (0.14 + value * 0.7) + ')';
                ctx.beginPath();
                ctx.moveTo(cx + cos * inner, cy + sin * inner);
                ctx.lineTo(cx + cos * outer, cy + sin * outer);
                ctx.stroke();
            }
        }

        function drawExecutingArc(cx, cy, radius, hue) {
            ctx.lineWidth = Math.max(2, radius * 0.05);
            ctx.lineCap = 'round';
            ctx.strokeStyle = 'hsla(' + hue + ',84%,64%,0.75)';
            ctx.setLineDash([radius * 0.9, radius * 0.55]);
            ctx.lineDashOffset = -clock * radius * 1.4;
            ctx.beginPath();
            ctx.arc(cx, cy, radius + 20, 0, TAU);
            ctx.stroke();
            ctx.setLineDash([]);
        }

        function updateParticles(dt) {
            for (let i = 0; i < particles.length; i += 1) {
                const p = particles[i];
                if (p.life <= 0) continue;
                p.life -= dt;
                p.x += p.vx * dt;
                p.y += p.vy * dt;
            }
        }

        function drawParticles() {
            ctx.globalCompositeOperation = 'lighter';
            for (let i = 0; i < particles.length; i += 1) {
                const p = particles[i];
                if (p.life <= 0) continue;
                const fade = p.life / p.maxLife;
                ctx.fillStyle = 'hsla(' + p.hue + ',82%,68%,' + (fade * 0.8) + ')';
                ctx.beginPath();
                ctx.arc(p.x, p.y, p.size * (0.6 + fade * 0.6), 0, TAU);
                ctx.fill();
            }
            ctx.globalCompositeOperation = 'source-over';
        }

        function update(dt) {
            clock += dt;
            for (let i = 0; i < blobs.length; i += 1) blobs[i].ang += blobs[i].speed * dt;

            if (state === 'idle' || state === 'error') inTarget = 0;
            inLevel += (inTarget - inLevel) * Math.min(1, dt * 20);
            if (inLevel < 0.004 && inTarget === 0) inLevel = 0;

            let outTarget = readOutputLevel();
            if (outTarget == null) {
                // No provider tap available: gentle synthetic pulse while speaking.
                outTarget = state === 'speaking' ? 0.4 + 0.28 * Math.sin(clock * 7) : 0;
            }
            outLevel += (outTarget - outLevel) * Math.min(1, dt * 16);
            if (outLevel < 0.004 && outTarget === 0) outLevel = 0;

            if (speechNow && !prevSpeech) {
                const cx = width / 2;
                const cy = height * 0.44;
                for (let i = 0; i < 8; i += 1) {
                    spawnParticle(cx, cy, Math.min(width, height) * 0.3, hueFor(state));
                }
            }
            prevSpeech = speechNow;
            if (outLevel > 0.45 && Math.random() < dt * 7) {
                spawnParticle(width / 2, height * 0.44, Math.min(width, height) * 0.2, hueFor('speaking'));
            }

            for (let i = 0; i < SEGMENTS; i += 1) {
                const noise = 0.5 + 0.5 * Math.sin(i * 0.7 + clock * 5.2) * Math.sin(i * 0.23 - clock * 3.1);
                const target = inLevel * (0.45 + 0.55 * noise);
                segLevels[i] += (target - segLevels[i]) * Math.min(1, dt * 18);
            }

            const hasSpectrum = readOutputSpectrum();
            for (let i = 0; i < SPECTRUM_BARS; i += 1) {
                const target = hasSpectrum ? spectrum[i] : 0;
                specSmooth[i] += (target - specSmooth[i]) * Math.min(1, dt * 14);
            }

            if (errorFlash > 0) errorFlash = Math.max(0, errorFlash - dt * 1.4);
            updateParticles(dt);
        }

        function draw() {
            const energy = clamp01(Math.max(inLevel, outLevel));
            const hue = hueFor(state);
            const cx = width / 2;
            const cy = height * 0.44;
            const base = Math.min(width, height);
            const ringRadius = base * 0.3;
            const orbRadius = base * 0.11;

            drawBackground(hue, energy);
            drawMicRing(cx, cy, ringRadius, accentHue);
            if (state === 'executing' || state === 'connecting' || state === 'reconnecting') {
                drawExecutingArc(cx, cy, orbRadius, hue);
            }
            drawOrb(cx, cy, orbRadius, state === 'speaking' ? hueFor('speaking') : hue,
                state === 'speaking' ? outLevel : energy * 0.35,
                state === 'speaking' || state === 'listening');
            drawSpectrumRing(cx, cy, orbRadius, hueFor('speaking'));
            drawParticles();

            if (state === 'parked') {
                ctx.fillStyle = 'rgba(6,10,14,0.42)';
                ctx.fillRect(0, 0, width, height);
            }
            if (errorFlash > 0) {
                const flash = ctx.createRadialGradient(cx, cy, 0, cx, cy, base * 0.8);
                flash.addColorStop(0, 'hsla(4,84%,58%,' + (errorFlash * 0.3) + ')');
                flash.addColorStop(1, 'hsla(4,84%,58%,0)');
                ctx.fillStyle = flash;
                ctx.fillRect(0, 0, width, height);
            }
        }

        function frame(now) {
            raf = 0;
            if (!running || disposed) return;
            const dt = Math.min(0.05, Math.max(0.001, (now - lastTime) / 1000));
            lastTime = now;
            update(dt);
            draw();
            if (shouldAnimate()) raf = window.requestAnimationFrame(frame);
            else running = false;
        }

        function shouldAnimate() {
            return !disposed && enabled && !reducedMotion && inView && !document.hidden;
        }

        function start() {
            if (!shouldAnimate() || running) return;
            running = true;
            lastTime = performance.now();
            raf = window.requestAnimationFrame(frame);
        }

        function stop() {
            running = false;
            if (raf) window.cancelAnimationFrame(raf);
            raf = 0;
        }

        function drawStatic() {
            if (!width || !height) return;
            const hue = hueFor(state);
            const cx = width / 2;
            const cy = height * 0.44;
            const base = Math.min(width, height);
            ctx.fillStyle = surfaceColor;
            ctx.fillRect(0, 0, width, height);
            const gradient = ctx.createRadialGradient(cx, cy, 0, cx, cy, base * 0.7);
            gradient.addColorStop(0, 'hsla(' + hue + ',60%,50%,0.16)');
            gradient.addColorStop(1, 'hsla(' + hue + ',60%,50%,0)');
            ctx.fillStyle = gradient;
            ctx.fillRect(0, 0, width, height);
            ctx.strokeStyle = 'hsla(' + accentHue + ',60%,60%,0.25)';
            ctx.lineWidth = 1.5;
            ctx.beginPath();
            ctx.arc(cx, cy, base * 0.3, 0, TAU);
            ctx.stroke();
        }

        function clear() {
            ctx.clearRect(0, 0, width, height);
        }

        const onLevel = event => {
            const detail = event.detail || {};
            if (typeof detail.rms === 'number') {
                inTarget = muted ? 0 : clamp01(detail.rms * 3.2);
            }
            if (!muted) speechNow = detail.speech === true;
            else speechNow = false;
        };
        const onState = event => {
            const next = (event.detail && event.detail.state) || runtime.state || 'idle';
            if (next === 'error' && state !== 'error') errorFlash = 1;
            state = next;
            if (state === 'idle') {
                inTarget = 0;
                speechNow = false;
            }
            if (reducedMotion) drawStatic();
            else start();
        };
        const onMute = event => {
            muted = !!(event.detail && event.detail.muted);
            if (muted) {
                inTarget = 0;
                speechNow = false;
            }
        };
        const onVisibility = () => {
            if (document.hidden) stop();
            else start();
        };

        runtime.addEventListener('level', onLevel);
        runtime.addEventListener('state', onState);
        runtime.addEventListener('mute', onMute);
        document.addEventListener('visibilitychange', onVisibility);

        let resizeObserver = null;
        if (typeof window.ResizeObserver === 'function') {
            resizeObserver = new ResizeObserver(() => resize());
            resizeObserver.observe(canvas);
        }
        let intersectionObserver = null;
        if (typeof window.IntersectionObserver === 'function') {
            intersectionObserver = new IntersectionObserver(entries => {
                inView = entries.some(entry => entry.isIntersecting);
                if (inView) start();
                else stop();
            });
            intersectionObserver.observe(canvas);
        }

        resize();
        if (reducedMotion) drawStatic();
        else start();

        return {
            setEnabled(next) {
                enabled = !!next;
                if (!enabled) {
                    stop();
                    clear();
                } else if (reducedMotion) {
                    drawStatic();
                } else {
                    start();
                }
            },
            dispose() {
                disposed = true;
                stop();
                runtime.removeEventListener('level', onLevel);
                runtime.removeEventListener('state', onState);
                runtime.removeEventListener('mute', onMute);
                document.removeEventListener('visibilitychange', onVisibility);
                if (resizeObserver) resizeObserver.disconnect();
                if (intersectionObserver) intersectionObserver.disconnect();
                particles.length = 0;
                clear();
            }
        };
    }

    window.LiveSpeechFX = { create };
})();
