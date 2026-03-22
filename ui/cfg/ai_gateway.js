// cfg/ai_gateway.js — AI Gateway section module

function renderAIGatewaySection(section) {
    const gw = configData.ai_gateway || {};
    const enabled = gw.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // Info banner
    html += `<div class="wh-notice ai-gw-info-notice">
        <span>🌩️</span>
        <div><small>${t('config.ai_gateway.info')}</small></div>
    </div>`;

    // Enabled toggle
    html += `<div class="ai-gw-toggle-row">
        <span class="ai-gw-toggle-label">${t('config.ai_gateway.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="ai_gateway.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>☁️</span>
            <div>
                <strong>${t('config.ai_gateway.disabled_notice')}</strong><br>
                <small>${t('config.ai_gateway.disabled_desc')}</small>
            </div>
        </div>`;
    }

    // Config fields
    html += `<div class="ai-gw-grid">`;

    // Account ID
    html += `<label class="ai-gw-label">
        <span class="ai-gw-label-text">${t('config.ai_gateway.account_id')}</span>
        <input class="cfg-input ai-gw-input-spaced" data-path="ai_gateway.account_id" value="${escapeAttr(gw.account_id || '')}"
            placeholder="${escapeAttr(t('config.ai_gateway.account_id_placeholder'))}"
            onchange="setNestedValue(configData,'ai_gateway.account_id',this.value);setDirty(true)">
    </label>`;

    // Gateway ID
    html += `<label class="ai-gw-label">
        <span class="ai-gw-label-text">${t('config.ai_gateway.gateway_id')}</span>
        <input class="cfg-input ai-gw-input-spaced" data-path="ai_gateway.gateway_id" value="${escapeAttr(gw.gateway_id || '')}"
            placeholder="${escapeAttr(t('config.ai_gateway.gateway_id_placeholder'))}"
            onchange="setNestedValue(configData,'ai_gateway.gateway_id',this.value);setDirty(true)">
    </label>`;

    html += `</div>`; // close grid

    html += `</div>`; // close section
    document.getElementById('content').innerHTML = html;
}
