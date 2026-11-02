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
