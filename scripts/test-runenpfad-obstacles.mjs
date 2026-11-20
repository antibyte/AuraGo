import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON, Date };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);

for (const file of [
    'biomes.js', 'fx.js', 'physics.js', 'behaviors.js', 'bosses.js', 'variety.js', 'profile.js', 'runes.js',
    'zones.js', 'zones-extra.js', 'gauntlets.js', 'swing.js', 'puzzles.js', 'dragons.js', 'npcs.js', 'npc-story.js',
    'npc-story-late.js', 'dialogue.js', 'trade.js', 'levels.js', 'combat.js', 'sim.js'
]) {
    const abs = path.join(rootDir, 'ui/js/desktop/apps/runenpfad', file);
    vm.runInContext(fs.readFileSync(abs, 'utf8'), sandbox, { filename: abs });
}

const Levels = sandbox.RunenpfadLevels;
const Sim = sandbox.RunenpfadSim;
const Extra = sandbox.RunenpfadZonesExtra;
const campaign = (id) => Levels.buildStage(id);
const endless = (floor) => Levels.buildStage(Levels.createEndlessConfig(floor, 4242));

for (const [id, kind] of [[3, 'thorn_roller'], [17, 'thorn_roller'], [6, 'moon_steps'], [21, 'moon_steps']]) {
    const stage = campaign(id);
    assert.ok(Levels.CAMPAIGN_LEVELS[id - 1].zones.includes(kind), `campaign ${id} lists ${kind}`);
    if (kind === 'thorn_roller') assert.equal(stage.thornRollers.length, 1, `campaign ${id} builds one roller`);
    else assert.equal(stage.moonSteps.length, 3, `campaign ${id} builds three moon steps`);
}

for (const floor of [4, 10, 16]) {
    const stage = endless(floor);
    assert.ok(stage.moonSteps.length === 3, `endless ${floor} builds three steps`);
    assert.deepEqual(stage.moonSteps.map((p) => [p.x, p.y]), endless(floor).moonSteps.map((p) => [p.x, p.y]), 'moon route is deterministic');
}
for (const floor of [5, 11, 17]) assert.equal(endless(floor).thornRollers.length, 1, `endless ${floor} builds one roller`);
for (const floor of [3, 6, 9, 12, 15]) {
    const stage = endless(floor);
    assert.equal(stage.moonSteps.length + stage.thornRollers.length, 0, `boss floor ${floor} stays clear`);
}

const TILE = 48;
function assertGroundRoute(stage, x0, x1) {
    const row = stage.groundY / TILE;
    for (let tx = Math.floor(x0 / TILE); tx <= Math.floor(x1 / TILE); tx++) {
        assert.ok(stage.solidMap[tx + ',' + row], `ground route remains solid at column ${tx}`);
    }
}

const rollerStage = campaign(3);
const roller = rollerStage.thornRollers[0];
assert.ok(roller.endX - roller.startX > TILE * 5 && roller.endX - roller.startX < TILE * 8, 'roller lane is short and bounded');
assertGroundRoute(rollerStage, roller.startX - roller.r, roller.endX + roller.r);

const moonStage = campaign(6);
assert.equal(moonStage.moonSteps.length, 3);
for (const p of moonStage.moonSteps) {
    assert.ok(moonStage.platforms.includes(p), 'moon state is the same object used by collision');
    assert.equal(p.w, 144);
    assert.equal(p.solid, false, 'steps start non-solid');
}
assertGroundRoute(moonStage, moonStage.moonSteps[0].routeStartX, moonStage.moonSteps[2].x + moonStage.moonSteps[2].w);
assert.ok(moonStage.collectibles.filter((c) => c.kind === 'coin' && c.x >= moonStage.moonSteps[0].routeStartX && c.x < moonStage.moonSteps[0].routeStartX + 18 * TILE).length >= 6, 'optional steps carry a small coin reward');

const moonSim = Sim.createSimulation({ mode: 'campaign', levelId: 6, hero: 'knight' });
moonSim.stage.enemies.length = 0;
moonSim.stage.hazards.length = 0;
const [step0, step1, step2] = moonSim.stage.moonSteps;
moonSim.elapsed = step0.offset + 1;
Extra.step(moonSim, 1 / 60);
assert.equal(step0.state, 'solid');
assert.equal(step0.solid, true);
moonSim.elapsed = step0.offset + 2.7;
Extra.step(moonSim, 1 / 60);
assert.equal(step0.state, 'warning');
assert.equal(step0.solid, true, 'warning remains landable');
moonSim.player.onGround = true;
moonSim.player.onPlatformRide = step0;
moonSim.elapsed = step0.offset + 3.4;
Extra.step(moonSim, 1 / 60);
assert.equal(step0.solid, true, 'occupied step does not vanish under the hero');
moonSim.elapsed = step0.offset + 8.6 + 0.6;
Extra.step(moonSim, 1 / 60);
assert.equal(step0.state, 'warning', 'occupied step stays marked as safe through the next rising phase');
assert.equal(step0.solid, true, 'occupied step remains solid across the cycle boundary');
moonSim.player.onGround = false;
moonSim.player.onPlatformRide = null;
Extra.step(moonSim, 1 / 60);
assert.equal(step0.solid, false, 'step resumes its rising phase after the hero leaves');
assert.equal(step0.state, 'rising');
moonSim.elapsed = step1.offset + 1;
Extra.step(moonSim, 1 / 60);
assert.equal(step1.state, 'solid');
moonSim.elapsed = step2.offset + 1;
Extra.step(moonSim, 1 / 60);
assert.equal(step2.state, 'solid');

// A real sim landing carries the moon contact and emits the pan-capable cue once per landing.
const landingSim = Sim.createSimulation({ mode: 'campaign', levelId: 6, hero: 'knight' });
landingSim.stage.enemies.length = 0;
landingSim.stage.hazards.length = 0;
const landingStep = landingSim.stage.moonSteps[0];
landingSim.elapsed = landingStep.offset + 1;
landingSim.player.x = landingStep.x + landingStep.w / 2;
landingSim.player.y = landingStep.y - 1;
landingSim.player.vy = 300;
landingSim.player.onGround = false;
const landingEvents = Sim.stepSimulation(landingSim, Sim.STEP, {});
assert.ok(landingEvents.some((e) => e.type === 'land' && e.detail.moonStep), 'land identifies its moon-step contact');
assert.ok(landingEvents.some((e) => e.type === 'moon_step' && e.detail.id === landingStep.id && e.detail.impactSpeed > 0), 'moon landing cue carries id and impact speed');

const hitSim = Sim.createSimulation({ mode: 'campaign', levelId: 3, hero: 'knight' });
hitSim.stage.enemies.length = 0;
const hitRoller = hitSim.stage.thornRollers[0];
hitRoller.speed = 0;
hitSim.player.x = hitRoller.x;
hitSim.player.y = hitRoller.y - 15;
hitSim.player.hp = 3;
Extra.step(hitSim, 0);
assert.equal(hitSim.player.hp, 2, 'the circle centre damages the hero');
hitSim.player.hp = 3;
hitSim.player.invulnTimer = 0;
hitSim.player.x = hitRoller.x + 32;
hitSim.player.y = hitRoller.y - 15;
Extra.step(hitSim, 0);
assert.equal(hitSim.player.hp, 3, 'a bounding-box-only upper corner does not hit at the same height');

const turnSim = Sim.createSimulation({ mode: 'campaign', levelId: 3, hero: 'knight' });
turnSim.stage.enemies.length = 0;
const turnRoller = turnSim.stage.thornRollers[0];
turnRoller.speed = 112;
turnRoller.x = turnRoller.endX - 1;
turnSim.player.x = turnRoller.endX;
turnSim.player.y = turnSim.stage.groundY - 260;
let events = Extra.step(turnSim, 1 / 30) || [];
assert.ok(turnSim.events.some((e) => e.type === 'roller_turn'), 'near direction change emits one turn event');
turnSim.events.length = 0;
Extra.step(turnSim, 1 / 30);
assert.equal(turnSim.events.some((e) => e.type === 'roller_turn'), false, 'a turn event does not repeat while travelling away');
turnRoller.x = turnRoller.endX - 1;
turnRoller.dir = 1;
turnSim.player.x = 0;
turnSim.events.length = 0;
Extra.step(turnSim, 1 / 30);
assert.equal(turnSim.events.some((e) => e.type === 'roller_turn'), false, 'distant turn does not emit an event');

console.log('Runenpfad obstacle tests passed: placement, safe route, moon timing/landing and roller circle hits/turn cues.');
