import * as THREE from 'three';

// Local handshakes are explicitly decorative. Real district traffic remains in
// the existing verified action pool; no local sequence manufactures telemetry.
export function createSocialSignals(scene) {
  const root=new THREE.Group();root.name='city-conversations';scene.add(root);
  const geometry=new THREE.TorusGeometry(1,.025,5,40,Math.PI*1.45),haloGeometry=new THREE.RingGeometry(.92,1,48);
  const up=new THREE.Vector3(0,0,1),direction=new THREE.Vector3(),pool=[];
  let count=0;
  for(let i=0;i<8;i++) {
    const meshes=[];
    for(let j=0;j<4;j++) {
      const material=new THREE.MeshBasicMaterial({color:0x80ddd0,transparent:true,opacity:0,blending:THREE.AdditiveBlending,depthWrite:false,side:THREE.DoubleSide,toneMapped:false});
      const mesh=new THREE.Mesh(j===3?haloGeometry:geometry,material);mesh.visible=false;root.add(mesh);meshes.push(mesh);
    }
    pool.push({meshes,age:10,from:null,to:null});
  }
  return {
    send(from,to,source='ambient') {
      const packet=pool.find(p=>p.age>=2.4);if(!packet)return false;
      Object.assign(packet,{from,to,age:0,source});count++;
      packet.meshes.forEach(m=>m.material.color.setHex(source==='live'?0xf5c17b:0x80ddd0));return true;
    },
    update(dt,animated=true) {
      for(const p of pool) {
        if(!animated||p.from?.enabled===false||p.to?.enabled===false)p.age=10;
        p.age+=dt;p.meshes.forEach(m=>{m.visible=false;});if(p.age>=2.4)continue;
        const from=p.from,to=p.to,dx=to.x-from.x,dz=to.z-from.z;
        direction.set(dx,(to.y+Math.min(2,to.maxY*.7))-(from.y+Math.min(2,from.maxY*.7)),dz).normalize();
        for(let j=0;j<3;j++) {
          const t=(p.age-j*.18)/1.55;if(t<0||t>1)continue;
          const m=p.meshes[j];m.visible=true;m.position.set(from.x+dx*t,THREE.MathUtils.lerp(from.y+Math.min(2,from.maxY*.7),to.y+Math.min(2,to.maxY*.7),t)+Math.sin(t*Math.PI)*.65,from.z+dz*t);
          m.quaternion.setFromUnitVectors(up,direction);m.rotateZ(j*.65);m.scale.setScalar(.25+Math.sin(t*Math.PI)*.65);
          m.material.opacity=Math.sin(Math.PI*t)*.75;
        }
        const arrival=(p.age-1.55)/.85;
        if(arrival>0){const m=p.meshes[3];m.visible=true;m.position.set(to.x,to.y+Math.min(2,to.maxY*.7),to.z);m.quaternion.setFromUnitVectors(up,direction);m.scale.setScalar(1.25*(1-arrival)+.12);m.material.opacity=Math.sin(arrival*Math.PI)*.85;}
      }
    },
    clear(){pool.forEach(p=>{p.age=10;p.meshes.forEach(m=>{m.visible=false;});});},
    stats:()=>({sent:count,active:pool.filter(p=>p.age<2.4).length,sources:pool.filter(p=>p.age<2.4).map(p=>p.source)}),
    dispose(){root.removeFromParent();geometry.dispose();haloGeometry.dispose();pool.forEach(p=>p.meshes.forEach(m=>m.material.dispose()));},
  };
}
