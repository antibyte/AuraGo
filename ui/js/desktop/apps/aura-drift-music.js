/* Aura Drift - adaptive score and biome ambience, all synthesised offline. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    const SCALES = {
        alpine: [0, 3, 5, 7, 10], desert: [0, 2, 5, 7, 9],
        harbour: [0, 3, 7, 10, 12], salt: [0, 2, 4, 7, 9],
        forest: [0, 2, 5, 7, 11], industry: [0, 1, 5, 7, 8],
        glacier: [0, 2, 3, 7, 10], metro: [0, 2, 4, 7, 9]
    };
    const TEMPI = {
        alpine: 100, desert: 118, harbour: 92,
        salt: 106, forest: 96, industry: 128,
        glacier: 84, metro: 122
    };
    const PROGRESSION = [0, 3, 1, 4];

    /* Every biome plays a different arrangement of the same sixteen-step grid:
       the drums, the bass rhythm and the lead voice all change. */
    const PATTERNS = {
        alpine: { kick: [0, 8], snare: [4, 12], hat: [1, 3, 5, 7, 9, 11, 13, 15], bass: [0, 6, 8, 14], lead: [2, 10], pad: true },
        desert: { kick: [0, 6, 8, 14], snare: [4, 12], hat: [2, 6, 10, 14], bass: [0, 3, 8, 11], lead: [5, 13], pad: true },
        harbour: { kick: [0, 3, 8, 11], snare: [4, 12], hat: [2, 6, 10, 14], bass: [0, 8], lead: [7, 15], pad: true },
        salt: { kick: [0, 4, 8, 12], snare: [4, 12], hat: [2, 6, 10, 14], bass: [0, 4, 8, 12], lead: [3, 11], pad: true },
        forest: { kick: [0, 6, 10], snare: [4, 12], hat: [1, 5, 9, 13], bass: [0, 5, 10], lead: [2, 7, 12], pad: false },
        industry: { kick: [0, 4, 8, 12], snare: [4, 12], hat: [2, 6, 10, 14], bass: [0, 2, 8, 10], lead: [6, 14], pad: true },
        glacier: { kick: [0, 10], snare: [6], hat: [4, 12], bass: [0, 8], lead: [7], pad: true },
        metro: { kick: [0, 4, 8, 12], snare: [4, 12], hat: [2, 6, 10, 14], bass: [0, 3, 8, 11], lead: [5, 13], pad: true }
    };

    /* The live synthesiser: the fallback when the sample pack is missing, and
       the ambience / sting layer that stays available next to it. `mode`
       switches the parts the pack replaces: the score, the beds with their
       one-shots, and the finish jingle. */
    function createLiveScore(ctx, noiseBuffer, destination) {
        const mode = { score: true, ambience: true, finish: true };
        let musicKey = 'alpine';
        let nextNote = ctx.currentTime + 0.1;
        let noteIndex = 0;
        let running = true;
        let ambience = { wind: 0.4, birds: 0, machinery: 0, water: 0, crowd: 0, reverb: 0.1 };
        let bedGain = null;
        const beds = {};
        let birdTimer = 2;
        let ambienceClock = 0;

        function noiseSource(buffer) {
            const source = ctx.createBufferSource();
            source.buffer = buffer;
            source.loop = true;
            source.start();
            return source;
        }

        function bed(filterType, freq, q) {
            const source = noiseSource(noiseBuffer);
            const filter = ctx.createBiquadFilter();
            filter.type = filterType; filter.frequency.value = freq;
            if (q) filter.Q.value = q;
            const gain = ctx.createGain(); gain.gain.value = 0;
            source.connect(filter).connect(gain).connect(destination);
            beds[filterType + ':' + freq] = { source, filter, gain };
            return beds[filterType + ':' + freq];
        }

        /* Ambience beds: wind, water, machinery and crowd. Birds and metal
           clanks are one-shots fired from the update loop. */
        function buildBeds() {
            bed('highpass', 700, 0.6);       /* wind */
            bed('lowpass', 420, 0.8);        /* water */
            bed('bandpass', 180, 1.6);       /* machinery */
            bed('bandpass', 900, 0.7);       /* crowd */
        }
        buildBeds();

        function noteFreq(scale, degree, root) {
            const steps = scale.length;
            const octave = Math.floor(degree / steps);
            const semi = scale[((degree % steps) + steps) % steps];
            return root * Math.pow(2, (semi + octave * 12) / 12);
        }

        function kick(time, level) {
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = 'sine';
            osc.frequency.setValueAtTime(128, time);
            osc.frequency.exponentialRampToValueAtTime(44, time + 0.11);
            gain.gain.setValueAtTime(level, time);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.16);
            osc.connect(gain).connect(destination);
            osc.start(time); osc.stop(time + 0.18);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); };
        }

        function hat(time, open) {
            const source = ctx.createBufferSource(); source.buffer = noiseBuffer;
            const filter = ctx.createBiquadFilter(); filter.type = 'highpass'; filter.frequency.value = 6800;
            const gain = ctx.createGain();
            gain.gain.setValueAtTime(open ? 0.12 : 0.07, time);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + (open ? 0.12 : 0.045));
            source.connect(filter).connect(gain).connect(destination);
            source.start(time); source.stop(time + 0.16);
            source.onended = () => { source.disconnect(); filter.disconnect(); gain.disconnect(); };
        }

        function snare(time, level) {
            const source = ctx.createBufferSource(); source.buffer = noiseBuffer;
            const filter = ctx.createBiquadFilter(); filter.type = 'bandpass'; filter.frequency.value = 2100; filter.Q.value = 0.8;
            const gain = ctx.createGain();
            gain.gain.setValueAtTime(level, time);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.11);
            source.connect(filter).connect(gain).connect(destination);
            source.start(time); source.stop(time + 0.13);
            source.onended = () => { source.disconnect(); filter.disconnect(); gain.disconnect(); };
            const osc = ctx.createOscillator(), og = ctx.createGain();
            osc.type = 'triangle'; osc.frequency.value = 190;
            og.gain.setValueAtTime(level * 0.7, time);
            og.gain.exponentialRampToValueAtTime(0.0001, time + 0.08);
            osc.connect(og).connect(destination);
            osc.start(time); osc.stop(time + 0.1);
            osc.onended = () => { osc.disconnect(); og.disconnect(); };
        }

        function bass(time, freq, dur, level) {
            const osc = ctx.createOscillator(), gain = ctx.createGain(), filter = ctx.createBiquadFilter();
            osc.type = musicKey === 'forest' ? 'triangle' : 'sawtooth';
            osc.frequency.value = freq;
            filter.type = 'lowpass'; filter.frequency.value = 420;
            gain.gain.setValueAtTime(0.0001, time);
            gain.gain.exponentialRampToValueAtTime(level, time + 0.012);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + dur);
            osc.connect(filter).connect(gain).connect(destination);
            osc.start(time); osc.stop(time + dur + 0.03);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); filter.disconnect(); };
        }

        function pluck(time, freq, level) {
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = 'triangle'; osc.frequency.value = freq;
            gain.gain.setValueAtTime(0.0001, time);
            gain.gain.exponentialRampToValueAtTime(level, time + 0.01);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.22);
            osc.connect(gain).connect(destination);
            osc.start(time); osc.stop(time + 0.25);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); };
        }

        function bell(time, freq, level) {
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = 'sine'; osc.frequency.value = freq;
            gain.gain.setValueAtTime(0.0001, time);
            gain.gain.exponentialRampToValueAtTime(level, time + 0.006);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 1.1);
            osc.connect(gain).connect(destination);
            osc.start(time); osc.stop(time + 1.2);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); };
        }

        function pad(time, freqs, dur) {
            for (const freq of freqs) {
                for (const detune of [-6, 6]) {
                    const osc = ctx.createOscillator(), gain = ctx.createGain(), filter = ctx.createBiquadFilter();
                    osc.type = 'sawtooth'; osc.frequency.value = freq; osc.detune.value = detune;
                    filter.type = 'lowpass'; filter.frequency.value = 900;
                    gain.gain.setValueAtTime(0.0001, time);
                    gain.gain.exponentialRampToValueAtTime(0.028, time + dur * 0.3);
                    gain.gain.exponentialRampToValueAtTime(0.0001, time + dur);
                    osc.connect(filter).connect(gain).connect(destination);
                    osc.start(time); osc.stop(time + dur + 0.05);
                    osc.onended = () => { osc.disconnect(); gain.disconnect(); filter.disconnect(); };
                }
            }
        }

        /* --- ambience one-shots ------------------------------------------- */
        function birdCall(time) {
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = 'sine';
            const base = 2400 + Math.random() * 900;
            osc.frequency.setValueAtTime(base, time);
            osc.frequency.exponentialRampToValueAtTime(base * 1.5, time + 0.05);
            osc.frequency.exponentialRampToValueAtTime(base * 0.8, time + 0.12);
            gain.gain.setValueAtTime(0.0001, time);
            gain.gain.exponentialRampToValueAtTime(0.05, time + 0.01);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.16);
            osc.connect(gain).connect(destination);
            osc.start(time); osc.stop(time + 0.2);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); };
        }

        function clank(time) {
            const source = ctx.createBufferSource(); source.buffer = noiseBuffer;
            const filter = ctx.createBiquadFilter(); filter.type = 'bandpass';
            filter.frequency.value = 2600 + Math.random() * 900; filter.Q.value = 5;
            const gain = ctx.createGain();
            gain.gain.setValueAtTime(0.07, time);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.26);
            source.connect(filter).connect(gain).connect(destination);
            source.start(time); source.stop(time + 0.3);
            source.onended = () => { source.disconnect(); filter.disconnect(); gain.disconnect(); };
        }

        function gull(time) {
            const osc = ctx.createOscillator(), gain = ctx.createGain();
            osc.type = 'sawtooth';
            osc.frequency.setValueAtTime(900, time);
            osc.frequency.exponentialRampToValueAtTime(1500, time + 0.07);
            osc.frequency.exponentialRampToValueAtTime(800, time + 0.22);
            const filter = ctx.createBiquadFilter(); filter.type = 'bandpass';
            filter.frequency.value = 1600; filter.Q.value = 3;
            gain.gain.setValueAtTime(0.0001, time);
            gain.gain.exponentialRampToValueAtTime(0.045, time + 0.02);
            gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.26);
            osc.connect(filter).connect(gain).connect(destination);
            osc.start(time); osc.stop(time + 0.3);
            osc.onended = () => { osc.disconnect(); gain.disconnect(); filter.disconnect(); };
        }

        function scheduleMusic(intensity) {
            if (!running || !mode.score) return;
            const tempo = TEMPI[musicKey] || 104;
            const beat = 60 / tempo / 2;
            const scale = SCALES[musicKey] || SCALES.alpine;
            const pattern = PATTERNS[musicKey] || PATTERNS.alpine;
            const root = musicKey === 'industry' ? 49 : 55;
            while (nextNote < ctx.currentTime + 0.12) {
                const step = noteIndex % 16;
                const bar = Math.floor(noteIndex / 16);
                const degree = PROGRESSION[bar % PROGRESSION.length];

                if (pattern.kick.indexOf(step) >= 0) kick(nextNote, 0.5);
                if (pattern.snare.indexOf(step) >= 0 && intensity > 0.3) {
                    snare(nextNote, 0.10 + intensity * 0.09);
                }
                if (pattern.hat.indexOf(step) >= 0 && intensity > 0.25) {
                    hat(nextNote, step % 8 === 7);
                }
                if (pattern.bass.indexOf(step) >= 0) {
                    bass(nextNote, noteFreq(scale, degree, root), beat * 1.7, 0.15 + intensity * 0.08);
                    if (intensity > 0.55 && step % 4 === 2) {
                        bass(nextNote, noteFreq(scale, degree + 2, root * 2), beat * 0.8, 0.06);
                    }
                }
                if (pattern.lead.indexOf(step) >= 0 && intensity > 0.42) {
                    if (musicKey === 'glacier') {
                        bell(nextNote, noteFreq(scale, degree + 4, root * 8), 0.05 + intensity * 0.05);
                    } else {
                        pluck(nextNote, noteFreq(scale, degree + 2 + (step % 3), root * 8), 0.045 + intensity * 0.05);
                    }
                }
                if (intensity > 0.68 && pattern.pad && step === 0) {
                    pad(nextNote, [
                        noteFreq(scale, degree, root * 4),
                        noteFreq(scale, degree + 2, root * 4),
                        noteFreq(scale, degree + 4, root * 4)
                    ], beat * 8);
                }
                nextNote += beat;
                noteIndex++;
            }
        }

        function updateAmbience(dt, conditions) {
            const mix = ambience;
            /* The rare one-shots (birds, gulls, distant clanks) run on their
               own clock, independent of the bed level throttle. */
            birdTimer -= dt;
            if (!mode.ambience) {
                ambienceClock -= dt;
                if (ambienceClock > 0) return;
                ambienceClock = 0.25;
                for (const key of Object.keys(beds)) beds[key].gain.gain.setTargetAtTime(0, ctx.currentTime, 0.3);
                return;
            }
            if (birdTimer <= 0) {
                birdTimer = 2.5 + Math.random() * 6;
                const at = ctx.currentTime;
                if ((mix.birds || 0) > 0.2) birdCall(at + Math.random() * 0.2);
                else if ((mix.water || 0) > 0.4 && Math.random() > 0.5) gull(at);
                else if ((mix.machinery || 0) > 0.4 && Math.random() > 0.6) clank(at);
            }
            /* The bed levels move slowly, so they are re-targeted a few times
               per second instead of every frame. */
            ambienceClock -= dt;
            if (ambienceClock > 0) return;
            ambienceClock = 0.25;
            const now = ctx.currentTime;
            const windLevel = clamp((mix.wind || 0) * 0.05 + (conditions && conditions.wind ? conditions.wind * 0.03 : 0), 0, 0.12);
            const windBed = beds['highpass:700'];
            const waterBed = beds['lowpass:420'];
            const machineBed = beds['bandpass:180'];
            const crowdBed = beds['bandpass:900'];
            if (windBed) windBed.gain.gain.setTargetAtTime(windLevel, now, 0.4);
            if (windBed) windBed.filter.frequency.setTargetAtTime(600 + (conditions ? conditions.intensity * 500 : 0), now, 0.6);
            if (waterBed) waterBed.gain.gain.setTargetAtTime((mix.water || 0) * 0.035, now, 0.6);
            if (machineBed) machineBed.gain.gain.setTargetAtTime((mix.machinery || 0) * 0.045, now, 0.6);
            if (crowdBed) crowdBed.gain.gain.setTargetAtTime((mix.crowd || 0) * 0.03, now, 0.8);
        }

        return {
            setMode(next) {
                /* A score that was off has missed its beats: restart the clock instead of bursting them out. */
                if (next.score && !mode.score) { nextNote = ctx.currentTime + 0.1; noteIndex = 0; }
                Object.assign(mode, next);
            },
            setBiome(id) {
                musicKey = Object.prototype.hasOwnProperty.call(SCALES, id) ? id : 'alpine';
                noteIndex = 0;
                nextNote = ctx.currentTime + 0.1;
                const profile = AD.conditionProfile ? AD.conditionProfile(musicKey) : null;
                if (profile && profile.ambience) ambience = profile.ambience;
            },
            setAmbience(next) { if (next) ambience = next; },
            update(dt, intensity, conditions) {
                if (!running) return;
                scheduleMusic(clamp(intensity, 0, 1));
                updateAmbience(dt, conditions);
            },
            /* The score reacts to the race: pickups and laps get their sting. */
            event(evt) {
                if (!running || !evt) return;
                const now = ctx.currentTime;
                if (evt.type === 'pickup') {
                    pluck(now, 720, 0.05); pluck(now + 0.07, 1080, 0.04);
                } else if (evt.type === 'lap') {
                    bell(now, 660, 0.06); bell(now + 0.12, 990, 0.05);
                } else if (evt.type === 'finish' && mode.finish) {
                    [523, 659, 784, 1046].forEach((f, i) => bell(now + i * 0.13, f, 0.07));
                }
            },
            stop() {
                running = false;
                for (const key of Object.keys(beds)) {
                    const item = beds[key];
                    try { item.source.stop(); } catch (_) { /* already stopped */ }
                    try { item.gain.disconnect(); } catch (_) { /* already gone */ }
                    delete beds[key];
                }
            }
        };
    }

    /* ------------------------------------------------------------------ *
     * Sample pack playback
     * ------------------------------------------------------------------ */
    const PACK_ROOT = '/3d/aura-drift/audio/';
    const PACK_LEVEL = 0.7;        /* stems are normalised to -16 LUFS summed; the live synth is quieter */
    const LAYER_SMOOTH = 0.8;      /* seconds for a layer gain to settle */
    const CROSSFADE = 1.5;         /* biome / view change */
    const PEAK_OPEN = 18000, PEAK_DULL = 700, DISCO_SECONDS = 4;
    const AMBIENCE_LEVEL = 0.5;
    const STINGER_LATE = 2;        /* a stinger that is still loading after this long is dropped */
    const AMB_GROUP = {
        alpine: 'amb.wind', desert: 'amb.desert', harbour: 'amb.sea', salt: 'amb.wind',
        forest: 'amb.forest', industry: 'amb.machinery', glacier: 'amb.snow', metro: 'amb.city'
    };
    const STINGERS = ['stinger.final_lap', 'stinger.disco', 'fanfare.win', 'fanfare.podium', 'fanfare.lose'];

    /* Adapter between the music and the sample bank. It prefers whatever the
       bank (AD.createSampleBank, T6.3) offers and otherwise fetches and
       decodes from the pack itself, so the music works with or without it.
         bank.musicBuffer(path) | bank.buffer(path)  -> AudioBuffer or Promise
         bank.groupBuffer(group) | bank.pick(group)  -> AudioBuffer or Promise
       Everything resolves to null on failure; nothing here ever throws. */
    function createPackAdapter(ctx, bank) {
        const buffers = new Map();
        let manifestP = null;

        function decode(bytes) {
            return new Promise((resolve, reject) => {
                const p = ctx.decodeAudioData(bytes, resolve, reject);
                if (p && typeof p.then === 'function') p.then(resolve, reject);
            });
        }
        function viaBank(names, arg) {
            if (!bank) return null;
            for (const name of names) {
                if (typeof bank[name] === 'function') {
                    try { return Promise.resolve(bank[name](arg)); } catch (_) { return null; }
                }
            }
            return null;
        }
        function buffer(path) {
            if (!path) return Promise.resolve(null);
            if (!buffers.has(path)) {
                let p = viaBank(['musicBuffer', 'buffer'], path);
                if (!p && AD.assetsFetchBytes) {
                    p = AD.assetsFetchBytes(PACK_ROOT + path).then(decode);
                }
                buffers.set(path, (p || Promise.resolve(null)).then(b => b || null, () => null));
            }
            return buffers.get(path);
        }
        function manifest() {
            if (!manifestP) {
                manifestP = AD.assetsFetchJson
                    ? AD.assetsFetchJson(PACK_ROOT + 'manifest.json').catch(() => null)
                    : Promise.resolve(null);
            }
            return manifestP;
        }
        return {
            buffer,
            /* The three stems of a biome, or the single file of menu / results. */
            scene(key) {
                return manifest().then(m => {
                    const entry = m && m.music && m.music[key];
                    if (!entry) return null;
                    if (typeof entry === 'string') {
                        return buffer(entry).then(b => b ? { bed: b } : null);
                    }
                    return Promise.all([buffer(entry.bed), buffer(entry.drive), buffer(entry.peak)]).then(list => (
                        list[0] && list[1] && list[2] ? { bed: list[0], drive: list[1], peak: list[2] } : null
                    ));
                });
            },
            /* A random member of an sfx group (stingers, fanfares, ambience). */
            group(name) {
                const direct = viaBank(['groupBuffer', 'pick'], name);
                if (direct) return direct.then(b => b || null, () => null);
                return manifest().then(m => {
                    const list = m && m.sfx && m.sfx[name];
                    if (!list || !list.length) return null;
                    return buffer(list[Math.floor(Math.random() * list.length)]);
                });
            }
        };
    }

    AD.createMusicEngine = function (ctx, noiseBuffer, destination, bank) {
        /* Everything the score plays goes through the duck gain; stingers and
           ambience bypass it so a fanfare is not ducked by itself. */
        const duckGain = ctx.createGain();
        duckGain.gain.value = 1;
        duckGain.connect(destination);
        const live = createLiveScore(ctx, noiseBuffer, duckGain);
        const pack = createPackAdapter(ctx, bank);

        let running = true;
        let biome = 'alpine';
        let view = 'race';                 /* 'race' | 'menu' | 'results' */
        let scene = null;                  /* the scene that is audible */
        let requestedKey = null, requestToken = 0;
        const fading = new Set();          /* scenes on their way out */
        const failed = new Set();          /* keys whose pack failed to load */
        const oneShots = new Set();
        let ambNode = null, ambKey = null, ambToken = 0;
        const ambFailed = new Set();
        let flags = { nitro: false, finalLap: false, lastPlace: false };
        let discoUntil = 0;
        let finishers = 0;
        let level = { drive: false, peak: false };
        let gainClock = 0;
        let lastIntensity = 0;

        function safe(fn) { try { return fn(); } catch (_) { return undefined; } }

        function killNode(node) {
            if (!node) return;
            for (const source of node.sources || []) {
                safe(() => source.stop());
                safe(() => source.disconnect());
            }
            for (const item of node.others || []) safe(() => item.disconnect());
        }

        /* ---- scenes ------------------------------------------------- */
        function fadeTo(param, value, seconds) {
            const now = ctx.currentTime;
            safe(() => {
                param.cancelScheduledValues(now);
                param.setValueAtTime(param.value, now);
                param.linearRampToValueAtTime(value, now + seconds);
            });
        }

        function startScene(key, stems) {
            const now = ctx.currentTime;
            const t0 = now + 0.05;           /* one start time for every stem keeps them phase locked */
            const master = ctx.createGain();
            master.gain.value = 0;
            master.connect(duckGain);
            const node = { key, master, sources: [], others: [master], layers: {}, filter: null };
            for (const name of ['bed', 'drive', 'peak']) {
                const buffer = stems[name];
                if (!buffer) continue;
                const source = ctx.createBufferSource();
                source.buffer = buffer;
                source.loop = true;
                const gain = ctx.createGain();
                gain.gain.value = name === 'bed' ? 1 : 0;
                let tail = source.connect(gain);
                if (name === 'peak') {
                    const filter = ctx.createBiquadFilter();
                    filter.type = 'lowpass';
                    filter.frequency.value = now < discoUntil ? PEAK_DULL : PEAK_OPEN;
                    tail.connect(filter);
                    tail = filter;
                    node.filter = filter;
                    node.others.push(filter);
                }
                tail.connect(master);
                source.start(t0);
                node.sources.push(source);
                node.others.push(gain);
                node.layers[name] = gain;
            }
            /* equal gain crossfade from whatever is audible */
            const old = scene;
            master.gain.setValueAtTime(0, now);
            master.gain.linearRampToValueAtTime(PACK_LEVEL, now + CROSSFADE);
            if (old) retire(old);
            scene = node;
            level = { drive: false, peak: false };
            applyLayers(true);
            live.setMode({ score: false, finish: false });
            return node;
        }

        /* Fade a scene or ambience bed out, then stop its sources. */
        function retire(node) {
            fading.add(node);
            fadeTo((node.master || node.gain).gain, 0, CROSSFADE);
            node.timer = setTimeout(() => { fading.delete(node); killNode(node); }, CROSSFADE * 1000 + 150);
        }

        function wanted() {
            if (view === 'menu') return 'menu';
            if (view === 'results') return 'results';
            return biome;
        }

        /* Make the audible scene match the wanted one, loading it if needed. */
        function sync() {
            if (!running) return;
            const key = wanted();
            syncAmbience();
            if (scene && scene.key === key) { requestedKey = key; return; }
            if (requestedKey === key || failed.has(key)) {
                if (failed.has(key)) fallBack();
                return;
            }
            requestedKey = key;
            const token = ++requestToken;
            pack.scene(key).then(stems => {
                if (!running || token !== requestToken) return;
                if (!stems) { failed.add(key); fallBack(); return; }
                startScene(key, stems);
                prefetchStingers();
            }, () => { failed.add(key); fallBack(); });
        }

        /* No pack for this key: leave any current scene and let the live
           synthesiser carry the race. */
        function fallBack() {
            if (scene) { retire(scene); scene = null; }
            live.setMode({ score: true, finish: true });
        }

        function prefetchStingers() {
            for (const name of STINGERS) pack.group(name);
        }

        /* ---- layer gains ------------------------------------------- */
        function applyLayers(force) {
            if (!scene || !scene.layers.drive) return;
            const now = ctx.currentTime;
            const i = lastIntensity;
            /* a little hysteresis keeps a layer from flapping around its threshold */
            const drive = i > 0.3 || (level.drive && i > 0.24);
            const peak = i > 0.7 || (level.peak && i > 0.64) || flags.nitro || flags.finalLap || flags.lastPlace;
            if (!force && drive === level.drive && peak === level.peak) return;
            level = { drive, peak };
            const tc = LAYER_SMOOTH / 3;
            safe(() => scene.layers.drive.gain.setTargetAtTime(drive ? 1 : 0, now, tc));
            safe(() => scene.layers.peak.gain.setTargetAtTime(peak ? 1 : 0, now, tc));
        }

        /* ---- ambience beds ----------------------------------------- */
        function syncAmbience() {
            const group = view === 'race' ? AMB_GROUP[biome] : null;
            if (group === ambKey) return;
            ambKey = group;
            const token = ++ambToken;
            if (ambNode) { retire(ambNode); ambNode = null; }
            if (!group || ambFailed.has(group)) { live.setMode({ ambience: true }); return; }
            pack.group(group).then(buffer => {
                if (!running || token !== ambToken) return;
                if (!buffer) { ambFailed.add(group); live.setMode({ ambience: true }); return; }
                const source = ctx.createBufferSource();
                source.buffer = buffer;
                source.loop = true;
                const gain = ctx.createGain();
                gain.gain.value = 0;
                source.connect(gain).connect(destination);
                source.start();
                ambNode = { sources: [source], others: [gain], gain };
                fadeTo(gain.gain, AMBIENCE_LEVEL, CROSSFADE);
                live.setMode({ ambience: false });
            }, () => { ambFailed.add(group); live.setMode({ ambience: true }); });
        }

        /* ---- one-shots --------------------------------------------- */
        function duck(db, seconds) {
            if (!running) return;
            const now = ctx.currentTime;
            const depth = Math.pow(10, -(db === undefined ? 6 : db) / 20);
            const hold = seconds === undefined ? 0.4 : seconds;
            safe(() => {
                duckGain.gain.cancelScheduledValues(now);
                duckGain.gain.setValueAtTime(duckGain.gain.value, now);
                duckGain.gain.linearRampToValueAtTime(depth, now + 0.03);
                duckGain.gain.setValueAtTime(depth, now + hold);
                duckGain.gain.linearRampToValueAtTime(1, now + hold + 0.35);
            });
        }

        function playGroup(name, gainValue, duckSeconds) {
            const asked = ctx.currentTime;
            return pack.group(name).then(buffer => {
                if (!running || !buffer || ctx.currentTime - asked > STINGER_LATE) return;
                const source = ctx.createBufferSource();
                source.buffer = buffer;
                const gain = ctx.createGain();
                gain.gain.value = gainValue;
                source.connect(gain).connect(destination);
                const item = { sources: [source], others: [gain] };
                oneShots.add(item);
                source.onended = () => { oneShots.delete(item); killNode(item); };
                source.start();
                if (duckSeconds) duck(6, Math.min(duckSeconds, Math.max(0.4, buffer.duration * 0.8)));
            }, () => {});
        }

        function discoDip() {
            const now = ctx.currentTime;
            discoUntil = now + DISCO_SECONDS;
            if (scene && scene.filter) {
                const f = scene.filter.frequency;
                safe(() => {
                    f.cancelScheduledValues(now);
                    f.setValueAtTime(f.value, now);
                    f.exponentialRampToValueAtTime(PEAK_DULL, now + 0.25);
                    f.setValueAtTime(PEAK_DULL, now + DISCO_SECONDS - 0.3);
                    f.exponentialRampToValueAtTime(PEAK_OPEN, now + DISCO_SECONDS + 0.5);
                });
            }
        }

        function isPlayer(evt) { return evt.car === undefined || evt.car === 0; }

        /* Finishing place: an explicit field wins, else the order in which
           finish events arrive (the first one home is first). */
        function finishPlace(evt) {
            const direct = evt.position || evt.place || evt.rank;
            return direct > 0 ? direct : finishers;
        }

        function fanfareFor(place) {
            if (place <= 1) return 'fanfare.win';
            if (place <= 3) return 'fanfare.podium';
            return 'fanfare.lose';                         /* the sad trombone */
        }

        function resetRace() {
            flags = { nitro: false, finalLap: false, lastPlace: false };
            finishers = 0;
            level = { drive: false, peak: false };
        }

        return {
            setBiome(id) {
                if (!running) return;
                const next = Object.prototype.hasOwnProperty.call(SCALES, id) ? id : 'alpine';
                if (next !== biome) resetRace();
                biome = next;
                live.setBiome(next);
                sync();
            },
            /* 'race' | 'menu' | 'results': menu and results have their own tracks. */
            setView(next) {
                if (!running) return;
                const v = next === 'menu' || next === 'results' ? next : 'race';
                if (v === view) return;
                view = v;
                if (v === 'race') resetRace();
                sync();
            },
            setAmbience(next) { live.setAmbience(next); },
            /* Extra race state for the peak layer: { nitro, finalLap, lastPlace }.
               `conditions` may carry the same names. */
            update(dt, intensity, conditions, state) {
                if (!running) return;
                lastIntensity = clamp(+intensity || 0, 0, 1);
                const src = state || conditions || {};
                if (state || conditions) {
                    flags.nitro = !!(src.nitro || src.boosting);
                    flags.lastPlace = !!(src.lastPlace || src.chase);
                    if (src.finalLap !== undefined) flags.finalLap = !!src.finalLap;
                }
                gainClock -= dt;
                if (gainClock <= 0) { gainClock = 0.2; applyLayers(false); }
                live.update(dt, lastIntensity, conditions);
            },
            event(evt) {
                if (!running || !evt) return;
                live.event(evt);
                switch (evt.type) {
                    case 'disco_on':
                        playGroup('stinger.disco', 0.9, 1.2);
                        discoDip();
                        break;
                    case 'final_lap':
                        if (!isPlayer(evt)) break;
                        flags.finalLap = true;
                        applyLayers(false);
                        playGroup('stinger.final_lap', 0.9, 1.2);
                        break;
                    case 'finish':
                        finishers++;
                        if (!isPlayer(evt)) break;
                        flags.finalLap = false; flags.nitro = false; flags.lastPlace = false;
                        applyLayers(false);
                        playGroup(fanfareFor(finishPlace(evt)), 1, 4);
                        break;
                }
            },
            /* Explosions and big stingers: music dips 6 dB for 0.4 s. */
            duck,
            stop() {
                if (!running) return;
                running = false;
                requestToken++; ambToken++;
                for (const node of [scene, ambNode, ...fading, ...oneShots]) {
                    if (node && node.timer) clearTimeout(node.timer);
                    killNode(node);
                }
                fading.clear(); oneShots.clear();
                scene = null; ambNode = null;
                live.stop();
                safe(() => duckGain.disconnect());
            }
        };
    };
})();
