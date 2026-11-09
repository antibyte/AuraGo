// AuraPinball render core (plan 3.13 / 7): WebGL2 renderer, tone mapping + bloom composer, tiers, playfield, cabinet,
// lighting, cameras. T13 attaches mechanics through scene.addMechanics(builder) (see the contract below).
import * as THREE from 'three';
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/addons/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/addons/postprocessing/UnrealBloomPass.js';
import { OutputPass } from 'three/addons/postprocessing/OutputPass.js';
import { ASSET_ROOT } from '../config.js';
import { FIELD_W, FIELD_H } from '../lights/shapes.js';
import { VISUALS } from '../tables/visuals.js';
import { TIERS, tierOf, createMaterials, disposeMaterials } from './materials.js';
import { paintPlayfieldArt, createLampTextures, createPlayfieldMaterial, buildWallGeometry } from './playfield.js';
import { createEnvironment, createLights, BALL_RADIUS } from './lighting.js';
import { buildCabinet, buildRoom } from './cabinet.js';
import { createCameraRig } from './camera.js';
import { buildMechanics } from './builders.js';

const TILT = 6.5 * Math.PI / 180;
const abortError = () => Object.assign(new Error('cancelled'), { name: 'AbortError' });

async function loadBitmap(url, signal) {
    try {
        const res = await fetch(url, { signal });
        if (!res.ok) return null;
        return await createImageBitmap(await res.blob());
    } catch (e) { if (signal && signal.aborted) throw abortError(); return null; }
}

// Backglass as a canvas (flipY texture) with a faint glass sheen; falls back to an accent gradient.
function backglassCanvas(img, palette) {
    const w = 1024, h = img ? Math.round(w * img.height / img.width) : 576, c = document.createElement('canvas');
    c.width = w; c.height = h;
    const g = c.getContext('2d');
    if (img) g.drawImage(img, 0, 0, w, h);
    else { const gr = g.createLinearGradient(0, 0, 0, h); gr.addColorStop(0, '#05070b'); gr.addColorStop(1, palette.accent || '#37d9ff'); g.fillStyle = gr; g.fillRect(0, 0, w, h); }
    const sheen = g.createLinearGradient(0, 0, w, h);
    sheen.addColorStop(0.2, 'rgba(255,255,255,0)'); sheen.addColorStop(0.32, 'rgba(255,255,255,.06)'); sheen.addColorStop(0.42, 'rgba(255,255,255,0)');
    g.fillStyle = sheen; g.fillRect(0, 0, w, h);
    const vg = g.createRadialGradient(w / 2, h / 2, h * 0.5, w / 2, h / 2, w * 0.62); vg.addColorStop(0, 'rgba(0,0,0,0)'); vg.addColorStop(1, 'rgba(0,0,0,.45)');
    g.fillStyle = vg; g.fillRect(0, 0, w, h);
    return { canvas: c };
}

export async function createScene(host, { tableId, layout, settings = {}, assetURL = p => p, signal, tr } = {}) {
    if (signal && signal.aborted) throw abortError();
    let tier = tierOf(settings.quality), quality = TIERS[settings.quality] ? settings.quality : 'high';
    let renderer;
    try {
        renderer = new THREE.WebGLRenderer({ antialias: false, alpha: false, powerPreference: 'high-performance' });
        if (!renderer.getContext()) throw new Error('no context');
    } catch (_) { throw new Error('webgl_error'); }
    const canvas = renderer.domElement;
    let disposed = false;
    const fail = e => { try { renderer.dispose(); } catch (_) { /* ignore */ } throw e; };

    // ---- assets ----
    let manifest;
    try {
        const res = await fetch(assetURL(ASSET_ROOT + 'manifest.json'), { signal });
        if (!res.ok) throw new Error('load_error');
        manifest = await res.json();
    } catch (e) { if (signal && signal.aborted) fail(abortError()); fail(e && e.message === 'load_error' ? e : new Error('load_error')); }
    const list = Array.isArray(manifest.tables) ? manifest.tables : Object.entries(manifest.tables || {}).map(([id, v]) => ({ id, ...v }));
    const entry = list.find(t => t.id === tableId) || {};
    let underlay = null, glassImg = null;
    try {
        [underlay, glassImg] = await Promise.all([
            (entry.underlay || entry.playfield) ? loadBitmap(assetURL(ASSET_ROOT + (entry.underlay || entry.playfield)), signal) : null,
            entry.backglass ? loadBitmap(assetURL(ASSET_ROOT + entry.backglass), signal) : null
        ]);
    } catch (e) { fail(e); }
    if (signal && signal.aborted) { for (const b of [underlay, glassImg]) if (b && b.close) b.close(); fail(abortError()); }

    // ---- renderer / scene ----
    const palette = { accent: layout.meta && layout.meta.accent, secondary: layout.meta && layout.meta.secondary };
    const maxAniso = renderer.capabilities.getMaxAnisotropy();
    const aniso = () => Math.min(maxAniso, tier.aniso);
    renderer.setClearColor('#03050a'); renderer.toneMapping = THREE.ACESFilmicToneMapping; renderer.toneMappingExposure = 1.05;
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.shadowMap.type = THREE.PCFShadowMap; renderer.shadowMap.enabled = tier.shadow > 0;
    canvas.className = 'pb-canvas'; canvas.setAttribute('aria-hidden', 'true'); canvas.style.width = '100%'; canvas.style.height = '100%'; canvas.style.display = 'block';
    host.appendChild(canvas);

    const scene = new THREE.Scene(); scene.background = new THREE.Color('#03050a');
    const camera = new THREE.PerspectiveCamera(36, 1, 0.05, 40);
    const tableRoot = new THREE.Group(); tableRoot.name = 'tableRoot'; tableRoot.rotation.x = TILT; scene.add(tableRoot);
    const mechanicsGroup = new THREE.Group(); mechanicsGroup.name = 'mechanics'; tableRoot.add(mechanicsGroup);
    let env = createEnvironment(renderer, palette);
    const mats = createMaterials({ envMap: env.texture, palette, aniso: aniso() });
    const lamps0 = { texture: createLampTextures(layout) };

    const forceFallback = /[?&]shader=fallback/.test((typeof location !== 'undefined' && location.search) || '') || settings.forceFallback === true;
    let art = paintPlayfieldArt({ layout, tr, tableId, size: tier.art, underlay, aniso: aniso() });
    const pf = createPlayfieldMaterial({ artTex: art.texture, lampTex: lamps0.texture, envMap: env.texture, lampGain: tier.lampGain, forceFallback, giColors: Object.fromEntries((layout.gi || []).map(g => [g.id, g.color])) });
    if (forceFallback) pf.fallback.enable();
    const plane = new THREE.Mesh(new THREE.PlaneGeometry(FIELD_W, FIELD_H), pf.material);
    plane.rotation.x = -Math.PI / 2; plane.position.set(0, 0, -FIELD_H / 2); plane.receiveShadow = true; plane.name = 'playfield';
    tableRoot.add(plane);
    const wallGeo = buildWallGeometry(layout), wallMeshes = [];
    for (const [geo, mat, name] of [[wallGeo.wood, mats.wood, 'woodRails'], [wallGeo.metal, mats.rail, 'metalGuides'], [wallGeo.plastic, mats.plasticWall, 'plasticWalls']]) {
        if (!geo) continue;
        const m = new THREE.Mesh(geo, mat); m.name = name; m.castShadow = name !== 'plasticWalls'; m.receiveShadow = true; if (name === 'plasticWalls') m.renderOrder = 2; tableRoot.add(m); wallMeshes.push(m);
    }
    const cabinet = buildCabinet({ tableRoot, layout, mats, tr, tableId, backglass: backglassCanvas(glassImg, palette), aniso: aniso() });
    const room = buildRoom({ scene, tableRoot, mats, accent: palette.accent, secondary: palette.secondary });
    const lights = createLights({ scene, tableRoot, layout, tier, palette, shadows: true });
    cabinet.meshes.forEach(m => { if (m.name === 'body' || m.name === 'headBody') m.castShadow = true; });

    // Placeholder balls (T13 replaces them with real ball meshes through addMechanics + useSimpleBalls(false)).
    const ballGeo = new THREE.SphereGeometry(BALL_RADIUS, 24, 16);
    const ballMat = new THREE.MeshStandardMaterial({ color: '#dfe7f1', metalness: 1, roughness: 0.07, envMap: env.texture, envMapIntensity: 3 });
    const ballMesh = new THREE.InstancedMesh(ballGeo, ballMat, 16); ballMesh.count = 0; ballMesh.frustumCulled = false; ballMesh.castShadow = true; ballMesh.name = 'simpleBalls';
    tableRoot.add(ballMesh);
    const tmpM = new THREE.Matrix4();

    const visual = VISUALS[tableId] || {};
    let toys = null, toysLow = false;
    const buildToysNow = () => {
        if (!visual.buildToys) return;
        if (toys) { if (toys.dispose) toys.dispose(); toys = null; }
        toysLow = quality === 'low';
        toys = visual.buildToys(THREE, art.kit, layout, { tier: quality, low: toysLow });
        if (toys && toys.group) tableRoot.add(toys.group);
    };
    buildToysNow();

    // ---- camera + composer ----
    const rig = createCameraRig({ camera, domElement: canvas, tableRoot, reduced: settings.reduced === true });
    const rt = new THREE.WebGLRenderTarget(4, 4, { type: THREE.HalfFloatType, samples: tier.samples });
    const composer = new EffectComposer(renderer, rt);
    const renderPass = new RenderPass(scene, camera);
    const bloom = new UnrealBloomPass(new THREE.Vector2(4, 4), 0.6, 0.4, 0.85);
    const outPass = new OutputPass();
    composer.addPass(renderPass); composer.addPass(bloom); composer.addPass(outPass);
    let sceneCalls = 0, sceneTris = 0;
    const origRender = renderPass.render.bind(renderPass);
    renderPass.render = (...a) => { origRender(...a); sceneCalls = renderer.info.render.calls; sceneTris = renderer.info.render.triangles; };
    renderer.debug.onShaderError = () => { if (!pf.fallback.active) pf.fallback.enable(); };

    let width = 0, height = 0, frames = 0, tAnim = 0, lastDmd = -1, lost = false, reducedNow = settings.reduced === true;
    const cachedRgba = new Uint8Array(1024), mechs = new Set();
    let simpleBalls = true;

    function applyTier() {
        tier = tierOf(quality);
        renderer.setPixelRatio(Math.min((typeof window !== 'undefined' && window.devicePixelRatio) || 1, tier.dpr));
        renderer.shadowMap.enabled = tier.shadow > 0;
        composer.setPixelRatio(renderer.getPixelRatio());
        for (const target of [composer.renderTarget1, composer.renderTarget2]) { target.samples = tier.samples; target.dispose(); }
        bloom.enabled = !!tier.bloom;
        if (tier.bloom) { bloom.strength = tier.bloom.strength * (reducedNow ? 0.6 : 1); bloom.radius = tier.bloom.radius; bloom.threshold = tier.bloom.threshold; }
        pf.uniforms.lampGain.value = tier.lampGain;
        lights.setTier(tier, true);
        for (const m of mechs) if (m.setQuality) m.setQuality(tier);
        scene.traverse(o => { if (o.material) (Array.isArray(o.material) ? o.material : [o.material]).forEach(m => { m.needsUpdate = true; }); });
        mats.woodTex.anisotropy = art.texture.anisotropy = aniso();
        width = 0; resize();
    }
    function rebuildArt() {
        if (art.canvas.width === tier.art[0]) return;
        const old = art; art = paintPlayfieldArt({ layout, tr, tableId, size: tier.art, underlay, aniso: aniso() });
        pf.material.map = art.texture; pf.material.needsUpdate = true; old.texture.dispose(); old.canvas.width = old.canvas.height = 1;
    }
    function resize() {
        if (disposed) return;
        const w = Math.max(1, host.clientWidth), h = Math.max(1, host.clientHeight);
        if (w === width && h === height) return;
        width = w; height = h;
        renderer.setSize(w, h, false); composer.setSize(w, h);
        if (tier.bloom && tier.bloom.half) bloom.setSize(Math.max(1, w >> 1), Math.max(1, h >> 1));
        rig.resize(w, h);
    }
    const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(resize) : null;
    if (observer) observer.observe(host);
    const onLost = e => { e.preventDefault(); lost = true; };
    const onRestored = () => {
        lost = false;
        const old = env, fresh = createEnvironment(renderer, palette);
        scene.traverse(o => { if (o.material) (Array.isArray(o.material) ? o.material : [o.material]).forEach(m => { if (m.envMap === old.texture) m.envMap = fresh.texture; }); });
        Object.values(mats).forEach(m => { if (m && m.envMap === old.texture) m.envMap = fresh.texture; });
        env = fresh; old.dispose();
    };
    canvas.addEventListener('webglcontextlost', onLost); canvas.addEventListener('webglcontextrestored', onRestored);
    const setCam = mode => { rig.setMode(mode); cabinet.head.visible = rig.mode !== 'play' && rig.mode !== 'top'; };
    applyTier(); rebuildArt(); setCam(['top', 'showcase', 'attract'].includes(settings.camera) ? settings.camera : 'play'); resize();

    function update(o = {}) {
        if (disposed || lost) return;
        const { snap, lamps, dmd, toyState, events = [], dt = 0, paused = false, reduced = false, interactive = true } = o;
        const step = paused ? 0 : Math.min(0.05, Math.max(0, dt));
        tAnim += step;
        if (reduced !== reducedNow) { reducedNow = reduced; if (tier.bloom) bloom.strength = tier.bloom.strength * (reduced ? 0.6 : 1); }
        if (lamps && lamps.rgba) {
            let changed = false;
            for (let i = 0; i < 1024; i++) if (cachedRgba[i] !== lamps.rgba[i]) { changed = true; break; }
            if (changed) { cachedRgba.set(lamps.rgba); lamps0.texture.stateData.set(cachedRgba); lamps0.texture.stateTex.needsUpdate = true; }
            if (pf.fallback.active) pf.fallback.update(layout, cachedRgba, tAnim);
        }
        const gl = lamps && lamps.giLevel, gv = pf.uniforms.giLevel.value;
        gv.set(1, 1);
        (layout.gi || []).forEach((g, k) => { const v = gl ? gl[k] : 1; if (g.id === 'upper') gv.y = v; else if (g.id === 'lower') gv.x = v; });
        lights.update(lamps, reduced);
        if (dmd && dmd.frame && dmd.version !== lastDmd) { lastDmd = dmd.version; cabinet.dmd.setFrame(dmd.frame); }
        let n = 0;
        if (snap && snap.balls && simpleBalls) for (const b of snap.balls) {
            if (b.hidden || n >= 16) continue;
            tmpM.makeTranslation(b.x, BALL_RADIUS + (b.h || 0), -b.y); ballMesh.setMatrixAt(n++, tmpM);
        }
        ballMesh.count = n; ballMesh.visible = simpleBalls; if (n) ballMesh.instanceMatrix.needsUpdate = true;
        const ctx = { snap, lamps, toyState, dt: step, time: tAnim, reduced, scene: api };
        for (const m of mechs) if (m.update) m.update(ctx);
        if (toys && toys.update) toys.update(step, tAnim, snap, toyState, lamps, events);
        rig.update(step, tAnim, { reducedNow: reduced, interactive });
        composer.render(step);
        frames++;
    }

    const api = {
        update,
        setQuality(q) { if (!TIERS[q] || q === quality) return; quality = q; applyTier(); rebuildArt(); if ((q === 'low') !== toysLow) buildToysNow(); },
        setCamera(mode) { setCam(mode); },
        setInsets(i) { rig.setInsets(i); },
        shake(dx, dy, strength) { rig.shake(dx, dy, strength); },
        hit() { /* T13/T16 hook: event visuals */ },
        // T13 contract: builder({THREE, root, layout, mats, env, tier, kit, assetURL, renderer, scene}) -> {update?(ctx), setQuality?(tier), dispose?()}.
        // `root` is a Group inside tableRoot (X = x, Y = h, Z = -y). ctx = {snap, lamps, toyState, dt, time, reduced, scene}.
        addMechanics(builder) {
            const handle = builder({ THREE, root: mechanicsGroup, layout, mats, env: env.texture, tier, kit: art.kit, assetURL, renderer, scene, tableRoot, camera });
            if (handle) mechs.add(handle);
            return handle;
        },
        useSimpleBalls(on) { simpleBalls = !!on; },
        stats() {
            const info = renderer.info;
            let gpu = 'webgl2';
            try { const gl = renderer.getContext(), ext = gl.getExtension('WEBGL_debug_renderer_info'); if (ext) gpu = String(gl.getParameter(ext.UNMASKED_RENDERER_WEBGL)); } catch (_) { /* ignore */ }
            return { ready: !disposed && !lost, quality, frames, drawCalls: sceneCalls, triangles: sceneTris, textures: info.memory.textures, geometries: info.memory.geometries, width, height, flipperOrder: true, renderer: gpu, fallback: pf.fallback.active, lights: lights.points.length };
        },
        // Handles for T13/T16 and tests.
        three: { THREE, renderer, scene, camera, tableRoot, mechanics: mechanicsGroup, materials: mats, get env() { return env.texture; }, kit: () => art.kit, rig, lights, composer },
        dispose() {
            if (disposed) return; disposed = true;
            if (observer) observer.disconnect();
            canvas.removeEventListener('webglcontextlost', onLost); canvas.removeEventListener('webglcontextrestored', onRestored);
            for (const m of mechs) if (m.dispose) m.dispose();
            mechs.clear();
            if (toys && toys.dispose) toys.dispose();
            rig.dispose(); lights.dispose(); cabinet.dispose(); room.dispose();
            for (const m of [plane, ...wallMeshes]) m.geometry.dispose();
            ballGeo.dispose(); ballMat.dispose(); ballMesh.dispose();
            art.texture.dispose(); art.canvas.width = art.canvas.height = 1;
            const t = lamps0.texture; t.maskTex.dispose(); t.stateTex.dispose(); t.giTex.dispose();
            pf.fallback.dispose(); pf.material.dispose(); disposeMaterials(mats);
            for (const b of [underlay, glassImg]) if (b && b.close) b.close();
            env.dispose();
            composer.passes.forEach(p => p.dispose && p.dispose()); composer.renderTarget1.dispose(); composer.renderTarget2.dispose(); rt.dispose();
            renderer.dispose();
            try { renderer.forceContextLoss(); } catch (_) { /* ignore */ }
            canvas.remove();
        }
    };
    api.addMechanics(buildMechanics); api.useSimpleBalls(false);
    return api;
}
export default createScene;
