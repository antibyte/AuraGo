// cfg/tts.js — TTS config section with Piper voice browser

function renderTTSSection(section) {
    const data = configData['tts'] || {};
    const piperData = data.piper || {};
    const piperEnabled = piperData.enabled === true;
    const currentProvider = data.provider || '';
    const elData = data.elevenlabs || {};
    const mmData = data.minimax || {};
    const supertonicData = data.supertonic || {};
    const supertonicAutoStart = supertonicData.auto_start === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.provider_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.provider_help') + '</div>';
    html += '<select class="field-select" data-path="tts.provider" onchange="ttsProviderChanged(this.value)">';
    html += '<option value=""' + (currentProvider === '' ? ' selected' : '') + '>— ' + t('config.tts.provider_none') + ' —</option>';
    html += '<option value="google"' + (currentProvider === 'google' ? ' selected' : '') + '>' + t('config.tts.provider_google') + '</option>';
    html += '<option value="elevenlabs"' + (currentProvider === 'elevenlabs' ? ' selected' : '') + '>' + t('config.tts.provider_elevenlabs') + '</option>';
    html += '<option value="minimax"' + (currentProvider === 'minimax' ? ' selected' : '') + '>' + t('config.tts.provider_minimax') + '</option>';
    html += '<option value="piper"' + (currentProvider === 'piper' ? ' selected' : '') + '>' + t('config.tts.provider_piper') + '</option>';
    html += '<option value="supertonic"' + (currentProvider === 'supertonic' ? ' selected' : '') + '>' + t('config.tts.provider_supertonic') + '</option>';
    html += '</select>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.language_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.language_help') + '</div>';
    html += ttsLanguageSelect('tts.language', data.language || 'de');
    html += '</div>';

    const showEL = currentProvider === 'elevenlabs';
    html += '<div id="tts-elevenlabs-section" class="tts-provider-section' + (showEL ? '' : ' is-hidden') + '">';
    html += '<div class="tts-subsection-title">🎤 ElevenLabs</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.elevenlabs_api_key_label') + '</div>';
    html += '<div class="adg-password-row">';
    html += '<div class="password-wrap adg-password-input">';
    html += '<input class="field-input adg-password-input" type="password" id="tts-elevenlabs-api-key" value="' + escapeAttr(cfgSecretValue(elData.api_key)) + '" placeholder="' + escapeAttr(cfgSecretPlaceholder(elData.api_key, t('config.tts.elevenlabs_api_key_placeholder'))) + '">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save adg-save-btn" onclick="ttsSaveElevenLabsKey()">💾</button>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.elevenlabs_voice_id_label') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.elevenlabs.voice_id" value="' + escapeAttr(elData.voice_id || '') + '" placeholder="' + t('config.tts.elevenlabs_voice_id_placeholder') + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.elevenlabs_model_id_label') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.elevenlabs.model_id" value="' + escapeAttr(elData.model_id || '') + '" placeholder="' + t('config.tts.elevenlabs_model_id_placeholder') + '">';
    html += '</div>';
    html += '</div>';

    const showMM = currentProvider === 'minimax';
    html += '<div id="tts-minimax-section" class="tts-provider-section' + (showMM ? '' : ' is-hidden') + '">';
    html += '<div class="tts-subsection-title">🎵 MiniMax TTS</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.minimax_api_key_label') + '</div>';
    html += '<div class="adg-password-row">';
    html += '<div class="password-wrap adg-password-input">';
    html += '<input class="field-input adg-password-input" type="password" id="tts-minimax-api-key" value="' + escapeAttr(cfgSecretValue(mmData.api_key)) + '" placeholder="' + escapeAttr(cfgSecretPlaceholder(mmData.api_key, t('config.tts.minimax_api_key_placeholder'))) + '">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save adg-save-btn" onclick="ttsSaveMiniMaxKey()">💾</button>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.minimax_voice_id_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.minimax_voice_id_help') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.minimax.voice_id" value="' + escapeAttr(mmData.voice_id || '') + '" placeholder="' + t('config.tts.minimax_voice_id_placeholder') + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.minimax_model_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.minimax_model_help') + '</div>';
    html += '<select class="field-select" data-path="tts.minimax.model_id">';
    html += '<option value="speech-2.8-hd"' + (mmData.model_id === 'speech-2.8-hd' || !mmData.model_id ? ' selected' : '') + '>' + t('config.tts.minimax_model_hd') + '</option>';
    html += '<option value="speech-2.8-turbo"' + (mmData.model_id === 'speech-2.8-turbo' ? ' selected' : '') + '>' + t('config.tts.minimax_model_turbo') + '</option>';
    html += '</select>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.minimax_speed_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.minimax_speed_help') + '</div>';
    html += '<input class="field-input" type="number" data-path="tts.minimax.speed" value="' + (mmData.speed || 1.0) + '" min="0.5" max="2.0" step="0.1" placeholder="' + t('config.tts.minimax_speed_placeholder') + '">';
    html += '</div>';
    html += '</div>';

    const showSupertonic = currentProvider === 'supertonic';
    html += '<div id="tts-supertonic-section" class="tts-provider-section' + (showSupertonic ? '' : ' is-hidden') + '">';
    html += '<div class="tts-subsection-title">Supertonic TTS (Local Docker)</div>';
    html += '<div id="supertonic-status-banner" class="adg-status-banner">' + t('config.tts.supertonic_checking') + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.supertonic_auto_start_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.supertonic_auto_start_help') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (supertonicAutoStart ? ' on' : '') + '" data-path="tts.supertonic.auto_start" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (supertonicAutoStart ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.supertonic_url_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.supertonic_url_help') + '</div>';
    html += '<input class="field-input" type="url" data-path="tts.supertonic.url" value="' + escapeAttr(supertonicData.url || 'http://127.0.0.1:7788') + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.supertonic_model_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.supertonic_model_help') + '</div>';
    const supertonicModel = supertonicData.model || 'supertonic-3';
    html += '<select class="field-select" data-path="tts.supertonic.model">';
    html += '<option value="supertonic-3"' + (supertonicModel === 'supertonic-3' ? ' selected' : '') + '>supertonic-3</option>';
    html += '<option value="supertonic-2"' + (supertonicModel === 'supertonic-2' ? ' selected' : '') + '>supertonic-2</option>';
    html += '</select>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.supertonic_voice_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.supertonic_voice_help') + '</div>';
    html += '<div class="tts-voice-row">';
    html += '<input class="field-input" type="text" id="supertonic-voice-input" data-path="tts.supertonic.voice" value="' + escapeAttr(supertonicData.voice || 'M1') + '">';
    html += '<button class="btn-save adg-save-btn" onclick="supertonicBrowseStyles()">' + t('config.tts.supertonic_load_voices') + '</button>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.supertonic_speed_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.supertonic_speed_help') + '</div>';
    html += '<input class="field-input" type="number" data-path="tts.supertonic.speed" value="' + (supertonicData.speed || 1.0) + '" min="0.7" max="2.0" step="0.05">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.supertonic_steps_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.supertonic_steps_help') + '</div>';
    html += '<input class="field-input" type="number" data-path="tts.supertonic.steps" value="' + (supertonicData.steps || 8) + '" min="1" max="100" step="1">';
    html += '</div>';

    const supertonicFormat = supertonicData.response_format || 'wav';
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.supertonic_format_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.supertonic_format_help') + '</div>';
    html += '<select class="field-select" data-path="tts.supertonic.response_format">';
    html += '<option value="wav"' + (supertonicFormat === 'wav' ? ' selected' : '') + '>' + t('config.tts.supertonic_format_wav') + '</option>';
    html += '<option value="flac"' + (supertonicFormat === 'flac' ? ' selected' : '') + '>' + t('config.tts.supertonic_format_flac') + '</option>';
    html += '<option value="ogg"' + (supertonicFormat === 'ogg' ? ' selected' : '') + '>' + t('config.tts.supertonic_format_ogg') + '</option>';
    html += '</select>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.supertonic_image_label') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.supertonic.image" value="' + escapeAttr(supertonicData.image || 'ghcr.io/antibyte/aurago-supertonic:latest') + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.supertonic_port_label') + '</div>';
    html += '<input class="field-input" type="number" data-path="tts.supertonic.container_port" value="' + (supertonicData.container_port || 7788) + '" min="1" max="65535">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.supertonic_data_path_label') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.supertonic.data_path" value="' + escapeAttr(supertonicData.data_path || 'data/supertonic') + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.supertonic_container_name_label') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.supertonic.container_name" value="' + escapeAttr(supertonicData.container_name || 'aurago-supertonic-tts') + '">';
    html += '</div>';

    html += '<div id="supertonic-style-modal" class="modal-overlay tts-voice-modal" onclick="if(event.target===this)supertonicCloseStyleModal()">';
    html += '<div class="modal">';
    html += '<div class="modal-header"><span>' + t('config.tts.supertonic_style_browser_title') + '</span><span class="modal-close" onclick="supertonicCloseStyleModal()">&times;</span></div>';
    html += '<div class="modal-body tts-voice-modal-body">';
    html += '<div id="supertonic-style-list" class="tts-voice-list">' + t('config.tts.supertonic_loading_voices') + '</div>';
    html += '</div></div></div>';

    html += '</div>';

    html += '<div class="tts-subsection-title">🗣️ Piper TTS (Local Docker)</div>';
    html += '<div id="piper-status-banner" class="adg-status-banner">' + t('config.tts.piper_checking') + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.piper_enabled_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.piper_enabled_help') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (piperEnabled ? ' on' : '') + '" data-path="tts.piper.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (piperEnabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.piper_voice_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.piper_voice_help') + '</div>';
    html += '<div class="tts-voice-row">';
    html += '<input class="field-input" type="text" id="piper-voice-input" data-path="tts.piper.voice" value="' + escapeAttr(piperData.voice || 'de_DE-thorsten-high') + '">';
    html += '<button class="btn-save adg-save-btn" onclick="piperBrowseVoices()">🔍 ' + t('config.tts.piper_browse_voices') + '</button>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.piper_speaker_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.piper_speaker_help') + '</div>';
    html += '<input class="field-input" type="number" data-path="tts.piper.speaker_id" value="' + (piperData.speaker_id || 0) + '" min="0">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.piper_port_label') + '</div>';
    html += '<input class="field-input" type="number" data-path="tts.piper.container_port" value="' + (piperData.container_port || 10200) + '" min="1" max="65535">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.piper_image_label') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.piper.image" value="' + escapeAttr(piperData.image || 'rhasspy/wyoming-piper:latest') + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.piper_data_path_label') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.piper.data_path" value="' + escapeAttr(piperData.data_path || 'data/piper') + '">';
    html += '</div>';

    html += '<div id="piper-voice-modal" class="modal-overlay tts-voice-modal" onclick="if(event.target===this)piperCloseVoiceModal()">';
    html += '<div class="modal">';
    html += '<div class="modal-header"><span>' + t('config.tts.piper_voice_browser_title') + '</span><span class="modal-close" onclick="piperCloseVoiceModal()">&times;</span></div>';
    html += '<div class="modal-body tts-voice-modal-body">';
    html += '<div id="piper-voice-list" class="tts-voice-list">' + t('config.tts.piper_loading_voices') + '</div>';
    html += '</div></div></div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;

    if (piperEnabled) {
        piperCheckStatus();
    } else {
        piperSetBanner('neutral', t('config.tts.piper_status_disabled'));
    }
    if (showSupertonic) {
        supertonicCheckStatus();
    }
}

function ttsLanguageSelect(path, selected) {
    const languages = ['de', 'en', 'fr', 'es', 'it', 'pt', 'nl', 'ja', 'zh'];
    const customOption = typeof CFG_OPTION_OTHER_CUSTOM === 'string' ? CFG_OPTION_OTHER_CUSTOM : 'Other / Custom';
    const current = String(selected || '').trim();
    const isCustom = current && !languages.includes(current);
    let html = '<select class="field-select" data-path="' + escapeAttr(path) + '" onchange="cfgToggleCustomInput(this)">';
    languages.forEach(code => {
        html += '<option value="' + code + '"' + (current === code ? ' selected' : '') + '>' + code + '</option>';
    });
    html += '<option value="' + escapeAttr(customOption) + '"' + (isCustom ? ' selected' : '') + '>' + cfgFieldOptionLabel(customOption) + '</option>';
    html += '</select>';
    html += '<input class="field-input cfg-custom-input' + (isCustom ? '' : ' is-hidden') + '" type="text" data-custom-for="' + escapeAttr(path) + '" value="' + escapeAttr(isCustom ? current : '') + '" placeholder="' + escapeAttr(t('config.tts.language_placeholder')) + '">';
    html += '<div class="field-help">' + t('config.tts.language_custom_help') + '</div>';
    return html;
}

function ttsProviderChanged(val) {
    const elSection = document.getElementById('tts-elevenlabs-section');
    if (elSection) elSection.classList.toggle('is-hidden', val !== 'elevenlabs');
    const mmSection = document.getElementById('tts-minimax-section');
    if (mmSection) mmSection.classList.toggle('is-hidden', val !== 'minimax');
    const supertonicSection = document.getElementById('tts-supertonic-section');
    if (supertonicSection) {
        supertonicSection.classList.toggle('is-hidden', val !== 'supertonic');
        if (val === 'supertonic') supertonicCheckStatus();
    }
}

function piperSetBanner(state, text) {
    const banner = document.getElementById('piper-status-banner');
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function supertonicSetBanner(state, text) {
    const banner = document.getElementById('supertonic-status-banner');
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function ttsSaveElevenLabsKey() {
    const input = document.getElementById('tts-elevenlabs-api-key');
    const value = input ? input.value.trim() : '';
    if (!value) {
        showToast(t('config.tts.elevenlabs_api_key_required'), 'warn');
        return;
    }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'tts_elevenlabs_api_key', value })
    })
        .then(r => r.json())
        .then(res => {
            if (res.status === 'ok' || res.success) {
                showToast(t('config.tts.elevenlabs_api_key_saved'), 'success');
                cfgMarkSecretStored(input, 'tts.elevenlabs.api_key');
            } else {
                showToast(res.message || t('config.tts.elevenlabs_api_key_save_failed'), 'error');
            }
        })
        .catch(() => showToast(t('config.tts.elevenlabs_api_key_save_failed'), 'error'));
}

function ttsSaveMiniMaxKey() {
    const input = document.getElementById('tts-minimax-api-key');
    const value = input ? input.value.trim() : '';
    if (!value) {
        showToast(t('config.tts.minimax_api_key_required'), 'warn');
        return;
    }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'tts_minimax_api_key', value })
    })
        .then(r => r.json())
        .then(res => {
            if (res.status === 'ok' || res.success) {
                showToast(t('config.tts.minimax_api_key_saved'), 'success');
                cfgMarkSecretStored(input, 'tts.minimax.api_key');
            } else {
                showToast(res.message || t('config.tts.minimax_api_key_save_failed'), 'error');
            }
        })
        .catch(() => showToast(t('config.tts.minimax_api_key_save_failed'), 'error'));
}

function piperCheckStatus() {
    piperSetBanner('neutral', '⏳ ' + t('config.tts.piper_checking'));

    fetch('/api/piper/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                piperSetBanner('neutral', t('config.tts.piper_status_disabled'));
            } else if (res.status === 'running') {
                let text = t('config.tts.piper_status_running');
                if (res.voice) text += ' — ' + res.voice;
                piperSetBanner('success', text);
            } else if (res.status === 'stopped') {
                piperSetBanner('warning', '🟡 ' + t('config.tts.piper_status_stopped'));
            } else {
                piperSetBanner('danger', '🔴 ' + t('config.tts.piper_status_error') + (res.error ? ': ' + res.error : ''));
            }
        })
        .catch(() => piperSetBanner('danger', '🔴 ' + t('config.tts.piper_status_error')));
}

function supertonicCheckStatus() {
    supertonicSetBanner('neutral', t('config.tts.supertonic_checking'));

    fetch('/api/supertonic/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                supertonicSetBanner('neutral', t('config.tts.supertonic_status_disabled'));
            } else if (res.status === 'running') {
                let text = t('config.tts.supertonic_status_running');
                const details = [];
                if (res.model) details.push(res.model);
                if (res.voices_loaded !== undefined) details.push(res.voices_loaded + ' ' + t('config.tts.supertonic_voices_loaded'));
                if (details.length) text += ' — ' + details.join(' · ');
                supertonicSetBanner('success', text);
            } else if (res.status === 'loading') {
                supertonicSetBanner('warning', t('config.tts.supertonic_status_loading'));
            } else if (res.status === 'stopped') {
                supertonicSetBanner('warning', t('config.tts.supertonic_status_stopped'));
            } else {
                const message = res.message || res.error || '';
                supertonicSetBanner('danger', t('config.tts.supertonic_status_error') + (message ? ': ' + message : ''));
            }
        })
        .catch(() => supertonicSetBanner('danger', t('config.tts.supertonic_status_error')));
}

function supertonicBrowseStyles() {
    const overlay = document.getElementById('supertonic-style-modal');
    if (!overlay) return;
    overlay.classList.add('active');
    const list = document.getElementById('supertonic-style-list');
    if (list) list.innerHTML = '<div class="tts-voice-loading">' + t('config.tts.supertonic_loading_voices') + '</div>';

    fetch('/api/supertonic/styles')
        .then(r => r.json())
        .then(res => {
            if (!list) return;
            if (res.error) {
                list.innerHTML = '<div class="tts-voice-error">' + escapeHtml(res.error) + '</div>';
                return;
            }
            const styles = res.styles || [];
            if (styles.length === 0) {
                list.innerHTML = '<div class="tts-voice-empty">' + t('config.tts.supertonic_no_voices') + '</div>';
                return;
            }
            let html = '';
            for (const style of styles) {
                const kind = style.kind || '';
                html += '<div class="tts-voice-item" data-style-name="' + escapeAttr(style.name || '') + '" onclick="supertonicSelectStyle(this.dataset.styleName)">';
                html += '<div class="tts-voice-item-name">' + escapeHtml(style.name || '') + '</div>';
                if (kind) html += '<div class="tts-voice-item-desc">' + escapeHtml(kind) + '</div>';
                html += '</div>';
            }
            list.innerHTML = html;
        })
        .catch(() => {
            if (list) list.innerHTML = '<div class="tts-voice-error">' + t('config.tts.supertonic_voice_error') + '</div>';
        });
}

function supertonicSelectStyle(name) {
    const input = document.getElementById('supertonic-voice-input');
    if (input) {
        input.value = name;
        input.dispatchEvent(new Event('input', { bubbles: true }));
    }
    supertonicCloseStyleModal();
}

function supertonicCloseStyleModal() {
    const overlay = document.getElementById('supertonic-style-modal');
    if (overlay) overlay.classList.remove('active');
}

function piperBrowseVoices() {
    const overlay = document.getElementById('piper-voice-modal');
    if (!overlay) return;
    overlay.classList.add('active');
    const list = document.getElementById('piper-voice-list');
    if (list) list.innerHTML = '<div class="tts-voice-loading">' + t('config.tts.piper_loading_voices') + '</div>';

    fetch('/api/piper/voices')
        .then(r => r.json())
        .then(res => {
            if (res.error) {
                list.innerHTML = '<div class="tts-voice-error">' + escapeHtml(res.error) + '</div>';
                return;
            }
            const voices = res.voices || [];
            if (voices.length === 0) {
                list.innerHTML = '<div class="tts-voice-empty">' + t('config.tts.piper_no_voices') + '</div>';
                return;
            }
            let html = '';
            for (const v of voices) {
                const langs = (v.languages || []).join(', ');
                const installed = v.installed ? '✅ ' : '';
                html += '<div class="tts-voice-item" onclick="piperSelectVoice(\'' + escapeAttr(v.name) + '\')">';
                html += '<div class="tts-voice-item-name">' + installed + escapeHtml(v.name) + '</div>';
                if (v.description) html += '<div class="tts-voice-item-desc">' + escapeHtml(v.description) + '</div>';
                if (langs) html += '<div class="tts-voice-item-lang">🌍 ' + escapeHtml(langs) + '</div>';
                html += '</div>';
            }
            list.innerHTML = html;
        })
        .catch(() => {
            list.innerHTML = '<div class="tts-voice-error">' + t('config.tts.piper_voice_error') + '</div>';
        });
}

function piperSelectVoice(name) {
    const input = document.getElementById('piper-voice-input');
    if (input) {
        input.value = name;
        input.dispatchEvent(new Event('input', { bubbles: true }));
    }
    piperCloseVoiceModal();
}

function piperCloseVoiceModal() {
    const overlay = document.getElementById('piper-voice-modal');
    if (overlay) overlay.classList.remove('active');
}
