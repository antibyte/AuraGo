(function(){
    'use strict';
    function create(host,options){
        const {tr,esc,signal}=options,M=window.SynthStudioModel,G=window.SynthStudioMusic;
        const previewAudio=window.SynthStudioAudio.create();
        let preview=null,degree=0,epoch=0,timer=null,nextTime=0,index=0,lastTrack='',lastHarmony='',held=new Map(),scheduled=[],padReleases=[];
        const label=(key,html)=>'<label>'+esc(tr(key))+html+'</label>';
        const select=(key,items)=>'<select data-music="'+key+'">'+items.map(([v,t])=>'<option value="'+v+'">'+esc(t)+'</option>').join('')+'</select>';
        const range=(key,min,max,step,value)=>'<input data-music="'+key+'" type="range" min="'+min+'" max="'+max+'" step="'+step+'" value="'+value+'"><output>'+value+'</output>';
        const check=key=>'<label><input data-music="'+key+'" type="checkbox"> '+esc(tr(key))+'</label>';
        const button=(key)=>'<button type="button" data-compose="'+key+'">'+esc(tr(key))+'</button>';
        host.innerHTML='<div class="ss-compose"><div class="ss-harmony">'+label('key',select('root',['C','C♯','D','D♯','E','F','F♯','G','G♯','A','A♯','B'].map((n,i)=>[i,n])))+label('scale',select('scale',[['major',tr('major')],['minor',tr('minor')]]))+check('scale_snap')+label('chord',select('seventh',[[0,tr('triad')],[1,tr('seventh')]]))+'</div><div class="ss-chord-pads"></div><details open><summary>'+esc(tr('arpeggiator'))+'</summary><div class="ss-generator-controls">'+check('arp_live')+label('order',select('order',['up','down','pendulum','random'].map(s=>[s,tr(s)])))+label('grid',select('division',[8,16,32].map(n=>[n,'1/'+n])))+label('octaves',select('octaves',[1,2,3].map(n=>[n,String(n)])))+label('gate',range('gate',.1,1,.05,.7))+label('seed','<input data-music="seed" type="text" maxlength="64" value="1">')+'</div></details><details><summary>'+esc(tr('variation_lab'))+'</summary><div class="ss-generator-controls">'+label('density',range('density',0,2,.1,1))+label('swing',range('swing',0,.6,.05,0))+label('humanize',range('humanize',0,1,.05,.2))+check('lock_pitch')+check('lock_rhythm')+'</div></details><details><summary>'+esc(tr('rhythm_generator'))+'</summary><div class="ss-generator-controls">'+label('steps',select('steps',[[16,'16'],[32,'32']]))+label('hits','<input data-music="hits" type="number" min="0" max="32" value="5">')+label('rotation','<input data-music="rotation" type="number" min="0" max="31" value="0">')+label('accent',select('accent',[1,2,3,4,8].map(n=>[n,String(n)])))+label('pitch',select('pitch',window.SynthStudioPresets.drums.map(d=>[d.drumNote,d.name])))+'</div></details><div class="ss-generator-actions">'+label('generator',select('generator',[['chord',tr('chord')],['arp',tr('arpeggiator')],['variation',tr('variation_lab')],['rhythm',tr('rhythm_generator')],['scale',tr('scale_apply')]]))+button('generate')+'<span data-compose-target></span></div><div class="ss-preview" hidden role="group" aria-label="'+esc(tr('preview'))+'">'+button('original')+button('variant')+button('apply')+button('cancel')+'<span data-preview-state role="status"></span></div></div>';
        const q=key=>host.querySelector('[data-music="'+key+'"]');
        const settings=()=>Object.fromEntries([...host.querySelectorAll('[data-music]')].map(e=>[e.dataset.music,e.type==='checkbox'?e.checked:e.type==='number'||e.type==='range'||['root','seventh','division','octaves','steps','accent','pitch'].includes(e.dataset.music)?Number(e.value):e.value]));
        function selection(){const p=options.project(),s=options.selection(),track=p.tracks.find(t=>t.id===s.trackId);return {p,track,clip:track?.clips.find(c=>c.id===s.clipId)};}
        function cancel(){epoch++;previewAudio.stop();preview=null;host.querySelector('.ss-preview').hidden=true;}
        function stopLive(){clearInterval(timer);timer=null;held.clear();for(const v of scheduled)v.cancel();scheduled=[];padReleases.forEach(fn=>fn());padReleases=[];}
        function stop(){cancel();stopLive();}
        function pump(){
            if(!held.size)return;
            const {p,track}=selection();if(!track){stopLive();return;}
            const now=options.audio.clock(),s=settings(),step=60/p.tempo*4/s.division;
            scheduled=scheduled.filter(v=>v.end>now);
            if(nextTime<now-.1){const missed=Math.ceil((now-nextTime)/step);index+=missed;nextTime+=missed*step;}
            try{while(nextTime<now+.075){
                const pitch=G.arpPitch([...held.values()].map(n=>n.pitch),s,index++);
                const velocity=[...held.values()][0].velocity,duration=step*s.gate;
                const cancel=options.audio.scheduleNote(track,pitch,velocity,nextTime,duration);
                scheduled.push({cancel,end:nextTime+duration+track.params.release});
                options.onArpNote?.(track,pitch,velocity,nextTime,duration);
                nextTime+=step;
            }}catch(error){stopLive();options.fail(error);}
        }
        async function hold(key,pitch,velocity=.8){
            if(!q('arp_live').checked)return false;
            const token=epoch;held.set(key,{pitch,velocity});
            try{await options.audio.unlock();if(token!==epoch||!held.has(key))return true;if(!timer){nextTime=options.audio.clock()+.015;index=0;pump();timer=setInterval(pump,25);}}catch(error){stopLive();options.fail(error);}
            return true;
        }
        function release(key){held.delete(key);if(!held.size)stopLive();}
        function generate(){
            if(options.blocked())return;
            const {p,track,clip}=selection();if(!clip)return;cancel();
            const s=settings();let notes;
            if(s.generator==='chord'){const start=Math.max(0,Math.min(clip.length-1,Math.round((options.cursor()-clip.start)/120)*120));notes=[...clip.notes,...G.chord(p.harmony,degree,!!s.seventh).map((pitch,i)=>({id:'chord-'+i,pitch,start,duration:Math.min(1920,clip.length-start),velocity:.75}))];}
            else if(s.generator==='arp'){const pitches=clip.notes.length?[...new Set(clip.notes.map(n=>n.pitch))]:G.chord(p.harmony,degree,!!s.seventh);notes=G.arpeggio(pitches,clip.length,s);}
            else if(s.generator==='variation')notes=G.variation(clip.notes,clip.length,p.harmony,{...s,lockPitch:s.lock_pitch,lockRhythm:s.lock_rhythm});
            else if(s.generator==='rhythm'){if(!isDrum(track))throw Error('invalid_project');notes=G.rhythm(clip.length,s);}
            else notes=clip.notes.map(n=>({...n,pitch:G.fit(n.pitch,p.harmony)}));
            const next=M.clone(p),target=next.tracks.find(t=>t.id===track.id).clips.find(c=>c.id===clip.id);
            target.notes=notes.map(n=>({...n,id:M.newID('note')}));
            M.validate(next);
            preview={base:JSON.stringify(p),p:next,track:track.id,clip:clip.id};
            host.querySelector('.ss-preview').hidden=false;
            host.querySelector('[data-preview-state]').textContent=tr('preview')+' · '+target.notes.length+' '+tr('notes');
        }
        function isDrum(t){return t&&(t.instrument==='gm-drums'||t.instrument==='drums-kit'||t.instrument.startsWith('drum-'));}
        function compare(variant){
            if(!preview)return;options.stopTransport();previewAudio.stop();
            const p=M.clone(variant?preview.p:JSON.parse(preview.base)),t=p.tracks.find(t=>t.id===preview.track),c=t.clips.find(c=>c.id===preview.clip);
            p.tracks=[t];t.clips=[c];c.start=0;t.automation=[];p.loop={enabled:true,start:0,end:c.length};
            const token=++epoch;previewAudio.unlock().then(()=>{if(token!==epoch)return;return previewAudio.play(p,0);}).catch(options.fail);
            host.querySelectorAll('[data-compose="original"],[data-compose="variant"]').forEach(b=>b.setAttribute('aria-pressed',String(b.dataset.compose===(variant?'variant':'original'))));
        }
        function pads(){
            const {p}=selection(),s=settings(),box=host.querySelector('.ss-chord-pads');
            box.innerHTML=Array.from({length:8},(_,i)=>'<button type="button" data-chord="'+i+'" aria-pressed="'+(i===degree)+'"><strong>'+['I','II','III','IV','V','VI','VII','I↑'][i]+'</strong><span>'+G.chord(p.harmony,i,!!s.seventh).map(n=>['C','C♯','D','D♯','E','F','F♯','G','G♯','A','A♯','B'][n%12]).join(' · ')+'</span></button>').join('');
        }
        async function padStart(button){
            stopLive();degree=Number(button.dataset.chord);host.querySelectorAll('[data-chord]').forEach(b=>b.setAttribute('aria-pressed',String(b===button)));
            const {p,track}=selection();if(!track)return;
            const pitches=G.chord(p.harmony,degree,!!settings().seventh),token=epoch;
            if(q('arp_live').checked){pitches.forEach(p=>hold('pad:'+p,p));return;}
            try{await options.audio.unlock();if(token!==epoch)return;padReleases=pitches.map(p=>options.audio.noteOn(track,p,.75));}catch(error){options.fail(error);}
        }
        host.addEventListener('pointerdown',e=>{const b=e.target.closest('[data-chord]');if(b&&e.button===0){e.preventDefault();b.focus();b.setPointerCapture(e.pointerId);padStart(b);}},{signal});
        host.addEventListener('pointerup',e=>{if(e.target.closest('[data-chord]')){epoch++;stopLive();}},{signal});
        host.addEventListener('pointercancel',()=>{epoch++;stopLive();},{signal});
        host.addEventListener('keydown',e=>{const b=e.target.closest('[data-chord]');if(b&&['Enter',' '].includes(e.key)){e.preventDefault();if(!e.repeat)padStart(b);}},{signal});
        host.addEventListener('keyup',e=>{if(e.target.closest('[data-chord]')&&['Enter',' '].includes(e.key)){e.preventDefault();epoch++;stopLive();}},{signal});
        host.addEventListener('click',e=>{const action=e.target.closest('[data-compose]')?.dataset.compose;try{
            if(action==='generate')generate();
            if(action==='cancel')cancel();
            if(action==='original'||action==='variant')compare(action==='variant');
            if(action==='apply'&&preview&&!options.blocked()){const current=preview;if(current.base!==JSON.stringify(options.project())){cancel();return;}const notes=current.p.tracks.find(t=>t.id===current.track).clips.find(c=>c.id===current.clip).notes;cancel();options.mutate(p=>{p.tracks.find(t=>t.id===current.track).clips.find(c=>c.id===current.clip).notes=notes;});options.showNotes();}
        }catch(error){options.fail(error);}},{signal});
        host.addEventListener('input',e=>{const out=e.target.parentElement.querySelector('output');if(out)out.textContent=e.target.value;},{signal});
        host.addEventListener('change',e=>{if(!e.target.dataset.music)return;cancel();stopLive();const s=settings();
            if(['root','scale','scale_snap'].includes(e.target.dataset.music))options.mutate(p=>{p.harmony={root:s.root,scale:s.scale,snap:s.scale_snap};},'metadata');
            pads();refresh();
        },{signal});
        function refresh(){
            const {p,track,clip}=selection();if(preview&&preview.base!==JSON.stringify(p))cancel();
            if(lastTrack!==track?.id){epoch++;stopLive();lastTrack=track?.id;}
            q('root').value=p.harmony.root;q('scale').value=p.harmony.scale;q('scale_snap').checked=p.harmony.snap;
            host.querySelector('[data-compose-target]').textContent=clip?.name||tr('select_clip');
            host.querySelector('[data-compose=generate]').disabled=options.blocked()||!clip||(q('generator').value==='rhythm'&&!isDrum(track));
            host.querySelector('[data-compose=apply]').disabled=options.blocked();
            host.querySelectorAll('input,select').forEach(e=>e.disabled=options.blocked());
            const h=JSON.stringify([p.harmony,settings().seventh]);if(h!==lastHarmony){lastHarmony=h;pads();}
        }
        signal.addEventListener('abort',()=>{stop();previewAudio.dispose();},{once:true});
        refresh();
        return {refresh,stop,hold,release,enabled:()=>q('arp_live').checked};
    }
    window.SynthStudioCompose={create};
})();

