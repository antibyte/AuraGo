(function(){
    'use strict';
    function create(state){
        const {esc,tr,root,find}=state,life=new AbortController();let left='',right='',matches=[],matchIndex=-1,suggestion=null,assistLife=null,highlights=[];
        const field=(label,name,type='text',value='')=>'<label class="sheets-field"><span>'+esc(tr(label))+'</span><input data-field="'+name+'" type="'+type+'" value="'+esc(value)+'"></label>';
        const select=(label,name,options,value)=>'<label class="sheets-field"><span>'+esc(tr(label))+'</span><select data-field="'+name+'">'+options.map(([v,k])=>'<option value="'+esc(v)+'"'+(v===value?' selected':'')+'>'+esc(tr(k))+'</option>').join('')+'</select></label>';
        const button=(key,label=key)=>'<button type="button" data-panel-action="'+key+'">'+esc(tr(label))+'</button>';
        const buttons=items=>'<div class="sheets-buttonrow">'+items.map(x=>Array.isArray(x)?button(...x):button(x)).join('')+'</div>';
        const section=(name,body)=>'<section class="sheets-panel-section"><h3>'+esc(tr(name))+'</h3>'+body+'</section>';
        const value=name=>root.querySelector('[data-field="'+name+'"]')?.value||'';
        const checked=name=>!!root.querySelector('[data-field="'+name+'"]')?.checked;
        const sheet=()=>state.book.getActiveSheet();
        function container(side,key,html){
            const panel=find('[data-'+side+']');panel.hidden=!key;
            panel.innerHTML=key?'<header class="sheets-panel-heading"><h2>'+esc(tr(key))+'</h2><button type="button" data-action="'+(side==='left'?'closeLeft':'closeRight')+'" aria-label="'+esc(tr('close'))+'">×</button></header><div class="sheets-panel-content">'+html+'</div>':'';
            if(state.readonly){const allowed=new Set(['searchRun','searchPrevious','searchNext','askAI','cancelAI','printNow']);panel.querySelectorAll('[data-panel-action]').forEach(x=>x.disabled=!allowed.has(x.dataset.panelAction));if(['format','data','chart','sheets'].includes(key))panel.querySelectorAll('input,select,textarea').forEach(x=>x.disabled=true);}
        }
        function showLeft(key){
            left=key;
            if(key==='search')container('left',key,field('find','query')+field('replace','replacement')+select('search_in','searchIn',[['values','values'],['formulas','formulas']],'values')+select('scope','scope',[['sheet','current_sheet'],['book','entire_workbook']],'sheet')+'<div class="sheets-checks"><label><input type="checkbox" data-field="case">'+esc(tr('match_case'))+'</label><label><input type="checkbox" data-field="whole">'+esc(tr('whole_cell'))+'</label></div>'+buttons(['searchRun','searchPrevious','searchNext','replaceOne','replaceAll'])+'<p data-search-status class="sheets-hint"></p><div data-search-results></div>');
            else if(key==='navigation'){
                const sheets=state.book.getSheets().map(s=>'<button data-nav-sheet="'+esc(s.getSheetId())+'">'+esc(s.getSheetName())+'</button>').join('');
                const names=state.book.getDefinedNames().map(n=>'<button data-name-range="'+esc(n.getName())+'">'+esc(n.getName())+'</button>').join('');
                container('left',key,section('sheets','<div class="sheets-navigation">'+sheets+'</div>')+section('named_ranges','<div class="sheets-navigation">'+names+'</div>'+field('name','rangeName')+buttons(['nameRange']))+section('templates',buttons([['templateBlank','template_blank'],['templateBudget','template_budget'],['templateProject','template_project'],['templateInventory','template_inventory']])));
            }else container('left','','');
        }
        function showRight(key){
            right=key;
            if(key==='format'){
                const r=state.selected(),style=r.getCellStyleData()||{};
                container('right',key,'<div class="sheets-selection-pill">'+esc(r.getA1Notation())+'</div>'+section('text',
                    field('font_color','fontColor','color',style.cl?.rgb||'#26374b')+field('fill_color','fillColor','color',style.bg?.rgb||'#ffffff')+
                    select('vertical_alignment','vertical',[['top','align_top'],['middle','align_middle'],['bottom','align_bottom']],'middle')+
                    select('format_borders','border',[['all','border_all'],['outside','border_outside'],['bottom','border_bottom'],['none','border_none']],'all')+
                    buttons(['setBorder','wrap','unmerge','clearFormat']))+
                    section('number_format',field('number_format','pattern','text',r.getNumberFormat()||'General')+buttons(['setNumberFormat','decimalLess','decimalMore']))+
                    section('dimensions',field('row_height','rowHeight','number','26')+field('column_width','columnWidth','number','112')+buttons(['setRowHeight','setColumnWidth','autoFitRows','autoFitColumns']))+
                    section('conditional_format',select('condition','cfCondition',[['greater','greater_than'],['less','less_than'],['equal','equal_to'],['contains','contains']],'greater')+field('value','cfValue','text','0')+field('fill_color','cfColor','color','#dcefe6')+buttons(['addCondition','clearConditions']))+
                    section('notes','<textarea data-field="note" rows="4" aria-label="'+esc(tr('notes'))+'">'+esc(r.getNote()?.note||'')+'</textarea>'+buttons(['setNote','deleteNote'])));
            }else if(key==='data'){
                const selected=state.selection(),columns=Array.from({length:Math.min(100,selected.endColumn-selected.startColumn+1)},(_,i)=>[String(selected.startColumn+i),SheetsData.columnName(selected.startColumn+i)]);
                container('right',key,section('sort',select('column','sortColumn',columns)+select('order','sortOrder',[['asc','ascending'],['desc','descending']],'asc')+select('then_by','sortSecond',[['','none'],...columns])+select('order','sortSecondOrder',[['asc','ascending'],['desc','descending']],'asc')+buttons(['sort']))+
                    section('filter',buttons(['toggleFilter'])+select('column','filterColumn',columns)+select('condition','filterKind',[['values','values'],['contains','contains'],['greaterThan','greater_than'],['lessThan','less_than'],['equal','equal_to'],['date','date']],'values')+field('value','filterValue')+buttons(['applyFilter','clearFilter']))+
                    section('validation',select('validation_type','validationType',[['list','dropdown_list'],['number','format_number'],['date','format_date']],'list')+field('allowed_values','validationValues','text','Yes,No')+field('minimum','validationMin','text','0')+field('maximum','validationMax','text','100')+buttons(['validate','clearValidation']))+
                    section('layout',buttons(['insertRow','deleteRow','insertColumn','deleteColumn','hideRows','showRows','hideColumns','showColumns','freeze','unfreeze']))+
                    section('paste_special',buttons(['pasteValues','pasteFormulas','pasteFormat','pasteTranspose'])));
            }else if(key==='chart'){
                const c=state.charts.selected;
                container('right',key,section('create_chart',select('chart_type','newChartType',[['column','chart_column'],['bar','chart_bar'],['line','chart_line'],['pie','chart_pie'],['scatter','chart_scatter']],'column')+buttons(['addChart'])+'<p class="sheets-hint">'+esc(tr('chart_select_range'))+'</p>')+
                    (c?section('chart_settings',c.readonly?'<p>'+esc(tr('chart_preserved'))+'</p>':field('title','chartTitle','text',c.title)+field('data_range','chartRange','text',c.range||'')+select('chart_type','chartType',[['column','chart_column'],['bar','chart_bar'],['line','chart_line'],['pie','chart_pie'],['scatter','chart_scatter']],c.type)+field('x_axis','chartXTitle','text',c.x_title||'')+field('y_axis','chartYTitle','text',c.y_title||'')+'<label class="sheets-field"><span>'+esc(tr('legend'))+'</span><input data-field="chartLegend" type="checkbox"'+(c.legend?' checked':'')+'></label>'+field('color','chartColor','color',c.colors?.[0]||'#467caf')+buttons(['updateChart','removeChart'])):''));
            }else if(key==='assist'){
                container('right',key,select('action','assistAction',[['formula','ai_formula'],['explain','ai_explain'],['clean','ai_clean'],['continue','ai_continue'],['summarize','ai_summarize'],['custom','custom']],'formula')+'<label class="sheets-field"><span>'+esc(tr('instruction'))+'</span><textarea data-field="instruction" rows="4"></textarea></label>'+buttons(['askAI','cancelAI'])+'<p class="sheets-hint">'+esc(tr('ai_selection_hint'))+'</p><div data-ai-result></div>');
                if(suggestion)renderSuggestion();
            }else if(key==='function'){
                container('right',key,field('function_search','functionQuery')+'<div data-functions class="sheets-functions"></div><p class="sheets-hint">'+esc(tr('english_formulas'))+'</p>');listFunctions('');
            }else if(key==='sheets'){
                container('right',key,section('sheet_options',field('name','sheetName','text',sheet().getSheetName())+buttons(['renameSheet','duplicateSheet','moveSheetLeft','moveSheetRight','deleteSheet','showAllSheets']))+section('templates',buttons([['templateBlank','template_blank'],['templateBudget','template_budget'],['templateProject','template_project'],['templateInventory','template_inventory']])));
            }else if(key==='print'){
                const p=state.aux.print.sheet===sheet().getSheetId()?state.aux.print:{};
                container('right',key,field('print_area','printArea','text',p.area||state.selected().getA1Notation())+select('orientation','orientation',[['portrait','portrait'],['landscape','landscape']],p.orientation||'portrait')+select('paper_size','paper',[['A4','A4'],['letter','Letter']],p.paper||'A4')+select('scaling','scaling',[['fit','fit_width'],['actual','actual_size']],p.scaling||'fit')+field('repeat_rows','repeatRows','number',p.repeatRows||0)+buttons(['printNow'])+'<p class="sheets-hint">'+esc(tr('print_hint'))+'</p>');
            }else container('right','','');
        }
        const functionInfo=state.lib.locales['en-US']['sheets-formula'].functionList;
        const functions=Object.fromEntries(Object.entries(functionInfo).map(([name,info])=>[name,Object.values(info.functionParameter||{}).map(p=>p.name).join(', ')]));
        functions.AVG=functions.AVERAGE;
        function listFunctions(query){const host=root.querySelector('[data-functions]');if(host)host.innerHTML=Object.keys(functions).filter(name=>name.includes(query.toUpperCase())).map(name=>'<button data-function="'+name+'"><strong>'+name+'</strong><small>'+esc(functions[name])+'</small></button>').join('');}
        function clearHighlights(){for(const h of highlights)h.dispose();highlights=[];}
        function formulaHints(text){
            clearHighlights();const host=find('[data-formula-hints]');host.hidden=!text.startsWith('=');if(host.hidden)return;
            const prefix=/([A-Z][A-Z0-9_.]*)$/i.exec(text)?.[1]||'',called=/([A-Z][A-Z0-9_.]*)\([^()]*$/i.exec(text)?.[1]?.toUpperCase(),suggestions=prefix?Object.keys(functions).filter(name=>name.startsWith(prefix.toUpperCase())).slice(0,6):[];
            host.innerHTML=suggestions.map(name=>'<button data-function="'+name+'">'+name+'</button>').join('')+(called&&functions[called]!==undefined?'<span>'+called+'('+esc(functions[called])+')</span>':'');
            const refs=text.replace(/"(?:[^"]|"")*"/g,'').match(/\$?[A-Z]{1,3}\$?\d+(?::\$?[A-Z]{1,3}\$?\d+)?/g)||[];
            refs.slice(0,8).forEach((ref,i)=>{try{highlights.push(sheet().highlightRanges([sheet().getRange(ref.replaceAll('$',''))],{stroke:['#5089d6','#d78754','#8e67be','#52a18a'][i%4],fill:'transparent'}));}catch(_){}});
        }
        function search(){
            const query=value('query');matches=[];matchIndex=-1;if(!query)return renderMatches();
            const sensitive=checked('case'),whole=checked('whole'),needle=sensitive?query:query.toLocaleLowerCase(state.locale),book=state.book.save(),ids=value('scope')==='book'?book.sheetOrder:[sheet().getSheetId()];
            searchRows: for(const id of ids)for(const [row,cols]of Object.entries(book.sheets[id].cellData))for(const [column,cell]of Object.entries(cols)){
                if(!cell)continue;const range=state.book.getSheetBySheetId(id).getRange(+row,+column),source=String(value('searchIn')==='formulas'?(range.getFormula()||''):(range.getValue()??'')),text=sensitive?source:source.toLocaleLowerCase(state.locale);
                if(whole?text===needle:text.includes(needle)){matches.push({sheet:id,row:+row,column:+column,text:source});if(matches.length>=10000)break searchRows;}
            }
            if(matches.length){matchIndex=0;goMatch();}renderMatches();
        }
        function goMatch(){const m=matches[matchIndex];if(!m)return;const active=state.book.setActiveSheet(m.sheet);active.getRange(m.row,m.column).activate();active.scrollToCell(m.row,m.column);state.refresh();}
        function renderMatches(){
            const status=root.querySelector('[data-search-status]');if(status)status.textContent=matches.length?tr('match_count',{current:matchIndex+1,total:matches.length}):tr('no_matches');
            const list=root.querySelector('[data-search-results]');if(list)list.innerHTML=matches.slice(0,200).map((m,i)=>'<button data-match="'+i+'"><b>'+SheetsData.columnName(m.column)+(m.row+1)+'</b><span>'+esc(m.text)+'</span></button>').join('');
        }
        async function replaceMatches(all){
            if(state.readonly)return;
            const targets=all?matches:[matches[matchIndex]].filter(Boolean),regex=new RegExp(value('query').replace(/[.*+?^${}()|[\]\\]/g,'\\$&'),checked('case')?'g':'gi');
            const grouped=new Map();for(const m of targets){const active=state.book.getSheetBySheetId(m.sheet),r=active.getRange(m.row,m.column),cell=structuredClone(r.getCellData()||{});
                if(value('searchIn')==='formulas'&&cell.f)cell.f=cell.f.replace(regex,()=>value('replacement'));else {cell.v=String(r.getRawValue()??'').replace(regex,()=>value('replacement'));cell.t=1;delete cell.f;delete cell.si;}
                if(!grouped.has(m.sheet))grouped.set(m.sheet,{});const values=grouped.get(m.sheet);(values[m.row]??={})[m.column]=cell;
            }
            for(const [id,values]of grouped)await state.api.executeCommand('sheet.command.set-range-values',{unitId:state.book.getId(),subUnitId:id,value:values});
            search();
        }
        async function askAI(){
            assistLife?.abort();assistLife=new AbortController();const controller=assistLife;state.signal.addEventListener('abort',()=>controller.abort(),{once:true});
            state.commitFormula();const selection=state.selection(),rows=selection.endRow-selection.startRow+1,cols=selection.endColumn-selection.startColumn+1;
            if(rows*cols>2000)throw Error(tr('ai_selection_limit'));
            const range=state.selected(),values=range.getRawValues(),formulas=range.getFormulas(),cells=[];
            values.forEach((row,r)=>row.forEach((v,c)=>cells.push({row:r+selection.startRow,column:c+selection.startColumn,value:v,formula:formulas[r]?.[c]||''})));
            const revision=state.revision,target=sheet().getSheetId(),action=value('assistAction');
            const host=root.querySelector('[data-ai-result]');host.textContent=tr('generating');
            const response=await fetch('/api/desktop/office/workbook/assist',{method:'POST',signal:controller.signal,headers:{'Content-Type':'application/json'},body:JSON.stringify({action,instruction:value('instruction'),context:sheet().getSheetName(),range:selection,cells,source_revision:revision})});
            const data=await response.json();if(!response.ok)throw Error(data.error||tr('assist_failed'));
            if(!Array.isArray(data.changes)||data.source_revision!==revision)throw Error(tr('assist_failed'));
            for(const c of data.changes)if(!Number.isInteger(c.row)||!Number.isInteger(c.column)||c.row<selection.startRow||c.row>selection.endRow||c.column<selection.startColumn||c.column>selection.endColumn)throw Error(tr('assist_failed'));
            suggestion={...data,selection,target,original:cells};renderSuggestion();
        }
        function renderSuggestion(){
            const host=root.querySelector('[data-ai-result]');if(!host||!suggestion)return;
            host.innerHTML='<p class="sheets-ai-explanation"></p><div class="sheets-ai-changes">'+suggestion.changes.map(c=>'<div><b>'+SheetsData.columnName(c.column)+(c.row+1)+'</b><code>'+esc(suggestion.original.find(v=>v.row===c.row&&v.column===c.column)?.formula||String(suggestion.original.find(v=>v.row===c.row&&v.column===c.column)?.value??''))+'</code><span aria-hidden="true">→</span><code>'+esc(c.formula||String(c.value??''))+'</code></div>').join('')+'</div>'+buttons(['applyAI']);
            host.firstElementChild.textContent=suggestion.explanation||'';
            const apply=host.querySelector('[data-panel-action="applyAI"]');apply.disabled=state.readonly||state.revision!==suggestion.source_revision||!suggestion.changes.length;
            if(state.revision!==suggestion.source_revision)host.insertAdjacentHTML('beforeend','<p class="sheets-hint">'+esc(tr('suggestion_stale'))+'</p>');
        }
        function applyAI(){
            if(!suggestion||state.readonly||state.revision!==suggestion.source_revision)throw Error(tr('suggestion_stale'));
            const r=suggestion.selection,active=state.book.getSheetBySheetId(suggestion.target),range=active.getRange(r),matrix=structuredClone(range.getCellDatas());
            for(const change of suggestion.changes){const row=change.row-r.startRow,column=change.column-r.startColumn,cell=matrix[row][column]||{};delete cell.si;delete cell.p;delete cell.f;cell.v=change.value??null;cell.t=typeof cell.v==='number'?2:typeof cell.v==='boolean'?3:1;if(change.formula){cell.f=change.formula;cell.v=null;cell.t=2;}matrix[row][column]=cell;}
            range.setValues(matrix);suggestion=null;const host=root.querySelector('[data-ai-result]');if(host)host.textContent=tr('applied');
        }
        let clipboard=null;
        async function copy(cut=false){const range=state.selected();range.activate();const cells=structuredClone(range.getCellDatas()),formulas=range.getFormulas();cells.forEach((row,r)=>row.forEach((cell,c)=>{if(cell?.si){cell.f=formulas[r][c];delete cell.si;}}));await state.clipboard[cut?'cut':'copy']();clipboard={cells,source:range.getRange(),sheet:sheet().getSheetId(),cut,text:await navigator.clipboard.readText()};}
        async function paste(kind='all'){
            const start=state.selection(),active=sheet();state.selected().activate();
            if(kind!=='transpose'){
                const type=({values:'special-paste-value',formulas:'special-paste-formula',format:'special-paste-format'})[kind]||'default-paste';
                const entries=await navigator.clipboard.read();if(entries.length){await state.clipboard.paste(entries[0],type);return;}
                const id=state.clipboard.copyContentCache().getLastCopyId();if(id){await state.clipboard.pasteByCopyId(id,type);return;}return;
            }
            const text=await navigator.clipboard.readText(),internal=clipboard&&clipboard.text===text;
            if(internal&&clipboard.cut)throw Error(tr('transpose_copy_first'));
            const source=internal?structuredClone(clipboard.cells):SheetsData.parseCSV(text,'\t').map(row=>row.map(v=>SheetsData.parseInput(v,state.locale)));
            const width=source.reduce((n,row)=>Math.max(n,row.length),0);if(!width||!source.length)return;
            const cells=Array.from({length:width},(_,r)=>source.map((row,c)=>{
                const cell=row[r]||{};
                if(internal&&cell.f)cell.f=state.api.getFormula().moveFormulaRefOffset(cell.f,start.startColumn+c-clipboard.source.startColumn-r,start.startRow+r-clipboard.source.startRow-c);
                delete cell.si;return cell;
            }));
            active.getRange(start.startRow,start.startColumn,cells.length,cells[0].length).setValues(cells);
        }

        async function action(key){
            const r=state.selected(),range=state.selection(),active=sheet(),rows=range.endRow-range.startRow+1,cols=range.endColumn-range.startColumn+1;
            if(key==='searchRun'){search();return;}if(key==='searchNext'||key==='searchPrevious'){if(matches.length){matchIndex=(matchIndex+(key==='searchNext'?1:matches.length-1))%matches.length;goMatch();renderMatches();}return;}
            if(key==='copy'){await copy();return;}if(key==='pasteSpecial'){showRight('data');return;}
            if(key==='askAI'){await askAI();return;}if(key==='cancelAI'){assistLife?.abort();return;}
            if(key==='printNow'){await print();return;}
            if(state.readonly)return;
            if(key==='replaceOne'||key==='replaceAll'){await replaceMatches(key==='replaceAll');return;}
            if(key==='cut'){await copy(true);return;}
            if(key==='paste'||key.startsWith('paste')){await paste(({pasteValues:'values',pasteFormulas:'formulas',pasteFormat:'format',pasteTranspose:'transpose'})[key]||'all');return;}
            if(key==='setBorder')r.setBorder(state.api.Enum.BorderType[({all:'ALL',outside:'OUTSIDE',bottom:'BOTTOM',none:'NONE'})[value('border')]],state.api.Enum.BorderStyleTypes.THIN,'#aab8c8');
            if(key==='wrap')r.setWrap(true);if(key==='unmerge')r.breakApart();if(key==='clearFormat')r.clear({formatOnly:true});
            if(key==='setNumberFormat')r.setNumberFormat(value('pattern'));
            if(key==='decimalMore'||key==='decimalLess'){const current=r.getNumberFormat()||'0',digits=(/\.([0#]+)/.exec(current)?.[1].length||0)+(key==='decimalMore'?1:-1);r.setNumberFormat('0'+(digits>0?'.'+'0'.repeat(Math.min(12,digits)):'')+(current.includes('%')?'%':''));}
            if(key==='setRowHeight')active.setRowHeights(range.startRow,rows,Math.max(10,Math.min(546,Number(value('rowHeight'))||26)));
            if(key==='setColumnWidth')active.setColumnWidths(range.startColumn,cols,Math.max(20,Math.min(1790,Number(value('columnWidth'))||112)));
            if(key==='autoFitRows')active.setRowAutoHeight(range.startRow,rows);if(key==='autoFitColumns')active.setColumnAutoWidth(range.startColumn,cols);
            if(key==='setNote')r.createOrUpdateNote({note:value('note'),width:220,height:140,show:false});if(key==='deleteNote')r.deleteNote();
            if(key==='addCondition'){const builder=r.createConditionalFormattingRule(),kind=value('cfCondition'),v=value('cfValue');const rule=(kind==='contains'?builder.whenTextContains(v):kind==='less'?builder.whenNumberLessThan(Number(v)):kind==='equal'?builder.whenNumberEqualTo(Number(v)):builder.whenNumberGreaterThan(Number(v))).setBackground(value('cfColor')).build();active.addConditionalFormattingRule(rule);}
            if(key==='clearConditions')r.clearConditionalFormatRules();
            if(key==='sort'){const first={column:Number(value('sortColumn'))-range.startColumn,ascending:value('sortOrder')==='asc'},second=value('sortSecond');r.sort(second!==''?[first,{column:Number(second)-range.startColumn,ascending:value('sortSecondOrder')==='asc'}]:[first]);}
            if(key==='toggleFilter'){const filter=r.getFilter();if(filter)filter.remove();else r.createFilter();}
            if(key==='clearFilter')r.getFilter()?.remove();
            if(key==='applyFilter'){
                const filter=r.getFilter()||r.createFilter(),column=Number(value('filterColumn')),kind=value('filterKind'),text=value('filterValue');
                if(!filter)throw Error(tr('invalid_range'));
                let criteria;if(kind==='values')criteria={filters:{filters:text.split(',').map(s=>s.trim())}};else if(kind==='date'){const parsed=SheetsData.parseInput(text,state.locale);criteria={customFilters:{customFilters:[{operator:'equal',val:parsed.v}]}};}else criteria={customFilters:{customFilters:[{operator:kind==='contains'?'equal':kind,val:kind==='contains'?'*'+text+'*':SheetsData.parseInput(text,state.locale).v}]}};
                filter.setColumnFilterCriteria(column,{colId:column-range.startColumn,...criteria});
            }
            if(key==='validate'){
                const builder=state.api.newDataValidation(),kind=value('validationType');
                if(kind==='list')builder.requireValueInList(value('validationValues').split(',').map(v=>v.trim()));
                else {const min=SheetsData.parseInput(value('validationMin'),state.locale).v,max=SheetsData.parseInput(value('validationMax'),state.locale).v;if(typeof min!=='number'||typeof max!=='number'||min>max)throw Error(tr('validation')+': '+tr('minimum')+' ≤ '+tr('maximum'));
                    if(kind==='number')builder.requireNumberBetween(min,max);else {const date=n=>new Date((n-25569+(n<60?1:0))*86400000);builder.requireDateBetween(date(min),date(max));}}
                r.setDataValidation(builder.setAllowInvalid(false).build());
            }
            if(key==='clearValidation')r.setDataValidation(null);
            if(key==='insertRow')active.insertRows(range.startRow,rows);if(key==='deleteRow')active.deleteRows(range.startRow,rows);
            if(key==='insertColumn')active.insertColumns(range.startColumn,cols);if(key==='deleteColumn')active.deleteColumns(range.startColumn,cols);
            if(key==='hideRows')active.hideRows(range.startRow,rows);if(key==='showRows')active.showRows(range.startRow,rows);if(key==='hideColumns')active.hideColumns(range.startColumn,cols);if(key==='showColumns')active.showColumns(range.startColumn,cols);
            if(key==='freeze')active.setFreeze({xSplit:range.startColumn,ySplit:range.startRow,startColumn:range.startColumn,startRow:range.startRow});
            if(key==='unfreeze')active.setFreeze({xSplit:0,ySplit:0,startColumn:-1,startRow:-1});
            if(key==='renameSheet')active.setName(value('sheetName'));if(key==='duplicateSheet')state.book.setActiveSheet(state.book.duplicateSheet(active));
            if(key==='moveSheetLeft'||key==='moveSheetRight'){const list=state.book.getSheets(),index=list.findIndex(s=>s.getSheetId()===active.getSheetId());state.book.moveSheet(active,Math.max(0,Math.min(list.length-1,index+(key==='moveSheetLeft'?-1:1))));}
            if(key==='deleteSheet'&&state.book.getSheets().length>1&&await state.confirmKey('delete_sheet_confirm'))state.book.deleteSheet(active);
            if(key==='showAllSheets')state.book.getSheets().forEach(s=>s.showSheet());
            if(key==='nameRange'){const name=value('rangeName');if(!/^[A-Za-z_][A-Za-z0-9_.]*$/.test(name))throw Error(tr('invalid_name'));state.book.insertDefinedName(name,"'"+active.getSheetName().replaceAll("'","''")+"'!"+r.getA1Notation());showLeft('navigation');}
            if(key.startsWith('template')){await state.save();const kind=key.slice(8).toLowerCase();await state.load('Documents/'+kind+'-'+crypto.randomUUID().slice(0,6)+'.xlsx',kind);return;}
            if(key==='addChart'){state.charts.add(value('newChartType'));showRight('chart');}
            if(key==='updateChart'){state.charts.update({type:value('chartType'),title:value('chartTitle'),range:value('chartRange'),legend:checked('chartLegend'),x_title:value('chartXTitle'),y_title:value('chartYTitle'),colors:[value('chartColor'),'#68a294','#e0aa62','#9979b9']});}
            if(key==='removeChart'){state.charts.remove();showRight('chart');}
            if(key==='applyAI')applyAI();
            state.refresh();
        }
        async function print(){
            await state.prepareOutput();
            const config={sheet:sheet().getSheetId(),area:value('printArea'),orientation:value('orientation'),paper:value('paper'),scaling:value('scaling'),repeatRows:Math.max(0,Math.min(100,Number(value('repeatRows'))||0))};
            const range=sheet().getRange(config.area),bounds=range.getRange();if((bounds.endRow-bounds.startRow+1)*(bounds.endColumn-bounds.startColumn+1)>200000)throw Error(tr('print_limit'));
            if(!state.readonly)state.setAux({...structuredClone(state.aux),print:config});
            // The engine's clipboard serializer preserves cell styles, dimensions and merges.
            const source=new DOMParser().parseFromString(range.generateHTML(),'text/html'),table=source.querySelector('table');
            if(!table)throw Error(tr('print_failed'));
            source.querySelectorAll('script,iframe,object,embed,link,meta,img').forEach(node=>node.remove());
            source.querySelectorAll('*').forEach(node=>Array.from(node.attributes).forEach(attr=>{if(/^on/i.test(attr.name)||['href','src','srcdoc'].includes(attr.name))node.removeAttribute(attr.name);}));
            if(config.repeatRows){const head=document.createElement('thead');Array.from(table.rows).slice(0,config.repeatRows).forEach(row=>head.append(row));table.prepend(head);}
            const start=sheet().getRange(bounds.startRow,bounds.startColumn).getCellRect(),end=sheet().getRange(bounds.endRow,bounds.endColumn).getCellRect(),width=end.right-start.left;
            const charts=state.aux.charts.filter(c=>c.sheet===sheet().getSheetId()&&!c.readonly).map(c=>{
                const rect=sheet().getRange(c.anchor).getCellRect(),canvas=find('[data-charts]').querySelector('[data-chart-id="'+CSS.escape(c.id)+'"] canvas');
                if(!canvas)return '';const x=rect.left-start.left+(c.x||0),y=rect.top-start.top+(c.y||0);
                if(x+c.width<0||y+c.height<0||x>width||y>end.bottom-start.top)return '';
                return '<figure style="left:'+x+'px;top:'+y+'px;width:'+c.width+'px;height:'+c.height+'px"><figcaption>'+esc(c.title)+'</figcaption><img src="'+canvas.toDataURL()+'"></figure>';
            }).join('');
            const frame=document.createElement('iframe');frame.className='sheets-print-frame';frame.setAttribute('title',tr('print'));root.append(frame);
            const pageWidth=(config.paper==='letter'?(config.orientation==='landscape'?279.4:215.9):(config.orientation==='landscape'?297:210))-30,scale=config.scaling==='fit'?Math.min(1,pageWidth*96/25.4/width):1;
            const doc=frame.contentDocument;doc.open();doc.write('<!doctype html><html><head><meta charset="utf-8"><title>'+esc(state.path.split('/').pop())+'</title><link rel="stylesheet" href="/css/desktop-app-sheets.css"><style>@page{size:'+config.paper+' '+config.orientation+';margin:15mm}body{margin:0;font:11pt Arial;color:#202d3e}.print-sheet{position:relative;zoom:'+scale+';width:'+width+'px;overflow:hidden}table{border-collapse:collapse}td{border:1px solid #dce2e9}thead{display:table-header-group}tr{break-inside:avoid}figure{position:absolute;box-sizing:border-box;margin:0;padding:12px;background:white;border:1px solid #e0e5eb;border-radius:6px;overflow:hidden}figure img{width:100%;height:calc(100% - 26px);object-fit:contain}figcaption{height:26px;font-weight:600}</style></head><body><section class="print-sheet">'+table.outerHTML+charts+'</section></body></html>');doc.close();await doc.fonts.ready;
            if(state.signal.aborted){frame.remove();return;}
            const remove=()=>frame.remove();frame.contentWindow.addEventListener('afterprint',remove,{once:true});state.signal.addEventListener('abort',remove,{once:true});frame.contentWindow.focus();frame.contentWindow.print();
        }
        root.addEventListener('click',event=>{
            const target=event.target.closest('[data-panel-action]');if(target){event.preventDefault();action(target.dataset.panelAction).catch(state.fail);}
            const match=event.target.closest('[data-match]');if(match){matchIndex=Number(match.dataset.match);goMatch();renderMatches();}
            const named=event.target.closest('[data-name-range]');if(named){const ref=state.book.getDefinedNames().find(n=>n.getName()===named.dataset.nameRange)?.getFormulaOrRefString();const m=/^'?((?:[^']|'')+)'?!([$A-Z0-9:]+)$/i.exec(ref||'');if(m){const s=state.book.getSheetByName(m[1].replace(/''/g,"'"));const r=s?.getRange(m[2]);if(r){state.book.setActiveSheet(s);r.activate();const b=r.getRange();s.scrollToCell(b.startRow,b.startColumn);}}}
            const nav=event.target.closest('[data-nav-sheet]');if(nav){state.book.setActiveSheet(nav.dataset.navSheet);state.refresh();}
            const fn=event.target.closest('[data-function]');if(fn){const input=find('[data-formula]');input.value=input.value.startsWith('=')?input.value.replace(/[A-Z]+$/i,fn.dataset.function+'('):'='+fn.dataset.function+'(';input.dispatchEvent(new Event('input',{bubbles:true}));input.focus({preventScroll:true});}
        },{signal:life.signal});
        root.addEventListener('change',event=>{
            const key=event.target.dataset.field;if(state.readonly||!key)return;
            state.run(()=>{if(key==='fontColor')return state.format('cl',{rgb:event.target.value});if(key==='fillColor')return state.format('bg',{rgb:event.target.value});if(key==='vertical')return state.format('vt',{top:1,middle:2,bottom:3}[event.target.value]);});
        },{signal:life.signal});
        root.addEventListener('input',event=>{if(event.target.dataset.field==='functionQuery')listFunctions(event.target.value);},{signal:life.signal});
        return {showLeft,showRight,action,formulaHints,refresh(){const pill=root.querySelector('.sheets-selection-pill');if(pill)pill.textContent=state.selected().getA1Notation();if(suggestion&&state.revision!==suggestion.source_revision){const b=root.querySelector('[data-panel-action="applyAI"]');if(b)b.disabled=true;}},dispose(){life.abort();assistLife?.abort();clearHighlights();}};
    }
    window.SheetsPanels={create};
})();
