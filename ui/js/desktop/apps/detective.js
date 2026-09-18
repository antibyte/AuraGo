(function () {
    'use strict';
    const instances = new Map();
    const base = '/api/desktop/detective';
    const active = c => ['queued', 'running'].includes(c?.run?.status);
    const key = () => window.crypto.randomUUID();
    function render(host, windowId, context) {
        dispose(windowId);
        const ctx = context || {}, v = window.DetectiveViews, e = v.esc;
        const tr = k => { const text=ctx.t?.('desktop.detective_' + k); return !text || text === 'desktop.detective_' + k ? k : text; };
        const controller = new AbortController();
        const state = {host, id: windowId, ctx, controller, timer: null, cases: [], current: null, events: [], tab: 'report', revision: 0, busy: false, disposed: false, fresh: false};
        instances.set(windowId, state);
        const request = async (path, options = {}) => {
            const opts = {...options, signal: controller.signal};
            if (opts.body) { opts.headers = {'Content-Type': 'application/json'}; opts.body = JSON.stringify(opts.body); }
            if (!ctx.api) throw new Error(tr('unavailable'));
            return ctx.api(base + path, opts);
        };
        const error = err => { if (!state.disposed && err.name !== 'AbortError') { const node = host.querySelector('.dt-error'); if (node) {node.hidden = false; node.textContent = err.message || tr('error');} } };
        function draw() {
            if (state.disposed) return;
            const c = state.current, caps = state.caps || {}, ro = ctx.readonly || caps.read_only || !caps.enabled || !caps.ready;
            host.innerHTML = `<div class="dt-app"><header class="dt-header"><div><b>Detective</b><span>${e(tr('subtitle'))}</span></div><button data-do="new" ${ro ? 'disabled' : ''}>+ ${e(tr('new'))}</button></header><div class="dt-error" role="alert" hidden></div><div class="dt-layout"><aside aria-label="${e(tr('cases'))}">${state.cases.map(item => `<button class="dt-case ${item.id === c?.id ? 'is-selected' : ''}" data-case="${e(item.id)}" aria-pressed="${item.id === c?.id}"><b>${e(item.request.topic)}</b><small>${e(tr(item.run.status))} · ${e(tr(item.request.effort))}</small></button>`).join('') || `<p>${e(tr('no_cases'))}</p>`}</aside><main class="dt-main"></main></div></div>`;
            const main = host.querySelector('.dt-main');
            if (!c) {drawForm(main, caps, ro);return;}
            const r = c.run, usage = r.usage || {}, p = r.profile || {}, revisions = c.reports || [];
            const chosen = state.revision ? revisions.find(x => x.revision === state.revision) : revisions.at(-1);
            main.innerHTML = `<section class="dt-status" aria-live="polite"><h2>${e(c.request.topic)}</h2><div><strong>${e(tr(r.status))}</strong><span>${e(tr(r.phase || 'research'))}</span><span>${Math.floor((usage.active_ms || 0)/60000)} / ${Math.ceil((p.seconds || 0)/60)} min · ${usage.tools || 0}/${p.tools || 0} ${e(tr('tools'))} · ${usage.pages || 0} ${e(tr('sources'))}</span></div>${r.reason ? `<p>${e(tr(r.reason) === 'desktop.detective_' + r.reason ? r.reason : tr(r.reason))}</p>` : ''}<div class="dt-actions">${active(c) ? `<button data-do="finish" ${ro ? 'disabled' : ''}>${e(tr('finish'))}</button><button data-do="stop" ${ro ? 'disabled' : ''}>${e(tr('stop'))}</button>` : `${r.status === 'draft' ? `<button data-do="start" ${ro ? 'disabled' : ''}>${e(tr('start'))}</button>` : r.status !== 'completed' ? `<button data-do="continue" ${ro ? 'disabled' : ''}>${e(tr('continue'))}</button>` : ''}<select class="dt-effort" aria-label="${e(tr('effort'))}">${['quick','normal','maximum'].map(x=>`<option value="${x}" ${c.request.effort===x?'selected':''}>${e(tr(x))}</option>`).join('')}</select><button data-do="deepen" ${ro ? 'disabled' : ''}>${e(tr('deepen'))}</button><button data-do="delete" ${ro ? 'disabled' : ''}>${e(tr('delete'))}</button>`}</div>${r.status === 'waiting_for_user' ? `<label>${e(tr('answer'))}<textarea class="dt-answer" maxlength="8000"></textarea></label>` : ''}</section><nav class="dt-tabs" aria-label="${e(tr('views'))}">${['report','sources','activity'].map(tab => `<button data-tab="${tab}" aria-pressed="${state.tab === tab}">${e(tr(tab))}${tab==='sources'?` (${(c.sources||[]).length})`:''}</button>`).join('')}</nav><div class="dt-export">${revisions.length ? `<label>${e(tr('revision'))}<select class="dt-revision">${revisions.map(x => `<option value="${x.revision}" ${x===chosen?'selected':''}>${x.revision}${x.partial?' · '+e(tr('partial')):''}</option>`).join('')}</select></label>${['md','pdf','docx'].map(format => `<a class="dt-download" href="${base}/cases/${encodeURIComponent(c.id)}/export?revision=${chosen.revision}&format=${format}">${format==='md'?'Markdown':format==='docx'?'Word':'PDF'}</a>`).join('')}<button data-do="autor" ${ro ? 'disabled' : ''}>${e(tr('autor'))}</button>` : ''}</div><div class="dt-content">${state.tab==='report'?v.report(chosen,tr):state.tab==='sources'?v.sources(c.sources,tr):v.activity(state.events,tr)}</div>`;
        }
        function drawForm(main, caps, ro) {
            main.innerHTML = `<form class="dt-form"><h1>${e(tr('new'))}</h1><p>${e(tr('intro'))}</p><label>${e(tr('topic'))}<textarea name="topic" required maxlength="8000" rows="3"></textarea></label><fieldset><legend>${e(tr('effort'))}</legend><div class="dt-profiles">${['quick','normal','maximum'].map(level => {const p = caps.profiles?.[level] || {}; return `<label><input type="radio" name="effort" value="${level}" ${level==='normal'?'checked':''}><b>${e(tr(level))}</b><small>${Math.ceil((p.seconds||0)/60)} min · ${p.tools||0} ${e(tr('tools'))}</small></label>`;}).join('')}</div></fieldset><details><summary>${e(tr('advanced'))}</summary><label>${e(tr('scope'))}<textarea name="scope" maxlength="8000"></textarea></label><label>${e(tr('language'))}<input name="language" value="${e(document.documentElement.lang || 'de')}" maxlength="30"></label><label>${e(tr('source_urls'))}<textarea name="source_urls" rows="2"></textarea></label><label>${e(tr('provider'))}<select name="provider_id">${(caps.providers||[]).map(p=>`<option value="${e(p.id)}" ${p.id===caps.provider_id?'selected':''}>${e(p.name||p.id)} · ${e(p.model)}</option>`).join('')}</select></label><label>${e(tr('model'))}<input name="model" placeholder="${e(caps.model||'')}" maxlength="200"></label>${(caps.private_sources||[]).length ? `<fieldset><legend>${e(tr('private_sources'))}</legend>${caps.private_sources.map(name=>`<label><input type="checkbox" name="private_sources" value="${e(name)}">${e(name)}</label>`).join('')}</fieldset>`:''}</details><p class="dt-capabilities">${e(tr('available'))}: ${e((caps.tools||[]).filter(x=>!['detective_report','invoke_tool','discover_tools','execute_skill','list_skills'].includes(x)).join(', '))}</p>${ro ? `<p role="status">${e(tr('unavailable'))}</p>`:''}<button type="submit" ${ro || state.busy ? 'disabled' : ''}>${e(tr('start'))}</button></form>`;
        }
        async function refresh(redraw = true) {
            const list = await request('/cases'); if(state.disposed)return;
            state.cases = list.cases || [];
            if (state.current) {
                const id = state.current.id;
                const c = await request('/cases/' + id);
                const data = await request('/cases/' + id + '/events?after=' + (state.events.at(-1)?.id || 0));
                if(state.disposed || state.current?.id!==id)return;
                state.current = c;state.events = [...state.events, ...(data.events || [])].slice(-500);
            }
            // Keep text selection, source disclosure, scroll and input focus stable.
            if (redraw) draw();
        }
        async function poll() {
            if(state.disposed)return;
            try {
                const wasActive = active(state.current), previous = state.current?.updated_at;
                await refresh(false);
                if (wasActive && previous !== state.current?.updated_at && !host.querySelector('textarea:focus,input:focus,select:focus') && state.tab !== 'sources') {
                    const scroll = host.querySelector('.dt-content')?.scrollTop || 0;
                    draw(); const node=host.querySelector('.dt-content');if(node)node.scrollTop=scroll;
                }
            } catch(err){error(err);}
            if(!state.disposed)state.timer=setTimeout(poll,3000);
        }
        host.addEventListener('submit', async ev => {
            if(!ev.target.matches('.dt-form'))return;ev.preventDefault();if(state.busy)return;
            state.busy=true;const data=new FormData(ev.target);ev.target.querySelector('[type=submit]').disabled=true;
            try {
                const body=Object.fromEntries(data);body.source_urls=String(body.source_urls||'').split(/\s+/).filter(Boolean);body.private_sources=data.getAll('private_sources');
                const c=await request('/cases',{method:'POST',body});state.current=c;state.events=[];state.fresh=false;
                await request('/cases/'+c.id+'/run',{method:'POST',body:{action:'start',idempotency_key:key()}});await refresh();
            } catch(err){error(err);ev.target.querySelector('[type=submit]').disabled=false;}finally{state.busy=false;}
        },{signal:controller.signal});
        host.addEventListener('change',ev=>{if(ev.target.matches('.dt-revision')){state.revision=Number(ev.target.value);draw();}},{signal:controller.signal});
        host.addEventListener('click',async ev=>{
            const el=ev.target.closest('button,a[data-ref],a.dt-download');if(!el||el.disabled||state.busy)return;
            if (el.matches('.dt-download')) {
                ev.preventDefault(); state.busy=true;
                try {
                    const response=await fetch(el.href,{signal:controller.signal,credentials:'same-origin'});
                    if(!response.ok){const problem=await response.json();throw new Error(problem.error||tr('error'));}
                    const blob=await response.blob();if(state.disposed)return;
                    const url=URL.createObjectURL(blob),link=document.createElement('a');
                    link.href=url;link.download='detective-report.'+(new URL(el.href).searchParams.get('format')||'md');
                    link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
                }catch(err){error(err);}finally{state.busy=false;}
                return;
            }
            if(el.dataset.tab){state.tab=el.dataset.tab;draw();return;}
            if(el.dataset.ref){ev.preventDefault();state.tab='sources';draw();const index=(state.current.sources||[]).findIndex(s=>s.id===el.dataset.ref);const target=host.querySelectorAll('.dt-source')[index];if(target){target.open=true;target.scrollIntoView({block:'nearest'});}return;}
            try{
                if(el.dataset.case){state.current=await request('/cases/'+el.dataset.case);state.events=[];state.revision=0;await refresh();return;}
                const action=el.dataset.do;
                if(action==='new'){state.current=null;state.fresh=true;draw();host.querySelector('[name=topic]')?.focus();return;}
                if(!action||!state.current)return;
                state.busy=true;
                if(action==='delete'){
                    const accepted=ctx.confirmDialog ? await ctx.confirmDialog(tr('delete'),tr('delete_confirm')) : window.confirm(tr('delete_confirm'));
                    if(!accepted)return;await request('/cases/'+state.current.id,{method:'DELETE'});state.current=null;
                }else if(action==='autor'){
                    const revision=state.revision||state.current.reports.at(-1).revision;
                    const result=await request('/cases/'+state.current.id+'/autor?revision='+revision,{method:'POST'});ctx.openApp?.('writer',{path:result.path});
                }else{
                    const body={action,idempotency_key:key(),effort:host.querySelector('.dt-effort')?.value||state.current.request.effort};
                    if(state.current.run.status==='waiting_for_user')body.answer=host.querySelector('.dt-answer')?.value||'';
                    await request('/cases/'+state.current.id+'/run',{method:'POST',body});
                }
                await refresh();
            }catch(err){error(err);}finally{state.busy=false;}
        },{signal:controller.signal});
        (async()=>{try{state.caps=await request('/capabilities');await refresh(false);if(!state.fresh&&state.cases.length){state.current=state.cases[0];await refresh(false);}draw();poll();}catch(err){draw();error(err);}})();
    }
    function dispose(id){const s=instances.get(id);if(!s)return;s.disposed=true;clearTimeout(s.timer);s.controller.abort();instances.delete(id);}
    window.DetectiveApp={render,dispose};
})();
