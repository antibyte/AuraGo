async officeQR=>{
    const check=(value,message)=>{if(!value)throw Error(message)},wait=ms=>new Promise(resolve=>setTimeout(resolve,ms));
    const until=async condition=>{for(let i=0;i<300;i++){if(condition())return;await wait(30);}throw Error('Browser condition timed out');};
    let app=SheetsApp.instances.get('test');check(app.session,'No workbook session');app.session.suspend();
    const bytes=Uint8Array.from(atob(officeQR),c=>c.charCodeAt(0)),file=new File([bytes],'qr.png',{type:'image/png'}),sheet=app.book.getActiveSheet();
    sheet.getRange('A1:B3').setValues([['Item','Value'],['One',1],['Two',2]]);sheet.setActiveRange(sheet.getRange('D8'));
    window.AuraDesktopModules={loadAppAssets:async name=>check(name==='qr','Shared QR asset request')};
    window.AuraQRDialog={open:options=>{window.sheetsQRDialog=options;return {close(){},closed:Promise.resolve()};}};
    await app.act('qr');check(sheetsQRDialog.windowId==='test'&&sheetsQRDialog.isCurrent(),'QR dialog origin');
    sheet.setActiveRange(sheet.getRange('E8'));let stale=false;try{await sheetsQRDialog.onInsert({file})}catch(_){stale=true}check(stale&&app.state.aux.images.length===0,'Changed selection QR insertion');
    sheet.setActiveRange(sheet.getRange('D8'));await sheetsQRDialog.onInsert({file,physicalWidthMM:210*25.4/96,physicalHeightMM:210*25.4/96});
    const id=app.state.aux.images[0].id;
    check(app.state.aux.images.length===1&&app.state.aux.images[0].anchor==='D8','Cell image insertion');
    await app.api.undo();check(app.state.aux.images.length===0,'One-step image undo');await app.api.redo();check(app.state.aux.images.length===1,'Image redo');
    app.state.images.update(id,{x:11,y:13,width:250,height:250});await app.api.undo();check(app.state.aux.images[0].width===210,'Resize undo');await app.api.redo();
    sheet.insertRows(3,2);check(app.state.aux.images[0].anchor==='D10','Image row anchor');await app.api.undo();check(app.state.aux.images[0].anchor==='D8','Structural image undo');await app.api.redo();
    sheet.insertColumns(0,1);check(app.state.aux.images[0].anchor==='E10','Image column anchor');
    sheet.setName('QR sheet');sheet.setActiveRange(sheet.getRange('B1:C5'));app.state.charts.add('column');
    app.session.resume();await app.state.save();check(!app.session.dirty,'Image+chart native save');
    const path=app.path;await app.state.load(path);check(app.state.aux.images.length===1&&app.state.aux.images[0].anchor==='E10'&&app.state.aux.charts.length===1,'Image+chart reopen');
    stale=false;try{await sheetsQRDialog.onInsert({file})}catch(_){stale=true}check(stale&&app.state.aux.images.length===1,'Old document QR insertion');
    await until(()=>document.querySelector('[data-image-id] img')?.naturalWidth===210);
    const element=document.querySelector('[data-image-id]');check(element.getBoundingClientRect().width>0,'Embedded image display');
    for(const theme of ['standard','fruity']){document.body.dataset.theme=theme;document.body.dataset.fruityMode=theme==='fruity'?'light':'';app.state.refresh();check(element.getBoundingClientRect().width>0,'Theme image display');}
    const currentID=app.state.aux.images[0].id;element.focus();element.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true}));check(app.state.aux.images[0].x===12,'Keyboard image move');
    element.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',shiftKey:true,bubbles:true}));check(app.state.aux.images[0].width===254,'Keyboard proportional resize');
    app.state.images.remove(currentID);check(app.state.aux.images.length===0,'Image delete');await app.api.undo();check(app.state.aux.images.length===1,'Image delete undo');
    app.book.getActiveSheet().setActiveRange(app.book.getActiveSheet().getRange('A1:K24'));
    window.AuraDesktopPrint.printHTML=async options=>{window.imagePrint=options.html};
    await app.act('print');document.querySelector('[data-panel-action="printNow"]').click();await until(()=>!!window.imagePrint);
    check(imagePrint.includes('sheets-print-image')&&imagePrint.includes(officeQR),'Print embeds current image pixels');
    await app.state.save();await app.act('saveAs');check(app.path==='Documents/copy.xlsx','Save As path');await app.state.load(app.path);check(app.state.aux.images.length===1,'Save As image reopen');
    const source=await(await fetch('/api/desktop/office/workbook?representation=editor-v2&path='+encodeURIComponent(app.path))).json(),persistedImage=source.images[0].data;
    app.session.suspend();app.state.images.update(app.state.aux.images[0].id,{x:31});await wait(750);
    const draft=await OfficeSession.draft('get',location.origin+':'+app.path,undefined,'sheets');check(draft?.snapshot.images[0].x===31,'Image recovery draft');
    const reopenPath=app.path;SheetsApp.dispose('test');SheetsApp.render(document.getElementById('host'),'test',{...sheetsContext,path:reopenPath});
    await until(()=>SheetsApp.instances.get('test')?.session&&SheetsApp.instances.get('test').state.aux.images?.[0]?.x===31);
    app=SheetsApp.instances.get('test');app.session.suspend();
    const oldFetch=window.fetch;window.fetch=async(url,options)=>options?.method==='PATCH'?new Response(JSON.stringify({error:'Conflict'}),{status:412,headers:{'Content-Type':'application/json'}}):oldFetch(url,options);
    let conflicted=false;try{await app.state.save()}catch(error){conflicted=error.status===412}finally{window.fetch=oldFetch}
    check(conflicted&&app.state.aux.images[0].x===31&&app.session.dirty,'Image conflict lost draft');
    SheetsApp.dispose('test');SheetsApp.render(document.getElementById('host'),'test',{...sheetsContext,path:reopenPath,readonly:true});await until(()=>!!SheetsApp.instances.get('test')?.session);
    app=SheetsApp.instances.get('test');const count=app.state.aux.images.length,revision=app.session.revision;
    app.state.images.remove(app.state.aux.images[0].id);check(app.state.aux.images.length===count&&app.session.revision===revision,'Readonly image delete');
    let refused=false;try{await app.state.images.add(new File([],'image.png',{type:'image/png'}))}catch(_){refused=true}check(refused,'Readonly image insertion');
    check(sheetErrors.length===0,sheetErrors.join('\n'));SheetsApp.dispose('test');return persistedImage;
}
