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
                    next.push({ x: rect.x, y: rect.y, width: rect.width, height: rect.height,
                        desktop: element.matches('.vd-icon, .vd-widget') && !element.classList.contains('vd-dragging') });
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
            ctx.strokeStyle = 'rgba(2,12,8,' + (.82 * alpha) + ')';
            ctx.lineWidth = 8;
            ctx.beginPath();
            ctx.moveTo(route.ax, route.ay);
            ctx.bezierCurveTo(route.c1x, route.c1y, route.c2x, route.c2y, route.bx, route.by);
            ctx.stroke();
            const cable = ctx.createLinearGradient(route.ax, route.ay, route.bx, route.by);
            cable.addColorStop(0, 'rgba(184,99,42,' + alpha + ')');
            cable.addColorStop(.52, 'rgba(249,190,99,' + alpha + ')');
            cable.addColorStop(1, 'rgba(119,65,30,' + alpha + ')');
            ctx.strokeStyle = cable;
            ctx.lineWidth = 3.5;
            ctx.shadowColor = 'rgba(219,151,70,.3)';
            ctx.shadowBlur = 3;
            ctx.stroke();
            ctx.shadowBlur = 0;
            ctx.fillStyle = 'rgba(157,255,227,' + alpha + ')';
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
            clipOutside(ctx, blockers.concat(overlayMasks.filter(mask => !mask.desktop)));
            const frame = { x: rect.x - 9, y: rect.y - 9, width: rect.width + 18, height: rect.height + 18 };
            const pulse = motion ? .5 + .5 * Math.sin(elapsed * 2.2 + z * .012) : .55;
            const gradient = ctx.createLinearGradient(frame.x, frame.y, frame.x + frame.width, frame.y + frame.height);
            gradient.addColorStop(0, '#2d5a3d');
            gradient.addColorStop(.5, '#0e2b20');
            gradient.addColorStop(1, '#284b31');
            ctx.lineJoin = 'round';
            ctx.lineWidth = 23;
            ctx.strokeStyle = '#061b12';
            ctx.shadowColor = '#0008';
            ctx.shadowBlur = 8;
            ctx.shadowOffsetY = 4;
            ctx.beginPath();
            ctx.roundRect(frame.x, frame.y, frame.width, frame.height, 13);
            ctx.stroke();
            ctx.shadowBlur = 0;
            ctx.shadowOffsetY = 0;
            ctx.lineWidth = 19;
            ctx.strokeStyle = gradient;
            ctx.beginPath();
            ctx.roundRect(frame.x, frame.y, frame.width, frame.height, 13);
            ctx.stroke();

            for (const offset of [-5, 4]) {
                ctx.lineWidth = 1;
                ctx.strokeStyle = offset < 0 ? '#c48c4c' : '#7f5431';
                ctx.beginPath();
                ctx.roundRect(frame.x + offset, frame.y + offset, frame.width - offset * 2, frame.height - offset * 2, 8);
                ctx.stroke();
            }
            const perimeter = 2 * (frame.width + frame.height);
            ctx.strokeStyle = '#ffe29b';
            ctx.shadowColor = '#ffc55f';
            ctx.shadowBlur = 7;
            ctx.lineWidth = 2;
            ctx.setLineDash([16, Math.max(20, perimeter / 4 - 16)]);
            ctx.lineDashOffset = motion ? -elapsed * 48 : -17;
            ctx.beginPath(); ctx.roundRect(frame.x - 5, frame.y - 5, frame.width + 10, frame.height + 10, 8); ctx.stroke();
            ctx.setLineDash([]); ctx.shadowBlur = 0;
            for (let i = 0; i < Math.min(8, Math.floor(rect.height / 82)); i++) {
                component(rect.x - 9, rect.y + 46 + i * 82, i % 3, Math.PI / 2, pulse);
                component(rect.x + rect.width + 9, rect.y + 72 + i * 82, (i + 1) % 3, Math.PI / 2, 1 - pulse);
            }
            for (let i = 0; i < Math.min(10, Math.floor(rect.width / 90)); i++) {
                component(rect.x + 44 + i * 90, rect.y + rect.height + 9, i % 3, 0, pulse);
            }
            for (const [x, y] of [[frame.x, frame.y], [frame.x + frame.width, frame.y], [frame.x, frame.y + frame.height], [frame.x + frame.width, frame.y + frame.height]]) {
                ctx.fillStyle = '#16231b';
                ctx.beginPath();
                ctx.arc(x, y, 5.5, 0, Math.PI * 2);
                ctx.fill();
                ctx.fillStyle = '#b5c1a6'; ctx.beginPath(); ctx.arc(x - .5, y - .7, 3.8, 0, Math.PI * 2); ctx.fill();
                ctx.strokeStyle = '#40503c'; ctx.lineWidth = 1.4;
                ctx.beginPath(); ctx.moveTo(x - 2, y); ctx.lineTo(x + 2, y); ctx.moveTo(x, y - 2); ctx.lineTo(x, y + 2); ctx.stroke();
            }
            ctx.restore();
            drawHeader(record, all, pulse);
        }

        function component(x, y, kind, angle, pulse) {
            ctx.save(); ctx.translate(x, y); ctx.rotate(angle);
            ctx.strokeStyle = '#bdc7b2'; ctx.lineWidth = 2;
            if (kind === 0) {
                for (let pin = -9; pin <= 9; pin += 6) {
                    ctx.beginPath(); ctx.moveTo(pin, -7); ctx.lineTo(pin, 7); ctx.stroke();
                }
                const chip = ctx.createLinearGradient(0, -5, 0, 5); chip.addColorStop(0, '#586259'); chip.addColorStop(.25, '#292d29'); chip.addColorStop(1, '#101811');
                ctx.fillStyle = chip; ctx.fillRect(-14, -5, 28, 10);
                ctx.fillStyle = '#82967b'; ctx.font = '5px monospace'; ctx.textAlign = 'center'; ctx.fillText('A-08', 0, 2);
                ctx.fillStyle = '#c9d3c0'; ctx.beginPath(); ctx.arc(-10, -2, 1, 0, Math.PI * 2); ctx.fill();
            } else if (kind === 1) {
                ctx.beginPath(); ctx.moveTo(-15, 0); ctx.lineTo(15, 0); ctx.stroke();
                ctx.fillStyle = '#d5ba83'; ctx.beginPath(); ctx.roundRect(-8, -3.5, 16, 7, 2); ctx.fill();
                for (const [at, color] of [[-5, '#a74725'], [-1, '#3c2621'], [3, '#d1a037']]) { ctx.fillStyle = color; ctx.fillRect(at, -3.5, 2, 7); }
                ctx.fillStyle = '#ffedbd88'; ctx.fillRect(-7, -3, 14, 1);
            } else {
                ctx.fillStyle = '#172a22'; ctx.fillRect(-7, -5, 14, 10);
                const cap = ctx.createLinearGradient(-5, 0, 5, 0); cap.addColorStop(0, '#18392e'); cap.addColorStop(.4, '#75ac88'); cap.addColorStop(.7, '#346b50'); cap.addColorStop(1, '#0d251c');
                ctx.fillStyle = cap; ctx.fillRect(-5, -5, 10, 10);
                ctx.fillStyle = '#c5d4bf'; ctx.beginPath(); ctx.ellipse(0, -4, 5, 2.6, 0, 0, Math.PI * 2); ctx.fill();
                ctx.strokeStyle = '#60715a'; ctx.lineWidth = .6; ctx.beginPath(); ctx.moveTo(-3, -4); ctx.lineTo(3, -4); ctx.stroke();
            }
            ctx.fillStyle = '#10271b'; ctx.fillRect(20, -3, 7, 6);
            ctx.shadowColor = kind % 2 ? '#7cf7ce' : '#ffd16c'; ctx.shadowBlur = 4 + pulse * 5;
            ctx.fillStyle = kind % 2 ? `rgba(122,247,206,${.45 + pulse * .55})` : `rgba(255,201,95,${.45 + pulse * .55})`;
            ctx.fillRect(21, -2, 5, 4); ctx.restore();
        }

        function drawHeader(record, all, pulse) {
            const bar = record.element?.querySelector('.vd-window-titlebar');
            if (!bar) return;
            const rect = bar.getBoundingClientRect();
            const y = rect.bottom - 12;
            ctx.save();
            clipOutside(ctx, all.filter(other => Number(other.z) > Number(record.z)).map(rectFor).filter(Boolean).concat(overlayMasks.filter(mask => !mask.desktop)));
            ctx.beginPath(); ctx.rect(rect.x + 6, rect.bottom - 23, rect.width - 12, 22); ctx.clip();
            ctx.strokeStyle = '#9a6838'; ctx.lineWidth = .8;
            for (let lane = 0; lane < 3; lane++) {
                ctx.beginPath(); ctx.moveTo(rect.x + 10, y - 5 + lane * 5);
                for (let x = rect.x + 20; x < rect.right; x += 42) { ctx.lineTo(x, y - 5 + lane * 5); ctx.lineTo(x + 6, y - 8 + lane * 5); ctx.lineTo(x + 24, y - 8 + lane * 5); }
                ctx.stroke();
            }
            for (let i = 0; i < Math.min(12, Math.floor(rect.width / 86)); i++) {
                component(rect.x + 34 + i * 86, y, i % 3, 0, motion ? .5 + .5 * Math.sin(elapsed * 2.6 - i * .9) : pulse);
            }
            ctx.strokeStyle = '#baf3da'; ctx.shadowColor = '#7fffd4'; ctx.shadowBlur = 5;
            ctx.setLineDash([8, 93]); ctx.lineDashOffset = motion ? -elapsed * 42 : 0;
            ctx.beginPath(); ctx.moveTo(rect.x, y + 8); ctx.lineTo(rect.right, y + 8); ctx.stroke();
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
