import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { SPECIES } from '../../chat/ocean-life-species.js';
import { createSchool, createRng } from '../../chat/ocean-life-boids.js';
import { MATERIAL_KIND, DOUBLE_SIDED } from '../../chat/ocean-life-shaders.js';
import * as P from './minigolf-sealife-paths.js';
import { createPirateSeaMaterials } from './minigolf-sealife-shaders.js';

// Animated sea life around the Pirate Bay island. Reuses the light system-world fish/jelly GLBs,
// the Ocean chat sharks/turtle/whale GLBs, and the abyss manta GLB, driven by Ocean chat GLSL shaders.
export const FOLDERS = Object.freeze({
    fish: 'system-world/sea-life/v1/',
    chat: 'chat-ocean-life/v1/',
    abyss: 'screensaver/abyss/v1/'
});

export const MODELS = Object.freeze([
    { id: 'fish_sardine', folder: 'fish', file: 'fish-sardine.lod1.glb' },
    { id: 'fish_tang', folder: 'fish', file: 'fish-tang.lod1.glb' },
    { id: 'fish_clown', folder: 'fish', file: 'fish-clown.lod1.glb' },
    { id: 'fish_butterfly', folder: 'fish', file: 'fish-butterfly.lod1.glb' },
    { id: 'shark_reef', folder: 'chat', file: 'shark-reef.glb' },
    { id: 'shark_hammer', folder: 'chat', file: 'shark-hammer.glb' },
    { id: 'turtle', folder: 'chat', file: 'turtle.glb' },
    { id: 'whale', folder: 'chat', file: 'whale.glb' },
    { id: 'manta', folder: 'abyss', file: 'manta.glb', length: 2.1, fixup: g => g.rotateY(Math.PI) },
    { id: 'jelly_moon', folder: 'fish', file: 'jelly-moon.lod1.glb' }
]);

const BIG = Object.freeze([
    { kind: 'shark', id: 'shark_reef', rate: 1.6 },
    { kind: 'hammer', id: 'shark_hammer', rate: 1.6 },
    { kind: 'turtle', id: 'turtle', rate: 2 },
    { kind: 'manta', id: 'manta', rate: 1 },
    { kind: 'whale', id: 'whale', rate: 3 },
    { kind: 'jelly', id: 'jelly_moon', rate: 1 }
]);

const IDENTITY = new THREE.Matrix4();
const ZERO = new Float32Array(16);
const MAX_RINGS = 48;

export function prepareModel(root, spec, fixup) {
    root.updateMatrixWorld(true);
    const parts = [];
    root.traverse(o => {
        if (!o.isMesh) return;
        const g = o.geometry;
        if (!o.matrixWorld.equals(IDENTITY)) g.applyMatrix4(o.matrixWorld);
        parts.push({ name: o.material?.name || 'body', geometry: g });
        o.material?.dispose?.();
    });
    const drop = () => parts.forEach(p => p.geometry.dispose());
    if (!parts.length) throw Error(spec.id + ' has no meshes');
    for (const p of parts) {
        for (const a of ['position', 'normal', 'uv', 'uv1']) {
            if (!p.geometry.getAttribute(a)) {
                drop();
                throw Error(spec.id + ' primitive "' + p.name + '" lacks ' + a);
            }
        }
    }
    if (fixup) parts.forEach(p => fixup(p.geometry));
    const box = new THREE.Box3();
    parts.forEach(p => {
        p.geometry.computeBoundingBox();
        box.union(p.geometry.boundingBox);
    });
    const depth = box.max.z - box.min.z;
    if (!(depth > 1e-6)) {
        drop();
        throw Error(spec.id + ' has no depth');
    }
    const scale = spec.length / depth;
    let radius = 0;
    parts.forEach(p => {
        p.geometry.scale(scale, scale, scale);
        p.geometry.computeBoundingSphere();
        radius = Math.max(radius, p.geometry.boundingSphere.radius);
        p.geometry.setAttribute('aAnim', p.geometry.getAttribute('uv1'));
        p.geometry.deleteAttribute('uv1');
    });
    return { spec, parts, scale, front: box.max.z * scale, radius };
}

const between = (rng, r) => r[0] + rng() * (r[1] - r[0]);

function spreadClusters(school, h, isle) {
    const n = school.count;
    const clusters = Math.max(1, Math.round(n / 8));
    const p = school.pos;
    const k = P.SCALE.fish;
    for (let c = 0; c < clusters; c++) {
        let x = 0, y = 0, z = 0, m = 0;
        for (let i = c; i < n; i += clusters) {
            x += p[i * 3];
            y += p[i * 3 + 1];
            z += p[i * 3 + 2];
            m++;
        }
        x /= m;
        y /= m;
        z /= m;
        for (let i = c; i < n; i += clusters) {
            const o = i * 3;
            p[o] = Math.min(h.x[1], Math.max(h.x[0], x + (p[o] - x) * k));
            p[o + 2] = Math.min(h.z[1], Math.max(h.z[0], z + (p[o + 2] - z) * k));
            p[o + 1] = Math.min(P.SEA - h.depth[0], Math.max(P.SEA - h.depth[1], y + (p[o + 1] - y) * (k - 1)));
            clampOutOfShore(p, o, isle);
        }
    }
}

function clampOutOfShore(pos, o, isle) {
    const dx = pos[o] - isle.cx;
    const dz = pos[o + 2] - isle.cz;
    const th = Math.atan2(dz, dx);
    const minR = P.shoreRadius(isle, th) + 1.4;
    const r = Math.hypot(dx, dz);
    if (r < minR) {
        const inv = r > 1e-5 ? 1 / r : 1;
        pos[o] = isle.cx + (r > 1e-5 ? dx * inv : 1) * minR;
        pos[o + 2] = isle.cz + (r > 1e-5 ? dz * inv : 0) * minR;
    }
}

function mergedParts(model) {
    const list = model.parts.flatMap(part => {
        const g = part.geometry.clone();
        const kind = MATERIAL_KIND[part.name] ?? 0;
        const count = g.getAttribute('position').count;
        g.setAttribute('aMat', new THREE.BufferAttribute(new Float32Array(count).fill(kind), 1));
        if (!DOUBLE_SIDED[part.name]) return [g];
        const back = g.clone();
        const idxAttr = back.getIndex();
        if (idxAttr) {
            const index = idxAttr.array.slice();
            for (let i = 0; i < index.length; i += 3) {
                const t = index[i + 1];
                index[i + 1] = index[i + 2];
                index[i + 2] = t;
            }
            back.setIndex(new THREE.BufferAttribute(index, 1));
        }
        const normal = back.getAttribute('normal');
        back.setAttribute('normal', new THREE.BufferAttribute(normal.array.map(v => -v), 3));
        return [g, back];
    });
    const merged = list.length === 1 ? list[0] : mergeGeometries(list);
    if (list.length > 1) list.forEach(g => g.dispose());
    merged.computeBoundingSphere();
    return [{ name: 'merged', geometry: merged }];
}

export function createPirateSeaLife({ load }) {
    const group = new THREE.Group();
    group.name = 'aurago-pirate-sealife';
    const clock = { value: 0 };
    const materials = createPirateSeaMaterials(THREE, clock);
    const models = new Map();
    const frustum = new THREE.Frustum();
    const proj = new THREE.Matrix4();
    const sphere = new THREE.Sphere();
    const speciesOf = id => SPECIES.find(s => s.id === id);

    let disposed = false;
    let active = false;
    let loaded = false;
    let loading = null;
    let failures = 0;
    let tier = 'medium';
    let course = null;
    let look = null;
    let waterRadius = 60;
    let layout = null;
    let pop = null;
    let builtKey = null;
    let now = 0;
    let posed = false;
    let leap = null;
    let cursor = 0;
    const lastCam = new Float32Array(17);

    function cameraMoved(camera) {
        const e = camera.matrixWorld.elements;
        const p0 = camera.projectionMatrix.elements[0];
        let diff = Math.abs(lastCam[16] - p0) > 1e-6;
        for (let i = 0; i < 16; i++) {
            if (Math.abs(lastCam[i] - e[i]) > 1e-5) diff = true;
            lastCam[i] = e[i];
        }
        lastCam[16] = p0;
        return diff;
    }

    function makeBatch(model, count, { tint = false, material, merge = false, renderOrder = 0 } = {}) {
        const n = Math.max(1, count);
        const inst = new THREE.InstancedBufferAttribute(new Float32Array(n * 4), 4);
        inst.setUsage(THREE.DynamicDrawUsage);
        const tintAttr = tint ? new THREE.InstancedBufferAttribute(new Float32Array(n * 3), 3) : null;
        const geometries = [];
        let first = null;
        const meshes = (merge ? mergedParts(model) : model.parts).map(part => {
            const g = merge ? part.geometry : part.geometry.clone();
            g.setAttribute('aInst', inst);
            if (tintAttr) g.setAttribute('aTint', tintAttr);
            geometries.push(g);
            const mesh = new THREE.InstancedMesh(g, material(part.name), n);
            mesh.frustumCulled = false;
            mesh.castShadow = false;
            mesh.receiveShadow = false;
            mesh.visible = false;
            mesh.count = 0;
            mesh.renderOrder = renderOrder;
            if (first) mesh.instanceMatrix = first.instanceMatrix;
            else {
                first = mesh;
                mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
            }
            group.add(mesh);
            return mesh;
        });
        return {
            meshes,
            inst,
            tint: tintAttr,
            matrix: first.instanceMatrix,
            geometries,
            show(k) {
                for (const m of meshes) {
                    m.count = k;
                    m.visible = k > 0;
                }
                if (k > 0) {
                    this.matrix.needsUpdate = true;
                    inst.needsUpdate = true;
                    if (tintAttr) tintAttr.needsUpdate = true;
                }
            },
            visible: () => meshes[0].visible,
            dispose() {
                meshes.forEach(m => {
                    m.removeFromParent();
                    m.dispose();
                });
                geometries.forEach(g => g.dispose());
            }
        };
    }

    function makeEffect(geometry, material, count, name) {
        const attr = new THREE.InstancedBufferAttribute(new Float32Array(count * 2), 2);
        attr.setUsage(THREE.DynamicDrawUsage);
        geometry.setAttribute(name, attr);
        const mesh = new THREE.InstancedMesh(geometry, material, count);
        mesh.frustumCulled = false;
        mesh.castShadow = false;
        mesh.receiveShadow = false;
        mesh.visible = false;
        mesh.count = 0;
        mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
        mesh.renderOrder = 1;
        group.add(mesh);
        return {
            mesh,
            attr,
            list: [],
            dispose() {
                mesh.removeFromParent();
                mesh.dispose();
                geometry.dispose();
            }
        };
    }

    function clearPop() {
        if (!pop) return;
        for (const fb of pop.fishBatches) fb.batch.dispose();
        for (const b of pop.big) b.batch.dispose();
        for (const l of pop.leapers.values()) l.dispose();
        pop.rings.dispose();
        pop.puffs.dispose();
        pop = null;
        builtKey = null;
        leap = null;
        posed = false;
    }

    function addRing(ringsList, x, z, t0, size, opacity = 1.0) {
        if (ringsList.length >= MAX_RINGS) ringsList.shift();
        ringsList.push({ x, z, t0, size, opacity });
    }

    function build() {
        if (disposed || !active || tier === 'low' || !loaded || !layout) {
            if (!active || tier === 'low') clearPop();
            return;
        }
        const key = `${course?.id || 'pirate'}:${tier}`;
        if (builtKey === key && pop) return;
        clearPop();

        const next = {
            layout,
            fishBatches: [],
            allSchools: [],
            big: [],
            leapers: new Map(),
            rings: null,
            puffs: null
        };

        const bySpecies = new Map();
        layout.habitats.forEach((h, idx) => {
            const model = models.get(h.species);
            if (!model) return;
            const rng = createRng(0x5ea1 + idx * 7919 + P.courseSeed(course));
            const k = P.SCALE.fish;
            const spec = model.spec;
            const w = h.x[1] - h.x[0];
            const d = h.z[1] - h.z[0];
            const ix = Math.min(1.0, 0.18 * w);
            const iz = Math.min(1.0, 0.18 * d);
            const bounds = {
                x: [h.x[0] + ix, h.x[1] - ix],
                y: [P.SEA - h.depth[1], P.SEA - h.depth[0]],
                z: [h.z[0] + iz, h.z[1] - iz]
            };
            const school = createSchool({
                count: h.count,
                rng,
                bounds,
                speed: spec.speed.map(v => v * k),
                size: [0.85, 1.15],
                params: { neighbor: 1.0 * k, maxTurn: 2.6, bounce: 1.8 }
            });
            spreadClusters(school, h, layout.isle);
            const instData = new Float32Array(h.count * 4);
            const amp = spec.swim.amp * spec.length;
            for (let i = 0; i < h.count; i++) {
                instData.set([school.phase[i], between(rng, spec.swim.freq), amp, 1], i * 4);
            }
            const cx = (h.x[0] + h.x[1]) * 0.5;
            const cz = (h.z[0] + h.z[1]) * 0.5;
            const cy = P.SEA - (h.depth[0] + h.depth[1]) * 0.5;
            const entry = {
                h,
                model,
                school,
                instData,
                acc: (idx % 4) * P.BOIDS_STEP / 4,
                hidden: -1,
                visible: false,
                centre: new THREE.Vector3(cx, cy, cz),
                radius: Math.hypot(w * 0.5, 1.0, d * 0.5) + 2.5
            };
            if (!bySpecies.has(h.species)) bySpecies.set(h.species, []);
            bySpecies.get(h.species).push(entry);
            next.allSchools.push(entry);
        });

        for (const [speciesId, schools] of bySpecies) {
            const model = models.get(speciesId);
            const total = schools.reduce((sum, s) => sum + s.school.count, 0);
            if (!model || !total) continue;
            const batch = makeBatch(model, total, {
                material: () => materials.creatureMerged(model),
                merge: true,
                renderOrder: 0
            });
            next.fishBatches.push({ speciesId, model, batch, schools, shown: 0 });
        }

        const cSeed = P.courseSeed(course);
        for (const def of BIG) {
            const list = def.kind === 'jelly' ? layout.jellies : layout.loops[def.kind];
            const model = models.get(def.id);
            const n = list?.length || 0;
            if (!model || !n) continue;
            const spec = model.spec;
            const rng = createRng(0xb16 + def.kind.length * 977 + def.kind.charCodeAt(0) + cSeed);
            const jelly = def.kind === 'jelly';
            const merge = !jelly;
            const material = jelly
                ? part => materials.jelly(part)
                : def.kind === 'manta'
                    ? () => materials.manta(model, 1.2)
                    : () => materials.creatureMerged(model);
            const batch = makeBatch(model, n, { tint: jelly, material, merge, renderOrder: 0 });
            const stroke = new Float32Array(n * 4);
            const tints = jelly ? new Float32Array(n * 3) : null;
            const amp = def.kind === 'whale' ? spec.swim.amp : spec.swim.amp * spec.length;
            for (let i = 0; i < n; i++) {
                if (jelly) {
                    const J = list[i];
                    stroke.set([J.pulse, J.freq, 0.4, 1], i * 4);
                    tints.set([
                        0.8 + 0.4 * ((i * 37) % 11) / 11,
                        0.85 + 0.3 * ((i * 53) % 7) / 7,
                        1
                    ], i * 3);
                } else {
                    stroke.set([rng(), between(rng, spec.swim.freq) * def.rate, amp, 1], i * 4);
                }
            }
            next.big.push({
                def,
                model,
                batch,
                list,
                n,
                stroke,
                tints,
                lastWake: new Float64Array(n).fill(-10),
                poses: Array.from({ length: n }, () => ({ x: 0, y: P.SEA, z: 0, fx: 0, fy: 0, fz: 1, roll: 0, size: 1 })),
                shown: 0
            });
        }

        const ringGeo = new THREE.RingGeometry(0.58, 1, 24, 1);
        ringGeo.rotateX(-Math.PI / 2);
        next.rings = makeEffect(ringGeo, materials.ring(), MAX_RINGS, 'aRing');
        const puffGeo = new THREE.PlaneGeometry(1, 1);
        next.puffs = makeEffect(puffGeo, materials.puff(), P.BLOW.count, 'aPuff');

        for (const id of ['fish_sardine', 'fish_tang']) {
            const model = models.get(id);
            if (!model) continue;
            const b = makeBatch(model, 1, {
                material: () => materials.creatureMerged(model),
                merge: true,
                renderOrder: 0
            });
            b.inst.setXYZW(0, 0, 16, model.spec.swim.amp * model.spec.length * 1.3, 1);
            next.leapers.set(id, b);
        }

        // Seed initial dorsal-fin wakes at the current clock so paused/reduced frames show surface ripples too.
        const initPose = { x: 0, y: P.SEA, z: 0, fx: 0, fy: 0, fz: 1, roll: 0 };
        for (const b of next.big) {
            if (b.def.kind !== 'shark' && b.def.kind !== 'hammer') continue;
            for (let i = 0; i < b.n; i++) {
                P.seaPose(layout.isle, b.def.kind, b.list[i], i, now, b.n, initPose);
                if (initPose.y + b.list[i].finTop > P.SEA + 0.02) {
                    b.lastWake[i] = now - 0.25;
                    addRing(next.rings.list, initPose.x - initPose.fx * 0.15, initPose.z - initPose.fz * 0.15, now - 0.25, 0.55, 0.55);
                }
            }
        }

        pop = next;
        builtKey = key;
        posed = false;
        leap = null;
        cursor = P.firstAtOrAfter(P.LEAP_STARTS, now);
    }

    const env = { repulsors: [], speedMul: 1, cohesionMul: 1 };

    function stepSchool(S, dt) {
        const { school, h } = S;
        const isle = pop.layout.isle;
        env.repulsors.length = 0;
        for (const b of pop.big) {
            if (b.def.kind !== 'shark' && b.def.kind !== 'hammer') continue;
            for (let i = 0; i < b.n; i++) {
                const p = b.poses[i];
                if (Math.hypot(p.x - S.centre.x, p.z - S.centre.z) < S.radius + 7) {
                    env.repulsors.push({ x: p.x, y: p.y, z: p.z, radius: 5.0, strength: 4.5 });
                }
            }
        }
        school.step(dt, env);
        const pos = school.pos;
        for (let i = 0; i < school.count; i++) {
            const o = i * 3;
            pos[o] = Math.min(h.x[1], Math.max(h.x[0], pos[o]));
            pos[o + 1] = Math.min(P.SEA - h.depth[0], Math.max(P.SEA - h.depth[1], pos[o + 1]));
            pos[o + 2] = Math.min(h.z[1], Math.max(h.z[0], pos[o + 2]));
            clampOutOfShore(pos, o, isle);
        }
    }

    function writeSchoolIntoBatch(S, batch, startSlot) {
        const { school, instData, acc, hidden } = S;
        const a = batch.matrix.array;
        const ia = batch.inst.array;
        const pos = school.pos;
        const vel = school.vel;
        const k = P.SCALE.fish;
        ia.set(instData, startSlot * 4);
        for (let i = 0; i < school.count; i++) {
            const slot = startSlot + i;
            if (i === hidden) {
                a.set(ZERO, slot * 16);
                continue;
            }
            const o = i * 3;
            const l = Math.hypot(vel[o], vel[o + 1], vel[o + 2]);
            P.place(
                a,
                slot * 16,
                pos[o] + vel[o] * acc,
                Math.min(P.SEA - 0.05, pos[o + 1] + vel[o + 1] * acc),
                pos[o + 2] + vel[o + 2] * acc,
                l > 1e-6 ? vel[o] : 1,
                l > 1e-6 ? vel[o + 1] : 0,
                l > 1e-6 ? vel[o + 2] : 0,
                school.size[i] * k,
                0
            );
        }
    }

    const leapOut = { x: 0, z: 0, h: 0, fx: 0, fy: 0, fz: 1 };
    const leapMath = { apex: 0, speed: 0 };

    function startLeap(k, t0) {
        const candidates = pop.allSchools.filter(S => S.visible && pop.leapers.has(S.h.species));
        if (!candidates.length) return false;
        const S = candidates[(Math.imul(k + 3, 2654435761) >>> 0) % candidates.length];
        const school = S.school;
        const i = (Math.imul(k + 7, 2654435761) >>> 0) % school.count;
        const o = i * 3;
        const x0 = school.pos[o];
        const z0 = school.pos[o + 2];
        const isle = pop.layout.isle;
        if (P.shoreClearance(isle, x0, z0) < P.KEEP_FROM_SHORE) return false;
        const l = Math.hypot(school.vel[o], school.vel[o + 2]) || 1;
        let dx = school.vel[o] / l || 1;
        let dz = school.vel[o + 2] / l;
        const lp = P.leapParams(k < 0 ? 9000 - k : k, leapMath);
        const dist = lp.speed * P.LEAP.duration;
        let landX = x0 + dx * dist;
        let landZ = z0 + dz * dist;
        if (P.shoreClearance(isle, landX, landZ) < P.KEEP_FROM_SHORE) {
            const ox = x0 - isle.cx;
            const oz = z0 - isle.cz;
            const ol = Math.hypot(ox, oz) || 1;
            dx = ox / ol;
            dz = oz / ol;
            landX = x0 + dx * dist;
            landZ = z0 + dz * dist;
            if (P.shoreClearance(isle, landX, landZ) < P.KEEP_FROM_SHORE) return false;
        }
        leap = { k, t0, school: S, fish: i, x0, z0, dx, dz, apex: lp.apex, speed: lp.speed };
        S.hidden = i;
        addRing(pop.rings.list, x0, z0, t0, 0.75, 1.0);
        addRing(pop.rings.list, landX, landZ, t0 + P.LEAP.duration, 0.75, 1.0);
        return true;
    }

    function endLeap() {
        if (!leap) return;
        const S = leap.school;
        const o = leap.fish * 3;
        const dist = leap.speed * P.LEAP.duration;
        const h = S.h;
        S.school.pos[o] = Math.min(h.x[1], Math.max(h.x[0], leap.x0 + leap.dx * dist));
        S.school.pos[o + 2] = Math.min(h.z[1], Math.max(h.z[0], leap.z0 + leap.dz * dist));
        S.school.pos[o + 1] = P.SEA - 0.5;
        clampOutOfShore(S.school.pos, o, pop.layout.isle);
        S.hidden = -1;
        pop.leapers.get(S.h.species)?.show(0);
        leap = null;
    }

    function updateLeaps(animated) {
        if (leap) {
            const s = (now - leap.t0) / P.LEAP.duration;
            if (s >= 1 || s < 0) {
                endLeap();
            } else {
                P.leapPose(leap.apex, leap.speed, leap.dx, leap.dz, s, leapOut);
                const b = pop.leapers.get(leap.school.h.species);
                P.place(
                    b.matrix.array,
                    0,
                    leap.x0 + leapOut.x,
                    P.SEA + leapOut.h,
                    leap.z0 + leapOut.z,
                    leapOut.fx,
                    leapOut.fy,
                    leapOut.fz,
                    leap.school.school.size[leap.fish] * P.SCALE.fish,
                    0
                );
                b.show(1);
            }
        }
        if (!animated || leap) return;
        while (cursor < P.LEAP_STARTS.length && P.LEAP_STARTS[cursor] < now - P.LEAP.duration) cursor++;
        if (cursor < P.LEAP_STARTS.length && P.LEAP_STARTS[cursor] <= now) {
            startLeap(cursor, P.LEAP_STARTS[cursor]);
            cursor++;
        }
    }

    const tilt = { x: 0, y: 0, z: 0 };

    function poseBig(B, i, t) {
        const kind = B.def.kind;
        const p = B.poses[i];
        const isle = pop.layout.isle;
        if (kind === 'jelly') {
            const J = B.list[i];
            P.jellyPosition(isle, J, t, tilt);
            const yaw = J.tilt + t * 0.03;
            const pitch = 0.28 * Math.sin(t * 0.07 + J.tilt);
            const c = Math.cos(pitch);
            p.x = tilt.x;
            p.y = tilt.y;
            p.z = tilt.z;
            p.fx = Math.cos(yaw) * c;
            p.fy = Math.sin(pitch);
            p.fz = Math.sin(yaw) * c;
            p.roll = 0.28 * Math.cos(t * 0.05 + J.tilt * 1.3);
            p.size = J.size;
            return;
        }
        const L = B.list[i];
        P.seaPose(isle, kind, L, i, t, B.n, p);
        p.size = L.size;
    }

    function updateBig(B, animated) {
        const a = B.batch.matrix.array;
        const ia = B.batch.inst.array;
        const isShark = B.def.kind === 'shark' || B.def.kind === 'hammer';
        let n = 0;
        for (let i = 0; i < B.n; i++) {
            poseBig(B, i, now);
            const p = B.poses[i];
            if (animated && isShark && p.y + B.list[i].finTop > P.SEA + 0.02 && now - B.lastWake[i] >= 0.45) {
                B.lastWake[i] = now;
                addRing(pop.rings.list, p.x - p.fx * 0.15, p.z - p.fz * 0.15, now, 0.55, 0.55);
            }
            const r = B.model.radius * p.size * 1.2;
            sphere.center.set(p.x, p.y, p.z);
            sphere.radius = r;
            if (!frustum.intersectsSphere(sphere)) continue;
            P.place(a, n * 16, p.x, p.y, p.z, p.fx, p.fy, p.fz, p.size, p.roll);
            ia.set(B.stroke.subarray(i * 4, i * 4 + 4), n * 4);
            if (B.batch.tint) B.batch.tint.array.set(B.tints.subarray(i * 3, i * 3 + 3), n * 3);
            n++;
        }
        B.shown = n;
        B.batch.show(n);
    }

    const puffOut = { x: 0, y: 0, z: 0, size: 1 };
    const ringHit = { k: -1 };
    const evHit = { k: 0, u: 0 };
    const whaleBase = { x: 0, y: 0, z: 0, fx: 0, fy: 0, fz: 1, roll: 0 };

    function updateEffects(animated) {
        const R = pop.rings;
        const Pf = pop.puffs;
        const whaleLoop = pop.layout.loops.whale[0];
        if (animated && whaleLoop) {
            const e = P.eventAt(P.WHALE_EVENTS, now, P.SURFACING.whale.rise, P.SURFACING.whale.hold + P.SURFACING.whale.fall, evHit);
            if (e && e.k !== ringHit.k) {
                ringHit.k = e.k;
                for (const du of [-0.8, 3.4]) {
                    P.seaPose(pop.layout.isle, 'whale', whaleLoop, 0, now - e.u + du, 1, whaleBase);
                    addRing(R.list, whaleBase.x, whaleBase.z, now - e.u + du, 3.0, 1.0);
                }
            }
        }
        for (let i = R.list.length - 1; i >= 0; i--) {
            if (now - R.list[i].t0 > P.LEAP.ring * 1.3) R.list.splice(i, 1);
        }
        let n = 0;
        const a = R.mesh.instanceMatrix.array;
        const ra = R.attr.array;
        for (const r of R.list) {
            const age = (now - r.t0) / P.LEAP.ring;
            if (age < 0 || age > 1 || n >= MAX_RINGS) continue;
            P.place(a, n * 16, r.x, P.SEA + 0.03, r.z, 0, 0, 1, r.size * (0.25 + 1.5 * age), 0);
            ra[n * 2] = age;
            ra[n * 2 + 1] = r.opacity;
            n++;
        }
        R.mesh.count = n;
        R.mesh.visible = n > 0;
        if (n > 0) {
            R.mesh.instanceMatrix.needsUpdate = true;
            R.attr.needsUpdate = true;
        }

        let m = 0;
        const pa = Pf.mesh.instanceMatrix.array;
        const pr = Pf.attr.array;
        if (whaleLoop) {
            for (let i = 0; i < P.BLOW.count; i++) {
                const age = P.blowPuff(pop.layout.isle, whaleLoop, now, i, puffOut);
                if (age < 0) continue;
                P.place(pa, m * 16, puffOut.x, puffOut.y, puffOut.z, 0, 0, 1, puffOut.size, 0);
                pr[m * 2] = age;
                pr[m * 2 + 1] = i;
                m++;
            }
        }
        Pf.mesh.count = m;
        Pf.mesh.visible = m > 0;
        if (m > 0) {
            Pf.mesh.instanceMatrix.needsUpdate = true;
            Pf.attr.needsUpdate = true;
        }
    }

    function ensureLoaded() {
        if (loading) return loading;
        loading = Promise.allSettled(MODELS.map(async m => {
            const spec = m.length ? { ...speciesOf(m.id), length: m.length } : speciesOf(m.id);
            return prepareModel(await load(FOLDERS[m.folder] + m.file), spec, m.fixup);
        })).then(results => {
            results.forEach((r, i) => {
                if (r.status === 'fulfilled') {
                    if (disposed) r.value.parts.forEach(p => p.geometry.dispose());
                    else models.set(MODELS[i].id, r.value);
                } else {
                    failures++;
                }
            });
            loaded = true;
        });
        return loading;
    }

    function activate() {
        if (!active || tier === 'low' || disposed) {
            clearPop();
            return;
        }
        if (loaded) build();
        else ensureLoaded().then(() => {
            if (!disposed && active && tier !== 'low') build();
        });
    }

    return {
        group,
        configure(options = {}) {
            active = true;
            course = options.course || course;
            look = options.look || look;
            waterRadius = options.waterRadius ?? waterRadius;
            tier = options.tier || tier;
            layout = P.buildCourseLayout(course, tier);
            materials.setEnvironment({ look, waterRadius, isle: layout.isle });
            activate();
        },
        clear() {
            active = false;
            clearPop();
        },
        setTier(nextTier) {
            const normalized = nextTier || 'medium';
            if (tier === normalized) return;
            tier = normalized;
            if (!active) return;
            layout = P.buildCourseLayout(course, tier);
            materials.setEnvironment({ look, waterRadius, isle: layout.isle });
            if (tier === 'low') clearPop();
            else activate();
        },
        update(dt, camera, animated = true) {
            if (!pop || !active || disposed || !camera) return;
            const advancing = animated && dt > 0;
            if (advancing) {
                const step = Math.min(0.1, dt);
                now += step;
                clock.value = now;
            }
            camera.updateMatrixWorld();
            const moved = cameraMoved(camera);
            if (!advancing && posed && !moved) return;

            proj.multiplyMatrices(camera.projectionMatrix, camera.matrixWorldInverse);
            frustum.setFromProjectionMatrix(proj);

            // Update big animals first so fish schools react to patrolling sharks.
            for (const B of pop.big) updateBig(B, advancing);

            const stepDt = advancing ? Math.min(0.1, dt) : 0;
            for (const fb of pop.fishBatches) {
                let slot = 0;
                for (const S of fb.schools) {
                    sphere.center.copy(S.centre);
                    sphere.radius = S.radius;
                    S.visible = frustum.intersectsSphere(sphere);
                    if (!S.visible) continue;
                    if (stepDt > 0) {
                        S.acc += stepDt;
                        let steps = 0;
                        while (S.acc >= P.BOIDS_STEP && steps < 2) {
                            stepSchool(S, P.BOIDS_STEP);
                            S.acc -= P.BOIDS_STEP;
                            steps++;
                        }
                        if (S.acc >= P.BOIDS_STEP) S.acc = 0;
                    }
                    writeSchoolIntoBatch(S, fb.batch, slot);
                    slot += S.school.count;
                }
                fb.shown = slot;
                fb.batch.show(slot);
            }

            updateLeaps(advancing);
            updateEffects(advancing);
            posed = true;
        },
        stats() {
            const bigCount = kind => pop?.big.find(b => b.def.kind === kind)?.n || 0;
            const fish = pop ? pop.allSchools.reduce((s, x) => s + x.school.count, 0) : 0;
            const visibleFish = pop ? pop.fishBatches.reduce((s, fb) => s + fb.shown, 0) : 0;
            let drawCalls = 0;
            if (pop) {
                for (const fb of pop.fishBatches) if (fb.batch.visible()) drawCalls += fb.batch.meshes.length;
                for (const B of pop.big) if (B.batch.visible()) drawCalls += B.batch.meshes.length;
                for (const l of pop.leapers.values()) if (l.visible()) drawCalls += l.meshes.length;
                if (pop.rings.mesh.visible) drawCalls++;
                if (pop.puffs.mesh.visible) drawCalls++;
            }
            return {
                active,
                loaded: !!pop,
                failures,
                tier,
                sharks: bigCount('shark'),
                hammerheads: bigCount('hammer'),
                turtles: bigCount('turtle'),
                mantas: bigCount('manta'),
                whale: bigCount('whale'),
                jellies: bigCount('jelly'),
                schools: pop?.allSchools.length || 0,
                fish,
                visibleBig: pop ? pop.big.reduce((s, b) => s + b.shown, 0) : 0,
                visibleFish,
                drawCalls
            };
        },
        dispose() {
            disposed = true;
            active = false;
            clearPop();
            materials.dispose();
            for (const m of models.values()) {
                m.parts.forEach(p => p.geometry.dispose());
            }
            models.clear();
            group.removeFromParent();
        }
    };
}
