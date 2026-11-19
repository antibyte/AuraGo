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
loadModule('ui/js/desktop/apps/runenpfad/swing.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/puzzles.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/dragons.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/npcs.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/npc-story.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/npc-story-late.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/dialogue.js', sandbox);
loadModule('ui/js/desktop/apps/runenpfad/trade.js', sandbox);
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

// Optional Windrochen rides use regular one-way platform collision and coin pickups.
{
    const rayLevels = [2, 4, 7, 9, 14, 17, 19, 22, 24];
    rayLevels.forEach((id) => {
        const stage = Levels.buildStage(id);
        assert.equal(stage.skyRays.length, 1, 'campaign level ' + id + ' has one optional sky ray');
        const ray = stage.skyRays[0];
        assert.ok(stage.movingPlatforms.includes(ray), 'the rendered ray is the rideable moving-platform object');
        assert.equal(ray.w, 144, 'the collision deck stays on the back');
        assert.equal(ray.h, 16);
        assert.equal(ray.endX - ray.startX, 240, 'the horizontal flight route is bounded');
        assert.equal(ray.baseY, stage.groundY - 82, 'the deck can be reached with one normal jump from flat ground');
        const floorRow = stage.groundY / stage.tileSize;
        for (let tx = Math.floor(ray.startX / stage.tileSize); tx <= Math.floor((ray.endX + ray.w - 1) / stage.tileSize); tx++) {
            assert.ok(stage.solidMap[tx + ',' + floorRow], 'the ray route has continuous ground below it (level ' + id + ')');
        }
        for (const camp of stage.npcs || []) {
            assert.ok(ray.endX + ray.w <= camp.campX0 || ray.startX >= camp.campX1, 'the ray avoids NPC camp and access area (level ' + id + ')');
        }
        const rayCoins = stage.collectibles.filter((item) => item.id.indexOf('sky_ray_coin_') === 0);
        assert.equal(rayCoins.length, 3, 'the route carries three ordinary coin pickups');
        assert.ok(rayCoins.every((item) => item.kind === 'coin' && item.collected === false && item.assetId === 'coin'));
    });
    assert.equal(Levels.buildStage(1).skyRays.length, 0, 'the starter tutorial remains unchanged');
    assert.equal(Levels.buildStage(5).skyRays.length, 0, 'boss arenas do not receive an extra platform route');
    const endlessRay = Levels.buildStage(Levels.createEndlessConfig(2, 4242)).skyRays[0];
    const endlessRayAgain = Levels.buildStage(Levels.createEndlessConfig(2, 4242)).skyRays[0];
    assert.ok(endlessRay, 'an early endless floor can have a ray');
    assert.deepEqual([endlessRay.startX, endlessRay.endX, endlessRay.baseY], [endlessRayAgain.startX, endlessRayAgain.endX, endlessRayAgain.baseY], 'ray placement is deterministic for an endless seed');
    assert.equal(Levels.buildStage(Levels.createEndlessConfig(3, 4242)).skyRays.length, 0, 'every third endless boss floor stays clear');

    for (const hero of ['knight', 'ranger']) {
        const s = Sim.createSimulation({ mode: 'campaign', levelId: 2, hero: hero });
        const ray = s.stage.skyRays[0];
        assert.ok(ray, hero + ' gets the level 2 ray opportunity');
        const floorRow = s.stage.groundY / s.stage.tileSize;
        for (let tx = 0; tx < s.stage.widthTiles; tx++) {
            for (let ty = 0; ty < floorRow; ty++) delete s.stage.solidMap[tx + ',' + ty];
            s.stage.solidMap[tx + ',' + floorRow] = true;
        }
        s.stage.platforms.length = 0;
        s.stage.hazards.length = 0;
        s.stage.crates.length = 0;
        s.stage.enemies.length = 0;
        s.stage.movingPlatforms = [ray];

        let board = null;
        let boardAttempts = 0;
        let boardedLand = null;
        let jumpTowardRay = false;
        let jumpDriveFrames = 0;
        for (let i = 0; i < 1800 && !board; i++) {
            if (s.player.onGround) jumpTowardRay = false;
            const input = { right: !jumpTowardRay || jumpDriveFrames > 0 };
            if (jumpDriveFrames > 0) jumpDriveFrames--;
            const nearRay = s.player.x >= ray.x - 72 && s.player.x <= ray.x + ray.w + 20;
            if (s.player.onGround && nearRay && boardAttempts < 8) {
                input.jumpPressed = true;
                jumpTowardRay = true;
                jumpDriveFrames = 8;
                boardAttempts++;
            }
            const events = Sim.stepSimulation(s, Sim.STEP, input);
            board = events.find((event) => event.type === 'sky_ray_board') || null;
            boardedLand = events.find((event) => event.type === 'land' && event.detail.skyRay) || boardedLand;
        }
        assert.ok(board, hero + ' boards through a real jump and moving-platform collision (no teleport)');
        assert.equal(boardedLand && boardedLand.detail.skyRay, true, 'a ray landing is identified for soft audio/material handling');
        assert.equal(ray.ridden, true, 'the rider state is visible to the renderer');

        let repeatedBoards = 0;
        let stepsOnRay = 0;
        s.player.stepTimer = 0;
        for (let i = 0; i < 3; i++) {
            const events = Sim.stepSimulation(s, Sim.STEP, { left: true });
            repeatedBoards += events.filter((event) => event.type === 'sky_ray_board').length;
            stepsOnRay += events.filter((event) => event.type === 'step' && event.detail.skyRay).length;
        }
        assert.equal(stepsOnRay, 1, 'a moving rider footstep carries the ray surface marker');

        const center = ray.w / 2;
        for (let i = 0; i < 24; i++) {
            const offset = s.player.x - ray.x;
            const input = offset < center - 22 ? { right: true } : offset > center + 22 ? { left: true } : {};
            const events = Sim.stepSimulation(s, Sim.STEP, input);
            repeatedBoards += events.filter((event) => event.type === 'sky_ray_board').length;
        }
        assert.equal(repeatedBoards, 0, 'standing or riding does not repeat the boarding event');
        assert.equal(s.player.onPlatformRide, ray, hero + ' remains carried by the ray');

        const rideOffset = s.player.x - ray.x;
        const directionBeforeTurn = ray.dir;
        let turned = false;
        for (let i = 0; i < 360 && !turned; i++) {
            Sim.stepSimulation(s, Sim.STEP, {});
            turned = ray.dir !== directionBeforeTurn;
            assert.equal(s.player.onPlatformRide, ray, hero + ' stays aboard through the route');
            assert.ok(Math.abs((s.player.x - ray.x) - rideOffset) < 3, hero + ' is carried across horizontal travel and the endpoint turn');
        }
        assert.equal(turned, true, 'the ride test reaches a route endpoint and reversal');

        const pausedAt = [ray.x, ray.y, s.player.x, s.player.y];
        s.state = 'paused';
        Sim.stepSimulation(s, Sim.STEP, { right: true });
        assert.deepEqual([ray.x, ray.y, s.player.x, s.player.y], pausedAt, 'pause freezes the ray and rider');
        s.state = 'playing';
        s.fx.hitstop = 0.08;
        Sim.stepSimulation(s, Sim.STEP, { right: true });
        assert.deepEqual([ray.x, ray.y, s.player.x, s.player.y], pausedAt, 'hitstop freezes the ray and rider');
        s.fx.hitstop = 0;

        const leap = Sim.stepSimulation(s, Sim.STEP, { jumpPressed: true }).filter((event) => event.type === 'sky_ray_leap');
        assert.equal(leap.length, 1, hero + ' gets one departure event from an intentional jump');
        assert.equal(ray.ridden, false, 'the renderer stops treating an airborne hero as a rider');
        assert.ok(s.player.vy < 0, hero + ' actually jumped off the ray');
        const following = Sim.stepSimulation(s, Sim.STEP, {}).filter((event) => event.type === 'sky_ray_leap');
        assert.equal(following.length, 0, 'the leap cue is edge-triggered');
    }
}

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
const resonanceLevel1 = Levels.buildStage(1).resonancePath;
assert.ok(resonanceLevel1, 'the tutorial shows an optional resonance path');
assert.equal(resonanceLevel1.timeLimit, 12, 'resonance path exposes its timed window');
assert.equal(resonanceLevel1.runes.length, 3, 'the tutorial path teaches a short ordered sequence');
assert.equal(resonanceLevel1.runes[1].y, resonanceLevel1.runes[0].y - 64, 'level 1 visibly teaches a jump');
assert.ok(resonanceLevel1.runes[0].x > Levels.buildStage(1).spawn.x + 30, 'the first rune sits ahead of the hero spawn');
const resonanceStarts = new Set();
for (let lvl = 1; lvl <= 25; lvl++) {
    const stage = Levels.buildStage(lvl);
    const path = stage.resonancePath;
    assert.ok(path && path.groundY > 0, `level ${lvl} places its path on a safe existing ground stretch`);
    assert.equal(path.runes.length, lvl === 1 ? 3 : 4);
    if (lvl > 1) resonanceStarts.add(path.runes[0].x);
    assert.deepEqual(path.runes, Levels.buildStage(lvl).resonancePath.runes, `level ${lvl} path placement is deterministic`);
    assert.ok(path.runes.every((r, i) => i === 0 || Math.hypot(r.x - path.runes[i - 1].x, r.y - path.runes[i - 1].y) > 30), `level ${lvl} keeps rune hitboxes distinct`);
    for (const rune of path.runes) {
        const tx = Math.floor(rune.x / stage.tileSize);
        const ty = Math.floor(path.groundY / stage.tileSize);
        assert.ok(stage.solidMap[tx + ',' + ty], `level ${lvl} rune stays over solid ground`);
        for (let head = 1; head <= 3; head++) assert.ok(!stage.solidMap[tx + ',' + (ty - head)], `level ${lvl} keeps jump headroom`);
        assert.ok(!(stage.platforms || []).some((p) => rune.x > p.x - 14 && rune.x < p.x + p.w + 14 && p.y >= path.groundY - 144 && p.y < path.groundY), `level ${lvl} avoids a platform overhang`);
    }
}
assert.ok(resonanceStarts.size > 1, 'later levels deterministically vary their safe path location');
assert.notDeepEqual(Levels.buildStage(2).resonancePath.runes.map((r) => r.y), Levels.buildStage(3).resonancePath.runes.map((r) => r.y), 'later levels rotate the jump arrangement');
const endlessResonance = new Set();
for (let floor = 1; floor <= 20; floor++) {
    const stage = Levels.buildStage({ mode: 'endless', endlessFloor: floor, seed: 9001 });
    assert.ok(stage.resonancePath && stage.resonancePath.groundY > 0, `endless floor ${floor} gets a safe resonance path`);
    assert.deepEqual(stage.resonancePath.runes, Levels.buildStage({ mode: 'endless', endlessFloor: floor, seed: 9001 }).resonancePath.runes, `endless floor ${floor} placement is deterministic`);
    endlessResonance.add(stage.resonancePath.runes[0].x);
}
assert.ok(endlessResonance.size > 1, 'endless floors vary their path location');

// Exercise the generated sequences with real fixed-step movement and jump inputs, never rune-position teleports.
function runResonanceWithInputs(levelId, hero) {
    const s = Sim.createSimulation({ mode: 'campaign', levelId, hero, profile });
    const path = s.stage.resonancePath;
    s.stage.enemies = [];
    s.stage.hazards = [];
    s.stage.boulders = [];
    s.stage.crates = [];
    s.stats = Object.assign({}, s.stats, { maxAirJumps: 0 });
    s.player.airJumpsLeft = 0;
    if (levelId > 1) {
        s.player.x = path.runes[0].x - 36;
        s.player.y = path.groundY;
        s.player.vx = 0;
        s.player.vy = 0;
        s.player.onGround = true;
    }
    const events = [];
    const jumpedAt = new Set();
    for (let frame = 0; frame < 1200 && !path.complete; frame++) {
        const index = path.nextRune;
        const target = path.runes[index];
        const dx = target.x - s.player.x;
        const elevated = target.y < path.runes[0].y - 30;
        let right = dx > 18;
        let left = dx < -18;
        if (!elevated && path.started && dx > 0 && dx < 48) right = false;
        const jumpPressed = elevated && s.player.onGround && dx <= 52 && !jumpedAt.has(index);
        if (jumpPressed) jumpedAt.add(index);
        Sim.stepSimulation(s, Sim.STEP, { right, left, jumpPressed });
        events.push(...s.events);
    }
    const target = path.runes[path.nextRune] || path.runes[path.runes.length - 1];
    assert.equal(path.complete, true, `level ${levelId} ${hero} completes its path by moving and jumping (rune ${path.nextRune}/${path.runes.length}, x ${Math.round(s.player.x)}, y ${Math.round(s.player.y)}, target ${Math.round(target.x)},${Math.round(target.y)}, ground ${path.groundY}, jumps ${events.filter((e) => e.type === 'jump').length}, issued ${Array.from(jumpedAt)})`);
    assert.ok(events.some((e) => e.type === 'resonance_start'), 'first touch starts the path timer');
    assert.ok(events.some((e) => e.type === 'resonance_complete'), 'last touch emits the single completion event');
    const requiredJumps = levelId === 3 ? 2 : 1;
    assert.ok(events.filter((e) => e.type === 'jump').length >= requiredJumps, `level ${levelId} pattern is possible without double jump`);
    assert.equal(s.profile.coins, profile.coins + 3, 'one successful path grants three coins');
    assert.ok(s.floatingTexts.some((ft) => ft.text === '+3' && ft.icon === 'coin'), 'path reward shows the existing coin icon');
    return s;
}
for (let levelId = 1; levelId <= 4; levelId++) {
    runResonanceWithInputs(levelId, 'knight');
    runResonanceWithInputs(levelId, 'ranger');
}

const landingSim = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero: 'knight', profile });
landingSim.player.y = landingSim.stage.resonancePath.groundY - 100;
landingSim.player.vy = 300;
landingSim.player.onGround = false;
let landingEvent = null;
for (let frame = 0; frame < 30 && !landingEvent; frame++) {
    landingEvent = Sim.stepSimulation(landingSim, Sim.STEP, {}).find((e) => e.type === 'land') || null;
}
assert.ok(landingEvent && landingEvent.detail.impactSpeed > 0, 'landing event reports a positive impact speed');
assert.equal(landingEvent.detail.sound, landingSim.stage.world.stepSound, 'landing sound follows the stage biome');

// A timeout rearms only after leaving Rune 0, and paused impact frames do not spend path time.
const retrySim = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero: 'knight', profile });
const retryPath = retrySim.stage.resonancePath;
retrySim.player.x = retryPath.runes[0].x;
retrySim.player.y = retryPath.runes[0].y + 22;
Variety.stepResonance(retrySim, 0);
assert.equal(retryPath.started, true);
retryPath.timeLeft = Sim.STEP / 2;
retrySim.events = [];
Variety.stepResonance(retrySim, Sim.STEP);
assert.equal(retryPath.expired, true);
assert.equal(retryPath.needsLeave, true);
assert.equal(retrySim.events.filter((e) => e.type === 'resonance_expire').length, 1);
retrySim.events = [];
Variety.stepResonance(retrySim, 20);
assert.equal(retrySim.events.length, 0, 'standing on rune 0 does not loop the failure sound');
retrySim.player.x += 60;
Variety.stepResonance(retrySim, 0);
assert.equal(retryPath.needsLeave, false, 'leaving rune 0 rearms a retry');
retrySim.player.x = retryPath.runes[0].x;
Variety.stepResonance(retrySim, 0);
assert.equal(retryPath.started, true, 're-entering rune 0 restarts the path');
assert.equal(retrySim.notice, 'runenpfad.notice.find_key', 'retry clears only the stale timeout notice');

const pauseSim = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero: 'knight', profile });
pauseSim.player.x = pauseSim.stage.resonancePath.runes[0].x;
pauseSim.player.y = pauseSim.stage.resonancePath.runes[0].y + 22;
Variety.stepResonance(pauseSim, 0);
pauseSim.stage.resonancePath.timeLeft = 8;
pauseSim.fx.hitstop = Sim.STEP;
Sim.stepSimulation(pauseSim, Sim.STEP, {});
assert.equal(pauseSim.stage.resonancePath.timeLeft, 8, 'impact freeze pauses the resonance timer');

const respawnPathSim = runResonanceWithInputs(2, 'knight');
const paidCoins = respawnPathSim.profile.coins;
respawnPathSim.player.hp = 1;
respawnPathSim.player.invulnTimer = 0;
respawnPathSim.player.y = respawnPathSim.stage.height + 80;
Sim.stepSimulation(respawnPathSim, Sim.STEP, {});
assert.equal(respawnPathSim.state, 'playing', 'the first defeat respawns at the checkpoint');
assert.equal(respawnPathSim.stage.resonancePath.complete, true, 'respawn retains the completed path state');
respawnPathSim.player.x = respawnPathSim.stage.resonancePath.runes.at(-1).x;
respawnPathSim.player.y = respawnPathSim.stage.resonancePath.runes.at(-1).y + 22;
Variety.stepResonance(respawnPathSim, 0);
assert.equal(respawnPathSim.profile.coins, paidCoins, 'respawn cannot farm the path reward');

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
            const s = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero: 'knight', profile });
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

// ---- Round 4: swinging ropes, puzzles and dragons ----
{
    const STEP = Sim.STEP;
    const T = 48;
    const Combat = sandbox.RunenpfadCombat;
    const Puzzles = sandbox.RunenpfadPuzzles;
    const Swing = sandbox.RunenpfadSwing;
    const Dragons = sandbox.RunenpfadDragons;
    assert.ok(Combat && Puzzles && Swing && Dragons, 'round 4 modules loaded');

    const blank = () => ({ left: false, right: false, up: false, down: false, jumpPressed: false, attackPressed: false, dashPressed: false, healPressed: false });
    const mk = (levelId, hero) => Sim.createSimulation({ mode: 'campaign', levelId, hero: hero || 'knight' });
    const tickN = (s, n, input, fn) => {
        const evs = [];
        for (let i = 0; i < n; i++) {
            if (fn) fn(s, i);
            Sim.stepSimulation(s, STEP, typeof input === 'function' ? input(i) : (input || {})).forEach((e) => evs.push(e));
        }
        return evs;
    };
    const seenR4 = (evs, type) => evs.filter((e) => e.type === type).map((e) => e.detail);
    const placeHero = (s, x, y) => { s.player.x = x; s.player.y = y; s.player.vx = 0; s.player.vy = 0; s.player.onGround = true; };
    const zonesOf = (id) => Levels.CAMPAIGN_LEVELS[id - 1].zones.map((z) => z.split('|')[0].split(':')[0]);

    // --- placement: every world gets a puzzle, ropes in >= 3 worlds, dragons in worlds 2-5, drakelings in cavern/desert/citadel ---
    {
        const worldHas = (w, pred) => Levels.CAMPAIGN_LEVELS.some((l) => l.worldIndex === w && l.zones.some((z) => pred(z.split('|')[0].split(':')[0])));
        const puzzleZones = ['rune_sequence', 'lever_bridge', 'ritual_chain'];
        for (let w = 0; w < 5; w++) assert.ok(worldHas(w, (z) => puzzleZones.includes(z)), `world ${w + 1} has a puzzle`);
        assert.ok([0, 1, 2, 3, 4].filter((w) => worldHas(w, (z) => z === 'swing_gorge')).length >= 3, 'swing gorges in at least three worlds');
        for (let w = 1; w < 5; w++) assert.ok(worldHas(w, (z) => z === 'dragon_strafe'), `world ${w + 1} has a dragon strafe`);
        assert.ok(!worldHas(0, (z) => z === 'dragon_strafe'), 'no dragon in world 1');
        [1, 2, 4].forEach((w) => assert.ok(Levels.CAMPAIGN_LEVELS.some((l) => l.worldIndex === w && l.enemies.includes('drakeling')), `drakelings in world ${w + 1}`));
        assert.ok(!zonesOf(1).some((z) => puzzleZones.includes(z) || z === 'swing_gorge' || z === 'dragon_strafe'), 'level 1 stays simple');
        const sty = [0, 1, 4].map((w) => Levels.buildStage(Levels.CAMPAIGN_LEVELS.find((l) => l.worldIndex === w && l.zones.includes('swing_gorge')).id).swings[0].style);
        assert.deepEqual(sty, ['vine', 'rope', 'chain'], 'swing style by biome');
        // every campaign level and endless floors 1-12 build and run
        for (let i = 1; i <= 25; i++) {
            const s = mk(i);
            tickN(s, 120, {});
            assert.equal(s.state, 'playing', `level ${i} runs`);
        }
        for (const seed of [9001, 777]) {
            for (let f = 1; f <= 12; f++) {
                const s = Sim.createSimulation({ mode: 'endless', endlessFloor: f, seed, hero: 'knight' });
                tickN(s, 60, {});
                assert.ok(s.stage.key && s.stage.portal, `endless floor ${f} builds`);
            }
        }
        assert.deepEqual(JSON.stringify(Levels.buildStage(4).levers), JSON.stringify(Levels.buildStage(4).levers), 'puzzle layout is deterministic');
    }

    // --- swinging ropes ---
    {
        const s = mk(3);
        s.stage.enemies.length = 0;
        const ropes = s.stage.swings;
        assert.ok(ropes.length >= 3 && ropes.length <= 4, 'a gorge hangs 3-4 ropes');
        ropes.forEach((r) => assert.ok(Number.isFinite(r.ax + r.ay + r.len + r.angle + r.omega) && r.style, 'rope fields'));
        const r0 = ropes[0];
        // free ropes sway gently on their own
        let maxA = 0;
        tickN(s, 240, {}, () => { maxA = Math.max(maxA, Math.abs(r0.angle)); });
        assert.ok(maxA > 0.04 && maxA < 0.4, 'a free rope sways gently, max angle ' + maxA);
        // grab: airborne hero with hands at the lower rope
        r0.angle = 0; r0.omega = 0;
        const endY = r0.ay + r0.len;
        s.player.x = r0.ax + 6; s.player.y = endY - 40 + 60; s.player.vx = 0; s.player.vy = -50; s.player.onGround = false;
        let ev = tickN(s, 2, {});
        assert.ok(s.player.swing && s.player.swing.id === r0.id, 'hero grabs the rope');
        assert.equal(s.player.animState, 'swing', 'animState while hanging');
        const gr = seenR4(ev, 'swing_grab');
        assert.ok(gr.length === 1 && gr[0].id === r0.id && Number.isFinite(gr[0].x + gr[0].y), 'swing_grab payload');
        // hands sit at the rope end once the grip settles
        tickN(s, 30, {});
        const hx = r0.ax + r0.len * Math.sin(r0.angle);
        const hy = r0.ay + r0.len * Math.cos(r0.angle);
        const gripLen = Math.hypot(s.player.x - r0.ax, s.player.y - 40 - r0.ay);
        assert.ok(Math.abs(gripLen - s.player.swing.grip) < 3 && s.player.swing.grip > r0.len * 0.5 && Math.hypot(s.player.x - hx, s.player.y - 40 - hy) < r0.len, 'hands follow the grip point on the rope');
        // pumping builds amplitude
        let maxAmp = 0;
        tickN(s, 150, { right: true }, () => { maxAmp = Math.max(maxAmp, Math.abs(r0.angle)); });
        assert.ok(maxAmp > 0.55 && maxAmp <= Swing.MAX_ANGLE + 1e-6, 'pumping raises the swing, amplitude ' + maxAmp);
        assert.ok(s.player.swing, 'still hanging while pumping');
        // release with jump: tangential launch plus boost
        let guard = 0;
        while (!(r0.omega > 1 && r0.angle > 0.3) && guard++ < 300) tickN(s, 1, { right: true });
        ev = tickN(s, 1, { right: true, jumpPressed: true });
        const rl = seenR4(ev, 'swing_release');
        assert.ok(!s.player.swing && rl.length === 1 && rl[0].id === r0.id && rl[0].speed > 100, 'swing_release payload');
        assert.ok(s.player.vy < 0, 'release launches upward');
        assert.ok(s.player.swingCool > 0, 'a release starts the re-grab cool-down');
        // damage drops the hero
        const s2 = mk(3);
        s2.stage.enemies.length = 0;
        const q = s2.stage.swings[0];
        const qe = Swing.endOf(q); s2.player.x = qe.x; s2.player.y = qe.y + 20; s2.player.vy = 0; s2.player.onGround = false;
        tickN(s2, 2, {});
        assert.ok(s2.player.swing, 'second hero hangs');
        s2.player.invulnTimer = 0;
        Combat.hurtPlayer(s2, 1, s2.player.x + 10);
        assert.equal(s2.player.swing, null, 'damage drops the hero');
        tickN(s2, 3, {});
        assert.ok(s2.player.animState === 'hurt' || s2.player.animState === 'jump', 'hurt animation instead of swing');
        // a grounded hero never grabs, and ranger jump after release keeps its air jump
        const s3 = mk(3, 'ranger');
        s3.stage.enemies.length = 0;
        const w = s3.stage.swings[0];
        const we = Swing.endOf(w); s3.player.x = we.x; s3.player.y = we.y + 20; s3.player.onGround = false;
        tickN(s3, 2, {});
        assert.ok(s3.player.swing, 'ranger hangs');
        tickN(s3, 8, {});
        tickN(s3, 1, { jumpPressed: true });
        assert.ok(!s3.player.swing && s3.player.airJumpsLeft >= 1, 'ranger keeps its air jump after the release');
    }

    // The bot: run right, jump at the ledge edge, hold right to pump and release with jump on the forward upswing.
    function crossGorge(levelId, hero) {
        const s = mk(levelId, hero);
        s.stage.enemies.length = 0;
        const p = s.player;
        const g = s.stage.groundY;
        const results = [];
        const groups = Array.from(new Set(s.stage.swings.map((r) => r.group)));
        for (const group of groups) {
            const ropes = s.stage.swings.filter((r) => r.group === group);
            const first = ropes[0];
            const last = ropes[ropes.length - 1];
            const spikes = s.stage.hazards.filter((h) => h.kind === 'spikes' && h.x > first.ax - 6 * T && h.x < last.ax + 6 * T);
            const nearEdge = Math.min.apply(null, spikes.map((h) => h.x));
            const farEdge = Math.max.apply(null, spikes.map((h) => h.x + T));
            p.x = nearEdge - 60; p.y = g; p.vx = 0; p.vy = 0; p.onGround = true; p.swing = null; p.invulnTimer = 0;
            s.checkpoint = { x: p.x, y: g };
            const hp0 = p.hp;
            let ok = false;
            let usedRopes = new Set();
            for (let i = 0; i < 60 * 30 && s.state === 'playing' && p.hp === hp0; i++) {
                const input = blank();
                input.right = true;
                if (p.swing) {
                    usedRopes.add(p.swing.id);
                    const r = s.stage.swings.find((o) => o.id === p.swing.id);
                    if (r.omega > 0.3 && r.angle > 0.7) input.jumpPressed = true;
                } else if (p.onGround) {
                    if (p.x >= farEdge + 4) { ok = true; break; }
                    if (p.x >= nearEdge - 8 && p.x < first.ax - 20) input.jumpPressed = true;
                }
                Sim.stepSimulation(s, STEP, input);
            }
            results.push({ group, ok: ok && usedRopes.size >= 2, used: usedRopes.size, ropes: ropes.length, x: p.x, hp: p.hp });
        }
        return results;
    }
    {
        let gorges = 0;
        for (const lv of Levels.CAMPAIGN_LEVELS.filter((l) => l.zones.includes('swing_gorge'))) {
            for (const hero of ['knight', 'ranger']) {
                crossGorge(lv.id, hero).forEach((r) => {
                    gorges++;
                    assert.ok(r.ok, `bot (${hero}) crosses the swing gorge of level ${lv.id}: ${JSON.stringify(r)}`);
                });
            }
        }
        assert.ok(gorges >= 8, 'bot crossed every campaign swing gorge with both heroes (' + gorges + ')');
        // endless floors with a gorge are crossable too
        let eg = 0;
        for (let f = 1; f <= 40 && eg < 3; f++) {
            const cfg = Levels.createEndlessConfig(f, 9001);
            if (!cfg.zones.includes('swing_gorge')) continue;
            eg++;
            const s = Sim.createSimulation({ mode: 'endless', endlessFloor: f, seed: 9001, hero: 'ranger' });
            assert.ok(s.stage.swings.length >= 3, 'endless gorge has ropes');
        }
        assert.ok(eg >= 1, 'endless pool contains swing gorges');
    }

    // --- rune_sequence ---
    {
        for (const [lv, len, vault] of [[2, 3, true], [9, 3, false], [14, 4, true], [22, 4, true]]) {
            const s = mk(lv);
            s.stage.enemies.length = 0;
            const o = s.stage.oracles[0];
            assert.ok(o && o.len === len && o.state === 'idle', `level ${lv} oracle with ${len} glyphs`);
            const stones = s.stage.glyphStones;
            assert.equal(stones.length, len, 'one stone per glyph');
            assert.ok(stones.every((st) => st.glyph >= 0 && st.glyph <= 3 && st.oracleId === o.id && st.y === o.y), 'stone fields');
            const gate = s.stage.gates.find((gt) => gt.id === o.gateId);
            assert.ok(gate && !gate.open && gate.tiles.every((t) => s.stage.solidMap[t.tx + ',' + t.ty]), 'vault gate is closed');
            assert.equal(o.vault, vault);
            assert.equal(!!gate.portcullis, !vault, 'required puzzles use a full-height gate');
        }
        const s = mk(2);
        s.stage.enemies.length = 0;
        const o = s.stage.oracles[0];
        const stones = s.stage.glyphStones;
        placeHero(s, o.x - 100, s.stage.groundY);
        const keep = () => { s.player.invulnTimer = 9; };
        let evs = tickN(s, 400, {}, (sim) => { if (o.state !== 'input') { sim.player.x = o.x - 100; } keep(); });
        assert.equal(o.state, 'input', 'the oracle shows the sequence and then waits for input');
        const shows = seenR4(evs, 'seq_show');
        assert.equal(shows.length, 3, 'seq_show once per glyph');
        shows.forEach((d, i) => assert.ok(d.oracleId === o.id && d.index === i && d.glyph === o.seq[i] && Number.isFinite(d.x + d.y), 'seq_show payload ' + i));
        // a correct order solves the puzzle and opens the vault; use a real melee attack for the first stone
        const stoneOf = (glyph) => stones.find((st) => st.glyph === glyph);
        const first = stoneOf(o.seq[0]);
        placeHero(s, first.x - 36, s.stage.groundY);
        s.player.facing = 1;
        evs = tickN(s, 4, (i) => ({ attackPressed: i === 0 }), keep);
        let hits = seenR4(evs, 'seq_hit');
        assert.ok(hits.length === 1 && hits[0].glyph === o.seq[0] && hits[0].step === 0, 'a melee hit on the right glyph counts');
        for (let i = 1; i < o.seq.length; i++) {
            const st = stoneOf(o.seq[i]);
            evs = tickN(s, 25, {});
            Puzzles.hitRect(s, st.x - 4, st.y - 30, 8, 8, 1);
        }
        evs = tickN(s, 2, {});
        assert.equal(o.state, 'solved', 'a correct sequence solves');
        const gate = s.stage.gates.find((gt) => gt.id === o.gateId);
        assert.ok(gate.open && gate.tiles.every((t) => !s.stage.solidMap[t.tx + ',' + t.ty]), 'the vault gate opens');
        // wrong order: fail and replay
        const f = mk(2);
        f.stage.enemies.length = 0;
        const fo = f.stage.oracles[0];
        placeHero(f, fo.x - 100, f.stage.groundY);
        tickN(f, 400, {}, (sim) => { sim.player.invulnTimer = 9; });
        assert.equal(fo.state, 'input');
        const seq = fo.seq.slice();
        const wrong = f.stage.glyphStones.find((st) => st.glyph !== seq[0]);
        f.events = [];
        Puzzles.hitRect(f, wrong.x - 4, wrong.y - 30, 8, 8, 1);
        evs = f.events.slice();
        assert.ok(seenR4(evs, 'seq_fail').length === 1 && fo.state === 'fail', 'a wrong hit fails');
        assert.equal(f.notice, 'runenpfad.notice.seq_fail');
        evs = tickN(f, 130, {}, (sim) => { sim.player.invulnTimer = 9; });
        assert.ok(fo.state === 'showing' || fo.state === 'input', 'the oracle replays after the failure');
        assert.ok(seenR4(evs, 'seq_show').length >= 1, 'the replay emits seq_show again');
        assert.deepEqual(fo.seq, seq, 'the same sequence is replayed');
        const gateF = f.stage.gates.find((gt) => gt.id === fo.gateId);
        assert.ok(!gateF.open, 'a failure keeps the vault closed');
        // hitting the idle oracle replays too; stomping a stone counts
        const g2 = mk(2);
        const go = g2.stage.oracles[0];
        placeHero(g2, go.x - 600, g2.stage.groundY);
        Puzzles.hitRect(g2, go.x - 5, go.y - 40, 10, 10, 1);
        assert.equal(go.state, 'showing', 'hitting the idle oracle starts the sequence');
        const st2 = mk(2);
        st2.stage.enemies.length = 0;
        const so = st2.stage.oracles[0];
        placeHero(st2, so.x - 100, st2.stage.groundY);
        tickN(st2, 400, {}, (sim) => { sim.player.invulnTimer = 9; });
        const target = st2.stage.glyphStones.find((x) => x.glyph === so.seq[0]);
        st2.player.x = target.x; st2.player.y = target.y - 110; st2.player.vy = 300; st2.player.onGround = false;
        evs = tickN(st2, 20, {}, (sim) => { sim.player.invulnTimer = 9; });
        assert.ok(seenR4(evs, 'seq_hit').length === 1, 'landing on a stone counts as a hit');
        // the knight slam shock counts as well
        const sl = mk(2);
        sl.stage.enemies.length = 0;
        const slo = sl.stage.oracles[0];
        placeHero(sl, slo.x - 100, sl.stage.groundY);
        tickN(sl, 400, {}, (sim) => { sim.player.invulnTimer = 9; });
        const sst = sl.stage.glyphStones.find((x) => x.glyph === slo.seq[0]);
        Puzzles.slamLand(sl, sst.x, sst.y);
        assert.equal(slo.step, 1, 'the slam shock hits glyph stones');
        // the ranger's bolt hits stones and levers too
        const rg = mk(2, 'ranger');
        rg.stage.enemies.length = 0;
        const rgo = rg.stage.oracles[0];
        placeHero(rg, rgo.x - 100, rg.stage.groundY);
        tickN(rg, 400, {}, (sim) => { sim.player.invulnTimer = 9; });
        const rst = rg.stage.glyphStones.find((x) => x.glyph === rgo.seq[0]);
        placeHero(rg, rst.x - 160, rg.stage.groundY);
        rg.player.facing = 1;
        evs = tickN(rg, 40, (i) => ({ attackPressed: i === 0 }), (sim) => { sim.player.invulnTimer = 9; });
        assert.ok(seenR4(evs, 'seq_hit').length === 1, 'a ranger bolt hits the glyph stone');
    }

    // --- lever_bridge ---
    {
        // every preset is solvable by some subset of its levers (toggles are self-inverse) and not solved at the start
        const solvable = (masks, on) => {
            const need = on.map((v) => !v);
            for (let sub = 1; sub < (1 << masks.length); sub++) {
                const t = [false, false, false, false];
                masks.forEach((m, i) => { if (sub & (1 << i)) m.forEach((k) => { t[k] = !t[k]; }); });
                if (t.every((v, k) => v === need[k])) return sub;
            }
            return 0;
        };
        Puzzles.LEVER_PRESETS.forEach((pr, i) => {
            assert.ok(pr.on.some((v) => !v), `preset ${i} does not start solved`);
            assert.ok(solvable(pr.masks, pr.on), `preset ${i} is solvable`);
        });
        const stagesWithLevers = [];
        for (let i = 1; i <= 25; i++) if (zonesOf(i).includes('lever_bridge')) stagesWithLevers.push(i);
        for (let f = 1; f <= 40; f++) {
            if (Levels.createEndlessConfig(f, 9001).zones.includes('lever_bridge')) stagesWithLevers.push(1000 + f);
        }
        assert.ok(stagesWithLevers.length >= 4, 'lever bridges appear in the campaign and the endless pool');
        for (const id of stagesWithLevers) {
            const st = id > 1000 ? Levels.buildStage(Levels.createEndlessConfig(id - 1000, 9001)) : Levels.buildStage(id);
            const segs = st.bridgeSegs;
            const masks = st.levers.map((l) => l.mask);
            assert.equal(segs.length, 4, 'four bridge segments');
            assert.equal(st.levers.length, 3, 'three levers');
            assert.ok(solvable(masks, segs.map((sg) => sg.on)), `lever bridge in stage ${id} is solvable`);
        }
        // play level 4: solve it with real lever hits, then walk across
        const s = mk(4);
        s.stage.enemies.length = 0;
        const segs = s.stage.bridgeSegs;
        const levers = s.stage.levers;
        assert.ok(segs.some((sg) => !sg.on), 'the bridge starts incomplete');
        segs.forEach((sg) => assert.ok(sg.tiles.every((t) => t.solid === sg.on), 'segment solidity follows its state'));
        const sub = solvable(levers.map((l) => l.mask), segs.map((sg) => sg.on));
        let evs = [];
        levers.forEach((lv, i) => {
            if (!(sub & (1 << i))) return;
            placeHero(s, lv.x - 30, s.stage.groundY);
            s.player.facing = 1;
            evs = evs.concat(tickN(s, 3, (k) => ({ attackPressed: k === 0 }), (sim) => { sim.player.invulnTimer = 9; }));
            evs = evs.concat(tickN(s, 20, {}, (sim) => { sim.player.invulnTimer = 9; }));
        });
        assert.ok(seenR4(evs, 'lever_pull').length === Array.from({ length: 3 }, (_, i) => (sub >> i) & 1).reduce((a, b) => a + b, 0), 'lever_pull per hit');
        const pull = seenR4(evs, 'lever_pull')[0];
        assert.ok(Number.isFinite(pull.x + pull.y) && typeof pull.on === 'boolean', 'lever_pull payload');
        assert.ok(seenR4(evs, 'bridge_toggle').length >= 1, 'bridge_toggle events');
        assert.equal(seenR4(evs, 'bridge_done').length, 1, 'bridge_done once all segments are on');
        assert.ok(segs.every((sg) => sg.on && sg.tiles.every((t) => t.solid)), 'solving makes every segment solid');
        assert.equal(s.notice, 'runenpfad.notice.bridge_done');
        assert.ok(levers.every((l) => l.locked), 'levers lock after the bridge is done');
        // walking across the finished bridge never hurts
        const first = segs[0];
        placeHero(s, first.x - 40, s.stage.groundY);
        const hp0 = s.player.hp;
        tickN(s, 200, { right: true });
        assert.ok(s.player.x > first.x + 8 * T && s.player.hp === hp0, 'the hero walks over the finished bridge, x=' + s.player.x);
    }

    // --- ritual_chain ---
    {
        const s = mk(6);
        s.stage.enemies.length = 0;
        const r = s.stage.rituals[0];
        assert.ok(r && r.state === 'idle' && r.timeLimit >= 8 && r.timeLimit <= 12, 'ritual starts idle with a time limit');
        assert.ok(r.stairs.length === 3 && r.braziers.length === 3 && r.braziers.every((b, i) => b.order === i && !b.lit), 'ritual layout');
        assert.ok(r.stairs.every((st) => !st.up && st.tiles.every((t) => !t.solid)), 'stairs start sunk');
        const g = s.stage.groundY;
        placeHero(s, r.plate.x, g);
        let evs = tickN(s, 3, {});
        assert.ok(r.plate.down && r.state === 'active', 'stepping on the plate starts the ritual');
        assert.equal(seenR4(evs, 'plate_down').length, 1);
        evs = evs.concat(tickN(s, 40, {}));
        assert.ok(seenR4(evs, 'stairs_up').length === 1 && r.stairs.every((st) => st.up && st.tiles.every((t) => t.solid)), 'stairs rise and become solid');
        assert.ok(r.t > 0 && r.t < r.timeLimit, 'ritual timer counts up');
        // light the braziers in order by touch
        evs = [];
        r.braziers.forEach((b) => {
            placeHero(s, b.x, b.y);
            evs = evs.concat(tickN(s, 2, {}, (sim) => { sim.player.invulnTimer = 9; sim.player.vy = 0; }));
        });
        const lit = seenR4(evs, 'brazier_lit');
        assert.deepEqual(lit.map((d) => d.order), [0, 1, 2], 'brazier_lit in order');
        assert.equal(seenR4(evs, 'ritual_done').length, 1, 'ritual_done');
        assert.equal(r.state, 'done');
        const gate = s.stage.gates.find((gt) => gt.id === r.gateId);
        assert.ok(gate.open, 'the gate opens');
        assert.ok(r.stairs.every((st) => st.up), 'stairs stay up after success');
        assert.equal(s.notice, 'runenpfad.notice.ritual_done');
        // wrong order fails and resets, the plate retries
        const f = mk(6);
        f.stage.enemies.length = 0;
        const fr = f.stage.rituals[0];
        placeHero(f, fr.plate.x, g);
        tickN(f, 40, {});
        placeHero(f, fr.braziers[1].x, fr.braziers[1].y);
        evs = tickN(f, 2, {}, (sim) => { sim.player.invulnTimer = 9; sim.player.vy = 0; });
        assert.equal(seenR4(evs, 'ritual_fail').length, 1, 'a wrong order fails');
        assert.ok(fr.state === 'idle' && !fr.plate.down && fr.braziers.every((b) => !b.lit) && fr.stairs.every((st) => !st.up && st.tiles.every((t) => !t.solid)), 'braziers go out and stairs sink');
        assert.ok(seenR4(evs, 'stairs_down').length === 1, 'stairs_down');
        placeHero(f, fr.plate.x - 200, g);
        tickN(f, 60, {});
        placeHero(f, fr.plate.x, g);
        tickN(f, 3, {});
        assert.equal(fr.state, 'active', 'the plate retries the ritual');
        // timeout fails too, and a hero on the top stair drops back down
        const t2 = mk(6);
        t2.stage.enemies.length = 0;
        const tr = t2.stage.rituals[0];
        placeHero(t2, tr.plate.x, g);
        tickN(t2, 20, {});
        const top = tr.stairs[2];
        placeHero(t2, top.x + 0.3 * T, top.y);
        tickN(t2, 30, {}, (sim) => { sim.player.x = top.x + 0.3 * T; });
        assert.ok(t2.player.onGround && Math.abs(t2.player.y - top.y) < 2, 'the hero stands on the high stair');
        evs = tickN(t2, Math.ceil((tr.timeLimit + 1) * 60), {}, (sim) => { sim.player.invulnTimer = 9; });
        assert.equal(seenR4(evs, 'ritual_fail').length, 1, 'a timeout fails');
        assert.ok(t2.player.y > top.y + T, 'the hero falls back down when the stairs sink');
        // fairness: a scripted run through the ritual succeeds for both heroes (levels 6, 13 required, 18)
        const walkTo = (sim, tx, ty, maxSec) => {
            const p = sim.player;
            for (let i = 0; i < maxSec * 60; i++) {
                const input = blank();
                const dx = tx - p.x;
                if (Math.abs(dx) > 6) { input.right = dx > 0; input.left = dx < 0; }
                if (p.onGround && ty < p.y - 30 && Math.abs(dx) < 100) input.jumpPressed = true;
                Sim.stepSimulation(sim, STEP, input);
                if (Math.abs(p.x - tx) < 14 && Math.abs(p.y - ty) < 6 && p.onGround) return true;
            }
            return false;
        };
        for (const lv of [6, 13, 18]) {
            for (const hero of ['knight', 'ranger']) {
                const b = mk(lv, hero);
                b.stage.enemies.length = 0;
                const rr = b.stage.rituals[0];
                placeHero(b, rr.plate.x - 120, g);
                walkTo(b, rr.plate.x, g, 4);
                const legs = [[rr.braziers[0].x, g], [rr.stairs[0].x + 0.5 * T, rr.stairs[0].y], [rr.stairs[1].x + 0.5 * T, rr.stairs[1].y], [rr.braziers[1].x - 8, rr.stairs[1].y], [rr.stairs[2].x + 0.5 * T, rr.stairs[2].y], [rr.braziers[2].x - 8, rr.stairs[2].y]];
                for (const leg of legs) { if (rr.state === 'done') break; walkTo(b, leg[0], leg[1], 4); }
                assert.equal(rr.state, 'done', `${hero} completes the ritual of level ${lv} in time (t=${rr.t.toFixed(1)})`);
                assert.ok(rr.t < rr.timeLimit - 3, 'the ritual leaves a comfortable margin');
            }
        }
    }

    // --- drakeling ---
    {
        assert.ok(Levels.ENEMY_DEFS.drakeling && Levels.ENEMY_DEFS.drakeling.art === 'enemy-drakeling' && Levels.ENEMY_DEFS.drakeling.prefix === 'quadcopter_', 'drakeling def');
        assert.equal(Levels.ENEMY_DEFS.drakeling.hp, 3);
        assert.equal(Levels.ENEMY_DEFS.drakeling.behavior, 'drakeling');
        const s = mk(8);
        s.stage.enemies.length = 0;
        const d = Levels.createEnemy('drakeling', 20 * T, s.stage.groundY, 15 * T, 25 * T, { id: 'dk1' });
        s.stage.enemies.push(d);
        placeHero(s, d.x - 220, s.stage.groundY);
        let evs = tickN(s, 300, {}, (sim) => { sim.player.invulnTimer = 9; sim.player.x = Math.min(sim.player.x, d.x - 150); });
        const atk = seenR4(evs, 'enemy_attack').filter((a) => a.kind === 'fireball');
        assert.ok(atk.length >= 2 && atk[0].defId === 'drakeling' && Number.isFinite(atk[0].x + atk[0].y), 'drakeling emits enemy_attack kind fireball, shots=' + atk.length);
        assert.ok(seenR4(evs, 'enemy_windup').length >= 1, 'drakeling winds up before spitting');
        // keeps its distance
        const dist = Math.abs(s.player.x - d.x);
        assert.ok(dist > 150 && dist < 330, 'drakeling hovers at a distance, ' + dist);
        // an unprotected hero gets hurt by the fireball; the projectile carries kind fireball
        const h = mk(8);
        h.stage.enemies.length = 0;
        const d2 = Levels.createEnemy('drakeling', 20 * T, h.stage.groundY, 15 * T, 25 * T, { id: 'dk2' });
        h.stage.enemies.push(d2);
        placeHero(h, d2.x - 220, h.stage.groundY);
        let kinds = new Set();
        const hp0 = h.player.hp;
        tickN(h, 400, {}, (sim) => { sim.projectiles.forEach((pr) => kinds.add(pr.kind)); sim.player.x = d2.x - 200; sim.player.y = sim.stage.groundY; sim.player.vy = 0; });
        assert.ok(kinds.has('fireball'), 'projectile kind fireball');
        assert.ok(h.player.hp < hp0, 'a fireball hurts the hero');
        d2.hp = 1;
        Combat.damageEnemy(h, d2, 5, d2.x);
        assert.ok(d2.dead, 'drakelings die to hits');
    }

    // --- dragon strafe ---
    {
        for (const [lv, kind] of [[8, 'ember'], [12, 'ember'], [16, 'frost'], [23, 'ember']]) {
            const s = mk(lv);
            s.stage.enemies.length = 0;
            const run = s.stage.dragonRuns[0];
            assert.ok(run && run.kind === kind && run.state === 'off', `level ${lv} has a ${kind} dragon run`);
            assert.ok(run.fire === null && Number.isFinite(run.x1 + run.x2 + run.laneY + run.laneH), 'dragon run fields');
        }
        const sim = mk(8);
        sim.stage.enemies.length = 0;
        const run = sim.stage.dragonRuns[0];
        const g = sim.stage.groundY;
        // shelters: roofs ~2 tiles above the ground, every hero position is within a short run of one
        const roofX = [];
        for (let tx = Math.floor(run.x1 / T); tx < run.x2 / T; tx++) if (sim.stage.solidMap[tx + ',' + (g / T - 3)]) roofX.push(tx);
        assert.ok(roofX.length >= 12, 'four three-tile roofs');
        let maxGap = 0;
        for (let tx = Math.floor(run.x1 / T); tx < run.x2 / T; tx++) maxGap = Math.max(maxGap, Math.min.apply(null, roofX.map((r) => Math.abs(r - tx))));
        assert.ok(maxGap <= 2, 'a shelter is always within two tiles, gap ' + maxGap);
        // timeline: far -> warn (>= 1.2 s) -> strafe -> leave -> far; warn events precede the breath
        const outside = () => { sim.player.x = run.x1 + 4.5 * T; sim.player.y = g; sim.player.vy = 0; sim.player.invulnTimer = 9; };
        placeHero(sim, run.x1 + 4.5 * T, g);
        const order = [];
        const states = [];
        let warnTicks = 0;
        let evs = [];
        for (let i = 0; i < 60 * 40; i++) {
            outside();
            const e = Sim.stepSimulation(sim, STEP, {});
            e.forEach((x) => { evs.push(x); if (/^dragon_/.test(x.type)) order.push(x.type); });
            if (states[states.length - 1] !== run.state) states.push(run.state);
            if (run.state === 'warn') warnTicks++;
            if (states.length >= 5 && run.state === 'far' && states.includes('leave')) break;
        }
        assert.deepEqual(states.slice(0, 5), ['far', 'warn', 'strafe', 'leave', 'far'], 'dragon run cycle ' + states.join(','));
        assert.ok(warnTicks >= 70, 'the warn phase lasts >= 1.2 s, ticks ' + warnTicks);
        assert.ok(order.indexOf('dragon_roar') >= 0 && order.indexOf('dragon_warn') >= 0 && order.indexOf('dragon_breath') > order.indexOf('dragon_warn'), 'warn precedes the strafe: ' + order.join(','));
        const warnEv = seenR4(evs, 'dragon_warn')[0];
        assert.ok(warnEv.x1 === run.x1 && warnEv.x2 === run.x2 && warnEv.laneY === run.laneY && warnEv.laneH === run.laneH, 'dragon_warn payload');
        const roarEv = seenR4(evs, 'dragon_roar')[0];
        assert.ok(roarEv.kind === 'ember' && Number.isFinite(roarEv.x + roarEv.y), 'dragon_roar payload');
        assert.equal(sim.stage.dragonHinted, true);
        // exposed vs sheltered hero over a full strafe
        function strafeRun(heroX, hero) {
            const t = mk(8, hero);
            t.stage.enemies.length = 0;
            const r = t.stage.dragonRuns[0];
            placeHero(t, r.x1 + heroX * T, g);
            const hp = t.player.hp;
            let burned = 0;
            let breath = false;
            for (let i = 0; i < 60 * 30; i++) {
                t.player.x = r.x1 + heroX * T; t.player.y = g; t.player.vy = 0;
                if (t.player.invulnTimer > 0.3) t.player.invulnTimer = 0.3; // hurt knock-back must not break the test
                const e = Sim.stepSimulation(t, STEP, {});
                e.forEach((x) => { if (x.type === 'hazard' && x.detail.kind === 'dragonfire') burned++; if (x.type === 'dragon_breath') breath = true; });
                if (breath && r.state === 'far') break;
            }
            return { burned, hp: t.player.hp, hp0: hp };
        }
        const exposed = strafeRun(5.5, 'knight');
        assert.ok(exposed.burned >= 1 && exposed.hp < exposed.hp0, 'dragon fire hurts an exposed hero ' + JSON.stringify(exposed));
        const covered = strafeRun(2.5, 'knight');
        assert.ok(covered.burned === 0 && covered.hp === covered.hp0, 'a sheltered hero is safe ' + JSON.stringify(covered));
        assert.ok(Dragons.sheltered && typeof Dragons.sheltered === 'function');
        // frost uses the same damage with a frost flag
        const fz = mk(16);
        fz.stage.enemies.length = 0;
        const fr = fz.stage.dragonRuns[0];
        let frostFlag = false;
        for (let i = 0; i < 60 * 25; i++) {
            fz.player.x = fr.x1 + 5.5 * T; fz.player.y = g; fz.player.vy = 0; fz.player.invulnTimer = Math.min(fz.player.invulnTimer, 0.3);
            Sim.stepSimulation(fz, STEP, {}).forEach((x) => { if (x.type === 'hazard' && x.detail.kind === 'dragonfire' && x.detail.frost) frostFlag = true; });
            if (frostFlag) break;
        }
        assert.ok(frostFlag, 'frost dragon breath carries the frost flag');
    }

    // --- dragon boss ---
    {
        const cfg6 = Levels.createEndlessConfig(6, 9001);
        const cfg9 = Levels.createEndlessConfig(9, 9001);
        const cfg12 = Levels.createEndlessConfig(12, 9001);
        const cfg24 = Levels.createEndlessConfig(24, 9001);
        const cfg3 = Levels.createEndlessConfig(3, 9001);
        assert.equal(cfg3.boss, 'boss_orb', 'floor 3 keeps the world boss');
        assert.equal(cfg6.boss, 'boss_dragon_ember', 'floor 6 is an ember dragon');
        assert.equal(cfg9.boss, 'boss_quad', 'floor 9 is the world boss again');
        assert.equal(cfg12.boss, 'boss_dragon_ember');
        assert.equal(cfg24.boss, 'boss_dragon_frost', 'glacier floors bring the frost dragon');
        assert.ok(Levels.ENEMY_DEFS.boss_dragon_ember.art === 'dragon-ember' && Levels.ENEMY_DEFS.boss_dragon_frost.bossNameKey === 'runenpfad.boss_dragon_frost' && Levels.ENEMY_DEFS.boss_dragon_ember.flying && Levels.ENEMY_DEFS.boss_dragon_ember.hp >= 55, 'dragon boss defs');
        for (const floor of [6, 24]) {
            const s = Sim.createSimulation({ mode: 'endless', endlessFloor: floor, seed: 9001, hero: 'knight' });
            const boss = s.stage.enemies.find((e) => e.id === s.stage.bossId);
            assert.ok(boss && boss.def.dragon, `floor ${floor} builds a dragon boss`);
            assert.ok(boss.y - 42 * boss.def.scale > 0, 'the dragon fits under the ceiling');
            const g = s.stage.groundY;
            const seenPat = new Set();
            let fireHazard = false;
            let fireballProj = false;
            let drakes = false;
            let kindOk = true;
            s.player.hp = s.player.maxHp = 9999;
            for (let i = 0; i < 60 * 150 && seenPat.size < 4; i++) {
                s.player.x = Math.max(boss.minX + 80, Math.min(boss.maxX - 80, s.player.x));
                if (i === 0) { s.player.x = boss.x - 200; s.player.y = g; }
                s.player.invulnTimer = 9;
                if (s.player.y < g - 1 && s.player.onGround) s.player.y = g;
                Sim.stepSimulation(s, STEP, {}).forEach((e) => {
                    if (e.type === 'boss_attack') { seenPat.add(e.detail.pattern); assert.ok(Number.isFinite(e.detail.x + e.detail.y)); }
                    if (e.type === 'boss_telegraph') assert.ok(e.detail.kind);
                });
                s.tempHazards.forEach((h) => { if (h.kind === 'dragonfire') { fireHazard = true; if (h.variant !== boss.def.dragon) kindOk = false; } });
                s.projectiles.forEach((pr) => { if (pr.kind === 'fireball' && pr.fromEnemy) fireballProj = true; });
                if (s.stage.enemies.some((e) => e.summoned && e.defId === 'drakeling' && !e.dead)) drakes = true;
            }
            assert.deepEqual(Array.from(seenPat).sort(), ['dive', 'fireballs', 'roar', 'strafe'], `dragon floor ${floor} runs every pattern once: ` + Array.from(seenPat).join(','));
            assert.ok(fireHazard && kindOk, 'strafe leaves dragonfire temp hazards tagged with the dragon kind');
            assert.ok(fireballProj, 'fireballs fly as projectile kind fireball');
            assert.ok(drakes, 'the roar summons drakelings');
            assert.ok(s.stage.enemies.filter((e) => e.summoned && e.defId === 'drakeling' && !e.dead).length <= 2, 'at most two drakelings');
        }
        // phase two doubles the strafe and the dragon dies like any boss
        const s = Sim.createSimulation({ mode: 'endless', endlessFloor: 6, seed: 9001, hero: 'knight' });
        const boss = s.stage.enemies.find((e) => e.id === s.stage.bossId);
        boss.hp = Math.floor(boss.maxHp * 0.4);
        Combat.damageEnemy(s, boss, 1, boss.x, true);
        assert.equal(boss.phase, 2, 'the dragon enrages at half health');
        let defeated = false;
        Combat.damageEnemy(s, boss, 9999, boss.x, true);
        Sim.stepSimulation(s, STEP, {}).forEach((e) => { if (e.type === 'boss_defeat') defeated = true; });
        assert.ok(boss.dead && s.tempHazards.length === 0, 'defeat clears the fire');
    }


    // --- human-like reach: release anywhere in a wide window of the forward swing, then climb / release / let go ---
    {
        const hang = (sim, r) => {
            const e = Swing.endOf(r);
            sim.player.x = e.x; sim.player.y = e.y + 20; sim.player.vx = 0; sim.player.vy = 0; sim.player.onGround = false;
            tickN(sim, 2, {});
            assert.ok(sim.player.swing && sim.player.swing.id === r.id, 'hero hangs on ' + r.id);
        };
        const trial = (levelId, hero, ropeIdx, target) => {
            const sim = mk(levelId, hero);
            sim.stage.enemies.length = 0;
            const p = sim.player;
            const all = sim.stage.swings;
            const group = all[0].group;
            const ropes = all.filter((r) => r.group === group);
            const last = ropes[ropes.length - 1];
            const spikes = sim.stage.hazards.filter((h) => h.kind === 'spikes' && h.x > ropes[0].ax - 6 * T && h.x < last.ax + 6 * T);
            const farEdge = Math.max.apply(null, spikes.map((h) => h.x + T));
            const r = ropes[ropeIdx];
            hang(sim, r);
            p.invulnTimer = 0;
            const hp0 = p.hp;
            let peak = 0;
            let guard = 0;
            let turned = false;
            while (guard++ < 900 && !(turned && r.omega > 0.2 && r.angle >= target)) {
                // pump to a moderate amplitude, let the swing turn around once, then wait for the forward swing
                tickN(sim, 1, peak < Math.max(0.45, target - 0.1) ? { right: true } : {});
                peak = Math.max(peak, r.angle);
                if (peak >= Math.max(0.45, target - 0.1) && r.omega < -0.2) turned = true;
            }
            tickN(sim, 1, { right: true, jumpPressed: true, jumpKey: true });
            for (let i = 0; i < 150; i++) {
                if (p.hp !== hp0 || sim.state !== 'playing') return false;
                if (p.swing) { return p.swing.id !== r.id; }
                if (p.onGround) return p.x >= farEdge - 2;
                tickN(sim, 1, { right: true });
            }
            return false;
        };
        const lo = -0.15;
        const hi = 0.9;
        let total = 0;
        for (const lv of Levels.CAMPAIGN_LEVELS.filter((l) => l.zones.includes('swing_gorge'))) {
            for (const hero of ['knight', 'ranger']) {
                const n = mk(lv.id, hero).stage.swings.filter((r) => r.group === mk(lv.id, hero).stage.swings[0].group).length;
                for (let ri = 0; ri < n; ri++) {
                    let okN = 0;
                    for (let k = 0; k < 10; k++) if (trial(lv.id, hero, ri, lo + (hi - lo) * k / 9)) okN++;
                    total++;
                    const bottom = trial(lv.id, hero, ri, -0.02);
                    assert.ok(okN >= 8, `level ${lv.id} ${hero} rope ${ri}: ${okN}/10 release moments reach the next anchor`);
                    assert.ok(bottom, `level ${lv.id} ${hero} rope ${ri}: release at the bottom works`);
                    console.log(`reach level ${lv.id} ${hero} rope ${ri}: ${okN}/10`);
                }
            }
        }
        assert.ok(total >= 24, 'reach tested on ropes ' + total);

        // climbing: Up never releases, Space does with the boost
        const c = mk(3);
        c.stage.enemies.length = 0;
        const cr = c.stage.swings[0];
        hang(c, cr);
        const g0 = c.player.swing.grip;
        tickN(c, 1, { up: true, jumpPressed: true });
        tickN(c, 30, { up: true });
        assert.ok(c.player.swing && c.player.swing.grip < g0 - 50, 'Up climbs the rope, grip ' + c.player.swing.grip);
        tickN(c, 200, { up: true });
        assert.ok(c.player.swing && Math.abs(c.player.swing.grip - cr.len * 0.18) < 1, 'climb stops near the top');
        const evc = tickN(c, 1, { jumpPressed: true, jumpKey: true });
        assert.ok(!c.player.swing && seenR4(evc, 'swing_release').length === 1 && c.player.vy < -250, 'Space releases with the jump boost, vy ' + c.player.vy);
        // down climbs to the end, then lets go without a boost
        const d = mk(3);
        d.stage.enemies.length = 0;
        const dr = d.stage.swings[0];
        hang(d, dr);
        tickN(d, 40, { up: true });
        const gUp = d.player.swing.grip;
        tickN(d, 20, { down: true });
        assert.ok(d.player.swing && d.player.swing.grip > gUp + 20, 'Down climbs down');
        let dn = 0;
        while (d.player.swing && dn++ < 300) tickN(d, 1, { down: true });
        assert.ok(!d.player.swing && d.player.vy > -120, 'holding Down at the end lets go without a boost');
        // climbing into a ceiling bonks instead of embedding
        const b = mk(3);
        b.stage.enemies.length = 0;
        const br = b.stage.swings[0];
        hang(b, br);
        const ty = Math.floor((br.ay + br.len - 100) / T);
        const bx = Math.floor(br.ax / T);
        for (let x = bx - 2; x <= bx + 2; x++) b.stage.solidMap[x + ',' + ty] = true;
        let emb = false;
        tickN(b, 120, { up: true }, (sim) => {
            const pl = sim.player;
            if (pl.swing) for (let tx = Math.floor((pl.x - 14) / T); tx <= Math.floor((pl.x + 14) / T); tx++) for (let yy = Math.floor((pl.y - 44) / T); yy <= Math.floor((pl.y - 1) / T); yy++) if (b.stage.solidMap[tx + ',' + yy]) emb = true;
        });
        assert.ok(!emb && b.player.swing && b.player.swing.grip > (ty + 1) * T - br.ay - 8, 'climbing bonks on a ceiling, grip ' + b.player.swing.grip);
    }

    // --- swing clearance: arcs never touch solid tiles, and a block in the arc stops the swing without embedding ---
    {
        const Ph = sandbox.RunenpfadPhysics;
        const overlapsSolid = (st, x, y) => {
            for (let tx = Math.floor((x - 14) / T); tx <= Math.floor((x + 14) / T); tx++) {
                for (let ty = Math.floor((y - 44) / T); ty <= Math.floor((y - 1) / T); ty++) if (Ph.isSolidTile(st, tx, ty)) return true;
            }
            return false;
        };
        const stages = [];
        Levels.CAMPAIGN_LEVELS.forEach((l) => stages.push(Levels.buildStage(l.id)));
        for (const seed of [9001, 777]) for (let f = 1; f <= 12; f++) stages.push(Levels.buildStage(Levels.createEndlessConfig(f, seed)));
        let swept = 0;
        stages.forEach((st) => st.swings.forEach((r) => {
            swept++;
            for (let a = -Swing.MAX_ANGLE; a <= Swing.MAX_ANGLE + 1e-9; a += 0.05) {
                const x = r.ax + r.len * Math.sin(a);
                const y = r.ay + r.len * Math.cos(a) + 40;
                assert.ok(!overlapsSolid(st, x, y), `rope ${r.id} of stage ${st.id} clear at angle ${a.toFixed(2)}`);
            }
        }));
        assert.ok(swept >= 20, 'swept ropes ' + swept);
        // no hazard rect or pendulum sweep of a neighbouring zone reaches the hero anywhere along any rope's arc
        const hit = (hx, hy, h) => hx + 14 > h.x && hx - 14 < h.x + h.w && hy > h.y && hy - 44 < h.y + h.h;
        const offenders = [];
        stages.forEach((st) => {
            const rects = st.hazards.filter((h) => h.kind !== 'pendulum');
            (st.pendulums || []).forEach((pd) => {
                for (let a = -pd.amp; a <= pd.amp + 1e-9; a += 0.04) rects.push({ x: pd.x + Math.sin(a) * pd.len - 23, y: pd.y + Math.cos(a) * pd.len - 29, w: 46, h: 42, kind: 'pendulum' });
            });
            st.swings.forEach((r) => {
                for (let a = -Swing.MAX_ANGLE; a <= Swing.MAX_ANGLE + 1e-9; a += 0.05) {
                    for (const k of [0.18, 0.5, 1]) {
                        const x = r.ax + r.len * k * Math.sin(a);
                        const y = r.ay + r.len * k * Math.cos(a) + 40;
                        const h = rects.find((q) => hit(x, y, q));
                        if (h) offenders.push(`${st.id}:${r.id}:${h.kind}@${Math.round(h.x)}`);
                    }
                }
            });
        });
        assert.deepEqual(Array.from(new Set(offenders)), [], 'no hazard reaches a rope arc');
        const s = mk(3);
        s.stage.enemies.length = 0;
        const r0 = s.stage.swings[0];
        const e0 = Swing.endOf(r0);
        s.player.x = e0.x; s.player.y = e0.y + 20; s.player.onGround = false; s.player.vy = 0;
        tickN(s, 2, {});
        assert.ok(s.player.swing, 'hanging');
        // a solid wall in the forward arc
        const bx = Math.floor((r0.ax + r0.len * Math.sin(0.5)) / T);
        for (let ty = 4; ty <= 11; ty++) s.stage.solidMap[(bx + 1) + ',' + ty] = true;
        let embedded = false;
        let maxA = 0;
        tickN(s, 400, { right: true }, (sim) => {
            if (sim.player.swing) {
                const pl = sim.player;
                if (overlapsSolid(sim.stage, pl.x, pl.y)) embedded = true;
                maxA = Math.max(maxA, r0.angle);
            }
        });
        assert.ok(!embedded, 'the hero is never seated inside the block');
        assert.ok(maxA < 0.95, 'the block stops the swing, max angle ' + maxA);
    }
}
console.log('Runenpfad round 4 checks passed: swing ropes, rune sequence, lever bridge, ritual chain, drakeling, dragon strafe, dragon boss.');

// ---- Polish round: translated floating labels and the "new best time" flag ----
{
    const Runes = sandbox.RunenpfadRunes;
    // Every floating-label key the DOM-free core writes resolves in all sixteen locales.
    const keys = new Set(Object.keys(Runes.DURATIONS).map((kind) => 'runenpfad.rune_' + kind));
    ['sim.js', 'combat.js', 'runes.js', 'zones-extra.js'].forEach((f) => {
        const src = fs.readFileSync(path.join(rootDir, 'ui/js/desktop/apps/runenpfad', f), 'utf8');
        for (const m of src.matchAll(/\bkey: '(runenpfad\.[a-z_]+)'(?! \+)/g)) keys.add(m[1]);
    });
    ['runenpfad.hud_gems', 'runenpfad.hud_key_on', 'runenpfad.hud_rune', 'runenpfad.float_block', 'runenpfad.float_open'].forEach((k) => {
        assert.ok(keys.has(k), 'the core still labels a floating text with ' + k);
    });
    const langDir = path.join(rootDir, 'ui', 'lang', 'runenpfad');
    const locales = fs.readdirSync(langDir).filter((f) => f.endsWith('.json'));
    assert.equal(locales.length, 16, 'sixteen Runenpfad locales');
    locales.forEach((f) => {
        const dict = JSON.parse(fs.readFileSync(path.join(langDir, f), 'utf8'));
        keys.forEach((k) => assert.ok(typeof dict[k] === 'string' && dict[k].trim(), f + ' translates the floating label ' + k));
    });

    // Clears level 1 at a fixed clock: one gem, the key, then the portal.
    const clear = (bestBefore) => {
        const prof = Sim.createDefaultProfile();
        if (bestBefore) prof.bestTimeByLevel[1] = bestBefore;
        const s = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero: 'knight', profile: prof });
        s.stage.enemies.length = 0;
        const gem = s.stage.collectibles.find((c) => c.kind === 'gem');
        s.player.x = gem.x;
        s.player.y = gem.y + 18;
        Sim.stepSimulation(s, Sim.STEP, {});
        const label = s.floatingTexts.find((ft) => ft.key === 'runenpfad.hud_gems');
        assert.ok(label && label.text === '+GEM' && label.suffix === ' 1/3', 'a gem pickup floats a translatable label with its count');
        s.player.x = s.stage.key.x;
        s.player.y = s.stage.key.y + 18;
        Sim.stepSimulation(s, Sim.STEP, {});
        assert.equal(s.hasKey, true, 'key collected');
        assert.ok(s.floatingTexts.some((ft) => ft.key === 'runenpfad.hud_key_on'), 'the key pickup floats a translatable label');
        s.elapsed = 60;
        s.player.x = s.stage.portal.x;
        s.player.y = s.stage.portal.y + 20;
        Sim.stepSimulation(s, Sim.STEP, {});
        assert.equal(s.state, 'won', 'portal with key wins level 1');
        return s;
    };
    const first = clear(0);
    assert.equal(first.newBest, false, 'a first clear sets the best time without calling it a new best');
    assert.equal(first.profile.bestTimeByLevel[1], 60);
    const faster = clear(80);
    assert.equal(faster.newBest, true, 'beating the stored time is a new best');
    assert.equal(faster.profile.bestTimeByLevel[1], 60);
    const slower = clear(40);
    assert.equal(slower.newBest, false, 'a slower clear is no new best');
    assert.equal(slower.profile.bestTimeByLevel[1], 40, 'a slower clear keeps the stored best time');
}
console.log('Runenpfad polish checks passed: floating label keys in 16 locales, new-best flag.');

// Round 5: fire and poison never float. Spore bursts and boss fire strips stand on the ground.
{
    const Physics = sandbox.RunenpfadPhysics;
    const mkSim = (levelId) => Sim.createSimulation({ mode: 'campaign', levelId: levelId || 1, hero: 'knight', profile: Levels.defaultProfile ? Levels.defaultProfile() : undefined });
    const TILE = 48;
    const sporeAt = (s, x, y, vx, vy) => s.projectiles.push({
        x: x, y: y, vx: vx, vy: vy, gravity: 0, fromEnemy: true, spore: true, kind: 'spore', damage: 1, radius: 8, life: 2.4, noDeflect: true, dead: false
    });
    const poison = (s) => s.tempHazards.filter((h) => h.kind === 'poison');
    const prep = () => {
        const s = mkSim(1);
        s.stage.enemies.length = 0;
        s.player.x = 100;
        s.player.y = s.stage.groundY;
        return s;
    };

    // Clears columns 10-34 down to a flat floor (no tiles above it, platforms, hazards or enemies) and returns its first column.
    const openCol = (s) => {
        const gt = s.stage.groundY / TILE;
        for (let tx = 10; tx < 35; tx++) {
            for (let ty = 0; ty < gt; ty++) delete s.stage.solidMap[tx + ',' + ty];
            s.stage.solidMap[tx + ',' + gt] = true;
        }
        s.stage.platforms.length = 0;
        s.stage.movingPlatforms.length = 0;
        s.stage.hazards.length = 0;
        return 12;
    };

    // groundBelow: floor, rock interior, platform tops and out-of-reach cases.
    {
        const s = prep();
        const gy = s.stage.groundY;
        const gx = (openCol(s) + 1) * TILE;
        assert.equal(Physics.groundBelow(s, gx, gy - 100, 3), gy, 'groundBelow finds the floor top below an open point');
        assert.equal(Physics.groundBelow(s, gx, gy - 4 * TILE - 20, 3), null, 'no ground within three tiles gives null');
        assert.equal(Physics.groundBelow(s, gx, gy + 10, 3), null, 'a point inside solid rock has no ground below it');
        s.stage.platforms.push({ x: gx - 40, y: gy - 120, w: 80, h: 16, solid: true, kind: 'oneway' });
        assert.equal(Physics.groundBelow(s, gx, gy - 160, 3), gy - 120, 'a platform top counts as ground and beats the floor');
        s.stage.movingPlatforms.push({ x: gx + 300, y: gy - 90, w: 90, h: 16 });
        assert.equal(Physics.groundBelow(s, gx + 340, gy - 150, 3), gy - 90, 'a moving platform top counts as ground');
    }

    // A rising spore that hits the wall of a ledge leaves its cloud on the floor in front of the wall.
    {
        const s = prep();
        const gy = s.stage.groundY;
        const gt = gy / TILE;
        const bx = openCol(s) + 2;
        for (let tx = bx; tx < bx + 4; tx++) for (let ty = gt - 3; ty < gt; ty++) s.stage.solidMap[tx + ',' + ty] = true;
        sporeAt(s, (bx - 1) * TILE + 30, gy - 100, 300, -160);
        for (let i = 0; i < 20 && !s.projectiles.every((p) => p.dead); i++) Sim.stepSimulation(s, Sim.STEP, {});
        const cloud = poison(s)[0];
        assert.ok(cloud, 'a spore hitting a ledge wall leaves a poison cloud');
        assert.equal(cloud.y + cloud.h, gy, 'the cloud bottom equals the ground top below the impact');
        assert.ok(cloud.x + cloud.w / 2 < bx * TILE, 'the cloud stays in open air in front of the wall');
    }

    // A spore that hits a ledge top from above lands on that ledge.
    {
        const s = prep();
        const gy = s.stage.groundY;
        const gt = gy / TILE;
        const lx = openCol(s) + 1;
        for (let tx = lx; tx < lx + 4; tx++) s.stage.solidMap[tx + ',' + (gt - 2)] = true;
        sporeAt(s, (lx + 1.5) * TILE, gt * TILE - 4 * TILE, 0, 300);
        for (let i = 0; i < 30 && !s.projectiles.every((p) => p.dead); i++) Sim.stepSimulation(s, Sim.STEP, {});
        const cloud = poison(s)[0];
        assert.ok(cloud && cloud.y + cloud.h === (gt - 2) * TILE, 'a spore landing on a ledge top stands the cloud on that ledge');
    }

    // No ground within reach: only an airborne puff, no lingering hazard.
    {
        const s = prep();
        const gy = s.stage.groundY;
        const ax = openCol(s) + 2;
        s.stage.solidMap[ax + ',' + (gy / TILE - 8)] = true;
        sporeAt(s, (ax - 1) * TILE + 30, gy - 8 * TILE + 20, 300, 0);
        let burst = null;
        for (let i = 0; i < 20 && !burst; i++) {
            Sim.stepSimulation(s, Sim.STEP, {});
            burst = s.events.find((e) => e.type === 'spore_burst') || null;
        }
        assert.ok(burst, 'the spore still bursts');
        assert.equal(poison(s).length, 0, 'a burst with no ground in reach leaves no poison hazard');
        assert.equal(burst.detail.airborne, true, 'the burst is flagged as an airborne puff');
    }

    // A hero hit while jumping also puts the cloud on the ground.
    {
        const s = prep();
        const gy = s.stage.groundY;
        const hx = (openCol(s) + 1) * TILE;
        s.player.x = hx;
        s.player.y = gy - 70;
        s.player.vy = 0;
        sporeAt(s, hx, gy - 70 - 20, 0, 0);
        Sim.stepSimulation(s, Sim.STEP, {});
        const cloud = poison(s)[0];
        assert.ok(cloud, 'a spore hitting an airborne hero bursts into a cloud');
        assert.equal(cloud.y + cloud.h, gy, 'the cloud from an airborne hero hit rests on the ground');
    }

    // The mech fire strip rests on the ground.
    {
        const s = Sim.createSimulation({ mode: 'campaign', levelId: 25, hero: 'knight' });
        const boss = s.stage.enemies.find((e) => e.def.boss);
        assert.ok(boss, 'level 25 has a boss');
        let strip = null;
        for (let i = 0; i < 60 * 90 && !strip; i++) {
            s.player.x = boss.x - 260;
            s.player.y = s.stage.groundY;
            s.player.invulnTimer = 5;
            s.player.hp = s.player.maxHp;
            if (i === 400 && !strip) boss.ai.cool = 0;
            Sim.stepSimulation(s, Sim.STEP, {});
            strip = s.tempHazards.find((h) => h.kind === 'fire') || null;
        }
        assert.ok(strip, 'the mech boss lays a fire strip');
        assert.equal(strip.y + strip.h, s.stage.groundY, 'the fire strip bottom equals the ground top');
    }
}
console.log('Runenpfad round 5 checks passed: grounded poison and fire.');

// Round 5: Storm trial gusts. Telegraphed, deterministic, only against the run direction, never a constant drift.
{
    const TILE = 48;
    const mkSim = (levelId) => Sim.createSimulation({ mode: 'campaign', levelId: levelId, hero: 'knight', profile: Levels.defaultProfile ? Levels.defaultProfile() : undefined });
    const flat = (s) => {
        const gt = s.stage.groundY / TILE;
        for (let tx = 10; tx < 40; tx++) {
            for (let ty = 0; ty < gt; ty++) delete s.stage.solidMap[tx + ',' + ty];
            s.stage.solidMap[tx + ',' + gt] = true;
        }
        s.stage.platforms.length = 0;
        s.stage.movingPlatforms.length = 0;
        s.stage.hazards.length = 0;
        s.stage.enemies.length = 0;
        s.tempHazards.length = 0;
    };
    const GUST_NOW = () => ({ state: 'gust', t: 99, dur: 100, strength: 1, dir: -1 });
    const CALM_NOW = () => ({ state: 'calm', t: 1e9, dur: 0, strength: 0, dir: -1 });
    // Runs ticks with the wind pinned to `wind()`, the hero starting at (x, y); returns the hero after the run.
    const trial = (wind, x, y, ticks, input) => {
        const s = mkSim(6);
        flat(s);
        s.player.x = x;
        s.player.y = y;
        s.player.vx = 0;
        s.player.vy = 0;
        for (let i = 0; i < ticks; i++) {
            s.wind = (i < 8 && y >= s.stage.groundY - 1) ? CALM_NOW() : wind(); // let a grounded hero settle first
            s.player.invulnTimer = 5;
            Sim.stepSimulation(s, Sim.STEP, i < 8 ? {} : (input || {}));
        }
        return s.player;
    };
    const schedule = () => {
        const s = mkSim(6);
        flat(s);
        const log = [];
        for (let i = 0; i < 60 * 40; i++) {
            s.player.invulnTimer = 5;
            Sim.stepSimulation(s, Sim.STEP, {});
            s.events.forEach((e) => { if (/^gust/.test(e.type)) log.push(e.type + '@' + s.stepCount + ':' + Math.round(e.detail.x)); });
            s.events.length = 0;
        }
        return log;
    };

    const a = schedule();
    const b = schedule();
    assert.deepEqual(a, b, 'the gust schedule is deterministic for the same seed');
    const types = a.map((e) => e.split('@')[0]);
    assert.ok(types.length >= 9, 'at least three gust cycles in 40 s: ' + types.length);
    types.forEach((t, i) => assert.equal(t, ['gust_warn', 'gust', 'gust_end'][i % 3], 'cycle order calm -> warn -> gust -> calm at ' + i));
    const at = (e) => Number(e.split('@')[1].split(':')[0]);
    for (let i = 0; i + 2 < a.length; i += 3) {
        const warnS = (at(a[i + 1]) - at(a[i])) / 60;
        const gustS = (at(a[i + 2]) - at(a[i + 1])) / 60;
        assert.ok(warnS > 0.85 && warnS < 0.95, 'warn lasts about 0.9 s: ' + warnS);
        assert.ok(gustS >= 1.55 && gustS <= 2.25, 'gust lasts 1.6-2.2 s: ' + gustS);
        if (i + 3 < a.length) {
            const calmS = (at(a[i + 3]) - at(a[i + 2])) / 60; // gust_end to the next gust_warn
            assert.ok(calmS > 3.4 && calmS < 6.1, 'calm lasts 3.5-6 s: ' + calmS);
        }
    }

    const gy = mkSim(6).stage.groundY;
    const x0 = 14 * TILE;
    const idle = trial(GUST_NOW, x0, gy, 90);
    assert.ok(idle.onGround && Math.abs(idle.x - x0) < 0.01, 'an idle hero on the ground does not move in a gust: ' + (idle.x - x0));
    const airWind = trial(GUST_NOW, x0, gy - 240, 24);
    const airCalm = trial(CALM_NOW, x0, gy - 240, 24);
    assert.ok(airWind.x < airCalm.x - 10, 'a hero in the air is pushed left by a gust: ' + (airWind.x - airCalm.x));
    const walkWind = trial(GUST_NOW, x0, gy, 90, { right: true });
    const walkCalm = trial(CALM_NOW, x0, gy, 90, { right: true });
    assert.ok(walkWind.x - x0 > 150, 'a hero walking right still makes forward progress in a gust: ' + (walkWind.x - x0));
    assert.ok(walkWind.x < walkCalm.x, 'the gust slows a walking hero');
    // jumping forward through a gust works too
    {
        const s = mkSim(6);
        flat(s);
        s.player.x = x0;
        s.player.y = gy;
        let start = s.player.x;
        for (let i = 0; i < 70; i++) {
            s.wind = GUST_NOW();
            s.player.invulnTimer = 5;
            Sim.stepSimulation(s, Sim.STEP, { right: true, jumpPressed: i === 2, jump: i < 30 });
        }
        assert.ok(s.player.x - start > 80, 'a hero jumping right in a gust still gets forward');
    }

    // No wind at all without the Storm trial; storm levels run it.
    [1, 2, 3, 4, 5, 7].forEach((id) => {
        const s = mkSim(id);
        for (let i = 0; i < 600; i++) { s.player.invulnTimer = 5; Sim.stepSimulation(s, Sim.STEP, {}); }
        assert.equal(s.wind, null, 'level ' + id + ' has no gusts');
        assert.ok(!s.stage.modEffects.gusts && !s.stage.modEffects.wind, 'level ' + id + ' has no wind effect');
    });
    [6, 12, 18, 24].forEach((id) => {
        const s = mkSim(id);
        assert.ok(s.stage.modEffects.gusts, 'level ' + id + ' carries the storm gusts');
        assert.ok(!s.stage.modEffects.wind, 'no constant wind on level ' + id);
    });
    // Rain only under an open sky; caves and glaciers get a wind bed.
    assert.notEqual(mkSim(6).stage.weather, 'rain', 'no rain in the cavern');
    assert.equal(mkSim(6).stage.ambience.secondary, 'wind-gentle', 'cavern storm ambience is wind');
    assert.notEqual(mkSim(18).stage.weather, 'rain', 'no rain on the glacier');
    assert.equal(mkSim(24).stage.weather, 'rain', 'the citadel storm keeps its rain');
}
console.log('Runenpfad round 5 checks passed: storm gusts.');

// ---- Round 6: lantern moth and bell toad enemy patterns ----
{
    const STEP = Sim.STEP;
    const TILE = Levels.TILE;
    const mothLevels = [3, 8, 14, 19, 23];
    const toadLevels = [4, 9, 13, 18, 24];
    const specialKinds = ['lantern_moth', 'bell_toad'];
    mothLevels.forEach((id) => {
        const a = Levels.buildStage(id);
        const b = Levels.buildStage(id);
        const moths = a.enemies.filter((e) => e.defId === 'lantern_moth');
        assert.equal(moths.length, 1, 'campaign level ' + id + ' replaces one enemy with a lantern moth');
        assert.equal(moths[0].def.art, 'enemy-lantern-moth');
        assert.deepEqual(a.enemies.map((e) => [e.defId, e.spawnX, e.spawnY]), b.enemies.map((e) => [e.defId, e.spawnX, e.spawnY]), 'moth placement stays deterministic on level ' + id);
        assert.ok(!(a.npcs || []).some((npc) => moths[0].spawnX >= npc.campX0 - TILE * 2 && moths[0].spawnX <= npc.campX1 + TILE * 2), 'moth is outside camp reach on level ' + id);
    });
    toadLevels.forEach((id) => {
        const stage = Levels.buildStage(id);
        const toads = stage.enemies.filter((e) => e.defId === 'bell_toad');
        assert.equal(toads.length, 1, 'campaign level ' + id + ' replaces one enemy with a bell toad');
        assert.equal(toads[0].def.art, 'enemy-bell-toad');
        assert.ok(stage.solidMap[Math.floor(toads[0].spawnX / TILE) + ',' + Math.floor(toads[0].spawnY / TILE)], 'toad begins on solid ground on level ' + id);
        assert.ok(!(stage.npcs || []).some((npc) => toads[0].spawnX >= npc.campX0 - TILE * 2 && toads[0].spawnX <= npc.campX1 + TILE * 2), 'toad is outside camp reach on level ' + id);
        if (id === 4 && stage.skyRays.length) {
            const ray = stage.skyRays[0];
            const inRayArea = toads[0].spawnX >= ray.startX - 130 && toads[0].spawnX <= ray.endX + ray.w + 130
                && toads[0].spawnY >= ray.y - 88 && toads[0].spawnY <= ray.y + 120;
            assert.equal(inRayArea, false, 'level 4 toad stays clear of the wind-ray route');
        }
    });
    [1, 2, 7, 8].forEach((floor) => {
        const stage = Levels.buildStage({ mode: 'endless', endlessFloor: floor, seed: 7331 });
        const expected = floor % 6 === 1 ? 'lantern_moth' : 'bell_toad';
        assert.equal(stage.enemies.filter((e) => e.defId === expected).length, 1, 'endless floor ' + floor + ' contains one ' + expected);
        assert.equal(stage.enemies.filter((e) => specialKinds.includes(e.defId)).length, 1, 'endless floor ' + floor + ' replaces instead of adding');
    });
    const hub = Levels.buildStage(0);
    assert.equal(hub.enemies.filter((e) => specialKinds.includes(e.defId)).length, 0, 'the village hub has neither new enemy');

    const arena = (hero, floorRow) => {
        const sim = Sim.createSimulation({ mode: 'campaign', levelId: 1, hero: hero || 'knight' });
        const stage = sim.stage;
        const row = floorRow == null ? 13 : floorRow;
        stage.enemies.length = 0;
        stage.npcs.length = 0;
        stage.platforms.length = 0;
        stage.movingPlatforms.length = 0;
        stage.hazards.length = 0;
        stage.solidMap = {};
        for (let tx = 0; tx < stage.widthTiles; tx++) {
            for (let ty = row; ty < stage.heightTiles; ty++) stage.solidMap[tx + ',' + ty] = true;
        }
        sim.projectiles.length = 0;
        sim.player.x = 10 * TILE;
        sim.player.y = row * TILE;
        sim.player.vx = 0;
        sim.player.vy = 0;
        sim.player.onGround = true;
        sim.player.coyoteTimer = 0.1;
        sim.player.invulnTimer = 0;
        sim.player.hurtTimer = 0;
        sim.player.attackCooldown = 0;
        sim.player.hp = sim.player.maxHp;
        sim.damageTaken = 0;
        return sim;
    };
    const spawnEnemy = (sim, kind, x, y) => {
        const enemy = Levels.createEnemy(kind, x, y, x - 4 * TILE, x + 4 * TILE, { id: 'test_' + kind });
        sim.stage.enemies.push(enemy);
        return enemy;
    };
    const step = (sim, input) => Sim.stepSimulation(sim, STEP, input || {});

    // The moth fixes its aim at the tell, dives on that same point and can be hit after a miss.
    {
        const sim = arena('knight');
        const floorY = sim.stage.groundY;
        const moth = spawnEnemy(sim, 'lantern_moth', 20.5 * TILE, floorY);
        sim.player.x = moth.x - 130;
        sim.player.facing = 1;
        let tell = null;
        for (let i = 0; i < 150 && !tell; i++) tell = step(sim).find((e) => e.type === 'moth_tell') || null;
        assert.ok(tell, 'the moth announces its dive');
        const lockedX = moth.moth.targetX;
        const lockedY = moth.moth.targetY;
        assert.equal(lockedX, sim.player.x, 'the moth aims at the player position when telegraphing');
        assert.equal(lockedY, sim.player.y - 20, 'the moth aims at the body target line');
        for (let i = 0; i < 12; i++) step(sim, { left: true });
        assert.equal(moth.moth.targetX, lockedX, 'moving during telegraph does not retarget the moth');
        let dive = null;
        for (let i = 0; i < 60 && !dive; i++) dive = step(sim).find((e) => e.type === 'moth_dive') || null;
        assert.ok(dive, 'the moth begins its dive after the tell');
        assert.equal(dive.detail.targetX, lockedX, 'the dive event preserves its locked target');
        step(sim);
        const progress = 1 - moth.moth.timer / moth.moth.diveDuration;
        assert.ok(Math.abs(moth.x - (moth.moth.sx + (lockedX - moth.moth.sx) * progress)) < 0.01, 'the dive follows a straight line');
        for (let i = 0; i < 80 && moth.moth.state === 'dive'; i++) step(sim);
        assert.equal(moth.moth.state, 'recover', 'a miss leaves the moth in its vulnerable recovery');
        assert.equal(moth.phased, false, 'recovery is not phased');
        const hp = moth.hp;
        const hit = step(sim, { right: true, attackPressed: true });
        assert.ok(hit.some((e) => e.type === 'enemy_hit' && e.detail.defId === 'lantern_moth'), 'the knight can punish the recovery with a real slash');
        assert.ok(moth.hp < hp, 'the moth takes recovery damage');
    }

    // Both hero attacks interrupt a charged call, and the toad can announce another attempt after resting.
    ['knight', 'ranger'].forEach((hero) => {
        const sim = arena(hero);
        const floorY = sim.stage.groundY;
        const toad = spawnEnemy(sim, 'bell_toad', 20.5 * TILE, floorY);
        const gap = hero === 'knight' ? 50 : 110;
        sim.player.x = toad.x - gap;
        sim.player.y = floorY;
        sim.player.facing = 1;
        toad.toad = { state: 'sit', cooldown: 0 };
        const announced = step(sim);
        assert.ok(announced.some((e) => e.type === 'toad_tell'), hero + ' sees the toad telegraph');
        assert.equal(toad.toad.state, 'tell');
        const hp = toad.hp;
        const attackEvents = step(sim, { attackPressed: true });
        let hit = attackEvents.some((e) => e.type === 'enemy_hit');
        const laterEvents = [];
        for (let i = 0; i < 20 && !hit; i++) {
            const evs = step(sim);
            laterEvents.push(...evs);
            hit = evs.some((e) => e.type === 'enemy_hit');
        }
        assert.ok(hit && toad.hp < hp, hero + ' attack damages the toad during its call');
        for (let i = 0; i < 3 && toad.toad.state === 'tell'; i++) step(sim);
        assert.equal(toad.toad.state, 'rest', hero + ' interrupts the call into a rest');
        assert.equal(toad.windup, 0, hero + ' clears the call telegraph');
        const retryEvents = [];
        for (let i = 0; i < 145; i++) retryEvents.push(...step(sim));
        assert.ok(retryEvents.some((e) => e.type === 'toad_tell'), hero + ' gets another telegraphed attempt after resting');
        assert.equal(retryEvents.some((e) => e.type === 'toad_wave'), false, hero + ' can interrupt before a wave is emitted');
    });

    // A ground wave can be jumped over by either hero with normal jump input.
    ['knight', 'ranger'].forEach((hero) => {
        const sim = arena(hero);
        const floorY = sim.stage.groundY;
        const toad = spawnEnemy(sim, 'bell_toad', 24.5 * TILE, floorY);
        sim.player.x = toad.x - 340;
        sim.player.y = floorY;
        sim.player.vx = 0;
        sim.player.onGround = true;
        toad.toad = { state: 'sit', cooldown: 0 };
        const log = [];
        for (let i = 0; i < 60 && !log.some((e) => e.type === 'toad_wave'); i++) log.push(...step(sim));
        const waveEvent = log.find((e) => e.type === 'toad_wave');
        assert.ok(waveEvent, hero + ' receives a warned ground wave');
        const wave = sim.projectiles.find((p) => p.kind === 'toad_wave');
        assert.ok(wave && wave.groundHug && wave.fromEnemy, 'the wave uses the normal enemy projectile path');
        assert.equal(wave.y, toad.y - 8, 'the wave spawns from the toad local surface');
        for (let i = 0; i < 42; i++) step(sim);
        assert.ok(wave.x > sim.player.x && wave.x - sim.player.x < 160 && !wave.dead, 'the wave approaches along the floor before the jump');
        step(sim, { jumpPressed: true, jump: true });
        for (let i = 0; i < 45; i++) step(sim, { jump: i < 20 });
        assert.ok(wave.x < sim.player.x - sim.player.w, hero + ' clears the wave through a normal jump');
        assert.equal(sim.damageTaken, 0, hero + ' takes no damage when jumping the low wave');
    });

    // A grounded hit uses the toad wave's flesh impact cue.
    {
        const sim = arena('knight');
        const toad = spawnEnemy(sim, 'bell_toad', 20.5 * TILE, sim.stage.groundY);
        sim.player.x = toad.x - 90;
        sim.player.y = sim.stage.groundY;
        sim.player.onGround = true;
        toad.toad = { state: 'sit', cooldown: 0 };
        let hit = null;
        for (let i = 0; i < 100 && !hit; i++) hit = step(sim).find((e) => e.type === 'player_hurt') || null;
        assert.ok(hit, 'an unjumped ground wave hits the player');
        assert.equal(hit.detail.sound, 'impact-flesh', 'the wave does not reuse a metal projectile impact');
    }

    // The wave keeps the toad's raised local height and ends at a drop or a blocking wall.
    ['pit', 'wall'].forEach((obstacle) => {
        const sim = arena('knight', 12);
        const surfaceY = 12 * TILE;
        const toad = spawnEnemy(sim, 'bell_toad', 20.5 * TILE, surfaceY);
        sim.player.x = toad.x + 300;
        sim.player.y = surfaceY;
        sim.player.onGround = true;
        toad.toad = { state: 'sit', cooldown: 0 };
        const obstacleTile = 23;
        if (obstacle === 'pit') delete sim.stage.solidMap[obstacleTile + ',12'];
        else sim.stage.solidMap[obstacleTile + ',11'] = true;
        let wave = null;
        for (let i = 0; i < 60 && !wave; i++) {
            step(sim);
            wave = sim.projectiles.find((p) => p.kind === 'toad_wave') || null;
        }
        assert.ok(wave, 'the toad releases a wave toward the ' + obstacle);
        assert.equal(wave.y, surfaceY - 8, 'the wave tracks the local raised floor instead of stage.groundY');
        for (let i = 0; i < 90 && !wave.dead; i++) step(sim);
        assert.equal(wave.dead, true, 'the wave ends safely at the ' + obstacle);
        assert.equal(sim.damageTaken, 0, 'the wave cannot hit across the ' + obstacle);
    });
}
console.log('Runenpfad round 6 checks passed: lantern moth, bell toad, and counters.');
