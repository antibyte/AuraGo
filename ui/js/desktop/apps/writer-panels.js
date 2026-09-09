(function () {
    'use strict';
    function create(app) {
        const {root,find,tr,esc}=app, life=new AbortController();
        let side='', matches=[], matchIndex=-1, searchSignature='', reviewSignature='', outlineSignature='';
        let aiRequest, suggestion, printFrame;
        const textButton=(id,label=id)=>'<button type="button" data-panel-command="'+id+'">'+esc(tr(label))+'</button>';
        const commandButton=(command,label)=>'<button type="button" data-command="'+esc(JSON.stringify(command))+'">'+esc(tr(label))+'</button>';
        const field=(label,input)=>'<label class="writer-field"><span>'+esc(tr(label))+'</span>'+input+'</label>';
        const input=(key,value,type='number',extra='')=>'<input data-field="'+key+'" type="'+type+'" value="'+esc(value)+'" '+extra+'>';
        const select=(key,items)=>'<select data-field="'+key+'">'+items.map(([value,label])=>'<option value="'+esc(value)+'">'+esc(tr(label))+'</option>').join('')+'</select>';
        const section=(label,body)=>'<section class="writer-panel-section"><h3>'+esc(tr(label))+'</h3>'+body+'</section>';
        function show(which,name) {
            const node=find('[data-'+which+']');node.hidden=!name;
            if(!name){node.replaceChildren();return;}
            if(which==='right')side=name;
            node.innerHTML='<div class="writer-panel-heading"><h2>'+esc(tr(name==='search'?'find':name))+'</h2><button type="button" data-action="'+name+'" aria-label="'+esc(tr('close'))+'">×</button></div><div class="writer-panel-content">'+content(name)+'</div>';
            if(name==='review')reviewSignature='';
            if(name==='outline')outlineSignature='';
            if(name==='search'){searchSignature='';node.querySelector('[data-field="query"]').focus();}
            if(name==='assist')renderSuggestion();
        }
        function content(name) {
            switch(name) {
                case 'outline': return '<p class="writer-hint">'+esc(tr('outline_hint'))+'</p><nav data-outline></nav>';
                case 'search': return field('find',input('query','','search','placeholder="'+esc(tr('search_placeholder'))+'"'))+
                    '<div class="writer-checks"><label><input type="checkbox" data-field="matchCase">'+esc(tr('match_case'))+'</label><label><input type="checkbox" data-field="wholeWord">'+esc(tr('whole_word'))+'</label></div>'+
                    '<div class="writer-buttonrow">'+textButton('previous')+textButton('next')+'</div><p data-matches role="status"></p>'+
                    section('replace',field('replace',input('replacement','','text'))+'<div class="writer-buttonrow">'+textButton('replace_one','replace')+textButton('replace_all')+'</div>');
                case 'format': return section('text',field('color',input('text.color','#172033','color'))+field('highlight',input('text.highlight','#fff2aa','color'))+
                    '<div class="writer-buttonrow">'+commandButton({type:'setAlignment',align:'justify'},'justify')+commandButton({type:'clearFormatting'},'clear_format')+'</div>')+
                    section('paragraph',field('line_spacing',input('lineSpacing',1.15,'number','min=".5" max="10" step=".05"'))+
                    field('before_spacing',input('beforePt',0,'number','min="0" max="200"'))+field('after_spacing',input('afterPt',8,'number','min="0" max="200"'))+
                    field('left_indent',input('leftIndent',0,'number','min="0" max="100"'))+field('first_indent',input('firstIndent',0,'number','min="-100" max="100"')))+
                    section('page_setup',field('paper',select('paper',[['a4','a4'],['letter','letter_paper']]))+
                    field('orientation',select('orientation',[['portrait','portrait'],['landscape','landscape']]))+
                    ['Top','Bottom','Left','Right'].map(x=>field('margin_'+x.toLowerCase(),input('margin'+x,25,'number','min="0" max="100"'))).join(''))+
                    section('table','<p class="writer-hint">'+esc(tr('table_resize_hint'))+'</p>'+
                    field('cell_fill',input('cellFill','#ffffff','color'))+
                    '<div class="writer-buttonrow">'+commandButton({type:'setTableBorders',scope:'all',spec:{style:'single',size:4,color:{kind:'hex',value:'64748B'}}},'borders')+
                    commandButton({type:'setTableBorders',scope:'none',target:'all'},'no_borders')+'</div>'+
                    '<div class="writer-buttonrow">'+commandButton({type:'insertRow',where:'below'},'row_add')+commandButton({type:'deleteRow'},'row_delete')+
                    commandButton({type:'insertColumn',where:'right'},'column_add')+commandButton({type:'deleteColumn'},'column_delete')+
                    commandButton({type:'mergeCells'},'merge_cells')+commandButton({type:'splitCell',rows:1,cols:2},'split_cell')+'</div>')+
                    section('image',field('image_width',input('imageWidth',80,'number','min="1" max="300"'))+field('alt_text',input('imageAlt','','text'))+
                    field('image_wrap',select('imageWrap',[['inline','inline'],['squareLeft','wrap_left'],['squareRight','wrap_right'],['topAndBottom','wrap_top']])));
                case 'insert':return section('content','<div class="writer-buttonrow">'+textButton('table_insert','table')+'<button data-action="image">'+esc(tr('image'))+'</button>'+
                    commandButton({type:'insertBreak',kind:'page'},'page_break')+commandButton({type:'insertNote',noteKind:'footnote'},'footnote')+
                    commandButton({type:'insertToc'},'toc')+textButton('hyperlink')+'</div>')+
                    section('headers','<div class="writer-buttonrow">'+commandButton({type:'editHeaderFooter',position:'header'},'header')+
                    commandButton({type:'editHeaderFooter',position:'footer'},'footer')+commandButton({type:'insertPageField',field:'PAGE_X_OF_Y'},'page_numbers')+
                    commandButton({type:'exitHeaderFooter'},'back_document')+'</div>')+
                    section('templates','<div class="writer-template-grid">'+['blank','letter','report','meeting'].map(x=>'<button data-action="'+x+'"><span class="writer-template-paper" aria-hidden="true"><i></i><i></i><i></i></span>'+esc(tr(x))+'</button>').join('')+'</div>');
                case 'review':return '<div class="writer-review-mode">'+field('tracking',select('tracking',[['editing','editing'],['suggesting','tracking']]))+
                    field('author',input('author',app.author,'text','maxlength="100"'))+'</div>'+
                    '<p class="writer-hint">'+esc(tr('tracking_hint'))+'</p><div class="writer-buttonrow">'+textButton('comment_add','comment')+textButton('accept_all')+textButton('reject_all')+'</div><div data-review></div>';
                case 'assist':return '<p class="writer-hint">'+esc(tr('assist_hint'))+'</p>'+textButton('paragraph_select','select_paragraph')+
                    field('ai_action',select('aiAction',['rewrite','shorten','expand','correct','translate','custom'].map(x=>[x,x])))+
                    field('language',input('aiLanguage','','text','placeholder="Deutsch"'))+
                    field('instruction','<textarea data-field="aiInstruction" rows="3" maxlength="2048"></textarea>')+
                    '<div class="writer-buttonrow">'+textButton('ai_generate','generate')+textButton('ai_cancel','cancel')+'</div><div data-suggestion aria-live="polite"></div>';
            }
            return '';
        }
        function value(key){return find('[data-field="'+key+'"]')?.value;}
        function checked(key){return !!find('[data-field="'+key+'"]')?.checked;}
        function update(left,right) {
            const editor=app.editor;if(!editor)return;
            for(const button of root.querySelectorAll('[data-command]')) {
                const command=JSON.parse(button.dataset.command);
                const allowed=app.can(command);
                button.disabled=!allowed.ok;
                button.title=allowed.ok?'':tr('command_unavailable')+(allowed.reason?' · '+allowed.reason:'');
            }
            for(const el of root.querySelectorAll('[data-panel-command="table_insert"],[data-panel-command="hyperlink"],[data-action="image"]')){
                el.disabled=app.readonly || editor.snapshot().editingMode==='suggesting';el.title=el.disabled?tr('tracking_unsupported'):'';
            }
            if(left==='outline') {
                const outline=editor.getOutline(),signature=JSON.stringify(outline);
                if(signature!==outlineSignature) {
                    outlineSignature=signature;
                    find('[data-outline]').innerHTML=outline.length?outline.map(x=>'<button data-block="'+esc(x.blockId)+'" style="--writer-heading-depth:'+Math.min(5,x.level)+'">'+esc(x.text)+'</button>').join(''):'<p class="writer-hint">'+esc(tr('outline_empty'))+'</p>';
                }
            }
            if(left==='search')search(false);
            const snapshot=editor.snapshot(),fmt=snapshot.formatting || {};
            if(right==='format') {
                const setup=snapshot.pageSetup, image=snapshot.image;
                const fields={lineSpacing:fmt.lineSpacing?.value || 1.15,beforePt:fmt.spaceBeforePt || 0,afterPt:fmt.spaceAfterPt || 0,
                    orientation:setup.orientation,paper:Math.abs(setup.pageWidthTwips-12240)<20?'letter':'a4'};
                for(const direction of ['Top','Bottom','Left','Right'])fields['margin'+direction]=Math.round(setup.marginsTwips[direction.toLowerCase()]*25.4/1440);
                for(const [key,v] of Object.entries(fields)) {
                    const el=find('[data-field="'+key+'"]');if(el && el!==document.activeElement)el.value=v;
                }
                for(const el of root.querySelectorAll('.writer-right input,.writer-right select')){
                    const key=el.dataset.field, tracking=snapshot.editingMode==='suggesting';
                    el.disabled=app.readonly || (tracking && ['paper','orientation','marginTop','marginBottom','marginLeft','marginRight','cellFill','imageWidth','imageAlt','imageWrap'].includes(key));
                    el.title=el.disabled?tr('command_unavailable'):'';
                }
                for(const key of ['cellFill']){const el=find('[data-field="'+key+'"]');if(el)el.disabled=app.readonly || snapshot.editingMode==='suggesting' || !editor.getSelectedTable();}
                for(const key of ['imageWidth','imageAlt','imageWrap']){const el=find('[data-field="'+key+'"]');if(el)el.disabled=app.readonly || snapshot.editingMode==='suggesting' || !image;}
            }
            if(right==='review') {
                const mode=find('[data-field="tracking"]');if(mode)mode.value=snapshot.editingMode;
                const items=editor.getReviewItems({placement:false}),signature=JSON.stringify(items.map(x=>[x.key,x.text,x.resolved,x.isActive,x.replyIds]));
                if(signature!==reviewSignature){reviewSignature=signature;renderReview(items);}
            }
            if(right==='assist' && suggestion && suggestion.revision!==app.revision)renderSuggestion();
            for(const el of root.querySelectorAll('[data-panel-command="comment_add"],[data-panel-command="accept_all"],[data-panel-command="reject_all"],[data-field="tracking"],[data-panel-command="replace_one"],[data-panel-command="replace_all"],[data-panel-command="ai_generate"]'))el.disabled=app.readonly;
        }
        function search(navigate) {
            const query=value('query') || '',signature=JSON.stringify([query,checked('matchCase'),checked('wholeWord'),app.revision]);
            if(signature!==searchSignature) {
                searchSignature=signature;matches=query?app.editor.findMatches(query,{matchCase:checked('matchCase'),wholeWord:checked('wholeWord')}):[];
                matchIndex=matches.length?Math.min(Math.max(0,matchIndex),matches.length-1):-1;
            }
            const node=find('[data-matches]');if(node)node.textContent=matches.length?tr('match_count',{current:matchIndex+1,total:matches.length}):tr('no_results');
            if(navigate && matches[matchIndex])app.editor.selectMatch(matches[matchIndex]);
        }
        function renderReview(items) {
            const node=find('[data-review]');if(!node)return;
            if(!items.length){node.innerHTML='<div class="writer-empty">'+esc(tr('review_empty'))+'</div>';return;}
            const card=(item,reply=false)=>'<article class="writer-review-card '+(reply?'writer-reply':'')+'" data-review-key="'+esc(item.key)+'"><header><button data-review-op="activate" '+(!item.activatable?'disabled':'')+'><span class="writer-avatar">'+esc(item.initials || '?')+'</span><strong>'+esc(item.author)+'</strong></button><time>'+esc(item.date?new Date(item.date).toLocaleDateString():'')+'</time></header>'+
                '<small>'+esc(tr(item.kind==='revision'?'change_'+item.revisionKind:item.resolved?'resolved':'comment'))+'</small>'+
                (item.replacedText?'<del>'+esc(item.replacedText)+'</del>':'')+'<p>'+esc(item.text)+'</p>'+
                (item.readOnly?'<p class="writer-hint">'+esc(tr('review_readonly'))+'</p>':'<div class="writer-buttonrow">'+
                (item.kind==='revision'?'<button data-review-op="accept">'+esc(tr('accept'))+'</button><button data-review-op="reject">'+esc(tr('reject'))+'</button>':'<button data-review-op="resolve">'+esc(tr(item.resolved?'reopen':'resolve'))+'</button>')+
                '<button data-review-op="reply">'+esc(tr('reply'))+'</button><button data-review-op="delete">'+esc(tr('delete'))+'</button></div>')+
                (!reply?items.filter(x=>item.replyIds?.includes(x.id)).map(x=>card(x,true)).join(''):'')+'</article>';
            node.innerHTML=items.filter(x=>!x.parentId && !x.parentRevisionId).map(x=>card(x)).join('');
            if(app.readonly)for(const el of node.querySelectorAll('[data-review-op]:not([data-review-op="activate"])'))el.disabled=true;
        }
        async function reviewAction(button) {
            const key=button.closest('[data-review-key]').dataset.reviewKey,editor=app.editor,operation=button.dataset.reviewOp;
            const item=editor.getReviewItems({placement:false}).find(x=>x.key===key);
            if(!item)return;
            let result;
            if(operation==='activate')result=editor.setActiveReviewItem(key);
            if(operation==='accept')result=editor.acceptReviewItem(key);
            if(operation==='reject')result=editor.rejectReviewItem(key);
            if(operation==='resolve')result=editor.setCommentResolved(key,!item.resolved);
            if(operation==='delete' && await app.ctx.confirmDialog(tr('delete'),tr('delete_comment')))result=editor.deleteReviewItem(key);
            if(operation==='reply'){const text=await app.prompt('reply');if(text?.trim())result=editor.replyToReviewItem(key,text,app.author);}
            if(result?.ok===false)app.notice(result.reason || tr('command_unavailable'),true);
            reviewSignature='';app.refresh();
        }
        async function generate() {
            if(app.readonly)return;
            aiRequest?.abort();suggestion=null;
            const editor=app.editor, text=editor.query({type:'selectedText'});
            if(!text?.trim()){app.notice(tr('select_text'),true);return;}
            const selection=structuredClone(editor.surface.state().selection),revision=app.revision;
            aiRequest=new AbortController();const request=aiRequest;
            find('[data-suggestion]').textContent=tr('generating');
            try {
                const response=await fetch('/api/desktop/office/assist',{method:'POST',headers:{'Content-Type':'application/json'},signal:request.signal,
                    body:JSON.stringify({action:value('aiAction'),text,context:'',instruction:value('aiInstruction') || '',language:value('aiLanguage') || '',source_revision:revision})});
                const result=await response.json();
                if(!response.ok)throw new Error(result.error || tr('assist_failed'));
                if(request!==aiRequest || editor!==app.editor)return;
                if(result.source_revision!==revision)throw new Error(tr('stale_suggestion'));
                suggestion={revision,selection,original:text,text:result.replacement};renderSuggestion();
            } catch(error) {
                if(error.name!=='AbortError'){app.notice(error.message,true);if(find('[data-suggestion]'))find('[data-suggestion]').textContent=tr('assist_failed');}
            }
        }
        function renderSuggestion() {
            const node=find('[data-suggestion]');if(!node)return;
            if(!suggestion){node.replaceChildren();return;}
            const stale=suggestion.revision!==app.revision;
            node.innerHTML=section('original','<div class="writer-suggestion-original">'+esc(suggestion.original)+'</div>')+
                section('suggestion','<div class="writer-suggestion-text">'+esc(suggestion.text)+'</div>')+
                (stale?'<p class="writer-hint">'+esc(tr('stale_suggestion'))+'</p>':'')+
                '<button class="writer-primary" data-panel-command="ai_apply" '+(stale || app.readonly?'disabled':'')+'>'+esc(tr('apply'))+'</button>';
        }
        async function command(id) {
            const editor=app.editor;if(!editor)return;
            switch(id) {
                case 'previous':case 'next':search(false);if(matches.length){matchIndex=(matchIndex+(id==='next'?1:-1)+matches.length)%matches.length;search(true);}break;
                case 'replace_one':if(matches[matchIndex])app.run({type:'replaceMatch',match:matches[matchIndex],text:value('replacement') || ''});searchSignature='';search(true);break;
                case 'replace_all':app.run({type:'replaceAllMatches',query:value('query'),text:value('replacement') || '',matchCase:checked('matchCase'),wholeWord:checked('wholeWord')});searchSignature='';search(false);break;
                case 'table_insert':{const answer=await app.prompt('table_dimensions','3 × 3');if(!answer)return;const [rows,cols]=answer.split(/[x×,;\s]+/).map(Number);if(rows>=1 && rows<=100 && cols>=1 && cols<=30)app.run({type:'insertTable',rows,cols});else app.notice(tr('table_dimensions'),true);break;}
                case 'comment_add':{const text=await app.prompt('comment');if(text?.trim()){const result=editor.addComment(text,app.author);if(!result.ok)app.notice(result.reason || tr('select_text'),true);}break;}
                case 'accept_all':case 'reject_all':{
                    const items=editor.getReviewItems({placement:false}).filter(x=>x.kind==='revision');
                    if(items.some(x=>x.readOnly)){app.notice(tr('review_readonly'),true);break;}
                    for(const item of items){
                        const result=id==='accept_all'?editor.acceptReviewItem(item.key):editor.rejectReviewItem(item.key);
                        if(!result.ok){app.notice(result.reason || tr('command_unavailable'),true);break;}
                    }
                    break;
                }
                case 'hyperlink':{const url=await app.prompt('hyperlink','https://');if(url)app.run({type:'insertHyperlink',href:url});break;}
                case 'paragraph_select':{
                    const selection=editor.surface.state().selection;if(!selection)return;
                    const id=selection.head.paragraphId, ids=editor.surface.session.paragraphIds();
                    const paragraph=editor.query({type:'paragraphs'})[ids.indexOf(id)];
                    if(!paragraph){app.notice(tr('select_text'),true);return;}
                    app.run({type:'setSelection',range:{anchor:{paragraphId:id,offset:0},head:{paragraphId:id,offset:paragraph.text.length}}});
                    app.pinSelection();break;
                }
                case 'ai_generate':await generate();break;
                case 'ai_cancel':aiRequest?.abort();suggestion=null;renderSuggestion();break;
                case 'ai_apply':
                    if(!app.readonly && suggestion && suggestion.revision===app.revision) {
                        if(app.run({type:'setSelection',range:suggestion.selection}))app.run({type:'paste',text:suggestion.text});
                        suggestion=null;renderSuggestion();
                    } else app.notice(tr('stale_suggestion'),true);
                    break;
            }
            app.refresh();
        }
        function change(el) {
            const key=el.dataset.field,editor=app.editor,v=el.value,n=Number(v);
            if(!editor)return;
            if(['query','matchCase','wholeWord'].includes(key)){searchSignature='';search(true);return;}
            if(key==='tracking'){app.run({type:'setEditingMode',mode:v});return;}
            if(key==='author'){if(v.trim())app.author=v.trim().slice(0,100);return;}
            if(key==='text.color' || key==='text.highlight'){app.run(app.lib.commandForSlotValue(key,v));return;}
            if(key==='lineSpacing')app.run({type:'setLineSpacing',rule:'multiple',value:n});
            if(key==='beforePt' || key==='afterPt')app.run({type:'setParagraphSpacing',[key]:n});
            if(key==='leftIndent' || key==='firstIndent')app.run({type:'setIndent',[key==='leftIndent'?'left':'firstLine']:Math.round(n*1440/25.4)});
            if(key?.startsWith('margin'))app.run({type:'setPageSetup',[key]:Math.round(n*1440/25.4),scope:'document'});
            if(key==='orientation')app.run({type:'setPageSetup',orientation:v});
            if(key==='paper') {
                let [pageWidth,pageHeight]=v==='letter'?[12240,15840]:[11906,16838];
                if(editor.snapshot().pageSetup.orientation==='landscape')[pageWidth,pageHeight]=[pageHeight,pageWidth];
                app.run({type:'setPageSetup',pageWidth,pageHeight});
            }
            if(key==='cellFill')app.run({type:'setCellFill',color:{kind:'hex',value:v.replace('#','')}});
            if(key==='imageWrap')app.run({type:'setImageWrapType',target:v});
            if(key==='imageAlt')app.run({type:'setImageProperties',alt:v,description:v});
            if(key==='imageWidth') {
                const image=editor.getSelectedImage();
                if(image)app.run({type:'setImageProperties',widthEmu:n*36000,heightEmu:n*36000*image.heightEmu/image.widthEmu});
            }
        }
        root.addEventListener('click',event=>{
            const target=event.target.closest('[data-command],[data-panel-command],[data-review-op],[data-block]');if(!target || target.disabled)return;
            if(target.dataset.command)app.run(JSON.parse(target.dataset.command));
            if(target.dataset.block)app.editor?.scrollToBlock(target.dataset.block);
            if(target.dataset.panelCommand)command(target.dataset.panelCommand).catch(error=>app.notice(error.message,true));
            if(target.dataset.reviewOp)reviewAction(target).catch(error=>app.notice(error.message,true));
        },{signal:life.signal});
        root.addEventListener('change',event=>change(event.target),{signal:life.signal});
        root.addEventListener('input',event=>{if(event.target.dataset.field==='query'){searchSignature='';search(true);}},{signal:life.signal});
        root.addEventListener('keydown',event=>{if(event.key==='Enter' && event.target.dataset.field==='query'){event.preventDefault();command(event.shiftKey?'previous':'next');}},{signal:life.signal});
        function template(kind) {
            const date=new Date().toLocaleDateString(document.documentElement.lang || 'en');
            if(kind==='letter')return '<p>'+esc(tr('template_sender'))+'</p><p>'+esc(tr('template_recipient'))+'</p><p>'+esc(date)+'</p><h1>'+esc(tr('template_subject'))+'</h1><p>'+esc(tr('template_greeting'))+'</p><p>'+esc(tr('template_body'))+'</p><p>'+esc(tr('template_closing'))+'</p>';
            if(kind==='report')return '<h1>'+esc(tr('report'))+'</h1><p>'+esc(date)+'</p><h2>'+esc(tr('summary'))+'</h2><p>'+esc(tr('template_body'))+'</p><h2>'+esc(tr('findings'))+'</h2><p>'+esc(tr('template_body'))+'</p><h2>'+esc(tr('next_steps'))+'</h2><ul><li>'+esc(tr('template_task'))+'</li></ul>';
            return '<h1>'+esc(tr('meeting'))+'</h1><p>'+esc(date)+'</p><h2>'+esc(tr('participants'))+'</h2><p>…</p><h2>'+esc(tr('agenda'))+'</h2><ol><li>'+esc(tr('template_subject'))+'</li></ol><h2>'+esc(tr('decisions'))+'</h2><p>…</p><h2>'+esc(tr('next_steps'))+'</h2><table><tr><td>'+esc(tr('task'))+'</td><td>'+esc(tr('owner'))+'</td><td>'+esc(tr('due'))+'</td></tr><tr><td>…</td><td>…</td><td>…</td></tr></table>';
        }
        async function print(bytes,lib,fonts) {
            // A separate snapshot keeps edits and review state out of the print transaction.
            printFrame?.remove();printFrame=document.createElement('iframe');printFrame.className='writer-print-frame';
            document.body.appendChild(printFrame);
            const doc=printFrame.contentDocument;
            doc.open();doc.write('<!doctype html><html><head><link rel="stylesheet" href="'+location.origin+'/js/vendor/writer/engine.css"></head><body><div class="docx-editor" id="pages"></div></body></html>');doc.close();
            await new Promise((resolve,reject)=>{printFrame.onload=resolve;setTimeout(resolve,500);});
            const printEditor=lib.createDocxEditor({container:doc.getElementById('pages'),document:bytes,fonts,mode:'view',zoom:1,modules:[lib.reviewModule]});
            try {
                await printEditor.save();
                for (const face of document.fonts) doc.fonts.add(face);
                await doc.fonts.ready;
                lib.paintSemanticLayout(doc.getElementById('pages'), printEditor.surface.layout(), {scale:96/72,ariaHidden:false,defaultFontFamily:'Calibri'});
                const setup=printEditor.snapshot().pageSetup;
                const style=doc.createElement('style');
                style.textContent='@page{size:'+setup.pageWidthTwips/20+'pt '+setup.pageHeightTwips/20+'pt;margin:0}html,body,#pages{margin:0!important;padding:0!important;height:auto!important;overflow:visible!important;background:white!important}.docx-page{position:relative!important;top:auto!important;left:auto!important;transform:none!important;break-after:page!important;break-inside:avoid!important;margin:0!important;box-shadow:none!important}.docx-page:last-child{break-after:auto!important}';
                doc.head.appendChild(style);
                printFrame.contentWindow.focus();printFrame.contentWindow.print();
            } finally {printEditor.destroy();setTimeout(()=>{printFrame?.remove();printFrame=null;},1000);}
        }
        function reset(){aiRequest?.abort();suggestion=null;renderSuggestion();matches=[];reviewSignature=outlineSignature=searchSignature='';}
        return {show,update,template,print,reset,dispose(){life.abort();reset();printFrame?.remove();}};
    }
    window.WriterPanels={create};
})();
