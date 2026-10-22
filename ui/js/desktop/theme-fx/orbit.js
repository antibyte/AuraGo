(function () {
    'use strict';

    // Orbit effect (registered as "orbit" with AuraDesktopThemeFx). The orchestrator of the theme: it owns the lifecycle of
    // the live space scene (orbit-scene.js, only with the theme wallpaper), the Canvas 2D particle layer (orbit-particles.js)
    // and the window bridge (orbit-windows.js). With motion it forwards window events and the pointer, answers clicks on the
    // empty desktop with a sonar ping, and runs one animation loop only while something is moving; without motion (the theme
    // declares staticFx) it only keeps the scene rendering still frames. It pauses with a hidden tab and releases everything
    // in stop(). inspect() and command() serve the browser tests (orbit.time, orbit.quality, orbit.capture, orbit.ping,
    // orbit.burst).
    const fx = window.AuraDesktopThemeFx;
    if (!fx) return;

    const VENDOR_URL = '/js/vendor/desktop-themes/orbit-runtime.js';
    // A click on the bare desktop (.vd-workspace) pings, unless it lands on any of these (the check of Voxel World's click burst).
    const PING_BLOCKERS = '.vd-window, .vd-icon, .vd-widget, .vd-taskbar, .vd-dock, .vd-start-menu, .vd-context-menu, .vd-modal-backdrop, .vd-toast-container, .vd-global-menu, .vd-radial-menu, .vd-sip-phone-gadget, button, a, input, textarea, select, [contenteditable]';
    const PING_INTERVAL_MS = 350;
    // A touch pings only as a tap: it must end within TAP_MAX_MS and move less than TAP_SLOP_PX, else it was the start of a pan.
    const TAP_MAX_MS = 500;
    const TAP_SLOP_PX = 8;
    const STARDUST_STEP = 18;
    const COVERAGE_INTERVAL_MS = 100;
    const COVERAGE_SETTLE_MS = 450;
    const MAX_DPR = 1.5;

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }

    function start({ layer, signal, options }) {
        const motion = !!(options && options.motion);
        const fullWallpaper = !!(options && options.wallpaper === 'orbit');
        const choreo = window.AuraOrbitChoreo;
        const Particles = window.AuraOrbitParticles;
        const Windows = window.AuraOrbitWindows;

        let stopped = false;
        let paused = false;
        let sceneReady = false;
        let lastError = '';
        let scene = null;
        let raf = 0;
        let lastFrame = 0;
        let lastPingAt = 0;
        let lastCoverageAt = 0;
        let coverageTimer = 0;
        let tap = null;                       // the touch that might become a tap: { id, x, y, at }
        let pointerFrame = 0;
        let stardustFrom = null;
        let stardustCarry = 0;
        let canvas = null;
        let context = null;
        let canvasDpr = 1;
        let system = null;
        let bridge = null;
        const pointer = { x: 0, y: 0, nx: 0, ny: 0, inside: false };

        function noteError(stage, error) {
            const message = error && error.message ? error.message : String(error || 'unknown failure');
            lastError = String(stage) + ': ' + message.slice(0, 240);
            if (window.console && typeof window.console.warn === 'function') console.warn('[orbit FX] ' + lastError);
        }

        function isCurrent() { return !stopped && !(signal && signal.aborted); }

        // ---- Particle layer ----------------------------------------------------------------------
        function sizeCanvas() {
            if (!canvas) return;
            canvasDpr = Math.min(window.devicePixelRatio || 1, MAX_DPR);
            canvas.width = Math.max(1, Math.round((layer.clientWidth || window.innerWidth || 1) * canvasDpr));
            canvas.height = Math.max(1, Math.round((layer.clientHeight || window.innerHeight || 1) * canvasDpr));
        }

        function busy() {
            if (!system) return false;
            const stats = system.stats();
            return stats.live > 0 || stats.rings > 0;
        }

        function frame(now) {
            raf = 0;
            if (stopped || paused) return;
            const dt = lastFrame ? clamp((now - lastFrame) / 1000, 0, 0.05) : 1 / 60;
            lastFrame = now;
            const tilting = bridge ? bridge.update(dt) : false;
            system.update(dt);
            Particles.drawCanvas2D(context, system, canvasDpr);
            if (busy() || tilting) raf = requestAnimationFrame(frame);
            else { lastFrame = 0; Particles.drawCanvas2D(context, system, canvasDpr); }
        }

        function kick() {
            if (stopped || paused || raf || !system) return;
            raf = requestAnimationFrame(frame);
        }

        // ---- Scene coverage ---------------------------------------------------------------------
        function snapshot() {
            const api = window.AuraDesktopWindowEffects;
            if (!api || typeof api.snapshot !== 'function') return [];
            try {
                const list = api.snapshot();
                return Array.isArray(list) ? list : [];
            } catch (_) { return []; }
        }

        // Share of the usable desktop (the viewport without the taskbar band) the largest visible window covers; a window
        // that hides the scene slows it down. A maximized window stops short of the floating taskbar, so it counts as covering.
        function refreshCoverage(force) {
            if (!scene) return;
            const now = performance.now();
            if (!force && now - lastCoverageAt < COVERAGE_INTERVAL_MS) return;
            lastCoverageAt = now;
            const viewW = Math.max(1, window.innerWidth), viewH = Math.max(1, window.innerHeight);
            let usableH = viewH;
            const bar = document.querySelector('.vd-taskbar');
            if (bar && !bar.hidden) {
                const b = bar.getBoundingClientRect();
                if (b.height > 0 && b.top > viewH / 2) usableH = Math.max(1, b.top - (viewH - b.bottom));
            }
            let largest = 0;
            for (const record of snapshot()) {
                if (!record || record.visible === false || record.minimized) continue;
                // the live element when there is one: a window may be mid-animation or resized from outside
                const r = record.element && typeof record.element.getBoundingClientRect === 'function' ? record.element.getBoundingClientRect() : record.rect;
                if (!r) continue;
                const x = Number(r.x ?? r.left), y = Number(r.y ?? r.top);
                const w = Math.min(x + r.width, viewW) - Math.max(x, 0);
                const h = Math.min(y + r.height, viewH) - Math.max(y, 0);
                if (w > 0 && h > 0) largest = Math.max(largest, w * h);
            }
            scene.setState({ coverage: Math.min(1, largest / (viewW * usableH)) });
        }

        // ---- Events ------------------------------------------------------------------------------
        function onWindowEvent(event) {
            const detail = event && event.detail;
            if (!detail) return;
            if (bridge) {
                bridge.handle(detail);
                kick();
            }
            refreshCoverage(detail.phase !== 'drag' && detail.phase !== 'resize');
            // a maximize or a snap finishes its transition after the event: measure again once it has settled
            if (coverageTimer) clearTimeout(coverageTimer);
            coverageTimer = setTimeout(() => { coverageTimer = 0; if (isCurrent()) refreshCoverage(true); }, COVERAGE_SETTLE_MS);
        }

        function overEmptyDesktop(target) {
            return target instanceof Element && !target.closest(PING_BLOCKERS) && !!target.closest('.vd-workspace');
        }

        function playPing(x) {
            if (!motion || paused || document.hidden || !window.DesktopSounds || typeof window.DesktopSounds.play !== 'function') return;
            const pan = clamp((x / Math.max(1, window.innerWidth)) * 2 - 1, -1, 1);
            try { window.DesktopSounds.play('orbit.ping', { variant: 'medium', strength: 0.8, pan }); } catch (_) { /* sound support is optional */ }
        }

        // True while the WebGL scene is on screen (it draws its own scanner pulse).
        function sceneShows() {
            if (!scene) return false;
            const state = scene.inspect();
            return state.backend === 'three-webgl' && state.ready && state.quality < 4;
        }

        function ping(x, y, force) {
            const now = performance.now();
            if (!force && now - lastPingAt < PING_INTERVAL_MS) return false;
            lastPingAt = now;
            if (system) {
                // One ring is enough: with the scene running its scanner pulse is the ring, the overlay keeps the sparks;
                // on the poster (no scene, software GL) the overlay ring stays.
                system.emitPing({ x, y });
                if (sceneShows()) system.rings().pop();
                kick();
            }
            if (scene) scene.ping(x, y);
            playPing(x);
            return true;
        }

        function onPointerDown(event) {
            if (event.pointerType === 'touch') tap = null;   // any new touch voids a pending tap (a second finger is a gesture)
            if (paused || event.button !== 0 || !overEmptyDesktop(event.target)) return;
            if (event.pointerType !== 'touch') ping(event.clientX, event.clientY, false);
            else if (event.isPrimary) tap = { id: event.pointerId, x: event.clientX, y: event.clientY, at: performance.now() };
        }

        function onPointerEnd(event) {
            if (!tap || tap.id !== event.pointerId) return;
            const { x, y, at } = tap;
            tap = null;
            if (event.type === 'pointerup' && performance.now() - at < TAP_MAX_MS && Math.hypot(event.clientX - x, event.clientY - y) < TAP_SLOP_PX) ping(x, y, false);
        }

        // The pointer variables of the skin and the scene parallax are written at most once per animation frame.
        function flushPointer() {
            pointerFrame = 0;
            if (stopped) return;
            const body = document.body;
            if (body) {
                body.style.setProperty('--vd-orbit-mx', (pointer.x / Math.max(1, window.innerWidth) * 100).toFixed(1) + '%');
                body.style.setProperty('--vd-orbit-my', (pointer.y / Math.max(1, window.innerHeight) * 100).toFixed(1) + '%');
            }
            if (scene) scene.setState({ pointer: pointer.inside ? { x: pointer.nx, y: pointer.ny } : null });
        }

        function onPointerMove(event) {
            if (tap && event.pointerId === tap.id && Math.hypot(event.clientX - tap.x, event.clientY - tap.y) >= TAP_SLOP_PX) tap = null;
            pointer.x = event.clientX;
            pointer.y = event.clientY;
            pointer.nx = clamp(event.clientX / Math.max(1, window.innerWidth) * 2 - 1, -1, 1);
            pointer.ny = clamp(event.clientY / Math.max(1, window.innerHeight) * 2 - 1, -1, 1);
            pointer.inside = true;
            if (!pointerFrame) pointerFrame = requestAnimationFrame(flushPointer);
            if (!system || paused || event.pointerType === 'touch' || !overEmptyDesktop(event.target)) { stardustFrom = null; return; }
            // one mote per STARDUST_STEP px of travel over the empty desktop
            if (stardustFrom) {
                const dx = event.clientX - stardustFrom.x, dy = event.clientY - stardustFrom.y;
                const travelled = Math.hypot(dx, dy);
                const dt = Math.max(0.008, (event.timeStamp - stardustFrom.at) / 1000);
                stardustCarry += travelled;
                let emitted = 0;
                while (stardustCarry >= STARDUST_STEP && emitted < 3) {
                    stardustCarry -= STARDUST_STEP;
                    emitted++;
                    system.emitStardust({ x: event.clientX - dx * stardustCarry / Math.max(travelled, 1), y: event.clientY - dy * stardustCarry / Math.max(travelled, 1) }, { x: dx / dt, y: dy / dt });
                }
                stardustCarry = Math.min(stardustCarry, STARDUST_STEP);
                if (emitted) kick();
            }
            stardustFrom = { x: event.clientX, y: event.clientY, at: event.timeStamp };
        }

        function onPointerLeave() {
            pointer.inside = false;
            stardustFrom = null;
            if (!pointerFrame && !stopped) pointerFrame = requestAnimationFrame(flushPointer);
        }

        // ---- Scene -------------------------------------------------------------------------------
        // The skin drops its backdrop blur at quality 2 and below (data-orbit-quality on the body): glass over a moving
        // scene is the costly part of a window frame on an integrated GPU.
        function publishQuality(level) {
            if (stopped || !document.body) return;
            document.body.dataset.orbitQuality = String(level);
        }

        function loadScene() {
            import(new URL(VENDOR_URL, window.location.href).href).then(vendor => {
                if (!isCurrent()) return;
                if (!window.AuraOrbitScene || !window.AuraOrbitShaders) throw new Error('scene modules missing');
                scene = window.AuraOrbitScene.create({ vendor, motion, choreo, shaders: window.AuraOrbitShaders, onQuality: publishQuality });
                if (paused) scene.pause();
                refreshCoverage(true);
                scene.ready.then(() => { sceneReady = true; });
            }).catch(error => {
                if (!isCurrent()) return;
                noteError('scene', error);
            });
        }

        // ---- Lifecycle ---------------------------------------------------------------------------
        function pause() {
            paused = true;
            if (raf) cancelAnimationFrame(raf);
            raf = 0;
            lastFrame = 0;
            if (scene) scene.pause();
        }

        function resume() {
            if (stopped || !paused) return;
            paused = false;
            if (scene) scene.resume();
            kick();
        }

        function stop() {
            if (stopped) return;
            stopped = true;
            if (raf) cancelAnimationFrame(raf);
            if (pointerFrame) cancelAnimationFrame(pointerFrame);
            if (coverageTimer) clearTimeout(coverageTimer);
            coverageTimer = 0;
            raf = pointerFrame = 0;
            if (bridge) bridge.dispose();
            if (scene) { try { scene.dispose(); } catch (_) { /* best effort */ } }
            scene = null;
            bridge = null;
            system = null;
            if (canvas) canvas.remove();
            canvas = context = null;
            const body = document.body;
            if (body) {
                body.style.removeProperty('--vd-orbit-mx');
                body.style.removeProperty('--vd-orbit-my');
                delete body.dataset.orbitQuality;
            }
        }

        function inspect() {
            return {
                ready: !fullWallpaper || sceneReady,
                motion,
                wallpaper: fullWallpaper ? 'orbit' : 'other',
                scene: scene ? scene.inspect() : null,
                particles: system ? system.stats() : null,
                windows: bridge ? bridge.inspect() : null,
                error: lastError
            };
        }

        function command(name, payload) {
            if (name === 'orbit.time' || name === 'orbit.quality' || name === 'orbit.capture') return scene ? scene.command(name, payload) : null;
            if (name === 'orbit.ping') {
                const x = Number(payload && payload.x), y = Number(payload && payload.y);
                if (!Number.isFinite(x) || !Number.isFinite(y)) return null;
                return ping(x, y, true);
            }
            if (name === 'orbit.burst') {
                if (!system || !payload) return null;
                const record = snapshot().find(item => item && String(item.id) === String(payload.windowId));
                const r = record && record.rect;
                if (!r) return null;
                const rect = { x: Number(r.x), y: Number(r.y), width: Number(r.width), height: Number(r.height) };
                if (payload.kind === 'materialize') system.emitMaterialize(rect);
                else if (payload.kind === 'dissolve') system.emitDissolve(rect);
                else return null;
                kick();
                return true;
            }
            return null;
        }

        if (motion) {
            canvas = document.createElement('canvas');
            canvas.className = 'vd-fx-orbit-particles';
            canvas.setAttribute('aria-hidden', 'true');
            canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;display:block;pointer-events:none;';
            layer.appendChild(canvas);
            context = canvas.getContext('2d', { alpha: true });
            sizeCanvas();
            system = Particles.createSystem({ capacity: 900, seed: 7 });
            bridge = Windows.create({ particles: system, choreo, onSound: playPing });
            const listen = { passive: true, signal };
            window.addEventListener('resize', sizeCanvas, listen);
            window.addEventListener('aurago:desktop-window', onWindowEvent, listen);
            window.addEventListener('aurago:desktop-windows-refresh', () => refreshCoverage(true), listen);
            window.addEventListener('pointermove', onPointerMove, listen);
            document.addEventListener('pointerdown', onPointerDown, listen);
            document.addEventListener('pointerup', onPointerEnd, listen);
            document.addEventListener('pointercancel', onPointerEnd, listen);
            document.documentElement.addEventListener('pointerleave', onPointerLeave, listen);
        }
        if (signal && typeof signal.addEventListener === 'function') signal.addEventListener('abort', stop, { once: true });
        if (fullWallpaper) loadScene();
        return { pause, resume, stop, inspect, command };
    }

    fx.register('orbit', { start });
}());
