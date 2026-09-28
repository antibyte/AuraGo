(function () {
    'use strict';

    const KIT = '/js/desktop/screensavers/gl-kit.js';
    const THEME_SCRIPTS = {
        abyss: ['/js/desktop/screensavers/abyss.js'],
        event_horizon: [KIT, '/js/desktop/screensavers/event-horizon.js'],
        aurora: [KIT, '/js/desktop/screensavers/aurora.js'],
        ink: [KIT, '/js/desktop/screensavers/ink.js'],
        stardust: [KIT, '/js/desktop/screensavers/stardust.js']
    };
    const QUALITY_LEVELS = ['low', 'medium', 'high'];
    const MAX_PIXELS = 3840 * 2160;
    const RESOLUTION_STEPS = [1, 0.85, 0.72, 0.6];
    const CALIBRATION_FRAMES = 24;
    const FPS_CAP = 60;
    const FPS_CAP_LONG = 30;
    const LONG_RUN_MS = 30 * 60 * 1000;
    const STOP_FADE_MS = 320;
    const HINT_MS = 3200;
    const CLOCK_CORNERS = [['start', 'end'], ['end', 'end'], ['start', 'start'], ['end', 'start']];

    const registry = new Map();
    let session = null;
    let lastReport = null;

    function register(id, factory, meta) {
        registry.set(String(id), { factory, meta: meta || {} });
    }

    function versionedURL(src) {
        const lazy = window.AuraLazyAssets;
        return lazy && typeof lazy.versionedURL === 'function' ? lazy.versionedURL(src) : src;
    }

    function alive(s) {
        return !!s && session === s && !s.stopped && (!s.opts.isCurrent || s.opts.isCurrent());
    }

    function isMobileViewport() {
        const coarse = !!(window.matchMedia && window.matchMedia('(pointer: coarse)').matches);
        return coarse && Math.min(window.innerWidth, window.innerHeight) < 820;
    }

    function translate(s, key) {
        return typeof s.opts.t === 'function' ? s.opts.t(key) : key;
    }

    function buildDom(s) {
        const root = document.createElement('div');
        root.id = 'vd-screensaver';
        root.className = 'vd-screensaver';
        root.dataset.theme = s.theme;
        root.dataset.state = 'loading';
        root.setAttribute('aria-hidden', 'true');
        root.innerHTML = '<div class="vd-screensaver-poster"></div>' +
            '<canvas class="vd-screensaver-canvas"></canvas>' +
            '<div class="vd-screensaver-vignette"></div>' +
            '<div class="vd-screensaver-clock" hidden><div class="vd-screensaver-time"></div><div class="vd-screensaver-date"></div></div>' +
            '<div class="vd-screensaver-hint"></div>';
        s.root = root;
        s.canvas = root.querySelector('.vd-screensaver-canvas');
        s.poster = root.querySelector('.vd-screensaver-poster');
        s.clock = root.querySelector('.vd-screensaver-clock');
        s.timeEl = root.querySelector('.vd-screensaver-time');
        s.dateEl = root.querySelector('.vd-screensaver-date');
        s.hint = root.querySelector('.vd-screensaver-hint');
        s.hint.textContent = translate(s, 'desktop.screensaver_wake_hint');
        document.body.appendChild(root);
        loadPoster(s);
        window.requestAnimationFrame(() => {
            if (alive(s)) root.dataset.visible = 'true';
        });
        s.timers.push(window.setTimeout(() => {
            if (alive(s)) s.hint.dataset.hidden = 'true';
        }, HINT_MS));
    }

    function loadPoster(s) {
        const url = versionedURL('/img/screensaver/' + s.theme + '.webp');
        const img = new Image();
        img.onload = () => {
            if (!s.root) return;
            s.poster.style.backgroundImage = 'url("' + url + '")';
            s.poster.dataset.loaded = 'true';
        };
        img.src = url;
    }

    function showPoster(s, reason) {
        s.fallbackReason = reason;
        s.root.dataset.state = 'poster';
        disposeScene(s);
    }

    function clockFormatters(lang) {
        try {
            return {
                time: new Intl.DateTimeFormat(lang, { hour: '2-digit', minute: '2-digit' }),
                date: new Intl.DateTimeFormat(lang, { weekday: 'long', day: 'numeric', month: 'long' })
            };
        } catch (_) {
            return {
                time: new Intl.DateTimeFormat('en', { hour: '2-digit', minute: '2-digit' }),
                date: new Intl.DateTimeFormat('en', { weekday: 'long', day: 'numeric', month: 'long' })
            };
        }
    }

    function placeClock(s) {
        let next = Math.floor(Math.random() * (CLOCK_CORNERS.length - (s.clockCorner >= 0 ? 1 : 0)));
        if (s.clockCorner >= 0 && next >= s.clockCorner) next++;
        s.clockCorner = next;
        s.clock.dataset.alignX = CLOCK_CORNERS[next][0];
        s.clock.dataset.alignY = CLOCK_CORNERS[next][1];
        s.clock.style.setProperty('--ss-clock-dx', (4 + Math.random() * 5).toFixed(2) + 'vw');
        s.clock.style.setProperty('--ss-clock-dy', (6 + Math.random() * 7).toFixed(2) + 'vh');
    }

    function updateClock(s) {
        const now = new Date();
        const minute = now.getHours() * 60 + now.getMinutes();
        if (minute === s.clockMinute) return;
        const first = s.clockMinute < 0;
        s.clockMinute = minute;
        const apply = () => {
            if (!s.root) return;
            s.timeEl.textContent = s.clockMode === 'date' ? '' : s.formatters.time.format(now);
            s.dateEl.textContent = s.formatters.date.format(now);
            placeClock(s);
            s.clock.dataset.fading = 'false';
        };
        if (first) {
            apply();
            return;
        }
        s.clock.dataset.fading = 'true';
        s.timers.push(window.setTimeout(apply, 900));
    }

    function startClock(s) {
        if (!s.opts.clock) return;
        s.formatters = clockFormatters(s.opts.lang || 'en');
        s.clock.hidden = false;
        s.clock.dataset.mode = s.clockMode;
        updateClock(s);
        s.clockTimer = window.setInterval(() => updateClock(s), 1000);
    }

    function sizeCanvas(s) {
        const mobile = isMobileViewport();
        const dpr = Math.min(window.devicePixelRatio || 1, mobile ? 1 : 1.5);
        const cssW = Math.max(1, window.innerWidth);
        const cssH = Math.max(1, window.innerHeight);
        let w = cssW * dpr * RESOLUTION_STEPS[s.resolutionStep];
        let h = cssH * dpr * RESOLUTION_STEPS[s.resolutionStep];
        const budget = Math.sqrt(MAX_PIXELS / (w * h));
        if (budget < 1) {
            w *= budget;
            h *= budget;
        }
        s.width = Math.max(1, Math.round(w));
        s.height = Math.max(1, Math.round(h));
        s.canvas.width = s.width;
        s.canvas.height = s.height;
        if (s.scene && typeof s.scene.resize === 'function') s.scene.resize(s.width, s.height);
    }

    function median(values) {
        const sorted = values.slice().sort((a, b) => a - b);
        return sorted[Math.floor(sorted.length / 2)] || 16.7;
    }

    function degrade(s) {
        if (s.resolutionStep < RESOLUTION_STEPS.length - 1 && (s.quality === 'low' || s.resolutionStep < 1)) {
            s.resolutionStep++;
            sizeCanvas(s);
        } else if (QUALITY_LEVELS.indexOf(s.quality) > 0) {
            s.quality = QUALITY_LEVELS[QUALITY_LEVELS.indexOf(s.quality) - 1];
            if (typeof s.scene.setQuality === 'function') s.scene.setQuality(s.quality);
        } else if (s.resolutionStep < RESOLUTION_STEPS.length - 1) {
            s.resolutionStep++;
            sizeCanvas(s);
        }
        s.downgrades++;
    }

    function adaptQuality(s, interval) {
        if (s.calibration.length < CALIBRATION_FRAMES) return;
        const target = Math.max(1000 / s.fpsCap, s.displayPeriod);
        s.frameEma = s.frameEma ? s.frameEma * 0.92 + interval * 0.08 : interval;
        if (s.time < 2.5 || s.time < s.cooldownUntil) return;
        if (s.frameEma > target * 1.4) s.slowFor += interval / 1000;
        else s.slowFor = Math.max(0, s.slowFor - interval / 2000);
        if (s.slowFor > 2) {
            s.slowFor = 0;
            s.frameEma = 0;
            s.cooldownUntil = s.time + 3;
            degrade(s);
        }
    }

    function readProbe(s) {
        const gl = s.canvas.getContext('webgl2');
        if (!gl) return null;
        const w = gl.drawingBufferWidth;
        const h = gl.drawingBufferHeight;
        const pixels = new Uint8Array(w * h * 4);
        gl.readPixels(0, 0, w, h, gl.RGBA, gl.UNSIGNED_BYTE, pixels);
        let sum = 0;
        let lit = 0;
        let max = 0;
        const step = Math.max(1, Math.floor((w * h) / 20000));
        let count = 0;
        for (let i = 0; i < w * h; i += step) {
            const o = i * 4;
            const lum = 0.2126 * pixels[o] + 0.7152 * pixels[o + 1] + 0.0722 * pixels[o + 2];
            sum += lum;
            if (lum > 12) lit++;
            if (lum > max) max = lum;
            count++;
        }
        return { mean: sum / count, max, litFraction: lit / count, width: w, height: h, error: gl.getError() };
    }

    function captureFrame(s, request) {
        const out = document.createElement('canvas');
        out.width = request.width || s.width;
        out.height = request.height || s.height;
        const ctx2d = out.getContext('2d');
        ctx2d.drawImage(s.canvas, 0, 0, out.width, out.height);
        return out.toDataURL(request.type || 'image/webp', request.quality || 0.86);
    }

    function loop(s, now) {
        if (!alive(s) || !s.scene) return;
        s.raf = window.requestAnimationFrame(t => loop(s, t));
        const rafDelta = s.lastRaf ? now - s.lastRaf : 16.7;
        s.lastRaf = now;
        if (s.calibration.length < CALIBRATION_FRAMES) {
            s.calibration.push(rafDelta);
            if (s.calibration.length === CALIBRATION_FRAMES) s.displayPeriod = median(s.calibration.slice(4));
        }
        s.fpsCap = Date.now() - s.startedAt > LONG_RUN_MS ? FPS_CAP_LONG : FPS_CAP;
        const interval = 1000 / s.fpsCap;
        if (s.lastRender && now - s.lastRender < interval - Math.min(4, s.displayPeriod * 0.5)) return;
        const elapsed = s.lastRender ? now - s.lastRender : interval;
        s.lastRender = now;
        const dt = Math.min(0.1, elapsed / 1000);
        s.time += dt;
        try {
            s.scene.frame(s.time, dt);
        } catch (err) {
            console.warn('Screensaver frame failed', err);
            showPoster(s, 'frame-error');
            return;
        }
        s.frames++;
        if (s.frames === 1) s.root.dataset.state = 'running';
        if (s.probeRequests.length) {
            const report = readProbe(s);
            s.probeRequests.splice(0).forEach(resolve => resolve(report));
        }
        if (s.captureRequests.length) {
            s.captureRequests.splice(0).forEach(entry => {
                try { entry.resolve(captureFrame(s, entry.request)); } catch (err) { entry.reject(err); }
            });
        }
        adaptQuality(s, elapsed);
    }

    function disposeScene(s) {
        if (s.raf) window.cancelAnimationFrame(s.raf);
        s.raf = 0;
        const scene = s.scene;
        s.scene = null;
        if (scene && typeof scene.dispose === 'function') {
            try { scene.dispose(); } catch (err) { console.warn('Screensaver dispose failed', err); }
        }
        s.probeRequests.splice(0).forEach(resolve => resolve(null));
        s.captureRequests.splice(0).forEach(entry => entry.reject(new Error('screensaver scene unavailable')));
    }

    function loadThemeScripts(theme) {
        if (registry.has(theme)) return Promise.resolve();
        const lazy = window.AuraLazyAssets;
        if (!lazy || typeof lazy.loadAll !== 'function') return Promise.reject(new Error('asset loader unavailable'));
        return lazy.loadAll({ scripts: THEME_SCRIPTS[theme] || [] });
    }

    function createSession(opts) {
        return {
            opts,
            theme: String(opts.theme || 'abyss'),
            quality: isMobileViewport() ? 'medium' : 'high',
            resolutionStep: 0,
            stopped: false,
            timers: [],
            startedAt: Date.now(),
            time: 0,
            frames: 0,
            lastRaf: 0,
            lastRender: 0,
            calibration: [],
            displayPeriod: 16.7,
            fpsCap: FPS_CAP,
            frameEma: 0,
            slowFor: 0,
            cooldownUntil: 0,
            downgrades: 0,
            clockMinute: -1,
            clockCorner: -1,
            clockMode: 'full',
            probeRequests: [],
            captureRequests: [],
            fallbackReason: '',
            scene: null,
            raf: 0
        };
    }

    async function start(opts) {
        stopNow('restart');
        const s = createSession(opts || {});
        session = s;
        const entry = registry.get(s.theme);
        s.clockMode = (entry && entry.meta.clock) || (s.theme === 'stardust' ? 'date' : 'full');
        buildDom(s);
        startClock(s);
        if (s.opts.reducedMotion) {
            showPoster(s, 'reduced-motion');
            return;
        }
        try {
            await loadThemeScripts(s.theme);
            if (!alive(s)) return;
            const registered = registry.get(s.theme);
            if (!registered) throw new Error('unknown screensaver theme: ' + s.theme);
            sizeCanvas(s);
            s.onContextLost = event => {
                if (event && event.preventDefault) event.preventDefault();
                if (alive(s)) showPoster(s, 'context-lost');
            };
            s.canvas.addEventListener('webglcontextlost', s.onContextLost, false);
            const scene = await registered.factory({
                canvas: s.canvas,
                width: s.width,
                height: s.height,
                quality: s.quality,
                lang: s.opts.lang || 'en',
                t: s.opts.t,
                versionedURL,
                isCurrent: () => alive(s)
            });
            if (!alive(s)) {
                if (scene && typeof scene.dispose === 'function') scene.dispose();
                return;
            }
            s.scene = scene;
            s.onResize = () => {
                window.clearTimeout(s.resizeTimer);
                s.resizeTimer = window.setTimeout(() => {
                    if (alive(s) && s.scene) sizeCanvas(s);
                }, 150);
            };
            s.onVisibility = () => {
                s.lastRender = 0;
                s.lastRaf = 0;
            };
            window.addEventListener('resize', s.onResize);
            document.addEventListener('visibilitychange', s.onVisibility);
            s.raf = window.requestAnimationFrame(t => loop(s, t));
        } catch (err) {
            if (!alive(s)) return;
            console.warn('Screensaver theme unavailable', err);
            s.error = String(err && err.message || err);
            showPoster(s, 'unavailable');
        }
    }

    function report(s) {
        return {
            theme: s.theme,
            state: s.root ? s.root.dataset.state : 'stopped',
            quality: s.quality,
            resolutionScale: RESOLUTION_STEPS[s.resolutionStep],
            width: s.width || 0,
            height: s.height || 0,
            frames: s.frames,
            time: s.time,
            fps: s.frameEma ? 1000 / s.frameEma : 0,
            fpsCap: s.fpsCap,
            displayPeriod: s.displayPeriod,
            downgrades: s.downgrades,
            fallbackReason: s.fallbackReason,
            error: s.error || '',
            clockMode: s.opts.clock ? s.clockMode : 'off',
            clockText: s.opts.clock ? (s.timeEl.textContent + ' ' + s.dateEl.textContent).trim() : '',
            sceneStats: s.scene && typeof s.scene.stats === 'function' ? s.scene.stats() : null
        };
    }

    function teardown(s) {
        s.stopped = true;
        disposeScene(s);
        s.timers.forEach(id => window.clearTimeout(id));
        s.timers = [];
        if (s.clockTimer) window.clearInterval(s.clockTimer);
        window.clearTimeout(s.resizeTimer);
        if (s.onResize) window.removeEventListener('resize', s.onResize);
        if (s.onVisibility) document.removeEventListener('visibilitychange', s.onVisibility);
        if (s.onContextLost && s.canvas) s.canvas.removeEventListener('webglcontextlost', s.onContextLost, false);
        if (s.root && s.root.parentNode) s.root.parentNode.removeChild(s.root);
        s.root = null;
    }

    function stopNow(reason) {
        const s = session;
        if (!s) return;
        lastReport = Object.assign(report(s), { stoppedBy: reason || 'stop' });
        session = null;
        teardown(s);
    }

    function stop(options) {
        const s = session;
        if (!s) return;
        const reason = (options && options.reason) || 'stop';
        lastReport = Object.assign(report(s), { stoppedBy: reason });
        session = null;
        s.stopped = true;
        disposeScene(s);
        if (reason === 'pagehide' || !s.root) {
            teardown(s);
            return;
        }
        s.root.dataset.state = 'stopping';
        window.setTimeout(() => teardown(s), STOP_FADE_MS);
    }

    function probe() {
        const s = session;
        if (!s || !s.scene) return Promise.resolve(null);
        return new Promise(resolve => s.probeRequests.push(resolve));
    }

    function capture(request) {
        const s = session;
        if (!s || !s.scene) return Promise.reject(new Error('screensaver scene unavailable'));
        return new Promise((resolve, reject) => s.captureRequests.push({ request: request || {}, resolve, reject }));
    }

    function inspect() {
        return {
            active: !!session,
            current: session ? report(session) : null,
            last: lastReport,
            registered: Array.from(registry.keys())
        };
    }

    window.AuraScreensavers = { register };
    window.AuraScreensaverHost = { start, stop, probe, capture, inspect, themes: Object.keys(THEME_SCRIPTS) };
})();
