import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/addons/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/addons/postprocessing/UnrealBloomPass.js';
import { OutputPass } from 'three/addons/postprocessing/OutputPass.js';
import { colorValue, createQualityGovernor, flagPolePivot, MAX_NAME_LENGTH, POWER_STOPS } from './minigolf-shared.js';
import { LOOKS, lookOf, tierConfig } from './minigolf-looks.js';
import { createIntro, overviewFor } from './minigolf-camera.js';
import { readColliders } from './minigolf-collision.js';
import { createSky, createEnvironment } from './minigolf-sky.js';
import { createWater } from './minigolf-water.js';
import { buildProps, createShaderState, patchMaterial } from './minigolf-props.js';
import { createFx } from './minigolf-fx.js';
import { createPirateSeaLife } from './minigolf-sealife.js';

const [POWER_A, POWER_B, POWER_C] = POWER_STOPS.map(([, hex]) => new THREE.Color(hex));
const powerTint = new THREE.Color();
// How far past the course bounds (metres) a panned view may centre.
const PAN_MARGIN = 3;

// Move the flag's origin onto its hoist and remember the rest yaw. A later turn
// about local Y then swings only the fly; the hoist stays on the pole.
function pinFlagToPole(node) {
    const attribute = node.isMesh && node.geometry?.getAttribute('position');
    if (!attribute || attribute.count < 2) return null;
    const positions = new Array(attribute.count);
    for (let i = 0; i < attribute.count; i++) positions[i] = [attribute.getX(i), attribute.getY(i), attribute.getZ(i)];
    const pivot = flagPolePivot(positions);
    if (!pivot) return null;
    for (let i = 0; i < attribute.count; i++) attribute.setXYZ(i, positions[i][0] - pivot[0], positions[i][1] - pivot[1], positions[i][2] - pivot[2]);
    attribute.needsUpdate = true;
    node.geometry.computeBoundingBox();
    node.geometry.computeBoundingSphere();
    const offset = new THREE.Vector3(pivot[0], pivot[1], pivot[2]).multiply(node.scale).applyQuaternion(node.quaternion);
    node.position.add(offset);
    return { node, base: node.rotation.y };
}

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
    sprite.userData.aspect = surface.width / surface.height;
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

const DRAG_DEAD_ZONE = 14;
// Aim dots: a chain of instanced spheres along the predicted path. The radius grows with distance so the dots keep a minimum
// on-screen size from the overview camera and stay small in the close aim camera.
const AIM_DOT_SPACING = .28, AIM_DOT_FIRST = .32, AIM_DOT_MAX = 96, AIM_DOT_RADIUS = .045, AIM_DOT_MAX_RADIUS = .09, AIM_DOT_MIN_PX = 2.5, TAIL_SCALE = .75, TAIL_TINT = .55;
const FLAG_NEAR = 1.8, FLAG_FAR = 2.2, FLAG_OPEN = .6, FLAG_RATE = 1 / .25;
const GLIDE_TIME = .4, SHAKE_TIME = .25, POP_TIME = .3, ORBIT_SWEEP = Math.PI * 2 / 3, ORBIT_BLEND = .4;
const smoothstep = t => t * t * (3 - 2 * t);
// easeOutBack with a 1.08 overshoot
const popEase = t => { const u = t - 1; return 1 + 2.5 * u * u * u + 1.5 * u * u; };

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
    // A left drag that misses the ball pans along the ground (a press on the ball aims, see down()).
    controls.mouseButtons.LEFT = THREE.MOUSE.PAN;
    controls.mouseButtons.RIGHT = THREE.MOUSE.ROTATE;
    controls.screenSpacePanning = false;
    controls.touches.ONE = null;
    controls.touches.TWO = THREE.TOUCH.DOLLY_PAN;
    // Only a hand zoom keeps the camera distance after a shot (otherwise the intro overview distance would stay); wheel dollies apply
    // during later damped updates, so the distance is watched for ~1.2 s after 'start' instead of compared at 'end'. Not armed during the intro.
    let userZoomed = false, zoomWatch = 0, zoomRef = 0;
    controls.addEventListener('start', () => {
        const idle = !intro;
        userOrbiting = true; camTween = null; orbit = null; skipIntro();
        if (idle) { if (!(zoomWatch > 0)) zoomRef = camera.position.distanceTo(controls.target); zoomWatch = 1.2; }
    });
    controls.addEventListener('end', () => { userOrbiting = false; });
    // Keeps a panned view over the course: the target stays within the bounds plus a margin, and the camera moves by the
    // same amount so the view direction does not change. The intro flight is not clamped.
    controls.addEventListener('change', () => {
        const bounds = course?.bounds;
        if (intro || !bounds) return;
        const [minX, , minZ] = bounds.min || [], [maxX, , maxZ] = bounds.max || [];
        if (![minX, minZ, maxX, maxZ].every(Number.isFinite)) return;
        const target = controls.target;
        const dx = THREE.MathUtils.clamp(target.x, minX - PAN_MARGIN, maxX + PAN_MARGIN) - target.x;
        const dz = THREE.MathUtils.clamp(target.z, minZ - PAN_MARGIN, maxZ + PAN_MARGIN) - target.z;
        if (!dx && !dz) return;
        target.x += dx; target.z += dz;
        camera.position.x += dx; camera.position.z += dz;
    });
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
    const aimDots = new THREE.InstancedMesh(new THREE.SphereGeometry(1, 10, 6), new THREE.MeshBasicMaterial({ transparent: true, opacity: .92, depthWrite: false }), AIM_DOT_MAX);
    aimDots.instanceMatrix.setUsage(THREE.DynamicDrawUsage); aimDots.setColorAt(0, new THREE.Color(1, 1, 1));
    aimDots.renderOrder = 2; aimDots.frustumCulled = false; aimDots.count = 0; aimDots.visible = false;
    // Sampled dot centres (x, ground y, z) and whether each belongs to the tail after the first contact.
    const aimPoints = new Float32Array(AIM_DOT_MAX * 3), aimTail = new Uint8Array(AIM_DOT_MAX), dotMatrix = new THREE.Matrix4(), dotPosition = new THREE.Vector3(), dotScale = new THREE.Vector3(), dotRotation = new THREE.Quaternion(), dotColor = new THREE.Color();
    let aimCount = 0;
    const cupGlow = new THREE.Mesh(new THREE.RingGeometry(.35, .43, 48).rotateX(-Math.PI / 2), new THREE.MeshBasicMaterial({ color: 0xffd35a, transparent: true, opacity: .8, side: THREE.DoubleSide, depthWrite: false }));
    cupGlow.renderOrder = 2; cupGlow.visible = false;
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
    scene.add(aimDots, cupGlow, previewEnd, powerRing, restMark, blob);
    const ball = new THREE.Group();
    scene.add(ball);
    const ghosts = new THREE.Group();
    scene.add(ghosts);
    const ray = new THREE.Raycaster(), pointer = new THREE.Vector2(), floor = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0);
    const hit = new THREE.Vector3(), puttOffset = new THREE.Vector3();
    let course = null, courseRoot = null, disposed = false, generation = 0, request = null;
    let quality = options.quality || 'auto', reduced = !!options.reducedMotion, cameraMode = false, canAim = false;
    let followEnabled = true, aimCamEnabled = false, camTween = null, userOrbiting = false;
    // Camera orbit around the cup (ace), camera shake, ball pop and the flag cloth opening near the ball.
    let orbit = null, shakeRest = 0, shakeStrength = 0, popTime = Infinity, ballScale = 1, flagLift = 0, flagTarget = 0;
    const shakeOffset = new THREE.Vector3(), orbitPoint = new THREE.Vector3();
    let aimDirection = [0, -1], aimPower = .35, pointerId = null, dragged = false, dragStart = null;
    // Opponent name tags: { sprite, x, y, z } of the ghost each tag floats over; resized every frame (tagFit).
    let tags = [], cursor = '';
    function setCursor(value) { if (value !== cursor) { cursor = value; canvas.style.cursor = value; } }
    let obstacleNodes = new Map(), flagNodes = [], spinNodes = [], putter = null, swing = 0, elapsed = 0;
    // Framing of the loaded island (overviewFor): overview pose, shadow camera size and open water radius.
    let overview = null;
    const governor = createQualityGovernor(), screenPoint = new THREE.Vector3();
    // Chosen quality and effective tier stay separate; auto follows the governor.
    const tier = () => quality === 'auto' ? governor.tier : quality;
    const cfg = () => tierConfig(tier());
    // One shader clock and motion switch feed the sky, water and every patched prop material.
    const shaderState = createShaderState();
    const fx = createFx(scene, LOOKS.pirate);
    fx.setReduced(reduced);
    const seaAbort = new AbortController();
    async function loadSeaModel(path) {
        const response = await fetch(options.assetURL('/3d/' + path), { signal: seaAbort.signal });
        if (!response.ok) throw Error('sea_asset');
        const bytes = await response.arrayBuffer();
        if (disposed) throw Error('disposed');
        const gltf = await new GLTFLoader().parseAsync(bytes, '');
        return gltf.scene;
    }
    const seaLife = createPirateSeaLife({ load: loadSeaModel });
    scene.add(seaLife.group);
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
        sky?.setTier(next); water?.setTier(next); fx.setTier(next); seaLife.setTier(level);
        sky?.setReduced(reduced);
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
    // Walks the polyline at AIM_DOT_SPACING from AIM_DOT_FIRST; dots on segments from the first contact on belong to the tail.
    function sampleAim(points, contact) {
        let n = 0, walked = 0, next = AIM_DOT_FIRST;
        for (let i = 1; i < points.length && n < AIM_DOT_MAX; i++) {
            const a = points[i - 1], b = points[i], length = Math.hypot(b[0] - a[0], b[1] - a[1], b[2] - a[2]);
            if (length < 1e-6) continue;
            while (n < AIM_DOT_MAX && next <= walked + length) {
                const k = (next - walked) / length;
                aimPoints[n * 3] = a[0] + (b[0] - a[0]) * k; aimPoints[n * 3 + 1] = a[1] + (b[1] - a[1]) * k - .16; aimPoints[n * 3 + 2] = a[2] + (b[2] - a[2]) * k;
                const tail = contact >= 0 && i - 1 >= contact;
                aimTail[n] = tail ? 1 : 0;
                aimDots.setColorAt(n, dotColor.setScalar(tail ? TAIL_TINT : 1));
                n++; next += AIM_DOT_SPACING;
            }
            walked += length;
        }
        aimCount = n; aimDots.count = n;
        if (aimDots.instanceColor) aimDots.instanceColor.needsUpdate = true;
    }
    // Radius in metres that keeps AIM_DOT_MIN_PX on screen, clamped; recomputed every frame because the camera moves.
    function fitAimDots() {
        if (!aimDots.visible) return;
        const span = 2 * Math.tan(camera.fov * Math.PI / 360), height = Math.max(1, host.clientHeight), cam = camera.position;
        for (let i = 0; i < aimCount; i++) {
            const x = aimPoints[i * 3], y = aimPoints[i * 3 + 1], z = aimPoints[i * 3 + 2];
            const distance = Math.max(.1, Math.hypot(x - cam.x, y - cam.y, z - cam.z)), perMeter = height / (span * distance);
            const r = Math.min(AIM_DOT_MAX_RADIUS, Math.max(AIM_DOT_RADIUS, AIM_DOT_MIN_PX / perMeter)) * (aimTail[i] ? TAIL_SCALE : 1);
            dotMatrix.compose(dotPosition.set(x, y + r, z), dotRotation, dotScale.setScalar(r));
            aimDots.setMatrixAt(i, dotMatrix);
        }
        aimDots.instanceMatrix.needsUpdate = true;
    }
    function updateAim() {
        const aiming = canAim && !cameraMode;
        powerRing.visible = aiming;
        const points = aiming && preview?.points?.length >= 2 ? preview.points : null;
        if (points && (previewDirty || lastPreviewPower !== aimPower)) {
            sampleAim(points, preview.contactIndex);
            aimDots.material.color.copy(powerColor(aimPower));
            const end = points[points.length - 1];
            previewEnd.position.set(end[0], end[1] - .1, end[2]);
            previewEnd.material.color.setHex(preview.cup ? look.fx.gold : preview.hazard ? 0xff6b5e : 0xffedb3);
            previewDirty = false;
            lastPreviewPower = aimPower;
            aimDots.visible = aimCount > 0;
            fitAimDots();
        }
        aimDots.visible = !!points && aimCount > 0;
        previewEnd.visible = !!points;
        cupGlow.visible = aiming && !!preview?.cup;
        if (cupGlow.visible) cupGlow.material.opacity = reduced ? .8 : .55 + .35 * Math.sin(elapsed * 4);
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
        if (!enabled) setCursor('');
        updateAim();
    }
    function focusBall(overviewPlan = false) {
        userZoomed = false; zoomWatch = 0;
        if (intro) {
            // Ends the flight in the end pose; a turn framing queued meanwhile runs afterwards.
            const turn = pendingTurn;
            intro = null; pendingTurn = null;
            focusBall(true);
            if (turn) frameTurn(...turn);
            return;
        }
        // Flush pending orbit/pan damping before moving the camera and its target together.
        camTween = null; orbit = null;
        const damping = controls.enableDamping;
        controls.enableDamping = false; controls.update();
        const pose = overviewPlan && course ? overviewPose() : null;
        const offset = pose ? new THREE.Vector3(...pose.position).sub(new THREE.Vector3(...pose.target)) : overviewPlan ? new THREE.Vector3(10, 13, 14)
            : camera.position.clone().sub(controls.target).clampLength(controls.minDistance, 8);
        const target = pose ? new THREE.Vector3(...pose.target) : ball.position.clone();
        controls.target.copy(target);
        camera.position.copy(target).add(offset);
        controls.update();
        controls.enableDamping = damping;
    }
    function frameTurn(angle, position) {
        if (intro) { pendingTurn = [angle, position ? Array.from(position) : position]; return; }
        if (!aimCamEnabled || !position || position.length < 3) return;
        zoomWatch = 0;
        const target = new THREE.Vector3(position[0], position[1], position[2]);
        const dirX = Math.sin(angle), dirZ = -Math.cos(angle);
        const destination = target.clone().add(new THREE.Vector3(-dirX * 6.4, 4.1, -dirZ * 6.4));
        orbit = null;
        if (reduced) {
            camera.position.copy(destination); controls.target.copy(target); controls.update();
            return;
        }
        camTween = { fromPosition: camera.position.clone(), fromTarget: controls.target.clone(), toPosition: destination, toTarget: target, time: 0, duration: .45 };
    }
    // Keeps the current view direction and moves onto the ball: 0.4 s, instant when reduced. The distance stays only after a hand zoom
    // (limit: orbit range), otherwise it is the close-up range 3-8 m.
    function glideToBall() {
        if (intro) return false;
        orbit = null; zoomWatch = 0;
        const offset = camera.position.clone().sub(controls.target).clampLength(controls.minDistance, userZoomed ? controls.maxDistance : 8);
        const toTarget = ball.position.clone(), toPosition = toTarget.clone().add(offset);
        if (reduced) {
            camTween = null;
            camera.position.copy(toPosition); controls.target.copy(toTarget); controls.update();
            return true;
        }
        camTween = { fromPosition: camera.position.clone(), fromTarget: controls.target.clone(), toPosition, toTarget, time: 0, duration: GLIDE_TIME };
        return true;
    }
    // Ace camera: swings 120 degrees around the cup at 35 degrees elevation and then stays; false when nothing starts.
    function orbitCup(seconds = 2) {
        if (reduced || intro || !course?.cup || !(seconds > 0)) return false;
        const cup = course.cup, distance = THREE.MathUtils.clamp(camera.position.distanceTo(controls.target), 4, 7);
        camTween = null; zoomWatch = 0;
        orbit = { cx: cup[0], cy: cup[1], cz: cup[2], azimuth: Math.atan2(camera.position.x - cup[0], camera.position.z - cup[2]), reach: distance * Math.cos(Math.PI * 35 / 180), rise: distance * Math.sin(Math.PI * 35 / 180),
            time: 0, duration: seconds, fromPosition: camera.position.clone(), fromTarget: controls.target.clone() };
        return true;
    }
    function shake(strength) {
        if (reduced || !(strength > 0)) return;
        shakeStrength = Math.max(shakeRest > 0 ? shakeStrength : 0, Math.min(1, strength));
        shakeRest = SHAKE_TIME;
    }
    function popBall() {
        if (reduced) { popTime = Infinity; ballScale = 1; ball.scale.setScalar(1); return; }
        popTime = 0; ballScale = .001; ball.scale.setScalar(ballScale);
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
        disposeTree(ghosts); ghosts.clear(); tags = [];
        players.forEach((player, index) => {
            if (index === active || player.done || !player.position) return;
            const mark = new THREE.Mesh(new THREE.SphereGeometry(.16, 12, 8), new THREE.MeshBasicMaterial({ color: colorValue(player.color), transparent: true, opacity: .33, depthWrite: false }));
            mark.position.fromArray(player.position); ghosts.add(mark);
            const tag = nameTag(player.name);
            tag.position.set(player.position[0], player.position[1] + .46, player.position[2]);
            ghosts.add(tag);
            tags.push({ sprite: tag, x: player.position[0], y: player.position[1], z: player.position[2] });
        });
    }
    // Keeps every tag 20 to 30 px tall on screen (0.34 m when that already fits) and hides it while its ghost sits under the active ball.
    function fitTags() {
        if (!tags.length) return;
        const span = 2 * Math.tan(camera.fov * Math.PI / 360), height = host.clientHeight;
        for (const tag of tags) {
            const sprite = tag.sprite, near = Math.hypot(tag.x - ball.position.x, tag.z - ball.position.z) < .55;
            sprite.visible = !near;
            if (near) continue;
            const perMeter = height / (span * Math.max(.1, sprite.position.distanceTo(camera.position)));
            const size = perMeter > 0 ? THREE.MathUtils.clamp(.34, 20 / perMeter, 30 / perMeter) : .34;
            sprite.scale.set(sprite.userData.aspect * size, size, 1);
            sprite.position.y = tag.y + .3 + size / 2;
        }
    }
    // Sun and shadow camera cover the loaded island: ±9 m around its centre for the small P2 islands, wider for large ones.
    function frameLight() {
        const half = overview?.shadowHalf ?? 9, centre = course?.island?.center, distance = 22 + (half - 9) * 1.5, shadowCamera = sun.shadow.camera;
        sun.target.position.set(Number(centre?.[0]) || 0, 0, Number(centre?.[1]) || 0);
        sun.position.fromArray(look.sun.direction).normalize().multiplyScalar(distance).add(sun.target.position);
        Object.assign(shadowCamera, { left: -half, right: half, top: half, bottom: -half, far: Math.max(60, distance + half * 2 + 12) });
        shadowCamera.updateProjectionMatrix();
    }
    function applyLook() {
        scene.background = new THREE.Color(look.background);
        // The fog thins out with the overview distance of large courses (overviewFor), so the lane stays as clear as in P2.
        scene.fog = look.fog ? new THREE.FogExp2(look.fog.color, look.fog.density * (overview?.fogScale ?? 1)) : null;
        renderer.toneMappingExposure = look.exposure;
        sun.color.set(look.sun.color); sun.intensity = look.sun.intensity;
        frameLight();
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
        // The physics meshes are nodes of the course GLB; the manifest only carries their digest.
        let colliders;
        try { colliders = readColliders(bytes); } catch (_) { throw Error('course_collision'); }
        // A course without floor collision would load but drop the ball through the world.
        if (!colliders.some(group => group.surface === 'grass')) throw Error('course_collision');
        const [gltf, kit] = await Promise.all([new GLTFLoader().parseAsync(bytes, ''), kitPromise]);
        if (disposed || generation !== version) { disposeTree(gltf.scene); return false; }
        if (courseRoot) { scene.remove(courseRoot); disposeTree(courseRoot); }
        disposeTree(ball); ball.clear(); ballColorKey = null;
        if (putter) { scene.remove(putter); disposeTree(putter); putter = null; }
        props?.dispose(); props = null; sky?.dispose(); water?.dispose();
        course = next; courseRoot = gltf.scene; scene.add(courseRoot);
        restMark.visible = false;
        obstacleNodes = new Map(); flagNodes = []; spinNodes = [];
        lookId = nextLook; look = LOOKS[lookId];
        overview = overviewFor(next);
        applyLook();
        sky = createSky(look, cfg(), shaderState); scene.add(sky.group);
        sky.setReduced(reduced);
        water = createWater(look, cfg(), shaderState, overview.waterRadius); scene.add(water.group);
        if (lookId === 'pirate') seaLife.configure({ course: next, look, waterRadius: overview.waterRadius, tier: tier() });
        else seaLife.clear();
        // Large islands are viewed from up to 60 m: widen the orbit limit and the far plane to match.
        const viewDistance = Math.hypot(...overview.position.map((value, a) => value - overview.target[a]));
        controls.maxDistance = Math.max(38, viewDistance * 1.15);
        camera.far = Math.max(180, 150 + viewDistance * 1.3); camera.updateProjectionMatrix();
        courseRoot.traverse(node => {
            if (node.isMesh) { node.castShadow = true; node.receiveShadow = true; node.material = patchMaterial(node.material, shaderState); }
            if (node.userData?.aurago_hidden) node.visible = false;
            const spin = node.userData?.aurago_spin;
            if (spin && Array.isArray(spin.axis) && spin.axis.length === 3 && spin.axis.every(Number.isFinite) && Number.isFinite(spin.speed)) {
                const axis = new THREE.Vector3(...spin.axis);
                if (axis.lengthSq() > 1e-12) spinNodes.push({ node, axis: axis.normalize(), speed: spin.speed });
            }
            if (node.name === 'water-shore' && node.isMesh) water.attachShore(node);
            if (node.name === 'flag' || node.name.startsWith('flag-cloth')) {
                const flag = pinFlagToPole(node);
                if (flag) flagNodes.push(flag);
            }
        });
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
        cupGlow.geometry.dispose();
        const cupRadius = Number.isFinite(next.cupRadius) ? next.cupRadius : .32;
        cupGlow.geometry = new THREE.RingGeometry(cupRadius + .03, cupRadius + .11, 48).rotateX(-Math.PI / 2);
        cupGlow.position.set(next.cup[0], next.cup[1] + .012, next.cup[2]);
        cupGlow.material.color.set(look.fx.gold);
        orbit = null; flagLift = 0; flagTarget = 0; popTime = Infinity; ballScale = 1; ball.scale.setScalar(1); shakeRest = 0; shakeStrength = 0;
        previewDirty = true;
        intro = null; pendingTurn = null; userZoomed = false; zoomWatch = 0;
        setBall(next.tee); focusBall(true); resize();
        startIntro();
        return { colliders };
    }
    const overviewPose = () => overview;
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
        if (type === 'strike') { swing = .25 + Math.min(1, speed) * .6; skipIntro(); fx.resetTrail(); }
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
        pointerId = null; dragged = false; dragStart = null; setCursor('');
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
        if (event.pointerType !== 'touch') setCursor('grabbing');
        // This listener runs first (capture), so a mouse or pen press on the ball never reaches OrbitControls and cannot pan.
        // A touch still does: its first finger must stay known to OrbitControls so a second finger can zoom and pan.
        if (event.pointerType !== 'touch') event.stopImmediatePropagation();
    }
    // Hover feedback for mouse and pen: grab over the ball, grabbing while dragging.
    function hover(event) {
        if (pointerId !== null) { setCursor('grabbing'); return; }
        if (!canAim || cameraMode || event.pointerType === 'touch') { setCursor(''); return; }
        const screen = ballScreen();
        setCursor(Math.hypot(event.clientX - screen.x, event.clientY - screen.y) <= 46 ? 'grab' : '');
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
    canvas.addEventListener('pointerdown', down, { capture: true }); canvas.addEventListener('pointermove', move); canvas.addEventListener('pointermove', hover);
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
        if (!reduced) for (const spin of spinNodes) spin.node.rotateOnAxis(spin.axis, spin.speed * dt);
        // The hole flag is authored in world space. Turning the node about Z orbits
        // it around the origin, so the cloth leaves the pole except while the sine is near zero.
        // The cloth opens while the ball is near the cup (hysteresis between FLAG_NEAR and FLAG_FAR).
        if (course?.cup) {
            const near = Math.hypot(ball.position.x - course.cup[0], ball.position.z - course.cup[2]);
            if (near < FLAG_NEAR) flagTarget = 1; else if (near > FLAG_FAR) flagTarget = 0;
        }
        flagLift = reduced ? flagTarget : flagLift + Math.max(-dt * FLAG_RATE, Math.min(dt * FLAG_RATE, flagTarget - flagLift));
        const wave = reduced ? 0 : Math.sin(elapsed * (2.2 + fx.flagBoost * 3)) * (.045 + fx.flagBoost * .12 + flagLift * .06);
        for (const flag of flagNodes) flag.node.rotation.y = flag.base + wave + flagLift * FLAG_OPEN;
        if (popTime < POP_TIME) {
            popTime += dt;
            ballScale = popTime >= POP_TIME ? 1 : Math.max(.001, popEase(popTime / POP_TIME));
            ball.scale.setScalar(ballScale);
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
        if (orbit && !userOrbiting) {
            orbit.time = Math.min(orbit.duration, orbit.time + dt);
            const progress = orbit.time / orbit.duration, azimuth = orbit.azimuth + ORBIT_SWEEP * smoothstep(progress), blend = smoothstep(Math.min(1, orbit.time / ORBIT_BLEND));
            orbitPoint.set(orbit.cx + Math.sin(azimuth) * orbit.reach, orbit.cy + orbit.rise, orbit.cz + Math.cos(azimuth) * orbit.reach);
            camera.position.lerpVectors(orbit.fromPosition, orbitPoint, blend);
            controls.target.set(orbit.cx, orbit.cy, orbit.cz).sub(orbit.fromTarget).multiplyScalar(blend).add(orbit.fromTarget);
            if (progress >= 1) orbit = null;
        }
        if (followEnabled && snapshot?.moving && !userOrbiting && !camTween && !orbit) {
            controls.target.lerp(ball.position, reduced ? 1 : Math.min(1, dt * 2.5));
        }
        controls.enableDamping = !reduced; controls.update();
        seaLife.update(dt, camera, motion);
        if (zoomWatch > 0) {
            zoomWatch -= dt;
            if (!camTween && !orbit && !intro && Math.abs(camera.position.distanceTo(controls.target) - zoomRef) > .5) { userZoomed = true; zoomWatch = 0; }
        }
        updateAim(); fitAimDots(); fitTags();
        // The shake is added after OrbitControls ran and removed again after rendering, so the controls never adopt it.
        let shaken = false;
        if (shakeRest > 0 && !reduced) {
            shakeRest = Math.max(0, shakeRest - dt);
            const amount = .05 * shakeStrength * (shakeRest / SHAKE_TIME);
            shakeOffset.set((Math.random() * 2 - 1) * amount, (Math.random() * 2 - 1) * amount, (Math.random() * 2 - 1) * amount);
            camera.position.add(shakeOffset); shaken = true;
            if (!shakeRest) shakeStrength = 0;
        }
        renderer.info.reset();
        if (tier() === 'low') renderer.render(scene, camera); else composer.render();
        if (shaken) camera.position.sub(shakeOffset);
        const change = governor.frame(dt, elapsed);
        if (change && quality === 'auto') { applyTier(); if (change === 'down') options.onQuality?.('low'); }
    }
    return {
        canvas, load, update, setBall, setGhosts, setAim, focusBall, burst, resetTrail: () => fx.resetTrail(), setQuality, frameTurn, setRestMark, panAt, setPreview, cancelDrag, screenPosition, skipIntro,
        glideToBall, orbitCup, shake, popBall, kickFlag: (amount = 1.2) => fx.kickFlag(amount),
        setFollow(value) { followEnabled = !!value; },
        setAimCamera(value) { aimCamEnabled = !!value; if (!value) camTween = null; },
        setReducedMotion(value) {
            reduced = !!value;
            fx.setReduced(reduced);
            sky?.setReduced(reduced);
            shaderState.golfMotion.value = cfg().motion && !reduced ? 1 : 0;
            if (reduced) { skipIntro(); orbit = null; popTime = Infinity; ballScale = 1; ball.scale.setScalar(1); shakeRest = 0; shakeStrength = 0; }
        },
        setCameraMode(value) { cameraMode = !!value; controls.touches.ONE = value ? THREE.TOUCH.ROTATE : null; controls.mouseButtons.LEFT = value ? THREE.MOUSE.ROTATE : THREE.MOUSE.PAN; setCursor(''); updateAim(); },
        stats() { const effects = fx.stats(); return { fps: governor.fps, triangles: renderer.info.render.triangles, calls: renderer.info.render.calls, particles: effects.particles, quality, tier: tier(), geometries: renderer.info.memory.geometries, preview: preview?.points?.length || 0,
            look: lookId, props: props?.instances || 0, env: !!scene.environment, intro: !!intro, trail: effects.trail,
            aimDots: aimDots.visible ? aimCount : 0, cupGlow: cupGlow.visible, flagLift, orbit: !!orbit, tween: !!camTween, userZoomed, shake: shakeRest, ballScale,
            seaLife: seaLife.stats(), sky: sky?.stats?.(),
            camera: { position: camera.position.toArray(), target: controls.target.toArray(), ball: ball.position.toArray(), projectedBall: ball.position.clone().project(camera).toArray(), distance: camera.position.distanceTo(controls.target) } }; },
        dispose() {
            if (disposed) return; disposed = true; generation++; request?.abort(); seaAbort.abort(); observer.disconnect(); controls.dispose();
            canvas.removeEventListener('pointerdown', down, { capture: true }); canvas.removeEventListener('pointermove', move); canvas.removeEventListener('pointermove', hover); canvas.removeEventListener('pointerup', up); canvas.removeEventListener('pointercancel', up); canvas.removeEventListener('webglcontextlost', lost);
            // Props, sky, water and effects leave the scene first so shared kit geometry is freed exactly once below.
            seaLife.dispose(); props?.dispose(); sky?.dispose(); water?.dispose(); fx.dispose(); aimDots.dispose();
            for (const target of environments.values()) target.dispose();
            for (const pending of kits.values()) pending.then(kit => kit && disposeTree(kit));
            canvas.removeEventListener('wheel', wheel);
            disposeTree(scene); sun.shadow.dispose(); bloom.dispose(); renderPass.dispose(); output.dispose(); composer.dispose(); renderer.dispose(); renderer.forceContextLoss(); canvas.remove();
        }
    };
}
