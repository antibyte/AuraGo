(function () {
    'use strict';
    const instances = new Map(), base = '/api/desktop/integrations/ruview/', warned = new Set();
    function dispose(id) { instances.get(id)?.(); instances.delete(id); }
    function render(host, id, ctx) {
        dispose(id);
        const controller = new AbortController(), esc = ctx.esc;
        // A missing key shows its own name, never a misleading label, and is reported once.
        const t = key => {
            const full = 'ruview.' + key, value = ctx.t(full);
            if (value === full && !warned.has(full)) { warned.add(full); console.warn('RuView: missing translation ' + full); }
            return value;
        };
        // Dynamic values go through literal maps so the static i18n lint sees every key.
        const pick = (map, key, fallback) => typeof key === 'string' && Object.hasOwn(map, key) ? map[key]() : fallback();
        const SOURCE = {esp32: () => t('esp32'), wifi: () => t('wifi'), simulated: () => t('simulated')};
        const MODE = {docker: () => t('docker'), external: () => t('external'), native: () => t('native')};
        const RECORDING = {unknown: () => t('recording_unknown'), started: () => t('recording_started'), stopped: () => t('recording_stopped')};
        const DATA_STATE = {
            unchecked: () => t('unchecked'), disabled: () => t('disabled'), starting: () => t('starting'), stopping: () => t('stopping'),
            stopped: () => t('stopped_state'), disconnected: () => t('disconnected'), stale: () => t('stale'), synthetic: () => t('demo'),
            live_unverified: () => t('unavailable'), live: () => t('unavailable')
        };
        const REASON = {
            status_unavailable: () => t('status_unavailable'), enable_first: () => t('enable_first'), working: () => t('working'),
            write_first: () => t('write_first'), external_runtime: () => t('external_runtime'), install_native: () => t('install_native')
        };
        const ERROR = {
            source_mismatch: () => t('source_mismatch'), unexpected_simulation: () => t('unexpected_simulation'),
            stop_failed: () => t('stop_failed'), process_exited: () => t('process_exited')
        };
        const errorText = message => message ? pick(ERROR, message, () => message) : '';
        let dead = false, tab = 'overview', status = {}, statusLoaded = false, statusFailed = false, wizardClose = null, relayDetach = null, actionError = '';
        let viewEpoch = 0, statusEpoch = 0, polling = false;
        let nodes = [], nodesAt = 0, nodesError = '', nodesNote = '', nodesLoaded = false, nodesScope = '';
        const pending = new Set();
        // The last known service data is kept on a later failure, but liveness claims are withdrawn.
        const disconnected = current => ({...current, ready: false, fresh: false, waiting_for_frames: false, udp_ready: false, data_state: 'disconnected'});
        host.innerHTML = '<div class="ruview-app"><nav class="ruview-toolbar" aria-label="' + esc(t('nav_label')) + '"><strong>RuView</strong>' +
            [['overview',t('overview')],['devices',t('devices')],['index.html',t('dashboard')],['observatory.html',t('observatory')],['pose-fusion.html',t('pose_fusion')]].map(([name,label]) => '<button data-tab="'+name+'">'+esc(label)+'</button>').join('') +
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
            // An unknown status must not read as "disabled": enabling is only the answer once it is known.
            if (!statusLoaded) return 'status_unavailable';
            if (!status.enabled) return 'enable_first';
            if (pending.has(action)) return 'working';
            if (action !== 'test' && action !== 'runtime/stop' && status.readonly) return 'write_first';
            if (String(action).startsWith('runtime/') && status.mode === 'external') return 'external_runtime';
            if (action === 'runtime/install' && status.mode !== 'native') return 'install_native';
            return '';
        }
        function disabled(action) { return reason(action) !== ''; }
        function receptionItem(name, label, state, detail) {
            const jump = state === 'attention' && (name === 'board' || name === 'csi')
                ? '<button type="button" data-open-tab="devices">' + esc(t('show_devices')) + '</button>' : '';
            const settings = state === 'attention' && !jump
                ? '<a href="/config#ruview" target="_blank" rel="noopener">' + esc(t('settings')) + '</a>' : '';
            return '<li data-state="' + state + '"><span>' + esc(label) + '</span><strong>' + esc(detail) + '</strong>' + jump + settings + '</li>';
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
            let csi = skip ? 'neutral' : 'unchecked', csiText = skip ? t('check_not_needed') : t('check_unchecked');
            if (statusLoaded && status.enabled === false) {
                // A disabled integration answers /nodes with 403 by design: that is not a fault, nothing was asked.
            } else if (nodesError) {
                board = 'attention'; boardText = t('check_attention');
                if (!skip) { csi = 'attention'; csiText = t('check_attention'); }
            } else if (nodesNote === 'not_applicable') {
                // The source has no board list: nothing to check, nothing to fix.
                board = 'neutral'; boardText = t('check_not_needed'); csi = 'neutral'; csiText = t('check_not_needed');
            } else if (nodesNote === 'not_running') {
                // Nothing was asked of a service that is not running; saved boards say nothing about reception.
            } else if (nodesLoaded && nodes.length) {
                board = 'done'; boardText = t('check_ok');
                if (!skip) {
                    const fresh = nodes.some(node => node.csi_status === 'active' && node.csi_sequence != null && source === 'esp32' && !status.simulated && nodeAge(node) != null && nodeAge(node) <= 10000);
                    csi = fresh ? 'done' : 'attention';
                    csiText = fresh ? t('check_ok') : t('check_attention');
                }
            }
            return '<ol class="ruview-check">' + [
                receptionItem('runtime', t('check_runtime'), runtime, runtimeText),
                receptionItem('udp', t('check_udp'), udp, udpText),
                receptionItem('receiver', t('check_receiver'), receiver, receiverText),
                receptionItem('board', t('check_board'), board, boardText),
                receptionItem('csi', t('check_csi'), csi, csiText)
            ].join('') + '</ol>';
        }
        function summary() {
            const focused = content.contains(document.activeElement) ? document.activeElement.dataset.action : null;
            const card = (title, value, detail) => '<article>'+esc(title)+'<strong>'+esc(value)+'</strong>'+esc(detail)+'</article>';
            const session = ['unknown','started','stopped'].includes(status.recording_session) ? status.recording_session : 'unknown';
            const actions = [['test',t('test')],['runtime/install',t('install')],['runtime/start',t('start')],['runtime/stop',t('stop')],['flash',t('flash')]];
            const priority = ['status_unavailable','enable_first','working','write_first','external_runtime','install_native'];
            const reasons = actions.map(([action]) => reason(action)).filter(Boolean);
            const visibleReason = priority.find(item => reasons.includes(item)) || '';
            const notStarted = status.enabled && !status.readonly && (status.mode === 'docker' || status.mode === 'native') && !status.ready && status.data_state === 'unchecked';
            // Before the first answer nothing is known: never claim "disabled" or a data state.
            const serviceText = !statusLoaded ? (statusFailed ? t('status_unavailable') : t('working'))
                : !status.enabled ? t('disabled')
                : status.ready ? t('ready')
                : status.data_state === 'starting' ? t('working')
                : status.data_state === 'unchecked' ? t('unchecked')
                : t('unavailable');
            const sensingText = !statusLoaded ? (statusFailed ? t('status_unavailable') : t('unchecked'))
                : status.fresh ? t('received')
                : status.simulated ? t('demo')
                : status.waiting_for_frames ? t('waiting_for_frames')
                : pick(DATA_STATE, status.data_state, () => t('unchecked'));
            content.innerHTML = '<h2>'+esc(t('overview'))+'</h2><div class="ruview-summary">' +
                card(t('service'),serviceText,status.mode?pick(MODE,status.mode,()=>status.mode):'—') +
                card(t('sensing'),sensingText,status.signal||'—') +
                card(t('source'),status.source?pick(SOURCE,status.source,()=>status.source):'—',status.observed_at?new Date(status.observed_at).toLocaleString():'—') +
                card(t('artifact'), status.revision_verified ? t('revision_verified') : t('revision_unverified'), status.revision || '—') +
                card(t('storage_card'), status.storage_limit ? t('storage_full') : t('storage_ok'), '') +
                card(t('recording'), pick(RECORDING, session, () => t('recording_unknown')), status.recording_ack === 'unconfirmed' ? t('recording_unconfirmed') : t('recording_note')) +
                '</div><p class="ruview-note">'+esc(t('quality'))+'</p>'+(status.readonly?'<p>'+esc(t('readonly_notice'))+'</p>':'') +
                (notStarted ? '<p class="ruview-note">' + esc(t('session_not_started')) + '</p>' : '') +
                checklist() +
                '<div class="ruview-toolbar">' +
                actions.map(([action,label]) => '<button data-action="'+action+'" aria-busy="'+pending.has(action)+'" ' + (reason(action) ? 'aria-describedby="ruview-action-reason" ' : '') + (disabled(action)?'disabled':'')+'>'+esc(label)+'</button>').join('') +
                '</div><p id="ruview-action-reason" class="ruview-note">' + esc(pick(REASON, visibleReason, () => '')) + '</p><p class="ruview-feedback" role="status">'+esc(errorText(actionError||status.error))+'</p>';
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
                row.querySelector('[data-fresh]').textContent=fresh?t('received'):node.csi_status==='offline'?t('device_offline'):t('stale');
            }
        }
        function devices() {
            if (!content.querySelector('[data-node-list]')) content.innerHTML='<h2>'+esc(t('devices'))+'</h2><p class="ruview-note">'+esc(t('devices_known'))+'</p><p role="status" class="ruview-feedback"></p><div data-node-list></div><button data-action="flash">'+esc(t('configure'))+'</button>';
            content.querySelector('[data-node-list]').innerHTML=nodes.map(node=>{
                const retained=node.csi_status==='offline'&&node.rssi_dbm!=null;
                const rssi=(node.rssi_dbm??'—')+' dBm';
                return '<article class="ruview-node"><strong>'+esc(t('node'))+' '+esc(node.node_id??node.id)+'</strong><div><span>'+esc(t('signal_csi'))+'</span><span data-fresh></span></div><div><span>'+esc(t('signal_rssi'))+'</span>'+(retained?esc(t('rssi_retained'))+' ':'')+esc(rssi)+'</div><div><span>'+esc(t('last_measurement'))+'</span><span data-age></span></div><button type="button" data-node="'+esc(node.node_id??node.id)+'"'+(disabled('flash')?' disabled':'')+'>'+esc(t('configure_node'))+'</button></article>';
            }).join('');
            content.querySelector('[role="status"]').textContent=statusLoaded&&status.enabled===false?t('disabled'):nodesError?t('devices_stale'):!nodesLoaded?t('working'):nodesNote==='not_applicable'?t('devices_not_applicable'):nodesNote==='not_running'?t('devices_not_running'):!nodes.length?t('devices_empty'):'';
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
            // live_state says why the live list is absent; an older backend only sends `available`.
            const known=['ok','not_applicable','not_running','unreachable','invalid'];
            const live=known.includes(data.live_state)?data.live_state:data.available===false?'unreachable':'ok';
            nodesAt=performance.now();nodesScope=data.scope||'';nodesLoaded=true;
            nodesNote=live==='not_applicable'||live==='not_running'?live:'';
            nodesError=live==='unreachable'||live==='invalid'?'unavailable':'';
        }
        async function refreshSnapshot(epoch) {
            const generation=statusEpoch, view=tab;
            if (view!=='overview'&&view!=='devices') return;
            try {
                const results=await Promise.allSettled([request('status'),request('nodes')]);
                if (dead||epoch!==viewEpoch||tab!==view||generation!==statusEpoch) return;
                if (results[0].status==='fulfilled') { status=results[0].value; statusLoaded=true; statusFailed=false; }
                else { statusFailed=true; status=disconnected(status); }
                if (results[0].status==='fulfilled' && results[1].status==='fulfilled') applyNodes(results[1].value);
                else nodesError='unavailable';
            } catch (error) {
                if (dead||epoch!==viewEpoch||tab!==view||generation!==statusEpoch) return;
                // Only the board list failed (a malformed /nodes body); /status was fine, so the service is not marked disconnected.
                nodesError=error.message||'unavailable';
            }
            if (!dead&&epoch===viewEpoch&&tab===view) { if (view==='devices') devices(); else summary(); }
        }
        function paint() {
            if (tab==='devices') devices();
            else if (tab==='overview') summary();
        }
        function dropRelay(){relayDetach?.();relayDetach=null;}
        async function show() {
            const epoch=++viewEpoch,generation=++statusEpoch;
            host.querySelectorAll('[data-tab]').forEach(button=>button.setAttribute('aria-pressed',String(button.dataset.tab===tab)));
            dropRelay();
            if (tab==='overview'||tab==='devices') {
                if (polling) { paint(); return; }
                polling=true;try{await refreshSnapshot(epoch);}finally{polling=false;}return;
            }
            try {
                const next=await request('status');
                if(dead||epoch!==viewEpoch||generation!==statusEpoch)return;
                status=next;statusLoaded=true;statusFailed=false;
                if(!status.enabled){content.textContent=t('disabled');return;}
                content.replaceChildren();
                if(tab==='pose-fusion.html'){const note=document.createElement('p');note.className='ruview-note';note.setAttribute('role','note');note.textContent=t('fusion_notice');content.append(note);}
                const {attachRuViewRelay,tourFrameName}=await import(window.AuraLazyAssets.versionedURL('/js/ruview/relay.mjs'));
                if(dead||epoch!==viewEpoch)return;
                const frame=document.createElement('iframe');frame.title='RuView · '+tab;frame.sandbox='allow-scripts';frame.allow="serial 'none'; camera 'none'; microphone 'none'";
                try{frame.name=localStorage.getItem('aurago.ruview.onboarding.done')==='true'?tourFrameName:'';}catch{frame.name='';}
                content.append(frame);relayDetach=attachRuViewRelay(frame,base);frame.src=base+'ui/'+tab;
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
                else{const next=await request(action,'POST');if(!dead&&generation===statusEpoch){status=next;statusLoaded=true;statusFailed=false;}}
            }catch(error){
                if(!dead&&generation===statusEpoch){actionError=error.message;if(action==='runtime/stop')status=disconnected(status);}
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
        const closed=()=>{dead=true;++viewEpoch;++statusEpoch;clearInterval(timer);clearInterval(ageTimer);controller.abort();dropRelay();wizardClose?.();host.querySelectorAll('iframe').forEach(frame=>frame.remove());};
        instances.set(id,closed);show();
    }
    window.RuViewApp={render,dispose};
})();
