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
            setQuality() {}, setBiome() {}, resize() {},
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

        /* DOM order: gl, world (2D fallback), old post, hud, then touch and overlay from the UI. */
        const root = document.createElement('div');
        root.className = 'agdrift-root';
        const world = document.createElement('canvas');
        world.className = 'agdrift-world';
        const postCanvas = document.createElement('canvas');
        postCanvas.className = 'agdrift-post';
        const hudCanvas = document.createElement('canvas');
        hudCanvas.className = 'agdrift-hud';
        root.appendChild(world);
        root.appendChild(postCanvas);
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
            postCanvas.hidden = three;
            if (glCanvas) glCanvas.hidden = !three;
            post = three ? renderer.post : (typeof AD.createPostProcessor === 'function'
                ? AD.createPostProcessor(world, postCanvas) : createNoopPost());
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
        let race = null, banter = null, cupIndex = 0, currentConfig = null;
        let cupScores = [0, 0, 0, 0];
        let view = 'menu';             /* menu | loading | play */
        let paused = false, raf = 0, last = 0, frozen = false, startToken = 0, assetsHeld = false;
        let lastPose = null, aspect = 16 / 9, pausedGlAt = 0;
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

        function resize() {
            const rect = root.getBoundingClientRect();
            const w = Math.max(320, rect.width || 960);
            const h = Math.max(240, rect.height || 540);
            const dpr = Math.min(2, window.devicePixelRatio || 1);
            aspect = w / h;
            renderer.resize(w, h, dpr);
            post.resize(w, h, dpr);
            hud.resize(w, h, dpr);
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
        }
        if (renderer && renderer.kind === 'three') {
            renderer.onContextLost = () => fallbackTo2D(true);
            renderer.onContextRestored = restore3D;
        }

        const ui = AD.createUI(root, labels, {
            onStart(config) {
                audio.unlock();
                cupIndex = 0;
                cupScores = [0, 0, 0, 0];
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
            onSettings(next) {
                AD.saveSettings(next, readonly);
                audio.setSettings(next);
                reducedMotion = next.reducedMotion ||
                    (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
                timeWarp.reduced = !!reducedMotion;
                if (next.quality === 'auto') governor.reset(); else governor.lock(next.quality);
                quality = applyQuality(governor.tier);
            }
        }, settings, t);

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
                laps: config.laps || track.laps,
                opponents: fieldSize(mode, config),
                weapons: settings.weapons && mode !== 'time_trial',
                ghostTape: mode === 'time_trial' && best ? best.tape : null,
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
            post.setBiome(track.biomeId);
            scheduler.reset();
            camera.reset();
            if (opts.intro && !reducedMotion) camera.script('intro');
            view = 'play';
            paused = false;
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
            if (typeof ui.showLoading === 'function') ui.showLoading(); else ui.hide();
            renderer.setShowcase({ carId: config.carId || settings.car, livery: settings.livery, biomeId: track.biomeId });
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
            renderer.setShowcase({ carId: settings.car, livery: settings.livery, biomeId: 'alpine' });
            audio.setBiome('alpine');
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

        /* HUD events for the previous HUD module, which has no event() of its own. */
        function legacyHud(evt) {
            if (evt.type === 'pickup' && evt.car === 0 && AD.WEAPONS[evt.id]) {
                hud.showMessage(AD.WEAPONS[evt.id].name, 1.1);
            } else if (evt.type === 'lap' && !reducedMotion) {
                hud.showMessage(labels.lap + ' ' + evt.lap + ' · ' + AD.formatTime(evt.time), 1.8);
            } else if (evt.type === 'go') {
                hud.showMessage(labels.ready, 0.9);
            } else if (evt.type === 'style_bank' && evt.car === 0) {
                hud.popup('+' + Math.round(evt.points), evt.multiplier > 2 ? '#ffd166' : '#7dd3fc');
            } else if (evt.type === 'near_miss' && evt.car === 0) {
                hud.popup(labels.combo + '!', '#ff9d4a');
            } else if (evt.type === 'sector') {
                hud.showMessage(labels.sector + ' ' + (evt.index + 1) + ' · ' + AD.formatTime(evt.time), 1.4);
            }
        }

        function drainEvents() {
            for (const evt of race.events) {
                renderer.event(evt, race);
                if (typeof hud.event === 'function') hud.event(evt, race); else legacyHud(evt);
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
            const key = race.track.id;
            const previous = bests[key];
            const record = !readonly && result.tapeComplete !== false && (!previous || result.time < previous.time);
            if (record) {
                const tape = Array.isArray(result.tape) ? AD.encodeTape(result.tape) : result.tape;
                bests[key] = { time: result.time, tape: tape, carId: race.cars[0].v.car.id };
                AD.saveBests(bests, readonly);
            }
            /* Cup points accumulate across the cup. */
            let cupLine = '';
            if (race.mode === 'cup' && race.standings) {
                const points = [25, 18, 15, 12];
                for (let i = 0; i < race.standings.length; i++) {
                    const slot = race.standings[i].index || 0;
                    const pts = typeof AD.cupPoints === 'function' ? AD.cupPoints(i + 1) : (points[i] || 12);
                    cupScores[slot] = (cupScores[slot] || 0) + pts;
                }
                cupLine = '  ' + labels.cup + ' ' + (cupScores[0] || 0);
            }
            ui.showResults(result, {
                field: race.cars.length,
                track: race.track.name + cupLine,
                record: record,
                hasNext: race.mode === 'cup' && cupIndex < AD.TRACKS.length - 1
            });
        }

        function clearHud() {
            hudCanvas.getContext('2d').clearRect(0, 0, hudCanvas.width, hudCanvas.height);
        }

        function loop(now) {
            raf = requestAnimationFrame(loop);
            const dt = Math.min(0.05, Math.max(0, (now - last) / 1000));
            last = now;
            if (!isActive() || document.hidden) return;
            try {
                frame(now, dt);
            } catch (error) {
                if (renderer.kind === 'three') fallbackTo2D(); else throw error;
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
                if (playing() && (race.state === 'running' || race.state === 'countdown')) readInput();
                scheduler.advance(simDt, step => AD.stepRace(race, step));
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
                if (renderer.kind !== 'three') post.render(race, dt, reducedMotion);
            }
            if (playing() && !paused) hud.render(race, dt, settings, lastPose);
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
                cupScores = [0, 0, 0, 0];
                const cfg = Object.assign({ mode: 'quick', trackId: AD.TRACKS[0].id }, config || {});
                return startRace(cfg, { intro: false });
            },
            snapshot,
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
            freeze(value) { frozen = !!value; },
            /* Garage showcase without the menu panel: { carId, livery, biomeId }. */
            showcase(config) {
                startToken++;
                race = null; view = 'menu'; paused = false;
                renderer.setShowcase(Object.assign({ carId: settings.car, livery: settings.livery, biomeId: 'alpine' }, config || {}));
                ui.hide();
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
                if (!isActive() || !race || !race.cars[0].isPlayer) return;
                if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Space'].includes(event.code)) event.preventDefault();
                if (event.code === 'KeyP' || event.code === 'Escape') {
                    if (race.state !== 'finished') setPaused(!paused);
                    return;
                }
                if (event.code === 'KeyR' && currentConfig) { startRace(currentConfig, { intro: true }); return; }
                if (paused || event.repeat && (event.code === 'KeyC' || event.code === 'KeyH')) return;
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
            setCamera(mode) { const d = currentDebug(); if (d) d.setCamera(mode); },
            warp(fraction) { const d = currentDebug(); if (d) d.warp(fraction); },
            give(id) { const d = currentDebug(); if (d) d.give(id); },
            fire() { const d = currentDebug(); if (d) d.fire(); },
            launch(vz) { const d = currentDebug(); if (d) d.launch(vz); },
            freeze(value) { const d = currentDebug(); if (d) d.freeze(value); },
            showcase(config) { const d = currentDebug(); if (d) d.showcase(config); }
        }
    };
})();
