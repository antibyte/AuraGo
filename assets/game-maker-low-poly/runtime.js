// Original MIT helpers for the pinned Three.js runtime. The game owns the clock.
import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { clone } from 'three/addons/utils/SkeletonUtils.js';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
export { THREE, OrbitControls };

const files = new Map();
const loader = new GLTFLoader();
const position = new THREE.Vector3();
const cameraPosition = new THREE.Vector3();
function fail(message) {
    const error = new Error('model_asset: ' + message);
    console.error(error.message);
    return error;
}
function disposeGLTF(gltf) {
    if (!gltf) return;
    const geometries = new Set(), materials = new Set(), textures = new Set(), skeletons = new Set();
    for (const scene of gltf.scenes) scene.traverse(node => {
        if (node.geometry) geometries.add(node.geometry);
        if (node.skeleton) skeletons.add(node.skeleton);
        for (const material of node.material ? (Array.isArray(node.material) ? node.material : [node.material]) : []) {
            materials.add(material);
            for (const value of Object.values(material)) if (value?.isTexture) textures.add(value);
        }
    });
    geometries.forEach(x => x.dispose()); materials.forEach(x => x.dispose());
    textures.forEach(x => { x.source?.data?.close?.(); x.dispose(); });
    skeletons.forEach(x => x.dispose());
}
function releaseFile(record) {
    if (--record.refs > 0) return;
    files.delete(record.key);
    record.controller.abort();
    disposeGLTF(record.gltf);
}
function acquireFile(file, base) {
    const url = new URL(file.file, base);
    if (url.origin !== base.origin || !url.pathname.startsWith(base.pathname) || !/^[a-f0-9]{64}$/.test(file.sha256)) {
        throw fail('invalid local model dependency');
    }
    const key = url.href + '#' + file.sha256;
    let record = files.get(key);
    if (record) { record.refs++; return record; }
    record = { key, refs: 1, controller: new AbortController(), gltf: null };
    record.promise = (async () => {
        const response = await fetch(url, { signal: record.controller.signal, credentials: 'same-origin' });
        if (!response.ok) throw fail('loading ' + file.file + ': HTTP ' + response.status);
        const bytes = await response.arrayBuffer();
        if (bytes.byteLength !== file.bytes) throw fail('size mismatch for ' + file.file);
        // The server verifies shipped bytes; secure contexts also verify the project copy.
        if (globalThis.crypto?.subtle) {
            const hash = [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))].map(x => x.toString(16).padStart(2, '0')).join('');
            if (hash !== file.sha256) throw fail('checksum mismatch for ' + file.file);
        }
        // Packs are self-contained GLBs. Never resolve an embedded remote URI.
        const header = new DataView(bytes);
        if (header.getUint32(0, true) !== 0x46546c67 || header.getUint32(4, true) !== 2) throw fail('invalid GLB');
        const json = JSON.parse(new TextDecoder().decode(new Uint8Array(bytes, 20, header.getUint32(12, true))));
        if ([...(json.buffers || []), ...(json.images || [])].some(x => x.uri)) throw fail('external GLB resources are forbidden');
        const gltf = await loader.parseAsync(bytes, '');
        if (record.refs === 0) { disposeGLTF(gltf); throw new DOMException('Disposed', 'AbortError'); }
        record.gltf = gltf;
        return gltf;
    })();
    files.set(key, record);
    return record;
}

export async function loadAsset(manifest, id, baseURL, options = {}) {
    const model = manifest.assets?.find(x => x.id === id);
    if (manifest.kind !== 'model3d' || !model?.lods?.length) throw fail('unknown model ' + id);
    const base = new URL(baseURL, document.baseURI);
    if (base.origin !== new URL(document.baseURI).origin || !['http:', 'https:'].includes(base.protocol)) throw fail('model assets must use the local game origin');
    if (!base.pathname.endsWith('/')) throw fail('baseURL must end with /');
    const records = [];
    let released = false;
    const release = () => { if (!released) { released = true; records.forEach(releaseFile); } };
    try {
        options.signal?.throwIfAborted();
        for (const file of model.lods) records.push(acquireFile(file, base));
        if (model.animation_library) records.push(acquireFile(model.animation_library, base));
        options.signal?.addEventListener('abort', release, { once: true });
        const decoded = await Promise.all(records.map(x => x.promise));
        options.signal?.throwIfAborted();
        if (released) throw new DOMException('Disposed', 'AbortError');
        const levels = decoded.slice(0, model.lods.length);
        const clips = (model.animation_library ? decoded.at(-1) : levels[0]).animations;
        for (const declared of model.animations || []) {
            if (!clips.some(x => x.name === declared.id)) throw fail('missing clip ' + declared.id + ' for ' + id);
        }
        return { model, levels, clips, users: 1, released: false, release };
    } catch (error) { release(); throw error; }
    finally { options.signal?.removeEventListener('abort', release); }
}

function dropAssetReference(asset) {
    if (asset.released) return;
    if (--asset.users === 0) { asset.released = true; asset.release(); }
}

export function releaseAsset(asset) {
    if (asset.ownerReleased) return;
    asset.ownerReleased = true;
    dropAssetReference(asset);
}

export function createInstance(asset, options = {}) {
    if (asset.released) throw fail('asset already released');
    const root = new THREE.Group();
    const levels = asset.levels.map(data => clone(data.scene));
    levels.forEach((level, index) => {
        level.visible = index === 0;
        level.traverse(node => { if (node.isMesh) { node.castShadow = options.shadows !== false; node.receiveShadow = true; } });
        root.add(level);
    });
    const mixers = asset.clips.length ? levels.map(level => new THREE.AnimationMixer(level)) : [];
    asset.users++;
    return { root, levels, mixers, asset, actions: [], current: null, time: 0, level: 0,
        disposed: false, forcedLOD: null, attachments: 0, onEvent: options.onEvent || null };
}

export function playAction(instance, name, options = {}) {
    if (instance.disposed) throw fail('instance already disposed');
    const declaration = instance.asset.model.animations.find(x => x.id === name);
    const clip = instance.asset.clips.find(x => x.name === name);
    if (!declaration || !clip) throw fail('unknown action ' + name + ' for ' + instance.asset.model.id);
    if (instance.current?.id === name && !options.restart) return;
    const fade = Math.max(0, Math.min(1, options.fade ?? .15));
    instance.actions.forEach(action => action.fadeOut(fade));
    instance.actions = instance.mixers.map(mixer => {
        const action = mixer.clipAction(clip);
        action.reset().setLoop(declaration.loop ? THREE.LoopRepeat : THREE.LoopOnce, declaration.loop ? Infinity : 1);
        action.clampWhenFinished = !declaration.loop;
        action.fadeIn(fade).play();
        return action;
    });
    instance.current = declaration; instance.time = 0;
}

export function updateInstance(instance, seconds, camera) {
    if (instance.disposed) return;
    const dt = Math.max(0, Math.min(.1, Number.isFinite(seconds) ? seconds : 0));
    const previous = instance.time; instance.time += dt;
    instance.mixers.forEach(mixer => mixer.update(dt));
    if (instance.onEvent && instance.current) {
        const { duration, loop, events = [] } = instance.current;
        for (const event of events) {
            const before = loop ? Math.floor((previous - event.time) / duration) : previous >= event.time ? 0 : -1;
            const after = loop ? Math.floor((instance.time - event.time) / duration) : instance.time >= event.time ? 0 : -1;
            if (after > before) instance.onEvent(event.name, instance);
        }
    }
    if (instance.levels.length) {
        let level = instance.attachments ? 0 : instance.forcedLOD;
        if (level == null && camera) {
            instance.root.getWorldPosition(position);
            const distance = camera.getWorldPosition(cameraPosition).distanceTo(position);
            const b = instance.asset.model.bounds;
            const size = Math.max(...b.max.map((x, i) => x - b.min[i])) * instance.root.scale.length() / Math.sqrt(3);
            level = distance > size * 28 ? 2 : distance > size * 12 ? 1 : 0;
        }
        level = Math.max(0, Math.min(instance.levels.length - 1, level ?? 0));
        if (level !== instance.level) {
            instance.levels[instance.level].visible = false; instance.levels[level].visible = true;
            instance.level = level;
        }
    }
}

export function attachToSocket(instance, id, object) {
    const socket = instance.asset.model.sockets?.find(x => x.id === id);
    if (!socket) throw fail('unknown socket ' + id);
    // Attachments belong to the full-detail rig; keep it active while attached.
    const node = instance.levels[0].getObjectByName(socket.node);
    if (!node) throw fail('missing socket node ' + socket.node);
    instance.attachments++; node.add(object);
    let detached = false;
    return () => {
        if (detached) return;
        detached = true; node.remove(object); instance.attachments--;
    };
}

export function setPart(instance, name, value) {
    const part = instance.asset.model.moving_parts?.find(x => x.node === name);
    if (!part || !Number.isFinite(value)) throw fail('invalid moving part ' + name);
    const axis = new THREE.Vector3(...part.axis).normalize();
    for (const level of instance.levels) {
        const node = level.getObjectByName(part.node);
        if (!node) throw fail('missing moving node ' + part.node);
        node.userData.polyRest ??= { position: node.position.clone(), quaternion: node.quaternion.clone() };
        if (['lift', 'slide'].includes(part.kind)) node.position.copy(node.userData.polyRest.position).addScaledVector(axis, value);
        else node.quaternion.copy(node.userData.polyRest.quaternion).multiply(new THREE.Quaternion().setFromAxisAngle(axis, value));
    }
}

export function createInstances(asset, matrices, lod = 0) {
    if (asset.released || asset.model.rig || asset.model.animations.length || asset.model.moving_parts.length) throw fail('instancing requires static assets');
    const source = asset.levels[Math.min(lod, asset.levels.length - 1)].scene;
    const root = new THREE.Group(), transform = new THREE.Matrix4();
    source.updateMatrixWorld(true);
    source.traverse(node => {
        if (!node.isMesh) return;
        const mesh = new THREE.InstancedMesh(node.geometry, node.material, matrices.length);
        matrices.forEach((matrix, i) => mesh.setMatrixAt(i, transform.copy(matrix).multiply(node.matrixWorld)));
        mesh.instanceMatrix.needsUpdate = true; mesh.castShadow = true; mesh.receiveShadow = true;
        mesh.computeBoundingSphere(); root.add(mesh);
    });
    asset.users++;
    return { root, asset, levels: [], mixers: [], actions: [], current: null, time: 0, disposed: false };
}

export function disposeInstance(instance) {
    if (instance.disposed) return;
    instance.disposed = true; instance.root.removeFromParent();
    instance.mixers.forEach(mixer => { mixer.stopAllAction(); mixer.uncacheRoot(mixer.getRoot()); });
    const skeletons = new Set();
    instance.root.traverse(node => {
        if (node.skeleton) skeletons.add(node.skeleton);
        if (node.isInstancedMesh) node.dispose();
    });
    skeletons.forEach(skeleton => skeleton.dispose());
    dropAssetReference(instance.asset);
}

export function assetCacheSize() { return files.size; }
