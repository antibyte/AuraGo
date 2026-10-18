(function () {
    'use strict';

    // Effects of the Crazy desktop theme: a confetti burst from the titlebar of every new window and a short
    // sparkle trail behind the mouse. The host (fx-core.js) owns the layer, the AbortSignal and the hidden-tab
    // pause; css/desktop-theme-crazy.css styles the layer and the particles.
    //
    // Budget: two fixed pools of <span>s, created once with createElement; every particle is one Web Animation
    // on transform and opacity. There is no per-frame loop and no timer, so nothing runs between spawns, and
    // a burst measures the page once (the titlebar rectangle).
    const CONFETTI_POOL = 36;
    const CONFETTI_PER_BURST = 28;
    const SPARKLE_POOL = 14;
    const SPARKLE_GAP_MS = 33;
    const SPARKLE_LIFE_MS = 450;
    const COLOR_COUNT = 6;
    // The layer sits below the taskbar, menus and dialogs: sparkles over them would never be seen.
    const SPARKLE_BLOCKERS = '.vd-window-content, .vd-taskbar, .vd-start-menu, .vd-context-menu, .vd-modal-backdrop, .vd-toast-container';

    function between(min, max) {
        return min + Math.random() * (max - min);
    }

    // The keyframe transform that puts a particle at a viewport position (the layer is fixed at 0,0).
    function place(x, y, degrees, scale) {
        return 'translate(' + x.toFixed(1) + 'px, ' + y.toFixed(1) + 'px) rotate(' + degrees.toFixed(0) + 'deg) scale(' + scale + ')';
    }

    // A fixed set of idle spans that each run one animation at a time. A span returns to the pool when its
    // animation finishes or is cancelled.
    function createPool(layer, size, classNameFor) {
        const nodes = [];
        const free = [];
        const playing = new Map();
        const fragment = document.createDocumentFragment();
        for (let i = 0; i < size; i++) {
            const node = document.createElement('span');
            node.className = classNameFor(i);
            fragment.appendChild(node);
            nodes.push(node);
            free.push(node);
        }
        layer.appendChild(fragment);

        // Runs one animation on a free span; false when every span is busy.
        function play(keyframes, options, onDone) {
            const node = free.pop();
            if (!node) return false;
            const animation = node.animate(keyframes, options);
            playing.set(node, animation);
            const settle = () => {
                if (playing.get(node) !== animation) return;
                playing.delete(node);
                free.push(node);
                if (onDone) onDone();
            };
            animation.onfinish = settle;
            animation.oncancel = settle;
            return true;
        }

        // Stops everything at once and takes every span back; no callback fires.
        function cancelAll() {
            playing.forEach((animation, node) => {
                animation.onfinish = null;
                animation.oncancel = null;
                animation.cancel();
                free.push(node);
            });
            playing.clear();
        }

        function dispose() {
            cancelAll();
            free.length = 0;
            nodes.forEach(node => node.remove());
            nodes.length = 0;
        }

        return { play, cancelAll, dispose };
    }

    function start({ layer, signal }) {
        const inert = { pause() {}, resume() {}, stop() {} };
        // The shell does not start effects in these cases; stay safe when something else asks.
        const reduced = typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
        const animationsOff = !!(document.body && document.body.dataset.animations === 'false');
        if (reduced || animationsOff || typeof Element.prototype.animate !== 'function') return inert;

        // Shape alternates strip and dot, colour steps every two spans, so both shapes meet every colour.
        const confetti = createPool(layer, CONFETTI_POOL, index =>
            'vd-fx-particle ' + (index % 2 ? 'vd-fx-dot' : 'vd-fx-strip') + ' vd-fx-c' + (Math.floor(index / 2) % COLOR_COUNT));
        const sparkles = createPool(layer, SPARKLE_POOL, index =>
            'vd-fx-particle vd-fx-star vd-fx-c' + (index % COLOR_COUNT));

        let paused = false;
        let stopped = false;
        let burstRemaining = 0;
        let lastSparkle = -Infinity;

        function onConfettiDone() {
            if (burstRemaining > 0) burstRemaining -= 1;
        }

        // One burst at a time: a window added while particles still fly does not start another.
        function burst(windowElement) {
            if (paused || stopped || burstRemaining > 0) return;
            const titlebar = windowElement.querySelector('.vd-window-titlebar');
            if (!titlebar) return;
            const rect = titlebar.getBoundingClientRect();
            if (rect.width < 1 || rect.height < 1) return;
            const centreX = rect.left + rect.width / 2;
            const centreY = rect.top + rect.height / 2;
            // The window is still bouncing in: spread over the titlebar width it has at rest.
            const half = (titlebar.offsetWidth || rect.width) / 2;
            const viewHeight = window.innerHeight;
            // Confetti shoots up where there is room and spills over the window where there is not.
            const upShare = Math.min(0.75, Math.max(0.2, centreY / (centreY + (viewHeight - centreY) * 0.5 + 1)));
            for (let i = 0; i < CONFETTI_PER_BURST; i++) {
                const startX = centreX + between(-0.8, 0.8) * half;
                const upward = Math.random() < upShare;
                const angle = upward ? between(-Math.PI * 0.92, -Math.PI * 0.08) : between(Math.PI * 0.1, Math.PI * 0.9);
                let reach = upward ? between(100, 280) : between(80, 220);
                if (upward && centreY - Math.sin(-angle) * reach < 16) {
                    reach = Math.max(40, (centreY - 16) / Math.max(0.25, Math.sin(-angle)));
                }
                const spin = between(-540, 540);
                const size = between(0.9, 1.5);
                const apexX = startX + Math.cos(angle) * reach;
                const apexY = centreY + Math.sin(angle) * reach;
                const endX = apexX + between(-40, 40);
                const endY = Math.min(viewHeight - 8, apexY + (upward ? between(120, 220) : between(20, 90)));
                // Rise fast, fall with gravity, and only fade out over the last fifth.
                const started = confetti.play([
                    { transform: place(startX, centreY, 0, size * 0.5), opacity: 1, easing: 'cubic-bezier(0.15, 0.75, 0.35, 1)' },
                    { transform: place(apexX, apexY, spin * 0.4, size), opacity: 1, offset: 0.4, easing: 'cubic-bezier(0.5, 0, 0.9, 0.55)' },
                    { transform: place(apexX + (endX - apexX) * 0.6, apexY + (endY - apexY) * 0.6, spin * 0.8, size), opacity: 1, offset: 0.8, easing: 'linear' },
                    { transform: place(endX, endY, spin, size * 0.9), opacity: 0 }
                ], { duration: Math.round(between(700, 900)), easing: 'linear' }, onConfettiDone);
                if (!started) break;
                burstRemaining += 1;
            }
        }

        function onWindowLayerChange(records) {
            for (const record of records) {
                for (const node of record.addedNodes) {
                    if (node.nodeType === 1 && node.classList.contains('vd-window')) {
                        burst(node);
                        return;
                    }
                }
            }
        }

        // At most one sparkle per SPARKLE_GAP_MS, mouse only, never over app content or shell surfaces.
        function onPointerMove(event) {
            if (paused || event.pointerType !== 'mouse') return;
            if (event.timeStamp - lastSparkle < SPARKLE_GAP_MS) return;
            const target = event.target;
            if (target && typeof target.closest === 'function' && target.closest(SPARKLE_BLOCKERS)) return;
            lastSparkle = event.timeStamp;
            const x = event.clientX - 7;
            const y = event.clientY - 7;
            const drift = between(-16, 16);
            sparkles.play([
                { transform: place(x, y, 0, 0.3), opacity: 1 },
                { transform: place(x + drift * 0.5, y + 10, 90, 1.1), opacity: 1, offset: 0.35 },
                { transform: place(x + drift, y + between(26, 40), 200, 0), opacity: 0 }
            ], { duration: SPARKLE_LIFE_MS, easing: 'ease-out' });
        }

        const windowLayer = document.getElementById('vd-window-layer');
        let observer = null;
        if (windowLayer && typeof MutationObserver === 'function') {
            observer = new MutationObserver(onWindowLayerChange);
            observer.observe(windowLayer, { childList: true });
        }
        document.addEventListener('pointermove', onPointerMove, { passive: true, signal });

        function pause() {
            paused = true;
            burstRemaining = 0;
            confetti.cancelAll();
            sparkles.cancelAll();
        }

        function resume() {
            if (!stopped) paused = false;
        }

        function stop() {
            if (stopped) return;
            stopped = true;
            if (observer) observer.disconnect();
            document.removeEventListener('pointermove', onPointerMove);
            confetti.dispose();
            sparkles.dispose();
        }

        // The host stops us before it aborts; this covers an abort without a stop.
        if (signal && typeof signal.addEventListener === 'function') signal.addEventListener('abort', stop, { once: true });
        return { pause, resume, stop };
    }

    window.AuraDesktopThemeFx.register('crazy', { start });
})();
