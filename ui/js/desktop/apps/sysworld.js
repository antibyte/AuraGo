(function () {
    'use strict';
    const instances = new Map();
    const QUALITY_STORAGE_KEY = 'aurago.desktop.sysworld.quality';
    const MOTION_STORAGE_KEY = 'aurago.desktop.sysworld.motion';
    const NS = () => window.SysWorld;
    const versioned = path => window.AuraLazyAssets?.versionedURL(path) ||
        path + '?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev');
    function readQuality() {
        try { const value=localStorage.getItem(QUALITY_STORAGE_KEY);if(['auto','low','medium','high','ultra'].includes(value))return value; } catch (_) {}
        return 'auto';
    }
    function render(container, windowId, context = {}) {
        dispose(windowId);
        const ctx = { ...context };
        ctx.esc ||= value => String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
        ctx.t ||= key=>key;
        const root = document.createElement('div');root.className='sysworld sw-inspecting';root.tabIndex=-1;
        const canvasHost = document.createElement('div');canvasHost.className='sysworld-canvas';root.append(canvasHost);
        container.replaceChildren(root);
        const motion=matchMedia('(prefers-reduced-motion: reduce)');
        let motionPreference='auto';
        try{const saved=localStorage.getItem(MOTION_STORAGE_KEY);if(['auto','on','off'].includes(saved))motionPreference=saved;}catch(_){}
        const motionOff=()=>inst.motion==='off'||(inst.motion==='auto'&&(motion.matches||document.body.dataset.animations==='false'));
        const inst={root,canvasHost,ctx,windowId,L:key=>ctx.t(key),motion:motionPreference,reducedMotion:motionOff,quality:readQuality(),entities:[],
            disposed:false,selected:'agent',mode:'orbit',photo:false,city:null,raf:0,elapsed:0,visible:true,inView:true,cleanup:[],load:new AbortController()};
        instances.set(windowId,inst);
        const win=root.closest('.vd-window');
        const isVisible=()=>!document.hidden&&inst.inView&&root.isConnected&&(!win||(!win.classList.contains('vd-space-hidden')&&win.style.display!=='none'));
        let last=0;
        const tick=time=>{
            inst.raf=0;if(inst.disposed||!inst.visible||inst.mode==='map'||!inst.city)return;
            const dt=last?Math.min(.2,(time-last)/1000):1/60;last=time;inst.elapsed+=dt;
            inst.city.update(dt,inst.elapsed);inst.hud.project(inst.city);
            inst.raf=requestAnimationFrame(tick);
        };
        const syncVisibility=()=>{
            inst.visible=isVisible();inst.city?.setVisible(inst.visible&&inst.mode!=='map');
            inst.sound?.setActive(!inst.replaying&&inst.visible&&inst.mode!=='map'&&(!win||win.classList.contains('active')));
            if(!inst.visible||inst.mode==='map'){cancelAnimationFrame(inst.raf);inst.raf=0;last=0;}
            else if(inst.city&&!inst.raf)inst.raf=requestAnimationFrame(tick);
        };
        inst.setMode=mode=>{
            if(inst.disposed)return;
            if(!inst.city&&mode!=='map'){inst.hud.error('sysworld.city.map_fallback');return;}
            if(motionOff()&&mode==='tour')mode='orbit';
            if(mode==='map')inst.setPhoto(false);
            inst.mode=mode;inst.city?.setMode(mode);inst.hud.mode(mode);syncVisibility();
            if(mode==='street')inst.city?.canvas.focus({preventScroll:true});
        };
        inst.select=(id,move=true)=>{
            if(inst.disposed)return;const e=inst.entities.find(e=>e.id===id);if(!e)return;
            inst.selected=id;inst.hud.select(id);if(move)inst.city?.focus(e.district);
            if(e.kind==='kgnode'&&id!=='graph')loadNeighborhood(e);
        };
        let detailRequest=null;
        async function loadNeighborhood(entity) {
            detailRequest?.abort();const controller=new AbortController();detailRequest=controller;
            const timeout=setTimeout(()=>controller.abort(),10000);
            try{
                const response=await fetch('/api/knowledge-graph/node?id='+encodeURIComponent(entity.payload.id),{credentials:'same-origin',signal:controller.signal});
                if(!response.ok)return;const data=await response.json();
                if(inst.disposed||inst.selected!==entity.id||detailRequest!==controller)return;
                if(Array.isArray(data.edges))entity.payload.related=data.edges.slice(0,100);
                inst.hud.select(entity.id);
            }catch(_){}finally{clearTimeout(timeout);}
        }
        inst.setQuality=quality=>{
            inst.quality=quality;try{localStorage.setItem(QUALITY_STORAGE_KEY,quality);}catch(_){}
            void inst.city?.setQuality(quality);
        };
        // Photo mode hides the interface; the saved PNG is the rendered frame only.
        inst.setPhoto=on=>{
            on=!!on&&!!inst.city&&inst.mode!=='map'&&!inst.disposed;if(on===inst.photo)return;
            inst.photo=on;inst.hud.photo(on);
            if(on)root.querySelector('[data-sw-action="photo-save"]')?.focus({preventScroll:true});else inst.city?.canvas.focus({preventScroll:true});
        };
        inst.savePhoto=async()=>{
            const blob=await inst.city?.capture();if(!blob||inst.disposed)return;
            const d=new Date(),p=n=>String(n).padStart(2,'0'),url=URL.createObjectURL(blob),link=document.createElement('a');
            link.href=url;link.download='aurago-system-world-'+d.getFullYear()+p(d.getMonth()+1)+p(d.getDate())+'-'+p(d.getHours())+p(d.getMinutes())+p(d.getSeconds())+'.png';
            link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
        };
        // H toggles photo mode outside text fields; Escape leaves it before the city sees the key.
        const photoKeys=e=>{
            if(e.ctrlKey||e.metaKey||e.altKey)return;
            if(e.key==='Escape'&&inst.photo){inst.setPhoto(false);e.preventDefault();e.stopPropagation();return;}
            if((e.key==='h'||e.key==='H')&&!e.repeat&&inst.city&&inst.mode!=='map'&&!e.target.closest?.('input,select,textarea,[contenteditable]')){inst.setPhoto(!inst.photo);e.preventDefault();e.stopPropagation();}
        };
        root.addEventListener('keydown',photoKeys,true);
        inst.cleanup.push(()=>root.removeEventListener('keydown',photoKeys,true));
        inst.toggleSound=()=>inst.sound?.toggle();
        inst.setVolume=value=>inst.sound?.setVolume(value);
        inst.setMotion=value=>{
            if(!['auto','on','off'].includes(value)||inst.disposed)return;
            inst.motion=value;try{localStorage.setItem(MOTION_STORAGE_KEY,value);}catch(_){}
            reduced();
        };
        const unlockSound=()=>inst.sound?.unlock();
        root.addEventListener('pointerdown',unlockSound);
        inst.cleanup.push(()=>root.removeEventListener('pointerdown',unlockSound));
        inst.hud=NS().createHud(inst);inst.hud.mode('orbit');
        inst.worldControls=NS().createWorldControls(inst);
        inst.worldControls.motion(inst.motion,motionOff());
        inst.applyWorldSnapshot=snapshot=>{
            syncVisibility();
            if(snapshot){inst.artifacts?.request?.abort();inst.city?.setHologram([],'local');}
            if(!snapshot){inst.entities=inst.liveEntities||[];inst.hud.update(inst.snapshot,inst.entities);inst.city?.setData(inst.entities,inst.snapshot.events);inst.city?.setWorld(inst.snapshot.sources.world?.data,false);return;}
            const metricRows=NS().worldRows||(()=>[]);
            const rows=snapshot.entities.map(e=>({...e,label:e.label||inst.L(NS().districts.find(d=>d[0]===e.id)?.[1]||'sysworld.city.unavailable'),at:e.at,source:e.source||'system-world/history',stale:snapshot.at-e.at>90000,rows:metricRows(e,snapshot.metrics),payload:{}}));
            inst.entities=rows;
            const metrics=snapshot.metrics;
            const data={sources:{system:{data:{cpu:{usage_percent:metrics.cpu},memory:{used_percent:metrics.ram}},at:snapshot.at}},events:[]};
            inst.hud.update(data,rows);inst.city?.setData(rows,[]);inst.city?.setWorld(snapshot,true);
        };
        inst.cleanup.push(NS().data.subscribe(snapshot=>{
            if(inst.disposed)return;
            const rows=NS().data.entities(snapshot,inst.L);inst.liveEntities=rows;if(!inst.replaying)inst.entities=rows;inst.snapshot=snapshot;
            if(!inst.replaying){inst.hud.update(snapshot,rows);inst.city?.setData(rows,snapshot.events);inst.city?.setWorld(snapshot.sources.world?.data,false);}inst.refreshArtifactFallback?.();
            inst.worldControls?.update(snapshot.sources.world?.data);
            if(!inst.selected){inst.selected='agent';inst.hud.select('agent');}
        }));
        const visibility=()=>syncVisibility();
        document.addEventListener('visibilitychange',visibility);
        const reduced=()=>{const off=motionOff();inst.city?.setReducedMotion(off);inst.worldControls.motion(inst.motion,off);if(off&&inst.mode==='tour')inst.setMode('orbit');};
        const motionObserver=new MutationObserver(reduced);motionObserver.observe(document.body,{attributes:true,attributeFilter:['data-animations']});
        inst.cleanup.push(()=>motionObserver.disconnect());
        motion.addEventListener('change',reduced);
        const io=new IntersectionObserver(entries=>{inst.inView=!!entries[0]?.isIntersecting;syncVisibility();},{threshold:.01});io.observe(root);
        const observer=new MutationObserver(syncVisibility);if(win)observer.observe(win,{attributes:true,attributeFilter:['class','style','data-space-hidden']});
        inst.cleanup.push(()=>{document.removeEventListener('visibilitychange',visibility);motion.removeEventListener('change',reduced);io.disconnect();observer.disconnect();detailRequest?.abort();});
        ctx.setWindowMenus?.(windowId,[{id:'view',label:ctx.t('desktop.menu_view'),items:[
            {label:inst.L('sysworld.btn.overview'),icon:'globe',action:()=>inst.setMode('orbit')},
            {label:inst.L('sysworld.city.street'),icon:'user',action:()=>inst.setMode('street')},
            {label:inst.L('sysworld.city.map'),icon:'map',action:()=>inst.setMode('map')},
            {label:inst.L('sysworld.city.tour'),icon:'play',action:()=>inst.setMode('tour')},
            {type:'separator'},{label:ctx.t('desktop.context_refresh'),icon:'refresh',action:()=>NS().data.refresh()},
        ]}]);
        inst.cleanup.push(()=>ctx.clearWindowMenus?.(windowId));
        // Memory hologram feed: bounded excerpts from the admin-only artifact endpoint. Until the
        // first live response, and whenever the feed fails, the archive's own counters are shown.
        const artifacts=inst.artifacts={source:'none',count:0,failures:0,timer:0,request:null};
        const fallbackArtifacts=()=>(inst.entities.find(e=>e.id==='memory')?.rows||[])
            .filter(r=>typeof r.value==='number'&&Number.isFinite(r.value))
            .map(r=>inst.L(r.key)+' '+Number(r.value).toLocaleString());
        const applyFallback=inst.refreshArtifactFallback=()=>{if(!inst.replaying&&artifacts.source!=='live')inst.city?.setHologram(fallbackArtifacts(),'local');};
        const scheduleArtifacts=delay=>{if(inst.disposed)return;clearTimeout(artifacts.timer);artifacts.timer=setTimeout(pollArtifacts,delay);};
        async function pollArtifacts(){
            artifacts.timer=0;if(inst.disposed)return;
            if(!inst.city||!inst.visible||inst.mode==='map'||inst.replaying){scheduleArtifacts(5000);return;}
            const controller=new AbortController();artifacts.request=controller;
            const timeout=setTimeout(()=>controller.abort(),10000);
            let delay=24000;
            try{
                const response=await fetch('/api/desktop/system-world/memory-artifacts',{credentials:'same-origin',cache:'no-store',signal:controller.signal});
                if(response.status!==429){
                    if(!response.ok)throw Error('artifacts unavailable');
                    const data=await response.json();if(inst.disposed||inst.replaying)return;
                    const list=Array.isArray(data?.artifacts)?data.artifacts.filter(a=>typeof a==='string'):[];
                    artifacts.failures=0;
                    if(list.length){artifacts.source='live';artifacts.count=list.length;inst.city?.setHologram(list,'live');}
                }
            }catch(_){
                if(inst.disposed)return;
                artifacts.failures++;delay=90000;
            }finally{clearTimeout(timeout);if(artifacts.request===controller)artifacts.request=null;}
            applyFallback();scheduleArtifacts(delay);
        }
        inst.cleanup.push(()=>{clearTimeout(artifacts.timer);artifacts.request?.abort();});
        async function loadCity(){
            try{
                const module=await import(versioned('/js/vendor/system-world/city.esm.js'));
                if(inst.disposed)return;
                inst.sound=module.createCityAmbience(value=>inst.hud.sound(value));
                inst.hud.sound(inst.sound.enabled());
                const city=await module.createCity(canvasHost,{
                    label:ctx.t('desktop.app_system_world'),memoryLabel:ctx.t('sysworld.zone.memory'),quality:inst.quality,reducedMotion:motionOff(),signal:inst.load.signal,
                    assetURL:file=>versioned('/3d/system-world/v1/'+file),resourceURL:versioned,
                    onListener:(x,y,z,fx,fz)=>{inst.sound?.setListener(x,y,z,fx,fz);inst.hud.orient(x,z,fx,fz);},
                    onProgress:(loaded,expected)=>inst.hud.progress(loaded,expected),onHover:id=>inst.hud.hover(id),onCut:()=>inst.hud.cut(),
                    onInteraction:(...args)=>inst.worldControls.interaction(...args),
                    onSociety:value=>inst.worldControls.society(value),replaying:()=>!!inst.replaying,
                    onDiscover:id=>inst.worldControls.discover(id),onTerminal:id=>inst.worldControls.terminal(id),
                    onSound:(kind,x,y,z)=>inst.sound?.effect(kind,x,y,z),onEnvironment:value=>inst.worldControls.environment(value),
                    onThunder:delay=>inst.sound?.thunder(delay),onMood:mood=>inst.sound?.setMood(mood),
                    onRobotError:()=>{if(!inst.disposed){inst.robotError=true;inst.hud.error('sysworld.city.robot_error');}},
                    onTourFocus:id=>{if(!inst.disposed){inst.selected=id;inst.hud.select(id);inst.hud.tour(id);}},
                    onSelect:id=>inst.select(id),busy:()=>inst.replaying?inst.entities.find(e=>e.id==='agent')?.state==='running':!!inst.snapshot?.sources.overview?.data?.agent?.busy,
                    onReady:()=>{if(!inst.disposed){inst.assetError=false;inst.hud.ready(!inst.robotError);}},
                    onError:()=>{if(!inst.disposed){inst.assetError=true;inst.hud.error('sysworld.city.asset_error');}},
                    onQuality:(value,actual)=>{if(!inst.disposed)inst.hud.quality(value,actual);},
                    onMode:value=>{if(!inst.disposed){inst.mode=value;inst.hud.mode(value);syncVisibility();}},
                    onContextLost:()=>{if(!inst.disposed){inst.setMode('map');inst.hud.error('sysworld.city.map_fallback');}},
                });
                if(inst.disposed){city.dispose();return;}
                inst.city=city;reduced();city.setData(inst.entities,inst.snapshot?.events);city.setWorld(inst.snapshot?.sources.world?.data,!!inst.replaying);city.setMode(inst.mode);inst.worldControls.ready();syncVisibility();
                // The first frames fade in while the camera glides into the overview (skipped with reduced motion).
                root.classList.add('sw-ready');if(!inst.assetError)city.intro();
                applyFallback();scheduleArtifacts(1500);
            }catch(_){
                if(inst.disposed)return;inst.mode='map';inst.hud.mode('map');inst.hud.error('sysworld.city.map_fallback');syncVisibility();
            }
        }
        void loadCity();
    }
    function dispose(windowId) {
        const inst=instances.get(windowId);if(!inst)return;
        inst.disposed=true;inst.load.abort();cancelAnimationFrame(inst.raf);
        inst.cleanup.forEach(fn=>fn());inst.worldControls?.dispose();inst.sound?.dispose();inst.city?.dispose();inst.hud.dispose();inst.root.remove();instances.delete(windowId);
    }
    // Bounded read-only diagnostics used by the rendering/lifecycle acceptance check.
    function inspect(windowId) {
        const inst=instances.get(windowId);
        const artifacts=inst?.artifacts?{source:inst.artifacts.source,count:inst.artifacts.count,failures:inst.artifacts.failures,polling:!!inst.artifacts.timer}:null;
        return inst?{sound:inst.sound?.stats(),motion:{preference:inst.motion,reduced:inst.reducedMotion()},selected:inst.selected,visible:inst.visible,mode:inst.mode,photo:inst.photo,entityCount:inst.entities.length,raf:!!inst.raf,artifacts,...inst.city?.stats()}:null;
    }
    window.SysWorldApp = { render, dispose, inspect };
})();
