// MIT. Shared player feedback and stage navigation; updated by the game loop.
export function levelChoices(scene, authored = []) {
  const levels = authored.length ? authored : scene?.levels || [];
  return levels.slice(0, 16).map((level, index) => ({...level, id:String(level.id || index), title:String(level.title || level.name || `Stage ${index + 1}`)}));
}
export function sceneForLevel(scene, id) {
  if (!scene?.levels?.length || id === undefined) return scene;
  if (!scene.levels.some(level => level.id === id)) throw Error('Unknown level: ' + id);
  return {...scene, levels:scene.levels.map(level => ({...level, active:level.id === id}))};
}
export function createGameFlow({root, feedback = () => {}, project = () => null, restart, advance, stage = 0, stages = [], labels = {}}) {
  const de = (document.documentElement.lang || navigator.language || '').startsWith('de');
  const text = {...(de ? {won:'Geschafft!', lost:'Game Over', ended:'Spiel beendet', death:'Leben verloren', damage:'Getroffen', respawn:'Bereit!', next:'Weiter', restart:'Neu starten', score:'Punkte', stage:'Abschnitt'} :
    {won:'Well done!', lost:'Game Over', ended:'Run finished', death:'Life lost', damage:'Hit', respawn:'Ready!', next:'Continue', restart:'Restart', score:'Score', stage:'Stage'}), ...labels};
  const controller = new AbortController(), reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const layer = document.createElement('div');layer.dataset.gameFlow = 'true';
  layer.style.cssText='position:absolute;inset:0;overflow:hidden;pointer-events:none;z-index:1050;font:600 16px/1.5 system-ui;color:#fff;';
  const notice=document.createElement('div');notice.setAttribute('role','status');
  notice.style.cssText='position:absolute;left:50%;top:24%;transform:translateX(-50%);text-align:center;max-width:85%;padding:9px 18px;border-radius:10px;background:#101822ee;opacity:0;';layer.append(notice);root.append(layer);
  let clock=0,previous=null,panel=null,noticeUntil=0,disposed=false;
  const particles=[],recent=new Map(),counts={};
  function message(value,seconds=1.1){notice.textContent=String(value).slice(0,160);noticeUntil=clock+seconds;notice.style.opacity='1';}
  function event(name,point,normal,material) {
    if(disposed || clock-(recent.get(name)??-99)<.06)return;
    if(recent.size>=32&&!recent.has(name))recent.delete(recent.keys().next().value);
    recent.set(name,clock);if(Object.keys(counts).length<32||Object.hasOwn(counts,name))counts[name]=(counts[name]||0)+1;
    feedback(name,point,normal,material);
    if(['damage','death'].includes(name)){message(text[name]);layer.style.boxShadow=`inset 0 0 ${reduced?25:90}px #e14c6477`;}
    if(name==='respawn')message(text.respawn);
    if(['hit','pickup','destroy'].includes(name)){
      const p=project(point);if(!p||!Number.isFinite(p.x)||!Number.isFinite(p.y))return;
      const marker=document.createElement('span');marker.dataset.gameFeedback=name;marker.textContent=name==='pickup'?'+':'✦';
      marker.style.cssText=`position:absolute;left:${p.x*100}%;top:${p.y*100}%;font:bold 28px monospace;color:${name==='pickup'?'#ffdf75':'#fff'};text-shadow:0 2px 3px #000;transform:translate(-50%,-50%);`;
      layer.append(marker);particles.push({node:marker,at:clock});while(particles.length>24)particles.shift().node.remove();
    }
  }
  function finish(state) {
    const outcome=state.outcome===1?'won':state.outcome===2?'lost':'ended';
    if(panel)return;
    const resultEvent=outcome==='won'?'win':'lose';
    if(outcome!=='ended'&&!counts[resultEvent])event(resultEvent);
    panel=document.createElement('section');panel.dataset.gameResult=outcome;panel.setAttribute('role','dialog');panel.setAttribute('aria-label',text[outcome]);
    panel.style.cssText='position:absolute;inset:0;display:flex;align-items:center;justify-content:center;background:#07111a99;';
    const card=document.createElement('div');card.style.cssText='pointer-events:auto;width:min(360px,85%);max-height:88%;overflow:auto;box-sizing:border-box;padding:24px;text-align:center;border:1px solid #ffffff55;border-radius:18px;background:#101b2af5;box-shadow:0 18px 60px #0008;';
    const title=document.createElement('h2');title.textContent=text[outcome];title.style.cssText='margin:0 0 10px;font-size:clamp(24px,4vw,36px)';
    const detail=document.createElement('p');detail.textContent=`${text.score}: ${Math.round(Number(state.score)||0)}`+(stages.length>1?` · ${text.stage} ${stage+1}/${stages.length}`:'');
    const buttons=document.createElement('div');buttons.style.cssText='display:flex;gap:10px;flex-wrap:wrap;justify-content:center';
    const add=(label,callback)=>{const button=document.createElement('button');button.textContent=label;button.style.cssText='font:inherit;min-height:44px;padding:10px 18px;border:1px solid #ffffff55;border-radius:9px;background:#244651;color:#fff;cursor:pointer';button.addEventListener('click',()=>{button.disabled=true;callback?.();},{signal:controller.signal});buttons.append(button);};
    if(outcome==='won'&&stage+1<stages.length&&advance)add(text.next,advance);
    add(text.restart,restart);card.append(title,detail,buttons);panel.append(card);layer.append(panel);
  }
  const api={event,message,
    update(dt,state,active=true){
      if(disposed||!active)return;clock+=Math.min(.1,Math.max(0,dt));
      if(previous){
        const lifeLost=Number.isFinite(state.lives)&&Number.isFinite(previous.lives)&&state.lives<previous.lives;
        const damaged=Number.isFinite(state.health)&&Number.isFinite(previous.health)&&state.health<previous.health;
        if(lifeLost||damaged&&state.health<=0)event('death');else if(damaged)event('damage');
      }
      if(state.ended||state.outcome===1||state.outcome===2)finish(state);
      previous={lives:state.lives,health:state.health};
      notice.style.opacity=clock<noticeUntil?'1':'0';
      if(clock-Math.max(recent.get('damage')??-99,recent.get('death')??-99)>.3)layer.style.boxShadow='none';
      for(let i=particles.length-1;i>=0;i--){const p=particles[i],age=clock-p.at;if(age>.4){p.node.remove();particles.splice(i,1)}else {p.node.style.opacity=String(1-age/.4);if(!reduced)p.node.style.transform=`translate(-50%,-50%) scale(${1+age*2})`;}}
    },
    reset(){previous=null;clock=noticeUntil=0;panel?.remove();panel=null;notice.style.opacity='0';layer.style.boxShadow='none';for(const p of particles)p.node.remove();particles.length=0;recent.clear();for(const key of Object.keys(counts))delete counts[key];},
    audit(){return {feedback:{...counts},result:panel?.dataset.gameResult||'',stage:stage+1,stages:Math.max(1,stages.length),markers:particles.length};},
    dispose(){if(disposed)return;disposed=true;controller.abort();layer.remove();particles.length=0;recent.clear();},
  };
  return api;
}
