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
    const PRELOAD_SFX = [
        'ui-click', 'jump', 'land', 'step-grass', 'laser', 'pistol',
        'pickup', 'crate-break', 'impact-flesh', 'impact-metal'
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

    function playFallbackTone(audioCtx, id, volume) {
        if (!audioCtx || audioCtx.state !== 'running') return;
        try {
            const now = audioCtx.currentTime;
            const osc = audioCtx.createOscillator();
            const gain = audioCtx.createGain();
            let fStart = 320;
            let fEnd = 220;
            let dur = 0.08;
            let type = 'triangle';
            if (id === 'jump') { fStart = 240; fEnd = 520; dur = 0.11; }
            else if (id === 'land') { fStart = 150; fEnd = 75; dur = 0.06; }
            else if (id === 'pickup') { fStart = 587; fEnd = 880; dur = 0.12; type = 'sine'; }
            else if (id === 'laser' || id === 'pistol') { fStart = 460; fEnd = 180; dur = 0.09; type = 'sawtooth'; }
            else if (id === 'victory') { fStart = 523; fEnd = 1046; dur = 0.25; type = 'sine'; }
            else if (id === 'defeat') { fStart = 280; fEnd = 110; dur = 0.25; }
            else if (id === 'ui-click') { fStart = 660; fEnd = 440; dur = 0.04; type = 'sine'; }
            else if (id.indexOf('step-') === 0) { fStart = 130; fEnd = 90; dur = 0.035; }
            osc.type = type;
            osc.frequency.setValueAtTime(fStart, now);
            osc.frequency.exponentialRampToValueAtTime(Math.max(40, fEnd), now + dur);
            const amp = Math.max(0.02, Math.min(0.28, (volume || 0.4) * 0.25));
            gain.gain.setValueAtTime(amp, now);
            gain.gain.exponentialRampToValueAtTime(0.001, now + dur);
            osc.connect(gain);
            gain.connect(audioCtx.destination);
            osc.start(now);
            osc.stop(now + dur + 0.01);
        } catch (err) { /* fallback tone suppressed */ }
    }

    function createAudioController(options) {
        options = options || {};
        const assetsById = validateSoundManifest(options.manifest);
        const signal = options.signal;
        const base = options.baseUrl || BASE_URL;

        let enabled = options.enabled !== undefined ? !!options.enabled : true;
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
            if (disposed || !enabled) return null;
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
            gain.gain.value = (Number(assetsById[wanted].gain) || 0.6) * 0.42;
            source.connect(gain);
            gain.connect(audioCtx.destination);
            source.start(0);
            ambienceSource = source;
            ambienceGain = gain;
        }

        function playBuffer(audioCtx, buffer, id, gainScale) {
            const source = audioCtx.createBufferSource();
            source.buffer = buffer;
            const gain = audioCtx.createGain();
            gain.gain.value = Math.min(1, (Number(assetsById[id].gain) || 0.75) * (gainScale || 0.75));
            source.connect(gain);
            gain.connect(audioCtx.destination);
            source.start(0);
        }

        function playCue(id, gainScale, cooldownMs, allowWhilePaused) {
            if (disposed || !enabled || (!active && !allowWhilePaused) || !assetsById[id]) return;
            const now = Date.now();
            const minGap = cooldownMs || 65;
            if ((lastPlayTime.get(id) || 0) + minGap > now) return;
            lastPlayTime.set(id, now);
            const audioCtx = ensureContext();
            if (!audioCtx) return;

            const cached = buffers.get(id);
            if (cached) {
                playBuffer(audioCtx, cached, id, gainScale);
                return;
            }
            playFallbackTone(audioCtx, id, gainScale);
            loadBuffer(id).then(function (buffer) {
                if (!buffer || disposed || !enabled || (!active && !allowWhilePaused)) return;
                if (!lastPlayTime.get(id) || Date.now() - now < 250) {
                    playBuffer(audioCtx, buffer, id, gainScale);
                }
            });
        }

        function unlock() {
            if (disposed || !enabled) return;
            const audioCtx = ensureContext();
            if (!audioCtx) return;
            if (audioCtx.state === 'suspended') {
                audioCtx.resume().then(function () {
                    if (active && currentAmbienceId && !ambienceSource) {
                        setAmbience(currentAmbienceId);
                    }
                }).catch(function () {});
            }
            PRELOAD_SFX.forEach(function (id) { loadBuffer(id); });
            if (active && currentAmbienceId && !ambienceSource) {
                setAmbience(currentAmbienceId);
            }
        }

        function handleEvents(events, worldAmbience) {
            if (!Array.isArray(events) || !enabled || !active) return;
            events.forEach(function (ev) {
                if (!ev) return;
                if (ev.type === 'step') playCue(ev.detail && ev.detail.sound ? ev.detail.sound : 'step-grass', 0.38, 220);
                else if (ev.type === 'jump') playCue('jump', 0.58, 90);
                else if (ev.type === 'land') playCue('land', 0.50, 110);
                else if (ev.type === 'attack_melee') playCue('laser', 0.45, 120);
                else if (ev.type === 'attack_ranged') playCue('pistol', 0.52, 100);
                else if (ev.type === 'crate_break') playCue('crate-break', 0.65, 80);
                else if (ev.type === 'enemy_hit') playCue((ev.detail && ev.detail.sound) || 'impact-flesh', 0.60, 70);
                else if (ev.type === 'enemy_kill' || ev.type === 'boss_defeat') playCue('explosion-small', 0.72, 100);
                else if (ev.type === 'player_hurt') playCue('impact-metal', 0.68, 200);
                else if (ev.type === 'pickup_coin' || ev.type === 'pickup_gem') playCue('pickup', 0.58, 60);
                else if (ev.type === 'pickup_key' || ev.type === 'checkpoint') playCue('door-wood', 0.68, 150);
                else if (ev.type === 'portal_win') playCue('victory', 0.80, 500);
                else if (ev.type === 'player_defeat') playCue('defeat', 0.75, 500);
            });
            if (worldAmbience && currentAmbienceId !== worldAmbience && !ambienceSource) {
                setAmbience(worldAmbience);
            }
        }

        function syncPlaybackState() {
            if (disposed) return;
            if (!enabled) {
                stopAmbience();
                if (ctx && ctx.state === 'running') {
                    ctx.suspend().catch(function () {});
                }
            } else if (!active) {
                stopAmbience();
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
            if (enabled) {
                unlock();
                playCue('ui-click', 0.5, 50, true);
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
            unlock: unlock,
            setAmbience: setAmbience,
            playUiClick: function () { playCue('ui-click', 0.5, 50, true); },
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
