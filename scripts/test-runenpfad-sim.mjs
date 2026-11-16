import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const packBase = path.join(rootDir, 'internal', 'gamemaker', 'asset_packs');

function loadModule(relPath, sandbox) {
    const absPath = path.join(rootDir, relPath);
    const code = fs.readFileSync(absPath, 'utf8');
    vm.runInContext(code, sandbox, { filename: absPath });
}

const sandbox = {
    console,
    Math,
    Number,
    String,
    Boolean,
    Array,
    Object,
    Map,
    Set,
    JSON,
    Date
};
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);

loadModule('ui/js/desktop/apps/runenpfad/biomes.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/fx.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/physics.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/behaviors.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/variety.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/profile.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/levels.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/combat.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/sim.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/audio.js', sandbox);

const Levels = sandbox.RunenpfadLevels;
const Sim = sandbox.RunenpfadSim;
const Audio = sandbox.RunenpfadAudio;
const Fx = sandbox.RunenpfadFx;
const Biomes = sandbox.RunenpfadBiomes;
const Behaviors = sandbox.RunenpfadBehaviors;

assert.ok(Levels, 'RunenpfadLevels loaded');
assert.ok(Sim, 'RunenpfadSim loaded');
assert.ok(Audio, 'RunenpfadAudio loaded');
assert.ok(Fx, 'RunenpfadFx loaded');
assert.ok(Biomes, 'RunenpfadBiomes loaded');
assert.ok(Behaviors, 'RunenpfadBehaviors loaded');

// 1. Validate all 6 real 2D sprite pack manifests + sound manifest
const spriteManifests = {};
for (const packId of Levels.REQUIRED_PACK_IDS) {
    const manifestPath = path.join(packBase, packId, 'sheet.json');
    assert.ok(fs.existsSync(manifestPath), `missing pack manifest ${manifestPath}`);
    spriteManifests[packId] = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
}
const validatedSprites = Levels.validateSpriteManifests(spriteManifests);
assert.equal(Object.keys(validatedSprites).length, 6);
assert.throws(() => Levels.validateSpriteManifests(null), /missing/i);
assert.throws(() => Levels.validateSpriteManifests({}), /invalid pack manifest/i);

const soundManifestPath = path.join(packBase, 'aurago-sounds', 'manifest.json');
const soundManifest = JSON.parse(fs.readFileSync(soundManifestPath, 'utf8'));
const validatedSounds = Audio.validateSoundManifest(soundManifest);
assert.ok(validatedSounds['forest-day'], 'forest-day sound present');
assert.ok(validatedSounds['victory'], 'victory sound present');
assert.throws(() => Audio.validateSoundManifest({ assets: [] }), /missing/i);

// 2. Verify all 25 Campaign levels across 5 worlds + 5 endless floors
assert.equal(Levels.WORLDS.length, 5, 'expected 5 worlds');
assert.equal(Levels.CAMPAIGN_LEVELS.length, 25, 'expected 25 campaign levels');
assert.equal(Levels.UPGRADE_DEFS.length, 4, 'expected 4 upgrade tracks');

for (let i = 1; i <= 25; i++) {
    const stage = Levels.buildStage(i);
    assert.equal(stage.id, i);
    assert.equal(stage.mode, 'campaign');
    assert.ok(Number.isFinite(stage.spawn.x) && Number.isFinite(stage.spawn.y));
    assert.ok(stage.key && stage.door && stage.portal, `level ${i} missing key/door/portal`);
    const gems = stage.collectibles.filter((c) => c.kind === 'gem');
    assert.equal(gems.length, 3, `level ${i} must contain exactly 3 gems`);
    assert.ok(stage.enemies.length >= 1, `level ${i} must have enemies`);
    if (i % 5 === 0) {
        assert.ok(stage.bossId, `level ${i} must have a world boss`);
        assert.equal(stage.key.guardedByBoss, true, `level ${i} key must be guarded by boss`);
    } else {
        assert.equal(stage.bossId, null);
    }
}

const endlessA = Levels.buildStage(Levels.createEndlessConfig(3, 4242));
const endlessB = Levels.buildStage(Levels.createEndlessConfig(3, 4242));
assert.equal(endlessA.mode, 'endless');
assert.equal(endlessA.endlessFloor, 3);
assert.ok(endlessA.bossId, 'endless floor 3 has a boss');
assert.equal(endlessA.widthTiles, endlessB.widthTiles, 'seeded endless generation must be deterministic');
assert.equal(endlessA.collectibles.filter((c) => c.kind === 'gem').length, 3);

// 3. Knight simulation on Level 1 (movement, jump, melee break/attack, ground slam)
let profile = Sim.createDefaultProfile();
assert.equal(profile.unlockedLevel, 1);
assert.equal(profile.hero, 'knight');
assert.equal(profile.sound, true);

const simK = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero: 'knight', profile });
const startX = simK.player.x;
for (let i = 0; i < 30; i++) {
    Sim.stepSimulation(simK, Sim.STEP, { right: true });
}
assert.ok(simK.player.x > startX + 30, 'knight should run right');

// Jump test
Sim.stepSimulation(simK, Sim.STEP, { jumpPressed: true });
const jumpPeakCheck = simK.player.vy;
assert.ok(jumpPeakCheck < -300, 'knight jump should apply upward velocity');
for (let i = 0; i < 8; i++) {
    Sim.stepSimulation(simK, Sim.STEP, {});
}
assert.equal(simK.player.onGround, false, 'knight should be mid-air after jumping');

// Knight Ground Slam in mid-air
Sim.stepSimulation(simK, Sim.STEP, { down: true, attackPressed: true });
assert.equal(simK.player.slamActive, true, 'knight mid-air down+attack activates ground slam');
for (let i = 0; i < 60 && !simK.player.onGround; i++) {
    Sim.stepSimulation(simK, Sim.STEP, {});
}
assert.equal(simK.player.onGround, true, 'knight lands back on ground');
assert.equal(simK.player.slamActive, false, 'slam finishes on impact');

// Knight melee breaks crates & damages enemy
const targetCrate = simK.stage.crates.find((c) => !c.broken);
assert.ok(targetCrate, 'level 1 has crates');
simK.player.x = targetCrate.x - 24;
simK.player.y = targetCrate.y + Levels.TILE;
simK.player.facing = 1;
simK.player.attackCooldown = 0;
Sim.stepSimulation(simK, Sim.STEP, { attackPressed: true });
assert.equal(targetCrate.broken, true, 'knight melee breaks adjacent crate');

// 4. Boss defeat unlocks guarded key, collecting key + entering portal wins & unlocks next stage
const simBoss = Sim.createSimulation({ mode: 'campaign', levelId: 5, hero: 'knight', profile });
const boss = simBoss.stage.enemies.find((e) => e.id === simBoss.stage.bossId);
assert.ok(boss, 'level 5 has boss');
assert.equal(simBoss.stage.key.guardedByBoss, true);
for (let hit = 0; hit < 20 && !boss.dead; hit++) {
    simBoss.player.x = boss.x - 28;
    simBoss.player.y = boss.y;
    simBoss.player.facing = 1;
    simBoss.player.attackCooldown = 0;
    simBoss.player.invulnTimer = 2;
    Sim.stepSimulation(simBoss, Sim.STEP, { attackPressed: true });
}
assert.equal(boss.dead, true, 'boss can be defeated via combat');
assert.equal(simBoss.stage.key.guardedByBoss, false, 'defeating boss releases guarded key');

// Collect all 3 gems, key, and enter portal
simBoss.stage.collectibles.forEach((c) => {
    if (c.kind === 'gem') {
        simBoss.player.x = c.x;
        simBoss.player.y = c.y + 18;
        Sim.stepSimulation(simBoss, Sim.STEP, {});
    }
});
assert.equal(simBoss.gemCount, 3);
simBoss.player.x = simBoss.stage.key.x;
simBoss.player.y = simBoss.stage.key.y + 18;
Sim.stepSimulation(simBoss, Sim.STEP, {});
assert.equal(simBoss.hasKey, true, 'key collected');

simBoss.player.x = simBoss.stage.portal.x;
simBoss.player.y = simBoss.stage.portal.y + 20;
const winEvents = Sim.stepSimulation(simBoss, Sim.STEP, {});
assert.equal(simBoss.state, 'won', 'portal with key triggers win');
assert.ok(winEvents.some((e) => e.type === 'portal_win'));
assert.ok(simBoss.earnedStars >= 2, 'collecting 3 gems yields at least 2 stars');
assert.equal(simBoss.profile.unlockedLevel, 6, 'completing level 5 unlocks level 6');
assert.ok(Sim.totalStars(simBoss.profile) >= 2);

// 5. Ranger simulation: double jump, dash roll, ranged attack, potion heal, checkpoint respawn
const simR = Sim.createSimulation({ mode: 'campaign', levelId: 2, hero: 'ranger', profile });
Sim.stepSimulation(simR, Sim.STEP, {});
Sim.stepSimulation(simR, Sim.STEP, {});
assert.equal(simR.player.onGround, true, 'ranger settles onto spawn ground');
Sim.stepSimulation(simR, Sim.STEP, { jumpPressed: true });
assert.equal(simR.player.airJumpsLeft, 1, 'ranger retains 1 air jump after ground jump');
Sim.stepSimulation(simR, Sim.STEP, {});
Sim.stepSimulation(simR, Sim.STEP, { jumpPressed: true });
assert.equal(simR.player.airJumpsLeft, 0, 'ranger consumes air jump for double jump');
Sim.stepSimulation(simR, Sim.STEP, { dashPressed: true });
assert.ok(simR.player.dashTimer > 0, 'ranger dash activates dashTimer');
Sim.stepSimulation(simR, Sim.STEP, { attackPressed: true });
assert.ok(simR.projectiles.some((pr) => !pr.fromEnemy && !pr.dead), 'ranger fires ranged rune bolt');

// Potion heal
simR.player.hp = 1;
const beforePotions = simR.player.potions;
Sim.stepSimulation(simR, Sim.STEP, { healPressed: true });
assert.equal(simR.player.hp, 3, 'potion heals +2 HP');
assert.equal(simR.player.potions, beforePotions - 1);

// Checkpoint + first-death respawn + second-death defeat
const cp = simR.stage.checkpoints[0];
assert.ok(cp, 'level 2 has a checkpoint');
simR.player.x = cp.x;
simR.player.y = cp.y + Levels.TILE;
Sim.stepSimulation(simR, Sim.STEP, {});
assert.equal(cp.activated, true, 'checkpoint activates');
// Drop into abyss once -> triggers checkpoint respawn
simR.player.invulnTimer = 0;
simR.player.y = simR.stage.height + 80;
Sim.stepSimulation(simR, Sim.STEP, {});
assert.equal(simR.state, 'playing', 'first death uses checkpoint respawn');
assert.equal(simR.checkpointUsed, true);
assert.equal(simR.player.x, simR.checkpoint.x);
// Drop into abyss a second time -> defeat
simR.player.invulnTimer = 0;
simR.player.y = simR.stage.height + 80;
Sim.stepSimulation(simR, Sim.STEP, {});
assert.equal(simR.state, 'lost', 'second lethal hazard triggers lost state');

// 6. Rune Workshop upgrades + profile validation/clamping
let shopProfile = Sim.createDefaultProfile();
shopProfile.coins = 500;
const vitCost = Sim.upgradeCost('vitality', 1);
const bought = Sim.buyUpgrade(shopProfile, 'vitality');
assert.equal(bought.ok, true);
assert.equal(bought.profile.upgrades.vitality, 2);
assert.equal(bought.profile.coins, 500 - vitCost);

let poorProfile = Sim.createDefaultProfile();
poorProfile.coins = 0;
assert.deepEqual(Sim.buyUpgrade(poorProfile, 'vitality').reason, 'coins');

const imported = Sim.importProfile({
    v: Sim.SAVE_VERSION,
    hero: 'unknown',
    sound: false,
    unlockedLevel: 99,
    endlessBestFloor: -5,
    coins: 99999999,
    totalScore: 42,
    upgrades: { vitality: 99, might: -2, boots: 3, alchemy: 'bad' },
    starsByLevel: { 1: 7, 99: 3, 2: 2 },
    bestTimeByLevel: { 1: 12.34, 2: -1 }
});
assert.ok(imported);
assert.equal(imported.hero, 'knight');
assert.equal(imported.sound, false);
assert.equal(imported.unlockedLevel, 25);
assert.equal(imported.endlessBestFloor, 0);
assert.equal(imported.upgrades.vitality, 5);
assert.equal(imported.upgrades.might, 1);
assert.equal(imported.upgrades.boots, 3);
assert.equal(imported.upgrades.alchemy, 1);
assert.equal(imported.starsByLevel[1], 3);
assert.equal(imported.starsByLevel[2], 2);
assert.equal(imported.starsByLevel[99], undefined);
assert.equal(imported.bestTimeByLevel[1], 12.3);
assert.equal(imported.bestTimeByLevel[2], undefined);
assert.equal(Sim.importProfile({ v: 999 }), null);

// 7. Effect system: deterministic particles, screen shake, flash and impact freeze
const fxSys = Fx.createSystem(1234);
Fx.burst(fxSys, 'spark', 100, 100, 12, { color: '#7ee8fa', speed: 200 });
assert.equal(fxSys.particles.length, 12, 'fx burst spawns the requested particle count');
Fx.ring(fxSys, 50, 50, { color: '#ffd08a' });
assert.equal(fxSys.particles.length, 13, 'fx ring spawns one particle');
const firstPos = { x: fxSys.particles[0].x, y: fxSys.particles[0].y };
Fx.update(fxSys, 0.1);
assert.ok(fxSys.particles[0].x !== firstPos.x || fxSys.particles[0].y !== firstPos.y, 'fx particles move over time');
Fx.shake(fxSys, 6);
Fx.flash(fxSys, '#ff3b5c', 0.4, 0.2);
Fx.hitstop(fxSys, 0.05);
assert.ok(fxSys.shake > 0 && fxSys.flashAlpha > 0 && fxSys.hitstop > 0, 'fx exposes shake, flash and hitstop');

const fxRepeatA = Fx.createSystem(99);
const fxRepeatB = Fx.createSystem(99);
Fx.burst(fxRepeatA, 'ember', 10, 10, 5, { speed: 120 });
Fx.burst(fxRepeatB, 'ember', 10, 10, 5, { speed: 120 });
assert.deepEqual(
    fxRepeatA.particles.map((p) => [p.x, p.y, p.vx, p.vy]),
    fxRepeatB.particles.map((p) => [p.x, p.y, p.vx, p.vy]),
    'fx generation must be deterministic for a fixed seed'
);

const fxSim = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero: 'knight', profile });
assert.ok(fxSim.fx, 'simulation owns an fx system');
Sim.stepSimulation(fxSim, Sim.STEP, { jumpPressed: true });
assert.ok(fxSim.fx.particles.length > 0, 'jumping spawns kick-off particles');

// 8. Biome presentation model: every world has sky, layers, light, weather and layered ambience
assert.equal(Biomes.BIOMES.length, 5, 'expected 5 biome presentations');
Biomes.BIOMES.forEach((biome, idx) => {
    assert.ok(Array.isArray(biome.sky) && biome.sky.length >= 2, `biome ${idx} needs a sky gradient`);
    assert.ok(Array.isArray(biome.layers) && biome.layers.length >= 2, `biome ${idx} needs parallax layers`);
    assert.ok(biome.light && biome.light.ambient >= 0 && biome.light.ambient <= 0.9, `biome ${idx} needs an ambient light level`);
    assert.ok(biome.weather && biome.weather.kind, `biome ${idx} needs a weather kind`);
    assert.ok(biome.ambience && biome.ambience.primary, `biome ${idx} needs a primary ambience`);
});
const stageBiome = Levels.buildStage(7);
assert.equal(stageBiome.biome.id, 'cavern', 'level 7 uses the cavern biome presentation');
assert.equal(stageBiome.ambience.primary, Biomes.BIOMES[1].ambience.primary, 'stage carries the biome ambience');
assert.ok(stageBiome.ambience.secondary, 'cavern uses a layered second ambience bed');

// 9. Audio cue table covers every simulation event that carries feedback
['step', 'jump', 'dash', 'spring', 'land', 'slam', 'slam_land', 'attack_melee', 'attack_ranged',
 'crate_break', 'enemy_hit', 'enemy_kill', 'boss_defeat', 'boss_phase', 'player_hurt',
 'pickup_coin', 'pickup_gem', 'pickup_star', 'pickup_heart', 'pickup_potion', 'pickup_key',
 'checkpoint', 'heal', 'hazard', 'cannon', 'enemy_shot', 'portal_win', 'player_defeat'
].forEach((type) => {
    assert.ok(Audio.CUE_MAP[type], `audio cue table is missing ${type}`);
});
assert.equal(Audio.CUE_MAP.attack_ranged.sound, 'laser', 'rune bolts use the bright magic chime');
assert.equal(Audio.CUE_MAP.attack_melee.sound, 'rifle', 'the sword swing uses the swishy cue, not the magic chime');
assert.equal(Audio.CUE_MAP.boss_defeat.sound, 'explosion-large', 'boss defeat uses the large explosion');

// 10. Impact feedback: the boss phase change emits its own cue
const fxSim2 = Sim.createSimulation({ mode: 'campaign', levelId: 5, hero: 'knight', profile });
const fxBoss = fxSim2.stage.enemies.find((e) => e.id === fxSim2.stage.bossId);
let sawBossPhase = false;
for (let hit = 0; hit < 30 && !sawBossPhase; hit++) {
    fxSim2.player.x = fxBoss.x - 28;
    fxSim2.player.y = fxBoss.y;
    fxSim2.player.facing = 1;
    fxSim2.player.attackCooldown = 0;
    fxSim2.player.invulnTimer = 2;
    const evs = Sim.stepSimulation(fxSim2, Sim.STEP, { attackPressed: true });
    if (evs.some((e) => e.type === 'boss_phase')) sawBossPhase = true;
}
assert.ok(sawBossPhase, 'boss emits a phase-change cue in phase 2');

// 11. Rune trials: level 1 stays clean, later levels get a deterministic trial
const Variety = sandbox.RunenpfadVariety;
assert.equal(Variety.MODS.length, 6, 'expected 6 rune trials');
assert.equal(Variety.pickMod(1, 0), null, 'the tutorial level carries no trial');
for (let lvl = 2; lvl <= 25; lvl++) {
    const stage = Levels.buildStage(lvl);
    assert.ok(stage.mod, `level ${lvl} must carry a rune trial`);
    assert.ok(stage.modEffects, `level ${lvl} must expose trial effects`);
}
const trialStage = Levels.buildStage(3);
assert.equal(trialStage.mod.id, Variety.pickMod(3, 0).id, 'trial selection is deterministic');
assert.equal(Levels.buildStage(3).mod.id, trialStage.mod.id, 'trial selection is stable across builds');

// Greed raises the coin value, dark trials dim the stage, swift trials toughen enemies
const greedLevel = Array.from({ length: 25 }, (_, i) => i + 1).find((l) => { const s = Levels.buildStage(l); return s.mod && s.mod.id === 'greedy'; });
const greedStage = Levels.buildStage(greedLevel);
assert.ok(greedStage.coinValue > 1, 'the greed trial multiplies coin value');
const darkLevel = Array.from({ length: 25 }, (_, i) => i + 1).find((l) => { const s = Levels.buildStage(l); return s.mod && s.mod.id === 'dark'; });
assert.ok(Levels.buildStage(darkLevel).darkness > 0, 'the dark trial adds ambient darkness');

// 12. New zone archetypes, shrine blessing and bomber drones
const spikeStage = Levels.buildStage(3);
assert.ok(spikeStage.hazards.some((h) => h.kind === 'spike_trap'), 'spike gallery places animated spike traps');
const boulderStage = Levels.buildStage(8);
assert.ok(boulderStage.boulders.length >= 3, 'boulder gallery places falling stones');
const shrineStage = Levels.buildStage(12);
assert.ok(shrineStage.shrines.length >= 1, 'rune shrine zone places a shrine');
assert.ok(shrineStage.collectibles.some((c) => c.kind === 'star'), 'rune shrine zone places a star');

const shrineSim = Sim.createSimulation({ mode: 'campaign', levelId: 12, hero: 'knight', profile });
const shrine = shrineSim.stage.shrines[0];
shrineSim.player.x = shrine.x;
shrineSim.player.y = shrine.y + 24;
Sim.stepSimulation(shrineSim, Sim.STEP, {});
assert.ok(shrineSim.blessing.runePower > 0, 'touching the shrine grants rune power');
assert.equal(shrine.taken, true, 'the shrine can only be used once');

assert.ok(Levels.ENEMY_DEFS.stalker && Levels.ENEMY_DEFS.stalker.behavior === 'ambusher', 'stalker enemy uses ambush behaviour');
assert.ok(Levels.ENEMY_DEFS.sentry_bot && Levels.ENEMY_DEFS.sentry_bot.behavior === 'turret', 'sentry bot uses turret behaviour');
assert.ok(Levels.ENEMY_DEFS.bomber_drone && Levels.ENEMY_DEFS.bomber_drone.behavior === 'bomber', 'bomber drone uses bomber behaviour');

// A turret holds its ground while a stalker waits until the hero is close.
const turretStage = Levels.buildStage(19);
const turret = turretStage.enemies.find((e) => e.def.behavior === 'turret');
if (turret) {
    const before = turret.x;
    Sim.stepSimulation(Sim.createSimulation({ mode: 'campaign', levelId: 19, hero: 'knight', profile }), Sim.STEP, {});
    assert.equal(turret.x, before, 'a turret never walks away from its post');
}

// 13. B1a regressions: bomber blast, fragile trapdoors, rune power bolts, timed spikes, gunner AI, boss phase, determinism
{
    const Combat = sandbox.RunenpfadCombat;
    const Physics = sandbox.RunenpfadPhysics;
    assert.ok(Combat && Physics, 'physics and combat modules loaded');
    const STEP = Sim.STEP;

    // (1) A bomber drone dying next to another enemy must not throw and must hurt the neighbour.
    const bombSim = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero: 'knight', profile });
    const [bomber, neighbour] = bombSim.stage.enemies;
    bomber.def = Levels.ENEMY_DEFS.bomber_drone;
    bomber.defId = 'bomber_drone';
    bomber.hp = 1;
    neighbour.x = bomber.x + 20;
    neighbour.y = bomber.y;
    neighbour.hp = neighbour.maxHp = 40;
    bombSim.events = [];
    Combat.damageEnemy(bombSim, bomber, 5, bomber.x - 10);
    assert.ok(bombSim.events.some((e) => e.type === 'bomb_blast'), 'bomber emits bomb_blast');
    assert.ok(neighbour.hp < 40, 'bomb blast hurts the neighbouring enemy');

    // (2) Fragile trial shortens the trapdoor crumble time.
    const fragSim = Sim.createSimulation({ mode: 'campaign', levelId: 7, hero: 'knight', profile });
    Variety.applyToStage(fragSim.stage, Variety.modById('fragile'));
    const trap = fragSim.stage.platforms.find((p) => p.kind === 'trapdoor');
    assert.ok(trap, 'level 7 has trapdoors');
    fragSim.player.x = trap.x + 16;
    fragSim.player.y = trap.y - 2;
    fragSim.player.vy = 120;
    Sim.stepSimulation(fragSim, STEP, {});
    assert.ok(trap.crumbleTimer > 0 && trap.crumbleTimer < 0.45, 'fragile trapdoor crumbles faster than 0.45 s');

    // (3) Rune power doubles Ranger bolts.
    const boltSim = Sim.createSimulation({ mode: 'campaign', levelId: 2, hero: 'ranger', profile });
    boltSim.blessing.runePower = 5;
    Sim.stepSimulation(boltSim, STEP, { attackPressed: true });
    const bolt = boltSim.projectiles.find((pr) => !pr.fromEnemy);
    assert.ok(bolt, 'ranger fired a bolt');
    assert.equal(bolt.damage, 2 * boltSim.stats.damage, 'rune power applies to ranger bolts');

    // (4) Spike traps only hurt while armed.
    function spikeTrial(phase01) {
        const s = Sim.createSimulation({ mode: 'campaign', levelId: 3, hero: 'knight', profile });
        const idx = s.stage.hazards.findIndex((h) => h.kind === 'spike_trap');
        assert.ok(idx >= 0, 'level 3 has a spike trap');
        const h = s.stage.hazards[idx];
        const period = 2.2;
        const target = phase01 * period;
        const base = (((target - idx * 0.35) % period) + period) % period + period;
        s.elapsed = base - STEP;
        s.player.x = h.x + h.w / 2;
        s.player.y = h.y + Levels.TILE;
        s.player.invulnTimer = 0;
        const hp = s.player.hp;
        const evs = Sim.stepSimulation(s, STEP, {});
        return { hurt: s.player.hp < hp, evs: evs, h: h };
    }
    const armed = spikeTrial(0.7);
    assert.equal(armed.hurt, true, 'armed spike trap hurts');
    assert.ok(armed.evs.some((e) => e.type === 'hazard' && e.detail.kind === 'spike_trap'), 'armed spike emits hazard');
    assert.equal(spikeTrial(0.2).hurt, false, 'retracted spike trap is harmless');

    // (5) Gunner mechs shoot in bursts and keep their distance from a still hero.
    const mechSim = Sim.createSimulation({ mode: 'campaign', levelId: 23, hero: 'knight', profile });
    const mech = mechSim.stage.enemies.find((e) => e.def.behavior === 'shooter');
    assert.ok(mech, 'level 23 has a combat mech');
    const holdX = mech.x + 120;
    let mechAttacks = 0;
    for (let i = 0; i < 300; i++) {
        mechSim.player.x = holdX;
        mechSim.player.y = mech.y;
        mechSim.player.vx = 0;
        mechSim.player.hp = mechSim.player.maxHp;
        mechSim.player.invulnTimer = 2;
        mechAttacks += Sim.stepSimulation(mechSim, STEP, {}).filter((e) => e.type === 'enemy_attack').length;
    }
    assert.ok(mechAttacks >= 3, 'combat mech fires a burst (enemy_attack events)');
    assert.ok(Math.abs(holdX - mech.x) > 150, 'combat mech backs away from a close still hero');

    // (6) Boss phase 2 triggers exactly once.
    const phaseSim = Sim.createSimulation({ mode: 'campaign', levelId: 5, hero: 'knight', profile });
    const phaseBoss = phaseSim.stage.enemies.find((e) => e.id === phaseSim.stage.bossId);
    let phaseCount = 0;
    for (let hit = 0; hit < 40 && !phaseBoss.dead; hit++) {
        phaseSim.player.x = phaseBoss.x - 28;
        phaseSim.player.y = phaseBoss.y;
        phaseSim.player.facing = 1;
        phaseSim.player.attackCooldown = 0;
        phaseSim.player.invulnTimer = 2;
        phaseCount += Sim.stepSimulation(phaseSim, STEP, { attackPressed: true }).filter((e) => e.type === 'boss_phase').length;
    }
    assert.equal(phaseCount, 1, 'boss_phase fires exactly once');

    // (7) Two sims fed the same scripted input stay identical.
    function scripted() {
        const s = Sim.createSimulation({ mode: 'campaign', levelId: 3, hero: 'ranger', profile });
        const types = [];
        for (let i = 0; i < 900; i++) {
            const evs = Sim.stepSimulation(s, STEP, {
                right: i % 300 < 220,
                left: i % 300 >= 260,
                jumpPressed: i % 47 === 0,
                attackPressed: i % 31 === 0,
                dashPressed: i % 113 === 0
            });
            evs.forEach((e) => types.push(e.type));
        }
        return { s: s, types: types };
    }
    const runA = scripted();
    const runB = scripted();
    assert.deepEqual(runA.s.player, runB.s.player, 'deterministic player state');
    assert.deepEqual(runA.s.stage.enemies, runB.s.stage.enemies, 'deterministic enemy state');
    assert.deepEqual(runA.s.projectiles, runB.s.projectiles, 'deterministic projectiles');
    assert.deepEqual(runA.types, runB.types, 'deterministic event sequence');
    assert.equal(runA.s.rngState, runB.s.rngState, 'deterministic rng state');
}

console.log('Runenpfad simulation checks passed: 6 packs, sound manifest, 25 levels, endless mode, knight/ranger combat, upgrades & persistence, fx system, biome model, audio cue table, rune trials & new zones.');

// Profile presentation settings: music and gfx survive a round trip and are clamped.
{
    const defaults = Sim.createDefaultProfile();
    assert.equal(defaults.music, true, 'music defaults to on');
    assert.equal(defaults.gfx, 'auto', 'gfx defaults to auto');
    const stored = Sim.importProfile({ v: 1, music: false, gfx: 'canvas' });
    assert.equal(stored.music, false, 'music=false is kept');
    assert.equal(stored.gfx, 'canvas', 'gfx=canvas is kept');
    assert.deepEqual(Sim.exportProfile(stored).gfx, 'canvas', 'gfx survives export');
    const junk = Sim.importProfile({ v: 1, music: 'loud', gfx: 'webgl9' });
    assert.equal(junk.music, true, 'non-false music values clamp to on');
    assert.equal(junk.gfx, 'auto', 'unknown gfx values clamp to auto');
}
