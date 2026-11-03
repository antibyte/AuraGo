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
        const actionIcons = {new:'file-plus',open:'folder-open',saveAs:'copy',docx:'file-text',html:'file-code',md:'notes',txt:'file-text',print:'printer',
            outline:'list',format:'sliders',review:'chat',assist:'star',insert:'plus',focus:'maximize',fit:'monitor'};
        // The shared icon atlases carry no keys for text alignment, underline, indent or
        // colour controls, so Autor draws those few glyphs itself, in the toolbar's line style.
        const glyphs = {
            'align-left':'<path d="M4 6h16M4 12h9M4 18h16"/>',
            'align-center':'<path d="M4 6h16M7.5 12h9M4 18h16"/>',
            'align-right':'<path d="M4 6h16M11 12h9M4 18h16"/>',
            'align-justify':'<path d="M4 6h16M4 12h16M4 18h16"/>',
            'format-underline':'<path d="M7 4v8a5 5 0 0 0 10 0V4"/><path d="M6 20h12"/>',
            'indent':'<path d="M4 5h16M4 12h8M4 19h16"/><path d="M12 8l4 4-4 4"/>',
            'outdent':'<path d="M4 5h16M4 12h8M4 19h16"/><path d="M16 8l-4 4 4 4"/>',
            'text-color':'<path d="M12 4L5.5 15M12 4l6.5 11"/><path d="M8.6 10.6h6.8"/><path d="M6 19.5h12" stroke-width="3"/>',
            'highlight':'<path d="M14 4l6 6-9 9H5v-6z"/><path d="M11 7l6 6"/><path d="M6 20.5h12" stroke-width="3"/>'
        };
        const icon = (key, fallback) => {
            const shape=glyphs[key];
            if(shape)return '<svg class="writer-glyph" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">'+shape+'</svg>';
            return ctx.iconMarkup?.(key, fallback, 'writer-icon', 18, 'action') || esc(fallback);
        };
        const actionIcon = (action, fallback) => icon(actionIcons[action] || action, fallback);
        // Action ids are not icon names; every slot names the shared key it draws.
        const slotIcons = {
            'history.undo':'undo','history.redo':'redo',
            'text.bold':'format-bold','text.italic':'format-italic','text.underline':'format-underline','text.strike':'format-strike',
            'alignment.left':'align-left','alignment.center':'align-center','alignment.right':'align-right','alignment.justify':'align-justify',
            'list.bullet':'list','list.numbered':'format-numbered','list.outdent':'outdent','list.indent':'indent',
            'text.color':'text-color','text.highlight':'highlight'
        };
        const toggleSlots = new Set(['text.bold','text.italic','text.underline','text.strike','alignment.left','alignment.center','alignment.right','alignment.justify','list.bullet','list.numbered']);
        const button = (action, label, symbol) => '<button type="button" data-action="' + action + '" title="' + esc(tr(label)) + '" aria-label="' + esc(tr(label)) + '">' + actionIcon(action, symbol) + '</button>';
        const slotButton = (slot, label, symbol) => '<button type="button" data-slot="' + slot + '" title="' + esc(tr(label)) + '" aria-label="' + esc(tr(label)) + '">' + icon(slotIcons[slot] || label, symbol) + '</button>';
        const colorField = (slot, label, value) => '<label class="writer-color" title="' + esc(tr(label)) + '">' + icon(slotIcons[slot], '') +
            '<input type="color" data-slot="' + slot + '" value="' + value + '" aria-label="' + esc(tr(label)) + '"></label>';
        const menuId = 'writer-document-menu-' + String(windowId).replace(/[^\w-]/g,'-');
        host.innerHTML = '<div class="writer-app" data-writer="' + esc(windowId) + '">' +
            '<header class="writer-documentbar"><div class="writer-document"><button class="writer-document-name" data-action="document" aria-expanded="false" aria-controls="' + menuId + '"><span data-name></span><span aria-hidden="true">⌄</span></button><span data-save-state role="status" aria-live="polite">' + esc(tr('loading')) + '</span></div>' +
            '<nav class="writer-views" aria-label="' + esc(tr('panels')) + '">' + button('outline','outline','☷') + button('search','find','⌕') + '<span class="writer-divider"></span>' +
            button('format','format','A') + button('review','review','☷') + button('assist','assist','✦') + '</nav></header>' +
            '<div class="writer-document-menu" id="' + menuId + '" data-document-menu hidden><strong data-location></strong><button data-action="rename">' + esc(tr('rename')) + '</button><button data-action="saveAs">' + esc(tr('save_as')) + '</button><button data-action="save">' + esc(tr('save')) + '</button></div>' +
            '<div class="writer-toolbar" role="toolbar" aria-label="' + esc(tr('format_bar')) + '">' +
            '<div class="writer-toolgroup">' + slotButton('history.undo','undo','↶') + slotButton('history.redo','redo','↷') + '</div>' +
            '<div class="writer-toolgroup writer-fonts"><select data-slot="styles.style" aria-label="' + esc(tr('style')) + '"></select><select data-slot="font.family" aria-label="' + esc(tr('font')) + '"></select><input type="number" data-slot="font.size" required min="1" max="200" step=".5" value="11" aria-label="' + esc(tr('size')) + '"></div>' +
            '<div class="writer-toolgroup">' + slotButton('text.bold','bold','B') + slotButton('text.italic','italic','I') + slotButton('text.underline','underline','U') + slotButton('text.strike','strike','S') + '</div>' +
            '<div class="writer-toolgroup">' + slotButton('alignment.left','align_left','≡') + slotButton('alignment.center','align_center','≡') + slotButton('alignment.right','align_right','≡') + slotButton('alignment.justify','justify','≡') + '</div>' +
            '<div class="writer-toolgroup">' + slotButton('list.bullet','bullets','•') + slotButton('list.numbered','numbering','1.') + slotButton('list.outdent','outdent','⇤') + slotButton('list.indent','indent','⇥') + '</div>' +
            '<div class="writer-toolgroup writer-colors">' + colorField('text.color','color','#172033') + colorField('text.highlight','highlight','#ffff00') + '</div>' +
            button('insert','insert','＋') + '</div>' +
            '<div class="writer-notice" data-notice role="alert" hidden><span data-notice-text></span><button data-action="retry" hidden>' + esc(tr('retry')) + '</button><button data-action="saveAs" hidden>' + esc(tr('save_as')) + '</button><button data-action="reload" hidden>' + esc(tr('reopen')) + '</button><button data-action="dismiss" aria-label="' + esc(tr('close')) + '"><span aria-hidden="true">×</span></button></div>' +
            '<div class="writer-workspace"><aside class="writer-left" data-left hidden></aside><main class="writer-canvas"><div class="writer-ruler" aria-hidden="true"><span>0</span><span>2</span><span>4</span><span>6</span><span>8</span><span>10</span><span>12</span><span>14</span><span>16</span></div><div class="writer-viewport docx-editor__scroll-container" data-scroll><div class="writer-editor docx-editor" data-editor></div></div><div class="writer-loading" data-loading><div class="writer-load-message"><p data-load-status role="status">' + esc(tr('loading')) + '</p><div class="writer-load-actions" data-load-actions hidden><button data-action="new"' + (ctx.readonly?' disabled':'') + '>' + esc(tr('new')) + '</button><button data-action="open">' + esc(tr('open')) + '</button></div></div></div></main><aside class="writer-right" data-right hidden></aside></div>' +
            '<footer class="writer-statusbar"><button data-action="page" data-page></button><span data-count></span><span class="writer-status-spacer"></span><span data-language title="' + esc(tr('language_unknown')) + '"></span>' +
            button('focus','focus_mode','⛶') + '<button data-action="zoomOut" aria-label="' + esc(tr('zoom_out')) + '">−</button><button data-action="fit" data-zoom title="' + esc(tr('fit')) + '">100%</button><button data-action="zoomIn" aria-label="' + esc(tr('zoom_in')) + '">+</button></footer>' +
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
            run,can,notice,prompt:(key,value)=>ctx.promptDialog(tr(key),value || ''),act,refresh:()=>refresh(),pinSelection,
            markCommand:(slot,value)=>slot==='text.color'?lib.commandForSlotValue(slot,colorValue(value).replace('#','')):
                slot==='text.highlight'?lib.commandForSlotValue(slot,highlightName(value)):lib.commandForSlotValue(slot,value),
            colorHex:value=>colorValue(value),highlightHex:value=>highlightHex(value),
            get author(){return author;},set author(value){author=value;localStorage.setItem('aurago.writer.author',value);editor.setAuthor(value);}
        };
        const panels = window.WriterPanels.create(state);
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
                fillToolbar();setMenus();refresh();
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
                fail(error,'load');setMenus();
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
        const styleLabels = {normal:'style_normal',title:'style_title',subtitle:'style_subtitle',quote:'style_quote',caption:'style_caption',nospacing:'style_no_spacing',listparagraph:'style_list_paragraph'};
        function styleLabel(style) {
            const id=String(style.styleId||'').toLowerCase();
            const heading=id.match(/^heading([1-9])$/);
            if(heading)return tr('style_heading',{level:heading[1]});
            return styleLabels[id]?tr(styleLabels[id]):style.name;
        }
        function fillToolbar() {
            const option=(value,label)=>'<option value="'+esc(value)+'">'+esc(label)+'</option>';
            find('[data-slot="styles.style"]').innerHTML=editor.getDocumentStyles().filter(x=>x.type==='paragraph').map(x=>option(x.styleId,styleLabel(x))).join('');
            find('[data-slot="font.family"]').innerHTML=editor.getAvailableFonts().map(x=>option(x,x)).join('');
        }
        function valueCommand(control) {
            const slot=control.dataset.slot;
            // The DOCX engine wants a bare hex for colours and a palette name for highlights.
            if(slot==='text.color')return lib.commandForSlotValue(slot,colorValue(control.value).replace('#',''));
            if(slot==='text.highlight')return lib.commandForSlotValue(slot,highlightName(control.value));
            // The toolbar displays points; the DOCX command consumes integer half-points.
            return lib.commandForSlotValue(slot,slot==='font.size'?control.valueAsNumber*2:control.value);
        }
        function refreshSoon(){clearTimeout(updateTimer);updateTimer=setTimeout(refresh,75);}
        const countWords = text => text.trim()?(Intl.Segmenter?[...new Intl.Segmenter(locale,{granularity:'word'}).segment(text)].filter(x=>x.isWordLike).length:text.trim().split(/\s+/).length):0;
        const colorValue = value => {
            const raw=value&&typeof value==='object'?(value.value??value.resolvedHex??''):value;
            const body=String(raw??'').trim().replace(/^#/,'');
            return /^[0-9a-fA-F]{6}$/.test(body)?'#'+body.toLowerCase():'';
        };
        // The OOXML highlight palette holds named colours only; pick the closest one.
        const highlightColors = {black:'000000',blue:'0000ff',cyan:'00ffff',darkBlue:'000080',darkCyan:'008080',darkGray:'808080',darkGreen:'008000',
            darkMagenta:'800080',darkRed:'800000',darkYellow:'808000',green:'00ff00',lightGray:'c0c0c0',magenta:'ff00ff',red:'ff0000',white:'ffffff',yellow:'ffff00'};
        function highlightName(value) {
            const hex=colorValue(value);
            if(!hex)return '';
            const red=parseInt(hex.slice(1,3),16),green=parseInt(hex.slice(3,5),16),blue=parseInt(hex.slice(5,7),16);
            let nearest='',distance=Infinity;
            for(const [name,value] of Object.entries(highlightColors)) {
                const delta=(red-parseInt(value.slice(0,2),16))**2+(green-parseInt(value.slice(2,4),16))**2+(blue-parseInt(value.slice(4,6),16))**2;
                if(delta<distance){distance=delta;nearest=name;}
            }
            return nearest;
        }
        const highlightHex = value => highlightColors[String(value??'').trim()]?'#'+highlightColors[String(value??'').trim()]:colorValue(value);
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
            const snapshot=editor.snapshot(),fmt=snapshot.formatting || {};
            for(const el of root.querySelectorAll('.writer-toolbar [data-slot]')) {
                const slot=el.dataset.slot;
                if(el.matches('button')) {
                    const command=lib.commandForSlot(slot),allowed=command && can(command);
                    el.disabled=!allowed?.ok;
                    el.title=el.getAttribute('aria-label')+(!allowed?.ok?' · '+tr('command_unavailable')+(allowed?.reason?': '+allowed.reason:''):'');
                    if(toggleSlots.has(slot))el.setAttribute('aria-pressed',String(!!command && editor.isActive(command)));
                    else el.removeAttribute('aria-pressed');
                } else if(slot==='text.color'||slot==='text.highlight') {
                    const hex=slot==='text.color'?colorValue(fmt.color):highlightHex(fmt.highlight);
                    if(hex && el!==document.activeElement)el.value=hex;
                    const allowed=can(valueCommand(el));
                    el.disabled=!allowed.ok;el.title=allowed.ok?'':tr('command_unavailable')+(allowed.reason?' · '+allowed.reason:'');
                } else if(document.activeElement!==el) {
                    el.value=slot==='styles.style'?fmt.styleId || 'Normal':slot==='font.family'?fmt.fontFamily || 'Calibri':fmt.fontSizePt || 11;
                    const allowed=can(valueCommand(el));
                    el.disabled=!allowed.ok;el.title=allowed.ok?'':tr('command_unavailable')+(allowed.reason?' · '+allowed.reason:'');
                }
            }
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
        }
        async function insertImage(file, options = {}) {
            if(!editor || ctx.readonly || !/^image\/(png|jpeg|webp)$/.test(file.type))throw new Error(tr('image_format'));
            if(editor.snapshot().editingMode==='suggesting')throw new Error(tr('tracking_unsupported'));
            if(file.size>20*1024*1024)throw new Error(tr('image_large'));
            const active=editor,revision=options.expectedRevision ?? editor.getDocumentHandle().revision;
            if(options.expectedEditor && (editor!==options.expectedEditor || revision!==editor.getDocumentHandle().revision))throw new Error(ctx.t('desktop.qr_stale'));
            const bitmap=await createImageBitmap(file);
            const command={type:'insertImage',data:new Uint8Array(await file.arrayBuffer()),mime:file.type,widthPoints:options.widthPoints ?? bitmap.width*.75,heightPoints:options.heightPoints ?? bitmap.height*.75,expectedPackageRevision:revision,description:file.name || ''};
            bitmap.close();if(disposed || editor!==active)return;
            if(options.selection) {
                if(ctx.readonly || editor.snapshot().editingMode==='suggesting' || editor.getDocumentHandle().revision!==revision)throw new Error(ctx.t('desktop.qr_stale'));
                // Native image insertion needs a caret; keep the selected text and insert at its captured start.
                const restored=editor.exec({type:'setSelection',range:{anchor:options.selection.anchor,head:options.selection.anchor}});
                if(!restored.ok)throw new Error(ctx.t('desktop.qr_stale'));
            }
            const result=await editor.executeImageCommand(command);if(!result.ok)throw new Error(result.reason || tr('command_unavailable'));
        }
        let qrOpening=false;
        async function insertQR() {
            if(qrOpening || !editor || ctx.readonly)return;
            if(editor.snapshot().editingMode==='suggesting')throw new Error(tr('tracking_unsupported'));
            const source=editor,revision=source.getDocumentHandle().revision,selection=structuredClone(source.surface.state().selection);
            const retained=source.retainSelection(),signal=documentLife.signal;
            const current=()=>!disposed && !signal.aborted && editor===source && !ctx.readonly && source.getDocumentHandle().revision===revision && source.snapshot().editingMode!=='suggesting';
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
                    case 'zoomIn':case 'zoomOut':editor?.setZoom(Math.max(.25,Math.min(3,editor.getZoom()+(action==='zoomIn'?.1:-.1))));refresh();break;
                    case 'page':{const value=await ctx.promptDialog(tr('go_page'),String(editor?.getCurrentPage() || 1));if(value && editor)editor.scrollToPage(Math.max(1,Math.min(editor.getTotalPages(),Number(value)||1)));break;}
                    case 'image':find('[data-image]').click();break;
                    case 'qr':await insertQR();break;
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
        function setMenus() {
            const item=(id,label,shortcut,disabled)=>({id,label:tr(label),icon:actionIcons[id] || id,shortcut,disabled,action:()=>act(id)});
            ctx.setWindowMenus?.(windowId,[
                {id:'file',labelKey:'desktop.menu_file',items:[
                    item('new','new','Ctrl+N',ctx.readonly),item('open','open','Ctrl+O'),
                    item('save','save','Ctrl+S',ctx.readonly || !editor),item('saveAs','save_as',null,ctx.readonly || !editor),
                    {type:'separator'},
                    {id:'gog-import',label:ctx.t('desktop.gog.import'),disabled:ctx.readonly,action:()=>gogOffice('import')},
                    {id:'gog-export',label:ctx.t('desktop.gog.export'),disabled:ctx.readonly||!editor,action:()=>gogOffice('export')},
                    {type:'separator'},item('docx','download_docx',null,!editor),item('html','export_html',null,!editor),item('md','export_md',null,!editor),item('txt','export_txt',null,!editor),
                    {type:'separator'},item('print','print','Ctrl+P',!editor)]},
                {id:'edit',labelKey:'desktop.menu_edit',items:[
                    {id:'undo',label:tr('undo'),icon:'undo',shortcut:'Ctrl+Z',action:()=>run({type:'undo'})},
                    {id:'redo',label:tr('redo'),icon:'redo',shortcut:'Ctrl+Y',action:()=>run({type:'redo'})},
                    item('search','find','Ctrl+F'),item('review','review'),item('format','format'),item('insert','insert')]},
                {id:'insert',label:tr('insert'),items:[item('image','image',null,ctx.readonly || !editor),
                    {id:'qr',label:ctx.t('desktop.qr_insert'),icon:'image',disabled:ctx.readonly || !editor || editor.snapshot().editingMode==='suggesting',action:()=>act('qr')}]},
                {id:'agent',labelKey:'desktop.menu_agent',items:[item('assist','assist')]},
                {id:'view',label:tr('view'),items:[item('outline','outline'),item('focus','focus_mode'),item('fit','fit')]}
            ]);
        }
        root.addEventListener('pointerdown',event=>{if(event.target.closest('.writer-toolbar,.writer-right,.writer-views'))pinSelection();},{signal:lifecycle.signal});
        root.addEventListener('click',event=>{
            const menu=find('[data-document-menu]');
            if(menu && !menu.hidden && !event.target.closest('.writer-document-menu,.writer-document-name'))closeDocumentMenu();
            const control=event.target.closest('[data-action],[data-slot]');if(!control || control.disabled)return;
            if(control.dataset.action)void act(control.dataset.action);else if(control.matches('button'))run(lib?.commandForSlot(control.dataset.slot));
        },{signal:lifecycle.signal});
        root.addEventListener('change',event=>{
            const el=event.target;if(!el.dataset.slot)return;
            if(!el.checkValidity()){el.reportValidity();return;}
            run(valueCommand(el));
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
        root.addEventListener('keydown',event=>{
            const command=event.ctrlKey || event.metaKey,action=command && {s:event.shiftKey?'saveAs':'save',o:'open',n:'new',f:'search',h:'search',p:'print'}[event.key.toLowerCase()];
            if(action){event.preventDefault();event.stopPropagation();act(action);}
            if(event.key==='Escape'){
                // The menu is the topmost layer; focus mode hides every panel and leaves on Escape.
                // The engine keeps the event: Escape also leaves a note scope or the format painter.
                if(!find('[data-document-menu]').hidden){closeDocumentMenu();return;}
                if(root.classList.contains('writer-focus')){setFocus(false);return;}
                if(rightPanel){showPanel('right',rightPanel);return;}
                if(leftPanel){showPanel('left',leftPanel);return;}
            }
        },{capture:true,signal:lifecycle.signal});
        window.addEventListener('beforeunload',event=>{if(queue?.dirty){event.preventDefault();event.returnValue='';}},{signal:lifecycle.signal});
        function cleanup() {
            if(disposed)return;disposed=true;generation++;
            clearTimeout(updateTimer);clearTimeout(draftTimer);clearTimeout(draftDeadline);
            lifecycle.abort();documentLife.abort();rulerResize.disconnect();panels.dispose();queue?.dispose();editor?.destroy();
            ctx.setWindowBeforeClose?.(windowId,null);ctx.clearWindowMenus?.(windowId);instances.delete(windowId);
        }
        setMenus();void load(path,ctx.path?null:'blank');
    }
    function dispose(windowId){instances.get(windowId)?.dispose();}
    window.WriterApp={render,dispose,instances};
})();
