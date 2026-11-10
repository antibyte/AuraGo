import {createWayfinder,livingPlaces} from './sysworld-places.js';
import {bodyShape} from './sysworld-traffic.js';
import {createSocialSignals} from './sysworld-signals.js';
import {lobbyGround} from './sysworld-lobbies.js';
import {onCarriageway} from './sysworld-ground.js';
import {springStep,findRig,adaptFeet} from './sysworld-footing.js';

// A walking robot is about 0.8 m across the shoulders; swinging arms may overlap furniture. The traffic body keeps
// the model's height but clamps the lateral half-extents so that the envelope (routing reach) is 0.5 m, which
// fits the 2.0-3.2 m doors and the 1.2 m aisles. Residents keep a personal-space margin among themselves.
export const RESIDENT_HALF=.35;
const clampBounds=b=>{const h=[0,2].map(k=>Math.min(RESIDENT_HALF/1.15,(b.max[k]-b.min[k])/2));
  return {min:[-h[0],b.min[1],-h[1]],max:[h[0],b.max[1],h[1]]};};
// The traffic body of a resident: one round envelope (covers every articulated pose, permits turning in place).
export function residentShape(bounds){const shape=bodyShape(clampBounds(bounds),1.15);shape.circles=[{x:0,z:0,r:shape.reach}];return shape;}
const turn=(from,to,dt)=>from+Math.atan2(Math.sin(to-from),Math.cos(to-from))*Math.min(1,dt*5);
// The walk and carry clips are authored for 1.65 m/s with planted feet; a faster stride plays them faster.
const stride=(a,name,speed)=>{const action=a.play?.(name);if(action)action.timeScale=speed/1.65;};
export function createSociety(scene,{traffic,camera,floor,catalog,onSelect,onDemonstrate,active,onGait,shadows}) {
  const actors=[],places=new Map(),slots=[],meetings=[],finder=createWayfinder(traffic,floor,catalog),signals=createSocialSignals(scene);
  let clock=0,selected=null,replay=false,tier='high',disposed=false,completed=0,selectedStatus='',lobbyVisits=0;
  const byPlace={},byResident={};
  // A tram that has not moved for longer than its 6 s station dwell is held up (a visitor on the rails): residents
  // neither wait for it nor flee from it. It brakes for whoever stands in front of it.
  const still=new Map();
  function stalled(b){const s=still.get(b.id);
    if(!s||Math.hypot(b.x-s.x,b.z-s.z)>.05||Math.abs(b.heading-s.heading)>.01){still.set(b.id,{x:b.x,z:b.z,heading:b.heading,since:clock});return false;}
    return clock-s.since>8;}
  const limit=()=>tier==='low'?3:tier==='medium'?10:19;
  function release(a) {if(a.slot){a.slot.owner=null;a.slot=null;}a.path=[];a.goal=null;}
  function describe(a) {return a?{id:a.id,role:a.role,state:a.state,source:a.source||'ambient',guided:!!a.guide}:null;}
  function select(a) {selected=a?.id||null;selectedStatus=a?.state||'';onSelect?.(describe(a));}
  function add(model,index,bounds) {
    const id='resident-'+index,shape=residentShape(bounds),role=['courier','technician','archivist'][index%3];let body=null;
    const near=slots.filter(s=>s.role===role).map(s=>({x:s.x,z:s.z+2.2,y:floor(s.x,s.z+2.2),heading:Math.PI}));
    // Residents of one role start at different workstations (two slots per place), so every place has a neighbour.
    near.push(...near.splice(0,Math.floor(index/3)*2%Math.max(1,near.length)));
    const candidates=[...near,...finder.points.map((_,n)=>finder.points[(index*7+n)%finder.points.length])];
    for(const p of candidates) {
      // Never spawn on a podium or a stair flight.
      if(lobbyGround(p.x,p.z)!==null)continue;
      body=traffic.register(id,shape,p,1);if(body)break;
    }
    if(!body){model.node.visible=false;return null;}
    const a={...model,id,index,body,role,state:'idle',path:[],until:index*.3,trip:index,cooldown:8+index*.7,source:'ambient',carrying:false,parcels:[],soundX:body.x,soundZ:body.z};
    model.node.traverse(n=>{if(n.isMesh&&/parcel/.test(n.name)){a.parcels.push(n);n.visible=false;}});
    actors.push(a);model.node.position.set(body.x,body.y,body.z);
    // Damped height and slope pitch for every resident; feet only when the rig is complete.
    a.height={y:body.y,v:0};a.pitch={y:0,v:0};model.node.rotation.order='YXZ';a.rig=findRig(model.node);a.shadow=shadows?.claim()||null;
    return a;
  }
  function patrol(id,body,node,pause) {const a={id,body,node,pause,role:'patrol',state:'patrol',cooldown:7+actors.length,path:[],soundX:body.x,soundZ:body.z};actors.push(a);return a;}
  // A route whose first real leg is already blocked at the body's own look-ahead probe cannot be walked from here
  // (a body that stands a hand's breadth off the graph line clips a fixture the line itself passes). Choose another.
  function openAhead(a,path) {
    const w=path.find(p=>Math.hypot(p.x-a.body.x,p.z-a.body.z)>.5);if(!w)return true;
    const dx=w.x-a.body.x,dz=w.z-a.body.z,d=Math.hypot(dx,dz),look=Math.min(1.8,d);
    return traffic.clear(a.body,a.body,{x:a.body.x+dx/d*look,y:a.body.y,z:a.body.z+dz/d*look,heading:a.body.heading});
  }
  function choose(a) {
    release(a);a.source='ambient';a.state='idle';a.until=clock+2;
    const eligible=slots.filter(s=>!s.owner&&!(s.lobby&&tier==='low')&&(a.role==='courier'?s.place===(a.carrying?'meeting-charge':'parcel-sorter'):s.role===a.role||s.place==='meeting-charge'));
    // The first job is the nearest workstation: a resident spawned beside a place starts there.
    const first=a.trip===a.index;
    if(first)eligible.sort((p,q)=>Math.hypot(p.x-a.body.x,p.z-a.body.z)-Math.hypot(q.x-a.body.x,q.z-a.body.z));
    // A short, reachable street walk, taken when no other workstation is free.
    const wander=()=>{
      for(let n=0;n<24;n++) {
        const p=finder.streetPoints[(a.trip++*13+a.index)%finder.streetPoints.length];
        if(Math.hypot(p.x-a.body.x,p.z-a.body.z)>30)continue;
        const path=finder.route(a.body,p);
        if(path.length&&openAhead(a,path)){a.path=path;a.state='walk';return true;}
      }
      return false;
    };
    // The place just worked at is skipped for the next two choices. When nothing else is free the resident
    // walks on instead of working the same place again; it falls back to that place only when it cannot walk.
    // After an outdoor job a resident prefers a free lobby workpoint within 70 m, so the halls are not left to chance
    // while the outdoor places (now six, two of them far from the residents' homes) keep attracting every choice.
    const indoorsNext=a.outdoor&&eligible.some(s=>s.lobby&&Math.hypot(s.x-a.body.x,s.z-a.body.z)<=70);
    for(const pass of [0,1]) {
      if(pass===1&&a.avoidLeft>0&&wander()){a.avoidLeft--;return;}
      for(let n=0;n<eligible.length;n++) {
        const slot=eligible[((first?0:a.trip)+n)%eligible.length];
        if(pass===0&&a.avoidLeft>0&&slot.place===a.avoid)continue;
        if(pass===0&&indoorsNext&&!slot.lobby)continue;
        const path=finder.route(a.body,slot);
        if(!path.length||!openAhead(a,path))continue;
        if(a.avoidLeft>0)a.avoidLeft--;
        slot.owner=a.id;a.slot=slot;a.path=path;a.goal=slot;a.trip++;a.state=a.carrying?'carry':'walk';return;
      }
    }
    // All workstations occupied: take a short, reachable walk instead of stacking.
    wander();
  }
  // Stand-off: of two residents in each other's way, the higher index (after waiting 1.2 s) steps aside (or back) and holds, then resumes.
  function standOff(a,forward,probe) {
    const obs=traffic.obstruction(a.body,a.body,probe);let p=null;
    for(const o of actors) {
      if(o===a||o.index===undefined||o.index>=a.index||!o.body.enabled)continue;
      const dx=o.body.x-a.body.x,dz=o.body.z-a.body.z;
      if(o.id===obs||(dx*dx+dz*dz<2.56&&dx*forward.x+dz*forward.z>0)){p=o;break;}
    }
    if(!p)return false;
    const px=p.body.x-a.body.x,pz=p.body.z-a.body.z,onRoad=onCarriageway(a.body.x,a.body.z);
    let tx=a.body.x,tz=a.body.z,best=Infinity;
    for(const [sx,sz] of [[forward.z,-forward.x],[-forward.z,forward.x]]) {
      const c={x:a.body.x+sx*.8,y:a.body.y,z:a.body.z+sz*.8,heading:a.body.heading},away=sx*px+sz*pz;
      if(away>=best||!traffic.clear(a.body,a.body,c)||(!onRoad&&onCarriageway(c.x,c.z)))continue;
      best=away;tx=c.x;tz=c.z;
    }
    if(best===Infinity) {
      const c={x:a.body.x-forward.x*.8,y:a.body.y,z:a.body.z-forward.z*.8,heading:a.body.heading};
      if(traffic.clear(a.body,a.body,c)&&(onRoad||!onCarriageway(c.x,c.z))){tx=c.x;tz=c.z;}
    }
    a.back={x:tx,z:tz,until:clock+1.5+(a.index%5)*.25};a.wait=0;a.state='idle';
    return true;
  }
  function endMeeting(m) {
    for(const a of [m.a,m.b]){release(a);a.meeting=null;a.cooldown=clock+12+(a.index||0);a.pause?.(false);a.state=a.pause?'patrol':'idle';a.until=clock+1;}
  }
  function suspend() {
    meetings.forEach(endMeeting);meetings.length=0;signals.clear();selected=null;onSelect?.(null);
    for(const a of actors){a.guide=null;a.pause?.(false,a.body);a.trafficYield=false;a.yieldTram=null;if(!a.pause){release(a);a.state='idle';a.until=clock+1;}}
  }
  function update(dt,animated) {
    if(disposed)return;
    if(animated)clock+=dt;
    for(const a of actors) {
      const enabled=a.pause||a.index<limit();
      if(enabled&&!a.body.enabled){if(!traffic.relocate(a.body,a.body)){a.node.visible=false;continue;}a.body.enabled=true;a.node.position.set(a.body.x,a.body.y,a.body.z);}
      if(!enabled&&a.body.enabled){release(a);a.body.enabled=false;if(a.meeting)endMeeting(a.meeting);}
      a.node.visible=!!enabled;
      // The travelled distance is the honest cue for a footfall: a resident that
      // stands still is silent, a resident that walks is heard at its own pace.
      if(a.soundX==null){a.soundX=a.body.x;a.soundZ=a.body.z;}
      const moved=Math.hypot(a.body.x-a.soundX,a.body.z-a.soundZ);
      a.soundX=a.body.x;a.soundZ=a.body.z;
      onGait?.(a.id,a.body.x,a.body.y,a.body.z,moved>.004,!!a.carrying,a.role==='patrol'?'glide':'');
      if(!enabled||!animated)continue;
      const tram=traffic.neighbors(a.body,22).sort((b,c)=>Math.hypot(b.x-a.body.x,b.z-a.body.z)-Math.hypot(c.x-a.body.x,c.z-a.body.z)).find(b=>{
        if(!b.id.startsWith('tram-')||stalled(b))return false;const dx=a.body.x-b.x,dz=a.body.z-b.z,c=Math.cos(b.heading),s=Math.sin(b.heading);
        return Math.abs(dx*c-dz*s)<a.body.reach+(a.trafficYield?4:2.3)&&dx*s+dz*c>-8-a.body.reach&&dx*s+dz*c<14;
      });
      if(!tram&&a.yieldTram){
        const train=traffic.body(a.yieldTram),dx=train?a.body.x-train.x:0,dz=train?a.body.z-train.z:0;
        // Stay at the crossing until the entire train has passed. Immediately
        // rejoining a patrol would otherwise keep stepping back in front of it.
        if(train?.enabled&&!stalled(train)&&Math.hypot(dx,dz)<26&&dx*Math.sin(train.heading)+dz*Math.cos(train.heading)>-train.reach-a.body.reach-1){a.pause?.(true);a.play?.('idle');continue;}
        a.yieldTram=null;
      }
      if(tram){
        a.trafficYield=true;a.yieldTram=tram.id;
        if(a.meeting){const meeting=a.meeting;endMeeting(meeting);meetings.splice(meetings.indexOf(meeting),1);}
        a.pause?.(true);
        const c=Math.cos(tram.heading),s=Math.sin(tram.heading),side=Math.sign((a.body.x-tram.x)*c-(a.body.z-tram.z)*s)||1;
        const lateral=(a.body.x-tram.x)*c-(a.body.z-tram.z)*s,along=Math.sign((a.body.x-tram.x)*s+(a.body.z-tram.z)*c)||1;
        // Commit only to a side with a complete exit from the rail corridor.
        // Short alternating probes can otherwise oscillate beside street furniture.
        const exits=[[c*side,-s*side,a.body.reach+4.5-Math.abs(lateral)],[-c*side,s*side,a.body.reach+4.5+Math.abs(lateral)],[s*along,c*along,1.2]];
        // At bends a diagonal can be the only gap between the tram and another
        // resident. Require increasing distance, so fallback steps cannot oscillate.
        const escape=Array.from({length:16},(_,i)=>[Math.cos(i*Math.PI/8),Math.sin(i*Math.PI/8),2]);
        const away=([x,z])=>x*(a.body.x-tram.x)+z*(a.body.z-tram.z);
        exits.push(...escape.filter(v=>away(v)>.2).sort((a,b)=>away(b)-away(a)));
        // Keep the chosen side for a moment: re-picking it every frame makes the
        // resident step left and right on the spot while the tram passes.
        const held=a.escape&&clock<a.escape.until?a.escape:null;
        const order=held?exits.filter(e=>e[0]===held.x&&e[1]===held.z).concat(exits.filter(e=>!(e[0]===held.x&&e[1]===held.z))):exits;
        let stepped=false;
        for(const [dx,dz,distance] of order){
          const probe={x:a.body.x+dx*distance,y:a.body.y,z:a.body.z+dz*distance,heading:a.body.heading};
          if(!traffic.clear(a.body,a.body,probe))continue;
          const next={...probe,x:a.body.x+dx*dt*1.8,z:a.body.z+dz*dt*1.8};next.y=a.pause?a.body.y:floor(next.x,next.z);
          traffic.propose(a.body,next,(_,b)=>{a.node.position.set(b.x,b.y,b.z);stride(a,'walk',1.8);});
          a.escape={x:dx,z:dz,until:clock+.8};a.state='yield';a.cooldown=clock+5;stepped=true;break;
        }
        // Nowhere free to step onto: stand still instead of marching on the spot.
        if(!stepped){a.play?.('idle');a.state='yield';}
        continue;
      }
      if(a.trafficYield){a.trafficYield=false;a.pause?.(false,a.body);a.state=a.pause?'patrol':a.path.length?'walk':'idle';}
      if(a.meeting)continue;
      if(a.pause){if(a.state==='greet'&&clock>a.until){a.pause(false);a.state='patrol';}continue;}
      if(a.guide&&Math.hypot(camera.position.x-a.body.x,camera.position.z-a.body.z)>9){a.play('idle');continue;}
      if(a.back) {
        const t=a.back,bx=t.x-a.body.x,bz=t.z-a.body.z,bd=Math.hypot(bx,bz);
        if(clock>=t.until)a.back=null;
        else {
          if(bd>.03) {
            const step=Math.min(bd,1.2*dt),nx=a.body.x+bx/bd*step,nz=a.body.z+bz/bd*step;
            traffic.propose(a.body,{x:nx,y:floor(nx,nz),z:nz,heading:a.body.heading},(ok,b)=>{a.node.position.set(b.x,b.y,b.z);if(ok)stride(a,'walk',1.2);else a.play('idle');});
          } else a.play('idle');
          continue;
        }
      }
      if(a.path.length) {
        const goal=a.path[0],dx=goal.x-a.body.x,dz=goal.z-a.body.z,d=Math.hypot(dx,dz),speed=a.guide?1.9:1.65;
        if(d<.18){a.path.shift();continue;}
        const advance=Math.min(d,speed*dt),forward={x:dx/d,z:dz/d};
        let x=a.body.x+forward.x*advance,z=a.body.z+forward.z*advance;
        let heading=turn(a.body.heading,Math.atan2(dx,dz),dt);
        // Predict nearby traffic before entering a gap; prefer the right side.
        // Never look past the waypoint: a goal beside a fixture would otherwise probe into its collider.
        const look=Math.min(1.8,d);
        const probe={x:a.body.x+forward.x*look,y:floor(x,z),z:a.body.z+forward.z*look,heading};
        if(!traffic.clear(a.body,a.body,probe)) {
          // Step around the obstruction, but stay committed to one side and keep facing
          // the goal: alternating sides turn the resident left and right on the spot.
          // Backing up is no detour at all - it opens the way for one frame and sends
          // the resident back and forth in front of the obstacle.
          const sides=[[forward.z,-forward.x],[-forward.z,forward.x]];
          const held=a.detour&&clock<a.detour.until?sides.findIndex(s=>s[0]===a.detour.x&&s[1]===a.detour.z):-1;
          const order=held>=0?[sides[held],...sides.filter((_,i)=>i!==held)]:sides;
          let clear=false;
          for(const [sx,sz] of order) {
            const candidate={x:a.body.x+sx*1.3,y:a.body.y,z:a.body.z+sz*1.3,heading:a.body.heading};
            if(!traffic.clear(a.body,a.body,candidate))continue;
            // A sidestep never pushes a resident from pavement or kerb onto a carriageway.
            if(onCarriageway(candidate.x,candidate.z)&&!onCarriageway(a.body.x,a.body.z))continue;
            // The step must actually open the way. A sidestep that leaves the street
            // blocked sends the resident circling its obstacle instead of past it.
            // The probe starts at the candidate: a diagonal sweep from the resident
            // clips the corner of the obstacle and refuses a sidestep that fits.
            const beyond={x:candidate.x+forward.x*1.8,y:floor(candidate.x,candidate.z),z:candidate.z+forward.z*1.8,heading:a.body.heading};
            if(!traffic.clear(a.body,candidate,beyond))continue;
            x=a.body.x+sx*advance;z=a.body.z+sz*advance;a.detour={x:sx,z:sz,until:clock+1.2};clear=true;break;
          }
          if(!clear){x=a.body.x;z=a.body.z;}
        } else a.detour=null;
        if(!traffic.clear(a.body,a.body,{x,y:floor(x,z),z,heading},false))heading=a.body.heading;
        const moving=Math.hypot(x-a.body.x,z-a.body.z)>.001;
        traffic.propose(a.body,{x,y:floor(x,z),z,heading},(accepted,b)=>{
          a.node.position.set(b.x,b.y,b.z);a.node.rotation.y=b.heading;
          if(!accepted||!moving){a.wait=(a.wait||0)+dt;a.play('idle');a.state='yield';}
          else{a.wait=0;a.state=a.guide?'guide':a.carrying?'carry':'walk';stride(a,a.carrying?'carry':'walk',speed);}
          if(a.wait>1.2&&!a.back&&standOff(a,forward,probe))a.play('idle');
          // Blocked within reach of the workstation by a neighbour or fixture: the work starts from here.
          else if(a.wait>1.2&&a.slot&&a.path.length===1&&d<2.2){a.path.length=0;a.wait=0;}
          if(a.wait>2.5){a.wait=0;release(a);a.state=a.guide?'guide':'idle';a.until=clock+.5;if(a.guide)a.path=finder.route(a.body,a.guide);}
        });
      } else if(a.state==='walk'||a.state==='carry'||a.state==='guide'||a.state==='yield') {
        if(a.guide&&Math.hypot(a.body.x-a.guide.x,a.body.z-a.guide.z)>.5){a.play('idle');if(clock>a.until){a.path=finder.route(a.body,a.guide);a.until=clock+2.5;}continue;}
        a.guide=null;a.state=a.slot?.place==='meeting-charge'?'charge':a.slot?'work':'idle';a.until=clock+5+(a.index%4);
        a.play(a.state==='work'?'work':'idle');if(a.slot&&!a.slot.lobby)onDemonstrate?.(a.slot.place,5);if(a.slot?.lobby)lobbyVisits++;a.outdoor=!!a.slot&&!a.slot.lobby;a.avoid=a.slot?.place||null;a.avoidLeft=a.avoid?2:0;completed++;byResident[a.id]=(byResident[a.id]||0)+1;if(a.slot)byPlace[a.slot.place]=(byPlace[a.slot.place]||0)+1;
      } else if(clock>a.until){
        if(a.role==='courier'&&a.slot&&(a.state==='work'||a.state==='charge')){a.carrying=a.slot.place==='parcel-sorter';a.parcels.forEach(n=>{n.visible=a.carrying;});}
        choose(a);
      }
    }
    if(!animated){signals.update(0,false);return;}
    // Reserve both partners for one finite exchange. No overlapping conversations.
    for(const a of actors) {
      if(!a.body.enabled||a.trafficYield||a.meeting||a.guide||a.state==='greet'||clock<a.cooldown||onCarriageway(a.body.x,a.body.z))continue;
      const b=actors.find(b=>b!==a&&b.body.enabled&&!b.trafficYield&&!onCarriageway(b.body.x,b.body.z)&&b.state!=='greet'&&!b.meeting&&!b.guide&&clock>=b.cooldown&&Math.abs(a.body.y-b.body.y)<2&&Math.hypot(a.body.x-b.body.x,a.body.z-b.body.z)>2.8&&Math.hypot(a.body.x-b.body.x,a.body.z-b.body.z)<6);
      if(!b)continue;
      const m={a,b,at:clock,sent:false,replied:false};meetings.push(m);
      for(const p of [a,b]){p.meeting=m;p.state='exchange';p.pause?.(true);p.play?.('greet');}
    }
    for(let i=meetings.length-1;i>=0;i--) {
      const m=meetings[i],age=clock-m.at;
      if(age>4.8||!m.a.body.enabled||!m.b.body.enabled){endMeeting(m);meetings.splice(i,1);continue;}
      for(const [a,b]of [[m.a,m.b],[m.b,m.a]]) {
        traffic.propose(a.body,{...a.body,heading:turn(a.body.heading,Math.atan2(b.body.x-a.body.x,b.body.z-a.body.z),dt)},(_,body)=>{a.node.rotation.y=body.heading;});
      }
      if(age>.6&&!m.sent){signals.send(m.a.body,m.b.body);m.sent=true;}
      if(age>2.4&&!m.replied){signals.send(m.b.body,m.a.body);m.replied=true;}
    }
    signals.update(dt,true);
    const current=actors.find(a=>a.id===selected);if(current&&current.state!==selectedStatus){selectedStatus=current.state;onSelect?.({...describe(current),refresh:true});}
  }
  // One fixed step after the mixer has posed the clips and traffic has committed every move.
  function settle(dt) {
    if(disposed)return;
    const feet=tier!=='low';
    for(const a of actors) {
      if(!a.height)continue;                             // hovering patrols own their height
      if(!a.body.enabled||!a.node.visible){a.shadow?.hide();continue;}
      const b=a.body,y=springStep(a.height,b.y,dt);
      const fx=Math.sin(b.heading),fz=Math.cos(b.heading),slope=floor(b.x+fx*.35,b.z+fz*.35)-floor(b.x-fx*.35,b.z-fz*.35);
      const pitch=springStep(a.pitch,Math.max(-.08,Math.min(.08,Math.atan2(slope,.7)*.5)),dt);
      a.node.position.set(b.x,y,b.z);a.node.rotation.y=b.heading;a.node.rotation.x=-pitch;
      if(feet&&a.rig)adaptFeet(a.rig,a.node,floor);
      a.shadow?.set(b.x,floor(b.x,b.z),b.z,.62,.5);
    }
  }
  return {
    add,patrol,update,suspend,
    settle,
    addPlace(place,entry) {
      if(places.has(place.id))return;places.set(place.id,place);
      for(const point of entry.navigation.workpoints||[])slots.push({id:place.id+':'+point.id,place:place.id,role:place.role,x:place.x+point.position[0],y:floor(place.x,place.z),z:place.z+point.position[2],owner:null,lobby:!!place.lobby});
    },
    nearby() {
      const rows=[];for(const a of actors)if(a.body.enabled){const distance=Math.hypot(camera.position.x-a.body.x,camera.position.z-a.body.z);if(distance<5&&Math.abs(camera.position.y-a.body.y)<8)rows.push({kind:'resident',id:a.id,distance,x:a.body.x,z:a.body.z});}
      for(const p of places.values()){if(p.lobby||p.kit)continue;const distance=Math.hypot(camera.position.x-p.x,camera.position.z-p.z);if(distance<7)rows.push({kind:'demonstrate',id:p.id,distance,x:p.x,z:p.z});}return rows;
    },
    inspect(id){const a=actors.find(a=>a.id===id);select(a);return describe(a);},
    action(verb,destination) {
      const a=actors.find(a=>a.id===selected);if(!a)return null;
      if(a.meeting){const meeting=a.meeting,i=meetings.indexOf(meeting);endMeeting(meeting);if(i>=0)meetings.splice(i,1);}
      if(verb==='greet'){release(a);a.guide=null;a.pause?.(true);a.state='greet';a.until=clock+3;a.cooldown=clock+5;a.play?.('greet');}
      if(verb==='guide'&&!a.pause&&destination){
        release(a);
        for(const radius of [0,1.5,3]){
          for(let i=0;i<(radius?8:1)&&!a.path.length;i++){const x=destination.x+Math.cos(i*Math.PI/4)*radius,z=destination.z+Math.sin(i*Math.PI/4)*radius;a.path=finder.route(a.body,{x,z,y:floor(x,z)});}
          if(a.path.length)break;
        }
        a.guide=a.path.length?a.path[a.path.length-1]:null;a.state=a.guide?'guide':'idle';
      }
      if(verb==='cancel'){release(a);a.guide=null;a.pause?.(false);a.state=a.pause?'patrol':'idle';a.until=clock+1;}
      select(a);return describe(a);
    },
    demonstrate(id,reduced=false){if(places.has(id))onDemonstrate?.(id,reduced?0:6);},
    setTier(value){tier=value;},
    setReplay(value){if(replay!==value){replay=value;suspend();}},
    event(e) {
      if(replay||!active()||!['started','succeeded','failed','sanitized','progress'].includes(e.state)||!Number.isFinite(e.at)||Date.now()-e.at>2500||e.at>Date.now())return;
      const place=livingPlaces.find(p=>p.district===(e.to==='agent'?e.from:e.to));
      if(place&&places.has(place.id))onDemonstrate?.(place.id,2,e.state);
    },
    stats:()=>({completed,lobbyVisits,byPlace:{...byPlace},byResident:{...byResident},rigs:actors.filter(a=>a.rig).length,meetings:meetings.length,reservations:slots.filter(s=>s.owner).length,signals:signals.stats(),residents:actors.filter(a=>a.body.enabled).map(a=>({...describe(a),x:a.body.x,z:a.body.z,heading:a.body.heading})),places:[...places.keys()]}),
    dispose(){disposed=true;suspend();actors.forEach(a=>traffic.remove(a.id));actors.length=0;places.clear();slots.length=0;signals.dispose();},
  };
}
