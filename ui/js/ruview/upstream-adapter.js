/* Reviewed adapter for RuView v3122; AuraGo owns authentication and permissions.
   An opaque frame (origin "null") cannot fetch or open sockets. It posts those
   calls to the Desktop parent, which relays only reviewed paths. */
(() => {
    'use strict';
    const base = location.pathname.includes('/desktop/integrations/') ? '/api/desktop/integrations/ruview/' : '/api/ruview/';
    const nativeFetch = window.fetch.bind(window), NativeSocket = window.WebSocket;
    const channel = 'aurago-ruview';
    const opaque = window.origin === 'null';
    const sockets = new Set(), requests = new Set(), pending = new Map(), relaySockets = new Map();
    let readonly=true, enabled=false, simulation=false, policyTimer, observer, capabilities=null, policyBusy=false, nextID=0;
    const controlStates=new WeakMap();
    const stopActions=new Set(['recording_stop','training_stop']);
    Object.defineProperty(window,'auragoRuViewSimulationAllowed',{value:()=>enabled&&simulation});
    const lifetime=new AbortController();
    const blocked = /\/(?:oauth|auth)(?:\/|$)|\/models\/(?:load|unload|lora\/activate)$|\/model\/sona\/activate$|\/pose\/(?:stats|zones\/summary)$/;
    function post(message) { window.parent.postMessage({channel, ...message}, '*'); }
    function classify(input) {
        const u = new URL(input, location.href);
        u.searchParams.delete('ticket');
        if (u.search || u.hash) throw new Error('Query filters are unavailable');
        const path = u.pathname;
        if (blocked.test(path)) throw new Error('Unavailable in this RuView version');
        if (path === base + 'status' || path === base + 'capabilities') return {app: path.slice(base.length)};
        if (path.startsWith(base + 'ui/') && path.endsWith('.wasm')) return {asset: path};
        const proxy = base + 'proxy';
        if (path.startsWith(proxy + '/')) return {path: path.slice(proxy.length)};
        if (path.startsWith('/api/') || path.startsWith('/ws/') || path.startsWith('/health')) return {path};
        throw new Error('External network access is unavailable in embedded RuView');
    }
    function call(message, signal) {
        const id = String(++nextID);
        return new Promise((resolve, reject) => {
            let pendingResult, timer;
            const finish = error => { clearTimeout(timer); pending.delete(id); signal?.removeEventListener('abort', abort); if (error) reject(error); else resolve(pendingResult); };
            const abort = () => finish(new DOMException('Aborted', 'AbortError'));
            timer = setTimeout(() => finish(new Error('RuView relay timed out')), 15000);
            pending.set(id, data => { pendingResult = data; finish(); });
            if (signal?.aborted) { abort(); return; }
            signal?.addEventListener('abort', abort, {once:true});
            try { post({...message, id}); } catch (error) { clearTimeout(timer); pending.delete(id); reject(error); }
        });
    }
    if (opaque) addEventListener('message', event => {
        if (event.source !== window.parent) return;
        const data = event.data;
        if (!data || data.channel !== channel || typeof data.id !== 'string') return;
        if (data.kind === 'http' && pending.has(data.id)) { const settle = pending.get(data.id); pending.delete(data.id); settle(data); return; }
        const socket = relaySockets.get(data.id);
        if (!socket || data.kind !== 'ws') return;
        if (data.event === 'open') { socket.readyState = 1; socket.dispatchEvent(new Event('open')); const queued = socket._queue; socket._queue = []; for (const item of queued) socket.send(item); }
        else if (data.event === 'message') socket.dispatchEvent(new MessageEvent('message', {data: data.data}));
        else if (data.event === 'error') socket.dispatchEvent(new Event('error'));
        else if (data.event === 'close') { socket.readyState = 3; relaySockets.delete(data.id); sockets.delete(socket); socket.dispatchEvent(new CloseEvent('close')); }
    });
    async function readBody(body) {
        if (body == null) return null;
        if (typeof body === 'string') return body;
        if (body instanceof URLSearchParams) return body.toString();
        if (body instanceof Blob) { if (body.size > 65536) throw new Error('Request body is too large'); return await body.text(); }
        if (body instanceof ArrayBuffer) return new TextDecoder().decode(body);
        if (ArrayBuffer.isView(body)) return new TextDecoder().decode(body.buffer);
        if (typeof body.getReader === 'function') {
            const reader = body.getReader(); const chunks = []; let total = 0;
            for (;;) { const step = await reader.read(); if (step.done) break; total += step.value.byteLength; if (total > 65536) throw new Error('Request body is too large'); chunks.push(step.value); }
            const joined = new Uint8Array(total); let offset = 0;
            for (const chunk of chunks) { joined.set(chunk, offset); offset += chunk.byteLength; }
            return new TextDecoder().decode(joined);
        }
        return String(body);
    }
    function routed(input) {
        const u = new URL(input, location.href);
        if (u.pathname.startsWith(base)) return u;
        if (blocked.test(u.pathname)) throw new Error('Unavailable in this RuView version');
        if (u.pathname.startsWith('/api/') || u.pathname.startsWith('/ws/') || u.pathname.startsWith('/health')) {
            u.protocol = location.protocol; u.host = location.host; u.pathname = base + 'proxy' + u.pathname; u.searchParams.delete('ticket'); return u;
        }
        if (u.origin !== location.origin) throw new Error('External network access is unavailable in embedded RuView');
        return u;
    }
    window.fetch = async (input, options = {}) => {
        const request = input instanceof Request ? input : null;
        const method=(options.method||request?.method||'GET').toUpperCase();
        const controller = new AbortController(); requests.add(controller);
        const signal = options.signal || request?.signal;
        const abort = () => controller.abort(); signal?.addEventListener('abort', abort, {once:true});
        if (signal?.aborted) abort();
        try {
            if (!opaque) {
                const url = routed(request ? request.url : input);
                const stop=method==='POST'&&[base+'proxy/api/v1/recording/stop',base+'proxy/api/v1/train/stop'].includes(url.pathname);
                if(url.pathname.startsWith(base+'proxy/')&&method!=='GET'&&method!=='HEAD'&&(!enabled||readonly&&!stop))throw new Error('Read-only in AuraGo');
                const headers = new Headers(options.headers || request?.headers);
                headers.delete('Authorization'); headers.delete('X-Api-Key');
                return await nativeFetch(url, {...(request ? {method:request.method,body:request.body} : {}), ...options, headers, signal:controller.signal, credentials:'same-origin'});
            }
            const target = classify(request ? request.url : input);
            const stop = method==='POST'&&(target.path==='/api/v1/recording/stop'||target.path==='/api/v1/train/stop');
            if(target.path&&method!=='GET'&&method!=='HEAD'&&(!enabled||readonly&&!stop))throw new Error('Read-only in AuraGo');
            const headers = new Headers(options.headers || request?.headers);
            headers.delete('Authorization'); headers.delete('X-Api-Key');
            const body = method==='GET'||method==='HEAD' ? null : await readBody(options.body ?? request?.body);
            if (body != null && body.length > 65536) throw new Error('Request body is too large');
            const reply = await call({kind:'http', method, path:target.path||'', app:target.app||'', asset:target.asset||'', contentType:headers.get('Content-Type')||'', accept:headers.get('Accept')||'', body}, controller.signal);
            if (reply.blocked) throw new Error('Unavailable in this RuView version');
            // A synthetic wasm response is not a network body. The pinned loader
            // falls back to arrayBuffer only when the type is not application/wasm.
            const type = target.asset ? 'application/octet-stream' : (reply.contentType || '');
            const responseHeaders = type ? {'Content-Type': type} : undefined;
            return new Response(reply.body ?? null, {status: reply.status || 502, headers: responseHeaders});
        } finally { requests.delete(controller); signal?.removeEventListener('abort', abort); }
    };
    function handlerProperty(socket, type) {
        const name = 'on' + type;
        Object.defineProperty(socket, name, {configurable:true, get(){return socket['_'+name]||null;}, set(fn){
            if (socket['_'+name]) socket.removeEventListener(type, socket['_'+name]);
            socket['_'+name] = typeof fn === 'function' ? fn : null;
            if (socket['_'+name]) socket.addEventListener(type, socket['_'+name]);
        }});
    }
    if (opaque) {
        window.WebSocket = class RelaySocket extends EventTarget {
            static CONNECTING=0; static OPEN=1; static CLOSING=2; static CLOSED=3;
            constructor(input) {
                super();
                this.readyState=0; this.bufferedAmount=0; this.extensions=''; this.protocol=''; this.binaryType='blob'; this.url=String(input); this._queue=[];
                const target=classify(input);
                if (!target.path || !(target.path.startsWith('/ws/') || target.path === '/api/v1/stream/pose')) throw new Error('WebSocket endpoint is unavailable');
                this._id=String(++nextID);
                for (const type of ['open','message','error','close']) handlerProperty(this, type);
                relaySockets.set(this._id, this); sockets.add(this);
                post({kind:'ws-open', id:this._id, path:target.path});
            }
            send(data){ if(this.readyState===0){this._queue.push(data);return;} if(this.readyState!==1)throw new Error('WebSocket is closed'); post({kind:'ws-send', id:this._id, data:typeof data==='string'?data:null}); }
            close(){ if(this.readyState===3)return; this.readyState=2; post({kind:'ws-close', id:this._id}); }
        };
    } else window.WebSocket = class extends NativeSocket {
        constructor(input, protocols) { const u=routed(input); u.protocol=location.protocol==='https:'?'wss:':'ws:'; super(u.href,protocols); sockets.add(this); this.addEventListener('close',()=>sockets.delete(this)); }
    };
    const memory = new Map();
    const tourKey='ruview-onboarding-done',savedTourKey='aurago.ruview.onboarding.done';
    let preferences;
    if (opaque) { if (window.name === 'aurago-tour-done') memory.set(tourKey,'true'); }
    else try { preferences=window.localStorage;if(preferences.getItem(savedTourKey)==='true')memory.set(tourKey,'true'); } catch (_) {}
    const storage = {
        getItem:k => /token|simulation|server|url/i.test(k)?null:(memory.get(k)??null),
        setItem(k,v){
            if(/token|simulation|server|url/i.test(k))return;
            memory.set(k,String(v));
            if(k!==tourKey)return;
            if(opaque){try{post({kind:'storage', op:String(v)==='true'?'set':'remove', value:String(v)==='true'?'true':''});}catch(_){}return;}
            try{if(String(v)==='true')preferences?.setItem(savedTourKey,'true');else preferences?.removeItem(savedTourKey);}catch(_){}
        },
        removeItem(k){memory.delete(k);if(k!==tourKey)return;if(opaque){try{post({kind:'storage', op:'remove'});}catch(_){}return;}try{preferences?.removeItem(savedTourKey);}catch(_){}},
        clear(){memory.clear();if(opaque){try{post({kind:'storage', op:'clear'});}catch(_){}return;}try{preferences?.removeItem(savedTourKey);}catch(_){} }
    };
    try { Object.defineProperty(window,'localStorage',{value:storage}); Object.defineProperty(window,'sessionStorage',{value:storage}); } catch (_) {}
    function gate(control, denied, reason){
        let state=controlStates.get(control);
        if(!state){
            const prototype=control instanceof HTMLSelectElement?HTMLSelectElement.prototype:HTMLButtonElement.prototype;
            const descriptor=Object.getOwnPropertyDescriptor(prototype,'disabled');
            state={business:descriptor.get.call(control),denied:false,title:control.title};controlStates.set(control,state);
            Object.defineProperty(control,'disabled',{configurable:true,get:()=>descriptor.get.call(control),set:value=>{state.business=!!value;descriptor.set.call(control,state.business||state.denied);}});
            state.apply=()=>descriptor.set.call(control,state.business||state.denied);
        }
        state.denied=denied;state.apply();control.title=denied?reason:state.title;
    }
    function policyControls(){
        for(const button of document.querySelectorAll('button,select[data-ruview-action]')){
            const label=button.textContent.trim();
            const action=button.dataset.ruviewAction;
            const unavailable=action?capabilities===null||capabilities.has(action):/^(Sign In|Connect Account)$/i.test(label);
            const mutator=!!action||/^(Calibrate|Start Calibration|Stop Calibration|Reset Calibration|Promote|Refine)/i.test(label);
            if(action||unavailable||mutator)gate(button,unavailable||mutator&&(!enabled||readonly&&!stopActions.has(action)),unavailable?'Unavailable in this RuView service':'Read-only in AuraGo');
        }
        document.querySelectorAll('input[type=password]').forEach(input=>{input.disabled=true;input.value='';input.placeholder='Managed by AuraGo';});
    }
    async function policy(){
        if(policyBusy||lifetime.signal.aborted)return;policyBusy=true;
        try{const load=opaque?window.fetch:nativeFetch;const response=await load(base+'status',{credentials:'same-origin',signal:lifetime.signal});if(!response.ok)throw Error();const state=await response.json();readonly=state.readonly!==false;enabled=state.enabled===true;simulation=state.source==='simulated'&&!state.error;
            if(capabilities===null){const response=await load(base+'capabilities',{credentials:'same-origin',signal:lifetime.signal});if(!response.ok)throw Error();const data=await response.json();if(!Array.isArray(data.unavailable))throw Error();capabilities=new Set(data.unavailable);}
        }
        catch{readonly=true;enabled=false;}
        finally{policyBusy=false;}
        if(!enabled)for(const socket of sockets)socket.close();
        policyControls();
    }
    addEventListener('DOMContentLoaded',()=>{policy();policyTimer=setInterval(policy,3000);observer=new MutationObserver(policyControls);observer.observe(document.body,{childList:true,subtree:true});});
    addEventListener('pagehide',()=>{lifetime.abort();clearInterval(policyTimer);observer?.disconnect();for(const s of sockets)s.close();for(const r of requests)r.abort();memory.clear();});
    document.addEventListener('click', event => {
        const link = event.target.closest('a[href]'); if (!link) return;
        const u = new URL(link.href,location.href);
        const pageOrigin = new URL(location.href).origin;
        if (u.origin!==pageOrigin || blocked.test(u.pathname)) { event.preventDefault();return; }
        const name=u.pathname.split('/').pop();
        if (['index.html','observatory.html','pose-fusion.html','viz.html'].includes(name)) {event.preventDefault();location.href=base+'ui/'+name;}
    });
})();
