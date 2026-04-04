// cfg/music_generation.js — Music Generation config section

function renderMusicGenerationSection(section) {
    const data = configData['music_generation'] || {};
    const enabled = data.enabled === true;
    const currentProvider = data.provider || 'minimax';
    const mmData = data.minimax || {};
    const glData = data.google_lyria || {};
    const hasMiniMaxKey = mmData.api_key === '••••••••';
    const hasGoogleLyriaKey = glData.api_key === '••••••••';

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Enabled toggle ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.music_gen.enabled_label') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabled ? ' on' : '') + '" data-path="music_generation.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    if (!enabled) {
        html += '<div class="cfg-notice">' + t('config.music_gen.disabled_notice') + '</div>';
        html += '</div>';
        document.getElementById('content').innerHTML = html;
        return;
    }

    // ── Provider select ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.music_gen.provider_label') + '</div>';
    html += '<div class="field-help">' + t('config.music_gen.provider_help') + '</div>';
    html += '<select class="field-input" data-path="music_generation.provider" onchange="musicProviderChanged(this.value)">';
    html += '<option value="minimax"' + (currentProvider === 'minimax' ? ' selected' : '') + '>MiniMax</option>';
    html += '<option value="google_lyria"' + (currentProvider === 'google_lyria' ? ' selected' : '') + '>Google Lyria</option>';
    html += '</select>';
    html += '</div>';

    // ── Max daily limit ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.music_gen.max_daily_label') + '</div>';
    html += '<div class="field-help">' + t('config.music_gen.max_daily_help') + '</div>';
    html += '<input class="field-input" type="number" data-path="music_generation.max_daily" value="' + (data.max_daily || 0) + '" min="0" placeholder="0">';
    html += '</div>';

    // ── MiniMax section ──
    const showMM = currentProvider === 'minimax';
    html += '<div id="music-minimax-section" style="' + (showMM ? '' : 'display:none;') + '">';
    html += '<div style="font-weight:600;font-size:0.92rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;margin:1.5rem 0 0.8rem;">🎵 MiniMax</div>';

    // API Key
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.music_gen.api_key_label') + '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;flex-wrap:wrap;">';
    html += '<div class="password-wrap" style="flex:1;min-width:240px;">';
    html += '<input class="field-input" type="password" id="music-minimax-api-key" value="" placeholder="' + escapeAttr(hasMiniMaxKey ? '••••••••' : 'Enter MiniMax API key') + '">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;white-space:nowrap;" onclick="musicSaveMiniMaxKey()">💾</button>';
    html += '</div>';
    html += '</div>';

    // Model
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.music_gen.model_label') + '</div>';
    html += '<select class="field-input" data-path="music_generation.minimax.model">';
    html += '<option value="music-2.5+"' + (mmData.model === 'music-2.5+' || !mmData.model ? ' selected' : '') + '>music-2.5+ (' + t('config.music_gen.recommended') + ')</option>';
    html += '<option value="music-2.5"' + (mmData.model === 'music-2.5' ? ' selected' : '') + '>music-2.5</option>';
    html += '</select>';
    html += '</div>';

    // Test button
    html += '<div class="field-group">';
    html += '<button class="btn-save" id="music-test-minimax-btn" onclick="musicTestConnection(\'minimax\')" style="padding:0.5rem 1.2rem;">';
    html += '🔌 ' + t('config.music_gen.test_btn') + '</button>';
    html += '<span id="music-test-minimax-result" style="margin-left:0.8rem;font-size:0.85rem;"></span>';
    html += '</div>';
    html += '</div>';

    // ── Google Lyria section ──
    const showGL = currentProvider === 'google_lyria';
    html += '<div id="music-google-lyria-section" style="' + (showGL ? '' : 'display:none;') + '">';
    html += '<div style="font-weight:600;font-size:0.92rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;margin:1.5rem 0 0.8rem;">🎶 Google Lyria</div>';

    // API Key
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.music_gen.api_key_label') + '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;flex-wrap:wrap;">';
    html += '<div class="password-wrap" style="flex:1;min-width:240px;">';
    html += '<input class="field-input" type="password" id="music-google-lyria-api-key" value="" placeholder="' + escapeAttr(hasGoogleLyriaKey ? '••••••••' : 'Enter Google API key') + '">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;white-space:nowrap;" onclick="musicSaveGoogleLyriaKey()">💾</button>';
    html += '</div>';
    html += '</div>';

    // Model
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.music_gen.model_label') + '</div>';
    html += '<select class="field-input" data-path="music_generation.google_lyria.model">';
    html += '<option value="lyria-3-clip-preview"' + (glData.model === 'lyria-3-clip-preview' || !glData.model ? ' selected' : '') + '>lyria-3-clip-preview (30s)</option>';
    html += '<option value="lyria-3-pro-preview"' + (glData.model === 'lyria-3-pro-preview' ? ' selected' : '') + '>lyria-3-pro-preview (1–5 min)</option>';
    html += '</select>';
    html += '</div>';

    // Test button
    html += '<div class="field-group">';
    html += '<button class="btn-save" id="music-test-google-btn" onclick="musicTestConnection(\'google_lyria\')" style="padding:0.5rem 1.2rem;">';
    html += '🔌 ' + t('config.music_gen.test_btn') + '</button>';
    html += '<span id="music-test-google-result" style="margin-left:0.8rem;font-size:0.85rem;"></span>';
    html += '</div>';
    html += '</div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
}

function musicProviderChanged(val) {
    const mmSection = document.getElementById('music-minimax-section');
    if (mmSection) mmSection.style.display = val === 'minimax' ? '' : 'none';
    const glSection = document.getElementById('music-google-lyria-section');
    if (glSection) glSection.style.display = val === 'google_lyria' ? '' : 'none';
}

function musicSaveMiniMaxKey() {
    const input = document.getElementById('music-minimax-api-key');
    const value = input ? input.value.trim() : '';
    if (!value) { showToast(t('config.music_gen.key_required'), 'warn'); return; }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'music_minimax_api_key', value })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            showToast(t('config.music_gen.key_saved'), 'success');
            if (input) { input.value = ''; input.placeholder = '••••••••'; }
        } else {
            showToast(res.message || t('config.music_gen.key_save_failed'), 'error');
        }
    })
    .catch(() => showToast(t('config.music_gen.key_save_failed'), 'error'));
}

function musicSaveGoogleLyriaKey() {
    const input = document.getElementById('music-google-lyria-api-key');
    const value = input ? input.value.trim() : '';
    if (!value) { showToast(t('config.music_gen.key_required'), 'warn'); return; }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'music_google_lyria_api_key', value })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            showToast(t('config.music_gen.key_saved'), 'success');
            if (input) { input.value = ''; input.placeholder = '••••••••'; }
        } else {
            showToast(res.message || t('config.music_gen.key_save_failed'), 'error');
        }
    })
    .catch(() => showToast(t('config.music_gen.key_save_failed'), 'error'));
}

function musicTestConnection(provider) {
    const btnId = provider === 'minimax' ? 'music-test-minimax-btn' : 'music-test-google-btn';
    const resultId = provider === 'minimax' ? 'music-test-minimax-result' : 'music-test-google-result';
    const btn = document.getElementById(btnId);
    const result = document.getElementById(resultId);

    if (btn) btn.disabled = true;
    if (result) { result.textContent = '⏳ ' + t('config.music_gen.testing'); result.style.color = 'var(--text-secondary)'; }

    fetch('/api/music-generation/test?provider=' + encodeURIComponent(provider))
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok') {
            if (result) { result.textContent = '✅ ' + t('config.music_gen.test_success'); result.style.color = 'var(--success)'; }
        } else {
            if (result) { result.textContent = '❌ ' + (res.message || t('config.music_gen.test_failed')); result.style.color = 'var(--error)'; }
        }
    })
    .catch(err => {
        if (result) { result.textContent = '❌ ' + err.message; result.style.color = 'var(--error)'; }
    })
    .finally(() => { if (btn) btn.disabled = false; });
}
