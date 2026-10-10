(function () {
    'use strict';
    const instances = new Map(), API = '/api/desktop/layerling/';
    const extensions = ['.lyl', '.stl', '.obj', '.3mf', '.step', '.stp', '.svg'];
    const readOperations = new Set(['get_scene', 'list_objects', 'list_edges', 'inspect_errors', 'capture_image', 'estimate_print', 'list_custom_shapes', 'list_reference_points']);

    async function render(root, id, ctx) {
        dispose(id);
        const t = key => ctx.t('layerling.' + key);
        const life = new AbortController();
        const s = { id, root, ctx, life, path: ctx.path || '', version: null, readonly: !!ctx.readonly, port: null, socket: null, editorID: '', pending: new Map(), started: false, loading: true, queue: null, jobs: Promise.resolve(), draftTimer: null, lastDraft: null };
        instances.set(id, s);
        const button = (action, label) => `<button type="button" class="vd-button" data-action="${action}">${ctx.esc(label)}</button>`;
        root.innerHTML = `<div class="vd-layerling"><div class="vd-layerling-toolbar" role="toolbar" aria-label="Layerling">
            ${button('new', t('new'))}${button('open', t('open'))}${button('save', t('save'))}${button('save_as', t('save_as'))}${button('import', t('import'))}${button('recover', t('recover'))}
            <select data-format aria-label="${ctx.esc(t('format'))}">${['stl','obj','3mf','step','png'].map(format=>`<option value="${format}">${format.toUpperCase()}</option>`).join('')}</select>
            ${button('export', t('export'))}${button('download', t('download'))}
            <a href="${API}ui/source.zip" download>${ctx.esc(t('source'))}</a><a href="${API}ui/LICENSE.txt" target="_blank" rel="noopener">AGPL-3.0</a>
        </div><div class="vd-layerling-status" role="status" aria-live="polite"></div><div class="vd-layerling-host"></div></div>`;
        s.status = root.querySelector('.vd-layerling-status');
        s.show = (text, error) => { s.status.textContent = text; s.status.classList.toggle('is-error', !!error); };
        s.show(t('loading'));
        s.run = job => { const next = s.jobs.then(() => { if(life.signal.aborted) throw Error(t('disconnected')); return job(); }); s.jobs = next.catch(error => { if(!life.signal.aborted) s.show(error.message, true); }); return next; };
        s.request = (action, params = {}) => {
            if (!s.port || life.signal.aborted) return Promise.reject(Error(t('disconnected')));
            return new Promise((resolve, reject) => {
                const key = crypto.randomUUID();
                const timer = setTimeout(() => { reject(Error(t('uncertain'))); s.port?.close(); s.port=null; s.socket?.close(); for(const pending of s.pending.values()){clearTimeout(pending.timer);pending.reject(Error(t('uncertain')));}s.pending.clear(); }, 90000);
                s.pending.set(key, { resolve, reject, timer });
                s.port.postMessage({ type: 'aurago.layerling.request', id: key, action, params });
            });
        };
        const signal = life.signal;
        const fileURL = path => API + 'file?path=' + encodeURIComponent(path);
        const agentHeaders = () => s.agentCommand ? {'X-Layerling-Editor':s.editorID,'X-Layerling-Command':s.agentCommand} : {};
        const draftKey = () => location.origin + ':layerling:' + ctx.sessionKey + ':' + (s.path || 'untitled');
        const draft = async bytes => {
            if(signal.aborted) return;
            const previous=s.lastDraft;
            const next = { id: crypto.randomUUID(), bytes, path:s.path, version:s.version, key:draftKey(), sessionKey:ctx.sessionKey, updated:Date.now() };
            await OfficeSession.draft('put', next.key, next, 'layerling');
            s.lastDraft=next;
            if(previous && previous.key!==next.key)await OfficeSession.draft('deleteIf',previous.key,{id:previous.id},'layerling');
        };
        const clearDraft = async () => { const saved=s.lastDraft; s.lastDraft=null; if(saved) await OfficeSession.draft('deleteIf', saved.key, {id:saved.id}, 'layerling'); };
        const serialize = async () => (await s.request('serialize')).bytes;
        const setPath = path => { s.path=path;ctx.updateWindowContext?.(id,{path}); };
        async function read(path) {
            const response = await fetch(fileURL(path), {signal, credentials:'same-origin',headers:agentHeaders()});
            if(!response.ok) throw Error(t('file_error') + ' (' + response.status + ')');
            const bytes = await response.arrayBuffer();
            return { bytes, version:response.headers.get('ETag') };
        }
        async function put(path, bytes, version, interactive) {
            if(s.readonly || signal.aborted) throw Error(t('readonly'));
            const response = await fetch(fileURL(path), {method:'PUT',signal,credentials:'same-origin',headers:{...agentHeaders(),...(version ? {'If-Match':version} : {'If-None-Match':'*'})},body:bytes});
            if(response.status === 412 && interactive) {
                const conflict = await response.json();
                const choice = await ctx.modalDialog({signal,title:t('conflict'),message:t('conflict_help'),choices:[{value:'replace',label:t('replace')},{value:'copy',label:t('copy')}]});
                if(choice==='replace' && conflict.conflict?.version) return put(path,bytes,conflict.conflict.version,true);
                if(choice==='copy') { const selected=await chooseSave(path);if(selected) return put(selected,bytes,null,true); }
                throw Error(t('canceled'));
            }
            if(!response.ok) throw Error(t('file_error') + ' (' + response.status + ')');
            const hash = await crypto.subtle.digest('SHA-256',bytes);
            return {path,version:'"'+Array.from(new Uint8Array(hash),v=>v.toString(16).padStart(2,'0')).join('')+'"'};
        }
        async function chooseSave(name) {
            const ext='.'+name.split('.').pop().toLowerCase();
            const result=await ctx.saveFileDialog({signal,filesEndpoint:API+'files',title:t('save_as'),initialPath:s.path.split('/').slice(0,-1).join('/')||'Documents/Layerling',defaultName:name.split('/').pop(),defaultExtension:ext,filters:[{label:'Layerling',extensions:[ext]}]});
            return result?.canceled ? null : result?.path;
        }
        function makeQueue() {
            s.queue?.dispose();
            s.queue=OfficeSession.create({serialize,backup:draft,write:async bytes=>{
                const saved=await put(s.saveTarget||s.path,bytes,s.saveTarget && s.saveTarget!==s.path ? null:s.version,s.interactive!==false);
                setPath(saved.path);s.version=saved.version;
            },clearBackup:clearDraft,onState:value=>{if(!s.loading && !signal.aborted) s.show(value.error ? value.error.message : value.saving ? t('saving') : value.dirty ? t('unsaved') : (s.path||t('ready')),!!value.error);}});
            // Desktop saves are explicit; the same revision queue protects concurrent saves.
            s.queue.suspend();
        }
        makeQueue();
        s.changed = () => {
            if(s.loading || s.readonly || signal.aborted) return;
            s.queue.changed();clearTimeout(s.draftTimer);
            s.draftTimer=setTimeout(()=>s.run(async()=>{if(s.queue.dirty) await draft(await serialize());}).catch(()=>{}),1500);
        };
        async function save(target, interactive=true) {
            if(s.readonly) throw Error(t('readonly'));
            target=target || s.path || await chooseSave('Untitled.lyl');
            if(!target) return false;
            if(!/\.lyl$/i.test(target)) throw Error(t('project_extension'));
            s.saveTarget=target;s.interactive=interactive;
            if(!s.queue.dirty || target!==s.path) s.queue.changed();
            try { await s.queue.save();await ctx.loadBootstrap?.();return true; }
            finally {s.saveTarget='';s.interactive=true;}
        }
        async function guard(replacing=false) {
            if(s.queue.pending) await s.queue.pending;
            if(!s.queue.dirty) return true;
            if(s.path && !s.readonly) {try{await save();return true;}catch(_){}}
            await draft(await serialize());
            if(!replacing) return !!await ctx.confirmDialog(t('unsaved'),t('leave_draft'));
            if(s.readonly) return false;
            const target=await chooseSave(s.path || 'Untitled.lyl');
            return target ? save(target) : false;
        }
        async function load(path, replacing=true) {
            if(replacing && !await guard(true)) return false;
            const project=path ? await read(path) : {bytes:null,version:null};
            s.loading=true;clearTimeout(s.draftTimer);
            try {await s.request('load',{bytes:project.bytes});setPath(path||'');s.version=project.version;makeQueue();}
            finally{s.loading=false;}
            s.show(s.path||t('ready'));return true;
        }
        async function restoreDraft(backup) {
            s.loading=true;
            try {
                let current=null;
                if(backup.path){try{current=await read(backup.path);}catch(_){}}
                await s.request('load',{bytes:backup.bytes});
                setPath(current?.version===backup.version ? backup.path : '');s.version=current?.version===backup.version ? current.version:null;
                makeQueue();s.lastDraft=backup;
            } finally {s.loading=false;}
            s.changed();
        }
        async function recover() {
            const all=await OfficeSession.draft('getAll',undefined,undefined,'layerling');
            const drafts=(all||[]).filter(d=>d?.bytes && ![...instances.values()].some(other=>other!==s && other.ctx.sessionKey===d.sessionKey)).sort((a,b)=>b.updated-a.updated);
            if(!drafts.length){s.show(t('ready'));return;}
            const chosen=await ctx.modalDialog({signal,title:t('recover'),message:t('recover_help'),choices:drafts.map(d=>({value:d.id,label:(d.path||t('new'))+' · '+new Date(d.updated).toLocaleString()}))});
            const backup=drafts.find(d=>d.id===chosen);
            if(backup && await guard(true))await restoreDraft(backup);
        }
        async function pick(importing) {
            const result=await ctx.openFileDialog({signal,filesEndpoint:API+'files',title:t(importing?'import':'open'),initialPath:'Documents/Layerling',filters:[{label:'Layerling',extensions:importing?extensions.slice(1):extensions}]});
            if(!result?.path || result.canceled) return false;
            return /\.lyl$/i.test(result.path) ? load(result.path) : importModel(result.path);
        }
        async function importModel(path) {if(s.readonly)throw Error(t('readonly'));if(!/\.(stl|obj|3mf|step|stp|svg)$/i.test(path))throw Error(t('file_error'));const file=await read(path);await s.request('import_model',{bytes:file.bytes,name:path.split('/').pop()});s.changed();return {path};}
        async function exportModel(path,format,interactive=true) {
            if(s.readonly) throw Error(t('readonly'));
            if(!new RegExp('\\.'+format+'$','i').test(path)) throw Error(t('file_error'));
            const file=await s.request('export_model',{format,name:path.split('/').pop().replace(/\.[^.]+$/,'')});
            return put(path,file.bytes,null,interactive);
        }
        s.agent = async command => {
            const op=command.operation,p=command.arguments||{};
            const policy=await ctx.api(API+'state',{signal});
            if(policy.agent_access==='off' || !['read','write'].includes(policy.agent_access) || (!readOperations.has(op) && (policy.agent_access!=='write'||policy.readonly))) throw Error(t('readonly'));
            if(op==='open_project') {if(s.queue.dirty)throw Error(t('unsaved'));if(!/\.lyl$/i.test(p.path))throw Error(t('project_extension'));await load(p.path,false);return {path:s.path};}
            if(op==='save_project') {await save(p.path,false);return {path:s.path,version:s.version};}
            if(op==='import_model') return importModel(p.path);
            if(op==='export_model') return exportModel(p.path,p.format,false);
            const value=await s.request(op,p);if(!readOperations.has(op))s.changed();return value;
        };
        const appearance = () => ({language:document.documentElement.lang || window.SYSTEM_LANG || 'en',theme:document.body.dataset.theme==='fruity' && document.body.dataset.fruityMode!=='dark' ? 'light':'dark'});
        s.start = async () => {
            if(s.started) return;s.started=true;
            const policy=await ctx.api(API+'state',{signal});s.readonly=!!policy.readonly;
            await s.request('init',appearance());
            await s.request('policy',{readonly:s.readonly});
            if(s.path && !/\.lyl$/i.test(s.path)) {const importing=s.path;setPath('');s.loading=false;await importModel(importing);} else await load(s.path,false);
            const backup=await OfficeSession.draft('get',draftKey(),undefined,'layerling');
            if(backup?.bytes && await ctx.confirmDialog(t('recover'),t('recover_help'))) {
                await restoreDraft(backup);
            }
            if(signal.aborted)return;
            const socket=new WebSocket((location.protocol==='https:'?'wss:':'ws:')+'//'+location.host+API+'connect?window_id='+encodeURIComponent(id));s.socket=socket;
            socket.onmessage=event=>{
                let command;try{command=JSON.parse(event.data);}catch(_){socket.close();return;}
                if(command.type==='connected'){s.editorID=command.editor_id;return;}
                s.run(async()=>{
                    if(socket.readyState!==WebSocket.OPEN)throw Error(t('disconnected'));
                    s.agentCommand=command.id;
                    try{const data=await s.agent(command);if(socket.readyState===WebSocket.OPEN)socket.send(JSON.stringify({id:command.id,ok:true,data}));}
                    catch(error){if(socket.readyState===WebSocket.OPEN)socket.send(JSON.stringify({id:command.id,ok:false,error:error.message.slice(0,1024)}));throw error;}
                    finally{s.agentCommand='';}
                }).catch(()=>{});
            };
            socket.onclose=()=>{s.editorID='';if(!signal.aborted)s.show(t('disconnected'),true);};
        };
        root.querySelector('[role="toolbar"]').addEventListener('click',event=>{
            const action=event.target.closest('[data-action]')?.dataset.action;if(!action)return;
            s.run(async()=>{
                if(action==='new'){if(s.readonly)throw Error(t('readonly'));return load('');}
                if(action==='open')return pick(false);
                if(action==='import')return pick(true);
                if(action==='recover')return recover();
                if(action==='save')return save();
                if(action==='save_as'){const target=await chooseSave(s.path||'Untitled.lyl');return target?save(target):false;}
                const format=root.querySelector('[data-format]').value;
                if(action==='export'){const target=await chooseSave('Model.'+format);return target?exportModel(target,format):false;}
                if(action==='download'){
                    const file=await s.request('export_model',{format,name:'Model'}),url=URL.createObjectURL(new Blob([file.bytes],{type:file.type}));
                    const a=document.createElement('a');a.href=url;a.download=file.name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
                }
            }).catch(()=>{});
        },{signal});
        s.onEvent = (event,data) => {
            if(event==='ready')s.start().catch(error=>s.show(error.message,true));
            if(event==='changed')s.changed();
            if(event==='open')s.run(()=>pick(false)).catch(()=>{});
            if(event==='import')s.run(()=>pick(true)).catch(()=>{});
            if(event==='export')s.run(async()=>{if(/\.lyl$/i.test(data.name))return save();const target=await chooseSave(data.name);if(target)await put(target,data.bytes,null,true);}).catch(()=>{});
        };
        document.addEventListener('aurago:desktop-policy',event=>{
            if(typeof event.detail?.readonly!=='boolean')return;
            s.readonly=event.detail.readonly;
            s.request('policy',{readonly:s.readonly}).catch(()=>{});
            if(s.readonly)s.show(t('readonly'));
        },{signal});
        const observer=new MutationObserver(()=>{if(s.started&&s.port)s.request('init',appearance()).catch(()=>{});});
        observer.observe(document.body,{attributes:true,attributeFilter:['data-theme','data-fruity-mode']});
        observer.observe(document.documentElement,{attributes:true,attributeFilter:['lang']});
        signal.addEventListener('abort',()=>observer.disconnect(),{once:true});
        ctx.setWindowBeforeClose?.(id,async()=>{await s.jobs;try{return await guard();}catch(error){s.show(error.message,true);return false;}});
        ctx.registerWindowCleanup?.(id,()=>dispose(id));
        try{
            await ctx.api(API+'state',{signal});
            if(signal.aborted)return;
            const gl=document.createElement('canvas').getContext('webgl2');
            if(!gl){s.show(t('webgl'),true);root.querySelectorAll('button,select').forEach(el=>el.disabled=true);return;}
            gl.getExtension('WEBGL_lose_context')?.loseContext();
            root.querySelector('.vd-layerling-host').append(ctx.makeFrame());
        }
        catch(error){s.show(t('disabled'),true);}
    }
    function connect(id,port,signal) {
        const s=instances.get(id);if(!s)return;
        s.port=port;
        port.addEventListener('message',event=>{
            if(signal.aborted||s.life.signal.aborted)return;const msg=event.data;
            if(msg?.type==='aurago.layerling.event')s.onEvent(msg.event,msg.data);
            if(msg?.type==='aurago.layerling.response'){
                const pending=s.pending.get(msg.id);if(!pending)return;s.pending.delete(msg.id);clearTimeout(pending.timer);
                msg.ok?pending.resolve(msg.data):pending.reject(Error(msg.error||'Layerling'));
            }
        },{signal});
        signal.addEventListener('abort',()=>{s.port=null;s.socket?.close();for(const p of s.pending.values()){clearTimeout(p.timer);p.reject(Error(s.ctx.t('layerling.disconnected')));}s.pending.clear();},{once:true});
    }
    function dispose(id) {const s=instances.get(id);if(!s)return;s.life.abort();s.socket?.close();s.queue?.dispose();clearTimeout(s.draftTimer);for(const p of s.pending.values()){clearTimeout(p.timer);p.reject(Error('Layerling closed'));}s.pending.clear();instances.delete(id);}
    window.LayerlingApp={render,connect,dispose,editorId:id=>instances.get(id)?.editorID||''};
})();
