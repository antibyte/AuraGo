// cfg/flows.js — EasyDrag flows: run limits, AI provider for AI steps and agent options.
function renderFlowsSection(section) {
    const data = configData.flows || (configData.flows = { enabled: true, max_parallel_runs: 8, max_parallel_nodes_per_run: 4, run_retention_days: 30, max_runs_per_flow: 200, ai_provider: '', agent: { read_only: false, allow_publish: false } });
    data.agent = data.agent || { read_only: false, allow_publish: false };
    const label = key => escapeHtml(t('config.flows.' + key));
    const toggle = (key, path, on) => '<div class="field-group"><div class="field-label">' + label(key) + '</div><div class="toggle ' + (on ? 'on' : '') + '" data-path="' + path + '" onclick="toggleBool(this)"></div></div>';
    const missionsOff = !!(configData.tools && configData.tools.missions && configData.tools.missions.enabled === false);
    let html = '<div class="cfg-section active"><div class="section-header">' + escapeHtml(section.label) + '</div><div class="section-desc">' + escapeHtml(section.desc) + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info">' + label('intro') + '</div>';
    if (missionsOff) html += '<div class="cfg-note-banner cfg-note-banner-warning">' + label('missions_off') + '</div>';
    html += toggle('enabled', 'flows.enabled', data.enabled !== false);
    html += '<div class="field-grid two-cols">';
    for (const [key, min, max, fallback] of [['max_parallel_runs', 1, 32, 8], ['max_parallel_nodes_per_run', 1, 16, 4], ['run_retention_days', 1, 365, 30], ['max_runs_per_flow', 10, 5000, 200]]) {
        const value = Number(data[key]) > 0 ? Number(data[key]) : fallback;
        html += '<div class="field-group"><div class="field-label">' + label(key) + '</div><input class="field-input" type="number" min="' + min + '" max="' + max + '" value="' + value + '" data-path="flows.' + key + '"></div>';
    }
    html += '</div>';
    html += '<div class="field-group"><div class="field-label">' + label('ai_provider') + '</div><input class="field-input" type="text" data-path="flows.ai_provider" value="' + escapeAttr(data.ai_provider || '') + '" placeholder="' + escapeAttr(t('config.flows.ai_provider_placeholder')) + '"></div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info">' + label('agent_note') + '</div>';
    html += toggle('agent_read_only', 'flows.agent.read_only', !!data.agent.read_only);
    html += toggle('agent_allow_publish', 'flows.agent.allow_publish', !!data.agent.allow_publish);
    html += '<div class="field-group"><a class="btn-save dc-test-btn" href="/desktop">' + label('open') + '</a></div></div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}
