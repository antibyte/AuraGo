async () => {
    const app=NotesApp.instances.get('test'), fetchOriginal=window.fetch;
    const check=(condition,message)=>{if(!condition)throw Error(message);};
    const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
    const until=async(fn,message,ms=6000)=>{const deadline=performance.now()+ms;for(;;){const value=await fn();if(value)return value;if(performance.now()>deadline)throw Error('Timed out: '+(typeof message==='function'?message():message));await sleep(25);}};
    const view=()=>app.editor.view, root=document.querySelector('.vd-notes-app');
    const keydown=(key,target=view().dom)=>target.dispatchEvent(new KeyboardEvent('keydown',{key,bubbles:true,cancelable:true}));
    const place=text=>{app.editor.setContent(text);app.editor.refreshDecorations();};
    const result={};

    // Index and chips
    await app.index.load();
    check(app.index.notes().length>=4&&app.index.note('Documents/Notes/ideen.md'),'Index not loaded');
    check(app.index.backlinks('Documents/Notes/aurora.md').some(item=>item.path==='Documents/Notes/ideen.md'),'Client-side link resolution missed ideen -> aurora');
    place('# Links\n\nSiehe [[ideen]] und [[Brandneu]].\n');
    await until(()=>view().dom.querySelectorAll('.notes-wikilink').length===2,'wikilink chips');
    const chips=()=>[...view().dom.querySelectorAll('.notes-wikilink')];
    check(!chips()[0].hasAttribute('data-unresolved'),'Resolved [[ideen]] styled as unresolved');
    check(chips()[1].hasAttribute('data-unresolved'),'Unresolved [[Brandneu]] not marked');
    const home=app.path;

    // Click opens, history works
    chips()[0].click();
    await until(()=>app.path==='Documents/Notes/ideen.md','opening ideen via chip');
    check(app.history.stack.at(-1)==='Documents/Notes/ideen.md'&&app.history.stack.at(-2)===home,'History did not push both notes');
    check(document.querySelector('[data-action=forward]').disabled,'Forward enabled without a forward entry');
    await app.act('back');
    check(app.path===home,'Back did not return');
    check(!document.querySelector('[data-action=forward]').disabled,'Forward stayed disabled after back');
    const stackLength=app.history.stack.length;
    await app.act('forward');
    check(app.path==='Documents/Notes/ideen.md'&&app.history.stack.length===stackLength,'Forward failed or pushed');
    root.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowLeft',altKey:true,bubbles:true,cancelable:true}));
    await until(()=>app.path===home,'Alt+Left');
    root.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',altKey:true,bubbles:true,cancelable:true}));
    await until(()=>app.path==='Documents/Notes/ideen.md','Alt+Right');
    await app.act('back');
    result.history=true;

    // Unresolved link creates the note
    await until(()=>chips().length===2,'chips after back');
    chips()[1].click();
    await until(()=>app.path==='Documents/Notes/Brandneu.md','creating Brandneu');
    const created=await(await fetchOriginal('/api/desktop/notes?path='+encodeURIComponent('Documents/Notes/Brandneu.md'))).json();
    check(created.content==='# Brandneu\n\n','New note content: '+JSON.stringify(created.content));
    await app.act('back');
    await until(()=>app.path===home&&chips().length===2,'back from Brandneu');
    await app.index.load();app.editor.refreshDecorations();
    check(!chips()[1].hasAttribute('data-unresolved'),'Created note still unresolved');
    result.create=true;

    // Backlinks tab and status bar
    chips()[0].click();
    await until(()=>app.path==='Documents/Notes/ideen.md','back to ideen');
    await app.act('backlinks');
    const tabs=document.querySelector('[role=tablist]');
    check(tabs&&tabs.querySelectorAll('[role=tab]').length===5&&tabs.querySelector('[role=tab][aria-selected=true]').dataset.rightTab==='backlinks','Tab strip missing or wrong tab');
    const linked=await until(()=>[...document.querySelectorAll('.notes-link-row[data-note-path]')].map(row=>row.dataset.notePath),'backlink rows');
    check(linked.includes(home)||linked.includes('Documents/Notes/verweise.md'),'Backlinks missing the linking note: '+linked);
    await until(()=>[...document.querySelectorAll('.notes-link-context')].some(el=>el.textContent.includes('[[ideen]]')),'backlink context lines');
    check(Number(document.querySelector('[data-backlinks]').dataset.count)>=1&&document.querySelector('[data-backlinks]').textContent,'Status bar backlink count missing');
    check(JSON.parse(localStorage.getItem('aurago.notes.right')).tab==='backlinks','Right sidebar state not persisted');
    document.querySelector('[data-unlinked]').open=true;
    await until(()=>document.querySelector('[data-unlinked-body] .notes-link-row'),'unlinked mentions');
    await app.act('outgoing');
    check(document.querySelector('.notes-link-row[data-note-path$="aurora.md"]'),'Outgoing links miss aurora');
    await app.act('sidebarRight');check(document.querySelector('[data-right]').hidden,'Sidebar toggle did not close');
    await app.act('sidebarRight');check(!document.querySelector('[data-right]').hidden&&document.querySelector('[role=tab][aria-selected=true]').dataset.rightTab==='outgoing','Sidebar toggle did not reopen on the last tab');
    await app.act('outgoing');check(document.querySelector('[data-right]').hidden,'act(tab) did not close the open tab');
    await app.act('backlinks');
    result.backlinks=true;

    // Editing scenarios run in the scratch note Brandneu so ideen.md keeps its content
    await app.act('back');
    await until(()=>app.path===home&&chips().length===2,'back to the first note');
    chips()[1].click();
    await until(()=>app.path==='Documents/Notes/Brandneu.md','opening the scratch note');

    // [[ autocomplete
    place('# Auto\n\nText ');
    const caretEnd=()=>{const sel=view().state.selection.constructor;view().dispatch(view().state.tr.setSelection(sel.atEnd(view().state.doc)));};
    caretEnd();
    view().dispatch(view().state.tr.insertText('[[Ide'));
    const popup=await until(()=>document.querySelector('.notes-suggest'),'suggest popup');
    check(popup.getAttribute('role')==='listbox'&&popup.querySelectorAll('[role=option]').length<=8,'Suggest popup structure');
    const labels=()=>[...document.querySelectorAll('.notes-suggest [role=option]')].map(el=>el.textContent);
    const at=labels().findIndex(text=>text.startsWith('Ideen'));
    check(at>=0,'No suggestion for Ideen: '+labels());
    for(let i=0;i<at;i++)keydown('ArrowDown');
    check(document.querySelectorAll('.notes-suggest [aria-selected=true]').length===1&&labels()[[...document.querySelectorAll('.notes-suggest [role=option]')].findIndex(el=>el.getAttribute('aria-selected')==='true')].startsWith('Ideen'),'ArrowDown did not select Ideen');
    keydown('Enter');
    check(app.editor.content().includes('[[ideen]]')&&!document.querySelector('.notes-suggest'),'Enter did not insert [[ideen]]: '+JSON.stringify(app.editor.content()));
    caretEnd();view().dispatch(view().state.tr.insertText('[[Ide'));
    await until(()=>document.querySelector('.notes-suggest'),'suggest popup again');
    keydown('Escape');
    check(!document.querySelector('.notes-suggest'),'Escape did not close the suggestions');
    place('# Auto\n\nText ');
    result.autocomplete=true;

    // Hover preview and embeds
    place('# Hover\n\nSiehe [[ideen]] und [[Nirgendwo]].\n');
    await until(()=>chips().length===2,'hover chips');
    // The engine's 450 ms hover delay can be stretched by timer throttling in headless Chrome: poll instead of sleeping.
    const card=document.querySelector('[data-preview]');
    chips()[0].dispatchEvent(new MouseEvent('mouseenter'));
    await until(()=>!card.hidden&&card.querySelector("[data-preview-body]")?.textContent.includes("Perspektive"),()=>"preview content: hidden="+card.hidden+" "+card.innerHTML.slice(0,200));
    const box=card.getBoundingClientRect(),bounds=root.getBoundingClientRect();
    check(box.width<=362&&box.height<=302&&box.left>=bounds.left&&box.right<=bounds.right+1,'Preview card outside the app or too large');
    chips()[0].dispatchEvent(new MouseEvent('mouseleave'));
    await until(()=>card.hidden,'preview hides after leaving');
    chips()[1].dispatchEvent(new MouseEvent('mouseenter'));
    await until(()=>!card.hidden&&card.querySelector('[data-create-wiki]'),()=>'missing-note hint: hidden='+card.hidden+' '+card.innerHTML.slice(0,160));
    keydown('Escape',root);check(card.hidden,'Escape did not hide the preview');
    const attachment=(app.index.data.attachments||[]).find(path=>/\.png$/i.test(path));
    check(attachment,'No attachment in the index');
    place('# Embeds\n\n![[ideen]]\n\n![['+attachment.split('/').pop()+']]\n\n![[nicht-da.png]]\n');
    const note=await until(()=>view().dom.querySelector('.notes-embed[data-kind=note] .notes-embed-body')?.textContent.includes('Perspektive')&&view().dom.querySelector('.notes-embed[data-kind=note]'),'note embed card');
    check(note.querySelector('.notes-embed-title').textContent==='Ideen','Embed card title');
    const image=await until(()=>view().dom.querySelector('.notes-embed[data-kind=image] img'),'image embed');
    check(image.loading==='lazy'&&image.getAttribute('src').startsWith('/api/desktop/download?inline=1&path='),'Image embed attributes');
    check(view().dom.querySelectorAll('.notes-embed[data-unresolved]').length===1,'Unresolved embed not marked');
    result.preview=true;

    // Targets with a slash are created below Documents/Notes, invalid names are refused, the breadcrumb shows the folder
    place('# Scratch\n\nSiehe [[Projekte/Alpha]] und [[bad:name]].\n');
    await until(()=>chips().length===2,'folder chips');
    chips()[1].click();
    await until(()=>!document.querySelector('[data-notice]').hidden,'invalid name notice');
    check(app.path==='Documents/Notes/Brandneu.md'&&document.querySelector('[data-notice]').dataset.error==='true','Invalid name was accepted');
    await app.act('dismiss');
    chips()[0].click();
    await until(()=>app.path==='Documents/Notes/Projekte/Alpha.md','creating Projekte/Alpha');
    const crumbs=document.querySelector('[data-crumbs]');
    check(!crumbs.hidden&&crumbs.textContent.includes('Projekte')&&crumbs.querySelector('[data-crumb="Documents/Notes/Projekte"]'),'Breadcrumb missing');
    await app.act('back');
    await until(()=>app.path==='Documents/Notes/Brandneu.md','back to the scratch note');
    result.folderCreate=true;

    // Move sends update_links and reports updated links
    place('# Scratch\n\nSiehe [[ideen]].\n');
    await until(()=>chips().length===1,'scratch chip');
    chips()[0].click();
    await until(()=>app.path==='Documents/Notes/ideen.md','opening ideen for the move');
    const bodies=[];
    window.fetch=async(url,options={})=>{if(options.method==='PATCH')bodies.push(JSON.parse(options.body));return fetchOriginal(url,options);};
    notesContext.promptDialog=async()=>'Documents/Notes/ideen-neu.md';
    try{await app.act('move');}finally{window.fetch=fetchOriginal;}
    check(app.path==='Documents/Notes/ideen-neu.md','Move did not open the new path: '+app.path);
    check(bodies.length===1&&bodies[0].update_links===true&&bodies[0].operation==='move','Move did not send update_links: '+JSON.stringify(bodies));
    const notice=document.querySelector('[data-notice]');
    check(!notice.hidden&&notice.dataset.error==='false','Links-updated notice missing');
    const verweise=await(await fetchOriginal('/api/desktop/notes?path='+encodeURIComponent('Documents/Notes/verweise.md'))).json();
    check(verweise.content.includes('[[ideen-neu]]')&&verweise.content.includes('[[ideen-neu#Plan|noch einmal]]'),'Mock did not rewrite links: '+verweise.content);
    check(!app.history.stack.includes('Documents/Notes/ideen.md'),'History kept the old path');
    await app.index.load();
    check(app.index.backlinks('Documents/Notes/ideen-neu.md').length>=1,'Backlinks lost after the move');
    await app.act('dismiss');
    result.move=true;

    check(errors.length===0,'Browser errors: '+errors.join('\n'));
    return {...result,notes:app.index.notes().length};
}
