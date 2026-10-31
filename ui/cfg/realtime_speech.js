// cfg/realtime_speech.js — dedicated realtime speech profile management

let realtimeSpeechState = null;
let realtimeSpeechCatalog = null;
let realtimeSpeechBaseline = '';

// Comparable snapshot of the draft (master settings + every profile incl. typed keys).
function rsSnapshot() {
    const s = realtimeSpeechState || {};
    return JSON.stringify({
        enabled: !!s.enabled,
        default_profile: s.default_profile || '',
        park_after_seconds: Number(s.park_after_seconds),
        profiles: (s.profiles || []).map(profile => Object.assign({}, profile))
    });
}

function rsMarkClean() {
    realtimeSpeechBaseline = rsSnapshot();
}

// Read by hasUnsavedConfigChanges() in js/config/main.js before the section is switched.
window.realtimeSpeechHasUnsavedChanges = function () {
    if (!realtimeSpeechState || !document.querySelector('[data-rs-save]')) return false;
    rsReadAll();
    return rsSnapshot() !== realtimeSpeechBaseline;
};

function rsEsc(value) {
    return String(value == null ? '' : value)
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
}

function rsID() {
    if (window.crypto && typeof window.crypto.randomUUID === 'function') {
        return 'voice-' + window.crypto.randomUUID().slice(0, 12);
    }
    return 'voice-' + Date.now().toString(36) + Math.random().toString(36).slice(2, 7);
}

function rsProvider(id) {
    return ((realtimeSpeechCatalog && realtimeSpeechCatalog.providers) || []).find(provider => provider.id === id);
}

function rsNormalizeProfile(profile) {
    const provider = rsProvider(profile.provider) || (realtimeSpeechCatalog.providers || [])[0] || {};
    return {
        id: profile.id || rsID(),
        name: profile.name || provider.label || '',
        provider: profile.provider || provider.id || 'openai',
        model: profile.model || provider.default_model || '',
        voice: profile.voice || provider.default_voice || '',
        enabled: profile.enabled !== false,
        api_key_set: !!profile.api_key_set,
        api_key: '',
        clear_api_key: false
    };
}

function rsProfileOptions(selected) {
    return (realtimeSpeechState.profiles || [])
        .filter(profile => profile.enabled)
        .map(profile => `<option value="${rsEsc(profile.id)}" ${profile.id === selected ? 'selected' : ''}>${rsEsc(profile.name)}</option>`)
        .join('');
}

function rsSelectOptions(items, selected, includeCurrent) {
    const choices = (items || []).filter(item => item.offered !== false);
    if (includeCurrent && selected && !choices.some(item => item.id === selected)) {
        const current = (items || []).find(item => item.id === selected);
        choices.unshift(current || { id: selected, label: selected, deprecated: true });
    }
    return choices.map(item => `<option value="${rsEsc(item.id)}" ${item.id === selected ? 'selected' : ''}>
        ${rsEsc(item.label || item.id)}${item.deprecated ? ' · ' + rsEsc(t('config.realtime_speech.deprecated')) : ''}
    </option>`).join('');
}

function rsProfileCard(profile, index) {
    const provider = rsProvider(profile.provider) || {};
    const providerOptions = (realtimeSpeechCatalog.providers || []).map(item =>
        `<option value="${rsEsc(item.id)}" ${item.id === profile.provider ? 'selected' : ''}>${rsEsc(item.label)}</option>`
    ).join('');
    const keyless = profile.provider === 'speech_lab' || provider.capabilities && provider.capabilities.requires_api_key === false;
    const keyState = keyless
        ? t('config.realtime_speech.keyless')
        : (profile.api_key_set
            ? t('config.realtime_speech.key_stored')
            : t('config.realtime_speech.key_missing'));
    return `<article class="rs-profile-card" data-rs-profile="${index}">
        <div class="rs-profile-head">
            <div>
                <span class="rs-profile-provider">${rsEsc(provider.label || profile.provider)}</span>
                <strong>${rsEsc(profile.name || profile.id)}</strong>
                <code>${rsEsc(profile.id)}</code>
            </div>
            <div class="rs-profile-head-actions">
                <label class="rs-inline-toggle">
                    <input type="checkbox" data-rs-field="enabled" aria-describedby="rs-profile-${index}-enabled-help" ${profile.enabled ? 'checked' : ''}>
                    <span>${rsEsc(t('config.realtime_speech.profile_enabled'))}</span>
                </label>
                <button type="button" class="btn btn-danger rs-icon-button" data-rs-delete="${index}" title="${rsEsc(t('config.realtime_speech.delete_profile'))}">×</button>
            </div>
        </div>
        <small class="field-help" id="rs-profile-${index}-enabled-help">${rsEsc(t('config.realtime_speech.profile_enabled_help'))}</small>
        <div class="rs-profile-grid">
            <label class="field-group">
                <span class="field-label">${rsEsc(t('config.realtime_speech.profile_name'))}</span>
                <input class="field-input" data-rs-field="name" value="${rsEsc(profile.name)}" maxlength="100">
                <small class="field-help">${rsEsc(t('config.realtime_speech.profile_name_help'))}</small>
            </label>
            <label class="field-group">
                <span class="field-label">${rsEsc(t('config.realtime_speech.provider'))}</span>
                <select class="field-input" data-rs-field="provider">${providerOptions}</select>
                <small class="field-help">${rsEsc(t('config.realtime_speech.provider_help'))}</small>
            </label>
            <label class="field-group">
                <span class="field-label">${rsEsc(t('config.realtime_speech.model'))}</span>
                <select class="field-input" data-rs-field="model">${rsSelectOptions(provider.models, profile.model, true)}</select>
                <small class="field-help">${rsEsc(t('config.realtime_speech.model_help'))}</small>
            </label>
            <label class="field-group">
                <span class="field-label">${rsEsc(t('config.realtime_speech.voice'))}</span>
                <select class="field-input" data-rs-field="voice">${rsSelectOptions(provider.voices, profile.voice, true)}</select>
                <small class="field-help">${rsEsc(t('config.realtime_speech.voice_help'))}</small>
            </label>
            ${keyless ? `<div class="field-group rs-key-field">
                <span class="field-label">${rsEsc(t('config.realtime_speech.api_key'))}</span>
                <small class="field-help">${rsEsc(t('config.realtime_speech.speech_lab_help'))}</small>
            </div>` : `<label class="field-group rs-key-field">
                <span class="field-label">${rsEsc(t('config.realtime_speech.api_key'))}</span>
                <input class="field-input" type="password" data-rs-field="api_key" value=""
                    placeholder="${rsEsc(profile.api_key_set ? t('config.realtime_speech.key_keep_placeholder') : t('config.realtime_speech.key_enter_placeholder'))}"
                    autocomplete="new-password">
                <small class="field-help">${rsEsc(t('config.realtime_speech.api_key_help'))}</small>
                <small class="${profile.api_key_set ? 'rs-key-set' : 'rs-key-missing'}">${rsEsc(keyState)}</small>
            </label>
            <div class="field-group rs-key-actions">
                <span class="field-label">${rsEsc(t('config.realtime_speech.vault_action'))}</span>
                <label class="rs-inline-toggle">
                    <input type="checkbox" data-rs-field="clear_api_key" ${profile.clear_api_key ? 'checked' : ''}>
                    <span>${rsEsc(t('config.realtime_speech.clear_key'))}</span>
                </label>
                <small class="field-help">${rsEsc(t('config.realtime_speech.clear_key_help'))}</small>
            </div>`}
        </div>
        <div class="rs-profile-actions">
            ${profile.provider === 'xai' && profile.api_key_set
                ? `<button type="button" class="btn btn-secondary" data-rs-refresh-voices="${index}">${rsEsc(t('config.realtime_speech.refresh_voices'))}</button>`
                : ''}
            <button type="button" class="btn btn-secondary" data-rs-test="${index}" data-config-action-draft="true">${rsEsc(t('config.realtime_speech.test_connection'))}</button>
            <span class="cfg-action-result rs-test-status" data-rs-test-status="${index}" role="status" aria-live="polite" aria-atomic="true"></span>
        </div>
    </article>`;
}

function rsRender() {
    const content = document.getElementById('content');
    const profiles = realtimeSpeechState.profiles || [];
    content.innerHTML = `<section class="cfg-section active rs-section">
        <div class="section-header">${rsEsc(t('config.realtime_speech.title'))}</div>
        <div class="section-desc">${rsEsc(t('config.realtime_speech.description'))}</div>
        <div class="rs-master-card">
            <h2 class="cfg-topic-heading">${rsEsc(t('config.refresh.operation'))}</h2>
            <label class="rs-master-toggle">
                <input type="checkbox" data-rs-master="enabled" aria-describedby="rs-master-enabled-help" ${realtimeSpeechState.enabled ? 'checked' : ''}>
                <span>
                    <strong>${rsEsc(t('config.realtime_speech.master_enabled'))}</strong>
                    <small class="field-help" id="rs-master-enabled-help">${rsEsc(t('config.realtime_speech.master_enabled_help'))}</small>
                </span>
            </label>
            <label class="field-group">
                <span class="field-label">${rsEsc(t('config.realtime_speech.default_profile'))}</span>
                <select class="field-input" data-rs-master="default_profile">
                    <option value="">${rsEsc(t('config.realtime_speech.no_default'))}</option>
                    ${rsProfileOptions(realtimeSpeechState.default_profile)}
                </select>
                <small class="field-help">${rsEsc(t('config.realtime_speech.default_profile_help'))}</small>
            </label>
            <label class="field-group">
                <span class="field-label">${rsEsc(t('config.realtime_speech.park_after'))}</span>
                <input class="field-input" type="number" min="5" max="60" step="1"
                    data-rs-master="park_after_seconds" value="${Number(realtimeSpeechState.park_after_seconds || 5)}">
                <small class="field-help">${rsEsc(t('config.realtime_speech.park_after_help'))}</small>
            </label>
        </div>
        <div class="rs-section-toolbar">
            <div>
                <strong>${rsEsc(t('config.realtime_speech.profiles'))}</strong>
                <span>${profiles.length}</span>
            </div>
            <button type="button" class="btn btn-secondary" data-rs-add>${rsEsc(t('config.realtime_speech.add_profile'))}</button>
        </div>
        <div class="rs-profile-list">
            ${profiles.length
                ? profiles.map(rsProfileCard).join('')
                : `<div class="rs-empty">${rsEsc(t('config.realtime_speech.no_profiles'))}</div>`}
        </div>
        <div class="rs-save-row">
            <small class="rs-save-note">${rsEsc(t('config.realtime_speech.save_note'))}</small>
            ${cfgActionResult.html('rs-save-btn-result')}
            <button type="button" class="btn-save" id="rs-save-btn" data-rs-save>${rsEsc(t('config.realtime_speech.save'))}</button>
        </div>
        <div class="rs-security-note">
            <strong>${rsEsc(t('config.realtime_speech.security_title'))}</strong>
            <p>${rsEsc(t('config.realtime_speech.security_note'))}</p>
        </div>
    </section>`;
    rsBind();
}

function rsReadCard(card) {
    const index = Number(card.dataset.rsProfile);
    const profile = realtimeSpeechState.profiles[index];
    card.querySelectorAll('[data-rs-field]').forEach(input => {
        const key = input.dataset.rsField;
        profile[key] = input.type === 'checkbox' ? input.checked : input.value;
    });
    return profile;
}

function rsReadAll() {
    document.querySelectorAll('[data-rs-profile]').forEach(rsReadCard);
    const enabled = document.querySelector('[data-rs-master="enabled"]');
    const defaultProfile = document.querySelector('[data-rs-master="default_profile"]');
    const park = document.querySelector('[data-rs-master="park_after_seconds"]');
    realtimeSpeechState.enabled = !!(enabled && enabled.checked);
    realtimeSpeechState.default_profile = defaultProfile ? defaultProfile.value : '';
    realtimeSpeechState.park_after_seconds = Number(park && park.value);
}

function rsProviderChanged(card) {
    const profile = rsReadCard(card);
    const provider = rsProvider(profile.provider);
    if (!provider) return;
    profile.model = provider.default_model;
    profile.voice = provider.default_voice;
    rsRender();
}

async function rsRequest(url, options) {
    const response = await fetch(url, Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {}));
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.error || body.message || ('HTTP ' + response.status));
    return body;
}

// Tests the profile as typed in (its own body, including a newly typed key).
async function rsTest(index) {
    const card = document.querySelector(`[data-rs-profile="${index}"]`);
    if (!card) return;
    const profile = rsReadCard(card);
    return cfgActionResult(card.querySelector(`[data-rs-test="${index}"]`), async () => {
        const body = await rsRequest('/api/realtime-speech/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(profile)
        });
        if (Array.isArray(body.voices) && body.voices.length) {
            const provider = rsProvider('xai');
            if (provider) provider.voices = body.voices;
            profile.voice = body.voice || profile.voice;
        }
        return { status: 'ok' };
    }, {
        target: card.querySelector(`[data-rs-test-status="${index}"]`),
        pending: t('config.realtime_speech.testing'),
        success: t('config.realtime_speech.test_success')
    });
}

async function rsRefreshVoices(index) {
    const card = document.querySelector(`[data-rs-profile="${index}"]`);
    if (!card) return;
    const profile = rsReadCard(card);
    return cfgActionResult(card.querySelector(`[data-rs-refresh-voices="${index}"]`), async () => {
        const body = await rsRequest('/api/realtime-speech/catalog?profile_id=' + encodeURIComponent(profile.id));
        const xai = (body.providers || []).find(provider => provider.id === 'xai');
        const localXAI = rsProvider('xai');
        if (xai && localXAI) localXAI.voices = xai.voices;
        rsRender();
        return { status: 'ok' };
    }, {
        target: card.querySelector(`[data-rs-test-status="${index}"]`),
        pending: t('config.realtime_speech.loading_voices'),
        success: t('config.realtime_speech.voices_loaded')
    });
}

async function rsSave() {
    rsReadAll();
    const park = Number(realtimeSpeechState.park_after_seconds);
    if (!Number.isInteger(park) || park < 5 || park > 60) {
        cfgActionResult.show('rs-save-btn-result', 'error', t('config.realtime_speech.park_invalid'));
        return false;
    }
    const ids = new Set();
    for (const profile of realtimeSpeechState.profiles) {
        if (!profile.id || ids.has(profile.id) || !profile.name.trim()) {
            cfgActionResult.show('rs-save-btn-result', 'error', t('config.realtime_speech.profile_invalid'));
            return false;
        }
        ids.add(profile.id);
    }
    let saved = false;
    await cfgActionResult(document.getElementById('rs-save-btn'), async () => {
        realtimeSpeechState = await rsRequest('/api/realtime-speech/config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(realtimeSpeechState)
        });
        realtimeSpeechState.profiles = (realtimeSpeechState.profiles || []).map(rsNormalizeProfile);
        // The section is re-rendered; the result line is found again by its id.
        rsRender();
        rsMarkClean();
        saved = true;
        return { status: 'ok' };
    }, {
        target: 'rs-save-btn-result',
        pending: t('config.realtime_speech.saving'),
        success: t('config.realtime_speech.saved')
    });
    return saved;
}
// The unsaved-changes dialog's "Save" saves profiles through the section's own endpoint.
window.realtimeSpeechSaveUnsaved = rsSave;

function rsBind() {
    document.querySelectorAll('[data-rs-profile]').forEach(card => {
        card.querySelector('[data-rs-field="provider"]').addEventListener('change', () => rsProviderChanged(card));
    });
    document.querySelector('[data-rs-add]')?.addEventListener('click', () => {
        const first = (realtimeSpeechCatalog.providers || [])[0] || {};
        realtimeSpeechState.profiles.push(rsNormalizeProfile({
            id: rsID(),
            name: t('config.realtime_speech.new_profile_name'),
            provider: first.id,
            model: first.default_model,
            voice: first.default_voice,
            enabled: true
        }));
        rsRender();
    });
    document.querySelectorAll('[data-rs-delete]').forEach(button => {
        button.addEventListener('click', async () => {
            rsReadAll();
            const index = Number(button.dataset.rsDelete);
            const removed = realtimeSpeechState.profiles[index];
            const label = (removed && (removed.name || removed.id)) || String(index + 1);
            if (!(await showConfirm(t('config.realtime_speech.confirm_delete_title'), t('config.realtime_speech.confirm_delete_message', { name: label })))) return;
            realtimeSpeechState.profiles.splice(index, 1);
            if (removed && realtimeSpeechState.default_profile === removed.id) realtimeSpeechState.default_profile = '';
            rsRender();
        });
    });
    document.querySelectorAll('[data-rs-test]').forEach(button => button.addEventListener('click', () => rsTest(Number(button.dataset.rsTest))));
    document.querySelectorAll('[data-rs-refresh-voices]').forEach(button => button.addEventListener('click', () => rsRefreshVoices(Number(button.dataset.rsRefreshVoices))));
    document.querySelector('[data-rs-save]')?.addEventListener('click', rsSave);
}

async function renderRealtimeSpeechSection() {
    const content = document.getElementById('content');
    content.innerHTML = `<div class="cfg-section active"><div class="cfg-loading-state">${rsEsc(t('config.realtime_speech.loading'))}</div></div>`;
    try {
        const [state, catalog] = await Promise.all([
            rsRequest('/api/realtime-speech/config'),
            rsRequest('/api/realtime-speech/catalog')
        ]);
        realtimeSpeechCatalog = catalog;
        realtimeSpeechState = state;
        realtimeSpeechState.profiles = (state.profiles || []).map(rsNormalizeProfile);
        rsRender();
        rsMarkClean();
    } catch (error) {
        content.innerHTML = `<div class="cfg-section active"><div class="rs-load-error">${rsEsc(error.message)}</div></div>`;
    }
}
