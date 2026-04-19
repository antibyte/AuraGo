(function () {
    'use strict';

    let canvas;
    let ctx;
    let chatBox;
    let resizeObserver = null;
    let animationId = null;
    let active = false;
    let lastTime = 0;
    let particleCount = 0;
    let trailCount = 0;

    let px = [];
    let py = [];
    let pvx = [];
    let pvy = [];
    let ps = [];
    let pa = [];
    let pd = [];
    let pk = [];

    let tx = [];
    let ty = [];
    let tvx = [];
    let tvy = [];
    let tl = [];
    let ta = [];
    let td = [];

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'sandstorm' &&
            !prefersReducedMotion() &&
            window.innerWidth >= 640;
    }

    function rand(min, max) {
        return min + Math.random() * (max - min);
    }

    function ensureCanvas() {
        if (canvas) return;
        canvas = document.createElement('canvas');
        canvas.id = 'sandstorm-overlay';
        Object.assign(canvas.style, {
            position: 'fixed',
            top: '0',
            left: '0',
            width: '0',
            height: '0',
            pointerEvents: 'none',
            zIndex: '2',
            opacity: '1',
            mixBlendMode: 'screen',
            display: 'none'
        });
        document.body.appendChild(canvas);
        ctx = canvas.getContext('2d', {
            alpha: true,
            desynchronized: true
        });
    }

    function updateBounds() {
        if (!canvas || !chatBox) return false;
        const rect = chatBox.getBoundingClientRect();
        if (!rect.width || !rect.height) {
            canvas.style.width = '0';
            canvas.style.height = '0';
            return false;
        }
        canvas.style.left = `${Math.round(rect.left)}px`;
        canvas.style.top = `${Math.round(rect.top)}px`;
        canvas.style.width = `${Math.round(rect.width)}px`;
        canvas.style.height = `${Math.round(rect.height)}px`;
        return true;
    }

    function resetParticle(i, width, height, fromEdge) {
        px[i] = fromEdge ? rand(-width * 0.18, width * 0.08) : rand(-width * 0.06, width * 1.04);
        py[i] = rand(-height * 0.08, height * 1.02);
        pvx[i] = rand(0.18, 0.55);
        pvy[i] = rand(-0.06, 0.12);
        ps[i] = rand(0.8, 2.8);
        pa[i] = rand(0.18, 0.62);
        pd[i] = rand(0.65, 1.45);
        pk[i] = Math.random() > 0.82 ? 1 : 0;
    }

    function resetTrail(i, width, height, fromEdge) {
        tx[i] = fromEdge ? rand(-width * 0.24, width * 0.05) : rand(-width * 0.08, width * 1.02);
        ty[i] = rand(-height * 0.1, height * 1.02);
        tvx[i] = rand(0.55, 1.25);
        tvy[i] = rand(-0.1, 0.1);
        tl[i] = rand(10, 26);
        ta[i] = rand(0.07, 0.18);
        td[i] = rand(0.8, 1.4);
    }

    function rebuildParticlePool(width, height) {
        const area = width * height;
        particleCount = Math.max(120, Math.min(280, Math.round(area / 4200)));
        trailCount = Math.max(24, Math.min(68, Math.round(area / 18000)));

        px = new Array(particleCount);
        py = new Array(particleCount);
        pvx = new Array(particleCount);
        pvy = new Array(particleCount);
        ps = new Array(particleCount);
        pa = new Array(particleCount);
        pd = new Array(particleCount);
        pk = new Array(particleCount);

        tx = new Array(trailCount);
        ty = new Array(trailCount);
        tvx = new Array(trailCount);
        tvy = new Array(trailCount);
        tl = new Array(trailCount);
        ta = new Array(trailCount);
        td = new Array(trailCount);

        for (let i = 0; i < particleCount; i++) {
            resetParticle(i, width, height, false);
        }
        for (let i = 0; i < trailCount; i++) {
            resetTrail(i, width, height, false);
        }
    }

    function resize() {
        ensureCanvas();
        if (!ctx || !updateBounds()) return;
        const dpr = Math.min(window.devicePixelRatio || 1, 1.75);
        const rect = canvas.getBoundingClientRect();
        const width = Math.max(1, Math.round(rect.width));
        const height = Math.max(1, Math.round(rect.height));
        canvas.width = Math.floor(width * dpr);
        canvas.height = Math.floor(height * dpr);
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        rebuildParticlePool(width, height);
    }

    function drawAmbientHaze(width, height, time) {
        const gustOffset = Math.sin(time * 0.00018) * width * 0.06;
        const rightOffset = Math.cos(time * 0.00013) * width * 0.05;

        const leftGust = ctx.createRadialGradient(
            width * 0.18 + gustOffset,
            height * 0.34,
            width * 0.04,
            width * 0.18 + gustOffset,
            height * 0.34,
            width * 0.36
        );
        leftGust.addColorStop(0, 'rgba(240, 210, 163, 0.16)');
        leftGust.addColorStop(0.46, 'rgba(214, 171, 114, 0.08)');
        leftGust.addColorStop(1, 'rgba(214, 171, 114, 0)');
        ctx.fillStyle = leftGust;
        ctx.fillRect(0, 0, width, height);

        const rightGust = ctx.createRadialGradient(
            width * 0.82 + rightOffset,
            height * 0.58,
            width * 0.03,
            width * 0.82 + rightOffset,
            height * 0.58,
            width * 0.28
        );
        rightGust.addColorStop(0, 'rgba(226, 191, 141, 0.12)');
        rightGust.addColorStop(0.42, 'rgba(193, 145, 89, 0.06)');
        rightGust.addColorStop(1, 'rgba(193, 145, 89, 0)');
        ctx.fillStyle = rightGust;
        ctx.fillRect(0, 0, width, height);
    }

    function updateAndDrawParticles(dt, time, width, height) {
        const baseWind = 0.42 + Math.sin(time * 0.00016) * 0.12;
        const verticalShear = Math.cos(time * 0.00011) * 0.04;

        for (let i = 0; i < particleCount; i++) {
            const swirl = Math.sin((py[i] * 0.018) + (time * 0.0013) + pd[i]) * 0.18 +
                Math.cos((px[i] * 0.007) - (time * 0.0009) + pd[i] * 1.4) * 0.08;
            const lift = Math.cos((px[i] * 0.01) + (time * 0.0008) + pd[i]) * 0.05;

            pvx[i] += ((baseWind + swirl) - pvx[i]) * 0.032 * dt;
            pvy[i] += ((verticalShear + lift) - pvy[i]) * 0.024 * dt;

            px[i] += pvx[i] * pd[i] * dt * 2.4;
            py[i] += pvy[i] * pd[i] * dt * 1.7;

            if (px[i] > width * 1.08 || py[i] < -height * 0.16 || py[i] > height * 1.12) {
                resetParticle(i, width, height, true);
            }

            if (pk[i] === 1) {
                ctx.strokeStyle = `rgba(242, 216, 177, ${pa[i] * 0.6})`;
                ctx.lineWidth = Math.max(0.7, ps[i] * 0.42);
                ctx.beginPath();
                ctx.moveTo(px[i], py[i]);
                ctx.lineTo(px[i] - pvx[i] * 8.5, py[i] - pvy[i] * 5.5);
                ctx.stroke();
            } else {
                ctx.fillStyle = `rgba(232, 199, 151, ${pa[i]})`;
                ctx.fillRect(px[i], py[i], ps[i], ps[i]);
            }
        }
    }

    function updateAndDrawTrails(dt, time, width, height) {
        for (let i = 0; i < trailCount; i++) {
            const gust = Math.sin((ty[i] * 0.01) + (time * 0.0011) + td[i]) * 0.35;
            tvx[i] += ((0.9 + gust) - tvx[i]) * 0.022 * dt;
            tvy[i] += (Math.cos((tx[i] * 0.006) + time * 0.0007 + td[i]) * 0.06 - tvy[i]) * 0.018 * dt;

            tx[i] += tvx[i] * dt * 2.6;
            ty[i] += tvy[i] * dt * 1.8;

            if (tx[i] > width * 1.1 || ty[i] < -height * 0.14 || ty[i] > height * 1.08) {
                resetTrail(i, width, height, true);
            }

            ctx.strokeStyle = `rgba(241, 212, 170, ${ta[i]})`;
            ctx.lineWidth = 1;
            ctx.beginPath();
            ctx.moveTo(tx[i], ty[i]);
            ctx.lineTo(tx[i] - tl[i], ty[i] - tvy[i] * 7.5);
            ctx.stroke();
        }
    }

    function render(time) {
        if (!active || !ctx) return;
        const rect = canvas.getBoundingClientRect();
        const width = Math.max(1, Math.round(rect.width));
        const height = Math.max(1, Math.round(rect.height));
        const dt = Math.min(1.8, (time - lastTime || 16.6) / 16.6);
        lastTime = time;

        ctx.clearRect(0, 0, width, height);
        drawAmbientHaze(width, height, time);
        updateAndDrawTrails(dt, time, width, height);
        updateAndDrawParticles(dt, time, width, height);

        animationId = window.requestAnimationFrame(render);
    }

    function start() {
        if (active || !shouldRun()) return;
        ensureCanvas();
        if (!ctx) return;
        active = true;
        canvas.style.display = 'block';
        resize();
        lastTime = 0;
        animationId = window.requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animationId) {
            window.cancelAnimationFrame(animationId);
            animationId = null;
        }
        if (canvas) {
            canvas.style.display = 'none';
        }
    }

    function sync() {
        if (shouldRun()) {
            start();
        } else {
            stop();
        }
    }

    function init() {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', init, { once: true });
            return;
        }

        chatBox = document.getElementById('chat-box');
        if (typeof ResizeObserver !== 'undefined' && chatBox) {
            resizeObserver = new ResizeObserver(() => {
                if (active) resize();
            });
            resizeObserver.observe(chatBox);
        }

        window.addEventListener('aurago:themechange', sync);
        window.addEventListener('resize', () => {
            if (active) resize();
            sync();
        });
        document.addEventListener('visibilitychange', () => {
            if (document.hidden) {
                stop();
            } else {
                sync();
            }
        });

        if (window.matchMedia) {
            const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
            if (mq.addEventListener) {
                mq.addEventListener('change', sync);
            } else if (mq.addListener) {
                mq.addListener(sync);
            }
        }

        if (typeof MutationObserver !== 'undefined') {
            new MutationObserver(sync).observe(document.documentElement, {
                attributes: true,
                attributeFilter: ['data-theme']
            });
        }

        sync();
    }

    window.AuraGoSandstorm = { start, stop, sync };
    init();
})();
