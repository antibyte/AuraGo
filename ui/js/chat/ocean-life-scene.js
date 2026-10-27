import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { SPECIES } from './ocean-life-species.js';
import { createRng } from './ocean-life-boids.js';
import {
    createSharedUniforms,
    createCreatureMaterial,
    createJellyMaterial,
    createMantaMaterial
} from './ocean-life-shaders.js';
import {
    createView,
    createSchoolActor,
    createCruiserActor,
    createDrifterActor,
    createWhaleActor
} from './ocean-life-actors.js';

// Ocean chat theme: Blender-authored sea life (assets/chat-ocean-life) drifting behind the messages.
// Geometry is baked to the species length, all animation runs in the vertex shaders. The bridge
// (ui/js/chat/ocean-life.js) owns the canvas, the DOM and the frame loop.

const FOV_HALF_DEGREES = 25;
const STILL_FRAME_SECONDS = 12;
const STILL_FRAME_STEP = 0.05;
const MAX_STEP = 0.1;
const IDENTITY = new THREE.Matrix4();
// What the shader factories need from three.js. Passing the whole namespace would stop the bundler
// from tree shaking it (+170 KB).
const SHADER_THREE = {
    Vector3: THREE.Vector3,
    ShaderMaterial: THREE.ShaderMaterial,
    DoubleSide: THREE.DoubleSide,
    FrontSide: THREE.FrontSide
};
const IDLE_INPUT = { pointer: { x: 0.5, y: 0.5, active: false }, busy: false };
// Tiefsee models are authored with the nose toward -Z; the sea life frame wants +Z.
const MODEL_FIXUPS = { manta: geometry => geometry.rotateY(Math.PI) };

function now() {
    return typeof performance !== 'undefined' ? performance.now() : Date.now();
}

async function loadModel(loader, env, spec) {
    const base = spec.base === 'abyss' ? env.abyssBase : env.modelBase;
    const url = env.versionedURL ? env.versionedURL(base + spec.file) : base + spec.file;
    const gltf = await loader.loadAsync(url);
    gltf.scene.updateMatrixWorld(true);
    const parts = [];
    gltf.scene.traverse(obj => {
        if (!obj.isMesh) return;
        const geometry = obj.geometry;
        if (!obj.matrixWorld.equals(IDENTITY)) geometry.applyMatrix4(obj.matrixWorld);
        parts.push({ name: (obj.material && obj.material.name) || 'body', geometry });
        if (obj.material && obj.material.dispose) obj.material.dispose();
    });
    const discard = () => parts.forEach(part => part.geometry.dispose());
    if (!parts.length) throw new Error(spec.file + ' has no meshes');
    for (const part of parts) {
        for (const attribute of ['position', 'normal', 'uv', 'uv1']) {
            if (!part.geometry.getAttribute(attribute)) {
                discard();
                throw new Error(spec.file + ' primitive "' + part.name + '" lacks ' + attribute);
            }
        }
    }
    if (MODEL_FIXUPS[spec.id]) parts.forEach(part => MODEL_FIXUPS[spec.id](part.geometry));

    const box = new THREE.Box3();
    parts.forEach(part => {
        part.geometry.computeBoundingBox();
        box.union(part.geometry.boundingBox);
    });
    const depth = box.max.z - box.min.z;
    if (!(depth > 1e-6)) {
        discard();
        throw new Error(spec.file + ' has no depth');
    }
    // bake the real-world size so instance matrices only carry the per-animal size jitter
    const scale = spec.length / depth;
    parts.forEach(part => {
        part.geometry.scale(scale, scale, scale);
        part.geometry.computeBoundingBox();
        part.geometry.computeBoundingSphere();
        // three maps TEXCOORD_1 to 'uv1'; the shaders read it as aAnim
        part.geometry.setAttribute('aAnim', part.geometry.getAttribute('uv1'));
        part.geometry.deleteAttribute('uv1');
    });
    return { spec, parts, scale, front: box.max.z * scale };
}

export async function createOceanLife(env) {
    const tier = env.quality === 'medium' || env.quality === 'low' ? env.quality : 'high';
    const rng = createRng((env.seed >>> 0) || 1);

    // The renderer comes first: without a WebGL context this throws before a single model is fetched or parsed.
    const renderer = new THREE.WebGLRenderer({
        canvas: env.canvas,
        alpha: true,
        antialias: true,
        premultipliedAlpha: true,
        powerPreference: 'high-performance',
        stencil: false
    });
    renderer.setClearColor(0x000000, 0);
    renderer.setPixelRatio(env.dpr || 1);
    renderer.setSize(env.width, env.height, false);
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.toneMapping = THREE.NoToneMapping;

    const loader = new GLTFLoader();
    const wanted = SPECIES.filter(spec => (spec.counts[tier] || 0) > 0);
    const results = await Promise.allSettled(wanted.map(spec => loadModel(loader, env, spec)));
    const models = [];
    results.forEach((result, i) => {
        if (result.status === 'fulfilled') models.push(result.value);
        else console.warn('[OceanLife] ' + wanted[i].id + ' unavailable:', result.reason);
    });
    if (!models.length) {
        renderer.dispose();
        throw new Error('[OceanLife] no sea life model could be loaded');
    }

    const view = createView(env.width, env.height);
    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(50, view.aspect, 0.5, 60);
    function updateCamera() {
        // keep the horizontal angle at 50 degrees, so the frustum width at depth d is 2 d tan(25 deg)
        camera.aspect = view.aspect;
        camera.fov = 2 * Math.atan(Math.tan(FOV_HALF_DEGREES * Math.PI / 180) / view.aspect) * 180 / Math.PI;
        camera.updateProjectionMatrix();
    }
    updateCamera();

    const shared = createSharedUniforms(SHADER_THREE);
    const ctx = {
        scene,
        view,
        rng,
        owned: [],
        busy: 0,
        pointer: { active: false, nx: 0, ny: 0 },
        repulsors: [],
        // seconds until the first whale pass (debug seam, see the bridge); anything else keeps the 60-120 s default
        firstWhaleDelay: env.firstWhaleDelay,
        makeMaterial(model, partName, extra) {
            const spec = model.spec;
            const fog = spec.fog == null ? 1 : spec.fog;
            const haze = spec.haze == null ? 1 : spec.haze;
            if (spec.shader === 'jelly') return createJellyMaterial(SHADER_THREE, { part: partName, shared, fog, haze });
            if (spec.shader === 'manta') {
                if (!model.material) {
                    model.material = createMantaMaterial(SHADER_THREE, { shared, flap: extra && extra.flap || 1.2, unit: model.scale, fog, haze });
                }
                return model.material;
            }
            return createCreatureMaterial(SHADER_THREE, {
                shader: spec.shader,
                variant: spec.variant,
                part: partName,
                shared,
                swimK: spec.swim.k,
                length: spec.length,
                front: model.front,
                fog,
                haze
            });
        }
    };

    const actors = [];
    models.forEach(model => {
        const spec = model.spec;
        const count = spec.counts[tier];
        if (spec.kind === 'school') actors.push(createSchoolActor(ctx, model, count));
        else if (spec.kind === 'cruiser') actors.push(createCruiserActor(ctx, model));
        else if (spec.kind === 'drifter') actors.push(createDrifterActor(ctx, model, count));
        else if (spec.kind === 'rare') actors.push(createWhaleActor(ctx, model));
    });
    // sharks scatter the schools
    actors.forEach(actor => { if (actor.repulsor) ctx.repulsors.push(actor.repulsor); });
    const whale = actors.find(actor => actor.id === 'whale') || null;
    const counts = {};
    let creatures = 0;
    actors.forEach(actor => {
        counts[actor.id] = actor.count;
        creatures += actor.count;
    });

    let frames = 0;
    let frameMs = 0;
    let animated = true;
    let disposed = false;
    let stillDone = false;
    let layoutDirty = false;

    function simulate(t, dt, input) {
        const step = Math.min(Math.max(dt, 0), MAX_STEP);
        const source = input || IDLE_INPUT;
        const pointer = source.pointer;
        ctx.pointer.active = !!(pointer && pointer.active);
        if (ctx.pointer.active) {
            // the pointer ray through the camera; school repulsors sit on it at their own depth
            ctx.pointer.nx = (2 * pointer.x - 1) * view.tanHalf;
            ctx.pointer.ny = (1 - 2 * pointer.y) * view.tanHalf / view.aspect;
        }
        ctx.busy += ((source.busy ? 1 : 0) - ctx.busy) * Math.min(1, step * 1.5);
        shared.uTime.value = t;
        shared.uBusy.value = ctx.busy;
        for (let i = 0; i < actors.length; i++) actors[i].update(t, step);
    }

    function render() {
        renderer.render(scene, camera);
        frames++;
    }

    function frame(t, dt, input) {
        if (disposed) return;
        const started = now();
        simulate(t, dt, input);
        render();
        const elapsed = now() - started;
        frameMs = frames <= 1 ? elapsed : frameMs + (elapsed - frameMs) * 0.1;
    }

    function resize(width, height, dpr) {
        if (disposed || !(width > 0 && height > 0)) return;
        layoutDirty = true;
        view.resize(width, height);
        renderer.setPixelRatio(dpr || 1);
        renderer.setSize(width, height, false);
        updateCamera();
        actors.forEach(actor => actor.refresh());
    }

    // Reduced motion: spread the animals out by simulating 12 s, render once, stay still. Calling it
    // again (the bridge does after a resize) re-renders the same picture; only a changed layout gets
    // 2 more simulated seconds so the animals settle inside the new bounds.
    function stillFrame() {
        if (disposed) return;
        if (!stillDone) {
            const steps = Math.round(STILL_FRAME_SECONDS / STILL_FRAME_STEP);
            for (let k = 1; k <= steps; k++) simulate(k * STILL_FRAME_STEP, STILL_FRAME_STEP, IDLE_INPUT);
            stillDone = true;
        } else if (layoutDirty) {
            const steps = Math.round(2 / STILL_FRAME_STEP);
            for (let k = 1; k <= steps; k++) simulate(STILL_FRAME_SECONDS + k * STILL_FRAME_STEP, STILL_FRAME_STEP, IDLE_INPUT);
        }
        layoutDirty = false;
        render();
        animated = false;
    }

    function stats() {
        return {
            animated,
            frames,
            creatures,
            species: Object.assign({}, counts),
            frameMs,
            whaleVisible: !!(whale && whale.visible),
            nextWhaleAt: whale ? whale.nextPass : null,
            quality: tier
        };
    }

    function dispose() {
        if (disposed) return;
        disposed = true;
        const done = new Set();
        ctx.owned.forEach(item => {
            if (!item || done.has(item) || !item.dispose) return;
            done.add(item);
            item.dispose();
        });
        scene.clear();
        // the bridge removes the canvas, so the WebGL context is left to the garbage collector
        renderer.dispose();
    }

    return { frame, resize, stillFrame, stats, dispose };
}
