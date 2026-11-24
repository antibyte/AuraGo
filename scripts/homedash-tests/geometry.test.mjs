import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs';
const Gm = () => loadEngine(['hd-core.js', 'hd-geometry.js']).E.geometry;
const cubic = (p0, p1, p2, p3, t) => (1 - t) ** 3 * p0 + 3 * (1 - t) ** 2 * t * p1 + 3 * (1 - t) * t * t * p2 + t ** 3 * p3;
export const tests = [
  ['arc, ticks, needle', () => { const g = Gm();
    assert.equal(g.arc(50, 50, 40, 0, 90), 'M50 10A40 40 0 0 1 90 50'); assert.equal(g.arc(50, 50, 40, 225, 270), 'M21.716 78.284A40 40 0 1 1 78.284 78.284');
    assert.equal(g.arc(50, 50, 42, -135, 270), g.arc(50, 50, 42, 225, 270)); assert.equal((g.arc(50, 50, 40, 0, 360).match(/A/g) || []).length, 2);
    assert.equal((g.ticks(50, 50, 40, 46, 225, 270, 2).match(/M/g) || []).length, 3);
    assert.equal(g.needle(50, 50, 44, 12, 3.2, 0.8), 'M49.6 6L50.4 6L51.6 50L51.6 62L48.4 62L48.4 50Z'); }],
  ['angles and needle jitter', () => { const g = Gm();
    assert.equal(g.valueToAngle(50, 0, 100, -135, 270), 0); assert.equal(g.valueToAngle(150, 0, 100, -135, 270), 135); assert.equal(g.valueToAngle(NaN, 0, 100, -135, 270), -135);
    assert.equal(g.needleChanged(50, 50.1, 0, 100), false); assert.equal(g.needleChanged(50, 50.3, 0, 100), true); eq(g.minMaxLast([3, null, 1, 5]), { min: 1, max: 5, last: 5 }); assert.equal(g.minMaxLast([null]), null); }],
  ['sparkline: padding, gaps, flat, empty', () => { const g = Gm(); const s = g.sparkline([1, 2, 3], { w: 100, h: 40 });
    assert.ok(s.line.startsWith('M2 ')); assert.equal((s.line.match(/C/g) || []).length, 2); assert.ok(s.area.endsWith('Z')); assert.equal(s.last.x, 98); assert.ok(Math.abs(s.last.y - 5.484) < 0.01);
    assert.equal((g.sparkline([1, null, 3, 4], { w: 100, h: 40 }).line.match(/M/g) || []).length, 2); assert.equal(g.sparkline([null, null]).line, ''); assert.ok(!g.sparkline([5, 5, 5]).line.includes('NaN')); }],
  ['monotone sparkline never overshoots a segment', () => { const g = Gm(); const { line } = g.sparkline([0, 10, 10, 10, 0, 5, 5, 20, 19, 30], { w: 300, h: 100 });
    let x0, y0; for (const tok of line.match(/[MC][^MC]*/g)) { const n = tok.slice(1).trim().split(/[\s,]+/).map(Number);
      if (tok[0] === 'M') { [x0, y0] = n; continue; } const [, c1, , c2, x3, y3] = n;
      for (let t = 0; t <= 1; t += 0.05) { const y = cubic(y0, c1, c2, y3, t); assert.ok(y >= Math.min(y0, y3) - 1e-6 && y <= Math.max(y0, y3) + 1e-6, `overshoot ${y}`); } x0 = x3; y0 = y3; } }],
  ['seven-segment map and paths', () => { const g = Gm();
    assert.deepEqual('0123456789'.split('').map(c => g.SEGMENTS[c].length), [6, 2, 5, 5, 4, 5, 6, 3, 7, 6]); assert.equal(g.SEGMENTS['-'], 'g');
    eq(g.sevenSeg('21.5'), [{ char: '2', segs: ['a', 'b', 'd', 'e', 'g'], dp: false }, { char: '1', segs: ['b', 'c'], dp: true }, { char: '5', segs: ['a', 'c', 'd', 'f', 'g'], dp: false }]);
    eq(g.sevenSeg('.5')[0], { char: ' ', segs: [], dp: true }); const p = g.segmentPaths(12, 20, 2.4, 0.6);
    for (const k of ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'dp']) assert.ok(p[k].startsWith('M') && p[k].endsWith('Z'), k); }]
];
