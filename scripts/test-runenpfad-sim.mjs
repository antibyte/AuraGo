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
loadModule('ui/js/desktop/apps/runenpfad/bosses.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/variety.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/profile.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/runes.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/zones.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/zones-extra.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/levels.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/combat.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/sim.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/audio.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/music.js', sandbox);

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
assert.notEqual(Audio.CUE_MAP.attack_melee.sound, 'rifle', 'the sword swing no longer uses the rifle sample');
assert.equal(Audio.CUE_MAP.attack_melee.synth, 'whoosh', 'the sword swing is a synthesized whoosh');
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

// B4: audio cue coverage, procedural music composition and controller smoke test
{
    const Music = sandbox.RunenpfadMusic;
    ['pickup_star', 'pickup_treasure', 'hazard', 'door_open', 'enemy_attack', 'spike_rise', 'player_fall',
     'boss_intro', 'boss_attack', 'boss_slam', 'boss_telegraph', 'combo', 'secret_break', 'lantern_lit', 'bounce'
    ].forEach((type) => assert.ok(Audio.CUE_MAP[type], `audio cue table is missing ${type}`));
    ['charge', 'pound', 'leap', 'volley', 'ring', 'laser', 'dive', 'summon', 'missiles', 'groundfire'].forEach((p) => {
        assert.ok(Audio.CUE_MAP.boss_attack.by.map[p], `boss_attack pattern ${p} needs a cue`);
    });
    assert.equal(Audio.CUE_MAP.boss_attack.by.map.missiles.sound, 'reload', 'mech missiles use the reload sample');
    assert.equal(Audio.CUE_MAP.door_open.by.map.metal.sound, 'door-metal');
    assert.deepEqual(Array.from(Audio.COIN_LADDER), [0, 2, 4, 7, 9, 12, 14, 16], 'coin chime ladder');
    assert.equal(Audio.hashUnit('jump', 3), Audio.hashUnit('jump', 3), 'cue variation is deterministic');
    assert.ok(Audio.panFor(1000, { x: 0, w: 800 }) <= 0.7 && Audio.panFor(-500, { x: 0, w: 800 }) >= -0.7, 'pan stays inside +-0.7');

    const biomeIds = ['woodland', 'cavern', 'desert', 'glacier', 'citadel'];
    biomeIds.forEach((b) => assert.ok(Music.THEMES[b] && Music.THEMES['boss_' + b], `music theme for ${b} and its boss variant`));
    assert.equal(Music.THEMES.citadel.scaleName, 'harmonicMinor');
    assert.ok(Music.THEMES.boss_woodland.bpm > Music.THEMES.woodland.bpm * 1.1, 'boss tempo is faster');
    Object.keys(Music.THEMES).forEach((id) => {
        const theme = Music.THEMES[id];
        for (let bar = 0; bar < 16; bar++) {
            Music.LAYERS.forEach((layer) => {
                const a = JSON.stringify(Music.composeBar(id, layer, bar, 4242));
                assert.equal(a, JSON.stringify(Music.composeBar(id, layer, bar, 4242)), `${id}/${layer}/${bar} is deterministic`);
                JSON.parse(a).forEach((n) => {
                    assert.ok(n.t >= 0 && n.t < 4 && n.dur > 0, 'note timing inside the bar');
                    if (n.midi !== null) {
                        const pc = (((n.midi - theme.root) % 12) + 12) % 12;
                        assert.ok(theme.scale.includes(pc), `${id}/${layer} note ${n.midi} is outside the scale`);
                    }
                });
            });
        }
    });
    assert.notEqual(JSON.stringify(Music.composeBar('woodland', 'melody', 0, 1)), JSON.stringify(Music.composeBar('woodland', 'melody', 1, 1)), 'melody phrases vary');
    assert.ok(Music.composeBar('boss_desert', 'boss', 0, 1).length === 15, 'boss themes carry stabs plus the driving ostinato');
    assert.ok(Music.THEMES.boss_desert.scale.includes(1) && Music.THEMES.boss_desert.scale.includes(3) && !Music.THEMES.boss_desert.scale.includes(4), 'boss modes are minor with a flat 2nd (no major tonic)');
    assert.equal(Music.composeBar('woodland', 'boss', 0, 1).length, 0, 'calm themes have no ostinato');

    // Intensity reads the simulation: a live enemy right next to the player raises danger.
    const musSim = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero: 'knight', profile });
    const calm = Music.intensityFor(musSim);
    assert.equal(calm.state, 'playing');
    const near = musSim.stage.enemies.find((e) => !e.dead);
    near.x = musSim.player.x + 40;
    near.y = musSim.player.y;
    assert.ok(Music.intensityFor(musSim).danger > 0.7, 'a close enemy raises danger');
    assert.equal(Music.themeIdFor(musSim, true), 'boss_woodland');

    // Controller smoke test on a recording fake Web Audio context.
    const fakeParam = () => ({ value: 0, setValueAtTime() {}, linearRampToValueAtTime() {}, exponentialRampToValueAtTime() {}, setTargetAtTime() {}, cancelScheduledValues() {} });
    const fakeNode = () => new Proxy({ connect: (n) => n, disconnect() {}, start() {}, stop() {}, getChannelData: () => new Float32Array(48000) }, {
        get(t, k) { if (!(k in t)) t[k] = fakeParam(); return t[k]; },
        set(t, k, v) { t[k] = v; return true; }
    });
    let nodeCount = 0;
    const fakeCtx = { currentTime: 0, state: 'running', sampleRate: 48000, destination: fakeNode() };
    ['createGain', 'createOscillator', 'createBiquadFilter', 'createBufferSource'].forEach((m) => { fakeCtx[m] = () => { nodeCount++; return fakeNode(); }; });
    fakeCtx.createBuffer = () => fakeNode();
    const port = { ctx: null, bus: fakeNode(), reverb: fakeNode() };
    const music = Music.createMusicController(port);
    assert.ok(music, 'controller is created before the AudioContext exists');
    assert.equal(Music.createMusicController(null), null, 'no port, no controller');
    music.update(Music.intensityFor(musSim), musSim);
    assert.equal(music.getState().theme, null, 'no context yet: nothing scheduled');
    port.ctx = fakeCtx;
    let maxVoices = 0;
    for (let i = 0; i < 600; i++) {
        fakeCtx.currentTime = i / 60;
        music.update(Music.intensityFor(musSim), musSim);
        maxVoices = Math.max(maxVoices, music.getState().voices);
    }
    assert.equal(music.getState().theme, 'woodland');
    assert.ok(nodeCount > 50, 'notes were scheduled');
    assert.ok(maxVoices > 0 && maxVoices <= Music.VOICE_CAP, 'voice cap respected');
    musSim.state = 'won';
    music.update(Music.intensityFor(musSim), musSim);
    assert.equal(music.getState().fadedOut, true, 'music fades out on win');
    music.dispose();
    music.update(Music.intensityFor(musSim), musSim);

    ['audio.js', 'music.js'].forEach((f) => {
        const src = fs.readFileSync(path.join(rootDir, 'ui/js/desktop/apps/runenpfad', f), 'utf8');
        assert.ok(!src.includes('Math.random'), `${f} must not use Math.random`);
        assert.ok(!src.includes('Date.now'), `${f} must not use Date.now`);
    });
}

// B5: boss patterns, variety zones, ropes and combo.
{
    const Zones = sandbox.RunenpfadZones;
    const Combat = sandbox.RunenpfadCombat;
    const STEP = Sim.STEP;
    const prof = Sim.createDefaultProfile();
    assert.ok(Zones && Zones.ZONE_IDS.length === 7, 'zones module exposes seven archetypes');
    const mk = (levelId, hero) => Sim.createSimulation({ mode: 'campaign', levelId, hero: hero || 'knight', profile: prof });

    // Endless floors 1-15 keep the stage invariants; every zone archetype is used by the campaign.
    for (let f = 1; f <= 15; f++) {
        const st = Levels.buildStage(Levels.createEndlessConfig(f, 9001));
        assert.ok(st.key && st.door && st.portal, `endless floor ${f} has key, door and portal`);
        assert.equal(st.collectibles.filter((c) => c.kind === 'gem').length, 3, `endless floor ${f} has 3 gems`);
        assert.equal(!!st.bossId, f % 3 === 0, `endless floor ${f} boss placement`);
    }
    const used = new Set();
    Levels.CAMPAIGN_LEVELS.forEach((l) => l.zones.forEach((z) => z.split('|').forEach((s) => used.add(s))));
    Zones.ZONE_IDS.forEach((z) => assert.ok(used.has(z), `campaign uses zone ${z}`));
    assert.deepEqual(Levels.buildStage(12).hazards.length, Levels.buildStage(12).hazards.length, 'slot variants stay fixed per level');

    // Every boss runs 900 ticks next to a passive hero, introduces itself and varies its attacks.
    [5, 10, 15, 20, 25].forEach((lvl) => {
        const s = mk(lvl);
        const b = s.stage.enemies.find((e) => e.id === s.stage.bossId);
        const patterns = new Set();
        let intro = false;
        let telegraphs = 0;
        for (let i = 0; i < 900; i++) {
            s.player.x = b.x - 140;
            s.player.y = s.stage.groundY;
            s.player.vx = 0;
            s.player.hp = s.player.maxHp;
            s.player.invulnTimer = 2;
            Sim.stepSimulation(s, STEP, {}).forEach((e) => {
                if (e.type === 'boss_attack') patterns.add(e.detail.pattern);
                if (e.type === 'boss_intro') intro = true;
                if (e.type === 'boss_telegraph') telegraphs++;
            });
            assert.ok(Number.isFinite(b.x) && Number.isFinite(b.y), `boss ${lvl} stays finite`);
        }
        assert.ok(intro, `boss ${lvl} introduces itself`);
        assert.ok(telegraphs >= 2, `boss ${lvl} telegraphs its attacks`);
        assert.ok(patterns.size >= 2, `boss ${lvl} uses at least two patterns (got ${[...patterns]})`);
    });
    // The intro banner decays and the first attack waits for the intro to settle.
    const introSim = mk(5);
    const introBoss = introSim.stage.enemies.find((e) => e.id === introSim.stage.bossId);
    introSim.player.x = introBoss.x - 200;
    introSim.player.y = introSim.stage.groundY;
    let firstAttack = -1;
    for (let i = 0; i < 400; i++) {
        introSim.player.invulnTimer = 2;
        const evs = Sim.stepSimulation(introSim, STEP, {});
        if (i === 5) assert.ok(introSim.bossIntro && introSim.bossIntro.timer > 0, 'boss intro banner is active');
        if (firstAttack < 0 && evs.some((e) => e.type === 'boss_attack')) firstAttack = i;
    }
    assert.ok(firstAttack > 60, 'the first boss attack comes more than a second after the intro');
    assert.equal(introSim.bossIntro, null, 'boss intro banner clears');

    // Defeating a boss slows time and the combo multiplier rises with chained kills.
    const kill = mk(5);
    const kb = kill.stage.enemies.find((e) => e.id === kill.stage.bossId);
    for (let h = 0; h < 40 && !kb.dead; h++) {
        kill.player.x = kb.x - 28; kill.player.y = kb.y; kill.player.facing = 1;
        kill.player.attackCooldown = 0; kill.player.invulnTimer = 2;
        Sim.stepSimulation(kill, STEP, { attackPressed: true });
    }
    assert.ok(kb.dead && kill.fx.slowmo > 0, 'boss defeat triggers slow motion');

    const cs = mk(1);
    const comboEvents = [];
    cs.stage.enemies.slice(0, 3).forEach((e) => { Combat.damageEnemy(cs, e, 99, e.x); });
    cs.events.forEach((e) => { if (e.type === 'combo') comboEvents.push(e.detail); });
    assert.equal(cs.combo.count, 3);
    assert.equal(cs.combo.mul, 1.5, 'three chained kills raise the multiplier to 1.5');
    assert.ok(comboEvents.length >= 2 && comboEvents[comboEvents.length - 1].mul === 1.5, 'combo events carry count and mul');
    for (let i = 0; i < 200; i++) Sim.stepSimulation(cs, STEP, {});
    assert.equal(cs.combo.count, 0, 'the combo decays after 2.5 s');
    assert.equal(cs.combo.mul, 1);

    // Conveyor belts carry a rider.
    const cv = mk(13);
    const belt = cv.stage.platforms.find((p) => p.kind === 'conveyor');
    assert.ok(belt, 'level 13 has a conveyor belt');
    cv.player.x = belt.x + 24; cv.player.y = belt.y - 1;
    Sim.stepSimulation(cv, STEP, {});
    Sim.stepSimulation(cv, STEP, {});
    const bx0 = cv.player.x;
    for (let i = 0; i < 30; i++) Sim.stepSimulation(cv, STEP, {});
    assert.ok((cv.player.x - bx0) * Math.sign(belt.conveyor) > 15, 'conveyor moves its rider');

    // Updraft lifts: vy is lower inside the wind column than outside.
    function windVy(withWind) {
        const s = mk(8);
        const wz = s.stage.windZones[0];
        assert.ok(wz, 'level 8 has an updraft');
        if (!withWind) s.stage.windZones = [];
        s.player.x = wz.x + wz.w / 2; s.player.y = wz.y + 120; s.player.vy = 200;
        Sim.stepSimulation(s, STEP, {});
        return s.player.vy;
    }
    assert.ok(windVy(true) < windVy(false) - 20, 'updraft reduces downward speed');

    // Cracked wall: three hits break it and clear its solid tiles.
    const sv = mk(4);
    const wall = sv.stage.secretWalls[0];
    assert.ok(wall && wall.tiles.length === 2, 'level 4 has a secret wall');
    assert.ok(wall.tiles.every((t) => sv.stage.solidMap[t.tx + ',' + t.ty]), 'wall tiles start solid');
    sv.player.x = wall.x - 30; sv.player.y = sv.stage.groundY;
    const secretEvents = [];
    for (let i = 0; i < 3; i++) {
        sv.player.facing = 1; sv.player.attackCooldown = 0;
        Sim.stepSimulation(sv, STEP, { attackPressed: true }).forEach((e) => secretEvents.push(e.type));
    }
    assert.equal(wall.broken, true, 'three hits break the secret wall');
    assert.ok(wall.tiles.every((t) => !sv.stage.solidMap[t.tx + ',' + t.ty] && t.tile.broken), 'broken wall frees its solid keys');
    assert.ok(secretEvents.includes('secret_break'), 'secret_break is emitted');
    assert.equal(sv.notice, 'runenpfad.notice.secret_found');

    // Pendulum saws hurt through the hazard list.
    const pe = mk(7);
    const pd = pe.stage.pendulums[0];
    pe.elapsed = -STEP;
    pe.player.x = pd.x; pe.player.y = pe.stage.groundY; pe.player.invulnTimer = 0;
    const peHp = pe.player.hp;
    const peEvents = Sim.stepSimulation(pe, STEP, {});
    assert.ok(pe.player.hp < peHp, 'a pendulum saw hurts');
    assert.ok(peEvents.some((e) => e.type === 'hazard' && e.detail.kind === 'pendulum'), 'pendulum emits hazard');

    // Bounce mushrooms launch higher than plain springs.
    const bs = mk(3);
    const mush = bs.stage.platforms.find((p) => p.bounce);
    assert.ok(mush, 'level 3 has a bounce mushroom');
    bs.player.x = mush.x + 24; bs.player.y = mush.y - 4; bs.player.vy = 100;
    const bEvents = [].concat(Sim.stepSimulation(bs, STEP, {}), Sim.stepSimulation(bs, STEP * 2, {}));
    assert.ok(bs.player.vy < -800, 'bounce mushroom launches the hero');
    assert.ok(bEvents.some((e) => e.type === 'bounce'), 'bounce event is emitted');

    // Lanterns light on touch and expose light sources.
    const ls = mk(4);
    const ln = ls.stage.lanterns[0];
    ls.player.x = ln.x; ls.player.y = ls.stage.groundY;
    const lEvents = Sim.stepSimulation(ls, STEP, {});
    assert.ok(ln.lit && lEvents.some((e) => e.type === 'lantern_lit'), 'touching a lantern lights it');
    assert.ok(Zones.lights(ls).some((l) => l.radius > 150), 'lit lanterns emit light');

    // Ropes: climb up, hold the climb state, jump off sideways.
    const rs = mk(9);
    const rope = rs.stage.ropes[0];
    assert.ok(rope, 'level 9 has a rope');
    rs.player.x = rope.x; rs.player.y = rope.top + 200; rs.player.vy = 0;
    const ry0 = rs.player.y;
    for (let i = 0; i < 20; i++) Sim.stepSimulation(rs, STEP, { up: true });
    assert.equal(rs.player.climbKind, 'rope', 'the hero climbs the rope');
    assert.equal(rs.player.animState, 'climb');
    assert.ok(rs.player.y < ry0 - 30, 'climbing up decreases y');
    Sim.stepSimulation(rs, STEP, { jumpPressed: true, right: true });
    assert.ok(rs.player.vy < 0 && !rs.player.onLadder, 'the hero can jump off a rope');
    Sim.stepSimulation(rs, STEP, { right: true });
    assert.equal(rs.player.climbKind, null);
    const ladderSim = mk(3);
    const lad = ladderSim.stage.ladders[0];
    ladderSim.player.x = lad.x + 24; ladderSim.player.y = lad.y + 40;
    Sim.stepSimulation(ladderSim, STEP, { up: true });
    assert.equal(ladderSim.player.climbKind, 'ladder', 'ladders report their climb kind');

    // rope_chasm is completable at base upgrades: a scripted hero hops rope to rope without touching spikes.
    // plan[i] = ticks to climb on rope i before jumping off sideways (right + jump).
    const crossChasm = (hero, plan) => {
        const cs = mk(9, hero);
        const ropes = cs.stage.ropes;
        const T = cs.stage.tileSize;
        const edge = ropes[0].x - 2 * T;
        const goalX = ropes[ropes.length - 1].x + 3 * T;
        const p = cs.player;
        p.x = edge - 30; p.y = cs.stage.groundY; p.vx = 0; p.vy = 0;
        let phase = 'run', climbed = 0, skip = null, launchVx = 0, launched = 0;
        for (let i = 0; i < 900; i++) {
            const inp = {};
            const onRope = p.onLadder && p.climbKind === 'rope';
            if (onRope) {
                if (phase !== 'rope') { phase = 'rope'; climbed = 0; }
                const idx = Math.max(0, ropes.findIndex((r) => Math.abs(p.x - r.x) <= 14));
                if (climbed < plan[idx]) { inp.up = true; climbed++; }
                else { inp.jumpPressed = true; inp.right = true; phase = 'air'; skip = ropes[idx]; launched = i; }
            } else if (phase === 'run') {
                inp.right = true;
                if (p.x >= edge - 10) { inp.jumpPressed = true; phase = 'air'; }
            } else {
                inp.right = true;
                if (skip && Math.abs(p.x - skip.x) > 30) skip = null;
                if (ropes.some((r) => r !== skip && Math.abs(p.x - r.x) <= 12)) inp.up = true;
            }
            Sim.stepSimulation(cs, STEP, inp);
            if (launched && i === launched + 12 && !launchVx) launchVx = p.vx;
            assert.equal(p.hp, p.maxHp, `${hero} must not touch spikes (tick ${i})`);
            if (p.onGround && p.x >= goalX && i > 30) return { i, launchVx };
        }
        return null;
    };
    const kRes = crossChasm('knight', [0, 0, 12]);
    assert.ok(kRes, 'the knight (no air jump) can cross rope_chasm via the ropes');
    assert.ok(kRes.launchVx > 150, 'jumping off a rope sideways gives a usable horizontal launch');
    assert.ok(crossChasm('ranger', [0, 0, 3]), 'the ranger can cross rope_chasm via the ropes');
}

// Ladders can be climbed down from the platform at their top, and climbed back up to it.
{
    const STEP = Sim.STEP, TILE = 48;
    const ls = Sim.createSimulation({ mode: "campaign", levelId: 3, hero: "knight", profile });
    const col = ls.stage.ladders.filter((l) => l.x === ls.stage.ladders[0].x);
    const top = col.reduce((a, b) => (b.y < a.y ? b : a));
    const bottom = col.reduce((a, b) => (b.y > a.y ? b : a));
    const p = ls.player;
    p.x = top.x + 24; p.y = top.y; p.vx = 0; p.vy = 0;
    Sim.stepSimulation(ls, STEP, {});
    assert.ok(p.onGround && Math.abs(p.y - top.y) < 1 && !p.onLadder, 'the hero stands on the platform atop the ladder');
    let climbedDown = false;
    for (let i = 0; i < 200 && !(p.y > bottom.y + 24); i++) {
        Sim.stepSimulation(ls, STEP, { down: true });
        if (i > 5 && p.y < bottom.y) climbedDown = climbedDown || p.climbKind === 'ladder';
        else if (i > 5) assert.equal(p.climbKind, 'ladder', `still on the ladder while descending (tick ${i})`);
    }
    assert.ok(climbedDown, 'Down on the top platform grabs the ladder');
    assert.ok(p.y > bottom.y + 24, 'the hero climbs down to the bottom of the ladder');
    for (let i = 0; i < 40; i++) Sim.stepSimulation(ls, STEP, { down: true });
    assert.ok(p.y <= bottom.y + TILE + 0.01, 'holding Down at the bottom never drops through the ground');
    for (let i = 0; i < 400 && p.y > top.y + 1; i++) Sim.stepSimulation(ls, STEP, { up: true });
    for (let i = 0; i < 30; i++) Sim.stepSimulation(ls, STEP, {});
    assert.ok(p.onGround && Math.abs(p.y - top.y) < 1.5, 'climbing up steps off onto the top platform');
    // Jump still leaves the ladder.
    Sim.stepSimulation(ls, STEP, { down: true });
    Sim.stepSimulation(ls, STEP, { down: true });
    assert.equal(p.climbKind, 'ladder');
    Sim.stepSimulation(ls, STEP, { jumpPressed: true });
    assert.ok(!p.onLadder && p.vy < 0, 'Jump leaves the ladder');
}

// Round 3: new enemies, stomp, rune power-ups, rune chests, extra zones and campaign coverage.
{
    const Combat = sandbox.RunenpfadCombat;
    const Runes = sandbox.RunenpfadRunes;
    const Zones = sandbox.RunenpfadZones;
    const ZonesExtra = sandbox.RunenpfadZonesExtra;
    const STEP = Sim.STEP;
    assert.ok(Combat && Runes && ZonesExtra, 'combat, runes and zones-extra are loaded');

    // Sim on level 1 with no enemies; the hero stands on the flat ground.
    const mk = (hero) => {
        const s = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero: hero || 'knight', profile });
        s.stage.enemies.length = 0;
        s.player.x = 200;
        s.player.y = s.stage.groundY;
        return s;
    };
    const spawn = (s, kind, x, id) => {
        const e = Levels.createEnemy(kind, x, s.stage.groundY, x - 48, x + 48, { id: id || 't_' + kind });
        s.stage.enemies.push(e);
        return e;
    };
    // Steps n ticks and returns every event seen; `each` runs before every tick.
    const run = (s, n, input, each) => {
        const events = [];
        for (let i = 0; i < n; i++) {
            if (each) each(s, i);
            Sim.stepSimulation(s, STEP, input || {}).forEach((e) => events.push(e));
        }
        return events;
    };
    const keep = (s) => { s.player.invulnTimer = 9; };
    const seen = (events, type) => events.filter((e) => e.type === type);

    // Content table: five new enemies with art ids and boss art ids.
    ['sporecap', 'crystal_bat', 'scarab', 'frost_wraith', 'clock_knight'].forEach((id) => {
        assert.ok(Levels.ENEMY_DEFS[id] && Levels.ENEMY_DEFS[id].art, `enemy ${id} exists with an art sheet`);
    });
    ['boss_boar', 'boss_goblin', 'boss_orb', 'boss_quad', 'boss_mech'].forEach((id) => {
        assert.ok(/^boss-/.test(Levels.ENEMY_DEFS[id].art) && Levels.ENEMY_DEFS[id].scale >= 1.9, `${id} has art and a larger scale`);
    });

    // Sporecap lobs a spore that bursts into a poison cloud.
    {
        const s = mk();
        spawn(s, 'sporecap', 400);
        let sporeFlew = false;
        let cloud = null;
        const ev = run(s, 240, {}, (sim) => {
            keep(sim);
            if (sim.projectiles.some((p) => p.spore && !p.dead)) sporeFlew = true;
            if (!cloud) cloud = sim.tempHazards.find((h) => h.kind === 'poison') || null;
        });
        assert.ok(sporeFlew, 'sporecap lobs a spore projectile');
        assert.ok(seen(ev, 'spore_burst').length >= 1 && cloud, 'the spore becomes a poison cloud hazard');
        assert.ok(cloud.w === 64 && cloud.h === 44 && cloud.damage === 1, 'poison cloud is 64x44 and deals 1 damage');
    }

    // Crystal bat dives on the hero from its perch, then climbs back.
    {
        const s = mk();
        const bat = spawn(s, 'crystal_bat', 400);
        let dived = false;
        let deepest = 0;
        run(s, 240, {}, (sim) => {
            sim.player.x = 400;
            keep(sim);
            if (bat.diving) dived = true;
            deepest = Math.max(deepest, bat.y);
        });
        assert.ok(dived, 'crystal bat enters its dive');
        assert.ok(deepest > s.stage.groundY - 120, 'the dive reaches the hero');
        assert.ok(bat.y < s.stage.groundY - 60, 'the bat climbs back toward its perch');
    }

    // Scarab blocks frontal hits, takes rear hits, stomps and piercing damage.
    {
        const s = mk();
        const scarab = spawn(s, 'scarab', 500);
        scarab.facing = 1;
        s.events = [];
        Combat.damageEnemy(s, scarab, 2, scarab.x + 40);
        assert.equal(scarab.hp, scarab.maxHp, 'a frontal hit is blocked');
        assert.ok(seen(s.events, 'enemy_block').length === 1 && scarab.blockFlash > 0, 'a block emits enemy_block');
        Combat.damageEnemy(s, scarab, 1, scarab.x - 40);
        assert.equal(scarab.hp, scarab.maxHp - 1, 'a rear hit lands');
        Combat.damageEnemy(s, scarab, 1, scarab.x + 40, true);
        assert.equal(scarab.hp, scarab.maxHp - 2, 'piercing damage ignores the shield');
    }

    // Frost wraith phases out: intangible, no contact damage, cannot be hit.
    {
        const s = mk();
        const w = spawn(s, 'frost_wraith', 900);
        const ev = run(s, 220, {}, (sim) => { sim.player.x = 200; keep(sim); });
        const phaseEvents = seen(ev, 'enemy_phase');
        assert.ok(phaseEvents.length >= 1 && phaseEvents[0].detail.phased === true, 'wraith emits enemy_phase when it phases out');
        w.phased = true;
        w.phaseT = 5;
        w.x = s.player.x;
        w.y = s.player.y;
        s.player.invulnTimer = 0;
        const hp = s.player.hp;
        Sim.stepSimulation(s, STEP, {});
        assert.equal(s.player.hp, hp, 'a phased wraith deals no contact damage');
        Combat.damageEnemy(s, w, 5, w.x + 30);
        assert.equal(w.hp, w.maxHp, 'a phased wraith cannot be hit');
    }

    // Clock knight winds up, then slams with a shock wave each way.
    {
        const s = mk();
        const k = spawn(s, 'clock_knight', 420);
        let wound = false;
        const ev = run(s, 150, {}, (sim) => { sim.player.x = 360; keep(sim); if (k.windup > 0) wound = true; });
        assert.ok(wound && seen(ev, 'enemy_windup').length >= 1, 'brute winds up first');
        assert.ok(seen(ev, 'enemy_slam').length >= 1, 'brute slams afterwards');
        assert.ok(s.projectiles.filter((p) => p.kind === 'shock').length >= 1, 'the slam sends shock waves');
    }

    // Stomp: damages, bounces, no contact damage, bosses once per 0.6 s, fire adds damage.
    {
        // Drops the hero on a 10 hp slime; returns the enemy, the sim, the stomp event and the bounce velocity.
        const stomp = (opts) => {
            const s = mk();
            const e = spawn(s, 'slime', 300);
            e.hp = e.maxHp = 10;
            if (opts.rune) Runes.grant(s, opts.rune, 200, 500);
            s.player.y = s.stage.groundY - 70;
            s.player.vy = 300;
            const hp = s.player.hp;
            for (let i = 0; i < 60; i++) {
                s.player.x = e.x;
                const hit = seen(Sim.stepSimulation(s, STEP, { jump: !!opts.held }), 'stomp')[0];
                if (hit) return { s, e, hit, hp, vy: s.player.vy };
            }
            return { s, e, hit: null, hp, vy: 0 };
        };
        const a = stomp({});
        assert.ok(a.hit && a.hit.detail.enemyId === a.e.id, 'falling onto an enemy emits stomp');
        assert.equal(a.e.hp, 9, 'a stomp deals 1 damage');
        assert.equal(a.s.player.hp, a.hp, 'a stomp causes no contact damage');
        assert.ok(Math.abs(a.vy + 470) < 1, 'stomp bounce is -470');
        assert.ok(Math.abs(stomp({ held: true }).vy + 600) < 1, 'held-jump stomp bounce is -600');
        assert.equal(stomp({ rune: 'fire' }).e.hp, 8, 'a stomp with the fire rune deals 2 damage');
        // Bosses take a stomp at most once per 0.6 s.
        const sb = mk();
        const bossE = Levels.createEnemy('boss_boar', 300, sb.stage.groundY, 100, 500, { id: 'bb' });
        sb.stage.enemies.push(bossE);
        const eh = 38 * bossE.def.scale;
        const drop = () => {
            sb.player.x = bossE.x;
            sb.player.y = bossE.y - eh + 4;
            sb.player.vy = 300;
            return Combat.stompEnemy(sb, bossE, eh, false);
        };
        const hp0 = bossE.hp;
        assert.ok(drop() && bossE.hp < hp0, 'bosses can be stomped');
        const hp1 = bossE.hp;
        bossE.stompCool = 0.4;
        assert.ok(drop() && bossE.hp === hp1, 'a boss takes at most one stomp per 0.6 s');
    }

    // Runes: every kind applies its effect and expires.
    {
        assert.deepEqual(JSON.parse(JSON.stringify(Runes.DURATIONS)), { fire: 15, frost: 20, storm: 15, magnet: 18 });
        // Fire: +1 melee damage.
        const f = mk();
        const slime = spawn(f, 'slime', 230);
        slime.hp = slime.maxHp = 10;
        Runes.grant(f, 'fire', f.player.x, f.player.y);
        f.player.facing = 1;
        run(f, 1, { attackPressed: true }, keep);
        assert.equal(slime.hp, 10 - (f.stats.damage + 1), 'fire rune adds 1 hit damage');
        // Frost: absorbs exactly one hit, slows nearby enemies.
        const fr = mk();
        const near = spawn(fr, 'slime', 260);
        Runes.grant(fr, 'frost', 200, 500);
        assert.equal(fr.player.rune.charges, 1);
        assert.equal(Runes.enemySlow(fr, near), 0.6, 'frost slows enemies within 130 px');
        const frHp = fr.player.hp;
        fr.events = [];
        Combat.hurtPlayer(fr, 1, 100);
        assert.equal(fr.player.hp, frHp, 'frost absorbs one hit');
        assert.ok(seen(fr.events, 'rune_shield_break').length === 1, 'frost emits rune_shield_break');
        fr.player.invulnTimer = 0;
        Combat.hurtPlayer(fr, 1, 100);
        assert.equal(fr.player.hp, frHp - 1, 'the next hit lands');
        // Storm: extra air jump, speed, kills zap the nearest other enemy.
        const st = mk('ranger');
        const a = spawn(st, 'slime', 300, 'za');
        const b = spawn(st, 'slime', 380, 'zb');
        b.hp = b.maxHp = 5;
        Runes.grant(st, 'storm', 200, 500);
        assert.equal(st.player.airJumpsLeft, 2, 'storm gives the ranger a second air jump');
        assert.equal(Runes.speedMul(st), 1.12);
        st.events = [];
        Combat.damageEnemy(st, a, 99, a.x);
        const zap = seen(st.events, 'rune_zap');
        assert.ok(zap.length === 1 && b.hp === 4, 'a storm kill zaps the nearest enemy for 1');
        ['x1', 'y1', 'x2', 'y2'].forEach((k) => assert.ok(Number.isFinite(zap[0].detail[k]), `rune_zap carries ${k}`));
        // Magnet: pulls coins in and pays one extra.
        const mg = mk();
        mg.stage.collectibles.length = 0;
        mg.stage.collectibles.push({ id: 't_coin', kind: 'coin', assetId: 'coin', animated: false, x: mg.player.x + 250, y: mg.player.y - 22, collected: false });
        Runes.grant(mg, 'magnet', 200, 500);
        const coins0 = mg.stageCoins;
        run(mg, 60, {}, keep);
        const coinItem = mg.stage.collectibles.find((c) => c.id === 't_coin');
        assert.ok(coinItem.collected, 'magnet pulls the coin to the hero');
        assert.equal(mg.stageCoins - coins0, Math.max(1, Math.round(mg.stage.coinValue || 1)) + 1, 'magnet coins are worth +1');
        // Orb pickup replaces the active rune, runes expire.
        const ob = mk();
        Runes.grant(ob, 'magnet', 200, 500);
        Runes.orb(ob, 'fire', ob.player.x, ob.player.y - 22);
        const pick = run(ob, 2, {}, keep);
        assert.ok(seen(pick, 'rune_pickup').length === 1 && ob.player.rune.kind === 'fire' && ob.player.rune.duration === 15, 'a rune orb replaces the active rune');
        ob.player.rune.timeLeft = 0.03;
        const exp = run(ob, 4, {}, keep);
        assert.ok(ob.player.rune === null && seen(exp, 'rune_expire')[0].detail.kind === 'fire', 'runes expire with rune_expire');
    }

    // Rune chests: touching opens, an orb appears; hits open them too.
    {
        const s = Sim.createSimulation({ mode: 'campaign', levelId: 3, hero: 'knight', profile });
        s.stage.enemies.length = 0;
        const chest = s.stage.chests[0];
        assert.ok(chest && !chest.opened && ['fire', 'frost', 'storm', 'magnet'].includes(chest.kind), 'level 3 holds a rune chest');
        s.player.x = chest.x;
        s.player.y = chest.y;
        const ev = run(s, 2, {}, keep);
        const open = seen(ev, 'chest_open');
        assert.ok(chest.opened && open.length === 1 && open[0].detail.kind === chest.kind, 'touching a chest opens it');
        const orbItem = s.stage.collectibles.find((c) => c.type === 'rune_orb');
        assert.ok(orbItem && orbItem.kind === chest.kind && orbItem.y < chest.y, 'the chest spawns a rune orb above it');
        const s2 = Sim.createSimulation({ mode: 'campaign', levelId: 3, hero: 'knight', profile });
        const c2 = s2.stage.chests[0];
        Zones.hitRect(s2, c2.x - 5, c2.y - 20, 10, 10, 1);
        assert.ok(c2.opened, 'a hit opens a chest');
    }

    // Extra zones build and their steps emit events.
    {
        const zoneSim = (zone, worldIndex) => {
            const s = mk();
            s.stage = Levels.buildStage({ id: 99, worldIndex: worldIndex, stageInWorld: 1, nameKey: 'x', parTime: 90, zones: ['meadow', zone, 'key_sanctum'], enemies: ['slime'] });
            s.stage.enemies.length = 0;
            s.player.x = s.stage.spawn.x;
            s.player.y = s.stage.spawn.y;
            return s;
        };
        // Geyser field launches the hero.
        const g = zoneSim('geyser_field', 2);
        const gy = g.stage.geysers[0];
        assert.ok(g.stage.geysers.length >= 1 && gy.w > 0 && gy.period > 0, 'geyser_field builds geysers');
        g.player.x = gy.x + gy.w / 2;
        g.player.y = g.stage.groundY;
        let topY = g.player.y;
        const gev = run(g, 200, {}, (sim) => { topY = Math.min(topY, sim.player.y); });
        assert.ok(seen(gev, 'geyser_burst').length >= 1, 'geysers announce eruptions');
        assert.ok(topY < g.stage.groundY - 200, 'an eruption launches the hero');
        // Rune gate: three switches open the gate and uncover the chest.
        const r = zoneSim('rune_gate', 1);
        const gate = r.stage.gates[0];
        assert.equal(r.stage.switches.length, 3, 'rune_gate has three switches');
        assert.ok(r.stage.switches.some((sw) => sw.y < r.stage.groundY - 3 * Levels.TILE), 'one switch sits up high');
        assert.ok(r.stage.chests.length === 1 && r.stage.chests[0].x > gate.x, 'a rune chest waits behind the gate');
        const gateKey = gate.tiles[0].tx + ',' + gate.tiles[0].ty;
        assert.ok(!gate.open && r.stage.solidMap[gateKey], 'the gate starts solid');
        r.events = [];
        r.stage.switches.forEach((sw) => { Zones.hitRect(r, sw.x - 5, sw.y - 5, 10, 10, 1); });
        assert.equal(seen(r.events, 'switch_on').length, 3, 'each switch emits switch_on');
        assert.ok(seen(r.events, 'gate_open').length === 1 && gate.open && !r.stage.solidMap[gateKey], 'all switches open the gate');
        // Flame jets warn, fire and burn.
        const fj = zoneSim('flame_jets', 2);
        const jet = fj.stage.flameJets[0];
        assert.ok(fj.stage.flameJets.length >= 2 && ['idle', 'warn', 'fire'].includes(jet.state), 'flame_jets builds jets');
        fj.player.x = jet.x - 140;
        fj.player.y = fj.stage.groundY;
        const fev = run(fj, 420, {}, (sim) => { keep(sim); sim.player.x = jet.x - 140; });
        assert.ok(seen(fev, 'jet_warn').length >= 1 && seen(fev, 'jet_fire').length >= 1, 'jets emit jet_warn and jet_fire');
        const fj2 = zoneSim('flame_jets', 4);
        const j2 = fj2.stage.flameJets[0];
        const hp0 = fj2.player.hp;
        run(fj2, 300, {}, (sim) => { sim.player.x = j2.x; sim.player.y = sim.stage.groundY; sim.player.invulnTimer = 0; });
        assert.ok(fj2.player.hp < hp0, 'standing in a flame jet burns the hero');
    }

    // Campaign: every level builds and steps; chests from level 3; every world has a new mechanic and a new enemy.
    {
        const newKinds = ['sporecap', 'crystal_bat', 'scarab', 'frost_wraith', 'clock_knight'];
        const seenKinds = new Set();
        for (let i = 1; i <= 25; i++) {
            const s = Sim.createSimulation({ mode: 'campaign', levelId: i, hero: 'knight', profile });
            if (i >= 3) assert.ok(s.stage.chests.length >= 1, `level ${i} holds a rune chest`);
            s.stage.enemies.forEach((e) => seenKinds.add(e.defId));
            run(s, 5, {});
            const boss = s.stage.enemies.find((e) => e.id === s.stage.bossId);
            if (i % 5 === 0) {
                assert.ok(boss && boss.x >= boss.minX && boss.x <= boss.maxX, `level ${i} arena contains its boss`);
                assert.ok(boss.y - 42 * boss.def.scale > 0, `level ${i} boss fits under the ceiling`);
            }
        }
        newKinds.forEach((k) => assert.ok(seenKinds.has(k), `${k} appears in the campaign`));
        for (let w = 0; w < 5; w++) {
            const lvls = Levels.CAMPAIGN_LEVELS.filter((l) => l.worldIndex === w);
            assert.ok(lvls.some((l) => l.zones.some((z) => ZonesExtra.has(z))), `world ${w + 1} has a new mechanic`);
            assert.ok(lvls.some((l) => l.enemies.some((k) => newKinds.includes(k))), `world ${w + 1} has a new enemy`);
        }
        assert.ok(!Levels.CAMPAIGN_LEVELS[0].enemies.includes('sporecap') && !Levels.CAMPAIGN_LEVELS[1].enemies.includes('sporecap'), 'sporecap stays out of the first two levels');
        for (const seed of [9001, 4242]) {
            for (let f = 1; f <= 12; f++) {
                const s = Sim.createSimulation({ mode: 'endless', endlessFloor: f, seed: seed, hero: 'ranger', profile });
                assert.ok(s.stage.chests.length >= 1, `endless floor ${f} holds a rune chest`);
                run(s, 3, {});
            }
        }
    }

    // fire_strip announces itself with boss_attack.
    {
        const s = Sim.createSimulation({ mode: 'campaign', levelId: 25, hero: 'knight', profile });
        const boss = s.stage.enemies.find((e) => e.id === s.stage.bossId);
        const patterns = new Set();
        run(s, 3600, {}, (sim) => { sim.player.x = boss.x - 320; sim.player.y = sim.stage.groundY; keep(sim); }).forEach((e) => {
            if (e.type === 'boss_attack') patterns.add(e.detail.pattern);
        });
        assert.ok(patterns.has('fire_strip'), 'fire_strip emits boss_attack with pattern fire_strip');
    }
}

console.log('Runenpfad round 3 checks passed: new enemies, stomp, runes, chests, extra zones, campaign coverage.');

// Round 3 review fixes: wraith hover lane, per-gate switches, storm expiry.
{
    const Combat = sandbox.RunenpfadCombat;
    const Runes = sandbox.RunenpfadRunes;
    const Zones = sandbox.RunenpfadZones;
    const STEP = Sim.STEP;
    const mk = (hero) => {
        const s = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero, profile });
        s.stage.enemies.length = 0;
        s.player.x = 200;
        s.player.y = s.stage.groundY;
        return s;
    };
    // A visible wraith next to a grounded hero can be hit by a melee swing and by a bolt.
    for (const hero of ['knight', 'ranger']) {
        const s = mk(hero);
        const w = Levels.createEnemy('frost_wraith', 260, s.stage.groundY, 200, 320, { id: 'fw' });
        w.hp = w.maxHp = 10;
        s.stage.enemies.push(w);
        for (let i = 0; i < 240; i++) {
            s.player.invulnTimer = 9;
            w.phaseT = 99;
            Sim.stepSimulation(s, STEP, {});
        }
        s.player.facing = Math.sign(w.x - s.player.x) || 1;
        Sim.stepSimulation(s, STEP, { attackPressed: true });
        for (let i = 0; i < 20 && w.hp === 10; i++) { w.phaseT = 99; s.player.invulnTimer = 9; Sim.stepSimulation(s, STEP, {}); }
        assert.ok(w.hp < 10, `a visible wraith takes damage from the ${hero}'s attack`);
    }
    // Each rune gate opens from its own switches only.
    {
        const s = mk();
        s.stage = Levels.buildStage({ id: 99, worldIndex: 1, stageInWorld: 1, nameKey: 'x', parTime: 90, zones: ['meadow', 'rune_gate', 'rune_gate', 'key_sanctum'], enemies: ['slime'] });
        const [gA, gB] = s.stage.gates;
        assert.ok(gA && gB && gA.switchIds.length === 3 && gB.switchIds.length === 3, 'two gates with their own switches');
        s.events = [];
        s.stage.switches.filter((sw) => gA.switchIds.includes(sw.id)).forEach((sw) => Zones.hitRect(s, sw.x - 5, sw.y - 5, 10, 10, 1));
        assert.ok(gA.open && !gB.open, 'gate A opens from its own switches while gate B stays shut');
    }
    // Storm expiry gives the extra air jump back.
    {
        const s = mk('ranger');
        s.player.y = s.stage.groundY - 300;
        s.player.airJumpsLeft = 1;
        Runes.grant(s, 'storm', s.player.x, s.player.y);
        assert.equal(s.player.airJumpsLeft, 2);
        s.player.rune.timeLeft = 0.03;
        for (let i = 0; i < 4; i++) { s.player.invulnTimer = 9; Sim.stepSimulation(s, STEP, {}); }
        assert.ok(s.player.rune === null && s.player.airJumpsLeft === 1 && !s.player.onGround, 'storm expiry mid-air removes the extra air jump');
    }
    // Chasers stop instead of flip-flopping (dead zone, pit edges) and resume patrol afterwards.
    {
        const mkW = () => {
            const s = Sim.createSimulation({ mode: 'campaign', levelId: 2, hero: 'knight', profile });
            s.stage.enemies.length = 0;
            const T = s.stage.tileSize;
            const w = Levels.createEnemy('wolf', 20 * T, s.stage.groundY, 8 * T, 40 * T, { id: 'wf' });
            s.stage.enemies.push(w);
            return { s, w, T };
        };
        const tick = (s, n, each) => { for (let i = 0; i < n; i++) { s.player.invulnTimer = 9; s.player.vx = 0; if (each) each(i); Sim.stepSimulation(s, STEP, {}); } };
        // (a) hero ~2 tiles directly above the wolf
        {
            const { s, w, T } = mkW();
            tick(s, 30, () => { s.player.x = w.x; s.player.y = w.y - 2 * T; });
            let flips = 0, last = w.facing;
            tick(s, 60, () => { s.player.x = w.x + 2; s.player.y = w.y - 2 * T; if (w.facing !== last) { flips++; last = w.facing; } });
            assert.ok(flips <= 1, 'wolf under the hero flips at most once, got ' + flips);
            assert.equal(w.vx, 0, 'wolf under the hero stands still');
            assert.equal(w.animState, 'idle', 'wolf under the hero idles');
        }
        // (b) pit edge: chaser stops at the edge without flipping
        {
            const { s, w, T } = mkW();
            const edge = 24;
            s.stage.platforms.length = 0;
            s.stage.solidMap = {};
            for (let tx = 8; tx < edge; tx++) for (let ty = 13; ty < s.stage.heightTiles; ty++) s.stage.solidMap[tx + ',' + ty] = true;
            w.x = (edge - 3) * T;
            w.maxX = 40 * T;
            let flips = 0, last = w.facing;
            tick(s, 150, () => { s.player.x = (edge + 4) * T; s.player.y = s.stage.groundY; if (w.facing !== last) { flips++; last = w.facing; } });
            assert.ok(w.x < edge * T && w.x > (edge - 1) * T - 8, 'wolf stays on the platform at the edge, x=' + w.x);
            assert.equal(w.vx, 0, 'wolf stops at the pit edge');
            assert.equal(w.facing, 1, 'wolf keeps facing the hero at the edge');
            assert.ok(flips <= 1, 'at most the initial turn at the edge, got ' + flips);
        }
        // (c) hero leaves range: wolf patrols again
        {
            const { s, w, T } = mkW();
            tick(s, 60, () => { s.player.x = w.x; s.player.y = w.y - 2 * T; });
            assert.equal(w.vx, 0);
            let moved = false;
            tick(s, 6, () => { s.player.x = w.x + 2000; s.player.y = w.y; });
            moved = w.vx !== 0;
            assert.ok(moved, 'wolf resumes patrol once the hero leaves range');
        }
    }
}
console.log('Runenpfad round 3 review fixes passed.');
