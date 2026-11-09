// AuraPinball dev harness. Query: table, mode=2d|engine|game, quality, camera, debug=1, lang, scene, seed, autoplay.
// Modes are entries of MODES; later tasks (T12 engine, T13 mechanics, T17 game) replace the hook functions below.
// Automation: window.pinballHarness (see the object built in boot()).
const BASE = '/js/desktop/apps/pinball/';
const loadErrors = [];
const tolerant = async path => {
    try { return await import(BASE + path); } catch (e) { loadErrors.push(path + ': ' + (e && e.message || e)); return null; }
};

const q = new URLSearchParams(location.search);
export const params = {
    table: q.get('table') || 'neon',
    mode: q.get('mode') || '2d',
    quality: q.get('quality') || 'low',
    camera: q.get('camera') || '',
    debug: q.get('debug') === '1',
    lang: q.get('lang') || 'en',
    scene: q.get('scene') || '',
    seed: Number(q.get('seed')) || 1,
    autoplay: q.get('autoplay') === '1' || (q.get('scene') || '').startsWith('follow')
};

const stage = document.getElementById('stage');
const note = document.getElementById('note');
function showNote(text) { note.textContent = text; note.classList.add('on'); }

// ---- mode hooks (filled in by later tasks) ---------------------------------------------------------------------------
// English strings for the harness (the app passes its own tr). Keys follow the plan's section 9.
const HARNESS_TEXT = {
    card_replay: 'REPLAY AT {score}',
    card_rules_neon: 'DROP FIREWALL · LOCK 3 BALLS\nDATA CORE STARTS MODES\nN-E-O-N ADVANCES BONUS ×',
    card_rules_leviathan: 'PEARL LIGHTS LOCK\nSHIP STARTS MODES\nSONAR LIGHTS DEEP GRAB',
    card_rules_temple: 'SUN OPENS THE IDOL\nPOPS LIGHT THE LOCK\nIDOL STARTS MODES',
    lamp_extra_ball: 'EXTRA BALL', lamp_shoot_again: 'SHOOT AGAIN', lamp_lock: 'LOCK', lamp_jackpot: 'JACKPOT', lamp_super: 'SUPER',
    lamp_kickback: 'KICKBACK', lamp_mode: 'MODE', lamp_combo: 'COMBO', lamp_wizard: 'WIZARD', lamp_spinner: 'SPINNER',
    dmd_press_start: 'PRESS START', dmd_free_play: 'FREE PLAY', dmd_high_scores: 'HIGH SCORES', dmd_replay_at: 'REPLAY AT', dmd_game_over: 'GAME OVER'
};
const harnessTr = (key, vars) => {
    const text = HARNESS_TEXT[key];
    if (text === undefined) return key;
    return text.replace(/\{(\w+)\}/g, (m, k) => (vars && vars[k] !== undefined ? vars[k] : m));
};

// Lobby preview framings (x,y,h in metres) for the `preview` scene: neon Data Core, leviathan sea monster head, temple sun idol.
const PREVIEW_SPOTS = { neon: '0,0.72,0.04', leviathan: '0.105,0.84,0.2', temple: '0,0.8,0.05' };

// T12 engine mode: render core + lamp controller (attract playlist) + DMD, physics balls from the harness world.
async function modeEngine(h) {
    const [sceneMod, lampMod, dmdMod, rasterMod] = await Promise.all([
        import(BASE + 'render/scene.js'), import(BASE + 'lights/controller.js'), import(BASE + 'dmd/core.js'), import(BASE + 'dmd/raster.js')
    ]);
    const layout = h.layout, ctrl = new AbortController();
    const spec = params.scene || '';
    const cameraName = ['follow', 'play', 'top', 'showcase', 'attract'].includes(params.camera) ? params.camera : spec === 'attract' ? 'attract' : 'play';
    const sc = await sceneMod.createScene(h.stage, { tableId: params.table, layout, settings: { quality: params.quality, camera: cameraName, reduced: false }, assetURL: p => p, signal: ctrl.signal, tr: harnessTr });
    const lamps = lampMod.createLamps(layout, { seed: params.seed });
    const dmd = dmdMod.createDmd({ raster: rasterMod.createRaster(), tr: harnessTr, locale: params.lang });
    dmd.setBase('attract', { title: layout.meta.title, scores: [{ name: 'AAA', score: 12345670 }, { name: 'BBB', score: 8200000 }, { name: 'CCC', score: 5100000 }], replayAt: 2000000 });
    const mechanical = spec !== 'attract' && spec !== 'showcase';
    if (mechanical) lamps.setModes(Uint8Array.from(layout.lamps, (_, i) => (i % 4 === 3 ? 2 : 1)));
    // `preview` = lobby card framing: the table's signature spot, no ball, lamps lit (see PREVIEW_SPOTS); `preview:x,y,h` overrides the spot.
    const previewSpot = spec === 'preview' || spec.startsWith('preview:') ? (/^preview:(-?[\d.]+)[,;](-?[\d.]+)[,;]?(-?[\d.]*)/.exec(spec) ? spec : 'preview:' + PREVIEW_SPOTS[params.table]) : '';
    const closeup = /^(?:closeup|preview):(-?[\d.]+)[,;](-?[\d.]+)[,;]?(-?[\d.]*)/.exec(previewSpot || spec);
    if (closeup) {
        sc.three.rig.closeup(Number(closeup[1]), Number(closeup[2]), Number(closeup[3]) || 0);
        if (!previewSpot && params.ball !== '0') { h.placeBall(Number(closeup[1]) + 0.012, Number(closeup[2]) - 0.006, 0, 0); h.step(2); h.clock.paused = true; }   // a resting ball for material checks
    }
    if (spec === 'play' || spec === 'action' || spec === 'top') { h.placeBall(-0.06, 0.34, 0.2, -0.3); h.placeBall(0.1, 0.52, -0.1, -0.2); }
    const inset = () => sc.setInsets({ top: Number(new URLSearchParams(location.search).get('inset') ?? (cameraName === 'top' ? 0 : 0.24)) * h.stage.clientHeight });
    inset(); addEventListener('resize', inset);
    const list = layout.meta.attract || [];
    let li = 0, lt = 0;
    const startShow = () => { if (!list.length) return; lamps.stopShows(); lamps.show(list[li][0], { duration: list[li][1] / 1000, loop: false }); };
    if (!mechanical) startShow();
    let last = performance.now(), raf = 0, disposed = false, tTotal = 0;
    const frame = now => {
        if (disposed) return;
        const dt = Math.min(0.05, (now - last) / 1000); last = now; tTotal += dt;
        const alpha = h.advance(dt), snap = h.renderSnapshot(alpha);
        if (!mechanical) { lt += dt; if (list.length && lt * 1000 >= list[li][1]) { lt = 0; li = (li + 1) % list.length; startShow(); } }
        lamps.update(dt); dmd.update(dt);
        sc.update({ snap, lamps, dmd: { frame: dmd.frame, version: dmd.version }, toyState: null, dt, time: tTotal, paused: false, reduced: false, interactive: true });
        raf = requestAnimationFrame(frame);
    };
    // Warm up the lamp shows so the first screenshot already has lit inserts.
    for (let i = 0; i < 24; i++) lamps.update(0.05);
    for (let i = 0; i < 40; i++) dmd.update(0.05);
    raf = requestAnimationFrame(frame);
    h.engine = { scene: sc, lamps, dmd };
    await new Promise(resolve => { const t0 = performance.now(); const wait = () => (sc.stats().frames >= 2 || performance.now() - t0 > 60000 ? resolve() : setTimeout(wait, 50)); wait(); });
    return { dispose() { disposed = true; cancelAnimationFrame(raf); removeEventListener('resize', inset); ctrl.abort(); sc.dispose(); } };
}
// T17 game mode: the real app (mountGame) inside the page. `autoplay=1` (or scene=play) starts a game and lets the bot flip.
async function modeGame(h) {
    const [gameMod, storageMod, botMod] = await Promise.all([import(BASE + 'game.js'), import(BASE + 'config.js'), import('/testdata/pinball-bot.js')]);
    if (!document.querySelector('link[data-pb-css]')) {
        const link = document.createElement('link');
        link.rel = 'stylesheet'; link.href = '/css/desktop-app-pinball.css'; link.dataset.pbCss = '1';
        document.head.append(link);
        await new Promise(resolve => { link.onload = link.onerror = resolve; });
    }
    const dict = await fetch('/lang/desktop/' + params.lang + '.json').then(r => r.json()).catch(() => ({}));
    document.documentElement.lang = params.lang;
    const stored = (() => { try { return JSON.parse(localStorage.getItem(storageMod.STORAGE.settings)) || {}; } catch (_) { return {}; } })();
    const camera = ['follow', 'play', 'top'].includes(params.camera) ? params.camera : stored.camera;
    localStorage.setItem(storageMod.STORAGE.settings, JSON.stringify({ ...stored, quality: params.quality, camera, cameraRev: storageMod.CAMERA_REV }));
    const game = gameMod.mountGame(h.stage, { t: key => dict[key] ?? key, assetURL: p => p, isActive: () => true, testing: true, table: params.table, seed: params.seed });
    h.game = game;
    const ready = () => { const i = game.inspect(); return i.panel === 'lobby' || i.panel === 'error' ? i : null; };
    await new Promise(resolve => { const t0 = performance.now(); const wait = () => (ready() || performance.now() - t0 > 90000 ? resolve() : setTimeout(wait, 100)); wait(); });
    if (game.inspect().panel === 'error') throw new Error('game failed to load');
    // Audio needs a user gesture: the first click or key press unlocks it.
    const unlock = () => game.internals().audio?.unlock().then(() => game.internals().audio?.resume());
    addEventListener('pointerdown', unlock, { once: true }); addEventListener('keydown', unlock, { once: true });
    if (params.autoplay || params.scene === 'play') {
        const bot = botMod.createBot(h.layout, params.seed);
        game.start(false, ['AAA']);
        game.autoplay((balls, tick) => bot.controls(balls, tick));
        // scene=follow[:seconds]: no fast-forward, the bot plays in real time so the ball cam can be shot at several moments.
        if (!params.scene.startsWith('follow')) { game.advance(480 * 14); await new Promise(resolve => setTimeout(resolve, 600)); }
    }
    return { dispose() { game.dispose(); } };
}
async function mode2d(h) {
    const mod = await import('/testdata/pinball-debug2d.js').catch(e => { loadErrors.push(String(e)); return null; });
    if (!mod) throw new Error('debug2d unavailable: ' + loadErrors.join('; '));
    return mod.mount2d(h);
}
const MODES = { '2d': mode2d, engine: modeEngine, game: modeGame };

// ---- boot --------------------------------------------------------------------------------------------------------------
async function boot() {
    const tablesMod = await tolerant('tables/index.js');
    const worldMod = await tolerant('physics/world.js');
    const snapMod = await tolerant('physics/snapshot.js');
    if (!tablesMod || !worldMod) throw new Error('core modules missing: ' + loadErrors.join('; '));
    const table = tablesMod.tableOf(params.table);
    const layout = table.layout;
    const world = worldMod.createWorld(layout, { seed: params.seed });
    const STEP = 1 / 480;
    const input = { left: false, right: false, plunger: 0 };
    const events = [];
    let prev = world.snapshot(), cur = world.snapshot(), blend = null, acc = 0, nextId = 1;
    const clock = { paused: false, scale: 1 };

    const h = {
        params, stage, table, layout, world, input, events, clock, STEP, loadErrors, tolerant, showNote, modes: MODES,
        get tick() { return cur.tick; },
        step(n = 1, patch) {
            if (patch) Object.assign(input, patch);
            for (let i = 0; i < n; i++) {
                world.step(input);
                for (const e of world.events()) { events.push(e); if (events.length > 2000) events.splice(0, 500); }
                const t = prev; prev = cur; cur = world.snapshot(t);
            }
            return cur;
        },
        setInput(patch) { Object.assign(input, patch); return input; },
        snapshot() { return world.snapshot(); },
        command(cmd) { return world.command(cmd); },
        placeBall(x, y, vx = 0, vy = 0) { return world.placeBall(nextId++ + 100, x, y, vx, vy); },
        addBall(auto = false) { world.command({ type: 'addBall', id: nextId++, auto }); },
        reset() { world.reset(); events.length = 0; prev = world.snapshot(); cur = world.snapshot(); acc = 0; },
        // Real-time driver: fixed STEP accumulator honouring clock.paused / clock.scale; returns interpolation alpha.
        advance(dtSec) {
            if (!clock.paused) {
                acc += Math.min(0.1, dtSec) * clock.scale;
                let n = 0;
                while (acc >= STEP && n < 2400) { h.step(1); acc -= STEP; n++; }
                if (n >= 2400) acc = 0;
            }
            return acc / STEP;
        },
        // Interpolated snapshot for rendering (alpha from advance()).
        renderSnapshot(alpha) {
            if (!snapMod || clock.paused) return cur;
            blend = snapMod.lerpSnapshot(prev, cur, Math.max(0, Math.min(1, alpha)), blend);
            return blend;
        }
    };
    window.pinballHarness = h;

    if (params.scene === 'play' && params.mode === '2d') h.addBall(true);
    if (params.autoplay && params.mode !== 'game') startAutoplay(h);
    const run = MODES[params.mode] || mode2d;
    h.view = await run(h);
    h.ready = true;
    document.title = 'AuraPinball harness: ' + params.table + ' ' + params.mode;
}

// Tiny bot: serves a ball with the plunger, then flips whenever a ball is low and close to a flipper.
function startAutoplay(h) {
    let phase = 0;
    h.autoplayTimer = setInterval(() => {
        const s = h.world.snapshot();
        if (!s.balls.length) { if (h.world.stats().pending === 0 && h.world.trough > 0) h.addBall(false); h.setInput({ left: false, right: false, plunger: 0 }); return; }
        const shooter = s.balls.some(b => b.mode === 'shooter');
        if (shooter) { phase++; h.setInput({ plunger: phase > 3 ? 0 : 1 }); return; }
        phase = 0;
        let left = false, right = false;
        for (const b of s.balls) if (b.y < 0.2 && b.vy < 0.2) { if (b.x < h.layout.meta.cx) left = true; else right = true; }
        h.setInput({ left, right, plunger: 0 });
    }, 120);
}

boot().catch(e => { console.error(e); showNote('harness failed: ' + (e && e.message || e)); window.pinballHarnessError = String(e && e.stack || e); });
