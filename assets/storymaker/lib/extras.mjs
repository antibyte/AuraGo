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
