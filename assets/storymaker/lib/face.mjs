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
