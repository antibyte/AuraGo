/* Aura Drift - audio: buses, limiter, event mapping (section 2.6), tyres and the public API.
 *
 * Buses: engine, sfx, ambience and music feed the master through a glue compressor and a brick-wall
 * limiter, so a pile-up of explosions cannot clip. Samples come from aura-drift-audio-bank.js, the
 * engine from aura-drift-engine.js, the score, stingers, fanfares and ambience from
 * aura-drift-music.js. Every one of them is optional: a missing module only makes the game quieter. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    /* Exactly the groups of the pack manifest (horn.<carId> expands per car); the contract test scans this. */
    AD.AUDIO_GROUPS = [
        'impact.light', 'impact.heavy', 'impact.metal', 'scrape.metal', 'tyre.squeal', 'explosion.small', 'explosion.big', 'crate.break',
        'prop.hay', 'prop.cone', 'prop.barrel', 'prop.tyres', 'crowd.cheer', 'crowd.aww',
        'animal.cow', 'animal.sheep', 'animal.penguin', 'animal.llama', 'animal.crab',
        'amb.wind', 'amb.sea', 'amb.forest', 'amb.machinery', 'amb.city', 'amb.snow', 'amb.desert',
        'weapon.oil', 'weapon.mine_drop', 'weapon.emp_charge', 'weapon.emp_pulse', 'weapon.rocket_fire', 'weapon.rocket_lock', 'weapon.mg', 'weapon.shield_on', 'weapon.shield_hit',
        'weapon.duck_launch', 'weapon.duck_quack', 'weapon.rail_charge', 'weapon.rail_fire', 'weapon.flame_loop', 'weapon.frost_throw', 'weapon.frost_burst', 'weapon.horn_blast',
        'weapon.hole_loop', 'weapon.disco_on', 'weapon.glove', 'weapon.harpoon_fire', 'weapon.harpoon_clank', 'weapon.cow_launch', 'weapon.cow_land', 'weapon.cloak_on', 'weapon.overdrive_on',
        'sting.perfect', 'sting.pancake', 'sting.strike', 'sting.slingshot', 'sting.moo', 'sting.trick', 'turbo.1', 'turbo.2', 'turbo.3', 'turbo.tick', 'pad.boost', 'ramp.launch',
        'land.soft', 'land.hard', 'roulette.tick', 'roulette.done', 'bomb.tick', 'bomb.blast', 'jingle.softeis', 'horn.<carId>', 'ufo.beam',
        'ui.count', 'ui.go', 'ui.lap', 'ui.bank', 'whoosh', 'stinger.final_lap', 'stinger.disco', 'fanfare.win', 'fanfare.podium', 'fanfare.lose'
    ];

    /* Groups that make the first race sound right; fetched right after the engine of the chosen car. */
    const CORE_GROUPS = [
        'ui.count', 'ui.go', 'ui.lap', 'ui.bank', 'impact.light', 'impact.heavy', 'impact.metal', 'explosion.small', 'explosion.big', 'crate.break',
        'turbo.1', 'turbo.2', 'turbo.3', 'turbo.tick', 'pad.boost', 'ramp.launch', 'land.soft', 'land.hard', 'whoosh', 'tyre.squeal', 'scrape.metal',
        'sting.perfect', 'sting.trick', 'sting.slingshot', 'prop.hay', 'prop.cone', 'prop.barrel', 'prop.tyres'
    ];
    const BIOME_AMBIENCE = {
        alpine: 'amb.wind', desert: 'amb.desert', harbour: 'amb.sea', salt: 'amb.desert', forest: 'amb.forest', industry: 'amb.machinery', glacier: 'amb.snow', metro: 'amb.city'
    };
    const WEAPON_GROUP = {
        oil: 'weapon.oil', gnome_mine: 'weapon.mine_drop', emp: 'weapon.emp_charge', rocket: 'weapon.rocket_fire', mg: 'weapon.mg', shield: 'weapon.shield_on',
        ducks: 'weapon.duck_launch', railgun: 'weapon.rail_charge', frost: 'weapon.frost_throw', air_horn: 'weapon.horn_blast', disco: 'weapon.disco_on',
        glove: 'weapon.glove', harpoon: 'weapon.harpoon_fire', cow: 'weapon.cow_launch', cloak: 'weapon.cloak_on', overdrive: 'weapon.overdrive_on'
    };
    const PROP_GROUP = { hay: 'prop.hay', cone: 'prop.cone', barrel: 'prop.barrel', tyre: 'prop.tyres' };
    const REVERB = { harbour: 0.34, industry: 0.30, metro: 0.26, glacier: 0.22, alpine: 0.16, forest: 0.12, salt: 0.06, desert: 0.06 };
    const RANGE = 120;
    /* Engine bus level per view: silent in the garage, a low bed under the results, full in a race. */
    const ENGINE_BED = { menu: 0, results: 0.3, race: 1 };
    const MASTER_TRIM = 1.0;            /* volume 1 = full scale into the glue compressor and the -3 dB limiter */
    const MUSIC_LEVEL = 0.32;           /* the score under the race */
    const AMBIENCE_LEVEL = 0.8;         /* the world around the track */
    /* The voice commentary is a dialogue bus of its own: it goes straight to the
     * limiter and skips the glue compressor, because that compressor has a makeup
     * gain which would lift a quiet line and squash a loud one again. So a line is
     * measured here and mixed to a fixed speech level. While a driver speaks, the
     * race steps back under the voice. */
    const VOICE_RMS = 0.12;             /* the level a spoken line is mixed to */
    const VOICE_PEAK = 0.95;            /* the ceiling of a line; the limiter takes the rest */
    const VOICE_DUCK = { engine: 0.18, music: 0.25, ambience: 0.4 };

    AD.createAudio = function (settings) {
        const Ctor = window.AudioContext || window.webkitAudioContext;
        let ctx = null, master = null, comp = null, limiter = null;
        let engineBus = null, sfxBus = null, ambBus = null, musicBus = null, voiceBus = null, reverb = null, reverbSend = null;
        let bank = null, engine = null, music = null;
        let noise = null;
        let volume = settings.volume, muted = settings.muted;
        let running = false, carId = null, biomeId = null, view = 'race';
        let lastTick = 0, heading = 0, myX = 0, myY = 0, lastHornAt = -9, lastKerbAt = -9, lastPassAt = -9;
        let rollGain = null, windGain = null, surfaceGain = null, surfaceFilter = null, sprayGain = null, sprayFilter = null;
        let liveNodes = [];
        let voiceSpeaking = false;       /* a driver is speaking and the race steps back */
        let voicesPlaying = 0;
        const loops = { squeal: null, scrape: null };
        const worldLoops = new Map();
        const lastPlayed = new Map();

        function makeImpulse(seconds, decay) {
            const length = Math.max(1, Math.floor(ctx.sampleRate * seconds));
            const buffer = ctx.createBuffer(2, length, ctx.sampleRate);
            for (let ch = 0; ch < 2; ch++) {
                const data = buffer.getChannelData(ch);
                for (let i = 0; i < length; i++) data[i] = (Math.random() * 2 - 1) * Math.pow(1 - i / length, decay);
            }
            return buffer;
        }
        function loopNoise(filterType, freq, gainValue, out) {
            const source = ctx.createBufferSource();
            source.buffer = noise; source.loop = true;
            const filter = ctx.createBiquadFilter();
            filter.type = filterType; filter.frequency.value = freq;
            const gain = ctx.createGain(); gain.gain.value = gainValue;
            source.connect(filter).connect(gain).connect(out);
            source.start();
            liveNodes.push(source);
            return { filter: filter, gain: gain };
        }

        function unlock() {
            if (!Ctor) return false;
            if (ctx) { if (ctx.state === 'suspended') ctx.resume().catch(() => {}); return true; }
            try { ctx = new Ctor(); } catch (_) { return false; }
            /* master -> glue compressor -> brick-wall limiter -> speakers */
            comp = ctx.createDynamicsCompressor();
            comp.threshold.value = -16; comp.knee.value = 10; comp.ratio.value = 3; comp.attack.value = 0.005; comp.release.value = 0.18;
            limiter = ctx.createDynamicsCompressor();
            limiter.threshold.value = -3; limiter.knee.value = 0; limiter.ratio.value = 20; limiter.attack.value = 0.001; limiter.release.value = 0.08;
            master = ctx.createGain();
            master.gain.value = muted ? 0 : volume * MASTER_TRIM;
            master.connect(comp).connect(limiter).connect(ctx.destination);
            engineBus = ctx.createGain(); engineBus.gain.value = ENGINE_BED[view] === undefined ? 1 : ENGINE_BED[view]; engineBus.connect(master);
            sfxBus = ctx.createGain(); sfxBus.gain.value = 1; sfxBus.connect(master);
            ambBus = ctx.createGain(); ambBus.gain.value = AMBIENCE_LEVEL; ambBus.connect(master);
            musicBus = ctx.createGain(); musicBus.gain.value = MUSIC_LEVEL; musicBus.connect(master);
            /* The voice commentary has its own bus: it speaks over the race and must
             * not push the limiter into clipping. It goes straight to the limiter
             * and skips the glue compressor, so a line keeps the level it was mixed
             * to. The bus carries the volume of the player. */
            voiceBus = ctx.createGain(); voiceBus.gain.value = muted ? 0 : volume * MASTER_TRIM; voiceBus.connect(limiter);
            reverb = ctx.createConvolver();
            reverb.buffer = makeImpulse(1.9, 2.6);
            reverbSend = ctx.createGain(); reverbSend.gain.value = 0;
            reverbSend.connect(reverb); reverb.connect(master);

            if (typeof AD.createSampleBank === 'function') {
                bank = AD.createSampleBank(ctx, { sfx: sfxBus, amb: ambBus, engine: engineBus, send: reverbSend });
                noise = bank.noise;
            } else {
                noise = ctx.createBuffer(1, ctx.sampleRate, ctx.sampleRate);
                bank = { noise: noise, play() { return null; }, tone() {}, noiseHit() {}, has() { return false; }, ready() { return Promise.resolve(null); }, load() { return Promise.resolve(0); },
                    loadEngine() { return Promise.resolve(null); }, engineLayers() { return null; }, stats() { return { samples: 0, failed: 0, fallback: 0 }; }, dispose() {} };
            }
            if (typeof AD.createEngineAudio === 'function') engine = AD.createEngineAudio(ctx, engineBus, bank, { send: reverbSend });

            /* Tyre texture that stays live: rolling, wind, surface grit and wet spray. */
            const tyre = loopNoise('bandpass', 1900, 0, sfxBus); rollGain = tyre.gain;
            const wind = loopNoise('highpass', 900, 0, sfxBus); windGain = wind.gain;
            const surface = loopNoise('highpass', 1400, 0, sfxBus); surfaceGain = surface.gain; surfaceFilter = surface.filter;
            const spray = loopNoise('highpass', 2600, 0, sfxBus); sprayGain = spray.gain; sprayFilter = spray.filter;

            music = typeof AD.createMusicEngine === 'function'
                ? AD.createMusicEngine(ctx, noise, musicBus, bank)
                : { setBiome() {}, setView() {}, setAmbience() {}, update() {}, event() {}, duck() {}, stop() {} };
            running = true;
            if (carId && engine) engine.setCar(carId);
            if (biomeId) applyBiome(biomeId);
            music.setView(view);
            preload();
            return true;
        }

        /* Lazy, prioritised fetching: engine of the chosen car (queued by engine.setCar), the core effects,
           the biome ambience, then everything else. Reduced data mode (saveData) decodes on first use. */
        function preload() {
            bank.ready().then(m => {
                if (!m || !ctx) return;
                bank.load(CORE_GROUPS.concat(['horn.' + (carId || 'vektor')]), 1);
                if (biomeId && BIOME_AMBIENCE[biomeId]) bank.load([BIOME_AMBIENCE[biomeId]], 3);
                if (!bank.lazy) bank.load(bank.groups().filter(g => CORE_GROUPS.indexOf(g) < 0 && g.indexOf('amb.') !== 0 && g.indexOf('horn.') !== 0), 6);
            });
        }
        function applyBiome(id) {
            music.setBiome(id);
            if (reverbSend && ctx) reverbSend.gain.setTargetAtTime(REVERB[id] !== undefined ? REVERB[id] : 0.15, ctx.currentTime, 0.4);
            if (bank && BIOME_AMBIENCE[id]) bank.load([BIOME_AMBIENCE[id]], 3);
        }

        /* The level of a line: measured, not guessed. The model output is quiet,
         * so a line is lifted to a fixed speech level; the peak stays under the
         * ceiling and the limiter behind the bus takes what is left. */
        function voiceGain(samples) {
            let sum = 0, peak = 0;
            for (let i = 0; i < samples.length; i++) {
                const v = samples[i];
                sum += v * v;
                const a = v < 0 ? -v : v;
                if (a > peak) peak = a;
            }
            const rms = Math.sqrt(sum / samples.length);
            if (!isFinite(rms) || rms < 0.001 || peak < 0.001) return 1;   /* silence stays silence */
            const gain = Math.min(VOICE_RMS / rms, VOICE_PEAK / peak);
            if (gain > 6) return 6;            /* a whisper is lifted, not amplified into noise */
            if (gain < 0.5) return 0.5;
            return gain;
        }

        /* While a driver speaks the race steps back: the engine, the score and the
         * ambience go down, the voice is the only thing on top. */
        function applyVoiceMix(tc) {
            if (!ctx) return;
            const now = ctx.currentTime;
            const bed = ENGINE_BED[view] === undefined ? 1 : ENGINE_BED[view];
            if (engineBus) engineBus.gain.setTargetAtTime(bed * (voiceSpeaking ? VOICE_DUCK.engine : 1), now, tc);
            if (musicBus) musicBus.gain.setTargetAtTime(MUSIC_LEVEL * (voiceSpeaking ? VOICE_DUCK.music : 1), now, tc);
            if (ambBus) ambBus.gain.setTargetAtTime(AMBIENCE_LEVEL * (voiceSpeaking ? VOICE_DUCK.ambience : 1), now, tc);
        }

        /* ---- spatial placement ---- */
        function spatial(x, y) {
            if (x === undefined || y === undefined || !isFinite(x) || !isFinite(y)) return { gain: 1, pan: 0, lp: 0 };
            const dx = x - myX, dy = y - myY;
            const dist = Math.hypot(dx, dy);
            if (dist > RANGE) return null;
            const lateral = (-dx * Math.sin(heading) + dy * Math.cos(heading)) / Math.max(1, dist);
            return { gain: 1 / (1 + dist * dist / 900), pan: clamp(lateral, -1, 1) * 0.85 * Math.min(1, dist / 10), lp: dist > 30 ? clamp(9000 - dist * 60, 1500, 9000) : 0 };
        }
        const mine = e => e.car === 0 || e.by === 0 || e.a === 0 || e.b === 0 || e.target === 0 || e.other === 0 || e.owner === 0;
        function P(group, sp, extra) {
            if (!sp || !ctx || muted || !bank) return null;
            const x = extra || {};
            return bank.play(group, {
                gain: sp.gain * (x.gain === undefined ? 1 : x.gain), pan: sp.pan, lowpass: sp.lp || x.lowpass || 0, rate: x.rate, delay: x.delay, dur: x.dur,
                loop: x.loop, send: sp.lp ? 0.25 : 0.06, vary: x.vary === undefined ? 0.04 : x.vary, bus: x.bus
            });
        }
        /* Same group from the same car inside gap seconds is one sound (the sim emits some actions twice). */
        function fresh(key, gap) {
            const now = ctx.currentTime, last = lastPlayed.get(key);
            if (last !== undefined && now - last < gap) return false;
            lastPlayed.set(key, now);
            return true;
        }
        function duck(db, seconds) { if (music && music.duck) music.duck(db, seconds); }
        const CENTER = { gain: 1, pan: 0, lp: 0 };

        function startWorldLoop(key, group, e, gain) {
            stopWorldLoop(key);
            const h = bank.play(group, { loop: true, gain: 0 });
            if (h) worldLoops.set(key, { h: h, group: group, car: e.car, x: e.x, y: e.y, gain: gain || 1 });
        }
        function stopWorldLoop(key) {
            const l = worldLoops.get(key);
            if (l) { l.h.stop(0.12); worldLoops.delete(key); }
        }

        /* Ends every long-running world loop and the tyre and scrape loops (race end, restart, menu). */
        function stopWorld() {
            for (const key of Array.from(worldLoops.keys())) stopWorldLoop(key);
            for (const name of ['squeal', 'scrape']) {
                const h = loops[name];
                if (h && !h.ended) { try { h.stop(0.1); } catch (_) { /* gone */ } }
                loops[name] = null;
            }
            if (ctx) {
                const now = ctx.currentTime;
                for (const g of [rollGain, windGain, surfaceGain, sprayGain]) if (g) g.gain.setTargetAtTime(0, now, 0.05);
            }
        }

        function roulette(sp) {
            for (let i = 0; i < 6; i++) P('roulette.tick', sp, { delay: i * 0.1 + i * i * 0.012, rate: 1 + i * 0.04 });
            P('roulette.done', sp, { delay: 0.95 });
        }

        function event(evt, race) {
            if (!ctx || muted || !evt || !bank) return;
            const me = mine(evt);
            const sp = evt.x !== undefined ? spatial(evt.x, evt.y) : (me ? CENTER : null);
            switch (evt.type) {
                case 'count': P('ui.count', CENTER, { rate: evt.value > 0 ? 1 : 1.5, vary: 0 }); break;
                case 'go': P('ui.go', CENTER, { vary: 0 }); P('crowd.cheer', CENTER, { gain: 0.7, delay: 0.1 }); break;
                case 'perfect_start': if (evt.car === 0) P('sting.perfect', CENTER); break;
                case 'wheelspin': if (sp && (evt.car === 0 || me)) P('tyre.squeal', sp, { dur: 0.7, gain: 0.8 }); break;
                case 'lap': if (evt.car === 0) P('ui.lap', CENTER); break;
                case 'final_lap': if (evt.car === 0) duck(6, 0.6); break;   /* the stinger itself belongs to the music */
                case 'wall_hit': {
                    const k = clamp((evt.speed || 20) / 40, 0.3, 1);
                    P('impact.metal', sp, { gain: k });
                    if (evt.heavy) P('impact.heavy', sp, { gain: 0.8 * k });
                    break;
                }
                case 'car_hit': {
                    const k = clamp((evt.impulse || 10) / 20, 0.35, 1);
                    P(evt.heavy ? 'impact.heavy' : 'impact.light', sp, { gain: k });
                    if (evt.heavy) P('impact.metal', sp, { gain: 0.5 * k, delay: 0.02 });
                    break;
                }
                case 'drift_level': if (evt.car === 0) P('turbo.tick', CENTER, { rate: 1 + (evt.level || 1) * 0.1, vary: 0 }); break;
                case 'turbo_burst': P('turbo.' + clamp(Math.round(evt.level || 1), 1, 3), evt.car === 0 ? CENTER : sp); break;
                case 'style_bank': if (evt.car === 0) P('ui.bank', CENTER, { rate: 1 + Math.min(8, evt.multiplier || 1) * 0.03, vary: 0 }); break;
                case 'near_miss': if (evt.car === 0 || evt.other === 0) { if (fresh('near', 0.15)) P('whoosh', sp || CENTER, { gain: 0.9 }); } break;
                case 'slingshot': if (evt.car === 0) P('sting.slingshot', CENTER); break;
                case 'takeoff': P('ramp.launch', evt.car === 0 ? CENTER : sp); break;
                case 'trick': if (evt.car === 0) P('sting.trick', CENTER, { rate: 1 + Math.min(5, evt.chain || 0) * 0.05 }); break;
                case 'landing': {
                    const s2 = evt.car === 0 ? CENTER : sp;
                    const q = evt.quality;
                    P(q === 'crash' || q === 'wobble' ? 'land.hard' : 'land.soft', s2, { gain: q === 'wobble' ? 0.7 : 1 });
                    if (q === 'crash') {
                        P('impact.heavy', s2, { gain: 0.8 });
                        if (evt.car === 0) { P('sting.pancake', CENTER, { delay: 0.05 }); P('crowd.aww', CENTER, { gain: 0.8, delay: 0.3 }); }
                    }
                    break;
                }
                case 'j_turn': if (sp && (evt.car === 0 || me)) P('tyre.squeal', sp, { dur: 0.8, gain: 0.9 }); break;
                case 'boost_pad': P('pad.boost', evt.car === 0 ? CENTER : sp); break;
                case 'pickup':
                    P('crate.break', sp);
                    if (evt.car === 0) roulette(CENTER);
                    break;
                case 'smash': {
                    const key = Object.keys(PROP_GROUP).filter(k => String(evt.kind).indexOf(k) >= 0)[0];
                    P(key ? PROP_GROUP[key] : 'impact.light', sp);
                    break;
                }
                case 'animal_cross': P('animal.' + evt.kind, sp, { gain: 0.6 }); break;
                case 'animal_hit':
                    P('animal.' + evt.kind, sp);
                    if (evt.kind === 'cow') P('sting.moo', sp, { delay: 0.05 });
                    break;
                case 'strike': P('sting.strike', evt.car === 0 ? CENTER : sp); P('crowd.cheer', CENTER, { gain: 0.8, delay: 0.15 }); break;
                case 'ufo_abduct': P('ufo.beam', sp); break;
                case 'weapon': {
                    const group = WEAPON_GROUP[evt.id];
                    if (group && fresh(group + evt.car, 0.12)) P(group, sp, evt.id === 'glove' ? { rate: 0.85, gain: 0.6 } : null);
                    break;
                }
                case 'weapon_end': stopWorldLoop('flame' + evt.car); break;
                case 'hit':
                    P(evt.heavy ? 'explosion.big' : 'explosion.small', sp);
                    if (sp && sp.gain > 0.3) duck(6, 0.4);
                    break;
                case 'shield': if (fresh('shield_on' + evt.car, 0.12)) P('weapon.shield_on', sp); break;
                case 'shield_block': P('weapon.shield_hit', sp); break;
                case 'rocket_lock': if (evt.car === 0 || evt.target === 0) P('weapon.rocket_lock', CENTER); break;
                case 'emp_charge': if (fresh('weapon.emp_charge' + evt.car, 0.12)) P('weapon.emp_charge', sp); break;
                case 'emp_pulse': P('weapon.emp_pulse', sp); if (sp && sp.gain > 0.3) duck(6, 0.4); break;
                case 'rail_charge': P('weapon.rail_charge', sp); break;
                case 'rail_fire': P('weapon.rail_fire', sp); if (sp && sp.gain > 0.3) duck(6, 0.4); break;
                case 'flame_start': startWorldLoop('flame' + evt.car, 'weapon.flame_loop', evt); break;
                case 'flame_end': stopWorldLoop('flame' + evt.car); break;
                case 'frost_throw': P('weapon.frost_throw', sp); break;
                case 'frost_burst': P('weapon.frost_burst', sp); break;
                case 'horn': P('weapon.horn_blast', sp); break;
                case 'hole_open': startWorldLoop('hole', 'weapon.hole_loop', evt); break;
                case 'hole_close': stopWorldLoop('hole'); break;
                case 'disco_on': P('weapon.disco_on', sp || CENTER); break;
                case 'glove': if (evt.hit) P('weapon.glove', sp, { rate: 1.15 }); break;
                case 'harpoon_fire': if (fresh('weapon.harpoon_fire' + evt.car, 0.12)) P('weapon.harpoon_fire', sp || CENTER); break;
                case 'harpoon_attach': P('weapon.harpoon_clank', sp || CENTER); break;
                case 'cow_launch': if (fresh('weapon.cow_launch' + evt.car, 0.12)) P('weapon.cow_launch', sp || CENTER); break;
                case 'cow_land': P('weapon.cow_land', sp); if (sp && sp.gain > 0.3) duck(6, 0.4); break;
                case 'duck_launch': if (fresh('weapon.duck_launch' + evt.car, 0.12)) P('weapon.duck_launch', sp || CENTER); for (let i = 0; i < Math.min(3, evt.count || 3); i++) P('weapon.duck_quack', sp || CENTER, { delay: 0.15 + i * 0.13, rate: 0.9 + i * 0.1, gain: 0.7 }); break;
                case 'duck_hit': P('weapon.duck_quack', sp); break;
                case 'cloak_on': P('weapon.cloak_on', sp || CENTER); break;
                case 'overdrive_on': P('weapon.overdrive_on', sp || CENTER); break;
                case 'eliminated':
                    P('explosion.big', sp || CENTER);
                    if (evt.car === 0) P('crowd.aww', CENTER, { delay: 0.2 });
                    duck(6, 0.6);
                    break;
                case 'bomb_given': P('roulette.done', sp || (me ? CENTER : null), { rate: 0.8 }); break;
                case 'bomb_pass': P('whoosh', sp); break;
                case 'bomb_tick': {
                    const target = race && race.cars && race.cars[evt.car];
                    const s = evt.car === 0 ? CENTER : (target && target.v ? spatial(target.v.x, target.v.y) : null);
                    P('bomb.tick', s, { rate: clamp(1.7 - (evt.fuse || 0) * 0.1, 0.9, 1.8), vary: 0 });
                    break;
                }
                case 'bomb_blast': P('bomb.blast', sp); if (sp && sp.gain > 0.2) duck(6, 0.6); break;
                case 'respawn': if (evt.car === 0) P('whoosh', CENTER, { rate: 0.8, gain: 0.6 }); break;
                case 'clock': if (evt.seconds > 0 && evt.seconds <= 10) P('ui.count', CENTER, { gain: 0.6, vary: 0 }); break;
                default: break;
            }
            if (music) {
                if (evt.type === 'finish' && evt.car === 0 && evt.position === undefined && race && race.standings) {
                    const place = race.standings.indexOf(race.cars[0]) + 1;
                    music.event(place > 0 ? Object.assign({}, evt, { position: place }) : evt);
                } else music.event(evt);
            }
        }

        function ui(kind) {
            if (!ctx || muted || !bank) return;
            const o = { dest: sfxBus };
            if (kind === 'confirm') { bank.tone(560, 0.09, 'triangle', 0.10, 780, o); bank.tone(840, 0.12, 'sine', 0.05, 1120, o); }
            else if (kind === 'back') bank.tone(420, 0.10, 'triangle', 0.08, 260, o);
            else if (kind === 'move') bank.tone(700, 0.04, 'sine', 0.05, 0, o);
            else if (kind === 'start') { bank.tone(330, 0.16, 'sawtooth', 0.09, 660, o); bank.noiseHit(0.06, 0.2, 1800, 'bandpass', o); }
        }

        function horn() {
            if (!ctx || muted || !bank) return;
            const now = ctx.currentTime;
            const again = now - lastHornAt < 1.2;
            lastHornAt = now;
            P('horn.' + (carId || 'vektor'), CENTER, { vary: 0.02 });
            /* The ice-cream van answers a second press with its full jingle. */
            if (carId === 'softeis' && again) P('jingle.softeis', CENTER, { delay: 0.2 });
        }

        /* Per-surface tyre character: asphalt hums, gravel rattles, snow crunches, wet asphalt sprays, ice is almost silent. */
        function surfaceProfile(surface) {
            switch (surface) {
                case 'gravel': return { freq: 1500, type: 'bandpass', level: 1.0, q: 1.2 };
                case 'sand': return { freq: 420, type: 'lowpass', level: 1.15, q: 0.8 };
                case 'mud': return { freq: 700, type: 'bandpass', level: 0.85, q: 2.2 };
                case 'snow': return { freq: 950, type: 'bandpass', level: 0.7, q: 1.4 };
                case 'ice': return { freq: 2600, type: 'highpass', level: 0.25, q: 0.7 };
                case 'wet': return { freq: 2200, type: 'highpass', level: 0.8, q: 0.7 };
                case 'salt': return { freq: 1800, type: 'bandpass', level: 0.55, q: 1.0 };
                default: return { freq: 420, type: 'bandpass', level: 0.5, q: 0.9 };
            }
        }

        /* A loop that started as a live recipe is swapped for the sample once it has been decoded. */
        function loopSlot(name, group) {
            let h = loops[name];
            if (!h || h.ended || (h.synth && bank.has(group))) {
                if (h && !h.ended) h.stop(0.05);
                h = loops[name] = bank.play(group, { loop: true, gain: 0, bus: 'sfx' });
            }
            return h;
        }

        function tyres(v, active, now, race) {
            const speed = Math.max(0, v.vx);
            const surface = v.surface || 'asphalt';
            const profile = surfaceProfile(surface);
            const on = active ? 1 : 0;
            rollGain.gain.setTargetAtTime(clamp(speed / 90, 0, 1) * 0.05 * profile.level * on, now, 0.06);
            surfaceGain.gain.setTargetAtTime(clamp(speed / 80, 0, 1) * 0.09 * profile.level * on, now, 0.08);
            surfaceFilter.type = profile.type;
            surfaceFilter.frequency.setTargetAtTime(profile.freq, now, 0.1);
            if (surfaceFilter.Q) surfaceFilter.Q.setTargetAtTime(profile.q, now, 0.1);
            sprayGain.gain.setTargetAtTime(surface === 'wet' ? clamp(speed / 70, 0, 1) * 0.05 * on : 0, now, 0.1);
            sprayFilter.frequency.setTargetAtTime(1800 + speed * 20, now, 0.1);
            windGain.gain.setTargetAtTime((clamp(speed / 90, 0, 1) * 0.055 + clamp(v.slipstream || 0, 0, 1) * 0.03) * on, now, 0.1);

            if (active && !muted && !v.airborne && speed > 10 && race && race.track && typeof AD.sampleTrack === 'function') {
                const sm = AD.sampleTrack(race.track, v.x, v.y);
                if (sm && Math.abs(Math.abs(sm.offset) - sm.halfWidth) < 1.15 && now - lastKerbAt > clamp(1.1 / speed, 0.024, 0.075)) {
                    lastKerbAt = now;
                    bank.tone(68 + speed * 1.1, 0.03, 'sawtooth', 0.04, 52 + speed * 0.8, { dest: sfxBus });
                }
            }
            if (active && !muted && race && race.cars && now - lastPassAt > 0.45) {
                for (let i = 1; i < race.cars.length; i++) {
                    const rv = race.cars[i] && race.cars[i].v;
                    if (!rv || rv.out) continue;
                    const d = Math.hypot(rv.x - v.x, rv.y - v.y);
                    const rel = Math.hypot((rv.vx || 0) * Math.cos(rv.psi || 0) - (v.vx || 0) * Math.cos(v.psi || 0), (rv.vx || 0) * Math.sin(rv.psi || 0) - (v.vx || 0) * Math.sin(v.psi || 0));
                    if (d < 6.5 && rel > 22) {
                        lastPassAt = now;
                        P('whoosh', spatial(rv.x, rv.y), { rate: clamp(0.85 + rel / 65, 0.9, 1.45), gain: 0.55 });
                        break;
                    }
                }
            }

            const squeal = loopSlot('squeal', 'tyre.squeal');
            if (squeal) {
                const k = clamp((v.slip - 1.6) * 0.25, 0, 1) * (surface === 'ice' ? 1.2 : 1) * (v.airborne ? 0 : on);
                squeal.setGain(k, 0.04);
                squeal.setRate(0.85 + clamp(v.slip, 0, 6) * 0.05, 0.06);
            }
            const scrape = loopSlot('scrape', 'scrape.metal');
            if (scrape) scrape.setGain(clamp(v.impact * 1.2, 0, 1) * on, 0.03);
        }

        function followLoops(race) {
            for (const l of worldLoops.values()) {
                const car = l.car !== undefined && race.cars[l.car];
                const sp = car && car.v ? spatial(car.v.x, car.v.y) : spatial(l.x, l.y);
                l.h.setGain(sp ? sp.gain * l.gain : 0, 0.08);
                if (sp) { l.h.setPan(sp.pan); if (sp.lp) l.h.setLowpass(sp.lp); }
            }
        }

        return {
            unlock,
            available() { return !!Ctor; },
            ui,
            setSettings(next) {
                volume = clamp(next.volume, 0, 1); muted = !!next.muted;
                if (master && ctx) master.gain.setTargetAtTime(muted ? 0 : volume * MASTER_TRIM, ctx.currentTime, 0.03);
                /* The voice bus carries the volume of the player too. */
                if (voiceBus && ctx) voiceBus.gain.setTargetAtTime(muted ? 0 : volume * MASTER_TRIM, ctx.currentTime, 0.03);
            },
            setBiome(id) {
                biomeId = id;
                if (ctx && music) applyBiome(id);
            },
            setCar(id) {
                if (!id) return;
                carId = id;
                if (ctx && engine) engine.setCar(id);
                if (ctx && bank) bank.load(['horn.' + id], 2);
            },
            /* 'menu' | 'results' | 'race': menu and results have their own tracks. */
            setView(next) {
                view = next === 'menu' || next === 'results' ? next : 'race';
                if (ctx && music) music.setView(view);
                applyVoiceMix(0.25);
            },
            stopWorld,
            worldLoopCount() { return worldLoops.size; },
            horn,
            /* The voice commentary: samples arrive from the Woof worker at 24 kHz and
             * the context resamples them on playback. */
            voice(samples, rate) {
                if (!ctx || !voiceBus || !samples || !samples.length) return 0;
                const buffer = ctx.createBuffer(1, samples.length, rate || 24000);
                const target = buffer.getChannelData(0);
                /* The model speaks quietly: the line is lifted to a fixed speech level,
                 * so one driver is as loud as the next. */
                const gain = voiceGain(samples);
                const count = Math.min(samples.length, target.length);
                for (let i = 0; i < count; i++) target[i] = samples[i] * gain;
                const src = ctx.createBufferSource();
                src.buffer = buffer;
                src.connect(voiceBus);
                src.start();
                liveNodes.push(src);
                /* The race steps back while the line is spoken and comes back after. */
                voicesPlaying++;
                if (voicesPlaying === 1) { voiceSpeaking = true; applyVoiceMix(0.08); }
                src.onended = () => {
                    voicesPlaying = Math.max(0, voicesPlaying - 1);
                    if (voicesPlaying === 0) { voiceSpeaking = false; applyVoiceMix(0.3); }
                };
                return buffer.duration;
            },
            update(race, active, pose) {
                if (!ctx || !running) return;
                const v = race.cars[0].v;
                const now = ctx.currentTime;
                const tick = clamp(now - lastTick, 0.004, 0.1);
                lastTick = now;
                heading = pose && pose.heading !== undefined ? pose.heading : v.psi;
                myX = v.x; myY = v.y;
                if (engine) engine.update(race, active, tick, heading);
                tyres(v, active, now, race);
                followLoops(race);
                const conditions = race.conditions ? race.conditions.state : null;
                const player = race.cars[0];
                const speed = Math.max(0, v.vx);
                const flags = {
                    nitro: !!v.boost,
                    finalLap: !!race.laps && player.lap >= race.laps - 1 && race.laps > 1,
                    lastPlace: race.cars.length > 1 && !!race.standings && race.standings[race.standings.length - 1] === player
                };
                music.update(tick, clamp(speed / 70, 0, 1), conditions, flags);
                if (conditions && conditions.ambience) music.setAmbience(conditions.ambience);
            },
            event,
            stats() {
                const s = bank ? bank.stats() : { samples: 0, failed: 0, fallback: 0 };
                return { samples: s.samples, failed: s.failed, fallback: s.fallback, voices: s.voices || 0, engine: engine ? engine.mode() : 'none' };
            },
            suspend() { if (ctx && ctx.state === 'running') ctx.suspend().catch(() => {}); running = false; },
            resume() { if (ctx && ctx.state === 'suspended') ctx.resume().catch(() => {}); running = true; },
            dispose() {
                running = false;
                if (music) { try { music.stop(); } catch (_) { /* stopped */ } }
                if (engine) engine.dispose();
                for (const l of worldLoops.values()) { try { l.h.stop(0); } catch (_) { /* gone */ } }
                worldLoops.clear();
                if (bank) bank.dispose();
                for (const src of liveNodes) { try { src.stop(); } catch (_) { /* stopped */ } }
                liveNodes = [];
                if (ctx) { try { ctx.close(); } catch (_) { /* already closed */ } }
                ctx = null; master = null; comp = null; limiter = null; reverb = null; reverbSend = null;
                bank = null; engine = null; music = null; voiceBus = null;
                voiceSpeaking = false; voicesPlaying = 0;
                loops.squeal = null; loops.scrape = null;
            }
        };
    };
})();
