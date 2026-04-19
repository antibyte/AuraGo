(function () {
    'use strict';

    // ─── Configuration ───
    const MAX_CLOUDS = 6;
    const MAX_FLYING = 300;
    const MAX_SETTLED = 600;
    const MAX_TRAILS = 50;

    const GRAVITY = 0.08;
    const BASE_WIND = 0.55;
    const GUST_STRENGTH = 2.8;
    const GUST_INTERVAL_MIN = 4000;
    const GUST_INTERVAL_MAX = 9000;
    const GUST_DURATION = 1800;
    const SETTLE_CHANCE = 0.35;
    const SETTLED_FADE_CHANCE = 0.003;
    const DOM_SCAN_INTERVAL = 400;

    // ─── State ───
    let canvas, ctx;
    let chatBox;
    let resizeObserver = null;
    let animationId = null;
    let active = false;
    let lastTime = 0;
    let lastDomScan = 0;

    // Flying particles
    let fx = [], fy = [], fvx = [], fvy = [], fs = [], fa = [], fd = [], fk = [];
    let fCount = 0;

    // Trail particles (streaks)
    let tx = [], ty = [], tvx = [], tvy = [], tl = [], ta = [], td = [];
    let tCount = 0;

    // Settled particles (sand piled on elements)
    let sx = [], sy = [], ss = [], sa = [], ssid = [];
    let sCount = 0;

    // Background clouds
    let cx = [], cy = [], cr = [], ca = [], cvx = [], cvy = [];
    let cCount = 0;

    // Surface rects where sand can settle (relative to chat-box)
    let surfaces = [];

    // Wind / gust state
    let gustActive = false;
    let gustEndTime = 0;
    let nextGustTime = 0;
    let windOffset = 0;

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
        ctx = canvas.getContext('2d', { alpha: true, desynchronized: true });
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

    // ─── Surface scanning ───
    function scanSurfaces(width, height) {
        if (!chatBox) return;
        const boxRect = chatBox.getBoundingClientRect();
        const newSurfaces = [];

        const selectors = [
            '.bubble',
            '.app-header',
            '.app-footer',
            '.input-wrap',
            '.btn-send',
            '.avatar',
            '.greeting-icon',
            '.composer-panel',
            '.attachments-panel'
        ];

        selectors.forEach(sel => {
            document.querySelectorAll(sel).forEach(el => {
                const r = el.getBoundingClientRect();
                const top = r.top - boxRect.top;
                const left = r.left - boxRect.left;
                const right = left + r.width;
                const bottom = top + r.height;
                // Keep surfaces even if partially outside canvas so sand accumulates on visible edges
                if (right > -20 && left < width + 20 && bottom > -20 && top < height + 20) {
                    newSurfaces.push({
                        left: Math.max(-10, left),
                        right: Math.min(width + 10, right),
                        top: Math.max(-10, top),
                        bottom: Math.min(height + 10, bottom),
                        width: r.width,
                        height: r.height
                    });
                }
            });
        });

        surfaces = newSurfaces;
    }

    function findSurfaceHit(x, y, vy) {
        // Only settle when falling downward
        if (vy <= 0.05) return null;
        for (let i = 0; i < surfaces.length; i++) {
            const s = surfaces[i];
            if (x >= s.left && x <= s.right && y >= s.top - 3 && y <= s.top + 6) {
                return s;
            }
        }
        return null;
    }

    // ─── Particle spawners ───
    function spawnFlying(i, width, height, fromEdge) {
        fx[i] = fromEdge ? rand(-width * 0.2, width * 0.05) : rand(-width * 0.08, width * 1.06);
        fy[i] = fromEdge ? rand(-height * 0.1, height * 0.4) : rand(-height * 0.1, height * 1.04);
        fvx[i] = rand(0.25, 0.75);
        fvy[i] = rand(-0.2, 0.35);
        fs[i] = rand(0.7, 2.4);
        fa[i] = rand(0.2, 0.65);
        fd[i] = rand(0.7, 1.5);
        fk[i] = Math.random() > 0.78 ? 1 : 0;
    }

    function spawnTrail(i, width, height, fromEdge) {
        tx[i] = fromEdge ? rand(-width * 0.25, width * 0.04) : rand(-width * 0.1, width * 1.04);
        ty[i] = rand(-height * 0.1, height * 1.04);
        tvx[i] = rand(0.6, 1.4);
        tvy[i] = rand(-0.08, 0.08);
        tl[i] = rand(12, 32);
        ta[i] = rand(0.06, 0.16);
        td[i] = rand(0.8, 1.4);
    }

    function spawnCloud(i, width, height) {
        cx[i] = rand(-width * 0.3, width * 1.1);
        cy[i] = rand(-height * 0.1, height * 1.1);
        cr[i] = rand(width * 0.08, width * 0.28);
        ca[i] = rand(0.03, 0.09);
        cvx[i] = rand(0.02, 0.08);
        cvy[i] = rand(-0.01, 0.01);
    }

    function addSettled(x, y, surface) {
        if (sCount >= MAX_SETTLED) return;
        sx[sCount] = x + rand(-3, 3);
        sy[sCount] = surface.top - rand(0, 4);
        ss[sCount] = rand(1.2, 2.8);
        sa[sCount] = rand(0.35, 0.65);
        ssid[sCount] = Math.floor(rand(0, surfaces.length));
        sCount++;
    }

    function removeSettled(i) {
        if (i < sCount - 1) {
            sx[i] = sx[sCount - 1];
            sy[i] = sy[sCount - 1];
            ss[i] = ss[sCount - 1];
            sa[i] = sa[sCount - 1];
            ssid[i] = ssid[sCount - 1];
        }
        sCount--;
    }

    function convertSettledToFlying(i) {
        if (fCount >= MAX_FLYING) {
            removeSettled(i);
            return;
        }
        fx[fCount] = sx[i];
        fy[fCount] = sy[i] - rand(0, 3);
        fvx[fCount] = rand(1.2, 3.5) + (gustActive ? GUST_STRENGTH * 0.5 : 0);
        fvy[fCount] = rand(-1.2, -0.3);
        fs[fCount] = ss[i];
        fa[fCount] = sa[i];
        fd[fCount] = rand(0.8, 1.4);
        fk[fCount] = 0;
        fCount++;
        removeSettled(i);
    }

    // ─── Pools ───
    function rebuildPools(width, height) {
        const area = width * height;
        fCount = Math.max(80, Math.min(MAX_FLYING, Math.round(area / 5500)));
        tCount = Math.max(20, Math.min(MAX_TRAILS, Math.round(area / 22000)));
        cCount = Math.max(2, Math.min(MAX_CLOUDS, Math.round(area / 120000)));

        fx = new Float32Array(MAX_FLYING);
        fy = new Float32Array(MAX_FLYING);
        fvx = new Float32Array(MAX_FLYING);
        fvy = new Float32Array(MAX_FLYING);
        fs = new Float32Array(MAX_FLYING);
        fa = new Float32Array(MAX_FLYING);
        fd = new Float32Array(MAX_FLYING);
        fk = new Uint8Array(MAX_FLYING);

        tx = new Float32Array(MAX_TRAILS);
        ty = new Float32Array(MAX_TRAILS);
        tvx = new Float32Array(MAX_TRAILS);
        tvy = new Float32Array(MAX_TRAILS);
        tl = new Float32Array(MAX_TRAILS);
        ta = new Float32Array(MAX_TRAILS);
        td = new Float32Array(MAX_TRAILS);

        sx = new Float32Array(MAX_SETTLED);
        sy = new Float32Array(MAX_SETTLED);
        ss = new Float32Array(MAX_SETTLED);
        sa = new Float32Array(MAX_SETTLED);
        ssid = new Uint16Array(MAX_SETTLED);
        sCount = 0;

        cx = new Float32Array(MAX_CLOUDS);
        cy = new Float32Array(MAX_CLOUDS);
        cr = new Float32Array(MAX_CLOUDS);
        ca = new Float32Array(MAX_CLOUDS);
        cvx = new Float32Array(MAX_CLOUDS);
        cvy = new Float32Array(MAX_CLOUDS);

        for (let i = 0; i < fCount; i++) spawnFlying(i, width, height, false);
        for (let i = 0; i < tCount; i++) spawnTrail(i, width, height, false);
        for (let i = 0; i < cCount; i++) spawnCloud(i, width, height);
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
        rebuildPools(width, height);
        scanSurfaces(width, height);
    }

    // ─── Drawing helpers ───
    function drawClouds(width, height, time) {
        for (let i = 0; i < cCount; i++) {
            const g = ctx.createRadialGradient(cx[i], cy[i], 0, cx[i], cy[i], cr[i]);
            const alpha = ca[i] * (0.7 + Math.sin(time * 0.0003 + i * 2.1) * 0.3);
            g.addColorStop(0, `rgba(220, 190, 145, ${alpha})`);
            g.addColorStop(0.5, `rgba(190, 150, 100, ${alpha * 0.5})`);
            g.addColorStop(1, `rgba(160, 120, 75, 0)`);
            ctx.fillStyle = g;
            ctx.fillRect(cx[i] - cr[i], cy[i] - cr[i], cr[i] * 2, cr[i] * 2);
        }
    }

    function updateAndDrawClouds(dt, width, height) {
        for (let i = 0; i < cCount; i++) {
            cx[i] += (cvx[i] + windOffset * 0.3) * dt;
            cy[i] += cvy[i] * dt;
            if (cx[i] - cr[i] > width * 1.15) {
                cx[i] = -cr[i] - rand(width * 0.05, width * 0.2);
                cy[i] = rand(-height * 0.1, height * 1.1);
            }
        }
    }

    function updateAndDrawFlying(dt, time, width, height) {
        const gust = gustActive ? GUST_STRENGTH * (1 + Math.sin(time * 0.01) * 0.3) : 0;
        const baseWindLocal = BASE_WIND + Math.sin(time * 0.0002) * 0.15 + gust;

        for (let i = 0; i < fCount; i++) {
            const swirl = Math.sin((fy[i] * 0.016) + (time * 0.0012) + fd[i]) * 0.22 +
                Math.cos((fx[i] * 0.006) - (time * 0.0008) + fd[i] * 1.6) * 0.1;
            const lift = Math.cos((fx[i] * 0.009) + (time * 0.0007) + fd[i]) * 0.06;

            fvx[i] += ((baseWindLocal + swirl) - fvx[i]) * 0.035 * dt;
            fvy[i] += ((GRAVITY + lift) - fvy[i]) * 0.028 * dt;

            const nextX = fx[i] + fvx[i] * fd[i] * dt * 2.6;
            const nextY = fy[i] + fvy[i] * fd[i] * dt * 1.8;

            // Surface collision -> settle
            if (fvy[i] > 0.1 && !gustActive) {
                const hit = findSurfaceHit(nextX, nextY, fvy[i]);
                if (hit && Math.random() < SETTLE_CHANCE) {
                    addSettled(nextX, nextY, hit);
                    // respawn this flying particle from left edge
                    spawnFlying(i, width, height, true);
                    continue;
                }
            }

            fx[i] = nextX;
            fy[i] = nextY;

            if (fx[i] > width * 1.12 || fy[i] < -height * 0.18 || fy[i] > height * 1.15) {
                spawnFlying(i, width, height, true);
            }

            if (fk[i] === 1) {
                ctx.strokeStyle = `rgba(242, 216, 177, ${fa[i] * 0.55})`;
                ctx.lineWidth = Math.max(0.6, fs[i] * 0.4);
                ctx.beginPath();
                ctx.moveTo(fx[i], fy[i]);
                ctx.lineTo(fx[i] - fvx[i] * 9, fy[i] - fvy[i] * 6);
                ctx.stroke();
            } else {
                ctx.fillStyle = `rgba(232, 199, 151, ${fa[i]})`;
                ctx.fillRect(fx[i], fy[i], fs[i], fs[i]);
            }
        }
    }

    function updateAndDrawTrails(dt, time, width, height) {
        const gust = gustActive ? GUST_STRENGTH * 0.6 : 0;
        for (let i = 0; i < tCount; i++) {
            const swirl = Math.sin((ty[i] * 0.009) + (time * 0.001) + td[i]) * 0.4;
            tvx[i] += ((0.95 + gust + swirl) - tvx[i]) * 0.024 * dt;
            tvy[i] += (Math.cos((tx[i] * 0.005) + time * 0.0006 + td[i]) * 0.07 - tvy[i]) * 0.018 * dt;

            tx[i] += tvx[i] * dt * 2.8;
            ty[i] += tvy[i] * dt * 2;

            if (tx[i] > width * 1.12 || ty[i] < -height * 0.16 || ty[i] > height * 1.12) {
                spawnTrail(i, width, height, true);
            }

            ctx.strokeStyle = `rgba(241, 212, 170, ${ta[i]})`;
            ctx.lineWidth = 1;
            ctx.beginPath();
            ctx.moveTo(tx[i], ty[i]);
            ctx.lineTo(tx[i] - tl[i], ty[i] - tvy[i] * 8);
            ctx.stroke();
        }
    }

    function updateAndDrawSettled(dt, time, width, height) {
        // During gusts, blow settled sand away
        if (gustActive) {
            for (let i = sCount - 1; i >= 0; i--) {
                if (Math.random() < 0.08 * dt) {
                    convertSettledToFlying(i);
                }
            }
            return;
        }

        // Slowly fade some settled particles (wind erosion)
        for (let i = sCount - 1; i >= 0; i--) {
            if (Math.random() < SETTLED_FADE_CHANCE * dt) {
                removeSettled(i);
                continue;
            }
            const drift = Math.sin(time * 0.0005 + ssid[i]) * 0.3;
            const x = sx[i] + drift;
            const y = sy[i];
            const size = ss[i];
            // Main grain
            ctx.fillStyle = `rgba(195, 162, 118, ${sa[i] * 0.95})`;
            ctx.fillRect(x, y, size, size);
            // Brighter highlight for 3D mound feel
            ctx.fillStyle = `rgba(230, 202, 158, ${sa[i] * 0.45})`;
            ctx.fillRect(x + size * 0.1, y + size * 0.1, size * 0.5, size * 0.5);
            // Subtle shadow beneath
            ctx.fillStyle = `rgba(160, 128, 88, ${sa[i] * 0.3})`;
            ctx.fillRect(x + size * 0.3, y + size * 0.4, size * 0.6, size * 0.6);
        }
    }

    function drawAmbientHaze(width, height, time) {
        const gustOffset = Math.sin(time * 0.00016) * width * 0.07;
        const rightOffset = Math.cos(time * 0.00011) * width * 0.06;

        const leftGust = ctx.createRadialGradient(
            width * 0.15 + gustOffset, height * 0.32,
            width * 0.03, width * 0.15 + gustOffset, height * 0.32, width * 0.38
        );
        leftGust.addColorStop(0, 'rgba(240, 210, 163, 0.14)');
        leftGust.addColorStop(0.5, 'rgba(214, 171, 114, 0.06)');
        leftGust.addColorStop(1, 'rgba(214, 171, 114, 0)');
        ctx.fillStyle = leftGust;
        ctx.fillRect(0, 0, width, height);

        const rightGust = ctx.createRadialGradient(
            width * 0.85 + rightOffset, height * 0.6,
            width * 0.02, width * 0.85 + rightOffset, height * 0.6, width * 0.3
        );
        rightGust.addColorStop(0, 'rgba(226, 191, 141, 0.1)');
        rightGust.addColorStop(0.45, 'rgba(193, 145, 89, 0.05)');
        rightGust.addColorStop(1, 'rgba(193, 145, 89, 0)');
        ctx.fillStyle = rightGust;
        ctx.fillRect(0, 0, width, height);
    }

    function drawGustWarning(width, height, time) {
        if (!gustActive) return;
        const intensity = Math.min(1, (GUST_DURATION - (gustEndTime - time)) / 400);
        const fade = Math.min(1, (gustEndTime - time) / 600);
        const alpha = intensity * fade * 0.06;
        const grad = ctx.createLinearGradient(0, 0, width, 0);
        grad.addColorStop(0, `rgba(240, 210, 170, 0)`);
        grad.addColorStop(0.4, `rgba(230, 195, 145, ${alpha})`);
        grad.addColorStop(0.7, `rgba(220, 180, 120, ${alpha * 0.7})`);
        grad.addColorStop(1, `rgba(200, 160, 100, 0)`);
        ctx.fillStyle = grad;
        ctx.fillRect(0, 0, width, height);
    }

    // ─── Render loop ───
    function render(time) {
        if (!active || !ctx) return;
        const rect = canvas.getBoundingClientRect();
        const width = Math.max(1, Math.round(rect.width));
        const height = Math.max(1, Math.round(rect.height));
        const dt = Math.min(2.5, (time - lastTime || 16.6) / 16.6);
        lastTime = time;

        // Wind gust logic
        if (time >= nextGustTime && !gustActive) {
            gustActive = true;
            gustEndTime = time + GUST_DURATION;
            nextGustTime = time + GUST_DURATION + rand(GUST_INTERVAL_MIN, GUST_INTERVAL_MAX);
        }
        if (gustActive && time >= gustEndTime) {
            gustActive = false;
        }
        windOffset = gustActive ? GUST_STRENGTH : 0;

        // Periodic DOM scan
        if (time - lastDomScan > DOM_SCAN_INTERVAL) {
            scanSurfaces(width, height);
            lastDomScan = time;
        }

        ctx.clearRect(0, 0, width, height);

        drawClouds(width, height, time);
        updateAndDrawClouds(dt, width, height);
        drawAmbientHaze(width, height, time);
        updateAndDrawTrails(dt, time, width, height);
        updateAndDrawSettled(dt, time, width, height);
        updateAndDrawFlying(dt, time, width, height);
        drawGustWarning(width, height, time);

        animationId = window.requestAnimationFrame(render);
    }

    function start() {
        if (active || !shouldRun()) return;
        ensureCanvas();
        if (!ctx) return;
        active = true;
        canvas.style.display = 'block';
        resize();
        nextGustTime = performance.now() + rand(2000, 5000);
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
        if (chatBox) {
            chatBox.addEventListener('scroll', () => {
                if (active) {
                    const rect = canvas.getBoundingClientRect();
                    scanSurfaces(Math.max(1, Math.round(rect.width)), Math.max(1, Math.round(rect.height)));
                }
            }, { passive: true });
        }
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
