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
