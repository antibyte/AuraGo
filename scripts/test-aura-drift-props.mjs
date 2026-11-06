/* Aura Drift - prop placement checks (pure part of aura-drift-props.js and a source hygiene check of sky and props).
 * Run: node scripts/test-aura-drift-props.mjs
 */
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadSim } from './aura-drift-sim-loader.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const AD = loadSim(['look', 'props']);
const manifest = JSON.parse(readFileSync(path.join(root, 'ui/3d/aura-drift/v1/manifest.json'), 'utf8'));
const failures = [];
function check(name, condition, detail) {
    if (condition) console.log('  ok  ' + name);
    else { failures.push(name + (detail ? ' - ' + detail : '')); console.log(' FAIL ' + name + (detail ? ' - ' + detail : '')); }
}

console.log('Aura Drift props');

const tracks = AD.TRACKS.map(m => AD.buildTrack(m));
const daily = [AD.buildTrack(AD.dailyTrack(20261106)), AD.buildTrack(AD.dailyTrack(777))];
const all = tracks.concat(daily);
const metaFor = id => (manifest.props && manifest.props[id]) || {};

/* Same seed, same placements; another seed, other placements. */
let unequal = [];
for (const track of all) {
    const a = JSON.stringify(AD.scatterProps(track, track.biomeId, metaFor(track.biomeId)));
    const b = JSON.stringify(AD.scatterProps(track, track.biomeId, metaFor(track.biomeId)));
    if (a !== b) unequal.push(track.id);
    const f1 = JSON.stringify(AD.furnitureProps(track)), f2 = JSON.stringify(AD.furnitureProps(track));
    if (f1 !== f2) unequal.push(track.id + ' furniture');
}
check('same track and seed give identical placements', unequal.length === 0, unequal.join(', '));
const reseeded = AD.buildTrack(AD.TRACKS[0], 4242);
const base = AD.scatterProps(tracks[0], tracks[0].biomeId, metaFor('alpine'));
const other = AD.scatterProps(reseeded, reseeded.biomeId, metaFor('alpine'));
check('another seed gives other placements', JSON.stringify(base.slice(0, 20)) !== JSON.stringify(other.slice(0, 20)));

/* Nothing inside the corridor, nothing on a road elsewhere on the loop. */
let corridor = [];
let nonFinite = 0;
for (const track of all) {
    const list = AD.scatterProps(track, track.biomeId, metaFor(track.biomeId));
    for (const p of list) {
        if (!isFinite(p.x) || !isFinite(p.y) || !isFinite(p.yaw)) nonFinite++;
        let best = Infinity, hw = 0;
        for (const s of track.samples) {
            const d = Math.hypot(p.x - s.x, p.y - s.y);
            if (d < best) { best = d; hw = s.halfWidth; }
        }
        if (best < hw + 5.4) corridor.push(track.id + ':' + p.name + '@' + best.toFixed(1) + '<' + (hw + 6).toFixed(1));
    }
}
check('no scatter prop within the corridor plus margin (brute force)', corridor.length === 0, corridor.slice(0, 3).join(' | '));
check('every placement is finite', nonFinite === 0);

/* Furniture never stands on the road either, except the gantry and the arches that span it. */
let furnBad = [];
for (const track of all) {
    for (const p of AD.furnitureProps(track)) {
        if (p.name === 'gantry' || p.name === 'inflatable_arch') continue;
        let best = Infinity, hw = 0;
        for (const s of track.samples) { const d = Math.hypot(p.x - s.x, p.y - s.y); if (d < best) { best = d; hw = s.halfWidth; } }
        if (best < hw + 2.5) furnBad.push(track.id + ':' + p.name + '@' + best.toFixed(1));
    }
}
check('furniture stays beside the road', furnBad.length === 0, furnBad.slice(0, 3).join(' | '));

/* Tall props keep the inside of curves clear. */
let sight = [];
for (const track of tracks) {
    const meta = metaFor(track.biomeId);
    for (const p of AD.scatterProps(track, track.biomeId, meta)) {
        if (p.back || !(meta[p.name] && meta[p.name].height > 3)) continue;
        const smp = AD.sampleTrack(track, p.x, p.y);
        const d = Math.hypot(p.x - smp.x, p.y - smp.y);
        if (d < smp.halfWidth + 18 && track.curvature[smp.index] > 0.02) {
            const n = track.samples.length, a = track.samples[smp.index], b = track.samples[(smp.index + 4) % n];
            const cross = a.tx * b.ty - a.ty * b.tx;
            if ((cross > 0 ? 1 : -1) === Math.sign(smp.offset)) sight.push(track.id + ':' + p.name);
        }
    }
}
check('no tall prop on the inner side of a tight curve near the road', sight.length === 0, sight.slice(0, 3).join(' | '));

/* Every biome is dense and shows its own signature props. */
const signature = {
    alpine: ['pine_a', 'rock_a'], desert: ['cactus_a', 'mesa_rock'], harbour: ['container_stack', 'neon_sign', 'fishing_boat'],
    salt: ['salt_crystal'], forest: ['oak', 'birch'], industry: ['pipes', 'silo'], glacier: ['ice_block', 'snow_pine'],
    metro: ['skyscraper_a', 'skyscraper_b', 'skyscraper_c', 'street_lamp']
};
for (const track of tracks) {
    const list = AD.scatterProps(track, track.biomeId, metaFor(track.biomeId));
    const names = new Set(list.map(p => p.name));
    const missing = signature[track.biomeId].filter(n => !names.has(n));
    const perKm = list.length / (track.length / 1000);
    console.log('      ' + track.biomeId + ': ' + list.length + ' placements, ' + perKm.toFixed(0) + ' per km, ' + names.size + ' kinds');
    check(track.biomeId + ' has its signature props', missing.length === 0, missing.join(','));
    check(track.biomeId + ' is dense enough (>= 90 per km)', perKm >= 90, perKm.toFixed(1));
}

/* Furniture contents. */
for (const track of tracks) {
    const f = AD.furnitureProps(track);
    const count = n => f.filter(p => p.name === n).length;
    const brands = new Set(f.filter(p => p.name === 'hoarding').map(p => p.brand));
    const signs = f.filter(p => p.name === 'road_sign');
    const bad = [];
    if (count('gantry') !== 1) bad.push('gantry ' + count('gantry'));
    if (count('grandstand') !== 4) bad.push('grandstands ' + count('grandstand'));
    if (count('inflatable_arch') !== 2) bad.push('arches ' + count('inflatable_arch'));
    if (count('hoarding') < 3) bad.push('hoardings ' + count('hoarding'));
    if (brands.size < 2) bad.push('brands ' + brands.size);
    if (count('hotdog_stand') !== 2 || count('camper_van') !== 4) bad.push('stands/vans');
    if (count('marshal_post') < 4) bad.push('marshals ' + count('marshal_post'));
    if (count('gnome_statue') < 1) bad.push('gnomes');
    if (signs.some(s => !(s.sign >= 0 && s.sign < AD.SIGN_COUNT))) bad.push('sign ids');
    if (f.some(p => p.name === 'hoarding' && !(p.brand >= 0 && p.brand < AD.BRAND_COUNT))) bad.push('brand ids');
    check(track.id + ' furniture complete', bad.length === 0, bad.join('; '));
}

/* The streaming filter honours the cap, the radius and the order. */
const tiers = { low: { radius: 220, cap: 300 }, medium: { radius: 320, cap: 700 }, high: { radius: 480, cap: 1400 }, ultra: { radius: 700, cap: 2200 } };
let capBad = [], orderBad = [], radiusBad = [];
for (const track of tracks) {
    const list = AD.scatterProps(track, track.biomeId, metaFor(track.biomeId)).concat(AD.furnitureProps(track));
    const cells = AD.propIndex(list);
    for (const [tier, lim] of Object.entries(tiers)) {
        for (const frac of [0, 0.21, 0.5, 0.83]) {
            const p = AD.pointAt(track, track.length * frac);
            const view = { x: p.x, y: p.y, fx: p.tx, fy: p.ty };
            for (const cap of [lim.cap, 25, 0]) {
                const sel = AD.filterProps(list, cells, view, { radius: lim.radius, cap: cap, cone: 0.42, near: 70 });
                if (sel.length > cap) capBad.push(track.id + ' ' + tier + ' cap ' + cap + ' got ' + sel.length);
                let prev = -1;
                for (const i of sel) {
                    const d = Math.hypot(list[i].x - view.x, list[i].y - view.y);
                    if (d < prev - 1e-9) orderBad.push(track.id + ' ' + tier);
                    prev = d;
                    if (d > lim.radius * list[i].reach + 1e-6) radiusBad.push(track.id + ' ' + tier);
                }
            }
        }
    }
}
check('filter never exceeds the instance cap', capBad.length === 0, capBad.slice(0, 2).join(' | '));
check('filter returns nearest first', orderBad.length === 0);
check('filter stays inside the tier radius (scaled by reach)', radiusBad.length === 0);
{
    const track = tracks[0], list = AD.scatterProps(track, track.biomeId, metaFor(track.biomeId));
    const p = AD.pointAt(track, 400);
    const sel = AD.filterProps(list, AD.propIndex(list), { x: p.x, y: p.y, fx: p.tx, fy: p.ty }, { radius: 320, cap: 700, cone: 0.42, near: 70 });
    check('a medium window selects a few hundred props, not thousands', sel.length > 40 && sel.length <= 700, String(sel.length));
}

/* Source hygiene for the two 3D modules. */
for (const name of ['sky', 'props']) {
    const file = 'ui/js/desktop/apps/aura-drift-' + name + '.js';
    const text = readFileSync(path.join(root, file), 'utf8');
    check(name + '.js stays below 1000 lines', text.split('\n').length < 1000, String(text.split('\n').length));
    check(name + '.js has no network or dynamic import', !/https?:\/\/|XMLHttpRequest|\bimport\(|\bfetch\(/.test(text));
    check(name + '.js avoids ad-filter words', !/banner|sponsor|billboard|promo|advert|\/ads\//i.test(text));
    check(name + '.js has no integer hash noise in GLSL', !/floatBitsToUint|uint\s+\w+\s*=|<<\s*\d+u/.test(text));
}

if (failures.length) {
    console.log('\n' + failures.length + ' check(s) failed');
    process.exit(1);
}
console.log('\nAll prop checks passed');
