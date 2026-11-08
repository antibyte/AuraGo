/**
 * Shared types for the bone auto-mapping system.
 *
 * These live in their own module (rather than in BoneAutoMapper) so the
 * tokenizer / vocabulary / resolver modules can use them without creating an
 * import cycle back through BoneAutoMapper. BoneAutoMapper re-exports all three
 * so existing `import { BoneSide } from './BoneAutoMapper'` call sites keep working.
 */

/**
 * Bone categories for grouping bones by anatomical area
 */
export enum BoneCategory {
  Torso = 'torso',
  Arms = 'arms',
  Hands = 'hands',
  Legs = 'legs',
  Wings = 'wings',
  Tail = 'tail',
  Unknown = 'unknown'
}

/**
 * Side of the body a bone belongs to.
 * Unknown means "the name carried no side information" - which is different from
 * Center ("this bone is deliberately on the midline"). Side inheritance in
 * BoneChainResolver only fills in Unknown, never overwrites Center.
 */
export enum BoneSide {
  Left = 'left',
  Right = 'right',
  Center = 'center',
  Unknown = 'unknown'
}

/**
 * Canonical humanoid joint slots. Every bone - source or target - is resolved to
 * one of these, and mapping is then a lookup on (slot, side, ordinal) rather than
 * a comparison of two rig-specific bone names.
 */
export enum BoneSlot {
  Pelvis = 'pelvis',
  Spine = 'spine',
  Neck = 'neck',
  Head = 'head',
  Clavicle = 'clavicle',
  UpperArm = 'upperarm',
  LowerArm = 'lowerarm',
  Hand = 'hand',
  Thigh = 'thigh',
  Calf = 'calf',
  Foot = 'foot',
  Ball = 'ball',
  FingerThumb = 'finger.thumb',
  FingerIndex = 'finger.index',
  FingerMiddle = 'finger.middle',
  FingerRing = 'finger.ring',
  FingerPinky = 'finger.pinky',
  Tail = 'tail',
  Wing = 'wing',
  Unknown = 'unknown'
}

/**
 * Metadata extracted from a bone name plus its place in the skeleton hierarchy
 */
export interface BoneMetadata {
  name: string // Original bone name
  normalized_name: string // Normalized version for matching
  side: BoneSide // Which side of the body
  category: BoneCategory // Anatomical category
  parent_name: string | null // Name of parent bone, null if this bone has no bone parent
  depth: number // Distance from the root bone (root = 0)
  children_names: string[] // Names of direct child bones
  slot: BoneSlot // Resolved canonical joint slot
  slot_ordinal: number // 1-based position within its (slot, side) chain; 0 until resolved
  is_leaf_name: boolean // Name carried an end/tip/leaf/nub marker
}
