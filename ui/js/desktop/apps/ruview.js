(function(){
    'use strict';
    const instances=new Map(),base='/api/desktop/integrations/ruview/';
    function dispose(id){instances.get(id)?.();instances.delete(id);}
    function render(host,id,ctx){
        dispose(id);const controller=new AbortController(),t=key=>{const value=ctx.t('ruview.'+key);return value==='ruview.'+key?ctx.t('ruview.unavailable'):value;},esc=ctx.esc;let dead=false,tab='overview',status={},wizardClose=null,actionError='';
        const errorText=message=>{if(!message)return '';const key='ruview.'+message,value=ctx.t(key);return value===key?message:value;};
        host.innerHTML=`<div class="ruview-app"><nav class="ruview-toolbar" aria-label="RuView"><strong>RuView</strong>${[['overview','overview'],['devices','devices'],['index.html','dashboard'],['observatory.html','observatory'],['pose-fusion.html','pose_fusion']].map(([name,label])=>`<button data-tab="${name}">${esc(t(label))}</button>`).join('')}<a href="/config#ruview" target="_blank" rel="noopener">${esc(t('settings'))}</a></nav><div class="ruview-content"></div></div>`;
        const content=host.querySelector('.ruview-content');
        async function request(route,method='GET'){const response=await fetch(base+route,{method,signal:controller.signal});const body=await response.json().catch(()=>({}));if(!response.ok)throw Error(errorText(body.error||body.message||('HTTP '+response.status)));return body;}
        function summary(){
            content.innerHTML=`<h2>${esc(t('overview'))}</h2><div class="ruview-summary"><article>${esc(t('service'))}<strong>${esc(t(status.ready?'ready':'unavailable'))}</strong>${esc(status.mode?t(status.mode):'—')}</article><article>${esc(t('sensing'))}<strong>${esc(t(status.fresh?'received':status.simulated?'demo':status.data_state==='live_unverified'?'unavailable':status.data_state||'unchecked'))}</strong>${esc(status.signal||'—')}</article><article>${esc(t('source'))}<strong>${esc(status.source?t(status.source):'—')}</strong>${esc(status.observed_at?new Date(status.observed_at).toLocaleString():'—')}</article></div><p class="ruview-note">${esc(t('quality'))}</p><div class="ruview-toolbar">${[['test','test'],['runtime/install','install'],['runtime/start','start'],['runtime/stop','stop'],['flash','flash']].map(([action,label])=>`<button data-action="${action}" ${(!status.enabled||action!=='test'&&status.readonly||action.startsWith('runtime/')&&status.mode==='external'||action==='runtime/install'&&status.mode!=='native')?'disabled':''}>${esc(t(label))}</button>`).join('')}</div><p class="ruview-feedback" role="status">${esc(errorText(actionError||status.error))}</p>`;
        }
        async function show(){
            const mine=tab;host.querySelectorAll('[data-tab]').forEach(b=>b.setAttribute('aria-pressed',String(b.dataset.tab===tab)));
            try{status=await request('status');if(dead||tab!==mine)return;
                if(tab==='overview'){summary();return}
                if(tab==='devices'){const data=await request('proxy/api/v1/nodes');if(dead||tab!==mine)return;const nodes=Array.isArray(data)?data:(data.nodes||[]);content.innerHTML=`<h2>${esc(t('devices'))}</h2>`;if(!nodes.length){const empty=document.createElement('p');empty.textContent=t('no_frames');content.append(empty);}for(const node of nodes){const row=document.createElement('article');row.className='ruview-node';row.innerHTML=`<strong>${esc(t('node'))} ${esc(node.node_id??node.id)}</strong><div><span>CSI</span>${esc(t(node.csi_status==='active'?'received':'stale'))}</div><div><span>RSSI</span>${esc(node.rssi_dbm??'—')} dBm</div><div><span>${esc(t('source'))}</span>${esc(node.csi_last_seen_ms??'—')} ms</div>`;content.append(row);}const button=document.createElement('button');button.dataset.action='flash';button.textContent=t('configure');button.disabled=!status.enabled||status.readonly;content.append(button);return}
                if(!status.enabled){content.textContent=t('disabled');return}
                content.replaceChildren();if(mine==='pose-fusion.html'){const note=document.createElement('p');note.className='ruview-note';note.setAttribute('role','note');note.textContent=t('fusion_notice');content.append(note);}const frame=document.createElement('iframe');frame.title='RuView · '+mine;frame.sandbox='allow-scripts allow-same-origin';frame.allow="serial 'none'; camera 'none'; microphone 'none'";frame.src=base+'ui/'+mine;content.append(frame);
            }catch(error){if(!dead)content.textContent=errorText(error.message);}
        }
        host.addEventListener('click',async event=>{
            const button=event.target.closest('button');if(!button)return;
            if(button.dataset.tab){tab=button.dataset.tab;content.replaceChildren();await show();return}
            if(!button.dataset.action)return;button.disabled=true;actionError='';
            try{if(button.dataset.action==='flash'){const {openWizard}=await import(window.AuraLazyAssets.versionedURL('/js/ruview/wizard.mjs'));wizardClose=await openWizard({base,t:ctx.t});if(dead)wizardClose?.();}else{status=await request(button.dataset.action,'POST');if(tab==='overview'&&!dead)summary();}}
            catch(error){actionError=error.message;const out=host.querySelector('[role="status"]');if(out)out.textContent=errorText(actionError);}
            finally{if(button.isConnected)button.disabled=false;}
        },{signal:controller.signal});
        const timer=setInterval(async()=>{if(dead||tab!=='overview'||document.hidden)return;try{const next=await request('status');if(!dead&&tab==='overview'){const changed=JSON.stringify({...status,last_frame_age_ms:null})!==JSON.stringify({...next,last_frame_age_ms:null});status=next;if(changed)summary();}}catch{if(!dead&&tab==='overview'&&(status.ready||status.fresh)){status={...status,ready:false,fresh:false,data_state:'disconnected'};summary();}}},3000);
        const closed=()=>{dead=true;clearInterval(timer);controller.abort();wizardClose?.();host.querySelectorAll('iframe').forEach(f=>f.remove());};instances.set(id,closed);show();
    }
    window.RuViewApp={render,dispose};
})();
