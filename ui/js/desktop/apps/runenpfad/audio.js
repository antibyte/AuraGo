/* Runenpfad Web Audio controller: sample pack + procedural synth layers, buses, per-biome reverb. */
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
        'ui-click', 'jump', 'land', 'step-grass', 'laser', 'pickup', 'crate-break',
        'impact-flesh', 'impact-metal', 'impact-stone', 'explosion-small', 'electric-spark'
    ];
    const FADE_SECONDS = 1.2;
    const COIN_LADDER = [0, 2, 4, 7, 9, 12, 14, 16];
    const COIN_RESET_SECONDS = 0.7;
    const BIOME_BY_AMBIENCE = { 'forest-day': 'woodland', 'forest-night': 'cavern', 'wind-strong': 'desert', 'wind-gentle': 'glacier', industrial: 'citadel' };
    // Reverb tail length (s) and wet level per biome.
    const REVERBS = {
        woodland: { seconds: 1.2, wet: 0.16 },
        cavern: { seconds: 2.8, wet: 0.35 },
        desert: { seconds: 0.9, wet: 0.1 },
        glacier: { seconds: 2.2, wet: 0.26 },
        citadel: { seconds: 1.6, wet: 0.2 }
    };

    // Event -> cue table. `sound: null` means the event carries its own sound id, `sound: false` means synth only.
    // `synth` names a procedural layer (gain `sg`, pitch/speed scale `sr`); `vary` is a deterministic detune amount;
    // `by` overrides fields by an event detail value (or '@biome').
    const CUE_MAP = {
        step: { sound: null, gain: 0.38, cooldown: 220, vary: 0.14 },
        jump: { sound: 'jump', gain: 0.58, cooldown: 90, vary: 0.1 },
        dash: { sound: 'jump', gain: 0.36, cooldown: 140, rate: 1.32, vary: 0.08, synth: 'whoosh', sg: 0.4, sr: 1.3 },
        spring: { sound: 'jump', gain: 0.5, cooldown: 90, rate: 1.45, vary: 0.08, synth: 'boing', sg: 0.35 },
        bounce: { sound: 'jump', gain: 0.4, cooldown: 90, rate: 1.6, vary: 0.08, synth: 'boing', sg: 0.4, sr: 1.2 },
        land: { sound: 'land', gain: 0.5, cooldown: 110, vary: 0.12 },
        slam: { sound: 'shotgun', gain: 0.78, cooldown: 120, rate: 0.92 },
        slam_land: { sound: 'impact-stone', gain: 0.74, cooldown: 120, rate: 0.95 },
        attack_melee: { sound: false, gain: 0.5, cooldown: 110, vary: 0.12, synth: 'whoosh', sg: 0.7 },
        attack_ranged: { sound: 'laser', gain: 0.45, cooldown: 90, rate: 1.05, vary: 0.1, synth: 'zap', sg: 0.45 },
        crate_break: { sound: 'crate-break', gain: 0.65, cooldown: 80, vary: 0.16 },
        enemy_hit: { sound: null, gain: 0.6, cooldown: 70, vary: 0.16 },
        enemy_kill: { sound: 'explosion-small', gain: 0.7, cooldown: 100, vary: 0.14 },
        enemy_shot: { sound: 'pistol', gain: 0.4, cooldown: 90, vary: 0.14 },
        enemy_attack: {
            sound: false, gain: 0.4, cooldown: 120, vary: 0.1,
            by: { key: 'kind', map: { shot: { synth: 'zap', sg: 0.22, sr: 0.8 }, lunge: { synth: 'whoosh', sg: 0.5, sr: 0.7 } } }
        },
        boss_intro: { sound: 'thunder', gain: 0.7, cooldown: 600, rate: 0.75, synth: 'warn', sg: 0.4, sr: 0.5 },
        boss_phase: { sound: 'thunder', gain: 0.8, cooldown: 300 },
        boss_defeat: { sound: 'explosion-large', gain: 0.86, cooldown: 200 },
        boss_slam: { sound: 'explosion-large', gain: 0.7, cooldown: 200, rate: 0.8 },
        boss_telegraph: { sound: false, gain: 0.4, cooldown: 350, synth: 'warn', sg: 0.3, sr: 1 },
        boss_attack: {
            sound: false, gain: 0.6, cooldown: 150,
            by: {
                key: 'pattern',
                map: {
                    charge: { sound: 'impact-stone', rate: 0.8, synth: 'whoosh', sg: 0.6, sr: 0.5 },
                    pound: { sound: 'impact-stone', rate: 0.7, gain: 0.8 },
                    leap: { sound: 'jump', rate: 0.6, synth: 'whoosh', sg: 0.5, sr: 0.7 },
                    volley: { synth: 'whoosh', sg: 0.6, sr: 1.3 },
                    ring: { sound: 'laser', rate: 0.7, synth: 'zap', sg: 0.4, sr: 0.7 },
                    laser: { sound: 'laser', rate: 0.55, gain: 0.7, synth: 'zap', sg: 0.35, sr: 0.5 },
                    dive: { synth: 'whoosh', sg: 0.7, sr: 0.6 },
                    summon: { sound: 'electric-spark', rate: 0.6, gain: 0.6 },
                    missiles: { sound: 'reload', gain: 0.7 },
                    groundfire: { sound: 'fire', gain: 0.7 }
                }
            }
        },
        bomb_blast: { sound: 'explosion-large', gain: 0.7, cooldown: 120, rate: 1.05 },
        boulder_land: { sound: 'impact-stone', gain: 0.62, cooldown: 160, rate: 0.9 },
        player_hurt: { sound: null, gain: 0.66, cooldown: 180, vary: 0.1 },
        player_defeat: { sound: 'defeat', gain: 0.75, cooldown: 500 },
        player_fall: {
            sound: false, gain: 0.6, cooldown: 400,
            by: { key: '@biome', map: { woodland: { sound: 'water-splash' }, glacier: { sound: 'water-splash' }, '*': { synth: 'whoosh', sg: 0.6, sr: 0.35 } } }
        },
        pickup_coin: { sound: 'pickup', gain: 0.42, cooldown: 40, rate: 1.0, ladder: true, synth: 'chime', sg: 0.3 },
        pickup_gem: { sound: 'laser', gain: 0.6, cooldown: 90, rate: 0.92 },
        pickup_star: { sound: 'electric-spark', gain: 0.62, cooldown: 90, rate: 1.1, synth: 'chime', sg: 0.35, sr: 2 },
        pickup_treasure: { sound: 'pickup', gain: 0.6, cooldown: 120, rate: 0.8, synth: 'chime', sg: 0.4, sr: 1.5 },
        pickup_heart: { sound: 'pickup', gain: 0.56, cooldown: 120, rate: 0.88 },
        pickup_potion: { sound: 'electric-spark', gain: 0.56, cooldown: 120, rate: 0.86 },
        pickup_key: { sound: 'electric-spark', gain: 0.72, cooldown: 140, rate: 0.94 },
        pickup_shrine: { sound: 'electric-spark', gain: 0.75, cooldown: 200, rate: 0.8 },
        checkpoint: { sound: 'fire', gain: 0.55, cooldown: 200 },
        heal: { sound: 'pickup', gain: 0.56, cooldown: 150, rate: 0.84 },
        combo: { sound: false, gain: 0.5, cooldown: 90, synth: 'combo', sg: 0.35 },
        secret_break: { sound: 'impact-stone', gain: 0.6, cooldown: 200, rate: 0.85, synth: 'chime', sg: 0.4, sr: 1.5 },
        lantern_lit: { sound: 'fire', gain: 0.42, cooldown: 200, rate: 1.2, synth: 'chime', sg: 0.3, sr: 1.8 },
        hazard: {
            sound: 'fire', gain: 0.4, cooldown: 260,
            by: { key: 'kind', map: { spikes: { sound: 'impact-metal', gain: 0.5 }, spike: { sound: 'impact-metal', gain: 0.5 }, boulder: { sound: 'impact-stone', gain: 0.55 } } }
        },
        spike_rise: { sound: 'impact-metal', gain: 0.3, cooldown: 200, rate: 1.7 },
        door_open: { sound: 'door-wood', gain: 0.65, cooldown: 300, by: { key: 'material', map: { wood: { sound: 'door-wood' }, metal: { sound: 'door-metal' } } } },
        cannon: { sound: 'shotgun', gain: 0.45, cooldown: 150, vary: 0.1 },
        portal_win: { sound: 'victory', gain: 0.8, cooldown: 500 }
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

    // Deterministic [0,1) hash of (cueId, counter) for pitch variation.
    function hashUnit(cueId, counter) {
        const s = String(cueId) + ':' + counter;
        let h = 2166136261 >>> 0;
        for (let i = 0; i < s.length; i++) {
            h ^= s.charCodeAt(i);
            h = Math.imul(h, 16777619) >>> 0;
        }
        h ^= h >>> 15;
        h = Math.imul(h, 2246822519) >>> 0;
        h ^= h >>> 13;
        return (h >>> 0) / 4294967296;
    }

    function panFor(x, camera) {
        if (typeof x !== 'number' || !camera || !(camera.w > 0)) return 0;
        const half = camera.w / 2;
        return Math.max(-1, Math.min(1, (x - (camera.x + half)) / half)) * 0.7;
    }

    function buildImpulse(ac, biome) {
        const spec = REVERBS[biome] || REVERBS.woodland;
        const sr = ac.sampleRate;
        const len = Math.floor(spec.seconds * sr);
        const buf = ac.createBuffer(2, len, sr);
        for (let ch = 0; ch < 2; ch++) {
            const data = buf.getChannelData(ch);
            let seed = Math.floor(hashUnit(biome, ch) * 4294967295) >>> 0;
            let lp = 0;
            for (let i = 0; i < len; i++) {
                seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
                const n = seed / 2147483648 - 1;
                const k = i / len;
                lp += (n - lp) * (0.65 - 0.52 * k);
                data[i] = lp * Math.exp(-5.2 * k) * (1 - k);
            }
        }
        return buf;
    }

    // Procedural layers. Each one builds its own nodes into `dest` and disconnects when finished.
    const SYNTHS = {
        whoosh: function (ac, noise, dest, t, o) {
            const dur = 0.16 / o.sr;
            const src = ac.createBufferSource();
            src.buffer = noise;
            const bp = ac.createBiquadFilter();
            bp.type = 'bandpass';
            bp.Q.value = 1.3;
            bp.frequency.setValueAtTime(600 * o.sr, t);
            bp.frequency.exponentialRampToValueAtTime(2400 * o.sr, t + dur);
            const g = ac.createGain();
            g.gain.setValueAtTime(0.0001, t);
            g.gain.linearRampToValueAtTime(o.gain, t + dur * 0.35);
            g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
            src.connect(bp);
            bp.connect(g);
            g.connect(dest);
            src.start(t, o.v * 0.5);
            src.stop(t + dur + 0.02);
            return [src, [src, bp, g]];
        },
        zap: function (ac, noise, dest, t, o) {
            const dur = 0.12;
            const osc = ac.createOscillator();
            osc.type = 'square';
            osc.frequency.setValueAtTime(2200 * o.sr, t);
            osc.frequency.exponentialRampToValueAtTime(520 * o.sr, t + dur);
            const bp = ac.createBiquadFilter();
            bp.type = 'bandpass';
            bp.frequency.value = 1500 * o.sr;
            bp.Q.value = 2;
            const g = ac.createGain();
            g.gain.setValueAtTime(o.gain * 0.4, t);
            g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
            osc.connect(bp);
            bp.connect(g);
            g.connect(dest);
            osc.start(t);
            osc.stop(t + dur + 0.02);
            return [osc, [osc, bp, g]];
        },
        boing: function (ac, noise, dest, t, o) {
            const dur = 0.2;
            const osc = ac.createOscillator();
            osc.type = 'sine';
            osc.frequency.setValueAtTime(260 * o.sr, t);
            osc.frequency.exponentialRampToValueAtTime(760 * o.sr, t + dur * 0.7);
            const g = ac.createGain();
            g.gain.setValueAtTime(o.gain * 0.6, t);
            g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
            osc.connect(g);
            g.connect(dest);
            osc.start(t);
            osc.stop(t + dur + 0.02);
            return [osc, [osc, g]];
        },
        warn: function (ac, noise, dest, t, o) {
            const dur = 0.5;
            const osc = ac.createOscillator();
            osc.type = 'triangle';
            osc.frequency.setValueAtTime(220 * o.sr * 2, t);
            osc.frequency.linearRampToValueAtTime(440 * o.sr * 2, t + dur);
            const g = ac.createGain();
            g.gain.setValueAtTime(0.0001, t);
            g.gain.linearRampToValueAtTime(o.gain * 0.5, t + dur * 0.6);
            g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
            osc.connect(g);
            g.connect(dest);
            osc.start(t);
            osc.stop(t + dur + 0.02);
            return [osc, [osc, g]];
        },
        chime: function (ac, noise, dest, t, o) {
            const dur = 0.55;
            const f = (o.freq || 1046) * o.sr;
            const osc = ac.createOscillator();
            osc.type = 'sine';
            osc.frequency.value = f;
            const osc2 = ac.createOscillator();
            osc2.type = 'sine';
            osc2.frequency.value = f * 2.01;
            const g2 = ac.createGain();
            g2.gain.value = 0.25;
            const g = ac.createGain();
            g.gain.setValueAtTime(0.0001, t);
            g.gain.linearRampToValueAtTime(o.gain * 0.5, t + 0.006);
            g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
            osc.connect(g);
            osc2.connect(g2);
            g2.connect(g);
            g.connect(dest);
            osc.start(t);
            osc2.start(t);
            osc.stop(t + dur + 0.02);
            osc2.stop(t + dur + 0.02);
            return [osc, [osc, osc2, g2, g]];
        },
        combo: function (ac, noise, dest, t, o) {
            const step = COIN_LADDER[Math.max(0, Math.min(COIN_LADDER.length - 1, (o.count || 2) - 2))];
            return SYNTHS.chime(ac, noise, dest, t, { gain: o.gain, sr: Math.pow(2, step / 12), freq: 523, v: o.v });
        }
    };

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
            osc.onended = function () { try { osc.disconnect(); gain.disconnect(); } catch (err) { /* gone */ } };
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
        let sfxBus = null;
        let ambBus = null;
        let musicBus = null;
        let reverbIn = null;
        let reverbOut = null;
        let convolver = null;
        let noiseBuf = null;
        let biome = 'woodland';
        let ambienceSpec = { primary: '', secondary: '' };
        let ambienceDuck = 1;
        let coinStep = -1;
        let lastCoinAt = -9;
        const layers = [];
        const buffers = new Map();
        const pendingLoads = new Map();
        const impulses = new Map();
        const lastPlay = new Map();
        const counters = new Map();

        function ensureContext() {
            if (disposed || !enabled) return null;
            if (!ctx) {
                const AudioCtx = root.AudioContext || root.webkitAudioContext;
                if (!AudioCtx) return null;
                ctx = new AudioCtx();
                // Buses -> master -> glue compressor -> brickwall-ish limiter.
                master = ctx.createGain();
                master.gain.value = 0.92;
                const glue = ctx.createDynamicsCompressor();
                glue.threshold.value = -14;
                glue.knee.value = 22;
                glue.ratio.value = 7;
                glue.attack.value = 0.004;
                glue.release.value = 0.2;
                const limiter = ctx.createDynamicsCompressor();
                limiter.threshold.value = -3;
                limiter.knee.value = 0;
                limiter.ratio.value = 20;
                limiter.attack.value = 0.001;
                limiter.release.value = 0.08;
                master.connect(glue);
                glue.connect(limiter);
                limiter.connect(ctx.destination);
                sfxBus = ctx.createGain();
                ambBus = ctx.createGain();
                musicBus = ctx.createGain();
                musicBus.gain.value = 0.45;
                [sfxBus, ambBus, musicBus].forEach(function (b) { b.connect(master); });
                reverbIn = ctx.createGain();
                reverbOut = ctx.createGain();
                convolver = ctx.createConvolver();
                reverbIn.connect(convolver);
                convolver.connect(reverbOut);
                reverbOut.connect(master);
                const sfxSend = ctx.createGain();
                sfxSend.gain.value = 0.55;
                sfxBus.connect(sfxSend);
                sfxSend.connect(reverbIn);
                noiseBuf = ctx.createBuffer(1, ctx.sampleRate, ctx.sampleRate);
                const data = noiseBuf.getChannelData(0);
                let seed = 0x9E3779B9;
                for (let i = 0; i < data.length; i++) {
                    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
                    data[i] = seed / 2147483648 - 1;
                }
                applyBiome(biome);
            }
            if (ctx.state === 'suspended') {
                ctx.resume().catch(function () {});
            }
            return ctx;
        }

        function applyBiome(next) {
            biome = REVERBS[next] ? next : 'woodland';
            if (!ctx || !convolver) return;
            if (!impulses.has(biome)) impulses.set(biome, buildImpulse(ctx, biome));
            convolver.buffer = impulses.get(biome);
            reverbOut.gain.setTargetAtTime(REVERBS[biome].wet, ctx.currentTime, 0.2);
        }

        function outNode() {
            return sfxBus || (ctx ? ctx.destination : null);
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

        function killLayer(layer) {
            try { layer.source.stop(); } catch (err) { /* stopped */ }
            try { layer.source.disconnect(); } catch (err) { /* disconnected */ }
            try { layer.gain.disconnect(); } catch (err) { /* disconnected */ }
        }

        function fadeOutLayer(layer) {
            const idx = layers.indexOf(layer);
            if (idx >= 0) layers.splice(idx, 1);
            if (!ctx) { killLayer(layer); return; }
            try {
                const now = ctx.currentTime;
                layer.gain.gain.cancelScheduledValues(now);
                layer.gain.gain.setValueAtTime(layer.gain.gain.value, now);
                layer.gain.gain.linearRampToValueAtTime(0, now + FADE_SECONDS);
                layer.source.onended = function () { killLayer(layer); };
                layer.source.stop(now + FADE_SECONDS + 0.05);
            } catch (err) { killLayer(layer); }
        }

        function stopAmbience() {
            layers.splice(0).forEach(killLayer);
        }

        function ambienceLevel(id, scale) {
            if (!id || !assetsById[id]) return 0;
            return (Number(assetsById[id].gain) || 0.6) * 0.42 * scale;
        }

        function applyAmbienceGain() {
            if (!ctx) return;
            layers.forEach(function (layer) {
                layer.gain.gain.cancelScheduledValues(ctx.currentTime);
                layer.gain.gain.setTargetAtTime(ambienceLevel(layer.id, layer.scale) * ambienceDuck, ctx.currentTime, 0.15);
            });
        }

        function startAmbienceLayer(id, scale, slot) {
            const audioCtx = ensureContext();
            if (!audioCtx) return;
            loadBuffer(id).then(function (buffer) {
                if (!buffer || disposed || !enabled || !active) return;
                if (ambienceSpec.primary !== id && ambienceSpec.secondary !== id) return;
                layers.filter(function (l) { return l.slot === slot; }).forEach(fadeOutLayer);
                const source = audioCtx.createBufferSource();
                source.buffer = buffer;
                source.loop = true;
                const gain = audioCtx.createGain();
                const now = audioCtx.currentTime;
                gain.gain.setValueAtTime(0, now);
                gain.gain.linearRampToValueAtTime(ambienceLevel(id, scale) * ambienceDuck, now + FADE_SECONDS);
                source.connect(gain);
                gain.connect(ambBus || outNode());
                source.start(0);
                layers.push({ id: id, slot: slot, scale: scale, source: source, gain: gain });
            });
        }

        // Accepts a plain id or { primary, secondary } for layered biome ambience.
        async function setAmbience(spec) {
            const next = typeof spec === 'string' || !spec
                ? { primary: spec || '', secondary: '' }
                : { primary: spec.primary || '', secondary: spec.secondary || '' };
            const prev = ambienceSpec;
            ambienceSpec = next;
            if (BIOME_BY_AMBIENCE[next.primary] && BIOME_BY_AMBIENCE[next.primary] !== biome) applyBiome(BIOME_BY_AMBIENCE[next.primary]);
            if (next.primary === prev.primary && next.secondary === prev.secondary) return;
            // Layers whose sound is no longer wanted crossfade out; new ones fade in.
            layers.slice().forEach(function (layer) {
                if (layer.id !== next.primary && layer.id !== next.secondary) fadeOutLayer(layer);
            });
            if (!enabled || !active || disposed) return;
            if (!next.primary) return;
            const has = function (id, slot) { return layers.some(function (l) { return l.id === id && l.slot === slot; }); };
            if (!has(next.primary, 1)) startAmbienceLayer(next.primary, 1, 1);
            if (next.secondary && !has(next.secondary, 2)) startAmbienceLayer(next.secondary, 0.55, 2);
        }

        // Ducks the ambience bed (used while a world boss is alive).
        function setAmbienceDuck(level) {
            ambienceDuck = Math.max(0, Math.min(1, Number(level) || 0));
            applyAmbienceGain();
        }

        function route(pan) {
            if (pan && ctx && typeof ctx.createStereoPanner === 'function') {
                const p = ctx.createStereoPanner();
                p.pan.value = pan;
                p.connect(outNode());
                return p;
            }
            return outNode();
        }

        function releaseRoute(node) {
            if (node && node !== sfxBus && (!ctx || node !== ctx.destination) && typeof node.pan !== 'undefined') {
                try { node.disconnect(); } catch (err) { /* gone */ }
            }
        }

        function playBuffer(audioCtx, buffer, id, gainScale, rate, pan) {
            const source = audioCtx.createBufferSource();
            source.buffer = buffer;
            if (rate && rate > 0) source.playbackRate.value = rate;
            const gain = audioCtx.createGain();
            gain.gain.value = Math.min(1, (Number(assetsById[id].gain) || 0.75) * (gainScale || 0.75));
            const dest = route(pan);
            source.connect(gain);
            gain.connect(dest);
            source.onended = function () {
                try { source.disconnect(); gain.disconnect(); } catch (err) { /* gone */ }
                releaseRoute(dest);
            };
            source.start(0);
        }

        // Cooldown gate on the audio clock (seconds); returns true when the cue may play.
        function gate(key, cooldownMs) {
            const now = ctx.currentTime;
            if (lastPlay.has(key) && lastPlay.get(key) + (cooldownMs || 65) / 1000 > now) return false;
            lastPlay.set(key, now);
            return true;
        }

        function playSample(id, gainScale, rate, pan) {
            const audioCtx = ctx;
            const cached = buffers.get(id);
            if (cached) {
                playBuffer(audioCtx, cached, id, gainScale, rate, pan);
                return;
            }
            const t0 = audioCtx.currentTime;
            playFallbackTone(audioCtx, id, gainScale, outNode());
            loadBuffer(id).then(function (buffer) {
                if (!buffer || disposed || !enabled || !ctx) return;
                if (ctx.currentTime - t0 < 0.25) playBuffer(ctx, buffer, id, gainScale, rate, pan);
            });
        }

        function playSynth(name, o, pan) {
            const fn = SYNTHS[name];
            if (!fn || !ctx || ctx.state !== 'running' || !noiseBuf) return;
            const dest = route(pan);
            try {
                const res = fn(ctx, noiseBuf, dest, ctx.currentTime + 0.005, o);
                res[0].onended = function () {
                    res[1].forEach(function (n) { try { n.disconnect(); } catch (err) { /* gone */ } });
                    releaseRoute(dest);
                };
            } catch (err) { releaseRoute(dest); }
        }

        function variation(key, amount) {
            if (!amount) return 1;
            const n = counters.get(key) || 0;
            counters.set(key, n + 1);
            return 1 + (hashUnit(key, n) * 2 - 1) * amount;
        }

        // Plays an arbitrary sample cue (used by the UI click); respects mute and the cooldown.
        function playCue(id, gainScale, cooldownMs, allowWhilePaused, rate) {
            if (disposed || !enabled || (!active && !allowWhilePaused) || !assetsById[id]) return;
            if (!ensureContext() || !gate('cue:' + id, cooldownMs)) return;
            playSample(id, gainScale, rate, 0);
        }

        function unlock() {
            if (disposed || !enabled) return;
            const audioCtx = ensureContext();
            if (!audioCtx) return;
            if (audioCtx.state === 'suspended') {
                audioCtx.resume().then(function () {
                    if (active && ambienceSpec.primary && !layers.length) {
                        restartAmbience();
                    }
                }).catch(function () {});
            }
            PRELOAD_SFX.forEach(function (id) { loadBuffer(id); });
            if (active && ambienceSpec.primary && !layers.length) {
                restartAmbience();
            }
        }

        function restartAmbience() {
            const spec = ambienceSpec;
            ambienceSpec = { primary: '', secondary: '' };
            setAmbience(spec);
        }

        function resolveCue(ev, cue, d) {
            if (!cue.by) return cue;
            const k = cue.by.key === '@biome' ? biome : (d ? d[cue.by.key] : undefined);
            const over = cue.by.map[k] || cue.by.map['*'];
            return over ? Object.assign({}, cue, over) : cue;
        }

        function handleEvents(events, worldAmbience, camera) {
            if (Array.isArray(events) && events.length && !disposed && enabled && active) {
                const audioCtx = ensureContext();
                for (let i = 0; audioCtx && i < events.length; i++) {
                    const ev = events[i];
                    const base = ev && CUE_MAP[ev.type];
                    if (!base) continue;
                    const d = ev.detail || null;
                    if (ev.type === 'enemy_kill' && d && d.boss) continue;
                    const cue = resolveCue(ev, base, d);
                    const key = ev.type + ':' + ((d && (d.pattern || d.kind || d.material)) || '');
                    if (!gate(key, cue.cooldown)) continue;
                    let id = cue.sound;
                    if (id === null) {
                        id = (d && d.sound) || null;
                        if (!id && ev.type === 'step') id = 'step-grass';
                        if (!id && ev.type === 'player_hurt') id = 'impact-flesh';
                    }
                    const pan = panFor(d && d.x, camera);
                    let rate = (cue.rate || 1) * variation(key, cue.vary);
                    let sr = cue.sr || 1;
                    if (cue.ladder) {
                        if (audioCtx.currentTime - lastCoinAt > COIN_RESET_SECONDS) coinStep = 0;
                        else coinStep = Math.min(COIN_LADDER.length - 1, coinStep + 1);
                        lastCoinAt = audioCtx.currentTime;
                        const ratio = Math.pow(2, COIN_LADDER[coinStep] / 12);
                        rate *= ratio;
                        sr = ratio;
                    }
                    if (id && assetsById[id]) playSample(id, cue.gain, rate, pan);
                    if (cue.synth) playSynth(cue.synth, { gain: cue.sg || 0.4, sr: sr, v: hashUnit(key, counters.get(key) || 0), count: d && d.count, freq: cue.ladder ? 1046 : 0 }, pan);
                }
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
                if (ambienceSpec.primary && !layers.length) {
                    restartAmbience();
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
            impulses.clear();
            const closing = ctx;
            ctx = null;
            sfxBus = ambBus = musicBus = reverbIn = reverbOut = convolver = master = noiseBuf = null;
            if (closing) {
                closing.close().catch(function () {});
            }
        }

        // Lazy port for the music controller: the context only exists after the first unlock.
        const port = {
            get ctx() { return ctx; },
            get bus() { return musicBus; },
            get reverb() { return reverbIn; }
        };

        return {
            isEnabled: function () { return enabled; },
            setEnabled: setEnabled,
            setActive: setActive,
            unlock: unlock,
            setAmbience: setAmbience,
            setAmbienceDuck: setAmbienceDuck,
            playUiClick: function () { playCue('ui-click', 0.5, 50, true); },
            handleEvents: handleEvents,
            musicPort: function () { return port; },
            dispose: dispose
        };
    }

    root.RunenpfadAudio = {
        BASE_URL: BASE_URL,
        REQUIRED_SOUNDS: REQUIRED_SOUNDS,
        CUE_MAP: CUE_MAP,
        COIN_LADDER: COIN_LADDER,
        REVERBS: REVERBS,
        hashUnit: hashUnit,
        panFor: panFor,
        validateSoundManifest: validateSoundManifest,
        createAudioController: createAudioController
    };
})(typeof window !== 'undefined' ? window : globalThis);
