// Administrator action rules and exact, one-shot action decisions.
(function () {
    'use strict';
    let dictionary;
    let catalog;
    let showing = false;
    const queue = [];
    const visible = new Set();
    const languages = new Set(['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']);

    async function words() {
        if (!dictionary) {
            let language = String(window.SYSTEM_LANG || document.documentElement.lang || 'en').toLowerCase().split('-')[0];
            if (!languages.has(language)) language = 'en';
            dictionary = fetch('/lang/action_policy/' + language + '.json').then(async response => {
                if (!response.ok) throw new Error('locale_unavailable');
                const values = await response.json();
                return Object.fromEntries(Object.entries(values).map(([key, value]) => [key.replace(/^action_policy\./, ''), value]));
            }).catch(error => { dictionary = null; throw error; });
        }
        return dictionary;
    }
    function element(tag, text, parent, className) {
        const node = document.createElement(tag);
        if (text != null) node.textContent = text;
        if (className) node.className = className;
        if (parent) parent.appendChild(node);
        return node;
    }
    function unwrap(value) {
        if (value && value.detail) {
            try { return typeof value.detail === 'string' ? JSON.parse(value.detail) : value.detail; } catch (_) { return null; }
        }
        return value;
    }
    async function show(value) {
        const action = unwrap(value);
        if (!action || !action.id || !action.session_id || visible.has(action.id)) return;
        visible.add(action.id);
        queue.push(action);
        if (showing) return;
        showing = true;
        try {
            while (queue.length) await display(queue.shift());
        } finally { showing = false; }
    }
    async function display(item) {
        let text;
        try { text = await words(); } catch (_) { visible.delete(item.id); return; }
        if (Date.parse(item.expires_at) <= Date.now()) { visible.delete(item.id); return; }
        const dialog = element('dialog', null, document.body, 'action-policy-dialog');
        const title = element('h2', text.approval_title, dialog);
        title.id = 'action-approval-title';
        dialog.setAttribute('aria-labelledby', title.id);
        element('p', text.approval_help, dialog);
        const details = element('dl', null, dialog);
        for (const [label, value] of [[text.tool,item.action.tool],[text.operation,item.action.operation],[text.target,item.action.target]]) {
            element('dt', label, details); element('dd', value, details);
        }
        element('pre', item.arguments_preview || '', dialog);
        const status = element('p', text.expires + ': ' + new Date(item.expires_at).toLocaleTimeString(), dialog, 'action-policy-state');
        status.setAttribute('role', 'status');
        const controls = element('div', null, dialog, 'action-policy-actions');
        const reject = element('button', text.reject, controls);
        const approve = element('button', text.approve, controls);
        const close = element('button', text.close, controls);
        approve.type = reject.type = close.type = 'button';
        let busy = false;
        const expire = () => {
            if (Date.now() < Date.parse(item.expires_at) || busy) return;
            approve.disabled = reject.disabled = true;
            status.textContent = text.expired;
        };
        const timer = setInterval(expire, 1000);
        async function decide(decision) {
            if (busy || Date.now() >= Date.parse(item.expires_at)) { expire(); return; }
            busy = true;
            approve.disabled = reject.disabled = true;
            status.textContent = text.working;
            try {
                const response = await fetch('/api/agent/actions', {
                    method: 'POST', headers: {'Content-Type':'application/json','X-Session-ID':item.session_id},
                    body: JSON.stringify({id:item.id,decision})
                });
                if (!response.ok) {
                    status.textContent = response.status === 409 ? text.expired : response.status === 403 ? text.admin_only : text.failed;
                    return;
                }
                const result = await response.json();
                status.textContent = (result.status === 'success' ? text.done : text.denied) + (result.result ? '\n' + result.result : '');
            } catch (_) { status.textContent = text.unknown; }
            finally { busy = false; }
        }
        approve.onclick = () => decide('approve');
        reject.onclick = () => decide('reject');
        await new Promise(resolve => {
            dialog.addEventListener('close', resolve, {once:true});
            close.onclick = () => dialog.close();
            dialog.showModal();
            reject.focus();
        });
        clearInterval(timer);
        visible.delete(item.id);
        dialog.remove();
    }
    async function load(sessionID) {
        try {
            const response = await fetch('/api/agent/actions?' + (sessionID === '*' ? 'all=1' : 'session_id=' + encodeURIComponent(sessionID || 'default')));
            if (!response.ok) return;
            const result = await response.json();
            for (const pending of result.pending || []) show(pending);
        } catch (_) { /* Reconnection or the next explicit refresh can recover pending decisions. */ }
    }
    async function tools() {
        if (!catalog) catalog = fetch('/api/agent/action-policy/catalog').then(async response => {
            if (!response.ok) throw new Error('admin_required');
            return response.json();
        }).catch(error => { catalog = null; throw error; });
        return catalog;
    }
    async function mountSettings(host, config) {
        if (!host || host.querySelector('.action-policy-panel')) return;
        const text = await words();
        if (!host.isConnected) return;
        const runLabels = Object.fromEntries(['interactive','mission','flow','background','coagent','maintenance'].map(run => [run, text['runs.' + run]]));
        const panel = element('section', null, host, 'action-policy-panel');
        element('h3', text.title, panel);
        element('p', text.description, panel);
        const cfg = config.agent = config.agent || {};
        const policy = cfg.action_policy = cfg.action_policy || {enabled:true,allow:[],deny:[]};
        function changed() {
            if (typeof setNestedValue === 'function') setNestedValue(config, 'agent.action_policy', policy);
            if (window.AuraConfigState) {
                window.AuraConfigState.markAtomic('agent.action_policy');
                window.AuraConfigState.set('agent.action_policy', policy);
            }
            if (typeof setDirty === 'function') setDirty(true);
        }
        const enabledLabel = element('label', null, panel);
        const enabled = element('input', null, enabledLabel);
        enabled.type = 'checkbox'; enabled.checked = policy.enabled !== false;
        element('span', text.enabled, enabledLabel);
        enabled.onchange = () => { policy.enabled = enabled.checked; changed(); };
        const status = element('p', '', panel, 'action-policy-state'); status.setAttribute('role','status');
        let schemas;
        try { schemas = await tools(); } catch (_) { status.textContent = text.admin_only; enabled.disabled = true; return; }
        if (!panel.isConnected) return;
        const functions = schemas.map(schema => schema.function).filter(Boolean).sort((a,b) => a.name.localeCompare(b.name));
        function field(row, label, value, options, update, labels) {
            const wrapper = element('label', label, row);
            const input = element(options ? 'select' : 'input', null, wrapper);
            if (options) for (const option of Array.from(new Set([...options, value || '']))) { const item = element('option',labels && labels[option] || option || '*',input); item.value = option; }
            input.value = value || '';
            input.onchange = () => { update(input.value); changed(); };
            return input;
        }
        for (const effect of ['allow','deny']) {
            element('h4', text[effect], panel);
            const rows = element('div', null, panel);
            policy[effect] = Array.isArray(policy[effect]) ? policy[effect] : [];
            const render = () => {
                rows.replaceChildren();
                policy[effect].forEach((rule,index) => {
                    const row = element('div',null,rows,'action-policy-rule');
                    field(row,text.tool,rule.tool,['*',...functions.map(fn => fn.name)],value => { rule.tool = value; render(); });
                    const schema = functions.find(fn => fn.name === rule.tool);
                    const properties = schema && schema.parameters && schema.parameters.properties || {};
                    const operations = properties.operation && properties.operation.enum || properties.method && properties.method.enum || ['execute'];
                    field(row,text.operation,rule.operation,['*',...operations],value => { rule.operation=value; });
                    field(row,text.target,rule.target,null,value => { rule.target=value; });
                    field(row,text.run,rule.run_type,['','interactive','mission','flow','background','coagent','maintenance'],value => { rule.run_type=value; },runLabels);
                    field(row,text.scope,rule.scope_id,null,value => { rule.scope_id=value; });
                    const remove = element('button',text.remove,row); remove.type='button';
                    remove.onclick = () => { policy[effect].splice(index,1); changed(); render(); };
                });
            };
            render();
            const add = element('button',text.add,panel); add.type='button';
            add.onclick = () => { policy[effect].push({tool:'',operation:'execute',target:''}); changed(); render(); };
        }
        element('p',text.save_hint,panel);
        element('h4',text.preview,panel);
        const preview = element('div',null,panel,'action-policy-rule');
        let selected = functions[0] && functions[0].name || '';
        field(preview,text.tool,selected,functions.map(fn=>fn.name),value=>{selected=value;});
        let missionID='', flowID='';
        field(preview,runLabels.mission,missionID,null,value=>{missionID=value;});
        field(preview,runLabels.flow,flowID,null,value=>{flowID=value;});
        const argumentsLabel = element('label',text.arguments,panel);
        const argumentsInput = element('textarea',null,argumentsLabel); argumentsInput.value='{}'; argumentsInput.rows=3;
        const check = element('button',text.check,panel); check.type='button';
        check.onclick = async () => {
            check.disabled=true; status.textContent=text.working;
            try {
                const args = JSON.parse(argumentsInput.value);
                const response = await fetch('/api/agent/action-policy/preview',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({tool:selected,arguments:args,mission_id:missionID,flow_id:flowID})});
                if (!response.ok) throw new Error();
                const result=await response.json();
                status.textContent=(text[result.state] || text.failed)+'\n'+result.action.tool+' / '+result.action.operation+' / '+result.action.target;
            } catch (_) { status.textContent=text.failed; }
            finally { check.disabled=false; }
        };
        const refresh = element('button',text.pending,panel); refresh.type='button'; refresh.onclick=()=>load('*');
    }
    window.AuraActionPolicy = Object.freeze({show,load,mountSettings});
})();
