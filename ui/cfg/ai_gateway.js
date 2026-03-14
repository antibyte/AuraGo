// cfg/ai_gateway.js — AI Gateway section module

function renderAIGatewaySection(section) {
    const gw = configData.ai_gateway || {};
    const enabled = gw.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // Info banner
    html += `<div class="wh-notice" style="border-color:var(--accent);background:rgba(99,102,241,0.06);">
        <span>🌩️</span>
        <div><small>${t('config.ai_gateway.info')}</small></div>
    </div>`;

    // Enabled toggle
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.ai_gateway.enabled_label')}</span>
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
    html += `<div style="display:grid;grid-template-columns:1fr 1fr;gap:0.8rem 1.2rem;margin-top:1rem;">`;

    // Account ID
    html += `<label style="display:block;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.ai_gateway.account_id')}</span>
        <input class="cfg-input" data-path="ai_gateway.account_id" value="${escapeAttr(gw.account_id || '')}"
            placeholder="${escapeAttr(t('config.ai_gateway.account_id_placeholder'))}"
            style="width:100%;margin-top:0.2rem;"
            onchange="setNestedValue(configData,'ai_gateway.account_id',this.value);setDirty(true)">
    </label>`;

    // Gateway ID
    html += `<label style="display:block;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.ai_gateway.gateway_id')}</span>
        <input class="cfg-input" data-path="ai_gateway.gateway_id" value="${escapeAttr(gw.gateway_id || '')}"
            placeholder="${escapeAttr(t('config.ai_gateway.gateway_id_placeholder'))}"
            style="width:100%;margin-top:0.2rem;"
            onchange="setNestedValue(configData,'ai_gateway.gateway_id',this.value);setDirty(true)">
    </label>`;

    html += `</div>`; // close grid

    html += `</div>`; // close section
    document.getElementById('content').innerHTML = html;
}
