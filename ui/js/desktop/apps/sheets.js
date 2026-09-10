(function () {
    'use strict';
    const instances=new Map();
    let enginePromise;
    const escapeHTML=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const basename=path=>String(path).split('/').pop();
    const newPath=()=> 'Documents/untitled-'+crypto.randomUUID().slice(0,8)+'.xlsx';
    const endpoint=path=>'/api/desktop/office/workbook?representation=editor-v2&path='+encodeURIComponent(path);
    async function responseOK(response) {
        if(response.ok)return response;
        const body=await response.json().catch(()=>({})),error=new Error(body.error||body.message||('HTTP '+response.status));
        error.status=response.status;throw error;
    }
    function render(host, windowId, context) {
        dispose(windowId);
        const ctx=context||{},esc=ctx.esc||escapeHTML;
        const tr=(key,params)=>{
            for(const prefix of ['sheets_','writer_','']){const id='desktop.'+prefix+key,value=ctx.t?.(id,params);if(value&&value!==id)return value;}
            return key;
        };
        const icons={new:'file-plus',open:'folder-open',save:'save',saveAs:'copy',navigation:'list',search:'search',format:'sliders',data:'grid',chart:'analytics',assist:'star',undo:'undo',redo:'redo',print:'printer',addSheet:'plus',sheetMenu:'more-horizontal',borders:'grid',fill:'palette',color:'palette',merge:'columns',zoomIn:'zoom-in',zoomOut:'zoom-out',zoomReset:'search',more:'menu',closeLeft:'x',closeRight:'x',cancelFormula:'x',commitFormula:'check',expandFormula:'chevron-down'};
        const button=(action,label,symbol)=>'<button type="button" data-action="'+action+'" title="'+esc(tr(label||action))+'" aria-label="'+esc(tr(label||action))+'">'+(ctx.iconMarkup?.(icons[action]||action,symbol,'sheets-icon',18,'action')||esc(symbol))+'</button>';
        host.innerHTML='<div class="sheets-app"><header class="sheets-documentbar"><div class="sheets-document"><button class="sheets-document-name" data-action="document"><span data-name></span><span aria-hidden="true">⌄</span></button><span data-save-state role="status" aria-live="polite">'+esc(tr('loading'))+'</span></div><nav class="sheets-views">'+button('navigation','navigation','☷')+button('search','find','⌕')+'<span class="sheets-divider"></span>'+button('format','format','A')+button('data','data','▦')+button('chart','chart','▥')+button('assist','assist','✦')+'</nav></header>'+
            '<div class="sheets-document-menu" data-document-menu hidden><strong data-location></strong><button data-action="rename">'+esc(tr('rename'))+'</button><button data-action="saveAs">'+esc(tr('save_as'))+'</button><button data-action="save">'+esc(tr('save'))+'</button></div>'+
            '<div class="sheets-toolbar" role="toolbar" aria-label="'+esc(tr('format'))+'"><div class="sheets-toolgroup">'+button('undo','undo','↶')+button('redo','redo','↷')+'</div><div class="sheets-toolgroup sheets-fonts"><select data-format="font" aria-label="'+esc(tr('font'))+'">'+['Arial','Calibri','Carlito','Geist','Georgia','Times New Roman','Courier New'].map(name=>'<option>'+name+'</option>').join('')+'</select><input data-format="size" type="number" min="1" max="200" step=".5" value="11" aria-label="'+esc(tr('size'))+'"></div><div class="sheets-toolgroup">'+button('bold','bold','B')+button('italic','italic','I')+button('underline','underline','U')+'</div><div class="sheets-toolgroup"><select data-format="number" aria-label="'+esc(tr('number_format'))+'">'+[['General','general'],['0.00','format_number'],['0.00%','format_percent'],['#,##0.00 "€"','format_currency'],['yyyy-mm-dd','format_date'],['@','format_text']].map(([v,k])=>'<option value="'+esc(v)+'">'+esc(tr(k))+'</option>').join('')+'</select></div><div class="sheets-toolgroup">'+button('left','align_left','≡')+button('center','align_center','≡')+button('right','align_right','≡')+'</div><div class="sheets-toolgroup">'+button('borders','format_borders','▦')+button('fill','fill_color','▰')+button('merge','merge_cells','↔')+'</div>'+button('more','more','•••')+'</div>'+
            '<div class="sheets-formula-line"><input data-address aria-label="'+esc(tr('address'))+'" value="A1" spellcheck="false"><button data-action="function" class="sheets-fx" title="'+esc(tr('function_assistant'))+'">ƒx</button><div class="sheets-formula-wrap"><textarea data-formula rows="1" spellcheck="false" aria-label="'+esc(tr('formula'))+'"></textarea><div data-formula-hints class="sheets-formula-hints" hidden></div></div><span data-formula-actions hidden>'+button('cancelFormula','cancel','×')+button('commitFormula','apply','✓')+'</span>'+button('expandFormula','expand_formula','⌄')+'</div>'+
            '<div class="sheets-notice" data-notice role="alert" hidden><span data-notice-text></span><button data-action="retry">'+esc(tr('retry'))+'</button><button data-action="saveAs">'+esc(tr('save_as'))+'</button><button data-action="dismiss" aria-label="'+esc(tr('close'))+'">×</button></div>'+
            '<div class="sheets-workspace"><aside class="sheets-left" data-left hidden></aside><main class="sheets-canvas"><div class="sheets-engine" data-engine></div><div class="sheets-charts" data-charts></div><div class="sheets-loading" data-loading>'+esc(tr('loading'))+'</div></main><aside class="sheets-right" data-right hidden></aside></div>'+
            '<div class="sheets-tabsbar"><div data-tabs class="sheets-tabs" role="tablist" aria-label="'+esc(tr('navigation'))+'"></div>'+button('addSheet','add_sheet','+')+button('sheetMenu','sheet_options','•••')+'</div><footer class="sheets-statusbar"><span data-selection></span><span class="sheets-status-spacer"></span><span data-stats></span>'+button('zoomOut','zoom_out','−')+'<button data-action="zoomReset" data-zoom>100%</button>'+button('zoomIn','zoom_in','+')+'</footer></div>';
        const root=host.firstElementChild,find=selector=>root.querySelector(selector),mount=find('[data-engine]');
        const lifecycle=new AbortController();
        let documentLife=new AbortController(),engine,api,book,lib,queue,bridge,panels,chartView;
        let path=ctx.path||newPath(),etag=null,loading=true,disposed=false,generation=0,leftPanel='',rightPanel='',refreshTimer;
        let aux={charts:[],print:{}},operations=[],loadingFailed=false,structuralLocked=false,formulaTarget=null,formulaChanged=false,pin=null,lastDraft=null,backupTimer,backupDeadline,sourceData=null,recoveryConflict=false,nativeEdit=null;
        const disposables=[],locale=document.documentElement.lang||'en';
        const state={ctx,esc,tr,root,find,locale,act,notice,fail,refresh,run,selected,selection,commitFormula,snapshot,prepareOutput,
            get api(){return api;},get book(){return book;},get charts(){return chartView;},get lib(){return lib;},get readonly(){return !!ctx.readonly;},
            get revision(){return queue?.revision||0;},get signal(){return documentLife.signal;},get aux(){return aux;},
            setAux(value){if(!ctx.readonly)bridge.commit(value,book.getId());},
            format(type,value){return api.executeCommand('sheet.command.set-style',{unitId:book.getId(),subUnitId:sheet().getSheetId(),style:{type,value}});},
            get clipboard(){return bridge.clipboard();},
            get path(){return path;},get etag(){return etag;},
            prompt:(key,value)=>ctx.promptDialog(tr(key),value??''),
            confirmKey:key=>ctx.confirmDialog(tr(key)),
            download,save,load
        };
        const instance={act,dispose:cleanup,get api(){return api;},get book(){return book;},get session(){return queue;},get path(){return path;},get state(){return state;}};
        instances.set(windowId,instance);
        ctx.wireContextMenuBoundary?.(host);ctx.setWindowBeforeClose?.(windowId,guard);ctx.registerWindowCleanup?.(windowId,cleanup);
        function notice(text,error=false){if(disposed)return;find('[data-notice-text]').textContent=text||'';find('[data-notice]').hidden=!text;find('[data-notice]').dataset.error=String(error);}
        function fail(error){if(error?.name!=='AbortError'&&!disposed)notice(error?.status===412?tr('conflict'):(error?.message||String(error)),true);}
        function sheet(){return book?.getActiveSheet();}
        function selection(){return sheet()?.getActiveRange()?.getRange()||(pin&&pin.sheet===sheet()?.getSheetId()?pin.range:null)||{startRow:0,endRow:0,startColumn:0,endColumn:0};}
        function selected(){return sheet()?.getRange(selection());}
        function pinSelection(){if(book)pin={sheet:sheet().getSheetId(),range:sheet().getActiveRange()?.getRange()||selection()};}
        function run(fn){if(!book||loading||ctx.readonly)return;try{const result=fn();if(result?.catch)result.catch(fail);refreshSoon();return result;}catch(error){fail(error);}}
        function draftKey(){return location.origin+':'+path;}
        async function backup(value,revision){
            const entry={id:crypto.randomUUID(),snapshot:value,revision,etag,path,sourceData,updated:Date.now()};
            try{await OfficeSession.draft('put',draftKey(),entry,'sheets');lastDraft=entry;return true;}catch(_){notice(tr('draft_unavailable'),true);return false;}
        }
        function scheduleBackup(){
            clearTimeout(backupTimer);
            const store=()=>{clearTimeout(backupTimer);clearTimeout(backupDeadline);backupTimer=backupDeadline=null;if(book&&queue?.dirty)backup(snapshot(),queue.revision);};
            backupTimer=setTimeout(store,600);if(!backupDeadline)backupDeadline=setTimeout(store,4000);
        }
        function snapshot(){
            commitFormula();
            const workbook=structuredClone(book.save());
            for(const id of workbook.sheetOrder){const active=book.getSheetBySheetId(id);for(const [row,columns]of Object.entries(workbook.sheets[id].cellData||{}))for(const [col,cell]of Object.entries(columns))if(cell?.si){cell.f=active.getRange(+row,+col).getFormula();delete cell.si;}}

            workbook.custom={...(workbook.custom||{}),auragoPrint:aux.print};
            return {schema_version:2,workbook,charts:structuredClone(aux.charts),operations:structuredClone(operations)};
        }
        async function write(value,target=path,expected=etag){
            if(recoveryConflict&&(target===path||!sourceData)){const e=new Error(tr('draft_conflict'));e.status=412;throw e;}
            const source=target!==path&&sourceData?{source_data:sourceData}:target!==path&&etag?{source_path:path,source_etag:etag}:{};
            const response=await responseOK(await fetch(endpoint(target),{method:'PATCH',credentials:'same-origin',signal:documentLife.signal,
                headers:{'Content-Type':'application/json',...(expected?{'If-Match':expected}:{'If-None-Match':'*'})},body:JSON.stringify({...value,...source})}));
            const result=await response.json(),version=response.headers.get('ETag');
            if(target===path){etag=version;sourceData=result.source_data||sourceData;operations.splice(0,value.operations.length);}
            return {etag:version,sourceData:result.source_data};
        }
        function makeQueue(){
            queue=OfficeSession.create({readonly:ctx.readonly,serialize:()=>snapshot(),backup,write,
                clearBackup:async()=>{const key=draftKey(),draft=await OfficeSession.draft('get',key,undefined,'sheets').catch(()=>null);if(draft?.id===lastDraft?.id)await OfficeSession.draft('delete',key,undefined,'sheets').catch(()=>{});},
                onState:value=>{find('[data-save-state]').textContent=tr(value.error?'save_failed':value.saving?'saving':value.dirty?'unsaved':'saved');find('[data-save-state]').dataset.state=value.error?'error':value.dirty?'dirty':'saved';if(value.error)fail(value.error);}
            });
        }
        async function load(target,template){
            const token=++generation;
            releaseDocument();documentLife=new AbortController();path=target;etag=null;sourceData=null;recoveryConflict=false;nativeEdit=null;loading=true;loadingFailed=false;operations=[];lastDraft=null;pin=null;formulaTarget=null;formulaChanged=false;
            aux={charts:[],print:{}};mount.replaceChildren();find('[data-charts]').replaceChildren();
            find('[data-name]').textContent=basename(path);find('[data-location]').textContent=path;find('[data-loading]').hidden=false;find('[data-loading]').textContent=tr('loading');notice('');
            ctx.updateWindowContext?.(windowId,{path});
            try{
                enginePromise ||= import('/js/vendor/sheets/engine.js').catch(error=>{enginePromise=null;throw error;});lib=await enginePromise;
                let doc;
                if(template)doc=window.SheetsData.template(template,tr);
                else if(/\.csv$/i.test(target)){doc=await window.SheetsData.importCSV(state,target);if(!doc)throw new DOMException('Cancelled','AbortError');path=target.replace(/\.csv$/i,'')+'-import-'+crypto.randomUUID().slice(0,6)+'.xlsx';}
                else {const response=await responseOK(await fetch(endpoint(target),{signal:documentLife.signal,cache:'no-store'}));etag=response.headers.get('ETag');doc=await response.json();sourceData=doc.source_data||null;}
                if(disposed||token!==generation)return;
                if(doc?.schema_version!==2||!doc.workbook?.sheetOrder?.length)throw Error(tr('import_failed'));
                const recovery=await OfficeSession.draft('get',draftKey(),undefined,'sheets').catch(()=>null);
                let restored=false;
                if(recovery&&!ctx.readonly&&await ctx.confirmDialog(tr('restore_draft'))){
                    restored=true;
                    doc.workbook=recovery.snapshot.workbook;doc.charts=recovery.snapshot.charts;operations=recovery.snapshot.operations||[];lastDraft=recovery;
                    if(recovery.etag!==etag&&recovery.etag){recoveryConflict=true;sourceData=recovery.sourceData||null;notice(tr('draft_conflict'),true);}
                    else sourceData=recovery.sourceData||sourceData;
                }
                if(disposed||token!==generation)return;
                aux={charts:doc.charts||[],print:doc.workbook.custom?.auragoPrint||{}};
                structuralLocked=!!doc.structural_locked;
                const localeName=({de:'de-DE',es:'es-ES',fr:'fr-FR',it:'it-IT',ja:'ja-JP',pl:'pl-PL',pt:'pt-BR',zh:'zh-CN'})[locale.split('-')[0]]||'en-US',localeID=localeName.replace('-','');
                const created=lib.createUniver({locale:localeID,locales:{[localeID]:localeName==='en-US'&&!locale.startsWith('en')?SheetsData.nativeLocale(lib.locales[localeName],tr,locale):lib.locales[localeName]},logLevel:4,presets:[
                    lib.UniverSheetsCorePreset({container:mount,header:false,toolbar:false,formulaBar:false,footer:false,contextMenu:!ctx.showContextMenu,disableAutoFocus:true,workerURL:'/js/vendor/sheets/worker.js',customFontFamily:['Geist','Carlito'],formula:{initialFormulaComputing:0,function:[lib.averageAlias]}}),
                    lib.UniverSheetsFilterPreset(),lib.UniverSheetsSortPreset(),lib.UniverSheetsDataValidationPreset(),lib.UniverSheetsConditionalFormattingPreset(),lib.UniverSheetsFindReplacePreset(),lib.UniverSheetsNotePreset(),lib.UniverSheetsHyperLinkPreset(),lib.UniverSheetsTablePreset()
                ]});
                engine=created.univer;api=created.univerAPI;api.toggleDarkMode(false);api.getFormula().setFormulaReturnDependencyTree(true);
                await Promise.all(['11px Arial','bold 11px Arial','italic 11px Arial','11px Calibri','11px Carlito','11px "Times New Roman"','11px "Courier New"'].map(font=>document.fonts.load(font)));
                if(disposed||token!==generation)return;
                bridge=lib.installSheetActions(engine,{book:()=>book,get:()=>aux,set:value=>{aux=value;chartView?.refresh();},parse:(cell,context)=>{
                    if(!nativeEdit||nativeEdit.row!==context.row||nativeEdit.column!==context.col)return cell;
                    const value=SheetsData.parseInput(nativeEdit.text,locale);if(value.t===1)value.t=4;nativeEdit=null;
                    return {...cell,...value,...(value.s?{s:{...book.getSheetBySheetId(context.subUnitId).getRange(context.row,context.col).getCellStyleData(),...value.s}}:{}),...(!value.f?{f:null,si:null}:{}),p:null};
                }});
                doc.workbook.id='sheets-'+windowId;doc.workbook.locale=localeID;
                book=api.createWorkbook(doc.workbook);book.setEditable(!ctx.readonly);book.setNumfmtLocal(locale.split('-')[0]==='zh'?'zh_CN':locale.replace('-','_'));
                panels=window.SheetsPanels.create(state);chartView=window.SheetsCharts.create(state);
                loading=false;makeQueue();
                disposables.push(api.addEvent(api.Event.BeforeSheetEditEnd,event=>{nativeEdit=event.isConfirm?{row:event.row,column:event.column,text:event.value.toPlainText().replace(/\r?\n$/,'')}:null;}));
                const structureTypes={'sheet.mutation.insert-row':'insertRows','sheet.mutation.remove-rows':'deleteRows','sheet.mutation.insert-col':'insertColumns','sheet.mutation.remove-col':'deleteColumns'};
                disposables.push(api.addEvent(api.Event.BeforeCommandExecute,event=>{
                    if(structuralLocked&&(/sheet\.(command|mutation)\.(insert-row|insert-col|remove-row|remove-col|set-worksheet-name|remove-sheet|insert-sheet|move-range|move-rows|move-cols)/.test(event.id))){event.cancel=true;notice(tr('structure_locked'),true);}
                }));
                disposables.push(api.addEvent(api.Event.CommandExecuted,event=>{
                    if(loading||disposed)return;
                    if(event.type===2&&!event.options?.applyFormulaCalculationResult&&!event.options?.onlyLocal&&!/^(formula|doc)\.|sheet\.mutation\.(set-formula|set-array-formula)/.test(event.id)){
                        const kind=structureTypes[event.id],p=event.params;
                        if(kind&&p?.range){const rows=/Rows$/.test(kind);operations.push({type:kind,sheet:p.subUnitId,index:rows?p.range.startRow:p.range.startColumn,count:rows?p.range.endRow-p.range.startRow+1:p.range.endColumn-p.range.startColumn+1});}
                        queue.changed();scheduleBackup();
                    }
                    refreshSoon();
                }));
                disposables.push(api.addEvent(api.Event.SelectionChanged,()=>{pin=null;if(!formulaChanged)formulaTarget=null;refreshSoon();}));
                disposables.push(api.addEvent(api.Event.Scroll,()=>chartView?.position()));
                disposables.push(api.addEvent(api.Event.LifeCycleChanged,()=>refreshSoon()));
                disposables.push(api.getFormula().calculationEnd(()=>{refreshSoon();chartView.refresh();}));
                if(template||restored||/\.csv$/i.test(target)){queue.changed();scheduleBackup();}
                find('[data-name]').textContent=basename(path);find('[data-location]').textContent=path;find('[data-loading]').hidden=true;
                ctx.updateWindowContext?.(windowId,{path});refresh();renderMenus();
                if(doc.limitations?.length)notice(doc.limitations.map(key=>tr(key)).join(' · '));
            }catch(error){if(token!==generation||disposed)return;loading=true;loadingFailed=true;find('[data-loading]').textContent=tr('load_failed');fail(error);}
        }
        function refreshSoon(){clearTimeout(refreshTimer);refreshTimer=setTimeout(refresh,40);}
        function refresh(){
            if(!book||disposed||loading)return;
            const active=sheet(),r=selection(),range=active.getRange(r);
            if(document.activeElement!==find('[data-address]'))find('[data-address]').value=range.getA1Notation();
            if(!formulaChanged&&document.activeElement!==find('[data-formula]'))find('[data-formula]').value=range.getFormula()||String(range.getRawValue()??'');
            const style=range.getCellStyleData();
            for(const [key,val]of Object.entries({bold:style?.bl===1,italic:style?.it===1,underline:style?.ul?.s===1,left:style?.ht===1,center:style?.ht===2,right:style?.ht===3}))find('[data-action="'+key+'"]').setAttribute('aria-pressed',String(val));
            for(const [key,value]of Object.entries({font:style?.ff||'Arial',size:style?.fs||11,number:style?.n?.pattern||'General'})){const input=find('[data-format="'+key+'"]');if(document.activeElement!==input)input.value=value;}
            const rows=r.endRow-r.startRow+1,cols=r.endColumn-r.startColumn+1;
            find('[data-selection]').textContent=tr('selection_size',{rows,columns:cols});
            const values=rows*cols<=200000?range.getRawValues().flat():Object.entries(active.getSheet().getSnapshot().cellData||{}).filter(([row])=>+row>=r.startRow&&+row<=r.endRow).flatMap(([,cells])=>Object.entries(cells).filter(([col])=>+col>=r.startColumn&&+col<=r.endColumn).map(([,cell])=>cell?.v)),numbers=values.filter(v=>typeof v==='number'&&Number.isFinite(v)),nf=new Intl.NumberFormat(locale,{maximumFractionDigits:6});
            find('[data-stats]').textContent=(numbers.length?tr('status_sum')+' '+nf.format(numbers.reduce((a,b)=>a+b,0))+'   '+tr('status_avg')+' '+nf.format(numbers.reduce((a,b)=>a+b,0)/numbers.length)+'   ':'')+tr('status_count')+' '+nf.format(values.filter(v=>v!==null&&v!==undefined&&v!=='').length);
            find('[data-zoom]').textContent=Math.round(active.getZoom()*100)+'%';
            const tabs=book.getSheets().map(s=>({id:s.getSheetId(),name:s.getSheetName()})),signature=JSON.stringify([tabs,active.getSheetId()]);
            if(find('[data-tabs]').dataset.signature!==signature){find('[data-tabs]').dataset.signature=signature;find('[data-tabs]').innerHTML=tabs.map(s=>'<button type="button" role="tab" data-sheet="'+esc(s.id)+'" aria-selected="'+(s.id===active.getSheetId())+'">'+esc(s.name)+'</button>').join('');}
            panels?.refresh();chartView?.refresh();
        }
        function commitFormula(){
            if(!formulaChanged||!book||ctx.readonly)return;
            const target=formulaTarget||{sheet:sheet().getSheetId(),range:selection()},active=book.getSheetBySheetId(target.sheet);
            if(!active)return;
            active.getRange(target.range.startRow,target.range.startColumn).setValue(window.SheetsData.parseInput(find('[data-formula]').value,locale));
            formulaChanged=false;formulaTarget=null;find('[data-formula-actions]').hidden=true;find('[data-formula-hints]').hidden=true;
        }
        function cancelFormula(){formulaChanged=false;formulaTarget=null;find('[data-formula-actions]').hidden=true;find('[data-formula-hints]').hidden=true;find('[data-formula]').blur();refresh();}
        async function save(){if(book)await book.endEditingAsync(true);commitFormula();if(queue)await queue.save();}
        async function saveAs(){
            if(!book||ctx.readonly)return;
            await book.endEditingAsync(true);commitFormula();queue.suspend();
            try{
                if(queue.pending)await queue.pending.catch(()=>{});
                const picked=await ctx.saveFileDialog({title:tr('save_as'),initialPath:path.slice(0,path.lastIndexOf('/')),defaultName:basename(path),filters:[{label:'Excel',extensions:['.xlsx','.xlsm']}]});if(!picked)return;
                const target=typeof picked==='string'?picked:picked.path;if(!target)return;
                const value=snapshot(),revision=queue.revision,saved=await write(value,target,target===path?etag:null);
                if(queue.revision!==revision){notice(tr('copy_saved_new_changes'));return;}
                path=target;etag=saved.etag;sourceData=saved.sourceData||sourceData;recoveryConflict=false;operations=[];queue.dispose();makeQueue();find('[data-name]').textContent=basename(path);find('[data-location]').textContent=path;ctx.updateWindowContext?.(windowId,{path});notice('');
            }finally{queue.resume();}
        }
        async function guard(){
            if(!queue||ctx.readonly)return true;await book.endEditingAsync(true);commitFormula();if(!queue.dirty&&!queue.pending)return true;
            try{await save();return true;}catch(error){fail(error);return !!await ctx.confirmDialog(tr('discard_changes'));}
        }
        async function prepareOutput(){
            await book.endEditingAsync(true);commitFormula();const revision=queue.revision;
            await new Promise((resolve,reject)=>{let done=false;const finish=error=>{if(done)return;done=true;clearTimeout(timer);event.dispose();error?reject(error):resolve();};const event=api.getFormula().calculationEnd(()=>finish());const timer=setTimeout(()=>finish(new Error(tr('calculation_timeout'))),10000);api.getFormula().executeCalculation();});
            if(queue.revision!==revision)throw Error(tr('snapshot_changed'));
            return snapshot();
        }
        async function download(format){
            if(!book)return;await book.endEditingAsync(true);commitFormula();
            if(format==='csv'&&!await ctx.confirmDialog(tr('csv_loss_warning')))return;
            const value=await prepareOutput(),response=await responseOK(await fetch('/api/desktop/office/export?kind=workbook&format='+format+'&sheet='+encodeURIComponent(sheet().getSheetName()),{method:'POST',headers:{'Content-Type':'application/json'},signal:documentLife.signal,body:JSON.stringify({...value,...(sourceData?{source_data:sourceData}:etag?{source_path:path,source_etag:etag}:{})})}));
            const url=URL.createObjectURL(await response.blob()),link=document.createElement('a');link.href=url;link.download=basename(path).replace(/\.[^.]+$/,'.'+format);link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
        }
        async function act(action){
            try{
                if(action==='dismiss'){notice('');return;}
                if(action==='document'){find('[data-document-menu]').hidden=!find('[data-document-menu]').hidden;return;}
                if(action==='retry'){if(loadingFailed)await load(path);else await save();return;}
                if(action==='save'){await save();return;}if(action==='saveAs'||action==='rename'){await saveAs();return;}
                if(action==='new'){if(await guard())await load(newPath(),'blank');return;}
                if(action==='open'){if(!await guard())return;const result=await ctx.openFileDialog({title:tr('open'),filters:[{label:'Excel / CSV',extensions:['.xlsx','.xlsm','.csv']}]});if(result)await load(typeof result==='string'?result:result.path);return;}
                if(!book||loading)return;
                if(action==='chartSelected'){rightPanel='chart';panels.showRight('chart');return;}
                if(['navigation','search','format','data','chart','assist','function','more','borders','fill','sheetMenu'].includes(action)){
                    pinSelection();const left=['navigation','search'].includes(action),key=({more:'format',borders:'format',fill:'format',function:'function',sheetMenu:'sheets'})[action]||action;
                    if(left){leftPanel=leftPanel===key?'':key;panels.showLeft(leftPanel);}else{rightPanel=rightPanel===key?'':key;panels.showRight(rightPanel);}return;
                }
                if(action==='closeLeft'){leftPanel='';panels.showLeft('');return;}if(action==='closeRight'){rightPanel='';panels.showRight('');return;}
                if(action==='commitFormula'){commitFormula();refresh();return;}if(action==='cancelFormula'){cancelFormula();return;}
                if(action==='expandFormula'){find('[data-formula]').rows=find('[data-formula]').rows===1?4:1;return;}
                if(action==='xlsx'||action==='csv'){await download(action==='xlsx'&&/\.xlsm$/i.test(path)?'xlsm':action);return;}
                if(action==='print'){panels.showRight('print');rightPanel='print';return;}
                if(action.startsWith('zoom')){const zoom=action==='zoomReset'?1:Math.max(.25,Math.min(3,sheet().getZoom()+(action==='zoomIn'?.1:-.1)));sheet().zoom(zoom);refresh();return;}
                if(action==='copy')return await panels.action('copy');
                return await run(()=>{
                    const r=selected(),style=r.getCellStyleData();
                    if(action==='undo')return api.undo();if(action==='redo')return api.redo();
                    if(action==='bold')return state.format('bl',style?.bl===1?0:1);if(action==='italic')return state.format('it',style?.it===1?0:1);if(action==='underline')return state.format('ul',{s:style?.ul?.s===1?0:1});
                    if(['left','center','right'].includes(action))return state.format('ht',{left:1,center:2,right:3}[action]);
                    if(action==='merge')return r.merge({isForceMerge:true});
                    if(action==='addSheet'){const added=book.insertSheet();book.setActiveSheet(added);return;}
                    return panels.action(action);
                });
            }catch(error){fail(error);}
        }
        function renderMenus(){
            const item=(label,action,icon,shortcut)=>({id:action,label:tr(label),icon:icons[icon||action]||icon||action,shortcut,action:()=>act(action)});
            ctx.setWindowMenus?.(windowId,[
                {id:'file',labelKey:'desktop.menu_file',items:[item('new','new','new','Ctrl+N'),item('open','open','open','Ctrl+O'),item('save','save','save','Ctrl+S'),item('save_as','saveAs','saveAs'),{type:'separator'},item('download_xlsx','xlsx','file'),item('export_csv','csv','file-text'),item('print','print','print','Ctrl+P')]},
                {id:'edit',labelKey:'desktop.menu_edit',items:[item('undo','undo','undo','Ctrl+Z'),item('redo','redo','redo','Ctrl+Y'),{type:'separator'},item('find','search','search','Ctrl+F'),item('copy','copy','copy'),item('cut','cut','scissors'),item('paste','paste','clipboard'),item('paste_special','pasteSpecial','clipboard')]},
                {id:'agent',labelKey:'desktop.menu_agent',items:[item('assist','assist')]},
                {id:'view',label:tr('view'),items:[item('navigation','navigation'),item('format','format'),item('data','data'),item('chart','chart'),item('assist','assist')]}
            ]);
        }
        root.addEventListener('pointerdown',event=>{if(!event.target.closest('.sheets-engine,.sheets-charts'))pinSelection();const btn=event.target.closest('.sheets-toolbar button,.sheets-views button');if(btn)event.preventDefault();},{signal:lifecycle.signal});
        root.addEventListener('click',event=>{const tab=event.target.closest('[data-sheet]');if(tab&&book){commitFormula();pin=null;book.setActiveSheet(tab.dataset.sheet);refresh();return;}const target=event.target.closest('[data-action]');if(target)act(target.dataset.action);},{signal:lifecycle.signal});
        mount.addEventListener('contextmenu',event=>{
            if(!book||!ctx.showContextMenu)return;
            event.preventDefault();event.stopPropagation();pinSelection();
            const item=(key,icon)=>({label:tr(key),icon,disabled:!!ctx.readonly&&key!=='copy',action:()=>act(key)});
            ctx.showContextMenu(event.clientX,event.clientY,[item('copy','copy'),item('cut','scissors'),item('paste','clipboard'),{label:tr('paste_special'),icon:'clipboard',items:['pasteValues','pasteFormulas','pasteFormat','pasteTranspose'].map(key=>item(key,'clipboard'))},{separator:true},item('insertRow','plus'),item('insertColumn','plus'),item('deleteRow','trash'),item('deleteColumn','trash'),{separator:true},item('format','sliders'),item('data','grid'),item('chart','analytics')]);
        },{signal:lifecycle.signal});
        root.addEventListener('change',event=>{const key=event.target.dataset.format;if(key)run(()=>{const value=event.target.value,r=selected();if(key==='font')return state.format('ff',value);if(key==='size'){const size=Number(value);if(size>=1&&size<=200)return state.format('fs',size);}if(key==='number')return state.format('n',{pattern:value});});},{signal:lifecycle.signal});
        find('[data-address]').addEventListener('keydown',event=>{if(event.key==='Enter'&&book){event.preventDefault();try{const r=sheet().getRange(event.target.value.trim().toUpperCase());pin=null;r.activate();sheet().scrollToCell(r.getRange().startRow,r.getRange().startColumn);event.target.blur();refresh();}catch(_){notice(tr('invalid_range'),true);}}},{signal:lifecycle.signal});
        find('[data-formula]').addEventListener('input',()=>{if(!book||ctx.readonly)return;if(!formulaTarget)formulaTarget={sheet:sheet().getSheetId(),range:selection()};formulaChanged=true;find('[data-formula-actions]').hidden=false;panels.formulaHints(find('[data-formula]').value);},{signal:lifecycle.signal});
        find('[data-formula]').addEventListener('keydown',event=>{if(event.key==='Enter'&&!event.shiftKey){event.preventDefault();commitFormula();event.target.blur();refresh();}if(event.key==='Escape'){event.preventDefault();cancelFormula();}},{signal:lifecycle.signal});
        root.addEventListener('keydown',event=>{
            if(!(event.ctrlKey||event.metaKey)||event.altKey)return;
            const action={s:event.shiftKey?'saveAs':'save',o:'open',n:'new',p:'print',f:'search'}[event.key.toLowerCase()];if(action){event.preventDefault();event.stopPropagation();act(action);}
        },{signal:lifecycle.signal});
        if(ctx.readonly){find('[data-formula]').readOnly=true;root.querySelectorAll('.sheets-toolbar button,.sheets-toolbar input,.sheets-toolbar select,[data-action="addSheet"]').forEach(control=>control.disabled=true);}
        const resize=new ResizeObserver(()=>{root.dataset.narrow=String(root.clientWidth<900);chartView?.position();});resize.observe(root);
        window.addEventListener('beforeunload',event=>{if(queue?.dirty){event.preventDefault();event.returnValue='';}},{signal:lifecycle.signal});
        function releaseDocument(){
            documentLife.abort();queue?.dispose();queue=null;panels?.dispose();chartView?.dispose();panels=chartView=null;
            for(const item of disposables.splice(0))item?.dispose();engine?.dispose();engine=api=book=null;
            clearTimeout(refreshTimer);clearTimeout(backupTimer);clearTimeout(backupDeadline);backupTimer=backupDeadline=null;
        }
        function cleanup(){if(disposed)return;disposed=true;generation++;releaseDocument();lifecycle.abort();resize.disconnect();instances.delete(windowId);}
        renderMenus();load(path,ctx.path?null:'blank');
    }
    function dispose(id){instances.get(id)?.dispose();}
    window.SheetsApp={render,dispose,instances};
})();
