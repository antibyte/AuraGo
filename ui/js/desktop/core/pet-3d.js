(function () {
    'use strict';

    const choices = {
        idle: ['idle-a', 'idle-talking', 'idle-foldarms', 'idle-listening', 'idle-subtle'],
        waving: ['greeting'], review: ['confused', 'idle-foldarms'],
        waiting: ['idle-listening', 'tired-hunched', 'shiver'], running: ['interact', 'idle-talking'],
        'running-left': ['walk-female', 'walk-happy', 'jog', 'run-female'], 'running-right': ['walk-female', 'walk-happy', 'jog', 'run-female'],
        jumping: ['jump-2', 'victory-fist-pump'], failed: ['reject', 'confused'],
        dancing: ['dance-simple', 'dance-arm-pump', 'dab'], sitting: ['sit-ground-idle'],
        sleeping: ['sleeping'], nodding: ['head-nod', 'yes'], clapping: ['clapping'],
        stretching: ['tired-hunched', 'idle-foldarms', 'idle-listening', 'shiver'], playing: ['guitar-playing'],
        thinking: ['confused', 'idle-foldarms'], working: ['interact'], editing: ['fixing-kneeling'],
        testing: ['idle-listening'], success: ['clapping', 'head-nod'],
        error: ['reject', 'confused'], celebrating: ['victory-fist-pump', 'dance-arm-pump'],
        dragging: ['ledge-hang'], falling: ['freefall'], landing: ['jump-land']
    };
    const idleRestClips = ['idle-a', 'idle-talking', 'idle-foldarms', 'idle-listening', 'idle-subtle'];
    const activeIdleBeats = ['greeting', 'head-nod', 'yes', 'clapping', 'victory-fist-pump', 'confused', 'interact', 'dab', 'dance-simple', 'jump-2'];
    const quietIdleBeats = ['dance-simple', 'dance-arm-pump', 'dab', 'guitar-playing', 'jump-2', 'victory-fist-pump', 'clapping', 'fixing-kneeling', 'interact', 'sit-ground-idle', 'confused', 'tired-hunched'];
    const arrivalBeats = ['head-nod', 'yes', 'victory-fist-pump', 'clapping', 'dab', 'greeting', 'confused', 'dance-simple', 'guitar-playing', 'interact'];
    const petHappyBeats = ['clapping', 'victory-fist-pump', 'dance-simple', 'yes', 'greeting', 'dab'];
    const hoverBeats = ['head-nod', 'idle-listening', 'idle-talking', 'yes', 'confused'];
    const swooshBeats = ['confused', 'jump-2', 'shiver'];
    const clickCycle = ['waving', 'celebrating', 'dancing', 'clapping', 'jumping', 'playing', 'nodding'];
    const grooveClips = new Set(['dance-simple', 'dance-arm-pump', 'guitar-playing', 'victory-fist-pump', 'clapping']);
    const clamp = (v, low, high) => Math.max(low, Math.min(high, v));
    const randRange = (low, high) => low + Math.random() * (high - low);

    function disposeModel(model) {
        const textures = new Set();
        model.traverse(object => {
            if (object.geometry) object.geometry.dispose();
            for (const material of Array.isArray(object.material) ? object.material : object.material ? [object.material] : []) {
                for (const value of Object.values(material)) if (value && value.isTexture) textures.add(value);
                material.dispose();
            }
            if (object.skeleton) object.skeleton.dispose();
        });
        textures.forEach(texture => {
            if (texture.image && typeof texture.image.close === 'function') texture.image.close();
            texture.dispose();
        });
    }

    async function create(host, options) {
        const T = window.AuraDriftThree;
        const { signal, versionedURL, onFailure, scale } = options;
        const folder = window.PetRuntime?.modelAssetFolder?.(options.petId);
        if (!folder) throw new Error('Unknown bundled 3D pet');
        const read = async (name, maxBytes) => {
            const response = await fetch(versionedURL('/3d/pets/' + folder + '/' + name), { signal, credentials: 'same-origin' });
            if (!response.ok) throw new Error('Pet asset HTTP ' + response.status);
            const bytes = await response.arrayBuffer();
            if (bytes.byteLength > maxBytes) throw new Error('Pet asset exceeds budget');
            return bytes;
        };
        const [binary, motionBytes] = await Promise.all([read('model.glb', 8 * 1024 * 1024), read('motions.json', 10 * 1024 * 1024)]);
        signal.throwIfAborted();
        const pack = JSON.parse(new TextDecoder().decode(motionBytes));
        if (pack.version !== 1 || !Array.isArray(pack.clips) || pack.clips.length > 64) throw new Error('Invalid pet motion pack');
        const clips = new Map(pack.clips.map(item => [item.id, { clip: T.AnimationClip.parse(item.clip), loop: item.loop }]));
        const gltf = await new T.GLTFLoader().parseAsync(binary, '');
        if (signal.aborted) { disposeModel(gltf.scene); signal.throwIfAborted(); }
        let renderer;
        try { renderer = new T.WebGLRenderer({ alpha: true, antialias: true, powerPreference: 'low-power' }); }
        catch (error) { disposeModel(gltf.scene); throw error; }
        const scene = new T.Scene();
        const model = gltf.scene;
        scene.add(model);
        const camera = new T.OrthographicCamera(-1.02, 1.02, 1.1, -1.1, .1, 20);
        camera.position.set(0, 1.1, 4);
        camera.lookAt(0, 1.1, 0);
        let pixelRatio = Math.min(3, (window.devicePixelRatio || 1) * Math.max(.5, scale()));
        renderer.setPixelRatio(pixelRatio);
        renderer.setSize(192, 208);
        renderer.outputColorSpace = T.SRGBColorSpace;
        renderer.toneMapping = T.ACESFilmicToneMapping;
        renderer.toneMappingExposure = 1.15;
        renderer.setClearColor(0, 0);
        scene.add(new T.HemisphereLight(0xffffff, 0x798093, 2.8));
        const key = new T.DirectionalLight(0xfff3ee, 3.2);
        key.position.set(-2, 3, 4); scene.add(key);
        const rim = new T.DirectionalLight(0xe5d9ff, 2);
        rim.position.set(2, 3, -2); scene.add(rim);
        const mixer = new T.AnimationMixer(model);
        const bones = new Map();
        model.traverse(object => { if (object.isBone) bones.set(object.name.replace(/[^a-z0-9]/gi, '').toLowerCase(), object); });
        const head = bones.get('defspine006');
        const chest = bones.get('defspine003');
        const headBase = head ? head.quaternion.clone() : null;
        const chestBase = chest ? chest.quaternion.clone() : null;
        let skinMesh = null, topVertex = 0;
        model.traverse(object => { if (object.isSkinnedMesh) skinMesh = object; });
        for (let i = 0; i < skinMesh.geometry.attributes.position.count; i++) if (skinMesh.geometry.attributes.position.getY(i) > skinMesh.geometry.attributes.position.getY(topVertex)) topVertex = i;
        const skinTip = () => skinMesh.applyBoneTransform(topVertex, new T.Vector3().fromBufferAttribute(skinMesh.geometry.attributes.position, topVertex)).applyMatrix4(skinMesh.matrixWorld).toArray();
        let action = null;
        let currentClipId = '';
        let currentClipLoop = true;
        let currentClipDuration = 2;
        let state = 'idle';
        let idlePhase = 'rest';
        let activityCount = 0;
        let disposed = false;
        let frame = null;
        let last = 0;
        let elapsed = 0;
        let frames = 0;
        let nextIdle = 2.8;
        let pointerX = window.innerWidth / 2;
        let pointerY = window.innerHeight / 2;
        let hasPointer = false;
        let lastPointerAt = -10;
        let lastUserInputAt = -10;
        let lastPointerSampleAt = -10;
        let lastPointerSampleX = pointerX;
        let lastPointerSampleY = pointerY;
        let lookTargetX = 0, lookTargetY = 0, nextLookAt = 1.6;
        let gazeX = 0, gazeY = 0, gazeRoll = 0, yaw = .12;
        let dragVelocity = 0;
        let hovering = false, hoverEnteredAt = -10, hoverLeftAt = -10, hoverTriggered = false, hoverCooldown = 0;
        let petStrokes = 0, petDir = 0, petLastX = 0, petWindowStart = -10, petCooldown = 0;
        let swooshCooldown = 0;
        let clickIndex = 0, lastClickAt = -20;
        let pendingRoam = null;
        const recentClips = [];
        const recentAmbient = [];
        const reduced = window.matchMedia('(prefers-reduced-motion: reduce)');
        const motionReduced = () => reduced.matches || document.body.dataset.animations === 'false';
        const parentQ = new T.Quaternion(), deltaQ = new T.Quaternion(), localQ = new T.Quaternion();
        const axisY = new T.Vector3(0, 1, 0), axisX = new T.Vector3(1, 0, 0), axisZ = new T.Vector3(0, 0, 1);

        function rememberClip(id) {
            const idx = recentClips.indexOf(id);
            if (idx >= 0) recentClips.splice(idx, 1);
            recentClips.push(id);
            if (recentClips.length > 4) recentClips.shift();
        }

        function pickClip(list, avoidRecent = true) {
            if (!list || !list.length) return 'idle-subtle';
            if (list.length === 1) return list[0];
            const r = Math.random();
            if (r === 0) return list[0];
            const pool = avoidRecent ? list.filter(id => id !== currentClipId && !recentClips.includes(id)) : list.filter(id => id !== currentClipId);
            const candidates = pool.length ? pool : list.filter(id => id !== currentClipId);
            const src = candidates.length ? candidates : list;
            return src[Math.floor(r * src.length)];
        }

        function startClip(id, cfg = {}) {
            const entry = clips.get(id) || clips.get('idle-subtle');
            if (!entry) return false;
            const resolvedId = clips.has(id) ? id : 'idle-subtle';
            const shouldLoop = Boolean(cfg.forceLoop || entry.loop || resolvedId === 'shiver');
            const nextAction = mixer.clipAction(entry.clip);
            nextAction.setLoop(shouldLoop ? T.LoopRepeat : T.LoopOnce, shouldLoop ? Infinity : 1);
            nextAction.clampWhenFinished = !shouldLoop;
            if (action !== nextAction) {
                nextAction.reset().setEffectiveWeight(1).setEffectiveTimeScale(1);
                nextAction.play();
                if (action) action.crossFadeTo(nextAction, cfg.fade ?? .3, false);
                action = nextAction;
            } else if (cfg.restart || !shouldLoop) {
                nextAction.reset().setEffectiveWeight(1).setEffectiveTimeScale(1).play();
            }
            currentClipId = resolvedId;
            currentClipLoop = shouldLoop;
            currentClipDuration = entry.clip.duration || 1.5;
            rememberClip(resolvedId);
            renderer.domElement.dataset.clip = resolvedId;
            return true;
        }

        function scheduleNextIdle(phase) {
            if (phase === 'rest') {
                nextIdle = elapsed + randRange(2.2, 4.4);
                return;
            }
            if (!currentClipLoop) {
                nextIdle = elapsed + Math.max(.8, currentClipDuration + .08);
            } else if (currentClipId === 'shiver') {
                nextIdle = elapsed + randRange(1.1, 1.6);
            } else if (currentClipId === 'guitar-playing' || currentClipId === 'sit-ground-idle' || currentClipId === 'fixing-kneeling') {
                nextIdle = elapsed + randRange(4.0, 6.4);
            } else {
                nextIdle = elapsed + randRange(2.8, 4.8);
            }
        }

        function playIdleBeat(pool, fade = .32) {
            const id = pickClip(pool, true);
            idlePhase = 'beat';
            if (startClip(id, { forceLoop: false, fade, restart: true })) {
                activityCount++;
                scheduleNextIdle('beat');
            }
        }

        function playIdleRest(explicitClip, fade = .34) {
            const id = explicitClip || pickClip(idleRestClips, true);
            idlePhase = 'rest';
            if (startClip(id, { forceLoop: true, fade, restart: false })) {
                scheduleNextIdle('rest');
            }
        }

        function play(next, explicitClip) {
            if (disposed) return;
            const prevState = state;
            state = next;
            if (next !== 'running-left' && next !== 'running-right') pendingRoam = null;
            if (next === 'idle') {
                const arrived = (prevState === 'running-left' || prevState === 'running-right') && !explicitClip && !motionReduced() && Math.random() > .32;
                if (arrived) playIdleBeat(arrivalBeats, .28);
                else playIdleRest(explicitClip, prevState === 'dragging' || prevState === 'landing' ? .22 : .32);
            } else {
                idlePhase = 'rest';
                const id = explicitClip
                    || ((next === 'running-left' || next === 'running-right') && pendingRoam?.clip)
                    || pickClip(choices[next] || choices.idle, false);
                const forceLoop = ['running-left', 'running-right', 'dragging', 'falling'].includes(next) || id === 'shiver';
                const fade = ['dragging', 'falling', 'landing'].includes(next) ? .18 : .28;
                startClip(id, { forceLoop, fade, restart: true });
            }
            host.dataset.motion = next;
            if (!frame && !document.hidden) { last = 0; frame = requestAnimationFrame(tick); }
        }

        function aim(bone, horizontal, vertical, roll = 0) {
            if (!bone) return;
            bone.parent.getWorldQuaternion(parentQ);
            deltaQ.setFromAxisAngle(axisY, horizontal);
            deltaQ.multiply(localQ.setFromAxisAngle(axisX, vertical));
            if (roll) deltaQ.multiply(localQ.setFromAxisAngle(axisZ, roll));
            localQ.copy(parentQ).invert().multiply(deltaQ).multiply(parentQ);
            bone.quaternion.premultiply(localQ);
        }

        function tick(now) {
            frame = null;
            if (disposed || document.hidden) return;
            const dt = Math.min(.05, Math.max(0, (now - (last || now)) / 1000));
            if (last && now - last < 1000 / 30 - 1) { frame = requestAnimationFrame(tick); return; }
            last = now; elapsed += dt;
            const quiet = motionReduced();
            // Constant animation tracks may skip writes: remove the previous gaze layer first.
            if (head) head.quaternion.copy(headBase);
            if (chest) chest.quaternion.copy(chestBase);
            mixer.update(quiet ? 0 : dt);
            if (head) headBase.copy(head.quaternion);
            if (chest) chestBase.copy(chest.quaternion);
            if (!quiet && state === 'idle') {
                const beatFinished = idlePhase === 'beat' && !currentClipLoop && action && action.time >= Math.max(.2, currentClipDuration - .1);
                if (beatFinished || elapsed >= nextIdle) {
                    if (idlePhase === 'beat') {
                        playIdleRest(null, .32);
                    } else if (Math.random() < .78) {
                        const pool = (elapsed - lastUserInputAt) < 15 ? activeIdleBeats : quietIdleBeats;
                        playIdleBeat(pool, .32);
                    } else {
                        playIdleRest(null, .34);
                        activityCount++;
                    }
                } else if (hovering && !hoverTriggered && elapsed - hoverEnteredAt >= .28 && elapsed >= hoverCooldown && idlePhase === 'rest') {
                    hoverTriggered = true;
                    hoverCooldown = elapsed + 7;
                    playIdleBeat(hoverBeats, .26);
                }
            } else if (!quiet && !currentClipLoop && !['landing', 'dragging', 'falling'].includes(state) && action && action.time >= Math.max(.2, currentClipDuration - .1)) {
                const follow = pickClip(choices[state] || idleRestClips, false);
                startClip(follow !== currentClipId ? follow : pickClip(idleRestClips, true), { forceLoop: follow === currentClipId, fade: .28, restart: true });
            }
            const rect = host.getBoundingClientRect();
            const nextRatio = Math.min(3, (window.devicePixelRatio || 1) * Math.max(.5, scale()));
            if (nextRatio !== pixelRatio) { pixelRatio = nextRatio; renderer.setPixelRatio(pixelRatio); }
            const pointerActive = hasPointer && (elapsed - lastPointerAt) < 3.2;
            if (!quiet && !pointerActive && elapsed >= nextLookAt) {
                const roll = Math.random();
                if (roll < .22) {
                    lookTargetX = 0; lookTargetY = -.02;
                } else {
                    lookTargetX = randRange(-.32, .32);
                    lookTargetY = randRange(-.12, .1);
                }
                nextLookAt = elapsed + randRange(1.6, 3.8);
            }
            const horizontal = quiet ? 0 : clamp((pointerX - rect.left - rect.width / 2) / Math.max(160, window.innerWidth * .32), -1, 1);
            const vertical = quiet ? 0 : clamp((pointerY - rect.top - rect.height * .2) / Math.max(160, window.innerHeight * .4), -1, 1);
            const targetGazeX = quiet ? 0 : pointerActive ? horizontal * .36 : lookTargetX;
            const targetGazeY = quiet ? 0 : pointerActive ? vertical * .16 : lookTargetY;
            const tiltScale = (state === 'thinking' || state === 'review' || currentClipId === 'confused' || hovering) ? .22 : .14;
            const targetRoll = quiet ? 0 : clamp(-targetGazeX * tiltScale, -.08, .08);
            const gazeRate = pointerActive ? 7 : 3.2;
            gazeX = T.MathUtils.damp(gazeX, targetGazeX, gazeRate, dt);
            gazeY = T.MathUtils.damp(gazeY, targetGazeY, gazeRate, dt);
            gazeRoll = T.MathUtils.damp(gazeRoll, targetRoll, gazeRate, dt);
            const moving = state === 'running-left' || state === 'running-right';
            const grooving = !quiet && (state === 'dancing' || state === 'playing' || state === 'celebrating' || (state === 'idle' && grooveClips.has(currentClipId)));
            let targetYaw = .12;
            if (moving) targetYaw = state === 'running-left' ? -1.35 : 1.35;
            else if (state === 'sleeping') targetYaw = .22;
            else if (state === 'dragging' && !quiet) targetYaw = .08 + clamp(dragVelocity * .00018, -.25, .25);
            else if (!quiet && grooving) targetYaw = .12 + gazeX * .55 + Math.sin(elapsed * 2.6) * .14;
            else if (!quiet) targetYaw = .12 + gazeX * .65 + Math.sin(elapsed * .75) * .07;
            yaw = T.MathUtils.damp(yaw, targetYaw, moving ? 6 : 4.5, dt);
            model.rotation.y = yaw;
            model.rotation.z = state === 'dragging' && !quiet ? clamp(dragVelocity * -.00008, -.14, .14) : 0;
            model.updateMatrixWorld(true);
            if (!quiet && !['sleeping', 'falling'].includes(state) && currentClipId !== 'sleeping') {
                aim(head, gazeX, gazeY, gazeRoll);
                aim(chest, gazeX * .18, gazeY * .05, 0);
            }
            renderer.render(scene, camera);
            frames++;
            if (!quiet && !frame) frame = requestAnimationFrame(tick);
        }

        function pointer(event) {
            const prevX = lastPointerSampleX, prevY = lastPointerSampleY, dtSample = elapsed - lastPointerSampleAt;
            pointerX = event.clientX; pointerY = event.clientY;
            hasPointer = true; lastPointerAt = elapsed; lastUserInputAt = elapsed;
            lastPointerSampleX = event.clientX; lastPointerSampleY = event.clientY; lastPointerSampleAt = elapsed;
            if (disposed || document.hidden || motionReduced() || event.buttons !== 0) { petStrokes = 0; return; }
            const rect = host.getBoundingClientRect();
            if (rect.width <= 0 || rect.height <= 0) return;
            const over = event.clientX >= rect.left - 12 && event.clientX <= rect.right + 12 && event.clientY >= rect.top - 12 && event.clientY <= rect.bottom + 12;
            if (over && !hovering) {
                hovering = true; hoverEnteredAt = elapsed; hoverTriggered = (elapsed - hoverLeftAt) < 4;
                petLastX = event.clientX; petDir = 0; petStrokes = 0; petWindowStart = elapsed;
            } else if (!over && hovering) {
                hovering = false; hoverLeftAt = elapsed; petStrokes = 0; petDir = 0;
            }
            if (over) {
                const dx = event.clientX - petLastX;
                if (Math.abs(dx) >= 10) {
                    const dir = dx > 0 ? 1 : -1;
                    if (elapsed - petWindowStart > 1.4) { petStrokes = 0; petWindowStart = elapsed; petDir = dir; }
                    else if (petDir && dir !== petDir) { petStrokes++; petDir = dir; }
                    else if (!petDir) { petDir = dir; petWindowStart = elapsed; }
                    petLastX = event.clientX;
                    if (petStrokes >= 3 && elapsed >= petCooldown) {
                        petStrokes = 0; petCooldown = elapsed + 3.5; hoverTriggered = true; hoverCooldown = elapsed + 6;
                        if (state === 'idle') playIdleBeat(petHappyBeats, .22);
                        else if ((state === 'sitting' || state === 'sleeping') && typeof options.onActivity === 'function') options.onActivity('celebrating');
                    }
                }
            } else if (state === 'idle' && idlePhase === 'rest' && elapsed > 4 && elapsed >= swooshCooldown && dtSample > .005 && dtSample < .12) {
                const dist = Math.hypot(event.clientX - (rect.left + rect.width / 2), event.clientY - (rect.top + rect.height / 2));
                const speed = Math.hypot(event.clientX - prevX, event.clientY - prevY) / dtSample;
                if (dist < Math.max(110, rect.width * .75) && speed > 1500) {
                    swooshCooldown = elapsed + 9;
                    playIdleBeat(swooshBeats, .2);
                }
            }
        }
        function desktopPointerDown(event) {
            if (disposed || host.contains(event.target)) return;
            pointerX = event.clientX; pointerY = event.clientY;
            hasPointer = true; lastPointerAt = elapsed; lastUserInputAt = elapsed;
        }
        function desktopKeyDown() { if (!disposed) lastUserInputAt = elapsed; }
        function visibility() { if (frame) cancelAnimationFrame(frame); frame = null; last = 0; if (!document.hidden && !disposed) frame = requestAnimationFrame(tick); }
        function contextLost(event) { event.preventDefault(); dispose(); onFailure(); }
        function dispose() {
            if (disposed) return;
            disposed = true;
            if (frame) cancelAnimationFrame(frame);
            document.removeEventListener('pointermove', pointer);
            document.removeEventListener('pointerdown', desktopPointerDown);
            document.removeEventListener('keydown', desktopKeyDown);
            document.removeEventListener('visibilitychange', visibility);
            reduced.removeEventListener('change', visibility);
            motionObserver.disconnect();
            signal.removeEventListener('abort', dispose);
            renderer.domElement.removeEventListener('webglcontextlost', contextLost);
            mixer.stopAllAction(); mixer.uncacheRoot(model);
            disposeModel(model);
            renderer.dispose(); renderer.forceContextLoss(); renderer.domElement.remove();
            host.classList.remove('is-3d-ready');
        }
        document.addEventListener('pointermove', pointer, { passive: true });
        document.addEventListener('pointerdown', desktopPointerDown, { passive: true });
        document.addEventListener('keydown', desktopKeyDown, { passive: true });
        document.addEventListener('visibilitychange', visibility);
        reduced.addEventListener('change', visibility);
        const motionObserver = new MutationObserver(visibility);
        motionObserver.observe(document.body, { attributes: true, attributeFilter: ['data-animations'] });
        renderer.domElement.addEventListener('webglcontextlost', contextLost);
        signal.addEventListener('abort', dispose, { once: true });
        host.appendChild(renderer.domElement);
        host.classList.add('is-3d-ready');
        play('idle', 'idle-subtle');
        return {
            setState: play, dispose,
            setDrag(velocity) { dragVelocity = velocity; if (state !== 'dragging') play('dragging'); },
            duration(next) {
                if ((next === 'running-left' || next === 'running-right') && pendingRoam?.durationMs) return pendingRoam.durationMs;
                const item = (next === state && currentClipId && clips.get(currentClipId)) || clips.get((choices[next] || choices.idle)[0]);
                const base = item ? item.clip.duration * 1000 : 2000;
                if (next === 'dancing') return currentClipId === 'dab' ? Math.max(base, 2100) : Math.max(base * 2.5, 4200);
                if (next === 'playing') return Math.max(base * 1.5, 5200);
                if (next === 'clapping') return Math.max(base * 2, 2800);
                if (next === 'sitting') return Math.max(base * 2, 6200);
                if (next === 'sleeping') return Math.max(base * 4, 6800);
                if (next === 'editing') return Math.max(base, 4200);
                if (next === 'stretching') return Math.max(base, 2400);
                if (next === 'thinking' || next === 'working' || next === 'celebrating') return Math.max(base, 2600);
                return base;
            },
            pickAmbientState(states) {
                if (!Array.isArray(states) || !states.length) return 'waving';
                const r = Math.random();
                if (r === 0) return states[0];
                const userActive = (elapsed - lastUserInputAt) < 20;
                const pool = states.filter(s => !recentAmbient.includes(s) && !(userActive && s === 'sleeping'));
                const src = pool.length ? pool : states;
                const chosen = src[Math.floor(r * src.length)];
                recentAmbient.push(chosen);
                if (recentAmbient.length > 2) recentAmbient.shift();
                return chosen;
            },
            planRoam({ startX, startY, w, h, gravity, floorTop, clamp: clampBounds }) {
                const r = Math.random();
                let clip = 'walk-female', distance = 110, speed = 85, direction = -1, desiredY = startY;
                if (r > 0) {
                    if (r < .42) {
                        clip = Math.random() < .5 ? 'walk-female' : 'walk-happy';
                        distance = randRange(110, 195); speed = 92;
                    } else if (r < .76) {
                        clip = 'jog'; distance = randRange(195, 320); speed = 152;
                    } else {
                        clip = 'run-female'; distance = randRange(270, 430); speed = 225;
                    }
                    const toCursorX = pointerX - (startX + w / 2);
                    const politeGap = Math.max(120, w * .72);
                    if (hasPointer && (elapsed - lastPointerAt) < 12 && Math.abs(toCursorX) > politeGap + 95 && Math.random() < .48) {
                        direction = toCursorX < 0 ? -1 : 1;
                        distance = Math.min(distance, Math.max(95, Math.abs(toCursorX) - politeGap));
                    } else if (startX < 140) {
                        direction = 1;
                    } else if (startX > window.innerWidth - w - 140) {
                        direction = -1;
                    } else {
                        direction = Math.random() < .5 ? -1 : 1;
                    }
                    if (!gravity && Math.random() < .65) {
                        const maxY = Math.max(48, (floorTop || window.innerHeight) - h - 8);
                        desiredY = clamp(startY + randRange(-95, 95), 48, maxY);
                    }
                }
                let target = clampBounds({ x: startX + direction * distance, y: desiredY, w, h });
                if (Math.abs(target.x - startX) < 48) {
                    direction *= -1;
                    target = clampBounds({ x: startX + direction * distance, y: desiredY, w, h });
                }
                if (gravity) target.y = startY;
                const actualDist = Math.hypot(target.x - startX, target.y - startY);
                pendingRoam = { clip, durationMs: Math.round(clamp(actualDist / speed * 1000, 1200, 3200)) };
                return { startX, startY, targetX: target.x, targetY: target.y, deltaX: target.x - startX };
            },
            nextClickAction(current) {
                if (current === 'sleeping' || current === 'sitting') {
                    lastClickAt = elapsed; clickIndex = 1; return 'jumping';
                }
                if (elapsed - lastClickAt > 12) clickIndex = 0;
                lastClickAt = elapsed;
                const chosen = clickCycle[clickIndex % clickCycle.length];
                clickIndex++;
                return chosen;
            },
            inspect() {
                return { state, clip: renderer.domElement.dataset.clip, time: action ? action.time : 0, frames,
                    bones: bones.size, clips: clips.size, triangles: renderer.info.render.triangles,
                    reducedMotion: motionReduced(), gaze: [gazeX, gazeY],
                    gazeAngle: head ? head.quaternion.angleTo(headBase) : 0,
                    headWorld: head ? head.getWorldPosition(new T.Vector3()).toArray() : [],
                    pelvisWorld: bones.get('defspine').getWorldPosition(new T.Vector3()).toArray(),
                    skinTip: skinTip(), idlePhase, activityCount, disposed };
            }
        };
    }
    window.AuraPunkPet3D = { create };
})();
