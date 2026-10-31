let _mqttSection = null;

async function renderMQTTSection(section) {
    if (section) _mqttSection = section; else section = _mqttSection;
    const data = configData['mqtt'] || {};
    const enabled = data.enabled === true;
    const tls = data.tls || {};
    const buf = data.buffer || {};
    const availability = data.availability || {};
    const tlsEnabled = tls.enabled === true;
    const cleanSession = data.clean_session !== false;
    const passwordPlaceholder = cfgSecretPlaceholder(data.password, t('config.mqtt.password_placeholder'));

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // Status banner
    html += cfgStatusBanner.html('mqtt-status-banner', enabled ? (data.broker ? 'checking' : 'unconfigured') : 'off', { detail: enabled && !data.broker ? t('config.mqtt.status_no_broker') : '' });

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
        <div id="mqtt-tls-transport-message" class="pw-field-error" role="alert" hidden></div>
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
        <div class="adg-password-row">
            <div class="password-wrap cfg-password-input">
                <input class="field-input adg-password-input" type="password" id="mqtt-password" data-path="mqtt.password" value="${escapeAttr(cfgSecretValue(data.password))}" placeholder="${escapeAttr(passwordPlaceholder)}" autocomplete="off">
                <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
        </div>
    </div>`;

    // Topics
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.topics_label')}</div>
        <div class="field-help">${t('help.mqtt.topics')}</div>
        <input class="field-input" type="text" data-type="array" data-path="mqtt.topics" value="${escapeAttr(Array.isArray(data.topics) ? data.topics.join(', ') : (data.topics || ''))}" placeholder="home/#, sensors/+">
    </div>`;

    // QoS dropdown
    const qos = data.qos || 0;
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.qos_label')}</div>
        <div class="field-help">${t('help.mqtt.qos')}</div>
        <select class="field-select" data-type="number" data-path="mqtt.qos">
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

    // Relay without broker login (relays and MQTT mission triggers)
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.allow_unauthenticated_relay_label')}</div>
        <div class="field-help">${t('help.mqtt.allow_unauthenticated_relay')}</div>
        <div class="toggle-wrap">
            <div class="toggle${data.allow_unauthenticated_relay ? ' on' : ''}" data-path="mqtt.allow_unauthenticated_relay" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${data.allow_unauthenticated_relay ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // Relay over plaintext or skipped certificate verification
    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.allow_untrusted_transport_label')}</div>
        <div class="field-help">${t('help.mqtt.allow_untrusted_transport')}</div>
        <div class="toggle-wrap">
            <div class="toggle${data.allow_untrusted_transport ? ' on' : ''}" data-path="mqtt.allow_untrusted_transport" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${data.allow_untrusted_transport ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
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

    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.clean_session_label')}</div>
        <div class="field-help">${t('help.mqtt.clean_session')}</div>
        <div class="toggle-wrap">
            <div class="toggle${cleanSession ? ' on' : ''}" data-path="mqtt.clean_session" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${cleanSession ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.trigger_min_interval_label')}</div>
        <div class="field-help">${t('help.mqtt.trigger_min_interval_seconds')}</div>
        <input class="field-input" type="number" min="0" max="86400" data-path="mqtt.trigger_min_interval_seconds" value="${escapeAttr(String(data.trigger_min_interval_seconds || 0))}" placeholder="0">
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

    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.buffer_max_age_hours_label')}</div>
        <div class="field-help">${t('help.mqtt.buffer_max_age_hours')}</div>
        <input class="field-input" type="number" min="0" max="8760" data-path="mqtt.buffer.max_age_hours" value="${escapeAttr(String(buf.max_age_hours || 0))}" placeholder="0">
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.buffer_max_payload_bytes_label')}</div>
        <div class="field-help">${t('help.mqtt.buffer_max_payload_bytes')}</div>
        <input class="field-input" type="number" min="1024" max="10485760" data-path="mqtt.buffer.max_payload_bytes" value="${escapeAttr(String(buf.max_payload_bytes || 262144))}" placeholder="262144">
    </div>`;

    // Availability / LWT Section
    html += `<hr class="cfg-section-hr">`;
    html += `<div class="cfg-section-title">📶 ${t('config.mqtt.availability_title')}</div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.mqtt.availability_enabled_label')}</div>
        <div class="field-help">${t('help.mqtt.availability_enabled')}</div>
        <div class="toggle-wrap">
            <div class="toggle${availability.enabled ? ' on' : ''}" data-path="mqtt.availability.enabled" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${availability.enabled ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    if (availability.enabled) {
        const availabilityQos = availability.qos == null ? 1 : availability.qos;
        html += `<div class="field-group">
            <div class="field-label">${t('config.mqtt.availability_topic_label')}</div>
            <div class="field-help">${t('help.mqtt.availability_topic')}</div>
            <input class="field-input" type="text" data-path="mqtt.availability.topic" value="${escapeAttr(availability.topic || 'aurago/status')}" placeholder="aurago/status">
        </div>`;

        html += `<div class="field-grid two-cols">
            <div class="field-group">
                <div class="field-label">${t('config.mqtt.availability_online_payload_label')}</div>
                <div class="field-help">${t('help.mqtt.availability_payloads')}</div>
                <input class="field-input" type="text" data-path="mqtt.availability.online_payload" value="${escapeAttr(availability.online_payload || 'online')}" placeholder="online">
            </div>
            <div class="field-group">
                <div class="field-label">${t('config.mqtt.availability_offline_payload_label')}</div>
                <div class="field-help">${t('help.mqtt.availability_payloads')}</div>
                <input class="field-input" type="text" data-path="mqtt.availability.offline_payload" value="${escapeAttr(availability.offline_payload || 'offline')}" placeholder="offline">
            </div>
        </div>`;

        html += `<div class="field-group">
            <div class="field-label">${t('config.mqtt.availability_qos_label')}</div>
            <div class="field-help">${t('help.mqtt.availability_qos')}</div>
            <select class="field-select" data-type="number" data-path="mqtt.availability.qos">
                <option value="0"${availabilityQos === 0 ? ' selected' : ''}>${t('config.mqtt.qos_0')}</option>
                <option value="1"${availabilityQos === 1 ? ' selected' : ''}>${t('config.mqtt.qos_1')}</option>
                <option value="2"${availabilityQos === 2 ? ' selected' : ''}>${t('config.mqtt.qos_2')}</option>
            </select>
        </div>`;

        html += `<div class="field-group">
            <div class="field-label">${t('config.mqtt.availability_retain_label')}</div>
            <div class="field-help">${t('help.mqtt.availability_retain')}</div>
            <div class="toggle-wrap">
                <div class="toggle${availability.retain !== false ? ' on' : ''}" data-path="mqtt.availability.retain" onclick="toggleBool(this)"></div>
                <span class="toggle-label">${availability.retain !== false ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
            </div>
        </div>`;
    }

    html += `<hr class="cfg-section-hr">`;
    html += `<div class="cfg-actions-row">
        <button class="btn-save adg-test-btn" onclick="mqttTestConnection()" id="mqtt-test-btn" data-config-action-draft="true">${t('config.mqtt.test_btn')}</button>
        ${cfgActionResult.html('mqtt-test-result')}
    </div>`;

    // Messages preview
    html += `<hr class="cfg-section-hr">`;
    html += `<div class="cfg-section-title">📋 ${t('config.mqtt.messages_title')}</div>`;
    html += `<div id="mqtt-messages-area" class="cfg-messages-area">
        <div id="mqtt-messages-list" class="cfg-messages-list">${t('config.mqtt.messages_loading')}</div>
        <div class="pw-u-inline-actions">
            <button class="btn btn-sm btn-secondary" onclick="mqttRefreshMessages()">🔄 ${t('config.mqtt.refresh_btn')}</button>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    const mqttTransportMessage = document.getElementById('mqtt-tls-transport-message');
    const mqttBrokerInput = document.querySelector('[data-path="mqtt.broker"]');
    const mqttTLSToggle = document.querySelector('[data-path="mqtt.tls.enabled"]');
    const refreshMQTTTLSMessage = () => {
        if (!mqttTransportMessage) return;
        const broker = mqttBrokerInput ? String(mqttBrokerInput.value || '') : '';
        const schemeMatch = broker.match(/^([a-z][a-z0-9+.-]*):/i);
        const scheme = schemeMatch ? schemeMatch[1].toLowerCase() : '';
        const plaintext = ['tcp', 'mqtt', 'ws', 'unix'].includes(scheme);
        const tlsEnabledNow = !!(mqttTLSToggle && mqttTLSToggle.classList.contains('on'));
        mqttTransportMessage.hidden = !(tlsEnabledNow && plaintext);
        if (tlsEnabledNow && plaintext) {
            mqttTransportMessage.textContent = t('config.mqtt.tls_plaintext_error');
        }
    };
    if (mqttBrokerInput) {
        mqttBrokerInput.addEventListener('input', refreshMQTTTLSMessage);
        mqttBrokerInput.addEventListener('change', refreshMQTTTLSMessage);
    }
    if (mqttTLSToggle) mqttTLSToggle.addEventListener('click', refreshMQTTTLSMessage);
    refreshMQTTTLSMessage();

    if (enabled && data.broker) {
        mqttCheckStatus();
        mqttRefreshMessages();
    }
}

// Check MQTT connection status
function mqttCheckStatus() {
    const banner = document.getElementById('mqtt-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/mqtt/status'), res => {
        if (!res || !res.status || res.status === 'disabled') return { state: 'off' };
        if (res.status === 'no_broker') return { state: 'unconfigured', detail: t('config.mqtt.status_no_broker') };
        const stateInfo = res.connection_state ? ` · ${res.connection_state}` : '';
        const sourceInfo = res.credential_source
            ? ` · ${t('config.mqtt.credential_source')}: ${res.credential_source}` : '';
        if (res.connected) {
            const tlsInfo = res.tls_enabled ? ' (TLS)' : '';
            const activeBroker = res.active_broker || res.broker || '';
            const brokerInfo = activeBroker && res.broker && activeBroker !== res.broker
                ? `${activeBroker} ← ${res.broker}` : activeBroker;
            const buffered = Number(res.buffer_len || 0);
            return { state: 'connected', detail: `${t('config.mqtt.status_connected')} — ${brokerInfo}${tlsInfo}${stateInfo}${sourceInfo} · ${buffered} ${t('config.mqtt.buffered_suffix')}` };
        }
        const lastError = res.stats && res.stats.last_error ? ` — ${res.stats.last_error}` : '';
        return { state: 'error', detail: t('config.mqtt.status_disconnected') + stateInfo + sourceInfo + lastError };
    });
}

// Test connection
function mqttTestConnection() {
    const btn = document.getElementById('mqtt-test-btn');
    return cfgActionResult(btn, () => fetch('/api/mqtt/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['mqtt']) })
    }), {
        target: 'mqtt-test-result',
        pending: t('config.mqtt.testing'),
        success: t('config.mqtt.test_ok'),
        failure: t('config.mqtt.test_fail'),
        onSuccess: () => mqttCheckStatus()
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
                const time = m.timestamp ? new Date(m.timestamp).toLocaleTimeString() : '';
                const topic = escapeHtml(m.topic || '');
                const truncated = m.payload_truncated ? ` ${t('config.mqtt.payload_truncated')}` : '';
                const payload = escapeHtml(typeof m.payload === 'string' ? m.payload : JSON.stringify(m.payload));
                return `<div class="cfg-message-item">
                    <span class="cfg-message-time">${time}</span>
                    <span class="cfg-message-topic">${topic}</span>
                    <span class="cfg-message-payload">${payload}${truncated}</span>
                </div>`;
            }).join('');
            list.innerHTML = html;
        })
        .catch(() => {
            list.innerHTML = `<div class="cfg-error-text">${t('config.mqtt.messages_error')}</div>`;
        });
}

