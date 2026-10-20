(function () {
    'use strict';

    // Pointer interaction of the Green Slime gel, a part of the simulation (see green_slime_sim.js):
    // hover attraction (sticky lean toward a slow pointer), wake (a fast pointer dents the gel), poke
    // (a press on the visible gel) and the pull-able strand with its snap and recoil. DOM-free: the
    // bridge passes coordinates and timestamps in seconds.

    const kit = window.AuraGreenSlimeGeometry;
    const { clamp, lerp, smoothstep, cyclicDelta } = kit;
    const { STRAND_SNAP, STRAND_HOLD, STRAND_OMEGA } = kit.constants;
    const { pointAt, project } = kit.geometry;

    function create(core) {
        const { motion, jitter, rng, surfaces, state, counters } = core;
        const pointer = { x: -10000, y: -10000, speed: 0, at: -1, down: false, seen: false };
        const scratch = {};
        const scratchB = {};
        let strand = null;

        function pointerMove(x, y, now, buttons) {
            if (!motion) return;
            const px = pointer.x, py = pointer.y;
            const elapsed = now - pointer.at;
            if (pointer.seen && elapsed > 0 && elapsed < .25) {
                const instant = Math.hypot(x - px, y - py) / Math.max(.004, elapsed);
                pointer.speed = lerp(pointer.speed, instant, .45);
            } else pointer.speed = 0;
            const previousSeen = pointer.seen;
            pointer.x = x; pointer.y = y; pointer.at = now; pointer.seen = true;
            if (strand && buttons != null && !(buttons & 1)) pointerUp();
            if (previousSeen && pointer.speed > 320) wake(px, py, x, y, now);
        }

        // A fast pointer crossing or ploughing through the gel dents it where it passes.
        function wake(px, py, x, y, now) {
            for (const win of state.env.windows) {
                const surface = surfaces.get(String(win.id));
                if (!surface || !surface.layout || now - surface.wakeAt < .07 || surface.grow < .6) continue;
                const a = project(surface.layout, px, py, scratch);
                const b = project(surface.layout, x, y, scratchB);
                if (a.dist <= -2 || b.dist <= -2) continue;
                const insideA = a.dist < core.radiusAt(surface, a.s), insideB = b.dist < core.radiusAt(surface, b.s);
                if (!insideA && !insideB) continue;
                if (core.blockedAt(surface, x, y)) continue;
                surface.wakeAt = now;
                const strength = clamp(pointer.speed * .0045, .8, 4.2);
                const entering = insideB && !insideA;
                const leaving = insideA && !insideB;
                core.rippleImpulse(surface, b.s, (leaving ? .45 : -1) * strength, 22 + pointer.speed * .02);
                if (entering || (insideB && insideA && pointer.speed > 500)) core.nudgeNodes(surface, b.s, -pointer.speed * .04, 40);
            }
        }

        function hitGel(x, y) {
            const windows = state.env.windows;
            for (let i = windows.length - 1; i >= 0; i--) {
                const surface = surfaces.get(String(windows[i].id));
                if (!surface || !surface.layout) continue;
                const hit = project(surface.layout, x, y, {});
                const radius = core.radiusAt(surface, hit.s);
                if (hit.dist <= 0 || hit.dist > radius || core.blockedAt(surface, x, y)) continue;
                return { id: surface.id, s: hit.s, dist: hit.dist, radius, nx: hit.nx, ny: hit.ny };
            }
            return null;
        }

        // A press on the visible gel: dent, ring ripple, maybe a small bubble a moment later.
        function poke(x, y) {
            if (!motion) return null;
            const hit = hitGel(x, y);
            if (!hit) return null;
            const surface = surfaces.get(hit.id);
            counters.pokeCount++;
            core.rippleImpulse(surface, hit.s, -2.6, 16);
            core.nudgeNodes(surface, hit.s, -36, 28);
            if (rng() < .6) surface.pending.push({ at: state.clock + .3, kind: 'bubble', x, y });
            return hit;
        }

        function pointerDown(x, y, now) {
            if (!motion) return null;
            pointer.x = x; pointer.y = y; pointer.at = now; pointer.seen = true; pointer.down = true;
            const hit = poke(x, y);
            if (!hit) return null;
            const surface = surfaces.get(hit.id);
            const anchor = pointAt(surface.layout, hit.s, scratch);
            const depth = clamp(hit.dist, 3, Math.max(3, hit.radius - 2));
            strand = { id: hit.id, s: hit.s, depth, state: 'pressed',
                tx: x, ty: y, tvx: 0, tvy: 0, born: state.clock, pulledAt: 0, threshold: Math.max(18, depth + 12),
                ax: anchor.x + anchor.nx * depth, ay: anchor.y + anchor.ny * depth, length: 0, recoil: 0, stub: 0, dirX: anchor.nx, dirY: anchor.ny };
            return { id: hit.id, s: hit.s };
        }

        function pointerUp() {
            pointer.down = false;
            if (!strand) return;
            const surface = surfaces.get(strand.id);
            if (strand.state === 'pulled' && surface) snap(surface);
            else if (strand.state !== 'recoil') strand = null;
        }

        // Hover attraction: the gel leans toward a slow pointer near it, smoothed so it feels viscous.
        function update(dt, now) {
            if (!motion) return;
            if (pointer.seen && now - pointer.at > .08) pointer.speed *= Math.exp(-dt / .12);
            for (const win of state.env.windows) {
                const surface = surfaces.get(String(win.id));
                if (!surface || !surface.layout) continue;
                let target = 0, hoverS = surface.hoverS;
                if (pointer.seen && !(strand && strand.id === surface.id) && now - pointer.at < 6) {
                    const hit = project(surface.layout, pointer.x, pointer.y, scratch);
                    const gap = hit.dist - core.radiusAt(surface, hit.s);
                    if (hit.dist > 0 && gap < 60 && !core.blockedAt(surface, pointer.x, pointer.y)) {
                        target = 10 * (1 - smoothstep(0, 60, gap)) * (1 - smoothstep(140, 480, pointer.speed));
                        hoverS = hit.s;
                    }
                }
                surface.hoverAmp += (target - surface.hoverAmp) * (1 - Math.exp(-dt / (target > surface.hoverAmp ? .22 : .35)));
                if (surface.hoverAmp < .05) surface.hoverS = hoverS;
                else if (target > 0) surface.hoverS += cyclicDelta(hoverS, surface.hoverS, surface.layout.length) * (1 - Math.exp(-dt / .12));
                surface.hover = surface.hoverAmp > .02 ? { s: surface.hoverS, amp: surface.hoverAmp, sigma: 52 } : null;
            }
        }

        function snap(surface) {
            const s = strand;
            const length = s.length;
            const radii = kit.strandRadii(length);
            const dirX = (s.tx - s.ax) / (length || 1), dirY = (s.ty - s.ay) / (length || 1);
            // The thread parts near its thin end: the tip becomes a drop, the middle sheds satellites.
            core.addDrop(s.tx, s.ty, Math.max(2.6, radii.tip * 1.5), clamp(s.tvx, -900, 900), clamp(s.tvy, -900, 900), 2.8, { origin: surface.id, wobble: .5 });
            const satellites = 1 + Math.floor(rng() * 2);
            for (let i = 0; i < satellites; i++) {
                const u = .45 + .3 * rng();
                core.addDrop(lerp(s.tx, s.ax, u), lerp(s.ty, s.ay, u), jitter(1.2, 2.2), s.tvx * .6 + jitter(-30, 30), s.tvy * .6 + jitter(-30, 10) - 20, 2, { origin: surface.id, wobble: .5 });
            }
            counters.strandSnaps++;
            // The base recoils with overshoot and the stub retracts into the gel.
            const pull = surface.pull || (surface.pull = { s: s.s, amp: 0, v: 0, sigma: 26 });
            pull.v = -Math.max(pull.amp, 2) * 18;
            core.rippleImpulse(surface, s.s, 1.8, 20);
            core.nudgeNodes(surface, s.s, 40, 40);
            s.state = 'recoil';
            s.recoil = 0;
            s.stub = Math.max(14, length * .32);
            s.dirX = dirX; s.dirY = dirY;
            s.tx = s.ax + dirX * s.stub; s.ty = s.ay + dirY * s.stub;
        }

        function step(dt) {
            if (!strand) return;
            const surface = surfaces.get(strand.id);
            if (!surface || !surface.layout) { strand = null; return; }
            const anchor = pointAt(surface.layout, strand.s, scratch);
            strand.ax = anchor.x + anchor.nx * strand.depth;
            strand.ay = anchor.y + anchor.ny * strand.depth;
            if (strand.state === 'recoil') {
                strand.recoil += dt;
                const k = Math.max(0, 1 - strand.recoil / .25);
                const stub = strand.stub * k * k;
                strand.length = stub;
                strand.tx = strand.ax + strand.dirX * stub;
                strand.ty = strand.ay + strand.dirY * stub;
                if (strand.recoil >= .25) strand = null;
                return;
            }
            // The tip follows the pointer with a critically damped lag (exact solution of the step).
            const w = STRAND_OMEGA, e = Math.exp(-w * dt);
            const dx = strand.tx - pointer.x, dy = strand.ty - pointer.y;
            const nvx = (strand.tvx - w * (strand.tvx + w * dx) * dt) * e;
            const nvy = (strand.tvy - w * (strand.tvy + w * dy) * dt) * e;
            strand.tx = pointer.x + (dx + (strand.tvx + w * dx) * dt) * e;
            strand.ty = pointer.y + (dy + (strand.tvy + w * dy) * dt) * e;
            strand.tvx = nvx; strand.tvy = nvy;
            strand.length = Math.hypot(strand.tx - strand.ax, strand.ty - strand.ay);
            if (strand.state === 'pressed' && strand.length > strand.threshold) {
                strand.state = 'pulled';
                strand.pulledAt = state.clock;
                counters.strandCount++;
            }
            if (strand.state !== 'pulled') return;
            // The gel stretches toward the tip while the thread is drawn out.
            const pull = surface.pull || (surface.pull = { s: strand.s, amp: 0, v: 0, sigma: 26 });
            const wanted = Math.min(7, strand.length * .055);
            const before = pull.amp;
            pull.s = strand.s;
            pull.amp += (wanted - pull.amp) * (1 - Math.exp(-dt / .06));
            pull.v = dt > 0 ? (pull.amp - before) / dt : 0;
            if (strand.length > STRAND_SNAP || state.clock - strand.pulledAt > STRAND_HOLD || !pointer.down) snap(surface);
        }

        // After a snap (or without a strand) the bulge at the anchor oscillates back to rest.
        function relax(surface, dt) {
            const pull = surface.pull;
            if (!pull || (strand && strand.id === surface.id && strand.state !== 'recoil')) return;
            const w = 24, z = .3, steps = dt > .025 ? 2 : 1, h = dt / steps;
            for (let n = 0; n < steps; n++) {
                pull.v += (-w * w * pull.amp - 2 * z * w * pull.v) * h;
                pull.amp += pull.v * h;
            }
            if (Math.abs(pull.amp) < .02 && Math.abs(pull.v) < .2) surface.pull = null;
        }

        function view(surface) {
            if (!strand || strand.id !== surface.id || strand.state === 'pressed') return null;
            const radii = kit.strandRadii(Math.max(strand.length, strand.state === 'recoil' ? 14 : 0));
            return { ax: strand.ax, ay: strand.ay, bx: strand.tx, by: strand.ty,
                root: radii.root, mid: radii.mid, tip: radii.tip, beads: strand.state === 'recoil' ? 0 : radii.beads };
        }

        function clear(id) { if (strand && strand.id === id) strand = null; }
        function rescale(id, ratio) { if (strand && strand.id === id) strand.s *= ratio; }
        function sample() {
            return strand && strand.state !== 'pressed'
                ? [{ window: strand.id, state: strand.state, length: strand.length, ax: strand.ax, ay: strand.ay, bx: strand.tx, by: strand.ty }] : [];
        }
        function reset() { strand = null; }

        return { pointerMove, pointerDown, pointerUp, poke, hitGel, update, step, relax, view, clear, rescale, sample, reset,
            get strand() { return strand; } };
    }

    window.AuraGreenSlimeTouch = Object.freeze({ create });
})();
