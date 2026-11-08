// https://gabormakesgames.com/blog_transforms_transforms.html
// https://gabormakesgames.com/blog_transforms_transform_world.html

import Quat from './Quat'
import Vec3 from './Vec3'

export default class Transform {
  // #region MAIN
  public rot: Quat = new Quat(0, 0, 0, 1)
  public pos: Vec3 = new Vec3()
  public scl: Vec3 = new Vec3(1, 1, 1)

  /**
   * There are multiple constructor formats, so the parameters look a bit odd.
   * @param rot  - can be a Quat or a Transform to copy from
   * @param pos - position vector if rot is a Quat
   * @param scl  - scale vector if rot is a Quat
   */
  constructor (rot?: Quat | Transform, pos?: Vec3, scl?: Vec3) {
    if (rot instanceof Transform) {
      this.copy(rot)
    } else if ((rot != null) && (pos != null) && (scl != null)) {
      this.set(rot, pos, scl)
    }
  }
  // #endregion

  // #region SETTERS / GETTERS
  copy (t: Transform): this {
    this.rot[0] = t.rot[0]
    this.rot[1] = t.rot[1]
    this.rot[2] = t.rot[2]
    this.rot[3] = t.rot[3]

    this.pos[0] = t.pos[0]
    this.pos[1] = t.pos[1]
    this.pos[2] = t.pos[2]

    this.scl[0] = t.scl[0]
    this.scl[1] = t.scl[1]
    this.scl[2] = t.scl[2]
    return this
  }

  set (r?: Quat, p?: Vec3, s?: Vec3): this {
    if (r != null) {
      this.rot[0] = r[0]
      this.rot[1] = r[1]
      this.rot[2] = r[2]
      this.rot[3] = r[3]
    }
    if (p != null) {
      this.pos[0] = p[0]
      this.pos[1] = p[1]
      this.pos[2] = p[2]
    }
    if (s != null) {
      this.scl[0] = s[0]
      this.scl[1] = s[1]
      this.scl[2] = s[2]
    }
    return this
  }

  clone (): Transform {
    return new Transform(this)
  }
  // #endregion

  // #region OPERATIONS
  mul (tran: Transform): this {
    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // POSITION - parent.position + ( parent.rotation * ( parent.scale * child.position ) )
    const p = new Vec3(
      this.scl[0] * tran.pos[0], // Scale
      this.scl[1] * tran.pos[1],
      this.scl[2] * tran.pos[2]
    )

    qTransform(this.rot, p, p) // Rotation
    this.pos[0] += p[0] // Translation
    this.pos[1] += p[1]
    this.pos[2] += p[2]

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // SCALE - parent.scale * child.scale
    this.scl[0] *= tran.scl[0]
    this.scl[1] *= tran.scl[1]
    this.scl[2] *= tran.scl[2]

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // ROTATION - parent.rotation * child.rotation
    qMul(this.rot, tran.rot, this.rot)

    return this
  }

  // Computing Transforms in reverse, Child - > Parent
  pmul (tran: Transform): this {
    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // POSITION - parent.position + ( parent.rotation * ( parent.scale * child.position ) )
    // The only difference for this func, We use the IN.scl & IN.rot instead of THIS.scl * THIS.rot
    // Consider that this Object is the child and the input is the Parent.

    // Scale
    this.pos[0] *= tran.scl[0]
    this.pos[1] *= tran.scl[1]
    this.pos[2] *= tran.scl[2]

    // Rotation
    qTransform(tran.rot, this.pos, this.pos)

    // Translation
    this.pos[0] += tran.pos[0]
    this.pos[1] += tran.pos[1]
    this.pos[2] += tran.pos[2]

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // SCALE - parent.scale * child.scale
    this.scl[0] *= tran.scl[0]
    this.scl[1] *= tran.scl[1]
    this.scl[2] *= tran.scl[2]

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // ROTATION - parent.rotation * child.rotation
    qMul(tran.rot, this.rot, this.rot) // Must Rotate from Parent->Child, need PMUL
    return this
  }
  // #endregion

  // #region FROM OPERATORS

  /**
   * combining hierarchical transformations, such as when calculating the world transform
   * of a child object given its local transform and its parent’s world transform.
   * @param transform_parent transform of the parent
   * @param transform_child transform of the child
   * @returns instance for chaining. Also mutates the original transform that calls this method
   */
  fromMul (transform_parent: Transform, transform_child: Transform): this {
    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // POSITION - parent.position + (  ( parent.scale * child.position ) * parent.rotation )
    const v = new Vec3( // parent.scale * child.position
      transform_parent.scl[0] * transform_child.pos[0],
      transform_parent.scl[1] * transform_child.pos[1],
      transform_parent.scl[2] * transform_child.pos[2]
    )

    qTransform(transform_parent.rot, v, v) // * parent.rotation
    this.pos[0] = transform_parent.pos[0] + v[0] // parent.position +
    this.pos[1] = transform_parent.pos[1] + v[1]
    this.pos[2] = transform_parent.pos[2] + v[2]

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // SCALE - parent.scale * child.scale
    this.scl[0] = transform_parent.scl[0] * transform_child.scl[0]
    this.scl[1] = transform_parent.scl[1] * transform_child.scl[1]
    this.scl[2] = transform_parent.scl[2] * transform_child.scl[2]

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // ROTATION - parent.rotation * child.rotation
    // this.rot.fromMul( tp.rot, tc.rot );
    qMul(transform_parent.rot, transform_child.rot, this.rot)

    return this
  }

  fromInvert (t: Transform): this {
    // Invert Rotation
    qInvert(t.rot, this.rot)

    // Invert Scale
    this.scl[0] = 1 / t.scl[0]
    this.scl[1] = 1 / t.scl[1]
    this.scl[2] = 1 / t.scl[2]

    // NOTE: This doesn't seem to work in practice when
    // dealing with scaling and dealing with vec3 transform
    // between world > local. Just negate pos seems to work

    // Invert Position : rotInv * ( invScl * -Pos )
    // this.pos
    //     .fromNegate( t.pos )
    //     .mul( this.scl )
    // //         // .transformQuat( this.rot );

    this.pos[0] = -t.pos[0] * this.scl[0]
    this.pos[1] = -t.pos[1] * this.scl[1]
    this.pos[2] = -t.pos[2] * this.scl[2]
    qTransform(this.rot, this.pos, this.pos)

    // this.pos[0] = -t.pos[0];
    // this.pos[1] = -t.pos[1];
    // this.pos[2] = -t.pos[2];

    return this
  }
  // #endregion

  // #region TRANSFORMATION

  // Regular Applying transform, Does not work well for inversed transforms
  // when dealing with World to Local Transformation
  transformVec3 (v: Vec3, out: Vec3 = new Vec3()): Vec3 {
    // GLSL - vecQuatRotation(model.rotation, a_position.xyz * model.scale) + model.position;

    // Vector * Scale
    const vx = v[0] * this.scl[0]
    const vy = v[1] * this.scl[1]
    const vz = v[2] * this.scl[2]

    // ( Rotation * Vector3 ) + Translation
    const qx = this.rot[0]
    const qy = this.rot[1]
    const qz = this.rot[2]
    const qw = this.rot[3]
    const x1 = qy * vz - qz * vy
    const y1 = qz * vx - qx * vz
    const z1 = qx * vy - qy * vx
    const x2 = qw * x1 + qy * z1 - qz * y1
    const y2 = qw * y1 + qz * x1 - qx * z1
    const z2 = qw * z1 + qx * y1 - qy * x1

    out[0] = (vx + 2 * x2) + this.pos[0] // + Translation
    out[1] = (vy + 2 * y2) + this.pos[1]
    out[2] = (vz + 2 * z2) + this.pos[2]

    return out
  }

  // When using an inversed transform, use this to transform
  // WorldSpace vectors to local space
  transformVec3Rev (v: Vec3, out: Vec3 = new Vec3()): Vec3 {
    // Translation
    const vx = v[0] + this.pos[0]
    const vy = v[1] + this.pos[1]
    const vz = v[2] + this.pos[2]

    // ( Rotation * Vector3 ) * scale
    const qx = this.rot[0]
    const qy = this.rot[1]
    const qz = this.rot[2]
    const qw = this.rot[3]
    const x1 = qy * vz - qz * vy
    const y1 = qz * vx - qx * vz
    const z1 = qx * vy - qy * vx
    const x2 = qw * x1 + qy * z1 - qz * y1
    const y2 = qw * y1 + qz * x1 - qx * z1
    const z2 = qw * z1 + qx * y1 - qy * x1

    out[0] = (vx + 2 * x2) * this.scl[0]
    out[1] = (vy + 2 * y2) * this.scl[1]
    out[2] = (vz + 2 * z2) * this.scl[2]
    return out
  }

  toLocalPos (wp: Vec3, out: Vec3 = new Vec3()): Vec3 {
    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // Invert Transform Rot & Scl
    const qi = qInvert(this.rot)
    const si = new Vec3(
      1 / this.scl[0],
      1 / this.scl[1],
      1 / this.scl[2]
    )

    // Invert Transform Pos
    const pi = new Vec3(
      -this.pos[0] * si[0],
      -this.pos[1] * si[1],
      -this.pos[2] * si[2]
    )

    qTransform(qi, pi, pi)

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // Invert Transform on WorldSpace Position
    // invertRot * ( invertScl * WPos ) + invertPos;

    const p = new Vec3(
      si[0] * wp[0],
      si[1] * wp[1],
      si[2] * wp[2]
    )

    qTransform(qi, p, p)

    out[0] = pi[0] + p[0]
    out[1] = pi[1] + p[1]
    out[2] = pi[2] + p[2]
    return out
  }

  toLocalRot (wq: Quat, out: Quat = new Quat(0, 0, 0, 1)): number[] {
    return qMul(qInvert(this.rot), wq, out)
  }

  // #endregion
}

// #region INDEPENDANCE FROM VEC3/QUAT
function qTransform (q: Quat, v: Vec3, out: Vec3 = new Vec3()): Vec3 {
  const qx = q.x
  const qy = q.y
  const qz = q.z
  const qw = q.w

  const vx = v.x
  const vy = v.y
  const vz = v.z

  const x1 = qy * vz - qz * vy
  const y1 = qz * vx - qx * vz
  const z1 = qx * vy - qy * vx
  const x2 = qw * x1 + qy * z1 - qz * y1
  const y2 = qw * y1 + qz * x1 - qx * z1
  const z2 = qw * z1 + qx * y1 - qy * x1

  // set the output vector3
  out.x = vx + 2 * x2
  out.y = vy + 2 * y2
  out.z = vz + 2 * z2
  return out
}

function qInvert (q: Quat, out: Quat = new Quat(0, 0, 0, 1)): Quat {
  const a0 = q[0]
  const a1 = q[1]
  const a2 = q[2]
  const a3 = q[3]
  const dot = a0 * a0 + a1 * a1 + a2 * a2 + a3 * a3

  if (dot === 0) { out[0] = out[1] = out[2] = out[3] = 0; return out }

  const inverse_dot_product = 1.0 / dot // let invDot = dot ? 1.0/dot : 0;
  out[0] = -a0 * inverse_dot_product
  out[1] = -a1 * inverse_dot_product
  out[2] = -a2 * inverse_dot_product
  out[3] = a3 * inverse_dot_product
  return out
}

function qMul (a: Quat, b: Quat, out: Quat = new Quat(0, 0, 0, 1)): Quat {
  const ax = a[0]; const ay = a[1]; const az = a[2]; const aw = a[3]
  const bx = b[0]; const by = b[1]; const bz = b[2]; const bw = b[3]
  out[0] = ax * bw + aw * bx + ay * bz - az * by
  out[1] = ay * bw + aw * by + az * bx - ax * bz
  out[2] = az * bw + aw * bz + ax * by - ay * bx
  out[3] = aw * bw - ax * bx - ay * by - az * bz
  return out
}
// #endregion
