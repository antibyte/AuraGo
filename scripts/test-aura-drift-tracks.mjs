/* Aura Drift - track layout, validation, streaming data and surface checks.
 * Run: node scripts/test-aura-drift-tracks.mjs
 */
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const sandbox = { window: {}, console };
vm.createContext(sandbox);
for (const file of [
    'ui/js/desktop/apps/aura-drift-core.js',
    'ui/js/desktop/apps/aura-drift-style.js',
    'ui/js/desktop/apps/aura-drift-track.js'
]) {
    vm.runInContext(readFileSync(path.join(root, file), 'utf8'), sandbox, { filename: file });
}
const AD = sandbox.window.AuraDriftCore;
const failures = [];
function check(name, condition, detail) {
    if (condition) console.log('  ok  ' + name);
    else { failures.push(name + (detail ? ' — ' + detail : '')); console.log(' FAIL ' + name + (detail ? ' — ' + detail : '')); }
}

console.log('Aura Drift tracks');
const built = [];
for (const manifest of AD.TRACKS) {
    const track = AD.buildTrack(manifest);
    const validation = AD.validateTrack(track);
    built.push(track);
    check(manifest.id + ' passes the drivability validator', validation.ok, validation.reasons.join('; '));
    check(manifest.id + ' is a large track', track.length > 1500, 'length=' + Math.round(track.length));
    check(manifest.id + ' has twelve checkpoints', track.gates.length === 12);
    const sample = AD.sampleTrack(track, track.samples[0].x, track.samples[0].y);
    check(manifest.id + ' centreline sample resolves', Math.abs(sample.offset) < 2, 'offset=' + sample.offset.toFixed(2));
    const point = AD.pointAt(track, track.length * 0.37);
    const roundTrip = AD.sampleTrack(track, point.x, point.y);
    check(manifest.id + ' spline lookup round-trips', Math.abs(roundTrip.s - track.length * 0.37) < 12,
        'delta=' + Math.abs(roundTrip.s - track.length * 0.37).toFixed(2));
    const props = AD.chunkProps(track, 1, 1);
    check(manifest.id + ' chunk props avoid the road', props.every(p => {
        const s = AD.sampleTrack(track, p.x, p.y);
        return Math.abs(s.offset) >= s.halfWidth + 7;
    }));
}

const signatures = built.map(t => t.points.map(p => Math.round(p[0]) + ':' + Math.round(p[1])).join('|'));
check('every layout is genuinely distinct', new Set(signatures).size === AD.TRACKS.length);
const widths = built.map(t => t.width);
check('tracks differ in width', new Set(widths).size >= 4, widths.join(','));
const baseSurfaces = built.map(t => t.biome.base);
check('tracks differ in grip profile', new Set(baseSurfaces).size >= 4, baseSurfaces.join(','));

/* Drift zones are derived from the geometry: they must sit inside the lap
   and they must exist on every layout, otherwise the style score has no
   corner rewards. */
let zoneOk = true, zoneReasons = [];
for (const track of built) {
    if (!track.driftZones.length) { zoneOk = false; zoneReasons.push(track.id + ': none'); continue; }
    for (const zone of track.driftZones) {
        if (zone.from < 0 || zone.to > track.length || zone.to < zone.from) {
            zoneOk = false; zoneReasons.push(track.id + ': ' + zone.from.toFixed(0) + '-' + zone.to.toFixed(0));
        }
    }
}
check('every layout has drift zones inside the lap', zoneOk, zoneReasons.slice(0, 3).join(' | '));

let sectorOk = true, sectorReasons = [];
for (const track of built) {
    if (!track.sectors || track.sectors.length !== 2) { sectorOk = false; sectorReasons.push(track.id + ': splits'); continue; }
    if (track.sectors[1] <= track.sectors[0]) { sectorOk = false; sectorReasons.push(track.id + ': order'); }
}
check('every layout is split into three sectors', sectorOk, sectorReasons.slice(0, 3).join(' | '));

let dailyOk = true, dailyReasons = [];
for (let seed = 1; seed <= 20; seed++) {
    const manifest = AD.dailyTrack(seed * 7919);
    const track = AD.buildTrack(manifest, manifest.seed);
    const result = AD.validateTrack(track);
    if (!result.ok) { dailyOk = false; dailyReasons.push(seed + ': ' + result.reasons.join('; ')); }
}
check('twenty generated daily routes are drivable', dailyOk, dailyReasons.slice(0, 3).join(' | '));

const wet = AD.surfaceAt(built[2], 0, 0, { halfWidth: 10 });
check('harbour uses a low-grip wet surface', wet.grip < 0.8, 'grip=' + wet.grip);
const runoff = AD.surfaceAt(built[0], 0, 40, { halfWidth: 11 });
check('runoff swaps to the biome surface', runoff.type === 'gravel', 'type=' + runoff.type);

if (failures.length) {
    console.error('\n' + failures.length + ' track check(s) failed');
    process.exit(1);
}
console.log('\nAll track checks passed.');
