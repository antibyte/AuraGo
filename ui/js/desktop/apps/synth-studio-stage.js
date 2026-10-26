(function(){
    'use strict';
    function create(host,options){
        const {tr,esc,signal,audio}=options,M=window.SynthStudioModel,P=window.SynthStudioPerformance,BAR=1920;
        const section=document.createElement('section');section.className='ss-stage';host.prepend(section);
        section.innerHTML='<div class="ss-stage-heading"><strong>'+esc(tr('scenes'))+'</strong><button data-stage="sceneCreate">'+esc(tr('scene_from_loop'))+'</button><button data-stage="sceneRename">'+esc(tr('name'))+'</button><button data-stage="sceneDelete">'+esc(tr('delete'))+'</button></div><div class="ss-scene-pads"></div><div class="ss-macro"><div data-xy class="ss-xy" role="group" aria-label="'+esc(tr('xy_hint'))+'"><span class="ss-xy-dot"></span><span class="ss-xy-label">'+esc(tr('tone'))+' →<br>'+esc(tr('reverb'))+' ↑</span></div><div class="ss-macro-controls">'+['tone','reverb'].map(key=>'<label>'+esc(tr(key))+'<input data-macro="'+key+'" type="range" min="0" max="1" step=".01" value=".5"><output>.50</output></label>').join('')+'<label><input data-automation-record type="checkbox"> '+esc(tr('automation_record'))+'</label><button data-stage="clearAutomation">'+esc(tr('automation_clear'))+'</button></div></div><details class="ss-automation-editor"><summary>'+esc(tr('automation_edit'))+'</summary><div class="ss-automation-fields"><select data-automation-parameter aria-label="'+esc(tr('automation_edit'))+'"><option value="tone">'+esc(tr('tone'))+'</option><option value="reverb">'+esc(tr('reverb'))+'</option></select><label>'+esc(tr('automation_point'))+'<input data-automation-index type="number" min="1" step="1" value="1"><output data-automation-count></output></label><label>'+esc(tr('bar_position'))+'<input data-automation-time type="number" min="1" step=".0625"></label><label>'+esc(tr('value'))+'<input data-automation-value type="number" min="0" max="1" step=".01"></label><button data-stage="automationApply">'+esc(tr('apply'))+'</button><button data-stage="automationDelete">'+esc(tr('delete'))+'</button></div></details><div class="ss-take-controls"><button data-stage="takeRecord">'+esc(tr('take_record'))+'</button><button data-stage="takeStop">'+esc(tr('stop'))+'</button><span data-take-status role="status"></span><button data-stage="takeAdopt">'+esc(tr('take_adopt'))+'</button><button data-stage="takeDiscard">'+esc(tr('cancel'))+'</button></div>';
        let active='',queued='',selected='',take=null,pendingTake=null,takeTick=0,gesture=null,epoch=0,levelTimer=null,draftWrite=Promise.resolve(),lastSceneMarkup='',automationTrack='',draftSaved=false,draftError=false,lastDisplay=-Infinity;
        const draftKey=location.origin+':synth-studio:take:'+options.sessionKey;
        const q=s=>section.querySelector(s),track=()=>options.track(),playing=()=>options.playing();
        const editable=()=>options.editable();
        const range=()=>{
            const p=options.project(),scene=p.scenes.find(s=>s.id===(queued||active));
            if(scene)return scene;
            if(p.loop.enabled)return {...p.loop,name:p.name};
            const start=Math.floor(options.cursor()/BAR)*BAR,end=Math.max(start+BAR,...p.tracks.flatMap(t=>t.clips.map(c=>Math.ceil((c.start+c.length)/BAR)*BAR)));
            return {start,end,name:p.name};
        };
        function runtime(p){const scene=p.scenes.find(s=>s.id===(queued||active));return scene?{...p,loop:{enabled:true,start:scene.start,end:scene.end}}:p;}
        function sceneApplied(id=queued){if(id){active=id;if(queued===id)queued='';refresh();}}
        function saveDraft(){if(!pendingTake)return draftWrite;draftSaved=false;draftError=false;const value={id:M.newID('draft'),content:JSON.stringify(pendingTake),updated:Date.now()};draftWrite=draftWrite.catch(()=>{}).then(()=>OfficeSession.draft('put',draftKey,value,'synth-studio'));draftWrite.then(()=>{draftSaved=true;refresh();},error=>{draftError=true;refresh();options.fail(error);});return draftWrite;}
        async function clearDraft(snapshot){await draftWrite.catch(()=>{});const d=await OfficeSession.draft('get',draftKey,undefined,'synth-studio');if(d?.content===JSON.stringify(snapshot))await OfficeSession.draft('deleteIf',draftKey,{id:d.id},'synth-studio');}
        function update(values){
            const t=options.project().tracks.find(t=>t.id===(gesture?.id||track()?.id));if(!t)return;const p={...options.project(),tracks:options.project().tracks.map(t=>({...t,params:{...t.params}}))},target=p.tracks.find(v=>v.id===t.id);Object.assign(target.params,values);audio.updateTracks(p);
            showValues(target.params);
        }
        function showValues(values){
            for(const key of ['tone','reverb']){const input=q('[data-macro="'+key+'"]');input.value=values[key];input.parentElement.querySelector('output').textContent=values[key].toFixed(2);}
            q('.ss-xy-dot').style.left=values.tone*100+'%';q('.ss-xy-dot').style.bottom=values.reverb*100+'%';
        }
        function gestureTime(){return take?audio.elapsed()*options.project().tempo/60*480:audio.currentTick();}
        function begin(control){
            if(!['tone','reverb'].includes(control.key)||!editable()||!track())return false;
            if(gesture)return true;
            const t=track(),keys=control.xy?['tone','reverb']:[control.key],record=(take||q('[data-automation-record]').checked)&&playing();
            gesture={mode:control.mode,external:control.external===true,id:t.id,keys,values:{...t.params},points:[],record,last:-Infinity,begin:gestureTime()};
            keys.forEach(key=>{audio.setManual(t.id,key,true);gesture.points.push({tick:gesture.begin,parameter:key,value:playing()?P.valueAt(t,key,audio.currentTick()):t.params[key]});});
            return true;
        }
        function sample(force=false){
            if(!gesture?.record)return;
            const now=audio.clock();if(!force&&now-gesture.last<.05)return;gesture.last=now;
            for(const parameter of gesture.keys)gesture.points.push({tick:gestureTime(),parameter,value:gesture.values[parameter]});

        }
        function preview(control){
            if(!gesture||gesture.id!==control.id||!gesture.keys.includes(control.key))return false;
            gesture.values[control.key]=control.value;update(Object.fromEntries(gesture.keys.map(k=>[k,gesture.values[k]])));sample();return true;
        }
        function end(cancel=false){
            if(!gesture)return false;
            if(!cancel)sample(true);
            const g=gesture;gesture=null;
            try{
                if(!cancel&&(editable()||g.record&&take)){
                    if(g.record&&take){
                        const end=Math.min(take.limit,Math.max(1,Math.round(audio.elapsed()*options.project().tempo/60*480)));
                        const original=options.project().tracks.find(t=>t.id===g.id);
                        for(const parameter of g.keys)g.points.push({tick:end+1,parameter,value:P.valueAt(original,parameter,audio.currentTick())});
                        const limit=take.automation(g.id,g.points,end);audio.setStopAfter(limit*60/options.project().tempo/480);
                    }else if(g.record){
                        options.mutate(p=>P.mergeGesture(p.tracks.find(t=>t.id===g.id),g.points),'automation');
                    }else{
                        const t=options.project().tracks.find(t=>t.id===g.id);
                        const keys=g.keys.filter(k=>!playing()||!t.automation.some(p=>p.parameter===k));
                        if(keys.length)options.mutate(p=>{const target=p.tracks.find(t=>t.id===g.id);keys.forEach(k=>target.params[k]=g.values[k]);},'mix');
                    }
                }
            }catch(error){options.fail(error);if(take)options.stop();}
            finally{audio.updateTracks(options.project());g.keys.forEach(k=>audio.setManual(g.id,k,false));refresh();}
            return true;
        }
        function finishTake(){
            if(!take)return;const current=take;take=null;
            if(takeTick===0&&audio.elapsed()===0)return;
            const until=Math.max(1,Math.min(current.limit,Math.round(Math.max(takeTick,audio.elapsed()*options.project().tempo/60*480))));
            try{pendingTake=current.render(until);pendingTake.name=(options.project().name+' · '+tr('take')).slice(0,96);window.SynthStudioAudio.prepareRender(pendingTake);saveDraft();}catch(error){options.fail(error);}
        }
        function stop(){epoch++;end(false);finishTake();active='';queued='';refresh();}
        function onTick(tick,meta){
            const t=track(),now=audio.clock();if(t&&!gesture&&now-lastDisplay>=.1){lastDisplay=now;showValues({tone:P.valueAt(t,'tone',tick),reverb:P.valueAt(t,'reverb',tick)});}
            if(take){takeTick=Math.min(take.limit,Math.max(0,meta.elapsedSeconds*options.project().tempo/60*480));q('[data-take-status]').textContent=tr('remaining')+' '+Math.max(0,Math.ceil((take.limit-takeTick)/BAR))+' '+tr('bars');}
        }
        async function launch(id){
            const p=options.project(),scene=p.scenes.find(s=>s.id===id);if(!scene||!options.canPlay())return;
            selected=id;
            if(playing()){
                const previous=queued;queued=id;let request;
                try{request=audio.queueProject({...p,loop:{enabled:true,start:scene.start,end:scene.end}},scene.start,()=>sceneApplied(id));}catch(error){queued=previous;refresh();throw error;}
                if(!request){queued='';refresh();return;}
                if(take&&request){try{const limit=take.scene(scene,request.elapsedSeconds*p.tempo/60*480);audio.setStopAfter(limit*60/p.tempo/480);}catch(error){options.stop();options.fail(error);return;}}
            }else{
                const token=++epoch;queued=id;
                options.play(scene.start,{...p,loop:{enabled:true,start:scene.start,end:scene.end}},()=>{if(token!==epoch)return;active=id;queued='';refresh();});
            }
            refresh();
        }
        function automationPoint(){
            const parameter=q('[data-automation-parameter]').value,points=(track()?.automation||[]).filter(p=>p.parameter===parameter),input=q('[data-automation-index]');
            const index=Math.max(0,Math.min(points.length-1,Math.floor(Number(input.value)||1)-1));
            return {points,index,point:points[index]};
        }
        function refreshAutomation(){
            if(automationTrack!==track()?.id){automationTrack=track()?.id;q('[data-automation-index]').value=1;}
            const {points,index,point}=automationPoint(),locked=!editable()||!!take||!point;
            q('[data-automation-index]').max=Math.max(1,points.length);q('[data-automation-index]').value=index+1;
            q('[data-automation-count]').textContent='/ '+points.length;
            q('[data-automation-time]').value=point?Number((point.tick/BAR+1).toFixed(4)):'';
            q('[data-automation-value]').value=point?point.value:'';
            for(const selector of ['[data-automation-time]','[data-automation-value]','[data-stage=automationApply]','[data-stage=automationDelete]'])q(selector).disabled=locked;
        }
        function editAutomation(remove=false){
            if(!editable()||take||!track())return;const {point}=automationPoint();if(!point)return;
            const id=track().id,tick=Math.round((Number(q('[data-automation-time]').value)-1)*BAR),value=Number(q('[data-automation-value]').value);
            options.mutate(p=>{const t=p.tracks.find(t=>t.id===id),index=t.automation.findIndex(v=>v.tick===point.tick&&v.parameter===point.parameter);if(index<0)return;
                if(remove)t.automation.splice(index,1);else t.automation[index]={tick,parameter:point.parameter,value};
            },'automation');
        }
        const actions={
            async sceneCreate(){if(!editable()||take)return;const p=options.project(),token=epoch;if(p.scenes.length>=8)return;const name=await options.prompt(tr('scene_name'),tr('scene')+' '+(p.scenes.length+1));if(!name?.trim()||!editable()||token!==epoch)return;const id=M.newID('scene');options.mutate(p=>p.scenes.push({id,name:name.trim().slice(0,96),start:p.loop.start,end:p.loop.end}),'metadata');selected=id;refresh();},
            async sceneRename(){if(!editable()||take)return;const s=options.project().scenes.find(s=>s.id===selected),token=epoch;if(!s)return;const name=await options.prompt(tr('scene_name'),s.name);if(name?.trim()&&editable()&&token===epoch)options.mutate(p=>{const scene=p.scenes.find(x=>x.id===s.id);if(scene)scene.name=name.trim().slice(0,96);},'metadata');},
            sceneDelete(){if(!editable()||take)return;if(selected===active||selected===queued)options.stop();options.mutate(p=>p.scenes=p.scenes.filter(s=>s.id!==selected),'metadata');selected='';refresh();},
            automationApply(){editAutomation();},
            automationDelete(){editAutomation(true);},
            clearAutomation(){const t=track();if(!editable()||take||!t)return;options.mutate(p=>p.tracks.find(v=>v.id===t.id).automation=[],'automation');},
            async takeRecord(){
                if(!editable()||take)return;const requestEpoch=epoch;
                if(pendingTake&&!await options.confirm(tr('take'),tr('replace_take')))return;
                if(!editable()||requestEpoch!==epoch)return;
                const r=range();options.stop();
                try{const recorder=P.createTake(options.project(),r,window.SynthStudioAudio.prepareRender);take=recorder;takeTick=0;pendingTake=null;const token=++epoch;
                    options.play(r.start,{...options.project(),loop:{enabled:true,start:r.start,end:r.end}},()=>{if(token!==epoch||take!==recorder)return;audio.setStopAfter(recorder.limit*60/options.project().tempo/480);active=options.project().scenes.find(s=>s.id===r.id)?.id||'';refresh();});options.refresh();
                }catch(error){take=null;options.fail(error);}
            },
            takeStop(){options.stop();},
            async takeAdopt(){if(!pendingTake||!editable()||take)return;const snapshot=pendingTake;if(await options.adopt(M.clone(snapshot))){if(pendingTake===snapshot)pendingTake=null;await clearDraft(snapshot);refresh();}},
            async takeDiscard(){if(!editable()||take)return;const snapshot=pendingTake;pendingTake=null;await clearDraft(snapshot);refresh();}
        };
        section.addEventListener('click',e=>{const scene=e.target.closest('[data-scene]');if(scene){launch(scene.dataset.scene).catch(options.fail);return;}const action=e.target.closest('[data-stage]')?.dataset.stage;if(action)Promise.resolve().then(()=>actions[action]?.()).catch(options.fail);},{signal});
        let pointer=null;
        const xy=q('[data-xy]');
        function move(e){if(pointer!==e.pointerId||!gesture)return;const rect=xy.getBoundingClientRect();gesture.values.tone=Math.max(0,Math.min(1,(e.clientX-rect.left)/rect.width));gesture.values.reverb=Math.max(0,Math.min(1,1-(e.clientY-rect.top)/rect.height));update({tone:gesture.values.tone,reverb:gesture.values.reverb});sample();}
        xy.addEventListener('pointerdown',e=>{if(e.button!==0||!begin({id:track()?.id,key:'tone',xy:true}))return;pointer=e.pointerId;xy.setPointerCapture(pointer);e.preventDefault();move(e);},{signal});
        xy.addEventListener('pointermove',move,{signal});xy.addEventListener('pointerup',e=>{if(pointer===e.pointerId){move(e);pointer=null;end();}},{signal});
        xy.addEventListener('pointercancel',()=>{pointer=null;end(true);},{signal});
        section.addEventListener('pointerdown',e=>{if(e.target.dataset.macro)begin({id:track()?.id,key:e.target.dataset.macro,mode:'pointer'});},{signal});
        section.addEventListener('input',e=>{const key=e.target.dataset.macro;if(key){if(!gesture)begin({id:track()?.id,key,mode:'change'});preview({id:track()?.id,key,value:Number(e.target.value)});}},{signal});
        section.addEventListener('change',e=>{if(e.target.dataset.macro&&gesture?.mode==='change')end();if(e.target.matches('[data-automation-parameter],[data-automation-index]'))refreshAutomation();},{signal});
        section.addEventListener('keydown',e=>{if(e.key==='Escape'&&gesture){e.preventDefault();end(true);return;}if(e.target.dataset.macro&&['ArrowLeft','ArrowRight','ArrowUp','ArrowDown','Home','End','PageUp','PageDown'].includes(e.key))begin({id:track()?.id,key:e.target.dataset.macro,mode:'keyboard'});},{signal});
        section.addEventListener('keyup',e=>{if(e.target.dataset.macro&&gesture?.mode==='keyboard')end();},{signal});
        window.addEventListener('pointerup',()=>{if(gesture&&!gesture.external&&pointer===null)end();},{signal});
        window.addEventListener('pointercancel',()=>{if(gesture&&!gesture.external){pointer=null;end(true);}},{signal});
        section.addEventListener('focusout',e=>{if(e.target.dataset.macro&&gesture?.mode==='keyboard')end();},{signal});
        function refresh(){
            const p=options.project(),t=track();
            refreshAutomation();
            if(gesture&&gesture.id!==t?.id)end(true);
            const markup=Array.from({length:8},(_,i)=>{const s=p.scenes[i],state=s?.id===queued?'queued':s?.id===active?'active':'stopped';return '<button type="button" '+(s?'data-scene="'+s.id+'"'+(!options.canPlay()?' disabled':''):'disabled')+' class="'+(selected===s?.id?'is-selected ':'')+state+'" aria-pressed="'+(state==='active')+'"><strong>'+esc(s?.name||'—')+'</strong><span>'+esc(tr(state))+'</span></button>';}).join('');
            if(markup!==lastSceneMarkup){const focus=document.activeElement?.dataset.scene;lastSceneMarkup=markup;q('.ss-scene-pads').innerHTML=markup;if(focus)q('[data-scene="'+focus+'"]')?.focus({preventScroll:true});}
            for(const name of ['sceneCreate','sceneRename','sceneDelete','clearAutomation'])q('[data-stage="'+name+'"]').disabled=!editable()||!!take||(name==='sceneCreate'&&p.scenes.length>=8);
            q('[data-stage=takeRecord]').disabled=!editable()||!!take;q('[data-stage=takeStop]').disabled=!take;
            q('[data-stage=takeAdopt]').disabled=!pendingTake||!editable()||!!take;q('[data-stage=takeDiscard]').disabled=!pendingTake||!editable()||!!take;
            q('[data-automation-record]').disabled=!editable()||!!take;
            q('[data-xy]').setAttribute('aria-disabled',String(!t||!editable()));
            q('[data-take-status]').textContent=take?tr('recording'):pendingTake?tr(draftError?'save_error':draftSaved?'take_ready':'saving'):tr('stopped');
            for(const key of ['tone','reverb'])q('[data-macro="'+key+'"]').disabled=!t||!editable();
            if(t&&!gesture)showValues(playing()?{tone:P.valueAt(t,'tone',audio.currentTick()),reverb:P.valueAt(t,'reverb',audio.currentTick())}:t.params);
        }
        const meter=document.createElement('div');meter.className='ss-output-meter';meter.innerHTML='<meter min="0" max="1" value="0" aria-label="'+esc(tr('output_level'))+'"></meter><span>'+esc(tr('output_level'))+'</span><span data-clip-light>'+esc(tr('clipping'))+'</span>';options.transport.append(meter);
        let peakUntil=0;
        levelTimer=setInterval(()=>{const value=audio.meter();meter.querySelector('meter').value=Math.min(1,value);if(value>=.99)peakUntil=performance.now()+1200;meter.classList.toggle('is-clipping',performance.now()<peakUntil);},100);
        Promise.resolve().then(()=>options.whenReady()).then(async()=>{const recoveryEpoch=epoch,d=await OfficeSession.draft('get',draftKey,undefined,'synth-studio');if(d?.content&&epoch===recoveryEpoch&&!signal.aborted&&await options.confirm(tr('take'),tr('recover_prompt'))&&!signal.aborted&&epoch===recoveryEpoch){pendingTake=M.validate(d.content);draftSaved=true;refresh();}}).catch(options.fail);
        signal.addEventListener('abort',()=>{stop();clearInterval(levelTimer);},{once:true});
        refresh();
        return {runtime,sceneApplied,onTick,stop,refresh,begin,preview,end,active:()=>!!take,guard:()=>saveDraft(),readyTake:()=>pendingTake&&M.clone(pendingTake)};
    }
    window.SynthStudioStage={create};
})();
