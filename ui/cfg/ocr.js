function renderOCRSection(section) {
    const tr = key => t('ocr.' + key);
    const data = configData.ocr || (configData.ocr = {enabled:false,agent_access:true,automatic_chat:true,automatic_indexing:true});
    const root = document.createElement('section'); root.className = 'cfg-section active';
    const controller = new AbortController(); let timer = null, pending = false;
    const node = (tag, cls, text, parent = root) => { const el = document.createElement(tag); el.className = cls; if (text) el.textContent = text; parent.append(el); return el; };
    node('h2','section-header','OCR');
    node('p','section-desc',tr('description'));
    for (const key of ['enabled','agent_access','automatic_chat','automatic_indexing']) {
        const group = node('label','field-group');
        const checked = key === 'enabled' ? data[key] === true : data[key] !== false;
        const input = node('button','toggle' + (checked ? ' on' : ''),null,group); input.type = 'button'; input.dataset.path = 'ocr.' + key;
        input.setAttribute('role','switch'); input.setAttribute('aria-checked',String(checked)); input.setAttribute('aria-label',tr(key));
        input.addEventListener('click', () => { toggleBool(input); update(); });
        node('span','field-label',tr(key),group);
    }
    const status = node('p','field-help',tr('loading')); status.setAttribute('role','status');
    const notice = node('p','field-help',tr('save_first'));
    const actions = node('div','cfg-actions');
    const buttons = {};
    for (const action of ['setup','start','stop','test']) {
        const button = node('button','btn btn-secondary',tr(action === 'start' ? 'runtime_start' : action),actions); button.type = 'button'; buttons[action] = button;
        button.addEventListener('click',async () => {
            if (pending || hasUnsavedConfigChanges()) return;
            pending = true; update();
            try {
                const response = await fetch('/api/ocr/' + action,{method:'POST',signal:controller.signal});
                if (!response.ok) throw new Error('ocr_request_failed');
                const payload = await response.json();
                if (!root.isConnected) return;
                status.textContent = payload.ok ? tr('test_ok') : tr('accepted');
            } catch (err) { if (err.name !== 'AbortError' && root.isConnected) status.textContent = tr('error'); }
            finally { pending = false; update(); }
        },{signal:controller.signal});
    }
    node('p','field-help',tr('limits'));
    const license = node('a','field-help',tr('license')); license.href = 'https://huggingface.co/datalab-to/indra-ocr/blob/bd02167fdf2005a19962cc15188f8496a2c87d3b/LICENSE'; license.target = '_blank'; license.rel = 'noopener';
    document.getElementById('content').replaceChildren(root);
    const update = () => { const dirty = hasUnsavedConfigChanges(); notice.hidden = !dirty; Object.values(buttons).forEach(button => { button.disabled = pending || dirty || !data.enabled; }); };
    root.addEventListener('change',update,{signal:controller.signal});
    attachChangeListeners(); update();
    async function refresh() {
        if (!root.isConnected || controller.signal.aborted) return;
        try {
            const response = await fetch('/api/ocr/status',{signal:controller.signal});
            if (!response.ok) throw new Error('status_failed');
            const payload = await response.json();
            if (root.isConnected && !pending) status.textContent = tr('state_' + payload.state);
        } catch (err) { if (root.isConnected && err.name !== 'AbortError') status.textContent = tr('error'); }
        update();
    }
    const dispose = () => { clearInterval(timer); controller.abort(); observer.disconnect(); };
    const observer = new MutationObserver(() => { if (!root.isConnected) dispose(); });
    observer.observe(document.body,{childList:true,subtree:true});
    window.addEventListener('cfg:section-leave',dispose,{once:true,signal:controller.signal});
    window.addEventListener('pagehide',dispose,{once:true,signal:controller.signal});
    timer = setInterval(refresh,2000); refresh();
}
