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
