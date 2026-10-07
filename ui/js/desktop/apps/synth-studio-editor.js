(function () {
    'use strict';
    const BAR = 1920, PPQ = 480;
    const uid = () => window.SynthStudioModel.newID('note');
    const esc = s => String(s).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const drums = () => window.SynthStudioPresets.drums;
    const isDrum = t => t && (t.instrument === 'drums-kit' || t.instrument === 'gm-drums' || t.instrument.startsWith('drum-'));
    function create(host, options) {
        const life = new AbortController(), signal = life.signal, tr = options.tr;
        host.tabIndex = 0;
        let trackId = '', clipId = '', selected = new Set(), beatWidth = 40, snap = 120, playingTick = 0, gesture = null;
        let previousClip = '', rollScroll = null;
        host.innerHTML = '<div class="ss-arrangement" data-arrangement><div class="ss-arrange-inner"></div></div><section class="ss-note-editor" data-note-editor></section>';
        const arrangement = host.querySelector('[data-arrangement]'), inner = arrangement.firstElementChild, editor = host.querySelector('[data-note-editor]');
        const get = () => { const project = options.getProject(); const track = project.tracks.find(t => t.id === trackId); return {project, track, clip:track?.clips.find(c => c.id === clipId)}; };
        const px = tick => tick / PPQ * beatWidth;
        const quantize = value => Math.max(0, Math.round(value / snap) * snap);
        const emit = () => options.onSelect?.({trackId,clipId,notes:[...selected]});
        function select(tid, cid = '') { trackId = tid; clipId = cid; selected.clear(); emit(); render(); }
        function change(fn) { if (options.readonly()) return; options.mutate(fn); }
        function render() {
            const {project,track,clip} = get();
            const end = Math.max(BAR * 8, ...project.tracks.flatMap(t => t.clips.map(c => c.start + c.length + BAR)));
            const bars = Math.ceil(end / BAR), width = px(bars * BAR), oldX = arrangement.scrollLeft, oldY = arrangement.scrollTop;
            const ruler = Array.from({length:bars},(_,i) => '<span style="left:'+px(i*BAR)+'px">'+(i+1)+'</span>').join('');
            inner.style.setProperty('--ss-beat', beatWidth+'px');
            inner.innerHTML = '<div class="ss-ruler"><div class="ss-track-label">'+esc(tr('tracks'))+'</div><div class="ss-ruler-grid" data-seek style="width:'+width+'px">'+ruler+'</div></div>' +
                project.tracks.map((t,i) => '<div class="ss-track '+(t.id===trackId?'is-selected':'')+'" data-track="'+t.id+'" style="--ss-track-color:var(--ss-color-'+(i%6)+')"><div class="ss-track-head"><button class="ss-track-name" data-select-track="'+t.id+'">'+esc(t.name)+'</button><div class="ss-track-buttons">'+['mute','solo'].map(a => '<button data-track-action="'+a+'" data-id="'+t.id+'" aria-label="'+esc(tr(a))+'" aria-pressed="'+!!t[a]+'">'+(a==='mute'?'M':'S')+'</button>').join('')+'<button data-track-action="arm" data-id="'+t.id+'" aria-label="'+esc(tr('arm'))+'" aria-pressed="'+(t.id===trackId)+'">●</button></div><input type="range" min="0" max="1" step=".01" value="'+t.volume+'" data-track-volume="'+t.id+'" aria-label="'+esc(tr('volume'))+'"'+(options.readonly()?' disabled':'')+'></div><div class="ss-lane" data-lane="'+t.id+'" style="width:'+width+'px">'+t.clips.map(c => '<div role="button" tabindex="0" class="ss-clip '+(c.id===clipId?'is-selected':'')+'" data-clip="'+c.id+'" data-track-id="'+t.id+'" aria-label="'+esc(c.name)+'" style="left:'+px(c.start)+'px;width:'+Math.max(16,px(c.length)-2)+'px"><span>'+esc(c.name)+'</span><div class="ss-clip-notes" aria-hidden="true">'+c.notes.slice(0,128).map(n => '<i style="left:'+(n.start/c.length*100)+'%;width:'+Math.max(.7,n.duration/c.length*100)+'%;top:'+((84-n.pitch)%24/24*20)+'px"></i>').join('')+'</div><b class="ss-clip-resize" data-resize aria-label="'+esc(tr('resize'))+'"></b></div>').join('')+'</div></div>').join('') +
                '<div class="ss-add-lane" style="min-width:'+(width+180)+'px" data-empty-drop><button data-add-track>＋ '+esc(tr('add_track'))+'</button><span>'+esc(tr('drop_hint'))+'</span></div><div class="ss-playhead" aria-hidden="true"></div>';
            arrangement.scrollLeft = oldX; arrangement.scrollTop = oldY;
            setPlayhead(playingTick);
            const oldRoll = editor.querySelector('.ss-roll-scroll'); if(oldRoll) rollScroll = {left:oldRoll.scrollLeft,top:oldRoll.scrollTop};
            if (!clip) {
                editor.innerHTML = '<div class="ss-editor-empty"><span aria-hidden="true">♫</span><strong>'+esc(tr('select_clip'))+'</strong><p>'+esc(tr('note_hint'))+'</p></div>'; return;
            }
            const drum = isDrum(track), pitches = drum ? drums().map(d => d.drumNote) : Array.from({length:128},(_,i) => 127-i), rowH = drum ? 25 : 17;
            // Imported GM percussion outside the custom kit remains editable.
            if(drum) clip.notes.forEach(n=>{if(!pitches.includes(n.pitch))pitches.push(n.pitch);});
            const noteWidth = Math.max(px(clip.length), 400), noteHeight = pitches.length*rowH;
            editor.innerHTML = '<header class="ss-editor-header"><strong>'+esc(clip.name)+'</strong><span>'+esc(tr(drum?'drum_editor':'piano_roll'))+'</span><button data-delete-notes'+(!selected.size||options.readonly()?' disabled':'')+'>'+esc(tr('delete'))+'</button><label>'+esc(tr('velocity'))+' <input type="range" data-velocity min=".05" max="1" step=".05" value="'+(clip.notes.find(n=>selected.has(n.id))?.velocity ?? .8)+'"'+(!selected.size||options.readonly()?' disabled':'')+'></label></header><div class="ss-roll-scroll"><div class="ss-roll" style="width:'+(noteWidth+92)+'px;height:'+noteHeight+'px;--ss-beat:'+beatWidth+'px;--ss-row:'+rowH+'px"><div class="ss-keys">'+pitches.map(p => { const d=drums().find(d=>d.drumNote===p); return '<button class="'+(!drum&&[1,3,6,8,10].includes(p%12)?'is-black':'')+'" data-key="'+p+'" style="height:'+rowH+'px">'+esc(drum?(d?.name||'GM '+p):noteName(p))+'</button>'; }).join('')+'</div><div class="ss-note-grid" data-grid style="width:'+noteWidth+'px;height:'+noteHeight+'px">'+clip.notes.map(n => {const y=pitches.indexOf(n.pitch); if(y<0)return ''; return '<div role="button" tabindex="0" data-note="'+n.id+'" class="ss-note '+(selected.has(n.id)?'is-selected':'')+'" style="left:'+px(n.start)+'px;top:'+(y*rowH+1)+'px;width:'+Math.max(6,px(n.duration)-1)+'px;height:'+(rowH-2)+'px;opacity:'+(.45+n.velocity*.55)+'" aria-label="'+esc(noteName(n.pitch))+'"><b data-note-resize></b></div>';}).join('')+'</div></div></div>';
            const scroll=editor.querySelector('.ss-roll-scroll');
            if(previousClip===clipId&&rollScroll){scroll.scrollTop=rollScroll.top;scroll.scrollLeft=rollScroll.left;} else scroll.scrollTop=drum?0:Math.max(0,(127-(clip.notes[0]?.pitch??72)-3)*rowH);
            previousClip=clipId;
            editor.dataset.pitches=JSON.stringify(pitches);editor.dataset.rowHeight=rowH;
        }
        function noteName(p) {return ['C','C♯','D','D♯','E','F','F♯','G','G♯','A','A♯','B'][p%12]+(Math.floor(p/12)-1);}
        function setPlayhead(tick) {playingTick=tick;const el=inner.querySelector('.ss-playhead');if(el)el.style.transform='translateX('+(180+px(tick))+'px)';}
        function deleteNotes() { if(!selected.size)return;change(p=>{const c=p.tracks.find(t=>t.id===trackId)?.clips.find(c=>c.id===clipId);if(c)c.notes=c.notes.filter(n=>!selected.has(n.id));});selected.clear();render(); }
        function dropItem(data, tid, tick) {options.onDrop?.(data,tid,quantize(tick));}
        host.addEventListener('dragover',e=>{if(options.readonly())return;e.preventDefault();e.dataTransfer.dropEffect='copy';},{signal});
        host.addEventListener('drop',e=>{
            if(options.readonly())return;e.preventDefault();
            const raw=e.dataTransfer.getData('application/x-aurago-synth');if(!raw)return;
            try { const data=JSON.parse(raw),lane=e.target.closest('[data-lane]');dropItem(data,lane?.dataset.lane||'',lane?(e.clientX-lane.getBoundingClientRect().left)/beatWidth*PPQ:0); } catch {options.onError?.('invalid_project');}
        },{signal});
        host.addEventListener('click',e=>{
            const target=e.target.closest('button');
            if(target?.dataset.selectTrack){select(target.dataset.selectTrack);return;}
            if(target?.hasAttribute('data-add-track')){options.onAdd?.();return;}
            if(target?.dataset.trackAction){const a=target.dataset.trackAction,id=target.dataset.id;if(a==='arm')select(id);else change(p=>{const t=p.tracks.find(t=>t.id===id);t[a]=!t[a];});return;}
            if(target?.hasAttribute('data-delete-notes'))deleteNotes();
            const ruler=e.target.closest('[data-seek]');if(ruler)options.onSeek?.(quantize((e.clientX-ruler.getBoundingClientRect().left)/beatWidth*PPQ));
        },{signal});
        host.addEventListener('change',e=>{
            if(e.target.dataset.trackVolume){const id=e.target.dataset.trackVolume,v=Number(e.target.value);change(p=>{p.tracks.find(t=>t.id===id).volume=v;});}
            if(e.target.hasAttribute('data-velocity')){const value=Number(e.target.value);change(p=>{p.tracks.find(t=>t.id===trackId)?.clips.find(c=>c.id===clipId)?.notes.forEach(n=>{if(selected.has(n.id))n.velocity=value;});});}
        },{signal});
        host.addEventListener('dblclick',e=>{
            const lane=e.target.closest('[data-lane]');if(!lane||e.target.closest('[data-clip]')||options.readonly())return;
            options.onNewClip?.(lane.dataset.lane,quantize((e.clientX-lane.getBoundingClientRect().left)/beatWidth*PPQ));
        },{signal});
        host.addEventListener('pointerdown',e=>{
            if(e.button!==0)return;
            if(!e.target.closest('input,select,button'))host.focus({preventScroll:true});
            const key=e.target.closest('[data-key]');if(key){e.preventDefault();options.onAudition?.(Number(key.dataset.key));return;}
            const box=e.target.closest('[data-clip]');
            if(box){
                e.preventDefault();e.stopPropagation();
                trackId=box.dataset.trackId;clipId=box.dataset.clip;selected.clear();emit();
                const c=get().clip;if(!c)return;
                gesture={kind:e.target.hasAttribute('data-resize')?'clipResize':'clipMove',x:e.clientX,y:e.clientY,clip:structuredClone(c),track:trackId,copy:e.altKey,repeat:e.shiftKey,element:box,moved:false};
                if(options.readonly()){gesture=null;render();}return;
            }
            const note=e.target.closest('[data-note]');
            if(note){
                e.preventDefault();e.stopPropagation();const n=get().clip?.notes.find(n=>n.id===note.dataset.note);if(!n)return;
                if(e.shiftKey){if(selected.has(n.id))selected.delete(n.id);else selected.add(n.id);}else if(!selected.has(n.id))selected=new Set([n.id]);
                emit();
                gesture={kind:e.target.hasAttribute('data-note-resize')?'noteResize':'noteMove',x:e.clientX,y:e.clientY,note:structuredClone(n),ids:[...selected],moved:false,element:note};
                if(options.readonly()){gesture=null;render();}return;
            }
            const grid=e.target.closest('[data-grid]');if(!grid||options.readonly())return;
            e.preventDefault();const {clip}=get();if(!clip)return;
            const rect=grid.getBoundingClientRect(),pitches=JSON.parse(editor.dataset.pitches),pitch=pitches[Math.floor((e.clientY-rect.top)/Number(editor.dataset.rowHeight))];
            const start=quantize((e.clientX-rect.left)/beatWidth*PPQ);if(pitch==null||start>=clip.length)return;
            const id=uid();selected=new Set([id]);change(p=>{p.tracks.find(t=>t.id===trackId).clips.find(c=>c.id===clipId).notes.push({id,pitch,start,duration:Math.min(snap,clip.length-start),velocity:.8});});emit();options.onAudition?.(pitch);
        },{signal});
        document.addEventListener('pointermove',e=>{
            if(!gesture)return;const dx=e.clientX-gesture.x,dy=e.clientY-gesture.y;
            if(Math.abs(dx)+Math.abs(dy)>3)gesture.moved=true;
            if(gesture.kind==='clipMove'||gesture.kind==='noteMove')gesture.element.style.transform='translate('+dx+'px,'+(gesture.kind==='noteMove'?dy:0)+'px)';
            else gesture.element.style.width=Math.max(6,px((gesture.clip?.length||gesture.note?.duration)+Math.round(dx/beatWidth*PPQ)))+'px';
        },{signal});
        document.addEventListener('pointerup',e=>{
            if(!gesture)return;const g=gesture;gesture=null;
            if(!g.moved){render();return;}const delta=Math.round((e.clientX-g.x)/beatWidth*PPQ/snap)*snap;
            if(g.kind.startsWith('clip')){
                const target=document.elementFromPoint(e.clientX,e.clientY)?.closest('[data-lane]')?.dataset.lane||g.track;
                change(p=>{
                    const src=p.tracks.find(t=>t.id===g.track),dst=p.tracks.find(t=>t.id===target)||src;
                    if(g.kind==='clipMove'){
                        if(isDrum(src)!==isDrum(dst))return;
                        const c=g.copy?window.SynthStudioModel.cloneClip(g.clip,Math.max(0,g.clip.start+delta)):src.clips.find(c=>c.id===g.clip.id);
                        if(!g.copy)src.clips=src.clips.filter(x=>x.id!==c.id);
                        c.start=Math.max(0,g.clip.start+delta);dst.clips.push(c);trackId=dst.id;clipId=c.id;
                    }else{
                        const c=src.clips.find(c=>c.id===g.clip.id),length=Math.max(snap,g.clip.length+delta);
                        if(g.repeat&&length>g.clip.length){
                            c.length=length;window.SynthStudioModel.validate(p);
                            const original=structuredClone(c.notes),controls=structuredClone(c.controllers);
                            if((original.length+controls.length)*Math.ceil(length/g.clip.length)>50000)throw Object.assign(new Error('Repeated clip exceeds event limit'),{code:'E_PROJECT_LIMIT'});
                            if(original.length||controls.length)for(let offset=g.clip.length;offset<length;offset+=g.clip.length){c.notes.push(...original.map(n=>({...n,id:uid(),start:n.start+offset})));c.controllers.push(...controls.map(a=>({...a,tick:a.tick+offset})));}
                        }
                        c.length=length;c.notes=c.notes.filter(n=>n.start<length).map(n=>({...n,duration:Math.min(n.duration,length-n.start)}));c.controllers=c.controllers.filter(c=>c.tick<length);
                    }
                });emit();
            }else{
                const pitches=JSON.parse(editor.dataset.pitches),dy=Math.round((e.clientY-g.y)/Number(editor.dataset.rowHeight));
                change(p=>{const c=p.tracks.find(t=>t.id===trackId)?.clips.find(c=>c.id===clipId);if(!c)return;
                    const notes=c.notes.filter(n=>g.ids.includes(n.id));if(!notes.length)return;
                    const bounded=Math.max(-Math.min(...notes.map(n=>n.start)),Math.min(delta,c.length-Math.max(...notes.map(n=>n.start+n.duration))));
                    notes.forEach(n=>{if(g.kind==='noteResize')n.duration=Math.max(1,Math.min(c.length-n.start,Math.max(snap,n.duration+delta)));else{n.start+=bounded;const i=pitches.indexOf(n.pitch);n.pitch=pitches[Math.max(0,Math.min(pitches.length-1,i+dy))];}});
                });
            }
            render();
        },{signal});
        document.addEventListener('pointercancel',()=>{gesture=null;render();},{signal});
        host.addEventListener('keydown',e=>{
            if(e.target.matches('input,select,textarea')||options.readonly())return;
            const {track,clip}=get();if(!clip)return;
            if(e.key==='Delete'||e.key==='Backspace'){e.preventDefault();if(selected.size)deleteNotes();else{change(p=>{const t=p.tracks.find(t=>t.id===trackId);t.clips=t.clips.filter(c=>c.id!==clipId);});clipId='';render();}return;}
            if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='d'){e.preventDefault();change(p=>{const t=p.tracks.find(t=>t.id===trackId),copy=window.SynthStudioModel.cloneClip(clip,clip.start+clip.length);t.clips.push(copy);clipId=copy.id;});return;}
            if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='a'&&e.target.closest('[data-note-editor]')){e.preventDefault();selected=new Set(clip.notes.map(n=>n.id));render();return;}
            if(['ArrowLeft','ArrowRight','ArrowUp','ArrowDown'].includes(e.key)){e.preventDefault();const dt=e.key==='ArrowLeft'?-snap:e.key==='ArrowRight'?snap:0,dp=e.key==='ArrowUp'?1:e.key==='ArrowDown'?-1:0;
                change(p=>{const c=p.tracks.find(t=>t.id===track.id).clips.find(c=>c.id===clip.id);if(!selected.size)c.start=Math.max(0,c.start+dt);else c.notes.forEach(n=>{if(selected.has(n.id)){n.start=Math.max(0,Math.min(c.length-n.duration,n.start+dt));n.pitch=Math.max(0,Math.min(127,n.pitch+dp));}});});
            }
        },{signal});
        return {render,select,selection:()=>({trackId,clipId,notes:[...selected]}),setPlayhead,setZoom(value){beatWidth=Math.max(20,Math.min(100,value));render();},setSnap(value){snap=Math.max(30,value);},dispose(){life.abort();gesture=null;host.replaceChildren();}};
    }
    window.SynthStudioEditor={create};
})();
