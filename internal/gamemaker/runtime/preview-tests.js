// Server-injected finite test driver. Never accepts or evaluates JavaScript.
(function () {
  let running = false;
  const channel = new URLSearchParams(location.hash.slice(1)).get('gm-channel');
  const keys = { LEFT:37,RIGHT:39,UP:38,DOWN:40,W:87,A:65,S:83,D:68,SPACE:32,R:82,ESC:27,ENTER:13 };
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
    if (!b?.scene?.sys?.isActive() || !b.player?.active) throw Error('Missing live bindGameTest scene/player');
    return b;
  }
  async function reset() {await press('R',80);await wait(420);}
  function snapshot(inspectAssets) {
    const {scene,state,player}=binding();
    const result={player_x:player.x,player_y:player.y,
      // Phaser 4.2.1 has no public timer enumeration; this adapter is pinned and
      // exercised by the real-runtime browser fixture when Phaser is upgraded.
      object_count:scene.children.list.length,timer_count:new Set([...scene.time._active,...scene.time._pendingInsertion].filter(timer=>!scene.time._pendingRemoval.includes(timer))).size,
      listener_count:scene.input.keyboard.eventNames().reduce((n,event)=>n+scene.input.keyboard.listenerCount(event),0),
      elapsed_ms:scene.elapsed,...inspectAssets(scene)};
    for(const name of ['actions','score','hits','spawns','turns','ticks','ended'])if(Number.isFinite(state[name]))result[name]=state[name];
    return result;
  }
  async function step(command) {
    if(!command||!Number.isFinite(command.ms||0)||command.ms<0||command.ms>4000)throw Error('Invalid test duration');
    const ms=command.ms||50;
    switch(command.action){
      case 'key': await press(command.key,ms);break;
      case 'wait': await wait(ms);break;
      case 'observe': await wait(34);break;
      case 'pointer': {
        const {scene}=binding(),canvas=scene.game.canvas,rect=canvas.getBoundingClientRect();
        if(!Number.isFinite(command.x)||!Number.isFinite(command.y)||command.x<0||command.y<0||command.x>1920||command.y>1080)throw Error('Invalid pointer');
        const options={bubbles:true,clientX:rect.left+command.x/scene.scale.width*rect.width,clientY:rect.top+command.y/scene.scale.height*rect.height,button:0,buttons:1};
        canvas.dispatchEvent(new MouseEvent('mousedown',options));await wait(ms);canvas.dispatchEvent(new MouseEvent('mouseup',{...options,buttons:0}));break;
      }
      default: throw Error('Unsupported test command');
    }
  }
  async function capture() {
    try {
      const {scene}=binding();
      return await new Promise(resolve=>{
        const timer=setTimeout(()=>resolve(''),500);
        scene.game.renderer.snapshot(image=>{clearTimeout(timer);resolve(image.src.length<=700000?image.src:'');},'image/png');
      });
    } catch (_) {return '';}
  }
  window.addEventListener('message',async event=>{
    const data=event.data;
    if(event.source!==parent||!channel||data?.channel!==channel||data.source!=='aurago-studio'||data.type!=='run-tests'||running)return;
    if(!Array.isArray(data.scenarios)||data.scenarios.length>16)return;
    running=true;
    const observations=[],images=[];let expired=false,initial;
    const deadline=setTimeout(()=>{expired=true;for(const key of Object.keys(keys))keyEvent(key,false);},55000);
    try {
      const {inspectAssets}=await import('./vendor/aurago-game-1.js');
      // Startup must remain alive before test input starts.
      await wait(3100);
      const first=await capture();if(first)images.push(first);
      for(const scenario of data.scenarios){
        if(expired)throw Error('Gameplay driver exceeded 55 seconds');
        if(!Array.isArray(scenario.steps)||scenario.steps.length>8)throw Error('Too many test steps');
        await reset();const current=snapshot(inspectAssets);if(!initial)initial=current;
        const before=scenario.id==='required_restart'?initial:current;let restart1;
        for(const command of scenario.steps){await step(command);if(scenario.id==='required_restart'&&command.action==='wait'&&!restart1)restart1=snapshot(inspectAssets);if(expired)throw Error('Gameplay driver timed out');}
        const after=snapshot(inspectAssets);
        if(restart1)for(const [key,value] of Object.entries(restart1))after['restart1_'+key]=value;
        observations.push({id:scenario.id,before,after});
		if(scenario.id==='required_rules'){const actionImage=await capture();if(actionImage&&images.length<2)images.push(actionImage);}
      }
      if(images.length<2){const last=await capture();if(last)images.push(last);}
    } catch(error) {
      // Missing evidence remains unavailable in the server comparator.
      parent.postMessage({source:'aurago-game',channel,type:'diagnostic',message:String(error).slice(0,1000)},'*');
    } finally {
      clearTimeout(deadline);
      for(const key of Object.keys(keys))keyEvent(key,false);
      try{await reset();}catch(_){}
      parent.postMessage({source:'aurago-game',channel,type:'gameplay',observations,images},'*');
    }
  });
})();
