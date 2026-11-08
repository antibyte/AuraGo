import assert from 'node:assert/strict';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { TABLES } from '../ui/js/desktop/apps/pinball/tables/index.js';
import { validateTable } from '../ui/js/desktop/apps/pinball/tables/validate.js';
import { copy } from '../ui/js/desktop/apps/pinball/config.js';

// 1. All three stubs validate cleanly.
for (const [id, table] of Object.entries(TABLES)) {
    const result = validateTable(table);
    assert.deepEqual(result.errors, [], id + ' must validate');
    assert.equal(table.layout.id, id);
    assert.ok(table.layout.lamps.length <= 200 && table.layout.flashers.length <= 6);
}

// 2. Mirror symmetry about cx within 1e-9.
for (const [id, { layout }] of Object.entries(TABLES)) {
    const cx = layout.meta.cx, near = (a, b, what) => assert.ok(Math.abs(a - b) < 1e-9, id + ' ' + what + ': ' + a + ' vs ' + b);
    const fl = layout.flippers.find(f => f.id === 'left'), fr = layout.flippers.find(f => f.id === 'right');
    near(fl.x + fr.x, 2 * cx, 'flipper pivots'); near(fl.y, fr.y, 'flipper y');
    assert.equal(fl.side, 1); assert.equal(fr.side, -1);
    const posts = new Map(layout.posts.map(p => [p.id, p])), walls = new Map(layout.walls.map(w => [w.id, w]));
    let checked = 0;
    for (const p of layout.posts) if (p.id.includes('L_p') || p.id === 'dividerPostL') {
        const q = posts.get(p.id.replace('L', 'R')); assert.ok(q, p.id);
        near(p.x + q.x, 2 * cx, p.id); near(p.y, q.y, p.id); checked++;
    }
    for (const w of layout.walls) if (/^(sling|divider|inlaneGuide)L/.test(w.id)) {
        const q = walls.get(w.id.replace('L', 'R')); assert.ok(q, w.id);
        w.points.forEach((pt, i) => { near(pt[0] + q.points[i][0], 2 * cx, w.id); near(pt[1], q.points[i][1], w.id); checked++; });
    }
    assert.ok(checked >= 12, 'mirror check covered the sling vertices');
}

// 3. A broken copy yields exactly the four named errors.
{
    const broken = copy(TABLES.neon);
    const lampTemplate = broken.layout.lamps[0];
    broken.layout.walls.push({ ...broken.layout.walls[0] });                                    // duplicate id
    broken.layout.paths.push({ id: 'bad', kind: 'wire', entry: { a: [0, 0.5], b: [0.03, 0.5] }, points: [[0, 0.5, 0], [0, 0.6, 0.02]], exit: { kind: 'hole', hole: 'nowhere' } }); // dangling exit
    for (let i = 0; broken.layout.lamps.length < 256; i++) broken.layout.lamps.push({ ...lampTemplate, id: 'extra' + i }); // 256 lamps
    broken.layout.walls.push({ id: 'gapA', material: 'metal', points: [[0, 0.40], [0, 0.50]], closed: false, render: 'metal', height: 0.03 });
    broken.layout.walls.push({ id: 'gapB', material: 'metal', points: [[0.015, 0.40], [0.015, 0.50]], closed: false, render: 'metal', height: 0.03 }); // 0.015 m gap
    const result = validateTable(broken);
    assert.deepEqual(result.errors.map(e => e.split(':')[0]).sort(), ['clearance', 'dangling-ref', 'duplicate-id', 'too-many-lamps'], result.errors.join('\n'));
    broken.layout.allowGaps.push(['gapA', 'gapB']);
    assert.ok(!validateTable(broken).errors.some(e => e.startsWith('clearance')), 'allowGaps whitelists the pair');
}

// 4. i18n key scan: every literal dmd_/mode_/lamp_/card_/help_ key in the pinball sources exists in en.json.
{
    const en = JSON.parse(readFileSync(new URL('../ui/lang/desktop/en.json', import.meta.url), 'utf8'));
    const files = [];
    const walk = dir => { for (const name of readdirSync(dir)) { const p = join(dir, name); if (statSync(p).isDirectory()) walk(p); else if (p.endsWith('.js')) files.push(p); } };
    walk(fileURLToPath(new URL('../ui/js/desktop/apps/pinball/', import.meta.url)));
    const missing = [];
    for (const file of files) for (const m of readFileSync(file, 'utf8').matchAll(/['"`]((?:dmd|mode|lamp|card|help)_[a-z0-9_]+)['"`]/g)) {
        if (!('pinball.' + m[1] in en)) missing.push(file.split(/[\\/]pinball[\\/]/)[1] + ': pinball.' + m[1]);
    }
    for (const table of Object.values(TABLES)) for (const key of validateTable(table).keys) if (!('pinball.' + key in en)) missing.push('layout key pinball.' + key);
    assert.deepEqual([...new Set(missing)], [], 'keys missing from en.json');
}
console.log('pinball tables ok');
