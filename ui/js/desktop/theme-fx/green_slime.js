(function () {
    'use strict';

    const MAX_DROPS = 72;
    const MAX_DEFORMATION = 18;
    const SURFACE_SAMPLES = 32;
    const SURFACE_NORMALS = Object.freeze([{ x: 0, y: -1 }, { x: 1, y: 0 }, { x: 0, y: 1 }, { x: -1, y: 0 }]);
    const MASK_SELECTORS = '.vd-icon, .vd-widget, .vd-taskbar, .vd-dock, .vd-start-menu, .vd-context-menu, .vd-modal-backdrop, .vd-modal, .vd-dialog, [role="dialog"], dialog[open], .vd-toast-container, .vd-global-menu, .vd-radial-menu';

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

    function start({ layer, signal, options }) {
        const motion = !!(options && options.motion);
        const canvas = document.createElement('canvas');
        canvas.className = 'vd-fx-slime-canvas';
        canvas.setAttribute('aria-hidden', 'true');
        canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;display:block;pointer-events:none;';
        layer.appendChild(canvas);
        const ctx = canvas.getContext('2d', { alpha: true, desynchronized: true });
        if (!ctx) return { stop() { canvas.remove(); } };

        let width = 0;
        let height = 0;
        let dpr = 1;
        let raf = 0;
        let paused = false;
        let stopped = false;
        let lastTime = 0;
        let dirty = true;
        const records = new Map();
        const freeDrops = [];
        const springs = new Set();
        const springAt = new Map();
        const surfaces = new Map();
        const motionDrives = new Map();
        let clock = 0;
        let frameCount = 0;
        let dropCount = 0;
        let maxDisplacement = 0;
        let currentMaxDisplacement = 0;
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
            canvas.width = Math.max(1, Math.round(width * dpr));
            canvas.height = Math.max(1, Math.round(height * dpr));
            ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
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
                for (const id of surfaces.keys()) {
                    if (!records.has(id)) {
                        surfaces.delete(id);
                        motionDrives.delete(id);
                    }
                }
                motionDrives.clear();
                dirty = true;
            } catch (_) { /* the desktop may be tearing down */ }
        }

        function addDrop(x, y, size, vx, vy, life) {
            if (freeDrops.length >= MAX_DROPS) freeDrops.shift();
            freeDrops.push({ x, y, size, vx, vy, age: 0, life, wobble: Math.random() * Math.PI * 2 });
            dropCount++;
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
            if (record.phase === 'close') {
                if (rect && motion && !record.silent) {
                    for (let i = 0; i < 5; i++) {
                        const corner = i % 2 ? rect.x + rect.width : rect.x;
                        addDrop(corner + (Math.random() - .5) * 16, rect.y + rect.height + 3,
                            5 + Math.random() * 8, (Math.random() - .5) * 46, 30 + Math.random() * 65, 1.2 + Math.random() * .6);
                    }
                }
                records.delete(id);
                surfaces.delete(id);
                motionDrives.delete(id);
                dirty = true;
                requestFrame();
                return;
            }
            if (rect && usable(record)) records.set(id, { ...record, rect });
            else {
                records.delete(id);
                surfaces.delete(id);
                motionDrives.delete(id);
            }
            if (record.phase === 'drag' && record.velocity) {
                const vx = Number(record.velocity.x) || 0;
                const vy = Number(record.velocity.y) || 0;
                const speed = Math.hypot(vx, vy);
                if (speed > 8) motionDrives.set(id, { x: vx / speed, y: vy / speed, strength: Math.min(13, speed * .014) });
            } else if (record.phase !== 'drag-end') motionDrives.delete(id);
            if (!record.silent && (record.phase === 'open' || record.phase === 'focus')) springControls(record);
            dirty = true;
            requestFrame();
        }

        function clipOut(ctx2d, blockers) {
            ctx2d.beginPath();
            ctx2d.rect(0, 0, width, height);
            ctx2d.clip();
            for (const rect of blockers) {
                ctx2d.beginPath();
                ctx2d.rect(0, 0, width, height);
                ctx2d.rect(rect.x, rect.y, rect.width, rect.height);
                ctx2d.clip('evenodd');
            }
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
                        desktop: element.matches('.vd-icon, .vd-widget') && !element.classList.contains('vd-dragging') });
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

        function surfaceTargets(rect) {
            const outline = { x: rect.x - 8, y: rect.y - 8, width: rect.width + 16, height: rect.height + 16 };
            const sideCount = SURFACE_SAMPLES / 4;
            const corner = 14;
            const points = [];
            for (let i = 0; i < sideCount; i++) {
                const t = i / (sideCount - 1);
                points.push({ x: outline.x + corner + (outline.width - corner * 2) * t, y: outline.y });
            }
            for (let i = 0; i < sideCount; i++) {
                const t = i / (sideCount - 1);
                points.push({ x: outline.x + outline.width, y: outline.y + corner + (outline.height - corner * 2) * t });
            }
            for (let i = 0; i < sideCount; i++) {
                const t = i / (sideCount - 1);
                points.push({ x: outline.x + corner + (outline.width - corner * 2) * (1 - t), y: outline.y + outline.height });
            }
            for (let i = 0; i < sideCount; i++) {
                const t = i / (sideCount - 1);
                points.push({ x: outline.x, y: outline.y + corner + (outline.height - corner * 2) * (1 - t) });
            }
            return points;
        }

        function followSurface(id, rect, dt) {
            const baseTargets = surfaceTargets(rect);
            const drive = motionDrives.get(id);
            const targets = baseTargets.map((point, i) => {
                if (!motion) return point;
                const side = Math.floor(i / (SURFACE_SAMPLES / 4));
                const normal = SURFACE_NORMALS[side];
                // Slow, travelling surface tension, independent of dragging. Adjacent
                // points have different phases so the skin flows instead of translating.
                const breathing = Math.sin(clock * 1.35 + i * .71) * 4.5 + Math.sin(clock * .63 - i * 1.11) * 1.8;
                const living = { x: point.x + normal.x * breathing, y: point.y + normal.y * breathing };
                if (!drive || drive.strength < .1) return living;
                const trailing = Math.max(0, -(drive.x * normal.x + drive.y * normal.y));
                const along = (i % (SURFACE_SAMPLES / 4) + .5) / (SURFACE_SAMPLES / 4);
                const bulge = drive.strength * trailing * (.42 + .58 * Math.sin(Math.PI * along));
                return { x: living.x - drive.x * bulge, y: living.y - drive.y * bulge };
            });
            let surface = surfaces.get(id);
            if (!surface) {
                const seed = Array.from(id).reduce((sum, letter) => sum + letter.charCodeAt(0), 0);
                surface = {
                    nodes: baseTargets.map(point => ({ ...point, vx: 0, vy: 0 })),
                    drips: [17, 19, 21, 22, 13, 27].map((node, i) => ({ node,
                        period: 4.6 + i * .61, offset: (seed % 17) * .23 + i * 1.13, cycle: -1 }))
                };
                surfaces.set(id, surface);
            }
            const nodes = surface.nodes;
            if (!motion || dt <= 0) {
                nodes.forEach((node, i) => {
                    node.x = targets[i].x;
                    node.y = targets[i].y;
                    node.vx = 0;
                    node.vy = 0;
                });
                return surface;
            }

            const offsets = nodes.map((node, i) => ({ x: node.x - baseTargets[i].x, y: node.y - baseTargets[i].y }));
            const step = clamp(dt, 0, 1 / 30);
            const spring = 112;
            const tension = 22;
            const damping = Math.exp(-17 * step);
            for (let i = 0; i < nodes.length; i++) {
                const node = nodes[i];
                const target = targets[i];
                const previous = offsets[(i + nodes.length - 1) % nodes.length];
                const next = offsets[(i + 1) % nodes.length];
                const neighborX = (previous.x + next.x) * .5;
                const neighborY = (previous.y + next.y) * .5;
                const ax = (target.x - node.x) * spring + (neighborX - offsets[i].x) * tension;
                const ay = (target.y - node.y) * spring + (neighborY - offsets[i].y) * tension;
                node.vx = clamp((node.vx + ax * step) * damping, -720, 720);
                node.vy = clamp((node.vy + ay * step) * damping, -720, 720);
                node.x += node.vx * step;
                node.y += node.vy * step;
                let dx = node.x - baseTargets[i].x;
                let dy = node.y - baseTargets[i].y;
                const distance = Math.hypot(dx, dy);
                if (distance > MAX_DEFORMATION) {
                    const scale = MAX_DEFORMATION / distance;
                    node.x = baseTargets[i].x + dx * scale;
                    node.y = baseTargets[i].y + dy * scale;
                    const outward = node.vx * dx + node.vy * dy;
                    if (outward > 0) {
                        node.vx -= outward * dx / (distance * distance);
                        node.vy -= outward * dy / (distance * distance);
                    }
                    dx = node.x - baseTargets[i].x;
                    dy = node.y - baseTargets[i].y;
                }
                currentMaxDisplacement = Math.max(currentMaxDisplacement, Math.hypot(dx, dy));
            }
            if (drive) {
                drive.strength *= Math.exp(-4.5 * step);
                if (drive.strength < .1) motionDrives.delete(id);
            }
            maxDisplacement = Math.max(maxDisplacement, currentMaxDisplacement);
            return surface;
        }

        function detachPeriodicDrop(id, surface) {
            if (!motion) return;
            for (const drip of surface.drips) {
                const time = (clock + drip.offset) / drip.period;
                const cycle = Math.floor(time);
                const phase = time - cycle;
                if (phase < .84 || drip.cycle === cycle) continue;
                drip.cycle = cycle;
                const node = surface.nodes[drip.node];
                addDrop(node.x, node.y + 44, 6.5, node.vx * .08, 28, 1.6);
                // Recoil travels back into the membrane after the neck tears.
                node.vy -= 48;
            }
        }

        function contourPath(nodes) {
            ctx.beginPath();
            const first = nodes[0];
            const last = nodes[nodes.length - 1];
            ctx.moveTo((first.x + last.x) * .5, (first.y + last.y) * .5);
            for (let i = 0; i < nodes.length; i++) {
                const current = nodes[i];
                const next = nodes[(i + 1) % nodes.length];
                ctx.quadraticCurveTo(current.x, current.y, (current.x + next.x) * .5, (current.y + next.y) * .5);
            }
            ctx.closePath();
        }

        function drawDrop(drop, alpha) {
            const stretch = 1 + Math.min(1.2, Math.abs(drop.vy) / 220);
            const size = drop.size;
            const gradient = ctx.createLinearGradient(drop.x - size, drop.y - size, drop.x + size, drop.y + size);
            gradient.addColorStop(0, '#f2ffbc');
            gradient.addColorStop(.26, '#c3f34c');
            gradient.addColorStop(.62, '#75ac1e');
            gradient.addColorStop(1, '#354f0c');
            ctx.globalAlpha = alpha;
            ctx.fillStyle = gradient;
            ctx.shadowColor = 'rgba(126,158,40,.48)';
            ctx.shadowBlur = 12;
            ctx.beginPath();
            ctx.moveTo(drop.x, drop.y - size * stretch);
            ctx.bezierCurveTo(drop.x + size * .9, drop.y - size * .3, drop.x + size * .82, drop.y + size * .48 * stretch, drop.x + size * .35, drop.y + size * .76 * stretch);
            ctx.quadraticCurveTo(drop.x, drop.y + size * stretch, drop.x - size * .35, drop.y + size * .76 * stretch);
            ctx.bezierCurveTo(drop.x - size * .82, drop.y + size * .48 * stretch, drop.x - size * .9, drop.y - size * .3, drop.x, drop.y - size * stretch);
            ctx.closePath();
            ctx.fill();
            ctx.shadowBlur = 0;
            ctx.fillStyle = 'rgba(250,255,214,.9)';
            ctx.beginPath();
            ctx.ellipse(drop.x - size * .25, drop.y, size * .14, size * .4, -.25, 0, Math.PI * 2);
            ctx.fill();
            ctx.globalAlpha = 1;
        }

        function drawDrip(point, len, dripWidth, phase) {
            const y = point.y;
            const end = y + len;
            const gradient = ctx.createLinearGradient(point.x - dripWidth, y, point.x + dripWidth, end);
            gradient.addColorStop(0, '#e5ff91');
            gradient.addColorStop(.28, '#b2e936');
            gradient.addColorStop(.66, '#73a51d');
            gradient.addColorStop(1, '#3d6413');
            ctx.fillStyle = gradient;
            ctx.shadowColor = 'rgba(12,28,1,.5)';
            ctx.shadowBlur = 5;
            ctx.shadowOffsetY = 3;
            const bulb = 4 + phase * 4;
            const neck = Math.max(.7, dripWidth * (1 - phase * .9));
            ctx.beginPath();
            ctx.moveTo(point.x - dripWidth, y);
            ctx.bezierCurveTo(point.x - neck, y + len * .3, point.x - neck, end - bulb * 1.8, point.x - bulb, end - bulb);
            ctx.bezierCurveTo(point.x - bulb * 1.35, end + bulb, point.x + bulb * 1.35, end + bulb, point.x + bulb, end - bulb);
            ctx.bezierCurveTo(point.x + neck, end - bulb * 1.8, point.x + neck, y + len * .3, point.x + dripWidth, y);
            ctx.closePath();
            ctx.fill();
            ctx.shadowBlur = 0;
            ctx.shadowOffsetY = 0;
            ctx.fillStyle = 'rgba(249,255,210,.92)';
            ctx.beginPath();
            ctx.ellipse(point.x - bulb * .3, end - bulb * .4, 1.5, bulb * .55, -.3, 0, Math.PI * 2);
            ctx.fill();
        }

        function drawWindow(record, rect, surface, blockers) {
            if (!rect || !surface) return;
            ctx.save();
            clipOut(ctx, blockers);
            const nodes = surface.nodes;
            const first = nodes[0];
            const gradient = ctx.createLinearGradient(first.x, first.y, nodes[16].x, nodes[16].y);
            gradient.addColorStop(0, '#bce65b');
            gradient.addColorStop(.28, '#83b529');
            gradient.addColorStop(.62, '#537f19');
            gradient.addColorStop(1, '#9bcb38');
            ctx.lineJoin = 'round';
            ctx.lineWidth = 28;
            ctx.strokeStyle = '#38550d';
            ctx.shadowColor = 'rgba(8,19,0,.5)';
            ctx.shadowBlur = 9;
            ctx.shadowOffsetY = 5;
            contourPath(nodes);
            ctx.stroke();
            ctx.shadowBlur = 0;
            ctx.shadowOffsetY = 0;
            ctx.lineWidth = 24;
            ctx.strokeStyle = gradient;
            contourPath(nodes);
            ctx.stroke();
            // Broad rounded gel lobes merge into the membrane. Their highlights
            // follow the spring nodes, so the material keeps its volume as it bends.
            for (let i = 2; i < nodes.length; i += 5) {
                const node = nodes[i];
                const radius = 12.5 + (motion ? Math.sin(clock * .72 + i) * 2 : 0);
                const gel = ctx.createRadialGradient(node.x - 4, node.y - 5, 1, node.x, node.y, radius);
                gel.addColorStop(0, '#efffb4');
                gel.addColorStop(.19, '#badd53');
                gel.addColorStop(.58, '#88b92b');
                gel.addColorStop(1, '#547f17');
                ctx.fillStyle = gel;
                ctx.beginPath(); ctx.arc(node.x, node.y, radius, 0, Math.PI * 2); ctx.fill();
            }
            ctx.lineWidth = 2.5;
            ctx.strokeStyle = 'rgba(237,255,183,.87)';
            ctx.save();
            ctx.translate(-5, -6);
            contourPath(nodes);
            ctx.stroke();
            ctx.restore();
            for (let i = 1; i < nodes.length; i += 2) {
                const node = nodes[i];
                const radius = 1.5 + (i % 4) * .7;
                const drift = motion ? Math.sin(clock * .75 + i) * 2 : 0;
                ctx.fillStyle = '#547d16';
                ctx.strokeStyle = '#d2f277';
                ctx.lineWidth = .8;
                ctx.beginPath();
                ctx.ellipse(node.x + drift, node.y, radius, radius * 1.2, .3, 0, Math.PI * 2);
                ctx.fill(); ctx.stroke();
                ctx.fillStyle = '#f4ffc8';
                ctx.beginPath(); ctx.arc(node.x + drift - .6, node.y - radius * .55, .7, 0, Math.PI * 2); ctx.fill();
            }
            for (const drip of surface.drips) {
                const phase = motion ? ((clock + drip.offset) / drip.period) % 1 : .45;
                const growth = phase < .84 ? phase / .84 : Math.max(0, (1 - phase) * 1.7);
                drawDrip(nodes[drip.node], 7 + 37 * growth * growth, 9 - growth * 3, growth);
            }
            ctx.restore();
        }

        function render(time) {
            if (stopped || paused) return;
            frameCount++;
            const dt = lastTime ? clamp((time - lastTime) / 1000, 0, .05) : 0;
            lastTime = time;
            currentMaxDisplacement = 0;
            if (motion) clock += dt;
            if (motion) {
                for (let i = freeDrops.length - 1; i >= 0; i--) {
                    const drop = freeDrops[i];
                    drop.age += dt;
                    drop.vy += 250 * dt;
                    drop.x += drop.vx * dt;
                    drop.y += drop.vy * dt;
                    if (drop.age >= drop.life || drop.y > height + 40) freeDrops.splice(i, 1);
                }
            }

            if (!dirty && !motion) return;
            dirty = false;
            ctx.clearRect(0, 0, width, height);
            const all = Array.from(records.values()).sort((a, b) => (Number(a.z) || 0) - (Number(b.z) || 0));
            const overlays = overlayMasks();
            const blockers = all.map(rectFor).filter(Boolean).concat(overlays);
            for (const record of all) {
                const rect = rectFor(record);
                if (!rect) continue;
                const id = String(record.id);
                const surface = followSurface(id, rect, dt);
                detachPeriodicDrop(id, surface);
                const frameBlockers = all.filter(other => (Number(other.z) || 0) >= (Number(record.z) || 0)).map(rectFor).filter(Boolean).concat(overlays.filter(mask => !mask.desktop));
                drawWindow(record, rect, surface, frameBlockers);
            }
            if (freeDrops.length) {
                ctx.save();
                clipOut(ctx, blockers);
                for (const drop of freeDrops) drawDrop(drop, clamp(1 - drop.age / drop.life, 0, 1));
                ctx.restore();
            }
            if (motion && freeDrops.length) dirty = true;
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

        function pause() {
            paused = true;
            if (raf) cancelAnimationFrame(raf);
            raf = 0;
            lastTime = 0;
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
            freeDrops.length = 0;
            records.clear();
            surfaces.clear();
            motionDrives.clear();
            canvas.remove();
        }

        function inspect() {
            return {
                ready: !!ctx,
                motion,
                paused,
                rafPending: !!raf,
                windowCount: records.size,
                frameCount,
                maxDisplacement: Math.min(MAX_DEFORMATION, maxDisplacement),
                currentMaxDisplacement: Math.min(MAX_DEFORMATION, currentMaxDisplacement),
                dropCount,
                activeDrops: freeDrops.length,
                maskCount: cachedOverlayMasks.length,
                maskRefreshCount,
                surfaceCount: surfaces.size,
                surfaceNodeCount: surfaces.size * SURFACE_SAMPLES
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
        window.addEventListener('aurago:desktop-windows-refresh', () => { snapshot(); requestFrame(); }, { signal });
        if (signal && typeof signal.addEventListener === 'function') signal.addEventListener('abort', stop, { once: true });
        resize();
        snapshot();
        requestFrame();
        return { pause, resume, stop, inspect };
    }

    window.AuraDesktopThemeFx.register('green_slime', { start });
})();
