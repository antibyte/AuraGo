// MIT. Engine adapters own geometry, this controller owns settings and game-time lifecycle.
import {createAudio} from './audio.js';
const clamp=(x,a,b)=>Math.max(a,Math.min(b,x));
export function createPresentation({adapter,config,root,report=console.warn,controls=true}) {
    config ||= {effects:[],sounds:[],bindings:[],quality:'auto'};
    const records=new Map((config.effects||[]).map(e=>[e.id,e])), active=new Map(), lifecycle=new AbortController();
    const audio=createAudio({sounds:config.sounds,base:config.base,root,report,dimension:adapter.dimension});
    const usage={events:{},effects:{},sounds:{}};
    const count=(group,id)=>{if(Object.keys(group).length<64||Object.hasOwn(group,id))group[id]=Math.min(1000000,(group[id]||0)+1)};
    const bindings=new Map();for(const b of config.bindings||[])bindings.set(b.event,[...(bindings.get(b.event)||[]),b.sound]);
    let quality=config.quality||'auto',level=2,time=0,paused=false,inactive=document.hidden,disposed=false,slow=0,fast=0,blood=true,thunderAt=0;
    const motion=matchMedia('(prefers-reduced-motion: reduce)');let reduced=motion.matches;
    function applyQuality(){adapter.quality(level,reduced)}
    function params(id,options={}){
        const record=records.get(id);if(!record)throw Error('presentation: unknown imported effect '+id);
        const p={...record.defaults};
        for(const [key,value] of Object.entries(options)){
            if(['position','normal'].includes(key)){if(!Array.isArray(value)||value.length!==3||value.some(x=>!Number.isFinite(x)||Math.abs(x)>1e6))throw Error('presentation: invalid '+key);p[key]=value.slice()}
            else if(key==='color'){if(!/^#[a-f0-9]{6}$/i.test(value))throw Error('presentation: invalid color');p[key]=value}
            else if(key==='fixed'){if(typeof value!=='boolean')throw Error('presentation: invalid fixed');p[key]=value}
            else if(['intensity','scale','lifetime','cycle','hour','width','depth','y','speed','density','radius'].includes(key)){if(!Number.isFinite(value))throw Error('presentation: invalid '+key);p[key]=clamp(value,key==='y'?-10000:0,key==='cycle'?86400:key==='width'||key==='depth'?1000:key==='hour'?24:key==='lifetime'?120:100)}
            else throw Error('presentation: unknown parameter '+key);
        }
        return p;
    }
    function set(id,options={}){const p=params(id,options);active.set(id,p);adapter.set(id,p);return api}
    function emit(id,options={}){
        if(paused||inactive||disposed)return;
        const p=params(id,options);
        if(id.startsWith('blood')&&!blood){if(records.has('metal-sparks'))adapter.emit('metal-sparks',p);return}
        adapter.emit(id,p);count(usage.effects,id);
    }
    const customEvents=new Map();
    const eventEffects={shot:['muzzle-flash'],hit:['blood-spray','blood-pool','blood-decal','metal-sparks','stone-debris','hit-flash'],pickup:['pickup-glow'],splash:['water-splash','water-ripple'],win:['magic']};
    function event(name,position=[0,0,0],normal=[0,1,0],material='flesh'){
        if(paused||inactive||disposed)return;
        count(usage.events,name);
        for(const id of customEvents.has(name)?customEvents.get(name).effects:eventEffects[name]||[])if(records.has(id)&&(!id.startsWith('blood')||material==='flesh')&&(id!=='metal-sparks'||material==='metal')&&(id!=='stone-debris'||material==='stone'))emit(id,{position,normal});
        const ids=customEvents.has(name)?customEvents.get(name).sounds:bindings.get(name),id=ids?.[Math.floor(Math.random()*ids.length)];if(id){count(usage.sounds,id);audio.play(id,{position:adapter.dimension==='2d'?[position[0],position[1],0]:position});}
    }
    function syncPause(){audio.setPaused(paused||inactive)}
    lifecycle.signal.addEventListener('abort',()=>audio.dispose(),{once:true});
    document.addEventListener('visibilitychange',()=>{inactive=document.hidden;syncPause()},{signal:lifecycle.signal});
    motion.addEventListener('change',e=>{reduced=e.matches;applyQuality()},{signal:lifecycle.signal});
    const env=records.get(config.environment);
    let ambient=(env?.sounds||[]).filter(id=>config.sounds?.find(s=>s.id===id)?.loop);
    if(bindings.has('ambient'))ambient.push(...bindings.get('ambient'));
    const api={audio,set,emit,event,
        bindEvent(name,{effect,sound}={}){
            if(typeof name!=='string'||!name||name.length>64)throw Error('presentation: invalid event name');
            if(effect&&!records.has(effect))throw Error('presentation: event effect was not imported '+effect);
            if(sound&&!config.sounds?.some(s=>s.id===sound))throw Error('presentation: event sound was not imported '+sound);
            customEvents.set(name,{effects:effect?[effect]:[],sounds:sound?[sound]:[]});return api;
        },
        setEnvironment(id){const e=records.get(id);if(e?.category!=='environment')throw Error('presentation: environment was not imported');active.clear();adapter.clear?.();for(const effect of e.effects)set(effect);ambient=e.sounds.filter(s=>config.sounds?.find(a=>a.id===s)?.loop);audio.ambience(ambient);return api},
        setPaused(v){paused=!!v;syncPause()},setActive(v){inactive=!v||document.hidden;syncPause()},
        setBlood(v){blood=!!v;adapter.blood?.(blood)},
        setQuality(v){if(!['auto','low','medium','high'].includes(v))throw Error('presentation: unknown quality');quality=v;level=v==='low'?0:v==='medium'?1:2;slow=fast=0;applyQuality()},
        registerSurface(...args){return adapter.registerSurface(...args)},applyObject(object,id,options){return adapter.applyObject(object,id,params(id,options))},
        update(dt){
            if(paused||inactive||disposed)return;const elapsed=clamp(dt,0,.1);time+=elapsed;
            if(quality==='auto'){if(dt>.025){slow+=elapsed;fast=0}else{fast+=elapsed;slow=Math.max(0,slow-elapsed)}if(slow>2&&level>0){level--;slow=0;applyQuality()}if(fast>12&&level<2){level++;fast=0;applyQuality()}}
            if(adapter.update(elapsed,time)?.thunder)thunderAt=time+1.2;
            if(thunderAt&&time>=thunderAt){thunderAt=0;if(config.sounds?.some(s=>s.id==='thunder'))audio.play('thunder',{gain:.4})}
            const cycle=active.get('day-night');if(cycle&&ambient.includes('forest-day')&&ambient.includes('forest-night')){
                const hour=cycle.fixed?cycle.hour:(cycle.hour+time*24/Math.max(1,cycle.cycle))%24;
                audio.ambience([hour>6&&hour<19?'forest-day':'forest-night',...ambient.filter(s=>!s.startsWith('forest-'))]);
            }else audio.ambience(ambient);
        },
        render(){adapter.render?.()},resize(){adapter.resize?.()},
        reset(){time=thunderAt=0;adapter.reset();audio.reset();slow=fast=0;for(const group of Object.values(usage))for(const key of Object.keys(group))delete group[key]},
        // Counts show controller delivery; audio requests do not prove audibility.
        audit(){return {events:{...usage.events},effects:{...usage.effects},sounds:{...usage.sounds}}},
        stats(){return {...adapter.stats(),...audio.stats(),quality:['low','medium','high'][level],time}},
        dispose(){if(disposed)return;disposed=true;lifecycle.abort();panel?.remove();adapter.dispose()},
    };
    let panel;
    if(controls&&(records.size||config.sounds?.length)){
        panel=document.createElement('div');panel.className='aurago-game-presentation';panel.style.cssText='position:absolute;right:12px;top:12px;z-index:1100;display:flex;gap:8px;align-items:center;padding:7px 10px;border:1px solid #ffffff30;border-radius:12px;background:#101822d9;color:#fff;font:12px system-ui;max-width:90%';
        const mute=document.createElement('button');mute.textContent='♪';mute.title='Sound';mute.setAttribute('aria-label','Mute sound');mute.setAttribute('aria-pressed','false');let muted=false;
        mute.onclick=()=>{muted=!muted;mute.textContent=muted?'♫ ×':'♪';mute.setAttribute('aria-pressed',String(muted));audio.setMuted(muted)};
        const volume=document.createElement('input');volume.type='range';volume.min='0';volume.max='100';volume.value='55';volume.style.width='68px';volume.setAttribute('aria-label','Volume');volume.oninput=()=>audio.setVolume(+volume.value/100);
        const qualitySelect=document.createElement('select');qualitySelect.setAttribute('aria-label','Effects quality');for(const id of ['auto','low','medium','high'])qualitySelect.add(new Option(id,id));qualitySelect.value=quality;qualitySelect.onchange=()=>api.setQuality(qualitySelect.value);
        panel.append(mute,volume,qualitySelect);
        if([...records.keys()].some(id=>id.startsWith('blood'))){const b=document.createElement('button');b.textContent='●';b.title='Blood effects';b.setAttribute('aria-label','Blood effects');b.setAttribute('aria-pressed','true');b.onclick=()=>{api.setBlood(!blood);b.setAttribute('aria-pressed',String(blood))};panel.append(b)}
        panel.style.colorScheme='dark';panel.style.accentColor='#71dfce';panel.style.flexWrap='wrap';
        for(const input of panel.querySelectorAll('button,select'))input.style.cssText='font:inherit;color:inherit;border:1px solid #ffffff28;border-radius:7px;background:#243443;padding:6px;min-height:'+(matchMedia('(pointer: coarse)').matches?44:32)+'px';
        root.append(panel);
    }
    for(const a of records.values())if(a.category!=='environment')set(a.id);
    api.setQuality(quality);audio.ambience(ambient);syncPause();return api;
}
