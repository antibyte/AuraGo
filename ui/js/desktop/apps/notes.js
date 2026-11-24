(function () {
    'use strict';
    const instances = new Map(), DIR = 'Documents/Notes', META = DIR + '/notes.meta.json';
    const basename = path => String(path || '').split('/').pop();
    const dirname = path => String(path || '').split('/').slice(0, -1).join('/');
    const RECENT_MAX = 20;
    const storage = {get: key => {try{return localStorage.getItem(key);}catch(_){return null;}}, set: (key, value) => {try{localStorage.setItem(key, value);}catch(_){}}};
    function resolveNoteLinkPath(href, from) {
        if (!href || /^[a-z][a-z\d+.-]*:/i.test(href) || href.startsWith('/') || href.startsWith('#')) return null;
        if (from?.startsWith('Trash/Notes/')) from = DIR + '/' + from.split('/').slice(3).join('/');
        if (!from) return null;
        const parts = from.split('/').slice(0, -1);
        let decoded;
        try { decoded = decodeURIComponent(href.split(/[?#]/)[0]).replace(/\\/g, '/'); } catch (_) { return null; }
        for (const part of decoded.split('/')) { if (part === '..') parts.pop(); else if (part && part !== '.') parts.push(part); }
        const path = parts.join('/');
        return path.startsWith(DIR + '/') ? path : null;
    }
    const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    async function request(url, options = {}) {
        const response = await fetch(url, {credentials:'same-origin',cache:'no-store',...options});
        const body = await response.json().catch(()=>({}));
        if (!response.ok) { const error = new Error(body.error || body.message || ('HTTP '+response.status)); error.status=response.status; throw error; }
        return body;
    }
    const endpoint = path => '/api/desktop/notes' + (path?'?path='+encodeURIComponent(path):'');
    const TABS=['backlinks','outgoing','localGraph','outline','info','assist'], RIGHT_KEY='aurago.notes.right', GRAPH_KEY='aurago.notes.graph';
    function readGraphPrefs(){
        const prefs={tags:false,unresolved:true,orphans:true,depth:1};
        try{const value=JSON.parse(localStorage.getItem(GRAPH_KEY)||'{}');if(value&&typeof value==='object'){for(const key of ['tags','unresolved','orphans'])if(typeof value[key]==='boolean')prefs[key]=value[key];if(value.depth===2)prefs.depth=2;}}catch(_){}
        return prefs;
    }
    function writeGraphPrefs(prefs){try{localStorage.setItem(GRAPH_KEY,JSON.stringify(prefs));}catch(_){}}
    function readRight(){try{const value=JSON.parse(localStorage.getItem(RIGHT_KEY)||'{}');return value&&typeof value==='object'?{open:!!value.open,tab:String(value.tab||'')}:{open:false,tab:''};}catch(_){return {open:false,tab:''};}}
    function writeRight(open,tab){try{localStorage.setItem(RIGHT_KEY,JSON.stringify({open,tab}));}catch(_){}}
    // "# ^ [ ]" cannot appear in a wikilink target (Obsidian forbids them in note names for that reason).
    const INVALID_NAME=/[\\:*?"<>|#^\[\]]/, PROPS_KEY='aurago.notes.properties', CALLOUTS=['note','info','tip','success','question','warning','danger','bug','example','quote'], RESERVED_PROPS=['tags','aliases','alias'];
    // History shortcuts: Alt+Arrow (Cmd+Alt+Arrow on macOS, where Option+Arrow moves by word). They never fire from editable targets.
    const IS_MAC = /mac|iphone|ipad/i.test(String(typeof navigator === 'undefined' ? '' : navigator.userAgentData?.platform || navigator.platform || ''));
    const HISTORY_KEYS = {back: IS_MAC ? '⌘⌥←' : 'Alt+←', forward: IS_MAC ? '⌘⌥→' : 'Alt+→'};
    const isEditableTarget = el => !!el?.closest?.('input,textarea,select,[contenteditable]:not([contenteditable=false]),.cm-content');
    function historyShortcut(event) {
        if (event.defaultPrevented || event.ctrlKey || event.shiftKey || !event.altKey || event.metaKey !== IS_MAC) return '';
        if (!['ArrowLeft', 'ArrowRight'].includes(event.key) || isEditableTarget(event.target)) return '';
        return event.key === 'ArrowLeft' ? 'back' : 'forward';
    }
    // Excerpts (embedded note cards, hover previews) render another note's Markdown inside the app. Only plain reading
    // markup survives: no data-*, ids, forms, maps, media or links, and images only from note attachments or http(s).
    const EXCERPT_TAGS = ['p','h1','h2','h3','h4','h5','h6','ul','ol','li','blockquote','pre','code','em','strong','del','s','mark','a','img','table','thead','tbody','tfoot','tr','th','td','hr','br','span','input'];
    const EXCERPT_ATTRS = ['href','src','alt','title','type','checked','disabled','colspan','rowspan','align','start'];
    const EXCERPT_FORBID_ATTR = ['id','name','srcset','usemap','poster','popover','popovertarget','form','formaction','tabindex','style','class'];
    function sanitizeExcerpt(html, resolveSrc) {
        const hook = node => {
            if (node.nodeType !== 1) return;
            if (node.nodeName === 'IMG') {
                const url = resolveSrc(node.getAttribute('src') || '');
                if (!/^(https?:\/\/|\/api\/desktop\/download\?)/i.test(url || '')) { node.parentNode?.removeChild(node); return; }
                node.setAttribute('src', url); node.setAttribute('loading', 'lazy');
            } else if (node.nodeName === 'A') {
                for (const name of ['href', 'target', 'rel']) node.removeAttribute(name);
            } else if (node.nodeName === 'INPUT') {
                if (node.getAttribute('type') !== 'checkbox') { node.parentNode?.removeChild(node); return; }
                node.setAttribute('disabled', '');
            }
        };
        DOMPurify.addHook('afterSanitizeAttributes', hook);
        try { return DOMPurify.sanitize(html, {ALLOWED_TAGS: EXCERPT_TAGS, ALLOWED_ATTR: EXCERPT_ATTRS, FORBID_ATTR: EXCERPT_FORBID_ATTR, ALLOW_DATA_ATTR: false, ALLOW_ARIA_ATTR: false}); }
        finally { DOMPurify.removeHook('afterSanitizeAttributes', hook); }
    }
    function render(host, windowId, ctx = {}) {
        dispose(windowId);
        const stored=readRight();
        const esc=ctx.esc||escapeHTML, tr=(key,params)=>{
            for(const prefix of ['notes_','writer_','']){const id='desktop.'+prefix+key,value=ctx.t?.(id,params);if(value&&value!==id)return value;}
            return key;
        };
        const icons={library:'columns',new:'file-plus',search:'search',pin:'bookmark',info:'info',outline:'list',assist:'star',source:'file-code',focus:'maximize',insert:'plus',
            save:'save',saveAs:'copy',rename:'pencil',move:'folder',trash:'trash',restore:'undo',open:'folder-open',print:'printer',md:'notes',html:'file-code',txt:'file-text',
            undo:'undo',redo:'redo',bold:'format-bold',italic:'format-italic',strike:'format-strike',bullet:'list',ordered:'format-numbered',quote:'format-quote',code:'code',link:'link',close:'x',closePanel:'x',dismiss:'x',cancelAssist:'x',task:'check-square',
            back:'chevron-left',forward:'chevron-right',sidebarRight:'columns',backlinks:'link',outgoing:'external',localGraph:'network',graph:'network',graphFit:'maximize',graphClose:'x',switcher:'search',highlight:'highlighter',wikilink:'brackets',findPrev:'chevron-up',findNext:'chevron-down',findClose:'x',commandPalette:'command',daily:'calendar',dailyPrev:'chevron-left',dailyNext:'chevron-right',callout:'format-quote',insertTemplate:'file-plus'};
        const menuIcons={highlight:'edit',wikilink:'link',commandPalette:'search'};
        const inlineSvg=body=>'<svg class="notes-icon" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">'+body+'</svg>';
        // Keys the desktop mini-icon set does not ship; drawn inline so they never fall back to the generic sprite.
        const INLINE_ICONS={
            highlighter:inlineSvg('<path d="M9 11l6 6"/><path d="M14.5 4.5l5 5-8.5 8.5-5-5z"/><path d="M6 13l-2.5 6.5L10 17.5"/><path d="M4 21h6"/>'),
            brackets:inlineSvg('<path d="M9 4H5v16h4"/><path d="M15 4h4v16h-4"/><path d="M9.5 9v6M14.5 9v6"/>'),
            command:inlineSvg('<path d="M9 9V6.5a2.5 2.5 0 1 0-2.5 2.5H9zm0 0h6m-6 0v6m6-6V6.5A2.5 2.5 0 1 1 17.5 9H15zm0 0v6m0 0h2.5a2.5 2.5 0 1 1-2.5 2.5V15zm0 0H9m0 0H6.5A2.5 2.5 0 1 0 9 17.5V15z"/>'),
            tag:inlineSvg('<path d="M4 4h7l9 9-7 7-9-9z"/><circle cx="8.5" cy="8.5" r="1.2"/>')
        };
        const icon=key=>{const name=icons[key]||key;return INLINE_ICONS[name]||ctx.iconMarkup?.(name,'','notes-icon',18,'action')||'<span aria-hidden="true">•</span>';};
        const button=(action,label=action,text=false)=>'<button type="button" data-action="'+action+'" title="'+esc(tr(label)+(HISTORY_KEYS[action]?' ('+HISTORY_KEYS[action]+')':''))+'" aria-label="'+esc(tr(label))+'">'+(text?esc(tr(label)):icon(action))+'</button>';
        host.innerHTML='<div class="vd-notes-app" data-notes="'+esc(windowId)+'">'+
            '<header class="notes-documentbar"><div class="notes-document">'+button('library')+button('back','history_back')+button('forward','history_forward')+'<nav class="notes-crumbs" data-crumbs aria-label="'+esc(tr('breadcrumb'))+'" hidden></nav><span class="notes-daily-nav" data-daily-nav hidden>'+button('dailyPrev','daily_prev')+'<span data-daily-date></span>'+button('dailyNext','daily_next')+'</span><button class="notes-name" data-action="document"><span data-name>'+esc(tr('title'))+'</span><span aria-hidden="true">⌄</span></button><span data-save-state role="status" aria-live="polite"></span></div><nav aria-label="'+esc(tr('panels'))+'">'+button('switcher')+button('graph')+button('daily')+button('pin')+button('sidebarRight','right_sidebar')+'</nav></header>'+
            '<div class="vd-notes-toolbar" role="toolbar" aria-label="'+esc(tr('format'))+'"><div class="notes-toolgroup">'+button('undo')+button('redo')+'</div><div class="notes-toolgroup"><select data-style aria-label="'+esc(tr('style'))+'"><option value="p">'+esc(tr('paragraph'))+'</option>'+[1,2,3].map(n=>'<option value="'+n+'">'+esc(tr('heading'))+' '+n+'</option>').join('')+'</select></div><div class="notes-toolgroup">'+button('bold')+button('italic')+button('strike')+'</div><div class="notes-toolgroup">'+button('bullet','bullets')+button('ordered','numbering')+button('task')+button('quote')+'</div><div class="notes-toolgroup">'+button('link')+button('wikilink')+button('highlight')+button('code')+button('insert')+'</div><span class="notes-spacer"></span>'+button('source','markdown_source')+'</div>'+
            '<div class="notes-findbar" data-findbar role="search" aria-label="'+esc(tr('find'))+'" hidden><input data-find type="text" autocomplete="off" spellcheck="false" placeholder="'+esc(tr('find'))+'" aria-label="'+esc(tr('find'))+'"><input data-replace type="text" autocomplete="off" spellcheck="false" placeholder="'+esc(tr('replace_placeholder'))+'" aria-label="'+esc(tr('replace'))+'"><span class="notes-find-count" data-find-count role="status" aria-live="polite"></span><span class="notes-find-buttons">'+button('findPrev','find_previous')+button('findNext','find_next')+'<button type="button" data-action="findCase" aria-pressed="false" title="'+esc(tr('case_sensitive'))+'" aria-label="'+esc(tr('case_sensitive'))+'">Aa</button>'+button('replaceOne','replace',true)+button('replaceAll','replace_all',true)+button('findClose','close')+'</span></div>'+
            '<div class="notes-notice" data-notice role="alert" hidden><span data-notice-text></span>'+button('retry','retry',true)+button('saveAs','save_as',true)+button('dismiss','close')+'</div>'+
            '<div class="notes-workspace"><aside class="notes-library" aria-label="'+esc(tr('library'))+'"><div class="notes-library-host" data-sidebar></div></aside>'+
            '<main class="notes-main"><div class="notes-scroll" data-scroll><div class="notes-page"><div class="notes-properties" data-properties hidden></div><div class="notes-note-meta" data-note-meta></div><div data-editor></div></div></div><div class="notes-empty" data-empty><div class="notes-empty-icon">'+icon('md')+'</div><h2>'+esc(tr('empty_title'))+'</h2><p>'+esc(tr('empty_description'))+'</p>'+button('new','new_note',true)+'</div><div class="notes-loading" data-loading hidden>'+esc(tr('loading'))+'</div>'+
            '<section class="notes-graph" data-graph hidden aria-label="'+esc(tr('graph'))+'"><div class="notes-graph-bar" role="toolbar" aria-label="'+esc(tr('graph'))+'"><input type="search" data-graph-filter autocomplete="off" spellcheck="false" placeholder="'+esc(tr('graph_filter'))+'" aria-label="'+esc(tr('graph_filter'))+'"><span class="notes-graph-toggles">'+['tags','unresolved','orphans'].map(key=>'<button type="button" data-graph-toggle="'+key+'" aria-pressed="false">'+esc(tr(key==='tags'?'tags':'graph_'+key))+'</button>').join('')+'</span><span class="notes-graph-actions">'+button('graphFit','graph_fit')+button('graphClose','graph_close')+'</span></div><div class="notes-graph-host" data-graph-host></div><p class="notes-graph-empty" data-graph-empty hidden>'+esc(tr('graph_empty'))+'</p></section></main><aside class="notes-right" data-right hidden></aside></div>'+
            '<footer class="notes-statusbar"><button type="button" data-action="backlinks" data-backlinks disabled></button><span data-count></span><span class="notes-spacer"></span><span data-mode></span>'+button('focus','focus_mode')+'<button data-action="zoomOut" aria-label="'+esc(tr('zoom_out'))+'">−</button><button data-action="zoomReset" data-zoom>100%</button><button data-action="zoomIn" aria-label="'+esc(tr('zoom_in'))+'">+</button></footer><div class="notes-popup" data-popup hidden></div><div class="notes-preview" data-preview hidden role="dialog" aria-label="'+esc(tr('preview'))+'"></div><input data-upload type="file" multiple hidden><input data-import type="file" accept=".md,.markdown,.txt,text/markdown,text/plain" hidden></div>';
        const root=host.firstElementChild, life=new AbortController();
        // App chrome is never looked up inside the editor or an excerpt: embedded note content must not shadow popups, panels or inputs.
        const untrusted=el=>{const area=el.closest('[data-editor],.notes-excerpt');return !!area&&!(area===el&&el.hasAttribute('data-editor'));};
        const find=selector=>{const first=root.querySelector(selector);if(!first||!untrusted(first))return first;for(const el of root.querySelectorAll(selector))if(!untrusted(el))return el;return null;};
        let docLife=new AbortController(), editor, queue, current, documentGeneration=0, disposed=false, busy=false, loading=false, navigating=false, navTask=null, zoom=100, right=stored.open&&TABS.includes(stored.tab)?stored.tab:'', lastTab=TABS.includes(stored.tab)?stored.tab:'backlinks', updateTimer, draftTimer, draftDeadline, sseTimer;
        let meta={version:1,pinned:[],sort:'modified',last_note:'',recent:[]}, list=[], trashNotes=[], trash=false, listGeneration=0, currentDraft, assistController, proposal;
        let sidebar, sidebarFrame=0, contextTarget=null, findCase=false;
        let graph=null, localGraph=null, graphOpen=false, graphPointer=false, graphFilter='', graphLabels=new Map(), localLabels=new Map(), graphPrefs=readGraphPrefs();
        let metaTask=Promise.resolve(), sourceRequested=false, propsOpen=storage.get(PROPS_KEY)!=='0', propsForce=false, propsSignature='';
        const draftKey=()=>location.origin+':'+current.path;
        let indexLoaded=false, indexSignature='', stack=[], stackPos=-1, previewTimer, previewToken=0, previewScroll=0;
        const excerpts=new Map();
        const contextCache=new Map();
        const index=NotesIndex.create({request:(url,options)=>request(url,{...options,signal:life.signal}),onChange:indexChanged});
        const instance={act,commands:()=>commands(),openSwitcher:()=>openSwitcher(),openCommands:()=>openCommands(),get sidebar(){return sidebar;},get meta(){return meta;},get editor(){return editor;},get session(){return queue;},get path(){return current?.path;},get index(){return index;},get graph(){return graph;},get localGraph(){return localGraph;},get graphOpen(){return graphOpen;},get history(){return {stack:[...stack],position:stackPos};},dispose:cleanup};
        instances.set(windowId,instance);
        window.AuraDesktopEmail?.attach(windowId, '.notes-documentbar', () => ({
            key: docLife.signal, signal: docLife.signal, subject: current?.title || '',
            disabled: !editor || !current || busy || disposed, formats: ['md','html','txt'],
            createFiles: (format,signal) => exportNote(format,true,signal)
        }));
        ctx.registerWindowCleanup?.(windowId,cleanup);ctx.setWindowBeforeClose?.(windowId,guard);ctx.wireContextMenuBoundary?.(host);
        ctx.registerSessionAction?.(windowId,{
            inspect:()=>({dirty:!!queue?.dirty,revision:JSON.stringify([documentGeneration,queue?.revision??0]),label:current?.title||tr('title')}),
            decide:snapshot=>ctx.decideSessionAction?.({title:snapshot.title,label:snapshot.label,dirty:snapshot.dirty}),
            save:async expected=>{
                if(disposed||busy||loading||!queue||JSON.stringify([documentGeneration,queue.revision])!==expected)return false;
                await queue.save();
                return !queue.dirty&&JSON.stringify([documentGeneration,queue.revision])===expected?{revision:expected}:false;
            }
        });
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
            root.querySelectorAll('.vd-notes-toolbar [data-action]').forEach(el=>{el.disabled=!editor || (el.dataset.action!=='source'&&(!writable()||(editor.sourceMode&&!['undo','redo','highlight','wikilink'].includes(el.dataset.action))));});
            root.querySelectorAll('[data-action="pin"]').forEach(el=>{el.disabled=!current||ctx.readonly||trash;el.setAttribute('aria-pressed',String(meta.pinned.includes(current?.path)));});
            find('[data-action="source"]').setAttribute('aria-pressed',String(!!editor?.sourceMode));
            root.querySelectorAll('[data-action="new"]').forEach(el=>el.disabled=!!ctx.readonly||busy);
            root.querySelectorAll('[data-action="back"]').forEach(el=>el.disabled=stackPos<=0||busy);
            root.querySelectorAll('[data-action="forward"]').forEach(el=>el.disabled=stackPos>=stack.length-1||busy);
            root.querySelectorAll('[data-action="sidebarRight"]').forEach(el=>el.setAttribute('aria-pressed',String(!!right)));
            root.querySelectorAll('.notes-documentbar [data-action="graph"]').forEach(el=>el.setAttribute('aria-pressed',String(graphOpen)));
            renderCrumbs();updateDailyNav();updateBacklinkCount();
            menus();updateDetails();scheduleSidebar();
        }
        function renderCrumbs(){
            const nav=find('[data-crumbs]'),path=current?.path||'',trashed=path.startsWith('Trash/Notes/'),base=trashed?'Trash/Notes':DIR,folders=path.startsWith(base+'/')?path.slice(base.length+1).split('/').slice(0,-1):[];
            const segments=trashed?[tr('trash'),...folders]:folders;
            nav.hidden=!segments.length;
            nav.innerHTML=segments.map((name,i)=>'<span class="notes-crumb" role="button" tabindex="0" data-crumb="'+esc((trashed?'Trash/Notes':DIR)+(trashed?folders.slice(0,i).map(f=>'/'+f).join(''):'/'+folders.slice(0,i+1).join('/')))+'">'+esc(name)+'</span><span class="notes-crumb-sep" aria-hidden="true">›</span>').join('');
        }
        function updateBacklinkCount(){
            const button=find('[data-backlinks]');if(!button)return;
            const count=current&&indexLoaded&&!current.path.startsWith('Trash/')?index.backlinks(current.path).length:0;
            button.textContent=tr('backlinks_count',{count});button.disabled=!current;button.dataset.count=String(count);button.title=tr('backlinks');
        }
        function updateDetails(){
            if(!editor||disposed)return;
            const text=editor.text,words=text.match(/[\p{L}\p{N}]+(?:['’\-][\p{L}\p{N}]+)*/gu)||[];
            find('[data-count]').textContent=tr('counts',{words:words.length,chars:[...text].length});
            if(current) {const title=NotesFrontmatter.deriveTitle(editor.content(),basename(current.path));if(title!==current.title){current.title=title;scheduleSidebar();}find('[data-name]').textContent=current.title;}
            if(right==='outline'||right==='outgoing')renderPanelBody();
            renderProperties();
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
                    index.refresh().catch(()=>{});
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
        // Navigations (open, new, move, history) run one at a time: the slot is claimed before the first await,
        // so a second request that arrives while guard() is still saving or asking is dropped instead of interleaving.
        async function navigate(task){
            if(disposed||busy||navigating)return;
            navigating=true;
            return navTask=(async()=>{
                try{return await task();}
                finally{navigating=false;if(!disposed)syncChrome();}
            })();
        }
        const loadNote=(path,supplied,options)=>navigate(()=>loadInner(path,supplied,options));
        async function loadInner(path, supplied, options={}) {
            if(disposed||!await guard())return;
            busy=true;loading=true;find('[data-loading]').hidden=false;syncChrome();
            try {
                const note=supplied||await request(endpoint(path),{signal:life.signal});
                if(disposed)return;
                let draft=await OfficeSession.draft('get',location.origin+':'+note.path,undefined,'notes').catch(()=>null);
                if(draft?.content===note.content){await OfficeSession.draft('delete',location.origin+':'+note.path,undefined,'notes').catch(()=>{});draft=null;}
                const recovered=draft&&!ctx.readonly&&!note.path.startsWith('Trash/')&&await ctx.confirmDialog?.(tr('recover'),tr('recover_prompt'));
                const next=await NotesEditor.create(document.createElement('div'),{content:recovered?draft.content:note.content,readonly:ctx.readonly||note.path.startsWith('Trash/'),tr,
                    upload:file=>uploadFile(file,true),resolveURL:href=>resolveURL(href,note.path),openLink:openLink,changed,popupHost:root,
                    resolveWiki:value=>resolveWiki(value,note.path),openWiki:(value,event)=>openWiki(value,event,note.path),embedWiki:(value,element,parsed)=>embedWiki(value,element,note.path,parsed),previewWiki:(value,anchor)=>previewWiki(value,anchor,note.path),
                    suggest:(query,suggestOptions)=>suggest(query,suggestOptions,note.path),
                    tagClick:tag=>{showLibrary();sidebar?.focusSearch('tag:'+tag);}});
                if(disposed){await next.destroy();return;}
                queue?.dispose();docLife.abort();docLife=new AbortController();documentGeneration++;clearTimeout(draftTimer);clearTimeout(draftDeadline);
                const previousPath=current?.path;await editor?.destroy();editor=next;current=note;if(graphOpen&&previousPath!==note.path)closeGraph(false);currentDraft=recovered?draft:null;
                find('[data-editor]').replaceChildren(next.view.dom.closest('.notes-rich').parentElement);
                if(sourceRequested&&!editor.sourceMode)await editor.mode(true);
                if(recovered&&draft.etag!==note.version)current.version=draft.etag;
                propsForce=false;propsSignature='';makeQueue();loading=false;
                if(recovered)queue.changed();else saveStatus();
                find('[data-scroll]').scrollTop=0;proposal=null;assistController?.abort();
                ctx.updateWindowContext?.(windowId,{path:note.path});ctx.recordRecentFile?.(note.path,'notes');
                if(editor.sourceMode&&NotesEditor.needsSource(editor.content()))notice(tr('source_preserved'));
                else notice(recovered?tr('recovered'):'');
                trash=note.path.startsWith('Trash/');
                if(!trash)updateMeta({last_note:note.path,recent:[note.path,...(meta.recent||[]).filter(path=>path!==note.path)].slice(0,RECENT_MAX)});
                if(options.push!==false)pushHistory(note.path);
                hidePreview();clearFindMark();
                scheduleSidebar();renderRight();if(root.classList.contains('notes-narrow'))root.classList.add('notes-hide-library');
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
        function noteFolder(){const path=current?.path||'';return path.startsWith(DIR+'/')?dirname(path):DIR;}
        async function createNote(kind='blank',content,folder){
            if(ctx.readonly)return;
            return navigate(()=>createInner(kind,content,folder));
        }
        async function createInner(kind,content,folder){
            if(!await guard())return;
            busy=true;syncChrome();
            try{const text=content??template(kind);const note=await request(endpoint(),{method:'POST',signal:life.signal,headers:{'Content-Type':'application/json'},body:JSON.stringify({title:NotesFrontmatter.deriveTitle(text,tr('new_note')),content:text,folder:folder||noteFolder()})});
                busy=false;trash=false;await index.load().catch(()=>{});await refreshList();await loadInner(note.path,note);editor?.focus();
            }catch(error){fail(error);}finally{busy=false;syncChrome();}
        }
        // Plain list (initial note choice) and trash list (sidebar), paged 200 at a time.
        async function listAll(flag){
            const all=[];
            for(let offset=0;offset<2000;offset+=200){
                const result=await request(endpoint()+'?'+new URLSearchParams({trash:String(flag),offset:String(offset),limit:'200'}),{signal:life.signal});
                all.push(...(result.notes||[]));
                if(!result.notes?.length||all.length>=(result.total||0))break;
            }
            return all;
        }
        async function refreshList(){
            const gen=++listGeneration,[plain,trashed]=await Promise.all([listAll(false),listAll(true)]);
            if(disposed||gen!==listGeneration)return;
            list=plain;trashNotes=trashed;scheduleSidebar();
        }
        function scheduleSidebar(){
            if(sidebarFrame||disposed||!sidebar)return;
            sidebarFrame=requestAnimationFrame(()=>{sidebarFrame=0;if(!disposed)sidebar?.render();});
        }
        const stamp=value=>Date.parse(value)||0;
        function sidebarState(){
            return {notes:index.notes().map(note=>({path:note.path,title:note.path===current?.path&&current.title?current.title:note.title,modified:stamp(note.modified),tags:note.tags||[]})),
                folders:index.data.folders||[],current:current?.path,pinned:meta.pinned,sort:meta.sort,trash,
                trashNotes:trashNotes.map(note=>({path:note.path,title:note.title,modified:stamp(note.modified)})),tagCounts:index.tags()};
        }
        function showLibrary(){root.classList.remove('notes-hide-library');}
        async function searchNotes(text){
            const parsed=NotesSidebar.parseQuery(text),result=await request(endpoint()+'?'+new URLSearchParams({q:parsed.q,tag:parsed.tag,folder:parsed.folder,trash:'false',offset:'0',limit:'50'}),{signal:life.signal});
            return {notes:(result.notes||[]).map(note=>({...note,modified:stamp(note.modified)})),total:result.total};
        }
        function validFolderName(name){return name&&!INVALID_NAME.test(name)&&name.split('/').every(part=>part.trim()&&part!=='.'&&part!=='..'&&!part.startsWith('.'));}
        async function newFolder(parent){
            if(ctx.readonly||busy)return;
            const value=await ctx.promptDialog?.(tr('new_folder'),'');
            const name=String(value||'').trim().replace(/^\/+|\/+$/g,'');
            if(!name)return;
            if(!validFolderName(name)){notice(tr('invalid_name',{name}),true);return;}
            try{
                const folder=(parent||DIR)+'/'+name;
                await request(endpoint(),{method:'POST',signal:life.signal,headers:{'Content-Type':'application/json'},body:JSON.stringify({operation:'mkdir',folder})});
                await index.load().catch(()=>{});showLibrary();sidebar?.revealPath(folder);
            }catch(error){fail(error);}
        }
        function renameMeta(oldPath,newPath){
            const swap=paths=>paths.map(path=>path===oldPath?newPath:path);
            if([...(meta.pinned||[]),...(meta.recent||[])].includes(oldPath)||meta.last_note===oldPath)updateMeta({pinned:swap(meta.pinned||[]),recent:swap(meta.recent||[]),...(meta.last_note===oldPath?{last_note:newPath}:{})});
        }
        async function moveToFolder(path,folder){
            // A drop is a deliberate request: it waits for a running navigation instead of being dropped.
            while(navigating&&navTask&&!disposed)await navTask.catch(()=>{});
            if(ctx.readonly||busy||path.startsWith('Trash/'))return;
            const target=folder+'/'+basename(path);
            if(target===path)return;
            if(path===current?.path)return await moveNote('move',target);
            await navigate(async()=>{
                // The server may rewrite links inside the open note: save it first so nothing is lost, reload it afterwards.
                if(!await guard())return;
                busy=true;syncChrome();
                let moved;
                try{
                    const old=await request(endpoint(path),{signal:life.signal});
                    moved=await request(endpoint(),{method:'PATCH',signal:life.signal,headers:{'Content-Type':'application/json','If-Match':old.version},body:JSON.stringify({path,new_path:target,operation:'move',update_links:true})});
                    renameHistory(path,moved.path);renameMeta(path,moved.path);
                    await index.load().catch(()=>{});await refreshList();sidebar?.revealPath(moved.path);
                    if(current&&!queue?.dirty){
                        const fresh=await request(endpoint(current.path),{signal:life.signal});
                        if(fresh.version!==current.version){busy=false;await loadInner(current.path,fresh,{push:false});}
                    }
                }catch(error){fail(error);}
                finally{busy=false;syncChrome();}
                if(!moved)return;
                if(moved.link_update_failed?.length)notice(tr('links_update_failed',{paths:moved.link_update_failed.join(', ')}),true);
                else if(moved.updated_links>0)notice(tr('links_updated',{count:moved.updated_links}));
            });
        }
        function openContextMenu(path,kind,event){
            if(disposed)return;
            contextTarget={path,kind};
            const pinned=meta.pinned.includes(path);
            let items=kind==='folder'?[['ctx:newNote','new_note'],['ctx:newFolder','new_folder']]:kind==='trash'?[['ctx:open','open'],['ctx:restore','restore']]:
                [['ctx:open','open'],['ctx:rename','rename'],['ctx:move','move'],['ctx:pin',pinned?'unpin':'pin'],['ctx:trash','trash']];
            if(ctx.readonly)items=items.filter(([action])=>action==='ctx:open');
            if(items.length)popup(items,{x:event?.clientX??0,y:event?.clientY??0});
        }
        async function contextAction(name){
            const target=contextTarget;contextTarget=null;
            if(!target||disposed)return;
            const {path,kind}=target;
            if(name==='open'){await loadNote(path);return;}
            if(name==='newNote'){await createNote('blank',undefined,kind==='folder'?path:dirname(path));return;}
            if(name==='newFolder'){await newFolder(kind==='folder'?path:dirname(path));return;}
            if(name==='pin'){if(!ctx.readonly)updateMeta({pinned:meta.pinned.includes(path)?meta.pinned.filter(p=>p!==path):[...meta.pinned,path]});return;}
            if(path!==current?.path){await loadNote(path);if(current?.path!==path)return;}
            await moveNote(name==='rename'?'move':name);
        }
        function updateMeta(change){
            if(ctx.readonly)return;
            Object.assign(meta,change);syncChrome();
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
            return navigate(()=>saveAsInner(chosen));
        }
        async function saveAsInner(chosen){
            busy=true;syncChrome();queue.suspend();
            try{await queue.pending?.catch(()=>{});const text=await editor.relocateLinks(editor.content(),url=>relocatedURL(url,current.path,chosen.path));
                const note=await request(endpoint(chosen.path),{method:'PUT',signal:life.signal,headers:{'Content-Type':'application/json','If-None-Match':'*'},body:JSON.stringify({path:chosen.path,content:text})});
                queue.dispose();queue=null;busy=false;await index.load().catch(()=>{});await loadInner(note.path,note);await refreshList();
            }finally{busy=false;queue?.resume();syncChrome();}
        }
        async function moveNote(operation,preset){
            if(!current||ctx.readonly)return;
            return navigate(()=>moveInner(operation,preset));
        }
        async function moveInner(operation,preset){
            if(!current||!await guard())return;
            let target='';
            if(operation==='trash'&&!await ctx.confirmDialog?.(tr('trash'),tr('trash_confirm')))return;
            if(operation==='move'){
                target=preset||await ctx.promptDialog?.(tr('move_prompt'),current.path);
                if(!target||target===current.path)return;
                if(!preset&&target.split('/').some(part=>INVALID_NAME.test(part))){notice(tr('invalid_name',{name:target}),true);return;}
            }
            busy=true;syncChrome();
            try{const oldPath=current.path,note=await request(endpoint(),{method:'PATCH',signal:life.signal,headers:{'Content-Type':'application/json','If-Match':current.version},body:JSON.stringify({path:current.path,new_path:target,operation,...(operation==='move'?{update_links:true}:{})})});
                renameHistory(oldPath,note.path);renameMeta(oldPath,note.path);
                queue.dispose();queue=null;await editor.destroy();editor=null;current=null;busy=false;trash=operation==='trash';await index.load().catch(()=>{});await refreshList();await loadInner(note.path,note,{push:false});
                if(note.link_update_failed?.length)notice(tr('links_update_failed',{paths:note.link_update_failed.join(', ')}),true);
                else if(note.updated_links>0)notice(tr('links_updated',{count:note.updated_links}));
            }finally{busy=false;syncChrome();}
        }
        function resolvePath(href,from=current?.path){
            return resolveNoteLinkPath(href, from);
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
        // ---- right sidebar, history and index-based wikilinks (Phase 3)
        function setRight(tab){
            right=tab||'';if(TABS.includes(right))lastTab=right;
            writeRight(!!right,lastTab);renderRight();
        }
        function pushHistory(path){
            if(stack[stackPos]===path)return;
            stack=stack.slice(0,stackPos+1);stack.push(path);
            if(stack.length>100)stack.shift();
            stackPos=stack.length-1;
        }
        function renameHistory(oldPath,newPath){stack=stack.map(path=>path===oldPath?newPath:path);}
        async function go(delta){
            await navigate(async()=>{
                const target=stack[stackPos+delta];
                if(target===undefined)return;
                await loadInner(target,undefined,{push:false});
                if(current?.path===target){stackPos+=delta;syncChrome();}
            });
        }
        function headingsOf(text){
            const headings=[];let fence='';
            for(const line of NotesFrontmatter.strip(String(text||'')).split('\n')){
                const mark=/^\s*(```+|~~~+)/.exec(line);
                if(mark){fence=!fence?mark[1][0]:line.trim()[0]===fence?'':fence;continue;}
                if(fence)continue;
                const heading=/^#{1,6}\s+(.+?)\s*#*\s*$/.exec(line.replace(/\r$/,''));
                if(heading)headings.push(heading[1].trim());
            }
            return headings;
        }
        function jumpToHeading(subpath){
            if(!editor||!subpath||subpath.startsWith('^'))return;
            const wanted=subpath.split('#').pop().trim().toLowerCase(),hit=editor.outline().find(heading=>heading.text.trim().toLowerCase()===wanted);
            if(hit)editor.jump(hit.pos);
        }
        // Null while the index is not loaded: chips must not be styled as unresolved by mistake.
        function resolveWiki(value,from=current?.path){
            if(!indexLoaded||!from)return null;
            const path=index.resolve(from,value,false);
            return {path,exists:!!path};
        }
        async function openWiki(value,event,from=current?.path){
            if(!from||disposed)return;
            hidePreview();
            if(!indexLoaded)await index.load().catch(()=>{});
            const parsed=NotesIndex.parseWikiValue(value),path=indexLoaded?index.resolve(from,value,false):'';
            if(path){
                if(!/\.md$/i.test(path)){window.open('/api/desktop/download?path='+encodeURIComponent(path),'_blank','noopener,noreferrer');return;}
                if(path!==current?.path)await loadNote(path);
                if(current?.path===path)jumpToHeading(parsed.subpath);
                return;
            }
            await createWikiNote(parsed.target);
        }
        async function createWikiNote(target){
            if(ctx.readonly||busy)return;
            const name=String(target||'').trim().replace(/\.md$/i,'');
            if(!name)return;
            if(INVALID_NAME.test(name)||name.split('/').some(part=>!part.trim()||part==='.'||part==='..')){notice(tr('invalid_name',{name}),true);return;}
            const folder=name.includes('/')?DIR:noteFolder(),path=folder+'/'+name+'.md';
            try{
                let note;
                try{note=await request(endpoint(path),{method:'PUT',signal:life.signal,headers:{'Content-Type':'application/json','If-None-Match':'*'},body:JSON.stringify({path,content:'# '+name.split('/').pop()+'\n\n'})});}
                catch(error){if(error.status!==412)throw error;}
                await index.load().catch(()=>{});await refreshList();await loadNote(path,note);editor?.focus();
            }catch(error){fail(error);}
        }
        // Rendered excerpt of a note for embeds and hover cards, cached per path and version.
        function excerpt(path){
            const meta=index.note(path),key=path+'|'+(meta?.modified||'')+'|'+(meta?.size||'');
            let hit=excerpts.get(key);if(hit)return hit;
            hit=(async()=>{
                const note=await request(endpoint(path),{signal:life.signal});
                const text=NotesFrontmatter.strip(String(note.content||'')).slice(0,4000).replace(/!?\[\[([^\[\]\n]+)\]\]/g,(match,value)=>{const parsed=NotesIndex.parseWikiValue(value);return parsed.alias||parsed.target||parsed.subpath||value;});
                const wrapper=document.createElement('div');
                wrapper.innerHTML=sanitizeExcerpt(marked.parse(text),src=>resolveURL(src,path));
                const title=meta?.title||NotesFrontmatter.deriveTitle(note.content||'',basename(path).replace(/\.md$/i,'')),first=wrapper.firstElementChild;
                if(first?.tagName==='H1'&&first.textContent.trim()===title)first.remove();
                return {title,html:wrapper.innerHTML};
            })();
            excerpts.set(key,hit);hit.catch(()=>excerpts.delete(key));
            if(excerpts.size>60)excerpts.delete(excerpts.keys().next().value);
            return hit;
        }
        function embedWiki(value,element,from,parsed){
            const token=element._embedToken=Symbol();
            delete element.dataset.kind;
            const path=indexLoaded&&from?index.resolve(from,value,true):'';
            if(!path){element.textContent='![['+value+']]';return;}
            if(/\.(png|jpe?g|gif|webp|svg)$/i.test(path)){
                const img=document.createElement('img'),size=/^(\d+)(?:x(\d+))?$/.exec(parsed?.alias||'');
                img.loading='lazy';img.alt=size?basename(path):(parsed?.alias||basename(path));
                img.src='/api/desktop/download?inline=1&path='+encodeURIComponent(path);
                if(size){img.style.width=size[1]+'px';if(size[2])img.style.height=size[2]+'px';}
                element.dataset.kind='image';element.replaceChildren(img);return;
            }
            if(/\.md$/i.test(path)){
                const title=document.createElement('strong'),body=document.createElement('div');
                title.className='notes-embed-title';title.textContent=index.note(path)?.title||basename(path).replace(/\.md$/i,'');
                body.className='notes-embed-body notes-excerpt';body.textContent=tr('loading');
                element.dataset.kind='note';element.replaceChildren(title,body);
                excerpt(path).then(result=>{if(element._embedToken!==token||disposed)return;title.textContent=result.title;body.innerHTML=result.html;}).catch(()=>{if(element._embedToken===token&&!disposed)body.textContent=tr('request_failed');});
                return;
            }
            const name=document.createElement('span');name.textContent=basename(path);
            element.dataset.kind='file';element.replaceChildren(name);
        }
        function hidePreview(){clearTimeout(previewTimer);previewToken++;const card=find('[data-preview]');if(card)card.hidden=true;}
        function placePreview(anchor){
            const card=find('[data-preview]'),box=anchor.getBoundingClientRect(),bounds=root.getBoundingClientRect(),width=card.offsetWidth,height=card.offsetHeight;
            let left=box.left-bounds.left,top=box.bottom-bounds.top+6;
            left=Math.min(Math.max(8,left),Math.max(8,bounds.width-width-8));
            if(top+height>bounds.height-8){const above=box.top-bounds.top-height-6;top=above>8?above:Math.max(8,bounds.height-height-8);}
            card.style.left=Math.round(left)+'px';card.style.top=Math.round(top)+'px';
        }
        function previewWiki(value,anchor,from=current?.path){
            clearTimeout(previewTimer);
            if(value===null){previewTimer=setTimeout(hidePreview,200);return;}
            if(!from||!indexLoaded||!anchor||anchor.hasAttribute('data-embed'))return;
            const token=++previewToken,path=index.resolve(from,value,false),card=find('[data-preview]');
            previewScroll=find('[data-scroll]').scrollTop;
            if(path&&!/\.md$/i.test(path)){card.hidden=true;return;}
            if(!path){
                card.innerHTML='<button type="button" class="notes-preview-missing" data-create-wiki="'+esc(value)+'">'+esc(tr('preview_missing'))+'</button>';
                card.hidden=false;placePreview(anchor);return;
            }
            card.innerHTML='<button type="button" class="notes-preview-title" data-note-path="'+esc(path)+'">'+esc(index.note(path)?.title||basename(path).replace(/\.md$/i,''))+'</button><div class="notes-preview-body notes-excerpt" data-preview-body>'+esc(tr('loading'))+'</div>';
            card.hidden=false;placePreview(anchor);
            excerpt(path).then(result=>{
                if(token!==previewToken||disposed)return;
                card.querySelector('.notes-preview-title').textContent=result.title;card.querySelector('[data-preview-body]').innerHTML=result.html||'<p class="notes-muted">'+esc(tr('empty_note'))+'</p>';
                placePreview(anchor);
            }).catch(()=>{if(token===previewToken&&!disposed)card.querySelector('[data-preview-body]').textContent=tr('request_failed');});
        }
        function folderLabel(path){const parts=String(path||'').split('/');parts.pop();const rel=parts.join('/');return rel.startsWith(DIR+'/')?rel.slice(DIR.length+1):rel===DIR?'':rel;}
        // [[ autocomplete: notes (and attachments for embeds), or the headings of a note after '#'.
        async function suggest(query,options={},from=current?.path){
            if(!indexLoaded||!from)return [];
            const hash=query.indexOf('#');
            if(hash>=0){
                const name=query.slice(0,hash),wanted=query.slice(hash+1).trim().toLowerCase(),path=name.trim()?index.resolve(from,name,false):from;
                if(!path||!/\.md$/i.test(path))return [];
                let text;
                if(path===current?.path&&editor)text=editor.content();
                else{try{text=(await request(endpoint(path),{signal:life.signal})).content;}catch(_){return [];}}
                const prefix=name.trim()?index.linkText(path):'';
                return headingsOf(text).filter(heading=>!wanted||heading.toLowerCase().includes(wanted)).slice(0,8).map(heading=>({label:heading,detail:index.note(path)?.title||'',value:prefix+'#'+heading.replace(/[\[\]|#]/g,' ').replace(/\s+/g,' ').trim()}));
            }
            const entry=path=>({label:index.note(path)?.title||basename(path).replace(/\.md$/i,''),detail:folderLabel(path),value:index.linkText(path)});
            if(!query.trim())return [...index.notes()].filter(note=>note.path!==from).sort((a,b)=>new Date(b.modified)-new Date(a.modified)).slice(0,8).map(note=>entry(note.path));
            const hits=index.search(query,8).map(hit=>({score:hit.score,item:entry(hit.path)}));
            if(options.embed)for(const path of index.data.attachments||[]){const score=NotesIndex.fuzzy(query,basename(path))*2.5;if(score>0)hits.push({score,item:{label:basename(path),detail:folderLabel(path),value:index.linkText(path)}});}
            return hits.sort((a,b)=>b.score-a.score).slice(0,8).map(hit=>hit.item);
        }
        function indexChanged(data){
            indexLoaded=true;
            const signature=(data.notes||[]).map(note=>note.path+'|'+(note.aliases||[]).join(',')).sort().join('\n')+'\n'+[...(data.attachments||[])].sort().join('\n');
            const changedNames=signature!==indexSignature;indexSignature=signature;
            if(changedNames&&editor&&!disposed)editor.refreshDecorations();
            if(right==='backlinks'||right==='outgoing')renderPanelBody();
            if(graphOpen)updateGraph();if(right==='localGraph')updateLocalGraph();
            updateBacklinkCount();updateDailyNav();menus();scheduleSidebar();root.__notesPalette?.update();
        }
        async function uploadFile(file,imageOnly=false){
            if(!writable())throw Error(tr('readonly'));
            if(file.size>20*1024*1024)throw Error(tr('attachment_too_large'));
            if(imageOnly&&!['image/png','image/jpeg','image/webp','image/gif'].includes(file.type))throw Error(tr('image_type'));
            const form=new FormData();form.append('path',DIR+'/.attachments');form.append('unique','1');form.append('file',file,crypto.randomUUID().slice(0,8)+'-'+file.name);
            const result=await request('/api/desktop/upload',{method:'POST',headers:{'If-None-Match':'*'},body:form,signal:docLife.signal});
            return relativePath(current.path,result.path);
        }
        function popup(items,anchor){
            const el=find('[data-popup]');el.innerHTML=items.map(([action,label,raw])=>raw?'<button type="button" data-action="'+esc(action)+'" title="'+esc(label)+'">'+esc(label)+'</button>':button(action,label,true)).join('');el.hidden=false;
            const bounds=root.getBoundingClientRect();
            if(typeof anchor?.x==='number'){
                const width=el.offsetWidth,height=el.offsetHeight;
                el.style.left=Math.round(Math.max(8,Math.min(anchor.x-bounds.left,bounds.width-width-8)))+'px';el.style.top=Math.round(Math.max(8,Math.min(anchor.y-bounds.top,bounds.height-height-8)))+'px';
                el.querySelector('button')?.focus({preventScroll:true});return;
            }
            const box=anchor?.getBoundingClientRect();el.style.top=Math.min((box?.bottom||bounds.top+48)-bounds.top+4,bounds.height-220)+'px';el.style.left=Math.min(Math.max(8,(box?.left||bounds.left+12)-bounds.left),Math.max(8,bounds.width-230))+'px';
            el.querySelector('button')?.focus({preventScroll:true});
        }
        function renderRight(){
            const panel=find('[data-right]'),open=!!right;destroyLocalGraph();panel.hidden=!open;
            root.querySelectorAll('[data-action="sidebarRight"]').forEach(el=>el.setAttribute('aria-pressed',String(open)));
            if(!open){panel.replaceChildren();return;}
            panel.innerHTML='<div class="notes-tabs" role="tablist" aria-label="'+esc(tr('panels'))+'">'+TABS.map(tab=>'<button type="button" role="tab" id="'+esc(windowId)+'-tab-'+tab+'" data-right-tab="'+tab+'" aria-selected="'+String(tab===right)+'" aria-controls="'+esc(windowId)+'-panel" tabindex="'+(tab===right?'0':'-1')+'" title="'+esc(tabLabel(tab))+'" aria-label="'+esc(tabLabel(tab))+'">'+icon(tab)+'</button>').join('')+'</div><header class="notes-panel-heading"><h2>'+esc(tabLabel(right))+'</h2>'+button('closePanel','close')+'</header><div class="notes-panel-content" data-panel-content role="tabpanel" id="'+esc(windowId)+'-panel" aria-labelledby="'+esc(windowId)+'-tab-'+right+'"></div>';
            renderPanelBody();
        }
        const tabLabel=tab=>tr(tab==='localGraph'?'local_graph':tab);
        function renderPanelBody(){
            const content=find('[data-panel-content]');if(!content||!right)return;
            if(!editor||!current){delete content.dataset.backlinkSig;content.textContent=tr('select_note');return;}
            if(right==='localGraph'){renderLocalGraph(content);return;}
            if(right==='backlinks'||right==='outgoing'){
                const options={tr,esc,index,path:current.path,title:current.title,content:right==='outgoing'?editor.content():'',request:(url,options)=>request(url,options),signal:life.signal,cache:contextCache};
                if(right==='outgoing'){NotesPanels.outgoing(content,options);return;}
                // Every autosave refreshes the index: rebuild only when the list of linking notes changed, and keep scroll and focus when it does.
                const signature=NotesPanels.backlinksSignature(index,current.path);
                if(content.dataset.backlinkSig===signature)return;
                const scroll=content.scrollTop,focused=content.contains(document.activeElement)?document.activeElement.dataset?.notePath:null;
                content.dataset.backlinkSig=signature;
                NotesPanels.backlinks(content,options);
                content.scrollTop=scroll;
                if(focused)[...content.querySelectorAll('[data-note-path]')].find(el=>el.dataset.notePath===focused)?.focus({preventScroll:true});
                return;
            }
            if(right==='outline'){
                const headings=editor.outline();content.innerHTML=headings.length?headings.map(h=>'<button class="notes-outline-item" data-jump="'+h.pos+'" style="--level:'+h.level+'">'+esc(h.text)+'</button>').join(''):'<p class="notes-muted">'+esc(tr('outline_empty'))+'</p>';
            }else if(right==='info'){
                content.innerHTML='<p class="notes-muted">'+esc(tr('tags_in_properties'))+'</p><dl><dt>'+esc(tr('location'))+'</dt><dd>'+esc(current.path)+'</dd><dt>'+esc(tr('modified'))+'</dt><dd>'+esc(new Date(current.modified).toLocaleString())+'</dd></dl><div class="notes-permissions">'+icon('info')+'<p>'+esc(tr('agent_permissions'))+'</p></div>'+button(trash?'restore':'move',trash?'restore':'move',true)+button('trash','trash',true);
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
        async function exportNote(format,asFile=false,signal=docLife.signal){
            if(!editor)return;
            const text=editor.content(),title=current.title||tr('title'),from=current.path;
            if(format==='md'||format==='txt'){const content=format==='md'?text:editor.text,type=format==='md'?'text/markdown;charset=utf-8':'text/plain;charset=utf-8',name=basename(from).replace(/\.md$/i,'')+'.'+format;if(asFile)return [new File([content],name,{type})];download(content,type,name);return;}
            const html=DOMPurify.sanitize(marked.parse(NotesFrontmatter.strip(text)),{FORBID_TAGS:['style','iframe','form','input','script'],FORBID_ATTR:['style']});
            const wrapper=document.createElement('div');wrapper.innerHTML=html;
            for(const img of wrapper.querySelectorAll('img')){
                const url=resolveURL(img.getAttribute('src'),from);if(!url){img.remove();continue;}
                if(url.startsWith('/api/')){
                    const response=await fetch(url,{credentials:'same-origin',signal});
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
            if(format==='html'){const name=basename(from).replace(/\.md$/i,'.html'),type='text/html;charset=utf-8';if(asFile)return [new File([page],name,{type})];download(page,type,name);return;}
            await window.AuraDesktopPrint.printHTML({html:page,title:tr('print'),failureMessage:ctx.t('desktop.print_failed'),className:'notes-print-frame',signal:life.signal});
        }
        function download(content,type,name){const url=URL.createObjectURL(new Blob([content],{type})),a=document.createElement('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}
        async function act(action,event){
            if(disposed)return;
            if(!['document','insert','templates'].includes(action))find('[data-popup]').hidden=true;
            try {
                if(action==='new')return await createNote();
                if(action.startsWith('template:'))return await createNote(action.split(':')[1]);
                if(action.startsWith('utemplate:'))return await createFromTemplate(action.slice(10));
                if(action.startsWith('callout:')){if(writable()&&CALLOUTS.includes(action.slice(8)))editor.insertCallout(action.slice(8),'');return;}
                if(action==='callout'){if(writable())popup(CALLOUTS.map(type=>['callout:'+type,'callout_'+type]),find('.vd-notes-toolbar [data-action="insert"]'));return;}
                if(action==='daily')return await openDaily();
                if(action==='dailyPrev'||action==='dailyNext')return await dailyStep(action==='dailyPrev'?-1:1);
                if(action==='insertTemplate')return openInsertTemplate();
                if(action==='save')return await queue?.save();
                if(action==='retry'){notice('');return queue?.error?await queue.save():await refreshList();}
                if(action==='graph'){if(graphOpen)closeGraph();else openGraph();return;}
                if(action==='graphFit'){graph?.fit();return;}
                if(action==='graphClose')return closeGraph();
                if(action==='switcher')return openSwitcher();
                if(action==='commandPalette')return openCommands();
                if(action==='search')return openFind();
                if(action==='findNext'||action==='findPrev'){runFind(action==='findNext'?1:-1);return;}
                if(action==='findCase'){findCase=!findCase;find('[data-action="findCase"]').setAttribute('aria-pressed',String(findCase));runFind(0);return;}
                if(action==='findClose')return closeFind();
                if(action==='replaceOne'||action==='replaceAll')return replaceMatches(action==='replaceAll');
                if(action.startsWith('ctx:'))return await contextAction(action.slice(4));
                if(action==='saveAs')return await saveAs();
                if(action==='open'){const selected=await ctx.openFileDialog?.({title:tr('open'),initialPath:DIR,filters:[{label:'Markdown',extensions:['.md']}]});if(selected?.path&&!selected.canceled){if(selected.path.startsWith(DIR+'/'))await loadNote(selected.path);else await importDesktopFile(selected.path);}return;}
                if(action==='import'){find('[data-import]').click();return;}
                if(action==='source'){if(editor){const ok=await editor.mode(!editor.sourceMode);sourceRequested=editor.sourceMode;if(!ok)notice(tr('source_preserved'));syncChrome();}return;}
                if(action==='library'){root.classList.toggle('notes-hide-library');return;}
                if(action==='wikilink'&&writable()){const range=editor.selection();editor.replaceSelection('[[',{...range,from:range.to,to:range.to});editor.focus();return;}
                if(action==='focus'){root.classList.toggle('notes-focus');return;}
                if(TABS.includes(action)){setRight(right===action?'':action);return;}
                if(action==='sidebarRight'){setRight(right?'':lastTab);return;}
                if(action==='closePanel'){setRight('');return;}
                if(action==='back'||action==='forward'){await go(action==='back'?-1:1);return;}
                if(action==='dismiss'){notice('');return;}
                if(action==='showTrash'){trash=!trash;scheduleSidebar();return;}
                if(action==='pin'&&writable()){updateMeta({pinned:meta.pinned.includes(current.path)?meta.pinned.filter(p=>p!==current.path):[...meta.pinned,current.path]});return;}
                if(['trash','move','restore','rename'].includes(action))return await moveNote(action==='rename'?'move':action);
                if(action==='document'){popup([['rename','rename'],['move','move'],['saveAs','save_as'],['save','save'],['templates','templates']],event?.target||find('[data-action="document"]'));return;}
                if(action==='templates'){if(!indexLoaded)await index.load().catch(()=>{});popup([...['blank','meeting','checklist','journal'].map(kind=>['template:'+kind,'template_'+kind]),...userTemplates().map(note=>['utemplate:'+note.path,note.title||basename(note.path).replace(/\.md$/i,''),true])],event?.target||find('[data-action="document"]'));return;}
                if(action==='insert'){popup([['table','table'],['addRowAfter','insert_row'],['addColumnAfter','insert_column'],['deleteRow','delete_row'],['deleteColumn','delete_column'],['deleteTable','delete_table'],['image','image'],['attachment','attachment'],['mermaid','diagram'],['codeblock','code_block'],['rule','horizontal_rule'],['callout','callout'],['insertTemplate','insert_template']],event?.target);return;}
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
                                if(['md','txt','html','print'].includes(action))return await exportNote(action);
                if(['zoomOut','zoomIn','zoomReset'].includes(action)){zoom=action==='zoomReset'?100:Math.min(180,Math.max(70,zoom+(action==='zoomIn'?10:-10)));root.style.setProperty('--notes-zoom',zoom/100);find('[data-zoom]').textContent=zoom+'%';return;}
                if(editor&&!busy)editor.command(action);
            }catch(error){fail(error);}
        }
        // One list drives the menu bar and the command palette (Phase 5/6 append their commands here).
        // Menu labels stay short under their menu; the palette lists commands without that context.
        const PALETTE_LABELS={md:'export_as_md',html:'export_as_html',txt:'export_as_txt',trash:'palette_trash',link:'palette_link',callout:'palette_callout',info:'palette_info'};
        function commands(){
            const ro=!!ctx.readonly,w=!!writable(),e=!!editor,text=!!editor&&!editor.sourceMode;
            const c=(id,label,group,shortcut,enabled=true,extra={})=>({id,label:tr(label),paletteLabel:PALETTE_LABELS[id]?tr(PALETTE_LABELS[id]):tr(label),group,shortcut:shortcut||'',enabled:!!enabled,...extra});
            return [
                c('new','new_note','file','Ctrl+N',!ro&&!busy),c('templates','templates','file','',!ro,{palette:false}),c('daily','daily','file','',!busy),c('dailyPrev','daily_prev','file','',!!dailyNeighbor(-1)),c('dailyNext','daily_next','file','',!!dailyNeighbor(1)),c('switcher','switcher','file','Ctrl+O'),c('open','open','file','',true),c('import','import','file','',!ro),
                c('save','save','file','Ctrl+S',w),c('saveAs','save_as','file','',w),c('rename','rename','file','',w),
                ...['md','html','txt'].map(format=>c(format,'export_'+format,'file','',e)),c('print','print','file','',e),
                c('undo','undo','edit','Ctrl+Z',w),c('redo','redo','edit','Ctrl+Y',w),c('search','find','edit','Ctrl+F',e),c('insert','insert','edit','',w,{palette:false}),c('callout','callout','edit','',w),c('insertTemplate','insert_template','edit','',w),c('trash','trash','edit','',w),
                c('bold','bold','format','',w&&text),c('italic','italic','format','',w&&text),c('strike','strike','format','',w&&text),c('highlight','highlight','format','Ctrl+Shift+H',w),c('code','code','format','',w&&text),c('link','link','format','',w&&text),c('wikilink','wikilink','format','',w),
                c('commandPalette','commands','view','Ctrl+P',true,{palette:false}),c('library','library','view'),c('graph','graph','view','Ctrl+G'),c('sidebarRight','right_sidebar','view'),c('backlinks','backlinks','view'),c('outgoing','outgoing','view'),c('localGraph','local_graph','view'),c('outline','outline','view'),c('info','info','view'),c('source','markdown_source','view','Ctrl+E',e),c('focus','focus_mode','view'),
                c('assist','assist','agent','',w)
            ];
        }
        function menus(){
            const all=commands(),by=Object.fromEntries(all.map(command=>[command.id,command]));
            const entries=ids=>ids.map(id=>id==='-'?{type:'separator'}:{id,label:by[id].label,icon:menuIcons[id]||icons[id]||'notes',shortcut:by[id].shortcut||null,disabled:!by[id].enabled,action:()=>act(id)});
            ctx.setWindowMenus?.(windowId,[
                {id:'file',labelKey:'desktop.menu_file',items:entries(['new','templates','daily','dailyPrev','dailyNext','switcher','open','import','-','save','saveAs','rename','-','md','html','txt','print'])},
                {id:'edit',labelKey:'desktop.menu_edit',items:entries(['undo','redo','search','insert','callout','insertTemplate','trash'])},
                {id:'format',labelKey:'desktop.menu_format',items:entries(['bold','italic','strike','highlight','code','link','wikilink'])},
                {id:'view',label:tr('view'),items:entries(['commandPalette','-','library','graph','sidebarRight','backlinks','outgoing','localGraph','outline','info','source','focus'])},
                {id:'agent',labelKey:'desktop.menu_agent',items:entries(['assist'])}
            ]);
        }
        const groupLabel=group=>({file:ctx.t?.('desktop.menu_file')||'File',edit:ctx.t?.('desktop.menu_edit')||'Edit',format:ctx.t?.('desktop.menu_format')||'Format',view:tr('view'),agent:ctx.t?.('desktop.menu_agent')||'Agent'}[group]||group);
        // ---- graph (global view and local tab)
        function graphSource(options){
            const notes=index.notes().map(note=>note.path===current?.path&&current.title?{...note,title:current.title}:note);
            return NotesGraph.fromIndex(notes,{linkPath:(note,link)=>index.linkPath(note,link),fuzzy:NotesIndex.fuzzy,current:current?.path,...options});
        }
        function syncGraphToggles(){
            for(const key of ['tags','unresolved','orphans'])find('[data-graph-toggle="'+key+'"]')?.setAttribute('aria-pressed',String(!!graphPrefs[key]));
        }
        function updateGraph(){
            if(!graph||disposed)return;
            const data=graphSource({tags:graphPrefs.tags,unresolved:graphPrefs.unresolved,orphans:graphPrefs.orphans,filter:graphFilter});
            graphLabels=new Map(data.nodes.map(node=>[node.id,node.label]));
            graph.setData(data);
            find('[data-graph-empty]').hidden=data.nodes.length>0;
        }
        function openGraph(){
            if(disposed||!window.NotesGraph)return;
            if(graphOpen){find('[data-graph-host] canvas')?.focus({preventScroll:true});return;}
            hidePreview();find('[data-popup]').hidden=true;
            graphOpen=true;root.classList.add('notes-show-graph');find('[data-graph]').hidden=false;syncGraphToggles();
            find('[data-graph-filter]').value=graphFilter;
            const host=find('[data-graph-host]');
            graph=NotesGraph.create(host,{label:tr('graph'),onOpen:id=>openGraphNode(id),onHover:id=>{host.dataset.hover=id||'';}});
            if(!indexLoaded)index.load().catch(()=>{});
            updateGraph();syncChrome();
            host.querySelector('canvas')?.focus({preventScroll:true});
        }
        function closeGraph(focus=true){
            if(!graphOpen)return;
            graphOpen=false;graphPointer=false;graph?.destroy();graph=null;
            find('[data-graph-host]').dataset.hover='';
            find('[data-graph]').hidden=true;root.classList.remove('notes-show-graph');
            syncChrome();
            if(focus&&!disposed)editor?.focus();
        }
        async function openGraphNode(id){
            if(disposed)return;
            const label=graphLabels.get(id)||'';
            if(id.startsWith('tag:')){closeGraph(false);showLibrary();sidebar?.focusSearch('tag:'+id.slice(4));return;}
            if(id.startsWith('unresolved:')){closeGraph(false);await createWikiNote(label);return;}
            const same=id===current?.path;
            closeGraph(false);
            if(!same)await loadNote(id);
            if(current?.path===id)editor?.focus();
        }
        function destroyLocalGraph(){localGraph?.destroy();localGraph=null;}
        function updateLocalGraph(){
            if(disposed||right!=='localGraph'||!current)return;
            const host=find('[data-local-graph]');if(!host)return;
            if(!localGraph&&window.NotesGraph)localGraph=NotesGraph.create(host,{label:tr('local_graph'),onOpen:id=>openLocalNode(id),onHover:id=>{host.dataset.hover=id||'';}});
            if(!localGraph)return;
            const select=find('[data-graph-depth]');if(select)select.value=String(graphPrefs.depth);
            const data=NotesGraph.neighborhood(graphSource({tags:graphPrefs.tags,unresolved:graphPrefs.unresolved,orphans:true}),current.path,graphPrefs.depth);
            localLabels=new Map(data.nodes.map(node=>[node.id,node.label]));
            localGraph.setData(data);
        }
        function renderLocalGraph(content){
            destroyLocalGraph();
            content.innerHTML='<div class="notes-localgraph"><label class="notes-graph-depth">'+esc(tr('graph_depth'))+'<select data-graph-depth><option value="1">1</option><option value="2">2</option></select></label><div class="notes-localgraph-host" data-local-graph></div></div>';
            updateLocalGraph();
        }
        function openLocalNode(id){
            if(id.startsWith('tag:')){showLibrary();sidebar?.focusSearch('tag:'+id.slice(4));return;}
            if(id.startsWith('unresolved:')){createWikiNote(localLabels.get(id)||'');return;}
            if(id!==current?.path)loadNote(id);
        }
        // ---- daily notes, templates
        const lang=()=>document.documentElement.lang||'en';
        function userTemplates(){
            const dir=NotesTemplates.TEMPLATE_DIR+'/';
            return index.notes().filter(note=>note.path.startsWith(dir)&&/\.md$/i.test(note.path)).sort((a,b)=>String(a.title||a.path).localeCompare(String(b.title||b.path),lang()));
        }
        async function openDaily(){
            if(disposed||busy||loading||!window.NotesTemplates)return;
            const now=new Date(),path=NotesTemplates.dailyPath(now);
            if(!indexLoaded)await index.load().catch(()=>{});
            if(!index.note(path)){
                if(ctx.readonly)return;
                try{
                    const title=NotesTemplates.formatDate(now,'YYYY-MM-DD','en'),wanted=(NotesTemplates.TEMPLATE_DIR+'/Daily.md').toLowerCase(),template=index.notes().find(note=>note.path.toLowerCase()===wanted);
                    const text=template?NotesTemplates.expand(String((await request(endpoint(template.path),{signal:life.signal})).content||''),{title,date:now,lang:lang()}):NotesTemplates.defaultDaily(now,lang());
                    try{await request(endpoint(path),{method:'PUT',signal:life.signal,headers:{'Content-Type':'application/json','If-None-Match':'*'},body:JSON.stringify({path,content:text})});}
                    catch(error){if(error.status!==412)throw error;}
                    await index.load().catch(()=>{});await refreshList();
                }catch(error){fail(error);return;}
            }
            if(current?.path!==path)await loadNote(path);
            if(current?.path===path)editor?.focus();
        }
        function dailyNeighbor(direction){
            const date=current?NotesTemplates.dailyDate(current.path):'';
            if(!date||!indexLoaded)return '';
            let best='';
            for(const note of index.notes()){
                const other=NotesTemplates.dailyDate(note.path);
                if(!other||other===date)continue;
                if(direction<0?other<date&&(!best||other>best):other>date&&(!best||other<best))best=other;
            }
            return best?NotesTemplates.DAILY_DIR+'/'+best+'.md':'';
        }
        async function dailyStep(direction){
            const path=dailyNeighbor(direction);
            if(path&&!busy&&!loading)await loadNote(path);
        }
        function updateDailyNav(){
            const nav=find('[data-daily-nav]');if(!nav||!window.NotesTemplates)return;
            const date=current?NotesTemplates.dailyDate(current.path):'';
            nav.hidden=!date;if(!date)return;
            find('[data-daily-date]').textContent=date;
            find('[data-action="dailyPrev"]').disabled=!dailyNeighbor(-1)||busy;find('[data-action="dailyNext"]').disabled=!dailyNeighbor(1)||busy;
        }
        async function createFromTemplate(path){
            if(ctx.readonly||busy)return;
            const source=await request(endpoint(path),{signal:life.signal});
            await createNote('blank',NotesTemplates.expand(String(source.content||''),{title:tr('new_note'),date:new Date(),lang:lang()}));
        }
        function openInsertTemplate(){
            if(!writable()||!window.NotesPalette)return;
            if(!indexLoaded)index.load().catch(()=>{});
            const builtins=['meeting','checklist','journal'];
            NotesPalette.open(root,{placeholder:tr('insert_template'),ariaLabel:tr('insert_template'),emptyText:tr('no_results'),
                items:query=>{
                    const user=userTemplates().map(note=>({id:'u:'+note.path,label:note.title||basename(note.path).replace(/\.md$/i,''),detail:folderLabel(note.path),icon:icon('md')}));
                    const built=builtins.map(kind=>({id:'b:'+kind,label:tr('template_'+kind),detail:'',icon:icon('md')}));
                    const all=[...user,...built];
                    if(query.trim())return all.map(item=>({...item,score:NotesIndex.fuzzy(query,item.label)})).filter(item=>item.score>0).sort((a,b)=>b.score-a.score);
                    return user.length?all:[...all,{id:'',label:tr('no_templates'),detail:tr('templates_hint',{folder:NotesTemplates.TEMPLATE_DIR})}];
                },
                onPick:async item=>{
                    if(!item.id||!writable())return;
                    try{
                        const title=current?.title||tr('new_note'),opts={title,date:new Date(),lang:lang()};
                        const text=item.id.startsWith('u:')?NotesTemplates.expand(NotesFrontmatter.strip(String((await request(endpoint(item.id.slice(2)),{signal:life.signal})).content||'')),opts):template(item.id.slice(2)).replace(/^# .*\n\n/,'');
                        if(!writable())return;
                        editor.insert(text);editor.focus();
                    }catch(error){fail(error);}
                },
                footer:[{keys:'↑↓',label:tr('palette_navigate')},{keys:'↵',label:tr('palette_run')},{keys:'Esc',label:tr('palette_close')}]});
        }
        // ---- properties bar: tags and aliases as chips, other frontmatter keys read-only
        function propsData(){
            const content=editor.content(),fields=NotesFrontmatter.fields(content),keys=fields.map(field=>field.key.toLowerCase());
            const aliasKey=keys.includes('alias')&&!keys.includes('aliases')?'alias':'aliases';
            return {content,has:fields.length>0,aliasKey,tags:NotesFrontmatter.parse(content).tags,aliases:NotesFrontmatter.listValues(content,aliasKey),others:fields.filter(field=>!RESERVED_PROPS.includes(field.key.toLowerCase()))};
        }
        function renderProperties(force=false){
            const bar=find('[data-properties]');if(!bar)return;
            if(!editor||!current){bar.hidden=true;bar.replaceChildren();propsSignature='';return;}
            if(!force&&bar.contains(document.activeElement))return;
            const ro=!writable(),data=propsData(),show=data.has||(propsForce&&!ro);
            const signature=JSON.stringify([current.path,ro,propsOpen,show,data.tags,data.aliases,data.others]);
            if(!force&&signature===propsSignature)return;
            propsSignature=signature;
            if(!show){
                bar.hidden=ro;bar.dataset.state='empty';
                bar.innerHTML=ro?'':'<button type="button" class="notes-props-add" data-props-add>+ '+esc(tr('add_property'))+'</button>';
                return;
            }
            const chip=(kind,value,label,removeLabel)=>'<span class="notes-chip" data-chip="'+kind+'"><span class="notes-chip-text">'+esc(label)+'</span>'+(ro?'':'<button type="button" data-chip-remove="'+kind+'" data-value="'+esc(value)+'" title="'+esc(removeLabel)+'" aria-label="'+esc(removeLabel)+'">×</button>')+'</span>';
            const row=(kind,label,values,chipOf,placeholder,list)=>{
                if(ro&&!values.length)return '';
                return '<div class="notes-prop-row" data-prop="'+kind+'"><span class="notes-prop-key">'+esc(label)+'</span><span class="notes-prop-value notes-chips">'+values.map(chipOf).join('')+(ro?'':'<input type="text" data-prop-input="'+kind+'" autocomplete="off" spellcheck="false"'+(list?' list="'+esc(list)+'"':'')+' placeholder="'+esc(placeholder)+'" aria-label="'+esc(placeholder)+'">')+'</span></div>';
            };
            const listId=windowId+'-prop-tags',known=index.tags().map(entry=>entry.tag).filter(tag=>!data.tags.includes(tag)).slice(0,60);
            const summary=data.tags.map(tag=>'#'+tag).join(' ');
            bar.hidden=false;bar.dataset.state=propsOpen?'open':'closed';
            bar.innerHTML='<button type="button" class="notes-props-toggle" data-props-toggle aria-expanded="'+propsOpen+'"><span class="notes-props-twist" aria-hidden="true">'+icon('forward')+'</span><span>'+esc(tr('properties'))+'</span>'+(propsOpen?'':'<span class="notes-props-summary">'+esc(summary)+'</span>')+'</button>'+
                (propsOpen?'<div class="notes-props-body">'+row('tags',tr('tags'),data.tags,tag=>chip('tags',tag,'#'+tag,tr('tag_remove',{tag})),tr('tag_placeholder'),listId)+row('aliases',tr('aliases'),data.aliases,alias=>chip('aliases',alias,alias,tr('alias_remove',{alias})),tr('alias_placeholder'))+
                    data.others.map(field=>'<div class="notes-prop-row notes-prop-field" data-prop-field="'+esc(field.key)+'"><span class="notes-prop-key">'+esc(field.key)+'</span><span class="notes-prop-value">'+esc(field.value)+'</span></div>').join('')+
                    (ro?'':'<datalist id="'+esc(listId)+'">'+known.map(tag=>'<option value="'+esc(tag)+'"></option>').join('')+'</datalist>')+'</div>':'');
        }
        function propInput(kind){return find('[data-prop-input="'+kind+'"]');}
        function setProp(kind,values,refocus=true){
            if(!writable())return;
            const data=propsData(),next=kind==='tags'?NotesFrontmatter.updateTags(data.content,values):NotesFrontmatter.updateList(data.content,data.aliasKey,values);
            propsForce=false;
            if(next!==data.content)editor.setContent(next);
            renderProperties(true);if(refocus)propInput(kind)?.focus({preventScroll:true});
        }
        function addProp(kind,raw,refocus=true){
            const data=propsData(),list=kind==='tags'?data.tags:data.aliases,next=[...list];
            for(const part of String(raw||'').split(',')){
                const value=kind==='tags'?part.trim().replace(/^#+/,'').replace(/\s+/g,'-'):part.trim();
                if(value&&!next.some(existing=>existing.toLowerCase()===value.toLowerCase()))next.push(value);
            }
            if(next.length===list.length){const input=propInput(kind);if(input)input.value='';return;}
            setProp(kind,next,refocus);
        }
        function removeProp(kind,value){
            const data=propsData(),list=kind==='tags'?data.tags:data.aliases;
            setProp(kind,list.filter(existing=>existing!==value));
        }
        // ---- quick switcher, command palette
        function noteItem(path){return {id:path,label:index.note(path)?.title||basename(path).replace(/\.md$/i,''),detail:folderLabel(path),icon:icon('md')};}
        function recentPaths(){return (meta.recent||[]).filter(path=>path!==current?.path&&(!indexLoaded||index.note(path))).slice(0,RECENT_MAX);}
        function exactNote(query){
            const wanted=query.trim().toLowerCase().replace(/\.md$/i,'');
            return index.notes().some(note=>String(note.title||'').toLowerCase()===wanted||basename(note.path).replace(/\.md$/i,'').toLowerCase()===wanted);
        }
        function openSwitcher(){
            if(disposed||!window.NotesPalette)return;
            if(!indexLoaded)index.load().catch(()=>{});
            NotesPalette.open(root,{placeholder:tr('switcher_placeholder'),ariaLabel:tr('switcher'),emptyText:tr('no_results'),
                items:query=>query.trim()?index.search(query,50).map(hit=>noteItem(hit.path)):recentPaths().map(noteItem),
                onPick:async item=>{await loadNote(item.id);if(current?.path===item.id)editor?.focus();},
                createLabel:query=>!ctx.readonly&&query.trim()&&!exactNote(query)?tr('create_note_named',{name:query.trim()}):null,
                onCreate:query=>createWikiNote(query),
                footer:[{keys:'↑↓',label:tr('palette_navigate')},{keys:'↵',label:tr('palette_open')},{keys:'⇧↵',label:tr('palette_create')},{keys:'Esc',label:tr('palette_close')}]});
        }
        function openCommands(){
            if(disposed||!window.NotesPalette)return;
            NotesPalette.open(root,{placeholder:tr('command_placeholder'),ariaLabel:tr('commands'),emptyText:tr('no_results'),
                items:query=>{
                    const list=commands().filter(command=>command.enabled&&command.palette!==false).map(command=>({id:command.id,label:command.paletteLabel,detail:groupLabel(command.group),hint:command.shortcut,score:query.trim()?NotesIndex.fuzzy(query,command.paletteLabel):1}));
                    return query.trim()?list.filter(item=>item.score>0).sort((a,b)=>b.score-a.score):list;
                },
                onPick:item=>{act(item.id);},
                footer:[{keys:'↑↓',label:tr('palette_navigate')},{keys:'↵',label:tr('palette_run')},{keys:'Esc',label:tr('palette_close')}]});
        }
        // ---- find bar
        function clearFindMark(){try{CSS.highlights?.delete('notes-find');}catch(_){}}
        function markFind(){
            clearFindMark();
            if(!editor||editor.sourceMode||find('[data-findbar]').hidden||typeof Highlight==='undefined'||!CSS.highlights)return;
            const {from,to}=editor.selection();if(from===to)return;
            try{const start=editor.view.domAtPos(from),end=editor.view.domAtPos(to),range=document.createRange();range.setStart(start.node,start.offset);range.setEnd(end.node,end.offset);CSS.highlights.set('notes-find',new Highlight(range));}catch(_){}
        }
        function openFind(){
            if(!editor)return;
            const bar=find('[data-findbar]'),input=find('[data-find]'),selected=editor.selection().text;
            bar.hidden=false;
            if(selected&&!selected.includes('\n')&&selected.length<=120)input.value=selected;
            input.focus();input.select();
            if(input.value)runFind(0);
        }
        function closeFind(){
            find('[data-findbar]').hidden=true;clearFindMark();find('[data-find-count]').textContent='';
            editor?.focus();
        }
        // direction 0 = incremental (keeps the current match), 1 = next, -1 = previous
        function runFind(direction){
            const query=find('[data-find]').value,out=find('[data-find-count]');
            if(!editor||!query){out.textContent='';clearFindMark();return 0;}
            const count=editor.findText(query,undefined,false,findCase,direction<0,direction===0);
            out.textContent=tr('matches',{count});out.dataset.count=String(count);
            markFind();return count;
        }
        function replaceMatches(all){
            const query=find('[data-find]').value;
            if(!editor||!writable()||!query)return;
            const count=editor.findText(query,find('[data-replace]').value,all,findCase);
            if(all){find('[data-find-count]').textContent=tr('replaced_count',{count});find('[data-find-count]').dataset.count='0';clearFindMark();}
            else runFind(1);
        }
        on(window,'beforeunload',event=>{if(queue?.dirty){backup();event.preventDefault();event.returnValue='';}});
        // Embedded notes and hover previews show foreign content: their markup never acts as app chrome (the card's own title and create buttons are outside the excerpt body).
        const inExcerpt=event=>!!event.target.closest?.('.notes-excerpt,.notes-embed');
        on(root,'click',event=>{
            if(inExcerpt(event))return;
            const toggle=event.target.closest('[data-graph-toggle]');if(toggle){graphPrefs[toggle.dataset.graphToggle]=!graphPrefs[toggle.dataset.graphToggle];writeGraphPrefs(graphPrefs);syncGraphToggles();updateGraph();if(right==='localGraph')updateLocalGraph();return;}
            const toggleProps=event.target.closest('[data-props-toggle]');if(toggleProps){propsOpen=!propsOpen;storage.set(PROPS_KEY,propsOpen?'1':'0');renderProperties(true);find('[data-props-toggle]')?.focus({preventScroll:true});return;}
            const addProps=event.target.closest('[data-props-add]');if(addProps){propsForce=true;renderProperties(true);propInput('tags')?.focus();return;}
            const removeChip=event.target.closest('[data-chip-remove]');if(removeChip){removeProp(removeChip.dataset.chipRemove,removeChip.dataset.value);return;}
            const tab=event.target.closest('[data-right-tab]');if(tab){setRight(tab.dataset.rightTab);find('[data-right-tab="'+tab.dataset.rightTab+'"]')?.focus();return;}
            const create=event.target.closest('[data-create-wiki]');if(create){hidePreview();openWiki(create.dataset.createWiki,event,current?.path);return;}
            const attachment=event.target.closest('[data-attachment-path]');if(attachment){window.open('/api/desktop/download?path='+encodeURIComponent(attachment.dataset.attachmentPath),'_blank','noopener,noreferrer');return;}
            const crumb=event.target.closest('[data-crumb]');if(crumb){showLibrary();sidebar?.setTab('files');sidebar?.revealPath(crumb.dataset.crumb);const folder=find('.notes-tree-row[data-kind=folder][data-path="'+crumb.dataset.crumb+'"]');if(folder?.getAttribute('aria-expanded')==='false')folder.click();return;}
            const row=event.target.closest('[data-note-path]');if(row){hidePreview();loadNote(row.dataset.notePath);return;}
            const jump=event.target.closest('[data-jump]');if(jump){editor?.jump(Number(jump.dataset.jump));return;}
            const target=event.target.closest('[data-action]');if(target&&!target.disabled)act(target.dataset.action,event);
            else if(!event.target.closest('[data-popup]'))find('[data-popup]').hidden=true;
        });
        on(root,'change',event=>{
            if(inExcerpt(event))return;
            if(event.target.matches('[data-graph-depth]')){graphPrefs.depth=event.target.value==='2'?2:1;writeGraphPrefs(graphPrefs);updateLocalGraph();return;}
            if(event.target.matches('[data-style]'))editor?.command('style',event.target.value);
        });
        on(root,'input',event=>{if(inExcerpt(event))return;if(event.target.matches('[data-find]'))runFind(0);if(event.target.matches('[data-graph-filter]')){graphFilter=event.target.value;updateGraph();}});
        on(root,'focusout',event=>{
            if(inExcerpt(event))return;
            const input=event.target.closest?.('[data-prop-input]');
            if(!input||!input.value.trim()||input.closest('[data-properties]').contains(event.relatedTarget)||!writable())return;
            addProp(input.dataset.propInput,input.value,false);
        });
        on(find('[data-graph]'),'pointerenter',()=>{graphPointer=true;});on(find('[data-graph]'),'pointerleave',()=>{graphPointer=false;});
        on(document,'keydown',event=>{if(event.key==='Escape'&&graphOpen&&graphPointer&&!event.defaultPrevented&&!root.contains(event.target)&&!root.__notesPalette){event.preventDefault();closeGraph();}});
        on(find('[data-upload]'),'change',async event=>{const files=[...event.target.files];event.target.value='';const active=editor;for(const file of files)try{const image=file.type.startsWith('image/'),url=await uploadFile(file,image);if(active!==editor||disposed)return;editor.insert((image?'!':'')+'['+file.name.replace(/[\[\]\\]/g,'')+']('+url+')');}catch(error){fail(error);}});
        on(find('[data-import]'),'change',async event=>{const file=event.target.files?.[0];event.target.value='';if(!file)return;if(file.size>2*1024*1024){notice(tr('note_too_large'),true);return;}try{await createNote('blank',await file.text());}catch(error){fail(error);}});
        on(root,'keydown',event=>{
            if(root.__notesPalette)return;
            if(event.key==='Escape')hidePreview();
            if(inExcerpt(event))return;
            if(event.key==='Escape'&&graphOpen&&!event.defaultPrevented&&(event.target.closest?.('[data-graph]')||graphPointer)){event.preventDefault();event.stopPropagation();closeGraph();return;}
            const bar=event.target.closest?.('[data-findbar]');
            if(bar&&!event.ctrlKey&&!event.metaKey&&!event.altKey){
                if(event.key==='Escape'){event.preventDefault();event.stopPropagation();closeFind();return;}
                if(event.key==='Enter'&&event.target.matches('[data-find]')){event.preventDefault();runFind(event.shiftKey?-1:1);return;}
                if(event.key==='Enter'&&event.target.matches('[data-replace]')){event.preventDefault();replaceMatches(false);return;}
            }
            const propInputEl=event.target.closest?.('[data-prop-input]');
            if(propInputEl&&!event.ctrlKey&&!event.metaKey&&!event.altKey&&!event.isComposing){
                const kind=propInputEl.dataset.propInput;
                if(event.key==='Enter'||event.key===','){event.preventDefault();if(propInputEl.value.trim())addProp(kind,propInputEl.value);return;}
                if(event.key==='Backspace'&&!propInputEl.value){const data=propsData(),list=kind==='tags'?data.tags:data.aliases;if(list.length){event.preventDefault();removeProp(kind,list[list.length-1]);}return;}
                if(event.key==='Escape'){propInputEl.value='';propInputEl.blur();editor?.focus();event.stopPropagation();return;}
            }
            const crumbKey=event.target.closest?.('[data-crumb]');
            if(crumbKey&&(event.key==='Enter'||event.key===' ')){event.preventDefault();crumbKey.click();return;}
            const history=historyShortcut(event);
            if(history){event.preventDefault();act(history);return;}
            const tabs=event.target.closest?.('[role=tablist]');
            if(tabs&&['ArrowLeft','ArrowRight','Home','End'].includes(event.key)){event.preventDefault();const all=[...tabs.querySelectorAll('[role=tab]')],at=all.indexOf(event.target),next=event.key==='Home'?0:event.key==='End'?all.length-1:(at+(event.key==='ArrowRight'?1:all.length-1))%all.length;setRight(all[next].dataset.rightTab);find('[data-right-tab="'+all[next].dataset.rightTab+'"]')?.focus();return;}
            const popup=find('[data-popup]');if(!popup.hidden&&['ArrowDown','ArrowUp','Escape'].includes(event.key)){event.preventDefault();const buttons=[...popup.querySelectorAll('button')];if(event.key==='Escape'){popup.hidden=true;editor?.focus();}else buttons[(buttons.indexOf(document.activeElement)+(event.key==='ArrowDown'?1:buttons.length-1))%buttons.length]?.focus();return;}
            if(!(event.ctrlKey||event.metaKey)||event.altKey)return;
            const key=event.key.toLowerCase(),action=event.shiftKey?(key==='h'?'highlight':''):{s:'save',o:'switcher',n:'new',p:'commandPalette',f:'search',e:'source',g:'graph'}[key];
            if(action){event.preventDefault();event.stopPropagation();act(action);}
        });
        on(find('[data-preview]'),'mouseenter',()=>clearTimeout(previewTimer));
        on(find('[data-preview]'),'mouseleave',()=>{previewTimer=setTimeout(hidePreview,200);});
        on(find('[data-scroll]'),'scroll',()=>{if(!find('[data-preview]').hidden&&Math.abs(find('[data-scroll]').scrollTop-previewScroll)>4)hidePreview();});
        const resize=new ResizeObserver(entries=>root.classList.toggle('notes-narrow',entries[0].contentRect.width<900));resize.observe(root);
        const desktopEvent=event=>{
            if(event?.type!=='desktop_changed')return;
            const payload=event.payload||{};if(![payload.path,payload.old_path,payload.new_path].some(path=>path?.startsWith(DIR+'/')||path?.startsWith('Trash/Notes/')))return;
            index.refresh().catch(()=>{});
            clearTimeout(sseTimer);sseTimer=setTimeout(async()=>{try{await refreshList();if(current&&!queue?.dirty&&!queue?.pending&&!busy&&!navigating){const updated=await request(endpoint(current.path),{signal:life.signal});if(updated.version!==current.version)await loadNote(updated.path,updated);}}catch(error){fail(error);}},500);
        };
        window.AuraSSE?.on?.('virtual_desktop_event',desktopEvent);
        function cleanup(){
            if(disposed)return;disposed=true;life.abort();docLife.abort();assistController?.abort();resize.disconnect();window.AuraSSE?.off?.('virtual_desktop_event',desktopEvent);
            for(const timer of [updateTimer,draftTimer,draftDeadline,sseTimer,previewTimer])clearTimeout(timer);
            cancelAnimationFrame(sidebarFrame);graph?.destroy();graph=null;destroyLocalGraph();root.__notesPalette?.close(true);clearFindMark();sidebar?.destroy();sidebar=null;
            queue?.dispose();editor?.destroy();ctx.clearWindowMenus?.(windowId);instances.delete(windowId);
        }
        instance.ready=(async()=>{
            try{const saved=await request(endpoint(META),{signal:life.signal}).catch(error=>{if(error.status!==404)throw error;return null;});
                if(saved){try{const data=JSON.parse(saved.content);if(data&&typeof data==='object')meta={...meta,...data,sort:data.sort==='name'?'name':'modified',pinned:Array.isArray(data.pinned)?data.pinned.filter(path=>typeof path==='string'&&path.startsWith(DIR+'/')):[],recent:Array.isArray(data.recent)?data.recent.filter(path=>typeof path==='string'&&path.startsWith(DIR+'/')).slice(0,RECENT_MAX):[]};}catch(_){notice(tr('request_failed'),true);}}
                if(disposed)return;await Promise.all([refreshList(),index.load().catch(()=>{})]);if(indexLoaded)meta.recent=meta.recent.filter(path=>index.note(path));const initial=ctx.path||meta.last_note;
                if(initial&&list.some(note=>note.path===initial))await loadNote(initial);
                else if(ctx.path){if(ctx.path.startsWith(DIR+'/'))await loadNote(ctx.path);else await importDesktopFile(ctx.path);}
                else if(list.length)await loadNote(list[0].path);
            }catch(error){fail(error);}finally{if(!disposed)syncChrome();}
        })();
        sidebar=NotesSidebar.create(find('[data-sidebar]'),{tr,icon:name=>name==='tag'||name==='collapse'?'':ctx.iconMarkup?.(name,'','notes-icon',16,'action')||'',storage,getState:sidebarState,root:DIR,
            onOpen:path=>{hidePreview();loadNote(path);},onMove:moveToFolder,onNewNote:folder=>createNote('blank',undefined,folder),onNewFolder:newFolder,onContextMenu:openContextMenu,
            onToggleTrash:()=>act('showTrash'),onSort:sort=>updateMeta({sort}),onSearch:searchNotes});
        syncChrome();
        return instance;
    }
    function dispose(id){instances.get(id)?.dispose();}
    window.NotesApp={render,dispose,instances,resolveLinkPath:resolveNoteLinkPath};
})();
