import * as THREE from 'three';
// Instanced moving parts of kit v3 designs (phase 4): banner cloth, skiff rigs, buoy lanterns, gull wings and whole
// hulls. One InstancedMesh per mesh of the template; a mesh under a `movable` part node turns about that node's origin
// (the pivot kit3.py authors from `pivots`). `only` keeps just the meshes under those part nodes. Materials stay the
// template's (shared kit materials): never dispose them here.
export function partOf(node,names){for(let o=node;o;o=o.parent)if(names.includes(o.name))return o;return null;}
export function createPartInstances(template,count,{only=null,movable=[]}={}){
  template.updateMatrixWorld(true);const rows=[];
  template.traverse(n=>{
    if(!n.isMesh||(only&&!partOf(n,only)))return;
    const part=partOf(n,movable),pivot=(part||template).matrixWorld.clone(),local=pivot.clone().invert().multiply(n.matrixWorld);
    const mesh=new THREE.InstancedMesh(n.geometry,n.material,count);mesh.frustumCulled=false;mesh.castShadow=true;mesh.receiveShadow=true;
    rows.push({mesh,part:part?.name||null,pivot,local});
  });
  const m=new THREE.Matrix4(),zero=new THREE.Matrix4().makeScale(0,0,0);
  return {
    meshes:rows.map(r=>r.mesh),
    // matrix places the whole design; turn(partName) returns a rotation Matrix4 for that part, or null.
    set(i,matrix,turn){for(const r of rows){m.multiplyMatrices(matrix,r.pivot);const t=r.part&&turn?turn(r.part):null;if(t)m.multiply(t);m.multiply(r.local);r.mesh.setMatrixAt(i,m);}},
    hide(i){for(const r of rows)r.mesh.setMatrixAt(i,zero);},
    commit(){for(const r of rows)r.mesh.instanceMatrix.needsUpdate=true;},
    dispose(){for(const r of rows)r.mesh.dispose();},
  };
}
