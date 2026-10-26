// cfg/tts.js — TTS config section with Piper voice browser

const ttsCloudProviders = {
    xai: { name: 'xAI', voice: 'eve', voices: ['eve', 'ara', 'leo', 'rex', 'sal', 'carina', 'zagan', 'helix', 'orion', 'luna', 'iris', 'altair', 'zenith', 'perseus', 'helios', 'lux', 'kepler', 'rigel', 'cosmo', 'celeste', 'ursa', 'sirius', 'lumen', 'castor', 'naksh', 'atlas', 'aurora', 'liora'] },
    openai: { name: 'OpenAI', voice: 'coral', voices: ['alloy', 'ash', 'ballad', 'coral', 'echo', 'fable', 'nova', 'onyx', 'sage', 'shimmer', 'verse', 'marin', 'cedar'], model: 'gpt-4o-mini-tts', models: ['gpt-4o-mini-tts', 'tts-1', 'tts-1-hd'], notice: 'openai_retirement_help' },
    stepfun: { name: 'StepFun', voice: 'jingdiannvsheng', voices: ['jingdiannvsheng'], model: 'step-tts-mini', models: ['step-tts-mini', 'stepaudio-3-tts'] },
    resemble: { name: 'Resemble AI', notice: 'resemble_archive_help' }
};

function renderTTSSection(section) {
    const data = window.AuraConfigState?.get('tts') || configData['tts'] || {};
    const piperData = data.piper || {};
    const piperEnabled = piperData.enabled === true;
    const currentProvider = data.provider || '';
    const elData = data.elevenlabs || {};
    const mmData = data.minimax || {};
    const mistralData = data.mistral || {};
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
    html += '<option value="phonon"' + (currentProvider === 'phonon' ? ' selected' : '') + '>Phonon (CPU)</option>';
    html += '<option value="sanotts"' + (currentProvider === 'sanotts' ? ' selected' : '') + '>sanoTTS (CPU)</option>';
    html += '<option value="google"' + (currentProvider === 'google' ? ' selected' : '') + '>' + t('config.tts.provider_google') + '</option>';
    html += '<option value="elevenlabs"' + (currentProvider === 'elevenlabs' ? ' selected' : '') + '>' + t('config.tts.provider_elevenlabs') + '</option>';
    html += '<option value="minimax"' + (currentProvider === 'minimax' ? ' selected' : '') + '>' + t('config.tts.provider_minimax') + '</option>';
    html += '<option value="mistral"' + (currentProvider === 'mistral' ? ' selected' : '') + '>' + t('config.tts.provider_mistral') + '</option>';
    Object.entries(ttsCloudProviders).forEach(([id, provider]) => {
        html += '<option value="' + id + '"' + (currentProvider === id ? ' selected' : '') + '>' + provider.name + '</option>';
    });
    html += '<option value="piper"' + (currentProvider === 'piper' ? ' selected' : '') + '>' + t('config.tts.provider_piper') + '</option>';
    html += '<option value="supertonic"' + (currentProvider === 'supertonic' ? ' selected' : '') + '>' + t('config.tts.provider_supertonic') + '</option>';
    html += '</select>';
    html += '<div class="field-help" id="tts-local-help">' + (currentProvider === 'sanotts' ? t('config.tts.sanotts_help') : '') + '</div>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.language_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.language_help') + '</div>';
    html += ttsLanguageSelect('tts.language', data.language || 'auto');
    html += '</div>';

    const phononVoice = data.phonon?.voice_id || '';
    html += '<div id="tts-phonon-section" class="tts-provider-section' + (currentProvider === 'phonon' ? '' : ' is-hidden') + '">';
    html += '<div class="tts-subsection-title">Phonon</div>';
    html += '<div class="field-help">' + t('config.tts.phonon_help') + '</div>';
    html += '<div class="field-group"><label class="field-label" for="phonon-voice-select">' + t('config.tts.phonon_voice_label') + '</label>';
    html += '<select class="field-select" id="phonon-voice-select" data-path="tts.phonon.voice_id"><option value="">' + t('config.tts.phonon_voice_auto') + '</option>';
    if (phononVoice) html += '<option value="' + escapeAttr(phononVoice) + '" selected>' + escapeHtml(phononVoice) + '</option>';
    html += '</select><div class="field-help">' + t('config.tts.phonon_voice_help') + '</div></div>';
    html += '<p class="field-help" id="phonon-status" role="status"></p>';
    html += '<div class="field-group"><label class="field-label" for="phonon-language-filter">' + t('config.tts.phonon_catalog_label') + '</label>';
    html += '<select id="phonon-language-filter" class="field-select" onchange="ttsPhononRenderCatalog()"><option value="">' + t('config.tts.phonon_all_languages') + '</option>';
    for (const lang of ['de','en','fr','es','pt']) html += '<option value="' + lang + '">' + lang + '</option>';
    html += '</select><div id="phonon-voice-list"></div></div>';
    html += '<div class="field-help">' + t('config.tts.phonon_license_help') + '</div></div>';

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

    const showMistral = currentProvider === 'mistral';
    html += '<div id="tts-mistral-section" class="tts-provider-section' + (showMistral ? '' : ' is-hidden') + '">';
    html += '<div class="tts-subsection-title">Mistral AI Voxtral TTS</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.mistral_api_key_label') + '</div>';
    html += '<div class="adg-password-row">';
    html += '<div class="password-wrap adg-password-input">';
    html += '<input class="field-input adg-password-input" type="password" id="tts-mistral-api-key" value="' + escapeAttr(cfgSecretValue(mistralData.api_key)) + '" placeholder="' + escapeAttr(cfgSecretPlaceholder(mistralData.api_key, t('config.tts.mistral_api_key_placeholder'))) + '">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save adg-save-btn" onclick="ttsSaveMistralKey()">💾</button>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.mistral_voice_id_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.mistral_voice_id_help') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.mistral.voice_id" value="' + escapeAttr(mistralData.voice_id || '') + '" placeholder="' + t('config.tts.mistral_voice_id_placeholder') + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.tts.mistral_model_id_label') + '</div>';
    html += '<div class="field-help">' + t('config.tts.mistral_model_id_help') + '</div>';
    html += '<input class="field-input" type="text" data-path="tts.mistral.model_id" value="' + escapeAttr(mistralData.model_id || 'voxtral-mini-tts-2603') + '" placeholder="voxtral-mini-tts-2603">';
    html += '</div>';
    html += '</div>';

    Object.entries(ttsCloudProviders).forEach(([id, provider]) => {
        const settings = data[id] || {};
        const path = 'tts.' + id;
        html += '<div id="tts-' + id + '-section" class="tts-provider-section' + (currentProvider === id ? '' : ' is-hidden') + '">';
        html += '<div class="tts-subsection-title">' + provider.name + '</div>';
        if (provider.notice) html += '<div class="adg-status-banner is-warning" role="note">' + t('config.tts.' + provider.notice) + '</div>';
        html += '<div class="field-group"><label class="field-label" for="tts-' + id + '-api-key">' + t('config.tts.cloud_api_key_label') + '</label>';
        html += '<div class="adg-password-row"><div class="password-wrap adg-password-input">';
        html += '<input class="field-input adg-password-input" type="password" id="tts-' + id + '-api-key" value="' + escapeAttr(cfgSecretValue(settings.api_key)) + '" placeholder="' + escapeAttr(cfgSecretPlaceholder(settings.api_key, provider.name)) + '"' + cfgNoAutofillAttrs(true) + '>';
        html += '</div><button type="button" class="btn-save adg-save-btn" onclick="ttsSaveCloudKey(\'' + id + '\')">' + t('config.tts.cloud_save_key') + '</button></div></div>';
        html += '<div class="field-group"><label class="field-label" for="tts-' + id + '-voice">' + t('config.tts.cloud_voice_label') + '</label>';
        if (provider.voices) {
            html += ttsCloudSelect(path + '.voice_id', settings.voice_id || provider.voice, provider.voices, 'tts-' + id + '-voice');
        } else {
            html += '<input class="field-input" id="tts-' + id + '-voice" type="text" data-path="' + path + '.voice_id" value="' + escapeAttr(settings.voice_id || '') + '" placeholder="voice_uuid">';
            html += '<div class="field-help">' + t('config.tts.resemble_voice_help') + '</div>';
        }
        html += '</div>';
        if (provider.models) {
            html += '<div class="field-group"><label class="field-label" for="tts-' + id + '-model">' + t('config.tts.cloud_model_label') + '</label>';
            html += ttsCloudSelect(path + '.model_id', settings.model_id || provider.model, provider.models, 'tts-' + id + '-model');
            html += '<div class="field-help">' + t('config.tts.cloud_model_help') + '</div></div>';
        }
        html += '</div>';
    });

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

    if (currentProvider === 'phonon') ttsPhononLoadVoices();

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
    const languages = ['auto', 'de', 'en', 'fr', 'es', 'it', 'pt', 'ar', 'cs', 'id', 'ro', 'ru', 'tr', 'vi', 'nl', 'ja', 'zh', 'hi', 'ne'];
    const customOption = typeof CFG_OPTION_OTHER_CUSTOM === 'string' ? CFG_OPTION_OTHER_CUSTOM : 'Other / Custom';
    const current = String(selected || '').trim();
    const isCustom = current && !languages.includes(current);
    let html = '<select class="field-select" data-path="' + escapeAttr(path) + '" onchange="cfgToggleCustomInput(this)">';
    languages.forEach(code => {
        html += '<option value="' + code + '"' + (current === code ? ' selected' : '') + '>' + (code === 'auto' ? t('config.tts.language_auto') : code) + '</option>';
    });
    html += '<option value="' + escapeAttr(customOption) + '"' + (isCustom ? ' selected' : '') + '>' + cfgFieldOptionLabel(customOption) + '</option>';
    html += '</select>';
    html += '<input class="field-input cfg-custom-input' + (isCustom ? '' : ' is-hidden') + '" type="text" data-custom-for="' + escapeAttr(path) + '" value="' + escapeAttr(isCustom ? current : '') + '" placeholder="' + escapeAttr(t('config.tts.language_placeholder')) + '">';
    html += '<div class="field-help">' + t('config.tts.language_custom_help') + '</div>';
    return html;
}

function ttsProviderChanged(val) {
    const localHelp = document.getElementById('tts-local-help');
    if (localHelp) localHelp.textContent = val === 'sanotts' ? t('config.tts.sanotts_help') : '';
    const phononSection = document.getElementById('tts-phonon-section');
    if (phononSection) {
        phononSection.classList.toggle('is-hidden', val !== 'phonon');
        if (val === 'phonon') ttsPhononLoadVoices();
    }
    Object.keys(ttsCloudProviders).forEach(id => {
        const section = document.getElementById('tts-' + id + '-section');
        if (section) section.classList.toggle('is-hidden', val !== id);
    });
    const elSection = document.getElementById('tts-elevenlabs-section');
    if (elSection) elSection.classList.toggle('is-hidden', val !== 'elevenlabs');
    const mmSection = document.getElementById('tts-minimax-section');
    if (mmSection) mmSection.classList.toggle('is-hidden', val !== 'minimax');
    const mistralSection = document.getElementById('tts-mistral-section');
    if (mistralSection) mistralSection.classList.toggle('is-hidden', val !== 'mistral');
    const supertonicSection = document.getElementById('tts-supertonic-section');
    if (supertonicSection) {
        supertonicSection.classList.toggle('is-hidden', val !== 'supertonic');
        if (val === 'supertonic') supertonicCheckStatus();
    }
}

async function ttsPhononLoadVoices() {
    const select = document.getElementById('phonon-voice-select');
    const list = document.getElementById('phonon-voice-list');
    if (!select || !list) return;
    const controller = new AbortController();
    list._phononRequest?.abort();
    list._phononRequest = controller;
    try {
        const response = await fetch('/api/tts/phonon/voices', {signal: controller.signal});
        if (!response.ok) throw new Error('catalog');
        const data = await response.json();
        if (controller.signal.aborted || document.getElementById('phonon-voice-list') !== list) return;
        const selected = select.value;
        list._phononVoices = data.voices || [];
        if (!list._phononLoaded) {
            const language = String(data.language || '').toLowerCase().replace('_','-').split('-')[0];
            const filter = document.getElementById('phonon-language-filter');
            if (filter && Array.from(filter.options).some(option => option.value === language)) filter.value = language;
            list._phononLoaded = true;
        }
        select.replaceChildren(new Option(t('config.tts.phonon_voice_auto'), ''));
        for (const voice of list._phononVoices) {
            if (voice.installed || voice.id === selected) {
                const option = new Option(voice.name + ' · ' + voice.locale, voice.id);
                select.appendChild(option);
            }
        }
        if (selected && !Array.from(select.options).some(option => option.value === selected)) select.appendChild(new Option(selected, selected));
        select.value = selected;
        const status = document.getElementById('phonon-status');
        if (status) status.textContent = t(data.ready ? 'config.tts.phonon_ready' : 'config.tts.phonon_fallback');
        ttsPhononRenderCatalog();
    } catch (error) {
        if (error.name !== 'AbortError' && document.getElementById('phonon-voice-list') === list) list.textContent = t('config.tts.phonon_catalog_error');
    }
}

function ttsPhononRenderCatalog() {
    const list = document.getElementById('phonon-voice-list');
    if (!list) return;
    const language = document.getElementById('phonon-language-filter')?.value || '';
    list.replaceChildren();
    for (const voice of list._phononVoices || []) {
        if (language && voice.language !== language) continue;
        const row = document.createElement('div');
        row.className = 'tts-voice-item tts-phonon-voice-item';
        const name = document.createElement('div');
        name.className = 'tts-voice-item-name';
        const gender = voice.gender === 'male' || voice.gender === 'female' ? ' · ' + t('config.tts.phonon_' + voice.gender) : '';
        name.textContent = voice.name + ' · ' + voice.locale + gender;
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'btn-save';
        button.textContent = t(voice.installed ? 'config.tts.phonon_installed' : 'config.tts.phonon_download');
        button.disabled = voice.installed === true;
        button.setAttribute('aria-label', button.textContent + ': ' + voice.name);
        button.addEventListener('click', async () => {
            button.disabled = true;
            button.textContent = t('config.tts.phonon_downloading');
            try {
                const response = await fetch('/api/tts/phonon/voices/' + encodeURIComponent(voice.id) + '/download', {method:'POST'});
                if (!response.ok) throw new Error('download');
                if (document.getElementById('phonon-voice-list') === list) await ttsPhononLoadVoices();
            } catch (_) {
                if (button.isConnected) {
                    button.disabled = false;
                    button.textContent = t('config.tts.phonon_download');
                    const status = document.getElementById('phonon-status');
                    if (status) status.textContent = t('config.tts.phonon_download_error');
                }
            }
        });
        row.append(name, button);
        list.appendChild(row);
    }
}

function ttsCloudSelect(path, value, options, id) {
    const custom = !options.includes(value);
    let html = '<select class="field-select" id="' + id + '" data-path="' + path + '" onchange="cfgToggleCustomInput(this)">';
    options.forEach(option => {
        html += '<option value="' + escapeAttr(option) + '"' + (option === value ? ' selected' : '') + '>' + escapeHtml(option) + '</option>';
    });
    html += '<option value="' + escapeAttr(CFG_OPTION_OTHER_CUSTOM) + '"' + (custom ? ' selected' : '') + '>' + cfgFieldOptionLabel(CFG_OPTION_OTHER_CUSTOM) + '</option></select>';
    html += '<input class="field-input cfg-custom-input' + (custom ? '' : ' is-hidden') + '" type="text" data-custom-for="' + path + '" aria-label="' + escapeAttr(t('config.field.other_custom_option')) + '" value="' + escapeAttr(custom ? value : '') + '">';
    return html;
}

async function ttsSaveCloudKey(provider) {
    if (!Object.hasOwn(ttsCloudProviders, provider)) return;
    const input = document.getElementById('tts-' + provider + '-api-key');
    const value = input ? input.value.trim() : '';
    if (!value) {
        showToast(t('config.tts.cloud_api_key_required'), 'warn');
        return;
    }
    try {
        const response = await fetch('/api/vault/secrets', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key: 'tts_' + provider + '_api_key', value })
        });
        const result = await response.json();
        if (!response.ok || !(result.status === 'ok' || result.success)) throw new Error('vault save failed');
        cfgMarkSecretStored(input, 'tts.' + provider + '.api_key');
        window.AuraConfigState?.markSaved('tts.' + provider + '.api_key', configData.tts[provider].api_key);
        showToast(t('config.tts.cloud_api_key_saved'), 'success');
    } catch (_) {
        showToast(t('config.tts.cloud_api_key_save_failed'), 'error');
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

function ttsSaveMistralKey() {
    const input = document.getElementById('tts-mistral-api-key');
    const value = input ? input.value.trim() : '';
    if (!value) {
        showToast(t('config.tts.mistral_api_key_required'), 'warn');
        return;
    }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'tts_mistral_api_key', value })
    })
        .then(r => r.json())
        .then(res => {
            if (res.status === 'ok' || res.success) {
                showToast(t('config.tts.mistral_api_key_saved'), 'success');
                cfgMarkSecretStored(input, 'tts.mistral.api_key');
            } else {
                showToast(res.message || t('config.tts.mistral_api_key_save_failed'), 'error');
            }
        })
        .catch(() => showToast(t('config.tts.mistral_api_key_save_failed'), 'error'));
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

async function supertonicLoadStyles(list, attempt = 0) {
    try {
        const response = await fetch('/api/supertonic/styles', { cache: 'no-store' });
        const res = await response.json();
        if (res.code === 'supertonic_starting' && attempt < 90) {
            const overlay = document.getElementById('supertonic-style-modal');
            if (!overlay || !overlay.classList.contains('active')) return;
            if (list) {
                list.innerHTML = '<div class="tts-voice-loading">' + t('config.tts.supertonic_loading_voices') + '</div>';
            }
            const headerDelay = Number.parseInt(response.headers.get('Retry-After') || '', 10);
            const bodyDelay = Number(res.retry_after_seconds || 0);
            const delaySeconds = Math.min(10, Math.max(1, headerDelay || bodyDelay || 2));
            window.setTimeout(() => supertonicLoadStyles(list, attempt + 1), delaySeconds * 1000);
            return;
        }
        if (!response.ok || res.error) {
            if (list) list.innerHTML = '<div class="tts-voice-error">' + t('config.tts.supertonic_voice_error') + '</div>';
            return;
        }
        const styles = res.styles || [];
        if (styles.length === 0) {
            if (list) list.innerHTML = '<div class="tts-voice-empty">' + t('config.tts.supertonic_no_voices') + '</div>';
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
        if (list) list.innerHTML = html;
    } catch (_) {
        if (list) list.innerHTML = '<div class="tts-voice-error">' + t('config.tts.supertonic_voice_error') + '</div>';
    }
}

function supertonicBrowseStyles() {
    const overlay = document.getElementById('supertonic-style-modal');
    if (!overlay) return;
    overlay.classList.add('active');
    const list = document.getElementById('supertonic-style-list');
    if (list) list.innerHTML = '<div class="tts-voice-loading">' + t('config.tts.supertonic_loading_voices') + '</div>';

    supertonicLoadStyles(list);
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
