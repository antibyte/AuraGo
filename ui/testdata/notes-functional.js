async () => {
    const app=NotesApp.instances.get('test'), fetchOriginal=window.fetch, draftOriginal=OfficeSession.draft;
    const check=(condition,message)=>{if(!condition)throw Error(message);};
    const savedContent=app.editor.content();
    const writeText=text=>{app.editor.setContent(text);};
    writeText('# Search\n\nHello **beautiful** world.\n');
    check(app.editor.findText('beautiful world')===1,'Search missed a formatting boundary');
    check(app.editor.findText('[missing]')===0,'Search interpreted a literal as regexp');
    app.editor.findText('beautiful world','wonderful world',true);
    check(app.editor.text.includes('wonderful world'),'Replacement failed');
    app.editor.command('undo');
    check(app.editor.content().includes('**beautiful**'),'Undo lost original formatting');
    const linked='---\nexample: "[literal](image.png)"\n---\n[link](image.png "title")\n\n\u0060[code](image.png)\u0060\n\n~~~md\n[code](image.png)\n~~~\n\n[ref]: image.png "title"\n\n[reference][ref]\n';
    const moved=app.editor.relocateLinks(linked,url=>'../'+url);
    check(moved.includes('[link](../image.png "title")'),'Inline relative link not moved');
    check(moved.includes('[ref]: ../image.png "title"'),'Reference destination not moved');
    check(moved.includes('example: "[literal](image.png)"')&&moved.includes('\u0060[code](image.png)\u0060')&&moved.includes('~~~md\n[code](image.png)\n~~~'),'Move rewrote literal source');

    await app.session.save();
    let release, started;
    const began=new Promise(resolve=>started=resolve);
    let first=true,active=0,maxActive=0;
    window.fetch=async(url,options={})=>{
        if(String(url).startsWith('/api/desktop/notes')&&options.method==='PUT'&&JSON.parse(options.body).path===app.path){
            active++;maxActive=Math.max(maxActive,active);
            if(first){first=false;started();await new Promise(resolve=>release=resolve);}
            try{return await fetchOriginal(url,options);}finally{active--;}
        }
        return fetchOriginal(url,options);
    };
    writeText('# First revision');
    const saving=app.session.save();await began;
    writeText('# Newest revision');release();await saving;
    check(app.session.dirty,'Old save acknowledged newer edits');
    await app.session.save();
    check(maxActive===1&&!app.session.dirty,'Saves were concurrent or failed to drain');
    window.fetch=fetchOriginal;
    let server=await(await fetchOriginal('/api/desktop/notes?path='+encodeURIComponent(app.path))).json();
    check(server.content.trim()==='# Newest revision','Newest revision did not reach server');

    await fetchOriginal('/api/desktop/notes',{method:'PUT',headers:{'Content-Type':'application/json','If-Match':server.version},body:JSON.stringify({path:app.path,content:'# Another instance'})});
    writeText('# Local conflict');
    let conflict=false;try{await app.session.save();}catch(error){conflict=error.status===412;}
    check(conflict&&app.session.dirty,'Conflict silently overwrote another instance');
    const oldPath=app.path;
    notesContext.confirmDialog=async()=>true;
    OfficeSession.draft=async()=>{throw Error('Storage unavailable');};
    check(!await closeGuard(),'Close accepted without either server save or recovery draft');
    OfficeSession.draft=draftOriginal;
    check(await closeGuard(),'Explicitly leaving a durable draft failed');
    const recovery=await draftOriginal('get',location.origin+':'+oldPath,undefined,'notes');
    check(recovery.content.trim()==='# Local conflict','Recovery did not preserve current text');
    await app.act('saveAs');
    check(app.path==='Documents/Notes/copy.md'&&app.editor.content().trim()==='# Local conflict'&&!app.session.dirty,'Conflict copy failed');
    server=await(await fetchOriginal('/api/desktop/notes?path='+encodeURIComponent(oldPath))).json();
    check(server.content==='# Another instance','Save As altered the other instance');

    notesContext.openFileDialog=async()=>({path:'Documents/Notes/missing.md'});
    await app.act('open');
    check(app.path==='Documents/Notes/copy.md'&&app.editor.content().trim()==='# Local conflict','Failed load replaced the current note');
    await app.act('dismiss');

    await app.act('assist');
    document.querySelector('[data-ai-scope]').value='paragraph';
    window.fetch=async(url,options)=>{
        if(url==='/api/desktop/office/assist'){
            const body=JSON.parse(options.body);
            return new Response(JSON.stringify({replacement:'Suggested text',source_revision:body.source_revision}),{status:200,headers:{'Content-Type':'application/json'}});
        }
        return fetchOriginal(url,options);
    };
    const before=app.editor.content();
    await app.act('generate');
    check(app.editor.content()===before,'AI changed text without acceptance');
    writeText('# Changed during review');
    await app.act('apply');
    check(app.editor.content()!=='# Suggested text','Stale AI proposal applied');
    await app.act('generate');await app.act('apply');
    check(app.editor.text.includes('Suggested text'),'Explicit AI apply failed');
    app.editor.command('undo');
    check(app.editor.text==='Changed during review','AI was not one undoable action');
    window.fetch=fetchOriginal;
    await app.act('closePanel');

    const recoveredHost=document.createElement('div');document.body.append(recoveredHost);
    const recoveredApp=NotesApp.render(recoveredHost,'recovery-test',{...notesContext,path:oldPath,setWindowBeforeClose:()=>{}});
    await recoveredApp.ready;
    check(recoveredApp.editor.content().trim()==='# Local conflict'&&recoveredApp.session.dirty,'Explicit recovery failed');
    recoveredApp.dispose();recoveredHost.remove();
    const key=location.origin+':atomic-test';
    await draftOriginal('put',key,{id:'new',content:'newest'},'notes');
    await draftOriginal('deleteIf',key,{id:'old'},'notes');
    check((await draftOriginal('get',key,undefined,'notes')).content==='newest','Old save deleted newer recovery');
    await draftOriginal('delete',key,undefined,'notes');

    writeText('# Diagram\n\n\u0060\u0060\u0060mermaid\ngraph LR\n A --> B\n\u0060\u0060\u0060\n');
    await new Promise((resolve,reject)=>{
        const done=()=>{if(document.querySelector('.notes-diagram svg')){clearTimeout(timeout);observer.disconnect();resolve();}};
        const observer=new MutationObserver(done);
        const timeout=setTimeout(()=>{observer.disconnect();reject(Error('Mermaid did not render'));},8000);
        observer.observe(document.getElementById('host'),{childList:true,subtree:true});done();
    });
    writeText(savedContent);await app.session.save();
    const host=document.createElement('div');document.body.append(host);
    const readOnly=await NotesEditor.create(host,{content:'# Locked',readonly:true,tr:key=>key,changed:()=>{throw Error('Readonly editor changed');}});
    readOnly.replaceSelection('Mutation',{from:1,to:7,source:false});readOnly.insert('Mutation');readOnly.command('bold');
    check(readOnly.content()==='# Locked','Readonly rich editor mutated');
    await readOnly.destroy();host.remove();
    const bigHost=document.createElement('div');document.body.append(bigHost);
    const big='# Large note\n\n'+'This remains intact.\n'.repeat(20000),start=performance.now();
    const bigEditor=await NotesEditor.create(bigHost,{content:big,readonly:false,tr:key=>key,changed:()=>{}});
    check(bigEditor.sourceMode&&bigEditor.content()===big,'Large note was lost or parsed into rich layout');
    check(performance.now()-start<10000,'Large source note is unresponsive');
    await bigEditor.destroy();bigHost.remove();

    check(errors.length===0,'Browser errors: '+errors.join('\n'));
    return {serialSaves:true,conflictCopy:true,recovery:true,readonly:true,aiRevision:true,linkPreservation:true,largeNoteChars:big.length};
}