(function () {
    'use strict';

    // DOM bridge of the Green Slime effect. The green_slime_sim.js family owns all state and physics;
    // this file reads window geometry, masks and pointer input, hands them to the simulation, completes
    // the scene (occluders, backdrop) and gives it to the active renderer. A renderer is { kind,
    // render(scene), resize(cssWidth, cssHeight, dpr), dispose() }; green_slime_canvas2d.js provides the
    // Canvas 2D one, which is the only renderer for now.

    const MASK_SELECTORS = '.vd-icon, .vd-widget, .vd-taskbar, .vd-dock, .vd-start-menu, .vd-window-menu-popover, .vd-context-menu, .vd-context-submenu-popover, .vd-modal-backdrop, .vd-modal, .vd-dialog, [role="dialog"], dialog[open], .vd-toast-container, .vd-global-menu, .vd-radial-menu';
    // A press that lands in a window, icon, widget, taskbar, menu or dialog belongs to the user, not to the gel.
    const POKE_BLOCKERS = MASK_SELECTORS + ', .vd-window';
    const OCCLUDER_LIMIT = 24;
    const UNDER_LIMIT = 16;

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
        const canvas = document.createElement('canvas');
        canvas.className = 'vd-fx-slime-canvas';
        canvas.setAttribute('aria-hidden', 'true');
        canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;display:block;pointer-events:none;';
        layer.appendChild(canvas);
        const renderer = slime ? window.AuraGreenSlimeCanvas2D.create(canvas) : null;
        if (!renderer) return { stop() { canvas.remove(); } };
        const sim = slime.createSim({ motion, seed: Date.now() >>> 0 });

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

        function invalidateMasks() {
            maskRectsDirty = true;
            dirty = true;
            requestFrame();
        }

        function resize() {
            width = Math.max(1, layer.clientWidth || window.innerWidth || 1);
            height = Math.max(1, layer.clientHeight || window.innerHeight || 1);
            dpr = Math.min(window.devicePixelRatio || 1, motion ? 1.5 : 1.25);
            renderer.resize(width, height, dpr);
            maskRectsDirty = true;
            dirty = true;
            requestFrame();
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
                attributeFilter: ['class', 'style', 'hidden', 'open', 'aria-hidden']
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
            scene.backdrop = { mode: 'translucent', image: null, rect: { x: 0, y: 0, w: env.width, h: env.height } };
            return scene;
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
            renderer.dispose();
        }

        // Test/debug hook: force a bubble, a poke or a drop. Motion-only.
        function command(name, payload) {
            if (!motion || stopped || paused) return null;
            const data = payload || {};
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
            return {
                ready: !!renderer,
                renderer: renderer.kind,
                backdrop: 'translucent',
                qualityScale: 1,
                contextLost: false,
                error: '',
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
        snapshot();
        requestFrame();
        return { pause, resume, stop, inspect, command };
    }

    window.AuraDesktopThemeFx.register('green_slime', { start });
})();
