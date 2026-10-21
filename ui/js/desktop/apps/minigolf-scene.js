import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/addons/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/addons/postprocessing/UnrealBloomPass.js';
import { OutputPass } from 'three/addons/postprocessing/OutputPass.js';
import { colorValue, createQualityGovernor, MAX_NAME_LENGTH, POWER_STOPS } from './minigolf-shared.js';
import { LOOKS, lookOf, tierConfig } from './minigolf-looks.js';
import { createIntro } from './minigolf-camera.js';
import { createSky, createEnvironment } from './minigolf-sky.js';
import { createWater } from './minigolf-water.js';
import { buildProps, createShaderState, patchMaterial } from './minigolf-props.js';
import { createFx } from './minigolf-fx.js';

const [POWER_A, POWER_B, POWER_C] = POWER_STOPS.map(([, hex]) => new THREE.Color(hex));
const powerTint = new THREE.Color();

function powerColor(value) {
    if (value <= .5) powerTint.copy(POWER_A).lerp(POWER_B, value * 2);
    else powerTint.copy(POWER_B).lerp(POWER_C, (value - .5) * 2);
    return powerTint;
}

function nameTag(text) {
    const label = Array.from(String(text || '').trim()).slice(0, MAX_NAME_LENGTH).join('') || 'Player';
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

const PREVIEW_POINTS = 400;
const DRAG_DEAD_ZONE = 14;

function dashedLine(opacity) {
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.BufferAttribute(new Float32Array(PREVIEW_POINTS * 3), 3));
    geometry.setAttribute('lineDistance', new THREE.BufferAttribute(new Float32Array(PREVIEW_POINTS), 1));
    geometry.setDrawRange(0, 0);
    const line = new THREE.Line(geometry, new THREE.LineDashedMaterial({ color: 0xdf914f, transparent: true, opacity, dashSize: .26, gapSize: .18, depthWrite: false }));
    line.visible = false;
    line.frustumCulled = false;
    return line;
}

function fillLine(line, points, from, to) {
    const positions = line.geometry.attributes.position, distances = line.geometry.attributes.lineDistance;
    const count = Math.max(0, Math.min(PREVIEW_POINTS, to - from));
    let total = 0;
    for (let i = 0; i < count; i++) {
        const point = points[from + i], previous = i ? points[from + i - 1] : point;
        total += Math.hypot(point[0] - previous[0], point[1] - previous[1], point[2] - previous[2]);
        positions.setXYZ(i, point[0], point[1] - .145, point[2]);
        distances.setX(i, total);
    }
    positions.needsUpdate = true; distances.needsUpdate = true;
    line.geometry.setDrawRange(0, count);
    return count >= 2;
}

export function createScene(host, options) {
    const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false, powerPreference: 'high-performance' });
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.toneMapping = THREE.NeutralToneMapping;
    renderer.toneMappingExposure = 1;
    // The composer renders several passes per frame; the scene resets the counters itself.
    renderer.info.autoReset = false;
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFShadowMap;
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
    controls.addEventListener('start', () => { userOrbiting = true; camTween = null; skipIntro(); });
    controls.addEventListener('end', () => { userOrbiting = false; });
    // Ambient light comes from the per-look environment map (scene.environment), not a hemisphere light.
    const sun = new THREE.DirectionalLight(0xffe4b6, 1.8);
    sun.position.set(-9, 18, 8);
    sun.castShadow = true;
    sun.shadow.bias = -.0003;
    sun.shadow.normalBias = .025;
    Object.assign(sun.shadow.camera, { left: -9, right: 9, top: 9, bottom: -9, near: .1, far: 60 });
    const rim = new THREE.DirectionalLight(0xffffff, 0);
    scene.add(sun, sun.target, rim);
    const composer = new EffectComposer(renderer);
    const renderPass = new RenderPass(scene, camera);
    const bloom = new UnrealBloomPass(new THREE.Vector2(640, 360), .22, .5, 1.1);
    const output = new OutputPass();
    composer.addPass(renderPass); composer.addPass(bloom); composer.addPass(output);
    const previewLine = dashedLine(.9), previewTail = dashedLine(.45);
    const previewEnd = new THREE.Mesh(new THREE.SphereGeometry(.07, 12, 8), new THREE.MeshBasicMaterial({ color: 0xffedb3, transparent: true, opacity: .9, depthWrite: false }));
    previewEnd.visible = false;
    let preview = null, previewDirty = false, lastPreviewPower = -1;
    const forward = new THREE.Vector3();
    const powerRing = new THREE.Mesh(new THREE.RingGeometry(.56, .7, 48), new THREE.MeshBasicMaterial({ color: 0xdf914f, transparent: true, opacity: .85, side: THREE.DoubleSide, depthWrite: false }));
    powerRing.rotation.x = -Math.PI / 2;
    powerRing.visible = false;
    const restMark = new THREE.Mesh(new THREE.RingGeometry(.24, .32, 32), new THREE.MeshBasicMaterial({ color: 0xdf914f, transparent: true, opacity: .8, side: THREE.DoubleSide, depthWrite: false }));
    restMark.rotation.x = -Math.PI / 2;
    restMark.visible = false;
    const blob = new THREE.Mesh(new THREE.CircleGeometry(.24, 24), new THREE.MeshBasicMaterial({ color: 0x07130e, transparent: true, opacity: .26, depthWrite: false }));
    blob.rotation.x = -Math.PI / 2;
    blob.visible = false;
    scene.add(previewLine, previewTail, previewEnd, powerRing, restMark, blob);
    const ball = new THREE.Group();
    scene.add(ball);
    const ghosts = new THREE.Group();
    scene.add(ghosts);
    const ray = new THREE.Raycaster(), pointer = new THREE.Vector2(), floor = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0);
    const hit = new THREE.Vector3(), puttOffset = new THREE.Vector3();
    let course = null, courseRoot = null, disposed = false, generation = 0, request = null;
    let quality = options.quality || 'auto', reduced = !!options.reducedMotion, cameraMode = false, canAim = false;
    let followEnabled = true, aimCamEnabled = false, camTween = null, userOrbiting = false;
    let aimDirection = [0, -1], aimPower = .35, pointerId = null, dragged = false, dragStart = null;
    let obstacleNodes = new Map(), flagNodes = [], windmill = null, putter = null, swing = 0, elapsed = 0;
    const governor = createQualityGovernor(), screenPoint = new THREE.Vector3();
    // Chosen quality and effective tier stay separate; auto follows the governor.
    const tier = () => quality === 'auto' ? governor.tier : quality;
    const cfg = () => tierConfig(tier());
    // One shader clock and motion switch feed the sky, water and every patched prop material.
    const shaderState = createShaderState();
    const fx = createFx(scene, LOOKS.pirate);
    fx.setReduced(reduced);
    const kits = new Map(), environments = new Map(), fxContext = { ball: ball.position, moving: false, speed: 0, color: undefined, surface: undefined, waterLevel: -1.15, groundY: 0 };
    let lookId = 'pirate', look = LOOKS.pirate, sky = null, water = null, props = null;
    let intro = null, introTime = 0, pendingTurn = null;
    // Last colour id applied to the ball meshes and handed to the effects; null forces the next lookup.
    let ballColorKey = null, fxColorKey = null;
    const introPosition = [0, 0, 0], introTarget = [0, 0, 0];
    // Props kits are fetched once per look; a failed fetch resolves to null so a course still plays without props.
    function kitFor(id) {
        if (!kits.has(id)) kits.set(id, fetch(options.assetURL(`/3d/minigolf/props-${id}.glb`))
            .then(response => { if (!response.ok) throw Error('kit'); return response.arrayBuffer(); })
            .then(bytes => new GLTFLoader().parseAsync(bytes, ''))
            .then(gltf => gltf.scene)
            .catch(() => { kits.delete(id); return null; }));
        return kits.get(id);
    }

    function resize() {
        if (disposed) return;
        const width = Math.max(1, host.clientWidth), height = Math.max(1, host.clientHeight);
        const dpr = Math.min(devicePixelRatio || 1, tier() === 'high' ? 2 : tier() === 'low' ? 1 : 1.5);
        renderer.setPixelRatio(dpr); renderer.setSize(width, height, false);
        composer.setPixelRatio(dpr); composer.setSize(width, height);
        bloom.setSize(Math.max(1, width >> 1), Math.max(1, height >> 1));
        camera.aspect = width / height; camera.updateProjectionMatrix();
        fx.setScale(height * dpr, camera.fov);
    }
    function applyTier() {
        const level = tier();
        const shadows = level !== 'low';
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
        const shadowSize = level === 'high' ? 2048 : 1024;
        if (sun.shadow.mapSize.x !== shadowSize) {
            sun.shadow.map?.dispose(); sun.shadow.map = null;
            sun.shadow.mapSize.set(shadowSize, shadowSize);
        }
        const next = cfg();
        sky?.setTier(next); water?.setTier(next); fx.setTier(next);
        shaderState.golfMotion.value = next.motion && !reduced ? 1 : 0;
        if (props && course && props.scatter !== next.scatter) void rebuildProps();
        resize();
    }
    // Rebuilds the instanced props, for example when a tier change adds or drops scatter decoration.
    async function rebuildProps() {
        const target = course, id = lookId;
        const kit = target?.props?.length ? await kitFor(id) : null;
        // A newer course load owns the props; this rebuild is stale.
        if (disposed || course !== target) return;
        props?.dispose(); props = null;
        if (kit) { props = buildProps(kit, id, target.props, { scatter: cfg().scatter, state: shaderState }); scene.add(props.group); }
    }
    function setQuality(value) {
        quality = ['auto', 'low', 'medium', 'high'].includes(value) ? value : 'auto';
        if (quality === 'auto') governor.reset(elapsed);
        applyTier();
    }
    function updateAim() {
        const aiming = canAim && !cameraMode;
        powerRing.visible = aiming;
        const points = aiming && preview?.points?.length >= 2 ? preview.points : null;
        if (points && (previewDirty || lastPreviewPower !== aimPower)) {
            const split = preview.contactIndex >= 0 ? preview.contactIndex + 1 : points.length;
            const tint = powerColor(aimPower);
            previewLine.userData.ready = fillLine(previewLine, points, 0, split);
            previewTail.userData.ready = fillLine(previewTail, points, Math.max(0, split - 1), points.length);
            previewLine.material.color.copy(tint);
            previewTail.material.color.copy(tint);
            const end = points[points.length - 1];
            previewEnd.position.set(end[0], end[1] - .1, end[2]);
            previewEnd.material.color.setHex(preview.hazard ? 0xff6b5e : 0xffedb3);
            previewDirty = false;
            lastPreviewPower = aimPower;
        }
        previewLine.visible = !!points && !!previewLine.userData.ready;
        previewTail.visible = !!points && !!previewTail.userData.ready;
        previewEnd.visible = !!points;
        if (aiming) {
            const ground = ball.position.y - .155;
            powerRing.position.set(ball.position.x, ground + .012, ball.position.z);
            powerRing.scale.setScalar(.4 + 1.05 * aimPower);
            powerRing.material.color.copy(powerColor(aimPower));
        }
        if (putter) {
            putter.visible = canAim || swing > 0;
            putter.position.copy(ball.position).add(puttOffset.set(-aimDirection[0] * (.38 + swing * .25), -.13, -aimDirection[1] * (.38 + swing * .25)));
            putter.rotation.y = Math.atan2(aimDirection[0], aimDirection[1]);
        }
    }
    function setPreview(result) {
        preview = result && Array.isArray(result.points) ? result : null;
        previewDirty = true;
        updateAim();
    }
    function setAim(direction, power, enabled = true) {
        aimDirection = direction; aimPower = power; canAim = enabled;
        updateAim();
    }
    function focusBall(overview = false) {
        if (intro) {
            // Ends the flight in the end pose; a turn framing queued meanwhile runs afterwards.
            const turn = pendingTurn;
            intro = null; pendingTurn = null;
            focusBall(true);
            if (turn) frameTurn(...turn);
            return;
        }
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
        if (intro) { pendingTurn = [angle, position ? Array.from(position) : position]; return; }
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
    // Ball (or given point) in CSS pixels relative to the stage; visible is false behind the camera or outside.
    function screenPosition(position) {
        if (position?.length >= 3) screenPoint.set(position[0], position[1], position[2]); else screenPoint.copy(ball.position);
        screenPoint.project(camera);
        const width = host.clientWidth, height = host.clientHeight;
        const x = (screenPoint.x + 1) * width / 2, y = (1 - screenPoint.y) * height / 2;
        return { x, y, visible: Number.isFinite(x) && Number.isFinite(y) && screenPoint.z > -1 && screenPoint.z < 1 && x >= 0 && x <= width && y >= 0 && y <= height };
    }
    function setBall(position, rotation, color) {
        ball.position.fromArray(position);
        if (rotation) ball.quaternion.fromArray(rotation);
        // update() calls this every frame; only a colour change walks the ball and resolves the palette.
        const colorKey = color ?? '';
        if (colorKey !== ballColorKey) {
            ballColorKey = colorKey;
            const hex = colorValue(color);
            ball.traverse(node => { if (node.isMesh) node.material.color.set(hex); });
        }
        blob.position.set(ball.position.x, ball.position.y - .152, ball.position.z);
        updateAim();
    }
    function setGhosts(players, active) {
        disposeTree(ghosts); ghosts.clear();
        players.forEach((player, index) => {
            if (index === active || player.done || !player.position) return;
            const mark = new THREE.Mesh(new THREE.SphereGeometry(.16, 12, 8), new THREE.MeshBasicMaterial({ color: colorValue(player.color), transparent: true, opacity: .33, depthWrite: false }));
            mark.position.fromArray(player.position); ghosts.add(mark);
            const tag = nameTag(player.name);
            tag.position.set(player.position[0], player.position[1] + .46, player.position[2]);
            ghosts.add(tag);
        });
    }
    function applyLook() {
        scene.background = new THREE.Color(look.background);
        scene.fog = look.fog ? new THREE.FogExp2(look.fog.color, look.fog.density) : null;
        renderer.toneMappingExposure = look.exposure;
        sun.color.set(look.sun.color); sun.intensity = look.sun.intensity;
        sun.position.fromArray(look.sun.direction).normalize().multiplyScalar(22); sun.target.position.set(0, 0, 0);
        rim.intensity = look.rim?.intensity ?? 0;
        if (look.rim) { rim.color.set(look.rim.color); rim.position.fromArray(look.rim.direction).normalize().multiplyScalar(20); }
        if (!environments.has(lookId)) environments.set(lookId, createEnvironment(renderer, look));
        scene.environment = environments.get(lookId).texture;
        scene.environmentIntensity = look.environment;
        bloom.strength = look.bloom.strength; bloom.radius = look.bloom.radius; bloom.threshold = look.bloom.threshold;
        fx.setLook(look);
    }
    async function load(next) {
        const version = ++generation;
        request?.abort(); request = new AbortController();
        const nextLook = lookOf(next);
        const kitPromise = next.props?.length ? kitFor(nextLook) : Promise.resolve(null);
        const response = await fetch(options.assetURL('/3d/minigolf/' + next.model), { signal: request.signal });
        if (!response.ok) throw Error('course_load');
        const bytes = await response.arrayBuffer();
        if (disposed || generation !== version) return false;
        const [gltf, kit] = await Promise.all([new GLTFLoader().parseAsync(bytes, ''), kitPromise]);
        if (disposed || generation !== version) { disposeTree(gltf.scene); return false; }
        if (courseRoot) { scene.remove(courseRoot); disposeTree(courseRoot); }
        disposeTree(ball); ball.clear(); ballColorKey = null;
        if (putter) { scene.remove(putter); disposeTree(putter); putter = null; }
        props?.dispose(); props = null; sky?.dispose(); water?.dispose();
        course = next; courseRoot = gltf.scene; scene.add(courseRoot);
        restMark.visible = false;
        obstacleNodes = new Map(); flagNodes = []; windmill = null;
        lookId = nextLook; look = LOOKS[lookId];
        applyLook();
        sky = createSky(look, cfg(), shaderState); scene.add(sky.group);
        water = createWater(look, cfg(), shaderState); scene.add(water.group);
        courseRoot.traverse(node => {
            if (node.isMesh) { node.castShadow = true; node.receiveShadow = true; node.material = patchMaterial(node.material, shaderState); }
            if (node.userData?.aurago_hidden) node.visible = false;
            if (node.name === 'water-shore' && node.isMesh) water.attachShore(node);
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
        props = kit ? buildProps(kit, lookId, next.props, { scatter: cfg().scatter, state: shaderState }) : null;
        if (props) scene.add(props.group);
        fx.clear();
        intro = null; pendingTurn = null;
        setBall(next.tee); focusBall(true); resize();
        startIntro();
        return true;
    }
    function overviewPose() {
        const target = new THREE.Vector3(...course.cup).add(new THREE.Vector3(...course.tee)).multiplyScalar(.5);
        return { position: target.clone().add(new THREE.Vector3(10, 13, 14)).toArray(), target: target.toArray() };
    }
    // The intro flight follows the course path from tee to cup and settles on the overview pose.
    function startIntro() {
        intro = null; introTime = 0; pendingTurn = null;
        if (reduced || !course?.path) return;
        const end = overviewPose();
        intro = createIntro(course.path, end.position, end.target);
    }
    function finishIntro() {
        intro = null;
        const turn = pendingTurn; pendingTurn = null;
        if (turn) frameTurn(...turn);
    }
    function skipIntro() {
        if (!intro) return false;
        intro = null;
        focusBall(true);
        finishIntro();
        return true;
    }
    function burst(type, position, speed = 1, info = {}) {
        if (type === 'strike') { swing = .25 + Math.min(1, speed) * .6; skipIntro(); }
        fx.burst(type, position, speed, info);
    }
    function pointAt(event) {
        const rect = canvas.getBoundingClientRect();
        pointer.set((event.clientX - rect.left) / rect.width * 2 - 1, 1 - (event.clientY - rect.top) / rect.height * 2);
        ray.setFromCamera(pointer, camera); floor.constant = -ball.position.y;
        return ray.ray.intersectPlane(floor, hit);
    }
    function ballScreen() {
        const rect = canvas.getBoundingClientRect(), projected = ball.position.clone().project(camera);
        return { x: rect.left + (projected.x + 1) * rect.width / 2, y: rect.top + (1 - projected.y) * rect.height / 2, width: rect.width, height: rect.height };
    }
    function dragDirection(event, screen) {
        if (pointAt(event)) {
            const x = ball.position.x - hit.x, z = ball.position.z - hit.z, length = Math.hypot(x, z);
            if (length > 1e-4) return [x / length, z / length];
        }
        // Ground ray missed: map the screen pull-back through the camera yaw.
        camera.getWorldDirection(forward);
        forward.y = 0;
        if (forward.lengthSq() < 1e-8) return null;
        forward.normalize();
        const sx = screen.x - event.clientX, sy = screen.y - event.clientY;
        const x = sx * -forward.z - sy * forward.x, z = sx * forward.x - sy * forward.z, length = Math.hypot(x, z);
        return length > 1e-6 ? [x / length, z / length] : null;
    }
    function endDrag() {
        if (pointerId === null) return false;
        if (canvas.hasPointerCapture(pointerId)) canvas.releasePointerCapture(pointerId);
        pointerId = null; dragged = false; dragStart = null;
        return true;
    }
    // Releases the pointer and restores the aim from before the press; a press that never changed the aim reports nothing.
    function cancelDrag() {
        if (pointerId === null) return false;
        const restore = dragStart;
        const changed = !!restore && (aimPower !== restore.power || aimDirection[0] !== restore.direction[0] || aimDirection[1] !== restore.direction[1]);
        endDrag();
        if (changed) { setAim(restore.direction, restore.power, canAim); options.onAim?.(aimDirection, aimPower, { phase: 'cancel' }); }
        return true;
    }
    function down(event) {
        skipIntro();
        // A second finger (pinch/pan) or another mouse button ends the drag without a shot.
        if (!event.isPrimary || event.button !== 0) { cancelDrag(); return; }
        if (!canAim || cameraMode) return;
        if (event.pointerType !== 'touch') {
            const screen = ballScreen();
            if (Math.hypot(event.clientX - screen.x, event.clientY - screen.y) > 46) return;
        }
        pointerId = event.pointerId; dragged = false; dragStart = { direction: aimDirection.slice(), power: aimPower };
        canvas.setPointerCapture(pointerId); event.preventDefault(); canvas.focus();
    }
    function move(event) {
        if (pointerId !== event.pointerId) return;
        // A mouse button released outside the window (alt-tab) never delivers pointerup.
        if (!canAim || cameraMode || (event.pointerType === 'mouse' && (!(event.buttons & 1) || (event.buttons & 2)))) { cancelDrag(); return; }
        const screen = ballScreen();
        const pixels = Math.hypot(event.clientX - screen.x, event.clientY - screen.y);
        dragged = pixels >= DRAG_DEAD_ZONE;
        if (!dragged) return;
        const span = Math.max(140, Math.min(320, .3 * Math.min(screen.width, screen.height)));
        const direction = dragDirection(event, screen);
        if (!direction) return;
        setAim(direction, Math.max(.02, Math.min(1, (pixels - DRAG_DEAD_ZONE) / (span - DRAG_DEAD_ZONE))), true);
        options.onAim?.(aimDirection, aimPower, { phase: 'drag' });
    }
    function up(event) {
        if (pointerId !== event.pointerId) return;
        const screen = ballScreen();
        const outside = Math.hypot(event.clientX - screen.x, event.clientY - screen.y) >= DRAG_DEAD_ZONE;
        if (!(event.type === 'pointerup' && event.button === 0 && dragged && outside && aimPower > 0 && canAim)) { cancelDrag(); return; }
        endDrag();
        options.onShot?.(aimDirection, aimPower);
    }
    const lost = event => { event.preventDefault(); options.onError?.('webgl_error'); };
    const wheel = () => { skipIntro(); };
    canvas.addEventListener('wheel', wheel, { passive: true });
    canvas.addEventListener('pointerdown', down); canvas.addEventListener('pointermove', move);
    canvas.addEventListener('pointerup', up); canvas.addEventListener('pointercancel', up);
    canvas.addEventListener('webglcontextlost', lost);
    const observer = new ResizeObserver(resize); observer.observe(host);
    setQuality(quality);

    function update(dt, snapshot) {
        if (disposed) return;
        elapsed += dt;
        if (snapshot?.position) setBall(snapshot.position, snapshot.rotation, snapshot.color);
        for (const pose of snapshot?.obstacles || []) {
            const node = obstacleNodes.get(pose.id);
            if (node) { node.position.fromArray(pose.position); node.quaternion.fromArray(pose.rotation); }
        }
        swing = Math.max(0, swing - dt * 3);
        const motion = !reduced && cfg().motion;
        if (motion) shaderState.golfTime.value += dt;
        if (!reduced) {
            if (windmill) windmill.rotation.z += dt * .35;
            for (const flag of flagNodes) flag.node.rotation.z = flag.z + Math.sin(elapsed * (2.2 + fx.flagBoost * 3)) * (.045 + fx.flagBoost * .12);
        }
        if (motion) props?.update(elapsed);
        const velocity = snapshot?.velocity;
        fxContext.moving = !!snapshot?.moving;
        fxContext.speed = velocity ? Math.hypot(velocity[0], velocity[1], velocity[2]) : 0;
        const fxKey = snapshot?.color ?? '';
        if (fxKey !== fxColorKey) { fxColorKey = fxKey; fxContext.color = colorValue(snapshot?.color); }
        fxContext.surface = snapshot?.surface;
        fxContext.waterLevel = course?.water?.[0]?.max?.[1] ?? -1.15;
        fxContext.groundY = (course?.tee?.[1] ?? .16) - .16;
        fx.update(dt, fxContext);
        if (intro && !userOrbiting) {
            introTime += dt;
            const done = intro.sample(introTime, introPosition, introTarget);
            camera.position.fromArray(introPosition); controls.target.fromArray(introTarget);
            if (done) finishIntro();
        }
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
        renderer.info.reset();
        if (tier() === 'low') renderer.render(scene, camera); else composer.render();
        const change = governor.frame(dt, elapsed);
        if (change && quality === 'auto') { applyTier(); if (change === 'down') options.onQuality?.('low'); }
    }
    return {
        canvas, load, update, setBall, setGhosts, setAim, focusBall, burst, setQuality, frameTurn, setRestMark, panAt, setPreview, cancelDrag, screenPosition, skipIntro,
        setFollow(value) { followEnabled = !!value; },
        setAimCamera(value) { aimCamEnabled = !!value; if (!value) camTween = null; },
        setReducedMotion(value) {
            reduced = !!value;
            fx.setReduced(reduced);
            shaderState.golfMotion.value = cfg().motion && !reduced ? 1 : 0;
            if (reduced) skipIntro();
        },
        setCameraMode(value) { cameraMode = !!value; controls.touches.ONE = value ? THREE.TOUCH.ROTATE : null; controls.mouseButtons.LEFT = value ? THREE.MOUSE.ROTATE : null; updateAim(); },
        stats() { const effects = fx.stats(); return { fps: governor.fps, triangles: renderer.info.render.triangles, calls: renderer.info.render.calls, particles: effects.particles, quality, tier: tier(), geometries: renderer.info.memory.geometries, preview: preview?.points?.length || 0,
            look: lookId, props: props?.instances || 0, env: !!scene.environment, intro: !!intro, trail: effects.trail,
            camera: { position: camera.position.toArray(), target: controls.target.toArray(), ball: ball.position.toArray(), projectedBall: ball.position.clone().project(camera).toArray(), distance: camera.position.distanceTo(controls.target) } }; },
        dispose() {
            if (disposed) return; disposed = true; generation++; request?.abort(); observer.disconnect(); controls.dispose();
            canvas.removeEventListener('pointerdown', down); canvas.removeEventListener('pointermove', move); canvas.removeEventListener('pointerup', up); canvas.removeEventListener('pointercancel', up); canvas.removeEventListener('webglcontextlost', lost);
            // Props, sky, water and effects leave the scene first so shared kit geometry is freed exactly once below.
            props?.dispose(); sky?.dispose(); water?.dispose(); fx.dispose();
            for (const target of environments.values()) target.dispose();
            for (const pending of kits.values()) pending.then(kit => kit && disposeTree(kit));
            canvas.removeEventListener('wheel', wheel);
            disposeTree(scene); sun.shadow.dispose(); bloom.dispose(); renderPass.dispose(); output.dispose(); composer.dispose(); renderer.dispose(); renderer.forceContextLoss(); canvas.remove();
        }
    };
}
