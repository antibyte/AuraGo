/* Runenpfad Web Audio controller powered by Game Maker aurago-sounds pack. */
(function (root) {
    'use strict';

    const BASE_URL = '/api/game-maker/asset-packs/aurago-sounds/';
    const REQUIRED_SOUNDS = [
        'forest-day', 'forest-night', 'wind-gentle', 'wind-strong', 'industrial',
        'step-grass', 'step-stone', 'step-gravel', 'step-wood', 'step-metal',
        'jump', 'land', 'pistol', 'laser', 'impact-flesh', 'impact-metal',
        'impact-stone', 'explosion-small', 'explosion-large', 'crate-break',
        'door-wood', 'door-metal', 'pickup', 'ui-click', 'victory', 'defeat',
        'rifle', 'shotgun', 'reload', 'electric-spark', 'fire', 'thunder',
        'stream', 'engine', 'space-hum', 'water-splash', 'rain-light', 'rain-heavy'
    ];
    const PRELOAD_SFX = [
        'ui-click', 'jump', 'land', 'step-grass', 'laser', 'rifle',
        'pickup', 'crate-break', 'impact-flesh', 'impact-metal', 'impact-stone',
        'explosion-small', 'electric-spark'
    ];

    // Event -> cue table. `sound: null` means the event carries its own sound id.
    // `rate` is the base playback rate; `vary` adds a small random detune per play.
    const CUE_MAP = {
        step: { sound: null, gain: 0.38, cooldown: 220, vary: 0.14 },
        jump: { sound: 'jump', gain: 0.58, cooldown: 90, vary: 0.1 },
        dash: { sound: 'jump', gain: 0.42, cooldown: 140, rate: 1.32, vary: 0.08 },
        spring: { sound: 'jump', gain: 0.6, cooldown: 90, rate: 1.45, vary: 0.08 },
        land: { sound: 'land', gain: 0.5, cooldown: 110, vary: 0.12 },
        slam: { sound: 'shotgun', gain: 0.78, cooldown: 120, rate: 0.92 },
        slam_land: { sound: 'impact-stone', gain: 0.74, cooldown: 120, rate: 0.95 },
        attack_melee: { sound: 'rifle', gain: 0.42, cooldown: 110, rate: 1.16, vary: 0.12 },
        attack_ranged: { sound: 'laser', gain: 0.5, cooldown: 90, rate: 1.05, vary: 0.1 },
        crate_break: { sound: 'crate-break', gain: 0.65, cooldown: 80, vary: 0.16 },
        enemy_hit: { sound: null, gain: 0.6, cooldown: 70, vary: 0.16 },
        enemy_kill: { sound: 'explosion-small', gain: 0.7, cooldown: 100, vary: 0.14 },
        boss_defeat: { sound: 'explosion-large', gain: 0.86, cooldown: 200 },
        boss_phase: { sound: 'thunder', gain: 0.8, cooldown: 300 },
        player_hurt: { sound: null, gain: 0.66, cooldown: 180, vary: 0.1 },
        pickup_coin: { sound: 'pickup', gain: 0.5, cooldown: 55, rate: 1.12, vary: 0.14 },
        pickup_gem: { sound: 'laser', gain: 0.6, cooldown: 90, rate: 0.92 },
        pickup_star: { sound: 'electric-spark', gain: 0.62, cooldown: 90, rate: 1.1 },
        pickup_heart: { sound: 'pickup', gain: 0.56, cooldown: 120, rate: 0.88 },
        pickup_potion: { sound: 'electric-spark', gain: 0.56, cooldown: 120, rate: 0.86 },
        pickup_key: { sound: 'electric-spark', gain: 0.72, cooldown: 140, rate: 0.94 },
        pickup_shrine: { sound: 'electric-spark', gain: 0.75, cooldown: 200, rate: 0.8 },
        bomb_blast: { sound: 'explosion-large', gain: 0.7, cooldown: 120, rate: 1.05 },
        boulder_land: { sound: 'impact-stone', gain: 0.62, cooldown: 160, rate: 0.9 },
        checkpoint: { sound: 'fire', gain: 0.55, cooldown: 200 },
        heal: { sound: 'pickup', gain: 0.56, cooldown: 150, rate: 0.84 },
        hazard: { sound: 'fire', gain: 0.4, cooldown: 260 },
        cannon: { sound: 'shotgun', gain: 0.45, cooldown: 150, vary: 0.1 },
        enemy_shot: { sound: 'pistol', gain: 0.4, cooldown: 90, vary: 0.14 },
        portal_win: { sound: 'victory', gain: 0.8, cooldown: 500 },
        player_defeat: { sound: 'defeat', gain: 0.75, cooldown: 500 }
    };

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

    function playFallbackTone(audioCtx, id, volume, outNode) {
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
            else if (id === 'laser' || id === 'electric-spark') { fStart = 700; fEnd = 1200; dur = 0.13; type = 'sine'; }
            else if (id === 'rifle' || id === 'shotgun') { fStart = 460; fEnd = 180; dur = 0.09; type = 'sawtooth'; }
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
            gain.connect(outNode || audioCtx.destination);
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
        let master = null;
        let ambienceSource = null;
        let ambienceSource2 = null;
        let ambienceGain = null;
        let ambienceGain2 = null;
        let ambienceSpec = { primary: '', secondary: '' };
        let ambienceDuck = 1;
        const buffers = new Map();
        const pendingLoads = new Map();
        const lastPlayTime = new Map();

        function ensureContext() {
            if (disposed || !enabled) return null;
            if (!ctx) {
                const AudioCtx = root.AudioContext || root.webkitAudioContext;
                if (!AudioCtx) return null;
                ctx = new AudioCtx();
                // Master chain keeps overlapping cues from clipping.
                master = ctx.createGain();
                master.gain.value = 0.92;
                const limiter = ctx.createDynamicsCompressor();
                limiter.threshold.value = -14;
                limiter.knee.value = 22;
                limiter.ratio.value = 7;
                limiter.attack.value = 0.004;
                limiter.release.value = 0.2;
                master.connect(limiter);
                limiter.connect(ctx.destination);
            }
            if (ctx.state === 'suspended') {
                ctx.resume().catch(function () {});
            }
            return ctx;
        }

        function outNode() {
            return master || (ctx ? ctx.destination : null);
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

        function stopSource(source, gain) {
            if (source) {
                try { source.stop(); } catch (err) { /* stopped */ }
                try { source.disconnect(); } catch (err) { /* disconnected */ }
            }
            if (gain) {
                try { gain.disconnect(); } catch (err) { /* disconnected */ }
            }
        }

        function stopAmbience() {
            stopSource(ambienceSource, ambienceGain);
            stopSource(ambienceSource2, ambienceGain2);
            ambienceSource = null;
            ambienceSource2 = null;
            ambienceGain = null;
            ambienceGain2 = null;
        }

        function applyAmbienceGain() {
            if (ambienceGain) ambienceGain.gain.value = ambienceLevel(ambienceSpec.primary, 1) * ambienceDuck;
            if (ambienceGain2) ambienceGain2.gain.value = ambienceLevel(ambienceSpec.secondary, 0.55) * ambienceDuck;
        }

        function ambienceLevel(id, scale) {
            if (!id) return 0;
            return (Number(assetsById[id].gain) || 0.6) * 0.42 * scale;
        }

        function startAmbienceLayer(id, scale, slot) {
            const audioCtx = ensureContext();
            if (!audioCtx) return;
            loadBuffer(id).then(function (buffer) {
                if (!buffer || disposed || !enabled || !active) return;
                if (ambienceSpec.primary !== id && ambienceSpec.secondary !== id) return;
                const source = audioCtx.createBufferSource();
                source.buffer = buffer;
                source.loop = true;
                const gain = audioCtx.createGain();
                gain.gain.value = ambienceLevel(id, scale) * ambienceDuck;
                source.connect(gain);
                gain.connect(outNode());
                source.start(0);
                if (slot === 2) {
                    stopSource(ambienceSource2, ambienceGain2);
                    ambienceSource2 = source;
                    ambienceGain2 = gain;
                } else {
                    stopSource(ambienceSource, ambienceGain);
                    ambienceSource = source;
                    ambienceGain = gain;
                }
            });
        }

        // Accepts a plain id or { primary, secondary } for layered biome ambience.
        async function setAmbience(spec) {
            const next = typeof spec === 'string' || !spec
                ? { primary: spec || '', secondary: '' }
                : { primary: spec.primary || '', secondary: spec.secondary || '' };
            const changed = next.primary !== ambienceSpec.primary || next.secondary !== ambienceSpec.secondary;
            ambienceSpec = next;
            if (!changed) return;
            stopAmbience();
            if (!enabled || !active || disposed || !ambienceSpec.primary) return;
            startAmbienceLayer(ambienceSpec.primary, 1, 1);
            if (ambienceSpec.secondary) startAmbienceLayer(ambienceSpec.secondary, 0.55, 2);
        }

        // Ducks the ambience bed (used while a world boss is alive).
        function setAmbienceDuck(level) {
            ambienceDuck = Math.max(0, Math.min(1, Number(level) || 0));
            applyAmbienceGain();
        }

        function playBuffer(audioCtx, buffer, id, gainScale, rate) {
            const source = audioCtx.createBufferSource();
            source.buffer = buffer;
            if (rate && rate > 0) source.playbackRate.value = rate;
            const gain = audioCtx.createGain();
            gain.gain.value = Math.min(1, (Number(assetsById[id].gain) || 0.75) * (gainScale || 0.75));
            source.connect(gain);
            gain.connect(outNode());
            source.start(0);
        }

        function playCue(id, gainScale, cooldownMs, allowWhilePaused, rate) {
            if (disposed || !enabled || (!active && !allowWhilePaused) || !assetsById[id]) return;
            const now = Date.now();
            const minGap = cooldownMs || 65;
            if ((lastPlayTime.get(id) || 0) + minGap > now) return;
            lastPlayTime.set(id, now);
            const audioCtx = ensureContext();
            if (!audioCtx) return;

            const cached = buffers.get(id);
            if (cached) {
                playBuffer(audioCtx, cached, id, gainScale, rate);
                return;
            }
            playFallbackTone(audioCtx, id, gainScale, outNode());
            loadBuffer(id).then(function (buffer) {
                if (!buffer || disposed || !enabled || (!active && !allowWhilePaused)) return;
                if (!lastPlayTime.get(id) || Date.now() - now < 250) {
                    playBuffer(audioCtx, buffer, id, gainScale, rate);
                }
            });
        }

        function unlock() {
            if (disposed || !enabled) return;
            const audioCtx = ensureContext();
            if (!audioCtx) return;
            if (audioCtx.state === 'suspended') {
                audioCtx.resume().then(function () {
                    if (active && ambienceSpec.primary && !ambienceSource) {
                        setAmbience(ambienceSpec);
                    }
                }).catch(function () {});
            }
            PRELOAD_SFX.forEach(function (id) { loadBuffer(id); });
            if (active && ambienceSpec.primary && !ambienceSource) {
                setAmbience(ambienceSpec);
            }
        }

        function cueRate(cue, vary) {
            const base = cue.rate || 1;
            if (!vary) return base;
            return base * (1 + (Math.random() * 2 - 1) * vary);
        }

        function handleEvents(events, worldAmbience) {
            if (!Array.isArray(events)) return;
            for (let i = 0; i < events.length; i++) {
                const ev = events[i];
                if (!ev) continue;
                const cue = CUE_MAP[ev.type];
                if (!cue) continue;
                let id = cue.sound;
                if (!id) {
                    id = (ev.detail && ev.detail.sound) || null;
                    if (!id && ev.type === 'step') id = 'step-grass';
                    if (!id && ev.type === 'player_hurt') id = 'impact-flesh';
                }
                if (!id || !assetsById[id]) continue;
                const rate = cueRate(cue, cue.vary);
                playCue(id, cue.gain, cue.cooldown, false, rate);
            }
            if (worldAmbience) {
                const spec = typeof worldAmbience === 'string' ? { primary: worldAmbience, secondary: '' } : worldAmbience;
                if ((spec.primary !== ambienceSpec.primary || spec.secondary !== ambienceSpec.secondary) && enabled && active) {
                    setAmbience(spec);
                }
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
                if (ambienceSpec.primary && !ambienceSource) {
                    setAmbience(ambienceSpec);
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
            setAmbienceDuck: setAmbienceDuck,
            playUiClick: function () { playCue('ui-click', 0.5, 50, true); },
            handleEvents: handleEvents,
            dispose: dispose
        };
    }

    root.RunenpfadAudio = {
        BASE_URL: BASE_URL,
        REQUIRED_SOUNDS: REQUIRED_SOUNDS,
        CUE_MAP: CUE_MAP,
        validateSoundManifest: validateSoundManifest,
        createAudioController: createAudioController
    };
})(typeof window !== 'undefined' ? window : globalThis);
