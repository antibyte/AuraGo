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
