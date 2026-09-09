(function () {
    'use strict';
    const instances = new Map();
    const MIME = 'application/vnd.openxmlformats-officedocument.wordprocessingml.document';
    let enginePromise, fontsPromise;
    const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const basename = path => String(path).split('/').pop();
    const newPath = () => 'Documents/untitled-' + crypto.randomUUID().slice(0, 8) + '.docx';
    const nativeURL = path => '/api/desktop/office/document?representation=docx&path=' + encodeURIComponent(path);
    async function responseOK(response) {
        if (response.ok) return response;
        const body = await response.json().catch(() => ({}));
        const error = new Error(body.error || body.message || ('HTTP ' + response.status));
        error.status = response.status;
        throw error;
    }
    function render(host, windowId, context) {
        dispose(windowId);
        const ctx = context || {}, esc = ctx.esc || escapeHTML;
        const tr = (key, params) => ctx.t?.('desktop.writer_' + key, params) || key;
        const icon = (key, fallback) => ctx.iconMarkup?.(key, fallback, 'writer-icon', 18, 'action') || esc(fallback);
        const button = (action, label, symbol) => '<button type="button" data-action="' + action + '" title="' + esc(tr(label)) + '" aria-label="' + esc(tr(label)) + '">' + icon(action, symbol) + '</button>';
        const slotButton = (slot, label, symbol) => '<button type="button" data-slot="' + slot + '" title="' + esc(tr(label)) + '" aria-label="' + esc(tr(label)) + '">' + icon(label, symbol) + '</button>';
        host.innerHTML = '<div class="writer-app" data-writer="' + esc(windowId) + '">' +
            '<header class="writer-documentbar"><div class="writer-document"><button class="writer-document-name" data-action="document"><span data-name></span><span aria-hidden="true">⌄</span></button><span data-save-state role="status" aria-live="polite">' + esc(tr('loading')) + '</span></div>' +
            '<nav class="writer-views" aria-label="' + esc(tr('panels')) + '">' + button('outline','outline','☷') + button('search','find','⌕') + '<span class="writer-divider"></span>' +
            button('format','format','A') + button('review','review','☷') + button('assist','assist','✦') + '</nav></header>' +
            '<div class="writer-document-menu" data-document-menu hidden><strong data-location></strong><button data-action="rename">' + esc(tr('rename')) + '</button><button data-action="saveAs">' + esc(tr('save_as')) + '</button><button data-action="save">' + esc(tr('save')) + '</button></div>' +
            '<div class="writer-toolbar" role="toolbar" aria-label="' + esc(tr('format')) + '">' +
            '<div class="writer-toolgroup">' + slotButton('history.undo','undo','↶') + slotButton('history.redo','redo','↷') + '</div>' +
            '<div class="writer-toolgroup writer-fonts"><select data-slot="styles.style" aria-label="' + esc(tr('style')) + '"></select><select data-slot="font.family" aria-label="' + esc(tr('font')) + '"></select><input type="number" data-slot="font.size" min="1" max="200" step=".5" value="11" aria-label="' + esc(tr('size')) + '"></div>' +
            '<div class="writer-toolgroup">' + slotButton('text.bold','bold','B') + slotButton('text.italic','italic','I') + slotButton('text.underline','underline','U') + '</div>' +
            '<div class="writer-toolgroup">' + slotButton('alignment.left','align_left','≡') + slotButton('alignment.center','align_center','≡') + slotButton('alignment.right','align_right','≡') + '</div>' +
            '<div class="writer-toolgroup">' + slotButton('list.bullet','bullets','•') + slotButton('list.numbered','numbering','1.') + slotButton('list.outdent','outdent','⇤') + slotButton('list.indent','indent','⇥') + '</div>' +
            button('insert','insert','＋') + '</div>' +
            '<div class="writer-notice" data-notice role="alert" hidden><span data-notice-text></span><button data-action="retry">' + esc(tr('retry')) + '</button><button data-action="saveAs">' + esc(tr('save_as')) + '</button><button data-action="dismiss" aria-label="' + esc(tr('close')) + '">×</button></div>' +
            '<div class="writer-workspace"><aside class="writer-left" data-left hidden></aside><main class="writer-canvas"><div class="writer-ruler" aria-hidden="true"><span>0</span><span>2</span><span>4</span><span>6</span><span>8</span><span>10</span><span>12</span><span>14</span><span>16</span></div><div class="writer-viewport docx-editor__scroll-container" data-scroll><div class="writer-editor docx-editor" data-editor></div></div><div class="writer-loading" data-loading>' + esc(tr('loading')) + '</div></main><aside class="writer-right" data-right hidden></aside></div>' +
            '<footer class="writer-statusbar"><button data-action="page" data-page></button><span data-count></span><span class="writer-status-spacer"></span><span data-language></span>' +
            button('focus','focus_mode','⛶') + '<button data-action="zoomOut" aria-label="' + esc(tr('zoom_out')) + '">−</button><button data-action="fit" data-zoom title="' + esc(tr('fit')) + '">100%</button><button data-action="zoomIn" aria-label="' + esc(tr('zoom_in')) + '">+</button></footer><input data-image type="file" accept="image/png,image/jpeg,image/webp" hidden></div>';
        const root = host.firstElementChild, find = selector => root.querySelector(selector), mount = find('[data-editor]');
        let editor, lib, queue, path = ctx.path || newPath(), etag = null, generation = 0, disposed = false, fileOperation = false;
        let loadFailed = false, loading = true, pin = null, updateTimer, draftTimer, draftDeadline, lastDraft = null;
        let statsRevision=-1, rulerSignature='', documentLanguage='', documentFonts;
        let leftPanel = '', rightPanel = '', author = localStorage.getItem('aurago.writer.author') || 'AuraGo';
        const lifecycle = new AbortController();
        let documentLife = new AbortController();
        const locale = document.documentElement.lang || 'en';
        const state = {ctx,esc,tr,root,find,get editor(){return editor;},get lib(){return lib;},
            get revision(){return queue?.revision || 0;}, get readonly(){return !!ctx.readonly;},
            run,can,notice,prompt:(key,value)=>ctx.promptDialog(tr(key),value || ''),act,refresh:()=>refresh(),pinSelection,
            get author(){return author;},set author(value){author=value;localStorage.setItem('aurago.writer.author',value);editor.setAuthor(value);}
        };
        const panels = window.WriterPanels.create(state);
        const instance = {dispose:cleanup,get editor(){return editor;},get session(){return queue;},get path(){return path;},act};
        instances.set(windowId,instance);
        ctx.wireContextMenuBoundary?.(host);
        ctx.setWindowBeforeClose?.(windowId,guard);
        ctx.registerWindowCleanup?.(windowId,cleanup);
        function pinSelection() {
            if (!editor) return;
            if (pin) editor.releaseSelection(pin);
            pin = editor.retainSelection();
        }
        function notice(message,error) {
            if (disposed) return;
            find('[data-notice-text]').textContent = message || '';
            find('[data-notice]').hidden = !message;
            find('[data-notice]').dataset.error = String(!!error);
        }
        function fail(error) {
            if (error?.name === 'AbortError' || disposed) return;
            notice(error?.status === 412 ? tr('conflict') : (error?.message || String(error)),true);
        }
        function can(command) {
            if(!editor || !command)return {ok:false};
            if(editor.snapshot().editingMode==='suggesting' && /^(insert(Table|Row|Column|Image|Note|Toc|Break|PageField|Hyperlink)|delete(Table|Row|Column|Note)|mergeCells|splitCell|set(PageSetup|TableBorders|CellFill|ImageProperties|ImageWrapType|NoteProperties)|convertNote)$/.test(command.type))
                return {ok:false,reason:tr('tracking_unsupported')};
            return ['mergeCells','splitCell'].includes(command.type)?lib.tableCommand(editor,command,true):editor.can(command);
        }
        function run(command) {
            if (!editor || !command || loading) return false;
            const allowed=can(command);if(!allowed.ok){notice(tr('command_unavailable')+(allowed.reason?' · '+allowed.reason:''),true);return false;}
            const result = ['mergeCells','splitCell'].includes(command.type) ? lib.tableCommand(editor,command) : editor.exec(command);
            if (!result.ok) notice(tr('command_unavailable') + (result.reason ? ' · ' + result.reason : ''),true);
            refresh();
            return result.ok;
        }
        function draftKey(){return location.origin+':'+path;}
        async function storeDraft(bytes,revision) {
            try {
                lastDraft = {id:crypto.randomUUID(),bytes,revision,etag,updated:Date.now(),path};
                await WriterSession.draft('put',draftKey(),lastDraft);return true;
            } catch (_) {notice(tr('draft_unavailable'),true);return false;}
        }
        function scheduleDraft() {
            clearTimeout(draftTimer);
            const backup = async () => {
                clearTimeout(draftTimer);clearTimeout(draftDeadline);draftTimer=draftDeadline=null;
                const active=editor,key=draftKey(),revision=queue?.revision;
                try {
                    const bytes=await active.save();
                    if(disposed || editor!==active || !queue?.dirty || key!==draftKey())return;
                    await storeDraft(bytes,revision);
                } catch(error){fail(error);}
            };
            draftTimer=setTimeout(backup,600);
            if(!draftDeadline)draftDeadline=setTimeout(backup,4000);
        }
        async function write(bytes,target=path,expected=etag) {
            const response=await responseOK(await fetch(nativeURL(target),{method:'PUT',credentials:'same-origin',signal:documentLife.signal,
                headers:{'Content-Type':MIME,...(expected?{'If-Match':expected}:{'If-None-Match':'*'})},body:bytes}));
            if(target===path)etag=response.headers.get('ETag');
            return response.headers.get('ETag');
        }
        function makeQueue() {
            queue=WriterSession.create({readonly:ctx.readonly,serialize:()=>editor.save(),backup:storeDraft,write:bytes=>write(bytes),
                clearBackup:async()=>{
                    const key=draftKey(),draft=await WriterSession.draft('get',key).catch(()=>null);
                    if(draft && lastDraft && draft.id===lastDraft.id)
                        await WriterSession.draft('delete',key).catch(()=>{});
                },
                onState:value=>{
                    find('[data-save-state]').textContent=tr(value.error?'save_failed':value.saving?'saving':value.dirty?'unsaved':'saved');
                    find('[data-save-state]').dataset.state=value.error?'error':value.dirty?'dirty':'saved';
                    if(value.error)fail(value.error);
                }
            });
        }
        async function load(target,template) {
            const token=++generation;
            documentLife.abort();documentLife=new AbortController();const requestLife=documentLife;
            queue?.dispose();queue=null;editor?.destroy();editor=null;panels.reset();
            clearTimeout(updateTimer);clearTimeout(draftTimer);clearTimeout(draftDeadline);
            draftTimer=draftDeadline=null;lastDraft=null;statsRevision=-1;rulerSignature='';
            mount.replaceChildren();path=target;etag=null;loading=true;loadFailed=false;pin=null;
            find('[data-loading]').hidden=false;find('[data-loading]').textContent=tr('loading');
            find('[data-name]').textContent=basename(path);find('[data-location]').textContent=path;notice('');
            find('[data-save-state]').textContent=tr('loading');
            ctx.updateWindowContext?.(windowId,{path});
            try {
                enginePromise ||= import('/js/vendor/writer/engine.js').catch(error=>{enginePromise=null;throw error;});lib=await enginePromise;
                fontsPromise ||= lib.defaultFonts().then(async result=>result.failures.length?lib.defaultFonts():result);
                let fonts=await fontsPromise;
                if(fonts.failures.length){fontsPromise=null;throw new Error(tr('fonts_failed'));}
                if(disposed || token!==generation)return;
                let bytes='blank',imported=null,loadedETag=null,loadedPath=target;
                if(!template) {
                    if(/\.docx$/i.test(target)) {
                        const response=await responseOK(await fetch(nativeURL(target),{signal:requestLife.signal,cache:'no-store'}));
                        loadedETag=response.headers.get('ETag');bytes=await response.arrayBuffer();
                    } else {
                        const response=await responseOK(await fetch('/api/desktop/office/document?path='+encodeURIComponent(target),{signal:requestLife.signal}));
                        imported=(await response.json()).document;
                        if(!imported || (typeof imported.text!=='string' && typeof imported.html!=='string'))throw new Error(tr('import_failed'));
                        if(/\.md$/i.test(target))imported.html=window.marked.parse(imported.text || '',{async:false});
                        loadedPath=target.replace(/\.[^.\/]+$/,'')+'-import-'+crypto.randomUUID().slice(0,6)+'.docx';
                    }
                }
                if(disposed || token!==generation)return;
                path=loadedPath;etag=loadedETag;
                const resources=lib.documentResources(bytes,fonts);fonts=resources.fonts;documentFonts=fonts;documentLanguage=resources.language;
                if(imported && ctx.readonly) {
                    const preview=lib.createDocxEditor({container:mount,document:'blank',fonts,modules:[lib.reviewModule]});
                    try {
                        if(preview.snapshot().isOpening)await preview.save();
                        const result=preview.exec({type:'paste',text:imported.text || '',html:sanitize(imported.html || '')});
                        if(!result.ok)throw new Error(tr('import_failed'));
                        restoreHTMLHeadings(preview,imported.html || '');
                        bytes=await preview.save();
                    } finally {preview.destroy();}
                }
                if(disposed || token!==generation)return;
                editor=lib.createDocxEditor({container:mount,document:bytes,fonts,author,locale,modules:[lib.reviewModule],zoomMode:{type:'fit',fit:'pageWidth',minZoom:.25,maxZoom:1},mode:ctx.readonly?'view':undefined,revisionStyles:'kind',onFontError:()=>notice(tr('fonts_failed'),true)});
                const mounted=editor;
                if(mounted.snapshot().isOpening)await mounted.save();
                if(disposed || token!==generation || editor!==mounted)return;
                if(editor.snapshot().parseError)throw new Error(String(editor.snapshot().parseError));
                loading=false;makeQueue();
                editor.on('change',()=>{if(!loading){queue.changed();scheduleDraft();refreshSoon();}});
                editor.on('selectionChange',refreshSoon);editor.on('error',fail);
                find('[data-scroll]').addEventListener('scroll',refreshSoon,{signal:documentLife.signal,passive:true});
                if(template || imported) {
                    if(!ctx.readonly) {
                        run({type:'setPageSetup',pageWidth:11906,pageHeight:16838,marginTop:1417,marginBottom:1417,marginLeft:1417,marginRight:1417});
                        const html=imported?sanitize(imported.html || ''):template!=='blank'?panels.template(template):'';
                        if(imported || html){run({type:'paste',text:imported?.text || '',html});restoreHTMLHeadings(editor,html);}
                    }
                    if(imported)notice(tr('import_copy'));
                }
                find('[data-loading]').hidden=true;
                find('[data-name]').textContent=basename(path);find('[data-location]').textContent=path;
                find('[data-save-state]').textContent=tr(queue.dirty?'unsaved':'saved');
                ctx.updateWindowContext?.(windowId,{path});
                fillToolbar();setMenus();refresh();
                const draft=await WriterSession.draft('get',draftKey()).catch(()=>null);
                if(disposed || token!==generation)return;
                const restore=draft?.bytes && !ctx.readonly && await ctx.confirmDialog(tr('recover'),tr('recover_detail'));
                if(disposed || token!==generation)return;
                if(restore) {
                    loading=true;editor.load(draft.bytes);loading=false;etag=draft.etag || null;queue.changed();notice(tr('recovered'));refresh();
                }
                const warnings=[];
                if(resources.substitutions.length)warnings.push(tr('font_substitution')+' '+resources.substitutions.join(', '));
                if(resources.unsupported.length)warnings.push(tr('preserved_parts')+' '+resources.unsupported.join(', '));
                if(warnings.length)notice(warnings.join(' · '));
            } catch(error) {
                if(disposed || token!==generation || error.name==='AbortError')return;
                loading=true;loadFailed=true;editor?.destroy();editor=null;queue?.dispose();queue=null;mount.replaceChildren();
                find('[data-loading]').textContent=tr('load_failed');find('[data-save-state]').textContent=tr('load_failed');
                fail(error);setMenus();
            }
        }
        function sanitize(html) {
            if(!window.DOMPurify)throw new Error(tr('import_failed'));
            return DOMPurify.sanitize(html,{USE_PROFILES:{html:true},FORBID_TAGS:['iframe','form','input','object','embed'],FORBID_ATTR:['srcset']});
        }
        function restoreHTMLHeadings(target,html) {
            // Core's HTML importer preserves heading appearance but not paragraph styles.
            const headings=new DOMParser().parseFromString(html,'text/html').querySelectorAll('h1,h2,h3,h4,h5,h6');
            if(!headings.length)return;
            const paragraphs=target.query({type:'paragraphs'}),ids=target.surface.session.paragraphIds();
            const selection=structuredClone(target.surface.state().selection),normalize=text=>text.replace(/\s+/g,' ').trim();
            let cursor=0;
            for(const heading of headings) {
                const text=normalize(heading.textContent);
                const index=paragraphs.findIndex((p,i)=>i>=cursor && normalize(p.text)===text);
                if(index<0 || !text)continue;
                cursor=index+1;
                target.exec({type:'setSelection',range:{anchor:{paragraphId:ids[index],offset:0},head:{paragraphId:ids[index],offset:0}}});
                const result=target.exec(lib.commandForSlotValue('styles.style','Heading'+heading.tagName.slice(1)));
                if(!result.ok)throw new Error(tr('import_failed'));
            }
            if(selection)target.exec({type:'setSelection',range:selection});
        }
        function fillToolbar() {
            const option=(value,label)=>'<option value="'+esc(value)+'">'+esc(label)+'</option>';
            find('[data-slot="styles.style"]').innerHTML=editor.getDocumentStyles().filter(x=>x.type==='paragraph').map(x=>option(x.styleId,x.name)).join('');
            find('[data-slot="font.family"]').innerHTML=editor.getAvailableFonts().map(x=>option(x,x)).join('');
        }
        function refreshSoon(){clearTimeout(updateTimer);updateTimer=setTimeout(refresh,75);}
        function refresh() {
            if(!editor || disposed || loading)return;
            const snapshot=editor.snapshot(),fmt=snapshot.formatting || {};
            for(const el of root.querySelectorAll('.writer-toolbar [data-slot]')) {
                const slot=el.dataset.slot;
                if(el.matches('button')) {
                    const command=lib.commandForSlot(slot),allowed=command && can(command);
                    el.disabled=!allowed?.ok;
                    el.title=el.getAttribute('aria-label')+(!allowed?.ok?' · '+tr('command_unavailable')+(allowed?.reason?': '+allowed.reason:''):'');
                    el.setAttribute('aria-pressed',String(!!command && editor.isActive(command)));
                } else if(document.activeElement!==el) {
                    el.value=slot==='styles.style'?fmt.styleId || 'Normal':slot==='font.family'?fmt.fontFamily || 'Calibri':fmt.fontSizePt || 11;
                    const allowed=can(lib.commandForSlotValue(slot,el.type==='number'?Number(el.value):el.value));
                    el.disabled=!allowed.ok;el.title=allowed.ok?'':tr('command_unavailable')+(allowed.reason?' · '+allowed.reason:'');
                }
            }
            find('[data-page]').textContent=tr('page_count',{current:editor.getCurrentPage('viewport'),total:snapshot.page.total});
            if(statsRevision!==queue.revision) {
                statsRevision=queue.revision;
                const text=editor.surface.session.bodyText();
                const words=text.trim()?(Intl.Segmenter?[...new Intl.Segmenter(locale,{granularity:'word'}).segment(text)].filter(x=>x.isWordLike).length:text.trim().split(/\s+/).length):0;
                find('[data-count]').textContent=tr('word_count',{count:words})+' · '+tr('char_count',{count:[...text].length});
            }
            const setup=snapshot.pageSetup,scale=editor.getZoom()/15,signature=JSON.stringify([setup,scale]);
            if(signature!==rulerSignature) {
                rulerSignature=signature;
                const ruler=find('.writer-ruler'),width=(setup.pageWidthTwips-setup.marginsTwips.left-setup.marginsTwips.right)*25.4/1440;
                ruler.style.width=(setup.pageWidthTwips*scale)+'px';ruler.style.paddingInline=setup.marginsTwips.left*scale+'px '+setup.marginsTwips.right*scale+'px';
                ruler.innerHTML=Array.from({length:Math.floor(width/20)+1},(_,i)=>'<span>'+i*2+'</span>').join('');
            }
            find('[data-language]').textContent=(documentLanguage || tr('language_unknown')).toUpperCase();find('[data-zoom]').textContent=Math.round(editor.getZoom()*100)+'%';
            panels.update(leftPanel,rightPanel);
        }
        async function guard() {
            if(fileOperation)return false;
            if(!queue?.dirty)return true;
            try{await queue.save();return !queue.dirty;}
            catch(_) {
                if(await ctx.confirmDialog(tr('unsaved'),tr('discard_confirm'))) {
                    if(editor)return storeDraft(await editor.save(),queue.revision);
                    return false;
                }
                return false;
            }
        }
        async function saveAs() {
            if(!editor || ctx.readonly || fileOperation)return;
            const sourceEditor=editor;
            const choice=await ctx.saveFileDialog({title:tr('save_as'),initialPath:path.split('/').slice(0,-1).join('/'),defaultName:basename(path),defaultExtension:'.docx',filters:[{label:'DOCX',extensions:['.docx']}]});
            if(!choice?.path || choice.canceled || editor!==sourceEditor || disposed)return;
            if(choice.path===path)return queue.save();
            if(fileOperation)return;fileOperation=true;
            const activeQueue=queue;activeQueue.suspend();
            try {
            await activeQueue.pending?.catch(()=>{});
            const captured=activeQueue.revision,bytes=await editor.save();
            const newETag=await write(bytes,choice.path,null);
            if(disposed)return;
            const changed=activeQueue.revision!==captured;
            queue.dispose();path=choice.path;etag=newETag;panels.reset();makeQueue();if(changed)queue.changed();
            find('[data-name]').textContent=basename(path);find('[data-location]').textContent=path;
            find('[data-save-state]').textContent=tr(changed?'unsaved':'saved');
            ctx.updateWindowContext?.(windowId,{path});notice(tr('saved'));ctx.loadBootstrap?.();
            } finally {fileOperation=false;if(queue===activeQueue)activeQueue.resume();}
        }
        async function rename() {
            if(!editor || ctx.readonly || fileOperation)return;
            const sourceEditor=editor;
            const name=await ctx.promptDialog(tr('rename'),basename(path));
            if(!name || name===basename(path) || editor!==sourceEditor || disposed)return;
            if(/[\\/]/.test(name) || !/\.docx$/i.test(name)){notice(tr('name_docx'),true);return;}
            if(fileOperation)return;fileOperation=true;
            const activeQueue=queue;activeQueue.suspend();
            try {
            if(!etag && !queue.dirty)queue.changed();
            await queue.save();
            const oldPath=path,target=path.split('/').slice(0,-1).concat(name).join('/');
            const response=await responseOK(await fetch(nativeURL(oldPath),{signal:documentLife.signal,cache:'no-store'}));
            if(response.headers.get('ETag')!==etag){const error=new Error(tr('conflict'));error.status=412;throw error;}
            const bytes=new Uint8Array(await response.arrayBuffer()),prefix=new TextEncoder().encode(target+'\0');
            const hashInput=new Uint8Array(prefix.length+bytes.length);hashInput.set(prefix);hashInput.set(bytes,prefix.length);
            const digest=await crypto.subtle.digest('SHA-256',hashInput);
            const targetETag='"'+[...new Uint8Array(digest)].map(x=>x.toString(16).padStart(2,'0')).join('')+'"';
            await responseOK(await fetch('/api/desktop/file',{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({old_path:oldPath,new_path:target}),signal:documentLife.signal}));
            path=target;etag=targetETag;find('[data-name]').textContent=name;find('[data-location]').textContent=path;
            ctx.updateWindowContext?.(windowId,{path});ctx.loadBootstrap?.();
            } finally {fileOperation=false;if(queue===activeQueue)activeQueue.resume();}
        }
        function showPanel(side,value) {
            if(side==='left')leftPanel=leftPanel===value?'':value;else rightPanel=rightPanel===value?'':value;
            panels.show(side,side==='left'?leftPanel:rightPanel);
            root.classList.toggle('has-left',!!leftPanel);root.classList.toggle('has-right',!!rightPanel);
            for(const el of root.querySelectorAll('.writer-views button'))el.setAttribute('aria-pressed',String([leftPanel,rightPanel].includes(el.dataset.action)));
            refresh();
        }
        async function insertImage(file) {
            if(!editor || ctx.readonly || !/^image\/(png|jpeg|webp)$/.test(file.type))throw new Error(tr('image_format'));
            if(editor.snapshot().editingMode==='suggesting')throw new Error(tr('tracking_unsupported'));
            if(file.size>20*1024*1024)throw new Error(tr('image_large'));
            const active=editor,revision=editor.getDocumentHandle().revision,bitmap=await createImageBitmap(file);
            const command={type:'insertImage',data:new Uint8Array(await file.arrayBuffer()),mime:file.type,widthPoints:bitmap.width*.75,heightPoints:bitmap.height*.75,expectedPackageRevision:revision,description:file.name || ''};
            bitmap.close();if(disposed || editor!==active)return;
            const result=await editor.executeImageCommand(command);if(!result.ok)throw new Error(result.reason || tr('command_unavailable'));
        }
        async function act(action) {
            try {
                if(['outline','search'].includes(action))return showPanel('left',action);
                if(['format','review','assist','insert'].includes(action))return showPanel('right',action);
                switch(action) {
                    case 'document':find('[data-document-menu]').hidden=!find('[data-document-menu]').hidden;break;
                    case 'dismiss':notice('');break;
                    case 'retry':if(loadFailed)await load(path);else await queue?.save();break;
                    case 'save':if(queue && !etag && !queue.dirty)queue.changed();await queue?.save();break;
                    case 'saveAs':await saveAs();break;
                    case 'rename':await rename();break;
                    case 'open':{
                        const choice=await ctx.openFileDialog({title:tr('open'),initialPath:'Documents',filters:[{label:tr('documents'),extensions:['.docx','.html','.htm','.md','.txt']}]});
                        if(choice?.path && !choice.canceled && await guard())await load(choice.path);break;
                    }
                    case 'new':if(!ctx.readonly && await guard()){await load(newPath(),'blank');showPanel('right','insert');}break;
                    case 'blank':case 'letter':case 'report':case 'meeting':if(!ctx.readonly && await guard())await load(newPath(),action);break;
                    case 'focus':root.classList.toggle('writer-focus');break;
                    case 'fit':editor?.setZoomMode({type:'fit',fit:'pageWidth',minZoom:.25,maxZoom:1});refresh();break;
                    case 'zoomIn':case 'zoomOut':editor?.setZoom(Math.max(.25,Math.min(3,editor.getZoom()+(action==='zoomIn'?.1:-.1))));refresh();break;
                    case 'page':{const value=await ctx.promptDialog(tr('go_page'),String(editor?.getCurrentPage() || 1));if(value && editor)editor.scrollToPage(Math.max(1,Math.min(editor.getTotalPages(),Number(value)||1)));break;}
                    case 'image':find('[data-image]').click();break;
                    case 'print':if(editor)await panels.print(await editor.save(),lib,documentFonts);break;
                    case 'docx':if(editor)download(await editor.save(),MIME,basename(path));break;
                    case 'html':case 'md':case 'txt':await exportSimple(action);break;
                }
            } catch(error){if(action==='print')notice(ctx.t('desktop.print_failed'),true);else fail(error);}
        }
        function download(bytes,type,name) {
            const url=URL.createObjectURL(new Blob([bytes],{type})),link=document.createElement('a');
            link.href=url;link.download=name;link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
        }
        async function exportSimple(format) {
            if(!editor || !await ctx.confirmDialog(tr('export'),tr('export_loss')))return;
            const bytes=await editor.save();
            const response=await responseOK(await fetch('/api/desktop/office/export?format='+format,{method:'POST',headers:{'Content-Type':MIME},body:bytes,signal:documentLife.signal}));
            download(await response.arrayBuffer(),response.headers.get('Content-Type'),basename(path).replace(/\.docx$/i,'.'+format));
        }
        function setMenus() {
            const item=(id,label,shortcut,disabled)=>({id,label:tr(label),icon:id,shortcut,disabled,action:()=>act(id)});
            ctx.setWindowMenus?.(windowId,[
                {id:'file',labelKey:'desktop.menu_file',items:[
                    item('new','new','Ctrl+N',ctx.readonly),item('open','open','Ctrl+O'),
                    item('save','save','Ctrl+S',ctx.readonly || !editor),item('saveAs','save_as',null,ctx.readonly || !editor),
                    {type:'separator'},item('docx','download_docx',null,!editor),item('html','export_html',null,!editor),item('md','export_md',null,!editor),item('txt','export_txt',null,!editor),
                    {type:'separator'},item('print','print','Ctrl+P',!editor)]},
                {id:'edit',labelKey:'desktop.menu_edit',items:[
                    {id:'undo',label:tr('undo'),shortcut:'Ctrl+Z',action:()=>run({type:'undo'})},
                    {id:'redo',label:tr('redo'),shortcut:'Ctrl+Y',action:()=>run({type:'redo'})},
                    item('search','find','Ctrl+F'),item('review','review'),item('format','format'),item('insert','insert')]},
                {id:'agent',labelKey:'desktop.menu_agent',items:[item('assist','assist')]},
                {id:'view',label:tr('view'),items:[item('outline','outline'),item('focus','focus_mode'),item('fit','fit')]}
            ]);
        }
        root.addEventListener('pointerdown',event=>{if(event.target.closest('.writer-toolbar,.writer-right,.writer-views'))pinSelection();},{signal:lifecycle.signal});
        root.addEventListener('click',event=>{
            const control=event.target.closest('[data-action],[data-slot]');if(!control || control.disabled)return;
            if(control.dataset.action)void act(control.dataset.action);else if(control.matches('button'))run(lib?.commandForSlot(control.dataset.slot));
        },{signal:lifecycle.signal});
        root.addEventListener('change',event=>{const el=event.target;if(el.dataset.slot)run(lib.commandForSlotValue(el.dataset.slot,el.type==='number'?Number(el.value):el.value));},{signal:lifecycle.signal});
        find('[data-image]').addEventListener('change',event=>{const file=event.target.files[0];if(file)insertImage(file).catch(fail);event.target.value='';},{signal:lifecycle.signal});
        for(const type of ['paste','drop'])mount.addEventListener(type,event=>{
            const files=[...(event.clipboardData?.files || event.dataTransfer?.files || [])].filter(x=>x.type.startsWith('image/'));
            if(!files.length){
                if(type==='paste' && editor?.snapshot().editingMode==='suggesting' && event.clipboardData?.types.includes('text/html')){
                    event.preventDefault();event.stopImmediatePropagation();run({type:'paste',text:event.clipboardData.getData('text/plain')});
                }
                return;
            }
            event.preventDefault();event.stopImmediatePropagation();
            void(async()=>{for(const file of files)await insertImage(file);})().catch(fail);
        },{capture:true,signal:lifecycle.signal});
        mount.addEventListener('dragover',event=>{if(event.dataTransfer?.types.includes('Files'))event.preventDefault();},{signal:lifecycle.signal});
        root.addEventListener('keydown',event=>{
            const command=event.ctrlKey || event.metaKey,action=command && {s:event.shiftKey?'saveAs':'save',o:'open',n:'new',f:'search',h:'search',p:'print'}[event.key.toLowerCase()];
            if(action){event.preventDefault();event.stopPropagation();act(action);}
            if(event.key==='Escape'){if(rightPanel)showPanel('right',rightPanel);else if(leftPanel)showPanel('left',leftPanel);root.classList.remove('writer-focus');}
        },{capture:true,signal:lifecycle.signal});
        window.addEventListener('beforeunload',event=>{if(queue?.dirty){event.preventDefault();event.returnValue='';}},{signal:lifecycle.signal});
        function cleanup() {
            if(disposed)return;disposed=true;generation++;
            clearTimeout(updateTimer);clearTimeout(draftTimer);clearTimeout(draftDeadline);
            lifecycle.abort();documentLife.abort();panels.dispose();queue?.dispose();editor?.destroy();
            ctx.setWindowBeforeClose?.(windowId,null);ctx.clearWindowMenus?.(windowId);instances.delete(windowId);
        }
        setMenus();void load(path,ctx.path?null:'blank');
    }
    function dispose(windowId){instances.get(windowId)?.dispose();}
    window.WriterApp={render,dispose,instances};
})();
