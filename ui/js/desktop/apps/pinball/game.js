// AuraPinball game shell: mounts the view, loads a table (scene, session, lamps, DMD, audio) and runs the frame loop.
import { STEP, FLIPPER } from './physics/constants.js';
import { lerpSnapshot } from './physics/snapshot.js';
import { STORAGE, normalizeSettings, normalizeScores, copy, clamp } from './config.js';
import { TABLES, tableOf } from './tables/index.js';
import { createSession } from './session.js';
import { createInput } from './input.js';
import { createScene } from './render/scene.js';
import { createLamps } from './lights/controller.js';
import { createDmd } from './dmd/core.js';
import { createRaster } from './dmd/raster.js';
import { createDmdView } from './dmd/view.js';
import { createAudio } from './audio/index.js';
import { createView } from './ui/views.js';
import { createPresenter } from './presenter.js';

const MAX_TICKS = 32, MAX_DT = .05, OVER_DELAY = 3.2;
const MENU_PANELS = ['settings', 'help', 'confirm_exit', 'binding'];

export function mountGame(host, context = {}) {
    const assetURL = context.assetURL || (path => path), locale = document.documentElement.lang || 'en';
    // tr(key, vars): the pinball.* translation with {n}/{score}/{name} substituted. Scene, DMD and voice pass unprefixed keys.
    const tr = (key, vars) => {
        const raw = context.t?.('pinball.' + key), text = raw && raw !== 'pinball.' + key ? String(raw) : key;
        return vars ? text.replace(/\{(\w+)\}/g, (m, name) => (name in vars ? String(vars[name]) : m)) : text;
    };
    const read = key => { try { return JSON.parse(localStorage.getItem(key)); } catch (_) { return null; } };
    let settings = normalizeSettings(read(STORAGE.settings)), scores = normalizeScores(read(STORAGE.scores)), tableId = TABLES[context.table] ? context.table : 'neon';
    let recent = [], session = null, scene = null, audio = null, lamps = null, dmd = null, presenter = null, loadController = null, modes = null, blend = null, autoplay = null, held = false;   // held: test-only physics freeze (the frame loop and camera keep running)
    let disposed = false, loading = false, generation = 0, raf = 0, last = 0, accumulator = 0, clock = 0, paused = true, visible = true, wasActive = true;
    let camera = settings.camera, count = 1, names = [], returnPanel = 'lobby', saved = false, overFor = 0;
    const life = new AbortController(), view = createView(host, tr, assetURL, TABLES), root = view.root;
    const windowElement = host.closest('.vd-window'), motion = matchMedia('(prefers-reduced-motion: reduce)');
    const raster = createRaster(), dmdView = createDmdView(view.dmdHost, { glow: !(settings.reduced || motion.matches) });
    dmdView.element.classList.remove('pb-dmd'); dmdView.element.classList.add('pb-dmd-screen');   // the HUD dock already is the bezel
    const active = () => !disposed && !document.hidden && (context.isActive?.() ?? true);
    const game = () => session?.game || null;
    const reducedNow = () => settings.reduced || motion.matches;

    function write(key, value) { try { localStorage.setItem(key, JSON.stringify(value)); } catch (_) { view.notice('storage_warning'); } }
    function updateInsets() {
        if (!scene) return;
        const dock = view.dmdHost.getBoundingClientRect(), stage = view.stage.getBoundingClientRect();
        scene.setInsets({ top: dock.height ? Math.max(0, dock.bottom - stage.top + 6) : 0 });
    }
    function applyCamera() { scene?.setCamera(camera === 'showcase' ? 'showcase' : game() ? camera : 'attract'); }
    function syncCamera() { for (const select of root.querySelectorAll('[data-setting=camera]')) select.value = camera; }
    function applyReduced() { root.dataset.reduced = String(settings.reduced); dmdView.setGlow(!reducedNow()); }
    function panel(kind, extra = {}) {
        const g = game();
        view.panel(kind, { table: tableId, settings, scores, game: g ? copy(g) : kind === 'lobby' ? {} : null, count, names, ...extra });
        if (kind === 'settings') view.setCallouts(audio?.voiceAvailable() ?? false);
        syncCamera(); updateInsets();
    }
    function unlockAudio() {
        const current = audio; if (!current) return;
        void current.unlock().then(async unlocked => {
            const ready = unlocked ? await current.ready() : false;
            if (ready === false && !disposed && audio === current) view.notice('audio_error');
        }).catch(() => { if (!disposed && audio === current) view.notice('audio_error'); });
    }
    function saveSettings() { settings = normalizeSettings(settings); write(STORAGE.settings, settings); input.setSettings(settings); audio?.setSettings(settings); applyReduced(); }

    function pause(reason) {
        const g = game();
        if (!g || g.phase === 'over') return;
        paused = true; input.clear(); audio?.pause(); last = 0; accumulator = 0;
        if (!MENU_PANELS.includes(view.kind())) panel('pause', { reason });
    }
    function resume() {
        const g = game();
        if (!g) { camera = settings.camera; applyCamera(); panel('lobby'); return; }
        if (g.phase === 'over' || loading) return;
        paused = false; last = 0; accumulator = 0; input.clear();
        if (camera === 'showcase') camera = settings.camera;
        applyCamera(); panel(''); audio?.resume(); unlockAudio(); view.focus(); schedule();
    }
    function toLobby() {
        paused = true; input.clear(); session?.endGame(); presenter?.reset(); audio?.music('attract'); blend = null; autoplay = null;
        camera = settings.camera; applyCamera(); panel('lobby');
    }
    function start(training = false) {
        if (loading || !session) return;
        input.clear(); unlockAudio(); presenter.reset(); blend = null;
        const players = (names.length ? names : [tr('player', { n: 1 })]).slice(0, training ? 1 : 4);
        session.startGame({ players, training, seed: context.seed ?? ((Date.now() % 0x7fffffff) || 1), scores: scores[tableId] });
        saved = false; overFor = 0; paused = true; camera = settings.camera; resume();
    }
    function finish(g) {
        saved = true; paused = true;
        if (!g.training) {
            const rows = g.results.map(row => ({ name: row.name, score: row.score, date: Date.now() }));
            scores = normalizeScores({ ...scores, [tableId]: [...scores[tableId], ...rows] }); write(STORAGE.scores, scores);
            session.setScores(scores[tableId]);
        }
        panel('game_over', { match: g.match ? g.match.digit : undefined });
    }

    const input = createInput(root, settings, { active, playing: () => !!game() && !paused && !loading, action(key) {
        if (key === 'focusLost' || key === 'disconnected') { pause(key === 'disconnected' ? 'disconnected' : undefined); return; }
        if (key === 'pause') {
            if (['settings', 'help', 'scores', 'confirm_exit'].includes(view.kind())) { back(); return; }
            if (!game()) return;
            if (paused && ['pause', 'showcase'].includes(view.kind())) resume(); else pause();
        } else if (key === 'start') { if (view.kind() === 'lobby') { names = view.names(); start(false); } }
        else if (key === 'camera' && game() && !paused) { camera = camera === 'follow' ? 'play' : camera === 'play' ? 'top' : 'follow'; applyCamera(); syncCamera(); }
        else if (key === 'reset' && game()?.training && !paused) session.resetTraining();
    }, binding() { saveSettings(); panel('settings'); } });

    function schedule() { if (!raf && visible && !disposed) raf = requestAnimationFrame(frame); }
    function syncVisibility() {
        visible = !document.hidden && root.isConnected && (!windowElement || !windowElement.hidden && windowElement.style.display !== 'none' && !windowElement.classList.contains('vd-space-hidden'));
        if (!visible) { pause(); cancelAnimationFrame(raf); raf = 0; last = 0; } else schedule();
    }
    document.addEventListener('visibilitychange', syncVisibility, { signal: life.signal });
    const observer = new MutationObserver(syncVisibility), resizer = new ResizeObserver(updateInsets);
    if (windowElement) observer.observe(windowElement, { attributes: true, attributeFilter: ['style', 'class', 'hidden'] });
    resizer.observe(root);
    root.addEventListener('webglcontextlost', event => { event.preventDefault(); pause(); panel('error', { error: 'webgl_error' }); }, { capture: true, signal: life.signal });

    function teardown() {
        session?.dispose(); scene?.dispose(); audio?.dispose();
        session = scene = audio = lamps = dmd = presenter = modes = blend = null;
    }
    async function loadTable(id) {
        const token = ++generation; loading = true; paused = true; saved = false; last = 0; accumulator = 0; input.clear();
        loadController?.abort(); loadController = new AbortController(); teardown();
        tableId = TABLES[id] ? id : 'neon';
        const table = tableOf(tableId), layout = table.layout, seed = context.seed ?? ((Date.now() % 0x7fffffff) || 1);
        root.style.setProperty('--pb-accent', layout.meta.accent); panel('loading');
        let next = null;
        try {
            next = await createScene(view.stage, { tableId, layout, settings: { ...settings, camera: 'attract' }, assetURL, signal: loadController.signal, tr });
            if (disposed || token !== generation) { next.dispose(); return; }
            scene = next; session = createSession(table, { seed }); lamps = createLamps(layout, { seed });
            dmd = createDmd({ raster, tr, locale }); audio = createAudio({ tableId, settings, assetURL, locale, tr });
            presenter = createPresenter({ layout, lamps, dmd, audio, scene, callouts: () => settings.callouts !== false, reduced: reducedNow });
            session.onPresentation(command => { presenter.handle(command); note('presentation', command); }); session.onEvent(ev => { presenter.event(ev); note('physics', ev); });
            session.setScores(scores[tableId]); modes = new Uint8Array(layout.lamps.length);
            loading = false; camera = settings.camera; applyReduced(); applyCamera(); panel('lobby'); schedule();
        } catch (error) {
            next?.dispose();
            if (!disposed && token === generation) { teardown(); loading = false; panel('error', { error: error && error.message === 'webgl_error' ? 'webgl_error' : 'load_error' }); }
        }
    }

    // Small ring of what just happened (physics events + presentation commands) for scene toys; drained once per frame.
    function note(source, ev) { if (recent.length < 48) recent.push({ source, type: ev.type, id: ev.id ?? ev.name ?? null }); }
    function doTicks(time, dt) {
        accumulator += dt;
        let steps = 0;
        while (accumulator >= STEP && steps < MAX_TICKS) {
            const controls = autoplay ? autoplay(session.snapshotPair().curr.balls, session.ticks) : input.at(time - (accumulator - STEP) * 1000);
            session.tick(controls); accumulator -= STEP; steps++;
        }
        if (steps >= MAX_TICKS) accumulator = 0;   // never spiral: drop the debt, slow down, but do not pause
    }
    function flippersOf(snap) {
        return (session.layout.flippers || []).map((f, i) => {
            const angle = snap.flippers[2 * i], rest = f.rest ?? FLIPPER.rest, up = f.up ?? FLIPPER.up;
            return { id: f.id, angle, up: angle > (rest + up) / 2 };
        });
    }
    function updateHud(snap, g, running) {
        const shooter = snap.balls.some(b => b.mode === 'shooter'), pulling = running && (shooter || snap.plunger > .02);
        view.hud.setPlunger(snap.plunger, pulling);
        const save = g?.ballSave, left = save && save.armed && !save.unlimited && g.phase !== 'bonus' && save.soft !== null ? Math.max(0, save.soft - g.time) : 0;   // counts down from the first scoring switch; hidden before the timer starts and once it ends
        view.hud.setBallSave(running ? left : 0);
        view.hud.setInitialsHint(g?.phase === 'initials');
        if (running) audio?.plunger(snap.plunger);
    }
    function frame(time) {
        raf = 0; if (disposed || !visible) return;
        const nowActive = active(); if (wasActive && !nowActive) pause(); wasActive = nowActive;
        const dt = last ? clamp((time - last) / 1000, 0, MAX_DT) : 0; last = time; clock += dt;
        input.poll();
        if (session && !loading) {
            const g = game(), running = !!g && !paused && !held;
            if (running) {
                for (const n of input.nudges()) { const dx = .2 * n.x, dy = .14 * Math.abs(n.z); if (session.nudge(dx, dy)) presenter.nudge(dx, dy); }
                doTicks(time, dt);
                if (g.phase === 'over' && !saved && (overFor += dt) > OVER_DELAY) finish(g);
            } else input.nudges();
            const step = paused && g ? 0 : dt;   // the attract show keeps running behind the lobby; only a paused round freezes
            presenter.attract(step, !g);
            session.lampModes(modes); lamps.setModes(modes); lamps.update(step);
            dmd.update(step, session.dmdState()); dmdView.draw(dmd.frame, dmd.version);
            const pair = session.snapshotPair(), snap = running ? (blend = lerpSnapshot(pair.prev, pair.curr, clamp(accumulator / STEP, 0, 1), blend)) : pair.curr;
            scene.update({ snap, lamps, dmd: { frame: dmd.frame, version: dmd.version }, toyState: session.toyState(), events: recent, dt, time: clock, paused, reduced: reducedNow(), interactive: nowActive });
            recent = [];
            updateHud(snap, g, running);
            if (running) audio?.roll(session.rollBalls());
        }
        schedule();
    }

    function back() { input.cancelCapture(); if (returnPanel === 'pause' && game()) panel('pause'); else panel('lobby'); }
    function chooseCamera(value) {
        camera = value;
        if (value === 'showcase') { if (!game()) names = view.names(); pause(); panel('showcase'); }
        else { settings.camera = value; saveSettings(); if (!game() && view.kind() === 'showcase') panel('lobby'); }
        applyCamera();
    }
    root.addEventListener('click', event => {
        const button = event.target.closest('button[data-action]'); if (!button || !root.contains(button)) return;
        const action = button.dataset.action;
        if (action.startsWith('table-')) { names = view.names(); void loadTable(action.slice(6)); return; }
        if (action === 'start' || action === 'training') { names = view.names(); start(action === 'training'); return; }
        if (action === 'retry') { void loadTable(tableId); return; }
        if (action === 'resume') { resume(); return; }
        if (action === 'pause') { pause(); return; }
        if (['settings', 'help', 'scores'].includes(action)) { returnPanel = game() ? 'pause' : 'lobby'; if (game()) pause(); panel(action); return; }
        if (action === 'back') { back(); return; }
        if (action === 'restart' || action === 'quit') { returnPanel = 'pause'; panel('confirm_exit', { restart: action === 'restart' }); return; }
        if (action === 'confirm_restart') { start(!!game()?.training); return; }
        if (action === 'lobby') { toLobby(); return; }
        if (action === 'fullscreen') { if (document.fullscreenElement === root) void document.exitFullscreen().catch(() => {}); else void root.requestFullscreen?.().catch(() => view.notice('fullscreen_error')); return; }
        if (action.startsWith('bind-')) { const [, device, key] = action.split('-'); panel('binding', { device }); input.capture(key, device === 'keys' ? 'key' : 'pad'); return; }
        if (action === 'cancel_binding') { input.cancelCapture(); panel('settings'); }
    }, { signal: life.signal });
    function change(event) {
        const target = event.target;
        if (target.dataset.field === 'players') { names = view.names(); count = Math.max(1, Math.min(4, Number(target.value) || 1)); panel('lobby'); return; }
        const key = target.dataset.setting; if (!key) return;
        if (key === 'camera') { chooseCamera(target.value); return; }
        const previous = settings.quality;
        if (key === 'quality') settings.quality = target.value;
        else if (key === 'reduced' || key === 'callouts') settings[key] = target.checked;
        else if (['effects', 'music', 'ambience'].includes(key)) settings[key] = Number(target.value);
        saveSettings(); if (previous !== settings.quality) scene?.setQuality(settings.quality);
    }
    root.addEventListener('change', change, { signal: life.signal });
    root.addEventListener('input', event => { if (event.target.type === 'range') change(event); }, { signal: life.signal });
    syncVisibility(); void loadTable(tableId);

    return {
        inspect() {
            const snap = session?.snapshotPair().curr, di = dmd?.inspect();
            return {
                table: tableId, panel: view.kind(), loading, paused, attract: !!session && !game(), settings: copy(settings), round: game() ? copy(game()) : null,
                physics: snap ? { tick: snap.tick, balls: snap.balls.map(b => ({ id: b.id, x: b.x, y: b.y, h: b.h, vx: b.vx, vy: b.vy, mode: b.mode })), flippers: flippersOf(snap) } : null,
                stats: session?.world.stats(), graphics: scene?.stats(), audio: audio?.inspect(), input: input.inspect(),
                lamps: lamps?.inspect() || { count: 0, lit: 0, shows: [] },
                dmd: di ? { base: di.base.scene, scene: di.scene, version: dmd.version, text: di.text } : null
            };
        },
        ...(context.testing ? {
            advance(ticks, command) {
                if (!game()) throw Error('round');
                input.poll();
                for (let i = 0; i < ticks && game().phase !== 'over'; i++) session.tick(command || (autoplay ? autoplay(session.snapshotPair().curr.balls, session.ticks) : input.at(performance.now())));
                return copy(game());
            },
            start(training = false, players) { names = players || names; start(training); },
            autoplay(fn) { autoplay = fn || null; },
            hold(on) { held = !!on; },
            placeBalls(coords) { return session.placeBalls(coords); },
            stress() { if (!game()) throw Error('round'); session.stress(4); resume(); },
            fire(ev) { session.inject(ev); },
            internals: () => ({ session, scene, lamps, dmd, audio, view, input, presenter, settings })
        } : {}),
        dispose() {
            if (disposed) return; disposed = true; generation++; loadController?.abort(); life.abort(); observer.disconnect(); resizer.disconnect();
            cancelAnimationFrame(raf); input.dispose(); teardown(); dmdView.dispose(); view.dispose();
        }
    };
}
