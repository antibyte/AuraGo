import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/addons/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/addons/postprocessing/UnrealBloomPass.js';
import { OutputPass } from 'three/addons/postprocessing/OutputPass.js';

const COLORS = [0xfff6df, 0xf6a378, 0x89c9ef, 0xc8a6ed];
const LOOKS = {
    garden: { sky: 0xc9e2d6, fog: 0xc9e2d6, sun: 0xffe4b6, ground: 0x6e9968 },
    coast: { sky: 0xbcdfe4, fog: 0xbcdfe4, sun: 0xffeed1, ground: 0x6baeb4 },
    night: { sky: 0x182e43, fog: 0x283c50, sun: 0xabbfee, ground: 0x283e49 }
};

function disposeTree(tree) {
    const geometries = new Set(), materials = new Set(), textures = new Set();
    tree.traverse(node => {
        if (node.geometry) geometries.add(node.geometry);
        for (const material of (Array.isArray(node.material) ? node.material : [node.material])) {
            if (!material) continue;
            materials.add(material);
            for (const value of Object.values(material)) if (value?.isTexture) textures.add(value);
        }
    });
    for (const texture of textures) { texture.dispose(); texture.source?.data?.close?.(); }
    for (const material of materials) material.dispose();
    for (const geometry of geometries) geometry.dispose();
}

export function createScene(host, options) {
    const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false, powerPreference: 'high-performance' });
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.toneMappingExposure = 1.18;
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    const canvas = renderer.domElement;
    canvas.tabIndex = 0;
    canvas.setAttribute('aria-label', options.label);
    host.append(canvas);
    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(43, 1, .1, 180);
    const controls = new OrbitControls(camera, canvas);
    controls.enableDamping = true;
    controls.dampingFactor = .10;
    controls.minDistance = 3;
    controls.maxDistance = 38;
    controls.maxPolarAngle = Math.PI * .46;
    controls.minPolarAngle = .18;
    controls.mouseButtons.LEFT = null;
    controls.mouseButtons.RIGHT = THREE.MOUSE.ROTATE;
    controls.touches.ONE = null;
    controls.touches.TWO = THREE.TOUCH.DOLLY_PAN;
    const ambient = new THREE.HemisphereLight(0xffffff, 0x60846e, 2.5);
    const sun = new THREE.DirectionalLight(0xffe4b6, 3);
    sun.position.set(-9, 18, 8);
    sun.castShadow = true;
    sun.shadow.bias = -.0003;
    sun.shadow.normalBias = .025;
    Object.assign(sun.shadow.camera, { left: -18, right: 18, top: 18, bottom: -18, near: .1, far: 65 });
    scene.add(ambient, sun, sun.target);
    const composer = new EffectComposer(renderer);
    const renderPass = new RenderPass(scene, camera);
    const bloom = new UnrealBloomPass(new THREE.Vector2(640, 360), .22, .5, 1.1);
    const output = new OutputPass();
    composer.addPass(renderPass); composer.addPass(bloom); composer.addPass(output);
    const aim = new THREE.ArrowHelper(new THREE.Vector3(0, 0, -1), new THREE.Vector3(), 2, 0xffedb3, .3, .17);
    aim.visible = false;
    scene.add(aim);
    const ball = new THREE.Group();
    scene.add(ball);
    const ghosts = new THREE.Group();
    scene.add(ghosts);
    const effectGeometry = new THREE.IcosahedronGeometry(.055, 0);
    const effectMaterial = new THREE.MeshBasicMaterial({ color: 0xffffff, transparent: true, opacity: .8, depthWrite: false });
    const effects = new THREE.InstancedMesh(effectGeometry, effectMaterial, 320);
    effects.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    effects.frustumCulled = false; effects.count = 0; scene.add(effects);
    const particleData = [], dummy = new THREE.Object3D(), effectColor = new THREE.Color();
    const waterTime = { value: 0 };
    const ray = new THREE.Raycaster(), pointer = new THREE.Vector2(), floor = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0);
    const hit = new THREE.Vector3();
    let course = null, courseRoot = null, disposed = false, generation = 0, request = null;
    let quality = options.quality || 'auto', reduced = !!options.reducedMotion, cameraMode = false, canAim = false;
    let aimDirection = [0, -1], aimPower = .35, pointerId = null, dragged = false;
    let obstacleNodes = new Map(), waterNodes = [], flagNodes = [], windmill = null, putter = null, swing = 0, elapsed = 0, trailAt = 0;
    let fpsSum = 0, fpsCount = 0, measuredFPS = 0, lastAdaptive = 0;
    const cap = () => quality === 'low' ? 60 : quality === 'high' ? 320 : 160;

    function resize() {
        if (disposed) return;
        const width = Math.max(1, host.clientWidth), height = Math.max(1, host.clientHeight);
        const dpr = Math.min(devicePixelRatio || 1, quality === 'high' ? 2 : quality === 'low' ? 1 : 1.5);
        renderer.setPixelRatio(dpr); renderer.setSize(width, height, false);
        composer.setPixelRatio(dpr); composer.setSize(width, height);
        bloom.setSize(Math.max(1, width >> 1), Math.max(1, height >> 1));
        camera.aspect = width / height; camera.updateProjectionMatrix();
    }
    function setQuality(value) {
        quality = ['auto', 'low', 'medium', 'high'].includes(value) ? value : 'auto';
        renderer.shadowMap.enabled = quality !== 'low';
        const shadowSize = quality === 'high' ? 2048 : 1024;
        if (sun.shadow.mapSize.x !== shadowSize) {
            sun.shadow.map?.dispose(); sun.shadow.map = null;
            sun.shadow.mapSize.set(shadowSize, shadowSize);
        }
        resize();
    }
    function updateAim() {
        aim.visible = canAim && !cameraMode;
        aim.position.copy(ball.position); aim.position.y += .02;
        aim.setDirection(new THREE.Vector3(aimDirection[0], 0, aimDirection[1]));
        aim.setLength(.5 + 2 * aimPower, .25, .14);
        if (putter) {
            putter.visible = canAim || swing > 0;
            putter.position.copy(ball.position).add(new THREE.Vector3(-aimDirection[0] * (.38 + swing * .25), -.13, -aimDirection[1] * (.38 + swing * .25)));
            putter.rotation.y = Math.atan2(aimDirection[0], aimDirection[1]);
        }
    }
    function setAim(direction, power, enabled = true) {
        aimDirection = direction; aimPower = power; canAim = enabled;
        updateAim();
    }
    function focusBall(overview = false) {
        const target = overview && course ? new THREE.Vector3(...course.cup).add(new THREE.Vector3(...course.tee)).multiplyScalar(.5) : ball.position.clone();
        controls.target.copy(target);
        camera.position.copy(target).add(overview ? new THREE.Vector3(10, 13, 14) : new THREE.Vector3(6, 8, 10));
        controls.update();
    }
    function setBall(position, rotation, color = 0) {
        ball.position.fromArray(position);
        if (rotation) ball.quaternion.fromArray(rotation);
        ball.traverse(node => { if (node.isMesh) node.material.color.setHex(COLORS[color % COLORS.length]); });
        updateAim();
    }
    function setGhosts(players, active) {
        disposeTree(ghosts); ghosts.clear();
        players.forEach((player, index) => {
            if (index === active || player.done || !player.position) return;
            const mark = new THREE.Mesh(new THREE.SphereGeometry(.16, 12, 8), new THREE.MeshBasicMaterial({ color: COLORS[index], transparent: true, opacity: .33, depthWrite: false }));
            mark.position.fromArray(player.position); ghosts.add(mark);
        });
    }
    async function load(next) {
        const version = ++generation;
        request?.abort(); request = new AbortController();
        const response = await fetch(options.assetURL('/3d/minigolf/' + next.model), { signal: request.signal });
        if (!response.ok) throw Error('course_load');
        const bytes = await response.arrayBuffer();
        if (disposed || generation !== version) return false;
        const gltf = await new GLTFLoader().parseAsync(bytes, '');
        if (disposed || generation !== version) { disposeTree(gltf.scene); return false; }
        if (courseRoot) { scene.remove(courseRoot); disposeTree(courseRoot); }
        disposeTree(ball); ball.clear();
        if (putter) { scene.remove(putter); disposeTree(putter); putter = null; }
        course = next; courseRoot = gltf.scene; scene.add(courseRoot);
        obstacleNodes = new Map(); waterNodes = []; flagNodes = []; windmill = null;
        const look = LOOKS[next.world] || LOOKS.garden;
        scene.background = new THREE.Color(look.sky);
        scene.fog = new THREE.FogExp2(look.fog, next.world === 'night' ? .022 : .013);
        sun.color.setHex(look.sun); sun.intensity = next.world === 'night' ? 1.1 : 3;
        ambient.intensity = next.world === 'night' ? 1.5 : 2.5;
        ambient.groundColor.setHex(look.ground);
        bloom.strength = next.world === 'night' ? .32 : .15;
        courseRoot.traverse(node => {
            if (node.isMesh) { node.castShadow = true; node.receiveShadow = true; }
            if (node.name.startsWith('water') && node.isMesh) {
                node.material = node.material.clone();
                node.material.onBeforeCompile = shader => {
                    shader.uniforms.golfWaterTime = waterTime;
                    shader.vertexShader = shader.vertexShader.replace('#include <common>', '#include <common>\nvarying vec2 golfWaterXZ;')
                        .replace('#include <begin_vertex>', '#include <begin_vertex>\ngolfWaterXZ = position.xz;');
                    shader.fragmentShader = shader.fragmentShader.replace('#include <common>', '#include <common>\nuniform float golfWaterTime;\nvarying vec2 golfWaterXZ;')
                        .replace('#include <color_fragment>', '#include <color_fragment>\nfloat wave = sin(golfWaterXZ.x * 3.2 + golfWaterTime * 0.65) * sin(golfWaterXZ.y * 2.7 - golfWaterTime * 0.45);\nfloat glint = pow(max(0.0, wave), 12.0);\ndiffuseColor.rgb *= 0.94 + wave * 0.06;\ndiffuseColor.rgb += vec3(0.12, 0.20, 0.19) * glint;');
                };
                node.material.customProgramCacheKey = () => 'auragolf-water-v1';
                waterNodes.push({ node, y: node.position.y });
            }
            if (node.name === 'flag' || node.name.startsWith('flag-cloth')) flagNodes.push({ node, z: node.rotation.z });
        });
        const sails = [];
        courseRoot.traverse(node => { if (node.name.startsWith('windmill-sail')) sails.push(node); });
        if (sails.length) {
            courseRoot.updateMatrixWorld(true);
            windmill = new THREE.Group();
            for (const sail of sails) windmill.position.add(sail.getWorldPosition(new THREE.Vector3()));
            windmill.position.divideScalar(sails.length); courseRoot.worldToLocal(windmill.position);
            courseRoot.add(windmill); windmill.updateMatrixWorld(true);
            for (const sail of sails) windmill.attach(sail);
        }
        for (const obstacle of next.obstacles || []) {
            const node = courseRoot.getObjectByName(obstacle.node);
            if (node) obstacleNodes.set(obstacle.id, node);
        }
        const template = courseRoot.getObjectByName('ball-template');
        if (template) {
            template.removeFromParent(); template.position.set(0, 0, 0); template.quaternion.identity();
            template.traverse(node => { if (node.isMesh) node.material = node.material.clone(); });
            ball.add(template);
        } else throw Error('ball_asset');
        putter = courseRoot.getObjectByName('putter-template');
        if (putter) { putter.removeFromParent(); scene.add(putter); }
        particleData.length = 0; effects.count = 0;
        setBall(next.tee); focusBall(true); resize();
        return true;
    }
    function burst(type, position, speed = 1) {
        if (type === 'strike') swing = .65;
        if (reduced || !position) return;
        const count = type === 'cup' || type === 'win' ? 64 : type === 'water' ? 32 : 9;
        const color = type === 'water' ? 0x92e2eb : type === 'cup' || type === 'win' ? 0xffd789 : 0xf3edc7;
        for (let i = 0; i < count && particleData.length < cap(); i++) {
            const angle = i * 2.399963, strength = .5 + Math.random() * Math.min(3, speed);
            particleData.push({ p: new THREE.Vector3(...position), v: new THREE.Vector3(Math.cos(angle) * strength, 1 + Math.random() * 2, Math.sin(angle) * strength), life: 1, color, size: .5 + Math.random(), float: false });
        }
    }
    function pointAt(event) {
        const rect = canvas.getBoundingClientRect();
        pointer.set((event.clientX - rect.left) / rect.width * 2 - 1, 1 - (event.clientY - rect.top) / rect.height * 2);
        ray.setFromCamera(pointer, camera); floor.constant = -ball.position.y;
        return ray.ray.intersectPlane(floor, hit);
    }
    function down(event) {
        if (event.button !== 0 || !canAim || cameraMode || !pointAt(event)) return;
        const screen = ball.position.clone().project(camera), rect = canvas.getBoundingClientRect();
        const distance = Math.hypot(event.clientX - rect.left - (screen.x + 1) * rect.width / 2, event.clientY - rect.top - (1 - screen.y) * rect.height / 2);
        if (distance > 46) return;
        pointerId = event.pointerId; dragged = false; canvas.setPointerCapture(pointerId); event.preventDefault(); canvas.focus();
    }
    function move(event) {
        if (pointerId !== event.pointerId || !pointAt(event)) return;
        const x = ball.position.x - hit.x, z = ball.position.z - hit.z, length = Math.hypot(x, z);
        dragged = length > .06;
        if (length > .03) { setAim([x / length, z / length], Math.min(1, length / 3), true); options.onAim?.(aimDirection, aimPower); }
    }
    function up(event) {
        if (pointerId !== event.pointerId) return;
        pointerId = null;
        if (canvas.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId);
        if (event.type === 'pointerup' && dragged && aimPower > .02 && canAim) options.onShot?.(aimDirection, aimPower);
        dragged = false;
    }
    const lost = event => { event.preventDefault(); options.onError?.('webgl_error'); };
    canvas.addEventListener('pointerdown', down); canvas.addEventListener('pointermove', move);
    canvas.addEventListener('pointerup', up); canvas.addEventListener('pointercancel', up);
    canvas.addEventListener('webglcontextlost', lost);
    const observer = new ResizeObserver(resize); observer.observe(host);
    setQuality(quality);

    function update(dt, snapshot) {
        if (disposed) return;
        elapsed += dt;
        if (snapshot?.position) setBall(snapshot.position, snapshot.rotation, snapshot.color || 0);
        for (const pose of snapshot?.obstacles || []) {
            const node = obstacleNodes.get(pose.id);
            if (node) { node.position.fromArray(pose.position); node.quaternion.fromArray(pose.rotation); }
        }
        swing = Math.max(0, swing - dt * 3);
        if (!reduced) {
            waterTime.value += dt;
            if (windmill) windmill.rotation.z += dt * .35;
            for (const water of waterNodes) water.node.position.y = water.y + Math.sin(elapsed * 1.3) * .018;
            for (const flag of flagNodes) flag.node.rotation.z = flag.z + Math.sin(elapsed * 2.2) * .045;
            if (snapshot?.moving && elapsed - trailAt > .05 && particleData.length < cap()) {
                trailAt = elapsed; particleData.push({ p: ball.position.clone(), v: new THREE.Vector3(), life: .45, color: COLORS[snapshot.color || 0], size: .7, float: true });
            }
            if (course && elapsed - (update.lastAmbient || 0) > .18 && particleData.length < cap() - 10) {
                update.lastAmbient = elapsed;
                const p = new THREE.Vector3((Math.random() - .5) * 13, .5 + Math.random() * 3, (Math.random() - .5) * 15);
                particleData.push({ p, v: new THREE.Vector3(.12, .04, .05), life: 3, color: course.world === 'night' ? 0xffed9b : 0xffd7c5, size: .5, float: true });
            }
        }
        let count = 0;
        for (let i = particleData.length - 1; i >= 0; i--) {
            const p = particleData[i]; p.life -= dt;
            if (p.life <= 0 || reduced) { particleData.splice(i, 1); continue; }
            if (!p.float) p.v.y -= dt * 4;
            p.p.addScaledVector(p.v, dt); dummy.position.copy(p.p); dummy.scale.setScalar(p.size * Math.min(1, p.life * 2)); dummy.updateMatrix();
            effects.setMatrixAt(count, dummy.matrix); effects.setColorAt(count++, effectColor.setHex(p.color));
        }
        effects.count = count; effects.instanceMatrix.needsUpdate = true;
        if (effects.instanceColor) effects.instanceColor.needsUpdate = true;
        controls.enableDamping = !reduced; controls.update(); updateAim();
        if (quality === 'low') renderer.render(scene, camera); else composer.render();
        if (dt > 0 && dt < .5) { fpsSum += dt; fpsCount++; }
        if (fpsSum > 2) { measuredFPS = fpsCount / fpsSum; fpsSum = 0; fpsCount = 0; }
        if (quality === 'auto' && elapsed - lastAdaptive > 6 && measuredFPS > 0 && measuredFPS < 38) {
            lastAdaptive = elapsed; setQuality('low'); options.onQuality?.('low');
        }
    }
    return {
        canvas, load, update, setBall, setGhosts, setAim, focusBall, burst, setQuality,
        setReducedMotion(value) { reduced = !!value; if (reduced) particleData.length = 0; },
        setCameraMode(value) { cameraMode = !!value; controls.touches.ONE = value ? THREE.TOUCH.ROTATE : null; controls.mouseButtons.LEFT = value ? THREE.MOUSE.ROTATE : null; updateAim(); },
        stats() { return { fps: measuredFPS, triangles: renderer.info.render.triangles, calls: renderer.info.render.calls, particles: particleData.length, quality, geometries: renderer.info.memory.geometries }; },
        dispose() {
            if (disposed) return; disposed = true; generation++; request?.abort(); observer.disconnect(); controls.dispose();
            canvas.removeEventListener('pointerdown', down); canvas.removeEventListener('pointermove', move); canvas.removeEventListener('pointerup', up); canvas.removeEventListener('pointercancel', up); canvas.removeEventListener('webglcontextlost', lost);
            disposeTree(scene); sun.shadow.dispose(); bloom.dispose(); renderPass.dispose(); output.dispose(); composer.dispose(); renderer.dispose(); renderer.forceContextLoss(); canvas.remove();
        }
    };
}
