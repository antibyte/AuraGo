import * as THREE from 'three';
import { placementMatrix } from './minigolf-looks.js';
import { clothPinFactors, edgeSamples } from './minigolf-shared.js';

// Time and a motion switch shared by every patched material of one scene.
export function createShaderState() {
    return { golfTime: { value: 0 }, golfMotion: { value: 1 } };
}

// Applies the kit material contract (glTF extras -> userData): emission tinted by
// the vertex colour, optional flame flicker, foliage sway, falling water, no
// transmission pass. `pinned` cloth reads the golfPin vertex weight (0 on the
// pole, 1 at the fly) instead of bending with height, which would lift the
// hoist off the pole.
export function patchMaterial(material, state, height = 1, pinned = false) {
    const data = material?.userData || {};
    const kind = data.aurago_kind;
    if (!kind) return material;
    if (material.transmission > 0) { material.transmission = 0; material.transparent = true; material.opacity = .55; material.depthWrite = false; }
    const glow = kind === 'emissive' || kind === 'glow';
    const sway = kind === 'foliage' || kind === 'cloth' ? Number(data.aurago_sway) || 0 : 0;
    const flicker = glow ? Math.max(0, Math.min(1, Number(data.aurago_flicker) || 0)) : 0;
    // Falling water: a thin sheet that is lit from both sides and lets the rock
    // show through it, with filaments scrolling down at the baked speed.
    const flow = Math.max(0, Number(data.aurago_flow) || 0);
    const clothPin = pinned && kind === 'cloth' && sway > 0;
    if (flow) { material.side = THREE.DoubleSide; material.transparent = true; material.opacity = .88; material.depthWrite = false; }
    // The glTF emissiveFactor (glow strength) and emissive strength (emissiveIntensity) stay as exported;
    // the shader below tints the result with the vertex colour.
    if (!glow && !sway && !flow) return material;
    const uniforms = { golfTime: state.golfTime, golfMotion: state.golfMotion, golfSway: { value: sway }, golfHeight: { value: Math.max(.1, Number(height) || 1) }, golfFlicker: { value: flicker }, golfFlow: { value: flow } };
    const lift = clothPin
        ? 'float golfLift = golfPin * golfPin * golfSway * golfMotion;'
        : `float golfLift = clamp(transformed.y / golfHeight, 0.0, 1.0);
golfLift = golfLift * golfLift * golfSway * golfMotion;`;
    material.onBeforeCompile = shader => {
        Object.assign(shader.uniforms, uniforms);
        shader.vertexShader = shader.vertexShader
            .replace('#include <common>', `#include <common>\nuniform float golfTime;\nuniform float golfMotion;\nuniform float golfSway;\nuniform float golfHeight;\nuniform float golfFlow;\nvarying float vGolfPhase;\n${flow ? 'varying vec3 vFlowWorld;' : ''}${clothPin ? '\nattribute float golfPin;' : ''}`)
            .replace('#include <begin_vertex>', `#include <begin_vertex>
#ifdef USE_INSTANCING
vGolfPhase = instanceMatrix[3].x * 0.9 + instanceMatrix[3].z * 1.3;
#else
vGolfPhase = 0.0;
#endif
${lift}
transformed.x += golfLift * sin(golfTime * 1.6 + vGolfPhase);
transformed.z += golfLift * 0.6 * sin(golfTime * 1.27 + vGolfPhase * 1.4);
${flow ? `transformed.x += sin(golfTime * 2.3 + transformed.y * 4.7 + vGolfPhase) * 0.016;
transformed.z += cos(golfTime * 1.9 + transformed.y * 3.9) * 0.013;` : ''}`)
            .replace('#include <project_vertex>', `#include <project_vertex>
${flow ? `#ifdef USE_INSTANCING
vFlowWorld = (modelMatrix * instanceMatrix * vec4(transformed, 1.0)).xyz;
#else
vFlowWorld = (modelMatrix * vec4(transformed, 1.0)).xyz;
#endif` : ''}`);
        shader.fragmentShader = shader.fragmentShader
            .replace('#include <common>', `#include <common>\nuniform float golfTime;\nuniform float golfMotion;\nuniform float golfFlicker;\nuniform float golfFlow;\nvarying float vGolfPhase;\n${flow ? `varying vec3 vFlowWorld;
float flowHash(vec3 p) {
    p = fract(p * 0.3183099 + vec3(0.11, 0.17, 0.23));
    p *= 17.0;
    return fract(p.x * p.y * p.z * (p.x + p.y + p.z));
}
float flowNoise(vec3 x) {
    vec3 i = floor(x), f = fract(x);
    f = f * f * (3.0 - 2.0 * f);
    return mix(mix(mix(flowHash(i), flowHash(i + vec3(1.0, 0.0, 0.0)), f.x),
                   mix(flowHash(i + vec3(0.0, 1.0, 0.0)), flowHash(i + vec3(1.0, 1.0, 0.0)), f.x), f.y),
               mix(mix(flowHash(i + vec3(0.0, 0.0, 1.0)), flowHash(i + vec3(1.0, 0.0, 1.0)), f.x),
                   mix(flowHash(i + vec3(0.0, 1.0, 1.0)), flowHash(i + vec3(1.0, 1.0, 1.0)), f.x), f.y), f.z);
}` : ''}`)
            .replace('#include <emissivemap_fragment>', `#include <emissivemap_fragment>
#ifdef USE_COLOR
totalEmissiveRadiance *= vColor.rgb;
#endif
totalEmissiveRadiance *= 1.0 - golfFlicker * golfMotion * (0.5 + 0.5 * sin(golfTime * 11.0 + vGolfPhase) * sin(golfTime * 6.7 + vGolfPhase * 1.9));
${flow ? `// The filaments travel down the sheet: sampling the noise above the
// world height moves every bright streak downwards over time.
float flowT = golfTime * golfFlow * golfMotion;
vec3 flowP = vec3(vFlowWorld.x * 8.5, vFlowWorld.y * 0.9 + flowT * 1.7, vFlowWorld.z * 8.5);
float flowN = flowNoise(flowP);
float flowM = flowNoise(flowP * 2.6 + vec3(3.7, 0.0, 9.1));
float flowFilament = smoothstep(0.52, 0.88, flowN * 0.6 + flowM * 0.5);
float flowSpray = smoothstep(0.8, 0.99, flowM);
diffuseColor.rgb *= 0.93 + 0.15 * flowN;
diffuseColor.rgb = mix(diffuseColor.rgb, vec3(0.95, 1.0, 0.99), flowFilament * 0.55);
diffuseColor.a *= 0.86 + 0.14 * flowN;
totalEmissiveRadiance = mix(totalEmissiveRadiance, vec3(0.86, 0.97, 0.96), flowFilament * 0.4);
totalEmissiveRadiance += vec3(0.3, 0.45, 0.48) * flowSpray * 0.25;` : ''}`);
    };
    material.customProgramCacheKey = () => `auragolf-kit-${glow ? 'g' : ''}${sway ? 's' : ''}${clothPin ? 'p' : ''}${flow ? 'w' : ''}`;
    return material;
}

function worldPoints(mesh) {
    const attribute = mesh.geometry?.getAttribute?.('position');
    if (!attribute) return [];
    const point = new THREE.Vector3(), out = new Array(attribute.count);
    for (let i = 0; i < attribute.count; i++) out[i] = point.fromBufferAttribute(attribute, i).applyMatrix4(mesh.matrixWorld).toArray();
    return out;
}

function anchorSamples(mesh) {
    const index = mesh.geometry?.getIndex?.();
    const indices = index ? Array.from({ length: index.count }, (_, i) => index.getX(i)) : null;
    return edgeSamples(worldPoints(mesh), indices);
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
        const meshes = [];
        node.traverse(mesh => { if (mesh.isMesh) meshes.push(mesh); });
        const hasCloth = meshes.some(mesh => mesh.material?.userData?.aurago_kind === 'cloth' && (Number(mesh.material?.userData?.aurago_sway) || 0) > 0);
        const anchors = [];
        if (hasCloth) for (const mesh of meshes) if (mesh.material?.userData?.aurago_kind !== 'cloth') for (const sample of anchorSamples(mesh)) anchors.push(sample);
        for (const mesh of meshes) {
            const data = mesh.material?.userData || {};
            const pinCloth = data.aurago_kind === 'cloth' && (Number(data.aurago_sway) || 0) > 0 && anchors.length > 0;
            if (pinCloth && !mesh.geometry.getAttribute('golfPin')) {
                mesh.geometry.setAttribute('golfPin', new THREE.BufferAttribute(Float32Array.from(clothPinFactors(worldPoints(mesh), anchors)), 1));
            }
            const material = patchMaterial(mesh.material.clone(), state, info.height, pinCloth && !!mesh.geometry.getAttribute('golfPin'));
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
        }
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
