(function () {
    'use strict';

    let catalogPromise, scriptPromise, personaPromise;
    const versioned = path => path + '?v=' + encodeURIComponent(window.BUILD_VERSION || window.AURAGO_BUILD_VERSION || 'dev');
    const clamp = value => Number.isFinite(value) ? Math.max(0, Math.min(1, value)) : 0;

    function catalog() {
        if (!catalogPromise) catalogPromise = fetch(versioned('/img/personas/animated/catalog.json'), { credentials: 'same-origin' })
            .then(response => {
                if (!response.ok) throw new Error('Persona catalog unavailable');
                return response.json();
            }).then(data => data.personas.filter(entry => entry && /^[a-z]+$/.test(entry.key) &&
                /^[a-z]+\.riv$/.test(entry.file) && typeof entry.artboard === 'string'))
            .catch(error => { catalogPromise = null; throw error; });
        return catalogPromise;
    }

    function initialPersona() {
        if (window._activePersonaIconKey) return Promise.resolve(window._activePersonaIconKey);
        if (!personaPromise) personaPromise = fetch('/api/personalities', { credentials: 'same-origin', cache: 'no-store' })
            .then(response => {
                if (!response.ok) throw new Error('Personality unavailable');
                return response.json();
            }).then(data => {
                const entry = (data.personalities || []).find(item => item && item.name === data.active);
                return entry && entry.core ? String(entry.name).toLowerCase() : 'custom';
            }).finally(() => { personaPromise = null; });
        return personaPromise;
    }

    function riveRuntime() {
        if (!scriptPromise) scriptPromise = new Promise((resolve, reject) => {
            const script = document.createElement('script');
            script.src = versioned('/js/vendor/rive/rive.js');
            script.onload = () => resolve(window.rive);
            script.onerror = () => { script.remove(); reject(new Error('Persona runtime unavailable')); };
            document.head.appendChild(script);
        }).then(rive => {
            rive.RuntimeLoader.setWasmUrl(versioned('/js/vendor/rive/rive.wasm'));
            rive.RuntimeLoader.setWasmFallbackUrl(null);
            return rive;
        }).catch(error => { scriptPromise = null; throw error; });
        return scriptPromise;
    }

    function mount(host, options) {
        const runtime = options.runtime;
        const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
        let visible = options.visible !== false, inView = !window.IntersectionObserver;
        let disposed = false, generation = 0, loading = false, attempted = false;
        let key = window._activePersonaIconKey || null, player = null, inputs = null, entry = null;
        let frame = 0, loadTimer = 0, lastTime = 0, mouth = 0, lastVoice = -Infinity, lastAdapter = null;
        let peak = 0.12, quietSince = 0, pose = 0, poseSince = 0;
        let nextExpression = 0, expressionUntil = 0, expressionMode = 0;
        host.innerHTML = '<img alt="" decoding="async"><canvas aria-hidden="true"></canvas>';
        const poster = host.querySelector('img'), canvas = host.querySelector('canvas');
        const fallback = versioned('/img/personas/custom.png');
        poster.src = fallback;
        poster.onerror = () => { if (poster.getAttribute('src') !== fallback) poster.src = fallback; };
        host.dataset.animated = 'false';

        function activeView() { return !disposed && visible && inView && !document.hidden && !motion.matches && host.isConnected; }
        function reset() {
            mouth = 0;
            lastVoice = -Infinity;
            lastTime = 0;
            peak = 0.12;
            quietSince = 0;
            pose = 0;
            poseSince = 0;
            nextExpression = expressionUntil = 0;
            if (inputs) { inputs.mode.value = 0; inputs.mouthOpen.value = 0; inputs.viseme.value = 0; inputs.headTilt.value = 0; }
        }
        function stopFrames() {
            window.cancelAnimationFrame(frame);
            frame = 0;
            reset();
            host.dataset.animated = 'false';
            if (player) player.pause();
        }
        function destroyPlayer() {
            stopFrames();
            window.clearTimeout(loadTimer);
            loadTimer = 0;
            inputs = null;
            if (player) player.cleanup();
            player = null;
        }
        function resize() {
            if (player && inputs) player.resizeDrawingSurfaceToCanvas(Math.min(2, window.devicePixelRatio || 1));
        }
        function outputLevel() {
            const adapter = runtime.adapter;
            if (!adapter) return 0;
            try {
                if (typeof adapter.getOutputLevel === 'function') return clamp(adapter.getOutputLevel());
                return adapter.player && typeof adapter.player.getOutputLevel === 'function' ? clamp(adapter.player.getOutputLevel()) : 0;
            } catch (_) { return 0; }
        }
        function tick(now) {
            frame = 0;
            if (!host.isConnected) { dispose(); return; }
            if (!activeView() || !inputs) return;
            if (lastAdapter !== runtime.adapter) { reset(); lastAdapter = runtime.adapter; }
            const state = runtime.state || 'idle';
            const blocked = !runtime.sessionId || runtime.userSpeaking || ['idle', 'closed', 'connecting', 'reconnecting', 'parked', 'error'].includes(state);
            const level = blocked ? 0 : outputLevel();
            const dt = lastTime ? Math.min(100, now - lastTime) : 16;
            lastTime = now;
            peak = Math.max(0.08, level, peak * Math.exp(-dt / 700));
            const target = level > 0.012 ? clamp((level / peak - 0.18) / 0.82) : 0;
            mouth = blocked ? 0 : mouth + (target - mouth) * (1 - Math.exp(-dt / (target > mouth ? 20 : 28)));
            quietSince = target ? 0 : quietSince || now;
            if (target > 0.08) lastVoice = now;
            if (blocked || (quietSince && now - quietSince >= 32)) mouth = 0;
            const speaking = !blocked && (mouth > 0.08 || now - lastVoice < 140);
            const restMode = runtime.sessionId && ['listening', 'speaking', 'executing'].includes(state)
                ? (runtime.actionActive ? 2 : 1) : 0;
            // Brief idle expressions are decorative; conversation activity always takes priority.
            const canExpress = !speaking && !runtime.userSpeaking && !runtime.providerSpeaking && !runtime.actionActive &&
                ['idle', 'closed', 'listening'].includes(state);
            if (!canExpress) nextExpression = expressionUntil = 0;
            else if (!nextExpression) nextExpression = now + 8000 + Math.random() * 8000;
            else if (now >= nextExpression) {
                const expressions = restMode === 1 ? [2, 4] : [1, 2, 4];
                expressionMode = expressions[Math.floor(Math.random() * expressions.length)];
                expressionUntil = now + 1200 + Math.random() * 1000;
                nextExpression = expressionUntil + 8000 + Math.random() * 8000;
            }
            inputs.mode.value = speaking ? 3 : canExpress && now < expressionUntil ? expressionMode : restMode;
            inputs.mouthOpen.value = mouth > 0.08 ? mouth : 0;
            // mouthOpen only gates these discrete assets; it does not morph an AA mouth.
            // ponytail: energy selects narrow/rounded/wide poses, not phonemes; use timed cues when available.
            const nextPose = mouth <= 0.08 ? 0 : entry.speechStyle === 'human-visemes'
                ? (mouth < 0.22 ? 4 : mouth < 0.48 ? 3 : mouth < 0.75 ? 2 : 1)
                : Math.min(4, 1 + Math.floor(mouth * 4));
            if (nextPose === 0 || pose === 0 || now - poseSince >= 60) {
                if (pose !== nextPose) poseSince = now;
                pose = nextPose;
            }
            inputs.viseme.value = pose;
            inputs.headTilt.value = Math.sin(now / 2300) * 0.14;
            frame = window.requestAnimationFrame(tick);
        }
        async function load() {
            const token = ++generation;
            loading = true;
            attempted = true;
            try {
                const [entries, resolved] = await Promise.all([catalog(), key ? Promise.resolve(key) : initialPersona()]);
                if (disposed || token !== generation) return;
                key = window._activePersonaIconKey || resolved;
                entry = entries.find(item => item.key === key) || null;
                host.dataset.persona = entry ? entry.key : 'custom';
                poster.src = entry ? versioned('/img/personas/' + entry.key + '.png') : fallback;
                if (!entry || !activeView()) return;
                const rive = await riveRuntime();
                if (disposed || token !== generation || !activeView()) return;
                const fail = () => {
                    if (disposed || token !== generation) return;
                    ++generation;
                    loading = false;
                    destroyPlayer();
                };
                // Also bounds runtimes that never call onLoadError after a WASM failure.
                loadTimer = window.setTimeout(fail, 15000);
                player = new rive.Rive({
                    src: versioned('/img/personas/animated/' + entry.file), canvas, artboard: entry.artboard,
                    stateMachine: 'VoicePersona', autoplay: false, enableRiveAssetCDN: false,
                    layout: new rive.Layout({ fit: rive.Fit.Contain, alignment: rive.Alignment.Center }),
                    onLoad() {
                        if (disposed || token !== generation) return;
                        const found = Object.fromEntries(player.stateMachineInputs('VoicePersona').map(input => [input.name, input]));
                        if (!['mode', 'viseme', 'mouthOpen', 'headTilt'].every(name => found[name])) { fail(); return; }
                        inputs = found;
                        loading = false;
                        window.clearTimeout(loadTimer);
                        reset();
                        resize();
                        syncVisibility();
                    },
                    onAdvance() {
                        if (token === generation && inputs && activeView()) host.dataset.animated = 'true';
                    },
                    onLoadError: fail
                });
            } catch (_) {
                if (!disposed && token === generation) destroyPlayer();
            } finally {
                if (token === generation && !player) loading = false;
            }
        }
        function syncVisibility() {
            if (disposed) return;
            if (!activeView()) {
                if (loading) { ++generation; loading = false; attempted = false; destroyPlayer(); }
                else stopFrames();
                // Resolve the selected PNG even when animation is disabled by accessibility settings.
                if (visible && inView && !document.hidden && motion.matches && !attempted) void load();
                return;
            }
            if (inputs) {
                resize();
                player.play('VoicePersona');
                if (!frame) frame = window.requestAnimationFrame(tick);
            } else if (!attempted) void load();
        }
        function onPersona(event) {
            const next = event.detail && event.detail.key || 'custom';
            if (next === key) return;
            key = next;
            ++generation;
            loading = false;
            attempted = false;
            destroyPlayer();
            poster.src = fallback;
            syncVisibility();
        }
        function onState() {
            if (runtime.userSpeaking || !runtime.sessionId || ['idle', 'closed', 'connecting', 'reconnecting', 'parked', 'error'].includes(runtime.state)) reset();
            syncVisibility();
        }
        function onMotion() {
            if (!inputs && !loading) attempted = false;
            syncVisibility();
        }
        const intersection = window.IntersectionObserver ? new IntersectionObserver(entries => {
            inView = entries.some(item => item.isIntersecting);
            syncVisibility();
        }) : null;
        const observer = window.ResizeObserver ? new ResizeObserver(resize) : null;
        if (intersection) intersection.observe(host);
        if (observer) observer.observe(host);
        window.addEventListener('aurago:persona-icon-change', onPersona);
        document.addEventListener('visibilitychange', syncVisibility);
        motion.addEventListener('change', onMotion);
        runtime.addEventListener('state', onState);
        runtime.addEventListener('error', reset);
        function dispose() {
            if (disposed) return;
            disposed = true;
            ++generation;
            destroyPlayer();
            if (intersection) intersection.disconnect();
            if (observer) observer.disconnect();
            window.removeEventListener('aurago:persona-icon-change', onPersona);
            document.removeEventListener('visibilitychange', syncVisibility);
            motion.removeEventListener('change', onMotion);
            runtime.removeEventListener('state', onState);
            runtime.removeEventListener('error', reset);
            poster.onerror = null;
            host.innerHTML = '';
        }
        syncVisibility();
        return { setVisible(value) {
            visible = !!value;
            if (!visible && !inputs) attempted = false;
            syncVisibility();
        }, dispose };
    }

    window.AuraRealtimeSpeechAvatar = { mount };
})();
