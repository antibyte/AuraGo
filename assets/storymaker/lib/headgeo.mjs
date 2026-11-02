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
