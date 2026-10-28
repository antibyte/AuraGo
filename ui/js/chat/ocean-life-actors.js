// Ocean chat theme: the moving parts of the sea life layer (schools, cruisers, jellies, the whale).
// Every factory builds its InstancedMeshes into ctx.scene and returns an actor
// { id, count, update(t, dt), refresh(), visible?, repulsor? }. The scene module owns loading,
// the renderer and the frame loop.
import * as THREE from 'three';
import { createSchool } from './ocean-life-boids.js';

export const TAU = Math.PI * 2;

// The species table reads freq as rad/s of the stroke phase; slow animals get a faster
// visible stroke than the table's raw number so that the motion is readable at a distance.
const STROKE_RATE = { fish: 1, shark: 1.6, turtle: 2, whale: 3, manta: 1, jelly: 1 };
const WHALE_PASS_SECONDS = 40;
const CRUISER_TURN_RATE = 1.8;
const DIVE_EASE_SECONDS = 2;  // time a diving cruiser needs between its path depth and the deep end of its band
const DIVE_MARGIN = 1;        // metres beyond the lane edge at which the dive starts
const DIVE_LOOKAHEAD = 1;     // seconds of travel the dive decision looks ahead

const _matrix = new THREE.Matrix4();
const _quat = new THREE.Quaternion();
const _rollQuat = new THREE.Quaternion();
const _pos = new THREE.Vector3();
const _target = new THREE.Vector3();
const _scale = new THREE.Vector3();
const _axis = new THREE.Vector3();
const _up = new THREE.Vector3(0, 1, 0);
const _zAxis = new THREE.Vector3(0, 0, 1);

export function clamp(x, lo, hi) {
    return x < lo ? lo : x > hi ? hi : x;
}

// GLSL-style smoothstep that also accepts edge0 > edge1 (falling edge).
export function smoothstep(edge0, edge1, x) {
    const t = clamp((x - edge0) / (edge1 - edge0), 0, 1);
    return t * t * (3 - 2 * t);
}

function between(rng, range) {
    return range[0] + rng() * (range[1] - range[0]);
}

function strokeAmplitude(spec) {
    // the whale's amp is already in metres, everything else is a fraction of the body length
    return spec.shader === 'whale' ? spec.swim.amp : spec.swim.amp * spec.length;
}

// Camera at the origin looking down -Z. The horizontal angle is fixed at 50 degrees, so the visible
// width at depth d is 2 * d * tan(25 deg) whatever the canvas aspect is.
export function createView(width, height) {
    const tanHalf = Math.tan(25 * Math.PI / 180);
    const view = {
        width,
        height,
        aspect: width / height,
        tanHalf,
        resize(w, h) {
            view.width = w;
            view.height = h;
            view.aspect = w / h;
        },
        frustumWidth(d) {
            return 2 * Math.abs(d) * tanHalf;
        },
        frustumHeight(d) {
            return 2 * Math.abs(d) * tanHalf / view.aspect;
        },
        // the central reading lane (the Black Matrix rule: at most 62 % of the width, 920 px)
        laneHalfWidth(z) {
            return Math.min(0.62, 920 / view.width) * 0.5 * view.frustumWidth(z);
        },
        // distant animals may cross the lane
        laneStrength(z) {
            return smoothstep(-28, -14, z);
        }
    };
    return view;
}

// Orientation: +Z of the model (the nose) looks from the position toward the target, bank rolls
// about the heading axis.
export function composeHeading(out, px, py, pz, tx, ty, tz, scale, roll) {
    _pos.set(px, py, pz);
    _target.set(tx, ty, tz);
    _matrix.lookAt(_target, _pos, _up);
    _quat.setFromRotationMatrix(_matrix);
    if (roll) {
        _rollQuat.setFromAxisAngle(_zAxis, roll);
        _quat.multiply(_rollQuat);
    }
    _scale.set(scale, scale, scale);
    return out.compose(_pos, _quat, _scale);
}

function fillBounds(bounds, view, depth) {
    const mid = (depth[0] + depth[1]) / 2;
    const hx = 0.46 * view.frustumWidth(mid);
    const hy = 0.42 * view.frustumHeight(mid);
    bounds.x[0] = -hx; bounds.x[1] = hx;
    bounds.y[0] = -hy; bounds.y[1] = hy;
    bounds.z[0] = Math.min(depth[0], depth[1]);
    bounds.z[1] = Math.max(depth[0], depth[1]);
}

// One InstancedMesh per material part, all sharing the per-instance aInst attribute
// (phase, frequency, amplitude, fade). Far bands render first so alpha blending stays ordered.
function createInstanced(ctx, model, count, extra) {
    const inst = new THREE.InstancedBufferAttribute(new Float32Array(count * 4), 4);
    inst.setUsage(THREE.DynamicDrawUsage);
    const tint = extra && extra.tint ? new THREE.InstancedBufferAttribute(new Float32Array(count * 3), 3) : null;
    const order = Math.round((model.spec.depth[0] + model.spec.depth[1]) / 2);
    const meshes = model.parts.map(part => {
        part.geometry.setAttribute('aInst', inst);
        if (tint) part.geometry.setAttribute('aTint', tint);
        const material = ctx.makeMaterial(model, part.name, extra);
        const mesh = new THREE.InstancedMesh(part.geometry, material, count);
        mesh.frustumCulled = false;
        mesh.renderOrder = order;
        mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
        ctx.scene.add(mesh);
        ctx.owned.push(mesh, part.geometry, material);
        return mesh;
    });
    return { meshes, inst, tint };
}

function flagMatrices(meshes) {
    for (let m = 0; m < meshes.length; m++) meshes[m].instanceMatrix.needsUpdate = true;
}

// Schools: boids with the reading lane, the pointer and the sharks as repulsors.
export function createSchoolActor(ctx, model, count) {
    const spec = model.spec;
    const view = ctx.view;
    const rng = ctx.rng;
    const mid = (spec.depth[0] + spec.depth[1]) / 2;
    const bounds = { x: [0, 0], y: [0, 0], z: [0, 0] };
    fillBounds(bounds, view, spec.depth);
    const school = createSchool({ count, rng, bounds, speed: spec.speed, size: [0.85, 1.15] });
    const { meshes, inst } = createInstanced(ctx, model, count);
    const rate = STROKE_RATE[spec.shader] || 1;
    const amp = strokeAmplitude(spec);
    const freq = new Float32Array(count);
    const beat = new Float32Array(count);
    for (let i = 0; i < count; i++) {
        freq[i] = between(rng, spec.swim.freq) * rate;
        beat[i] = rng();
        inst.setXYZW(i, beat[i], 0, amp, 1);
    }
    const repulsors = [];
    const pointerRepulsor = { x: 0, y: 0, z: mid, radius: 1.6 + 2 * spec.length, strength: 5 };
    const env = { laneHalfWidth: view.laneHalfWidth, laneStrength: 1, repulsors, speedMul: 1, cohesionMul: 1 };
    const matrix = new THREE.Matrix4();

    function update(t, dt) {
        env.laneStrength = view.laneStrength(mid);
        env.speedMul = 1 + 0.5 * ctx.busy;
        env.cohesionMul = 1 + 0.3 * ctx.busy;
        repulsors.length = 0;
        if (ctx.pointer.active) {
            pointerRepulsor.x = ctx.pointer.nx * -mid;
            pointerRepulsor.y = ctx.pointer.ny * -mid;
            repulsors.push(pointerRepulsor);
        }
        for (let r = 0; r < ctx.repulsors.length; r++) repulsors.push(ctx.repulsors[r]);
        school.step(dt, env);
        const pos = school.pos;
        const vel = school.vel;
        for (let i = 0; i < count; i++) {
            const o = i * 3;
            composeHeading(matrix, pos[o], pos[o + 1], pos[o + 2], pos[o] + vel[o], pos[o + 1] + vel[o + 1], pos[o + 2] + vel[o + 2], school.size[i], 0);
            for (let m = 0; m < meshes.length; m++) meshes[m].setMatrixAt(i, matrix);
            // the tail beat follows the swimming speed, so startled fish flutter faster
            const speed = Math.hypot(vel[o], vel[o + 1], vel[o + 2]);
            beat[i] = (beat[i] + dt * freq[i] * clamp(speed / school.cruise[i], 0.7, 2.4) / TAU) % 1;
            inst.setXYZW(i, beat[i], 0, amp, 1);
        }
        flagMatrices(meshes);
        inst.needsUpdate = true;
    }

    return { id: spec.id, count, update, refresh() { fillBounds(bounds, view, spec.depth); } };
}

// Cruisers (sharks, turtle, manta): one animal on a slow Lissajous loop inside its depth band.
export function createCruiserActor(ctx, model) {
    const spec = model.spec;
    const view = ctx.view;
    const rng = ctx.rng;
    const zc = (spec.depth[0] + spec.depth[1]) / 2;
    const halfBand = Math.abs(spec.depth[0] - spec.depth[1]) / 2;
    const speed = between(rng, spec.speed);
    const phi1 = rng() * TAU;
    const phi2 = rng() * TAU;
    const phi3 = rng() * TAU;
    const freq = between(rng, spec.swim.freq) * (STROKE_RATE[spec.shader] || 1);
    const { meshes, inst } = createInstanced(ctx, model, 1, { flap: freq });
    const amp = strokeAmplitude(spec);
    inst.setXYZW(0, rng(), freq, amp, 1);

    let reachX = 1;
    let reachY = 1;
    function refresh() {
        reachX = 0.55 * view.frustumWidth(zc);
        reachY = 0.3 * view.frustumHeight(zc);
    }
    refresh();

    function pathAt(theta, out) {
        return out.set(
            reachX * Math.sin(theta + phi1),
            reachY * Math.sin(0.6 * theta + phi2),
            zc + halfBand * Math.sin(0.4 * theta + phi3)
        );
    }

    let theta = 0;
    const here = new THREE.Vector3();
    const ahead = new THREE.Vector3();
    const tangent = new THREE.Vector3();
    const heading = pathAt(0.1, new THREE.Vector3()).sub(pathAt(0, new THREE.Vector3())).normalize();
    let yaw = Math.atan2(heading.x, heading.z);
    let bank = 0;
    let fade = 1;
    const matrix = new THREE.Matrix4();
    const repulsor = spec.shader === 'shark' ? { x: 0, y: 0, z: zc, radius: 2.5, strength: 3 } : null;
    const actor = { id: spec.id, count: 1, update, refresh, repulsor };

    // Dive (species.dive): while the animal is, or within DIVE_LOOKAHEAD seconds will be, closer than DIVE_MARGIN to the
    // reading lane it eases to the deep end of its band over DIVE_EASE_SECONDS, and back to its path afterwards. x and y
    // scale with the depth, so the animal keeps its place in the picture and only sinks into the fog (smaller, bluer, fainter).
    const dives = !!spec.dive;
    const deepZ = Math.min(spec.depth[0], spec.depth[1]);
    let diveRaw = 0;
    let divePrimed = false;

    function sink(point, k) {
        const z = point.z + (deepZ - point.z) * k;
        const f = z / point.z;
        point.set(point.x * f, point.y * f, z);
    }

    function applyDive(dt) {
        const vx = (ahead.x - here.x) * 10 * speed / reachX; // m/s along x; ahead is 0.1 rad further down the path
        const edge = view.laneHalfWidth(here.z) + DIVE_MARGIN;
        const inside = Math.abs(here.x) < edge || Math.abs(here.x + vx * DIVE_LOOKAHEAD) < edge;
        if (!divePrimed) {
            diveRaw = inside ? 1 : 0;
            divePrimed = true;
        } else {
            diveRaw = clamp(diveRaw + (inside ? dt : -dt) / DIVE_EASE_SECONDS, 0, 1);
        }
        const k = smoothstep(0, 1, diveRaw);
        sink(here, k);
        sink(ahead, k);
    }

    function update(t, dt) {
        theta += dt * speed / reachX;
        pathAt(theta, here);
        pathAt(theta + 0.1, ahead);
        if (dives) applyDive(dt);
        tangent.subVectors(ahead, here);
        if (tangent.lengthSq() > 1e-12) {
            tangent.normalize();
            // limited turn rate: the path can swing by 180 degrees near the screen edges
            const angle = Math.acos(clamp(heading.dot(tangent), -1, 1));
            const maxAngle = CRUISER_TURN_RATE * dt;
            if (angle <= maxAngle) {
                heading.copy(tangent);
            } else {
                _axis.crossVectors(heading, tangent);
                if (_axis.lengthSq() < 1e-10) _axis.copy(_up); else _axis.normalize();
                heading.applyAxisAngle(_axis, maxAngle).normalize();
            }
        }
        if (dt > 0) {
            const nextYaw = Math.atan2(heading.x, heading.z);
            let delta = nextYaw - yaw;
            if (delta > Math.PI) delta -= TAU; else if (delta < -Math.PI) delta += TAU;
            yaw = nextYaw;
            // bank into the turn (turning toward +X is a left turn, left side down)
            const target = clamp(-0.5 * delta / dt, -0.5, 0.5);
            bank += (target - bank) * Math.min(1, dt * 3);
        }
        composeHeading(matrix, here.x, here.y, here.z, here.x + heading.x, here.y + heading.y, here.z + heading.z, 1, bank);
        for (let m = 0; m < meshes.length; m++) meshes[m].setMatrixAt(0, matrix);
        flagMatrices(meshes);

        // calm down while swimming behind the text
        const lane = smoothstep(view.laneHalfWidth(here.z) + 0.8, view.laneHalfWidth(here.z) - 0.8, Math.abs(here.x)) * view.laneStrength(here.z);
        fade += ((1 - 0.55 * lane) - fade) * Math.min(1, dt * 2);
        inst.setW(0, fade);
        inst.needsUpdate = true;
        if (repulsor) {
            repulsor.x = here.x;
            repulsor.y = here.y;
            repulsor.z = here.z;
        }
    }
    return actor;
}

function contraction(c) {
    if (c < 0.22) {
        const x = c / 0.22;
        return x * x * (3 - 2 * x);
    }
    const x = (c - 0.22) / 0.78;
    return 1 - x * x * (3 - 2 * x);
}

// Drifters (moon jellies): pulse upward along the sides and respawn below the frame.
export function createDrifterActor(ctx, model, count) {
    const spec = model.spec;
    const view = ctx.view;
    const rng = ctx.rng;
    const { meshes, inst, tint } = createInstanced(ctx, model, count, { tint: true });
    const jellies = [];
    const matrix = new THREE.Matrix4();
    const quaternion = new THREE.Quaternion();
    const euler = new THREE.Euler();
    const scaleVec = new THREE.Vector3();

    function sideRange(z) {
        const lo = view.laneHalfWidth(z) + 0.5;
        return [lo, Math.max(lo + 0.3, 0.5 * view.frustumWidth(z) + 0.3)];
    }

    function place(j, initial) {
        j.z = between(rng, spec.depth);
        const range = sideRange(j.z);
        const halfH = 0.5 * view.frustumHeight(j.z);
        j.side = rng() < 0.5 ? -1 : 1;
        j.pos.set(j.side * (range[0] + rng() * (range[1] - range[0])),
            initial ? -halfH - 1 + rng() * (2 * halfH + 2) : -halfH - 1, j.z);
    }

    for (let i = 0; i < count; i++) {
        const j = {
            pos: new THREE.Vector3(),
            z: 0,
            side: 1,
            scale: 1.0 + rng(),
            phase: rng(),
            pulse: rng(),
            freq: between(rng, spec.swim.freq),
            base: between(rng, spec.speed),
            tiltPhase: rng() * TAU,
            drift: rng() * TAU,
            speed: 0
        };
        j.speed = j.base;
        place(j, true);
        tint.setXYZ(i, 0.8 + rng() * 0.4, 0.85 + rng() * 0.3, 1.0);
        jellies.push(j);
    }
    tint.needsUpdate = true;

    function update(t, dt) {
        const step = Math.min(dt, 0.1);
        for (let i = 0; i < count; i++) {
            const j = jellies[i];
            // the pulse cycle is accumulated here (not uTime * freq) so the busy speed-up never jumps
            j.pulse = (j.pulse + step * j.freq * (1 + 0.4 * ctx.busy)) % 1;
            const cycle = (j.pulse + j.phase) % 1;
            const k = contraction(cycle);
            const targetSpeed = j.base * (0.5 + 2.4 * k) * (0.7 + j.scale * 0.3);
            j.speed += (targetSpeed - j.speed) * Math.min(1, step * 2.5);
            const tiltX = Math.sin(t * 0.07 + j.tiltPhase) * 0.28;
            const tiltZ = Math.cos(t * 0.05 + j.tiltPhase * 1.3) * 0.28;
            euler.set(tiltX, j.tiltPhase + t * 0.03, tiltZ);
            quaternion.setFromEuler(euler);
            j.pos.x += (-Math.sin(tiltZ) * j.speed + Math.sin(t * 0.05 + j.drift) * 0.05) * step;
            j.pos.y += j.speed * step;

            const range = sideRange(j.z);
            const halfH = 0.5 * view.frustumHeight(j.z);
            const sx = j.side * j.pos.x;
            const wanted = clamp(sx, range[0], range[1]);
            j.pos.x = j.side * (sx + (wanted - sx) * Math.min(1, step * 2));
            if (j.pos.y > halfH + 1) place(j, false);

            const fade = clamp((halfH + 1 - j.pos.y) / 1.5, 0, 1) * clamp((j.pos.y + halfH + 1) / 1.5, 0, 1);
            scaleVec.setScalar(j.scale);
            matrix.compose(j.pos, quaternion, scaleVec);
            for (let m = 0; m < meshes.length; m++) meshes[m].setMatrixAt(i, matrix);
            inst.setXYZW(i, cycle, 0, clamp(j.speed / (j.base * 2.8), 0, 1), fade);
        }
        flagMatrices(meshes);
        inst.needsUpdate = true;
    }

    return { id: spec.id, count, update, refresh() {} };
}

// The rare whale: one slow crossing far back every few minutes, a shadow in the fog.
export function createWhaleActor(ctx, model) {
    const spec = model.spec;
    const view = ctx.view;
    const rng = ctx.rng;
    const z = spec.depth[0];
    const { meshes, inst } = createInstanced(ctx, model, 1);
    const freq = spec.swim.freq[0] * (STROKE_RATE.whale);
    const amp = strokeAmplitude(spec);
    const matrix = new THREE.Matrix4();
    // debug seam: window.__auraOceanLifeDebug.firstWhaleDelay (seconds, bridge -> env -> ctx) replaces the 60-120 s wait of the first pass
    const randomDelay = 60 + rng() * 60;
    const debugDelay = ctx.firstWhaleDelay;
    let nextPass = typeof debugDelay === 'number' && isFinite(debugDelay) && debugDelay > 0 ? debugDelay : randomDelay;
    let direction = rng() < 0.5 ? -1 : 1;
    let pass = null;
    // nextPass is the scheduled start (scene seconds) of the next pass; while one is running it is already the one after it
    const actor = { id: spec.id, count: 1, visible: false, update, refresh() {}, get nextPass() { return nextPass; } };

    function setVisible(on) {
        actor.visible = on;
        for (let m = 0; m < meshes.length; m++) meshes[m].visible = on;
    }
    setVisible(false);

    function update(t) {
        if (!pass && t >= nextPass) {
            pass = { start: t, dir: direction };
            direction = -direction;
            nextPass = t + WHALE_PASS_SECONDS + 180 + rng() * 120;
            setVisible(true);
        }
        if (!pass) return;
        const u = (t - pass.start) / WHALE_PASS_SECONDS;
        if (u >= 1) {
            pass = null;
            setVisible(false);
            return;
        }
        const reach = 0.7 * view.frustumWidth(z);
        const x = pass.dir * (-reach + 2 * reach * u);
        const y = -2 + Math.sin(t * 0.2) * 0.8;
        composeHeading(matrix, x, y, z, x + pass.dir, y + Math.cos(t * 0.2) * 0.08, z, 1, Math.sin(t * 0.15) * 0.05);
        for (let m = 0; m < meshes.length; m++) meshes[m].setMatrixAt(0, matrix);
        flagMatrices(meshes);
        const fade = Math.min(1, (t - pass.start) / 3, (pass.start + WHALE_PASS_SECONDS - t) / 3);
        inst.setXYZW(0, 0, freq, amp, Math.max(0, fade));
        inst.needsUpdate = true;
    }
    return actor;
}
