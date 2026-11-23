async () => {
    const app=NotesApp.instances.get('test'), fetchOriginal=window.fetch;
    const check=(condition,message)=>{if(!condition)throw Error(message);};
    const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
    const until=async(fn,message,ms=6000)=>{const deadline=performance.now()+ms;for(;;){const value=await fn();if(value)return value;if(performance.now()>deadline)throw Error('Timed out: '+(typeof message==='function'?message():message));await sleep(25);}};
    const root=document.querySelector('.vd-notes-app'), DIR='Documents/Notes';
    const view=()=>app.editor.view;
    const key=(name,options={},target=app.editor.view.dom)=>target.dispatchEvent(new KeyboardEvent('keydown',{key:name,bubbles:true,cancelable:true,...options}));
    const row=path=>document.querySelector('.notes-tree-row[data-path="'+path+'"]');
    const palette=()=>document.querySelector('.notes-palette');
    const paletteInput=()=>document.querySelector('.notes-palette-input');
    const type=(input,value)=>{input.value=value;input.dispatchEvent(new Event('input',{bubbles:true}));};
    const rows=()=>[...document.querySelectorAll('.notes-palette-row')];
    const labels=()=>rows().map(el=>el.querySelector('.notes-palette-label')?.textContent.trim());
    const result={};
    const bodies=[];
    window.fetch=async(url,options={})=>{if(['PATCH','POST'].includes(options.method)&&String(url).startsWith('/api/desktop/notes')){try{bodies.push({method:options.method,...JSON.parse(options.body)});}catch(_){}}return fetchOriginal(url,options);};
    try{
    await app.index.load();
    await app.act('dismiss');
    if(document.querySelector('[data-right]').hidden===false)await app.act('closePanel');

    // Tree: folders and notes, click opens
    app.sidebar.setTab('files');
    await until(()=>row(DIR+'/Projekte')&&row(DIR+'/ideen-neu.md'),'tree rows');
    check(row(DIR+'/Projekte').dataset.kind==='folder'&&row(DIR+'/ideen-neu.md').dataset.kind==='note','Tree kinds');
    check(document.querySelectorAll('.notes-tree-row[data-kind=note]').length>=4,'Too few notes in the tree');
    check(!document.querySelector('[data-list]')&&!document.querySelector('[data-search]'),'Old library markup still present');
    if(row(DIR+'/Projekte').getAttribute('aria-expanded')==='true'){row(DIR+'/Projekte').click();await until(()=>row(DIR+'/Projekte')?.getAttribute('aria-expanded')==='false'&&!row(DIR+'/Projekte/plan.md'),'folder collapses');}
    row(DIR+'/Projekte').click();
    await until(()=>row(DIR+'/Projekte')?.getAttribute('aria-expanded')==='true'&&row(DIR+'/Projekte/plan.md'),'folder expands');
    row(DIR+'/Projekte/plan.md').click();
    await until(()=>app.path===DIR+'/Projekte/plan.md','opening plan from the tree');
    await until(()=>row(DIR+'/Projekte/plan.md')?.classList.contains('is-current'),'current row highlighted');
    check(row(DIR+'/Projekte/plan.md').getAttribute('aria-current')==='page','aria-current missing');
    result.tree=true;

    // Drag and drop moves a note with update_links, root drop moves it back
    const dragStep=(from,to,type2='drop')=>{
        const dt=new DataTransfer(),init={bubbles:true,cancelable:true,dataTransfer:dt};
        from.dispatchEvent(new DragEvent('dragstart',init));to.dispatchEvent(new DragEvent('dragover',init));to.dispatchEvent(new DragEvent(type2,init));from.dispatchEvent(new DragEvent('dragend',init));
    };
    await until(()=>row(DIR+'/einkauf.md'),'einkauf row');
    dragStep(row(DIR+'/einkauf.md'),row(DIR+'/Projekte'));
    await until(()=>row(DIR+'/Projekte/einkauf.md'),'einkauf under Projekte');
    const patch=bodies.find(body=>body.method==='PATCH'&&body.path===DIR+'/einkauf.md');
    check(patch&&patch.update_links===true&&patch.operation==='move'&&patch.new_path===DIR+'/Projekte/einkauf.md','Drop did not send a move with update_links: '+JSON.stringify(bodies));
    check(app.index.note(DIR+'/Projekte/einkauf.md'),'Index lacks the moved note');
    dragStep(row(DIR+'/Projekte/einkauf.md'),document.querySelector('.notes-tree'));
    await until(()=>row(DIR+'/einkauf.md')&&!row(DIR+'/Projekte/einkauf.md'),'einkauf back at the root');
    await until(()=>!document.querySelector('[data-action="new"]').disabled,'drop finished');
    await app.act('dismiss');
    result.dragDrop=true;

    // New folder
    notesContext.promptDialog=async()=>'a:b';
    document.querySelector('[data-act=new-folder]').click();
    await until(()=>!document.querySelector('[data-notice]').hidden,'invalid folder name notice');
    check(!bodies.some(body=>body.operation==='mkdir'),'Invalid folder name reached the server');
    await app.act('dismiss');
    notesContext.promptDialog=async()=>'Archiv';
    document.querySelector('[data-act=new-folder]').click();
    await until(()=>row(DIR+'/Archiv'),'new folder row');
    const mkdir=bodies.find(body=>body.operation==='mkdir');
    check(mkdir&&mkdir.folder===DIR+'/Archiv','mkdir body: '+JSON.stringify(mkdir));
    notesContext.promptDialog=async(title,value)=>value;
    result.newFolder=true;

    // Search tab: tag token and marked words
    app.sidebar.focusSearch('tag:aurora');
    const cards=()=>[...document.querySelectorAll('.notes-search-card')].map(card=>card.dataset.path);
    await until(()=>cards().length>=1,()=>'tag results: '+cards());
    check(cards().includes(DIR+'/Projekte/plan.md')&&!cards().includes(DIR+'/verweise.md')&&!cards().includes(DIR+'/ideen-neu.md'),'tag:aurora results wrong: '+cards());
    const search=document.querySelector('.notes-nav-searchbox input');
    type(search,'Perspektive');
    await until(()=>document.querySelector('.notes-search-card mark'),'marked search words');
    check(document.querySelector('.notes-search-card mark').textContent.toLowerCase().startsWith('perspektive'),'Mark does not cover the search word');
    check(cards().includes(DIR+'/ideen-neu.md'),'Word search missed ideen-neu: '+cards());
    result.search=true;

    // Tags tab
    app.sidebar.setTab('tags');
    const tag=await until(()=>[...document.querySelectorAll('.notes-tag-name')].find(el=>el.dataset.tag==='aurora'),'aurora tag');
    tag.click();
    await until(()=>document.querySelector('[data-panel=search]:not([hidden])')&&document.querySelector('.notes-nav-searchbox input').value==='tag:aurora','tag click fills the search');
    result.tags=true;

    // Bookmarks
    await app.act('pin');
    await until(()=>app.meta.pinned.includes(DIR+'/Projekte/plan.md'),'plan pinned');
    app.sidebar.setTab('bookmarks');
    await until(()=>document.querySelector('.notes-bm-row[data-path="'+DIR+'/Projekte/plan.md"]'),'bookmark row');
    await app.act('pin');
    await until(()=>!app.meta.pinned.includes(DIR+'/Projekte/plan.md'),'plan unpinned');
    app.sidebar.setTab('files');
    result.bookmarks=true;

    // Breadcrumb reveals the folder
    await until(()=>row(DIR+'/Projekte'),'files tab again');
    document.querySelector('[data-act=collapse]').click();
    await until(()=>row(DIR+'/Projekte')?.getAttribute('aria-expanded')==='false'&&!row(DIR+'/Projekte/plan.md'),'collapse all');
    await app.act('library');
    check(root.classList.contains('notes-hide-library'),'Library did not hide');
    document.querySelector('.notes-crumb[data-crumb="'+DIR+'/Projekte"]').click();
    check(!root.classList.contains('notes-hide-library'),'Crumb click did not show the library');
    await until(()=>row(DIR+'/Projekte')?.getAttribute('aria-expanded')==='true'&&row(DIR+'/Projekte/plan.md'),'crumb reveals the folder');
    result.breadcrumb=true;

    // Quick switcher: open, fuzzy query, Enter
    key('o',{ctrlKey:true});
    await until(()=>palette()&&root.__notesPalette,'switcher open');
    check(document.activeElement===paletteInput(),'Palette input not focused');
    check(rows().length>=1&&!rows().some(el=>el.classList.contains('is-create')),'Empty query should list recent notes');
    key('p',{ctrlKey:true},paletteInput());
    check(document.querySelectorAll('.notes-palette').length===1,'Shortcut fired while the palette was open');
    type(paletteInput(),'ide');
    await until(()=>labels().includes('Ideen'),()=>'ideen in switcher: '+labels());
    const at=labels().indexOf('Ideen');
    for(let i=0;i<at;i++)key('ArrowDown',{},paletteInput());
    check(rows()[at].classList.contains('is-active'),'ArrowDown did not reach the row');
    key('Enter',{},paletteInput());
    await until(()=>app.path===DIR+'/ideen-neu.md','switcher opens ideen');
    check(!palette()&&app.meta.recent[0]===DIR+'/ideen-neu.md','Switcher did not close or record recent: '+JSON.stringify(app.meta.recent));
    result.switcher=true;

    // Shift+Enter creates the typed note next to the current one
    row(DIR+'/Projekte/plan.md').click();
    await until(()=>app.path===DIR+'/Projekte/plan.md','back to plan');
    key('o',{ctrlKey:true});
    await until(()=>palette(),'switcher again');
    type(paletteInput(),'Navigationstest');
    await until(()=>rows().some(el=>el.classList.contains('is-create')),'create row');
        key('Enter',{shiftKey:true},paletteInput());
    await until(()=>app.path===DIR+'/Projekte/Navigationstest.md','created in the current folder');
    await until(()=>row(DIR+'/Projekte/Navigationstest.md'),'created note in the tree');
    result.switcherCreate=true;

    // Command palette runs the new-note command
    key('p',{ctrlKey:true});
    await until(()=>palette(),'command palette open');
    const newLabel=app.commands().find(command=>command.id==='new').label;
    check(rows().some(el=>el.querySelector('.notes-palette-hint')?.textContent==='Ctrl+N'),'Command shortcut hint missing');
    check(!app.commands().filter(command=>command.id==='print')[0].shortcut,'Print kept a shortcut');
    type(paletteInput(),newLabel);
    await until(()=>labels().includes(newLabel),()=>'new note command: '+labels());
    for(let i=0;i<labels().indexOf(newLabel);i++)key('ArrowDown',{},paletteInput());
    const before=app.path;
    key('Enter',{},paletteInput());
    await until(()=>app.path&&app.path!==before&&/\/new-\d+\.md$/.test(app.path),'command created a note');
    check(app.path.startsWith(DIR+'/Projekte/'),'New note ignored the current folder: '+app.path);
    const createBody=bodies.filter(body=>body.method==='POST'&&body.content!==undefined).pop();
    check(createBody&&createBody.folder===DIR+'/Projekte','Create body folder: '+JSON.stringify(createBody));
    result.commandPalette=true;

    // Find bar
    app.editor.setContent('# Suche\n\nAlpha beta alpha gamma Alpha.\n');
    const bar=document.querySelector('[data-findbar]'),findInput=document.querySelector('[data-find]'),count=document.querySelector('[data-find-count]');
    check(bar.hidden,'Find bar visible before Ctrl+F');
    key('f',{ctrlKey:true});
    check(!bar.hidden&&document.activeElement===findInput,'Ctrl+F did not open and focus the find bar');
    check(!document.querySelector('[data-right-tab=search]'),'Old search panel still present');
    type(findInput,'alpha');
    check(count.dataset.count==='3'&&app.editor.selection().text.toLowerCase()==='alpha','Incremental find: '+count.textContent);
    const first=app.editor.selection().from;
    key('Enter',{},findInput);
    const second=app.editor.selection().from;
    check(second>first&&document.activeElement===findInput,'Enter did not move to the next match');
    key('Enter',{shiftKey:true},findInput);
    check(app.editor.selection().from===first,'Shift+Enter did not go back');
    key('Enter',{shiftKey:true},findInput);
    const last=app.editor.selection().from;
    check(last>second,'Previous did not wrap to the last match');
    key('Enter',{},findInput);
    check(app.editor.selection().from===first,'Next did not wrap');
    document.querySelector('[data-action=findCase]').click();
    check(document.querySelector('[data-action=findCase]').getAttribute('aria-pressed')==='true'&&count.dataset.count==='1','Case toggle: '+count.textContent);
    document.querySelector('[data-action=findCase]').click();
    check(count.dataset.count==='3','Case toggle off');
    type(document.querySelector('[data-replace]'),'omega');
    document.querySelector('[data-action=replaceAll]').click();
    check(app.editor.text.includes('omega beta omega gamma omega')&&!/alpha/i.test(app.editor.text),'Replace all failed: '+app.editor.text);
    key('Escape',{},findInput);
    check(bar.hidden&&app.editor.view.dom.contains(document.activeElement),'Escape did not close the find bar and focus the editor');
    result.find=true;

    // Highlight shortcut and wikilink button
    app.editor.setContent('# Marker\n\nbeta gamma\n');
    const sel=view().state.selection.constructor,start=view().state.doc.child(0).nodeSize+1;
    view().dispatch(view().state.tr.setSelection(sel.create(view().state.doc,start,start+4)));
    key('h',{ctrlKey:true,shiftKey:true});
    check(app.editor.content().includes('==beta=='),'Ctrl+Shift+H did not highlight: '+JSON.stringify(app.editor.content()));
    app.editor.setContent('# Wiki\n\nText ');
    view().dispatch(view().state.tr.setSelection(sel.atEnd(view().state.doc)));
    document.querySelector('[data-action=wikilink]').click();
    check(app.editor.content().includes('[[')||app.editor.text.includes('[['),'Wikilink button did not insert [[: '+JSON.stringify([app.editor.content(),app.editor.text,document.querySelector('[data-action=wikilink]').disabled]));
    await until(()=>document.querySelector('.notes-suggest'),'suggest popup from the wikilink button');
    key('Escape');
    app.editor.setContent('# Wiki\n\nText ');
    result.shortcuts=true;

    // Context menu
    const popup=document.querySelector('[data-popup]');
    const menu=(path,x=120,y=200)=>row(path).dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:x,clientY:y}));
    await until(()=>row(DIR+'/ideen-neu.md'),'ideen row');
    menu(DIR+'/ideen-neu.md');
    check(!popup.hidden,'Context menu did not open');
    const actions=[...popup.querySelectorAll('button')].map(button=>button.dataset.action);
    check(['ctx:open','ctx:rename','ctx:move','ctx:pin','ctx:trash'].every(name=>actions.includes(name)),'Note actions: '+actions);
    const box=popup.getBoundingClientRect(),bounds=root.getBoundingClientRect();
    check(box.left>=bounds.left&&box.right<=bounds.right+1&&box.bottom<=bounds.bottom+1&&Math.abs(box.left-120)<40,'Popup not placed at the pointer: '+JSON.stringify(box));
    popup.querySelector('[data-action="ctx:open"]').click();
    await until(()=>app.path===DIR+'/ideen-neu.md','context open');
    menu(DIR+'/Projekte');
    check([...popup.querySelectorAll('button')].map(button=>button.dataset.action).join()==='ctx:newNote,ctx:newFolder','Folder actions');
    popup.hidden=true;
    result.contextMenu=true;

    check(errors.length===0,'Browser errors: '+errors.join('\n'));
    return {...result,recent:app.meta.recent.length,notes:app.index.notes().length};
    }finally{window.fetch=fetchOriginal;notesContext.promptDialog=async(title,value)=>value;}
}
