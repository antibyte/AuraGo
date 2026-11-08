(function () {
    'use strict';

    const choices = {
        idle: ['idle-subtle', 'idle-a', 'idle-listening', 'idle-foldarms', 'idle-talking'],
        waving: ['greeting'], review: ['confused', 'idle-foldarms'],
        waiting: ['idle-listening', 'tired-hunched', 'shiver'], running: ['interact', 'idle-talking'],
        'running-left': ['walk-female', 'walk-happy', 'jog', 'run-female'], 'running-right': ['walk-female', 'walk-happy', 'jog', 'run-female'],
        jumping: ['jump-2', 'victory-fist-pump'], failed: ['reject', 'confused'],
        dancing: ['dance-simple', 'dance-arm-pump', 'dab'], sitting: ['sit-ground-idle'],
        sleeping: ['sleeping'], nodding: ['head-nod', 'yes'], clapping: ['clapping'],
        stretching: ['idle-foldarms', 'idle-listening'], playing: ['guitar-playing'],
        thinking: ['confused', 'idle-foldarms'], working: ['interact'], editing: ['fixing-kneeling'],
        testing: ['idle-listening'], success: ['clapping', 'head-nod'],
        error: ['reject', 'confused'], celebrating: ['victory-fist-pump', 'dance-arm-pump'],
        dragging: ['ledge-hang'], falling: ['freefall'], landing: ['jump-land']
    };
    const random = list => list[Math.floor(Math.random() * list.length)];
    const clamp = (v, low, high) => Math.max(low, Math.min(high, v));

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
        const read = async (name, maxBytes) => {
            const response = await fetch(versionedURL('/3d/pets/punk/' + name), { signal, credentials: 'same-origin' });
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
        let state = 'idle';
        let disposed = false;
        let frame = null;
        let last = 0;
        let elapsed = 0;
        let frames = 0;
        let nextIdle = 8;
        let pointerX = window.innerWidth / 2;
        let pointerY = window.innerHeight / 2;
        let gazeX = 0, gazeY = 0, yaw = .12;
        let dragVelocity = 0;
        const reduced = window.matchMedia('(prefers-reduced-motion: reduce)');
        const motionReduced = () => reduced.matches || document.body.dataset.animations === 'false';
        const parentQ = new T.Quaternion(), deltaQ = new T.Quaternion(), localQ = new T.Quaternion();
        const axisY = new T.Vector3(0, 1, 0), axisX = new T.Vector3(1, 0, 0);

        function play(next, explicitClip) {
            if (disposed) return;
            state = next;
            const id = explicitClip || random(choices[next] || choices.idle);
            const entry = clips.get(id) || clips.get('idle-subtle');
            if (!entry) return;
            const nextAction = mixer.clipAction(entry.clip);
            if (action !== nextAction) {
                nextAction.reset().setEffectiveWeight(1).setEffectiveTimeScale(1);
                nextAction.setLoop(entry.loop || ['idle', 'running-left', 'running-right', 'dragging', 'falling'].includes(next) ? T.LoopRepeat : T.LoopOnce, Infinity);
                nextAction.clampWhenFinished = true;
                nextAction.play();
                if (action) action.crossFadeTo(nextAction, .28, false);
                action = nextAction;
            } else {
                nextAction.reset().play();
            }
            renderer.domElement.dataset.clip = id;
            host.dataset.motion = next;
            nextIdle = elapsed + 8 + Math.random() * 7;
            if (!frame && !document.hidden) { last = 0; frame = requestAnimationFrame(tick); }
        }

        function aim(bone, horizontal, vertical) {
            if (!bone) return;
            bone.parent.getWorldQuaternion(parentQ);
            deltaQ.setFromAxisAngle(axisY, horizontal);
            deltaQ.multiply(localQ.setFromAxisAngle(axisX, vertical));
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
            if (!quiet && state === 'idle' && elapsed > nextIdle) play('idle');
            const rect = host.getBoundingClientRect();
            const nextRatio = Math.min(3, (window.devicePixelRatio || 1) * Math.max(.5, scale()));
            if (nextRatio !== pixelRatio) { pixelRatio = nextRatio; renderer.setPixelRatio(pixelRatio); }
            const horizontal = quiet ? 0 : clamp((pointerX - rect.left - rect.width / 2) / Math.max(160, window.innerWidth * .32), -1, 1);
            const vertical = quiet ? 0 : clamp((pointerY - rect.top - rect.height * .2) / Math.max(160, window.innerHeight * .4), -1, 1);
            gazeX = T.MathUtils.damp(gazeX, horizontal * .38, 7, dt);
            gazeY = T.MathUtils.damp(gazeY, vertical * .18, 7, dt);
            const moving = state === 'running-left' || state === 'running-right';
            yaw = T.MathUtils.damp(yaw, moving ? (state === 'running-left' ? -1.35 : 1.35) : .12, 6, dt);
            model.rotation.y = yaw;
            model.rotation.z = state === 'dragging' && !quiet ? clamp(dragVelocity * -.00008, -.14, .14) : 0;
            model.updateMatrixWorld(true);
            if (!quiet && !['sleeping', 'falling'].includes(state)) { aim(head, gazeX, gazeY); aim(chest, gazeX * .12, 0); }
            renderer.render(scene, camera);
            frames++;
            if (!quiet && !frame) frame = requestAnimationFrame(tick);
        }

        function pointer(event) { pointerX = event.clientX; pointerY = event.clientY; }
        function visibility() { if (frame) cancelAnimationFrame(frame); frame = null; last = 0; if (!document.hidden && !disposed) frame = requestAnimationFrame(tick); }
        function contextLost(event) { event.preventDefault(); dispose(); onFailure(); }
        function dispose() {
            if (disposed) return;
            disposed = true;
            if (frame) cancelAnimationFrame(frame);
            document.removeEventListener('pointermove', pointer);
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
            duration(next) { const item = clips.get((choices[next] || choices.idle)[0]); return item ? item.clip.duration * 1000 : 2000; },
            inspect() {
                return { state, clip: renderer.domElement.dataset.clip, time: action ? action.time : 0, frames,
                    bones: bones.size, clips: clips.size, triangles: renderer.info.render.triangles,
                    reducedMotion: motionReduced(), gaze: [gazeX, gazeY],
                    gazeAngle: head ? head.quaternion.angleTo(headBase) : 0,
                    headWorld: head ? head.getWorldPosition(new T.Vector3()).toArray() : [],
                    pelvisWorld: bones.get('defspine').getWorldPosition(new T.Vector3()).toArray(),
                    skinTip: skinTip(), disposed };
            }
        };
    }
    window.AuraPunkPet3D = { create };
})();
