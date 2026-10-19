(function () {
    'use strict';

    const MAX_PULSES = 20;
    const MAX_PACKETS = 42;
    const MASK_SELECTORS = '.vd-icon, .vd-widget, .vd-taskbar, .vd-dock, .vd-start-menu, .vd-context-menu, .vd-modal-backdrop, .vd-toast-container, .vd-global-menu, .vd-radial-menu';

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

    function visible(record) { return !!record && record.visible !== false && !record.minimized; }

    function start({ layer, signal, options }) {
        const motion = !!(options && options.motion);
        const canvas = document.createElement('canvas');
        canvas.className = 'vd-fx-dreams-canvas';
        canvas.setAttribute('aria-hidden', 'true');
        canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;display:block;pointer-events:none;';
        layer.appendChild(canvas);
        const ctx = canvas.getContext('2d', { alpha: true, desynchronized: true });
        if (!ctx) return { stop() { canvas.remove(); } };

        let width = 1;
        let height = 1;
        let dpr = 1;
        let raf = 0;
        let paused = false;
        let stopped = false;
        let lastTime = 0;
        let elapsed = 0;
        let frameCount = 0;
        let dirty = true;
        let resizeObserver = null;
        let maskObserver = null;
        const windows = new Map();
        let overlayMasks = [];
        const pulses = [];
        const packets = [];
        const sparkles = [];

        function readSnapshot() {
            const api = window.AuraDesktopWindowEffects;
            if (!api || typeof api.snapshot !== 'function') return;
            try {
                const snapshot = api.snapshot();
                if (!Array.isArray(snapshot)) return;
                const next = new Map();
                for (const record of snapshot) {
                    if (!visible(record) || record.id == null) continue;
                    const rect = rectFor(record);
                    if (rect) next.set(String(record.id), { ...record, rect });
                }
                windows.clear();
                for (const [id, record] of next) windows.set(id, record);
                refreshOverlayMasks();
                dirty = true;
            } catch (_) { /* the desktop may be tearing down */ }
        }

        function refreshOverlayMasks() {
            const next = [];
            for (const element of document.querySelectorAll(MASK_SELECTORS)) {
                if (!element.isConnected || element.hidden) continue;
                const style = window.getComputedStyle ? window.getComputedStyle(element) : null;
                if (style && (style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0)) continue;
                const rect = element.getBoundingClientRect();
                if (rect.width > 0 && rect.height > 0 && rect.bottom >= 0 && rect.right >= 0 && rect.top <= height && rect.left <= width) {
                    next.push({ x: rect.x, y: rect.y, width: rect.width, height: rect.height });
                }
            }
            overlayMasks = next;
        }

        function blockersFor(records) {
            return (records || Array.from(windows.values())).map(rectFor).filter(Boolean).concat(overlayMasks);
        }

        function addPulse(record, closing) {
            const rect = rectFor(record);
            if (!rect || !motion || record.silent) return;
            if (pulses.length >= MAX_PULSES) pulses.shift();
            const edge = closing ? 'bottom' : 'top';
            pulses.push({ id: String(record.id || ''), rect, age: 0, life: closing ? .58 : .82, edge, phase: Math.random() * Math.PI * 2 });
            const x = edge === 'top' ? rect.x + rect.width * (.25 + Math.random() * .5) : rect.x + rect.width * (.12 + Math.random() * .76);
            const y = edge === 'top' ? rect.y : rect.y + rect.height;
            if (packets.length >= MAX_PACKETS) packets.shift();
            packets.push({ x, y, age: 0, life: .95, direction: closing ? -1 : 1, size: 2 + Math.random() * 1.5 });
            for (let i = 0; i < 5; i++) {
                if (sparkles.length >= MAX_PULSES * 5) sparkles.shift();
                sparkles.push({ x: x + (Math.random() - .5) * 32, y: y + (Math.random() - .5) * 22, age: 0, life: .42 + Math.random() * .28 });
            }
        }

        function onWindowEvent(event) {
            const record = event && event.detail;
            if (!record || record.id == null) return;
            const id = String(record.id);
            if (record.phase === 'close') {
                if (!record.silent) addPulse(record, true);
                windows.delete(id);
            } else {
                if (visible(record)) {
                    const rect = rectFor(record);
                    if (rect) windows.set(id, { ...record, rect });
                }
                if (record.phase === 'open') addPulse(record, false);
            }
            dirty = true;
            refreshOverlayMasks();
            requestFrame();
        }

        function resize() {
            width = Math.max(1, layer.clientWidth || window.innerWidth || 1);
            height = Math.max(1, layer.clientHeight || window.innerHeight || 1);
            dpr = Math.min(window.devicePixelRatio || 1, motion ? 1.5 : 1.25);
            canvas.width = Math.max(1, Math.round(width * dpr));
            canvas.height = Math.max(1, Math.round(height * dpr));
            ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
            refreshOverlayMasks();
            dirty = true;
            requestFrame();
        }

        function clipOutside(ctx2d, rects) {
            ctx2d.beginPath();
            ctx2d.rect(0, 0, width, height);
            ctx2d.clip();
            for (const rect of rects) {
                ctx2d.beginPath();
                ctx2d.rect(0, 0, width, height);
                ctx2d.rect(rect.x, rect.y, rect.width, rect.height);
                ctx2d.clip('evenodd');
            }
        }

        function routeFor(a, b) {
            const ar = rectFor(a);
            const br = rectFor(b);
            if (!ar || !br) return null;
            const ax = ar.x + ar.width * .5;
            const ay = ar.y + ar.height + 3;
            const bx = br.x + br.width * .5;
            const by = br.y - 3;
            const dx = bx - ax;
            const dy = by - ay;
            const bend = Math.min(150, Math.max(38, Math.abs(dx) * .19 + Math.abs(dy) * .08));
            return { ax, ay, bx, by, c1x: ax + dx * .28, c1y: ay + bend, c2x: bx - dx * .28, c2y: by - bend };
        }

        function traceCable(route, alpha, phase) {
            ctx.save();
            ctx.lineCap = 'round';
            ctx.lineJoin = 'round';
            ctx.strokeStyle = 'rgba(1,19,42,' + (.52 * alpha) + ')';
            ctx.lineWidth = 5;
            ctx.beginPath();
            ctx.moveTo(route.ax, route.ay);
            ctx.bezierCurveTo(route.c1x, route.c1y, route.c2x, route.c2y, route.bx, route.by);
            ctx.stroke();
            const cable = ctx.createLinearGradient(route.ax, route.ay, route.bx, route.by);
            cable.addColorStop(0, 'rgba(40,229,255,' + (.82 * alpha) + ')');
            cable.addColorStop(.52, 'rgba(79,135,255,' + (.72 * alpha) + ')');
            cable.addColorStop(1, 'rgba(243,81,255,' + (.78 * alpha) + ')');
            ctx.strokeStyle = cable;
            ctx.lineWidth = 1.55;
            ctx.shadowColor = 'rgba(77,198,255,.5)';
            ctx.shadowBlur = 8;
            ctx.stroke();
            ctx.shadowBlur = 0;
            ctx.fillStyle = 'rgba(133,238,255,' + alpha + ')';
            for (let i = 0; i < 5; i++) {
                const t = (i / 5 + phase) % 1;
                const mt = 1 - t;
                const x = mt * mt * mt * route.ax + 3 * mt * mt * t * route.c1x + 3 * mt * t * t * route.c2x + t * t * t * route.bx;
                const y = mt * mt * mt * route.ay + 3 * mt * mt * t * route.c1y + 3 * mt * t * t * route.c2y + t * t * t * route.by;
                ctx.beginPath();
                ctx.arc(x, y, 1.15, 0, Math.PI * 2);
                ctx.fill();
            }
            ctx.restore();
        }

        function drawFrame(record, all) {
            const rect = rectFor(record);
            if (!rect) return;
            const z = Number(record.z) || 0;
            const blockers = [rect];
            for (const other of all) {
                if (other === record || (Number(other.z) || 0) <= z) continue;
                const otherRect = rectFor(other);
                if (otherRect) blockers.push(otherRect);
            }
            ctx.save();
            clipOutside(ctx, blockers.concat(overlayMasks));
            const frame = { x: rect.x - 3, y: rect.y - 3, width: rect.width + 6, height: rect.height + 6 };
            const pulse = motion ? .5 + .5 * Math.sin(elapsed * 1.4 + z * .012) : .55;
            const gradient = ctx.createLinearGradient(frame.x, frame.y, frame.x + frame.width, frame.y + frame.height);
            gradient.addColorStop(0, 'rgba(56,243,255,.82)');
            gradient.addColorStop(.5, 'rgba(107,115,255,.76)');
            gradient.addColorStop(1, 'rgba(246,84,245,.84)');
            ctx.lineJoin = 'round';
            ctx.lineWidth = 5;
            ctx.strokeStyle = 'rgba(45,208,255,' + (.18 + pulse * .14) + ')';
            ctx.shadowColor = 'rgba(55,206,255,.72)';
            ctx.shadowBlur = 15 + pulse * 8;
            ctx.beginPath();
            ctx.roundRect(frame.x, frame.y, frame.width, frame.height, 13);
            ctx.stroke();
            ctx.shadowBlur = 0;
            ctx.lineWidth = 1.6;
            ctx.strokeStyle = gradient;
            ctx.beginPath();
            ctx.roundRect(frame.x, frame.y, frame.width, frame.height, 13);
            ctx.stroke();

            const leds = [
                [rect.x + 13, rect.y + rect.height + 3, 'rgba(74,249,255,.96)'],
                [rect.x + rect.width - 13, rect.y + rect.height + 3, 'rgba(252,104,252,.96)'],
                [rect.x + rect.width + 3, rect.y + rect.height * .48, 'rgba(156,131,255,.96)']
            ];
            for (const [x, y, color] of leds) {
                const size = 2.4 + pulse * .8;
                ctx.fillStyle = color;
                ctx.shadowColor = color;
                ctx.shadowBlur = 10 + pulse * 6;
                ctx.beginPath();
                ctx.arc(x, y, size, 0, Math.PI * 2);
                ctx.fill();
                ctx.shadowBlur = 0;
            }
            ctx.restore();
        }

        function drawPulse(pulse) {
            const alpha = clamp(1 - pulse.age / pulse.life, 0, 1);
            const scale = pulse.edge === 'top' ? pulse.age / pulse.life : 1 - pulse.age / pulse.life;
            const rect = pulse.rect;
            ctx.save();
            clipOutside(ctx, blockersFor());
            ctx.strokeStyle = 'rgba(106,235,255,' + (alpha * .76) + ')';
            ctx.lineWidth = 1.2;
            ctx.shadowColor = 'rgba(80,198,255,' + (alpha * .8) + ')';
            ctx.shadowBlur = 13;
            const inset = 3 + scale * 28;
            ctx.beginPath();
            ctx.roundRect(rect.x - inset, rect.y - inset, rect.width + inset * 2, rect.height + inset * 2, 15 + inset * .25);
            ctx.stroke();
            ctx.restore();
        }

        function draw(time) {
            if (stopped || paused) return;
            const dt = lastTime ? clamp((time - lastTime) / 1000, 0, .05) : 0;
            lastTime = time;
            if (motion) elapsed += dt;
            if (motion) {
                for (const list of [pulses, packets, sparkles]) {
                    for (let i = list.length - 1; i >= 0; i--) {
                        list[i].age += dt;
                        if (list[i].age >= list[i].life) list.splice(i, 1);
                    }
                }
            }
            if (!dirty && !motion) return;
            dirty = false;
            ctx.clearRect(0, 0, width, height);
            frameCount++;
            const all = Array.from(windows.values()).sort((a, b) => (Number(a.z) || 0) - (Number(b.z) || 0));

            if (all.length > 1) {
                ctx.save();
                clipOutside(ctx, blockersFor(all));
                const limit = Math.min(5, all.length - 1);
                for (let i = 0; i < limit; i++) {
                    const route = routeFor(all[i], all[(i + 1) % all.length]);
                    if (route) traceCable(route, .78, motion ? (elapsed * .11 + i * .2) % 1 : .24 + i * .13);
                }
                ctx.restore();
            }
            for (const record of all) drawFrame(record, all);
            if (pulses.length) for (const pulse of pulses) drawPulse(pulse);
            if (packets.length || sparkles.length) {
                ctx.save();
                clipOutside(ctx, blockersFor(all));
                for (const packet of packets) {
                    const alpha = clamp(1 - packet.age / packet.life, 0, 1);
                    ctx.fillStyle = 'rgba(97,228,255,' + alpha + ')';
                    ctx.shadowColor = 'rgba(90,220,255,.86)';
                    ctx.shadowBlur = 12;
                    ctx.beginPath();
                    ctx.arc(packet.x, packet.y + packet.direction * packet.age * 42, packet.size * alpha, 0, Math.PI * 2);
                    ctx.fill();
                }
                ctx.shadowBlur = 0;
                for (const sparkle of sparkles) {
                    const alpha = clamp(1 - sparkle.age / sparkle.life, 0, 1);
                    ctx.strokeStyle = 'rgba(207,251,255,' + alpha + ')';
                    ctx.lineWidth = 1;
                    ctx.beginPath();
                    ctx.moveTo(sparkle.x - 3, sparkle.y);
                    ctx.lineTo(sparkle.x + 3, sparkle.y);
                    ctx.moveTo(sparkle.x, sparkle.y - 3);
                    ctx.lineTo(sparkle.x, sparkle.y + 3);
                    ctx.stroke();
                }
                ctx.restore();
            }
        }

        function requestFrame() {
            if (stopped || paused || raf || !motion) {
                if (!motion && dirty) draw(performance.now());
                return;
            }
            raf = requestAnimationFrame(time => {
                raf = 0;
                draw(time);
                requestFrame();
            });
        }

        function pause() {
            paused = true;
            if (raf) cancelAnimationFrame(raf);
            raf = 0;
            lastTime = 0;
        }

        function resume() {
            if (stopped) return;
            paused = false;
            readSnapshot();
            requestFrame();
        }

        function stop() {
            if (stopped) return;
            stopped = true;
            if (raf) cancelAnimationFrame(raf);
            raf = 0;
            if (resizeObserver) resizeObserver.disconnect();
            if (maskObserver) maskObserver.disconnect();
            windows.clear();
            overlayMasks = [];
            pulses.length = packets.length = sparkles.length = 0;
            canvas.remove();
        }

        function inspect() {
            return {
                activeId: 'electronic_dreams',
                ready: !!ctx,
                motion,
                paused,
                frameCount,
                rafPending: !!raf,
                windowCount: windows.size,
                maskCount: overlayMasks.length,
                pulseCount: pulses.length,
                packetCount: packets.length
            };
        }

        resizeObserver = typeof ResizeObserver === 'function' ? new ResizeObserver(resize) : null;
        if (resizeObserver) resizeObserver.observe(layer);
        maskObserver = typeof MutationObserver === 'function' ? new MutationObserver(() => {
            refreshOverlayMasks();
            dirty = true;
            requestFrame();
        }) : null;
        if (maskObserver && document.body) maskObserver.observe(document.body, {
            childList: true, subtree: true, attributes: true, attributeFilter: ['style', 'class', 'hidden']
        });
        window.addEventListener('resize', resize, { passive: true, signal });
        window.addEventListener('aurago:desktop-window', onWindowEvent, { signal });
        window.addEventListener('aurago:desktop-windows-refresh', () => { readSnapshot(); requestFrame(); }, { signal });
        if (signal && typeof signal.addEventListener === 'function') signal.addEventListener('abort', stop, { once: true });
        resize();
        readSnapshot();
        requestFrame();
        return { pause, resume, stop, inspect };
    }

    window.AuraDesktopThemeFx.register('electronic_dreams', { start });
})();
