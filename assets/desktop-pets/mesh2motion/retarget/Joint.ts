import Transform from './Transform'
import type * as THREE from 'three'

export class Joint {
  public name: string = ''
  public index: number = -1
  public pindex: number = -1
  public isRoot: boolean = false
  public children: number[] = [] // child joint indices
  public local: Transform = new Transform()
  public world: Transform = new Transform()
  public idx: number = 1 // set externally

  fromBone (b: THREE.Bone): this {
    this.name = b.name

    let v = b.position.toArray()
    this.local.pos[0] = v[0]
    this.local.pos[1] = v[1]
    this.local.pos[2] = v[2]

    const r: THREE.QuaternionTuple = b.quaternion.toArray()
    this.local.rot[0] = r[0]
    this.local.rot[1] = r[1]
    this.local.rot[2] = r[2]
    this.local.rot[3] = r[3]

    v = b.scale.toArray()
    this.local.scl[0] = v[0]
    this.local.scl[1] = v[1]
    this.local.scl[2] = v[2]

    return this
  }

  clone (): Joint {
    const j = new Joint()
    j.name = this.name
    j.index = this.index
    j.pindex = this.pindex
    j.isRoot = this.isRoot
    j.children = [...this.children]
    j.local.copy(this.local)
    j.world.copy(this.world)
    return j
  }
}
