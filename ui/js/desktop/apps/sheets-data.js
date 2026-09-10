(function(){
    'use strict';
    function columnName(index){let name='';for(let n=index+1;n>0;n=Math.floor((n-1)/26))name=String.fromCharCode(65+(n-1)%26)+name;return name;}
    function normalizeFormula(value){
        return value.replace(/"(?:[^"]|"")*"|'(?:[^']|'')*'|\bAVG(?=\s*\()/gi,match=>/^AVG$/i.test(match)?'AVERAGE':match);
    }
    function parseInput(value,locale='en',type='auto'){
        const text=String(value??'');
        if(type==='text'||text.startsWith("'"))return {v:type==='text'?text:text.slice(1),t:1};
        if(text.startsWith('='))return {f:normalizeFormula(text),v:null,t:2};
        if(text==='')return {v:null};
        if(/^(TRUE|FALSE)$/i.test(text))return {v:/^TRUE$/i.test(text)?1:0,t:3};
        const parts=new Intl.NumberFormat(locale).formatToParts(12345.6),decimal=parts.find(x=>x.type==='decimal')?.value||'.',group=parts.find(x=>x.type==='group')?.value||',';
        const trim=text.trim(),percent=trim.endsWith('%'),input=(percent?trim.slice(0,-1):trim).trim(),clean=input.split(group).join('').replace(decimal,'.').replace(/[\u00a0\u202f]/g,'');
        const integer=input.split(decimal)[0].replace(/^[-+]/,'').split(/[eE]/)[0],groups=integer.split(group),validGroups=groups.length===1||/^\d{1,3}$/.test(groups[0])&&groups.slice(1).every(part=>/^\d{3}$/.test(part));
        if(validGroups&&!/^[-+]?0\d/.test(clean)&&/^[-+]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][-+]?\d+)?$/.test(clean)){
            const number=Number(clean);if(Number.isFinite(number))return {v:percent?number/100:number,t:2,...(percent?{s:{n:{pattern:'0.00%'}}}:{})};
        }
        let date;
        const iso=/^(\d{4})-(\d{1,2})-(\d{1,2})$/.exec(trim);
        if(iso)date=[+iso[1],+iso[2],+iso[3]];
        else {
            const local=/^(\d{1,2})[./](\d{1,2})[./](\d{4})$/.exec(trim);
            if(local){const monthFirst=new Intl.DateTimeFormat(locale).formatToParts(new Date(2001,10,22)).find(x=>x.type==='day'||x.type==='month')?.type==='month';date=[+local[3],+local[monthFirst?1:2],+local[monthFirst?2:1]];}
        }
        if(date){const [y,m,d]=date,stamp=Date.UTC(y,m-1,d),check=new Date(stamp);if(y>=1900&&y<=9999&&check.getUTCMonth()===m-1&&check.getUTCDate()===d)return {v:stamp/86400000+25569-(stamp<Date.UTC(1900,2,1)?1:0),t:2,s:{n:{pattern:new Intl.DateTimeFormat(locale,{year:'numeric',month:'2-digit',day:'2-digit'}).formatToParts(new Date(2020,10,22)).map(p=>({year:'yyyy',month:'mm',day:'dd'})[p.type]||p.value).join('')}}};}
        return {v:text,t:1};
    }
    function parseCSV(text,delimiter=','){
        const rows=[],row=[];let cell='',quoted=false,closed=false;
        text=text.replace(/^\uFEFF/,'');
        for(let i=0;i<text.length;i++){
            const char=text[i];
            if(quoted){if(char==='"'&&text[i+1]==='"'){cell+='"';i++;}else if(char==='"'){quoted=false;closed=true;}else cell+=char;}
            else if(char==='"'&&cell===''&&!closed)quoted=true;
            else if(char===delimiter){row.push(cell);cell='';closed=false;}
            else if(char==='\n'||char==='\r'){if(char==='\r'&&text[i+1]==='\n')i++;row.push(cell);rows.push(row.splice(0));cell='';closed=false;}
            else {if(closed&&char.trim())throw Error('csv_invalid');cell+=char;}
            if(rows.length>100000||row.length>1000||cell.length>32767)throw Error('import_limit');
        }
        if(quoted)throw Error('csv_invalid');
        if(cell!==''||row.length){row.push(cell);rows.push(row);}
        if(rows.reduce((sum,row)=>sum+row.length,0)>1000000)throw Error('import_limit');
        return rows;
    }
    function template(kind,tr){
        const id='sheet-'+crypto.randomUUID().slice(0,8),name=tr(kind==='blank'?'sheet':'template_'+kind);
        const sheet={id,name,rowCount:1000,columnCount:26,defaultRowHeight:26,defaultColumnWidth:112,cellData:{},rowData:{},columnData:{0:{w:210}},mergeData:[],freeze:{xSplit:0,ySplit:0,startRow:-1,startColumn:-1},zoomRatio:1,showGridlines:1,rowHeader:{width:48},columnHeader:{height:26}};
        const book={id:crypto.randomUUID(),name,appVersion:'0.25.1',locale:'enUS',sheetOrder:[id],sheets:{[id]:sheet},styles:{
            title:{ff:'Arial',fs:24,bl:1,cl:{rgb:'#20364f'}},
            subtitle:{ff:'Arial',fs:10,cl:{rgb:'#6d7c91'}},
            header:{ff:'Arial',fs:11,bl:1,cl:{rgb:'#ffffff'},bg:{rgb:'#365f85'},vt:2},
            body:{ff:'Arial',fs:11,cl:{rgb:'#26374b'}},
            number:{ff:'Arial',fs:11,n:{pattern:'#,##0.00'},cl:{rgb:'#26374b'}},
            total:{ff:'Arial',fs:11,bl:1,bg:{rgb:'#e2ecf6'},cl:{rgb:'#244567'},n:{pattern:'#,##0.00'}}
        }};
        const doc={schema_version:2,workbook:book,charts:[],limitations:[],structural_locked:false};
        if(kind==='blank')return doc;
        const row=(r,values,style)=>{sheet.cellData[r]={};values.forEach((value,c)=>{sheet.cellData[r][c]=typeof value==='number'?{v:value,t:2,s:style||'number'}:String(value).startsWith('=')?{f:value,s:style||'number'}:{v:value,t:1,s:style||'body'};});};
        row(0,[name],'title');sheet.rowData[0]={h:48};sheet.mergeData.push({startRow:0,endRow:0,startColumn:0,endColumn:4});
        row(1,[tr('template_subtitle')],'subtitle');sheet.mergeData.push({startRow:1,endRow:1,startColumn:0,endColumn:4});
        if(kind==='budget'){
            row(3,['category','planned','actual','difference','notes'].map(tr),'header');sheet.rowData[3]={h:32};
            [['housing',1250,1250],['food',420,385],['transport',140,118],['leisure',180,205],['savings',350,350]].forEach(([key,planned,actual],i)=>row(i+4,[tr(key),planned,actual,'=B'+(i+5)+'-C'+(i+5),'']));
            row(10,[tr('total'),'=SUM(B5:B9)','=SUM(C5:C9)','=B11-C11',''],'total');sheet.rowData[10]={h:34};
            doc.charts=[{id:crypto.randomUUID(),sheet:id,type:'column',title:tr('template_budget'),range:'A4:C9',anchor:'F4',x:12,y:0,width:340,height:270,legend:true,colors:['#467caf','#b5cbdc']}];
        }else if(kind==='project'){
            row(3,['task','owner','start_date','due_date','progress'].map(tr),'header');
            ['planning','implementation','review','delivery'].forEach((key,i)=>row(i+4,[tr(key),'','','',i===0?1:0]));
            for(let r=4;r<8;r++)sheet.cellData[r][4]={v:r===4?1:0,t:2,s:{n:{pattern:'0%'}}};
        }else if(kind==='inventory'){
            row(3,['item','sku','quantity','unit_price','total'].map(tr),'header');
            ['notebooks','pens','folders','labels'].forEach((key,i)=>row(i+4,[tr(key),'SKU-00'+(i+1),[24,60,15,120][i],[4.5,1.2,3.8,.2][i],'=C'+(i+5)+'*D'+(i+5)]));
            row(10,[tr('total'),'','=SUM(C5:C8)','','=SUM(E5:E8)'],'total');
        }
        sheet.freeze={xSplit:0,ySplit:4,startRow:4,startColumn:0};
        return doc;
    }
    async function importCSV(state,path){
        const response=await fetch('/api/desktop/download?path='+encodeURIComponent(path),{signal:state.signal});
        if(!response.ok)throw Error(state.tr('import_failed'));
        const bytes=await response.arrayBuffer();if(bytes.byteLength>20*1024*1024)throw Error(state.tr('import_limit'));
        const dialog=document.createElement('dialog');dialog.className='sheets-import-dialog';
        dialog.innerHTML='<form method="dialog"><h2>'+state.esc(state.tr('csv_import'))+'</h2><div class="sheets-import-controls"><label>'+state.esc(state.tr('delimiter'))+'<select data-delimiter><option value=",">,</option><option value=";">;</option><option value="\t">Tab</option><option value="|">|</option></select></label><label>'+state.esc(state.tr('encoding'))+'<select data-encoding><option value="utf-8">UTF-8</option><option value="windows-1252">Windows-1252</option><option value="utf-16le">UTF-16 LE</option></select></label></div><p>'+state.esc(state.tr('csv_type_hint'))+'</p><div data-preview class="sheets-import-preview"></div><p data-error role="alert"></p><footer><button value="cancel">'+state.esc(state.tr('cancel'))+'</button><button value="import">'+state.esc(state.tr('import'))+'</button></footer></form>';
        state.root.append(dialog);let rows=[],types=[],failed=false;
        const preview=()=>{
            try{
                rows=parseCSV(new TextDecoder(dialog.querySelector('[data-encoding]').value,{fatal:true}).decode(bytes),dialog.querySelector('[data-delimiter]').value);
                const width=Math.max(0,...rows.slice(0,25).map(row=>row.length));types=Array.from({length:width},(_,i)=>types[i]||'auto');
                dialog.querySelector('[data-preview]').innerHTML='<table><thead><tr>'+types.map((v,i)=>'<th><select data-type="'+i+'"><option value="auto"'+(v==='auto'?' selected':'')+'>'+state.esc(state.tr('automatic'))+'</option><option value="text"'+(v==='text'?' selected':'')+'>'+state.esc(state.tr('format_text'))+'</option></select></th>').join('')+'</tr></thead><tbody>'+rows.slice(0,12).map(row=>'<tr>'+types.map((_,i)=>'<td>'+state.esc(row[i]||'')+'</td>').join('')+'</tr>').join('')+'</tbody></table>';
                dialog.querySelector('[data-error]').textContent='';failed=false;
            }catch(error){failed=true;dialog.querySelector('[data-error]').textContent=state.tr(error.message==='csv_invalid'?'csv_invalid':'import_failed');}
        };
        const sample=new TextDecoder().decode(bytes.slice(0,10000)).split(/\r?\n/)[0],candidate=[',',';','\t','|'].sort((a,b)=>sample.split(b).length-sample.split(a).length)[0];dialog.querySelector('[data-delimiter]').value=candidate;
        dialog.addEventListener('change',event=>{if(event.target.dataset.type!==undefined)types[+event.target.dataset.type]=event.target.value;else preview();});
        preview();dialog.showModal();
        const result=await new Promise(resolve=>{dialog.addEventListener('close',()=>resolve(dialog.returnValue==='import'&&!failed),{once:true});state.signal.addEventListener('abort',()=>{dialog.close('cancel');},{once:true});dialog.querySelector('form').addEventListener('submit',event=>{if(event.submitter?.value==='import'&&failed)event.preventDefault();});});dialog.remove();
        if(!result)return null;
        const doc=template('blank',state.tr),sheet=doc.workbook.sheets[doc.workbook.sheetOrder[0]];
        rows.forEach((row,r)=>{sheet.cellData[r]={};row.forEach((value,c)=>{sheet.cellData[r][c]=value.startsWith('=')?{v:value,t:1}:parseInput(value,state.locale,types[c]||'auto');});});
        sheet.rowCount=Math.max(1000,rows.length+50);sheet.columnCount=rows.reduce((max,row)=>Math.max(max,row.length),26);return doc;
    }
    // OSS does not ship every AuraGo locale. Localize the native controls that
    // remain visible inside our own chrome using the shared desktop vocabulary.
    function nativeLocale(base,tr,locale){
        const out=structuredClone(base),op={between:'{FORMULA1} ≤ x ≤ {FORMULA2}',greaterThan:'x > {FORMULA1}',greaterThanOrEqual:'x ≥ {FORMULA1}',lessThan:'x < {FORMULA1}',lessThanOrEqual:'x ≤ {FORMULA1}',equal:'x = {FORMULA1}',notEqual:'x ≠ {FORMULA1}',notBetween:'x < {FORMULA1} ∨ x > {FORMULA2}',legal:tr('validation')};
        const f=out['sheets-filter-ui'];if(f){
            Object.assign(f.toolbar,{'smart-toggle-filter-tooltip':tr('toggleFilter'),'clear-filter-criteria':tr('clearFilter'),'re-calc-filter-conditions':tr('applyFilter')});
            f.shortcut['smart-toggle-filter']=tr('toggleFilter');
            Object.assign(f.panel,{'clear-filter':tr('clearFilter'),cancel:tr('cancel'),confirm:tr('applyFilter'),'by-values':tr('values'),'by-colors':tr('color'),'filter-by-cell-fill-color':tr('fill_color'),'filter-by-cell-text-color':tr('font_color'),'filter-by-color-none':tr('none'),'by-conditions':tr('condition'),'filter-only':tr('applyFilter'),'search-placeholder':tr('find'),'select-all':tr('fm.select_all'),'input-values-placeholder':tr('value'),and:'∧',or:'∨',empty:'∅','?':'? = 1','*':'* = 0…'});
            Object.assign(f.conditions,{none:tr('none'),empty:'= ∅','not-empty':'≠ ∅','text-contains':tr('contains'),'does-not-contain':'¬ '+tr('contains'),'starts-with':tr('text')+'*','ends-with':'*'+tr('text'),equals:'=',equal:'=','not-equal':'≠','greater-than':'>','greater-than-or-equal':'>=','less-than':'<','less-than-or-equal':'<=',between:'a ≤ x ≤ b','not-between':'x < a ∨ x > b',custom:tr('custom')});
            f.date=Object.fromEntries(Array.from({length:12},(_,i)=>[i+1,new Intl.DateTimeFormat(locale,{month:'long'}).format(new Date(2000,i,1))]));
        }
        for(const name of ['sheets-data-validation','sheets-data-validation-ui']){
            const v=out[name];if(!v)continue;
            if(v.title)v.title=tr('validation');
            if(v.alert)v.alert={title:tr('validation'),ok:tr('ok')};
            if(v.error)v.error={title:tr('validation')+':'};
            for(const [type,label]of Object.entries({date:'format_date',decimal:'format_number',whole:'format_number',list:'dropdown_list',listMultiple:'dropdown_list',textLength:'text',any:'value',custom:'formula',checkbox:'value'})){
                const entry=v[type];if(!entry)continue;entry.title=tr(label);
                for(const key of ['error','validFail','emptyError','formulaError'])if(key in entry)entry[key]=tr('validation')+': '+tr(label);
                if(entry.errorMsg)entry.errorMsg=Object.fromEntries(Object.entries(op).map(([key,text])=>[key,tr(label)+': '+text]));
                if(entry.ruleName&&typeof entry.ruleName==='object')entry.ruleName=op;
                for(const [key,text]of Object.entries({add:'+',dropdown:tr('value'),options:tr('allowed_values'),edit:tr('edit'),name:tr('allowed_values'),customOptions:tr('custom'),refOptions:tr('data_range')}))if(key in entry)entry[key]=text;
            }
            if(v.errorMsg)v.errorMsg=Object.fromEntries(Object.entries(op).map(([key,text])=>[key,tr('validation')+': '+text]));
            if(v.ruleName)v.ruleName=op;
            if(v.validFail)for(const key of Object.keys(v.validFail))v.validFail[key]=tr('validation')+': '+tr('value')+' / '+tr('formula');
        }
        if(out['sheets-note-ui'])out['sheets-note-ui']={note:{placeholder:tr('notes')},rightClick:{addNote:tr('setNote'),deleteNote:tr('deleteNote'),toggleNote:tr('notes')}};
        if(out.sheets?.autoFill)out.sheets.autoFill={copy:tr('copy'),series:tr('ai_continue'),formatOnly:tr('pasteFormat'),noFormat:tr('pasteValues')};
        if(out['sheets-ui']?.button)Object.assign(out['sheets-ui'].button,{confirm:tr('ok'),cancel:tr('cancel'),close:tr('close'),insert:tr('insert'),prevPage:tr('searchPrevious'),nextPage:tr('searchNext')});
        if(out.ui?.textEditor)out.ui.textEditor={formulaError:tr('english_formulas'),rangeError:tr('invalid_range')};
        if(out['sheets-filter'])out['sheets-filter']={command:{'not-valid-filter-range':tr('invalid_range')},msg:{'filter-header-forbidden':tr('structure_locked')}};
        return out;
    }
    window.SheetsData={columnName,normalizeFormula,parseInput,parseCSV,template,importCSV,nativeLocale};
})();
