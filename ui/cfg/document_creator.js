// cfg/document_creator.js — Document Creator config section module

let _dcSection = null;

async function renderDocumentCreatorSection(section) {
    if (section) _dcSection = section; else section = _dcSection;
    const toolsCfg = configData.tools || {};
    const data = toolsCfg.document_creator || {};
    const enabled = data.enabled === true;
    const backend = data.backend || 'maroto';
    const gotCfg = data.gotenberg || {};

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Enabled toggle ──
    html += '<div class="dc-toggle-row-main">';
    html += '<span class="dc-toggle-label">' + t('config.document_creator.enabled_label') + '</span>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="tools.document_creator.enabled" onclick="toggleBool(this);dcUpdateEnabled(this.classList.contains(\'on\'))"></div>';
    html += '</div>';

    if (!enabled) {
        html += '<div class="wh-notice"><span>📄</span><div>';
        html += '<strong>' + t('config.document_creator.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.document_creator.disabled_desc') + '</small>';
        html += '</div></div></div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    // ── Backend selector ──
    html += '<div class="field-group dc-field-group-top">';
    html += '<div class="field-label">' + t('config.document_creator.backend_label') + '</div>';
    const helpBackend = t('help.document_creator.backend');
    if (helpBackend) html += '<div class="field-help">' + helpBackend + '</div>';
    html += '<div class="dc-backend-row">';
    html += '<label class="dc-radio-label">';
    html += '<input type="radio" name="dc_backend" data-path="tools.document_creator.backend" value="maroto"' + (backend === 'maroto' ? ' checked' : '') + ' onchange="dcSwitchBackend(this.value)">';
    html += '<span class="dc-radio-text">' + t('config.document_creator.backend_maroto') + '</span>';
    html += '</label>';
    html += '<label class="dc-radio-label">';
    html += '<input type="radio" name="dc_backend" data-path="tools.document_creator.backend" value="gotenberg"' + (backend === 'gotenberg' ? ' checked' : '') + ' onchange="dcSwitchBackend(this.value)">';
    html += '<span class="dc-radio-text">' + t('config.document_creator.backend_gotenberg') + '</span>';
    html += '</label>';
    html += '</div></div>';

    // ── Output directory ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.document_creator.output_dir_label') + '</div>';
    const helpDir = t('help.document_creator.output_dir');
    if (helpDir) html += '<div class="field-help">' + helpDir + '</div>';
    html += '<input class="field-input" type="text" data-path="tools.document_creator.output_dir" value="' + escapeAttr(data.output_dir || 'data/documents') + '" placeholder="data/documents">';
    html += '</div>';

    // ── Gotenberg fields (conditional) ──
    var showGot = backend === 'gotenberg';
    html += '<div id="dc-gotenberg-fields" class="dc-gotenberg-box' + (showGot ? '' : ' is-hidden') + '">';
    html += '<div class="dc-gotenberg-title">' + t('config.document_creator.gotenberg_title') + '</div>';

    // URL
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.document_creator.gotenberg_url_label') + '</div>';
    const helpUrl = t('help.document_creator.gotenberg_url');
    if (helpUrl) html += '<div class="field-help">' + helpUrl + '</div>';
    html += '<input class="field-input" type="text" data-path="tools.document_creator.gotenberg.url" value="' + escapeAttr(gotCfg.url || 'http://gotenberg:3000') + '" placeholder="http://gotenberg:3000">';
    html += '</div>';

    // Timeout
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.document_creator.gotenberg_timeout_label') + '</div>';
    const helpTimeout = t('help.document_creator.gotenberg_timeout');
    if (helpTimeout) html += '<div class="field-help">' + helpTimeout + '</div>';
    html += '<div class="dc-timeout-row">';
    html += '<input class="field-input dc-timeout-input" type="number" data-path="tools.document_creator.gotenberg.timeout" value="' + (gotCfg.timeout || 120) + '" min="5" max="600">';
    html += '<span class="dc-seconds-label">' + t('config.document_creator.seconds') + '</span>';
    html += '</div></div>';

    // Test button
    html += '<div class="field-group">';
    html += '<button class="btn-save dc-test-btn" onclick="dcTestGotenberg()" id="dc-test-btn">🔌 ' + t('config.document_creator.test_gotenberg') + '</button>';
    html += '<span id="dc-test-result" class="dc-test-result"></span>';
    html += '</div>';

    html += '</div>'; // End gotenberg-fields
    html += '</div>'; // End cfg-section

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function dcUpdateEnabled(on) {
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.document_creator) configData.tools.document_creator = {};
    configData.tools.document_creator.enabled = on;
    setDirty(true);
    renderDocumentCreatorSection(null);
}

function dcSwitchBackend(val) {
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.document_creator) configData.tools.document_creator = {};
    configData.tools.document_creator.backend = val;
    setDirty(true);
    var el = document.getElementById('dc-gotenberg-fields');
    setHidden(el, val !== 'gotenberg');
}

async function dcTestGotenberg() {
    var btn = document.getElementById('dc-test-btn');
    var result = document.getElementById('dc-test-result');
    btn.disabled = true;
    result.textContent = t('config.document_creator.loading');
    result.className = 'dc-test-result';

    try {
        var resp = await fetch('/api/document-creator/test', { method: 'POST' });
        var body = await resp.json();
        if (resp.ok && body.status === 'success') {
            result.className = 'dc-test-result is-success';
            var backendInfo = body.active_backend ? ' (backend: ' + body.active_backend + ')' : '';
            result.textContent = t('config.document_creator.status_success') + ' ' + t('config.document_creator.test_ok') + backendInfo;
        } else {
            result.className = 'dc-test-result is-danger';
            result.textContent = t('config.document_creator.status_error') + ' ' + (body.message || ('HTTP ' + resp.status));
        }
    } catch (e) {
        result.className = 'dc-test-result is-danger';
        result.textContent = t('config.document_creator.status_error') + ' ' + e.message;
    } finally {
        btn.disabled = false;
    }
}
