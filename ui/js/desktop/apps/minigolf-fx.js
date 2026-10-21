import * as THREE from 'three';
import { TIERS } from './minigolf-looks.js';

const POINT_VERT = `attribute float golfSize; attribute float golfAlpha; attribute vec3 golfColor; uniform float golfScale;
varying vec3 vGolfColor; varying float vGolfAlpha;
void main() {
    vGolfColor = golfColor; vGolfAlpha = golfAlpha;
    vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
    gl_PointSize = golfSize * golfScale / max(0.1, -mvPosition.z);
    gl_Position = projectionMatrix * mvPosition;
}`;
const POINT_FRAG = `varying vec3 vGolfColor; varying float vGolfAlpha;
void main() {
    vec2 c = gl_PointCoord - 0.5; float d = dot(c, c);
    if (d > 0.25) discard;
    gl_FragColor = vec4(vGolfColor, vGolfAlpha * smoothstep(0.25, 0.04, d));
    #include <colorspace_fragment>
}`;
const TRAIL_POINTS = 28;
const tint = new THREE.Color();
const between = (a, b) => a + Math.random() * (b - a);

function createPool(max, blending, scale) {
    const geometry = new THREE.BufferGeometry();
    const attribute = (size) => new THREE.BufferAttribute(new Float32Array(max * size), size).setUsage(THREE.DynamicDrawUsage);
    const position = attribute(3), color = attribute(3), size = attribute(1), alpha = attribute(1);
    geometry.setAttribute('position', position); geometry.setAttribute('golfColor', color); geometry.setAttribute('golfSize', size); geometry.setAttribute('golfAlpha', alpha);
    geometry.setDrawRange(0, 0);
    const material = new THREE.ShaderMaterial({ uniforms: { golfScale: scale }, vertexShader: POINT_VERT, fragmentShader: POINT_FRAG, transparent: true, depthWrite: false, blending });
    const points = new THREE.Points(geometry, material);
    points.frustumCulled = false; points.renderOrder = 4;
    const velocity = new Float32Array(max * 3), life = new Float32Array(max), span = new Float32Array(max), base = new Float32Array(max);
    const grow = new Float32Array(max), gravity = new Float32Array(max), drag = new Float32Array(max), opacity = new Float32Array(max);
    let count = 0, limit = max;
    function move(from, to) {
        position.setXYZ(to, position.getX(from), position.getY(from), position.getZ(from));
        color.setXYZ(to, color.getX(from), color.getY(from), color.getZ(from));
        for (let a = 0; a < 3; a++) velocity[to * 3 + a] = velocity[from * 3 + a];
        life[to] = life[from]; span[to] = span[from]; base[to] = base[from]; grow[to] = grow[from]; gravity[to] = gravity[from]; drag[to] = drag[from]; opacity[to] = opacity[from];
    }
    return {
        points,
        get count() { return count; },
        setLimit(value) { limit = Math.max(0, Math.min(max, value)); count = Math.min(count, limit); },
        clear() { count = 0; geometry.setDrawRange(0, 0); },
        emit(x, y, z, vx, vy, vz, hex, sizeValue, lifeValue, g = 0, d = 0, growValue = 0, opacityValue = 1) {
            if (count >= limit) return false;
            const i = count++;
            tint.set(hex);
            position.setXYZ(i, x, y, z); color.setXYZ(i, tint.r, tint.g, tint.b);
            velocity[i * 3] = vx; velocity[i * 3 + 1] = vy; velocity[i * 3 + 2] = vz;
            base[i] = sizeValue; life[i] = span[i] = lifeValue; gravity[i] = g; drag[i] = d; grow[i] = growValue; opacity[i] = opacityValue;
            return true;
        },
        update(dt) {
            for (let i = 0; i < count; i++) {
                life[i] -= dt;
                if (life[i] <= 0) { move(--count, i); i--; continue; }
                const keep = Math.max(0, 1 - drag[i] * dt);
                velocity[i * 3] *= keep; velocity[i * 3 + 1] = velocity[i * 3 + 1] * keep - gravity[i] * dt; velocity[i * 3 + 2] *= keep;
                position.setXYZ(i, position.getX(i) + velocity[i * 3] * dt, position.getY(i) + velocity[i * 3 + 1] * dt, position.getZ(i) + velocity[i * 3 + 2] * dt);
                const t = life[i] / span[i];
                size.setX(i, base[i] * (1 + grow[i] * (1 - t)));
                alpha.setX(i, opacity[i] * Math.min(1, t * 3));
            }
            geometry.setDrawRange(0, count);
            position.needsUpdate = true; color.needsUpdate = true; size.needsUpdate = true; alpha.needsUpdate = true;
        },
        dispose() { geometry.dispose(); material.dispose(); }
    };
}

function createConfetti(max) {
    const mesh = new THREE.InstancedMesh(new THREE.PlaneGeometry(.075, .045), new THREE.MeshBasicMaterial({ side: THREE.DoubleSide, transparent: true, depthWrite: false }), max);
    mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage); mesh.frustumCulled = false; mesh.count = 0; mesh.renderOrder = 4;
    mesh.setColorAt(0, tint.set(0xffffff));
    const pos = new Float32Array(max * 3), vel = new Float32Array(max * 3), axis = new Float32Array(max * 3), angle = new Float32Array(max), spin = new Float32Array(max), life = new Float32Array(max), phase = new Float32Array(max);
    const colors = new Array(max).fill(0xffffff), matrix = new THREE.Matrix4(), quaternion = new THREE.Quaternion(), vector = new THREE.Vector3(), scale = new THREE.Vector3(), turn = new THREE.Vector3();
    let count = 0, limit = max;
    return {
        mesh,
        get count() { return count; },
        setLimit(value) { limit = Math.max(0, Math.min(max, value)); count = Math.min(count, limit); },
        clear() { count = 0; mesh.count = 0; },
        emit(x, y, z, vx, vy, vz, hex) {
            if (count >= limit) return;
            const i = count++, o = i * 3;
            pos[o] = x; pos[o + 1] = y; pos[o + 2] = z; vel[o] = vx; vel[o + 1] = vy; vel[o + 2] = vz;
            turn.set(between(-1, 1), between(-1, 1), between(-1, 1)).normalize(); axis[o] = turn.x; axis[o + 1] = turn.y; axis[o + 2] = turn.z;
            angle[i] = between(0, Math.PI * 2); spin[i] = between(4, 11); life[i] = between(2.2, 3.2); phase[i] = between(0, 6.28); colors[i] = hex;
        },
        update(dt, elapsed) {
            for (let i = 0; i < count; i++) {
                life[i] -= dt;
                if (life[i] <= 0) {
                    const last = --count;
                    for (let a = 0; a < 3; a++) { pos[i * 3 + a] = pos[last * 3 + a]; vel[i * 3 + a] = vel[last * 3 + a]; axis[i * 3 + a] = axis[last * 3 + a]; }
                    angle[i] = angle[last]; spin[i] = spin[last]; life[i] = life[last]; phase[i] = phase[last]; colors[i] = colors[last]; i--; continue;
                }
                const keep = Math.max(0, 1 - 1.6 * dt);
                vel[i * 3] = vel[i * 3] * keep + Math.sin(elapsed * 7 + phase[i]) * .6 * dt;
                vel[i * 3 + 1] = vel[i * 3 + 1] * keep - 2.4 * dt;
                vel[i * 3 + 2] = vel[i * 3 + 2] * keep + Math.cos(elapsed * 6 + phase[i]) * .6 * dt;
                for (let a = 0; a < 3; a++) pos[i * 3 + a] += vel[i * 3 + a] * dt;
                angle[i] += spin[i] * dt;
                quaternion.setFromAxisAngle(vector.fromArray(axis, i * 3), angle[i]);
                const fade = Math.min(1, life[i] * 2);
                matrix.compose(vector.fromArray(pos, i * 3), quaternion, scale.setScalar(fade));
                mesh.setMatrixAt(i, matrix); mesh.setColorAt(i, tint.set(colors[i]));
            }
            mesh.count = count;
            mesh.instanceMatrix.needsUpdate = true;
            if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
        },
        dispose() { mesh.geometry.dispose(); mesh.material.dispose(); mesh.dispose(); }
    };
}

function createRing() {
    const mesh = new THREE.Mesh(new THREE.RingGeometry(.86, 1, 64).rotateX(-Math.PI / 2), new THREE.MeshBasicMaterial({ transparent: true, opacity: 0, depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide }));
    mesh.visible = false; mesh.renderOrder = 3;
    let time = 0, duration = 0, size = 1, still = false;
    return {
        mesh,
        play(x, y, z, hex, reduced, maxSize = 1.6, length = .8) {
            mesh.position.set(x, y, z); mesh.material.color.set(hex); mesh.visible = true;
            time = 0; duration = reduced ? 1 : length; size = maxSize; still = reduced;
        },
        update(dt) {
            if (!mesh.visible) return;
            time += dt;
            const t = Math.min(1, time / duration);
            mesh.scale.setScalar(still ? size * .6 : Math.max(.01, size * t));
            mesh.material.opacity = still ? Math.sin(Math.PI * t) * .8 : (1 - t) * .9;
            if (t >= 1) mesh.visible = false;
        },
        dispose() { mesh.geometry.dispose(); mesh.material.dispose(); }
    };
}

function createTrail() {
    const geometry = new THREE.BufferGeometry();
    const position = new THREE.BufferAttribute(new Float32Array(TRAIL_POINTS * 6), 3).setUsage(THREE.DynamicDrawUsage);
    const fade = new THREE.BufferAttribute(new Float32Array(TRAIL_POINTS * 2), 1).setUsage(THREE.DynamicDrawUsage);
    const index = [];
    for (let i = 0; i < TRAIL_POINTS - 1; i++) { const a = i * 2; index.push(a, a + 1, a + 2, a + 1, a + 3, a + 2); }
    geometry.setIndex(index); geometry.setAttribute('position', position); geometry.setAttribute('golfFade', fade); geometry.setDrawRange(0, 0);
    const material = new THREE.ShaderMaterial({
        uniforms: { golfColor: { value: new THREE.Color() } },
        vertexShader: 'attribute float golfFade; varying float vGolfFade; void main(){ vGolfFade = golfFade; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }',
        fragmentShader: 'uniform vec3 golfColor; varying float vGolfFade; void main(){ gl_FragColor = vec4(golfColor * 1.4, vGolfFade * 0.75);\n#include <colorspace_fragment>\n}',
        transparent: true, depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide
    });
    const mesh = new THREE.Mesh(geometry, material);
    mesh.frustumCulled = false; mesh.renderOrder = 3;
    const points = new Float32Array(TRAIL_POINTS * 3);
    let count = 0, shrink = 0, shown = null;
    function reset() { count = 0; geometry.setDrawRange(0, 0); }
    return {
        mesh,
        get count() { return count; },
        reset,
        update(dt, ball, moving, speed, hex, enabled) {
            if (!enabled) { if (count) reset(); return; }
            // The colour often arrives as a CSS string; parse it only when it changes (a Color object may mutate in place).
            if (hex !== shown || typeof hex === 'object') { material.uniforms.golfColor.value.set(hex); shown = hex; }
            if (moving && speed > .6) {
                shrink = 0;
                if (!count || Math.hypot(ball.x - points[0], ball.z - points[2]) > .045) {
                    points.copyWithin(3, 0, (TRAIL_POINTS - 1) * 3);
                    points[0] = ball.x; points[1] = ball.y - .1; points[2] = ball.z;
                    count = Math.min(TRAIL_POINTS, count + 1);
                }
            } else {
                shrink += dt;
                while (shrink > 1 / 40 && count) { shrink -= 1 / 40; count--; }
            }
            for (let i = 0; i < count; i++) {
                const a = Math.max(0, i - 1), b = Math.min(count - 1, i + 1);
                const dx = points[a * 3] - points[b * 3], dz = points[a * 3 + 2] - points[b * 3 + 2], length = Math.hypot(dx, dz) || 1;
                const k = 1 - i / TRAIL_POINTS, width = .065 * k, px = -dz / length * width, pz = dx / length * width;
                position.setXYZ(i * 2, points[i * 3] + px, points[i * 3 + 1], points[i * 3 + 2] + pz);
                position.setXYZ(i * 2 + 1, points[i * 3] - px, points[i * 3 + 1], points[i * 3 + 2] - pz);
                fade.setX(i * 2, k * k); fade.setX(i * 2 + 1, k * k);
            }
            geometry.setDrawRange(0, Math.max(0, (count - 1) * 6));
            position.needsUpdate = true; fade.needsUpdate = true;
        },
        dispose() { geometry.dispose(); material.dispose(); }
    };
}

// Particles, trail, cup celebration and win fireworks for one scene.
export function createFx(parent, initialLook) {
    let look = initialLook, cfg = TIERS.medium, reduced = false, flagBoost = 0, ambientAt = 0, sandAt = 0, elapsed = 0;
    const rockets = [], scale = { value: 300 };
    const soft = createPool(320, THREE.NormalBlending, scale), glow = createPool(320, THREE.AdditiveBlending, scale);
    const confetti = createConfetti(160), ring = createRing(), ripple = createRing(), trail = createTrail();
    parent.add(soft.points, glow.points, confetti.mesh, ring.mesh, ripple.mesh, trail.mesh);
    function setTier(next) { cfg = next; soft.setLimit(next.particles); glow.setLimit(next.particles); confetti.setLimit(Math.round(next.particles / 2)); }
    function clear() { soft.clear(); glow.clear(); confetti.clear(); trail.reset(); rockets.length = 0; flagBoost = 0; ring.mesh.visible = false; ripple.mesh.visible = false; }
    setTier(cfg);
    function spray(pool, count, x, y, z, hex, o) {
        for (let i = 0; i < count; i++) {
            const angle = Math.random() * Math.PI * 2, out = between(o.out[0], o.out[1]);
            pool.emit(x, y, z, Math.cos(angle) * out, between(o.up[0], o.up[1]), Math.sin(angle) * out, Array.isArray(hex) ? hex[i % hex.length] : hex,
                between(o.size[0], o.size[1]), between(o.life[0], o.life[1]), o.gravity ?? 0, o.drag ?? 0, o.grow ?? 0, o.opacity ?? 1);
        }
    }
    function confettiBurst(x, y, z, count) {
        for (let i = 0; i < count; i++) {
            const angle = Math.random() * Math.PI * 2, out = between(.6, 2.2);
            confetti.emit(x, y, z, Math.cos(angle) * out, between(2.5, 5), Math.sin(angle) * out, look.fx.confetti[i % look.fx.confetti.length]);
        }
    }
    function burst(type, position, speed = 1, info = {}) {
        if (!position) return;
        const [x, y, z] = position, fx = look.fx, ground = y - .15, space = look.water.mode === 'void';
        if (type === 'cup') flagBoost = 2;
        if (type === 'cup' || type === 'win') ring.play(x, y + .012, z, fx.gold, reduced);
        if (reduced) return;
        if (type === 'strike') {
            if (space) return spray(soft, 10, x, ground, z, fx.sand, { up: [.3, .8], out: [.4, 1.1], size: [.12, .22], life: [.6, 1], gravity: .2, drag: 2.5, grow: 1.2, opacity: .6 });
            if (info.surface === 'sand') return spray(soft, 16, x, ground, z, fx.sand, { up: [.5, 1.2], out: [.5, 1.4], size: [.1, .2], life: [.6, 1.1], gravity: 2, drag: 2, grow: 1, opacity: .75 });
            return spray(soft, 8, x, ground, z, fx.grass, { up: [1, 2], out: [.4, 1], size: [.04, .07], life: [.4, .7], gravity: 7, drag: .5 });
        }
        if (type === 'impact') {
            const count = Math.round(4 + Math.min(1, speed) * 10);
            if (fx.wall === 'sparks') return spray(glow, count, x, y, z, 0xfff1b0, { up: [.6, 2], out: [1.5, 3.5], size: [.035, .06], life: [.25, .5], gravity: 5, drag: 1 });
            if (fx.wall === 'dust') return spray(soft, count, x, y, z, fx.stone, { up: [.1, .5], out: [.3, .8], size: [.1, .18], life: [.5, .9], gravity: -.15, drag: 3, grow: 1, opacity: .55 });
            return spray(soft, count, x, y, z, fx.wood, { up: [.8, 1.8], out: [.6, 1.6], size: [.035, .06], life: [.4, .7], gravity: 7, drag: .6 });
        }
        if (type === 'water') {
            if (space) return spray(soft, 26, x, y, z, fx.splash, { up: [-.2, .4], out: [.3, 1], size: [.16, .3], life: [.8, 1.4], gravity: 0, drag: 1.5, grow: 1.5, opacity: .5 });
            ripple.play(x, look.water.level + .02, z, 0xffffff, false, .9, .6);
            return spray(soft, 26, x, look.water.level + .02, z, fx.splash, { up: [2, 4.5], out: [.4, 1.4], size: [.06, .12], life: [.6, 1], gravity: 9.8, drag: .2 });
        }
        if (type === 'cup') {
            confettiBurst(x, y + .1, z, Math.round(cfg.particles * .45));
            return spray(glow, 24, x, y, z, fx.gold, { up: [1.2, 3], out: [.05, .3], size: [.05, .09], life: [.6, 1.1], gravity: -.3, drag: .4 });
        }
        if (type === 'win') {
            for (const [dx, dz] of [[-1.2, .4], [1.1, -.3], [0, 1.2]]) confettiBurst(x + dx, y + .2, z + dz, Math.round(cfg.particles * .2));
            [0, .35, .7].forEach((delay, i) => rockets.push({ delay, time: 0, x: x + (i - 1) * 1.6, y, z: z - 1 + i * .5, hex: fx.confetti[i % fx.confetti.length], done: false }));
        }
    }
    function ambient(dt, ctx) {
        ambientAt += dt;
        if (ambientAt < .25) return;
        ambientAt = 0;
        const angle = Math.random() * Math.PI * 2, fx = look.fx;
        if (fx.ambient === 'glint') {
            const r = between(5.4, 10);
            glow.emit(Math.cos(angle) * r, ctx.waterLevel + .05, Math.sin(angle) * r, 0, 0, 0, fx.ambientColor, .06, .6);
        } else if (fx.ambient === 'fireflies') {
            const r = between(0, 4.5);
            glow.emit(Math.cos(angle) * r, ctx.groundY + between(.3, 1.8), Math.sin(angle) * r, between(-.1, .1), between(-.05, .1), between(-.1, .1), fx.ambientColor, .05, 2.5);
        } else {
            const r = between(0, 6);
            soft.emit(Math.cos(angle) * r, ctx.groundY + between(-.5, 2), Math.sin(angle) * r, between(-.05, .05), between(0, .05), between(-.05, .05), fx.ambientColor, .12, 3, 0, 0, 0, .25);
        }
    }
    return {
        burst,
        get flagBoost() { return flagBoost; },
        setLook(next) { look = next; clear(); },
        setTier,
        setReduced(value) { reduced = !!value; if (reduced) { soft.clear(); glow.clear(); confetti.clear(); rockets.length = 0; } },
        setScale(height, fovDegrees) { scale.value = height / (2 * Math.tan(fovDegrees * Math.PI / 360)); },
        clear,
        // ctx: { ball: Vector3, moving, speed, color, surface, waterLevel, groundY }
        update(dt, ctx) {
            elapsed += dt;
            flagBoost = Math.max(0, flagBoost - dt);
            if (!reduced && cfg.motion) {
                ambient(dt, ctx);
                if (ctx.moving && ctx.surface === 'sand' && ctx.speed > .8 && (sandAt += dt) > 1 / 30) {
                    sandAt = 0;
                    soft.emit(ctx.ball.x, ctx.ball.y - .12, ctx.ball.z, between(-.2, .2), between(.2, .5), between(-.2, .2), look.fx.sand, between(.08, .14), between(.4, .7), 1, 2, 1, .6);
                }
            }
            for (const rocket of rockets) {
                if (rocket.done || (rocket.time += dt) < rocket.delay) continue;
                const t = rocket.time - rocket.delay;
                if (t < .7) glow.emit(rocket.x, rocket.y + 3 * t / .7, rocket.z, 0, 0, 0, 0xfff1d0, .06, .25);
                else {
                    rocket.done = true;
                    for (let i = 0; i < 50; i++) {
                        const u = Math.random() * 2 - 1, theta = Math.random() * Math.PI * 2, r = Math.sqrt(1 - u * u), speed = between(2, 3.5);
                        glow.emit(rocket.x, rocket.y + 3, rocket.z, Math.cos(theta) * r * speed, u * speed, Math.sin(theta) * r * speed, rocket.hex, between(.05, .08), between(1.2, 1.6), 1.5, .6);
                    }
                }
            }
            for (let i = rockets.length - 1; i >= 0; i--) if (rockets[i].done) rockets.splice(i, 1);
            soft.update(dt); glow.update(dt); confetti.update(dt, elapsed); ring.update(dt); ripple.update(dt);
            trail.update(dt, ctx.ball, ctx.moving, ctx.speed, ctx.color, cfg.trail);
        },
        stats() { return { particles: soft.count + glow.count + confetti.count, trail: trail.count }; },
        dispose() { soft.dispose(); glow.dispose(); confetti.dispose(); ring.dispose(); ripple.dispose(); trail.dispose(); for (const item of [soft.points, glow.points, confetti.mesh, ring.mesh, ripple.mesh, trail.mesh]) item.removeFromParent(); }
    };
}
