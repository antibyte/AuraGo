import assert from 'node:assert/strict';
import { lampPolygon, polygonArea, polygonCentroid, toUV, texelOf, canvasPx } from '../ui/js/desktop/apps/pinball/lights/shapes.js';
import { buildLampMask } from '../ui/js/desktop/apps/pinball/lights/mask.js';
import { createLamps } from '../ui/js/desktop/apps/pinball/lights/controller.js';
import { SHOWS } from '../ui/js/desktop/apps/pinball/lights/shows.js';
import { SHOWS as SHOW_NAMES } from '../ui/js/desktop/apps/pinball/contracts.js';

const SHAPES = ['circle', 'oval', 'triangle', 'arrow', 'rect', 'roundRect', 'diamond', 'star', 'shield', 'chevron'];

// Every shape: closed ring, positive area, area centroid within 1 mm of the lamp position (also rotated).
for (const shape of SHAPES) for (const angle of [0, 1.1, Math.PI]) {
    const lamp = { id: shape, x: 0.05, y: 0.4, shape, w: 0.02, h: 0.03, angle };
    const poly = lampPolygon(lamp);
    assert.ok(poly.length >= 3 && poly.every(p => p.every(Number.isFinite)), shape + ' vertices');
    assert.ok(polygonArea(poly) > 1e-5, shape + ' area');
    const c = polygonCentroid(poly);
    assert.ok(Math.hypot(c[0] - lamp.x, c[1] - lamp.y) < 0.001, shape + ' centroid');
}
// Angle 0 points +y, angle PI points -y.
const up = lampPolygon({ x: 0, y: 0.5, shape: 'arrow', w: 0.02, h: 0.03, angle: 0 }), down = lampPolygon({ x: 0, y: 0.5, shape: 'arrow', w: 0.02, h: 0.03, angle: Math.PI });
assert.ok(Math.max(...up.map(p => p[1])) > 0.5 && up[0][1] > 0.5 && down[0][1] < 0.5);

// 120 lamps over the playfield, shapes cycling, for the mask/controller tests.
const lamps = [];
for (let i = 0; i < 120; i++) {
    lamps.push({ id: 'l' + i, x: -0.21 + (i % 10) * 0.047, y: 0.12 + Math.floor(i / 10) * 0.075, shape: SHAPES[i % SHAPES.length], w: 0.02, h: 0.02, angle: 0,
        color: '#ff8030', mount: i === 119 ? 'toy' : 'insert', group: 'g' + (i % 4), rgb: i % 7 === 0 });
}
const layout = { lamps, flashers: [{ id: 'fa', x: 0, y: 0.8, h: 0.05, color: '#fff', r: 0.012 }, { id: 'fb', x: 0.1, y: 0.8, h: 0.05, color: '#fff', r: 0.012 }],
    gi: [{ id: 'lower', color: '#ffe0b0', yMin: 0, yMax: 0.5, bulbs: [[0, 0.2]] }, { id: 'upper', color: '#ffe0b0', yMin: 0.5, yMax: 1, bulbs: [[0, 0.7]] }] };

// Mask: centre texel R = index; 3 mm outside the insert G = index and B > 0; far away all zero.
const t0 = process.hrtime.bigint();
const mask = buildLampMask(layout, { width: 1024, height: 2048 });
const ms = Number(process.hrtime.bigint() - t0) / 1e6;
assert.equal(mask.data.length, 1024 * 2048 * 4);
const at = (x, y) => { const [tx, ty] = texelOf(x, y, mask.width, mask.height); const o = (Math.floor(ty) * mask.width + Math.floor(tx)) * 4; return mask.data.subarray(o, o + 4); };
assert.equal(at(lamps[0].x, lamps[0].y)[0], 1);
assert.equal(at(lamps[4].x, lamps[4].y)[0], 5);
const out = at(lamps[0].x + 0.01 + 0.003, lamps[0].y);
assert.equal(out[1], 1); assert.ok(out[2] > 0 && out[0] === 0);
assert.deepEqual([...at(-0.2, 1.0)], [0, 0, 0, 255]);
assert.deepEqual([...at(lamps[119].x, lamps[119].y)].slice(0, 3), [0, 0, 0]);
console.log('mask build ' + ms.toFixed(1) + ' ms for ' + lamps.length + ' lamps');
assert.ok(ms < 150 * 2, 'mask build time ' + ms);

// texelOf and canvasPx agree within 0.5 texel (canvas row 0 is the top, texture row 0 is v=0).
for (const [x, y] of [[0, 0.5], [-0.2, 0.1], [0.2, 1.0]]) {
    const [tx, ty] = texelOf(x, y, 1024, 2048), [cx, cy] = canvasPx(x, y, 1024, 2048), [u, v] = toUV(x, y);
    assert.ok(Math.abs(tx - cx) <= 0.5 && Math.abs(2048 - ty - cy) <= 0.5);
    assert.ok(Math.abs(u * 1024 - tx) < 1e-9 && Math.abs(v * 2048 - ty) < 1e-9);
}

// Fade: on >= 0.95 at 0.1 s, off <= 0.05 at 0.25 s; slow blink is 2 Hz.
const ctl = createLamps(layout);
const modes = new Uint8Array(lamps.length);
modes[0] = 1; modes[1] = 2;
ctl.setModes(modes);
for (let i = 0; i < 6; i++) ctl.update(1 / 60);
for (let i = 0; i < 6; i++) ctl.update(1 / 60);
assert.ok(ctl.intensity[0] >= 0.95, 'on at 0.1 s: ' + ctl.intensity[0]);
modes[0] = 0; ctl.setModes(modes);
for (let i = 0; i < 15; i++) ctl.update(1 / 60);
assert.ok(ctl.intensity[0] <= 0.05, 'off at 0.25 s: ' + ctl.intensity[0]);
{
    const c = createLamps(layout), m = new Uint8Array(lamps.length); m[1] = 2; c.setModes(m);
    let edges = 0, prev = false;
    for (let i = 0; i < 240; i++) { c.update(1 / 60); const lit = c.intensity[1] > 0.5; if (lit && !prev) edges++; prev = lit; }
    assert.ok(edges >= 7 && edges <= 9, 'slow blink rising edges in 4 s: ' + edges);
}

// sweepUp lights lower lamps first; blackout with replace beats a lower-priority sweep; priority decides replace conflicts; max vs replace.
{
    const lo = { x: 0, y: 0.2, u: 0.5, v: 0.2 / 1.07 }, hi = { x: 0, y: 0.9, u: 0.5, v: 0.9 / 1.07 };
    const p = { duration: 1 };
    for (const t of [0.2, 0.25, 0.3]) assert.ok(SHOWS.sweepUp(lo, t, p) > SHOWS.sweepUp(hi, t, p), 'sweep order at ' + t);
    assert.equal(SHOWS.sweepUp(hi, 0.05, p), 0);
    assert.deepEqual(Object.keys(SHOWS).sort(), [...SHOW_NAMES].sort());
    const a = SHOWS.sparkle({ index: 3 }, 0.5, { seed: 9 }), b = SHOWS.sparkle({ index: 3 }, 0.5, { seed: 9 });
    assert.equal(a, b);
}
{
    const c = createLamps(layout), m = new Uint8Array(lamps.length); m[2] = 1; c.setModes(m);
    c.show('strobe', { duration: 2, priority: 1, replace: true });
    c.show('blackout', { duration: 2, priority: 5 });            // higher priority replace: dark
    for (let i = 0; i < 20; i++) c.update(1 / 60);
    assert.ok(c.intensity[2] < 0.05, 'blackout wins: ' + c.intensity[2]);
    assert.deepEqual(c.inspect().shows.sort(), ['blackout', 'strobe']);
    c.stopShows();
    assert.equal(c.inspect().shows.length, 0);
    // Without replace a show only adds to the base (max).
    c.show('blackout', { duration: 2, replace: false });
    for (let i = 0; i < 20; i++) c.update(1 / 60);
    assert.ok(c.intensity[2] > 0.95, 'max keeps base lit: ' + c.intensity[2]);
    // A lower-priority replace is overridden by a higher-priority plain show.
    c.stopShows();
    c.show('blackout', { duration: 2, priority: 1 });
    c.show('breath', { duration: 2, priority: 2, period: 1 });
    for (let i = 0; i < 12; i++) c.update(1 / 60);
    assert.ok(c.intensity[2] > 0.2, 'higher priority show survives lower replace');
}

// GI off <= 0.05 in 0.2 s; flash peak >= 0.9 and <= 0.05 after 0.4 s.
{
    const c = createLamps(layout);
    assert.equal(c.giLevel[0], 1);
    c.gi('all', 'off');
    for (let i = 0; i < 12; i++) c.update(1 / 60);
    assert.ok(c.giLevel[0] <= 0.05 && c.giLevel[1] <= 0.05, 'gi off: ' + c.giLevel[0]);
    c.gi('lower', 'on');
    for (let i = 0; i < 30; i++) c.update(1 / 60);
    assert.ok(c.giLevel[0] > 0.95 && c.giLevel[1] <= 0.05);
    c.flash('fa', 300);
    let peak = 0;
    for (let i = 0; i < 24; i++) { c.update(1 / 60); peak = Math.max(peak, c.flashLevel[0]); }
    assert.ok(peak >= 0.9, 'flash peak ' + peak);
    assert.ok(c.flashLevel[0] <= 0.05 && c.flashLevel[1] === 0, 'flash end ' + c.flashLevel[0]);
}

// rgba: 256 texels, texel 0 black, texel i = colour x intensity; setColor changes it.
{
    const c = createLamps(layout), m = new Uint8Array(lamps.length); m[0] = 1; c.setModes(m);
    for (let i = 0; i < 30; i++) c.update(1 / 60);
    assert.equal(c.rgba.length, 1024);
    assert.deepEqual([...c.rgba.subarray(0, 3)], [0, 0, 0]);
    assert.ok(c.rgba[4] >= 250 && c.rgba[5] > 120 && c.rgba[6] > 40 && c.rgba[5] < 135);
    c.setColor('l0', '#0000ff');
    c.update(1 / 60);
    assert.ok(c.rgba[4] === 0 && c.rgba[6] >= 250);
}

console.log('pinball lights ok');
