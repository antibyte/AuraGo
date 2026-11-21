// GLB model loading for the table toys (plan R5 / C7, AM2). One file per table, key `toys`.
//   loadModels(glbMap, { assetURL, signal, env, tier, timeoutMs }) -> Map<key, { scene, nodes: Map<name, Object3D> }>
// A failed or missing file is simply absent from the map (one console.warn); an aborted signal rejects like the other loaders.
// cloneRoot(models, key, name) returns an independent copy of one root node (geometry and materials stay shared with the
// parsed scene) together with a name lookup, or null when the key or root is missing.
import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { ASSET_ROOT } from '../config.js';

const abortError = () => Object.assign(new Error('cancelled'), { name: 'AbortError' });

function postProcess(scene, { env, tier }) {
    const shadows = !!tier && tier !== 'low';
    const nodes = new Map(), seen = new Set();
    scene.traverse(o => {
        if (o.name) nodes.set(o.name, o);
        if (!o.isMesh) return;
        o.castShadow = shadows; o.receiveShadow = false; o.frustumCulled = false;
        for (const m of Array.isArray(o.material) ? o.material : [o.material]) {
            if (!m || seen.has(m)) continue;
            seen.add(m);
            const name = m.name || '';
            if ('envMap' in m && env) { m.envMap = env; m.envMapIntensity = 0.55; }
            if (name.startsWith('glow_')) {
                m.toneMapped = false;
                if (m.emissive && m.emissive.getHex() === 0) m.emissive.copy(m.color);
            } else if (name.startsWith('glass_')) {
                if (tier === 'low') { m.transparent = false; m.opacity = 1; } else { m.transparent = true; m.depthWrite = false; o.renderOrder = 2; }
            }
            m.needsUpdate = true;
        }
    });
    return nodes;
}

async function loadOne(url, opts) {
    const { signal, timeoutMs = 8000 } = opts;
    const ctl = new AbortController(), onAbort = () => ctl.abort();
    if (signal) signal.addEventListener('abort', onAbort, { once: true });
    const timer = setTimeout(() => ctl.abort(), timeoutMs);
    try {
        const res = await fetch(url, { signal: ctl.signal });
        if (!res.ok) throw new Error('http ' + res.status);
        const gltf = await new GLTFLoader().parseAsync(await res.arrayBuffer(), '');
        return { scene: gltf.scene, nodes: postProcess(gltf.scene, opts) };
    } finally {
        clearTimeout(timer);
        if (signal) signal.removeEventListener('abort', onAbort);
    }
}

export async function loadModels(glbMap, { assetURL = p => p, signal, env, tier, timeoutMs = 8000 } = {}) {
    const out = new Map();
    if (!glbMap || typeof glbMap !== 'object') return out;
    for (const [key, file] of Object.entries(glbMap)) {
        if (signal && signal.aborted) throw abortError();
        if (typeof file !== 'string' || !file) continue;
        try {
            out.set(key, await loadOne(assetURL(ASSET_ROOT + file), { signal, env, tier, timeoutMs }));
        } catch (e) {
            if (signal && signal.aborted) { disposeModels(out); throw abortError(); }
            console.warn('pinball: model "' + key + '" not loaded, using the procedural fallback (' + ((e && e.message) || e) + ')');
        }
    }
    return out;
}

export function cloneRoot(models, key, name) {
    const entry = models && models.get && models.get(key), src = entry && entry.nodes.get(name);
    if (!src) return null;
    const root = src.clone(true);
    root.position.set(0, 0, 0); root.rotation.set(0, 0, 0); root.scale.set(1, 1, 1);
    const find = n => (n === name ? root : root.getObjectByName(n) || null);
    return { root, find };
}

export function disposeModels(map) {
    if (!map || !map.forEach) return;
    const done = new Set();
    map.forEach(entry => {
        entry.scene.traverse(o => {
            if (o.geometry && !done.has(o.geometry)) { done.add(o.geometry); o.geometry.dispose(); }
            for (const m of Array.isArray(o.material) ? o.material : (o.material ? [o.material] : [])) {
                if (done.has(m)) continue;
                done.add(m);
                for (const [k, v] of Object.entries(m)) if (v && v.isTexture && k !== 'envMap') v.dispose(); // envMap belongs to the scene
                m.dispose();
            }
        });
    });
    map.clear();
}

export default loadModels;
