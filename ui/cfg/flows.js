// cfg/flows.js — EasyDrag flows: run limits, AI provider for AI steps and agent options.
// The switch and the four limits are read at boot, so the save answer reports a restart ("EasyDrag flows").
// The limits' ranges are validation rules in js/config/catalog.js. The agent options are saved but not
// enforced yet (planned for phase 4), so their switches stay locked.
function renderFlowsSection(section) {
    const limits = [['max_parallel_runs', 8], ['max_parallel_nodes_per_run', 4], ['run_retention_days', 30], ['max_runs_per_flow', 200]];
    const data = configData.flows || (configData.flows = { enabled: true, ...Object.fromEntries(limits), ai_provider: '', agent: { read_only: false, allow_publish: false } });
    data.agent = data.agent || { read_only: false, allow_publish: false };
    const rules = (window.AuraConfigCatalog && window.AuraConfigCatalog.validationRules) || {};
    const label = key => escapeHtml(t('config.flows.' + key));
    const toggle = (key, path, on, locked) => '<div class="field-group"><div class="field-label">' + label(key) + '</div>'
        + '<div class="toggle' + (on ? ' on' : '') + (locked ? ' cfg-toggle-disabled' : '') + '" data-path="' + path + '" aria-label="' + label(key) + '"'
        + (locked ? ' aria-disabled="true" aria-describedby="flows-agent-note"' : ' onclick="toggleBool(this)"') + '></div></div>';
    const missionsOff = !!(configData.tools && configData.tools.missions && configData.tools.missions.enabled === false);
    let html = '<div class="cfg-section active"><div class="section-header">' + escapeHtml(section.label) + '</div><div class="section-desc">' + escapeHtml(section.desc) + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info">' + label('intro') + '</div>';
    if (missionsOff) html += '<div class="cfg-note-banner cfg-note-banner-warning">' + label('missions_off') + '</div>';
    html += toggle('enabled', 'flows.enabled', data.enabled !== false, false);
    html += '<div class="field-grid two-cols">';
    for (const [key, fallback] of limits) {
        const rule = rules['flows.' + key] || {};
        const range = (rule.min != null ? ' min="' + rule.min + '"' : '') + (rule.max != null ? ' max="' + rule.max + '"' : '');
        const value = Number(data[key]) > 0 ? Number(data[key]) : fallback;
        html += '<div class="field-group"><div class="field-label">' + label(key) + '</div><input class="field-input" type="number"' + range + ' value="' + value + '" data-path="flows.' + key + '" aria-label="' + label(key) + '"></div>';
    }
    html += '</div>';
    // A provider reference (lang/meta.json: provider_ref, empty_label_key), drawn and refreshed by the shared
    // choice helpers: an unknown saved id stays selected, a failed list shows the hint with Retry.
    html += '<div class="field-group"><div class="field-label">' + label('ai_provider') + '</div><div class="field-help">' + label('ai_provider_help') + '</div>'
        + '<select class="field-select" data-path="flows.ai_provider" data-config-choice="providers" aria-label="' + label('ai_provider') + '">'
        + cfgChoiceOptionsHTML('providers', helpTexts['flows.ai_provider'] || {}, data.ai_provider || '') + '</select>' + cfgChoiceHintHTML('providers') + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info" id="flows-agent-note">' + label('agent_note') + '</div>';
    html += toggle('agent_read_only', 'flows.agent.read_only', !!data.agent.read_only, true);
    html += toggle('agent_allow_publish', 'flows.agent.allow_publish', !!data.agent.allow_publish, true);
    // The desktop opens EasyDrag from ?app=; without flows or missions it would open without the app.
    if (data.enabled !== false && !missionsOff) html += '<div class="field-group"><a class="btn-save dc-test-btn" href="/desktop?app=easydrag">' + label('open') + '</a></div>';
    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}
