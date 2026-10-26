(function () {
    'use strict';

    const VENDOR_URL = '/js/vendor/desktop-themes/voxel-runtime.js';
    const PIXELS_PER_METER = 100;
    const FIXED_STEP = 1 / 60;
    const MAX_FRAGMENTS = 256;
    const MASK_SELECTORS = '.vd-icon, .vd-widget, .vd-taskbar, .vd-dock, .vd-start-menu, .vd-context-menu, .vd-modal-backdrop, .vd-toast-container, .vd-global-menu, .vd-radial-menu';
    // A click on the bare desktop (.vd-workspace) bursts into earth fragments, unless it lands on any of these.
    const BURST_BLOCKERS = '.vd-window, .vd-icon, .vd-widget, .vd-taskbar, .vd-dock, .vd-start-menu, .vd-context-menu, .vd-modal-backdrop, .vd-toast-container, .vd-global-menu, .vd-radial-menu, .vd-sip-phone-gadget, button, a, input, textarea, select, [contenteditable]';
    const BURST_INTERVAL_MS = 350;
    // A touch bursts only as a tap: it must end within TAP_MAX_MS and move less than TAP_SLOP_PX, else it was the start of a pan.
    const TAP_MAX_MS = 500;
    const TAP_SLOP_PX = 8;
    const ASSEMBLY_MS = 620;
    // A frame hole held open while a window is dragged is repaired this long after the last drag or resize event at the latest.
    const DRAG_HOLD_MAX_MS = 3000;
    const MATERIALS = window.AuraVoxelGraphics.materials;

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
    function randomBetween(min, max) { return min + Math.random() * (max - min); }
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
    function usable(record) { return !!record && record.visible !== false && !record.minimized; }
    function toWorldX(x, width) { return (x - width / 2) / PIXELS_PER_METER; }
    function toWorldY(y, height) { return (height / 2 - y) / PIXELS_PER_METER; }
    function eventIsCurrent(signal, stopped) { return !stopped && !(signal && signal.aborted); }

    function start({ layer, signal, options }) {
        const motion = !!(options && options.motion);
        const fullWallpaper = !!(options && options.wallpaper === 'voxel_world');
        const fallback = document.createElement('canvas');
        fallback.className = 'vd-fx-voxel-fallback';
        fallback.setAttribute('aria-hidden', 'true');
        fallback.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;display:block;pointer-events:none;';
        layer.appendChild(fallback);
        const fallbackContext = fallback.getContext('2d', { alpha: true });

        let width = Math.max(1, layer.clientWidth || window.innerWidth || 1);
        let height = Math.max(1, layer.clientHeight || window.innerHeight || 1);
        let stopped = false;
        let paused = false;
        let raf = 0;
        let lastFrame = 0;
        let accumulator = 0;
        let frameCount = 0;
        let physicsSteps = 0;
        let slowFrames = 0;
        let fastSince = 0;
        let qualityLevel = 0;
        let dynamicLimit = MAX_FRAGMENTS;
        let physicsReady = false;
        let lastError = '';
        let records = new Map();
        let windowRects = [];
        let maskRects = [];
        let desktopRects = [];
        let lastSnapshotAt = 0;
        let lastSoundAt = 0;
        let lastPointerShearAt = 0;
        let lastBurstAt = 0;
        let tap = null;                   // the touch that might become a tap: { id, x, y, at }
        let pointerInside = false;
        let pointerX = -1000;
        let pointerY = -1000;
        let pointerVx = 0;
        let pointerVy = 0;
        let pointerAt = 0;
        let pointerActiveUntil = 0;
        let resizeObserver = null;
        let desktopObserver = null;
        let activeFragmentCount = 0;
        let lastRenderCost = 0;
        const pendingFractures = [];

        let RAPIER = null;
        let physicsWorld = null;
        let eventQueue = null;
        let floorBody = null;
        let pointerBody = null;
        let pointerCollider = null;
        const windowBodies = new Map();
        const iconBodies = new Map();
        const fragments = [];
        const colliderToFragment = new Map();
        const colliderToFrame = new Map();
        const colliderToIcon = new Map();
        const lastShearAt = new Map();
        const damageWindows = new Map();
        const assemblingWindows = new Map();
        let graphics = null;
        let diorama = null;
        let collisionCount = 0;
        let physicsCollisionCount = 0;
        let transitionUntil = 0;

        function noteError(stage, error) {
            const message = error && error.message ? error.message : String(error || 'unknown failure');
            lastError = String(stage) + ': ' + message.slice(0, 240);
            if (window.console && typeof window.console.warn === 'function') console.warn('[voxel_world FX] ' + lastError);
        }

        graphics = window.AuraVoxelGraphics.create({
            layer, fallback, fallbackContext, signal, fullWallpaper,
            readState: () => ({ width, height, stopped, paused, qualityLevel, windowRects, maskRects, desktopRects, damageWindows, assemblingWindows }),
            noteError,
            onContextLost: () => {
                if (raf) cancelAnimationFrame(raf);
                raf = 0;
            },
            onContextRestored: () => requestRender()
        });

        const fragmentPool = window.AuraVoxelFragments.create({
            fragments, colliderToFragment, graphics, materials: MATERIALS, maxFragments: MAX_FRAGMENTS,
            physics: () => ({ ready: physicsReady, world: physicsWorld, RAPIER }),
            onRecycled: () => { if (activeFragmentCount > 0) activeFragmentCount--; }
        });

        function snapshot(force) {
            const now = performance.now();
            if (!force && now - lastSnapshotAt < 110) return;
            lastSnapshotAt = now;
            const api = window.AuraDesktopWindowEffects;
            if (!api || typeof api.snapshot !== 'function') return;
            try {
                const source = api.snapshot();
                if (!Array.isArray(source)) return;
                const next = new Map();
                for (const record of source) {
                    if (!usable(record) || record.id == null) continue;
                    const rect = rectFor(record);
                    if (rect) next.set(String(record.id), { ...record, rect });
                }
                records = next;
                syncWindowBodies();
                refreshMaskRects();
                graphics.updateSceneInstances();
            } catch (_) { /* the shell can dispose while a snapshot is being read */ }
        }

        function refreshMaskRects() {
            const windows = [];
            for (const record of records.values()) {
                const rect = rectFor(record);
                if (!rect) continue;
                windows.push({ record, rect, z: Number(record.z) || 0 });
            }
            windowRects = windows.sort((a, b) => a.z - b.z).slice(-256);
            const next = [], desktop = [];
            for (const element of document.querySelectorAll(MASK_SELECTORS)) {
                if (!element.isConnected || element.hidden) continue;
                const style = window.getComputedStyle ? window.getComputedStyle(element) : null;
                if (style && (style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0)) continue;
                const rect = element.getBoundingClientRect();
                if (rect.width > 0 && rect.height > 0 && rect.bottom >= 0 && rect.right >= 0 && rect.top <= height && rect.left <= width) {
                    const target = element.matches('.vd-icon, .vd-widget') && !element.classList.contains('vd-dragging') ? desktop : next;
                    target.push({ x: rect.x, y: rect.y, width: rect.width, height: rect.height });
                }
            }
            maskRects = next.slice(0, 1024);
            desktopRects = desktop.slice(0, 1024);
            graphics.invalidateFallback();
            if (!graphics.rendererReady || graphics.contextLost) graphics.paintFallback();
        }

        // Share of the viewport the largest visible window covers; a window that hides the diorama slows it down.
        function coverageFraction() {
            let largest = 0;
            for (const item of windowRects) {
                const rect = item.rect;
                const w = Math.min(rect.x + rect.width, width) - Math.max(rect.x, 0);
                const h = Math.min(rect.y + rect.height, height) - Math.max(rect.y, 0);
                if (w > 0 && h > 0) largest = Math.max(largest, w * h);
            }
            return largest / Math.max(1, width * height);
        }

        function nodeAffectsMasks(node) {
            const element = node && node.nodeType === 1 ? node : node && node.parentElement;
            if (!element || element.closest('.vd-theme-fx-layer')) return false;
            try {
                return element.matches(MASK_SELECTORS) || !!element.closest(MASK_SELECTORS) ||
                    (!!element.querySelector && !!element.querySelector(MASK_SELECTORS));
            } catch (_) { return false; }
        }

        function mutationsAffectMasks(mutations) {
            for (const mutation of mutations) {
                if (nodeAffectsMasks(mutation.target)) return true;
                for (const node of mutation.addedNodes) if (nodeAffectsMasks(node)) return true;
                for (const node of mutation.removedNodes) if (nodeAffectsMasks(node)) return true;
            }
            return false;
        }

        function sound(eventName, strength, x, material) {
            if (!motion || document.hidden || !window.DesktopSounds || typeof window.DesktopSounds.play !== 'function') return;
            const now = performance.now();
            // A falling group has one audible texture, not one voice per cube.
            if (now - lastSoundAt < 250) return;
            lastSoundAt = now;
            const pan = clamp((x / Math.max(1, width)) * 2 - 1, -1, 1);
            try {
                window.DesktopSounds.play(eventName, {
                    material: MATERIALS[material] ? material : 'stone',
                    strength: clamp(strength, 0, 1),
                    pan
                });
            } catch (_) { /* sound support is optional */ }
        }

        function removeWindowBody(id) {
            const entry = windowBodies.get(String(id));
            if (!entry) return;
            for (const collider of entry.colliders || []) colliderToFrame.delete(collider.handle);
            if (physicsWorld) physicsWorld.removeRigidBody(entry.body);
            windowBodies.delete(String(id));
        }

        function syncWindowBodies() {
            if (!physicsReady || !physicsWorld) return;
            const wanted = new Set(records.keys());
            for (const [id, entry] of windowBodies) {
                if (!wanted.has(id)) {
                    removeWindowBody(id);
                }
            }
            for (const [id, record] of records) {
                const rect = rectFor(record);
                if (!rect) continue;
                let entry = windowBodies.get(id);
                if (entry && (Math.abs(entry.width - rect.width) > 1 || Math.abs(entry.height - rect.height) > 1)) {
                    removeWindowBody(id);
                    entry = null;
                }
                if (!entry) {
                    const centerX = toWorldX(rect.x + rect.width / 2, width);
                    const centerY = toWorldY(rect.y + rect.height / 2, height);
                    const body = physicsWorld.createRigidBody(RAPIER.RigidBodyDesc.kinematicPositionBased().setTranslation(centerX, centerY, 0));
                    const hw = rect.width / (2 * PIXELS_PER_METER);
                    const hh = rect.height / (2 * PIXELS_PER_METER);
                    const tilePx = frameStep(rect);
                    const tileWorld = tilePx / PIXELS_PER_METER;
                    const thickness = tileWorld * .46;
                    const colliders = [];
                    const addEdgeCollider = (side, index, cx, cy, hx, hy) => {
                        const collider = physicsWorld.createCollider(RAPIER.ColliderDesc.cuboid(hx, hy, .11)
                            .setTranslation(cx, cy, 0)
                            .setFriction(.78)
                            .setRestitution(.24)
                            .setActiveEvents(RAPIER.ActiveEvents.CONTACT_FORCE_EVENTS)
                            .setContactForceEventThreshold(.035), body);
                        colliders.push(collider);
                        colliderToFrame.set(collider.handle, { id, side, index });
                    };
                    const horizontalCount = Math.max(1, Math.ceil(rect.width / tilePx));
                    const verticalCount = Math.max(1, Math.ceil(rect.height / tilePx));
                    for (let i = 0; i < horizontalCount; i++) {
                        const x = -hw + (i + .5) * rect.width / horizontalCount / PIXELS_PER_METER;
                        const hx = rect.width / horizontalCount / PIXELS_PER_METER / 2 + .008;
                        addEdgeCollider('top', i, x, hh + thickness / 2, hx, thickness / 2);
                        addEdgeCollider('bottom', i, x, -hh - thickness / 2, hx, thickness / 2);
                    }
                    for (let i = 0; i < verticalCount; i++) {
                        const y = hh - (i + .5) * rect.height / verticalCount / PIXELS_PER_METER;
                        const hy = rect.height / verticalCount / PIXELS_PER_METER / 2 + .008;
                        addEdgeCollider('left', i, -hw - thickness / 2, y, thickness / 2, hy);
                        addEdgeCollider('right', i, hw + thickness / 2, y, thickness / 2, hy);
                    }
                    entry = { body, colliders, width: rect.width, height: rect.height, x: rect.x, y: rect.y };
                    windowBodies.set(id, entry);
                }
                const centerX = toWorldX(rect.x + rect.width / 2, width);
                const centerY = toWorldY(rect.y + rect.height / 2, height);
                entry.body.setNextKinematicTranslation({ x: centerX, y: centerY, z: 0 });
                const velocity = record.velocity || {};
                if (Math.abs(Number(velocity.x) || 0) + Math.abs(Number(velocity.y) || 0) < 8) {
                    entry.body.setLinvel({ x: 0, y: 0, z: 0 }, true);
                }
                entry.x = rect.x;
                entry.y = rect.y;
                entry.record = record;
            }
        }

        function syncIconBodies() {
            if (!physicsReady || !physicsWorld) return;
            const next = new Map();
            let ordinal = 0;
            for (const element of document.querySelectorAll('.vd-icon')) {
                if (!element.isConnected || element.hidden) continue;
                const rect = element.getBoundingClientRect();
                if (rect.width <= 1 || rect.height <= 1 || rect.right < 0 || rect.bottom < 0 || rect.left > width || rect.top > height) continue;
                const id = String(element.dataset.id || element.getAttribute('aria-label') || ordinal++);
                next.set(id, { rect: { x: rect.x, y: rect.y, width: rect.width, height: rect.height } });
            }
            for (const [id, body] of iconBodies) {
                if (!next.has(id)) {
                    if (body.collider) colliderToIcon.delete(body.collider.handle);
                    physicsWorld.removeRigidBody(body.body);
                    iconBodies.delete(id);
                }
            }
            for (const [id, item] of next) {
                const rect = item.rect;
                let entry = iconBodies.get(id);
                if (entry && (Math.abs(entry.width - rect.width) > 1 || Math.abs(entry.height - rect.height) > 1)) {
                    if (entry.collider) colliderToIcon.delete(entry.collider.handle);
                    physicsWorld.removeRigidBody(entry.body);
                    iconBodies.delete(id);
                    entry = null;
                }
                if (!entry) {
                    const body = physicsWorld.createRigidBody(RAPIER.RigidBodyDesc.fixed().setTranslation(
                        toWorldX(rect.x + rect.width / 2, width), toWorldY(rect.y + rect.height / 2, height), 0
                    ));
                    const collider = physicsWorld.createCollider(RAPIER.ColliderDesc.cuboid(
                        rect.width / (2 * PIXELS_PER_METER) + .035,
                        rect.height / (2 * PIXELS_PER_METER) + .035,
                        .12
                    ).setFriction(.8).setRestitution(.12)
                        .setActiveEvents(RAPIER.ActiveEvents.CONTACT_FORCE_EVENTS)
                        .setContactForceEventThreshold(.035), body);
                    entry = { body, collider, width: rect.width, height: rect.height, rect };
                    iconBodies.set(id, entry);
                    colliderToIcon.set(collider.handle, id);
                }
                entry.body.setTranslation({
                    x: toWorldX(rect.x + rect.width / 2, width),
                    y: toWorldY(rect.y + rect.height / 2, height),
                    z: 0
                }, true);
                entry.rect = rect;
            }
        }

        function resizeFallback() {
            width = Math.max(1, layer.clientWidth || window.innerWidth || 1);
            height = Math.max(1, layer.clientHeight || window.innerHeight || 1);
            graphics.invalidateFallback();
            graphics.paintFallback();
            if (diorama) diorama.resize();
            if (graphics.rendererReady) graphics.resizeRenderer();
            if (physicsReady) resizePhysicsFloor();
            snapshot(true);
        }

        function resizePhysicsFloor() {
            if (!physicsReady || !floorBody) return;
            const screenFloor = height - 58;
            const y = toWorldY(screenFloor, height) - .08;
            const collider = floorBody.collider(0);
            collider.setShape(new RAPIER.Cuboid(Math.max(1, width / (2 * PIXELS_PER_METER)), .08, 4));
            floorBody.setTranslation({ x: 0, y, z: 0 }, true);
            floorBody.wakeUp();
        }

        const materialForRecord = record => graphics.materialForRecord(record);
        const frameStep = rect => graphics.frameStep(rect);

        function edgeForVelocity(velocity) {
            const vx = Number(velocity && velocity.x) || 0;
            const vy = Number(velocity && velocity.y) || 0;
            return Math.abs(vx) > Math.abs(vy) ? (vx >= 0 ? 'right' : 'left') : (vy >= 0 ? 'bottom' : 'top');
        }

        function damageFrame(id, side, index, dragging) {
            const key = String(id);
            let damage = damageWindows.get(key);
            if (!damage) {
                damage = { holes: new Set(), repairAt: performance.now() + 1100, dragging: false, dragAt: 0 };
                damageWindows.set(key, damage);
            }
            if (damage.holes.size < 18) damage.holes.add(side + ':' + Math.max(0, index | 0));
            damage.dragging = !!dragging;
            damage.repairAt = dragging ? Infinity : performance.now() + 1100;
            if (dragging) damage.dragAt = performance.now();
            return damage;
        }

        function initPhysics() {
            const token = signal;
            Promise.resolve(RAPIER.init()).then(() => {
                if (!eventIsCurrent(token, stopped) || !graphics.rendererReady) return;
                physicsWorld = new RAPIER.World({ x: 0, y: -9.81, z: 0 });
                physicsWorld.timestep = FIXED_STEP;
                eventQueue = new RAPIER.EventQueue(true);
                floorBody = physicsWorld.createRigidBody(RAPIER.RigidBodyDesc.fixed().setTranslation(0, toWorldY(height - 58, height) - .08, 0));
                physicsWorld.createCollider(RAPIER.ColliderDesc.cuboid(Math.max(1, width / (2 * PIXELS_PER_METER)), .08, 4)
                    .setFriction(.9).setRestitution(.16)
                    .setActiveEvents(RAPIER.ActiveEvents.CONTACT_FORCE_EVENTS)
                    .setContactForceEventThreshold(.035), floorBody);
                pointerBody = physicsWorld.createRigidBody(RAPIER.RigidBodyDesc.kinematicPositionBased().setEnabled(false).setTranslation(0, 0, .2));
                pointerCollider = physicsWorld.createCollider(RAPIER.ColliderDesc.ball(.19).setFriction(.35).setRestitution(.28)
                    .setActiveEvents(RAPIER.ActiveEvents.CONTACT_FORCE_EVENTS).setContactForceEventThreshold(.02), pointerBody);
                physicsReady = true;
                syncWindowBodies();
                syncIconBodies();
                graphics.updateSceneInstances();
                while (pendingFractures.length) {
                    const item = pendingFractures.shift();
                    if (performance.now() - item.queuedAt <= 500) playFracture(item.record, item.rect, item.velocity, 1.7);
                }
                requestRender();
            }).catch(error => {
                graphics.setBackend(graphics.ready ? 'three-webgl-no-physics' : 'fallback');
                physicsReady = false;
                noteError('Rapier initialization', error);
            });
        }

        function spawnFragment(screenX, screenY, size, vx, vy, vz, color, material) {
            if (!motion || !physicsReady || activeFragmentCount >= dynamicLimit) return false;
            const acquired = fragmentPool.acquire();
            if (!acquired) return false;
            const { slot, index } = acquired;
            const x = toWorldX(screenX, width);
            const y = toWorldY(screenY, height);
            slot.size = clamp(size, 6, 18);
            const halfSize = slot.size / (2 * PIXELS_PER_METER);
            slot.collider.setShape(new RAPIER.Cuboid(halfSize, halfSize, halfSize));
            slot.age = 0;
            slot.settle = 0;
            slot.alpha = 1;
            slot.lastImpactAge = -1;
            slot.contactedStep = -10;
            slot.incoming = { x: vx, y: vy, z: vz };
            slot.active = true;
            slot.body.setTranslation({ x, y, z: .15 }, true);
            const initialAngle = randomBetween(-1, 1);
            slot.body.setRotation({ x: 0, y: 0, z: Math.sin(initialAngle / 2), w: Math.cos(initialAngle / 2) }, true);
            slot.body.setLinvel({ x: vx, y: vy, z: vz }, true);
            slot.body.setAngvel({ x: randomBetween(-3, 3), y: randomBetween(-3, 3), z: randomBetween(-6, 6) }, true);
            slot.body.wakeUp();
            slot.body.setEnabled(true);
            slot.material = MATERIALS[material] ? material : 'stone';
            slot.color = color || MATERIALS[slot.material][Math.floor(Math.random() * MATERIALS[slot.material].length)];
            graphics.setFragmentLook(index, slot.material, 1);
            activeFragmentCount++;
            graphics.commitFragments();
            requestRender();
            return true;
        }

        function playFracture(record, rect, velocity, countScale) {
            if (!motion || !physicsReady || !rect) return;
            const count = Math.max(20, Math.min(112, Math.round(rect.width * rect.height / 10500 * countScale)));
            const centerX = rect.x + rect.width / 2;
            const centerY = rect.y + rect.height / 2;
            const speedX = clamp(Number(velocity && velocity.x) || 0, -1600, 1600) / PIXELS_PER_METER;
            const speedY = -(clamp(Number(velocity && velocity.y) || 0, -1600, 1600) / PIXELS_PER_METER);
            const material = materialForRecord(record);
            const colors = MATERIALS[material];
            for (let i = 0; i < count; i++) {
                const x = rect.x + Math.random() * rect.width;
                const y = rect.y + Math.random() * rect.height;
                const dx = (x - centerX) / Math.max(1, rect.width * .5);
                const dy = (y - centerY) / Math.max(1, rect.height * .5);
                const impulse = randomBetween(1.15, 3.35) * (countScale > 1 ? 1.12 : 1);
                const color = colors[Math.floor(Math.random() * colors.length)];
                spawnFragment(x, y, randomBetween(8, 17),
                    dx * impulse + speedX * .48 + randomBetween(-1.3, 1.3),
                    Math.abs(dy) * impulse + 1.05 + speedY * .36 + randomBetween(-.65, 1.5),
                    randomBetween(-.5, .5), color, material);
            }
            const strength = clamp(Math.hypot(speedX, speedY) / 14 + count / 110, .25, 1);
            sound('window.fracture', strength, centerX, material);
        }

        function shearWindow(record, rect, velocity, contact, point) {
            const id = String(record.id);
            const speed = Math.hypot(Number(velocity && velocity.x) || 0, Number(velocity && velocity.y) || 0);
            const now = performance.now();
            if (!motion || record.silent || speed < (contact ? 280 : 920) || now - (lastShearAt.get(id) || 0) < 340) return;
            lastShearAt.set(id, now);
            const side = edgeForVelocity(velocity);
            const step = frameStep(rect);
            const indexCount = Math.max(1, Math.ceil((side === 'top' || side === 'bottom' ? rect.width : rect.height) / step));
            const sidePosition = side === 'top' || side === 'bottom'
                ? (point ? clamp(point.x - rect.x, 0, rect.width) : rect.width * .52)
                : (point ? clamp(point.y - rect.y, 0, rect.height) : rect.height * .52);
            const tileIndex = clamp(Math.floor(sidePosition / Math.max(1, side === 'top' || side === 'bottom' ? rect.width : rect.height) * indexCount), 0, indexCount - 1);
            damageFrame(id, side, tileIndex, !contact);
            const leadingX = side === 'left' ? rect.x : side === 'right' ? rect.x + rect.width : rect.x + rect.width * .52;
            const leadingY = side === 'top' ? rect.y : side === 'bottom' ? rect.y + rect.height : rect.y + rect.height * .52;
            const vx = clamp(Number(velocity.x) || 0, -1500, 1500) / PIXELS_PER_METER * .5;
            const vy = -(clamp(Number(velocity.y) || 0, -1500, 1500) / PIXELS_PER_METER * .3) + 1;
            const count = Math.min(14, Math.max(contact ? 3 : 4, Math.floor(speed / 180)));
            const material = materialForRecord(record);
            const colors = MATERIALS[material];
            for (let i = 0; i < count; i++) {
                spawnFragment(leadingX + randomBetween(-rect.width * .22, rect.width * .22), leadingY + randomBetween(-rect.height * .22, rect.height * .22), randomBetween(7, 13),
                    vx + randomBetween(-1.7, 1.7), vy + randomBetween(-.7, 1.3), randomBetween(-.3, .3),
                    colors[Math.floor(Math.random() * colors.length)], material);
            }
            sound('window.fracture', clamp(speed / 1800, .22, .75), leadingX, material);
            graphics.updateSceneInstances();
        }

        function intersects(a, b) {
            return !!a && !!b && a.x < b.x + b.width && a.x + a.width > b.x && a.y < b.y + b.height && a.y + a.height > b.y;
        }

        function detectMovingWindowContact(record, previousRect, rect, velocity) {
            if (!motion || !previousRect || !rect) return false;
            const speed = Math.hypot(Number(velocity && velocity.x) || 0, Number(velocity && velocity.y) || 0);
            if (speed < 280) return false;
            const swept = {
                x: Math.min(previousRect.x, rect.x),
                y: Math.min(previousRect.y, rect.y),
                width: Math.max(previousRect.x + previousRect.width, rect.x + rect.width) - Math.min(previousRect.x, rect.x),
                height: Math.max(previousRect.y + previousRect.height, rect.y + rect.height) - Math.min(previousRect.y, rect.y)
            };
            for (const element of document.querySelectorAll('.vd-icon')) {
                if (!element.isConnected || element.hidden) continue;
                const obstacle = element.getBoundingClientRect();
                if (obstacle.width > 0 && obstacle.height > 0 && intersects(swept, obstacle)) {
                    shearWindow(record, rect, velocity, true);
                    return true;
                }
            }
            for (const other of records.values()) {
                if (String(other.id) === String(record.id) || (Number(other.z) || 0) >= (Number(record.z) || 0)) continue;
                const obstacle = rectFor(other);
                if (obstacle && intersects(swept, obstacle)) {
                    shearWindow(record, rect, velocity, true);
                    return true;
                }
            }
            return false;
        }

        function onWindowEvent(event) {
            const record = event && event.detail;
            if (!record || record.id == null) return;
            const id = String(record.id);
            const rect = rectFor(record);
            if (record.phase === 'minimize' || record.phase === 'restore' || record.phase === 'maximize' || record.phase === 'snap') {
                transitionUntil = Math.max(transitionUntil, performance.now() + 480);
            }
            if (record.phase === 'close') {
                if (!record.silent && rect && motion) {
                    if (physicsReady) playFracture(record, rect, record.velocity || {}, 1.7);
                    else {
                        if (pendingFractures.length >= 8) pendingFractures.shift();
                        pendingFractures.push({ record, rect, velocity: record.velocity || {}, queuedAt: performance.now() });
                    }
                    for (let i = 0; i < 8; i++) {
                        const dustX = rect.x + (i + Math.random()) / 8 * rect.width;
                        graphics.spawnDust(dustX, rect.y + rect.height, 1, null, dustX < rect.x + rect.width / 2 ? -1 : 1);
                    }
                }
                removeWindowBody(id);
                damageWindows.delete(id);
                assemblingWindows.delete(id);
                records.delete(id);
            } else {
                const previous = records.get(id);
                const previousRect = previous && (previous.rect || rectFor(previous));
                if (usable(record) && rect) records.set(id, { ...record, rect });
                else {
                    records.delete(id);
                    assemblingWindows.delete(id);
                }
                if (record.phase === 'open') {
                    damageWindows.delete(id);
                    if (motion && !record.silent && rect && usable(record)) assemblingWindows.set(id, { at: performance.now(), duration: ASSEMBLY_MS, landed: 0 });
                }
                if ((record.phase === 'drag' || record.phase === 'resize') && !record.silent && rect) {
                    const velocity = record.velocity || {};
                    shearWindow(record, rect, velocity, false);
                    detectMovingWindowContact(record, previousRect, rect, velocity);
                    const damage = damageWindows.get(id);
                    if (damage) {
                        damage.dragging = true;
                        damage.repairAt = Infinity;
                        damage.dragAt = performance.now();
                    }
                }
                if ((record.phase === 'drag-end' || record.phase === 'resize-end') && damageWindows.has(id)) {
                    const damage = damageWindows.get(id);
                    damage.dragging = false;
                    damage.repairAt = performance.now() + 1100;
                }
            }
            snapshot(true);
            refreshMaskRects();
            graphics.updateSceneInstances();
            requestRender();
        }

        function onPointerMove(event) {
            if (tap && event.pointerId === tap.id && Math.hypot(event.clientX - tap.x, event.clientY - tap.y) >= TAP_SLOP_PX) tap = null;
            if (motion && diorama) diorama.setPointer(clamp(event.clientX / width * 2 - 1, -1, 1), clamp(event.clientY / height * 2 - 1, -1, 1), true);
            if (!motion || !physicsReady || event.pointerType === 'touch') return;
            const now = performance.now();
            const dt = pointerAt ? Math.max(.008, (now - pointerAt) / 1000) : .016;
            const dx = event.clientX - pointerX;
            const dy = event.clientY - pointerY;
            pointerVx = clamp(dx / dt, -2200, 2200);
            pointerVy = clamp(dy / dt, -2200, 2200);
            pointerX = event.clientX;
            pointerY = event.clientY;
            pointerAt = now;
            pointerActiveUntil = now + 150;
            pointerInside = true;
            if (!pointerBody.isEnabled()) pointerBody.setEnabled(true);
            pointerBody.setNextKinematicTranslation({
                x: toWorldX(pointerX, width),
                y: toWorldY(pointerY, height),
                z: .18
            });

            const speed = Math.hypot(pointerVx, pointerVy);
            const time = performance.now();
            if (speed > 1080 && time - lastPointerShearAt > 550) {
                for (const record of records.values()) {
                    const rect = rectFor(record);
                    if (!rect) continue;
                    const outsideX = Math.max(rect.x - pointerX, 0, pointerX - rect.x - rect.width);
                    const outsideY = Math.max(rect.y - pointerY, 0, pointerY - rect.y - rect.height);
                    const edgeDistance = Math.min(Math.abs(pointerX - rect.x), Math.abs(pointerX - rect.x - rect.width), Math.abs(pointerY - rect.y), Math.abs(pointerY - rect.y - rect.height));
                    if (outsideX + outsideY > 8 || edgeDistance > 10) continue;
                    shearWindow(record, rect, { x: pointerVx, y: pointerVy }, true, { x: pointerX, y: pointerY });
                    lastPointerShearAt = time;
                    break;
                }
            }
            requestRender();
        }

        function burstAt(x, y) {
            const now = performance.now();
            if (!motion || !physicsReady || paused || now - lastBurstAt < BURST_INTERVAL_MS) return;
            lastBurstAt = now;
            // The cursor's own collider would trap the fragments, so it rests until the next pointer move.
            if (pointerBody && pointerBody.isEnabled()) pointerBody.setEnabled(false);
            for (let i = 0; i < 7; i++) {
                spawnFragment(x + randomBetween(-5, 5), y + randomBetween(-4, 4), randomBetween(6, 11),
                    randomBetween(-1.4, 1.4), randomBetween(2.2, 3.6), randomBetween(-.3, .3), null, 'earth');
            }
            graphics.spawnDust(x, y, 4);
            sound('window.impact', .32, x, 'earth');
            requestRender();
        }

        // Mouse and pen burst on press. A touch may be the start of a pan of the workspace, so it bursts on release instead,
        // and only when it was a short tap (see onPointerEnd).
        function onPointerDown(event) {
            if (event.pointerType === 'touch') tap = null;   // any new touch voids a pending tap (a second finger is a gesture)
            const target = event.target;
            if (!motion || !physicsReady || paused || event.button !== 0 || !(target instanceof Element)) return;
            if (target.closest(BURST_BLOCKERS) || !target.closest('.vd-workspace')) return;
            if (event.pointerType !== 'touch') burstAt(event.clientX, event.clientY);
            else if (event.isPrimary) tap = { id: event.pointerId, x: event.clientX, y: event.clientY, at: performance.now() };
        }

        function onPointerEnd(event) {
            if (!tap || tap.id !== event.pointerId) return;
            const { x, y, at } = tap;
            tap = null;
            if (event.type === 'pointerup' && performance.now() - at < TAP_MAX_MS && Math.hypot(event.clientX - x, event.clientY - y) < TAP_SLOP_PX) burstAt(x, y);
        }

        function onPointerLeave() {
            if (diorama) diorama.setPointer(0, 0, false);
            pointerInside = false;
            pointerActiveUntil = 0;
            if (pointerBody && pointerBody.isEnabled()) pointerBody.setEnabled(false);
        }

        function onContactForce(contact) {
            const force = contact.totalForceMagnitude();
            if (!Number.isFinite(force) || force < .035) return;
            physicsCollisionCount++;
            const handle1 = contact.collider1();
            const handle2 = contact.collider2();
            const pointerHandle = pointerCollider && pointerCollider.handle;
            const pointerContact = pointerHandle === handle1 || pointerHandle === handle2;
            const otherHandle = pointerHandle === handle1 ? handle2 : handle1;
            if (pointerContact) {
                collisionCount++;
                const frame = colliderToFrame.get(otherHandle);
                const iconId = colliderToIcon.get(otherHandle);
                const speed = Math.hypot(pointerVx, pointerVy);
                if (frame && speed > 520) {
                    const record = records.get(frame.id);
                    const material = materialForRecord(record);
                    const now = performance.now();
                    if (now - (lastShearAt.get(frame.id) || 0) > 260) {
                        lastShearAt.set(frame.id, now);
                        damageFrame(frame.id, frame.side, frame.index, false);
                        const colors = MATERIALS[material];
                        for (let i = 0; i < 3; i++) spawnFragment(pointerX, pointerY, randomBetween(7, 12),
                            pointerVx / PIXELS_PER_METER * .28 + randomBetween(-1.1, 1.1),
                            -pointerVy / PIXELS_PER_METER * .2 + 1 + randomBetween(-.5, 1), randomBetween(-.3, .3),
                            colors[Math.floor(Math.random() * colors.length)], material);
                        sound('window.fracture', clamp(force * 1.8 + speed / 2600, .22, .72), pointerX, material);
                        graphics.updateSceneInstances();
                    }
                } else if (iconId && speed > 700) {
                    const material = 'wood';
                    const colors = MATERIALS[material];
                    for (let i = 0; i < 2; i++) spawnFragment(pointerX, pointerY, randomBetween(6, 10),
                        pointerVx / PIXELS_PER_METER * .25 + randomBetween(-1.2, 1.2),
                        -pointerVy / PIXELS_PER_METER * .18 + .9 + randomBetween(-.4, 1), randomBetween(-.25, .25),
                        colors[Math.floor(Math.random() * colors.length)], material);
                    sound('window.impact', clamp(force * 1.7, .15, .68), pointerX, material);
                }
                requestRender();
                return;
            }
            const first = colliderToFragment.get(handle1);
            const second = colliderToFragment.get(handle2);
            const slot = first && first.active ? first : second && second.active ? second : null;
            if (!slot) return;
            collisionCount++;
            if (first && first.active) first.contactedStep = physicsSteps;
            if (second && second.active) second.contactedStep = physicsSteps;
            // Contact force also describes the continuous weight of a resting pile.
            // Only incoming normal velocity is an audible impact; never amplify support forces.
            const a = first && first.active ? first.incoming : { x: 0, y: 0, z: 0 };
            const b = second && second.active ? second.incoming : { x: 0, y: 0, z: 0 };
            const normal = contact.maxForceDirection();
            const approach = Math.abs((a.x - b.x) * normal.x + (a.y - b.y) * normal.y + (a.z - b.z) * normal.z);
            if (approach < .8 || slot.body.isSleeping() || slot.age - slot.lastImpactAge < .32 || slot.age > 3.8) return;
            slot.lastImpactAge = slot.age;
            if (first && first.active) first.lastImpactAge = first.age;
            if (second && second.active) second.lastImpactAge = second.age;
            const translation = slot.body.translation();
            const screenX = translation.x * PIXELS_PER_METER + width / 2;
            sound('window.impact', clamp((approach - .65) / 7, .06, .65), screenX, slot.material);
        }

        function physicsStep(dt) {
            if (!physicsReady || !physicsWorld) return;
            if (pointerBody && pointerInside && performance.now() - pointerAt > 850) {
                pointerInside = false;
                pointerBody.setEnabled(false);
            }
            for (const slot of fragments) if (slot.active) slot.incoming = slot.body.linvel();
            physicsWorld.step(eventQueue);
            physicsSteps++;
            eventQueue.drainContactForceEvents(onContactForce);
            for (let i = 0; i < fragments.length; i++) {
                const slot = fragments[i];
                if (!slot.active) continue;
                slot.age += dt;
                const velocity = slot.body.linvel();
                const speed = Math.hypot(velocity.x, velocity.y, velocity.z);
                const pos = slot.body.translation();
                const angular = slot.body.angvel();
                const supported = slot.body.isSleeping() || physicsSteps - slot.contactedStep <= 2;
                if (supported && speed < .12 && Math.hypot(angular.x, angular.y, angular.z) < .8) slot.settle += dt;
                else slot.settle = 0;
                if (slot.settle > .3 && !slot.body.isSleeping()) slot.body.sleep();
                const fade = clamp((slot.age - 3.8) / 1.8, 0, 1);
                slot.alpha = 1 - fade;
                if (slot.age > 6.3 || slot.settle > 1.6 || pos.y < -height / PIXELS_PER_METER - 2) {
                    fragmentPool.recycle(slot, i);
                }
                graphics.setFragmentMatrix(slot, i);
            }
            graphics.commitFragments();
        }

        function adjustQuality(cost, now) {
            if (cost > 24) {
                slowFrames++;
                fastSince = 0;
                if (slowFrames >= 14 && qualityLevel < 4) {
                    qualityLevel++;
                    slowFrames = 0;
                    dynamicLimit = qualityLevel >= 4 ? 64 : qualityLevel >= 3 ? 128 : MAX_FRAGMENTS;
                    graphics.resizeRenderer();
                }
            } else {
                slowFrames = Math.max(0, slowFrames - 1);
                if (cost < 12 && qualityLevel > 0) {
                    if (!fastSince) fastSince = now;
                    if (now - fastSince > 7000) {
                        qualityLevel--;
                        dynamicLimit = qualityLevel >= 4 ? 64 : qualityLevel >= 3 ? 128 : MAX_FRAGMENTS;
                        fastSince = now;
                        graphics.resizeRenderer();
                    }
                } else fastSince = 0;
            }
        }

        function renderOnce() {
            if (!graphics || paused || stopped) return;
            snapshot(false);
            refreshMaskRects();
            const cost = graphics.render();
            if (cost == null) { graphics.paintFallback(); return; }
            frameCount++;
            lastRenderCost = cost;
            adjustQuality(lastRenderCost, performance.now());
        }

        // A window can leave the snapshot mid-effect (space switch, minimise, visibility flip) and a drag-end can fail to arrive;
        // neither may keep the loop alive: stale assemblies are dropped, and a held drag damage is repaired normally 3 s after the last drag.
        function reapStaleEffects(now) {
            for (const [id, assembly] of assemblingWindows) {
                if (now - assembly.at > assembly.duration + 100 || !records.has(id)) assemblingWindows.delete(id);
            }
            for (const damage of damageWindows.values()) {
                if (damage.dragging && now - damage.dragAt > DRAG_HOLD_MAX_MS) { damage.dragging = false; damage.repairAt = now + 1100; }
            }
        }

        function animate(time) {
            raf = 0;
            if (stopped || paused || !graphics.rendererReady || graphics.contextLost) return;
            const begin = performance.now();
            const frameInterval = lastFrame ? Math.max(0, time - lastFrame) : 1000 / 60;
            const dt = lastFrame ? clamp((time - lastFrame) / 1000, 0, .08) : FIXED_STEP;
            lastFrame = time;
            accumulator = Math.min(accumulator + dt, FIXED_STEP * 5);
            if (physicsReady) {
                while (accumulator >= FIXED_STEP) {
                    physicsStep(FIXED_STEP);
                    accumulator -= FIXED_STEP;
                }
            }
            if (transitionUntil && time >= transitionUntil) {
                transitionUntil = 0;
                snapshot(true);
            }
            snapshot(false);
            graphics.updateDust(dt);
            reapStaleEffects(begin);
            // The update also settles the tiles and removes an assembly once it has finished.
            if (assemblingWindows.size) graphics.updateSceneInstances();
            const assembling = assemblingWindows.size > 0;
            const cost = graphics.render();
            frameCount++;
            const callbackCost = performance.now() - begin;
            lastRenderCost = cost == null ? callbackCost : cost;
            adjustQuality(Math.max(frameInterval, callbackCost), performance.now());
            let repairing = false;
            for (const [id, damage] of damageWindows) {
                if (performance.now() >= damage.repairAt) {
                    damageWindows.delete(id);
                    graphics.updateSceneInstances();
                } else repairing = true;
            }
            if (activeFragmentCount > 0 || repairing || assembling || graphics.dustActive() || performance.now() < pointerActiveUntil || performance.now() < transitionUntil) {
                if (!raf) raf = requestAnimationFrame(animate);
            } else {
                // Ignore the idle gap before the next animation burst.
                lastFrame = 0;
                renderOnce();
            }
        }

        function requestRender() {
            if (stopped || paused) return;
            if (!motion || (activeFragmentCount === 0 && damageWindows.size === 0 && assemblingWindows.size === 0 && !graphics.dustActive() &&
                performance.now() >= pointerActiveUntil && performance.now() >= transitionUntil)) {
                if (graphics.rendererReady) renderOnce();
                else graphics.paintFallback();
                return;
            }
            if (!raf) raf = requestAnimationFrame(animate);
        }

        function onRefresh() {
            snapshot(true);
            requestRender();
            refreshMaskRects();
            syncIconBodies();
            graphics.updateSceneInstances();
            requestRender();
        }

        function pause() {
            paused = true;
            if (raf) cancelAnimationFrame(raf);
            raf = 0;
            lastFrame = 0;
            accumulator = 0;
            if (diorama) diorama.pause();
            if (pointerBody && pointerBody.isEnabled()) pointerBody.setEnabled(false);
        }

        function resume() {
            if (stopped) return;
            paused = false;
            if (diorama) diorama.resume();
            snapshot(true);
            refreshMaskRects();
            requestRender();
        }

        function inspect() {
            const fragmentSamples = [];
            const materialTypes = new Set();
            for (const slot of fragments) {
                if (!slot.active) continue;
                if (materialTypes.size < 3) materialTypes.add(slot.material);
                if (fragmentSamples.length < 4) {
                    const position = slot.body.translation();
                    const rotation = slot.body.rotation();
                    fragmentSamples.push({
                        x: Math.round(position.x * 1000) / 1000,
                        y: Math.round(position.y * 1000) / 1000,
                        z: Math.round(position.z * 1000) / 1000,
                        rotation: {
                            x: Math.round(rotation.x * 1000) / 1000,
                            y: Math.round(rotation.y * 1000) / 1000,
                            z: Math.round(rotation.z * 1000) / 1000,
                            w: Math.round(rotation.w * 1000) / 1000
                        },
                        material: slot.material,
                        age: Math.round(slot.age * 100) / 100
                    });
                }
            }
            const graphicsState = graphics.inspect();
            return {
                activeId: 'voxel_world',
                motion,
                paused,
                ready: graphicsState.ready,
                renderer: graphicsState.renderer,
                rendererReady: graphicsState.rendererReady,
                physicsReady,
                fragmentCount: activeFragmentCount,
                maxFragments: MAX_FRAGMENTS,
                fragmentSamples,
                materialTypes: Array.from(materialTypes),
                physicsSteps,
                frameCount,
                rafPending: !!raf,
                quality: qualityLevel,
                lastRenderCostMs: Math.round(lastRenderCost * 10) / 10,
                frameVoxelCount: graphicsState.frameVoxelCount,
                maxFrameVoxels: graphicsState.maxFrameVoxels,
                backdropVoxelCount: diorama ? diorama.voxelCount : 0,
                collisionCount,
                physicsCollisionCount,
                windowCount: records.size,
                windowOccluderCount: windowRects.length,
                wallpaper: graphicsState.wallpaper,
                scene: diorama ? diorama.inspect() : null,
                error: lastError
            };
        }

        // Test and debug hooks (AuraDesktopThemeFx.command): the diorama's hour, frozen clock and quality lock.
        function command(name, payload) {
            if (diorama) {
                const number = key => (payload && payload[key] != null && Number.isFinite(Number(payload[key])) ? Number(payload[key]) : null);
                if (name === 'voxel.time') diorama.setHourOverride(number('hour'));
                else if (name === 'voxel.freeze') diorama.setFreeze(number('t'));
                else if (name === 'voxel.quality') diorama.setQuality(number('level'));
            }
            return diorama ? diorama.inspect() : null;
        }

        function stop() {
            if (stopped) return;
            stopped = true;
            if (raf) cancelAnimationFrame(raf);
            raf = 0;
            if (resizeObserver) resizeObserver.disconnect();
            if (desktopObserver) desktopObserver.disconnect();
            if (diorama) diorama.dispose();
            diorama = null;
            if (physicsWorld) {
                for (const slot of fragments) slot.body.setEnabled(false);
                for (const entry of windowBodies.values()) physicsWorld.removeRigidBody(entry.body);
                for (const entry of iconBodies.values()) physicsWorld.removeRigidBody(entry.body);
                windowBodies.clear();
                iconBodies.clear();
                try { physicsWorld.free(); } catch (_) { }
            }
            if (eventQueue) { try { eventQueue.free(); } catch (_) { } }
            graphics.dispose();
            fallback.remove();
            records.clear();
            fragments.length = 0;
            colliderToFragment.clear();
            colliderToFrame.clear();
            colliderToIcon.clear();
            maskRects.length = 0;
            desktopRects.length = 0;
            windowRects.length = 0;
            damageWindows.clear();
            assemblingWindows.clear();
            pendingFractures.length = 0;
        }

        function loaded(vendor) {
            if (!eventIsCurrent(signal, stopped)) return;
            RAPIER = vendor && vendor.RAPIER;
            if (!graphics.createScene(vendor)) {
                if (graphics.backend === 'loading') graphics.setBackend('fallback');
                if (!lastError) noteError('scene initialization', 'WebGL scene could not be created');
                graphics.paintFallback();
                return;
            }
            if (fullWallpaper && window.AuraVoxelScene) {
                try {
                    diorama = window.AuraVoxelScene.create({
                        THREE: vendor.THREE, motion,
                        readState: () => ({ width, height, coverage: coverageFraction() }),
                        noteError
                    });
                    if (paused) diorama.pause();
                } catch (error) {
                    diorama = null;
                    noteError('diorama scene', error);
                }
            }
            if (motion) initPhysics();
            snapshot(true);
            refreshMaskRects();
            if (motion) syncIconBodies();
            graphics.updateSceneInstances();
            requestRender();
        }

        resizeObserver = typeof ResizeObserver === 'function' ? new ResizeObserver(resizeFallback) : null;
        if (resizeObserver) resizeObserver.observe(layer);
        window.addEventListener('resize', resizeFallback, { passive: true, signal });
        window.addEventListener('aurago:desktop-window', onWindowEvent, { signal });
        window.addEventListener('aurago:desktop-windows-refresh', onRefresh, { signal });
        document.addEventListener('pointermove', onPointerMove, { passive: true, signal });
        document.addEventListener('pointerdown', onPointerDown, { passive: true, signal });
        document.addEventListener('pointerup', onPointerEnd, { passive: true, signal });
        document.addEventListener('pointercancel', onPointerEnd, { passive: true, signal });
        document.addEventListener('pointerleave', onPointerLeave, { passive: true, signal });
        document.addEventListener('visibilitychange', () => { if (document.hidden) pause(); else resume(); }, { signal });
        if (signal && typeof signal.addEventListener === 'function') signal.addEventListener('abort', stop, { once: true });
        if (typeof MutationObserver === 'function') {
            desktopObserver = new MutationObserver(mutations => {
                if (!mutationsAffectMasks(mutations)) return;
                refreshMaskRects();
                if (physicsReady) syncIconBodies();
                graphics.updateSceneInstances();
                requestRender();
            });
            if (document.body) desktopObserver.observe(document.body, {
                childList: true,
                subtree: true,
                attributes: true,
                attributeFilter: ['style', 'class', 'hidden']
            });
        }

        resizeFallback();
        snapshot(true);
        refreshMaskRects();
        if (!fallbackContext) graphics.setBackend('no-canvas');
        const runtimeURL = new URL(VENDOR_URL, window.location.href).href;
        import(runtimeURL).then(loaded).catch(error => {
            if (!eventIsCurrent(signal, stopped)) return;
            graphics.setBackend('fallback');
            noteError('vendor import', error);
            graphics.paintFallback();
        });
        return { pause, resume, stop, inspect, command };
    }

    window.AuraDesktopThemeFx.register('voxel_world', { start });
})();
