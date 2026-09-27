// Shared, scene-independent swept collision checks. Metres, Y up.
const GAP = .06;
const clamp = (n, a, b) => Math.max(a, Math.min(b, n));
const angleDelta = (a, b) => Math.atan2(Math.sin(b-a), Math.cos(b-a));
const pose = p => ({x:p.x, y:p.y || 0, z:p.z, heading:p.heading || 0});

// A short chain of overlapping circles encloses the full rectangular footprint,
// including its corners. Unlike one large circle it permits passing beside a tram.
export function bodyShape(bounds, scale=1) {
  const lo=bounds.min.map(v=>v*scale), hi=bounds.max.map(v=>v*scale);
  const hx=(hi[0]-lo[0])/2, hz=(hi[2]-lo[2])/2, longX=hx>hz;
  const major=Math.max(hx,hz), minor=Math.min(hx,hz), n=Math.min(7,Math.max(1,Math.ceil(major/Math.max(.3,minor))));
  const half=major/n, circles=[];
  for(let i=0;i<n;i++) {
    const offset=-major+half+i*half*2;
    circles.push({x:(lo[0]+hi[0])/2+(longX?offset:0),z:(lo[2]+hi[2])/2+(longX?0:offset),r:Math.hypot(minor,half)});
  }
  return {circles,minY:lo[1],maxY:hi[1],reach:Math.max(...circles.map(c=>Math.hypot(c.x,c.z)+c.r))};
}
function circleAt(c,p) {
  const co=Math.cos(p.heading),si=Math.sin(p.heading);
  return {x:p.x+c.x*co+c.z*si,z:p.z-c.x*si+c.z*co,r:c.r};
}
function slab(a,b,lo,hi,range) {
  const d=b-a;
  if(Math.abs(d)<1e-10)return a>=lo&&a<=hi;
  const x=(lo-a)/d,y=(hi-a)/d;
  range[0]=Math.max(range[0],Math.min(x,y));range[1]=Math.min(range[1],Math.max(x,y));
  return range[0]<=range[1];
}
function vertical(a,ap,an,b,bp,bn) {
  const range=[0,1];
  if(!slab(ap.y-bp.y,an.y-bn.y,b.minY-a.maxY+GAP,b.maxY-a.minY-GAP,range))return null;
  return range;
}
function pairHit(a,ap,an,b,bp,bn) {
  const reach=a.reach+b.reach+GAP;
  if(Math.min(ap.x,an.x)-Math.max(bp.x,bn.x)>reach||Math.min(bp.x,bn.x)-Math.max(ap.x,an.x)>reach||Math.min(ap.z,an.z)-Math.max(bp.z,bn.z)>reach||Math.min(bp.z,bn.z)-Math.max(ap.z,an.z)>reach)return false;
  const interval=vertical(a,ap,an,b,bp,bn);if(!interval)return false;
  // Rotation sweeps an arc; expanding the chord by arc length is conservative.
  for(const ac of a.circles)for(const bc of b.circles) {
    const pad=GAP+Math.abs(angleDelta(ap.heading,an.heading))*Math.hypot(ac.x,ac.z)+Math.abs(angleDelta(bp.heading,bn.heading))*Math.hypot(bc.x,bc.z);
    const p=circleAt(ac,ap),q=circleAt(ac,an),u=circleAt(bc,bp),v=circleAt(bc,bn);
    const x=p.x-u.x,z=p.z-u.z,dx=q.x-v.x-x,dz=q.z-v.z-z;
    const t=clamp(-(x*dx+z*dz)/Math.max(1e-12,dx*dx+dz*dz),...interval),r=ac.r+bc.r+pad;
    if((x+dx*t)**2+(z+dz*t)**2<r*r)return true;
  }
  return false;
}
function solidHit(body,from,to,solid) {
  const r=body.reach+GAP;
  if(solid.world&&(Math.min(from.x,to.x)-r>solid.world[2]||Math.max(from.x,to.x)+r<solid.world[0]||Math.min(from.z,to.z)-r>solid.world[3]||Math.max(from.z,to.z)+r<solid.world[1]))return false;
  if(Math.min(from.y,to.y)+body.minY>=solid.max[1]||Math.max(from.y,to.y)+body.maxY<=solid.min[1])return false;
  const co=Math.cos(solid.heading||0),si=Math.sin(solid.heading||0);
  const local=p=>({x:(p.x-solid.x)*co-(p.z-solid.z)*si,z:(p.x-solid.x)*si+(p.z-solid.z)*co});
  for(const circle of body.circles) {
    const p=local(circleAt(circle,from)),q=local(circleAt(circle,to));
    const r=circle.r+GAP+Math.abs(angleDelta(from.heading,to.heading))*Math.hypot(circle.x,circle.z),range=[0,1];
    if(slab(from.y,to.y,solid.min[1]-body.maxY+GAP,solid.max[1]-body.minY-GAP,range)&&
       slab(p.x,q.x,solid.min[0]-r,solid.max[0]+r,range)&&slab(p.z,q.z,solid.min[2]-r,solid.max[2]+r,range))return true;
  }
  return false;
}

export function createTraffic() {
  const bodies=new Map(),solids=new Map(),requests=new Map();
  let stops=0,steps=0,clock=0,revision=0;
  const ignored=(a,b)=>a.id===b.id||a.ignore===b.id||b.ignore===a.id||a.enabled===false||b.enabled===false;
  function obstruction(body,from,to,actors=true) {
    if(![to.x,to.y,to.z,to.heading].every(Number.isFinite))return 'invalid-pose';
    for(const s of solids.values())if(s.enabled!==false&&!(s.owner&&(s.owner===body.ignore||s.owner===body.id))&&solidHit(body,from,to,s))return s.id;
    if(actors)for(const b of bodies.values())if(!ignored(body,b)&&pairHit(body,from,to,b,b,b))return b.id;
    return null;
  }
  const clear=(body,from,to,actors=true)=>!obstruction(body,from,to,actors);
  function register(id,shape,initial,priority=0) {
    if(bodies.has(id))return bodies.get(id);
    const body={id,...shape,...pose(initial),priority,enabled:true,blocked:0};
    if(!clear(body,body,body))return null;
    bodies.set(id,body);return body;
  }
  function findFree(body,position,maxRadius=12) {
    const candidate={...body,enabled:true};
    for(let radius=0;radius<=maxRadius;radius+=.5)for(let n=0;n<(radius?24:1);n++) {
      const p={...pose(position),x:position.x+Math.cos(n*Math.PI/12)*radius,z:position.z+Math.sin(n*Math.PI/12)*radius};
      if(clear(candidate,p,p))return p;
    }
    return null;
  }
  function relocate(body,position,maxRadius=12){const p=findFree(body,position,maxRadius);if(!p)return false;Object.assign(body,p);return true;}
  function solve(dt) {
    // ponytail: pairwise checks suit the bounded 24 residents/6 drones/2 trams;
    // use a spatial grid only if the population contract grows substantially.
    const rows=[...bodies.values()].filter(b=>b.enabled!==false).map(b=>{
      const request=requests.get(b.id),next=request?.next||pose(b);
      return {body:b,from:pose(b),next,request,accepted:clear(b,b,next,false)};
    });
    // A rejected intention becomes stationary. Recheck until no accepted path
    // can hit a newly stopped actor; priority must never mean pushing it aside.
    for(let pass=0;pass<=rows.length;pass++) {
      let changed=false;
      for(const r of rows)if(r.accepted&&r.body.follow&&rows.some(other=>other.body.id===r.body.follow&&!other.accepted)){r.accepted=false;changed=true;}
      for(let i=0;i<rows.length;i++)for(let j=i+1;j<rows.length;j++) {
        const a=rows[i],b=rows[j];if(ignored(a.body,b.body)||(!a.accepted&&!b.accepted))continue;
        const an=a.accepted?a.next:a.from,bn=b.accepted?b.next:b.from;
        if(!pairHit(a.body,a.from,an,b.body,b.from,bn))continue;
        const moving=r=>r.request&&r.accepted&&(Math.hypot(r.next.x-r.from.x,r.next.y-r.from.y,r.next.z-r.from.z)>1e-8||Math.abs(angleDelta(r.from.heading,r.next.heading))>1e-8);
        const am=moving(a),bm=moving(b);if(!am&&!bm)continue;
        const ax=a.next.x-a.from.x,az=a.next.z-a.from.z,bx=b.next.x-b.from.x,bz=b.next.z-b.from.z;
        const following=ax*bx+az*bz>.8*Math.hypot(ax,az)*Math.hypot(bx,bz);
        const aAlone=am&&!pairHit(a.body,a.from,a.next,b.body,b.from,b.from),bAlone=bm&&!pairHit(a.body,a.from,a.from,b.body,b.from,b.next);
        const loser=!am?b:!bm?a:aAlone&&!bAlone?b:bAlone&&!aAlone?a:a.body.priority!==b.body.priority?(a.body.priority<b.body.priority?a:b):
          following?((b.from.x-a.from.x)*ax+(b.from.z-a.from.z)*az>0?a:b):(a.body.id>b.body.id?a:b);
        loser.accepted=false;changed=true;
      }
      if(!changed)break;
    }
    for(const r of rows) {
      if(r.accepted){Object.assign(r.body,r.next);r.body.blocked=0;r.body.obstruction=null;}
      else if(r.request){r.body.blocked+=dt;r.body.obstruction=obstruction(r.body,r.from,r.next);stops++;}
    }
    for(const r of rows)r.request?.commit?.(r.accepted,r.body);
    requests.clear();steps++;clock+=dt;
  }
  return {
    register,relocate,findFree,clear,obstruction,solve,
    begin(){requests.clear();},
    propose(body,next,commit){if(body?.enabled!==false&&body)requests.set(body.id,{next:pose(next),commit});},
    remove(id){bodies.delete(id);requests.delete(id);},
    solid(id,box){const old=solids.get(id),next={id,x:0,z:0,heading:0,...box};if(old&&old.x===next.x&&old.z===next.z&&old.heading===next.heading&&old.enabled===next.enabled&&old.min.every((v,i)=>v===next.min[i])&&old.max.every((v,i)=>v===next.max[i]))return;
      const c=Math.cos(next.heading),s=Math.sin(next.heading),xs=[],zs=[];for(const x of [next.min[0],next.max[0]])for(const z of [next.min[2],next.max[2]]){xs.push(next.x+x*c+z*s);zs.push(next.z-x*s+z*c);}next.world=[Math.min(...xs),Math.min(...zs),Math.max(...xs),Math.max(...zs)];
      solids.set(id,next);revision++;},
    removeSolid(id){if(solids.delete(id))revision++;},
    removeOwner(owner){for(const [id,s]of solids)if(s.owner===owner){solids.delete(id);revision++;}},
    revision:()=>revision,
    solidClear(solid){return [...bodies.values()].every(b=>b.enabled===false||(solid.owner&&b.ignore===solid.owner)||!solidHit(b,b,b,{x:0,z:0,heading:0,...solid}));},
    ceiling(body,x,z){let y=0;for(const s of solids.values())if(s.enabled!==false&&solidHit(body,{x,y:s.min[1],z,heading:0},{x,y:s.min[1],z,heading:0},s))y=Math.max(y,s.max[1]-body.minY+1);return y;},
    body:id=>bodies.get(id),
    neighbors(body,distance=12){return [...bodies.values()].filter(b=>!ignored(body,b)&&Math.abs(b.y-body.y)<4&&Math.hypot(b.x-body.x,b.z-body.z)<distance);},
    stats:()=>({bodies:bodies.size,solids:solids.size,stops,steps,time:clock,poses:[...bodies.values()].filter(b=>b.enabled!==false).map(b=>({id:b.id,x:b.x,y:b.y,z:b.z,heading:b.heading,blocked:b.blocked,obstruction:b.obstruction}))}),
    dispose(){bodies.clear();solids.clear();requests.clear();},
  };
}

export function fixedSteps(callback,step=1/60) {
  let accumulator=0;
  return dt=>{
    if(!(dt>0)){accumulator=0;return 0;}
    accumulator=Math.min(accumulator+dt,step*6);
    let count=0;while(accumulator>=step-1e-9){callback(step);accumulator-=step;count++;}return count;
  };
}
