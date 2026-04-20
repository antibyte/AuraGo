(function () {
    'use strict';

    const THEME = 'black-matrix';
    const CHARSET = '01<>[]{}#*+-=ZXCVBNMアイウエオカキクケコサシスセソ';
    const FPS = 24;

    let canvas;
    let ctx;
    let chatBox;
    let animationId = null;
    let active = false;
    let columns = [];
    let resizeObserver = null;
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
        return currentTheme() === THEME && !prefersReducedMotion();
    }

    function randomChar() {
        return CHARSET[Math.floor(Math.random() * CHARSET.length)];
    }

    function ensureCanvas() {
        if (canvas) return;
        canvas = document.createElement('canvas');
        canvas.id = 'black-matrix-overlay';
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
        return true;
    }

    function createColumn(index, gap, fontSize) {
        return {
            x: index * gap + Math.random() * gap * 0.25,
            y: -Math.random() * height,
            speed: 28 + Math.random() * 46,
            length: 7 + Math.floor(Math.random() * 15),
            fontSize: fontSize
        };
    }

    function initColumns(force) {
        if (!width || !height) return;

        const fontSize = width < 640 ? 14 : 18;
        const gap = Math.max(12, Math.round(fontSize * 0.92));
        const count = Math.ceil(width / gap) + 1;

        if (!force && columns.length === count) return;

        columns = Array.from({ length: count }, function (_, index) {
            return createColumn(index, gap, fontSize);
        });
    }

    function resetColumn(column) {
        column.y = -Math.random() * height * 0.7;
        column.speed = 28 + Math.random() * 46;
        column.length = 7 + Math.floor(Math.random() * 15);
        column.fontSize = width < 640 ? 14 : 18;
    }

    function drawColumn(column) {
        const step = column.fontSize * 0.88;
        ctx.font = '600 ' + column.fontSize + 'px "Oxanium", "Cascadia Code", monospace';
        ctx.textBaseline = 'top';

        for (let i = 0; i < column.length; i += 1) {
            const y = column.y - i * step;
            if (y < -column.fontSize || y > height + column.fontSize) continue;

            if (i === 0) {
                ctx.shadowBlur = 14;
                ctx.shadowColor = 'rgba(146, 242, 109, 0.24)';
                ctx.fillStyle = 'rgba(232, 255, 226, 0.22)';
            } else {
                ctx.shadowBlur = 0;
                const alpha = Math.max(0.02, 0.1 - (i / column.length) * 0.08);
                ctx.fillStyle = 'rgba(123, 255, 91, ' + alpha.toFixed(3) + ')';
            }

            ctx.fillText(randomChar(), column.x, y);
        }
    }

    function frame(now) {
        if (!active || !ctx) return;

        if (now - lastFrame < 1000 / FPS) {
            animationId = window.requestAnimationFrame(frame);
            return;
        }

        const delta = Math.min((now - lastFrame) / 1000, 0.1) || (1 / FPS);
        lastFrame = now;

        ctx.clearRect(0, 0, width, height);

        const haze = ctx.createLinearGradient(0, 0, 0, height);
        haze.addColorStop(0, 'rgba(255, 255, 255, 0.015)');
        haze.addColorStop(0.25, 'rgba(0, 0, 0, 0)');
        haze.addColorStop(1, 'rgba(0, 0, 0, 0.06)');
        ctx.fillStyle = haze;
        ctx.fillRect(0, 0, width, height);

        for (let i = 0; i < columns.length; i += 1) {
            const column = columns[i];
            column.y += column.speed * delta;

            if (column.y - column.length * column.fontSize > height + 40) {
                resetColumn(column);
            }

            drawColumn(column);
        }

        animationId = window.requestAnimationFrame(frame);
    }

    function attachObservers() {
        if (listenersAttached) return;
        listenersAttached = true;

        window.addEventListener('resize', function () {
            if (!active) return;
            if (updateBounds()) initColumns(true);
        });

        window.addEventListener('aurago:themechange', sync);

        if (window.ResizeObserver) {
            resizeObserver = new ResizeObserver(function () {
                if (!active) return;
                if (updateBounds()) initColumns(true);
            });
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

        if (resizeObserver) {
            resizeObserver.disconnect();
            resizeObserver.observe(chatBox);
        }

        if (!updateBounds()) return;

        initColumns(true);
        active = true;
        canvas.style.display = 'block';
        lastFrame = performance.now();
        animationId = window.requestAnimationFrame(frame);
    }

    function stop() {
        active = false;
        if (animationId) {
            window.cancelAnimationFrame(animationId);
            animationId = null;
        }
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

            if (updateBounds()) initColumns(false);
            return;
        }

        if (active) stop();
    }

    document.addEventListener('DOMContentLoaded', sync);
    attachObservers();
})();
