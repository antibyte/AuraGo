import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

const sandbox = { console, Math, Number, String, Boolean, Array, Object, Map, Set, JSON };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);
const file = path.join(rootDir, 'ui/js/desktop/apps/runenpfad/camera-rig.js');
const src = fs.readFileSync(file, 'utf8');
assert.ok(!/\b(document|Math\.random|Date\.now)\b/.test(src.replace(/\/\*.*?\*\//g, '').replace(/\/\/.*$/gm, '')), 'camera-rig stays DOM-free and deterministic');
vm.runInContext(src, sandbox, { filename: file });
const Rig = sandbox.RunenpfadCameraRig;
assert.ok(Rig && typeof Rig.create === 'function', 'RunenpfadCameraRig.create exists');

const DT = 1 / 60;
const VW = 960;
const VH = 540;
const base = { dt: DT, px: 2000, py: 900, facing: 1, vx: 0, vy: 0, grounded: true, climbing: false, vw: VW, vh: VH, minX: 0, maxX: 6000, maxY: 1200 };

function run(rig, frames, patch, step) {
    let o = Object.assign({}, base, patch);
    let r = null;
    for (let i = 0; i < frames; i++) {
        if (step) step(o, i);
        r = rig.update(o);
    }
    return { o, r };
}

// 1. Facing right at rest: hero at 40 % from the left.
{
    const rig = Rig.create();
    const { o, r } = run(rig, 120, {});
    assert.ok(Math.abs((o.px - r.x) / VW - 0.4) < 0.005, 'rest facing right: hero at 40 %');
    assert.ok(Math.abs((o.py - r.y) / VH - 0.62) < 0.005, 'rest: hero at 62 % of the height');
}

// 2. Turn left and walk 1 s: hero at about 60 % from the left.
{
    const rig = Rig.create();
    run(rig, 60, {});
    const { o, r } = run(rig, 60, { facing: -1, vx: -200 }, (oo) => { oo.px -= 200 * DT; });
    const frac = (o.px - r.x) / VW;
    assert.ok(Math.abs(frac - 0.6) < 0.04, 'after walking left for 1 s the hero sits near 60 %, got ' + frac.toFixed(3));
}

// 3. A left-right flick within 0.2 s does not move the lead.
{
    const rig = Rig.create();
    const a = run(rig, 120, {});
    const before = a.r.x;
    run(rig, 6, { facing: -1 });
    const b = run(rig, 60, { facing: 1 });
    assert.ok(Math.abs(b.r.x - before) < 1, 'flick leaves the camera in place, moved ' + Math.abs(b.r.x - before));
}

// 4. A 100 px jump on flat ground changes camera y by less than 8 px.
{
    const rig = Rig.create();
    const y0 = run(rig, 120, {}).r.y;
    let worst = 0;
    const o = Object.assign({}, base, { grounded: false });
    let vy = -632; // v0 for a 100 px rise at g = 2000
    let h = 0;
    for (let i = 0; i < 80 && (i < 2 || h > 0); i++) {
        vy += 2000 * DT;
        h = Math.max(0, h - vy * DT);
        o.vy = vy;
        o.py = 900 - h;
        o.grounded = h === 0;
        worst = Math.max(worst, Math.abs(rig.update(o).y - y0));
    }
    assert.ok(worst < 8, 'jump bob under 8 px, got ' + worst.toFixed(2));
}

// 5. A 500 px teleport snaps in one update.
{
    const rig = Rig.create();
    run(rig, 60, {});
    const { o, r } = run(rig, 1, { px: 2500 });
    assert.ok(Math.abs((o.px - r.x) / VW - 0.4) < 0.001, 'teleport snaps to the 40 % anchor');
}

// 6. The result never leaves the clamps (including maxY <= 0).
{
    const rig = Rig.create();
    let ok = true;
    for (let i = 0; i < 600; i++) {
        const px = i < 300 ? -500 + i * 8 : 7000 - (i - 300) * 20;
        const r = rig.update(Object.assign({}, base, { px: px, py: 2000 - i * 5, facing: i % 90 < 45 ? 1 : -1, grounded: i % 3 === 0 }));
        if (r.x < 0 || r.x > 6000 || r.y < 0 || r.y > 1200) ok = false;
    }
    assert.ok(ok, 'camera stays inside minX/maxX/0..maxY');
    const flat = Rig.create();
    const r = flat.update(Object.assign({}, base, { maxY: -40 }));
    assert.equal(r.y, -40, 'maxY <= 0 keeps maxY as the camera y');
}

console.log('PASS runenpfad camera rig');
