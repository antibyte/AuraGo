(function () {
    'use strict';

    const THEME = 'black-matrix';
    const CHARSET = '01<>[]{}#*+-=ZXCVBNMアイウエオカキクケコサシスセソ';
    const FPS = 30;
    // Two depth planes: a small, dim far field behind a brighter near field.
    // Glyphs stay fixed in their grid cells; drops only light them up, so the
    // field reads as code rain instead of per-frame noise.
    const LAYERS = [
        {
            font: '500 {size}px "Geist Mono", "Cascadia Code", monospace',
            size: 12,
            mobileSize: 10,
            speed: [4.5, 8],
            trail: [12, 24],
            density: 0.46,
            alpha: 0.11,
            head: 0.24,
            glow: 0,
            color: '96, 206, 84'
        },
        {
            font: '600 {size}px "Oxanium", "Cascadia Code", monospace',
            size: 17,
            mobileSize: 14,
            speed: [7, 13],
            trail: [8, 18],
            density: 0.32,
            alpha: 0.24,
            head: 0.52,
            glow: 10,
            color: '128, 246, 98'
        }
    ];
    const MUTATE_CHANCE = 0.035;
    const BUSY_SPEED = 1.7;
    const BUSY_GLOW = 1.3;

    let canvas;
    let ctx;
    let chatBox;
    let statusEl;
    let animationId = null;
    let active = false;
    let animated = true;
    let planes = [];
    let laneMask = null;
    let edgeMask = null;
    let busyLevel = 0;
    let resizeObserver = null;
    let observedBox = null;
    let themeObserver = null;
    let listenersAttached = false;
    let width = 0;
    let height = 0;
    let lastFrame = 0;

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function currentTheme() {
        return document.documentElement.getAttribute('data-theme') || 'dark';
    }

    function shouldRun() {
        return currentTheme() === THEME;
    }

    function rand(min, max) {
        return min + Math.random() * (max - min);
    }

    function randomChar() {
        return CHARSET[Math.floor(Math.random() * CHARSET.length)];
    }

    function isBusy() {
        if (!statusEl || !statusEl.isConnected) statusEl = document.getElementById('agentStatusContainer');
        return !!statusEl && !statusEl.classList.contains('is-hidden');
    }

    function ensureCanvas() {
        if (canvas) return;
        canvas = document.createElement('canvas');
        canvas.id = 'black-matrix-overlay';
        canvas.setAttribute('aria-hidden', 'true');
        Object.assign(canvas.style, {
            position: 'fixed',
            top: '0',
            left: '0',
            width: '0',
            height: '0',
            pointerEvents: 'none',
            zIndex: '2',
            display: 'none'
        });
        document.body.appendChild(canvas);
        ctx = canvas.getContext('2d', { alpha: true });
    }

    function buildMasks() {
        // Fade the rain behind the centred reading lane and at the top/bottom
        // edges so it frames the conversation instead of running through it.
        const narrow = width < 640;
        const lane = Math.min(width * (narrow ? 0.9 : 0.62), 920);
        const start = Math.max(0, (width / 2 - lane * 0.78) / width);
        const end = Math.min(1, (width / 2 + lane * 0.78) / width);
        laneMask = ctx.createLinearGradient(0, 0, width, 0);
        laneMask.addColorStop(0, 'rgba(0, 0, 0, 0)');
        laneMask.addColorStop(start, 'rgba(0, 0, 0, 0)');
        laneMask.addColorStop(0.5, 'rgba(0, 0, 0, ' + (narrow ? 0.5 : 0.72) + ')');
        laneMask.addColorStop(end, 'rgba(0, 0, 0, 0)');
        laneMask.addColorStop(1, 'rgba(0, 0, 0, 0)');

        edgeMask = ctx.createLinearGradient(0, 0, 0, height);
        edgeMask.addColorStop(0, 'rgba(0, 0, 0, 0.78)');
        edgeMask.addColorStop(0.1, 'rgba(0, 0, 0, 0)');
        edgeMask.addColorStop(0.88, 'rgba(0, 0, 0, 0)');
        edgeMask.addColorStop(1, 'rgba(0, 0, 0, 0.55)');
    }

    function updateBounds() {
        if (!canvas || !ctx || !chatBox) return false;

        const rect = chatBox.getBoundingClientRect();
        const nextWidth = Math.round(rect.width);
        const nextHeight = Math.round(rect.height);

        if (!nextWidth || !nextHeight) {
            canvas.style.width = '0';
            canvas.style.height = '0';
            return false;
        }

        const dpr = Math.min(window.devicePixelRatio || 1, 2);
        width = nextWidth;
        height = nextHeight;

        canvas.style.left = Math.round(rect.left) + 'px';
        canvas.style.top = Math.round(rect.top) + 'px';
        canvas.style.width = width + 'px';
        canvas.style.height = height + 'px';
        canvas.width = Math.round(width * dpr);
        canvas.height = Math.round(height * dpr);

        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx.textBaseline = 'top';
        buildMasks();
        return true;
    }

    function spawnDrop(plane, scatter) {
        const layer = plane.layer;
        const trail = Math.round(rand(layer.trail[0], layer.trail[1]));
        return {
            col: Math.floor(Math.random() * plane.cols),
            head: scatter ? rand(-trail * 0.5, plane.rows + trail * 0.5) : -rand(0, plane.rows * 0.35),
            speed: rand(layer.speed[0], layer.speed[1]),
            trail: trail,
            wait: scatter ? 0 : rand(0.15, 2.4)
        };
    }

    function layoutPlane(plane) {
        const size = width < 640 ? plane.layer.mobileSize : plane.layer.size;
        const cellW = Math.max(9, Math.round(size * 0.86));
        const cellH = Math.round(size * 1.08);
        const cols = Math.ceil(width / cellW) + 1;
        const rows = Math.ceil(height / cellH) + 1;
        if (plane.cols === cols && plane.rows === rows && plane.size === size) return;

        plane.size = size;
        plane.cellW = cellW;
        plane.cellH = cellH;
        plane.cols = cols;
        plane.rows = rows;
        plane.font = plane.layer.font.replace('{size}', size);
        plane.glyphs = Array.from({ length: cols * rows }, randomChar);

        const count = Math.max(1, Math.round(cols * plane.layer.density));
        plane.drops = plane.drops.slice(0, count);
        plane.drops.forEach(function (drop) { drop.col %= cols; });
        while (plane.drops.length < count) plane.drops.push(spawnDrop(plane, true));
    }

    function initPlanes(force) {
        if (!width || !height) return;
        if (force || !planes.length) {
            planes = LAYERS.map(function (layer) { return { layer: layer, drops: [] }; });
        }
        planes.forEach(layoutPlane);
    }

    function stepPlane(plane, delta, speedMul) {
        for (let i = 0; i < plane.drops.length; i += 1) {
            const drop = plane.drops[i];
            if (drop.wait > 0) {
                drop.wait -= delta;
                continue;
            }
            drop.head += drop.speed * speedMul * delta;
            if (drop.head - drop.trail > plane.rows) {
                plane.drops[i] = spawnDrop(plane, false);
            }
        }
    }

    function drawPlane(plane, alphaMul) {
        const layer = plane.layer;
        ctx.font = plane.font;

        for (let d = 0; d < plane.drops.length; d += 1) {
            const drop = plane.drops[d];
            if (drop.wait > 0) continue;

            const headRow = Math.floor(drop.head);
            const x = drop.col * plane.cellW;

            for (let i = drop.trail; i >= 0; i -= 1) {
                const row = headRow - i;
                if (row < 0 || row >= plane.rows) continue;

                const idx = row * plane.cols + drop.col;
                if (Math.random() < (i === 0 ? 0.3 : MUTATE_CHANCE)) plane.glyphs[idx] = randomChar();
                const y = row * plane.cellH;

                if (i === 0) {
                    ctx.shadowBlur = layer.glow * alphaMul;
                    ctx.shadowColor = 'rgba(' + layer.color + ', 0.55)';
                    ctx.fillStyle = 'rgba(232, 255, 222, ' + Math.min(1, layer.head * alphaMul).toFixed(3) + ')';
                    ctx.fillText(plane.glyphs[idx], x, y);
                    ctx.shadowBlur = 0;
                    continue;
                }

                const fade = 1 - i / (drop.trail + 1);
                const alpha = layer.alpha * alphaMul * fade * fade;
                if (alpha < 0.008) continue;
                ctx.fillStyle = 'rgba(' + layer.color + ', ' + alpha.toFixed(3) + ')';
                ctx.fillText(plane.glyphs[idx], x, y);
            }
        }
    }

    function render(delta) {
        // Ease toward a quicker, brighter rain while the agent is working.
        const target = isBusy() ? 1 : 0;
        busyLevel += (target - busyLevel) * Math.min(1, delta * 1.6);
        const speedMul = 1 + busyLevel * (BUSY_SPEED - 1);
        const alphaMul = 1 + busyLevel * (BUSY_GLOW - 1);

        ctx.clearRect(0, 0, width, height);
        for (let i = 0; i < planes.length; i += 1) {
            if (delta > 0) stepPlane(planes[i], delta, speedMul);
            drawPlane(planes[i], alphaMul);
        }

        ctx.save();
        ctx.globalCompositeOperation = 'destination-out';
        ctx.fillStyle = laneMask;
        ctx.fillRect(0, 0, width, height);
        ctx.fillStyle = edgeMask;
        ctx.fillRect(0, 0, width, height);
        ctx.restore();
    }

    function frame(now) {
        if (!active || !ctx) return;

        if (now - lastFrame < 1000 / FPS - 1) {
            animationId = window.requestAnimationFrame(frame);
            return;
        }

        const delta = Math.min((now - lastFrame) / 1000, 0.1) || (1 / FPS);
        lastFrame = now;
        render(delta);

        animationId = window.requestAnimationFrame(frame);
    }

    function startLoop() {
        if (animationId || !animated || document.hidden) return;
        lastFrame = performance.now();
        animationId = window.requestAnimationFrame(frame);
    }

    function stopLoop() {
        if (animationId) {
            window.cancelAnimationFrame(animationId);
            animationId = null;
        }
    }

    function refresh() {
        if (!active) {
            if (shouldRun()) start();
            return;
        }
        if (!updateBounds()) return;
        initPlanes(false);
        // With reduced motion the field is a still frame; repaint it at the new size.
        if (!animated) render(0);
    }

    function attachObservers() {
        if (listenersAttached) return;
        listenersAttached = true;

        window.addEventListener('resize', refresh);
        window.addEventListener('aurago:themechange', sync);

        // A hidden tab has nothing to look at, so it should not keep painting.
        document.addEventListener('visibilitychange', function () {
            if (!active) return;
            if (document.hidden) stopLoop();
            else startLoop();
        });

        const motionQuery = window.matchMedia ? window.matchMedia('(prefers-reduced-motion: reduce)') : null;
        if (motionQuery && typeof motionQuery.addEventListener === 'function') {
            motionQuery.addEventListener('change', function () {
                if (active) stop();
                sync();
            });
        }

        if (window.ResizeObserver) {
            resizeObserver = new ResizeObserver(refresh);
        }

        themeObserver = new MutationObserver(sync);
        themeObserver.observe(document.documentElement, {
            attributes: true,
            attributeFilter: ['data-theme']
        });
    }

    function start() {
        ensureCanvas();
        if (!ctx) return;

        chatBox = document.getElementById('chat-box');
        if (!chatBox) return;

        attachObservers();

        // Re-observing fires the callback again, which would re-enter start()
        // every frame while the chat box has no size yet.
        if (resizeObserver && observedBox !== chatBox) {
            resizeObserver.disconnect();
            resizeObserver.observe(chatBox);
            observedBox = chatBox;
        }

        if (!updateBounds()) return;

        initPlanes(true);
        active = true;
        animated = !prefersReducedMotion();
        busyLevel = isBusy() ? 1 : 0;
        canvas.style.display = 'block';

        if (animated) startLoop();
        else render(0);
    }

    function stop() {
        active = false;
        stopLoop();
        if (canvas) {
            canvas.style.display = 'none';
            if (ctx && width && height) {
                ctx.clearRect(0, 0, width, height);
            }
        }
    }

    function sync() {
        if (shouldRun()) {
            if (!active) {
                start();
                return;
            }

            refresh();
            return;
        }

        if (active) stop();
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', sync);
    } else {
        sync();
    }
    attachObservers();
})();
