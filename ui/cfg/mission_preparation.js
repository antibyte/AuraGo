// cfg/mission_preparation.js — Mission Preparation configuration section module

function renderMissionPreparationSection(section) {
    const data = configData['mission_preparation'] || {};
    const form = window.AuraConfigForm;
    const prefix = 'mission_preparation.';
    document.getElementById('content').innerHTML = form.renderSpec({
        key: 'mission_preparation',
        label: section.label,
        desc: section.desc,
        fields: [
            form.toggle({ label: t('config.mission_preparation.enabled_label'), help: t('help.mission_preparation.enabled'), path: prefix + 'enabled', value: data.enabled === true }),
            form.field({ label: t('config.mission_preparation.provider_label'), help: t('help.mission_preparation.provider'), path: prefix + 'provider', value: data.provider || '', placeholder: t('config.mission_preparation.provider_placeholder') }),
            form.number({ label: t('config.mission_preparation.timeout_seconds_label'), help: t('help.mission_preparation.timeout_seconds'), path: prefix + 'timeout_seconds', value: data.timeout_seconds || 120, min: 30, max: 600 }),
            form.number({ label: t('config.mission_preparation.max_essential_tools_label'), help: t('help.mission_preparation.max_essential_tools'), path: prefix + 'max_essential_tools', value: data.max_essential_tools || 5, min: 1, max: 20 }),
            form.number({ label: t('config.mission_preparation.cache_expiry_hours_label'), help: t('help.mission_preparation.cache_expiry_hours'), path: prefix + 'cache_expiry_hours', value: data.cache_expiry_hours || 24, min: 1, max: 720 }),
            form.number({ label: t('config.mission_preparation.min_confidence_label'), help: t('help.mission_preparation.min_confidence'), path: prefix + 'min_confidence', value: data.min_confidence || 0.5, min: 0, max: 1, step: 0.1 }),
            form.toggle({ label: t('config.mission_preparation.auto_prepare_scheduled_label'), help: t('help.mission_preparation.auto_prepare_scheduled'), path: prefix + 'auto_prepare_scheduled', value: data.auto_prepare_scheduled !== false })
        ]
    });
    attachChangeListeners();
}
