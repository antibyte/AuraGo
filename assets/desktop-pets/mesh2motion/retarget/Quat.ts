import Vec3 from './Vec3.js'

// Since this extending Array, don't need to create members for x,y,z,w
// they are implicitly created by the Array base class.
export default class Quat extends Array {
  // optional copy contructor
  constructor (v: Quat | number | null = null, y?: number, z?: number, w?: number) {
    super(4)

    if (typeof v === 'number' && y !== undefined && z !== undefined && w !== undefined) {
      this[0] = v
      this[1] = y
      this[2] = z
      this[3] = w
      return
    }

    if (v === null) {
      this.identity()
      return
    }

    if (typeof v === 'object' && v?.length === 4) {
      this.copy(v)
    }
  }

  // #region SETTERS
  identity (): this {
    this[0] = 0
    this[1] = 0
    this[2] = 0
    this[3] = 1
    return this
  }

  // typescript enforce 4 elements for array
  copy (v: Quat): this {
    this[0] = v[0]
    this[1] = v[1]
    this[2] = v[2]
    this[3] = v[3]
    return this
  }

  // typescript enforce 4 elements for array
  copyTo (v: Quat): this {
    v[0] = this[0]
    v[1] = this[1]
    v[2] = this[2]
    v[3] = this[3]
    return this
  }

  // copyObj...but just pass in the individual components
  xyzw (x: number, y: number, z: number, w: number): this {
    this[0] = x
    this[1] = y
    this[2] = z
    this[3] = w
    return this
  }

  // object needs to contain x,y,z,w keys. Ok to have other properties since
  // we only care about these four.
  copyObj (o: { x: number, y: number, z: number, w: number, [key: string]: unknown }): this {
    this[0] = o.x
    this[1] = o.y
    this[2] = o.z
    this[3] = o.w
    return this
  }
  // #endregion

  // #region GETTERS

  // typesafe aliases for components
  get x (): number { return this[0] }
  get y (): number { return this[1] }
  get z (): number { return this[2] }
  get w (): number { return this[3] }

  clone (): Quat { return new Quat(this) }
  // #endregion

  // #region OPERATIONS
  /** Multiple Quaternion onto this Quaternion */
  mul (q: Quat): this {
    const ax = this[0]; const ay = this[1]; const az = this[2]; const aw = this[3]
    const bx = q[0]; const by = q[1]; const bz = q[2]; const bw = q[3]
    this[0] = ax * bw + aw * bx + ay * bz - az * by
    this[1] = ay * bw + aw * by + az * bx - ax * bz
    this[2] = az * bw + aw * bz + ax * by - ay * bx
    this[3] = aw * bw - ax * bx - ay * by - az * bz
    return this
  }

  /** PreMultiple Quaternions onto this Quaternion */
  pmul (q: Quat): this {
    const ax = q[0]; const ay = q[1]; const az = q[2]; const aw = q[3]
    const bx = this[0]; const by = this[1]; const bz = this[2]; const bw = this[3]
    this[0] = ax * bw + aw * bx + ay * bz - az * by
    this[1] = ay * bw + aw * by + az * bx - ax * bz
    this[2] = az * bw + aw * bz + ax * by - ay * bx
    this[3] = aw * bw - ax * bx - ay * by - az * bz
    return this
  }

  /** Normalize the quaternion, making it a unit quaternion 0-1 range for each component
   *  Important for proper rotation behavior since unit quaternions represent valid rotations */
  norm (): this {
    let len = (this[0] ** 2) + (this[1] ** 2) + (this[2] ** 2) + (this[3] ** 2)
    if (len > 0) {
      len = 1 / Math.sqrt(len)
      this[0] *= len
      this[1] *= len
      this[2] *= len
      this[3] *= len
    }
    return this
  }

  invert (): this {
    const a0 = this[0]
    const a1 = this[1]
    const a2 = this[2]
    const a3 = this[3]

    // dot product for quaternion
    const dot = (a0 * a0) + (a1 * a1) + (a2 * a2) + (a3 * a3)

    // handle zero length quaternion.
    // inverting a zero length quat is not valid.
    if (dot === 0) {
      this[0] = this[1] = this[2] = this[3] = 0
      console.warn('Quat.invert(): zero length quaternion, cannot invert.')
      return this
    }

    const inverted_dot_product = 1.0 / dot
    this[0] = -a0 * inverted_dot_product
    this[1] = -a1 * inverted_dot_product
    this[2] = -a2 * inverted_dot_product
    this[3] = a3 * inverted_dot_product
    return this
  }
  // #endregion

  // #region SPECIAL OPERATORS
  /** Inverts the quaternion passed in, then pre multiplies to this quaternion. */
  pmulInvert (q: Quat): this {
    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // q.invert()
    let ax = q[0]
    let ay = q[1]
    let az = q[2]
    let aw = q[3]

    const dot = ax * ax + ay * ay + az * az + aw * aw

    if (dot === 0) {
      ax = ay = az = aw = 0
    } else {
      const dot_inv = 1.0 / dot
      ax = -ax * dot_inv
      ay = -ay * dot_inv
      az = -az * dot_inv
      aw = aw * dot_inv
    }

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // Quat.mul( a, b );
    const bx = this[0]
    const by = this[1]
    const bz = this[2]
    const bw = this[3]
    this[0] = ax * bw + aw * bx + ay * bz - az * by
    this[1] = ay * bw + aw * by + az * bx - ax * bz
    this[2] = az * bw + aw * bz + ax * by - ay * bx
    this[3] = aw * bw - ax * bx - ay * by - az * bz
    return this
  }

  /**
   * Pre-multiplies this quaternion by a rotation defined by an axis and angle.
   * Commonly used for applying incremental rotations to an existing orientation.
   */
  pmulAxisAngle (axis: Vec3, rad: number): this {
    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // Quat.AxisAngle()
    const half = rad * 0.5
    const s = Math.sin(half)
    const ax = axis[0] * s
    const ay = axis[1] * s
    const az = axis[2] * s
    const aw = Math.cos(half)

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // Quat.mul( a, b );
    const bx = this[0]
    const by = this[1]
    const bz = this[2]
    const bw = this[3]
    this[0] = (ax * bw) + (aw * bx) + (ay * bz) - (az * by)
    this[1] = (ay * bw) + (aw * by) + (az * bx) - (ax * bz)
    this[2] = (az * bw) + (aw * bz) + (ax * by) - (ay * bx)
    this[3] = (aw * bw) - (ax * bx) - (ay * by) - (az * bz)
    return this
  }

  /** Pre-multiplies this quaternion by the shortest rotation from vector a to vector b */
  pmulSwing (a: Vec3, b: Vec3): this {
    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // fromSwing
    const dot = Vec3.dot(a, b)
    if (dot < -0.999999) { // 180 opposites
      const tmp = new Vec3().fromCross(new Vec3(-1, 0, 0), a)

      if (tmp.len < 0.000001) tmp.fromCross(new Vec3(0, 1, 0), a)
      this.pmulAxisAngle(tmp.norm(), Math.PI)
      return this
    } else if (dot > 0.999999) { // Same Direction
      return this // Creates identity, so exist early
    }

    const v = Vec3.cross(a, b, new Vec3(0, 0, 0))
    let ax = v[0]
    let ay = v[1]
    let az = v[2]
    let aw = 1 + dot

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // Normalize
    let len = ax ** 2 + ay ** 2 + az ** 2 + aw ** 2
    if (len > 0) {
      len = 1 / Math.sqrt(len)
      ax *= len
      ay *= len
      az *= len
      aw *= len
    }

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // Dot Negate
    const dot2 = this[0] * ax + this[1] * ay + this[2] * az + this[3] * aw
    if (dot2 < 0) {
      ax = -ax
      ay = -ay
      az = -az
      aw = -aw
    }

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // Quat.mul( a, b );
    const bx = this[0]
    const by = this[1]
    const bz = this[2]
    const bw = this[3]
    this[0] = ax * bw + aw * bx + ay * bz - az * by
    this[1] = ay * bw + aw * by + az * bx - ax * bz
    this[2] = az * bw + aw * bz + ax * by - ay * bx
    this[3] = aw * bw - ax * bx - ay * by - az * bz

    return this
  }

  dotNegate (chk: Quat): this {
    // quat.dot
    const dot = this[0] * chk[0] +
                    this[1] * chk[1] +
                    this[2] * chk[2] +
                    this[3] * chk[3]

    if (dot < 0) {
      // quat.negate
      this[0] = -this[0]
      this[1] = -this[1]
      this[2] = -this[2]
      this[3] = -this[3]
    }

    return this
  }
  // #endregion

  // #region FROM OPS
  fromMul (a: Quat, b: Quat): this {
    const ax = a[0]
    const ay = a[1]
    const az = a[2]
    const aw = a[3]

    const bx = b[0]
    const by = b[1]
    const bz = b[2]
    const bw = b[3]

    this[0] = ax * bw + aw * bx + ay * bz - az * by
    this[1] = ay * bw + aw * by + az * bx - ax * bz
    this[2] = az * bw + aw * bz + ax * by - ay * bx
    this[3] = aw * bw - ax * bx - ay * by - az * bz
    return this
  }

  fromInvert (q: Quat): this {
    const a0 = q[0]
    const a1 = q[1]
    const a2 = q[2]
    const a3 = q[3]
    const dot = a0 * a0 + a1 * a1 + a2 * a2 + a3 * a3

    if (dot === 0) {
      this[0] = this[1] = this[2] = this[3] = 0
      return this
    }

    const inverted_dot_product: number = 1.0 / dot // let invDot = dot ? 1.0/dot : 0;
    this[0] = -a0 * inverted_dot_product
    this[1] = -a1 * inverted_dot_product
    this[2] = -a2 * inverted_dot_product
    this[3] = a3 * inverted_dot_product
    return this
  }

  fromPolar (lon: number, lat: number, up: Vec3 | null = null): this {
    lat = Math.max(Math.min(lat, 89.999999), -89.999999) // Clamp lat, going to 90+ makes things spring around.

    const phi = (90 - lat) * 0.01745329251 // PI / 180
    const theta = lon * 0.01745329251
    const phi_s = Math.sin(phi)

    const v: Vec3 = [
      -(phi_s * Math.sin(theta)),
      Math.cos(phi),
      phi_s * Math.cos(theta)
    ] as Vec3

    this.fromLook(v, up || new Vec3(0, 1, 0))
    return this
  }

  fromLook (fwd: Vec3, up: Vec3 = new Vec3(0, 1, 0)): this {
    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // Orthogonal axes to make a mat3x3
    const zAxis = new Vec3(fwd)
    const xAxis = new Vec3().fromCross(up, zAxis).norm() // Right

    // Z & UP are parallel
    if (xAxis.lenSqr === 0) {
      if (Math.abs(up[2]) === 1) zAxis[0] += 0.0001 // shift x when Fwd or Bak
      else zAxis[2] += 0.0001 // shift z

      zAxis.norm() // ReNormalize
      xAxis.fromCross(up, zAxis).norm() // Redo Right
    }

    const yAxis = new Vec3().fromCross(zAxis, xAxis).norm() // Up
    const m = [...xAxis, ...yAxis, ...zAxis]

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    // Mat3 to Quat
    // Algorithm in Ken Shoemake's article in 1987 SIGGRAPH course notes
    // article "Quat Calculus and Fast Animation".
    let fRoot
    const fTrace = m[0] + m[4] + m[8] // Diagonal axis

    if (fTrace > 0.0) {
      // |w| > 1/2, may as well choose w > 1/2
      fRoot = Math.sqrt(fTrace + 1.0) // 2w
      this[3] = 0.5 * fRoot

      fRoot = 0.5 / fRoot // 1/(4w)
      this[0] = (m[5] - m[7]) * fRoot
      this[1] = (m[6] - m[2]) * fRoot
      this[2] = (m[1] - m[3]) * fRoot
    } else {
      // |w| <= 1/2
      let i = 0
      if (m[4] > m[0]) i = 1
      if (m[8] > m[i * 3 + i]) i = 2

      const j = (i + 1) % 3
      const k = (i + 2) % 3

      fRoot = Math.sqrt(m[i * 3 + i] - m[j * 3 + j] - m[k * 3 + k] + 1.0)
      this[i] = 0.5 * fRoot
      fRoot = 0.5 / fRoot
      this[3] = (m[j * 3 + k] - m[k * 3 + j]) * fRoot
      this[j] = (m[j * 3 + i] + m[i * 3 + j]) * fRoot
      this[k] = (m[k * 3 + i] + m[i * 3 + k]) * fRoot
    }

    return this
  }

  /** Using unit vectors, Shortest swing rotation from Direction A to Direction B  */
  fromSwing (a: Vec3, b: Vec3): this {
    // http://physicsforgames.blogspot.com/2010/03/Quat-tricks.html
    const dot = Vec3.dot(a, b)

    if (dot < -0.999999) { // 180 opposites
      const tmp = new Vec3().fromCross(new Vec3(-1, 0, 0), a)

      if (tmp.len < 0.000001) tmp.fromCross(new Vec3(0, 1, 0), a)
      this.fromAxisAngle(tmp.norm(), Math.PI)
    } else if (dot > 0.999999) { // Same Direction
      this[0] = 0
      this[1] = 0
      this[2] = 0
      this[3] = 1
    } else {
      const v = Vec3.cross(a, b, new Vec3(0, 0, 0))
      this[0] = v[0]
      this[1] = v[1]
      this[2] = v[2]
      this[3] = 1 + dot
      this.norm()
    }

    return this
  }

  /** Axis must be normlized, Angle in Radians  */
  fromAxisAngle (axis: Vec3, rad: number): this {
    const half = rad * 0.5
    const s = Math.sin(half)
    this[0] = axis[0] * s
    this[1] = axis[1] * s
    this[2] = axis[2] * s
    this[3] = Math.cos(half)
    return this
  }

  fromAxes (xAxis: Vec3, yAxis: Vec3, zAxis: Vec3): this {
    const m00: number = xAxis[0]
    const m01: number = xAxis[1]
    const m02: number = xAxis[2]

    const m10: number = yAxis[0]
    const m11: number = yAxis[1]
    const m12: number = yAxis[2]

    const m20: number = zAxis[0]
    const m21: number = zAxis[1]
    const m22: number = zAxis[2]

    const t: number = m00 + m11 + m22
    let x: number, y: number, z: number, w: number, s: number

    if (t > 0.0) {
      s = Math.sqrt(t + 1.0)
      w = s * 0.5 // |w| >= 0.5
      s = 0.5 / s
      x = (m12 - m21) * s
      y = (m20 - m02) * s
      z = (m01 - m10) * s
    } else if ((m00 >= m11) && (m00 >= m22)) {
      s = Math.sqrt(1.0 + m00 - m11 - m22)
      x = 0.5 * s// |x| >= 0.5
      s = 0.5 / s
      y = (m01 + m10) * s
      z = (m02 + m20) * s
      w = (m12 - m21) * s
    } else if (m11 > m22) {
      s = Math.sqrt(1.0 + m11 - m00 - m22)
      y = 0.5 * s // |y| >= 0.5
      s = 0.5 / s
      x = (m10 + m01) * s
      z = (m21 + m12) * s
      w = (m20 - m02) * s
    } else {
      s = Math.sqrt(1.0 + m22 - m00 - m11)
      z = 0.5 * s // |z| >= 0.5
      s = 0.5 / s
      x = (m20 + m02) * s
      y = (m21 + m12) * s
      w = (m01 - m10) * s
    }

    this[0] = x
    this[1] = y
    this[2] = z
    this[3] = w
    return this
  }
  // #endregion

  // #region ROTATIONS
  rotX (rad: number): this {
    // https://github.com/toji/gl-matrix/blob/master/src/gl-matrix/quat.js
    rad *= 0.5

    const ax = this[0]; const ay = this[1]; const az = this[2]; const aw = this[3]
    const bx = Math.sin(rad); const bw = Math.cos(rad)

    this[0] = ax * bw + aw * bx
    this[1] = ay * bw + az * bx
    this[2] = az * bw - ay * bx
    this[3] = aw * bw - ax * bx
    return this
  }

  rotY (rad: number): this {
    rad *= 0.5

    const ax = this[0]; const ay = this[1]; const az = this[2]; const aw = this[3]
    const by = Math.sin(rad); const bw = Math.cos(rad)

    this[0] = ax * bw - az * by
    this[1] = ay * bw + aw * by
    this[2] = az * bw + ax * by
    this[3] = aw * bw - ay * by
    return this
  }

  rotZ (rad: number): this {
    rad *= 0.5

    const ax = this[0]; const ay = this[1]; const az = this[2]; const aw = this[3]
    const bz = Math.sin(rad)
    const bw = Math.cos(rad)

    this[0] = ax * bw + ay * bz
    this[1] = ay * bw - ax * bz
    this[2] = az * bw + aw * bz
    this[3] = aw * bw - az * bz
    return this
  }
  // #endregion

  // #region CONVERT

  /* Convert a 3x3 rotation matrix to a quaternion
   * @param m - 3x3 rotation matrix as a flat number array (length 9)
   * @returns This quaternion set to represent the rotation of the matrix
   */
  fromMat3 (m: number[]): this {
    // https://github.com/toji/gl-matrix/blob/master/src/gl-matrix/quat.js#L305
    // Algorithm in Ken Shoemake's article in 1987 SIGGRAPH course notes
    // article "Quat Calculus and Fast Animation".
    let fRoot: number
    const fTrace: number = m[0] + m[4] + m[8]

    if (fTrace > 0.0) {
      // |w| > 1/2, may as well choose w > 1/2
      fRoot = Math.sqrt(fTrace + 1.0) // 2w
      this[3] = 0.5 * fRoot

      fRoot = 0.5 / fRoot // 1/(4w)
      this[0] = (m[5] - m[7]) * fRoot
      this[1] = (m[6] - m[2]) * fRoot
      this[2] = (m[1] - m[3]) * fRoot
    } else {
      // |w| <= 1/2
      let i = 0

      if (m[4] > m[0]) i = 1
      if (m[8] > m[i * 3 + i]) i = 2

      const j = (i + 1) % 3
      const k = (i + 2) % 3

      fRoot = Math.sqrt(m[i * 3 + i] - m[j * 3 + j] - m[k * 3 + k] + 1.0)
      this[i] = 0.5 * fRoot

      fRoot = 0.5 / fRoot
      this[3] = (m[j * 3 + k] - m[k * 3 + j]) * fRoot
      this[j] = (m[j * 3 + i] + m[i * 3 + j]) * fRoot
      this[k] = (m[k * 3 + i] + m[i * 3 + k]) * fRoot
    }
    return this
  }
  // #endregion
}
