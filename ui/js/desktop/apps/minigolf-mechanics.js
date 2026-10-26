// Pure AuraGolf mechanics: moving-obstacle poses, mechanic zones and their validation.
// No imports, no Rapier: the live step and the preview ghost drive the same
// functions through a small ball adapter (see `stepZones`).
export const OBSTACLE_KINDS = Object.freeze(['rotate', 'slide', 'spin', 'gate', 'boulder']);
export const MECHANIC_KINDS = Object.freeze(['bumper', 'boost', 'teleport', 'cannon']);
// Only these end a shot; impact, bumper, boost, teleport and cannon just report.
export const TERMINAL_EVENTS = Object.freeze(new Set(['rest', 'cup', 'water', 'out']));
export const TELEPORT_LOCKOUT = 0.5;
const ID = /^[a-z][a-z0-9-]{0,31}$/;
const WALL_SURFACES = ['wood', 'stone', 'metal'];

const normalize = vector => { const length = Math.hypot(...vector) || 1; return vector.map(value => value / length); };
const vec = (value, length = 3) => Array.isArray(value) && value.length === length && value.every(Number.isFinite);
const positive = value => Number.isFinite(value) && value > 0;

// Quaternions are [x, y, z, w]; `axis` must already be a unit vector.
export function axisAngle(axis, angle) {
    const half = angle / 2, sine = Math.sin(half);
    return [axis[0] * sine, axis[1] * sine, axis[2] * sine, Math.cos(half)];
}

export const multiply = (a, b) => [
    a[3] * b[0] + a[0] * b[3] + a[1] * b[2] - a[2] * b[1],
    a[3] * b[1] - a[0] * b[2] + a[1] * b[3] + a[2] * b[0],
    a[3] * b[2] + a[0] * b[1] - a[1] * b[0] + a[2] * b[3],
    a[3] * b[3] - a[0] * b[0] - a[1] * b[1] - a[2] * b[2]
];

// Pose of an obstacle at simulation time `time`: { position, rotation }.
export function obstaclePose(spec, time) {
    const phase = time * spec.speed + (spec.phase || 0);
    const axis = normalize(spec.axis || [0, 1, 0]);
    const base = spec.rotation || [0, 0, 0, 1];
    const at = offset => spec.position.map((value, i) => value + axis[i] * offset);
    switch (spec.kind) {
    case 'rotate': return { position: spec.position, rotation: multiply(axisAngle(axis, Math.sin(phase) * spec.amplitude), base) };
    case 'spin': return { position: spec.position, rotation: multiply(axisAngle(axis, phase), base) };
    case 'slide': return { position: at(Math.sin(phase) * spec.amplitude), rotation: base };
    case 'gate': return { position: at(spec.amplitude * Math.max(-1, Math.min(1, (spec.sharp || 2) * Math.sin(phase)))), rotation: base };
    case 'boulder': {
        const offset = spec.amplitude * Math.sin(phase);
        // Rolls about the horizontal cross axis (up x axis) by distance / radius.
        return { position: at(offset), rotation: multiply(axisAngle(normalize([axis[2], 0, -axis[0]]), offset / spec.radius), base) };
    }
    default: return { position: spec.position, rotation: base };
    }
}

// Zones test the ball centre. Boxes are oriented by `yaw` (radians about +Y).
export function inBox(zone, p) {
    const dx = p[0] - zone.position[0], dy = p[1] - zone.position[1], dz = p[2] - zone.position[2];
    const cos = Math.cos(zone.yaw || 0), sin = Math.sin(zone.yaw || 0);
    return Math.abs(dx * cos - dz * sin) <= zone.half[0] && Math.abs(dy) <= zone.half[1] && Math.abs(dx * sin + dz * cos) <= zone.half[2];
}
export const inDisc = (zone, p, height = 0.4) => Math.hypot(p[0] - zone.position[0], p[2] - zone.position[2]) <= zone.radius
    && p[1] >= zone.position[1] - 0.05 && p[1] <= zone.position[1] + height;

export const boostVelocity = (zone, v) => { const s = Math.max(zone.speed, Math.hypot(v[0], v[2])); return [zone.direction[0] * s, v[1], zone.direction[1] * s]; };
export const teleportVelocity = (zone, v) => { const s = Math.max(zone.minSpeed || 0, Math.hypot(v[0], v[2])); return [zone.direction[0] * s, 0, zone.direction[1] * s]; };
// Horizontal speed away from the bumper is raised to at least `kick`.
export function bumperVelocity(bumper, ballPosition, v) {
    let nx = ballPosition[0] - bumper.position[0], nz = ballPosition[2] - bumper.position[2];
    const length = Math.hypot(nx, nz) || 1;
    nx /= length; nz /= length;
    const add = Math.max(0, bumper.kick - (v[0] * nx + v[2] * nz));
    return [v[0] + nx * add, v[1], v[2] + nz * add];
}

export const createZoneState = () => ({ inside: new Set(), teleportUntil: -Infinity, captured: null });
export function resetZoneState(state) { state.inside.clear(); state.teleportUntil = -Infinity; state.captured = null; }

// Bumper contacts that began this step: kick the ball away and report them.
export function applyBumpers(hits, ball, emit) {
    for (const bumper of new Set(hits)) {
        const position = ball.position();
        ball.setVelocity(bumperVelocity(bumper, position, ball.velocity()));
        emit?.('bumper', position, { id: bumper.id });
    }
}

// One fixed step of zone logic: cannon, teleporters, boost pads (edge-triggered, once per entry).
// `ball` adapter: position(), velocity(), setVelocity(v, spin), place(position), hold(position), release(v).
// With `stop(kind)` (preview) teleporter and cannon entries end the path instead of acting.
export function stepZones(zones, state, ball, time, { emit, stop } = {}) {
    const captured = state.captured;
    if (captured) {
        if (time + 1e-9 < captured.release) return;
        state.captured = null;
        ball.release(captured.velocity);
        emit?.('cannon', captured.muzzle, { id: captured.id });
        return;
    }
    const position = ball.position();
    for (const zone of zones) {
        const inside = zone.kind === 'boost' ? inBox(zone, position) : inDisc(zone, position);
        if (!inside) { state.inside.delete(zone.id); continue; }
        if (state.inside.has(zone.id)) continue;
        state.inside.add(zone.id);
        if (zone.kind === 'teleport' && time < state.teleportUntil) continue;
        if (zone.kind !== 'boost' && stop) { stop(zone.kind); return; }
        if (zone.kind === 'cannon') {
            state.captured = { id: zone.id, release: time + (zone.delay || 0), muzzle: zone.muzzle, velocity: zone.velocity };
            ball.hold(zone.muzzle);
            return;
        }
        if (zone.kind === 'teleport') {
            const next = teleportVelocity(zone, ball.velocity());
            ball.place(zone.exit);
            ball.setVelocity(next, true);
            state.teleportUntil = time + TELEPORT_LOCKOUT;
            emit?.('teleport', position, { id: zone.id, exit: zone.exit.slice() });
            return;
        }
        const next = boostVelocity(zone, ball.velocity());
        ball.setVelocity(next);
        emit?.('boost', position, { id: zone.id, speed: Math.hypot(next[0], next[2]) });
    }
}

function validPart(part) {
    const shape = part?.shape ?? 'box';
    if (!part || (part.position !== undefined && !vec(part.position)) || (part.rotation !== undefined && !vec(part.rotation, 4))) return false;
    if (shape === 'box') return vec(part.halfExtents) && part.halfExtents.every(positive);
    if (shape === 'ball') return positive(part.radius);
    return shape === 'cylinder' && positive(part.radius) && positive(part.halfHeight);
}

// Throws TypeError for an obstacle the physics cannot build.
export function validateObstacle(spec) {
    const axis = spec?.axis || [0, 1, 0], parts = spec?.colliders;
    const ok = spec && typeof spec.id === 'string' && ID.test(spec.id) && vec(spec.position) && OBSTACLE_KINDS.includes(spec.kind)
        && vec(axis) && Math.hypot(...axis) > 1e-6 && Number.isFinite(spec.speed) && Number.isFinite(spec.phase || 0)
        // A spin turns at a constant rate, so it needs no amplitude.
        && (spec.kind === 'spin' ? spec.amplitude === undefined || Number.isFinite(spec.amplitude) : Number.isFinite(spec.amplitude))
        && (spec.rotation === undefined || vec(spec.rotation, 4))
        && (spec.sharp === undefined || (spec.sharp >= 1 && spec.sharp <= 4))
        && (spec.kind !== 'boulder' || (positive(spec.radius) && Math.abs(axis[1]) < 1e-6 * Math.hypot(...axis)))
        && (parts === undefined || (Array.isArray(parts) && parts.every(validPart)))
        && (parts?.length || (vec(spec.halfExtents) && spec.halfExtents.every(positive)));
    if (!ok) throw new TypeError('Invalid minigolf obstacle');
    return spec;
}

const unitDirection = value => vec(value, 2) && Math.abs(Math.hypot(...value) - 1) <= 0.01;

// Throws TypeError for a mechanic the physics cannot build.
export function validateMechanic(spec) {
    const common = spec && typeof spec.id === 'string' && ID.test(spec.id) && MECHANIC_KINDS.includes(spec.kind) && vec(spec.position);
    const ok = common && ({
        bumper: () => positive(spec.radius) && positive(spec.height) && positive(spec.kick) && (spec.surface === undefined || WALL_SURFACES.includes(spec.surface)),
        boost: () => vec(spec.half) && spec.half.every(positive) && Number.isFinite(spec.yaw ?? 0) && unitDirection(spec.direction) && positive(spec.speed),
        teleport: () => positive(spec.radius) && vec(spec.exit) && unitDirection(spec.direction) && Number.isFinite(spec.minSpeed ?? 0) && (spec.minSpeed ?? 0) >= 0,
        cannon: () => positive(spec.radius) && vec(spec.muzzle) && vec(spec.velocity) && Number.isFinite(spec.delay ?? 0) && (spec.delay ?? 0) >= 0
    })[spec.kind]();
    if (!ok) throw new TypeError('Invalid minigolf mechanic');
    return spec;
}
