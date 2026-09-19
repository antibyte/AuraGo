(function () {
    'use strict';
    const instances = new Map(), R = window.PersonalRadioRuntime;
    function dispose(id) { const s = instances.get(id); if (!s) return; s.disposed = true; s.abort.abort(); if(s.preview){s.preview.pause();s.preview.src='';URL.revokeObjectURL(s.previewURL);} if (s.unsubscribe) s.unsubscribe(); if (s.ctx.clearWindowMenus) s.ctx.clearWindowMenus(id); instances.delete(id); }
    function render(host,id,ctx) {
        dispose(id);
        const s = { host, id, ctx, data: null, selected: '', tab: 'program', editing: false, busy: false, abort: new AbortController() };
        const t = key => ctx.t('personalRadio.' + key), esc = ctx.esc;
        s.t = t; s.esc = esc; instances.set(id,s);
        host.innerHTML = '<div class="pr-app"><header class="pr-header"><div class="pr-brand"><span aria-hidden="true">◉</span><div><strong>Personal Radio</strong><small>' + esc(t('subtitle')) + '</small></div></div><div class="pr-header-actions"><select data-pr="station" aria-label="' + esc(t('station')) + '"></select><button type="button" data-action="new">＋ ' + esc(t('new_station')) + '</button><button type="button" data-action="settings">' + esc(t('settings')) + '</button></div></header><div class="pr-notice" data-pr="notice" role="status" hidden></div><div class="pr-deck"><div class="pr-air"><span data-pr="state">' + esc(t('stopped')) + '</span><div class="pr-record" aria-hidden="true"><i></i></div></div><div class="pr-now"><small data-pr="kind">' + esc(t('now')) + '</small><h1 data-pr="title">' + esc(t('welcome')) + '</h1><p data-pr="theme">' + esc(t('setup_hint')) + '</p><progress data-pr="progress" value="0" max="1"></progress><div class="pr-transport"><button type="button" class="pr-primary" data-action="start">▶ ' + esc(t('start')) + '</button><button type="button" data-action="pause" aria-label="' + esc(t('pause')) + '">Ⅱ</button><button type="button" data-action="skip" aria-label="' + esc(t('skip')) + '">▶|</button><button type="button" data-action="stop" aria-label="' + esc(t('stop')) + '">■</button><label class="pr-volume">' + esc(t('volume')) + '<input data-pr="volume" type="range" min="0" max="1" step="0.02" value="0.8"></label></div></div><aside class="pr-reserve"><strong data-pr="reserve">0 '+esc(t('minutes'))+'</strong><span>'+esc(t('prepared'))+'</span><progress data-pr="buffer" value="0" max="1"></progress><small data-pr="jobs"></small><small data-pr="news-time"></small></aside></div><nav class="pr-tabs">' + ['program','library','news'].map(tab => '<button type="button" data-tab="' + tab + '">' + esc(t(tab)) + '</button>').join('') + '</nav><main class="pr-content" data-pr="content"></main></div>';
        s.q = name => host.querySelector('[data-pr="'+name+'"]');
        const preparation = document.createElement('section');
        preparation.className = 'pr-preparation'; preparation.dataset.pr = 'preparation'; preparation.hidden = true;
        preparation.innerHTML = '<div role="status" aria-live="polite" aria-atomic="true"><div class="pr-preparation-heading"><span class="pr-activity" aria-hidden="true"></span><strong data-pr="preparation-title"></strong></div><p data-pr="preparation-counts"></p></div><progress data-pr="preparation-progress" max="1" value="0"></progress><p data-pr="preparation-hint"></p>';
        s.q('progress').before(preparation);
        s.q('volume').value = R.volumeValue;
        s.q('volume').addEventListener('input', e => R.volume(e.target.value));
        s.q('station').addEventListener('change', e => { s.selected = e.target.value; s.editing = false; s.subview = false; draw(s,true); });
        host.addEventListener('click', e => {
            const button = e.target.closest('button'); if (!button || s.disposed) return;
            if (button.dataset.tab) { s.tab = button.dataset.tab; s.editing = false; s.subview = false; draw(s,true); }
            if (button.dataset.action) action(s,button.dataset.action).catch(err => notice(s,err));
        },{ signal:s.abort.signal });
        s.unsubscribe = R.subscribe((data,error) => { if (s.disposed) return; s.data=data; draw(s,false); if(error) notice(s,Error(error),false); });
        R.init(ctx).catch(err=>notice(s,err));
        if(ctx.setWindowMenus) ctx.setWindowMenus(id,[{label:t('station'),items:[{label:t('new_station'),action:()=>edit(s,true)},{label:t('settings'),action:()=>edit(s,false)},{label:t('stop'),action:()=>R.control('stop').catch(err=>notice(s,err))}]}]);
    }
    function notice(s,err,persist=true) { const el=s.q('notice'); if(!el)return; el.hidden=false; const code=String(err.message||''); if(persist) s.error = code; const known=['radio_more_music_needed','radio_limit','radio_device_busy','radio_audio_unlock','radio_invalid_genres','radio_region_required','radio_conflict','radio_format_unsupported','radio_production_too_slow','radio_music_unavailable','radio_generation_failed','radio_tts_unavailable','radio_unavailable']; el.textContent=s.t(known.includes(code)?code:'error'); }
    function station(s) { return s.data && (s.data.stations||[]).find(x=>x.id===s.selected); }
    function draw(s,force) {
        if(!s.data)return;
        const d=s.data,st=d.state||{},p=station(s),active=!!(p&&st.station_id===p.id),current=active&&(st.queue||[]).find(x=>x.id===st.current),list=d.stations||[];
        if(!s.selected&&list.length){s.selected=list[0].id;return draw(s,true);}
        const options=list.map(x=>'<option value="'+s.esc(x.id)+'">'+s.esc(x.name)+'</option>').join('');
        if(s.options!==options){s.q('station').innerHTML=options;s.options=options;} s.q('station').value=s.selected;
        s.q('state').textContent=s.t(active?st.status:'stopped');s.host.querySelector('.pr-record').classList.toggle('is-playing',active&&st.status==='playing');
        s.q('title').textContent=current?current.title:p?p.name:s.t('welcome'); s.q('kind').textContent=current?s.t(current.kind):s.t('now');
        s.q('theme').textContent=active&&st.theme?st.theme:p?p.topics:s.t('setup_hint');
        const position=R.position();s.q('progress').max=position.duration||1;s.q('progress').value=position.position||0;
        s.q('reserve').textContent=(active?Math.floor(st.buffer_ms/60000):0)+' / '+(active?Math.ceil(st.required_ms/60000):p?p.reserve_minutes:30)+' '+s.t('minutes');
        s.q('buffer').max=st.required_ms||1;s.q('buffer').value=active?st.buffer_ms:0;
        s.q('jobs').textContent=active?(st.music_busy?s.t('generating'):st.editor_busy?s.t('editing'):st.relaxed?s.t('relaxed'):''):'';
        preparationStatus(s, p, st, active);
        const next=active&&st.next_news&&new Date(st.next_news);s.q('news-time').textContent=next&&next.getFullYear()>2000?s.t('next_news')+' '+next.toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'}):'';
        const readonly=s.ctx.readonly||d.capabilities&&d.capabilities.read_only;
        for(const name of ['new','settings','start'])s.host.querySelector('[data-action="'+name+'"]').disabled=readonly||!d.available||(name!=='new'&&!p)||s.busy;
        const starting = !!R.startingStation;
        s.q('station').disabled = starting;
        for(const name of ['new','settings','start'])if(starting)s.host.querySelector('[data-action="'+name+'"]').disabled=true;
        if(active && R.owned() && st.status!=='stopped' && st.status!=='paused')s.host.querySelector('[data-action="start"]').disabled=true;
        for(const name of ['pause','stop','skip'])s.host.querySelector('[data-action="'+name+'"]').disabled=readonly||!R.owned()||!active||s.busy;
        if (!(st.queue || []).length) s.host.querySelector('[data-action="skip"]').disabled = true;
        s.host.querySelector('[data-action="pause"]').disabled = readonly || !R.owned() || !active || st.status==='preparing' || s.busy;
        s.host.querySelector('[data-action="start"]').textContent=starting?s.t('starting'):active&&st.status==='paused'?'▶ '+s.t('resume'):'▶ '+s.t('start');
        s.host.querySelector('[data-action="start"]').setAttribute('aria-busy', String(starting));
        if(!d.available)notice(s,Error('radio_unavailable'),false);else if(active&&st.code)notice(s,Error(st.code),false);else if(active&&st.editorial_code)notice(s,Error(st.editorial_code),false);else if(!s.busy&&!s.error)s.q('notice').hidden=true;
        if(s.editing||s.subview)return;
        const signature=JSON.stringify([s.selected,s.tab,s.tab==='program'&&active?st.queue:[],s.tab==='news'&&active?st.news:[],st.news_code]);
        if(force||s.signature!==signature){s.signature=signature;content(s);}
        s.host.querySelectorAll('[data-tab]').forEach(el=>el.classList.toggle('is-active',el.dataset.tab===s.tab));
    }
    function preparationStatus(s, p, st, active) {
        const starting = p && R.startingStation === p.id;
        const visible = !!(starting || (active && st.status !== 'stopped' && !st.music_ready));
        s.q('preparation').hidden = !visible;
        if (!visible) return;
        let key = 'preparing';
        if (starting) key = 'starting';
        else if (st.status === 'paused') key = 'paused';
        else if (st.opening_status === 'writing') key = 'opening_writing';
        else if (st.opening_status === 'synthesizing') key = 'opening_synthesizing';
        else if (st.opening_status === 'playing') key = 'opening_playing';
        else if (st.music_busy) key = 'generating';
        const tracks = active ? st.track_count || 0 : 0, requiredTracks = p.min_tracks;
        const milliseconds = active ? st.buffer_ms || 0 : 0, requiredMS = active && st.required_ms || p.reserve_minutes * 60000;
        const number = value => new Intl.NumberFormat(window.SYSTEM_LANG || 'en', { maximumFractionDigits: 1 }).format(value);
        const counts = s.ctx.t('personalRadio.startup_counts', { tracks, requiredTracks, minutes: number(milliseconds / 60000), requiredMinutes: number(requiredMS / 60000) });
        const text = (name, value) => { const el = s.q(name); if (el.textContent !== value) el.textContent = value; };
        text('preparation-title', s.t(key)); text('preparation-counts', counts);
        text('preparation-hint', s.t(p.mode === 'local' ? 'startup_local_hint' : 'startup_hint'));
        s.q('preparation-progress').value = Math.min(1, milliseconds / requiredMS, tracks / requiredTracks);
        s.q('preparation-progress').setAttribute('aria-label', counts);
        s.q('preparation').classList.toggle('is-busy', !!(starting || st.music_busy || st.editor_busy));
    }
    function content(s) {
        const host=s.q('content'),p=station(s),st=s.data.state||{},t=s.t,e=s.esc;
        if(!p){host.innerHTML='<div class="pr-empty"><h2>'+e(t('welcome'))+'</h2><p>'+e(t('setup_hint'))+'</p><button type="button" class="pr-primary" data-action="new">'+e(t('new_station'))+'</button></div>';return;}
        if(s.tab==='library'){library(s);return;}
        if(s.tab==='news'){
            const items=st.station_id===p.id?st.news||[]:s.newsCache&&s.newsCache[p.id]||[];if(st.station_id!==p.id&&!(s.newsCache&&s.newsCache[p.id])){R.request('stations/'+p.id+'/news').then(data=>{s.newsCache=s.newsCache||{};s.newsCache[p.id]=data.items||[];if(!s.disposed&&s.tab==='news'&&s.selected===p.id)content(s);}).catch(err=>notice(s,err));}host.innerHTML='<h2>'+e(t('news'))+'</h2>'+(st.news_code?'<p class="pr-hint">'+e(t('news_failed'))+'</p>':'')+(items.length?items.map(x=>'<article class="pr-bulletin"><time>'+e(new Date(x.due).toLocaleString())+'</time><p>'+e(x.text)+'</p><div>'+x.sources.map(src=>{let url;try{url=new URL(src.url);}catch(_){return '';}return ['https:','http:'].includes(url.protocol)?'<a target="_blank" rel="noopener noreferrer" href="'+e(url.href)+'">'+e(src.title)+'</a>':'';}).join('')+'</div></article>').join(''):'<p class="pr-empty">'+e(t(p.news_minutes?'no_news':'news_off'))+'</p>');return;
        }
        const queue=st.station_id===p.id?st.queue||[]:[];
        host.innerHTML='<div class="pr-section-head"><h2>'+e(t('program'))+'</h2><span>'+e(t('background_hint'))+'</span></div>'+(queue.length?'<ol class="pr-queue">'+queue.map((x,i)=>'<li><span class="pr-order">'+(i+1)+'</span><span class="pr-type">'+e(t(x.kind))+'</span><div><strong>'+e(x.title)+'</strong>'+(x.text?'<p>'+e(x.text)+'</p>':'')+'</div><time>'+Math.floor(x.duration_ms/60000)+':'+String(Math.floor(x.duration_ms/1000)%60).padStart(2,'0')+'</time></li>').join('')+'</ol>':'<div class="pr-empty">'+e(t('queue_empty'))+'</div>')+'<div class="pr-capabilities">'+['music','tts','llm','news'].map(key=>'<span class="'+(s.data.capabilities[key]?'is-ready':'')+'">'+(s.data.capabilities[key]?'● ':'○ ')+e(t(key==='llm'?'editor':key))+'</span>').join('')+'</div>';
    }
    async function library(s) {
        s.subview=false;const p=station(s);if(!p)return;const id=p.id,host=s.q('content'),t=s.t,e=s.esc;
        host.innerHTML='<div class="pr-section-head"><h2>'+e(t('library'))+'</h2><div class="pr-library-actions"><button type="button" data-action="upload">'+e(t('upload'))+'</button><button type="button" data-action="desktop_file">'+e(t('desktop_file'))+'</button><button type="button" data-action="existing">'+e(t('existing'))+'</button><button type="button" data-action="folder">'+e(t('folder'))+'</button><button type="button" data-action="cleanup">'+e(t('cleanup'))+'</button></div></div><p class="pr-hint">'+e(t('import_hint'))+'</p><label class="pr-field"><span>'+e(t('import_genre'))+'</span><select data-pr-import-genre><option value="">'+e(t('any_genre'))+'</option>'+p.genres.map(g=>'<option value="'+e(g.name)+'">'+e(g.name)+'</option>').join('')+'</select></label><div data-pr-tracks>'+e(t('loading'))+'</div><input type="file" data-pr-upload multiple accept=".mp3,.wav" hidden>';
        host.querySelector('[data-pr-upload]').addEventListener('change',e=>uploads(s,Array.from(e.target.files||[])).catch(err=>notice(s,err)));
        try{const data=await R.request('stations/'+id+'/tracks');if(s.disposed||s.selected!==id||s.tab!=='library'||s.editing||s.subview)return;(host.querySelector('[data-pr-tracks]')||document.createElement('div')).innerHTML=(data.items||[]).length?data.items.map(x=>'<div class="pr-track"><div><strong>'+e(x.title)+'</strong><small>'+e(t(x.origin))+' · '+Math.ceil(x.duration_ms/60000)+' '+e(t('minutes'))+' · '+x.plays+' '+e(t('plays'))+'</small></div><button type="button" data-track="'+x.id+'" data-op="favorite" aria-label="'+e(t('favorite'))+'">'+(x.favorite?'★':'☆')+'</button><button type="button" data-track="'+x.id+'" data-op="weight">'+e(t('less'))+'</button><button type="button" data-track="'+x.id+'" data-op="blocked">'+e(t(x.blocked?'unblock':'block'))+'</button><button type="button" data-track="'+x.id+'" data-op="remove" aria-label="'+e(t('remove'))+'">×</button></div>').join(''):'<p class="pr-empty">'+e(t('no_music'))+'</p>';
            host.querySelectorAll('[data-track]').forEach(button=>{button.disabled=!!s.ctx.readonly;button.addEventListener('click',async()=>{const x=data.items.find(x=>x.id===button.dataset.track),op=button.dataset.op;try{if(op==='remove'){await R.request('stations/'+id+'/tracks/'+x.id,'DELETE');}else{const body={favorite:x.favorite,blocked:x.blocked,weight:x.weight};if(op==='weight')body.weight=25;else body[op]=!body[op];await R.request('stations/'+id+'/tracks/'+x.id,'PATCH',body);}await library(s);}catch(err){notice(s,err);}});});
        }catch(err){notice(s,err);}
    }
    function importGenre(s) { const input=s.host.querySelector('[data-pr-import-genre]'); return input?input.value:''; }
    async function uploads(s,files) {
        if(s.busy||s.ctx.readonly)return;s.busy=true;const p=station(s);
        try{for(let i=0;i<files.length;i++){s.q('notice').hidden=false;s.q('notice').textContent=s.t('importing')+' '+(i+1)+' / '+files.length;const file=files[i];await s.ctx.api('/api/desktop/personal-radio/stations/'+p.id+'/upload?name='+encodeURIComponent(file.name)+'&genre='+encodeURIComponent(importGenre(s)),{method:'POST',body:file,signal:s.abort.signal});}await library(s);await R.refresh();}finally{s.busy=false;draw(s,false);}
    }
    function edit(s,isNew) {
        if(s.ctx.readonly||!s.data)return;
        let p=isNew?JSON.parse(JSON.stringify(s.data.defaults)):station(s);if(!p)return;
        if(!isNew&&s.data.state.station_id===p.id&&s.data.state.status!=='stopped'){notice(s,Error('radio_conflict'));return;}
        if(isNew){p.language=(window.SYSTEM_LANG||document.documentElement.lang||'de').slice(0,2);p.timezone=Intl.DateTimeFormat().resolvedOptions().timeZone||'UTC';}
        s.editing=true;
        window.PersonalRadioSettings.render(s.q('content'),p,{t:s.t,esc:s.esc,cancel:()=>{s.editing=false;draw(s,true);},save:async value=>{try{const saved=await R.request(value.id?'stations/'+value.id:'stations',value.id?'PATCH':'POST',value);s.selected=saved.id;s.editing=false;s.error='';await R.refresh();draw(s,true);}catch(err){notice(s,err);}}});
    }
    async function action(s,name) {
        if(s.ctx.readonly&&name!=='existing')return;
        s.error='';s.q('notice').hidden=true; if(name==='new')return edit(s,true);if(name==='settings')return edit(s,false);
        const p=station(s);if(!p)return;
        if(name==='start'){if(s.preview)s.preview.pause();if(s.data.state.station_id===p.id&&s.data.state.status==='paused'&&R.owned())return R.control('resume');try{await R.start(p.id,false);}catch(err){if(err.message==='radio_device_busy'&&s.ctx.confirmDialog&&await s.ctx.confirmDialog('Personal Radio',s.t('takeover'))){await R.start(p.id,true);}else throw err;}return;}
        if(['pause','stop','skip'].includes(name))return R.control(name);
        if(name==='upload')return s.host.querySelector('[data-pr-upload]').click();
        if(name==='desktop_file'){const picked=await s.ctx.openFileDialog({title:s.t('desktop_file'),path:'Documents',filters:[{label:s.t('music'),extensions:['.mp3','.wav']}],accept:'.mp3,.wav'});if(picked&&!picked.canceled){const path=typeof picked==='string'?picked:picked.path;if(path){await R.request('stations/'+p.id+'/imports','POST',{path,genre:importGenre(s)});await library(s);}}return;}
        if(name==='existing'){
            s.subview=true;
            const host=s.q('content');host.innerHTML='<h2>'+s.esc(s.t('existing'))+'</h2><button type="button" data-action="library">'+s.esc(s.t('back'))+'</button><div data-existing></div><button type="button" data-more>'+s.esc(s.t('more'))+'</button>';
            let offset=0;const load=async()=>{const data=await R.request('library?offset='+offset);if(s.disposed||!s.subview||!host.querySelector('[data-existing]'))return;offset+=data.items.length;const box=host.querySelector('[data-existing]');for(const x of data.items){const row=document.createElement('button');row.className='pr-import-row';row.textContent='＋ '+(x.title||x.filename);row.disabled=!!s.ctx.readonly;row.onclick=async()=>{row.disabled=true;try{await R.request('stations/'+p.id+'/imports','POST',{media_id:x.id,genre:importGenre(s)});row.textContent='✓ '+(x.title||x.filename);}catch(err){row.disabled=false;notice(s,err);}};box.appendChild(row);}host.querySelector('[data-more]').hidden=offset>=data.total;};host.querySelector('[data-more]').onclick=()=>load().catch(err=>notice(s,err));await load();return;
        }
        if(name==='cleanup'){if(await s.ctx.confirmDialog(s.t('cleanup'),s.t('cleanup_confirm'))){await R.request('unused','DELETE');await library(s);}return;}
        if(name==='folder'){
            const picked=await s.ctx.openFileDialog({title:s.t('folder'),mode:'select-folder',path:'Music'});
            if(!picked||picked.canceled)return;
            s.busy=true;let offset=0,more=true,count=0;
            try{while(more&&!s.disposed){const files=await R.request('files?path='+encodeURIComponent(picked.path)+'&offset='+offset);for(const path of files.paths){await R.request('stations/'+p.id+'/imports','POST',{path,genre:importGenre(s)},s.abort.signal);count++;s.q('notice').hidden=false;s.q('notice').textContent=s.t('importing')+' '+count;}more=files.more;offset=files.next_offset;}await library(s);await R.refresh();}finally{s.busy=false;draw(s,false);}return;
        }
        if(name==='preview'){
            if(s.preview){s.preview.pause();URL.revokeObjectURL(s.previewURL);}
            const response=await fetch('/api/desktop/personal-radio/stations/'+p.id+'/preview',{method:'POST',credentials:'same-origin',signal:s.abort.signal});
            if(!response.ok)throw Error('radio_tts_unavailable');const blob=await response.blob();if(s.disposed)return;
            s.previewURL=URL.createObjectURL(blob);s.preview=new Audio(s.previewURL);await s.preview.play();return;
        }
        if(name==='delete'){if(await s.ctx.confirmDialog(p.name,s.t('delete_confirm'))){await s.ctx.api('/api/desktop/personal-radio/stations/'+p.id,{method:'DELETE',headers:{'If-Match':String(p.revision)}});s.selected='';s.editing=false;await R.refresh();draw(s,true);}return;}
        if(name==='library')return library(s);
    }
    window.PersonalRadioApp={render,dispose};
})();
