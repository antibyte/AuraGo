import {createWayfinder,livingPlaces} from './sysworld-places.js';
import {bodyShape} from './sysworld-traffic.js';
import {createSocialSignals} from './sysworld-signals.js';

const turn=(from,to,dt)=>from+Math.atan2(Math.sin(to-from),Math.cos(to-from))*Math.min(1,dt*5);
export function createSociety(scene,{traffic,camera,floor,onSelect,onDemonstrate,active}) {
  const actors=[],places=new Map(),slots=[],meetings=[],finder=createWayfinder(traffic,floor),signals=createSocialSignals(scene);
  let clock=0,selected=null,replay=false,tier='high',disposed=false,completed=0,selectedStatus='';
  const limit=()=>tier==='low'?3:tier==='medium'?10:19;
  function release(a) {if(a.slot){a.slot.owner=null;a.slot=null;}a.path=[];a.goal=null;}
  function describe(a) {return a?{id:a.id,role:a.role,state:a.state,source:a.source||'ambient',guided:!!a.guide}:null;}
  function select(a) {selected=a?.id||null;selectedStatus=a?.state||'';onSelect?.(describe(a));}
  function add(model,index,bounds) {
    const id='resident-'+index,shape=bodyShape(bounds,1.15),role=['courier','technician','archivist'][index%3];let body=null;
    // A round envelope covers all articulated poses and permits turning in place.
    shape.circles=[{x:0,z:0,r:shape.reach}];
    const near=slots.filter(s=>s.role===role).map(s=>({x:s.x,z:s.z+2.2,y:floor(s.x,s.z+2.2),heading:Math.PI}));
    const candidates=[...near,...finder.points.map((_,n)=>finder.points[(index*7+n)%finder.points.length])];
    for(const p of candidates) {
      body=traffic.register(id,shape,p,1);if(body)break;
    }
    if(!body){model.node.visible=false;return null;}
    const a={...model,id,index,body,role,state:'idle',path:[],until:index*.3,trip:index,cooldown:8+index*.7,source:'ambient',carrying:false,parcels:[]};
    model.node.traverse(n=>{if(n.isMesh&&/parcel/.test(n.name)){a.parcels.push(n);n.visible=false;}});
    actors.push(a);model.node.position.set(body.x,body.y,body.z);return a;
  }
  function patrol(id,body,node,pause) {const a={id,body,node,pause,role:'patrol',state:'patrol',cooldown:7+actors.length,path:[]};actors.push(a);return a;}
  function choose(a) {
    release(a);a.source='ambient';a.state='idle';a.until=clock+2;
    const eligible=slots.filter(s=>!s.owner&&(a.role==='courier'?s.place===(a.carrying?'meeting-charge':'parcel-sorter'):s.role===a.role||s.place==='meeting-charge'));
    for(let n=0;n<eligible.length;n++) {
      const slot=eligible[(a.trip+n)%eligible.length],path=finder.route(a.body,slot);
      if(!path.length)continue;
      slot.owner=a.id;a.slot=slot;a.path=path;a.goal=slot;a.trip++;a.state=a.carrying?'carry':'walk';return;
    }
    // All workstations occupied: take a short, reachable walk instead of stacking.
    for(let n=0;n<8;n++) {
      const p=finder.points[(a.trip++*13+a.index)%finder.points.length],path=finder.route(a.body,p);
      if(path.length){a.path=path;a.state='walk';return;}
    }
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
      a.node.visible=!!enabled;if(!enabled||!animated)continue;
      const tram=traffic.neighbors(a.body,22).sort((b,c)=>Math.hypot(b.x-a.body.x,b.z-a.body.z)-Math.hypot(c.x-a.body.x,c.z-a.body.z)).find(b=>{
        if(!b.id.startsWith('tram-'))return false;const dx=a.body.x-b.x,dz=a.body.z-b.z,c=Math.cos(b.heading),s=Math.sin(b.heading);
        return Math.abs(dx*c-dz*s)<a.body.reach+(a.trafficYield?4:2.3)&&dx*s+dz*c>-8-a.body.reach&&dx*s+dz*c<14;
      });
      if(!tram&&a.yieldTram){
        const train=traffic.body(a.yieldTram),dx=train?a.body.x-train.x:0,dz=train?a.body.z-train.z:0;
        // Stay at the crossing until the entire train has passed. Immediately
        // rejoining a patrol would otherwise keep stepping back in front of it.
        if(train?.enabled&&Math.hypot(dx,dz)<26&&dx*Math.sin(train.heading)+dz*Math.cos(train.heading)>-train.reach-a.body.reach-1){a.pause?.(true);a.play?.('idle');continue;}
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
        for(const [dx,dz,distance] of exits){
          const probe={x:a.body.x+dx*distance,y:a.body.y,z:a.body.z+dz*distance,heading:a.body.heading};
          if(!traffic.clear(a.body,a.body,probe))continue;
          const next={...probe,x:a.body.x+dx*dt*1.8,z:a.body.z+dz*dt*1.8};next.y=a.pause?a.body.y:floor(next.x,next.z);
          traffic.propose(a.body,next,(_,b)=>{a.node.position.set(b.x,b.y,b.z);a.play?.('walk');});a.state='yield';a.cooldown=clock+5;break;
        }
        continue;
      }
      if(a.trafficYield){a.trafficYield=false;a.pause?.(false,a.body);a.state=a.pause?'patrol':a.path.length?'walk':'idle';}
      if(a.meeting)continue;
      if(a.pause){if(a.state==='greet'&&clock>a.until){a.pause(false);a.state='patrol';}continue;}
      if(a.guide&&Math.hypot(camera.position.x-a.body.x,camera.position.z-a.body.z)>9){a.play('idle');continue;}
      if(a.path.length) {
        const goal=a.path[0],dx=goal.x-a.body.x,dz=goal.z-a.body.z,d=Math.hypot(dx,dz),speed=a.guide?1.9:1.65;
        if(d<.18){a.path.shift();continue;}
        const advance=Math.min(d,speed*dt),forward={x:dx/d,z:dz/d};
        let x=a.body.x+forward.x*advance,z=a.body.z+forward.z*advance;
        let heading=turn(a.body.heading,Math.atan2(dx,dz),dt);
        // Predict nearby traffic before entering a gap; prefer the right side.
        const probe={x:a.body.x+forward.x*1.8,y:floor(x,z),z:a.body.z+forward.z*1.8,heading};
        if(!traffic.clear(a.body,a.body,probe)) {
          let clear=false;
          for(const [sx,sz] of [[forward.z,-forward.x],[-forward.z,forward.x],[-forward.x,-forward.z]]) {
            const candidate={x:a.body.x+sx*1.3,y:a.body.y,z:a.body.z+sz*1.3,heading:a.body.heading};
            if(traffic.clear(a.body,a.body,candidate)){x=a.body.x+sx*advance;z=a.body.z+sz*advance;heading=turn(a.body.heading,Math.atan2(sx,sz),dt);clear=true;break;}
          }
          if(!clear){x=a.body.x;z=a.body.z;}
        }
        if(!traffic.clear(a.body,a.body,{x,y:floor(x,z),z,heading},false))heading=a.body.heading;
        const moving=Math.hypot(x-a.body.x,z-a.body.z)>.001;
        traffic.propose(a.body,{x,y:floor(x,z),z,heading},(accepted,b)=>{
          a.node.position.set(b.x,b.y,b.z);a.node.rotation.y=b.heading;
          if(!accepted||!moving){a.wait=(a.wait||0)+dt;a.play('idle');a.state='yield';}
          else{a.wait=0;a.state=a.guide?'guide':a.carrying?'carry':'walk';a.play(a.carrying?'carry':'walk');}
          if(a.wait>2.5){a.wait=0;release(a);a.state=a.guide?'guide':'idle';a.until=clock+.5;if(a.guide)a.path=finder.route(a.body,a.guide);}
        });
      } else if(a.state==='walk'||a.state==='carry'||a.state==='guide'||a.state==='yield') {
        if(a.guide&&Math.hypot(a.body.x-a.guide.x,a.body.z-a.guide.z)>.5){a.play('idle');if(clock>a.until){a.path=finder.route(a.body,a.guide);a.until=clock+2.5;}continue;}
        a.guide=null;a.state=a.slot?.place==='meeting-charge'?'charge':a.slot?'work':'idle';a.until=clock+5+(a.index%4);
        a.play(a.state==='work'?'work':'idle');if(a.slot)onDemonstrate?.(a.slot.place,5);completed++;
      } else if(clock>a.until){
        if(a.role==='courier'&&a.slot&&(a.state==='work'||a.state==='charge')){a.carrying=a.slot.place==='parcel-sorter';a.parcels.forEach(n=>{n.visible=a.carrying;});}
        choose(a);
      }
    }
    if(!animated){signals.update(0,false);return;}
    // Reserve both partners for one finite exchange. No overlapping conversations.
    for(const a of actors) {
      if(!a.body.enabled||a.trafficYield||a.meeting||a.guide||a.state==='greet'||clock<a.cooldown)continue;
      const b=actors.find(b=>b!==a&&b.body.enabled&&!b.trafficYield&&b.state!=='greet'&&!b.meeting&&!b.guide&&clock>=b.cooldown&&Math.abs(a.body.y-b.body.y)<2&&Math.hypot(a.body.x-b.body.x,a.body.z-b.body.z)>2.8&&Math.hypot(a.body.x-b.body.x,a.body.z-b.body.z)<6);
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
  return {
    add,patrol,update,suspend,
    addPlace(place,entry) {
      if(places.has(place.id))return;places.set(place.id,place);
      for(const point of entry.navigation.workpoints||[])slots.push({id:place.id+':'+point.id,place:place.id,role:place.role,x:place.x+point.position[0],y:floor(place.x,place.z),z:place.z+point.position[2],owner:null});
    },
    nearby() {
      const rows=[];for(const a of actors)if(a.body.enabled){const distance=Math.hypot(camera.position.x-a.body.x,camera.position.z-a.body.z);if(distance<5&&Math.abs(camera.position.y-a.body.y)<8)rows.push({kind:'resident',id:a.id,distance,x:a.body.x,z:a.body.z});}
      for(const p of places.values()){const distance=Math.hypot(camera.position.x-p.x,camera.position.z-p.z);if(distance<7)rows.push({kind:'demonstrate',id:p.id,distance,x:p.x,z:p.z});}return rows;
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
    stats:()=>({completed,meetings:meetings.length,reservations:slots.filter(s=>s.owner).length,signals:signals.stats(),residents:actors.filter(a=>a.body.enabled).map(a=>({...describe(a),x:a.body.x,z:a.body.z})),places:[...places.keys()]}),
    dispose(){disposed=true;suspend();actors.forEach(a=>traffic.remove(a.id));actors.length=0;places.clear();slots.length=0;signals.dispose();},
  };
}
