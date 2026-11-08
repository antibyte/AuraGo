import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { RoomEnvironment } from 'three/addons/environments/RoomEnvironment.js';
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/addons/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/addons/postprocessing/UnrealBloomPass.js';
import { OutputPass } from 'three/addons/postprocessing/OutputPass.js';
import { ASSET_ROOT, tableOf, clamp } from './tables.js';

const TIERS = { low: { dpr: 1, shadow: 512, bloom: false }, medium: { dpr: 1.25, shadow: 1024, bloom: true }, high: { dpr: 1.5, shadow: 2048, bloom: true } };

export async function createScene(host, tableId, settings, assetURL, signal) {
    const table = tableOf(tableId), scene = new THREE.Scene(), camera = new THREE.PerspectiveCamera(38, 1, .01, 12);
    // The player's view looks along +Z; mirror display X so authored right stays screen-right.
    const tableRoot = new THREE.Group(); tableRoot.scale.x = -1; scene.add(tableRoot);
    let renderer;
    try { renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false, powerPreference: 'high-performance' }); }
    catch (_) { throw Error('webgl_error'); }
    renderer.setClearColor('#060910'); renderer.toneMapping = THREE.ACESFilmicToneMapping; renderer.toneMappingExposure = .78;
    renderer.shadowMap.enabled = true; renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    renderer.info.autoReset = false;
    renderer.domElement.className = 'pb-canvas'; renderer.domElement.setAttribute('aria-hidden', 'true');
    host.append(renderer.domElement);
    const controls = new OrbitControls(camera, renderer.domElement);
    controls.target.set(0, .07, .64); controls.minDistance = .6; controls.maxDistance = 2.8; controls.maxPolarAngle = Math.PI * .49; controls.enableDamping = true; controls.enabled = false;
    const pmrem = new THREE.PMREMGenerator(renderer), room = new RoomEnvironment();
    const environment = pmrem.fromScene(room, .04); room.dispose(); scene.environment = environment.texture; scene.environmentIntensity = .3;
    const key = new THREE.DirectionalLight('#e8f3ff', 1.3); key.position.set(-.6, 1.8, -.4); key.castShadow = true;
    key.shadow.camera.left = -.5; key.shadow.camera.right = .5; key.shadow.camera.top = .9; key.shadow.camera.bottom = -.9; key.shadow.camera.near = .1; key.shadow.camera.far = 4; key.shadow.bias = -.00012; key.shadow.normalBias = .0006;
    key.target.position.set(0, 0, .65); scene.add(key, key.target);
    const fill = new THREE.DirectionalLight(table.secondary, .7); fill.position.set(.6, .8, 1.4); scene.add(fill);
    scene.add(new THREE.HemisphereLight('#a5d7ff', '#182022', .3));
    for (const [x,z] of [[-.22,.38],[.22,.65],[0,1.05]]) { const light = new THREE.PointLight(table.accent, .035, .6, 2); light.position.set(x,.15,z); scene.add(light); }
    const ground = new THREE.Mesh(new THREE.PlaneGeometry(6, 6), new THREE.MeshStandardMaterial({ color: '#02040a', roughness: 1, metalness: 0, envMapIntensity: 0 }));
    ground.rotation.x = -Math.PI/2; ground.position.y = -.215; ground.position.z = .6; ground.receiveShadow = true; scene.add(ground);
    const composer = new EffectComposer(renderer); composer.addPass(new RenderPass(scene, camera));
    const bloom = new UnrealBloomPass(new THREE.Vector2(800, 600), .25, .4, 1.1); composer.addPass(bloom); composer.addPass(new OutputPass());
    const ballGeometry = new THREE.SphereGeometry(.0135, 32, 20), ballMaterial = new THREE.MeshPhysicalMaterial({ color: '#eff5ff', metalness: 1, roughness: .075, clearcoat: 1, clearcoatRoughness: .04, envMapIntensity: 1.5 });
    const balls = new Map(), textures = [], pulses = new Map();
    let model = null, disposed = false, quality = settings.quality, mode = 'play', width = 0, height = 0, frameCount = 0, dirty = true, lastReduced = null, lastTick = -1;
    controls.addEventListener('change', () => { dirty = true; });
    function releaseGraph(root) { const geometries = new Set(), materials = new Set(); root?.traverse(o => { if (o.geometry) geometries.add(o.geometry); for (const m of Array.isArray(o.material) ? o.material : o.material ? [o.material] : []) materials.add(m); }); geometries.forEach(g => g.dispose()); materials.forEach(m => { for (const v of Object.values(m)) if (v?.isTexture && !textures.includes(v)) v.dispose(); m.dispose(); }); }
    function dispose() {
        if (disposed) return; disposed = true; observer.disconnect(); controls.dispose();
        releaseGraph(model); ground.geometry.dispose(); ground.material.dispose(); ballGeometry.dispose(); ballMaterial.dispose();
        for (const t of textures) { t.dispose(); t.image?.close?.(); } environment.dispose(); pmrem.dispose();
        key.shadow.map?.dispose(); composer.passes.forEach(p => p.dispose?.()); composer.dispose(); renderer.dispose(); renderer.forceContextLoss(); renderer.domElement.remove();
        balls.clear(); pulses.clear();
    }
    function resize() {
        if (disposed) return;
        const w = Math.max(1, host.clientWidth), h = Math.max(1, host.clientHeight);
        if (w === width && h === height) return; width = w; height = h;
        camera.aspect = w/h; camera.updateProjectionMatrix(); renderer.setSize(w,h,false); composer.setSize(w,h); dirty = true;
        frameCamera();
    }
    const observer = new ResizeObserver(resize); observer.observe(host);
    function frameCamera() {
        if (mode === 'showcase') return;
        const distance = camera.aspect < .85 ? 1.18 / Math.max(.55,camera.aspect) : 1;
        if (mode === 'top') { camera.position.set(0,1.58*distance,.59); camera.lookAt(0,0,.64); }
        else { camera.position.set(.015,1.13*distance,-.76*distance); camera.lookAt(0,.08,.65); }
        controls.target.set(0,.055,.65);
    }
    function setQuality(next) {
        quality = TIERS[next] ? next : 'high'; const tier = TIERS[quality];
        renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, tier.dpr)); composer.setPixelRatio(renderer.getPixelRatio());
        key.shadow.mapSize.set(tier.shadow,tier.shadow); key.shadow.map?.dispose(); key.shadow.map = null;
        bloom.enabled = tier.bloom; bloom.strength = settings.reduced ? .14 : .25;
        model?.traverse(obj => { if (!obj.isMesh) return; for (const mat of Array.isArray(obj.material) ? obj.material : [obj.material]) if (mat.name === 'Glass' || mat.name === 'TubeGlass') { const transmission=mat.name==='TubeGlass'&&quality==='high';mat.transmission=transmission ? .95 : 0;mat.transparent=!transmission;mat.opacity=transmission ? 1 : .18;mat.depthWrite=transmission;mat.needsUpdate=true; } });
        width = 0; resize();
    }
    async function fetchFile(path, type) { const response = await fetch(assetURL(ASSET_ROOT + path), { signal }); if (!response.ok) throw Error('load_error'); return response[type](); }
    async function texture(path, color) {
        const blob = await fetchFile(path, 'blob'), bitmap = await createImageBitmap(blob);
        const map = new THREE.Texture(bitmap); map.flipY = false; map.colorSpace = color ? THREE.SRGBColorSpace : THREE.NoColorSpace; map.anisotropy = Math.min(8,renderer.capabilities.getMaxAnisotropy()); map.needsUpdate = true; textures.push(map);
        if (disposed || signal.aborted) { map.dispose(); bitmap.close(); throw Error('cancelled'); }
        return map;
    }
    try {
        const manifest = await fetchFile('manifest.json', 'json');
        const entry = manifest?.version === 1 && manifest.tables?.find(t => t.id === table.id);
        if (!entry || ![entry.model,entry.physics,entry.playfield,entry.backglass,entry.normal,entry.ao].every(p => typeof p === 'string' && /^[a-z0-9-]+\.(glb|json|webp|png)$/.test(p))) throw Error('load_error');
        const loaded = await Promise.all([fetchFile(entry.model,'arrayBuffer'),fetchFile(entry.physics,'json'),texture(entry.playfield,true),texture(entry.backglass,true),texture(entry.normal,false),texture(entry.ao,false)]);
        const gltf = await new GLTFLoader().parseAsync(loaded[0], ''); model = gltf.scene;
        if (signal.aborted) throw Error('cancelled');
        const [,physics,paint,back,normal,ao] = loaded;
        if (physics.id !== table.id) throw Error('load_error');
        model.traverse(obj => {
            if (!obj.isMesh) return; obj.castShadow = true; obj.receiveShadow = true;
            for (const mat of Array.isArray(obj.material) ? obj.material : [obj.material]) {
                mat.envMapIntensity = .75;
                if (['Chrome','Gold','Ramp'].includes(mat.name)) { mat.normalMap=normal; mat.normalScale.set(.08,.08); }
                if (mat.name === 'Playfield') { mat.map=paint; mat.normalMap=normal; mat.normalScale.set(.25,.25); mat.aoMap=ao; mat.aoMapIntensity=.5; mat.roughness=.28; }
                if (mat.name === 'Backglass') { mat.map=back; mat.emissive.set('#ffffff'); mat.emissiveMap=back; mat.emissiveIntensity=.65; mat.toneMapped=false; }
                if (mat.name === 'Glass' || mat.name === 'TubeGlass') { mat.color.set('#c0e3e8');mat.metalness=0;mat.thickness=.0015;mat.roughness=.06;mat.ior=1.35;mat.envMapIntensity=.35; }
                mat.needsUpdate=true;
            }
        });
        const lamps=[];
        model.traverse(obj=>{if(obj.isMesh&&obj.name.startsWith('shot-lamp-')){obj.material=obj.material.clone();lamps.push(obj);}});
        tableRoot.add(model); setQuality(quality); resize();
        function update(snapshot, delta, paused, reduced, interactive = true, playerState = null) {
            if (disposed) return;
            if (paused && (!interactive || mode !== 'showcase') && !dirty && !pulses.size && lastReduced === reduced && lastTick === snapshot?.tick) return;
            dirty = false; lastTick = snapshot?.tick; lastReduced = reduced; bloom.strength = reduced ? .14 : .25; frameCount++;
            for (const [name,p] of Object.entries(snapshot?.actors || {})) { const node=model.getObjectByName(name); if (node) { node.position.fromArray(p.position); node.quaternion.fromArray(p.rotation); } }
            const present=new Set();
            for (const p of snapshot?.balls || []) {
                present.add(p.id); let mesh=balls.get(p.id);
                if (!mesh) { mesh=new THREE.Mesh(ballGeometry,ballMaterial); mesh.castShadow=true; tableRoot.add(mesh); balls.set(p.id,mesh); }
                mesh.position.fromArray(p.position); mesh.quaternion.fromArray(p.rotation);
            }
            for (const [id,mesh] of balls) if (!present.has(id)) { tableRoot.remove(mesh); balls.delete(id); }
            const jaw=model.getObjectByName('hero-jaw'); if (jaw) jaw.rotation.x=(snapshot?.gate || 0)*.42;
            for(const [side,name] of [[-1,'left'],[1,'right']]){const door=model.getObjectByName('temple-door-'+name);if(door)door.position.x=side*(.016+.036*Math.min(1,(playerState?.bank.length||0)/3));}
            for(const lamp of lamps)lamp.material.emissiveIntensity=playerState?.finale?(reduced?2.2:2.2+Math.sin((snapshot?.time||0)*5)*.8):.7;
            for (const [name,pulse] of pulses) { pulse.life=Math.max(0,pulse.life-delta); const node=model.getObjectByName(name); if (node?.material?.emissive) { node.material.emissive.set(table.accent); node.material.emissiveIntensity=pulse.life*1.7; } if (!pulse.life) pulses.delete(name); }
            controls.enabled=mode==='showcase' && paused && interactive; if (controls.enabled) controls.update();
            renderer.info.reset(); composer.render();
        }
        function stats() {
            const gl=renderer.getContext(),debug=gl.getExtension('WEBGL_debug_renderer_info');
            camera.updateMatrixWorld();
            const left=model.getObjectByName('flipper-left').getWorldPosition(new THREE.Vector3()).project(camera);
            const right=model.getObjectByName('flipper-right').getWorldPosition(new THREE.Vector3()).project(camera);
            return {ready:true,quality,frames:frameCount,drawCalls:renderer.info.render.calls,triangles:renderer.info.render.triangles,textures:renderer.info.memory.textures,geometries:renderer.info.memory.geometries,width,height,actorOrder:left.x<right.x,renderer:gl.getParameter(debug?debug.UNMASKED_RENDERER_WEBGL:gl.RENDERER)};
        }
        return { physics, update, setQuality, setCamera(next) { mode=next; dirty=true; frameCamera(); }, hit(event) { if (event.type==='bumper') { const node=model.getObjectByName(event.id); if (node?.material && !node.userData.pulseMaterial) { node.material=node.material.clone(); node.userData.pulseMaterial=true; } pulses.set(event.id,{life:.3}); } }, stats, dispose };
    } catch (error) { dispose(); throw error; }
}
