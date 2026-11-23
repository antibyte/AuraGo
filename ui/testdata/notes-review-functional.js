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
    }finally{window.fetch=fetchOriginal;}
    check(errors.length===0,'Browser errors: '+errors.join('\n'));
    await app.act('dismiss');
    return result;
}
