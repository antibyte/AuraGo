(function () {
    'use strict';

    function readTemplateData() {
        var el = document.getElementById('aurago-template-data');
        if (!el) return {};
        try {
            return JSON.parse(el.textContent || '{}');
        } catch (err) {
            console.error('Failed to parse AuraGo template data', err);
            return {};
        }
    }

    var data = readTemplateData();
    window.SYSTEM_LANG = data.systemLang || document.documentElement.lang || 'en';
    window.BUILD_VERSION = data.buildVersion || window.BUILD_VERSION || 'dev';
    window.SHOW_TOOL_RESULTS = data.showToolResults === true;
    window.AGENT_DEBUG_MODE = data.debugMode === true;
    window.PERSONALITY_ENABLED = data.personalityEnabled === true;
    window.I18N = data.i18n || {};
    window.I18N_META = data.i18nMeta || {};
})();
