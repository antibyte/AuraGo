/* Leuchtfeuer view. Camera placement uses LeuchtfeuerSim.cameraOffset only.
   Movement reads camera.getWorldDirection and LeuchtfeuerSim.wishFromCamera.
   glTF forward "+Z" (manifest) is mounted with rotation.y = π onto parent −Z. */
(function (root) {
    const PACK = '/api/game-maker/asset-packs/aurago-pirates-3d/';
    const RUNTIME = '/api/game-maker/asset-packs/runtime/';
    const LOOK = 1.3;
    const EYE = 2.2;
    const MIN_DIST = 4;
    const MAX_DIST = 12;

    function lod0Manifest(manifest) {
        const allow = new Set(root.LeuchtfeuerWorld.MODEL_IDS);
        return {
            id: manifest.id,
            kind: manifest.kind,
            assets: manifest.assets.filter(function (asset) { return allow.has(asset.id); }).map(function (asset) {
                const copy = Object.assign({}, asset);
                copy.lods = asset.lods.slice(0, 1);
                return copy;
            })
        };
    }

    function play(actions, instance, clip) {
        try { actions.playAction(instance, clip); } catch (err) { /* missing clip stays on the last pose */ }
    }

    async function mount(stage, world, sim, options) {
        const signal = options.signal;
        const assetsMod = await import(RUNTIME + 'aurago-three-assets-1.js');
        const fxMod = await import(RUNTIME + 'aurago-effects-3d-1.js');
        await import(RUNTIME + 'three-0.186.1.module.min.js');
        if (signal.aborted) throw new DOMException('Disposed', 'AbortError');
        const THREE = assetsMod.THREE;
        const canvas = document.createElement('canvas');
        stage.replaceChildren(canvas);
        const renderer = new THREE.WebGLRenderer({ canvas: canvas, antialias: true, alpha: false });
        renderer.setPixelRatio(Math.min(root.devicePixelRatio || 1, 1.5));
        renderer.setSize(stage.clientWidth || 1, stage.clientHeight || 1, false);
        renderer.outputColorSpace = THREE.SRGBColorSpace;
        const scene = new THREE.Scene();
        scene.background = new THREE.Color(0x8eb8c8);
        const camera = new THREE.PerspectiveCamera(55, 1, 0.05, 200);
        const ambient = new THREE.AmbientLight(0xffffff, 0.4);
        const sun = new THREE.DirectionalLight(0xfff2d8, 1.15);
        sun.position.set(-12, 22, 8);
        scene.add(ambient, sun);
        const filtered = lod0Manifest(options.manifest);
        const loaded = new Map();
        await Promise.all(root.LeuchtfeuerWorld.MODEL_IDS.map(async function (id) {
            loaded.set(id, await assetsMod.loadAsset(filtered, id, PACK, { signal: signal }));
        }));
        if (signal.aborted) throw new DOMException('Disposed', 'AbortError');

        const props = [];
        const actors = {};
        world.placements.forEach(function (place) {
            const asset = loaded.get(place.id);
            const instance = assetsMod.createInstance(asset, { shadows: false });
            const parent = new THREE.Group();
            parent.position.set(place.x, place.y, place.z);
            parent.add(instance.root);
            scene.add(parent);
            props.push({ id: place.id, instance: instance, parent: parent, asset: asset });
        });
        world.actors.forEach(function (actor) {
            if (actor.role === 'bird') {
                const prop = props.find(function (item) { return item.id === actor.id; });
                if (prop) actors.bird = { actor: actor, instance: prop.instance, parent: prop.parent, asset: prop.asset, clip: '' };
                return;
            }
            const asset = loaded.get(actor.id);
            const instance = assetsMod.createInstance(asset, { shadows: false });
            const parent = new THREE.Group();
            const mountPoint = new THREE.Group();
            if (asset.model && asset.model.forward === '+Z') mountPoint.rotation.y = Math.PI;
            mountPoint.add(instance.root);
            parent.add(mountPoint);
            scene.add(parent);
            actors[actor.role === 'guard' ? actor.role + actor.index : actor.role] = {
                actor: actor, instance: instance, parent: parent, asset: asset, clip: ''
            };
        });

        const walk = new THREE.Group();
        const walkMat = new THREE.MeshBasicMaterial({ transparent: true, opacity: 0, depthWrite: false, side: THREE.DoubleSide });
        world.surfaces.forEach(function (surface) {
            const width = Math.max(0.2, surface.maxX - surface.minX);
            const depth = Math.max(0.2, surface.maxZ - surface.minZ);
            const mesh = new THREE.Mesh(new THREE.PlaneGeometry(width, depth), walkMat);
            mesh.rotation.x = -Math.PI / 2;
            mesh.position.set((surface.minX + surface.maxX) / 2, surface.y + 0.03, (surface.minZ + surface.maxZ) / 2);
            mesh.userData.walk = true;
            walk.add(mesh);
        });
        scene.add(walk);
        const raycaster = new THREE.Raycaster();
        const pointer = new THREE.Vector2();
        const forward = new THREE.Vector3();
        const modelForward = new THREE.Vector3();

        const presentation = fxMod.createPresentation({
            adapter: fxMod.createThreeAdapter({ scene: scene, camera: camera, renderer: renderer, sun: sun, ambient: ambient }),
            config: options.config,
            root: stage,
            report: function () {},
            controls: false
        });
        presentation.setEnvironment('coast');
        presentation.audio.setMuted(true);
        walk.children.forEach(function (mesh) { presentation.registerSurface(mesh, 'ground'); });

        let paused = true;
        let orbitYaw = sim.yaw + Math.PI;
        let distance = 7;
        let dragging = false;
        let dragX = 0;
        let dragY = 0;
        let moved = 0;
        let resize = true;
        let stepClock = 0;
        let fireClock = 0;
        let chestOpen = false;
        let interactUntil = 0;
        let frame = 0;
        let last = 0;
        let disposed = false;
        const keys = new Set();
        const guardLast = {};

        function fit() {
            const width = stage.clientWidth || 1;
            const height = stage.clientHeight || 1;
            camera.aspect = width / Math.max(1, height);
            camera.updateProjectionMatrix();
            renderer.setSize(width, height, false);
            presentation.resize();
        }

        function placeCamera() {
            const offset = root.LeuchtfeuerSim.cameraOffset(orbitYaw, distance);
            camera.position.set(sim.x + offset.x, sim.y + EYE, sim.z + offset.z);
            camera.lookAt(sim.x, sim.y + LOOK, sim.z);
            camera.updateMatrixWorld();
        }

        function wish() {
            camera.getWorldDirection(forward);
            forward.y = 0;
            if (forward.lengthSq() < 1e-6) return { x: 0, z: 0 };
            forward.normalize();
            let strafe = 0;
            let ahead = 0;
            if (keys.has('KeyA') || keys.has('ArrowLeft')) strafe -= 1;
            if (keys.has('KeyD') || keys.has('ArrowRight')) strafe += 1;
            if (keys.has('KeyW') || keys.has('ArrowUp')) ahead += 1;
            if (keys.has('KeyS') || keys.has('ArrowDown')) ahead -= 1;
            return root.LeuchtfeuerSim.wishFromCamera(forward.x, forward.z, strafe, ahead);
        }

        function setClip(entry, clip) {
            if (!entry || entry.clip === clip) return;
            play(assetsMod, entry.instance, clip);
            entry.clip = clip;
        }

        function syncActors(dt) {
            const player = actors.player;
            if (player) {
                player.parent.position.set(sim.x, sim.y, sim.z);
                player.parent.rotation.y = sim.yaw;
                if (sim.picked) interactUntil = 0.6;
                if (interactUntil > 0) {
                    interactUntil -= dt;
                    setClip(player, 'interact');
                } else setClip(player, sim.moving ? 'walk' : 'idle');
            }
            const fisher = actors.fisher;
            if (fisher) {
                fisher.parent.position.set(fisher.actor.x, fisher.actor.y, fisher.actor.z);
                fisher.parent.rotation.y = root.LeuchtfeuerSim.yawFacing(fisher.actor.faceX, fisher.actor.faceZ);
                setClip(fisher, 'idle');
            }
            world.guards.forEach(function (guard, index) {
                const entry = actors['guard' + index];
                if (!entry) return;
                const pose = root.LeuchtfeuerSim.patrolPose(guard, sim.time);
                const prev = guardLast[index] || pose;
                const dx = pose.x - prev.x;
                const dz = pose.z - prev.z;
                entry.parent.position.set(pose.x, pose.y, pose.z);
                if (dx * dx + dz * dz > 1e-6) entry.parent.rotation.y = root.LeuchtfeuerSim.yawFacing(dx, dz);
                guardLast[index] = pose;
                setClip(entry, 'walk');
            });
            const bird = actors.bird;
            if (bird) setClip(bird, 'idle');
            const chest = props.find(function (item) { return item.id === 'equipment-treasure-chest'; });
            if (chest && sim.parts.glass && !chestOpen) {
                play(assetsMod, chest.instance, 'open');
                chestOpen = true;
            }
            if (chest && !sim.parts.glass && chestOpen) {
                play(assetsMod, chest.instance, 'close');
                chestOpen = false;
            }
            Object.keys(actors).forEach(function (key) {
                assetsMod.updateInstance(actors[key].instance, dt, camera);
            });
            props.forEach(function (item) { assetsMod.updateInstance(item.instance, dt, camera); });
        }

        function effects(dt) {
            const position = [sim.x, sim.y + 1, sim.z];
            if (sim.picked) presentation.event('pickup', position);
            if (sim.splashed) presentation.event('splash', position);
            if (sim.won) presentation.event('win', position);
            if (sim.moving) {
                stepClock += dt;
                if (stepClock > 0.45) {
                    stepClock = 0;
                    const sample = sim.surface === 'beach' ? 'step-gravel' : 'step-wood';
                    presentation.audio.play(sample, { position: position });
                }
            } else stepClock = 0;
            if (sim.lit) {
                fireClock += dt;
                const lamp = world.spots.find(function (spot) { return spot.id === 'lamp'; });
                if (fireClock > 0.35 && lamp) {
                    fireClock = 0;
                    presentation.emit('fire', { position: [lamp.x, lamp.y + 1.1, lamp.z], intensity: 0.8 });
                }
            }
            const body = actors.player;
            if (body) {
                body.parent.updateMatrixWorld();
                modelForward.set(0, 0, -1).applyQuaternion(body.parent.quaternion);
                presentation.audio.listener(
                    [sim.x, sim.y + 1.6, sim.z],
                    [modelForward.x, modelForward.y, modelForward.z],
                    [0, 1, 0]
                );
            }
        }

        function frameOf(now) {
            if (disposed) return;
            frame = root.requestAnimationFrame(frameOf);
            const dt = last ? Math.min(0.05, (now - last) / 1000) : 0;
            last = now;
            if (resize) { resize = false; fit(); }
            const active = !paused && (!options.isActive || options.isActive());
            presentation.setPaused(!active || root.document.hidden);
            if (active && !root.document.hidden) {
                const pressed = options.takeInteract ? options.takeInteract() : false;
                root.LeuchtfeuerSim.step(sim, { wish: wish(), interact: pressed }, dt);
                syncActors(dt);
                effects(dt);
                if (options.onFrame) options.onFrame(sim);
            }
            placeCamera();
            presentation.update(dt);
            presentation.render();
        }

        function onKey(event) {
            if (event.type === 'keyup') { keys.delete(event.code); return; }
            if (event.repeat || paused) return;
            if (options.isActive && !options.isActive()) return;
            keys.add(event.code);
        }
        function onContext(event) {
            if (event.target === canvas) event.preventDefault();
        }
        function onPointerDown(event) {
            if (event.button !== 0 && event.button !== 2) return;
            dragging = event.button === 2;
            dragX = event.clientX;
            dragY = event.clientY;
            moved = 0;
        }
        function onPointerMove(event) {
            if (!dragging) return;
            const dx = event.clientX - dragX;
            moved += Math.abs(dx);
            dragX = event.clientX;
            orbitYaw -= dx * 0.005;
        }
        function onPointerUp(event) {
            const wasDrag = moved > 6;
            dragging = false;
            if (wasDrag || event.button !== 0 || paused) return;
            const rect = canvas.getBoundingClientRect();
            pointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
            pointer.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;
            raycaster.setFromCamera(pointer, camera);
            const hit = raycaster.intersectObjects(walk.children, false)[0];
            if (!hit) return;
            sim.target = { x: hit.point.x, z: hit.point.z };
        }
        function onWheel(event) {
            if (event.target !== canvas) return;
            event.preventDefault();
            distance = Math.min(MAX_DIST, Math.max(MIN_DIST, distance + Math.sign(event.deltaY) * 0.4));
        }
        function onLost() {
            if (options.onLost) options.onLost();
        }
        const observer = new root.ResizeObserver(function () { resize = true; });
        observer.observe(stage);
        canvas.addEventListener('webglcontextlost', onLost);
        root.addEventListener('keydown', onKey);
        root.addEventListener('keyup', onKey);
        stage.addEventListener('pointerdown', onPointerDown);
        root.addEventListener('pointermove', onPointerMove);
        root.addEventListener('pointerup', onPointerUp);
        stage.addEventListener('wheel', onWheel, { passive: false });
        stage.addEventListener('contextmenu', onContext);
        const api = {
            setPaused: function (value) { paused = !!value; if (paused) keys.clear(); },
            setMuted: function (value) { presentation.audio.setMuted(!!value); },
            muted: function () { return presentation.audio.preferences.muted; },
            unlock: function (event) { return presentation.audio.unlock(event); },
            hold: function (code, down) { if (down) keys.add(code); else keys.delete(code); },
            dispose: function () {
                if (disposed) return;
                disposed = true;
                paused = true;
                root.cancelAnimationFrame(frame);
                observer.disconnect();
                canvas.removeEventListener('webglcontextlost', onLost);
                root.removeEventListener('keydown', onKey);
                root.removeEventListener('keyup', onKey);
                stage.removeEventListener('pointerdown', onPointerDown);
                root.removeEventListener('pointermove', onPointerMove);
                root.removeEventListener('pointerup', onPointerUp);
                stage.removeEventListener('wheel', onWheel);
                stage.removeEventListener('contextmenu', onContext);
                try { presentation.dispose(); } catch (err) { /* already gone */ }
                Object.keys(actors).forEach(function (key) {
                    try { assetsMod.disposeInstance(actors[key].instance); } catch (err) { /* released */ }
                });
                props.forEach(function (item) {
                    try { assetsMod.disposeInstance(item.instance); } catch (err) { /* released */ }
                });
                loaded.forEach(function (asset) {
                    try { assetsMod.releaseAsset(asset); } catch (err) { /* released */ }
                });
                walkMat.dispose();
                renderer.dispose();
            }
        };
        signal.addEventListener('abort', function () { api.dispose(); });
        if (signal.aborted) return api;
        paused = false;
        if (options.onReady) options.onReady();
        frame = root.requestAnimationFrame(frameOf);
        return api;
    }

    root.LeuchtfeuerView = { mount: mount, PACK: PACK, RUNTIME: RUNTIME };
})(typeof window !== 'undefined' ? window : globalThis);
