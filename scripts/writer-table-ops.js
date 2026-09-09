// MIT. Table merge/split through the core's public, atomic tree operations.
import {findNode,parentNodeOf} from '@docx-editor.dev/core/store';
const W='http://schemas.openxmlformats.org/wordprocessingml/2006/main';
const children=(node,name)=>(node.children || []).filter(x=>x.namespaceUri===W && x.localName===name);
const child=(node,name)=>children(node,name)[0];
const attr=(node,name)=>node?.attributes?.find(x=>x.namespaceUri===W && x.localName===name)?.value;
const element=(name,kind='generic',nodes=[],attributes=[])=>({id:'autor:'+crypto.randomUUID(),kind,namespaceUri:W,localName:name,prefix:'w',namespaceBindings:[],attributes,children:nodes});
const property=(name,value)=>element(name,'generic',[],[{kind:'genericExtension',namespaceUri:W,localName:'val',prefix:'w',value:String(value)}]);
const span=cell=>Math.max(1,Number(attr(child(child(cell,'tcPr') || {},'gridSpan'),'val')) || 1);
function cellProperties(cell,values) {
    const original=child(cell,'tcPr') || element('tcPr');
    const props={...original,children:original.children.filter(x=>!Object.hasOwn(values,x.localName))};
    for(const [name,value] of Object.entries(values))if(value!==null)props.children.push(property(name,value));
    return {...cell,children:[props,...cell.children.filter(x=>x!==original && x.localName!=='tcPr')]};
}
function cellWidth(cell,width) {
    const props=child(cell,'tcPr') || element('tcPr');
    const old=child(props,'tcW') || element('tcW');
    const attributes=old.attributes.filter(x=>!['w','type'].includes(x.localName));
    attributes.push(...['w','type'].map((name,i)=>({kind:'genericExtension',namespaceUri:W,localName:name,prefix:'w',value:i?'dxa':String(Math.round(width))})));
    return {...cell,children:[{...props,children:[...props.children.filter(x=>x!==old),{...old,attributes}]},...cell.children.filter(x=>x.localName!=='tcPr')]};
}
function grid(table) {
    return children(table,'tr').map((row,r)=>{
        let col=Number(attr(child(child(row,'trPr') || {},'gridBefore'),'val')) || 0;
        return children(row,'tc').map(cell=>{const start=col;col+=span(cell);return {cell,row:r,start,end:col-1};});
    });
}
function firstParagraph(node) {
    if(node.localName==='p')return node;
    for(const item of node.children || []){const found=firstParagraph(item);if(found)return found;}
}
export function tableCommand(editor,command,dry=false) {
    const refuse=reason=>({ok:false,code:'unsupported',reason});
    if(editor.snapshot().editingMode!=='editing')return refuse('Table structure cannot be reliably tracked; switch to editing mode.');
    const selected=editor.getSelectedTable(),surface=editor.surface;
    if(!selected || !surface)return refuse('Place the caret in a table.');
    const scope=surface.storyScope(),part=surface.session.partFor(scope),table=findNode(part,selected.blockId);
    if(!table || table.localName!=='tbl')return refuse('The selected table is unavailable.');
    const allowed=editor.can({type:'deleteTable'});if(!allowed.ok)return allowed;
    const parent=parentNodeOf(part,table.id),position=parent?.children.indexOf(table);
    const following=parent?.children[position+1];
    if(!following || following.localName!=='p')return refuse('This table needs a following paragraph before its structure can be changed.');
    const rows=children(table,'tr'),cells=grid(table);
    let replacement;
    if(command.type==='mergeCells') {
        const selection=editor.getTableCellSelection();
        if(!selection || selection.tableId!==table.id || selection.cellIds.length<2)return refuse('Select a rectangle containing at least two cells.');
        const {from:top,to:bottom}=selection.rows,{from:left,to:right}=selection.columns;
        const chosen=cells.flat().filter(x=>x.row>=top && x.row<=bottom && x.start>=left && x.end<=right);
        if(chosen.length!==selection.cellIds.length || chosen.some(x=>child(child(x.cell,'tcPr') || {},'vMerge')))return refuse('Split existing vertical merges before combining this selection.');
        for(let r=top;r<=bottom;r++) {
            const row=chosen.filter(x=>x.row===r);
            if(!row.length || row[0].start!==left || row.at(-1).end!==right)return refuse('Select complete cells in a rectangle.');
        }
        const contents=chosen.flatMap(x=>x.cell.children.filter(n=>n.localName!=='tcPr'));
        const updated=new Map();
        for(let r=top;r<=bottom;r++) {
            const selectedCells=chosen.filter(x=>x.row===r).map(x=>x.cell),ids=new Set(selectedCells.map(x=>x.id));
            let merged=cellProperties(selectedCells[0],{gridSpan:right-left+1,vMerge:top===bottom?null:r===top?'restart':'continue'});
            const widths=children(child(table,'tblGrid') || {},'gridCol').map(x=>Number(attr(x,'w')) || 1440);
            merged=cellWidth(merged,widths.slice(left,right+1).reduce((sum,x)=>sum+x,0));
            merged={...merged,children:[child(merged,'tcPr'),...(r===top?contents:[element('p','paragraph')])]};
            let placed=false;
            updated.set(rows[r].id,{...rows[r],children:rows[r].children.flatMap(x=>{
                if(!ids.has(x.id))return [x];if(placed)return [];placed=true;return [merged];
            })});
        }
        replacement={...table,children:table.children.map(x=>updated.get(x.id) || x)};
    } else {
        if(command.rows!==1 || command.cols!==2)return refuse('Split supports two columns.');
        const current=cells[selected.cell?.row]?.find(x=>x.start<=selected.cell.column && x.end>=selected.cell.column);
        if(!current)return refuse('Place the caret in a table cell.');
        const targets=new Set([current.cell.id]);
        if(child(child(current.cell,'tcPr') || {},'vMerge')) {
            let top=current.row;
            const at=r=>cells[r]?.find(x=>x.start===current.start && x.end===current.end);
            while(top>0 && attr(child(child(at(top)?.cell || {},'tcPr') || {},'vMerge'),'val')!=='restart')top--;
            for(let r=top;r<cells.length;r++) {
                const entry=at(r),merge=child(child(entry?.cell || {},'tcPr') || {},'vMerge');
                if(!merge || (r>top && attr(merge,'val')==='restart'))break;
                targets.add(entry.cell.id);
            }
        }
        const double=span(current.cell)===1,updated=new Map();
        for(const row of rows)updated.set(row.id,{...row,children:row.children.flatMap(cell=>{
            if(cell.localName!=='tc')return [cell];
            const size=span(cell)*(double?2:1);
            const adjusted=double?cellProperties(cell,{gridSpan:size}):cell;
            if(!targets.has(cell.id))return [adjusted];
            const left=Math.floor(size/2),right=size-left;
            const totalWidth=Number(attr(child(child(cell,'tcPr') || {},'tcW'),'w')) || 1440;
            const first=cellWidth(cellProperties(adjusted,{gridSpan:left,vMerge:null}),totalWidth*left/size);
            const second={...cellWidth(cellProperties(adjusted,{gridSpan:right,vMerge:null}),totalWidth*right/size),id:'autor:'+crypto.randomUUID()};
            second.children=[child(second,'tcPr'),element('p','paragraph')];
            return [first,second];
        })});
        replacement={...table,children:table.children.map(x=>{
            if(x.localName==='tblGrid' && double)return {...x,children:x.children.flatMap(col=>{
                if(col.localName!=='gridCol')return [col];
                const width=Math.max(2,Number(attr(col,'w')) || 1440);
                return [Math.floor(width/2),width-Math.floor(width/2)].map(w=>({...col,id:'autor:'+crypto.randomUUID(),attributes:col.attributes.map(a=>a.localName==='w'?{...a,value:String(w)}:a)}));
            })};
            return updated.get(x.id) || x;
        })};
    }
    if(dry)return {ok:true};
    let result;
    surface.commitReviewOps(()=>{
        result=surface.session.applyTreeOps([
            {op:'deleteBlock',blockId:table.id},
            {op:'insertFragment',paragraphId:following.id,offset:0,blocks:[replacement],lastMarkCovered:true}
        ],null,null,scope);
        return result;
    },'package-scoped');
    if(!result?.committed)return refuse(result?.reason || 'The table change was refused.');
    const nextPart=surface.session.partFor(scope),nextParent=findNode(nextPart,parent.id);
    const nextTable=nextParent?.children.find((x,i)=>i>=position && x.localName==='tbl');
    const paragraph=nextTable && firstParagraph(nextTable);
    if(paragraph)surface.setSelection({anchor:{paragraphId:paragraph.id,offset:0},head:{paragraphId:paragraph.id,offset:0}});
    return {ok:true,changed:true};
}
