// cfg/flows.js — EasyDrag flows: run limits, AI provider for AI steps and agent options.
// The switch and the four limits are read at boot, so the save answer reports a restart ("EasyDrag flows").
// The agent options are saved but not enforced yet (planned for phase 4), so their switches stay locked.
function renderFlowsSection(section) {
    const data = configData.flows || (configData.flows = { enabled: true, max_parallel_runs: 8, max_parallel_nodes_per_run: 4, run_retention_days: 30, max_runs_per_flow: 200, ai_provider: '', agent: { read_only: false, allow_publish: false } });
    data.agent = data.agent || { read_only: false, allow_publish: false };
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
    for (const [key, min, max, fallback] of [['max_parallel_runs', 1, 32, 8], ['max_parallel_nodes_per_run', 1, 16, 4], ['run_retention_days', 1, 365, 30], ['max_runs_per_flow', 10, 5000, 200]]) {
        const value = Number(data[key]) > 0 ? Number(data[key]) : fallback;
        html += '<div class="field-group"><div class="field-label">' + label(key) + '</div><input class="field-input" type="number" min="' + min + '" max="' + max + '" value="' + value + '" data-path="flows.' + key + '" aria-label="' + label(key) + '"></div>';
    }
    html += '</div>';
    // A provider reference (deleting the provider warns). A saved id missing from the list stays selected.
    const current = String(data.ai_provider || '');
    const providers = Array.isArray(providersCache) ? providersCache : [];
    let options = '<option value=""' + (current ? '' : ' selected') + '>' + label('ai_provider_main') + '</option>';
    if (current && !providers.some(p => String(p.id) === current)) {
        const missing = providersLoaded ? 'config.field.option_missing' : 'config.field.option_list_unavailable';
        options += '<option value="' + escapeAttr(current) + '" selected>' + escapeHtml(t(missing, { value: current })) + '</option>';
    }
    for (const p of providers) {
        const id = String(p.id);
        const text = (p.name || id) + (p.type ? ' [' + p.type + ']' : '') + (p.model ? ' — ' + p.model : '');
        options += '<option value="' + escapeAttr(id) + '"' + (id === current ? ' selected' : '') + '>' + escapeHtml(text) + '</option>';
    }
    html += '<div class="field-group"><div class="field-label">' + label('ai_provider') + '</div><div class="field-help">' + label('ai_provider_help') + '</div>'
        + '<select class="field-select" data-path="flows.ai_provider" aria-label="' + label('ai_provider') + '">' + options + '</select></div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info" id="flows-agent-note">' + label('agent_note') + '</div>';
    html += toggle('agent_read_only', 'flows.agent.read_only', !!data.agent.read_only, true);
    html += toggle('agent_allow_publish', 'flows.agent.allow_publish', !!data.agent.allow_publish, true);
    // The desktop opens EasyDrag from ?app=; without flows or missions it would open without the app.
    if (data.enabled !== false && !missionsOff) html += '<div class="field-group"><a class="btn-save dc-test-btn" href="/desktop?app=easydrag">' + label('open') + '</a></div>';
    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}
