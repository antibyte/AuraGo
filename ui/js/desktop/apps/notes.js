(function () {
    'use strict';
    const instances = new Map(), DIR = 'Documents/Notes', META = DIR + '/notes.meta.json';
    const basename = path => String(path || '').split('/').pop();
    const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    async function request(url, options = {}) {
        const response = await fetch(url, {credentials:'same-origin',cache:'no-store',...options});
        const body = await response.json().catch(()=>({}));
        if (!response.ok) { const error = new Error(body.error || body.message || ('HTTP '+response.status)); error.status=response.status; throw error; }
        return body;
    }
    const endpoint = path => '/api/desktop/notes' + (path?'?path='+encodeURIComponent(path):'');
    function render(host, windowId, ctx = {}) {
        dispose(windowId);
        const esc=ctx.esc||escapeHTML, tr=(key,params)=>{
            for(const prefix of ['notes_','writer_','']){const id='desktop.'+prefix+key,value=ctx.t?.(id,params);if(value&&value!==id)return value;}
            return key;
        };
        const icons={library:'columns',new:'file-plus',search:'search',pin:'bookmark',info:'info',outline:'list',assist:'star',source:'file-code',focus:'maximize',insert:'plus',
            save:'save',saveAs:'copy',rename:'pencil',move:'folder',trash:'trash',restore:'undo',open:'folder-open',print:'printer',md:'notes',html:'file-code',txt:'file-text',
            undo:'undo',redo:'redo',bold:'format-bold',italic:'format-italic',strike:'format-strike',bullet:'list',ordered:'format-numbered',quote:'format-quote',code:'code',link:'link',close:'x',closePanel:'x',dismiss:'x',cancelAssist:'x',task:'check-square'};
        const icon=key=>ctx.iconMarkup?.(icons[key]||key,'','notes-icon',18,'action')||'<span aria-hidden="true">•</span>';
        const button=(action,label=action,text=false)=>'<button type="button" data-action="'+action+'" title="'+esc(tr(label))+'" aria-label="'+esc(tr(label))+'">'+(text?esc(tr(label)):icon(action))+'</button>';
        host.innerHTML='<div class="vd-notes-app" data-notes="'+esc(windowId)+'">'+
            '<header class="notes-documentbar"><div class="notes-document">'+button('library')+'<button class="notes-name" data-action="document"><span data-name>'+esc(tr('title'))+'</span><span aria-hidden="true">⌄</span></button><span data-save-state role="status" aria-live="polite"></span></div><nav aria-label="'+esc(tr('panels'))+'">'+button('pin')+button('outline')+button('info')+button('assist')+'</nav></header>'+
            '<div class="vd-notes-toolbar" role="toolbar" aria-label="'+esc(tr('format'))+'"><div class="notes-toolgroup">'+button('undo')+button('redo')+'</div><div class="notes-toolgroup"><select data-style aria-label="'+esc(tr('style'))+'"><option value="p">'+esc(tr('paragraph'))+'</option>'+[1,2,3].map(n=>'<option value="'+n+'">'+esc(tr('heading'))+' '+n+'</option>').join('')+'</select></div><div class="notes-toolgroup">'+button('bold')+button('italic')+button('strike')+'</div><div class="notes-toolgroup">'+button('bullet','bullets')+button('ordered','numbering')+button('task')+button('quote')+'</div><div class="notes-toolgroup">'+button('link')+button('code')+button('insert')+'</div><span class="notes-spacer"></span>'+button('source','markdown_source')+'</div>'+
            '<div class="notes-notice" data-notice role="alert" hidden><span data-notice-text></span>'+button('retry','retry',true)+button('saveAs','save_as',true)+button('dismiss','close')+'</div>'+
            '<div class="notes-workspace"><aside class="notes-library" aria-label="'+esc(tr('library'))+'"><div class="notes-library-head"><strong>'+esc(tr('title'))+'</strong>'+button('new','new_note')+'</div><div class="notes-library-filters"><input type="search" data-search placeholder="'+esc(tr('search_all'))+'" aria-label="'+esc(tr('search_all'))+'"><div class="notes-filter-row"><select data-folder aria-label="'+esc(tr('folder'))+'"><option value="">'+esc(tr('all_notes'))+'</option></select><select data-sort aria-label="'+esc(tr('sort'))+'"><option value="modified">'+esc(tr('recent'))+'</option><option value="name">'+esc(tr('name'))+'</option></select></div><input data-tag placeholder="'+esc(tr('filter_tag'))+'" aria-label="'+esc(tr('filter_tag'))+'"></div><div class="notes-list" data-list role="list" aria-label="'+esc(tr('title'))+'"></div><div class="notes-library-footer"><button data-action="showTrash">'+icon('trash')+' '+esc(tr('trash'))+'</button><span data-total></span></div></aside>'+
            '<main class="notes-main"><div class="notes-scroll" data-scroll><div class="notes-page"><div class="notes-note-meta" data-note-meta></div><div data-editor></div></div></div><div class="notes-empty" data-empty><div class="notes-empty-icon">'+icon('md')+'</div><h2>'+esc(tr('empty_title'))+'</h2><p>'+esc(tr('empty_description'))+'</p>'+button('new','new_note',true)+'</div><div class="notes-loading" data-loading hidden>'+esc(tr('loading'))+'</div></main><aside class="notes-right" data-right hidden></aside></div>'+
            '<footer class="notes-statusbar"><span data-count></span><span class="notes-spacer"></span><span data-mode></span>'+button('focus','focus_mode')+'<button data-action="zoomOut" aria-label="'+esc(tr('zoom_out'))+'">−</button><button data-action="zoomReset" data-zoom>100%</button><button data-action="zoomIn" aria-label="'+esc(tr('zoom_in'))+'">+</button></footer><div class="notes-popup" data-popup hidden></div><input data-upload type="file" multiple hidden><input data-import type="file" accept=".md,.markdown,.txt,text/markdown,text/plain" hidden></div>';
        const root=host.firstElementChild, find=selector=>root.querySelector(selector), life=new AbortController();
        let docLife=new AbortController(), editor, queue, current, disposed=false, busy=false, loading=false, zoom=100, right='', searchTimer, updateTimer, draftTimer, draftDeadline, sseTimer;
        let meta={version:1,pinned:[],sort:'modified',last_note:''}, list=[], total=0, trash=false, page=0, listGeneration=0, currentDraft, assistController, proposal;
        let metaTask=Promise.resolve(), sourceRequested=false;
        const draftKey=()=>location.origin+':'+current.path;
        const instance={act,get editor(){return editor;},get session(){return queue;},get path(){return current?.path;},dispose:cleanup};
        instances.set(windowId,instance);
        ctx.registerWindowCleanup?.(windowId,cleanup);ctx.setWindowBeforeClose?.(windowId,guard);ctx.wireContextMenuBoundary?.(host);
        const on=(target,event,fn)=>target.addEventListener(event,fn,{signal:life.signal});
        function notice(message,error=false){if(disposed)return;find('[data-notice-text]').textContent=message||'';find('[data-notice]').hidden=!message;find('[data-notice]').dataset.error=error;}
        function fail(error){if(disposed||error?.name==='AbortError')return;notice(tr(error.status===412?'conflict':error.status===428?'conflict':'request_failed'),true);}
        function saveStatus(state={}) {
            const value=state.error?'error':state.saving?'saving':state.dirty?'unsaved':current?'saved':'';
            find('[data-save-state]').textContent=value?tr(value):'';find('[data-save-state]').dataset.state=value;
            if(state.error)fail(state.error);
        }
        function writable(){return editor&&!ctx.readonly&&!current?.path.startsWith('Trash/')&&!busy;}
        function syncChrome(){
            find('[data-editor]').inert=busy||loading;
            find('[data-name]').textContent=current?.title||tr('title');
            find('[data-name]').title=current?.path||'';
            find('[data-note-meta]').textContent=current?basename(current.path)+' · '+new Date(current.modified).toLocaleDateString(document.documentElement.lang||'en',{day:'numeric',month:'long',year:'numeric'}):'';
            find('[data-empty]').hidden=!!editor;find('[data-scroll]').hidden=!editor;
            find('[data-mode]').textContent=editor?tr(editor.sourceMode?'markdown_source':'formatted'):'';
            find('[data-style]').disabled=!writable()||!!editor?.sourceMode;
            root.querySelectorAll('.vd-notes-toolbar [data-action]').forEach(el=>{el.disabled=!editor || (el.dataset.action!=='source'&&(!writable()||(editor.sourceMode&&!['undo','redo'].includes(el.dataset.action))));});
            root.querySelectorAll('[data-action="pin"]').forEach(el=>{el.disabled=!current||ctx.readonly||trash;el.setAttribute('aria-pressed',String(meta.pinned.includes(current?.path)));});
            find('[data-action="source"]').setAttribute('aria-pressed',String(!!editor?.sourceMode));
            root.querySelectorAll('[data-action="new"]').forEach(el=>el.disabled=!!ctx.readonly||busy);
            menus();updateDetails();
        }
        function updateDetails(){
            if(!editor||disposed)return;
            const text=editor.text,words=text.match(/[\p{L}\p{N}]+(?:['’\-][\p{L}\p{N}]+)*/gu)||[];
            find('[data-count]').textContent=tr('counts',{words:words.length,chars:[...text].length});
            if(current) {current.title=NotesFrontmatter.deriveTitle(editor.content(),basename(current.path));find('[data-name]').textContent=current.title;const row=find('[data-list]').querySelector('[aria-current="true"] strong');if(row)row.textContent=current.title;}
            if(right==='outline')renderRight();
            if(proposal&&proposal.revision!==queue?.revision){find('[data-apply]')?.setAttribute('disabled','');find('[data-ai-state]')?.replaceChildren(document.createTextNode(tr('stale_suggestion')));}
        }
        function changed(){
            if(loading||disposed)return;
            queue?.changed();clearTimeout(updateTimer);updateTimer=setTimeout(updateDetails,160);
            clearTimeout(draftTimer);draftTimer=setTimeout(backup,600);if(!draftDeadline)draftDeadline=setTimeout(backup,4000);
        }
        async function backup(content=editor?.content(),revision=queue?.revision){
            clearTimeout(draftTimer);clearTimeout(draftDeadline);draftTimer=draftDeadline=null;
            if(!current||!editor||!queue?.dirty||disposed)return !queue?.dirty;
            const value={id:crypto.randomUUID(),content,revision,etag:current.version,updated:Date.now(),path:current.path};
            currentDraft=value;
            try{await OfficeSession.draft('put',draftKey(),value,'notes');return true;}catch(_){notice(tr('draft_unavailable'),true);return false;}
        }
        async function clearBackup(){
            if(!currentDraft)return;
            const key=draftKey(),id=currentDraft.id;
            await OfficeSession.draft('deleteIf',key,{id},'notes');
            if(currentDraft?.id===id)currentDraft=null;
        }
        function makeQueue(){
            queue=OfficeSession.create({readonly:ctx.readonly||current.path.startsWith('Trash/'),serialize:()=>editor.content(),backup,
                write:async content=>{
                    const saved=await request(endpoint(current.path),{method:'PUT',signal:docLife.signal,headers:{'Content-Type':'application/json','If-Match':current.version},body:JSON.stringify({path:current.path,content})});
                    current.version=saved.version;current.modified=saved.modified;current.content=content;
                },clearBackup,onState:saveStatus});
        }
        async function guard(){
            if(busy||loading)return false;
            if(!queue?.dirty)return true;
            try{await queue.save();return !queue.dirty;}catch(error){
                fail(error);if(!await backup())return false;
                return !!(await ctx.confirmDialog?.(tr('unsaved'),tr('leave_draft')));
            }
        }
        async function loadNote(path, supplied) {
            if(disposed||busy||!await guard())return;
            busy=true;loading=true;find('[data-loading]').hidden=false;syncChrome();
            try {
                const note=supplied||await request(endpoint(path),{signal:life.signal});
                if(disposed)return;
                let draft=await OfficeSession.draft('get',location.origin+':'+note.path,undefined,'notes').catch(()=>null);
                if(draft?.content===note.content){await OfficeSession.draft('delete',location.origin+':'+note.path,undefined,'notes').catch(()=>{});draft=null;}
                const recovered=draft&&!ctx.readonly&&!note.path.startsWith('Trash/')&&await ctx.confirmDialog?.(tr('recover'),tr('recover_prompt'));
                const next=await NotesEditor.create(document.createElement('div'),{content:recovered?draft.content:note.content,readonly:ctx.readonly||note.path.startsWith('Trash/'),tr,
                    upload:file=>uploadFile(file,true),resolveURL:href=>resolveURL(href,note.path),openLink:openLink,changed});
                if(disposed){await next.destroy();return;}
                queue?.dispose();docLife.abort();docLife=new AbortController();clearTimeout(draftTimer);clearTimeout(draftDeadline);
                await editor?.destroy();editor=next;current=note;currentDraft=recovered?draft:null;
                find('[data-editor]').replaceChildren(next.view.dom.closest('.notes-rich').parentElement);
                if(sourceRequested&&!editor.sourceMode)await editor.mode(true);
                if(recovered&&draft.etag!==note.version)current.version=draft.etag;
                makeQueue();loading=false;
                if(recovered)queue.changed();else saveStatus();
                find('[data-scroll]').scrollTop=0;proposal=null;assistController?.abort();
                ctx.updateWindowContext?.(windowId,{path:note.path});ctx.recordRecentFile?.(note.path,'notes');
                if(editor.sourceMode&&NotesEditor.needsSource(editor.content()))notice(tr('source_preserved'));
                else notice(recovered?tr('recovered'):'');
                if(!trash)updateMeta({last_note:note.path});
                renderList();renderRight();if(root.classList.contains('notes-narrow'))root.classList.add('notes-hide-library');
            }catch(error){fail(error);}
            finally{loading=false;busy=false;if(!disposed){find('[data-loading]').hidden=true;syncChrome();}}
        }
        async function importDesktopFile(path){
            if(ctx.readonly)return;
            if(!await ctx.confirmDialog?.(tr('import'),tr('import_copy')))return;
            const file=await request('/api/desktop/file?path='+encodeURIComponent(path),{signal:life.signal});
            if(typeof file.content!=='string'||new TextEncoder().encode(file.content).length>2*1024*1024){notice(tr('note_too_large'),true);return;}
            await createNote('blank',file.content);
        }
        function template(kind){
            const title=tr('template_'+kind);
            if(kind==='meeting')return '# '+title+'\n\n'+new Date().toLocaleDateString()+'\n\n## '+tr('agenda')+'\n\n- \n\n## '+tr('decisions')+'\n\n## '+tr('tasks')+'\n\n- [ ] \n';
            if(kind==='checklist')return '# '+title+'\n\n- [ ] \n- [ ] \n';
            if(kind==='journal')return '# '+new Date().toLocaleDateString()+'\n\n## '+tr('thoughts')+'\n\n## '+tr('next_steps')+'\n';
            return '# '+tr('new_note')+'\n\n';
        }
        async function createNote(kind='blank',content){
            if(ctx.readonly||busy||!await guard())return;
            busy=true;syncChrome();
            try{const text=content??template(kind);const note=await request(endpoint(),{method:'POST',signal:life.signal,headers:{'Content-Type':'application/json'},body:JSON.stringify({title:NotesFrontmatter.deriveTitle(text,tr('new_note')),content:text,folder:find('[data-folder]').value||DIR})});
                busy=false;trash=false;await refreshList();await loadNote(note.path,note);editor?.focus();
            }catch(error){fail(error);}finally{busy=false;syncChrome();}
        }
        async function refreshList(append=false){
            const gen=++listGeneration,query=new URLSearchParams({q:find('[data-search]').value,tag:find('[data-tag]').value,folder:find('[data-folder]').value,trash:String(trash),offset:String(page*200),limit:'200'});
            const result=await request(endpoint()+'?'+query,{signal:life.signal});
            if(disposed||gen!==listGeneration)return;
            list=append?list.concat(result.notes||[]):result.notes||[];total=result.total;
            const folder=find('[data-folder]'),old=folder.value;folder.innerHTML='<option value="">'+esc(tr('all_notes'))+'</option>'+(result.folders||[]).map(path=>'<option value="'+esc(path)+'">'+esc(path.slice(DIR.length+1))+'</option>').join('');folder.value=old;
            find('[data-total]').textContent=String(total);find('[data-action="showTrash"]').setAttribute('aria-pressed',String(trash));renderList();
        }
        function renderList(){
            const rows=[...list].sort((a,b)=>Number(meta.pinned.includes(b.path))-Number(meta.pinned.includes(a.path))||(meta.sort==='name'?a.title.localeCompare(b.title):new Date(b.modified)-new Date(a.modified)));
            find('[data-list]').innerHTML=rows.map(note=>'<button class="notes-list-row" role="listitem" data-note-path="'+esc(note.path)+'" aria-current="'+String(note.path===current?.path)+'"><span class="notes-list-title"><strong>'+esc(note.title)+'</strong>'+(meta.pinned.includes(note.path)?icon('pin'):'')+'</span><span class="notes-list-snippet">'+esc(note.snippet||tr('empty_note'))+'</span><span class="notes-list-meta"><time>'+esc(new Date(note.modified).toLocaleDateString(document.documentElement.lang||'en',{month:'short',day:'numeric'}))+'</time><span>'+esc((note.tags||[]).map(tag=>'#'+tag).join(' '))+'</span></span></button>').join('')+(list.length<total?button('more','load_more',true):!rows.length?'<p class="notes-list-empty">'+esc(tr('no_results'))+'</p>':'');
        }
        function updateMeta(change){
            if(ctx.readonly)return;
            Object.assign(meta,change);renderList();syncChrome();
            metaTask=metaTask.catch(()=>{}).then(async()=>{
                for(let attempt=0;attempt<2;attempt++){
                    const currentMeta=await request(endpoint(META),{signal:life.signal}).catch(error=>{if(error.status===404)return null;throw error;});
                    const value={...JSON.parse(currentMeta?.content||'{}'),...change,version:1};
                    try{await request(endpoint(META),{method:'PUT',signal:life.signal,headers:{'Content-Type':'application/json',...(currentMeta?{'If-Match':currentMeta.version}:{'If-None-Match':'*'})},body:JSON.stringify({path:META,content:JSON.stringify(value)})});return;}
                    catch(error){if(error.status!==412||attempt)throw error;}
                }
            }).catch(fail);
        }
        async function saveAs(){
            if(!editor||ctx.readonly||busy)return;
            const chosen=await ctx.saveFileDialog?.({title:tr('save_as'),initialPath:DIR,defaultName:basename(current.path),defaultExtension:'.md',filters:[{label:'Markdown',extensions:['.md']}]});
            if(!chosen?.path||chosen.canceled||disposed)return;
            if(!chosen.path.startsWith(DIR+'/')||!chosen.path.toLowerCase().endsWith('.md')){notice(tr('notes_location'),true);return;}
            if(chosen.path===current.path)return queue.save();
            busy=true;syncChrome();queue.suspend();
            try{await queue.pending?.catch(()=>{});const text=await editor.relocateLinks(editor.content(),url=>relocatedURL(url,current.path,chosen.path));
                const note=await request(endpoint(chosen.path),{method:'PUT',signal:life.signal,headers:{'Content-Type':'application/json','If-None-Match':'*'},body:JSON.stringify({path:chosen.path,content:text})});
                queue.dispose();queue=null;busy=false;await loadNote(note.path,note);await refreshList();
            }finally{busy=false;queue?.resume();syncChrome();}
        }
        async function moveNote(operation){
            if(!current||ctx.readonly||busy||!await guard())return;
            let target='', relocated;
            if(operation==='trash'&&!await ctx.confirmDialog?.(tr('trash'),tr('trash_confirm')))return;
            if(operation==='move'){
                target=await ctx.promptDialog?.(tr('move_prompt'),current.path);
                if(!target||target===current.path)return;
                relocated=await editor.relocateLinks(editor.content(),url=>relocatedURL(url,current.path,target));
            }
            busy=true;syncChrome();
            try{const note=await request(endpoint(),{method:'PATCH',signal:life.signal,headers:{'Content-Type':'application/json','If-Match':current.version},body:JSON.stringify({path:current.path,new_path:target,operation})});
                queue.dispose();queue=null;await editor.destroy();editor=null;current=null;busy=false;trash=operation==='trash';await refreshList();await loadNote(note.path,note);
                if(relocated!==undefined&&relocated!==note.content){editor.setContent(relocated);await queue.save();}
            }finally{busy=false;syncChrome();}
        }
        function resolvePath(href,from=current?.path){
            if(!href||/^[a-z][a-z\d+.-]*:/i.test(href)||href.startsWith('/')||href.startsWith('#'))return null;
            if(from?.startsWith('Trash/Notes/'))from=DIR+'/'+from.split('/').slice(3).join('/');
            if(!from)return null;
            const parts=from.split('/').slice(0,-1);
            let decoded;try{decoded=decodeURIComponent(href.split(/[?#]/)[0]);}catch(_){return null;}
            for(const part of decoded.split('/')){if(part==='..')parts.pop();else if(part&&part!=='.')parts.push(part);}
            const path=parts.join('/');return path.startsWith(DIR+'/')?path:null;
        }
        function resolveURL(href,from=current?.path){
            if(!from)return '';
            if(/^https?:\/\//i.test(href))return href;
            const path=resolvePath(href,from);return path?'/api/desktop/download?inline=1&path='+encodeURIComponent(path):'';
        }
        function relativePath(from,to){
            const a=from.split('/').slice(0,-1),b=to.split('/');while(a.length&&a[0]===b[0]){a.shift();b.shift();}return [...a.map(()=> '..'),...b].map(part=>part==='..'?part:encodeURIComponent(part)).join('/');
        }
        function relocatedURL(url,from,to){
            const path=resolvePath(url,from),suffix=url.match(/[?#].*$/)?.[0]||'';
            return path?relativePath(to,path)+suffix:url;
        }
        function openLink(href){
            const path=resolvePath(href);
            if(path&&/\.md$/i.test(path)){loadNote(path);return;}
            const url=path?'/api/desktop/download?path='+encodeURIComponent(path):/^https?:\/\//i.test(href)?href:null;
            if(url)window.open(url,'_blank','noopener,noreferrer');
        }
        async function uploadFile(file,imageOnly=false){
            if(!writable())throw Error(tr('readonly'));
            if(file.size>20*1024*1024)throw Error(tr('attachment_too_large'));
            if(imageOnly&&!['image/png','image/jpeg','image/webp','image/gif'].includes(file.type))throw Error(tr('image_type'));
            const form=new FormData();form.append('path',DIR+'/.attachments');form.append('unique','1');form.append('file',file,crypto.randomUUID().slice(0,8)+'-'+file.name);
            const result=await request('/api/desktop/upload',{method:'POST',body:form,signal:docLife.signal});
            return relativePath(current.path,result.path);
        }
        function popup(items,anchor){
            const el=find('[data-popup]');el.innerHTML=items.map(([action,label])=>button(action,label,true)).join('');el.hidden=false;
            const box=anchor?.getBoundingClientRect(),bounds=root.getBoundingClientRect();el.style.top=Math.min((box?.bottom||bounds.top+48)-bounds.top+4,bounds.height-220)+'px';el.style.left=Math.min(Math.max(8,(box?.left||bounds.left+12)-bounds.left),Math.max(8,bounds.width-230))+'px';
            el.querySelector('button')?.focus({preventScroll:true});
        }
        function renderRight(){
            const panel=find('[data-right]');panel.hidden=!right;
            if(!right)return;
            panel.innerHTML='<header class="notes-panel-heading"><h2>'+esc(tr(right))+'</h2>'+button('closePanel','close')+'</header><div class="notes-panel-content" data-panel-content></div>';
            const content=find('[data-panel-content]');
            if(!editor){content.textContent=tr('select_note');return;}
            if(right==='outline'){
                const headings=editor.outline();content.innerHTML=headings.length?headings.map(h=>'<button class="notes-outline-item" data-jump="'+h.pos+'" style="--level:'+h.level+'">'+esc(h.text)+'</button>').join(''):'<p class="notes-muted">'+esc(tr('outline_empty'))+'</p>';
            }else if(right==='info'){
                const tags=NotesFrontmatter.parse(editor.content()).tags;
                content.innerHTML='<label>'+esc(tr('tags'))+'<input data-tags value="'+esc(tags.join(', '))+'" '+(!writable()?'disabled':'')+'></label><p class="notes-muted">'+esc(tr('tags_hint'))+'</p><dl><dt>'+esc(tr('location'))+'</dt><dd>'+esc(current.path)+'</dd><dt>'+esc(tr('modified'))+'</dt><dd>'+esc(new Date(current.modified).toLocaleString())+'</dd></dl><div class="notes-permissions">'+icon('info')+'<p>'+esc(tr('agent_permissions'))+'</p></div>'+button(trash?'restore':'move',trash?'restore':'move',true)+button('trash','trash',true);
            }else if(right==='search'){
                content.innerHTML='<label>'+esc(tr('find'))+'<input data-find></label><label>'+esc(tr('replace'))+'<input data-replace></label><label class="notes-checkbox"><input type="checkbox" data-case>'+esc(tr('case_sensitive'))+'</label>'+button('findNext','find_next',true)+button('replaceOne','replace',true)+button('replaceAll','replace_all',true)+'<p data-find-count role="status"></p>';
            }else if(right==='assist'){
                content.innerHTML='<p class="notes-muted">'+esc(tr('assist_hint'))+'</p><label>'+esc(tr('selection_scope'))+'<select data-ai-scope><option value="selection">'+esc(tr('selection'))+'</option><option value="paragraph">'+esc(tr('current_paragraph'))+'</option></select></label><label>'+esc(tr('action'))+'<select data-ai-action>'+['rewrite','shorten','expand','correct','translate','custom'].map(action=>'<option value="'+action+'">'+esc(tr(action))+'</option>').join('')+'</select></label><label>'+esc(tr('instruction'))+'<textarea data-instruction rows="3"></textarea></label><label>'+esc(tr('target_language'))+'<input data-language value="'+esc(document.documentElement.lang||'en')+'"></label><div class="notes-panel-actions">'+button('generate','generate',true)+button('cancelAssist','cancel',true)+'</div><p data-ai-state role="status"></p><div data-proposal></div>';
            }
        }
        async function assist(){
            if(!writable())return;
            const selected=editor.selection(find('[data-ai-scope]').value==='paragraph');
            if(!selected.text.trim()){notice(tr('select_text'),true);return;}
            assistController?.abort();assistController=new AbortController();const revision=queue.revision,path=current.path;
            find('[data-ai-state]').textContent=tr('generating');
            try{
                const result=await request('/api/desktop/office/assist',{method:'POST',signal:assistController.signal,headers:{'Content-Type':'application/json'},body:JSON.stringify({action:find('[data-ai-action]').value,text:selected.text,context:'',instruction:find('[data-instruction]').value,language:find('[data-language]').value,source_revision:revision})});
                if(disposed||path!==current.path||right!=='assist')return;
                proposal={revision:result.source_revision,path,selected,text:result.replacement};
                find('[data-proposal]').innerHTML='<h3>'+esc(tr('original'))+'</h3><pre>'+esc(selected.text)+'</pre><h3>'+esc(tr('suggestion'))+'</h3><pre>'+esc(result.replacement)+'</pre><button data-action="apply" data-apply '+(queue.revision!==revision?'disabled':'')+'>'+esc(tr('apply'))+'</button>';
                find('[data-ai-state]').textContent=tr(queue.revision!==revision?'stale_suggestion':'review_suggestion');
            }catch(error){if(error.name!=='AbortError')fail(error);}
        }
        async function exportNote(format){
            if(!editor)return;
            const text=editor.content(),title=current.title||tr('title'),from=current.path;
            if(format==='md'||format==='txt'){download(format==='md'?text:editor.text,'text/plain;charset=utf-8',basename(current.path).replace(/\.md$/i,'')+'.'+format);return;}
            const html=DOMPurify.sanitize(marked.parse(NotesFrontmatter.strip(text)),{FORBID_TAGS:['style','iframe','form','input','script'],FORBID_ATTR:['style']});
            const wrapper=document.createElement('div');wrapper.innerHTML=html;
            for(const img of wrapper.querySelectorAll('img')){
                const url=resolveURL(img.getAttribute('src'),from);if(!url){img.remove();continue;}
                if(url.startsWith('/api/')){
                    const response=await fetch(url,{credentials:'same-origin',signal:life.signal});
                    if(!response.ok)throw Error('Attachment export failed');
                    const blob=await response.blob();if(blob.size>20*1024*1024||!/^image\/(png|jpeg|webp|gif)$/.test(blob.type))throw Error('Unsupported attachment export');
                    img.src=await new Promise((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(reader.result);reader.onerror=()=>reject(reader.error);reader.readAsDataURL(blob);});
                }else img.src=url;
            }
            for(const link of wrapper.querySelectorAll('a')){
                const href=link.getAttribute('href');if(href?.startsWith('#'))continue;
                const url=resolveURL(href,from);if(url)link.href=new URL(url,location.origin).href;else link.removeAttribute('href');
                link.rel='noopener noreferrer';
            }
            const page='<!doctype html><html lang="'+esc(document.documentElement.lang||'en')+'"><meta charset="utf-8"><title>'+esc(title)+'</title><style>body{font:16px/1.65 system-ui,sans-serif;color:#182331;background:white;margin:24mm;overflow-wrap:anywhere}img{max-width:100%}table{border-collapse:collapse;width:100%}td,th{border:1px solid #aab4c4;padding:8px}pre{white-space:pre-wrap;background:#edf0f4;padding:12px}blockquote{border-left:3px solid #8a9ab0;padding-left:16px}h1,h2,h3{line-height:1.25;break-after:avoid}tr,img{break-inside:avoid}@page{size:A4;margin:0}</style>'+wrapper.innerHTML+'</html>';
            if(format==='html'){download(page,'text/html;charset=utf-8',basename(current.path).replace(/\.md$/i,'.html'));return;}
            const frame=document.createElement('iframe');frame.className='notes-print-frame';frame.title=tr('print');frame.srcdoc=page;document.body.append(frame);
            frame.onload=async()=>{frame.contentWindow.addEventListener('afterprint',()=>frame.remove(),{once:true});await frame.contentDocument.fonts.ready;await Promise.all([...frame.contentDocument.images].map(img=>img.decode().catch(()=>{})));frame.contentWindow.focus();frame.contentWindow.print();};
            life.signal.addEventListener('abort',()=>frame.remove(),{once:true});
        }
        function download(content,type,name){const url=URL.createObjectURL(new Blob([content],{type})),a=document.createElement('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}
        async function act(action,event){
            if(disposed)return;
            if(!['document','insert','templates'].includes(action))find('[data-popup]').hidden=true;
            try {
                if(action==='new')return await createNote();
                if(action.startsWith('template:'))return await createNote(action.split(':')[1]);
                if(action==='save')return await queue?.save();
                if(action==='retry'){notice('');return queue?.error?await queue.save():await refreshList();}
                if(action==='saveAs')return await saveAs();
                if(action==='open'){const selected=await ctx.openFileDialog?.({title:tr('open'),initialPath:DIR,filters:[{label:'Markdown',extensions:['.md']}]});if(selected?.path&&!selected.canceled){if(selected.path.startsWith(DIR+'/'))await loadNote(selected.path);else await importDesktopFile(selected.path);}return;}
                if(action==='import'){find('[data-import]').click();return;}
                if(action==='source'){if(editor){const ok=await editor.mode(!editor.sourceMode);sourceRequested=editor.sourceMode;if(!ok)notice(tr('source_preserved'));syncChrome();}return;}
                if(action==='library'){root.classList.toggle('notes-hide-library');return;}
                if(action==='focus'){root.classList.toggle('notes-focus');return;}
                if(['outline','info','assist','search'].includes(action)){right=right===action?'':action;renderRight();return;}
                if(action==='closePanel'){right='';renderRight();return;}
                if(action==='dismiss'){notice('');return;}
                if(action==='showTrash'){trash=!trash;page=0;await refreshList();return;}
                if(action==='more'){page++;await refreshList(true);return;}
                if(action==='pin'&&writable()){updateMeta({pinned:meta.pinned.includes(current.path)?meta.pinned.filter(p=>p!==current.path):[...meta.pinned,current.path]});return;}
                if(['trash','move','restore','rename'].includes(action))return await moveNote(action==='rename'?'move':action);
                if(action==='document'){popup([['rename','rename'],['move','move'],['saveAs','save_as'],['save','save'],['templates','templates']],event?.target||find('[data-action="document"]'));return;}
                if(action==='templates'){popup(['blank','meeting','checklist','journal'].map(kind=>['template:'+kind,'template_'+kind]),event?.target);return;}
                if(action==='insert'){popup([['table','table'],['addRowAfter','insert_row'],['addColumnAfter','insert_column'],['deleteRow','delete_row'],['deleteColumn','delete_column'],['deleteTable','delete_table'],['image','image'],['attachment','attachment'],['mermaid','diagram'],['codeblock','code_block'],['rule','horizontal_rule']],event?.target);return;}
                if(action==='image'||action==='attachment'){if(!writable())return;find('[data-upload]').accept=action==='image'?'image/png,image/jpeg,image/webp,image/gif':'';find('[data-upload]').click();return;}
                if(['table','mermaid','codeblock','rule','task'].includes(action)&&writable()){
                    const snippets={table:'| '+tr('column')+' 1 | '+tr('column')+' 2 |\n| --- | --- |\n|  |  |\n',mermaid:'```mermaid\ngraph LR\n  A --> B\n```\n',codeblock:'```\n\n```\n',rule:'\n---\n',task:'- [ ] '};
                    editor.insert(snippets[action]);return;
                }
                if(action==='link'&&writable()){const url=await ctx.promptDialog?.(tr('link_url'),'https://');if(url&&(/^https?:\/\//i.test(url)||resolvePath(url)))editor.command('link',url);return;}
                if(['generate','cancelAssist','apply'].includes(action)){
                    if(action==='generate')return await assist();
                    if(action==='cancelAssist'){assistController?.abort();if(find('[data-ai-state]'))find('[data-ai-state]').textContent=tr('cancelled');return;}
                    if(proposal&&proposal.revision===queue?.revision&&proposal.path===current?.path&&writable()){editor.replaceSelection(proposal.text,proposal.selected);proposal=null;renderRight();}else notice(tr('stale_suggestion'),true);return;
                }
                if(['findNext','replaceOne','replaceAll'].includes(action)&&editor){const count=editor.findText(find('[data-find]').value,action==='findNext'?undefined:find('[data-replace]').value,action==='replaceAll',find('[data-case]').checked);find('[data-find-count]').textContent=tr('matches',{count});return;}
                if(['md','txt','html','print'].includes(action))return await exportNote(action);
                if(['zoomOut','zoomIn','zoomReset'].includes(action)){zoom=action==='zoomReset'?100:Math.min(180,Math.max(70,zoom+(action==='zoomIn'?10:-10)));root.style.setProperty('--notes-zoom',zoom/100);find('[data-zoom]').textContent=zoom+'%';return;}
                if(editor&&!busy)editor.command(action);
            }catch(error){fail(error);}
        }
        function menus(){
            const item=(id,label,shortcut,disabled=false)=>({id,label:tr(label),icon:icons[id]||'notes',shortcut,disabled,action:()=>act(id)});
            ctx.setWindowMenus?.(windowId,[
                {id:'file',labelKey:'desktop.menu_file',items:[item('new','new_note','Ctrl+N',ctx.readonly),item('templates','templates',null,ctx.readonly),item('open','open','Ctrl+O'),item('import','import',null,ctx.readonly),{type:'separator'},item('save','save','Ctrl+S',!writable()),item('saveAs','save_as',null,!writable()),item('rename','rename',null,!writable()),{type:'separator'},...['md','html','txt'].map(format=>item(format,'export_'+format,null,!editor)),item('print','print','Ctrl+P',!editor)]},
                {id:'edit',labelKey:'desktop.menu_edit',items:[item('undo','undo','Ctrl+Z',!writable()),item('redo','redo','Ctrl+Y',!writable()),item('search','find','Ctrl+F',!editor),item('insert','insert',null,!writable()),item('trash','trash',null,!writable())]},
                {id:'view',label:tr('view'),items:[item('library','library'),item('outline','outline'),item('info','info'),item('source','markdown_source',null,!editor),item('focus','focus_mode')]},
                {id:'agent',labelKey:'desktop.menu_agent',items:[item('assist','assist',null,!writable())]}
            ]);
        }
        on(window,'beforeunload',event=>{if(queue?.dirty){backup();event.preventDefault();event.returnValue='';}});
        on(root,'click',event=>{
            const row=event.target.closest('[data-note-path]');if(row){loadNote(row.dataset.notePath);return;}
            const jump=event.target.closest('[data-jump]');if(jump){editor?.jump(Number(jump.dataset.jump));return;}
            const target=event.target.closest('[data-action]');if(target&&!target.disabled)act(target.dataset.action,event);
            else if(!event.target.closest('[data-popup]'))find('[data-popup]').hidden=true;
        });
        on(root,'change',event=>{
            if(event.target.matches('[data-style]'))editor?.command('style',event.target.value);
            if(event.target.matches('[data-tags]')&&writable())editor.setContent(NotesFrontmatter.updateTags(editor.content(),event.target.value.split(',').map(tag=>tag.trim()).filter(Boolean)));
            if(event.target.matches('[data-folder]')){page=0;refreshList().catch(fail);}
            if(event.target.matches('[data-sort]'))updateMeta({sort:event.target.value});
        });
        on(root,'input',event=>{if(event.target.matches('[data-search],[data-tag]')){clearTimeout(searchTimer);searchTimer=setTimeout(()=>{page=0;refreshList().catch(fail);},250);}});
        on(find('[data-upload]'),'change',async event=>{const files=[...event.target.files];event.target.value='';const active=editor;for(const file of files)try{const image=file.type.startsWith('image/'),url=await uploadFile(file,image);if(active!==editor||disposed)return;editor.insert((image?'!':'')+'['+file.name.replace(/[\[\]\\]/g,'')+']('+url+')');}catch(error){fail(error);}});
        on(find('[data-import]'),'change',async event=>{const file=event.target.files?.[0];event.target.value='';if(!file)return;if(file.size>2*1024*1024){notice(tr('note_too_large'),true);return;}try{await createNote('blank',await file.text());}catch(error){fail(error);}});
        on(root,'keydown',event=>{
            const popup=find('[data-popup]');if(!popup.hidden&&['ArrowDown','ArrowUp','Escape'].includes(event.key)){event.preventDefault();const buttons=[...popup.querySelectorAll('button')];if(event.key==='Escape'){popup.hidden=true;editor?.focus();}else buttons[(buttons.indexOf(document.activeElement)+(event.key==='ArrowDown'?1:buttons.length-1))%buttons.length]?.focus();return;}
            if(!(event.ctrlKey||event.metaKey)||event.altKey)return;
            const action={s:'save',o:'open',n:'new',p:'print',f:'search'}[event.key.toLowerCase()];
            if(action){event.preventDefault();event.stopPropagation();act(action);}
        });
        const resize=new ResizeObserver(entries=>root.classList.toggle('notes-narrow',entries[0].contentRect.width<900));resize.observe(root);
        const desktopEvent=event=>{
            if(event?.type!=='desktop_changed')return;
            const payload=event.payload||{};if(![payload.path,payload.old_path,payload.new_path].some(path=>path?.startsWith(DIR+'/')||path?.startsWith('Trash/Notes/')))return;
            clearTimeout(sseTimer);sseTimer=setTimeout(async()=>{try{await refreshList();if(current&&!queue?.dirty&&!queue?.pending&&!busy){const updated=await request(endpoint(current.path),{signal:life.signal});if(updated.version!==current.version)await loadNote(updated.path,updated);}}catch(error){fail(error);}},500);
        };
        window.AuraSSE?.on?.('virtual_desktop_event',desktopEvent);
        function cleanup(){
            if(disposed)return;disposed=true;life.abort();docLife.abort();assistController?.abort();resize.disconnect();window.AuraSSE?.off?.('virtual_desktop_event',desktopEvent);
            for(const timer of [searchTimer,updateTimer,draftTimer,draftDeadline,sseTimer])clearTimeout(timer);
            queue?.dispose();editor?.destroy();ctx.clearWindowMenus?.(windowId);instances.delete(windowId);
        }
        instance.ready=(async()=>{
            try{const saved=await request(endpoint(META),{signal:life.signal}).catch(error=>{if(error.status!==404)throw error;return null;});
                if(saved){try{const data=JSON.parse(saved.content);if(data&&typeof data==='object')meta={...meta,...data,sort:data.sort==='name'?'name':'modified',pinned:Array.isArray(data.pinned)?data.pinned.filter(path=>typeof path==='string'&&path.startsWith(DIR+'/')):[]};}catch(_){notice(tr('request_failed'),true);}}
                if(disposed)return;find('[data-sort]').value=meta.sort;await refreshList();const initial=ctx.path||meta.last_note;
                if(initial&&list.some(note=>note.path===initial))await loadNote(initial);
                else if(ctx.path){if(ctx.path.startsWith(DIR+'/'))await loadNote(ctx.path);else await importDesktopFile(ctx.path);}
                else if(list.length)await loadNote(list[0].path);
            }catch(error){fail(error);}finally{if(!disposed)syncChrome();}
        })();
        syncChrome();
        return instance;
    }
    function dispose(id){instances.get(id)?.dispose();}
    window.NotesApp={render,dispose,instances};
})();
