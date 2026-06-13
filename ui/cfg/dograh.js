// cfg/dograh.js - Dograh integration section module

let _dograhSection = null;

const dograhEndpoints = {
    status: '/api/dograh/status',
    test: '/api/dograh/test',
    start: '/api/dograh/start',
    stop: '/api/dograh/stop',
    recreate: '/api/dograh/recreate',
    provisionWebhook: '/api/dograh/provision-webhook',
    registerMCP: '/api/dograh/register-aurago-mcp-tool'
};

const dograhFallbackText = {
    'config.section.dograh.label': 'Dograh',
    'config.section.dograh.desc': 'Managed Dograh workflow automation stack and MCP bridge',
    'config.dograh.enabled_label': 'Enable Dograh',
    'config.dograh.mode_label': 'Mode',
    'config.dograh.mode_managed': 'Managed stack',
    'config.dograh.mode_external': 'External Dograh',
    'config.dograh.readonly_label': 'Read-only helpers',
    'config.dograh.auto_start_label': 'Start automatically',
    'config.dograh.api_url_label': 'Dograh API URL',
    'config.dograh.ui_url_label': 'Dograh UI URL',
    'config.dograh.api_key_label': 'Dograh API key',
    'config.dograh.host_label': 'Bind host',
    'config.dograh.api_host_port_label': 'API host port',
    'config.dograh.ui_host_port_label': 'UI host port',
    'config.dograh.telemetry_label': 'Enable Dograh telemetry',
    'config.dograh.mcp_client_label': 'Connect AuraGo to Dograh MCP',
    'config.dograh.mcp_server_tool_label': 'Allow Dograh to call AuraGo MCP',
    'config.dograh.credential_uuid_label': 'Dograh credential UUID',
    'config.dograh.allowed_tools_label': 'AuraGo tool filter',
    'config.dograh.webhook_slug_label': 'Webhook slug',
    'config.dograh.disabled_notice': 'Dograh is disabled',
    'config.dograh.disabled_desc': 'Enable Dograh to manage the local stack or connect to an external Dograh API.',
    'config.dograh.sidecar_note': 'Managed mode starts PostgreSQL, Redis, MinIO, Dograh API and Dograh UI containers.',
    'config.dograh.start_button': 'Start',
    'config.dograh.stop_button': 'Stop',
    'config.dograh.recreate_button': 'Recreate',
    'config.dograh.test_button': 'Test',
    'config.dograh.webhook_button': 'Provision webhook',
    'config.dograh.mcp_register_button': 'Register MCP tool',
    'config.dograh.vault_section_title': 'Vault',
    'config.dograh.mcp_section_title': 'MCP',
    'config.dograh.webhook_section_title': 'Webhook',
    'config.dograh.testing': 'Testing Dograh...',
    'config.dograh.starting': 'Starting Dograh...',
    'config.dograh.stopping': 'Stopping Dograh...',
    'config.dograh.recreating': 'Recreating Dograh...',
    'config.dograh.status_prefix': 'Status:',
    'config.dograh.status_error': 'Dograh status unavailable:',
    'config.dograh.setup_required': 'Setup required',
    'config.dograh.admin_setup_required': 'Create an API key in Dograh, then store it here.',
    'config.dograh.webhook_token_hint': 'Webhook token created. Copy it into Dograh now; AuraGo will not show it again.',
    'help.dograh.enabled': 'Shows Dograh in AuraGo and enables stack, MCP and webhook helpers.',
    'help.dograh.api_key': 'Dograh API key from the Dograh UI. AuraGo stores it only in the Vault and sends it as X-API-Key.',
    'help.dograh.api_url': 'Base URL AuraGo uses for Dograh API calls and the /api/v1/mcp/ endpoint.',
    'help.dograh.ui_url': 'Browser URL opened from the integrations drawer for the Dograh UI.',
    'help.dograh.mode': 'Managed starts the Dograh Docker stack. External connects to an existing Dograh API.',
    'help.dograh.readonly': 'Blocks AuraGo helper actions that create or modify Dograh resources.',
    'help.dograh.auto_start': 'Starts the managed Dograh stack automatically when AuraGo starts.',
    'help.dograh.telemetry_enabled': 'Allows Dograh upstream telemetry. Keep disabled for private home-lab deployments.',
    'help.dograh.mcp_client_enabled': 'Adds Dograh as a runtime MCP server so the agent can call Dograh tools.',
    'help.dograh.mcp_server_tool_enabled': 'Allows AuraGo to register its own authenticated /mcp endpoint as a Dograh tool.',
    'help.dograh.credential_uuid': 'Credential UUID created in Dograh for the AuraGo MCP Bearer token.',
    'help.dograh.allowed_tools': 'Optional comma-separated AuraGo MCP tools Dograh may call.',
    'help.dograh.webhook_slug': 'AuraGo webhook path used for callbacks sent from Dograh workflows.',
    'help.dograh.host': 'Network interface for managed Dograh ports. 127.0.0.1 keeps them local.',
    'help.dograh.api_host_port': 'Host port mapped to the managed Dograh API container.',
    'help.dograh.ui_host_port': 'Host port mapped to the managed Dograh UI container.'
};

function dograhText(key, fallback) {
    let value = '';
    if (typeof t === 'function') value = t(key);
    if (typeof value === 'string' && value.trim() !== '' && value !== key) return value;
    if (Object.prototype.hasOwnProperty.call(dograhFallbackText, key)) return dograhFallbackText[key];
    if (typeof fallback === 'string' && fallback.trim() !== '' && fallback !== key) return fallback;
    return '';
}

function dograhEnsureData() {
    if (!configData.dograh) configData.dograh = {};
    const data = configData.dograh;
    if (typeof data.auto_start !== 'boolean') data.auto_start = true;
    if (typeof data.readonly !== 'boolean') data.readonly = true;
    if (typeof data.mcp_client_enabled !== 'boolean') data.mcp_client_enabled = true;
    if (typeof data.mcp_server_tool_enabled !== 'boolean') data.mcp_server_tool_enabled = true;
    if (!data.mode) data.mode = 'managed';
    if (!data.api_url) data.api_url = 'http://127.0.0.1:8000';
    if (!data.ui_url) data.ui_url = 'http://127.0.0.1:3010';
    if (!data.host) data.host = '127.0.0.1';
    if (!data.api_host_port) data.api_host_port = 8000;
    if (!data.ui_host_port) data.ui_host_port = 3010;
    if (!data.api_image || data.api_image === 'dograhai/dograh-api:latest') data.api_image = 'ghcr.io/dograh-hq/dograh-api:latest';
    if (!data.ui_image || data.ui_image === 'dograhai/dograh-ui:latest') data.ui_image = 'ghcr.io/dograh-hq/dograh-ui:latest';
    if (!data.postgres_image) data.postgres_image = 'pgvector/pgvector:pg17';
    if (!data.redis_image) data.redis_image = 'redis:7';
    if (!data.minio_image) data.minio_image = 'minio/minio:latest';
    if (!data.callback_webhook_slug) data.callback_webhook_slug = 'dograh-callback';
    if (!data.aurago_mcp_tool_name) data.aurago_mcp_tool_name = 'AuraGo';
    return data;
}

async function renderDograhSection(section) {
    if (section) _dograhSection = section; else section = _dograhSection;
    const data = dograhEnsureData();
    const enabled = data.enabled === true;
    const managed = data.mode !== 'external';

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + dograhText('config.section.dograh.label', section.label) + '</div>';
    html += '<div class="section-desc">' + dograhText('config.section.dograh.desc', section.desc) + '</div>';
    html += dograhToggleRow('config.dograh.enabled_label', 'help.dograh.enabled', enabled, 'dograh.enabled', "dograhToggleEnabled(this.classList.contains('on'))");

    if (!enabled) {
        html += '<div class="wh-notice"><span>▧</span><div><strong>' + dograhText('config.dograh.disabled_notice') + '</strong><br>';
        html += '<small>' + dograhText('config.dograh.disabled_desc') + '</small></div></div></div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="cfg-note-banner cfg-note-banner-info">▧ ' + dograhText('config.dograh.sidecar_note') + '</div>';
    html += '<div id="dograh-status-box" class="adg-status-banner">' + escapeHtml(dograhText('config.dograh.status_prefix')) + ' ...</div>';
    html += '<div class="cfg-actions-row">';
    html += dograhActionButton('dograh-test-btn', 'config.dograh.test_button', 'dograhAction(\\'test\\')', 'adg-test-btn', '🔌 ');
    html += dograhActionButton('dograh-start-btn', 'config.dograh.start_button', 'dograhAction(\\'start\\')', 'btn-secondary', '▶ ');
    html += dograhActionButton('dograh-stop-btn', 'config.dograh.stop_button', 'dograhAction(\\'stop\\')', 'btn-secondary', '⏹ ');
    html += dograhActionButton('dograh-recreate-btn', 'config.dograh.recreate_button', 'dograhAction(\\'recreate\\')', 'btn-secondary', '🔄 ');
    html += '<span id="dograh-test-result" class="adg-test-result"></span>';
    html += '</div>';

    html += '<div class="field-grid two-cols">';
    html += dograhField('config.dograh.mode_label', 'help.dograh.mode',
        '<select class="field-select" data-path="dograh.mode" onchange="setNestedValue(configData,\\'dograh.mode\\',this.value);setDirty(true);renderDograhSection(null)">' +
        '<option value="managed"' + (managed ? ' selected' : '') + '>' + dograhText('config.dograh.mode_managed') + '</option>' +
        '<option value="external"' + (!managed ? ' selected' : '') + '>' + dograhText('config.dograh.mode_external') + '</option>' +
        '</select>');
    html += dograhToggleRow('config.dograh.readonly_label', 'help.dograh.readonly', data.readonly !== false, 'dograh.readonly');
    if (managed) html += dograhToggleRow('config.dograh.auto_start_label', 'help.dograh.auto_start', data.auto_start !== false, 'dograh.auto_start');
    html += dograhToggleRow('config.dograh.telemetry_label', 'help.dograh.telemetry_enabled', data.telemetry_enabled === true, 'dograh.telemetry_enabled');
    html += '</div>';

    html += '<div class="field-grid two-cols">';
    html += dograhInput('config.dograh.api_url_label', 'help.dograh.api_url', 'dograh.api_url', data.api_url || 'http://127.0.0.1:8000');
    html += dograhInput('config.dograh.ui_url_label', 'help.dograh.ui_url', 'dograh.ui_url', data.ui_url || 'http://127.0.0.1:3010');
    html += '</div>';

    if (managed) {
        html += '<div class="field-grid two-cols">';
        html += dograhInput('config.dograh.host_label', 'help.dograh.host', 'dograh.host', data.host || '127.0.0.1');
        html += dograhNumber('config.dograh.api_host_port_label', 'help.dograh.api_host_port', 'dograh.api_host_port', data.api_host_port || 8000);
        html += dograhNumber('config.dograh.ui_host_port_label', 'help.dograh.ui_host_port', 'dograh.ui_host_port', data.ui_host_port || 3010);
        html += '</div>';
    }

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + escapeHtml(dograhText('config.dograh.vault_section_title')) + '</div>';
    html += dograhSecretField('config.dograh.api_key_label', 'help.dograh.api_key', 'dograh-api-key-input', 'dograh.api_key', 'dg_••••••••');
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + escapeHtml(dograhText('config.dograh.mcp_section_title')) + '</div>';
    html += '<div class="field-grid two-cols">';
    html += dograhToggleRow('config.dograh.mcp_client_label', 'help.dograh.mcp_client_enabled', data.mcp_client_enabled !== false, 'dograh.mcp_client_enabled');
    html += dograhToggleRow('config.dograh.mcp_server_tool_label', 'help.dograh.mcp_server_tool_enabled', data.mcp_server_tool_enabled !== false, 'dograh.mcp_server_tool_enabled');
    html += dograhInput('config.dograh.credential_uuid_label', 'help.dograh.credential_uuid', 'dograh.aurago_mcp_credential_uuid', data.aurago_mcp_credential_uuid || '');
    html += dograhInput('config.dograh.allowed_tools_label', 'help.dograh.allowed_tools', 'dograh.aurago_mcp_allowed_tools', Array.isArray(data.aurago_mcp_allowed_tools) ? data.aurago_mcp_allowed_tools.join(', ') : '');
    html += '</div><div class="cfg-actions-row">';
    html += dograhButton('dograh-mcp-btn', 'config.dograh.mcp_register_button', 'dograhAction(\\'registerMCP\\')');
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + escapeHtml(dograhText('config.dograh.webhook_section_title')) + '</div>';
    html += dograhInput('config.dograh.webhook_slug_label', 'help.dograh.webhook_slug', 'dograh.callback_webhook_slug', data.callback_webhook_slug || 'dograh-callback');
    html += '<div class="cfg-actions-row">' + dograhButton('dograh-webhook-btn', 'config.dograh.webhook_button', 'dograhAction(\\'provisionWebhook\\')') + '</div>';
    html += '</div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    dograhRefreshStatus();
}

function dograhField(labelKey, helpKey, controlHTML) {
    const helpText = dograhText(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + escapeHtml(dograhText(labelKey)) + '</div>';
    if (helpText) html += '<div class="field-help">' + escapeHtml(helpText) + '</div>';
    html += controlHTML;
    return html + '</div>';
}

function dograhInput(labelKey, helpKey, path, value) {
    const arrayType = path === 'dograh.aurago_mcp_allowed_tools' ? ' data-type="array"' : '';
    return dograhField(labelKey, helpKey, '<input class="field-input" type="text" value="' + escapeAttr(value || '') + '" data-path="' + escapeAttr(path) + '"' + arrayType + '>');
}

function dograhNumber(labelKey, helpKey, path, value) {
    return dograhField(labelKey, helpKey, '<input class="field-input" type="number" min="1" max="65535" value="' + escapeAttr(value || '') + '" data-path="' + escapeAttr(path) + '">');
}

function dograhSecretField(labelKey, helpKey, id, path, placeholder) {
    const value = cfgSecretValue(path.split('.').reduce((o, k) => o && o[k], configData));
    return dograhField(labelKey, helpKey,
        '<div class="password-wrap"><input class="field-input" type="password" id="' + id + '" value="' + escapeAttr(value) + '" placeholder="' + escapeAttr(placeholder) + '" data-path="' + escapeAttr(path) + '" autocomplete="off"><button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button></div>');
}

function dograhToggleRow(labelKey, helpKey, enabled, path, onclick) {
    const handler = onclick || "dograhTogglePath('" + path + "', this.classList.contains('on'))";
    return dograhField(labelKey, helpKey,
        '<div class="toggle ' + (enabled ? 'on' : '') + '" onclick="' + handler + '"></div>' +
        '<input type="hidden" data-path="' + escapeAttr(path) + '" value="' + (enabled ? 'true' : 'false') + '">');
}

function dograhButton(id, labelKey, onclick) {
    return dograhActionButton(id, labelKey, onclick, 'btn-secondary', '');
}

function dograhActionButton(id, labelKey, onclick, btnClass, prefix) {
    return '<button type="button" id="' + id + '" class="btn-save ' + btnClass + '" onclick="' + onclick + '">' + prefix + escapeHtml(dograhText(labelKey)) + '</button>';
}

function dograhSetBanner(state, html) {
    const box = document.getElementById('dograh-status-box');
    if (!box) return;
    box.className = 'adg-status-banner' + (state ? ' is-' + state : '');
    box.innerHTML = html;
}

function dograhActionLoadingText(action) {
    const map = {
        test: 'config.dograh.testing',
        start: 'config.dograh.starting',
        stop: 'config.dograh.stopping',
        recreate: 'config.dograh.recreating'
    };
    return dograhText(map[action] || 'config.dograh.status_prefix') || action;
}

function dograhToggleEnabled(currentlyOn) {
    setNestedValue(configData, 'dograh.enabled', !currentlyOn);
    setDirty(true);
    renderDograhSection(null);
}

function dograhTogglePath(path, currentlyOn) {
    setNestedValue(configData, path, !currentlyOn);
    setDirty(true);
    renderDograhSection(null);
}

async function dograhRefreshStatus() {
    if (!document.getElementById('dograh-status-box')) return;
    try {
        const res = await fetch(dograhEndpoints.status, { credentials: 'same-origin' });
        const body = await res.json();
        dograhRenderStatus(body);
    } catch (err) {
        dograhSetBanner('danger', escapeHtml(dograhText('config.dograh.status_error') + ' ' + err.message));
    }
}

function dograhStatusState(body) {
    const status = String(body && body.status ? body.status : '').toLowerCase();
    if (body && body.admin_setup_required) return 'warning';
    if (body && body.setup_required) return 'warning';
    if (status === 'running' || status === 'ok' || status === 'connected') return 'success';
    if (status === 'error' || status === 'failed' || status === 'stopped') return 'danger';
    return '';
}

function dograhRenderStatus(body) {
    const parts = ['<strong>' + escapeHtml(dograhText('config.dograh.status_prefix')) + '</strong> ' + escapeHtml((body && body.status) || 'unknown')];
    if (body && body.admin_setup_required) parts.push('<span>' + escapeHtml(dograhText('config.dograh.admin_setup_required')) + '</span>');
    if (body && body.setup_required && body.message) parts.push('<span>' + escapeHtml(body.message) + '</span>');
    if (body && body.api_url) parts.push('<a href="' + escapeAttr(body.api_url) + '" target="_blank" rel="noopener noreferrer">API</a>');
    if (body && body.ui_url) parts.push('<a href="' + escapeAttr(body.ui_url) + '" target="_blank" rel="noopener noreferrer">UI</a>');
    dograhSetBanner(dograhStatusState(body), parts.join('<br>'));
}

async function dograhAction(action) {
    const result = document.getElementById('dograh-test-result');
    const btn = document.getElementById('dograh-' + action + '-btn');
    const loadingText = dograhActionLoadingText(action);
    if (btn) btn.disabled = true;
    if (action === 'test' && result) {
        result.className = 'adg-test-result';
        result.textContent = loadingText;
    } else {
        dograhSetBanner('', '<strong>' + escapeHtml(dograhText('config.dograh.status_prefix')) + '</strong> ' + escapeHtml(loadingText));
    }
    const endpoint = dograhEndpoints[action];
    if (!endpoint) {
        const msg = dograhText('config.dograh.status_error') + ' unknown action';
        if (action === 'test' && result) {
            result.className = 'adg-test-result is-danger';
            result.textContent = msg;
        } else {
            dograhSetBanner('danger', escapeHtml(msg));
        }
        if (btn) btn.disabled = false;
        return;
    }
    try {
        const res = await fetch(endpoint, {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ dograh: configData.dograh || {} })
        });
        const body = await res.json().catch(() => ({}));
        if (!res.ok) {
            const msg = body.error || body.message || res.statusText;
            if (action === 'test' && result) {
                result.className = 'adg-test-result is-danger';
                result.textContent = msg;
            } else {
                dograhSetBanner('danger', escapeHtml(dograhText('config.dograh.status_error') + ' ' + msg));
            }
            return;
        }
        if (body.token) {
            dograhSetBanner('warning', escapeHtml(dograhText('config.dograh.webhook_token_hint')) + '<br><code>' + escapeHtml(body.token) + '</code>');
            return;
        }
        if (action === 'test' && result) {
            const ok = body.status !== 'error';
            result.className = ok ? 'adg-test-result is-success' : 'adg-test-result is-danger';
            result.textContent = (body.message || body.status || '').toString();
        }
        dograhRenderStatus(body);
        setTimeout(dograhRefreshStatus, 900);
    } catch (err) {
        const msg = dograhText('config.dograh.status_error') + ' ' + err.message;
        if (action === 'test' && result) {
            result.className = 'adg-test-result is-danger';
            result.textContent = msg;
        } else {
            dograhSetBanner('danger', escapeHtml(msg));
        }
    } finally {
        if (btn) btn.disabled = false;
    }
}
