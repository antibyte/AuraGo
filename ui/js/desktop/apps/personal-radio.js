(function () {
    'use strict';
    const instances = new Map(), R = window.PersonalRadioRuntime;
    const ICON = {
        play: '<svg class="pr-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M8 5.2v13.6L19.2 12z"/></svg>',
        pause: '<svg class="pr-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><rect x="6.4" y="5.2" width="3.7" height="13.6" rx="1.2"/><rect x="13.9" y="5.2" width="3.7" height="13.6" rx="1.2"/></svg>',
        next: '<svg class="pr-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M5.4 5.2v13.6L15 12z"/><rect x="16.2" y="5.2" width="2.8" height="13.6" rx="1.2"/></svg>',
        stop: '<svg class="pr-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><rect x="6" y="6" width="12" height="12" rx="2.4"/></svg>'
    };
    const STATUS = { playing: 'is-live', ready: 'is-live', buffering: 'is-busy', preparing: 'is-busy', paused: 'is-paused', stopped: 'is-stopped' };
    const clock = ms => { const total = Math.max(0, Math.round((Number(ms) || 0) / 1000)); return Math.floor(total / 60) + ':' + String(total % 60).padStart(2, '0'); };
    // Writing the same text again would re-announce a live region, so only real changes are applied.
    const setText = (el, value) => { if (el && el.textContent !== value) el.textContent = value; };
    function dispose(id) { const s = instances.get(id); if (!s) return; s.disposed = true; s.abort.abort(); if(s.preview){s.preview.pause();s.preview.src='';URL.revokeObjectURL(s.previewURL);} if (s.unsubscribe) s.unsubscribe(); if (s.ctx.clearWindowMenus) s.ctx.clearWindowMenus(id); instances.delete(id); }
    function render(host,id,ctx) {
        dispose(id);
        const s = { host, id, ctx, data: null, selected: '', tab: 'program', editing: false, busy: false, abort: new AbortController() };
        const t = key => ctx.t('personalRadio.' + key), esc = ctx.esc;
        const uid = 'pr-' + String(id).replace(/[^a-zA-Z0-9_-]/g, '');
        s.t = t; s.esc = esc; s.uid = uid; instances.set(id,s);
        host.innerHTML = '<div class="pr-app">' +
            '<header class="pr-header"><div class="pr-brand"><img class="pr-brand-icon" src="/img/personal-radio-icon.png" alt="" width="48" height="48" draggable="false"><div class="pr-brand-text"><strong>Personal Radio</strong><small>' + esc(t('subtitle')) + '</small></div></div>' +
            '<div class="pr-header-actions"><label class="pr-station"><span class="pr-label-text">' + esc(t('station')) + '</span><select data-pr="station" aria-label="' + esc(t('station')) + '"></select></label>' +
            '<button type="button" data-action="new">＋ ' + esc(t('new_station')) + '</button>' +
            '<button type="button" data-action="settings">' + esc(t('settings')) + '</button></div></header>' +
            '<div class="pr-notice" data-pr="notice" role="status" hidden><span class="pr-notice-mark" aria-hidden="true"></span><p data-pr="notice-text"></p><button type="button" class="pr-notice-close" data-action="dismiss" aria-label="' + esc(ctx.t('desktop.close')) + '" title="' + esc(ctx.t('desktop.close')) + '">×</button></div>' +
            '<section class="pr-deck">' +
                '<div class="pr-card pr-air"><span class="pr-status" data-pr="status"><i class="pr-status-dot" aria-hidden="true"></i><span data-pr="state">' + esc(t('stopped')) + '</span></span>' +
                '<div class="pr-record" aria-hidden="true"><i></i></div><p class="pr-air-note">' + esc(t('background_hint')) + '</p></div>' +
                '<div class="pr-card pr-now"><div class="pr-now-body"><div class="pr-now-head"><span class="pr-badge" data-pr="kind">' + esc(t('now')) + '</span><h1 data-pr="title" aria-live="polite">' + esc(t('welcome')) + '</h1><p data-pr="theme">' + esc(t('setup_hint')) + '</p></div>' +
                '<section class="pr-preparation" data-pr="preparation" hidden><div role="status" aria-live="polite" aria-atomic="true"><div class="pr-preparation-heading"><span class="pr-activity" aria-hidden="true"></span><strong data-pr="preparation-title"></strong></div><p data-pr="preparation-counts"></p></div><progress data-pr="preparation-progress" max="1" value="0"></progress><p data-pr="preparation-hint"></p></section></div>' +
                '<div class="pr-scrub"><progress data-pr="progress" value="0" max="1" aria-label="' + esc(t('playback_position')) + '"></progress><div class="pr-times"><span data-pr="elapsed">0:00</span><span data-pr="duration">0:00</span></div></div>' +
                '<div class="pr-transport"><button type="button" class="pr-primary" data-action="start"><span class="pr-btn-icon" data-pr="start-icon">' + ICON.play + '</span><span data-pr="start-label">' + esc(t('start')) + '</span></button>' +
                '<button type="button" class="pr-icon-btn" data-action="pause" data-pr="pause-icon" aria-label="' + esc(t('pause')) + '" title="' + esc(t('pause')) + '">' + ICON.pause + '</button>' +
                '<button type="button" class="pr-icon-btn" data-action="skip" data-pr="skip-icon" aria-label="' + esc(t('skip')) + '" title="' + esc(t('skip')) + '">' + ICON.next + '</button>' +
                '<button type="button" class="pr-icon-btn" data-action="stop" data-pr="stop-icon" aria-label="' + esc(t('stop')) + '" title="' + esc(t('stop')) + '">' + ICON.stop + '</button>' +
                '<label class="pr-volume"><span class="pr-volume-label">' + esc(t('volume')) + '</span><input data-pr="volume" type="range" min="0" max="1" step="0.02" value="0.8" aria-label="' + esc(t('volume')) + '"><output class="pr-volume-value" data-pr="volume-value" aria-hidden="true">80%</output></label></div></div>' +
                '<div class="pr-card pr-reserve"><span class="pr-label-text">' + esc(t('prepared')) + '</span><strong data-pr="reserve">0 ' + esc(t('minutes')) + '</strong><progress data-pr="buffer" value="0" max="1" aria-label="' + esc(t('prepared')) + '"></progress>' +
                '<div class="pr-chips"><span class="pr-chip" data-pr="jobs" hidden></span><span class="pr-chip" data-pr="news-time" hidden></span></div></div></section>' +
            '<nav class="pr-tabs" role="tablist" aria-label="' + esc(t('sections')) + '">' + ['program','library','news'].map((tab, index) => '<button type="button" role="tab" id="' + uid + '-tab-' + tab + '" data-tab="' + tab + '" aria-controls="' + uid + '-panel" aria-selected="' + (index === 0 ? 'true' : 'false') + '" tabindex="' + (index === 0 ? '0' : '-1') + '">' + esc(t(tab)) + '</button>').join('') + '</nav>' +
            '<main class="pr-content" id="' + uid + '-panel" role="tabpanel" aria-labelledby="' + uid + '-tab-program" tabindex="0" data-pr="content"></main></div>';
        s.q = name => host.querySelector('[data-pr="'+name+'"]');
        s.q('volume').value = R.volumeValue;
        s.q('volume-value').textContent = Math.round(R.volumeValue * 100) + '%';
        s.q('volume').addEventListener('input', e => { R.volume(e.target.value); s.q('volume-value').textContent = Math.round(R.volumeValue * 100) + '%'; });
        s.q('station').addEventListener('change', e => { s.selected = e.target.value; s.editing = false; s.subview = false; draw(s,true); });
        host.addEventListener('click', e => {
            const button = e.target.closest('button'); if (!button || s.disposed) return;
            if (button.dataset.tab) { selectTab(s,button.dataset.tab); return; }
            if (button.dataset.action) action(s,button.dataset.action).catch(err => notice(s,err));
        },{ signal:s.abort.signal });
        host.addEventListener('keydown', e => {
            if (s.disposed || e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) return;
            const tabs = host.querySelector('.pr-tabs');
            if (tabs && tabs.contains(e.target)) {
                const order = ['program','library','news'];
                const current = order.indexOf(e.target.dataset.tab);
                if (current < 0) return;
                let next = current;
                if (e.key === 'ArrowRight') next = (current + 1) % order.length;
                else if (e.key === 'ArrowLeft') next = (current + order.length - 1) % order.length;
                else if (e.key === 'Home') next = 0;
                else if (e.key === 'End') next = order.length - 1;
                else return;
                e.preventDefault();
                selectTab(s,order[next]);
                const button = host.querySelector('[data-tab="'+order[next]+'"]');
                if (button) button.focus();
                return;
            }
            if (e.target !== host && !host.contains(e.target)) return;
            if (e.target.closest('button, input, select, textarea, a, [role="tab"], [role="button"], [contenteditable="true"]')) return;
            const tag = (e.target.tagName || '').toLowerCase();
            if (tag === 'input' || tag === 'textarea' || tag === 'select' || e.target.isContentEditable) return;
            if (e.key === ' ' || e.key === 'k' || e.key === 'K') {
                e.preventDefault();
                const st=s.data&&s.data.state;
                const live=st&&(st.status==='playing'||st.status==='buffering'||st.status==='ready');
                action(s,live&&R.owned()?'pause':'start').catch(err=>notice(s,err));
                return;
            }
            if (e.key === 'n' || e.key === 'N') { e.preventDefault(); action(s,'skip').catch(err => notice(s,err)); return; }
            if (e.key === 's' || e.key === 'S') { e.preventDefault(); action(s,'stop').catch(err => notice(s,err)); return; }
        },{ signal:s.abort.signal });
        s.unsubscribe = R.subscribe((data,error) => { if (s.disposed) return; s.data=data; draw(s,false); if(error) notice(s,Error(error),false); });
        R.init(ctx).catch(err=>notice(s,err));
        if(ctx.setWindowMenus) ctx.setWindowMenus(id,[{label:t('station'),items:[{label:t('new_station'),action:()=>edit(s,true)},{label:t('settings'),action:()=>edit(s,false)},{label:t('stop'),action:()=>R.control('stop').catch(err=>notice(s,err))}]}]);
    }
    function selectTab(s,tab) { if (!tab || s.tab === tab) return; s.tab = tab; s.editing = false; s.subview = false; draw(s,true); }
    function say(s,message) { const el=s.q('notice-text'); if(!el)return; el.textContent=message; s.q('notice').hidden=false; }
    function notice(s,err,persist=true) {
        const el=s.q('notice'); if(!el)return;
        const code=String(err.message||'');
        if(persist) s.error = code;
        el.setAttribute('role',persist?'alert':'status');
        el.classList.toggle('is-error',persist);
        const known=['radio_more_music_needed','radio_limit','radio_device_busy','radio_audio_unlock','radio_invalid_genres','radio_region_required','radio_conflict','radio_format_unsupported','radio_production_too_slow','radio_music_unavailable','radio_registry_unavailable','radio_generation_failed','radio_editorial_failed','radio_tts_unavailable','radio_llm_unavailable','radio_unavailable'];
        say(s,s.t(known.includes(code)?code:'error'));
    }
    function station(s) { return s.data && (s.data.stations||[]).find(x=>x.id===s.selected); }
    function draw(s,force) {
        if(!s.data)return;
        const d=s.data,st=d.state||{},p=station(s),active=!!(p&&st.station_id===p.id),current=active&&(st.queue||[]).find(x=>x.id===st.current),list=d.stations||[];
        if(!s.selected&&list.length){s.selected=list[0].id;return draw(s,true);}
        const options=list.map(x=>'<option value="'+s.esc(x.id)+'">'+s.esc(x.name)+'</option>').join('');
        if(s.options!==options){s.q('station').innerHTML=options;s.options=options;} s.q('station').value=s.selected;
        setText(s.q('state'),s.t(active?st.status:'stopped'));
        s.q('status').className='pr-status '+(STATUS[st.status]||'is-stopped')+(active?'':' is-idle');
        s.host.querySelector('.pr-record').classList.toggle('is-playing',active&&st.status==='playing');
        setText(s.q('title'),current?current.title:p?p.name:s.t('welcome')); setText(s.q('kind'),current?s.t(current.kind):s.t('now'));
        setText(s.q('theme'),active&&st.theme?st.theme:p?p.topics:s.t('setup_hint'));
        const position=R.position();s.q('progress').max=position.duration||1;s.q('progress').value=position.position||0;
        setText(s.q('elapsed'),clock(position.position)); setText(s.q('duration'),clock(position.duration));
        const requiredMinutes=active?Math.ceil(st.required_ms/60000):p?p.reserve_minutes:0;
        setText(s.q('reserve'),(active?Math.floor(st.buffer_ms/60000):0)+(requiredMinutes?' / '+requiredMinutes:'')+' '+s.t('minutes'));
        s.q('buffer').max=st.required_ms||(p?p.min_tracks:2);s.q('buffer').value=active?(st.required_ms?st.buffer_ms:st.track_count):0;
        const jobs=active?(st.music_busy?s.t('generating'):st.editor_busy?s.t('editing'):st.relaxed?s.t('relaxed'):''):'';
        setText(s.q('jobs'),jobs);s.q('jobs').hidden=!jobs;
        preparationStatus(s, p, st, active);
        const next=active&&st.next_news&&new Date(st.next_news);const newsTime=next&&next.getFullYear()>2000?s.t('next_news')+' '+next.toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'}):'';
        setText(s.q('news-time'),newsTime);s.q('news-time').hidden=!newsTime;
        const readonly=s.ctx.readonly||d.capabilities&&d.capabilities.read_only;
        for(const name of ['new','settings','start'])s.host.querySelector('[data-action="'+name+'"]').disabled=readonly||!d.available||(name!=='new'&&!p)||s.busy;
        const starting = !!R.startingStation;
        s.q('station').disabled = starting;
        for(const name of ['new','settings','start'])if(starting)s.host.querySelector('[data-action="'+name+'"]').disabled=true;
        if(active && R.owned() && st.status!=='stopped' && st.status!=='paused')s.host.querySelector('[data-action="start"]').disabled=true;
        for(const name of ['pause','stop','skip'])s.host.querySelector('[data-action="'+name+'"]').disabled=readonly||!R.owned()||!active||s.busy;
        if (!(st.queue || []).length) s.host.querySelector('[data-action="skip"]').disabled = true;
        s.host.querySelector('[data-action="pause"]').disabled = readonly || !R.owned() || !active || st.status==='preparing' || s.busy;
        const label=starting?s.t('starting'):active&&st.status==='paused'?s.t('resume'):s.t('start');
        setText(s.q('start-label'),label);
        s.host.querySelector('[data-action="start"]').setAttribute('aria-busy', String(starting));
        const paused=active&&st.status==='paused';
        const pauseButton=s.host.querySelector('[data-action="pause"]');
        pauseButton.innerHTML=paused?ICON.play:ICON.pause;
        pauseButton.setAttribute('aria-label',s.t(paused?'resume':'pause'));pauseButton.title=s.t(paused?'resume':'pause');
        if(!d.available)notice(s,Error('radio_unavailable'),false);else if(active&&st.library_status==='failed')notice(s,Error('radio_registry_unavailable'),false);else if(active&&st.code)notice(s,Error(st.code),false);else if(active&&st.editorial_code)notice(s,Error(st.editorial_code),false);else if(!s.busy&&!s.error)s.q('notice').hidden=true;
        if(s.editing||s.subview)return;
        const signature=JSON.stringify([s.selected,s.tab,s.tab==='program'&&active?st.queue:[],s.tab==='news'&&active?st.news:[],st.news_code]);
        if(force||s.signature!==signature){s.signature=signature;content(s);}
        s.host.querySelectorAll('[data-tab]').forEach(el=>{
            const on=el.dataset.tab===s.tab;
            el.classList.toggle('is-active',on);el.setAttribute('aria-selected',String(on));el.tabIndex=on?0:-1;
        });
        s.q('content').setAttribute('aria-labelledby',s.uid+'-tab-'+s.tab);
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
        else if (st.library_status === 'searching') key = 'library_searching';
        else if (st.library_status === 'importing') key = 'library_loading';
        else if (st.library_status === 'failed') key = 'radio_registry_unavailable';
        else if (st.music_busy) key = 'generating';
        const tracks = active ? st.track_count || 0 : 0, requiredTracks = p.min_tracks;
        const milliseconds = active ? st.buffer_ms || 0 : 0, requiredMS = active && st.required_ms || p.reserve_minutes * 60000;
        const number = value => new Intl.NumberFormat(window.SYSTEM_LANG || 'en', { maximumFractionDigits: 1 }).format(value);
        const counts = s.ctx.t('personalRadio.'+(requiredMS?'startup_counts':'startup_tracks'), { tracks, requiredTracks, minutes: number(milliseconds / 60000), requiredMinutes: number(requiredMS / 60000) });
        const text = (name, value) => { const el = s.q(name); if (el.textContent !== value) el.textContent = value; };
        text('preparation-title', s.t(key)); text('preparation-counts', counts);
        text('preparation-hint', s.t(p.mode === 'local' ? 'startup_local_hint' : 'startup_hint'));
        s.q('preparation-progress').value = Math.min(1, requiredMS ? milliseconds / requiredMS : 1, tracks / requiredTracks);
        s.q('preparation-progress').setAttribute('aria-label', counts);
        s.q('preparation').classList.toggle('is-busy', !!(starting || st.music_busy || st.editor_busy || ['searching','importing'].includes(st.library_status)));
    }
    function content(s) {
        const host=s.q('content'),p=station(s),st=s.data.state||{},t=s.t,e=s.esc;
        if(!p){host.innerHTML='<div class="pr-empty"><div class="pr-empty-mark" aria-hidden="true"></div><h2>'+e(t('welcome'))+'</h2><p>'+e(t('setup_hint'))+'</p><button type="button" class="pr-primary" data-action="new">'+e(t('new_station'))+'</button></div>';return;}
        if(s.tab==='library'){library(s);return;}
        if(s.tab==='news'){
            const items=st.station_id===p.id?st.news||[]:s.newsCache&&s.newsCache[p.id]||[];
            if(st.station_id!==p.id&&!(s.newsCache&&s.newsCache[p.id])){host.innerHTML='<h2>'+e(t('news'))+'</h2><div class="pr-loading" role="status">'+e(t('loading'))+'</div>';R.request('stations/'+p.id+'/news').then(data=>{s.newsCache=s.newsCache||{};s.newsCache[p.id]=data.items||[];if(!s.disposed&&s.tab==='news'&&s.selected===p.id)content(s);}).catch(err=>notice(s,err));return;}
            host.innerHTML='<h2>'+e(t('news'))+'</h2>'+(st.news_code?'<p class="pr-hint">'+e(t('news_failed'))+'</p>':'')+(items.length?'<div class="pr-bulletins">'+items.map(x=>'<article class="pr-bulletin"><time>'+e(new Date(x.due).toLocaleString())+'</time><p>'+e(x.text)+'</p><div class="pr-sources">'+(x.sources||[]).map(src=>{let url;try{url=new URL(src.url);}catch(_){return '';}return ['https:','http:'].includes(url.protocol)?'<a target="_blank" rel="noopener noreferrer" href="'+e(url.href)+'">'+e(src.title)+'</a>':'';}).join('')+'</div></article>').join('')+'</div>':'<div class="pr-empty"><p>'+e(t(p.news_minutes?'no_news':'news_off'))+'</p></div>');return;
        }
        const queue=st.station_id===p.id?st.queue||[]:[];
        host.innerHTML='<div class="pr-section-head"><h2>'+e(t('program'))+'</h2><span class="pr-count">'+queue.length+'</span></div>'+(queue.length?'<ol class="pr-queue">'+queue.map((x,i)=>'<li'+(st.current===x.id?' class="is-current" aria-current="true"':'')+'><span class="pr-order">'+(i+1)+'</span><span class="pr-type">'+e(t(x.kind))+'</span><div class="pr-queue-text"><strong>'+e(x.title)+'</strong>'+(x.text?'<p>'+e(x.text)+'</p>':'')+'</div><time>'+clock(x.duration_ms)+'</time></li>').join('')+'</ol>':'<div class="pr-empty"><p>'+e(t('queue_empty'))+'</p></div>')+'<div class="pr-capabilities">'+['music','tts','llm','news'].map(key=>'<span class="'+(s.data.capabilities[key]?'is-ready':'')+'">'+(s.data.capabilities[key]?'● ':'○ ')+e(t(key==='llm'?'editor':key))+'</span>').join('')+'</div>';
    }
    async function library(s) {
        s.subview=false;const p=station(s);if(!p)return;const id=p.id,host=s.q('content'),t=s.t,e=s.esc;
        const genre=s.importGenre||'';
        host.innerHTML='<div class="pr-section-head"><h2>'+e(t('library'))+'</h2><span class="pr-count" data-pr="track-count">0</span></div>'+
            '<div class="pr-library-bar"><label class="pr-field"><span>'+e(t('import_genre'))+'</span><select data-pr-import-genre><option value="">'+e(t('any_genre'))+'</option>'+p.genres.map(g=>'<option value="'+e(g.name)+'"'+(g.name===genre?' selected':'')+'>'+e(g.name)+'</option>').join('')+'</select></label>'+
            '<label class="pr-field pr-search"><span class="pr-visually-hidden">'+e(ctx_text(s,'desktop.search'))+'</span><input type="search" data-pr-library-query placeholder="'+e(ctx_text(s,'desktop.search'))+'" value="'+e(s.libraryQuery||'')+'"></label></div>'+
            '<div class="pr-library-actions"><button type="button" data-action="upload">'+e(t('upload'))+'</button><button type="button" data-action="desktop_file">'+e(t('desktop_file'))+'</button><button type="button" data-action="existing">'+e(t('existing'))+'</button><button type="button" data-action="folder">'+e(t('folder'))+'</button></div>'+
            '<p class="pr-hint">'+e(t('import_hint'))+'</p>'+
            '<div class="pr-storage"><button type="button" class="pr-quiet" data-action="cleanup">'+e(t('cleanup'))+'</button></div>'+
            '<div data-pr-tracks class="pr-tracks"><div class="pr-loading" role="status">'+e(t('loading'))+'</div></div><input type="file" data-pr-upload multiple accept=".mp3,.wav" hidden>';
        host.querySelector('[data-pr-upload]').addEventListener('change',e=>uploads(s,Array.from(e.target.files||[])).catch(err=>notice(s,err)));
        const genreSelect=host.querySelector('[data-pr-import-genre]');
        if(genreSelect)genreSelect.addEventListener('change',e=>{s.importGenre=e.target.value;});
        const query=host.querySelector('[data-pr-library-query]');
        if(query)query.addEventListener('input',e=>{s.libraryQuery=e.target.value;renderTracks(s);});
        try{
            const data=await R.request('stations/'+id+'/tracks');
            if(s.disposed||s.selected!==id||s.tab!=='library'||s.editing||s.subview)return;
            s.libraryItems=data.items||[];
            renderTracks(s);
        }catch(err){notice(s,err);}
    }
    function ctx_text(s,key){ return s.ctx.t(key); }
    function renderTracks(s) {
        const host=s.q('content');if(!host)return;
        const box=host.querySelector('[data-pr-tracks]');if(!box)return;
        const items=s.libraryItems||[],query=(s.libraryQuery||'').trim().toLowerCase();
        const filtered=query?items.filter(x=>String(x.title||'').toLowerCase().includes(query)):items;
        const counter=host.querySelector('[data-pr="track-count"]');
        if(counter)counter.textContent=String(items.length);
        if(!items.length){box.innerHTML='<div class="pr-empty"><p>'+s.esc(s.t('no_music'))+'</p></div>';return;}
        if(!filtered.length){box.innerHTML='<div class="pr-empty"><p>'+s.esc(s.t('no_music'))+'</p></div>';return;}
        const t=s.t,e=s.esc;
        box.innerHTML=filtered.map(x=>'<div class="pr-track"><button type="button" class="pr-track-fav'+(x.favorite?' is-on':'')+'" data-track="'+e(x.id)+'" data-op="favorite" aria-pressed="'+(x.favorite?'true':'false')+'" aria-label="'+e(t('favorite'))+'" title="'+e(t('favorite'))+'">'+(x.favorite?'★':'☆')+'</button>'+
            '<div class="pr-track-text"><strong>'+e(x.title)+'</strong><small>'+e(t(x.origin))+' · '+Math.ceil(x.duration_ms/60000)+' '+e(t('minutes'))+' · '+x.plays+' '+e(t('plays'))+'</small></div>'+
            '<div class="pr-track-actions"><button type="button" data-track="'+e(x.id)+'" data-op="weight">'+e(t('less'))+'</button><button type="button" data-track="'+e(x.id)+'" data-op="blocked">'+e(t(x.blocked?'unblock':'block'))+'</button><button type="button" class="pr-track-remove" data-track="'+e(x.id)+'" data-op="remove" aria-label="'+e(t('remove'))+'" title="'+e(t('remove'))+'">×</button></div></div>').join('');
        box.querySelectorAll('[data-track]').forEach(button=>{
            button.disabled=!!s.ctx.readonly;
            button.addEventListener('click',async()=>{
                const x=(s.libraryItems||[]).find(y=>y.id===button.dataset.track),op=button.dataset.op;if(!x)return;
                const focus={track:button.dataset.track,op:button.dataset.op};
                button.disabled=true;
                try{if(op==='remove'){await R.request('stations/'+s.selected+'/tracks/'+x.id,'DELETE');}else{const body={favorite:x.favorite,blocked:x.blocked,weight:x.weight};if(op==='weight')body.weight=25;else body[op]=!body[op];await R.request('stations/'+s.selected+'/tracks/'+x.id,'PATCH',body);}
                    const data=await R.request('stations/'+s.selected+'/tracks');
                    if(s.disposed)return;
                    s.libraryItems=data.items||[];
                    renderTracks(s);
                    const again=s.q('content').querySelector('[data-track="'+focus.track+'"][data-op="'+focus.op+'"]');
                    if(again)again.focus();
                }catch(err){button.disabled=false;notice(s,err);}
            });
        });
    }
    function importGenre(s) { const input=s.host.querySelector('[data-pr-import-genre]'); return input&&input.value!==undefined?input.value:(s.importGenre||''); }
    async function uploads(s,files) {
        if(s.busy||s.ctx.readonly)return;s.busy=true;const p=station(s);
        try{for(let i=0;i<files.length;i++){say(s,s.t('importing')+' '+(i+1)+' / '+files.length);const file=files[i];await s.ctx.api('/api/desktop/personal-radio/stations/'+p.id+'/upload?name='+encodeURIComponent(file.name)+'&genre='+encodeURIComponent(importGenre(s)),{method:'POST',body:file,signal:s.abort.signal});}await library(s);await R.refresh();}finally{s.busy=false;draw(s,false);}
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
        if(name==='dismiss'){s.error='';s.q('notice').hidden=true;return;}
        if(s.ctx.readonly&&name!=='existing')return;
        s.error='';s.q('notice').hidden=true; if(name==='new')return edit(s,true);if(name==='settings')return edit(s,false);
        const p=station(s);if(!p)return;
        if(name==='start'){if(s.preview)s.preview.pause();if(s.data.state.station_id===p.id&&s.data.state.status==='paused'&&R.owned())return R.control('resume');try{await R.start(p.id,false);}catch(err){if(err.message==='radio_device_busy'&&s.ctx.confirmDialog&&await s.ctx.confirmDialog('Personal Radio',s.t('takeover'))){await R.start(p.id,true);}else throw err;}return;}
        if(['pause','stop','skip'].includes(name))return R.control(name);
        if(name==='upload')return s.host.querySelector('[data-pr-upload]').click();
        if(name==='desktop_file'){const picked=await s.ctx.openFileDialog({title:s.t('desktop_file'),path:'Documents',filters:[{label:s.t('music'),extensions:['.mp3','.wav']}],accept:'.mp3,.wav'});if(picked&&!picked.canceled){const path=typeof picked==='string'?picked:picked.path;if(path){await R.request('stations/'+p.id+'/imports','POST',{path,genre:importGenre(s)});await library(s);}}return;}
        if(name==='existing'){
            s.subview=true;
            const host=s.q('content');host.innerHTML='<div class="pr-section-head"><h2>'+s.esc(s.t('existing'))+'</h2><button type="button" data-action="library">'+s.esc(s.t('back'))+'</button></div><div data-existing class="pr-import-list"></div><button type="button" data-more class="pr-quiet">'+s.esc(s.t('more'))+'</button>';
            let offset=0;const load=async()=>{const data=await R.request('library?offset='+offset);if(s.disposed||!s.subview||!host.querySelector('[data-existing]'))return;offset+=data.items.length;const box=host.querySelector('[data-existing]');for(const x of data.items){const row=document.createElement('button');row.className='pr-import-row';row.textContent='＋ '+(x.title||x.filename);row.disabled=!!s.ctx.readonly;row.onclick=async()=>{row.disabled=true;try{await R.request('stations/'+p.id+'/imports','POST',{media_id:x.id,genre:importGenre(s)});row.textContent='✓ '+(x.title||x.filename);}catch(err){row.disabled=false;notice(s,err);}};box.appendChild(row);}host.querySelector('[data-more]').hidden=offset>=data.total;};host.querySelector('[data-more]').onclick=()=>load().catch(err=>notice(s,err));await load();return;
        }
        if(name==='cleanup'){if(await s.ctx.confirmDialog(s.t('cleanup'),s.t('cleanup_confirm'))){await R.request('unused','DELETE');await library(s);}return;}
        if(name==='folder'){
            const picked=await s.ctx.openFileDialog({title:s.t('folder'),mode:'select-folder',path:'Music'});
            if(!picked||picked.canceled)return;
            s.busy=true;let offset=0,more=true,count=0;
            try{while(more&&!s.disposed){const files=await R.request('files?path='+encodeURIComponent(picked.path)+'&offset='+offset);for(const path of files.paths){await R.request('stations/'+p.id+'/imports','POST',{path,genre:importGenre(s)},s.abort.signal);count++;say(s,s.t('importing')+' '+count);}more=files.more;offset=files.next_offset;}await library(s);await R.refresh();}finally{s.busy=false;draw(s,false);}return;
        }
        if(name==='preview'){
            if(s.preview){s.preview.pause();URL.revokeObjectURL(s.previewURL);}
            const button=s.q('content')&&s.q('content').querySelector('[data-action="preview"]');
            if(button)button.disabled=true;
            try{
                const response=await fetch('/api/desktop/personal-radio/stations/'+p.id+'/preview',{method:'POST',credentials:'same-origin',signal:s.abort.signal});
                if(!response.ok)throw Error('radio_tts_unavailable');const blob=await response.blob();if(s.disposed)return;
                s.previewURL=URL.createObjectURL(blob);s.preview=new Audio(s.previewURL);
                s.preview.onended=()=>{if(button&&!s.disposed)button.disabled=false;};
                await s.preview.play();
            }catch(err){if(button&&!s.disposed)button.disabled=false;throw err;}
            return;
        }
        if(name==='delete'){if(await s.ctx.confirmDialog(p.name,s.t('delete_confirm'))){await s.ctx.api('/api/desktop/personal-radio/stations/'+p.id,{method:'DELETE',headers:{'If-Match':String(p.revision)}});s.selected='';s.editing=false;await R.refresh();draw(s,true);}return;}
        if(name==='library')return library(s);
    }
    window.PersonalRadioApp={render,dispose};
})();
