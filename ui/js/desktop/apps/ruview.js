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
        host.innerHTML = '<div class="ruview-app"><nav class="ruview-toolbar" aria-label="' + esc(t('nav_label')) + '"><strong>RuView</strong>' +
            [['overview','overview'],['devices','devices'],['index.html','dashboard'],['observatory.html','observatory'],['pose-fusion.html','pose_fusion']].map(([name,label]) => '<button data-tab="'+name+'">'+esc(t(label))+'</button>').join('') +
            '<a href="/config#ruview" target="_blank" rel="noopener">'+esc(t('settings'))+'</a></nav><div class="ruview-content"></div></div>';
        const content = host.querySelector('.ruview-content');
        async function request(route, method = 'GET') {
            const response = await fetch(base + route, {method, signal: controller.signal});
            const body = await response.json().catch(() => ({}));
            if (!response.ok) throw Error(errorText(body.error || body.message || ('HTTP ' + response.status)));
            return body;
        }
        function formatAge(ms) {
            if (ms == null || !Number.isFinite(ms) || ms < 0) return '—';
            if (ms < 1000) return t('age_ms').replace('{n}', String(Math.round(ms)));
            if (ms < 60000) {
                const seconds = ms / 1000;
                const rounded = ms < 10000 && Math.round(seconds * 10) % 10 ? seconds.toFixed(1) : String(Math.round(seconds));
                return t('age_s').replace('{n}', rounded);
            }
            return t('age_min').replace('{n}', String(Math.round(ms / 60000)));
        }
        function nodeAge(node) {
            return Number.isFinite(node?.csi_last_seen_ms) && node.csi_last_seen_ms >= 0 ? Math.round(node.csi_last_seen_ms + performance.now() - nodesAt) : null;
        }
        function reason(action) {
            if (!status.enabled) return 'enable_first';
            if (pending.has(action)) return 'working';
            if (action !== 'test' && action !== 'runtime/stop' && status.readonly) return 'write_first';
            if (String(action).startsWith('runtime/') && status.mode === 'external') return 'external_runtime';
            if (action === 'runtime/install' && status.mode !== 'native') return 'install_native';
            return '';
        }
        function disabled(action) { return reason(action) !== ''; }
        function receptionItem(name, state, detail) {
            const jump = state === 'attention' && (name === 'board' || name === 'csi')
                ? '<button type="button" data-open-tab="devices">' + esc(t('show_devices')) + '</button>' : '';
            const settings = state === 'attention' && !jump
                ? '<a href="/config#ruview" target="_blank" rel="noopener">' + esc(t('settings')) + '</a>' : '';
            return '<li data-state="' + state + '"><span>' + esc(t('check_' + name)) + '</span><strong>' + esc(detail) + '</strong>' + jump + settings + '</li>';
        }
        function checklist() {
            const enabled = status.enabled === true;
            const source = status.source || '';
            let runtime = 'unchecked', runtimeText = t('check_unchecked');
            if (enabled && status.ready) { runtime = 'done'; runtimeText = t('check_ok'); }
            else if (enabled && (status.data_state === 'disconnected' || (status.error && !status.ready && status.data_state !== 'starting' && status.data_state !== 'unchecked'))) {
                runtime = 'attention'; runtimeText = t('check_attention');
            }
            let udp = 'unchecked', udpText = t('check_unchecked');
            if (enabled && status.udp_scope === 'external') { udp = 'neutral'; udpText = t('udp_operator'); }
            else if (enabled && status.udp_scope === 'managed' && status.udp_ready) { udp = 'done'; udpText = t('check_ok'); }
            else if (enabled && status.udp_scope === 'managed') { udp = 'attention'; udpText = t('check_attention'); }
            const skip = source === 'wifi' || source === 'simulated';
            let receiver = 'unchecked', receiverText = t('check_unchecked');
            if (skip) { receiver = 'neutral'; receiverText = t('check_not_needed'); }
            else if (source === 'esp32' && status.receiver_configured) { receiver = 'done'; receiverText = t('check_ok'); }
            else if (source === 'esp32' && enabled) { receiver = 'attention'; receiverText = t('check_attention'); }
            let board = 'unchecked', boardText = t('check_unchecked');
            let csi = 'neutral', csiText = t('check_not_needed');
            if (!skip) {
                csi = 'unchecked'; csiText = t('check_unchecked');
                if (nodesError) { board = 'attention'; boardText = t('check_attention'); csi = 'attention'; csiText = t('check_attention'); }
                else if (nodesLoaded && nodes.length) {
                    board = 'done'; boardText = t('check_ok');
                    const fresh = nodes.some(node => node.csi_status === 'active' && node.csi_sequence != null && source === 'esp32' && !status.simulated && nodeAge(node) != null && nodeAge(node) <= 10000);
                    csi = fresh ? 'done' : 'attention';
                    csiText = fresh ? t('check_ok') : t('check_attention');
                }
            } else if (nodesError) { board = 'attention'; boardText = t('check_attention'); }
            else if (nodesLoaded && nodes.length) { board = 'done'; boardText = t('check_ok'); }
            return '<ol class="ruview-check">' + [
                receptionItem('runtime', runtime, runtimeText),
                receptionItem('udp', udp, udpText),
                receptionItem('receiver', receiver, receiverText),
                receptionItem('board', board, boardText),
                receptionItem('csi', csi, csiText)
            ].join('') + '</ol>';
        }
        function summary() {
            const focused = content.contains(document.activeElement) ? document.activeElement.dataset.action : null;
            const card = (title, value, detail) => '<article>'+esc(t(title))+'<strong>'+esc(value)+'</strong>'+esc(detail)+'</article>';
            const session = ['unknown','started','stopped'].includes(status.recording_session) ? status.recording_session : 'unknown';
            const actions = [['test','test'],['runtime/install','install'],['runtime/start','start'],['runtime/stop','stop'],['flash','flash']];
            const priority = ['enable_first','working','write_first','external_runtime','install_native'];
            const reasons = actions.map(([action]) => reason(action)).filter(Boolean);
            const visibleReason = priority.find(item => reasons.includes(item)) || '';
            const notStarted = status.enabled && !status.readonly && (status.mode === 'docker' || status.mode === 'native') && !status.ready && status.data_state === 'unchecked';
            content.innerHTML = '<h2>'+esc(t('overview'))+'</h2><div class="ruview-summary">' +
                card('service',t(!status.enabled?'disabled':status.ready?'ready':status.data_state==='starting'?'working':status.data_state==='unchecked'?'unchecked':'unavailable'),status.mode?t(status.mode):'—') +
                card('sensing',t(status.fresh?'received':status.simulated?'demo':status.data_state==='live_unverified'?'unavailable':status.data_state||'unchecked'),status.signal||'—') +
                card('source',status.source?t(status.source):'—',status.observed_at?new Date(status.observed_at).toLocaleString():'—') +
                card('artifact', t(status.revision_verified ? 'revision_verified' : 'revision_unverified'), status.revision || '—') +
                card('storage_card', t(status.storage_limit ? 'storage_full' : 'storage_ok'), '') +
                card('recording', t('recording_' + session), status.recording_ack === 'unconfirmed' ? t('recording_unconfirmed') : t('recording_note')) +
                '</div><p class="ruview-note">'+esc(t('quality'))+'</p>'+(status.readonly?'<p>'+esc(t('readonly_notice'))+'</p>':'') +
                (notStarted ? '<p class="ruview-note">' + esc(t('session_not_started')) + '</p>' : '') +
                checklist() +
                '<div class="ruview-toolbar">' +
                actions.map(([action,label]) => '<button data-action="'+action+'" aria-busy="'+pending.has(action)+'" ' + (reason(action) ? 'aria-describedby="ruview-action-reason" ' : '') + (disabled(action)?'disabled':'')+'>'+esc(t(label))+'</button>').join('') +
                '</div><p id="ruview-action-reason" class="ruview-note">' + esc(visibleReason ? t(visibleReason) : '') + '</p><p class="ruview-feedback" role="status">'+esc(errorText(actionError||status.error))+'</p>';
            if (focused) Array.from(content.querySelectorAll('[data-action]')).find(button => button.dataset.action === focused && !button.disabled)?.focus({preventScroll:true});
        }
        function deviceAges() {
            if (dead || tab !== 'devices') return;
            for (const [index,row] of Array.from(content.querySelectorAll('.ruview-node')).entries()) {
                const node=nodes[index]; if (!node) continue;
                const age=nodeAge(node);
                const slot=row.querySelector('[data-age]');
                slot.textContent=formatAge(age);
                if (age == null) delete slot.dataset.ageMs; else slot.dataset.ageMs=String(age);
                const fresh=!nodesError&&status.source==='esp32'&&!status.simulated&&node.csi_status==='active'&&node.csi_sequence!=null&&age!=null&&age<=10000;
                row.querySelector('[data-fresh]').textContent=t(fresh?'received':node.csi_status==='offline'?'device_offline':'stale');
            }
        }
        function devices() {
            if (!content.querySelector('[data-node-list]')) content.innerHTML='<h2>'+esc(t('devices'))+'</h2><p class="ruview-note">'+esc(t('devices_known'))+'</p><p role="status" class="ruview-feedback"></p><div data-node-list></div><button data-action="flash">'+esc(t('configure'))+'</button>';
            content.querySelector('[data-node-list]').innerHTML=nodes.map(node=>{
                const retained=node.csi_status==='offline'&&node.rssi_dbm!=null;
                const rssi=(node.rssi_dbm??'—')+' dBm';
                return '<article class="ruview-node"><strong>'+esc(t('node'))+' '+esc(node.node_id??node.id)+'</strong><div><span>'+esc(t('signal_csi'))+'</span><span data-fresh></span></div><div><span>'+esc(t('signal_rssi'))+'</span>'+(retained?esc(t('rssi_retained'))+' ':'')+esc(rssi)+'</div><div><span>'+esc(t('last_measurement'))+'</span><span data-age></span></div><button type="button" data-node="'+esc(node.node_id??node.id)+'"'+(disabled('flash')?' disabled':'')+'>'+esc(t('configure_node'))+'</button></article>';
            }).join('');
            content.querySelector('[role="status"]').textContent=nodesError?t('devices_stale'):!nodesLoaded?t('working'):!nodes.length?t('devices_empty'):'';
            content.querySelector('[data-action="flash"]').disabled=disabled('flash');
            deviceAges();
        }
        function applyNodes(data) {
            const incoming=Array.isArray(data)?data:data.nodes;
            if (!Array.isArray(incoming)) throw Error('Invalid nodes response');
            const previous=new Map(nodesScope===data.scope?nodes.map(node=>[node.node_id,node]):[]);
            nodes=incoming.map(node=>{
                const old=previous.get(node.node_id);
                if(node.csi_status!=='offline'||!old||node.rssi_dbm!=null||old.rssi_dbm==null)return node;
                return {...node,rssi_dbm:old.rssi_dbm};
            });
            nodesAt=performance.now();nodesScope=data.scope||'';nodesError=data.available===false?'unavailable':'';nodesLoaded=true;
        }
        async function refreshSnapshot(epoch) {
            const generation=statusEpoch, view=tab;
            if (view!=='overview'&&view!=='devices') return;
            try {
                const results=await Promise.allSettled([request('status'),request('nodes')]);
                if (dead||epoch!==viewEpoch||tab!==view||generation!==statusEpoch) return;
                if (results[0].status==='fulfilled') status=results[0].value;
                else status={...status,ready:false,fresh:false,data_state:'disconnected'};
                if (results[0].status==='fulfilled' && results[1].status==='fulfilled') applyNodes(results[1].value);
                else nodesError='unavailable';
            } catch (error) {
                if (dead||epoch!==viewEpoch||tab!==view||generation!==statusEpoch) return;
                nodesError=error.message;
                if (view==='overview') status={...status,ready:false,fresh:false,data_state:'disconnected'};
            }
            if (!dead&&epoch===viewEpoch&&tab===view) { if (view==='devices') devices(); else summary(); }
        }
        function paint() {
            if (tab==='devices') devices();
            else if (tab==='overview') summary();
        }
        async function show() {
            const epoch=++viewEpoch,generation=++statusEpoch;
            host.querySelectorAll('[data-tab]').forEach(button=>button.setAttribute('aria-pressed',String(button.dataset.tab===tab)));
            if (tab==='overview'||tab==='devices') {
                if (polling) { paint(); return; }
                polling=true;try{await refreshSnapshot(epoch);}finally{polling=false;}return;
            }
            try {
                const next=await request('status');
                if(dead||epoch!==viewEpoch||generation!==statusEpoch)return;
                status=next;
                if(!status.enabled){content.textContent=t('disabled');return;}
                content.replaceChildren();
                if(tab==='pose-fusion.html'){const note=document.createElement('p');note.className='ruview-note';note.setAttribute('role','note');note.textContent=t('fusion_notice');content.append(note);}
                const frame=document.createElement('iframe');frame.title='RuView · '+tab;frame.sandbox='allow-scripts allow-same-origin';frame.allow="serial 'none'; camera 'none'; microphone 'none'";frame.src=base+'ui/'+tab;content.append(frame);
            }catch(error){if(!dead&&epoch===viewEpoch)content.textContent=errorText(error.message);}
        }
        host.addEventListener('click',async event=>{
            const button=event.target.closest('button');if(!button||button.disabled)return;
            const nextTab=button.dataset.tab||button.dataset.openTab;
            if(nextTab){tab=nextTab;content.replaceChildren();await show();return;}
            const nodeID=Number(button.dataset.node);
            if(Number.isInteger(nodeID)){
                const {openWizard}=await import(window.AuraLazyAssets.versionedURL('/js/ruview/wizard.mjs'));
                wizardClose=await openWizard({base,t:ctx.t,nodeID});if(dead)wizardClose?.();return;
            }
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
            polling=true;const epoch=viewEpoch;
            try { await refreshSnapshot(epoch); }
            finally{polling=false;}
        },3000);
        const ageTimer=setInterval(()=>{if(!dead&&!document.hidden&&host.checkVisibility())deviceAges();},1000);
        const closed=()=>{dead=true;++viewEpoch;++statusEpoch;clearInterval(timer);clearInterval(ageTimer);controller.abort();wizardClose?.();host.querySelectorAll('iframe').forEach(frame=>frame.remove());};
        instances.set(id,closed);show();
    }
    window.RuViewApp={render,dispose};
})();
