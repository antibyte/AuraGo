let telephoneAgentState = null;
let telephoneAgentCatalog = null;
let telephoneAgentSaved = '';
let telephoneAgentInherited = {};
let telephoneAgentBlockers = [];
let telephoneAgentToolQuery = '';

function taEsc(value) {
    return escapeHtml(String(value == null ? '' : value));
}

function taNormalize(payload) {
    const source = payload && payload.config ? payload.config : {};
    const voice = source.voice || {};
    const behavior = voice.behavior || {};
    return {
        inbound_route: source.inbound_route || 'agent',
        auto_answer_delay_ms: Number(source.auto_answer_delay_ms ?? 1000),
        voice: {
            backend: voice.backend || 'classic',
            realtime_profile_id: voice.realtime_profile_id || '',
            agent_provider_id: voice.agent_provider_id || '',
            classic: {
                asr_provider_id: voice.classic?.asr_provider_id || '',
                asr_mode: voice.classic?.asr_mode || 'whisper',
                tts_provider: voice.classic?.tts_provider || ''
            },
            behavior: {
                greeting_enabled: behavior.greeting_enabled !== false,
                greeting: behavior.greeting || '',
                purpose: behavior.purpose || '',
                speaking_style: behavior.speaking_style || '',
                additional_prohibitions: behavior.additional_prohibitions || '',
                unavailable_request_behavior: behavior.unavailable_request_behavior || 'explain',
                failure_message: behavior.failure_message || '',
                goodbye_message: behavior.goodbye_message || ''
            },
            language: voice.language || 'auto',
            allowed_tools: Array.isArray(voice.allowed_tools) ? voice.allowed_tools.slice().sort() : [],
            persist_transcripts: !!voice.persist_transcripts,
            max_call_duration_seconds: Number(voice.max_call_duration_seconds || 3600),
			idle_timeout_seconds: Number(voice.idle_timeout_seconds || 120),
			turn_timeout_seconds: Number(voice.turn_timeout_seconds || 60),
			max_response_chars: Number(voice.max_response_chars || 1200)
        }
    };
}

function taComparable(value) {
    return JSON.stringify(value || {});
}

function taOptions(items, selected, emptyKey) {
    const options = [`<option value="">${taEsc(t(emptyKey))}</option>`];
    (items || []).forEach(item => {
        const label = `${item.name || item.id}${item.model ? ` · ${item.model}` : ''}${item.ready ? '' : ` · ${t('config.telephone_agent.not_ready')}`}`;
		const reason = item.ready || !item.reason ? '' : ` · ${item.reason}`;
		options.push(`<option value="${taEsc(item.id)}" ${item.id === selected ? 'selected' : ''} ${item.ready ? '' : 'disabled'}>${taEsc(label + reason)}</option>`);
    });
    return options.join('');
}

function taLabel(key, inheritedKey) {
	const suffix = telephoneAgentInherited?.[inheritedKey] ? ` · ${t('config.telephone_agent.inherited')}` : '';
	return taEsc(t(key) + suffix);
}

function taBlockerMarkup() {
    if (!telephoneAgentBlockers.length) {
        return `<div class="ta-status-card is-ready"><strong>${taEsc(t('config.telephone_agent.ready'))}</strong><span>${taEsc(t('config.telephone_agent.ready_desc'))}</span></div>`;
    }
    const items = telephoneAgentBlockers.map(code => `<li>${taEsc(t(`config.telephone_agent.blocker.${code}`))}</li>`).join('');
    return `<div class="ta-status-card is-blocked">
        <div><strong>${taEsc(t('config.telephone_agent.blocked'))}</strong><ul>${items}</ul></div>
        <button type="button" class="btn btn-secondary" data-ta-action="open-sip">${taEsc(t('config.telephone_agent.open_sip'))}</button>
    </div>`;
}

function taToolMarkup(selected) {
    const tools = Array.isArray(telephoneAgentCatalog?.tools) ? telephoneAgentCatalog.tools : [];
    const query = telephoneAgentToolQuery.trim().toLowerCase();
    const visible = tools.filter(tool => !query || `${tool.id} ${tool.description || ''}`.toLowerCase().includes(query));
    if (!visible.length) return `<p class="ta-empty">${taEsc(t('config.telephone_agent.no_tools'))}</p>`;
    const selectedSet = new Set(selected || []);
    return visible.map(tool => `<label class="ta-tool">
        <input type="checkbox" value="${taEsc(tool.id)}" data-ta-tool ${selectedSet.has(tool.id) ? 'checked' : ''}>
        <span><strong>${taEsc(tool.id)}</strong><small>${taEsc(tool.description || '')}</small></span>
    </label>`).join('');
}

function taRender() {
    const c = telephoneAgentState;
    const providers = telephoneAgentCatalog?.providers || [];
    const profiles = telephoneAgentCatalog?.realtime_profiles || [];
    const ttsProviders = telephoneAgentCatalog?.tts_providers || [];
    const classic = c.voice.backend === 'classic';
    document.getElementById('content').innerHTML = `<section class="cfg-section active telephone-agent-section">
        <div class="section-header">${taEsc(t('config.telephone_agent.title'))}</div>
        <div class="section-desc">${taEsc(t('config.telephone_agent.desc'))}</div>
        ${taBlockerMarkup()}

        <div class="ta-grid">
            <fieldset class="ta-card">
                <legend>${taEsc(t('config.telephone_agent.routing'))}</legend>
                <label>${taEsc(t('config.sip.inbound_route'))}
                    <select class="field-select" data-ta="inbound_route">
                        <option value="agent" ${c.inbound_route === 'agent' ? 'selected' : ''}>${taEsc(t('config.sip.route_agent'))}</option>
                        <option value="manual" ${c.inbound_route === 'manual' ? 'selected' : ''}>${taEsc(t('config.sip.route_manual'))}</option>
                        <option value="reject" ${c.inbound_route === 'reject' ? 'selected' : ''}>${taEsc(t('config.sip.route_reject'))}</option>
                    </select>
                </label>
                <label>${taEsc(t('config.sip.auto_answer_delay'))}<input class="field-input" type="number" min="0" max="60000" data-ta="auto_answer_delay_ms" value="${c.auto_answer_delay_ms}"></label>
            </fieldset>

            <fieldset class="ta-card">
                <legend>${taEsc(t('config.telephone_agent.pipeline'))}</legend>
                <label>${taEsc(t('config.sip.voice_backend'))}
                    <select class="field-select" data-ta="voice.backend">
                        <option value="classic" ${classic ? 'selected' : ''}>${taEsc(t('config.sip.backend_classic'))}</option>
                        <option value="gemini_live" ${classic ? '' : 'selected'}>Gemini Live</option>
                    </select>
                </label>
				<label>${taLabel('config.telephone_agent.agent_provider', 'agent_provider_id')}<select class="field-select" data-ta="voice.agent_provider_id">${taOptions(providers, c.voice.agent_provider_id, 'config.telephone_agent.choose_provider')}</select></label>
                ${classic ? `
					<label>${taLabel('config.telephone_agent.asr_provider', 'asr_provider_id')}<select class="field-select" data-ta="voice.classic.asr_provider_id">${taOptions(providers, c.voice.classic.asr_provider_id, 'config.telephone_agent.choose_provider')}</select></label>
					<label>${taLabel('config.telephone_agent.asr_mode', 'asr_mode')}<select class="field-select" data-ta="voice.classic.asr_mode">
                        ${(telephoneAgentCatalog?.asr_modes || []).map(mode => `<option value="${taEsc(mode)}" ${mode === c.voice.classic.asr_mode ? 'selected' : ''}>${taEsc(mode)}</option>`).join('')}
                    </select></label>
					<label>${taLabel('config.telephone_agent.tts_provider', 'tts_provider')}<select class="field-select" data-ta="voice.classic.tts_provider">${taOptions(ttsProviders, c.voice.classic.tts_provider, 'config.telephone_agent.choose_provider')}</select></label>
                ` : `<label>${taEsc(t('config.sip.realtime_profile'))}<select class="field-select" data-ta="voice.realtime_profile_id">${taOptions(profiles, c.voice.realtime_profile_id, 'config.telephone_agent.choose_profile')}</select></label>`}
                <label>${taEsc(t('config.sip.language'))}
                    <select class="field-select" data-ta="voice.language">
                        ${['auto', 'de', 'en', 'fr', 'es', 'it', 'nl', 'pt', 'pl', 'cs', 'da', 'no', 'sv', 'el', 'hi', 'ja', 'zh'].map(language => `<option value="${language}" ${language === c.voice.language ? 'selected' : ''}>${language}</option>`).join('')}
                    </select>
                </label>
            </fieldset>

            <fieldset class="ta-card ta-card-wide">
                <legend>${taEsc(t('config.telephone_agent.behavior'))}</legend>
                <label class="ta-check"><input type="checkbox" data-ta="voice.behavior.greeting_enabled" ${c.voice.behavior.greeting_enabled ? 'checked' : ''}>${taEsc(t('config.telephone_agent.greeting_enabled'))}</label>
                <label>${taEsc(t('config.telephone_agent.greeting'))}<textarea class="field-textarea" maxlength="500" data-ta="voice.behavior.greeting">${taEsc(c.voice.behavior.greeting)}</textarea></label>
                <label>${taEsc(t('config.telephone_agent.purpose'))}<textarea class="field-textarea" maxlength="8000" data-ta="voice.behavior.purpose">${taEsc(c.voice.behavior.purpose)}</textarea></label>
                <label>${taEsc(t('config.telephone_agent.style'))}<textarea class="field-textarea" maxlength="8000" data-ta="voice.behavior.speaking_style">${taEsc(c.voice.behavior.speaking_style)}</textarea></label>
                <label>${taEsc(t('config.telephone_agent.prohibitions'))}<textarea class="field-textarea" maxlength="8000" data-ta="voice.behavior.additional_prohibitions">${taEsc(c.voice.behavior.additional_prohibitions)}</textarea></label>
                <label>${taEsc(t('config.telephone_agent.unavailable'))}<select class="field-select" data-ta="voice.behavior.unavailable_request_behavior">
                    <option value="explain" ${c.voice.behavior.unavailable_request_behavior === 'explain' ? 'selected' : ''}>${taEsc(t('config.telephone_agent.explain'))}</option>
                    <option value="explain_and_end" ${c.voice.behavior.unavailable_request_behavior === 'explain_and_end' ? 'selected' : ''}>${taEsc(t('config.telephone_agent.explain_end'))}</option>
                </select></label>
                <label>${taEsc(t('config.telephone_agent.failure_message'))}<textarea class="field-textarea" maxlength="500" data-ta="voice.behavior.failure_message">${taEsc(c.voice.behavior.failure_message)}</textarea></label>
                <label>${taEsc(t('config.telephone_agent.goodbye'))}<textarea class="field-textarea" maxlength="500" data-ta="voice.behavior.goodbye_message">${taEsc(c.voice.behavior.goodbye_message)}</textarea></label>
            </fieldset>

            <fieldset class="ta-card">
                <legend>${taEsc(t('config.telephone_agent.tools'))}</legend>
                <p class="ta-hint">${taEsc(t('config.telephone_agent.tools_desc'))}</p>
                <input class="field-input" type="search" data-ta-search placeholder="${taEsc(t('config.telephone_agent.search_tools'))}" value="${taEsc(telephoneAgentToolQuery)}">
                <div class="ta-tools" data-ta-tools>${taToolMarkup(c.voice.allowed_tools)}</div>
            </fieldset>

            <fieldset class="ta-card">
                <legend>${taEsc(t('config.telephone_agent.privacy_limits'))}</legend>
                <label class="ta-check"><input type="checkbox" data-ta="voice.persist_transcripts" ${c.voice.persist_transcripts ? 'checked' : ''}>${taEsc(t('config.sip.persist_transcripts'))}</label>
                <label>${taEsc(t('config.sip.max_duration'))}<input class="field-input" type="number" min="30" max="86400" data-ta="voice.max_call_duration_seconds" value="${c.voice.max_call_duration_seconds}"></label>
				<label>${taEsc(t('config.telephone_agent.idle_timeout'))}<input class="field-input" type="number" min="15" max="3600" data-ta="voice.idle_timeout_seconds" value="${c.voice.idle_timeout_seconds}"></label>
				<label>${taEsc(t('config.telephone_agent.turn_timeout'))}<input class="field-input" type="number" min="15" max="300" data-ta="voice.turn_timeout_seconds" value="${c.voice.turn_timeout_seconds}"></label>
				<label>${taEsc(t('config.telephone_agent.max_response_chars'))}<input class="field-input" type="number" min="200" max="5000" data-ta="voice.max_response_chars" value="${c.voice.max_response_chars}"></label>
                <p class="ta-hint">${taEsc(t('config.telephone_agent.snapshot_note'))}</p>
            </fieldset>
        </div>
        <div class="ta-actions">
            <span id="telephone-agent-status" role="status" aria-live="polite"></span>
            <button type="button" class="btn btn-secondary" data-ta-action="preflight">${taEsc(t('config.telephone_agent.preflight'))}</button>
            <label class="ta-check"><input type="checkbox" data-ta-live-confirm>${taEsc(t('config.telephone_agent.confirm_live'))}</label>
            <button type="button" class="btn btn-secondary" data-ta-action="live-test">${taEsc(t('config.telephone_agent.live_test'))}</button>
            <button type="button" class="btn-save" data-ta-action="save">${taEsc(t('config.sip.save'))}</button>
        </div>
    </section>`;
    taBind();
}

function taAssign(target, path, value) {
    const parts = path.split('.');
    let cursor = target;
    parts.forEach((part, index) => {
        if (index === parts.length - 1) cursor[part] = value;
        else cursor = cursor[part] = cursor[part] || {};
    });
}

function taRead() {
    const next = JSON.parse(JSON.stringify(telephoneAgentState));
    document.querySelectorAll('[data-ta]').forEach(input => {
        let value = input.type === 'checkbox' ? input.checked : input.value;
        if (input.type === 'number') value = Number(value);
        taAssign(next, input.dataset.ta, value);
    });
    const selectedTools = new Set(Array.isArray(next.voice.allowed_tools) ? next.voice.allowed_tools : []);
    document.querySelectorAll('[data-ta-tool]').forEach(input => {
        if (input.checked) selectedTools.add(input.value);
        else selectedTools.delete(input.value);
    });
    next.voice.allowed_tools = Array.from(selectedTools).sort();
    return next;
}

function taMarkDirty() {
    if (typeof setDirty === 'function') setDirty(true);
}

function taBind() {
    document.querySelectorAll('[data-ta]').forEach(input => {
        input.addEventListener('input', taMarkDirty);
        input.addEventListener('change', () => {
            const pipelineChanged = input.dataset.ta === 'voice.backend';
            telephoneAgentState = taRead();
            taMarkDirty();
            if (pipelineChanged) taRender();
        });
    });
    document.querySelectorAll('[data-ta-tool]').forEach(input => input.addEventListener('change', () => {
        telephoneAgentState = taRead();
        taMarkDirty();
    }));
    document.querySelector('[data-ta-search]')?.addEventListener('input', event => {
        telephoneAgentState = taRead();
        telephoneAgentToolQuery = event.target.value;
        const list = document.querySelector('[data-ta-tools]');
        if (list) {
            list.innerHTML = taToolMarkup(telephoneAgentState.voice.allowed_tools);
            list.querySelectorAll('[data-ta-tool]').forEach(input => input.addEventListener('change', () => {
                telephoneAgentState = taRead();
                taMarkDirty();
            }));
        }
    });
    document.querySelectorAll('[data-ta-action]').forEach(button => button.addEventListener('click', async () => {
        const action = button.dataset.taAction;
        if (action === 'open-sip') {
            await navigateToConfigSection('sip');
        } else if (action === 'save') {
            await taSave();
        } else if (action === 'preflight') {
            await taTest(false);
        } else if (action === 'live-test') {
            await taTest(true);
        }
    }));
}

async function taRequest(path, options) {
    const response = await fetch(path, Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {}));
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.error || body.message || `HTTP ${response.status}`);
    return body;
}

async function taSave() {
    const status = document.getElementById('telephone-agent-status');
    try {
        const payload = taRead();
        if (status) status.textContent = t('config.sip.saving');
        const response = await taRequest('/api/sip/agent', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        telephoneAgentState = taNormalize(response);
        telephoneAgentBlockers = response.blockers || [];
        telephoneAgentInherited = response.inherited || {};
        telephoneAgentSaved = taComparable(telephoneAgentState);
        taRender();
        const renderedStatus = document.getElementById('telephone-agent-status');
        if (renderedStatus) renderedStatus.textContent = t('config.sip.saved');
        if (typeof setDirty === 'function') setDirty(false);
        return true;
    } catch (error) {
        if (status) status.textContent = error.message;
        return false;
    }
}

async function taTest(live) {
    const status = document.getElementById('telephone-agent-status');
    if (taComparable(taRead()) !== telephoneAgentSaved) {
        if (status) status.textContent = t('config.telephone_agent.save_before_test');
        return;
    }
    const confirmed = !!document.querySelector('[data-ta-live-confirm]')?.checked;
    if (live && !confirmed) {
        if (status) status.textContent = t('config.telephone_agent.confirm_required');
        return;
    }
    try {
        if (status) status.textContent = t('config.telephone_agent.testing');
        const result = await taRequest('/api/sip/agent/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ live, confirm_live: confirmed })
        });
        if (status) status.textContent = result.mode === 'live' ? t('config.telephone_agent.live_ok') : t('config.telephone_agent.preflight_ok');
    } catch (error) {
        if (status) status.textContent = error.message;
    }
}

window.telephoneAgentHasUnsavedChanges = function () {
    if (!telephoneAgentState) return false;
    if (document.querySelector('[data-ta]')) return taComparable(taRead()) !== telephoneAgentSaved;
    return taComparable(telephoneAgentState) !== telephoneAgentSaved;
};

window.telephoneAgentSaveUnsaved = taSave;
window.telephoneAgentDiscardUnsaved = function () {
    if (!telephoneAgentSaved) return;
    telephoneAgentState = JSON.parse(telephoneAgentSaved);
    if (document.querySelector('.telephone-agent-section')) taRender();
};

async function renderTelephoneAgentSection() {
    document.getElementById('content').innerHTML = `<section class="cfg-section active telephone-agent-section"><div class="section-header">${taEsc(t('config.telephone_agent.title'))}</div><div class="loading-state">${taEsc(t('config.telephone_agent.loading'))}</div></section>`;
    try {
        const [payload, catalog] = await Promise.all([
            taRequest('/api/sip/agent'),
            taRequest('/api/sip/agent/catalog')
        ]);
        telephoneAgentState = taNormalize(payload);
        telephoneAgentCatalog = catalog;
        telephoneAgentInherited = payload.inherited || {};
        telephoneAgentBlockers = payload.blockers || [];
        telephoneAgentSaved = taComparable(telephoneAgentState);
        taRender();
    } catch (error) {
        document.getElementById('content').innerHTML = `<section class="cfg-section active telephone-agent-section"><div class="section-header">${taEsc(t('config.telephone_agent.title'))}</div><div class="ta-status-card is-blocked" role="alert">${taEsc(error.message)}</div></section>`;
    }
}
