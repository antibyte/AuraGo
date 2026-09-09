/* One installation-wide plant, one click-through desktop overlay. */
(function () {
    'use strict';
    let current = null;
    function mount(context) {
        current?.dispose();
        const {api,t,esc,confirm,readonly} = context, tr = key => t('desktop.leafy_'+key);
        const lifecycle = new AbortController(), signal = lifecycle.signal, motion = matchMedia('(prefers-reduced-motion: reduce)');
        const root = document.createElement('section'); root.className='vd-leafy'; root.setAttribute('aria-label','Leafy');
        root.innerHTML='<canvas class="vd-leafy-canvas" aria-hidden="true"></canvas><button class="vd-leafy-pot" type="button"></button><button class="vd-leafy-move" type="button"></button>';
        document.body.appendChild(root);
        let canvas=root.querySelector('canvas');
        const pot=root.querySelector('.vd-leafy-pot'),move=root.querySelector('.vd-leafy-move');
        pot.setAttribute('aria-label',tr('care')); move.textContent='⠿'; move.title=tr('move'); move.setAttribute('aria-label',tr('move'));
        const panel=document.createElement('section'); panel.className='vd-leafy-panel'; panel.setAttribute('aria-label',tr('care')); panel.hidden=true; document.body.appendChild(panel);
        const shell=document.createElement('button'); shell.type='button'; shell.className='vd-leafy-shell'; shell.title=tr('care'); shell.setAttribute('aria-label',tr('care')); shell.setAttribute('aria-expanded','false');
        shell.innerHTML='<img src="'+window.AuraLazyAssets.versionedURL('/img/leafy/icon.svg')+'" alt="">';
        (document.querySelector('.vd-taskbar-system')||document.body).prepend(shell);
        let snapshot=null, renderer=null, fallback=false, dead=false, busy=false, hidden=false, scissors=false, selection=null, anchor={x:.40,y:.91};
        let poll=0,breeze=0,frame=0,resize=0,undoTimer=0,fetching=false,pendingRefresh=false,drag=null;
        let pendingAction=null, message='', bounds={width:innerWidth,height:innerHeight}, renderKey='';
        try {const saved=JSON.parse(localStorage.getItem('aurago.leafy.anchor.v1')); if(Number.isFinite(saved?.x)&&Number.isFinite(saved?.y))anchor=saved;} catch (_) {}
        const clamp=(n,min,max)=>Math.max(min,Math.min(max,n));
        const plant=()=>snapshot?.plant;
        const visible=()=>!dead&&!document.hidden&&!hidden&&document.body.dataset.widgets!=='false';
        const animated=()=>visible()&&!fallback&&!motion.matches&&document.body.dataset.animations!=='false'&&!plant()?.dead&&!plant()?.vacation_at;
        function listen(target,type,handler,options={}) {target.addEventListener(type,handler,{...options,signal});}
        function stopMotion() {clearTimeout(breeze);cancelAnimationFrame(frame);breeze=frame=0;}
        function scheduleBreeze() {
            stopMotion();
            if(!animated()||!renderer)return;
            breeze=setTimeout(()=>{
                let start=0,last=0;
                const tick=now=>{
                    if(!animated()||!renderer)return;
                    if(!start)start=now;
                    if(now-last>=34){renderer.render(Math.sin((now-start)/850)*Math.sin(Math.PI*(now-start)/3000));last=now;}
                    if(now-start<3000)frame=requestAnimationFrame(tick);
                    else {renderer.render();scheduleBreeze();}
                };
                frame=requestAnimationFrame(tick);
            },22000);
        }
        function openPanel(open=true) {
            panel.hidden=!open; shell.setAttribute('aria-expanded',String(open));
            if(open){drawPanel();positionPanel();panel.querySelector('button')?.focus();}
            else {setScissors(false);shell.focus();}
        }
        function button(action,label,disabled=false,extra='') {return '<button type="button" data-action="'+action+'" '+(disabled?'disabled ':'')+extra+'>'+esc(tr(label))+'</button>';}
        function drawPanel() {
            if(dead)return;
            const focused=panel.contains(document.activeElement)?document.activeElement:null;
            const focusAction=focused?.dataset.action,focusBranch=focused?.hasAttribute('data-branch');
            const p=plant(),blocked=busy||readonly()||!!pendingAction;
            const careBlocked=blocked||!p||p.dead||!!p.vacation_at;
            let stateKey=!p?'unplanted':p.dead?'dead':p.vacation_at?'vacation':p.moisture<20||p.vitality<35?'wilting':p.age_hours<168?'growing':'thriving';
            shell.dataset.needsCare=String(!!p&&!p.dead&&!p.vacation_at&&(p.moisture<30||p.nutrients<15));
            panel.innerHTML='<header><img src="'+window.AuraLazyAssets.versionedURL('/img/leafy/icon.svg')+'" alt=""><div><strong>Leafy</strong><span>'+esc(tr(stateKey))+'</span></div>'+button('close','close',false,'aria-label="'+esc(tr('close'))+'"')+'</header>'+
                (p?'<div class="vd-leafy-meters">'+[['moisture','water_level'],['nutrients','nutrients'],['vitality','vitality']].map(([key,label])=>'<label><span>'+esc(tr(label))+'</span><meter min="0" max="100" low="20" high="70" optimum="100" value="'+p[key]+'" data-meter="'+key+'">'+Math.round(p[key])+'%</meter><output>'+Math.round(p[key])+'%</output></label>').join('')+'</div><p class="vd-leafy-age">'+esc(tr('age').replace('{hours}',Math.floor(p.age_hours)))+'</p>':'<p>'+esc(tr('intro'))+'</p>')+
                '<div class="vd-leafy-tools">'+(p&&!p.dead?button('water','water',careBlocked)+button('fertilize','fertilize',careBlocked)+button('scissors','scissors',careBlocked,'aria-pressed="'+scissors+'"')+button('vacation',p.vacation_at?'resume':'pause',blocked):button('replant',p?'replant':'plant',blocked))+'</div>'+
                (scissors&&p?'<div class="vd-leafy-pruning"><p>'+esc(tr('cut_hint'))+'</p><label>'+esc(tr('branch'))+'<select data-branch aria-label="'+esc(tr('branch'))+'"><option value="">'+esc(tr('choose'))+'</option>'+p.branches.filter(b=>b.nodes.length>1).map(b=>'<option value="'+b.id+'" '+(b.id===selection?.branch?'selected':'')+'>'+esc(tr('branch'))+' '+(b.id+1)+'</option>').join('')+'</select></label>'+button('cut','cut',!selection||careBlocked)+button('trim','trim',careBlocked)+'</div>':'')+
                (p?.undo?button('undo_prune','undo',blocked):'')+
                '<footer>'+button('hide',hidden?'show':'hide')+(p&&!p.dead?button('replant','replant',blocked):'')+'</footer>'+
                '<p class="vd-leafy-status" role="status" aria-live="polite">'+esc(message||tr(p?.vacation_at?'vacation_hint':'time_hint'))+'</p>'+
                (pendingAction?button('retry','retry',busy):'')+
                (fallback?'<small>'+esc(tr('fallback'))+'</small>':'');
            positionPanel();
            if(focusAction)panel.querySelector('[data-action="'+focusAction+'"]')?.focus({preventScroll:true});
            else if(focusBranch)panel.querySelector('[data-branch]')?.focus({preventScroll:true});
        }
        function positionPanel() {
            const w=Math.min(310,innerWidth-24),height=panel.offsetHeight||330;
            panel.style.width=w+'px';
            panel.style.left=clamp(bounds.left+bounds.width*anchor.x+90,12,innerWidth-w-12)+'px';
            panel.style.top=clamp(bounds.top+bounds.height*anchor.y-height,Math.max(12,bounds.top+8),Math.max(12,innerHeight-height-75))+'px';
        }
        function setScissors(value) {
            scissors=!!value&&!hidden; selection=null;
            root.classList.toggle('vd-leafy-cutting',scissors);
            renderer?.select(null); drawPanel();
        }
        function size() {
            const workspace=document.getElementById('vd-workspace')?.getBoundingClientRect();
            const top=Math.max(0,workspace?.top||0),bottom=Math.min(innerHeight-56,workspace?.bottom||innerHeight-56);
            bounds={left:0,top,width:innerWidth,height:Math.max(220,bottom-top)};
            root.style.top=top+'px'; root.style.height=bounds.height+'px';
            anchor.x=clamp(anchor.x,Math.min(.45,90/bounds.width),Math.max(.55,1-90/bounds.width)); anchor.y=clamp(anchor.y,Math.min(.6,190/bounds.height),Math.min(.97,1-48/bounds.height));
            const scale=Math.min(1.1,Math.max(.62,bounds.height/950));
            Object.assign(pot.style,{left:(bounds.width*anchor.x-85*scale)+'px',top:(bounds.height*anchor.y-172*scale)+'px',width:170*scale+'px',height:170*scale+'px'});
            Object.assign(move.style,{left:(bounds.width*anchor.x-17)+'px',top:(bounds.height*anchor.y+1)+'px'});
            positionPanel();
        }
        function draw() {
            if(dead)return;
            size();
            root.hidden=hidden||document.body.dataset.widgets==='false';
            shell.hidden=document.body.dataset.widgets==='false';
            if(shell.hidden)panel.hidden=true;
            if(!visible()||!renderer){stopMotion();return;}
            if(drag)return;
            const p=plant()||{seed:731,age_hours:0,moisture:0,nutrients:0,vitality:100,branches:[]};
            const light=document.body.dataset.theme==='fruity'&&document.body.dataset.fruityMode!=='dark';
            const key=JSON.stringify([p,bounds.width,bounds.height,anchor,light,devicePixelRatio]);
            if(key!==renderKey){renderer.update(p,bounds.width,bounds.height,anchor,light);renderKey=key;if(selection)renderer.select(selection);}
            scheduleBreeze();
        }
        async function createRenderer(forceFallback=false) {
            if(dead)return;
            if(renderer){const old=renderer;renderer=null;old.dispose();}
            canvas.remove();canvas=document.createElement('canvas');canvas.className='vd-leafy-canvas';canvas.setAttribute('aria-hidden','true');root.prepend(canvas);
            fallback=forceFallback;
            if(!fallback){
                try {
                    if(!window.THREE)await window.AuraLazyAssets.loadScript('/js/vendor/three.min.js');
                    await window.AuraLazyAssets.loadScript('/js/desktop/leafy/renderer.js');
                    await window.AuraLeafyRenderer.prepare();
                    if(dead)return;
                    renderer=window.AuraLeafyRenderer.create(canvas);
                } catch (_) {fallback=true;canvas.remove();canvas=document.createElement('canvas');canvas.className='vd-leafy-canvas';canvas.setAttribute('aria-hidden','true');root.prepend(canvas);}
            }
            if(dead)return;
            if(fallback)renderer=window.AuraLeafyFallback.create(canvas);
            listen(canvas,'webglcontextlost',event=>{event.preventDefault();if(!dead&&!fallback)createRenderer(true);},{once:true});
            listen(canvas,'pointermove',event=>{
                if(!scissors||!renderer?.layout)return;
                const next=window.AuraLeafyGeometry.pick(renderer.layout,event.clientX-bounds.left,bounds.height-(event.clientY-bounds.top));
                if(next?.branch===selection?.branch&&next?.node===selection?.node)return;
                selection=next;renderer.select(selection);
            });
            listen(canvas,'pointerdown',event=>{
                if(!scissors||!renderer?.layout)return;
                event.preventDefault();
                selection=window.AuraLeafyGeometry.pick(renderer.layout,event.clientX-bounds.left,bounds.height-(event.clientY-bounds.top));
                renderer.select(selection);drawPanel();
            });
            renderKey='';draw();drawPanel();
        }
        function accept(data) {
            if(dead||!data)return;
            if(snapshot?.plant&&data.plant&&data.plant.revision<snapshot.plant.revision)return;
            snapshot=data;draw();drawPanel();clearTimeout(poll);clearTimeout(undoTimer);
            const delay=clamp(Date.parse(data.next_update_at||data.next_update)-Date.parse(data.server_time),1000,3600000);
            if(!document.hidden)poll=setTimeout(refresh,Number.isFinite(delay)?delay:60000);
            if(plant()?.undo)undoTimer=setTimeout(refresh,Math.max(100,Date.parse(plant().undo.until)-Date.parse(data.server_time)+100));
        }
        async function refresh() {
            if(dead||document.hidden)return;
            if(fetching||busy){pendingRefresh=true;return;}
            fetching=true;
            try {accept(await api('/api/desktop/plant',{signal}));}
            catch(err){if(!dead&&err.name!=='AbortError'){message=tr('error');drawPanel();clearTimeout(poll);poll=setTimeout(refresh,30000);}}
            finally {fetching=false;if(pendingRefresh&&!dead){pendingRefresh=false;queueMicrotask(refresh);}}
        }
        async function action(type,fields={}) {
            if(dead||busy||readonly())return;
            if(type!=='retry'){
                if(pendingAction)return;
                pendingAction={action:type,action_id:(crypto.randomUUID?.()||Date.now().toString(36)+'_'+Math.random().toString(36).slice(2)),revision:plant()?.revision||0,...fields};
            }
            if(!pendingAction)return;
            busy=true;message='';drawPanel();
            try{
                const data=await api('/api/desktop/plant/actions',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(pendingAction),signal});
                if(dead)return;
                const completed=pendingAction.action;pendingAction=null;
                if(completed==='prune'||completed==='trim'){selection=null;renderer?.select(null);}
                if((completed==='water'||completed==='fertilize')&&animated()){
                    const effect=document.createElement('div');effect.className='vd-leafy-care-fx '+completed;
                    effect.style.left=(bounds.width*anchor.x)+'px';effect.style.top=(bounds.height*anchor.y-160*Math.min(1.1,Math.max(.62,bounds.height/950)))+'px';
                    effect.innerHTML='<i></i><i></i><i></i>';root.appendChild(effect);
                    effect.addEventListener('animationend',()=>effect.remove(),{once:true,signal});
                }
                message=tr(completed==='water'?'watered':completed==='fertilize'?'fed':'saved');
                accept(data);
            }catch(err){
                if(!dead&&err.name!=='AbortError'){
                    if(err.body?.snapshot){pendingAction=null;message=tr('conflict');accept(err.body.snapshot);}
                    else if(err.body?.error==='desktop_read_only'){pendingAction=null;message=tr('invalid');}
                    else if(err.body?.error==='invalid_plant_action'){pendingAction=null;message=tr('invalid');pendingRefresh=true;}
                    else message=tr('error');
                }
            }finally{busy=false;drawPanel();if(pendingRefresh&&!dead){pendingRefresh=false;refresh();}}
        }
        listen(shell,'click',()=>openPanel(panel.hidden)); listen(pot,'click',()=>openPanel(panel.hidden));
        listen(panel,'click',async event=>{
            const a=event.target.closest('button[data-action]')?.dataset.action;if(!a)return;
            if(a==='close')return openPanel(false);
            if(a==='hide'){hidden=!hidden;setScissors(false);draw();drawPanel();return;}
            if(a==='scissors'){setScissors(!scissors);return;}
            if(a==='replant'&&plant()&&!await confirm(tr('replant'),tr('replant_confirm')))return;
            if(a==='trim'&&!await confirm(tr('trim'),tr('trim_confirm')))return;
            if(dead)return;
            if(a==='vacation')return action(a,{paused:!plant()?.vacation_at});
            if(a==='cut'){if(selection)return action('prune',selection);return;}
            action(a);
        });
        listen(panel,'change',event=>{
            if(!event.target.matches('[data-branch]'))return;
            const b=plant()?.branches.find(b=>String(b.id)===event.target.value);
            selection=b?{branch:b.id,node:Math.max(1,Math.floor(b.nodes.length*.65))}:null;
            renderer?.select(selection);drawPanel();panel.querySelector('[data-branch]')?.focus();
        });
        listen(document,'keydown',event=>{
            if(event.key==='Escape'&&(!panel.hidden||scissors)){setScissors(false);openPanel(false);}
        });
        listen(move,'pointerdown',event=>{
            if(event.button!==0)return;event.preventDefault();if(scissors)setScissors(false);move.setPointerCapture(event.pointerId);
            drag={id:event.pointerId,x:event.clientX,y:event.clientY,anchor:{...anchor}};stopMotion();
        });
        listen(move,'pointermove',event=>{
            if(!drag||event.pointerId!==drag.id)return;
            anchor={x:drag.anchor.x+(event.clientX-drag.x)/bounds.width,y:drag.anchor.y+(event.clientY-drag.y)/bounds.height};
            size();
            canvas.style.transform='translate('+((anchor.x-drag.anchor.x)*bounds.width)+'px,'+((anchor.y-drag.anchor.y)*bounds.height)+'px)';
        });
        const endDrag=()=>{if(drag){if(move.hasPointerCapture(drag.id))move.releasePointerCapture(drag.id);drag=null;canvas.style.transform='';draw();try{localStorage.setItem('aurago.leafy.anchor.v1',JSON.stringify(anchor));}catch(_){}scheduleBreeze();}};
        listen(move,'pointerup',endDrag);listen(move,'pointercancel',endDrag);
        listen(move,'keydown',event=>{
            const delta={ArrowLeft:[-.02,0],ArrowRight:[.02,0],ArrowUp:[0,-.02],ArrowDown:[0,.02]}[event.key];
            if(delta){event.preventDefault();event.stopPropagation();anchor.x+=delta[0];anchor.y+=delta[1];draw();try{localStorage.setItem('aurago.leafy.anchor.v1',JSON.stringify(anchor));}catch(_){}}
        });
        listen(window,'resize',()=>{clearTimeout(resize);resize=setTimeout(draw,120);});
        listen(document,'visibilitychange',()=>{clearTimeout(poll);stopMotion();if(!document.hidden){draw();refresh();}});
        listen(document,'aurago:plant-change',refresh);listen(window,'online',refresh);
        listen(motion,'change',()=>{renderKey='';draw();});
        const observer=new MutationObserver(()=>{draw();drawPanel();});
        observer.observe(document.body,{attributes:true,attributeFilter:['data-theme','data-fruity-mode','data-animations','data-widgets','data-density']});
        current={
            dispose(){
                if(dead)return;dead=true;endDrag();lifecycle.abort();observer.disconnect();stopMotion();
                clearTimeout(poll);clearTimeout(resize);clearTimeout(undoTimer);renderer?.dispose();renderer=null;
                root.remove();panel.remove();shell.remove();if(current===this)current=null;
            },
            // Read-only diagnostics also make lifecycle and GPU-budget checks reproducible.
            metrics:()=>({renderer:fallback?'canvas2d':'webgl',...renderer?.metrics(),revision:plant()?.revision||0,age:plant()?.age_hours||0,active:visible(),scheduledFrames:!!frame}),
            get layout(){return renderer?.layout;}
        };
        drawPanel();size();createRenderer();refresh();
        return current;
    }
    window.AuraLeafy={mount,get active(){return current;}};
})();
