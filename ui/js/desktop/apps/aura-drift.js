/* Aura Drift - Virtual Desktop app shell, input, frame loop, lifecycle and debug API. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const instances = new Map();

    /* Every factory the shell needs. A stale service-worker cache can serve an
       older module-loader.js that predates one of these files; fail visibly
       instead of leaving an empty dark window. The three.js vendor bundle,
       render3d, assets, banter, audio-bank, engine, music and the old WebGL2
       post pass are optional: the game falls back to Canvas 2D without them. */
    const REQUIRED = [
        ['aura-drift-core.js', 'loadSettings'],
        ['aura-drift-core.js', 'createScheduler'],
        ['aura-drift-core.js', 'createQualityGovernor'],
        ['aura-drift-physics.js', 'createVehicle'],
        ['aura-drift-track.js', 'buildTrack'],
        ['aura-drift-tricks.js', 'updateVehicleTricks'],
        ['aura-drift-features.js', 'createFeatures'],
        ['aura-drift-weapons.js', 'fireWeapon'],
        ['aura-drift-projectiles.js', 'updateProjectiles'],
        ['aura-drift-modes.js', 'updateMode'],
        ['aura-drift-ai.js', 'aiInput'],
        ['aura-drift-race.js', 'createRace'],
        ['aura-drift-camera.js', 'createChaseCamera'],
        ['aura-drift-render.js', 'createRenderer'],
        ['aura-drift-hud.js', 'createHud'],
        ['aura-drift-audio.js', 'createAudio'],
        ['aura-drift-ui.js', 'createUI']
    ];

    function missingModules() {
        const core = window.AuraDriftCore;
        const missing = [];
        for (const [file, fn] of REQUIRED) {
            if ((!core || typeof core[fn] !== 'function') && missing.indexOf(file) < 0) missing.push(file);
        }
        return missing;
    }

    function showLoadError(root, t, details) {
        const overlay = document.createElement('div');
        overlay.className = 'agdrift-load-error';
        const panel = document.createElement('div');
        panel.className = 'agdrift-load-error-panel';
        const title = document.createElement('h2');
        title.textContent = t('desktop.app_error_title');
        const hint = document.createElement('p');
        hint.textContent = t('aura_drift.load_error_hint');
        const detail = document.createElement('code');
        detail.textContent = details.join(', ');
        const reload = document.createElement('button');
        reload.type = 'button';
        reload.className = 'agdrift-primary';
        reload.textContent = t('aura_drift.restart');
        reload.addEventListener('click', () => window.location.reload(), { once: true });
        panel.append(title, hint, detail, reload);
        overlay.appendChild(panel);
        root.appendChild(overlay);
    }

    /* Canvas 2D remains fully playable when the optional WebGL2 pass is missing. */
    function createNoopPost() {
        return {
            available: false,
            enabled: false,
            setQuality() {}, setBiome() {}, resize() {}, impulse() {}, setHoles() {},
            render() { return false; },
            stats() { return { available: false, enabled: false, degraded: true, quality: 'none', biome: 'none' }; },
            dispose() {}
        };
    }

    function dispose(id) {
        const entry = instances.get(id);
        if (!entry) return;
        entry.teardown();
        window.removeEventListener('keydown', entry.onKeyDown, true);
        window.removeEventListener('keyup', entry.onKeyUp, true);
        window.removeEventListener('blur', entry.onBlur);
        document.removeEventListener('visibilitychange', entry.onVisibility);
        if (entry.observer) entry.observer.disconnect();
        entry.host.replaceChildren();
        instances.delete(id);
    }

    function labelsFor(t) {
        const keys = {
            title: 'aura_drift.title', subtitle: 'aura_drift.subtitle',
            quickRace: 'aura_drift.quick_race', timeTrial: 'aura_drift.time_trial',
            cup: 'aura_drift.cup', daily: 'aura_drift.daily',
            track: 'aura_drift.track', laps: 'aura_drift.laps',
            quality: 'aura_drift.quality', auto: 'aura_drift.auto',
            low: 'aura_drift.low', medium: 'aura_drift.medium',
            high: 'aura_drift.high', ultra: 'aura_drift.ultra',
            sound: 'aura_drift.sound', muted: 'aura_drift.muted',
            weaponsOff: 'aura_drift.weapons_off', reducedMotion: 'aura_drift.reduced_motion',
            showGhost: 'aura_drift.show_ghost', controlsHint: 'aura_drift.controls_hint',
            bestLap: 'aura_drift.best_lap', record: 'aura_drift.record',
            start: 'aura_drift.start', pause: 'aura_drift.pause',
            resume: 'aura_drift.resume', restart: 'aura_drift.restart',
            menu: 'aura_drift.menu', results: 'aura_drift.results',
            finish: 'aura_drift.finish', position: 'aura_drift.position',
            newRecord: 'aura_drift.new_record', nextTrack: 'aura_drift.next_track',
            ready: 'aura_drift.ready', lap: 'aura_drift.lap',
            weapon: 'aura_drift.weapon', loading: 'aura_drift.loading',
            catchup: 'aura_drift.catchup', nitro: 'aura_drift.nitro',
            style: 'aura_drift.style', combo: 'aura_drift.combo',
            sector: 'aura_drift.sector', bestCombo: 'aura_drift.best_combo',
            conditions: 'aura_drift.conditions'
        };
        const result = {};
        for (const [name, key] of Object.entries(keys)) result[name] = t(key);
        return result;
    }

    function dailySeed() {
        return Number(new Date().toISOString().slice(0, 10).replace(/-/g, ''));
    }

    function render(host, id, context) {
        dispose(id);
        context = context || {};
        const t = typeof context.t === 'function' ? context.t : key => key;
        const isActive = typeof context.isActive === 'function' ? context.isActive : () => true;
        const readonly = !!context.readonly;
        const labels = labelsFor(t);

        /* DOM order: gl, world (2D fallback), hud, then touch and overlay from the UI. */
        const root = document.createElement('div');
        root.className = 'agdrift-root';
        const world = document.createElement('canvas');
        world.className = 'agdrift-world';
        const hudCanvas = document.createElement('canvas');
        hudCanvas.className = 'agdrift-hud';
        root.appendChild(world);
        root.appendChild(hudCanvas);
        host.replaceChildren(root);

        const missing = missingModules();
        if (missing.length) {
            showLoadError(root, t, missing);
            return;
        }

        const settings = AD.loadSettings();
        const bests = AD.loadBests(readonly);

        /* Renderer: three.js when the vendor bundle and render3d are present and WebGL2
           works, else the Canvas 2D fallback (created only then). */
        let renderer = null, glCanvas = null, post = null, renderer3d = null;
        if (window.AuraDriftThree && typeof AD.createRenderer3D === 'function') {
            glCanvas = document.createElement('canvas');
            glCanvas.className = 'agdrift-gl';
            root.insertBefore(glCanvas, world);
            try { renderer = AD.createRenderer3D(glCanvas, settings, t); } catch (_) { renderer = null; }
            if (!renderer) { glCanvas.remove(); glCanvas = null; }
        }
        function attachSurfaces() {
            const three = !!renderer && renderer.kind === 'three';
            world.hidden = three;
            if (glCanvas) glCanvas.hidden = !three;
            post = three && renderer.post ? renderer.post : createNoopPost();
        }
        if (!renderer) renderer = AD.createRenderer(world, settings);
        attachSurfaces();

        const hud = AD.createHud(hudCanvas, labels, t);
        const audio = AD.createAudio(settings);
        const camera = AD.createChaseCamera({ mode: settings.camera });
        const scheduler = AD.createScheduler();
        const timeWarp = AD.createTimeWarp({ reduced: false });
        const governor = AD.createQualityGovernor(settings.quality === 'auto' ? 'high' : settings.quality);
        if (settings.quality !== 'auto') governor.lock(settings.quality);
        const pressed = new Set();
        let race = null, banter = null, cupIndex = 0, currentConfig = null, ui = null;
        let cupScores = [0, 0, 0, 0, 0, 0];
        let view = 'menu';             /* menu -> loading -> play -> results (the results panel overlays 'play') */
        let paused = false, raf = 0, last = 0, frozen = false, startToken = 0, assetsHeld = false;
        let lastPose = null, aspect = 16 / 9, pausedGlAt = 0;
        let pendingResults = null, resultsShown = false, appliedQuality = settings.quality;
        let tier = governor.tier;
        let reducedMotion = settings.reducedMotion ||
            (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
        timeWarp.reduced = !!reducedMotion;

        function applyQuality(next) {
            if (next) tier = next;
            const chosen = settings.quality === 'auto' ? tier : settings.quality;
            tier = chosen;
            renderer.setQuality(chosen);
            post.setQuality(chosen);
            return AD.QUALITY[chosen] || AD.QUALITY.high;
        }
        let quality = applyQuality();
        /* The garage the menu shows; kept so a renderer swap can restore it. */
        let showSpec = { carId: settings.car, livery: settings.livery, biomeId: 'alpine' };
        function showcaseNow(spec) {
            if (spec) showSpec = Object.assign({}, showSpec, spec);
            renderer.setShowcase(showSpec);
        }

        function resize() {
            const rect = root.getBoundingClientRect();
            const w = Math.max(320, rect.width || 960);
            const h = Math.max(240, rect.height || 540);
            const dpr = Math.min(2, window.devicePixelRatio || 1);
            aspect = w / h;
            renderer.resize(w, h, dpr);
            post.resize(w, h, dpr);
            hud.resize(w, h, dpr);
            /* Touch pads are two 56 px rows plus padding: lift the gauges clear of them. */
            if (typeof hud.setInset === 'function') hud.setInset(window.matchMedia && window.matchMedia('(pointer: coarse)').matches ? 140 : 0);
        }
        const observer = window.ResizeObserver ? new ResizeObserver(resize) : null;
        if (observer) observer.observe(root);
        window.addEventListener('resize', resize);
        resize();

        /* A throwing three.js frame drops 3D for good; a lost WebGL context only parks it
           (keep) until the browser restores the context. Either way the race continues in Canvas 2D. */
        function fallbackTo2D(keep) {
            if (renderer.kind !== 'three') return;
            const old = renderer;
            if (keep) {
                renderer3d = old;
            } else {
                try { old.dispose(); } catch (_) { /* already gone */ }
                if (glCanvas) { glCanvas.remove(); glCanvas = null; }
            }
            renderer = AD.createRenderer(world, settings);
            attachSurfaces();
            quality = applyQuality();
            resize();
            if (race) renderer.setTrack(race.track, race);
            else if (view === 'menu') showcaseNow();
            if (ui) ui.setFallback(true);
        }
        function restore3D() {
            if (!renderer3d || renderer.kind === 'three') return;
            const flat = renderer;
            renderer = renderer3d;
            renderer3d = null;
            try { flat.dispose(); } catch (_) { /* nothing to free */ }
            attachSurfaces();
            quality = applyQuality();
            resize();
            if (race) renderer.setTrack(race.track, race);
            else if (view === 'menu') showcaseNow();
            if (ui) ui.setFallback(false);
        }
        if (renderer && renderer.kind === 'three') {
            renderer.onContextLost = () => fallbackTo2D(true);
            renderer.onContextRestored = restore3D;
        }

        ui = AD.createUI(root, labels, {
            readonly: readonly,
            onStart(config) {
                audio.unlock();
                cupIndex = 0;
                cupScores = [0, 0, 0, 0, 0, 0];
                settings.car = config.carId || settings.car;
                if (config.livery !== undefined) settings.livery = config.livery;
                AD.saveSettings(settings, readonly);
                startRace(config, { intro: true });
            },
            onRestart() { startRace(currentConfig || { mode: 'quick', trackId: AD.TRACKS[0].id, laps: 3 }, { intro: true }); },
            onResume() { setPaused(false); },
            onQuit() { showMenu(); },
            onNext() {
                cupIndex++;
                startRace(currentConfig, { intro: true });
            },
            onUi(kind) { audio.ui(kind); },
            /* Garage selection: the live 3D showcase follows car, livery and the track's biome. */
            onShowcase(spec) {
                settings.car = spec.carId;
                settings.livery = spec.livery;
                AD.saveSettings(settings, readonly);
                showcaseNow(spec);
            },
            onCamera() {
                settings.camera = camera.cycle();
                AD.saveSettings(settings, readonly);
            },
            onResetBests() {
                for (const key of Object.keys(bests)) delete bests[key];
                AD.saveBests(bests, readonly);
                ui.setBests(bests);
            },
            onSettings(next) {
                AD.saveSettings(next, readonly);
                audio.setSettings(next);
                reducedMotion = next.reducedMotion ||
                    (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
                timeWarp.reduced = !!reducedMotion;
                if (next.camera && next.camera !== camera.mode) camera.setMode(next.camera);
                if (next.quality !== appliedQuality) {
                    appliedQuality = next.quality;
                    if (next.quality === 'auto') governor.reset(); else governor.lock(next.quality);
                    quality = applyQuality(governor.tier);
                }
            }
        }, settings, t);
        ui.setFallback(renderer.kind !== 'three');

        function buildTrack(config) {
            if (config.mode === 'daily') return AD.buildTrack(AD.dailyTrack(dailySeed()), dailySeed());
            if (config.mode === 'cup') return AD.buildTrack(AD.TRACKS[cupIndex % AD.TRACKS.length]);
            return AD.buildTrack(AD.trackById(config.trackId || AD.TRACKS[0].id));
        }

        /* The mode table decides the field (section 2.8); a config value may only
           change it where rivals are part of the mode. */
        function fieldSize(mode, config) {
            const rules = AD.modeRules(mode);
            let count = rules.opponents;
            if (count > 0 && config && Number.isFinite(config.opponents)) count = Math.floor(config.opponents);
            if (count <= 0) return 0;
            const min = (mode === 'elimination' || mode === 'hot_potato') ? 1 : 0;
            return Math.max(min, Math.min(5, count));
        }

        function beginRace(track, config, opts) {
            const best = bests[track.id];
            const mode = config.mode || 'quick';
            const playerCar = config.carId || settings.car;
            if (banter && typeof banter.dispose === 'function') banter.dispose();
            race = AD.createRace({
                track: track,
                mode: mode,
                seed: config.seed,
                /* Records are per track, so only quick races may change the lap count. */
                laps: mode === 'quick' ? (config.laps || track.laps) : track.laps,
                opponents: fieldSize(mode, config),
                weapons: settings.weapons && mode !== 'time_trial',
                /* Only a time-trial run is a replayable ghost: it has no weapons and no rescue. */
                ghostTape: mode === 'time_trial' && best && best.tape ? best.tape : null,
                ghostCar: best && best.carId,
                showGhost: settings.showGhost,
                readonly: readonly,
                playerCar: playerCar,
                livery: config.livery !== undefined ? config.livery : settings.livery
            });
            banter = typeof AD.createBanter === 'function'
                ? AD.createBanter({ seed: race.seed, t: t, race: race }) : null;
            renderer.setTrack(track, race);
            audio.setBiome(track.biomeId);
            if (typeof audio.setCar === 'function') audio.setCar(playerCar);
            if (typeof audio.setView === 'function') audio.setView('race');
            post.setBiome(track.biomeId);
            scheduler.reset();
            camera.reset();
            if (opts.intro && !reducedMotion) camera.script('intro');
            view = 'play';
            paused = false;
            pendingResults = null;
            resultsShown = false;
            ui.hide();
            setPaused(false);
            audio.resume();
        }

        /* Resolves once the race exists. Optional asset loading shows the loading view first. */
        function startRace(config, opts) {
            opts = opts || { intro: true };
            currentConfig = Object.assign({}, config);
            const token = ++startToken;
            const track = buildTrack(config);
            const preload = [];
            const assets = AD.assets;
            if (assets && typeof assets.loadCore === 'function') {
                try {
                    if (!assetsHeld && typeof assets.acquire === 'function') { assets.acquire(); assetsHeld = true; }
                    preload.push(assets.loadCore());
                    if (typeof assets.loadBiome === 'function') preload.push(assets.loadBiome(track.biomeId));
                } catch (_) { /* stand-ins and synth cover missing assets */ }
            }
            if (!preload.length) {
                beginRace(track, config, opts);
                return Promise.resolve(true);
            }
            view = 'loading';
            race = null;
            paused = false;
            pendingResults = null;
            resultsShown = false;
            if (typeof ui.showLoading === 'function') ui.showLoading(); else ui.hide();
            showcaseNow({
                carId: config.carId || settings.car,
                livery: config.livery !== undefined ? config.livery : settings.livery,
                biomeId: track.biomeId
            });
            return Promise.all(preload.map(p => Promise.resolve(p).catch(() => null))).then(() => {
                if (token === startToken && instances.get(id) === entry) beginRace(track, config, opts);
                return true;
            });
        }

        function showMenu() {
            startToken++;
            race = null;
            view = 'menu';
            paused = false;
            pendingResults = null;
            resultsShown = false;
            showcaseNow({ carId: settings.car, livery: settings.livery });
            audio.setBiome('alpine');
            if (typeof audio.setView === 'function') audio.setView('menu');
            ui.showMenu(bests);
        }

        function setPaused(value) {
            paused = value;
            if (value) {
                pressed.clear();
                audio.suspend();
                ui.showPause();
            } else {
                ui.hide();
                audio.resume();
                last = performance.now();
            }
        }

        const playing = () => view === 'play' && race && race.cars[0].isPlayer;

        function readInput() {
            const input = race.playerInput;
            const touch = ui.touch;
            const key = name => pressed.has(name);
            input.throttle = (key('ArrowUp') || key('KeyW') || touch.throttle) ? 1 : 0;
            input.brake = (key('ArrowDown') || key('KeyS') || touch.brake) ? 1 : 0;
            /* Raw arcade steering: right is +1 and the solver turns the nose to the
               right on the screen, forward and in reverse. Camera and 2D view follow
               the car heading, so no input-side flip exists anywhere. */
            input.steer = (((key('ArrowRight') || key('KeyD') || touch.right) ? 1 : 0) -
                ((key('ArrowLeft') || key('KeyA') || touch.left) ? 1 : 0));
            input.handbrake = (key('ShiftLeft') || key('ShiftRight') || touch.handbrake) ? 1 : 0;
            input.fire = (key('Space') || touch.fire) ? 1 : 0;
            /* Ctrl+W closes a browser tab, so nitro is also on E (D6). */
            input.boost = (key('KeyE') || key('ControlLeft') || key('ControlRight') || touch.boost) ? 1 : 0;
        }

        function drainEvents() {
            for (const evt of race.events) {
                renderer.event(evt, race);
                hud.event(evt, race);
                audio.event(evt, race);
                if (banter) banter.onEvent(evt, race);
                camera.onEvent(evt);
                timeWarp.onEvent(evt);
            }
            race.events.length = 0;
        }

        function finishRace() {
            if (!race || !race.result || race.result.saved) return;
            const result = race.result;
            result.saved = true;
            const track = race.track, carId = race.cars[0].v.car.id;
            /* Bests: a time-trial lap record (with a complete tape, D5) or a drift-attack score.
               A quick-race tape is never stored: weapons and rescue would desync the ghost. */
            let record = false;
            if (!readonly && race.mode === 'time_trial' && result.tapeComplete === true && !result.dnf) {
                const previous = bests[track.id];
                if (!previous || result.time < previous.time) {
                    const tape = Array.isArray(result.tape) ? AD.encodeTape(result.tape) : result.tape;
                    bests[track.id] = { time: result.time, tape: tape, carId: carId };
                    record = true;
                }
            } else if (!readonly && race.mode === 'drift_attack' && result.score > 0) {
                const previous = bests['drift:' + track.id];
                if (!previous || result.score > previous.score) {
                    bests['drift:' + track.id] = { score: Math.round(result.score), carId: carId };
                    record = true;
                }
            }
            if (record) AD.saveBests(bests, readonly);
            if (typeof AD.pickResultTitle === 'function') result.title = AD.pickResultTitle(result, race.cars[0], race);
            /* Cup points accumulate across the cup (six slots, AD.cupPoints per place). */
            let cupTotal;
            if (race.mode === 'cup' && race.standings) {
                race.standings.forEach((car, place) => {
                    cupScores[car.index] = (cupScores[car.index] || 0) + AD.cupPoints(place + 1);
                });
                cupTotal = cupScores[0];
            }
            /* The panel waits a beat so the finish call-out and the orbit camera get their moment. */
            pendingResults = {
                wait: reducedMotion ? 0.4 : 1.8,
                result: result,
                info: {
                    field: race.cars.length,
                    track: AD.trackName(track, t),
                    record: record,
                    cupTotal: cupTotal,
                    hasNext: race.mode === 'cup' && cupIndex < AD.TRACKS.length - 1
                }
            };
        }

        function clearHud() {
            hudCanvas.getContext('2d').clearRect(0, 0, hudCanvas.width, hudCanvas.height);
        }

        /* Debug autopilot: the player slot drives itself with the opponent controller (AD.aiInput), fires when a
           weapon is held and lines up the next ramp. Used by the playthrough harness only. */
        let autopilot = null;
        function driveAuto() {
            if (!autopilot || !race || !playing() || race.state === 'finished') return;
            const car = race.cars[0], v = car.v;
            if (v.out) return;
            car.ai.skill = car.ai.aggression = autopilot.skill;
            let lane = 0;
            if (autopilot.ramps && race.features && race.features.ramps && !v.airborne) {
                const here = AD.sampleTrack(race.track, v.x, v.y), L = race.track.length;
                for (const r of race.features.ramps) {
                    const ds = ((r.s - here.s) % L + L) % L;
                    if (ds > 8 && ds < 90) { lane = r.offset || 0; break; }
                }
            }
            car.ai.lane = lane;
            const input = AD.aiInput(race, car), target = race.playerInput;
            for (const key of Object.keys(input)) target[key] = input[key];
        }
        function stepOnce(step) { if (autopilot) driveAuto(); AD.stepRace(race, step); }

        /* One presentation tick for the events and effects a batch of steps produced (the RAF loop minus the wall clock). */
        function presentTick(dt) {
            renderer.emit(race, dt);
            audio.update(race, true, lastPose);
            if (banter) {
                const msg = banter.update(dt);
                if (msg && typeof hud.radio === 'function') hud.radio(msg);
            }
            if (pendingResults) {
                pendingResults.wait -= dt;
                if (pendingResults.wait <= 0) {
                    if (typeof audio.setView === 'function') audio.setView('results');
                    ui.showResults(pendingResults.result, pendingResults.info);
                    pendingResults = null;
                    resultsShown = true;
                }
            }
            lastPose = camera.update(race, dt, { reduced: reducedMotion, paused: false, lowZoom: !!quality.lowZoom });
            renderer.render(race, lastPose, dt, reducedMotion);
            if (playing() && !resultsShown) hud.render(race, dt, settings, lastPose); else clearHud();
        }

        function loop(now) {
            raf = requestAnimationFrame(loop);
            const dt = Math.min(0.05, Math.max(0, (now - last) / 1000));
            last = now;
            if (!isActive() || document.hidden) return;
            try {
                frame(now, dt);
            } catch (error) {
                if (renderer.kind === 'three') {
                    if (window.console && console.error) console.error('Aura Drift: 3D frame failed, continuing in 2D', error);
                    fallbackTo2D();
                } else throw error;
            }
        }

        function frame(now, dt) {
            if (view === 'menu' || view === 'loading' || !race) {
                renderer.showcase(dt, reducedMotion);
                if (view === 'loading' && typeof ui.progress === 'function' && AD.assets && typeof AD.assets.progress === 'function') {
                    ui.progress(AD.assets.progress());
                }
                clearHud();
                return;
            }
            const simDt = paused ? 0 : timeWarp.scale(dt);
            if (!paused && !frozen) {
                if (!autopilot && playing() && (race.state === 'running' || race.state === 'countdown')) readInput();
                scheduler.advance(simDt, stepOnce);
                drainEvents();
                renderer.emit(race, simDt);
                audio.update(race, true, lastPose);
                if (banter) {
                    const msg = banter.update(dt);
                    if (msg && typeof hud.radio === 'function') hud.radio(msg);
                }
                if (race.state === 'finished' && race.cars[0].isPlayer) finishRace();
            }
            lastPose = camera.update(race, dt, { reduced: reducedMotion, paused: paused || frozen, lowZoom: !!quality.lowZoom });
            /* A paused GL scene needs only 10 frames per second. */
            if (!(paused && renderer.kind === 'three' && now - pausedGlAt < 100)) {
                if (paused) pausedGlAt = now;
                renderer.render(race, lastPose, paused ? 0 : simDt, reducedMotion);
            }
            if (pendingResults) {
                pendingResults.wait -= dt;
                if (pendingResults.wait <= 0) {
                    if (typeof audio.setView === 'function') audio.setView('results');
                    ui.showResults(pendingResults.result, pendingResults.info);
                    pendingResults = null;
                    resultsShown = true;
                }
            }
            if (playing() && !paused && !resultsShown) hud.render(race, dt, settings, lastPose);
            else clearHud();
            if (!paused && !frozen) {
                const next = governor.sample(dt);
                if (next) quality = applyQuality(next);
            }
        }

        function snapshot() {
            if (!race || !lastPose) return null;
            const v = race.cars[0].v;
            const pose = JSON.parse(JSON.stringify(lastPose));
            return {
                pose: pose, aspect: aspect, kind: renderer.kind,
                car: [v.x, v.z + 0.5, v.y],
                aim: [v.x + Math.cos(v.psi) * 30, v.z + 0.5, v.y + Math.sin(v.psi) * 30],
                psi: v.psi,
                /* Body state for visual checks: heading, steer, air pose. */
                body: { z: v.z, roll: v.roll, pitch: v.pitch, steer: v.steer, vx: v.vx, r: v.r, airborne: !!v.airborne }
            };
        }

        const debug = {
            start(config) {
                cupIndex = 0;
                cupScores = [0, 0, 0, 0, 0, 0];
                const cfg = Object.assign({ mode: 'quick', trackId: AD.TRACKS[0].id }, config || {});
                return startRace(cfg, { intro: false });
            },
            snapshot,
            /* Scene access for visual forensics (survey scripts): the live three.js scene and camera, or null in 2D. */
            race() { return race; },
            three() { return renderer && renderer.scene ? { scene: renderer.scene, camera: renderer.camera, T: window.AuraDriftThree } : null; },
            breakdown() { return renderer.breakdown ? renderer.breakdown() : null; },
            setCamera(mode) { camera.setMode(mode); },
            warp(fraction) {
                if (!race) return;
                const track = race.track;
                const s = ((fraction % 1) + 1) % 1 * track.length;
                const p = AD.pointAt(track, s);
                const car = race.cars[0], v = car.v;
                v.x = p.x; v.y = p.y; v.psi = Math.atan2(p.ty, p.tx);
                v.vy = 0; v.r = 0; v.z = 0; v.vz = 0; v.airborne = false;
                car.lastS = s; car.progress = car.lap * track.length + s;
                camera.snap();
            },
            give(weaponId) {
                if (!race) return;
                AD.giveWeapon(race, race.cars[0], weaponId);
                race.cars[0].weaponLock = 0;
            },
            fire() { if (race) AD.fireWeapon(race, race.cars[0]); },
            launch(vz) { if (race) AD.launch(race, race.cars[0], vz || 9, 'debug'); },
            /* The player slot drives itself: opts { skill 0.3..1.2, ramps }. */
            autopilot(on, opts) {
                autopilot = on ? Object.assign({ skill: 0.9, ramps: true }, opts || {}) : null;
                if (!on) for (const key of Object.keys(race ? race.playerInput : {})) if (typeof race.playerInput[key] === 'number') race.playerInput[key] = 0;
            },
            /* Steps the sim without waiting for animation frames: events go to renderer, hud, audio, banter, camera and
               time warp as in the loop, and every opts.renderEvery-th step (30) is rendered. Returns a status line. */
            fastForward(seconds, opts) {
                if (!race) return null;
                const every = Math.max(1, (opts && opts.renderEvery) | 0) || 30;
                let left = Math.round(seconds / AD.STEP), ticks = 0, rendered = 0;
                while (left-- > 0 && race && !paused) {
                    if (!autopilot && playing() && (race.state === 'running' || race.state === 'countdown')) readInput();
                    stepOnce(AD.STEP);
                    drainEvents();
                    if (race.state === 'finished' && race.cars[0].isPlayer) finishRace();
                    if (++ticks % every === 0) { presentTick(Math.min(0.05, AD.STEP * every)); rendered++; }
                }
                return { time: race ? race.time : 0, state: race ? race.state : view, rendered: rendered, results: resultsShown };
            },
            pause(value) { if (race && view === 'play') setPaused(value === undefined ? !paused : !!value); },
            quit() { showMenu(); },
            /* Shell state for harnesses. */
            view() { return { view: view, ghost: !!(race && race.ghost), track: race ? race.track.id : '', paused: paused, results: resultsShown, pending: !!pendingResults, race: race ? race.state : '', cup: cupIndex, cupScores: cupScores.slice() }; },
            freeze(value) { frozen = !!value; },
            /* Garage showcase without the menu panel: { carId, livery, biomeId }. */
            showcase(config) {
                startToken++;
                race = null; view = 'menu'; paused = false;
                pendingResults = null; resultsShown = false;
                showcaseNow(Object.assign({ carId: settings.car, livery: settings.livery, biomeId: 'alpine' }, config || {}));
                ui.hide();
            },
            /* The results panel over the garage with a demo result; `spec` overrides any field. */
            results(spec) {
                startToken++;
                race = null; view = 'menu'; paused = false;
                pendingResults = null; resultsShown = false;
                const track = AD.trackById((spec && spec.trackId) || AD.TRACKS[0].id);
                const result = Object.assign({
                    mode: 'quick', time: 94.372, bestLap: 46.118, position: 2, field: 6, carId: settings.car, points: 18,
                    score: 18450, medal: 'silver', title: 'air_force', eliminatedAt: null, dnf: false, won: false,
                    style: { banked: 18450, pending: 0, maxMultiplier: 4.5, drifts: 12 },
                    tricks: { counts: { barrel_roll: 3, flat_spin: 2, cow_vault: 1, drift_pink: 4, slingshot: 2 }, best: 1800 },
                    tally: { airTime: 23.4, animalHits: 5, smashed: 7 }
                }, spec || {});
                showcaseNow({ carId: result.carId, livery: settings.livery, biomeId: track.biome });
                ui.showResults(result, {
                    field: result.field, track: AD.trackName(track, t), record: result.mode === 'time_trial',
                    hasNext: result.mode === 'cup', cupTotal: result.mode === 'cup' ? 61 : undefined
                });
            }
        };

        const entry = {
            host, observer, debug,
            teardown() {
                if (raf) cancelAnimationFrame(raf);
                raf = 0;
                startToken++;
                window.removeEventListener('resize', resize);
                renderer.dispose();
                if (renderer3d) renderer3d.dispose();
                renderer3d = null;
                post.dispose();
                hud.dispose();
                audio.dispose();
                ui.destroy();
                if (banter && typeof banter.dispose === 'function') banter.dispose();
                if (assetsHeld && AD.assets && typeof AD.assets.release === 'function') AD.assets.release();
                assetsHeld = false;
            },
            stats() {
                const rs = renderer.stats();
                const car = race ? race.cars[0] : null;
                const audioStats = typeof audio.stats === 'function' ? audio.stats() : null;
                return {
                    state: view === 'play' && race ? race.state : view,
                    time: race ? race.time : 0,
                    speedKmh: car ? AD.vehicleSpeedKmh(car.v) : 0,
                    paused: paused,
                    mode: race ? race.mode : '',
                    trackId: race ? race.track.id : '',
                    carId: car ? car.v.car.id : settings.car,
                    quality: tier,
                    camera: camera.mode,
                    field: race ? race.cars.length : 0,
                    renderer: Object.assign({
                        kind: renderer.kind, calls: 0, triangles: 0, geometries: 0, textures: 0, particles: 0,
                        props: 0, standIns: 0, wetness: 0, chunks: 0, projection: null
                    }, rs),
                    post: post.stats(),
                    audio: Object.assign({ samples: 0, failed: 0, fallback: true }, audioStats || {})
                };
            },
            onKeyDown(event) {
                if (!isActive() || view !== 'play' || !race || !race.cars[0].isPlayer) return;
                /* Menus own the keyboard: only the race itself may swallow arrows and Space. */
                if (event.code === 'KeyP' || event.code === 'Escape') {
                    if (race.state !== 'finished') setPaused(!paused);
                    return;
                }
                if (event.code === 'KeyR' && currentConfig && !event.repeat) { startRace(currentConfig, { intro: true }); return; }
                if (race.state === 'finished' || paused) return;
                if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Space'].includes(event.code)) event.preventDefault();
                if (event.repeat && (event.code === 'KeyC' || event.code === 'KeyH')) return;
                if (event.code === 'KeyC') {
                    settings.camera = camera.cycle();
                    AD.saveSettings(settings, readonly);
                    return;
                }
                if (event.code === 'KeyT') {
                    /* Marshal rescue; consumed (and refused in time trial) by AD.stepRace. */
                    if (!event.repeat && playing()) race.rescueRequest = true;
                    return;
                }
                if (event.code === 'KeyH') {
                    if (typeof audio.horn === 'function') audio.horn();
                    return;
                }
                audio.unlock();
                pressed.add(event.code);
            },
            onKeyUp(event) { pressed.delete(event.code); },
            onBlur() {
                pressed.clear();
                if (playing() && !paused && race.state !== 'finished') setPaused(true);
            },
            onVisibility() {
                if (document.hidden) {
                    pressed.clear();
                    if (playing() && !paused && race.state !== 'finished') setPaused(true);
                } else last = performance.now();
            }
        };
        instances.set(id, entry);
        window.addEventListener('keydown', entry.onKeyDown, true);
        window.addEventListener('keyup', entry.onKeyUp, true);
        window.addEventListener('blur', entry.onBlur);
        document.addEventListener('visibilitychange', entry.onVisibility);
        showMenu();
        last = performance.now();
        raf = requestAnimationFrame(loop);
    }

    /* The debug surface acts on the most recently rendered window. */
    function currentDebug() {
        let entry = null;
        for (const item of instances.values()) entry = item;
        return entry ? entry.debug : null;
    }

    window.AuraDriftApp = {
        render, dispose,
        /* Read-only diagnostic surface for smoke tests and the browser console. */
        stats() {
            const result = [];
            for (const entry of instances.values()) result.push(entry.stats());
            return result;
        },
        debug: {
            start(config) { const d = currentDebug(); return d ? d.start(config) : Promise.resolve(false); },
            snapshot() { const d = currentDebug(); return d ? d.snapshot() : null; },
            breakdown() { const d = currentDebug(); return d ? d.breakdown() : null; },
            three() { const d = currentDebug(); return d ? d.three() : null; },
            race() { const d = currentDebug(); return d ? d.race() : null; },
            setCamera(mode) { const d = currentDebug(); if (d) d.setCamera(mode); },
            warp(fraction) { const d = currentDebug(); if (d) d.warp(fraction); },
            give(id) { const d = currentDebug(); if (d) d.give(id); },
            fire() { const d = currentDebug(); if (d) d.fire(); },
            launch(vz) { const d = currentDebug(); if (d) d.launch(vz); },
            autopilot(on, opts) { const d = currentDebug(); if (d) d.autopilot(on, opts); },
            fastForward(seconds, opts) { const d = currentDebug(); return d ? d.fastForward(seconds, opts) : null; },
            pause(value) { const d = currentDebug(); if (d) d.pause(value); },
            quit() { const d = currentDebug(); if (d) d.quit(); },
            view() { const d = currentDebug(); return d ? d.view() : null; },
            freeze(value) { const d = currentDebug(); if (d) d.freeze(value); },
            showcase(config) { const d = currentDebug(); if (d) d.showcase(config); },
            results(spec) { const d = currentDebug(); if (d) d.results(spec); }
        }
    };
})();
