import { createScene } from './minigolf-scene.js';
import { createPhysics, strengthToPower, MAX_ROLL } from './minigolf-physics.js';
import { createRound, beginShot, resolveShot, advanceHole, validateSave, cloneSave, normalizeBest, recordHole, recordRound } from './minigolf-model.js';
import { AUDIO_CREDITS, createAudio } from './minigolf-audio.js';
import { createView } from './minigolf-views.js';
import { tutorialStart, tutorialNext, tutorialView } from './minigolf-tutorial.js';
import { DEFAULT_COLORS, colorValue, assignColors, cleanName, uniqueNames, worldIds, fillText } from './minigolf-shared.js';

const SAVE_KEY = 'aurago.desktop.minigolf.round.v2';
const SETTINGS_KEY = 'aurago.desktop.minigolf.settings.v1';
const BEST_KEY = 'aurago.desktop.minigolf.best.v2';
const LEGACY_KEYS = ['aurago.desktop.minigolf.round.v1', 'aurago.desktop.minigolf.best.v1'];
const QUALITIES = ['auto', 'low', 'medium', 'high'];
const MECHANIC_EVENTS = new Set(['boost', 'teleport', 'cannon', 'bumper']);
const finiteUnit = (v, fallback) => Number.isFinite(v) ? Math.max(0, Math.min(1, v)) : fallback;
const plain = value => !!value && typeof value === 'object' && !Array.isArray(value);

// One line per known field; P5 adds its own lines. saveSettings() keeps unknown stored fields.
function normalizeSettings(stored) {
    const s = plain(stored) ? stored : {};
    return {
        music: finiteUnit(s.music, .2),
        effects: finiteUnit(s.effects, .65),
        quality: QUALITIES.includes(s.quality) ? s.quality : 'auto',
        reducedMotion: s.reducedMotion === true,
        follow: s.follow !== false,
        aimCamera: s.aimCamera === true,
        magnet: s.magnet !== false,
        tutorial: Number.isInteger(s.tutorial) && s.tutorial >= 0 && s.tutorial <= 2 ? s.tutorial : s.tutorialDone === true ? 1 : 0,
        showMap: s.showMap !== false
    };
}

// v1 saves and bests are dropped without migration; true when the one-time notice is due.
function migrateLegacy() {
    try {
        if (!LEGACY_KEYS.some(key => localStorage.getItem(key) !== null)) return false;
        const current = [SAVE_KEY, BEST_KEY].some(key => localStorage.getItem(key) !== null);
        LEGACY_KEYS.forEach(key => localStorage.removeItem(key));
        return !current;
    } catch (_) { return false; }
}

export function mountGame(host, context = {}) {
    const tr = key => context.t?.('minigolf.' + key) || key;
    const assetURL = context.assetURL || (path => path);
    host.dataset.golfIcon = assetURL('/img/desktop-icons/minigolf.svg');
    const view = createView(host, tr, { assetURL, onFocusLost: () => focusStage() }), root = view.root;
    const media = matchMedia('(prefers-reduced-motion: reduce)');
    let storageWarned = false, storedSettings = {}, settings = normalizeSettings(null);
    let best = null, checkpoint = null, manifest = null, round = null, course = null, physics = null, scene = null;
    let worlds = [], newBest = false;
    // dragActive: a pointer drag is aiming; meterLeft: ms the power meter stays after the last change; sessionShots: shots this session.
    let dragActive = false, meterLeft = 0, sessionShots = 0, hintShown = false;
    let disposed = false, busy = false, loading = true, visible = true, inView = true, cameraMode = false;
    let panelKind = 'loading', track = null, tracks = [];
    // The start flow lives here: setup survives step changes and trips to help, settings and credits.
    // returnStack holds where those panels go back to (newest last, at most three), each entry with the opener to focus.
    const setup = { mode: 'full', holeId: null, players: [] };
    let menuStep = 'home', returnStack = [], tutorial = { step: null, finished: false };
    view.topbar(panelKind, false);
    let angle = 0, strength = .35, generation = 0, raf = 0, last = 0, previousMusic = .2;
    // strength is the UI value (drag, slider, keys); physics power = strengthToPower(strength).
    let previewAt = 0, previewKey = '';
    let lastSurface = 'grass', soundTime = 0, reduced = false;
    const held = { left: false, right: false, up: false, down: false, fine: false };
    const fetchController = new AbortController(), events = [], cleanup = [];
    const windowElement = root.closest('.vd-window');
    function read(key) { try { const data = localStorage.getItem(key); return data && data.length < 65536 ? JSON.parse(data) : null; } catch (_) { return null; } }
    function write(key, value) {
        try { localStorage.setItem(key, JSON.stringify(value)); return true; }
        catch (_) { if (!storageWarned) { storageWarned = true; view.hud.toast('storage_error'); } return false; }
    }
    const stored = read(SETTINGS_KEY);
    if (plain(stored)) storedSettings = stored;
    settings = normalizeSettings(storedSettings);
    view.hud.map.setVisible(settings.showMap);
    function saveSettings() {
        const next = { ...storedSettings, ...settings };
        delete next.tutorialDone;
        storedSettings = next;
        return write(SETTINGS_KEY, next);
    }
    const knownMode = mode => mode === 'full' || mode === 'practice' || worlds.includes(mode);
    const playerLabel = index => fillText(tr('player'), { n: index + 1 });
    function focusStage() { if (!disposed && panelKind === '') scene?.canvas?.focus({ preventScroll: true }); }
    function tutorialEvent(event) {
        const next = tutorialNext(tutorial, event);
        if (next === tutorial) return;
        tutorial = next;
        if (tutorial.finished && settings.tutorial < 2) { settings.tutorial = 2; saveSettings(); }
    }
    const previewOf = id => { const value = manifest?.worlds?.find?.(world => world?.id === id)?.preview; return typeof value === 'string' && /^[a-z0-9][a-z0-9._/-]*\.(webp|png|jpe?g|avif)$/i.test(value) && !value.includes('..') ? value : null; };
    function renderMenu(focus) {
        view.menu({ step: menuStep, courses: manifest.courses, worlds: worlds.map(id => ({ id, preview: previewOf(id) })), best, setup,
            saved: checkpoint && !checkpoint.completed ? { hole: checkpoint.holeIndex + 1, total: checkpoint.courseIds.length, players: checkpoint.players.length, mode: checkpoint.mode, holeId: checkpoint.courseIds[0] } : null }, focus);
    }
    function prefillSetup() {
        const source = round || checkpoint;
        if (source?.players?.length) setup.players = source.players.map(p => ({ name: p.name, color: p.color }));
        else if (!setup.players.length) setup.players = [{ name: playerLabel(0), color: DEFAULT_COLORS[0] }];
    }
    function setPlayerCount(count) {
        const players = setup.players.slice(0, count);
        while (players.length < count) players.push({ name: playerLabel(players.length), color: undefined });
        const colors = assignColors(players.map(p => p.color));
        setup.players = players.map((p, i) => ({ ...p, color: colors[i] }));
    }
    const audio = createAudio({ assetURL, onTrack(value) { track = value; view.track(value); }, onError() { if (!disposed) view.hud.toast('audio_error'); } });
    audio.setMusicVolume(settings.music); audio.setEffectsVolume(settings.effects);

    function persist() { if (round) { checkpoint = cloneSave(round); write(SAVE_KEY, checkpoint); } }
    function effectiveReduced() { return settings.reducedMotion || media.matches || document.body.dataset.animations === 'false'; }
    function updateMotion() { reduced = effectiveReduced(); scene?.setReducedMotion(reduced); view.setReduced(reduced); }
    function direction() { return [Math.sin(angle), -Math.cos(angle)]; }
    function clearHeld() { held.left = held.right = held.up = held.down = false; held.fine = false; }
    function syncAmbient() {
        const world = !loading && course && round && !round.completed && !panelKind ? course.world : null;
        audio.setAmbient(world);
        audio.setMusic(panelKind === 'menu' || !round || round.completed || !course ? 'menu' : course.world);
    }
    function snapTowardCup(dir) {
        if (!settings.magnet || !course?.cup || !round) return dir;
        const player = round.players[round.turnIndex];
        if (!player || player.done || !physics) return dir;
        const position = physics.getBall();
        if (!position) return dir;
        const wx = course.cup[0] - position[0], wz = course.cup[2] - position[2];
        const dLength = Math.hypot(dir[0], dir[1]), wLength = Math.hypot(wx, wz);
        if (!(dLength > 0) || !(wLength > .4)) return dir;
        const dx = dir[0] / dLength, dz = dir[1] / dLength, ox = wx / wLength, oz = wz / wLength;
        const delta = Math.atan2(dx * oz - dz * ox, dx * ox + dz * oz);
        const limit = 4 * Math.PI / 180, step = Math.sign(delta) * Math.min(Math.abs(delta), 3 * Math.PI / 180);
        if (Math.abs(delta) > limit) return dir;
        const cos = Math.cos(step), sin = Math.sin(step);
        return [dx * cos - dz * sin, dx * sin + dz * cos];
    }
    function allowAim() { return !!round && !round.completed && !busy && !loading && panelKind === '' && !round.players.every(p => p.done); }
    function showStrength() { view.power(strength, strengthToPower(strength) * MAX_ROLL, document.documentElement.lang); }
    function markStrength() { meterLeft = 1200; }
    function updateAim() {
        const enabled = allowAim();
        scene?.setAim(direction(), strength, enabled); showStrength();
        if (!enabled) { scene?.setPreview?.(null); previewKey = ''; }
        // A keyboard user who putts from the footer button must keep arrow keys, Space and Escape working.
        const controls = [root.querySelector('[data-action="shoot"]'), root.querySelector('[data-power]')];
        const hadFocus = !enabled && controls.includes(document.activeElement);
        for (const control of controls) control.disabled = !enabled;
        if (hadFocus) focusStage();
    }
    // Recompute at most every 66 ms while the aim changes, every 150 ms on
    // courses with moving obstacles, and never for an unchanged static aim.
    function refreshPreview() {
        if (!scene?.setPreview || !physics || cameraMode || !allowAim()) return;
        const dir = snapTowardCup(direction()), ball = physics.getBall();
        const key = [dir[0], dir[1], strength, ball[0], ball[2]].map(value => value.toFixed(3)).join(',');
        const interval = key !== previewKey ? 66 : course?.obstacles?.length ? 150 : Infinity;
        const now = performance.now();
        if (now - previewAt < interval) return;
        previewAt = now; previewKey = key;
        scene.setPreview(physics.predict(dir, strengthToPower(strength)));
    }
    function setTurn() {
        if (!round || round.completed || !course) return;
        const player = round.players[round.turnIndex];
        physics?.setBall(player.position);
        scene?.setBall(player.position, [0, 0, 0, 1], player.color);
        scene?.setGhosts(round.players, round.turnIndex);
        angle = Math.atan2(course.cup[0] - player.position[0], -(course.cup[2] - player.position[2]));
        strength = .35; cameraMode = false; scene?.setCameraMode(false); view.setCamera(false);
        view.update(round, course, busy); updateAim();
        scene?.frameTurn?.(angle, player.position);
    }
    function setPanel(kind) {
        clearHeld();
        panelKind = kind; updateAim(); syncAudio();
        if (kind) tutorialEvent({ type: 'panel' });
        root.dataset.panel = kind;
        view.topbar(kind, !!round && !round.completed);
        syncAmbient();
    }
    function syncAudio() {
        if (!visible || panelKind === 'pause' || panelKind === 'error') audio.pause();
        else audio.resume();
        if (panelKind || busy === false) audio.rolling(0, lastSurface);
    }
    function syncVisibility() {
        visible = !document.hidden && inView && root.isConnected && (!windowElement || (!windowElement.classList.contains('vd-space-hidden') && windowElement.style.display !== 'none' && !windowElement.hidden));
        if (!visible) { cancelAnimationFrame(raf); raf = 0; last = 0; }
        else if (!raf && !disposed) raf = requestAnimationFrame(frame);
        syncAudio();
    }
    function message(key) { view.hud.toast(key); }
    function holeBanner() {
        const player = round.players[round.turnIndex];
        view.hud.banner(fillText(tr('hole_par'), { hole: course.number, par: course.par }),
            [tr('course_' + course.number), round.players.length > 1 ? fillText(tr('turn_of'), { name: player.name }) : ''], colorValue(player.color), 2.2);
    }
    function turnBanner() { const player = round.players[round.turnIndex]; view.hud.banner(fillText(tr('turn_of'), { name: player.name }), [], colorValue(player.color), 1.6); }
    async function loadCourse(next, preview = false) {
        const version = ++generation; loading = true; busy = false; events.length = 0;
        physics?.dispose(); physics = null; course = next; previewKey = '';
        updateAim(); view.loading(true);
        try {
            if (!scene) scene = createScene(view.canvasHost, { assetURL, quality: settings.quality, reducedMotion: effectiveReduced(), label: tr('title'),
                onShot: shoot, onAim(dir, amount, info) { angle = Math.atan2(dir[0], -dir[1]); strength = amount; dragActive = info?.phase === 'drag'; markStrength(); showStrength(); tutorialEvent(info?.phase === 'cancel' ? { type: 'cancel' } : { type: 'drag', strength: amount }); },
                onError() { view.loading(false); setPanel('error'); view.error('webgl_error'); }, onQuality() { message('quality_changed'); }
            });
            const loaded = await scene.load(next);
            if (!loaded || disposed || version !== generation) return;
            // v2 courses read their collision meshes from the GLB; physics and the lane map share this one object.
            const playable = { ...next, colliders: loaded.colliders };
            const nextPhysics = await createPhysics(playable, { onEvent: event => events.push(event) });
            if (disposed || version !== generation) { nextPhysics.dispose(); return; }
            physics = nextPhysics; loading = false;
            view.hud.map.setCourse(playable);
            scene.setFollow?.(settings.follow); scene.setAimCamera?.(settings.aimCamera);
            syncAmbient();
            if (!preview && round) { setTurn(); holeBanner(); }
            else scene.setBall(next.tee);
            view.loading(false);
            updateAim(); syncVisibility();
        } catch (error) {
            if (disposed || version !== generation) return;
            loading = false; view.loading(false); setPanel('error'); view.error(scene ? 'load_error' : 'webgl_error');
        }
    }
    function showMenu() {
        if (busy && checkpoint && manifest) { round = cloneSave(checkpoint); busy = false; setTurn(); }
        view.hud.clearToasts();
        returnStack = []; prefillSetup(); menuStep = 'home'; setPanel('menu'); renderMenu(); view.progress(null);
    }
    async function startRound({ mode, holeId, players }) {
        const courses = mode === 'full' || mode === 'practice' ? manifest.courses : manifest.courses.filter(c => c.world === mode);
        round = createRound(courses, { players, mode, holeId });
        newBest = false; returnStack = []; tutorial = settings.tutorial < 2 ? tutorialStart() : { step: null, finished: false }; persist(); view.close(); setPanel('');
        await loadCourse(manifest.courses.find(c => c.id === round.courseIds[0]));
        focusStage();
    }
    async function start() {
        if (!manifest || loading) return;
        const names = uniqueNames(setup.players.map((p, i) => cleanName(p.name, playerLabel(i))));
        const colors = assignColors(setup.players.map(p => p.color));
        await startRound({ mode: setup.mode, holeId: setup.mode === 'practice' ? setup.holeId : undefined, players: names.map((name, i) => ({ name, color: colors[i] })) });
    }
    async function resume() {
        if (!checkpoint || !validateSave(checkpoint, manifest.courses) || checkpoint.completed) return;
        round = cloneSave(checkpoint); returnStack = []; tutorial = settings.tutorial < 2 ? tutorialStart() : { step: null, finished: false }; view.close(); setPanel('');
        await loadCourse(manifest.courses.find(c => c.id === round.courseIds[round.holeIndex]));
        if (!disposed && round.players.every(p => p.done)) showScore(); else focusStage();
    }
    function shoot(dir = direction(), amount = strength) {
        dragActive = false;
        if (!allowAim() || cameraMode || !visible || context.isActive?.() === false) return;
        dir = snapTowardCup(dir);
        if (!physics.shoot(dir, strengthToPower(amount))) return;
        if (!beginShot(round)) { physics.setBall(round.players[round.turnIndex].position); return; }
        angle = Math.atan2(dir[0], -dir[1]); strength = amount; busy = true; clearHeld();
        meterLeft = 0; sessionShots++; tutorialEvent({ type: 'shot' });
        const ballPosition = physics.getBall();
        audio.effect('strike', { speed: amount * 12, pan: scene?.panAt?.(ballPosition) ?? 0 });
        scene.burst('strike', ballPosition, amount * 2, { surface: lastSurface });
        scene.setRestMark?.(null);
        view.update(round, course, busy); updateAim();
    }
    // Mechanic events never end the shot: sound and sparks only.
    function mechanicEvent(event) {
        if (!busy) return;
        const pan = scene?.panAt?.(event.position) ?? 0;
        if (event.type === 'boost') { audio.effect('boost', { speed: event.speed, pan }); scene.burst('spark', event.position); }
        else if (event.type === 'teleport') {
            audio.effect('teleport', { pan });
            scene.burst('spark', event.position); scene.burst('spark', event.exit);
            scene.resetTrail?.();
        } else if (event.type === 'cannon') { audio.effect('cannon', { pan }); scene.burst('spark', event.position, 2); }
        else if (event.type === 'bumper') scene.burst('impact', event.position, 1, { surface: lastSurface });
    }
    function handleEvents() {
        for (const event of events.splice(0)) {
            if (MECHANIC_EVENTS.has(event.type)) { mechanicEvent(event); continue; }
            if (event.type === 'impact') {
                lastSurface = event.surface || 'grass';
                if (busy && event.speed > .25 && performance.now() - soundTime > 65) {
                    soundTime = performance.now();
                    audio.effect('impact', { speed: event.speed, surface: lastSurface, pan: scene?.panAt?.(event.position) ?? 0 });
                    scene.burst('impact', event.position, event.speed * .25, { surface: lastSurface });
                }
                continue;
            }
            const finishingPlayer = round?.players[round.turnIndex], before = round?.turnIndex;
            if (!busy || !round || !resolveShot(round, event, course)) continue;
            busy = false; audio.rolling(0, lastSurface); tutorialEvent({ type: 'resolved' });
            if (event.type === 'water' || event.type === 'out') {
                audio.effect('splash', { speed: 1, pan: scene?.panAt?.(event.position) ?? 0 });
                scene.burst('water', event.position); message(event.type);
                scene?.setRestMark?.(round.players[round.turnIndex].position);
            } else if (event.type === 'cup') {
                audio.effect('cup', { speed: 1, pan: scene?.panAt?.(event.position) ?? 0 });
                scene.burst('cup', course.cup); message('sunk');
            }
            if (event.type !== 'cup' && finishingPlayer.shots >= 12) message('limit');
            persist();
            if (round.players.every(p => p.done)) {
                if (recordHole(best, round)) { write(BEST_KEY, best); if (round.mode === 'practice') { newBest = true; message('new_best'); } }
                view.update(round, course, false);
                if (round.mode === 'practice') nextHole().catch(fail); else showScore(true);
            }
            else {
                setTurn(); if (round.players.length > 1 && round.turnIndex !== before) turnBanner();
                // focusBall cancels the turn tween setTurn just started; re-issue it (a no-op while the aim camera is off).
                if (!reduced) { scene.focusBall(); scene.frameTurn?.(angle, round.players[round.turnIndex].position); }
            }
        }
    }
    function showScore(animate = false, focus) {
        if (!round) return;
        setPanel('scorecard'); view.scorecard({ round, courses: manifest.courses, best, newBest, animate }, focus);
    }
    function fail() { if (!disposed) { setPanel('error'); view.error('load_error'); } }
    async function nextHole() {
        if (!advanceHole(round, manifest.courses)) return;
        persist();
        if (round.completed) {
            if (recordRound(best, round)) { write(BEST_KEY, best); newBest = true; message('new_best'); }
            audio.effect('win', { speed: 1 }); scene.burst('win', course.cup); showScore(true);
        } else { view.close(); setPanel(''); await loadCourse(manifest.courses.find(c => c.id === round.courseIds[round.holeIndex])); focusStage(); }
    }
    // Remembers the panel that help, settings or credits cover, and the button that opened them (a selector for the view to focus on return).
    const openerSelector = button => button?.dataset?.action ? (button.closest('.mg-topbar') ? '.mg-topbar' : '.mg-panel') + ' [data-action="' + button.dataset.action + '"]' : undefined;
    function pushReturn(kind, button) {
        if (panelKind === kind) return;
        returnStack.push({ panel: panelKind, menuStep, focus: openerSelector(button) });
        if (returnStack.length > 3) returnStack.shift();
    }
    const inRoundPanels = () => !!round && !round.completed && returnStack.some(entry => entry.panel === '' || entry.panel === 'pause');
    function reopen(target) {
        const focus = target.focus;
        if (target.panel === 'menu') { menuStep = target.menuStep; setPanel('menu'); renderMenu(focus); }
        else if (target.panel === 'pause') { setPanel('pause'); view.pause(focus); }
        else if (target.panel === 'settings') { setPanel('settings'); view.settings(settings, track, focus); }
        else if (target.panel === 'credits') { setPanel('credits'); view.credits(tracks, focus); }
        else if (target.panel === 'help') { setPanel('help'); view.help({ inRound: inRoundPanels() }, focus); }
        else if (target.panel === 'scorecard') showScore(false, focus);
        else { view.close(); setPanel(''); focusStage(); }
    }
    function closePanel() {
        if (['help', 'settings', 'credits'].includes(panelKind)) {
            if (returnStack.length) { reopen(returnStack.pop()); return; }
            if (panelKind === 'credits') { setPanel('settings'); view.settings(settings, track); return; }
        }
        if (!round || round.completed) { showMenu(); return; }
        if (round.players.every(p => p.done)) { showScore(); return; }
        view.close(); setPanel(''); focusStage();
    }
    const actions = {
        start, resume, shoot: () => shoot(), 'next-hole': nextHole,
        menu: showMenu, scorecard: () => showScore(), focus: () => scene?.focusBall(),
        pause() { if (panelKind === 'pause') closePanel(); else { setPanel('pause'); view.pause(); } },
        async again() {
            if (!round) return;
            await startRound({ mode: round.mode, holeId: round.mode === 'practice' ? round.courseIds[0] : undefined, players: round.players.map(p => ({ name: p.name, color: p.color })) });
        },
        holes() {
            if (!round) return;
            setup.players = round.players.map(p => ({ name: p.name, color: p.color })); setup.mode = 'practice'; menuStep = 'holes'; setPanel('menu'); renderMenu();
        },
        back() {
            menuStep = menuStep === 'players' && setup.mode === 'practice' ? 'holes' : 'home';
            renderMenu(menuStep === 'holes' ? `[data-hole-id="${CSS.escape(setup.holeId || '')}"]` : `[data-round="${CSS.escape(setup.mode)}"]`);
        },
        settings(button) { pushReturn('settings', button); setPanel('settings'); view.settings(settings, track); },
        help(button) { if (panelKind === 'help') return; pushReturn('help', button); setPanel('help'); view.help({ inRound: inRoundPanels() }); },
        close: closePanel,
        camera() { cameraMode = !cameraMode; scene?.setCameraMode(cameraMode); view.setCamera(cameraMode); },
        map() { settings.showMap = !settings.showMap; saveSettings(); view.hud.map.setVisible(settings.showMap); },
        'tutorial-skip': () => tutorialEvent({ type: 'skip' }),
        'tutorial-done': () => tutorialEvent({ type: 'confirm' }),
        'tutorial-again'() { tutorial = tutorialStart(); returnStack = []; reopen({ panel: '' }); },
        'next-track': () => audio.nextTrack(),
        mute() { if (settings.music > 0) { previousMusic = settings.music; settings.music = 0; } else settings.music = previousMusic || .2; audio.setMusicVolume(settings.music); saveSettings(); view.settings(settings, track, '[data-action="mute"]'); },
        credits(button) { pushReturn('credits', button); setPanel('credits'); view.credits(tracks); },
        async retry() { if (!manifest) await initialize(); else { scene?.dispose(); scene = null; view.close(); setPanel(round ? '' : 'menu'); await loadCourse(course || manifest.courses[0], !round); focusStage(); if (!round && panelKind !== 'error') showMenu(); } }
    };
    function click(event) {
        const card = event.target.closest('[data-round]'), tile = event.target.closest('[data-hole-id]');
        if (card && panelKind === 'menu') { setup.mode = card.dataset.round; if (setup.mode !== 'practice') setup.holeId = null; menuStep = setup.mode === 'practice' ? 'holes' : 'players'; renderMenu(); return; }
        if (tile && panelKind === 'menu') { setup.mode = 'practice'; setup.holeId = tile.dataset.holeId; menuStep = 'players'; renderMenu(); return; }
        const button = event.target.closest('[data-action]');
        if (button && !button.disabled && actions[button.dataset.action]) {
            audio.ui(button.dataset.action);
            Promise.resolve(actions[button.dataset.action](button)).catch(fail);
        }
    }
    function input(event) {
        const element = event.target;
        if (element.matches('[data-power]')) { strength = Number(element.value) / 100; markStrength(); updateAim(); tutorialEvent({ type: 'strength', strength }); }
        else if (element.matches('[data-music]')) { settings.music = Number(element.value) / 100; audio.setMusicVolume(settings.music); saveSettings(); view.muteLabel(settings.music === 0); }
        else if (element.matches('[data-effects]')) { settings.effects = Number(element.value) / 100; audio.setEffectsVolume(settings.effects); saveSettings(); }
        else if (element.matches('[data-player-name]')) { const player = setup.players[Number(element.closest('[data-player-index]').dataset.playerIndex)]; if (player) player.name = element.value; }
    }
    function change(event) {
        const element = event.target;
        if (element.matches('[data-player-count]')) { setPlayerCount(Number(element.value)); renderMenu('[data-player-count]:checked'); }
        if (element.matches('[data-player-color]')) { const player = setup.players[Number(element.closest('[data-player-index]').dataset.playerIndex)]; if (player) { player.color = element.value; view.syncPlayerColors(setup.players); } }
        if (element.matches('[data-quality]')) { settings.quality = element.value; scene?.setQuality(settings.quality); saveSettings(); }
        if (element.matches('[data-motion]')) { settings.reducedMotion = element.checked; updateMotion(); saveSettings(); }
        if (element.matches('[data-follow]')) { settings.follow = element.checked; scene?.setFollow?.(settings.follow); saveSettings(); }
        if (element.matches('[data-aim-camera]')) { settings.aimCamera = element.checked; scene?.setAimCamera?.(settings.aimCamera); saveSettings(); }
        if (element.matches('[data-magnet]')) { settings.magnet = element.checked; saveSettings(); }
    }
    const heldMap = { ArrowLeft: 'left', ArrowRight: 'right', ArrowUp: 'up', ArrowDown: 'down' };
    function key(event) {
        if (context.isActive?.() === false || event.ctrlKey || event.metaKey || event.altKey) return;
        if (event.type === 'keydown') scene?.skipIntro?.();
        const name = heldMap[event.key];
        if (event.key === 'Shift') { held.fine = event.type === 'keydown'; return; }
        if (name && event.type === 'keyup') { held[name] = false; return; }
        if (event.key === '?' && event.type === 'keydown') {
            if (event.target.matches('textarea,input:not([type=range]):not([type=checkbox]):not([type=radio])') || ['help', 'loading', 'error'].includes(panelKind)) return;
            event.preventDefault(); actions.help(); return;
        }
        if (event.key === 'Escape') {
            if (event.type !== 'keydown') return;
            if (scene?.cancelDrag?.()) { event.preventDefault(); return; }
            event.preventDefault();
            if (panelKind === 'menu') { if (menuStep !== 'home') actions.back(); return; }
            if (!['loading', 'error'].includes(panelKind)) panelKind ? closePanel() : actions.pause();
            return;
        }
        if (event.type !== 'keydown') return;
        const typing = event.target.closest('input,select,textarea,button');
        if (event.key === ' ') {
            if (typing || !allowAim()) return;
            event.preventDefault();
            if (!event.repeat) shoot();
            return;
        }
        if (!name || typing || !allowAim()) return;
        event.preventDefault();
        held.fine = event.shiftKey;
        if (event.repeat) return;
        held[name] = true;
        const fine = event.shiftKey;
        if (name === 'left') angle -= fine ? .008 : .035;
        if (name === 'right') angle += fine ? .008 : .035;
        if (name === 'up') strength = Math.min(1, strength + (fine ? .005 : .02));
        if (name === 'down') strength = Math.max(.02, strength - (fine ? .005 : .02));
        if (name === 'left' || name === 'right') tutorialEvent({ type: 'turn' });
        if (name === 'up' || name === 'down') { markStrength(); tutorialEvent({ type: 'strength', strength }); }
        updateAim();
    }
    function gesture() { void audio.unlock().then(syncAudio).catch(() => { if (!disposed) view.hud.toast('audio_error'); }); }
    root.addEventListener('click', click); root.addEventListener('input', input); root.addEventListener('change', change);
    root.addEventListener('keydown', key); root.addEventListener('keyup', key); window.addEventListener('blur', clearHeld);
    root.addEventListener('pointerdown', gesture); root.addEventListener('keydown', gesture);
    document.addEventListener('visibilitychange', syncVisibility); media.addEventListener('change', updateMotion);
    cleanup.push(() => { root.removeEventListener('click', click); root.removeEventListener('input', input); root.removeEventListener('change', change); root.removeEventListener('keydown', key); root.removeEventListener('keyup', key); window.removeEventListener('blur', clearHeld); root.removeEventListener('pointerdown', gesture); root.removeEventListener('keydown', gesture); document.removeEventListener('visibilitychange', syncVisibility); media.removeEventListener('change', updateMotion); });
    const visibilityObserver = new MutationObserver(syncVisibility);
    if (windowElement) visibilityObserver.observe(windowElement, { attributes: true, attributeFilter: ['style', 'class', 'hidden'] });
    const motionObserver = new MutationObserver(updateMotion); motionObserver.observe(document.body, { attributes: true, attributeFilter: ['data-animations'] });
    const intersection = new IntersectionObserver(entries => { inView = !!entries[0]?.isIntersecting; syncVisibility(); }); intersection.observe(root);
    cleanup.push(() => { visibilityObserver.disconnect(); motionObserver.disconnect(); intersection.disconnect(); });
    function frame(time) {
        raf = 0; if (disposed || !visible) return;
        const dt = last ? Math.min(.12, (time - last) / 1000) : 0; last = time;
        let snapshot = physics?.getSnapshot();
        if (physics && !loading && panelKind === '' && !round?.completed) { snapshot = physics.step(dt); handleEvents(); }
        if (scene) {
            snapshot = { ...(physics?.getSnapshot() || snapshot), obstacles: physics?.getObstacles() || [], color: round?.players[round.turnIndex]?.color || DEFAULT_COLORS[0], surface: lastSurface };
            scene.update(panelKind === 'pause' ? 0 : dt, snapshot);
        }
        if (busy && !panelKind && snapshot?.velocity) audio.rolling(Math.hypot(...snapshot.velocity), lastSurface, scene?.panAt?.(snapshot.position) ?? 0);
        if ((held.left || held.right || held.up || held.down) && !panelKind && allowAim() && dt) {
            const fine = held.fine;
            const rotate = (held.right ? 1 : 0) - (held.left ? 1 : 0);
            const lift = (held.up ? 1 : 0) - (held.down ? 1 : 0);
            if (rotate) angle += rotate * (fine ? .35 : 1.15) * dt;
            if (lift) { strength = Math.max(.02, Math.min(1, strength + lift * .55 * dt)); markStrength(); tutorialEvent({ type: 'strength', strength }); }
            if (rotate || lift) updateAim();
        }
        if (!busy && !panelKind && !loading) refreshPreview();
        if (round && course && !panelKind && snapshot && settings.showMap) {
            view.hud.map.update(round.players, round.turnIndex, round.players.map((player, index) => index === round.turnIndex ? (snapshot.position || player.position) : player.position), snapshot.obstacles || []);
        }
        meterLeft = Math.max(0, meterLeft - dt * 1000);
        const meterOn = !!scene && allowAim() && !cameraMode && (dragActive || meterLeft > 0);
        const ball = scene && (meterOn || tutorial.step) ? scene.screenPosition?.() || { visible: false } : { visible: false };
        view.hud.meter(meterOn ? ball : null);
        const hintOn = allowAim() && sessionShots < 3 && !tutorial.step;
        if (hintOn !== hintShown) { hintShown = hintOn; view.hud.hint(hintOn ? tr('aim_hint') : ''); }
        view.coach.render(tutorialView(tutorial, { panel: !!panelKind, camera: cameraMode, canAim: allowAim(), ballVisible: ball.visible }), ball);
        view.hud.tick(time);
        raf = requestAnimationFrame(frame);
    }
    async function initialize() {
        loading = true; view.loading(true);
        try {
            const response = await fetch(assetURL('/3d/minigolf/manifest.json'), { signal: fetchController.signal });
            if (!response.ok) throw Error('manifest');
            const data = await response.json();
            if (disposed) return;
            // Version 1 lists colliders in the manifest; version 2 reads them from the GLB (the asset test checks the digest).
            if (![1, 2].includes(data.version) || !Array.isArray(data.courses) || data.courses.length !== 18 || data.courses.some(c => !/^course-\d{2}\.glb$/.test(c.model) || (data.version === 2 && typeof c.collision?.digest !== 'string'))) throw Error('manifest');
            manifest = data;
            const legacyReset = migrateLegacy();
            worlds = worldIds(manifest.courses);
            best = normalizeBest(read(BEST_KEY), manifest.courses);
            const saved = read(SAVE_KEY);
            checkpoint = saved && validateSave(saved, manifest.courses) && knownMode(saved.mode) ? saved : null;
            tracks = AUDIO_CREDITS;
            await loadCourse(manifest.courses[0], true);
            if (!disposed && panelKind !== 'error') { showMenu(); if (legacyReset) message('progress_reset'); }
        } catch (_) { if (!disposed) { loading = false; view.loading(false); setPanel('error'); view.error('load_error'); } }
    }
    updateMotion(); syncVisibility(); void initialize();
    return {
        inspect() { return { round: round && cloneSave(round), course: course?.id, busy, loading, visible, panel: panelKind, menuStep: panelKind === 'menu' ? menuStep : null, tutorial: tutorial.step, physics: physics?.getSnapshot(), graphics: scene?.stats(), audio: audio.getState(), hud: view.hud.inspect() }; },
        dispose() {
            if (disposed) return; disposed = true; generation++; fetchController.abort(); cancelAnimationFrame(raf);
            for (const dispose of cleanup) dispose();
            physics?.dispose(); physics = null; scene?.dispose(); scene = null;
            audio.dispose(); visible = false; raf = 0; view.dispose();
        }
    };
}
