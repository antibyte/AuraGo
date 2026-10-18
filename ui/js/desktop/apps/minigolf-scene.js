import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/addons/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/addons/postprocessing/UnrealBloomPass.js';
import { OutputPass } from 'three/addons/postprocessing/OutputPass.js';

const COLORS = [0xfff6df, 0xf6a378, 0x89c9ef, 0xc8a6ed];
const LOOKS = {
    garden: { sky: 0x88aa9e, fog: 0x88aa9e, sun: 0xffe4b6, ground: 0x6e9968 },
    coast: { sky: 0x86acb8, fog: 0x86acb8, sun: 0xffeed1, ground: 0x6baeb4 },
    night: { sky: 0x182e43, fog: 0x283c50, sun: 0xabbfee, ground: 0x283e49 }
};
const POWER_A = new THREE.Color(0x7fe0a6), POWER_B = new THREE.Color(0xffd789), POWER_C = new THREE.Color(0xff8f77);
const powerTint = new THREE.Color();

function powerColor(value) {
    if (value <= .5) powerTint.copy(POWER_A).lerp(POWER_B, value * 2);
    else powerTint.copy(POWER_B).lerp(POWER_C, (value - .5) * 2);
    return powerTint;
}

function nameTag(text) {
    const label = String(text || '').trim().slice(0, 34) || 'Player';
    const surface = document.createElement('canvas');
    let context = surface.getContext('2d');
    let size = 30;
    context.font = `700 ${size}px system-ui, sans-serif`;
    let width = Math.ceil(context.measureText(label).width) + 26;
    if (width > 460) {
        size = Math.max(14, Math.floor(size * 460 / width));
        context.font = `700 ${size}px system-ui, sans-serif`;
        width = Math.min(460, Math.ceil(context.measureText(label).width) + 20);
    }
    surface.width = width;
    surface.height = size + 20;
    context = surface.getContext('2d');
    context.font = `700 ${size}px system-ui, sans-serif`;
    context.fillStyle = 'rgba(11, 24, 19, .78)';
    context.beginPath();
    context.roundRect(1, 1, surface.width - 2, surface.height - 2, 9);
    context.fill();
    context.fillStyle = '#fff9ec';
    context.textAlign = 'center';
    context.textBaseline = 'middle';
    context.fillText(label, surface.width / 2, surface.height / 2);
    const texture = new THREE.CanvasTexture(surface);
    texture.colorSpace = THREE.SRGBColorSpace;
    const sprite = new THREE.Sprite(new THREE.SpriteMaterial({ map: texture, transparent: true, depthWrite: false }));
    sprite.scale.set((surface.width / surface.height) * .34, .34, 1);
    return sprite;
}

function buildSky(world) {
    const look = LOOKS[world] || LOOKS.garden;
    const top = world === 'night' ? 0x0c1430 : world === 'coast' ? 0x9cc8de : 0xa6cfe2;
    const group = new THREE.Group();
    const dome = new THREE.Mesh(
        new THREE.SphereGeometry(150, 32, 18),
        new THREE.ShaderMaterial({
            side: THREE.BackSide, depthWrite: false,
            uniforms: { topColor: { value: new THREE.Color(top) }, bottomColor: { value: new THREE.Color(look.fog) } },
            vertexShader: 'varying vec3 vSkyPosition;\nvoid main(){ vSkyPosition = position; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }',
            fragmentShader: 'uniform vec3 topColor;\nuniform vec3 bottomColor;\nvarying vec3 vSkyPosition;\nvoid main(){ float h = clamp(normalize(vSkyPosition).y * 0.5 + 0.5, 0.0, 1.0); gl_FragColor = vec4(mix(bottomColor, topColor, smoothstep(0.44, 0.9, h)), 1.0); }'
        })
    );
    group.add(dome);
    if (world === 'night') {
        const positions = new Float32Array(360 * 3);
        for (let i = 0; i < 360; i++) {
            const theta = Math.random() * Math.PI * 2;
            const y = .06 + Math.random() * .92;
            const radius = Math.sqrt(Math.max(0, 1 - y * y));
            positions[i * 3] = Math.cos(theta) * radius * 140;
            positions[i * 3 + 1] = y * 140;
            positions[i * 3 + 2] = Math.sin(theta) * radius * 140;
        }
        const starGeometry = new THREE.BufferGeometry();
        starGeometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
        group.add(new THREE.Points(starGeometry, new THREE.PointsMaterial({ color: 0xf6f4e8, size: 1.7, sizeAttenuation: false, transparent: true, opacity: .9, fog: false, depthWrite: false })));
        const moon = new THREE.Mesh(new THREE.CircleGeometry(5.5, 28), new THREE.MeshBasicMaterial({ color: 0xf7f1d6, fog: false, depthWrite: false }));
        moon.position.set(70, 78, -64);
        moon.lookAt(0, 0, 0);
        group.add(moon);
    }
    return group;
}

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
    renderer.toneMappingExposure = .9;
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
    controls.addEventListener('start', () => { userOrbiting = true; camTween = null; });
    controls.addEventListener('end', () => { userOrbiting = false; });
    const ambient = new THREE.HemisphereLight(0xffffff, 0x60846e, 1.1);
    const sun = new THREE.DirectionalLight(0xffe4b6, 1.8);
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
    const aimLineGeometry = new THREE.BufferGeometry();
    aimLineGeometry.setAttribute('position', new THREE.BufferAttribute(new Float32Array(6), 3));
    const aimLine = new THREE.Line(aimLineGeometry, new THREE.LineDashedMaterial({ color: 0xdf914f, transparent: true, opacity: .7, dashSize: .3, gapSize: .22 }));
    aimLine.visible = false;
    aimLine.frustumCulled = false;
    const powerRing = new THREE.Mesh(new THREE.RingGeometry(.56, .7, 48), new THREE.MeshBasicMaterial({ color: 0xdf914f, transparent: true, opacity: .85, side: THREE.DoubleSide, depthWrite: false }));
    powerRing.rotation.x = -Math.PI / 2;
    powerRing.visible = false;
    const restMark = new THREE.Mesh(new THREE.RingGeometry(.24, .32, 32), new THREE.MeshBasicMaterial({ color: 0xdf914f, transparent: true, opacity: .8, side: THREE.DoubleSide, depthWrite: false }));
    restMark.rotation.x = -Math.PI / 2;
    restMark.visible = false;
    const blob = new THREE.Mesh(new THREE.CircleGeometry(.24, 24), new THREE.MeshBasicMaterial({ color: 0x07130e, transparent: true, opacity: .26, depthWrite: false }));
    blob.rotation.x = -Math.PI / 2;
    blob.visible = false;
    scene.add(aimLine, powerRing, restMark, blob);
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
    let skyGroup = null, followEnabled = true, aimCamEnabled = false, camTween = null, userOrbiting = false;
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
        const shadows = quality !== 'low';
        if (renderer.shadowMap.enabled !== shadows) {
            renderer.shadowMap.enabled = shadows;
            const seen = new Set();
            scene.traverse(node => {
                for (const material of (Array.isArray(node.material) ? node.material : [node.material])) {
                    if (material && !seen.has(material)) { seen.add(material); material.needsUpdate = true; }
                }
            });
        }
        blob.visible = !shadows;
        const shadowSize = quality === 'high' ? 2048 : 1024;
        if (sun.shadow.mapSize.x !== shadowSize) {
            sun.shadow.map?.dispose(); sun.shadow.map = null;
            sun.shadow.mapSize.set(shadowSize, shadowSize);
        }
        resize();
    }
    function updateAim() {
        const aiming = canAim && !cameraMode;
        aim.visible = aiming;
        aimLine.visible = aiming;
        powerRing.visible = aiming;
        aim.position.copy(ball.position); aim.position.y += .02;
        aim.setDirection(new THREE.Vector3(aimDirection[0], 0, aimDirection[1]));
        aim.setLength(.5 + 2 * aimPower, .25, .14);
        if (aiming) {
            const length = .5 + 2 * aimPower, ground = ball.position.y - .155;
            const positions = aimLine.geometry.attributes.position;
            positions.setXYZ(0, ball.position.x, ground, ball.position.z);
            positions.setXYZ(1, ball.position.x + aimDirection[0] * length, ground, ball.position.z + aimDirection[1] * length);
            positions.needsUpdate = true;
            aimLine.computeLineDistances();
            powerRing.position.set(ball.position.x, ground + .012, ball.position.z);
            powerRing.scale.setScalar(.4 + 1.05 * aimPower);
            powerRing.material.color.copy(powerColor(aimPower));
        }
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
        // Flush pending orbit/pan damping before moving the camera and its target together.
        camTween = null;
        const damping = controls.enableDamping;
        controls.enableDamping = false; controls.update();
        const offset = overview ? new THREE.Vector3(10, 13, 14)
            : camera.position.clone().sub(controls.target).clampLength(controls.minDistance, 8);
        const target = overview && course ? new THREE.Vector3(...course.cup).add(new THREE.Vector3(...course.tee)).multiplyScalar(.5) : ball.position.clone();
        controls.target.copy(target);
        camera.position.copy(target).add(offset);
        controls.update();
        controls.enableDamping = damping;
    }
    function frameTurn(angle, position) {
        if (!aimCamEnabled || !position || position.length < 3) return;
        const target = new THREE.Vector3(position[0], position[1], position[2]);
        const dirX = Math.sin(angle), dirZ = -Math.cos(angle);
        const destination = target.clone().add(new THREE.Vector3(-dirX * 6.4, 4.1, -dirZ * 6.4));
        if (reduced) {
            camera.position.copy(destination); controls.target.copy(target); controls.update();
            return;
        }
        camTween = { fromPosition: camera.position.clone(), fromTarget: controls.target.clone(), toPosition: destination, toTarget: target, time: 0, duration: .45 };
    }
    function setRestMark(position) {
        if (!position || position.length < 3) { restMark.visible = false; return; }
        restMark.position.set(position[0], position[1] - .143, position[2]);
        restMark.visible = true;
    }
    function panAt(position) {
        if (!position || position.length < 3) return 0;
        const projected = new THREE.Vector3(position[0], position[1], position[2]).project(camera);
        if (!Number.isFinite(projected.x)) return 0;
        return Math.max(-1, Math.min(1, projected.x)) * .8;
    }
    function setBall(position, rotation, color = 0) {
        ball.position.fromArray(position);
        if (rotation) ball.quaternion.fromArray(rotation);
        ball.traverse(node => { if (node.isMesh) node.material.color.setHex(COLORS[color % COLORS.length]); });
        blob.position.set(ball.position.x, ball.position.y - .152, ball.position.z);
        updateAim();
    }
    function setGhosts(players, active) {
        disposeTree(ghosts); ghosts.clear();
        players.forEach((player, index) => {
            if (index === active || player.done || !player.position) return;
            const mark = new THREE.Mesh(new THREE.SphereGeometry(.16, 12, 8), new THREE.MeshBasicMaterial({ color: COLORS[index], transparent: true, opacity: .33, depthWrite: false }));
            mark.position.fromArray(player.position); ghosts.add(mark);
            const tag = nameTag(player.name);
            tag.position.set(player.position[0], player.position[1] + .46, player.position[2]);
            ghosts.add(tag);
        });
    }
    function addWaterFoam(node) {
        node.updateWorldMatrix(true, false);
        const bounds = new THREE.Box3().setFromObject(node);
        if (!Number.isFinite(bounds.min.x)) return;
        const width = bounds.max.x - bounds.min.x, depth = bounds.max.z - bounds.min.z;
        if (width < .6 || depth < .6) return;
        const y = bounds.max.y + .02, edge = Math.min(.18, Math.min(width, depth) * .12);
        const material = new THREE.MeshBasicMaterial({ color: 0xf4fbfa, transparent: true, opacity: .4, depthWrite: false });
        const group = new THREE.Group();
        const strip = (sw, sd, cx, cz) => {
            const mesh = new THREE.Mesh(new THREE.BoxGeometry(sw, .03, sd), material);
            mesh.position.set(cx, y, cz);
            group.add(mesh);
        };
        const cx = (bounds.min.x + bounds.max.x) / 2, cz = (bounds.min.z + bounds.max.z) / 2;
        strip(width, edge, cx, bounds.min.z + edge / 2);
        strip(width, edge, cx, bounds.max.z - edge / 2);
        strip(edge, Math.max(.01, depth - edge * 2), bounds.min.x + edge / 2, cz);
        strip(edge, Math.max(.01, depth - edge * 2), bounds.max.x - edge / 2, cz);
        scene.add(group);
        group.updateMatrixWorld(true);
        node.attach(group);
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
        if (skyGroup) { scene.remove(skyGroup); disposeTree(skyGroup); skyGroup = null; }
        course = next; courseRoot = gltf.scene; scene.add(courseRoot);
        skyGroup = buildSky(next.world); scene.add(skyGroup);
        restMark.visible = false;
        obstacleNodes = new Map(); waterNodes = []; flagNodes = []; windmill = null;
        const look = LOOKS[next.world] || LOOKS.garden;
        scene.background = new THREE.Color(look.sky);
        scene.fog = new THREE.FogExp2(look.fog, next.world === 'night' ? .022 : .013);
        sun.color.setHex(look.sun); sun.intensity = next.world === 'night' ? .8 : 1.8;
        ambient.intensity = next.world === 'night' ? .9 : 1.1;
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
                addWaterFoam(node);
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
        if (type === 'strike') swing = .25 + Math.min(1, speed) * .6;
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
        if (event.pointerType !== 'touch') {
            const screen = ball.position.clone().project(camera), rect = canvas.getBoundingClientRect();
            const distance = Math.hypot(event.clientX - rect.left - (screen.x + 1) * rect.width / 2, event.clientY - rect.top - (1 - screen.y) * rect.height / 2);
            if (distance > 46) return;
        }
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
        if (camTween && !userOrbiting) {
            camTween.time = Math.min(camTween.duration, camTween.time + dt);
            const progress = camTween.time / camTween.duration;
            const eased = progress >= 1 ? 1 : progress * progress * (3 - 2 * progress);
            camera.position.lerpVectors(camTween.fromPosition, camTween.toPosition, eased);
            controls.target.lerpVectors(camTween.fromTarget, camTween.toTarget, eased);
            if (progress >= 1) camTween = null;
        }
        if (followEnabled && snapshot?.moving && !userOrbiting && !camTween) {
            controls.target.lerp(ball.position, reduced ? 1 : Math.min(1, dt * 2.5));
        }
        controls.enableDamping = !reduced; controls.update(); updateAim();
        if (quality === 'low') renderer.render(scene, camera); else composer.render();
        if (dt > 0 && dt < .5) { fpsSum += dt; fpsCount++; }
        if (fpsSum > 2) { measuredFPS = fpsCount / fpsSum; fpsSum = 0; fpsCount = 0; }
        if (quality === 'auto' && elapsed - lastAdaptive > 6 && measuredFPS > 0 && measuredFPS < 38) {
            lastAdaptive = elapsed; setQuality('low'); options.onQuality?.('low');
        }
    }
    return {
        canvas, load, update, setBall, setGhosts, setAim, focusBall, burst, setQuality, frameTurn, setRestMark, panAt,
        setFollow(value) { followEnabled = !!value; },
        setAimCamera(value) { aimCamEnabled = !!value; if (!value) camTween = null; },
        setReducedMotion(value) { reduced = !!value; if (reduced) particleData.length = 0; },
        setCameraMode(value) { cameraMode = !!value; controls.touches.ONE = value ? THREE.TOUCH.ROTATE : null; controls.mouseButtons.LEFT = value ? THREE.MOUSE.ROTATE : null; updateAim(); },
        stats() { return { fps: measuredFPS, triangles: renderer.info.render.triangles, calls: renderer.info.render.calls, particles: particleData.length, quality, geometries: renderer.info.memory.geometries,
            camera: { position: camera.position.toArray(), target: controls.target.toArray(), ball: ball.position.toArray(), projectedBall: ball.position.clone().project(camera).toArray(), distance: camera.position.distanceTo(controls.target) } }; },
        dispose() {
            if (disposed) return; disposed = true; generation++; request?.abort(); observer.disconnect(); controls.dispose();
            canvas.removeEventListener('pointerdown', down); canvas.removeEventListener('pointermove', move); canvas.removeEventListener('pointerup', up); canvas.removeEventListener('pointercancel', up); canvas.removeEventListener('webglcontextlost', lost);
            disposeTree(scene); sun.shadow.dispose(); bloom.dispose(); renderPass.dispose(); output.dispose(); composer.dispose(); renderer.dispose(); renderer.forceContextLoss(); canvas.remove();
        }
    };
}
