// Derived facial topology: an actual mouth opening, attached lips and a recessed oral cavity.
import * as T from 'three';
import {decodePNG,encodePNG} from './png.mjs';

export const FACE_NAMES=['ai','e','o','u','mbp','fv','blink','happy','laugh','sad','angry','surprised','confused','thinking','worried','determined'];
export const FACE_LANDMARKS={
  hoodie:{mouth:[-.024,1.258],width:.080,smile:.014,eyeY:1.414,eyeX:.091},
  professor:{mouth:[.0034,1.237],width:.048,smile:0,eyeY:1.407,eyeX:.09},
  punk:{mouth:[0,1.454],width:.034,smile:.004,eyeY:1.541,eyeX:.049}
};
const shapes={ai:[1.02,.014],e:[1.10,.007],o:[.68,.017],u:[.57,.011],mbp:[.98,0],fv:[1.02,.004],laugh:[1.12,.026],surprised:[.72,.023]};
const smooth=x=>{x=Math.max(0,Math.min(1,x));return x*x*(3-2*x);};

export function authorFace(raw,id){
  const jsonLength=raw.readUInt32LE(12),doc=JSON.parse(raw.subarray(20,20+jsonLength));
  const original=Buffer.from(raw.subarray(28+jsonLength)),chunks=[original],arrays=new Map();let offset=original.length;
  const cfg=FACE_LANDMARKS[id],[cx,cy]=cfg.mouth,w=cfg.width,half=w*.24,cutY=cy+cfg.smile*.5,radius=w*2.3;
  const append=bytes=>{const padding=Buffer.alloc((4-bytes.length%4)%4),index=doc.bufferViews.length;doc.bufferViews.push({buffer:0,byteOffset:offset,byteLength:bytes.length});chunks.push(bytes,padding);offset+=bytes.length+padding.length;return index;};
  const accessor=(array,type,componentType=5126)=>{
    const size={SCALAR:1,VEC2:2,VEC3:3,VEC4:4}[type],a={bufferView:append(Buffer.from(array.buffer,array.byteOffset,array.byteLength)),componentType,count:array.length/size,type};
    if(type==='VEC3'){a.min=[Infinity,Infinity,Infinity];a.max=[-Infinity,-Infinity,-Infinity];for(let i=0;i<array.length;i++){a.min[i%3]=Math.min(a.min[i%3],array[i]);a.max[i%3]=Math.max(a.max[i%3],array[i]);}}
    const index=doc.accessors.push(a)-1;arrays.set(index,array);return index;
  };
  function values(index){
    const a=doc.accessors[index],view=doc.bufferViews[a.bufferView],size={5121:1,5123:2,5125:4,5126:4}[a.componentType],count={SCALAR:1,VEC2:2,VEC3:3,VEC4:4}[a.type],out=[];
    const array=arrays.get(index);
    for(let i=0;i<a.count;i++){
      const at=(view.byteOffset||0)+(a.byteOffset||0)+i*(view.byteStride||size*count),row=[];
      for(let k=0;k<count;k++){let n=array?array[i*count+k]:a.componentType===5126?original.readFloatLE(at+k*size):size===4?original.readUInt32LE(at+k*size):size===2?original.readUInt16LE(at+k*size):original.readUInt8(at+k*size);if(a.normalized)n/=size===2?65535:255;row.push(n);}out.push(row);
    }
    return out;
  }
  const primary=doc.meshes[0].primitives[0],data=Object.fromEntries(Object.entries(primary.attributes).map(([key,index])=>[key,values(index)])),points=data.POSITION,indices=values(primary.indices).flat();
  const geometry=new T.BufferGeometry();geometry.setAttribute('position',new T.Float32BufferAttribute(points.flat(),3));geometry.setIndex(indices);
  const surface=new T.Mesh(geometry,new T.MeshBasicMaterial({side:T.DoubleSide}));surface.updateMatrixWorld(true);const ray=new T.Raycaster(),surfaceHits=new Map();
  function front(x,y){const key=x.toFixed(8)+','+y.toFixed(8);if(surfaceHits.has(key))return surfaceHits.get(key);ray.set(new T.Vector3(x,y,1),new T.Vector3(0,0,-1));const hit=ray.intersectObject(surface)[0];if(!hit)throw new Error('Facial landmark misses '+id);surfaceHits.set(key,hit);return hit;}
  const lipZ=front(cx,cy).point.z;
  function shape(name){const s=shapes[name]||[1,0];return [s[0],s[1]*w/.052];}
  function deform(p,name){
    const [x,y,z]=p,dx=x-cx,dy=y-cy,q=p.slice();if(z<lipZ-.08)return q;
    const mouth=Math.max(0,1-(dx/radius)**2-(dy/(radius*.85))**2),[width,open]=shape(name);
    const crease=cfg.smile*Math.min(1,(dx/(w*.88))**2),lower=smooth((crease-dy)/(w*.06)),pin=Math.max(0,1-(dx/(w*.95))**2),angle=open/(w*2.5)*lower*mouth*pin;
    // Rotate the lower face about a jaw hinge; the upper lip and mouth corners stay anchored.
    const jy=dy-w*.7,jz=z-(lipZ-w*2.5);
    q[1]+=(jy*Math.cos(angle)-jz*Math.sin(angle)-jy);q[2]+=(jy*Math.sin(angle)+jz*Math.cos(angle)-jz);
    q[0]+=dx*(width-1)*Math.max(0,1-(dy/(w*.8))**2)*mouth*.25;
    if(['happy','laugh','sad','worried'].includes(name))q[1]+=(name==='sad'||name==='worried'?-.006:.005)*w/.052*Math.min(1,Math.abs(dx)/w)*mouth;
    const ex=Math.abs(dx)-cfg.eyeX,ey=y-cfg.eyeY,eye=Math.max(0,1-(ex/(cfg.eyeX*.64))**2-(ey/(cfg.eyeX*.45))**2);
    if(name==='blink')q[1]-=ey*.93*eye;
    const brow=Math.max(0,1-(ex/(cfg.eyeX*.7))**2-((ey-cfg.eyeX*.5)/(cfg.eyeX*.35))**2);
    if(name==='angry'||name==='determined')q[1]-=.009*brow*(1-ex/cfg.eyeX);
    if(name==='surprised'||name==='confused'||name==='thinking')q[1]+=.01*brow*(name==='confused'&&dx<0?-.5:1);
    if(name==='worried'||name==='sad')q[1]+=.008*brow*(1-ex/cfg.eyeX);
    return q;
  }
  function sparse(source,changed){
    const ids=[],deltas=[],min=[0,0,0],max=[0,0,0];
    source.forEach((p,i)=>{const d=changed[i].map((n,k)=>n-p[k]);if(d.every(n=>Math.abs(n)<1e-8))return;ids.push(i);deltas.push(...d);d.forEach((n,k)=>{min[k]=Math.min(min[k],n);max[k]=Math.max(max[k],n);});});
    const a={componentType:5126,count:source.length,type:'VEC3',min,max};
    if(ids.length)a.sparse={count:ids.length,indices:{bufferView:append(Buffer.from((source.length<=65535?new Uint16Array(ids):new Uint32Array(ids)).buffer)),componentType:source.length<=65535?5123:5125},values:{bufferView:append(Buffer.from(new Float32Array(deltas).buffer))}};
    return doc.accessors.push(a)-1;
  }
  function skin(vertices,bary){
    const weights=new Map();vertices.forEach((vertex,k)=>data.WEIGHTS_0[vertex].forEach((n,i)=>weights.set(data.JOINTS_0[vertex][i],(weights.get(data.JOINTS_0[vertex][i])||0)+n*bary[k])));
    const row=[...weights].filter(([,n])=>n>1e-8).sort((a,b)=>b[1]-a[1]).slice(0,4),sum=row.reduce((a,[,n])=>a+n,0);if(!(sum>0))throw new Error('Missing facial skin weights');while(row.length<4)row.push([0,0]);return {joints:row.map(([j])=>j),weights:row.map(([,n])=>n/sum)};
  }
  function sample(hit){const f=hit.face,v=[f.a,f.b,f.c],b=T.Triangle.getBarycoord(hit.point,...v.map(i=>new T.Vector3(...points[i])),new T.Vector3()).toArray();return {...skin(v,b),uv:[0,1].map(k=>v.reduce((n,index,i)=>n+data.TEXCOORD_0[index][k]*b[i],0))};}
  function attributeRows(rows){return {POSITION:accessor(new Float32Array(rows.map(v=>v.p).flat()),'VEC3'),NORMAL:accessor(new Float32Array(rows.map(v=>v.n).flat()),'VEC3'),TEXCOORD_0:accessor(new Float32Array(rows.map(v=>v.uv).flat()),'VEC2'),JOINTS_0:accessor(new Uint16Array(rows.map(v=>v.joints).flat()),'VEC4',5123),WEIGHTS_0:accessor(new Float32Array(rows.map(v=>v.weights).flat()),'VEC4')};}
  // Subtract a convex slit from front-facing triangles, retaining interpolated UVs and skin weights.
  const contour=Array.from({length:32},(_,i)=>{const a=i/32*Math.PI*2;return [cx+Math.cos(a)*w,cutY+Math.sin(a)*half];}),patch=[],boundary=new Map();
  const idxAccessor=doc.accessors[primary.indices],idxView=doc.bufferViews[idxAccessor.bufferView],idxStart=(idxView.byteOffset||0)+(idxAccessor.byteOffset||0),idxSize=idxAccessor.componentType===5125?4:2;
  function split(poly,a,b){
    const inside=[],outside=[],distance=v=>(b[0]-a[0])*(v.p[1]-a[1])-(b[1]-a[1])*(v.p[0]-a[0]);
    for(let i=0;i<poly.length;i++){const p=poly[i],q=poly[(i+1)%poly.length],dp=distance(p),dq=distance(q);(dp>=-1e-12?inside:outside).push(p);if((dp>=-1e-12)!==(dq>=-1e-12)){const u=dp/(dp-dq),v={p:p.p.map((n,k)=>n+(q.p[k]-n)*u),b:p.b.map((n,k)=>n+(q.b[k]-n)*u)};inside.push(v);outside.push(v);}}
    return {inside,outside};
  }
  let cut=0;
  for(let at=0;at<indices.length;at+=3){
    const verts=indices.slice(at,at+3),tri=verts.map((vertex,k)=>({p:points[vertex],b:[0,1,2].map(j=>j===k?1:0)}));
    if(tri.every(v=>v.p[0]<cx-w)||tri.every(v=>v.p[0]>cx+w)||tri.every(v=>v.p[1]<cutY-half)||tri.every(v=>v.p[1]>cutY+half)||tri.some(v=>v.p[2]<lipZ-.045))continue;
    let remaining=tri;const pieces=[];
    for(let edge=0;edge<contour.length&&remaining.length;edge++){const parts=split(remaining,contour[edge],contour[(edge+1)%contour.length]);if(parts.outside.length>=3)pieces.push(parts.outside);remaining=parts.inside;}
    if(remaining.length<3)continue;
    cut++;
    for(let k=0;k<3;k++){if(idxSize===4)original.writeUInt32LE(0,idxStart+(at+k)*4);else original.writeUInt16LE(0,idxStart+(at+k)*2);}
    const row=v=>{const normal=[0,1,2].map(k=>verts.reduce((n,index,i)=>n+data.NORMAL[index][k]*v.b[i],0));return {p:v.p,n:new T.Vector3(...normal).normalize().toArray(),uv:[0,1].map(k=>verts.reduce((n,index,i)=>n+data.TEXCOORD_0[index][k]*v.b[i],0)),...skin(verts,v.b)};};
    for(const poly of pieces)for(let j=1;j<poly.length-1;j++)patch.push(...[poly[0],poly[j],poly[j+1]].map(row));
    for(const v of remaining){
      const border=contour.some((a,i)=>{const b=contour[(i+1)%contour.length];return Math.abs((b[0]-a[0])*(v.p[1]-a[1])-(b[1]-a[1])*(v.p[0]-a[0]))<1e-10;});
      if(border){const r=row(v),key=v.p.map(n=>n.toFixed(8)).join(',');boundary.set(key,r);}
    }
  }
  if(!cut||boundary.size<24)throw new Error('Mouth topology was not cut: '+id);
  primary.extras={...primary.extras,storymakerMouthCut:true};
  doc.meshes[0].primitives.push({attributes:attributeRows(patch),material:primary.material,extras:{storymakerPart:'face-patch'}});
  for(const mesh of doc.meshes){
    mesh.extras={...mesh.extras,targetNames:FACE_NAMES};mesh.weights=FACE_NAMES.map(()=>0);
    for(const p of mesh.primitives){
      const source=values(p.attributes.POSITION),faces=p.indices===undefined?source.map((_,i)=>i):values(p.indices).flat(),base=values(p.attributes.NORMAL),reference=normals(source,faces);
      p.targets=FACE_NAMES.map(name=>{
        const changed=source.map(v=>deform(v,name)),target={POSITION:sparse(source,changed)};
        if(shapes[name]){
          const normal=normals(changed,faces).map((n,i)=>{
            const delta=n.map((v,k)=>v-reference[i][k]);if(delta.every(v=>Math.abs(v)<1e-7))return base[i];
            return new T.Vector3(...base[i].map((v,k)=>v+delta[k])).normalize().toArray();
          });
          target.NORMAL=sparse(base,normal);
        }
        return target;
      });
    }
  }
  const edge=[...boundary.values()].sort((a,b)=>Math.atan2((a.p[1]-cutY)/half,(a.p[0]-cx)/w)-Math.atan2((b.p[1]-cutY)/half,(b.p[0]-cx)/w));
  const angles=edge.map(v=>Math.atan2((v.p[1]-cutY)/half,(v.p[0]-cx)/w));
  function inner(a,name){const [width,open]=shape(name),c=Math.cos(a),s=Math.sin(a),rounded=name==='u'||name==='o',restX=cx+c*w*.88,x=cx+(restX-cx)*width,closedY=cy+cfg.smile*c*c,y=cy+cfg.smile*c*c*(rounded?.55:1)+s*(w*.004+open*(rounded?.22:.08))-open*(rounded?.8:.92)*Math.max(0,-s)**.75;
    const corners=['happy','laugh'].includes(name)?.004:['sad','worried'].includes(name)?-.004:0;
    return [x,y+corners*Math.abs(c)*w/.052,front(restX,closedY).point.z+(rounded?w*.03:0)-open*.12*Math.max(0,-s)];}
  function normals(p,faces){const g=new T.BufferGeometry();g.setAttribute('position',new T.Float32BufferAttribute(p.flat(),3));g.setIndex(faces);g.computeVertexNormals();const n=Array.from(g.attributes.normal.array),rows=p.map((_,i)=>n.slice(i*3,i*3+3));g.dispose();return rows;}
  const originalNode=doc.nodes.findIndex(n=>n.mesh===0),parent=doc.nodes.find(n=>n.children?.includes(originalNode));
  function addPart(name,source,faces,material,target,skins,uv){
    const normal=normals(source,faces),attributes={POSITION:accessor(new Float32Array(source.flat()),'VEC3'),NORMAL:accessor(new Float32Array(normal.flat()),'VEC3'),JOINTS_0:accessor(new Uint16Array(skins.map(v=>v.joints).flat()),'VEC4',5123),WEIGHTS_0:accessor(new Float32Array(skins.map(v=>v.weights).flat()),'VEC4')};
    if(uv)attributes.TEXCOORD_0=accessor(new Float32Array(uv.flat()),'VEC2');
    const targets=FACE_NAMES.map(shapeName=>{const changed=source.map((p,i)=>target(p,i,shapeName));return {POSITION:sparse(source,changed),NORMAL:sparse(normal,normals(changed,faces))};});
    const mesh=doc.meshes.push({name,primitives:[{attributes,indices:accessor(new Uint16Array(faces),'SCALAR',5123),material,targets}],weights:FACE_NAMES.map(()=>0),extras:{targetNames:FACE_NAMES,storymakerPart:name}})-1;
    const node=doc.nodes.push({...doc.nodes[originalNode],name,mesh})-1;delete doc.nodes[node].children;if(parent)parent.children.push(node);else doc.scenes[0].nodes.push(node);
  }
  const count=edge.length,faces=[];for(let ring=0;ring<2;ring++)for(let i=0;i<count;i++){const a=ring*count+i,b=ring*count+(i+1)%count,c=a+count,d=b+count;faces.push(a,b,c,b,d,c);}
  function lipTexture(){
    // Reproject the existing face texture so new lip triangles never interpolate across UV islands.
    const material=structuredClone(doc.materials[primary.material]),texture=doc.textures[material.pbrMetallicRoughness.baseColorTexture.index];
    const view=doc.bufferViews[doc.images[texture.source].bufferView],source=decodePNG(original.subarray(view.byteOffset,view.byteOffset+view.byteLength));
    const width=256,height=128,pixels=Buffer.alloc(width*height*4),depth=new Float32Array(width*height).fill(-Infinity);
    const project=p=>[(p[0]-cx+w)/(w*2)*width,(cutY+half-p[1])/(half*2)*height];
    for(let at=0;at<indices.length;at+=3){
      const ids=indices.slice(at,at+3),p=ids.map(i=>points[i]);if(p.every(v=>v[2]<lipZ-.12))continue;
      const q=p.map(project),left=Math.max(0,Math.floor(Math.min(...q.map(v=>v[0])))),right=Math.min(width-1,Math.ceil(Math.max(...q.map(v=>v[0]))));
      const top=Math.max(0,Math.floor(Math.min(...q.map(v=>v[1])))),bottom=Math.min(height-1,Math.ceil(Math.max(...q.map(v=>v[1]))));
      if(left>right||top>bottom)continue;
      const [a,b,c]=q,den=(b[1]-c[1])*(a[0]-c[0])+(c[0]-b[0])*(a[1]-c[1]);if(Math.abs(den)<1e-10)continue;
      for(let y=top;y<=bottom;y++)for(let x=left;x<=right;x++){
        const u=((b[1]-c[1])*(x+.5-c[0])+(c[0]-b[0])*(y+.5-c[1]))/den;
        const v=((c[1]-a[1])*(x+.5-c[0])+(a[0]-c[0])*(y+.5-c[1]))/den,weights=[u,v,1-u-v];if(weights.some(n=>n<0))continue;
        const pos=y*width+x,z=weights.reduce((n,f,i)=>n+f*p[i][2],0);if(z<depth[pos])continue;depth[pos]=z;
        const uv=[0,1].map(k=>ids.reduce((n,index,i)=>n+data.TEXCOORD_0[index][k]*weights[i],0));
        const tx=Math.max(0,Math.min(source.width-1,uv[0]*source.width-.5)),ty=Math.max(0,Math.min(source.height-1,uv[1]*source.height-.5)),ix=Math.floor(tx),iy=Math.floor(ty),fx=tx-ix,fy=ty-iy;
        for(let k=0;k<4;k++){let value=0;for(let dy=0;dy<2;dy++)for(let dx=0;dx<2;dx++)value+=source.pixels[(Math.min(source.height-1,iy+dy)*source.width+Math.min(source.width-1,ix+dx))*4+k]*(dx?fx:1-fx)*(dy?fy:1-fy);pixels[pos*4+k]=Math.round(value);}
      }
    }
    const empty=Array.from(depth,(_,i)=>i).filter(i=>!Number.isFinite(depth[i]));
    if(empty.length>width*height*.05)throw new Error('Incomplete lip texture: '+id);
    // Bleed adjacent texels into tiny source-mesh gaps, including the nostril edge above the lip.
    while(empty.length){let filled=0;for(let i=empty.length-1;i>=0;i--){const at=empty[i],near=[at-width,at+width,at%width?at-1:-1,at%width<width-1?at+1:-1].find(n=>n>=0&&n<depth.length&&Number.isFinite(depth[n]));if(near===undefined)continue;pixels.copy(pixels,at*4,near*4,near*4+4);depth[at]=depth[near];empty.splice(i,1);filled++;}if(!filled)throw new Error('Unreachable lip texels: '+id);}
    const sourceIndex=doc.images.push({bufferView:append(encodePNG({width,height,pixels})),mimeType:'image/png',name:'Attached lips'})-1;
    const textureIndex=doc.textures.push({...texture,source:sourceIndex})-1;
    material.name='Attached lips';delete material.normalTexture;material.pbrMetallicRoughness.baseColorTexture={index:textureIndex};
    return doc.materials.push(material)-1;
  }
  const lipPoints=(name)=>[0,.55,1].flatMap(u=>edge.map((v,i)=>{
    const rest=inner(angles[i],'rest'),inside=inner(angles[i],name),outer=deform(v.p,name),p=v.p.map((n,k)=>n+(rest[k]-n)*u);
    if(u>0)p[2]=front(p[0],p[1]).point.z;
    return p.map((n,k)=>n+(outer[k]-v.p[k])*(1-u)+(inside[k]-rest[k])*u);
  }));
  const lipFrames=Object.fromEntries(['rest',...FACE_NAMES].map(name=>[name,lipPoints(name)]));
  addPart('Storymaker_Mouth_Lips',lipFrames.rest,faces,lipTexture(),(_,i,name)=>lipFrames[name][i],[...edge,...edge,...edge],lipFrames.rest.map(p=>[(p[0]-cx+w)/(w*2),(cutY+half-p[1])/(half*2)]));
  const flesh=doc.materials.push({name:'Mouth interior',doubleSided:true,pbrMetallicRoughness:{baseColorFactor:[.025,.004,.007,1],metallicFactor:0,roughnessFactor:1}})-1;
  const cavityPoints=name=>{const [,open]=shape(name);return [...angles.map(a=>inner(a,name)),...angles.map(a=>{const p=inner(a,name);return [cx+(p[0]-cx)*.65,cy-open*.45+(p[1]-cy+open*.45)*.6,p[2]-w*.65];}),[cx,cy-open*.45,lipZ-w*.8]];};
  const cavityFaces=[];for(let i=0;i<count;i++){const j=(i+1)%count;cavityFaces.push(i,j,i+count,j,j+count,i+count,i+count,j+count,count*2);}
  const cavityFrames=Object.fromEntries(['rest',...FACE_NAMES].map(name=>[name,cavityPoints(name)]));
  const middle=sample(front(cx,cy));addPart('Storymaker_Mouth_Cavity',cavityFrames.rest,cavityFaces,flesh,(_,i,name)=>cavityFrames[name][i],[...edge,...edge,middle]);
  const toothMaterial=doc.materials.push({name:'Teeth',doubleSided:true,pbrMetallicRoughness:{baseColorFactor:[.82,.76,.65,1],metallicFactor:0,roughnessFactor:.6}})-1;
  const teeth=[],toothSkin=[],toothFaces=[];
  for(let row=0;row<3;row++)for(let i=0;i<=16;i++){const x=cx+(i/8-1)*w*.70,y=cy+cfg.smile*((x-cx)/(w*.88))**2,hit=front(x,y);teeth.push([x,y+w*(row===0?.12:row===1?-.04:-.06),hit.point.z-w*(row===2?.13:.08)]);toothSkin.push(sample(hit));}
  for(let row=0;row<2;row++)for(let i=0;i<16;i++){const a=row*17+i;toothFaces.push(a,a+1,a+17,a+1,a+18,a+17);}
  addPart('Storymaker_Mouth_Teeth',teeth,toothFaces,toothMaterial,p=>p,toothSkin);
  const tongueMaterial=doc.materials.push({name:'Tongue',doubleSided:true,pbrMetallicRoughness:{baseColorFactor:[.35,.065,.09,1],metallicFactor:0,roughnessFactor:.85}})-1;
  const tongue=[[cx,cy-w*.09,lipZ-w*.30]],tongueFaces=[];
  for(let i=0;i<24;i++){const a=i/24*Math.PI*2;tongue.push([cx+Math.cos(a)*w*.5,cy-w*.13,lipZ-w*.34+Math.sin(a)*w*.16]);tongueFaces.push(0,i+1,(i+1)%24+1);}
  addPart('Storymaker_Mouth_Tongue',tongue,tongueFaces,tongueMaterial,(p,_,name)=>{const [,open]=shape(name);return [p[0],p[1]-open*.72,p[2]-open*.10];},tongue.map(()=>middle));
  doc.buffers[0].byteLength=offset;
  const text=Buffer.from(JSON.stringify(doc)),json=Buffer.alloc(Math.ceil(text.length/4)*4,32);text.copy(json);const bin=Buffer.concat(chunks),header=Buffer.alloc(20),tail=Buffer.alloc(8);
  header.write('glTF');header.writeUInt32LE(2,4);header.writeUInt32LE(28+json.length+bin.length,8);header.writeUInt32LE(json.length,12);header.writeUInt32LE(0x4e4f534a,16);tail.writeUInt32LE(bin.length);tail.writeUInt32LE(0x004e4942,4);
  geometry.dispose();surface.material.dispose();return Buffer.concat([header,json,tail,bin]);
}
