// MIT. Opaque-origin previews use only the authenticated parent; exports use IndexedDB.
import {validateVoxelState} from './voxel-rules.js';
const MAX_BYTES=4*1024*1024;
function localIdentity(d){const byID=(a,b)=>a.id<b.id?-1:a.id>b.id?1:0;return JSON.stringify([1,'generator-1','save-1',d.mode,d.seed,d.size,d.terrain,[...d.blocks].sort((a,b)=>a.id-b.id).map(b=>[b.id,b.key,b.material]),[...d.items].sort(byID).map(i=>[i.id,i.block,i.stack]),[...(d.enemies||[])].sort(byID).map(e=>[e.id,e.behavior,e.count])]);}
export function createVoxelPersistence({definition,snapshot,restore,status}) {
  const params=new URLSearchParams(location.hash.slice(1)),channel=params.get('gm-channel'),embedded=window.parent!==window;
  const controller=new AbortController(),pending=new Map();let db=null,version=0,loaded=false,temporary=false,conflict=false,disposed=false,inflight=null,lastSave=0;
  // A sandboxed top-level preview must never silently become a local exported game.
  const bridge=Boolean(channel),localKey=location.pathname.replace(/[^/]*$/,'')+localIdentity(definition);
  window.addEventListener('message',event=>{
    if(event.source!==parent||event.data?.source!=='aurago-voxel-host'||event.data.channel!==channel||event.data.type!=='play_state')return;
    const p=pending.get(event.data.request);if(!p)return;pending.delete(event.data.request);clearTimeout(p.timer);
    if(event.data.error){const error=new Error('Voxel save request failed');error.code=event.data.error;p.reject(error);}else p.resolve(event.data.result);
  },{signal:controller.signal});
  async function openDB(){return new Promise((resolve,reject)=>{
    const request=indexedDB.open('aurago-voxel',1);request.onupgradeneeded=()=>request.result.createObjectStore('worlds');request.onerror=()=>reject(request.error);request.onblocked=()=>reject(new Error('Voxel storage blocked'));request.onsuccess=()=>{db=request.result;db.onversionchange=()=>db.close();resolve(db);};
  });}
  async function request(operation,payload){
    if(disposed)throw new Error('Voxel storage closed');
    if(bridge){if(!embedded)return {temporary:true,version:0,state:null};return new Promise((resolve,reject)=>{
      const id=crypto.randomUUID?.()||String(Date.now())+'-'+Math.random();const timer=setTimeout(()=>{pending.delete(id);reject(new Error('Voxel host timeout'));},10000);
      pending.set(id,{resolve,reject,timer});parent.postMessage({source:'aurago-voxel',type:'play_state',channel,request:id,operation,version,payload},'*');
    });}
    if(embedded)return {temporary:true,version:0,state:null};
    if(!db)await openDB();
    return new Promise((resolve,reject)=>{
      const tx=db.transaction('worlds',operation==='load'?'readonly':'readwrite'),store=tx.objectStore('worlds');let result,error;
      const get=store.get(localKey);get.onsuccess=()=>{
        const existing=get.result||{version:0,state:null};
        if(operation==='load'){result=existing;return;}
        if(existing.version!==version){error=Object.assign(new Error('Stale voxel save'),{code:'conflict'});tx.abort();return;}
        result={version:version+1,state:operation==='reset'?null:payload};store.put(result,localKey);
      };
      tx.oncomplete=()=>resolve(result);tx.onabort=tx.onerror=()=>reject(error||tx.error||new Error('Voxel storage failed'));
    });
  }
  function report(error){if(disposed)return;conflict=error.code==='conflict';status(conflict?'conflict':'saveError');}
  async function load(){
    try {const result=await request('load');if(disposed)return;temporary=Boolean(result.temporary);version=result.version;conflict=false;
      if(result.state){validateVoxelState(definition,result.state);restore(result.state);}loaded=true;status(temporary?'transient':'');return result.state;
    } catch(error){loaded=false;report(error);return null;}
  }
  const api={ready:null,
    async save(force=false){if(!loaded||temporary||conflict||disposed)return false;if(inflight){const ok=await inflight;if(!force||!ok)return ok;return api.save(force);}if(!force&&Date.now()-lastSave<5000)return false;
      const state=snapshot();try{validateVoxelState(definition,state);if(new TextEncoder().encode(JSON.stringify(state)).length>MAX_BYTES)throw new Error('Voxel save exceeds 4 MiB');}catch(error){report(error);return false;}
      lastSave=Date.now();status('saving');inflight=(async()=>{try{const result=await request('save',state);version=result.version;status(bridge?'saved':'local');return true;}catch(error){report(error);return false;}finally{inflight=null;}})();return inflight;
    },
    async reload(){if(inflight)await inflight;return load();},
    async reset(){if(inflight)await inflight;if(!loaded||temporary||conflict)return false;try{const result=await request('reset');version=result.version;status('');return true;}catch(error){report(error);return false;}},
    get temporary(){return temporary;},get loaded(){return loaded;},
    dispose(){disposed=true;controller.abort();for(const p of pending.values()){clearTimeout(p.timer);p.reject(new Error('Voxel storage closed'));}pending.clear();db?.close();},
  };
  api.ready=load();return api;
}
