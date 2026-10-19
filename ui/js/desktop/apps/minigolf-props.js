import * as THREE from 'three';
import { placementMatrix } from './minigolf-looks.js';

// Time and a motion switch shared by every patched material of one scene.
export function createShaderState() {
    return { golfTime: { value: 0 }, golfMotion: { value: 1 } };
}

// Applies the kit material contract (glTF extras -> userData): emission tinted by
// the vertex colour, optional flame flicker, foliage sway, no transmission pass.
export function patchMaterial(material, state, height = 1) {
    const data = material?.userData || {};
    const kind = data.aurago_kind;
    if (!kind) return material;
    if (material.transmission > 0) { material.transmission = 0; material.transparent = true; material.opacity = .55; material.depthWrite = false; }
    const glow = kind === 'emissive' || kind === 'glow';
    const sway = kind === 'foliage' || kind === 'cloth' ? Number(data.aurago_sway) || 0 : 0;
    const flicker = glow ? Math.max(0, Math.min(1, Number(data.aurago_flicker) || 0)) : 0;
    if (glow) material.emissive?.setRGB(1, 1, 1);
    if (!glow && !sway) return material;
    const uniforms = { golfTime: state.golfTime, golfMotion: state.golfMotion, golfSway: { value: sway }, golfHeight: { value: Math.max(.1, Number(height) || 1) }, golfFlicker: { value: flicker } };
    material.onBeforeCompile = shader => {
        Object.assign(shader.uniforms, uniforms);
        shader.vertexShader = shader.vertexShader
            .replace('#include <common>', '#include <common>\nuniform float golfTime;\nuniform float golfMotion;\nuniform float golfSway;\nuniform float golfHeight;\nvarying float vGolfPhase;')
            .replace('#include <begin_vertex>', `#include <begin_vertex>
#ifdef USE_INSTANCING
vGolfPhase = instanceMatrix[3].x * 0.9 + instanceMatrix[3].z * 1.3;
#else
vGolfPhase = 0.0;
#endif
float golfLift = clamp(transformed.y / golfHeight, 0.0, 1.0);
golfLift = golfLift * golfLift * golfSway * golfMotion;
transformed.x += golfLift * sin(golfTime * 1.6 + vGolfPhase);
transformed.z += golfLift * 0.6 * sin(golfTime * 1.27 + vGolfPhase * 1.4);`);
        shader.fragmentShader = shader.fragmentShader
            .replace('#include <common>', '#include <common>\nuniform float golfTime;\nuniform float golfMotion;\nuniform float golfFlicker;\nvarying float vGolfPhase;')
            .replace('#include <emissivemap_fragment>', `#include <emissivemap_fragment>
#ifdef USE_COLOR
totalEmissiveRadiance *= vColor.rgb;
#endif
totalEmissiveRadiance *= 1.0 - golfFlicker * golfMotion * (0.5 + 0.5 * sin(golfTime * 11.0 + vGolfPhase) * sin(golfTime * 6.7 + vGolfPhase * 1.9));`);
    };
    material.customProgramCacheKey = () => `auragolf-kit-${glow ? 'g' : ''}${sway ? 's' : ''}`;
    return material;
}

// One InstancedMesh per prop primitive. Geometry stays owned by the cached kit;
// dispose() releases only the instance buffers and the cloned materials.
export function buildProps(kit, look, placements, { scatter = true, state }) {
    const group = new THREE.Group();
    group.name = 'aurago-props';
    const byName = new Map();
    for (const entry of placements || []) {
        if (!Array.isArray(entry) || typeof entry[0] !== 'string') continue;
        if (!byName.has(entry[0])) byName.set(entry[0], []);
        byName.get(entry[0]).push(entry);
    }
    const spinners = [], materials = [], matrix = new THREE.Matrix4(), turn = new THREE.Matrix4(), scratch = new Array(16);
    let instances = 0;
    kit.updateMatrixWorld(true);
    for (const [name, entries] of byName) {
        const node = kit.getObjectByName(`prop-${look}-${name}`);
        if (!node) continue;
        const info = node.userData?.aurago_prop || {};
        const tags = Array.isArray(info.tags) ? info.tags : [];
        if (!scatter && tags.includes('scatter')) continue;
        node.traverse(mesh => {
            if (!mesh.isMesh) return;
            const material = patchMaterial(mesh.material.clone(), state, info.height);
            materials.push(material);
            const base = mesh.matrixWorld.clone();
            const inst = new THREE.InstancedMesh(mesh.geometry, material, entries.length);
            entries.forEach((entry, index) => inst.setMatrixAt(index, matrix.fromArray(placementMatrix(entry, scratch)).multiply(base)));
            inst.name = `props-${name}`; inst.castShadow = !tags.includes('flat'); inst.receiveShadow = true;
            inst.computeBoundingSphere();
            group.add(inst);
            if (info.spin) {
                // The turning pose leaves the static bounding sphere, so these few instances skip culling.
                inst.frustumCulled = false;
                spinners.push({ inst, entries, base, axis: new THREE.Vector3().fromArray(info.spin.axis || [0, 1, 0]).normalize(), speed: Number(info.spin.speed) || .3 });
            }
        });
        instances += entries.length;
    }
    return {
        group, instances, scatter,
        update(elapsed) {
            for (let s = 0; s < spinners.length; s++) {
                const spinner = spinners[s], list = spinner.entries;
                turn.makeRotationAxis(spinner.axis, elapsed * spinner.speed);
                for (let index = 0; index < list.length; index++) spinner.inst.setMatrixAt(index, matrix.fromArray(placementMatrix(list[index], scratch)).multiply(turn).multiply(spinner.base));
                spinner.inst.instanceMatrix.needsUpdate = true;
            }
        },
        dispose() { group.removeFromParent(); for (const child of group.children) child.dispose(); for (const material of materials) material.dispose(); }
    };
}
