// Developer CLI: searches a playable route to the cup for AuraGolf courses and verifies it by replay.
//
//   node scripts/minigolf-route-search.mjs [--dir assets/minigolf/build/p3] [--course pirate-04] [--json]
//
// --dir     directory with manifest.json (version 1 or 2) and the course GLBs; default assets/minigolf/build/p3
// --course  one course: its id, its number, or <world|look>-<NN> (for example temple-06); default all courses
// --json    print one JSON document instead of the readable report
//
// Without a route the report shows the best resting position and the distance left per depth, which helps
// when tuning jump widths, ramps or cannon arcs. Exit status 1 when any course has no verified route.
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createPhysics } from '../ui/js/desktop/apps/minigolf-physics.js';
import { findRoute, loadCourses, replayRoute, timedRoute } from './lib/minigolf-routes.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const args = process.argv.slice(2);
const option = name => {
    const index = args.indexOf(name);
    if (index < 0) return null;
    const value = args[index + 1];
    if (!value || value.startsWith('--')) { console.error(`${name} needs a value`); process.exit(2); }
    return value;
};
const known = new Set(['--dir', '--course', '--json']);
for (const arg of args) if (arg.startsWith('--') && !known.has(arg)) { console.error(`unknown option ${arg}\nusage: node scripts/minigolf-route-search.mjs [--dir DIR] [--course ID] [--json]`); process.exit(2); }
const json = args.includes('--json');
const dir = path.resolve(option('--dir') ?? path.join(root, 'assets/minigolf/build/p3'));
const wanted = option('--course');

const pad = value => String(value).padStart(2, '0');
const matches = (course, name) => course.id === name || String(course.number) === name || [course.world, course.look].some(group => group && `${group}-${pad(course.number)}` === name);

const { manifest, courses: all } = await loadCourses(dir);
const courses = wanted ? all.filter(course => matches(course, wanted)) : all;
if (!courses.length) { console.error(`no course matches "${wanted}" in ${dir} (ids: ${all.map(course => course.id).join(', ')})`); process.exit(2); }
if (!json) console.log(`manifest v${manifest.version}, ${courses.length} course${courses.length === 1 ? '' : 's'} from ${dir}`);

const results = [];
for (const course of courses) {
    const events = [], trace = [];
    const physics = await createPhysics(course, { onEvent: event => events.push(event) });
    const started = performance.now();
    let result;
    try {
        const found = findRoute(physics, course, events, { trace });
        const route = timedRoute(physics, course, found, events);
        const replay = route ? replayRoute(physics, course, route, events) : null;
        result = { id: course.id, number: course.number, par: course.par ?? null, found: !!found, verified: !!replay?.ok, shots: route ? route.length : found ? found.length : null, route, trace };
    } finally {
        physics.dispose();
    }
    result.seconds = Math.round((performance.now() - started) / 100) / 10;
    results.push(result);
    if (json) continue;
    if (result.verified) {
        console.log(`${result.id}: route found, ${result.shots} shot${result.shots === 1 ? '' : 's'} (par ${result.par}) in ${result.seconds} s, replay ok`);
        result.route.forEach((shot, index) => console.log(`  shot ${index + 1}: angle ${shot.angle.toFixed(4)} rad (${(shot.angle * 180 / Math.PI).toFixed(1)} deg), power ${shot.power}${shot.wait ? `, wait ${shot.wait.toFixed(3)} s` : ''}`));
        console.log(`  route ${JSON.stringify(result.route)}`);
    } else {
        console.log(`${result.id}: ${result.found ? 'route found but the timed replay failed' : 'no route found'} in ${result.seconds} s (par ${result.par})`);
        for (const level of trace) console.log(`  depth ${level.depth}: ${level.states} resting positions${level.best ? `, best ${level.best.remaining.toFixed(2)} m left at [${level.best.position.map(value => value.toFixed(2)).join(', ')}] after ${level.best.shots} shot${level.best.shots === 1 ? '' : 's'}` : ''}`);
    }
}
if (json) console.log(JSON.stringify(results, null, 2));
process.exit(results.every(result => result.verified) ? 0 : 1);
