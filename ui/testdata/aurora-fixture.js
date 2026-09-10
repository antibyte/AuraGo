window.fixtureErrors = [];
addEventListener('error', e => fixtureErrors.push(e.error?.stack||e.message));
addEventListener('unhandledrejection', e => fixtureErrors.push(String(e.reason)));
const nativeFetch = window.fetch.bind(window);
const reply = data => new Response(JSON.stringify(data), {headers:{'Content-Type':'application/json'}});
const sampleFiles = [
    {name:'Documents',path:'Documents',type:'directory',modified:'2026-09-09T10:00:00Z'},
    {name:'Projects',path:'Projects',type:'directory',modified:'2026-09-09T09:30:00Z'},
    {name:'Welcome.md',path:'Welcome.md',type:'file',size:428,modified:'2026-09-09T10:30:00Z'}
];
const fixtureNotes=new Map([['Documents/Notes/aurora.md',{path:'Documents/Notes/aurora.md',title:'Kleine Ideen, große Möglichkeiten',version:'"1"',tags:['design','aurora'],modified:'2026-09-10T10:00:00Z',snippet:'Ein ruhiger Ort für Gedanken, Projekte und neue Perspektiven.',content:'# Kleine Ideen, große Möglichkeiten\n\nEin ruhiger Ort für Gedanken, Projekte und alles, was noch wachsen darf.\n\n## Der nächste Schritt\n\n- [x] Die Idee festhalten\n- [ ] Gemeinsam etwas Besonderes entwickeln\n- [ ] Die kleinen Details nicht vergessen\n\n## Werkstattnotizen\n\nGute Werkzeuge lassen uns **konzentriert arbeiten**. Der Inhalt steht im Mittelpunkt.\n\n> Eine gute Idee beginnt oft mit einer kleinen Notiz.\n\n| Projekt | Status |\n| --- | --- |\n| Aurora Workstation | In Arbeit |\n| Leafy | Wächst weiter |\n'}],['Documents/Notes/ideen.md',{path:'Documents/Notes/ideen.md',title:'Eine neue Perspektive',version:'"1"',tags:['ideen'],modified:'2026-09-09T15:00:00Z',snippet:'Manchmal beginnt etwas Großes mit einem kleinen Gedanken.',content:'# Eine neue Perspektive\n\nManchmal beginnt etwas Großes mit einem kleinen Gedanken.'}]]);
window.fixtureRequests=[];
window.fetch=async(url,options={})=>{
    const path=String(url);
    if(path.startsWith('/history')) return reply([]);
    if(!path.startsWith('/api/')) return nativeFetch(url,options);
    fixtureRequests.push(path);
    if(path.startsWith('/api/desktop/notes')) {
        let name=new URL(path,location.origin).searchParams.get('path');
        if(options.method==='PUT'||options.method==='POST'){
            const body=JSON.parse(options.body||'{}');name=body.path||name||'Documents/Notes/new.md';
            const previous=fixtureNotes.get(name);
            if(previous && options.headers?.['If-Match']!==previous.version)return new Response('{"error":"Conflict"}',{status:412});
            fixtureNotes.set(name,{path:name,title:name.split('/').pop().replace('.md',''),version:'"'+Date.now()+'"',modified:new Date().toISOString(),content:body.content});
        }
        if(name)return fixtureNotes.has(name)?reply(fixtureNotes.get(name)):new Response('{}',{status:404});
        const notes=[...fixtureNotes.values()].filter(n=>n.path.endsWith('.md'));return reply({notes,total:notes.length,folders:[]});
    }
    if(path.startsWith('/api/i18n')) return reply({data:window.I18N});
    if(path==='/api/desktop/bootstrap') return reply(aurora.state.bootstrap);
    if(path.startsWith('/api/desktop/settings')) return reply({settings:aurora.state.bootstrap.settings});
    if(path.startsWith('/api/desktop/file?'))return reply({content:'# Aurora Workstation\n\nA quiet place for ambitious ideas.\n\n- Design the desktop\n- Build something playful\n- Make every detail count\n'});
    if(path.startsWith('/api/desktop/viewer'))return reply({content:'# Aurora Workstation\n\nWelcome home. Make something remarkable.',type:'text'});
    if(path.startsWith('/api/desktop/files/read')) return reply({content:'# Welcome to AuraGo\n\nYour ideas, tools and home lab.\n'});
    if(path.startsWith('/api/desktop/files') && /path=Music/.test(path))return reply({files:[{name:'Galaga.mp3',path:'Music/Galaga.mp3',type:'file',web_path:'/img/audio/galaga.mp3'}]});
    if(path.startsWith('/api/desktop/files') && /path=Photos/.test(path))return reply({files:['city_rain','alpine_dawn','ocean_cliff','paper_waves'].map(name=>({name:name+'.jpg',path:'Photos/'+name+'.jpg',type:'file',media_kind:'image',web_path:'/img/wallpapers/'+name+'.jpg'}))});
    if(path.startsWith('/api/todos'))return reply([{id:'design',title:'Aurora Workstation',description:'Polish the small things.',status:'open',priority:'high',items:[{id:'1',title:'Explore the new desktop',done:true},{id:'2',title:'Create something remarkable',done:false}]}]);
    if(path.startsWith('/api/appointments'))return reply([{id:'review',title:'Design review',date_time:new Date().toISOString(),end_time:new Date(Date.now()+3600000).toISOString(),status:'upcoming',participants:[]}]);
    if(path.startsWith('/api/contacts'))return reply([{id:1,name:'Alex Morgan',email:'alex@example.test',relationship:'Team'},{id:2,name:'Sam Rivera',email:'sam@example.test',relationship:'Friends'},{id:3,name:'Jamie Chen',email:'jamie@example.test',relationship:'Team'}]);
    if(path.startsWith('/api/people/'))return reply([]);
    if(path.startsWith('/api/code-studio/status'))return reply({code_studio:{enabled:true,running:true}});
    if(path.startsWith('/api/code-studio/files'))return reply({path:'/workspace',files:sampleFiles});
    if(path.startsWith('/api/dashboard/system'))return reply({cpu:{usage_percent:18.4,cores:16,model_name:'Aurora Workstation'},memory:{used_percent:42.7,used:14602888806,total:34359738368},disk:{used_percent:31.2,used:624000000000,total:2000000000000},network:{bytes_sent:10000000,bytes_recv:500000000},uptime:90321});
    if(path.startsWith('/api/desktop/files')) return reply({path:'',files:sampleFiles,entries:sampleFiles,roots:[]});
    if(path.includes('/store/catalog')) return reply({catalog:[],installed:[],docker_available:false,mutations_allowed:false});
    if(path.includes('/meshcore/')) return reply({enabled:false,connected:false,conversations:[],contacts:[],channels:[],messages:[],device:{}});
    if(path.includes('/history')) return reply({messages:[],history:[],sessions:[]});
    if(path.includes('/personality')) return reply({name:'Aura',personality:'friendly'});
    if(path.includes('/notes.meta')) return reply({content:'{}'});
    return reply({enabled:false,available:false,connected:false,readonly:true,
        apps:[],items:[],entries:[],files:[],projects:[],notes:[],tasks:[],events:[],contacts:[],
        cameras:[],streams:[],devices:[],hosts:[],runs:[],presets:[],providers:[],models:[],
        personalities:[],voices:[],sessions:[],messages:[],history:[],volumes:[],computers:[],
        connections:[],printers:[],logs:[],data:[],settings:{},status:{},config:{},stats:{}});
};
window.fixtureReady=(async()=>{
    const words=await (await nativeFetch('/fixture-words')).json();
    const apps=await (await nativeFetch('/fixture-apps')).json();
    document.documentElement.lang='de';
    window.SYSTEM_LANG='de';window.BUILD_VERSION='aurora-fixture';
    window.I18N=words;
    const translate=(key,args)=>{let value=words[key]||key;for(const [k,v] of Object.entries(args||{}))value=value.replaceAll('{{'+k+'}}',v);return value;};
    window.i18n={t:translate,getLanguage:()=> 'de'};window.t=translate;
    aurora.state.bootstrap={enabled:true,builtin_apps:apps,installed_apps:[],widgets:[],shortcuts:[],desktop_files:sampleFiles,workspace:{readonly:false},settings:{
        'appearance.theme':'standard','appearance.wallpaper':'city_rain','appearance.accent':'blue',
        'appearance.density':'comfortable','appearance.icon_theme':'papirus','appearance.fruity_mode':'light',
        'windows.animations':false,'windows.restore_session':false,'desktop.show_widgets':false
    }};
    document.getElementById('vd-disabled').hidden=true;
    await aurora.loadIconManifest();
    aurora.applyDesktopSettings();aurora.renderIcons();aurora.renderStartButtonIcon();aurora.renderStartApps();
    aurora.wireShellChromeControls();aurora.bindViewportMetrics();
    document.addEventListener('keydown',aurora.handleDesktopKeydown);
    document.getElementById('vd-clock').textContent='Mi. 9. Sept.   14:32';
})();
window.fixtureTheme=theme=>{
    aurora.closeContextMenu(true);
    Object.assign(aurora.state.bootstrap.settings,{'appearance.theme':theme.startsWith('fruity')?'fruity':'standard','appearance.fruity_mode':theme.endsWith('dark')?'dark':'light','appearance.icon_theme':theme.startsWith('fruity')?'whitesur':'papirus'});
    aurora.applyDesktopSettings();aurora.renderIcons();aurora.renderStartButtonIcon();
};
window.fixtureOpen=async id=>{
    await AuraDesktopModules.loadAppAssets(id);
    window.fixtureAppId=id;
    fixtureRequests.length=0;
    if(id==='music-player'){await aurora.launchStandaloneWebamp({});return;}
    aurora.openApp(id, id==='viewer'?{path:'Welcome.md'}:id==='notes'?{path:'Documents/Notes/aurora.md'}:id==='editor'?{path:'Welcome.md',content:'# Aurora Workstation\n\nWelcome home.'}:{});
    await new Promise(r=>setTimeout(r,300));
    await document.fonts.ready;
    await Promise.all([...document.querySelectorAll('.vd-window-content img')].filter(im=>!im.loading || im.loading!=='lazy').map(im=>im.decode().catch(()=>{})));
    if(id==='music-player'){
        for(let n=0;n<40 && !document.getElementById('webamp');n++)await new Promise(r=>setTimeout(r,100));
    }
};
window.fixtureCloseAll=()=>{aurora.closeContextMenu(true);aurora.disposeWebampMusic('');[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id));};
window.fixtureArrange=()=>{
    const width=innerWidth,height=innerHeight;
    if(width<821){
        const settings=[...aurora.state.windows.values()].find(w=>w.appId==='settings');
        aurora.focusWindow(settings.id);return;
    }
    const positions=[{left:40,top:44,width:Math.min(900,width*.61),height:height*.66},{left:width*.53,top:66,width:width*.44,height:height*.63},{left:width*.22,top:height*.46,width:Math.min(760,width*.56),height:height*.42}];
    [...aurora.state.windows.values()].forEach((w,i)=>{w.element.classList.remove('maximized');for(const [k,v]of Object.entries(positions[i]||positions[0]))w.element.style[k]=v+'px';});
    aurora.showContextMenu(width-290,height-350,[{id:'new',label:'Neue Datei',icon:'file-plus'},{id:'folder',label:'Neuer Ordner',icon:'folder-plus'},{separator:true},{id:'settings',label:'Einstellungen',icon:'settings'},{id:'wallpaper',label:'Hintergrundbild',icon:'wallpaper',items:[{id:'city',label:'City Rain',icon:'check'},{id:'aurora',label:'Aurora',icon:'square'}]},{id:'refresh',label:'Aktualisieren',icon:'refresh'}]);
};
window.fixtureCheckMenus=()=>{
    const check=(ok,message)=>{if(!ok)throw Error(message);};
    const [files,writer]=[...aurora.state.windows.values()];
    let selected='';
    const menus=id=>[{id:'verify',label:'Verify',items:[
        {id:'run',label:'Run',icon:'run',action:()=>{selected=id;}},
        {id:'disabled',label:'Disabled',disabled:true,action:()=>{selected='disabled';}}
    ]}];
    const owner=()=>document.querySelector('#vd-global-menu-host > nav')?.dataset.ownerWindow;
    for(const win of [files,writer]) aurora.setWindowMenus(win.id,menus(win.id));
    check(owner()===writer.id,'Global menu must belong to the active writer');
    const original=document.querySelector('#vd-global-menu-host > nav');
    aurora.focusWindow(files.id);
    check(owner()===files.id,'Focus must move the file menu');
    document.querySelector('#vd-global-menu-host [data-window-menu="verify"]').click();
    document.querySelector('#vd-global-menu-host [data-menu-action$="/run"]').click();
    check(selected===files.id,'Global action must dispatch to its owner');
    document.querySelector('#vd-global-menu-host [data-menu-action$="/disabled"]').click();
    check(selected===files.id,'Disabled menu action ran');
    aurora.focusWindow(writer.id);
    check(document.querySelector('#vd-global-menu-host > nav')===original,'Focus must retain the menu DOM');
    aurora.switchSpace('2');check(!owner(),'Empty space retained a menu');
    aurora.switchSpace('1');check(owner()===writer.id,'Space return lost menu owner');
    aurora.minimizeWindow(writer.id);check(owner()!==writer.id,'Minimized window kept global menu');
    aurora.focusWindow(files.id);check(owner()===files.id,'Focus after minimize lost owner');
    fixtureTheme('standard');
    check(document.querySelector('#vd-global-bar').hidden,'Standard retained global bar');
    check(files.element.querySelector('.vd-window-menubar'),'Standard lost local menus');
    fixtureTheme('fruity-dark');check(owner()===files.id,'Theme switch lost menu owner');
    aurora.clearWindowMenus(files.id);check(!owner(),'Cleared menu retained an owner');
    check(!files.element.classList.contains('has-global-menu'),'Cleared menu retained header layout');
    aurora.setWindowMenus(files.id,menus(files.id));
    aurora.closeWindow(files.id);check(owner()!==files.id,'Closed window retained global menu');
    return '';
};
window.fixtureCheckIcons=()=>{
    const check=(ok,message)=>{if(!ok)throw Error(message);};
    const manifest=aurora.state.miniIconManifest;
    check(Object.keys(manifest.icons).length===80,'Mini atlas coverage changed');
    const host=document.createElement('div');document.body.appendChild(host);
    for(const size of [16,20,24]){
        host.innerHTML=aurora.iconMarkup('settings','S','',size,'action');
        const icon=host.firstElementChild;
        check(icon.classList.contains('vd-mini-icon'),'Explicit action did not route to atlas');
        check(icon.getBoundingClientRect().width===size,'Mini icon has incorrect size');
        check(getComputedStyle(icon).filter==='none','Action icon has a color filter');
        const image=getComputedStyle(icon).backgroundImage;
        fixtureTheme('standard');
        check(getComputedStyle(icon).backgroundImage!==image,'Open icon did not react to theme');
        fixtureTheme('fruity-light');
        check(!aurora.iconMarkup('settings','S','vd-taskbar-icon',16).includes('vd-mini-icon'),'Small app logo routed to mini');
        check(aurora.iconMarkup('check','V','',size,'action').includes('symbols.svg'),'Check must use SVG');
        for(const key of ['check-square','square','sort','refresh','widgets','layout','undo','redo','list','grid','columns','eye','eye-off','zoom-in','zoom-out','external','keyboard','contrast']){
            for(const role of ['vd-context-papirus-icon','vd-window-menu-papirus-icon','vd-tool-icon']){
                check(aurora.iconMarkup(key,'',role,size).includes('vd-mini-icon'),'Action used monochrome fallback: '+key);
            }
        }
        for(const key of ['chevron-right','x','minus','maximize']){
            check(aurora.iconMarkup(key,'','',size,'action').includes('symbols.svg'),'Structural glyph must stay sharp: '+key);
        }
        check(!aurora.iconMarkup('missing-icon','?','',size,'action').includes('vd-mini-icon'),'Unknown icon must fall back');
    }
    host.remove();return '';
};
window.fixtureAppResult=id=>{
    if(id==='music-player')return aurora.state.webampMusic?.instance && document.getElementById('webamp')?'':'Webamp did not render';
    const w=[...aurora.state.windows.values()].at(-1);
    if(!w)return 'No window';
    const content=w.element.querySelector('[data-window-content]');
    if(!content||!content.textContent.trim()&&!content.querySelector('canvas,iframe,video,textarea'))return 'Empty app';
    return '';
};


window.fixtureEdgeMenu=async()=>{
    aurora.showContextMenu(innerWidth-10,innerHeight-20,[{label:'Menu',icon:'settings',items:Array.from({length:12},(_,i)=>({label:'Wallpaper '+i,icon:i?'wallpaper':'check'}))}]);
    const button=document.querySelector('.vd-context-submenu > button');button.focus();
    const minimum=matchMedia('(pointer: coarse)').matches?44:document.body.dataset.density==='compact'?30:34;
    if(button.getBoundingClientRect().height<minimum)throw Error('Menu touch/density target too small');
    await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
    const pop=document.querySelector('.vd-context-submenu-popover'),r=pop.getBoundingClientRect();
    if(r.left<0||r.right>innerWidth||r.top<0||r.bottom>innerHeight)throw Error('Submenu outside viewport: '+JSON.stringify(r));
    if(!pop.contains(document.elementFromPoint(r.left+r.width/2,r.top+20)))throw Error('Submenu clipped');
    return '';
};
window.fixtureIconContact=()=>{
    fixtureCloseAll();
    const host=document.createElement('div');host.id='fixture-icon-contact';
    host.style.cssText='position:fixed;inset:60px 5% 80px;padding:24px;overflow:auto;background:var(--vd-theme-app-bg);color:var(--vd-text);border-radius:12px;z-index:400;display:grid;grid-template-columns:repeat(8,1fr);gap:16px';
    for(const key of Object.keys(aurora.state.miniIconManifest.icons)){
        const cell=document.createElement('div');cell.style.cssText='display:flex;gap:10px;align-items:center;flex-wrap:wrap;border-bottom:1px solid var(--vd-theme-border);padding:8px';
        cell.innerHTML=[16,20,24].map(size=>aurora.iconMarkup(key,'?', '',size,'action')).join('')+'<small style="width:100%">'+key+'</small>';
        host.appendChild(cell);
    }
    document.body.appendChild(host);
};
window.fixtureCaptureInfo=()=>{
    const w=[...aurora.state.windows.values()].at(-1);
    const r=w?.element.getBoundingClientRect();
    return JSON.stringify({bounds:r?{x:r.x,y:r.y,width:r.width,height:r.height}:null,requests:fixtureRequests,
        fallbacks:[...document.querySelectorAll('[data-vd-icon-key]')].filter(el=>/(btn|tool|action|menu)-icon/.test(el.className)).map(el=>el.dataset.vdIconKey)});
};

window.fixtureCheckWindowPopover=async()=>{
    Object.assign(fixtureWin.element.style,{left:(innerWidth-400)+'px',top:(innerHeight-250)+'px',width:'390px',height:'240px'});
    aurora.setWindowMenus(fixtureWin.id,[{id:'edge',label:'Menu',items:Array.from({length:12},(_,i)=>({id:String(i),label:'Action '+i,action:()=>{}}))}]);
    document.querySelector('[data-window-menu="edge"]').click();
    await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
    const pop=document.querySelector('.vd-window-menu.open > .vd-window-menu-popover'),r=pop.getBoundingClientRect();
    if(r.left<0||r.right>innerWidth||r.top<0||r.bottom>innerHeight)throw Error('Window menu outside viewport');
    if(!pop.contains(document.elementFromPoint(r.left+r.width/2,r.top+20)))throw Error('Window menu clipped');
};

window.fixtureCheckTouchLayout=async()=>{
    const active=aurora.state.windows.get(aurora.state.activeWindowId),r=active.element.getBoundingClientRect();
    if(r.width!==innerWidth||r.left!==0||r.top!==0||r.height<innerHeight-130)throw Error('Touch window does not fill workspace');
    const dock=document.querySelector('.vd-taskbar-apps').getBoundingClientRect(),system=document.querySelector('.vd-taskbar-system').getBoundingClientRect();
    if(dock.left<0||dock.right>innerWidth||system.left<0||system.right>innerWidth)throw Error('Touch dock outside viewport');
    if(document.body.dataset.theme==='fruity' && dock.bottom>system.top)throw Error('Touch dock overlaps status bar');
    if(document.body.dataset.globalMenus!=='false')throw Error('Touch menus must stay in window');
    aurora.setWindowMenus(active.id,[{id:'touch-check',label:'Menu',items:Array.from({length:5},(_,i)=>({id:String(i),label:'Action '+i,action:()=>{}}))}]);
    active.element.querySelector('[data-window-menu="touch-check"]').click();
    await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
    const pop=active.element.querySelector('.vd-window-menu.open > .vd-window-menu-popover'),bounds=pop.getBoundingClientRect();
    if(!pop.contains(document.elementFromPoint(bounds.left+30,bounds.bottom-20)))throw Error('Touch window menu clipped');
};
