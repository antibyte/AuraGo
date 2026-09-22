(function () {
    'use strict';
    const R = window.RTLSDRRuntime, instances = new Map();
    const defaults = { frequency_hz: 100e6, mode: 'wfm', bandwidth_hz: 180000, gain_db: 0, agc: true, ppm: 0, squelch_db: -100, stereo: true };
    function dispose(id) { const s = instances.get(id); if (!s) return; s.disposed = true; s.abort.abort(); clearInterval(s.receiverTimer); clearTimeout(s.tuneTimer); s.unsub?.(); s.preview?.pause(); instances.delete(id); }
    function render(host, id, ctx) {
        dispose(id);
        const s = { host, id, ctx, abort: new AbortController(), tab: 'receive', tuning: { ...defaults }, receiver: {}, pending: false, disposed: false, data: null, editing: false };
        s.t = key => ctx.t('rtlSdr.' + key); s.esc = ctx.esc; instances.set(id, s);
        const t = key => ctx.esc(s.t(key));
        host.innerHTML = `<div class="sdr-app"><header class="sdr-header"><div class="sdr-wordmark"><span class="sdr-logo" aria-hidden="true">)))</span><div><strong>RTL-SDR</strong><small>${t('subtitle')}</small></div></div>${button(s,'show-setup','setup')}<div class="sdr-connection"><i data-sdr="lamp"></i><span data-sdr="connection">${t('stopped')}</span></div></header>
          <div class="sdr-notice" data-sdr="notice" role="status" hidden></div><div class="sdr-priority" data-sdr="priority" role="status" hidden></div>
          <nav class="sdr-tabs" aria-label="RTL-SDR">${['receive','recordings','schedules'].map(tab => `<button type="button" data-tab="${tab}" aria-pressed="${tab === 'receive'}">${t(tab)}</button>`).join('')}</nav>
          <main data-sdr="content"></main><footer class="sdr-footer"><span data-sdr="footer">${t('background_hint')}</span><span data-sdr="storage"></span></footer></div>`;
        s.q = key => host.querySelector('[data-sdr="' + key + '"]');
        host.addEventListener('click', e => { const b = e.target.closest('button'); if (!b || s.disposed) return; if (b.dataset.tab) { if(s.tab==='receive')readTuning(s); s.tab = b.dataset.tab; content(s); } if (b.dataset.action) action(s, b).catch(err => notice(s, err)); }, { signal: s.abort.signal });
        s.unsub = R.subscribe((value, error) => { if (s.disposed) return; s.data = value; if (!s.initialized && value) { s.tuning = { ...defaults, ...value.state.tuning }; s.initialized = true; content(s); } update(s); if (error) notice(s, Error(error)); });
        R.init(ctx).catch(err => notice(s, err)); content(s);
        s.receiverTimer = setInterval(() => receiver(s), 350);
    }
    function notice(s, error) { if (s.disposed) return; const key = String(error.message || error); const known = ['sdr_busy','sdr_quota_exceeded','sdr_read_only','sdr_device_missing','sdr_usb_permissions','sdr_docker_disabled','sdr_linux_required','sdr_image_unavailable','sdr_disabled','sdr_audio_unlock','sdr_service_user','sdr_local_docker_required','sdr_data_mount_required']; s.q('notice').textContent = s.t(known.includes(key) ? key : 'error'); s.q('notice').hidden = false; }
    function button(s, action, label, attrs = '') { return `<button type="button" data-action="${action}" ${attrs}>${s.esc(s.t(label))}</button>`; }
    function field(s, key, input) { return `<label>${s.esc(s.t(key))}${input}</label>`; }
    function value(s, name) { return s.q(name)?.value; }
    function content(s) {
        const t = key => s.esc(s.t(key)); s.preview?.pause(); s.preview = null; s.signature = '';
        s.host.querySelectorAll('[data-tab]').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.tab === s.tab)));
        if (s.tab === 'receive') {
            s.q('content').innerHTML = `<section class="sdr-deck"><div class="sdr-display"><div class="sdr-display-top"><span data-sdr="mode-label">WFM · STEREO</span><span data-sdr="level">— dB</span></div><div class="sdr-frequency" data-sdr="digits" role="group" aria-label="${t('frequency')}"></div><div class="sdr-station"><strong data-sdr="station">${t('ready')}</strong><span data-sdr="radiotext"></span></div><div class="sdr-meter"><i data-sdr="meter"></i></div></div><div class="sdr-tuning"><button type="button" class="sdr-knob" data-sdr="knob" role="slider" aria-label="${t('tuning')}" aria-valuemin="24" aria-valuemax="1766" aria-valuenow="100"><i></i></button><small>${t('tuning')}</small><div class="sdr-step">${button(s,'down','decrease')}${button(s,'up','increase')}</div></div></section>
              <div class="sdr-transport">${button(s,'play','listen','class="sdr-primary"')}${button(s,'stop','stop')}${button(s,'mute',R.muted?'unmute':'mute')}<label class="sdr-volume">${t('volume')}<input data-sdr="volume" aria-label="${t('volume')}" type="range" min="0" max="1" step=".01" value="${R.volumeValue}"></label>${button(s,'favorite','favorite')}</div>
              <div class="sdr-tune-row">${field(s,'frequency','<input data-sdr="frequency" type="number" step="0.001" min="0.1" max="2200" inputmode="decimal">')}<span class="sdr-unit">MHz</span>${button(s,'apply','tune')}${field(s,'mode','<select data-sdr="mode">'+['wfm','nfm','am','usb','lsb','dab'].map(m=>'<option value="'+m+'">'+(m==='dab'?'DAB+':m.toUpperCase())+'</option>').join('')+'</select>')}</div>
              <div class="sdr-scope"><canvas data-sdr="spectrum" height="210" role="slider" tabindex="0" aria-label="${t('spectrum')}" aria-valuemin="24" aria-valuemax="1766" aria-valuenow="100"></canvas><div class="sdr-scope-labels"><span data-sdr="low"></span><span>${t('spectrum_hint')}</span><span data-sdr="high"></span></div></div>
              <div class="sdr-station-toolbar"><h3>${t('stations')}</h3>${button(s,'scan','scan')}${button(s,'cancel-scan','cancel')}</div><div class="sdr-stations" data-sdr="stations"></div>
              <details class="sdr-settings"><summary>${t('advanced')}</summary><div class="sdr-fields">${field(s,'bandwidth','<input data-sdr="bandwidth" type="number" min="1000" max="1536000" step="100">')}${field(s,'gain','<select data-sdr="gain"></select>')}${field(s,'agc','<input data-sdr="agc" type="checkbox">')}${field(s,'ppm','<input data-sdr="ppm" type="number" min="-200" max="200">')}${field(s,'squelch','<input data-sdr="squelch" type="range" min="-140" max="0">')}${field(s,'stereo','<input data-sdr="stereo" type="checkbox">')}</div>${button(s,'apply','apply')}<div class="sdr-setup" data-sdr="setup"></div></details>`;
            const on = (name, type, fn) => s.q(name).addEventListener(type, fn, { signal: s.abort.signal });
            on('volume','input',e=>R.volume(e.target.value)); on('frequency','keydown',e=>{if(e.key==='Enter')action(s,{dataset:{action:'apply'}}).catch(err=>notice(s,err));});
            on('frequency','change',()=>{readTuning(s);controls(s);retune(s);});
            on('mode','change',e=>{readTuning(s);s.tuning.bandwidth_hz={wfm:180000,nfm:12500,am:9000,usb:2800,lsb:2800,dab:1536000}[e.target.value];controls(s);update(s);retune(s);});
            for(const name of ['bandwidth','gain','agc','ppm','squelch','stereo'])on(name,'change',()=>{readTuning(s);controls(s);retune(s);});
            on('knob','wheel',e=>{e.preventDefault();step(s,e.deltaY<0?1:-1);});
            on('knob','keydown',e=>{if(['ArrowUp','ArrowRight','ArrowDown','ArrowLeft','Home','End'].includes(e.key)){e.preventDefault();if(e.key==='Home')s.tuning.frequency_hz=s.receiver.minimum_hz||24e6;else if(e.key==='End')s.tuning.frequency_hz=s.receiver.maximum_hz||1766e6;else step(s,['ArrowUp','ArrowRight'].includes(e.key)?1:-1);controls(s);retune(s);}});
            let drag; on('knob','pointerdown',e=>{drag={y:e.clientY,f:s.tuning.frequency_hz};e.target.setPointerCapture(e.pointerId);}); on('knob','pointermove',e=>{if(!drag)return;s.tuning.frequency_hz=clamp(s,drag.f+Math.round((drag.y-e.clientY)/3)*increment(s));controls(s);}); on('knob','pointerup',()=>{if(drag){drag=null;retune(s);}}); on('knob','pointercancel',()=>{drag=null;});
            on('spectrum','click',e=>{if(s.tuning.mode==='dab'||s.data?.state?.active_job||s.data?.state?.read_only)return;const r=e.currentTarget.getBoundingClientRect(),span=s.receiver.span_hz||2400000,center=s.receiver.center_hz||s.tuning.frequency_hz;s.tuning.frequency_hz=clamp(s,Math.round((center+(e.clientX-r.left)/r.width*span-span/2)/increment(s))*increment(s));controls(s);retune(s);});
            on('spectrum','keydown',e=>{if(e.key==='ArrowLeft'||e.key==='ArrowRight'){e.preventDefault();step(s,e.key==='ArrowRight'?1:-1);}});
            on('digits','keydown',e=>{const b=e.target.closest('[data-power]');if(b&&(e.key==='ArrowUp'||e.key==='ArrowDown')){e.preventDefault();s.tuning.frequency_hz=clamp(s,s.tuning.frequency_hz+(e.key==='ArrowUp'?1:-1)*Number(b.dataset.power));controls(s);retune(s);}});
            on('digits','wheel',e=>{const b=e.target.closest('[data-power]');if(!b)return;e.preventDefault();s.tuning.frequency_hz=clamp(s,s.tuning.frequency_hz+(e.deltaY<0?1:-1)*Number(b.dataset.power));controls(s);retune(s);});
            controls(s); setup(s); stations(s);
        } else {
            const schedule=s.tab==='schedules';
            s.q('content').innerHTML=`<div class="sdr-library-heading"><div><h2>${t(s.tab)}</h2><p>${t(schedule?'schedule_hint':'recording_hint')}</p></div><span data-sdr="quota"></span></div><form class="sdr-job-form" data-sdr="job-form">${field(s,'name','<input data-sdr="name" maxlength="120" autocomplete="off">')}${field(s,'duration','<input data-sdr="duration" type="number" min="0.1" max="120" value="10" step="0.1">')}${schedule?field(s,'start','<input data-sdr="start" type="datetime-local" required>')+field(s,'timezone','<input data-sdr="timezone" value="'+s.esc(Intl.DateTimeFormat().resolvedOptions().timeZone||'UTC')+'" required>')+field(s,'repeat','<select data-sdr="repeat">'+['once','daily','weekly'].map(v=>'<option value="'+v+'">'+t(v)+'</option>').join('')+'</select>'):''}${field(s,'transcribe','<input data-sdr="transcribe" type="checkbox" checked>')}<button type="submit" class="sdr-primary">${t(schedule?'save_schedule':'record')}</button><small data-sdr="job-tune"></small></form><div data-sdr="jobs" class="sdr-jobs"></div><section data-sdr="result" class="sdr-result" hidden></section>`;
            if(schedule){const date=new Date(Date.now()+3600000);date.setMinutes(0,0,0);s.q('start').value=new Date(date-date.getTimezoneOffset()*60000).toISOString().slice(0,16);}
            s.q('job-form').addEventListener('submit',e=>{e.preventDefault();saveJob(s,schedule).catch(err=>notice(s,err));},{signal:s.abort.signal});jobs(s);
        }
        update(s);
    }
    function increment(s){return {wfm:100000,nfm:12500,am:9000,usb:100,lsb:100,dab:0}[s.tuning.mode]||0;}
    function clamp(s,f){return Math.max(s.receiver.minimum_hz||100000,Math.min(s.receiver.maximum_hz||2200000000,f));}
    function step(s,n){if(s.tuning.mode==='dab'||s.data?.state?.active_job||s.data?.state?.read_only)return;s.tuning.frequency_hz=clamp(s,s.tuning.frequency_hz+n*increment(s));controls(s);retune(s);}
    function retune(s){
        clearTimeout(s.tuneTimer);
        const allowed=()=>!s.disposed&&R.playing&&!s.data?.state?.active_job&&!s.data?.state?.scanning&&!s.ctx.readonly&&!s.data?.state?.read_only&&(s.tuning.mode!=='dab'||!!s.tuning.service_id);
        if(!allowed())return;
        s.tuneTimer=setTimeout(()=>{
            if(!allowed())return;
            // Changes made while the decoder switches mode must not be lost.
            if(s.pending){retune(s);return;}
            run(s,()=>R.tune({...s.tuning})).catch(err=>notice(s,err));
        },400);
    }
    function controls(s){
        if(s.tab!=='receive')return;const tune=s.tuning;
        s.q('frequency').value=(tune.frequency_hz/1e6).toFixed(6);s.q('mode').value=tune.mode;s.q('bandwidth').value=tune.bandwidth_hz;s.q('agc').checked=tune.agc;s.q('stereo').checked=tune.stereo;s.q('ppm').value=tune.ppm;s.q('squelch').value=tune.squelch_db;
        s.q('gain').disabled=tune.agc;
        digits(s,tune);
        for(const name of ['knob','spectrum']){s.q(name).setAttribute('aria-valuenow',String(tune.frequency_hz/1e6));s.q(name).setAttribute('aria-valuetext',(tune.frequency_hz/1e6).toFixed(6)+' MHz');}
        s.q('knob').style.setProperty('--sdr-angle',((tune.frequency_hz/Math.max(1,increment(s)))*8%360)+'deg');
    }
    function digits(s,tune){
        const text=(tune.frequency_hz/1e6).toFixed(6).padStart(10,'0'), target=s.q('digits');
        if(target.dataset.frequency===text)return;
        const focused=target.contains(document.activeElement)?document.activeElement.dataset.power:null;
        target.dataset.frequency=text;
        let power=10**(text.indexOf('.')+5);
        target.innerHTML=[...text].map(c=>{if(c==='.')return '<span class="sdr-dot">.</span>';const html='<button type="button" data-power="'+power+'" aria-label="'+s.esc(s.t('digit'))+' '+power+' Hz">'+c+'</button>';power/=10;return html;}).join('')+'<small>MHz</small>';
        if(focused)target.querySelector('[data-power="'+focused+'"]').focus({preventScroll:true});
    }
    function readTuning(s){s.tuning={...s.tuning,frequency_hz:Math.round(Number(value(s,'frequency'))*1e6),mode:value(s,'mode'),bandwidth_hz:Number(value(s,'bandwidth')),gain_db:Number(value(s,'gain')||0),agc:s.q('agc').checked,stereo:s.q('stereo').checked,ppm:Number(value(s,'ppm')),squelch_db:Number(value(s,'squelch'))};return s.tuning;}
    async function run(s,fn){if(s.pending)return;s.pending=true;update(s);s.q('notice').hidden=true;try{return await fn();}finally{s.pending=false;if(!s.disposed){await R.refresh();update(s);}}}
    function update(s){
        if(!s.data)return;const st=s.data.state||{},rt=s.data.runtime||{};s.q('connection').textContent=s.t(st.active_job?'recording':st.scanning?'scanning':R.playing?'live':rt.status==='ready'?'ready':rt.status==='preparing'?'preparing':'stopped');s.q('lamp').classList.toggle('is-live',R.playing||!!st.active_job);s.q('storage').textContent=(st.used_bytes/1073741824).toFixed(2)+' / '+(st.quota_bytes/1073741824).toFixed(0)+' GB';
        const next=(st.schedules||[]).filter(x=>x.enabled&&Date.parse(x.next_start_at||x.start_at)>Date.now()&&Date.parse(x.next_start_at||x.start_at)<Date.now()+60000)[0];s.q('priority').hidden=!st.active_job&&!next;s.q('priority').textContent=s.t(st.active_job?'recording_priority':'recording_soon');
        const ro=s.ctx.readonly||st.read_only||!s.data.config?.enabled;
        if(s.tab==='receive'){stations(s);const scan=s.host.querySelector('[data-action="cancel-scan"]');scan.hidden=!st.scanning;s.host.querySelector('[data-action="scan"]').disabled=ro||s.pending||st.scanning||!!st.active_job;s.host.querySelector('[data-action="mute"]').textContent=s.t(R.muted?'unmute':'mute');s.q('volume').value=R.volumeValue;s.q('knob').disabled=ro||s.tuning.mode==='dab'||!!st.active_job;}
        else{jobs(s);s.q('job-tune').textContent=(s.tuning.label||'')+' · '+(s.tuning.mode==='dab'?'DAB+ '+(s.tuning.dab_block||'—'):(s.tuning.frequency_hz/1e6).toFixed(3)+' MHz · '+s.tuning.mode.toUpperCase());}
        s.host.querySelectorAll('[data-action]').forEach(b=>{const a=b.dataset.action;const local=['mute','stop','result','playback','close-result','show-setup'];b.disabled=s.pending||(!local.includes(a)&&!['enable','prepare','save-config'].includes(a)&&ro);});
        s.host.querySelectorAll('.sdr-job-form button[type="submit"]').forEach(b=>b.disabled=s.pending||ro);

        if(s.ctx.readonly)s.host.querySelectorAll('[data-action="enable"],[data-action="save-config"],[data-action="prepare"]').forEach(b=>b.disabled=true);
        if(s.tab==='receive'){
            const busy=!!st.active_job||st.scanning;
            s.host.querySelector('[data-action="scan"]').disabled=ro||busy||s.pending;
            s.q('knob').disabled=ro||busy||s.pending||s.tuning.mode==='dab';
            s.host.querySelectorAll('[data-power],[data-sdr="frequency"],[data-sdr="mode"]').forEach(b=>b.disabled=ro||busy||s.tuning.mode==='dab');
            s.q('mode').disabled=ro||busy;
            s.host.querySelectorAll('[data-action="play"],[data-action="apply"],[data-action="up"],[data-action="down"],[data-action="station"]').forEach(b=>b.disabled=ro||busy||s.pending||b.dataset.action==='play'&&s.tuning.mode==='dab'&&!s.tuning.service_id);
        }
        if(rt.error&&!s.pending)notice(s,Error(rt.error));
    }
    async function setup(s){
        if(!s.q('setup'))return;const t=key=>s.esc(s.t(key));let devices=[];try{devices=await R.request('devices','GET',undefined,s.abort.signal);}catch(_){}if(s.disposed||!s.q('setup'))return;
        const cfg=s.data?.config||{};s.q('setup').innerHTML=`<h3>${t('setup')}</h3><p>${t('setup_hint')}</p><div class="sdr-fields">${field(s,'device','<select data-sdr="device"><option value="">'+t('automatic')+'</option>'+devices.map(d=>'<option value="'+s.esc(d.id)+'">'+s.esc(d.name+' · '+d.id)+'</option>').join('')+'</select>')}${field(s,'quota','<input data-sdr="quota-gb" type="number" min="1" max="1000" value="'+(cfg.quota_gb||10)+'">')}${field(s,'allow_agent','<input data-sdr="allow-agent" type="checkbox" '+(cfg.allow_agent?'checked':'')+'>')}${field(s,'readonly','<input data-sdr="readonly" type="checkbox" '+(cfg.read_only?'checked':'')+'>')}</div>${button(s,'enable',cfg.enabled?'disable':'enable')}${button(s,'prepare','prepare')}${button(s,'save-config','save')}`;s.q('device').value=cfg.device||'';if(!cfg.enabled)s.q('setup').closest('details').open=true;
    }
    function stations(s){const target=s.q('stations');if(!target)return;const st=s.data?.state||{};const items=[...(st.favorites||[]).map(v=>({...v,favorite:true})),...(st.stations||[])];const sig=JSON.stringify(items);if(sig===s.stationSignature&&target.children.length)return;s.stationSignature=sig;target.innerHTML=items.length?items.map(v=>'<div class="sdr-station-item">'+button(s,'station','listen','data-id="'+s.esc(v.id)+'"')+'<span>'+s.esc(v.name)+'</span><small>'+s.esc(v.tuning.dab_block||((v.tuning.frequency_hz/1e6).toFixed(3)+' MHz'))+'</small>'+(v.favorite?button(s,'delete-favorite','remove','data-id="'+s.esc(v.id)+'"'):'')+'</div>').join(''):'<p class="sdr-empty">'+s.esc(s.t('stations_empty'))+'</p>';}
    function jobs(s){const st=s.data?.state||{},rows=s.tab==='schedules'?st.schedules||[]:st.recordings||[],sig=JSON.stringify(rows);if(s.signature===sig)return;s.signature=sig;s.q('jobs').innerHTML=rows.length?rows.slice().reverse().map(r=>`<article class="sdr-job"><div><strong>${s.esc(r.name||r.tuning?.label||s.t('untitled'))}</strong><small>${s.esc(new Date(s.tab==='schedules'&&r.next_start_at&&!r.next_start_at.startsWith('0001-')?r.next_start_at:r.start_at).toLocaleString())} · ${s.esc(s.t(r.status||r.repeat))} · ${Math.round((r.duration_seconds||0)/60)} ${s.esc(s.t('minutes'))}</small>${r.error||r.asr_error?'<span class="sdr-warning">'+s.esc(s.t(r.asr_error?'asr_failed':r.status==='missed'?'missed':r.error==='sdr_device_lost'?'sdr_device_lost':'partial_hint'))+'</span>':''}</div><div class="sdr-job-actions">${s.tab==='recordings'?button(s,'result','details','data-id="'+r.id+'"')+(r.id===st.active_job?button(s,'stop-recording','stop','data-id="'+r.id+'"'):''):''}${button(s,s.tab==='recordings'?'delete-recording':'delete-schedule','remove','data-id="'+r.id+'"')}</div></article>`).join(''):'<p class="sdr-empty">'+s.esc(s.t(s.tab==='schedules'?'schedules_empty':'recordings_empty'))+'</p>';}
    async function saveJob(s,schedule){return run(s,async()=>{const body={name:value(s,'name'),tuning:s.tuning,duration_seconds:Math.round(Number(value(s,'duration'))*60),transcribe:s.q('transcribe').checked};if(schedule){const local=value(s,'start'),zone=value(s,'timezone');body.timezone=zone;body.start_at=zonedISO(local,zone);body.repeat=value(s,'repeat');body.enabled=true;}await R.request(schedule?'schedules':'recordings','POST',body);});}
    function zonedISO(local,zone){const desired=new Date(local+'Z').getTime();if(!Number.isFinite(desired))throw Error('invalid');const formatter=new Intl.DateTimeFormat('en-CA',{timeZone:zone,year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',hourCycle:'h23'});let instant=desired;for(let i=0;i<3;i++){const p=Object.fromEntries(formatter.formatToParts(new Date(instant)).map(x=>[x.type,x.value]));const rendered=Date.UTC(+p.year,+p.month-1,+p.day,+p.hour,+p.minute,+p.second);if(rendered===desired)return new Date(instant).toISOString();instant+=desired-rendered;}throw Error('invalid');}
    async function result(s,id){const r=await R.request('recordings/'+id);if(s.disposed||!s.q('result'))return;const box=s.q('result');box.hidden=false;box.innerHTML='<div class="sdr-result-head"><h3>'+s.esc(r.name||s.t('untitled'))+'</h3>'+button(s,'close-result','close')+'</div>'+(r.bytes?'<audio controls preload="metadata" src="/api/desktop/rtl-sdr/recordings/'+id+'/audio"></audio><a class="sdr-download" href="/api/desktop/rtl-sdr/recordings/'+id+'/audio?download=1">'+s.esc(s.t('download'))+'</a>'+button(s,'transcribe','retry_asr','data-id="'+id+'"'):'')+'<div class="sdr-transcript">'+(r.segments||[]).map(seg=>'<p><time>'+Math.floor(seg.start_seconds/60)+':'+String(Math.floor(seg.start_seconds%60)).padStart(2,'0')+'</time><span>'+s.esc(seg.error?s.t('asr_failed'):seg.text||s.t('no_speech'))+'</span></p>').join('')+'</div>';s.preview=box.querySelector('audio');box.scrollIntoView({block:'nearest'});}
    async function action(s,b){if(b.dataset.action==='show-setup'){s.tab='receive';content(s);const section=s.host.querySelector('.sdr-settings');section.open=true;section.scrollIntoView({block:'start'});return;}const a=b.dataset.action,id=b.dataset.id;if(a==='mute'){R.mute();return;}if(a==='up'||a==='down'){step(s,a==='up'?1:-1);return;}if(a==='close-result'){s.preview?.pause();s.q('result').hidden=true;return;}if(a==='result')return result(s,id);
        return run(s,async()=>{
            if(a==='play'||a==='apply'){clearTimeout(s.tuneTimer);readTuning(s);controls(s);await R.tune({...s.tuning});}
            else if(a==='stop'){clearTimeout(s.tuneTimer);await R.stop();}
            else if(a==='scan')await R.request('scan','POST',{});
            else if(a==='cancel-scan')await R.request('scan','DELETE');
            else if(a==='favorite'){readTuning(s);await R.request('favorites','POST',{name:s.receiver.label?.trim()||s.tuning.label||((s.tuning.frequency_hz/1e6).toFixed(3)+' MHz'),tuning:s.tuning});}
            else if(a==='station'){const st=s.data.state,item=[...(st.favorites||[]),...(st.stations||[])].find(x=>x.id===id);if(item){s.tuning={...item.tuning};controls(s);await R.tune(s.tuning);}}
            else if(a==='delete-favorite')await R.request('favorites/'+id,'DELETE');
            else if(a==='stop-recording')await R.request('recordings/'+id+'/stop','POST',{});
            else if(a==='transcribe')await R.request('recordings/'+id+'/transcribe','POST',{});
            else if(a==='delete-recording'||a==='delete-schedule'){if(await s.ctx.confirmDialog(s.t('remove'),s.t('delete_confirm')))await R.request((a==='delete-recording'?'recordings/':'schedules/')+id,'DELETE');}
            else if(a==='prepare')await R.request('setup','POST',{});
            else if(a==='enable'||a==='save-config'){const body={device:value(s,'device'),quota_gb:Number(value(s,'quota-gb')),allow_agent:s.q('allow-agent').checked,read_only:s.q('readonly').checked};if(a==='enable')body.enabled=!s.data.config.enabled;await R.request('config','PUT',body);await R.refresh();await setup(s);}
        });
    }
    async function receiver(s){if(s.disposed||s.tab!=='receive'||s.receiving||document.hidden||!s.data?.config?.enabled)return;s.receiving=true;try{const r=await R.request('receiver','GET',undefined,s.abort.signal);if(s.disposed||s.tab!=='receive')return;s.receiver=r;const actual=(s.data?.state?.recordings||[]).find(v=>v.id===s.data?.state?.active_job)?.tuning||s.tuning;digits(s,actual);s.q('mode-label').textContent=(actual.mode==='dab'?'DAB+':actual.mode.toUpperCase())+(r.stereo?' · STEREO':'');s.q('station').textContent=r.label?.trim()||s.tuning.label||s.t('ready');s.q('radiotext').textContent=r.text?.trim()||r.tuner||'';s.q('level').textContent=(s.tuning.mode==='dab'?Number(r.snr_db||0).toFixed(1)+' dB SNR':Number(r.power_db||-100).toFixed(1)+' dB');s.q('meter').style.width=Math.max(0,Math.min(100,100+(r.power_db||-100)))+'%';const gains=JSON.stringify(r.gains_db);if(gains!==s.gains){s.gains=gains;s.q('gain').innerHTML=(r.gains_db||[]).map(g=>'<option value="'+g+'">'+g+' dB</option>').join('');s.q('gain').value=s.tuning.gain_db;}spectrum(s,r);}catch(_){}finally{s.receiving=false;}}
    function spectrum(s,r){const canvas=s.q('spectrum'),bins=r.spectrum;if(!canvas||!bins?.length)return;const width=Math.max(320,Math.round(canvas.getBoundingClientRect().width));if(canvas.width!==width){canvas.width=width;canvas.height=210;}const c=canvas.getContext('2d'),h=75;c.drawImage(canvas,0,h,width,135,0,h+1,width,135);const row=c.createImageData(width,1);for(let x=0;x<width;x++){const db=bins[Math.min(bins.length-1,Math.floor(x/width*bins.length))],v=Math.max(0,Math.min(1,(db+100)/75)),i=x*4;row.data[i]=Math.round(255*Math.max(0,(v-.45)*1.8));row.data[i+1]=Math.round(225*Math.sin(v*Math.PI));row.data[i+2]=Math.round(55+180*(1-v));row.data[i+3]=255;}c.putImageData(row,0,h);c.fillStyle='#0a121c';c.fillRect(0,0,width,h);c.strokeStyle='#1f3241';c.lineWidth=1;for(let y=15;y<h;y+=20){c.beginPath();c.moveTo(0,y);c.lineTo(width,y);c.stroke();}c.strokeStyle='#75eadc';c.beginPath();for(let x=0;x<width;x++){const v=bins[Math.floor(x/width*bins.length)];const y=Math.max(1,Math.min(h-1,h-(v+110)/100*h));if(x)c.lineTo(x,y);else c.moveTo(x,y);}c.stroke();c.strokeStyle='#f5b968';c.beginPath();c.moveTo(width/2,0);c.lineTo(width/2,210);c.stroke();const center=r.center_hz||s.tuning.frequency_hz,span=r.span_hz||2400000;s.q('low').textContent=((center-span/2)/1e6).toFixed(3);s.q('high').textContent=((center+span/2)/1e6).toFixed(3)+' MHz';}
    window.RTLSDRApp={render,dispose};
})();
