// cfg/speech_lab.js — optional local ASR/TTS orchestrator

let speechLabSection = null;
let speechLabStatus = null;
let speechLabCatalog = null;
let speechLabCapability = null;
let speechLabSuggestions = null;
let speechLabShowExperimental = false;
let speechLabProviders = [];
let speechLabProviderLoadFailed = false;

function speechLabEnsureData() {
    if (!configData.speech_lab) configData.speech_lab = {};
    const data = configData.speech_lab;
    if (!data.base_url) data.base_url = 'http://s2s-vulkan:8765';
    if (!data.language) data.language = 'de';
    delete data.voice;
    data.chat_llm_provider_id = String(data.chat_llm_provider_id || '').trim();
    if (!data.timeout_seconds) data.timeout_seconds = 60;
    return data;
}

async function renderSpeechLabSection(section) {
    if (section) speechLabSection = section;
    speechLabSection = speechLabSection || section;
    const data = speechLabEnsureData();
    await speechLabLoadProviders();
    let html = '<div class="cfg-section active" id="speech_lab">';
    html += '<div class="section-header">' + escapeHtml(speechLabSection.label) + '</div>';
    html += '<div class="section-desc">' + escapeHtml(speechLabSection.desc) + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info">' + escapeHtml(t('config.speech_lab.ownership_note')) + '</div>';
    html += speechLabToggle('speech_lab.enabled', data.enabled === true, 'config.speech_lab.enabled', 'config.speech_lab.enabled_help');
    html += speechLabToggle('speech_lab.sip_enabled', data.sip_enabled === true, 'config.speech_lab.sip_enabled', 'config.speech_lab.sip_enabled_help');
    html += speechLabToggle('speech_lab.chat_input_enabled', data.chat_input_enabled === true, 'config.speech_lab.chat_input_enabled', 'config.speech_lab.chat_input_enabled_help');
    html += speechLabToggle('speech_lab.chat_output_enabled', data.chat_output_enabled === true, 'config.speech_lab.chat_output_enabled', 'config.speech_lab.chat_output_enabled_help');
    html += '<div class="cfg-group-title cfg-group-title-top">' + escapeHtml(t('config.speech_lab.connection')) + '</div>';
    html += speechLabField('speech_lab.base_url', data.base_url, 'url', 'config.speech_lab.base_url', 'config.speech_lab.base_url_help');
    html += speechLabField('speech_lab.advanced_ui_url', data.advanced_ui_url || '', 'url', 'config.speech_lab.advanced_ui_url', 'config.speech_lab.advanced_ui_url_help');
    html += speechLabField('speech_lab.language', data.language, 'text', 'config.speech_lab.language', 'config.speech_lab.language_help');
    html += speechLabProviderField(data.chat_llm_provider_id);
    if (speechLabProviderLoadFailed) {
        html += '<div class="cfg-note-banner cfg-note-banner-warning">' + escapeHtml(t('config.speech_lab.provider_load_failed')) + '</div>';
    }
    html += speechLabField('speech_lab.timeout_seconds', data.timeout_seconds, 'number', 'config.speech_lab.timeout', 'config.speech_lab.timeout_help', ' min="1" max="60"');
    html += '<div class="cfg-group-title cfg-group-title-top">' + escapeHtml(t('config.speech_lab.runtime')) + '</div>';
    html += '<div id="speech-lab-status" class="cfg-note-banner">' + escapeHtml(t('config.speech_lab.checking')) + '</div>';
    html += '<div class="field-group"><button type="button" class="btn-secondary" onclick="speechLabRefresh()">' + escapeHtml(t('config.speech_lab.refresh')) + '</button>';
    if (data.advanced_ui_url) {
        html += ' <a id="speech-lab-advanced-link" class="btn-secondary" href="' + escapeAttr(data.advanced_ui_url) + '" target="_blank" rel="noopener noreferrer">' + escapeHtml(t('config.speech_lab.open_advanced')) + '</a>';
    } else {
        html += ' <button id="speech-lab-advanced-link" type="button" class="btn-secondary" disabled>' + escapeHtml(t('config.speech_lab.open_advanced')) + '</button>';
    }
    html += '</div>';
    if (!data.advanced_ui_url) {
        html += '<div class="cfg-note-banner cfg-note-banner-warning">' + escapeHtml(t('config.speech_lab.advanced_ui_url_help')) + '</div>';
    }
    html += '<div id="speech-lab-capability"></div><div id="speech-lab-suggestions"></div><div id="speech-lab-stack"></div>';
    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    await speechLabRefresh();
}

function speechLabToggle(path, enabled, labelKey, helpKey) {
    return '<div class="field-group"><div class="field-label">' + escapeHtml(t(labelKey)) + '</div>' +
        '<div class="field-help">' + escapeHtml(t(helpKey)) + '</div><div class="toggle-wrap">' +
        '<div class="toggle' + (enabled ? ' on' : '') + '" data-path="' + escapeAttr(path) + '" onclick="toggleBool(this)"></div>' +
        '<span class="toggle-label">' + escapeHtml(enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span></div></div>';
}

function speechLabField(path, value, type, labelKey, helpKey, extra) {
    return '<label class="field-group"><span class="field-label">' + escapeHtml(t(labelKey)) + '</span>' +
        '<span class="field-help">' + escapeHtml(t(helpKey)) + '</span><input class="field-input" type="' + type + '" data-path="' +
        escapeAttr(path) + '" value="' + escapeAttr(value) + '"' + (extra || '') + '></label>';
}

function speechLabProviderField(selected) {
    let options = '<option value="">' + escapeHtml(t('config.speech_lab.global_provider')) + '</option>';
    let selectedFound = !selected;
    speechLabProviders.filter(provider => provider.runtime_chat?.eligible === true).forEach(provider => {
        const id = String(provider.id || '').trim();
        if (!id) return;
        if (id === selected) selectedFound = true;
        let label = [provider.name || id, provider.model || ''].filter(Boolean).join(' · ');
        if (provider.runtime_chat?.configured !== true) label += ' — ' + t('config.speech_lab.provider_not_configured');
        options += '<option value="' + escapeAttr(id) + '"' + (id === selected ? ' selected' : '') + '>' + escapeHtml(label) + '</option>';
    });
    if (!selectedFound) {
        const selectedProvider = speechLabProviders.find(provider => String(provider.id || '').trim() === selected);
        const label = selectedProvider ? [selectedProvider.name || selected, selectedProvider.model || ''].filter(Boolean).join(' · ') : selected;
        options += '<option value="' + escapeAttr(selected) + '" selected disabled>' + escapeHtml(label + ' — ' + t('config.speech_lab.provider_unavailable')) + '</option>';
    }
    return '<label class="field-group"><span class="field-label">' + escapeHtml(t('config.speech_lab.chat_llm_provider')) + '</span>' +
        '<span class="field-help">' + escapeHtml(t('config.speech_lab.chat_llm_provider_help')) + '</span>' +
        '<select class="field-select" data-path="speech_lab.chat_llm_provider_id">' + options + '</select></label>';
}

async function speechLabLoadProviders() {
    speechLabProviderLoadFailed = false;
    try {
        const response = await fetch('/api/providers');
        const providers = await speechLabJSON(response);
        speechLabProviders = Array.isArray(providers) ? providers : [];
    } catch (_) {
        speechLabProviders = [];
        speechLabProviderLoadFailed = true;
    }
}

async function speechLabRefresh() {
    const statusNode = document.getElementById('speech-lab-status');
    if (!statusNode) return;
    statusNode.className = 'cfg-note-banner';
    statusNode.textContent = t('config.speech_lab.checking');
    const requests = [
        fetch('/api/speech-lab/status').then(speechLabJSON),
        fetch('/api/speech-lab/capability').then(speechLabJSON),
        fetch('/api/speech-lab/catalog').then(speechLabJSON),
        fetch('/api/speech-lab/suggestions?language=' + encodeURIComponent(speechLabEnsureData().language || 'de')).then(speechLabJSON)
    ];
    const results = await Promise.allSettled(requests);
    speechLabStatus = results[0].status === 'fulfilled' ? results[0].value : null;
    speechLabCapability = results[1].status === 'fulfilled' ? results[1].value : null;
    speechLabCatalog = results[2].status === 'fulfilled' ? results[2].value : null;
    speechLabSuggestions = results[3].status === 'fulfilled' ? results[3].value : null;
    if (!speechLabStatus) {
        statusNode.className = 'cfg-note-banner cfg-note-banner-warning';
        statusNode.textContent = t('config.speech_lab.unreachable');
    } else if (!speechLabStatus.enabled) {
        statusNode.className = 'cfg-note-banner';
        statusNode.textContent = t('config.speech_lab.disabled');
    } else {
        statusNode.className = 'cfg-note-banner ' + (speechLabStatus.ready ? 'cfg-note-banner-success' : 'cfg-note-banner-warning');
        statusNode.textContent = t(speechLabStatus.ready ? 'config.speech_lab.ready' : 'config.speech_lab.not_ready') +
            (speechLabStatus.environment_managed ? ' · ' + t('config.speech_lab.env_managed') : '');
        const activeParts = [speechLabStatus.asr_id, speechLabStatus.tts_id, speechLabStatus.voice].filter(Boolean);
        if (activeParts.length) statusNode.textContent += ' · ' + t('config.speech_lab.active_combination') + ': ' + activeParts.join(' + ');
    }
    const baseInput = document.querySelector('[data-path="speech_lab.base_url"]');
    if (baseInput && speechLabStatus && speechLabStatus.environment_managed) baseInput.disabled = true;
    speechLabRenderCapability();
    speechLabRenderSuggestions();
    speechLabRenderStack();
}

async function speechLabJSON(response) {
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.message || data.error || ('HTTP ' + response.status));
    return data;
}

function speechLabRenderCapability() {
    const node = document.getElementById('speech-lab-capability');
    if (!node || !speechLabCapability) return;
    const gpu = speechLabCapability.gpu || speechLabCapability.hardware || {};
    const tier = speechLabCapability.tier || gpu.tier || t('config.speech_lab.unknown');
    const name = speechLabCapability.device_name || speechLabCapability.vendor || gpu.name || gpu.device_name || gpu.vendor || '';
    node.innerHTML = '<div class="cfg-card"><div class="cfg-card-title">' + escapeHtml(t('config.speech_lab.capability')) + '</div>' +
        '<p>' + escapeHtml(String(tier)) + (name ? ' · ' + escapeHtml(String(name)) : '') + '</p></div>';
}

function speechLabRenderSuggestions() {
    const node = document.getElementById('speech-lab-suggestions');
    if (!node || !speechLabSuggestions) return;
    const pairs = speechLabSuggestions.suggested_pairs || speechLabSuggestions.suggested_presets || [];
    let html = '<div class="cfg-card"><div class="cfg-card-title">' + escapeHtml(t('config.speech_lab.suggestions')) + '</div>';
    html += '<div class="field-help">' + escapeHtml(t('config.speech_lab.heuristic')) + '</div>';
    if (!pairs.length) html += '<p>' + escapeHtml(t('config.speech_lab.no_suggestions')) + '</p>';
    pairs.slice(0, 4).forEach(pair => {
        const label = pair.name || pair.id || [pair.asr_id, pair.tts_id].filter(Boolean).join(' + ');
        html += '<span class="cfg-chip">' + escapeHtml(label || t('config.speech_lab.suggestion')) + '</span> ';
    });
    const cautions = Array.isArray(speechLabSuggestions.caution) ? speechLabSuggestions.caution : [];
    cautions.slice(0, 3).forEach(item => {
        html += '<p class="field-help">' + escapeHtml(item.reason || String(item)) + '</p>';
    });
    node.innerHTML = html + '</div>';
}

function speechLabRenderStack() {
    const node = document.getElementById('speech-lab-stack');
    if (!node || !speechLabCatalog) return;
    const backends = (speechLabCatalog.backends || []).filter(item => item.available &&
        (speechLabShowExperimental || item.stable === true || item.selected_variant?.stable === true));
    const tts = backends.filter(speechLabIsTTS);
    const asr = backends.filter(item => !speechLabIsTTS(item));
    let html = '<div class="cfg-card"><div class="cfg-card-title">' + escapeHtml(t('config.speech_lab.stack')) + '</div>';
    const activeParts = [speechLabStatus?.asr_id || '—', speechLabStatus?.tts_id || '—', speechLabStatus?.voice || '—'];
    html += '<div class="cfg-note-banner cfg-note-banner-info"><strong>' + escapeHtml(t('config.speech_lab.active_combination')) + ':</strong> ' + escapeHtml(activeParts.join(' + ')) + '</div>';
    html += '<label class="field-group"><span class="field-label">ASR</span><select id="speech-lab-asr" class="field-select">' + speechLabOptions(asr, speechLabStatus && speechLabStatus.asr_id) + '</select></label>';
    html += '<label class="field-group"><span class="field-label">TTS</span><select id="speech-lab-tts" class="field-select" onchange="speechLabUpdateVoices()">' + speechLabOptions(tts, speechLabStatus && speechLabStatus.tts_id) + '</select></label>';
    html += '<label class="field-group"><span class="field-label">' + escapeHtml(t('config.speech_lab.voice')) + '</span><span class="field-help">' + escapeHtml(t('config.speech_lab.voice_help')) + '</span><select id="speech-lab-voice" class="field-select"></select></label>';
    html += '<label class="field-group"><input id="speech-lab-experimental" type="checkbox" ' + (speechLabShowExperimental ? 'checked' : '') + ' onchange="speechLabToggleExperimental(this.checked)"> ' + escapeHtml(t('config.speech_lab.show_experimental')) + '</label>';
    html += '<button type="button" class="btn-save" onclick="speechLabApplyStack()">' + escapeHtml(t('config.speech_lab.apply_stack')) + '</button></div>';
    node.innerHTML = html;
    speechLabUpdateVoices();
}

function speechLabIsTTS(backend) {
    if (String(backend.stage || '').toLowerCase() === 'tts') return true;
    if (String(backend.stage || '').toLowerCase() === 'asr') return false;
    const protocol = String(backend.protocol || '').toLowerCase();
    return (backend.voices || []).length > 0 || !!backend.default_voice || protocol.includes('tts') || protocol.includes('speech');
}

function speechLabOptions(items, selected) {
    return items.map(item => '<option value="' + escapeAttr(item.id) + '" ' + (item.id === selected ? 'selected' : '') + '>' +
        escapeHtml(item.name || item.id) + '</option>').join('');
}

function speechLabUpdateVoices() {
    const ttsID = document.getElementById('speech-lab-tts')?.value;
    const backend = (speechLabCatalog?.backends || []).find(item => item.id === ttsID) || {};
    const voice = document.getElementById('speech-lab-voice');
    if (!voice) return;
    const voices = [...new Set((Array.isArray(backend.voices) ? backend.voices : [])
        .map(value => String(value || '').trim()).filter(Boolean))];
    const defaultVoice = String(backend.default_voice || '').trim();
    if (!voices.length && defaultVoice) voices.push(defaultVoice);
    const activeVoice = ttsID === speechLabStatus?.tts_id ? String(speechLabStatus?.voice || '').trim() : '';
    const selected = voices.includes(activeVoice) ? activeVoice : (voices.includes(defaultVoice) ? defaultVoice : (voices[0] || ''));
    voice.innerHTML = voices.map(value => '<option value="' + escapeAttr(value) + '"' + (value === selected ? ' selected' : '') + '>' + escapeHtml(value) + '</option>').join('');
    voice.disabled = voices.length === 0;
}

function speechLabToggleExperimental(enabled) {
    speechLabShowExperimental = !!enabled;
    speechLabRenderStack();
}

async function speechLabApplyStack() {
    if (typeof isDirty !== 'undefined' && isDirty) {
        showToast(t('config.speech_lab.save_first'), 'warn');
        return;
    }
    const request = {
        asr_id: document.getElementById('speech-lab-asr')?.value || '',
        tts_id: document.getElementById('speech-lab-tts')?.value || '',
        voice: document.getElementById('speech-lab-voice')?.value || ''
    };
    if (!request.asr_id || !request.tts_id) {
        showToast(t('config.speech_lab.select_stack'), 'warn');
        return;
    }
    const oldStack = (speechLabStatus?.asr_id || '—') + ' + ' + (speechLabStatus?.tts_id || '—') + ' + ' + (speechLabStatus?.voice || '—');
    const nextStack = request.asr_id + ' + ' + request.tts_id + ' + ' + (request.voice || '—');
    if (!await showConfirm(t('config.speech_lab.confirm').replace('{old}', oldStack).replace('{next}', nextStack))) return;
    try {
        const response = await fetch('/api/speech-lab/stack', {
            method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(request)
        });
        const result = await speechLabJSON(response);
        showToast(result.message || t('config.speech_lab.applied'), 'success');
        await speechLabRefresh();
    } catch (error) {
        showToast(error.message || t('config.speech_lab.apply_failed'), 'error');
    }
}

window.renderSpeechLabSection = renderSpeechLabSection;
window.speechLabRefresh = speechLabRefresh;
window.speechLabApplyStack = speechLabApplyStack;
window.speechLabUpdateVoices = speechLabUpdateVoices;
window.speechLabToggleExperimental = speechLabToggleExperimental;
