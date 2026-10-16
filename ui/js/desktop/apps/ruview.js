(function () {
    'use strict';
    const instances = new Map(), base = '/api/desktop/integrations/ruview/';
    function dispose(id) { instances.get(id)?.(); instances.delete(id); }
    function render(host, id, ctx) {
        dispose(id);
        const controller = new AbortController(), esc = ctx.esc;
        const t = key => { const value = ctx.t('ruview.' + key); return value === 'ruview.' + key ? ctx.t('ruview.unavailable') : value; };
        const errorText = message => { if (!message) return ''; const key = 'ruview.' + message, value = ctx.t(key); return value === key ? message : value; };
        let dead = false, tab = 'overview', status = {}, wizardClose = null, actionError = '';
        let viewEpoch = 0, statusEpoch = 0, polling = false;
        let nodes = [], nodesAt = 0, nodesError = '', nodesLoaded = false, nodesScope = '';
        const pending = new Set();
        host.innerHTML = '<div class="ruview-app"><nav class="ruview-toolbar" aria-label="RuView"><strong>RuView</strong>' +
            [['overview','overview'],['devices','devices'],['index.html','dashboard'],['observatory.html','observatory'],['pose-fusion.html','pose_fusion']].map(([name,label]) => '<button data-tab="'+name+'">'+esc(t(label))+'</button>').join('') +
            '<a href="/config#ruview" target="_blank" rel="noopener">'+esc(t('settings'))+'</a></nav><div class="ruview-content"></div></div>';
        const content = host.querySelector('.ruview-content');
        async function request(route, method = 'GET') {
            const response = await fetch(base + route, {method, signal: controller.signal});
            const body = await response.json().catch(() => ({}));
            if (!response.ok) throw Error(errorText(body.error || body.message || ('HTTP ' + response.status)));
            return body;
        }
        function disabled(action) {
            return !status.enabled || (action !== 'test' && action !== 'runtime/stop' && status.readonly) ||
                (action.startsWith('runtime/') && status.mode === 'external') ||
                (action === 'runtime/install' && status.mode !== 'native') || pending.has(action);
        }
        function summary() {
            const focused = content.contains(document.activeElement) ? document.activeElement.dataset.action : null;
            const card = (title, value, detail) => '<article>'+esc(t(title))+'<strong>'+esc(value)+'</strong>'+esc(detail)+'</article>';
            content.innerHTML = '<h2>'+esc(t('overview'))+'</h2><div class="ruview-summary">' +
                card('service',t(!status.enabled?'disabled':status.ready?'ready':status.data_state==='starting'?'working':status.data_state==='unchecked'?'unchecked':'unavailable'),status.mode?t(status.mode):'—') +
                card('sensing',t(status.fresh?'received':status.simulated?'demo':status.data_state==='live_unverified'?'unavailable':status.data_state||'unchecked'),status.signal||'—') +
                card('source',status.source?t(status.source):'—',status.observed_at?new Date(status.observed_at).toLocaleString():'—') +
                '</div><p class="ruview-note">'+esc(t('quality'))+'</p>'+(status.readonly?'<p>'+esc(t('readonly_notice'))+'</p>':'')+'<div class="ruview-toolbar">' +
                [['test','test'],['runtime/install','install'],['runtime/start','start'],['runtime/stop','stop'],['flash','flash']].map(([action,label]) => '<button data-action="'+action+'" aria-busy="'+pending.has(action)+'" '+(disabled(action)?'disabled':'')+'>'+esc(t(label))+'</button>').join('') +
                '</div><p class="ruview-feedback" role="status">'+esc(errorText(actionError||status.error))+'</p>';
            if (focused) Array.from(content.querySelectorAll('[data-action]')).find(button => button.dataset.action === focused && !button.disabled)?.focus({preventScroll:true});
        }
        function deviceAges() {
            if (dead || tab !== 'devices') return;
            for (const [index,row] of Array.from(content.querySelectorAll('.ruview-node')).entries()) {
                const node=nodes[index]; if (!node) continue;
                const age=Number.isFinite(node.csi_last_seen_ms)&&node.csi_last_seen_ms>=0?Math.round(node.csi_last_seen_ms+performance.now()-nodesAt):null;
                row.querySelector('[data-age]').textContent=age==null?'—':age+' ms';
                const fresh=!nodesError&&status.source==='esp32'&&!status.simulated&&node.csi_status==='active'&&node.csi_sequence!=null&&age!=null&&age<=10000;
                row.querySelector('[data-fresh]').textContent=t(fresh?'received':node.csi_status==='offline'?'device_offline':'stale');
            }
        }
        function devices() {
            if (!content.querySelector('[data-node-list]')) content.innerHTML='<h2>'+esc(t('devices'))+'</h2><p class="ruview-note">'+esc(t('devices_known'))+'</p><p role="status" class="ruview-feedback"></p><div data-node-list></div><button data-action="flash">'+esc(t('configure'))+'</button>';
            content.querySelector('[data-node-list]').innerHTML=nodes.map(node=>'<article class="ruview-node"><strong>'+esc(t('node'))+' '+esc(node.node_id??node.id)+'</strong><div><span>CSI</span><span data-fresh></span></div><div><span>RSSI</span>'+esc(node.rssi_dbm??'—')+' dBm</div><div><span>'+esc(t('last_measurement'))+'</span><span data-age></span></div></article>').join('');
            content.querySelector('[role="status"]').textContent=nodesError?t('devices_stale'):!nodesLoaded?t('working'):!nodes.length?t('devices_empty'):'';
            content.querySelector('[data-action="flash"]').disabled=disabled('flash');
            deviceAges();
        }
        async function refreshDevices(epoch) {
            const generation=statusEpoch;
            try {
                // Wait for both requests even if one fails, so the next poll cannot overlap its sibling.
                const results=await Promise.allSettled([request('status'),request('nodes')]);
                const failed=results.find(result=>result.status==='rejected');if(failed)throw failed.reason;
                const [next,data]=results.map(result=>result.value);
                if (dead||epoch!==viewEpoch||tab!=='devices'||generation!==statusEpoch) return;
                const incoming=Array.isArray(data)?data:data.nodes;
                if (!Array.isArray(incoming)) throw Error('Invalid nodes response');
                const now=performance.now(),previous=new Map(nodesScope===data.scope?nodes.map(node=>[node.node_id,node]):[]);
                nodes=incoming.map(node=>{
                    const old=previous.get(node.node_id);
                    if(node.csi_status!=='offline'||!old)return node;
                    return {...node,rssi_dbm:old.rssi_dbm,csi_last_seen_ms:Number.isFinite(old.csi_last_seen_ms)?old.csi_last_seen_ms+now-nodesAt:null};
                });
                status=next;nodesAt=now;nodesScope=data.scope;nodesError=data.available===false?'unavailable':'';nodesLoaded=true;
            } catch(error) {
                if(dead||epoch!==viewEpoch||tab!=='devices'||generation!==statusEpoch) return;
                nodesError=error.message;
            }
            if(!dead&&epoch===viewEpoch&&tab==='devices') devices();
        }
        async function show() {
            const epoch=++viewEpoch,generation=++statusEpoch;
            host.querySelectorAll('[data-tab]').forEach(button=>button.setAttribute('aria-pressed',String(button.dataset.tab===tab)));
            if(tab==='devices') { devices();if(polling)return;polling=true;try{await refreshDevices(epoch);}finally{polling=false;}return; }
            try {
                const next=await request('status');
                if(dead||epoch!==viewEpoch||generation!==statusEpoch)return;
                status=next;
                if(tab==='overview'){summary();return;}
                if(!status.enabled){content.textContent=t('disabled');return;}
                content.replaceChildren();
                if(tab==='pose-fusion.html'){const note=document.createElement('p');note.className='ruview-note';note.setAttribute('role','note');note.textContent=t('fusion_notice');content.append(note);}
                const frame=document.createElement('iframe');frame.title='RuView · '+tab;frame.sandbox='allow-scripts allow-same-origin';frame.allow="serial 'none'; camera 'none'; microphone 'none'";frame.src=base+'ui/'+tab;content.append(frame);
            }catch(error){if(!dead&&epoch===viewEpoch)content.textContent=errorText(error.message);}
        }
        host.addEventListener('click',async event=>{
            const button=event.target.closest('button');if(!button||button.disabled)return;
            if(button.dataset.tab){tab=button.dataset.tab;content.replaceChildren();await show();return;}
            const action=button.dataset.action;if(!action||disabled(action))return;
            pending.add(action);actionError='';button.disabled=true;
            const generation=++statusEpoch;
            try {
                if(action==='flash'){const {openWizard}=await import(window.AuraLazyAssets.versionedURL('/js/ruview/wizard.mjs'));wizardClose=await openWizard({base,t:ctx.t});if(dead)wizardClose?.();}
                else{const next=await request(action,'POST');if(!dead&&generation===statusEpoch)status=next;}
            }catch(error){
                if(!dead&&generation===statusEpoch){actionError=error.message;if(action==='runtime/stop')status={...status,ready:false,fresh:false,data_state:'disconnected'};}
            }finally{
                pending.delete(action);
                if(!dead){if(tab==='overview')summary();else if(tab==='devices')devices();}
            }
        },{signal:controller.signal});
        const timer=setInterval(async()=>{
            if(dead||document.hidden||!host.checkVisibility())return;
            if(polling||pending.size||!['overview','devices'].includes(tab))return;
            polling=true;const epoch=viewEpoch,generation=statusEpoch;
            try {
                if(tab==='devices')await refreshDevices(epoch);
                else{const next=await request('status');if(!dead&&tab==='overview'&&epoch===viewEpoch&&generation===statusEpoch){status=next;summary();}}
            }catch{
                if(!dead&&tab==='overview'&&epoch===viewEpoch&&generation===statusEpoch){status={...status,ready:false,fresh:false,data_state:'disconnected'};summary();}
            }finally{polling=false;}
        },3000);
        const ageTimer=setInterval(()=>{if(!dead&&!document.hidden&&host.checkVisibility())deviceAges();},1000);
        const closed=()=>{dead=true;++viewEpoch;++statusEpoch;clearInterval(timer);clearInterval(ageTimer);controller.abort();wizardClose?.();host.querySelectorAll('iframe').forEach(frame=>frame.remove());};
        instances.set(id,closed);show();
    }
    window.RuViewApp={render,dispose};
})();
