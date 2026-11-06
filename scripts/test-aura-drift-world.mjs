/* Aura Drift - pure checks for the 3D world modules (road ribbon, distance field, terrain heights).
 * Run: node scripts/test-aura-drift-world.mjs
 */
import { loadSim } from './aura-drift-sim-loader.mjs';

const AD = loadSim(['look', 'terrain', 'road']);
const failures = [];
function check(name, condition, detail) {
    if (condition) console.log('  ok  ' + name);
    else { failures.push(name + (detail ? ' - ' + detail : '')); console.log(' FAIL ' + name + (detail ? ' - ' + detail : '')); }
}

console.log('Aura Drift world');
const rng = AD.rng(4242);
let totalMs = 0;
for (const manifest of AD.TRACKS) {
    const track = AD.buildTrack(manifest);
    const id = manifest.id;

    /* ribbon closes */
    const rib = AD.roadRibbon(track, 0, 18);
    const last = rib.positions.length / 3 - 2;
    let seam = 0;
    for (let c = 0; c < 2; c++) {
        for (let a = 0; a < 3; a++) seam = Math.max(seam, Math.abs(rib.positions[c * 3 + a] - rib.positions[(last + c) * 3 + a]));
    }
    const vEnd = rib.uvs[(last) * 2 + 1];
    check(id + ' ribbon first and last ring coincide', seam < 0.01, 'gap=' + seam.toFixed(4));
    check(id + ' ribbon v is an integer at the seam', Number.isInteger(vEnd) && vEnd >= 1 && rib.uvs[1] === 0, 'v=' + vEnd);
    check(id + ' ribbon indices stay in range', rib.indices.every(i => i < rib.positions.length / 3));
    const wide = AD.roadRibbon(track, 10, 18);
    const w0 = Math.hypot(wide.positions[0] - wide.positions[3], wide.positions[2] - wide.positions[5]);
    check(id + ' grow widens the ribbon by 2 x grow', Math.abs(w0 - (track.samples[0].halfWidth + 10) * 2) < 0.01, 'w=' + w0.toFixed(2));

    /* distance field */
    const t0 = Date.now();
    const field = AD.buildDistanceField(track, 8);
    const ms = Date.now() - t0;
    totalMs += ms;
    let worst = 0, checked = 0;
    for (let n = 0; n < 400; n++) {
        const s = track.samples[Math.floor(rng() * track.samples.length)];
        const off = (rng() * 2 - 1) * 100;
        const x = s.x + s.nx * off, y = s.y + s.ny * off;
        let ref = Infinity;
        for (const p of track.samples) ref = Math.min(ref, Math.hypot(p.x - x, p.y - y));
        if (ref > 100) continue;
        worst = Math.max(worst, Math.abs(field.at(x, y) - ref));
        if (ref < 55) {
            /* sampleTrack only searches the 3x3 neighbouring cells, so compare it where it is exact */
            const near = AD.sampleTrack(track, x, y);
            worst = Math.max(worst, Math.abs(field.at(x, y) - Math.hypot(x - near.x, y - near.y)));
        }
        checked++;
    }
    check(id + ' distance field agrees with sampleTrack within 8 m', worst < 8 && checked > 100, 'worst=' + worst.toFixed(2) + ' n=' + checked);
    check(id + ' distance field is clamped far away', field.at(track.bounds.maxX + 3000, track.bounds.maxY + 3000) >= 160);

    /* corridor floor */
    let high = -Infinity, bad = 0;
    const smp = {};
    for (let i = 0; i < track.samples.length; i += 5) {
        const s = track.samples[i];
        for (const f of [-0.95, -0.5, 0, 0.5, 0.95]) {
            const x = s.x + s.nx * f * s.halfWidth, y = s.y + s.ny * f * s.halfWidth;
            const dist = field.at(x, y);
            field.info(x, y, smp);
            const h = AD.terrainHeight(manifest.biome, dist, x, y, smp);
            high = Math.max(high, h);
            if (!(h <= -0.2)) bad++;
        }
    }
    check(id + ' terrain is at most -0.2 inside the corridor', bad === 0, 'max=' + high.toFixed(2) + ' bad=' + bad);

    /* hills rise beyond the shoulder and stay finite */
    let top = -Infinity, nan = 0;
    for (let n = 0; n < 600; n++) {
        const s = track.samples[Math.floor(rng() * track.samples.length)];
        const off = (rng() < 0.5 ? -1 : 1) * (20 + rng() * 900);
        const x = s.x + s.nx * off, y = s.y + s.ny * off;
        const dist = field.at(x, y);
        field.info(x, y, smp);
        const h = AD.terrainHeight(manifest.biome, dist, x, y, smp);
        if (!Number.isFinite(h)) nan++;
        top = Math.max(top, h);
    }
    check(id + ' terrain heights are finite', nan === 0);
    const flatBiomes = { harbour: 1, metro: 1 };
    if (!flatBiomes[manifest.biome]) check(id + ' terrain rises off the road', top > 8, 'top=' + top.toFixed(1));

    /* colours are finite and bounded */
    const pal = AD.terrainPalette(manifest.biome, AD.LOOK[manifest.biome]);
    const c = [0, 0, 0];
    let colBad = 0;
    for (let n = 0; n < 300; n++) {
        AD.terrainColor(pal, rng() * 200 - 1, rng() * 1.5, rng() * 2000, rng() * 2000, rng() * 300, c);
        if (!c.every(v => Number.isFinite(v) && v >= 0 && v <= 1.5)) colBad++;
    }
    check(id + ' terrain colours stay in range', colBad === 0);
}
check('distance fields build fast (all eight under 1.5 s in total)', totalMs < 1500, 'total=' + totalMs + ' ms');

/* determinism */
const t1 = AD.buildTrack(AD.TRACKS[0]);
const f1 = AD.buildDistanceField(t1, 8), f2 = AD.buildDistanceField(t1, 8);
const sa = {}, sb = {};
f1.info(t1.samples[10].x + 40, t1.samples[10].y, sa); f2.info(t1.samples[10].x + 40, t1.samples[10].y, sb);
check('terrain height is deterministic', AD.terrainHeight('alpine', f1.at(300, 200), 300, 200, sa) === AD.terrainHeight('alpine', f2.at(300, 200), 300, 200, sb));

if (failures.length) {
    console.log('\n' + failures.length + ' failure(s):');
    for (const f of failures) console.log(' - ' + f);
    process.exit(1);
}
console.log('\nall world checks passed');
