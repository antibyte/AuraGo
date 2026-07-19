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
        const qualityScale = typeof inst.qualityScale === 'number' ? inst.qualityScale : 1;
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
            const n = Math.max(60, Math.round(count * qualityScale));
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
            starLayers.push({ points: points, driftY: driftY, driftX: driftX });
        }

        addStarLayer(1500, 950, 1.6, 0xffffff, 0.85, 0.0021, 0.0004);
        addStarLayer(850, 760, 2.6, 0x9fc8ff, 0.6, -0.0015, 0.0009);
        addStarLayer(380, 560, 3.8, 0xffe2c4, 0.45, 0.001, -0.0006);

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
            nebulae.push({ mat: mat, base: opacity, speed: pulseSpeed, phase: pulsePhase });
        }

        addNebula(0x1b2a6e, -430, 150, -640, 760, 0.16, 0.11, 0);
        addNebula(0x3c1a5e, 470, -90, -580, 680, 0.14, 0.09, 1.7);
        addNebula(0x0e3a52, 130, 330, -720, 600, 0.13, 0.13, 3.1);
        addNebula(0x4a2418, -170, -270, -660, 560, 0.1, 0.07, 4.4);
        addNebula(0x123d33, 540, 230, -500, 520, 0.1, 0.1, 5.6);

        // ------------------------------------------------------------------
        // Energy grid floor below the infrastructure field
        // ------------------------------------------------------------------
        const grid = new THREE.GridHelper(680, 68, 0x2a4a6a, 0x101f33);
        grid.position.y = (L.infraY != null ? L.infraY : -34) - 6;
        grid.material.transparent = true;
        grid.material.opacity = 0.2;
        grid.material.depthWrite = false;
        scene.add(grid);

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

        function update(dt, elapsed) {
            controls.update();
            checkResize();
            // Very slow starfield drift keeps the backdrop alive.
            for (let i = 0; i < starLayers.length; i++) {
                const layer = starLayers[i];
                layer.points.rotation.y += layer.driftY * dt;
                layer.points.rotation.x += layer.driftX * dt;
            }
            // Gentle nebula breathing, grid pulse and core light shimmer.
            for (let i = 0; i < nebulae.length; i++) {
                const neb = nebulae[i];
                neb.mat.opacity = neb.base * (0.85 + 0.15 * Math.sin(elapsed * neb.speed + neb.phase));
            }
            grid.material.opacity = 0.17 + 0.05 * Math.sin(elapsed * 0.6);
            coreLight.intensity = 1.2 + 0.18 * Math.sin(elapsed * 2.1);
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
            setPaused: setPaused,
            isPaused: function () { return paused; },
            dispose: dispose
        };
    };
})();
