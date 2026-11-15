/* Leuchtfeuer simulation. No DOM, no Three.js, no fetch. */
(function (root) {
    const STEP = 1 / 60;
    const SPEED = 3.4;
    const PARTS = ['glass', 'wick', 'oil'];

    function cameraOffset(yaw, distance) {
        return { x: Math.sin(yaw) * distance, z: -Math.cos(yaw) * distance };
    }

    /* Parent -Z points along (dx, dz). The glTF +Z front is mounted onto that axis.
       Negating 0 yields -0, and atan2(-0, -1) is -π; coerce so +Z is +π. */
    function yawFacing(dx, dz) {
        const x = Object.is(-dx, -0) ? 0 : -dx;
        const z = Object.is(-dz, -0) ? 0 : -dz;
        return Math.atan2(x, z);
    }

    /* fwd is the camera's flattened world direction. right = forward × up. */
    function wishFromCamera(fwdX, fwdZ, strafe, forward) {
        const len = Math.hypot(fwdX, fwdZ);
        if (!(len > 0)) return { x: 0, z: 0 };
        const fx = fwdX / len;
        const fz = fwdZ / len;
        const rx = -fz;
        const rz = fx;
        let x = fx * forward + rx * strafe;
        let z = fz * forward + rz * strafe;
        const mag = Math.hypot(x, z);
        if (mag > 1) { x /= mag; z /= mag; }
        return { x: x, z: z };
    }

    function contains(surface, x, z) {
        return x >= surface.minX && x <= surface.maxX && z >= surface.minZ && z <= surface.maxZ;
    }

    function surfaceAt(world, x, z, currentY) {
        let best = null;
        let bestGap = Infinity;
        world.surfaces.forEach(function (surface) {
            if (!contains(surface, x, z)) return;
            const gap = Math.abs(surface.y - currentY);
            if (gap < bestGap) { best = surface; bestGap = gap; }
        });
        return best;
    }

    function blocked(world, x, z, y) {
        const head = y + 1.7;
        for (let i = 0; i < world.blockers.length; i++) {
            const box = world.blockers[i];
            if (head < box.minY || y > box.maxY) continue;
            const cx = Math.max(box.minX, Math.min(x, box.maxX));
            const cz = Math.max(box.minZ, Math.min(z, box.maxZ));
            const dx = x - cx;
            const dz = z - cz;
            if (dx * dx + dz * dz < world.radius * world.radius) return true;
        }
        return false;
    }

    function patrolPeriod(guard) {
        return (2 * Math.abs(guard.to - guard.from)) / guard.speed;
    }

    function patrolPose(guard, time) {
        const period = patrolPeriod(guard);
        let u = ((time + guard.phase * period) % period) / period;
        if (u < 0) u += 1;
        const along = u < 0.5 ? u * 2 : (1 - u) * 2;
        return { x: guard.x, y: guard.y, z: guard.from + (guard.to - guard.from) * along };
    }

    function fresh(world) {
        const spawn = world.spawn;
        return {
            world: world,
            x: spawn.x,
            y: spawn.y,
            z: spawn.z,
            yaw: spawn.yaw,
            parts: { glass: false, wick: false, oil: false },
            lit: false,
            time: 0,
            acc: 0,
            target: null,
            moving: false,
            surface: 'beach',
            notice: '',
            missing: PARTS.slice(),
            splashed: false,
            bumped: false,
            picked: '',
            won: false
        };
    }

    function createSim(world) {
        return fresh(world);
    }

    function placeSpawn(sim) {
        const spawn = sim.world.spawn;
        sim.x = spawn.x;
        sim.y = spawn.y;
        sim.z = spawn.z;
        sim.yaw = spawn.yaw;
        sim.target = null;
        sim.moving = false;
        sim.surface = 'beach';
    }

    function nearest(sim) {
        let best = null;
        let bestD = sim.world.reach;
        sim.world.spots.forEach(function (spot) {
            const d = Math.hypot(sim.x - spot.x, sim.z - spot.z);
            if (d <= bestD) { best = spot; bestD = d; }
        });
        return best;
    }

    function missingParts(sim) {
        return PARTS.filter(function (part) { return !sim.parts[part]; });
    }

    function interact(sim) {
        const spot = nearest(sim);
        if (!spot) {
            sim.notice = 'leuchtfeuer.notice.far';
            return sim;
        }
        if (spot.part === 'lamp') {
            sim.missing = missingParts(sim);
            if (sim.missing.length) {
                sim.notice = 'leuchtfeuer.notice.missing';
                return sim;
            }
            if (!sim.lit) {
                sim.lit = true;
                sim.won = true;
                sim.notice = 'leuchtfeuer.notice.lit';
            }
            return sim;
        }
        if (!sim.parts[spot.part]) {
            sim.parts[spot.part] = true;
            sim.picked = spot.part;
            sim.notice = 'leuchtfeuer.notice.' + spot.part;
        }
        sim.missing = missingParts(sim);
        return sim;
    }

    function tick(sim, input, dt) {
        const world = sim.world;
        sim.time += dt;
        let dirX = 0;
        let dirZ = 0;
        const wish = input.wish || { x: 0, z: 0 };
        if (wish.x || wish.z) {
            sim.target = null;
            dirX = wish.x;
            dirZ = wish.z;
        } else if (sim.target) {
            const tx = sim.target.x - sim.x;
            const tz = sim.target.z - sim.z;
            const dist = Math.hypot(tx, tz);
            if (dist < 0.25) sim.target = null;
            else { dirX = tx / dist; dirZ = tz / dist; }
        }
        sim.moving = dirX !== 0 || dirZ !== 0;
        if (sim.moving) {
            sim.yaw = yawFacing(dirX, dirZ);
            const stepX = dirX * SPEED * dt;
            const stepZ = dirZ * SPEED * dt;
            let nx = sim.x + stepX;
            let nz = sim.z;
            if (blocked(world, nx, nz, sim.y)) nx = sim.x;
            nz = sim.z + stepZ;
            if (blocked(world, nx, nz, sim.y)) nz = sim.z;
            const ground = surfaceAt(world, nx, nz, sim.y);
            if (!ground) {
                sim.splashed = true;
                sim.notice = 'leuchtfeuer.notice.water';
                placeSpawn(sim);
            } else {
                sim.x = nx;
                sim.z = nz;
                sim.y = ground.y;
                sim.surface = ground.id;
            }
        }
        for (let i = 0; i < world.guards.length; i++) {
            const pose = patrolPose(world.guards[i], sim.time);
            if (Math.hypot(sim.x - pose.x, sim.z - pose.z) < world.guardReach) {
                sim.bumped = true;
                sim.notice = 'leuchtfeuer.notice.guard';
                placeSpawn(sim);
                break;
            }
        }
        if (input.interact) interact(sim);
    }

    function step(sim, input, dt) {
        sim.splashed = false;
        sim.bumped = false;
        sim.picked = '';
        sim.won = false;
        if (!(dt > 0)) return sim;
        sim.acc += Math.min(dt, STEP * 3);
        let count = 0;
        while (sim.acc >= STEP && count < 3) {
            tick(sim, input || {}, STEP);
            sim.acc -= STEP;
            count += 1;
        }
        if (count === 3) sim.acc = 0;
        return sim;
    }

    function onSurface(world, x, y, z) {
        return world.surfaces.some(function (surface) {
            return contains(surface, x, z) && Math.abs(surface.y - y) < 0.02;
        });
    }

    function exportState(sim) {
        return {
            v: 1,
            x: sim.x, y: sim.y, z: sim.z, yaw: sim.yaw,
            parts: { glass: !!sim.parts.glass, wick: !!sim.parts.wick, oil: !!sim.parts.oil },
            lit: !!sim.lit,
            time: sim.time
        };
    }

    function importState(sim, raw) {
        const world = sim.world;
        const ok = raw && raw.v === 1 && raw.parts && typeof raw.parts === 'object' && !Array.isArray(raw.parts);
        const keys = ok ? Object.keys(raw.parts) : [];
        const known = ok && keys.every(function (key) { return PARTS.indexOf(key) >= 0; });
        const nums = ok && [raw.x, raw.y, raw.z, raw.yaw, raw.time].every(function (value) { return typeof value === 'number' && Number.isFinite(value); });
        if (!ok || !known || !nums || typeof raw.lit !== 'boolean' || !onSurface(world, raw.x, raw.y, raw.z)) {
            const reset = fresh(world);
            Object.assign(sim, reset);
            return sim;
        }
        sim.x = raw.x;
        sim.y = raw.y;
        sim.z = raw.z;
        sim.yaw = raw.yaw;
        sim.time = raw.time;
        sim.lit = raw.lit;
        sim.parts = { glass: !!raw.parts.glass, wick: !!raw.parts.wick, oil: !!raw.parts.oil };
        sim.missing = missingParts(sim);
        sim.target = null;
        sim.acc = 0;
        const ground = surfaceAt(world, sim.x, sim.z, sim.y);
        sim.surface = ground ? ground.id : 'beach';
        return sim;
    }

    root.LeuchtfeuerSim = {
        SPEED: SPEED,
        cameraOffset: cameraOffset,
        yawFacing: yawFacing,
        wishFromCamera: wishFromCamera,
        patrolPose: patrolPose,
        patrolPeriod: patrolPeriod,
        createSim: createSim,
        step: step,
        interact: interact,
        exportState: exportState,
        importState: importState,
        nearest: nearest
    };
})(typeof window !== 'undefined' ? window : globalThis);
