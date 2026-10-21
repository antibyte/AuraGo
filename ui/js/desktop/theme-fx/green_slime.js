(function () {
    'use strict';

    const MAX_DROPS = 72;
    const MAX_DEFORMATION = 18;
    const SURFACE_SAMPLES = 32;
    const FRAME_DEPTH = 19;
    const SURFACE_NORMALS = Object.freeze([{ x: 0, y: -1 }, { x: 1, y: 0 }, { x: 0, y: 1 }, { x: -1, y: 0 }]);
    const MASK_SELECTORS = '.vd-icon, .vd-widget, .vd-taskbar, .vd-dock, .vd-start-menu, .vd-window-menu-popover, .vd-context-menu, .vd-context-submenu-popover, .vd-modal-backdrop, .vd-modal, .vd-dialog, [role="dialog"], dialog[open], .vd-toast-container, .vd-global-menu, .vd-radial-menu';

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
        // The material and DOM window must be presented in the same compositor
        // frame. A desynchronized canvas can visibly trail a fast pointer drag.
        const ctx = canvas.getContext('2d', { alpha: true });
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
        const pointer = { x: -10000, y: -10000, energy: 0, at: 0 };
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
                    springAt.delete(id);
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
                springAt.delete(id);
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
            // Window-manager motion is already frame-batched. Paint its new bounds
            // now, even when our animation callback ran earlier in this frame.
            if (motion && /^(drag|resize|snap|maximize|restore)/.test(record.phase)) render(performance.now());
            requestFrame();
        }

        function clipOut(ctx2d, blockers) {
            ctx2d.beginPath();
            ctx2d.rect(0, 0, width, height);
            ctx2d.clip();
            for (const rect of blockers) {
                ctx2d.beginPath();
                ctx2d.rect(0, 0, width, height);
                if (rect.radii) ctx2d.roundRect(rect.x, rect.y, rect.width, rect.height, rect.radii);
                else ctx2d.rect(rect.x, rect.y, rect.width, rect.height);
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
            const outline = { x: rect.x - FRAME_DEPTH, y: rect.y - FRAME_DEPTH, width: rect.width + FRAME_DEPTH * 2, height: rect.height + FRAME_DEPTH * 2 };
            const sideCount = SURFACE_SAMPLES / 4;
            const corner = 34;
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
            let surface = surfaces.get(id);
            if (!surface) {
                const seed = Array.from(id).reduce((sum, letter) => sum + letter.charCodeAt(0), 0);
                surface = {
                    // Only local deformation is sprung. Window translation/resize
                    // reprojects every node immediately, without storing world lag.
                    nodes: baseTargets.map(point => ({ ...point, dx: 0, dy: 0, vx: 0, vy: 0 })),
                    seed: seed * .17,
                    drips: [17, 19, 21, 22, 13, 27].map((node, i) => ({ node,
                        period: 4.6 + i * .61, offset: (seed % 17) * .23 + i * 1.13, cycle: -1 }))
                };
                surfaces.set(id, surface);
            }
            const nodes = surface.nodes;
            const offsets = nodes.map(node => ({ x: node.dx, y: node.dy }));
            const step = motion ? clamp(dt, 0, 1 / 30) : 0;
            const spring = 155;
            const tension = 22;
            const damping = Math.exp(-9.5 * step);
            for (let i = 0; i < nodes.length; i++) {
                const node = nodes[i];
                const normal = SURFACE_NORMALS[Math.floor(i / (SURFACE_SAMPLES / 4))];
                const breathing = motion
                    ? Math.sin(clock * 1.65 + i * .71 + surface.seed) * 5.1 + Math.sin(clock * .83 - i * 1.11) * 1.5
                    : Math.sin(i * .71 + surface.seed) * 1.8;
                const along = (i % (SURFACE_SAMPLES / 4) + .5) / (SURFACE_SAMPLES / 4);
                const drag = drive ? -(drive.x * normal.x + drive.y * normal.y) * drive.strength * (.4 + .6 * Math.sin(Math.PI * along)) : 0;
                const distance = Math.hypot(pointer.x - baseTargets[i].x, pointer.y - baseTargets[i].y);
                const touch = motion ? Math.exp(-distance * distance / 1800) * pointer.energy * Math.cos(distance * .09 - clock * 12) : 0;
                const target = clamp(breathing + drag + touch, -6, 16);
                const previous = offsets[(i + nodes.length - 1) % nodes.length];
                const next = offsets[(i + 1) % nodes.length];
                const neighborX = (previous.x + next.x) * .5;
                const neighborY = (previous.y + next.y) * .5;
                if (!motion) {
                    node.dx = normal.x * target;
                    node.dy = normal.y * target;
                    node.vx = node.vy = 0;
                } else if (step > 0) {
                    const ax = (normal.x * target - node.dx) * spring + (neighborX - node.dx) * tension;
                    const ay = (normal.y * target - node.dy) * spring + (neighborY - node.dy) * tension;
                    node.vx = clamp((node.vx + ax * step) * damping, -180, 180);
                    node.vy = clamp((node.vy + ay * step) * damping, -180, 180);
                    node.dx += node.vx * step;
                    node.dy += node.vy * step;
                }
                // Preserve a substantial meniscus around the fixed inner aperture,
                // including when the leading edge compresses during a hard reversal.
                const inward = node.dx * normal.x + node.dy * normal.y;
                if (inward < -6) {
                    node.dx += normal.x * (-6 - inward);
                    node.dy += normal.y * (-6 - inward);
                }
                const deformation = Math.hypot(node.dx, node.dy);
                if (deformation > MAX_DEFORMATION) {
                    node.dx *= MAX_DEFORMATION / deformation;
                    node.dy *= MAX_DEFORMATION / deformation;
                    node.vx *= .5;
                    node.vy *= .5;
                }
                node.x = baseTargets[i].x + node.dx;
                node.y = baseTargets[i].y + node.dy;
                currentMaxDisplacement = Math.max(currentMaxDisplacement, Math.hypot(node.dx, node.dy));
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
                const shape = dripShape(surface, drip, 1);
                addDrop(shape.x + shape.sway, shape.y + shape.length, 6.5, node.vx * .18, 28, 1.6);
                // Recoil travels back into the membrane after the neck tears.
                node.vy -= 48;
            }
        }

        function contourPath(nodes, inset = 0) {
            const points = inset ? nodes.map((node, i) => {
                const normal = SURFACE_NORMALS[Math.floor(i / (SURFACE_SAMPLES / 4))];
                return { x: node.x - normal.x * inset, y: node.y - normal.y * inset };
            }) : nodes;
            ctx.beginPath();
            const first = points[0];
            const last = points[points.length - 1];
            ctx.moveTo((first.x + last.x) * .5, (first.y + last.y) * .5);
            for (let i = 0; i < points.length; i++) {
                const current = points[i];
                const next = points[(i + 1) % points.length];
                ctx.quadraticCurveTo(current.x, current.y, (current.x + next.x) * .5, (current.y + next.y) * .5);
            }
            ctx.closePath();
        }

        function sampleContour(nodes, position) {
            const wrapped = ((position % nodes.length) + nodes.length) % nodes.length;
            const i = Math.floor(wrapped), t = wrapped - i, u = 1 - t;
            const previous = nodes[(i + nodes.length - 1) % nodes.length];
            const node = nodes[i], next = nodes[(i + 1) % nodes.length];
            const a = { x: (previous.x + node.x) / 2, y: (previous.y + node.y) / 2 };
            const b = { x: (node.x + next.x) / 2, y: (node.y + next.y) / 2 };
            const tx = 2 * u * (node.x - a.x) + 2 * t * (b.x - node.x);
            const ty = 2 * u * (node.y - a.y) + 2 * t * (b.y - node.y);
            const length = Math.hypot(tx, ty) || 1;
            return { x: u * u * a.x + 2 * u * t * node.x + t * t * b.x,
                y: u * u * a.y + 2 * u * t * node.y + t * t * b.y,
                nx: ty / length, ny: -tx / length, angle: Math.atan2(ty, tx) };
        }

        function dripShape(surface, drip, growth) {
            const point = sampleContour(surface.nodes, drip.node + .5);
            return { x: point.x - point.nx * 6, y: point.y - 4,
                length: 6 + (32 + (drip.node % 3) * 8) * growth * growth,
                sway: motion ? Math.sin(clock * 1.8 + drip.offset) * 3 * growth + surface.nodes[drip.node].vx * .018 : 0 };
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
            const x = point.x + point.sway;
            const gradient = ctx.createLinearGradient(point.x - dripWidth, y, x + dripWidth, end);
            gradient.addColorStop(0, '#b4d96c');
            gradient.addColorStop(.25, '#a1cc45');
            gradient.addColorStop(.56, '#699722');
            gradient.addColorStop(.86, '#36591a');
            gradient.addColorStop(1, '#9abd47');
            ctx.fillStyle = gradient;
            ctx.shadowColor = 'rgba(12,28,1,.5)';
            ctx.shadowBlur = 5;
            ctx.shadowOffsetY = 3;
            const bulb = 4 + phase * 4;
            const neck = Math.max(.7, dripWidth * (1 - phase * .9));
            ctx.beginPath();
            ctx.moveTo(point.x - dripWidth, y);
            ctx.bezierCurveTo(point.x - neck, y + len * .3, x - neck, end - bulb * 1.8, x - bulb, end - bulb);
            ctx.bezierCurveTo(x - bulb * 1.35, end + bulb, x + bulb * 1.35, end + bulb, x + bulb, end - bulb);
            ctx.bezierCurveTo(x + neck, end - bulb * 1.8, point.x + neck, y + len * .3, point.x + dripWidth, y);
            ctx.closePath();
            ctx.fill();
            ctx.shadowBlur = 0;
            ctx.shadowOffsetY = 0;
            ctx.fillStyle = 'rgba(249,255,210,.92)';
            ctx.beginPath();
            ctx.ellipse(x - bulb * .3, end - bulb * .4, 1.2, bulb * .55, -.3, 0, Math.PI * 2);
            ctx.fill();
            ctx.strokeStyle = 'rgba(241,255,196,.65)';
            ctx.lineWidth = 1;
            ctx.beginPath();
            ctx.moveTo(point.x - dripWidth * .6, y + 2);
            ctx.bezierCurveTo(point.x - neck * .65, y + len * .3, x - neck * .7, end - bulb * 2, x - bulb * .65, end - bulb);
            ctx.stroke();
        }

        function drawWindow(record, rect, surface, blockers) {
            if (!rect || !surface) return;
            ctx.save();
            const paint = { x: rect.x - 48, y: rect.y - 48, width: rect.width + 96, height: rect.height + 144 };
            ctx.beginPath();
            ctx.rect(paint.x, paint.y, paint.width, paint.height);
            ctx.clip();
            blockers = blockers.filter(r => r.x < paint.x + paint.width && r.x + r.width > paint.x && r.y < paint.y + paint.height && r.y + r.height > paint.y);
            clipOut(ctx, blockers);
            const nodes = surface.nodes;
            // Draw attached drops first; the body covers their roots, so a neck
            // grows out of one continuous volume instead of sitting on its edge.
            for (const drip of surface.drips) {
                const phase = motion ? ((clock + drip.offset) / drip.period) % 1 : .45;
                const growth = phase < .84 ? phase / .84 : Math.max(0, (1 - phase) * 1.7);
                const shape = dripShape(surface, drip, growth);
                drawDrip(shape, shape.length, 9 - growth * 3, growth);
            }
            const gradient = ctx.createLinearGradient(rect.x, rect.y - 20, rect.x + rect.width * .45, rect.y + rect.height);
            gradient.addColorStop(0, '#8eaf3f');
            gradient.addColorStop(.22, '#6b8f25');
            gradient.addColorStop(.58, '#42631b');
            gradient.addColorStop(1, '#85a834');
            ctx.lineJoin = 'round';
            ctx.fillStyle = gradient;
            // The DOM already supplies the window's soft cast shadow. Blurring a
            // second window-sized silhouette every frame is costly in software.
            contourPath(nodes);
            ctx.fill();
            // A filled membrane, bounded by the live outer curve and the anchored
            // round window aperture. Unlike a displaced stroke, it cannot detach.
            contourPath(nodes);
            ctx.clip();
            ctx.lineWidth = 4;
            ctx.strokeStyle = 'rgba(29,49,9,.65)';
            ctx.stroke();
            const volume = ctx.createLinearGradient(rect.x - 20, rect.y - 20, rect.x + rect.width, rect.y + rect.height);
            volume.addColorStop(0, '#cae884');
            volume.addColorStop(.3, '#96bc49');
            volume.addColorStop(.65, '#6d9026');
            volume.addColorStop(1, '#b5d75b');
            ctx.lineWidth = 11;
            ctx.strokeStyle = volume;
            contourPath(nodes, 7);
            ctx.stroke();
            // Long, soft reflections flow over the curved material, with thin
            // Fresnel glints at grazing angles and darker absorbed light inside.
            for (let i = 0; i < 8; i++) {
                const p = sampleContour(nodes, i * 4.13 + .8 + surface.seed + clock * .12);
                ctx.save();
                ctx.translate(p.x - p.nx * 7, p.y - p.ny * 7);
                ctx.rotate(p.angle);
                ctx.scale(7, 1);
                const sheen = ctx.createRadialGradient(0, 0, .3, 0, 0, 6);
                sheen.addColorStop(0, 'rgba(244,255,211,.72)');
                sheen.addColorStop(.3, 'rgba(218,245,150,.3)');
                sheen.addColorStop(1, 'rgba(218,245,150,0)');
                ctx.fillStyle = sheen;
                ctx.fillRect(-6, -6, 12, 12);
                ctx.restore();
            }
            const reflection = ctx.createLinearGradient(rect.x, rect.y, rect.x + rect.width * .6, rect.y + rect.height);
            reflection.addColorStop(0, 'rgba(250,255,224,.95)');
            reflection.addColorStop(.42, 'rgba(222,246,159,.5)');
            reflection.addColorStop(.75, 'rgba(198,232,121,.13)');
            reflection.addColorStop(1, 'rgba(239,255,180,.64)');
            ctx.strokeStyle = reflection;
            ctx.lineWidth = 1.4;
            contourPath(nodes, 3);
            ctx.stroke();
            ctx.beginPath();
            ctx.roundRect(rect.x - 1.5, rect.y - 1.5, rect.width + 3, rect.height + 3, (rect.radii || [22, 22, 22, 22]).map(r => r + 1.5));
            ctx.strokeStyle = 'rgba(20,38,6,.6)';
            ctx.lineWidth = 5;
            ctx.stroke();
            ctx.strokeStyle = 'rgba(194,226,112,.62)';
            ctx.lineWidth = 1;
            ctx.stroke();
            // Air inclusions advect through the membrane; each has a dark lens,
            // a refracted lower rim and an off-centre wet highlight.
            for (let i = 0; i < 22; i++) {
                const p = sampleContour(nodes, i * 1.457 + surface.seed + clock * (.07 + (i % 3) * .025));
                const radius = 1.1 + (i % 4) * .65;
                const x = p.x - p.nx * (7 + (i % 3) * 2), y = p.y - p.ny * (7 + (i % 3) * 2);
                ctx.fillStyle = 'rgba(38,66,12,.45)';
                ctx.beginPath();
                ctx.ellipse(x, y, radius, radius * 1.23, p.angle, 0, Math.PI * 2);
                ctx.fill();
                ctx.strokeStyle = 'rgba(222,247,162,.65)';
                ctx.lineWidth = .7;
                ctx.beginPath(); ctx.arc(x, y, radius, .1, Math.PI * .85); ctx.stroke();
                ctx.fillStyle = 'rgba(251,255,229,.85)';
                ctx.beginPath(); ctx.ellipse(x - radius * .33, y - radius * .5, .65, radius * .43, -.4, 0, Math.PI * 2); ctx.fill();
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
            pointer.energy *= Math.exp(-5 * dt);
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
            const all = Array.from(records.values()).sort((a, b) => (Number(a.z) || 0) - (Number(b.z) || 0)).map(record => {
                const rect = rectFor(record);
                if (rect) {
                    const style = record.element ? getComputedStyle(record.element) : null;
                    rect.radii = style
                        ? [style.borderTopLeftRadius, style.borderTopRightRadius, style.borderBottomRightRadius, style.borderBottomLeftRadius].map(r => parseFloat(r) || 0)
                        : [22, 22, 22, 22];
                }
                return { record, rect };
            }).filter(item => item.rect);
            const overlays = overlayMasks();
            const blockers = all.map(item => item.rect).concat(overlays);
            for (const { record, rect } of all) {
                const id = String(record.id);
                const surface = followSurface(id, rect, dt);
                detachPeriodicDrop(id, surface);
                const frameBlockers = all.filter(other => (Number(other.record.z) || 0) >= (Number(record.z) || 0)).map(item => item.rect).concat(overlays.filter(mask => !mask.desktop));
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
            springAt.clear();
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
        if (motion) window.addEventListener('pointermove', event => {
            if (paused) return;
            const now = performance.now();
            const distance = Math.hypot(event.clientX - pointer.x, event.clientY - pointer.y);
            pointer.energy = clamp(distance / Math.max(8, now - pointer.at) * 2.5, 0, 7);
            pointer.x = event.clientX;
            pointer.y = event.clientY;
            pointer.at = now;
        }, { passive: true, signal });
        window.addEventListener('aurago:desktop-windows-refresh', () => { snapshot(); requestFrame(); }, { signal });
        if (signal && typeof signal.addEventListener === 'function') signal.addEventListener('abort', stop, { once: true });
        resize();
        snapshot();
        requestFrame();
        return { pause, resume, stop, inspect };
    }

    window.AuraDesktopThemeFx.register('green_slime', { start });
})();
