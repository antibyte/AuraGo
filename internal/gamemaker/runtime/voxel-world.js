// MIT. Finite deterministic block world. Rendering, collision and saves share this grid.
export const CHUNK = 16;
export function seedHash(text) { let n=2166136261; for(const c of String(text)){n=Math.imul(n^c.charCodeAt(0),16777619);}return n>>>0; }
export function noise(seed,x,z) { let n=(seed^Math.imul(x,374761393)^Math.imul(z,668265263))>>>0;n=Math.imul(n^(n>>>13),1274126177);return ((n^(n>>>16))>>>0)/4294967296; }
const faces = [
  {n:[1,0,0],v:[[1,0,1],[1,0,0],[1,1,0],[1,1,1]]},
  {n:[-1,0,0],v:[[0,0,0],[0,0,1],[0,1,1],[0,1,0]]},
  {n:[0,1,0],v:[[0,1,1],[1,1,1],[1,1,0],[0,1,0]]},
  {n:[0,-1,0],v:[[0,0,0],[1,0,0],[1,0,1],[0,0,1]]},
  {n:[0,0,1],v:[[0,0,1],[1,0,1],[1,1,1],[0,1,1]]},
  {n:[0,0,-1],v:[[1,0,0],[0,0,0],[0,1,0],[1,1,0]]},
];
export class VoxelWorld {
  constructor(definition) {
    this.definition=definition;this.size=[...definition.size];this.seed=seedHash(definition.seed);
    this.blocks=new Map(definition.blocks.map(b=>[b.id,b]));this.data=new Uint8Array(this.size[0]*this.size[1]*this.size[2]);
    this.dirty=new Set();this.changed=new Set();this.revision=0;
    this.material=Object.fromEntries([...definition.blocks].sort((a,b)=>b.id-a.id).map(b=>[b.material,b.id]));
    this.generate();
  }
  inside(x,y,z){return Number.isInteger(x)&&Number.isInteger(y)&&Number.isInteger(z)&&x>=0&&y>=0&&z>=0&&x<this.size[0]&&y<this.size[1]&&z<this.size[2];}
  index(x,y,z){return x+this.size[0]*(z+this.size[2]*y);}
  get(x,y,z){return this.inside(x,y,z)?this.data[this.index(x,y,z)]:0;}
  chunkKey(x,y,z){return `${Math.floor(x/16)},${Math.floor(y/16)},${Math.floor(z/16)}`;}
  set(x,y,z,id) {
    if(!this.inside(x,y,z)||y===0||!Number.isInteger(id)||(id!==0&&!this.blocks.has(id)))return false;
    const i=this.index(x,y,z);if(this.data[i]===id)return false;this.data[i]=id;this.revision++;
    this.changed.add(this.chunkKey(x,y,z));this.dirty.add(this.chunkKey(x,y,z));
    for(const {n} of faces){const p=[x+n[0],y+n[1],z+n[2]];if(this.inside(...p))this.dirty.add(this.chunkKey(...p));}
    return true;
  }
  height(x,z) {
    const [w,h,d]=this.size,m=this.material;
    const smooth=(scale)=>{
      const gx=Math.floor(x/scale),gz=Math.floor(z/scale),u=x/scale-gx,v=z/scale-gz;
      const a=u*u*(3-2*u),b=v*v*(3-2*v);
      return (noise(this.seed,gx,gz)*(1-a)+noise(this.seed,gx+1,gz)*a)*(1-b)+(noise(this.seed,gx,gz+1)*(1-a)+noise(this.seed,gx+1,gz+1)*a)*b;
    };
    let top=Math.floor(h*.24);
    if(this.definition.terrain!=='flat')top+=Math.floor(smooth(12)*6+smooth(5)*2);
    if(this.definition.terrain==='island')top-=Math.floor(Math.max(0,Math.hypot((x-w/2)/(w/2),(z-d/2)/(d/2))-.35)*h*.32);
    return Math.max(2,Math.min(h-8,top));
  }
  generate() {
    const [w,h,d]=this.size,m=this.material;
    for(let z=0;z<d;z++)for(let x=0;x<w;x++) {
      const top=this.height(x,z);
      for(let y=0;y<=top;y++)this.data[this.index(x,y,z)]=y===top?(top<5?m.sand:m.grass):y>top-3?m.dirt:noise(this.seed+y,x,z)>.966?m.ore:m.stone;
    }
    const cx=Math.floor(w/2),cz=Math.floor(d/2);
    for(let z=3;z<d-3;z+=3)for(let x=3;x<w-3;x+=3)if(noise(this.seed+19,x,z)>.77&&Math.hypot(x-cx,z-cz)>5&&this.height(x,z)>5)this.tree(x,z);
    // Every generated world has a reachable first resource; no test-only mutations.
    this.tree(cx,cz-4);
    for(let y=0;y<h/16;y++)for(let z=0;z<d/16;z++)for(let x=0;x<w/16;x++)this.dirty.add(`${x},${y},${z}`);
  }
  tree(x,z) {
    const m=this.material,y=this.height(x,z)+1;
    for(let dy=2;dy<5;dy++)for(let dz=-2;dz<=2;dz++)for(let dx=-2;dx<=2;dx++)if(Math.abs(dx)+Math.abs(dz)<4&&this.inside(x+dx,y+dy,z+dz)&&!this.get(x+dx,y+dy,z+dz))this.data[this.index(x+dx,y+dy,z+dz)]=m.leaves;
    for(let dy=0;dy<4;dy++)this.data[this.index(x,y+dy,z)]=m.wood;
  }
  spawn() {
    const x=Math.floor(this.size[0]/2),z=Math.floor(this.size[2]/2);
    for(let y=this.size[1]-3;y>0;y--)if(this.get(x,y,z)&&!this.get(x,y+1,z)&&!this.get(x,y+2,z))return {x:x+.5,y:y+1.02,z:z+.5};
    return {x:x+.5,y:this.size[1]-2,z:z+.5};
  }
  collides(p,width=.6,height=1.8) {
    const [w,h,d]=this.size,r=width/2;
    if(p.x-r<0||p.z-r<0||p.x+r>w||p.z+r>d||p.y<1||p.y+height>h)return true;
    for(let y=Math.floor(p.y+1e-5);y<=Math.floor(p.y+height-1e-5);y++)for(let z=Math.floor(p.z-r+1e-5);z<=Math.floor(p.z+r-1e-5);z++)for(let x=Math.floor(p.x-r+1e-5);x<=Math.floor(p.x+r-1e-5);x++)if(this.get(x,y,z))return true;
    return false;
  }
  move(p,delta,width=.6,height=1.8) {
    const count=Math.max(1,Math.ceil(Math.max(...Object.values(delta).map(Math.abs))/.15)),blocked={x:false,y:false,z:false};
    for(let i=0;i<count;i++)for(const axis of ['x','z','y']){
      const n=(delta[axis]||0)/count;if(!n)continue;const next={...p,[axis]:p[axis]+n};
      if(this.collides(next,width,height))blocked[axis]=true;else p[axis]+=n;
    }
    return blocked;
  }
  ray(origin,direction,range=6) {
    if(![...Object.values(origin),...direction,range].every(Number.isFinite)||range<0||range>8)return null;
    const length=Math.hypot(...direction);if(length<1e-6)return null;
    const dir=direction.map(v=>v/length),cell=[Math.floor(origin.x),Math.floor(origin.y),Math.floor(origin.z)],o=[origin.x,origin.y,origin.z];
    const step=dir.map(v=>Math.sign(v)),delta=dir.map(v=>v?Math.abs(1/v):Infinity);
    const next=dir.map((v,a)=>v?(cell[a]+(v>0?1:0)-o[a])/v:Infinity);
    let distance=0,previous=[...cell],normal=[0,0,0];
    for(let n=0;n<64&&distance<=range;n++){
      const id=this.get(...cell);if(id)return {cell:[...cell],previous,normal,id,distance};
      previous=[...cell];const a=next[0]<next[1]?(next[0]<next[2]?0:2):(next[1]<next[2]?1:2);
      distance=next[a];next[a]+=delta[a];cell[a]+=step[a];normal=[0,0,0];normal[a]=-step[a];
    }
    return null;
  }
  mesh(key) {
    const c=key.split(',').map(Number),positions=[],normals=[],uv=[],indices=[];
    for(let y=c[1]*16;y<(c[1]+1)*16;y++)for(let z=c[2]*16;z<(c[2]+1)*16;z++)for(let x=c[0]*16;x<(c[0]+1)*16;x++){
      const id=this.get(x,y,z);if(!id)continue;
      for(const f of faces){if(this.get(x+f.n[0],y+f.n[1],z+f.n[2]))continue;
        const base=positions.length/3,tile=id-1,tx=tile%8,ty=Math.floor(tile/8),uvs=[[0,0],[1,0],[1,1],[0,1]];
        f.v.forEach((v,i)=>{positions.push(x+v[0],y+v[1],z+v[2]);normals.push(...f.n);uv.push((tx+.04+uvs[i][0]*.92)/8,(ty+.04+uvs[i][1]*.92)/8);});
        indices.push(base,base+1,base+2,base,base+2,base+3);
      }
    }
    return {positions,normals,uv,indices};
  }
  snapshotChunks() {
    return [...this.changed].sort().map(key=>{
      const c=key.split(',').map(Number),blocks=[];
      for(let y=0;y<16;y++)for(let z=0;z<16;z++)for(let x=0;x<16;x++)blocks.push(this.get(c[0]*16+x,c[1]*16+y,c[2]*16+z));
      // Run lengths keep a completely edited maximum-size world below 4 MiB in typical play.
      const runs=[];for(const id of blocks){if(runs.length&&runs[runs.length-2]===id)runs[runs.length-1]++;else runs.push(id,1);}
      return {position:c,runs};
    });
  }
  restoreChunks(chunks) {
    for(const {position:c,runs} of chunks){let i=0;
      for(let r=0;r<runs.length;r+=2)for(let n=0;n<runs[r+1];n++,i++){
        const x=c[0]*16+i%16,z=c[2]*16+Math.floor(i/16)%16,y=c[1]*16+Math.floor(i/256);
        this.data[this.index(x,y,z)]=runs[r];
      }
      this.changed.add(c.join(','));
    }
    for(let y=0;y<this.size[1]/16;y++)for(let z=0;z<this.size[2]/16;z++)for(let x=0;x<this.size[0]/16;x++)this.dirty.add(`${x},${y},${z}`);
    this.revision++;
  }
}
