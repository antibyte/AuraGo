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
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Enabled toggle ──
    html += '<div class="cfg-group-title">' + t('config.refresh.operation') + '</div>';
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.document_creator.enabled_label') + '</div>';
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
    html += '<section id="dc-gotenberg-fields" class="cfg-topic' + (showGot ? '' : ' is-hidden') + '">';
    html += '<h2 class="cfg-topic-heading">' + t('config.document_creator.gotenberg_title') + '</h2>';

    html += '<div class="field-group"><label for="dc-allow-remote-resources">';
    html += '<input id="dc-allow-remote-resources" type="checkbox" data-path="tools.document_creator.allow_remote_resources" aria-describedby="dc-remote-help"' + (data.allow_remote_resources === true ? ' checked' : '') + '> ';
    html += t('config.document_creator.allow_remote_resources_label') + '</label>';
    html += '<div id="dc-remote-help" class="field-help">' + t('help.document_creator.allow_remote_resources') + '</div></div>';

    // URL
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.document_creator.gotenberg_url_label') + '</div>';
    const helpUrl = t('help.document_creator.gotenberg_url');
    if (helpUrl) html += '<div class="field-help">' + helpUrl + '</div>';
    html += '<input class="field-input" type="text" data-path="tools.document_creator.gotenberg.url" value="' + escapeAttr(gotCfg.url || '') + '">';
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
    html += '<button class="btn-save dc-test-btn" onclick="dcTestGotenberg()" id="dc-test-btn" data-config-action-draft="true">🔌 ' + t('config.document_creator.test_gotenberg') + '</button>';
    html += cfgActionResult.html('dc-test-result');
    html += '<div class="field-help">' + t('config.document_creator.test_saved_settings') + '</div>';
    html += '</div>';

    html += '</section>'; // End gotenberg-fields
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

// Checks the URL typed in (unsaved included). The restart hint for the managed
// container still looks at the saved configuration on the server.
async function dcTestGotenberg() {
    return cfgActionResult(document.getElementById('dc-test-btn'), () => fetch('/api/document-creator/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['tools.document_creator']) })
    }), {
        target: 'dc-test-result',
        pending: t('config.document_creator.loading'),
        success: t('config.document_creator.test_verified'),
        failure: outcome => {
            const body = outcome.body || {};
            if (outcome.httpStatus >= 400) return t('config.document_creator.status_error');
            return t('config.document_creator.' + (body.reachable === true ? 'test_unverified' : 'test_unreachable'));
        },
        interpret: outcome => {
            const body = outcome.body || {};
            if (outcome.httpStatus >= 400) return null;
            // A running but outdated managed container is a warning, not a failed URL.
            if (body.restart_required) return { state: 'warning', reason: t('config.document_creator.restart_required') };
            if (body.status === 'success' && body.security_verified === true) return { state: 'success', reason: '' };
            return { state: 'error' };
        }
    });
}
