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
    // the sky light gains fill as the key light weakens, and moonlight is stronger. dayKey and dayHemi scale the key and the
    // sky/fill light by day (more contrast; they fade out towards night). rim is a second directional light without shadows from
    // behind the island (relative to the camera: away from it, a little to the right, above) that outlines the silhouette.
    const LIGHT = {
        groundBounce: .7, groundGain: 2.2, fill: 2, nightKey: 3.2, nightHemi: 1.4, dayKey: 1.15, dayHemi: .85,
        rim: { day: .35, night: .55, nightColor: '#8fa8ff', side: .45, up: .55 }
    };
    const KEY_TAU = 1.2;                  // seconds the key light needs to swing to a new direction
    // The fog colour and the cloud sea's haze (uniforms.uHaze) are the sky gradient at this screen row (share of the height from the
    // top), where the sea meets the sky: the far edge of the sea disappears into the sky behind it.
    const FOG = { near: .95, far: 2.3, skyRow: .5 };   // near and far in camera distances
    // The tilt-shift target is drawn at min(dpr, 1) with 2 MSAA samples (a 4x half-float target costs about 56 bytes per pixel) and
    // only when the picture is small enough: width x height x min(dpr, 1)^2 <= maxPixels.
    const TILT = { focusY: .47, band: .16, feather: .28, maxBlur: 3.2, vignette: .16, saturation: 1.08, samples: 2, maxPixels: 2.4e6 };
    // Screen-space sky. Positions are fractions of the screen (x from the left, y from the top), sizes fractions of its height.
    // The sun crosses from x .88 to x .12 between the rise and set hours; the moon from x .85 to x .15 between set and rise.
    // halos are [size multiple, alpha] of stepped square halos around the sun; craters are [x, y, half size] in moon sizes; the moon's
    // halo is a round glow of halo / 2 moon sizes in radius, added to the sky with strength haloAlpha.
    const SKY = {
        horizonAt: .62,
        sun: { size: .06, x: .88, dx: .76, y: .46, dy: .36, rise: 6.5, set: 19.5, gain: 2.2, tint: [1, .94, .72], halos: [[2.4, .15], [1.6, .35]], glow: .22, glowReach: 3.2 },
        // moon: a pixel-art disc `cells` blocks across minus a disc of `cutRadius` blocks offset by `cut` (to the upper right), with
        // one-block craters at the listed block centres.
        moon: {
            size: .044, x: .85, dx: .7, y: .42, dy: .3, color: '#e8ecff', gain: 1.8, cells: 10, cut: [2.3, -1.5], cutRadius: 4,
            craters: [[-3.5, .5], [-2.5, 3.5], [-4.5, -1.5]], crater: .22, halo: 4.2, haloAlpha: .09, haloColor: '#8fa8ff', haloGain: 1.4
        },
        stars: { cell: 12, small: .04, bright: .006, fadeFrom: .35, fadeTo: .9, gain: 1.5, parallax: 6 }   // cell in px at 1080 high; parallax in px per degree of camera yaw
    };
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
    // The quality ladder follows the achieved cadence. Steps down: the 30-frame mean of the gaps between rendered frames exceeds
    // downGap x the expected gap, or a single gap exceeds stallMs, or the mean CPU cost of renderer.render() exceeds downCost.
    // Steps up (never above the initial level, never while locked): mean gap under upGap x the expected gap and mean cost under
    // upCost for upAfterMs. The expected gap is the frame interval rounded up to whole display refreshes. Gaps across pause/resume,
    // hidden pages, throttling and the first frame after a quality change are ignored.
    const ADAPT = { window: 30, downCost: 12, upCost: 5, upAfterMs: 25000, downGap: 1.45, upGap: 1.1, stallMs: 250, warmup: 4 };
    // Q3 renders one timed frame (including GPU time) every PROBE_MS; upAfterMs worth of probes under upCost step up to Q2.
    const PROBE = { level: 3, everyMs: 5000 };
    const RESIZE_DEBOUNCE_MS = 150;       // static redraw after a resize, and the stable-size wait before the tilt-shift target is recreated
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

    function create(options) {
        const THREE = options.THREE;
        const motion = !!options.motion;
        const read = options.readState;
        const noteError = typeof options.noteError === 'function' ? options.noteError : function () {};
        const Island = window.AuraVoxelIsland;
        const Mesher = window.AuraVoxelMesher;
        const SkyModel = window.AuraVoxelSky;
        const Blocks = window.AuraVoxelBlocks;
        const Shaders = window.AuraVoxelShaders;

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
        let rim = null;
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
        let fogLinear = [.8, .9, 1];
        let resizeTimer = 0;                  // static mode: coalesced redraw after a resize
        let targetTimer = 0;                  // debounced re-creation of the tilt-shift target
        let skipGap = true;                   // the next rendered frame's gap is not a cadence sample
        let lastTick = 0;
        let refresh = 34;                     // display refresh period: the shortest gap between animation frames seen (ms)
        const stamps = [];
        const costs = [];
        const gaps = [];
        const probeCosts = [];
        let warm = ADAPT.warmup;
        let fastSince = 0;
        let parallax = { yaw: 0, pitch: 0, goalYaw: 0, goalPitch: 0 };
        const sunTint = setSrgb(new THREE.Color(), SKY.sun.tint);   // pushes the sun towards yellow (white-yellow at noon, deep orange at golden hour)
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
            uTexel: { value: TEXEL },
            uSkyHorizon: { value: new THREE.Color() },   // the water mixes towards it
            uHaze: { value: new THREE.Color() }          // the sky colour at the horizon row (the cloud sea blends into it)
        };
        const skyUniforms = {
            uTop: { value: new THREE.Color() }, uHorizon: { value: new THREE.Color() }, uBottom: { value: new THREE.Color() },
            uKey: { value: new THREE.Color() }, uSunColor: { value: new THREE.Color() },
            uSunPos: { value: new THREE.Vector2(.5, .3) }, uMoonPos: { value: new THREE.Vector2(.5, .3) }, uResolution: { value: new THREE.Vector2(1, 1) },
            uSunVisible: { value: 1 }, uMoonVisible: { value: 0 }, uStars: { value: 0 }, uTime: uniforms.uTime, uYaw: { value: 0 }
        };
        const postUniforms = {
            tDiffuse: { value: null }, uPx: { value: new THREE.Vector2(1, 1) }, uMaxBlur: { value: TILT.maxBlur },
            uBand: { value: TILT.band }, uFeather: { value: TILT.feather }, uFocusY: { value: TILT.focusY }, uVignette: { value: TILT.vignette },
            uSaturation: { value: TILT.saturation }
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

        // Flowers and tufts are taller than wide; a decoration with its own height (lanterns) keeps it.
        function addDecorations(parent, list, material) {
            if (!list.length) return;
            const geometry = track(new THREE.BoxGeometry(1, 1, 1));
            const mesh = new THREE.InstancedMesh(geometry, material, list.length);
            const matrix = new THREE.Matrix4();
            const position = new THREE.Vector3();
            const scale = new THREE.Vector3();
            const quaternion = new THREE.Quaternion();
            const color = new THREE.Color();
            list.forEach((item, i) => {
                const tall = item.height || item.size * DECO_TALL;
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
            materials.glowDeco = patch(new THREE.MeshLambertMaterial({ color: 0xffffff }), 'glowdeco');
            island = Island.generate();
            const world = new THREE.Group();
            scene.add(world);
            addGrid(island, world);
            addDecorations(world, island.decorations.filter(item => !item.glow), materials.deco);
            addDecorations(world, island.decorations.filter(item => item.glow), materials.glowDeco);
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
            rim = new THREE.DirectionalLight(0xffffff, 0);
            rim.castShadow = false;
            rim.target.position.set(TARGET[0], TARGET[1], TARGET[2]);
            scene.add(hemi, key, key.target, rim, rim.target);
        }

        function buildSky() {
            const material = track(new THREE.ShaderMaterial({
                uniforms: skyUniforms, vertexShader: Shaders.SKY_VERTEX, fragmentShader: Shaders.skyFragment(SKY),
                side: THREE.DoubleSide, depthWrite: false, depthTest: false, fog: false
            }));
            skyMesh = new THREE.Mesh(track(new THREE.PlaneGeometry(2, 2)), material);
            skyMesh.frustumCulled = false;
            skyMesh.renderOrder = -1000;
            scene.add(skyMesh);
        }

        function buildPost() {
            postMaterial = track(new THREE.ShaderMaterial({
                uniforms: postUniforms, vertexShader: Shaders.POST_VERTEX, fragmentShader: Shaders.POST_FRAGMENT,
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
            // Drawn straight to the canvas the fog is mixed in after tone mapping; the tilt-shift pass tone-maps it later.
            const rgb = tiltOn ? fogLinear : acesFilmic(fogLinear);
            scene.fog.color.setRGB(rgb[0], rgb[1], rgb[2], THREE.LinearSRGBColorSpace);
        }

        // The sky gradient (linear colours, the same smoothsteps as the shader) at a screen row from the top (0..1).
        function skyGradientAt(row) {
            const smooth = (a, b, x) => { const t = clamp((x - a) / (b - a), 0, 1); return t * t * (3 - 2 * t); };
            const toHorizon = smooth(0, SKY.horizonAt, row);
            const toBottom = smooth(SKY.horizonAt, 1, row);
            const top = skyUniforms.uTop.value;
            const horizon = skyUniforms.uHorizon.value;
            const bottom = skyUniforms.uBottom.value;
            return ['r', 'g', 'b'].map(c => (top[c] + (horizon[c] - top[c]) * toHorizon) * (1 - toBottom) + bottom[c] * toBottom);
        }

        // Sun and moon paths in screen fractions: the sun from the rise hour to the set hour, the moon from the set hour to the rise hour.
        function updateSkyPaths(hour) {
            const sun = SKY.sun;
            const moon = SKY.moon;
            const dayT = clamp((hour - sun.rise) / (sun.set - sun.rise), 0, 1);
            const nightT = clamp(((hour - sun.set + 24) % 24) / (24 - (sun.set - sun.rise)), 0, 1);
            skyUniforms.uSunPos.value.set(sun.x - sun.dx * dayT, sun.y - sun.dy * Math.sin(Math.PI * dayT));
            skyUniforms.uMoonPos.value.set(moon.x - moon.dx * nightT, moon.y - moon.dy * Math.sin(Math.PI * nightT));
        }

        function updateSky(snap, now) {
            const hour = hourOverride !== null ? hourOverride : SkyModel.hourFromDate();
            sky = SkyModel.at(hour);
            lastSkyAt = now;
            const day = 1 - sky.night;
            setSrgb(hemi.color, sky.hemiSky);
            setSrgb(hemi.groundColor, sky.hemiGround.map((c, i) => c + (sky.cloudTint[i] - c) * LIGHT.groundBounce)).multiplyScalar(LIGHT.groundGain);
            hemi.intensity = sky.hemiIntensity * (1 + LIGHT.fill * (1 - clamp(sky.keyIntensity / 3, 0, 1))) * (1 + (LIGHT.dayHemi - 1) * day) * (1 + (LIGHT.nightHemi - 1) * sky.night);
            setSrgb(key.color, sky.keyColor);
            key.intensity = sky.keyIntensity * (1 + (LIGHT.nightKey - 1) * sky.night) * (1 + (LIGHT.dayKey - 1) * day);
            const rimNight = hexRGB(LIGHT.rim.nightColor);
            setSrgb(rim.color, sky.skyHorizon.map((c, i) => c + (rimNight[i] - c) * sky.night));
            rim.intensity = LIGHT.rim.day + (LIGHT.rim.night - LIGHT.rim.day) * sky.night;
            keyGoal.fromArray(sky.keyDir).normalize();
            if (snap) keyCurrent.copy(keyGoal);
            setSrgb(skyUniforms.uTop.value, sky.skyTop);
            setSrgb(skyUniforms.uHorizon.value, sky.skyHorizon);
            setSrgb(skyUniforms.uBottom.value, sky.skyBottom);
            setSrgb(skyUniforms.uKey.value, sky.keyColor);
            setSrgb(skyUniforms.uSunColor.value, sky.keyColor);
            skyUniforms.uSunColor.value.multiply(sunTint).multiplyScalar(SKY.sun.gain);
            setSrgb(uniforms.uSkyHorizon.value, sky.skyHorizon);
            updateSkyPaths(sky.hour);
            fogLinear = skyGradientAt(FOG.skyRow);
            uniforms.uHaze.value.setRGB(fogLinear[0], fogLinear[1], fogLinear[2], THREE.LinearSRGBColorSpace);
            applyFog();
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
            scene.fog.near = cameraDistance * FOG.near;
            scene.fog.far = cameraDistance * FOG.far;
            skyUniforms.uYaw.value = yaw - BASE_YAW;
            // Rim light: from behind the island (away from the camera), a little to the right of the view, and above.
            const sy = Math.sin(yaw * rad);
            const cy = Math.cos(yaw * rad);
            const rx = -sy + cy * LIGHT.rim.side;
            const rz = -cy - sy * LIGHT.rim.side;
            const rl = Math.hypot(rx, LIGHT.rim.up, rz) || 1;
            rim.position.set(TARGET[0] + rx / rl * SHADOW.distance, TARGET[1] + LIGHT.rim.up / rl * SHADOW.distance, TARGET[2] + rz / rl * SHADOW.distance);
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
        function readSize() {
            const s = state();
            width = Math.max(1, Math.round(Number(s.width) || window.innerWidth || 1));
            height = Math.max(1, Math.round(Number(s.height) || window.innerHeight || 1));
            dpr = Math.min(window.devicePixelRatio || 1, QUALITY[quality].dpr);
        }

        // The tilt-shift target is drawn at min(dpr, 1); the post pass scales it to the canvas.
        function targetSize() {
            const r = Math.min(dpr, 1);
            return [Math.max(1, Math.round(width * r)), Math.max(1, Math.round(height * r))];
        }

        function tiltWanted() {
            const r = Math.min(dpr, 1);
            return !!QUALITY[quality].tilt && tiltCapable && width * height * r * r <= TILT.maxPixels;
        }

        function ensureTarget(w, h) {
            if (!target || target.width !== w || target.height !== h) {
                if (target) target.dispose();
                target = new THREE.WebGLRenderTarget(w, h, { samples: TILT.samples, type: THREE.HalfFloatType, depthBuffer: true, stencilBuffer: false });
                postUniforms.tDiffuse.value = target.texture;
            }
            postUniforms.uPx.value.set(1 / w, 1 / h);
            postUniforms.uMaxBlur.value = TILT.maxBlur * w / Math.max(1, width);
        }

        function clearTargetTimer() {
            if (targetTimer) clearTimeout(targetTimer);
            targetTimer = 0;
        }

        // Switches the tilt-shift pass on or off and keeps its target at the picture size. A changed size only re-creates the target
        // once it has been stable for RESIZE_DEBOUNCE_MS (the old target keeps being used, scaled by the post pass, meanwhile) unless
        // there is no target yet or `immediate` is set.
        function updateTilt(immediate) {
            const want = tiltWanted();
            if (want !== tiltOn) {
                tiltOn = want;
                applyFog();
                if (!want) {
                    clearTargetTimer();
                    if (target) { target.dispose(); target = null; postUniforms.tDiffuse.value = null; }
                    return;
                }
            }
            if (!tiltOn) return;
            const size = targetSize();
            if (target && target.width === size[0] && target.height === size[1]) { clearTargetTimer(); return; }
            if (!target || immediate) { clearTargetTimer(); ensureTarget(size[0], size[1]); return; }
            clearTargetTimer();
            targetTimer = setTimeout(() => {
                targetTimer = 0;
                if (!tiltOn || disposed || !renderer) return;
                const next = targetSize();
                ensureTarget(next[0], next[1]);
            }, RESIZE_DEBOUNCE_MS);
        }

        function applySize(immediate) {
            if (resizeTimer) { clearTimeout(resizeTimer); resizeTimer = 0; }
            if (!renderer || !camera || disposed) return;
            readSize();
            renderer.setPixelRatio(dpr);
            renderer.setSize(width, height, false);
            camera.aspect = width / height;
            applyViewOffset();
            camera.updateProjectionMatrix();
            updateTilt(immediate);
            if (!built || contextLost) return;
            if (raf) { lastRender = 0; skipGap = true; }   // draw on the next frame: resizing cleared the canvas
            else renderStatic();
        }

        // Without a frame loop (reduced motion, Q3) a resize is redrawn once, 150 ms after the last resize event.
        function resize() {
            if (!renderer || !camera || disposed) return;
            if (built && !raf) {
                if (resizeTimer) clearTimeout(resizeTimer);
                resizeTimer = setTimeout(() => applySize(true), RESIZE_DEBOUNCE_MS);
                return;
            }
            applySize(false);
        }

        function applyQuality(level) {
            const next = clamp(Math.round(level), 0, QUALITY.length - 1);
            const spec = QUALITY[next];
            const changed = next !== quality;
            quality = next;
            const wantShadow = spec.shadow > 0;
            const shadowToggled = wantShadow !== shadowOn;
            shadowOn = wantShadow;
            renderer.shadowMap.enabled = wantShadow;
            key.castShadow = wantShadow;
            if (!wantShadow) {
                // The shadow map is released while the shadows are off (2048 x 2048 depth is 16 MB or more).
                if (key.shadow.map) { key.shadow.map.dispose(); key.shadow.map = null; }
                shadowSize = 0;
            } else if (spec.shadow !== shadowSize) {
                shadowSize = spec.shadow;
                key.shadow.mapSize.set(shadowSize, shadowSize);
                if (key.shadow.map) { key.shadow.map.dispose(); key.shadow.map = null; }
            }
            shadowDirty = true;
            if (shadowToggled) for (const name of Object.keys(materials)) materials[name].needsUpdate = true;
            if (spec.hidden) hideHost();
            else if (host.style.display === 'none' && backend === 'three-webgl') showHost();
            if (life && typeof life.setQuality === 'function') {
                try { life.setQuality(spec.life); } catch (error) { noteError('scene life', error); }
            }
            warm = ADAPT.warmup;
            costs.length = 0;
            gaps.length = 0;
            probeCosts.length = 0;
            fastSince = 0;
            skipGap = true;
            if (changed || !built) applySize(true);
            else updateTilt(true);
            if (changed) startLoops();
        }

        function setQualityLevel(level, automatic) {
            if (automatic && qualityLock !== null) return;
            applyQuality(level);
        }

        // ---- Adaptive quality ----------------------------------------------------------------
        function mean(list) { return list.reduce((a, b) => a + b, 0) / list.length; }

        // The display refresh period is the shortest gap between animation frames seen (a slow scene cannot raise it).
        function trackRefresh(now) {
            if (lastTick) {
                const delta = now - lastTick;
                if (delta >= 4 && delta < refresh) refresh = delta;
            }
            lastTick = now;
        }

        // The gap between rendered frames the display can deliver for an interval: whole refresh periods.
        function expectedGap(interval) {
            return Math.max(interval, Math.ceil((interval - FRAME_SLACK) / refresh) * refresh);
        }

        // cost: CPU ms of the frame; gap: ms since the previous rendered frame, or null when it is not a cadence sample.
        function adapt(cost, gap, interval, now) {
            if (throttled) { fastSince = 0; return; }
            if (warm > 0) { warm--; return; }
            costs.push(cost);
            if (costs.length > ADAPT.window) costs.shift();
            if (gap !== null) {
                gaps.push(gap);
                if (gaps.length > ADAPT.window) gaps.shift();
            }
            if (qualityLock !== null) { fastSince = 0; return; }
            const expected = expectedGap(interval);
            const full = costs.length >= ADAPT.window && gaps.length >= ADAPT.window;
            const meanCost = mean(costs);
            const meanGap = gaps.length ? mean(gaps) : 0;
            const stalled = gap !== null && gap > ADAPT.stallMs;
            if (quality < QUALITY.length - 1 && (stalled || (full && (meanCost > ADAPT.downCost || meanGap > expected * ADAPT.downGap)))) {
                applyQuality(quality + 1);
            } else if (quality > initialQuality && full && meanCost < ADAPT.upCost && meanGap < expected * ADAPT.upGap) {
                if (!fastSince) fastSince = now;
                else if (now - fastSince >= ADAPT.upAfterMs) applyQuality(quality - 1);
            } else fastSince = 0;
        }

        // Q3 has no frame loop: one timed render (CPU and GPU, gl.finish) every PROBE.everyMs; probes worth ADAPT.upAfterMs under
        // ADAPT.upCost step back up to Q2.
        function probeTick() {
            if (!built || disposed || paused || contextLost || !renderer || quality !== PROBE.level) return;
            const begin = performance.now();
            if (frozen === null) lifeTime += PROBE.everyMs / 1000;
            drawFrame(0, begin);
            try { renderer.getContext().finish(); } catch (_) { /* the probe then measures the CPU only */ }
            lastCost = performance.now() - begin;
            probeCosts.push(lastCost);
            const needed = Math.ceil(ADAPT.upAfterMs / PROBE.everyMs);
            while (probeCosts.length > needed) probeCosts.shift();
            if (qualityLock === null && quality > initialQuality && probeCosts.length >= needed && mean(probeCosts) < ADAPT.upCost) applyQuality(quality - 1);
        }

        function onVisibility() {
            // Frames stop while the page is hidden: the first gap after it says nothing about the cadence.
            skipGap = true;
            lastTick = 0;
            gaps.length = 0;
            fastSince = 0;
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
            // The sky works in pixels of the surface it is drawn to: the tilt-shift target or the canvas.
            if (tiltOn && target) skyUniforms.uResolution.value.set(target.width, target.height);
            else skyUniforms.uResolution.value.set(Math.round(width * dpr), Math.round(height * dpr));
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
                if (tiltOn && target) {
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
            trackRefresh(now);
            const s = state();
            const wasThrottled = throttled;
            throttled = (Number(s.coverage) || 0) >= THROTTLE.coverage;
            if (throttled !== wasThrottled) { skipGap = true; gaps.length = 0; fastSince = 0; }
            const interval = throttled ? THROTTLE.interval : QUALITY[quality].interval;
            if (lastRender && now - lastRender < interval - FRAME_SLACK) return;
            const sample = !!lastRender && !skipGap && !document.hidden;
            const gap = lastRender ? now - lastRender : interval;
            const dt = lastRender ? Math.min(gap / 1000, .1) : 1 / 30;
            skipGap = false;
            lastRender = now;
            if (frozen === null) lifeTime += dt;
            drawFrame(dt, now);
            adapt(lastCost, sample ? gap : null, interval, now);
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
            skipGap = true;
            gaps.length = 0;
            fastSince = 0;
            if (!motion) {
                renderStatic();
                staticTimer = setInterval(renderStatic, STATIC_REFRESH_MS);
            } else if (!spec.interval) {
                renderStatic();
                staticTimer = setInterval(probeTick, PROBE.everyMs);
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
            applySize(true);
            startLoops();
        }

        buildTimer = setTimeout(build, 0);
        document.addEventListener('visibilitychange', onVisibility);

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
                lastCostMs: Math.round(lastCost * 10) / 10, gapMs: gaps.length ? Math.round(mean(gaps) * 10) / 10 : 0, refreshMs: Math.round(refresh * 10) / 10,
                anchors, life: lifeState
            };
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            if (buildTimer) clearTimeout(buildTimer);
            buildTimer = 0;
            if (resizeTimer) clearTimeout(resizeTimer);
            resizeTimer = 0;
            clearTargetTimer();
            document.removeEventListener('visibilitychange', onVisibility);
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
