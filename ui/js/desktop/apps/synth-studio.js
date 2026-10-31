(function () {
    'use strict';
    const instances=new Map(), BAR=1920, PPQ=480, MAX_TICK=10000000, MAX_EVENTS=50000, MAX_CLIPS=256, MAX_PROJECT_BYTES=5*1024*1024, RECORDING_TICKS=BAR*64;
    const escape=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    function utf8ByteLength(value){if(typeof TextEncoder==='function')return new TextEncoder().encode(value).byteLength;let bytes=0;for(const character of value){const code=character.codePointAt(0);bytes+=code<=0x7f?1:code<=0x7ff?2:code<=0xffff?3:4;}return bytes;}
    function render(host,id,context) {
        dispose(id);
        const ctx=context||{}, M=window.SynthStudioModel, P=window.SynthStudioPresets, life=new AbortController(), signal=life.signal;
        const tr=(key,params)=>ctx.t?.('synthStudio.'+key,params)||key, esc=ctx.esc||escape;
        const icon=(name,fallback)=>ctx.iconMarkup?.(name,fallback,'ss-icon',16,'action')||fallback;
        const button=(action,key,symbol,extra='')=>'<button type="button" data-action="'+action+'" title="'+esc(tr(key))+'" aria-label="'+esc(tr(key))+'" '+extra+'>'+symbol+'</button>';
        let history=M.history(M.create(tr('untitled'))), currentProject=history.current(), audio=window.SynthStudioAudio.create(), storage, editor, disposed=false, busy=false, playing=false, recording=null, cursor=0;
        let category='all', libraryMode='instruments', revoked=false, selectedInstrument='bass-sub', countIn=true, quantize=120, latency=0, metronome=false;
        let activeTag='', tourIndex=-1;
        const LIBRARY_KEY='aura:synth-studio:library';
        let libraryPrefs={favourites:[],recent:[]};
        try{const stored=JSON.parse(localStorage.getItem(LIBRARY_KEY)||'{}');if(stored&&typeof stored==='object'){if(Array.isArray(stored.favourites))libraryPrefs.favourites=stored.favourites;if(Array.isArray(stored.recent))libraryPrefs.recent=stored.recent;}}catch(_){}
        const saveLibraryPrefs=()=>{try{localStorage.setItem(LIBRARY_KEY,JSON.stringify(libraryPrefs));}catch(_){}};
        const FX_LABELS={chorus:'chorus',phaser:'phaser',delay:'delay',reverb:'reverb',gate:'gate',tremolo:'fx_tremolo',flanger:'fx_flanger',rotary:'fx_rotary',bitcrush:'fx_bitcrush',softclip:'fx_softclip',eq3:'fx_eq3',width:'fx_width',cabinet:'fx_cabinet'};
        const FX_PARAM_LABELS={mix:'fx_mix',rate:'fx_rate',depth:'fx_depth',feedback:'fx_feedback',amount:'fx_amount',freq:'fx_freq',low:'fx_low',mid:'fx_mid',high:'fx_high',bits:'fx_bits',time:'fx_time',size:'fx_size',decay:'fx_decay',model:'fx_model',tone:'tone'};
        const TOUR_STEPS=[{panel:'notes',key:'tour_arrange'},{panel:'sound',key:'tour_sound'},{panel:'performance',key:'tour_performance'},{key:'tour_record'},{key:'tour_export'}];
        let workspace, composer, stage, parameterGesture=null;
        let live=new Map(), storageState={}, midiState={}, generation=0, ready=false;
        const timers=new Set(), later=(fn,delay)=>{const timer=setTimeout(()=>{timers.delete(timer);fn();},delay);timers.add(timer);};
        const eventCount=p=>p.tracks.reduce((total,t)=>total+(t.automation?.length||0)+t.clips.reduce((clipTotal,c)=>clipTotal+c.notes.length+c.controllers.length,0),0);
        const readonly=()=>!!ctx.readonly||revoked, blocked=()=>disposed||!ready||readonly()||busy||!!storageState.loading||!!recording||!!stage?.active();
        // Keep one validated view snapshot; MIDI packets must not clone the whole song.
        const project=()=>currentProject, track=()=>project().tracks.find(t=>t.id===editor?.selection().trackId);
        window.AuraDesktopEmail?.attach(id, '.ss-file-actions', () => ({
            key:currentProject,signal,disabled:blocked(),subject:project().name,formats:['wav','midi',{id:'project',label:'Synth Studio (.aurasynth)'}],
            createFiles:async format=>format==='project'?[new File([JSON.stringify(M.validate(M.clone(project())))],(project().name.replace(/[<>:"/\\|?*]/g,'_')||'Synth Studio')+'.aurasynth',{type:'application/json'})]:exportFile(format,true)
        }));
        const fail=error=>{if(disposed)return;let code=typeof error==='string'?error:error?.code;if(code?.startsWith('E_PROJECT_'))code=code==='E_PROJECT_LIMIT'?'too_many_tracks':'invalid_project';else if(code==='E_MIDI_DENIED')code='denied';else if(code==='E_MIDI_UNSUPPORTED')code='unsupported';else if(code?.startsWith('E_MIDI_DEVICE'))code='no_device';else if(code?.startsWith('E_MIDI_'))code='unsupported_midi';else if(code==='SYNTH_STUDIO_RENDER_BUDGET')code='export_limit';notice(tr(['unsupported','denied','no_device','invalid_project','unsupported_midi','too_many_tracks','export_limit','readonly'].includes(code)?code:'error'));};
        function notice(text){const n=host.querySelector('[data-notice]');if(n){n.textContent=text||'';n.hidden=!text;}}
        host.innerHTML='<div class="ss-app" tabindex="0"><header class="ss-header"><div class="ss-brand"><span class="ss-brand-mark" aria-hidden="true">≋</span><div><strong>Synth Studio</strong><button class="ss-project-name" data-action="rename" data-project-name></button></div></div><nav class="ss-file-actions">'+button('new','new',icon('file-plus','＋'))+button('open','open',icon('folder-open','▱'))+button('save','save',icon('save','▣'))+button('saveAs','save_as',icon('copy','⧉'))+'<select data-export aria-label="'+esc(tr('export'))+'"><option value="">'+esc(tr('export'))+'</option><option value="wav">WAV</option><option value="midi">MIDI</option></select>'+button('demo','demo',tr('demo'))+'</nav></header>'+
            '<div class="ss-transport"><div class="ss-transport-buttons">'+button('play','play','▶','class="ss-play"')+button('pause','pause','Ⅱ')+button('stop','stop','■')+button('record','record','●','class="ss-record"')+'</div><output class="ss-position" data-position>01 : 01</output><label class="ss-tempo"><input data-tempo type="number" inputmode="numeric" enterkeyhint="done" min="40" max="240" value="120" aria-label="'+esc(tr('tempo'))+'"><span>BPM</span></label><span class="ss-time-signature">4/4</span><label><input data-metronome type="checkbox"> '+esc(tr('metronome'))+'</label><label><input data-loop type="checkbox"> '+esc(tr('loop'))+'</label><label class="ss-loop-range"><input data-loop-start type="number" inputmode="numeric" enterkeyhint="done" min="1" value="1" aria-label="'+esc(tr('loop_start'))+'"> — <input data-loop-end type="number" inputmode="numeric" enterkeyhint="done" min="2" value="5" aria-label="'+esc(tr('loop_end'))+'"></label><div class="ss-history">'+button('undo','undo','↶')+button('redo','redo','↷')+'</div><button type="button" class="ss-master-toggle" data-action="master" title="'+esc(tr('fx_master'))+'" aria-label="'+esc(tr('fx_master'))+'" aria-expanded="false" aria-controls="ss-master-panel">'+esc(tr('fx_master'))+'</button><canvas class="ss-analyser" data-analyser width="320" height="54" role="img" aria-label="'+esc(tr('analyser'))+'"></canvas></div>'+
            '<section class="ss-master-panel" id="ss-master-panel" data-master-panel hidden aria-label="'+esc(tr('fx_master'))+'"></section>'+
            '<div class="ss-notice" data-notice role="status" hidden></div><div class="ss-workspace"><aside class="ss-library"><div class="ss-library-tabs"><button data-library-mode="instruments" aria-pressed="true">'+esc(tr('instruments'))+'</button><button data-library-mode="patterns" aria-pressed="false">'+esc(tr('patterns'))+'</button></div><input class="ss-search" data-search type="search" inputmode="search" enterkeyhint="search" placeholder="'+esc(tr('search'))+'" aria-label="'+esc(tr('search'))+'"><div class="ss-categories" data-categories></div><div class="ss-library-tags" data-library-tags></div><div class="ss-library-list" data-library-list></div><button class="ss-midi-toggle" data-action="midi">⌨ '+esc(tr('midi_connect'))+'</button></aside><main class="ss-main"><div class="ss-arrange-tools"><span>'+esc(tr('arrangement'))+'</span><label>'+esc(tr('snap'))+' <select data-snap><option value="480">1/4</option><option value="240">1/8</option><option value="120" selected>1/16</option><option value="60">1/32</option></select></label><label>'+esc(tr('zoom'))+' <input type="range" data-zoom min="20" max="100" value="40" aria-label="'+esc(tr('zoom'))+'"></label>'+button('inspector','your_sound','☷','class="ss-inspector-toggle"')+button('library','library','♫','class="ss-compact-only"')+'</div><div class="ss-editor-host" data-editor-host></div></main><aside class="ss-inspector" data-inspector></aside></div>'+
            '<section class="ss-midi-panel" data-midi-panel hidden><strong>'+esc(tr('midi_title'))+'</strong><button data-action="connect">'+esc(tr('midi_connect'))+'</button><select data-midi-input aria-label="'+esc(tr('midi_device'))+'"><option value="">'+esc(tr('midi_device'))+'</option></select><span data-midi-state role="status"></span><label><input data-count-in type="checkbox" checked> '+esc(tr('count_in'))+'</label><label>'+esc(tr('latency'))+' <input data-latency type="number" inputmode="numeric" enterkeyhint="done" min="-200" max="200" step="1" value="0"> ms</label><label><input data-record-quantize type="checkbox" checked> '+esc(tr('quantize'))+'</label>'+button('importMidi','import_midi',tr('import_midi'))+'</section>'+
            '<div class="ss-tour" data-tour hidden></div>'+
            '<footer class="ss-footer"><span data-save-state role="status"></span><span class="ss-footer-hint">'+esc(tr('shortcuts'))+'</span><span class="ss-midi-led" data-midi-led aria-hidden="true"></span><span data-footer-count></span><button type="button" class="ss-tour-button" data-action="tour">'+esc(tr('tour'))+'</button></footer><input type="file" data-midi-file accept=".mid,.midi,audio/midi" hidden></div>';
        const root=host.firstElementChild, find=q=>root.querySelector(q);
        editor=window.SynthStudioEditor.create(find('[data-editor-host]'),{getProject:project,readonly:blocked,tr,mutate,onTrackVolume:true,onSelect:()=>{renderInspector();composer?.refresh();stage?.refresh();},onAdd:()=>insert({type:'instrument',id:selectedInstrument},'',0),onDrop:insert,onNewClip:newClip,onSeek:seek,onAudition:p=>audition(p),onNoteOn:p=>noteOn(p,.8,performance.now(),17),onNoteOff:p=>noteOff(p,performance.now(),17),onError:fail});
        workspace=window.SynthStudioWorkspace.create(root,{tr,signal,project,blocked,onTour:()=>{tourIndex=0;renderTour();}});
        composer=window.SynthStudioCompose.create(workspace.performance,{tr,esc,signal,project,selection:()=>editor.selection(),cursor:()=>cursor,blocked,mutate,audio,fail,stopTransport:()=>stop(false,true),showNotes:()=>workspace.panel('notes'),onArpNote:captureArp});
        const midi=window.SynthStudioMIDI.create({onNoteOn:e=>noteOn(e.pitch,e.velocity,e.time,e.channel),onNoteOff:e=>noteOff(e.pitch,e.time,e.channel),onController:e=>controller(e),onState:s=>{midiState=s;if(s.status!=='connected'){if(recording)stop();else panic();}renderMIDI();}});
        stage=window.SynthStudioStage.create(workspace.performance,{tr,esc,signal,audio,project,track,mutate,playing:()=>playing,cursor:()=>cursor,editable:()=>!disposed&&ready&&!readonly()&&!busy&&!storageState.loading&&!recording,canPlay:()=>!disposed&&ready&&!busy&&!storageState.loading,play,stop,refresh,fail,prompt:(title,value)=>ctx.promptDialog?.(title,value),confirm:(title,message)=>ctx.confirmDialog?.(title,message),adopt:p=>storage.newDocument(p,{dirty:true}),transport:find('.ss-transport'),whenReady:()=>storage.ready,sessionKey:ctx.sessionKey||ctx.state?.windows?.get(id)?.sessionKey||id});
        storage=window.SynthStudioStorage.create({...ctx,windowId:id},{serialize:project,validate:M.validate,beforeLoad:()=>stop(),onLoaded:p=>{if(p)loadProject(p);},onState:s=>{const loaded=storageState.loading&&!s.loading;storageState=s;if(loaded)refresh();else status();}});
        function status(){
            if(disposed)return;const p=project();find('[data-project-name]').textContent=p.name;find('[data-project-name]').disabled=blocked();
            find('[data-save-state]').textContent=readonly()?tr('readonly'):storageState.legacyCopy?tr('legacy_copy'):storageState.error?tr('save_error'):storageState.saving?tr('saving'):storageState.dirty?tr('unsaved'):storageState.path?tr('saved'):tr('local_draft');
            find('[data-action="record"]').classList.toggle('is-recording',!!recording);find('[data-action="record"]').setAttribute('aria-pressed',String(!!recording));
            find('[data-action="play"]').classList.toggle('is-active',playing);find('[data-tempo]').value=p.tempo;find('[data-loop]').checked=p.loop.enabled;
            find('[data-loop-start]').value=Math.floor(p.loop.start/BAR)+1;find('[data-loop-end]').value=Math.floor(p.loop.end/BAR)+1;
            for(const a of ['new','save','saveAs','demo','record','importMidi'])find('[data-action="'+a+'"]')?.toggleAttribute('disabled',!ready||readonly()||busy||!!storageState.loading);
            find('[data-action="open"]').disabled=!ready||busy||!!storageState.loading;find('[data-action="play"]').disabled=!ready||busy||!!storageState.loading;find('[data-tempo]').disabled=blocked();find('[data-loop]').disabled=blocked();find('[data-loop-start]').disabled=blocked();find('[data-loop-end]').disabled=blocked();
            find('[data-action="undo"]').disabled=blocked()||!history.canUndo();find('[data-action="redo"]').disabled=blocked()||!history.canRedo();find('[data-export]').disabled=busy||!!recording||!!stage?.active();root.querySelectorAll('[data-insert]').forEach(el=>el.disabled=blocked());
            const item=ctx.state?.windows?.get(id);if(item){item.context={...(item.context||{}),path:storageState.path||''};if(item.element)item.element.classList.toggle('ss-dirty',!!storageState.dirty);}
            const counter=find('[data-footer-count]');if(counter)counter.textContent=(P.presets?.length||0)+' SYNTHS · '+(M.FX_TYPES?.length||0)+' FX · '+(P.gm?.length||0)+' GM';
        }
        function refresh(){
            const active=document.activeElement,attrs=['data-param','data-track-name','data-instrument','data-track-volume','data-velocity','data-master-param'],attr=attrs.find(a=>active?.hasAttribute(a)),value=attr?active.getAttribute(attr):'';
            editor.render();renderInspector();renderMaster();status();workspace?.refresh();composer?.refresh();stage?.refresh();
            if(attr)root.querySelectorAll('['+attr+']').forEach(el=>{if(el.getAttribute(attr)===value)el.focus({preventScroll:true});});
        }
        function loadProject(p){stop();history=M.history(M.upgrade(p));currentProject=history.current();cursor=0;editor.select(p.tracks[0]?.id||'',p.tracks[0]?.clips[0]?.id||'');workspace.performance.scrollTop=0;refresh();}
        function mutate(fn,kind='timeline'){
            if(blocked())return;
            try{const next=M.clone(project());fn(next);M.validate(next);if(playing){if(kind==='mix')audio.updateTracks(next);else if(kind==='automation')audio.setAutomation(next);else if(kind!=='metadata'){audio.queueProject({...stage.runtime(next),metronome},undefined,()=>{notice('');stage.sceneApplied();});notice(tr('change_queued'));}}history.commit(next);currentProject=history.current();storage.changed();refresh();}catch(error){fail(error);}
        }
        function updatedHistory(direction){if(blocked())return;stop();history[direction]();currentProject=history.current();storage.changed();refresh();}
        function insert(data,tid,tick){
            if(blocked())return;let cid='',chosen=tid;
            mutate(p=>{
                const pattern=data.type==='pattern'?M.patterns.find(x=>x.id===data.id):null,inst=pattern?.instrument||data.id;
                if(!pattern&&!P.get(inst)&&!['drums-kit','gm-drums'].includes(inst))throw Error('invalid_project');
                const drum=inst.startsWith('drum-')||inst==='drums-kit'||inst==='gm-drums';
                let t=p.tracks.find(t=>t.id===tid);
                if(!t){t=drum?p.tracks.find(t=>t.instrument==='drums-kit'||t.instrument==='gm-drums'||t.instrument.startsWith('drum-')):null;if(!t){t=M.addTrack(p,drum?'drums-kit':inst);t.name=drum?tr('drums'):P.get(inst)?.name||tr('track');}}
                const isDrum=t.instrument==='drums-kit'||t.instrument==='gm-drums'||t.instrument.startsWith('drum-');if(drum!==isDrum&&pattern)throw Error('invalid_project');
                const c=pattern?M.cloneClip(pattern.clip,tick):M.createClip(t.instrument,tick);c.name=pattern?patternName(pattern):P.get(inst)?.name||tr('clip');
                if(!pattern&&inst.startsWith('drum-'))c.notes=[{id:M.newID('note'),pitch:P.get(inst).drumNote,start:0,duration:120,velocity:.8}];
                t.clips.push(c);chosen=t.id;cid=c.id;
            });if(cid)editor.select(chosen,cid);
        }
        function newClip(tid,tick){let cid='';mutate(p=>{const t=p.tracks.find(t=>t.id===tid),c=M.createClip(t.instrument,tick);c.name=tr('clip');t.clips.push(c);cid=c.id;});editor.select(tid,cid);}
        function patternName(pattern){return tr('pattern_'+pattern.id);}
        function markRecent(id){const list=libraryPrefs.recent.filter(x=>x!==id);list.unshift(id);libraryPrefs.recent=list.slice(0,12);saveLibraryPrefs();}
        function renderLibrary(){
            const rail=[{id:'all',name:tr('all')},{id:'favourites',name:tr('lib_favourites')},{id:'recent',name:tr('lib_recent')},...P.categories.map(c=>({id:c.id,name:tr(c.id)})),{id:'gm',name:'General MIDI'}];
            find('[data-categories]').innerHTML=rail.map(c=>'<button data-category="'+esc(c.id)+'" aria-pressed="'+(c.id===category)+'">'+esc(c.name)+'</button>').join('');
            const query=find('[data-search]').value.toLowerCase();
            const base=libraryMode==='patterns'?M.patterns:[...P.presets,...P.gm.map(p=>({...p,category:'gm'})),{id:'gm-drums',name:tr('gm_drums'),category:'gm'}];
            const items=base.map(x=>libraryMode==='patterns'?{...x,name:patternName(x),tags:[]}:{...x,tags:Array.isArray(x.tags)?x.tags:[]})
                .filter(x=>category==='favourites'?libraryPrefs.favourites.includes(x.id):category==='recent'?libraryPrefs.recent.includes(x.id):category==='all'||x.category===category)
                .filter(x=>!query||x.name.toLowerCase().includes(query)||x.tags.some(t=>t.includes(query)))
                .filter(x=>!activeTag||x.tags.includes(activeTag));
            const vocabulary=libraryMode==='instruments'&&Array.isArray(P.tags)?P.tags:[];
            find('[data-library-tags]').innerHTML=vocabulary.map(t=>'<button data-tag="'+esc(t)+'" aria-pressed="'+String(activeTag===t)+'">'+esc(tr('tag_'+t))+'</button>').join('');
            find('[data-library-list]').innerHTML=items.map(x=>{
                const favourite=libraryPrefs.favourites.includes(x.id);
                const chips=x.tags.slice(0,2).map(t=>'<em class="ss-tag">'+esc(tr('tag_'+t))+'</em>').join('');
                return '<div class="ss-library-item '+(x.id===selectedInstrument?'is-selected':'')+'" draggable="'+!readonly()+'" data-library-id="'+esc(x.id)+'" data-library-type="'+(libraryMode==='patterns'?'pattern':'instrument')+'"><button data-preview="'+esc(x.id)+'" aria-label="'+esc(tr('preview')+' '+x.name)+'">'+(libraryMode==='patterns'?'▥':'♪')+'</button><button class="ss-library-name" data-choose="'+esc(x.id)+'"><span>'+esc(x.name)+'</span><small>'+(libraryMode==='patterns'?esc(tr('pattern')):chips)+'</small></button>'+(libraryMode==='patterns'?'':'<button class="ss-favourite" data-favourite="'+esc(x.id)+'" aria-pressed="'+String(favourite)+'" aria-label="'+esc(tr('lib_favourites'))+'">'+(favourite?'★':'☆')+'</button>')+'<button class="ss-insert" data-insert="'+esc(x.id)+'" aria-label="'+esc(tr('add'))+'">＋</button></div>';
            }).join('')||'<p class="ss-muted">'+esc(tr('no_results'))+'</p>';
            root.querySelectorAll('[data-insert]').forEach(el=>el.disabled=blocked());
            root.querySelectorAll('[data-library-mode]').forEach(el=>el.setAttribute('aria-pressed',String(el.dataset.libraryMode===libraryMode)));
        }
        function renderInspector(){
            const t=track(),pane=find('[data-inspector]');if(!t){pane.innerHTML='<div class="ss-inspector-empty"><span>♫</span><strong>'+esc(tr('your_sound'))+'</strong><p>'+esc(tr('inspector_hint'))+'</p></div>';return;}
            const drum=t.instrument==='drums-kit'||t.instrument==='gm-drums'||t.instrument.startsWith('drum-'),items=drum?[{id:'drums-kit',name:tr('drums')},{id:'gm-drums',name:tr('gm_drums')},...P.drums]:[...P.presets.filter(x=>x.category!=='drums'),...P.gm];
            const slider=(key,min,max,step,value)=>'<label class="ss-parameter"><span>'+esc(tr(key))+'</span><input data-param="'+key+'" type="range" min="'+min+'" max="'+max+'" step="'+step+'" value="'+value+'" aria-label="'+esc(tr(key))+'"><output>'+Number(value).toFixed(2)+'</output><button type="button" data-reset-param="'+key+'" title="'+esc(tr('reset'))+'">'+esc(tr('reset'))+'</button></label>';
            const sound=[['pan',-1,1,.01,t.pan],['tone',0,1,.01,t.params.tone],['resonance',0,1,.01,t.params.resonance??0],['drive',0,1,.01,t.params.drive??0],['attack',0,2,.01,t.params.attack],['release',.02,3,.01,t.params.release]];
            const sends=[['reverb',0,1,.01,t.params.reverb],['delay',0,1,.01,t.params.delay]];
            const inserts=Array.isArray(t.fx?.inserts)?t.fx.inserts:[];
            const rack=inserts.map((insert,index)=>{
                const def=M.FX_TYPES.find(x=>x.id===insert.type);if(!def)return '';
                const rows=def.params.map(p=>{const label=FX_PARAM_LABELS[p.key]||p.key;return '<label class="ss-parameter"><span>'+esc(tr(label))+'</span><input data-fx-param="'+index+':'+p.key+'" type="range" min="'+p.min+'" max="'+p.max+'" step="'+p.step+'" value="'+insert[p.key]+'" aria-label="'+esc(tr(label))+'"><output>'+Number(insert[p.key]).toFixed(2)+'</output></label>';}).join('');
                return '<section class="ss-fx-slot" data-fx-slot="'+index+'"><header><select data-fx-type="'+index+'" aria-label="'+esc(tr('fx_chain'))+'">'+M.FX_TYPES.map(x=>'<option value="'+x.id+'"'+(x.id===insert.type?' selected':'')+'>'+esc(tr(FX_LABELS[x.id]||x.id))+'</option>').join('')+'</select><button type="button" data-fx-remove="'+index+'" aria-label="'+esc(tr('fx_remove'))+'">✕</button></header><div class="ss-fx-params">'+rows+'</div></section>';
            }).join('');
            pane.innerHTML='<h3>'+esc(tr('track'))+'</h3><input data-track-name type="text" inputmode="text" enterkeyhint="done" maxlength="80" value="'+esc(t.name)+'" aria-label="'+esc(tr('name'))+'"><label>'+esc(tr('instrument'))+'<select data-instrument>'+items.map(i=>'<option value="'+esc(i.id)+'"'+(i.id===t.instrument?' selected':'')+'>'+esc(i.name)+'</option>').join('')+'</select></label>'+
                '<h4>'+esc(tr('your_sound'))+'</h4><div class="ss-param-grid">'+sound.map(p=>slider(...p)).join('')+'</div>'+
                '<h4>'+esc(tr('fx_send'))+'</h4><div class="ss-param-grid">'+sends.map(p=>slider(...p)).join('')+'</div>'+
                '<h4>'+esc(tr('fx_chain'))+'</h4><div class="ss-fx-rack">'+(rack||'<p class="ss-muted">'+esc(tr('fx_empty'))+'</p>')+'</div>'+
                (inserts.length<M.MAX_INSERTS?'<button type="button" data-fx-add class="ss-fx-add">＋ '+esc(tr('fx_add'))+'</button>':'')+
                '<div class="ss-mini-keys">'+[60,62,64,65,67,69,71,72].map((p,i)=>'<button data-audition="'+(drum?P.drums[i].drumNote:p)+'" aria-label="'+esc(tr('preview'))+'">'+(drum?'•':['C','D','E','F','G','A','B','C'][i])+'</button>').join('')+'</div><button data-action="deleteTrack" class="ss-delete-track">'+esc(tr('delete_track'))+'</button><p class="ss-help">'+esc(tr('edit_hint'))+'</p>';
            pane.querySelectorAll('input,select,[data-reset-param],[data-action="deleteTrack"],[data-fx-add],[data-fx-remove]').forEach(el=>el.disabled=blocked()&&!(stage?.active()&&['tone','reverb'].includes(el.dataset.param)));
        }
        // The master strip is a project wide control. It sits in the transport
        // area so the mix can be judged without opening the sound panel.
        const MASTER_GROUPS=[
            {label:'fx_output',fields:[['volume','volume',0,1.5,.01],['width','fx_width',0,2,.01]]},
            {label:'fx_compressor',cols:3,fields:[['threshold','fx_threshold',-60,0,.5],['ratio','fx_ratio',1,20,.1],['knee','fx_knee',0,40,.5],['attack','attack',0,1,.001],['release','release',0,1,.01]]},
            {label:'fx_eq3',fields:[['low','fx_low',-18,18,.5],['mid','fx_mid',-18,18,.5],['high','fx_high',-18,18,.5]]},
            {label:'fx_send',cols:3,fields:[['plateSize','fx_size',0,1,.01],['plateDecay','fx_decay',0,1,.01],['echoTime','fx_time',.01,2,.005],['echoFeedback','fx_feedback',0,.85,.01]],select:'echoSync'}
        ];
        const ECHO_DIVISIONS=['off','1/4','1/8','1/8T','1/16','1/16T'];
        function renderMaster(){
            const panel=find('[data-master-panel]');if(!panel)return;
            const defaults=M.MASTER_DEFAULTS||{},m={...defaults,...(project().master||{})};
            const field=([key,label,min,max,step])=>{const value=Number(m[key]??defaults[key]??0);return '<label class="ss-parameter"><span>'+esc(tr(label))+'</span><input data-master-param="'+key+'" type="range" min="'+min+'" max="'+max+'" step="'+step+'" value="'+value+'" aria-label="'+esc(tr(label))+'"><output>'+value.toFixed(2)+'</output></label>';};
            const sync=String(m.echoSync??'1/8');
            const options=ECHO_DIVISIONS.map(value=>'<option value="'+value+'"'+(value===sync?' selected':'')+'>'+esc(value==='off'?tr('fx_off'):value)+'</option>').join('');
            panel.innerHTML=MASTER_GROUPS.map(group=>{
                const select=group.select?('<label class="ss-parameter ss-master-select"><span>'+esc(tr('fx_echo_sync'))+'</span><select data-master-select="'+group.select+'" aria-label="'+esc(tr('fx_echo_sync'))+'">'+options+'</select></label>'):'';
                return '<div class="ss-master-group"><h4>'+esc(tr(group.label))+'</h4><div class="ss-master-fields" style="--ss-master-cols:'+(group.cols||2)+'">'+group.fields.map(field).join('')+select+'</div></div>';
            }).join('');
            panel.querySelectorAll('input,select').forEach(el=>el.disabled=blocked());
        }

        function controlTarget(el) {
            if(el.dataset.masterParam)return {master:true,key:el.dataset.masterParam,value:Number(el.value)};
            if(el.dataset.param)return {id:track()?.id,key:el.dataset.param,value:Number(el.value)};
            if(el.dataset.fxParam){const parts=el.dataset.fxParam.split(':');return {id:track()?.id,fxIndex:Number(parts[0]),fxKey:parts[1],value:Number(el.value)};}
            if(el.dataset.trackVolume)return {id:el.dataset.trackVolume,key:'volume',value:Number(el.value)};
            return null;
        }
        function setParameter(p, c) {if(c.master){p.master={...(p.master||{}),[c.key]:c.value};return;}const t=p.tracks.find(t=>t.id===c.id);if(!t)return;
            if(c.fxIndex!==undefined){const insert=t.fx?.inserts?.[c.fxIndex];if(insert)insert[c.fxKey]=c.value;return;}
            if(['pan','volume'].includes(c.key))t[c.key]=c.value;else t.params[c.key]=c.value;}
        function previewParameter(el) {
            const c=controlTarget(el);if(!c||(!c.master&&!c.id))return;
            if(!c.master&&stage?.preview(c)){const out=el.parentElement.querySelector('output');if(out)out.textContent=c.value.toFixed(2);return;}
            if(blocked())return;
            const p={...project(),master:{...(project().master||{})},tracks:project().tracks.map(t=>({...t,params:{...t.params},fx:t.fx?{...t.fx,inserts:t.fx.inserts.map(insert=>({...insert}))}:undefined}))};setParameter(p,c);audio.updateTracks(p);
            const out=el.parentElement.querySelector('output');if(out)out.textContent=Number(c.value).toFixed(2);
        }
        function finishParameter(cancel=false) {
            const g=parameterGesture;if(!g)return;parameterGesture=null;
            if(g.staged){stage.end(cancel);refresh();return;}
            if(cancel||blocked()){audio.updateTracks(project());refresh();return;}
            const c=controlTarget(g.element);if(c)mutate(p=>setParameter(p,{...c,id:g.id}),'mix');
        }
        root.addEventListener('pointerdown',e=>{const c=controlTarget(e.target);if(c&&(c.master||c.id)&&(!blocked()||(!c.master&&stage?.active()&&['tone','reverb'].includes(c.key))))parameterGesture={element:e.target,id:c.id,staged:c.master?null:stage?.begin({...c,external:true})};},{signal});
        root.addEventListener('keydown',e=>{if(e.key==='Escape'&&parameterGesture){e.preventDefault();finishParameter(true);return;}const c=controlTarget(e.target);if(c&&(c.master||c.id)&&(!blocked()||(!c.master&&stage?.active()&&['tone','reverb'].includes(c.key)))&&['ArrowLeft','ArrowRight','ArrowUp','ArrowDown','Home','End','PageUp','PageDown'].includes(e.key))parameterGesture||={element:e.target,id:c.id,staged:c.master?null:stage?.begin({...c,external:true})};},{signal});
        window.addEventListener('pointerup',()=>finishParameter(),{signal});
        window.addEventListener('keyup',e=>{if(parameterGesture&&e.target===parameterGesture.element)finishParameter();},{signal});
        window.addEventListener('pointercancel',()=>finishParameter(true),{signal});
        function renderMIDI(){
            if(disposed)return;const input=find('[data-midi-input]'),value=input.value;
            input.innerHTML='<option value="">'+esc(tr('midi_device'))+'</option>'+midi.inputs().map(x=>'<option value="'+esc(x.id)+'">'+esc(x.name)+'</option>').join('');if([...input.options].some(o=>o.value===value))input.value=value;
            find('[data-midi-state]').textContent=tr(['connected','disconnected','unsupported','denied'].includes(midiState.status)?midiState.status:'disconnected');
        }
        function position(tick,meta){if(meta)stage?.onTick(tick,meta);if(recording?.phase==='record'&&tick>=recording.start+recording.maxLength){const limited=recording.timelineLimited;stop();if(limited)fail({code:'E_PROJECT_LIMIT'});return;}cursor=Math.max(0,tick);editor.setPlayhead(cursor);find('[data-position]').textContent=String(Math.floor(cursor/BAR)+1).padStart(2,'0')+' : '+String(Math.floor(cursor%BAR/PPQ)+1).padStart(2,'0');}
        async function play(at=cursor,override,onStarted){
            if(disposed||busy)return;composer?.stop();const epoch=++generation;try{await audio.unlock();if(disposed||epoch!==generation)return;playing=true;status();const running=audio.play({...(override||stage.runtime(project())),metronome},at,position,()=>{if(epoch===generation){if(recording)finishRecording();stage?.stop();playing=false;refresh();}});await Promise.resolve();if(epoch===generation&&audio.running())onStarted?.();await running;}catch(error){stage?.stop();playing=false;fail(error);refresh();}
        }
        function panic(){for(const [key,n] of [...live.entries()])noteOff(Number(key.split(':')[1]),performance.now(),n.channel);audio.panic?.();}
        function stop(reset=false,keepPreview=false){generation++;if(!keepPreview)composer?.stop();stage?.stop();const tick=audio.stop();notice('');if(recording)finishRecording();panic();playing=false;position(reset?0:tick||cursor);refresh();}
        function seek(tick){const resume=playing;stop();position(tick);if(resume)play(tick);}
        async function audition(pitch=60,instrument){
            if(stage?.active())return;const epoch=generation;try{await audio.unlock();if(disposed||epoch!==generation)return;const targetInst=instrument||selectedInstrument,presetParams=M.defaultParamsFor?.(targetInst)||{tone:.5,resonance:0,drive:0,attack:.01,release:.3,reverb:.1,delay:0},presetInserts=M.defaultInsertsFor?.(targetInst)||[];const t=track()||{id:'audition',instrument:targetInst,volume:.65,pan:0,params:presetParams,fx:{inserts:presetInserts}};const release=audio.noteOn(instrument?{...t,id:'audition-'+instrument,instrument,params:presetParams,fx:{inserts:presetInserts}}:t,pitch,.7);later(()=>release?.(),300);}catch(error){fail(error);}
        }
        function recordTick(time){const r=recording;if(!r||r.phase!=='record')return 0;return Math.min(r.start+r.maxLength,Math.max(r.start,audio.currentTick()+(time-performance.now()-latency)/1000*project().tempo/60*PPQ));}
        function stopForRecordingLimit(){if(!recording)return;stop();fail({code:'E_PROJECT_LIMIT'});}
        // Recorded event fields and IDs are ASCII, so JSON string length is its exact UTF-8 byte count.
        function eventBytes(value){return JSON.stringify(value).length;}
        async function noteOn(pitch,velocity=.8,time=performance.now(),channel=16){
            if(stage?.active())return;const t=recording?project().tracks.find(t=>t.id===recording.trackId):track();if(!t||disposed)return;const epoch=generation,key=channel+':'+pitch;noteOff(pitch,time,channel);
            if(project().harmony.snap&&!M.isDrumInstrument?.(t.instrument)&&!t.instrument.includes('drum'))pitch=window.SynthStudioMusic.fit(pitch,project().harmony);
            if(composer?.enabled()){composer.hold(key,pitch,velocity);return;}
            const r=recording&&recording.phase==='record'?recording:null,start=r?recordTick(time):null;
            if(r){
                if(start>=r.start+r.maxLength||r.used>=r.capacity){stopForRecordingLimit();return;}
                const q=r.quantizeStep,relativeStart=Math.min(r.maxLength-q,Math.max(0,Math.round((start-r.start)/q)*q)),noteID=M.newID('note'),separatorBytes=r.noteCount?1:0;
                const reservedBytes=eventBytes({id:noteID,pitch,start:relativeStart,duration:r.maxLength-relativeStart,velocity})+separatorBytes;
                if(r.bytesUsed+reservedBytes>r.byteCapacity){stopForRecordingLimit();return;}
                r.used++;r.noteCount++;r.bytesUsed+=reservedBytes;
                live.set(key,{id:noteID,pitch,channel,release:null,start,velocity,recording:r,reservedBytes,separatorBytes});
            }else{live.set(key,{pitch,channel,release:null,start:null,velocity});}
            const note=live.get(key);
            try{await audio.unlock();if(disposed||epoch!==generation||live.get(key)!==note)return;note.release=audio.noteOn(t,pitch,velocity);find('[data-midi-led]').classList.add('is-on');}catch(error){if(live.get(key)===note){live.delete(key);if(note.recording===recording){recording.used--;recording.noteCount--;recording.bytesUsed-=note.reservedBytes;}}fail(error);}
        }
        function noteOff(pitch,time=performance.now(),channel=16){
            const key=channel+':'+pitch,n=live.get(key);composer?.release(key);if(!n)return;n.release?.();live.delete(key);if(!live.size)find('[data-midi-led]').classList.remove('is-on');
            if(recording?.phase==='record'&&n.recording===recording&&n.start!==null){
                const r=recording,end=Math.min(recordTick(time),r.start+r.maxLength),q=r.quantizeStep,maxEnd=r.start+r.maxLength;
                const absoluteStart=Math.min(maxEnd-q,Math.max(r.start,Math.round(n.start/q)*q)),start=absoluteStart-r.start;
                const duration=Math.min(Math.max(q,Math.round((end-n.start)/q)*q),maxEnd-absoluteStart);
                const note={id:n.id,pitch:n.pitch,start,duration,velocity:n.velocity};
                r.bytesUsed+=eventBytes(note)+n.separatorBytes-n.reservedBytes;
                r.notes.push(note);
            }
        }
        function captureArp(track,pitch,velocity,at,duration){
            const r=recording;if(!r||r.phase!=='record'||r.trackId!==track.id)return;
            const seconds=60/project().tempo/PPQ,q=r.quantizeStep;
            const raw=audio.currentTick()+(at-audio.clock())/seconds-r.start;
            if(raw<0)return;const start=Math.max(0,Math.round(raw/q)*q);
            if(start>=r.maxLength){stopForRecordingLimit();return;}
            const note={id:M.newID('note'),pitch,start,duration:Math.min(r.maxLength-start,Math.max(q,Math.round(duration/seconds/q)*q)),velocity};
            const bytes=eventBytes(note)+(r.noteCount?1:0);
            if(r.used>=r.capacity||r.bytesUsed+bytes>r.byteCapacity){stopForRecordingLimit();return;}
            r.notes.push(note);r.used++;r.noteCount++;r.bytesUsed+=bytes;
        }
        function controller(e){const t=recording?project().tracks.find(t=>t.id===recording.trackId):track();if(!t)return;const r=recording?.phase==='record'?recording:null;if(r){const tick=recordTick(e.time)-r.start;if(tick>=r.maxLength||r.used>=r.capacity){stopForRecordingLimit();return;}const event={tick:Math.min(r.maxLength-1,Math.max(0,Math.round(tick))),type:e.type,value:e.value},bytes=eventBytes(event)+(r.controllerCount?1:0);if(r.bytesUsed+bytes>r.byteCapacity){stopForRecordingLimit();return;}r.used++;r.controllerCount++;r.bytesUsed+=bytes;r.controllers.push(event);}audio.addController?.(t,e.type,e.value);}
        async function startRecording(){
            if(!ready||readonly()||busy||storageState.loading)return;if(recording){stop();return;}
            const existingEvents=eventCount(project());if(existingEvents>=MAX_EVENTS){if(playing)stop();fail({code:'E_PROJECT_LIMIT'});return;}
            const start=Math.floor(cursor/BAR)*BAR,availableLength=Math.floor((MAX_TICK-start)/BAR)*BAR,maxLength=Math.min(RECORDING_TICKS,availableLength);
            if(!Number.isFinite(start)||start<0||maxLength<BAR){if(playing)stop();fail({code:'E_PROJECT_LIMIT'});return;}
            if(!track()){
                const isDrum=selectedInstrument.startsWith('drum-')||selectedInstrument==='drums-kit'||selectedInstrument==='gm-drums';
                const drumTrack=isDrum?project().tracks.find(t=>t.instrument==='drums-kit'||t.instrument==='gm-drums'||t.instrument.startsWith('drum-')):null;
                const insertEvents=selectedInstrument.startsWith('drum-')?1:0;
                if(existingEvents+insertEvents>=MAX_EVENTS||(drumTrack&&drumTrack.clips.length>=MAX_CLIPS-1)){if(playing)stop();fail({code:'E_PROJECT_LIMIT'});return;}
                insert({type:'instrument',id:selectedInstrument},'',cursor);
            }
            const selected=track();if(!selected)return;
            if(selected.clips.length>=MAX_CLIPS){if(playing)stop();fail({code:'E_PROJECT_LIMIT'});return;}
            const capacity=MAX_EVENTS-eventCount(project());if(capacity<=0){if(playing)stop();fail({code:'E_PROJECT_LIMIT'});return;}
            const clip=M.createClip(selected.instrument,start);clip.name=tr('recording');clip.length=maxLength;
            const serializedProject=JSON.stringify(project()),byteCapacity=MAX_PROJECT_BYTES-utf8ByteLength(serializedProject)-utf8ByteLength(JSON.stringify(clip))-(selected.clips.length?1:0);
            if(byteCapacity<eventBytes({tick:0,type:'bend',value:0})){if(playing)stop();fail({code:'E_PROJECT_LIMIT'});return;}
            stop();const pending=++generation,quantized=find('[data-record-quantize]').checked;
            const session={trackId:selected.id,start,maxLength,phase:'starting',notes:[],controllers:[],quantize:quantized,quantizeStep:quantized?quantize:1,capacity,used:0,noteCount:0,controllerCount:0,bytesUsed:0,byteCapacity,clip,timelineLimited:availableLength<RECORDING_TICKS};
            recording=session;refresh();
            try{await audio.unlock();}catch(error){if(recording===session){recording=null;refresh();}fail(error);return;}
            if(disposed||pending!==generation||readonly()){if(recording===session){recording=null;refresh();}return;}
            recording.phase=countIn?'countin':'record';
            const epoch=++generation;
            const begin=()=>{
                if(disposed||epoch!==generation||!recording)return;
                recording.phase='record';
                const p=M.clone(project());p.loop.enabled=false;p.metronome=metronome;
                const t=p.tracks.find(t=>t.id===selected.id),silent=M.createClip(t.instrument,start);silent.notes=[];silent.length=maxLength;t.clips.push(silent);
                playing=true;audio.play(p,start,position,()=>{if(recording){const limited=recording.timelineLimited;stop();if(limited)fail({code:'E_PROJECT_LIMIT'});}}).catch(error=>{if(recording)stop();fail(error);});refresh();
            };
            playing=true;refresh();
            if(countIn){const p=M.create('Count in'),t=M.addTrack(p,'drums-kit'),c=M.createClip('drums-kit',0);c.notes=[];c.length=BAR;t.clips.push(c);p.tempo=project().tempo;p.metronome=true;try{await audio.play(p,0,tick=>{find('[data-position]').textContent=tr('count_in')+' '+(Math.floor(tick/PPQ)+1);},begin);}catch(error){if(recording===session)stop();fail(error);}}else begin();
        }
        function finishRecording(){
            if(!recording)return;const r=recording;for(const n of [...live.values()])noteOff(n.pitch,performance.now(),n.channel);recording=null;
            if(r.phase==='record'&&(r.notes.length||r.controllers.length)){
                try{const p=M.clone(project()),t=p.tracks.find(t=>t.id===r.trackId),c=r.clip;c.notes=r.notes;c.controllers=r.controllers;c.length=Math.max(BAR,Math.ceil(Math.max(...r.notes.map(n=>n.start+n.duration),...r.controllers.map(c=>c.tick+1),1)/BAR)*BAR);t.clips.push(c);history.commit(M.validate(p));currentProject=history.current();storage.changed();editor.select(t.id,c.id);}catch(error){fail(error);}
            }refresh();
        }
        async function replaceProject(p){stop();await storage.newDocument(p,{dirty:true});}
        async function exportFile(type,asFile=false){
            if(busy||recording||stage.active())return;busy=true;stop();status();notice(tr('export')+' '+type.toUpperCase()+'…');const snapshot=M.clone(project()),epoch=++generation;snapshot.loop.enabled=false;
            try {
                const blob=type==='wav'?await window.SynthStudioAudio.renderWav(snapshot):new Blob([window.SynthStudioMIDI.exportFile(snapshot)],{type:'audio/midi'});
                if(disposed||epoch!==generation)return [];
                const name=(snapshot.name.replace(/[<>:"/\\|?*]/g,'_')||'Synth Studio')+(type==='wav'?'.wav':'.mid');
                if(asFile)return [new File([blob],name,{type:blob.type})];
                const url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),30000);
                notice(type==='midi'?tr('midi_export_hint')+' '+tr('midi_automation_hint'):'');
            }catch(error){if(asFile)throw error;fail(error);}finally{busy=false;status();}
        }
        const actions={play:()=>playing?stop():play(),pause:()=>stop(),stop:()=>stop(true),record:startRecording,new:()=>replaceProject(M.create(tr('untitled'))),demo:()=>{const p=M.demo();p.tracks.forEach(t=>{const pat=M.patterns.find(x=>x.instrument===t.instrument);if(pat){t.name=patternName(pat);t.clips.forEach(c=>c.name=patternName(pat));}});return replaceProject(p);},open:()=>{stop();return storage.open();},save:()=>storage.save(false),saveAs:()=>storage.save(true),undo:()=>updatedHistory('undo'),redo:()=>updatedHistory('redo'),library:()=>{root.classList.remove('ss-inspector-open');root.classList.toggle('ss-library-open');},inspector:()=>{root.classList.remove('ss-library-open');root.classList.toggle('ss-inspector-open');},midi:()=>{find('[data-midi-panel]').hidden=!find('[data-midi-panel]').hidden;},connect:async()=>{await midi.connect();renderMIDI();},importMidi:()=>find('[data-midi-file]').click(),master:()=>{const panel=find('[data-master-panel]');if(!panel)return;panel.hidden=!panel.hidden;const toggle=find('[data-action="master"]');if(toggle)toggle.setAttribute('aria-expanded',String(!panel.hidden));},tour:()=>{tourIndex=0;renderTour();},tourNext:()=>{tourIndex=Math.min(TOUR_STEPS.length-1,tourIndex+1);renderTour();},tourBack:()=>{tourIndex=Math.max(0,tourIndex-1);renderTour();},tourSkip:()=>{tourIndex=-1;renderTour();},rename:async()=>{if(blocked())return;const value=await ctx.promptDialog?.(tr('name'),project().name);if(value?.trim())mutate(p=>{p.name=value.trim().slice(0,80);});},deleteTrack:async()=>{const t=track();if(!t||blocked())return;if(await ctx.confirmDialog?.(tr('delete_track'),tr('delete_track_confirm')))mutate(p=>{p.tracks=p.tracks.filter(x=>x.id!==t.id);});}};

        // A five step tour that walks through the workbench panels so the new
        // features are discoverable without a manual.
        function renderTour(){
            const overlay=find('[data-tour]');
            if(tourIndex<0){overlay.hidden=true;overlay.innerHTML='';return;}
            const step=TOUR_STEPS[tourIndex];
            if(step.panel)workspace.panel(step.panel);
            overlay.hidden=false;
            overlay.innerHTML='<div class="ss-tour-card"><span class="ss-tour-step">'+(tourIndex+1)+' / '+TOUR_STEPS.length+'</span><p>'+esc(tr(step.key))+'</p><div class="ss-tour-actions">'+button('tourBack','tour_back','←')+button('tourNext','tour_next','→')+'<button type="button" data-action="tourSkip">'+esc(tr('tour_skip'))+'</button></div></div>';
        }

        // The master analyser is feedback only: it never influences the audio.
        function drawAnalyser(){
            if(disposed)return;
            const canvas=find('[data-analyser]');
            if(canvas&&canvas.isConnected){
                const ctx2d=canvas.getContext?.('2d');
                if(ctx2d){
                    const bins=audio.spectrum(48);
                    const width=canvas.width,height=canvas.height;
                    ctx2d.clearRect(0,0,width,height);
                    if(bins){
                        const step=width/bins;
                        for(let i=0;i<bins;i++){
                            const value=bins[i]/255;
                            const bar=Math.max(1,value*height);
                            ctx2d.fillStyle='rgba(23,104,81,'+(0.25+value*0.75).toFixed(3)+')';
                            ctx2d.fillRect(i*step,height-bar,Math.max(1,step-1),bar);
                        }
                    }else{
                        ctx2d.fillStyle='rgba(120,132,146,.35)';
                        ctx2d.fillRect(0,height-1,width,1);
                    }
                }
            }
            analyserFrame=requestAnimationFrame(drawAnalyser);
        }
        let analyserFrame=0;

        actions.library=()=>workspace.toggleLibrary();
        actions.inspector=()=>workspace.panel('sound');
        root.addEventListener('click',e=>{
            const target=e.target.closest('button');if(!target)return;
            if(target.dataset.template){replaceProject(M.template(target.dataset.template,tr('template_'+target.dataset.template))).catch(fail);return;}
            if(target.dataset.resetParam){const key=target.dataset.resetParam,t=track();if(t)mutate(p=>setParameter(p,{id:t.id,key,value:key==='pan'?0:M.defaultParamsFor(t.instrument)[key]}),'mix');return;}
            if(target.dataset.favourite){const id=target.dataset.favourite,list=libraryPrefs.favourites,at=list.indexOf(id);if(at>=0)list.splice(at,1);else list.unshift(id);saveLibraryPrefs();renderLibrary();return;}
            if(target.dataset.tag){activeTag=activeTag===target.dataset.tag?'':target.dataset.tag;renderLibrary();return;}
            if(target.dataset.fxAdd){mutate(p=>{const t=p.tracks.find(t=>t.id===track()?.id);if(!t)return;if(!t.fx)t.fx={inserts:[]};if(t.fx.inserts.length>=M.MAX_INSERTS)return;t.fx.inserts.push(M.insertSpec('chorus',{mix:.3}));},'mix');return;}
            if(target.dataset.fxRemove){const index=Number(target.dataset.fxRemove);mutate(p=>{const t=p.tracks.find(t=>t.id===track()?.id);if(t?.fx?.inserts)t.fx.inserts.splice(index,1);},'mix');return;}
            if(target.dataset.choose){selectedInstrument=target.dataset.choose;markRecent(target.dataset.choose);renderLibrary();return;}
            if(target.dataset.action){Promise.resolve().then(()=>actions[target.dataset.action]?.()).catch(fail);return;}
            if(target.dataset.category){category=target.dataset.category;activeTag='';renderLibrary();return;}
            if(target.dataset.libraryMode){libraryMode=target.dataset.libraryMode;activeTag='';renderLibrary();return;}
            if(target.dataset.insert){selectedInstrument=target.dataset.insert;markRecent(target.dataset.insert);insert({type:libraryMode==='patterns'?'pattern':'instrument',id:target.dataset.insert},'',cursor);renderLibrary();return;}
            if(target.dataset.preview){const pattern=M.patterns.find(p=>p.id===target.dataset.preview);if(pattern&&libraryMode==='patterns'){const p=M.create('Preview'),t=M.addTrack(p,pattern.instrument.startsWith('drum-')?'drums-kit':pattern.instrument);t.clips.push(M.cloneClip(pattern.clip,0));stop();audio.play(p,0,()=>{},()=>{playing=false;status();}).catch(fail);}else{const preset=P.get(target.dataset.preview);markRecent(target.dataset.preview);audition(preset?.drumNote||60,target.dataset.preview);}return;}
            if(target.dataset.audition)audition(Number(target.dataset.audition));
        },{signal});
        root.addEventListener('dragstart',e=>{const item=e.target.closest('[data-library-id]');if(!item||blocked()){e.preventDefault();return;}e.dataTransfer.setData('application/x-aurago-synth',JSON.stringify({type:item.dataset.libraryType,id:item.dataset.libraryId}));e.dataTransfer.effectAllowed='copy';},{signal});
        root.addEventListener('input',e=>{previewParameter(e.target);if(e.target.hasAttribute('data-search'))renderLibrary();if(e.target.hasAttribute('data-zoom'))editor.setZoom(Number(e.target.value));},{signal});
        root.addEventListener('change',e=>{
            const el=e.target;
            if(el.hasAttribute('data-export')){const value=el.value;el.value='';if(value)exportFile(value);}
            if(el.hasAttribute('data-tempo'))mutate(p=>{p.tempo=Number(el.value);});
            if(el.hasAttribute('data-loop'))mutate(p=>{p.loop.enabled=el.checked;});
            if(el.hasAttribute('data-loop-start')||el.hasAttribute('data-loop-end'))mutate(p=>{p.loop.start=(Number(find('[data-loop-start]').value)-1)*BAR;p.loop.end=(Number(find('[data-loop-end]').value)-1)*BAR;});
            if(el.hasAttribute('data-metronome'))metronome=el.checked;
            if(el.hasAttribute('data-snap')){quantize=Number(el.value);editor.setSnap(quantize);}
            if(el.hasAttribute('data-track-name'))mutate(p=>{p.tracks.find(t=>t.id===track().id).name=el.value;});
            if(el.hasAttribute('data-instrument'))mutate(p=>{const t=p.tracks.find(t=>t.id===track().id);t.instrument=el.value;if(M.defaultParamsFor)t.params=M.defaultParamsFor(el.value);});
            if(el.hasAttribute('data-fx-type')){const index=Number(el.dataset.fxType),type=el.value;mutate(p=>{const t=p.tracks.find(t=>t.id===track()?.id);if(t?.fx?.inserts?.[index])t.fx.inserts[index]=M.insertSpec(type,{});},'mix');}
            if(el.hasAttribute('data-master-select')){const key=el.dataset.masterSelect;mutate(p=>{p.master={...(p.master||{}),[key]:el.value};},'mix');}
            const parameter=controlTarget(el);if(parameter&&(parameter.master||parameter.id)&&!parameterGesture)mutate(p=>setParameter(p,parameter),'mix');
            if(el.hasAttribute('data-midi-input')){panic();midi.select(el.value).catch(fail);}
            if(el.hasAttribute('data-count-in'))countIn=el.checked;
            if(el.hasAttribute('data-latency'))latency=Math.max(-200,Math.min(200,Number(el.value)||0));
            if(el.hasAttribute('data-midi-file')){const file=el.files[0];el.value='';if(file){if(file.size>10*1024*1024){fail('invalid_project');return;}file.arrayBuffer().then(bytes=>window.SynthStudioMIDI.importFile(bytes)).then(replaceProject).catch(fail);}}
        },{signal});
        const keyboard={a:60,w:61,s:62,e:63,d:64,f:65,t:66,g:67,y:68,h:69,u:70,j:71,k:72};
        root.addEventListener('keydown',e=>{
            if(e.defaultPrevented||e.target.matches('input,textarea,select')||e.repeat)return;
            if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='s'){e.preventDefault();storage.save(e.shiftKey).catch(fail);return;}
            if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='z'){e.preventDefault();updatedHistory(e.shiftKey?'redo':'undo');return;}
            if(e.ctrlKey||e.metaKey||e.altKey)return;
            if(e.code==='Space'&&!e.target.closest('button')){e.preventDefault();actions.play();return;}
            if(keyboard[e.key.toLowerCase()]!=null){e.preventDefault();noteOn(keyboard[e.key.toLowerCase()],.8);}
        },{signal});
        window.addEventListener('keyup',e=>{if(keyboard[e.key.toLowerCase()]!=null)noteOff(keyboard[e.key.toLowerCase()]);},{signal});
        window.addEventListener('blur',()=>{if(recording)stop();else {composer?.stop();panic();}},{signal});
        document.addEventListener('visibilitychange',()=>{if(document.hidden)stop();},{signal});
        document.addEventListener('aurago:desktop-policy',e=>{if(typeof e.detail?.readonly==='boolean'){revoked=e.detail.readonly;ctx.readonly=revoked;if(revoked)stop();refresh();}},{signal});
        document.addEventListener('aurago:auth-ended',()=>dispose(id),{signal});
        window.addEventListener('beforeunload',e=>{if(storage.state.dirty){e.preventDefault();e.returnValue='';}},{signal});
        window.addEventListener('pagehide',()=>dispose(id),{signal});
        const guard=async()=>{stop();await stage?.guard();return storage.guard();};
        ctx.setWindowBeforeClose?.(id,guard);
        ctx.registerWindowCleanup?.(id,()=>dispose(id));
        const instance={guard,dispose(){if(disposed)return;stage?.stop();disposed=true;generation++;cancelAnimationFrame(analyserFrame);audio.stop();composer?.stop();panic();timers.forEach(clearTimeout);timers.clear();life.abort();midi.dispose();audio.dispose();storage.dispose();editor.dispose();},project:()=>M.clone(project()),editor,storage,stage,audio};
        instances.set(id,instance);renderLibrary();refresh();analyserFrame=requestAnimationFrame(drawAnalyser);
        Promise.resolve(storage.ready).catch(fail).finally(()=>{if(!disposed){ready=true;refresh();}});
        return instance;
    }
    function dispose(id){const instance=instances.get(id);if(instance){instances.delete(id);instance.dispose();}}
    window.SynthStudioApp={render,dispose,instances};
})();
