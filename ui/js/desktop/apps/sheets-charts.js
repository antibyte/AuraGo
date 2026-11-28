(function(){
    'use strict';
    function create(state){
        const layer=state.find('[data-charts]'),items=new Map(),life=new AbortController();let selectedID=null;
        const palette=['#467caf','#68a294','#e0aa62','#9979b9','#cf7880'];
        function sourceValues(ref){
            const match=/^=?('(?:[^']|'')*'|[^!]+)!([$A-Z0-9:]+)$/i.exec(ref||'');
            if(!match)return [];
            const name=match[1].replace(/^'|'$/g,'').replace(/''/g,"'"),sheet=state.book.getSheetByName(name);
            try{return sheet?.getRange(match[2].replaceAll('$','')).getRawValues().flat()||[];}catch(_){return [];}
        }
        function chartData(chart){
            const sheet=state.book.getSheetBySheetId(chart.sheet),colors=chart.colors?.length?chart.colors:palette;
            if(chart.range){
                const range=sheet.getRange(chart.range),matrix=range.getRawValues(),labels=matrix.slice(1).map(row=>String(row[0]??''));
                const sets=[];
                for(let c=1;c<(matrix[0]?.length||0);c++){
                    const color=colors[(c-1)%colors.length],values=matrix.slice(1).map(row=>typeof row[c]==='number'?row[c]:null);
                    sets.push({label:String(matrix[0][c]??''),data:chart.type==='scatter'?matrix.slice(1).map(row=>({x:Number(row[0]),y:typeof row[c]==='number'?row[c]:null})):values,
                        backgroundColor:chart.type==='pie'?labels.map((_,i)=>colors[i%colors.length]):color,borderColor:color,borderWidth:chart.type==='line'?2:0,borderRadius:chart.type==='column'||chart.type==='bar'?3:0,pointRadius:chart.type==='scatter'?3:2,tension:.2});
                    if(chart.type==='pie')break;
                }
                return {labels,datasets:sets};
            }
            const sets=(chart.series||[]).map((series,i)=>{
                const x=sourceValues(series.categories),y=sourceValues(series.values),color=colors[i%colors.length];
                return {label:String(sourceValues(series.name)[0]??series.name??''),data:chart.type==='scatter'?y.map((v,i)=>({x:Number(x[i]),y:v})):y,backgroundColor:chart.type==='pie'?x.map((_,i)=>colors[i%colors.length]):color,borderColor:color,borderWidth:chart.type==='line'?2:0,pointRadius:3};
            });
            return {labels:sourceValues(chart.series?.[0]?.categories).map(String),datasets:sets};
        }
        function position(){
            if(life.signal.aborted||!state.book||state.api.getCurrentLifecycleStage()<2||!state.book.getActiveSheet()?.getSkeleton())return;
            const active=state.book.getActiveSheet(),zoom=active.getZoom(),scroll=active.getScrollState(),origin=active.getRange(scroll.sheetViewStartRow||0,scroll.sheetViewStartColumn||0).getCellRect(),first=active.getRange('A1').getCellRect();
            const dx=origin.left-first.left+(scroll.offsetX||0),dy=origin.top-first.top+(scroll.offsetY||0),freeze=active.getFreeze();
            for(const chart of state.aux.charts){
                const item=items.get(chart.id);if(!item)continue;item.element.hidden=chart.sheet!==active.getSheetId();if(item.element.hidden)continue;
                try{
                    const rect=active.getRange(chart.anchor||'A1').getCellRect();
                    Object.assign(item.element.style,{left:(rect.left+(chart.x||0)-(active.getRange(chart.anchor||'A1').getRange().startColumn<(freeze?.xSplit||0)?0:dx))*zoom+'px',top:(rect.top+(chart.y||0)-(active.getRange(chart.anchor||'A1').getRange().startRow<(freeze?.ySplit||0)?0:dy))*zoom+'px',width:chart.width*zoom+'px',height:chart.height*zoom+'px'});
                }catch(_){item.element.hidden=true;}
            }
        }
        function refresh(){
            if(life.signal.aborted||!state.book)return;
            const alive=new Set(state.aux.charts.map(c=>c.id));
            for(const [id,item]of items)if(!alive.has(id)){item.chart?.destroy();item.element.remove();items.delete(id);}
            for(const spec of state.aux.charts){
                let item=items.get(spec.id);
                if(!item){
                    const element=document.createElement('section');element.className='sheets-chart';element.dataset.chartId=spec.id;element.tabIndex=0;element.setAttribute('aria-label',spec.title||state.tr('chart'));
                    element.innerHTML='<div class="sheets-chart-drag" data-chart-drag><span></span><button type="button" data-chart-settings aria-label="'+state.esc(state.tr('chart_settings'))+'">•••</button></div><div class="sheets-chart-surface"><canvas></canvas></div><button type="button" class="sheets-chart-resize" data-chart-resize aria-label="'+state.esc(state.tr('resize_chart'))+'"></button>';
                    layer.append(element);item={element,chart:null,signature:''};items.set(spec.id,item);
                    element.addEventListener('pointerdown',event=>pointerDown(event,spec.id),{signal:life.signal});
                    element.addEventListener('keydown',event=>{if(event.key==='Delete'&&!state.readonly){event.preventDefault();remove(spec.id);}if(event.key==='Enter'){select(spec.id);state.act('chartSelected');}},{signal:life.signal});
                    element.querySelector('[data-chart-settings]').addEventListener('click',()=>{select(spec.id);state.act('chartSelected');},{signal:life.signal});
                }
                item.element.classList.toggle('is-selected',selectedID===spec.id);item.element.querySelector('[data-chart-drag] span').textContent=spec.title;
                item.element.querySelector('[data-chart-resize]').hidden=state.readonly||spec.readonly;
                if(spec.readonly){item.element.querySelector('.sheets-chart-surface').textContent=state.tr('chart_preserved');continue;}
                let data;try{data=chartData(spec);}catch(_){continue;}
                const signature=JSON.stringify([spec.type,spec.title,spec.legend,spec.x_title,spec.y_title,data]);
                if(signature!==item.signature&&window.Chart){
                    item.chart?.destroy();
                    item.chart=new Chart(item.element.querySelector('canvas'),{type:({column:'bar',bar:'bar',line:'line',pie:'pie',scatter:'scatter'})[spec.type],data,
                        options:{responsive:true,maintainAspectRatio:false,animation:false,indexAxis:spec.type==='bar'?'y':'x',interaction:{intersect:false,mode:'nearest'},
                            plugins:{legend:{display:!!spec.legend,position:'bottom',labels:{usePointStyle:true,boxWidth:7,color:'#63748a',font:{size:10},padding:14}},title:{display:false}},
                            ...(spec.type==='pie'?{}:{scales:{x:{grid:{display:false},border:{display:false},ticks:{color:'#7b8796',font:{size:10},maxRotation:0,maxTicksLimit:12},title:{display:!!spec.x_title,text:spec.x_title}},y:{grid:{color:'#e9edf2'},border:{display:false},ticks:{color:'#7b8796',font:{size:10},maxTicksLimit:6},title:{display:!!spec.y_title,text:spec.y_title}}}})}});
                    item.signature=signature;
                }
            }
            position();
        }
        function select(id){selectedID=id;for(const [key,item]of items)item.element.classList.toggle('is-selected',key===id);}
        function pointerDown(event,id){
            select(id);if(state.readonly)return;
            const spec=state.aux.charts.find(c=>c.id===id);if(!spec||spec.readonly)return;
            const resize=event.target.closest('[data-chart-resize]'),drag=event.target.closest('[data-chart-drag]');if(!resize&&!drag||event.target.closest('[data-chart-settings]'))return;
            event.preventDefault();event.stopPropagation();const element=items.get(id).element,startX=event.clientX,startY=event.clientY,zoom=state.book.getActiveSheet().getZoom();let dx=0,dy=0;
            element.setPointerCapture(event.pointerId);
            const move=e=>{dx=(e.clientX-startX)/zoom;dy=(e.clientY-startY)/zoom;if(resize){element.style.width=Math.max(240,spec.width+dx)*zoom+'px';element.style.height=Math.max(170,spec.height+dy)*zoom+'px';}else element.style.transform='translate('+dx*zoom+'px,'+dy*zoom+'px)';};
            const finish=e=>{
                element.removeEventListener('pointermove',move);element.removeEventListener('pointerup',finish);element.removeEventListener('pointercancel',cancel);element.style.transform='';
                if(element.hasPointerCapture(e.pointerId))element.releasePointerCapture(e.pointerId);
                if(Math.abs(dx)+Math.abs(dy)<2){position();return;}
                const next=structuredClone(state.aux),chart=next.charts.find(c=>c.id===id);
                if(resize){chart.width=Math.round(Math.max(240,spec.width+dx));chart.height=Math.round(Math.max(170,spec.height+dy));}else{chart.x=Math.round((spec.x||0)+dx);chart.y=Math.round((spec.y||0)+dy);}
                state.setAux(next);state.refresh();
            };
            const cancel=()=>{element.removeEventListener('pointermove',move);element.removeEventListener('pointerup',finish);element.removeEventListener('pointercancel',cancel);element.style.transform='';position();};
            element.addEventListener('pointermove',move);element.addEventListener('pointerup',finish);element.addEventListener('pointercancel',cancel);life.signal.addEventListener('abort',cancel,{once:true});
        }
        function add(type='column'){
            const range=state.selected(),r=range.getRange();if(r.endColumn<=r.startColumn||r.endRow<=r.startRow){state.notice(state.tr('chart_select_range'),true);return;}
            const id=crypto.randomUUID(),anchor=SheetsData.columnName(r.endColumn+2)+(r.startRow+1),next=structuredClone(state.aux);
            next.charts.push({id,sheet:state.book.getActiveSheet().getSheetId(),type,title:state.tr('chart'),range:range.getA1Notation(),anchor,x:0,y:0,width:480,height:290,legend:true,colors:palette});
            selectedID=id;state.setAux(next);state.refresh();return id;
        }
        function update(changes){if(changes.range){const range=state.book.getActiveSheet().getRange(changes.range).getRange();if(range.endColumn<=range.startColumn||range.endRow<=range.startRow||range.endColumn-range.startColumn>127||range.endRow-range.startRow>100000)throw Error(state.tr('chart_select_range'));}if(!selectedID)return;const next=structuredClone(state.aux),chart=next.charts.find(c=>c.id===selectedID);if(!chart||chart.readonly)return;Object.assign(chart,changes);state.setAux(next);}
        function remove(id=selectedID){if(state.readonly)return;const next=structuredClone(state.aux);next.charts=next.charts.filter(c=>c.id!==id||c.readonly);state.setAux(next);selectedID=null;state.refresh();}
        return {refresh,position,add,update,remove,select,get selected(){return state.aux.charts.find(c=>c.id===selectedID);},dispose(){life.abort();for(const item of items.values()){item.chart?.destroy();item.element.remove();}items.clear();}};
    }
    window.SheetsCharts={create};
})();
