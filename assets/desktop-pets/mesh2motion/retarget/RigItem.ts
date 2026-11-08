import { type Joint } from './Joint'
import Quat from './Quat'
import Vec3 from './Vec3'

// Rig data about a single joint
export class RigItem {
  public idx: number = -1 // Joint Index
  public pidx: number = -1 // Parent Joint Index
  public readonly swing: Vec3 = new Vec3(0, 0, 1) // Swing Direction - Z
  public readonly twist: Vec3 = new Vec3(0, 1, 0) // Twist Direction - Y

  /**
   * Initializes the RigItem from a Joint, setting its index, parent index,
   * and computing the swing and twist directions based on the joint's world rotation.
   * @param j - The Joint to initialize from.
   * @param swing - Optional swing direction vector.
   * @param twist - Optional twist direction vector.
   * @returns The current RigItem instance for chaining.
   */
  public fromJoint (j: Joint, swing: Vec3 | null = null, twist: Vec3 | null = null): this {
    this.idx = j.index
    this.pidx = j.pindex

    // Compute inverse direction on the current joint rotation
    if (swing !== null || twist !== null) {
      const joint_world_rotation: Quat = j.world.rot
      const q = new Quat().fromInvert(joint_world_rotation)
      if (swing !== null) this.swing.fromQuat(q, swing)
      if (twist !== null) this.twist.fromQuat(q, twist)
    }

    return this
  }
}
