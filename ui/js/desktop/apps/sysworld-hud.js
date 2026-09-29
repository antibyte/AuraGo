(function () {
    'use strict';
    const NS = window.SysWorld = window.SysWorld || {};
    // [id, label key, icon, x, z, radius] in city metres; mirrors the scene's districts.
    NS.districts = [
        ['agent','sysworld.zone.core','chat',0,-12,13], ['infra','sysworld.zone.infra','cpu',-43,-53,18],
        ['integrations','sysworld.zone.integrations','network',43,-10,14], ['missions','sysworld.zone.missions','calendar',43,36,15],
        ['memory','sysworld.zone.memory','folder',-43,-10,14], ['graph','sysworld.zone.graph','globe',-43,36,17],
        ['operations','sysworld.city.operations','settings',0,36,6],
    ];
    // Plan geometry mirrored from sysworld-layout.js; scripts/test-system-world-layout.mjs keeps both in step.
    NS.map = {
        island: [-85,-94,85,80], xs: [-67,-18,18,67], zs: [-77,-32,13,59], halfWidth: 6,
        tram: [[-67,-77],[-67,59],[67,59],[67,-77]], stops: [[-74,-53],[-74,-10],[-43,64.5],[0,64.5],[43,64.5],[74,-10],[0,-84]],
        pavilions: [[0,73],[-43,73],[43,73]], pavilion: 12, pad: [78,30],
    };
    const SVG = 'http://www.w3.org/2000/svg', SPARK = 60, PIN_FAR = 260, VIEW = [-100,-107,200,200];
    const svgEl = (tag, attrs) => { const n = document.createElementNS(SVG, tag); for (const [k, v] of Object.entries(attrs || {})) n.setAttribute(k, v); return n; };
    // Labels never cover these panels; their rectangles are cached by a ResizeObserver, never read per frame.
    const PANELS = '.sw-top,.sw-rail,.sw-info,.sw-bottom,.sw-world,.sw-tour,.sw-message';
    const TOP = { l: -1e9, r: 1e9, t: -1e9, b: 0 };
    const wrap = angle => (angle + 540) % 360 - 180;
    const overlaps = (a, b) => a.l < b.r && b.l < a.r && a.t < b.b && b.t < a.b;
    NS.createHud = function (inst) {
        const L = inst.L, root = inst.root, esc = inst.ctx.esc;
        const icon = name => inst.ctx.iconMarkup?.(name, '', 'sw-icon', 20, 'action') || '';
        const el = (tag, cls, text) => { const n = document.createElement(tag); n.className = cls || ''; if(text != null)n.textContent = text; return n; };
        const btn = (action, label, symbol) => {
            const node = el('button', 'sw-btn'); node.type = 'button'; node.dataset.swAction = action;
            node.title = label; node.setAttribute('aria-label', label);
            node.innerHTML = (symbol ? icon(symbol) : '') + '<span class="sw-btn-label">' + esc(label) + '</span>'; return node;
        };
        const shell = el('div','sysworld-hud'); root.append(shell);
        const fade = el('div','sw-fade'); shell.append(fade);
        // Telescope view: lens, reticle and an AR tag for the district the reticle rests on.
        const scopeView=el('div','sw-scope'),scopeHead=el('div','sw-scope-head'),scopeZoom=el('span'),scopeBearing=el('span');
        const scopeTag=el('div','sw-scope-tag'),scopeTitle=el('strong'),scopeState=el('span','sw-scope-state'),scopeMetric=el('span','sw-muted'),scopeRange=el('span','sw-muted');
        scopeTitle.setAttribute('aria-live','polite');scopeHead.append(el('strong','',L('sysworld.city.scope')),scopeZoom,scopeBearing);
        scopeTag.append(scopeTitle,scopeState,scopeMetric,scopeRange);scopeView.hidden=true;
        scopeView.append(el('div','sw-scope-lens'),el('div','sw-scope-reticle'),scopeHead,scopeTag,el('div','sw-scope-hint',L('sysworld.city.scope_hint')));shell.append(scopeView);
        const header = el('header','sw-top sw-glass');
        const wordmark = el('div','sw-wordmark','AURA / '); wordmark.append(el('strong','',L('sysworld.city.metropolis')));
        header.append(wordmark);
        // Street compass: north is -z; ticks every 15°, district dots in their state colour.
        const compass = el('div','sw-compass'), dial = el('div','sw-compass-dial'), facing = el('div','sw-compass-caption'), bearings = [], compassDots = {};
        compass.hidden = true; compass.setAttribute('aria-hidden','true');
        for (const [key,angle] of [['n',0],['e',90],['s',180],['w',270]]) { const n = el('span','sw-compass-mark',L('sysworld.compass.'+key)); n.dataset.cardinal = key; dial.append(n); bearings.push({ n, angle }); }
        NS.districts.forEach(([id,key,,x,z]) => { const n = el('i','sw-compass-district'); dial.append(n); compassDots[id] = n; bearings.push({ n, id, key, x, z }); });
        compass.append(dial, facing); header.append(compass);
        const stats = el('div','sw-stats'), statEls = {}, sparks = {}, history = { cpu: [], ram: [] };
        for (const name of ['cpu','ram','missions','agent']) {
            const node = el('div','sw-stat'); node.append(el('span','sw-muted',L('sysworld.stats.'+name)));
            statEls[name] = el('strong','', '—'); statEls[name].dataset.swStat = name; node.append(statEls[name]);
            if (history[name]) { sparks[name] = sparkline(); node.append(sparks[name]); }
            stats.append(node);
        }
        header.append(stats); shell.append(header);
        const rail = el('aside','sw-rail sw-glass');
        const search = el('input','sw-search'); search.type = 'search'; search.placeholder = L('sysworld.city.search'); search.setAttribute('aria-label',search.placeholder);
        // Narrow layouts show an icon column; this toggle opens the full rail with search as a drawer.
        const drawer = btn('districts',L('sysworld.city.search'),'search'); drawer.classList.add('sw-rail-toggle'); drawer.setAttribute('aria-expanded','false');
        const openRail = open => {
            rail.classList.toggle('sw-open',open); drawer.setAttribute('aria-expanded',String(open));
            if(open)search.focus({preventScroll:true}); else if(search.value){search.value='';renderResults();}
        };
        drawer.addEventListener('click', () => openRail(!rail.classList.contains('sw-open')));
        rail.addEventListener('keydown', e => { if(e.key==='Escape'&&rail.classList.contains('sw-open')){openRail(false);drawer.focus();e.stopPropagation();} });
        rail.append(drawer, search);
        const nav = el('nav','sw-districts'); nav.setAttribute('aria-label',L('sysworld.legend'));
        const counts = {}, districtButtons = {};
        NS.districts.forEach(([id,key,symbol]) => {
            const node = btn(id,L(key),symbol); node.dataset.swDistrict = id; delete node.dataset.swAction;
            counts[id] = el('small','','—'); node.append(counts[id]); nav.append(node); districtButtons[id] = node;
            node.addEventListener('click', () => inst.select(id));
        }); rail.append(nav);
        const list = el('div','sw-results'); list.hidden = true;
        rail.append(list);
        const caption = el('div','sw-rail-caption',L('sysworld.city.scene_hint')); rail.append(caption); shell.append(rail);
        const inspector = el('aside','sw-info sw-glass');
        const panelHead = el('header','sw-info-head');
        const eyebrow = el('div','sw-eyebrow'), title = el('h2',''), close = btn('close', L('desktop.close'),'x');
        panelHead.append(eyebrow,title,close);
        const state = el('div','sw-state'); panelHead.append(state); inspector.append(panelHead);
        const tabs = el('div','sw-tabs'); tabs.setAttribute('role','tablist'); const tabButtons = {};
        for (const id of ['overview','entities','events']) {
            const node = btn('tab-'+id,L('sysworld.city.'+id),''); node.setAttribute('role','tab'); tabs.append(node); tabButtons[id]=node;
            node.addEventListener('click', () => { activeTab = id; renderPanel(true); });
        }
        inspector.append(tabs);
        const body = el('div','sw-info-body'); body.tabIndex=0; inspector.append(body);
        const source = el('footer','sw-source'); inspector.append(source); shell.append(inspector);
        // Plan view without WebGL: water, quay, streets, tram loop, pavilions, district blocks in state colours, camera.
        const map = el('div','sw-map sw-glass'); map.hidden=true; map.setAttribute('aria-label',L('sysworld.city.map'));
        map.append(el('h2','',L('sysworld.city.map')));
        const P=NS.map,[x0,z0,x1,z1]=P.island,hw=P.halfWidth,mapBlocks={},mapButtons={},mapCounts={};
        const stage=el('div','sw-map-stage'),board=el('div','sw-map-board'),plan=svgEl('svg',{class:'sw-map-svg',viewBox:VIEW.join(' '),'aria-hidden':'true'});
        plan.append(svgEl('rect',{class:'sw-map-water',x:VIEW[0],y:VIEW[1],width:VIEW[2],height:VIEW[3]}),svgEl('rect',{class:'sw-map-island',x:x0,y:z0,width:x1-x0,height:z1-z0,rx:5}));
        for(const x of P.xs)plan.append(svgEl('rect',{class:'sw-map-street',x:x-hw,y:z0,width:hw*2,height:z1-z0}));
        for(const z of P.zs)plan.append(svgEl('rect',{class:'sw-map-street',x:x0,y:z-hw,width:x1-x0,height:hw*2}));
        for(let i=0;i<3;i++)for(let j=0;j<3;j++){
            const l=P.xs[i]+hw,r=P.xs[i+1]-hw,t=P.zs[j]+hw,b=P.zs[j+1]-hw,block=svgEl('rect',{class:'sw-map-block',x:l,y:t,width:r-l,height:b-t,rx:2});
            const d=NS.districts.find(d=>d[3]>l&&d[3]<r&&d[4]>t&&d[4]<b);if(d)mapBlocks[d[0]]=block;plan.append(block);
        }
        for(const x of P.xs)plan.append(svgEl('line',{class:'sw-map-lane',x1:x,y1:z0+4,x2:x,y2:z1-4}));
        for(const z of P.zs)plan.append(svgEl('line',{class:'sw-map-lane',x1:x0+4,y1:z,x2:x1-4,y2:z}));
        plan.append(svgEl('polygon',{class:'sw-map-tram',points:P.tram.map(p=>p.join(',')).join(' ')}));
        for(const [x,z]of P.pavilions)plan.append(svgEl('rect',{class:'sw-map-pavilion',x:x-P.pavilion/2,y:z-P.pavilion/2,width:P.pavilion,height:P.pavilion,rx:1.5}));
        plan.append(svgEl('circle',{class:'sw-map-pad',cx:P.pad[0],cy:P.pad[1],r:4.5}));
        for(const [x,z]of P.stops)plan.append(svgEl('circle',{class:'sw-map-stop',cx:x,cy:z,r:1.8}));
        const viewer=svgEl('g',{class:'sw-map-camera'}),seen=[0,0,0,-1];viewer.append(svgEl('path',{d:'M0 0L-10 -27Q0 -31 10 -27Z'}),svgEl('circle',{r:2.8}));viewer.style.display='none';plan.append(viewer);
        board.append(plan);
        NS.districts.forEach(([id,key,symbol,x,z])=>{
            const node=btn(id,L(key),symbol); node.style.left=((x-VIEW[0])/VIEW[2]*100)+'%';node.style.top=((z-VIEW[1])/VIEW[3]*100)+'%';
            mapCounts[id]=el('small','','—');node.append(mapCounts[id]);node.addEventListener('click',()=>inst.select(id));board.append(node);mapButtons[id]=node;
        });
        const legend=el('div','sw-map-legend');
        for(const [value,key]of[['running','sysworld.state.running'],['idle','sysworld.state.idle'],['error','sysworld.state.error'],['unknown','sysworld.city.unknown']]){const item=el('span','',L(key));item.dataset.state=value;legend.append(item);}
        stage.append(board);map.append(stage,legend);shell.append(map);
        // Tour lower third: station, district, state and one key figure.
        const tour=el('div','sw-tour sw-glass'),tourStep=el('div','sw-eyebrow'),tourTitle=el('strong','sw-tour-title'),tourMeta=el('div','sw-tour-meta');
        const tourState=el('span','sw-state'),tourMetric=el('span','sw-muted');tourMeta.append(tourState,tourMetric);tour.append(tourStep,tourTitle,tourMeta);tour.hidden=true;shell.append(tour);
        const controls = el('footer','sw-bottom sw-glass'), modes = el('div','sw-modes'), modeButtons={};
        for(const [id,key,symbol] of [['orbit','sysworld.btn.overview','globe'],['street','sysworld.city.street','users'],['tour','sysworld.city.tour','run'],['map','sysworld.city.map','grid']]){
            const node=btn(id,L(key),symbol);node.dataset.swMode=id;delete node.dataset.swAction;modes.append(node);modeButtons[id]=node;
            node.addEventListener('click',()=>inst.setMode(id));
        }
        const quality=el('select','sw-quality'); quality.setAttribute('aria-label',L('sysworld.btn.quality'));
        for(const id of ['auto','low','medium','high','ultra'])quality.add(new Option(L(id==='auto'?'sysworld.city.auto':'sysworld.quality.'+id),id));
        quality.value=inst.quality; quality.addEventListener('change',()=>inst.setQuality(quality.value));
        const status = el('span','sw-live');
        const refresh=btn('refresh',L('desktop.context_refresh'),'refresh');refresh.addEventListener('click',()=>NS.data.refresh());
        const sound=btn('sound',L('sysworld.city.sound_on'),'speaker');sound.disabled=true;sound.setAttribute('aria-pressed','false');
        sound.addEventListener('click',()=>inst.toggleSound());
        const volume=el('input','sw-volume');volume.type='range';volume.min=0;volume.max=35;volume.value=18;volume.hidden=true;
        volume.setAttribute('aria-label',L('desktop.radio_volume'));volume.title=L('desktop.radio_volume');
        volume.addEventListener('input',()=>inst.setVolume(Number(volume.value)/100));
        const photo=btn('photo',L('sysworld.city.photo'),'camera');photo.disabled=true;photo.setAttribute('aria-pressed','false');
        photo.addEventListener('click',()=>inst.setPhoto(true));
        controls.append(modes,status,sound,volume,quality,photo,refresh);shell.append(controls);
        const help=el('div','sw-street-help sw-glass');help.hidden=true;help.append(el('span','',L('sysworld.city.street_help')));
        const lock=btn('pointer-lock',L('sysworld.city.mouse_look'),'eye');lock.addEventListener('click',()=>inst.city?.lockPointer()?.catch(()=>{}));help.append(lock);
        const movement=el('div','sw-movement');
        for(const [key,label]of[['KeyW','↑'],['KeyA','←'],['KeyS','↓'],['KeyD','→']]){
            const button=btn(key,label,'');button.setAttribute('aria-label',L('sysworld.city.move')+' '+label);
            button.addEventListener('pointerdown',e=>{button.setPointerCapture(e.pointerId);inst.city?.moveKey(key,true);});
            ['pointerup','pointercancel','lostpointercapture'].forEach(type=>button.addEventListener(type,()=>inst.city?.moveKey(key,false)));
            movement.append(button);
        }help.append(movement);shell.append(help);
        const message=el('div','sw-message sw-glass');message.setAttribute('role','status');message.hidden=true;shell.append(message);
        // Loading card fed by real byte progress (models of the tier plus surface textures).
        const loading=el('div','sw-loading sw-glass'),track=el('b','sw-loading-track'),loadingBar=el('i','sw-loading-bar');
        loading.setAttribute('role','progressbar');loading.setAttribute('aria-label',L('sysworld.loading'));
        for(const [name,value]of[['aria-valuemin','0'],['aria-valuemax','100'],['aria-valuenow','0']])loading.setAttribute(name,value);
        track.append(loadingBar);loading.append(el('span','',L('sysworld.loading')),track);shell.append(loading);
        const photoBar=el('div','sw-photo-bar sw-glass');photoBar.hidden=true;
        const save=btn('photo-save',L('sysworld.city.photo_save'),'download');save.addEventListener('click',()=>inst.savePhoto());
        const leave=btn('photo-exit',L('sysworld.city.photo_exit'),'x');leave.addEventListener('click',()=>inst.setPhoto(false));
        photoBar.append(el('span','sw-muted',L('sysworld.city.photo_hint')),save,leave);shell.append(photoBar);
        const labels=el('div','sw-labels');root.append(labels);
        const pins={};NS.districts.forEach(([id,key])=>{
            const p=btn(id,L(key),'');p.className='sw-pin';p.dataset.district=id;p.dataset.pin='off';p.addEventListener('click',()=>inst.select(id));labels.append(p);pins[id]=p;
        });
        if(root.clientWidth<760){inspector.hidden=true;root.classList.remove('sw-inspecting');}
        let current=[], snapshot={sources:{},events:[]}, selected='agent', activeTab='overview', panelSignature='', resultSignature='';
        let focusDistrict='agent', hovered=null, photoOn=false, loadingPercent=0, sparkAt=0, observer=null, panels=[], bounds={width:0,height:0}, heading=null, viewerKnown=false;
        const pinSize=new Map(), pinAt=new Map();
        function sparkline() {
            const svg=document.createElementNS(SVG,'svg');svg.setAttribute('class','sw-spark');svg.setAttribute('viewBox','0 0 60 18');
            svg.setAttribute('preserveAspectRatio','none');svg.setAttribute('aria-hidden','true');
            svg.append(document.createElementNS(SVG,'path'),document.createElementNS(SVG,'polyline'));return svg;
        }
        function drawSpark(svg, values) {
            const step=60/(SPARK-1),start=60-(values.length-1)*step;
            const points=values.map((v,i)=>(start+i*step).toFixed(1)+','+(17-Math.max(0,Math.min(100,v))*.16).toFixed(1));
            svg.lastChild.setAttribute('points',points.join(' '));
            svg.firstChild.setAttribute('d',values.length>1?'M'+start.toFixed(1)+',18L'+points.join('L')+'L60,18Z':'');
        }
        function districtState(id, value) {
            if(pins[id].dataset.state===value)return;
            for(const node of [pins[id],districtButtons[id],compassDots[id],mapBlocks[id],mapButtons[id]])node.dataset.state=value;
        }
        // Last known camera on the plan; clamped to the plan edge while the camera hovers outside it.
        function placeViewer() {
            if(!viewerKnown)return;
            const x=Math.max(VIEW[0]+6,Math.min(VIEW[0]+VIEW[2]-6,seen[0])),z=Math.max(VIEW[1]+6,Math.min(VIEW[1]+VIEW[3]-6,seen[1]));
            viewer.setAttribute('transform','translate('+x.toFixed(1)+' '+z.toFixed(1)+') rotate('+(Math.atan2(seen[2],-seen[3])*180/Math.PI).toFixed(1)+')');viewer.style.display='';
        }
        function heat() {
            for(const [id,p]of Object.entries(pins)){p.classList.toggle('sw-hot',id===focusDistrict||id===hovered);districtButtons[id].classList.toggle('sw-hover',id===hovered);}
        }
        function measure() {
            const r=root.getBoundingClientRect();bounds={width:r.width,height:r.height};panels=[];
            for(const n of root.querySelectorAll(PANELS)){const b=n.getBoundingClientRect();if(b.width&&b.height)panels.push({l:b.left-r.left,t:b.top-r.top,r:b.right-r.left,b:b.bottom-r.top});}
        }
        function observe() {
            observer=new ResizeObserver(entries=>{
                for(const entry of entries){const size=entry.borderBoxSize?.[0];if(entry.target.classList.contains('sw-pin')&&size?.inlineSize)pinSize.set(entry.target,[size.inlineSize,size.blockSize]);}
                measure();
            });
            for(const n of [root,...root.querySelectorAll(PANELS),...Object.values(pins)])observer.observe(n);
        }
        function formatUptimePhrase(key, vars) { return inst.ctx.t(key, vars); }
        function fmtMoney(v) {
            if(typeof v!=='number')return '—';
            const amount = v.toLocaleString(undefined,{minimumFractionDigits:2,maximumFractionDigits:2});
            return formatUptimePhrase('desktop.looper_cost', { amount: amount });
        }
        function format(value, format) {
            if(value==null || value==='') return '—';
            if(format==='money')return fmtMoney(value);
            if(format==='uptime'){
                const minutes=Math.floor(value/60),hours=Math.floor(minutes/60);
                return hours>=24?formatUptimePhrase('desktop.system_info_uptime_days_hours',{days:Math.floor(hours/24),hours:hours%24}):
                    hours>0?formatUptimePhrase('desktop.system_info_uptime_hours_minutes',{hours,minutes:minutes%60}):
                    formatUptimePhrase('desktop.system_info_uptime_minutes',{minutes});
            }
            if(format==='date')return String(value).startsWith('0001-')?'—':new Date(value).toLocaleString();
            if(format==='bytes'||format==='rate'){
                const units=['bytes','kib','mib','gib','tib'];let n=value,i=0;while(n>=1024&&i<4){n/=1024;i++;}
                return n.toLocaleString(undefined,{maximumFractionDigits:1})+' '+L('desktop.'+units[i])+(format==='rate'?'/s':'');
            }
            if(format==='percent')return Number(value).toLocaleString(undefined,{maximumFractionDigits:1})+'%';
            if(format==='temperature')return Number(value).toLocaleString(undefined,{maximumFractionDigits:1})+' °C';
            if(format==='number')return Number(value).toLocaleString();
            return String(value);
        }
        function stateText(e) {
            if(e.stale)return L(e.at?'sysworld.city.stale':'sysworld.city.unavailable');
            const known=['running','queued','idle','error','done','waiting','exited','paused'];
            if(known.includes(e.state))return L('sysworld.state.'+e.state);
            if(e.state==='configured'||e.state==='unknown')return L('sysworld.city.'+e.state);
            if(e.state==='disabled')return L('sysworld.panel.disabled');
            return String(e.state);
        }
        function entityList(parent, records) {
            const max=100; // ponytail: paged display, the bounded API payload remains searchable in full.
            for(const e of records.slice(0,max)){
                const button=el('button','sw-entity');button.type='button';button.dataset.entity=e.id;
                button.append(el('span','',e.label),el('small','sw-muted',stateText(e)));
                button.addEventListener('click',()=>inst.select(e.id));parent.append(button);
            }
            if(!records.length)parent.append(el('p','sw-muted',L('sysworld.city.no_results')));
            if(records.length>max)parent.append(el('p','sw-muted',inst.ctx.t('sysworld.city.showing',{shown:max,total:records.length})));
        }
        function renderPanel(force=false) {
            const e=current.find(e=>e.id===selected);if(!e)return;
            const related=current.filter(item=>item.district===e.district && item.id!==e.district);
            const relevant=snapshot.events.filter(event=>event.district===e.district);
            const signature=JSON.stringify([e,activeTab,activeTab==='entities'?related:activeTab==='events'?relevant:null]);
            if(!force && signature===panelSignature)return; panelSignature=signature;
            eyebrow.textContent=L(NS.districts.find(d=>d[0]===e.district)?.[1]||'sysworld.sec.details');
            title.textContent=e.label;state.textContent=stateText(e);state.dataset.state=e.stale?'stale':e.state;
            Object.entries(tabButtons).forEach(([id,b])=>{b.setAttribute('aria-selected',String(activeTab===id));b.classList.toggle('active',activeTab===id);});
            const scroll=body.scrollTop;body.replaceChildren();
            if(activeTab==='entities')entityList(body,related);
            else if(activeTab==='events'){
                for(const event of relevant){const item=el('div','sw-event');item.append(el('time','sw-muted',new Date(event.at).toLocaleTimeString()),el('span','',event.label||L('sysworld.city.event_'+event.kind)));body.append(item);}
                if(!relevant.length)body.append(el('p','sw-muted',L('sysworld.events.empty')));
            }else{
                const list=el('dl','sw-detail');
                for(const r of e.rows){const row=el('div','sw-detail-row');row.append(el('dt','sw-muted',L(r.key)),el('dd','',format(r.value,r.format)));
                    if(r.format==='percent'&&typeof r.value==='number'){const bar=el('meter','sw-meter');bar.min=0;bar.max=100;bar.value=Math.max(0,Math.min(100,r.value));bar.setAttribute('aria-label',L(r.key));row.append(bar);}list.append(row);}
                body.append(list);
                if(e.payload?.related?.length){
                    body.append(el('h3','',L('sysworld.sec.relations')));
                    for(const rel of e.payload.related.slice(0,20)){
                        const other=String(rel.source)===String(e.payload.id)?rel.target:rel.source;
                        const target=current.find(n=>n.id==='node:'+other);
                        const button=el('button','sw-entity');button.type='button';button.textContent=(target?.label||String(other))+' · '+String(rel.relation||'');
                        button.addEventListener('click',()=>inst.select('node:'+other));body.append(button);
                    }
                }
                if(e.id===e.district){const b=btn('entities',L('sysworld.city.entities')+' · '+related.length,'list');b.addEventListener('click',()=>{activeTab='entities';renderPanel(true);});body.append(b);}
            }
            body.scrollTop=scroll;
            source.replaceChildren(el('span','',L('sysworld.city.source')),el('code','',e.source),
                el('time','',e.at?new Date(e.at).toLocaleTimeString():L('sysworld.city.unavailable')));
        }
        function renderResults() {
            const query=search.value.trim().toLocaleLowerCase(); list.hidden=!query; nav.hidden=!!query;caption.hidden=!!query;
            const matches=current.filter(e=>(e.label+' '+e.id).toLocaleLowerCase().includes(query));
            const sig=JSON.stringify([query,matches.map(e=>[e.id,e.label,stateText(e)])]);if(sig===resultSignature)return;resultSignature=sig;
            list.replaceChildren();if(query)entityList(list,matches);
        }
        search.addEventListener('input',renderResults);
        close.addEventListener('click',()=>{inspector.hidden=true;inst.root.classList.remove('sw-inspecting');});
        return {
            update(data, rows) {
                snapshot=data;current=rows;inst.entities=rows;
                const metric=NS.data.normalizeSystemMetrics(data.sources.system?.data), agent=data.sources.overview?.data?.agent, system=data.sources.system;
                statEls.cpu.textContent=format(metric.cpu,'percent');statEls.ram.textContent=format(metric.ram,'percent');
                statEls.cpu.classList.toggle('sw-stale',!system?.at||!!system?.failed);
                statEls.ram.classList.toggle('sw-stale',!system?.at||!!system?.failed);
                statEls.missions.textContent=format(data.sources.overview?.data?.missions?.total,'number');
                statEls.agent.textContent=typeof agent?.busy==='boolean'?L(agent.busy?'sysworld.agent.busy':'sysworld.agent.idle'):'—';
                statEls.agent.parentElement.classList.toggle('sw-busy',agent?.busy===true);
                // Live history only: replayed snapshots never enter the sparklines.
                if(!inst.replaying&&system?.at&&system.at!==sparkAt&&!system.failed){
                    sparkAt=system.at;
                    for(const key of ['cpu','ram'])if(Number.isFinite(metric[key])){history[key].push(metric[key]);if(history[key].length>SPARK)history[key].shift();drawSpark(sparks[key],history[key]);}
                }
                stats.classList.toggle('sw-replay',!!inst.replaying);
                NS.districts.forEach(([id])=>{
                    counts[id].textContent=mapCounts[id].textContent=rows.filter(e=>e.district===id&&e.id!==id).length||'·';
                    const e=rows.find(e=>e.id===id);districtState(id,!e||e.stale?'unknown':e.state);
                });
                focusDistrict=rows.find(e=>e.id===selected)?.district||focusDistrict;heat();
                const stale=rows.filter(e=>e.id===e.district&&e.stale).length;status.textContent=L(stale?'sysworld.city.partial':'sysworld.city.live');
                status.classList.toggle('sw-stale',stale>0);renderPanel();renderResults();
            },
            select(id) {
                selected=id;activeTab='overview';inspector.hidden=false;inst.root.classList.add('sw-inspecting');
                const entity=current.find(e=>e.id===id);focusDistrict=entity?.district||null;heat();
                if(rail.classList.contains('sw-open')){if(list.contains(document.activeElement)||document.activeElement===search)drawer.focus({preventScroll:true});openRail(false);}
                for(const [district,b]of Object.entries(districtButtons)){const active=entity?.district===district;b.classList.toggle('active',active);b.setAttribute('aria-current',String(active));}
                renderPanel(true);
            },
            hover(id){if(hovered===id)return;hovered=id;heat();},
            mode(value) {
                root.dataset.mode=value;
                map.hidden=value!=='map';labels.hidden=value==='map'||value==='street';help.hidden=value!=='street';
                compass.hidden=value!=='street';tour.hidden=value!=='tour';photo.hidden=value==='map';
                if(value==='street')heading=null;
                if(value==='map')placeViewer();
                Object.entries(modeButtons).forEach(([id,b])=>{b.classList.toggle('active',value===id);b.setAttribute('aria-pressed',String(value===id));});
            },
            tour(id) {
                const index=NS.districts.findIndex(d=>d[0]===id);if(index<0)return;
                const e=current.find(e=>e.id===id),row=e?.rows?.find(r=>r.value!=null&&r.value!=='');
                tourStep.textContent=L('sysworld.city.tour')+' · '+inst.ctx.t('sysworld.city.tour_step',{index:index+1,total:NS.districts.length});
                tourTitle.textContent=L(NS.districts[index][1]);
                tourState.textContent=e?stateText(e):'';tourState.dataset.state=e?(e.stale?'stale':e.state):'unknown';
                tourMetric.textContent=row?L(row.key)+' '+format(row.value,row.format):'';
                if(!inst.reducedMotion())tour.animate([{opacity:0,transform:'translateY(8px)'},{opacity:1,transform:'none'}],{duration:450,easing:'cubic-bezier(.2,.7,.2,1)'});
            },
            // Street heading; only redrawn after a turn of 1.5° or a step of 1.5 m.
            orient(x,z,fx,fz) {
                seen[0]=x;seen[1]=z;seen[2]=fx;seen[3]=fz;viewerKnown=true;
                if(compass.hidden)return;
                const h=Math.atan2(fx,-fz)*180/Math.PI;
                if(heading&&Math.abs(wrap(h-heading[0]))<1.5&&Math.hypot(x-heading[1],z-heading[2])<1.5)return;
                heading=[h,x,z];dial.style.setProperty('--h',h.toFixed(1));
                let ahead=null;
                for(const b of bearings){
                    const r=wrap((b.id?Math.atan2(b.x-x,z-b.z)*180/Math.PI:b.angle)-h);b.n.style.setProperty('--r',(r/90).toFixed(3));
                    if(b.id&&Math.abs(r)<14){const d=Math.hypot(b.x-x,b.z-z);if(!ahead||d<ahead.d)ahead={key:b.key,d};}
                }
                const text=ahead?L(ahead.key)+' · '+Math.round(ahead.d).toLocaleString()+' m':'';
                if(facing.textContent!==text)facing.textContent=text;
            },
            cut(){if(!inst.reducedMotion())fade.animate([{opacity:.92},{opacity:0}],{duration:450,easing:'ease-out'});},
            scope(view){
                const on=!!view;if(scopeView.hidden===on){scopeView.hidden=!on;root.classList.toggle('sw-scoping',on);}
                if(!on)return;
                const d=NS.districts.find(d=>d[0]===view.target),e=d&&current.find(e=>e.id===d[0]),row=e?.rows?.find(r=>r.value!=null&&r.value!=='');
                scopeZoom.textContent=view.zoom.toLocaleString(undefined,{minimumFractionDigits:1,maximumFractionDigits:1})+'×';scopeBearing.textContent=view.bearing+'°';
                const title=d?L(d[1]):L('sysworld.city.scope_idle');if(scopeTitle.textContent!==title)scopeTitle.textContent=title;
                scopeTag.classList.toggle('sw-idle',!d);scopeState.hidden=!e;scopeState.textContent=e?stateText(e):'';scopeState.dataset.state=e?(e.stale?'stale':e.state):'unknown';
                scopeMetric.textContent=row?L(row.key)+' '+format(row.value,row.format):'';scopeRange.textContent=d&&view.distance?view.distance.toLocaleString()+' m':'';
            },
            progress(loaded,expected){
                const percent=expected>0?Math.min(99,Math.floor(loaded/expected*100)):0;if(percent<=loadingPercent)return;
                loadingPercent=percent;loading.setAttribute('aria-valuenow',String(percent));loadingBar.style.transform='scaleX('+percent/100+')';
            },
            photo(on){photoOn=on;root.classList.toggle('sw-photo',on);photoBar.hidden=!on;photo.classList.toggle('active',on);photo.setAttribute('aria-pressed',String(on));},
            quality(value,actual){quality.value=value;quality.title=L('sysworld.btn.quality')+': '+L('sysworld.quality.'+actual);},
            sound(enabled){sound.disabled=false;sound.setAttribute('aria-pressed',String(enabled));sound.classList.toggle('active',enabled);
                sound.title=L(enabled?'sysworld.city.sound_off':'sysworld.city.sound_on');sound.setAttribute('aria-label',sound.title);volume.hidden=!enabled;},
            ready(clear=true){
                loadingPercent=100;loading.setAttribute('aria-valuenow','100');loadingBar.style.transform='scaleX(1)';loading.classList.add('sw-done');
                photo.disabled=false;if(clear)message.hidden=true;
            },
            error(key){message.textContent=L(key);message.hidden=false;loading.classList.add('sw-done');},
            project(city){
                if(labels.hidden||photoOn)return;
                if(!observer)observe();
                const items=NS.districts.map(([id])=>({id,p:pins[id],point:city.project(id)}));
                const rank=item=>item.id===focusDistrict?0:item.id===hovered?1:item.p.dataset.state==='error'?2:item.p.dataset.state==='running'?3:4;
                // Shown pins keep their place against slightly nearer rivals, so orbiting does not flicker.
                const reach=item=>(item.point?.distance??1e9)-(item.p.dataset.pin==='off'?0:40);
                items.sort((a,b)=>rank(a)-rank(b)||reach(a)-reach(b));
                const placed=[];
                for(const item of items){
                    const {p,point}=item,[w,h]=pinSize.get(p)||[90,24];let pin='off';
                    let shift=0;
                    if(point?.visible){
                        const box={l:point.x-w/2-4,r:point.x+w/2+4,t:point.y-h-4,b:point.y+14};
                        // A label cut by a panel's lower edge (or the top of the view) slides down its building by up to 90 px.
                        for(const o of [TOP,...panels])if(overlaps(o,box)&&o.b-box.t<=90)shift=Math.max(shift,o.b+2-box.t);
                        box.t+=shift;box.b+=shift;
                        if(box.l>=0&&box.t>=0&&box.r<=bounds.width&&box.b<=bounds.height&&!placed.some(o=>overlaps(o,box))&&!panels.some(o=>overlaps(o,box))){
                            placed.push(box);pin=rank(item)<2||point.distance<PIN_FAR?'near':'far';
                        }
                    }
                    if(p.dataset.pin!==pin)p.dataset.pin=pin;
                    if(pin==='off')continue;
                    const x=Math.round(point.x*2)/2,y=Math.round((point.y+shift)*2)/2,at=pinAt.get(p);
                    if(!at||at[0]!==x||at[1]!==y){pinAt.set(p,[x,y]);p.style.transform='translate('+x+'px,'+y+'px) translate(-50%,-100%)';}
                }
            },
            dispose(){observer?.disconnect();shell.remove();labels.remove();},
        };
    };
})();
