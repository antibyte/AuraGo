import { createContext, runInContext } from 'node:vm';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const context = {
    console, Math, Number, JSON, Object, Array, Error, TypeError, Infinity
};
context.globalThis = context;
context.window = context;
createContext(context);
for (const name of ['world.js', 'sim.js']) {
    const code = readFileSync(join(root, 'ui/js/desktop/apps/leuchtfeuer', name), 'utf8');
    runInContext(code, context, { filename: name });
}
const World = context.LeuchtfeuerWorld;
const Sim = context.LeuchtfeuerSim;
const manifest = JSON.parse(readFileSync(join(root, 'internal/gamemaker/asset_packs/aurago-pirates-3d/manifest.json'), 'utf8'));

let failed = 0;
function check(name, ok) {
    if (!ok) {
        failed += 1;
        console.error('FAIL', name);
    }
}
function near(a, b, eps = 1e-6) {
    return Math.abs(a - b) <= eps;
}

const world = World.createWorld(manifest);
check('allowlist', world.models.length === 16);
const barrel = World.assetOf(manifest, 'equipment-barrel');
const oil = world.spots.find(spot => spot.part === 'oil');
const box = barrel.collider;
const centerY = oil.y + box.center[1];
const halfY = box.size[1] / 2;
const gap = Math.max(0, oil.y - (centerY + halfY), (centerY - halfY) - oil.y);
check('barrel within 1.2m', gap <= 1.2 && gap < 0.2);
check('deck joins gangway', world.surfaces.find(s => s.id === 'gangway').maxX === world.surfaces.find(s => s.id === 'deck').minX);

const lookNorth = Sim.wishFromCamera(0, 1, 0, 1);
check('W toward camera +Z', near(lookNorth.x, 0) && near(lookNorth.z, 1));
const rightNorth = Sim.wishFromCamera(0, 1, 1, 0);
check('D is screen right when looking +Z', near(rightNorth.x, -1) && near(rightNorth.z, 0));
const lookSouth = Sim.wishFromCamera(0, -1, 0, 1);
check('W toward camera -Z', near(lookSouth.x, 0) && near(lookSouth.z, -1));
const rightSouth = Sim.wishFromCamera(0, -1, 1, 0);
check('D is +X when looking -Z', near(rightSouth.x, 1) && near(rightSouth.z, 0));
const behind = Sim.cameraOffset(0, 8);
check('camera sits on -Z', near(behind.x, 0) && near(behind.z, -8));
check('face +Z', near(Sim.yawFacing(0, 1), Math.PI));
check('face -Z', near(Sim.yawFacing(0, -1), 0));
check('face +X', near(Sim.yawFacing(1, 0), -Math.PI / 2));

const sim = Sim.createSim(world);
const forward = { wish: { x: 0, z: 1 }, interact: false };
for (let i = 0; i < 20; i++) Sim.step(sim, forward, 1 / 60);
check('walk stays on the beach or pier', sim.z > 9 && sim.splashed === false && sim.y > 0);

const edge = Sim.createSim(world);
edge.x = 5.9;
edge.z = 9;
edge.y = 0.35;
edge.parts.glass = true;
let sawSplash = false;
const wishEast = { wish: Sim.wishFromCamera(1, 0, 0, 1), interact: false };
for (let i = 0; i < 30; i++) {
    Sim.step(edge, wishEast, 1 / 60);
    if (edge.splashed) {
        sawSplash = near(edge.x, 0) && near(edge.z, 9) && edge.parts.glass === true;
        break;
    }
}
check('water returns to the beach', sawSplash);

function at(part) {
    const spot = world.spots.find(item => item.part === part);
    const state = Sim.createSim(world);
    state.x = spot.x;
    state.y = spot.y;
    state.z = spot.z;
    return state;
}
const chest = at('glass');
Sim.interact(chest);
Sim.interact(chest);
check('glass once', chest.parts.glass === true && chest.parts.wick === false);
const far = Sim.createSim(world);
Sim.interact(far);
check('too far', far.parts.glass === false && far.notice === 'leuchtfeuer.notice.far');
const wick = at('wick');
Sim.interact(wick);
check('wick', wick.parts.wick === true);
const oilSim = at('oil');
Sim.interact(oilSim);
check('oil', oilSim.parts.oil === true);

const lamp = at('lamp');
Sim.interact(lamp);
check('lamp lists missing', lamp.lit === false && lamp.missing.join() === 'glass,wick,oil');
lamp.parts = { glass: true, wick: true, oil: true };
Sim.interact(lamp);
const lit = lamp.lit;
Sim.interact(lamp);
check('lamp lights once', lit === true && lamp.lit === true);

const guard = Sim.createSim(world);
const pose = Sim.patrolPose(world.guards[0], 0);
guard.x = pose.x;
guard.y = pose.y;
guard.z = pose.z;
guard.parts.glass = true;
Sim.step(guard, { wish: { x: 0, z: 0 }, interact: false }, 1 / 60);
check('guard sends you back with the glass', guard.bumped === true && guard.parts.glass === true && near(guard.x, 0) && near(guard.z, 9));
const period = Sim.patrolPeriod(world.guards[0]);
const start = Sim.patrolPose(world.guards[0], 0.2);
const again = Sim.patrolPose(world.guards[0], 0.2 + period);
check('patrol loops', near(start.z, again.z) && near(start.x, again.x));
const moved = Sim.patrolPose(world.guards[0], 2);
check('patrol moves', Math.abs(moved.z - start.z) > 0.5);

const round = Sim.createSim(world);
round.x = 0.2;
round.y = 1.07;
round.z = 16;
round.parts.glass = true;
round.parts.oil = true;
round.time = 4;
const saved = Sim.exportState(round);
const other = Sim.createSim(world);
Sim.importState(other, saved);
check('round trip', other.parts.glass && other.parts.oil && !other.parts.wick && near(other.z, 16));
Sim.importState(other, { v: 1, x: 0, y: 0, z: 0, yaw: 0, time: 0, lit: false, parts: { glass: true, rope: true } });
check('unknown part resets', near(other.x, 0) && other.parts.glass === false && near(other.z, 9));
Sim.importState(other, { v: 1, x: NaN, y: 0, z: 0, yaw: 0, time: 0, lit: false, parts: {} });
check('NaN resets', near(other.z, 9));
Sim.importState(other, { v: 1, x: 80, y: 0, z: 0, yaw: 0, time: 0, lit: false, parts: {} });
check('off the map resets', near(other.z, 9) && other.x === 0);

const tavern = Sim.createSim(world);
tavern.x = 3;
tavern.y = 1.15;
tavern.z = 0;
for (let i = 0; i < 80; i++) Sim.step(tavern, { wish: { x: 1, z: 0 }, interact: false }, 1 / 60);
check('tavern stops the walk', tavern.x < 4.6 && tavern.splashed === false);

if (failed) {
    console.error(failed + ' failed');
    process.exit(1);
}
console.log('leuchtfeuer sim ok');
