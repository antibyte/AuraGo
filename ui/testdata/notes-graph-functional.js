async () => {
    const app=NotesApp.instances.get('test');
    const check=(condition,message)=>{if(!condition)throw Error(message);};
    const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
    const until=async(fn,message,ms=8000)=>{const deadline=performance.now()+ms;for(;;){const value=await fn();if(value)return value;if(performance.now()>deadline)throw Error('Timed out: '+(typeof message==='function'?message():message));await sleep(25);}};
    const root=document.querySelector('.vd-notes-app'), DIR='Documents/Notes';
    const key=(name,options={},target=app.editor.view.dom)=>target.dispatchEvent(new KeyboardEvent('keydown',{key:name,bubbles:true,cancelable:true,...options}));
    const graphEl=()=>document.querySelector('[data-graph]');
    const host=()=>document.querySelector('[data-graph-host]');
    const canvas=()=>host().querySelector('canvas');
    const toggle=name=>document.querySelector('[data-graph-toggle='+name+']');
    const result={};
    const ATTACHMENT=/\.(png|jpe?g|gif|webp|svg|bmp|avif|ico|pdf|mp3|wav|ogg|m4a|flac|mp4|webm|mov|mkv|zip|csv|json|txt)$/i;

    // Expected node counts, computed independently from the index.
    const expected=({tags,unresolved})=>{
        const notes=app.index.notes(),unresolvedSet=new Set(),tagSet=new Set();
        for(const note of notes){
            for(const link of note.links||[]){
                if(app.index.linkPath(note,link)||link.embed||(link.kind&&link.kind!=='wiki'))continue;
                const target=NotesIndex.parseWikiValue(link.target).target.trim().replace(/\.md$/i,'');
                if(target&&!ATTACHMENT.test(target))unresolvedSet.add(target.toLowerCase());
            }
            for(const tag of note.tags||[])if(String(tag).trim())tagSet.add(tag);
        }
        return notes.length+(unresolved?unresolvedSet.size:0)+(tags?tagSet.size:0);
    };
    const positions=()=>{const out={};for(const note of app.index.notes()){const p=app.graph.screenPosition(note.path);if(p)out[note.path]=p;}return out;};
    const settled=async()=>{
        let previous=JSON.stringify(positions()),stable=0;
        await until(async()=>{await sleep(80);const now=JSON.stringify(positions());stable=now===previous?stable+1:0;previous=now;return stable>=3;},'graph layout settles',12000);
    };
    const pointer=(type,id,x,y,extra={})=>canvas().dispatchEvent(new PointerEvent(type,{pointerId:7,pointerType:'mouse',isPrimary:true,button:0,buttons:type==='pointerdown'?1:0,bubbles:true,cancelable:true,clientX:canvas().getBoundingClientRect().left+x,clientY:canvas().getBoundingClientRect().top+y,...extra}));
    const dist=(a,b)=>Math.hypot(a.x-b.x,a.y-b.y);
    const noteEdges=()=>{const edges=[];for(const note of app.index.notes())for(const link of note.links||[]){const path=app.index.linkPath(note,link);if(path&&path!==note.path&&app.index.note(path))edges.push([note.path,path]);}return edges;};

    await app.index.load();
    await app.act('dismiss');
    if(!document.querySelector('[data-right]').hidden)await app.act('closePanel');
    check(!app.graph&&!app.graphOpen&&graphEl().hidden,'Graph open before Ctrl+G');
    check(document.querySelector('.notes-documentbar [data-action=graph]'),'Document bar graph button missing');
    check(app.commands().some(command=>command.id==='graph'&&command.group==='view'&&command.shortcut==='Ctrl+G'),'View command graph missing');
    check(window.menus.find(menu=>menu.id==='view').items.some(item=>item.id==='graph'),'Graph missing in the View menu');

    // Open with Ctrl+G
    key('g',{ctrlKey:true});
    await until(()=>app.graph&&app.graphOpen&&!graphEl().hidden,'Ctrl+G opens the graph');
    check(document.querySelector('[data-scroll]').offsetParent===null,'Note area still visible behind the graph');
    check(app.editor,'Editor instance was dropped');
    check(document.querySelector('.notes-documentbar [data-action=graph]').getAttribute('aria-pressed')==='true','Graph button not pressed');
    check(toggle('tags').getAttribute('aria-pressed')==='false'&&toggle('unresolved').getAttribute('aria-pressed')==='true'&&toggle('orphans').getAttribute('aria-pressed')==='true','Default toggles wrong');
    const base=expected({tags:false,unresolved:true});
    check(app.graph.nodeCount()===base,'Node count '+app.graph.nodeCount()+' !== expected '+base);
    check(canvas()&&canvas().getBoundingClientRect().width>300&&canvas().getBoundingClientRect().height>200,'Canvas has no size');
    check(!document.querySelector('[data-graph-empty]').offsetParent,'Empty state visible with notes');
    result.nodes=base;
    await settled();
    const box=host().getBoundingClientRect(),bar=document.querySelector('.notes-graph-bar').getBoundingClientRect();
    check(box.top>=bar.bottom-1,'Toolbar overlaps the canvas');
    for(const [id,p] of Object.entries(positions()))check(p.x>=0&&p.y>=0&&p.x<=box.width&&p.y<=box.height,'Node outside the view after fit: '+id+' '+JSON.stringify(p)+' '+box.width+'x'+box.height);
    result.fit=true;

    // Toggles
    toggle('tags').click();
    const withTags=expected({tags:true,unresolved:true});
    await until(()=>app.graph.nodeCount()===withTags,()=>'tag nodes: '+app.graph.nodeCount()+' vs '+withTags);
    check(withTags>base,'Fixture has no tags to test');
    check(toggle('tags').getAttribute('aria-pressed')==='true'&&JSON.parse(localStorage.getItem('aurago.notes.graph')).tags===true,'Tags toggle not persisted');
    toggle('tags').click();
    await until(()=>app.graph.nodeCount()===base,'tags off again');
    toggle('unresolved').click();
    const noUnresolved=expected({tags:false,unresolved:false});
    await until(()=>app.graph.nodeCount()===noUnresolved,()=>'unresolved off: '+app.graph.nodeCount()+' vs '+noUnresolved);
    toggle('unresolved').click();
    await until(()=>app.graph.nodeCount()===base,'unresolved on again');
    toggle('orphans').click();
    await until(()=>app.graph.nodeCount()<base,'orphans hidden');
    check(app.graph.nodeCount()>0,'Orphans toggle removed everything');
    toggle('orphans').click();
    await until(()=>app.graph.nodeCount()===base,'orphans shown again');
    result.toggles=true;

    // Filter
    const input=document.querySelector('[data-graph-filter]');
    const sample=app.index.notes().find(note=>note.title&&note.title.length>=4);
    input.value=sample.title.slice(0,4);input.dispatchEvent(new Event('input',{bubbles:true}));
    await until(()=>app.graph.nodeCount()<base&&app.graph.nodeCount()>=1,()=>'filter result: '+app.graph.nodeCount());
    input.value='zzzzqqq';input.dispatchEvent(new Event('input',{bubbles:true}));
    await until(()=>app.graph.nodeCount()===0&&document.querySelector('[data-graph-empty]').offsetParent,'filter without a match shows the empty state');
    input.value='';input.dispatchEvent(new Event('input',{bubbles:true}));
    await until(()=>app.graph.nodeCount()===base,'filter cleared');
    result.filter=true;

    // Hover, wheel zoom, click
    await settled();
    app.graph.fit();
    const edges=noteEdges();
    check(edges.length>=1,'Fixture has no note links');
    const pos=id=>app.graph.screenPosition(id);
    const hoverTarget=async id=>{
        const p=pos(id);
        pointer('pointermove',7,p.x,p.y);
        return host().dataset.hover===id;
    };
    const [a,b]=edges[0];
    check(await hoverTarget(a)||await hoverTarget(b),'Hover did not mark a node');
    check(canvas().style.cursor==='pointer','Cursor not a pointer over a node');
    pointer('pointermove',7,1,1);
    await until(()=>!host().dataset.hover&&canvas().style.cursor!=='pointer','hover cleared');
    result.hover=true;

    const before=dist(pos(a),pos(b)),at=pos(a);
    canvas().dispatchEvent(new WheelEvent('wheel',{deltaY:-400,clientX:canvas().getBoundingClientRect().left+at.x,clientY:canvas().getBoundingClientRect().top+at.y,bubbles:true,cancelable:true}));
    await until(()=>dist(pos(a),pos(b))>before*1.3,()=>'wheel zoom: '+before+' -> '+dist(pos(a),pos(b)));
    const zoomed=dist(pos(a),pos(b));
    canvas().dispatchEvent(new WheelEvent('wheel',{deltaY:600,clientX:canvas().getBoundingClientRect().left+at.x,clientY:canvas().getBoundingClientRect().top+at.y,bubbles:true,cancelable:true}));
    await until(()=>dist(pos(a),pos(b))<zoomed*0.9,'wheel zoom out');
    result.zoom=true;

    app.graph.fit();
    let clickId=null;
    for(const path of app.index.notes().map(note=>note.path).filter(path=>path!==app.path)){
        const p=pos(path);if(!p)continue;
        pointer('pointermove',7,p.x,p.y);
        if(host().dataset.hover===path){clickId=path;break;}
    }
    check(clickId,'No hoverable note node to click');
    const p=pos(clickId);
    pointer('pointerdown',7,p.x,p.y);pointer('pointerup',7,p.x,p.y);
    await until(()=>app.path===clickId&&!app.graphOpen,()=>'click opens the note: '+app.path+' open='+app.graphOpen);
    check(graphEl().hidden&&!app.graph&&document.querySelector('[data-scroll]').offsetParent,'Graph did not close after the click');
    check(app.history.stack[app.history.position]===clickId,'Click did not push history');
    result.click=true;

    // Esc closes and returns focus; not while the palette is open
    await app.act('graph');
    await until(()=>app.graphOpen&&document.activeElement===canvas(),'graph reopened with focus on the canvas');
    app.openSwitcher();
    await until(()=>root.__notesPalette,'palette open over the graph');
    key('Escape',{},document.querySelector('.notes-palette-input'));
    await until(()=>!root.__notesPalette,'palette closed');
    check(app.graphOpen,'Esc in the palette closed the graph');
    canvas().focus();
    key('Escape',{},canvas());
    await until(()=>!app.graphOpen,'Esc closes the graph');
    check(app.editor.view.dom.contains(document.activeElement),'Focus did not return to the editor');
    // command route and toolbar close button
    await app.act('graph');await until(()=>app.graphOpen,'graph via command');
    document.querySelector('[data-action=graphFit]').click();
    document.querySelector('[data-action=graphClose]').click();
    await until(()=>!app.graphOpen,'close button');
    result.escape=true;

    // Local graph tab
    const linked=edges.map(edge=>edge[0]).find(path=>!path.slice(DIR.length+1).includes('/')&&path!==app.path)||edges[0][0];
    await app.act('closePanel');
    if(app.path!==linked){
        app.sidebar.setTab('files');
        await until(()=>document.querySelector('.notes-tree-row[data-path="'+linked+'"]'),'tree row of the linked note');
        document.querySelector('.notes-tree-row[data-path="'+linked+'"]').click();
        await until(()=>app.path===linked,'open the linked note');
    }
    await app.act('localGraph');
    const tab=document.querySelector('[data-right] [role=tab][aria-selected=true]');
    check(tab&&tab.dataset.rightTab==='localGraph','Local graph tab not selected');
    await until(()=>app.localGraph&&document.querySelector('[data-local-graph] canvas'),'local graph canvas');
    const local=()=>app.localGraph;
    await until(()=>local().nodeCount()>=2,()=>'local nodes: '+local()?.nodeCount());
    const depth1=local().nodeCount();
    check(document.querySelector('[data-local-graph] canvas').getBoundingClientRect().height>=270,'Local canvas too small');
    const select=document.querySelector('[data-graph-depth]');
    select.value='2';select.dispatchEvent(new Event('change',{bubbles:true}));
    await until(()=>local().nodeCount()>=depth1,()=>'depth 2: '+local().nodeCount()+' vs '+depth1);
    check(JSON.parse(localStorage.getItem('aurago.notes.graph')).depth===2,'Depth not persisted');
    const depth2=local().nodeCount();
    select.value='1';select.dispatchEvent(new Event('change',{bubbles:true}));
    await until(()=>local().nodeCount()===depth1,'depth back to 1');
    result.local={depth1,depth2};
    await app.act('closePanel');
    check(!app.localGraph,'Local graph kept after the panel closed');
    localStorage.removeItem('aurago.notes.graph');

    check(errors.length===0,'Browser errors: '+errors.join('\n'));
    return result;
}
