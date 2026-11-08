import { STEP, tableOf, normalizeSettings, normalizeScores, copy } from './tables.js';
import { createSimulation } from './physics.js';
import { createRound, currentPlayer, gateOpen, takeActions, tickRound, handleEvent, nudgeRound } from './rules.js';
import { createInput } from './input.js';
import { createAudio } from './audio.js';
import { createScene } from './scene.js';
import { createView } from './views.js';

const SETTINGS = 'aurago.desktop.pinball.settings.v1', SCORES = 'aurago.desktop.pinball.scores.v1';

export function mountGame(host, context = {}) {
    const tr = key => context.t?.('pinball.' + key) || key;
    const assetURL = context.assetURL || (path => path);
    const read = key => { try { return JSON.parse(localStorage.getItem(key)); } catch (_) { return null; } };
    let settings = normalizeSettings(read(SETTINGS)), scores = normalizeScores(read(SCORES)), table = 'neon';
    let scene = null, physics = null, audio = null, round = null, loadController = null;
    let disposed = false, loading = false, starting = false, generation = 0, raf = 0, last = 0, accumulator = 0;
    let paused = true, visible = true, wasActive = true, camera = 'play', count = 1, names = [], returnPanel = 'lobby', saved = false, lastNotice = -1, lastLeft = false, lastRight = false;
    const life = new AbortController(), view = createView(host, tr, assetURL), root = view.root, windowElement = host.closest('.vd-window'), motion = matchMedia('(prefers-reduced-motion: reduce)');
    const active = () => !disposed && !document.hidden && (context.isActive?.() ?? true);
    function write(key, value) { try { localStorage.setItem(key, JSON.stringify(value)); } catch (_) { view.notice('storage_warning'); } }
    function panel(kind, extra = {}) { view.panel(kind, { table, settings, scores, game: round, count, names, ...extra }); }
    function pause(reason) {
        if (!round || round.phase === 'finished') return;
        paused = true; input.clear(); audio?.pause(); last = 0; accumulator = 0;
        if (!['settings', 'help', 'confirm_exit', 'binding'].includes(view.kind())) panel('pause', { reason });
    }
    function resume() {
        if (!round) { camera='play';scene?.setCamera(camera);root.querySelector('[data-setting=camera]').value=camera;panel('lobby');return; }
        if (!round || round.phase === 'finished' || loading) return;
        paused = false; last = 0; accumulator = 0; input.clear();
        camera = camera === 'showcase' ? 'play' : camera; scene?.setCamera(camera);
        root.querySelector('[data-setting=camera]').value = camera;
        panel(''); audio?.resume(); unlockAudio(); view.focus(); schedule();
    }
    function unlockAudio() {
        const current = audio; if (!current) return;
        void current.unlock().then(async unlocked => {
            const ready = unlocked ? await current.ready() : false;
            if (ready === false && !disposed && audio === current) view.notice('audio_error');
        }).catch(() => { if (!disposed && audio === current) view.notice('audio_error'); });
    }
    function saveSettings() { settings = normalizeSettings(settings); write(SETTINGS, settings); input.setSettings(settings); audio?.setSettings(settings); }
    const input = createInput(root, settings, { active, playing: () => !!round && !paused && !loading, action(key) {
        if (key === 'focusLost' || key === 'disconnected') { pause(key === 'disconnected' ? 'disconnected' : undefined); return; }
        if (key === 'pause') {
            if (['settings', 'help', 'scores', 'confirm_exit'].includes(view.kind())) { back(); return; }
            if (!round) return; if (paused && ['pause', 'showcase'].includes(view.kind())) resume(); else pause();
        }
        if (key === 'camera' && round) { camera = camera === 'play' ? 'top' : 'play'; scene?.setCamera(camera); root.querySelector('[data-setting=camera]').value = camera; }
        if (key === 'reset' && round?.training && !paused) {
            const id = ++round.serial; round.active = [id]; round.actions.length = 0; round.launched = false; round.multiball = false; round.saverUntil = 0; round.tiltEnergy = 0;
            currentPlayer(round).tilt = false; currentPlayer(round).warnings = 0; physics.setCoils(true); physics.resetBall(id);
        }
    }, binding() { saveSettings(); panel('settings'); } });
    function schedule() { if (!raf && visible && !disposed) raf = requestAnimationFrame(frame); }
    function syncVisibility() {
        visible = !document.hidden && root.isConnected && (!windowElement || !windowElement.hidden && windowElement.style.display !== 'none' && !windowElement.classList.contains('vd-space-hidden'));
        if (!visible) { pause(); cancelAnimationFrame(raf); raf = 0; last = 0; } else schedule();
    }
    document.addEventListener('visibilitychange', syncVisibility, { signal: life.signal });
    const observer = new MutationObserver(syncVisibility);
    if (windowElement) observer.observe(windowElement, { attributes: true, attributeFilter: ['style', 'class', 'hidden'] });
    root.addEventListener('webglcontextlost', event => { event.preventDefault(); pause(); panel('error', { error: 'webgl_error' }); }, { capture: true, signal: life.signal });
    async function loadTable(id) {
        const token = ++generation; loading = true; paused = true; round = null; saved = false; last = 0; accumulator = 0; input.clear();
        loadController?.abort(); loadController = new AbortController();
        physics?.dispose(); physics = null; scene?.dispose(); scene = null; audio?.dispose(); audio = null;
        table = tableOf(id).id; root.style.setProperty('--pb-accent', tableOf(table).accent); panel('loading');
        let nextScene = null, nextPhysics = null;
        try {
            nextScene = await createScene(view.stage, table, settings, assetURL, loadController.signal);
            nextPhysics = await createSimulation(nextScene.physics);
            if (disposed || token !== generation) { nextScene.dispose(); nextPhysics.dispose(); return; }
            scene = nextScene; physics = nextPhysics; audio = createAudio(table, settings, assetURL); loading = false; panel('lobby'); schedule();
        } catch (error) {
            nextScene?.dispose(); nextPhysics?.dispose();
            if (!disposed && token === generation) { loading = false; panel('error', { error: error.message === 'webgl_error' ? 'webgl_error' : 'load_error' }); }
        }
    }
    async function start(training = false) {
        if (loading || starting || !scene) return;
        starting = true; input.clear(); audio?.resume(); unlockAudio();
        const token = generation; paused = true;
        try {
            const next = await createSimulation(scene.physics);
            if (disposed || token !== generation) { next.dispose(); return; }
            physics?.dispose(); physics = next;
            round = createRound(table, names.length ? names : [tr('player').replace('{n}', '1')], training);
            saved = false; lastNotice = -1; lastLeft = false; lastRight = false; resume();
        } catch (_) { audio?.pause(); if (!disposed) panel('error', { error: 'load_error' }); }
        finally { starting = false; }
    }
    function actions() {
        for (const action of takeActions(round)) {
            if (action.type === 'feed') physics.feed(action.id, action.auto);
            if (action.type === 'eject') physics.eject(action.id);
            if (action.type === 'coils') physics.setCoils(action.enabled);
            if (action.type === 'reset_targets') physics.resetTargets();
        }
    }
    function stepOne(command) {
        if (command.left && !lastLeft) audio?.effect('flipper', -.1);
        if (command.right && !lastRight) audio?.effect('flipper', .1);
        if (!command.left && lastLeft) audio?.effect('flipper', -.1, .4);
        if (!command.right && lastRight) audio?.effect('flipper', .1, .4);
        lastLeft = command.left; lastRight = command.right;
        physics.setGate(gateOpen(round)); physics.setCoils(!currentPlayer(round).tilt); physics.setTargets(currentPlayer(round).bank);
        physics.step(command); tickRound(round, physics.snapshot().time);
        for (const event of physics.events()) {
            handleEvent(round, event); scene?.hit(event);
            if (['bumper', 'sling', 'target', 'launch', 'drain', 'lock'].includes(event.type)) audio?.effect(event.type, physics.snapshot().balls.find(b => b.id === event.ball)?.position[0] || 0);
        }
        actions();
        if (lastNotice !== round.noticeSerial) { lastNotice = round.noticeSerial; if (['jackpot', 'bonus', 'multiball', 'mission_complete'].includes(round.notice)) audio?.effect(round.notice === 'bonus' ? 'bonus' : 'jackpot'); }
        if (round.phase === 'finished' && !saved) {
            saved = true; paused = true; audio?.pause();
            if (!round.training) { scores[table] = [...scores[table], ...round.players.map(p => ({ name: p.name, score: p.score, date: Date.now() }))].sort((a, b) => b.score - a.score).slice(0, 10); write(SCORES, scores); }
            panel('game_over');
        }
    }
    function frame(time) {
        raf = 0; if (disposed || !visible) return;
        const nowActive = active(); if (wasActive && !nowActive) pause(); wasActive = nowActive;
        const dt = last ? Math.max(0, (time - last) / 1000) : 0; last = time;
        input.poll();
        if (round && !paused && !loading) {
            if (dt > .1) pause('performance_pause');
            else {
                for (const n of input.nudges()) if (nudgeRound(round, Math.min(1, Math.hypot(n.x, n.z)))) physics.nudge(n.x, n.z);
                accumulator += dt; let steps = 0;
                while (accumulator >= STEP && steps++ < 48 && !paused) { stepOne(input.at(time - (accumulator - STEP) * 1000)); accumulator -= STEP; }
            }
        }
        const snapshot = physics?.snapshot(round && !paused ? accumulator / STEP : 1);
        scene?.update(snapshot, Math.min(dt, .1), paused, settings.reduced || motion.matches, nowActive, round ? currentPlayer(round) : null);
        if (round) { view.updateHUD(round, snapshot); audio?.rolling(paused ? 0 : Math.max(0, ...snapshot.balls.filter(b => b.grounded).map(b => Math.hypot(...b.velocity)))); }
        schedule();
    }
    function back() { input.cancelCapture(); if (returnPanel === 'pause' && round) panel('pause'); else panel('lobby'); }
    root.addEventListener('click', event => {
        const button = event.target.closest('button[data-action]'); if (!button || !root.contains(button)) return;
        const action = button.dataset.action;
        if (action.startsWith('table-')) { names = view.names(); void loadTable(action.slice(6)); return; }
        if (action === 'start' || action === 'training') { names = view.names(); void start(action === 'training'); return; }
        if (action === 'retry') { void loadTable(table); return; }
        if (action === 'resume') { resume(); return; }
        if (action === 'pause') { pause(); return; }
        if (['settings', 'help', 'scores'].includes(action)) { returnPanel = round ? 'pause' : 'lobby'; if (round) pause(); panel(action); return; }
        if (action === 'back') { back(); return; }
        if (action === 'restart' || action === 'quit') { returnPanel = 'pause'; panel('confirm_exit', { restart: action === 'restart' }); return; }
        if (action === 'confirm_restart') { void start(round?.training); return; }
        if (action === 'lobby') { paused = true; round = null; input.clear(); audio?.pause(); physics?.clearBalls(); panel('lobby'); return; }
        if (action === 'fullscreen') { if (document.fullscreenElement === root) void document.exitFullscreen().catch(() => {}); else void root.requestFullscreen?.().catch(() => view.notice('fullscreen_error')); return; }
        if (action.startsWith('bind-')) { const [, device, key] = action.split('-'); panel('binding', { device }); input.capture(key, device === 'keys' ? 'key' : 'pad'); return; }
        if (action === 'cancel_binding') { input.cancelCapture(); panel('settings'); }
    }, { signal: life.signal });
    function change(event) {
        const target = event.target;
        if (target.dataset.field === 'players') { names = view.names(); count = Math.max(1, Math.min(4, Number(target.value) || 1)); panel('lobby'); return; }
        const key = target.dataset.setting; if (!key) return;
        if (key === 'camera') { camera = target.value; if (camera === 'showcase') { if(!round)names=view.names();pause();panel('showcase'); } else if(!round&&view.kind()==='showcase')panel('lobby');scene?.setCamera(camera);return; }
        const previous = settings.quality;
        if (key === 'quality') settings.quality = target.value;
        else if (key === 'reduced') settings.reduced = target.checked;
        else if (['effects', 'ambience'].includes(key)) settings[key] = Number(target.value);
        saveSettings(); if (previous !== settings.quality) scene?.setQuality(settings.quality);
    }
    root.addEventListener('change', change, { signal: life.signal });
    root.addEventListener('input', event => { if (event.target.type === 'range') change(event); }, { signal: life.signal });
    syncVisibility(); void loadTable(table);
    return {
        inspect: () => ({ table, panel: view.kind(), loading, paused, settings: copy(settings), round: round ? copy(round) : null, physics: physics?.snapshot(), stats: physics?.stats(), graphics: scene?.stats(), audio: audio?.inspect(), input: input.inspect() }),
        ...(context.testing ? {
            advance(ticks, command) { if (!round) throw Error('round'); input.poll(); const controls = command || input.at(performance.now()); for (let i = 0; i < ticks && round.phase !== 'finished'; i++) stepOne(controls); view.updateHUD(round, physics.snapshot()); return copy(round); },
            stress() {
                if (!round) throw Error('round');
                physics.clearBalls(); round.actions.length = 0; round.training = true; round.multiball = true; round.launched = true; round.saverUntil = 0;
                round.active = [++round.serial, ++round.serial, ++round.serial];
                round.active.forEach((id, i) => physics.placeBall(id, [-.13 + i * .13, .04, .7 + i * .12], [i - 1, 0, 2]));
                resume();
            }
        } : {}),
        dispose() { if (disposed) return; disposed = true; generation++; loadController?.abort(); life.abort(); observer.disconnect(); cancelAnimationFrame(raf); input.dispose(); audio?.dispose(); physics?.dispose(); scene?.dispose(); view.dispose(); }
    };
}
