// Hover-pack flight: pure and deterministic, no scene and no clock of its own. The caller owns
// collision (`clear`), the street level (`ground`), where a visitor may stand (`walkable`) and the
// motion preference. Positions are eye positions in metres, Y up; `yaw` is the camera heading
// (forward = -sin, -cos), so a forward input flies where the visitor looks.
export const hoverLimits={cruise:12,boost:26,climb:7,sink:7,ease:5,eye:2.4,takeoff:1.5,takeoffTime:.7,ceiling:150,minX:-140,maxX:140,minZ:-150,maxZ:130,glide:8,descend:6,reach:40,bob:.03};
const hover=hoverLimits,clamp=(n,a,b)=>Math.max(a,Math.min(b,n)),smooth=t=>t<=0?0:t>=1?1:t*t*(3-2*t);
export function createHoverFlight({clear,ground,walkable,reduced=()=>false}){
  let phase='idle',done=false,x=0,y=0,z=0,vx=0,vy=0,vz=0,t=0,base=0,clock=0,plan=null,bobY=0;
  const floorAt=(px,pz)=>ground(px,pz)+hover.eye,pose=()=>({x,y:y+bobY,z}),stop=()=>{vx=vy=vz=0;};
  const ease=(v,to,dt)=>{v+=(to-v)*(1-Math.exp(-hover.ease*dt));return Math.abs(v)<.01&&!to?0:v;};
  // Nearest standable street-level spot: 1.5 m rings up to `reach`, coarser rings out to the flight
  // limits (a visitor over the sea). A gliding landing also needs a free column above the spot.
  function findSpot(column,avoid=[]){
    for(let r=0;r<=Math.max(hover.maxX-hover.minX,hover.maxZ-hover.minZ);r+=r<hover.reach?1.5:4){
      const n=r?Math.ceil(r*Math.PI*2/(r<hover.reach?1.5:4)):1;
      for(let i=0;i<n;i++){
        const a=i/n*Math.PI*2,sx=x+Math.cos(a)*r,sz=z+Math.sin(a)*r;
        if(sx<hover.minX||sx>hover.maxX||sz<hover.minZ||sz>hover.maxZ||avoid.some(s=>Math.hypot(s.x-sx,s.z-sz)<1)||!walkable(sx,sz))continue;
        const sy=floorAt(sx,sz);
        if(column&&!clear({x:sx,y:Math.max(y,sy),z:sz},{x:sx,y:sy,z:sz}))continue;
        return {x:sx,y:sy,z:sz,blocked:0,time:0,skipped:avoid};
      }
    }
    return null;
  }
  const arrive=spot=>{x=spot.x;y=spot.y;z=spot.z;stop();bobY=0;phase='idle';done=true;plan=null;};
  function slide(dt){
    const nx=clamp(x+vx*dt,hover.minX,hover.maxX),nz=clamp(z+vz*dt,hover.minZ,hover.maxZ);
    if(nx!==x+vx*dt)vx=0;if(nz!==z+vz*dt)vz=0;
    if(nx!==x){if(clear({x,y,z},{x:nx,y,z}))x=nx;else vx=0;}
    if(nz!==z){if(clear({x,y,z},{x,y,z:nz}))z=nz;else vz=0;}
    const floor=floorAt(x,z),ny=clamp(y+vy*dt,floor,hover.ceiling);
    if(ny!==y+vy*dt)vy=0;
    if(ny!==y){if(clear({x,y,z},{x,y:ny,z}))y=ny;else vy=0;}
    if(y<floor){y=floor;vy=Math.max(0,vy);}
  }
  function glide(dt){
    const p=plan,dx=p.x-x,dz=p.z-z,dist=Math.hypot(dx,dz),drop=y-p.y;p.time+=dt;
    // A landing that cannot finish in a minute settles on its spot instead of hovering forever.
    if(dist<.05&&drop<.02||p.time>60){arrive(p);return;}
    const sh=dist>.05?Math.min(hover.glide,Math.max(.8,dist*1.4)):0,sv=drop>.02?Math.min(hover.descend,Math.max(.6,drop*1.8)):0;
    const mx=dist>.05?dx/dist*Math.min(sh*dt,dist):0,mz=dist>.05?dz/dist*Math.min(sh*dt,dist):0,my=-Math.min(sv*dt,Math.max(0,drop)),here={x,y,z};
    if(clear(here,{x:x+mx,y:y+my,z:z+mz})){x+=mx;y+=my;z+=mz;vx=mx/dt;vz=mz/dt;vy=my/dt;p.blocked=0;}
    else if((mx||mz)&&clear(here,{x:x+mx,y,z:z+mz})){x+=mx;z+=mz;vx=mx/dt;vz=mz/dt;vy=0;p.blocked=0;}
    else{
      // Walled in: rise above the obstacle, or choose another spot when the column itself is blocked.
      stop();
      if(dist>=.05){if(y<hover.ceiling&&clear(here,{x,y:y+hover.descend*dt,z})){y+=hover.descend*dt;vy=hover.descend;}}
      else if((p.blocked+=dt)>.5){const next=findSpot(true,[...p.skipped,p]);if(next)plan=next;else{phase='flying';plan=null;}}
    }
  }
  return {
    // Takeoff lifts the visitor ~1.5 m on its own before the controls take over.
    start(position){x=position.x;y=Math.max(position.y,floorAt(position.x,position.z));z=position.z;stop();phase='takeoff';done=false;t=0;base=y;clock=0;plan=null;bobY=0;return pose();},
    // Adopt a position set from outside (a rejected move), keeping the phase.
    place(position){x=position.x;y=position.y;z=position.z;stop();bobY=0;},
    step(dt,input={},yaw=0){
      dt=clamp(dt,0,.1);
      if(phase==='takeoff'){
        t=reduced()?hover.takeoffTime:Math.min(hover.takeoffTime,t+dt);
        const ny=base+hover.takeoff*smooth(t/hover.takeoffTime);
        if(ny>y&&clear({x,y,z},{x,y:ny,z}))y=ny;
        if(t>=hover.takeoffTime)phase='flying';
      }else if(phase==='landing'){if(dt>0)glide(dt);}
      else if(phase==='flying'){
        let f=clamp(+input.forward||0,-1,1),r=clamp(+input.right||0,-1,1);const m=Math.hypot(f,r);if(m>1){f/=m;r/=m;}
        const speed=input.boost?hover.boost:hover.cruise,sn=Math.sin(yaw),cs=Math.cos(yaw),up=clamp(+input.up||0,-1,1);
        vx=ease(vx,(-sn*f+cs*r)*speed,dt);vz=ease(vz,(-cs*f-sn*r)*speed,dt);vy=ease(vy,up*(up>0?hover.climb:hover.sink),dt);
        slide(dt);
        // A subtle bob that never leaves the space between the street-level floor and the ceiling.
        clock+=dt;bobY=reduced()?0:Math.min(hover.bob,Math.max(0,y-floorAt(x,z)),hover.ceiling-y)*Math.sin(clock*2.1);
      }
      return pose();
    },
    // Begin the landing; reports whether a spot was found. `instant` (and reduced motion) lands at once.
    land(instant=false){
      const now=instant||reduced();if(phase==='idle')return false;if(phase==='landing'&&!now)return true;
      const spot=findSpot(!now);if(!spot)return false;
      if(now){arrive(spot);return true;}
      plan=spot;phase='landing';bobY=0;return true;
    },
    // Leave the flight where it is (used when the ride ends for another reason).
    reset(){phase='idle';done=false;plan=null;stop();bobY=0;},
    state(){return {phase,flying:phase!=='idle',landing:phase==='landing',done,x,y:y+bobY,z,altitude:Math.max(0,y-floorAt(x,z)),speed:Math.hypot(vx,vz),climb:vy};},
  };
}
