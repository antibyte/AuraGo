(function () {
    'use strict';

    // Orbit scene (global AuraOrbitScene). The live WebGL space scene on its own canvas behind the shell (z-index -1): a
    // gas giant with banded clouds, storms, night lights, lightning, atmosphere and auroras, a shadowed ring system with
    // dust, a moon, a nebula sky, three star layers, the sun with a lens flare, comets and shooting stars, the Blender
    // models (orbit-models.js), and a post chain (bloom, grade with chromatic aberration, grain, meteors and the scanner
    // pulse, tone mapping). The surface of the giant and the nebula are baked once into textures; frames only sample them.
    // The quality ladder, the cadence and the pause/dispose rules mirror the Voxel World scene (voxel-scene.js).
    //
    // World: the planet sits at the origin with radius R = 100. The camera looks at it from a fixed direction at a
    // distance that is solved from the screen composition (choreo.composition); a lens shift moves the planet to its
    // place on screen. The ring plane, the planet axis and the sun are fixed in the frame of that neutral camera.
    // window.AuraOrbitScene = { create }.

    const R = 100;
    const FOV = 36;
    const CAMERA = { azimuth: 28, elevation: 12 };                 // degrees, the neutral camera direction
    const RING = { inner: 1.35, outer: 2.35, elevation: 23, roll: -13 };   // the ring plane is seen from 23 degrees above, major axis rising to the left
    const LIGHT = { screenAngle: 150, phase: 74, color: [1.14, 1.06, 0.95] };   // sun direction on screen and phase angle
    const SHELL = 1.045;
    const SEED = 11;
    const STATIC_TIME = 44;               // a moment of the cycle with the shuttle out and no comet, shooting star or flash
    const START_TIME = 34;
    const SKY_EXTENT = [1.75, 0.96];                               // half extents in rad of the baked nebula plate
    const BAKE_SIZE = [2048, 1024];
    // The vortices of the cloud decks as longitude offsets (in turns) from the meridian that faces the camera at STATIC_TIME. The big
    // storm and the oval face the camera there; the two spots sit about 110 degrees to either side, so a vortex is never far from the middle of the disc.
    const STORM = { lat: 0.20, offsetU: 0.045, ovalLat: 0.70, ovalOffsetU: -0.03, spotLat: -0.32, spotOffsetU: 0.30, spot2Lat: 0.10, spot2OffsetU: -0.30 };
    const MOON = { radius: 9, orbit: 260, period: 300, inclination: 6, phase: 1.476 };
    const SUN_DISTANCE = 2500;
    const POINTER_TAU = 0.35;
    const OPACITY_FADE_MS = 900;
    const COMET_SLOTS = 2;
    const COMET_POINTS = 64;
    const PULSE_SECONDS = 1.4;

    // dpr: pixel ratio cap; samples: MSAA of the HDR target; bloom: 1 full / 0.5 half / 0 off; steps: atmosphere samples; dust and
    // stars: point budgets; rocks: asteroid instances; interval: ms between frames (0 = one frame per minute).
    const QUALITY = [
        { dpr: 1.5, samples: 2, bloom: 1, steps: 12, dust: 6000, stars: 1, rocks: 140, detail: 1, interval: 33 },
        { dpr: 1.25, samples: 0, bloom: 0.5, steps: 8, dust: 4000, stars: 1, rocks: 90, detail: 1, interval: 33 },
        { dpr: 1, samples: 0, bloom: 0, steps: 6, dust: 2000, stars: 0.5, rocks: 50, detail: 0, interval: 42 },
        { dpr: 0.85, samples: 0, bloom: 0, steps: 6, dust: 2000, stars: 0.5, rocks: 50, detail: 0, interval: 0 },
        { dpr: 1, samples: 0, bloom: 0, steps: 6, dust: 2000, stars: 0.5, rocks: 50, detail: 0, interval: 0, hidden: true }
    ];
    const STAR_LAYERS = [{ count: 3500, size: [1.0, 1.9], bright: [0.25, 0.85], radius: 2700, shift: 5 },
        { count: 1500, size: [1.5, 2.6], bright: [0.5, 1.2], radius: 2680, shift: 12 },
        { count: 400, size: [2.2, 3.8], bright: [1.0, 2.6], radius: 2660, shift: 24 }];
    const DUST_MAX = 6000;
    const PIXEL_BUDGET = [3.7e6, 3.0e6, 2.2e6, 1.6e6, 1.6e6];
    const MSAA_MAX_PIXELS = PIXEL_BUDGET[0];
    const FRAME_SLACK = 3;
    const THROTTLE = { coverage: 0.96, interval: 500 };
    const STATIC_REFRESH_MS = 60000;
    const ADAPT = { window: 30, downCost: 12, upCost: 5, upAfterMs: 25000, downGap: 1.45, upGap: 1.1, stallMs: 250, warmup: 4 };
    const PROBE = { level: 3, everyMs: 5000 };
    const RESIZE_DEBOUNCE_MS = 150;
    const INITIAL_PIXELS = 4.2e6;
    const SOFTWARE_GPU = /swiftshader|llvmpipe|lavapipe|softpipe|software|basic render/i;
    const RAD = Math.PI / 180;

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
    function smoothstep(a, b, x) { const t = clamp((x - a) / (b - a), 0, 1); return t * t * (3 - 2 * t); }
    // The cloud decks turn as one at SPIN rad/s; on top of that each latitude swings by zonalVar() back and forth with a
    // period of SHEAR_PERIOD seconds, so the shear between latitudes stays bounded (about 7 degrees) and the planet looks
    // the same after hours. zonalVar() mirrors the shader. surfaceShift is the texture shift in turns.
    const SPIN = 0.0046;
    const SHEAR_PERIOD = 360;
    const TAU = Math.PI * 2;
    function zonalVar(lat) { return 0.0014 * Math.sin(lat * 5 + 0.4) + 0.0007 * Math.sin(lat * 13 + 1.9); }
    function shearPhase(t) { return SHEAR_PERIOD / (TAU * TAU) * Math.sin(TAU * t / SHEAR_PERIOD); }
    function spinTurns(t) { return (((SPIN * t / TAU) % 1) + 1) % 1; }
    function surfaceShift(lat, t) { return spinTurns(t) + zonalVar(lat) * shearPhase(t); }

    function create(options) {
        const vendor = options.vendor;
        const THREE = vendor.THREE;
        const choreo = options.choreo;
        const shaders = options.shaders;
        const motion = !!options.motion;
        const assetsBase = options.assetsBase || '/3d/desktop-theme-orbit/v1/';

        const host = document.createElement('div');
        host.className = 'vd-fx-orbit-scene';
        host.setAttribute('aria-hidden', 'true');
        host.style.cssText = 'position:fixed;inset:0;z-index:-1;pointer-events:none;overflow:hidden;opacity:0;' +
            (motion ? 'transition:opacity ' + OPACITY_FADE_MS + 'ms ease;' : '');
        const canvas = document.createElement('canvas');
        canvas.style.cssText = 'display:block;width:100%;height:100%;';
        host.appendChild(canvas);
        document.body.insertBefore(host, document.body.firstChild);

        let renderer = null, composer = null, scene = null, camera = null, refCam = null;
        let flareScene = null, flareCamera = null, gradePass = null, bloomPass = null, models = null;
        let disposed = false, built = false, ready = false, paused = false, contextLost = false;
        let backend = 'loading', gpu = '', lastError = '';
        let buildTimer = 0, raf = 0, staticTimer = 0, resizeTimer = 0;
        let quality = 0, initialQuality = 0, qualityLock = null;
        let width = 1, height = 1, dpr = 1, distance = 340;
        let frames = 0, buildMs = 0, triangles = 0, lastCost = 0, lastRender = 0, clock = START_TIME;
        let frozen = null, throttled = false, skipGap = true, lastTick = 0, refresh = 34;
        let coverage = 0, pulseAt = -1, resolveReady = null;
        let samplesNow = -1, bloomNow = -1;
        const stamps = [], costs = [], gaps = [], probeCosts = [];
        let warm = ADAPT.warmup, fastSince = 0;
        const pointer = { x: 0, y: 0, goalX: 0, goalY: 0 };
        const disposables = [];
        const timings = {};
        let buildStart = 0;
        function mark(name) { timings[name] = Math.round(performance.now() - buildStart); }
        const readyPromise = new Promise(resolve => { resolveReady = resolve; });

        const basis = { right: new THREE.Vector3(), up: new THREE.Vector3(), back: new THREE.Vector3() };
        const ring = { n: new THREE.Vector3(), e1: new THREE.Vector3(), e2: new THREE.Vector3(), quaternion: new THREE.Quaternion() };
        const U = {
            time: { value: 0 },
            noise: { value: null },
            sunDir: { value: new THREE.Vector3(0, 1, 0) },
            sunColor: { value: new THREE.Vector3(LIGHT.color[0], LIGHT.color[1], LIGHT.color[2]) },
            ringN: { value: new THREE.Vector3(0, 1, 0) },
            ringRange: { value: new THREE.Vector4(RING.inner * R, RING.outer * R, 0, 0) }
        };
        const objects = {};
        const bolts = [0, 1, 2, 3].map(() => new THREE.Vector4());
        const rig = new THREE.Quaternion();
        // the direction of the screen centre (the lens shift moves it away from the planet): comets cross around it
        const view = { forward: new THREE.Vector3(0, 0, -1), right: new THREE.Vector3(1, 0, 0), up: new THREE.Vector3(0, 1, 0) };
        const tmp = { a: new THREE.Vector3(), b: new THREE.Vector3(), c: new THREE.Vector3(), d: new THREE.Vector3() };

        function track(item) { disposables.push(item); return item; }
        function noteError(stage, error) {
            const message = error && error.message ? error.message : String(error || 'unknown failure');
            lastError = String(stage) + ': ' + message.slice(0, 240);
            if (window.console && typeof window.console.warn === 'function') console.warn('[orbit FX] ' + lastError);
            if (typeof options.noteError === 'function') { try { options.noteError(stage, error); } catch (_) { /* optional */ } }
        }
        function hideHost() { host.style.display = 'none'; }
        function showHost() { host.style.display = ''; }
        function finishReady() { if (resolveReady) { resolveReady(); resolveReady = null; } }

        // ---- Renderer ------------------------------------------------------------------------------
        function releaseRenderer() {
            if (!renderer) return;
            try { renderer.dispose(); renderer.forceContextLoss(); } catch (_) { /* the context may already be gone */ }
            renderer = null;
        }

        function createRenderer() {
            try {
                renderer = new THREE.WebGLRenderer({ canvas, antialias: false, alpha: false, powerPreference: 'high-performance', preserveDrawingBuffer: false, stencil: false });
            } catch (error) {
                renderer = null;
                backend = 'none';
                hideHost();
                noteError('WebGL initialization', error);
                return false;
            }
            try {
                const gl = renderer.getContext();
                const info = gl.getExtension('WEBGL_debug_renderer_info');
                gpu = String(gl.getParameter(info ? info.UNMASKED_RENDERER_WEBGL : gl.RENDERER) || '');
            } catch (_) { gpu = ''; }
            if (SOFTWARE_GPU.test(gpu) && window.__auraOrbitSoftwareGL !== true) {
                backend = 'software';
                hideHost();
                releaseRenderer();
                return false;
            }
            renderer.setClearColor(0x000000, 1);
            renderer.outputColorSpace = THREE.SRGBColorSpace;
            renderer.toneMapping = THREE.ACESFilmicToneMapping;
            renderer.toneMappingExposure = 1.05;
            renderer.info.autoReset = false;
            canvas.addEventListener('webglcontextlost', event => {
                event.preventDefault();
                if (disposed) return;
                contextLost = true;
                backend = 'lost';
                stopLoops();
                hideHost();
                noteError('WebGL', 'context lost');
            });
            canvas.addEventListener('webglcontextrestored', () => {
                if (disposed) return;
                try { bakeAll(); } catch (error) { noteError('re-bake', error); return; }
                contextLost = false;
                backend = 'three-webgl';
                if (!QUALITY[quality].hidden) showHost();
                startLoops();
            });
            backend = 'three-webgl';
            return true;
        }

        // ---- Frame of the neutral camera ----------------------------------------------------------
        function computeFrame() {
            const az = CAMERA.azimuth * RAD, el = CAMERA.elevation * RAD;
            basis.back.set(Math.sin(az) * Math.cos(el), Math.sin(el), Math.cos(az) * Math.cos(el)).normalize();
            basis.right.set(0, 1, 0).cross(basis.back).normalize();
            basis.up.crossVectors(basis.back, basis.right).normalize();
            const rEl = RING.elevation * RAD, rRoll = RING.roll * RAD;
            ring.n.set(0, 0, 0).addScaledVector(basis.right, -Math.sin(rRoll) * Math.cos(rEl)).addScaledVector(basis.up, Math.cos(rRoll) * Math.cos(rEl)).addScaledVector(basis.back, Math.sin(rEl)).normalize();
            ring.quaternion.setFromUnitVectors(new THREE.Vector3(0, 0, 1), ring.n);
            ring.e1.set(1, 0, 0).applyQuaternion(ring.quaternion);
            ring.e2.set(0, 1, 0).applyQuaternion(ring.quaternion);
            const phase = LIGHT.phase * RAD, sa = LIGHT.screenAngle * RAD;
            U.sunDir.value.set(0, 0, 0).addScaledVector(basis.right, Math.sin(phase) * Math.cos(sa)).addScaledVector(basis.up, Math.sin(phase) * Math.sin(sa)).addScaledVector(basis.back, Math.cos(phase)).normalize();
            U.ringN.value.copy(ring.n);
        }

        // ---- Bake: the surface of the giant and the nebula plate ----------------------------------
        let bakeScene = null, bakeCamera = null, bakeQuad = null;
        function bakeTexture(fragmentShader, uniforms, size, mipmaps, anisotropy) {
            const material = new THREE.ShaderMaterial({ uniforms, vertexShader: shaders.fullscreenVertex, fragmentShader, depthTest: false, depthWrite: false, blending: THREE.NoBlending });
            const target = new THREE.WebGLRenderTarget(size[0], size[1], {
                depthBuffer: false, stencilBuffer: false, generateMipmaps: mipmaps, type: THREE.UnsignedByteType, format: THREE.RGBAFormat,
                minFilter: mipmaps ? THREE.LinearMipmapLinearFilter : THREE.LinearFilter, magFilter: THREE.LinearFilter
            });
            target.texture.colorSpace = THREE.SRGBColorSpace;
            target.texture.anisotropy = anisotropy || 1;
            bakeQuad.material = material;
            renderer.setRenderTarget(target);
            renderer.render(bakeScene, bakeCamera);
            renderer.setRenderTarget(null);
            material.dispose();
            return target;
        }

        function stormColumn(lat, offset) {
            const inverse = objects.planet.quaternion.clone().invert();
            const dir = basis.back.clone().applyQuaternion(inverse);
            const phi = Math.atan2(dir.z, -dir.x);
            const face = (phi / (2 * Math.PI) + 1) % 1;
            return face + offset + surfaceShift(lat, STATIC_TIME);
        }

        function bakeAll() {
            if (!bakeScene) {
                bakeScene = new THREE.Scene();
                bakeCamera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1);
                bakeQuad = new THREE.Mesh(track(new THREE.PlaneGeometry(2, 2)), null);
                bakeQuad.frustumCulled = false;
                bakeScene.add(bakeQuad);
            }
            const aniso = Math.min(8, renderer.capabilities.getMaxAnisotropy());
            if (objects.planetPlate) objects.planetPlate.dispose();
            if (objects.skyPlate) objects.skyPlate.dispose();
            objects.planetPlate = bakeTexture(shaders.planetBakeFragment, {
                uNoise: U.noise,
                uStorm: { value: new THREE.Vector2(stormColumn(STORM.lat, STORM.offsetU), STORM.lat) },
                uOval: { value: new THREE.Vector2(stormColumn(STORM.ovalLat, STORM.ovalOffsetU), STORM.ovalLat) },
                uSpot: { value: new THREE.Vector2(stormColumn(STORM.spotLat, STORM.spotOffsetU), STORM.spotLat) },
                uSpot2: { value: new THREE.Vector2(stormColumn(STORM.spot2Lat, STORM.spot2OffsetU), STORM.spot2Lat) }
            }, BAKE_SIZE, true, aniso);
            objects.skyPlate = bakeTexture(shaders.skyBakeFragment, {
                uNoise: U.noise, uExtent: { value: new THREE.Vector2(SKY_EXTENT[0], SKY_EXTENT[1]) },
                uBand: { value: new THREE.Vector3(-0.45, 1.0, 0.10) }
            }, BAKE_SIZE, false, 1);
            if (objects.planetMaterial) objects.planetMaterial.uniforms.uAlbedo.value = objects.planetPlate.texture;
            if (objects.skyMaterial) objects.skyMaterial.uniforms.uPlate.value = objects.skyPlate.texture;
        }

        // ---- World ----------------------------------------------------------------------------------
        function material(spec) {
            return track(new THREE.ShaderMaterial(spec));
        }

        function buildPlanet() {
            const planet = new THREE.Mesh(track(new THREE.SphereGeometry(R, 160, 120)), null);
            planet.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), ring.n);
            scene.add(planet);
            objects.planet = planet;
            objects.ringTexture = track(window.AuraOrbitTextures.ringProfile(THREE, choreo, Math.min(8, renderer.capabilities.getMaxAnisotropy())));
            bakeAll();
            objects.planetMaterial = material({
                uniforms: {
                    uAlbedo: { value: objects.planetPlate.texture }, uRingTex: { value: objects.ringTexture }, uSunDir: U.sunDir, uSunColor: U.sunColor,
                    uRingN: U.ringN, uRimColor: { value: new THREE.Vector3(0.20, 0.38, 0.70) }, uRingRange: U.ringRange, uBolt: { value: bolts },
                    uSpin: { value: 0 }, uShear: { value: 0 }, uTime: U.time, uDetail: { value: 1 }, uGain: { value: 0.74 }, uNoise: U.noise
                },
                vertexShader: shaders.planetVertex, fragmentShader: shaders.planetFragment
            });
            planet.material = objects.planetMaterial;

            const atmosphere = new THREE.Mesh(track(new THREE.SphereGeometry(R * SHELL, 96, 64)), material({
                uniforms: {
                    uSunDir: U.sunDir, uAxis: U.ringN, uDay: { value: new THREE.Vector3(0.20, 0.58, 1.0) }, uDusk: { value: new THREE.Vector3(0.20, 0.10, 0.95) },
                    uPlanetR: { value: R }, uShellR: { value: R * SHELL }, uIntensity: { value: 0.40 }, uAurora: { value: 1.1 }, uTime: U.time, uSteps: { value: 12 }, uNoise: U.noise
                },
                vertexShader: shaders.atmosphereVertex, fragmentShader: shaders.atmosphereFragment,
                transparent: true, depthWrite: false, blending: THREE.AdditiveBlending
            }));
            atmosphere.renderOrder = 2;
            scene.add(atmosphere);
            objects.atmosphere = atmosphere;
        }

        function buildRings() {
            const mesh = new THREE.Mesh(track(new THREE.RingGeometry(RING.inner * R, RING.outer * R, 256, 1)), material({
                uniforms: {
                    uProfile: { value: objects.ringTexture }, uSunDir: U.sunDir, uSunColor: U.sunColor, uNormal: U.ringN,
                    uRange: U.ringRange, uPlanetR: { value: R }, uTime: U.time
                },
                vertexShader: shaders.ringVertex, fragmentShader: shaders.ringFragment,
                transparent: true, depthWrite: false, side: THREE.DoubleSide
            }));
            mesh.quaternion.copy(ring.quaternion);
            mesh.renderOrder = 3;
            scene.add(mesh);
            objects.rings = mesh;

            // ring dust: points on circular orbits in a thin slab, inner ones faster (the vertex shader moves them)
            const rng = choreo.createRng(SEED * 31);
            const orbit = new Float32Array(DUST_MAX * 4);
            for (let i = 0; i < DUST_MAX; i++) {
                const r = (RING.inner + (RING.outer - RING.inner) * Math.sqrt(rng() * 0.96 + 0.02)) * R;
                orbit[i * 4] = r; orbit[i * 4 + 1] = rng() * Math.PI * 2;
                orbit[i * 4 + 2] = (rng() - 0.5) * 2.4 * (0.4 + rng()); orbit[i * 4 + 3] = rng();
            }
            const geometry = track(new THREE.BufferGeometry());
            geometry.setAttribute('position', new THREE.BufferAttribute(new Float32Array(DUST_MAX * 3), 3));
            geometry.setAttribute('aOrbit', new THREE.BufferAttribute(orbit, 4));
            const dust = new THREE.Points(geometry, objects.dustMaterial = material({
                uniforms: {
                    uProfile: { value: objects.ringTexture }, uRange: U.ringRange, uSunDir: U.sunDir, uTime: U.time, uPx: { value: 1 }, uPlanetR: { value: R }, uRef: { value: distance }
                },
                vertexShader: shaders.dustVertex, fragmentShader: shaders.dustFragment, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending
            }));
            dust.quaternion.copy(ring.quaternion);
            dust.frustumCulled = false;
            dust.renderOrder = 4;
            scene.add(dust);
            objects.dust = dust;
        }

        function buildMoon() {
            const moon = new THREE.Mesh(track(new THREE.SphereGeometry(MOON.radius, 64, 48)), objects.moonMaterial = material({
                uniforms: { uSunDir: U.sunDir, uSunColor: U.sunColor, uPlanetShine: { value: new THREE.Vector3(0.55, 0.50, 0.42) }, uPlanetPos: { value: new THREE.Vector3() }, uPlanetR: { value: R }, uNoise: U.noise },
                vertexShader: shaders.moonVertex, fragmentShader: shaders.moonFragment
            }));
            scene.add(moon);
            objects.moon = moon;
        }

        // The sky root follows the camera: nebula, stars, sun and comets sit at infinity.
        function buildSky() {
            const root = new THREE.Group();
            scene.add(root);
            objects.skyRoot = root;
            const sky = new THREE.Mesh(track(new THREE.SphereGeometry(3000, 48, 32)), objects.skyMaterial = material({
                uniforms: { uPlate: { value: objects.skyPlate.texture }, uExtent: { value: new THREE.Vector2(SKY_EXTENT[0], SKY_EXTENT[1]) }, uGain: { value: 1 } },
                vertexShader: shaders.skyVertex, fragmentShader: shaders.skyFragment, side: THREE.BackSide, depthWrite: false, depthTest: false
            }));
            sky.quaternion.setFromRotationMatrix(new THREE.Matrix4().makeBasis(basis.right, basis.up, basis.back));
            sky.renderOrder = -100;
            sky.frustumCulled = false;
            root.add(sky);

            const rng = choreo.createRng(SEED * 7);
            const forward = basis.back.clone().negate();
            objects.stars = STAR_LAYERS.map((layer, index) => {
                const position = new Float32Array(layer.count * 3);
                const size = new Float32Array(layer.count), bright = new Float32Array(layer.count), temp = new Float32Array(layer.count), phase = new Float32Array(layer.count);
                for (let i = 0; i < layer.count; i++) {
                    // a cone around the view direction, denser toward the nebula band
                    let cos = 0, phi = 0;
                    for (let tries = 0; tries < 4; tries++) {
                        cos = 1 + (Math.cos(64 * RAD) - 1) * rng();
                        phi = rng() * Math.PI * 2;
                        const sin = Math.sqrt(1 - cos * cos);
                        tmp.a.copy(forward).multiplyScalar(cos).addScaledVector(basis.right, Math.cos(phi) * sin).addScaledVector(basis.up, Math.sin(phi) * sin);
                        const off = tmp.a.dot(tmp.b.set(-0.45, 1.0, 0.10).normalize());
                        if (rng() < 0.35 + 0.65 * Math.exp(-off * off / 0.09)) break;
                    }
                    position[i * 3] = tmp.a.x * layer.radius; position[i * 3 + 1] = tmp.a.y * layer.radius; position[i * 3 + 2] = tmp.a.z * layer.radius;
                    const k = Math.pow(rng(), index === 0 ? 2.2 : 1.6);
                    size[i] = layer.size[0] + (layer.size[1] - layer.size[0]) * k;
                    bright[i] = layer.bright[0] + (layer.bright[1] - layer.bright[0]) * k;
                    temp[i] = rng(); phase[i] = rng();
                }
                const geometry = track(new THREE.BufferGeometry());
                geometry.setAttribute('position', new THREE.BufferAttribute(position, 3));
                geometry.setAttribute('aSize', new THREE.BufferAttribute(size, 1));
                geometry.setAttribute('aBright', new THREE.BufferAttribute(bright, 1));
                geometry.setAttribute('aTemp', new THREE.BufferAttribute(temp, 1));
                geometry.setAttribute('aPhase', new THREE.BufferAttribute(phase, 1));
                const points = new THREE.Points(geometry, material({
                    uniforms: { uPx: { value: 1 }, uTime: U.time, uTwinkle: { value: motion ? 1 : 0 } },
                    vertexShader: shaders.starVertex, fragmentShader: shaders.starFragment, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending
                }));
                points.frustumCulled = false;
                points.renderOrder = 1;
                points.userData.count = layer.count;
                points.userData.shift = layer.shift;
                root.add(points);
                return points;
            });

            // the sun: a billboard far away in the screen direction of the composition, plus the comets
            objects.sunMaterial = material({
                uniforms: { uSize: { value: 520 }, uColor: { value: new THREE.Vector3(1.0, 0.92, 0.78) }, uGain: { value: 1 } },
                vertexShader: shaders.billboardVertex, fragmentShader: shaders.sunFragment, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending
            });
            const sun = new THREE.Mesh(track(new THREE.PlaneGeometry(2, 2)), objects.sunMaterial);
            sun.frustumCulled = false;
            sun.renderOrder = 5;
            root.add(sun);
            objects.sun = sun;

            const tAttr = new Float32Array(COMET_POINTS);
            for (let i = 0; i < COMET_POINTS; i++) tAttr[i] = i / (COMET_POINTS - 1);
            objects.comets = [];
            for (let i = 0; i < COMET_SLOTS; i++) {
                const geometry = track(new THREE.BufferGeometry());
                geometry.setAttribute('position', new THREE.BufferAttribute(new Float32Array(COMET_POINTS * 3), 3));
                geometry.setAttribute('aT', new THREE.BufferAttribute(tAttr, 1));
                const comet = new THREE.Points(geometry, material({
                    uniforms: { uHead: { value: new THREE.Vector3() }, uTail: { value: new THREE.Vector3() }, uLen: { value: 240 }, uPx: { value: 1 }, uAlpha: { value: 0 } },
                    vertexShader: shaders.cometVertex, fragmentShader: shaders.cometFragment, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending
                }));
                comet.frustumCulled = false;
                comet.visible = false;
                comet.renderOrder = 6;
                root.add(comet);
                objects.comets.push(comet);
            }
        }

        // Ghosts of the lens flare, drawn in screen space after the scene (and bloomed with it).
        function buildFlares() {
            flareScene = new THREE.Scene();
            flareCamera = new THREE.OrthographicCamera(-1, 1, 1, -1, -1, 1);
            // line: where the ghost sits on the line from the screen centre (0) through the sun (1); negative is beyond the centre
            const specs = [
                { line: 1.0, w: 1.15, h: 0.11, shape: 3, alpha: 0.05, color: [0.55, 0.75, 1.0] },
                { line: 0.62, w: 0.045, h: 0.045, shape: 0, alpha: 0.03, color: [1.0, 0.72, 0.42] },
                { line: 0.25, w: 0.07, h: 0.07, shape: 1, alpha: 0.010, color: [0.45, 0.78, 1.0] },
                { line: -0.15, w: 0.05, h: 0.05, shape: 2, alpha: 0.04, color: [0.7, 0.55, 1.0] },
                { line: -0.5, w: 0.12, h: 0.12, shape: 0, alpha: 0.006, color: [1.0, 0.8, 0.55] }
            ];
            const geometry = track(new THREE.PlaneGeometry(2, 2));
            objects.flares = specs.map(spec => {
                const mesh = new THREE.Mesh(geometry, material({
                    uniforms: { uColor: { value: new THREE.Vector3(spec.color[0], spec.color[1], spec.color[2]) }, uAlpha: { value: spec.alpha }, uShape: { value: spec.shape } },
                    vertexShader: shaders.flareVertex, fragmentShader: shaders.flareFragment, transparent: true, depthWrite: false, depthTest: false, blending: THREE.AdditiveBlending
                }));
                mesh.frustumCulled = false;
                mesh.userData = spec;
                flareScene.add(mesh);
                return mesh;
            });
        }

        // Metals need something to reflect: a dim gradient sky with a sun disc and the warm glow of the giant, filtered once.
        function buildEnvironment() {
            const envScene = new THREE.Scene();
            const envMaterial = new THREE.ShaderMaterial({
                uniforms: {
                    uSun: { value: U.sunDir.value.clone() },
                    uPlanet: { value: basis.back.clone().negate().addScaledVector(basis.right, 0.45).addScaledVector(basis.up, -0.35).normalize() }
                },
                vertexShader: 'varying vec3 vDir; void main() { vDir = normalize(position); gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }',
                fragmentShader: 'precision highp float; varying vec3 vDir; uniform vec3 uSun; uniform vec3 uPlanet;' +
                    'void main() { vec3 d = normalize(vDir);' +
                    ' vec3 col = vec3(0.010, 0.014, 0.034) + vec3(0.040, 0.034, 0.090) * smoothstep(-0.3, 1.0, d.y);' +
                    ' col += vec3(0.020, 0.030, 0.060) * smoothstep(0.2, -1.0, d.y);' +
                    ' float s = max(dot(d, uSun), 0.0);' +
                    ' col += vec3(1.0, 0.92, 0.78) * (pow(s, 900.0) * 90.0 + pow(s, 30.0) * 0.9 + pow(s, 4.0) * 0.10);' +
                    ' float p = max(dot(d, uPlanet), 0.0);' +
                    ' col += vec3(0.95, 0.66, 0.40) * pow(p, 2.5) * 0.34;' +
                    ' gl_FragColor = vec4(col, 1.0); }',
                side: THREE.BackSide, depthWrite: false
            });
            const mesh = new THREE.Mesh(new THREE.SphereGeometry(50, 32, 16), envMaterial);
            envScene.add(mesh);
            const generator = new THREE.PMREMGenerator(renderer);
            const target = generator.fromScene(envScene, 0.04);
            generator.dispose();
            mesh.geometry.dispose();
            envMaterial.dispose();
            objects.envTarget = target;
            scene.environment = target.texture;
            scene.environmentIntensity = 0.7;
        }

        function buildLighting() {
            const sun = new THREE.DirectionalLight(0xfff1dc, 2.7);
            sun.position.copy(U.sunDir.value).multiplyScalar(1000);
            scene.add(sun, new THREE.AmbientLight(0x1a2350, 0.35), new THREE.HemisphereLight(0x4060c0, 0x100818, 0.25));
        }

        // ---- Post chain -----------------------------------------------------------------------------
        function destroyComposer() {
            if (!composer) return;
            try { composer.dispose(); } catch (_) { /* best effort */ }
            for (const pass of composer.passes || []) { try { if (pass.dispose) pass.dispose(); } catch (_) { /* best effort */ } }
            composer = null; gradePass = null; bloomPass = null;
        }

        function buildComposer() {
            destroyComposer();
            const spec = QUALITY[quality];
            const target = new THREE.WebGLRenderTarget(Math.max(1, Math.round(width * dpr)), Math.max(1, Math.round(height * dpr)), {
                type: THREE.HalfFloatType, samples: spec.samples, depthBuffer: true, stencilBuffer: false
            });
            composer = new vendor.EffectComposer(renderer, target);
            composer.setPixelRatio(dpr);
            composer.setSize(width, height);
            composer.addPass(new vendor.RenderPass(scene, camera));
            const flares = new vendor.RenderPass(flareScene, flareCamera);
            flares.clear = false;
            flares.clearDepth = true;
            composer.addPass(flares);
            if (spec.bloom > 0) {
                bloomPass = new vendor.UnrealBloomPass(new THREE.Vector2(width * dpr, height * dpr), 0.85, 0.55, 0.72);
                if (spec.bloom < 1) {
                    const base = bloomPass.setSize.bind(bloomPass);
                    bloomPass.setSize = (w, h) => base(Math.max(2, w >> 1), Math.max(2, h >> 1));
                    bloomPass.setSize(width * dpr, height * dpr);
                }
                composer.addPass(bloomPass);
            }
            gradePass = new vendor.ShaderPass({
                uniforms: {
                    tDiffuse: { value: null }, uRes: { value: new THREE.Vector2(1, 1) }, uAspect: { value: 1 }, uTime: { value: 0 }, uVignette: { value: 0.35 },
                    uAberration: { value: 0.0016 }, uGrain: { value: 0.035 }, uPulseCenter: { value: new THREE.Vector2(0.5, 0.5) }, uPulseTime: { value: -1 },
                    uMeteorA: { value: [0, 1, 2].map(() => new THREE.Vector4()) }, uMeteorB: { value: [0, 1, 2].map(() => new THREE.Vector4()) }
                },
                vertexShader: shaders.gradeVertex, fragmentShader: shaders.gradeFragment
            });
            composer.addPass(gradePass);
            composer.addPass(new vendor.OutputPass());
            samplesNow = spec.samples;
            bloomNow = spec.bloom;
        }

        // ---- Camera, lens, layout -------------------------------------------------------------------
        function applyLens() {
            const comp = choreo.composition(width, height);
            const fov = FOV * RAD;
            const radius = comp.planet.radius * Math.min(width, height);
            const tanA = radius * Math.tan(fov / 2) / (height / 2);
            distance = R / (tanA / Math.sqrt(1 + tanA * tanA));
            for (const cam of [camera, refCam]) {
                cam.aspect = width / height;
                cam.setViewOffset(width, height, (0.5 - comp.planet.x) * width, (0.5 - comp.planet.y) * height, width, height);
                cam.updateProjectionMatrix();
            }
            refCam.position.copy(basis.back).multiplyScalar(distance);
            refCam.up.set(0, 1, 0);
            refCam.lookAt(0, 0, 0);
            refCam.updateMatrixWorld(true);
            return comp;
        }

        function rayPoint(sx, sy, dist, out) {
            tmp.d.set(sx * 2 - 1, 1 - sy * 2, 0.5).unproject(refCam).sub(refCam.position).normalize();
            return out.copy(refCam.position).addScaledVector(tmp.d, dist);
        }

        function layoutWorld() {
            const comp = applyLens();
            // the sun billboard sits in the sky root, in the direction of its place on screen
            rayPoint(comp.sun.x, comp.sun.y, 1, tmp.c);
            objects.sun.position.copy(tmp.c).sub(refCam.position).normalize().multiplyScalar(SUN_DISTANCE);
            objects.dustMaterial.uniforms.uRef.value = distance;
            rayPoint(0.5, 0.5, 1, tmp.c);
            view.forward.copy(tmp.c).sub(refCam.position).normalize();
            view.right.crossVectors(view.forward, basis.up).normalize();
            view.up.crossVectors(view.right, view.forward).normalize();
            if (models) models.layout({ rayPoint, basis, composition: comp, ringN: ring.n, ringE1: ring.e1, ringE2: ring.e2 });
            return comp;
        }

        function updateCamera(dt, t) {
            if (motion) {
                const blend = 1 - Math.exp(-dt / POINTER_TAU);
                pointer.x += (pointer.goalX - pointer.x) * blend;
                pointer.y += (pointer.goalY - pointer.y) * blend;
            }
            const pose = choreo.cameraPose(t, motion ? pointer : null, motion);
            const az = CAMERA.azimuth * RAD + pose.yaw, el = CAMERA.elevation * RAD + pose.pitch;
            camera.position.set(Math.sin(az) * Math.cos(el), Math.sin(el), Math.cos(az) * Math.cos(el)).multiplyScalar(distance);
            camera.up.set(0, 1, 0);
            camera.lookAt(0, 0, 0);
            camera.rotateZ(pose.roll);
            camera.updateMatrixWorld(true);
        }

        // ---- Per-frame world update -----------------------------------------------------------------
        function eventEnvelope(age) {
            if (age < 0 || age > 0.25) return 0;
            return Math.max(Math.exp(-Math.pow((age - 0.03) / 0.035, 2)), 0.7 * Math.exp(-Math.pow((age - 0.13) / 0.05, 2)));
        }

        function updateLightning(t) {
            for (const bolt of bolts) bolt.w = 0;
            const events = choreo.eventsBetween(SEED, 'lightning', t - 0.3, t + 1e-6);
            // flashes are placed around the part of the night side that faces the camera
            tmp.a.copy(U.sunDir.value).negate().multiplyScalar(0.55).addScaledVector(basis.back, 0.85).normalize();
            tmp.b.crossVectors(tmp.a, ring.n).normalize();
            tmp.c.crossVectors(tmp.a, tmp.b).normalize();
            let slot = 0;
            for (const event of events) {
                const k = eventEnvelope(t - event.start) * event.strength;
                if (k <= 0 || slot >= bolts.length) continue;
                const spread = 0.75;
                tmp.d.copy(tmp.a).addScaledVector(tmp.b, Math.cos(event.lon) * spread * (0.4 + Math.abs(event.lat))).addScaledVector(tmp.c, Math.sin(event.lon) * spread * 0.7 + event.lat * 0.5).normalize();
                bolts[slot].set(tmp.d.x, tmp.d.y, tmp.d.z, k);
                slot++;
            }
        }

        function updateComets(t) {
            const events = choreo.eventsBetween(SEED, 'comet', t - 11, t);
            const sunSky = tmp.c.copy(objects.sun.position).normalize();
            let slot = 0;
            for (const event of events) {
                if (t >= event.start + event.duration || slot >= objects.comets.length) continue;
                const k = (t - event.start) / event.duration;
                const point = objects.comets[slot++];
                const place = (p, out) => {
                    // squeeze the sky cone toward the view direction so that most comets cross the picture
                    const lx = p.x / 2400 * 0.5, ly = p.y / 2400 * 0.5, lz = -Math.sqrt(Math.max(0, 1 - lx * lx - ly * ly));
                    return out.set(0, 0, 0).addScaledVector(view.right, lx).addScaledVector(view.up, ly).addScaledVector(view.forward, -lz);
                };
                const from = place(event.from, tmp.a).clone();
                const to = place(event.to, tmp.b).clone();
                const head = from.lerp(to, k).normalize();
                const tail = tmp.d.copy(head).sub(sunSky);
                tail.addScaledVector(head, -tail.dot(head)).normalize();
                const u = point.material.uniforms;
                u.uHead.value.copy(head).multiplyScalar(2400);
                u.uTail.value.copy(tail);
                u.uAlpha.value = smoothstep(0, 0.12, k) * (1 - smoothstep(0.82, 1, k));
                point.visible = true;
            }
            for (let i = slot; i < objects.comets.length; i++) objects.comets[i].visible = false;
        }

        function updateMeteors(t) {
            // the pass holds its own copies of the uniform arrays (cloneUniforms deep-clones them)
            const meteorA = gradePass.uniforms.uMeteorA.value, meteorB = gradePass.uniforms.uMeteorB.value;
            for (let i = 0; i < 3; i++) meteorB[i].set(0, 0, 0, 0);
            const events = choreo.eventsBetween(SEED, 'meteor', t - 1.2, t + 1e-6);
            let slot = 0;
            for (const event of events) {
                const age = t - event.start;
                if (age < 0 || age > event.duration || slot >= 3) continue;
                const p = age / event.duration;
                meteorA[slot].set(event.x0, event.y0, event.angle, event.length);
                meteorB[slot].set(p, smoothstep(0, 0.12, p) * (1 - smoothstep(0.7, 1, p)), 0, 0);
                slot++;
            }
        }

        // Projects the sun and the planet to place the flare ghosts; the planet covers the sun and its ghosts.
        function updateFlares() {
            const aspect = width / height;
            tmp.a.copy(objects.sun.position).applyQuaternion(objects.skyRoot.quaternion).add(camera.position).project(camera);
            const sx = tmp.a.x, sy = tmp.a.y;
            const front = tmp.a.z < 1 ? 1 : 0;
            tmp.b.set(0, 0, 0).project(camera);
            const dist = camera.position.length();
            const rNdc = Math.tan(Math.asin(R / dist)) / Math.tan(FOV * RAD / 2);
            const gap = Math.hypot((sx - tmp.b.x) * aspect, sy - tmp.b.y);
            const occluded = smoothstep(rNdc * 0.97, rNdc * 1.06, gap);
            const edge = 1 - smoothstep(1.0, 1.7, Math.max(Math.abs(sx), Math.abs(sy)));
            // the ghosts are for a sun at the edge of the frame; toward the centre they fade to a quarter
            const near = 0.25 + 0.75 * smoothstep(0.3, 0.85, Math.max(Math.abs(sx), Math.abs(sy)));
            const vis = front * occluded * edge * near;
            flareCamera.left = -aspect; flareCamera.right = aspect;
            flareCamera.updateProjectionMatrix();
            for (const mesh of objects.flares) {
                const spec = mesh.userData;
                mesh.position.set(sx * aspect * spec.line, sy * spec.line, 0);
                mesh.scale.set(spec.w, spec.h, 1);
                mesh.material.uniforms.uAlpha.value = spec.alpha * vis;
                mesh.visible = vis > 0.003;
            }
            objects.sunMaterial.uniforms.uGain.value = 1;
        }

        function updateWorld(t, dt) {
            U.time.value = t % 100000;
            objects.planetMaterial.uniforms.uSpin.value = spinTurns(t);
            objects.planetMaterial.uniforms.uShear.value = shearPhase(t);
            updateLightning(t);
            const a = (MOON.phase + 2 * Math.PI * t / MOON.period);
            const inc = MOON.inclination * RAD;
            tmp.a.copy(ring.e1).multiplyScalar(Math.cos(a));
            tmp.b.copy(ring.e2).multiplyScalar(Math.cos(inc)).addScaledVector(ring.n, Math.sin(inc)).multiplyScalar(Math.sin(a));
            objects.moon.position.copy(tmp.a).add(tmp.b).multiplyScalar(MOON.orbit);
            objects.moonMaterial.uniforms.uPlanetPos.value.set(0, 0, 0);
            objects.skyRoot.position.copy(camera.position);
            // a slow sway of about half a degree, never a drift: the nebula plate and the star cone cover a window of the sky
            objects.skyRoot.quaternion.setFromAxisAngle(basis.up, 0.01 * Math.sin(TAU * t / 900));
            for (const layer of objects.stars) {
                const k = layer.userData.shift;
                layer.position.copy(basis.right).multiplyScalar(-pointer.x * k).addScaledVector(basis.up, pointer.y * k);
            }
            updateComets(t);
            updateMeteors(t);
            updateFlares();
            tmp.a.copy(camera.position).normalize();
            rig.setFromUnitVectors(basis.back, tmp.a);
            if (models) models.update(t, dt, quality >= 2, rig);
            if (pulseAt >= 0) pulseAt += dt;
            const grade = gradePass.uniforms;
            grade.uTime.value = t % 1000;
            grade.uPulseTime.value = pulseAt >= 0 && pulseAt < PULSE_SECONDS ? pulseAt : -1;
            if (pulseAt >= PULSE_SECONDS) pulseAt = -1;
            grade.uAspect.value = width / height;
            grade.uRes.value.set(canvas.width, canvas.height);
        }

        // ---- Quality, size --------------------------------------------------------------------------
        function readSize() {
            width = Math.max(1, Math.round(window.innerWidth || 1));
            height = Math.max(1, Math.round(window.innerHeight || 1));
            dpr = Math.min(window.devicePixelRatio || 1, QUALITY[quality].dpr, Math.sqrt(PIXEL_BUDGET[quality] / (width * height)));
        }

        function applySize() {
            if (resizeTimer) { clearTimeout(resizeTimer); resizeTimer = 0; }
            if (!renderer || !camera || disposed) return;
            readSize();
            renderer.setPixelRatio(dpr);
            renderer.setSize(width, height, false);
            const spec = QUALITY[quality];
            if (!composer || samplesNow !== spec.samples || bloomNow !== spec.bloom) buildComposer();
            else { composer.setPixelRatio(dpr); composer.setSize(width, height); }
            layoutWorld();
            for (const layer of objects.stars) layer.material.uniforms.uPx.value = dpr;
            for (const comet of objects.comets) comet.material.uniforms.uPx.value = dpr;
            objects.dustMaterial.uniforms.uPx.value = dpr;
            if (models) models.setQuality(QUALITY[quality].rocks, dpr);
            if (!built || contextLost) return;
            if (raf) { lastRender = 0; skipGap = true; } else renderStatic();
        }

        function resize() {
            if (!renderer || !camera || disposed) return;
            if (built && !raf) {
                if (resizeTimer) clearTimeout(resizeTimer);
                resizeTimer = setTimeout(applySize, RESIZE_DEBOUNCE_MS);
                return;
            }
            applySize();
        }

        function applyQuality(level) {
            const next = clamp(Math.round(level), 0, QUALITY.length - 1);
            const spec = QUALITY[next];
            const changed = next !== quality;
            quality = next;
            if (typeof options.onQuality === 'function') { try { options.onQuality(next); } catch (_) { /* optional */ } }
            objects.atmosphere.material.uniforms.uSteps.value = spec.steps;
            objects.planetMaterial.uniforms.uDetail.value = spec.detail;
            objects.dust.geometry.setDrawRange(0, spec.dust);
            for (const layer of objects.stars) layer.geometry.setDrawRange(0, Math.round(layer.userData.count * spec.stars));
            if (models) models.setQuality(spec.rocks, dpr);
            if (spec.hidden) hideHost();
            else if (host.style.display === 'none' && backend === 'three-webgl') showHost();
            warm = ADAPT.warmup; costs.length = 0; gaps.length = 0; probeCosts.length = 0; fastSince = 0; skipGap = true;
            if (changed || !built) applySize();
            if (changed) startLoops();
        }

        function setQualityLevel(level, automatic) {
            if (automatic && qualityLock !== null) return;
            applyQuality(level);
        }

        // ---- Adaptive quality (the numbers of the Voxel World scene) ----------------------------------
        function mean(list) { return list.reduce((a, b) => a + b, 0) / list.length; }
        function trackRefresh(now) {
            if (lastTick) {
                const delta = now - lastTick;
                if (delta >= 4 && delta < refresh) refresh = delta;
            }
            lastTick = now;
        }
        function expectedGap(interval) { return Math.max(interval, Math.ceil((interval - FRAME_SLACK) / refresh) * refresh); }

        function adapt(cost, gap, interval, now) {
            if (throttled) { fastSince = 0; return; }
            if (warm > 0) { warm--; return; }
            costs.push(cost);
            if (costs.length > ADAPT.window) costs.shift();
            if (gap !== null) { gaps.push(gap); if (gaps.length > ADAPT.window) gaps.shift(); }
            if (qualityLock !== null) { fastSince = 0; return; }
            const expected = expectedGap(interval);
            const full = costs.length >= ADAPT.window && gaps.length >= ADAPT.window;
            const meanCost = mean(costs);
            const meanGap = gaps.length ? mean(gaps) : 0;
            const stalled = gap !== null && gap > ADAPT.stallMs;
            if (quality < QUALITY.length - 1 && (stalled || (full && (meanCost > ADAPT.downCost || meanGap > expected * ADAPT.downGap)))) {
                setQualityLevel(quality + 1, true);
            } else if (quality > initialQuality && full && meanCost < ADAPT.upCost && meanGap < expected * ADAPT.upGap) {
                if (!fastSince) fastSince = now;
                else if (now - fastSince >= ADAPT.upAfterMs) setQualityLevel(quality - 1, true);
            } else fastSince = 0;
        }

        function probeTick() {
            if (!built || disposed || paused || contextLost || !renderer || quality !== PROBE.level) return;
            const begin = performance.now();
            if (frozen === null) clock += PROBE.everyMs / 1000;
            drawFrame(0, begin);
            try { renderer.getContext().finish(); } catch (_) { /* the probe then measures the CPU only */ }
            lastCost = performance.now() - begin;
            probeCosts.push(lastCost);
            const needed = Math.ceil(ADAPT.upAfterMs / PROBE.everyMs);
            while (probeCosts.length > needed) probeCosts.shift();
            if (qualityLock === null && quality > initialQuality && probeCosts.length >= needed && mean(probeCosts) < ADAPT.upCost) setQualityLevel(quality - 1, true);
        }

        function onVisibility() { skipGap = true; lastTick = 0; gaps.length = 0; fastSince = 0; }

        // ---- Frames -----------------------------------------------------------------------------------
        function sceneTime() { return frozen !== null ? frozen : motion ? clock : STATIC_TIME; }

        function drawFrame(dt, now) {
            if (!renderer || disposed || contextLost || !built) return;
            const begin = performance.now();
            const t = sceneTime();
            try {
                updateCamera(dt, t);
                updateWorld(t, dt);
                renderer.info.reset();
                composer.render(dt);
            } catch (error) {
                backend = 'error';
                stopLoops();
                hideHost();
                noteError('render', error);
                finishReady();
                return;
            }
            triangles = renderer.info.render.triangles;
            frames++;
            lastCost = performance.now() - begin;
            stamps.push(now);
            while (stamps.length && now - stamps[0] > 2000) stamps.shift();
            if (!ready) {
                ready = true;
                if (!QUALITY[quality].hidden) host.style.opacity = '1';
                finishReady();
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
            const wasThrottled = throttled;
            throttled = coverage >= THROTTLE.coverage;
            if (throttled !== wasThrottled) { skipGap = true; gaps.length = 0; fastSince = 0; }
            const interval = throttled ? THROTTLE.interval : QUALITY[quality].interval;
            if (lastRender && now - lastRender < interval - FRAME_SLACK) return;
            const sample = !!lastRender && !skipGap && !document.hidden;
            const gap = lastRender ? now - lastRender : interval;
            const dt = lastRender ? Math.min(gap / 1000, 0.1) : 1 / 30;
            skipGap = false;
            lastRender = now;
            if (frozen === null) clock += dt;
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
            lastRender = 0; skipGap = true; gaps.length = 0; fastSince = 0;
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

        // ---- Build ------------------------------------------------------------------------------------
        function build() {
            buildTimer = 0;
            if (disposed) return;
            const started = performance.now();
            buildStart = started;
            try {
                if (!createRenderer()) { finishReady(); return; }
                U.noise.value = track(window.AuraOrbitTextures.noise(THREE, choreo.createRng(SEED * 101)));
                computeFrame();
                scene = new THREE.Scene();
                camera = new THREE.PerspectiveCamera(FOV, 1, 1, 6000);
                refCam = new THREE.PerspectiveCamera(FOV, 1, 1, 6000);
                scene.add(camera);
                mark('setup');
                buildPlanet();
                mark('planet');
                buildRings();
                buildMoon();
                buildSky();
                buildFlares();
                buildLighting();
                buildEnvironment();
                mark('world');
                models = window.AuraOrbitModels.create({
                    THREE, GLTFLoader: vendor.GLTFLoader, scene, choreo, assetsBase, shaders, noteError, pixelRatio: () => dpr
                });
                readSize();
                const ratio = Math.min(window.devicePixelRatio || 1, QUALITY[0].dpr);
                initialQuality = width * height * ratio * ratio > INITIAL_PIXELS ? 1 : 0;
                quality = -1;
                applyQuality(qualityLock !== null ? qualityLock : initialQuality);
                mark('quality');
            } catch (error) {
                backend = 'error';
                hideHost();
                noteError('build', error);
                releaseRenderer();
                finishReady();
                return;
            }
            buildMs = Math.round(performance.now() - started);
            built = true;
            models.load().catch(error => noteError('models', error)).then(() => {
                if (disposed) return;
                layoutWorld();
                models.setQuality(QUALITY[quality].rocks, dpr);
                if (!raf) renderStatic();
            });
            startLoops();
        }

        buildTimer = setTimeout(build, 0);
        document.addEventListener('visibilitychange', onVisibility);
        window.addEventListener('resize', resize, { passive: true });

        // ---- Public surface ---------------------------------------------------------------------------
        function setState(state) {
            if (!state) return;
            if (state.coverage != null) coverage = Number(state.coverage) || 0;
            if (state.pointer !== undefined && motion) {
                const p = state.pointer;
                pointer.goalX = p ? clamp(Number(p.x) || 0, -1, 1) : 0;
                pointer.goalY = p ? clamp(Number(p.y) || 0, -1, 1) : 0;
            }
        }

        function ping(x, y) {
            if (!renderer || !gradePass) return;
            gradePass.uniforms.uPulseCenter.value.set(clamp(x / Math.max(1, width), 0, 1), 1 - clamp(y / Math.max(1, height), 0, 1));
            pulseAt = 0;
            skipGap = true;
        }

        function pause() { paused = true; stopLoops(); }
        function resume() { if (!paused) return; paused = false; startLoops(); }

        function lockQuality(level) {
            qualityLock = level !== null && level !== undefined && Number.isFinite(Number(level)) ? clamp(Math.round(Number(level)), 0, QUALITY.length - 1) : null;
            if (qualityLock !== null && built) setQualityLevel(qualityLock, false);
        }

        function capture(w, h, grain) {
            if (!built || !renderer || contextLost) return null;
            const saved = { width, height, dpr };
            const cw = clamp(Math.round(w) || 1920, 64, 4096), ch = clamp(Math.round(h) || 1080, 64, 4096);
            width = cw; height = ch; dpr = 1;
            renderer.setPixelRatio(1);
            renderer.setSize(cw, ch, false);
            composer.setPixelRatio(1);
            composer.setSize(cw, ch);
            layoutWorld();
            const grainUniform = gradePass.uniforms.uGrain, savedGrain = grainUniform.value;
            if (grain !== null) grainUniform.value = grain;
            drawFrame(0, performance.now());
            const url = canvas.toDataURL('image/png');
            grainUniform.value = savedGrain;
            width = saved.width; height = saved.height; dpr = saved.dpr;
            applySize();
            return url;
        }

        function command(name, payload) {
            const number = key => (payload && payload[key] != null && Number.isFinite(Number(payload[key])) ? Number(payload[key]) : null);
            if (name === 'orbit.time') {
                frozen = number('t');
                if (built && !raf) renderStatic();
                return inspect();
            }
            if (name === 'orbit.quality') { lockQuality(number('level')); return inspect(); }
            if (name === 'orbit.capture') return capture(number('width') || 1920, number('height') || 1080, number('grain'));
            return null;
        }

        function planetOnScreen() {
            if (!camera || !built) return null;
            tmp.a.set(0, 0, 0).project(camera);
            const dist = camera.position.length();
            const radius = Math.tan(Math.asin(R / dist)) / Math.tan(FOV * RAD / 2) * height / 2;
            return { x: Math.round((tmp.a.x * 0.5 + 0.5) * width), y: Math.round((-tmp.a.y * 0.5 + 0.5) * height), radius: Math.round(radius) };
        }

        function inspect() {
            const now = performance.now();
            while (stamps.length && now - stamps[0] > 2000) stamps.shift();
            const modelState = models ? models.inspect() : null;
            return {
                ready, backend, gpu, quality, qualityLocked: qualityLock !== null,
                pixels: { width: renderer ? canvas.width : 0, height: renderer ? canvas.height : 0 },
                fps: Math.round(stamps.length / 2 * 10) / 10, frames, buildMs, triangles: Math.round(triangles),
                models: models ? { ...models.status } : { station: 'pending', shuttle: 'pending', satellite: 'pending', asteroids: 'pending' },
                bloom: QUALITY[Math.max(quality, 0)].bloom > 0, throttled, coverage: Math.round(coverage * 1000) / 1000, pulse: pulseAt >= 0 ? Math.round(pulseAt * 100) / 100 : null, time: Math.round(sceneTime() * 100) / 100, error: lastError,
                lastCostMs: Math.round(lastCost * 10) / 10, gapMs: gaps.length ? Math.round(mean(gaps) * 10) / 10 : 0, refreshMs: Math.round(refresh * 10) / 10,
                timings, planet: planetOnScreen(), distance: Math.round(distance * 10) / 10, scenery: modelState
            };
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            if (buildTimer) clearTimeout(buildTimer);
            if (resizeTimer) clearTimeout(resizeTimer);
            buildTimer = resizeTimer = 0;
            document.removeEventListener('visibilitychange', onVisibility);
            window.removeEventListener('resize', resize);
            stopLoops();
            if (models) { try { models.dispose(); } catch (_) { /* best effort */ } }
            destroyComposer();
            for (const key of ['planetPlate', 'skyPlate', 'envTarget']) { if (objects[key]) { try { objects[key].dispose(); } catch (_) { /* best effort */ } } }
            for (const item of disposables) { try { item.dispose(); } catch (_) { /* best effort */ } }
            releaseRenderer();
            host.remove();
            finishReady();
        }

        return { ready: readyPromise, setState, ping, pause, resume, dispose, command, inspect };
    }

    window.AuraOrbitScene = Object.freeze({ create });
}());
