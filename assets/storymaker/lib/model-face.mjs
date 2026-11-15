// Local facial shape keys on derived pet models. The source GLB and rigid skull are preserved.
import * as T from 'three';

export const FACE_NAMES=['ai','e','o','u','mbp','fv','blink','happy','laugh','sad','angry','surprised','confused','thinking','worried','determined'];
export const FACE_LANDMARKS={
  hoodie:{mouth:[-.024,1.278],width:.052,eyeY:1.414,eyeX:.091,lip:[.42,.17,.13,1]},
  professor:{mouth:[.0034,1.259],width:.048,eyeY:1.407,eyeX:.09,lip:[.38,.17,.13,1]},
  punk:{mouth:[0,1.472],width:.027,eyeY:1.541,eyeX:.049,lip:[.38,.075,.12,1]}
};
const shapes={ai:[1.05,.025],e:[1.2,.015],o:[.68,.027],u:[.5,.018],mbp:[.96,0],fv:[1.05,.006],laugh:[1.16,.03],surprised:[.7,.025]};

export function authorFace(raw,id){
  const jsonLength=raw.readUInt32LE(12), doc=JSON.parse(raw.subarray(20,20+jsonLength));
  const original=raw.subarray(28+jsonLength), chunks=[original]; let offset=original.length;
  const cfg=FACE_LANDMARKS[id], [cx,cy]=cfg.mouth, radius=cfg.width*2.3;
  const append=bytes=>{const padding=Buffer.alloc((4-bytes.length%4)%4), index=doc.bufferViews.length;doc.bufferViews.push({buffer:0,byteOffset:offset,byteLength:bytes.length});chunks.push(bytes,padding);offset+=bytes.length+padding.length;return index;};
  const accessor=(array,type,componentType=5126)=>{
    const size={SCALAR:1,VEC2:2,VEC3:3,VEC4:4}[type], a={bufferView:append(Buffer.from(array.buffer,array.byteOffset,array.byteLength)),componentType,count:array.length/size,type};
    if(type==='VEC3'){a.min=[Infinity,Infinity,Infinity];a.max=[-Infinity,-Infinity,-Infinity];for(let i=0;i<array.length;i++){a.min[i%3]=Math.min(a.min[i%3],array[i]);a.max[i%3]=Math.max(a.max[i%3],array[i]);}}
    return doc.accessors.push(a)-1;
  };
  function positions(index){
    const a=doc.accessors[index], b=doc.bufferViews[a.bufferView], out=[];
    for(let i=0;i<a.count;i++){const at=(b.byteOffset||0)+(a.byteOffset||0)+i*(b.byteStride||12);out.push([original.readFloatLE(at),original.readFloatLE(at+4),original.readFloatLE(at+8)]);}
    return out;
  }
  const primary=doc.meshes[0].primitives[0], points=positions(primary.attributes.POSITION);
  const geometry=new T.BufferGeometry();geometry.setAttribute('position',new T.Float32BufferAttribute(points.flat(),3));
  const ia=doc.accessors[primary.indices],iv=doc.bufferViews[ia.bufferView],ib=(iv.byteOffset||0)+(ia.byteOffset||0), indices=[];
  for(let i=0;i<ia.count;i++)indices.push(ia.componentType===5125?original.readUInt32LE(ib+i*4):original.readUInt16LE(ib+i*2));
  geometry.setIndex(indices);const surface=new T.Mesh(geometry,new T.MeshBasicMaterial({side:T.DoubleSide}));surface.updateMatrixWorld(true);
  const ray=new T.Raycaster();
  let faceShape='rest',lastHit;
  function front(x,y,name='rest'){
    if(faceShape!==name){
      const pos=geometry.attributes.position;
      points.forEach((p,i)=>{const q=name==='rest'?p:deform(p,name);pos.setXYZ(i,...q);});
      geometry.computeBoundingSphere();faceShape=name;
    }
    ray.set(new T.Vector3(x,y,1),new T.Vector3(0,0,-1));const hit=ray.intersectObject(surface)[0];
    if(!hit)throw new Error('Facial landmark misses '+id);lastHit=hit;return hit.point.z;
  }
  const lipZ=front(cx,cy);
  function deform(p,name){
    const [x,y,z]=p, dx=x-cx,dy=y-cy, face=z>lipZ-.08;
    let q=p.slice();
    if(!face)return q;
    const mouth=Math.max(0,1-Math.pow(dx/radius,2)-Math.pow(dy/(radius*.85),2));
    const shape=shapes[name]||[1,0], lower=1/(1+Math.exp(dy*180));
    q[0]+=dx*(shape[0]-1)*mouth;
    q[1]-=shape[1]*lower*mouth;
    q[2]-=shape[1]*.15*lower*mouth;
    if(['happy','laugh','sad','worried'].includes(name))q[1]+=(name==='sad'||name==='worried'?-.01:.01)*Math.min(1,Math.abs(dx)/cfg.width)*mouth;
    if(name==='mbp')q[1]-=dy*.12*mouth;
    const ex=Math.abs(dx)-cfg.eyeX, ey=y-cfg.eyeY, eye=Math.max(0,1-Math.pow(ex/(cfg.eyeX*.64),2)-Math.pow(ey/(cfg.eyeX*.45),2));
    if(name==='blink')q[1]-=ey*.93*eye;
    const brow=Math.max(0,1-Math.pow(ex/(cfg.eyeX*.7),2)-Math.pow((ey-cfg.eyeX*.5)/(cfg.eyeX*.35),2));
    if(name==='angry'||name==='determined')q[1]-=.009*brow*(1-ex/cfg.eyeX);
    if(name==='surprised'||name==='confused'||name==='thinking')q[1]+=.01*brow*(name==='confused'&&dx<0?-.5:1);
    if(name==='worried'||name==='sad')q[1]+=.008*brow*(1-ex/cfg.eyeX);
    return q;
  }
  function sparse(points,changed){
    const ids=[],values=[],min=[0,0,0],max=[0,0,0];
    for(let i=0;i<points.length;i++){const d=changed[i].map((n,j)=>n-points[i][j]);if(d.every(n=>Math.abs(n)<1e-8))continue;ids.push(i);values.push(...d);for(let j=0;j<3;j++){min[j]=Math.min(min[j],d[j]);max[j]=Math.max(max[j],d[j]);}}
    const a={componentType:5126,count:points.length,type:'VEC3',min,max};
    if(ids.length)a.sparse={count:ids.length,indices:{bufferView:append(Buffer.from(new Uint32Array(ids).buffer)),componentType:5125},values:{bufferView:append(Buffer.from(new Float32Array(values).buffer))}};
    return doc.accessors.push(a)-1;
  }
  for(const mesh of doc.meshes){
    mesh.extras={...mesh.extras,targetNames:FACE_NAMES};mesh.weights=FACE_NAMES.map(()=>0);
    for(const p of mesh.primitives){const source=positions(p.attributes.POSITION);p.targets=FACE_NAMES.map(name=>({POSITION:sparse(source,source.map(v=>deform(v,name)))}));}
  }
  const joint=doc.skins[0].joints.findIndex(i=>doc.nodes[i].name.replaceAll('.','')==='DEF-spine006');
  if(joint<0)throw new Error('Missing head joint: '+id);
  function four(index,vertex){
    const a=doc.accessors[index],view=doc.bufferViews[a.bufferView],size={5121:1,5123:2,5126:4}[a.componentType];
    if(!size||a.type!=='VEC4')throw new Error('Unsupported face skin: '+id);
    const start=(view.byteOffset||0)+(a.byteOffset||0)+vertex*(view.byteStride||size*4);
    return Array.from({length:4},(_,i)=>{const at=start+i*size,v=size===4?original.readFloatLE(at):size===2?original.readUInt16LE(at):original.readUInt8(at);return a.normalized?v/(size===2?65535:255):v;});
  }
  function skinAtHit(hit){
    const f=hit.face,b=T.Triangle.getBarycoord(hit.point,new T.Vector3(...points[f.a]),new T.Vector3(...points[f.b]),new T.Vector3(...points[f.c]),new T.Vector3()),weights=new Map();
    [f.a,f.b,f.c].forEach((vertex,k)=>{const joints=four(primary.attributes.JOINTS_0,vertex),values=four(primary.attributes.WEIGHTS_0,vertex);values.forEach((w,i)=>weights.set(joints[i],(weights.get(joints[i])||0)+w*b.getComponent(k)));});
    const row=[...weights].filter(([,w])=>w>0).sort((a,b)=>b[1]-a[1]).slice(0,4),sum=row.reduce((n,[,w])=>n+w,0);
    if(!(sum>0))throw new Error('Missing face skin weights: '+id);
    while(row.length<4)row.push([0,0]);
    return {joints:row.map(([j])=>j),weights:row.map(([,w])=>w/sum)};
  }
  const mouthPrimitives=[];
  for(const kind of ['cavity','lip']){
    const source=[],uv=[],faces=[],skinJoints=[],skinWeights=[],count=32;
    const rings=kind==='cavity'?[0,1]:[1,1.15];
    for(const ring of rings)for(let i=0;i<=count;i++){
      const a=i/count*Math.PI*2,x=cx+Math.cos(a)*cfg.width*ring,y=cy+Math.sin(a)*.0025*ring;
      source.push([x,y,front(x,y)+.004+(kind==='lip'?.001:0)]);uv.push([Math.cos(a),Math.sin(a),ring]);
      const skin=skinAtHit(lastHit);skinJoints.push(...skin.joints);skinWeights.push(...skin.weights);
    }
    for(let i=0;i<count;i++){const a=i,b=i+1,c=i+count+1,d=c+1;faces.push(a,c,b,b,c,d);}
    const targets=FACE_NAMES.map(name=>{
      const shape=shapes[name]||[1,0], corners=['happy','laugh'].includes(name)?.005:['sad','worried'].includes(name)?-.005:0;
      return {POSITION:sparse(source,source.map((p,i)=>{
        const [x,y,ring]=uv[i],width=cfg.width*shape[0],height=.0025+shape[1]*.5;
        const nx=cx+x*width*ring,ny=cy-shape[1]*.45+y*height*ring+corners*Math.abs(x);
        return [nx,ny,front(nx,ny,name)+.004+(kind==='lip'?.001:0)+(name==='u'?.002:0)];
      }))};
    });
    const material=doc.materials.push({name:'Storymaker_'+kind,doubleSided:true,pbrMetallicRoughness:{baseColorFactor:kind==='cavity'?[.028,.007,.011,1]:cfg.lip,metallicFactor:0,roughnessFactor:.85}})-1;
    mouthPrimitives.push({attributes:{POSITION:accessor(new Float32Array(source.flat()),'VEC3'),JOINTS_0:accessor(new Uint16Array(skinJoints),'VEC4',5123),WEIGHTS_0:accessor(new Float32Array(skinWeights),'VEC4')},indices:accessor(new Uint16Array(faces),'SCALAR',5123),material,targets});
  }
  const meshIndex=doc.meshes.push({name:'Storymaker_Lips',primitives:mouthPrimitives,weights:FACE_NAMES.map(()=>0),extras:{targetNames:FACE_NAMES}})-1;
  const originalNode=doc.nodes.findIndex(n=>n.mesh===0), node=doc.nodes.push({...doc.nodes[originalNode],name:'Storymaker_Lips',mesh:meshIndex})-1;
  delete doc.nodes[node].children;
  const parent=doc.nodes.find(n=>n.children?.includes(originalNode));
  if(parent)parent.children.push(node);else doc.scenes[0].nodes.push(node);
  doc.buffers[0].byteLength=offset;
  const text=Buffer.from(JSON.stringify(doc)),json=Buffer.alloc(Math.ceil(text.length/4)*4,32);text.copy(json);
  const bin=Buffer.concat(chunks),header=Buffer.alloc(20),tail=Buffer.alloc(8);
  header.write('glTF');header.writeUInt32LE(2,4);header.writeUInt32LE(28+json.length+bin.length,8);header.writeUInt32LE(json.length,12);header.writeUInt32LE(0x4e4f534a,16);
  tail.writeUInt32LE(bin.length);tail.writeUInt32LE(0x004e4942,4);
  geometry.dispose();surface.material.dispose();
  return Buffer.concat([header,json,tail,bin]);
}
