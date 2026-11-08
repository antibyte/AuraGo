// Apply rotation to the first join in the chain
// by using one of the inverse direction as the
// rotational axis
export class AxisAdditive {
  constructor (chName, axis = 'y', rad = 0) {
    this.chainName = chName
    this.onAxis = axis
    this.angle = rad
  }

  apply (rt) {
    if (this.angle === 0) return

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    const ch = rt.tarRig.chains[this.chainName]
    const itm = ch[0]
    const ptran = rt.pose.getWorld(itm.pidx)
    const ctran = new Transform().fromMul(ptran, rt.pose.joints[itm.idx].local) // Less computive
    // const ctran = rt.pose.getWorld( itm.idx );

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    const axis = new Vec3()
    switch (this.onAxis) {
      case 'y': axis.fromQuat(ctran.rot, itm.twist); break
      default:
        console.log('ERROR : AxisAdditive - Axis not implemented', this.onAxis)
        return
    }

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    const q = new Quat()
      .fromAxisAngle(axis, this.angle) // Compute WS Rotation
      .mul(ctran.rot) // Apply to joint
      .pmulInvert(ptran.rot) // To LocalSpace

    rt.pose.setRot(itm.idx, q)
  }
}
