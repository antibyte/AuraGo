import * as THREE from 'three';
// Soft blob shadows under residents, the companion and the ground projection of hover robots.
// One instanced quad, one draw call; a slot is claimed once and moved every step.
export function createContactShadows(scene,{capacity=40}={}){
  const geometry=new THREE.PlaneGeometry(1,1);geometry.rotateX(-Math.PI/2);
  const strength=new THREE.InstancedBufferAttribute(new Float32Array(capacity),1);geometry.setAttribute('swShadow',strength);
  const material=new THREE.ShaderMaterial({transparent:true,depthWrite:false,polygonOffset:true,polygonOffsetFactor:-2,polygonOffsetUnits:-2,
    vertexShader:'attribute float swShadow;varying vec2 vUv;varying float vA;void main(){vUv=uv;vA=swShadow;vec4 p=vec4(position,1.0);\n#ifdef USE_INSTANCING\np=instanceMatrix*p;\n#endif\ngl_Position=projectionMatrix*modelViewMatrix*p;}',
    fragmentShader:'varying vec2 vUv;varying float vA;void main(){float r=length(vUv-.5)*2.;gl_FragColor=vec4(0.,0.,0.,vA*(1.-smoothstep(.1,1.,r)));}'});
  const mesh=new THREE.InstancedMesh(geometry,material,capacity),matrix=new THREE.Matrix4();
  mesh.name='contact-shadows';mesh.frustumCulled=false;mesh.renderOrder=1;mesh.castShadow=false;mesh.receiveShadow=false;
  for(let i=0;i<capacity;i++)mesh.setMatrixAt(i,matrix.makeScale(0,0,0));scene.add(mesh);
  let used=0;
  return {
    claim(){
      if(used>=capacity)return null;const i=used++;
      return {
        set(x,y,z,radius,alpha){mesh.setMatrixAt(i,matrix.makeScale(radius*2,1,radius*2).setPosition(x,y+.015,z));strength.setX(i,alpha);mesh.instanceMatrix.needsUpdate=true;strength.needsUpdate=true;},
        hide(){mesh.setMatrixAt(i,matrix.makeScale(0,0,0));strength.setX(i,0);mesh.instanceMatrix.needsUpdate=true;strength.needsUpdate=true;},
      };
    },
    stats:()=>({used,capacity}),
    dispose(){mesh.removeFromParent();mesh.dispose();geometry.dispose();material.dispose();},
  };
}
