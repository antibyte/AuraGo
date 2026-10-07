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
        let category='bass', libraryMode='instruments', revoked=false, selectedInstrument='bass-sub', countIn=true, quantize=120, latency=0, metronome=false;
        let live=new Map(), storageState={}, midiState={}, generation=0, ready=false;
        const timers=new Set(), later=(fn,delay)=>{const timer=setTimeout(()=>{timers.delete(timer);fn();},delay);timers.add(timer);};
        const eventCount=p=>p.tracks.reduce((total,t)=>total+t.clips.reduce((clipTotal,c)=>clipTotal+c.notes.length+c.controllers.length,0),0);
        const readonly=()=>!!ctx.readonly||revoked, blocked=()=>!ready||readonly()||busy||!!storageState.loading||!!recording;
        // Keep one validated view snapshot; MIDI packets must not clone the whole song.
        const project=()=>currentProject, track=()=>project().tracks.find(t=>t.id===editor?.selection().trackId);
        const fail=error=>{if(disposed)return;let code=typeof error==='string'?error:error?.code;if(code?.startsWith('E_PROJECT_'))code=code==='E_PROJECT_LIMIT'?'too_many_tracks':'invalid_project';else if(code==='E_MIDI_DENIED')code='denied';else if(code==='E_MIDI_UNSUPPORTED')code='unsupported';else if(code?.startsWith('E_MIDI_DEVICE'))code='no_device';else if(code?.startsWith('E_MIDI_'))code='unsupported_midi';else if(code==='SYNTH_STUDIO_RENDER_BUDGET')code='export_limit';notice(tr(['unsupported','denied','no_device','invalid_project','unsupported_midi','too_many_tracks','export_limit','readonly'].includes(code)?code:'error'));};
        function notice(text){const n=host.querySelector('[data-notice]');if(n){n.textContent=text||'';n.hidden=!text;}}
        host.innerHTML='<div class="ss-app" tabindex="0"><header class="ss-header"><div class="ss-brand"><span class="ss-brand-mark" aria-hidden="true">≋</span><div><strong>Synth Studio</strong><button class="ss-project-name" data-action="rename" data-project-name></button></div></div><nav class="ss-file-actions">'+button('new','new',icon('file-plus','＋'))+button('open','open',icon('folder-open','▱'))+button('save','save',icon('save','▣'))+button('saveAs','save_as',icon('copy','⧉'))+'<select data-export aria-label="'+esc(tr('export'))+'"><option value="">'+esc(tr('export'))+'</option><option value="wav">WAV</option><option value="midi">MIDI</option></select>'+button('demo','demo',tr('demo'))+'</nav></header>'+
            '<div class="ss-transport"><div class="ss-transport-buttons">'+button('play','play','▶','class="ss-play"')+button('pause','pause','Ⅱ')+button('stop','stop','■')+button('record','record','●','class="ss-record"')+'</div><output class="ss-position" data-position>01 : 01</output><label class="ss-tempo"><input data-tempo type="number" inputmode="numeric" enterkeyhint="done" min="40" max="240" value="120" aria-label="'+esc(tr('tempo'))+'"><span>BPM</span></label><span class="ss-time-signature">4/4</span><label><input data-metronome type="checkbox"> '+esc(tr('metronome'))+'</label><label><input data-loop type="checkbox"> '+esc(tr('loop'))+'</label><label class="ss-loop-range"><input data-loop-start type="number" inputmode="numeric" enterkeyhint="done" min="1" value="1" aria-label="'+esc(tr('loop_start'))+'"> — <input data-loop-end type="number" inputmode="numeric" enterkeyhint="done" min="2" value="5" aria-label="'+esc(tr('loop_end'))+'"></label><div class="ss-history">'+button('undo','undo','↶')+button('redo','redo','↷')+'</div></div>'+
            '<div class="ss-notice" data-notice role="status" hidden></div><div class="ss-workspace"><aside class="ss-library"><div class="ss-library-tabs"><button data-library-mode="instruments" aria-pressed="true">'+esc(tr('instruments'))+'</button><button data-library-mode="patterns" aria-pressed="false">'+esc(tr('patterns'))+'</button></div><input class="ss-search" data-search type="search" inputmode="search" enterkeyhint="search" placeholder="'+esc(tr('search'))+'" aria-label="'+esc(tr('search'))+'"><div class="ss-categories" data-categories></div><div class="ss-library-list" data-library-list></div><button class="ss-midi-toggle" data-action="midi">⌨ '+esc(tr('midi_connect'))+'</button></aside><main class="ss-main"><div class="ss-arrange-tools"><span>'+esc(tr('arrangement'))+'</span><label>'+esc(tr('snap'))+' <select data-snap><option value="480">1/4</option><option value="240">1/8</option><option value="120" selected>1/16</option><option value="60">1/32</option></select></label><label>'+esc(tr('zoom'))+' <input type="range" data-zoom min="20" max="100" value="40" aria-label="'+esc(tr('zoom'))+'"></label>'+button('inspector','your_sound','☷','class="ss-inspector-toggle"')+button('library','library','♫','class="ss-compact-only"')+'</div><div class="ss-editor-host" data-editor-host></div></main><aside class="ss-inspector" data-inspector></aside></div>'+
            '<section class="ss-midi-panel" data-midi-panel hidden><strong>'+esc(tr('midi_title'))+'</strong><button data-action="connect">'+esc(tr('midi_connect'))+'</button><select data-midi-input aria-label="'+esc(tr('midi_device'))+'"><option value="">'+esc(tr('midi_device'))+'</option></select><span data-midi-state role="status"></span><label><input data-count-in type="checkbox" checked> '+esc(tr('count_in'))+'</label><label>'+esc(tr('latency'))+' <input data-latency type="number" inputmode="numeric" enterkeyhint="done" min="-200" max="200" step="1" value="0"> ms</label><label><input data-record-quantize type="checkbox" checked> '+esc(tr('quantize'))+'</label>'+button('importMidi','import_midi',tr('import_midi'))+'</section>'+
            '<footer class="ss-footer"><span data-save-state role="status"></span><span class="ss-footer-hint">'+esc(tr('shortcuts'))+'</span><span class="ss-midi-led" data-midi-led aria-hidden="true"></span><span>50 SYNTHS · 128 GM</span></footer><input type="file" data-midi-file accept=".mid,.midi,audio/midi" hidden></div>';
        const root=host.firstElementChild, find=q=>root.querySelector(q);
        editor=window.SynthStudioEditor.create(find('[data-editor-host]'),{getProject:project,readonly:blocked,tr,mutate,onSelect:renderInspector,onAdd:()=>insert({type:'instrument',id:selectedInstrument},'',0),onDrop:insert,onNewClip:newClip,onSeek:seek,onAudition:p=>audition(p),onError:fail});
        const midi=window.SynthStudioMIDI.create({onNoteOn:e=>noteOn(e.pitch,e.velocity,e.time,e.channel),onNoteOff:e=>noteOff(e.pitch,e.time,e.channel),onController:e=>controller(e),onState:s=>{midiState=s;if(s.status!=='connected'){if(recording)stop();else panic();}renderMIDI();}});
        storage=window.SynthStudioStorage.create({...ctx,windowId:id},{serialize:project,validate:M.validate,beforeLoad:()=>stop(),onLoaded:p=>{if(p)loadProject(p);},onState:s=>{storageState=s;status();}});
        function status(){
            if(disposed)return;const p=project();find('[data-project-name]').textContent=p.name;find('[data-project-name]').disabled=blocked();
            find('[data-save-state]').textContent=readonly()?tr('readonly'):storageState.error?tr('save_error'):storageState.saving?tr('saving'):storageState.dirty?tr('unsaved'):storageState.path?tr('saved'):tr('local_draft');
            find('[data-action="record"]').classList.toggle('is-recording',!!recording);find('[data-action="record"]').setAttribute('aria-pressed',String(!!recording));
            find('[data-action="play"]').classList.toggle('is-active',playing);find('[data-tempo]').value=p.tempo;find('[data-loop]').checked=p.loop.enabled;
            find('[data-loop-start]').value=Math.floor(p.loop.start/BAR)+1;find('[data-loop-end]').value=Math.floor(p.loop.end/BAR)+1;
            for(const a of ['new','save','saveAs','demo','record','importMidi'])find('[data-action="'+a+'"]')?.toggleAttribute('disabled',!ready||readonly()||busy||!!storageState.loading);
            find('[data-action="open"]').disabled=!ready||busy||!!storageState.loading;find('[data-action="play"]').disabled=!ready||busy||!!storageState.loading;find('[data-tempo]').disabled=blocked();find('[data-loop]').disabled=blocked();find('[data-loop-start]').disabled=blocked();find('[data-loop-end]').disabled=blocked();
            find('[data-action="undo"]').disabled=blocked()||!history.canUndo();find('[data-action="redo"]').disabled=blocked()||!history.canRedo();find('[data-export]').disabled=busy||!!recording;
            const item=ctx.state?.windows?.get(id);if(item){item.context={...(item.context||{}),path:storageState.path||''};if(item.element)item.element.classList.toggle('ss-dirty',!!storageState.dirty);}
        }
        function refresh(){
            const active=document.activeElement,attrs=['data-param','data-track-name','data-instrument','data-track-volume','data-velocity'],attr=attrs.find(a=>active?.hasAttribute(a)),value=attr?active.getAttribute(attr):'';
            editor.render();renderInspector();status();
            if(attr)root.querySelectorAll('['+attr+']').forEach(el=>{if(el.getAttribute(attr)===value)el.focus({preventScroll:true});});
        }
        function loadProject(p){stop();history=M.history(M.validate(p));currentProject=history.current();cursor=0;editor.select('','');refresh();}
        function mutate(fn){
            if(blocked())return;const wasPlaying=playing,at=wasPlaying?audio.currentTick():cursor;
            try{const next=M.clone(project());fn(next);history.commit(M.validate(next));currentProject=history.current();if(wasPlaying){audio.stop();playing=false;play(at);}storage.changed();refresh();}catch(error){fail(error);}
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
        function renderLibrary(){
            find('[data-categories]').innerHTML=[...P.categories,{id:'gm',name:'General MIDI'}].map(c=>'<button data-category="'+esc(c.id)+'" aria-pressed="'+(c.id===category)+'">'+esc(c.id==='gm'?'General MIDI':tr(c.id))+'</button>').join('');
            const query=find('[data-search]').value.toLowerCase(),list=libraryMode==='patterns'?M.patterns:category==='gm'?[...P.gm,{id:'gm-drums',name:tr('gm_drums'),category:'gm'}]:P.presets;
            const items=list.map(x=>libraryMode==='patterns'?{...x,name:patternName(x)}:x).filter(x=>x.category===category&&x.name.toLowerCase().includes(query));
            find('[data-library-list]').innerHTML=items.map((x,i)=>'<div class="ss-library-item '+(x.id===selectedInstrument?'is-selected':'')+'" draggable="'+!readonly()+'" data-library-id="'+esc(x.id)+'" data-library-type="'+(libraryMode==='patterns'?'pattern':'instrument')+'"><button data-preview="'+esc(x.id)+'" aria-label="'+esc(tr('preview')+' '+x.name)+'">'+(libraryMode==='patterns'?'▥':'♪')+'</button><button class="ss-library-name" data-insert="'+esc(x.id)+'"><span>'+esc(x.name)+'</span><small>'+(libraryMode==='patterns'?esc(tr('pattern')):String(i+1).padStart(2,'0'))+'</small></button><button class="ss-insert" data-insert="'+esc(x.id)+'" aria-label="'+esc(tr('add'))+'">＋</button></div>').join('')||'<p class="ss-muted">'+esc(tr('no_results'))+'</p>';
            root.querySelectorAll('[data-library-mode]').forEach(el=>el.setAttribute('aria-pressed',String(el.dataset.libraryMode===libraryMode)));
        }
        function renderInspector(){
            const t=track(),pane=find('[data-inspector]');if(!t){pane.innerHTML='<div class="ss-inspector-empty"><span>♫</span><strong>'+esc(tr('your_sound'))+'</strong><p>'+esc(tr('inspector_hint'))+'</p></div>';return;}
            const drum=t.instrument==='drums-kit'||t.instrument==='gm-drums'||t.instrument.startsWith('drum-'),items=drum?[{id:'drums-kit',name:tr('drums')},{id:'gm-drums',name:tr('gm_drums')},...P.drums]:[...P.presets.filter(x=>x.category!=='drums'),...P.gm];
            pane.innerHTML='<h3>'+esc(tr('track'))+'</h3><input data-track-name type="text" inputmode="text" enterkeyhint="done" maxlength="80" value="'+esc(t.name)+'" aria-label="'+esc(tr('name'))+'"><label>'+esc(tr('instrument'))+'<select data-instrument>'+items.map(i=>'<option value="'+esc(i.id)+'"'+(i.id===t.instrument?' selected':'')+'>'+esc(i.name)+'</option>').join('')+'</select></label>'+[['pan',-1,1,.01,t.pan],['tone',0,1,.01,t.params.tone],['attack',0,2,.01,t.params.attack],['release',.02,3,.01,t.params.release],['reverb',0,1,.01,t.params.reverb],['delay',0,1,.01,t.params.delay]].map(([key,min,max,step,value])=>'<label class="ss-parameter"><span>'+esc(tr(key))+'</span><input data-param="'+key+'" type="range" min="'+min+'" max="'+max+'" step="'+step+'" value="'+value+'" aria-label="'+esc(tr(key))+'"></label>').join('')+'<div class="ss-mini-keys">'+[60,62,64,65,67,69,71,72].map((p,i)=>'<button data-audition="'+(drum?P.drums[i].drumNote:p)+'" aria-label="'+esc(tr('preview'))+'">'+(drum?'•':['C','D','E','F','G','A','B','C'][i])+'</button>').join('')+'</div><button data-action="deleteTrack" class="ss-delete-track">'+esc(tr('delete_track'))+'</button><p class="ss-help">'+esc(tr('edit_hint'))+'</p>';
            pane.querySelectorAll('input,select,[data-action="deleteTrack"]').forEach(el=>el.disabled=blocked());
        }
        function renderMIDI(){
            if(disposed)return;const input=find('[data-midi-input]'),value=input.value;
            input.innerHTML='<option value="">'+esc(tr('midi_device'))+'</option>'+midi.inputs().map(x=>'<option value="'+esc(x.id)+'">'+esc(x.name)+'</option>').join('');if([...input.options].some(o=>o.value===value))input.value=value;
            find('[data-midi-state]').textContent=tr(['connected','disconnected','unsupported','denied'].includes(midiState.status)?midiState.status:'disconnected');
        }
        function position(tick){if(recording?.phase==='record'&&tick>=recording.start+recording.maxLength){const limited=recording.timelineLimited;stop();if(limited)fail({code:'E_PROJECT_LIMIT'});return;}cursor=Math.max(0,tick);editor.setPlayhead(cursor);find('[data-position]').textContent=String(Math.floor(cursor/BAR)+1).padStart(2,'0')+' : '+String(Math.floor(cursor%BAR/PPQ)+1).padStart(2,'0');}
        async function play(at=cursor){
            if(disposed||busy)return;const epoch=++generation;try{await audio.unlock();if(disposed||epoch!==generation)return;playing=true;status();await audio.play({...project(),metronome},at,position,()=>{if(epoch===generation){if(recording)finishRecording();playing=false;status();}});}catch(error){playing=false;fail(error);status();}
        }
        function panic(){for(const n of [...live.values()])noteOff(n.pitch,performance.now(),n.channel);audio.panic?.();}
        function stop(reset=false){generation++;const tick=audio.stop();if(recording)finishRecording();panic();playing=false;position(reset?0:tick||cursor);status();}
        function seek(tick){const resume=playing;stop();position(tick);if(resume)play(tick);}
        async function audition(pitch=60,instrument){
            const epoch=generation;try{await audio.unlock();if(disposed||epoch!==generation)return;const t=track()||{instrument:instrument||selectedInstrument,volume:.65,pan:0,params:{tone:.5,attack:.01,release:.3,reverb:.1,delay:0}};const release=audio.noteOn(instrument?{...t,instrument}:t,pitch,.7);later(()=>release?.(),300);}catch(error){fail(error);}
        }
        function recordTick(time){const r=recording;if(!r||r.phase!=='record')return 0;return Math.min(r.start+r.maxLength,Math.max(r.start,audio.currentTick()+(time-performance.now()-latency)/1000*project().tempo/60*PPQ));}
        function stopForRecordingLimit(){if(!recording)return;stop();fail({code:'E_PROJECT_LIMIT'});}
        // Recorded event fields and IDs are ASCII, so JSON string length is its exact UTF-8 byte count.
        function eventBytes(value){return JSON.stringify(value).length;}
        async function noteOn(pitch,velocity=.8,time=performance.now(),channel=16){
            const t=recording?project().tracks.find(t=>t.id===recording.trackId):track();if(!t||disposed)return;const epoch=generation,key=channel+':'+pitch;noteOff(pitch,time,channel);
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
            const key=channel+':'+pitch,n=live.get(key);if(!n)return;n.release?.();live.delete(key);if(!live.size)find('[data-midi-led]').classList.remove('is-on');
            if(recording?.phase==='record'&&n.recording===recording&&n.start!==null){
                const r=recording,end=Math.min(recordTick(time),r.start+r.maxLength),q=r.quantizeStep,maxEnd=r.start+r.maxLength;
                const absoluteStart=Math.min(maxEnd-q,Math.max(r.start,Math.round(n.start/q)*q)),start=absoluteStart-r.start;
                const duration=Math.min(Math.max(q,Math.round((end-n.start)/q)*q),maxEnd-absoluteStart);
                const note={id:n.id,pitch:n.pitch,start,duration,velocity:n.velocity};
                r.bytesUsed+=eventBytes(note)+n.separatorBytes-n.reservedBytes;
                r.notes.push(note);
            }
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
        async function exportFile(type){
            if(busy||recording)return;busy=true;stop();status();notice(tr('export')+' '+type.toUpperCase()+'…');const snapshot=M.clone(project()),epoch=++generation;
            try{const blob=type==='wav'?await window.SynthStudioAudio.renderWav(snapshot):new Blob([window.SynthStudioMIDI.exportFile(snapshot)],{type:'audio/midi'});if(disposed||epoch!==generation)return;const url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=(snapshot.name.replace(/[<>:"/\\|?*]/g,'_')||'Synth Studio')+(type==='wav'?'.wav':'.mid');a.click();setTimeout(()=>URL.revokeObjectURL(url),30000);notice(type==='midi'?tr('midi_export_hint'):'');}catch(error){fail(error);}finally{busy=false;status();}
        }
        const actions={play:()=>playing?stop():play(),pause:()=>stop(),stop:()=>stop(true),record:startRecording,new:()=>replaceProject(M.create(tr('untitled'))),demo:()=>{const p=M.demo();p.tracks.forEach(t=>{const pat=M.patterns.find(x=>x.instrument===t.instrument);if(pat){t.name=patternName(pat);t.clips.forEach(c=>c.name=patternName(pat));}});return replaceProject(p);},open:()=>{stop();return storage.open();},save:()=>storage.save(false),saveAs:()=>storage.save(true),undo:()=>updatedHistory('undo'),redo:()=>updatedHistory('redo'),library:()=>{root.classList.remove('ss-inspector-open');root.classList.toggle('ss-library-open');},inspector:()=>{root.classList.remove('ss-library-open');root.classList.toggle('ss-inspector-open');},midi:()=>{find('[data-midi-panel]').hidden=!find('[data-midi-panel]').hidden;},connect:async()=>{await midi.connect();renderMIDI();},importMidi:()=>find('[data-midi-file]').click(),rename:async()=>{if(blocked())return;const value=await ctx.promptDialog?.(tr('name'),project().name);if(value?.trim())mutate(p=>{p.name=value.trim().slice(0,80);});},deleteTrack:async()=>{const t=track();if(!t||blocked())return;if(await ctx.confirmDialog?.(tr('delete_track'),tr('delete_track_confirm')))mutate(p=>{p.tracks=p.tracks.filter(x=>x.id!==t.id);});}};
        root.addEventListener('click',e=>{
            const target=e.target.closest('button');if(!target)return;
            if(target.dataset.action){Promise.resolve().then(()=>actions[target.dataset.action]?.()).catch(fail);return;}
            if(target.dataset.category){category=target.dataset.category;renderLibrary();return;}
            if(target.dataset.libraryMode){libraryMode=target.dataset.libraryMode;renderLibrary();return;}
            if(target.dataset.insert){selectedInstrument=target.dataset.insert;insert({type:libraryMode==='patterns'?'pattern':'instrument',id:target.dataset.insert},'',cursor);renderLibrary();return;}
            if(target.dataset.preview){const pattern=M.patterns.find(p=>p.id===target.dataset.preview);if(pattern&&libraryMode==='patterns'){const p=M.create('Preview'),t=M.addTrack(p,pattern.instrument.startsWith('drum-')?'drums-kit':pattern.instrument);t.clips.push(M.cloneClip(pattern.clip,0));stop();audio.play(p,0,()=>{},()=>{playing=false;status();}).catch(fail);}else{const preset=P.get(target.dataset.preview);audition(preset?.drumNote||60,target.dataset.preview);}return;}
            if(target.dataset.audition)audition(Number(target.dataset.audition));
        },{signal});
        root.addEventListener('dragstart',e=>{const item=e.target.closest('[data-library-id]');if(!item||blocked()){e.preventDefault();return;}e.dataTransfer.setData('application/x-aurago-synth',JSON.stringify({type:item.dataset.libraryType,id:item.dataset.libraryId}));e.dataTransfer.effectAllowed='copy';},{signal});
        root.addEventListener('input',e=>{if(e.target.hasAttribute('data-search'))renderLibrary();if(e.target.hasAttribute('data-zoom'))editor.setZoom(Number(e.target.value));},{signal});
        root.addEventListener('change',e=>{
            const el=e.target;
            if(el.hasAttribute('data-export')){const value=el.value;el.value='';if(value)exportFile(value);}
            if(el.hasAttribute('data-tempo'))mutate(p=>{p.tempo=Number(el.value);});
            if(el.hasAttribute('data-loop'))mutate(p=>{p.loop.enabled=el.checked;});
            if(el.hasAttribute('data-loop-start')||el.hasAttribute('data-loop-end'))mutate(p=>{p.loop.start=(Number(find('[data-loop-start]').value)-1)*BAR;p.loop.end=(Number(find('[data-loop-end]').value)-1)*BAR;});
            if(el.hasAttribute('data-metronome'))metronome=el.checked;
            if(el.hasAttribute('data-snap')){quantize=Number(el.value);editor.setSnap(quantize);}
            if(el.hasAttribute('data-track-name'))mutate(p=>{p.tracks.find(t=>t.id===track().id).name=el.value;});
            if(el.hasAttribute('data-instrument'))mutate(p=>{p.tracks.find(t=>t.id===track().id).instrument=el.value;});
            if(el.dataset.param)mutate(p=>{const t=p.tracks.find(t=>t.id===track().id);if(el.dataset.param==='pan')t.pan=Number(el.value);else t.params[el.dataset.param]=Number(el.value);});
            if(el.hasAttribute('data-midi-input')){panic();midi.select(el.value).catch(fail);}
            if(el.hasAttribute('data-count-in'))countIn=el.checked;
            if(el.hasAttribute('data-latency'))latency=Math.max(-200,Math.min(200,Number(el.value)||0));
            if(el.hasAttribute('data-midi-file')){const file=el.files[0];el.value='';if(file){if(file.size>10*1024*1024){fail('invalid_project');return;}file.arrayBuffer().then(bytes=>window.SynthStudioMIDI.importFile(bytes)).then(replaceProject).catch(fail);}}
        },{signal});
        const keyboard={a:60,w:61,s:62,e:63,d:64,f:65,t:66,g:67,y:68,h:69,u:70,j:71,k:72};
        root.addEventListener('keydown',e=>{
            if(e.target.matches('input,textarea,select')||e.repeat)return;
            if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='s'){e.preventDefault();storage.save(e.shiftKey).catch(fail);return;}
            if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='z'){e.preventDefault();updatedHistory(e.shiftKey?'redo':'undo');return;}
            if(e.ctrlKey||e.metaKey||e.altKey)return;
            if(e.code==='Space'){e.preventDefault();actions.play();return;}
            if(keyboard[e.key.toLowerCase()]!=null){e.preventDefault();noteOn(keyboard[e.key.toLowerCase()],.8);}
        },{signal});
        window.addEventListener('keyup',e=>{if(keyboard[e.key.toLowerCase()]!=null)noteOff(keyboard[e.key.toLowerCase()]);},{signal});
        window.addEventListener('blur',()=>{if(recording)stop();else panic();},{signal});
        document.addEventListener('visibilitychange',()=>{if(document.hidden)stop();},{signal});
        document.addEventListener('aurago:desktop-policy',e=>{if(typeof e.detail?.readonly==='boolean'){revoked=e.detail.readonly;ctx.readonly=revoked;if(revoked)stop();refresh();}},{signal});
        document.addEventListener('aurago:auth-ended',()=>dispose(id),{signal});
        window.addEventListener('beforeunload',e=>{if(storage.state.dirty){e.preventDefault();e.returnValue='';}},{signal});
        window.addEventListener('pagehide',()=>dispose(id),{signal});
        const guard=async()=>{stop();return storage.guard();};
        ctx.setWindowBeforeClose?.(id,guard);
        ctx.registerWindowCleanup?.(id,()=>dispose(id));
        const instance={guard,dispose(){if(disposed)return;disposed=true;generation++;audio.stop();panic();timers.forEach(clearTimeout);timers.clear();life.abort();midi.dispose();audio.dispose();storage.dispose();editor.dispose();},project:()=>M.clone(project()),editor,storage};
        instances.set(id,instance);renderLibrary();refresh();
        Promise.resolve(storage.ready).catch(fail).finally(()=>{if(!disposed){ready=true;refresh();}});
        return instance;
    }
    function dispose(id){const instance=instances.get(id);if(instance){instances.delete(id);instance.dispose();}}
    window.SynthStudioApp={render,dispose,instances};
})();
