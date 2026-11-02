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
