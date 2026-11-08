import RAPIER from '@dimforge/rapier3d-compat';
import { STEP, BALL_RADIUS, BALL_MASS, clamp } from './tables.js';

const ready = RAPIER.init();
const STATIC_GROUP = 0x00010002, BALL_GROUP = 0x00020007, ACTOR_GROUP = 0x00040002;
const SURFACES = { wood: [.18, .08], metal: [.12, .45], rubber: [.8, .72] };
const vector = a => ({ x: a[0], y: a[1], z: a[2] });
const array = v => [v.x, v.y, v.z];
const validVector = a => Array.isArray(a) && a.length === 3 && a.every(n => Number.isFinite(n) && Math.abs(n) < 100);
const pose = body => ({ position: array(body.translation()), rotation: [body.rotation().x, body.rotation().y, body.rotation().z, body.rotation().w] });
const length = v => Math.hypot(v.x, v.y, v.z);

export function validatePhysics(data) {
    if (!data || data.version !== 1 || !['neon', 'leviathan', 'temple'].includes(data.id) || !validVector(data.shooter)) throw Error('physics');
    for (const key of ['meshes', 'flippers', 'bumpers', 'targets', 'sensors']) if (!Array.isArray(data[key]) || data[key].length > 100) throw Error('physics');
    if (!data.meshes.length || data.flippers.length < 2 || data.flippers.length > 3 || data.bumpers.length !== 3 || data.targets.length !== 3) throw Error('physics');
    for (const mesh of data.meshes) {
        if (!SURFACES[mesh.surface] || !Array.isArray(mesh.vertices) || mesh.vertices.length % 3 || mesh.vertices.length > 600000 || !mesh.vertices.every(n => Number.isFinite(n) && Math.abs(n) < 4) || !Array.isArray(mesh.indices) || mesh.indices.length % 3 || mesh.indices.length > 1200000 || !mesh.indices.every(n => Number.isInteger(n) && n >= 0 && n < mesh.vertices.length / 3)) throw Error('physics');
    }
    const ids = new Set();
    for (const item of [...data.flippers, ...data.bumpers, ...data.targets, ...data.sensors]) {
        if (!item || !validVector(item.at) || typeof item.id !== 'string' || !/^[a-z0-9-]{1,40}$/.test(item.id) || ids.has(item.id)) throw Error('physics');
        ids.add(item.id);
    }
    for (const f of data.flippers) if (![1, -1].includes(f.side) || !Array.isArray(f.vertices) || f.vertices.length < 24 || f.vertices.length % 3 || !f.vertices.every(n => Number.isFinite(n) && Math.abs(n) <= .2)) throw Error('physics');
    for (const b of data.bumpers) if (!(b.radius > .01 && b.radius < .05)) throw Error('physics');
    for (const s of data.sensors) if (!['ramp', 'orbit', 'lock', 'skill', 'upper', 'spinner'].includes(s.type) || !validVector(s.half) || s.half.some(n => n <= 0)) throw Error('physics');
    if (data.magnet && (!validVector(data.magnet.at) || !(data.magnet.radius > 0 && data.magnet.radius <= .1) || !(data.magnet.force > 0 && data.magnet.force <= 1) || !(data.magnet.hold > 0 && data.magnet.hold <= 2))) throw Error('physics');
    return data;
}

// Swept sensors also see a fast sphere that crosses a volume in one fixed tick.
export function segmentBox(a, b, center, half) {
    let low = 0, high = 1;
    for (let axis = 0; axis < 3; axis++) {
        const d = b[axis] - a[axis], min = center[axis] - half[axis] - BALL_RADIUS, max = center[axis] + half[axis] + BALL_RADIUS;
        if (Math.abs(d) < 1e-10) { if (a[axis] < min || a[axis] > max) return false; }
        else { let x = (min - a[axis]) / d, y = (max - a[axis]) / d; if (x > y) [x, y] = [y, x]; low = Math.max(low, x); high = Math.min(high, y); if (low > high) return false; }
    }
    return true;
}

export async function createSimulation(raw) {
    const data = validatePhysics(raw);
    await ready;
    const tilt = 6.5 * Math.PI / 180;
    const world = new RAPIER.World({ x: 0, y: -9.81 * Math.cos(tilt), z: -9.81 * Math.sin(tilt) });
    world.timestep = STEP; world.numSolverIterations = 8; world.maxCcdSubsteps = 4;
    world.integrationParameters.normalizedAllowedLinearError = .0001;
    const queue = new RAPIER.EventQueue(true), balls = new Map(), actors = [], tags = new Map(), events = [], pending = [], targets = [], cooldowns = new Map();
    let ticks = 0, disposed = false, coils = true, gateWanted = false, gateHeight = 0, pull = 0, releaseUntil = 0, autoStart = null, searchCount = 0;
    let magneticBall = null, magneticUntil = 0, magneticReady = 0;
    const fixed = world.createRigidBody(RAPIER.RigidBodyDesc.fixed());
    const fixedCollider = (desc, tag) => {
        const collider = world.createCollider(desc.setCollisionGroups(STATIC_GROUP), fixed);
        if (tag) tags.set(collider.handle, tag);
        return collider;
    };
    for (const mesh of data.meshes) {
        const [friction, restitution] = SURFACES[mesh.surface];
        fixedCollider(RAPIER.ColliderDesc.trimesh(new Float32Array(mesh.vertices), new Uint32Array(mesh.indices), RAPIER.TriMeshFlags.FIX_INTERNAL_EDGES)
            .setFriction(friction).setRestitution(restitution), { surface: mesh.surface });
    }
    for (const item of data.bumpers) fixedCollider(RAPIER.ColliderDesc.cylinder(.024, item.radius).setTranslation(...item.at).setFriction(.7).setRestitution(.65).setActiveEvents(RAPIER.ActiveEvents.COLLISION_EVENTS), { ...item, type: 'bumper' });
    for (const side of [-1, 1]) {
        const vertices = [];
        for (const y of [0, .026]) for (const [x, z] of [[.194, .385], [.130, .295], [.139, .246]]) vertices.push(side * x, y, z);
        fixedCollider(RAPIER.ColliderDesc.convexHull(new Float32Array(vertices)).setFriction(.7).setRestitution(.65).setActiveEvents(RAPIER.ActiveEvents.COLLISION_EVENTS), { type: 'sling', id: 'sling-' + side, side });
    }
    for (const item of data.flippers) {
        const body = world.createRigidBody(RAPIER.RigidBodyDesc.dynamic().setTranslation(...item.at).setCanSleep(false).setCcdEnabled(true));
        world.createCollider(RAPIER.ColliderDesc.convexHull(new Float32Array(item.vertices)).setMass(.14).setFriction(.85).setRestitution(.68).setCollisionGroups(ACTOR_GROUP), body);
        const joint = world.createImpulseJoint(RAPIER.JointData.revolute(vector(item.at), { x: 0, y: 0, z: 0 }, { x: 0, y: 1, z: 0 }), fixed, body, true);
        joint.setLimits(-.55, .55); joint.configureMotorModel(RAPIER.MotorModel.ForceBased); joint.setMotorMaxForce(1);
        actors.push({ ...item, body, joint, rest: item.side * .42, raised: -item.side * .50 });
    }
    for (const item of data.targets) {
        const body = world.createRigidBody(RAPIER.RigidBodyDesc.kinematicPositionBased().setTranslation(...item.at));
        const c = world.createCollider(RAPIER.ColliderDesc.cuboid(.015, .021, .0045).setFriction(.2).setRestitution(.4).setCollisionGroups(ACTOR_GROUP).setActiveEvents(RAPIER.ActiveEvents.COLLISION_EVENTS), body);
        const target = { ...item, body, down: false, height: 0 }; targets.push(target); tags.set(c.handle, { type: 'target', id: item.id, target });
    }
    const gate = world.createRigidBody(RAPIER.RigidBodyDesc.kinematicPositionBased().setTranslation(0, .037, 1.055));
    world.createCollider(RAPIER.ColliderDesc.cuboid(.046, .035, .004).setFriction(.2).setRestitution(.4).setCollisionGroups(ACTOR_GROUP), gate);
    const plunger = world.createRigidBody(RAPIER.RigidBodyDesc.dynamic().setTranslation(.301, .018, .13).setCanSleep(false).setCcdEnabled(true));
    world.createCollider(RAPIER.ColliderDesc.cuboid(.0175, .011, .0075).setMass(.3).setFriction(.1).setRestitution(.35).setCollisionGroups(ACTOR_GROUP), plunger);
    const spring = world.createImpulseJoint(RAPIER.JointData.prismatic({ x: .301, y: .018, z: .13 }, { x: 0, y: 0, z: 0 }, { x: 0, y: 0, z: 1 }), fixed, plunger, true);
    spring.setLimits(-.075, .025); spring.configureMotorModel(RAPIER.MotorModel.ForceBased); spring.setMotorMaxForce(55);
    const spinner = world.createRigidBody(RAPIER.RigidBodyDesc.dynamic().setTranslation(-.215, .036, .91).setAngularDamping(.15).setCcdEnabled(true));
    world.createCollider(RAPIER.ColliderDesc.cuboid(.0135, .022, .0015).setMass(.015).setFriction(.1).setRestitution(.35).setCollisionGroups(ACTOR_GROUP), spinner);
    world.createImpulseJoint(RAPIER.JointData.revolute({ x: -.215, y: .057, z: .91 }, { x: 0, y: .021, z: 0 }, { x: 1, y: 0, z: 0 }), fixed, spinner, true);
    function event(type, ball, id, extra = {}) { events.push({ type, ball, id, tick: ticks, ...extra }); }
    function spawn(id, position = data.shooter, velocity = [0, 0, 0]) {
        if (balls.has(id) || balls.size >= 3 || !validVector(position) || !validVector(velocity)) throw Error('ball');
        const body = world.createRigidBody(RAPIER.RigidBodyDesc.dynamic().setTranslation(...position).setLinvel(...velocity).setCanSleep(false).setCcdEnabled(true).setLinearDamping(.002).setAngularDamping(.002));
        const collider = world.createCollider(RAPIER.ColliderDesc.ball(BALL_RADIUS).setMass(BALL_MASS).setFriction(.3).setRestitution(.75).setRestitutionCombineRule(RAPIER.CoefficientCombineRule.Min).setCollisionGroups(BALL_GROUP).setActiveEvents(RAPIER.ActiveEvents.COLLISION_EVENTS), body);
        const record = { id, body, collider, previous: pose(body), current: pose(body), sensors: new Set(), shot: false, lastActivity: ticks, pocketAt: null, resting: 0, searches: 0 };
        balls.set(id, record); tags.set(collider.handle, { type: 'ball', id });
        return record;
    }
    function removeBall(id) { const ball = balls.get(id); if (!ball) return; tags.delete(ball.collider.handle); world.removeRigidBody(ball.body); balls.delete(id); for (const key of cooldowns.keys()) if (key.startsWith(id + ':')) cooldowns.delete(key); }
    const waiting = () => [...balls.values()].find(b => { const p = b.body.translation(); return p.x > .276 && p.z < .4 && p.y > -.01; });
    function feedPending() {
        if (!pending.length || waiting()) return;
        const next = pending.shift(); spawn(next.id); autoStart = next.auto ? ticks * STEP : null;
    }
    function contact(a, b, started) {
        if (!started) return;
        let tagA = tags.get(a), tagB = tags.get(b);
        if (tagB?.type === 'ball') [tagA, tagB] = [tagB, tagA];
        if (tagA?.type !== 'ball' || !tagB || tagB.type === 'ball') return;
        const ball = balls.get(tagA.id); if (!ball) return;
        ball.lastActivity = ticks;
        if (!coils || !['target', 'bumper', 'sling'].includes(tagB.type)) return;
        const key = tagA.id + ':' + tagB.id;
        if (ticks - (cooldowns.get(key) ?? -10000) < 24) return;
        cooldowns.set(key, ticks);
        if (tagB.type === 'target') { if (tagB.target.down) return; tagB.target.down = true; }
        else {
            const p = ball.body.translation(), v = vector(ball.previousVelocity || [0, 0, 0]);
            let dx = tagB.type === 'sling' ? -tagB.side * .6 : p.x - tagB.at[0], dz = tagB.type === 'sling' ? .8 : p.z - tagB.at[2];
            const norm = Math.hypot(dx, dz) || 1; dx /= norm; dz /= norm;
            if (v.x * dx + v.z * dz >= -.1) return;
            ball.body.applyImpulse({ x: dx * BALL_MASS * 2.8, y: .002, z: dz * BALL_MASS * 2.8 }, true);
        }
        event(tagB.type, ball.id, tagB.id);
    }
    function step(input = {}) {
        if (disposed) throw Error('disposed');
        ticks++; const time = ticks * STEP;
        feedPending();
        for (const actor of actors) {
            const pressed = actor.id === 'right' ? input.right : input.left;
            const powered = coils && pressed;
            actor.joint.setMotorMaxForce(powered ? 1 : .16);
            actor.joint.configureMotorPosition(powered ? actor.raised : actor.rest, powered ? 8 : 1.4, powered ? .10 : .045);
        }
        for (const target of targets) {
            const wanted = target.down ? -.065 : 0;
            target.height += clamp(wanted - target.height, -STEP * .65, STEP * .65);
            target.body.setNextKinematicTranslation({ x: target.at[0], y: target.at[1] + target.height, z: target.at[2] });
        }
        gateHeight += clamp((coils && gateWanted ? .12 : 0) - gateHeight, -STEP * .4, STEP * .4);
        gate.setNextKinematicTranslation({ x: 0, y: .037 + gateHeight, z: 1.055 });
        let command = clamp(Number(input.plunger) || 0, 0, 1);
        const returned = waiting();
        if (coils && autoStart === null && returned?.shot && length(returned.body.linvel()) < .12 && ticks-returned.lastActivity > 480) autoStart=time;
        if (autoStart !== null) { const elapsed = time - autoStart; command = elapsed < .9 ? .8 : 0; if (elapsed > 1.2) autoStart = null; }
        const oldPull = pull;
        if (command > .02 && coils) pull = Math.min(command, pull + STEP * 1.4);
        else if (oldPull > 0) {
            pull = 0; releaseUntil = time + .18;
            const ball = waiting(); if (ball && coils) { ball.shot = true; event('launch', ball.id, 'plunger'); }
        }
        spring.configureMotorPosition(coils && time < releaseUntil ? .022 : -.07 * pull, 2800, time < releaseUntil ? 4 : 18);
        if (data.magnet && coils) {
            if (magneticBall !== null && (time >= magneticUntil || !balls.has(magneticBall))) {
                const ball = balls.get(magneticBall); if (ball) ball.body.applyImpulse({ x: 0, y: 0, z: -.055 }, true);
                magneticBall = null; magneticReady = time + 8;
            }
            if (magneticBall === null && time >= magneticReady) {
                const ball = [...balls.values()].find(b => { const p = b.body.translation(); return p.y < .045 && Math.hypot(p.x-data.magnet.at[0],p.z-data.magnet.at[2]) < data.magnet.radius; });
                if (ball) { magneticBall = ball.id; magneticUntil = time + data.magnet.hold; event('magnet', ball.id, 'relic'); }
            }
            const ball = balls.get(magneticBall);
            if (ball) {
                const p = ball.body.translation(), v = ball.body.linvel();
                const fx = (data.magnet.at[0]-p.x)*14-v.x*1.6, fz = (data.magnet.at[2]-p.z)*14-v.z*1.6;
                const scale = Math.min(1,data.magnet.force/(Math.hypot(fx,fz)||1));
                ball.body.applyImpulse({x:fx*scale*STEP,y:0,z:fz*scale*STEP},true);
            }
        } else magneticBall = null;
        for (const ball of balls.values()) { ball.previous = pose(ball.body); ball.previousVelocity = array(ball.body.linvel()); }
        world.step(queue); queue.drainCollisionEvents(contact);
        for (const ball of [...balls.values()]) {
            const p = ball.body.translation(), v = ball.body.linvel(); ball.current = pose(ball.body);
            if (![p.x, p.y, p.z, v.x, v.y, v.z].every(Number.isFinite) || p.y < -.19 || p.z < -.08 || Math.abs(p.x) > .6 || p.z > 1.55 || p.y > 1.5) { event('drain', ball.id, 'drain'); removeBall(ball.id); continue; }
            for (const sensor of data.sensors) {
                const eligible = sensor.type !== 'lock' || p.y < -.008;
                const inside = eligible && sensor.at.every((c, i) => Math.abs(ball.current.position[i] - c) <= sensor.half[i] + BALL_RADIUS);
                const crossing = eligible && !ball.sensors.has(sensor.id) && segmentBox(ball.previous.position, ball.current.position, sensor.at, sensor.half);
                const key = ball.id + ':sensor:' + sensor.id;
                if (crossing && ticks - (cooldowns.get(key) ?? -10000) > 144) {
                    cooldowns.set(key, ticks); ball.lastActivity = ticks;
                    if (sensor.type !== 'lock' || p.y < -.008) {
                        event(sensor.type, ball.id, sensor.id);
                        if (sensor.type === 'lock') ball.pocketAt = time;
                    }
                }
                if (inside) ball.sensors.add(sensor.id); else ball.sensors.delete(sensor.id);
            }
            if (!coils && p.y < -.01 && p.z > 1.05) { event('drain', ball.id, 'pocket'); removeBall(ball.id); continue; }
            // Rolling resistance is a contact torque; airborne balls keep their spin.
            let supported = false;
            world.contactPairsWith(ball.collider, other => world.contactPair(ball.collider, other, manifold => { if (Math.abs(manifold.normal().y) > .5) supported = true; }));
            ball.supported = supported;
            if (supported && Math.abs(v.y) < .15) {
                const omega = ball.body.angvel(), speed = length(omega);
                if (speed > .01) { const inertia = .4 * BALL_MASS * BALL_RADIUS ** 2, amount = Math.min(speed, .08 / BALL_RADIUS * STEP) * inertia / speed; ball.body.applyTorqueImpulse({ x: -omega.x * amount, y: -omega.y * amount, z: -omega.z * amount }, true); }
            }
            const cradled = p.z < .30 && (input.left || input.right);
            ball.resting = length(v) < .08 && !cradled && !(p.x > .276 && p.z < .4) ? ball.resting + STEP : 0;
            if (coils && ball.resting > 15) {
                ball.resting=10; ball.searches++; searchCount++;
                if (ball.searches > 3) { event('drain',ball.id,'search'); removeBall(ball.id); }
                else if (p.y < 0) eject(ball.id);
                else ball.body.applyImpulse({x:-Math.sign(p.x)*.008,y:.09,z:-.025},true);
                events.push({type:'search',ball:ball.id,tick:ticks});
            }
        }
        return ticks;
    }
    function eject(id) {
        const b = balls.get(id); if (!b || !coils) return;
        b.body.applyImpulse({ x: 0, y: .30, z: -.032 }, true); b.pocketAt = null; b.lastActivity = ticks;
    }
    function snapshot(alpha = 1) {
        alpha = clamp(alpha, 0, 1);
        const ballStates = [...balls.values()].map(b => ({
            id: b.id,
            position: b.current.position.map((n, i) => b.previous.position[i] + (n - b.previous.position[i]) * alpha),
            rotation: b.current.rotation,
            velocity: array(b.body.linvel()), angularVelocity: array(b.body.angvel()), grounded: !!b.supported
        }));
        const actorStates = actors.map(a => ['flipper-' + a.id, pose(a.body)]).concat(
            targets.map(t => [t.id, pose(t.body)]), [['gate', pose(gate)], ['plunger', pose(plunger)], ['spinner', pose(spinner)]]);
        return { time: ticks * STEP, tick: ticks, pull, coils, gate: gateHeight / .12, balls: ballStates, actors: Object.fromEntries(actorStates) };
    }
    // Settle unpowered actuators before feeding the first ball.
    for (let i = 0; i < 240; i++) step(); ticks = 0; events.length = 0;
    return {
        step, snapshot, events: () => events.splice(0), feed(id, auto = false) { if (balls.has(id) || pending.some(p => p.id === id) || balls.size + pending.length >= 3) throw Error('feed'); pending.push({ id, auto }); },
        setGate(value) { gateWanted = !!value; },
        setCoils(value) {
            const next = !!value;
            if (coils && !next) {
                for (const item of pending) event('drain', item.id, 'trough');
                pending.length = 0;
                const ball = waiting(); if (ball) { event('drain', ball.id, 'shooter'); removeBall(ball.id); }
            }
            coils = next;
        },
        resetTargets() { targets.forEach(t => { t.down = false; }); }, setTargets(ids) { targets.forEach(t => { t.down = ids.includes(t.id); }); }, eject,
        nudge(x, z) { if (coils) for (const b of balls.values()) b.body.applyImpulse({ x: clamp(x, -1, 1) * BALL_MASS * .30, y: 0, z: clamp(z, -1, 1) * BALL_MASS * .30 }, true); },
        placeBall(id, position, velocity = [0, 0, 0]) { removeBall(id); return spawn(id, position, velocity); },
        resetBall(id) { pending.length = 0; for (const key of [...balls.keys()]) removeBall(key); spawn(id); pull = 0; autoStart = null; },
        clearBalls() { pending.length = 0; for (const key of [...balls.keys()]) removeBall(key); pull = 0; autoStart = null; },
        stats: () => ({ bodies: world.bodies.len(), colliders: world.colliders.len(), balls: balls.size, pending: pending.length, ticks, searchCount }),
        dispose() { if (disposed) return; disposed = true; pending.length = 0; balls.clear(); tags.clear(); cooldowns.clear(); events.length = 0; queue.free(); world.free(); }
    };
}
