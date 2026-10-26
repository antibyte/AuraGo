(function(){
    'use strict';
    function create(state){
        const layer=state.find('[data-charts]'),items=new Map(),life=new AbortController();let selectedID=null;
        const labels={title:state.ctx.t?.('desktop.qr_title'),image_resize:state.ctx.t?.('desktop.qr_image_resize'),image_move:state.ctx.t?.('desktop.qr_image_move'),image_preserved:state.ctx.t?.('desktop.qr_image_preserved'),stale:state.ctx.t?.('desktop.qr_stale')};
        const label=key=>labels[key]||key;
        const imageSource=image=>/^image\/(png|jpeg)$/.test(image.mime)&&/^[A-Za-z0-9+/]*={0,2}$/.test(image.data||'')&&image.data?'data:'+image.mime+';base64,'+image.data:'';
        function scaledSize(image,factor){factor=Math.max(1/Math.min(image.width,image.height),Math.min(4096/Math.max(image.width,image.height),factor));return {width:Math.round(image.width*factor),height:Math.round(image.height*factor)};}
        function position(){
            if(life.signal.aborted||!state.book||state.api.getCurrentLifecycleStage()<2||!state.book.getActiveSheet()?.getSkeleton())return;
            const sheet=state.book.getActiveSheet(),zoom=sheet.getZoom(),scroll=sheet.getScrollState(),origin=sheet.getRange(scroll.sheetViewStartRow||0,scroll.sheetViewStartColumn||0).getCellRect(),first=sheet.getRange('A1').getCellRect();
            const dx=origin.left-first.left+(scroll.offsetX||0),dy=origin.top-first.top+(scroll.offsetY||0),freeze=sheet.getFreeze();
            for(const image of state.aux.images||[]){
                const item=items.get(image.id);if(!item)continue;item.element.hidden=image.sheet!==sheet.getSheetId();if(item.element.hidden)continue;
                try{
                    const range=sheet.getRange(image.anchor),rect=range.getCellRect(),anchor=range.getRange();
                    Object.assign(item.element.style,{left:(rect.left+(image.x||0)-(anchor.startColumn<(freeze?.xSplit||0)?0:dx))*zoom+'px',top:(rect.top+(image.y||0)-(anchor.startRow<(freeze?.ySplit||0)?0:dy))*zoom+'px',width:image.width*zoom+'px',height:image.height*zoom+'px'});
                }catch(_){item.element.hidden=true;}
            }
        }
        function select(id){selectedID=id;for(const [key,item]of items)item.element.classList.toggle('is-selected',key===id);}
        function refresh(){
            if(life.signal.aborted||!state.book)return;
            const alive=new Set((state.aux.images||[]).map(image=>image.id));
            for(const [id,item]of items)if(!alive.has(id)){item.element.remove();items.delete(id);}
            for(const image of state.aux.images||[]){
                let item=items.get(image.id);
                if(!item){
                    const element=document.createElement('div');element.className='sheets-image';element.dataset.imageId=image.id;element.tabIndex=0;
                    const pixels=document.createElement('img');pixels.draggable=false;pixels.referrerPolicy='no-referrer';element.append(pixels);
                    const remove=document.createElement('button');remove.type='button';remove.className='sheets-image-remove';remove.dataset.imageRemove='';remove.textContent='×';remove.setAttribute('aria-label',state.tr('delete'));element.append(remove);
                    const resize=document.createElement('button');resize.type='button';resize.className='sheets-image-resize';resize.dataset.imageResize='';resize.setAttribute('aria-label',label('image_resize'));element.append(resize);
                    layer.append(element);item={element,pixels};items.set(image.id,item);
                    element.addEventListener('pointerdown',event=>pointerDown(event,image.id),{signal:life.signal});
                    element.addEventListener('focus',()=>select(image.id),{signal:life.signal});
                    element.addEventListener('keydown',event=>{
                        if(state.readonly)return;
                        if(event.key==='Delete'||event.key==='Backspace'){event.preventDefault();event.stopPropagation();removeImage(image.id);return;}
                        const move={ArrowLeft:[-1,0],ArrowRight:[1,0],ArrowUp:[0,-1],ArrowDown:[0,1]}[event.key];if(!move)return;
                        const value=(state.aux.images||[]).find(entry=>entry.id===image.id);if(!value||value.readonly)return;
                        event.preventDefault();event.stopPropagation();
                        if(event.shiftKey)update(image.id,scaledSize(value,1+(move[0]||move[1])*4/value.width));
                        else update(image.id,{x:(value.x||0)+move[0],y:(value.y||0)+move[1]});
                    },{signal:life.signal});
                    remove.addEventListener('click',event=>{event.preventDefault();event.stopPropagation();removeImage(image.id);},{signal:life.signal});
                }
                const editable=!state.readonly&&!image.readonly;
                item.element.classList.toggle('is-selected',selectedID===image.id);item.element.classList.toggle('is-readonly',!editable);
                item.element.setAttribute('aria-label',image.alt||state.tr('image'));item.element.title=image.readonly?label('image_preserved'):label('image_move');item.pixels.alt=image.alt||state.tr('image');
                item.element.querySelector('[data-image-remove]').hidden=!editable;item.element.querySelector('[data-image-resize]').hidden=!editable;
                const src=imageSource(image);
                if(item.pixels.getAttribute('src')!==src){if(src)item.pixels.src=src;else item.pixels.removeAttribute('src');}
            }
            position();
        }
        function update(id,changes){
            if(state.readonly)return;
            const next=structuredClone(state.aux),image=(next.images||[]).find(entry=>entry.id===id);if(!image||image.readonly)return;
            Object.assign(image,changes);state.setAux(next);state.refresh();
        }
        function removeImage(id=selectedID){
            if(state.readonly)return;
            const next=structuredClone(state.aux);next.images=(next.images||[]).filter(image=>image.id!==id||image.readonly);selectedID=null;state.setAux(next);state.refresh();
        }
        function pointerDown(event,id){
            select(id);const image=(state.aux.images||[]).find(entry=>entry.id===id);if(state.readonly||!image||image.readonly||event.button!==0||event.target.closest('[data-image-remove]'))return;
            event.preventDefault();event.stopPropagation();const element=items.get(id).element,resize=!!event.target.closest('[data-image-resize]'),startX=event.clientX,startY=event.clientY,zoom=state.book.getActiveSheet().getZoom();let dx=0,dy=0;
            element.focus({preventScroll:true});element.setPointerCapture(event.pointerId);
            const dimensions=()=>scaledSize(image,1+(Math.abs(dx)>Math.abs(dy)?dx/image.width:dy/image.height));
            const move=e=>{dx=(e.clientX-startX)/zoom;dy=(e.clientY-startY)/zoom;if(resize){const value=dimensions();element.style.width=value.width*zoom+'px';element.style.height=value.height*zoom+'px';}else element.style.transform='translate('+dx*zoom+'px,'+dy*zoom+'px)';};
            const cleanup=()=>{element.removeEventListener('pointermove',move);element.removeEventListener('pointerup',finish);element.removeEventListener('pointercancel',cancel);life.signal.removeEventListener('abort',cancel);element.style.transform='';if(element.hasPointerCapture(event.pointerId))element.releasePointerCapture(event.pointerId);};
            const finish=()=>{cleanup();if(Math.abs(dx)+Math.abs(dy)<2){position();return;}if(resize)update(id,dimensions());else update(id,{x:Math.round((image.x||0)+dx),y:Math.round((image.y||0)+dy)});};
            const cancel=()=>{cleanup();position();};
            element.addEventListener('pointermove',move);element.addEventListener('pointerup',finish);element.addEventListener('pointercancel',cancel);life.signal.addEventListener('abort',cancel,{once:true});
        }
        async function add(file,options={}){
            if(state.readonly)throw Error(state.tr('readonly'));
            const revision=state.revision,signal=state.signal,sheet=state.book.getActiveSheet().getSheetId(),anchor=state.selected().getA1Notation().split(':')[0];
            if(!file||file.type!=='image/png'||file.size>10*1024*1024)throw Error(state.tr('import_failed'));
            const bytes=new Uint8Array(await file.arrayBuffer());if(signal.aborted||state.signal!==signal||state.revision!==revision||state.book.getActiveSheet().getSheetId()!==sheet||state.selected().getA1Notation().split(':')[0]!==anchor)throw Error(label('stale'));
            let binary='';for(let offset=0;offset<bytes.length;offset+=32768)binary+=String.fromCharCode(...bytes.subarray(offset,offset+32768));
            const image={id:crypto.randomUUID(),sheet,anchor,x:0,y:0,width:Math.round(options.width||240),height:Math.round(options.height||240),mime:'image/png',data:btoa(binary),alt:label('title')};
            if(!Number.isFinite(image.width)||!Number.isFinite(image.height)||image.width<1||image.width>4096||image.height<1||image.height>4096||(state.aux.images||[]).length>=256||(state.aux.images||[]).reduce((total,item)=>total+(item.data?.length||0)*.75,bytes.length)>16*1024*1024)throw Error(state.tr('import_failed'));
            const next=structuredClone(state.aux);next.images||=[];next.images.push(image);selectedID=image.id;state.setAux(next);state.refresh();return image.id;
        }
        function printHTML(bounds){
            const sheet=state.book.getActiveSheet(),start=sheet.getRange(bounds.startRow,bounds.startColumn).getCellRect(),end=sheet.getRange(bounds.endRow,bounds.endColumn).getCellRect();
            return (state.aux.images||[]).filter(image=>image.sheet===sheet.getSheetId()&&imageSource(image)).map(image=>{
                const rect=sheet.getRange(image.anchor).getCellRect(),x=rect.left-start.left+(image.x||0),y=rect.top-start.top+(image.y||0);
                if(x+image.width<0||y+image.height<0||x>end.right-start.left||y>end.bottom-start.top)return '';
                return '<img class="sheets-print-image" style="position:absolute;left:'+x+'px;top:'+y+'px;width:'+image.width+'px;height:'+image.height+'px;object-fit:contain" src="'+imageSource(image)+'" alt="'+state.esc(image.alt)+'">';
            }).join('');
        }
        return {refresh,position,add,update,remove:removeImage,select,printHTML,dispose(){life.abort();for(const item of items.values())item.element.remove();items.clear();}};
    }
    window.SheetsImages={create};
})();
