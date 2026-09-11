(function () {
    'use strict';
    // One viewer per asset browser; the surrounding modal owns disposal.
    function mount(state, host, manifest, model) {
        const { esc, t } = state.context;
        const abort = new AbortController();
        let runtime, asset, instance, renderer, controls, observer, visibleObserver;
        let frame = 0, last = 0, playing = false, visible = true, disposed = false;
        const size = model.bounds.max.map((v, i) => v - model.bounds.min[i]);
        host.innerHTML = `<h3>${esc(model.name)}</h3><p>${esc(model.description)}</p>
            <div class="gm-model-stage" data-model-stage role="img" aria-label="${esc(model.name + ': ' + t('game_maker.model_rotate'))}" tabindex="0"></div>
            <p class="gm-model-measures">${esc(t('game_maker.model_dimensions'))}: ${size.map(v => v.toLocaleString(undefined, { maximumFractionDigits: 2 })).join(' × ')} m · ${model.lods[0].triangles.toLocaleString()} △</p>
            <div class="gm-model-controls">
                <label>${esc(t('game_maker.assets_animation'))}<select data-model-animation><option value="">${esc(t('game_maker.model_static'))}</option>${model.animations.map(a => `<option value="${esc(a.id)}">${esc(a.id)}</option>`).join('')}</select></label>
                <label>${esc(t('game_maker.model_lod'))}<select data-model-lod>${model.lods.map((l, i) => `<option value="${i}">LOD ${i} · ${l.triangles.toLocaleString()} △</option>`).join('')}</select></label>
                <button type="button" data-model-play>${esc(t('game_maker.assets_play'))}</button>
                <button type="button" data-model-pause>${esc(t('game_maker.assets_pause'))}</button>
            </div><p data-model-status role="status">${esc(t('game_maker.loading'))}</p>`;
        const stage = host.querySelector('[data-model-stage]'), status = host.querySelector('[data-model-status]');
        const animation = host.querySelector('[data-model-animation]'), lod = host.querySelector('[data-model-lod]');
        let scene, camera;
        function draw(now) {
            frame = 0;
            if (disposed || !renderer || !instance || !visible || document.hidden) return;
            const seconds = last ? Math.min(.1, (now - last) / 1000) : 0;
            last = now;
            if (playing) {
                runtime.updateInstance(instance, seconds);
                if (!instance.current.loop && instance.time >= instance.current.duration) playing = false;
            }
            renderer.render(scene, camera);
            if (playing) invalidate();
        }
        function invalidate() {
            if (!disposed && !frame && visible && !document.hidden) frame = requestAnimationFrame(draw);
        }
        function visibility() { last = 0; invalidate(); }
        document.addEventListener('visibilitychange', visibility, { signal: abort.signal });
        function play() {
            if (!instance || !animation.value) return;
            try {
                runtime.playAction(instance, animation.value, { restart: true, fade: .12 });
                playing = true; last = 0; invalidate();
            } catch (error) { status.textContent = error.message; }
        }
        host.querySelector('[data-model-play]').addEventListener('click', play, { signal: abort.signal });
        host.querySelector('[data-model-pause]').addEventListener('click', () => { playing = false; }, { signal: abort.signal });
        animation.addEventListener('change', () => {
            playing = false;
            if (animation.value) play();
            else if (instance) { instance.mixers.forEach(m => { m.stopAllAction(); m.update(0); }); invalidate(); }
        }, { signal: abort.signal });
        lod.addEventListener('change', () => {
            if (!instance) return;
            instance.forcedLOD = Number(lod.value);
            runtime.updateInstance(instance, 0); invalidate();
        }, { signal: abort.signal });
        (async () => {
            runtime = await import(state.api.assetPackFileURL('runtime', 'aurago-three-assets-1.js'));
            if (disposed) return;
            asset = await runtime.loadAsset(manifest, model.id,
                new URL(state.api.assetPackFileURL(manifest.id, ''), location.href), { signal: abort.signal });
            if (disposed) { runtime.releaseAsset(asset); asset = null; return; }
            const T = runtime.THREE;
            scene = new T.Scene();
            camera = new T.PerspectiveCamera(36, 1, .005, 5000);
            renderer = new T.WebGLRenderer({ antialias: true, alpha: true });
            renderer.setPixelRatio(Math.min(2, devicePixelRatio));
            renderer.outputColorSpace = T.SRGBColorSpace;
            renderer.toneMapping = T.ACESFilmicToneMapping;
            renderer.toneMappingExposure = 1.1;
            renderer.shadowMap.enabled = true;
            renderer.shadowMap.type = T.PCFSoftShadowMap;
            stage.appendChild(renderer.domElement);
            instance = runtime.createInstance(asset);
            scene.add(instance.root);
            const bounds = new T.Box3().setFromObject(instance.root);
            const center = bounds.getCenter(new T.Vector3()), extent = Math.max(...size, .05);
            camera.position.copy(center).add(new T.Vector3(1.15, .8, 1.8).multiplyScalar(extent));
            camera.lookAt(center);
            scene.add(new T.HemisphereLight(0xd6e7ff, 0x423429, 2.5));
            const key = new T.DirectionalLight(0xffeccd, 3);
            key.position.copy(center).add(new T.Vector3(-1, 2, 1).multiplyScalar(extent));
            key.target.position.copy(center);scene.add(key, key.target);
            key.castShadow = true;key.shadow.mapSize.set(1024, 1024);
            Object.assign(key.shadow.camera, { left: -extent, right: extent, top: extent, bottom: -extent, near: .001, far: extent * 8 });
            key.shadow.bias = -.0002;
            key.shadow.normalBias = extent * .003;
            const ground = new T.Mesh(new T.PlaneGeometry(extent * 6, extent * 6), new T.ShadowMaterial({ opacity: .25 }));
            ground.rotation.x = -Math.PI / 2;ground.position.set(center.x, bounds.min.y - .001, center.z);ground.receiveShadow = true;scene.add(ground);
            controls = new runtime.OrbitControls(camera, renderer.domElement);
            controls.target.copy(center);controls.enableDamping = false;
            controls.minDistance = extent * .25;controls.maxDistance = extent * 8;
            controls.addEventListener('change', invalidate);controls.update();
            stage.addEventListener('keydown', event => {
                if (event.key !== '+' && event.key !== '-') return;
                event.preventDefault();
                camera.position.sub(controls.target).multiplyScalar(event.key === '+' ? .9 : 1.1).add(controls.target);
                controls.update();invalidate();
            }, { signal: abort.signal });
            observer = new ResizeObserver(() => {
                if (disposed) return;
                const width = stage.clientWidth, height = stage.clientHeight;
                if (!width || !height) return;
                renderer.setSize(width, height, false);camera.aspect = width / height;camera.updateProjectionMatrix();invalidate();
            });
            observer.observe(stage);
            visibleObserver = new IntersectionObserver(entries => {
                visible = entries[0].isIntersecting;last = 0;if (visible) invalidate();
            });
            visibleObserver.observe(stage);
            status.textContent = t('game_maker.model_rotate');
            invalidate();
        })().catch(error => {
            if (!disposed && error.name !== 'AbortError') {
                status.textContent = t('game_maker.assets_load_failed') + ': ' + error.message;
                status.setAttribute('role', 'alert');
            }
        });
        return () => {
            if (disposed) return;
            disposed = true;abort.abort();cancelAnimationFrame(frame);
            observer?.disconnect();visibleObserver?.disconnect();controls?.dispose();
            if (instance) runtime.disposeInstance(instance);
            if (asset) runtime.releaseAsset(asset);
            if (scene) scene.traverse(node => {
                if (node.material?.isShadowMaterial) { node.geometry.dispose(); node.material.dispose(); }
                node.shadow?.dispose();
            });
            renderer?.dispose();renderer?.forceContextLoss();
            renderer?.domElement.remove();
        };
    }
    window.GameMakerStudioModels = { mount };
})();
