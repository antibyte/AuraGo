// cfg/scraper.js — Web Scraper section module

        function renderWebScraperSection(section) {
            // Ensure nested config objects exist
            if (!configData.tools) configData.tools = {};
            if (!configData.tools.web_scraper) configData.tools.web_scraper = {};
            const wsCfg = configData.tools.web_scraper;
            const wsEnabled = wsCfg.enabled === true;
            const summaryOn = wsCfg.summary_mode === true;

            let html = `<div class="cfg-section active">
                <div class="section-header">${section.icon} ${section.label}</div>
                <div class="section-desc">${section.desc}</div>`;

            // Enabled toggle
            html += `<div class="ws-toggle-row ws-toggle-row-main">
                <span class="ws-toggle-label-main">${t('config.web_scraper.enabled_label')}</span>
                <div class="toggle ${wsEnabled ? 'on' : ''}" data-path="tools.web_scraper.enabled" onclick="toggleBool(this)"></div>
            </div>`;

            if (!wsEnabled) {
                html += `<div class="wh-notice">
                    <span>🕷️</span>
                    <div>
                        <strong>${t('config.web_scraper.disabled_notice')}</strong><br>
                        <small>${t('config.web_scraper.disabled_desc')}</small>
                    </div>
                </div>`;
            }

            // Summary mode section
            html += `<div class="ws-summary-box">
                <div class="ws-toggle-row">
                    <span class="ws-summary-title">📝 ${t('config.web_scraper.summary_mode')}</span>
                    <div class="toggle ${summaryOn ? 'on' : ''}" data-path="tools.web_scraper.summary_mode" onclick="toggleBool(this)"></div>
                </div>
                <div class="ws-summary-desc">
                    ${t('config.web_scraper.summary_desc')}
                </div>`;

            // Provider dropdown (only relevant when summary mode is on)
            const curProvider = wsCfg.summary_provider || '';
            html += `<label class="ws-provider-label ${summaryOn ? '' : 'is-disabled'}">
                <span class="ws-provider-text">${t('config.web_scraper.summary_provider')} <small class="ws-provider-hint">(${t('config.web_scraper.empty_main_llm')})</small></span>
                <select class="cfg-input ws-provider-select" data-path="tools.web_scraper.summary_provider"
                    onchange="setNestedValue(configData,'tools.web_scraper.summary_provider',this.value);setDirty(true)">
                    <option value=""${!curProvider ? ' selected' : ''}>${t('config.web_scraper.use_main_llm')}</option>`;
            providersCache.forEach(p => {
                const sel = (String(curProvider) === String(p.id)) ? ' selected' : '';
                const name = p.name || p.id;
                const badge = p.type ? (' [' + p.type + ']') : '';
                const model = p.model ? (' — ' + p.model) : '';
                html += `<option value="${escapeAttr(p.id)}"${sel}>${escapeAttr(name + badge + model)}</option>`;
            });
            html += `</select></label>`;

            html += `</div>`; // close summary mode box

            html += `</div>`; // close cfg-section
            document.getElementById('content').innerHTML = html;
            attachChangeListeners();
        }
