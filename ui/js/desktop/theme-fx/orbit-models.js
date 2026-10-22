(function () {
    'use strict';

    // Orbit models (global AuraOrbitModels). Loads the four Blender models of the theme (station, shuttle, satellite,
    // asteroids; GLB files under assetsBase) and animates them: the habitat ring turns, the beacons blink, a few window
    // panes flicker, the shuttle undocks, cruises and docks again (choreo.shuttleOffset), its engine glows and leaves an
    // exhaust trail, the satellite crosses the foreground and turns its dish toward the planet, and the asteroids tumble
    // as instanced meshes. Every model is optional: a file that fails to load is reported as 'error' and the rest of the
    // scene keeps working. The scene hands in the placement rules through layout(); nothing here touches the renderer.
    // window.AuraOrbitModels = { create }.

    const FILES = ['station', 'shuttle', 'satellite', 'asteroids'];
    const STATION_RING_DIAMETER = 22;     // scene units
    const STATION_DISTANCE = 150;         // from the camera
    const STATION_TILT = 30;              // degrees: the spine leans toward the camera
    const STATION_LEAN = -13;             // degrees: and to the side on screen
    const STATION_RING_SPEED = 0.09;      // rad/s
    const SHUTTLE_SCALE = 1.9;            // the shuttle is drawn larger than true scale so that it reads
    const SHUTTLE_HALF = 0.0875;          // half the model length in metres (the origin is the nose tip)
    const SATELLITE_SPAN = 8;             // wing span in scene units
    const SATELLITE_CYCLE = 70;
    const SATELLITE_TRAVEL = 46;
    const SATELLITE_DISTANCE = 92;
    const ROCK_VARIANTS = 5;
    const ROCK_TOTAL = 140;
    const EXHAUST_POINTS = 48;
    const EXHAUST_LIFE = 0.8;
    const EXHAUST_RATE = 60;
    const BEACON_PERIOD = 1;
    const BEACON_ON = 0.12;
    // The station, the satellite and the foreground rocks follow this share of the camera's orbit around the planet, so
    // the drift and the pointer parallax of the camera never carry them out of their place on screen (the rest is parallax).
    const FOLLOW = 0.7;

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
    function smooth(a, b, x) { const t = clamp((x - a) / (b - a), 0, 1); return t * t * (3 - 2 * t); }

    function create(options) {
        const THREE = options.THREE;
        const choreo = options.choreo;
        const scene = options.scene;
        const assetsBase = String(options.assetsBase || '/3d/desktop-theme-orbit/v1/');
        const exhaustShader = options.shaders;
        const windowTime = { value: 0 };
        const status = { station: 'pending', shuttle: 'pending', satellite: 'pending', asteroids: 'pending' };
        const disposables = [];

        let station = null;          // { root, model, ring, dock, beacons: [{ material, phase }], scale }
        let shuttle = null;          // { holder, engine, exhaust }
        let satellite = null;        // { root, dish }
        let rocks = null;            // { meshes: [InstancedMesh], records: [] }
        let trail = null;            // { points, positions, ages, head, carry, last }
        let ctx = null;
        const follow = new THREE.Quaternion();
        let rockQuota = ROCK_TOTAL;
        let shuttleState = 'docked';
        let engineLevel = 0;
        let disposed = false;
        const tmp = { a: new THREE.Vector3(), b: new THREE.Vector3(), q: new THREE.Quaternion(), m: new THREE.Matrix4(), s: new THREE.Vector3(), c: new THREE.Color(), e: new THREE.Euler() };

        function track(item) { disposables.push(item); return item; }

        // ---- Loading ------------------------------------------------------------------------------
        function disposeObject(root) {
            root.traverse(node => {
                if (node.geometry) node.geometry.dispose();
                const materials = Array.isArray(node.material) ? node.material : node.material ? [node.material] : [];
                for (const material of materials) {
                    for (const key of Object.keys(material)) if (material[key] && material[key].isTexture) material[key].dispose();
                    material.dispose();
                }
            });
        }

        function load() {
            const loader = new options.GLTFLoader();
            const jobs = FILES.map(name => loader.loadAsync(assetsBase + name + '.glb').then(gltf => {
                if (disposed) { disposeObject(gltf.scene); return; }
                try {
                    ({ station: setupStation, shuttle: setupShuttle, satellite: setupSatellite, asteroids: setupAsteroids })[name](gltf.scene);
                    status[name] = 'ok';
                } catch (error) {
                    status[name] = 'error';
                    disposeObject(gltf.scene);
                    if (options.noteError) options.noteError('model ' + name, error);
                }
            }, error => {
                status[name] = 'error';
                if (options.noteError) options.noteError('model ' + name, error);
            }));
            // the shuttle rides on the station, so they are wired together once both exist
            return Promise.all(jobs).then(() => { attachShuttle(); relayout(); });
        }

        // ---- Station ------------------------------------------------------------------------------
        function patchWindows(material) {
            material.onBeforeCompile = shader => {
                shader.uniforms.uWinTime = windowTime;
                shader.vertexShader = shader.vertexShader
                    .replace('#include <common>', '#include <common>\nvarying vec3 vWinPos;')
                    .replace('#include <begin_vertex>', '#include <begin_vertex>\nvWinPos = position;');
                shader.fragmentShader = shader.fragmentShader
                    .replace('#include <common>', '#include <common>\nvarying vec3 vWinPos;\nuniform float uWinTime;\nfloat winHash(vec3 p) { return fract(sin(dot(p, vec3(12.9898, 78.233, 37.719))) * 43758.5453); }')
                    .replace('#include <emissivemap_fragment>', '#include <emissivemap_fragment>\n' +
                        'float winCell = winHash(floor(vWinPos * 24.0));\n' +
                        'float winOn = step(0.11, winCell) * (0.86 + 0.14 * sin(uWinTime * (0.35 + winCell * 1.7) + winCell * 61.0));\n' +
                        'totalEmissiveRadiance *= winOn;');
            };
            material.customProgramCacheKey = () => 'orbit-windows';
            material.needsUpdate = true;
        }

        function setupStation(model) {
            const ring = model.getObjectByName('RingRotor');
            const dock = model.getObjectByName('DockPort');
            const box = new THREE.Box3().setFromObject(ring || model);
            const size = box.getSize(new THREE.Vector3());
            const scale = STATION_RING_DIAMETER / Math.max(size.x, size.z, 0.01);
            const root = new THREE.Group();
            root.name = 'OrbitStation';
            model.scale.setScalar(scale);
            root.add(model);
            const beacons = [];
            model.traverse(node => {
                if (!node.isMesh) return;
                const materials = Array.isArray(node.material) ? node.material : [node.material];
                for (const material of materials) {
                    const name = String(material.name || '');
                    if (/emissive/i.test(name)) {
                        material.userData.baseIntensity = material.emissiveIntensity || 1;
                        if (/window/i.test(name)) { material.emissiveIntensity = 3.0; patchWindows(material); }
                        else if (/beacon/i.test(name)) beacons.push({ material, phase: /green/i.test(name) ? 0.5 : 0 });
                        else material.emissiveIntensity = 2.6;
                    }
                }
            });
            scene.add(root);
            station = { root, model, ring, dock, beacons, scale, base: { position: new THREE.Vector3(), quaternion: new THREE.Quaternion() } };
            track(() => { scene.remove(root); disposeObject(root); });
        }

        // ---- Shuttle ------------------------------------------------------------------------------
        function setupShuttle(model) {
            const holder = new THREE.Group();
            holder.name = 'OrbitShuttle';
            model.scale.setScalar(SHUTTLE_SCALE);
            model.position.z = SHUTTLE_HALF * SHUTTLE_SCALE;   // the holder's origin is the middle of the body
            holder.add(model);
            const glow = model.getObjectByName('EngineGlow');
            const engine = glow && glow.material ? glow.material : null;
            if (engine) engine.emissiveIntensity = 0;
            for (const name of ['NavLightPort', 'NavLightStarboard', 'NoseLights']) {
                const node = model.getObjectByName(name);
                if (node && node.material) node.material.emissiveIntensity = 3;
            }
            shuttle = { holder, model, engine, exhaust: model.getObjectByName('Exhaust') || null };
            track(() => { holder.removeFromParent(); disposeObject(holder); });
        }

        function attachShuttle() {
            if (!shuttle || shuttle.attached) return;
            // Without the station the shuttle has no dock: drop it.
            if (!station) { shuttle.holder.removeFromParent(); disposeObject(shuttle.holder); shuttle = null; status.shuttle = 'error'; return; }
            station.model.add(shuttle.holder);
            shuttle.attached = true;
            if (!trail) buildTrail();
        }

        function buildTrail() {
            const positions = new Float32Array(EXHAUST_POINTS * 3);
            const ages = new Float32Array(EXHAUST_POINTS).fill(99);
            const geometry = new THREE.BufferGeometry();
            geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
            geometry.setAttribute('aAge', new THREE.BufferAttribute(ages, 1));
            const material = new THREE.ShaderMaterial({
                uniforms: { uLife: { value: EXHAUST_LIFE }, uPx: { value: options.pixelRatio ? options.pixelRatio() : 1 }, uRef: { value: 150 } },
                vertexShader: exhaustShader.exhaustVertex, fragmentShader: exhaustShader.exhaustFragment,
                transparent: true, depthWrite: false, blending: THREE.AdditiveBlending
            });
            const points = new THREE.Points(geometry, material);
            points.frustumCulled = false;
            points.renderOrder = 9;
            scene.add(points);
            trail = { points, positions, ages, head: 0, carry: 0, last: new THREE.Vector3(), has: false, material };
            track(() => { scene.remove(points); geometry.dispose(); material.dispose(); });
        }

        // ---- Satellite ----------------------------------------------------------------------------
        function setupSatellite(model) {
            const box = new THREE.Box3().setFromObject(model);
            const size = box.getSize(new THREE.Vector3());
            const scale = SATELLITE_SPAN / Math.max(size.x, size.z, 0.01);
            const root = new THREE.Group();
            root.name = 'OrbitSatellite';
            model.scale.setScalar(scale);
            root.add(model);
            root.visible = false;
            scene.add(root);
            satellite = { root, dish: model.getObjectByName('Dish') || null };
            track(() => { scene.remove(root); disposeObject(root); });
        }

        // ---- Asteroids ----------------------------------------------------------------------------
        function setupAsteroids(model) {
            const meshes = [];
            for (let i = 0; i < ROCK_VARIANTS; i++) {
                const source = model.getObjectByName('Rock' + i);
                if (!source || !source.isMesh) continue;
                const material = source.material.clone();
                material.roughness = Math.max(material.roughness, 0.85);
                const mesh = new THREE.InstancedMesh(source.geometry, material, ROCK_TOTAL);
                mesh.frustumCulled = false;
                mesh.count = 0;
                mesh.instanceColor = new THREE.InstancedBufferAttribute(new Float32Array(ROCK_TOTAL * 3).fill(1), 3);
                scene.add(mesh);
                meshes.push(mesh);
                track(() => { scene.remove(mesh); mesh.dispose(); material.dispose(); });
            }
            if (!meshes.length) throw new Error('no Rock0..Rock4 nodes');
            rocks = { meshes, records: [] };
            track(() => { model.traverse(node => { if (node.geometry) node.geometry.dispose(); }); });
        }

        function buildRocks() {
            if (!rocks || !ctx) return;
            const rng = choreo.createRng(2718);
            const basis = ctx.basis;
            const records = [];
            for (let i = 0; i < ROCK_TOTAL; i++) {
                const near = rng() < 0.58;
                const position = new THREE.Vector3();
                let scale;
                if (near) {
                    // a sparse arc sweeping around the lower left corner, in front of everything
                    const u = rng();
                    const spread = (rng() - 0.5) * 0.30;
                    const x = -1.12 + 0.95 * u + spread * 0.5;
                    const y = -0.30 - 0.85 * Math.sin(u * Math.PI * 0.5) + spread;
                    ctx.rayPoint((x + 1) / 2, (1 - y) / 2, 105 + 150 * rng() * rng(), position);
                    scale = 0.45 + 3.2 * Math.pow(rng(), 2.4);
                } else {
                    // the far side of the giant: loose rocks on the ring plane behind it
                    const angle = rng() * Math.PI * 2;
                    const radius = 285 + 150 * rng();
                    position.copy(ctx.ringE1).multiplyScalar(Math.cos(angle) * radius).addScaledVector(ctx.ringE2, Math.sin(angle) * radius).addScaledVector(ctx.ringN, (rng() - 0.5) * 36);
                    if (position.dot(basis.back) > radius * 0.15) position.negate();
                    scale = 0.8 + 2.4 * Math.pow(rng(), 1.8);
                }
                const axis = new THREE.Vector3(rng() - 0.5, rng() - 0.5, rng() - 0.5).normalize();
                records.push({
                    variant: i % ROCK_VARIANTS, near, position, scale, axis, speed: (0.04 + 0.16 * rng()) * (rng() < 0.5 ? -1 : 1), phase: rng() * Math.PI * 2,
                    tone: 0.30 + 0.32 * rng(), warm: rng()
                });
            }
            rocks.records = records;
        }

        // ---- Layout -------------------------------------------------------------------------------
        // The scene calls this whenever the framing changes: { rayPoint(sx, sy, distance, out), basis: { right, up, back },
        // composition, ringN, ringE1, ringE2 } with the world-space vectors of the neutral camera.
        function layout(next) {
            ctx = next;
            relayout();
        }

        function relayout() {
            if (!ctx) return;
            if (station) {
                const { basis, composition } = ctx;
                ctx.rayPoint(composition.station.x, composition.station.y, STATION_DISTANCE, tmp.a);
                station.base.position.copy(tmp.a);
                // a narrow window gives the station less width to work with
                station.root.scale.setScalar(clamp(composition.aspect / 1.3, 0.62, 1));
                const tilt = STATION_TILT * Math.PI / 180;
                const spine = tmp.b.copy(basis.up).multiplyScalar(Math.cos(tilt)).addScaledVector(basis.back, Math.sin(tilt));
                spine.applyAxisAngle(basis.back, STATION_LEAN * Math.PI / 180).normalize();
                const q1 = new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 1, 0), spine);
                // spin about the spine until the solar panel axis lies across the picture
                const panel = new THREE.Vector3(1, 0, 0).applyQuaternion(q1);
                const target = basis.right.clone().addScaledVector(spine, -basis.right.dot(spine)).normalize();
                const angle = Math.atan2(new THREE.Vector3().crossVectors(panel, target).dot(spine), panel.dot(target)) + 0.30;
                station.base.quaternion.copy(new THREE.Quaternion().setFromAxisAngle(spine, angle).multiply(q1));
                placeStation();
            }
            buildRocks();
        }

        // ---- Animation ----------------------------------------------------------------------------
        function placeStation() {
            station.root.position.copy(station.base.position).applyQuaternion(follow);
            station.root.quaternion.copy(follow).multiply(station.base.quaternion);
        }

        function updateStation(t) {
            if (!station) return;
            placeStation();
            if (station.ring) station.ring.rotation.y = t * STATION_RING_SPEED;
            for (const beacon of station.beacons) {
                const phase = ((t / BEACON_PERIOD + beacon.phase) % 1 + 1) % 1;
                beacon.material.emissiveIntensity = phase < BEACON_ON / BEACON_PERIOD ? 5 : 0.35;
            }
        }

        function updateShuttle(t, dt) {
            if (!shuttle || !shuttle.attached || !station) return;
            const state = choreo.shuttleState(t);
            const off = choreo.shuttleOffset(state);
            shuttleState = state.phase;
            const p = state.progress;
            let nose = 1;
            if (state.phase === 'undock') nose = 1 - smooth(0.35, 1, p);
            else if (state.phase === 'cruise') nose = 0;
            else if (state.phase === 'approach') nose = smooth(0.2, 0.95, p);
            let level = 0;
            if (state.phase === 'undock') level = 0.15 + 0.85 * smooth(0, 0.55, p);
            else if (state.phase === 'cruise') level = 0.85 + 0.15 * Math.sin(p * Math.PI * 14);
            else if (state.phase === 'approach') level = 0.45 * (1 - smooth(0.6, 1, p)) + 0.1;
            else if (state.phase === 'dock') level = 0.30 * (1 - smooth(0.2, 1, p));
            engineLevel = level;
            const holder = shuttle.holder;
            const dock = station.dock ? station.dock.position : tmp.s.set(0, -0.85, 0);
            holder.position.set(dock.x + off.x, dock.y - SHUTTLE_HALF * SHUTTLE_SCALE + off.y, dock.z + off.z);
            holder.quaternion.setFromEuler(tmp.e.set(-nose * Math.PI / 2, off.yaw, 0, 'YXZ'));
            if (shuttle.engine) shuttle.engine.emissiveIntensity = level > 0.01 ? 0.5 + 6 * level : 0;
            updateTrail(dt, level);
        }

        function updateTrail(dt, level) {
            if (!trail) return;
            const ages = trail.ages;
            for (let i = 0; i < EXHAUST_POINTS; i++) ages[i] += dt;
            if (level > 0.2 && shuttle && shuttle.exhaust) {
                shuttle.exhaust.getWorldPosition(tmp.a);
                if (!trail.has) { trail.last.copy(tmp.a); trail.has = true; }
                trail.carry += dt * EXHAUST_RATE;
                const count = Math.min(Math.floor(trail.carry), 4);
                trail.carry -= Math.floor(trail.carry);
                for (let k = 1; k <= count; k++) {
                    const i = trail.head;
                    trail.head = (trail.head + 1) % EXHAUST_POINTS;
                    tmp.b.lerpVectors(trail.last, tmp.a, k / count);
                    trail.positions[i * 3] = tmp.b.x;
                    trail.positions[i * 3 + 1] = tmp.b.y;
                    trail.positions[i * 3 + 2] = tmp.b.z;
                    ages[i] = (count - k) / EXHAUST_RATE;
                }
                trail.last.copy(tmp.a);
            } else {
                trail.has = false;
            }
            trail.points.geometry.attributes.position.needsUpdate = true;
            trail.points.geometry.attributes.aAge.needsUpdate = true;
        }

        function updateSatellite(t) {
            if (!satellite || !ctx) return;
            const local = ((t % SATELLITE_CYCLE) + SATELLITE_CYCLE) % SATELLITE_CYCLE;
            if (local > SATELLITE_TRAVEL) { satellite.root.visible = false; return; }
            const k = local / SATELLITE_TRAVEL;
            const sx = -0.12 + 1.24 * k;
            const sy = ctx.composition.station.y + 0.12 + 0.05 * Math.sin(k * Math.PI);
            ctx.rayPoint(sx, sy, SATELLITE_DISTANCE, tmp.a);
            satellite.root.position.copy(tmp.a).applyQuaternion(follow);
            satellite.root.visible = true;
            satellite.root.rotation.set(0.35 + t * 0.05, t * 0.11, 0.2 + Math.sin(t * 0.07) * 0.2);
            if (satellite.dish) {
                satellite.root.updateMatrixWorld(true);
                satellite.dish.lookAt(0, 0, 0);
            }
        }

        function updateRocks(t) {
            if (!rocks || !rocks.records.length) return;
            const counters = new Array(rocks.meshes.length).fill(0);
            const records = rocks.records;
            const n = Math.min(rockQuota, records.length);
            for (let i = 0; i < n; i++) {
                const r = records[i];
                const mesh = rocks.meshes[r.variant % rocks.meshes.length];
                const slot = counters[r.variant % rocks.meshes.length]++;
                tmp.q.setFromAxisAngle(r.axis, r.phase + t * r.speed);
                tmp.s.setScalar(r.scale);
                tmp.m.compose(r.near ? tmp.b.copy(r.position).applyQuaternion(follow) : r.position, tmp.q, tmp.s);
                mesh.setMatrixAt(slot, tmp.m);
                tmp.c.setHSL(0.06 + 0.04 * r.warm, 0.10 + 0.14 * r.warm, r.tone);
                mesh.setColorAt(slot, tmp.c);
            }
            for (let v = 0; v < rocks.meshes.length; v++) {
                const mesh = rocks.meshes[v];
                mesh.count = counters[v];
                mesh.instanceMatrix.needsUpdate = true;
                if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
            }
        }

        function update(t, dt, lowRate, rig) {
            windowTime.value = t % 6000;
            if (rig) follow.identity().slerp(rig, FOLLOW);
            updateStation(t);
            updateShuttle(t, dt);
            updateSatellite(t);
            if (!lowRate || Math.floor(t * 30) % 2 === 0) updateRocks(t);
        }

        function setQuality(rockCount, pixelRatio) {
            rockQuota = clamp(Math.round(rockCount), 0, ROCK_TOTAL);
            if (trail) trail.material.uniforms.uPx.value = pixelRatio || 1;
        }

        function inspect() {
            return {
                status: { ...status }, shuttle: shuttleState, engine: Math.round(engineLevel * 100) / 100,
                rocks: rocks ? Math.min(rockQuota, rocks.records.length) : 0, satellite: !!(satellite && satellite.root.visible)
            };
        }

        function dispose() {
            disposed = true;
            for (const item of disposables.splice(0)) { try { item(); } catch (_) { /* best effort */ } }
            station = shuttle = satellite = rocks = trail = null;
        }

        return { load, layout, update, setQuality, inspect, dispose, status };
    }

    window.AuraOrbitModels = Object.freeze({ create });
}());
