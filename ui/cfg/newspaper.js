// Newspaper is an optional background capability; the Desktop profile owns the topics.
let newspaperBudgetPreviewTimer = null;
let newspaperBudgetPreviewController = null;
let newspaperBudgetPreviewVersion = 0;

function renderNewspaperSection(section) {
    const data = configData.newspaper || (configData.newspaper = {enabled:false,readonly:false,budget_mode:'fixed',overview_sources:[],max_minutes:30,max_pages:60,max_searches:32,max_editions:365,allow_email:false,allow_telegram:false});
    const label = key => escapeHtml(t('config.newspaper.' + key));
    const budgetMode = data.budget_mode === 'auto' ? 'auto' : 'fixed';
    const sources = Array.isArray(data.overview_sources) ? data.overview_sources : [];
    let html = '<div class="cfg-section active"><div class="section-header">' + escapeHtml(section.label) + '</div><div class="section-desc">' + escapeHtml(section.desc) + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info">' + label('setup_note') + '</div>';
    for (const key of ['enabled','readonly','allow_email','allow_telegram']) {
        html += '<div class="field-group"><div class="field-label">' + label(key) + '</div><div class="toggle ' + (data[key] ? 'on' : '') + '" data-path="newspaper.' + key + '" onclick="toggleBool(this)"></div></div>';
    }
    html += '<div class="field-grid two-cols">';
    for (const [key,min,max,fallback] of [['max_minutes',1,60,30],['max_pages',1,60,60],['max_searches',1,64,32],['max_editions',30,3650,365]]) {
        const value = Number.isFinite(Number(data[key])) && Number(data[key]) > 0 ? Number(data[key]) : fallback;
        html += '<div class="field-group"><div class="field-label">' + label(key) + '</div><input class="field-input" type="number" min="' + min + '" max="' + max + '" value="' + value + '" data-path="newspaper.' + key + '"></div>';
    }
    html += '</div><p class="field-help">' + label('fixed_limits_only') + '</p><div class="field-group"><label class="field-label" for="newspaper-budget-mode">' + label('budget_mode') + '</label><select id="newspaper-budget-mode" class="field-select" data-path="newspaper.budget_mode"><option value="auto" ' + (budgetMode === 'auto' ? 'selected' : '') + '>' + label('budget_auto') + '</option><option value="fixed" ' + (budgetMode === 'fixed' ? 'selected' : '') + '>' + label('budget_fixed') + '</option></select><p class="field-help">' + label('budget_mode_help') + '</p></div>';
    html += '<div class="field-group"><div class="field-label">' + label('overview_sources') + '</div><p class="field-help">' + label('overview_sources_help') + '</p><div id="newspaper-overview-sources" role="group" aria-label="' + label('overview_sources') + '">';
    for (const [id,key] of [['google_news','source_google_news'],['hacker_news','source_hacker_news'],['techmeme','source_techmeme']]) {
        html += '<label class="np-source-choice"><input type="checkbox" data-overview-source="' + id + '" ' + (sources.includes(id) ? 'checked' : '') + '> ' + label(key) + '</label>';
    }
    html += '</div><div class="field-actions"><button class="btn-save" type="button" data-overview-preset="recommended">' + label('preset_recommended') + '</button><button class="btn-save" type="button" data-overview-preset="none">' + label('preset_none') + '</button></div><input class="field-input is-hidden" type="text" data-path="newspaper.overview_sources" data-type="array" value="' + escapeHtml(sources.join(', ')) + '" tabindex="-1" aria-hidden="true"></div>';
    html += '<section class="cfg-note-banner cfg-note-banner-info" aria-live="polite"><strong>' + label('budget_preview') + '</strong><div id="newspaper-budget-preview" role="status">' + label('budget_preview_loading') + '</div></section>';
    html += '<div class="field-group"><button class="btn-save dc-test-btn" type="button" onclick="newspaperCheckReadiness()">' + label('check') + '</button><a class="btn-save dc-test-btn" href="/desktop">' + label('open') + '</a><div id="newspaper-config-status" class="dc-test-result" role="status"></div></div></div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    const sourceInputs = [...document.querySelectorAll('[data-overview-source]')];
    const sourceValue = () => sourceInputs.filter(input => input.checked).map(input => input.dataset.overviewSource);
    const updateSources = () => {
        const hidden = document.querySelector('[data-path="newspaper.overview_sources"]');
        if (hidden) {
            hidden.value = sourceValue().join(', ');
            hidden.dispatchEvent(new Event('input', {bubbles:true}));
        }
        newspaperPreviewBudget();
    };
    sourceInputs.forEach(input => input.addEventListener('change', updateSources));
    document.querySelectorAll('[data-overview-preset]').forEach(button => button.addEventListener('click', () => {
        const chosen = button.dataset.overviewPreset === 'recommended' ? ['google_news','hacker_news','techmeme'] : [];
        sourceInputs.forEach(input => { input.checked = chosen.includes(input.dataset.overviewSource); });
        updateSources();
    }));
    document.querySelectorAll('#content [data-path="newspaper.budget_mode"], #content [data-path="newspaper.max_pages"], #content [data-path="newspaper.max_searches"], #content [data-path="newspaper.max_minutes"]').forEach(input => {
        input.addEventListener('input', newspaperPreviewBudget);
        input.addEventListener('change', newspaperPreviewBudget);
    });
    newspaperPreviewBudget();
}

function newspaperPreviewBudget() {
    clearTimeout(newspaperBudgetPreviewTimer);
    newspaperBudgetPreviewTimer = setTimeout(newspaperLoadBudgetPreview, 140);
}

async function newspaperLoadBudgetPreview() {
    const target = document.getElementById('newspaper-budget-preview');
    if (!target) return;
    const version = ++newspaperBudgetPreviewVersion;
    newspaperBudgetPreviewController?.abort();
    newspaperBudgetPreviewController = new AbortController();
    target.textContent = t('config.newspaper.budget_preview_loading');
    const value = path => document.querySelector('[data-path="newspaper.' + path + '"]')?.value;
    const number = (key, fallback) => {
        const parsed = Number(value(key));
        return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
    };
    const body = {
        budget_mode: value('budget_mode') === 'auto' ? 'auto' : 'fixed',
        max_pages: number('max_pages', 60),
        max_searches: number('max_searches', 32),
        max_minutes: number('max_minutes', 30),
        overview_sources: [...document.querySelectorAll('[data-overview-source]:checked')].map(input => input.dataset.overviewSource)
    };
    try {
        const response = await fetch('/api/desktop/newspaper/budget-preview', {
            method:'POST', credentials:'same-origin', cache:'no-store', signal:newspaperBudgetPreviewController.signal,
            headers:{'Content-Type':'application/json'}, body:JSON.stringify(body)
        });
        if (!response.ok) throw new Error('HTTP ' + response.status);
        const result = await response.json();
        if (version !== newspaperBudgetPreviewVersion || !target.isConnected) return;
        newspaperRenderBudgetPreview(target, result);
    } catch (error) {
        if (error.name === 'AbortError' || version !== newspaperBudgetPreviewVersion || !target.isConnected) return;
        target.textContent = t('config.newspaper.budget_preview_failed');
    }
}

function newspaperRenderBudgetPreview(target, result) {
    const budget = result?.effective_budget;
    target.replaceChildren();
    if (!budget || typeof budget !== 'object') {
        target.textContent = t('config.newspaper.budget_preview_failed');
        return;
    }
    const mode = budget.mode === 'auto' ? 'auto' : 'fixed';
    const summary = document.createElement('p');
    summary.textContent = t('config.newspaper.effective_budget_summary')
        .replace('{topics}', Number(budget.topics) || 0)
        .replace('{pages}', Number(budget.pages) || 0)
        .replace('{searches}', Number(budget.searches) || 0)
        .replace('{overviews}', Number(budget.overviews) || 0)
        .replace('{minutes}', Number(budget.minutes) || 0)
        .replace('{candidates}', Number(budget.candidates) || 0)
        .replace('{stories}', Number(budget.stories) || 0)
        .replace('{editor_calls}', Number(budget.editor_calls) || 0);
    target.append(summary);
    const modeLabel = document.createElement('p');
    modeLabel.textContent = t('config.newspaper.budget_' + mode);
    target.append(modeLabel);
    const monetary = document.createElement('p');
    monetary.textContent = result.monetary_budget_enabled === true ? t('config.newspaper.monetary_budget_enabled') : t('config.newspaper.monetary_budget_disabled');
    target.append(monetary);
    if (mode === 'fixed' && budget.shared_pages === true) {
        const fixed = document.createElement('p');
        fixed.textContent = t('config.newspaper.fixed_budget_note').replace('{pages}', Number(budget.pages) || 0);
        target.append(fixed);
    }
}

async function newspaperCheckReadiness() {
    const target = document.getElementById('newspaper-config-status');
    if (!target) return;
    target.textContent = t('config.newspaper.checking');
    try {
        const response = await fetch('/api/desktop/newspaper/capabilities', {credentials:'same-origin',cache:'no-store'});
        if (!response.ok) throw new Error('HTTP ' + response.status);
        const caps = await response.json();
        target.textContent = [caps.enabled ? t('config.newspaper.active') : t('config.newspaper.inactive'), caps.research_ready ? t('config.newspaper.research_ready') : t('config.newspaper.research_blocked'), caps.email_ready ? t('config.newspaper.email_ready') : t('config.newspaper.email_pending'), caps.telegram_ready ? t('config.newspaper.telegram_ready') : t('config.newspaper.telegram_pending')].join(' · ');
        const message = reason => t('config.newspaper.reason_' + (['ready','disabled','key_missing','network_disabled','feeds_missing','scraper_disabled','read_only','model_missing','skill_missing','rate_limited','quota_exhausted','access_denied'].includes(reason) ? reason : 'failed'));
        if (caps.research_reason) { const p = document.createElement('p'); p.textContent = message(caps.research_reason); target.append(p); }
        const list = document.createElement('ul');
        for (const tool of caps.research_tools || []) {
            if (!['brave_search','ddg_search','rss','web_scraper'].includes(tool.id)) continue;
            const row = document.createElement('li');
            row.textContent = t('config.newspaper.tool_' + tool.id) + ': ' + message(tool.reason || tool.state);
            if (tool.last_error) row.textContent += ' · ' + t('config.newspaper.last_attempt').replace('{result}', message(tool.last_error));
            list.append(row);
        }
        target.append(list);
    } catch (_) { target.textContent = t('config.newspaper.check_failed'); }
}
