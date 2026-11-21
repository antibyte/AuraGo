// AuraPinball planar playfield reflection (plan R3a). A mirror camera renders only REFLECT_LAYER objects (balls, flasher
// domes, GI bulbs, bumper caps, flipper tops) into a small render target; the playfield shader adds it as a faint
// lacquer reflection. The mirror and oblique-clip math is ported from three/examples/jsm/objects/Reflector.js.
// The plane is tableRoot's local XZ plane (the playfield). Low tier: no render target, no extra pass.
import * as THREE from 'three';

export const REFLECT_LAYER = 3;

// size = share of the canvas, every = render every n-th frame, strength = fed to the playfield shader.
const TIER_REFLECT = {
    high: { size: 0.5, every: 1, strength: 0.22 },
    medium: { size: 0.25, every: 2, strength: 0.15 }
};

export function tierName(tier) {
    if (!tier) return 'high';
    if (tier.shadow === 0 && tier.lights <= 2) return 'low';
    return tier.shadow >= 2048 ? 'high' : 'medium';
}

export function createPlayfieldReflection({ renderer, scene, camera, tableRoot }) {
    const black = new THREE.DataTexture(new Uint8Array([0, 0, 0, 255]), 1, 1, THREE.RGBAFormat);
    black.needsUpdate = true;
    const mirror = new THREE.PerspectiveCamera();
    mirror.layers.set(REFLECT_LAYER);
    const matrix = new THREE.Matrix4();
    const plane = new THREE.Plane(), clip = new THREE.Vector4(), q = new THREE.Vector4();
    const pos = new THREE.Vector3(), camPos = new THREE.Vector3(), normal = new THREE.Vector3(), view = new THREE.Vector3();
    const look = new THREE.Vector3(), target = new THREE.Vector3(), rot = new THREE.Matrix4();
    const prevColor = new THREE.Color();
    let rt = null, cfg = null, frame = 0, drawCalls = 0;
    const api = {
        texture: black, matrix, strength: 0,
        get drawCalls() { return drawCalls; },
        get active() { return !!rt; },
        setTier(tier, w, h) {
            cfg = TIER_REFLECT[tierName(tier)] || null;
            if (!cfg) {
                if (rt) { rt.dispose(); rt = null; }
                api.texture = black; api.strength = 0; drawCalls = 0;
                return;
            }
            const tw = Math.max(16, Math.round(w * cfg.size)), th = Math.max(16, Math.round(h * cfg.size));
            if (!rt) rt = new THREE.WebGLRenderTarget(tw, th, { type: THREE.HalfFloatType, samples: 0, depthBuffer: true });
            else if (rt.width !== tw || rt.height !== th) rt.setSize(tw, th);
            api.texture = rt.texture; api.strength = cfg.strength; frame = 0;
        },
        // Renders the mirrored layer. Call right before the composer; `skip` holds it back (first frames: shadow maps not built yet).
        update(skip) {
            if (!rt || !cfg || skip) return;
            if (frame++ % cfg.every !== 0) return;
            tableRoot.updateWorldMatrix(true, false);
            camera.updateMatrixWorld();
            pos.setFromMatrixPosition(tableRoot.matrixWorld);
            camPos.setFromMatrixPosition(camera.matrixWorld);
            rot.extractRotation(tableRoot.matrixWorld);
            normal.set(0, 1, 0).applyMatrix4(rot);
            view.subVectors(pos, camPos);
            if (view.dot(normal) > 0) return;                       // camera below the playfield
            view.reflect(normal).negate().add(pos);
            rot.extractRotation(camera.matrixWorld);
            look.set(0, 0, -1).applyMatrix4(rot).add(camPos);
            target.subVectors(pos, look).reflect(normal).negate().add(pos);
            mirror.position.copy(view);
            mirror.up.set(0, 1, 0).applyMatrix4(rot).reflect(normal);
            mirror.lookAt(target);
            mirror.far = camera.far; mirror.near = camera.near;
            mirror.updateMatrixWorld();
            mirror.projectionMatrix.copy(camera.projectionMatrix);
            matrix.set(0.5, 0, 0, 0.5, 0, 0.5, 0, 0.5, 0, 0, 0.5, 0.5, 0, 0, 0, 1);
            matrix.multiply(mirror.projectionMatrix).multiply(mirror.matrixWorldInverse);
            // Oblique near plane: clip everything under the playfield.
            plane.setFromNormalAndCoplanarPoint(normal, pos).applyMatrix4(mirror.matrixWorldInverse);
            clip.set(plane.normal.x, plane.normal.y, plane.normal.z, plane.constant);
            const p = mirror.projectionMatrix.elements;
            q.x = (Math.sign(clip.x) + p[8]) / p[0]; q.y = (Math.sign(clip.y) + p[9]) / p[5]; q.z = -1; q.w = (1 + p[10]) / p[14];
            clip.multiplyScalar(2 / clip.dot(q));
            p[2] = clip.x; p[6] = clip.y; p[10] = clip.z + 1; p[14] = clip.w;

            const oldRT = renderer.getRenderTarget(), oldAuto = renderer.shadowMap.autoUpdate, oldAlpha = renderer.getClearAlpha();
            const oldBg = scene.background, oldXr = renderer.xr.enabled;
            renderer.getClearColor(prevColor);
            renderer.xr.enabled = false; renderer.shadowMap.autoUpdate = false; scene.background = null;
            renderer.setClearColor(0x000000, 1);
            renderer.setRenderTarget(rt); renderer.clear();
            renderer.render(scene, mirror);
            drawCalls = renderer.info.render.calls;
            renderer.setRenderTarget(oldRT); renderer.setClearColor(prevColor, oldAlpha);
            renderer.shadowMap.autoUpdate = oldAuto; renderer.xr.enabled = oldXr; scene.background = oldBg;
        },
        dispose() { if (rt) rt.dispose(); rt = null; black.dispose(); }
    };
    return api;
}
