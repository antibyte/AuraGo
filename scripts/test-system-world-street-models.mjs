// Kit v3 street modules against the analytic profile: every packed GLB, placed exactly like the renderer
// places it, is rasterised from above (highest up-facing surface per 10 cm cell). Wherever streetProfile
// defines a street the surface must exist and match within 1.5 / 2 / 3 cm at LOD 0 / 1 / 2 (coarser arcs by design), at every LOD.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';
import {streets} from '../ui/js/desktop/apps/sysworld-layout.js';
import {streetProfile} from '../ui/js/desktop/apps/sysworld-ground.js';
import {streetPlacements as kitPlacements} from '../ui/js/desktop/apps/sysworld-scene.js';

const dir='ui/3d/system-world/v3/',manifest=JSON.parse(await fs.readFile(dir+'manifest.json','utf8')),H=.1,LIMIT=[.015,.02,.03];
const ids=[...new Set(kitPlacements.map(p=>p.asset))];
await MeshoptDecoder.ready;const loader=new GLTFLoader().setMeshoptDecoder(MeshoptDecoder);
const NX=Math.round((streets.maxX-streets.minX)/H),NZ=Math.round((streets.maxZ-streets.minZ)/H);
for(const level of [0,1,2]){
  const templates=new Map();
  for(const id of ids){
    const entry=manifest.assets.find(a=>a.id===id);assert.ok(entry,'Kit v3 lacks '+id);
    const data=await fs.readFile(dir+entry.lods.find(l=>l.level===level).file);
    const gltf=await loader.parseAsync(data.buffer.slice(data.byteOffset,data.byteOffset+data.byteLength),'');
    gltf.scene.updateMatrixWorld(true);const tris=[],a=new THREE.Vector3(),b=new THREE.Vector3(),c=new THREE.Vector3(),n=new THREE.Vector3();
    let vc=0;gltf.scene.traverse(m=>{if(!m.isMesh)return;if(m.geometry.getAttribute('color'))vc++;
      assert.ok(m.material.vertexColors,id+': baked AO must load as vertex colours');
      const pos=m.geometry.getAttribute('position'),index=m.geometry.getIndex(),count=index?index.count:pos.count;
      for(let i=0;i<count;i+=3){
        a.fromBufferAttribute(pos,index?index.getX(i):i).applyMatrix4(m.matrixWorld);b.fromBufferAttribute(pos,index?index.getX(i+1):i+1).applyMatrix4(m.matrixWorld);c.fromBufferAttribute(pos,index?index.getX(i+2):i+2).applyMatrix4(m.matrixWorld);
        n.subVectors(b,a).cross(c.clone().sub(a)).normalize();if(Math.abs(n.y)>.2)tris.push([a.x,a.y,a.z,b.x,b.y,b.z,c.x,c.y,c.z]);
      }});
    assert.ok(vc>0,id+' carries COLOR_0');templates.set(id,tris);
  }
  const top=new Float32Array(NX*NZ).fill(-Infinity);
  for(const p of kitPlacements){
    const co=Math.cos(p.angle),si=Math.sin(p.angle),w=(x,z)=>[p.x+x*co+z*si,p.z-x*si+z*co];
    for(const t of templates.get(p.asset)){
      const [ax,az]=w(t[0],t[2]),[bx,bz]=w(t[3],t[5]),[cx,cz]=w(t[6],t[8]),ay=p.y+t[1],by=p.y+t[4],cy=p.y+t[7];
      const det=(bz-cz)*(ax-cx)+(cx-bx)*(az-cz);if(Math.abs(det)<1e-12)continue;
      const i0=Math.max(0,Math.ceil((Math.min(ax,bx,cx)-streets.minX)/H-.5)),i1=Math.min(NX-1,Math.floor((Math.max(ax,bx,cx)-streets.minX)/H-.5));
      const k0=Math.max(0,Math.ceil((Math.min(az,bz,cz)-streets.minZ)/H-.5)),k1=Math.min(NZ-1,Math.floor((Math.max(az,bz,cz)-streets.minZ)/H-.5));
      for(let i=i0;i<=i1;i++)for(let k=k0;k<=k1;k++){
        const x=streets.minX+(i+.5)*H,z=streets.minZ+(k+.5)*H;
        const l1=((bz-cz)*(x-cx)+(cx-bx)*(z-cz))/det,l2=((cz-az)*(x-cx)+(ax-cx)*(z-cz))/det,l3=1-l1-l2;
        if(l1<-1e-7||l2<-1e-7||l3<-1e-7)continue;
        const y=l1*ay+l2*by+l3*cy,idx=i*NZ+k;if(y>top[idx])top[idx]=y;
      }
    }
  }
  let gaps=0,bad=0,worst=0,examples=[];
  for(let i=0;i<NX;i++)for(let k=0;k<NZ;k++){
    const x=streets.minX+(i+.5)*H,z=streets.minZ+(k+.5)*H,p=streetProfile(x,z);if(!p)continue;
    const y=top[i*NZ+k];
    if(y===-Infinity){gaps++;if(examples.length<10)examples.push(['gap',x,z]);continue;}
    const d=Math.abs(y-p.height);worst=Math.max(worst,d);if(d>LIMIT[level]){bad++;if(examples.length<10)examples.push([d.toFixed(3),x,z,p.kind]);}
  }
  assert.ok(gaps===0&&bad===0,`LOD${level}: gaps ${gaps}, over tolerance ${bad} (limit ${LIMIT[level]*1000} mm), worst ${worst.toFixed(4)} m, e.g. ${JSON.stringify(examples)}`);
  console.log(`Street models LOD${level}: ${kitPlacements.length} placements match the profile (worst ${(worst*1000).toFixed(1)} mm).`);
}
