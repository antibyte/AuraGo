/* Runenpfad Web Audio controller powered by Game Maker aurago-sounds pack. */
(function (root) {
    'use strict';

    const BASE_URL = '/api/game-maker/asset-packs/aurago-sounds/';
    const REQUIRED_SOUNDS = [
        'forest-day', 'forest-night', 'wind-gentle', 'wind-strong', 'industrial',
        'step-grass', 'step-stone', 'step-gravel', 'step-wood', 'step-metal',
        'jump', 'land', 'pistol', 'laser', 'impact-flesh', 'impact-metal',
        'explosion-small', 'crate-break', 'door-wood', 'pickup', 'ui-click',
        'victory', 'defeat'
    ];

    function validateSoundManifest(manifest) {
        if (!manifest || !Array.isArray(manifest.assets)) {
            throw new Error('runenpfad audio manifest missing');
        }
        const byId = {};
        manifest.assets.forEach(function (asset) {
            if (asset && asset.id && Array.isArray(asset.files) && asset.files[0] && asset.files[0].file) {
                byId[asset.id] = asset;
            }
        });
        REQUIRED_SOUNDS.forEach(function (id) {
            if (!byId[id]) {
                throw new Error('runenpfad audio missing ' + id);
            }
        });
        return byId;
    }

    function createAudioController(options) {
        options = options || {};
        const assetsById = validateSoundManifest(options.manifest);
        const signal = options.signal;
        const base = options.baseUrl || BASE_URL;

        let enabled = false;
        let active = true;
        let disposed = false;
        let ctx = null;
        let currentAmbienceId = '';
        let ambienceSource = null;
        let ambienceGain = null;
        const buffers = new Map();
        const pendingLoads = new Map();
        const lastPlayTime = new Map();

        function ensureContext() {
            if (disposed || !enabled || !active) return null;
            if (!ctx) {
                const AudioCtx = root.AudioContext || root.webkitAudioContext;
                if (!AudioCtx) return null;
                ctx = new AudioCtx();
            }
            if (ctx.state === 'suspended') {
                ctx.resume().catch(function () {});
            }
            return ctx;
        }

        async function loadBuffer(id) {
            if (disposed || !assetsById[id]) return null;
            if (buffers.has(id)) return buffers.get(id);
            if (pendingLoads.has(id)) return pendingLoads.get(id);
            const audioCtx = ensureContext();
            if (!audioCtx) return null;

            const file = assetsById[id].files[0].file;
            const promise = fetch(base + file, { signal: signal, credentials: 'same-origin' })
                .then(function (resp) {
                    if (!resp.ok) throw new Error('audio fetch ' + id);
                    return resp.arrayBuffer();
                })
                .then(function (arr) {
                    if (disposed) return null;
                    return audioCtx.decodeAudioData(arr);
                })
                .then(function (decoded) {
                    if (decoded && !disposed) buffers.set(id, decoded);
                    pendingLoads.delete(id);
                    return decoded;
                })
                .catch(function () {
                    pendingLoads.delete(id);
                    return null;
                });
            pendingLoads.set(id, promise);
            return promise;
        }

        function stopAmbience() {
            if (ambienceSource) {
                try { ambienceSource.stop(); } catch (err) { /* stopped */ }
                try { ambienceSource.disconnect(); } catch (err) { /* disconnected */ }
                ambienceSource = null;
            }
            if (ambienceGain) {
                try { ambienceGain.disconnect(); } catch (err) { /* disconnected */ }
                ambienceGain = null;
            }
        }

        async function setAmbience(id) {
            currentAmbienceId = id || '';
            stopAmbience();
            if (!enabled || !active || disposed || !currentAmbienceId || !assetsById[currentAmbienceId]) return;
            const wanted = currentAmbienceId;
            const buffer = await loadBuffer(wanted);
            const audioCtx = ensureContext();
            if (!buffer || !audioCtx || disposed || !enabled || !active || currentAmbienceId !== wanted) return;
            const source = audioCtx.createBufferSource();
            source.buffer = buffer;
            source.loop = true;
            const gain = audioCtx.createGain();
            gain.gain.value = (Number(assetsById[wanted].gain) || 0.5) * 0.25;
            source.connect(gain);
            gain.connect(audioCtx.destination);
            source.start(0);
            ambienceSource = source;
            ambienceGain = gain;
        }

        function playCue(id, gainScale, cooldownMs) {
            if (disposed || !enabled || !active || !assetsById[id]) return;
            const now = Date.now();
            const minGap = cooldownMs || 65;
            if ((lastPlayTime.get(id) || 0) + minGap > now) return;
            lastPlayTime.set(id, now);
            const audioCtx = ensureContext();
            if (!audioCtx) return;

            loadBuffer(id).then(function (buffer) {
                if (!buffer || disposed || !enabled || !active) return;
                const source = audioCtx.createBufferSource();
                source.buffer = buffer;
                const gain = audioCtx.createGain();
                gain.gain.value = Math.min(1, (Number(assetsById[id].gain) || 0.7) * (gainScale || 0.5));
                source.connect(gain);
                gain.connect(audioCtx.destination);
                source.start(0);
            });
        }

        function handleEvents(events, worldAmbience) {
            if (!Array.isArray(events) || !enabled || !active) return;
            events.forEach(function (ev) {
                if (!ev) return;
                if (ev.type === 'step') playCue(ev.detail && ev.detail.sound ? ev.detail.sound : 'step-grass', 0.22, 220);
                else if (ev.type === 'jump') playCue('jump', 0.4, 90);
                else if (ev.type === 'land') playCue('land', 0.35, 110);
                else if (ev.type === 'attack_melee') playCue('laser', 0.28, 120);
                else if (ev.type === 'attack_ranged') playCue('pistol', 0.35, 100);
                else if (ev.type === 'crate_break') playCue('crate-break', 0.5, 80);
                else if (ev.type === 'enemy_hit') playCue((ev.detail && ev.detail.sound) || 'impact-flesh', 0.45, 70);
                else if (ev.type === 'enemy_kill' || ev.type === 'boss_defeat') playCue('explosion-small', 0.55, 100);
                else if (ev.type === 'player_hurt') playCue('impact-metal', 0.5, 200);
                else if (ev.type === 'pickup_coin' || ev.type === 'pickup_gem') playCue('pickup', 0.42, 60);
                else if (ev.type === 'pickup_key' || ev.type === 'checkpoint') playCue('door-wood', 0.55, 150);
                else if (ev.type === 'portal_win') playCue('victory', 0.65, 500);
                else if (ev.type === 'player_defeat') playCue('defeat', 0.6, 500);
            });
            if (worldAmbience && currentAmbienceId !== worldAmbience && !ambienceSource) {
                setAmbience(worldAmbience);
            }
        }

        function syncPlaybackState() {
            if (disposed) return;
            if (!enabled || !active) {
                stopAmbience();
                if (ctx && ctx.state === 'running') {
                    ctx.suspend().catch(function () {});
                }
            } else {
                ensureContext();
                if (currentAmbienceId && !ambienceSource) {
                    setAmbience(currentAmbienceId);
                }
            }
        }

        function setEnabled(next) {
            enabled = !!next;
            syncPlaybackState();
            if (enabled && active) {
                playCue('ui-click', 0.35, 50);
            }
        }

        function setActive(next) {
            active = !!next;
            syncPlaybackState();
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            stopAmbience();
            buffers.clear();
            pendingLoads.clear();
            if (ctx) {
                ctx.close().catch(function () {});
                ctx = null;
            }
        }

        return {
            isEnabled: function () { return enabled; },
            setEnabled: setEnabled,
            setActive: setActive,
            setAmbience: setAmbience,
            playUiClick: function () { playCue('ui-click', 0.35, 50); },
            handleEvents: handleEvents,
            dispose: dispose
        };
    }

    root.RunenpfadAudio = {
        BASE_URL: BASE_URL,
        REQUIRED_SOUNDS: REQUIRED_SOUNDS,
        validateSoundManifest: validateSoundManifest,
        createAudioController: createAudioController
    };
})(typeof window !== 'undefined' ? window : globalThis);
