(function () {
    'use strict';
    const NS = window.SysWorld = window.SysWorld || {};
    NS.districts = [
        ['agent','sysworld.zone.core','chat',0,-12], ['infra','sysworld.zone.infra','cpu',-43,-53],
        ['integrations','sysworld.zone.integrations','network',43,-10], ['missions','sysworld.zone.missions','calendar',43,36],
        ['memory','sysworld.zone.memory','folder',-43,-10], ['graph','sysworld.zone.graph','globe',-43,36],
        ['operations','sysworld.city.operations','settings',0,36],
    ];
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
        const header = el('header','sw-top sw-glass');
        const wordmark = el('div','sw-wordmark','AURA / '); wordmark.append(el('strong','',L('sysworld.city.metropolis')));
        header.append(wordmark);
        const stats = el('div','sw-stats'), statEls = {};
        for (const name of ['cpu','ram','missions','agent']) {
            const node = el('div','sw-stat'); node.append(el('span','sw-muted',L('sysworld.stats.'+name)));
            statEls[name] = el('strong','', '—'); statEls[name].dataset.swStat = name; node.append(statEls[name]); stats.append(node);
        }
        header.append(stats); shell.append(header);
        const rail = el('aside','sw-rail sw-glass');
        const search = el('input','sw-search'); search.type = 'search'; search.placeholder = L('sysworld.city.search'); search.setAttribute('aria-label',search.placeholder);
        rail.append(search);
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
        const map = el('div','sw-map sw-glass'); map.hidden=true; map.setAttribute('aria-label',L('sysworld.city.map'));
        const mapHeading=el('h2','',L('sysworld.city.map')); map.append(mapHeading);
        const mapGrid=el('div','sw-map-grid');
        NS.districts.forEach(([id,key,symbol,x,z])=>{
            const node=btn(id,L(key),symbol); node.style.left=((x+83)/166*100)+'%';node.style.top=((z+88)/168*100)+'%';
            node.addEventListener('click',()=>inst.select(id));mapGrid.append(node);
        });map.append(mapGrid);shell.append(map);
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
        controls.append(modes,status,quality,refresh);shell.append(controls);
        const help=el('div','sw-street-help sw-glass');help.hidden=true;help.append(el('span','',L('sysworld.city.street_help')));
        const lock=btn('pointer-lock',L('sysworld.city.mouse_look'),'eye');lock.addEventListener('click',()=>inst.city?.lockPointer()?.catch(()=>{}));help.append(lock);
        const movement=el('div','sw-movement');
        for(const [key,label]of[['KeyW','↑'],['KeyA','←'],['KeyS','↓'],['KeyD','→']]){
            const button=btn(key,label,'');button.setAttribute('aria-label',L('sysworld.city.move')+' '+label);
            button.addEventListener('pointerdown',e=>{button.setPointerCapture(e.pointerId);inst.city?.moveKey(key,true);});
            ['pointerup','pointercancel','lostpointercapture'].forEach(type=>button.addEventListener(type,()=>inst.city?.moveKey(key,false)));
            movement.append(button);
        }help.append(movement);shell.append(help);
        const message=el('div','sw-message sw-glass',L('sysworld.loading'));message.setAttribute('role','status');shell.append(message);
        const labels=el('div','sw-labels');root.append(labels);
        const pins={};NS.districts.forEach(([id,key])=>{
            const p=btn(id,L(key),'');p.className='sw-pin';p.dataset.district=id;p.addEventListener('click',()=>inst.select(id));labels.append(p);pins[id]=p;
        });
        if(root.clientWidth<760){inspector.hidden=true;root.classList.remove('sw-inspecting');}
        let current=[], snapshot={sources:{},events:[]}, selected='agent', activeTab='overview', panelSignature='', resultSignature='';
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
            if(format==='bytes'){
                const units=['bytes','kib','mib','gib','tib'];let n=value,i=0;while(n>=1024&&i<4){n/=1024;i++;}
                return n.toLocaleString(undefined,{maximumFractionDigits:1})+' '+L('desktop.'+units[i]);
            }
            if(format==='percent')return Number(value).toLocaleString(undefined,{maximumFractionDigits:1})+'%';
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
                const metric=NS.data.normalizeSystemMetrics(data.sources.system?.data), agent=data.sources.overview?.data?.agent;
                statEls.cpu.textContent=format(metric.cpu,'percent');statEls.ram.textContent=format(metric.ram,'percent');
                statEls.cpu.classList.toggle('sw-stale',!data.sources.system?.at||!!data.sources.system?.failed);
                statEls.ram.classList.toggle('sw-stale',!data.sources.system?.at||!!data.sources.system?.failed);
                statEls.missions.textContent=format(data.sources.overview?.data?.missions?.total,'number');
                statEls.agent.textContent=typeof agent?.busy==='boolean'?L(agent.busy?'sysworld.agent.busy':'sysworld.agent.idle'):'—';
                NS.districts.forEach(([id])=>{counts[id].textContent=rows.filter(e=>e.district===id&&e.id!==id).length||'·';});
                const stale=rows.filter(e=>e.id===e.district&&e.stale).length;status.textContent=L(stale?'sysworld.city.partial':'sysworld.city.live');
                status.classList.toggle('sw-stale',stale>0);renderPanel();renderResults();
            },
            select(id) {
                selected=id;activeTab='overview';inspector.hidden=false;inst.root.classList.add('sw-inspecting');
                const entity=current.find(e=>e.id===id);
                for(const [district,b]of Object.entries(districtButtons)){const active=entity?.district===district;b.classList.toggle('active',active);b.setAttribute('aria-current',String(active));}
                renderPanel(true);
            },
            mode(value) {
                map.hidden=value!=='map';labels.hidden=value==='map'||value==='street';help.hidden=value!=='street';
                Object.entries(modeButtons).forEach(([id,b])=>{b.classList.toggle('active',value===id);b.setAttribute('aria-pressed',String(value===id));});
            },
            quality(value,actual){quality.value=value;quality.title=L('sysworld.btn.quality')+': '+L('sysworld.quality.'+actual);},
            ready(){message.hidden=true;},
            error(key){message.textContent=L(key);message.hidden=false;},
            project(city){for(const[id,p]of Object.entries(pins)){const point=city.project(id);p.hidden=!point?.visible;if(point)p.style.transform='translate('+point.x+'px,'+point.y+'px) translate(-50%,-100%)';}},
            dispose(){shell.remove();labels.remove();},
        };
    };
})();
