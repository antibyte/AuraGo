// cfg/scraper.js — Web Scraper section module

        function renderWebScraperSection(section) {
            // Ensure nested config objects exist
            if (!configData.tools) configData.tools = {};
            if (!configData.tools.web_scraper) configData.tools.web_scraper = {};
            const wsCfg = configData.tools.web_scraper;
            const wsEnabled = wsCfg.enabled === true;
            const summaryOn = wsCfg.summary_mode === true;

            let html = `<div class="cfg-section active">
                <div class="section-header">${section.label}</div>
                <div class="section-desc">${section.desc}</div>`;

            // Enabled toggle
            html += `<div class="cfg-toggle-row-highlight">
                <span class="cfg-toggle-label">${t('config.web_scraper.enabled_label')}</span>
                <div class="toggle ${wsEnabled ? 'on' : ''}" data-path="tools.web_scraper.enabled" aria-describedby="ws-enabled-help" onclick="toggleBool(this)"></div>
            </div>
            <div class="field-help" id="ws-enabled-help">${t('help.tools.web_scraper.enabled')}</div>`;

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
                <div class="cfg-toggle-row">
                    <span class="cfg-toggle-title" id="ws-summary-title">📝 ${t('config.web_scraper.summary_mode')}</span>
                    <div class="toggle ${summaryOn ? 'on' : ''}" data-path="tools.web_scraper.summary_mode" aria-labelledby="ws-summary-title" aria-describedby="ws-summary-help" onclick="toggleBool(this)"></div>
                </div>
                <div class="ws-summary-desc field-help" id="ws-summary-help">
                    ${t('help.tools.web_scraper.summary_mode')}
                </div>`;

            html += `<div class="wh-notice ${summaryOn ? '' : 'is-disabled'}">
                <span>⚡</span>
                <div>
                    <strong>${t('config.web_scraper.summary_provider')}</strong><br>
                    <small>${t('config.web_scraper.summary_desc')}</small>
                </div>
            </div>`;

            html += `</div>`; // close summary mode box

            html += `</div>`; // close cfg-section
            document.getElementById('content').innerHTML = html;
            attachChangeListeners();
        }
