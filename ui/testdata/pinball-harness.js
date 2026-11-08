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
    autoplay: q.get('autoplay') === '1'
};

const stage = document.getElementById('stage');
const note = document.getElementById('note');
function showNote(text) { note.textContent = text; note.classList.add('on'); }

// ---- mode hooks (filled in by later tasks) ---------------------------------------------------------------------------
async function modeEngine(h) {
    // T12/T13 hook: import render/scene.js here and mount it on h.stage, drawing from h.renderSnapshot().
    showNote('mode=engine: renderer not available yet (T12/T13)');
    return { dispose() {} };
}
async function modeGame(h) {
    // T17 hook: import session.js / game.js here and mount the full game on h.stage.
    showNote('mode=game: game orchestration not available yet (T17)');
    return { dispose() {} };
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
    if (params.autoplay) startAutoplay(h);
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
