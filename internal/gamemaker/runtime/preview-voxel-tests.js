// Server-only driver. Observes the shared grid; all play uses public DOM inputs.
(function(){
  const channel=new URLSearchParams(location.hash.slice(1)).get('gm-channel');
  let running=false,deadline=0;
  const wait=ms=>new Promise(resolve=>setTimeout(resolve,ms));
  const binding=()=>window.__AURAGO_GAME_TEST__;
  const observe=cells=>binding()?.observeVoxel(cells);
  const key=(name,down)=>window.dispatchEvent(new KeyboardEvent(down?'keydown':'keyup',{key:name,bubbles:true}));
  async function press(name,ms=50){key(name,true);try{await wait(Math.min(ms,Math.max(0,deadline-performance.now())));}finally{key(name,false);}}
  function evidence(o){return {player:o.player,inventory:o.inventory,blocks:o.blocks,enemies:o.enemies.map(({id,position,health})=>({id,position,health})),paused:o.paused};}
  function count(o,id){return o.inventory.reduce((n,s)=>n+(s?.item===id?s.count:0),0);}
  function remaining(end){return performance.now()<Math.min(end,deadline)&&!document.hidden;}
  async function aim(point,end){
    while(remaining(end)){
      const p=observe().player,d=[point[0]-p.position[0],point[1]-p.position[1]-1.62,point[2]-p.position[2]];
      const yaw=Math.atan2(-d[0],-d[2]),pitch=Math.atan2(d[1],Math.hypot(d[0],d[2]));
      const dy=((yaw-p.yaw+Math.PI*3)%(Math.PI*2))-Math.PI,dp=pitch-p.pitch;
      if(Math.abs(dy)<.045&&Math.abs(dp)<.045)return true;
      if(Math.abs(dy)>.045)await press(dy>0?'ArrowLeft':'ArrowRight',Math.min(180,Math.max(10,Math.abs(dy)/1.2*1000)));
      if(Math.abs(dp)>.045)await press(dp>0?'ArrowUp':'ArrowDown',Math.min(180,Math.max(10,Math.abs(dp)/1.2*1000)));
    }return false;
  }
  async function mine(item,end){
    const o=observe(),held=o.items.find(i=>i.id===o.inventory[o.selected]?.item);
    const candidates=o.targets.filter(t=>(!item||t.item===item)&&(o.mode==='creative'||t.tier<=(held?.tier||0))&&Math.hypot(t.point[0]-o.player.position[0],t.point[2]-o.player.position[2])>1);
    const ingredient=o.recipes[0]&&Object.keys(o.recipes[0].ingredients)[0];
    const target=candidates.find(t=>t.item===ingredient)||candidates[0];
    if(!target||!await aim(target.point,end))return null;
    const cells=[target.cell],before=evidence(observe(cells));
    await press('e',Math.min((o.mode==='creative'?.12:target.hardness)*1000+120,Math.max(0,end-performance.now())));
    return {voxel_before:before,voxel_after:evidence(observe(cells))};
  }
  async function craft(end){
    let o=observe();const recipe=o.recipes.find(r=>o.mode==='creative'||Object.entries(r.ingredients).every(([id,n])=>n<=count(o,id)+2&&o.targets.some(t=>t.item===id&&t.tier===0)));
    if(!recipe)return null;
    for(const [id,n] of Object.entries(recipe.ingredients))while(observe().mode!=='creative'&&count(observe(),id)<n&&remaining(end)){if(!await mine(id,end))return null;}
    const before=evidence(observe());await press('i');
    const button=[...document.querySelectorAll('[data-recipe]')].find(b=>b.dataset.recipe===recipe.id);
    button?.click();await wait(35);const after=evidence(observe());await press('i');return {voxel_before:before,voxel_after:after};
  }
  async function place(end){
    let o=observe(),slot=o.inventory.findIndex(s=>s&&o.items.some(i=>i.id===s.item&&i.block));
    if(slot<0){await mine(null,end);o=observe();slot=o.inventory.findIndex(s=>s&&o.items.some(i=>i.id===s.item&&i.block));}
    if(slot<0||slot>8)return null;await press(String(slot+1));
    const target=o.targets.find(t=>t.cell[1]<o.player.position[1]&&t.distance<5&&Math.hypot(t.point[0]-o.player.position[0],t.point[2]-o.player.position[2])>1.4);
    if(!target||!await aim(target.point,end))return null;
    const cells=observe().target?[observe().target.previous]:[];const before=evidence(observe(cells));await press('f');return {voxel_before:before,voxel_after:evidence(observe(cells))};
  }
  async function combat(end){
    const initial=observe(),enemy=initial.enemies.filter(e=>e.health>0).sort((a,b)=>Math.hypot(a.position[0]-initial.player.position[0],a.position[2]-initial.player.position[2])-Math.hypot(b.position[0]-initial.player.position[0],b.position[2]-initial.player.position[2]))[0];
    if(!enemy)return null;
    const before=evidence(initial);
    while(remaining(end)){
      const o=observe(),current=o.enemies.find(e=>e.id===enemy.id);if(!current||current.health<enemy.health)break;
      if(!await aim(current.position,end))break;
      const distance=Math.hypot(current.position[0]-o.player.position[0],current.position[2]-o.player.position[2]);
      if(distance>2.7)await press('w',170);else await press('e',400);
    }return {voxel_before:before,voxel_after:evidence(observe())};
  }
  async function capture(){
    try{const source=binding()?.canvas;if(!source?.width)return null;const c=document.createElement('canvas');c.width=640;c.height=Math.round(640*source.height/source.width);if(c.height>1280)return null;c.getContext('2d').drawImage(source,0,0,c.width,c.height);const image=c.toDataURL('image/png');if(image.length>700000)return null;return {image,width:c.width,height:c.height,controlled:running,scenario:'voxel',hud:document.querySelector('[data-voxel-status]')?.textContent.slice(0,4000)||''};}catch{return null;}
  }
  window.addEventListener('message',async event=>{
    const d=event.data;if(event.source!==parent||!channel||d?.channel!==channel||d.source!=='aurago-studio')return;
    if(d.type==='capture'&&!running){const c=await capture();parent.postMessage({source:'aurago-game',channel,type:'capture',request_id:d.request_id,captures:c?[c]:[]},'*');return;}
    if(d.type!=='run-tests'||running||!Array.isArray(d.scenarios)||d.scenarios.length>16)return;
    running=true;deadline=performance.now()+55000;const observations=[],captures=[];
    try{
      while(!observe()?.ready&&remaining(deadline))await wait(50);
      if(!observe()?.ready)throw Error('Voxel world did not become ready');
      await press('Enter');await wait(200);
      const image=await capture();if(image)captures.push(image);
      for(const scenario of d.scenarios){
        if(!remaining(deadline))break;const end=Math.min(deadline,performance.now()+6000),before=binding().snapshot();let state;
        switch(scenario.metric){
          case 'voxel_move': {const a=evidence(observe());await press('d',220);state={voxel_before:a,voxel_after:evidence(observe())};await press('a',220);break;}
          case 'voxel_jump': {const a=evidence(observe());await press(' ',200);state={voxel_before:a,voxel_after:evidence(observe())};await wait(600);break;}
          case 'voxel_mine': state=await mine(null,end);break;
          case 'voxel_craft':state=await craft(end);break;
          case 'voxel_place':state=await place(end);break;
          case 'voxel_combat':state=await combat(end);break;
          case 'voxel_pause':await press('Escape');{const a=evidence(observe());await press('w',250);state={voxel_before:a,voxel_after:evidence(observe())};}await press('Escape');break;
          case 'voxel_respawn':{const a=evidence(observe());await press('r');state={voxel_before:a,voxel_after:evidence(observe())};break;}
          default:for(const s of (scenario.steps||[]).slice(0,8)){if(!remaining(end))break;if(s.action==='key'){const names={LEFT:'ArrowLeft',RIGHT:'ArrowRight',UP:'ArrowUp',DOWN:'ArrowDown',SPACE:' ',ESC:'Escape',ENTER:'Enter'};await press(names[s.key]||String(s.key).toLowerCase(),Math.min(4000,s.ms||50));}else if(s.action==='wait'||s.action==='observe')await wait(Math.min(4000,s.ms||34));}
        }
        observations.push({id:scenario.id,before,after:binding().snapshot(),...state});
      }
      const imageAfter=await capture();if(imageAfter)captures.push(imageAfter);
    }catch(error){parent.postMessage({source:'aurago-game',channel,type:'diagnostic',message:String(error).slice(0,1000)},'*');}
    finally{for(const k of ['w','a','s','d','e','f',' ','ArrowLeft','ArrowRight','ArrowUp','ArrowDown'])key(k,false);running=false;parent.postMessage({source:'aurago-game',channel,type:'gameplay',observations,images:[],captures},'*');}
  });
})();
