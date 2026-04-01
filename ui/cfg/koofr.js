// cfg/koofr.js — Koofr integration config panel

function renderKoofrSection(section) {
    const data = configData.koofr || {};
    const enabled = data.enabled === true;
    const readonly = data.readonly === true;
    const hasAppPassword = data.app_password === '••••••••';

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.koofr.enabled_label') + '</div>';
    html += '<div class="field-help">' + t('help.koofr.enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabled ? ' on' : '') + '" data-path="koofr.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.koofr.readonly_label') + '</div>';
    html += '<div class="field-help">' + t('help.koofr.read_only') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readonly ? ' on' : '') + '" data-path="koofr.readonly" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readonly ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.koofr.base_url_label') + '</div>';
    html += '<div class="field-help">' + t('help.koofr.base_url') + '</div>';
    html += '<input class="field-input" type="text" data-path="koofr.base_url" value="' + escapeAttr(data.base_url || 'https://app.koofr.net') + '" placeholder="https://app.koofr.net">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.koofr.username_label') + '</div>';
    html += '<div class="field-help">' + t('help.koofr.username') + '</div>';
    html += '<input class="field-input" type="text" data-path="koofr.username" value="' + escapeAttr(data.username || '') + '" placeholder="name@example.com">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.koofr.app_password_label') + '</div>';
    html += '<div class="field-help">' + t('help.koofr.app_password') + '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;flex-wrap:wrap;">';
        html += '<div class="password-wrap" style="flex:1;min-width:240px;">';
        html += '<input class="field-input" type="password" id="koofr-app-password" placeholder="' + escapeAttr(hasAppPassword ? '••••••••' : t('config.koofr.app_password_placeholder')) + '" style="flex:1;min-width:240px;">';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;white-space:nowrap;" onclick="koofrSaveAppPassword()">💾 ' + t('config.koofr.save_vault') + '</button>';
    html += '</div></div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
}

function koofrSaveAppPassword() {
    const input = document.getElementById('koofr-app-password');
    const value = input ? input.value.trim() : '';
    if (!value) {
        showToast(t('config.koofr.app_password_empty'), 'error');
        return;
    }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'koofr_password', value })
    })
        .then(r => r.json())
        .then(res => {
            if (res.status === 'ok' || res.success) {
                showToast(t('config.koofr.app_password_saved'), 'success');
                if (input) {
                    input.value = '';
                    input.placeholder = '••••••••';
                }
                if (!configData.koofr) configData.koofr = {};
                configData.koofr.app_password = '••••••••';
            } else {
                showToast(res.message || t('config.koofr.app_password_save_failed'), 'error');
            }
        })
        .catch(() => showToast(t('config.koofr.app_password_save_failed'), 'error'));
}
