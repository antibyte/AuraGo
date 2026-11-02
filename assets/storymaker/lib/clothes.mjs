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
