(function () {
    'use strict';

    // DOM-free state and physics of the Green Slime gel (no document, no performance, no Math.random).
    // green_slime.js feeds it window geometry and pointer input, takes the finished scene out and hands
    // that to a renderer. The simulation is split over four files (bundle order): green_slime_geometry.js
    // (constants, rounded-rectangle geometry, profile), green_slime_bubbles.js, green_slime_touch.js and
    // this one, which owns the surfaces, the drops and the per-frame update and exports
    // window.AuraGreenSlimeSim = { createSim, geometry, constants, strandRadii }.
    //
    // Every window owns
    //  - 32 spring nodes (the old jelly contour: breathing, drag inertia, hover attraction),
    //  - a 128-sample periodic ripple wave over the arclength of its rounded rectangle,
    //  - drips, bubbles and a radius table (outer gel distance from the window edge per arclength sample).

    const kit = window.AuraGreenSlimeGeometry;
    const { clamp, lerp, wrap, cyclicDelta, inRect, hashString, mulberry32, geometry, surfaceTargets, buildProfile, softFloor, SURFACE_NORMALS } = kit;
    const { SURFACE_SAMPLES, TABLE_SAMPLES, MAX_DROPS, MAX_SPRAY, MAX_SPLATS, MIN_GEL_RADIUS, MAX_GEL_RADIUS, MAX_DEFORMATION,
        SAG_TOP, SAG_BOTTOM, GRAVITY, SPRAY_GRAVITY, RIPPLE_SPEED, RIPPLE_DAMPING, RIPPLE_STIFFNESS, RIPPLE_LIMIT, TWO_PI } = kit.constants;
    const { layout, pointAt, project, normalizeRadii } = geometry;

    const EMPTY_ENV = Object.freeze({ dt: 0, now: 0, width: 0, height: 0, dpr: 1, windows: [], masks: [], taskbar: null });

    function createSim(options) {
        const motion = !!(options && options.motion);
        const rng = mulberry32(options && options.seed != null ? options.seed : 0x2c9f1a);
        const surfaces = new Map();
        const motionDrives = new Map();
        const drops = [];
        const spray = [];
        const splats = [];
        const counters = {
            dropCount: 0, sprayCount: 0, bubbleCount: 0, bubbleBursts: 0, strandCount: 0, strandSnaps: 0,
            absorbedDrops: 0, splatCount: 0, pokeCount: 0
        };
        const stageEntries = { nucleate: 0, grow: 0, hold: 0, burst: 0, crater: 0 };
        // Shared with the parts: the clock and the geometry of the latest update.
        const state = { clock: 0, env: EMPTY_ENV };
        const scratch = {};
        let maxDisplacement = 0;
        let currentMaxDisplacement = 0;

        function jitter(low, high) { return low + rng() * (high - low); }

        const core = { motion, rng, jitter, surfaces, state, counters, stageEntries,
            radiusAt, blockedAt, rippleImpulse, nudgeNodes, addDrop, addSpray };
        const bubbles = window.AuraGreenSlimeBubbles.create(core);
        const touch = window.AuraGreenSlimeTouch.create(core);

        // -----------------------------------------------------------------------------------------
        // Surfaces

        function createSurface(id) {
            const seed = Array.from(id).reduce((sum, letter) => sum + letter.charCodeAt(0), 0);
            return {
                id,
                // Only local deformation is sprung. Window translation/resize reprojects every
                // node immediately, without storing world lag.
                nodes: surfaceTargets({ x: 0, y: 0, width: 400, height: 300 }).map(point => ({ ...point, dx: 0, dy: 0, vx: 0, vy: 0 })),
                seed: seed * .17,
                hash: hashString(id),
                drips: [17, 19, 21, 22, 13, 27].map((node, i) => ({ node,
                    period: 4.6 + i * .61, offset: (seed % 17) * .23 + i * 1.13, cycle: -1, sw: 0, swv: 0 })),
                ripple: new Float32Array(TABLE_SAMPLES),
                rippleV: new Float32Array(TABLE_SAMPLES),
                rippleLive: false,
                energy: 0,
                radius: new Float32Array(TABLE_SAMPLES).fill((SAG_TOP + SAG_BOTTOM) / 2),
                off: new Float64Array(SURFACE_SAMPLES),
                lastX: new Float64Array(SURFACE_SAMPLES),
                lastY: new Float64Array(SURFACE_SAMPLES),
                slope: new Float64Array(SURFACE_SAMPLES),
                grow: 1, growV: 0,
                z: 0, active: false,
                sig: null, layout: null, profile: null,
                winV: { x: 0, y: 0 }, dragAt: -9, flingAt: -9, wakeAt: -9,
                bubbles: [], pending: [],
                hoverAmp: 0, hoverS: 0, hover: null,
                pull: null,
                frozenKey: '', frozen: null,
                age: 0
            };
        }

        function surfaceFor(id, rect) {
            let surface = surfaces.get(id);
            if (!surface) {
                surface = createSurface(id);
                surfaces.set(id, surface);
            }
            const r = normalizeRadii(rect.width, rect.height, rect.radii);
            const sig = [rect.x, rect.y, rect.width, rect.height, r[0], r[1], r[2], r[3]];
            if (!surface.sig || sig.some((value, i) => value !== surface.sig[i])) {
                const shape = surface.sig && sig.slice(2).every((value, i) => value === surface.sig[i + 2]);
                const previousLength = surface.layout ? surface.layout.length : 0;
                surface.sig = sig;
                surface.layout = layout(rect, r);
                if (!shape || !surface.profile) surface.profile = buildProfile(rect.width, rect.height, r);
                if (previousLength > 0 && Math.abs(surface.layout.length - previousLength) > .01) {
                    // Resizing stretches the perimeter: what sits on it (bubbles, a thread's anchor) moves
                    // along proportionally instead of jumping to another place.
                    const ratio = surface.layout.length / previousLength;
                    bubbles.rescale(surface, ratio);
                    touch.rescale(id, ratio);
                    for (const item of surface.pending) if (item.s != null) item.s *= ratio;
                    if (surface.pull) surface.pull.s *= ratio;
                    surface.hoverS *= ratio;
                }
            }
            return surface;
        }

        function removeWindow(id) {
            surfaces.delete(id);
            motionDrives.delete(id);
            touch.clear(id);
        }

        function radiusAt(surface, s) {
            const f = wrap(s, surface.layout.length) / surface.layout.length * TABLE_SAMPLES - .5;
            const k0 = Math.floor(f), t = f - k0;
            return lerp(surface.radius[wrap(k0, TABLE_SAMPLES)], surface.radius[wrap(k0 + 1, TABLE_SAMPLES)], t);
        }

        // Is (x, y) hidden behind a window above this surface or behind a solid overlay?
        function blockedAt(surface, x, y) {
            const env = state.env;
            for (const win of env.windows) {
                if (String(win.id) !== surface.id && win.z >= surface.z && inRect(win.rect, x, y)) return true;
            }
            for (const mask of env.masks) if (!mask.desktop && inRect(mask, x, y)) return true;
            return false;
        }

        // -----------------------------------------------------------------------------------------
        // Ripple wave

        function rippleImpulse(surface, s, amplitude, width) {
            const L = surface.layout.length, ds = L / TABLE_SAMPLES;
            const sigma = Math.max(ds * .8, width * .5);
            const reach = Math.ceil(sigma * 3 / ds);
            const center = Math.round(s / ds - .5);
            for (let j = -reach; j <= reach; j++) {
                const k = wrap(center + j, TABLE_SAMPLES);
                const d = cyclicDelta((k + .5) * ds, s, L);
                surface.ripple[k] = clamp(surface.ripple[k] + amplitude * Math.exp(-d * d / (2 * sigma * sigma)), -RIPPLE_LIMIT, RIPPLE_LIMIT);
            }
            surface.rippleLive = true;
        }

        // One-dimensional damped wave equation u_tt = c^2 u_xx - gamma u_t - kappa u over the
        // periodic arclength, symplectic Euler with CFL sub-steps.
        function rippleStep(surface, dt) {
            if (!surface.rippleLive || dt <= 0) return;
            const u = surface.ripple, v = surface.rippleV;
            const ds = surface.layout.length / TABLE_SAMPLES;
            const cs = RIPPLE_SPEED / ds;
            const steps = clamp(Math.ceil(cs * dt / .8), 1, 8);
            const h = dt / steps, c2 = cs * cs;
            for (let n = 0; n < steps; n++) {
                for (let k = 0; k < TABLE_SAMPLES; k++) {
                    const laplace = u[(k + TABLE_SAMPLES - 1) & 127] + u[(k + 1) & 127] - 2 * u[k];
                    v[k] += (c2 * laplace - RIPPLE_DAMPING * v[k] - RIPPLE_STIFFNESS * u[k]) * h;
                }
                for (let k = 0; k < TABLE_SAMPLES; k++) u[k] += v[k] * h;
            }
            let energy = 0;
            for (let k = 0; k < TABLE_SAMPLES; k++) energy += u[k] * u[k] + v[k] * v[k] / c2;
            surface.energy = energy;
            if (energy < 2e-4) {
                u.fill(0); v.fill(0);
                surface.energy = 0;
                surface.rippleLive = false;
            }
        }

        function nudgeNodes(surface, s, amount, sigma) {
            const L = surface.layout.length, nodeS = surface.profile.nodeS;
            for (let i = 0; i < surface.nodes.length; i++) {
                const d = cyclicDelta(nodeS[i], s, L);
                const weight = Math.exp(-d * d / (2 * sigma * sigma));
                if (weight < .02) continue;
                const normal = SURFACE_NORMALS[Math.floor(i / (SURFACE_SAMPLES / 4))];
                surface.nodes[i].vx += normal.x * amount * weight;
                surface.nodes[i].vy += normal.y * amount * weight;
            }
        }

        // The gel radius per arclength sample: gravity sag + the spring-node offsets (cubic Hermite
        // over the node arclengths) + ripple wave + strand bulge, softly floored, scaled by the open
        // ooze factor (the thin meniscus ring itself never disappears).
        function computeRadius(surface) {
            const profile = surface.profile, nodes = surface.nodes, off = surface.off, slope = surface.slope;
            for (let n = 0; n < SURFACE_SAMPLES; n++) {
                const normal = SURFACE_NORMALS[n >> 3];
                off[n] = nodes[n].dx * normal.x + nodes[n].dy * normal.y;
            }
            for (let n = 0; n < SURFACE_SAMPLES; n++) {
                slope[n] = (off[(n + 1) & 31] - off[(n + 31) & 31]) / profile.den[n];
            }
            const L = profile.length, pull = surface.pull, ooze = clamp(surface.grow, 0, 1.15);
            for (let k = 0; k < TABLE_SAMPLES; k++) {
                const a = profile.ia[k], b = profile.ib[k], gap = profile.gap[k];
                let r = profile.sag[k]
                    + profile.h00[k] * off[a] + profile.h10[k] * gap * slope[a]
                    + profile.h01[k] * off[b] + profile.h11[k] * gap * slope[b]
                    + surface.ripple[k];
                if (pull && pull.amp !== 0) {
                    const d = cyclicDelta((k + .5) / TABLE_SAMPLES * L, pull.s, L);
                    r += pull.amp * Math.exp(-d * d / (2 * pull.sigma * pull.sigma));
                }
                r = MIN_GEL_RADIUS + (softFloor(r) - MIN_GEL_RADIUS) * ooze;
                surface.radius[k] = clamp(r, MIN_GEL_RADIUS, MAX_GEL_RADIUS);
            }
        }

        // -----------------------------------------------------------------------------------------
        // Spring nodes (the jelly contour)

        function followSurface(id, rect, dt) {
            const surface = surfaceFor(id, rect);
            const baseTargets = surfaceTargets(rect);
            const drive = motionDrives.get(id);
            const nodes = surface.nodes;
            // Neighbour coupling reads last frame's offsets, not the ones being updated.
            const lastX = surface.lastX, lastY = surface.lastY;
            for (let i = 0; i < nodes.length; i++) { lastX[i] = nodes[i].dx; lastY[i] = nodes[i].dy; }
            const step = motion ? clamp(dt, 0, 1 / 30) : 0;
            const spring = 155;
            const tension = 22;
            const damping = Math.exp(-9.5 * step);
            const hover = surface.hover;
            const nodeS = surface.profile.nodeS, L = surface.layout.length;
            for (let i = 0; i < nodes.length; i++) {
                const node = nodes[i];
                const normal = SURFACE_NORMALS[i >> 3];
                const breathing = motion
                    ? Math.sin(state.clock * 1.65 + i * .71 + surface.seed) * 5.1 + Math.sin(state.clock * .83 - i * 1.11) * 1.5
                    : Math.sin(i * .71 + surface.seed) * 1.8;
                const along = (i % (SURFACE_SAMPLES / 4) + .5) / (SURFACE_SAMPLES / 4);
                const drag = drive ? -(drive.x * normal.x + drive.y * normal.y) * drive.strength * (.4 + .6 * Math.sin(Math.PI * along)) : 0;
                // The gel leans toward a slow pointer hovering near it (sticky, viscous).
                let attraction = 0;
                if (hover) {
                    const d = cyclicDelta(nodeS[i], hover.s, L);
                    attraction = hover.amp * Math.exp(-d * d / (2 * hover.sigma * hover.sigma));
                }
                const target = clamp(breathing + drag + attraction, -6, 16);
                const previous = (i + nodes.length - 1) % nodes.length, next = (i + 1) % nodes.length;
                const neighborX = (lastX[previous] + lastX[next]) * .5;
                const neighborY = (lastY[previous] + lastY[next]) * .5;
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
                const deformation = Math.sqrt(node.dx * node.dx + node.dy * node.dy);
                if (deformation > MAX_DEFORMATION) {
                    node.dx *= MAX_DEFORMATION / deformation;
                    node.dy *= MAX_DEFORMATION / deformation;
                    node.vx *= .5;
                    node.vy *= .5;
                }
                node.x = baseTargets[i].x + node.dx;
                node.y = baseTargets[i].y + node.dy;
                currentMaxDisplacement = Math.max(currentMaxDisplacement, Math.min(deformation, MAX_DEFORMATION));
            }
            if (drive) {
                drive.strength *= Math.exp(-4.5 * step);
                if (drive.strength < .1) motionDrives.delete(id);
            }
            maxDisplacement = Math.max(maxDisplacement, currentMaxDisplacement);
            return surface;
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

        // -----------------------------------------------------------------------------------------
        // Drips and drops

        function addDrop(x, y, size, vx, vy, life, extra) {
            if (drops.length >= MAX_DROPS) drops.shift();
            const drop = { x, y, size, vx, vy, age: 0, life,
                wobAmp: extra && extra.wobble != null ? extra.wobble : .26,
                wobFreq: 8 + rng() * 4, wobPhase: rng() * TWO_PI, wob: 0,
                origin: extra && extra.origin ? extra.origin : '' };
            drops.push(drop);
            counters.dropCount++;
            return drop;
        }

        function addSpray(x, y, size, vx, vy, life) {
            if (spray.length >= MAX_SPRAY) spray.shift();
            spray.push({ x, y, size, vx, vy, age: 0, life });
            counters.sprayCount++;
        }

        function dripGrowth(surface, drip) {
            const phase = motion ? ((state.clock + drip.offset) / drip.period) % 1 : .45;
            return phase < .84 ? phase / .84 : Math.max(0, (1 - phase) * 1.7);
        }

        function dripShape(surface, drip, growth) {
            const point = sampleContour(surface.nodes, drip.node + .5);
            const idle = motion ? Math.sin(state.clock * 1.8 + drip.offset) * 3 * growth + surface.nodes[drip.node].vx * .018 : 0;
            // The root sits just inside the outline of the radius table (the gel is thicker than the old
            // contour at the bottom), so a neck grows out of the gel itself instead of hiding in it.
            const hit = project(surface.layout, point.x, point.y, scratch);
            const lift = radiusAt(surface, hit.s) - 3 - hit.dist;
            return { x: point.x + hit.nx * lift, y: point.y + hit.ny * lift - 1,
                length: 6 + (32 + (drip.node % 3) * 8) * growth * growth,
                // The pendulum carries the drip against the window's motion (sloshing).
                sway: idle + drip.sw * (.25 + .75 * growth) };
        }

        function detachPeriodicDrop(id, surface) {
            if (!motion || surface.grow < .9) return;
            for (const drip of surface.drips) {
                const time = (state.clock + drip.offset) / drip.period;
                const cycle = Math.floor(time);
                const phase = time - cycle;
                if (phase < .84 || drip.cycle === cycle) continue;
                drip.cycle = cycle;
                const node = surface.nodes[drip.node];
                const shape = dripShape(surface, drip, 1);
                const side = SURFACE_NORMALS[Math.floor(drip.node / (SURFACE_SAMPLES / 4))];
                addDrop(shape.x + shape.sway, shape.y + shape.length, 6.5, node.vx * .18 + side.x * 36, 28, 2.8, { origin: id, wobble: .34 });
                // Recoil travels back into the membrane after the neck tears.
                node.vy -= 48;
            }
        }

        function stepDrops(dt) {
            const env = state.env, taskbar = env.taskbar;
            for (let i = drops.length - 1; i >= 0; i--) {
                const drop = drops[i];
                const previousY = drop.y;
                drop.age += dt;
                drop.vy += GRAVITY * dt;
                drop.x += drop.vx * dt;
                drop.y += drop.vy * dt;
                drop.wob = drop.wobAmp * Math.exp(-4.2 * drop.age) * Math.sin(TWO_PI * drop.wobFreq * drop.age + drop.wobPhase);
                const foot = drop.size * .8;
                if (taskbar && previousY + foot < taskbar.y && drop.y + foot >= taskbar.y
                    && drop.x >= taskbar.x + 4 && drop.x <= taskbar.x + taskbar.width - 4) {
                    landDrop(drop, taskbar.y);
                    drops.splice(i, 1);
                    continue;
                }
                if (absorbDrop(drop)) { drops.splice(i, 1); continue; }
                if (drop.age >= drop.life || drop.y > env.height + 40) drops.splice(i, 1);
            }
        }

        // A falling drop that reaches another window's visible gel merges into it: the surface
        // bulges for a moment and a ring travels around the frame.
        function absorbDrop(drop) {
            const windows = state.env.windows;
            for (let i = windows.length - 1; i >= 0; i--) {
                const surface = surfaces.get(String(windows[i].id));
                if (!surface || !surface.layout) continue;
                if (surface.id === drop.origin && drop.age < .6) continue;
                const lay = surface.layout;
                if (drop.x < lay.x - 50 || drop.x > lay.x + lay.w + 50 || drop.y < lay.y - 50 || drop.y > lay.y + lay.h + 50) continue;
                const hit = project(lay, drop.x, drop.y, scratch);
                if (hit.dist <= 0 || hit.dist >= radiusAt(surface, hit.s) - 1.5) continue;
                if (blockedAt(surface, drop.x, drop.y)) continue;
                rippleImpulse(surface, hit.s, clamp(.9 + drop.size * .22, .9, 3.4), 18 + drop.size * 2);
                nudgeNodes(surface, hit.s, 14 + drop.size * 3, 30);
                counters.absorbedDrops++;
                return true;
            }
            return false;
        }

        function landDrop(drop, edgeY) {
            counters.splatCount++;
            for (const splat of splats) {
                if (Math.abs(splat.x - drop.x) < (splat.r + drop.size) * 1.2 && splat.age < splat.life - 1) {
                    // Two drops that touch run into one blob.
                    splat.r = Math.min(14, Math.sqrt(splat.r * splat.r + drop.size * drop.size * .8));
                    splat.x = lerp(splat.x, drop.x, .3);
                    splat.age = 0;
                    splat.life = jitter(2.4, 3.8);
                    return;
                }
            }
            if (splats.length >= MAX_SPLATS) splats.shift();
            splats.push({ x: drop.x, y: edgeY, r: clamp(drop.size * .95, 2.5, 11), age: 0, life: jitter(2.2, 4) });
        }

        function stepSplats(dt) {
            for (let i = splats.length - 1; i >= 0; i--) {
                const splat = splats[i];
                splat.age += dt;
                if (state.env.taskbar) splat.y = state.env.taskbar.y;
                if (splat.age >= splat.life) splats.splice(i, 1);
            }
        }

        function stepSpray(dt) {
            for (let i = spray.length - 1; i >= 0; i--) {
                const drop = spray[i];
                drop.age += dt;
                drop.vy += SPRAY_GRAVITY * dt;
                drop.vx *= Math.exp(-.8 * dt);
                drop.x += drop.vx * dt;
                drop.y += drop.vy * dt;
                if (drop.age >= drop.life || drop.y > state.env.height + 40) spray.splice(i, 1);
            }
        }

        // -----------------------------------------------------------------------------------------
        // Window motion: inertia, sloshing drips, hard stops

        function setWindowVelocity(surface, vx, vy, now) {
            const dvx = vx - surface.winV.x;
            const previousSpeed = Math.hypot(surface.winV.x, surface.winV.y);
            const speed = Math.hypot(vx, vy);
            // The root of a pendulum accelerates: its bob changes velocity against the window's.
            for (const drip of surface.drips) drip.swv -= dvx * .12;
            const hardStop = motion && previousSpeed >= 520 && speed < previousSpeed * .4 && now - surface.flingAt > 1.2;
            const dirX = surface.winV.x / (previousSpeed || 1), dirY = surface.winV.y / (previousSpeed || 1);
            surface.winV.x = vx;
            surface.winV.y = vy;
            if (hardStop) {
                surface.flingAt = now;
                fling(surface, dirX, dirY, previousSpeed);
            }
        }

        // A hard stop throws ripe drips and a few droplets off the leading edge.
        function fling(surface, dirX, dirY, speed) {
            if (!surface.layout) return;
            for (const drip of surface.drips) {
                const growth = dripGrowth(surface, drip);
                const time = (state.clock + drip.offset) / drip.period;
                if (growth < .55 || time - Math.floor(time) >= .84) continue;
                const node = surface.nodes[drip.node];
                const shape = dripShape(surface, drip, growth);
                addDrop(shape.x + shape.sway, shape.y + shape.length, 3 + growth * 3, dirX * speed * .2, 20 + dirY * speed * .1, 2.4, { origin: surface.id, wobble: .42 });
                // Restart the drip small.
                drip.offset += (1 - (time - Math.floor(time))) * drip.period;
                node.vy -= 30;
            }
            let lead = 0, leadDot = -2;
            const L = surface.layout.length;
            for (let k = 0; k < TABLE_SAMPLES; k++) {
                const p = pointAt(surface.layout, (k + .5) / TABLE_SAMPLES * L, scratch);
                const dot = p.nx * dirX + p.ny * dirY;
                if (dot > leadDot) { leadDot = dot; lead = (k + .5) / TABLE_SAMPLES * L; }
            }
            const count = 2 + Math.floor(rng() * 3);
            for (let i = 0; i < count; i++) {
                const s = lead + jitter(-40, 40);
                const p = pointAt(surface.layout, s, scratch);
                const radius = radiusAt(surface, s);
                addDrop(p.x + p.nx * radius, p.y + p.ny * radius, jitter(2, 3.8),
                    dirX * speed * .22 + p.nx * 30 + jitter(-25, 25), dirY * speed * .22 + p.ny * 30 - 25 + jitter(-20, 20), 2.2, { origin: surface.id, wobble: .45 });
            }
            rippleImpulse(surface, lead, 2.8, 38);
            const extra = 1 + Math.floor(rng() * 2);
            for (let i = 0; i < extra; i++) bubbles.spawn({ windowId: surface.id, fast: true });
        }

        function windowEvent(event, now) {
            if (!event || event.id == null) return;
            const id = String(event.id);
            const rect = event.rect;
            if (event.phase === 'close') {
                if (rect && motion && !event.silent) {
                    for (let i = 0; i < 5; i++) {
                        const corner = i % 2 ? rect.x + rect.width : rect.x;
                        addDrop(corner + (rng() - .5) * 16, rect.y + rect.height + 3,
                            5 + rng() * 8, (rng() - .5) * 46, 30 + rng() * 65, 2.4 + rng() * .6, { wobble: .4 });
                    }
                }
                removeWindow(id);
                return;
            }
            let surface = surfaces.get(id);
            if (event.phase === 'open' && motion && !event.silent && rect) {
                // The gel oozes out of the new window.
                surface = surfaceFor(id, rect);
                surface.grow = 0;
                surface.growV = 0;
            }
            if (event.phase === 'drag' && event.velocity) {
                const vx = Number(event.velocity.x) || 0;
                const vy = Number(event.velocity.y) || 0;
                const speed = Math.hypot(vx, vy);
                if (speed > 8) motionDrives.set(id, { x: vx / speed, y: vy / speed, strength: Math.min(13, speed * .014) });
                if (surface) { setWindowVelocity(surface, vx, vy, now); surface.dragAt = now; }
            } else if (event.phase !== 'drag-end') motionDrives.delete(id);
            if (event.phase === 'drag-end' && surface) {
                // A release right after a quick drag stops the window dead; a long pause already did.
                if (now - surface.dragAt > .14) { surface.winV.x = surface.winV.y = 0; }
                else setWindowVelocity(surface, 0, 0, now);
            }
        }

        // Windows the bridge no longer knows about disappear; drag drives are reset like a fresh snapshot.
        function syncWindows(ids) {
            for (const id of [...surfaces.keys()]) if (!ids.has(id)) removeWindow(id);
            motionDrives.clear();
        }

        // -----------------------------------------------------------------------------------------
        // Per-frame update

        function runPending(surface) {
            for (let i = surface.pending.length - 1; i >= 0; i--) {
                const item = surface.pending[i];
                if (state.clock < item.at) continue;
                surface.pending.splice(i, 1);
                if (item.kind === 'ripple') rippleImpulse(surface, item.s, item.amplitude, item.width);
                else if (item.kind === 'bubble') bubbles.spawn({ windowId: surface.id, at: { x: item.x, y: item.y }, fast: true });
            }
        }

        function stepGrow(surface, dt) {
            if (surface.grow === 1 && surface.growV === 0) return;
            const w = 13, z = .6, h = dt / 2;
            for (let n = 0; n < 2; n++) {
                surface.growV += (w * w * (1 - surface.grow) - 2 * z * w * surface.growV) * h;
                surface.grow += surface.growV * h;
            }
            if (Math.abs(1 - surface.grow) < .002 && Math.abs(surface.growV) < .02) { surface.grow = 1; surface.growV = 0; }
            surface.grow = clamp(surface.grow, 0, 1.15);
        }

        function stepPendulums(surface, dt) {
            const w = 9.2, z = .2;
            for (const drip of surface.drips) {
                drip.swv += (-w * w * drip.sw - 2 * z * w * drip.swv) * dt;
                drip.sw = clamp(drip.sw + drip.swv * dt, -16, 16);
            }
        }

        function stepSurface(surface, dt, now) {
            if (!motion) return;
            surface.age += dt;
            // No events during a drag for a while: the window stands still.
            if ((surface.winV.x || surface.winV.y) && now - surface.dragAt > .14) setWindowVelocity(surface, 0, 0, now);
            stepGrow(surface, dt);
            rippleStep(surface, dt);
            stepPendulums(surface, dt);
            touch.relax(surface, dt);
            runPending(surface);
            detachPeriodicDrop(surface.id, surface);
            bubbles.step(surface, dt);
        }

        function update(next) {
            const env = state.env = next || EMPTY_ENV;
            const dt = clamp(env.dt || 0, 0, .05);
            const now = env.now || 0;
            currentMaxDisplacement = 0;
            if (motion) state.clock += dt;
            const seen = new Set();
            for (const win of env.windows) {
                const id = String(win.id);
                win.rect.radii = win.radii;
                const surface = surfaceFor(id, win.rect);
                surface.z = win.z;
                surface.active = !!win.active;
                seen.add(id);
            }
            for (const id of [...surfaces.keys()]) if (!seen.has(id)) removeWindow(id);
            touch.update(dt, now);
            for (const win of env.windows) {
                const surface = followSurface(String(win.id), win.rect, dt);
                stepSurface(surface, dt, now);
            }
            if (motion) {
                touch.step(dt);
                bubbles.spawnAmbient(dt);
                stepDrops(dt);
                stepSpray(dt);
                stepSplats(dt);
            }
            for (const surface of surfaces.values()) computeRadius(surface);
        }

        // -----------------------------------------------------------------------------------------
        // Scene

        function sceneWindow(surface, win) {
            const drips = [];
            const ooze = clamp(surface.grow, 0, 1);
            for (const drip of surface.drips) {
                // A freshly opened window has not built up its drips yet.
                const growth = dripGrowth(surface, drip) * ooze;
                const shape = dripShape(surface, drip, growth);
                const root = 9 - growth * 3;
                drips.push({ x: shape.x, y: shape.y, length: shape.length, root, neck: Math.max(.7, root * (1 - growth * .9)),
                    bulb: 4 + growth * 4, sway: shape.sway });
            }
            const shown = surface.bubbles.map(bubble => bubbles.view(surface, bubble));
            if (!motion) {
                const frozen = bubbles.frozen(surface);
                if (frozen) shown.push(frozen);
            }
            const thread = touch.view(surface);
            const rect = surface.layout;
            return { id: surface.id, rect: { x: rect.x, y: rect.y, width: rect.w, height: rect.h }, radii: rect.radii,
                z: win.z, active: !!win.active, grow: Math.min(1, surface.grow), radius: surface.radius,
                drips, bubbles: shown, strands: thread ? [thread] : [], occluders: [], under: [] };
        }

        function scene(next) {
            const e = next || state.env;
            const out = { time: state.clock, width: e.width, height: e.height, dpr: e.dpr, motion, backdrop: null,
                windows: [], drops: [], splats: [], dropOccluders: [] };
            for (const win of e.windows) {
                const surface = surfaces.get(String(win.id));
                if (surface && surface.layout) out.windows.push(sceneWindow(surface, win));
            }
            for (const drop of drops) {
                out.drops.push({ x: drop.x, y: drop.y, r: drop.size, vx: drop.vx, vy: drop.vy, wobble: drop.wob,
                    alpha: clamp((drop.life - drop.age) / .25, 0, 1) });
            }
            for (const drop of spray) {
                out.drops.push({ x: drop.x, y: drop.y, r: drop.size, vx: drop.vx, vy: drop.vy, wobble: 0,
                    alpha: clamp((drop.life - drop.age) / .15, 0, 1), spray: true });
            }
            for (const splat of splats) {
                const age = splat.age;
                out.splats.push({ x: splat.x, y: splat.y, r: splat.r * (1 - .28 * age / splat.life),
                    spread: 1 - Math.exp(-11 * age) * Math.cos(24 * age),
                    alpha: age > splat.life - 1.2 ? clamp((splat.life - age) / 1.2, 0, 1) : 1 });
            }
            return out;
        }

        function stats() {
            const shown = bubbles.report();
            let energy = 0;
            for (const surface of surfaces.values()) energy += surface.energy;
            const threads = touch.sample();
            return {
                dropCount: counters.dropCount,
                activeDrops: drops.length,
                sprayCount: counters.sprayCount,
                activeSpray: spray.length,
                bubbleCount: counters.bubbleCount,
                activeBubbles: shown.active,
                bubbleBursts: counters.bubbleBursts,
                bubbleStages: shown.stages,
                bubbleStageEntries: { ...stageEntries },
                bubbleSamples: shown.samples,
                strandCount: counters.strandCount,
                activeStrands: threads.length,
                strandSamples: threads,
                strandSnaps: counters.strandSnaps,
                absorbedDrops: counters.absorbedDrops,
                splatCount: counters.splatCount,
                activeSplats: splats.length,
                splatSamples: splats.map(splat => ({ x: splat.x, y: splat.y, r: splat.r, age: splat.age })),
                rippleEnergy: Math.round(energy * 1000) / 1000,
                pokeCount: counters.pokeCount,
                maxDisplacement: Math.min(MAX_DEFORMATION, maxDisplacement),
                currentMaxDisplacement: Math.min(MAX_DEFORMATION, currentMaxDisplacement),
                surfaceCount: surfaces.size,
                surfaceNodeCount: surfaces.size * SURFACE_SAMPLES
            };
        }

        function reset() {
            surfaces.clear();
            motionDrives.clear();
            drops.length = 0;
            spray.length = 0;
            splats.length = 0;
            touch.reset();
            state.env = EMPTY_ENV;
        }

        return {
            motion, surfaces, motionDrives,
            update, scene, stats, reset,
            surfaceTargets, followSurface, detachPeriodicDrop, addDrop, sampleContour, dripShape,
            rippleImpulse, rippleStep, radiusAt, windowEvent, syncWindows, removeWindow,
            spawnBubble: bubbles.spawn,
            poke: touch.poke, pointerMove: touch.pointerMove, pointerDown: touch.pointerDown, pointerUp: touch.pointerUp, hitGel: touch.hitGel,
            get clock() { return state.clock; },
            get strand() { return touch.strand; },
            get drops() { return drops; },
            get spray() { return spray; },
            get splats() { return splats; }
        };
    }

    window.AuraGreenSlimeSim = Object.freeze({
        createSim, geometry, strandRadii: kit.strandRadii,
        constants: kit.constants
    });
})();
