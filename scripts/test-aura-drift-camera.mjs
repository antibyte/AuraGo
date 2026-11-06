/* Aura Drift - steering regression: holding right must move the car to the right
 * of the screen on every track, in every chase mode and in the 2D fallback.
 * Run: node scripts/test-aura-drift-camera.mjs
 */
import { loadSim } from './aura-drift-sim-loader.mjs';

const AD = loadSim(['camera']);
const failures = [];
function check(name, condition, detail) {
    if (condition) console.log('  ok  ' + name);
    else { failures.push(name + (detail ? ' - ' + detail : '')); console.log(' FAIL ' + name + (detail ? ' - ' + detail : '')); }
}

const ASPECT = 16 / 9;
const W = 960, H = 540;
const manifests = AD.TRACKS.map(t => ({ label: t.id, manifest: t, seed: undefined }));
manifests.push({ label: 'daily', manifest: AD.dailyTrack(20261106), seed: 20261106 });

function freshRace(entry) {
    const track = AD.buildTrack(entry.manifest, entry.seed);
    const race = AD.createRace({ track, mode: 'time_trial', opponents: 0, weapons: false });
    race.state = 'running';
    race.countdown = 0;
    return race;
}

function stepFor(race, steps, input) {
    Object.assign(race.playerInput, AD.createInput(), input);
    for (let i = 0; i < steps; i++) AD.stepRace(race, AD.STEP);
    race.events.length = 0;
}

/* Three-space helpers: [x, up, y_sim]. */
const carPoint = v => [v.x, v.z + 0.5, v.y];
const ahead = (v, d) => [v.x + Math.cos(v.psi) * d, v.z + 0.5, v.y + Math.sin(v.psi) * d];
const project = (pose, p) => AD.projectToScreen(pose, p[0], p[1], p[2], ASPECT);

function settledPose(camera, race, reduced) {
    let pose = null;
    for (let i = 0; i < 240; i++) pose = camera.update(race, 1 / 60, { reduced });
    return pose;
}

console.log('Aura Drift camera and steering');

/* --- 3D chase: every track x every mode x both directions ------------------ */
let startHeadings = [];
for (const entry of manifests) {
    for (const mode of AD.CAMERA_MODES) {
        const results = {};
        for (const dir of [1, -1]) {
            const race = freshRace(entry);
            const v = race.cars[0].v;
            if (mode === AD.CAMERA_MODES[0] && dir === 1) startHeadings.push(entry.label + ':' + v.psi.toFixed(2));
            stepFor(race, 120, { throttle: 1 });
            const camera = AD.createChaseCamera({ mode });
            const pose0 = settledPose(camera, race, false);
            const aim0 = ahead(v, 30);
            const carDepth = project(pose0, carPoint(v)).depth;
            let noseUp = true;
            if (carDepth > 0) noseUp = project(pose0, ahead(v, 5)).y > project(pose0, carPoint(v)).y;
            const centred = Math.abs(project(pose0, aim0).x) < 0.2;
            stepFor(race, 60, { throttle: 1, steer: dir });
            const aim1 = ahead(v, 30);
            results[dir] = { noseUp, centred, x0: project(pose0, aim0).x, x1: project(pose0, aim1).x };
        }
        const label = entry.label + '/' + mode;
        check(label + ' nose points up the screen', results[1].noseUp && results[-1].noseUp);
        check(label + ' straight aim is centred', results[1].centred && results[-1].centred,
            'x=' + results[1].x0.toFixed(3));
        check(label + ' steer right moves the aim right', results[1].x1 > results[1].x0 + 0.01,
            results[1].x0.toFixed(3) + ' -> ' + results[1].x1.toFixed(3));
        check(label + ' steer left moves the aim left', results[-1].x1 < results[-1].x0 - 0.01,
            results[-1].x0.toFixed(3) + ' -> ' + results[-1].x1.toFixed(3));
    }
}
console.log('  start headings: ' + startHeadings.join(' '));

/* --- 2D fallback: same procedure through project2D ---------------------------- */
for (const entry of manifests) {
    const results = {};
    for (const dir of [1, -1]) {
        const race = freshRace(entry);
        const v = race.cars[0].v;
        stepFor(race, 120, { throttle: 1 });
        const camera = AD.createChaseCamera({ mode: 'chase' });
        const pose0 = settledPose(camera, race, false);
        const point = p => AD.project2D(pose0, p[0], p[2], W, H);   /* three [x,up,y] -> sim (x,y) */
        const nose = point(ahead(v, 5)), car = point(carPoint(v));
        const aim0 = point(ahead(v, 30));
        stepFor(race, 60, { throttle: 1, steer: dir });
        const aim1 = point(ahead(v, 30));
        results[dir] = { noseUp: nose[1] < car[1], x0: aim0[0], x1: aim1[0] };
    }
    check(entry.label + ' 2D nose maps to screen up', results[1].noseUp && results[-1].noseUp);
    check(entry.label + ' 2D steer right moves the aim to larger sx', results[1].x1 > results[1].x0 + 1,
        results[1].x0.toFixed(1) + ' -> ' + results[1].x1.toFixed(1));
    check(entry.label + ' 2D steer left moves the aim to smaller sx', results[-1].x1 < results[-1].x0 - 1,
        results[-1].x0.toFixed(1) + ' -> ' + results[-1].x1.toFixed(1));
}

/* The heading every track starts with is pi/2 (down the old north-up screen):
   world +x, the car's left, must land on smaller sx. */
{
    const theta = AD.screenRotation(Math.PI / 2);
    check('screenRotation(pi/2) = -pi', Math.abs(theta + Math.PI) < 1e-9, String(theta));
    const origin = AD.project2D({ x: 0, y: 0, scale: 5, heading: Math.PI / 2 }, 0, 0, W, H);
    const left = AD.project2D({ x: 0, y: 0, scale: 5, heading: Math.PI / 2 }, 10, 0, W, H);
    const forward = AD.project2D({ x: 0, y: 0, scale: 5, heading: Math.PI / 2 }, 0, 10, W, H);
    check('heading pi/2: world +x (car left) maps to smaller sx', left[0] < origin[0] - 1, left[0] + ' vs ' + origin[0]);
    check('heading pi/2: world +y (nose) maps to screen up', forward[1] < origin[1] - 1, forward[1] + ' vs ' + origin[1]);
}

/* --- reverse keeps the key direction ------------------------------------------ */
for (const dir of [1, -1]) {
    const race = freshRace(manifests[0]);
    const v = race.cars[0].v;
    const psi0 = v.psi;
    stepFor(race, 240, { brake: 1, steer: dir });
    check('reverse engages (dir ' + dir + ')', v.vx < -0.5, 'vx=' + v.vx.toFixed(2));
    const turned = AD.wrapAngle(v.psi - psi0);
    check('reversing with steer ' + dir + ' rotates the nose the way the key points', turned * dir > 0.02, 'dpsi=' + turned.toFixed(3));
    const camera = AD.createChaseCamera({ mode: 'chase' });
    const pose = settledPose(camera, race, false);
    check('reversing camera keeps the nose up (dir ' + dir + ')', project(pose, ahead(v, 5)).y > project(pose, carPoint(v)).y);
}

/* --- heading follow ---------------------------------------------------------- */
{
    const v = { x: 0, y: 0, z: 0, psi: 0, vx: 30, vy: 0, airborne: false };
    const st = { h: 0 };
    const params = { velBlend: 0.45, follow: 5.5 };
    v.psi = 0.8;
    const h1 = AD.chaseHeading(st, v, 1 / 60, params);
    check('heading follow is finite and lags a sudden turn', Number.isFinite(h1) && h1 > 0 && h1 < 0.8, 'h=' + h1);
    /* Drift: nose at 0.5 rad, travel along 0. Camera settles between the two. */
    const d = { x: 0, y: 0, z: 0, psi: 0.5, vx: 30 * Math.cos(0.5), vy: -30 * Math.sin(0.5), airborne: false };
    const sd = { h: 0.5 };
    for (let i = 0; i < 600; i++) AD.chaseHeading(sd, d, 1 / 60, params);
    check('heading lags the nose during a drift', sd.h > 0.05 && sd.h < 0.45, 'h=' + sd.h);
    /* Spin: the car faces backwards against its travel, the camera follows travel. */
    const s = { x: 0, y: 0, z: 0, psi: 3.0, vx: -12, vy: -15, airborne: false };   /* solver caps vx at -12 */
    const travel = Math.atan2(-12 * Math.sin(3.0) - 15 * Math.cos(3.0), -12 * Math.cos(3.0) + 15 * Math.sin(3.0));
    const ss = { h: 3.0 };
    for (let i = 0; i < 600; i++) AD.chaseHeading(ss, s, 1 / 60, params);
    check('spin: camera follows travel, not the nose', Math.abs(AD.wrapAngle(ss.h - travel)) < 0.05,
        'h=' + ss.h + ' travel=' + travel);
}

/* --- reduced motion ------------------------------------------------------------ */
{
    const v = { x: 10, y: 5, z: 0, psi: 0.3, vx: 40, vy: 0, airborne: false };
    const base = { dist: 9.5, height: 3.7, ahead: 7, targetY: 1.0, fov: 60, velBlend: 0.45, follow: 5.5 };
    const quiet = AD.computeChasePose({ h: 0.3, punch: 1, shake: 1, lift: 0, seed: 7 }, v, Object.assign({ reduced: true }, base));
    const calm = AD.computeChasePose({ h: 0.3, punch: 0, shake: 0, lift: 0, seed: 7 }, v, Object.assign({ reduced: true }, base));
    check('reduced motion has zero shake and roll', quiet.roll === 0 && quiet.eye.every((x, i) => x === calm.eye[i]));
    check('reduced motion has no FOV terms', quiet.fov === 60, 'fov=' + quiet.fov);
    const shaken = AD.computeChasePose({ h: 0.3, punch: 1, shake: 1, lift: 0, seed: 7 }, v, Object.assign({ reduced: false }, base));
    check('full motion shakes and punches the FOV', shaken.fov > 60 && shaken.eye.some((x, i) => x !== calm.eye[i]));
}

/* --- scripts and time warp ------------------------------------------------------ */
{
    const race = freshRace(manifests[0]);
    const v = race.cars[0].v;
    const camera = AD.createChaseCamera({ mode: 'chase' });
    settledPose(camera, race, false);
    camera.script('intro');
    const first = camera.update(race, 1 / 60, { reduced: false });
    const toEye = [first.eye[0] - v.x, first.eye[2] - v.y];
    check('intro starts in front of the car', toEye[0] * Math.cos(camera.heading) + toEye[1] * Math.sin(camera.heading) > 3);
    for (let i = 0; i < 200; i++) camera.update(race, 1 / 60, { reduced: false });
    const after = camera.update(race, 1 / 60, { reduced: false });
    check('intro ends in the chase seat', project(after, ahead(v, 5)).y > project(after, carPoint(v)).y);
    const reduced = AD.createChaseCamera({ mode: 'chase' });
    reduced.script('intro');
    const quietPose = reduced.update(race, 1 / 60, { reduced: true });
    check('reduced motion skips the intro', Math.hypot(quietPose.eye[0] - v.x, quietPose.eye[2] - v.y) < 12);
}
{
    const warp = AD.createTimeWarp({ reduced: false });
    warp.trigger('hitstop');
    const slowed = warp.scale(0.01);
    check('hit-stop slows to 0.1', Math.abs(slowed - 0.001) < 1e-9, String(slowed));
    for (let i = 0; i < 10; i++) warp.scale(0.01);
    check('hit-stop expires', warp.scale(0.01) === 0.01 && warp.value === 1);
    warp.onEvent({ type: 'finish', car: 0 });
    check('finish slows to 0.35', Math.abs(warp.scale(0.1) - 0.035) < 1e-9);
    const calm = AD.createTimeWarp({ reduced: true });
    calm.trigger('slowmo');
    check('reduced motion forces time scale 1', calm.scale(0.02) === 0.02);
}

if (failures.length) {
    console.log('\n' + failures.length + ' failure(s):');
    for (const f of failures) console.log(' - ' + f);
    process.exit(1);
}
console.log('\nAll camera checks passed.');
