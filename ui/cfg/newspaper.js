// Newspaper is an optional background capability; the Desktop profile owns the topics.
function renderNewspaperSection(section) {
    const data = configData.newspaper || (configData.newspaper = {enabled:false,readonly:false,max_minutes:30,max_pages:60,max_editions:365,allow_email:false,allow_telegram:false});
    const label = key => escapeHtml(t('config.newspaper.' + key));
    let html = '<div class="cfg-section active"><div class="section-header">' + escapeHtml(section.label) + '</div><div class="section-desc">' + escapeHtml(section.desc) + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info">' + label('setup_note') + '</div>';
    for (const key of ['enabled','readonly','allow_email','allow_telegram']) {
        html += '<div class="field-group"><div class="field-label">' + label(key) + '</div><div class="toggle ' + (data[key] ? 'on' : '') + '" data-path="newspaper.' + key + '" onclick="toggleBool(this)"></div></div>';
    }
    html += '<div class="field-grid two-cols">';
    for (const [key,min,max,fallback] of [['max_minutes',1,60,30],['max_pages',1,60,60],['max_editions',30,3650,365]]) {
        const value = Number.isFinite(Number(data[key])) && Number(data[key]) > 0 ? Number(data[key]) : fallback;
        html += '<div class="field-group"><div class="field-label">' + label(key) + '</div><input class="field-input" type="number" min="' + min + '" max="' + max + '" value="' + value + '" data-path="newspaper.' + key + '"></div>';
    }
    html += '</div><div class="field-group"><button class="btn-save dc-test-btn" type="button" onclick="newspaperCheckReadiness()">' + label('check') + '</button><a class="btn-save dc-test-btn" href="/desktop">' + label('open') + '</a><span id="newspaper-config-status" class="dc-test-result"></span></div></div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
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
    } catch (_) { target.textContent = t('config.newspaper.check_failed'); }
}
