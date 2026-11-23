(function () {
    'use strict';
    let libraries, mermaidPromise, diagramSeq=0;
    function mermaid(){return mermaidPromise ||= new Promise((resolve,reject)=>{if(window.mermaid){resolve(window.mermaid);return;}const script=document.createElement('script');script.src='/js/vendor/mermaid.min.js';script.onload=()=>{window.mermaid.initialize({startOnLoad:false,securityLevel:'strict',theme:'neutral',suppressErrorRendering:true,maxTextSize:30000});resolve(window.mermaid);};script.onerror=()=>{mermaidPromise=null;script.remove();reject(Error('Diagram renderer unavailable'));};document.head.append(script);});}
    const asset = path => path;
    async function load() {
        if (!libraries) libraries = Promise.all([import(asset('/js/vendor/notes/engine.js')), import(asset('/js/vendor/codemirror-bundle.esm.js'))]).catch(error => { libraries = null; throw error; });
        return libraries;
    }
    // Unsupported syntax remains editable as literal Markdown, without a lossy rich round trip.
    function needsSource(text) {
        // ponytail: large notes use CodeMirror virtualization; rich layout is bounded at 200k characters.
        if(text.length>200000)return true;
        const body = window.NotesFrontmatter.strip(text).replace(/^```[^\n]*\n[\s\S]*?^```\s*$/gm, '');
        return /(^|\n)\s*(?:<[/!A-Za-z]|:::\S|\[\^[^\]]+\]:)|\[\^[^\]]+\]|\$\$|<\/?[A-Za-z][^>]*>/.test(body) || (window.NotesFrontmatter.parse(text).present && !window.NotesFrontmatter.parse(text).valid);
    }
    // Callout icons: the desktop mini-icon sprite has no info/help/alert glyphs, so the editor ships its own.
    const CALLOUT_ICON_PATHS = {
        info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5"/><path d="M12 8h.01"/>',
        star: '<path d="m12 3 2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1-4.4-4.3 6.1-.9z"/>',
        check: '<path d="m5 12.5 4.5 4.5L19 7.5"/>',
        'help-circle': '<circle cx="12" cy="12" r="9"/><path d="M9.5 9.5a2.5 2.5 0 1 1 3.6 2.2c-.7.4-1.1.9-1.1 1.8"/><path d="M12 17h.01"/>',
        'alert-triangle': '<path d="M10.3 4.2 2.7 17.5A2 2 0 0 0 4.4 20.5h15.2a2 2 0 0 0 1.7-3L13.7 4.2a2 2 0 0 0-3.4 0z"/><path d="M12 9.5v4"/><path d="M12 17h.01"/>',
        'alert-octagon': '<path d="M8 3h8l5 5v8l-5 5H8l-5-5V8z"/><path d="M12 8v5"/><path d="M12 16.5h.01"/>',
        list: '<path d="M9 6h11M9 12h11M9 18h11"/><path d="M4.5 6h.01M4.5 12h.01M4.5 18h.01"/>',
        'format-quote': '<path d="M10 8H6.5A1.5 1.5 0 0 0 5 9.5V13h4.5a1.5 1.5 0 0 0 1.5-1.5z"/><path d="M5 13c0 2.5-.7 4-2 5"/><path d="M19 8h-3.5A1.5 1.5 0 0 0 14 9.5V13h4.5a1.5 1.5 0 0 0 1.5-1.5z"/><path d="M14 13c0 2.5-.7 4-2 5"/>',
        'file-text': '<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/><path d="M9 13h6M9 17h6"/>'
    };
    function calloutIcon(name) {
        const paths = CALLOUT_ICON_PATHS[name];
        return paths ? '<svg class="notes-icon" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + paths + '</svg>' : '';
    }
    async function create(mount, options) {
        const [lib, cm] = await load();
        const raw = String(options.content || ''), parsed = NotesFrontmatter.parse(raw);
        let prefix = parsed.valid ? raw.slice(0, raw.length - NotesFrontmatter.strip(raw).length) : '';
        let originalBody = NotesFrontmatter.strip(raw), originalDoc, sourceMode = false, source, muted = true, disposed = false;
        mount.innerHTML = '<div class="notes-rich"></div><div class="notes-source" hidden></div>';
        const rich = mount.firstElementChild, sourceHost = mount.lastElementChild;
        const crepe = new lib.CrepeBuilder({root: rich, defaultValue: needsSource(raw)?'':NotesFrontmatter.strip(raw)})
            .addFeature(lib.listItem).addFeature(lib.cursor)
            .addFeature(lib.codeMirror, {copyText:options.tr('copy'),searchPlaceholder:options.tr('language'),noResultText:options.tr('no_results'),previewLabel:options.tr('preview'),previewLoading:options.tr('loading'),previewToggleText:()=>options.tr('preview'),renderPreview:(language,content,apply)=>{
                if(language!=='mermaid')return null;
                const placeholder=document.createElement('div');placeholder.textContent=options.tr('loading');
                if(content.length>30000){placeholder.textContent=options.tr('mermaid_failed');return placeholder;}
                mermaid().then(async api=>{const result=await api.render('notes-diagram-'+(++diagramSeq),content);if(disposed)return;const preview=document.createElement('div');preview.className='notes-diagram';preview.innerHTML=window.DOMPurify.sanitize(result.svg,{USE_PROFILES:{svg:true,svgFilters:true}});apply(preview);}).catch(()=>{if(!disposed){placeholder.textContent=options.tr('mermaid_failed');apply(placeholder);}});
                return placeholder;
            }})
            .addFeature(lib.placeholder, {text: options.tr('start_writing')})
            .addFeature(lib.imageBlock, {onUpload: options.upload, inlineOnUpload: options.upload, blockOnUpload: options.upload, proxyDomURL: options.resolveURL,
                inlineUploadPlaceholderText: options.tr('image_url'), blockUploadPlaceholderText: options.tr('image_url'),
                inlineUploadButton: options.tr('upload'), blockUploadButton: options.tr('upload'), blockCaptionPlaceholderText: options.tr('caption')});
        crepe.editor.use(lib.obsidian({
            resolve: value => options.resolveWiki?.(value) || null,
            open: (value, event) => options.openWiki?.(value, event),
            embed: options.embedWiki ? (value, element, parsedValue) => options.embedWiki(value, element, parsedValue) : undefined,
            preview: (value, anchor) => options.previewWiki?.(value, anchor),
            tag: (value, event) => options.tagClick?.(value, event),
            icon: name => (options.icon ? options.icon(name) : '') || calloutIcon(name),
            label: type => {
                const canonical = String(type || '').toLowerCase();
                const key = 'callout_' + canonical, text = options.tr(key);
                return text && text !== key ? text : canonical.charAt(0).toUpperCase() + canonical.slice(1);
            }
        }));
        await crepe.create();
        const view = crepe.editor.action(ctx => ctx.get(lib.editorViewCtx));
        originalDoc = view.state.doc;
        crepe.setReadonly(options.readonly);
        view.dom.setAttribute('aria-label', options.tr('writing_area'));
        view.dom.setAttribute('spellcheck', 'true');
        const previousDispatch = view.props.dispatchTransaction;
        view.setProps({
            dispatchTransaction(transaction) {
                if (previousDispatch) previousDispatch.call(view, transaction);
                else view.updateState(view.state.apply(transaction));
                if (!muted && transaction.docChanged) options.changed();
                options.selectionChanged?.();
            },
            handleDOMEvents: {
                click: (_, event) => {
                    const link = event.target.closest('a');
                    if (link) { event.preventDefault(); options.openLink?.(link.getAttribute('href')); return true; }
                    return false;
                }
            }
        });
        const body = () => crepe.getMarkdown().replace(/\n/g, parsed.eol || '\n');
        const content = () => sourceMode ? source.state.doc.toString() : prefix + (view.state.doc.eq(originalDoc) ? originalBody : body());
        const focus = () => {
            if (sourceMode) source.contentDOM.focus({preventScroll:true});
            else view.dom.focus({preventScroll:true});
        };
        function setContent(text, history = true) {
            if (options.readonly || disposed) return;
            if (sourceMode) source.dispatch({changes:{from:0,to:source.state.doc.length,insert:text}});
            else {
                const fm = NotesFrontmatter.parse(text);
                const oldPrefix = prefix;
                prefix = fm.valid ? text.slice(0, text.length - NotesFrontmatter.strip(text).length) : '';
                const doc = crepe.editor.action(ctx => ctx.get(lib.parserCtx)(NotesFrontmatter.strip(text)));
                view.dispatch(lib.closeHistory(view.state.tr).replaceWith(0, view.state.doc.content.size, doc.content).setMeta('addToHistory',history));
                if(oldPrefix !== prefix) options.changed();
            }
        }
        async function mode(value) {
            if (value === sourceMode) return true;
            if (!value && needsSource(content())) return false;
            const text = content();
            muted = true;
            if (value) {
                if (!source) source = new cm.EditorView({parent:sourceHost,state:cm.EditorState.create({doc:text,extensions:[
                    cm.lineNumbers(), cm.history(), cm.markdown(), cm.drawSelection(), cm.syntaxHighlighting(cm.defaultHighlightStyle),
                    cm.keymap.of([...cm.defaultKeymap,...cm.historyKeymap,...cm.searchKeymap]),
                    cm.EditorView.lineWrapping, cm.EditorState.readOnly.of(!!options.readonly), cm.EditorView.editable.of(!options.readonly),
                    cm.EditorView.contentAttributes.of({'aria-label':options.tr('markdown_source')}),
                    cm.EditorView.updateListener.of(update => { if(update.docChanged&&!muted)options.changed(); }),
                ]})});
                else source.dispatch({changes:{from:0,to:source.state.doc.length,insert:text}});
                // Keep the source editor configuration and history between visits.
                sourceMode = true;
            } else {
                const fm = NotesFrontmatter.parse(text);
                prefix = fm.valid ? text.slice(0,text.length-NotesFrontmatter.strip(text).length) : '';
                originalBody = NotesFrontmatter.strip(text);
                const doc = crepe.editor.action(ctx=>ctx.get(lib.parserCtx)(NotesFrontmatter.strip(text)));
                if (!view.state.doc.eq(doc)) view.dispatch(lib.closeHistory(view.state.tr).replaceWith(0,view.state.doc.content.size,doc.content));
                originalDoc = view.state.doc;
                sourceMode = false;
            }
            rich.hidden = sourceMode; sourceHost.hidden = !sourceMode; muted = false;
            return true;
        }
        function selection(paragraph = false) {
            if(sourceMode) {
                const range = source.state.selection.main;
                const line = source.state.doc.lineAt(range.from);
                const from = range.empty&&paragraph?line.from:range.from, to=range.empty&&paragraph?line.to:range.to;
                return {from,to,text:source.state.sliceDoc(from,to),source:true};
            }
            let {from,to,$from,empty} = view.state.selection;
            if(empty&&paragraph){from=$from.start();to=$from.end();}
            return {from,to,text:view.state.doc.textBetween(from,to,'\n'),source:false};
        }
        function replaceSelection(text, range = selection()) {
            if(options.readonly)return false;
            if(range.source) source.dispatch({changes:{from:range.from,to:range.to,insert:text},selection:{anchor:range.from+text.length}});
            else view.dispatch(lib.closeHistory(view.state.tr).insertText(text,range.from,range.to));
            return true;
        }
        function insert(markdown) {
            if(options.readonly)return;
            if(sourceMode){replaceSelection(markdown);return;}
            const doc=crepe.editor.action(ctx=>ctx.get(lib.parserCtx)(markdown));
            view.dispatch(lib.closeHistory(view.state.tr).replaceSelection(new lib.Slice(doc.content,0,0)));
            focus();
        }
        function command(name,value) {
            if(options.readonly)return false;
            if(sourceMode){
                if(name==='highlight'){const range=selection();replaceSelection('=='+range.text+'==',range);focus();return true;}
                return name==='undo'?cm.undo(source):name==='redo'?cm.redo(source):false;
            }
            const s=view.state.schema;
            let action;
            const marks={bold:'strong',italic:'emphasis',strike:'strike_through',code:'inlineCode'};
            if(marks[name]) {const mark=s.marks[marks[name]];if(mark)action=lib.toggleMark(mark);}
            else if(name==='style')action=lib.setBlockType(value==='p'?s.nodes.paragraph:s.nodes.heading,value==='p'?null:{level:Number(value)});
            else if(name==='bullet'||name==='ordered')action=lib.wrapInList(s.nodes[name==='bullet'?'bullet_list':'ordered_list']);
            else if(name==='quote')action=lib.wrapIn(s.nodes.blockquote);
            else if(name==='indent')action=lib.sinkListItem(s.nodes.list_item);
            else if(name==='outdent')action=lib.liftListItem(s.nodes.list_item);
            else if(name==='link')action=lib.toggleMark(s.marks.link,{href:value});
            else if(name==='align')action=lib.setCellAttr('alignment',value);
            else if(name==='highlight')action=lib.obsidianCommands.toggleHighlight;
            else action=lib[name];
            const result=typeof action==='function'&&action(view.state,view.dispatch,view);
            focus();return !!result;
        }
        function insertWiki(value, embed=false) {
            if(options.readonly)return;
            if(sourceMode){replaceSelection((embed?'!':'')+'[['+value+']]');focus();return;}
            lib.obsidianCommands.insertWiki(view,value,embed);focus();
        }
        function insertCallout(type='note', title='') {
            if(options.readonly)return;
            if(sourceMode){insert('> [!'+type+']'+(title?' '+title:'')+'\n> ');return;}
            lib.obsidianCommands.insertCallout(view,type,title);focus();
        }
        // Chips resolve links when they are drawn: swapping the node-view factories makes ProseMirror redraw them.
        const baseNodeViews = {...(view.nodeViews || {})};
        function refreshDecorations() {
            if(disposed||sourceMode)return;
            const views={};
            for(const name of ['wiki_link','hashtag','callout_marker'])if(baseNodeViews[name])views[name]=(...args)=>baseNodeViews[name](...args);
            view.setProps({nodeViews:views});
            view.dispatch(view.state.tr.setMeta('notesRefresh',true));
        }
        function outline() {
            if(sourceMode)return source.state.doc.toString().split('\n').reduce((all,line,index)=>{const m=/^(#{1,6})\s+(.+)/.exec(line);if(m)all.push({level:m[1].length,text:m[2],pos:source.state.doc.line(index+1).from});return all;},[]);
            const result=[];view.state.doc.descendants((node,pos)=>{if(node.type.name==='heading')result.push({level:node.attrs.level,text:node.textContent,pos});});return result;
        }
        function jump(pos) {
            if(sourceMode)source.dispatch({selection:{anchor:pos},scrollIntoView:true});
            else view.dispatch(view.state.tr.setSelection(lib.TextSelection.near(view.state.doc.resolve(pos+1))).scrollIntoView());
            focus();
        }
        function findText(query, replacement, all = false, caseSensitive = false) {
            if(!query)return 0;
            const matches=[],pattern=new RegExp(query.replace(/[.*+?^${}()|[\]\\]/g,'\\$&'),caseSensitive?'gu':'giu');
            const scan=(text,start)=>{for(const match of text.matchAll(pattern))matches.push({from:start+match.index,to:start+match.index+match[0].length});};
            if(sourceMode)scan(source.state.doc.toString(),0);
            else view.state.doc.descendants((node,pos)=>{if(node.isTextblock){scan(node.textBetween(0,node.content.size,'','\ufffc'),pos+1);return false;}});
            if(replacement!==undefined&&!options.readonly){
                const chosen=all?matches:matches.slice(0,1);
                if(sourceMode)source.dispatch({changes:chosen.map(m=>({...m,insert:replacement}))});
                else {let tr=lib.closeHistory(view.state.tr);for(const m of chosen.toReversed())tr=tr.insertText(replacement,m.from,m.to);view.dispatch(tr);}
            }else if(matches.length){
                const current=selection().to,match=matches.find(m=>m.from>=current)||matches[0];
                if(sourceMode)source.dispatch({selection:{anchor:match.from,head:match.to},scrollIntoView:true});
                else view.dispatch(view.state.tr.setSelection(lib.TextSelection.create(view.state.doc,match.from,match.to)).scrollIntoView());
            }
            return matches.length;
        }
        function relocateLinks(text,resolve) {
            const markdown=NotesFrontmatter.strip(text),offset=text.length-markdown.length;
            const tree=crepe.editor.action(ctx=>ctx.get(lib.remarkCtx).parse(markdown)),edits=[];
            function visit(node){
                if(['link','image','definition'].includes(node.type)){
                    const start=node.position.start.offset,segment=markdown.slice(start,node.position.end.offset);
                    let cursor=0;
                    if(node.type==='definition')cursor=segment.indexOf(']:')+2;
                    else {
                        cursor=segment[0]==='!'?2:1;let depth=1;
                        while(cursor<segment.length&&depth){const c=segment[cursor++];if(c==='\\')cursor++;else if(c==='[')depth++;else if(c===']')depth--;}
                        while(/\s/.test(segment[cursor]||'')&&cursor<segment.length)cursor++;
                        if(segment[cursor++]!=='(')return;
                    }
                    while(cursor<segment.length&&/\s/.test(segment[cursor]))cursor++;
                    const angle=segment[cursor]==='<';if(angle)cursor++;
                    const begin=cursor;let depth=0;
                    while(cursor<segment.length){const c=segment[cursor];if(c==='\\'){cursor+=2;continue;}if(angle&&c==='>')break;if(!angle){if(c==='(')depth++;if(c===')'&&depth--===0||/\s/.test(c))break;}cursor++;}
                    const url=resolve(node.url);if(url!==node.url)edits.push({from:offset+start+begin,to:offset+start+cursor,url});
                }
                for(const child of node.children||[])visit(child);
            }
            visit(tree);
            for(const edit of edits.sort((a,b)=>b.from-a.from))text=text.slice(0,edit.from)+edit.url+text.slice(edit.to);
            return text;
        }
        muted=false;
        if(needsSource(raw))await mode(true);
        return {content,setContent,relocateLinks,mode,selection,replaceSelection,insert,command,insertWiki,insertCallout,refreshDecorations,outline,jump,findText,focus,
            get sourceMode(){return sourceMode;},get view(){return view;},
            get text(){return sourceMode?NotesFrontmatter.strip(content()):view.state.doc.textBetween(0,view.state.doc.content.size,'\n');},
            async destroy(){disposed=true;source?.destroy();await crepe.destroy();}
        };
    }
    window.NotesEditor={create,needsSource};
})();
