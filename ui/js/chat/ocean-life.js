(function () {
    'use strict';

    // Ocean chat theme: three.js sea life behind the messages. Loaded by ocean-shader.js; the heavy
    // renderer lives in /js/vendor/chat-ocean-life/ocean-life.esm.js and is imported on demand.
    const THEME = 'ocean';
    const BUNDLE = '/js/vendor/chat-ocean-life/ocean-life.esm.js';
    const MODEL_BASE = '/3d/chat-ocean-life/v1/';
    const ABYSS_BASE = '/3d/screensaver/abyss/v1/';
    const MIN_WIDTH = 520;
    const MAX_DPR = 1.5;
    const MAX_PIXELS = 2560 * 1440;
    const MAX_DT = 0.05;

    let canvas = null;
    let chatBox = null;
    let statusEl = null;
    let scene = null;
    let loading = null;
    // generation the chain in `loading` was started for; stop() bumps `generation`, which supersedes that chain
    let loadingGeneration = 0;
    // The last load failed (no WebGL context, no model): later resizes must not re-fetch and re-parse everything.
    // Cleared by a real theme change away from Ocean.
    let failed = false;
    let generation = 0;
    let active = false;
    let animated = true;
    let animationId = null;
    let lastFrame = 0;
    let startTime = 0;
    let resizeObserver = null;
    let observedBox = null;
    let listenersAttached = false;
    let width = 0;
    let height = 0;
    let dpr = 1;
    const pointer = { x: 0.5, y: 0.5, active: false };

    function versioned(path) {
        return path + '?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev');
    }

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function currentTheme() {
        return document.documentElement.getAttribute('data-theme') || 'dark';
    }

    function shouldRun() {
        return currentTheme() === THEME && window.innerWidth >= MIN_WIDTH;
    }

    function isBusy() {
        if (!statusEl || !statusEl.isConnected) statusEl = document.getElementById('agentStatusContainer');
        return !!statusEl && !statusEl.classList.contains('is-hidden');
    }

    function quality() {
        const pixels = width * height * dpr * dpr;
        if (pixels >= 1.2e6) return 'high';
        if (pixels >= 0.5e6) return 'medium';
        return 'low';
    }

    function ensureCanvas() {
        if (canvas && canvas.isConnected) return;
        canvas = document.createElement('canvas');
        canvas.id = 'ocean-life-overlay';
        canvas.setAttribute('aria-hidden', 'true');
        Object.assign(canvas.style, {
            position: 'fixed', top: '0', left: '0', width: '0', height: '0',
            pointerEvents: 'none', zIndex: '2', display: 'none'
        });
        canvas.addEventListener('webglcontextlost', function (event) {
            event.preventDefault();
            stop();
        });
        const caustics = document.getElementById('ocean-overlay');
        if (caustics && caustics.parentNode === document.body) document.body.insertBefore(canvas, caustics);
        else document.body.appendChild(canvas);
    }

    function updateBounds() {
        if (!canvas || !chatBox) return false;
        const rect = chatBox.getBoundingClientRect();
        const nextWidth = Math.round(rect.width);
        const nextHeight = Math.round(rect.height);
        if (!nextWidth || !nextHeight) {
            canvas.style.width = '0';
            canvas.style.height = '0';
            return false;
        }
        width = nextWidth;
        height = nextHeight;
        dpr = Math.min(window.devicePixelRatio || 1, MAX_DPR, Math.sqrt(MAX_PIXELS / (width * height)));
        canvas.style.left = Math.round(rect.left) + 'px';
        canvas.style.top = Math.round(rect.top) + 'px';
        canvas.style.width = width + 'px';
        canvas.style.height = height + 'px';
        return true;
    }

    function input() {
        return { pointer: { x: pointer.x, y: pointer.y, active: pointer.active }, busy: isBusy() };
    }

    function frame(now) {
        animationId = null;
        if (!active || !scene || !animated) return;
        const dt = Math.min((now - lastFrame) / 1000, MAX_DT) || (1 / 60);
        lastFrame = now;
        scene.frame((now - startTime) / 1000, dt, input());
        animationId = window.requestAnimationFrame(frame);
    }

    function startLoop() {
        if (animationId || !animated || !scene || document.hidden) return;
        lastFrame = performance.now();
        animationId = window.requestAnimationFrame(frame);
    }

    function stopLoop() {
        if (animationId) {
            window.cancelAnimationFrame(animationId);
            animationId = null;
        }
    }

    function seed() {
        return (Math.floor(Math.random() * 0xffffffff) >>> 0) || 1;
    }

    // Debug seam for tests and the harness: window.__auraOceanLifeDebug = { firstWhaleDelay: <seconds> } (read at start()) brings the first whale pass forward.
    function firstWhaleDelay() {
        const debug = window.__auraOceanLifeDebug;
        const delay = debug && Number(debug.firstWhaleDelay);
        return delay > 0 && isFinite(delay) ? delay : undefined;
    }

    // Called when a load chain ends: forgets `pending` if it is still the newest chain and reports whether
    // it still belongs to the current generation (false: a stop() superseded it while it was loading).
    function settle(pending, myGeneration) {
        if (loading === pending) loading = null;
        return myGeneration === generation;
    }

    function start() {
        if (active || failed) return;
        // a load that stop() already superseded must not block a restart
        if (loading && loadingGeneration === generation) return;
        chatBox = document.getElementById('chat-box');
        if (!chatBox) return;
        ensureCanvas();
        if (resizeObserver && observedBox !== chatBox) {
            resizeObserver.disconnect();
            resizeObserver.observe(chatBox);
            observedBox = chatBox;
        }
        if (!updateBounds()) return;

        const myGeneration = ++generation;
        animated = !prefersReducedMotion();
        const pending = import(versioned(BUNDLE)).then(function (module) {
            if (myGeneration !== generation || !shouldRun()) return null;
            return module.createOceanLife({
                canvas: canvas, width: width, height: height, dpr: dpr, quality: quality(), seed: seed(),
                modelBase: MODEL_BASE, abyssBase: ABYSS_BASE, versionedURL: versioned, firstWhaleDelay: firstWhaleDelay()
            });
        }).then(function (created) {
            const current = settle(pending, myGeneration);
            if (created && current && shouldRun()) {
                scene = created;
                active = true;
                startTime = performance.now();
                canvas.style.display = 'block';
                if (animated) startLoop();
                else scene.stillFrame();
                return;
            }
            if (created) created.dispose();
            // superseded while loading: look at the state as it is now
            if (!current) sync();
        }).catch(function (err) {
            const current = settle(pending, myGeneration);
            console.warn('[OceanLife] sea life layer unavailable:', err);
            if (current) failed = true;
            else sync();
        });
        loading = pending;
        loadingGeneration = myGeneration;
    }

    function stop() {
        generation += 1;
        active = false;
        stopLoop();
        if (scene) {
            scene.dispose();
            scene = null;
        }
        if (canvas) {
            canvas.style.display = 'none';
            if (canvas.parentNode) canvas.parentNode.removeChild(canvas);
            canvas = null;
        }
    }

    function refresh() {
        if (!active) {
            if (shouldRun()) start();
            return;
        }
        if (!shouldRun()) {
            stop();
            return;
        }
        if (!updateBounds() || !scene) return;
        scene.resize(width, height, dpr);
        if (!animated) scene.stillFrame();
    }

    function sync() {
        // a real theme change away from Ocean lifts the failure latch, so picking Ocean again retries once
        if (currentTheme() !== THEME) failed = false;
        if (shouldRun()) {
            if (!active) start();
            else refresh();
            return;
        }
        if (active || loading) stop();
    }

    function attachObservers() {
        if (listenersAttached) return;
        listenersAttached = true;

        window.addEventListener('resize', refresh);
        window.addEventListener('aurago:themechange', sync);

        document.addEventListener('visibilitychange', function () {
            if (!active) return;
            if (document.hidden) stopLoop();
            else startLoop();
        });

        const motionQuery = window.matchMedia ? window.matchMedia('(prefers-reduced-motion: reduce)') : null;
        if (motionQuery && typeof motionQuery.addEventListener === 'function') {
            motionQuery.addEventListener('change', function () {
                // also drops a load in flight: it would start with the old `animated` value
                if (active || loading) stop();
                sync();
            });
        }

        if (window.ResizeObserver) resizeObserver = new ResizeObserver(refresh);

        new MutationObserver(sync).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });

        document.addEventListener('mousemove', function (event) {
            if (!active || !chatBox) return;
            const rect = chatBox.getBoundingClientRect();
            if (!rect.width || !rect.height) return;
            const x = (event.clientX - rect.left) / rect.width;
            const y = (event.clientY - rect.top) / rect.height;
            pointer.active = x >= 0 && x <= 1 && y >= 0 && y <= 1;
            if (pointer.active) {
                pointer.x = x;
                pointer.y = y;
            }
        }, { passive: true });
        document.addEventListener('mouseleave', function () { pointer.active = false; });
    }

    function stats() {
        const base = scene ? scene.stats() : {};
        return Object.assign({ running: active, reducedMotion: !animated }, base);
    }

    window.AuraGoOceanLife = { start: start, stop: stop, sync: sync };
    window.__auraOceanLife = { stats: stats };

    // The listeners go up at load, whatever the theme or width is: a narrow window or another theme at load
    // must still start the sea life later. start() only swaps what the ResizeObserver watches.
    attachObservers();
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', sync, { once: true });
    else sync();
})();
