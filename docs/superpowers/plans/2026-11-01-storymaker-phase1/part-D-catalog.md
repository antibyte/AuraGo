## Part D: Starter catalog generator and outputs

Inputs: contract sections 3, 3a, 3b, 10 and 12 and the spec sections Catalog and rig, Story document (ID form) and Decisions (catalog style). No other part is needed. Outputs: `ui/img/storymaker/**` (`catalog.json` with version `70d5cf51a9a9`, `rig/human.json`, 23 props, 4 environments, 76 thumbnails), `ui/lang/storymaker_catalog/*.json`, the generator under `assets/storymaker/` and `scripts/build-storymaker-catalog.mjs`. Part D edits no existing file; `package.json` is part C's (it adds `build:storymaker-catalog` and `check:storymaker-catalog`). The generator is plain Node ESM without dependencies; run the node tests with one file per call (`node --test <file>`).

Go test runs use the shared cache settings: `GOCACHE=D:\dev-cache\go-build`, `GOTMPDIR=D:\tmp\go-build`.

Style in one paragraph: flat vector explainer look, no outlines on silhouettes, limited palettes, a single second tone (the outline colour at low opacity) for shading, filled ribbons instead of strokes where round caps matter, rounded forms. Characters are 1000 rig units tall and occupy 55 percent of the stage height at depth 1. The rig, its conventions (which limb is near, negative angles swing towards +x, hands and feet attach at fixed lengths) and the budget (about 2.4 MiB in 105 files, limit 30 MiB and 2000 files) are in the README written in Task D8; the format notes for other parts are in the part notes.

### Task D1: Path reader, shape helpers, proportions and the section 3a renderer

**Files:**
- Create: `assets/storymaker/lib/util.mjs`, `assets/storymaker/lib/pathdata.mjs`, `assets/storymaker/lib/dims.mjs`, `assets/storymaker/lib/svg.mjs`
- Test: `assets/storymaker/test/geometry.test.mjs`

The pure geometry kit every later task builds on. `svg.mjs` implements contract section 3a literally (bone transforms, slot order per view, variant keys, colour tokens, facing left as a mirror) and draws thumbnails and review sheets, so the art is judged under the same rules the player implements. Run all commands from the worktree root.

- [ ] **Step 1: Write the failing test**

`assets/storymaker/test/geometry.test.mjs`:

```js
// Task D1: the path reader, the shape helpers and the section 3a renderer.
// Run: node --test assets/storymaker/test/geometry.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePath } from '../lib/pathdata.mjs';
import { n, poly, smooth, ellipse, roundRect, capsule, limb, ribbon, rng } from '../lib/util.mjs';
import { boneWorlds, evalKeys, shapeSvg, Ctx } from '../lib/svg.mjs';
import { BODIES, BODY_NAMES, hipHeight } from '../lib/dims.mjs';

test('the path reader accepts the commands the generators emit and rejects malformed data', () => {
  for (const d of ['M0 0L1 1Z', 'M1.5-2C1 2 3 4 5 6Q1 2 3 4A5 5 0 1 0 10 10Z', 'M0 0h5v5H0z', 'M0 0 10 10 20 0']) assert.ok(parsePath(d).ok, d);
  for (const d of ['', 'L1 1', 'M0', 'M0 0L1', 'M0 0A5 5 0 2 0 1 1', 'M0 0Zx', 'M0 0 L 1 e']) assert.equal(parsePath(d).ok, false, d);
});

test('shape helpers emit valid path data and round to one decimal', () => {
  assert.equal(n(1.26), 1.3);
  assert.equal(Object.is(n(-0.01), -0), false);
  const shapes = [
    poly([[0, 0], [10, 0], [5, 8]]), smooth([[0, 0], [10, 2], [20, 0], [10, -6]]), ellipse(5, 5, 4, 2), roundRect(0, 0, 40, 20, 6),
    capsule(0, 0, 30, 10, 8), limb(0, 100, 30, 20, 2), ribbon([[0, 0], [10, 4], [20, 0]], 4), ribbon([[0, 0], [30, 0]], (t) => 6 - 4 * t),
  ];
  for (const d of shapes) assert.ok(parsePath(d).ok, d);
  const r = rng(5);
  assert.notEqual(r(), r());
  assert.equal(rng(5)(), rng(5)());
});

test('bone transforms follow section 3a and negative angles swing towards +x', () => {
  const rig = {
    bones: [
      { name: 'root', parent: null, rest: { front: { x: 0, y: 0, angle: 0 } } },
      { name: 'arm', parent: 'root', rest: { front: { x: 10, y: -100, angle: 0 } } },
      { name: 'hand', parent: 'arm', rest: { front: { x: 0, y: 50, angle: 0 } } },
    ],
    bodies: { tall: { bones: { arm: { rest: { front: { y: -120 } } } } } },
  };
  const at = (w, x, y) => [w[0] * x + w[2] * y + w[4], w[1] * x + w[3] * y + w[5]];
  const stand = boneWorlds(rig, 'none', 'front', {}, {});
  assert.deepEqual(at(stand.get('hand'), 0, 0).map(Math.round), [10, -50]);
  const raised = boneWorlds(rig, 'none', 'front', { arm: -90 }, {});
  assert.deepEqual(at(raised.get('hand'), 0, 0).map(Math.round), [60, -100]); // hangs 50 below, -90 swings it to +x
  const gesture = boneWorlds(rig, 'none', 'front', { arm: -90 }, { arm: 90 });
  assert.deepEqual(at(gesture.get('hand'), 0, 0).map(Math.round), [10, -50]); // pose offsets and deltas add up
  assert.deepEqual(at(boneWorlds(rig, 'tall', 'front').get('arm'), 0, 0).map(Math.round), [10, -120]); // body override
});

test('key frames blend linearly, hold the latest hands and return to the first key', () => {
  const keys = [{ t: 0, bones: {} }, { t: 0.5, bones: { a: 10 }, hands: { r: 'point' }, offset_y: 4 }, { t: 1, bones: {} }];
  assert.equal(evalKeys(keys, 0.25).bones.a, 5);
  assert.equal(evalKeys(keys, 0.5).bones.a, 10);
  assert.equal(evalKeys(keys, 0.75).hands.r, 'point');
  assert.equal(evalKeys(keys, 1).bones.a, 0);
  assert.equal(evalKeys(keys, 0.25).offset_y, 2);
});

test('shapes resolve $tokens through the given maps and report unresolved ones', () => {
  const ctx = new Ctx();
  const svg = shapeSvg({ d: 'M0 0L1 1Z', fill: '$skin', opacity: 0.5 }, [{ skin: '#c4875f' }], ctx);
  assert.match(svg, /fill="#c4875f"/);
  assert.match(svg, /opacity="0.5"/);
  shapeSvg({ d: 'M0 0L1 1Z', fill: '$nope' }, [{}], ctx);
  assert.deepEqual([...ctx.unresolved], ['$nope']);
});

test('body proportions share the hand and foot attachment lengths', () => {
  for (const name of BODY_NAMES) {
    const b = BODIES[name];
    assert.equal(b.torsoLen, 255, name);
    assert.ok(hipHeight(b) >= 375 && hipHeight(b) <= 440, name);
  }
});
```


- [ ] **Step 2: Run the test to verify it fails**

Run: `node --test assets/storymaker/test/geometry.test.mjs`
Expected: FAIL (`ERR_MODULE_NOT_FOUND`: cannot find `assets/storymaker/lib/pathdata.mjs`)

- [ ] **Step 3: Implement**

`assets/storymaker/lib/util.mjs`:

```js
// Shared geometry helpers for the Storymaker catalog generators.
// Everything here is pure and deterministic: no Date, no Math.random, no I/O.

/** Round to one decimal, never "-0". Rig units are about 1000 high, so 0.1 is far below a pixel. */
export function n(v) {
  const r = Math.round(v * 10) / 10;
  return Object.is(r, -0) ? 0 : r;
}

/** Round to two decimals (for opacities, ratios). */
export function n2(v) {
  const r = Math.round(v * 100) / 100;
  return Object.is(r, -0) ? 0 : r;
}

const f = (v) => String(n(v));

export const rad = (deg) => (deg * Math.PI) / 180;
export const deg = (r) => (r * 180) / Math.PI;
export const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));
export const lerp = (a, b, t) => a + (b - a) * t;

/** Piecewise-linear interpolation through [[x, y], ...] sorted by x. */
export function interp(table, x) {
  if (x <= table[0][0]) return table[0][1];
  for (let i = 1; i < table.length; i++) {
    if (x <= table[i][0]) {
      const [x0, y0] = table[i - 1];
      const [x1, y1] = table[i];
      return y0 + ((y1 - y0) * (x - x0)) / (x1 - x0);
    }
  }
  return table[table.length - 1][1];
}

/** Smooth monotone-ish interpolation (Catmull-Rom on the y values, uniform in the table index). */
export function interpSmooth(table, x) {
  if (x <= table[0][0]) return table[0][1];
  const last = table.length - 1;
  if (x >= table[last][0]) return table[last][1];
  let i = 1;
  while (x > table[i][0]) i++;
  const [x0, y0] = table[i - 1];
  const [x1, y1] = table[i];
  const ym = i - 2 >= 0 ? table[i - 2][1] : y0;
  const yp = i + 1 <= last ? table[i + 1][1] : y1;
  const t = (x - x0) / (x1 - x0);
  const m0 = (y1 - ym) / 2;
  const m1 = (yp - y0) / 2;
  const t2 = t * t;
  const t3 = t2 * t;
  return (2 * t3 - 3 * t2 + 1) * y0 + (t3 - 2 * t2 + t) * m0 + (-2 * t3 + 3 * t2) * y1 + (t3 - t2) * m1;
}

// ---------------------------------------------------------------- path strings

export const M = (x, y) => `M${f(x)} ${f(y)}`;
export const L = (x, y) => `L${f(x)} ${f(y)}`;
export const C = (x1, y1, x2, y2, x, y) => `C${f(x1)} ${f(y1)} ${f(x2)} ${f(y2)} ${f(x)} ${f(y)}`;
export const Q = (x1, y1, x, y) => `Q${f(x1)} ${f(y1)} ${f(x)} ${f(y)}`;
/** SVG arc; flags are written with spaces so every parser reads them the same way. */
export const A = (rx, ry, rot, large, sweep, x, y) => `A${f(rx)} ${f(ry)} ${f(rot)} ${large ? 1 : 0} ${sweep ? 1 : 0} ${f(x)} ${f(y)}`;
export const Z = 'Z';

export function poly(points, close = true) {
  if (!points.length) return '';
  let d = M(points[0][0], points[0][1]);
  for (let i = 1; i < points.length; i++) d += L(points[i][0], points[i][1]);
  return close ? d + Z : d;
}

/**
 * Smooth curve through points (Catmull-Rom converted to cubic Beziers).
 * A point may carry a third element 'c' to mark a corner (no smoothing at that vertex).
 */
export function smooth(points, { closed = true, tension = 1 } = {}) {
  const pts = points;
  const cnt = pts.length;
  if (cnt < 2) return '';
  const get = (i) => {
    if (closed) return pts[((i % cnt) + cnt) % cnt];
    return pts[Math.min(cnt - 1, Math.max(0, i))];
  };
  let d = M(pts[0][0], pts[0][1]);
  const segs = closed ? cnt : cnt - 1;
  for (let i = 0; i < segs; i++) {
    const p0 = get(i - 1);
    const p1 = get(i);
    const p2 = get(i + 1);
    const p3 = get(i + 2);
    const k = tension / 6;
    const c1 = p1[2] === 'c' ? [p1[0], p1[1]] : [p1[0] + (p2[0] - p0[0]) * k, p1[1] + (p2[1] - p0[1]) * k];
    const c2 = p2[2] === 'c' ? [p2[0], p2[1]] : [p2[0] - (p3[0] - p1[0]) * k, p2[1] - (p3[1] - p1[1]) * k];
    d += C(c1[0], c1[1], c2[0], c2[1], p2[0], p2[1]);
  }
  return closed ? d + Z : d;
}

export function ellipse(cx, cy, rx, ry) {
  return `${M(cx - rx, cy)}${A(rx, ry, 0, 1, 0, cx + rx, cy)}${A(rx, ry, 0, 1, 0, cx - rx, cy)}${Z}`;
}
export const circle = (cx, cy, r) => ellipse(cx, cy, r, r);

export function rect(x, y, w, h) {
  return `${M(x, y)}${L(x + w, y)}${L(x + w, y + h)}${L(x, y + h)}${Z}`;
}

export function roundRect(x, y, w, h, r) {
  const rr = Math.min(r, w / 2, h / 2);
  return (
    M(x + rr, y) + L(x + w - rr, y) + A(rr, rr, 0, 0, 1, x + w, y + rr) + L(x + w, y + h - rr) +
    A(rr, rr, 0, 0, 1, x + w - rr, y + h) + L(x + rr, y + h) + A(rr, rr, 0, 0, 1, x, y + h - rr) +
    L(x, y + rr) + A(rr, rr, 0, 0, 1, x + rr, y) + Z
  );
}

/** Rounded rect with individual corner radii [tl, tr, br, bl]. */
export function roundRect4(x, y, w, h, [tl, tr, br, bl]) {
  const lim = Math.min(w, h) / 2;
  tl = Math.min(tl, lim); tr = Math.min(tr, lim); br = Math.min(br, lim); bl = Math.min(bl, lim);
  const arc = (r, ex, ey) => (r > 0 ? A(r, r, 0, 0, 1, ex, ey) : '');
  return (
    M(x + tl, y) + L(x + w - tr, y) + arc(tr, x + w, y + tr) + L(x + w, y + h - br) + arc(br, x + w - br, y + h) +
    L(x + bl, y + h) + arc(bl, x, y + h - bl) + L(x, y + tl) + arc(tl, x + tl, y) + Z
  );
}

/** Capsule from (x1,y1) to (x2,y2), full width w, round ends. */
export function capsule(x1, y1, x2, y2, w) {
  const dx = x2 - x1;
  const dy = y2 - y1;
  const len = Math.hypot(dx, dy) || 1;
  const nx = (-dy / len) * (w / 2);
  const ny = (dx / len) * (w / 2);
  const r = w / 2;
  return (
    M(x1 + nx, y1 + ny) + L(x2 + nx, y2 + ny) + A(r, r, 0, 0, 0, x2 - nx, y2 - ny) + L(x1 - nx, y1 - ny) +
    A(r, r, 0, 0, 0, x1 + nx, y1 + ny) + Z
  );
}

/**
 * A limb along +y from y0 to y1 with width w0 at the top and w1 at the bottom, round ends.
 * 'bulge' (units) widens the middle slightly for a softer muscle line.
 */
export function limb(y0, y1, w0, w1, bulge = 0, cx = 0) {
  const r0 = w0 / 2;
  const r1 = w1 / 2;
  const ym = (y0 + y1) / 2;
  const wm = (r0 + r1) / 2 + bulge;
  return (
    M(cx - r0, y0) + C(cx - r0, y0 - r0 * 0.56, cx - r0 * 0.56, y0 - r0, cx, y0 - r0) +
    C(cx + r0 * 0.56, y0 - r0, cx + r0, y0 - r0 * 0.56, cx + r0, y0) +
    C(cx + wm, ym - (y1 - y0) * 0.1, cx + wm, ym + (y1 - y0) * 0.1, cx + r1, y1) +
    C(cx + r1, y1 + r1 * 0.56, cx + r1 * 0.56, y1 + r1, cx, y1 + r1) +
    C(cx - r1 * 0.56, y1 + r1, cx - r1, y1 + r1 * 0.56, cx - r1, y1) +
    C(cx - wm, ym + (y1 - y0) * 0.1, cx - wm, ym - (y1 - y0) * 0.1, cx - r0, y0) + Z
  );
}

/** Dense samples of the open or closed Catmull-Rom spline through the points. */
export function sampleSpline(points, per = 6, closed = false) {
  const cnt = points.length;
  if (cnt < 3) return points.slice();
  const get = (i) => (closed ? points[((i % cnt) + cnt) % cnt] : points[Math.min(cnt - 1, Math.max(0, i))]);
  const out = [];
  const segs = closed ? cnt : cnt - 1;
  for (let i = 0; i < segs; i++) {
    const p0 = get(i - 1);
    const p1 = get(i);
    const p2 = get(i + 1);
    const p3 = get(i + 2);
    for (let k = 0; k < per; k++) {
      const t = k / per;
      const t2 = t * t;
      const t3 = t2 * t;
      out.push([
        0.5 * (2 * p1[0] + (-p0[0] + p2[0]) * t + (2 * p0[0] - 5 * p1[0] + 4 * p2[0] - p3[0]) * t2 + (-p0[0] + 3 * p1[0] - 3 * p2[0] + p3[0]) * t3),
        0.5 * (2 * p1[1] + (-p0[1] + p2[1]) * t + (2 * p0[1] - 5 * p1[1] + 4 * p2[1] - p3[1]) * t2 + (-p0[1] + 3 * p1[1] - 3 * p2[1] + p3[1]) * t3),
      ]);
    }
  }
  if (!closed) out.push(points[cnt - 1].slice(0, 2));
  return out;
}

/**
 * A stroke turned into a filled outline (so every renderer draws round caps).
 * points: centre line (smoothed internally); width: number or function (t 0..1) -> width.
 */
export function ribbon(points, width) {
  const wf = typeof width === 'function' ? width : () => width;
  const pts = sampleSpline(points, 4);
  const cnt = pts.length;
  const left = [];
  const right = [];
  const dirs = [];
  for (let i = 0; i < cnt; i++) {
    const p0 = pts[Math.max(0, i - 1)];
    const p1 = pts[Math.min(cnt - 1, i + 1)];
    const dx = p1[0] - p0[0];
    const dy = p1[1] - p0[1];
    const len = Math.hypot(dx, dy) || 1;
    const w = wf(i / (cnt - 1)) / 2;
    dirs.push([dx / len, dy / len]);
    left.push([pts[i][0] + (-dy / len) * w, pts[i][1] + (dx / len) * w]);
    right.push([pts[i][0] - (-dy / len) * w, pts[i][1] - (dx / len) * w]);
  }
  const cap = (p, dir, w, sign) => {
    const nx = -dir[1] * sign;
    const ny = dir[0] * sign;
    const out = [];
    for (let k = 1; k <= 3; k++) {
      const th = (k * Math.PI) / 4;
      out.push([p[0] + w * (Math.cos(th) * nx + Math.sin(th) * dir[0] * sign), p[1] + w * (Math.cos(th) * ny + Math.sin(th) * dir[1] * sign)]);
    }
    return out;
  };
  const seq = [...left, ...cap(pts[cnt - 1], dirs[cnt - 1], wf(1) / 2, 1), ...right.reverse(), ...cap(pts[0], dirs[0], wf(0) / 2, -1)];
  return poly(seq);
}

// ---------------------------------------------------------------- shapes

/** A flat-filled shape. */
export function fill(d, color, opacity) {
  const s = { d, fill: color };
  if (opacity !== undefined && opacity !== 1) s.opacity = n2(opacity);
  return s;
}
/** Shade overlay: the character/palette outline colour at low opacity (the "second tone"). */
export const shade = (d, a = 0.14) => fill(d, '$outline', a);
/** Light overlay. */
export const light = (d, a = 0.2) => fill(d, '#ffffff', a);
/** A stroked path (use sparingly; strokes assume round caps and joins). */
export function line(d, color, width, opacity) {
  const s = { d, stroke: color, stroke_width: n(width) };
  if (opacity !== undefined && opacity !== 1) s.opacity = n2(opacity);
  return s;
}
export function gradient(d, type, stops, geom, opacity) {
  const s = { d, gradient: { type, stops, ...geom } };
  if (opacity !== undefined && opacity !== 1) s.opacity = n2(opacity);
  return s;
}
export const linear = (d, stops, x1, y1, x2, y2, opacity) => gradient(d, 'linear', stops, { x1: n(x1), y1: n(y1), x2: n(x2), y2: n(y2) }, opacity);
export const radial = (d, stops, cx, cy, r, opacity) => gradient(d, 'radial', stops, { cx: n(cx), cy: n(cy), r: n(r) }, opacity);

/** Deterministic pseudo random numbers (mulberry32) for decorative variation. */
export function rng(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** Stable JSON: keys keep their insertion order, which the generators control. */
export const compact = (obj) => JSON.stringify(obj) + '\n';
```

`assets/storymaker/lib/pathdata.mjs`:

```js
// A strict SVG path data reader used to check every generated shape (part C draws them with Path2D, which
// silently ignores malformed data, so the generator must never emit any).

const ARGS = { M: 2, L: 2, H: 1, V: 1, C: 6, S: 4, Q: 4, T: 2, A: 7, Z: 0 };
const TOKEN = /\s*,?\s*(?:([MmLlHhVvCcSsQqTtAaZz])|(-?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?))/y;

/** Returns {ok, error?, commands: [{cmd, args}]}. */
export function parsePath(d) {
  const commands = [];
  if (typeof d !== 'string' || !d.trim()) return { ok: false, error: 'empty path', commands };
  TOKEN.lastIndex = 0;
  let cmd = null;
  let args = [];
  let pos = 0;
  const flush = () => {
    if (cmd === null) return null;
    const need = ARGS[cmd.toUpperCase()];
    if (need === 0) {
      if (args.length) return `Z takes no arguments`;
      commands.push({ cmd, args });
      return null;
    }
    if (args.length === 0 || args.length % need !== 0) return `${cmd} has ${args.length} numbers, needs a multiple of ${need}`;
    for (let i = 0; i < args.length; i += need) commands.push({ cmd: i === 0 ? cmd : cmd === 'M' ? 'L' : cmd === 'm' ? 'l' : cmd, args: args.slice(i, i + need) });
    return null;
  };
  while (pos < d.length) {
    TOKEN.lastIndex = pos;
    const m = TOKEN.exec(d);
    if (!m) {
      if (d.slice(pos).trim() === '') break;
      return { ok: false, error: `unexpected text at ${pos}: ${d.slice(pos, pos + 12)}`, commands };
    }
    pos = TOKEN.lastIndex;
    if (m[1]) {
      const err = flush();
      if (err) return { ok: false, error: err, commands };
      if (cmd === null && m[1].toUpperCase() !== 'M') return { ok: false, error: 'path must start with M', commands };
      cmd = m[1];
      args = [];
    } else {
      if (cmd === null) return { ok: false, error: 'number before a command', commands };
      args.push(Number(m[2]));
    }
  }
  const err = flush();
  if (err) return { ok: false, error: err, commands };
  for (const c of commands) {
    if (c.args.some((v) => !Number.isFinite(v))) return { ok: false, error: 'non-finite number', commands };
    if (c.cmd.toUpperCase() === 'A' && ![0, 1].includes(c.args[3])) return { ok: false, error: 'arc large-arc flag must be 0 or 1', commands };
    if (c.cmd.toUpperCase() === 'A' && ![0, 1].includes(c.args[4])) return { ok: false, error: 'arc sweep flag must be 0 or 1', commands };
  }
  if (!commands.length) return { ok: false, error: 'no commands', commands };
  return { ok: true, commands };
}
```

`assets/storymaker/lib/dims.mjs`:

```js
// Proportions of the shared human rig, in rig units (an adult is 1000 units from the feet line to the top of the head).
// Rig space: y points down, the origin is the root = the middle of the feet line.
//
// Bone frames: every bone's local axes are parallel to rig space at angle 0, and its origin is its joint.
// Limbs extend along local +y, the torso/neck/head extend along local -y. Rest angles are 0 in every view except
// for small natural offsets (arms hang a little away from the body, the elder stoops).
//
// Views are drawn facing right (+x). "Right" limbs (_r) are the character's right side: in the front view they are
// on the screen left (x < 0); in three_quarter and profile (facing right) they are the near limbs, drawn in front of
// the torso. Negative angles swing a limb towards +x (forward when facing right).

export const UNITS_HEIGHT = 1000;
export const RIG_HEIGHT = 0.55; // fraction of the stage height an adult occupies at depth scale 1
export const YAW = { front: 0, three_quarter: 38, profile: 90 };
export const VIEWS = ['front', 'three_quarter', 'profile'];

export const cosYaw = (view) => Math.cos((YAW[view] * Math.PI) / 180);
export const sinYaw = (view) => Math.sin((YAW[view] * Math.PI) / 180);
/** Screen x of a body-frame point (X lateral: +X is the character's left, Z forward). */
export const projX = (view, X, Z = 0) => X * cosYaw(view) + Z * sinYaw(view);

// Fixed lengths shared by every body type (the end parts hang on these two).
export const LOWER_ARM = 135;
export const LOWER_LEG = 205;
export const ANKLE = 20; // ankle joint height above the feet line

/**
 * Body types. Widths are half widths of the silhouette in the front view.
 * stoop: forward offset of the head/shoulders in the angled views; belly: forward bulge of the waist.
 */
export const BODIES = {
  average: {
    scale: 1, shoulderX: 80, hipX: 40, torsoLen: 255, upperArm: 150, upperLeg: 215,
    shoulderHW: 100, waistHW: 80, hipHW: 90, chestD: 62, waistD: 56, hipD: 58, belly: 0, stoop: 0,
    arm: 52, forearm: 44, thigh: 76, shin: 56, neckHW: 30,
  },
  slim: {
    scale: 1, shoulderX: 72, hipX: 36, torsoLen: 255, upperArm: 150, upperLeg: 215,
    shoulderHW: 88, waistHW: 62, hipHW: 80, chestD: 52, waistD: 46, hipD: 50, belly: 0, stoop: 0,
    arm: 44, forearm: 38, thigh: 64, shin: 48, neckHW: 26,
  },
  broad: {
    scale: 1, shoulderX: 96, hipX: 44, torsoLen: 255, upperArm: 150, upperLeg: 215,
    shoulderHW: 120, waistHW: 100, hipHW: 104, chestD: 74, waistD: 70, hipD: 66, belly: 8, stoop: 0,
    arm: 62, forearm: 52, thigh: 86, shin: 62, neckHW: 36,
  },
  elder: {
    scale: 0.97, shoulderX: 78, hipX: 40, torsoLen: 255, upperArm: 148, upperLeg: 210,
    shoulderHW: 96, waistHW: 86, hipHW: 90, chestD: 60, waistD: 64, hipD: 58, belly: 10, stoop: 14,
    arm: 46, forearm: 40, thigh: 70, shin: 50, neckHW: 28,
  },
  child: {
    scale: 0.82, shoulderX: 54, hipX: 30, torsoLen: 255, upperArm: 120, upperLeg: 150,
    shoulderHW: 70, waistHW: 62, hipHW: 68, chestD: 48, waistD: 46, hipD: 46, belly: 6, stoop: 0,
    arm: 38, forearm: 34, thigh: 54, shin: 44, neckHW: 22,
  },
};

export const BODY_NAMES = ['slim', 'average', 'broad', 'child', 'elder'];

/** Hip joint height above the feet line for a body type. */
export const hipHeight = (b) => LOWER_LEG + ANKLE + b.upperLeg;
export const NECK_LEN = 35;
export const HEAD_PIVOT = 44; // head joint above the neck joint
export const WAIST = 25; // torso joint above the hip joint
```

`assets/storymaker/lib/svg.mjs`:

```js
// A small renderer that follows contract section 3a literally, producing SVG.
// It is used for the committed thumbnails and for the review sheets, so the art is judged under the same
// transform and variant rules the player (part C) implements for Canvas2D.
//
//   world(b) = world(parent) . T(rest[view].x, rest[view].y) . R(rest[view].angle + pose.bones[b] + sum deltas[b])
//
// Angles are degrees, clockwise positive (y points down). Slot shapes are in the bone's local coordinates.
// Facing left is scale(-1, 1) about the root x after the view is chosen. Strokes are drawn with round caps.

import { n, n2 } from './util.mjs';

export const VIEWS = ['front', 'three_quarter', 'profile'];

// ---------------------------------------------------------------- matrices (SVG order: [a b c d e f])
const IDENT = [1, 0, 0, 1, 0, 0];
const mul = (m, k) => [
  m[0] * k[0] + m[2] * k[1], m[1] * k[0] + m[3] * k[1],
  m[0] * k[2] + m[2] * k[3], m[1] * k[2] + m[3] * k[3],
  m[0] * k[4] + m[2] * k[5] + m[4], m[1] * k[4] + m[3] * k[5] + m[5],
];
const T = (x, y) => [1, 0, 0, 1, x, y];
const R = (degrees) => {
  const a = (degrees * Math.PI) / 180;
  return [Math.cos(a), Math.sin(a), -Math.sin(a), Math.cos(a), 0, 0];
};
const matStr = (m) => `matrix(${m.map((v) => Math.round(v * 10000) / 10000).join(' ')})`;

/** World matrices of all bones for a view, body type, pose offsets and extra deltas (gesture/cycle). */
export function boneWorlds(rig, bodyName, view, poseBones = {}, deltas = {}) {
  const body = rig.bodies?.[bodyName] || {};
  const worlds = new Map();
  for (const bone of rig.bones) {
    const ov = body.bones?.[bone.name]?.rest?.[view] || {};
    const rest = { ...bone.rest[view], ...ov };
    const angle = (rest.angle || 0) + (poseBones[bone.name] || 0) + (deltas[bone.name] || 0);
    const local = mul(T(rest.x || 0, rest.y || 0), R(angle));
    worlds.set(bone.name, bone.parent ? mul(worlds.get(bone.parent), local) : local);
  }
  return worlds;
}

// ---------------------------------------------------------------- keys (gestures and cycles)
/** Linear blend of key frames {t, bones, hands?, offset_y?} at time t (0..1). Missing bones count as 0. */
export function evalKeys(keys, t) {
  let i = 0;
  while (i < keys.length - 2 && t > keys[i + 1].t) i++;
  const k0 = keys[i];
  const k1 = keys[Math.min(i + 1, keys.length - 1)];
  const span = k1.t - k0.t;
  const u = span > 0 ? Math.min(1, Math.max(0, (t - k0.t) / span)) : 0;
  const bones = {};
  for (const name of new Set([...Object.keys(k0.bones || {}), ...Object.keys(k1.bones || {})])) {
    const a = k0.bones?.[name] || 0;
    const b = k1.bones?.[name] || 0;
    bones[name] = a + (b - a) * u;
  }
  let hands = {};
  for (const k of keys) if (k.t <= t && k.hands) hands = { ...hands, ...k.hands };
  const oy = (k0.offset_y || 0) + ((k1.offset_y || 0) - (k0.offset_y || 0)) * u;
  return { bones, hands, offset_y: oy };
}

// ---------------------------------------------------------------- colours and shapes
export class Ctx {
  constructor() {
    this.gradients = [];
    this.gid = 0;
    this.unresolved = new Set();
    this.missing = new Set();
  }
}

function colourOf(c, maps, ctx) {
  if (typeof c !== 'string') return c;
  if (c[0] !== '$') return c;
  const key = c.slice(1);
  for (const m of maps) if (m && Object.prototype.hasOwnProperty.call(m, key)) return m[key];
  ctx.unresolved.add(c);
  return '#ff00ff';
}

export function shapeSvg(shape, maps, ctx) {
  let fillAttr = 'none';
  if (shape.gradient) {
    const g = shape.gradient;
    const id = `g${ctx.gid++}`;
    const stops = g.stops.map(([o, c]) => `<stop offset="${o}" stop-color="${colourOf(c, maps, ctx)}"/>`).join('');
    ctx.gradients.push(
      g.type === 'radial'
        ? `<radialGradient id="${id}" gradientUnits="userSpaceOnUse" cx="${g.cx}" cy="${g.cy}" r="${g.r}">${stops}</radialGradient>`
        : `<linearGradient id="${id}" gradientUnits="userSpaceOnUse" x1="${g.x1}" y1="${g.y1}" x2="${g.x2}" y2="${g.y2}">${stops}</linearGradient>`,
    );
    fillAttr = `url(#${id})`;
  } else if (shape.fill) {
    fillAttr = colourOf(shape.fill, maps, ctx);
  }
  let attrs = `d="${shape.d}" fill="${fillAttr}"`;
  if (shape.stroke) attrs += ` stroke="${colourOf(shape.stroke, maps, ctx)}" stroke-width="${shape.stroke_width ?? 1}" stroke-linecap="round" stroke-linejoin="round"`;
  if (shape.opacity !== undefined && shape.opacity !== 1) attrs += ` opacity="${n2(shape.opacity)}"`;
  return `<path ${attrs}/>`;
}

export function shapesSvg(shapes, maps, ctx) {
  return shapes.map((s) => shapeSvg(s, maps, ctx)).join('');
}

// ---------------------------------------------------------------- characters
/** Colour maps for a character (the token rules of section 3a). */
export function characterMaps(rig, params, palette) {
  const outfit = rig.params.outfit[params.outfit];
  return [
    {
      skin: rig.params.skin[params.skin],
      hair: rig.params.hair_color[params.hair_color],
      top: outfit.colors.top,
      bottom: outfit.colors.bottom,
      shoes: outfit.colors.shoes,
    },
    rig.params.fixed,
    palette,
  ];
}

function slotKey(rig, slot, params, pose, expression, mouth) {
  const outfit = rig.params.outfit[params.outfit];
  switch (slot.param) {
    case 'body': return params.body;
    case 'skin': return params.skin;
    case 'hair': case 'hair_back': return params.hair;
    case 'eyes': return `${params.eyes}/${expression.eyes}`;
    case 'brows': return `${params.brows}/${expression.brows}`;
    case 'mouth': return mouth || expression.mouth;
    case 'hand': return slot.name === 'hand_l' ? (pose.hands?.l || 'open') : (pose.hands?.r || 'open');
    case 'top': case 'bottom': case 'shoes': return outfit[slot.param];
    case 'glasses': case 'facial_hair': case 'accessory': return params[slot.param];
    default: return undefined;
  }
}

/**
 * Draw a character in rig space (root at the origin). Returns the SVG fragment; the caller wraps it in the
 * placement transform. pose = {bones, hands}; deltas = extra bone deltas (gesture/cycle); expression = {eyes, brows, mouth}.
 */
export function characterSvg(rig, params, palette, ctx, opts = {}) {
  const view = opts.view || 'three_quarter';
  const pose = opts.pose || { bones: {} };
  const deltas = opts.deltas || {};
  const expression = opts.expression || { eyes: 'neutral', brows: 'neutral', mouth: 'rest' };
  const hands = { ...(pose.hands || {}), ...(opts.hands || {}) };
  const worlds = boneWorlds(rig, params.body, view, pose.bones || {}, deltas);
  const maps = characterMaps(rig, params, palette);
  const order = rig.slots.map((s, i) => ({ s, i })).sort((a, b) => a.s.z[view] - b.s.z[view] || a.i - b.i);
  let out = '';
  for (const { s: slot } of order) {
    const key = slotKey(rig, slot, params, { ...pose, hands }, expression, opts.mouth);
    if (key === undefined || key === 'none') continue;
    let variant = rig.variants[slot.name]?.[key];
    if (!variant && slot.param === 'eyes') variant = rig.variants[slot.name]?.[`${params.eyes}/neutral`];
    if (!variant && slot.param === 'brows') variant = rig.variants[slot.name]?.[`${params.brows}/neutral`];
    if (!variant) { ctx.missing.add(`${slot.name}:${key}`); continue; }
    const shapes = variant[view];
    if (!shapes || !shapes.length) continue;
    out += `<g transform="${matStr(worlds.get(slot.bone))}">${shapesSvg(shapes, maps, ctx)}</g>`;
  }
  return out;
}

/** A complete character placed at (x, y) (feet) with screen scale s; facing 'left' mirrors about the root. */
export function placedCharacter(rig, params, palette, ctx, opts = {}) {
  const body = rig.bodies?.[params.body] || { scale: 1 };
  const s = (opts.scale ?? 1) * (body.scale ?? 1);
  const oy = (opts.offset_y || 0) + (opts.pose?.offset_y || 0);
  const sx = opts.facing === 'left' ? -s : s;
  const inner = characterSvg(rig, params, palette, ctx, opts);
  return `<g transform="translate(${n(opts.x || 0)} ${n(opts.y || 0)}) scale(${n2(sx)} ${n2(s)}) translate(0 ${n(oy)})">${inner}</g>`;
}

// ---------------------------------------------------------------- documents
export function svgDoc(viewBox, inner, ctx, { width, height, background } = {}) {
  const [x, y, w, h] = viewBox;
  const defs = ctx && ctx.gradients.length ? `<defs>${ctx.gradients.join('')}</defs>` : '';
  const bg = background ? `<rect x="${x}" y="${y}" width="${w}" height="${h}" fill="${background}"/>` : '';
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${x} ${y} ${w} ${h}" width="${width ?? w}" height="${height ?? h}">${defs}${bg}${inner}</svg>\n`;
}

/** Props resolve their colours from palette tokens only. */
export function propSvg(prop, palette, ctx) {
  return shapesSvg(prop.shapes, [palette], ctx);
}

/** Environments resolve from env.tokens[variant], then palette tokens. */
export function environmentSvg(env, variant, palette, ctx) {
  const maps = [env.tokens[variant], palette];
  return env.layers.map((layer) => `<g data-layer="${layer.name}">${shapesSvg(layer.shapes, maps, ctx)}</g>`).join('');
}
```


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node --test assets/storymaker/test/geometry.test.mjs`
Expected: PASS (6 tests)

- [ ] **Step 5: Commit**

```bash
git add assets/storymaker/lib/util.mjs assets/storymaker/lib/pathdata.mjs assets/storymaker/lib/dims.mjs assets/storymaker/lib/svg.mjs assets/storymaker/test/geometry.test.mjs
git commit -m "feat(storymaker): catalog geometry kit and section 3a svg renderer" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```


### Task D2: Head surface model, face features, hair and head extras

**Files:**
- Create: `assets/storymaker/lib/headgeo.mjs`, `assets/storymaker/lib/face.mjs`, `assets/storymaker/lib/hair.mjs`, `assets/storymaker/lib/extras.mjs`
- Test: `assets/storymaker/test/parts.test.mjs`

The head is a stack of elliptical sections; a view with yaw 0, 38 or 90 degrees maps surface points to the screen, so skull, ears, eyes, brows, mouths, hair (11 styles), glasses, facial hair and accessories stay consistent across front, three-quarter and profile. Eyes and brows are keyed `<style>/<state>`, mouths by shape.

- [ ] **Step 1: Write the failing test**

`assets/storymaker/test/parts.test.mjs`:

```js
// Task D2: head surface model, face features, hair and head extras.
// Run: node --test assets/storymaker/test/parts.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePath } from '../lib/pathdata.mjs';
import { VIEWS } from '../lib/dims.mjs';
import { edges, surf, wrap, facing, HEAD_TOP } from '../lib/headgeo.mjs';
import { headShapes, eyeShapes, browShapes, mouthShapes, EYE_STYLES, EYE_STATES, BROW_STYLES, BROW_STATES, MOUTH_NAMES } from '../lib/face.mjs';
import { HAIR_STYLES, hairShapes } from '../lib/hair.mjs';
import { GLASSES, FACIAL_HAIR, ACCESSORIES, glassesShapes, facialHairShapes, accessoryShapes } from '../lib/extras.mjs';

const TOKENS = new Set(['skin', 'hair', 'outline', 'eye_white', 'iris', 'lip', 'accent', 'accent2']);

function check(where, shapes) {
  assert.ok(Array.isArray(shapes), where);
  for (const [i, s] of shapes.entries()) {
    const p = parsePath(s.d);
    assert.ok(p.ok, `${where}[${i}]: ${p.error} (${s.d.slice(0, 60)})`);
    assert.ok(s.fill || s.stroke, `${where}[${i}] draws nothing`);
    for (const c of [s.fill, s.stroke]) if (typeof c === 'string' && c[0] === '$') assert.ok(TOKENS.has(c.slice(1)), `${where}: ${c}`);
  }
}

test('the head surface keeps the three views consistent', () => {
  // the silhouette is symmetric in the front view and moves towards +x when the head turns right
  const [l0, r0] = edges('front', -120);
  assert.ok(Math.abs(l0 + r0) < 1e-6);
  const [l1, r1] = edges('profile', -120);
  assert.ok(r1 > r0 * 0.8 && l1 > -110 && l1 < -90);
  assert.equal(surf('front', 0, -120)[0], 0);
  assert.ok(surf('profile', 0, -120)[0] > 80, 'the face centre sits at the front edge in profile');
  // the far eye is hidden in profile, both eyes are visible in the front view
  assert.ok(facing('front', -41, -112) && facing('front', 41, -112));
  assert.ok(facing('profile', -41, -112) && !facing('profile', 41, -112));
  assert.ok(wrap('three_quarter', 41, -112)[0] > wrap('three_quarter', -41, -112)[0]);
  assert.ok(HEAD_TOP < -200);
});

test('every face, hair and extra variant is valid path data in every view', () => {
  for (const v of VIEWS) {
    for (const body of ['average', 'elder']) check(`head ${body} ${v}`, headShapes(body, v));
    for (const style of Object.keys(EYE_STYLES)) for (const st of EYE_STATES) check(`eyes ${style}/${st} ${v}`, eyeShapes(style, st, v));
    for (const style of Object.keys(BROW_STYLES)) for (const st of BROW_STATES) check(`brows ${style}/${st} ${v}`, browShapes(style, st, v));
    for (const m of MOUTH_NAMES) check(`mouth ${m} ${v}`, mouthShapes(m, v));
    for (const h of HAIR_STYLES) {
      const { back, front } = hairShapes(h, v);
      check(`hair back ${h} ${v}`, back);
      check(`hair front ${h} ${v}`, front);
      if (h !== 'bald') assert.ok(front.length > 0, `${h} ${v} draws hair`);
    }
    for (const g of GLASSES.filter((x) => x !== 'none')) check(`glasses ${g} ${v}`, glassesShapes(g, v));
    for (const f of FACIAL_HAIR.filter((x) => x !== 'none')) check(`facial hair ${f} ${v}`, facialHairShapes(f, v));
    for (const a of ACCESSORIES.filter((x) => x !== 'none')) check(`accessory ${a} ${v}`, accessoryShapes(a, v));
  }
});

test('the profile shows one eye and one brow, the other views two', () => {
  const count = (shapes) => shapes.length;
  assert.ok(count(eyeShapes('round', 'neutral', 'front')) > count(eyeShapes('round', 'neutral', 'profile')));
  assert.equal(browShapes('soft', 'neutral', 'front').length, 2);
  assert.equal(browShapes('soft', 'neutral', 'profile').length, 1);
});
```


- [ ] **Step 2: Run the test to verify it fails**

Run: `node --test assets/storymaker/test/parts.test.mjs`
Expected: FAIL (`ERR_MODULE_NOT_FOUND`: cannot find `assets/storymaker/lib/headgeo.mjs`)

- [ ] **Step 3: Implement**

`assets/storymaker/lib/headgeo.mjs`:

```js
// Head geometry shared by the skull, the face features, the hair and the accessories.
//
// Head space: origin at the head joint (the top of the neck), y up is negative, x is the character's left (screen
// right in the front view), z points forward. The skull is a stack of elliptical cross sections:
// half width hw(y), front depth cf(y), back depth cb(y). A view with yaw phi maps a surface point (X, y, Z) to the
// screen as x = X cos(phi) + Z sin(phi); its depth towards the viewer is Z cos(phi) - X sin(phi).
// Hair, ears and face features are all expressed on this surface, so the three views stay consistent.

import { interpSmooth, smooth, poly } from './util.mjs';
import { YAW } from './dims.mjs';

export const HEAD_TOP = -223;
export const HEAD_CHIN = -6;

const HW = [[-223, 0], [-218, 44], [-205, 72], [-180, 91], [-150, 97], [-115, 95], [-80, 86], [-50, 70], [-26, 48], [-12, 26], [-6, 0]];
const CF = [[-223, 0], [-218, 42], [-205, 70], [-180, 88], [-150, 92], [-115, 90], [-80, 84], [-50, 74], [-26, 58], [-12, 34], [-6, 0]];
const CB = [[-223, 0], [-212, 54], [-195, 84], [-170, 100], [-140, 104], [-100, 98], [-60, 86], [-30, 60], [-14, 30], [-6, 0]];

export const hw = (y) => interpSmooth(HW, y);
export const cf = (y) => interpSmooth(CF, y);
export const cb = (y) => interpSmooth(CB, y);

const yawRad = (view) => (YAW[view] * Math.PI) / 180;

/** Left and right extreme x of the skull at height y (grown outwards by 'grow' units for hair). */
export function edges(view, y, grow = 0) {
  const ph = yawRad(view);
  const h = hw(y) + (hw(y) > 0 ? grow : 0);
  const f = cf(y) + (cf(y) > 0 ? grow : 0);
  const b = cb(y) + (cb(y) > 0 ? grow : 0);
  return [-Math.hypot(h * Math.cos(ph), b * Math.sin(ph)), Math.hypot(h * Math.cos(ph), f * Math.sin(ph))];
}

/** Row heights from the top of the skull to the chin, denser at both ends. */
export function rowYs(count = 26, top = HEAD_TOP, chin = HEAD_CHIN) {
  const out = [];
  for (let i = 0; i <= count; i++) {
    const t = (1 - Math.cos((Math.PI * i) / count)) / 2;
    out.push(top + (chin - top) * t);
  }
  return out;
}

/** Closed outline of the skull for a view as a list of points. */
export function outlinePoints(view, grow = 0, ys = rowYs()) {
  const right = [];
  const left = [];
  for (const y of ys) {
    const [xl, xr] = edges(view, y, grow);
    right.push([xr, y]);
    left.push([xl, y]);
  }
  // top point and chin point appear once
  const pts = [...right.slice(0, -1), [0, ys[ys.length - 1]], ...left.slice(1, -1).reverse()];
  return pts;
}

export const silhouetteD = (view, grow = 0) => smooth(outlinePoints(view, grow), { tension: 1 });

/** Surface point at azimuth lam (degrees, 0 = front, + towards the character's left) and height y. */
export function surf(view, lamDeg, y, grow = 0) {
  const ph = yawRad(view);
  const lam = (lamDeg * Math.PI) / 180;
  const front = Math.cos(lam) >= 0;
  const h = hw(y) + grow;
  const c = (front ? cf(y) : cb(y)) + grow;
  const X = h * Math.sin(lam);
  const Z = c * Math.cos(lam);
  return [X * Math.cos(ph) + Z * Math.sin(ph), y, Z * Math.cos(ph) - X * Math.sin(ph)];
}

/** Face plane point (X, y) wrapped on the front surface. Returns [x, y, depth]. */
export function wrap(view, X, y, lift = 0) {
  const ph = yawRad(view);
  const h = hw(y);
  const k = h > 1 ? Math.min(1, Math.abs(X) / h) : 1;
  const Z = (cf(y) + lift) * Math.sqrt(Math.max(0, 1 - k * k));
  return [X * Math.cos(ph) + Z * Math.sin(ph), y, Z * Math.cos(ph) - X * Math.sin(ph)];
}

/** True when the face point faces the viewer well enough to be drawn. */
export const facing = (view, X, y, min = 6) => wrap(view, X, y)[2] > min;

/** Map a list of face-plane points. */
export const wrapAll = (view, pts, lift = 0) => pts.map(([X, y]) => wrap(view, X, y, lift).slice(0, 2));

/** Sample an ellipse in the face plane. */
export function faceEllipse(cx, cy, rx, ry, count = 20, rot = 0) {
  const pts = [];
  const co = Math.cos(rot);
  const si = Math.sin(rot);
  for (let i = 0; i < count; i++) {
    const t = (i / count) * Math.PI * 2;
    const u = rx * Math.cos(t);
    const v = ry * Math.sin(t);
    pts.push([cx + u * co - v * si, cy + u * si + v * co]);
  }
  return pts;
}

export const wrappedEllipseD = (view, cx, cy, rx, ry, count = 20, rot = 0, lift = 0) =>
  smooth(wrapAll(view, faceEllipse(cx, cy, rx, ry, count, rot), lift));
export const wrappedPolyD = (view, pts, lift = 0) => poly(wrapAll(view, pts, lift));
export const wrappedSmoothD = (view, pts, closed = true, lift = 0) => smooth(wrapAll(view, pts, lift), { closed });

// Feature anchor points (head space).
export const EYE_Y = -112;
export const EYE_X = 41;
export const BROW_Y = -150;
export const NOSE_Y = -84;
export const MOUTH_Y = -46;
```

`assets/storymaker/lib/face.mjs`:

```js
// Skull, ears, nose, eyes, brows and mouths for the human rig (all three views, built from the head surface model).

import { fill, shade, poly, smooth, ribbon, ellipse, rad } from './util.mjs';
import { YAW } from './dims.mjs';
import {
  edges, rowYs, silhouetteD, surf, wrap, facing, wrapAll, faceEllipse,
  wrappedEllipseD, EYE_X, EYE_Y, BROW_Y, MOUTH_Y, hw,
} from './headgeo.mjs';

const sinPhi = (view) => Math.sin(rad(YAW[view]));
const cosPhi = (view) => Math.cos(rad(YAW[view]));

// ---------------------------------------------------------------- skull
function sideShade(view) {
  // a crescent of the second tone along the right edge (light comes from the upper left)
  const ys = rowYs(18, -200, -24);
  const outer = [];
  const inner = [];
  ys.forEach((y, i) => {
    const [, xr] = edges(view, y);
    const t = i / (ys.length - 1);
    const w = (view === 'profile' ? 9 : 17) * Math.sin(Math.PI * Math.min(1, Math.max(0, t))) ** 0.8 + 1.5;
    outer.push([xr, y]);
    inner.push([xr - w, y]);
  });
  return shade(smooth([...outer, ...inner.reverse()], { tension: 1 }), 0.12);
}

function ear(view, side) {
  // returns an ear shape in the head slot, or null when it is hidden
  const y = -98;
  if (view === 'front') {
    const x = side * (hw(y) + 4);
    return [fill(ellipse(x, y, 13, 22), '$skin'), shade(ellipse(x + side * 1.5, y, 6.5, 13), 0.2)];
  }
  if (side > 0) return null; // the far ear is hidden by the face
  const [x] = surf(view, -92, y);
  const w = 11 * (0.35 + 0.65 * sinPhi(view));
  return [fill(ellipse(x, y, w, 21), '$skin'), shade(ellipse(x - 1, y, w * 0.5, 12), 0.2)];
}

function nose(view) {
  const tip = wrap(view, 0, -79, 13);
  const c = wrap(view, 0, -72);
  if (view === 'front') {
    return [{ d: ribbon(wrapAll(view, [[-7, -75], [-3, -72], [0, -71.5], [3, -72], [7, -75]]), 3.2), fill: '$outline', opacity: 0.2 }];
  }
  if (view === 'three_quarter') {
    const top = wrap(view, 0, -103);
    return [
      { d: ribbon([[top[0] + 1, -101], [tip[0] - 2, tip[1] - 7], [tip[0] - 3, tip[1]]], 3.4), fill: '$outline', opacity: 0.2 },
      { d: ribbon([[tip[0] - 11, -73], [tip[0] - 4, -69.5], [tip[0] + 3, -72]], 4), fill: '$outline', opacity: 0.3 },
    ];
  }
  const top = wrap(view, 0, -100);
  const shape = smooth([[top[0] - 2, -102], [top[0] + 3, -95], [tip[0] - 1, tip[1] - 7], [tip[0] + 2, tip[1] - 1], [tip[0] - 1, tip[1] + 5], [c[0] - 3, -69], [c[0] - 12, -75]], { tension: 1 });
  return [fill(shape, '$skin'), { d: ribbon([[tip[0] - 9, tip[1] + 4], [tip[0] - 4, tip[1] + 6.5]], 3.6), fill: '$outline', opacity: 0.3 }];
}

function blush(view) {
  const out = [];
  for (const side of [-1, 1]) {
    if (!facing(view, side * 58, -70, 14)) continue;
    out.push(fill(wrappedEllipseD(view, side * 58, -70, 15, 8.5, 16), '#ff6b6b', 0.2));
  }
  return out;
}

function elderLines(view) {
  const out = [];
  for (const side of [-1, 1]) {
    if (!facing(view, side * 40, -90, 14)) continue;
    // laugh lines either side of the mouth and crow's feet
    out.push({ d: ribbon(wrapAll(view, [[side * 24, -78], [side * 30, -62], [side * 28, -48]]), 3), fill: '$outline', opacity: 0.22 });
    out.push({ d: ribbon(wrapAll(view, [[side * 62, -118], [side * 69, -113]]), 2.6), fill: '$outline', opacity: 0.2 });
    out.push({ d: ribbon(wrapAll(view, [[side * 62, -110], [side * 70, -108]]), 2.6), fill: '$outline', opacity: 0.2 });
  }
  return out;
}

/** Shapes of the head slot for a body type (all bodies share the skull; the elder adds age lines). */
export function headShapes(body, view) {
  const out = [];
  const eFar = ear(view, 1);
  const eNear = ear(view, -1);
  if (view === 'front') { out.push(...eFar, ...eNear); }
  out.push(fill(silhouetteD(view), '$skin'));
  out.push(sideShade(view));
  if (view !== 'front' && eNear) out.push(...eNear);
  out.push(...nose(view));
  out.push(...blush(view));
  if (body === 'elder') out.push(...elderLines(view));
  return out;
}

// ---------------------------------------------------------------- eyes
export const EYE_STYLES = {
  round: { rx: 16, ry: 19, ir: 11.5, hi: 3.6 },
  almond: { rx: 18.5, ry: 13.5, ir: 9.6, hi: 3.2, lens: true },
  big: { rx: 19, ry: 22, ir: 14, hi: 4.4 },
};
export const EYE_STATES = ['neutral', 'wide', 'smile', 'laugh', 'sad', 'angry', 'lookup', 'narrow', 'worried'];

// lid depth (fraction of the eye height covered from the top) at the inner and outer end, lower lid, iris scale, iris offset
const EYE_STATE = {
  neutral: { li: 0.12, lo: 0.12, low: 0, is: 1, dx: 0, dy: 0 },
  wide: { li: 0, lo: 0, low: 0, is: 0.8, dx: 0, dy: 0, grow: 1.12 },
  smile: { li: 0.08, lo: 0.08, low: 0.3, is: 1, dx: 0, dy: -1 },
  sad: { li: 0.1, lo: 0.42, low: 0, is: 1, dx: 0, dy: 3 },
  angry: { li: 0.42, lo: 0.08, low: 0, is: 0.95, dx: 0, dy: 2 },
  lookup: { li: 0.1, lo: 0.1, low: 0, is: 1, dx: 5, dy: -7 },
  narrow: { li: 0.38, lo: 0.32, low: 0.06, is: 1, dx: 0, dy: 2 },
  worried: { li: 0.04, lo: 0.3, low: 0, is: 0.9, dx: 0, dy: 1, grow: 1.06 },
};

function eyeOne(style, stateName, view, dir) {
  const S = EYE_STYLES[style];
  const st = EYE_STATE[stateName];
  const cx = dir * EYE_X;
  const cy = EYE_Y;
  const g = st?.grow || 1;
  const rx = S.rx * g;
  const ry = S.ry * g;
  if (!facing(view, cx, cy, 8)) return [];
  const W = (pts) => wrapAll(view, pts);
  if (stateName === 'laugh') {
    const arc = [];
    for (let i = 0; i <= 6; i++) {
      const t = i / 6;
      arc.push([cx - rx * 0.95 + rx * 1.9 * t, cy + 4 - Math.sin(Math.PI * t) * ry * 0.78]);
    }
    return [{ d: ribbon(W(arc), (t) => 6.5 - 2.4 * Math.abs(t - 0.5)), fill: '$outline', opacity: 0.92 }];
  }
  const lensPts = [];
  const N = 14;
  for (let i = 0; i < N; i++) {
    const t = (i / N) * Math.PI * 2;
    let ux = Math.cos(t);
    let uy = Math.sin(t);
    if (S.lens) uy = Math.sign(uy) * Math.abs(uy) ** 1.25;
    lensPts.push([cx + ux * rx, cy + uy * ry]);
  }
  const out = [fill(smooth(W(lensPts)), '$eye_white')];
  const irisR = S.ir * st.is * (g > 1 ? 1.05 : 1);
  const ic = [cx + st.dx * dir * 0, cy + st.dy];
  const icx = cx + (stateName === 'lookup' ? st.dx : 0);
  out.push(fill(wrappedEllipseD(view, icx, ic[1], irisR, irisR, 12), '$iris'));
  out.push(fill(wrappedEllipseD(view, icx - irisR * 0.32, ic[1] - irisR * 0.34, S.hi, S.hi, 8), '#ffffff'));
  // lid: skin coloured cap above a line from the inner to the outer end
  if (st.li > 0 || st.lo > 0) {
    const lid = [];
    const steps = 8;
    for (let i = 0; i <= steps; i++) {
      const t = i / steps;
      const u = -dir * rx + dir * 2 * rx * t;
      const depth = st.li + (st.lo - st.li) * t;
      lid.push([cx + u, cy - ry + 2 * ry * depth - Math.sin(Math.PI * t) * ry * 0.1]);
    }
    const capPts = [[cx - dir * (rx + 4), cy - ry - 8], ...lid.map(([x, y], i) => (i === 0 ? [x - dir * 4, y] : i === lid.length - 1 ? [x + dir * 4, y] : [x, y])), [cx + dir * (rx + 4), cy - ry - 8]];
    out.push(fill(poly(W(capPts)), '$skin'));
    out.push({ d: ribbon(W(lid), (t) => 4.2 - 1.2 * t), fill: '$outline', opacity: 0.85 });
  }
  if (st.low > 0) {
    const low = [];
    const steps = 8;
    for (let i = 0; i <= steps; i++) {
      const t = i / steps;
      const u = -rx - 3 + (2 * rx + 6) * t;
      low.push([cx + u, cy + ry - 2 * ry * st.low + Math.sin(Math.PI * t) * ry * 0.34]);
    }
    const capPts = [...low, [cx + rx + 4, cy + ry + 8], [cx - rx - 4, cy + ry + 8]];
    out.push(fill(poly(W(capPts)), '$skin'));
  }
  return out;
}

export function eyeShapes(style, stateName, view) {
  return [...eyeOne(style, stateName, view, -1), ...eyeOne(style, stateName, view, 1)];
}

// ---------------------------------------------------------------- brows
export const BROW_STYLES = { soft: { w: 7.5, taper: 0.45 }, bold: { w: 11.5, taper: 0.25 }, thin: { w: 4.8, taper: 0.4 } };
export const BROW_STATES = ['neutral', 'raised', 'sad', 'angry', 'confused', 'think', 'worried', 'determined'];

// offsets [inner, outer], arch height, thickness factor; 'perSide' overrides for asymmetric states
const BROW_STATE = {
  neutral: { i: 0, o: 4, arch: 5, k: 1 },
  raised: { i: -12, o: -9, arch: 7, k: 1 },
  sad: { i: -11, o: 5, arch: 3, k: 1 },
  angry: { i: 9, o: -7, arch: -1, k: 1.15 },
  confused: { side: { '-1': { i: -9, o: -14, arch: 8, k: 1 }, '1': { i: 4, o: 0, arch: 1, k: 1 } } },
  think: { side: { '-1': { i: -9, o: -12, arch: 7, k: 1 }, '1': { i: 0, o: 4, arch: 5, k: 1 } } },
  worried: { i: -13, o: 1, arch: 5, k: 1 },
  determined: { i: 8, o: -1, arch: 0, k: 1.2 },
};

export function browShapes(style, stateName, view) {
  const S = BROW_STYLES[style];
  const out = [];
  for (const dir of [-1, 1]) {
    if (!facing(view, dir * EYE_X, EYE_Y, 8)) continue;
    const base = BROW_STATE[stateName];
    const st = base.side ? base.side[String(dir)] : base;
    const pts = [];
    for (let i = 0; i <= 3; i++) {
      const t = i / 3;
      pts.push([dir * (15 + 47 * t), BROW_Y + st.i + (st.o - st.i) * t - Math.sin(Math.PI * t) * st.arch]);
    }
    const width = (t) => S.w * st.k * (1 - S.taper * t);
    out.push({ d: ribbon(wrapAll(view, pts), width), fill: '$outline', opacity: 0.9 });
  }
  return out;
}

// ---------------------------------------------------------------- mouths
export const MOUTH_NAMES = ['rest', 'ai', 'e', 'o', 'u', 'mbp', 'fv', 'smile', 'laugh', 'frown', 'squiggle', 'side', 'wobble', 'firm', 'grit'];
const CAVITY = '#5b1d2a';
const TONGUE = '#e8707f';
const TEETH = '#ffffff';

export function mouthShapes(name, view) {
  // the mouth is placed with the squeeze model: centre on the surface, width scaled by the foreshortening
  const c = wrap(view, 0, MOUTH_Y);
  let sx = Math.max(0.3, Math.cos(rad(YAW[view])));
  let x0 = c[0];
  if (view === 'profile') {
    const [, xr] = edges(view, MOUTH_Y);
    x0 = xr - 14;
  }
  const y0 = MOUTH_Y;
  const P = (pts) => pts.map(([u, v]) => [x0 + u * sx, y0 + v]);
  const line = (pts, w) => ({ d: ribbon(P(pts), w), fill: '$outline', opacity: 0.9 });
  const rr = (u0, v0, u1, v1, r) => smooth(P([[u0 + r, v0], [u1 - r, v0], [u1, v0 + r], [u1, v1 - r], [u1 - r, v1], [u0 + r, v1], [u0, v1 - r], [u0, v0 + r]]), { tension: 0.9 });
  const oval = (cu, cv, ru, rv) => smooth(P(faceEllipse(cu, cv, ru, rv, 18)));
  switch (name) {
    case 'rest': return [line([[-14, 0], [-6, 3.5], [6, 3.5], [14, 0]], 4.6)];
    case 'smile': return [line([[-18, -3], [-10, 4], [0, 7], [10, 4], [18, -3]], 4.8)];
    case 'frown': return [line([[-15, 6], [-7, -1], [7, -1], [15, 6]], 4.8)];
    case 'mbp': return [line([[-14, 2], [14, 2]], 5.4)];
    case 'firm': return [line([[-14, 2], [-4, 3], [8, 2], [15, -1]], 5)];
    case 'squiggle': return [line([[-16, 3], [-9, -2], [-2, 4], [5, -2], [12, 3], [17, 0]], 4.4)];
    case 'wobble': return [line([[-14, 4], [-8, -1], [-1, 4], [6, -1], [14, 4]], 4.4)];
    case 'side': return [line([[-10, 4], [-2, 4], [8, 1], [15, -4]], 4.8)];
    case 'ai': return [
      fill(oval(0, 7, 15, 12.5), CAVITY),
      fill(rr(-11, -1, 11, 4.5, 2), TEETH),
      fill(oval(0, 13.5, 8.5, 4.6), TONGUE),
    ];
    case 'e': return [
      fill(rr(-17, 0, 17, 11, 5), CAVITY),
      fill(rr(-14, 0.2, 14, 4.8, 1.8), TEETH),
      fill(oval(0, 9, 8, 2.6), TONGUE),
    ];
    case 'o': return [
      fill(oval(0, 7, 8.5, 10.5), CAVITY),
      fill(oval(0, 11, 5, 4), TONGUE),
      { d: smooth(P(faceEllipse(0, 7, 10.2, 12.2, 18))) + smooth(P(faceEllipse(0, 7, 8.2, 10.2, 18)).reverse()), fill: '$lip', opacity: 0.9 },
    ];
    case 'u': return [
      fill(oval(0, 5, 5.5, 5.5), CAVITY),
      { d: smooth(P(faceEllipse(0, 5, 9.4, 8.4, 18))) + smooth(P(faceEllipse(0, 5, 5.2, 5.2, 18)).reverse()), fill: '$lip', opacity: 0.95 },
    ];
    case 'fv': return [
      fill(rr(-14, -1, 14, 9, 4.5), CAVITY),
      fill(rr(-11, -0.8, 11, 4.6, 1.6), TEETH),
      fill(smooth(P([[-13, 4.4], [0, 7.2], [13, 4.4], [11, 10.5], [0, 12], [-11, 10.5]]), { tension: 1 }), '$lip'),
    ];
    case 'laugh': return [
      fill(smooth(P([[-23, -4], [-12, -6], [0, -6.5], [12, -6], [23, -4], [19, 9], [9, 17], [0, 18.5], [-9, 17], [-19, 9]]), { tension: 0.9 }), CAVITY),
      fill(smooth(P([[-20, -4], [0, -6], [20, -4], [18.5, 1.5], [0, 0.5], [-18.5, 1.5]]), { tension: 0.9 }), TEETH),
      fill(smooth(P([[-10, 14], [0, 9], [10, 14], [0, 18.5]]), { tension: 1 }), TONGUE),
    ];
    case 'grit': return [
      fill(rr(-18, -5, 18, 8, 4.5), CAVITY),
      fill(rr(-15.5, -3.5, 15.5, 6.5, 2.5), TEETH),
      { d: poly(P([[-0.9, -3.5], [0.9, -3.5], [0.9, 6.5], [-0.9, 6.5]])), fill: '$outline', opacity: 0.5 },
      { d: poly(P([[-8.9, -3.5], [-7.1, -3.5], [-7.1, 6.5], [-8.9, 6.5]])), fill: '$outline', opacity: 0.5 },
      { d: poly(P([[7.1, -3.5], [8.9, -3.5], [8.9, 6.5], [7.1, 6.5]])), fill: '$outline', opacity: 0.5 },
    ];
    default: throw new Error(`unknown mouth ${name}`);
  }
}
```

`assets/storymaker/lib/hair.mjs`:

```js
// Hair styles. A style is a set of bands on the head surface (a hairline curve and an optional upper curve) plus
// a few extra solids (bun, ponytail, back mass). All of it is built on the head surface model, so front,
// three-quarter and profile agree. Styles return {back, front}: back goes to the hair_back slot (behind the
// head and the body), front to the hair slot (over the head).

import { fill, shade, light, smooth, poly, circle, ellipse, ribbon, interpSmooth, rad, rng } from './util.mjs';
import { YAW } from './dims.mjs';
import { HEAD_TOP, edges, surf, hw } from './headgeo.mjs';

const lamLeft = (view) => -90 - YAW[view];
const lamRight = (view) => 90 - YAW[view];
const clampv = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

/** Hairline height as a function of azimuth from a table over |lambda|; 'skew' makes the left/right sides differ. */
function hairlineFn(table, skew = 0) {
  return (lam) => {
    let a = Math.abs(((lam + 180) % 360 + 360) % 360 - 180); // fold to 0..180
    let y = interpSmooth(table, a);
    if (skew) {
      const s = lam > 0 ? 1 : -1;
      y += skew * s * Math.exp(-((a - 20) ** 2) / 1800) * (a < 120 ? 1 : 0);
    }
    return y;
  };
}

const norm = (lam) => ((lam + 180) % 360 + 360) % 360 - 180;

/** A cap: everything above the hairline. */
export function capShape(view, { low, t = 11, tLow = 4, color }) {
  const lL = lamLeft(view);
  const lR = lamRight(view);
  const N = 28;
  const lower = [];
  for (let i = 0; i <= N; i++) {
    const lam = lL + ((lR - lL) * i) / N;
    const [x, y] = surf(view, norm(lam), low(norm(lam)), tLow);
    lower.push([x, y]);
  }
  const yR = low(norm(lR));
  const yL = low(norm(lL));
  const rowsUp = (yStart, side) => {
    const out = [];
    const ys = [];
    const rowsN = 12;
    for (let i = 0; i <= rowsN; i++) {
      const u = i / rowsN;
      ys.push(yStart + (HEAD_TOP - yStart) * (1 - Math.cos((Math.PI / 2) * u)));
    }
    for (const y of ys) {
      const g = tLow + (t - tLow) * clampv((yStart - y) / 70, 0, 1);
      const [xl, xr] = edges(view, y, g);
      out.push([side > 0 ? xr : xl, y]);
    }
    return out;
  };
  const right = rowsUp(yR, 1);
  const left = rowsUp(yL, -1);
  const top = [(right[right.length - 1][0] + left[left.length - 1][0]) / 2, HEAD_TOP - t * 0.9];
  const pts = [...lower, ...right.slice(1, -1), top, ...left.slice(1, -1).reverse()];
  pts[0] = [pts[0][0], pts[0][1], 'c'];
  pts[N] = [pts[N][0], pts[N][1], 'c'];
  const d = smooth(pts, { tension: 1 });
  return { d, lower, right: right.slice(0, -1), left };
}

/** A band between two curves over an azimuth range. */
export function bandShape(view, { a, b, low, high, tLow = 5, tHigh = 8 }) {
  const lL = lamLeft(view);
  const lR = lamRight(view);
  const from = Math.max(a, lL);
  const to = Math.min(b, lR);
  if (to - from < 4) return null;
  const N = 14;
  const lower = [];
  const upper = [];
  for (let i = 0; i <= N; i++) {
    const lam = from + ((to - from) * i) / N;
    lower.push(surf(view, norm(lam), low(norm(lam)), tLow).slice(0, 2));
    upper.push(surf(view, norm(lam), high(norm(lam)), tHigh).slice(0, 2));
  }
  const pts = [...lower, ...upper.reverse()];
  pts[0] = [pts[0][0], pts[0][1], 'c'];
  pts[N] = [pts[N][0], pts[N][1], 'c'];
  return smooth(pts, { tension: 0.9 });
}

function strands(view, lams, y0, y1, g0, g1, opacity = 0.12) {
  const out = [];
  for (const lam of lams) {
    const L = norm(lam);
    const lo = surf(view, L, y0, g0);
    const hi = surf(view, L, y1, g1);
    if (lo[2] < -2 || hi[2] < -2) continue;
    const mid = [(lo[0] + hi[0]) / 2 + (lam > 0 ? 3 : -3), (lo[1] + hi[1]) / 2];
    out.push({ d: ribbon([[lo[0], lo[1]], mid, [hi[0], hi[1]]], 3), fill: '$outline', opacity });
  }
  return out;
}

function sheen(view, y, opacity = 0.18) {
  // a lighter swoosh over the crown on the lit (left) side
  const lams = [-100 - YAW[view] * 0.5, -70, -40, -10].map((l) => norm(l));
  const pts = [];
  for (const lam of lams) {
    const p = surf(view, lam, y + Math.abs(lam) * 0.05, 8);
    if (p[2] < 0) continue;
    pts.push([p[0], p[1]]);
  }
  if (pts.length < 3) return [];
  const mid = pts[Math.floor(pts.length / 2)];
  return [{ d: ribbon([[mid[0] - 22, mid[1] + 2], [mid[0], mid[1] - 4], [mid[0] + 20, mid[1] + 3]], (t) => 8 - 6 * Math.abs(t - 0.5)), fill: '#ffffff', opacity }];
}

function sideShadeCap(view, cap) {
  // the second tone follows the cap's own right edge, thinning out to nothing at both ends
  const rows = cap.right.filter((p) => p[1] > HEAD_TOP + 14);
  if (rows.length < 3) return shade('M0 0L0 0Z', 0);
  const y0 = rows[0][1];
  const y1 = rows[rows.length - 1][1];
  const width = view === 'profile' ? 8 : 15;
  const outer = rows.map(([x, y]) => [x - 0.5, y]);
  const inner = rows.map(([x, y]) => {
    const t = (y - y0) / (y1 - y0);
    return [x - width * Math.sin(Math.PI * t) ** 0.9 - 0.5, y];
  });
  return shade(smooth([...outer, ...inner.reverse()], { tension: 1 }), 0.12);
}

const TABLES = {
  buzz: [[0, -184], [50, -172], [80, -150], [100, -132], [140, -92], [180, -64]],
  short: [[0, -188], [40, -182], [70, -164], [95, -138], [130, -100], [180, -66]],
  messy: [[0, -184], [40, -178], [70, -158], [95, -134], [130, -98], [180, -64]],
  curly: [[0, -178], [40, -172], [70, -150], [95, -122], [130, -90], [180, -56]],
  bun: [[0, -186], [40, -180], [70, -162], [95, -142], [130, -118], [180, -100]],
  ponytail: [[0, -184], [40, -178], [70, -160], [95, -140], [130, -112], [180, -92]],
  bob: [[0, -168], [25, -160], [40, -140], [55, -112], [70, -86], [90, -62], [120, -58], [180, -64]],
  long: [[0, -166], [25, -158], [40, -138], [55, -104], [70, -66], [90, -20], [120, 30], [180, 40]],
};

export const HAIR_STYLES = ['bald', 'buzz', 'short', 'side-part', 'messy', 'curly', 'bun', 'ponytail', 'bob', 'long', 'balding'];

/** Back-of-head solids drawn behind everything. */
function backMass(view, style) {
  const out = [];
  const ph = rad(YAW[view]);
  const sinp = Math.sin(ph);
  const cosp = Math.cos(ph);
  const spread = (w, dz) => [-(w * cosp + dz * sinp), w * cosp];
  if (style === 'bun') {
    const bx = -52 * sinp;
    const by = -236;
    out.push(fill(circle(bx, by, 38), '$hair'));
    out.push(light(ellipse(bx - 10, by - 12, 14, 8), 0.2));
    out.push({ d: ribbon([[bx - 36, by + 20], [bx, by + 34], [bx + 36, by + 20]], 6), fill: '$accent', opacity: 0.9 });
  } else if (style === 'ponytail') {
    const bx = -86 * sinp;
    if (view !== 'front') {
      out.push(fill(smooth([[bx - 6, -165], [bx - 38, -130], [bx - 52, -70], [bx - 38, -18], [bx - 14, -52], [bx - 8, -110]], { tension: 1 }), '$hair'));
      out.push(shade(smooth([[bx - 22, -150], [bx - 36, -100], [bx - 28, -40], [bx - 20, -80]], { tension: 1 }), 0.14));
      out.push({ d: ribbon([[bx - 20, -158], [bx - 2, -150]], 8), fill: '$accent', opacity: 0.95 });
    }
  } else if (style === 'bob' || style === 'long') {
    const bottom = style === 'bob' ? -52 : 112;
    const [xl, xr] = spread(hw(-120) + 12, -20);
    const wob = style === 'bob' ? 0 : 6;
    const pts = [
      [xl * 0.9, -190], [xr * 0.9, -190], [xr, -140], [xr + wob, -90], [xr * 0.96 + wob, bottom], [xl * 0.96 - wob, bottom], [xl - wob, -90], [xl, -140],
    ];
    out.push(fill(smooth(pts, { tension: 1 }), '$hair'));
    out.push(shade(smooth([[xr * 0.3, -170], [xr, -140], [xr + wob, -90], [xr * 0.96 + wob, bottom], [xr * 0.3, bottom - 10]], { tension: 1 }), 0.14));
  } else if (style === 'curly') {
    const pts = [];
    const r = rng(7);
    for (let i = 0; i < 9; i++) {
      const lam = -170 + i * 42.5;
      const p = surf(view, norm(lam), -150 + r() * 10, 18);
      pts.push(fill(circle(p[0] * 1.02, p[1] + 6, 26 + r() * 4), '$hair'));
    }
    out.push(...pts);
  }
  return out;
}

/** Front hair (over the head) for a style in a view. */
function frontHair(view, style) {
  const out = [];
  switch (style) {
    case 'buzz': {
      const c = capShape(view, { low: hairlineFn(TABLES.buzz), t: 4, tLow: 3 });
      out.push(fill(c.d, '$hair'), { d: c.d, fill: '$outline', opacity: 0.08 }, sideShadeCap(view, c));
      break;
    }
    case 'short': {
      const c = capShape(view, { low: hairlineFn(TABLES.short), t: 11, tLow: 4 });
      out.push(fill(c.d, '$hair'), sideShadeCap(view, c), ...sheen(view, -204), ...strands(view, [-50, -20, 20, 50], -186, -214, 5, 9));
      break;
    }
    case 'side-part': {
      const c = capShape(view, { low: hairlineFn(TABLES['short'], 14), t: 13, tLow: 4 });
      out.push(fill(c.d, '$hair'), sideShadeCap(view, c), ...sheen(view, -206));
      // the parting line and the sweep
      const p0 = surf(view, norm(34), -190, 4);
      const p1 = surf(view, norm(10), -218, 12);
      const p2 = surf(view, norm(-30), -214, 12);
      if (p0[2] > -2) out.push({ d: ribbon([[p0[0], p0[1]], [p1[0], p1[1]], [p2[0], p2[1]]], 3.4), fill: '$outline', opacity: 0.28 });
      out.push(...strands(view, [-60, -35, -10], -176, -208, 5, 11, 0.14));
      break;
    }
    case 'messy': {
      const c = capShape(view, { low: hairlineFn(TABLES.messy), t: 14, tLow: 4 });
      out.push(fill(c.d, '$hair'));
      const r = rng(11);
      for (let i = 0; i < 9; i++) {
        const lam = norm(-150 - YAW[view] + i * (300 / 8) + (r() - 0.5) * 12);
        const a = surf(view, norm(lam - 11), -196, 12);
        const b = surf(view, norm(lam + 11), -196, 12);
        const tipy = -222 - 14 - r() * 14;
        const tip = surf(view, lam, -214, 16);
        if (tip[2] < -4 && Math.abs(lam) > 100) continue;
        out.push(fill(poly([[a[0], a[1]], [tip[0] + (r() - 0.5) * 8, tipy], [b[0], b[1]]]), '$hair'));
      }
      out.push(sideShadeCap(view, c), ...sheen(view, -204));
      break;
    }
    case 'curly': {
      const c = capShape(view, { low: hairlineFn(TABLES.curly), t: 20, tLow: 5 });
      out.push(fill(c.d, '$hair'));
      const r = rng(5);
      for (let i = 0; i < 11; i++) {
        const lam = norm(-120 - YAW[view] + i * 24);
        const y = -178 - 28 * Math.sin((i / 10) * Math.PI) + (r() - 0.5) * 6;
        const p = surf(view, lam, y > -205 ? y : -205, 18);
        if (p[2] < -6) continue;
        out.push(fill(circle(p[0], p[1] - 6, 21 + r() * 5), '$hair'));
      }
      out.push(sideShadeCap(view, c), ...sheen(view, -204));
      for (const lam of [-60, -25, 15, 50]) {
        const p = surf(view, norm(lam), -190, 18);
        if (p[2] > 0) out.push({ d: ribbon([[p[0] - 9, p[1] - 6], [p[0], p[1] - 14], [p[0] + 9, p[1] - 6]], 3), fill: '$outline', opacity: 0.14 });
      }
      break;
    }
    case 'bun': {
      const c = capShape(view, { low: hairlineFn(TABLES.bun), t: 11, tLow: 4 });
      out.push(fill(c.d, '$hair'), sideShadeCap(view, c), ...sheen(view, -204), ...strands(view, [-45, -15, 20, 50], -186, -214, 5, 9));
      break;
    }
    case 'ponytail': {
      const c = capShape(view, { low: hairlineFn(TABLES.ponytail), t: 11, tLow: 4 });
      out.push(fill(c.d, '$hair'), sideShadeCap(view, c), ...sheen(view, -204), ...strands(view, [-45, -15, 20, 50], -186, -214, 5, 9));
      break;
    }
    case 'bob': {
      const c = capShape(view, { low: hairlineFn(TABLES.bob), t: 13, tLow: 6 });
      out.push(fill(c.d, '$hair'), sideShadeCap(view, c), ...sheen(view, -206));
      out.push(...strands(view, [-62, -44, 44, 62], -126, -190, 7, 11, 0.12));
      break;
    }
    case 'long': {
      const c = capShape(view, { low: hairlineFn(TABLES.long), t: 13, tLow: 6 });
      out.push(fill(c.d, '$hair'), sideShadeCap(view, c), ...sheen(view, -206));
      out.push(...strands(view, [-70, -50, 50, 70], -90, -190, 7, 11, 0.12));
      break;
    }
    case 'balding': {
      // thin hair hugging the skull; a skin coloured cap hides the crown, leaving a horseshoe
      const hair = capShape(view, { low: hairlineFn([[0, -186], [40, -180], [70, -150], [95, -112], [130, -88], [180, -66]]), t: 2, tLow: 2 });
      const bald = capShape(view, { low: hairlineFn([[0, -176], [40, -176], [60, -163], [72, -158], [95, -152], [130, -150], [180, -148]]), t: 2, tLow: 2 });
      out.push(fill(hair.d, '$hair'), fill(bald.d, '$skin'), sideShadeCap(view, bald));
      break;
    }
    default: break;
  }
  return out;
}

export function hairShapes(style, view) {
  if (style === 'bald') return { back: [], front: [] };
  return { back: backMass(view, style), front: frontHair(view, style) };
}
```

`assets/storymaker/lib/extras.mjs`:

```js
// Glasses, facial hair and head accessories, all placed on the head surface model.

import { fill, shade, smooth, ellipse, ribbon, rad, interp } from './util.mjs';
import { YAW } from './dims.mjs';
import {
  edges, surf, wrap, facing, wrapAll, faceEllipse, hw, EYE_X, EYE_Y, MOUTH_Y,
} from './headgeo.mjs';
import { capShape } from './hair.mjs';

const FRAME = '#262a3c';

// ---------------------------------------------------------------- glasses
export const GLASSES = ['none', 'round', 'square'];

function ring(view, cx, cy, rx, ry, w, square) {
  const outer = [];
  const inner = [];
  const N = 22;
  for (let i = 0; i < N; i++) {
    const t = (i / N) * Math.PI * 2;
    let ux = Math.cos(t);
    let uy = Math.sin(t);
    if (square) {
      const k = 0.78;
      ux = Math.sign(ux) * Math.abs(ux) ** k;
      uy = Math.sign(uy) * Math.abs(uy) ** k;
    }
    outer.push([cx + ux * rx, cy + uy * ry]);
    inner.push([cx + ux * (rx - w), cy + uy * (ry - w)]);
  }
  const o = wrapAll(view, outer, 4);
  const inn = wrapAll(view, inner, 4);
  return { frame: smooth(o) + smooth(inn.reverse()), lens: smooth(wrapAll(view, inner, 4)) };
}

export function glassesShapes(kind, view) {
  const out = [];
  const rx = kind === 'square' ? 27 : 25;
  const ry = kind === 'square' ? 21 : 24;
  const sides = [];
  for (const dir of [-1, 1]) {
    if (!facing(view, dir * EYE_X, EYE_Y, 8)) continue;
    const r = ring(view, dir * EYE_X, EYE_Y, rx, ry, 4.6, kind === 'square');
    out.push(fill(r.lens, '#bfe7ff', 0.16), fill(r.frame, FRAME));
    sides.push(dir);
  }
  if (sides.length === 2) out.push({ d: ribbon(wrapAll(view, [[-EYE_X + rx - 1, EYE_Y - 4], [0, EYE_Y - 8], [EYE_X - rx + 1, EYE_Y - 4]], 4), 4), fill: FRAME });
  // temple arm of the near side towards the ear
  if (view !== 'front') {
    const a = wrap(view, -EYE_X - rx + 2, EYE_Y - 4, 4);
    const e = surf(view, -92, EYE_Y - 6, 3);
    out.push({ d: ribbon([[a[0], a[1]], [(a[0] + e[0]) / 2, EYE_Y - 6], [e[0], e[1]]], 3.6), fill: FRAME });
  }
  return out;
}

// ---------------------------------------------------------------- facial hair
export const FACIAL_HAIR = ['none', 'stubble', 'mustache', 'goatee', 'beard'];

/**
 * Region of the lower face below the curve top(|lambda|), between azimuths -reach and +reach, down to the jaw line.
 * Where the region ends at the head's limb the jaw silhouette is followed, otherwise a meridian.
 */
function jawRegion(view, top, { reach = 100, grow = 5 } = {}) {
  const lL = -90 - YAW[view];
  const lR = 90 - YAW[view];
  const from = Math.max(lL, -reach);
  const to = Math.min(lR, reach);
  const upper = [];
  const N = 16;
  for (let i = 0; i <= N; i++) {
    const lam = from + ((to - from) * i) / N;
    const p = surf(view, lam, top(Math.abs(lam)), grow);
    upper.push([p[0], p[1]]);
  }
  const rowsDown = (lam, limb, side) => {
    const yTop = top(Math.abs(lam));
    const out = [];
    for (let i = 1; i <= 10; i++) {
      const y = yTop + ((-7 - yTop) * i) / 10;
      out.push(limb ? [edges(view, y, grow)[side > 0 ? 1 : 0], y] : [surf(view, lam, y, grow)[0], y]);
    }
    return out;
  };
  const right = rowsDown(to, to === lR, 1);
  const left = rowsDown(from, from === lL, -1).reverse();
  const pts = [...upper, ...right, [(right[right.length - 1][0] + left[0][0]) / 2, -4], ...left];
  pts[0] = [pts[0][0], pts[0][1], 'c'];
  pts[N] = [pts[N][0], pts[N][1], 'c'];
  return pts;
}

function mouthHole(view) {
  const c = wrap(view, 0, MOUTH_Y);
  const sx = Math.max(0.3, Math.cos(rad(YAW[view])));
  let x0 = c[0];
  if (view === 'profile') x0 = edges(view, MOUTH_Y)[1] - 14;
  const pts = faceEllipse(0, 0, 27, 14, 16).map(([u, v]) => [x0 + u * sx, MOUTH_Y + v - 1]);
  return smooth(pts.reverse());
}

function mustache(view) {
  const c = wrap(view, 0, -64);
  const sx = Math.max(0.3, Math.cos(rad(YAW[view])));
  const x0 = view === 'profile' ? edges(view, -64)[1] - 16 : c[0];
  const d = smooth([[x0 - 30 * sx, -59], [x0 - 14 * sx, -69], [x0, -66], [x0 + 14 * sx, -69], [x0 + 30 * sx, -59], [x0 + 16 * sx, -56], [x0, -60], [x0 - 16 * sx, -56]], { tension: 1 });
  return [fill(d, '$hair'), shade(d, 0.1)];
}

export function facialHairShapes(kind, view) {
  const out = [];
  if (kind === 'stubble') {
    const top = (a) => interp([[0, -62], [25, -63], [45, -66], [65, -76], [85, -100], [100, -110]], a);
    out.push(fill(smooth(jawRegion(view, top, { grow: 1.5, reach: 97 }), { tension: 1 }), '$hair', 0.2));
  } else if (kind === 'beard') {
    const top = (a) => interp([[0, -60], [25, -62], [45, -66], [65, -76], [85, -98], [100, -106]], a);
    const d = smooth(jawRegion(view, top, { grow: 6 }), { tension: 1 }) + mouthHole(view);
    out.push(fill(d, '$hair'), shade(d, 0.1));
  } else if (kind === 'goatee') {
    const top = (a) => -33 + a * 0.12;
    const d = smooth(jawRegion(view, top, { reach: 36, grow: 5 }), { tension: 1 }) + mouthHole(view);
    out.push(fill(d, '$hair'), shade(d, 0.1));
  }
  if (kind === 'mustache' || kind === 'goatee' || kind === 'beard') out.push(...mustache(view));
  return out;
}

// ---------------------------------------------------------------- accessories
export const ACCESSORIES = ['none', 'headset', 'cap', 'helmet', 'headscarf'];

const DARK = '#2a2d3e';

function earCup(view, side) {
  const y = -98;
  if (view === 'front') {
    const x = side * (hw(y) + 14);
    return [fill(ellipse(x, y, 16, 27), DARK), fill(ellipse(x + side * 2, y, 8, 15), '$accent')];
  }
  if (side > 0) return [];
  const [x] = surf(view, -92, y, 6);
  const w = 12 + 8 * Math.sin(rad(YAW[view]));
  return [fill(ellipse(x, y, w, 27), DARK), fill(ellipse(x - 1, y, w * 0.5, 15), '$accent')];
}

function headband(view, lift = 10, width = 9) {
  const ph = rad(YAW[view]);
  const pts = [];
  for (let i = 0; i <= 18; i++) {
    const th = -Math.PI / 2 + (Math.PI * i) / 18;
    const X = (hw(-130) + lift) * Math.sin(th);
    pts.push([X * Math.cos(ph), -98 - (125 + lift) * Math.cos(th)]);
  }
  return { d: ribbon(pts, width), fill: DARK };
}

export function accessoryShapes(kind, view) {
  const out = [];
  if (kind === 'headset') {
    out.push(headband(view));
    out.push(...earCup(view, -1), ...earCup(view, 1));
    // microphone boom on the near side
    const a = view === 'front' ? [-(hw(-98) + 14), -86] : [surf(view, -92, -86, 8)[0], -86];
    const m = wrap(view, -38, -52, 8);
    const mid = [(a[0] + m[0]) / 2 - 8, -50];
    out.push({ d: ribbon([a, mid, [m[0], m[1]]], 4.5), fill: DARK });
    out.push(fill(ellipse(m[0], m[1], 8, 6), '$accent'));
  } else if (kind === 'cap' || kind === 'helmet' || kind === 'headscarf') {
    const hl = (lam) => (kind === 'headscarf' ? (Math.abs(lam) < 100 ? -176 + Math.abs(lam) * 0.15 : -120) : -176 + Math.abs(lam) * 0.05);
    const color = kind === 'helmet' ? '#f5c518' : kind === 'headscarf' ? '$accent2' : '$accent';
    const c = capShape(view, { low: hl, t: kind === 'helmet' ? 22 : 17, tLow: kind === 'helmet' ? 20 : 15 });
    out.push(fill(c.d, color), shade(c.d, 0.08));
    if (kind === 'cap') {
      const v = wrap(view, 0, -176, 0);
      const front = view === 'profile' ? [v[0] + 4, -170] : [v[0] + 16 * Math.sin(rad(YAW[view])), -170];
      const vis = smooth([[front[0] - 62 * Math.cos(rad(YAW[view])) - 6, -176], [front[0] + 62 * Math.cos(rad(YAW[view])) + 6, -176], [front[0] + 66 * Math.cos(rad(YAW[view])) + 26 * Math.sin(rad(YAW[view])), -166], [front[0] + 40 * Math.sin(rad(YAW[view])) + 40, -158], [front[0] - 66 * Math.cos(rad(YAW[view])) + 20 * Math.sin(rad(YAW[view])), -164]], { tension: 1 });
      out.push(fill(vis, color), shade(vis, 0.22));
    } else if (kind === 'helmet') {
      out.push(fill(smooth([[-120, -170], [120, -170], [124, -160], [-124, -160]].map(([x, y]) => [x * Math.cos(rad(YAW[view])) * 0.9, y]), { tension: 1 }), color));
      out.push({ d: ribbon([[surf(view, -20, -200, 22)[0], -206], [surf(view, 20, -200, 22)[0], -206]], 10), fill: '#ffffff', opacity: 0.35 });
    }
  }
  return out;
}
```


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node --test assets/storymaker/test/parts.test.mjs`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add assets/storymaker/lib/headgeo.mjs assets/storymaker/lib/face.mjs assets/storymaker/lib/hair.mjs assets/storymaker/lib/extras.mjs assets/storymaker/test/parts.test.mjs
git commit -m "feat(storymaker): parametric head, face, hair and accessories" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```


### Task D3: Bodies, garments and the assembled rig

**Files:**
- Create: `assets/storymaker/lib/body.mjs`, `assets/storymaker/lib/clothes.mjs`, `assets/storymaker/lib/rig.mjs`
- Test: `assets/storymaker/test/rig.test.mjs`

Torso, pelvis, neck, limbs, hands (open, fist, point, hold), shoes, six outfits (tops, bottoms, shoes) and the assembly of `rig/human.json`: 13 bones with per-view rest transforms, 34 slots with per-view draw order, variants for every parameter value, five body types and the parameter lists. The test renders all four starter characters in every view with every expression, hand and mouth and fails on any missing variant or unresolved token.

- [ ] **Step 1: Write the failing test**

`assets/storymaker/test/rig.test.mjs`:

```js
// Task D3: bodies, clothes and the assembled rig/human.json.
// Run: node --test assets/storymaker/test/rig.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePath } from '../lib/pathdata.mjs';
import { buildRig } from '../lib/rig.mjs';
import { VIEWS } from '../lib/dims.mjs';
import { Ctx, characterSvg } from '../lib/svg.mjs';
import { MOUTH_NAMES } from '../lib/face.mjs';

const rig = buildRig();
const PALETTE = { bg: '#fff7e6', bg2: '#ffe0a3', fg: '#1f2a44', accent: '#ff7a45', accent2: '#36cfc9', text: '#1f2a44', outline: '#1f2a44' };
const STARTERS = [
  { body: 'average', skin: 'medium', hair: 'bun', hair_color: 'dark-brown', eyes: 'round', brows: 'soft', facial_hair: 'none', glasses: 'none', outfit: 'business', accessory: 'none' },
  { body: 'broad', skin: 'light', hair: 'side-part', hair_color: 'chestnut', eyes: 'almond', brows: 'bold', facial_hair: 'stubble', glasses: 'none', outfit: 'suit', accessory: 'none' },
  { body: 'slim', skin: 'brown', hair: 'messy', hair_color: 'black', eyes: 'big', brows: 'thin', facial_hair: 'none', glasses: 'square', outfit: 'it', accessory: 'headset' },
  { body: 'elder', skin: 'tan', hair: 'balding', hair_color: 'white', eyes: 'almond', brows: 'soft', facial_hair: 'mustache', glasses: 'round', outfit: 'cardigan', accessory: 'none' },
];

test('the rig has the 13 bones, per-view rest transforms and per-view slot order', () => {
  assert.equal(rig.id, 'rig.human/1');
  assert.deepEqual(rig.bones.map((b) => b.name), ['root', 'hips', 'torso', 'neck', 'head', 'upper_arm_l', 'lower_arm_l', 'upper_arm_r', 'lower_arm_r', 'upper_leg_l', 'lower_leg_l', 'upper_leg_r', 'lower_leg_r']);
  for (const b of rig.bones) for (const v of VIEWS) assert.ok(['x', 'y', 'angle'].every((k) => Number.isFinite(b.rest[v][k])), `${b.name} ${v}`);
  const slot = (name) => rig.slots.find((s) => s.name === name);
  for (const end of ['hand_l', 'hand_r', 'foot_l', 'foot_r']) assert.ok(slot(end), end);
  // far limbs sit behind the torso in the angled views, near limbs in front; in the front view arms sit above it
  assert.ok(slot('arm_u_l').z.three_quarter < slot('torso').z.three_quarter);
  assert.ok(slot('arm_u_r').z.three_quarter > slot('torso').z.three_quarter);
  assert.ok(slot('arm_u_l').z.front > slot('torso').z.front);
  assert.equal(rig.seat_drop, 215);
});

test('the _r limbs are on the screen right in the front view and the near side in the angled views', () => {
  const x = (bone, view) => rig.bones.find((b) => b.name === bone).rest[view].x;
  assert.ok(x('upper_arm_r', 'front') > 0 && x('upper_arm_l', 'front') < 0);
  assert.ok(x('upper_arm_r', 'three_quarter') < 0 && x('upper_arm_l', 'three_quarter') > 0);
});

test('every starter character renders in every view with every expression, hand and mouth', () => {
  const exprs = [['neutral', 'neutral', 'rest'], ['smile', 'neutral', 'smile'], ['laugh', 'raised', 'laugh'], ['sad', 'sad', 'frown'], ['angry', 'angry', 'grit'], ['wide', 'raised', 'o'], ['neutral', 'confused', 'squiggle'], ['lookup', 'think', 'side'], ['worried', 'worried', 'wobble'], ['narrow', 'determined', 'firm']];
  for (const params of STARTERS) {
    for (const view of VIEWS) {
      for (const [eyes, brows, mouth] of exprs) {
        for (const hand of ['open', 'fist', 'point', 'hold']) {
          const ctx = new Ctx();
          characterSvg(rig, params, PALETTE, ctx, { view, pose: { bones: {}, hands: { l: hand, r: hand } }, expression: { eyes, brows, mouth } });
          assert.deepEqual([...ctx.missing], [], `${params.hair} ${view} ${eyes}`);
          assert.deepEqual([...ctx.unresolved], []);
        }
      }
      for (const m of MOUTH_NAMES) {
        const ctx = new Ctx();
        characterSvg(rig, params, PALETTE, ctx, { view, mouth: m });
        assert.equal(ctx.missing.size, 0);
      }
    }
  }
});

test('all variants are valid path data and every parameter value has shapes', () => {
  for (const [slot, keys] of Object.entries(rig.variants)) {
    for (const [key, views] of Object.entries(keys)) {
      for (const v of VIEWS) {
        assert.ok(Array.isArray(views[v]), `${slot}/${key}/${v}`);
        for (const s of views[v]) assert.ok(parsePath(s.d).ok, `${slot}/${key}/${v}`);
      }
    }
  }
  for (const body of rig.params.body) {
    assert.ok(rig.bodies[body], body);
    assert.ok(rig.variants.torso[body] && rig.variants.pelvis[body], body);
  }
  for (const style of rig.params.hair) assert.ok(rig.variants.hair[style] && rig.variants.hair_back[style], style);
  for (const o of Object.values(rig.params.outfit)) {
    assert.ok(rig.variants.top_det[o.top] && rig.variants.sleeve_u_r[o.top] && rig.variants.pelvis_det[o.bottom] && rig.variants.foot_r[o.shoes]);
  }
  assert.ok(rig.params.skin.medium && rig.params.hair_color['dark-brown']);
  assert.deepEqual(Object.keys(rig.params.fixed).sort(), ['eye_white', 'iris', 'lip', 'outline']);
});

test('body types differ from the average rig only by the rest offsets they override', () => {
  assert.equal(rig.bodies.average.bones, undefined);
  assert.ok(rig.bodies.broad.bones.upper_arm_l.rest.front.x < rig.bones.find((b) => b.name === 'upper_arm_l').rest.front.x);
  assert.equal(rig.bodies.child.scale < 1, true);
});
```


- [ ] **Step 2: Run the test to verify it fails**

Run: `node --test assets/storymaker/test/rig.test.mjs`
Expected: FAIL (`ERR_MODULE_NOT_FOUND`: cannot find `assets/storymaker/lib/rig.mjs`)

- [ ] **Step 3: Implement**

`assets/storymaker/lib/body.mjs`:

```js
// Torso, pelvis, neck, limbs, hands and feet of the human rig, plus the base layers that clothes sit on.
// Shapes are in the local frame of their bone (see dims.mjs). Every function returns a list of Shapes.

import { fill, shade, light, smooth, poly, capsule, limb, roundRect, ribbon, interpSmooth, rad } from './util.mjs';
import { BODIES, YAW, LOWER_ARM, LOWER_LEG, ANKLE } from './dims.mjs';

const phi = (view) => rad(YAW[view]);
const cosP = (view) => Math.cos(phi(view));
const sinP = (view) => Math.sin(phi(view));

/** Extreme x of an elliptical section with half width h and front/back depths f/b in a view. */
export function extremes(view, h, f, b) {
  return [-Math.hypot(h * cosP(view), b * sinP(view)), Math.hypot(h * cosP(view), f * sinP(view))];
}

// ---------------------------------------------------------------- torso
/** Torso section at y (torso local, 0 = waist, negative up) for a body: [halfWidth, frontDepth, backDepth]. */
export function torsoSection(b, y) {
  const L = b.torsoLen;
  const s = Math.max(-0.2, Math.min(1.4, -y / L)); // 1 at the neck base, 0 at the waist
  const k = 1 - s; // 0 at the neck, 1 at the waist
  const hwT = [[0, b.neckHW + 12], [0.05, b.shoulderHW * 0.74], [0.11, b.shoulderHW * 0.97], [0.17, b.shoulderHW], [0.4, b.shoulderHW * 0.94], [0.72, b.waistHW * 1.03], [1, b.waistHW], [1.12, (b.waistHW + b.hipHW) / 2], [1.25, b.hipHW]];
  const dfT = [[0, 26], [0.08, b.chestD * 0.78], [0.3, b.chestD], [0.6, b.chestD * 0.96], [0.85, b.waistD + b.belly], [1, b.waistD + b.belly], [1.2, b.hipD + b.belly * 0.4]];
  const dbT = [[0, 24 + b.stoop * 0.4], [0.08, b.chestD * 0.8 + b.stoop], [0.3, b.chestD * 0.92 + b.stoop], [0.7, b.waistD * 0.9 + b.stoop * 0.4], [1, b.waistD * 0.88], [1.2, b.hipD * 0.9]];
  return [interpSmooth(hwT, k), interpSmooth(dfT, k), interpSmooth(dbT, k)];
}

/** Outline points of the torso garment down to hem y (torso local), flared by 'grow' units at the hem. */
export function torsoOutline(bodyName, view, hemY, { grow = 0, hemGrow = 0, hemRound = 12 } = {}) {
  const b = BODIES[bodyName];
  const L = b.torsoLen;
  const top = -L;
  const ys = [];
  const steps = 12;
  for (let i = 0; i <= steps; i++) ys.push(top + ((hemY - top) * i) / steps);
  const right = [];
  const left = [];
  for (const y of ys) {
    const t = (y - top) / (hemY - top);
    const g = grow + hemGrow * t * t;
    const [h, f, bk] = torsoSection(b, y);
    const [xl, xr] = extremes(view, h + g, f + g * 0.6, bk + g * 0.6);
    right.push([xr, y]);
    left.push([xl, y]);
  }
  const r = Math.max(2, Math.min(hemRound, (right[right.length - 1][0] - left[left.length - 1][0]) / 3));
  const pts = [
    [(right[0][0] + left[0][0]) / 2, top],
    ...right.slice(1, -1),
    [right[right.length - 1][0], hemY - r, 'c'],
    [right[right.length - 1][0] - r * 0.5, hemY - r * 0.2],
    [(right[right.length - 1][0] + left[left.length - 1][0]) / 2 + (right[right.length - 1][0] - left[left.length - 1][0]) * 0.3, hemY],
    [(right[right.length - 1][0] + left[left.length - 1][0]) / 2 - (right[right.length - 1][0] - left[left.length - 1][0]) * 0.3, hemY],
    [left[left.length - 1][0] + r * 0.5, hemY - r * 0.2],
    [left[left.length - 1][0], hemY - r, 'c'],
    ...left.slice(1, -1).reverse(),
  ];
  return pts;
}

export const torsoD = (bodyName, view, hemY, opts) => smooth(torsoOutline(bodyName, view, hemY, opts), { tension: 0.9 });

/** Centre line x of the garment at height y (front surface of the torso in the angled views). */
export function centreX(bodyName, view, y) {
  const b = BODIES[bodyName];
  const [, f] = torsoSection(b, y);
  return f * sinP(view);
}

export function torsoBase(bodyName, view) {
  const d = torsoD(bodyName, view, 30);
  const out = [fill(d, '$top')];
  // a crescent of the second tone on the right flank
  const b = BODIES[bodyName];
  const ys = [];
  for (let i = 0; i <= 8; i++) ys.push(-b.torsoLen * 0.78 + (b.torsoLen * 0.78 + 26) * (i / 8));
  const outer = [];
  const inner = [];
  ys.forEach((y, i) => {
    const [h, f, bk] = torsoSection(b, y);
    const [, xr] = extremes(view, h, f, bk);
    const w = (view === 'profile' ? 10 : 20) * Math.sin((Math.PI * (i + 0.5)) / (ys.length)) + 2;
    outer.push([xr, y]);
    inner.push([xr - w, y]);
  });
  out.push(shade(smooth([...outer, ...inner.reverse()], { tension: 1 }), 0.11));
  return out;
}

// ---------------------------------------------------------------- neck and pelvis
export function neckShapes(bodyName, view) {
  const b = BODIES[bodyName];
  const w = b.neckHW * 2 * (view === 'profile' ? 0.86 : 1);
  const dx = view === 'front' ? 0 : 4 * sinP(view);
  const d = limb(-52, 10, w * 0.96, w, 0, dx);
  const iw = w / 2 - 2;
  const under = poly([[dx - iw, -52], [dx + iw, -52], [dx + iw, -26], [dx + iw * 0.5, -16], [dx - iw * 0.5, -16], [dx - iw, -26]]);
  return [fill(d, '$skin'), shade(under, 0.2)];
}

export function pelvisBase(bodyName, view) {
  const b = BODIES[bodyName];
  const h = b.hipHW;
  const [xl, xr] = extremes(view, h, b.hipD + b.belly * 0.4, b.hipD * 0.95);
  const lean = 0;
  const pts = [
    [xl * 0.97 + lean, -40], [xr * 0.97 + lean, -40],
    [xr + lean, -4], [xr * 0.92 + lean, 38], [(xr + xl) / 2 + (xr - xl) * 0.22, 56],
    [(xr + xl) / 2 - (xr - xl) * 0.22, 56], [xl * 0.92 + lean, 38], [xl + lean, -4],
  ];
  return [fill(smooth(pts, { tension: 1 }), '$bottom')];
}

// ---------------------------------------------------------------- limbs
const FAR_SHADE = 0.2;
const farShade = (shapes) => shapes.filter((s) => s.fill && s.opacity === undefined).map((s) => shade(s.d, FAR_SHADE));

export function armSkin(bodyName, kind, view, far) {
  const b = BODIES[bodyName];
  const d = kind === 'upper' ? limb(0, b.upperArm, b.arm, b.arm * 0.86, 1.2) : limb(0, LOWER_ARM, b.forearm, b.forearm * 0.78, 0.8);
  const out = [fill(d, '$skin')];
  if (view !== 'front' && far) out.push(...farShade(out));
  return out;
}

export function legBase(bodyName, kind, view, far) {
  const b = BODIES[bodyName];
  const d = kind === 'upper' ? limb(0, b.upperLeg, b.thigh, b.thigh * 0.84, 1.2) : limb(0, LOWER_LEG - 14, b.shin * 1.14, b.shin * 0.82, 0.4);
  const out = [fill(d, '$bottom')];
  if (view !== 'front' && far) out.push(...farShade(out));
  return out;
}

// ---------------------------------------------------------------- hands
/** Hands hang from the wrist at local (0, LOWER_ARM). 'dir' is the thumb side (+1 towards +x). */
export function handShapes(kind, dir, view) {
  const y0 = LOWER_ARM - 4;
  const T = (pts) => pts.map(([x, y]) => [x * dir, y]);
  const out = [];
  switch (kind) {
    case 'open': {
      out.push(fill(smooth(T([[-17, y0], [-22, y0 + 30], [-21, y0 + 62], [-12, y0 + 76], [0, y0 + 79], [12, y0 + 75], [20, y0 + 62], [21, y0 + 30], [17, y0]]), { tension: 0.95 }), '$skin'));
      out.push(fill(capsule(15 * dir, y0 + 20, 33 * dir, y0 + 46, 13), '$skin'));
      for (const x of [-9, 0, 9]) out.push({ d: ribbon([[x * dir, y0 + 52], [x * dir, y0 + 72]], 2.4), fill: '$outline', opacity: 0.2 });
      out.push(shade(smooth(T([[8, y0 + 8], [20, y0 + 30], [20, y0 + 62], [13, y0 + 75], [6, y0 + 50]]), { tension: 1 }), 0.1));
      break;
    }
    case 'fist': {
      out.push(fill(roundRect(-21, y0 + 6, 42, 40, 16), '$skin'));
      for (const y of [18, 28, 38]) out.push({ d: ribbon([[-18 * dir, y0 + y], [-4 * dir, y0 + y + 1.5], [14 * dir, y0 + y]], 2.6), fill: '$outline', opacity: 0.22 });
      out.push(fill(capsule(20 * dir, y0 + 16, 26 * dir, y0 + 2, 14), '$skin'));
      break;
    }
    case 'point': {
      out.push(fill(roundRect(-20, y0 + 4, 40, 34, 14), '$skin'));
      out.push(fill(capsule(-6 * dir, y0 + 26, -6 * dir, y0 + 80, 13), '$skin'));
      out.push(fill(capsule(20 * dir, y0 + 14, 25 * dir, y0 + 2, 13), '$skin'));
      for (const y of [22, 32]) out.push({ d: ribbon([[2 * dir, y0 + y], [10 * dir, y0 + y + 1], [17 * dir, y0 + y]], 2.4), fill: '$outline', opacity: 0.22 });
      break;
    }
    case 'hold': {
      out.push(fill(roundRect(-21, y0 + 4, 44, 46, 20), '$skin'));
      out.push(fill(capsule(19 * dir, y0 + 22, 28 * dir, y0 + 6, 14), '$skin'));
      out.push({ d: ribbon([[-16 * dir, y0 + 26], [0, y0 + 30], [16 * dir, y0 + 26]], 3), fill: '$outline', opacity: 0.2 });
      out.push(shade(roundRect(-21, y0 + 30, 44, 20, 10), 0.08));
      break;
    }
    default: throw new Error(`unknown hand ${kind}`);
  }
  return out;
}

// ---------------------------------------------------------------- feet
/** Shoes in the lower-leg frame; the ankle joint is at (0, LOWER_LEG), the sole at y = LOWER_LEG + ANKLE. */
export function shoeShapes(kind, view, side, far) {
  const ay = LOWER_LEG;
  const sole = ay + ANKLE;
  const out = [];
  if (view === 'front') {
    const turn = side * 5; // toes point slightly outwards
    const w = kind === 'pumps' ? 24 : 28;
    const pts = [[-w + turn, ay - 12], [w + turn, ay - 12], [w + 4 + turn, ay + 6], [w + 2 + turn, sole], [-w - 2 + turn, sole], [-w - 4 + turn, ay + 6]];
    out.push(fill(smooth(pts, { tension: 0.9 }), '$shoes'));
    out.push(light(smooth([[-w + 8 + turn, ay + 4], [w - 8 + turn, ay + 4], [w - 5 + turn, ay + 11], [-w + 5 + turn, ay + 11]], { tension: 1 }), 0.18));
    out.push(shade(poly([[-w - 2 + turn, sole - 7], [w + 2 + turn, sole - 7], [w + 2 + turn, sole], [-w - 2 + turn, sole]]), 0.22));
    return out;
  }
  const len = view === 'profile' ? 1 : 0.86;
  const X = (x) => (x < 0 ? x : x * len); // the heel stays put, the forefoot foreshortens in the angled view
  if (kind === 'pumps') {
    const body = [[-25, 193], [-27, 209], [-21, 217], [-6, 216], [14, 219], [44, 222], [72, 224], [84, 219], [78, 210], [56, 204], [30, 199], [10, 194]].map(([x, y]) => [X(x), y]);
    out.push(fill(smooth(body, { tension: 1 }), '$shoes'));
    out.push(fill(poly([[X(-25), 207], [X(-9), 207], [X(-7), sole], [X(-22), sole]]), '$shoes'));
    out.push(light(smooth([[X(50), 207], [X(72), 211], [X(80), 216], [X(56), 213]], { tension: 1 }), 0.25));
  } else {
    const high = kind === 'loafers' ? 0 : 6;
    const pts = [[-24, ay - 16 - high], [-26, sole - 10], [-20, sole], [60, sole], [84, sole - 4], [90, sole - 16], [70, sole - 28], [36, ay - 4], [10, ay - 12 - high]].map(([x, y]) => [X(x), y]);
    out.push(fill(smooth(pts, { tension: 0.95 }), '$shoes'));
    out.push(light(smooth([[X(40), sole - 24], [X(70), sole - 20], [X(82), sole - 14], [X(50), sole - 14]], { tension: 1 }), 0.2));
    out.push(fill(poly([[X(-25), sole - 7], [X(89), sole - 7], [X(89), sole], [X(-25), sole]]), kind === 'sneakers' ? '#f4f1ea' : '$shoes'));
    if (kind === 'sneakers') {
      out.push({ d: ribbon([[X(20), ay - 4], [X(30), ay + 4], [X(42), ay + 12]], 3), fill: '#ffffff', opacity: 0.9 });
      out.push({ d: ribbon([[X(10), ay - 8], [X(22), ay - 1], [X(34), ay + 7]], 3), fill: '#ffffff', opacity: 0.9 });
    }
    if (kind === 'oxfords') out.push(shade(ribbon([[X(26), ay - 2], [X(46), ay + 10], [X(60), ay + 18]], 3), 0.25));
  }
  if (far) out.push(...farShade(out));
  return out;
}
```

`assets/storymaker/lib/clothes.mjs`:

```js
// Garments: tops (details on the torso, sleeves on the arms), bottoms (pelvis and leg overlays), outfits.
// Tops are drawn over the body-specific torso base (coloured $top); their details are laid out for the average torso.

import { fill, shade, light, smooth, poly, circle, limb, roundRect4, ribbon, rad } from './util.mjs';
import { BODIES, YAW, LOWER_ARM, LOWER_LEG } from './dims.mjs';
import { centreX, torsoD, extremes } from './body.mjs';

const NECK = -255; // neck base in torso space
const cosP = (view) => Math.cos(rad(YAW[view]));
const sinP = (view) => Math.sin(rad(YAW[view]));
const AVG = 'average';
const SHIRT = '#f5f1ea'; // blouse and shirt fabric
const BAND = 0.2;

/** Map garment-front coordinates (X lateral, y) to torso space for a view. */
const mk = (view) => {
  const sx = cosP(view);
  return (X, y) => [centreX(AVG, view, y) + X * sx, y];
};

/** Skin opening of a neckline: kind round | v | square. depth in units below the neck base. */
function neckOpening(view, kind, depth, half) {
  const m = mk(view);
  if (view === 'profile') {
    const f = centreX(AVG, view, NECK + depth * 0.6);
    return [fill(smooth([[f - 9, NECK - 2], [f + 3, NECK - 2], [f + 5, NECK + depth * 0.5], [f - 4, NECK + depth * 0.6]], { tension: 1 }), '$skin')];
  }
  let pts;
  if (kind === 'v') pts = [m(-half, NECK - 2), m(half, NECK - 2), m(half * 0.12, NECK + depth, 'c'), m(-half * 0.12, NECK + depth, 'c')];
  else if (kind === 'square') pts = [m(-half, NECK - 2), m(half, NECK - 2), m(half, NECK + depth), m(-half, NECK + depth)];
  else pts = [m(-half, NECK - 2), m(half, NECK - 2), m(half * 0.8, NECK + depth * 0.7), m(0, NECK + depth), m(-half * 0.8, NECK + depth * 0.7)];
  return [fill(smooth(pts, { tension: 1 }), '$skin')];
}

function centreLine(view, y0, y1, w = 2.6, opacity = 0.22) {
  const m = mk(view);
  return { d: ribbon([m(0, y0), m(0, (y0 + y1) / 2), m(0, y1)], w), fill: '$outline', opacity };
}

function buttons(view, ys, r = 4.6, x = 0, color = '#ffffff', opacity = 0.9) {
  const m = mk(view);
  if (view === 'profile') return [];
  return ys.map((y) => fill(circle(...m(x, y), r * (view === 'front' ? 1 : 0.9)), color, opacity));
}

/** Hem extension below the base torso (base ends at y = 30). */
function hemExtension(view, y1, grow = 10, round = 12) {
  return fill(torsoD(AVG, view, y1, { grow, hemGrow: 2, hemRound: round }), '$top');
}

function sideCrescent(view, y0, y1, w = 14) {
  // right-flank second tone for a layer that extends beyond the base
  const m = mk(view);
  return shade(smooth([m(50, y0), m(60, (y0 + y1) / 2), m(48, y1), m(48 - w, y1), m(52 - w, (y0 + y1) / 2)], { tension: 1 }), 0.1);
}

// ---------------------------------------------------------------- tops
export const TOPS = ['tee', 'hoodie', 'blazer', 'suit-jacket', 'cardigan', 'shirt'];

export function topDetail(top, view) {
  const m = mk(view);
  const out = [];
  const side = view === 'profile';
  switch (top) {
    case 'tee': {
      out.push(...neckOpening(view, 'round', 30, 30));
      if (!side) out.push({ d: ribbon([m(-34, NECK + 2), m(-30, NECK + 16), m(-16, NECK + 28), m(0, NECK + 32), m(16, NECK + 28), m(30, NECK + 16), m(34, NECK + 2)], 7), fill: '$outline', opacity: 0.2 });
      out.push(hemExtension(view, 36, 4, 10));
      break;
    }
    case 'hoodie': {
      out.push(hemExtension(view, 58, 12, 14));
      out.push(...neckOpening(view, 'round', 22, 30));
      // hood roll around the neck
      const hood = side
        ? [m(-26, NECK - 6), m(-34, NECK + 18), m(-8, NECK + 34), m(16, NECK + 22), m(12, NECK - 6)]
        : [m(-50, NECK - 8), m(-48, NECK + 22), m(-24, NECK + 34), m(0, NECK + 38), m(24, NECK + 34), m(48, NECK + 22), m(50, NECK - 8), m(30, NECK - 2), m(0, NECK + 12), m(-30, NECK - 2)];
      out.push(fill(smooth(hood, { tension: 1 }), '$top'));
      out.push(shade(smooth(hood, { tension: 1 }), 0.16));
      if (!side) {
        out.push({ d: ribbon([m(-14, NECK + 26), m(-16, NECK + 60), m(-15, NECK + 82)], 4), fill: '#f3efe6', opacity: 0.95 });
        out.push({ d: ribbon([m(14, NECK + 26), m(16, NECK + 60), m(15, NECK + 82)], 4), fill: '#f3efe6', opacity: 0.95 });
        out.push(shade(roundRect4(...m(-52, -70), 104 * cosP(view), 62, [10, 10, 16, 16]), 0.12));
        out.push({ d: ribbon([m(-50, -66), m(-36, -72), m(36, -72), m(50, -66)], 3.2), fill: '$outline', opacity: 0.22 });
      }
      out.push(shade(ribbonBand(view, 46, 58), 0.16));
      break;
    }
    case 'blazer':
    case 'suit-jacket': {
      const suit = top === 'suit-jacket';
      out.push(hemExtension(view, 64, 14, 16));
      if (side) {
        out.push(...neckOpening(view, 'v', 40, 22));
        out.push(shade(ribbonBand(view, 56, 64), 0.1));
        out.push(fill(smooth([[centreX(AVG, view, -240) - 4, -246], [centreX(AVG, view, -200) + 4, -228], [centreX(AVG, view, -170), -210], [centreX(AVG, view, -196) - 8, -228]], { tension: 1 }), SHIRT));
        break;
      }
      const depth = suit ? 70 : 88;
      const half = suit ? 28 : 36;
      // shirt / blouse in the V
      out.push(fill(smooth([m(-half, NECK - 2), m(half, NECK - 2), m(half * 0.1, NECK + depth + 6, 'c'), m(-half * 0.1, NECK + depth + 6, 'c')], { tension: 1 }), SHIRT));
      out.push(...neckOpeningVSkin(view, half, suit ? 30 : 44));
      if (suit) {
        out.push(fill(smooth([m(-9, NECK + 14), m(9, NECK + 14), m(6, NECK + 30), m(13, NECK + 112), m(0, NECK + 128), m(-13, NECK + 112), m(-6, NECK + 30)], { tension: 0.9 }), '$accent'));
        out.push(shade(smooth([m(0, NECK + 30), m(13, NECK + 112), m(0, NECK + 128), m(-2, NECK + 100)], { tension: 1 }), 0.15));
        out.push(fill(poly([m(-12, NECK + 6), m(12, NECK + 6), m(8, NECK + 20), m(-8, NECK + 20)]), '$accent'));
      }
      // lapels
      const lapelL = [m(-half - 2, NECK - 2), m(-half - 22, NECK + 6), m(-half - 30, NECK + 56), m(-half * 0.1, NECK + depth + 4, 'c'), m(-half * 0.1 - 2, NECK + depth - 10), m(-half + 2, NECK + 28)];
      const lapelR = [m(half + 2, NECK - 2), m(half + 22, NECK + 6), m(half + 30, NECK + 56), m(half * 0.1, NECK + depth + 4, 'c'), m(half * 0.1 + 2, NECK + depth - 10), m(half - 2, NECK + 28)];
      out.push(fill(smooth(lapelL, { tension: 0.8 }), '$top'), shade(smooth(lapelL, { tension: 0.8 }), 0.18));
      out.push(fill(smooth(lapelR, { tension: 0.8 }), '$top'), shade(smooth(lapelR, { tension: 0.8 }), 0.1));
      out.push({ d: ribbon([m(0, NECK + depth + 4), m(0, -30), m(0, 20)], 3), fill: '$outline', opacity: 0.25 });
      out.push(...buttons(view, [NECK + depth + 26, NECK + depth + 66], 5, 0, suit ? '#1f2433' : '#fdf6ec', 0.9));
      // pocket flaps
      out.push({ d: ribbon([m(-80, -36), m(-52, -39)], 4), fill: '$outline', opacity: 0.22 });
      out.push({ d: ribbon([m(52, -39), m(80, -36)], 4), fill: '$outline', opacity: 0.22 });
      out.push(shade(ribbonBand(view, 52, 64), 0.1));
      break;
    }
    case 'cardigan': {
      out.push(hemExtension(view, 62, 12, 14));
      if (side) {
        out.push(...neckOpening(view, 'round', 26, 24));
        out.push(fill(poly([[centreX(AVG, view, -226) - 12, -226], [centreX(AVG, view, -226) - 3, -226], [centreX(AVG, view, 30) - 3, 30], [centreX(AVG, view, 30) - 12, 30]]), SHIRT));
        out.push(shade(ribbonBand(view, 50, 62), 0.18));
        break;
      }
      out.push(...neckOpening(view, 'round', 32, 32));
      out.push(fill(poly([m(-24, NECK + 6), m(24, NECK + 6), m(24, 60), m(-24, 60)]), SHIRT));
      out.push(...neckOpening(view, 'round', 30, 28));
      out.push({ d: ribbon([m(-34, NECK + 2), m(-30, NECK + 14), m(-16, NECK + 24), m(0, NECK + 28), m(16, NECK + 24), m(30, NECK + 14), m(34, NECK + 2)], 6), fill: '$outline', opacity: 0.16 });
      const bandL = poly([m(-36, NECK + 4), m(-24, NECK + 4), m(-24, 62), m(-36, 62)]);
      const bandR = poly([m(24, NECK + 4), m(36, NECK + 4), m(36, 62), m(24, 62)]);
      out.push(shade(bandL, 0.2), shade(bandR, 0.2));
      out.push({ d: ribbon([m(-24, NECK + 8), m(-24, 62)], 2.4), fill: '$outline', opacity: 0.25 });
      out.push({ d: ribbon([m(24, NECK + 8), m(24, 62)], 2.4), fill: '$outline', opacity: 0.25 });
      out.push(...buttons(view, [-170, -118, -66, -14], 5, 29, '#e9dcc4', 0.95));
      out.push(shade(roundRect4(...m(-76, -52), 40 * cosP(view), 40, [4, 4, 12, 12]), 0.12));
      out.push(shade(roundRect4(...m(36, -52), 40 * cosP(view), 40, [4, 4, 12, 12]), 0.12));
      out.push(shade(ribbonBand(view, 50, 62), 0.16));
      break;
    }
    case 'shirt': {
      out.push(hemExtension(view, 40, 6, 10));
      if (side) {
        out.push(...neckOpening(view, 'v', 22, 20));
        out.push(fill(smooth([[centreX(AVG, view, -250) - 8, -252], [centreX(AVG, view, -250) + 6, -250], [centreX(AVG, view, -226) + 6, -226], [centreX(AVG, view, -240) - 6, -236]], { tension: 1 }), '$top'));
        break;
      }
      out.push(...neckOpening(view, 'v', 34, 26));
      const collarL = [m(-30, NECK - 4), m(-6, NECK + 6), m(-3, NECK + 40), m(-34, NECK + 18)];
      const collarR = [m(30, NECK - 4), m(6, NECK + 6), m(3, NECK + 40), m(34, NECK + 18)];
      out.push(fill(poly(collarL), '$top'), light(poly(collarL), 0.35), shade(poly(collarL), 0.1));
      out.push(fill(poly(collarR), '$top'), light(poly(collarR), 0.3), shade(poly(collarR), 0.16));
      out.push({ d: ribbon([m(0, NECK + 38), m(0, -20), m(0, 30)], 2.6), fill: '$outline', opacity: 0.2 });
      out.push(...buttons(view, [NECK + 62, NECK + 112, NECK + 162, NECK + 212], 4.2, 0, '#ffffff', 0.85));
      out.push({ d: ribbon([m(34, -190), m(60, -192), m(60, -160), m(47, -152), m(34, -160), m(34, -190)], 2.6), fill: '$outline', opacity: 0.2 });
      break;
    }
    default: throw new Error(`unknown top ${top}`);
  }
  return out;
}

function neckOpeningVSkin(view, half, depth) {
  const m = mk(view);
  return [fill(smooth([m(-half * 0.64, NECK - 2), m(half * 0.64, NECK - 2), m(half * 0.08, NECK + depth, 'c'), m(-half * 0.08, NECK + depth, 'c')], { tension: 1 }), '$skin')];
}

function ribbonBand(view, y0, y1) {
  // a horizontal band across the hem region (a rib or hem line) as a polygon following the torso outline
  const m = mk(view);
  const hw = view === 'profile' ? 54 : 110;
  const pts = view === 'profile' ? [[-52, y0], [54, y0], [56, y1], [-54, y1]] : [m(-hw, y0), m(hw, y0), m(hw + 2, y1), m(-hw - 2, y1)];
  return poly(pts);
}

// ---------------------------------------------------------------- sleeves
/** Long sleeves: upper arm 0..150, lower arm 0..135. Short sleeves cover the upper arm only. */
export function sleeveUpper(top, far, view) {
  let out;
  if (top === 'tee') {
    out = [fill(limb(0, 82, 68, 66, 0), '$top')];
    out.push({ d: ribbon([[-30, 80], [0, 86], [30, 80]], 4), fill: '$outline', opacity: 0.18 });
  } else {
    const w0 = top === 'hoodie' ? 70 : 66;
    const w1 = top === 'hoodie' ? 64 : 58;
    out = [fill(limb(0, 154, w0, w1, 1.5), '$top')];
  }
  if (view !== 'front' && far) out.push(shade(out[0].d, 0.2));
  return out;
}

export function sleeveLower(top, far, view) {
  if (top === 'tee') return [];
  const hood = top === 'hoodie';
  const w0 = hood ? 62 : 56;
  const w1 = hood ? 58 : 50;
  const out = [fill(limb(0, LOWER_ARM - 2, w0, w1, 0.5), '$top')];
  if (hood || top === 'cardigan') out.push(shade(limb(LOWER_ARM - 18, LOWER_ARM - 2, w1 + 2, w1 + 2, 0), 0.18));
  if (top === 'blazer' || top === 'suit-jacket') out.push(fill(limb(LOWER_ARM - 8, LOWER_ARM + 1, w1 - 6, w1 - 8, 0), SHIRT));
  if (view !== 'front' && far) out.push(shade(out[0].d, 0.2));
  return out;
}

// ---------------------------------------------------------------- bottoms
export const BOTTOMS = ['trousers', 'jeans', 'skirt'];

export function pelvisDetail(bottom, view) {
  if (bottom !== 'skirt') return [];
  // waist band to the top of the thighs; the cones on the thigh bones carry the rest of the skirt
  const half = (y) => 84 + 16 * (y / 172);
  const depth = (y) => 58 + 8 * (y / 172);
  const at = (y) => extremes(view, half(y), depth(y), depth(y));
  const rows = [-30, 0, 30, 60, 84];
  const right = rows.map((y) => [at(y)[1], y]);
  const left = rows.map((y) => [at(y)[0], y]);
  const pts = [...right, [(right[4][0] + left[4][0]) / 2 + (right[4][0] - left[4][0]) * 0.25, 90], [(right[4][0] + left[4][0]) / 2 - (right[4][0] - left[4][0]) * 0.25, 90], ...left.reverse()];
  const d = smooth(pts, { tension: 0.9 });
  const shadeR = smooth([[at(-30)[1] * 0.45, -30], right[0], right[1], right[2], right[3], right[4], [right[4][0] * 0.4, 86]], { tension: 1 });
  return [fill(d, '$bottom'), shade(shadeR, 0.1)];
}

/** Knee-length A-line skirt cones on the thighs; trousers/jeans add seams and cuffs. */
export function legDetailUpper(bottom, kind, view, far) {
  if (bottom === 'skirt') {
    const flare = view === 'front' ? 1 : 1.04;
    const cone = smooth([[-38 * flare, -8], [38 * flare, -8], [60 * flare, 150], [52 * flare, 168], [0, 172], [-52 * flare, 168], [-60 * flare, 150]], { tension: 0.95 });
    const skinKnee = limb(138, BODIES[AVG].upperLeg, 74, 66, 0);
    const out = [fill(skinKnee, '$skin'), fill(cone, '$bottom'), shade(smooth([[10, -6], [38, -4], [60, 150], [52, 168], [18, 172]], { tension: 1 }), 0.12)];
    out.push({ d: ribbon([[-50, 163], [0, 168], [50, 163]], 3), fill: '$outline', opacity: 0.16 });
    if (view !== 'front' && far) out.push(shade(cone, 0.2));
    return out;
  }
  if (bottom === 'jeans') {
    return [{ d: ribbon([[34, 6], [36, 90], [31, 150]], 2.6), fill: '#ffffff', opacity: 0.2 }];
  }
  return [];
}

export function legDetailLower(bottom, kind, view, far) {
  if (bottom === 'skirt') {
    const out = [fill(limb(-2, LOWER_LEG - 12, 66, 52, 0), '$skin')];
    out.push(shade(limb(-2, LOWER_LEG - 12, 18, 12, 0, 14), 0.1));
    if (view !== 'front' && far) out.push(shade(out[0].d, 0.2));
    return out;
  }
  const out = [{ d: ribbon([[-19, LOWER_LEG - 24], [0, LOWER_LEG - 21], [19, LOWER_LEG - 24]], 5), fill: '$outline', opacity: 0.2 }];
  if (bottom === 'jeans') out.push({ d: ribbon([[-19, LOWER_LEG - 31], [19, LOWER_LEG - 31]], 4), fill: '#ffffff', opacity: 0.16 });
  return out;
}

// ---------------------------------------------------------------- outfits
export const SHOES = ['pumps', 'oxfords', 'sneakers', 'loafers'];

export const OUTFITS = {
  business: { top: 'blazer', bottom: 'skirt', shoes: 'pumps', colors: { top: '#2f5d8a', bottom: '#27324d', shoes: '#7a2d3a' } },
  suit: { top: 'suit-jacket', bottom: 'trousers', shoes: 'oxfords', colors: { top: '#3d4558', bottom: '#3d4558', shoes: '#2a211c' } },
  office: { top: 'shirt', bottom: 'trousers', shoes: 'oxfords', colors: { top: '#8fb8de', bottom: '#2f3a52', shoes: '#33261e' } },
  casual: { top: 'tee', bottom: 'jeans', shoes: 'sneakers', colors: { top: '#f2b544', bottom: '#3f5f8f', shoes: '#e9e6df' } },
  it: { top: 'hoodie', bottom: 'jeans', shoes: 'sneakers', colors: { top: '#5b5f9a', bottom: '#33425f', shoes: '#d9d6cf' } },
  cardigan: { top: 'cardigan', bottom: 'trousers', shoes: 'loafers', colors: { top: '#7b8f5a', bottom: '#575e6b', shoes: '#5a3e2b' } },
};
```

`assets/storymaker/lib/rig.mjs`:

```js
// Assembles rig/human.json: 13 bones, slots with per-view draw order, variants for every parameter value,
// body types and the parameter lists (contract section 3).

import { BODIES, BODY_NAMES, VIEWS, UNITS_HEIGHT, RIG_HEIGHT, LOWER_ARM, LOWER_LEG, ANKLE, WAIST, NECK_LEN, HEAD_PIVOT, hipHeight, cosYaw, sinYaw } from './dims.mjs';
import { headShapes, eyeShapes, browShapes, mouthShapes, EYE_STYLES, EYE_STATES, BROW_STYLES, BROW_STATES, MOUTH_NAMES } from './face.mjs';
import { torsoBase, neckShapes, pelvisBase, armSkin, legBase, handShapes, shoeShapes } from './body.mjs';
import { TOPS, BOTTOMS, SHOES, OUTFITS, topDetail, sleeveUpper, sleeveLower, pelvisDetail, legDetailUpper, legDetailLower } from './clothes.mjs';
import { HAIR_STYLES, hairShapes } from './hair.mjs';
import { GLASSES, FACIAL_HAIR, ACCESSORIES, glassesShapes, facialHairShapes, accessoryShapes } from './extras.mjs';

export const SKIN = {
  porcelain: '#fbe3d3', light: '#f6d2b6', tan: '#e9b88f', olive: '#d6a077',
  medium: '#c4875f', brown: '#9d6643', dark: '#76492f', deep: '#52321f',
};
export const HAIR_COLOR = {
  black: '#25211f', 'dark-brown': '#3f2c22', brown: '#6b4731', chestnut: '#8d5535',
  blonde: '#e4b963', red: '#b9532c', grey: '#aeb1b9', white: '#eceef2',
};
export const FIXED = { outline: '#2b2d42', eye_white: '#ffffff', iris: '#3b2f2f', lip: '#c9605f' };
export const HANDS = ['open', 'fist', 'point', 'hold'];

const rest = (x, y, angle = 0) => ({ x: round(x), y: round(y), angle });
const round = (v) => Math.round(v * 10) / 10;

/** Rest transforms of all bones for a body type and view. */
function restFor(bodyName, view) {
  const b = BODIES[bodyName];
  const cos = cosYaw(view);
  const sin = sinYaw(view);
  const lean = b.stoop * sin; // the elder's head and shoulders sit forward
  // front: _r on the screen right; angled views (facing right): _r is the near limb on the back side
  const armAngles = {
    front: { r: -5, l: 5 },
    three_quarter: { r: 4, l: -7 },
    profile: { r: 3, l: -4 },
  }[view];
  const sgn = view === 'front' ? 1 : -1; // x side of the _r limbs
  const elbow = view === 'front' ? 0 : -5;
  const legSpread = view === 'front' ? 1.5 : 0;
  const sx = b.shoulderX;
  const hx = b.hipX;
  const hips = hipHeight(b);
  return {
    root: rest(0, 0),
    hips: rest(0, -hips),
    torso: rest(0, -WAIST),
    neck: rest(lean * 0.55, -b.torsoLen),
    head: rest(lean * 0.45, -HEAD_PIVOT),
    upper_arm_l: rest(-sgn * sx * cos + lean * 0.2, -(b.torsoLen - 46), armAngles.l),
    lower_arm_l: rest(0, b.upperArm, elbow),
    upper_arm_r: rest(sgn * sx * cos + lean * 0.2, -(b.torsoLen - 46), armAngles.r),
    lower_arm_r: rest(0, b.upperArm, elbow),
    upper_leg_l: rest(-sgn * hx * cos, 0, legSpread),
    lower_leg_l: rest(0, b.upperLeg, 0),
    upper_leg_r: rest(sgn * hx * cos, 0, -legSpread),
    lower_leg_r: rest(0, b.upperLeg, 0),
  };
}

const BONE_DEFS = [
  ['root', null, 0],
  ['hips', 'root', WAIST],
  ['torso', 'hips', BODIES.average.torsoLen],
  ['neck', 'torso', NECK_LEN],
  ['head', 'neck', 250],
  ['upper_arm_l', 'torso', BODIES.average.upperArm],
  ['lower_arm_l', 'upper_arm_l', LOWER_ARM],
  ['upper_arm_r', 'torso', BODIES.average.upperArm],
  ['lower_arm_r', 'upper_arm_r', LOWER_ARM],
  ['upper_leg_l', 'hips', BODIES.average.upperLeg],
  ['lower_leg_l', 'upper_leg_l', LOWER_LEG],
  ['upper_leg_r', 'hips', BODIES.average.upperLeg],
  ['lower_leg_r', 'upper_leg_r', LOWER_LEG],
];

function buildBones() {
  return BONE_DEFS.map(([name, parent, length]) => {
    const r = {};
    for (const v of VIEWS) r[v] = restFor('average', v)[name];
    return { name, parent, length, rest: r };
  });
}

function buildBodies() {
  const out = {};
  for (const name of BODY_NAMES) {
    const body = BODIES[name];
    const entry = { scale: body.scale };
    const bones = {};
    for (const [bone, , baseLength] of BONE_DEFS) {
      const o = {};
      const length = lengthFor(name, bone);
      if (length !== baseLength) o.length = length;
      for (const v of VIEWS) {
        const base = restFor('average', v)[bone];
        const mine = restFor(name, v)[bone];
        const diff = {};
        if (mine.x !== base.x) diff.x = mine.x;
        if (mine.y !== base.y) diff.y = mine.y;
        if (Object.keys(diff).length) (o.rest ||= {})[v] = diff;
      }
      if (Object.keys(o).length) bones[bone] = o;
    }
    if (Object.keys(bones).length) entry.bones = bones;
    out[name] = entry;
  }
  return out;
}

function lengthFor(bodyName, bone) {
  const b = BODIES[bodyName];
  if (bone.startsWith('upper_arm')) return b.upperArm;
  if (bone.startsWith('upper_leg')) return b.upperLeg;
  if (bone === 'torso') return b.torsoLen;
  const base = BONE_DEFS.find((d) => d[0] === bone);
  return base[2];
}

// ---------------------------------------------------------------- slots
// z per view: [front, three_quarter, profile]. Far (left) limbs sit behind the torso in the angled views and the
// near (right) limbs in front of it. In the front view both arms sit above the torso.
const SLOT_DEFS = [
  ['hair_back', 'head', 'hair', [1, 1, 1]],
  ['arm_u_l', 'upper_arm_l', 'body', [60, 10, 10]],
  ['sleeve_u_l', 'upper_arm_l', 'top', [61, 11, 11]],
  ['arm_l_l', 'lower_arm_l', 'body', [62, 12, 12]],
  ['sleeve_l_l', 'lower_arm_l', 'top', [63, 13, 13]],
  ['hand_l', 'lower_arm_l', 'hand', [64, 14, 14]],
  ['leg_u_l', 'upper_leg_l', 'body', [20, 20, 20]],
  ['leg_det_u_l', 'upper_leg_l', 'bottom', [21, 21, 21]],
  ['leg_l_l', 'lower_leg_l', 'body', [22, 22, 22]],
  ['leg_det_l_l', 'lower_leg_l', 'bottom', [23, 23, 23]],
  ['foot_l', 'lower_leg_l', 'shoes', [24, 24, 24]],
  ['leg_u_r', 'upper_leg_r', 'body', [25, 26, 26]],
  ['leg_det_u_r', 'upper_leg_r', 'bottom', [26, 27, 27]],
  ['leg_l_r', 'lower_leg_r', 'body', [27, 28, 28]],
  ['leg_det_l_r', 'lower_leg_r', 'bottom', [28, 29, 29]],
  ['foot_r', 'lower_leg_r', 'shoes', [29, 30, 30]],
  ['pelvis', 'hips', 'body', [31, 31, 31]],
  ['pelvis_det', 'hips', 'bottom', [32, 32, 32]],
  ['torso', 'torso', 'body', [40, 40, 40]],
  ['neck', 'neck', 'body', [41, 41, 41]],
  ['top_det', 'torso', 'top', [42, 42, 42]],
  ['head', 'head', 'body', [50, 50, 50]],
  ['facial_hair', 'head', 'facial_hair', [51, 51, 51]],
  ['mouth', 'head', 'mouth', [52, 52, 52]],
  ['eyes', 'head', 'eyes', [53, 53, 53]],
  ['brows', 'head', 'brows', [54, 54, 54]],
  ['hair', 'head', 'hair', [56, 56, 56]],
  ['glasses', 'head', 'glasses', [58, 58, 58]],
  ['accessory', 'head', 'accessory', [59, 59, 59]],
  ['arm_u_r', 'upper_arm_r', 'body', [60, 70, 70]],
  ['sleeve_u_r', 'upper_arm_r', 'top', [61, 71, 71]],
  ['arm_l_r', 'lower_arm_r', 'body', [62, 72, 72]],
  ['sleeve_l_r', 'lower_arm_r', 'top', [63, 73, 73]],
  ['hand_r', 'lower_arm_r', 'hand', [64, 74, 74]],
];

function buildSlots() {
  return SLOT_DEFS.map(([name, bone, param, z]) => ({ name, bone, param, z: { front: z[0], three_quarter: z[1], profile: z[2] } }));
}

const perView = (fn) => {
  const o = {};
  for (const v of VIEWS) o[v] = fn(v);
  return o;
};

function buildVariants() {
  const V = {};
  const set = (slot, key, fn) => {
    (V[slot] ||= {})[key] = perView(fn);
  };
  // head group
  for (const body of BODY_NAMES) set('head', body, (v) => headShapes(body, v));
  for (const body of BODY_NAMES) set('neck', body, (v) => neckShapes(body, v));
  for (const style of Object.keys(EYE_STYLES)) for (const st of EYE_STATES) set('eyes', `${style}/${st}`, (v) => eyeShapes(style, st, v));
  for (const style of Object.keys(BROW_STYLES)) for (const st of BROW_STATES) set('brows', `${style}/${st}`, (v) => browShapes(style, st, v));
  for (const m of MOUTH_NAMES) set('mouth', m, (v) => mouthShapes(m, v));
  for (const h of HAIR_STYLES) {
    set('hair', h, (v) => hairShapes(h, v).front);
    set('hair_back', h, (v) => hairShapes(h, v).back);
  }
  for (const g of GLASSES) if (g !== 'none') set('glasses', g, (v) => glassesShapes(g, v));
  for (const f of FACIAL_HAIR) if (f !== 'none') set('facial_hair', f, (v) => facialHairShapes(f, v));
  for (const a of ACCESSORIES) if (a !== 'none') set('accessory', a, (v) => accessoryShapes(a, v));
  // body group
  for (const body of BODY_NAMES) {
    set('torso', body, (v) => torsoBase(body, v));
    set('pelvis', body, (v) => pelvisBase(body, v));
  }
  for (const side of ['l', 'r']) {
    const far = side === 'l';
    const sd = side === 'r' ? 1 : -1;
    for (const body of BODY_NAMES) {
      set(`arm_u_${side}`, body, (v) => armSkin(body, 'upper', v, far));
      set(`arm_l_${side}`, body, (v) => armSkin(body, 'lower', v, far));
      set(`leg_u_${side}`, body, (v) => legBase(body, 'upper', v, far));
      set(`leg_l_${side}`, body, (v) => legBase(body, 'lower', v, far));
    }
    for (const top of TOPS) {
      set(`sleeve_u_${side}`, top, (v) => sleeveUpper(top, far, v));
      set(`sleeve_l_${side}`, top, (v) => sleeveLower(top, far, v));
    }
    for (const bottom of BOTTOMS) {
      set(`leg_det_u_${side}`, bottom, (v) => legDetailUpper(bottom, 'upper', v, far));
      set(`leg_det_l_${side}`, bottom, (v) => legDetailLower(bottom, 'lower', v, far));
    }
    for (const shoe of SHOES) set(`foot_${side}`, shoe, (v) => shoeShapes(shoe, v, sd, far));
    for (const h of HANDS) set(`hand_${side}`, h, (v) => handShapes(h, v === 'front' ? (side === 'r' ? -1 : 1) : 1, v));
  }
  for (const top of TOPS) set('top_det', top, (v) => topDetail(top, v));
  for (const bottom of BOTTOMS) set('pelvis_det', bottom, (v) => pelvisDetail(bottom, v));
  return V;
}

export function buildRig() {
  return {
    id: 'rig.human/1',
    units_height: UNITS_HEIGHT,
    height: RIG_HEIGHT,
    seat_drop: hipHeight(BODIES.average) - (LOWER_LEG + ANKLE),
    bones: buildBones(),
    slots: buildSlots(),
    variants: buildVariants(),
    bodies: buildBodies(),
    params: {
      body: BODY_NAMES,
      skin: SKIN,
      hair: HAIR_STYLES,
      hair_color: HAIR_COLOR,
      eyes: Object.keys(EYE_STYLES),
      brows: Object.keys(BROW_STYLES),
      facial_hair: FACIAL_HAIR,
      glasses: GLASSES,
      outfit: OUTFITS,
      accessory: ACCESSORIES,
      fixed: FIXED,
    },
  };
}
```


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node --test assets/storymaker/test/rig.test.mjs`
Expected: PASS (5 tests)

- [ ] **Step 5: Commit**

```bash
git add assets/storymaker/lib/body.mjs assets/storymaker/lib/clothes.mjs assets/storymaker/lib/rig.mjs assets/storymaker/test/rig.test.mjs
git commit -m "feat(storymaker): human rig with bodies, outfits and variants" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```


### Task D4: Poses, gestures and motion cycles

**Files:**
- Create: `assets/storymaker/lib/motion.mjs`
- Test: `assets/storymaker/test/motion.test.mjs`

Ten poses, nine gestures as additive key poses that return to the base pose, and the walk and idle cycles. Seated poses carry `offset_y` (the root drop, 215 rig units) so the feet stay on the floor line; the walk speed (16 percent of the design width per second) follows from the stride formula in the comment.

- [ ] **Step 1: Write the failing test**

`assets/storymaker/test/motion.test.mjs`:

```js
// Task D4: poses, gestures and walk/idle cycles.
// Run: node --test assets/storymaker/test/motion.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { POSES, GESTURES, CYCLES, SEAT_DROP } from '../lib/motion.mjs';
import { evalKeys } from '../lib/svg.mjs';

const BONES = ['root', 'hips', 'torso', 'neck', 'head', 'upper_arm_l', 'lower_arm_l', 'upper_arm_r', 'lower_arm_r', 'upper_leg_l', 'lower_leg_l', 'upper_leg_r', 'lower_leg_r'];
const HANDS = ['open', 'fist', 'point', 'hold'];

test('the ten poses and nine gestures exist and only use known bones and hands', () => {
  assert.deepEqual(Object.keys(POSES), ['stand', 'sit', 'typing', 'point-left', 'point-right', 'wave', 'think', 'present', 'arms-crossed', 'hands-on-hips']);
  assert.deepEqual(Object.keys(GESTURES), ['point', 'wave', 'shrug', 'think', 'present', 'thumbs-up', 'celebrate', 'facepalm', 'count']);
  const all = [...Object.values(POSES), ...Object.values(GESTURES).flatMap((g) => g.keys), ...Object.values(CYCLES).flatMap((c) => c.keys)];
  for (const item of all) {
    for (const [b, deg] of Object.entries(item.bones)) assert.ok(BONES.includes(b) && Number.isFinite(deg), b);
    for (const h of Object.values(item.hands || {})) assert.ok(HANDS.includes(h), h);
  }
});

test('seated poses drop the root so that the feet stay on the floor line', () => {
  assert.equal(SEAT_DROP, 215);
  for (const name of ['sit', 'typing']) {
    assert.equal(POSES[name].seated, true);
    assert.equal(POSES[name].offset_y, SEAT_DROP);
    assert.equal(POSES[name].bones.upper_leg_r, -90); // thighs forward
    assert.equal(POSES[name].bones.lower_leg_r, 90); // shins vertical
  }
  assert.equal(POSES.stand.offset_y, undefined);
});

test('every gesture starts and ends on the base pose and has a positive duration', () => {
  for (const [name, g] of Object.entries(GESTURES)) {
    assert.equal(g.keys[0].t, 0, name);
    assert.equal(g.keys.at(-1).t, 1, name);
    assert.deepEqual(g.keys[0].bones, g.keys.at(-1).bones, name);
    assert.ok(g.duration > 0, name);
    g.keys.forEach((k, i) => i && assert.ok(k.t >= g.keys[i - 1].t, name));
  }
  assert.deepEqual(GESTURES.point.keys[1].bones, { upper_arm_r: -85, lower_arm_r: -10 });
});

test('the walk cycle is periodic, mirrored over half a period and drops the root while the legs are spread', () => {
  const walk = CYCLES.walk;
  assert.equal(walk.period, 1);
  const a = evalKeys(walk.keys, 0);
  const b = evalKeys(walk.keys, 0.5);
  const c = evalKeys(walk.keys, 1);
  assert.deepEqual(a.bones, c.bones);
  assert.equal(a.bones.upper_leg_r, -b.bones.upper_leg_r);
  assert.equal(a.bones.upper_leg_l, -b.bones.upper_leg_l);
  assert.equal(a.bones.upper_arm_r, -b.bones.upper_arm_r);
  assert.ok(a.offset_y > evalKeys(walk.keys, 0.25).offset_y, 'lowest at contact, highest while passing');
  // stride: 420 units of leg at 18 degrees; speed in percent of the 1920 px width at 0.55 * 1080 / 1000 px per unit
  const step = 2 * 420 * Math.sin((18 * Math.PI) / 180);
  assert.equal(Math.round(((2 * step * 0.55 * 1.08) / 1920) * 100), walk.speed);
  assert.equal(CYCLES.idle.speed, undefined);
  assert.equal(CYCLES.idle.period, 3);
});
```


- [ ] **Step 2: Run the test to verify it fails**

Run: `node --test assets/storymaker/test/motion.test.mjs`
Expected: FAIL (`ERR_MODULE_NOT_FOUND`: cannot find `assets/storymaker/lib/motion.mjs`)

- [ ] **Step 3: Implement**

`assets/storymaker/lib/motion.mjs`:

```js
// Poses, gestures and motion cycles for the human rig.
//
// Angles are degrees added to the bone's rest angle (poses) or on top of the pose (gestures, cycles).
// Negative swings a limb towards +x, which is forward for a right-facing character. "_r" is the near limb in the
// three_quarter and profile views and sits on the screen right in the front view.
// Gesture and cycle keys are additive deltas, linearly interpolated; every gesture ends at its first key (t = 1).

import { BODIES, hipHeight, LOWER_LEG, ANKLE } from './dims.mjs';

/** Root drop (rig units) of a seated pose: hips come down so that the feet stay on the floor line. */
export const SEAT_DROP = hipHeight(BODIES.average) - (LOWER_LEG + ANKLE);

const SIT_LEGS = { upper_leg_l: -90, upper_leg_r: -90, lower_leg_l: 90, lower_leg_r: 90 };

export const POSES = {
  stand: { bones: {}, hands: { l: 'open', r: 'open' } },
  sit: {
    bones: { ...SIT_LEGS, upper_arm_r: -14, lower_arm_r: -66, upper_arm_l: -14, lower_arm_l: -66 },
    hands: { l: 'open', r: 'open' }, view: 'three_quarter', seated: true, offset_y: SEAT_DROP,
  },
  typing: {
    bones: { ...SIT_LEGS, upper_arm_l: -30, lower_arm_l: -70, upper_arm_r: -30, lower_arm_r: -70, head: 4 },
    hands: { l: 'open', r: 'open' }, view: 'three_quarter', seated: true, offset_y: SEAT_DROP,
  },
  'point-left': { bones: { upper_arm_l: 88, lower_arm_l: 4, head: -3 }, hands: { l: 'point' }, view: 'front' },
  'point-right': { bones: { upper_arm_r: -88, lower_arm_r: -4, head: 3 }, hands: { r: 'point' }, view: 'three_quarter' },
  wave: { bones: { upper_arm_r: -128, lower_arm_r: -22, head: 4 }, hands: { r: 'open' }, view: 'three_quarter' },
  think: { bones: { upper_arm_r: -62, lower_arm_r: -114, upper_arm_l: 8, lower_arm_l: 0, head: -5 }, hands: { r: 'fist' }, view: 'three_quarter' },
  present: { bones: { upper_arm_r: -70, lower_arm_r: -14, torso: -2 }, hands: { r: 'open' }, view: 'three_quarter' },
  'arms-crossed': { bones: { upper_arm_r: -4, lower_arm_r: 102, upper_arm_l: 8, lower_arm_l: -98 }, hands: { l: 'fist', r: 'fist' }, view: 'front' },
  'hands-on-hips': { bones: { upper_arm_r: -38, lower_arm_r: 70, upper_arm_l: 38, lower_arm_l: -70 }, hands: { l: 'fist', r: 'fist' }, view: 'front' },
};

export const GESTURES = {
  point: {
    duration: 1.2,
    keys: [{ t: 0, bones: {} }, { t: 0.3, bones: { upper_arm_r: -85, lower_arm_r: -10 }, hands: { r: 'point' } }, { t: 0.8, bones: { upper_arm_r: -85, lower_arm_r: -10 }, hands: { r: 'point' } }, { t: 1, bones: {} }],
  },
  wave: {
    duration: 1.6,
    keys: [
      { t: 0, bones: {} },
      { t: 0.18, bones: { upper_arm_r: -126, lower_arm_r: -14 }, hands: { r: 'open' } },
      { t: 0.32, bones: { upper_arm_r: -126, lower_arm_r: 12 } },
      { t: 0.46, bones: { upper_arm_r: -126, lower_arm_r: -40 } },
      { t: 0.6, bones: { upper_arm_r: -126, lower_arm_r: 12 } },
      { t: 0.74, bones: { upper_arm_r: -126, lower_arm_r: -40 } },
      { t: 0.84, bones: { upper_arm_r: -126, lower_arm_r: -14 } },
      { t: 1, bones: {} },
    ],
  },
  shrug: {
    duration: 1.4,
    keys: [
      { t: 0, bones: {} },
      { t: 0.3, bones: { upper_arm_r: -30, lower_arm_r: -66, upper_arm_l: -18, lower_arm_l: -54, head: 6, torso: -1 }, hands: { l: 'open', r: 'open' } },
      { t: 0.75, bones: { upper_arm_r: -30, lower_arm_r: -66, upper_arm_l: -18, lower_arm_l: -54, head: 6, torso: -1 }, hands: { l: 'open', r: 'open' } },
      { t: 1, bones: {} },
    ],
  },
  think: {
    duration: 2,
    keys: [
      { t: 0, bones: {} },
      { t: 0.3, bones: { upper_arm_r: -62, lower_arm_r: -114, head: -6 }, hands: { r: 'fist' } },
      { t: 0.8, bones: { upper_arm_r: -62, lower_arm_r: -114, head: -6 }, hands: { r: 'fist' } },
      { t: 1, bones: {} },
    ],
  },
  present: {
    duration: 1.4,
    keys: [
      { t: 0, bones: {} },
      { t: 0.3, bones: { upper_arm_r: -66, lower_arm_r: -16 }, hands: { r: 'open' } },
      { t: 0.8, bones: { upper_arm_r: -66, lower_arm_r: -16 }, hands: { r: 'open' } },
      { t: 1, bones: {} },
    ],
  },
  'thumbs-up': {
    duration: 1.4,
    keys: [
      { t: 0, bones: {} },
      { t: 0.3, bones: { upper_arm_r: -52, lower_arm_r: -78, head: 3 }, hands: { r: 'fist' } },
      { t: 0.8, bones: { upper_arm_r: -52, lower_arm_r: -78, head: 3 }, hands: { r: 'fist' } },
      { t: 1, bones: {} },
    ],
  },
  celebrate: {
    duration: 1.6,
    keys: [
      { t: 0, bones: {} },
      { t: 0.25, bones: { upper_arm_r: -158, lower_arm_r: -8, upper_arm_l: 158, lower_arm_l: 8, head: -5 }, hands: { l: 'open', r: 'open' } },
      { t: 0.45, bones: { upper_arm_r: -150, lower_arm_r: -14, upper_arm_l: 150, lower_arm_l: 14, head: -5 } },
      { t: 0.65, bones: { upper_arm_r: -158, lower_arm_r: -8, upper_arm_l: 158, lower_arm_l: 8, head: -5 } },
      { t: 0.85, bones: { upper_arm_r: -150, lower_arm_r: -14, upper_arm_l: 150, lower_arm_l: 14, head: -5 } },
      { t: 1, bones: {} },
    ],
  },
  facepalm: {
    duration: 1.6,
    keys: [
      { t: 0, bones: {} },
      { t: 0.3, bones: { upper_arm_r: -58, lower_arm_r: -122, head: 10, torso: 3 }, hands: { r: 'open' } },
      { t: 0.8, bones: { upper_arm_r: -58, lower_arm_r: -122, head: 10, torso: 3 }, hands: { r: 'open' } },
      { t: 1, bones: {} },
    ],
  },
  count: {
    duration: 2,
    keys: [
      { t: 0, bones: {} },
      { t: 0.2, bones: { upper_arm_r: -48, lower_arm_r: -62 }, hands: { r: 'point' } },
      { t: 0.35, bones: { upper_arm_r: -48, lower_arm_r: -52 } },
      { t: 0.5, bones: { upper_arm_r: -48, lower_arm_r: -66 } },
      { t: 0.65, bones: { upper_arm_r: -48, lower_arm_r: -52 } },
      { t: 0.8, bones: { upper_arm_r: -48, lower_arm_r: -62 } },
      { t: 1, bones: {} },
    ],
  },
};

// Walk: in the profile view. Legs swing +-18 degrees, so a step is 2 * 420 * sin(18) = 260 rig units and one cycle of
// two steps is 520. The root drops while the legs are spread (420 * (1 - cos 18) = 20.6), which keeps the feet on the floor.
// speed = percent of the design width per second at depth scale 1, for a 1 s period:
// 520 rig units * (0.55 * 1080 / 1000) px per unit / 1920 px * 100 = 16.
export const CYCLES = {
  walk: {
    period: 1,
    speed: 16,
    keys: [
      { t: 0, bones: { upper_leg_r: -18, lower_leg_r: 3, upper_leg_l: 18, lower_leg_l: 14, upper_arm_r: 22, lower_arm_r: -8, upper_arm_l: -22, lower_arm_l: -30, torso: 1 }, offset_y: 21 },
      { t: 0.25, bones: { upper_leg_r: 0, lower_leg_r: 4, upper_leg_l: -24, lower_leg_l: 56, upper_arm_r: 2, lower_arm_r: -14, upper_arm_l: -2, lower_arm_l: -20, torso: 0 }, offset_y: 0 },
      { t: 0.5, bones: { upper_leg_r: 18, lower_leg_r: 14, upper_leg_l: -18, lower_leg_l: 3, upper_arm_r: -22, lower_arm_r: -30, upper_arm_l: 22, lower_arm_l: -8, torso: 1 }, offset_y: 21 },
      { t: 0.75, bones: { upper_leg_r: -24, lower_leg_r: 56, upper_leg_l: 0, lower_leg_l: 4, upper_arm_r: -2, lower_arm_r: -20, upper_arm_l: 2, lower_arm_l: -14, torso: 0 }, offset_y: 0 },
      { t: 1, bones: { upper_leg_r: -18, lower_leg_r: 3, upper_leg_l: 18, lower_leg_l: 14, upper_arm_r: 22, lower_arm_r: -8, upper_arm_l: -22, lower_arm_l: -30, torso: 1 }, offset_y: 21 },
    ],
  },
  idle: {
    period: 3,
    keys: [
      { t: 0, bones: { torso: 0, head: 0, upper_arm_r: 0, upper_arm_l: 0 } },
      { t: 0.5, bones: { torso: 1.2, head: -0.8, upper_arm_r: 1.5, upper_arm_l: -1.5 }, offset_y: -2 },
      { t: 1, bones: { torso: 0, head: 0, upper_arm_r: 0, upper_arm_l: 0 } },
    ],
  },
};

```


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node --test assets/storymaker/test/motion.test.mjs`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add assets/storymaker/lib/motion.mjs assets/storymaker/test/motion.test.mjs
git commit -m "feat(storymaker): poses, gestures, walk and idle cycles" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```


### Task D5: The 23 starter props

**Files:**
- Create: `assets/storymaker/lib/props.mjs`
- Test: `assets/storymaker/test/props.test.mjs`

Flat props with a second tone, coloured from palette tokens (`$accent`, `$accent2`, `$fg`, `$bg2`) and a few fixed neutrals so they read on both palettes. Each stands on the bottom centre of its viewbox and carries its size as a fraction of the stage height.

- [ ] **Step 1: Write the failing test**

`assets/storymaker/test/props.test.mjs`:

```js
// Task D5: the 23 starter props.
// Run: node --test assets/storymaker/test/props.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePath } from '../lib/pathdata.mjs';
import { PROPS } from '../lib/props.mjs';

const PALETTE_TOKENS = ['bg', 'bg2', 'fg', 'accent', 'accent2', 'text', 'outline'];
const IDS = ['laptop', 'phone', 'monitor', 'server', 'router', 'cloud', 'database', 'lock', 'key', 'shield', 'mail', 'document', 'coffee-cup', 'desk', 'chair', 'plant', 'check', 'cross', 'warning', 'idea', 'question', 'arrow-right', 'gear'];

test('all 23 props exist with sizes, tags and a hand hold for the cup', () => {
  assert.deepEqual(Object.keys(PROPS).sort(), [...IDS].sort());
  for (const [name, p] of Object.entries(PROPS)) {
    assert.ok(p.h > 0 && p.h < 0.4, name);
    assert.ok(p.tags.length >= 3, name);
  }
  assert.equal(PROPS['coffee-cup'].hold, true);
  assert.equal(PROPS.lock.h, 0.14);
  assert.equal(PROPS.shield.h, 0.16);
});

test('each prop is valid path data in its viewbox and uses only palette tokens and hex colours', () => {
  for (const [name, p] of Object.entries(PROPS)) {
    const { viewbox, shapes } = p.build();
    assert.equal(viewbox.length, 4, name);
    assert.ok(viewbox[2] > 0 && viewbox[3] > 0, name);
    assert.ok(shapes.length >= 3, `${name} has detail`);
    for (const [i, s] of shapes.entries()) {
      assert.ok(parsePath(s.d).ok, `${name}[${i}]`);
      for (const c of [s.fill, s.stroke, ...(s.gradient ? s.gradient.stops.map((st) => st[1]) : [])].filter(Boolean)) {
        assert.ok(c[0] === '$' ? PALETTE_TOKENS.includes(c.slice(1)) : /^#[0-9a-f]{6}$/i.test(c), `${name}[${i}] ${c}`);
      }
    }
  }
});

test('props are deterministic', () => {
  for (const p of Object.values(PROPS)) assert.equal(JSON.stringify(p.build()), JSON.stringify(p.build()));
});
```


- [ ] **Step 2: Run the test to verify it fails**

Run: `node --test assets/storymaker/test/props.test.mjs`
Expected: FAIL (`ERR_MODULE_NOT_FOUND`: cannot find `assets/storymaker/lib/props.mjs`)

- [ ] **Step 3: Implement**

`assets/storymaker/lib/props.mjs`:

```js
// The 23 starter props. Flat shapes with a second tone, coloured from palette tokens ($accent, $accent2, $fg, $bg2)
// and a few fixed neutrals so they read on both starter palettes. Every prop stands on the bottom centre of its
// viewbox (anchor 0.5, 1) and is sized by its height as a fraction of the stage height.

import {
  fill, shade, light, line, linear, ellipse, circle, rect, roundRect, roundRect4, poly, ribbon, M, L, C, Z, A,
  rad,
} from './util.mjs';

const WHITE = '#ffffff';
const PAPER = '#f7f8fb';
const STEEL = '#c9d0dc';
const STEEL_DARK = '#8f99ae';
const INK = '#2b3350';
const GREEN = '#34c77b';
const RED = '#ef5b5b';
const YELLOW = '#f7b731';

const groundShadow = (cx, y, rx) => shade(ellipse(cx, y, rx, rx * 0.12), 0.12);

function laptop() {
  const s = [];
  s.push(groundShadow(120, 166, 100));
  s.push(fill(roundRect(36, 8, 168, 112, 9), '$fg'));
  s.push(fill(roundRect(44, 16, 152, 96, 4), '#eef3fb'));
  s.push(linear(roundRect(44, 16, 152, 96, 4), [[0, '$accent2'], [1, '$accent']], 44, 16, 196, 112, 0.22));
  // chart on the screen
  for (const [i, h] of [30, 52, 38, 66].entries()) s.push(fill(roundRect(62 + i * 28, 100 - h, 18, h, 3), i % 2 ? '$accent' : '$accent2'));
  s.push(fill(roundRect(62, 28, 52, 7, 3), '$fg', 0.55));
  s.push(fill(roundRect(62, 40, 34, 7, 3), '$fg', 0.3));
  s.push(fill(poly([[16, 124], [224, 124], [236, 152], [4, 152]]), '#d7dce6'));
  s.push(fill(roundRect4(4, 148, 232, 12, [0, 0, 8, 8]), STEEL_DARK));
  s.push(fill(roundRect(98, 128, 44, 9, 4), STEEL, 0.9));
  s.push(shade(poly([[120, 124], [224, 124], [236, 152], [120, 152]]), 0.06));
  s.push(light(roundRect(48, 18, 70, 6, 3), 0.35));
  return { viewbox: [0, 0, 240, 170], shapes: s };
}

function phone() {
  const s = [];
  s.push(groundShadow(50, 196, 38));
  s.push(fill(roundRect(8, 4, 84, 188, 16), '$fg'));
  s.push(fill(roundRect(14, 14, 72, 168, 9), '#f4f7fd'));
  s.push(fill(roundRect(36, 18, 28, 6, 3), '$fg'));
  s.push(fill(roundRect(22, 36, 40, 22, 9), '$accent2'));
  s.push(fill(roundRect(38, 66, 40, 22, 9), '$accent'));
  s.push(fill(roundRect(22, 96, 52, 22, 9), '$accent2', 0.8));
  for (let i = 0; i < 3; i++) s.push(fill(roundRect(22 + i * 20, 140, 16, 16, 5), i === 1 ? '$accent' : '$fg', i === 1 ? 1 : 0.25));
  s.push(fill(roundRect(36, 168, 28, 4, 2), '$fg', 0.35));
  s.push(light(poly([[14, 14], [60, 14], [14, 70]]), 0.22));
  return { viewbox: [0, 0, 100, 200], shapes: s };
}

function monitor() {
  const s = [];
  s.push(groundShadow(120, 196, 90));
  s.push(fill(roundRect(76, 176, 88, 14, 6), STEEL_DARK));
  s.push(fill(poly([[104, 144], [136, 144], [142, 178], [98, 178]]), STEEL));
  s.push(fill(roundRect(8, 6, 224, 142, 10), '$fg'));
  s.push(fill(roundRect(16, 14, 208, 124, 5), '#eef3fb'));
  s.push(linear(roundRect(16, 14, 208, 124, 5), [[0, '$accent2'], [1, '$accent']], 16, 14, 224, 138, 0.2));
  s.push(fill(roundRect(30, 28, 70, 8, 4), '$fg', 0.45));
  s.push(fill(roundRect(30, 44, 46, 8, 4), '$fg', 0.25));
  for (const [i, h] of [28, 50, 36, 70, 58].entries()) s.push(fill(roundRect(104 + i * 22, 126 - h, 14, h, 3), i % 2 ? '$accent' : '$accent2'));
  s.push(line(poly([[30, 112], [52, 92], [72, 100], [92, 70]], false), '$accent', 5));
  s.push(light(poly([[16, 14], [110, 14], [16, 96]]), 0.18));
  return { viewbox: [0, 0, 240, 200], shapes: s };
}

function server() {
  const s = [];
  s.push(groundShadow(70, 216, 58));
  s.push(fill(roundRect(14, 6, 112, 206, 8), '#3a4261'));
  for (let i = 0; i < 4; i++) {
    const y = 16 + i * 48;
    s.push(fill(roundRect(22, y, 96, 40, 5), '#4a5478'));
    s.push(fill(roundRect(30, y + 8, 56, 6, 3), '#2b3350'));
    s.push(fill(roundRect(30, y + 20, 56, 6, 3), '#2b3350'));
    s.push(fill(circle(100, y + 14, 5), i % 2 ? '$accent2' : GREEN));
    s.push(fill(circle(100, y + 28, 5), i === 2 ? '$accent' : '$accent2', 0.8));
    s.push(light(rect(22, y, 96, 4), 0.1));
  }
  s.push(shade(roundRect4(84, 6, 42, 206, [0, 8, 8, 0]), 0.14));
  return { viewbox: [0, 0, 140, 220], shapes: s };
}

function router() {
  const s = [];
  s.push(groundShadow(110, 134, 90));
  s.push(fill(roundRect(24, 14, 10, 62, 5), '$fg'));
  s.push(fill(roundRect(186, 14, 10, 62, 5), '$fg'));
  s.push(fill(roundRect(8, 70, 204, 56, 14), '$accent'));
  s.push(shade(roundRect4(8, 100, 204, 26, [0, 0, 14, 14]), 0.16));
  s.push(light(roundRect(18, 76, 184, 8, 4), 0.25));
  for (let i = 0; i < 4; i++) s.push(fill(circle(48 + i * 24, 98, 5.5), i === 3 ? '$accent2' : WHITE, i === 3 ? 1 : 0.92));
  s.push(fill(roundRect(150, 90, 40, 8, 4), WHITE, 0.55));
  // signal arcs
  for (const [i, r] of [14, 26, 38].entries()) s.push(line(`${M(110 - r, 58 - r * 0.2)}${A(r, r, 0, 0, 1, 110 + r, 58 - r * 0.2)}`, '$accent2', 5, 0.95 - i * 0.2));
  return { viewbox: [0, 0, 220, 140], shapes: s };
}

function cloud() {
  const s = [];
  const body = (dy = 0, color = '$accent2', opacity) => [
    fill(circle(76, 98 + dy, 40), color, opacity), fill(circle(124, 70 + dy, 54), color, opacity), fill(circle(176, 100 + dy, 42), color, opacity),
    fill(roundRect(44, 100 + dy, 170, 40, 20), color, opacity),
  ];
  s.push(...body());
  s.push(shade(roundRect4(44, 120, 170, 20, [0, 0, 20, 20]), 0.1));
  s.push(light(`${M(88, 60)}${A(30, 30, 0, 0, 1, 130, 28)}${A(36, 36, 0, 0, 0, 96, 60)}${Z}`, 0.35));
  return { viewbox: [0, 0, 240, 160], shapes: s };
}

function database() {
  const s = [];
  s.push(groundShadow(80, 196, 66));
  const disc = (y, color) => {
    s.push(fill(`${M(14, y)}${L(14, y + 42)}${A(66, 20, 0, 0, 0, 146, y + 42)}${L(146, y)}${Z}`, color));
    s.push(shade(`${M(90, y)}${L(146, y)}${L(146, y + 42)}${A(66, 20, 0, 0, 1, 90, y + 62)}${Z}`, 0.12));
    s.push(fill(ellipse(80, y, 66, 20), color));
    s.push(light(ellipse(80, y, 66, 20), 0.2));
    s.push(fill(circle(124, y + 28, 4.5), WHITE, 0.9));
  };
  disc(116, '$accent');
  disc(66, '$accent2');
  disc(16, '$accent');
  return { viewbox: [0, 0, 160, 200], shapes: s };
}

function lock() {
  const s = [];
  s.push(groundShadow(60, 146, 44));
  s.push(fill(`${M(34, 66)}${L(34, 44)}${A(26, 28, 0, 0, 1, 86, 44)}${L(86, 66)}${L(72, 66)}${L(72, 44)}${A(12, 14, 0, 0, 0, 48, 44)}${L(48, 66)}${Z}`, '#9aa4b8'));
  s.push(fill(roundRect(14, 64, 92, 80, 14), '$accent'));
  s.push(shade(roundRect4(60, 64, 46, 80, [0, 14, 14, 0]), 0.14));
  s.push(light(roundRect(22, 70, 76, 8, 4), 0.3));
  s.push(fill(circle(60, 96, 11), '$fg'));
  s.push(fill(poly([[55, 100], [65, 100], [68, 124], [52, 124]]), '$fg'));
  return { viewbox: [0, 0, 120, 150], shapes: s };
}

function key() {
  const s = [];
  s.push(fill(circle(52, 52, 38), '$accent'));
  s.push(fill(circle(52, 52, 16), '$bg'));
  s.push(shade(`${M(52, 14)}${A(38, 38, 0, 0, 1, 52, 90)}${L(52, 52)}${Z}`, 0.12));
  s.push(fill(roundRect(84, 44, 118, 16, 8), '$accent'));
  s.push(fill(roundRect(152, 58, 14, 28, 5), '$accent'));
  s.push(fill(roundRect(176, 58, 14, 20, 5), '$accent'));
  s.push(shade(roundRect(84, 52, 118, 8, 4), 0.14));
  s.push(light(`${M(30, 36)}${A(26, 26, 0, 0, 1, 56, 18)}${L(56, 24)}${A(20, 20, 0, 0, 0, 36, 40)}${Z}`, 0.3));
  return { viewbox: [0, 0, 210, 104], shapes: s };
}

function shield() {
  const s = [];
  const body = `${M(80, 8)}${C(100, 24, 130, 28, 148, 28)}${L(148, 84)}${C(148, 128, 116, 160, 80, 178)}${C(44, 160, 12, 128, 12, 84)}${L(12, 28)}${C(30, 28, 60, 24, 80, 8)}${Z}`;
  s.push(fill(body, '$accent2'));
  s.push(shade(`${M(80, 8)}${C(100, 24, 130, 28, 148, 28)}${L(148, 84)}${C(148, 128, 116, 160, 80, 178)}${Z}`, 0.14));
  s.push(fill(`${M(80, 24)}${C(96, 36, 118, 40, 134, 41)}${L(134, 84)}${C(134, 120, 108, 148, 80, 164)}${C(52, 148, 26, 120, 26, 84)}${L(26, 41)}${C(42, 40, 64, 36, 80, 24)}${Z}`, WHITE, 0.2));
  s.push({ d: ribbon([[50, 88], [72, 112], [112, 66]], 16), fill: WHITE });
  return { viewbox: [0, 0, 160, 190], shapes: s };
}

function mail() {
  const s = [];
  s.push(groundShadow(100, 146, 80));
  s.push(fill(roundRect(8, 24, 184, 118, 12), '#f2f4fa'));
  s.push(shade(roundRect4(8, 84, 184, 58, [0, 0, 12, 12]), 0.07));
  s.push(fill(poly([[8, 38], [100, 100], [192, 38], [192, 142], [8, 142]]).replace(/Z$/, 'Z'), '#e3e8f3'));
  s.push(fill(`${M(8, 36)}${C(8, 28, 14, 24, 22, 24)}${L(178, 24)}${C(186, 24, 192, 28, 192, 36)}${L(100, 92)}${Z}`, '$accent'));
  s.push(shade(`${M(100, 92)}${L(192, 36)}${L(192, 44)}${L(100, 100)}${Z}`, 0.15));
  s.push(fill(circle(100, 92, 9), '$accent2'));
  return { viewbox: [0, 0, 200, 150], shapes: s };
}

function document() {
  const s = [];
  s.push(groundShadow(75, 198, 58));
  s.push(fill(`${M(14, 8)}${L(100, 8)}${L(136, 44)}${L(136, 188)}${L(14, 188)}${Z}`, PAPER));
  s.push(fill(`${M(100, 8)}${L(136, 44)}${L(100, 44)}${Z}`, '#d9dfec'));
  s.push(shade(`${M(110, 8)}${L(136, 8)}${L(136, 188)}${L(110, 188)}${Z}`, 0.04));
  s.push(fill(roundRect(28, 24, 50, 9, 4), '$accent'));
  for (let i = 0; i < 7; i++) s.push(fill(roundRect(28, 58 + i * 17, i % 3 === 2 ? 56 : 92, 7, 3.5), '$fg', i % 3 === 2 ? 0.2 : 0.3));
  s.push(fill(circle(104, 156, 14), '$accent2'));
  s.push({ d: ribbon([[98, 156], [102, 161], [110, 150]], 4), fill: WHITE });
  return { viewbox: [0, 0, 150, 200], shapes: s };
}

function coffeeCup() {
  const s = [];
  s.push(groundShadow(78, 160, 62));
  for (const x of [58, 82, 106]) s.push({ d: ribbon([[x, 44], [x - 8, 32], [x + 6, 20], [x - 4, 6]], 5), fill: '$fg', opacity: 0.18 });
  s.push(fill(ellipse(80, 150, 70, 12), '#e1e6f0'));
  s.push(fill(`${M(28, 56)}${L(132, 56)}${L(122, 134)}${A(10, 10, 0, 0, 1, 112, 144)}${L(48, 144)}${A(10, 10, 0, 0, 1, 38, 134)}${Z}`, '$accent'));
  s.push(shade(`${M(90, 56)}${L(132, 56)}${L(122, 134)}${A(10, 10, 0, 0, 1, 112, 144)}${L(84, 144)}${Z}`, 0.14));
  s.push(fill(rect(28, 56, 104, 14), WHITE, 0.22));
  s.push(fill(ellipse(80, 56, 52, 9), '#5a3a2a'));
  s.push(light(ellipse(70, 54, 22, 3.4), 0.25));
  s.push(line(`${M(130, 70)}${C(160, 68, 160, 112, 124, 114)}`, '$accent', 11));
  return { viewbox: [0, 0, 160, 166], shapes: s };
}

function desk() {
  const s = [];
  s.push(groundShadow(210, 192, 190));
  s.push(fill(roundRect(10, 20, 400, 22, 7), '#b98a5e'));
  s.push(fill(rect(10, 34, 400, 8), '#8f6a45'));
  s.push(light(roundRect(18, 22, 384, 5, 2.5), 0.3));
  s.push(fill(roundRect(26, 42, 16, 142, 3), '#8f6a45'));
  s.push(fill(roundRect(378, 42, 16, 142, 3), '#8f6a45'));
  s.push(fill(roundRect(250, 42, 128, 104, 6), '#a67a50'));
  for (const y of [50, 88]) {
    s.push(fill(roundRect(258, y, 112, 32, 5), '#c4966a'));
    s.push(fill(roundRect(298, y + 11, 32, 8, 4), '#6f4f33'));
  }
  return { viewbox: [0, 0, 420, 196], shapes: s };
}

function chair() {
  const s = [];
  s.push(groundShadow(90, 216, 70));
  s.push(fill(roundRect(24, 6, 36, 118, 16), '$accent'));
  s.push(shade(roundRect4(42, 6, 18, 118, [0, 16, 16, 0]), 0.14));
  s.push(fill(roundRect(24, 106, 112, 26, 12), '$accent'));
  s.push(shade(roundRect4(24, 120, 112, 12, [0, 0, 12, 12]), 0.16));
  s.push(fill(rect(70, 130, 10, 54), STEEL_DARK));
  s.push(fill(poly([[28, 200], [70, 184], [80, 184], [128, 200], [128, 208], [28, 208]]), '#6b7690'));
  for (const x of [34, 78, 122]) s.push(fill(circle(x, 210, 7), INK));
  s.push(fill(roundRect(110, 112, 24, 8, 4), '#6b7690'));
  return { viewbox: [0, 0, 160, 220], shapes: s };
}

function plant() {
  const s = [];
  s.push(groundShadow(70, 196, 50));
  const leaf = (x, y, ang, len, color, op) => {
    const a = rad(ang);
    const dx = Math.sin(a);
    const dy = -Math.cos(a);
    const tipx = x + dx * len;
    const tipy = y + dy * len;
    const nx = -dy * len * 0.32;
    const ny = dx * len * 0.32;
    s.push(fill(`${M(x, y)}${C(x + dx * len * 0.35 + nx, y + dy * len * 0.35 + ny, x + dx * len * 0.8 + nx * 0.8, y + dy * len * 0.8 + ny * 0.8, tipx, tipy)}${C(x + dx * len * 0.8 - nx * 0.8, y + dy * len * 0.8 - ny * 0.8, x + dx * len * 0.35 - nx, y + dy * len * 0.35 - ny, x, y)}${Z}`, color, op));
    s.push({ d: ribbon([[x, y], [x + dx * len * 0.5, y + dy * len * 0.5], [tipx, tipy]], 2.4), fill: WHITE, opacity: 0.35 });
  };
  for (const [a, l, c] of [[-62, 84, '#2fa86a'], [58, 88, '#2fa86a'], [-30, 108, '#38c27d'], [28, 112, '#38c27d'], [0, 120, '#46d28a']]) leaf(70, 118, a, l, c);
  s.push(fill(`${M(30, 112)}${L(110, 112)}${L(100, 190)}${A(8, 8, 0, 0, 1, 92, 196)}${L(48, 196)}${A(8, 8, 0, 0, 1, 40, 190)}${Z}`, '$accent'));
  s.push(fill(roundRect(24, 104, 92, 16, 6), '$accent'));
  s.push(shade(`${M(76, 104)}${L(116, 104)}${L(116, 120)}${L(110, 112)}${L(100, 190)}${A(8, 8, 0, 0, 1, 92, 196)}${L(76, 196)}${Z}`, 0.14));
  s.push(light(roundRect(30, 106, 52, 4, 2), 0.3));
  return { viewbox: [0, 0, 140, 200], shapes: s };
}

const badge = (color, glyph) => {
  const s = [fill(circle(80, 80, 70), color), shade(`${M(80, 10)}${A(70, 70, 0, 0, 1, 80, 150)}${Z}`, 0.12), light(`${M(34, 52)}${A(52, 52, 0, 0, 1, 90, 26)}${A(60, 60, 0, 0, 0, 40, 60)}${Z}`, 0.22)];
  s.push(...glyph);
  return { viewbox: [0, 0, 160, 160], shapes: s };
};

function check() {
  return badge(GREEN, [{ d: ribbon([[46, 82], [70, 106], [114, 56]], 18), fill: WHITE }]);
}

function cross() {
  return badge(RED, [{ d: ribbon([[54, 54], [106, 106]], 18), fill: WHITE }, { d: ribbon([[106, 54], [54, 106]], 18), fill: WHITE }]);
}

function warning() {
  const s = [];
  const tri = `${M(80, 14)}${C(86, 14, 90, 17, 93, 22)}${L(150, 120)}${C(156, 130, 150, 142, 138, 142)}${L(22, 142)}${C(10, 142, 4, 130, 10, 120)}${L(67, 22)}${C(70, 17, 74, 14, 80, 14)}${Z}`;
  s.push(fill(tri, YELLOW));
  s.push(shade(`${M(80, 14)}${C(86, 14, 90, 17, 93, 22)}${L(150, 120)}${C(156, 130, 150, 142, 138, 142)}${L(80, 142)}${Z}`, 0.1));
  s.push(fill(roundRect(72, 54, 16, 50, 8), '$fg'));
  s.push(fill(circle(80, 122, 9), '$fg'));
  return { viewbox: [0, 0, 160, 150], shapes: s };
}

function idea() {
  const s = [];
  for (let i = 0; i < 7; i++) {
    const a = rad(-120 + i * 40);
    s.push({ d: ribbon([[80 + Math.sin(a) * 66, 76 - Math.cos(a) * 66], [80 + Math.sin(a) * 86, 76 - Math.cos(a) * 86]], 7), fill: YELLOW, opacity: 0.9 });
  }
  s.push(fill(`${M(52, 118)}${C(36, 100, 30, 84, 34, 66)}${C(40, 36, 64, 18, 80, 18)}${C(96, 18, 120, 36, 126, 66)}${C(130, 84, 124, 100, 108, 118)}${C(104, 126, 102, 132, 102, 140)}${L(58, 140)}${C(58, 132, 56, 126, 52, 118)}${Z}`, YELLOW));
  s.push(shade(`${M(80, 18)}${C(96, 18, 120, 36, 126, 66)}${C(130, 84, 124, 100, 108, 118)}${C(104, 126, 102, 132, 102, 140)}${L(80, 140)}${Z}`, 0.1));
  s.push(light(`${M(50, 60)}${C(52, 44, 62, 34, 74, 30)}${C(64, 40, 58, 52, 58, 66)}${Z}`, 0.5));
  s.push(fill(roundRect(56, 142, 48, 11, 5), '#9aa4b8'));
  s.push(fill(roundRect(60, 156, 40, 11, 5), '#7d88a0'));
  s.push(fill(roundRect4(68, 168, 24, 10, [0, 0, 5, 5]), '#6b7690'));
  s.push({ d: ribbon([[66, 126], [66, 96], [80, 82], [94, 96], [94, 126]], 4), fill: '$fg', opacity: 0.35 });
  return { viewbox: [0, 0, 160, 182], shapes: s };
}

function question() {
  const s = [];
  s.push(fill(`${M(80, 8)}${C(122, 8, 152, 36, 152, 74)}${C(152, 112, 122, 138, 86, 138)}${L(56, 164)}${L(60, 136)}${C(32, 128, 8, 106, 8, 74)}${C(8, 36, 38, 8, 80, 8)}${Z}`, '$accent'));
  s.push(shade(`${M(80, 8)}${C(122, 8, 152, 36, 152, 74)}${C(152, 112, 122, 138, 86, 138)}${L(80, 138)}${Z}`, 0.12));
  s.push(light(`${M(24, 60)}${C(28, 34, 50, 20, 76, 18)}${C(52, 28, 38, 44, 36, 70)}${Z}`, 0.22));
  s.push(line(`${M(58, 56)}${C(58, 40, 102, 40, 102, 62)}${C(102, 78, 80, 78, 80, 94)}`, WHITE, 14));
  s.push(fill(circle(80, 116, 8.5), WHITE));
  return { viewbox: [0, 0, 160, 166], shapes: s };
}

function arrowRight() {
  const s = [];
  s.push(fill(`${M(10, 54)}${L(118, 54)}${L(118, 22)}${L(190, 80)}${L(118, 138)}${L(118, 106)}${L(10, 106)}${Z}`, '$accent'));
  s.push(shade(`${M(10, 80)}${L(190, 80)}${L(118, 138)}${L(118, 106)}${L(10, 106)}${Z}`, 0.14));
  s.push(light(`${M(14, 58)}${L(122, 58)}${L(122, 30)}${L(150, 52)}${L(14, 52)}${Z}`, 0.25));
  return { viewbox: [0, 0, 200, 160], shapes: s };
}

function gear() {
  const s = [];
  const teeth = 8;
  const pts = [];
  for (let i = 0; i < teeth; i++) {
    const base = (i / teeth) * 360 - 22.5;
    for (const [da, r] of [[0, 58], [8, 58], [15, 78], [30, 78], [37, 58]]) {
      const a = rad(base + da);
      pts.push([80 + Math.sin(a) * r, 80 - Math.cos(a) * r]);
    }
  }
  const body = poly(pts);
  s.push(fill(body, '#8d97ad'));
  s.push(fill(circle(80, 80, 52), '#a4adc0'));
  s.push(shade(`${M(80, 28)}${A(52, 52, 0, 0, 1, 80, 132)}${Z}`, 0.12));
  s.push(fill(circle(80, 80, 24), '$bg'));
  s.push(shade(circle(80, 80, 24), 0.12));
  s.push(light(`${M(36, 60)}${A(48, 48, 0, 0, 1, 72, 32)}${L(72, 40)}${A(40, 40, 0, 0, 0, 44, 62)}${Z}`, 0.3));
  return { viewbox: [0, 0, 160, 160], shapes: s };
}

/** name -> {build, h (fraction of the stage height), tags, hold} */
export const PROPS = {
  laptop: { build: laptop, h: 0.1, tags: ['computer', 'notebook', 'work', 'office'] },
  phone: { build: phone, h: 0.11, tags: ['mobile', 'smartphone', 'call', 'app'] },
  monitor: { build: monitor, h: 0.16, tags: ['screen', 'display', 'computer', 'desktop'] },
  server: { build: server, h: 0.26, tags: ['rack', 'datacenter', 'hosting', 'it'] },
  router: { build: router, h: 0.09, tags: ['network', 'wifi', 'internet', 'gateway'] },
  cloud: { build: cloud, h: 0.14, tags: ['internet', 'storage', 'online', 'hosting'] },
  database: { build: database, h: 0.17, tags: ['data', 'storage', 'sql', 'records'] },
  lock: { build: lock, h: 0.14, tags: ['security', 'lock', 'padlock', 'protection'] },
  key: { build: key, h: 0.08, tags: ['security', 'password', 'access', 'unlock'] },
  shield: { build: shield, h: 0.16, tags: ['security', 'shield', 'protection', 'firewall'] },
  mail: { build: mail, h: 0.1, tags: ['email', 'message', 'letter', 'phishing'] },
  document: { build: document, h: 0.16, tags: ['file', 'paper', 'report', 'contract'] },
  'coffee-cup': { build: coffeeCup, h: 0.08, tags: ['coffee', 'break', 'drink', 'cup'], hold: true },
  desk: { build: desk, h: 0.2, tags: ['table', 'furniture', 'office', 'workplace'] },
  chair: { build: chair, h: 0.22, tags: ['seat', 'furniture', 'office', 'sit'] },
  plant: { build: plant, h: 0.2, tags: ['nature', 'decor', 'green', 'pot'] },
  check: { build: check, h: 0.1, tags: ['yes', 'ok', 'correct', 'success'] },
  cross: { build: cross, h: 0.1, tags: ['no', 'wrong', 'error', 'denied'] },
  warning: { build: warning, h: 0.12, tags: ['alert', 'danger', 'attention', 'caution'] },
  idea: { build: idea, h: 0.14, tags: ['light bulb', 'tip', 'insight', 'thought'] },
  question: { build: question, h: 0.13, tags: ['ask', 'unknown', 'help', 'why'] },
  'arrow-right': { build: arrowRight, h: 0.08, tags: ['direction', 'next', 'flow', 'then'] },
  gear: { build: gear, h: 0.12, tags: ['settings', 'process', 'engine', 'configuration'] },
};

```


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node --test assets/storymaker/test/props.test.mjs`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add assets/storymaker/lib/props.mjs assets/storymaker/test/props.test.mjs
git commit -m "feat(storymaker): 23 starter props" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```


### Task D6: The four environments with spots, floor lines, day and night and aspect overrides

**Files:**
- Create: `assets/storymaker/lib/environments.mjs`
- Test: `assets/storymaker/test/environments.test.mjs`

Plain stage, office, home office and abstract tech (1920 x 1080 design frame, parallax layers drawn back to front). Day colours follow the palette (`$bg`, `$bg2`, `$accent`, `$accent2`); the night variant overrides `bg`/`bg2` and the local tokens. The 9:16 and 1:1 overrides keep every spot inside the slice of the design frame that the output shows. The last test places a seated actor at the desk spot and checks that the hips meet the chair and the hands the desk top.

- [ ] **Step 1: Write the failing test**

`assets/storymaker/test/environments.test.mjs`:

```js
// Task D6: the four starter environments.
// Run: node --test assets/storymaker/test/environments.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePath } from '../lib/pathdata.mjs';
import { ENVIRONMENTS, ENV_META } from '../lib/environments.mjs';
import { buildRig } from '../lib/rig.mjs';
import { POSES } from '../lib/motion.mjs';
import { boneWorlds } from '../lib/svg.mjs';

const PALETTE_TOKENS = ['bg', 'bg2', 'fg', 'accent', 'accent2', 'text', 'outline'];
const BASE = ['far-left', 'left', 'center', 'right', 'far-right'];
const EXTRA = { 'plain-stage': ['board'], office: ['desk', 'door', 'board', 'window'], 'home-office': ['desk', 'door', 'window'], 'abstract-tech': ['board'] };

test('four environments with the five base spots, their extra spots and a floor line', () => {
  assert.deepEqual(Object.keys(ENVIRONMENTS), ['plain-stage', 'office', 'home-office', 'abstract-tech']);
  for (const [name, meta] of Object.entries(ENV_META)) {
    for (const s of [...BASE, ...EXTRA[name]]) assert.ok(meta.spots[s], `${name} ${s}`);
    assert.ok(meta.floor.y_far < meta.floor.y_near && meta.floor.scale_far < meta.floor.scale_near, name);
  }
  assert.equal(ENV_META.office.spots.desk.face, 'right');
  assert.equal(ENV_META.office.spots.door.face, 'left');
});

test('layers are valid path data and only use palette tokens or tokens defined for day and night', () => {
  for (const [name, build] of Object.entries(ENVIRONMENTS)) {
    const env = build();
    assert.ok(env.layers.length >= 3, name);
    const day = Object.keys(env.tokens.day).sort();
    const night = Object.keys(env.tokens.night).filter((k) => !PALETTE_TOKENS.includes(k)).sort();
    assert.deepEqual(night, day.filter((k) => !PALETTE_TOKENS.includes(k)), `${name}: night defines the same local tokens`);
    const known = new Set([...PALETTE_TOKENS, ...day]);
    for (const layer of env.layers) {
      assert.ok(layer.parallax >= 0 && layer.parallax <= 1, `${name} ${layer.name}`);
      for (const [i, s] of layer.shapes.entries()) {
        assert.ok(parsePath(s.d).ok, `${name} ${layer.name}[${i}]`);
        for (const c of [s.fill, s.stroke, ...(s.gradient ? s.gradient.stops.map((st) => st[1]) : [])].filter(Boolean)) {
          if (c[0] === '$') assert.ok(known.has(c.slice(1)), `${name} ${layer.name}[${i}] ${c}`);
        }
      }
    }
  }
});

test('the 9:16 and 1:1 crops keep every spot in view', () => {
  // the output shows a slice of the design frame as wide as it is at the design height
  const half = { '9:16': (100 * ((1080 * 9) / 16)) / 1920 / 2, '1:1': (100 * 1080) / 1920 / 2 };
  for (const [name, meta] of Object.entries(ENV_META)) {
    for (const [asp, h] of Object.entries(half)) {
      const a = meta.aspects[asp];
      assert.ok(typeof a.focus_x === 'number', `${name} ${asp}`);
      for (const [spot, base] of Object.entries(meta.spots)) {
        const x = (a.spots?.[spot] || base).x;
        assert.ok(x >= a.focus_x - h + 3 && x <= a.focus_x + h - 3, `${name} ${asp} ${spot} x=${x}`);
      }
      for (const spot of Object.keys(a.spots || {})) assert.ok(meta.spots[spot], `${name} ${asp} overrides unknown spot ${spot}`);
    }
  }
});

test('a seated actor at the desk spot sits on the chair and types on the desk (both offices)', () => {
  const rig = buildRig();
  const m = ENV_META.office;
  const depth = m.floor.scale_far + ((m.spots.desk.y - m.floor.y_far) / (m.floor.y_near - m.floor.y_far)) * (m.floor.scale_near - m.floor.scale_far);
  const unit = (rig.height * 1080 * depth) / 1000; // screen px per rig unit
  const feetX = (m.spots.desk.x / 100) * 1920;
  const feetY = (m.spots.desk.y / 100) * 1080;
  const worlds = boneWorlds(rig, 'average', 'three_quarter', POSES.typing.bones, {});
  const at = (bone, x, y) => {
    const w = worlds.get(bone);
    const px = w[0] * x + w[2] * y + w[4];
    const py = w[1] * x + w[3] * y + w[5] + POSES.typing.offset_y;
    return [feetX + px * unit, feetY + py * unit];
  };
  const [, hipsY] = at('hips', 0, 0);
  const [handX, handY] = at('lower_arm_r', 0, 135 + 40);
  const desk = ENVIRONMENTS.office().layers.find((l) => l.name === 'furniture').shapes.find((s) => s.fill === '$wood' && s.d.startsWith('M788 698'));
  const home = ENVIRONMENTS['home-office']().layers.find((l) => l.name === 'furniture').shapes.find((s) => s.fill === '$deskwood' && s.d.startsWith('M788 698'));
  assert.ok(desk && home, 'both desks have their top plate at y = 698 px');
  assert.ok(Math.abs(handY - 698) < 45, `hands at ${Math.round(handY)} px, desk top at 698 px`);
  assert.ok(handX > 780 && handX < 1300, `hands at x = ${Math.round(handX)} px, above the desk`);
  assert.ok(Math.abs(hipsY - 774) < 30, `hips at ${Math.round(hipsY)} px, chair seat at 774 px`);
});
```


- [ ] **Step 2: Run the test to verify it fails**

Run: `node --test assets/storymaker/test/environments.test.mjs`
Expected: FAIL (`ERR_MODULE_NOT_FOUND`: cannot find `assets/storymaker/lib/environments.mjs`)

- [ ] **Step 3: Implement**

`assets/storymaker/lib/environments.mjs`:

```js
// The four starter environments (1920 x 1080 design frame). Layers are drawn in array order; parallax runs from
// 0 (fixed sky) to 1 (floor). Day colours follow the palette where possible ($bg, $bg2, $accent, $accent2); the night
// tokens override bg/bg2 and the local tokens. Local tokens exist in both variants.

import {
  fill, shade, light, line, linear, radial, ellipse, circle, rect, roundRect, roundRect4, poly, M, L, C, Z, A, rng,
} from './util.mjs';

const W = 1920;
const H = 1080;

const BASE_SPOTS = {
  'far-left': { x: 12, y: 88 },
  left: { x: 30, y: 88 },
  center: { x: 50, y: 88 },
  right: { x: 70, y: 88 },
  'far-right': { x: 88, y: 88 },
};

/** Spots, floor and aspect overrides per environment (design frame percent; y is the feet line). */
export const ENV_META = {
  'plain-stage': {
    spots: { ...BASE_SPOTS, board: { x: 74, y: 62 } },
    floor: { y_near: 96, y_far: 70, scale_near: 1.05, scale_far: 0.8 },
    aspects: {
      '9:16': { focus_x: 50, spots: { 'far-left': { x: 38, y: 88 }, left: { x: 42, y: 88 }, right: { x: 58, y: 88 }, 'far-right': { x: 62, y: 88 }, board: { x: 56, y: 62 } } },
      '1:1': { focus_x: 50, spots: { 'far-left': { x: 25, y: 88 }, 'far-right': { x: 75, y: 88 }, board: { x: 68, y: 62 } } },
    },
  },
  office: {
    spots: { ...BASE_SPOTS, desk: { x: 36, y: 84, face: 'right' }, door: { x: 90, y: 82, face: 'left' }, board: { x: 70, y: 60 }, window: { x: 18, y: 82 } },
    floor: { y_near: 96, y_far: 72, scale_near: 1.05, scale_far: 0.82 },
    aspects: {
      '9:16': { focus_x: 44, spots: { 'far-left': { x: 33, y: 88 }, left: { x: 37, y: 88 }, center: { x: 44, y: 88 }, right: { x: 51, y: 88 }, 'far-right': { x: 55, y: 88 }, door: { x: 56, y: 82, face: 'left' }, board: { x: 56, y: 60 }, window: { x: 33, y: 82 } } },
      '1:1': { focus_x: 45, spots: { 'far-left': { x: 24, y: 88 }, center: { x: 45, y: 88 }, right: { x: 60, y: 88 }, 'far-right': { x: 66, y: 88 }, door: { x: 66, y: 82, face: 'left' }, board: { x: 62, y: 60 }, window: { x: 24, y: 82 } } },
    },
  },
  'home-office': {
    spots: { ...BASE_SPOTS, desk: { x: 36, y: 84, face: 'right' }, door: { x: 90, y: 82, face: 'left' }, window: { x: 14, y: 82 } },
    floor: { y_near: 96, y_far: 72, scale_near: 1.05, scale_far: 0.82 },
    aspects: {
      '9:16': { focus_x: 44, spots: { 'far-left': { x: 33, y: 88 }, left: { x: 37, y: 88 }, center: { x: 44, y: 88 }, right: { x: 51, y: 88 }, 'far-right': { x: 55, y: 88 }, door: { x: 56, y: 82, face: 'left' }, window: { x: 33, y: 82 } } },
      '1:1': { focus_x: 45, spots: { 'far-left': { x: 24, y: 88 }, center: { x: 45, y: 88 }, right: { x: 60, y: 88 }, 'far-right': { x: 66, y: 88 }, door: { x: 66, y: 82, face: 'left' }, window: { x: 24, y: 82 } } },
    },
  },
  'abstract-tech': {
    spots: { ...BASE_SPOTS, board: { x: 70, y: 58 } },
    floor: { y_near: 96, y_far: 72, scale_near: 1.05, scale_far: 0.82 },
    aspects: {
      '9:16': { focus_x: 50, spots: { 'far-left': { x: 38, y: 88 }, left: { x: 42, y: 88 }, right: { x: 58, y: 88 }, 'far-right': { x: 62, y: 88 }, board: { x: 56, y: 58 } } },
      '1:1': { focus_x: 50, spots: { 'far-left': { x: 25, y: 88 }, 'far-right': { x: 75, y: 88 }, board: { x: 68, y: 58 } } },
    },
  },
};

// ---------------------------------------------------------------- plain stage
function plainStage() {
  const horizon = 756;
  const sky = [
    linear(rect(0, 0, W, H), [[0, '$bg'], [0.7, '$bg2'], [1, '$bg2']], 0, 0, 0, horizon),
    fill(circle(300, 250, 230), '#ffffff', 0.3),
    fill(circle(1640, 190, 150), '$accent2', 0.12),
    fill(circle(1500, 420, 270), '#ffffff', 0.18),
    fill(circle(520, 520, 90), '$accent', 0.08),
    fill(circle(1180, 120, 34), '$accent', 0.16),
    fill(circle(150, 600, 52), '$accent2', 0.14),
  ];
  const r = rng(3);
  for (let i = 0; i < 18; i++) sky.push(fill(circle(80 + r() * 1760, 40 + r() * 220, 2 + r() * 3), '$star', 0.7));
  const back = [
    fill(`${M(0, 660)}${C(300, 600, 620, 640, 900, 664)}${C(1200, 690, 1500, 620, 1920, 650)}${L(1920, horizon)}${L(0, horizon)}${Z}`, '$bg2'),
    shade(`${M(0, 660)}${C(300, 600, 620, 640, 900, 664)}${C(1200, 690, 1500, 620, 1920, 650)}${L(1920, horizon)}${L(0, horizon)}${Z}`, 0.05),
    fill(`${M(0, 700)}${C(400, 660, 800, 700, 1200, 700)}${C(1500, 700, 1700, 670, 1920, 690)}${L(1920, horizon)}${L(0, horizon)}${Z}`, '$bg2'),
    shade(`${M(0, 700)}${C(400, 660, 800, 700, 1200, 700)}${C(1500, 700, 1700, 670, 1920, 690)}${L(1920, horizon)}${L(0, horizon)}${Z}`, 0.08),
  ];
  const floor = [
    fill(rect(0, horizon, W, H - horizon), '$bg2'),
    shade(rect(0, horizon, W, H - horizon), 0.07),
    fill(ellipse(960, 900, 760, 120), '#ffffff', 0.3),
    fill(ellipse(960, 904, 520, 76), '#ffffff', 0.22),
    shade(rect(0, horizon, W, 4), 0.1),
    shade(rect(0, 1010, W, 70), 0.06),
  ];
  return {
    layers: [{ name: 'sky', parallax: 0, shapes: sky }, { name: 'hills', parallax: 0.3, shapes: back }, { name: 'floor', parallax: 1, shapes: floor }],
    tokens: {
      day: { star: '#ffffff' },
      night: { bg: '#1b2142', bg2: '#2c356b', star: '#dfe6ff' },
    },
  };
}

// ---------------------------------------------------------------- shared furniture for the two offices
function chair(x, color) {
  const s = [];
  s.push(shade(ellipse(x + 80, 905, 100, 12), 0.14));
  s.push(fill(roundRect(x - 12, 560, 34, 236, 16), color));
  s.push(shade(roundRect4(x + 6, 560, 16, 236, [0, 16, 16, 0]), 0.14));
  s.push(fill(roundRect(x - 12, 774, 164, 36, 16), color));
  s.push(shade(roundRect4(x - 12, 796, 164, 14, [0, 0, 16, 16]), 0.16));
  s.push(fill(rect(x + 60, 808, 16, 66), '#6b7690'));
  s.push(fill(poly([[x - 6, 902], [x + 60, 874], [x + 76, 874], [x + 146, 902], [x + 146, 912], [x - 6, 912]]), '#4b5573'));
  for (const dx of [0, 70, 140]) s.push(fill(circle(x + dx, 914, 10), '#2b3350'));
  return s;
}

function laptopSide(x, y) {
  return [
    fill(roundRect(x, y - 12, 160, 12, 5), '#c9d0dc'),
    shade(roundRect4(x, y - 5, 160, 5, [0, 0, 5, 5]), 0.2),
    fill(poly([[x + 146, y - 10], [x + 160, y - 10], [x + 196, y - 118], [x + 182, y - 122]]), '#9aa4b8'),
    fill(poly([[x + 148, y - 12], [x + 154, y - 12], [x + 188, y - 114], [x + 183, y - 116]]), '$accent2', 0.9),
    light(rect(x + 8, y - 11, 120, 3), 0.4),
  ];
}

function mug(x, y, color) {
  return [fill(roundRect(x, y - 40, 36, 40, 7), color), shade(roundRect4(x + 18, y - 40, 18, 40, [0, 7, 7, 0]), 0.14), line(`${M(x + 36, y - 30)}${C(x + 54, y - 30, x + 54, y - 10, x + 34, y - 10)}`, color, 7), fill(ellipse(x + 18, y - 40, 15, 4), '#5a3a2a')];
}

function deskLamp(x, y, light1) {
  const s = [];
  s.push(fill(ellipse(x, y, 30, 7), '#4b5573'));
  s.push(line(`${M(x, y - 4)}${L(x + 6, y - 90)}${L(x - 34, y - 140)}`, '#4b5573', 8));
  s.push(fill(`${M(x - 62, y - 126)}${L(x - 14, y - 160)}${L(x + 2, y - 128)}${L(x - 44, y - 106)}${Z}`, light1));
  s.push(fill(`${M(x - 44, y - 106)}${L(x - 120, y + 8)}${L(x + 10, y + 8)}${L(x + 2, y - 128)}${Z}`, '$lampglow', 0.16));
  return s;
}

function plantPot(x, y, s = 1) {
  const out = [];
  const leaves = [[-70, 170, '#2fa86a'], [64, 176, '#2fa86a'], [-32, 220, '#38c27d'], [30, 226, '#38c27d'], [0, 250, '#46d28a']];
  for (const [ang, len, color] of leaves) {
    const a = (ang * Math.PI) / 180;
    const dx = Math.sin(a) * len * s;
    const dy = -Math.cos(a) * len * s;
    const nx = -Math.cos(a) * len * 0.2 * s;
    const ny = -Math.sin(a) * len * 0.2 * s;
    out.push(fill(`${M(x, y - 70 * s)}${C(x + dx * 0.4 + nx, y - 70 * s + dy * 0.4 + ny, x + dx * 0.85 + nx, y - 70 * s + dy * 0.85 + ny, x + dx, y - 70 * s + dy)}${C(x + dx * 0.85 - nx, y - 70 * s + dy * 0.85 - ny, x + dx * 0.4 - nx, y - 70 * s + dy * 0.4 - ny, x, y - 70 * s)}${Z}`, color));
  }
  out.push(fill(`${M(x - 52 * s, y - 80 * s)}${L(x + 52 * s, y - 80 * s)}${L(x + 42 * s, y - 8 * s)}${L(x - 42 * s, y - 8 * s)}${Z}`, '$accent'));
  out.push(shade(`${M(x, y - 80 * s)}${L(x + 52 * s, y - 80 * s)}${L(x + 42 * s, y - 8 * s)}${L(x, y - 8 * s)}${Z}`, 0.14));
  return out;
}

function windowCity(x, y, w, h) {
  const s = [];
  s.push(fill(roundRect(x, y, w, h, 12), '$frame'));
  const gx = x + 20;
  const gy = y + 20;
  const gw = w - 40;
  const gh = h - 40;
  s.push(linear(rect(gx, gy, gw, gh), [[0, '$sky'], [1, '$sky2']], 0, gy, 0, gy + gh));
  s.push(fill(circle(gx + gw * 0.76, gy + 80, 34), '$orb'));
  const r = rng(11);
  let bx = gx;
  while (bx < gx + gw) {
    const bw = 40 + Math.floor(r() * 50);
    const bh = 90 + Math.floor(r() * 170);
    const width = Math.min(bw, gx + gw - bx);
    s.push(fill(rect(bx, gy + gh - bh, width, bh), r() > 0.5 ? '$city' : '$city2'));
    for (let wy = gy + gh - bh + 14; wy < gy + gh - 16; wy += 28) for (let wx = bx + 8; wx < bx + width - 12; wx += 22) if (r() > 0.45) s.push(fill(rect(wx, wy, 10, 14), '$lit'));
    bx += width + 4;
  }
  s.push(fill(rect(x + w / 2 - 4, gy, 8, gh), '$frame'));
  s.push(fill(rect(gx, y + h * 0.5 - 4, gw, 8), '$frame'));
  s.push(light(poly([[gx, gy], [gx + gw * 0.5, gy], [gx, gy + gh * 0.6]]), 0.12));
  s.push(fill(roundRect(x - 20, y + h - 4, w + 40, 20, 6), '$frame'));
  s.push(shade(roundRect4(x - 20, y + h + 8, w + 40, 8, [0, 0, 6, 6]), 0.16));
  return s;
}

function door(x, y, w, h) {
  return [
    fill(roundRect(x - 16, y - 16, w + 32, h + 16, 8), '$frame'),
    fill(rect(x, y, w, h), '$wood'),
    fill(roundRect(x + 22, y + 26, w - 44, h * 0.42, 6), '$wood2'),
    fill(roundRect(x + 22, y + h * 0.5, w - 44, h * 0.42, 6), '$wood2'),
    shade(rect(x + w * 0.55, y, w * 0.45, h), 0.08),
    fill(circle(x + w - 26, y + h * 0.5, 9), '#e8c25a'),
  ];
}

// ---------------------------------------------------------------- office
function office() {
  const base = 777;
  const wall = [
    fill(rect(0, 0, W, base), '$bg'),
    fill(rect(0, 640, W, base - 640), '$wall2'),
    fill(rect(0, 632, W, 10), '$wood'),
    fill(rect(0, base - 14, W, 14), '$wood2'),
    ...windowCity(150, 150, 400, 470),
    // framed pictures
    fill(roundRect(640, 220, 170, 130, 8), '$wood'),
    fill(roundRect(652, 232, 146, 106, 4), '$bg2'),
    fill(circle(700, 280, 22), '$accent'),
    fill(poly([[724, 322], [760, 262], [790, 322]]), '$accent2'),
    fill(roundRect(836, 250, 90, 100, 8), '$wood'),
    fill(roundRect(846, 260, 70, 80, 4), '$bg2'),
    fill(circle(881, 300, 18), '$accent2'),
    // clock above the door
    fill(circle(1536, 190, 54), '$frame'),
    fill(circle(1536, 190, 44), '$bg'),
    line(`${M(1536, 190)}${L(1536, 158)}`, '$ink', 5),
    line(`${M(1536, 190)}${L(1560, 198)}`, '$ink', 5),
    fill(circle(1536, 190, 5), '$ink'),
    // whiteboard
    fill(roundRect(1040, 170, 610, 498, 14), '$boardframe'),
    fill(roundRect(1054, 184, 582, 470, 8), '$board'),
    light(poly([[1054, 184], [1280, 184], [1054, 400]]), 0.5),
    fill(roundRect(1250, 658, 190, 14, 6), '$boardframe'),
    fill(roundRect(1280, 650, 36, 10, 3), '$accent'),
    fill(roundRect(1326, 650, 36, 10, 3), '$accent2'),
    ...door(1650, 236, 184, 541),
  ];
  const floor = [
    fill(rect(0, base, W, H - base), '$floor'),
    ...Array.from({ length: 9 }, (_, i) => fill(rect(0, base + 34 + i * 34, W, 3), '$floor2')),
    ...Array.from({ length: 30 }, (_, i) => fill(rect(((i * 197) % 1900) + 10, base + 34 + (i % 9) * 34, 3, 34), '$floor2')),
    fill(roundRect(540, 846, 880, 170, 26), '$accent2', 0.22),
    fill(roundRect(560, 862, 840, 138, 18), '$accent2', 0.14),
    shade(rect(0, base, W, 6), 0.1),
    shade(rect(0, 1010, W, 70), 0.06),
  ];
  const furniture = [
    ...plantPot(120, 880, 1),
    ...chair(630, '$accent'),
    // desk: top plate, drawer unit on the right, open knee space on the left
    shade(ellipse(1050, 912, 330, 14), 0.14),
    fill(roundRect(780, 698, 540, 28, 8), '$wood'),
    light(roundRect(788, 700, 524, 6, 3), 0.3),
    fill(rect(780, 718, 540, 10), '$wood2'),
    fill(roundRect(1126, 726, 190, 182, 6), '$wood2'),
    fill(roundRect(1138, 738, 166, 76, 6), '$wood'),
    fill(roundRect(1138, 820, 166, 76, 6), '$wood'),
    fill(roundRect(1196, 768, 50, 10, 5), '#6f4f33'),
    fill(roundRect(1196, 850, 50, 10, 5), '#6f4f33'),
    fill(roundRect(794, 726, 18, 182, 4), '$wood2'),
    ...laptopSide(880, 698),
    ...mug(1118, 698, '$accent'),
    ...deskLamp(1262, 698, '$accent'),
  ];
  return {
    layers: [{ name: 'wall', parallax: 0.1, shapes: wall }, { name: 'floor', parallax: 1, shapes: floor }, { name: 'furniture', parallax: 0.7, shapes: furniture }],
    tokens: {
      day: { wall2: '#efe2cf', wood: '#c9a27a', wood2: '#b58a62', frame: '#ffffff', sky: '#a9dcf3', sky2: '#e6f6fd', city: '#8eb2cc', city2: '#7399b6', lit: '#a9c6da', orb: '#fff3b8', floor: '#d9b894', floor2: '#c8a37c', board: '#ffffff', boardframe: '#c3cad8', ink: '#2b3350', lampglow: '#ffffff' },
      night: { bg: '#232a4d', bg2: '#2f3865', wall2: '#1d2442', wood: '#7a5a44', wood2: '#664a38', frame: '#c9d0ea', sky: '#141a3a', sky2: '#27306a', city: '#1e2650', city2: '#2a3470', lit: '#ffd98a', orb: '#f4f1de', floor: '#3a3b5c', floor2: '#2f304e', board: '#e9edf7', boardframe: '#8c95b3', ink: '#0f1226', lampglow: '#ffd98a' },
    },
  };
}

// ---------------------------------------------------------------- home office
function bookshelf(x, y, w, h) {
  const s = [fill(roundRect(x, y, w, h, 8), '$wood2'), fill(rect(x + 12, y + 12, w - 24, h - 24), '$shelfback')];
  const r = rng(5);
  const colors = ['$accent', '$accent2', '#f2b544', '#6c7bd6', '#e86f6f', '#58b99a'];
  const rows = 5;
  const rowH = (h - 24) / rows;
  for (let i = 0; i < rows; i++) {
    const by = y + 12 + (i + 1) * rowH;
    let bx = x + 16;
    while (bx < x + w - 40) {
      const bw = 14 + Math.floor(r() * 16);
      const bh = rowH - 14 - Math.floor(r() * 26);
      s.push(fill(rect(bx, by - bh, bw, bh), colors[Math.floor(r() * colors.length)]));
      bx += bw + 2;
    }
    s.push(fill(rect(x + 12, by, w - 24, 8), '$wood'));
  }
  return s;
}

function homeOffice() {
  const base = 777;
  const wall = [
    fill(rect(0, 0, W, base), '$wallh'),
    ...Array.from({ length: 24 }, (_, i) => fill(rect(40 + i * 80, 0, 3, base), '$wallh2', 0.6)),
    fill(rect(0, base - 18, W, 18), '$wood2'),
    // window with curtains
    fill(roundRect(110, 130, 360, 470, 12), '$frame'),
    linear(rect(128, 148, 324, 434), [[0, '$sky'], [1, '$sky2']], 0, 148, 0, 582),
    fill(circle(360, 240, 32), '$orb'),
    fill(`${M(128, 520)}${C(210, 470, 300, 500, 452, 480)}${L(452, 582)}${L(128, 582)}${Z}`, '$hill'),
    fill(circle(230, 440, 46), '$tree'),
    fill(rect(224, 470, 12, 110), '$wood2'),
    fill(rect(285, 148, 8, 434), '$frame'),
    fill(roundRect(70, 100, 440, 14, 7), '$wood2'),
    fill(`${M(84, 114)}${C(150, 200, 130, 400, 168, 600)}${L(84, 600)}${Z}`, '$accent'),
    fill(`${M(496, 114)}${C(430, 200, 450, 400, 412, 600)}${L(496, 600)}${Z}`, '$accent'),
    shade(`${M(112, 130)}${C(146, 240, 138, 420, 168, 600)}${L(146, 600)}${C(120, 420, 124, 240, 98, 130)}${Z}`, 0.1),
    shade(`${M(468, 130)}${C(434, 240, 442, 420, 412, 600)}${L(434, 600)}${C(460, 420, 456, 240, 482, 130)}${Z}`, 0.1),
    // shelf with books and a plant over the desk
    fill(roundRect(780, 300, 480, 18, 6), '$wood'),
    fill(rect(820, 300, 10, 40), '$wood2'),
    fill(rect(1210, 300, 10, 40), '$wood2'),
    fill(rect(840, 226, 22, 74), '$accent2'),
    fill(rect(864, 240, 18, 60), '$accent'),
    fill(rect(884, 232, 24, 68), '#f2b544'),
    fill(roundRect(1100, 246, 70, 54, 8), '$accent'),
    fill(circle(1118, 232, 24), '#38c27d'),
    fill(circle(1150, 224, 28), '#2fa86a'),
    fill(circle(1136, 200, 22), '#46d28a'),
    // framed art
    fill(roundRect(800, 400, 150, 190, 8), '$wood'),
    fill(roundRect(814, 414, 122, 162, 4), '$bg'),
    fill(circle(875, 470, 30), '$accent2'),
    fill(poly([[826, 566], [875, 500], [924, 566]]), '$accent'),
    fill(roundRect(980, 410, 220, 120, 8), '$wood'),
    fill(roundRect(992, 422, 196, 96, 4), '$bg2'),
    line(`${M(1004, 500)}${L(1050, 466)}${L(1096, 486)}${L(1176, 438)}`, '$accent', 6),
    ...bookshelf(1330, 90, 250, 687),
    ...door(1650, 236, 184, 541),
  ];
  const floor = [
    fill(rect(0, base, W, H - base), '$floor'),
    ...Array.from({ length: 10 }, (_, i) => fill(rect(0, base + 30 + i * 31, W, 3), '$floor2')),
    ...Array.from({ length: 34 }, (_, i) => fill(rect(((i * 173) % 1880) + 10, base + 30 + (i % 10) * 31, 3, 31), '$floor2')),
    fill(ellipse(900, 940, 520, 70), '$accent', 0.22),
    fill(ellipse(900, 940, 440, 54), '$accent', 0.14),
    fill(poly([[200, base], [420, base], [640, 1000], [60, 1000]]), '$sun', 0.22),
    shade(rect(0, base, W, 6), 0.1),
    shade(rect(0, 1010, W, 70), 0.06),
  ];
  const furniture = [
    ...chair(630, '$accent2'),
    shade(ellipse(1050, 912, 330, 14), 0.14),
    fill(roundRect(780, 698, 540, 24, 8), '$deskwood'),
    light(roundRect(788, 700, 524, 6, 3), 0.3),
    fill(roundRect(1290, 722, 20, 186, 4), '$deskwood2'),
    fill(roundRect(798, 722, 20, 186, 4), '$deskwood2'),
    fill(roundRect(820, 726, 460, 34, 6), '$deskwood2'),
    ...laptopSide(880, 698),
    ...mug(1118, 698, '$accent2'),
    ...deskLamp(1250, 698, '$accent'),
    ...plantPot(1210, 906, 0.8),
  ];
  return {
    layers: [{ name: 'wall', parallax: 0.1, shapes: wall }, { name: 'floor', parallax: 1, shapes: floor }, { name: 'furniture', parallax: 0.7, shapes: furniture }],
    tokens: {
      day: { wallh: '#f3e6d6', wallh2: '#ead9c3', wood: '#c9a27a', wood2: '#b58a62', deskwood: '#e7cfae', deskwood2: '#d3b48c', frame: '#ffffff', sky: '#bfe6f6', sky2: '#eaf8fd', hill: '#8fcf9a', tree: '#52b788', orb: '#fff3b8', floor: '#d8b48c', floor2: '#c79f77', shelfback: '#efe0c9', sun: '#fff1b8', lampglow: '#ffffff' },
      night: { bg: '#2c2540', bg2: '#3d3560', wallh: '#2d2744', wallh2: '#352e52', wood: '#6e5340', wood2: '#5c4535', deskwood: '#8a6f58', deskwood2: '#74594a', frame: '#c4c0de', sky: '#14193a', sky2: '#2a2f6a', hill: '#1f3b4a', tree: '#19424a', orb: '#f4f1de', floor: '#3b3354', floor2: '#322b49', shelfback: '#241f38', sun: '#8d8fe0', lampglow: '#ffd98a' },
    },
  };
}

// ---------------------------------------------------------------- abstract tech
function abstractTech() {
  const horizon = 777;
  const sky = [
    linear(rect(0, 0, W, H), [[0, '$t1'], [1, '$t2']], 0, 0, 0, horizon),
    radial(circle(1500, 330, 760), [[0, '$accent2'], [1, '$t1']], 1500, 330, 760, 0.35),
    radial(circle(260, 640, 520), [[0, '$accent'], [1, '$t1']], 260, 640, 520, 0.22),
  ];
  const r = rng(21);
  for (let i = 0; i < 70; i++) sky.push(fill(circle(r() * W, r() * 640, 1 + r() * 2.4), '#ffffff', 0.25 + r() * 0.45));
  const back = [];
  const nodes = [];
  for (let i = 0; i < 26; i++) nodes.push([80 + r() * 1760, 60 + r() * 640]);
  nodes.forEach(([x, y], i) => {
    const near = nodes.map(([nx, ny], j) => [Math.hypot(nx - x, ny - y), j]).filter(([d, j]) => j > i && d < 320).sort((a, b) => a[0] - b[0]).slice(0, 2);
    for (const [, j] of near) back.push(line(`${M(x, y)}${L(nodes[j][0], nodes[j][1])}`, '$grid', 2, 0.35));
  });
  nodes.forEach(([x, y], i) => {
    back.push(fill(circle(x, y, 14 + (i % 3) * 4), '$accent2', 0.12));
    back.push(fill(circle(x, y, 5 + (i % 3) * 1.5), i % 4 === 0 ? '$accent' : '$accent2', 0.9));
  });
  const mid = [];
  // floating cards
  for (const [x, y, w, h, c] of [[120, 160, 230, 150, '$accent2'], [1560, 520, 250, 160, '$accent'], [300, 520, 200, 120, '$accent2']]) {
    mid.push(fill(roundRect(x, y, w, h, 16), '$panel', 0.55));
    mid.push(line(roundRect(x, y, w, h, 16), c, 3, 0.8));
    for (let i = 0; i < 3; i++) mid.push(fill(roundRect(x + 18, y + 22 + i * 26, w * (0.7 - i * 0.16), 10, 5), c, 0.7 - i * 0.15));
  }
  // HUD board
  const board = [
    fill(roundRect(1040, 170, 610, 480, 22), '$panel', 0.5),
    line(roundRect(1040, 170, 610, 480, 22), '$accent2', 4, 0.9),
    line(`${M(1040, 230)}${L(1040, 190)}${A(20, 20, 0, 0, 1, 1060, 170)}${L(1100, 170)}`, '#ffffff', 6, 0.9),
    line(`${M(1650, 590)}${L(1650, 630)}${A(20, 20, 0, 0, 1, 1630, 650)}${L(1590, 650)}`, '#ffffff', 6, 0.9),
    fill(roundRect(1068, 196, 180, 14, 7), '$accent2', 0.7),
    fill(circle(1618, 204, 8), '$accent'),
  ];
  const floor = [
    fill(rect(0, horizon, W, H - horizon), '$panel'),
    linear(rect(0, horizon, W, H - horizon), [[0, '$accent2'], [1, '$panel']], 0, horizon, 0, H, 0.25),
  ];
  for (let i = -14; i <= 14; i++) floor.push(line(`${M(960 + i * 40, horizon)}${L(960 + i * 360, H)}`, '$grid', 2, 0.55));
  let y = horizon;
  let step = 8;
  while (y < H) {
    floor.push(line(`${M(0, y)}${L(W, y)}`, '$grid', 2, 0.45));
    y += step;
    step *= 1.38;
  }
  floor.push(fill(rect(0, horizon - 2, W, 5), '$accent2', 0.9));
  floor.push(fill(rect(0, horizon - 14, W, 12), '$accent2', 0.18));
  return {
    layers: [{ name: 'sky', parallax: 0, shapes: sky }, { name: 'network', parallax: 0.3, shapes: back }, { name: 'cards', parallax: 0.5, shapes: mid }, { name: 'floor', parallax: 1, shapes: floor }, { name: 'hud', parallax: 0.7, shapes: board }],
    tokens: {
      day: { t1: '#16295f', t2: '#2c58c8', grid: '#7fbaff', panel: '#0e1a47' },
      night: { t1: '#090d2b', t2: '#1b2160', grid: '#4f7dff', panel: '#070b22' },
    },
  };
}

export const ENVIRONMENTS = {
  'plain-stage': plainStage,
  office,
  'home-office': homeOffice,
  'abstract-tech': abstractTech,
};

```


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node --test assets/storymaker/test/environments.test.mjs`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add assets/storymaker/lib/environments.mjs assets/storymaker/test/environments.test.mjs
git commit -m "feat(storymaker): four starter environments" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```


### Task D7: Catalog manifest, thumbnails, locale names, build script and the committed outputs

**Files:**
- Create: `assets/storymaker/lib/names.mjs`, `assets/storymaker/lib/catalog.mjs`, `assets/storymaker/lib/validate.mjs`, `assets/storymaker/lib/review.mjs`
- Create: `scripts/build-storymaker-catalog.mjs`
- Create (generated by the build, committed): `ui/img/storymaker/catalog.json`, `ui/img/storymaker/rig/human.json`, `ui/img/storymaker/props/*.json` (23), `ui/img/storymaker/environments/*.json` (4), `ui/img/storymaker/thumbs/*.svg` (76), `ui/lang/storymaker_catalog/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`
- Test: `assets/storymaker/test/catalog.test.mjs`, `ui/storymaker_catalog_test.go`

Names in 16 locales, the manifest (76 entries, 2 palettes, 4 Geist font faces), thumbnails (deterministic SVG compositions of the same shapes, no rasterising), the version hash, the build-time validator, the review sheets and the build script with `--check`. The Go test pins the `storymaker-catalog/1` contract on the committed files: required IDs, referenced files, parseable path data, known colour tokens, spots and 9:16 / 1:1 overrides, the version hash, the 16 locale files and the asset-set rules (`assets/web-assets.json`: directory `img`/`lang`, extensions `.json` and `.svg`). The loader `internal/i18n/i18n.go` merges every directory below `ui/lang/`, so `ui/lang/storymaker_catalog/` needs no loader change. Part C adds the two package scripts (`build:storymaker-catalog`, `check:storymaker-catalog`); this task runs the script with `node` directly. The expected `version` is `70d5cf51a9a9`.

- [ ] **Step 1: Write the failing test**

`assets/storymaker/test/catalog.test.mjs`:

```js
// Task D7: catalog assembly, the build validator and the build script.
// The schema checks on the committed files live in ui/storymaker_catalog_test.go (task D8).
// Run: node --test assets/storymaker/test/catalog.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildOutputs, versionOf } from '../lib/catalog.mjs';
import { validateOutputs } from '../lib/validate.mjs';
import { NAMES, LANGS } from '../lib/names.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

test('two builds produce identical bytes and the version hash follows the files', () => {
  const a = buildOutputs();
  const b = buildOutputs();
  assert.deepEqual([...a.files.keys()], [...b.files.keys()]);
  for (const [p, text] of a.files) assert.equal(text, b.files.get(p), p);
  assert.equal(a.manifest.version, versionOf(a.files));
  assert.match(a.manifest.version, /^[0-9a-f]{12}$/);
  for (const text of a.files.values()) assert.ok(!text.includes('\r'), 'LF only');
  assert.equal(a.files.size, 105);
});

test('the generated catalog passes the build validator', () => {
  assert.deepEqual(validateOutputs(buildOutputs()), []);
});

test('the validator catches missing ids, bad path data, unknown tokens and a stale version', () => {
  const clone = () => {
    const built = buildOutputs();
    return { ...built, files: new Map(built.files), entries: built.entries.slice() };
  };
  let c = clone();
  c.entries = c.entries.filter((e) => e.id !== 'prop.lock');
  assert.ok(validateOutputs(c).some((p) => p.includes('prop.lock')));

  c = clone();
  const gear = JSON.parse(c.files.get('props/gear.json'));
  gear.shapes[0].d = 'M0 0L10';
  c.files.set('props/gear.json', JSON.stringify(gear));
  assert.ok(validateOutputs(c).some((p) => p.includes('bad path data')));

  c = clone();
  const key = JSON.parse(c.files.get('props/key.json'));
  key.shapes[0].fill = '$nope';
  c.files.set('props/key.json', JSON.stringify(key));
  assert.ok(validateOutputs(c).some((p) => p.includes('unknown token $nope')));

  c = clone();
  c.files.set('props/key.json', c.files.get('props/key.json') + ' ');
  assert.ok(validateOutputs(c).some((p) => p.includes('version')));
});

test('locale rows have 16 real translations for every entry and palette', () => {
  const { manifest } = buildOutputs();
  const ids = [...manifest.entries.map((e) => e.id), ...manifest.palettes.map((p) => p.id)];
  assert.deepEqual(Object.keys(NAMES).sort(), ids.sort());
  for (const row of Object.values(NAMES)) assert.equal(row.length, LANGS.length);
});

test('the manifest follows the contract: stage, rig, fonts, palettes and entry shapes', () => {
  const { manifest, entries } = buildOutputs();
  assert.deepEqual(manifest.stage, { width: 1920, height: 1080, safe: { x: 5, y: 5, w: 90, h: 90 } });
  assert.deepEqual(manifest.rig, { id: 'rig.human/1', file: 'rig/human.json' });
  assert.ok(manifest.fonts.every((f) => f.name === 'default' && f.family === 'Geist' && f.file.startsWith('/fonts/') && f.licence === 'OFL-1.1'));
  assert.deepEqual(manifest.palettes.map((p) => p.id), ['palette.sunny', 'palette.calm']);
  const byId = Object.fromEntries(entries.map((e) => [e.id, e]));
  assert.equal(byId['pose.typing'].name, 'typing');
  assert.equal(byId['pose.typing'].name_key, 'storymaker.catalog.pose_typing');
  assert.equal(byId['char.office-woman-1'].name, 'char.office-woman-1');
  assert.deepEqual(byId['prop.lock'].usage_example, { do: 'show', item: 'prop.lock', at: 'right', entry: 'pop' });
  assert.deepEqual(byId['env.office'].variants, ['day', 'night']);
  assert.equal(byId['cycle.walk'].speed, 16);
  assert.equal(byId['cycle.idle'].speed, undefined);
  assert.equal(entries.length, 76);
});

test('node scripts/build-storymaker-catalog.mjs --check is green on the committed outputs', () => {
  const r = spawnSync(process.execPath, [path.join(root, 'scripts', 'build-storymaker-catalog.mjs'), '--check'], { cwd: root, encoding: 'utf8' });
  assert.equal(r.status, 0, r.stdout + r.stderr);
});
```

`ui/storymaker_catalog_test.go`:

```go
package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The Storymaker starter catalog (ui/img/storymaker) is generated by scripts/build-storymaker-catalog.mjs.
// These tests pin the storymaker-catalog/1 contract on the committed files: required IDs, referenced files,
// parseable path data, known colour tokens, spots and aspect overrides, the version hash, the 16 locale name files
// and the asset-set rules.

const storymakerCatalogDir = "img/storymaker"

var storymakerLangs = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

var storymakerPaletteTokens = []string{"bg", "bg2", "fg", "accent", "accent2", "text", "outline"}
var storymakerCharacterTokens = []string{"skin", "hair", "top", "bottom", "shoes", "outline", "eye_white", "iris", "lip"}

var storymakerRequired = map[string][]string{
	"character":   {"char.office-woman-1", "char.office-man-1", "char.it-tech-1", "char.senior-man-1"},
	"prop":        {"prop.laptop", "prop.phone", "prop.monitor", "prop.server", "prop.router", "prop.cloud", "prop.database", "prop.lock", "prop.key", "prop.shield", "prop.mail", "prop.document", "prop.coffee-cup", "prop.desk", "prop.chair", "prop.plant", "prop.check", "prop.cross", "prop.warning", "prop.idea", "prop.question", "prop.arrow-right", "prop.gear"},
	"environment": {"env.plain-stage", "env.office", "env.home-office", "env.abstract-tech"},
	"pose":        {"pose.stand", "pose.sit", "pose.typing", "pose.point-left", "pose.point-right", "pose.wave", "pose.think", "pose.present", "pose.arms-crossed", "pose.hands-on-hips"},
	"gesture":     {"gesture.point", "gesture.wave", "gesture.shrug", "gesture.think", "gesture.present", "gesture.thumbs-up", "gesture.celebrate", "gesture.facepalm", "gesture.count"},
	"expression":  {"expr.neutral", "expr.happy", "expr.laugh", "expr.sad", "expr.angry", "expr.surprised", "expr.confused", "expr.thinking", "expr.worried", "expr.determined"},
	"mouth":       {"mouth.rest", "mouth.ai", "mouth.e", "mouth.o", "mouth.u", "mouth.mbp", "mouth.fv"},
	"cycle":       {"cycle.walk", "cycle.idle"},
	"text_style":  {"text.title", "text.caption", "text.label"},
	"transition":  {"trans.cut", "trans.fade", "trans.slide-left", "trans.wipe"},
}

var storymakerPrefix = map[string]string{
	"character": "char.", "prop": "prop.", "environment": "env.", "pose": "pose.", "gesture": "gesture.", "expression": "expr.",
	"mouth": "mouth.", "cycle": "cycle.", "text_style": "text.", "transition": "trans.",
}

var storymakerBones = []string{"root", "hips", "torso", "neck", "head", "upper_arm_l", "lower_arm_l", "upper_arm_r", "lower_arm_r", "upper_leg_l", "lower_leg_l", "upper_leg_r", "lower_leg_r"}
var storymakerSlotParams = []string{"body", "skin", "hair", "hair_back", "eyes", "brows", "mouth", "hand", "top", "bottom", "shoes", "glasses", "facial_hair", "accessory"}
var storymakerViews = []string{"front", "three_quarter", "profile"}

type smCatalog struct {
	Schema   string           `json:"schema"`
	Version  string           `json:"version"`
	Stage    map[string]any   `json:"stage"`
	Rig      map[string]any   `json:"rig"`
	Fonts    []map[string]any `json:"fonts"`
	Palettes []smPalette      `json:"palettes"`
	Entries  []map[string]any `json:"entries"`
}

type smPalette struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	NameKey string            `json:"name_key"`
	Tokens  map[string]string `json:"tokens"`
}

func smLoad(t *testing.T) (smCatalog, map[string]any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(storymakerCatalogDir, "catalog.json"))
	if err != nil {
		t.Fatalf("read catalog.json: %v", err)
	}
	var c smCatalog
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("parse catalog.json: %v", err)
	}
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	return c, raw
}

func smReadJSON(t *testing.T, rel string, into any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(storymakerCatalogDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
}

func smStr(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func smNum(m map[string]any, key string) float64 {
	n, _ := m[key].(float64)
	return n
}

func smObj(m map[string]any, key string) map[string]any {
	o, _ := m[key].(map[string]any)
	return o
}

func smHas(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- path data

var (
	smPathToken = regexp.MustCompile(`^[\s,]*(?:([MmLlHhVvCcSsQqTtAaZz])|(-?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?))`)
	smPathArgs  = map[byte]int{'M': 2, 'L': 2, 'H': 1, 'V': 1, 'C': 6, 'S': 4, 'Q': 4, 'T': 2, 'A': 7, 'Z': 0}
)

// smCheckPath validates SVG path data strictly (Path2D silently ignores malformed data).
func smCheckPath(d string) error {
	rest := d
	var cmd byte
	var args []float64
	flush := func() error {
		if cmd == 0 {
			return nil
		}
		need := smPathArgs[cmd&^0x20]
		if need == 0 {
			if len(args) != 0 {
				return fmt.Errorf("%c takes no arguments", cmd)
			}
			return nil
		}
		if len(args) == 0 || len(args)%need != 0 {
			return fmt.Errorf("%c has %d numbers, needs a multiple of %d", cmd, len(args), need)
		}
		if cmd&^0x20 == 'A' {
			for i := 0; i < len(args); i += 7 {
				if (args[i+3] != 0 && args[i+3] != 1) || (args[i+4] != 0 && args[i+4] != 1) {
					return fmt.Errorf("arc flags must be 0 or 1")
				}
			}
		}
		return nil
	}
	if strings.TrimSpace(d) == "" {
		return fmt.Errorf("empty path")
	}
	first := true
	for strings.TrimSpace(rest) != "" {
		m := smPathToken.FindStringSubmatch(rest)
		if m == nil {
			return fmt.Errorf("unexpected text %q", truncateForReport(rest))
		}
		rest = rest[len(m[0]):]
		if m[1] != "" {
			if err := flush(); err != nil {
				return err
			}
			if first && m[1] != "M" && m[1] != "m" {
				return fmt.Errorf("path must start with M")
			}
			first = false
			cmd = m[1][0]
			args = args[:0]
		} else {
			if cmd == 0 {
				return fmt.Errorf("number before a command")
			}
			v, err := strconv.ParseFloat(m[2], 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Errorf("bad number %q", m[2])
			}
			args = append(args, v)
		}
	}
	return flush()
}

var smHexColour = regexp.MustCompile(`^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$`)

func smCheckShapes(t *testing.T, where string, shapes []any, known []string) {
	t.Helper()
	for i, raw := range shapes {
		s, _ := raw.(map[string]any)
		if s == nil {
			t.Errorf("%s[%d]: not an object", where, i)
			return
		}
		if err := smCheckPath(smStr(s, "d")); err != nil {
			t.Errorf("%s[%d]: bad path data: %v", where, i, err)
			return
		}
		colours := []string{}
		if f := smStr(s, "fill"); f != "" {
			colours = append(colours, f)
		}
		if f := smStr(s, "stroke"); f != "" {
			colours = append(colours, f)
		}
		if g := smObj(s, "gradient"); g != nil {
			if typ := smStr(g, "type"); typ != "linear" && typ != "radial" {
				t.Errorf("%s[%d]: gradient type %q", where, i, typ)
			}
			stops, _ := g["stops"].([]any)
			if len(stops) < 2 {
				t.Errorf("%s[%d]: gradient needs two stops", where, i)
			}
			for _, st := range stops {
				pair, _ := st.([]any)
				if len(pair) != 2 {
					t.Errorf("%s[%d]: bad gradient stop", where, i)
					continue
				}
				c, _ := pair[1].(string)
				colours = append(colours, c)
			}
		} else if smStr(s, "fill") == "" && smStr(s, "stroke") == "" {
			t.Errorf("%s[%d]: shape draws nothing", where, i)
		}
		for _, c := range colours {
			if strings.HasPrefix(c, "$") {
				if !smHas(known, c[1:]) {
					t.Errorf("%s[%d]: unknown colour token %s", where, i, c)
					return
				}
			} else if !smHexColour.MatchString(c) {
				t.Errorf("%s[%d]: bad colour %q", where, i, c)
				return
			}
		}
		if op, ok := s["opacity"].(float64); ok && (op < 0 || op > 1) {
			t.Errorf("%s[%d]: opacity %v", where, i, op)
		}
	}
}

func TestStorymakerCatalogPathParser(t *testing.T) {
	good := []string{"M0 0L10 10Z", "M1.5-2L3 4C1 2 3 4 5 6Q1 2 3 4A5 5 0 1 0 10 10z", "M0 0h10v5H0z", "M0 0 10 10 20 0"}
	bad := []string{"", "L1 1", "M0", "M0 0L1", "M0 0A5 5 0 2 0 1 1", "M0 0Zx", "M0 0 1 2 3", "M0 0 L 1 e"}
	for _, d := range good {
		if err := smCheckPath(d); err != nil {
			t.Errorf("%q should parse: %v", d, err)
		}
	}
	for _, d := range bad {
		if err := smCheckPath(d); err == nil {
			t.Errorf("%q should be rejected", d)
		}
	}
}

// ---------------------------------------------------------------- manifest

func TestStorymakerCatalogManifest(t *testing.T) {
	c, raw := smLoad(t)
	if c.Schema != "storymaker-catalog/1" {
		t.Fatalf("schema = %q", c.Schema)
	}
	if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(c.Version) {
		t.Fatalf("version %q is not 12 hex digits", c.Version)
	}
	stage := smObj(raw, "stage")
	safe := smObj(stage, "safe")
	if smNum(stage, "width") != 1920 || smNum(stage, "height") != 1080 || smNum(safe, "x") != 5 || smNum(safe, "y") != 5 || smNum(safe, "w") != 90 || smNum(safe, "h") != 90 {
		t.Errorf("stage = %v", stage)
	}
	if smStr(c.Rig, "id") != "rig.human/1" || smStr(c.Rig, "file") != "rig/human.json" {
		t.Errorf("rig = %v", c.Rig)
	}
	if len(c.Fonts) == 0 {
		t.Fatal("no fonts")
	}
	hasDefault := false
	for _, f := range c.Fonts {
		if smStr(f, "name") == "default" {
			hasDefault = true
		}
		file := smStr(f, "file")
		if !strings.HasPrefix(file, "/fonts/") {
			t.Errorf("font file %q must be an AuraGo /fonts/ path", file)
			continue
		}
		if _, err := os.Stat(filepath.FromSlash("fonts/" + strings.TrimPrefix(file, "/fonts/"))); err != nil {
			t.Errorf("font file %s missing: %v", file, err)
		}
		if smStr(f, "family") == "" || smStr(f, "licence") == "" || smNum(f, "weight") < 100 {
			t.Errorf("font entry incomplete: %v", f)
		}
	}
	if !hasDefault {
		t.Error("no font named default")
	}
	names := map[string]bool{}
	for _, p := range c.Palettes {
		names[p.Name] = true
		if p.ID != "palette."+p.Name || p.NameKey != smNameKey(p.ID) {
			t.Errorf("palette %+v", p)
		}
		for _, tok := range storymakerPaletteTokens {
			if !smHexColour.MatchString(p.Tokens[tok]) {
				t.Errorf("palette %s token %s = %q", p.Name, tok, p.Tokens[tok])
			}
		}
	}
	if !names["sunny"] || !names["calm"] {
		t.Errorf("palettes = %v", names)
	}

	ids := map[string]string{}
	for _, e := range c.Entries {
		id, kind := smStr(e, "id"), smStr(e, "kind")
		if _, dup := ids[id]; dup {
			t.Errorf("duplicate id %s", id)
		}
		ids[id] = kind
		if !strings.HasPrefix(id, storymakerPrefix[kind]) || storymakerPrefix[kind] == "" {
			t.Errorf("%s: prefix does not match kind %q", id, kind)
		}
		if smStr(e, "name_key") != smNameKey(id) {
			t.Errorf("%s: name_key %q", id, smStr(e, "name_key"))
		}
		if smStr(e, "thumb") != "thumbs/"+id+".svg" {
			t.Errorf("%s: thumb %q", id, smStr(e, "thumb"))
		} else if _, err := os.Stat(filepath.Join(storymakerCatalogDir, "thumbs", id+".svg")); err != nil {
			t.Errorf("%s: thumbnail missing", id)
		}
		if tags, _ := e["tags"].([]any); len(tags) == 0 {
			t.Errorf("%s: no tags", id)
		}
		if _, ok := e["usage_example"].(map[string]any); !ok {
			t.Errorf("%s: usage_example must be an object", id)
		}
		// bare names for the enum-like kinds, the id otherwise
		bare := map[string]bool{"pose": true, "gesture": true, "expression": true, "mouth": true, "cycle": true, "text_style": true, "transition": true}
		want := id
		if bare[kind] {
			want = strings.SplitN(id, ".", 2)[1]
		}
		if smStr(e, "name") != want {
			t.Errorf("%s: name %q, want %q", id, smStr(e, "name"), want)
		}
		if file := smStr(e, "file"); file != "" {
			if _, err := os.Stat(filepath.Join(storymakerCatalogDir, filepath.FromSlash(file))); err != nil {
				t.Errorf("%s: file %s missing", id, file)
			}
		}
	}
	for kind, list := range storymakerRequired {
		for _, id := range list {
			if ids[id] != kind {
				t.Errorf("required %s %s missing", kind, id)
			}
		}
	}
	// required fields per kind (what Go's LoadCatalog validates)
	for _, e := range c.Entries {
		id := smStr(e, "id")
		need := map[string][]string{
			"character": {"params"}, "prop": {"file", "size", "anchor"}, "environment": {"file", "spots", "floor", "variants"},
			"pose": {"bones"}, "gesture": {"keys", "duration"}, "expression": {"eyes", "brows", "mouth"}, "mouth": {"shape"},
			"cycle": {"period", "keys"}, "text_style": {"font", "size", "weight", "color", "align", "max_lines"}, "transition": {"duration"},
		}[smStr(e, "kind")]
		for _, f := range need {
			if _, ok := e[f]; !ok {
				t.Errorf("%s: required field %s missing", id, f)
			}
		}
	}
}

func smNameKey(id string) string {
	return "storymaker.catalog." + strings.Replace(id, ".", "_", 1)
}

// ---------------------------------------------------------------- rig

func TestStorymakerCatalogRig(t *testing.T) {
	c, _ := smLoad(t)
	var rig struct {
		ID          string  `json:"id"`
		UnitsHeight float64 `json:"units_height"`
		Height      float64 `json:"height"`
		Bones       []struct {
			Name   string                    `json:"name"`
			Parent *string                   `json:"parent"`
			Length float64                   `json:"length"`
			Rest   map[string]map[string]any `json:"rest"`
		} `json:"bones"`
		Slots []struct {
			Name  string         `json:"name"`
			Bone  string         `json:"bone"`
			Param string         `json:"param"`
			Z     map[string]any `json:"z"`
		} `json:"slots"`
		Variants map[string]map[string]map[string][]any `json:"variants"`
		Bodies   map[string]struct {
			Scale float64 `json:"scale"`
		} `json:"bodies"`
		Params struct {
			Body       []string                  `json:"body"`
			Skin       map[string]string         `json:"skin"`
			Hair       []string                  `json:"hair"`
			HairColor  map[string]string         `json:"hair_color"`
			Eyes       []string                  `json:"eyes"`
			Brows      []string                  `json:"brows"`
			FacialHair []string                  `json:"facial_hair"`
			Glasses    []string                  `json:"glasses"`
			Outfit     map[string]map[string]any `json:"outfit"`
			Accessory  []string                  `json:"accessory"`
			Fixed      map[string]string         `json:"fixed"`
		} `json:"params"`
	}
	smReadJSON(t, "rig/human.json", &rig)
	if rig.ID != "rig.human/1" || rig.UnitsHeight <= 0 || rig.Height <= 0 || rig.Height >= 1 {
		t.Fatalf("rig header: %+v", rig.ID)
	}
	if len(rig.Bones) != len(storymakerBones) {
		t.Fatalf("%d bones", len(rig.Bones))
	}
	for i, b := range rig.Bones {
		if b.Name != storymakerBones[i] {
			t.Errorf("bone %d = %s, want %s", i, b.Name, storymakerBones[i])
		}
		for _, v := range storymakerViews {
			r := b.Rest[v]
			if r == nil {
				t.Errorf("bone %s lacks rest.%s", b.Name, v)
				continue
			}
			for _, k := range []string{"x", "y", "angle"} {
				if _, ok := r[k].(float64); !ok {
					t.Errorf("bone %s rest.%s.%s", b.Name, v, k)
				}
			}
		}
	}
	known := append(append([]string{}, storymakerCharacterTokens...), storymakerPaletteTokens...)
	for _, s := range rig.Slots {
		if !smHas(storymakerBones, s.Bone) {
			t.Errorf("slot %s: unknown bone %s", s.Name, s.Bone)
		}
		if !smHas(storymakerSlotParams, s.Param) {
			t.Errorf("slot %s: unknown param %s", s.Name, s.Param)
		}
		for _, v := range storymakerViews {
			if _, ok := s.Z[v].(float64); !ok {
				t.Errorf("slot %s: z.%s", s.Name, v)
			}
		}
		if len(rig.Variants[s.Name]) == 0 {
			t.Errorf("slot %s has no variants", s.Name)
		}
	}
	for _, slot := range []string{"hand_l", "hand_r", "foot_l", "foot_r"} {
		found := false
		for _, s := range rig.Slots {
			found = found || s.Name == slot
		}
		if !found {
			t.Errorf("end part slot %s missing", slot)
		}
	}
	for slot, keys := range rig.Variants {
		for key, views := range keys {
			for _, v := range storymakerViews {
				shapes, ok := views[v]
				if !ok {
					t.Errorf("variant %s/%s lacks %s", slot, key, v)
					continue
				}
				smCheckShapes(t, fmt.Sprintf("rig %s/%s/%s", slot, key, v), shapes, known)
			}
		}
	}
	// the parameter lists cover every value the starter characters use, and each value has shapes
	for _, name := range rig.Params.Body {
		if _, ok := rig.Bodies[name]; !ok {
			t.Errorf("body %s has no body entry", name)
		}
		if _, ok := rig.Variants["torso"][name]; !ok {
			t.Errorf("body %s has no torso variant", name)
		}
	}
	for _, style := range rig.Params.Hair {
		if _, ok := rig.Variants["hair"][style]; !ok {
			t.Errorf("hair %s has no variants", style)
		}
	}
	for name, o := range rig.Params.Outfit {
		for _, k := range []string{"top", "bottom", "shoes"} {
			if _, ok := o[k].(string); !ok {
				t.Errorf("outfit %s lacks %s", name, k)
			}
		}
		colours, _ := o["colors"].(map[string]any)
		for _, k := range []string{"top", "bottom", "shoes"} {
			if c, _ := colours[k].(string); !smHexColour.MatchString(c) {
				t.Errorf("outfit %s colour %s", name, k)
			}
		}
		if _, ok := rig.Variants["top_det"][smStr(o, "top")]; !ok {
			t.Errorf("outfit %s: top %v has no shapes", name, o["top"])
		}
		if _, ok := rig.Variants["foot_l"][smStr(o, "shoes")]; !ok {
			t.Errorf("outfit %s: shoes %v have no shapes", name, o["shoes"])
		}
	}
	for _, tok := range []string{"outline", "eye_white", "iris", "lip"} {
		if !smHexColour.MatchString(rig.Params.Fixed[tok]) {
			t.Errorf("params.fixed.%s", tok)
		}
	}
	for _, e := range c.Entries {
		switch smStr(e, "kind") {
		case "character":
			p := smObj(e, "params")
			id := smStr(e, "id")
			if !smHas(rig.Params.Body, smStr(p, "body")) || !smHas(rig.Params.Hair, smStr(p, "hair")) || !smHas(rig.Params.Eyes, smStr(p, "eyes")) ||
				!smHas(rig.Params.Brows, smStr(p, "brows")) || !smHas(rig.Params.FacialHair, smStr(p, "facial_hair")) || !smHas(rig.Params.Glasses, smStr(p, "glasses")) ||
				!smHas(rig.Params.Accessory, smStr(p, "accessory")) {
				t.Errorf("%s: params outside the lists: %v", id, p)
			}
			if rig.Params.Skin[smStr(p, "skin")] == "" || rig.Params.HairColor[smStr(p, "hair_color")] == "" || rig.Params.Outfit[smStr(p, "outfit")] == nil {
				t.Errorf("%s: unknown skin, hair colour or outfit", id)
			}
			// every expression of the catalog resolves to variants for this character's eyes and brows
			for _, ex := range c.Entries {
				if smStr(ex, "kind") != "expression" {
					continue
				}
				if _, ok := rig.Variants["eyes"][smStr(p, "eyes")+"/"+smStr(ex, "eyes")]; !ok {
					t.Errorf("%s: no eyes variant for %s", id, smStr(ex, "id"))
				}
				if _, ok := rig.Variants["brows"][smStr(p, "brows")+"/"+smStr(ex, "brows")]; !ok {
					t.Errorf("%s: no brows variant for %s", id, smStr(ex, "id"))
				}
				if _, ok := rig.Variants["mouth"][smStr(ex, "mouth")]; !ok {
					t.Errorf("no mouth variant %s", smStr(ex, "mouth"))
				}
			}
		case "mouth":
			if _, ok := rig.Variants["mouth"][smStr(e, "shape")]; !ok {
				t.Errorf("%s: rig has no mouth %s", smStr(e, "id"), smStr(e, "shape"))
			}
		case "pose":
			smCheckBones(t, smStr(e, "id"), smObj(e, "bones"))
		case "gesture", "cycle":
			keys, _ := e["keys"].([]any)
			if len(keys) < 2 {
				t.Errorf("%s: needs at least two keys", smStr(e, "id"))
				continue
			}
			first, _ := keys[0].(map[string]any)
			last, _ := keys[len(keys)-1].(map[string]any)
			if smNum(first, "t") != 0 || smNum(last, "t") != 1 {
				t.Errorf("%s: keys must run from t=0 to t=1", smStr(e, "id"))
			}
			prev := -1.0
			for _, k := range keys {
				key, _ := k.(map[string]any)
				if smNum(key, "t") < prev {
					t.Errorf("%s: keys out of order", smStr(e, "id"))
				}
				prev = smNum(key, "t")
				smCheckBones(t, smStr(e, "id"), smObj(key, "bones"))
			}
			if smStr(e, "kind") == "gesture" {
				a, _ := json.Marshal(first["bones"])
				b, _ := json.Marshal(last["bones"])
				if !bytes.Equal(a, b) {
					t.Errorf("%s: a gesture must return to its first key", smStr(e, "id"))
				}
			}
		}
	}
}

func smCheckBones(t *testing.T, id string, bones map[string]any) {
	t.Helper()
	for b, v := range bones {
		if !smHas(storymakerBones, b) {
			t.Errorf("%s: unknown bone %s", id, b)
		}
		if _, ok := v.(float64); !ok {
			t.Errorf("%s: bone %s angle %v", id, b, v)
		}
	}
}

// ---------------------------------------------------------------- props and environments

func TestStorymakerCatalogPropsAndEnvironments(t *testing.T) {
	c, _ := smLoad(t)
	for _, e := range c.Entries {
		id := smStr(e, "id")
		switch smStr(e, "kind") {
		case "prop":
			var f struct {
				Viewbox []float64 `json:"viewbox"`
				Shapes  []any     `json:"shapes"`
			}
			smReadJSON(t, smStr(e, "file"), &f)
			if len(f.Viewbox) != 4 || f.Viewbox[2] <= 0 || f.Viewbox[3] <= 0 || len(f.Shapes) == 0 {
				t.Errorf("%s: viewbox %v, %d shapes", id, f.Viewbox, len(f.Shapes))
			}
			smCheckShapes(t, smStr(e, "file"), f.Shapes, storymakerPaletteTokens)
			if h := smNum(smObj(e, "size"), "h"); h <= 0 || h >= 1 {
				t.Errorf("%s: size.h %v", id, h)
			}
			a := smObj(e, "anchor")
			if smNum(a, "x") < 0 || smNum(a, "x") > 1 || smNum(a, "y") < 0 || smNum(a, "y") > 1 {
				t.Errorf("%s: anchor %v", id, a)
			}
		case "environment":
			smCheckEnvironment(t, e)
		}
	}
}

func smCheckEnvironment(t *testing.T, e map[string]any) {
	t.Helper()
	id := smStr(e, "id")
	var f struct {
		Viewbox []float64 `json:"viewbox"`
		Layers  []struct {
			Name     string  `json:"name"`
			Parallax float64 `json:"parallax"`
			Shapes   []any   `json:"shapes"`
		} `json:"layers"`
		Tokens map[string]map[string]string `json:"tokens"`
	}
	smReadJSON(t, smStr(e, "file"), &f)
	if len(f.Viewbox) != 4 || f.Viewbox[2] != 1920 || f.Viewbox[3] != 1080 {
		t.Errorf("%s: viewbox %v", id, f.Viewbox)
	}
	if len(f.Layers) < 3 {
		t.Errorf("%s: needs parallax layers", id)
	}
	known := append([]string{}, storymakerPaletteTokens...)
	for k := range f.Tokens["day"] {
		known = append(known, k)
	}
	for k := range f.Tokens["day"] {
		if _, ok := f.Tokens["night"][k]; !ok {
			t.Errorf("%s: token %s has no night value", id, k)
		}
	}
	for k, v := range f.Tokens["night"] {
		if _, ok := f.Tokens["day"][k]; !ok && !smHas(storymakerPaletteTokens, k) {
			t.Errorf("%s: night token %s has no day value", id, k)
		}
		if !smHexColour.MatchString(v) {
			t.Errorf("%s: night token %s = %q", id, k, v)
		}
	}
	for _, l := range f.Layers {
		if l.Parallax < 0 || l.Parallax > 1 || len(l.Shapes) == 0 {
			t.Errorf("%s: layer %s", id, l.Name)
		}
		smCheckShapes(t, smStr(e, "file")+" layer "+l.Name, l.Shapes, known)
	}
	spots := smObj(e, "spots")
	want := []string{"far-left", "left", "center", "right", "far-right"}
	switch id {
	case "env.plain-stage", "env.abstract-tech":
		want = append(want, "board")
	case "env.office":
		want = append(want, "desk", "door", "board", "window")
	case "env.home-office":
		want = append(want, "desk", "door", "window")
	}
	for _, s := range want {
		if spots[s] == nil {
			t.Errorf("%s: spot %s missing", id, s)
		}
	}
	for name, raw := range spots {
		sp, _ := raw.(map[string]any)
		if smNum(sp, "x") < 0 || smNum(sp, "x") > 100 || smNum(sp, "y") < 0 || smNum(sp, "y") > 100 {
			t.Errorf("%s: spot %s out of range", id, name)
		}
		if face := smStr(sp, "face"); face != "" && face != "left" && face != "right" {
			t.Errorf("%s: spot %s face %q", id, name, face)
		}
	}
	fl := smObj(e, "floor")
	if !(smNum(fl, "y_far") < smNum(fl, "y_near") && smNum(fl, "scale_far") < smNum(fl, "scale_near") && smNum(fl, "scale_far") > 0) {
		t.Errorf("%s: floor %v", id, fl)
	}
	if v, _ := json.Marshal(e["variants"]); string(v) != `["day","night"]` {
		t.Errorf("%s: variants %s", id, v)
	}
	// 9:16 and 1:1 show a slice of the 16:9 design frame as wide as the output is at the design height;
	// every spot (with its override) must sit inside that slice with a small margin
	aspects := smObj(e, "aspects")
	for asp, half := range map[string]float64{"9:16": 100 * (1080.0 * 9 / 16) / 1920 / 2, "1:1": 100 * 1080.0 / 1920 / 2} {
		a := smObj(aspects, asp)
		if a == nil || a["focus_x"] == nil {
			t.Errorf("%s: aspect %s override missing", id, asp)
			continue
		}
		focus := smNum(a, "focus_x")
		over := smObj(a, "spots")
		for name, raw := range spots {
			sp, _ := raw.(map[string]any)
			if o := smObj(over, name); o != nil {
				sp = o
			}
			if x := smNum(sp, "x"); x < focus-half+3 || x > focus+half-3 {
				t.Errorf("%s: spot %s (x=%v) lies outside the %s crop around %v", id, name, x, asp, focus)
			}
		}
		for name := range over {
			if spots[name] == nil {
				t.Errorf("%s: %s overrides unknown spot %s", id, asp, name)
			}
		}
	}
}

// ---------------------------------------------------------------- files, version, thumbnails, budgets

func smLF(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

func TestStorymakerCatalogFilesVersionAndBudget(t *testing.T) {
	c, _ := smLoad(t)
	var paths []string
	var total int64
	err := filepath.WalkDir(storymakerCatalogDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(storymakerCatalogDir, p)
		rel = filepath.ToSlash(rel)
		info, _ := d.Info()
		total += info.Size()
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	if len(paths) > 2000 || total > 30<<20 {
		t.Errorf("catalog budget exceeded: %d files, %d bytes (limit 2000 files, 30 MiB)", len(paths), total)
	}
	h := sha256.New()
	for _, p := range paths {
		if p == "catalog.json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(storymakerCatalogDir, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		h.Write(smLF(data))
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".json" && ext != ".svg" {
			t.Errorf("%s: extension %s is not .json or .svg", p, ext)
		}
	}
	if got := hex.EncodeToString(h.Sum(nil))[:12]; got != c.Version {
		t.Errorf("catalog version %s, but the files hash to %s: run node scripts/build-storymaker-catalog.mjs", c.Version, got)
	}
	// every file below the catalog directory is referenced by the manifest
	referenced := map[string]bool{"catalog.json": true, smStr(c.Rig, "file"): true}
	for _, e := range c.Entries {
		referenced[smStr(e, "thumb")] = true
		if f := smStr(e, "file"); f != "" {
			referenced[f] = true
		}
	}
	for _, p := range paths {
		if !referenced[p] {
			t.Errorf("%s is not referenced by catalog.json", p)
		}
	}
	svgTag := regexp.MustCompile(`(?i)<script|<image|<foreignObject|\shref=|xlink:|url\(\s*['"]?(?:https?:)?//`)
	for _, e := range c.Entries {
		data, err := os.ReadFile(filepath.Join(storymakerCatalogDir, filepath.FromSlash(smStr(e, "thumb"))))
		if err != nil {
			continue
		}
		text := string(data)
		if !strings.HasPrefix(text, "<svg xmlns=\"http://www.w3.org/2000/svg\" ") || svgTag.MatchString(text) {
			t.Errorf("%s: thumbnail is not a plain self-contained svg", smStr(e, "id"))
		}
		for _, m := range regexp.MustCompile(` d="([^"]*)"`).FindAllStringSubmatch(text, -1) {
			if err := smCheckPath(m[1]); err != nil {
				t.Errorf("%s: thumbnail path: %v", smStr(e, "id"), err)
				break
			}
		}
		if strings.Contains(strings.ReplaceAll(text, "http://www.w3.org/2000/svg", ""), "http") {
			t.Errorf("%s: thumbnail references an external URL", smStr(e, "id"))
		}
	}
}

// The asset-set build drops files that are outside an allowlisted directory or have an allowlisted extension
// missing (assets/web-assets.json, cmd/assetpack/main.go), so the catalog must stay inside it.
func TestStorymakerCatalogFitsTheAssetSet(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "assets", "web-assets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Roots []struct {
			Source      string   `json:"source"`
			Directories []string `json:"directories"`
			Extensions  []string `json:"extensions"`
		} `json:"roots"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, r := range cfg.Roots {
		if r.Source != "ui" {
			continue
		}
		for _, dir := range []string{"img", "lang"} {
			if !smHas(r.Directories, dir) {
				t.Errorf("web-assets.json ui root does not ship the %s directory", dir)
			}
		}
		for _, ext := range []string{".json", ".svg"} {
			if !smHas(r.Extensions, ext) {
				t.Errorf("web-assets.json ui root does not ship %s files", ext)
			}
		}
		return
	}
	t.Fatal("no ui root in assets/web-assets.json")
}

// ---------------------------------------------------------------- locale names

func TestStorymakerCatalogLocaleNames(t *testing.T) {
	c, _ := smLoad(t)
	var keys []string
	for _, p := range c.Palettes {
		keys = append(keys, p.NameKey)
	}
	for _, e := range c.Entries {
		keys = append(keys, smStr(e, "name_key"))
	}
	tables := map[string]map[string]string{}
	for _, lang := range storymakerLangs {
		path := filepath.Join("lang", "storymaker_catalog", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		tables[lang] = m
		for _, k := range keys {
			if strings.TrimSpace(m[k]) == "" {
				t.Errorf("%s: missing %s", lang, k)
			}
		}
		for k := range m {
			if !strings.HasPrefix(k, "storymaker.catalog.") {
				t.Errorf("%s: key %s is outside the reserved storymaker.catalog. prefix", lang, k)
			}
			if !smHas(keys, k) {
				t.Errorf("%s: key %s names no catalog entry", lang, k)
			}
		}
	}
	// real translations, not English copies: most names differ from English in every other language
	for _, lang := range storymakerLangs {
		if lang == "en" {
			continue
		}
		same := 0
		for _, k := range keys {
			if tables[lang][k] == tables["en"][k] {
				same++
			}
		}
		if float64(same) > 0.35*float64(len(keys)) {
			t.Errorf("%s: %d of %d names equal the English name", lang, same, len(keys))
		}
	}
	if tables["de"]["storymaker.catalog.prop_lock"] != "Schloss" || tables["de"]["storymaker.catalog.env_office"] != "Büro" {
		t.Errorf("German names look wrong: %q %q", tables["de"]["storymaker.catalog.prop_lock"], tables["de"]["storymaker.catalog.env_office"])
	}
}
```


- [ ] **Step 2: Run the test to verify it fails**

Run: `node --test assets/storymaker/test/catalog.test.mjs`
     `go test ./ui -run TestStorymakerCatalog -count=1`
Expected: FAIL (node: `ERR_MODULE_NOT_FOUND` for `assets/storymaker/lib/catalog.mjs`; go: `read catalog.json: ... no such file or directory`)

- [ ] **Step 3: Implement**

`assets/storymaker/lib/names.mjs`:

```js
// Display names of the catalog entries in the 16 desktop locales.
// Order of every row: cs da de el en es fr hi it ja nl no pl pt sv zh
// The build script writes them to ui/lang/storymaker_catalog/<lang>.json as storymaker.catalog.<id with "." -> "_">.

export const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];

export const NAMES = {
  // characters
  'char.office-woman-1': ['Kancelářská pracovnice', 'Kontoransat (kvinde)', 'Büroangestellte', 'Υπάλληλος γραφείου (γυναίκα)', 'Office woman', 'Oficinista (mujer)', 'Employée de bureau', 'ऑफिस कर्मचारी (महिला)', 'Impiegata', 'オフィスの女性', 'Kantoormedewerkster', 'Kontordame', 'Pracownica biurowa', 'Funcionária de escritório', 'Kontorsanställd (kvinna)', '办公室女职员'],
  'char.office-man-1': ['Kancelářský pracovník', 'Kontoransat (mand)', 'Büroangestellter', 'Υπάλληλος γραφείου (άνδρας)', 'Office man', 'Oficinista (hombre)', 'Employé de bureau', 'ऑफिस कर्मचारी (पुरुष)', 'Impiegato', 'オフィスの男性', 'Kantoormedewerker', 'Kontormann', 'Pracownik biurowy', 'Funcionário de escritório', 'Kontorsanställd (man)', '办公室男职员'],
  'char.it-tech-1': ['IT technik', 'IT-tekniker', 'IT-Techniker', 'Τεχνικός πληροφορικής', 'IT technician', 'Técnico informático', 'Technicien informatique', 'आईटी तकनीशियन', 'Tecnico informatico', 'ITエンジニア', 'IT-technicus', 'IT-tekniker', 'Technik IT', 'Técnico de TI', 'IT-tekniker', 'IT 技术员'],
  'char.senior-man-1': ['Starší muž', 'Ældre mand', 'Älterer Herr', 'Ηλικιωμένος άνδρας', 'Senior man', 'Hombre mayor', 'Homme âgé', 'बुज़ुर्ग पुरुष', 'Uomo anziano', '年配の男性', 'Oudere man', 'Eldre mann', 'Starszy mężczyzna', 'Homem idoso', 'Äldre man', '年长男士'],
  // props
  'prop.laptop': ['Notebook', 'Bærbar computer', 'Laptop', 'Φορητός υπολογιστής', 'Laptop', 'Portátil', 'Ordinateur portable', 'लैपटॉप', 'Portatile', 'ノートパソコン', 'Laptop', 'Bærbar PC', 'Laptop', 'Portátil', 'Bärbar dator', '笔记本电脑'],
  'prop.phone': ['Mobilní telefon', 'Mobiltelefon', 'Smartphone', 'Κινητό τηλέφωνο', 'Phone', 'Teléfono móvil', 'Téléphone', 'फ़ोन', 'Telefono', 'スマートフォン', 'Telefoon', 'Mobiltelefon', 'Telefon', 'Telefone', 'Mobiltelefon', '手机'],
  'prop.monitor': ['Monitor', 'Skærm', 'Monitor', 'Οθόνη', 'Monitor', 'Monitor', 'Écran', 'मॉनिटर', 'Monitor', 'モニター', 'Monitor', 'Skjerm', 'Monitor', 'Monitor', 'Skärm', '显示器'],
  'prop.server': ['Server', 'Server', 'Server', 'Διακομιστής', 'Server', 'Servidor', 'Serveur', 'सर्वर', 'Server', 'サーバー', 'Server', 'Server', 'Serwer', 'Servidor', 'Server', '服务器'],
  'prop.router': ['Router', 'Router', 'Router', 'Δρομολογητής', 'Router', 'Router', 'Routeur', 'राउटर', 'Router', 'ルーター', 'Router', 'Ruter', 'Router', 'Router', 'Router', '路由器'],
  'prop.cloud': ['Oblak', 'Sky', 'Wolke', 'Σύννεφο', 'Cloud', 'Nube', 'Nuage', 'क्लाउड', 'Nuvola', 'クラウド', 'Wolk', 'Sky', 'Chmura', 'Nuvem', 'Moln', '云'],
  'prop.database': ['Databáze', 'Database', 'Datenbank', 'Βάση δεδομένων', 'Database', 'Base de datos', 'Base de données', 'डेटाबेस', 'Database', 'データベース', 'Database', 'Database', 'Baza danych', 'Base de dados', 'Databas', '数据库'],
  'prop.lock': ['Zámek', 'Lås', 'Schloss', 'Λουκέτο', 'Lock', 'Candado', 'Cadenas', 'ताला', 'Lucchetto', '南京錠', 'Slot', 'Lås', 'Kłódka', 'Cadeado', 'Lås', '挂锁'],
  'prop.key': ['Klíč', 'Nøgle', 'Schlüssel', 'Κλειδί', 'Key', 'Llave', 'Clé', 'चाबी', 'Chiave', '鍵', 'Sleutel', 'Nøkkel', 'Klucz', 'Chave', 'Nyckel', '钥匙'],
  'prop.shield': ['Štít', 'Skjold', 'Schild', 'Ασπίδα', 'Shield', 'Escudo', 'Bouclier', 'ढाल', 'Scudo', 'シールド', 'Schild', 'Skjold', 'Tarcza', 'Escudo', 'Sköld', '盾牌'],
  'prop.mail': ['E-mail', 'E-mail', 'E-Mail', 'Email', 'Mail', 'Correo', 'Courriel', 'ईमेल', 'Email', 'メール', 'E-mail', 'E-post', 'E-mail', 'E-mail', 'E-post', '邮件'],
  'prop.document': ['Dokument', 'Dokument', 'Dokument', 'Έγγραφο', 'Document', 'Documento', 'Document', 'दस्तावेज़', 'Documento', '書類', 'Document', 'Dokument', 'Dokument', 'Documento', 'Dokument', '文档'],
  'prop.coffee-cup': ['Šálek kávy', 'Kaffekop', 'Kaffeetasse', 'Φλιτζάνι καφέ', 'Coffee cup', 'Taza de café', 'Tasse de café', 'कॉफ़ी का कप', 'Tazza di caffè', 'コーヒーカップ', 'Koffiekopje', 'Kaffekopp', 'Filiżanka kawy', 'Chávena de café', 'Kaffekopp', '咖啡杯'],
  'prop.desk': ['Psací stůl', 'Skrivebord', 'Schreibtisch', 'Γραφείο', 'Desk', 'Escritorio', 'Bureau', 'डेस्क', 'Scrivania', 'デスク', 'Bureau', 'Skrivebord', 'Biurko', 'Secretária', 'Skrivbord', '办公桌'],
  'prop.chair': ['Židle', 'Stol', 'Stuhl', 'Καρέκλα', 'Chair', 'Silla', 'Chaise', 'कुर्सी', 'Sedia', '椅子', 'Stoel', 'Stol', 'Krzesło', 'Cadeira', 'Stol', '椅子'],
  'prop.plant': ['Rostlina', 'Plante', 'Pflanze', 'Φυτό', 'Plant', 'Planta', 'Plante', 'पौधा', 'Pianta', '観葉植物', 'Plant', 'Plante', 'Roślina', 'Planta', 'Växt', '绿植'],
  'prop.check': ['Zaškrtnutí', 'Flueben', 'Haken', 'Σημάδι επιλογής', 'Check mark', 'Marca de verificación', 'Coche', 'सही का निशान', 'Segno di spunta', 'チェックマーク', 'Vinkje', 'Hake', 'Znacznik wyboru', 'Visto', 'Bock', '对勾'],
  'prop.cross': ['Křížek', 'Kryds', 'Kreuz', 'Σταυρός', 'Cross mark', 'Cruz', 'Croix', 'क्रॉस का निशान', 'Croce', 'バツ印', 'Kruisje', 'Kryss', 'Krzyżyk', 'Cruz', 'Kryss', '叉号'],
  'prop.warning': ['Varování', 'Advarsel', 'Warnung', 'Προειδοποίηση', 'Warning', 'Advertencia', 'Avertissement', 'चेतावनी', 'Avviso', '警告', 'Waarschuwing', 'Advarsel', 'Ostrzeżenie', 'Aviso', 'Varning', '警告'],
  'prop.idea': ['Nápad', 'Idé', 'Idee', 'Ιδέα', 'Idea', 'Idea', 'Idée', 'विचार', 'Idea', 'アイデア', 'Idee', 'Idé', 'Pomysł', 'Ideia', 'Idé', '点子'],
  'prop.question': ['Otazník', 'Spørgsmål', 'Frage', 'Ερώτηση', 'Question', 'Pregunta', 'Question', 'प्रश्न', 'Domanda', '質問', 'Vraag', 'Spørsmål', 'Pytanie', 'Pergunta', 'Fråga', '问题'],
  'prop.arrow-right': ['Šipka doprava', 'Pil mod højre', 'Pfeil nach rechts', 'Βέλος δεξιά', 'Arrow right', 'Flecha a la derecha', 'Flèche vers la droite', 'दायाँ तीर', 'Freccia a destra', '右矢印', 'Pijl naar rechts', 'Pil mot høyre', 'Strzałka w prawo', 'Seta para a direita', 'Pil åt höger', '右箭头'],
  'prop.gear': ['Ozubené kolo', 'Tandhjul', 'Zahnrad', 'Γρανάζι', 'Gear', 'Engranaje', 'Engrenage', 'गियर', 'Ingranaggio', '歯車', 'Tandwiel', 'Tannhjul', 'Koło zębate', 'Engrenagem', 'Kugghjul', '齿轮'],
  // environments
  'env.plain-stage': ['Prázdné pódium', 'Enkel scene', 'Einfache Bühne', 'Απλή σκηνή', 'Plain stage', 'Escenario liso', 'Scène simple', 'सादा मंच', 'Palco semplice', 'シンプルなステージ', 'Eenvoudig podium', 'Enkel scene', 'Prosta scena', 'Palco simples', 'Enkel scen', '纯色舞台'],
  'env.office': ['Kancelář', 'Kontor', 'Büro', 'Γραφείο', 'Office', 'Oficina', 'Bureau', 'ऑफिस', 'Ufficio', 'オフィス', 'Kantoor', 'Kontor', 'Biuro', 'Escritório', 'Kontor', '办公室'],
  'env.home-office': ['Domácí kancelář', 'Hjemmekontor', 'Homeoffice', 'Γραφείο στο σπίτι', 'Home office', 'Oficina en casa', 'Bureau à domicile', 'होम ऑफिस', 'Ufficio in casa', '在宅オフィス', 'Thuiskantoor', 'Hjemmekontor', 'Biuro domowe', 'Escritório em casa', 'Hemmakontor', '家庭办公室'],
  'env.abstract-tech': ['Abstraktní technologie', 'Abstrakt teknologi', 'Abstrakte Technik', 'Αφηρημένη τεχνολογία', 'Abstract tech', 'Tecnología abstracta', 'Technologie abstraite', 'अमूर्त तकनीक', 'Tecnologia astratta', '抽象的なテック背景', 'Abstracte tech', 'Abstrakt teknologi', 'Abstrakcyjna technologia', 'Tecnologia abstrata', 'Abstrakt teknik', '抽象科技背景'],
  // poses
  'pose.stand': ['Stoj', 'Stående', 'Stehen', 'Όρθια στάση', 'Stand', 'De pie', 'Debout', 'खड़े होना', 'In piedi', '立つ', 'Staan', 'Stå', 'Stanie', 'De pé', 'Stå', '站立'],
  'pose.sit': ['Sed', 'Sidde', 'Sitzen', 'Καθιστή στάση', 'Sit', 'Sentado', 'Assis', 'बैठना', 'Seduto', '座る', 'Zitten', 'Sitte', 'Siedzenie', 'Sentado', 'Sitta', '坐着'],
  'pose.typing': ['Psaní', 'Skrive på computer', 'Tippen', 'Πληκτρολόγηση', 'Typing', 'Escribiendo', 'Taper au clavier', 'टाइप करना', 'Digitare', 'タイピング', 'Typen', 'Skrive', 'Pisanie na klawiaturze', 'A escrever', 'Skriva', '打字'],
  'pose.point-left': ['Ukazuje vlevo', 'Peger til venstre', 'Zeigt nach links', 'Δείχνει αριστερά', 'Point left', 'Señala a la izquierda', 'Montre à gauche', 'बाईं ओर इशारा', 'Indica a sinistra', '左を指す', 'Wijst naar links', 'Peker til venstre', 'Wskazuje w lewo', 'Aponta para a esquerda', 'Pekar åt vänster', '指向左侧'],
  'pose.point-right': ['Ukazuje vpravo', 'Peger til højre', 'Zeigt nach rechts', 'Δείχνει δεξιά', 'Point right', 'Señala a la derecha', 'Montre à droite', 'दाईं ओर इशारा', 'Indica a destra', '右を指す', 'Wijst naar rechts', 'Peker til høyre', 'Wskazuje w prawo', 'Aponta para a direita', 'Pekar åt höger', '指向右侧'],
  'pose.wave': ['Mává', 'Vinker', 'Winken', 'Χαιρετισμός', 'Wave', 'Saludar', 'Saluer', 'हाथ हिलाना', 'Salutare', '手を振る', 'Zwaaien', 'Vinke', 'Machanie', 'Acenar', 'Vinka', '挥手'],
  'pose.think': ['Přemýšlí', 'Tænker', 'Nachdenken', 'Σκέψη', 'Think', 'Pensar', 'Réfléchir', 'सोचना', 'Pensare', '考える', 'Nadenken', 'Tenke', 'Myślenie', 'Pensar', 'Fundera', '思考'],
  'pose.present': ['Prezentuje', 'Præsenterer', 'Präsentieren', 'Παρουσίαση', 'Present', 'Presentar', 'Présenter', 'प्रस्तुत करना', 'Presentare', '紹介する', 'Presenteren', 'Presentere', 'Prezentowanie', 'Apresentar', 'Presentera', '展示'],
  'pose.arms-crossed': ['Zkřížené ruce', 'Korslagte arme', 'Verschränkte Arme', 'Σταυρωμένα χέρια', 'Arms crossed', 'Brazos cruzados', 'Bras croisés', 'हाथ बाँधे हुए', 'Braccia incrociate', '腕組み', 'Armen over elkaar', 'Armer i kors', 'Skrzyżowane ręce', 'Braços cruzados', 'Korslagda armar', '双臂交叉'],
  'pose.hands-on-hips': ['Ruce v bok', 'Hænderne i siden', 'Hände in die Hüften', 'Χέρια στη μέση', 'Hands on hips', 'Manos en la cintura', 'Mains sur les hanches', 'कमर पर हाथ', 'Mani sui fianchi', '腰に手を当てる', 'Handen in de zij', 'Hendene i siden', 'Ręce na biodrach', 'Mãos na cintura', 'Händerna i sidan', '双手叉腰'],
  // gestures
  'gesture.point': ['Ukázat', 'Pege', 'Zeigen', 'Δείχνω', 'Point', 'Señalar', 'Montrer', 'इशारा करना', 'Indicare', '指差し', 'Wijzen', 'Peke', 'Wskazać', 'Apontar', 'Peka', '指向'],
  'gesture.wave': ['Mávnout', 'Vinke', 'Winken', 'Χαιρετώ', 'Wave', 'Saludar', 'Saluer', 'हाथ हिलाना', 'Salutare', '手を振る', 'Zwaaien', 'Vinke', 'Pomachać', 'Acenar', 'Vinka', '挥手'],
  'gesture.shrug': ['Pokrčit rameny', 'Trække på skuldrene', 'Schulterzucken', 'Σήκωμα ώμων', 'Shrug', 'Encogerse de hombros', 'Hausser les épaules', 'कंधे उचकाना', 'Alzare le spalle', '肩をすくめる', 'Schouderophalen', 'Trekke på skuldrene', 'Wzruszenie ramionami', 'Encolher os ombros', 'Rycka på axlarna', '耸肩'],
  'gesture.think': ['Zamyslet se', 'Tænke', 'Nachdenken', 'Σκέφτομαι', 'Think', 'Pensar', 'Réfléchir', 'सोचना', 'Pensare', '考え込む', 'Nadenken', 'Tenke', 'Zamyślić się', 'Pensar', 'Fundera', '思考'],
  'gesture.present': ['Představit', 'Præsentere', 'Präsentieren', 'Παρουσιάζω', 'Present', 'Presentar', 'Présenter', 'प्रस्तुत करना', 'Presentare', '手で示す', 'Presenteren', 'Presentere', 'Zaprezentować', 'Apresentar', 'Presentera', '展示'],
  'gesture.thumbs-up': ['Palec nahoru', 'Tommelfinger op', 'Daumen hoch', 'Μπράβο', 'Thumbs up', 'Pulgar hacia arriba', 'Pouce levé', 'अंगूठा ऊपर', 'Pollice in su', 'サムズアップ', 'Duim omhoog', 'Tommel opp', 'Kciuk w górę', 'Polegar para cima', 'Tummen upp', '竖大拇指'],
  'gesture.celebrate': ['Oslavovat', 'Fejre', 'Jubeln', 'Πανηγυρίζω', 'Celebrate', 'Celebrar', 'Célébrer', 'जश्न मनाना', 'Festeggiare', '喜ぶ', 'Vieren', 'Feire', 'Świętować', 'Celebrar', 'Fira', '庆祝'],
  'gesture.facepalm': ['Facepalm', 'Facepalm', 'Facepalm', 'Facepalm', 'Facepalm', 'Facepalm', 'Facepalm', 'फेसपाम', 'Facepalm', '顔に手を当てる', 'Facepalm', 'Facepalm', 'Facepalm', 'Facepalm', 'Facepalm', '捂脸'],
  'gesture.count': ['Počítat', 'Tælle', 'Zählen', 'Μέτρημα', 'Count', 'Contar', 'Compter', 'गिनना', 'Contare', '数える', 'Tellen', 'Telle', 'Liczyć', 'Contar', 'Räkna', '数数'],
  // expressions
  'expr.neutral': ['Neutrální', 'Neutral', 'Neutral', 'Ουδέτερη', 'Neutral', 'Neutral', 'Neutre', 'सामान्य', 'Neutra', '無表情', 'Neutraal', 'Nøytral', 'Neutralna', 'Neutra', 'Neutral', '平静'],
  'expr.happy': ['Šťastný', 'Glad', 'Fröhlich', 'Χαρούμενος', 'Happy', 'Feliz', 'Heureux', 'खुश', 'Felice', 'うれしい', 'Blij', 'Glad', 'Szczęśliwy', 'Feliz', 'Glad', '开心'],
  'expr.laugh': ['Smích', 'Grin', 'Lachen', 'Γέλιο', 'Laugh', 'Risa', 'Rire', 'हँसी', 'Risata', '笑う', 'Lachen', 'Latter', 'Śmiech', 'Riso', 'Skratt', '大笑'],
  'expr.sad': ['Smutný', 'Trist', 'Traurig', 'Λυπημένος', 'Sad', 'Triste', 'Triste', 'उदास', 'Triste', '悲しい', 'Verdrietig', 'Trist', 'Smutny', 'Triste', 'Ledsen', '难过'],
  'expr.angry': ['Naštvaný', 'Vred', 'Wütend', 'Θυμωμένος', 'Angry', 'Enfadado', 'En colère', 'गुस्से में', 'Arrabbiato', '怒っている', 'Boos', 'Sint', 'Zły', 'Zangado', 'Arg', '生气'],
  'expr.surprised': ['Překvapený', 'Overrasket', 'Überrascht', 'Έκπληκτος', 'Surprised', 'Sorprendido', 'Surpris', 'हैरान', 'Sorpreso', '驚いた', 'Verrast', 'Overrasket', 'Zaskoczony', 'Surpreendido', 'Förvånad', '惊讶'],
  'expr.confused': ['Zmatený', 'Forvirret', 'Verwirrt', 'Μπερδεμένος', 'Confused', 'Confundido', 'Perplexe', 'उलझन में', 'Confuso', '困惑', 'Verward', 'Forvirret', 'Zdezorientowany', 'Confuso', 'Förvirrad', '困惑'],
  'expr.thinking': ['Přemýšlející', 'Tænkende', 'Nachdenklich', 'Σκεπτικός', 'Thinking', 'Pensativo', 'Pensif', 'सोच में', 'Pensieroso', '考え中', 'Nadenkend', 'Tenkende', 'Zamyślony', 'Pensativo', 'Tänkande', '思考中'],
  'expr.worried': ['Ustaraný', 'Bekymret', 'Besorgt', 'Ανήσυχος', 'Worried', 'Preocupado', 'Inquiet', 'चिंतित', 'Preoccupato', '心配', 'Bezorgd', 'Bekymret', 'Zmartwiony', 'Preocupado', 'Orolig', '担心'],
  'expr.determined': ['Odhodlaný', 'Fast besluttet', 'Entschlossen', 'Αποφασισμένος', 'Determined', 'Decidido', 'Déterminé', 'दृढ़ निश्चयी', 'Determinato', '決意', 'Vastberaden', 'Bestemt', 'Zdeterminowany', 'Determinado', 'Beslutsam', '坚定'],
  // mouth shapes
  'mouth.rest': ['Klidná ústa', 'Hvile', 'Ruhe', 'Ηρεμία', 'Rest', 'Reposo', 'Repos', 'विश्राम', 'Riposo', '閉じた口', 'Rust', 'Hvile', 'Spoczynek', 'Repouso', 'Vila', '静止'],
  'mouth.ai': ['Samohlásky A/I', 'Vokal A/I', 'Vokal A/I', 'Φωνήεν A/I', 'Vowel A/I', 'Vocal A/I', 'Voyelle A/I', 'स्वर A/I', 'Vocale A/I', '母音 A/I', 'Klinker A/I', 'Vokal A/I', 'Samogłoska A/I', 'Vogal A/I', 'Vokal A/I', '元音 A/I'],
  'mouth.e': ['Samohláska E', 'Vokal E', 'Vokal E', 'Φωνήεν E', 'Vowel E', 'Vocal E', 'Voyelle E', 'स्वर E', 'Vocale E', '母音 E', 'Klinker E', 'Vokal E', 'Samogłoska E', 'Vogal E', 'Vokal E', '元音 E'],
  'mouth.o': ['Samohláska O', 'Vokal O', 'Vokal O', 'Φωνήεν O', 'Vowel O', 'Vocal O', 'Voyelle O', 'स्वर O', 'Vocale O', '母音 O', 'Klinker O', 'Vokal O', 'Samogłoska O', 'Vogal O', 'Vokal O', '元音 O'],
  'mouth.u': ['Samohláska U', 'Vokal U', 'Vokal U', 'Φωνήεν U', 'Vowel U', 'Vocal U', 'Voyelle U', 'स्वर U', 'Vocale U', '母音 U', 'Klinker U', 'Vokal U', 'Samogłoska U', 'Vogal U', 'Vokal U', '元音 U'],
  'mouth.mbp': ['Rty M/B/P', 'Læber M/B/P', 'Lippen M/B/P', 'Χείλη M/B/P', 'Lips M/B/P', 'Labios M/B/P', 'Lèvres M/B/P', 'होंठ M/B/P', 'Labbra M/B/P', '唇 M/B/P', 'Lippen M/B/P', 'Lepper M/B/P', 'Wargi M/B/P', 'Lábios M/B/P', 'Läppar M/B/P', '双唇 M/B/P'],
  'mouth.fv': ['Zuby F/V', 'Tænder F/V', 'Zähne F/V', 'Δόντια F/V', 'Teeth F/V', 'Dientes F/V', 'Dents F/V', 'दाँत F/V', 'Denti F/V', '歯 F/V', 'Tanden F/V', 'Tenner F/V', 'Zęby F/V', 'Dentes F/V', 'Tänder F/V', '牙齿 F/V'],
  // cycles
  'cycle.walk': ['Chůze', 'Gang', 'Gehen', 'Περπάτημα', 'Walk', 'Caminar', 'Marche', 'चलना', 'Camminata', '歩く', 'Lopen', 'Gange', 'Chodzenie', 'Caminhar', 'Gå', '走路'],
  'cycle.idle': ['Klidový stav', 'Hvilestilling', 'Ruhehaltung', 'Αναμονή', 'Idle', 'En reposo', 'Au repos', 'निष्क्रिय', 'A riposo', '待機', 'Rustpose', 'Hvilestilling', 'Postawa spoczynkowa', 'Em repouso', 'Vilopose', '待机'],
  // text styles
  'text.title': ['Nadpis', 'Overskrift', 'Titel', 'Τίτλος', 'Title', 'Título', 'Titre', 'शीर्षक', 'Titolo', 'タイトル', 'Titel', 'Tittel', 'Tytuł', 'Título', 'Rubrik', '标题'],
  'text.caption': ['Titulek', 'Billedtekst', 'Untertitel', 'Λεζάντα', 'Caption', 'Subtítulo', 'Légende', 'कैप्शन', 'Didascalia', '字幕', 'Ondertitel', 'Bildetekst', 'Napis', 'Legenda', 'Bildtext', '字幕'],
  'text.label': ['Popisek', 'Etiket', 'Beschriftung', 'Ετικέτα', 'Label', 'Etiqueta', 'Étiquette', 'लेबल', 'Etichetta', 'ラベル', 'Label', 'Etikett', 'Etykieta', 'Rótulo', 'Etikett', '标签'],
  // transitions
  'trans.cut': ['Střih', 'Klip', 'Schnitt', 'Κοπή', 'Cut', 'Corte', 'Coupe', 'कट', 'Taglio', 'カット', 'Cut', 'Klipp', 'Cięcie', 'Corte', 'Klipp', '切换'],
  'trans.fade': ['Prolnutí', 'Fade', 'Überblendung', 'Σβήσιμο', 'Fade', 'Fundido', 'Fondu', 'फ़ेड', 'Dissolvenza', 'フェード', 'Overvloeien', 'Toning', 'Zanikanie', 'Desvanecer', 'Toning', '淡入淡出'],
  'trans.slide-left': ['Posun doleva', 'Skub til venstre', 'Nach links schieben', 'Ολίσθηση αριστερά', 'Slide left', 'Deslizar a la izquierda', 'Glisser vers la gauche', 'बाईं ओर स्लाइड', 'Scorri a sinistra', '左へスライド', 'Naar links schuiven', 'Skyv til venstre', 'Przesunięcie w lewo', 'Deslizar para a esquerda', 'Skjut åt vänster', '向左滑动'],
  'trans.wipe': ['Stírání', 'Wipe-overgang', 'Wischblende', 'Εναλλαγή σάρωσης', 'Wipe', 'Cortinilla', 'Volet', 'वाइप', 'Tendina', 'ワイプ', 'Veegovergang', 'Wipe-overgang', 'Wycieranie', 'Cortina', 'Svepövergång', '擦除'],
  // palettes
  'palette.sunny': ['Slunečná', 'Solrig', 'Sonnig', 'Ηλιόλουστη', 'Sunny', 'Soleada', 'Ensoleillée', 'धूप भरी', 'Soleggiata', 'サニー', 'Zonnig', 'Solfylt', 'Słoneczna', 'Ensolarada', 'Solig', '阳光'],
  'palette.calm': ['Klidná', 'Rolig', 'Ruhig', 'Ήρεμη', 'Calm', 'Tranquila', 'Calme', 'शांत', 'Calma', 'カーム', 'Rustig', 'Rolig', 'Spokojna', 'Calma', 'Lugn', '宁静'],
};

/** storymaker.catalog.<id with "." replaced by "_"> */
export const nameKey = (id) => `storymaker.catalog.${id.replace('.', '_')}`;

/** {lang: {key: name}} for every language, keys in the order of NAMES. */
export function localeTables() {
  const out = {};
  LANGS.forEach((lang, i) => {
    const table = {};
    for (const [id, row] of Object.entries(NAMES)) table[nameKey(id)] = row[i];
    out[lang] = table;
  });
  return out;
}
```

`assets/storymaker/lib/catalog.mjs`:

```js
// Assembles the catalog (storymaker-catalog/1): manifest entries, rig, props, environments, thumbnails.
// buildOutputs() returns a Map of repo-relative output paths (below ui/img/storymaker/) to file contents.

import crypto from 'node:crypto';
import { buildRig } from './rig.mjs';
import { PROPS } from './props.mjs';
import { ENVIRONMENTS, ENV_META } from './environments.mjs';
import { POSES, GESTURES, CYCLES } from './motion.mjs';
import { nameKey } from './names.mjs';
import { Ctx, svgDoc, placedCharacter, propSvg, environmentSvg, evalKeys, shapesSvg } from './svg.mjs';
import { roundRect, fill, line, poly, circle } from './util.mjs';

export const STAGE = { width: 1920, height: 1080, safe: { x: 5, y: 5, w: 90, h: 90 } };

export const PALETTES = {
  sunny: { bg: '#fff7e6', bg2: '#ffe0a3', fg: '#1f2a44', accent: '#ff7a45', accent2: '#36cfc9', text: '#1f2a44', outline: '#1f2a44' },
  calm: { bg: '#eaf4f4', bg2: '#cfe7ef', fg: '#1b3a4b', accent: '#3a7ca5', accent2: '#81c3d7', text: '#16425b', outline: '#16425b' },
};
const THUMB_PALETTE = PALETTES.sunny;

export const FONTS = [
  { name: 'default', family: 'Geist', weight: 400, file: '/fonts/Geist-Regular.woff2', licence: 'OFL-1.1' },
  { name: 'default', family: 'Geist', weight: 500, file: '/fonts/Geist-Medium.woff2', licence: 'OFL-1.1' },
  { name: 'default', family: 'Geist', weight: 600, file: '/fonts/Geist-SemiBold.woff2', licence: 'OFL-1.1' },
  { name: 'default', family: 'Geist', weight: 700, file: '/fonts/Geist-Bold.woff2', licence: 'OFL-1.1' },
];

export const CHARACTERS = {
  'office-woman-1': {
    slug: 'anna', tags: ['woman', 'office', 'business', 'employee', 'presenter'],
    params: { body: 'average', skin: 'medium', hair: 'bun', hair_color: 'dark-brown', eyes: 'round', brows: 'soft', facial_hair: 'none', glasses: 'none', outfit: 'business', accessory: 'none' },
  },
  'office-man-1': {
    slug: 'ben', tags: ['man', 'office', 'business', 'suit', 'manager'],
    params: { body: 'broad', skin: 'light', hair: 'side-part', hair_color: 'chestnut', eyes: 'almond', brows: 'bold', facial_hair: 'stubble', glasses: 'none', outfit: 'suit', accessory: 'none' },
  },
  'it-tech-1': {
    slug: 'ida', tags: ['it', 'tech', 'support', 'admin', 'headset', 'glasses'],
    params: { body: 'slim', skin: 'brown', hair: 'messy', hair_color: 'black', eyes: 'big', brows: 'thin', facial_hair: 'none', glasses: 'square', outfit: 'it', accessory: 'headset' },
  },
  'senior-man-1': {
    slug: 'otto', tags: ['senior', 'elderly', 'man', 'retired', 'grandfather'],
    params: { body: 'elder', skin: 'tan', hair: 'balding', hair_color: 'white', eyes: 'almond', brows: 'soft', facial_hair: 'mustache', glasses: 'round', outfit: 'cardigan', accessory: 'none' },
  },
};

// expression -> eye, brow and mouth variants of the rig
export const EXPRESSIONS = {
  neutral: { eyes: 'neutral', brows: 'neutral', mouth: 'rest', tags: ['neutral', 'calm'] },
  happy: { eyes: 'smile', brows: 'neutral', mouth: 'smile', tags: ['happy', 'smile', 'pleased'] },
  laugh: { eyes: 'laugh', brows: 'raised', mouth: 'laugh', tags: ['laugh', 'joy', 'funny'] },
  sad: { eyes: 'sad', brows: 'sad', mouth: 'frown', tags: ['sad', 'unhappy', 'cry'] },
  angry: { eyes: 'angry', brows: 'angry', mouth: 'grit', tags: ['angry', 'mad', 'furious'] },
  surprised: { eyes: 'wide', brows: 'raised', mouth: 'o', tags: ['surprised', 'shock', 'amazed'] },
  confused: { eyes: 'neutral', brows: 'confused', mouth: 'squiggle', tags: ['confused', 'puzzled', 'unsure'] },
  thinking: { eyes: 'lookup', brows: 'think', mouth: 'side', tags: ['thinking', 'wonder', 'ponder'] },
  worried: { eyes: 'worried', brows: 'worried', mouth: 'wobble', tags: ['worried', 'anxious', 'nervous'] },
  determined: { eyes: 'narrow', brows: 'determined', mouth: 'firm', tags: ['determined', 'focused', 'resolute'] },
};

export const MOUTHS = ['rest', 'ai', 'e', 'o', 'u', 'mbp', 'fv'];

const POSE_TAGS = {
  stand: ['stand', 'idle'], sit: ['sit', 'seated', 'chair'], typing: ['typing', 'computer', 'sit', 'keyboard'],
  'point-left': ['point', 'left', 'show'], 'point-right': ['point', 'right', 'show'], wave: ['wave', 'hello', 'greet'],
  think: ['think', 'ponder', 'chin'], present: ['present', 'offer', 'show'], 'arms-crossed': ['arms crossed', 'skeptical', 'wait'],
  'hands-on-hips': ['hands on hips', 'confident', 'proud'],
};
const GESTURE_TAGS = {
  point: ['point', 'show'], wave: ['wave', 'hello'], shrug: ['shrug', 'unsure', 'dont know'], think: ['think', 'ponder'],
  present: ['present', 'offer'], 'thumbs-up': ['thumbs up', 'approve', 'good'], celebrate: ['celebrate', 'cheer', 'success'],
  facepalm: ['facepalm', 'oops', 'mistake'], count: ['count', 'number', 'list'],
};

const TEXT_STYLES = {
  title: { font: 'default', size: 0.07, weight: 700, color: '$text', align: 'center', max_lines: 2, tags: ['title', 'heading', 'headline'] },
  caption: { font: 'default', size: 0.042, weight: 500, color: '#ffffff', box: { fill: '#1f2a44d9', radius: 0.012, pad: 0.012 }, align: 'center', max_lines: 2, tags: ['caption', 'subtitle', 'lower third'] },
  label: { font: 'default', size: 0.034, weight: 600, color: '$text', box: { fill: '$bg2', radius: 0.02, pad: 0.01 }, align: 'center', max_lines: 2, tags: ['label', 'tag', 'small text'] },
};

const TRANSITIONS = { cut: { duration: 0, tags: ['cut', 'instant'] }, fade: { duration: 0.5, tags: ['fade', 'dissolve'] }, 'slide-left': { duration: 0.5, tags: ['slide', 'push'] }, wipe: { duration: 0.5, tags: ['wipe', 'reveal'] } };

const ENV_TAGS = {
  'plain-stage': ['plain', 'stage', 'neutral', 'gradient'],
  office: ['office', 'work', 'desk', 'whiteboard', 'workplace'],
  'home-office': ['home', 'office', 'remote', 'desk', 'bookshelf'],
  'abstract-tech': ['tech', 'abstract', 'network', 'digital', 'cyber'],
};

function entry(id, kind, name, tags, usage, extra) {
  return { id, kind, name, name_key: nameKey(id), tags, thumb: `thumbs/${id}.svg`, usage_example: usage, ...extra };
}

/** All manifest entries in a stable order. */
export function buildEntries() {
  const out = [];
  for (const [n, c] of Object.entries(CHARACTERS)) {
    const id = `char.${n}`;
    out.push(entry(id, 'character', id, c.tags, { cast: [{ id: c.slug, character: id }] }, { params: c.params }));
  }
  for (const [n, p] of Object.entries(PROPS)) {
    const id = `prop.${n}`;
    const usage = { do: 'show', item: id, at: 'right', entry: n === 'shield' || n === 'cloud' ? 'fade' : 'pop' };
    const extra = { file: `props/${n}.json`, size: { h: p.h }, anchor: { x: 0.5, y: 1 } };
    if (p.hold) extra.hold = true;
    out.push(entry(id, 'prop', id, p.tags, usage, extra));
  }
  for (const n of Object.keys(ENVIRONMENTS)) {
    const id = `env.${n}`;
    const m = ENV_META[n];
    out.push(entry(id, 'environment', id, ENV_TAGS[n], { environment: id }, { file: `environments/${n}.json`, spots: m.spots, floor: m.floor, variants: ['day', 'night'], aspects: m.aspects }));
  }
  for (const [n, p] of Object.entries(POSES)) {
    const extra = { bones: p.bones };
    if (p.hands) extra.hands = p.hands;
    if (p.view) extra.view = p.view;
    if (p.seated) extra.seated = true;
    if (p.offset_y) extra.offset_y = p.offset_y;
    out.push(entry(`pose.${n}`, 'pose', n, POSE_TAGS[n], { pose: n }, extra));
  }
  for (const [n, g] of Object.entries(GESTURES)) {
    out.push(entry(`gesture.${n}`, 'gesture', n, GESTURE_TAGS[n], { do: 'gesture', who: 'anna', gesture: n }, { duration: g.duration, keys: g.keys }));
  }
  for (const [n, e] of Object.entries(EXPRESSIONS)) {
    out.push(entry(`expr.${n}`, 'expression', n, e.tags, n === 'neutral' ? { expression: n } : { do: 'emote', who: 'anna', expression: n }, { eyes: e.eyes, brows: e.brows, mouth: e.mouth }));
  }
  for (const m of MOUTHS) out.push(entry(`mouth.${m}`, 'mouth', m, ['mouth', 'lip sync', m], {}, { shape: m }));
  out.push(entry('cycle.walk', 'cycle', 'walk', ['walk', 'move'], { do: 'walk', who: 'anna', to: 'center' }, { period: CYCLES.walk.period, speed: CYCLES.walk.speed, keys: CYCLES.walk.keys }));
  out.push(entry('cycle.idle', 'cycle', 'idle', ['idle', 'breathing'], {}, { period: CYCLES.idle.period, keys: CYCLES.idle.keys }));
  for (const [n, t] of Object.entries(TEXT_STYLES)) {
    const { tags, ...rest } = t;
    const usage = n === 'title' ? { do: 'show', text: 'Hallo', at: 'center' } : { do: 'show', text: 'Hallo', at: 'center', style: n };
    out.push(entry(`text.${n}`, 'text_style', n, tags, usage, rest));
  }
  for (const [n, t] of Object.entries(TRANSITIONS)) out.push(entry(`trans.${n}`, 'transition', n, t.tags, { transition: n }, { duration: t.duration }));
  return out;
}

export function buildPalettes() {
  return Object.entries(PALETTES).map(([name, tokens]) => ({ id: `palette.${name}`, name, name_key: nameKey(`palette.${name}`), tokens }));
}

// ---------------------------------------------------------------- thumbnails
const REF = CHARACTERS['office-woman-1'].params;

function charThumb(rig, params, opts, viewBox, size) {
  const ctx = new Ctx();
  const g = placedCharacter(rig, params, THUMB_PALETTE, ctx, { x: 0, y: 0, scale: 1, facing: 'right', ...opts });
  return svgDoc(viewBox, g, ctx, { width: size[0], height: size[1] });
}

function textThumb(name) {
  const ctx = new Ctx();
  const maps = [THUMB_PALETTE];
  let shapes;
  if (name === 'title') {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), '$bg'), fill(roundRect(52, 60, 216, 26, 8), '$text'), fill(roundRect(92, 100, 136, 18, 6), '$accent')];
  } else if (name === 'caption') {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), '$bg'), fill(circle(250, 60, 34), '$accent2', 0.5), fill(roundRect(44, 112, 232, 44, 10), '#1f2a44', 0.85), fill(roundRect(64, 124, 192, 8, 4), '#ffffff'), fill(roundRect(94, 140, 132, 8, 4), '#ffffff', 0.8)];
  } else {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), '$bg'), fill(roundRect(96, 62, 128, 40, 20), '$bg2'), fill(roundRect(114, 76, 92, 12, 6), '$text'), line('M160 104L160 140', '$accent', 4), fill(circle(160, 146, 8), '$accent')];
  }
  return svgDoc([0, 0, 320, 180], shapesSvg(shapes, maps, ctx), ctx, { width: 320, height: 180 });
}

function transThumb(name) {
  const ctx = new Ctx();
  const maps = [THUMB_PALETTE];
  const A = '$accent2';
  const B = '$accent';
  let shapes;
  if (name === 'cut') {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), A), fill('M190 0L310 0A10 10 0 0 1 320 10L320 170A10 10 0 0 1 310 180L130 180Z', B), line('M190 0L130 180', '#ffffff', 5)];
  } else if (name === 'fade') {
    shapes = [{ d: roundRect(0, 0, 320, 180, 10), gradient: { type: 'linear', stops: [[0, A], [1, B]], x1: 0, y1: 0, x2: 320, y2: 0 } }];
  } else if (name === 'slide-left') {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), A), fill('M150 0L310 0A10 10 0 0 1 320 10L320 170A10 10 0 0 1 310 180L150 180Z', B), line('M110 90L50 90M74 66L50 90L74 114', '#ffffff', 8)];
  } else {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), A), fill('M170 0L310 0A10 10 0 0 1 320 10L320 170A10 10 0 0 1 310 180L170 180Z', B), fill(poly([[160, 0], [184, 0], [184, 180], [160, 180]]), '#ffffff', 0.85), line('M138 90L84 90M106 68L84 90L106 112', '#ffffff', 8)];
  }
  return svgDoc([0, 0, 320, 180], shapesSvg(shapes, maps, ctx), ctx, { width: 320, height: 180 });
}

/** One SVG per entry id. */
export function buildThumbs(rig, entries) {
  const out = new Map();
  for (const e of entries) {
    const id = e.id;
    let svg;
    switch (e.kind) {
      case 'character':
        svg = charThumb(rig, e.params, { view: 'three_quarter' }, [-200, -1025, 400, 575], [220, 316]);
        break;
      case 'prop': {
        const p = PROPS[id.slice(5)].build();
        const ctx = new Ctx();
        const inner = propSvg(p, THUMB_PALETTE, ctx);
        const [, , w, h] = p.viewbox;
        svg = svgDoc(p.viewbox, inner, ctx, { width: Math.round((160 * w) / h), height: 160 });
        break;
      }
      case 'environment': {
        const env = ENVIRONMENTS[id.slice(4)]();
        const ctx = new Ctx();
        const inner = environmentSvg(env, 'day', THUMB_PALETTE, ctx);
        svg = svgDoc([0, 0, 1920, 1080], inner, ctx, { width: 320, height: 180 });
        break;
      }
      case 'pose': {
        const p = POSES[e.name];
        // seated poses are cropped lower, standing ones from the head to the knees
        const drop = p.offset_y || 0;
        const h = drop ? 1075 - drop : 720;
        svg = charThumb(rig, REF, { view: p.view || 'three_quarter', pose: p, offset_y: drop }, [-260, -1025 + drop, 520, h], [200, Math.round((200 * h) / 520)]);
        break;
      }
      case 'gesture': {
        const g = GESTURES[e.name];
        const peak = g.keys.reduce((a, b) => (Object.keys(b.bones).length > Object.keys(a.bones).length ? b : a), g.keys[0]);
        svg = charThumb(rig, REF, { view: 'three_quarter', pose: POSES.stand, deltas: peak.bones, hands: peak.hands || {} }, [-260, -1025, 520, 720], [200, 277]);
        break;
      }
      case 'expression': {
        svg = charThumb(rig, REF, { view: 'front', expression: { eyes: e.eyes, brows: e.brows, mouth: e.mouth } }, [-135, -1010, 270, 270], [180, 180]);
        break;
      }
      case 'mouth':
        svg = charThumb(rig, REF, { view: 'front', mouth: e.shape }, [-110, -895, 220, 190], [180, 155]);
        break;
      case 'cycle': {
        if (e.name === 'walk') {
          const k = evalKeys(e.keys, 0);
          svg = charThumb(rig, REF, { view: 'profile', pose: POSES.stand, deltas: k.bones, offset_y: k.offset_y }, [-260, -1025, 520, 1075], [200, 413]);
        } else {
          svg = charThumb(rig, REF, { view: 'three_quarter', pose: POSES.stand }, [-260, -1025, 520, 1075], [200, 413]);
        }
        break;
      }
      case 'text_style':
        svg = textThumb(e.name);
        break;
      case 'transition':
        svg = transThumb(e.name);
        break;
      default:
        throw new Error(`no thumbnail for ${e.kind}`);
    }
    out.set(`thumbs/${id}.svg`, svg);
  }
  return out;
}

// ---------------------------------------------------------------- manifest and hashing
const lf = (s) => s.replace(/\r\n/g, '\n');

/** sha256 over the LF-normalised bytes of every output except catalog.json in sorted path order, first 12 hex. */
export function versionOf(files) {
  const h = crypto.createHash('sha256');
  for (const p of [...files.keys()].filter((k) => k !== 'catalog.json').sort()) h.update(Buffer.from(lf(files.get(p)), 'utf8'));
  return h.digest('hex').slice(0, 12);
}

function manifestText(manifest) {
  const lines = [];
  const j = (v) => JSON.stringify(v);
  lines.push('{' + [`"schema":${j(manifest.schema)}`, `"version":${j(manifest.version)}`, `"stage":${j(manifest.stage)}`, `"rig":${j(manifest.rig)}`].join(',') + ',');
  lines.push(` "fonts":[\n${manifest.fonts.map((f) => '  ' + j(f)).join(',\n')}\n ],`);
  lines.push(` "palettes":[\n${manifest.palettes.map((p) => '  ' + j(p)).join(',\n')}\n ],`);
  lines.push(` "entries":[\n${manifest.entries.map((e) => '  ' + j(e)).join(',\n')}\n ]}`);
  return lines.join('\n') + '\n';
}

/** Every output file below ui/img/storymaker/ as a Map(relative path -> text). */
export function buildOutputs() {
  const rig = buildRig();
  const entries = buildEntries();
  const files = new Map();
  files.set('rig/human.json', JSON.stringify(rig) + '\n');
  for (const [n, p] of Object.entries(PROPS)) {
    const b = p.build();
    files.set(`props/${n}.json`, JSON.stringify({ viewbox: b.viewbox, shapes: b.shapes }) + '\n');
  }
  for (const [n, f] of Object.entries(ENVIRONMENTS)) {
    const e = f();
    files.set(`environments/${n}.json`, JSON.stringify({ viewbox: [0, 0, 1920, 1080], layers: e.layers, tokens: e.tokens }) + '\n');
  }
  for (const [p, svg] of buildThumbs(rig, entries)) files.set(p, svg);
  const manifest = {
    schema: 'storymaker-catalog/1',
    version: versionOf(files),
    stage: STAGE,
    rig: { id: 'rig.human/1', file: 'rig/human.json' },
    fonts: FONTS,
    palettes: buildPalettes(),
    entries,
  };
  files.set('catalog.json', manifestText(manifest));
  return { files, manifest, rig, entries };
}

```

`assets/storymaker/lib/validate.mjs`:

```js
// Checks of the generated catalog against contract section 3. The build refuses to write when anything fails;
// the Go test ui/storymaker_catalog_test.go repeats the schema checks on the committed files.

import { parsePath } from './pathdata.mjs';
import { versionOf, PALETTES } from './catalog.mjs';
import { NAMES, LANGS, nameKey } from './names.mjs';
import { Ctx, characterSvg } from './svg.mjs';
import { MOUTH_NAMES } from './face.mjs';

export const REQUIRED = {
  character: ['char.office-woman-1', 'char.office-man-1', 'char.it-tech-1', 'char.senior-man-1'],
  prop: ['laptop', 'phone', 'monitor', 'server', 'router', 'cloud', 'database', 'lock', 'key', 'shield', 'mail', 'document', 'coffee-cup', 'desk', 'chair', 'plant', 'check', 'cross', 'warning', 'idea', 'question', 'arrow-right', 'gear'].map((n) => `prop.${n}`),
  environment: ['env.plain-stage', 'env.office', 'env.home-office', 'env.abstract-tech'],
  pose: ['stand', 'sit', 'typing', 'point-left', 'point-right', 'wave', 'think', 'present', 'arms-crossed', 'hands-on-hips'].map((n) => `pose.${n}`),
  gesture: ['point', 'wave', 'shrug', 'think', 'present', 'thumbs-up', 'celebrate', 'facepalm', 'count'].map((n) => `gesture.${n}`),
  expression: ['neutral', 'happy', 'laugh', 'sad', 'angry', 'surprised', 'confused', 'thinking', 'worried', 'determined'].map((n) => `expr.${n}`),
  mouth: ['rest', 'ai', 'e', 'o', 'u', 'mbp', 'fv'].map((n) => `mouth.${n}`),
  cycle: ['cycle.walk', 'cycle.idle'],
  text_style: ['text.title', 'text.caption', 'text.label'],
  transition: ['trans.cut', 'trans.fade', 'trans.slide-left', 'trans.wipe'],
};
const PREFIX = { character: 'char.', prop: 'prop.', environment: 'env.', pose: 'pose.', gesture: 'gesture.', expression: 'expr.', mouth: 'mouth.', cycle: 'cycle.', text_style: 'text.', transition: 'trans.' };
const BONES = ['root', 'hips', 'torso', 'neck', 'head', 'upper_arm_l', 'lower_arm_l', 'upper_arm_r', 'lower_arm_r', 'upper_leg_l', 'lower_leg_l', 'upper_leg_r', 'lower_leg_r'];
const SLOT_PARAMS = ['body', 'skin', 'hair', 'hair_back', 'eyes', 'brows', 'mouth', 'hand', 'top', 'bottom', 'shoes', 'glasses', 'facial_hair', 'accessory'];
const VIEWS = ['front', 'three_quarter', 'profile'];
const CHAR_TOKENS = ['skin', 'hair', 'top', 'bottom', 'shoes', 'outline', 'eye_white', 'iris', 'lip'];
const PALETTE_TOKENS = ['bg', 'bg2', 'fg', 'accent', 'accent2', 'text', 'outline'];

function shapeProblems(where, shapes, known, out) {
  shapes.forEach((s, i) => {
    const p = parsePath(s.d);
    if (!p.ok) out.push(`${where}[${i}]: bad path data: ${p.error}`);
    const colours = [s.fill, s.stroke, ...(s.gradient ? s.gradient.stops.map((st) => st[1]) : [])].filter((c) => typeof c === 'string');
    if (!s.fill && !s.stroke && !s.gradient) out.push(`${where}[${i}]: shape draws nothing`);
    for (const c of colours) {
      if (c[0] === '$' && !known.has(c.slice(1))) out.push(`${where}[${i}]: unknown token ${c}`);
      if (c[0] !== '$' && !/^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$/.test(c)) out.push(`${where}[${i}]: bad colour ${c}`);
    }
    if (s.opacity !== undefined && !(s.opacity >= 0 && s.opacity <= 1)) out.push(`${where}[${i}]: opacity out of range`);
  });
}

export function validateOutputs(built, { maxProblems = 40 } = {}) {
  const out = [];
  const { files, manifest, rig, entries } = built;
  const add = (m) => { if (out.length < maxProblems) out.push(m); };

  if (manifest.schema !== 'storymaker-catalog/1') add('schema must be storymaker-catalog/1');
  if (manifest.version !== versionOf(files)) add('manifest version does not match the output files');
  if (!/^[0-9a-f]{12}$/.test(manifest.version)) add('version must be 12 hex digits');
  const ids = new Set();
  for (const e of entries) {
    if (ids.has(e.id)) add(`duplicate id ${e.id}`);
    ids.add(e.id);
    if (!e.id.startsWith(PREFIX[e.kind] || '\0')) add(`${e.id}: prefix does not match kind ${e.kind}`);
    if (e.name_key !== nameKey(e.id)) add(`${e.id}: wrong name_key`);
    if (!NAMES[e.id]) add(`${e.id}: no names`);
    if (!files.has(e.thumb)) add(`${e.id}: missing thumbnail ${e.thumb}`);
    if (!Array.isArray(e.tags) || !e.tags.length) add(`${e.id}: no tags`);
    if (typeof e.usage_example !== 'object') add(`${e.id}: usage_example must be an object`);
    if (e.file && !files.has(e.file)) add(`${e.id}: missing file ${e.file}`);
  }
  for (const [kind, list] of Object.entries(REQUIRED)) for (const id of list) if (!ids.has(id)) add(`missing required ${kind} ${id}`);
  for (const p of manifest.palettes) {
    if (!NAMES[p.id]) add(`${p.id}: no names`);
    for (const t of PALETTE_TOKENS) if (!/^#[0-9a-f]{6}$/.test(p.tokens[t] || '')) add(`${p.id}: token ${t} missing`);
  }
  if (!manifest.palettes.some((p) => p.name === 'sunny') || !manifest.palettes.some((p) => p.name === 'calm')) add('palettes sunny and calm are required');

  // rig
  const rigTokens = new Set([...CHAR_TOKENS, ...PALETTE_TOKENS]);
  if (rig.bones.map((b) => b.name).join() !== BONES.join()) add('rig bones differ from the contract list');
  for (const b of rig.bones) for (const v of VIEWS) if (!b.rest[v]) add(`bone ${b.name} lacks rest.${v}`);
  for (const s of rig.slots) {
    if (!BONES.includes(s.bone)) add(`slot ${s.name}: unknown bone`);
    if (!SLOT_PARAMS.includes(s.param)) add(`slot ${s.name}: unknown param ${s.param}`);
    for (const v of VIEWS) if (!Number.isInteger(s.z[v])) add(`slot ${s.name}: z.${v} missing`);
    if (!rig.variants[s.name]) add(`slot ${s.name}: no variants`);
  }
  for (const [slot, keys] of Object.entries(rig.variants)) {
    for (const [key, views] of Object.entries(keys)) {
      for (const v of VIEWS) {
        if (!Array.isArray(views[v])) add(`variant ${slot}/${key} lacks ${v}`);
        else shapeProblems(`rig ${slot}/${key}/${v}`, views[v], rigTokens, out);
      }
    }
  }
  for (const name of Object.keys(rig.bodies)) if (!rig.params.body.includes(name)) add(`body ${name} not in params.body`);
  for (const b of rig.params.body) if (!rig.bodies[b]) add(`params.body lists ${b} without a body entry`);

  // every starter character, every view, every expression and hand renders without missing variants or tokens
  for (const e of entries.filter((x) => x.kind === 'character')) {
    for (const v of VIEWS) {
      for (const ex of entries.filter((x) => x.kind === 'expression')) {
        for (const hand of ['open', 'fist', 'point', 'hold']) {
          const ctx = new Ctx();
          characterSvg(rig, e.params, PALETTES.sunny, ctx, { view: v, pose: { bones: {}, hands: { l: hand, r: hand } }, expression: ex });
          if (ctx.missing.size) add(`${e.id} ${v} ${ex.id}: missing variants ${[...ctx.missing].join(',')}`);
          if (ctx.unresolved.size) add(`${e.id} ${v}: unresolved tokens ${[...ctx.unresolved].join(',')}`);
        }
      }
      for (const m of MOUTH_NAMES) {
        const ctx = new Ctx();
        characterSvg(rig, e.params, PALETTES.sunny, ctx, { view: v, mouth: m });
        if (ctx.missing.size) add(`${e.id} ${v} mouth ${m}: missing variants`);
      }
    }
    for (const [k, list] of Object.entries({ body: rig.params.body, hair: rig.params.hair, eyes: rig.params.eyes, brows: rig.params.brows, glasses: rig.params.glasses, facial_hair: rig.params.facial_hair, accessory: rig.params.accessory })) {
      if (!list.includes(e.params[k])) add(`${e.id}: ${k}=${e.params[k]} not in the parameter list`);
    }
    if (!rig.params.skin[e.params.skin]) add(`${e.id}: unknown skin`);
    if (!rig.params.hair_color[e.params.hair_color]) add(`${e.id}: unknown hair colour`);
    if (!rig.params.outfit[e.params.outfit]) add(`${e.id}: unknown outfit`);
  }
  // every parameter value has shapes (so a future character can use it)
  for (const style of rig.params.hair) if (!rig.variants.hair[style] || !rig.variants.hair_back[style]) add(`hair style ${style} has no variants`);
  for (const [name, o] of Object.entries(rig.params.outfit)) {
    for (const [slot, key] of [['top_det', o.top], ['sleeve_u_l', o.top], ['pelvis_det', o.bottom], ['leg_det_u_l', o.bottom], ['foot_l', o.shoes]]) if (!rig.variants[slot]?.[key]) add(`outfit ${name}: ${slot}/${key} missing`);
    for (const c of ['top', 'bottom', 'shoes']) if (!/^#[0-9a-f]{6}$/.test(o.colors[c])) add(`outfit ${name}: colour ${c}`);
  }

  // poses, gestures, cycles reference known bones, hands and views
  const checkBones = (where, bones) => { for (const [b, deg] of Object.entries(bones || {})) { if (!BONES.includes(b)) add(`${where}: unknown bone ${b}`); if (!Number.isFinite(deg)) add(`${where}: bad angle`); } };
  const checkHands = (where, hands) => { for (const [side, h] of Object.entries(hands || {})) if (!['l', 'r'].includes(side) || !['open', 'fist', 'point', 'hold'].includes(h)) add(`${where}: bad hands`); };
  for (const e of entries) {
    if (e.kind === 'pose') { checkBones(e.id, e.bones); checkHands(e.id, e.hands); if (e.view && !VIEWS.includes(e.view)) add(`${e.id}: bad view`); }
    if (e.kind === 'gesture' || e.kind === 'cycle') {
      if (!(e.keys[0].t === 0 && e.keys[e.keys.length - 1].t === 1)) add(`${e.id}: keys must run from t=0 to t=1`);
      e.keys.forEach((k, i) => { checkBones(`${e.id} key ${i}`, k.bones); checkHands(`${e.id} key ${i}`, k.hands); if (i && k.t < e.keys[i - 1].t) add(`${e.id}: keys out of order`); });
      if (e.kind === 'gesture') {
        const first = JSON.stringify(e.keys[0].bones);
        const last = JSON.stringify(e.keys[e.keys.length - 1].bones);
        if (first !== last) add(`${e.id}: a gesture must return to its first key`);
        if (!(e.duration > 0)) add(`${e.id}: duration`);
      } else if (!(e.period > 0)) add(`${e.id}: period`);
    }
    if (e.kind === 'expression') {
      if (!rig.variants.eyes[`round/${e.eyes}`] || !rig.variants.brows[`soft/${e.brows}`] || !rig.variants.mouth[e.mouth]) add(`${e.id}: eyes/brows/mouth variant missing in the rig`);
    }
    if (e.kind === 'mouth' && !rig.variants.mouth[e.shape]) add(`${e.id}: rig has no mouth ${e.shape}`);
  }

  // props: palette tokens only
  for (const e of entries.filter((x) => x.kind === 'prop')) {
    const f = JSON.parse(files.get(e.file));
    shapeProblems(e.file, f.shapes, new Set(PALETTE_TOKENS), out);
    if (!(e.size.h > 0 && e.size.h < 1)) add(`${e.id}: size.h`);
    if (f.viewbox.length !== 4 || !(f.viewbox[2] > 0 && f.viewbox[3] > 0)) add(`${e.file}: viewbox`);
  }
  // environments
  const BASE = ['far-left', 'left', 'center', 'right', 'far-right'];
  const EXTRA = { 'env.plain-stage': ['board'], 'env.office': ['desk', 'door', 'board', 'window'], 'env.home-office': ['desk', 'door', 'window'], 'env.abstract-tech': ['board'] };
  // a 9:16 or 1:1 output shows a vertical slice of the 16:9 design frame, as wide as the output is at the design height
  const cropHalf = { '9:16': (100 * ((1080 * 9) / 16)) / 1920 / 2, '1:1': (100 * 1080) / 1920 / 2 };
  for (const e of entries.filter((x) => x.kind === 'environment')) {
    const f = JSON.parse(files.get(e.file));
    for (const v of ['day', 'night']) if (!f.tokens[v]) add(`${e.file}: tokens.${v} missing`);
    const dayKeys = Object.keys(f.tokens.day);
    const nightKeys = Object.keys(f.tokens.night);
    const known = new Set(PALETTE_TOKENS);
    for (const k of dayKeys) known.add(k);
    for (const k of nightKeys) if (!PALETTE_TOKENS.includes(k) && !dayKeys.includes(k)) add(`${e.file}: night token ${k} has no day value`);
    for (const k of dayKeys) if (!nightKeys.includes(k)) add(`${e.file}: day token ${k} has no night value`);
    for (const l of f.layers) {
      if (!(l.parallax >= 0 && l.parallax <= 1)) add(`${e.file}: layer ${l.name} parallax`);
      shapeProblems(`${e.file} layer ${l.name}`, l.shapes, known, out);
    }
    for (const s of [...BASE, ...EXTRA[e.id]]) if (!e.spots[s]) add(`${e.id}: spot ${s} missing`);
    for (const [name, sp] of Object.entries(e.spots)) if (!(sp.x >= 0 && sp.x <= 100 && sp.y >= 0 && sp.y <= 100)) add(`${e.id}: spot ${name} out of range`);
    const fl = e.floor;
    if (!(fl.y_far < fl.y_near && fl.scale_far < fl.scale_near)) add(`${e.id}: floor`);
    if (JSON.stringify(e.variants) !== '["day","night"]') add(`${e.id}: variants`);
    for (const asp of ['9:16', '1:1']) {
      const a = e.aspects?.[asp];
      if (!a || typeof a.focus_x !== 'number') { add(`${e.id}: aspect ${asp} override missing`); continue; }
      const half = cropHalf[asp];
      for (const name of Object.keys(e.spots)) {
        const x = (a.spots?.[name] || e.spots[name]).x;
        if (x < a.focus_x - half + 3 || x > a.focus_x + half - 3) add(`${e.id}: spot ${name} (x=${x}) lies outside the ${asp} crop around ${a.focus_x}`);
      }
    }
  }
  // thumbnails are svg and parse as path data
  for (const [p, text] of files) {
    if (!p.startsWith('thumbs/')) continue;
    if (!text.startsWith('<svg ') || /<script|<image|href=/i.test(text)) add(`${p}: not a plain svg`);
    for (const m of text.matchAll(/ d="([^"]*)"/g)) { const r = parsePath(m[1]); if (!r.ok) { add(`${p}: bad path: ${r.error}`); break; } }
  }
  // names
  for (const [id, row] of Object.entries(NAMES)) {
    if (row.length !== LANGS.length || row.some((s) => typeof s !== 'string' || !s.trim())) add(`names ${id}: needs ${LANGS.length} non-empty strings`);
    if (!ids.has(id) && !manifest.palettes.some((p) => p.id === id)) add(`names ${id}: no such entry`);
  }
  LANGS.forEach((lang, i) => {
    if (lang === 'en') return;
    const same = Object.values(NAMES).filter((row) => row[i] === row[LANGS.indexOf('en')]).length;
    if (same > Object.keys(NAMES).length * 0.35) add(`names ${lang}: ${same} of ${Object.keys(NAMES).length} are copies of the English name`);
  });
  return out;
}
```

`assets/storymaker/lib/review.mjs`:

```js
// Review sheets: the cast, the parameter variety, props and environments rendered with the section 3a renderer
// (svg.mjs). Used by `build-storymaker-catalog.mjs --review <dir>`; the SVG sheets are screenshotted with Chrome.

import { Ctx, placedCharacter, svgDoc, evalKeys, propSvg, environmentSvg } from './svg.mjs';
import { CHARACTERS, EXPRESSIONS, PALETTES } from './catalog.mjs';
import { POSES, GESTURES, CYCLES } from './motion.mjs';
import { PROPS } from './props.mjs';
import { ENVIRONMENTS, ENV_META } from './environments.mjs';
import { HAIR_STYLES } from './hair.mjs';
import { FACIAL_HAIR, GLASSES, ACCESSORIES } from './extras.mjs';
import { OUTFITS } from './clothes.mjs';
import { BODY_NAMES } from './dims.mjs';

const PAL = PALETTES.sunny;
const label = (x, y, text, size = 26) => `<text x="${x}" y="${y}" font-family="sans-serif" font-size="${size}" fill="#1f2a44" stroke="#ffffff" stroke-width="${Math.max(2, size / 7)}" paint-order="stroke">${text}</text>`;
const cell = (x, y, w, h, viewBox, inner) => `<svg x="${x}" y="${y}" width="${w}" height="${h}" viewBox="${viewBox.join(' ')}">${inner}</svg>`;
const FULL = [-230, -1030, 460, 1090];
const HEAD = [-135, -1010, 270, 270];
const NEUTRAL = { eyes: 'neutral', brows: 'neutral', mouth: 'rest' };

function actor(rig, params, ctx, opts) {
  return placedCharacter(rig, params, PAL, ctx, { x: 0, y: 0, scale: 1, facing: 'right', expression: NEUTRAL, ...opts });
}

/** All starters in three views, expressions, poses, gestures and the walk cycle. */
export function castSheet(rig) {
  const ctx = new Ctx();
  let inner = '';
  let y = 0;
  const chars = Object.entries(CHARACTERS);
  // row 1: four characters in front / three quarter / profile
  const cw = 230;
  const ch = 545;
  chars.forEach(([name, c], ci) => {
    ['front', 'three_quarter', 'profile'].forEach((view, vi) => {
      inner += cell((ci * 3 + vi) * cw, y, cw, ch, [-230, -1030, 460, 1090], actor(rig, c.params, ctx, { view }));
    });
    inner += label(ci * 3 * cw + 8, y + 20, name, 20);
  });
  y += ch;
  const width = cw * 12;
  // rows 2-3: expressions on the four characters (front and three quarter)
  const exprNames = Object.keys(EXPRESSIONS);
  const ew = 176;
  const eh = 176;
  chars.forEach(([name, c], ci) => {
    exprNames.forEach((en, ei) => {
      const e = EXPRESSIONS[en];
      const x = (ei % 10) * ew + (ci % 2) * 0;
      const yy = y + (ci * 2) * eh;
      inner += cell(x, yy, ew, eh, HEAD, actor(rig, c.params, ctx, { view: 'front', expression: e }));
      inner += cell(x, yy + eh, ew, eh, HEAD, actor(rig, c.params, ctx, { view: 'three_quarter', expression: e }));
    });
  });
  y += chars.length * 2 * eh;
  // poses and gestures on the first character
  const pw = 230;
  const ph = 424;
  const cells = [];
  for (const [n, p] of Object.entries(POSES)) cells.push({ n, view: p.view || 'three_quarter', pose: p });
  for (const [n, g] of Object.entries(GESTURES)) {
    const peak = g.keys.reduce((a, b) => (Object.keys(b.bones).length > Object.keys(a.bones).length ? b : a), g.keys[0]);
    cells.push({ n: `gesture ${n}`, view: 'three_quarter', pose: POSES.stand, deltas: peak.bones, hands: peak.hands });
  }
  for (const t of [0, 0.25, 0.5, 0.75]) {
    const k = evalKeys(CYCLES.walk.keys, t);
    cells.push({ n: `walk ${t}`, view: 'profile', pose: POSES.stand, deltas: k.bones, offset_y: k.offset_y });
  }
  const per = Math.floor(width / pw);
  cells.forEach((c, i) => {
    const x = (i % per) * pw;
    const yy = y + Math.floor(i / per) * ph;
    inner += cell(x, yy + 24, pw, ph - 24, FULL, actor(rig, chars[0][1].params, ctx, { view: c.view, pose: c.pose, deltas: c.deltas, hands: c.hands, offset_y: c.offset_y }));
    inner += label(x + 6, yy + 18, `${c.n} (${c.view.replace('three_quarter', '3/4')})`, 17);
  });
  y += Math.ceil(cells.length / per) * ph;
  return { svg: svgDoc([0, 0, width, y], inner, ctx, { background: PAL.bg }), width, height: y };
}

/** Every value of the parameter lists on the reference character. */
export function varietySheet(rig) {
  const ctx = new Ctx();
  let inner = '';
  const base = CHARACTERS['office-woman-1'].params;
  const man = CHARACTERS['office-man-1'].params;
  let y = 0;
  const hw = 170;
  const hh = 170;
  const row = (title, items, make, view = 'three_quarter', box = HEAD, w = hw, h = hh, perRow = 12) => {
    inner += label(6, y + 24, title, 22);
    y += 30;
    items.forEach((it, i) => {
      const xx = (i % perRow) * w;
      const yy = y + Math.floor(i / perRow) * (h + 22);
      inner += cell(xx, yy, w, h, box, actor(rig, make(it), ctx, { view }));
      inner += label(xx + 4, yy + h + 16, it, 15);
    });
    y += Math.ceil(items.length / perRow) * (h + 22) + 8;
  };
  const width = hw * 12;
  row('hair (front)', HAIR_STYLES, (v) => ({ ...base, hair: v }), 'front');
  row('hair (three quarter)', HAIR_STYLES, (v) => ({ ...base, hair: v }), 'three_quarter');
  row('hair (profile)', HAIR_STYLES, (v) => ({ ...base, hair: v }), 'profile');
  row('hair colours and skin tones', ['black', 'dark-brown', 'brown', 'chestnut', 'blonde', 'red', 'grey', 'white'], (v) => ({ ...base, hair_color: v, hair: 'short' }), 'front');
  row('skin', ['porcelain', 'light', 'tan', 'olive', 'medium', 'brown', 'dark', 'deep'], (v) => ({ ...base, skin: v }), 'front');
  row('facial hair (front)', FACIAL_HAIR, (v) => ({ ...man, facial_hair: v }), 'front');
  row('facial hair (three quarter)', FACIAL_HAIR, (v) => ({ ...man, facial_hair: v }), 'three_quarter');
  row('facial hair (profile)', FACIAL_HAIR, (v) => ({ ...man, facial_hair: v }), 'profile');
  row('glasses', GLASSES, (v) => ({ ...base, glasses: v }), 'front');
  row('glasses (three quarter)', GLASSES, (v) => ({ ...base, glasses: v }), 'three_quarter');
  row('accessory (front)', ACCESSORIES, (v) => ({ ...base, hair: 'short', accessory: v }), 'front');
  row('accessory (three quarter)', ACCESSORIES, (v) => ({ ...base, hair: 'short', accessory: v }), 'three_quarter');
  row('accessory (profile)', ACCESSORIES, (v) => ({ ...base, hair: 'short', accessory: v }), 'profile');
  row('eyes / brows', ['round', 'almond', 'big'], (v) => ({ ...base, eyes: v, brows: v === 'round' ? 'soft' : v === 'almond' ? 'bold' : 'thin' }), 'front');
  // outfits and bodies are full-body rows
  const fw = 200;
  const fh = 480;
  inner += label(6, y + 24, 'outfits (three quarter, front)', 22);
  y += 30;
  Object.keys(OUTFITS).forEach((o, i) => {
    inner += cell(i * fw * 2, y, fw, fh, FULL, actor(rig, { ...base, outfit: o }, ctx, { view: 'three_quarter' }));
    inner += cell(i * fw * 2 + fw, y, fw, fh, FULL, actor(rig, { ...base, outfit: o }, ctx, { view: 'front' }));
    inner += label(i * fw * 2 + 4, y + fh + 16, o, 15);
  });
  y += fh + 30;
  inner += label(6, y + 24, 'bodies (front, three quarter, profile), outfit casual', 22);
  y += 30;
  BODY_NAMES.forEach((b, i) => {
    ['front', 'three_quarter', 'profile'].forEach((view, vi) => {
      inner += cell(i * fw * 2 + vi * (fw * 0.62), y, fw * 0.62, fh * 0.8, FULL, actor(rig, { ...base, body: b, outfit: 'casual' }, ctx, { view }));
    });
    inner += label(i * fw * 2 + 4, y + fh * 0.8 + 16, b, 15);
  });
  y += fh * 0.8 + 30;
  return { svg: svgDoc([0, 0, width, y], inner, ctx, { background: PAL.bg }), width, height: y };
}

const depthScale = (meta, y) => {
  const f = meta.floor;
  return f.scale_far + ((y - f.y_far) / (f.y_near - f.y_far)) * (f.scale_near - f.scale_far);
};

/** Props on both palettes, then the environments in day and night with actors standing on the spots. */
export function envPropSheet(rig) {
  const ctx = new Ctx();
  let inner = '';
  let y = 0;
  const names = Object.keys(PROPS);
  for (const pname of ['sunny', 'calm']) {
    const pal = PALETTES[pname];
    inner += `<rect x="0" y="${y}" width="1920" height="${Math.ceil(names.length / 8) * 250}" fill="${pal.bg}"/>`;
    names.forEach((n, i) => {
      const p = PROPS[n].build();
      const [, , w, h] = p.viewbox;
      const s = Math.min(200 / w, 200 / h);
      const x = (i % 8) * 240 + 20;
      const yy = y + Math.floor(i / 8) * 250 + 10;
      inner += `<g transform="translate(${x + (200 - w * s) / 2} ${yy + 200 - h * s}) scale(${s})">${propSvg(p, pal, ctx)}</g>${label(x, yy + 226, n, 16)}`;
    });
    y += Math.ceil(names.length / 8) * 250;
  }
  const ew = 960;
  const eh = 540;
  let i = 0;
  const envNames = Object.keys(ENVIRONMENTS);
  for (const pname of ['sunny', 'calm']) {
    const pal = PALETTES[pname];
    for (const variant of ['day', 'night']) {
      for (const n of envNames) {
        const env = ENVIRONMENTS[n]();
        const meta = ENV_META[n];
        let g = environmentSvg(env, variant, pal, ctx);
        const place = (spot, ch, pose, facing) => {
          const sp = meta.spots[spot];
          const sc = (rig.height * 1080 * depthScale(meta, sp.y)) / 1000;
          return placedCharacter(rig, CHARACTERS[ch].params, pal, ctx, { view: 'three_quarter', x: (sp.x / 100) * 1920, y: (sp.y / 100) * 1080, scale: sc, facing, pose, offset_y: pose.offset_y || 0, expression: NEUTRAL });
        };
        g += place(meta.spots.desk ? 'desk' : 'left', 'office-woman-1', meta.spots.desk ? POSES.typing : POSES.stand, 'right');
        g += place('center', 'office-man-1', POSES['point-right'], 'right');
        g += place('right', 'it-tech-1', POSES.stand, 'left');
        g += place('far-right', 'senior-man-1', POSES.present, 'left');
        inner += cell((i % 2) * ew, y + Math.floor(i / 2) * eh, ew, eh, [0, 0, 1920, 1080], g);
        inner += label((i % 2) * ew + 10, y + Math.floor(i / 2) * eh + 30, `${n} ${variant} ${pname}`, 26);
        i++;
      }
    }
  }
  y += Math.ceil(i / 2) * eh;
  return { svg: svgDoc([0, 0, 1920, y], inner, ctx, { background: '#888' }), width: 1920, height: y };
}
```

`scripts/build-storymaker-catalog.mjs`:

```js
// Builds the Storymaker starter catalog from the code-authored generators in assets/storymaker/.
//
//   node scripts/build-storymaker-catalog.mjs                  write ui/img/storymaker/** and ui/lang/storymaker_catalog/*.json
//   node scripts/build-storymaker-catalog.mjs --check          exit 1 when a committed output differs from the generators
//   node scripts/build-storymaker-catalog.mjs --review <dir>   render the review sheets (SVG + PNG through headless Chrome)
//                                                              optional: --chrome <path-to-chrome>
//
// The build is deterministic: no clocks, no randomness outside seeded generators, LF line endings.

import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { buildOutputs } from '../assets/storymaker/lib/catalog.mjs';
import { localeTables, LANGS } from '../assets/storymaker/lib/names.mjs';
import { validateOutputs } from '../assets/storymaker/lib/validate.mjs';
import { castSheet, varietySheet, envPropSheet } from '../assets/storymaker/lib/review.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const outDir = path.join(root, 'ui', 'img', 'storymaker');
const langDir = path.join(root, 'ui', 'lang', 'storymaker_catalog');
const SUBDIRS = ['rig', 'props', 'environments', 'thumbs'];

const lf = (s) => s.replace(/\r\n/g, '\n');

function expectedFiles() {
  const built = buildOutputs();
  const files = new Map();
  for (const [p, text] of built.files) files.set(path.join(outDir, ...p.split('/')), text);
  const tables = localeTables();
  for (const lang of LANGS) files.set(path.join(langDir, `${lang}.json`), JSON.stringify(tables[lang], null, 2) + '\n');
  return { files, built };
}

function listExisting() {
  const found = [];
  const walk = (dir) => {
    if (!fs.existsSync(dir)) return;
    for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) walk(p);
      else found.push(p);
    }
  };
  for (const d of SUBDIRS) walk(path.join(outDir, d));
  if (fs.existsSync(path.join(outDir, 'catalog.json'))) found.push(path.join(outDir, 'catalog.json'));
  walk(langDir);
  return found;
}

function write() {
  const { files, built } = expectedFiles();
  const problems = validateOutputs(built);
  if (problems.length) {
    console.error(problems.join('\n'));
    process.exit(1);
  }
  for (const [p, text] of files) {
    fs.mkdirSync(path.dirname(p), { recursive: true });
    fs.writeFileSync(p, text, 'utf8');
  }
  for (const p of listExisting()) if (!files.has(p)) fs.rmSync(p);
  let bytes = 0;
  for (const text of built.files.values()) bytes += Buffer.byteLength(text);
  console.log(`storymaker catalog ${built.manifest.version}: ${built.files.size} files, ${(bytes / 1048576).toFixed(2)} MiB in ui/img/storymaker, ${LANGS.length} locale files`);
}

function check() {
  const { files, built } = expectedFiles();
  const problems = validateOutputs(built);
  for (const [p, text] of files) {
    if (!fs.existsSync(p)) problems.push(`missing: ${path.relative(root, p)}`);
    else if (lf(fs.readFileSync(p, 'utf8')) !== text) problems.push(`differs: ${path.relative(root, p)}`);
  }
  for (const p of listExisting()) if (!files.has(p)) problems.push(`stale: ${path.relative(root, p)}`);
  if (problems.length) {
    console.error(problems.join('\n'));
    console.error('Run: node scripts/build-storymaker-catalog.mjs');
    process.exit(1);
  }
  console.log(`storymaker catalog ${built.manifest.version} is up to date`);
}

function findChrome(explicit) {
  const candidates = [
    explicit, process.env.CHROME_PATH,
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
    '/usr/bin/google-chrome', '/usr/bin/chromium', '/usr/bin/chromium-browser',
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  ].filter(Boolean);
  return candidates.find((c) => fs.existsSync(c));
}

function review(dir, chromeArg) {
  fs.mkdirSync(dir, { recursive: true });
  const { built } = expectedFiles();
  const sheets = [['cast-sheet', castSheet(built.rig)], ['variety-sheet', varietySheet(built.rig)], ['env-prop-sheet', envPropSheet(built.rig)]];
  const chrome = findChrome(chromeArg);
  for (const [name, sheet] of sheets) {
    const svgPath = path.resolve(dir, `${name}.svg`);
    fs.writeFileSync(svgPath, sheet.svg, 'utf8');
    console.log(`wrote ${svgPath} (${sheet.width} x ${sheet.height})`);
    if (!chrome) continue;
    const png = path.resolve(dir, `${name}.png`);
    const r = spawnSync(chrome, ['--headless=new', '--disable-gpu', '--hide-scrollbars', '--force-device-scale-factor=1', `--window-size=${sheet.width},${sheet.height}`, `--screenshot=${png}`, `file:///${svgPath.replace(/\\/g, '/')}`], { stdio: 'ignore' });
    console.log(r.status === 0 && fs.existsSync(png) ? `wrote ${png}` : `chrome failed for ${name}`);
  }
  if (!chrome) console.log('no Chrome found: pass --chrome <path> or set CHROME_PATH to also render PNGs');
}

const args = process.argv.slice(2);
if (args.includes('--check')) check();
else if (args.includes('--review')) review(path.resolve(args[args.indexOf('--review') + 1] || 'disposable/storymaker-review'), args.includes('--chrome') ? args[args.indexOf('--chrome') + 1] : undefined);
else write();
```


Generate and verify the outputs. The generated files are not part of this plan; the build writes them:

```bash
node scripts/build-storymaker-catalog.mjs
node scripts/build-storymaker-catalog.mjs --check
```

Expected: `storymaker catalog 70d5cf51a9a9: 105 files, 2.37 MiB in ui/img/storymaker, 16 locale files`, then `storymaker catalog 70d5cf51a9a9 is up to date`. A different hash means the generator code differs from this plan (or Node's `Math` functions differ in the last digit, which would be a determinism problem to fix before committing).

Look at the art once (keep the sheets outside the repository; `disposable/` is git-ignored):

```bash
node scripts/build-storymaker-catalog.mjs --review disposable/storymaker-review
```

This writes `cast-sheet`, `variety-sheet` and `env-prop-sheet` as SVG and, when Chrome is found (`--chrome <path>` or `CHROME_PATH`), as PNG. On the Windows dev box use the installed Chrome; no binary inside the repository can start it (see the rules in the contract).


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node --test assets/storymaker/test/catalog.test.mjs`
     `go test ./ui -run TestStorymakerCatalog -count=1`
Expected: PASS (node: 6 tests; go: 7 `TestStorymakerCatalog*` tests, `ok aurago/ui`). Also run `go test ./ui -run 'TestShippedWebAssetsLoadNothingExternal|TestTranslations_' -count=1` (PASS: the new files stay within the allowlist and the new locale directory passes the 16-language key and flat-structure tests).

- [ ] **Step 5: Commit**

```bash
git add assets/storymaker/lib/names.mjs assets/storymaker/lib/catalog.mjs assets/storymaker/lib/validate.mjs assets/storymaker/lib/review.mjs assets/storymaker/test/catalog.test.mjs scripts/build-storymaker-catalog.mjs ui/storymaker_catalog_test.go ui/img/storymaker ui/lang/storymaker_catalog
git commit -m "feat(storymaker): starter catalog generator, outputs and schema tests" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```


### Task D8: README and the final verification

**Files:**
- Create: `assets/storymaker/README.md`

The README records the style rules, the licence (original work by the AuraGo project; Geist is SIL OFL 1.1 and only referenced), the budgets, how to rebuild and check, and the rig conventions that part C and later catalog work rely on.

- [ ] **Step 1: Write the README**

`assets/storymaker/README.md`:

````markdown
# Storymaker starter catalog

Code-authored generators for the Storymaker starter catalog: one parametric human rig with four starter
characters, 23 props, four environments, poses, gestures, expressions, mouth shapes, walk and idle cycles, text
styles, transitions and two palettes. `scripts/build-storymaker-catalog.mjs` turns them into the committed files
below `ui/img/storymaker/` (format `storymaker-catalog/1`, spec section Catalog and rig).

The catalog is original work by the AuraGo project and ships under the repository licence. It contains no traced,
scanned or downloaded artwork. The default font is Geist (SIL OFL 1.1, already shipped in `ui/fonts`,
see `THIRD_PARTY_LIBRARIES.md`); the catalog only references it by path.

## Run

```bash
node scripts/build-storymaker-catalog.mjs                 # write ui/img/storymaker/** and ui/lang/storymaker_catalog/*.json
node scripts/build-storymaker-catalog.mjs --check         # exit 1 when a committed output differs from the generators
node scripts/build-storymaker-catalog.mjs --review <dir>  # review sheets as SVG and PNG (headless Chrome; --chrome <path>)
node --test assets/storymaker/test/geometry.test.mjs assets/storymaker/test/parts.test.mjs \
  assets/storymaker/test/rig.test.mjs assets/storymaker/test/motion.test.mjs assets/storymaker/test/props.test.mjs \
  assets/storymaker/test/environments.test.mjs assets/storymaker/test/catalog.test.mjs
go test ./ui -run TestStorymakerCatalog -count=1       # schema checks on the committed files
```

The build is deterministic: no clocks, no randomness outside seeded generators (`rng`), LF line endings, numbers
rounded to one decimal. `catalog.json` carries `version`, the first 12 hex digits of the SHA-256 over the
LF-normalised bytes of every file below `ui/img/storymaker/` except `catalog.json`, taken in sorted path order.
Run the build after every generator change and commit the outputs together with it.
`--review` writes `cast-sheet`, `variety-sheet` and `env-prop-sheet` (keep them outside the repository, for
example in the git-ignored `disposable/`).

## Layout

| Path | Role |
| --- | --- |
| `lib/util.mjs` | path builders, shape helpers (`fill`, `shade`, `light`, `ribbon`, `limb`), seeded `rng` |
| `lib/pathdata.mjs` | strict SVG path reader used by the validator and the tests |
| `lib/svg.mjs` | the section 3a renderer (bones, slots, variants, tokens) that draws thumbnails and review sheets |
| `lib/dims.mjs` | rig proportions and the five body types |
| `lib/headgeo.mjs`, `face.mjs`, `hair.mjs`, `extras.mjs` | head surface model, skull, eyes, brows, mouths, hair, glasses, facial hair, accessories |
| `lib/body.mjs`, `clothes.mjs`, `rig.mjs` | torso, limbs, hands, shoes, garments, the assembled `rig/human.json` |
| `lib/motion.mjs` | poses, gestures, walk and idle cycles |
| `lib/props.mjs`, `environments.mjs` | the 23 props and the four environments with spots and aspect overrides |
| `lib/names.mjs` | display names in the 16 locales |
| `lib/catalog.mjs`, `validate.mjs`, `review.mjs` | manifest, thumbnails and hash; build-time validation; review sheets |
| `test/*.test.mjs` | generator tests (`node --test`) |

Outputs: `catalog.json`, `rig/human.json`, `props/<name>.json`, `environments/<name>.json`,
`thumbs/<id>.svg` (one per entry; characters are the rig posed in `stand`, three-quarter view, resolved colours),
and `ui/lang/storymaker_catalog/<lang>.json` (keys `storymaker.catalog.<id with "." replaced by "_">`).
All outputs are `.json` or `.svg` below `ui/img` and `ui/lang`, which the asset-set allowlist ships
(`assets/web-assets.json`).

## Style rules

- Flat vector explainer look: clean rounded shapes, no outlines on silhouettes, a limited palette. Depth comes from
  one second tone (`shade`: the outline colour at low opacity) and an occasional light tone (`light`: white at low
  opacity), never from gradients on characters. Facial line work uses `$outline`.
- Characters use the tokens `$skin $hair $top $bottom $shoes $outline $eye_white $iris $lip`; props and
  environments use palette tokens (`$bg $bg2 $fg $accent $accent2 $text $outline`) plus a few fixed neutrals so they
  read on both palettes. Environments add local tokens defined for both `day` and `night`; the night variant may
  override `bg` and `bg2`.
- No external assets, fonts or images; no `<image>`, `<script>` or `href` in thumbnails.
- Strokes are avoided for anything that needs round caps: line work is drawn as filled ribbons (`ribbon`). The
  remaining strokes assume round caps and joins.

## Budgets

The catalog stays within 30 MiB and 2000 files (spec); it currently is about 2.4 MiB in 105 files.
`TestStorymakerCatalogFilesVersionAndBudget` enforces both limits and that every file below `ui/img/storymaker`
is referenced by `catalog.json`.

## Rig conventions (section 3a additions)

- Views face right. `_r` is the character's right limb: in the front view it is on the screen right (x > 0); in
  `three_quarter` and `profile` it is the near limb, drawn in front of the torso. Negative angles swing a limb
  towards +x (forward when facing right). Bone local frames are parallel to rig space at angle 0; limbs extend along
  local +y, torso, neck and head along local -y.
- Hands hang at local `y = 135` of the lower arms and feet at `y = 205` of the lower legs, for every body type, so
  body variants never move the end parts. Body types differ through rest offsets, shapes keyed by `body`, and `scale`.
- The slot `param` for back hair is `hair` (the same key as the front hair); no slot uses `hair_back` or `skin`.
- `pose.offset_y` (rig units, positive = down) shifts the root like a cycle's `offset_y`; seated poses use
  `rig.seat_drop` (215) so that the feet stay on the floor line.
- A shape draws its `fill` (or gradient) if present and its `stroke` if present; a shape with none of them is invalid.
  Gradient stop colours may be `$tokens`.
- Environment layers are listed in draw order (back to front); `parallax` is per layer and not monotonic.
- Aspect overrides: a 9:16 or 1:1 output shows a slice of the 16:9 design frame as wide as the output at the design
  height (31.6 % and 56.3 % of the width). `aspects[...].focus_x` is the slice centre and `spots` replaces the spots
  that would fall outside it.
- The `child` body is a small adult-proportioned figure; child-specific heads and garments are catalog expansion work.

## Adding to the catalog

1. Add the generator code (a prop in `lib/props.mjs`, a character in `CHARACTERS` of `lib/catalog.mjs`, ...).
2. Add its names to `lib/names.mjs` (all 16 languages, real translations) and the required id to `lib/validate.mjs`
   and `ui/storymaker_catalog_test.go` when it is part of a contract list.
3. Run the build, look at the review sheets, run the tests, commit generator and outputs together.
````


- [ ] **Step 2: Run the whole verification**

```bash
node scripts/build-storymaker-catalog.mjs --check
for f in assets/storymaker/test/*.test.mjs; do node --test "$f" || exit 1; done
go test ./ui -run 'TestStorymakerCatalog|TestShippedWebAssetsLoadNothingExternal|TestTranslations_' -count=1
```

Expected: `storymaker catalog 70d5cf51a9a9 is up to date`, 31 node tests pass (6 + 3 + 5 + 4 + 3 + 4 + 6 across the seven test files), `ok aurago/ui`.

- [ ] **Step 3: Commit**

```bash
git add assets/storymaker/README.md
git commit -m "docs(storymaker): catalog generator README" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

