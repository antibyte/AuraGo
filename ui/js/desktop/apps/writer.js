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
        const commands = window.WriterCommands.create({tr, t: (key, params) => ctx.t?.(key, params) || key, readonly: !!ctx.readonly,
            editor: () => editor, lib: () => lib, loading: () => loading, act: id => act(id), notice, refresh: () => refresh(),
            format: value => Number(value).toLocaleString(locale), active: id => activeAction(id),
            clipboard: kind => kind === 'write' ? !!navigator.clipboard?.writeText : !!navigator.clipboard?.readText});
        const {can, run} = commands, colors = window.WriterCommands.colors;
        const icon = window.WriterToolbar.icons(ctx, esc);
        const button = id => '<button type="button" data-action="' + id + '" title="' + esc(commands.tooltip(id)) + '" aria-label="' + esc(commands.label(id)) + '">' + icon(commands.get(id).icon, commands.get(id).fallback) + '</button>';
        const menuId = 'writer-document-menu-' + String(windowId).replace(/[^\w-]/g,'-');
        host.innerHTML = '<div class="writer-app" data-writer="' + esc(windowId) + '">' +
            '<header class="writer-documentbar"><div class="writer-document"><button class="writer-document-name" data-action="document" aria-expanded="false" aria-controls="' + menuId + '"><span data-name></span><span aria-hidden="true">⌄</span></button><span data-save-state role="status" aria-live="polite">' + esc(tr('loading')) + '</span></div>' +
            '<nav class="writer-views" aria-label="' + esc(tr('panels')) + '">' + button('outline') + button('search') + '<span class="writer-divider"></span>' +
            button('format') + button('review') + button('assist') + '</nav></header>' +
            '<div class="writer-document-menu" id="' + menuId + '" data-document-menu hidden><strong data-location></strong><button data-action="rename">' + esc(tr('rename')) + '</button><button data-action="saveAs">' + esc(tr('save_as')) + '</button><button data-action="save">' + esc(tr('save')) + '</button></div>' +
            '<div class="writer-toolbar-wrap" data-toolbar></div>' +
            '<div class="writer-notice" data-notice role="alert" hidden><span data-notice-text></span><button data-action="retry" hidden>' + esc(tr('retry')) + '</button><button data-action="saveAs" hidden>' + esc(tr('save_as')) + '</button><button data-action="reload" hidden>' + esc(tr('reopen')) + '</button><button data-action="dismiss" aria-label="' + esc(tr('close')) + '"><span aria-hidden="true">×</span></button></div>' +
            '<div class="writer-workspace"><aside class="writer-left" data-left hidden></aside><main class="writer-canvas"><div class="writer-ruler" aria-hidden="true"><span>0</span><span>2</span><span>4</span><span>6</span><span>8</span><span>10</span><span>12</span><span>14</span><span>16</span></div><div class="writer-viewport docx-editor__scroll-container" data-scroll><div class="writer-editor docx-editor" data-editor></div></div><div class="writer-loading" data-loading><div class="writer-load-message"><p data-load-status role="status">' + esc(tr('loading')) + '</p><div class="writer-load-actions" data-load-actions hidden><button data-action="new"' + (ctx.readonly?' disabled':'') + '>' + esc(tr('new')) + '</button><button data-action="open">' + esc(tr('open')) + '</button></div></div></div></main><aside class="writer-right" data-right hidden></aside></div>' +
            '<footer class="writer-statusbar"><button data-action="page" data-page></button><span data-count></span><span class="writer-status-spacer"></span><span data-language title="' + esc(tr('language_unknown')) + '"></span>' +
            button('focus') + '<button data-action="zoomOut" aria-label="' + esc(tr('zoom_out')) + '">−</button><button data-action="zoomMenu" data-zoom aria-haspopup="menu" title="' + esc(tr('zoom')) + '">100%</button><button data-action="zoomIn" aria-label="' + esc(tr('zoom_in')) + '">+</button></footer>' +
            '<button class="writer-focus-exit" data-action="focus" hidden title="' + esc(tr('focus_exit')) + '" aria-label="' + esc(tr('focus_exit')) + '">' + icon('restore','⛶') + '</button>' +
            '<input data-image type="file" accept="image/png,image/jpeg,image/webp" hidden></div>';
        const root = host.firstElementChild, find = selector => root.querySelector(selector), mount = find('[data-editor]');
        // The page box moves with the canvas and the zoom, so the ruler follows the engine's layout.
        const rulerResize=new ResizeObserver(()=>{rulerSignature='';if(editor&&!loading)syncRuler(editor.snapshot().pageSetup);});
        rulerResize.observe(find('.writer-canvas'));
        rulerResize.observe(mount);
        let editor, lib, queue, path = ctx.path || newPath(), etag = null, generation = 0, disposed = false, fileOperation = false;
        let loadFailed = false, loading = true, pin = null, updateTimer, draftTimer, draftDeadline, lastDraft = null;
        let statsRevision=-1, rulerSignature='', documentLanguage='', documentFonts, documentStats='';
        let leftPanel = '', rightPanel = '', author = localStorage.getItem('aurago.writer.author') || 'AuraGo';
        let noticeKind = '', retrySaveAs = null;
        const lifecycle = new AbortController();
        let documentLife = new AbortController();
        const locale = document.documentElement.lang || 'en';
        const state = {ctx,esc,tr,root,find,get editor(){return editor;},get lib(){return lib;},
            get revision(){return queue?.revision || 0;}, get readonly(){return !!ctx.readonly;},
            commands,run,can,blocked:commands.blocked,notice,prompt:(key,value)=>ctx.promptDialog(tr(key),value || ''),
            windowId,signal:()=>documentLife.signal,countWords:text=>countWords(text),menuSignature,fileMenuItems,
            act,refresh:()=>refresh(),pinSelection,
            markCommand:commands.valueCommand,colorHex:colors.hex,highlightHex:colors.highlightHex,
            get author(){return author;},set author(value){author=value;localStorage.setItem('aurago.writer.author',value);editor.setAuthor(value);}
        };
        const panels = window.WriterPanels.create(state);
        const toolbar = window.WriterToolbar.create(state);
        const dialogs = window.WriterDialogs.create(state);
        const menus = window.WriterMenus.create(state);
        const instance = {dispose:cleanup,get editor(){return editor;},get session(){return queue;},get path(){return path;},act,reloadIfChanged};
        instances.set(windowId,instance);
        window.AuraDesktopEmail?.attach(windowId, '.writer-documentbar', () => ({
            key: documentLife.signal, signal: documentLife.signal, subject: basename(path),
            disabled: !editor || loading || loadFailed || fileOperation,
            formats: ['docx','html','md','txt'],
            createFiles: async (format, signal) => format === 'docx'
                ? [new File([await editor.save()], basename(path), {type:MIME})]
                : exportSimple(format, true, signal)
        }));
        ctx.wireContextMenuBoundary?.(host);
        ctx.setWindowBeforeClose?.(windowId,guard);
        ctx.registerWindowCleanup?.(windowId,cleanup);
        function pinSelection() {
            if (!editor) return;
            if (pin) editor.releaseSelection(pin);
            pin = editor.retainSelection();
        }
        function notice(message, error, kind) {
            if (disposed) return;
            noticeKind = message ? (kind || 'info') : '';
            if (!message) retrySaveAs = null;
            const banner = find('[data-notice]');
            find('[data-notice-text]').textContent = message || '';
            banner.hidden = !message;
            banner.dataset.error = String(!!error);
            banner.dataset.kind = noticeKind;
            const visible = action => action === 'retry' ? ['save','saveAs','load'].includes(noticeKind)
                : action === 'saveAs' ? ['conflict','save','saveAs'].includes(noticeKind)
                : action === 'reload' ? noticeKind === 'conflict' : false;
            for (const action of ['retry','saveAs','reload']) {
                const button = banner.querySelector('[data-action="'+action+'"]');
                if (button) button.hidden = !visible(action);
            }
        }
        function fail(error, kind) {
            if (error?.name === 'AbortError' || disposed) return;
            const resolved = kind || (error?.status === 412 ? 'conflict' : 'save');
            if (resolved !== 'saveAs') retrySaveAs = null;
            notice(resolved === 'conflict' ? tr('conflict') : (error?.message || String(error)), true, resolved);
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
                    else if(!value.dirty && !value.saving && (noticeKind==='conflict' || noticeKind==='save'))notice('');
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
            find('[data-loading]').hidden=false;find('[data-load-status]').textContent=tr('loading');find('[data-load-actions]').hidden=true;
            find('[data-name]').textContent=basename(path);find('[data-location]').textContent=path;notice('');
            find('[data-save-state]').textContent=tr('loading');delete find('[data-save-state]').dataset.state;
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
                toolbar.fill();menus.sync(true);refresh();
                const draft=await WriterSession.draft('get',draftKey()).catch(()=>null);
                if(disposed || token!==generation)return;
                const restore=draft?.bytes && !ctx.readonly && await ctx.confirmDialog(tr('recover'),tr('recover_detail'));
                if(disposed || token!==generation)return;
                if(restore) {
                    loading=true;editor.load(draft.bytes);loading=false;etag=draft.etag || null;queue.changed();notice(tr('recovered'));refresh();
                }
                if(editor.snapshot().isOpening)await editor.save();
                if(disposed || token!==generation || editor!==mounted)return;
                const start={paragraphId:editor.surface.session.paragraphIds()[0],offset:0};
                if(start.paragraphId)editor.exec({type:'setSelection',range:{anchor:start,head:start}});
                const owner=host.closest('.vd-window');
                if(!ctx.readonly && (!owner || owner.classList.contains('active')))editor.focus();
                find('[data-scroll]').scrollTop=0;
                const warnings=[];
                if(resources.substitutions.length)warnings.push(tr('font_substitution')+' '+resources.substitutions.join(', '));
                if(resources.unsupported.length)warnings.push(tr('preserved_parts')+' '+resources.unsupported.join(', '));
                if(warnings.length)notice(warnings.join(' · '));
            } catch(error) {
                if(disposed || token!==generation || error.name==='AbortError')return;
                loading=false;loadFailed=true;editor?.destroy();editor=null;queue?.dispose();queue=null;mount.replaceChildren();
                find('[data-loading]').hidden=false;find('[data-load-status]').textContent=tr('load_failed');find('[data-load-actions]').hidden=false;
                find('[data-save-state]').textContent=tr('load_failed');find('[data-save-state]').dataset.state='error';
                if(error.status===404)ctx.updateWindowContext?.(windowId,{path:''});
                fail(error,'load');menus.sync(true);
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
        function refreshSoon(){clearTimeout(updateTimer);updateTimer=setTimeout(refresh,75);}
        const countWords = text => text.trim()?(Intl.Segmenter?[...new Intl.Segmenter(locale,{granularity:'word'}).segment(text)].filter(x=>x.isWordLike).length:text.trim().split(/\s+/).length):0;
        // The ruler mirrors the engine's own page box, so the zero mark sits on the text edge.
        function syncRuler(setup) {
            const ruler=find('.writer-ruler'),canvas=find('.writer-canvas'),page=root.querySelector('.docx-page');
            if(!ruler||!canvas||!setup)return;
            const canvasRect=canvas.getBoundingClientRect(),pageRect=page?page.getBoundingClientRect():null;
            const content=page?page.querySelector(':scope > .docx-page-content'):null;
            const contentRect=content?content.getBoundingClientRect():null;
            const zoom=editor.getZoom(),scale=zoom/15;
            const width=pageRect?Math.round(pageRect.width*100)/100:setup.pageWidthTwips*scale;
            const left=pageRect?Math.round((pageRect.left-canvasRect.left)*100)/100:0;
            const padLeft=contentRect?Math.round((contentRect.left-pageRect.left)*100)/100:setup.marginsTwips.left*scale;
            const padRight=contentRect?Math.round((pageRect.right-contentRect.right)*100)/100:setup.marginsTwips.right*scale;
            if(ruler.dataset.width!==String(width)){ruler.style.width=width+'px';ruler.dataset.width=String(width);}
            if(ruler.dataset.left!==String(left)){ruler.style.marginLeft=left+'px';ruler.style.marginRight='0';ruler.dataset.left=String(left);}
            if(ruler.dataset.pad!==padLeft+'/'+padRight){ruler.style.paddingLeft=padLeft+'px';ruler.style.paddingRight=padRight+'px';ruler.dataset.pad=padLeft+'/'+padRight;}
            const contentMm=contentRect?contentRect.width*25.4/(96*zoom):(setup.pageWidthTwips-setup.marginsTwips.left-setup.marginsTwips.right)*25.4/1440;
            const ticks=Math.max(1,Math.floor(contentMm/20)+1);
            if(ruler.dataset.ticks!==String(ticks)){
                ruler.dataset.ticks=String(ticks);
                ruler.innerHTML=Array.from({length:ticks},(_,i)=>'<span>'+i*2+'</span>').join('');
            }
        }
        function refresh() {
            if(!editor || disposed || loading)return;
            const snapshot=editor.snapshot();
            toolbar.refresh();menus.sync();
            find('[data-page]').textContent=tr('page_count',{current:editor.getCurrentPage('viewport'),total:snapshot.page.total});
            if(statsRevision!==queue.revision) {
                statsRevision=queue.revision;
                const text=editor.surface.session.bodyText();
                documentStats=tr('word_count',{count:countWords(text)})+' · '+tr('char_count',{count:[...text].length});
            }
            let stats=documentStats;
            const selected=snapshot.selectionCollapsed?'':editor.query({type:'selectedText'});
            if(selected?.trim())stats+=' · '+tr('selection_count',{count:countWords(selected)});
            find('[data-count]').textContent=stats;
            const setup=snapshot.pageSetup,signature=JSON.stringify([setup,editor.getZoom(),leftPanel,rightPanel]);
            if(signature!==rulerSignature){rulerSignature=signature;syncRuler(setup);}
            const language=find('[data-language]');
            language.textContent=documentLanguage?documentLanguage.toUpperCase():'—';
            language.title=documentLanguage||tr('language_unknown');
            find('[data-zoom]').textContent=Math.round(editor.getZoom()*100)+'%';
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
        // An open request for the file shown here (Files, the agent's open_in_app)
        // reloads it when it changed on disk since it was loaded or last saved.
        // Unsaved edits are never replaced; the conflict notice explains instead.
        async function reloadIfChanged() {
            if(disposed || fileOperation)return;
            if(loadFailed)return load(path);
            if(loading || !editor || !etag || queue?.pending)return;
            const token=generation;
            try {
                const response=await responseOK(await fetch(nativeURL(path),{signal:documentLife.signal,cache:'no-store'}));
                const current=response.headers.get('ETag');response.body?.cancel().catch(()=>{});
                if(disposed || token!==generation || loading || queue?.pending || current===etag)return;
                if(queue?.dirty){notice(tr('conflict'),true,'conflict');return;}
                await load(path);
            } catch(error){fail(error);}
        }
        async function saveAs() {
            if(!editor || ctx.readonly || fileOperation)return;
            const sourceEditor=editor;
            const choice=await ctx.saveFileDialog({title:tr('save_as'),initialPath:path.split('/').slice(0,-1).join('/'),defaultName:basename(path),defaultExtension:'.docx',filters:[{label:'DOCX',extensions:['.docx']}]});
            if(!choice?.path || choice.canceled || editor!==sourceEditor || disposed)return;
            if(choice.path===path)return queue.save();
            await commitCopy(choice.path);
        }
        async function commitCopy(target, preset) {
            if(!editor || ctx.readonly || fileOperation || disposed)return;
            const sourceEditor=editor;
            fileOperation=true;
            const activeQueue=queue;activeQueue.suspend();
            const probed=!!preset?.probed;
            let expected=probed?preset.expected:null, decided=probed;
            try {
                await activeQueue.pending?.catch(()=>{});
                if(!decided) {
                    const probe=await fetch(nativeURL(target),{signal:documentLife.signal,cache:'no-store'});
                    if(probe.status===404) {
                        expected=null;decided=true;
                        await probe.body?.cancel().catch(()=>{});
                    } else if(probe.ok) {
                        expected=probe.headers.get('ETag');
                        await probe.body?.cancel().catch(()=>{});
                        if(!expected){const error=new Error(tr('conflict'));error.status=412;throw error;}
                        const name=basename(target);
                        const replace=await ctx.confirmDialog(ctx.t?.('desktop.file_dialog_overwrite_title') || tr('save_as'), ctx.t?.('desktop.file_dialog_overwrite_message',{name}) || name);
                        if(!replace || disposed || editor!==sourceEditor){retrySaveAs=null;return;}
                        decided=true;
                    } else await responseOK(probe);
                }
                if(disposed || editor!==sourceEditor)return;
                const captured=activeQueue.revision,bytes=await editor.save();
                const newETag=await write(bytes,target,expected);
                if(disposed)return;
                const changed=activeQueue.revision!==captured;
                queue.dispose();path=target;etag=newETag;panels.reset();makeQueue();if(changed)queue.changed();
                find('[data-name]').textContent=basename(path);find('[data-location]').textContent=path;
                find('[data-save-state]').textContent=tr(changed?'unsaved':'saved');
                ctx.updateWindowContext?.(windowId,{path});
                retrySaveAs=null;notice(tr('saved'));ctx.loadBootstrap?.();
            } catch(error) {
                if(error?.name==='AbortError' || disposed)return;
                retrySaveAs={path:target,expected,probed:decided};
                notice(error.status===412?tr('conflict'):(error.message || String(error)),true,'saveAs');
            } finally {fileOperation=false;if(queue===activeQueue)activeQueue.resume();}
        }
        async function rename() {
            if(!editor || ctx.readonly || fileOperation)return;
            const sourceEditor=editor;
            const name=await ctx.promptDialog(tr('rename'),basename(path));
            if(!name || name===basename(path) || editor!==sourceEditor || disposed)return;
            if(/[\\/]/.test(name) || !/\.docx$/i.test(name)){notice(tr('name_docx'),true,'info');return;}
            if(fileOperation)return;fileOperation=true;
            const activeQueue=queue;activeQueue.suspend();
            let stage='save';
            try {
                if(!etag && !queue.dirty)queue.changed();
                await queue.save();
                stage='move';
                const oldPath=path,target=path.split('/').slice(0,-1).concat(name).join('/');
                const current=await responseOK(await fetch(nativeURL(oldPath),{signal:documentLife.signal,cache:'no-store'}));
                const currentETag=current.headers.get('ETag');
                await current.body?.cancel().catch(()=>{});
                if(currentETag!==etag){notice(tr('conflict'),true,'conflict');return;}
                const moved=await fetch('/api/desktop/file',{method:'PATCH',headers:{'Content-Type':'application/json','If-None-Match':'*'},body:JSON.stringify({old_path:oldPath,new_path:target}),signal:documentLife.signal});
                if(moved.status===412){
                    await moved.body?.cancel().catch(()=>{});
                    notice(ctx.t?.('desktop.fm.paste_exists',{name}) || name,true,'info');
                    return;
                }
                await responseOK(moved);
                await moved.body?.cancel().catch(()=>{});
                stage='adopt';
                path=target;etag=null;
                find('[data-name]').textContent=name;find('[data-location]').textContent=path;
                ctx.updateWindowContext?.(windowId,{path});
                const adopted=await responseOK(await fetch(nativeURL(target),{signal:documentLife.signal,cache:'no-store'}));
                etag=adopted.headers.get('ETag');
                await adopted.body?.cancel().catch(()=>{});
                ctx.loadBootstrap?.();
            } catch(error) {
                if(error?.name==='AbortError' || disposed || stage==='save')return;
                notice(error?.message || String(error),true,'info');
            } finally {fileOperation=false;if(queue===activeQueue)activeQueue.resume();}
        }
        function showPanel(side,value) {
            if(side==='left')leftPanel=leftPanel===value?'':value;else rightPanel=rightPanel===value?'':value;
            panels.show(side,side==='left'?leftPanel:rightPanel);
            root.classList.toggle('has-left',!!leftPanel);root.classList.toggle('has-right',!!rightPanel);
            for(const el of root.querySelectorAll('.writer-views button'))el.setAttribute('aria-pressed',String([leftPanel,rightPanel].includes(el.dataset.action)));
            refresh();
        }
        function closeDocumentMenu() {
            const menu=find('[data-document-menu]');
            if(menu)menu.hidden=true;
            find('.writer-document-name')?.setAttribute('aria-expanded','false');
        }
        function toggleDocumentMenu() {
            const menu=find('[data-document-menu]');
            if(!menu)return;
            menu.hidden=!menu.hidden;
            find('.writer-document-name')?.setAttribute('aria-expanded',String(!menu.hidden));
        }
        function setFocus(on) {
            root.classList.toggle('writer-focus',on);
            const exit=find('.writer-focus-exit');
            if(exit)exit.hidden=!on;
            menus.sync();
        }
        // Whether an app action is switched on; menus show it as a check mark.
        function activeAction(id) {
            if(id==='outline' || id==='search')return leftPanel===id;
            if(['format','review','assist','insert'].includes(id))return rightPanel===id;
            if(id==='focus')return root.classList.contains('writer-focus');
            if(id==='tracking')return commands.tracking();
            return false;
        }
        // The engine rejects a zoom outside its range instead of clamping it.
        function zoomTo(value) {
            if(!editor)return;
            editor.setZoom(Math.max(.25,Math.min(3,Math.round(value*100)/100)));
            refresh();
        }
        // The engine fits the page width only; a whole page is the smaller of the two ratios.
        function wholePageZoom() {
            const sheet=root.querySelector('.docx-page'),view=find('[data-scroll]');
            if(!sheet || !view)return editor.getZoom();
            const box=sheet.getBoundingClientRect();
            if(!box.width || !box.height)return editor.getZoom();
            return editor.getZoom()*Math.min((view.clientWidth-32)/box.width,(view.clientHeight-32)/box.height);
        }
        // A menu click carries no clipboard event, so the clipboard is read here. Tracking takes
        // text only, like the paste handler below.
        async function pasteFromClipboard(plain) {
            if(!editor)return;
            if(!navigator.clipboard?.readText)throw new Error(ctx.t('desktop.clipboard_read_unavailable'));
            let text='',html='';
            if(!plain && !commands.tracking() && navigator.clipboard.read) {
                try {
                    for(const entry of await navigator.clipboard.read()) {
                        if(entry.types.includes('text/html'))html=await (await entry.getType('text/html')).text();
                        if(entry.types.includes('text/plain'))text=await (await entry.getType('text/plain')).text();
                    }
                } catch(_) { html=''; }
            }
            if(!html && !text)text=await navigator.clipboard.readText();
            if(!html && !text)return;
            run(plain?{type:'pasteWithoutFormatting',text}:html?{type:'paste',text,html:sanitize(html)}:{type:'paste',text});
        }
        async function insertImage(file, options = {}) {
            if(!editor || ctx.readonly || !/^image\/(png|jpeg|webp)$/.test(file.type))throw new Error(tr('image_format'));
            const refusal=commands.blocked('insertImage');if(refusal)throw new Error(refusal);
            if(file.size>20*1024*1024)throw new Error(tr('image_large'));
            const active=editor,revision=options.expectedRevision ?? editor.getDocumentHandle().revision;
            if(options.expectedEditor && (editor!==options.expectedEditor || revision!==editor.getDocumentHandle().revision))throw new Error(ctx.t('desktop.qr_stale'));
            const bitmap=await createImageBitmap(file);
            const command={type:'insertImage',data:new Uint8Array(await file.arrayBuffer()),mime:file.type,widthPoints:options.widthPoints ?? bitmap.width*.75,heightPoints:options.heightPoints ?? bitmap.height*.75,expectedPackageRevision:revision,description:file.name || ''};
            bitmap.close();if(disposed || editor!==active)return;
            if(options.selection) {
                if(ctx.readonly || commands.blocked('insertImage') || editor.getDocumentHandle().revision!==revision)throw new Error(ctx.t('desktop.qr_stale'));
                // Native image insertion needs a caret; keep the selected text and insert at its captured start.
                const restored=editor.exec({type:'setSelection',range:{anchor:options.selection.anchor,head:options.selection.anchor}});
                if(!restored.ok)throw new Error(ctx.t('desktop.qr_stale'));
            }
            const result=await editor.executeImageCommand(command);if(!result.ok)throw new Error(result.reason || tr('command_unavailable'));
        }
        let qrOpening=false;
        async function insertQR() {
            if(qrOpening || !editor || ctx.readonly)return;
            const refusal=commands.blocked('insertImage');if(refusal)throw new Error(refusal);
            const source=editor,revision=source.getDocumentHandle().revision,selection=structuredClone(source.surface.state().selection);
            const retained=source.retainSelection(),signal=documentLife.signal;
            const current=()=>!disposed && !signal.aborted && editor===source && !ctx.readonly && source.getDocumentHandle().revision===revision && !commands.blocked('insertImage');
            qrOpening=true;
            try {
                await window.AuraDesktopModules.loadAppAssets('qr');
                if(!current())return;
                const dialog=window.AuraQRDialog.open({host:root,windowId,t:ctx.t,signal,isCurrent:current,
                    insertion:{unit:'mm',width:40},onInsert:result=>insertImage(result.file,{expectedEditor:source,expectedRevision:revision,selection,
                        widthPoints:result.physicalWidthMM*72/25.4,heightPoints:result.physicalHeightMM*72/25.4})});
                await dialog.closed;
            } finally { if(retained)source.releaseSelection(retained);qrOpening=false; }
        }
        async function act(action) {
            try {
                if(['outline','search'].includes(action))return showPanel('left',action);
                if(['format','review','assist','insert'].includes(action))return showPanel('right',action);
                switch(action) {
                    case 'document':toggleDocumentMenu();break;
                    case 'dismiss':notice('');break;
                    case 'retry':
                        if(noticeKind==='saveAs' && retrySaveAs)await commitCopy(retrySaveAs.path,retrySaveAs);
                        else if(loadFailed || noticeKind==='load')await load(path);
                        else await queue?.save();
                        break;
                    case 'reload':
                        if(queue?.dirty && !await ctx.confirmDialog(tr('unsaved'),tr('discard_confirm')))return;
                        await load(path);
                        break;
                    case 'save':closeDocumentMenu();if(queue && !etag && !queue.dirty)queue.changed();await queue?.save();break;
                    case 'saveAs':closeDocumentMenu();await saveAs();break;
                    case 'rename':closeDocumentMenu();await rename();break;
                    case 'open':{
                        const choice=await ctx.openFileDialog({title:tr('open'),initialPath:'Documents',filters:[{label:tr('documents'),extensions:['.docx','.html','.htm','.md','.txt']}]});
                        if(choice?.path && !choice.canceled && await guard())await load(choice.path);break;
                    }
                    case 'new':if(!ctx.readonly && await guard()){await load(newPath(),'blank');showPanel('right','insert');}break;
                    case 'blank':case 'letter':case 'report':case 'meeting':if(!ctx.readonly && await guard())await load(newPath(),action);break;
                    case 'focus':setFocus(!root.classList.contains('writer-focus'));break;
                    case 'fit':editor?.setZoomMode({type:'fit',fit:'pageWidth',minZoom:.25,maxZoom:1});refresh();break;
                    case 'zoomIn':case 'zoomOut':if(editor)zoomTo(editor.getZoom()+(action==='zoomIn'?.1:-.1));break;
                    case 'zoomPage':if(editor)zoomTo(wholePageZoom());break;
                    case 'zoomMenu':{
                        const box=find('[data-zoom]').getBoundingClientRect(),ids=['fit','zoomPage','zoom50','zoom75','zoom100','zoom125','zoom150','zoom200'];
                        if(ctx.showContextMenu)ctx.showContextMenu(box.left,box.top,ids.map(id=>Object.assign(commands.menuItem(id),{icon:'zoom-in'})));
                        else await act('fit');
                        break;
                    }
                    case 'tracking':run({type:'setEditingMode',mode:commands.tracking()?'editing':'suggesting'});break;
                    case 'paste':case 'pastePlain':
                        try{await pasteFromClipboard(action==='pastePlain');}
                        catch(_){notice(ctx.t('desktop.clipboard_read_unavailable'),true,'info');}
                        break;
                    case 'page':case 'goToPage':await dialogs.goToPage();break;
                    case 'table':await dialogs.table();break;
                    case 'link':await dialogs.link();break;
                    case 'wordCount':await dialogs.wordCount();break;
                    case 'shortcuts':await dialogs.shortcuts();break;
                    case 'image':find('[data-image]').click();break;
                    case 'qr':await insertQR();break;
                    case 'print':if(editor)await panels.print(await editor.save(),lib,documentFonts);break;
                    case 'docx':if(editor)download(await editor.save(),MIME,basename(path));break;
                    case 'html':case 'md':case 'txt':await exportSimple(action);break;
                    default:{const preset=/^zoom(\d+)$/.exec(action);if(preset && editor)zoomTo(Number(preset[1])/100);}
                }
            } catch(error){if(action==='print')notice(ctx.t('desktop.print_failed'),true);else fail(error);}
        }
        function download(bytes,type,name) {
            const url=URL.createObjectURL(new Blob([bytes],{type})),link=document.createElement('a');
            link.href=url;link.download=name;link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
        }
        async function exportSimple(format, asFile=false, signal=documentLife.signal) {
            if(!editor || !await ctx.confirmDialog(tr('export'),tr('export_loss')))return false;
            const bytes=await editor.save();
            const response=await responseOK(await fetch('/api/desktop/office/export?format='+format,{method:'POST',headers:{'Content-Type':MIME},body:bytes,signal}));
            const data=await response.arrayBuffer(),type=response.headers.get('Content-Type'),name=basename(path).replace(/\.docx$/i,'.'+format);
            if(asFile)return [new File([data],name,{type})];
            download(data,type,name);
        }
        async function gogOffice(mode) {
            if(fileOperation||disposed)return;
            try {
                if(mode==='import'&&!await guard())return;
                if(mode==='export'){if(!queue)return;if(!etag&&!queue.dirty)queue.changed();await queue.save();if(queue.dirty)throw Error(tr('unsaved'));}
                const result=await window.AuraGogDesktop.office(ctx,'docs',mode,path,lifecycle.signal);
                if(result?.file?.path&&!disposed)await load(result.file.path);
            }catch(error){fail(error);}
        }
        function fileMenuItems() {
            return [{id:'gog-import',label:ctx.t('desktop.gog.import'),disabled:ctx.readonly,action:()=>gogOffice('import')},
                {id:'gog-export',label:ctx.t('desktop.gog.export'),disabled:ctx.readonly||!editor,action:()=>gogOffice('export')}];
        }
        // Everything a menu shows as enabled or checked. The bar is rebuilt when this changes.
        function menuSignature() {
            const s=editor && !loading?editor.snapshot():null;
            return JSON.stringify([!!s,!!ctx.readonly,s?.editingMode,s?.reviewDisplayMode,!!s?.showParagraphMarks,!!s?.canUndo,!!s?.canRedo,!!s?.selectionCollapsed,
                !!s?.table,!!s?.image,s?.scope?.kind,s?.formatting?.lineSpacing?.value,leftPanel,rightPanel,root.classList.contains('writer-focus'),
                ...(s?['mergeCells','splitCell','pasteFormat','previousChange','nextChange','backDocument'].map(id=>commands.state(id).enabled):[])]);
        }
        root.addEventListener('pointerdown',event=>{if(event.target.closest('.writer-toolbar-wrap,.writer-right,.writer-views'))pinSelection();},{signal:lifecycle.signal});
        root.addEventListener('click',event=>{
            const menu=find('[data-document-menu]');
            if(menu && !menu.hidden && !event.target.closest('.writer-document-menu,.writer-document-name'))closeDocumentMenu();
            const control=event.target.closest('[data-action]');if(!control || control.disabled)return;
            void act(control.dataset.action);
        },{signal:lifecycle.signal});
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
        // One wheel notch (deltaY 100) is about ten percent; a trackpad pinch sends many small deltas,
        // so the factor follows the delta and the document is zoomed once per frame.
        let wheelZoom=0,wheelFrame=0;
        find('[data-scroll]').addEventListener('wheel',event=>{
            if(!(event.ctrlKey || event.metaKey) || !editor)return;
            event.preventDefault();
            if(!event.deltaY)return;
            wheelZoom=Math.max(.25,Math.min(3,(wheelZoom || editor.getZoom())*Math.exp(-Math.max(-200,Math.min(200,event.deltaY))/1000)));
            if(wheelFrame)return;
            wheelFrame=requestAnimationFrame(()=>{wheelFrame=0;const target=wheelZoom;wheelZoom=0;zoomTo(target);});
        },{passive:false,signal:lifecycle.signal});
        root.addEventListener('keydown',event=>{
            const action=commands.actionForKey(event);
            if(action){event.preventDefault();event.stopPropagation();commands.invoke(action);}
            if(event.key==='Escape'){
                // The overflow popover and the menu are the topmost layers; focus mode hides every panel and leaves on Escape.
                // The engine keeps the event: Escape also leaves a note scope or the format painter.
                if(toolbar.escape())return;
                if(!find('[data-document-menu]').hidden){closeDocumentMenu();return;}
                if(root.classList.contains('writer-focus')){setFocus(false);return;}
                if(rightPanel){showPanel('right',rightPanel);return;}
                if(leftPanel){showPanel('left',leftPanel);return;}
            }
        },{capture:true,signal:lifecycle.signal});
        window.addEventListener('beforeunload',event=>{if(queue?.dirty){event.preventDefault();event.returnValue='';}},{signal:lifecycle.signal});
        function cleanup() {
            if(disposed)return;disposed=true;generation++;
            clearTimeout(updateTimer);clearTimeout(draftTimer);clearTimeout(draftDeadline);cancelAnimationFrame(wheelFrame);
            lifecycle.abort();documentLife.abort();rulerResize.disconnect();toolbar.dispose();panels.dispose();queue?.dispose();editor?.destroy();
            ctx.setWindowBeforeClose?.(windowId,null);ctx.clearWindowMenus?.(windowId);instances.delete(windowId);
        }
        menus.sync(true);void load(path,ctx.path?null:'blank');
    }
    function dispose(windowId){instances.get(windowId)?.dispose();}
    window.WriterApp={render,dispose,instances};
})();
