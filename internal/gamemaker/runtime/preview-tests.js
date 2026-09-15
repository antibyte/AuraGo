// Server-injected finite test driver. Never accepts or evaluates JavaScript.
(function () {
  let running = false;
  const channel = new URLSearchParams(location.hash.slice(1)).get('gm-channel');
  const keys = { LEFT:37,RIGHT:39,UP:38,DOWN:40,W:87,A:65,S:83,D:68,SPACE:32,R:82,P:80,ESC:27,ENTER:13,F:70,Q:81,E:69 };
  const names = { LEFT:'ArrowLeft',RIGHT:'ArrowRight',UP:'ArrowUp',DOWN:'ArrowDown',SPACE:' ',ESC:'Escape',ENTER:'Enter' };
  const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
  function keyEvent(name, down) {
    if (!keys[name]) throw Error('Unsupported test key');
    const event = new KeyboardEvent(down ? 'keydown' : 'keyup', {key:names[name] || name.toLowerCase(),code: name==='SPACE'?'Space':names[name] || 'Key'+name,keyCode:keys[name],which:keys[name],bubbles:true});
    window.dispatchEvent(event);
  }
  async function press(name, ms) {keyEvent(name,true);try{await wait(ms);}finally{keyEvent(name,false);}}
  function binding() {
    const b = window.__AURAGO_GAME_TEST__;
    if (b?.kind === 'three' && b.alive?.() && b.canvas instanceof HTMLCanvasElement && typeof b.snapshot === 'function') return b;
    if (!b?.scene?.sys?.isActive() || !b.player?.active) throw Error('Missing live bindGameTest scene/player: assign this.player to the controlled object in GameScene.setup(). Preserve common.ts create/update; custom scenes must call bindGameTest(this,state,player) in create() on every restart.');
    return b;
  }
  async function reset() {await press('R',80);await wait(420);}
  function snapshot(inspectAssets) {
    const b=binding();if(b.kind==='three')return b.snapshot();
    const {scene,state,player}=b;
    const result={player_x:player.x,player_y:player.y,
      // Phaser 4.2.1 has no public timer enumeration; this adapter is pinned and
      // exercised by the real-runtime browser fixture when Phaser is upgraded.
      object_count:scene.children.list.length,timer_count:new Set([...scene.time._active,...scene.time._pendingInsertion].filter(timer=>!scene.time._pendingRemoval.includes(timer))).size,
      listener_count:scene.input.keyboard.eventNames().reduce((n,event)=>n+scene.input.keyboard.listenerCount(event),0),
      elapsed_ms:scene.elapsed,...inspectAssets(scene)};
    for(const name of ['actions','score','hits','spawns','turns','ticks','ended','lives','health','goal_remaining','outcome','hit_events','pickup_events','win_events','lose_events'])if(Number.isFinite(state[name]))result[name]=state[name];
    // Older source templates counted pickups as hits; observe their real pickup
    // events without changing game state or treating combat hits as collection.
    const pickups=scene.auditGame?.()?.events?.pickup;
    if(Number.isFinite(pickups))result.pickup_events=pickups;
    return result;
  }
  function evidence() {
    const b=binding(), source=b.kind==='three'?b.audit?.():b.scene.auditGame?.() || b.scene.builder?.audit?.();
    if(!source)return undefined;
    const out={outcome:source.outcome,trace:(source.event_trace||[]).slice(-32).map(e=>({type:e.type,id:e.id,at:e.at})),roles:(source.roles||[]).slice(0,64).map(r=>({role:r.role,asset_id:r.asset_id,count:r.count})),
      presentation:source.presentation?Object.fromEntries(['events','effects','sounds'].map(key=>[key,Object.fromEntries(Object.entries(source.presentation[key]||{}).slice(0,64))])):undefined,
      events:Object.fromEntries(Object.entries(source.events||{}).slice(0,24)),faults:(source.faults||[]).slice(0,16),
      nodes:(source.nodes||[]).slice(0,128).map(n=>({id:n.id,active:n.active,health:n.health,x:n.x,y:n.y,z:n.z||0}))};
    if(JSON.stringify(out).length>50000)throw Error('Scene evidence exceeds its bounded report size');
    return out;
  }
  async function step(command) {
    if(!command||!Number.isFinite(command.ms||0)||command.ms<0||command.ms>4000)throw Error('Invalid test duration');
    const ms=command.ms||50;
    switch(command.action){
      case 'key': await press(command.key,ms);break;
      case 'wait': await wait(ms);break;
      case 'observe': await wait(34);break;
      case 'pointer': {
        const b=binding(),scene=b.scene,canvas=b.kind==='three'?b.canvas:scene.game.canvas,rect=canvas.getBoundingClientRect();
        if(!Number.isFinite(command.x)||!Number.isFinite(command.y)||command.x<0||command.y<0||command.x>1920||command.y>1080)throw Error('Invalid pointer');
        const options={bubbles:true,clientX:rect.left+command.x/(scene?.scale.width||960)*rect.width,clientY:rect.top+command.y/(scene?.scale.height||540)*rect.height,button:0,buttons:1};
        const EventType=b.kind==='three'?PointerEvent:MouseEvent,prefix=b.kind==='three'?'pointer':'mouse';
        canvas.dispatchEvent(new EventType(prefix+'down',options));await wait(ms);canvas.dispatchEvent(new EventType(prefix+'up',{...options,buttons:0}));break;
      }
      default: throw Error('Unsupported test command');
    }
  }
  let capturing=false;
  async function capture(scenario='start') {
    if(capturing)return null;capturing=true;
    try {
      const b=window.__AURAGO_GAME_TEST__,canvas=b?.canvas||b?.scene?.game?.canvas||[...document.querySelectorAll('canvas')].find(c=>c.width>1&&c.height>1&&c.getBoundingClientRect().width>0);
      if(!canvas||canvas.width*canvas.height>16777216)throw Error('No bounded visible game canvas');
      const out=document.createElement('canvas');
      let source=canvas;
      if(b?.scene?.game?.renderer?.snapshot) {
        source=await new Promise((resolve,reject)=>{
          const timer=setTimeout(()=>reject(Error('Snapshot timeout')),1000);
          b.scene.game.renderer.snapshot(image=>{clearTimeout(timer);resolve(image)},'image/png');
        });
      } else {
        // Read after the game's already scheduled render, without preserving its drawing buffer.
        await new Promise((resolve,reject)=>{
          let first,second,timer;
          const cleanup=()=>{clearTimeout(timer);cancelAnimationFrame(first);cancelAnimationFrame(second);window.removeEventListener('pagehide',hidden)};
          const hidden=()=>{cleanup();reject(Error('Capture closed'))};
          window.addEventListener('pagehide',hidden,{once:true});
          timer=setTimeout(()=>{cleanup();reject(Error('Render timeout'))},1000);
          first=requestAnimationFrame(()=>{second=requestAnimationFrame(()=>{try{
            out.width=canvas.width;out.height=canvas.height;out.getContext('2d').drawImage(canvas,0,0);source=out;cleanup();resolve();
          }catch(e){cleanup();reject(e)}})});
        });

      }
      const scaled=document.createElement('canvas'),w=source.naturalWidth||source.width,h=source.naturalHeight||source.height;
      if(!w||!h)throw Error('Empty capture');
      let edge=1280,image='';
      do {const ratio=Math.min(1,edge/Math.max(w,h));scaled.width=Math.max(1,Math.round(w*ratio));scaled.height=Math.max(1,Math.round(h*ratio));
        const ctx=scaled.getContext('2d');ctx.drawImage(source,0,0,scaled.width,scaled.height);
        const pixels=ctx.getImageData(0,0,scaled.width,scaled.height).data;
        if(!pixels.some((v,i)=>i%4===3&&v))throw Error('Empty drawing buffer');
        image=scaled.toDataURL('image/png');edge=Math.floor(edge*.75);
      }while(image.length>700000&&edge>=160);
      if(image.length>700000)throw Error('Capture too large');
      const hud=[...document.querySelectorAll('[data-hud],#hud,.hud,[role="status"]')].slice(0,8).map(el=>{
        const r=el.getBoundingClientRect();return {text:(el.innerText||'').slice(0,200),x:Math.round(r.x),y:Math.round(r.y),width:Math.round(r.width),height:Math.round(r.height)};
      });
      return {image,controlled:!['start','current_view'].includes(scenario),scenario:String(scenario).slice(0,96),at:new Date().toISOString(),width:scaled.width,height:scaled.height,hud:JSON.stringify(hud).slice(0,1000)};
    } catch (_) {return null;}finally{capturing=false;}
  }
  window.addEventListener('message',async event=>{
    const d=event.data;
    if(event.source!==parent||!channel||d?.channel!==channel||d.source!=='aurago-studio'||d.type!=='capture'||running||capturing)return;
    if(typeof d.request_id!=='string'||d.request_id.length>96)return;
    const record=await capture('current_view');
    parent.postMessage({source:'aurago-game',channel,type:'capture',request_id:d.request_id,captures:record?[record]:[]},'*');
  });
  const objectIDs=new WeakMap();let nextID=0;
  const identity=o=>{if(!objectIDs.has(o))objectIDs.set(o,'observed-'+(++nextID));return objectIDs.get(o)};
  const children=g=>Array.isArray(g)?g:g?.getChildren?.()||[];
  const overlaps=(a,b,pad=0)=>Math.abs(a.x-b.x)<(a.w+b.w)/2+pad&&Math.abs(a.y-b.y)<(a.h+b.h)/2+pad;
  function targetView(){
    const b=binding();if(b.kind==='three')return b.observeTargets?.();
    const {scene,player}=b,records=[...(scene.builder?.nodes?.values?.()||[])],byObject=new Map(records.map(r=>[r.object,r]));
    const all=[...new Set([...(scene.gameObjects||[]),...scene.children.list,...records.map(r=>r.object)])];
    const colliders=scene.physics.world.colliders.getActive();
    const contains=(group,o)=>group===o||children(group).includes(o);
    const linked=(o,other)=>colliders.some(c=>c.active&&!c.overlapOnly&&(contains(c.object1,o)&&contains(c.object2,other)||contains(c.object2,o)&&contains(c.object1,other)));
    const shots=new Set([...children(scene.shots),...children(scene.bullets)]),board=children(scene.cells);
    const view=scene.cameras.main.worldView;
    const item=o=>{
      const r=byObject.get(o),body=o.body,rect=body?{x:body.x,y:body.y,width:body.width,height:body.height}:o.getBounds?.();
      if(!rect)return null;
      const roles=[o.__gmRole,...(o.__gmAssets||[]).map(a=>a.role),r?.node.role,...(r?.node.behaviors||[]).map(a=>a.type)].filter(Boolean);
      if(children(scene.enemies).includes(o))roles.push('enemy');if(o===scene.ball)roles.push('ball');if(board.includes(o))roles.push('cell');if(shots.has(o))roles.push('projectile');
      const active=o.active===true&&(!body||body.enable!==false),x=rect.x+rect.width/2,y=rect.y+rect.height/2;
      return {id:r?.node.id||o.__auragoBrickID||o.__gmID||identity(o),roles,asset_ids:(o.__gmAssets||[]).map(a=>a.asset_id).filter(Boolean),x,y,z:0,w:rect.width,h:rect.height,active,
        visible:active&&x+rect.width/2>=view.x&&x-rect.width/2<=view.right&&y+rect.height/2>=view.y&&y-rect.height/2<=view.bottom,
        solid:active&&body?.immovable&&linked(o,player),health:r?.health??o.__hp,mark:board.includes(o)?scene.marks?.[board.indexOf(o)]:Boolean(o.opened),
        vx:body?.velocity?.x||0,vy:body?.velocity?.y||0,grounded:body?.blocked?.down||body?.touching?.down,
        sx:(x-view.x)*scene.cameras.main.zoom,sy:(y-view.y)*scene.cameras.main.zoom};
    };
    const targets=all.filter(o=>o!==player&&(o.body||board.includes(o))).slice(0,256).map(item).filter(Boolean);
    return {kind:'2d',active:!scene.manualPause&&scene.previewActive!==false,mode:scene.physics.world.gravity.y?'platformer':'topdown',player:item(player),targets,projectiles:targets.filter(o=>o.roles.includes('projectile')),bounds:scene.physics.world.bounds,truncated:all.length>512};
  }
  // A bounded grid supplies waypoints around registered colliders. It never
  // moves objects, and unsupported platform physics remains unverified.
  function waypoint(view,target){
    const p=view.player,solids=view.targets.filter(o=>o.solid&&o.id!==target.id),bounds=view.bounds;
    // Walking aims at the target's horizontal position, not its lower centre.
    // Ground contact is support, not a wall; retain actual penetrating obstacles.
    if(view.mode==='platformer')target={...target,y:p.y};
    const blocked=(x,y)=>solids.some(o=>overlaps({x,y,w:p.w,h:p.h},o,view.mode==='platformer'?-.05:.05));
    const lineClear=(a,b)=>{const n=Math.min(128,Math.ceil(Math.hypot(b.x-a.x,b.y-a.y)/Math.max(p.w,p.h,.3)));for(let i=1;i<=n;i++)if(blocked(a.x+(b.x-a.x)*i/n,a.y+(b.y-a.y)*i/n))return false;return true};
    if(lineClear(p,target))return target;
    const cell=Math.max(view.kind==='3d'?.75:16,p.w,p.h,Math.max(bounds.width||bounds.w,bounds.height||bounds.h)/48);
    const width=Math.ceil((bounds.width||bounds.w)/cell),height=Math.ceil((bounds.height||bounds.h)/cell);
    if(width*height>4096)return null;
    const at=o=>[Math.max(0,Math.min(width-1,Math.floor((o.x-bounds.x)/cell))),Math.max(0,Math.min(height-1,Math.floor((o.y-bounds.y)/cell)))];
    const [sx,sy]=at(p),[tx,ty]=at(target),start=sy*width+sx,goal=ty*width+tx,queue=[start],previous=new Map([[start,-1]]);
    for(let i=0;i<queue.length&&queue.length<=4096;i++){
      const here=queue[i];if(here===goal)break;
      for(const [dx,dy] of [[1,0],[-1,0],[0,1],[0,-1]]){
        const x=here%width+dx,y=Math.floor(here/width)+dy,next=y*width+x;
        if(x<0||x>=width||y<0||y>=height||previous.has(next)||blocked(bounds.x+(x+.5)*cell,bounds.y+(y+.5)*cell))continue;
        previous.set(next,here);queue.push(next);
      }
    }
    if(!previous.has(goal))return null;
    let next=goal;while(previous.get(next)!==start&&previous.get(next)!==-1)next=previous.get(next);
    return {x:bounds.x+(next%width+.5)*cell,y:bounds.y+(Math.floor(next/width)+.5)*cell};
  }
  function movePointer(point,down=false){
    const b=binding(),canvas=b.kind==='three'?b.canvas:b.scene.game.canvas,rect=canvas.getBoundingClientRect();
    const x=rect.left+point.sx/(b.scene?.scale.width||960)*rect.width,y=rect.top+point.sy/(b.scene?.scale.height||540)*rect.height;
    canvas.dispatchEvent(new MouseEvent('mousemove',{bubbles:true,clientX:x,clientY:y,buttons:0}));
    if(down){canvas.dispatchEvent(new MouseEvent('mousedown',{bubbles:true,clientX:x,clientY:y,button:0,buttons:1}));canvas.dispatchEvent(new MouseEvent('mouseup',{bubbles:true,clientX:x,clientY:y,button:0,buttons:0}));}
  }
  async function pursue(command,scenario,baseline,inspectAssets,expired){
    const result={target:command.target,mode:command.mode,samples:0,inputs:0,contacts:0,effects:0,reason:'no_target'},held=new Set();
    const modes=['move','aim','reach','interact','catch','avoid','select'];
    if(!modes.includes(command.mode)||typeof command.target!=='string'||!command.target||command.target.length>96||command.ms<100||command.ms>4000)throw Error('Invalid target command');
    const apply=want=>{for(const key of held)if(!want.has(key)){keyEvent(key,false);held.delete(key)}for(const key of want)if(!held.has(key)){keyEvent(key,true);held.add(key);result.inputs++}};
    const changed=()=>{const now=snapshot(inspectAssets)[scenario.metric],before=baseline[scenario.metric];return Number.isFinite(now)&&Number.isFinite(before)&&(scenario.compare==='increased'?now>before:scenario.compare==='decreased'?now<before:scenario.compare==='changed'?now!==before:scenario.compare==='equals'?now===scenario.value:now>=scenario.value)};
    let prior=new Map(),priorActions=baseline.actions||0,lastFire=-1000,lock='',direction=null,saw=false,observedMiss=false;
    const begin=performance.now();
    try{
      while(performance.now()-begin<command.ms&&!expired()&&result.samples<160){
        if(document.hidden){result.reason='inactive';break;}
        const view=targetView();result.samples++;
        if(!view?.player||view.truncated){result.reason='unsupported';break;}
        if(view.active===false){result.reason='inactive';break;}
        if(command.mode==='move'){
          if(command.target!=='player'){result.reason='unsupported';break;}
          const p=view.player,moved=Math.hypot(snapshot(inspectAssets).player_x-baseline.player_x,snapshot(inspectAssets).player_y-baseline.player_y);
          if(result.inputs&&moved>.05){result.effects=1;result.reason='complete';break;}
          const size=view.kind==='3d'?1:Math.max(p.w,p.h),options=[[1,0],[-1,0],[0,1],[0,-1]];
          const index=Math.floor((performance.now()-begin)/250)%4,vector=options[index],x=p.x+vector[0]*size,y=p.y+vector[1]*size,bounds=view.bounds;
          const free=x>bounds.x&&x<bounds.x+(bounds.width||bounds.w)&&y>bounds.y&&y<bounds.y+(bounds.height||bounds.h)&&!view.targets.some(o=>o.solid&&overlaps({...p,x,y},o));
          if(!free||view.mode==='platformer'&&vector[1]){apply(new Set());result.reason='blocked';await wait(50);continue;}
          let key=vector[0]?(vector[0]>0?'D':'A'):(vector[1]>0?(view.kind==='3d'?'W':'S'):(view.kind==='3d'?'S':'W'));
          if(view.mode==='fps'&&vector[0])key=vector[0]>0?'A':'D';
          result.contacts++;result.reason='timeout';apply(new Set([key]));await wait(50);continue;
        }
        // Models sometimes name the exact catalog asset (coin) instead of its
        // role (item). Resolve only metadata on actual objects, never guessed art.
        const direct=o=>o.id===command.target||o.roles?.some(r=>r===command.target||r.startsWith(command.target+'_'));
        const hasDirect=view.targets.some(direct);
        const p=view.player,now=snapshot(inspectAssets),candidates=view.targets.filter(o=>o.active&&o.visible&&
          (hasDirect?direct(o):o.asset_ids?.includes(command.target))&&
          (command.mode!=='select'||!o.mark));
        if(command.mode==='avoid'&&view.kind==='2d'&&view.targets.some(o=>o.id===lock&&o.y-o.h/2>(view.bounds.bottom??view.bounds.y+view.bounds.height)))observedMiss=true;
        if(observedMiss&&Number.isFinite(now.lives)&&now.lives<baseline.lives)result.effects=1;
        const relevant=o=>o.id===lock||command.mode==='catch'&&(String(o.id).startsWith('brick-')||o.roles?.some(r=>r==='block'||r.startsWith('block_')));
        const effects=view.targets.filter(o=>{const old=prior.get(o.id);return old&&relevant(old)&&(old.active&&!o.active||Number.isFinite(old.health)&&Number.isFinite(o.health)&&o.health<old.health||old.mark!==undefined&&o.mark!==old.mark)}).length;
        // Destroyed scene nodes may disappear from the host collection entirely.
        const removed=[...prior.values()].filter(o=>o.active&&relevant(o)&&!view.targets.some(n=>n.id===o.id)).length;
        result.effects+=Math.min(1,effects+removed);
        if(result.inputs&&result.effects&&changed()){result.reason='complete';break;}
        prior=new Map(view.targets.map(o=>[o.id,{...o}]));
        if(now.ended){result.reason='timeout';break;}
        const target=candidates.find(o=>o.id===lock)||candidates.sort((a,b)=>Math.hypot(a.x-p.x,a.y-p.y)-Math.hypot(b.x-p.x,b.y-p.y))[0];
        if(!target){apply(new Set(view.mode==='fps'&&view.targets.some(o=>o.active&&(o.id===command.target||o.roles?.includes(command.target)))?['RIGHT']:[]));await wait(50);continue;}
        lock=target.id;saw=true;result.reason='timeout';
        const want=new Set(),elapsed=performance.now()-begin,dx=target.x-p.x,dy=target.y-p.y;
        const near=overlaps(p,target,view.kind==='3d'?1:3);
        const steer=point=>{
          if(!point){result.reason='blocked';return;}
          let x=point.x-p.x,y=point.y-p.y,tolerance=view.kind==='3d'?.15:4;
          if(view.mode==='fps'){const a=p.aim,c=Math.cos(a),s=Math.sin(a);[x,y]=[-(x*c-y*s),x*s+y*c];}
          if(Math.abs(x)>tolerance)want.add(x>0?'D':'A');
          if(view.mode!=='platformer'&&Math.abs(y)>tolerance)want.add(y>0?(view.kind==='3d'?'W':'S'):(view.kind==='3d'?'S':'W'));
          if(['flight','space'].includes(view.mode)&&Math.abs(target.z-p.z)>.25)want.add(target.z>p.z?'E':'Q');
        };
        let fire=false;
        if(command.mode==='select'){
          if(view.kind!=='2d'){result.reason='unsupported';break;}
          if(elapsed-lastFire>250){movePointer(target,true);result.inputs++;result.contacts++;lastFire=elapsed;}
        }else if(command.mode==='catch'||command.mode==='avoid'){
          if(view.kind!=='2d'){result.reason='unsupported';break;}
          const destination=command.mode==='catch'?target.x+(target.vx||0)*Math.max(0,(p.y-target.y)/Math.max(1,target.vy||1)):(target.x<p.x?view.bounds.right-p.w:view.bounds.x+p.w);
          steer({x:Math.max(view.bounds.x+p.w/2,Math.min(view.bounds.right-p.w/2,destination)),y:p.y});
          fire=Math.abs(target.vx||0)+Math.abs(target.vy||0)<1;
        }else if(command.mode==='aim'){
          if(view.kind==='3d'){
            if(view.mode==='fps'){
              const eye=view.eye,desired=Math.atan2(target.x-eye.x,target.y-eye.y),yaw=Math.atan2(Math.sin(desired-p.aim),Math.cos(desired-p.aim));
              const pitch=Math.atan2(target.z-eye.z,Math.hypot(target.x-eye.x,target.y-eye.y))-p.pitch;
              if(Math.abs(yaw)>.025)want.add(yaw>0?'LEFT':'RIGHT');if(Math.abs(pitch)>.025)want.add(pitch>0?'UP':'DOWN');
              fire=view.aimed===target.id;
              if(!fire&&Math.abs(yaw)<.08&&Math.abs(pitch)<.08)steer(waypoint(view,target));
            }else if(view.mode==='space'){
              steer({x:target.x,y:Math.min(p.y,target.y-4)});fire=view.aimed===target.id;
            }else {result.reason='unsupported';break;}
            if(fire&&(now.actions||0)>priorActions)result.contacts++;
          }else{
            movePointer(target); // Actual aim input, including moving targets.
            const projectile=view.projectiles.find(o=>o.active&&Math.hypot(o.vx,o.vy)>10);
            if(projectile&&!direction){const n=Math.hypot(projectile.vx,projectile.vy);direction={x:projectile.vx/n,y:projectile.vy/n};}
            if(direction){
              // First observe the real projectile direction. Pointer-aimed games
              // simply keep aiming; fixed-axis shooters align the player instead.
              const cross=dx*direction.y-dy*direction.x;
              if(Math.abs(cross)>(target.w+p.w)/3)steer(waypoint(view,{...target,x:target.x-direction.x*120,y:target.y-direction.y*120}));
            }
            const steps=Math.min(128,Math.ceil(Math.hypot(dx,dy)/8));
            const obstructed=view.targets.some(o=>o.solid&&o.id!==target.id&&Array.from({length:steps},(_,i)=>i/steps).some(t=>overlaps({x:p.x+dx*t,y:p.y+dy*t,w:2,h:2},o)));
            if(obstructed)steer(waypoint(view,target));
            fire=!obstructed;
            if(view.projectiles.some(o=>o.active&&overlaps(o,target)))result.contacts++;
          }
        }else{
          steer(waypoint(view,target));
          if(view.mode==='platformer'){
            // Only jump from a real grounded body; unsupported routes time out.
            if(p.grounded&&(target.y< p.y-p.h/2||view.targets.some(o=>o.solid&&Math.abs(o.x-p.x)<p.w+o.w/2&&o.y<p.y)))fire=true;
          }
          if(overlaps(p,target)){result.contacts++;}
          if(near&&command.mode==='interact')fire=true;
          const vertical=view.kind!=='3d'||Math.abs((p.z+.85)-target.z)<(.85+(target.depth||0)/2);
          if(overlaps(p,target)&&vertical&&result.inputs&&changed()&&['player_x','player_y','health','lives','outcome'].includes(scenario.metric)){result.effects++;result.reason='complete';break;}
        }
        if(fire&&elapsed-lastFire>=300){want.add('SPACE');lastFire=elapsed;}
        priorActions=now.actions||0;apply(want);await wait(50);
      }
      if(result.reason==='no_target'&&saw)result.reason='timeout';
      // A skipped vertical probe must not erase the free horizontal attempts.
      if(command.mode==='move'&&result.reason==='blocked'&&result.inputs&&result.contacts)result.reason='timeout';
    }finally{apply(new Set());}
    return result;
  }
  window.addEventListener('message',async event=>{
    const data=event.data;
    if(event.source!==parent||!channel||data?.channel!==channel||data.source!=='aurago-studio'||data.type!=='run-tests'||running)return;
    if(!Array.isArray(data.scenarios)||data.scenarios.length>16)return;
    running=true;
    const observations=[],images=[],captures=[];let expired=false,initial;
    const deadline=setTimeout(()=>{expired=true;for(const key of Object.keys(keys))keyEvent(key,false);},55000);
    try {
      // A visible canvas precedes asynchronous GLB loading. Wait for the real
      // game binding within the same bounded test deadline.
      await wait(3100);
      while (!window.__AURAGO_GAME_TEST__ && !expired) await wait(100);
      if (expired) throw Error('Game initialization exceeded the test deadline');
      const inspectAssets = binding().kind==='three' ? null : (await import('./vendor/aurago-game-1.js')).inspectAssets;
      const first=await capture('start');if(first)captures.push(first);
      for(const scenario of data.scenarios){
        if(expired)throw Error('Gameplay driver exceeded 55 seconds');
        if(!Array.isArray(scenario.steps)||scenario.steps.length>8)throw Error('Too many test steps');
        await reset();const current=snapshot(inspectAssets);if(!initial)initial=current;
        const before=scenario.id==='required_restart'?initial:current, evidenceBefore=evidence(),targetRuns=[];let restart1;
        for(const command of scenario.steps){if(command.action==='target')targetRuns.push(await pursue(command,scenario,before,inspectAssets,()=>expired));else await step(command);if(scenario.id==='required_restart'&&command.action==='wait'&&!restart1)restart1=snapshot(inspectAssets);if(expired)throw Error('Gameplay driver timed out');}
        const after=snapshot(inspectAssets);
        if(restart1)for(const [key,value] of Object.entries(restart1))after['restart1_'+key]=value;
        observations.push({id:scenario.id,before,after,evidence_before:evidenceBefore,evidence_after:evidence(),target_runs:targetRuns});
		if(scenario.id==='required_rules'){const actionImage=await capture(scenario.id);if(actionImage&&captures.length<2)captures.push(actionImage);}
      }
      if(captures.length<2){const last=await capture('after_scenarios');if(last)captures.push(last);}
    } catch(error) {
      // Missing evidence remains unavailable in the server comparator.
      parent.postMessage({source:'aurago-game',channel,type:'diagnostic',message:String(error).slice(0,1000)},'*');
    } finally {
      clearTimeout(deadline);
      for(const key of Object.keys(keys))keyEvent(key,false);
      try{await reset();}catch(_){}
      parent.postMessage({source:'aurago-game',channel,type:'gameplay',observations,images,captures},'*');
    }
  });
})();
