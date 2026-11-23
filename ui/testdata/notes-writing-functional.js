async () => {
    const app=NotesApp.instances.get('test'), fetchOriginal=window.fetch;
    const check=(condition,message)=>{if(!condition)throw Error(message);};
    const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
    const until=async(fn,message,ms=6000)=>{const deadline=performance.now()+ms;for(;;){const value=await fn();if(value)return value;if(performance.now()>deadline)throw Error('Timed out: '+(typeof message==='function'?message():message));await sleep(25);}};
    const DIR='Documents/Notes', TPL=DIR+'/Templates', PROP=DIR+'/eigenschaften.md', DAILY=DIR+'/Daily';
    const key=(name,options={},target)=>target.dispatchEvent(new KeyboardEvent('keydown',{key:name,bubbles:true,cancelable:true,...options}));
    const row=path=>document.querySelector('.notes-tree-row[data-path="'+path+'"]');
    const result={};
    const puts=[],dailyPuts=()=>puts.filter(path=>NotesTemplates.dailyDate(path));
    const put=async(path,content)=>{const response=await fetchOriginal('/api/desktop/notes?path='+encodeURIComponent(path),{method:'PUT',headers:{'Content-Type':'application/json','If-None-Match':'*'},body:JSON.stringify({path,content})});check(response.ok,'PUT '+path+' failed: '+response.status);};
    const open=async path=>{
        app.sidebar.setTab('files');
        await until(()=>row(path),'tree row '+path);
        row(path).click();
        await until(()=>app.path===path&&app.editor,'open '+path);
        await sleep(120);
    };
    const bar=()=>document.querySelector('[data-properties]');
    const chips=kind=>[...document.querySelectorAll('[data-properties] [data-chip="'+kind+'"] .notes-chip-text')].map(el=>el.textContent);
    const input=kind=>document.querySelector('[data-properties] [data-prop-input="'+kind+'"]');
    const typeIn=(el,value,name='Enter')=>{el.focus();el.value=value;key(name,{},el);};
    const popupButtons=()=>[...document.querySelectorAll('[data-popup] button')];
    const dailyNav=()=>document.querySelector('[data-daily-nav]');
    window.fetch=async(url,options={})=>{if(options.method==='PUT'&&String(url).startsWith('/api/desktop/notes'))puts.push(decodeURIComponent(String(url).split('path=')[1]||''));return fetchOriginal(url,options);};
    try{
    await app.index.load();
    await app.act('dismiss');
    if(document.querySelector('[data-right]').hidden===false)await app.act('closePanel');
    document.querySelector('.vd-notes-app').classList.remove('notes-hide-library');

    // Templates folder is created through the same PUT path as every other note (new folder, no mkdir).
    await put(TPL+'/Daily.md','# {{date:YYYY-MM-DD}} Journal\n\n## {{date:dddd}}\n');
    await put(TPL+'/Projekt.md','---\ntags: [vorlage]\n---\n# Projektvorlage\n\nNotiz: {{title}} am {{date}} um {{time}}.\n');
    await put(PROP,'---\ntags: [design, aurora]\nowner: Preserve this field\n---\n# Kleine Ideen, große Möglichkeiten\n\nEin ruhiger Ort für Gedanken, Projekte und alles, was noch wachsen darf.\n\n## Der nächste Schritt\n\n- [x] Die Idee festhalten\n- [ ] Gemeinsam etwas Besonderes entwickeln\n\n> Eine gute Idee beginnt oft mit einer kleinen Notiz.\n');
    await app.index.load();
    check(app.index.note(TPL+'/Daily.md')&&app.index.note(TPL+'/Projekt.md'),'Templates missing from the index');

    // Daily note: button creates it from Templates/Daily.md, second click only opens it
    const now=new Date(), today=NotesTemplates.dailyPath(now), todayName=NotesTemplates.dailyDate(today);
    const dailyButton=document.querySelector('.notes-documentbar [data-action="daily"]');
    check(dailyButton&&!dailyButton.disabled,'Daily button missing');
    check(dailyNav().hidden,'Daily nav visible on a normal note');
    dailyButton.click();
    await until(()=>app.path===today&&app.editor,'daily note opened');
    const weekday=NotesTemplates.formatDate(now,'dddd','de');
    const text=app.editor.content();
    check(text.includes('# '+todayName+' Journal')&&text.includes('## '+weekday),'Daily template not expanded: '+text);
    check(!text.includes('{{'),'Unexpanded variable in daily note');
    check(dailyPuts().length===1&&dailyPuts()[0]===today,'Daily note not created exactly once: '+puts.join(','));
    check(!dailyNav().hidden&&document.querySelector('[data-daily-date]').textContent===todayName,'Daily nav missing on the daily note');
    result.daily={path:app.path,created:dailyPuts().length};
    await open(DIR+'/einkauf.md');
    check(dailyNav().hidden,'Daily nav stayed visible on a normal note');
    puts.length=0;
    dailyButton.click();
    await until(()=>app.path===today,'daily note reopened');
    check(dailyPuts().length===0,'Second daily click wrote: '+puts.join(','));
    check(app.index.notes().filter(note=>NotesTemplates.dailyDate(note.path)===todayName).length===1,'Duplicate daily note');

    // Navigation walks the existing daily notes only
    const older=['2020-01-05','2020-01-03'];
    for(const name of older)await put(DAILY+'/'+name+'.md','# '+name+'\n');
    await app.index.load();
    const prev=()=>dailyNav().querySelector('[data-action="dailyPrev"]'),next=()=>dailyNav().querySelector('[data-action="dailyNext"]');
    check(!prev().disabled&&next().disabled,'Prev/next state on the newest daily note');
    const commandState=id=>app.commands().find(command=>command.id===id)?.enabled;
    check(commandState('daily')&&commandState('dailyPrev')&&!commandState('dailyNext'),'Daily commands: '+JSON.stringify(app.commands().filter(c=>c.id.startsWith('daily')).map(c=>[c.id,c.enabled])));
    const before=dailyPuts().length;
    prev().click();await until(()=>app.path===DAILY+'/2020-01-05.md','prev to 2020-01-05');
    prev().click();await until(()=>app.path===DAILY+'/2020-01-03.md','prev to 2020-01-03');
    check(prev().disabled&&!next().disabled,'Prev not disabled at the oldest note');
    next().click();await until(()=>app.path===DAILY+'/2020-01-05.md','next to 2020-01-05');
    await app.act('dailyNext');await until(()=>app.path===today,'next command back to today');
    check(next().disabled,'Next not disabled at the newest note');
    check(dailyPuts().length===before,'Navigation created notes');
    result.navigation={walked:3};

    // New note from a user template
    await open(PROP);
    await app.act('templates');
    const userButton=document.querySelector('[data-popup] [data-action="utemplate:'+TPL+'/Projekt.md"]');
    check(userButton&&!document.querySelector('[data-popup]').hidden,'User template missing from the popup: '+popupButtons().map(b=>b.dataset.action));
    check(popupButtons().some(b=>b.dataset.action==='template:blank'),'Built-in templates missing');
    check(userButton.textContent.trim()==='Projektvorlage','User template label: '+userButton.textContent);
    const previous=app.path;
    userButton.click();
    await until(()=>app.path&&app.path!==previous&&app.editor,'note from template opened');
    const created=app.editor.content();
    check(/Notiz: .+ am \d{4}-\d{2}-\d{2} um \d{2}:\d{2}\./.test(created)&&!created.includes('{{')&&created.includes('# Projektvorlage'),'Template not expanded: '+created);
    result.template={path:app.path};

    // Insert template at the caret
    check(app.commands().some(command=>command.id==='insertTemplate'&&command.enabled),'insertTemplate command missing');
    app.editor.focus();
    const titleNow=document.querySelector('[data-name]').textContent;
    await app.act('insertTemplate');
    await until(()=>document.querySelector('.notes-palette-input'),'template palette');
    await until(()=>[...document.querySelectorAll('.notes-palette-label')].some(el=>el.textContent.includes('Projektvorlage')),'template rows');
    const query=document.querySelector('.notes-palette-input');query.value='Projekt';query.dispatchEvent(new Event('input',{bubbles:true}));
    await sleep(120);
    key('Enter',{},query);
    await until(()=>app.editor.content().split('Notiz: ').length===3,'template inserted');
    const inserted=app.editor.content();
    check(inserted.split('tags:').length===2,'Frontmatter inserted with the template: '+inserted);
    check(inserted.includes('Notiz: '+titleNow+' am '),'Title variable not expanded: '+inserted);
    check(!document.querySelector('.notes-palette'),'Palette stayed open');
    result.insertTemplate=true;

    // Callout through the Insert menu
    await app.act('insert');
    const calloutEntry=document.querySelector('[data-popup] [data-action="callout"]');
    check(calloutEntry&&!document.querySelector('[data-popup]').hidden,'Callout entry missing in the Insert menu');
    calloutEntry.click();
    await until(()=>document.querySelector('[data-popup] [data-action="callout:warning"]'),'callout type popup');
    const types=popupButtons().map(b=>b.dataset.action.replace('callout:',''));
    check(['note','info','tip','success','question','warning','danger','bug','example','quote'].every(type=>types.includes(type))&&types.length===10,'Callout types: '+types);
    document.querySelector('[data-popup] [data-action="callout:warning"]').click();
    await until(()=>document.querySelector('.notes-callout[data-callout=warning]'),'warning callout rendered');
    check(app.editor.content().includes('> [!warning]'),'Callout not in the Markdown: '+app.editor.content());
    await app.act('callout');
    check(popupButtons().length===10,'Callout command popup');
    document.querySelector('[data-popup]').hidden=true;
    result.callout=true;

    // Properties bar on a note with tags and an opaque owner key: chips, read-only field, add/remove
    await open(PROP);
    check(!bar().hidden&&bar().querySelector('[data-props-toggle]'),'Properties bar missing');
    const startTags=NotesFrontmatter.parse(app.editor.content()).tags;
    check(JSON.stringify(chips('tags'))===JSON.stringify(startTags.map(tag=>'#'+tag)),'Tag chips: '+chips('tags')+' vs '+startTags);
    const labels=await (await fetchOriginal('/lang/desktop/de.json')).json();
    check(bar().querySelector('[data-prop=aliases] .notes-prop-key').textContent===labels['desktop.notes_aliases'],'Alias label: '+bar().querySelector('[data-prop=aliases] .notes-prop-key').textContent);
    check(bar().querySelector('[data-prop-field="owner"] .notes-prop-value').textContent==='Preserve this field','Owner field not shown read-only');
    check(!bar().querySelector('[data-prop-field="tags"]')&&!bar().querySelector('[data-prop-field="aliases"]'),'Tags listed twice');
    typeIn(input('tags'),'#neu tag');
    await until(()=>chips('tags').includes('#neu-tag'),'tag chip added');
    let content=app.editor.content();
    check(NotesFrontmatter.parse(content).tags.includes('neu-tag')&&content.includes('owner: Preserve this field')&&/^---\n/.test(content),'Tag write/owner: '+content.slice(0,200));
    check(app.session.dirty||document.querySelector('[data-save-state]').dataset.state!=='saved','Adding a tag did not mark the note dirty');
    check(document.activeElement===input('tags'),'Focus left the tag input after adding');
    key('Backspace',{},input('tags'));
    await until(()=>!chips('tags').includes('#neu-tag'),'Backspace removes the last chip');
    typeIn(input('tags'),'zwei');
    await until(()=>chips('tags').includes('#zwei'),'second tag');
    bar().querySelector('[data-chip-remove="tags"][data-value="zwei"]').click();
    await until(()=>!chips('tags').includes('#zwei'),'chip x removes');
    check(JSON.stringify(NotesFrontmatter.parse(app.editor.content()).tags)===JSON.stringify(startTags),'Tags changed unexpectedly');
    typeIn(input('aliases'),'Nordlicht');
    await until(()=>chips('aliases').includes('Nordlicht'),'alias chip added');
    content=app.editor.content();
    check(/aliases:\s*Nordlicht/.test(content)&&content.includes('owner: Preserve this field'),'Alias write: '+content.slice(0,220));
    bar().querySelector('[data-chip-remove="aliases"]').click();
    await until(()=>!chips('aliases').length,'alias removed');
    check(!/aliases:/.test(app.editor.content()),'Alias line left behind');
    // collapse toggle is remembered
    bar().querySelector('[data-props-toggle]').click();
    check(localStorage.getItem('aurago.notes.properties')==='0'&&!bar().querySelector('.notes-props-body')&&bar().querySelector('[data-props-toggle]').getAttribute('aria-expanded')==='false','Collapse state');
    bar().querySelector('[data-props-toggle]').click();
    check(localStorage.getItem('aurago.notes.properties')==='1'&&bar().querySelector('.notes-props-body'),'Expand state');
    result.properties={tags:startTags};

    // Typing in the body never re-renders the bar while it holds focus
    input('tags').focus();
    const stable=input('tags');
    app.editor.view.dispatch(app.editor.view.state.tr.insertText('x',2));
    await sleep(400);
    check(input('tags')===stable,'Properties bar re-rendered under the focused input');
    document.activeElement.blur();

    // A note without frontmatter shows only "+ Properties"; adding a tag creates the block
    await open(DIR+'/einkauf.md');
    check(!app.editor.content().startsWith('---'),'einkauf has frontmatter');
    check(bar().querySelector('[data-props-add]')&&!input('tags'),'Add-properties button missing');
    bar().querySelector('[data-props-add]').click();
    await until(()=>input('tags')&&input('aliases'),'empty inputs revealed');
    typeIn(input('tags'),'kauf');
    await until(()=>chips('tags').includes('#kauf'),'tag chip on a plain note');
    check(/^---\ntags: kauf\n---\n/.test(app.editor.content()),'Frontmatter block not created: '+app.editor.content().slice(0,80));
    bar().querySelector('[data-chip-remove="tags"]').click();
    await until(()=>bar().querySelector('[data-props-add]'),'back to the compact button');
    check(!app.editor.content().startsWith('---'),'Frontmatter not dropped again');
    result.plain=true;
    await app.act('dismiss');
    }finally{window.fetch=fetchOriginal;}
    check(errors.length===0,'Browser errors: '+errors.join('\n'));
    return result;
}
