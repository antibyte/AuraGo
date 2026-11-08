// Compute a rotation axis between the first & last
// points of the chain, the apply the twist rotation

import Quat from './Quat'
import { type Retargeter } from './Retargeter'
import { type RigItem } from './RigItem'
import Transform from './Transform'
import Vec3 from './Vec3'

// to the first joint of the chain
export class ChainTwistAdditive {
  public chainName: string
  public angle: number

  constructor (chain_name: string, rad: number = 0) {
    this.chainName = chain_name
    this.angle = rad
  }

  apply (rt: Retargeter): void {
    if (this.angle === 0 || rt.tarRig === null) return

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    const ch: RigItem[] = rt.tarRig.chains[this.chainName]
    const itm0 = ch[0]
    const itm1 = ch[ch.length - 1]

    if (rt.pose === null) {
      console.warn('ChainTwistAdditive: Missing working pose.')
      return
    }

    const ptran: Transform = rt.pose.getWorld(itm0.pidx)
    const ctran: Transform = new Transform().fromMul(ptran, rt.pose.joints[itm0.idx].local) // Chain Start Transform
    const etran: Transform = rt.pose.getWorld(itm1.idx) // Chain End Transform

    // Debug.pnt.add( ctran.pos, 0xff0000, 2, 0 );
    // Debug.pnt.add( etran.pos, 0xffff00, 2, 0 );

    // ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    const axis = new Vec3().fromSub(etran.pos, ctran.pos).norm()
    const q = new Quat()
      .fromAxisAngle(axis, this.angle) // Compute WS Rotation
      .mul(ctran.rot) // Apply to joint
      .pmulInvert(ptran.rot) // To LocalSpace

    rt.pose.setRot(itm0.idx, q)
  }
}
