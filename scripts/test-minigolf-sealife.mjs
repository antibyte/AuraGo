import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import { LOOKS } from '../ui/js/desktop/apps/minigolf-looks.js';
import { overviewFor } from '../ui/js/desktop/apps/minigolf-camera.js';
import * as P from '../ui/js/desktop/apps/minigolf-sealife-paths.js';
import {
    REMAP,
    WATER,
    CREATURE_FRAGMENT,
    JELLY_FRAGMENT,
    MANTA_FRAGMENT,
    RING_FRAGMENT,
    PUFF_FRAGMENT,
    createPirateSeaMaterials
} from '../ui/js/desktop/apps/minigolf-sealife-shaders.js';
import { FOLDERS, MODELS, createPirateSeaLife } from '../ui/js/desktop/apps/minigolf-sealife.js';

const manifest = JSON.parse(await fs.readFile('ui/3d/minigolf/manifest.json', 'utf8'));
const pirateCourses = manifest.courses.filter(c => c.world === 'pirate');
assert.equal(pirateCourses.length, 6, 'expected 6 pirate courses');

// ---- 1. Layout, island clearance, shark fin exposure, habitats, determinism ----
const pos = {};
for (const course of pirateCourses) {
    const low = P.buildCourseLayout(course, 'low');
    assert.deepEqual(
        [low.loops.shark.length, low.loops.hammer.length, low.loops.turtle.length, low.loops.manta.length, low.loops.whale.length, low.jellies.length, low.habitats.length],
        [0, 0, 0, 0, 0, 0, 0],
        `${course.id} low tier is empty`
    );

    const med = P.buildCourseLayout(course, 'medium');
    assert.equal(med.loops.shark.length, 7, `${course.id} medium reef sharks`);
    assert.equal(med.loops.hammer.length, 3, `${course.id} medium hammerheads`);
    assert.equal(med.loops.turtle.length, 3, `${course.id} medium turtles`);
    assert.equal(med.loops.manta.length, 2, `${course.id} medium mantas`);
    assert.equal(med.loops.whale.length, 1, `${course.id} medium whale`);
    assert.equal(med.jellies.length, 4, `${course.id} medium jellies`);
    assert.equal(med.habitats.length, 6, `${course.id} medium schools`);

    const highA = P.buildCourseLayout(course, 'high');
    const highB = P.buildCourseLayout(course, 'high');
    assert.deepEqual(highA, highB, `${course.id} layout is deterministic`);
    assert.equal(highA.loops.shark.length, 10, `${course.id} high reef sharks`);
    assert.equal(highA.loops.hammer.length, 5, `${course.id} high hammerheads`);
    assert.equal(highA.loops.turtle.length, 4, `${course.id} high turtles`);
    assert.equal(highA.loops.manta.length, 3, `${course.id} high mantas`);
    assert.equal(highA.loops.whale.length, 1, `${course.id} high whale`);
    assert.equal(highA.jellies.length, 6, `${course.id} high jellies`);
    assert.equal(highA.habitats.length, 11, `${course.id} high schools`);

    const waterR = overviewFor(course).waterRadius;
    const isle = highA.isle;

    for (const [kind, loops] of Object.entries(highA.loops)) {
        loops.forEach((L, i) => {
            let finOut = 0;
            let submerged = 0;
            let samples = 0;
            for (let t = 0; t <= 1200; t += 1) {
                P.seaPosition(isle, kind, L, i, t, loops.length, pos);
                assert.ok(Number.isFinite(pos.x + pos.y + pos.z), `${course.id} ${kind}[${i}] finite`);
                const clear = P.shoreClearance(isle, pos.x, pos.z);
                assert.ok(clear >= P.KEEP_FROM_SHORE - 1e-6, `${course.id} ${kind}[${i}] clearance ${clear}`);
                const distFromOrigin = Math.hypot(pos.x, pos.z);
                assert.ok(distFromOrigin < waterR, `${course.id} ${kind}[${i}] inside water disc (${distFromOrigin} < ${waterR})`);
                samples++;
                if (kind === 'shark' || kind === 'hammer') {
                    if (pos.y + L.finTop > P.SEA + 0.02) finOut++;
                    else submerged++;
                }
            }
            if (kind === 'shark' || kind === 'hammer') {
                assert.ok(finOut / samples > 0.15, `${course.id} ${kind}[${i}] dorsal fin breaks surface (${finOut}/${samples})`);
                assert.ok(submerged / samples > 0.15, `${course.id} ${kind}[${i}] dives submerged (${submerged}/${samples})`);
            }
        });
    }

    highA.jellies.forEach((J, i) => {
        for (let t = 0; t <= 1200; t += 5) {
            P.jellyPosition(isle, J, t, pos);
            assert.ok(Number.isFinite(pos.x + pos.y + pos.z), `${course.id} jelly[${i}] finite`);
            const clear = P.shoreClearance(isle, pos.x, pos.z);
            assert.ok(clear >= P.KEEP_FROM_SHORE - 1e-6, `${course.id} jelly[${i}] clearance ${clear}`);
            const depth = P.SEA - pos.y;
            assert.ok(depth >= 0.3 && depth <= 1.2, `${course.id} jelly[${i}] depth ${depth}`);
        }
    });

    highA.habitats.forEach((h, i) => {
        assert.ok(h.x[1] > h.x[0] && h.z[1] > h.z[0], `${course.id} habitat[${i}] non-empty`);
        assert.ok(h.depth[0] >= 0.2 && h.depth[1] <= 1.3, `${course.id} habitat[${i}] depth ${h.depth}`);
        const xm = (h.x[0] + h.x[1]) * 0.5;
        const zm = (h.z[0] + h.z[1]) * 0.5;
        for (const [px, pz] of [
            [h.x[0], h.z[0]], [h.x[1], h.z[0]], [h.x[0], h.z[1]], [h.x[1], h.z[1]],
            [xm, h.z[0]], [xm, h.z[1]], [h.x[0], zm], [h.x[1], zm]
        ]) {
            const clear = P.shoreClearance(isle, px, pz);
            assert.ok(clear >= P.KEEP_FROM_SHORE - 1e-6, `${course.id} habitat[${i}] clearance ${clear}`);
        }
    });
}

// ---- 2. Surfacing, blow puffs, leaps, seaRemap, shaders, place() ----
const sampleLayout = P.buildCourseLayout(pirateCourses[0], 'high');
const samplePose = {};
const turtleCount = sampleLayout.loops.turtle.length;
const TE = P.TURTLE_EVENTS[5];
const whoTurtle = 5 % turtleCount;
for (let i = 0; i < turtleCount; i++) {
    const d = P.depthAt('turtle', sampleLayout.loops.turtle[i], i, TE + 1.0, turtleCount);
    if (i === whoTurtle) assert.ok(d <= 0.10, `surfacing turtle depth ${d}`);
    else assert.ok(d >= 0.44, `other turtles stay deeper ${d}`);
}
assert.ok(P.depthAt('turtle', sampleLayout.loops.turtle[whoTurtle], whoTurtle, TE + 3.5, turtleCount) <= 0.10);

const WE = P.WHALE_EVENTS[3];
const whaleLoop = sampleLayout.loops.whale[0];
assert.equal(P.depthAt('whale', whaleLoop, 0, WE - 20, 1), P.SURFACING.whale.cruise);
assert.ok(P.depthAt('whale', whaleLoop, 0, WE + 1.0, 1) < 1.326 * P.SCALE.whale, 'whale back rises above water');
P.seaPose(sampleLayout.isle, 'whale', whaleLoop, 0, WE + 4.0, 1, samplePose);
assert.ok(samplePose.fy < -0.1, `whale dives nose down: ${samplePose.fy}`);
let maxRise = 0;
let puffsAlive = 0;
const pf = {};
for (let t = WE; t < WE + 8; t += 0.05) {
    for (let i = 0; i < P.BLOW.count; i++) {
        const age = P.blowPuff(sampleLayout.isle, whaleLoop, t, i, pf);
        if (age >= 0) {
            puffsAlive++;
            maxRise = Math.max(maxRise, pf.y - P.SEA);
            assert.ok(age <= 1 && Number.isFinite(pf.x + pf.z));
        }
    }
}
assert.ok(puffsAlive > 0 && maxRise > 2.2 && maxRise < 3.0, `whale blow rise ${maxRise}`);

const lp = {};
for (let k = 0; k < 120; k++) {
    const q = P.leapParams(k, {});
    assert.ok(q.apex >= 0.45 && q.apex <= 0.85 && q.speed >= 2.2 && q.speed <= 3.0);
    P.leapPose(q.apex, q.speed, 1, 0, 0, lp);
    assert.equal(lp.h, 0);
    P.leapPose(q.apex, q.speed, 1, 0, 0.5, lp);
    assert.ok(Math.abs(lp.h - q.apex) < 1e-9);
    P.leapPose(q.apex, q.speed, 1, 0, 1, lp);
    assert.ok(Math.abs(lp.h) < 1e-12);
}

const cam = [20, 15, 25];
const remapOut = {};
for (const p of [[8, -1.8, 4], [12, -2.5, -6], [-4, -1.2, 2]]) {
    P.seaRemap(...cam, ...p, remapOut);
    const toP = p.map((v, i) => v - cam[i]);
    const toS = [remapOut.x, remapOut.y, remapOut.z].map((v, i) => v - cam[i]);
    const cross = [
        toP[1] * toS[2] - toP[2] * toS[1],
        toP[2] * toS[0] - toP[0] * toS[2],
        toP[0] * toS[1] - toP[1] * toS[0]
    ];
    assert.ok(Math.hypot(...cross) < 1e-6 * Math.hypot(...toP) * Math.hypot(...toS), 'seaRemap stays on view ray');
    assert.ok(remapOut.y >= P.SEA - 1e-9, 'seaRemap raises below-surface point to >= SEA');
    assert.ok(Math.hypot(...toS) < Math.hypot(...toP), 'seaRemap is nearer camera');
}
assert.ok(P.remapBias(0.2, 10) > P.remapBias(1.5, 10), 'shallower wins over deeper');
assert.ok(Number.isFinite(P.seaRemap(cam[0], P.SEA + 0.001, cam[2], 2, -5, 2, {}).y), 'finite near sea surface');

assert.ok(REMAP.includes('max(C.y - uSea, 0.02)') && REMAP.includes('h / max(C.y - P.y, h)') && REMAP.includes('max(length(toC), 0.001)'));
assert.ok(REMAP.includes('0.03 + 0.02 * clamp(6.0 - below, 0.0, 6.0)') && REMAP.includes('1.0 + 0.01 * len'));
for (const m of REMAP.matchAll(/\/([^;,)]{0,16})/g)) {
    const rhs = m[1].trim();
    assert.ok(rhs.startsWith('max(') || rhs.startsWith('len'), `unguarded division in REMAP: /${m[1]}`);
}
for (const m of WATER.replace(/\x2f\x2f.*$/gm, '').matchAll(/\/([^;,)]{0,16})/g)) {
    const rhs = m[1].trim();
    assert.ok(rhs.startsWith('max('), `unguarded division in WATER: /${m[1]}`);
}
for (const [name, frag] of [
    ['CREATURE_FRAGMENT', CREATURE_FRAGMENT],
    ['JELLY_FRAGMENT', JELLY_FRAGMENT],
    ['MANTA_FRAGMENT', MANTA_FRAGMENT],
    ['RING_FRAGMENT', RING_FRAGMENT]
]) {
    assert.ok(frag.includes('insideIsle(vWorldPos)'), `${name} uses insideIsle`);
    assert.ok(frag.includes('#include <fog_fragment>'), `${name} includes fog_fragment`);
    assert.ok(frag.includes('#include <tonemapping_fragment>'), `${name} includes tonemapping_fragment`);
    assert.ok(frag.includes('#include <colorspace_fragment>'), `${name} includes colorspace_fragment`);
}
assert.ok(PUFF_FRAGMENT.includes('#include <fog_fragment>') && PUFF_FRAGMENT.includes('#include <tonemapping_fragment>') && PUFF_FRAGMENT.includes('#include <colorspace_fragment>'));

{
    const a = new Float32Array(32);
    const m = new THREE.Matrix4();
    for (const f of [[1, 0, 0], [0, 0, -1], [0.3, 0.5, 0.8], [-0.4, -0.9, 0.1]]) {
        for (const roll of [0, 0.35]) {
            P.place(a, 0, 1, -1.5, 2, ...f, 1.5, roll);
            m.fromArray(a, 0);
            const z = new THREE.Vector3().setFromMatrixColumn(m, 2);
            const n = new THREE.Vector3(...f).normalize();
            assert.ok(z.clone().divideScalar(1.5).distanceTo(n) < 1e-6, 'place nose along heading');
            const x = new THREE.Vector3().setFromMatrixColumn(m, 0).divideScalar(1.5);
            const y = new THREE.Vector3().setFromMatrixColumn(m, 1).divideScalar(1.5);
            const det = x.clone().cross(y).dot(z.clone().divideScalar(1.5));
            assert.ok(Math.abs(det - 1) < 1e-5, `orthonormal right-handed basis det=${det}`);
        }
    }
    P.place(a, 0, 0, 0, 0, 0, 0.3, 1, 1, 0);
    m.fromArray(a, 0);
    assert.ok(new THREE.Vector3().setFromMatrixColumn(m, 1).y > 0.9, 'level fish keep back up');
}

{
    const mats = createPirateSeaMaterials(THREE, { value: 0 });
    mats.setEnvironment({ look: LOOKS.pirate, waterRadius: 60, isle: sampleLayout.isle });
    const species = { spec: { shader: 'shark', variant: 0, swim: { k: 1.8 }, length: 1.8 }, front: 0.81, scale: 1 };
    assert.equal(mats.creatureMerged(species).fog, true);
    mats.jelly('bell');
    mats.manta(species);
    mats.ring();
    mats.puff();
    assert.equal(mats.count(), 5);
    mats.dispose();
    assert.equal(mats.count(), 0);
}

// ---- 3. Asset manifest & GLB contract checks ----
const manifests = {
    fish: 'ui/3d/system-world/sea-life/v1/',
    chat: 'ui/3d/chat-ocean-life/v1/',
    abyss: 'ui/3d/screensaver/abyss/v1/'
};
const loadedManifests = {};
for (const [k, dir] of Object.entries(manifests)) {
    loadedManifests[k] = JSON.parse(await fs.readFile(dir + 'manifest.json', 'utf8'));
}
for (const m of MODELS) {
    const dir = 'ui/3d/' + FOLDERS[m.folder];
    assert.equal(dir, manifests[m.folder]);
    const entry = loadedManifests[m.folder].assets.find(a => a.id === m.id);
    assert.ok(entry && entry.file === m.file, `${m.id} present in ${m.folder} manifest`);
    const bytes = await fs.readFile(dir + m.file);
    assert.equal(bytes.length, entry.bytes, `${m.file} byte length`);
    assert.equal(crypto.createHash('sha256').update(bytes).digest('hex'), entry.sha256, `${m.file} sha256`);
    assert.equal(bytes.readUInt32LE(0), 0x46546c67, `${m.file} binary glTF`);
    const json = JSON.parse(bytes.subarray(20, 20 + bytes.readUInt32LE(12)).toString());
    assert.ok(!json.extensionsRequired?.length, `${m.file} needs no extensions`);
    for (const prim of json.meshes.flatMap(x => x.primitives)) {
        for (const attr of ['POSITION', 'NORMAL', 'TEXCOORD_0', 'TEXCOORD_1']) {
            assert.ok(prim.attributes[attr] !== undefined, `${m.file} missing ${attr}`);
        }
    }
}

// ---- 4. Runtime module stub tests ----
const partNames = {
    fish_sardine: ['body', 'fin', 'eye'],
    fish_tang: ['body', 'fin', 'eye'],
    fish_clown: ['body', 'fin', 'eye'],
    fish_butterfly: ['body', 'fin', 'eye'],
    shark_reef: ['body', 'fin', 'eye'],
    shark_hammer: ['body', 'fin', 'eye'],
    turtle: ['shell', 'body', 'flipper', 'eye'],
    whale: ['body', 'fin', 'flipper', 'fluke', 'eye'],
    manta: ['manta_top', 'manta_belly'],
    jelly_moon: ['bell', 'gonad', 'arm', 'tentacle']
};
const fileToId = new Map(MODELS.map(m => [FOLDERS[m.folder] + m.file, m.id]));
const flush = () => new Promise(r => setTimeout(r, 20));
const camFor = course => {
    const ov = overviewFor(course);
    const c = new THREE.PerspectiveCamera(43, 1280 / 760, 0.1, 200);
    c.position.set(...ov.position);
    c.lookAt(...ov.target);
    c.updateMatrixWorld();
    c.updateProjectionMatrix();
    return c;
};

{
    const requested = [];
    const stubLoad = path => {
        requested.push(path);
        const id = fileToId.get(path);
        if (!id) return Promise.reject(Error('unknown ' + path));
        const root = new THREE.Group();
        for (const name of partNames[id]) {
            const g = new THREE.BoxGeometry(0.4, 0.3, 1.6);
            g.setAttribute('uv1', g.getAttribute('uv').clone());
            root.add(new THREE.Mesh(g, new THREE.MeshBasicMaterial({ name })));
        }
        return Promise.resolve(root);
    };
    const sea = createPirateSeaLife({ load: stubLoad });
    const cam0 = camFor(pirateCourses[0]);

    // Low tier loads nothing.
    sea.configure({ course: pirateCourses[0], look: LOOKS.pirate, waterRadius: 60, tier: 'low' });
    await flush();
    assert.equal(requested.length, 0, 'low tier loads nothing');
    assert.equal(sea.stats().loaded, false);
    assert.equal(sea.stats().drawCalls, 0);

    // Medium tier loads 10 models once.
    sea.setTier('medium');
    await flush();
    await flush();
    assert.equal(requested.length, 10, 'medium tier loads all 10 GLBs once');
    let st = sea.stats();
    assert.equal(st.failures, 0);
    assert.equal(st.loaded, true);
    assert.equal(st.sharks, 7);
    assert.equal(st.hammerheads, 3);
    assert.equal(st.schools, 6);

    // Frozen initial frame poses once and freezes thereafter.
    sea.update(0.016, cam0, false);
    st = sea.stats();
    assert.ok(st.visibleBig > 0 && st.visibleFish > 0 && st.drawCalls > 0, `initial frozen frame draws: ${JSON.stringify(st)}`);
    const snap = sea.group.children.map(m => Array.from(m.instanceMatrix.array.subarray(0, Math.max(1, m.count) * 16)));
    sea.update(0.5, cam0, false);
    sea.update(0.5, cam0, false);
    sea.group.children.forEach((m, i) => {
        if (m.visible) {
            const cur = Array.from(m.instanceMatrix.array.subarray(0, Math.max(1, m.count) * 16));
            assert.deepEqual(cur, snap[i], 'reduced motion keeps matrices frozen');
        }
    });

    // High tier updates counts without re-fetching.
    sea.setTier('high');
    sea.update(0.05, cam0, true);
    st = sea.stats();
    assert.equal(requested.length, 10, 'tier change reuses cached GLBs');
    assert.equal(st.sharks, 10);
    assert.equal(st.hammerheads, 5);
    assert.equal(st.schools, 11);

    // Animate a few frames & verify finite matrices.
    for (let i = 0; i < 40; i++) sea.update(0.05, cam0, true);
    for (const m of sea.group.children) {
        assert.equal(m.frustumCulled, false);
        const a = m.instanceMatrix.array;
        for (let i = 0; i < m.count * 16; i++) assert.ok(Number.isFinite(a[i]), 'finite instance matrix');
    }

    // Switching to pirate-02 reuses cached GLBs.
    const cam1 = camFor(pirateCourses[1]);
    sea.configure({ course: pirateCourses[1], look: LOOKS.pirate, waterRadius: 60, tier: 'high' });
    sea.update(0.05, cam1, true);
    assert.equal(requested.length, 10, 'switching pirate courses reuses cached GLBs');
    assert.ok(sea.stats().visibleBig > 0);

    // Switching to temple/moon clears batches.
    sea.clear();
    st = sea.stats();
    assert.equal(st.active, false);
    assert.equal(st.loaded, false);
    assert.equal(st.drawCalls, 0);
    assert.equal(st.visibleBig, 0);
    assert.equal(sea.group.children.length, 0);

    // Returning to pirate restores without re-fetching.
    sea.configure({ course: pirateCourses[5], look: LOOKS.pirate, waterRadius: overviewFor(pirateCourses[5]).waterRadius, tier: 'medium' });
    sea.update(0.05, camFor(pirateCourses[5]), true);
    assert.equal(requested.length, 10, 'returning to pirate reuses cached GLBs');
    assert.ok(sea.stats().active && sea.stats().loaded && sea.stats().visibleBig > 0);

    sea.dispose();
    assert.equal(sea.group.parent, null);
    assert.equal(sea.group.children.length, 0);
}

// Failed asset loader fails softly and counts 10 failures.
{
    const sea = createPirateSeaLife({ load: () => Promise.reject(Error('offline')) });
    sea.configure({ course: pirateCourses[0], look: LOOKS.pirate, waterRadius: 60, tier: 'medium' });
    await flush();
    await flush();
    const st = sea.stats();
    assert.equal(st.failures, 10);
    assert.equal(st.visibleBig, 0);
    assert.equal(st.drawCalls, 0);
    sea.update(0.05, camFor(pirateCourses[0]), true);
    sea.dispose();
}

// ---- 5. Source guards ----
for (const file of ['minigolf-sealife-paths.js', 'minigolf-sealife-shaders.js', 'minigolf-sealife.js']) {
    const text = await fs.readFile('ui/js/desktop/apps/' + file, 'utf8');
    assert.ok(!/Math\.random/.test(text), `${file} must not use Math.random`);
    assert.ok(!/https?:\/\//.test(text), `${file} must not contain external URLs`);
    assert.ok(!/ocean-life-(scene|actors)/.test(text), `${file} must not import chat scene/actors`);
}

console.log('AuraGolf Pirate Bay sea life: paths, island clearance, shark fin exposure, shaders, manifests, runtime lifecycle verified.');
