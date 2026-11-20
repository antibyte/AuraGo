import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const t0 = Date.now();

const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON, Date, Uint8Array };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);

// Same order as module-loader.js (the interiors modules sit between the duel and levels modules).
const MODULES = ['biomes', 'fx', 'physics', 'behaviors', 'bosses', 'variety', 'profile', 'runes', 'zones', 'zones-extra', 'gauntlets', 'swing', 'puzzles', 'dragons',
    'npcs', 'npc-story', 'npc-story-late', 'dialogue', 'trade', 'folk', 'folk-story', 'duel-patterns', 'duel', 'place-rooms', 'interior-mobs', 'interior', 'places',
    'village-rooms', 'village', 'levels', 'combat', 'sim'];
for (const m of MODULES) {
    const abs = path.join(rootDir, `ui/js/desktop/apps/runenpfad/${m}.js`);
    if (fs.existsSync(abs)) vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}

// Deep copy of a simulation, built inside the vm context so prototypes match. Shared references stay shared
// (pendulum hazards, platform rides, sky rays), functions are copied by reference and the static tile data is not copied at all.
vm.runInContext(`(function () {
    const STATIC = ['tiles', 'solidMap', 'scenery', 'world', 'biome'];
    function copy(v, memo) {
        if (v === null || typeof v !== 'object') return v;
        if (memo.has(v)) return memo.get(v);
        const out = Array.isArray(v) ? [] : Object.create(Object.getPrototypeOf(v));
        memo.set(v, out);
        const keys = Object.keys(v);
        for (let i = 0; i < keys.length; i++) out[keys[i]] = copy(v[keys[i]], memo);
        return out;
    }
    globalThis.__cloneSim = function (sim) {
        const memo = new Map();
        STATIC.forEach(function (k) { if (sim.stage[k]) memo.set(sim.stage[k], sim.stage[k]); });
        return copy(sim, memo);
    };
})();`, sandbox);

const G = sandbox.RunenpfadGauntlets;
const Levels = sandbox.RunenpfadLevels;
const Sim = sandbox.RunenpfadSim;
const Variety = sandbox.RunenpfadVariety;
assert.ok(G && Levels && Sim && Variety && sandbox.__cloneSim, 'modules loaded');
const cloneSim = sandbox.__cloneSim;
const T = 48;
const STEP = Sim.STEP;
let passed = 0;
function check(name, fn) {
    try {
        fn();
        passed += 1;
    } catch (err) {
        console.error('FAILED: ' + name);
        throw err;
    }
}

// Variant per campaign level, hard-coded so a changed assignment fails loudly.
const ASSIGN = ['thorn_hop', 'crumble_steps', 'spike_tunnel', 'pendulum_bridge', 'trap_isle', 'spike_tunnel', 'guarded_isle', 'crumble_chain', 'ferry', 'crumble_steps',
    'cannon_islands', 'jet_islands', 'guarded_isle', 'cannon_crossfire', 'trap_isle', 'trap_isle', 'ferry', 'crumble_steps', 'pendulum_bridge', 'thorn_hop',
    'jet_islands', 'conveyor_gauntlet', 'crumble_chain', 'cannon_crossfire', 'spike_tunnel'];
const HEROES = ['knight', 'ranger'];
const camp = Levels.CAMPAIGN_LEVELS;

// ---------------------------------------------------------------- A. structure
const slotOf = (cfg) => cfg.zones.findIndex((z) => z.indexOf('gauntlet:') === 0);
const flagged = (stage) => ['platforms', 'hazards', 'movingPlatforms', 'cannons', 'flameJets', 'pendulums', 'enemies', 'crates'].map((k) => (stage[k] || []).filter((o) => o.gauntlet));
const BEFORE = new Set(['lantern_moth', 'bell_toad']);
check('structure', () => {
    assert.equal(JSON.stringify(G.ZONE_IDS), '["gauntlet"]');
    assert.equal(G.VARIANTS.length, 12);
    G.VARIANTS.forEach((v) => assert.ok(G.PAR[v] > 0 && G.WIDTHS[v] >= 12 && ASSIGN.includes(v), `variant ${v} has a par bonus, a width and a level`));
    assert.equal(G.parBonusFor(['meadow', 'gauntlet:ferry', 'key_sanctum']), G.PAR.ferry);
    for (let w = 0; w < 5; w++) {
        const inWorld = camp.filter((c) => c.worldIndex === w).map((c) => ASSIGN[c.id - 1]);
        assert.equal(new Set(inWorld).size, inWorld.length, `world ${w} repeats no variant`);
    }
    for (const cfg of camp) {
        const v = ASSIGN[cfg.id - 1];
        assert.equal(cfg.zones.filter((z) => z.indexOf('gauntlet:') === 0).length, 1, `level ${cfg.id} holds one gauntlet slot`);
        assert.equal(slotOf(cfg), cfg.zones.length - 2, `level ${cfg.id} gauntlet sits before the sanctum or arena`);
        assert.equal(cfg.zones[slotOf(cfg)], 'gauntlet:' + v);
        const stage = Levels.buildStage(cfg.id);
        const gz = stage.gauntlets;
        assert.equal(gz.length, 1, `level ${cfg.id} builds exactly one gauntlet`);
        assert.equal(gz[0].x1 - gz[0].x0, G.WIDTHS[v] * T);
        assert.equal(stage.parTime, cfg.parTime + G.PAR[v], `level ${cfg.id} par includes the bonus`);
        assert.ok(stage.key && stage.door && stage.portal, `level ${cfg.id} keeps key, door and portal`);
        assert.equal(stage.collectibles.filter((c) => c.kind === 'gem').length, 3, `level ${cfg.id} keeps 3 gems`);
        assert.ok(!stage.chests.some((c) => c.x > gz[0].x0 && c.x < gz[0].x1), `level ${cfg.id} has no chest inside the gauntlet`);
        assert.equal(JSON.stringify(flagged(stage)), JSON.stringify(flagged(Levels.buildStage(cfg.id))), `level ${cfg.id} gauntlet objects are deterministic`);
        if (cfg.id === 7 || cfg.id === 13) {
            const guard = stage.enemies.find((e) => e.gauntlet);
            assert.equal(guard.defId, cfg.id === 7 ? 'goblin' : 'service_robot', `level ${cfg.id} guard kind survives the special-enemy swap`);
        }
        // Rng neutrality: everything the zones before the gauntlet drew stays identical without the slot.
        const bare = Levels.buildStage(Object.assign({}, cfg, { zones: cfg.zones.filter((z) => z.indexOf('gauntlet:') !== 0) }));
        const mod = Variety.pickMod(cfg.id, 0);
        const zoneCount = bare.enemies.length - (mod && mod.effects.extraEnemies ? mod.effects.extraEnemies : 0);
        const left = (s) => s.enemies.filter((e) => !e.gauntlet).slice(0, zoneCount).filter((e) => e.spawnX < gz[0].x0);
        const a = left(bare);
        const b = left(stage);
        assert.equal(a.length, b.length, `level ${cfg.id} keeps its enemy count left of the gauntlet`);
        a.forEach((e, i) => {
            if (BEFORE.has(e.defId) || BEFORE.has(b[i].defId)) return;
            assert.deepEqual([b[i].defId, b[i].spawnX, b[i].spawnY, b[i].shootTimer], [e.defId, e.spawnX, e.spawnY, e.shootTimer], `level ${cfg.id} enemy ${i} is unchanged`);
        });
        if (cfg.boss) assert.equal(stage.enemies.find((e) => e.def.boss).shootTimer, bare.enemies.find((e) => e.def.boss).shootTimer, `level ${cfg.id} boss rng is unchanged`);
    }
    assert.equal((Levels.buildStage(0).gauntlets || []).length, 0, 'the hub holds no gauntlet');
    for (const seed of [9001, 4242]) {
        for (let floor = 1; floor <= 15; floor++) assert.equal((Levels.buildStage({ mode: 'endless', endlessFloor: floor, seed }).gauntlets || []).length, 0, `endless floor ${floor} holds no gauntlet`);
    }
});

// Rune-trial spawns and sky rays keep out of gauntlet spans (and rays out of flame-jet columns).
check('trial spawns and sky rays avoid gauntlets', () => {
    for (const cfg of camp) {
        const stage = Levels.buildStage(cfg.id);
        const g = stage.gauntlets[0];
        stage.enemies.filter((e) => !e.gauntlet && !e.def.boss).forEach((e) => {
            assert.ok(!(e.spawnX >= g.x0 && e.spawnX < g.x1), `level ${cfg.id} enemy spawns outside the gauntlet`);
            if (e.spawnX < g.x0 + 2 * T && e.spawnX > g.x0 - 6 * T && e.maxX > e.minX) assert.ok(!(e.minX < g.x1 && e.maxX > g.x0) || e.maxX - e.minX > 12 * T, `level ${cfg.id} patrol stays out of the gauntlet`);
        });
        stage.collectibles.filter((c) => /^hb_/.test(c.id)).forEach((c) => assert.ok(!(c.x >= g.x0 && c.x < g.x1), `level ${cfg.id} bonus heart is outside the gauntlet`));
        (stage.skyRays || []).forEach((r) => {
            const a = Number.isFinite(r.startX) ? r.startX : r.x;
            const b = (Number.isFinite(r.endX) ? r.endX : r.x) + r.w;
            assert.ok(!(a < g.x1 && b > g.x0), `level ${cfg.id} sky ray avoids the gauntlet`);
            stage.flameJets.forEach((j) => assert.ok(!(a < j.x + j.w / 2 && b > j.x - j.w / 2), `level ${cfg.id} sky ray avoids flame jets`));
        });
    }
});

// An isolated stage per variant and assigned world: level id 1 means no trial, chest, sky ray or special enemy.
const PAIRS = ASSIGN.map((variant, i) => ({ id: i + 1, variant, world: camp[i].worldIndex }));
const isoCfg = (variant, world) => ({ id: 1, worldIndex: world, stageInWorld: 1, nameKey: 'x', parTime: 90, zones: ['meadow', 'gauntlet:' + variant, 'key_sanctum'], enemies: ['slime'] });
check('isolated stages', () => {
    for (const { variant, world } of PAIRS) {
        const stage = Levels.buildStage(isoCfg(variant, world));
        const gz = stage.gauntlets[0];
        const tag = `${variant}/w${world}`;
        assert.equal(gz.x0, 11 * T, tag + ' starts after the meadow');
        assert.ok(stage.key && stage.door && stage.portal, tag + ' keeps key, door and portal');
        const inside = (x0, x1) => assert.ok(x0 >= gz.x0 && x1 <= gz.x1, `${tag} object [${x0},${x1}] stays within [${gz.x0},${gz.x1}]`);
        stage.platforms.filter((p) => p.gauntlet).forEach((p) => inside(p.x, p.x + p.w));
        stage.hazards.filter((h) => h.gauntlet).forEach((h) => {
            assert.ok(['spikes', 'lava', 'spike_trap', 'pendulum'].includes(h.kind), tag + ' hazard kind ' + h.kind);
            if (h.kind === 'spike_trap') assert.ok(Number.isFinite(h.period) && Number.isFinite(h.offset), tag + ' spike trap has period and offset');
            if (h.kind !== 'pendulum') inside(h.x, h.x + h.w);
        });
        stage.movingPlatforms.filter((m) => m.gauntlet).forEach((m) => { assert.equal(m.w, 96, tag + ' mover is drawn 2 tiles wide'); inside(m.startX, m.endX + m.w); });
        stage.pendulums.filter((pd) => pd.gauntlet).forEach((pd) => inside(pd.x - Math.sin(pd.amp) * pd.len - 23, pd.x + Math.sin(pd.amp) * pd.len + 23));
        stage.cannons.filter((c) => c.gauntlet).concat(stage.flameJets.filter((j) => j.gauntlet), stage.crates.filter((c) => c.gauntlet), stage.enemies.filter((e) => e.gauntlet)).forEach((o) => inside(o.x, o.x));
    }
});

// ---------------------------------------------------------------- scripted hero
const tick = (sim, input) => Sim.stepSimulation(sim, STEP * 1.000001, input || {});
function lcg(seed) {
    let s = (seed >>> 0) || 1;
    return () => { s = (Math.imul(1664525, s) + 1013904223) >>> 0; return s / 4294967296; };
}
function prepare(sim, x) {
    sim.fx = null;
    const p = sim.player;
    p.x = x; p.y = sim.stage.groundY; p.vx = 0; p.vy = 0; p.onGround = true;
    for (let i = 0; i < 20; i++) tick(sim, {});
}
function isoSim(variant, world, hero) {
    const sim = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero, profile: Sim.createDefaultProfile() });
    sim.stage = Levels.buildStage(isoCfg(variant, world));
    sim.checkpoint = { x: sim.stage.spawn.x, y: sim.stage.spawn.y };
    prepare(sim, sim.stage.gauntlets[0].x0 + 40);
    return sim;
}
function campSim(levelId, hero) {
    const sim = Sim.createSimulation({ mode: 'campaign', levelId, hero, profile: Sim.createDefaultProfile() });
    sim.stage.enemies = sim.stage.enemies.filter((e) => e.gauntlet);
    prepare(sim, sim.stage.gauntlets[0].x0 + 40);
    return sim;
}

const decel = (sim) => (sim.stage.world.friction < 1 ? 650 : 2400);
function surfaceBelow(sim, x, yFrom) {
    const st = sim.stage;
    const tx = Math.floor(x / T);
    let best = Infinity;
    for (let ty = Math.max(0, Math.ceil(yFrom / T)); ty < 16; ty++) if (st.solidMap[tx + ',' + ty]) { best = ty * T; break; }
    st.platforms.forEach((pl) => { if (pl.solid !== false && x >= pl.x && x < pl.x + pl.w && pl.y >= yFrom) best = Math.min(best, pl.y); });
    return best;
}
// Ballistic landing prediction for one held input, matching the sim's per-tick order (velocity first, then position).
function predictLand(p, speed, input, Ls) {
    const dt = 1 / 60;
    const tv = ((input.right ? 1 : 0) - (input.left ? 1 : 0)) * speed;
    let x = p.x, y = p.y, vx = p.vx, vy = p.vy;
    for (let n = 1; n <= 200; n++) {
        const diff = tv - vx;
        vx = Math.abs(diff) <= 1400 * dt ? tv : vx + Math.sign(diff) * 1400 * dt;
        vy = Math.min(760, vy + 1480 * dt);
        x += vx * dt; y += vy * dt;
        if (vy > 0 && y >= Ls) return { x, n };
    }
    return { x, n: 200 };
}
function moverAfter(mp, n) {
    let x = mp.x, dir = mp.dir;
    for (let i = 0; i < n; i++) {
        x += mp.speed * dir / 60;
        if (x >= mp.endX) { x = mp.endX; dir = -1; } else if (x <= mp.startX) { x = mp.startX; dir = 1; }
    }
    return x;
}
// The nearest living guard that is not behind the hero.
const aliveGuard = (sim) => sim.stage.enemies.filter((e) => e.gauntlet && !e.dead && e.hp > 0 && e.x >= sim.player.x - 20).sort((a, b) => a.x - b.x)[0];

// One scripted step per call. Returns the input for this tick, or null when the step is already finished.
function stepInput(sim, r, step) {
    const p = sim.player;
    const st = r.st || (r.st = { n: 0, phase: 'wait', cd: -1, air: false });
    st.n++;
    if (st.n > 900) { r.failed = true; return null; }
    const tx = r.x0 + (step.x || 0);
    if (step.k === 'run') return p.x >= tx ? null : { right: true };
    if (step.k === 'walk' || step.k === 'rel' || step.k === 'ride') {
        const mp = step.k === 'walk' ? null : p.onPlatformRide;
        const pos = mp ? p.x - mp.x : p.x;
        const goal = step.k === 'walk' ? tx : (step.k === 'ride' ? 48 : step.x);
        if (step.k !== 'walk' && !mp) return {};
        if (Math.abs(goal - pos) <= 6 && Math.abs(p.vx) < 25 && p.onGround) return null;
        const err = goal - (pos + Math.sign(p.vx) * p.vx * p.vx / (2 * decel(sim)));
        return Math.abs(err) <= 3 ? {} : (err > 0 ? { right: true } : { left: true });
    }
    if (step.k === 'fight') {
        const e = aliveGuard(sim);
        if (!e || st.n > 240) return null;
        const ahead = e.x - p.x;
        const ok = p.attackCooldown <= 0 && (sim.heroId === 'knight' ? ahead >= 26 && ahead <= 82 : ahead >= 0 && ahead <= 260 && Math.abs(e.y - p.y) < 30);
        return ok ? { attackPressed: true } : {};
    }
    // jump
    if (st.phase === 'wait') {
        if (!p.onGround && !(p.coyoteTimer > 0)) return {};
        const go = step.t === 'now' || p.x >= r.x0 + step.t + r.jit.jx;
        if (!go) return { right: true };
        if (st.cd < 0) st.cd = r.jit.jd;
        if (st.cd-- > 0) return { right: true };
        st.phase = 'air';
        st.Ls = null;
        return { right: true, jumpPressed: true };
    }
    if (!st.air) { if (!p.onGround) st.air = true; else if (st.n > 4) st.air = true; }
    if (st.air && p.onGround) return null;
    if (st.Ls == null) st.Ls = step.target === 'mover' ? sim.stage.movingPlatforms[0].y : surfaceBelow(sim, r.x0 + step.target, p.y - 100);
    let goal = r.x0 + step.target;
    if (step.target === 'mover') {
        const mp = sim.stage.movingPlatforms[0];
        goal = moverAfter(mp, predictLand(p, sim.stats.speed, { right: true }, st.Ls).n) + 48;
    }
    if (!Number.isFinite(st.Ls)) return { right: true };
    const cands = [{ right: true }, {}, { left: true }];
    const errs = cands.map((c) => Math.abs(predictLand(p, sim.stats.speed, c, st.Ls).x - goal));
    if (errs[0] <= 6) return cands[0];
    return cands[errs.indexOf(Math.min(...errs))];
}
function tickRunner(sim, r) {
    for (;;) {
        const step = r.steps[r.i];
        if (!step) return 'done';
        const input = stepInput(sim, r, step);
        if (r.failed) return 'fail';
        if (input === null) { r.i++; r.st = null; continue; }
        tick(sim, input);
        return 'run';
    }
}
function runSteps(sim, x0, steps, jit, limit) {
    const r = { i: 0, st: null, steps, jit, x0, failed: false };
    const d0 = sim.damageTaken;
    for (let n = 0; n < limit; n++) {
        const s = tickRunner(sim, r);
        if (sim.damageTaken > d0 || sim.state !== 'playing' || sim.player.y > 760) return { ok: false, n, dmg: true };
        if (s === 'fail') return { ok: false, n };
        if (s === 'done') return { ok: true, n };
    }
    return { ok: false, n: limit };
}
// True when doing nothing (or hopping once first) for `ticks` ticks gets the hero hurt.
function idleHit(sim, ticks, hop) {
    const c = cloneSim(sim);
    const d0 = c.damageTaken;
    for (let i = 0; i < ticks; i++) { tick(c, hop && i === 0 ? { jumpPressed: true } : {}); if (c.damageTaken > d0) return true; }
    return false;
}
// One tick of waiting: stand still, or hop in place when standing would be hit and the hop would not.
function waitTick(sim) {
    const hop = sim.player.onGround && idleHit(sim, 40, false) && !idleHit(sim, 70, true);
    tick(sim, hop ? { jumpPressed: true } : {});
}
// The gate look-ahead: wait (standing, or hopping when standing is hit) until the whole step list works on a clone.
function passGate(sim, x0, steps, jit) {
    const d0 = sim.damageTaken;
    for (let waited = 0; waited <= 900; waited += 2) {
        if (runSteps(cloneSim(sim), x0, steps, jit, 900).ok) {
            const real = runSteps(sim, x0, steps, jit, 900);
            return { ok: real.ok, waited };
        }
        waitTick(sim);
        waitTick(sim);
        if (sim.damageTaken > d0) return { ok: false, waited, hurt: true };
    }
    return { ok: false, waited: 900 };
}

const W = (x) => ({ k: 'walk', x });
const R = (x) => ({ k: 'run', x });
const J = (t, target) => ({ k: 'jump', t, target });
const FIGHT = { k: 'fight' };
const RIDE = { k: 'ride' };
const REL = (x) => ({ k: 'rel', x });
const gate = (timed, steps) => ({ gate: steps, timed });
const ROUTES = {
    thorn_hop: [W(40), gate(false, [J(90, 240), J(280, 470), R(540)])],
    crumble_steps: [W(40), gate(false, [J(90, 240), J('now', 384), J('now', 560), R(600)])],
    spike_tunnel: [W(100), gate(true, [W(240)]), gate(true, [W(384)]), gate(true, [R(560)])],
    pendulum_bridge: [W(170), gate(true, [R(540)])],
    trap_isle: [W(40), gate(false, [J(90, 260), W(280)]), gate(true, [J(400, 570), R(600)])],
    guarded_isle: [W(120), gate(true, [J(140, 300), FIGHT, W(360), J(400, 570), R(620)])],
    crumble_chain: [W(40), gate(false, [J(75, 216), J('now', 312), J('now', 408), J('now', 504), J('now', 640), R(680)])],
    ferry: [W(100), gate(false, [J(140, 'mover'), RIDE]), gate(true, [REL(84), J('now', 560), R(700)])],
    cannon_islands: [W(100), gate(false, [J(140, 290)]), gate(true, [J(320, 460)]), gate(true, [J(500, 680)]), gate(false, [R(800)])],
    jet_islands: [W(40), gate(false, [J(90, 270), W(290)]), gate(true, [J(380, 540), W(560)]), gate(true, [R(720)])],
    conveyor_gauntlet: [W(120), gate(true, [J(360, 520), R(700)])]
};
ROUTES.cannon_crossfire = ROUTES.cannon_islands;

// Runs the whole route; `arrivals` collects a clone at the start of every timed gate for the fairness scan.
function runRoute(sim, variant, rnd, arrivals) {
    const x0 = sim.stage.gauntlets[0].x0;
    const x1 = sim.stage.gauntlets[0].x1;
    const out = { ok: true, ticks: 0, maxWait: 0 };
    const start = sim.stepCount;
    const hurt = [];
    for (const item of ROUTES[variant].concat([{ k: 'run', x: x1 - x0 + 30 }])) {
        const jit = { jx: Math.floor(rnd() * 25) - 12, jd: Math.floor(rnd() * 4) };
        if (item.gate) {
            if (arrivals && item.timed) arrivals.push({ steps: item.gate, state: cloneSim(sim) });
            const g = passGate(sim, x0, item.gate, jit);
            out.maxWait = Math.max(out.maxWait, g.waited);
            if (!g.ok) { out.ok = false; out.why = `gate failed after ${g.waited} ticks of waiting at x ${Math.round(sim.player.x - x0)}`; break; }
        } else {
            const r = runSteps(sim, x0, [item], jit, 900);
            if (!r.ok) { out.ok = false; out.why = 'step ' + JSON.stringify(item) + ' failed'; break; }
        }
    }
    out.ticks = sim.stepCount - start;
    out.hurt = hurt;
    if (sim.damageTaken > 0) { out.ok = false; out.why = out.why || 'damage'; }
    if (sim.state !== 'playing') out.ok = false;
    if (sim.player.x < x1 + 24) { out.ok = false; out.why = out.why || 'did not pass the gauntlet'; }
    return out;
}
function startIdle(sim, rnd) {
    const n = Math.floor(rnd() * 151);
    for (let i = 0; i < n; i++) tick(sim, {});
}
const hurtEvents = (sim) => sim.events.some((e) => e.type === 'player_hurt');

// ---------------------------------------------------------------- B. scripted solutions + C. fairness windows
const arrivalsByRun = [];
check('scripted solutions', () => {
    for (const { variant, world, id } of PAIRS) {
        for (const hero of HEROES) {
            for (let seed = 0; seed < 4; seed++) {
                const rnd = lcg(id * 7919 + seed * 104729 + (hero === 'ranger' ? 17 : 0));
                const sim = isoSim(variant, world, hero);
                startIdle(sim, rnd);
                const arrivals = seed === 0 ? [] : null;
                const res = runRoute(sim, variant, rnd, arrivals);
                assert.ok(res.ok, `${variant}/w${world}/${hero}/seed${seed}: ${res.why}`);
                assert.equal(sim.damageTaken, 0, `${variant}/${hero} takes no damage`);
                assert.ok(res.ticks < 3600 + 1800, `${variant}/${hero} finishes in time (${res.ticks})`);
                assert.ok(res.maxWait < 900, `${variant}/${hero} waits less than 900 ticks`);
                if (arrivals) arrivalsByRun.push({ variant, world, hero, arrivals });
            }
        }
    }
});

let worst = { len: Infinity };
let fairGates = 0;
check('fairness windows', () => {
    const SCAN = 360;
    for (const run of arrivalsByRun) {
        const x0 = 11 * T;
        for (const arr of run.arrivals) {
            const idle = cloneSim(arr.state);
            const d0 = idle.damageTaken;
            const safe = [];
            for (let d = 0; d < SCAN; d++) {
                safe.push(runSteps(cloneSim(idle), x0, arr.steps, { jx: 0, jd: 0 }, 900).ok);
                waitTick(idle);
                if (idle.damageTaken > d0) { while (safe.length < SCAN) safe.push(false); break; }
            }
            let best = 0, cur = 0;
            safe.forEach((s) => { cur = s ? cur + 1 : 0; best = Math.max(best, cur); });
            const fraction = safe.filter(Boolean).length / safe.length;
            const tag = `${run.variant}/w${run.world}/${run.hero}`;
            if (process.env.GAUNTLET_VERBOSE) console.log(`${tag} gate ${run.arrivals.indexOf(arr)}: longest ${best}, safe ${(fraction * 100).toFixed(0)}%`);
            assert.ok(best >= 21, `${tag}: longest safe start window is ${best} ticks (< 21)`);
            assert.ok(fraction <= 0.85, `${tag}: ${(fraction * 100).toFixed(0)}% of start times are safe (> 85%)`);
            fairGates += 1;
            if (best < worst.len) worst = { len: best, tag };
        }
    }
});

// ---------------------------------------------------------------- D. campaign integration
check('campaign integration', () => {
    for (const cfg of camp) {
        const variant = ASSIGN[cfg.id - 1];
        for (const hero of HEROES) {
            const rnd = lcg(cfg.id * 31 + (hero === 'ranger' ? 5 : 0));
            const sim = campSim(cfg.id, hero);
            startIdle(sim, rnd);
            const res = runRoute(sim, variant, rnd, null);
            assert.ok(res.ok, `level ${cfg.id} (${variant}) ${hero}: ${res.why}`);
            assert.equal(sim.damageTaken, 0, `level ${cfg.id} ${hero} takes no damage`);
            assert.ok(!hurtEvents(sim), `level ${cfg.id} ${hero} sees no hurt event`);
        }
    }
});

// ---------------------------------------------------------------- E. naive policies fail
const perVariant = {};
let naiveFails = 0, naiveRuns = 0;
check('naive policies fail', () => {
    const policies = [(p) => ({ right: true }), (p) => ({ right: true, jumpPressed: p.onGround })];
    for (const { variant, world } of PAIRS) {
        const rec = perVariant[variant] || (perVariant[variant] = { fail: 0, runs: 0 });
        for (const policy of policies) {
            for (let delay = 0; delay <= 175; delay += 25) {
                const sim = isoSim(variant, world, 'knight');
                const x0 = sim.stage.gauntlets[0].x0;
                const x1 = sim.stage.gauntlets[0].x1;
                prepare(sim, x0 + 24);
                for (let i = 0; i < delay; i++) tick(sim, {});
                let won = false;
                for (let i = 0; i < 900 && sim.damageTaken === 0; i++) {
                    tick(sim, policy(sim.player));
                    if (sim.player.x > x1) { won = true; break; }
                }
                rec.runs += 1;
                naiveRuns += 1;
                if (!won || sim.damageTaken > 0) { rec.fail += 1; naiveFails += 1; }
            }
        }
    }
    for (const v of G.VARIANTS) {
        const rec = perVariant[v];
        if (v !== 'thorn_hop') assert.ok(rec.fail / rec.runs >= 0.5, `${v}: only ${rec.fail}/${rec.runs} naive runs fail`);
    }
    assert.ok(naiveFails / naiveRuns >= 0.75, `only ${naiveFails}/${naiveRuns} naive runs fail overall`);
});

const secs = ((Date.now() - t0) / 1000).toFixed(1);
console.log(`naive failures per variant: ${G.VARIANTS.map((v) => `${v} ${perVariant[v].fail}/${perVariant[v].runs}`).join(', ')}`);
console.log(`Runenpfad gauntlet checks passed: ${passed} groups (structure, ${PAIRS.length} isolated stages, ${PAIRS.length * 2 * 4} scripted runs, ${fairGates} timed-gate windows [shortest ${worst.len} ticks ${worst.tag}], 50 campaign runs, ${naiveFails}/${naiveRuns} naive failures) in ${secs}s.`);
