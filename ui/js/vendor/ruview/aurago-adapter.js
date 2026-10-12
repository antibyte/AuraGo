/* Reviewed adapter for RuView v3122; AuraGo owns authentication and permissions. */
(() => {
    'use strict';
    const base = location.pathname.includes('/desktop/integrations/') ? '/api/desktop/integrations/ruview/' : '/api/ruview/';
    const nativeFetch = window.fetch.bind(window), NativeSocket = window.WebSocket;
    const sockets = new Set(), requests = new Set();
    let readonly=true, enabled=false, simulation=false, policyTimer, observer;
    Object.defineProperty(window,'auragoRuViewSimulationAllowed',{value:()=>enabled&&simulation});
    const lifetime=new AbortController();
    const blocked = /\/(?:oauth|auth)(?:\/|$)|\/models\/(?:load|unload|lora\/activate)$|\/model\/sona\/activate$|\/pose\/(?:stats|zones\/summary)$/;
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
        const url = routed(request ? request.url : input);
        const method=(options.method||request?.method||'GET').toUpperCase();
        if(url.pathname.startsWith(base+'proxy/')&&method!=='GET'&&method!=='HEAD'&&(!enabled||readonly))throw new Error('Read-only in AuraGo');
        const headers = new Headers(options.headers || request?.headers);
        headers.delete('Authorization'); headers.delete('X-Api-Key');
        const controller = new AbortController(); requests.add(controller);
        const signal = options.signal || request?.signal;
        const abort = () => controller.abort(); signal?.addEventListener('abort', abort, {once:true});
        if (signal?.aborted) abort();
        try { return await nativeFetch(url, {...(request ? {method:request.method,body:request.body} : {}), ...options, headers, signal:controller.signal, credentials:'same-origin'}); }
        finally { requests.delete(controller); signal?.removeEventListener('abort', abort); }
    };
    window.WebSocket = class extends NativeSocket {
        constructor(input, protocols) { const u=routed(input); u.protocol=location.protocol==='https:'?'wss:':'ws:'; super(u.href,protocols); sockets.add(this); this.addEventListener('close',()=>sockets.delete(this)); }
    };
    // Upstream preferences remain in memory. Tokens, simulation fallbacks and
    // stored service origins must never override AuraGo's saved configuration.
    const memory = new Map();
    const storage = {getItem:k => /token|simulation|server|url/i.test(k)?null:(memory.get(k)??null),setItem:(k,v)=>{if(!/token|simulation|server|url/i.test(k))memory.set(k,String(v));},removeItem:k=>memory.delete(k),clear:()=>memory.clear()};
    try { Object.defineProperty(window,'localStorage',{value:storage}); Object.defineProperty(window,'sessionStorage',{value:storage}); } catch (_) {}
    function policyControls(){
        for(const button of document.querySelectorAll('button')){
            const label=button.textContent.trim();
            const unavailable=/^(Load Model|Unload Model|Activate LoRA|Activate Profile|Sign In|Sign in|Connect Account)$/i.test(label);
            const mutator=/^(Start Recording|Stop Recording|Delete|Start Training|Stop Training|Pretrain|LoRA|Calibrate|Start Calibration|Stop Calibration|Reset Calibration|Promote|Refine)/i.test(label);
            if(unavailable||mutator&&(!enabled||readonly)){button.disabled=true;button.dataset.auragoDisabled='1';button.title=unavailable?'Unavailable in this pinned RuView release':'Read-only in AuraGo';}
            else if(button.dataset.auragoDisabled==='1'){button.disabled=false;delete button.dataset.auragoDisabled;}
        }
        document.querySelectorAll('input[type=password]').forEach(input=>{input.disabled=true;input.value='';input.placeholder='Managed by AuraGo';});
    }
    async function policy(){
        try{const response=await nativeFetch(base+'status',{credentials:'same-origin',signal:lifetime.signal});if(!response.ok)throw Error();const state=await response.json();readonly=state.readonly;enabled=state.enabled;simulation=state.source==='simulated'&&!state.error;}
        catch{readonly=true;enabled=false;}
        if(!enabled)for(const socket of sockets)socket.close();
        policyControls();
    }
    addEventListener('DOMContentLoaded',()=>{policy();policyTimer=setInterval(policy,3000);observer=new MutationObserver(policyControls);observer.observe(document.body,{childList:true,subtree:true});});
    addEventListener('pagehide',()=>{lifetime.abort();clearInterval(policyTimer);observer?.disconnect();for(const s of sockets)s.close();for(const r of requests)r.abort();memory.clear();});
    document.addEventListener('click', event => {
        const link = event.target.closest('a[href]'); if (!link) return;
        const u = new URL(link.href,location.href);
        if (u.origin!==location.origin || blocked.test(u.pathname)) { event.preventDefault();return; }
        const name=u.pathname.split('/').pop();
        if (['index.html','observatory.html','pose-fusion.html','viz.html'].includes(name)) {event.preventDefault();location.href=base+'ui/'+name;}
    });
})();
