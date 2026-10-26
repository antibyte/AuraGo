(function () {
    'use strict';

    // Everything that moves on the floating isle besides the foliage sway: the cloud sea and the sky clouds (and the cloud
    // shadows they cast on the terrain), the waterfall with spray, foam, mist and a rainbow, the windmill sails, chimney smoke, a flock
    // of birds, fireflies, glow halos at lanterns, windows and crystals, a shooting star, the bobbing islets and the boat.
    //
    // Every motion is a closed-form function of the clock t (no integration, no state beyond the instance buffers), so a frozen
    // clock gives a still picture and a static (reduced motion) scene needs one update to place everything. Nothing here casts
    // a shadow. Positions come from island.features (x/z are cell indices, y is a world y); world x = x + .5 - origin x.

    // ---- Look tunables ------------------------------------------------------------------------
    // The camera rig of voxel-scene.js at its base pose; the sky clouds and the shooting star are placed in the camera's frame.
    const VIEW = { yaw: 40, pitch: 26, distance: 215, fov: 21, target: [0, 4, 0] };
    const DRIFT_SPEED = .9;                       // u/s of the sky clouds and their shadows, along the screen's right
    // The cloud sea: a smooth hazy floor under and around the island with puffy clouds on it, in a bowl: its surface is `top` below the
    // island and rises by `rise` between the radii `bowl`, clearing the island's underside and meeting the waterfall where the camera
    // sees it. A puff is a rigid cluster of 3-7 boxes in 1-3 tiers (a wide box in the middle, smaller ones around it and on top, each at
    // least 45 % as high as wide) standing in the floor and drifting along the screen's right. Puffs shrink where the bowl is steep
    // (slopeFade, height per distance) and around the waterfall's landing (clear, units), where the floor is a flat ledge (pad). They sit
    // on a lattice (pitch across and along the view, skip = share left empty) whose half width per depth (span) follows the camera's view
    // cone; near/far are depths along the view direction away from the island, eye the camera's horizontal distance from its axis. From
    // haze[0] to haze[1] the sea blends into the horizon colour and the puffs shrink away around far, so the far edge melts into the sky.
    // look: tops are the cloud tint x topGain, sides the tint mixed shadowMix towards the sky (mean of horizon and top colours) x shadowGain.
    const SEA = {
        top: -36, rise: 19, bowl: [14, 28], near: -110, far: 40, haze: [4, 34], eye: 193, speed: .6, fadeBand: 16,
        span: { slope: .55, base: 10, min: 66, max: 150 }, pitch: [11.5, 9], spread: .35, skip: .12,
        slopeFade: [.45, 1.15], clear: [8, 15], pad: [4.5, 11],
        floor: { drop: -3, sink: .8, rows: 40, cols: 48, wobble: .45, shade: [.9, 1], normalY: .5 },
        puff: { w: [4.2, 8], aspect: [.85, 1.1], h: [.47, .56], ring: [.6, .85], bump: .9, tiers: [.3, .75], shade: { low: [.92, 1], high: [.97, 1.04] } },
        look: { topGain: 1.55, shadowMix: .5, shadowGain: .9, sideLow: .78, key: .12 }
    };
    const SKY_CLOUD_LOOK = { gain: .92, glow: .42 };   // the sky clouds are lit like terrain: tint x gain as colour, tint x glow as emissive
    const SEA_LEGACY_DRAWS = 10350;                    // random draws the old sea took from the shared stream (keeps the sky clouds and fireflies where they were)
    // Sky clouds in the camera frame: d = distance along the view axis, q = height on screen in half pictures at that distance,
    // lat = world units right of the axis, w = cluster width, n = boxes.
    const SKY_CLOUDS = [
        { d: 330, q: .72, lat: -90, w: 36, n: 14 }, { d: 300, q: .5, lat: 120, w: 32, n: 12 }, { d: 250, q: .82, lat: 20, w: 28, n: 11 },
        { d: 175, q: .5, lat: -40, w: 16, n: 8 }, { d: 360, q: .4, lat: -170, w: 40, n: 14 }, { d: 215, q: .66, lat: 150, w: 24, n: 10 }
    ];
    const CLOUD_SHADOW = { scale: .045, strength: .28 };   // scale must match CLOUD_SHADOW_SCALE of voxel-blocks.js
    // Where the sheet meets the clouds: `foam` white cubes churn in a ring around the landing (rx/rz: reach along x and z, lift: height of
    // the bobbing, size: edge length) and `mist` soft cubes billow up from it (they grow from mistFrom to mistTo while they rise
    // mistRise over mistLife seconds and fade; mistAlpha is their peak opacity).
    const FALL = {
        vx: 1.2, gravity: 14, segments: 28, thickTop: .8, thickBottom: .4, sink: 1.2, spray: 140, sprayMin: .55, sprayMax: .8,
        foam: 26, foamRx: [.4, 3.4], foamRz: [.3, 2.6], foamLift: 3, foamSize: [1, 2.1],
        mist: 30, mistLife: 2.8, mistFrom: .8, mistTo: 2.3, mistRise: 3.6, mistAlpha: .5,
        deep: '#2a6fa8', shallow: '#4fb0d8', white: '#ffffff', sprayFrom: '#8fd3ff', rainbow: { alpha: .22, width: 30, height: 17, inner: 9.5, outer: 12.8 }
    };
    const SAILS = { turn: .6, spar: [.3, 7, .3], cloth: [1.6, 5, .12], clothOffset: .95, spar_color: '#b38a55', cloth_color: '#efe6d2', hubOut: .55 };
    const SMOKE = { perChimney: 10, from: .5, to: 1.6, rise: .9, drift: .35, life: 4, alpha: .65 };
    const BIRDS = { count: 7, a: 46, b: 34, yLow: 14, yHigh: 20, period: 48, flapHz: 6, flap: 35, glide: 8, bank: .32, back: 1.8, side: 1.5, color: '#3b3b4a', wing: '#4a4a5e' };
    const FIREFLIES = { count: 48, size: .18, glow: 1.6, color: '#d8ff7a' };
    const HALO = { lantern: 3.5, window: 2.2, crystal: 2.8, pull: 1.4, gain: .9, lanternColor: '#ffb84a', windowColor: '#ffc76a', crystalColor: '#c48dff' };
    const SHOOTING = { every: 42, spread: 17, duration: .8, length: 7, d: 300, q: .8 };
    const ISLET = { bob: .6, period: 9, turn: 1.5, turnPeriod: 41 };
    const BOAT = { size: [3, .8, 1.6], draft: .3, bob: .08, rock: 3, color: '#b38a55', rim: '#8a6539' };
    const QUALITY_HALF = { spray: .5, mist: .5, foam: .6, smoke: .5, birds: 4 / 7, fireflies: .5, seaDetails: .4 };

    function clamp(v, lo, hi) { return Math.max(lo, Math.min(hi, v)); }
    function lerp(a, b, t) { return a + (b - a) * t; }
    function smooth(a, b, x) { const t = clamp((x - a) / (b - a), 0, 1); return t * t * (3 - 2 * t); }
    function wrap(v, half) { return ((v + half) % (2 * half) + 2 * half) % (2 * half) - half; }
    function hexRGB(hex) {
        const n = parseInt(hex.slice(1), 16);
        return [(n >> 16 & 255) / 255, (n >> 8 & 255) / 255, (n & 255) / 255];
    }
    function mulberry32(seed) {
        let a = seed >>> 0;
        return function () {
            a = (a + 0x6d2b79f5) >>> 0;
            let t = a;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }
    function hash1(n) { const s = Math.sin(n * 127.1 + 311.7) * 43758.5453; return s - Math.floor(s); }

    // ---- GLSL ---------------------------------------------------------------------------------
    // The waterfall sheet: blocky streaks flowing down, white foam at the lip and where it dives into the clouds.
    const FALL_VERTEX = `
#include <fog_pars_vertex>
varying vec2 vUv;
void main() {
    vUv = uv;
    vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
    gl_Position = projectionMatrix * mvPosition;
    #include <fog_vertex>
}`;

    const FALL_FRAGMENT = `
#include <fog_pars_fragment>
uniform float uTime;
uniform float uLen;
uniform vec3 uDeep;
uniform vec3 uShallow;
uniform vec3 uFoam;
uniform vec3 uLight;
varying vec2 vUv;
float fHash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }
float fNoise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    return mix(mix(fHash(i), fHash(i + vec2(1.0, 0.0)), f.x), mix(fHash(i + vec2(0.0, 1.0)), fHash(i + vec2(1.0, 1.0)), f.x), f.y);
}
void main() {
    float flow = vUv.y - uTime * 7.0;
    vec2 q = floor(vec2(vUv.x * 12.0, flow * 2.5));
    float body = mix(fNoise(q * vec2(0.9, 0.35)), fNoise(q * vec2(1.7, 0.8) + 5.0), 0.5);
    vec3 col = mix(uDeep, uShallow, body);
    col = mix(col, uFoam, smoothstep(0.62, 0.9, body) * 0.55);
    vec2 fq = floor(vec2(vUv.x * 10.0 + uTime * 0.4, (vUv.y + uTime * 3.0) * 1.6));
    float foamNoise = fHash(fq);
    float lip = 1.0 - smoothstep(0.0, 1.8, vUv.y);
    float sink = smoothstep(uLen - 4.5, uLen - 0.4, vUv.y);
    float foam = clamp(lip * (0.45 + 0.75 * foamNoise) + sink * (0.55 + 0.6 * foamNoise), 0.0, 1.0);
    col = mix(col, uFoam, foam);
    gl_FragColor = vec4(col * uLight, mix(0.88, 1.0, foam));
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
    #include <fog_fragment>
}`;

    // The rainbow: seven stepped bands of an arc around the quad's bottom centre.
    const RAINBOW_FRAGMENT = `
uniform float uAlpha;
uniform vec2 uSize;
uniform vec2 uRadius;
varying vec2 vUv;
vec3 band(int i) {
    if (i == 0) return vec3(0.90, 0.12, 0.10);
    if (i == 1) return vec3(0.95, 0.45, 0.08);
    if (i == 2) return vec3(0.98, 0.82, 0.12);
    if (i == 3) return vec3(0.25, 0.75, 0.20);
    if (i == 4) return vec3(0.15, 0.55, 0.95);
    if (i == 5) return vec3(0.25, 0.20, 0.80);
    return vec3(0.55, 0.20, 0.75);
}
void main() {
    vec2 p = (vUv - vec2(0.5, 0.0)) * uSize;
    float t = (length(p) - uRadius.x) / (uRadius.y - uRadius.x);
    if (t < 0.0 || t > 1.0 || p.y < 0.0) discard;
    int i = int(floor((1.0 - t) * 7.0));
    float edge = smoothstep(0.0, 0.08, t) * smoothstep(0.0, 0.08, 1.0 - t) * smoothstep(0.0, 2.5, p.y);
    gl_FragColor = vec4(band(i), uAlpha * edge);
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
}`;

    // Additive glow sprites (halos, fireflies): per-point size (world units), alpha and colour.
    const POINT_VERTEX = `
attribute float aSize;
attribute float aAlpha;
attribute vec3 aColor;
uniform float uScale;
uniform float uPull;
varying float vAlpha;
varying vec3 vColor;
void main() {
    vec4 mv = modelViewMatrix * vec4(position, 1.0);
    mv.z += uPull;
    gl_Position = projectionMatrix * mv;
    gl_PointSize = clamp(aSize * uScale / max(-mv.z, 1.0), 1.0, 256.0);
    vAlpha = aAlpha;
    vColor = aColor;
}`;

    const POINT_FRAGMENT = `
varying float vAlpha;
varying vec3 vColor;
void main() {
    float r = length(gl_PointCoord - 0.5) * 2.0;
    float a = pow(max(1.0 - r, 0.0), 2.2) * vAlpha;
    gl_FragColor = vec4(vColor, a);
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
}`;

    // ---- Factory ------------------------------------------------------------------------------
    // create({ THREE, scene, island, origin: [ox, oy, oz], uniforms, patch, islets: [Group, Group], quality: life scale 1 / .5 / 0 })
    //   -> { update(dt, t, sky, motion), setQuality(scale), inspect(), dispose() }
    function create(options) {
        const THREE = options.THREE;
        const island = options.island;
        const features = island.features;
        const ox = options.origin[0];
        const oz = options.origin[2];
        const uniforms = options.uniforms;
        const isletGroups = options.islets || [];
        const rng = mulberry32(0x11fe);
        const rand = (range) => range[0] + (range[1] - range[0]) * rng();

        const root = new THREE.Group();
        root.name = 'voxel-life';
        const dyn = new THREE.Group();          // everything but the clouds: hidden at life scale 0
        root.add(dyn);
        options.scene.add(root);
        const owned = [];
        let scale = clamp(Number(options.quality), 0, 1);
        if (!Number.isFinite(scale)) scale = 1;
        let disposed = false;
        let skyNow = null;
        const counts = { clouds: 0, seaPuffs: 0, skyClusters: SKY_CLOUDS.length, spray: 0, foam: 0, mist: 0, smoke: 0, birds: 0, fireflies: 0, halos: 0 };

        const rad = Math.PI / 180;
        const yaw = VIEW.yaw * rad;
        const pitch = VIEW.pitch * rad;
        const fwd = [-Math.sin(yaw) * Math.cos(pitch), -Math.sin(pitch), -Math.cos(yaw) * Math.cos(pitch)];
        const right = [Math.cos(yaw), 0, -Math.sin(yaw)];
        const upv = [right[1] * fwd[2] - right[2] * fwd[1], right[2] * fwd[0] - right[0] * fwd[2], right[0] * fwd[1] - right[1] * fwd[0]];
        const fh = [-Math.sin(yaw), 0, -Math.cos(yaw)];   // horizontal view direction
        const camPos = VIEW.target.map((c, i) => c - fwd[i] * VIEW.distance);
        const tanHalf = Math.tan(VIEW.fov / 2 * rad);
        // A point in the camera frame: d along the view axis, q (half pictures) up, lat world units right.
        function framePoint(d, q, lat) {
            return [0, 1, 2].map(i => camPos[i] + fwd[i] * d + upv[i] * q * d * tanHalf + right[i] * lat);
        }
        const toWorld = (x, z) => [x + .5 - ox, z + .5 - oz];

        function own(item) { owned.push(item); return item; }
        function setSrgb(color, rgb) { return color.setRGB(rgb[0], rgb[1], rgb[2], THREE.SRGBColorSpace); }
        function colorOf(hex) { return setSrgb(new THREE.Color(), hexRGB(hex)); }
        function lambert(hex, extra) { return own(new THREE.MeshLambertMaterial(Object.assign({ color: colorOf(hex) }, extra))); }
        const unitBox = own(new THREE.BoxGeometry(1, 1, 1));
        function mesh(geometry, material, parent) {
            const m = new THREE.Mesh(geometry, material);
            m.castShadow = false;
            m.receiveShadow = false;
            (parent || dyn).add(m);
            return m;
        }
        function instanced(material, count, parent, geometry) {
            const m = new THREE.InstancedMesh(geometry || unitBox, material, count);
            m.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
            m.frustumCulled = false;
            m.castShadow = false;
            m.receiveShadow = false;
            const white = new THREE.Color(1, 1, 1);
            for (let i = 0; i < count; i++) m.setColorAt(i, white);
            (parent || dyn).add(m);
            return m;
        }
        // Writes a translation + scale matrix straight into the instance buffer.
        function putBox(array, i, x, y, z, sx, sy, sz) {
            const o = i * 16;
            array[o] = sx; array[o + 1] = 0; array[o + 2] = 0; array[o + 3] = 0;
            array[o + 4] = 0; array[o + 5] = sy; array[o + 6] = 0; array[o + 7] = 0;
            array[o + 8] = 0; array[o + 9] = 0; array[o + 10] = sz; array[o + 11] = 0;
            array[o + 12] = x; array[o + 13] = y; array[o + 14] = z; array[o + 15] = 1;
        }
        // Basic material whose opacity is a per-instance attribute (smoke, mist). `shaded` darkens the sides of the cubes a little so
        // that they read as puffs, not as flat patches.
        function fadeMaterial(color, shaded) {
            const material = own(new THREE.MeshBasicMaterial({ color, transparent: true, depthWrite: false }));
            const shade = shaded ? '\n\tvShade = .72 + .28 * clamp(normal.y * .8 + .4, 0., 1.);' : '';
            material.onBeforeCompile = shader => {
                shader.vertexShader = 'attribute float aFade;\nvarying float vFade;\nvarying float vShade;\n' + shader.vertexShader.replace('#include <begin_vertex>', '#include <begin_vertex>\n\tvFade = aFade;\n\tvShade = 1.;' + shade);
                shader.fragmentShader = 'varying float vFade;\nvarying float vShade;\n' + shader.fragmentShader.replace('#include <color_fragment>', '#include <color_fragment>\n\tdiffuseColor.a *= vFade;\n\tdiffuseColor.rgb *= vShade;');
            };
            material.customProgramCacheKey = () => shaded ? 'voxel-life-fade-shaded' : 'voxel-life-fade';
            return material;
        }
        function fadeBoxes(material, count) {
            const geometry = own(new THREE.BoxGeometry(1, 1, 1));
            const fade = new THREE.InstancedBufferAttribute(new Float32Array(count), 1);
            fade.setUsage(THREE.DynamicDrawUsage);
            geometry.setAttribute('aFade', fade);
            const m = instanced(material, count, dyn, geometry);
            m.renderOrder = 5;
            return { mesh: m, fade };
        }
        // Additive glow sprites with per-point position, size, alpha and colour (alpha and position are dynamic).
        function glowPoints(points, parent, pull) {
            const n = points.length;
            const geometry = own(new THREE.BufferGeometry());
            const position = new THREE.BufferAttribute(new Float32Array(n * 3), 3);
            const alpha = new THREE.BufferAttribute(new Float32Array(n), 1);
            const size = new Float32Array(n);
            const color = new Float32Array(n * 3);
            position.setUsage(THREE.DynamicDrawUsage);
            alpha.setUsage(THREE.DynamicDrawUsage);
            points.forEach((p, i) => {
                position.setXYZ(i, p.x, p.y, p.z);
                size[i] = p.size;
                const c = setSrgb(new THREE.Color(), hexRGB(p.color));
                color[i * 3] = c.r; color[i * 3 + 1] = c.g; color[i * 3 + 2] = c.b;
            });
            geometry.setAttribute('position', position);
            geometry.setAttribute('aAlpha', alpha);
            geometry.setAttribute('aSize', new THREE.BufferAttribute(size, 1));
            geometry.setAttribute('aColor', new THREE.BufferAttribute(color, 3));
            const material = own(new THREE.ShaderMaterial({
                uniforms: { uScale: { value: 500 }, uPull: { value: pull } }, vertexShader: POINT_VERTEX, fragmentShader: POINT_FRAGMENT,
                transparent: true, depthWrite: false, blending: THREE.AdditiveBlending, fog: false
            }));
            const object = new THREE.Points(geometry, material);
            object.frustumCulled = false;
            object.renderOrder = 6;
            const buffer = new THREE.Vector2();
            // pixels per world unit at distance 1: half the render height times the projection's y scale
            object.onBeforeRender = (renderer, _scene, camera) => {
                const target = renderer.getRenderTarget();
                const h = target ? target.height : renderer.getDrawingBufferSize(buffer).y;
                material.uniforms.uScale.value = h * .5 * camera.projectionMatrix.elements[5];
            };
            parent.add(object);
            return { object, position, alpha, size: n };
        }
        // First solid, non-vegetation surface at a cell column (world y of its top face).
        function groundY(cx, cz) {
            const M = window.AuraVoxelIsland ? window.AuraVoxelIsland.MATERIAL : null;
            const plane = island.w * island.d;
            for (let y = island.h - 1; y >= 0; y--) {
                const m = island.voxels[cx + cz * island.w + y * plane];
                if (!m || (M && (m === M.WATER || m === M.OAK_LEAVES || m === M.PINE_LEAVES || m === M.BIRCH_LEAVES || m === M.OAK_LOG || m === M.BIRCH_LOG))) continue;
                return y - island.yBase + 1;
            }
            return features.waterLevel + 1;
        }

        // ---- 1. Cloud sea ----------------------------------------------------------------------
        // The surface height at distance r from the island's axis (a bowl that clears the island's underside and rises to the level
        // where the waterfall lands) and its slope.
        function bowl(r) { return SEA.top + SEA.rise * smooth(SEA.bowl[0], SEA.bowl[1], r); }
        function bowlSlope(r) {
            const width = SEA.bowl[1] - SEA.bowl[0];
            const t = clamp((r - SEA.bowl[0]) / width, 0, 1);
            return SEA.rise * 6 * t * (1 - t) / width;
        }
        // Half width of the lattice at a depth: the view cone of wide pictures, never less than the narrow ones need.
        function seaSpan(depth) { return clamp(SEA.span.slope * (SEA.eye + depth) + SEA.span.base, SEA.span.min, SEA.span.max); }

        // Basic material (no scene lights) that shades by face: the top of a box gets the top colour, its sides the shadow colour (lighter
        // towards the top of the box and towards the key light); the floor mesh (normal.y = SEA.floor.normalY) mixes the two. The result
        // blends into the horizon colour with the distance behind the island (uniforms.uHaze is the sky colour at the horizon row), so the
        // far edge of the sea melts into the sky. Mixed before tone mapping, like the sky itself.
        const seaLook = { uTop: { value: new THREE.Color() }, uSide: { value: new THREE.Color() }, uKey: { value: new THREE.Vector3(0, 1, 0) } };
        function seaMaterial(floor) {
            const material = own(new THREE.MeshBasicMaterial({ color: 0xffffff, fog: false, vertexColors: floor, side: floor ? THREE.DoubleSide : THREE.FrontSide }));
            const range = new THREE.Vector4(fh[0], fh[2], SEA.haze[0], SEA.haze[1]);
            const low = SEA.look.sideLow;
            material.onBeforeCompile = shader => {
                Object.assign(shader.uniforms, seaLook, { uHaze: uniforms.uHaze, uHazeRange: { value: range } });
                shader.vertexShader = 'uniform vec4 uHazeRange;\nuniform vec3 uKey;\nvarying float vHaze;\nvarying float vTop;\nvarying float vShade;\n' +
                    shader.vertexShader.replace('#include <begin_vertex>', `#include <begin_vertex>
	vec4 cw = vec4(transformed, 1.0);
	vShade = 1.0;
	#ifdef USE_INSTANCING
	cw = instanceMatrix * cw;
	vShade = (${low.toFixed(3)} + ${(1 - low).toFixed(3)} * smoothstep(-0.5, 0.5, position.y)) * (1.0 + ${SEA.look.key.toFixed(3)} * dot(normal, uKey));
	#endif
	cw = modelMatrix * cw;
	vHaze = smoothstep(uHazeRange.z, uHazeRange.w, dot(cw.xz, uHazeRange.xy));
	vTop = clamp(normal.y, 0.0, 1.0);`);
                shader.fragmentShader = 'uniform vec3 uHaze;\nuniform vec3 uTop;\nuniform vec3 uSide;\nvarying float vHaze;\nvarying float vTop;\nvarying float vShade;\n' +
                    shader.fragmentShader.replace('#include <opaque_fragment>', 'outgoingLight = mix(mix(uSide * vShade, uTop, vTop) * diffuseColor.rgb, uHaze, vHaze);\n\t#include <opaque_fragment>');
            };
            material.customProgramCacheKey = () => 'voxel-life-sea';
            return material;
        }

        const STRIDE = 11;
        const sea = { n: 0, bases: 0, data: null, mesh: null, floor: null, material: null, floorMaterial: null };
        // Appends one puff at a lattice site to the box lists (core: the bottom tier, detail: what stands on it). A box is: site
        // lateral, site depth, offset across and along the view, bottom above the puff's base, size x/y/z, the row's half span, how far
        // its foot reaches from the site, shade. The bottom tier's boxes include the part of them sunk into the floor.
        function addPuff(srng, lat, depth, span, core, detail) {
            const P = SEA.puff;
            const pick = range => range[0] + (range[1] - range[0]) * srng();
            const w0 = pick(P.w);
            const d0 = w0 * pick(P.aspect);
            const roll = srng();
            const tiers = roll < P.tiers[0] ? 1 : roll < P.tiers[1] ? 2 : 3;
            const sink = SEA.floor.sink;
            // a box is at least 45 % as high as it is wide (the part sunk into the floor does not count)
            const tall = (sx, sz, h, sunk) => Math.max(h, .46 * Math.max(sx, sz) + (sunk || 0));
            const put = (list, dl, dd, y0, sx, sy, sz, foot, shade) => list.push([lat, depth, dl, dd, y0, sx, sy, sz, span, foot, shade]);
            const h0 = tall(w0, d0, Math.max(w0, d0) * pick(P.h) + sink + srng() * P.bump, sink);
            put(core, 0, 0, 0, w0, h0, d0, Math.max(w0, d0) / 2, pick(P.shade.low));
            const ring = 3 + (srng() < .5 ? 1 : 0);
            const a0 = srng() * 6.2832;
            for (let k = 0; k < ring; k++) {
                const a = a0 + (k + (srng() - .5) * .4) * 6.2832 / ring;
                const wk = w0 * pick(P.ring);
                const dk = wk * pick(P.aspect);
                const hk = tall(wk, dk, h0 * pick([.55, .92]), sink);
                const dl = Math.cos(a) * (w0 + wk) * .38;
                const dd = Math.sin(a) * (d0 + dk) * .38;
                put(core, dl, dd, 0, wk, hk, dk, Math.max(Math.abs(dl), Math.abs(dd)) + Math.max(wk, dk) / 2, pick(P.shade.low));
                if (tiers === 2 && ring === 3 && k === 0 && srng() < .6) put(detail, dl, dd, hk, wk * .62, tall(wk * .62, dk * .62, hk * .55), dk * .62, 0, pick(P.shade.high));
            }
            if (tiers > 1) {
                const wt = w0 * pick([.55, .75]);
                const dt = wt * pick(P.aspect);
                const ht = tall(wt, dt, h0 * pick([.4, .55]));
                const ol = (srng() - .5) * .5 * w0;
                const od = (srng() - .5) * .5 * d0;
                put(detail, ol, od, h0, wt, ht, dt, 0, pick(P.shade.high));
                if (tiers > 2) {
                    const wc = wt * pick([.55, .7]);
                    const dc = wc * pick(P.aspect);
                    put(detail, ol * 1.2, od * 1.2, h0 + ht, wc, tall(wc, dc, ht * .5), dc, 0, pick(P.shade.high));
                }
            }
        }

        function buildSea() {
            for (let i = 0; i < SEA_LEGACY_DRAWS; i++) rng();
            const srng = mulberry32(0x5ea1c);
            const core = [];
            const detail = [];
            let puffs = 0;
            for (let row = SEA.near; row <= SEA.far + 8; row += SEA.pitch[1]) {
                const span = seaSpan(row);
                const cols = Math.max(2, Math.round(2 * span / SEA.pitch[0]));
                for (let c = 0; c < cols; c++) {
                    if (srng() < SEA.skip) continue;
                    const lat = -span + (c + .5 + (srng() - .5) * 2 * SEA.spread) * 2 * span / cols;
                    addPuff(srng, lat, row + (srng() - .5) * 2 * SEA.spread * SEA.pitch[1], span, core, detail);
                    puffs++;
                }
            }
            const all = core.concat(detail);
            sea.n = all.length;
            sea.bases = core.length;
            sea.data = new Float32Array(all.length * STRIDE);
            sea.material = seaMaterial(false);
            sea.mesh = instanced(sea.material, all.length, root);
            const color = new THREE.Color();
            all.forEach((row, i) => {
                sea.data.set(row, i * STRIDE);
                sea.mesh.setColorAt(i, color.setRGB(row[10], row[10], row[10], THREE.LinearSRGBColorSpace));
            });
            sea.mesh.instanceColor.needsUpdate = true;
            counts.seaPuffs = puffs;
            buildSeaFloor();
        }

        // The floor: one smooth mesh over the lattice's trapezoid that follows the bowl, with a flat ledge where the waterfall lands.
        function buildSeaFloor() {
            const F = SEA.floor;
            const verts = (F.rows + 1) * (F.cols + 1);
            const position = new Float32Array(verts * 3);
            const normal = new Float32Array(verts * 3);
            const color = new Float32Array(verts * 3);
            const index = [];
            let v = 0;
            for (let j = 0; j <= F.rows; j++) {
                const depth = lerp(SEA.near - 8, SEA.far + 14, j / F.rows);
                const span = seaSpan(depth) + 10;
                for (let i = 0; i <= F.cols; i++, v++) {
                    const lat = lerp(-span, span, i / F.cols);
                    const x = lat * right[0] + depth * fh[0];
                    const z = lat * right[2] + depth * fh[2];
                    const ledge = 1 - smooth(SEA.pad[0], SEA.pad[1], Math.hypot(x - landing.x, z - landing.z));
                    const rolling = F.wobble * Math.sin(lat * .11 + depth * .07) * Math.cos(depth * .09 - lat * .05);
                    position[v * 3] = x;
                    position[v * 3 + 1] = lerp(bowl(Math.hypot(x, z)) + F.drop + rolling, seaFloor, ledge);
                    position[v * 3 + 2] = z;
                    normal[v * 3 + 1] = F.normalY;
                    const blotch = lerp(F.shade[0], F.shade[1], .5 + .5 * Math.sin(lat * .08 + 1.1) * Math.sin(depth * .1 - .4));
                    color[v * 3] = blotch * (1 - .3 * ledge);
                    color[v * 3 + 1] = blotch * (1 - .1 * ledge);
                    color[v * 3 + 2] = blotch;
                    if (i < F.cols && j < F.rows) index.push(v, v + 1, v + F.cols + 1, v + 1, v + F.cols + 2, v + F.cols + 1);
                }
            }
            const geometry = own(new THREE.BufferGeometry());
            for (const [name, array] of [['position', position], ['normal', normal], ['color', color]]) geometry.setAttribute(name, new THREE.BufferAttribute(array, 3));
            geometry.setIndex(index);
            sea.floorMaterial = seaMaterial(true);
            sea.floor = mesh(geometry, sea.floorMaterial, root);
            sea.floor.frustumCulled = false;
        }

        function updateSea(t) {
            const array = sea.mesh.instanceMatrix.array;
            const d = sea.data;
            const shift = SEA.speed * t;
            const baseDrop = SEA.floor.drop - SEA.floor.sink;
            for (let i = 0; i < sea.n; i++) {
                const o = i * STRIDE;
                const span = d[o + 8];
                const lat = wrap(d[o] + shift, span);
                const depth = d[o + 1];
                const x = lat * right[0] + depth * fh[0];
                const z = lat * right[2] + depth * fh[2];
                const r = Math.hypot(x, z);
                const slope = bowlSlope(r);
                // dissolves at the lattice's side and far edges, on the bowl's steep wall and around the waterfall's landing
                const s = (1 - smooth(span - SEA.fadeBand, span, Math.abs(lat))) * (1 - smooth(SEA.far - 10, SEA.far + 8, depth)) *
                    (1 - smooth(SEA.slopeFade[0], SEA.slopeFade[1], slope)) * smooth(SEA.clear[0], SEA.clear[1], Math.hypot(x - landing.x, z - landing.z));
                const foot = slope * d[o + 9] * s;      // the bottom tier reaches down to the floor on the low side of a slope
                const sy = d[o + 6] * s;
                const dl = d[o + 2] * s;
                const dd = d[o + 3] * s;
                putBox(array, i, x + dl * right[0] + dd * fh[0], bowl(r) + baseDrop + d[o + 4] * s + (sy - foot) / 2, z + dl * right[2] + dd * fh[2], d[o + 5] * s, sy + foot, d[o + 7] * s);
            }
            sea.mesh.instanceMatrix.needsUpdate = true;
        }

        // ---- 2. Sky clouds ---------------------------------------------------------------------
        const skyCloud = { n: 0, data: null, mesh: null, material: null, cluster: [] };
        function buildSkyClouds() {
            const rows = [];
            SKY_CLOUDS.forEach((c, k) => {
                for (let i = 0; i < c.n; i++) {
                    const u = (i + .5) / c.n * 2 - 1;
                    const hf = 1 - u * u;
                    const layer = i % 3 === 2 ? 1 : 0;
                    const h = (2.2 + 3 * hf) * (layer ? .8 : 1) * (.85 + .3 * rng());
                    const w = c.w * (.18 + rng() * .1) * (.6 + .5 * hf);
                    const dpt = 5 + rng() * 4;
                    rows.push([k, u * c.w / 2 + (rng() - .5) * c.w * .12, layer * 2.4 + h / 2, (rng() - .5) * dpt * .6, w, h, dpt, .94 + rng() * .08]);
                }
            });
            skyCloud.n = rows.length;
            skyCloud.data = new Float32Array(rows.length * 8);
            rows.forEach((row, i) => skyCloud.data.set(row, i * 8));
            skyCloud.material = lambert('#ffffff', { fog: true });
            skyCloud.mesh = instanced(skyCloud.material, rows.length, root);
            const color = new THREE.Color();
            rows.forEach((row, i) => skyCloud.mesh.setColorAt(i, color.setRGB(row[7], row[7], row[7], THREE.LinearSRGBColorSpace)));
            skyCloud.mesh.instanceColor.needsUpdate = true;
            // a cluster's wrap-around half width: a bit more than the view at its depth
            skyCloud.span = SKY_CLOUDS.map(c => c.d * tanHalf * 1.8 * 1.3 + c.w);
        }

        function updateSkyClouds(t) {
            const array = skyCloud.mesh.instanceMatrix.array;
            const d = skyCloud.data;
            const centers = SKY_CLOUDS.map((c, k) => {
                const lat = wrap(c.lat + DRIFT_SPEED * t, skyCloud.span[k]);
                const fade = 1 - smooth(skyCloud.span[k] - c.w * 1.4, skyCloud.span[k], Math.abs(lat));
                return { p: framePoint(c.d, c.q, lat), fade };
            });
            for (let i = 0; i < skyCloud.n; i++) {
                const o = i * 8;
                const c = centers[d[o]];
                const dl = d[o + 1];
                const dd = d[o + 3];
                putBox(array, i, c.p[0] + right[0] * dl + fh[0] * dd, c.p[1] + d[o + 2], c.p[2] + right[2] * dl + fh[2] * dd, d[o + 4] * c.fade, d[o + 5] * c.fade, d[o + 6] * c.fade);
            }
            skyCloud.mesh.instanceMatrix.needsUpdate = true;
            // the terrain's cloud shadows drift the same way (the noise coordinate is world position * scale + offset)
            uniforms.uCloudOffset.value.set(-CLOUD_SHADOW.scale * DRIFT_SPEED * t * right[0], -CLOUD_SHADOW.scale * DRIFT_SPEED * t * right[2]);
        }

        function updateClouds(t, sky) {
            const tint = skyNow.cloudTint;
            const horizon = skyNow.skyHorizon;
            const look = SEA.look;
            // tops: the tint; sides: the tint towards the sky's horizon colour, a little darker (sky-tinted shadow, never neutral grey)
            seaLook.uTop.value.setRGB(tint[0], tint[1], tint[2], THREE.SRGBColorSpace).multiplyScalar(look.topGain);
            seaLook.uSide.value.setRGB(...tint.map((c, i) => (c + ((horizon[i] + skyNow.skyTop[i]) / 2 - c) * look.shadowMix) * look.shadowGain), THREE.SRGBColorSpace);
            seaLook.uKey.value.fromArray(skyNow.keyDir);
            skyCloud.material.color.setRGB(tint[0], tint[1], tint[2], THREE.SRGBColorSpace).multiplyScalar(SKY_CLOUD_LOOK.gain);
            skyCloud.material.emissive.setRGB(tint[0], tint[1], tint[2], THREE.SRGBColorSpace).multiplyScalar(SKY_CLOUD_LOOK.glow);
            updateSea(t);
            updateSkyClouds(t);
            uniforms.uCloudShadow.value = CLOUD_SHADOW.strength * (1 - sky.night);
        }

        // ---- 3. Waterfall ----------------------------------------------------------------------
        const fall = { sheet: null, material: null, spray: null, sprayN: 0, mist: null, mistN: 0, mistMaterial: null, foam: null, foamMaterial: null, foamSeed: [], rainbow: null, rainbowMaterial: null };
        const wf = features.waterfall;
        const lip = { x: wf.x - ox, z0: wf.z - oz, z1: wf.z + wf.width - oz, top: wf.y + 1 };
        const seaFloor = bowl(Math.hypot(lip.x + 2.6, (lip.z0 + lip.z1) / 2));   // where the sheet lands
        const landing = { x: 0, z: 0 };                                          // the centre of the foam, on the sea ledge
        let fallDrop = 0;
        let fallTime = 0;
        function fallX(drop) { return lip.x + FALL.vx * Math.sqrt(2 * Math.max(drop, 0) / FALL.gravity); }

        function buildFall() {
            const bottom = seaFloor - FALL.sink;
            fallDrop = lip.top - bottom;
            fallTime = Math.sqrt(2 * fallDrop / FALL.gravity);
            landing.x = fallX(fallDrop) + .3;
            landing.z = (lip.z0 + lip.z1) / 2;
            const rows = FALL.segments;
            const positions = [];
            const uvs = [];
            const indices = [];
            let arc = 0;
            let prev = null;
            const rowStart = [];
            for (let k = 0; k <= rows; k++) {
                const u = k / rows;
                const y = lip.top - fallDrop * u;
                const x = fallX(lip.top - y);
                if (prev) arc += Math.hypot(x - prev[0], y - prev[1]);
                prev = [x, y];
                const half = lerp(FALL.thickTop, FALL.thickBottom, u) / 2;
                const base = positions.length / 3;
                positions.push(x + half, y, lip.z0, x + half, y, (lip.z0 + lip.z1) / 2, x + half, y, lip.z1, x - half, y, lip.z0, x - half, y, lip.z1);
                uvs.push(0, arc, .5, arc, 1, arc, .02, arc, .98, arc);
                rowStart.push(base);
            }
            fall.length = arc;
            for (let k = 0; k < rows; k++) {
                const a = rowStart[k];
                const b = rowStart[k + 1];
                // front: two quads across; sides: the outer edge to the back edge on each side
                indices.push(a, b, a + 1, a + 1, b, b + 1, a + 1, b + 1, a + 2, a + 2, b + 1, b + 2);
                indices.push(a, a + 3, b, b, a + 3, b + 3, a + 2, b + 2, a + 4, a + 4, b + 2, b + 4);
            }
            const geometry = own(new THREE.BufferGeometry());
            geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
            geometry.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
            geometry.setIndex(indices);
            const fogUniforms = THREE.UniformsUtils.clone(THREE.UniformsLib.fog);
            const u = {
                uTime: uniforms.uTime, uLen: { value: arc }, uDeep: { value: colorOf(FALL.deep) }, uShallow: { value: colorOf(FALL.shallow) },
                uFoam: { value: colorOf(FALL.white) }, uLight: { value: new THREE.Color(1, 1, 1) }
            };
            fall.material = own(new THREE.ShaderMaterial({
                uniforms: Object.assign(fogUniforms, u), vertexShader: FALL_VERTEX, fragmentShader: FALL_FRAGMENT, side: THREE.DoubleSide, transparent: true, fog: true
            }));
            fall.sheet = mesh(geometry, fall.material);
            fall.sheet.frustumCulled = false;

            // spray cubes leave the lip with the sheet's velocity and fall with it
            fall.sprayN = FALL.spray;
            fall.spray = instanced(own(new THREE.MeshLambertMaterial({ color: 0xffffff, emissive: colorOf('#2a4a66') })), FALL.spray, dyn);
            const mist = fadeBoxes(fadeMaterial(new THREE.Color(1, 1, 1), true), FALL.mist);
            fall.mist = mist.mesh;
            fall.mistFade = mist.fade;
            fall.mistMaterial = mist.mesh.material;
            fall.mistN = FALL.mist;

            // foam: opaque white cubes churning on the ledge where the sheet lands (hashed, so the shared random stream is left alone)
            fall.foamMaterial = own(new THREE.MeshLambertMaterial({ color: 0xffffff }));
            fall.foam = instanced(fall.foamMaterial, FALL.foam, dyn);
            for (let i = 0; i < FALL.foam; i++) {
                const h = k => hash1(i * 4.13 + k * 17.9);
                fall.foamSeed.push({
                    a: h(1) * 6.2832, rx: lerp(FALL.foamRx[0], FALL.foamRx[1], h(2)), rz: lerp(FALL.foamRz[0], FALL.foamRz[1], h(3)),
                    phase: h(4) * 6.2832, rate: 1.6 + h(5) * 2.2, size: lerp(FALL.foamSize[0], FALL.foamSize[1], h(6)), spin: (h(7) - .5) * .7
                });
            }

            // the rainbow: a camera-facing quad in the mist
            const rb = FALL.rainbow;
            fall.rainbowMaterial = own(new THREE.ShaderMaterial({
                uniforms: { uAlpha: { value: 0 }, uSize: { value: new THREE.Vector2(rb.width, rb.height) }, uRadius: { value: new THREE.Vector2(rb.inner, rb.outer) } },
                vertexShader: 'varying vec2 vUv;\nvoid main() {\n    vUv = uv;\n    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);\n}',
                fragmentShader: RAINBOW_FRAGMENT, transparent: true, depthWrite: false, fog: false, side: THREE.DoubleSide
            }));
            const quad = own(new THREE.PlaneGeometry(rb.width, rb.height));
            quad.translate(0, rb.height / 2, 0);
            fall.rainbow = mesh(quad, fall.rainbowMaterial);
            fall.rainbow.renderOrder = 4;
            fall.rainbow.frustumCulled = false;
            fall.rainbow.position.set(fallX(fallDrop) + 3, seaFloor - .6, (lip.z0 + lip.z1) / 2);
            fall.rainbow.onBeforeRender = (_renderer, _scene, camera) => {
                fall.rainbow.quaternion.copy(camera.quaternion);
                fall.rainbow.updateMatrix();
                fall.rainbow.updateMatrixWorld(true);
            };
        }

        function updateFall(t, sky) {
            const night = sky.night;
            // light on the sheet: warm daylight, cool moonlight, never black
            const key = sky.keyColor;
            const day = 1 - night;
            fall.material.uniforms.uLight.value.setRGB(
                lerp(.3, .55 + .45 * lerp(1, key[0], .5), day), lerp(.42, .55 + .45 * lerp(1, key[1], .5), day), lerp(.68, .55 + .45 * lerp(1, key[2], .5), day), THREE.LinearSRGBColorSpace);
            const count = Math.round(FALL.spray * (scale >= 1 ? 1 : QUALITY_HALF.spray));
            fall.spray.count = count;
            counts.spray = count;
            const cycle = fallTime + .35;
            const array = fall.spray.instanceMatrix.array;
            const colors = fall.spray.instanceColor.array;
            const from = hexRGB(FALL.sprayFrom).map(c => c * c);     // ~linear
            const lowLight = lerp(.45, 1, day);
            const width = lip.z1 - lip.z0;
            for (let i = 0; i < count; i++) {
                const clock = t + i / count * cycle;
                const n = Math.floor(clock / cycle);
                const age = clock - n * cycle;
                const r1 = hash1(i * 7.1 + n * 3.7);
                const r2 = hash1(i * 3.3 + n * 9.1 + 1);
                const r3 = hash1(i * 5.9 + n * 1.3 + 2);
                const size = lerp(FALL.sprayMin, FALL.sprayMax, r3);
                const vx = FALL.vx * (.8 + .6 * r1);
                const drop = .5 * FALL.gravity * age * age;
                const y = lip.top - .3 - r2 * .3 - drop;
                const alive = y > seaFloor - .5 && age < fallTime ? 1 : 0;
                const shrink = alive * smooth(0, .25, age) * (1 - smooth(fallTime - .5, fallTime, age) * .5);
                putBox(array, i, lip.x + vx * age + .4, y, lip.z0 + .15 + (width - .3) * r1 + (r2 - .5) * .6 * smooth(0, 1, age), size * shrink, size * shrink, size * shrink);
                const w = smooth(.35, 1, age / fallTime);
                colors[i * 3] = lerp(from[0], 1, w) * lowLight;
                colors[i * 3 + 1] = lerp(from[1], 1, w) * lowLight;
                colors[i * 3 + 2] = lerp(from[2], 1, w) * lowLight;
            }
            fall.spray.instanceMatrix.needsUpdate = true;
            fall.spray.instanceColor.needsUpdate = true;
            // foam: the cubes bob and pulse in a ring around the landing; the nearer its middle, the higher they churn
            const foamCount = Math.round(FALL.foam * (scale >= 1 ? 1 : QUALITY_HALF.foam));
            fall.foam.count = foamCount;
            counts.foam = foamCount;
            const farray = fall.foam.instanceMatrix.array;
            for (let i = 0; i < foamCount; i++) {
                const f = fall.foamSeed[i];
                const a = f.a + t * f.spin;
                const s = f.size * (.78 + .22 * Math.sin(t * f.rate * 1.7 + f.phase));
                const y = seaFloor + .1 + (.5 + .5 * Math.sin(t * f.rate + f.phase)) * FALL.foamLift * (1 - .6 * f.rx / FALL.foamRx[1]);
                putBox(farray, i, landing.x + Math.cos(a) * f.rx, y + s * .5, landing.z + Math.sin(a) * f.rz, s, s, s);
            }
            fall.foam.instanceMatrix.needsUpdate = true;
            fall.foamMaterial.emissive.setRGB(lerp(.08, .16, day), lerp(.1, .2, day), lerp(.2, .26, day), THREE.SRGBColorSpace);
            // mist: soft cubes rise from the foam, grow, pulse and fade over mistLife
            const mistCount = Math.round(FALL.mist * (scale >= 1 ? 1 : QUALITY_HALF.mist));
            fall.mist.count = mistCount;
            counts.mist = mistCount;
            const marray = fall.mist.instanceMatrix.array;
            for (let i = 0; i < mistCount; i++) {
                const phase = t / FALL.mistLife + i / mistCount;
                const life = phase % 1;
                const n = Math.floor(phase);
                const r1 = hash1(i * 2.3 + n * 5.1);
                const r2 = hash1(i * 6.7 + n * 1.9 + 4);
                const size = lerp(FALL.mistFrom, FALL.mistTo, smooth(0, 1, life)) * (.75 + .5 * r1) * (1 + .1 * Math.sin(t * 3 + i * 1.7));
                putBox(marray, i, landing.x + (r1 - .4) * 4 + life, seaFloor + .2 + life * FALL.mistRise + size * .5, landing.z + (r2 - .5) * 3.8, size, size, size);
                fall.mistFade.array[i] = FALL.mistAlpha * Math.sin(Math.PI * Math.min(life * 1.1, 1)) * (1 - life * .4);
            }
            fall.mist.instanceMatrix.needsUpdate = true;
            fall.mistFade.needsUpdate = true;
            fall.mistMaterial.color.setRGB(lerp(.5, .82, day), lerp(.62, .93, day), lerp(.85, 1, day), THREE.SRGBColorSpace);
            fall.rainbowMaterial.uniforms.uAlpha.value = FALL.rainbow.alpha * sky.rainbow;
            fall.rainbow.visible = sky.rainbow > .01;
        }

        // ---- 4. Windmill sails -----------------------------------------------------------------
        const sails = { group: null };
        function buildWindmill() {
            const hub = features.windmill.hub;
            const [hx, hz] = toWorld(hub[0], hub[2]);
            const group = new THREE.Group();
            group.position.set(hx, hub[1] + .5, hz - .5 + SAILS.hubOut + .15);
            dyn.add(group);
            const wood = lambert(SAILS.spar_color);
            const cloth = lambert(SAILS.cloth_color, { side: THREE.DoubleSide });
            const hubMesh = mesh(unitBox, wood, group);
            hubMesh.scale.set(.8, .8, .8);
            const shaft = mesh(unitBox, wood, dyn);
            shaft.scale.set(.4, .4, SAILS.hubOut + .5);
            shaft.position.set(hx, hub[1] + .5, hz - .5 + (SAILS.hubOut + .5) / 2 - .15);
            const spin = new THREE.Group();
            group.add(spin);
            for (let k = 0; k < 4; k++) {
                const arm = new THREE.Group();
                arm.rotation.z = k * Math.PI / 2;
                const spar = mesh(unitBox, wood, arm);
                spar.scale.set(SAILS.spar[0], SAILS.spar[1], SAILS.spar[2]);
                spar.position.set(0, .4 + SAILS.spar[1] / 2, 0);
                const sheet = mesh(unitBox, cloth, arm);
                sheet.scale.set(SAILS.cloth[0], SAILS.cloth[1], SAILS.cloth[2]);
                sheet.position.set(SAILS.clothOffset, 1.2 + SAILS.cloth[1] / 2, 0);
                spin.add(arm);
            }
            sails.group = spin;
        }

        // ---- 5. Chimney smoke ------------------------------------------------------------------
        const smoke = { mesh: null, fade: null, material: null, n: 0, per: SMOKE.perChimney };
        function buildSmoke() {
            const chimneys = features.chimneys || [];
            if (!chimneys.length) return;
            smoke.n = chimneys.length * SMOKE.perChimney;
            smoke.material = fadeMaterial(new THREE.Color(1, 1, 1));
            const parts = fadeBoxes(smoke.material, smoke.n);
            smoke.mesh = parts.mesh;
            smoke.fade = parts.fade;
        }

        function updateSmoke(t, sky) {
            if (!smoke.mesh) return;
            const chimneys = features.chimneys;
            const per = Math.max(2, Math.round(SMOKE.perChimney * (scale >= 1 ? 1 : QUALITY_HALF.smoke)));
            const array = smoke.mesh.instanceMatrix.array;
            const fog = sky.fogColor;
            smoke.material.color.setRGB(lerp(.5, fog[0], .7), lerp(.5, fog[1], .7), lerp(.52, fog[2], .7), THREE.SRGBColorSpace);
            let n = 0;
            chimneys.forEach(c => {
                const [x, z] = toWorld(c[0], c[2]);
                for (let k = 0; k < per; k++, n++) {
                    const age = ((t / SMOKE.life + k / per) % 1);
                    const size = lerp(SMOKE.from, SMOKE.to, age);
                    const idx = n;
                    putBox(array, idx, x + SMOKE.drift * age * SMOKE.life + Math.sin(age * 6 + k) * .12, c[1] + .3 + SMOKE.rise * age * SMOKE.life, z + Math.cos(age * 5 + k * 2) * .1, size, size, size);
                    smoke.fade.array[idx] = SMOKE.alpha * (1 - age) * smooth(0, .08, age);
                }
            });
            smoke.mesh.count = n;
            counts.smoke = n;
            smoke.mesh.instanceMatrix.needsUpdate = true;
            smoke.fade.needsUpdate = true;
        }

        // ---- 6. Birds --------------------------------------------------------------------------
        const birds = [];
        function buildBirds() {
            const material = lambert(BIRDS.color);
            const wingMaterial = lambert(BIRDS.wing);
            const wingGeometry = unitBox;
            for (let k = 0; k < BIRDS.count; k++) {
                const bird = new THREE.Group();
                bird.rotation.order = 'YXZ';
                const body = mesh(unitBox, material, bird);
                body.scale.set(.5, .35, 1);
                const wings = [1, -1].map(side => {
                    const pivot = new THREE.Group();
                    pivot.position.set(side * .22, .05, .05);
                    const wing = mesh(wingGeometry, wingMaterial, pivot);
                    wing.scale.set(1.1, .12, .5);
                    wing.position.set(side * .55, 0, 0);
                    bird.add(pivot);
                    return pivot;
                });
                dyn.add(bird);
                birds.push({ group: bird, wings, phase: k * 1.7 });
            }
        }

        function updateBirds(t, sky) {
            const count = sky.night > .55 ? 0 : Math.round(BIRDS.count * (scale >= 1 ? 1 : QUALITY_HALF.birds));   // the flock roosts at night
            counts.birds = count;
            const theta = 2 * Math.PI * t / BIRDS.period;
            const fx = -BIRDS.a * Math.sin(theta);
            const fz = BIRDS.b * Math.cos(theta);
            const len = Math.hypot(fx, fz) || 1;
            const dx = fx / len;
            const dz = fz / len;
            birds.forEach((bird, k) => {
                bird.group.visible = k < count;
                if (k >= count) return;
                const rank = Math.ceil(k / 2);
                const side = k === 0 ? 0 : (k % 2 ? -1 : 1);
                const lead = k === 0 ? 0 : rank * BIRDS.back;
                const lat = side * rank * BIRDS.side;
                const x = BIRDS.a * Math.cos(theta) - dx * lead - dz * lat;
                const z = BIRDS.b * Math.sin(theta) - dz * lead + dx * lat;
                const y = lerp(BIRDS.yLow, BIRDS.yHigh, .5 + .5 * Math.sin(2 * Math.PI * t / 13 + bird.phase)) + .4 * Math.sin(t * 1.3 + k);
                bird.group.position.set(x, y, z);
                bird.group.rotation.y = Math.atan2(dx, dz);
                bird.group.rotation.z = BIRDS.bank;
                bird.group.rotation.x = -.06 * Math.cos(t * BIRDS.flapHz * 2 * Math.PI + bird.phase);
                // flapping in bursts with glide pauses in between
                const burst = smooth(.15, .45, Math.sin(2 * Math.PI * t / 6.5 + bird.phase * .6));
                const angle = lerp(BIRDS.glide, BIRDS.flap * Math.sin(2 * Math.PI * BIRDS.flapHz * t + bird.phase), burst) * rad;
                bird.wings[0].rotation.z = angle;
                bird.wings[1].rotation.z = -angle;
            });
        }

        // ---- 7. Fireflies and 8. glow halos ------------------------------------------------------
        const flies = { homes: [], cubes: null, sprites: null, material: null };
        function buildFireflies() {
            const m = features.meadow;
            const lake = features.lake;
            for (let i = 0; i < FIREFLIES.count; i++) {
                if (m && i % 3 !== 2) {
                    const cx = Math.floor(lerp(m.x0, m.x1, rng()));
                    const cz = Math.floor(lerp(m.z0, m.z1, rng()));
                    const [x, z] = toWorld(cx, cz);
                    flies.homes.push({ x, z, y: groundY(cx, cz) + 1.1, reach: 2.4, lift: 1.1 });
                } else {
                    const [x, z] = toWorld(lake.x, lake.z);
                    const a = rng() * Math.PI * 2;
                    flies.homes.push({ x: x + Math.cos(a) * lake.r * .6, z: z + Math.sin(a) * lake.r * .6, y: features.waterLevel + 1.7, reach: 1.6, lift: .8 });
                }
            }
            flies.phase = flies.homes.map(() => [rng() * 40, rng() * 40, rng() * 40, .5 + rng() * .8, rng() * 6.28]);
            flies.material = own(new THREE.MeshBasicMaterial({ color: colorOf(FIREFLIES.color), fog: false }));
            flies.cubes = instanced(flies.material, FIREFLIES.count, dyn);
            const points = flies.homes.map(h => ({ x: h.x, y: h.y, z: h.z, size: FIREFLIES.glow, color: FIREFLIES.color }));
            flies.sprites = glowPoints(points, dyn, .3);
        }

        function updateFireflies(t, sky) {
            const visible = smooth(.3, .7, sky.night);
            const count = Math.round(FIREFLIES.count * (scale >= 1 ? 1 : QUALITY_HALF.fireflies));
            counts.fireflies = visible > 0 ? count : 0;
            flies.cubes.visible = flies.sprites.object.visible = visible > 0;
            if (visible <= 0) return;
            flies.cubes.count = count;
            const array = flies.cubes.instanceMatrix.array;
            const pos = flies.sprites.position;
            const alpha = flies.sprites.alpha;
            for (let i = 0; i < count; i++) {
                const h = flies.homes[i];
                const p = flies.phase[i];
                const x = h.x + Math.sin(t * .31 * p[3] + p[0]) * h.reach + Math.sin(t * .83 + p[1]) * h.reach * .35;
                const z = h.z + Math.cos(t * .27 * p[3] + p[1]) * h.reach + Math.cos(t * .71 + p[2]) * h.reach * .35;
                const y = h.y + Math.sin(t * .6 * p[3] + p[2]) * h.lift * .5 + h.lift * .3;
                const blink = smooth(.2, .75, Math.sin(t * (1.1 + p[3]) * 2 + p[4]) * .5 + .5);
                const s = FIREFLIES.size * (.35 + .65 * blink) * visible;
                putBox(array, i, x, y, z, s, s, s);
                pos.setXYZ(i, x, y, z);
                alpha.setX(i, blink * visible * HALO.gain);
            }
            for (let i = count; i < FIREFLIES.count; i++) alpha.setX(i, 0);
            flies.cubes.instanceMatrix.needsUpdate = true;
            pos.needsUpdate = true;
            alpha.needsUpdate = true;
        }

        const halos = { main: null, islet: null, kinds: [] };
        function buildHalos() {
            const list = [];
            (features.lanterns || []).forEach(l => { const [x, z] = toWorld(l[0], l[2]); list.push({ x, y: l[1], z, size: HALO.lantern, color: HALO.lanternColor, flicker: 7 }); });
            (features.windows || []).forEach(w => { const [x, z] = toWorld(w[0], w[2]); list.push({ x, y: w[1] + .5, z, size: HALO.window, color: HALO.windowColor, flicker: 3 }); });
            (features.crystals || []).forEach(c => { const [x, z] = toWorld(c[0], c[2]); list.push({ x, y: c[1] + .5, z, size: HALO.crystal, color: HALO.crystalColor, flicker: 2 }); });
            halos.kinds = list;
            halos.main = glowPoints(list, dyn, HALO.pull);
            // crystals on an islet ride along with it
            const withCrystals = island.islets.map((islet, i) => ({ islet, group: isletGroups[i] })).filter(e => e.group && e.islet.crystals);
            halos.islet = [];
            withCrystals.forEach(e => {
                const pts = e.islet.crystals.map(c => ({ x: c[0] + .5 - e.islet.w / 2, y: c[1] + .5, z: c[2] + .5 - e.islet.d / 2, size: HALO.crystal, color: HALO.crystalColor }));
                halos.islet.push(glowPoints(pts, e.group, HALO.pull));
            });
            counts.halos = list.length + halos.islet.reduce((sum, h) => sum + h.size, 0);
        }

        function updateHalos(t, sky) {
            const night = smooth(0, .5, sky.night);
            halos.main.object.visible = night > 0;
            halos.islet.forEach(h => { h.object.visible = night > 0; });
            if (night <= 0) return;
            halos.kinds.forEach((k, i) => halos.main.alpha.setX(i, night * HALO.gain * (.85 + .15 * Math.sin(t * k.flicker + i * 40))));
            halos.main.alpha.needsUpdate = true;
            halos.islet.forEach(h => {
                for (let i = 0; i < h.size; i++) h.alpha.setX(i, night * HALO.gain * (.85 + .15 * Math.sin(t * 2 + i * 5)));
                h.alpha.needsUpdate = true;
            });
        }

        // ---- 9. Shooting star ------------------------------------------------------------------
        const star = { head: null, tail: null };
        function buildStar() {
            const material = own(new THREE.MeshBasicMaterial({ color: 0xffffff, transparent: true, opacity: 0, fog: false, depthWrite: false }));
            const tailMaterial = own(new THREE.MeshBasicMaterial({ color: 0xb8ccff, transparent: true, opacity: 0, fog: false, depthWrite: false }));
            star.head = mesh(unitBox, material);
            star.tail = mesh(unitBox, tailMaterial);
            star.head.visible = star.tail.visible = false;
        }

        function updateStar(t, sky) {
            const slot = Math.floor(t / SHOOTING.every);
            const start = slot * SHOOTING.every + hash1(slot + .5) * SHOOTING.spread;
            const age = (t - start) / SHOOTING.duration;
            const on = sky.night > .6 && age >= 0 && age <= 1;
            star.head.visible = star.tail.visible = on;
            if (!on) return;
            const flip = hash1(slot + 9.1) > .5 ? 1 : -1;
            const lat0 = flip * (60 + hash1(slot + 3.3) * 90);
            const q0 = SHOOTING.q - hash1(slot + 5.5) * .18;
            const a = framePoint(SHOOTING.d, q0, lat0);
            const b = framePoint(SHOOTING.d, q0 - .14, lat0 - flip * 150);
            const px = lerp(a[0], b[0], age);
            const py = lerp(a[1], b[1], age);
            const pz = lerp(a[2], b[2], age);
            const dir = new THREE.Vector3(b[0] - a[0], b[1] - a[1], b[2] - a[2]).normalize();
            const fadeIn = Math.sin(Math.PI * age);
            star.head.position.set(px, py, pz);
            star.head.quaternion.setFromUnitVectors(new THREE.Vector3(1, 0, 0), dir);
            star.head.scale.set(SHOOTING.length, .25, .25);
            star.head.material.opacity = fadeIn;
            star.tail.position.set(px - dir.x * SHOOTING.length * 1.6, py - dir.y * SHOOTING.length * 1.6, pz - dir.z * SHOOTING.length * 1.6);
            star.tail.quaternion.copy(star.head.quaternion);
            star.tail.scale.set(SHOOTING.length * 2.2, .12, .12);
            star.tail.material.opacity = fadeIn * .45;
        }

        // ---- 10. Islets and 11. boat -----------------------------------------------------------
        const boat = { group: null };
        function buildBoat() {
            const b = features.boat;
            if (!b) return;
            const [x, z] = toWorld(b.x, b.z);
            const group = new THREE.Group();
            group.position.set(x, b.y, z);
            group.rotation.y = b.heading || 0;
            dyn.add(group);
            const hull = mesh(unitBox, lambert(BOAT.color), group);
            hull.scale.set(BOAT.size[0], BOAT.size[1], BOAT.size[2]);
            hull.position.y = BOAT.size[1] / 2 - BOAT.draft;
            const rim = lambert(BOAT.rim);
            const top = BOAT.size[1] - BOAT.draft;
            [[0, -1], [0, 1]].forEach(([, s]) => {
                const rail = mesh(unitBox, rim, group);
                rail.scale.set(BOAT.size[0] + .1, .18, .18);
                rail.position.set(0, top + .05, s * (BOAT.size[2] / 2 - .05));
            });
            [-1, 1].forEach(s => {
                const rail = mesh(unitBox, rim, group);
                rail.scale.set(.18, .18, BOAT.size[2]);
                rail.position.set(s * (BOAT.size[0] / 2 - .05), top + .05, 0);
            });
            const seat = mesh(unitBox, rim, group);
            seat.scale.set(.5, .14, BOAT.size[2] - .3);
            seat.position.set(-.3, top - .25, 0);
            boat.group = group;
            boat.baseY = b.y;
            boat.heading = b.heading || 0;
        }

        function updateMovers(t) {
            island.islets.forEach((islet, i) => {
                const g = isletGroups[i];
                if (!g) return;
                g.position.y = islet.offset[1] + Math.sin(2 * Math.PI * t / ISLET.period + islet.phase) * ISLET.bob;
                g.rotation.y = Math.sin(2 * Math.PI * t / ISLET.turnPeriod + islet.phase * 1.7) * ISLET.turn * rad;
            });
            if (boat.group) {
                boat.group.position.y = boat.baseY + Math.sin(t * 1.4) * BOAT.bob;
                boat.group.rotation.z = Math.sin(t * 1.1 + 1) * BOAT.rock * rad;
                boat.group.rotation.x = Math.sin(t * .9 + 2.4) * BOAT.rock * .5 * rad;
            }
            sails.group.rotation.z = -t * SAILS.turn;
        }

        // ---- Public surface --------------------------------------------------------------------
        buildFall();
        buildSea();
        buildSkyClouds();
        buildWindmill();
        buildSmoke();
        buildBirds();
        buildFireflies();
        buildHalos();
        buildStar();
        buildBoat();
        counts.clouds = sea.n + skyCloud.n;

        function setQuality(next) {
            scale = clamp(Number(next), 0, 1);
            if (!Number.isFinite(scale)) scale = 1;
            dyn.visible = scale > 0;
            sea.mesh.count = scale >= 1 ? sea.n : sea.bases + Math.floor((sea.n - sea.bases) * QUALITY_HALF.seaDetails);
            counts.clouds = sea.mesh.count + skyCloud.n;
        }
        setQuality(scale);

        function update(_dt, t, sky) {
            if (disposed || !sky) return;
            skyNow = sky;
            updateClouds(t, sky);
            if (scale <= 0) return;
            updateFall(t, sky);
            updateSmoke(t, sky);
            updateBirds(t, sky);
            updateFireflies(t, sky);
            updateHalos(t, sky);
            updateStar(t, sky);
            updateMovers(t);
        }

        function inspect() {
            return Object.assign({ visible: scale > 0, scale }, counts);
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            options.scene.remove(root);
            halos.islet.forEach(h => { h.object.parent && h.object.parent.remove(h.object); });
            for (const item of owned) { try { item.dispose(); } catch (_) { /* best effort */ } }
            for (const m of [sea.mesh, skyCloud.mesh, fall.spray, fall.foam, fall.mist, flies.cubes, smoke.mesh]) {
                if (m && typeof m.dispose === 'function') { try { m.dispose(); } catch (_) { /* best effort */ } }
            }
            owned.length = 0;
        }

        return { update, setQuality, inspect, dispose };
    }

    window.AuraVoxelLife = Object.freeze({ create });
})();
