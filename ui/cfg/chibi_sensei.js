// cfg/chibi_sensei.js — Chibi Sensei configuration section (phases P1 and P2): the switch, the dataset limits
// and the model slot for generating and cleaning examples (its provider through the shared provider
// dropdown, lang/meta.json help.chibi_sensei.generator.provider). Defaults and ranges match
// internal/config/chibi_sensei.go; the server validates them. Agent and runtime keys stay
// hidden until their phases ship.

let _chibiSenseiSection = null;

const CHIBI_SENSEI_DEFAULTS = Object.freeze({
    enabled: true,
    storage: Object.freeze({ max_gb: 20 }),
    dataset: Object.freeze({ max_rows: 50000 }),
    import: Object.freeze({ max_rows: 20000, max_file_mb: 50 }),
    generator: Object.freeze({ provider: '', model: '', max_rows_per_job: 500, max_daily_rows: 5000 }),
    training: Object.freeze({ machine_shape: 'NvidiaTeslaT4', delete_remote_after_run: true })
});

const CHIBI_SENSEI_MACHINE_SHAPES = Object.freeze(['NvidiaTeslaT4', 'NvidiaL4']);

function chibiCfgFill(target, defaults) {
    Object.keys(defaults).forEach(key => {
        const def = defaults[key];
        if (def && typeof def === 'object') {
            if (!target[key] || typeof target[key] !== 'object') target[key] = {};
            chibiCfgFill(target[key], def);
        } else if (typeof def === 'boolean' && typeof target[key] !== 'boolean') target[key] = def;
        else if (typeof def === 'number' && (typeof target[key] !== 'number' || !Number.isFinite(target[key]))) target[key] = def;
        else if (typeof def === 'string' && typeof target[key] !== 'string') target[key] = def;
    });
}

function chibiCfgEnsureData() {
    if (!configData.chibi_sensei || typeof configData.chibi_sensei !== 'object') configData.chibi_sensei = {};
    chibiCfgFill(configData.chibi_sensei, CHIBI_SENSEI_DEFAULTS);
    return configData.chibi_sensei;
}

function chibiCfgToggleRow(labelKey, helpKey, on, path, handler) {
    return window.AuraConfigForm.toggle({ label: t(labelKey), help: helpKey ? tOr(helpKey, '') : '', path, value: on, onclick: handler });
}

function chibiCfgNumber(labelKey, helpKey, path, value, min, max, step) {
    return window.AuraConfigForm.number({ label: t(labelKey), help: helpKey ? tOr(helpKey, '') : '', path, value, min, max, step });
}

function chibiCfgText(labelKey, helpKey, path, value, placeholder) {
    return window.AuraConfigForm.field({ type: 'text', label: t(labelKey), help: helpKey ? tOr(helpKey, '') : '', path, value: value || '', placeholder: placeholder || '' });
}

function renderChibiSenseiSection(section) {
    if (section) _chibiSenseiSection = section;
    else section = _chibiSenseiSection;
    const data = chibiCfgEnsureData();
    const enabled = data.enabled === true;
    const form = window.AuraConfigForm;

    const groups = [{ titleKey: 'config.refresh.operation', fields: [
        chibiCfgToggleRow('config.chibi_sensei.enabled_label', 'help.chibi_sensei.enabled', enabled, 'chibi_sensei.enabled', 'chibiCfgToggleEnabled(this)')
    ] }];
    let after = '';

    if (!enabled) {
        after = form.emptyState({ title: t('config.chibi_sensei.disabled_notice'), desc: t('config.chibi_sensei.disabled_desc') });
    } else {
        groups.push({ title: t('config.chibi_sensei.data_title'), fields: [
            chibiCfgNumber('config.chibi_sensei.storage_max_gb_label', 'config.chibi_sensei.storage_max_gb_help', 'chibi_sensei.storage.max_gb', data.storage.max_gb, 1, 500),
            chibiCfgNumber('config.chibi_sensei.dataset_max_rows_label', 'config.chibi_sensei.dataset_max_rows_help', 'chibi_sensei.dataset.max_rows', data.dataset.max_rows, 1000, 50000),
            chibiCfgNumber('config.chibi_sensei.import_max_rows_label', 'config.chibi_sensei.import_max_rows_help', 'chibi_sensei.import.max_rows', data.import.max_rows, 100, 50000),
            chibiCfgNumber('config.chibi_sensei.import_max_file_mb_label', 'config.chibi_sensei.import_max_file_mb_help', 'chibi_sensei.import.max_file_mb', data.import.max_file_mb, 1, 500)
        ] });
        groups.push({ title: t('config.chibi_sensei.generator_title'), fields: [
            '<div class="field-group"><div class="field-label">' + escapeHtml(t('config.chibi_sensei.generator_provider_label')) + '</div>'
            + '<div class="field-help">' + escapeHtml(t('config.chibi_sensei.generator_provider_help')) + '</div>'
            + '<select class="field-select" data-path="chibi_sensei.generator.provider" data-config-choice="providers" aria-label="' + escapeAttr(t('config.chibi_sensei.generator_provider_label')) + '">'
            + cfgChoiceOptionsHTML('providers', helpTexts['chibi_sensei.generator.provider'] || {}, data.generator.provider || '') + '</select>' + cfgChoiceHintHTML('providers') + '</div>',
            chibiCfgText('config.chibi_sensei.generator_model_label', 'config.chibi_sensei.generator_model_help', 'chibi_sensei.generator.model', data.generator.model, ''),
            chibiCfgNumber('config.chibi_sensei.generator_max_rows_per_job_label', 'config.chibi_sensei.generator_max_rows_per_job_help', 'chibi_sensei.generator.max_rows_per_job', data.generator.max_rows_per_job, 10, 5000),
            chibiCfgNumber('config.chibi_sensei.generator_max_daily_rows_label', 'config.chibi_sensei.generator_max_daily_rows_help', 'chibi_sensei.generator.max_daily_rows', data.generator.max_daily_rows, 100, 100000)
        ] });
        groups.push({ title: t('config.chibi_sensei.training_title'), fields: [
            form.select({ label: t('config.chibi_sensei.training_machine_shape_label'), help: tOr('config.chibi_sensei.training_machine_shape_help', ''),
                path: 'chibi_sensei.training.machine_shape', value: data.training.machine_shape,
                options: CHIBI_SENSEI_MACHINE_SHAPES.map(value => ({ value, label: t('config.chibi_sensei.machine_shape_' + value) })) }),
            chibiCfgToggleRow('config.chibi_sensei.training_delete_remote_label', 'config.chibi_sensei.training_delete_remote_help',
                data.training.delete_remote_after_run === true, 'chibi_sensei.training.delete_remote_after_run', 'toggleBool(this)')
        ] });
        after = '<div class="field-group"><div class="field-help">' + escapeHtml(t('config.chibi_sensei.later_note')) + '</div></div>'
            + '<div class="field-group"><a class="btn-save dc-test-btn" href="/desktop?app=chibi-sensei">' + escapeHtml(t('config.chibi_sensei.open_app')) + '</a></div>';
    }

    document.getElementById('content').innerHTML = form.renderSpec({ label: section.label, desc: section.desc, beforeHTML: '', groups, afterHTML: after });
    attachChangeListeners();
}

function chibiCfgToggleEnabled(el) {
    const next = !(el && el.classList && el.classList.contains('on'));
    // The re-render below detaches this switch, but the click still bubbles on to the
    // AuraConfigState document listener, which reads the clicked element. Give the old switch the
    // new value first, so that listener cannot write the previous value back into the draft.
    if (el && el.classList) {
        el.classList.toggle('on', next);
        el.setAttribute('aria-checked', next ? 'true' : 'false');
    }
    const data = chibiCfgEnsureData();
    data.enabled = next;
    setNestedValue(configData, 'chibi_sensei.enabled', next);
    if (window.AuraConfigState) window.AuraConfigState.set('chibi_sensei.enabled', next);
    renderChibiSenseiSection(null);
    setDirty(window.AuraConfigState ? window.AuraConfigState.isDirty() : true);
}
