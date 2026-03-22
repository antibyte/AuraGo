// cfg/tts.js — TTS config section with Piper voice browser

function renderTTSSection(section) {
    const data = configData['tts'] || {};
    const piperData = data.piper || {};
    const piperEnabled = piperData.enabled === true;
    const currentProvider = data.provider || '';

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Provider select ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.provider_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.provider_help') + '</div>';
    html += '<select class="field-input" data-path="tts.provider" onchange="ttsProviderChanged(this.value)">';
    html += '<option value=""' + (currentProvider === '' ? ' selected' : '') + '>— ' + t('config.tts.provider_none') + ' —</option>';
    html += '<option value="google"' + (currentProvider === 'google' ? ' selected' : '') + '>Google TTS</option>';
    html += '<option value="elevenlabs"' + (currentProvider === 'elevenlabs' ? ' selected' : '') + '>ElevenLabs</option>';
    html += '<option value="piper"' + (currentProvider === 'piper' ? ' selected' : '') + '>Piper (Local)</option>';
    html += '</select>';
    html += '</div>';

    // ── Language ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.language_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.language_help') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.language" value="' + escapeAttr(data.language || '') + '" placeholder="de">';
    html += '</div>';

    // ── ElevenLabs fields (shown when provider=elevenlabs) ──
    const elData = data.elevenlabs || {};
    const showEL = currentProvider === 'elevenlabs';
    html += '<div id="tts-elevenlabs-section" style="' + (showEL ? '' : 'display:none;') + '">';
    html += '<div style="font-weight:600;font-size:0.92rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;margin:1.5rem 0 0.8rem;">🎤 ElevenLabs</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">API Key</div>';
    html += '<input class="field-input" type="password" data-path="tts.elevenlabs.api_key" value="' + escapeAttr(elData.api_key || '') + '" placeholder="sk-...">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">Voice ID</div>';
    html += '<input class="field-input" type="text" data-path="tts.elevenlabs.voice_id" value="' + escapeAttr(elData.voice_id || '') + '" placeholder="21m00Tcm4TlvDq8ikWAM">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">Model ID</div>';
    html += '<input class="field-input" type="text" data-path="tts.elevenlabs.model_id" value="' + escapeAttr(elData.model_id || '') + '" placeholder="eleven_monolingual_v1">';
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
    html += '<div id="piper-voice-modal" class="modal">';
    html += '<div class="modal-content" style="max-width:600px;">';
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
    const modal = document.getElementById('piper-voice-modal');
    if (!modal) return;
    modal.classList.add('active');
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
    const modal = document.getElementById('piper-voice-modal');
    if (modal) modal.classList.remove('active');
}
