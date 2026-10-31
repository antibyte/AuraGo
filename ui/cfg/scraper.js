// cfg/scraper.js — Web Scraper section module

function renderWebScraperSection(section) {
    // Ensure nested config objects exist
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.web_scraper) configData.tools.web_scraper = {};
    const wsCfg = configData.tools.web_scraper;
    const wsEnabled = wsCfg.enabled === true;
    const summaryOn = wsCfg.summary_mode === true;
    const form = window.AuraConfigForm;

    // The summary mode only names the provider that is used; it does not add a setting of its own.
    const providerNote = '<div class="cfg-note-banner cfg-note-banner-info' + (summaryOn ? '' : ' is-disabled') + '"><strong>'
        + escapeHtml(t('config.web_scraper.summary_provider')) + '</strong><br><small>' + escapeHtml(t('config.web_scraper.summary_desc')) + '</small></div>';
    document.getElementById('content').innerHTML = form.renderSpec({
        label: section.label,
        desc: section.desc,
        groups: [
            { titleKey: 'config.refresh.operation', fields: [
                form.toggle({ label: t('config.web_scraper.enabled_label'), help: t('help.tools.web_scraper.enabled'), path: 'tools.web_scraper.enabled', value: wsEnabled })
            ] },
            { titleKey: 'config.refresh.settings', content:
                form.toggle({ label: t('config.web_scraper.summary_mode'), help: t('help.tools.web_scraper.summary_mode'), path: 'tools.web_scraper.summary_mode', value: summaryOn }) + providerNote }
        ],
        afterHTML: wsEnabled ? '' : form.emptyState({ title: t('config.web_scraper.disabled_notice'), desc: t('config.web_scraper.disabled_desc') })
    });
    attachChangeListeners();
}
