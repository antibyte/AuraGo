/* Reviewed adapter for RuView v3122; AuraGo owns authentication and permissions. */
(() => {
    'use strict';
    const base = location.pathname.includes('/desktop/integrations/') ? '/api/desktop/integrations/ruview/' : '/api/ruview/';
    const nativeFetch = window.fetch.bind(window), NativeSocket = window.WebSocket;
    const sockets = new Set(), requests = new Set();
    let readonly=true, enabled=false, simulation=false, policyTimer, observer, capabilities=null, policyBusy=false;
    const controlStates=new WeakMap();
    const stopActions=new Set(['recording_stop','training_stop']);
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
        const stop=method==='POST'&&[base+'proxy/api/v1/recording/stop',base+'proxy/api/v1/train/stop'].includes(url.pathname);
        if(url.pathname.startsWith(base+'proxy/')&&method!=='GET'&&method!=='HEAD'&&(!enabled||readonly&&!stop))throw new Error('Read-only in AuraGo');
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
    // Only tour completion persists; upstream tokens and service settings stay isolated.
    const memory = new Map();
    const tourKey='ruview-onboarding-done',savedTourKey='aurago.ruview.onboarding.done';
    let preferences;
    try { preferences=window.localStorage;if(preferences.getItem(savedTourKey)==='true')memory.set(tourKey,'true'); } catch (_) {}
    const storage = {
        getItem:k => /token|simulation|server|url/i.test(k)?null:(memory.get(k)??null),
        setItem(k,v){
            if(/token|simulation|server|url/i.test(k))return;
            memory.set(k,String(v));
            if(k===tourKey)try{if(String(v)==='true')preferences?.setItem(savedTourKey,'true');else preferences?.removeItem(savedTourKey);}catch(_){}
        },
        removeItem(k){memory.delete(k);if(k===tourKey)try{preferences?.removeItem(savedTourKey);}catch(_){}},
        clear(){memory.clear();try{preferences?.removeItem(savedTourKey);}catch(_){} }
    };
    try { Object.defineProperty(window,'localStorage',{value:storage}); Object.defineProperty(window,'sessionStorage',{value:storage}); } catch (_) {}
    // Preserve component-owned disabled state while applying the independent policy gate.
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
        try{const response=await nativeFetch(base+'status',{credentials:'same-origin',signal:lifetime.signal});if(!response.ok)throw Error();const state=await response.json();readonly=state.readonly!==false;enabled=state.enabled===true;simulation=state.source==='simulated'&&!state.error;
            if(capabilities===null){const response=await nativeFetch(base+'capabilities',{credentials:'same-origin',signal:lifetime.signal});if(!response.ok)throw Error();const data=await response.json();if(!Array.isArray(data.unavailable))throw Error();capabilities=new Set(data.unavailable);}
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
        if (u.origin!==location.origin || blocked.test(u.pathname)) { event.preventDefault();return; }
        const name=u.pathname.split('/').pop();
        if (['index.html','observatory.html','pose-fusion.html','viz.html'].includes(name)) {event.preventDefault();location.href=base+'ui/'+name;}
    });
})();
