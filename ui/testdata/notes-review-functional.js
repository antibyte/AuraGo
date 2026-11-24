async () => {
    const app=NotesApp.instances.get('test'), fetchOriginal=window.fetch;
    const check=(condition,message)=>{if(!condition)throw Error(message);};
    const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
    const until=async(fn,message,ms=6000)=>{const deadline=performance.now()+ms;for(;;){const value=await fn();if(value)return value;if(performance.now()>deadline)throw Error('Timed out: '+(typeof message==='function'?message():message));await sleep(25);}};
    const root=document.querySelector('.vd-notes-app'), DIR='Documents/Notes';
    const row=path=>document.querySelector('.notes-tree-row[data-path="'+path+'"]');
    const api=async(path,method,body,headers={})=>(await fetchOriginal('/api/desktop/notes?path='+encodeURIComponent(path),{method,headers:{'Content-Type':'application/json',...headers},body:body?JSON.stringify(body):undefined})).json();
    const seed=async(path,content)=>{const existing=await fetchOriginal('/api/desktop/notes?path='+encodeURIComponent(path));if(existing.ok){const note=await existing.json();return api(path,'PUT',{path,content},{'If-Match':note.version});}return api(path,'PUT',{path,content},{'If-None-Match':'*'});};
    const result={};
    const started=gate=>Promise.race([gate.began,sleep(5000).then(()=>{throw Error('The delayed save never started');})]);
    const log=[];
    window.fetch=async(url,options={})=>{log.push({url:String(url),method:options.method||'GET',body:options.body});return fetchOriginal(url,options);};
    try{
    await app.act('dismiss');
    if(!document.querySelector('[data-right]').hidden)await app.act('closePanel');
    root.classList.remove('notes-hide-library');
    app.sidebar.setTab('files');
    await seed(DIR+'/rev-a.md','# Rev A\n\nSiehe [[rev-b]].\n');
    await seed(DIR+'/rev-b.md','# Rev B\n');
    await app.index.load();
    await until(()=>row(DIR+'/rev-a.md')&&row(DIR+'/rev-b.md')&&row(DIR+'/Projekte'),'review rows');
    const open=async path=>{row(path).click();await until(()=>app.path===path&&!document.querySelector('[data-loading]').offsetParent,'opening '+path);};
    const delayPut=()=>{
        const gate={release:null,started:null};
        gate.hold=new Promise(resolve=>gate.release=resolve);gate.began=new Promise(resolve=>gate.started=resolve);
        const inner=window.fetch;
        window.fetch=async(url,options={})=>{
            if(String(url).startsWith('/api/desktop/notes')&&options.method==='PUT'&&!gate.done&&JSON.parse(options.body).path===app.path){gate.done=true;gate.started();await gate.hold;}
            return inner(url,options);
        };
        gate.restore=()=>{window.fetch=inner;};
        return gate;
    };

    // Re-entrancy: a dirty note, a slow save, and several navigations in a row. Exactly one wins.
    await open(DIR+'/rev-a.md');
    app.editor.setContent('# Rev A\n\nSiehe [[rev-b]].\n\nzweite Zeile\n');
    check(app.session.dirty,'edit did not mark the note dirty');
    let gate=delayPut(),posts=log.filter(entry=>entry.method==='POST').length;
    const first=app.act('new'),second=app.act('new');
    await started(gate);await sleep(60);
    gate.release();await Promise.all([first,second]);gate.restore();
    await until(()=>/\/new-\d+\.md$/.test(app.path||''),'new note open');
    check(log.filter(entry=>entry.method==='POST').length-posts===1,'Double new note created more than one note');
    check(app.history.stack.filter(path=>path===app.path).length===1,'History holds the new note twice: '+JSON.stringify(app.history));
    result.newTwice=true;

    app.editor.setContent('# Neu\n\nchange\n');
    gate=delayPut();
    const gets=log.length;
    const opens=[row(DIR+'/rev-a.md'),row(DIR+'/rev-b.md')];
    opens[0].click();opens[1].click();
    await started(gate);await sleep(60);
    gate.release();await until(()=>app.path===DIR+'/rev-a.md','first load wins');await sleep(200);gate.restore();
    const fetched=log.slice(gets).filter(entry=>entry.method==='GET'&&entry.url.includes('rev-b.md'));
    check(!fetched.length&&app.path===DIR+'/rev-a.md','The second load ran: '+JSON.stringify(fetched));
    check(app.history.stack.at(-1)===DIR+'/rev-a.md'&&app.history.stack.every((path,i,all)=>path!==all[i-1]),'History desynced: '+JSON.stringify(app.history));
    result.loadTwice=true;

    // Two quick Back requests move the history position exactly once.
    await open(DIR+'/rev-b.md');
    await open(DIR+'/rev-a.md');
    app.editor.setContent('# Rev A\n\nSiehe [[rev-b]].\n\ndritte Zeile\n');
    const before=app.history.position;
    gate=delayPut();
    const backs=[app.act('back'),app.act('back')];
    await started(gate);await sleep(60);
    gate.release();await Promise.all(backs);gate.restore();
    check(app.history.position===before-1,'Two Back requests moved the history by '+(before-app.history.position));
    result.backTwice=true;

    // Dragging another note: the server rewrites a link in the open note. The open note is saved first, then reloaded, and stays saveable.
    await open(DIR+'/rev-a.md');
    app.editor.setContent('# Rev A\n\nSiehe [[rev-b]].\n\nungespeichert\n');
    check(app.session.dirty,'edit did not mark rev-a dirty');
    const calls=[];
    const inner=window.fetch;
    window.fetch=async(url,options={})=>{
        if(String(url).startsWith('/api/desktop/notes')&&options.method)calls.push(options.method+' '+(JSON.parse(options.body||'{}').path||''));
        if(options.method==='PATCH'&&JSON.parse(options.body).path===DIR+'/rev-b.md'){
            const server=await(await fetchOriginal('/api/desktop/notes?path='+encodeURIComponent(DIR+'/rev-a.md'))).json();
            await api(DIR+'/rev-a.md','PUT',{path:DIR+'/rev-a.md',content:server.content.replace('[[rev-b]]','[[Projekte/rev-b]]')},{'If-Match':server.version});
        }
        return inner(url,options);
    };
    const dt=new DataTransfer(),init={bubbles:true,cancelable:true,dataTransfer:dt};
    row(DIR+'/rev-b.md').dispatchEvent(new DragEvent('dragstart',init));row(DIR+'/Projekte').dispatchEvent(new DragEvent('dragover',init));row(DIR+'/Projekte').dispatchEvent(new DragEvent('drop',init));row(DIR+'/rev-b.md')?.dispatchEvent(new DragEvent('dragend',init));
    await until(()=>row(DIR+'/Projekte/rev-b.md'),'rev-b under Projekte');
    await until(()=>!document.querySelector('[data-action="new"]').disabled&&app.editor.content().includes('[[Projekte/rev-b]]'),()=>'open note reloaded: '+app.editor.content());
    window.fetch=inner;
    check(app.editor.content().includes('ungespeichert'),'Unsaved text lost: '+app.editor.content());
    check(calls.indexOf('PUT '+DIR+'/rev-a.md')>=0&&calls.indexOf('PUT '+DIR+'/rev-a.md')<calls.indexOf('PATCH '+DIR+'/rev-b.md'),'Open note was not saved before the move: '+calls);
    app.editor.setContent(app.editor.content()+'\nnoch mehr\n');
    await app.session.save();
    check(!app.session.dirty&&document.querySelector('[data-notice]').dataset.error!=='true','Open note could not be saved after the move: '+document.querySelector('[data-notice-text]').textContent);
    const stored=await(await fetchOriginal('/api/desktop/notes?path='+encodeURIComponent(DIR+'/rev-a.md'))).json();
    check(stored.content.includes('noch mehr')&&stored.content.includes('[[Projekte/rev-b]]'),'Server lost an edit: '+stored.content);
    await app.act('dismiss');
    result.dropKeepsOpenNoteSaveable=true;

    // Alt+Arrow history shortcuts never fire from editable targets or after another handler took the key.
    const mac=/mac|iphone|ipad/i.test(String(navigator.userAgentData?.platform||navigator.platform||''));
    const arrow=(key,target)=>{const event=new KeyboardEvent('keydown',{key,altKey:true,metaKey:mac,bubbles:true,cancelable:true});target.dispatchEvent(event);return event;};
    await open(DIR+'/einkauf.md');await open(DIR+'/rev-a.md');
    const position=app.history.position;
    check(position>=1,'history too short for the shortcut test');
    check(arrow('ArrowLeft',app.editor.view.dom).defaultPrevented===false,'Alt+Arrow was swallowed in the editor');
    await app.act('search');
    const finder=document.querySelector('[data-find]');
    check(arrow('ArrowLeft',finder).defaultPrevented===false,'Alt+Arrow was swallowed in an input');
    const child=document.querySelector('[data-crumbs]')||document.querySelector('[data-name]');
    const taken=e=>e.preventDefault();child.addEventListener('keydown',taken);
    arrow('ArrowLeft',child);child.removeEventListener('keydown',taken);
    await sleep(150);
    check(app.history.position===position&&app.path===DIR+'/rev-a.md','A history shortcut ran from an editable or handled key');
    await app.act('findClose');
    check(arrow('ArrowLeft',document.querySelector('[data-action=forward]')).defaultPrevented===true,'Alt+Left from a button did not navigate');
    await until(()=>app.history.position===position-1,'Alt+Left from a button');
    arrow('ArrowRight',document.querySelector('[data-name]'));
    await until(()=>app.history.position===position,'Alt+Right from the document bar');
    const backTitle=document.querySelector('[data-action=back]').title;
    check(/\((Alt\+←|⌘⌥←)\)/.test(backTitle),'Back button title has no shortcut: '+backTitle);
    result.historyKeys=true;

    // Backlinks: an autosave of the open note does not rebuild the panel or refetch the linking notes.
    await open(DIR+'/Projekte/rev-b.md').catch(()=>{});
    if(app.path!==DIR+'/Projekte/rev-b.md'){app.sidebar.revealPath(DIR+'/Projekte/rev-b.md');await until(()=>row(DIR+'/Projekte/rev-b.md'),'rev-b row');await open(DIR+'/Projekte/rev-b.md');}
    if(!document.querySelector('[data-right]').hidden)await app.act('closePanel');
    await app.index.load();
    const linkers=()=>app.index.backlinks(app.path).map(item=>item.path);
    check(linkers().includes(DIR+'/rev-a.md'),'rev-a does not link rev-b in the index: '+linkers());
    const rowGets=()=>log.filter(entry=>entry.method==='GET'&&entry.url.includes(encodeURIComponent(DIR+'/rev-a.md'))).length;
    const base=rowGets();
    await app.act('backlinks');
    const item=await until(()=>document.querySelector('[data-linked-item="'+DIR+'/rev-a.md"]'),'backlinks row');
    await until(()=>item.querySelector('.notes-link-context'),'backlink context');
    const fetchedOnce=rowGets()-base;
    check(fetchedOnce===1,'Backlink context fetched '+fetchedOnce+' times');
    const link=item.querySelector('.notes-link-row');link.focus();
    for(let i=0;i<3;i++){
        app.editor.setContent('# Rev B\n\nrunde '+i+' '+Date.now()+'\n');
        await app.session.save();await app.index.refresh();await sleep(60);
    }
    check(document.querySelector('[data-linked-item="'+DIR+'/rev-a.md"]')===item,'The backlinks panel was rebuilt by an autosave');
    check(rowGets()-base===1,'Autosaves refetched the linking note: '+(rowGets()-base));
    check(document.activeElement===link,'Keyboard focus left the backlink row');
    // A new linking note changes the set: the panel rebuilds, contexts of known notes come from the cache.
    await seed(DIR+'/rev-c.md','# Rev C\n\nAuch [[Projekte/rev-b]] hier.\n');
    await app.index.load();
    await until(()=>document.querySelector('[data-linked-item="'+DIR+'/rev-c.md"]'),'new backlink row');
    await until(()=>document.querySelector('[data-linked-item="'+DIR+'/rev-c.md"] .notes-link-context'),'new backlink context');
    check(rowGets()-base===1,'The cached note was fetched again: '+(rowGets()-base));
    await app.act('closePanel');
    result.backlinksStable=true;

    // Excerpts (embeds and hover previews) carry foreign Markdown: injected markup never acts as app chrome.
    const NL=String.fromCharCode(10);
    const hostile=['# Inject','',
        '<span data-action="trash">X</span>','','<div data-popup>pop</div>','','<div data-preview>prev</div>','','<div data-panel-content>pc</div>','',
        '<div data-graph-host>gh</div>','','<div data-loading>ld</div>','','<div data-empty>em</div>','','<textarea data-instruction>evil</textarea>','',
        '<select data-ai-scope><option>x</option></select>','',
        '<button data-attachment-path="evil.png" data-create-wiki="Boom" data-note-path="Documents/Notes/einkauf.md" data-chip-remove="tags" data-jump="3">B</button>','',
        '<map name=m><area shape=default href="https://example.com/boom"></map><img usemap="#m" srcset="https://example.com/y.png 2x" src="data:image/png;base64,AAAA">','',
        '<video src="https://example.com/v.mp4" poster="https://example.com/p.png"></video>','',
        '<a href="javascript:alert(1)" data-x=1 id=evil name=evil>harmless link</a>','',
        '- [x] erledigt',''].join(NL);
    await seed(DIR+'/inject.md',hostile);
    await seed(DIR+'/host.md','# Host'+NL+NL+'Embed:'+NL+NL+'![[inject]]'+NL+NL+'Und ein Verweis [[inject]].'+NL);
    await app.index.load();
    await until(()=>row(DIR+'/host.md'),'host row');
    await open(DIR+'/host.md');
    const embedBody=await until(()=>document.querySelector('.notes-embed[data-kind=note] .notes-embed-body')?.textContent.includes('harmless link')&&document.querySelector('.notes-embed[data-kind=note] .notes-embed-body'),'embedded excerpt');
    const unsafe='[data-action],[data-popup],[data-preview],[data-panel-content],[data-graph-host],[data-loading],[data-empty],[data-instruction],[data-ai-scope],[data-attachment-path],[data-create-wiki],[data-note-path],[data-chip-remove],[data-jump],area,map,textarea,select,video,button,form,[srcset],[usemap],[poster],[id],[name],[href],[style]';
    const clean=(host,label)=>{
        check(!host.querySelector(unsafe),label+' keeps unsafe markup: '+host.innerHTML);
        check(!host.querySelector('img'),label+' keeps an image that does not resolve');
        check(host.textContent.includes('X')&&host.textContent.includes('harmless link'),label+' lost its text');
        check(host.querySelector('input[type=checkbox][disabled]'),label+' lost the disabled task checkbox');
        check(![...host.querySelectorAll('*')].some(el=>[...el.attributes].some(attr=>attr.name.startsWith('data-'))),label+' keeps data attributes');
    };
    clean(embedBody,'Embed card');
    const href=location.href,chrome=document.querySelector('.notes-popup'),chromePreview=document.querySelector('.notes-preview');
    const opened=[],openOriginal=window.open;window.open=(...args)=>{opened.push(args);return null;};
    const confirmOriginal=notesContext.confirmDialog;notesContext.confirmDialog=async()=>true;
    // Even markup that got into an excerpt after sanitizing (a future bug) must not act as app chrome.
    const EVIL='<span data-action="trash" id="e1">t</span><button data-attachment-path="evil.png" id="e2">a</button><button data-create-wiki="Boom" id="e3">c</button><button data-note-path="'+DIR+'/einkauf.md" id="e4">n</button><span data-jump="1" id="e5">j</span><span data-chip-remove="tags" data-value="x" id="e6">r</span><div data-popup id="e7"></div><div data-preview id="e8"></div><div data-panel-content id="e9"></div><div data-graph-host id="e10"></div><div data-loading id="e11"></div><div data-empty id="e12"></div><textarea data-instruction id="e13"></textarea><select data-ai-scope id="e14"></select><span data-right-tab="info" id="e15">tab</span><span data-crumb="'+DIR+'" id="e16">c</span>';
    const inject=host=>{const box=document.createElement('div');box.innerHTML=EVIL;host.append(...box.childNodes);};
    inject(embedBody);
    // Lookups of popup, preview, panel and loading state still reach the app chrome.
    await app.act('document');
    check(!chrome.hidden&&chrome.querySelector('button')&&!embedBody.querySelector('#e7').children.length,'The popup lookup went to the injected element');
    await app.act('dismiss');
    await app.act('outline');
    check(document.querySelector('[data-right] [data-panel-content]')&&document.querySelector('[data-right] [data-panel-content]').textContent.trim()&&!embedBody.querySelector('#e9').textContent,'The panel lookup went to the injected element');
    await app.act('closePanel');
    await app.act('assist');
    check(document.querySelector('[data-right] [data-instruction]')&&document.querySelector('[data-right] [data-ai-scope]'),'Assist panel missing');
    await app.act('closePanel');
    const chip=[...app.editor.view.dom.querySelectorAll('.notes-wikilink')].find(el=>el.textContent.includes('inject'));
    check(chip,'wikilink chip to inject');
    chip.dispatchEvent(new MouseEvent('mouseenter'));
    const card=await until(()=>!chromePreview.hidden&&chromePreview.querySelector('[data-preview-body]')?.textContent.includes('harmless link')&&chromePreview,'hover preview of the hostile note');
    const previewBody=card.querySelector('[data-preview-body]');
    clean(previewBody,'Preview card');
    check(!embedBody.querySelector('#e8').children.length&&!embedBody.querySelector('#e8').textContent,'The preview lookup went to the injected element');
    check(card.querySelector('.notes-preview-title[data-note-path]'),'Preview title lost its open action');
    // Clicks inside the preview excerpt do nothing: no trash, no navigation, no window, no writes.
    inject(previewBody);
    const logBefore=log.length,pathBefore=app.path;
    for(const id of ['e1','e2','e3','e4','e5','e6','e15','e16'])previewBody.querySelector('#'+id).click();
    await sleep(300);
    check(app.path===pathBefore&&location.href===href,'A click inside an excerpt navigated or trashed: '+app.path);
    check(!opened.length,'A click inside an excerpt opened a window');
    check(!log.slice(logBefore).some(entry=>['PATCH','POST','PUT'].includes(entry.method)),'A click inside an excerpt wrote: '+JSON.stringify(log.slice(logBefore)));
    check(document.querySelector('[data-right]').hidden,'An excerpt opened the right sidebar');
    chip.dispatchEvent(new MouseEvent('mouseleave'));
    // Clicks inside an embedded card only ever trigger the card's own behaviour (opening the embedded note).
    for(const id of ['e1','e2','e3','e5','e6','e15','e16'])embedBody.querySelector('#'+id).click();
    await sleep(300);
    check(!opened.length&&app.path!==DIR+'/einkauf.md'&&document.querySelector('[data-right]').hidden,'A click inside an embedded card acted as app chrome');
    check(!log.slice(logBefore).some(entry=>['PATCH','POST','PUT'].includes(entry.method)&&!String(entry.body).includes('notes.meta.json')),'A click inside an embedded card wrote');
    window.open=openOriginal;notesContext.confirmDialog=confirmOriginal;
    result.excerptsInert=true;

    // Names with # ^ [ ] cannot be linked by wikilinks: new folders, new notes and moves refuse them before any request.
    const press=(name,options,target)=>target.dispatchEvent(new KeyboardEvent('keydown',{key:name,bubbles:true,cancelable:true,...options}));
    const promptOriginal=notesContext.promptDialog;
    const writes=()=>log.slice(namesLog).filter(entry=>['PATCH','POST','PUT'].includes(entry.method)&&!String(entry.body).includes('notes.meta.json'));
    const refused=async(what)=>{
        await until(()=>!document.querySelector('[data-notice]').hidden&&document.querySelector('[data-notice]').dataset.error==='true',what+' notice');
        check(!writes().length,what+' reached the server: '+JSON.stringify(writes()));
        await app.act('dismiss');
    };
    let namesLog=log.length;
    for(const ch of ['#','^','[',']']){
        namesLog=log.length;
        notesContext.promptDialog=async()=>'ordner'+ch+'x';
        document.querySelector('[data-act=new-folder]').click();
        await refused('Folder name with '+ch);
        namesLog=log.length;
        press('o',{ctrlKey:true},app.editor.view.dom);
        const input=await until(()=>document.querySelector('.notes-palette-input'),'switcher for '+ch);
        input.value='Notiz'+ch+'x';input.dispatchEvent(new Event('input',{bubbles:true}));
        await until(()=>document.querySelector('.notes-palette-row.is-create'),'create row for '+ch);
        press('Enter',{shiftKey:true},input);
        await refused('Note name with '+ch);
        document.querySelector('.notes-palette-input')&&press('Escape',{},document.querySelector('.notes-palette-input'));
        await until(()=>!document.querySelector('.notes-palette'),'switcher closed');
        namesLog=log.length;
        notesContext.promptDialog=async()=>DIR+'/ziel'+ch+'name.md';
        await app.act('move');
        await refused('Move target with '+ch);
    }
    notesContext.promptDialog=promptOriginal;
    result.invalidNames=true;
    }finally{window.fetch=fetchOriginal;}
    check(errors.length===0,'Browser errors: '+errors.join('\n'));
    await app.act('dismiss');
    return result;
}
