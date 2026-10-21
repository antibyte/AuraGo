(function () {
    'use strict';

    // Live floating-isle diorama on its own canvas behind the desktop shell (z-index -1). It owns a
    // three.js renderer, the island meshes, the sky dome, the camera rig, shadows, an optional tilt-shift
    // pass and its own quality ladder. The overlay (voxel-graphics.js) never touches it.
    //
    // Colour handling: the mesher, the sky model and the palettes speak sRGB 0..1; three works in linear
    // light, so every colour that reaches a buffer or a uniform is converted on the way in.

    // ---- Look tunables ------------------------------------------------------------------------
    const FOV = 21;
    const TARGET = [0, 4, 0];             // look-at point in island world units (grid minus origin)
    const BASE_YAW = 40;                  // degrees
    const BASE_PITCH = 26;
    const FIT_HALF_HEIGHT = 34;           // world units shown above and below the target at its distance
    const NARROW_ASPECT = 1.5;            // narrower windows move the camera back by aspect / this
    const VIEW_SHIFT = { minAspect: 1.45, x: .07, y: .05 };   // island shifted right and up (share of the view)
    const PARALLAX = { yaw: 4, pitch: 2, tau: .35 };
    const BREATH = { yaw: 1.2, yawPeriod: 70, pitch: .5, pitchPeriod: 53 };
    const EXPOSURE = 1;
    const SWAY = .045;                    // foliage sway amplitude in world units
    const TEXEL = .14;                    // texel noise strength of the terrain
    const DECO_TALL = 1.6;                // flowers and tufts are this much taller than wide
    const SHADOW = { extent: 40, near: 1, far: 260, bias: -.0004, normalBias: .04, radius: 2, distance: 120, refreshMs: 4000 };
    // Extra light on top of the sky model so dusk, night and the underside never go black: the cloud sea
    // bounces light up onto the underside (ground colour = ground mixed towards the cloud tint, times gain),
    // the sky light gains fill as the key light weakens, and moonlight is stronger.
    const LIGHT = { groundBounce: .7, groundGain: 2.2, fill: 2, nightKey: 1.8 };
    // Added to the ray height before the sky gradient. The camera looks 26 degrees down, so without it the whole picture
    // would be below the horizon. At .64 the horizon colour sits just below the bottom edge and the top of the picture
    // is part way to the zenith colour (no horizon line crosses the view).
    const SKY_LIFT = .64;
    const KEY_TAU = 1.2;                  // seconds the key light needs to swing to a new direction
    const FOG = { near: .95, far: 2.3 };  // multiples of the camera distance
    const TILT = { focusY: .47, band: .16, feather: .28, maxBlur: 3.2, vignette: .16 };
    const SKY_RADIUS = 400;
    const SKY_SUN = { size: .045, core: [4, 3.6, 2.7] };
    const SKY_MOON = { size: .035, color: '#e8ecff', gain: 1.8, cut: [.45, .2] };
    const SKY_STARS = { cells: 220, threshold: .9965 };
    const OPACITY_FADE_MS = 700;

    // ---- Performance tunables -----------------------------------------------------------------
    // dpr: pixel ratio cap; shadow: map size (0 = off); interval: ms between frames (0 = one frame per minute).
    const QUALITY = [
        { dpr: 1.5, shadow: 2048, tilt: true, life: 1, interval: 33 },
        { dpr: 1.25, shadow: 1024, tilt: false, life: 1, interval: 33 },
        { dpr: 1, shadow: 0, tilt: false, life: .5, interval: 42 },
        { dpr: .85, shadow: 0, tilt: false, life: 0, interval: 0 },
        { dpr: 1, shadow: 0, tilt: false, life: 0, interval: 0, hidden: true }
    ];
    const FRAME_SLACK = 3;                // ms; requestAnimationFrame ticks are quantised
    const THROTTLE = { coverage: .96, interval: 500 };   // a window that hides the scene slows it down
    const STATIC_REFRESH_MS = 60000;
    const ADAPT = { window: 30, downCost: 12, upCost: 5, upAfterMs: 25000, gapFactor: 2.2, gapFrames: 45, warmup: 4 };
    const INITIAL_PIXELS = 4.2e6;
    const SOFTWARE_GPU = /swiftshader|llvmpipe|lavapipe|softpipe|software|basic render/i;

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
    function srgbToLinear(c) { return c <= .04045 ? c / 12.92 : Math.pow((c + .055) / 1.055, 2.4); }
    function toLinearArray(source) {
        const out = new Float32Array(source.length);
        for (let i = 0; i < source.length; i++) out[i] = srgbToLinear(source[i]);
        return out;
    }
    function hexRGB(hex) {
        const n = parseInt(hex.slice(1), 16);
        return [(n >> 16 & 255) / 255, (n >> 8 & 255) / 255, (n & 255) / 255];
    }
    // three's ACESFilmicToneMapping on the CPU, for the one colour (the fog) that is blended after tone mapping
    // when the scene is drawn straight to the canvas.
    function acesFilmic(rgb) {
        const k = EXPOSURE / .6;
        const r = rgb[0] * k, g = rgb[1] * k, b = rgb[2] * k;
        const fit = v => (v * (v + .0245786) - .000090537) / (v * (.983729 * v + .432951) + .238081);
        const x = fit(.59719 * r + .35458 * g + .04823 * b);
        const y = fit(.076 * r + .90834 * g + .01566 * b);
        const z = fit(.0284 * r + .13383 * g + .83777 * b);
        return [
            clamp(1.60475 * x - .53108 * y - .07367 * z, 0, 1),
            clamp(-.10208 * x + 1.10813 * y - .00605 * z, 0, 1),
            clamp(-.00327 * x - .07276 * y + 1.07602 * z, 0, 1)
        ];
    }

    const SKY_VERTEX = `
varying vec3 vDir;
void main() {
    vDir = position;
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}`;

    const SKY_FRAGMENT = `
uniform vec3 uTop;
uniform vec3 uHorizon;
uniform vec3 uBottom;
uniform vec3 uKey;
uniform vec3 uSunDir;
uniform vec3 uMoonDir;
uniform float uSunVisible;
uniform float uMoonVisible;
uniform float uStars;
uniform float uTime;
varying vec3 vDir;
float skyHash(vec3 p) { return fract(sin(dot(p, vec3(127.1, 311.7, 74.7))) * 43758.5453); }
void main() {
    vec3 v = normalize(vDir);
    float lifted = v.y + SKY_LIFT;
    vec3 col = lifted > 0.0 ? mix(uHorizon, uTop, pow(min(lifted, 1.0), 0.55)) : mix(uHorizon, uBottom, pow(min(-lifted, 1.0), 0.4));
    vec3 up = vec3(0.0, 1.0, 0.0);
    float sd = dot(v, uSunDir);
    vec3 sr = normalize(cross(uSunDir, up));
    vec3 su = cross(sr, uSunDir);
    vec2 p = vec2(dot(v, sr), dot(v, su)) / SUN_SIZE;
    if (sd > 0.0 && max(abs(p.x), abs(p.y)) < 1.0) col = mix(col, SUN_CORE, uSunVisible);
    col += uKey * (pow(max(sd, 0.0), 64.0) * 0.55 + pow(max(sd, 0.0), 8.0) * 0.12) * uSunVisible;
    float md = dot(v, uMoonDir);
    vec3 mr = normalize(cross(uMoonDir, up));
    vec3 mu = cross(mr, uMoonDir);
    vec2 q = vec2(dot(v, mr), dot(v, mu)) / MOON_SIZE;
    vec2 cutQ = q - MOON_CUT;
    if (md > 0.0 && max(abs(q.x), abs(q.y)) < 1.0 && max(abs(cutQ.x), abs(cutQ.y)) >= 1.0) col = mix(col, MOON_COLOR, uMoonVisible);
    vec3 g = floor(v * STAR_CELLS);
    float h = skyHash(g);
    float star = step(STAR_THRESHOLD, h) * (0.55 + 0.45 * sin(uTime * (1.0 + h * 3.0) + h * 40.0)) * uStars * smoothstep(0.02, 0.2, v.y);
    col += vec3(star);
    gl_FragColor = vec4(col, 1.0);
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
}`;

    const POST_VERTEX = `
varying vec2 vUv;
void main() {
    vUv = uv;
    gl_Position = vec4(position.xy, 0.0, 1.0);
}`;

    // Tilt-shift: a 12-tap golden-angle disc blur that grows towards the top and bottom of the picture, plus a
    // light vignette. It runs in linear light and ends with the renderer's tone mapping and colour conversion.
    const POST_FRAGMENT = `
uniform sampler2D tDiffuse;
uniform vec2 uPx;
uniform float uMaxBlur;
uniform float uBand;
uniform float uFeather;
uniform float uFocusY;
uniform float uVignette;
varying vec2 vUv;
void main() {
    float amount = smoothstep(uBand, uBand + uFeather, abs(vUv.y - uFocusY));
    float radius = uMaxBlur * amount;
    vec3 color = texture2D(tDiffuse, vUv).rgb;
    if (radius > 0.25) {
        vec3 sum = color;
        for (int i = 0; i < 12; i++) {
            float fi = float(i) + 0.5;
            float r = sqrt(fi / 12.0) * radius;
            float a = fi * 2.39996323;
            sum += texture2D(tDiffuse, vUv + vec2(cos(a), sin(a)) * r * uPx).rgb;
        }
        color = sum / 13.0;
    }
    color *= 1.0 - uVignette * pow(length((vUv - 0.5) * vec2(1.1, 1.0)) * 1.25, 2.2);
    gl_FragColor = vec4(color, 1.0);
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
}`;

    function glslFloat(value) { return Number.isInteger(value) ? value.toFixed(1) : String(value); }
    function glslVec3(rgb) { return 'vec3(' + rgb.map(glslFloat).join(', ') + ')'; }

    function skyFragmentSource() {
        const moon = hexRGB(SKY_MOON.color).map(c => srgbToLinear(c) * SKY_MOON.gain);
        return SKY_FRAGMENT
            .replace(/SKY_LIFT/g, glslFloat(SKY_LIFT)).replace(/SUN_SIZE/g, glslFloat(SKY_SUN.size)).replace(/SUN_CORE/g, glslVec3(SKY_SUN.core))
            .replace(/MOON_SIZE/g, glslFloat(SKY_MOON.size)).replace(/MOON_COLOR/g, glslVec3(moon))
            .replace(/MOON_CUT/g, 'vec2(' + SKY_MOON.cut.map(glslFloat).join(', ') + ')')
            .replace(/STAR_CELLS/g, glslFloat(SKY_STARS.cells)).replace(/STAR_THRESHOLD/g, glslFloat(SKY_STARS.threshold));
    }

    function create(options) {
        const THREE = options.THREE;
        const motion = !!options.motion;
        const read = options.readState;
        const noteError = typeof options.noteError === 'function' ? options.noteError : function () {};
        const Island = window.AuraVoxelIsland;
        const Mesher = window.AuraVoxelMesher;
        const SkyModel = window.AuraVoxelSky;
        const Blocks = window.AuraVoxelBlocks;

        const host = document.createElement('div');
        host.className = 'vd-fx-voxel-scene';
        host.setAttribute('aria-hidden', 'true');
        host.style.cssText = 'position:fixed;inset:0;z-index:-1;pointer-events:none;overflow:hidden;opacity:0;' +
            (motion ? 'transition:opacity ' + OPACITY_FADE_MS + 'ms ease;' : '');
        const canvas = document.createElement('canvas');
        canvas.style.cssText = 'display:block;width:100%;height:100%;';
        host.appendChild(canvas);
        document.body.insertBefore(host, document.body.firstChild);

        let renderer = null;
        let scene = null;
        let camera = null;
        let hemi = null;
        let key = null;
        let skyMesh = null;
        let postScene = null;
        let postCamera = null;
        let postMaterial = null;
        let target = null;
        let life = null;
        let island = null;
        let disposed = false;
        let built = false;
        let ready = false;
        let paused = false;
        let contextLost = false;
        let backend = 'loading';
        let gpu = '';
        let tiltCapable = false;
        let buildTimer = 0;
        let raf = 0;
        let staticTimer = 0;
        let quality = 0;
        let initialQuality = 0;
        let qualityLock = null;
        let width = 1;
        let height = 1;
        let dpr = 1;
        let frames = 0;
        let buildMs = 0;
        let triangles = 0;
        let voxels = 0;
        let lastCost = 0;
        let lastRender = 0;
        let lifeTime = 0;
        let frozen = null;
        let hourOverride = null;
        let sky = null;
        let lastSkyAt = -1e9;
        let lastShadowAt = -1e9;
        let shadowDirty = true;
        let shadowOn = false;
        let shadowSize = 0;
        let tiltOn = false;
        let throttled = false;
        let lifeFailed = false;
        let anchors = {};
        let cameraDistance = 215;
        let fogSrgb = [.8, .9, 1];
        const stamps = [];
        const costs = [];
        let warm = ADAPT.warmup;
        let slowGaps = 0;
        let fastSince = 0;
        let parallax = { yaw: 0, pitch: 0, goalYaw: 0, goalPitch: 0 };
        const keyCurrent = new THREE.Vector3(0, 1, 0);
        const keyGoal = new THREE.Vector3(0, 1, 0);
        const shadowDir = new THREE.Vector3(0, 1, 0);
        const disposables = [];
        const materials = {};
        const isletGroups = [];
        const anchorPoints = {};

        const uniforms = {
            uTime: { value: 0 },
            uSway: { value: SWAY },
            uNight: { value: 0 },
            uCloudOffset: { value: new THREE.Vector2(0, 0) },
            uCloudShadow: { value: 0 },
            uSun: { value: 1 },
            uTexel: { value: TEXEL }
        };
        const skyUniforms = {
            uTop: { value: new THREE.Color() }, uHorizon: { value: new THREE.Color() }, uBottom: { value: new THREE.Color() },
            uKey: { value: new THREE.Color() }, uSunDir: { value: new THREE.Vector3(0, 1, 0) }, uMoonDir: { value: new THREE.Vector3(0, 1, 0) },
            uSunVisible: { value: 1 }, uMoonVisible: { value: 0 }, uStars: { value: 0 }, uTime: uniforms.uTime
        };
        const postUniforms = {
            tDiffuse: { value: null }, uPx: { value: new THREE.Vector2(1, 1) }, uMaxBlur: { value: TILT.maxBlur },
            uBand: { value: TILT.band }, uFeather: { value: TILT.feather }, uFocusY: { value: TILT.focusY }, uVignette: { value: TILT.vignette }
        };

        function track(item) { disposables.push(item); return item; }
        function state() {
            try { return read() || {}; } catch (_) { return {}; }
        }
        function setSrgb(color, rgb) { return color.setRGB(rgb[0], rgb[1], rgb[2], THREE.SRGBColorSpace); }
        function hideHost() { host.style.display = 'none'; }
        function showHost() { host.style.display = ''; }

        // ---- Renderer ------------------------------------------------------------------------
        function releaseRenderer() {
            if (!renderer) return;
            try { renderer.dispose(); renderer.forceContextLoss(); } catch (_) { /* the context may already be gone */ }
            renderer = null;
        }

        function createRenderer() {
            try {
                renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: false, powerPreference: 'default', stencil: false, depth: true });
            } catch (error) {
                renderer = null;
                backend = 'error';
                hideHost();
                noteError('scene WebGL initialization', error);
                return false;
            }
            try {
                const gl = renderer.getContext();
                const info = gl.getExtension('WEBGL_debug_renderer_info');
                gpu = String(gl.getParameter(info ? info.UNMASKED_RENDERER_WEBGL : gl.RENDERER) || '');
            } catch (_) { gpu = ''; }
            // Software WebGL (SwiftShader, llvmpipe) cannot carry the diorama: leave the page to the poster wallpaper.
            // The hook lets a browser test or a developer render it on software anyway.
            if (SOFTWARE_GPU.test(gpu) && window.__auraVoxelSoftwareGL !== true) {
                backend = 'software';
                hideHost();
                releaseRenderer();
                return false;
            }
            tiltCapable = !!(renderer.extensions && (renderer.extensions.has('EXT_color_buffer_float') || renderer.extensions.has('EXT_color_buffer_half_float')));
            renderer.setClearColor(0x000000, 1);
            renderer.outputColorSpace = THREE.SRGBColorSpace;
            renderer.toneMapping = THREE.ACESFilmicToneMapping;
            renderer.toneMappingExposure = EXPOSURE;
            renderer.shadowMap.enabled = true;
            renderer.shadowMap.type = THREE.PCFShadowMap;
            renderer.shadowMap.autoUpdate = false;
            canvas.addEventListener('webglcontextlost', event => {
                event.preventDefault();
                if (disposed) return;
                contextLost = true;
                backend = 'context-lost';
                stopLoops();
                host.style.opacity = '0';
                noteError('scene WebGL', 'context lost');
            });
            canvas.addEventListener('webglcontextrestored', () => {
                if (disposed) return;
                contextLost = false;
                backend = 'three-webgl';
                shadowDirty = true;
                host.style.opacity = ready ? '1' : '0';
                startLoops();
            });
            backend = 'three-webgl';
            return true;
        }

        // ---- World ---------------------------------------------------------------------------
        function groupGeometry(group) {
            const geometry = track(new THREE.BufferGeometry());
            geometry.setAttribute('position', new THREE.BufferAttribute(group.positions, 3));
            geometry.setAttribute('normal', new THREE.BufferAttribute(group.normals, 3));
            geometry.setAttribute('color', new THREE.BufferAttribute(toLinearArray(group.colors), 3));
            geometry.setAttribute('uv', new THREE.BufferAttribute(group.uvs, 2));
            geometry.setAttribute('aAo', new THREE.BufferAttribute(group.ao, 1));
            geometry.setAttribute('aSeed', new THREE.BufferAttribute(group.seeds, 1));
            if (group.emissive) geometry.setAttribute('aGlow', new THREE.BufferAttribute(toLinearArray(group.emissive), 3));
            geometry.setIndex(new THREE.BufferAttribute(group.indices, 1));
            geometry.computeBoundingSphere();
            return geometry;
        }

        function addGrid(grid, parent) {
            const meshed = Mesher.mesh(grid, Island.MATERIAL_INFO);
            for (const name of ['solid', 'foliage', 'glow', 'water']) {
                const group = meshed.groups[name];
                if (!group || !group.faceCount) continue;
                const mesh = new THREE.Mesh(groupGeometry(group), materials[name]);
                mesh.castShadow = name !== 'water';
                mesh.receiveShadow = true;
                mesh.frustumCulled = false;   // the foliage sways and the water moves
                triangles += group.indices.length / 3;
                parent.add(mesh);
            }
        }

        function addDecorations(parent, list) {
            if (!list.length) return;
            const geometry = track(new THREE.BoxGeometry(1, 1, 1));
            const mesh = new THREE.InstancedMesh(geometry, materials.deco, list.length);
            const matrix = new THREE.Matrix4();
            const position = new THREE.Vector3();
            const scale = new THREE.Vector3();
            const quaternion = new THREE.Quaternion();
            const color = new THREE.Color();
            list.forEach((item, i) => {
                const tall = item.size * DECO_TALL;
                position.set(item.x + .5 - island.w / 2, item.y + tall / 2, item.z + .5 - island.d / 2);
                scale.set(item.size, tall, item.size);
                matrix.compose(position, quaternion, scale);
                mesh.setMatrixAt(i, matrix);
                mesh.setColorAt(i, color.setStyle(item.color));
            });
            mesh.instanceMatrix.needsUpdate = true;
            if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
            mesh.frustumCulled = false;
            mesh.receiveShadow = true;
            triangles += 12 * list.length;
            parent.add(mesh);
        }

        function worldPoint(x, y, z) { return new THREE.Vector3(x + .5 - island.w / 2, y, z + .5 - island.d / 2); }

        function buildWorld() {
            const patch = (material, kind) => track(Blocks.patchTerrainMaterial(THREE, material, uniforms, kind));
            materials.solid = patch(new THREE.MeshLambertMaterial({ vertexColors: true }), 'solid');
            materials.foliage = patch(new THREE.MeshLambertMaterial({ vertexColors: true }), 'foliage');
            materials.glow = patch(new THREE.MeshLambertMaterial({ vertexColors: true }), 'glow');
            materials.water = patch(new THREE.MeshStandardMaterial({ vertexColors: true, transparent: true, opacity: .84, roughness: .18, metalness: 0 }), 'water');
            materials.deco = patch(new THREE.MeshLambertMaterial({ color: 0xffffff }), 'deco');
            island = Island.generate();
            const world = new THREE.Group();
            scene.add(world);
            addGrid(island, world);
            addDecorations(world, island.decorations);
            voxels = island.solidCount;
            for (const islet of island.islets) {
                const group = new THREE.Group();
                group.position.set(islet.offset[0], islet.offset[1], islet.offset[2]);
                addGrid(islet, group);
                world.add(group);
                isletGroups.push(group);
                voxels += islet.solidCount;
            }
            const f = island.features;
            const windows = f.windows || [];
            const village = windows.length
                ? windows.reduce((sum, w) => [sum[0] + w[0] / windows.length, sum[1] + w[1] / windows.length, sum[2] + w[2] / windows.length], [0, 0, 0])
                : [35, 6, 28];
            anchorPoints.waterfall = worldPoint(f.waterfall.x + 2, f.waterfall.y - 8, f.waterfall.z);
            anchorPoints.village = worldPoint(village[0], village[1], village[2]);
            anchorPoints.windmill = worldPoint(f.windmill.hub[0], f.windmill.hub[1], f.windmill.hub[2]);
            anchorPoints.lake = worldPoint(f.lake.x, 4, f.lake.z);
        }

        function buildLighting() {
            hemi = new THREE.HemisphereLight(0xffffff, 0x444444, 1);
            key = new THREE.DirectionalLight(0xffffff, 1);
            key.castShadow = true;
            key.shadow.camera.left = -SHADOW.extent;
            key.shadow.camera.right = SHADOW.extent;
            key.shadow.camera.top = SHADOW.extent;
            key.shadow.camera.bottom = -SHADOW.extent;
            key.shadow.camera.near = SHADOW.near;
            key.shadow.camera.far = SHADOW.far;
            key.shadow.camera.updateProjectionMatrix();
            key.shadow.bias = SHADOW.bias;
            key.shadow.normalBias = SHADOW.normalBias;
            key.shadow.radius = SHADOW.radius;
            key.target.position.set(TARGET[0], TARGET[1], TARGET[2]);
            scene.add(hemi, key, key.target);
        }

        function buildSky() {
            const material = track(new THREE.ShaderMaterial({
                uniforms: skyUniforms, vertexShader: SKY_VERTEX, fragmentShader: skyFragmentSource(),
                side: THREE.BackSide, depthWrite: false, depthTest: false, fog: false
            }));
            skyMesh = new THREE.Mesh(track(new THREE.SphereGeometry(SKY_RADIUS, 32, 16)), material);
            skyMesh.frustumCulled = false;
            skyMesh.renderOrder = -1000;
            scene.add(skyMesh);
        }

        function buildPost() {
            postMaterial = track(new THREE.ShaderMaterial({
                uniforms: postUniforms, vertexShader: POST_VERTEX, fragmentShader: POST_FRAGMENT,
                toneMapped: true, depthTest: false, depthWrite: false
            }));
            const quad = new THREE.Mesh(track(new THREE.PlaneGeometry(2, 2)), postMaterial);
            quad.frustumCulled = false;
            postScene = new THREE.Scene();
            postScene.add(quad);
            postCamera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1);
        }

        function buildLife() {
            if (!window.AuraVoxelLife || typeof window.AuraVoxelLife.create !== 'function') return;
            try {
                // quality is the life scale of the current level: 1, .5 or 0
                life = window.AuraVoxelLife.create({
                    THREE, scene, island, origin: [island.w / 2, island.yBase, island.d / 2], uniforms,
                    patch: Blocks, islets: isletGroups, quality: QUALITY[quality].life
                });
            } catch (error) {
                life = null;
                noteError('scene life', error);
            }
        }

        // ---- Sky, light, camera --------------------------------------------------------------
        function applyFog() {
            if (!scene || !scene.fog) return;
            const linear = fogSrgb.map(srgbToLinear);
            // Drawn straight to the canvas the fog is mixed in after tone mapping; the tilt-shift pass tone-maps it later.
            const rgb = tiltOn ? linear : acesFilmic(linear);
            scene.fog.color.setRGB(rgb[0], rgb[1], rgb[2], THREE.LinearSRGBColorSpace);
        }

        function updateSky(snap, now) {
            const hour = hourOverride !== null ? hourOverride : SkyModel.hourFromDate();
            sky = SkyModel.at(hour);
            lastSkyAt = now;
            setSrgb(hemi.color, sky.hemiSky);
            setSrgb(hemi.groundColor, sky.hemiGround.map((c, i) => c + (sky.cloudTint[i] - c) * LIGHT.groundBounce)).multiplyScalar(LIGHT.groundGain);
            hemi.intensity = sky.hemiIntensity * (1 + LIGHT.fill * (1 - clamp(sky.keyIntensity / 3, 0, 1)));
            setSrgb(key.color, sky.keyColor);
            key.intensity = sky.keyIntensity * (1 + (LIGHT.nightKey - 1) * sky.night);
            keyGoal.fromArray(sky.keyDir).normalize();
            if (snap) keyCurrent.copy(keyGoal);
            fogSrgb = sky.fogColor;
            applyFog();
            setSrgb(skyUniforms.uTop.value, sky.skyTop);
            setSrgb(skyUniforms.uHorizon.value, sky.skyHorizon);
            setSrgb(skyUniforms.uBottom.value, sky.skyBottom);
            setSrgb(skyUniforms.uKey.value, sky.keyColor);
            skyUniforms.uSunDir.value.fromArray(sky.sunDir);
            skyUniforms.uMoonDir.value.fromArray(sky.moonDir);
            skyUniforms.uSunVisible.value = sky.sunVisible;
            skyUniforms.uMoonVisible.value = sky.moonVisible;
            skyUniforms.uStars.value = sky.stars;
            uniforms.uNight.value = sky.night;
            uniforms.uSun.value = sky.sunVisible;
        }

        function updateCamera(dt, t) {
            const aspect = width / height;
            if (motion) {
                const blend = 1 - Math.exp(-dt / PARALLAX.tau);
                parallax.yaw += (parallax.goalYaw - parallax.yaw) * blend;
                parallax.pitch += (parallax.goalPitch - parallax.pitch) * blend;
            }
            const rad = Math.PI / 180;
            let yaw = BASE_YAW + parallax.yaw;
            let pitch = BASE_PITCH + parallax.pitch;
            if (motion) {
                yaw += BREATH.yaw * Math.sin(2 * Math.PI * t / BREATH.yawPeriod);
                pitch += BREATH.pitch * Math.sin(2 * Math.PI * t / BREATH.pitchPeriod);
            }
            cameraDistance = FIT_HALF_HEIGHT / Math.tan(FOV / 2 * rad);
            if (aspect < NARROW_ASPECT) cameraDistance /= aspect / NARROW_ASPECT;
            const cp = Math.cos(pitch * rad);
            camera.position.set(
                TARGET[0] + cameraDistance * Math.sin(yaw * rad) * cp,
                TARGET[1] + cameraDistance * Math.sin(pitch * rad),
                TARGET[2] + cameraDistance * Math.cos(yaw * rad) * cp
            );
            camera.lookAt(TARGET[0], TARGET[1], TARGET[2]);
            camera.updateMatrixWorld();
            skyMesh.position.copy(camera.position);
            scene.fog.near = cameraDistance * FOG.near;
            scene.fog.far = cameraDistance * FOG.far;
        }

        function applyViewOffset() {
            if (width / height >= VIEW_SHIFT.minAspect) camera.setViewOffset(width, height, -width * VIEW_SHIFT.x, height * VIEW_SHIFT.y, width, height);
            else camera.clearViewOffset();
        }

        function projectAnchors() {
            const out = {};
            const v = new THREE.Vector3();
            for (const name of Object.keys(anchorPoints)) {
                v.copy(anchorPoints[name]).project(camera);
                out[name] = [Math.round((v.x * .5 + .5) * width), Math.round((-v.y * .5 + .5) * height)];
            }
            anchors = out;
        }

        // ---- Quality, size -------------------------------------------------------------------
        function ensureTarget(w, h) {
            if (target && target.width === w && target.height === h) return;
            if (target) target.dispose();
            target = new THREE.WebGLRenderTarget(w, h, { samples: 4, type: THREE.HalfFloatType, depthBuffer: true, stencilBuffer: false });
            postUniforms.tDiffuse.value = target.texture;
        }

        function resize() {
            if (!renderer || !camera) return;
            const s = state();
            width = Math.max(1, Math.round(Number(s.width) || window.innerWidth || 1));
            height = Math.max(1, Math.round(Number(s.height) || window.innerHeight || 1));
            const spec = QUALITY[quality];
            dpr = Math.min(window.devicePixelRatio || 1, spec.dpr);
            renderer.setPixelRatio(dpr);
            renderer.setSize(width, height, false);
            camera.aspect = width / height;
            applyViewOffset();
            camera.updateProjectionMatrix();
            if (tiltOn) {
                const w = Math.max(1, Math.round(width * dpr));
                const h = Math.max(1, Math.round(height * dpr));
                ensureTarget(w, h);
                postUniforms.uPx.value.set(1 / w, 1 / h);
                postUniforms.uMaxBlur.value = TILT.maxBlur * dpr;
            }
            if (built && (!raf && !contextLost)) renderStatic();
        }

        function applyQuality(level) {
            const next = clamp(Math.round(level), 0, QUALITY.length - 1);
            const spec = QUALITY[next];
            const changed = next !== quality;
            quality = next;
            const wantShadow = spec.shadow > 0;
            const wantTilt = spec.tilt && tiltCapable;
            const shadowToggled = wantShadow !== shadowOn;
            const tiltToggled = wantTilt !== tiltOn;
            shadowOn = wantShadow;
            tiltOn = wantTilt;
            renderer.shadowMap.enabled = wantShadow;
            key.castShadow = wantShadow;
            if (wantShadow && spec.shadow !== shadowSize) {
                shadowSize = spec.shadow;
                key.shadow.mapSize.set(shadowSize, shadowSize);
                if (key.shadow.map) { key.shadow.map.dispose(); key.shadow.map = null; }
            }
            shadowDirty = true;
            if (shadowToggled) for (const name of Object.keys(materials)) materials[name].needsUpdate = true;
            if (!tiltOn && target) { target.dispose(); target = null; postUniforms.tDiffuse.value = null; }
            if (tiltToggled) applyFog();
            if (spec.hidden) hideHost();
            else if (host.style.display === 'none' && backend === 'three-webgl') showHost();
            if (life && typeof life.setQuality === 'function') {
                try { life.setQuality(spec.life); } catch (error) { noteError('scene life', error); }
            }
            warm = ADAPT.warmup;
            costs.length = 0;
            slowGaps = 0;
            fastSince = 0;
            if (changed || !built) resize();
            if (changed) startLoops();
        }

        function setQualityLevel(level, automatic) {
            if (automatic && qualityLock !== null) return;
            applyQuality(level);
        }

        function adapt(cost, gap, interval, now) {
            if (warm > 0) { warm--; return; }
            costs.push(cost);
            if (costs.length > ADAPT.window) costs.shift();
            slowGaps = gap > interval * ADAPT.gapFactor ? slowGaps + 1 : Math.max(0, slowGaps - 1);
            if (qualityLock !== null || throttled) { fastSince = 0; return; }
            const mean = costs.reduce((a, b) => a + b, 0) / costs.length;
            if (quality < QUALITY.length - 1 && ((costs.length >= ADAPT.window && mean > ADAPT.downCost) || slowGaps >= ADAPT.gapFrames)) {
                applyQuality(quality + 1);
            } else if (quality > initialQuality && costs.length >= ADAPT.window && mean < ADAPT.upCost) {
                if (!fastSince) fastSince = now;
                else if (now - fastSince >= ADAPT.upAfterMs) applyQuality(quality - 1);
            } else fastSince = 0;
        }

        // ---- Frames --------------------------------------------------------------------------
        function drawFrame(dt, now) {
            if (!renderer || disposed || contextLost || !built) return;
            const begin = performance.now();
            const t = frozen !== null ? frozen : lifeTime;
            uniforms.uTime.value = t;
            if (!sky || hourOverride === null && now - lastSkyAt > 1000) updateSky(!sky, now);
            if (keyCurrent.distanceToSquared(keyGoal) > 1e-10) {
                keyCurrent.lerp(keyGoal, 1 - Math.exp(-Math.max(dt, .001) / KEY_TAU)).normalize();
            }
            key.position.set(TARGET[0] + keyCurrent.x * SHADOW.distance, TARGET[1] + keyCurrent.y * SHADOW.distance, TARGET[2] + keyCurrent.z * SHADOW.distance);
            updateCamera(dt, t);
            if (life && !lifeFailed) {
                try { life.update(dt, t, sky, motion); } catch (error) { lifeFailed = true; noteError('scene life', error); }
            }
            if (shadowOn && (shadowDirty || keyCurrent.dot(shadowDir) < .99999 || (motion && now - lastShadowAt > SHADOW.refreshMs))) {
                renderer.shadowMap.needsUpdate = true;
                shadowDir.copy(keyCurrent);
                lastShadowAt = now;
                shadowDirty = false;
            }
            try {
                if (tiltOn) {
                    renderer.setRenderTarget(target);
                    renderer.render(scene, camera);
                    renderer.setRenderTarget(null);
                    renderer.render(postScene, postCamera);
                } else {
                    renderer.render(scene, camera);
                }
            } catch (error) {
                renderer.setRenderTarget(null);
                backend = 'error';
                stopLoops();
                noteError('scene render', error);
                return;
            }
            projectAnchors();
            frames++;
            lastCost = performance.now() - begin;
            stamps.push(now);
            while (stamps.length && now - stamps[0] > 2000) stamps.shift();
            if (!ready) {
                ready = true;
                if (!QUALITY[quality].hidden) host.style.opacity = '1';
            }
        }

        function renderStatic() {
            if (!built || disposed || paused) return;
            drawFrame(0, performance.now());
        }

        function tick(now) {
            raf = 0;
            if (disposed || paused || !renderer || contextLost) return;
            raf = requestAnimationFrame(tick);
            const s = state();
            throttled = (Number(s.coverage) || 0) >= THROTTLE.coverage;
            const interval = throttled ? THROTTLE.interval : QUALITY[quality].interval;
            if (lastRender && now - lastRender < interval - FRAME_SLACK) return;
            const gap = lastRender ? now - lastRender : interval;
            const dt = lastRender ? Math.min(gap / 1000, .1) : 1 / 30;
            lastRender = now;
            if (frozen === null) lifeTime += dt;
            drawFrame(dt, now);
            adapt(lastCost, gap, interval, now);
        }

        function stopLoops() {
            if (raf) cancelAnimationFrame(raf);
            raf = 0;
            if (staticTimer) clearInterval(staticTimer);
            staticTimer = 0;
        }

        function startLoops() {
            stopLoops();
            if (!renderer || !built || disposed || paused || contextLost || backend === 'error') return;
            const spec = QUALITY[quality];
            if (spec.hidden) return;
            lastRender = 0;
            if (!motion || !spec.interval) {
                renderStatic();
                staticTimer = setInterval(renderStatic, STATIC_REFRESH_MS);
            } else {
                raf = requestAnimationFrame(tick);
            }
        }

        // ---- Build ---------------------------------------------------------------------------
        function build() {
            buildTimer = 0;
            if (disposed) return;
            const started = performance.now();
            try {
                if (!createRenderer()) return;
                scene = new THREE.Scene();
                scene.fog = new THREE.Fog(0xffffff, 100, 500);
                camera = new THREE.PerspectiveCamera(FOV, 1, 1, 1200);
                buildSky();
                buildLighting();
                buildWorld();
                buildPost();
                const s = state();
                const w = Number(s.width) || window.innerWidth || 1;
                const h = Number(s.height) || window.innerHeight || 1;
                const ratio = Math.min(window.devicePixelRatio || 1, QUALITY[0].dpr);
                initialQuality = w * h * ratio * ratio > INITIAL_PIXELS ? 1 : 0;
                quality = -1;
                applyQuality(initialQuality);
                buildLife();
            } catch (error) {
                backend = 'error';
                hideHost();
                noteError('scene build', error);
                releaseRenderer();
                return;
            }
            buildMs = Math.round(performance.now() - started);
            built = true;
            resize();
            startLoops();
        }

        buildTimer = setTimeout(build, 0);

        // ---- Public surface ------------------------------------------------------------------
        function setPointer(nx, ny, inside) {
            if (!motion) return;
            parallax.goalYaw = inside ? PARALLAX.yaw * clamp(nx, -1, 1) : 0;
            parallax.goalPitch = inside ? PARALLAX.pitch * clamp(ny, -1, 1) : 0;
        }

        function setHourOverride(hour) {
            hourOverride = Number.isFinite(Number(hour)) && hour !== null ? ((Number(hour) % 24) + 24) % 24 : null;
            if (!built) return;
            updateSky(true, performance.now());
            shadowDirty = true;
            if (!raf) renderStatic();
        }

        function setFreeze(seconds) {
            frozen = Number.isFinite(Number(seconds)) && seconds !== null ? Number(seconds) : null;
            if (built && !raf) renderStatic();
        }

        function lockQuality(level) {
            qualityLock = Number.isFinite(Number(level)) && level !== null ? clamp(Math.round(Number(level)), 0, QUALITY.length - 1) : null;
            if (qualityLock !== null && built) setQualityLevel(qualityLock, false);
        }

        function pause() {
            paused = true;
            stopLoops();
        }

        function resume() {
            if (!paused) return;
            paused = false;
            startLoops();
        }

        function inspect() {
            const now = performance.now();
            while (stamps.length && now - stamps[0] > 2000) stamps.shift();
            let lifeState = null;
            if (life && typeof life.inspect === 'function') {
                try { lifeState = life.inspect(); } catch (_) { lifeState = null; }
            }
            return {
                ready, backend, gpu, quality, qualityLocked: qualityLock !== null, fps: Math.round(stamps.length / 2 * 10) / 10, frames, buildMs,
                hour: sky ? Math.round(sky.hour * 100) / 100 : null, phase: sky ? sky.phase : '', night: sky ? Math.round(sky.night * 100) / 100 : 0,
                triangles: Math.round(triangles), voxels, shadows: shadowOn, tiltShift: tiltOn, throttled,
                lastCostMs: Math.round(lastCost * 10) / 10, anchors, life: lifeState
            };
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            if (buildTimer) clearTimeout(buildTimer);
            buildTimer = 0;
            stopLoops();
            if (life && typeof life.dispose === 'function') { try { life.dispose(); } catch (_) { /* best effort */ } }
            life = null;
            if (target) target.dispose();
            target = null;
            for (const item of disposables) { try { item.dispose(); } catch (_) { /* best effort */ } }
            if (key && key.shadow && key.shadow.map) key.shadow.map.dispose();
            releaseRenderer();
            host.remove();
        }

        return {
            get ready() { return ready; },
            get voxelCount() { return voxels; },
            setPointer, setHourOverride, setFreeze, setQuality: lockQuality, pause, resume, resize,
            renderNow: renderStatic, dispose, inspect
        };
    }

    window.AuraVoxelScene = Object.freeze({ create });
})();
