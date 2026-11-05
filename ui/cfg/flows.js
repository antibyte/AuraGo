// cfg/flows.js — EasyDrag flows: run limits, AI provider for AI steps and agent options.
// The switch and the four limits are read at boot, so the save answer reports a restart ("EasyDrag flows").
// The limits' ranges are validation rules in js/config/catalog.js. The agent options are saved but not
// enforced yet (planned for phase 4), so their switches stay locked.
function renderFlowsSection(section) {
    const limits = [['max_parallel_runs', 8], ['max_parallel_nodes_per_run', 4], ['run_retention_days', 30], ['max_runs_per_flow', 200]];
    const data = configData.flows || (configData.flows = { enabled: true, ...Object.fromEntries(limits), ai_provider: '', agent: { read_only: false, allow_publish: false } });
    data.agent = data.agent || { read_only: false, allow_publish: false };
    const rules = (window.AuraConfigCatalog && window.AuraConfigCatalog.validationRules) || {};
    const form = window.AuraConfigForm;
    const text = (key, values) => t('config.flows.' + key, values);
    const label = key => escapeHtml(text(key));
    // The two agent options are not enforced yet: shown, but locked.
    const lockedToggle = (key, path, on) => '<div class="field-group"><div class="field-label">' + label(key) + '</div>'
        + '<div class="toggle' + (on ? ' on' : '') + ' cfg-toggle-disabled" data-path="' + path + '" aria-label="' + label(key) + '" aria-disabled="true" aria-describedby="flows-agent-note"></div></div>';
    const missionsOff = !!(configData.tools && configData.tools.missions && configData.tools.missions.enabled === false);
    let before = '<div class="cfg-note-banner cfg-note-banner-info">' + label('intro') + '</div>';
    if (missionsOff) before += '<div class="cfg-note-banner cfg-note-banner-warning">' + label('missions_off') + '</div>';
    const limitFields = limits.map(([key, fallback]) => {
        const rule = rules['flows.' + key] || {};
        const value = Number(data[key]) > 0 ? Number(data[key]) : fallback;
        return form.number({ label: text(key), help: text(key + '_help'), path: 'flows.' + key, value, min: rule.min, max: rule.max });
    });
    // A provider reference (lang/meta.json: provider_ref, empty_label_key), drawn and refreshed by the shared
    // choice helpers: an unknown saved id stays selected, a failed list shows the hint with Retry.
    const provider = '<div class="field-group"><div class="field-label">' + label('ai_provider') + '</div><div class="field-help">' + label('ai_provider_help') + '</div>'
        + '<select class="field-select" data-path="flows.ai_provider" data-config-choice="providers" aria-label="' + label('ai_provider') + '">'
        + cfgChoiceOptionsHTML('providers', helpTexts['flows.ai_provider'] || {}, data.ai_provider || '') + '</select>' + cfgChoiceHintHTML('providers') + '</div>';
    // This is observed server metadata, independent of the unsaved checkbox draft.
    const observed = configData._flows_durable_status;
    const validStatus = observed && ['configured', 'effective', 'recovery_available'].every(key => typeof observed[key] === 'boolean');
    const unknown = t('config.status.unknown');
    const bool = key => validStatus ? t(observed[key] ? 'config.toggle.active' : 'config.toggle.inactive') : unknown;
    const count = key => validStatus && Number.isSafeInteger(observed[key]) && observed[key] >= 0 ? observed[key] : unknown;
    const runtime = '<div class="field-group" data-flows-durable-status><div class="field-label">' + label('durable_observed') + '</div><div class="field-help">' +
        [['durable_configured', 'configured'], ['durable_effective', 'effective'], ['durable_available', 'recovery_available']].map(([key, value]) => label(key) + ': ' + escapeHtml(bool(value))).join(' · ') +
        '</div><div class="field-help">' + escapeHtml(text('durable_counts', { waiting: count('waiting_count'), running: count('claimed_count'), blocked: count('blocked_count') })) + '</div></div>';
    const groups = [
        { titleKey: 'config.refresh.operation', fields: [form.toggle({ label: text('enabled'), help: text('enabled_help'), path: 'flows.enabled', value: data.enabled !== false }),
            form.toggle({ label: text('durable_enabled'), help: text('durable_enabled_help'), path: 'flows.durable_enabled', value: data.durable_enabled === true }), runtime] },
        { titleKey: 'config.refresh.settings', fields: limitFields.concat([provider,
            '<div class="cfg-note-banner cfg-note-banner-info" id="flows-agent-note">' + label('agent_note') + '</div>',
            lockedToggle('agent_read_only', 'flows.agent.read_only', !!data.agent.read_only),
            lockedToggle('agent_allow_publish', 'flows.agent.allow_publish', !!data.agent.allow_publish)]) }
    ];
    // The desktop opens EasyDrag from ?app=; without flows or missions it would open without the app.
    const open = data.enabled !== false && !missionsOff ? '<div class="field-group"><a class="btn-save dc-test-btn" href="/desktop?app=easydrag">' + label('open') + '</a></div>' : '';
    document.getElementById('content').innerHTML = form.renderSpec({ label: section.label, desc: section.desc, beforeHTML: before, groups, afterHTML: open });
    attachChangeListeners();
}
