(function () {
    'use strict';

    // Orbit window bridge (global AuraOrbitWindows). Listens to the aurago:desktop-window events (the orchestrator forwards
    // them to handle()) and turns them into CSS variables and particles: drag velocity drives three springs per window
    // (--vd-orbit-tilt-x, --vd-orbit-tilt-y in deg, --vd-orbit-lift in px) behind the attribute data-orbit-tilt, which the
    // skin turns into a perspective tilt; open, close and minimize emit particles, and a thruster glows at the trailing edge
    // of a dragged window. Windows are flat at rest: once the springs have settled the attribute and the variables are
    // removed again. The bridge never changes application state and never intercepts input.
    // window.AuraOrbitWindows = { create }; create({ particles, choreo, onSound }) -> { handle, update, dispose, inspect }.

    const SETTLE = 0.05;
    const THRUSTER_SPEED = 1800;
    const PROPERTIES = ['--vd-orbit-tilt-x', '--vd-orbit-tilt-y', '--vd-orbit-lift'];
    const UNITS = ['deg', 'deg', 'px'];

    function finite(value, fallback) { const n = Number(value); return Number.isFinite(n) ? n : fallback; }

    function rectOf(detail) {
        const r = detail && detail.rect;
        if (!r) return null;
        const rect = { x: finite(r.x, NaN), y: finite(r.y, NaN), width: finite(r.width, NaN), height: finite(r.height, NaN) };
        return Number.isFinite(rect.x + rect.y + rect.width + rect.height) && rect.width > 0 && rect.height > 0 ? rect : null;
    }

    function create(options) {
        const particles = options.particles;
        const choreo = options.choreo;
        const tracked = new Map();            // element -> { element, springs: [x, y, lift], targets: [x, y, lift], written: [..], releasing }

        function entryFor(element) {
            let entry = tracked.get(element);
            if (!entry) {
                entry = { element, springs: [{ value: 0, velocity: 0 }, { value: 0, velocity: 0 }, { value: 0, velocity: 0 }], targets: [0, 0, 0], written: [NaN, NaN, NaN], releasing: false };
                tracked.set(element, entry);
                element.setAttribute('data-orbit-tilt', '');
            }
            return entry;
        }

        function release(entry) {
            tracked.delete(entry.element);
            entry.element.removeAttribute('data-orbit-tilt');
            for (const name of PROPERTIES) entry.element.style.removeProperty(name);
        }

        function minimizeTarget(detail, rect) {
            const style = detail.element && detail.element.style;
            const dx = style ? parseFloat(style.getPropertyValue('--vd-minimize-dx')) : NaN;
            const dy = style ? parseFloat(style.getPropertyValue('--vd-minimize-dy')) : NaN;
            const cx = rect.x + rect.width / 2, cy = rect.y + rect.height / 2;
            if (Number.isFinite(dx) && Number.isFinite(dy)) return { x: cx + dx, y: cy + dy };
            return { x: cx, y: Math.max(cy, (window.innerHeight || 0) - 24) };
        }

        function thruster(detail, rect) {
            const vx = finite(detail.velocity && detail.velocity.x, 0), vy = finite(detail.velocity && detail.velocity.y, 0);
            const speed = Math.hypot(vx, vy);
            if (speed < 40) return;
            let point;
            if (Math.abs(vx) >= Math.abs(vy)) point = { x: vx > 0 ? rect.x : rect.x + rect.width, y: rect.y + rect.height / 2 };
            else point = { x: rect.x + rect.width / 2, y: vy > 0 ? rect.y : rect.y + rect.height };
            particles.emitThruster(point, { x: -vx / speed, y: -vy / speed }, Math.min(1, speed / THRUSTER_SPEED));
        }

        function handle(detail) {
            if (!detail || !particles) return;
            const rect = rectOf(detail);
            switch (detail.phase) {
                case 'open':
                    if (!detail.silent && detail.visible !== false && rect) particles.emitMaterialize(rect);
                    break;
                case 'close':
                    if (!detail.silent && rect) particles.emitDissolve(rect);
                    if (detail.element && tracked.has(detail.element)) release(tracked.get(detail.element));
                    break;
                case 'minimize':
                    if (rect) particles.emitStream(rect, minimizeTarget(detail, rect));
                    break;
                case 'drag-start':
                    if (detail.element && !detail.silent) { const entry = entryFor(detail.element); entry.releasing = false; }
                    break;
                case 'drag': {
                    if (!detail.element || detail.silent) break;
                    const entry = entryFor(detail.element);
                    entry.releasing = false;
                    const tilt = choreo.tiltFromVelocity(finite(detail.velocity && detail.velocity.x, 0), finite(detail.velocity && detail.velocity.y, 0));
                    entry.targets[0] = tilt.x; entry.targets[1] = tilt.y; entry.targets[2] = tilt.lift;
                    if (rect) thruster(detail, rect);
                    break;
                }
                case 'drag-end': {
                    const entry = detail.element && tracked.get(detail.element);
                    if (entry) { entry.targets[0] = entry.targets[1] = entry.targets[2] = 0; entry.releasing = true; }
                    break;
                }
                default:
            }
        }

        // Advances the springs; returns true while any window is still tracked (the orchestrator keeps its loop alive).
        function update(dt) {
            for (const entry of Array.from(tracked.values())) {
                if (!entry.element.isConnected) { tracked.delete(entry.element); continue; }
                let settled = true;
                for (let i = 0; i < 3; i++) {
                    entry.springs[i] = choreo.spring(entry.springs[i], entry.targets[i], dt);
                    if (Math.abs(entry.springs[i].value) >= SETTLE || Math.abs(entry.springs[i].velocity) >= SETTLE) settled = false;
                    const rounded = Math.round(entry.springs[i].value * 1000) / 1000;
                    if (rounded !== entry.written[i]) {
                        entry.written[i] = rounded;
                        entry.element.style.setProperty(PROPERTIES[i], rounded + UNITS[i]);
                    }
                }
                if (entry.releasing && settled) release(entry);
            }
            return tracked.size > 0;
        }

        function dispose() {
            for (const entry of Array.from(tracked.values())) release(entry);
            tracked.clear();
        }

        function inspect() {
            let tilting = 0;
            for (const entry of tracked.values()) if (entry.springs.some(s => Math.abs(s.value) >= SETTLE)) tilting++;
            return { tracked: tracked.size, tilting };
        }

        return { handle, update, dispose, inspect };
    }

    window.AuraOrbitWindows = Object.freeze({ create });
}());
