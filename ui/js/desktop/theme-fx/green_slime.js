(function () {
    'use strict';

    // DOM bridge of the Green Slime effect. The green_slime_sim.js family owns all state and physics;
    // this file reads window geometry, masks and pointer input, hands them to the simulation, completes
    // the scene (occluders, backdrop) and gives it to the active renderer. A renderer is { kind,
    // render(scene), resize(cssWidth, cssHeight, dpr, scale), dispose() }: green_slime_gl.js (WebGL2, the
    // gel material with refraction of the real wallpaper) is tried first, green_slime_canvas2d.js is the
    // fallback (no WebGL2, failed build, lost context). There is only ever one canvas: the fallback
    // replaces the GL canvas with a fresh one, because a canvas never gives up its WebGL context.

    const MASK_SELECTORS = '.vd-icon, .vd-widget, .vd-taskbar, .vd-dock, .vd-start-menu, .vd-window-menu-popover, .vd-context-menu, .vd-context-submenu-popover, .vd-modal-backdrop, .vd-modal, .vd-dialog, [role="dialog"], dialog[open], .vd-toast-container, .vd-global-menu, .vd-radial-menu';
    // A press that lands in a window, icon, widget, taskbar, menu or dialog belongs to the user, not to the gel.
    const POKE_BLOCKERS = MASK_SELECTORS + ', .vd-window';
    const OCCLUDER_LIMIT = 24;
    const UNDER_LIMIT = 16;
    const BACKDROP_MAX_SIZE = 2048;
    // Adaptive quality: when the average frame takes longer than this over a window, render at a lower scale.
    const SLOW_FRAME_MS = 22;
    const GIVE_UP_FRAME_MS = 45;
    const STALL_MS = 1500;
    const QUALITY_WINDOW_MS = 2000;
    const LOW_QUALITY_SCALE = .75;

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }

    function rectFor(record) {
        const measured = record && record.element && typeof record.element.getBoundingClientRect === 'function'
            ? record.element.getBoundingClientRect()
            : record && record.rect;
        if (!measured) return null;
        const x = Number(measured.x ?? measured.left);
        const y = Number(measured.y ?? measured.top);
        const width = Number(measured.width);
        const height = Number(measured.height);
        return Number.isFinite(x) && Number.isFinite(y) && width > 0 && height > 0
            ? { x, y, width, height }
            : null;
    }

    function usable(record) {
        return !!record && record.visible !== false && !record.minimized;
    }

    // Keeps the nearest (max - 1) rectangles and merges the rest into one bounding box. A bounding box
    // hides more than it should, never less, so content stays protected when the list overflows.
    function capList(list, max, focus) {
        if (list.length <= max) return list;
        const cx = focus.x + focus.width / 2, cy = focus.y + focus.height / 2;
        const ranked = list.map(item => ({ item, d: Math.hypot(item.x + item.width / 2 - cx, item.y + item.height / 2 - cy) }))
            .sort((a, b) => a.d - b.d);
        const keep = ranked.slice(0, max - 1).map(entry => entry.item);
        let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
        for (const entry of ranked.slice(max - 1)) {
            x0 = Math.min(x0, entry.item.x); y0 = Math.min(y0, entry.item.y);
            x1 = Math.max(x1, entry.item.x + entry.item.width); y1 = Math.max(y1, entry.item.y + entry.item.height);
        }
        keep.push({ x: x0, y: y0, width: x1 - x0, height: y1 - y0, radius: 0 });
        return keep;
    }

    function start({ layer, signal, options }) {
        const motion = !!(options && options.motion);
        const slime = window.AuraGreenSlimeSim;
        if (!slime) return { stop() {} };
        let canvas = null;
        let renderer = null;
        let rendererError = '';
        let contextLost = false;
        // The backdrop the gel refracts: 'loading' until the wallpaper image is decoded, 'texture' with a
        // bitmap, otherwise 'translucent' (live wallpaper, overlay effect, gradient layers, upload failure).
        let backdrop = { state: 'translucent', reason: 'pending', image: null, naturalWidth: 0, naturalHeight: 0 };
        let backdropKey = '';
        let backdropToken = 0;
        let backdropTimer = 0;
        const settleTimers = new Set();
        let qualityScale = 1;
        let qualityLocked = false;
        let qualitySum = 0;
        let qualityCount = 0;
        let qualityStart = 0;
        let slowWindows = 0;
        let lastRafTime = 0;

        function newCanvas() {
            const element = document.createElement('canvas');
            element.className = 'vd-fx-slime-canvas';
            element.setAttribute('aria-hidden', 'true');
            element.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;display:block;pointer-events:none;';
            layer.appendChild(element);
            return element;
        }

        // WebGL2 first. A canvas that handed out a WebGL context cannot give it up, so whenever the GL
        // attempt does not end in a working renderer the fallback gets a fresh canvas.
        function chooseRenderer() {
            canvas = newCanvas();
            const gl = window.AuraGreenSlimeGL
                ? window.AuraGreenSlimeGL.create(canvas, {
                    motion, signal,
                    onContextLost: () => switchToFallback('context-lost'),
                    onFailure: reason => switchToFallback(reason),
                    // The programs build on the driver's own threads; the gel appears when they are done.
                    onReady: () => { dirty = true; requestFrame(); }
                })
                : null;
            if (gl) { renderer = gl; return; }
            rendererError = window.AuraGreenSlimeGL ? window.AuraGreenSlimeGL.lastError : 'green_slime_gl.js missing';
            canvas.remove();
            canvas = newCanvas();
            renderer = window.AuraGreenSlimeCanvas2D.create(canvas)
                || { kind: 'none', render() {}, resize() {}, dispose() { canvas.remove(); } };
        }

        function switchToFallback(reason) {
            if (stopped || !renderer || renderer.kind !== 'webgl2') return;
            contextLost = reason === 'context-lost';
            rendererError = reason;
            backdropToken++;
            backdrop = { state: 'translucent', reason, image: null, naturalWidth: 0, naturalHeight: 0 };
            try { renderer.dispose(); } catch (_) { /* the context is gone anyway */ }
            canvas = newCanvas();
            renderer = window.AuraGreenSlimeCanvas2D.create(canvas)
                || { kind: 'none', render() {}, resize() {}, dispose() { canvas.remove(); } };
            renderer.resize(width, height, dpr, 1);
            qualityScale = 1;
            dirty = true;
            requestFrame();
        }

        let width = 0;
        let height = 0;
        let dpr = 1;
        let raf = 0;
        let paused = false;
        let stopped = false;
        let lastTime = 0;
        let dirty = true;
        const records = new Map();
        const springs = new Set();
        const springAt = new Map();
        let frameCount = 0;
        let resizeObserver = null;
        let mutationObserver = null;
        let maskRectsDirty = true;
        let cachedOverlayMasks = [];
        let maskRefreshCount = 0;
        const sim = slime.createSim({ motion, seed: Date.now() >>> 0 });

        function invalidateMasks() {
            maskRectsDirty = true;
            dirty = true;
            requestFrame();
        }

        function resize() {
            width = Math.max(1, layer.clientWidth || window.innerWidth || 1);
            height = Math.max(1, layer.clientHeight || window.innerHeight || 1);
            dpr = Math.min(window.devicePixelRatio || 1, motion ? 1.5 : 1.25);
            renderer.resize(width, height, dpr, qualityScale);
            maskRectsDirty = true;
            dirty = true;
            requestFrame();
        }

        // ---- Backdrop -----------------------------------------------------------------------------
        // The gel refracts the wallpaper itself. That is only possible when the page paints exactly one
        // image as "center / cover no-repeat fixed" (the green_slime wallpaper) and nothing animated
        // sits between it and the effect layer; every other desktop gets the translucent material.
        function discoverBackdrop() {
            const body = document.body;
            if (!body || typeof window.getComputedStyle !== 'function') return { reason: 'no document' };
            const live = document.getElementById('vd-wallpaper-live');
            if (live && !live.hidden && window.getComputedStyle(live).display !== 'none') return { reason: 'live wallpaper' };
            const overlay = document.querySelector('.vd-wallpaper-fx');
            if (overlay && window.getComputedStyle(overlay).display !== 'none') return { reason: 'wallpaper overlay' };
            const style = window.getComputedStyle(body);
            const layers = String(style.backgroundImage || '').trim();
            const single = /^url\(\s*(["']?)(.*?)\1\s*\)$/.exec(layers);
            if (!single || /\)\s*,/.test(layers)) return { reason: 'not a single image' };
            if (style.backgroundSize !== 'cover' || style.backgroundAttachment !== 'fixed' || style.backgroundRepeat !== 'no-repeat'
                || !/^(50% 50%|center center|center)$/.test(String(style.backgroundPosition).trim())) {
                return { reason: 'unsupported background layout: ' + [style.backgroundSize, style.backgroundPosition, style.backgroundAttachment, style.backgroundRepeat].join(' | ') };
            }
            return { url: single[2] };
        }

        // Where the cover-fitted wallpaper is drawn on the viewport (CSS px), like the CSS background.
        function coverRect() {
            const iw = backdrop.naturalWidth, ih = backdrop.naturalHeight;
            if (!backdrop.image || !iw || !ih) return { x: 0, y: 0, w: width, h: height };
            const scale = Math.max(width / iw, height / ih);
            return { x: (width - iw * scale) / 2, y: (height - ih * scale) / 2, w: iw * scale, h: ih * scale };
        }

        function settleBackdrop(state, reason, image, naturalWidth, naturalHeight) {
            backdrop = { state, reason, image: image || null, naturalWidth: naturalWidth || 0, naturalHeight: naturalHeight || 0 };
            dirty = true;
            requestFrame();
        }

        function refreshBackdrop() {
            backdropTimer = 0;
            if (stopped || !renderer) return;
            const found = discoverBackdrop();
            const key = (renderer.kind === 'webgl2' ? 'gl:' : 'flat:') + (found.url || found.reason);
            if (key === backdropKey) return;
            backdropKey = key;
            const token = ++backdropToken;
            if (!found.url || renderer.kind !== 'webgl2') {
                settleBackdrop('translucent', found.reason || 'renderer without textures');
                return;
            }
            backdrop = { state: 'loading', reason: '', image: null, naturalWidth: 0, naturalHeight: 0 };
            const image = new Image();
            image.decoding = 'async';
            image.src = found.url;
            const ready = typeof image.decode === 'function' ? image.decode() : new Promise((resolve, reject) => { image.onload = resolve; image.onerror = reject; });
            ready.then(() => {
                if (token !== backdropToken || stopped) return null;
                const limit = Math.min(BACKDROP_MAX_SIZE, renderer.maxTextureSize || BACKDROP_MAX_SIZE);
                const shrink = Math.min(1, limit / Math.max(image.naturalWidth, image.naturalHeight));
                const target = shrink < 1
                    ? { resizeWidth: Math.round(image.naturalWidth * shrink), resizeHeight: Math.round(image.naturalHeight * shrink), resizeQuality: 'high' }
                    : undefined;
                return createImageBitmap(image, target);
            }).then(bitmap => {
                if (!bitmap) return;
                if (token !== backdropToken || stopped || renderer.kind !== 'webgl2') { if (bitmap.close) bitmap.close(); return; }
                renderer.setBackdrop(bitmap);
                if (renderer.inspect().backdropReady) settleBackdrop('texture', '', bitmap, image.naturalWidth, image.naturalHeight);
                else settleBackdrop('translucent', 'texture upload failed');
            }).catch(() => {
                if (token === backdropToken && !stopped) settleBackdrop('translucent', 'image unavailable');
            });
        }

        function scheduleBackdropCheck(delay) {
            if (backdropTimer || stopped) return;
            backdropTimer = window.setTimeout(refreshBackdrop, typeof delay === 'number' ? delay : 60);
        }

        // Stylesheets and wallpaper settings land a moment after the effect starts, and a changed computed
        // style raises no DOM mutation: look again a few times while the page settles.
        function verifyBackdropLater() {
            for (const delay of [350, 1200, 3500]) {
                const timer = window.setTimeout(() => { settleTimers.delete(timer); scheduleBackdropCheck(0); }, delay);
                settleTimers.add(timer);
            }
        }

        function snapshot() {
            const api = window.AuraDesktopWindowEffects;
            if (!api || typeof api.snapshot !== 'function') return;
            try {
                const values = api.snapshot();
                if (!Array.isArray(values)) return;
                const next = new Map();
                for (const record of values) {
                    if (!usable(record) || record.id == null) continue;
                    const rect = rectFor(record);
                    if (!rect) continue;
                    next.set(String(record.id), { ...record, rect });
                }
                records.clear();
                for (const [id, record] of next) records.set(id, record);
                for (const id of springAt.keys()) if (!records.has(id)) springAt.delete(id);
                sim.syncWindows(new Set(records.keys()));
                dirty = true;
            } catch (_) { /* the desktop may be tearing down */ }
        }

        function springControls(record) {
            if (!motion || !record || !record.element) return;
            const now = performance.now();
            const id = String(record.id || '');
            if (now - (springAt.get(id) || 0) < 520) return;
            springAt.set(id, now);
            for (const button of record.element.querySelectorAll('.vd-window-button')) {
                if (typeof button.animate !== 'function') continue;
                const animation = button.animate([
                    { transform: 'scale(.82)', offset: 0 },
                    { transform: 'scale(1.14)', offset: .32 },
                    { transform: 'scale(.96)', offset: .63 },
                    { transform: 'scale(1.025)', offset: .82 },
                    { transform: 'scale(1)', offset: 1 }
                ], { duration: 440, easing: 'cubic-bezier(.18,.8,.3,1)', fill: 'none' });
                springs.add(animation);
                animation.onfinish = animation.oncancel = () => springs.delete(animation);
            }
        }

        function onWindowEvent(event) {
            const record = event && event.detail;
            if (!record || record.id == null) return;
            const id = String(record.id);
            const rect = rectFor(record);
            const now = performance.now() / 1000;
            if (record.phase === 'close') {
                sim.windowEvent({ id, phase: 'close', rect, silent: !!record.silent }, now);
                records.delete(id);
                springAt.delete(id);
                dirty = true;
                requestFrame();
                return;
            }
            if (rect && usable(record)) {
                records.set(id, { ...record, rect });
                sim.windowEvent({ id, phase: record.phase, rect, velocity: record.velocity, silent: !!record.silent }, now);
            } else {
                records.delete(id);
                sim.removeWindow(id);
            }
            if (!record.silent && (record.phase === 'open' || record.phase === 'focus')) springControls(record);
            dirty = true;
            // Window-manager motion is already frame-batched. Paint its new bounds
            // now, even when our animation callback ran earlier in this frame.
            if (motion && /^(drag|resize|snap|maximize|restore)/.test(record.phase)) render(performance.now());
            requestFrame();
        }

        function overlayMasks() {
            if (!maskRectsDirty) return cachedOverlayMasks;
            maskRectsDirty = false;
            const masks = [];
            for (const element of document.querySelectorAll(MASK_SELECTORS)) {
                if (!element.isConnected || element.hidden) continue;
                const style = window.getComputedStyle ? window.getComputedStyle(element) : null;
                if (style && (style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0)) continue;
                const rect = element.getBoundingClientRect();
                if (rect.width > 0 && rect.height > 0 && rect.bottom >= 0 && rect.right >= 0 && rect.top <= height && rect.left <= width) {
                    masks.push({ x: rect.x, y: rect.y, width: rect.width, height: rect.height,
                        desktop: element.matches('.vd-icon, .vd-widget') && !element.classList.contains('vd-dragging'),
                        taskbar: element.matches('.vd-taskbar') });
                }
            }
            cachedOverlayMasks = masks;
            maskRefreshCount++;
            return cachedOverlayMasks;
        }

        function observeMaskChanges() {
            if (typeof MutationObserver !== 'function') return;
            const root = document.documentElement || document.body;
            if (!root) return;
            mutationObserver = new MutationObserver(mutations => {
                if (mutations.some(mutation => mutation.type === 'attributes' && (mutation.target === document.body || mutation.target === document.documentElement
                    || (mutation.target.id === 'vd-wallpaper-live' || (mutation.target.classList && mutation.target.classList.contains('vd-wallpaper-fx')))))) scheduleBackdropCheck();
                const relevant = mutations.some(mutation => {
                    const target = mutation.target;
                    if (target === canvas || (canvas.contains && canvas.contains(target))) return false;
                    if (mutation.type !== 'childList') return true;
                    const changed = [...mutation.addedNodes, ...mutation.removedNodes];
                    return changed.some(node => node !== canvas && !(canvas.contains && canvas.contains(node)));
                });
                if (relevant) invalidateMasks();
            });
            mutationObserver.observe(root, {
                subtree: true,
                childList: true,
                attributes: true,
                attributeFilter: ['class', 'style', 'hidden', 'open', 'aria-hidden', 'data-wallpaper']
            });
        }

        // Everything the simulation needs of the page for one step.
        function buildEnv(dt, time) {
            const all = Array.from(records.values()).sort((a, b) => (Number(a.z) || 0) - (Number(b.z) || 0)).map(record => {
                const rect = rectFor(record);
                if (!rect) return null;
                const style = record.element ? getComputedStyle(record.element) : null;
                const radii = style
                    ? [style.borderTopLeftRadius, style.borderTopRightRadius, style.borderBottomRightRadius, style.borderBottomLeftRadius].map(r => parseFloat(r) || 0)
                    : [22, 22, 22, 22];
                return { id: String(record.id), rect, radii, z: Number(record.z) || 0, active: !!record.focused };
            }).filter(Boolean);
            const masks = overlayMasks();
            return { dt, now: time / 1000, width, height, dpr, motion, windows: all, masks,
                taskbar: masks.find(mask => mask.taskbar) || null };
        }

        // Completes the simulation's scene with what only the page knows: which rectangles hide the
        // gel of each window, which lie beneath it, and the backdrop.
        function buildScene(env, scene) {
            const solid = env.masks.filter(mask => !mask.desktop).map(mask => ({ x: mask.x, y: mask.y, width: mask.width, height: mask.height, radius: 0 }));
            const desktop = env.masks.filter(mask => mask.desktop).map(mask => ({ x: mask.x, y: mask.y, width: mask.width, height: mask.height }));
            const frames = scene.windows.map(win => ({ x: win.rect.x, y: win.rect.y, width: win.rect.width, height: win.rect.height,
                radii: win.radii, radius: Math.max(...win.radii), z: win.z }));
            scene.windows.forEach(win => {
                win.occluders = capList(frames.filter(other => other.z >= win.z).concat(solid), OCCLUDER_LIMIT, win.rect);
                win.under = capList(frames.filter(other => other.z < win.z).map(other => ({ x: other.x, y: other.y, width: other.width, height: other.height })).concat(desktop), UNDER_LIMIT, win.rect);
            });
            scene.dropOccluders = frames.concat(env.masks.map(mask => ({ x: mask.x, y: mask.y, width: mask.width, height: mask.height, radius: 0, taskbar: mask.taskbar })));
            const textured = backdrop.state === 'texture' && !!backdrop.image;
            scene.backdrop = { mode: textured ? 'texture' : 'translucent', image: textured ? backdrop.image : null, rect: coverRect() };
            return scene;
        }

        // Frame-time watch (motion only, real animation frames only): when the average frame of a 2 s window
        // exceeds SLOW_FRAME_MS the gel renders at a lower scale for the rest of the session, and when even
        // that stays above GIVE_UP_FRAME_MS the Canvas 2D fallback takes over. Pauses reset the watch and
        // gaps beyond STALL_MS say nothing about the renderer.
        function watchFrameTime(time) {
            const interval = lastRafTime ? time - lastRafTime : 0;
            lastRafTime = time;
            if (qualityLocked || !renderer || renderer.kind !== 'webgl2' || interval <= 0 || interval > STALL_MS) return;
            if (!qualityStart) qualityStart = time;
            qualitySum += interval;
            qualityCount++;
            if (time - qualityStart < QUALITY_WINDOW_MS) return;
            const average = qualitySum / qualityCount;
            qualitySum = 0; qualityCount = 0; qualityStart = 0;
            // One slow window can be the desktop busy elsewhere (opening apps, layout); two in a row are not.
            slowWindows = average > (qualityScale === 1 ? SLOW_FRAME_MS : GIVE_UP_FRAME_MS) ? slowWindows + 1 : 0;
            if (slowWindows < 2) return;
            slowWindows = 0;
            if (qualityScale === 1) setQuality(LOW_QUALITY_SCALE);
            else switchToFallback('renderer too slow');
        }

        function setQuality(scale) {
            qualityScale = scale;
            renderer.resize(width, height, dpr, qualityScale);
            dirty = true;
        }

        function render(time) {
            if (stopped || paused) return;
            frameCount++;
            time = Math.max(time, lastTime);
            const dt = lastTime ? clamp((time - lastTime) / 1000, 0, .05) : 0;
            lastTime = time;
            if (!dirty && !motion) return;
            dirty = false;
            const env = buildEnv(dt, time);
            sim.update(env);
            renderer.render(buildScene(env, sim.scene(env)));
        }

        function requestFrame() {
            if (stopped || paused || raf || !motion) {
                if (!motion && dirty) render(performance.now());
                return;
            }
            raf = requestAnimationFrame(time => {
                raf = 0;
                watchFrameTime(time);
                render(time);
                if (motion) requestFrame();
            });
        }

        // Pointer input is motion-only, passive and never consumed: the desktop below keeps every event.
        function onPointerMove(event) {
            if (paused || stopped) return;
            sim.pointerMove(event.clientX, event.clientY, performance.now() / 1000, event.buttons);
        }

        function onPointerDown(event) {
            if (paused || stopped || event.button !== 0) return;
            const target = event.target;
            if (target && typeof target.closest === 'function' && target.closest(POKE_BLOCKERS)) return;
            sim.pointerDown(event.clientX, event.clientY, performance.now() / 1000);
        }

        function onPointerEnd() {
            sim.pointerUp();
        }

        function pause() {
            paused = true;
            if (raf) cancelAnimationFrame(raf);
            raf = 0;
            lastTime = 0;
            lastRafTime = 0;
            qualitySum = 0; qualityCount = 0; qualityStart = 0; slowWindows = 0;
            sim.pointerUp();
            for (const animation of springs) {
                animation.onfinish = animation.oncancel = null;
                animation.cancel();
            }
            springs.clear();
        }

        function resume() {
            if (stopped) return;
            paused = false;
            snapshot();
            requestFrame();
        }

        function stop() {
            if (stopped) return;
            stopped = true;
            if (raf) cancelAnimationFrame(raf);
            raf = 0;
            if (resizeObserver) resizeObserver.disconnect();
            if (mutationObserver) mutationObserver.disconnect();
            for (const animation of springs) {
                animation.onfinish = animation.oncancel = null;
                animation.cancel();
            }
            springs.clear();
            springAt.clear();
            records.clear();
            sim.reset();
            backdropToken++;
            if (backdropTimer) window.clearTimeout(backdropTimer);
            backdropTimer = 0;
            for (const timer of settleTimers) window.clearTimeout(timer);
            settleTimers.clear();
            renderer.dispose();
            if (backdrop.image && backdrop.image.close) backdrop.image.close();
            backdrop = { state: 'translucent', reason: 'stopped', image: null, naturalWidth: 0, naturalHeight: 0 };
        }

        // Test/debug hook: force a bubble, a poke or a drop (motion only), or pin the render scale.
        function command(name, payload) {
            if (stopped) return null;
            const data = payload || {};
            if (name === 'quality') {
                // Pins the render scale and ends the adaptive watch (close-up screenshots need full resolution).
                const scale = clamp(Number(data.scale) || 1, .25, 1);
                qualityLocked = true;
                if (scale !== qualityScale) setQuality(scale);
                requestFrame();
                return qualityScale;
            }
            if (!motion || paused) return null;
            if (name === 'bubble') return sim.spawnBubble({ windowId: data.windowId, at: data.at, base: data.base, fast: data.fast });
            if (name === 'poke') {
                const hit = sim.poke(Number(data.x), Number(data.y));
                return hit ? hit.id : null;
            }
            if (name === 'drop') {
                const x = Number(data.x), y = Number(data.y);
                if (!Number.isFinite(x) || !Number.isFinite(y)) return null;
                sim.addDrop(x, y, Number(data.r) || 5, Number(data.vx) || 0, Number(data.vy) || 0, 4, { wobble: .4 });
                return true;
            }
            return null;
        }

        function inspect() {
            const gl = renderer.kind === 'webgl2' ? renderer.inspect() : null;
            return {
                ready: !!renderer && renderer.ready !== false,
                renderer: renderer.kind,
                backdrop: backdrop.state,
                backdropReason: backdrop.reason,
                qualityScale,
                contextLost,
                error: rendererError,
                gpu: gl ? gl.gpu : '',
                fade: gl ? gl.fade : 1,
                compileMs: gl ? gl.compileMs : 0,
                blockedMs: gl ? gl.blockedMs : 0,
                parallelCompile: gl ? gl.parallelCompile : false,
                lastRenderMs: gl ? gl.lastRenderMs : 0,
                averageRenderMs: gl ? gl.averageRenderMs : 0,
                firstRenderMs: gl ? gl.firstRenderMs : 0,
                drawCalls: gl ? gl.drawCalls : 0,
                quadCount: gl ? gl.quadCount : 0,
                motion,
                paused,
                rafPending: !!raf,
                windowCount: records.size,
                frameCount,
                maskCount: cachedOverlayMasks.length,
                maskRefreshCount,
                ...sim.stats()
            };
        }

        chooseRenderer();
        resizeObserver = typeof ResizeObserver === 'function' ? new ResizeObserver(resize) : null;
        if (resizeObserver) resizeObserver.observe(layer);
        observeMaskChanges();
        window.addEventListener('resize', resize, { passive: true, signal });
        window.addEventListener('scroll', invalidateMasks, { passive: true, capture: true, signal });
        window.addEventListener('transitionend', invalidateMasks, { passive: true, capture: true, signal });
        window.addEventListener('animationend', invalidateMasks, { passive: true, capture: true, signal });
        window.addEventListener('aurago:desktop-window', onWindowEvent, { signal });
        if (motion) {
            const pointerOptions = { passive: true, capture: true, signal };
            window.addEventListener('pointermove', onPointerMove, pointerOptions);
            window.addEventListener('pointerdown', onPointerDown, pointerOptions);
            window.addEventListener('pointerup', onPointerEnd, pointerOptions);
            window.addEventListener('pointercancel', onPointerEnd, pointerOptions);
        }
        window.addEventListener('aurago:desktop-windows-refresh', () => { snapshot(); requestFrame(); }, { signal });
        if (signal && typeof signal.addEventListener === 'function') signal.addEventListener('abort', stop, { once: true });
        resize();
        refreshBackdrop();
        verifyBackdropLater();
        snapshot();
        requestFrame();
        return { pause, resume, stop, inspect, command };
    }

    window.AuraDesktopThemeFx.register('green_slime', { start });
})();
