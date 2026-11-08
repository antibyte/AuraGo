// Poses, gestures and motion cycles for the human rig.
//
// Angles are degrees added to the bone's rest angle (poses) or on top of the pose (gestures, cycles).
// Negative swings a limb towards +x, which is forward for a right-facing character. "_r" is the near limb in the
// three_quarter and profile views and sits on the screen right in the front view.
// The bitmap rig fades out base arm angles while gesture arm targets take over; legacy vector evaluation stays additive.
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

