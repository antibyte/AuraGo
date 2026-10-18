import RAPIER from '@dimforge/rapier3d-compat';

const BALL_RADIUS = 0.16;
const FIXED_STEP = 1 / 120;
const MAX_STEPS = 16;
const GRAVITY = 9.81;
let initialized;

const vec3 = value => ({ x: value[0], y: value[1], z: value[2] });
const quat = value => ({ x: value[0], y: value[1], z: value[2], w: value[3] });
const array3 = value => [value.x, value.y, value.z];
const array4 = value => [value.x, value.y, value.z, value.w];
const speed = value => Math.hypot(value.x, value.y, value.z);
const finiteVec3 = value => Array.isArray(value) && value.length === 3 && value.every(Number.isFinite);
const lerp = (a, b, amount) => a + (b - a) * amount;

function slerpQuaternion(a, b, amount) {
    let dot = a[0] * b[0] + a[1] * b[1] + a[2] * b[2] + a[3] * b[3];
    const target = b.slice();
    if (dot < 0) {
        dot = -dot;
        for (let i = 0; i < 4; i++) target[i] = -target[i];
    }
    if (dot > 0.9995) {
        const value = a.map((component, index) => lerp(component, target[index], amount));
        const length = Math.hypot(...value) || 1;
        return value.map(component => component / length);
    }
    const angle = Math.acos(Math.max(-1, Math.min(1, dot)));
    const denominator = Math.sin(angle);
    const fromWeight = Math.sin((1 - amount) * angle) / denominator;
    const toWeight = Math.sin(amount * angle) / denominator;
    return a.map((component, index) => component * fromWeight + target[index] * toWeight);
}

function multiplyQuaternion(a, b) {
    return {
        x: a.w * b.x + a.x * b.w + a.y * b.z - a.z * b.y,
        y: a.w * b.y - a.x * b.z + a.y * b.w + a.z * b.x,
        z: a.w * b.z + a.x * b.y - a.y * b.x + a.z * b.w,
        w: a.w * b.w - a.x * b.x - a.y * b.y - a.z * b.z
    };
}

function rotationAt(obstacle, time) {
    const base = obstacle.rotation ? quat(obstacle.rotation) : { x: 0, y: 0, z: 0, w: 1 };
    if (obstacle.kind !== 'rotate') return base;
    const axis = obstacle.axis || [0, 1, 0];
    const length = Math.hypot(...axis) || 1;
    const angle = Math.sin(time * obstacle.speed + obstacle.phase) * obstacle.amplitude;
    const half = angle / 2;
    return multiplyQuaternion({ x: axis[0] / length * Math.sin(half), y: axis[1] / length * Math.sin(half), z: axis[2] / length * Math.sin(half), w: Math.cos(half) }, base);
}

function contains(bounds, position) {
    return position[0] >= bounds.min[0] && position[0] <= bounds.max[0]
        && position[1] >= bounds.min[1] && position[1] <= bounds.max[1]
        && position[2] >= bounds.min[2] && position[2] <= bounds.max[2];
}

export async function createPhysics(course, { onEvent = () => {} } = {}) {
    initialized ||= RAPIER.init();
    await initialized;
    if (!course || !finiteVec3(course.tee) || !finiteVec3(course.cup) || !finiteVec3(course.bounds?.min) || !finiteVec3(course.bounds?.max) || course.bounds.min.some((value, index) => value > course.bounds.max[index])) throw new TypeError('Invalid minigolf course');

    const world = new RAPIER.World({ x: 0, y: -GRAVITY, z: 0 });
    world.timestep = FIXED_STEP;
    const events = new RAPIER.EventQueue(true);
    const fixed = world.createRigidBody(RAPIER.RigidBodyDesc.fixed());
    const obstacleBodies = [];
    const surfaces = new Map();
    let ballBody;
    let ballCollider;
    let previousPosition;
    let currentPosition;
    let previousRotation;
    let currentRotation;
    let accumulator = 0;
    let time = 0;
    let activeShot = false;
    let shotLocked = false;
    let quietSteps = 0;
    let resolvedShots = 0;
    let disposed = false;
    const activeSurfaceContacts = new Map();

    const surfaceFriction = { grass: 0.82, sand: 1.1, wood: 0.52, stone: 0.68 };
    const addSurface = (collider, surface) => surfaces.set(collider.handle, surface || 'grass');
    const addBox = (position, halfExtents, rotation, surface, body = fixed) => {
        if (!finiteVec3(position) || !finiteVec3(halfExtents) || halfExtents.some(v => v <= 0)) throw new TypeError('Invalid minigolf box collider');
        const desc = RAPIER.ColliderDesc.cuboid(...halfExtents).setTranslation(...position).setFriction(surfaceFriction[surface] ?? 0.75).setRestitution(0.03).setActiveEvents(RAPIER.ActiveEvents.COLLISION_EVENTS);
        if (rotation) desc.setRotation(quat(rotation));
        const collider = world.createCollider(desc, body);
        addSurface(collider, surface);
        return collider;
    };

    try {
        for (const shape of course.colliders || []) {
            if (shape.type === 'box') addBox(shape.position, shape.halfExtents, shape.rotation, shape.surface);
            else if (shape.type === 'mesh') {
                if (!Array.isArray(shape.vertices) || shape.vertices.length < 9 || shape.vertices.length > 3_000_000 || shape.vertices.length % 3 || shape.vertices.some(value => !Number.isFinite(value)) || !Array.isArray(shape.indices) || !shape.indices.length || shape.indices.length % 3 || shape.indices.some(index => !Number.isInteger(index) || index < 0 || index >= shape.vertices.length / 3)) throw new TypeError('Invalid minigolf mesh collider');
                const collider = world.createCollider(RAPIER.ColliderDesc.trimesh(new Float32Array(shape.vertices), new Uint32Array(shape.indices)).setFriction(surfaceFriction[shape.surface] ?? 0.75).setRestitution(0.02).setActiveEvents(RAPIER.ActiveEvents.COLLISION_EVENTS), fixed);
                addSurface(collider, shape.surface);
            } else throw new TypeError('Unsupported minigolf collider');
        }

        const cupRadius = Number.isFinite(course.cupRadius) ? course.cupRadius : 0.32;
        const cupSensor = world.createCollider(RAPIER.ColliderDesc.cylinder(0.11, Math.max(0.05, cupRadius - 0.03)).setTranslation(course.cup[0], course.cup[1] - 0.27, course.cup[2]).setSensor(true), fixed);
        void cupSensor;
        for (const volume of course.water || []) {
            if (!finiteVec3(volume.min) || !finiteVec3(volume.max) || volume.min.some((value, index) => value > volume.max[index])) throw new TypeError('Invalid minigolf water volume');
        }

        for (const obstacle of course.obstacles || []) {
            if (typeof obstacle.id !== 'string' || !obstacle.id || !finiteVec3(obstacle.position) || !finiteVec3(obstacle.halfExtents) || !finiteVec3(obstacle.axis || [0, 1, 0]) || !['rotate', 'slide'].includes(obstacle.kind) || !Number.isFinite(obstacle.amplitude) || !Number.isFinite(obstacle.speed) || !Number.isFinite(obstacle.phase || 0)) throw new TypeError('Invalid minigolf obstacle');
            const body = world.createRigidBody(RAPIER.RigidBodyDesc.kinematicPositionBased().setTranslation(...obstacle.position).setRotation(obstacle.rotation ? quat(obstacle.rotation) : { x: 0, y: 0, z: 0, w: 1 }));
            // Obstacle collider poses are relative to their kinematic body. The
            // manifest's optional pieces describe the visible compound gate;
            // legacy/single-box entries still work through halfExtents.
            const parts = Array.isArray(obstacle.colliders) && obstacle.colliders.length
                ? obstacle.colliders
                : [{ position: [0, 0, 0], halfExtents: obstacle.halfExtents }];
            const colliders = parts.map(part => addBox(part.position || [0, 0, 0], part.halfExtents, part.rotation, obstacle.surface || 'wood', body));
            obstacleBodies.push({ spec: obstacle, body, colliders });
        }

        ballBody = world.createRigidBody(RAPIER.RigidBodyDesc.dynamic().setTranslation(...course.tee).setLinearDamping(0.9).setAngularDamping(0.18).setCcdEnabled(true));
        ballCollider = world.createCollider(RAPIER.ColliderDesc.ball(BALL_RADIUS).setMass(0.22).setFriction(0.75).setRestitution(0.12).setActiveEvents(RAPIER.ActiveEvents.COLLISION_EVENTS), ballBody);
        ballBody.setBodyType(RAPIER.RigidBodyType.Fixed, true);
        previousPosition = array3(ballBody.translation());
        currentPosition = previousPosition.slice();
        previousRotation = array4(ballBody.rotation());
        currentRotation = previousRotation.slice();
    } catch (error) {
        events.free();
        world.free();
        throw error;
    }

    function emit(type, position, extra = {}) {
        if (type !== 'impact') {
            activeShot = false;
            quietSteps = 0;
            shotLocked = type !== 'rest';
            resolvedShots++;
            activeSurfaceContacts.clear();
            ballBody.setLinvel({ x: 0, y: 0, z: 0 }, true);
            ballBody.setAngvel({ x: 0, y: 0, z: 0 }, true);
            ballBody.setBodyType(RAPIER.RigidBodyType.Fixed, true);
            resetRenderPose();
        }
        onEvent({ type, position: position.slice(), ...extra });
    }

    function updateObstacles() {
        for (const { spec, body } of obstacleBodies) {
            const { position, rotation } = obstaclePose(spec, time);
            body.setNextKinematicTranslation(vec3(position));
            body.setNextKinematicRotation(rotation);
        }
    }
    function obstaclePose(spec, atTime) {
            const phase = atTime * spec.speed + spec.phase;
            const offset = Math.sin(phase) * spec.amplitude;
            const axis = spec.axis || [0, 1, 0];
            const length = Math.hypot(...axis) || 1;
            const position = spec.kind === 'slide'
                ? [spec.position[0] + axis[0] / length * offset, spec.position[1] + axis[1] / length * offset, spec.position[2] + axis[2] / length * offset]
                : spec.position;
            return { position, rotation: rotationAt(spec, atTime) };
    }

    function resolveConditions() {
        if (!activeShot) return;
        const position = getBall();
        const inWater = (course.water || []).some(volume => contains(volume, position));
        if (inWater) { emit('water', position); return; }
        if (!contains(course.bounds, position)) { emit('out', position); return; }
        const dx = position[0] - course.cup[0];
        const dz = position[2] - course.cup[2];
        const cupRadius = Number.isFinite(course.cupRadius) ? course.cupRadius : 0.32;
        const velocity = ballBody.linvel();
        if (position[1] < course.cup[1] - 0.1 && Math.hypot(dx, dz) < cupRadius - BALL_RADIUS * 0.1 && Math.hypot(velocity.x, velocity.z) <= 2.4) {
            emit('cup', position, { speed: speed(velocity) });
            return;
        }
        const angular = ballBody.angvel();
        if (time > shotStartedAt + 0.15 && speed(velocity) < 0.075 && speed(angular) < 0.45) quietSteps++;
        else quietSteps = 0;
        if (quietSteps >= 30) emit('rest', position);
    }

    let shotStartedAt = 0;
    function getBall() { return array3(ballBody.translation()); }
    function resetRenderPose() {
        currentPosition = getBall();
        previousPosition = currentPosition.slice();
        currentRotation = array4(ballBody.rotation());
        previousRotation = currentRotation.slice();
    }
    function getObstacles() {
        return obstacleBodies.map(({ spec, body }) => ({ id: spec.id, position: array3(body.translation()), rotation: array4(body.rotation()) }));
    }
    function getSnapshot() {
        const velocity = ballBody.linvel();
        const alpha = Math.max(0, Math.min(1, accumulator / FIXED_STEP));
        const position = currentPosition.map((value, index) => lerp(previousPosition[index], value, alpha));
        return { position, rotation: slerpQuaternion(previousRotation, currentRotation, alpha), velocity: array3(velocity), moving: activeShot && speed(velocity) > 0.075, time };
    }
    function step(dt) {
        if (disposed) throw new Error('Minigolf physics has been disposed');
        if (!Number.isFinite(dt) || dt <= 0) return { ...getSnapshot(), obstacles: getObstacles() };
        accumulator += Math.min(dt, FIXED_STEP * MAX_STEPS);
        let count = 0;
        while (accumulator + 1e-10 >= FIXED_STEP && count < MAX_STEPS) {
            const resolvedBeforeStep = resolvedShots;
            updateObstacles();
            previousPosition = currentPosition.slice();
            previousRotation = currentRotation.slice();
            world.step(events);
            currentPosition = getBall();
            currentRotation = array4(ballBody.rotation());
            time += FIXED_STEP;
            accumulator -= FIXED_STEP;
            count++;
            events.drainCollisionEvents((first, second, started) => {
                if (first !== ballCollider.handle && second !== ballCollider.handle) return;
                const other = first === ballCollider.handle ? second : first;
                if (!started) { activeSurfaceContacts.delete(other); return; }
                activeSurfaceContacts.set(other, surfaces.get(other));
                emit('impact', getBall(), { speed: speed(ballBody.linvel()), surface: surfaces.get(other) });
            });
            if (activeShot && [...activeSurfaceContacts.values()].includes('sand')) {
                // Sand adds rolling resistance while contact is active, without
                // damping airborne motion or the ball's vertical velocity.
                const velocity = ballBody.linvel();
                const damping = Math.exp(-0.85 * FIXED_STEP);
                ballBody.setLinvel({ x: velocity.x * damping, y: velocity.y, z: velocity.z * damping }, true);
            }
            resolveConditions();
            if (resolvedShots !== resolvedBeforeStep) {
                // An event handler may already have positioned or launched the next ball.
                // Discard this frame's remaining time so that new turn never consumes it.
                accumulator = 0;
                break;
            }
        }
        if (accumulator >= FIXED_STEP) accumulator %= FIXED_STEP;
        return { ...getSnapshot(), obstacles: getObstacles() };
    }
    function shoot(direction, power) {
        if (disposed || activeShot || shotLocked || !finiteVec3([direction?.[0], 0, direction?.[1]]) || !Number.isFinite(power) || power <= 0 || power > 1) return false;
        const length = Math.hypot(direction[0], direction[1]);
        if (length < 1e-6) return false;
        const launchSpeed = 12 * power;
        ballBody.setBodyType(RAPIER.RigidBodyType.Dynamic, true);
        ballBody.wakeUp();
        ballBody.setLinvel({ x: direction[0] / length * launchSpeed, y: 0, z: direction[1] / length * launchSpeed }, true);
        ballBody.setAngvel({ x: 0, y: 0, z: 0 }, true);
        activeShot = true;
        quietSteps = 0;
        shotStartedAt = time;
        return true;
    }
    function setBall(position) {
        if (disposed || !finiteVec3(position)) return false;
        ballBody.setBodyType(RAPIER.RigidBodyType.Fixed, true);
        ballBody.setTranslation(vec3(position), true);
        ballBody.setRotation({ x: 0, y: 0, z: 0, w: 1 }, true);
        ballBody.setLinvel({ x: 0, y: 0, z: 0 }, true);
        ballBody.setAngvel({ x: 0, y: 0, z: 0 }, true);
        resetRenderPose();
        activeSurfaceContacts.clear();
        activeShot = false;
        shotLocked = false;
        quietSteps = 0;
        return true;
    }
    function resetTime() {
        time = 0; accumulator = 0; shotStartedAt = 0;
        for (const { spec, body } of obstacleBodies) {
            const pose = obstaclePose(spec, 0);
            body.setTranslation(vec3(pose.position), true);
            body.setRotation(pose.rotation, true);
            body.setNextKinematicTranslation(vec3(pose.position));
            body.setNextKinematicRotation(pose.rotation);
        }
    }
    function dispose() {
        if (disposed) return;
        disposed = true;
        events.free();
        world.free();
    }

    return { step, shoot, setBall, getBall, getSnapshot, getObstacles, resetTime, dispose };
}
