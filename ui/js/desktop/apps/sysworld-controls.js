(function(){
    'use strict';
    const NS=window.SysWorld=window.SysWorld||{};
    NS.createWorldControls=function(inst){
        const root=inst.root,L=key=>inst.L('sysworld.world.'+key),abort=new AbortController();
        let disposed=false,replay=null,playing=false,history=[],request=0,world=null,actionPending=null,inside=false,lastInteraction='',lastDiscoveries=-1;
        const timer=setInterval(()=>{if(playing&&inst.visible){range.value=Math.min(0,Number(range.value)+5);void loadReplay();}},2000);
        const el=(tag,cls,text)=>{const n=document.createElement(tag);n.className=cls||'';if(text!=null)n.textContent=text;return n;};
        const button=(label,fn)=>{const n=el('button','sw-btn',label);n.type='button';n.addEventListener('click',fn);return n;};
        const panel=el('details','sw-world sw-glass');panel.append(el('summary','',L('explore')));root.append(panel);
        const content=el('div','sw-world-body');panel.append(content);
        const notice=el('p','sw-world-notice');notice.setAttribute('role','status');content.append(notice);
        const selects={};
        const select=(key,values)=>{const label=el('label','sw-world-field',L(key)),input=el('select','sw-quality');input.setAttribute('aria-label',L(key));for(const value of values)input.add(new Option(L(value),value));label.append(input);content.append(label);selects[key]=input;return input;};
        const time=select('time',['local','day','evening','night']),weather=select('weather',['clear','rain','fog']);
        try{const saved=JSON.parse(localStorage.getItem('aurago.desktop.sysworld.environment')||'{}');if(['local','day','evening','night'].includes(saved.time))time.value=saved.time;if(['clear','rain','fog'].includes(saved.weather))weather.value=saved.weather;}catch(_){}
        const environment=()=>{const value={time:time.value,weather:weather.value};inst.city?.setEnvironment(value);inst.sound?.setEnvironment(inside,weather.value);try{localStorage.setItem('aurago.desktop.sysworld.environment',JSON.stringify(value));}catch(_){} };
        time.addEventListener('change',environment);weather.addEventListener('change',environment);
        const sound=el('details','sw-world-section');sound.append(el('summary','',L('audio')));
        for(const key of['ambience','effects','voice']){const label=el('label','sw-world-field',L(key)),input=el('input');input.type='range';input.min=0;input.max=100;input.value=100;input.setAttribute('aria-label',L(key));input.addEventListener('input',()=>inst.sound?.setChannel(key,Number(input.value)/100));label.append(input);sound.append(label);}content.append(sound);
        const destinations=el('div','sw-world-destinations');content.append(el('h3','',L('stations')),destinations);
        for(const[id,key]of NS.districts){const row=el('div','sw-world-destination');const stop=button(inst.L(key),()=>{inst.city?.visit(id);panel.open=false;});stop.dataset.worldStation=id;row.append(stop);if(['agent','memory','missions'].includes(id)){const room=button(L('interior'),()=>{inst.city?.enter(id);panel.open=false;});room.dataset.worldRoom=id;row.append(room);}destinations.append(row);}
        destinations.append(button(L('interact_drone'),()=>{inst.city?.visit('drone');panel.open=false;}));
        const discoveries=el('p','sw-muted');content.append(discoveries);
        const actions=el('details','sw-world-section');actions.append(el('summary','',L('terminal')));
        const target=el('select','sw-world-target');target.setAttribute('aria-label',L('target'));const verbs=el('div','sw-world-verbs');actions.append(target,verbs);content.append(actions);
        function showActions(){
            const entity=world?.entities?.find(e=>e.id===target.value);verbs.replaceChildren();
            if(!entity){verbs.append(el('p','sw-muted',L('no_actions')));return;}
            for(const verb of entity.actions||[]){const n=button(L(verb),()=>runAction(entity,verb));n.disabled=inst.replaying||!!actionPending||!!inst.ctx.readonly;verbs.append(n);}
            if(inst.replaying)verbs.append(el('p','sw-muted',L('replay_readonly')));
            if(entity.kind==='mission'&&inst.ctx.openApp)verbs.append(button(inst.L('desktop.context_open'),()=>inst.ctx.openApp('mission-control')));
            else if(['container','daemon'].includes(entity.kind)){const link=el('a','sw-btn',inst.L('desktop.context_open'));link.href=entity.kind==='container'?'/containers':'/skills';link.target='_blank';link.rel='noopener';verbs.append(link);}
        }
        target.addEventListener('change',showActions);
        async function runAction(entity,verb){
            if(inst.replaying||actionPending||inst.ctx.readonly)return;
            actionPending={entity:entity.id,verb,at:Date.now(),confirming:true};showActions();
            let confirmed=verb==='start';
            if(!confirmed){const text=L('confirm')+'\n'+entity.label+' · '+L(verb);confirmed=inst.ctx.confirmDialog?await inst.ctx.confirmDialog(L('terminal'),text):window.confirm(text);}
            if(!confirmed||disposed||inst.replaying){actionPending=null;showActions();return;}
            actionPending={entity:entity.id,verb,at:Date.now()};notice.textContent=L('pending');showActions();
            try{
                if(!inst.ctx.api)throw Error('Unavailable');
                const result=await inst.ctx.api('/api/desktop/system-world/actions',{method:'POST',signal:abort.signal,headers:{'Content-Type':'application/json'},body:JSON.stringify({entity:entity.id,action:verb,request_id:crypto.randomUUID(),confirmed})});
                if(disposed)return;if(!['accepted','completed'].includes(result.status))throw Error('Rejected');notice.textContent=L(result.status);if(result.status==='completed')actionPending=null;NS.data.refresh();showActions();
            }catch(_){if(!disposed){notice.textContent=L('failed');actionPending=null;showActions();}}
        }
        const timeline=el('details','sw-world-section');timeline.append(el('summary','',L('history')));content.append(timeline);
        const timeLabel=el('p','sw-muted',L('live')),range=el('input');range.type='range';range.min=-1440;range.max=0;range.value=0;range.step=1;range.setAttribute('aria-label',L('history'));
        const chart=el('canvas','sw-history-chart');chart.width=600;chart.height=100;chart.setAttribute('role','img');chart.setAttribute('aria-label',L('history_chart'));
        const playback=el('div','sw-world-verbs');
        const live=button(L('live'),()=>{request++;setReplay(null);}),play=button(L('play'),()=>{playing=!playing;if(!replay)range.value=-60;play.textContent=L(playing?'pause':'play');if(playing)void loadReplay();});
        const eventList=el('ol','sw-history-events');eventList.setAttribute('aria-label',inst.L('sysworld.city.events'));
        playback.append(live,play);timeline.append(chart,range,timeLabel,playback,eventList);
        const fetchJSON=async path=>{const response=await fetch('/api/desktop/system-world/'+path,{credentials:'same-origin',cache:'no-store',signal:abort.signal});if(!response.ok)throw Error('Unavailable');return response.json();};
        async function loadHistory(){try{history=await fetchJSON('history');if(!disposed){drawHistory();void loadEvents(replay?.at||Date.now());}}catch(_){if(!disposed)timeLabel.textContent=L('gap');}}
        let eventRequest=0;
        async function loadEvents(at){const token=++eventRequest;try{const events=await fetchJSON('events?since='+Math.floor(at-300000));if(disposed||token!==eventRequest)return;eventList.replaceChildren();for(const event of events.filter(e=>e.at<=at).slice(-12).reverse()){const e=event.entity,row=el('li');row.append(button(new Date(event.at).toLocaleTimeString()+' · '+(e.label||e.id)+' · '+e.state,()=>{range.value=Math.max(-1440,Math.floor((event.at-Date.now())/60000));void loadReplay();}));eventList.append(row);}}catch(_){if(!disposed&&token===eventRequest)eventList.replaceChildren();}}
        timeline.addEventListener('toggle',()=>{if(timeline.open)void loadHistory();});
        function drawHistory(){
            const ctx=chart.getContext('2d');ctx.clearRect(0,0,600,100);const since=Date.now()-86400000;
            for(const[key,color]of[['cpu','#62c8ba'],['ram','#d9ac6a']]){ctx.strokeStyle=color;ctx.lineWidth=2;ctx.beginPath();let last=0;for(const p of history.filter(v=>v.key===key)){const x=(p.at-since)/86400000*600,y=98-Math.min(100,p.average)*.94;if(p.at-last>90000)ctx.moveTo(x,y);else ctx.lineTo(x,y);last=p.at;}ctx.stroke();}
        }
        function setReplay(snapshot){
            replay=snapshot;inst.replaying=!!snapshot;
            if(!snapshot){playing=false;range.value=0;timeLabel.textContent=L('live');play.textContent=L('play');inst.applyWorldSnapshot?.(null);}
            else{timeLabel.textContent=new Date(snapshot.at).toLocaleString();inst.applyWorldSnapshot?.(snapshot);}
            showActions();if(timeline.open)void loadEvents(snapshot?.at||Date.now());
        }
        async function loadReplay(){const sequence=++request;if(Number(range.value)===0){setReplay(null);return;}inst.replaying=true;showActions();const at=Date.now()+Number(range.value)*60000;try{const snap=await fetchJSON('snapshot?at='+Math.floor(at));if(disposed||sequence!==request)return;setReplay(snap);}catch(_){if(!disposed&&sequence===request){playing=false;replay={at,metrics:{},entities:[]};inst.applyWorldSnapshot?.(replay);timeLabel.textContent=L('gap');play.textContent=L('play');}}}
        range.addEventListener('change',()=>void loadReplay());
        const interaction=button('',()=>inst.city?.interact());interaction.className='sw-interaction sw-glass';interaction.hidden=true;root.append(interaction);
        return {
            ready:environment,
            terminal(id){panel.open=true;actions.open=true;inst.select(id,false);},
            discover(id){panel.open=true;notice.textContent=inst.L(NS.districts.find(d=>d[0]===id)?.[1]||'sysworld.zone.core')+' — '+L('about_'+id);},
            environment(value){if(inside===value)return;inside=value;inst.sound?.setEnvironment(value,weather.value);},
            interaction(near,count){
                const kind=near?.kind||'';
                if(kind!==lastInteraction){lastInteraction=kind;interaction.hidden=!near;interaction.textContent=near?L('interact_'+kind)+' · E':'';interaction.dataset.kind=kind;}
                if(count!==lastDiscoveries){lastDiscoveries=count;discoveries.textContent=L('discoveries')+': '+count+' / 7';}
            },
            update(snapshot){
                world=snapshot;const previous=target.value;target.replaceChildren();
                for(const e of world?.entities||[])if(e.actions?.length)target.add(new Option(e.label||e.id,e.id));if([...target.options].some(o=>o.value===previous))target.value=previous;
                if(actionPending&&!actionPending.confirming){const e=world?.entities?.find(e=>e.id===actionPending.entity),expected=actionPending.verb==='start'?'running':null;
                    if(e&&e.at>actionPending.at&&((expected&&e.state===expected)||(!expected&&actionPending.verb!=='restart'&&['stopped','exited','cancelled'].includes(e.state)))){notice.textContent=L('completed');actionPending=null;}
                    else if(Date.now()-actionPending.at>60000){notice.textContent=L('unconfirmed');actionPending=null;}
                }showActions();
            },
            dispose(){disposed=true;clearInterval(timer);abort.abort();panel.remove();interaction.remove();},
        };
    };
})();
