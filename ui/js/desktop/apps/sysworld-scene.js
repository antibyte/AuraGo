(function () {
    'use strict';

    // System World — stage module: renderer, camera, OrbitControls and the
    // environmental dressing (starfield layers, nebula backdrop, energy grid,
    // lights). THREE and THREE.OrbitControls are loaded before this file, so
    // top-level THREE usage is allowed here (and only here).

    const NS = window.SysWorld = window.SysWorld || {};

    // Shared spatial layout constants for every System World module.
    NS.LAYOUT = {
        coreRadius: 6,
        orbitInner: 26,
        orbitOuter: 64,
        graphCenter: new THREE.Vector3(0, 34, -120),
        graphRadius: 55,
        memoryRadius: 15,
        missionRingRadius: 84,
        beltRadius: 130,
        infraY: -34
    };

    // One raycaster reused by every stage instance and every caller.
    const raycaster = new THREE.Raycaster();
    raycaster.params.Points.threshold = 2;
    raycaster.params.Line.threshold = 1;
    const ndcPointer = new THREE.Vector2();

    // Camera framing constants.
    const HOME_POS = new THREE.Vector3(0, 55, 150);
    const HOME_TARGET = new THREE.Vector3(0, 0, 0);
    const START_POS = new THREE.Vector3(-340, 400, 1050);
    const BASE_FOV = 60;
    const INTRO_FOV = 74;

    NS.createStage = function (inst) {
        const THREE = inst.THREE || window.THREE;
        const P = NS.PALETTE || {};
        const L = NS.LAYOUT;
        const host = inst.canvasHost;
        const CAPACITY_SCALE = 1.6; // particle buffers are always ultra-sized; tiers scale live
        const bgHex = P.bg != null ? P.bg : 0x020208;

        // ------------------------------------------------------------------
        // Renderer
        // ------------------------------------------------------------------
        const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false });
        renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
        renderer.setClearColor(bgHex, 1);
        const canvas = renderer.domElement;
        canvas.classList.add('sysworld-gl');
        host.appendChild(canvas);

        // ------------------------------------------------------------------
        // Scene / camera / controls
        // ------------------------------------------------------------------
        const scene = new THREE.Scene();
        scene.background = new THREE.Color(bgHex);
        scene.fog = new THREE.FogExp2(bgHex, 0.0016); // very subtle depth haze

        // fov 60 by default; introFlight() briefly widens it for the swoop.
        // Start position is far away and high for the intro flight.
        const camera = new THREE.PerspectiveCamera(BASE_FOV, 1, 0.1, 2500);
        camera.position.copy(START_POS);

        const controls = new THREE.OrbitControls(camera, canvas);
        controls.enableDamping = true;
        controls.dampingFactor = 0.06;
        controls.minDistance = 8;
        controls.maxDistance = 900;
        controls.target.copy(HOME_TARGET);
        controls.update();

        // ------------------------------------------------------------------
        // One soft radial dot texture per stage, shared by stars and nebulae
        // (allocated once at startup, tinted per material, never per frame)
        // ------------------------------------------------------------------
        const softDotTexture = (function () {
            const s = 64;
            const cv = document.createElement('canvas');
            cv.width = s;
            cv.height = s;
            const c2 = cv.getContext('2d');
            const grad = c2.createRadialGradient(s / 2, s / 2, 0, s / 2, s / 2, s / 2);
            grad.addColorStop(0, 'rgba(255,255,255,1)');
            grad.addColorStop(0.4, 'rgba(255,255,255,0.5)');
            grad.addColorStop(1, 'rgba(255,255,255,0)');
            c2.fillStyle = grad;
            c2.fillRect(0, 0, s, s);
            const tex = new THREE.CanvasTexture(cv);
            tex.needsUpdate = true;
            return tex;
        })();

        // ------------------------------------------------------------------
        // Lights: faint ambient + one central point light at the agent core
        // ------------------------------------------------------------------
        scene.add(new THREE.AmbientLight(0x3a4a66, 0.55));
        const coreLight = new THREE.PointLight(P.core != null ? P.core : 0x59d4ff, 1.25, 340, 2);
        coreLight.position.set(0, 0, 0);
        scene.add(coreLight);

        // ------------------------------------------------------------------
        // Starfield: three point layers with distinct look and slow drift
        // ------------------------------------------------------------------
        const starLayers = [];

        function addStarLayer(count, spread, size, color, opacity, driftY, driftX) {
            const n = Math.max(60, Math.round(count));
            const positions = new Float32Array(n * 3);
            for (let i = 0; i < n; i++) {
                // Uniform random direction on a spherical shell.
                const r = spread * (0.35 + Math.random() * 0.65);
                const theta = Math.random() * Math.PI * 2;
                const phi = Math.acos(Math.random() * 2 - 1);
                const sinPhi = Math.sin(phi);
                positions[i * 3] = r * sinPhi * Math.cos(theta);
                positions[i * 3 + 1] = r * Math.cos(phi);
                positions[i * 3 + 2] = r * sinPhi * Math.sin(theta);
            }
            const geom = new THREE.BufferGeometry();
            geom.setAttribute('position', new THREE.BufferAttribute(positions, 3));
            const mat = new THREE.PointsMaterial({
                size: size,
                map: softDotTexture,
                color: color,
                transparent: true,
                opacity: opacity,
                blending: THREE.AdditiveBlending,
                depthWrite: false,
                sizeAttenuation: true
            });
            const points = new THREE.Points(geom, mat);
            points.frustumCulled = false;
            scene.add(points);
            starLayers.push({ points: points, driftY: driftY, driftX: driftX, full: n });
        }

        addStarLayer(2200 * CAPACITY_SCALE, 980, 1.5, 0xffffff, 0.88, 0.0021, 0.0004);
        addStarLayer(1200 * CAPACITY_SCALE, 800, 2.4, 0x9fc8ff, 0.62, -0.0015, 0.0009);
        addStarLayer(560 * CAPACITY_SCALE, 620, 3.6, 0xffe2c4, 0.48, 0.001, -0.0006);
        addStarLayer(280 * CAPACITY_SCALE, 420, 5.2, 0xc5a3ff, 0.32, -0.0007, 0.0012);

        // Ultra-only twinkle layer: a fast-oscillating extra starfield that
        // makes the sky feel electric on the top tier.
        const twinkleLayer = (function () {
            const n = 560;
            const positions = new Float32Array(n * 3);
            for (let i = 0; i < n; i++) {
                const r = 520 * (0.3 + Math.random() * 0.7);
                const theta = Math.random() * Math.PI * 2;
                const phi = Math.acos(Math.random() * 2 - 1);
                const sinPhi = Math.sin(phi);
                positions[i * 3] = r * sinPhi * Math.cos(theta);
                positions[i * 3 + 1] = r * Math.cos(phi);
                positions[i * 3 + 2] = r * sinPhi * Math.sin(theta);
            }
            const geom = new THREE.BufferGeometry();
            geom.setAttribute('position', new THREE.BufferAttribute(positions, 3));
            const mat = new THREE.PointsMaterial({
                size: 2.6,
                map: softDotTexture,
                color: 0xcdf0ff,
                transparent: true,
                opacity: 0.5,
                blending: THREE.AdditiveBlending,
                depthWrite: false,
                sizeAttenuation: true
            });
            const points = new THREE.Points(geom, mat);
            points.frustumCulled = false;
            points.visible = false;
            scene.add(points);
            return points;
        })();

        // ------------------------------------------------------------------
        // Nebula backdrop: a few huge faint glow sprites far behind the scene
        // ------------------------------------------------------------------
        const nebulae = [];

        function addNebula(color, x, y, z, size, opacity, pulseSpeed, pulsePhase) {
            const mat = new THREE.SpriteMaterial({
                map: softDotTexture,
                color: color,
                transparent: true,
                opacity: opacity,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            const sprite = new THREE.Sprite(mat);
            sprite.position.set(x, y, z);
            sprite.scale.set(size, size, 1);
            scene.add(sprite);
            nebulae.push({ mat: mat, base: opacity, speed: pulseSpeed, phase: pulsePhase, sprite: sprite });
        }

        addNebula(0x1b2a6e, -430, 150, -640, 760, 0.18, 0.11, 0);
        addNebula(0x3c1a5e, 470, -90, -580, 700, 0.16, 0.09, 1.7);
        addNebula(0x0e3a52, 130, 330, -720, 640, 0.15, 0.13, 3.1);
        addNebula(0x4a2418, -170, -270, -660, 580, 0.12, 0.07, 4.4);
        addNebula(0x123d33, 540, 230, -500, 540, 0.12, 0.1, 5.6);
        addNebula(0x2a1858, -80, 40, -800, 900, 0.1, 0.06, 2.2);

        // ------------------------------------------------------------------
        // Floating dust motes around the system (close-range atmosphere)
        // ------------------------------------------------------------------
        const dustCount = Math.max(80, Math.round(420 * CAPACITY_SCALE));
        const dustPos = new Float32Array(dustCount * 3);
        const dustPhase = new Float32Array(dustCount);
        const dustSpeed = new Float32Array(dustCount);
        for (let i = 0; i < dustCount; i++) {
            const r = 18 + Math.random() * 210;
            const th = Math.random() * Math.PI * 2;
            const ph = Math.acos(Math.random() * 2 - 1);
            dustPos[i * 3] = r * Math.sin(ph) * Math.cos(th);
            dustPos[i * 3 + 1] = r * Math.cos(ph) * 0.55;
            dustPos[i * 3 + 2] = r * Math.sin(ph) * Math.sin(th);
            dustPhase[i] = Math.random() * Math.PI * 2;
            dustSpeed[i] = 0.15 + Math.random() * 0.45;
        }
        const dustGeom = new THREE.BufferGeometry();
        const dustPosAttr = new THREE.BufferAttribute(dustPos, 3);
        dustPosAttr.setUsage(THREE.DynamicDrawUsage);
        dustGeom.setAttribute('position', dustPosAttr);
        const dustMat = new THREE.PointsMaterial({
            size: 0.85,
            map: softDotTexture,
            color: 0xa8d4ff,
            transparent: true,
            opacity: 0.35,
            blending: THREE.AdditiveBlending,
            depthWrite: false,
            sizeAttenuation: true
        });
        const dustPoints = new THREE.Points(dustGeom, dustMat);
        dustPoints.frustumCulled = false;
        dustPoints.name = 'sysworld-dust';
        scene.add(dustPoints);

        // ------------------------------------------------------------------
        // Aurora ribbons — slow rotating translucent bands. In ultra they
        // swap to an animated flow shader (energy streaming along the band).
        // ------------------------------------------------------------------
        const auroras = [];
        function makeFlowMaterial(color, opacity) {
            return new THREE.ShaderMaterial({
                uniforms: {
                    uTime: { value: 0 },
                    uColor: { value: new THREE.Color(color) },
                    uOpacity: { value: opacity * 2.6 }
                },
                vertexShader: 'varying vec2 vUv;\n' +
                    'void main() {\n' +
                    '    vUv = uv;\n' +
                    '    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);\n' +
                    '}',
                fragmentShader: 'uniform float uTime;\nuniform vec3 uColor;\nuniform float uOpacity;\nvarying vec2 vUv;\n' +
                    'void main() {\n' +
                    '    float band = 0.5 + 0.5 * sin((vUv.x * 10.0 - uTime * 1.5) * 6.28318);\n' +
                    '    float flow = 0.5 + 0.5 * sin((vUv.x * 3.0 + uTime * 0.7) * 6.28318);\n' +
                    '    float a = uOpacity * (0.3 + 0.7 * band) * (0.55 + 0.45 * flow);\n' +
                    '    gl_FragColor = vec4(uColor * (0.8 + 0.7 * band), a);\n' +
                    '}',
                transparent: true,
                side: THREE.DoubleSide,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
        }
        function addAurora(radius, y, color, opacity, spin) {
            const mat = new THREE.MeshBasicMaterial({
                color: color,
                transparent: true,
                opacity: opacity,
                side: THREE.DoubleSide,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            const shaderMat = makeFlowMaterial(color, opacity);
            const mesh = new THREE.Mesh(new THREE.TorusGeometry(radius, radius * 0.035, 8, 96), mat);
            mesh.rotation.x = Math.PI / 2.15;
            mesh.position.y = y;
            mesh.name = 'sysworld-aurora';
            scene.add(mesh);
            auroras.push({ mesh: mesh, mat: mat, shaderMat: shaderMat, base: opacity, spin: spin, phase: Math.random() * 6 });
        }
        addAurora(95, 8, 0x3de0c8, 0.07, 0.04);
        addAurora(118, -4, 0x6a8dff, 0.055, -0.03);
        addAurora(142, 16, 0xc06bff, 0.04, 0.025);

        // ------------------------------------------------------------------
        // Energy grid floor below the infrastructure field
        // ------------------------------------------------------------------
        const grid = new THREE.GridHelper(680, 68, 0x2a4a6a, 0x101f33);
        grid.position.y = (L.infraY != null ? L.infraY : -34) - 6;
        grid.material.transparent = true;
        grid.material.opacity = 0.22;
        grid.material.depthWrite = false;
        scene.add(grid);

        // Concentric energy rings on the floor
        const floorRings = [];
        [40, 70, 100, 140].forEach(function (r, i) {
            const mat = new THREE.MeshBasicMaterial({
                color: i % 2 ? 0x3a7a9a : 0x2a5a7a,
                transparent: true,
                opacity: 0.12,
                side: THREE.DoubleSide,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            const mesh = new THREE.Mesh(new THREE.RingGeometry(r - 0.35, r + 0.35, 96), mat);
            mesh.rotation.x = -Math.PI / 2;
            mesh.position.y = grid.position.y + 0.15;
            scene.add(mesh);
            floorRings.push({ mesh: mesh, mat: mat, base: 0.1 + i * 0.015, phase: i * 0.9 });
        });

        // Ultra-only energy wave field: expanding radial pulses flowing
        // outward across the floor plane (animated shader texture).
        const waveMat = new THREE.ShaderMaterial({
            uniforms: {
                uTime: { value: 0 },
                uColor: { value: new THREE.Color(0x2fd9c0) },
                uOpacity: { value: 0.34 }
            },
            vertexShader: 'varying vec2 vPos;\n' +
                'void main() {\n' +
                '    vPos = position.xy;\n' +
                '    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);\n' +
                '}',
            fragmentShader: 'uniform float uTime;\nuniform vec3 uColor;\nuniform float uOpacity;\nvarying vec2 vPos;\n' +
                'void main() {\n' +
                '    float r = length(vPos) / 170.0;\n' +
                '    float wave = 0.5 + 0.5 * sin((r * 12.0 - uTime * 1.9) * 6.28318);\n' +
                '    float ripple = 0.5 + 0.5 * sin((r * 34.0 - uTime * 3.4) * 6.28318);\n' +
                '    float fade = smoothstep(1.0, 0.3, r) * smoothstep(0.02, 0.1, r);\n' +
                '    gl_FragColor = vec4(uColor * (0.7 + 0.6 * ripple), uOpacity * wave * fade);\n' +
                '}',
            transparent: true,
            side: THREE.DoubleSide,
            blending: THREE.AdditiveBlending,
            depthWrite: false
        });
        const waveOverlay = new THREE.Mesh(new THREE.PlaneGeometry(340, 340), waveMat);
        waveOverlay.rotation.x = -Math.PI / 2;
        waveOverlay.position.y = grid.position.y + 0.05;
        waveOverlay.visible = false;
        waveOverlay.name = 'sysworld-energy-wave';
        scene.add(waveOverlay);

        // Accent lights for graph / mission zones
        const graphLight = new THREE.PointLight(0x7ec8ff, 0.45, 220, 2);
        graphLight.position.copy(L.graphCenter || new THREE.Vector3(0, 34, -120));
        scene.add(graphLight);
        const missionLight = new THREE.PointLight(0xffd54f, 0.35, 180, 2);
        missionLight.position.set(0, 4, L.missionRingRadius || 84);
        scene.add(missionLight);

        // ------------------------------------------------------------------
        // Resize handling (pattern from ui/js/chat/stl-viewer.js: measure in
        // the frame loop, only touch renderer/camera when the size changed)
        // ------------------------------------------------------------------
        let lastW = 0;
        let lastH = 0;

        function checkResize() {
            const rect = host.getBoundingClientRect();
            const w = Math.max(1, Math.floor(rect.width));
            const h = Math.max(1, Math.floor(rect.height));
            if (w === lastW && h === lastH) return;
            lastW = w;
            lastH = h;
            renderer.setSize(w, h, false);
            camera.aspect = w / h;
            camera.updateProjectionMatrix();
        }
        checkResize();

        // ------------------------------------------------------------------
        // Camera flights
        // ------------------------------------------------------------------
        let flight = null;

        function flyTo(camPos, lookAtPos, duration, opts) {
            const o = opts || {};
            // Cancel any active flight so flights chain cleanly.
            if (flight) { flight.cancel(); flight = null; }
            const toPos = new THREE.Vector3(camPos.x, camPos.y, camPos.z);
            const toTarget = new THREE.Vector3(lookAtPos.x, lookAtPos.y, lookAtPos.z);
            const toFov = typeof o.fov === 'number' ? o.fov : camera.fov;
            if (!inst.fx || typeof inst.fx.tween !== 'function') {
                // Fx not ready yet: jump directly instead of tweening.
                camera.position.copy(toPos);
                controls.target.copy(toTarget);
                if (toFov !== camera.fov) {
                    camera.fov = toFov;
                    camera.updateProjectionMatrix();
                }
                controls.update();
                return;
            }
            controls.enabled = false;
            const fromPos = camera.position.clone();
            const fromTarget = controls.target.clone();
            const fromFov = camera.fov;
            flight = inst.fx.tween({
                duration: duration || 2,
                ease: 'inOutCubic',
                update: function (t) {
                    camera.position.lerpVectors(fromPos, toPos, t);
                    controls.target.lerpVectors(fromTarget, toTarget, t);
                    if (toFov !== fromFov) {
                        camera.fov = fromFov + (toFov - fromFov) * t;
                        camera.updateProjectionMatrix();
                    }
                },
                done: function () {
                    flight = null;
                    controls.enabled = true;
                }
            });
        }

        function resetView() {
            flyTo(HOME_POS, HOME_TARGET, 2.2, { fov: BASE_FOV });
        }

        function introFlight() {
            // Cinematic swoop from the far/high start position on open.
            camera.position.copy(START_POS);
            camera.fov = INTRO_FOV;
            camera.updateProjectionMatrix();
            flyTo(HOME_POS, HOME_TARGET, 3.5, { fov: BASE_FOV });
        }

        // ------------------------------------------------------------------
        // Picking helpers
        // ------------------------------------------------------------------
        function screenToNDC(clientX, clientY) {
            const rect = canvas.getBoundingClientRect();
            const w = rect.width || 1;
            const h = rect.height || 1;
            return {
                x: ((clientX - rect.left) / w) * 2 - 1,
                y: -((clientY - rect.top) / h) * 2 + 1
            };
        }

        function raycast(meshes, ndc) {
            if (!meshes || !meshes.length || !ndc) return null;
            ndcPointer.set(ndc.x, ndc.y);
            raycaster.setFromCamera(ndcPointer, camera);
            const hits = raycaster.intersectObjects(meshes, false);
            return hits.length ? hits[0] : null;
        }

        // ------------------------------------------------------------------
        // Frame update (driven by the entry's single RAF loop)
        // ------------------------------------------------------------------
        let paused = false;
        let currentTier = 'high';

        // Live quality levers: density via setDrawRange, structure visibility
        // and renderer pixel ratio. Buffers stay at ultra capacity, so every
        // tier switch is instant and rebuild-free.
        function setQuality(tier) {
            currentTier = tier;
            const starFrac = tier === 'low' ? 0.4 : tier === 'medium' ? 0.65 : 1;
            for (let i = 0; i < starLayers.length; i++) {
                const layer = starLayers[i];
                layer.points.geometry.setDrawRange(0, Math.max(60, Math.floor(layer.full * starFrac)));
            }
            dustPoints.visible = tier !== 'low';
            const dustFrac = tier === 'medium' ? 0.6 : 1;
            dustGeom.setDrawRange(0, Math.floor(dustCount * dustFrac));
            for (let i = 0; i < nebulae.length; i++) {
                if (nebulae[i].sprite) nebulae[i].sprite.visible = tier !== 'low';
            }
            for (let i = 0; i < auroras.length; i++) {
                const a = auroras[i];
                a.mesh.visible = tier !== 'low' && tier !== 'medium';
                a.mesh.material = tier === 'ultra' ? a.shaderMat : a.mat;
            }
            twinkleLayer.visible = tier === 'ultra';
            waveOverlay.visible = tier === 'ultra';
            const dpr = window.devicePixelRatio || 1;
            const pr = tier === 'low' ? 1 : tier === 'medium' ? Math.min(dpr, 1.5) : Math.min(dpr, 2);
            renderer.setPixelRatio(pr);
            lastW = 0;
            checkResize();
        }

        function update(dt, elapsed) {
            controls.update();
            checkResize();
            // Very slow starfield drift + soft twinkle keeps the backdrop alive.
            for (let i = 0; i < starLayers.length; i++) {
                const layer = starLayers[i];
                layer.points.rotation.y += layer.driftY * dt;
                layer.points.rotation.x += layer.driftX * dt;
                if (layer.points.material) {
                    layer.points.material.opacity = (0.42 + i * 0.08) + 0.12 * Math.sin(elapsed * (0.35 + i * 0.11) + i);
                }
            }
            // Gentle nebula breathing + slow drift.
            for (let i = 0; i < nebulae.length; i++) {
                const neb = nebulae[i];
                neb.mat.opacity = neb.base * (0.82 + 0.18 * Math.sin(elapsed * neb.speed + neb.phase));
                if (neb.sprite) {
                    neb.sprite.position.x += Math.sin(elapsed * 0.03 + neb.phase) * 0.02;
                    neb.sprite.position.y += Math.cos(elapsed * 0.025 + neb.phase) * 0.015;
                }
            }
            // Dust float
            for (let i = 0; i < dustCount; i++) {
                dustPos[i * 3 + 1] += Math.sin(elapsed * dustSpeed[i] + dustPhase[i]) * 0.008;
                dustPos[i * 3] += Math.cos(elapsed * dustSpeed[i] * 0.7 + dustPhase[i]) * 0.004;
            }
            dustPosAttr.needsUpdate = true;
            dustMat.opacity = 0.28 + 0.12 * Math.sin(elapsed * 0.4);
            dustPoints.rotation.y += dt * 0.01;

            // Aurora spin / breathe
            for (let i = 0; i < auroras.length; i++) {
                const a = auroras[i];
                a.mesh.rotation.z += a.spin * dt;
                a.mat.opacity = a.base * (0.75 + 0.35 * Math.sin(elapsed * 0.55 + a.phase));
            }
            // Ultra tier: drive the animated shader clocks and the fast
            // twinkle oscillation (all uniform writes, zero allocations).
            if (currentTier === 'ultra') {
                for (let i = 0; i < auroras.length; i++) {
                    auroras[i].shaderMat.uniforms.uTime.value = elapsed;
                }
                waveMat.uniforms.uTime.value = elapsed;
                twinkleLayer.material.opacity = 0.35 + 0.45 * Math.abs(Math.sin(elapsed * 3.1));
            }
            // Floor rings pulse outward rhythm
            for (let i = 0; i < floorRings.length; i++) {
                const fr = floorRings[i];
                fr.mat.opacity = fr.base * (0.7 + 0.45 * Math.sin(elapsed * 0.9 + fr.phase));
            }

            grid.material.opacity = 0.16 + 0.07 * Math.sin(elapsed * 0.6);
            coreLight.intensity = 1.25 + 0.28 * Math.sin(elapsed * 2.1);
            graphLight.intensity = 0.35 + 0.18 * Math.sin(elapsed * 1.1 + 1);
            missionLight.intensity = 0.28 + 0.16 * Math.sin(elapsed * 1.4 + 2);
            renderer.render(scene, camera);
        }

        // Entry pauses the RAF loop; the stage only tracks the flag.
        function setPaused(value) { paused = !!value; }

        // ------------------------------------------------------------------
        // Lifecycle
        // ------------------------------------------------------------------
        let disposed = false;

        function dispose() {
            if (disposed) return;
            disposed = true;
            if (flight) { flight.cancel(); flight = null; }
            if (controls && typeof controls.dispose === 'function') controls.dispose();
            // Detached aurora flow materials are not reachable via traverse.
            for (let i = 0; i < auroras.length; i++) auroras[i].shaderMat.dispose();
            scene.traverse(function (obj) {
                if (obj.geometry) obj.geometry.dispose();
                if (obj.material) {
                    const mats = Array.isArray(obj.material) ? obj.material : [obj.material];
                    for (let i = 0; i < mats.length; i++) {
                        if (mats[i].map && mats[i].map !== softDotTexture) mats[i].map.dispose();
                        mats[i].dispose();
                    }
                }
            });
            softDotTexture.dispose();
            renderer.dispose();
            if (typeof renderer.forceContextLoss === 'function') renderer.forceContextLoss();
            if (canvas.parentNode) canvas.parentNode.removeChild(canvas);
        }

        return {
            scene: scene,
            camera: camera,
            renderer: renderer,
            controls: controls,
            flyTo: flyTo,
            resetView: resetView,
            introFlight: introFlight,
            screenToNDC: screenToNDC,
            raycast: raycast,
            update: update,
            setQuality: setQuality,
            setPaused: setPaused,
            isPaused: function () { return paused; },
            dispose: dispose
        };
    };
})();
