/* Aura Drift - Virtual Desktop app shell, input and lifecycle. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const instances = new Map();
    const QUALITY_ORDER = ['low', 'medium', 'high', 'ultra'];

    /* Every factory the shell needs. A stale service-worker cache can serve an
       older module-loader.js that predates one of these files; fail visibly
       instead of leaving an empty dark window. createPostProcessor is
       deliberately absent because the WebGL2 pass is optional. */
    const REQUIRED = [
        ['aura-drift-core.js', 'loadSettings'],
        ['aura-drift-core.js', 'createScheduler'],
        ['aura-drift-physics.js', 'createVehicle'],
        ['aura-drift-track.js', 'buildTrack'],
        ['aura-drift-race.js', 'createRace'],
        ['aura-drift-render.js', 'createRenderer'],
        ['aura-drift-render.js', 'createCamera'],
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
        entry.stop();
        entry.renderer.dispose();
        entry.post.dispose();
        entry.hud.dispose();
        entry.audio.dispose();
        entry.ui.destroy();
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
            catchup: 'aura_drift.catchup', nitro: 'aura_drift.nitro'
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
        const bests = AD.loadBests();
        const renderer = AD.createRenderer(world, settings);
        const post = typeof AD.createPostProcessor === 'function'
            ? AD.createPostProcessor(world, postCanvas)
            : createNoopPost();
        const hud = AD.createHud(hudCanvas, labels);
        const audio = AD.createAudio(settings);
        const camera = AD.createCamera();
        const scheduler = AD.createScheduler();
        const pressed = new Set();
        let race = null, cupIndex = 0, currentConfig = null;
        let cupScores = [0, 0, 0, 0];
        let paused = false, raf = 0, last = 0;
        let qualityLevel = QUALITY_ORDER.includes(settings.quality) ? settings.quality : 'high';
        let frameAccum = 0, frameCount = 0;
        let reducedMotion = settings.reducedMotion ||
            (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);

        function applyQuality() {
            const chosen = settings.quality === 'auto' ? qualityLevel : settings.quality;
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
            renderer.resize(w, h, dpr);
            post.resize(w, h, dpr);
            hud.resize(w, h, dpr);
        }
        const observer = window.ResizeObserver ? new ResizeObserver(resize) : null;
        if (observer) observer.observe(root);
        window.addEventListener('resize', resize);
        resize();

        const ui = AD.createUI(root, labels, {
            onStart(config) {
                audio.unlock();
                cupIndex = 0;
                cupScores = [0, 0, 0, 0];
                startRace(config);
            },
            onRestart() { startRace(currentConfig || { mode: 'quick', trackId: AD.TRACKS[0].id, laps: 3 }); },
            onResume() { setPaused(false); },
            onQuit() { showMenu(); },
            onNext() {
                cupIndex++;
                startRace(currentConfig);
            },
            onSettings(next) {
                AD.saveSettings(next, readonly);
                audio.setSettings(next);
                reducedMotion = next.reducedMotion ||
                    (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
                quality = applyQuality();
            }
        }, settings);

        function buildTrack(config) {
            if (config.mode === 'daily') return AD.buildTrack(AD.dailyTrack(dailySeed()), dailySeed());
            if (config.mode === 'cup') return AD.buildTrack(AD.TRACKS[cupIndex % AD.TRACKS.length]);
            return AD.buildTrack(AD.trackById(config.trackId));
        }

        function startRace(config) {
            currentConfig = Object.assign({}, config);
            const track = buildTrack(config);
            const best = bests[track.id];
            const mode = config.mode;
            race = AD.createRace({
                track: track,
                mode: mode,
                laps: config.laps || track.laps,
                opponents: mode === 'time_trial' ? 0 : 3,
                weapons: settings.weapons && mode !== 'time_trial',
                ghostTape: mode === 'time_trial' && best ? best.tape : null,
                showGhost: settings.showGhost,
                readonly: readonly
            });
            renderer.setTrack(track);
            audio.setBiome(track.biomeId);
            post.setBiome(track.biomeId);
            scheduler.reset();
            paused = false;
            camera.shake = 0;
            ui.hide();
            setPaused(false);
            audio.resume();
        }

        function showMenu() {
            race = AD.createRace({ track: AD.buildTrack(AD.TRACKS[1]), opponents: 3, weapons: false });
            race.state = 'running';
            race.countdown = 0;
            race.cars[0].isPlayer = false;
            renderer.setTrack(race.track);
            audio.setBiome(race.track.biomeId);
            post.setBiome(race.track.biomeId);
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

        function readInput() {
            const input = race.playerInput;
            const touch = ui.touch;
            const key = name => pressed.has(name);
            input.throttle = (key('ArrowUp') || key('KeyW') || touch.throttle) ? 1 : 0;
            input.brake = (key('ArrowDown') || key('KeyS') || touch.brake) ? 1 : 0;
            /* Raw arcade steering: the solver maps this to a nose response
               that is identical forward and reverse (see physics steerMag),
               so no input-side flip is needed here. */
            input.steer = (((key('ArrowRight') || key('KeyD') || touch.right) ? 1 : 0) -
                ((key('ArrowLeft') || key('KeyA') || touch.left) ? 1 : 0));
            input.handbrake = (key('ShiftLeft') || key('ShiftRight') || touch.handbrake) ? 1 : 0;
            input.fire = (key('Space') || touch.fire) ? 1 : 0;
            input.boost = (key('ControlLeft') || key('ControlRight') || touch.boost) ? 1 : 0;
        }

        function drainEvents() {
            for (const evt of race.events) {
                audio.event(evt);
                if (evt.type === 'hit') {
                    renderer.burst('spark', evt.x, evt.y, evt.heavy ? 18 : 8, '#ffb347');
                    if (evt.heavy) camera.shake = Math.min(1, camera.shake + 0.6);
                } else if (evt.type === 'pickup') {
                    renderer.burst('spark', evt.x, evt.y, 10, '#8ce9ff');
                    if (evt.car === 0) hud.showMessage(AD.WEAPONS[evt.id].name, 1.1);
                } else if (evt.type === 'respawn') {
                    renderer.burst('spark', evt.x, evt.y, 14, '#a8e08a');
                } else if (evt.type === 'emp_pulse') {
                    renderer.burst('spark', evt.x, evt.y, 22, '#7dd3fc');
                } else if (evt.type === 'lap' && !reducedMotion) {
                    hud.showMessage(labels.lap + ' ' + evt.lap + ' · ' + AD.formatTime(evt.time), 1.8);
                } else if (evt.type === 'go') {
                    hud.showMessage(labels.ready, 0.9);
                }
            }
            race.events.length = 0;
        }

        function emitFx(dt) {
            if (!race || race.state !== 'running') return;
            const v = race.cars[0].v;
            const speed = Math.max(0, v.vx);
            const surface = v.surface || 'asphalt';
            const dustColour =
                surface === 'sand' ? '#d8b078' :
                surface === 'gravel' ? '#a89a84' :
                surface === 'mud' ? '#6b5a42' :
                surface === 'wet' ? '#9db4c8' :
                surface === 'ice' ? '#e2edf5' :
                surface === 'salt' ? '#f2f2ea' :
                race.track.biome.night ? '#8fa4bd' : '#c9cbd0';
            if (v.slip > 2.4 && speed > 8 && quality.particles > 0 && frameCount % 2 === 0) {
                const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
                for (const side of [-1, 1]) {
                    const x = v.x - cos * 1.7 + sin * side * 1.05;
                    const y = v.y - sin * 1.7 - cos * side * 1.05;
                    renderer.smoke(x, y, dustColour);
                    renderer.addDecal(x, y, v.psi, Math.min(1, v.slip / 9));
                }
            }
            if (v.boost && speed > 12 && frameCount % 3 === 0) {
                renderer.burst('boost', v.x - Math.cos(v.psi) * 2.4, v.y - Math.sin(v.psi) * 2.4, 2, '#8ce9ff');
            }
            if (v.damage > 62 && speed > 4 && frameCount % 7 === 0) {
                renderer.smoke(v.x, v.y - 1.2, '#5c5c62');
            }
            if (v.impact > 0.55 && quality.particles > 0) {
                renderer.burst('spark', v.x, v.y, 3, '#ffb347');
            }
        }

        function autoQuality(dt) {
            if (settings.quality !== 'auto') return;
            frameAccum += dt;
            if (frameAccum < 2.5) return;
            const avg = frameAccum / Math.max(1, frameCount);
            frameAccum = 0; frameCount = 0;
            const index = QUALITY_ORDER.indexOf(qualityLevel);
            if (avg > 0.024 && index > 0) qualityLevel = QUALITY_ORDER[index - 1];
            else if (avg < 0.014 && index < QUALITY_ORDER.length - 1) qualityLevel = QUALITY_ORDER[index + 1];
            else return;
            quality = applyQuality();
        }

        function finishRace() {
            if (!race || !race.result || race.mode === 'time_trial' && race.result.saved) return;
            const result = race.result;
            result.saved = true;
            const key = race.track.id;
            const previous = bests[key];
            const record = !readonly && (!previous || result.time < previous.time);
            if (record) {
                bests[key] = { time: result.time, tape: result.tape };
                AD.saveBests(bests, readonly);
            }
            /* Cup points: 25-18-15-12 for P1..P4, accumulated across the cup. */
            let cupLine = '';
            if (race.mode === 'cup' && race.standings) {
                const points = [25, 18, 15, 12];
                for (let i = 0; i < race.standings.length; i++) {
                    const slot = race.standings[i].index || 0;
                    cupScores[slot] = (cupScores[slot] || 0) + (points[i] || 12);
                }
                const me = cupScores[0] || 0;
                cupLine = '  ' + labels.cup + ' ' + me;
            }
            ui.showResults(result, {
                field: race.cars.length,
                track: race.track.name + cupLine,
                record: record,
                hasNext: race.mode === 'cup' && cupIndex < AD.TRACKS.length - 1
            });
        }

        function loop(now) {
            raf = requestAnimationFrame(loop);
            const dt = Math.min(0.05, Math.max(0, (now - last) / 1000));
            last = now;
            if (!isActive() || document.hidden || !race) return;
            frameAccum += dt; frameCount++;

            if (!paused) {
                if (race.state === 'running' && race.cars[0].isPlayer) readInput();
                scheduler.advance(dt, step => AD.stepRace(race, step));
                drainEvents();
                emitFx(dt);
                audio.update(race, true);
                if (race.state === 'finished' && race.cars[0].isPlayer) finishRace();
            }
            camera.update(race.cars[0].v, dt, quality, reducedMotion);
            renderer.render(race, camera, dt, reducedMotion);
            post.render(race, dt, reducedMotion);
            if (race.cars[0].isPlayer && !paused) hud.render(race, dt, settings);
            else hudCanvas.getContext('2d').clearRect(0, 0, hudCanvas.width, hudCanvas.height);
            autoQuality(dt);
        }

        const entry = {
            host, renderer, post, hud, audio, ui, observer,
            stats() {
                return {
                    state: race ? race.state : 'none',
                    time: race ? race.time : 0,
                    speedKmh: race ? AD.vehicleSpeedKmh(race.cars[0].v) : 0,
                    paused: paused,
                    quality: settings.quality === 'auto' ? qualityLevel : settings.quality,
                    renderer: renderer.stats(),
                    post: post.stats()
                };
            },
            stop() { if (raf) cancelAnimationFrame(raf); raf = 0; window.removeEventListener('resize', resize); },
            onKeyDown(event) {
                if (!isActive() || !race || !race.cars[0].isPlayer) return;
                if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Space'].includes(event.code)) event.preventDefault();
                if (event.code === 'KeyP' || event.code === 'Escape') { setPaused(!paused); return; }
                if (event.code === 'KeyR' && currentConfig) { startRace(currentConfig); return; }
                if (!paused) { audio.unlock(); pressed.add(event.code); }
            },
            onKeyUp(event) { pressed.delete(event.code); },
            onBlur() {
                pressed.clear();
                if (race && race.cars[0].isPlayer && !paused) setPaused(true);
            },
            onVisibility() {
                if (document.hidden) { pressed.clear(); if (race && race.cars[0].isPlayer && !paused) setPaused(true); }
                else last = performance.now();
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

    window.AuraDriftApp = {
        render, dispose,
        /* Read-only diagnostic surface for smoke tests and the browser console. */
        stats() {
            const result = [];
            for (const entry of instances.values()) result.push(entry.stats());
            return result;
        }
    };
})();
