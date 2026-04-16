// cfg/tts.js — TTS config section with Piper voice browser

function renderTTSSection(section) {
    const data = configData['tts'] || {};
    const piperData = data.piper || {};
    const piperEnabled = piperData.enabled === true;
    const currentProvider = data.provider || '';
    const elData = data.elevenlabs || {};
    const mmData = data.minimax || {};

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Provider select ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.provider_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.provider_help') + '</div>';
    html += '<select class="field-input" data-path="tts.provider" onchange="ttsProviderChanged(this.value)">';
    html += '<option value=""' + (currentProvider === '' ? ' selected' : '') + '>— ' + t('config.tts.provider_none') + ' —</option>';
    html += '<option value="google"' + (currentProvider === 'google' ? ' selected' : '') + '>' + t('config.tts.provider_google') + '</option>';
    html += '<option value="elevenlabs"' + (currentProvider === 'elevenlabs' ? ' selected' : '') + '>' + t('config.tts.provider_elevenlabs') + '</option>';
    html += '<option value="minimax"' + (currentProvider === 'minimax' ? ' selected' : '') + '>' + t('config.tts.provider_minimax') + '</option>';
    html += '<option value="piper"' + (currentProvider === 'piper' ? ' selected' : '') + '>' + t('config.tts.provider_piper') + '</option>';
    html += '</select>';
    html += '</div>';

    // ── Language ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.language_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.language_help') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.language" value="' + escapeAttr(data.language || '') + '" placeholder="' + t('config.tts.language_placeholder') + '">';
    html += '</div>';

    // ── ElevenLabs fields (shown when provider=elevenlabs) ──
    const showEL = currentProvider === 'elevenlabs';
    html += '<div id="tts-elevenlabs-section" style="' + (showEL ? '' : 'display:none;') + '">';
    html += '<div style="font-weight:600;font-size:0.92rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;margin:1.5rem 0 0.8rem;">🎤 ElevenLabs</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.elevenlabs_api_key_label') + '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;flex-wrap:wrap;">';
    html += '<div class="password-wrap" style="flex:1;min-width:240px;">';
    html += '<input class="field-input" type="password" id="tts-elevenlabs-api-key" value="' + escapeAttr(cfgSecretValue(elData.api_key)) + '" placeholder="' + escapeAttr(cfgSecretPlaceholder(elData.api_key, t('config.tts.elevenlabs_api_key_placeholder'))) + '">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;white-space:nowrap;" onclick="ttsSaveElevenLabsKey()">💾</button>';
    html += '</div>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.elevenlabs_voice_id_label') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.elevenlabs.voice_id" value="' + escapeAttr(elData.voice_id || '') + '" placeholder="' + t('config.tts.elevenlabs_voice_id_placeholder') + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.elevenlabs_model_id_label') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.elevenlabs.model_id" value="' + escapeAttr(elData.model_id || '') + '" placeholder="' + t('config.tts.elevenlabs_model_id_placeholder') + '">';
    html += '</div>';
    html += '</div>';

    // ── MiniMax fields (shown when provider=minimax) ──
    const showMM = currentProvider === 'minimax';
    html += '<div id="tts-minimax-section" style="' + (showMM ? '' : 'display:none;') + '">';
    html += '<div style="font-weight:600;font-size:0.92rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;margin:1.5rem 0 0.8rem;">🎵 MiniMax TTS</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.minimax_api_key_label') + '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;flex-wrap:wrap;">';
    html += '<div class="password-wrap" style="flex:1;min-width:240px;">';
    html += '<input class="field-input" type="password" id="tts-minimax-api-key" value="' + escapeAttr(cfgSecretValue(mmData.api_key)) + '" placeholder="' + escapeAttr(cfgSecretPlaceholder(mmData.api_key, t('config.tts.minimax_api_key_placeholder'))) + '">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;white-space:nowrap;" onclick="ttsSaveMiniMaxKey()">💾</button>';
    html += '</div>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.minimax_voice_id_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.minimax_voice_id_help') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.minimax.voice_id" value="' + escapeAttr(mmData.voice_id || '') + '" placeholder="' + t('config.tts.minimax_voice_id_placeholder') + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.minimax_model_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.minimax_model_help') + '</div>';
    html += '<select class="field-input" data-path="tts.minimax.model_id">';
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

    // ── Piper section ──
    html += '<div style="font-weight:600;font-size:0.92rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;margin:1.5rem 0 0.8rem;">🗣️ Piper TTS (Local Docker)</div>';

    // Status banner
    html += '<div id="piper-status-banner" style="margin-bottom:1rem;padding:0.8rem 1rem;border-radius:10px;font-size:0.84rem;background:var(--bg-tertiary);color:var(--text-secondary);">' + t('config.tts.piper_checking') + '</div>';

    // Enabled toggle
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.piper_enabled_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.piper_enabled_help') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (piperEnabled ? ' on' : '') + '" data-path="tts.piper.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (piperEnabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // Voice
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.piper_voice_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.piper_voice_help') + '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;">';
    html += '<input class="field-input" type="text" id="piper-voice-input" data-path="tts.piper.voice" value="' + escapeAttr(piperData.voice || 'de_DE-thorsten-high') + '" style="flex:1;">';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;" onclick="piperBrowseVoices()">🔍 ' + t('config.tts.piper_browse_voices') + '</button>';
    html += '</div></div>';

    // Speaker ID
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.piper_speaker_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.piper_speaker_help') + '</div>';
    html += '<input class="field-input" type="number" data-path="tts.piper.speaker_id" value="' + (piperData.speaker_id || 0) + '" min="0">';
    html += '</div>';

    // Container Port
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.piper_port_label') + '</div>';
    html += '<input class="field-input" type="number" data-path="tts.piper.container_port" value="' + (piperData.container_port || 10200) + '" min="1" max="65535">';
    html += '</div>';

    // Docker Image
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.piper_image_label') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.piper.image" value="' + escapeAttr(piperData.image || 'rhasspy/wyoming-piper:latest') + '">';
    html += '</div>';

    // Data Path
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.piper_data_path_label') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.piper.data_path" value="' + escapeAttr(piperData.data_path || 'data/piper') + '">';
    html += '</div>';

    // Voice browser modal (hidden by default)
    html += '<div id="piper-voice-modal" class="modal-overlay" onclick="if(event.target===this)piperCloseVoiceModal()">';
    html += '<div class="modal" style="max-width:600px;">';
    html += '<div class="modal-header"><span>' + t('config.tts.piper_voice_browser_title') + '</span><span class="modal-close" onclick="piperCloseVoiceModal()">&times;</span></div>';
    html += '<div class="modal-body" style="max-height:400px;overflow-y:auto;">';
    html += '<div id="piper-voice-list" style="padding:0.5rem;">' + t('config.tts.piper_loading_voices') + '</div>';
    html += '</div></div></div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;

    // Auto-check Piper status
    if (piperEnabled) {
        piperCheckStatus();
    } else {
        const banner = document.getElementById('piper-status-banner');
        if (banner) {
            banner.textContent = '⚪ ' + t('config.tts.piper_status_disabled');
            banner.style.background = 'var(--bg-tertiary)';
        }
    }
}

function ttsProviderChanged(val) {
    const elSection = document.getElementById('tts-elevenlabs-section');
    if (elSection) elSection.style.display = val === 'elevenlabs' ? '' : 'none';
    const mmSection = document.getElementById('tts-minimax-section');
    if (mmSection) mmSection.style.display = val === 'minimax' ? '' : 'none';
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
    const banner = document.getElementById('piper-status-banner');
    if (!banner) return;
    banner.style.background = 'var(--bg-tertiary)';
    banner.style.color = 'var(--text-secondary)';
    banner.textContent = '⏳ ' + t('config.tts.piper_checking');

    fetch('/api/piper/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                banner.textContent = '⚪ ' + t('config.tts.piper_status_disabled');
                banner.style.background = 'var(--bg-tertiary)';
            } else if (res.status === 'running') {
                banner.textContent = '🟢 ' + t('config.tts.piper_status_running');
                banner.style.background = 'rgba(72,199,142,0.1)';
                banner.style.color = '#48c78e';
                if (res.voice) banner.textContent += ' — ' + res.voice;
            } else if (res.status === 'stopped') {
                banner.textContent = '🟡 ' + t('config.tts.piper_status_stopped');
                banner.style.background = 'rgba(255,183,77,0.1)';
                banner.style.color = '#ffb74d';
            } else {
                banner.textContent = '🔴 ' + t('config.tts.piper_status_error') + (res.error ? ': ' + res.error : '');
                banner.style.background = 'rgba(255,82,82,0.08)';
                banner.style.color = '#ff5252';
            }
        })
        .catch(() => {
            banner.textContent = '🔴 ' + t('config.tts.piper_status_error');
            banner.style.background = 'rgba(255,82,82,0.08)';
            banner.style.color = '#ff5252';
        });
}

function piperBrowseVoices() {
    const overlay = document.getElementById('piper-voice-modal');
    if (!overlay) return;
    overlay.classList.add('active');
    const list = document.getElementById('piper-voice-list');
    if (list) list.innerHTML = '<div style="text-align:center;padding:1rem;">' + t('config.tts.piper_loading_voices') + '</div>';

    fetch('/api/piper/voices')
        .then(r => r.json())
        .then(res => {
            if (res.error) {
                list.innerHTML = '<div style="color:var(--danger);padding:1rem;">' + escapeAttr(res.error) + '</div>';
                return;
            }
            const voices = res.voices || [];
            if (voices.length === 0) {
                list.innerHTML = '<div style="padding:1rem;color:var(--text-secondary);">' + t('config.tts.piper_no_voices') + '</div>';
                return;
            }
            let html = '';
            for (const v of voices) {
                const langs = (v.languages || []).join(', ');
                const installed = v.installed ? '✅' : '';
                html += '<div class="voice-item" style="padding:0.6rem 0.8rem;border-bottom:1px solid var(--border-subtle);cursor:pointer;border-radius:6px;" onmouseover="this.style.background=\'var(--bg-hover)\'" onmouseout="this.style.background=\'\'" onclick="piperSelectVoice(\'' + escapeAttr(v.name) + '\')">';
                html += '<div style="font-weight:600;font-size:0.9rem;">' + installed + ' ' + escapeAttr(v.name) + '</div>';
                if (v.description) html += '<div style="font-size:0.8rem;color:var(--text-secondary);">' + escapeAttr(v.description) + '</div>';
                if (langs) html += '<div style="font-size:0.78rem;color:var(--text-tertiary);">🌍 ' + escapeAttr(langs) + '</div>';
                html += '</div>';
            }
            list.innerHTML = html;
        })
        .catch(err => {
            list.innerHTML = '<div style="color:var(--danger);padding:1rem;">' + t('config.tts.piper_voice_error') + '</div>';
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
