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
    html += '<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">';
    html += '<span style="font-size:0.85rem;color:var(--text-secondary);">' + t('config.document_creator.enabled_label') + '</span>';
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
    html += '<div class="field-group" style="margin-top:1rem;padding-top:1rem;border-top:1px solid var(--border-subtle);">';
    html += '<div class="field-label">' + t('config.document_creator.backend_label') + '</div>';
    const helpBackend = t('help.document_creator.backend');
    if (helpBackend) html += '<div class="field-help">' + helpBackend + '</div>';
    html += '<div style="display:flex;gap:1.5rem;flex-wrap:wrap;margin-top:0.6rem;">';
    html += '<label style="display:flex;align-items:center;gap:0.5rem;cursor:pointer;">';
    html += '<input type="radio" name="dc_backend" data-path="tools.document_creator.backend" value="maroto"' + (backend === 'maroto' ? ' checked' : '') + ' onchange="dcSwitchBackend(this.value)">';
    html += '<span style="font-size:0.85rem;">' + t('config.document_creator.backend_maroto') + '</span>';
    html += '</label>';
    html += '<label style="display:flex;align-items:center;gap:0.5rem;cursor:pointer;">';
    html += '<input type="radio" name="dc_backend" data-path="tools.document_creator.backend" value="gotenberg"' + (backend === 'gotenberg' ? ' checked' : '') + ' onchange="dcSwitchBackend(this.value)">';
    html += '<span style="font-size:0.85rem;">' + t('config.document_creator.backend_gotenberg') + '</span>';
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
    html += '<div id="dc-gotenberg-fields" style="display:' + (showGot ? 'block' : 'none') + ';margin-top:1rem;padding:1rem;border-radius:10px;background:rgba(99,179,237,0.05);border:1px solid rgba(99,179,237,0.15);">';
    html += '<div style="font-weight:600;font-size:0.9rem;color:var(--accent);margin-bottom:0.8rem;">🐳 Gotenberg</div>';

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
    html += '<div style="display:flex;align-items:center;gap:0.5rem;">';
    html += '<input class="field-input" type="number" data-path="tools.document_creator.gotenberg.timeout" value="' + (gotCfg.timeout || 120) + '" style="width:100px;" min="5" max="600">';
    html += '<span style="color:var(--text-secondary);font-size:0.85rem;">' + t('config.document_creator.seconds') + '</span>';
    html += '</div></div>';

    // Test button
    html += '<div class="field-group">';
    html += '<button class="btn-save" style="padding:0.5rem 1.2rem;font-size:0.85rem;" onclick="dcTestGotenberg()" id="dc-test-btn">🔌 ' + t('config.document_creator.test_gotenberg') + '</button>';
    html += '<span id="dc-test-result" style="margin-left:0.8rem;font-size:0.83rem;"></span>';
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
    if (el) el.style.display = val === 'gotenberg' ? 'block' : 'none';
}

async function dcTestGotenberg() {
    var btn = document.getElementById('dc-test-btn');
    var result = document.getElementById('dc-test-result');
    btn.disabled = true;
    result.textContent = '⏳ ...';
    result.style.color = 'var(--text-secondary)';

    try {
        var resp = await fetch('/api/document-creator/test', { method: 'POST' });
        var body = await resp.json();
        if (resp.ok && body.status === 'up') {
            result.style.color = 'var(--success, #34c759)';
            result.textContent = '✅ ' + t('config.document_creator.test_ok');
        } else {
            result.style.color = 'var(--danger, #ff3b30)';
            result.textContent = '❌ ' + (body.message || ('HTTP ' + resp.status));
        }
    } catch (e) {
        result.style.color = 'var(--danger, #ff3b30)';
        result.textContent = '❌ ' + e.message;
    } finally {
        btn.disabled = false;
    }
}
