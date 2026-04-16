let _mqttSection = null;

async function renderMQTTSection(section) {
    if (section) _mqttSection = section; else section = _mqttSection;
    const data = configData['mqtt'] || {};
    const enabled = data.enabled === true;
    const tls = data.tls || {};
    const buf = data.buffer || {};
    const tlsEnabled = tls.enabled === true;
    const passwordPlaceholder = cfgSecretPlaceholder(data.password, t('config.mqtt.password_placeholder'));

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // Status banner
    html += `<div id="mqtt-status-banner" class="cfg-status-banner">${t('config.mqtt.checking')}</div>`;

    // Enable toggle
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.enabled_label')}</div>
        <div class="field-help">${t('help.mqtt.enabled')}</div>
        <div class="toggle-wrap">
            <div class="toggle${enabled ? ' on' : ''}" data-path="mqtt.enabled" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${enabled ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // Broker
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.broker_label')}</div>
        <div class="field-help">${t('help.mqtt.broker')}</div>
        <input class="field-input" type="text" data-path="mqtt.broker" value="${escapeAttr(data.broker || '')}" placeholder="tcp://localhost:1883">
    </div>`;

    // Client ID
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.client_id_label')}</div>
        <div class="field-help">${t('help.mqtt.client_id')}</div>
        <input class="field-input" type="text" data-path="mqtt.client_id" value="${escapeAttr(data.client_id || 'aurago')}" placeholder="aurago">
    </div>`;

    // Username
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.username_label')}</div>
        <div class="field-help">${t('help.mqtt.username')}</div>
        <input class="field-input" type="text" data-path="mqtt.username" value="${escapeAttr(data.username || '')}" placeholder="">
    </div>`;

    // Password (Vault)
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.password_label')}</div>
        <div class="field-help">${t('help.mqtt.password_help')}</div>
        <div class="cfg-field-row">
            <div class="password-wrap cfg-password-input">
                <input class="field-input cfg-password-input" type="password" id="mqtt-password" value="${escapeAttr(cfgSecretValue(data.password))}" placeholder="${escapeAttr(passwordPlaceholder)}">
                <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
            <button class="btn-save cfg-save-btn-sm" onclick="mqttSavePassword()">💾 ${t('config.mqtt.save_vault')}</button>
        </div>
    </div>`;

    // Topics
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.topics_label')}</div>
        <div class="field-help">${t('help.mqtt.topics')}</div>
        <input class="field-input" type="text" data-path="mqtt.topics" value="${escapeAttr(Array.isArray(data.topics) ? data.topics.join(', ') : (data.topics || ''))}" placeholder="home/#, sensors/+">
    </div>`;

    // QoS dropdown
    const qos = data.qos || 0;
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.qos_label')}</div>
        <div class="field-help">${t('help.mqtt.qos')}</div>
        <select class="field-input" data-path="mqtt.qos">
            <option value="0"${qos === 0 ? ' selected' : ''}>${t('config.mqtt.qos_0')}</option>
            <option value="1"${qos === 1 ? ' selected' : ''}>${t('config.mqtt.qos_1')}</option>
            <option value="2"${qos === 2 ? ' selected' : ''}>${t('config.mqtt.qos_2')}</option>
        </select>
    </div>`;

    // Relay to Agent toggle
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.relay_to_agent_label')}</div>
        <div class="field-help">${t('help.mqtt.relay_to_agent')}</div>
        <div class="toggle-wrap">
            <div class="toggle${data.relay_to_agent ? ' on' : ''}" data-path="mqtt.relay_to_agent" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${data.relay_to_agent ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // Read-only toggle
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.readonly_label')}</div>
        <div class="field-help">${t('help.mqtt.read_only')}</div>
        <div class="toggle-wrap">
            <div class="toggle${data.readonly ? ' on' : ''}" data-path="mqtt.readonly" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${data.readonly ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // Connect Timeout
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.connect_timeout_label')}</div>
        <div class="field-help">${t('help.mqtt.connect_timeout')}</div>
        <input class="field-input" type="number" min="1" max="120" data-path="mqtt.connect_timeout" value="${escapeAttr(String(data.connect_timeout || 15))}" placeholder="15">
    </div>`;

    // TLS Section
    html += `<hr class="cfg-section-hr">`;
    html += `<div class="cfg-section-title">🔒 ${t('config.mqtt.tls_title')}</div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.tls_enabled_label')}</div>
        <div class="field-help">${t('help.mqtt.tls_enabled')}</div>
        <div class="toggle-wrap">
            <div class="toggle${tlsEnabled ? ' on' : ''}" data-path="mqtt.tls.enabled" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${tlsEnabled ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    if (tlsEnabled) {
        html += `<div class="field-group">
            <div class="field-label">${t('config.mqtt.ca_file_label')}</div>
            <div class="field-help">${t('help.mqtt.tls_ca_file')}</div>
            <input class="field-input" type="text" data-path="mqtt.tls.ca_file" value="${escapeAttr(tls.ca_file || '')}" placeholder="/path/to/ca.crt">
        </div>`;

        html += `<div class="field-group">
            <div class="field-label">${t('config.mqtt.cert_file_label')}</div>
            <div class="field-help">${t('help.mqtt.tls_cert_file')}</div>
            <input class="field-input" type="text" data-path="mqtt.tls.cert_file" value="${escapeAttr(tls.cert_file || '')}" placeholder="/path/to/client.crt">
        </div>`;

        html += `<div class="field-group">
            <div class="field-label">${t('config.mqtt.key_file_label')}</div>
            <div class="field-help">${t('help.mqtt.tls_key_file')}</div>
            <input class="field-input" type="text" data-path="mqtt.tls.key_file" value="${escapeAttr(tls.key_file || '')}" placeholder="/path/to/client.key">
        </div>`;

        html += `<div class="field-group">
            <div class="field-label">${t('config.mqtt.insecure_skip_verify_label')}</div>
            <div class="field-help">${t('help.mqtt.tls_insecure_skip_verify')}</div>
            <div class="toggle-wrap">
                <div class="toggle${tls.insecure_skip_verify ? ' on' : ''}" data-path="mqtt.tls.insecure_skip_verify" onclick="toggleBool(this)"></div>
                <span class="toggle-label">${tls.insecure_skip_verify ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
            </div>
        </div>`;
    }

    // Buffer Section
    html += `<hr class="cfg-section-hr">`;
    html += `<div class="cfg-section-title">📥 ${t('config.mqtt.buffer_title')}</div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.buffer_max_messages_label')}</div>
        <div class="field-help">${t('help.mqtt.buffer_max_messages')}</div>
        <input class="field-input" type="number" min="0" max="10000" data-path="mqtt.buffer.max_messages" value="${escapeAttr(String(buf.max_messages || 500))}" placeholder="500">
    </div>`;

    // Test Connection button
    html += `<hr class="cfg-section-hr">`;
    html += `<div class="field-group">
        <button class="btn-save cfg-save-btn-sm" onclick="mqttTestConnection()" id="mqtt-test-btn">🔌 ${t('config.mqtt.test_btn')}</button>
        <span id="mqtt-test-result" class="cfg-status-text"></span>
    </div>`;

    // Messages preview
    html += `<hr class="cfg-section-hr">`;
    html += `<div class="cfg-section-title">📋 ${t('config.mqtt.messages_title')}</div>`;
    html += `<div id="mqtt-messages-area" class="cfg-messages-area">
        <div id="mqtt-messages-list" class="cfg-messages-list">${t('config.mqtt.messages_loading')}</div>
        <div class="cfg-btn-row">
            <button class="btn btn-sm btn-secondary" onclick="mqttRefreshMessages()">🔄 ${t('config.mqtt.refresh_btn')}</button>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    // Initial status check
    if (enabled && data.broker) {
        mqttCheckStatus();
        mqttRefreshMessages();
    }
}

// Check MQTT connection status
function mqttCheckStatus() {
    const banner = document.getElementById('mqtt-status-banner');
    if (!banner) return;
    banner.className = 'cfg-status-banner';
    banner.textContent = t('config.mqtt.checking');

    fetch('/api/mqtt/status')
        .then(r => r.json())
        .then(res => {
            if (!res.status || res.status === 'disabled') {
                banner.className = 'cfg-status-banner';
                banner.textContent = '⚪ ' + t('config.mqtt.status_disabled');
                return;
            }
            if (res.status === 'no_broker') {
                banner.className = 'cfg-status-banner';
                banner.textContent = '⚪ ' + t('config.mqtt.status_no_broker');
                return;
            }
            if (res.connected) {
                banner.className = 'cfg-status-banner cfg-status-success';
                const tlsInfo = res.tls_enabled ? ' (TLS)' : '';
                banner.textContent = `🟢 ${t('config.mqtt.status_connected')} — ${escapeHtml(res.broker || '')}${tlsInfo}`;
            } else {
                banner.className = 'cfg-status-banner cfg-status-error';
                banner.textContent = '🔴 ' + t('config.mqtt.status_disconnected');
            }
        })
        .catch(() => {
            banner.className = 'cfg-status-banner cfg-status-error';
            banner.textContent = '🔴 ' + t('config.mqtt.status_error');
        });
}

// Test connection
function mqttTestConnection() {
    const btn = document.getElementById('mqtt-test-btn');
    const result = document.getElementById('mqtt-test-result');
    if (btn) btn.disabled = true;
    if (result) { result.textContent = t('config.mqtt.testing'); result.className = 'cfg-status-text'; }

    fetch('/api/mqtt/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'success') {
                result.className = 'cfg-status-text cfg-status-success';
                result.textContent = '✅ ' + t('config.mqtt.test_ok');
                mqttCheckStatus();
            } else {
                result.className = 'cfg-status-text cfg-status-error';
                result.textContent = '❌ ' + (res.message || t('config.mqtt.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.className = 'cfg-status-text cfg-status-error';
                result.textContent = '❌ ' + t('config.mqtt.test_fail');
            }
        });
}

// Refresh messages
function mqttRefreshMessages() {
    const list = document.getElementById('mqtt-messages-list');
    if (!list) return;
    list.innerHTML = `<div class="cfg-loading-text">${t('config.mqtt.messages_loading')}</div>`;

    fetch('/api/mqtt/messages')
        .then(r => r.json())
        .then(res => {
            if (!res.messages || res.messages.length === 0) {
                list.innerHTML = `<div class="cfg-empty-text">${t('config.mqtt.messages_empty')}</div>`;
                return;
            }
            const html = res.messages.map(m => {
                const time = m.timestamp ? new Date(m.timestamp * 1000).toLocaleTimeString() : '';
                const topic = escapeHtml(m.topic || '');
                const payload = escapeHtml(typeof m.payload === 'string' ? m.payload : JSON.stringify(m.payload));
                return `<div class="cfg-message-item">
                    <span class="cfg-message-time">${time}</span>
                    <span class="cfg-message-topic">${topic}</span>
                    <span class="cfg-message-payload">${payload}</span>
                </div>`;
            }).join('');
            list.innerHTML = html;
        })
        .catch(() => {
            list.innerHTML = `<div class="cfg-error-text">${t('config.mqtt.messages_error')}</div>`;
        });
}

// Save password to vault
function mqttSavePassword() {
    const input = document.getElementById('mqtt-password');
    const pw = input ? input.value.trim() : '';
    if (!pw) { showToast(t('config.mqtt.password_empty'), 'error'); return; }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'mqtt_password', value: pw })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            showToast(t('config.mqtt.password_saved'), 'success');
            cfgMarkSecretStored(input, 'mqtt.password');
        } else {
            showToast(res.message || t('config.mqtt.password_save_failed'), 'error');
        }
    })
    .catch(() => showToast(t('config.mqtt.password_save_failed'), 'error'));
}
