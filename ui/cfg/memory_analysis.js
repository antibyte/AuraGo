// cfg/memory_analysis.js — Memory Analysis config section module

let _memAnalysisSection = null;

async function renderMemoryAnalysisSection(section) {
    if (section) _memAnalysisSection = section; else section = _memAnalysisSection;
    const cfg = configData.memory_analysis || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ── Enabled toggle ──
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.memory_analysis.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="memory_analysis.enabled" onclick="toggleBool(this);setNestedValue(configData,'memory_analysis.enabled',this.classList.contains('on'));renderMemoryAnalysisSection(null)"></div>
    </div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>🧬</span>
            <div>
                <strong>${t('config.memory_analysis.disabled_notice')}</strong><br>
                <small>${t('config.memory_analysis.disabled_desc')}</small>
            </div>
        </div>`;
        html += `</div>`;
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    // ── Provider ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.memory_analysis.provider_title')}</div>
        <div class="field-group-desc">${t('config.memory_analysis.provider_desc')}</div>`;

    const curProvider = cfg.provider || '';
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.memory_analysis.provider_label')}</span>
        <select class="cfg-input" data-path="memory_analysis.provider" style="width:100%;margin-top:0.2rem;"
            onchange="setNestedValue(configData,'memory_analysis.provider',this.value);setDirty(true)">
            <option value=""${!curProvider ? ' selected' : ''}>${t('config.memory_analysis.select_provider')}</option>`;
    providersCache.forEach(p => {
        const sel = (String(curProvider) === String(p.id)) ? ' selected' : '';
        const name = p.name || p.id;
        const badge = p.type ? (' [' + p.type + ']') : '';
        const model = p.model ? (' — ' + p.model) : '';
        html += `<option value="${escapeAttr(p.id)}"${sel}>${escapeAttr(name + badge + model)}</option>`;
    });
    html += `</select></label>`;

    // Model override
    const curModel = cfg.model || '';
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.memory_analysis.model_label')} <small style="color:var(--text-tertiary);">(${t('config.memory_analysis.model_hint')})</small></span>
        <input type="text" class="cfg-input" data-path="memory_analysis.model" value="${escapeAttr(curModel)}"
            placeholder="google/gemini-2.0-flash-001, gpt-4o-mini..."
            style="width:100%;margin-top:0.2rem;">
    </label>`;
    html += `</div>`;

    // ── Real-Time Analysis ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.memory_analysis.realtime_title')}</div>
        <div class="field-group-desc">${t('config.memory_analysis.realtime_desc')}</div>`;

    const realTimeOn = cfg.real_time === true;
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.memory_analysis.realtime_label')}</span>
        <div class="toggle ${realTimeOn ? 'on' : ''}" data-path="memory_analysis.real_time" onclick="toggleBool(this)"></div>
    </div>`;
    html += `</div>`;

    // ── Auto-Confirm Threshold ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.memory_analysis.threshold_title')}</div>
        <div class="field-group-desc">${t('config.memory_analysis.threshold_desc')}</div>`;

    const curThreshold = cfg.auto_confirm_threshold != null ? cfg.auto_confirm_threshold : 0.92;
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.memory_analysis.threshold_label')}</span>
        <div style="display:flex;align-items:center;gap:0.8rem;margin-top:0.3rem;">
            <input type="range" class="cfg-input" data-path="memory_analysis.auto_confirm_threshold"
                min="0" max="1" step="0.01" value="${curThreshold}"
                style="flex:1;"
                oninput="this.nextElementSibling.textContent=this.value">
            <span style="font-size:0.82rem;color:var(--text-secondary);min-width:2.5rem;text-align:right;">${curThreshold}</span>
        </div>
    </label>`;
    html += `</div>`;

    // ── Weekly Reflection ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.memory_analysis.reflection_title')}</div>
        <div class="field-group-desc">${t('config.memory_analysis.reflection_desc')}</div>`;

    const reflectionOn = cfg.weekly_reflection !== false;
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.memory_analysis.reflection_label')}</span>
        <div class="toggle ${reflectionOn ? 'on' : ''}" data-path="memory_analysis.weekly_reflection" onclick="toggleBool(this)"></div>
    </div>`;

    // Reflection day
    const days = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'];
    const curDay = cfg.reflection_day || 'sunday';
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.memory_analysis.reflection_day_label')}</span>
        <select class="cfg-input" data-path="memory_analysis.reflection_day" style="width:100%;margin-top:0.2rem;"
            onchange="setNestedValue(configData,'memory_analysis.reflection_day',this.value);setDirty(true)">`;
    days.forEach(d => {
        const sel = (curDay === d) ? ' selected' : '';
        html += `<option value="${d}"${sel}>${t('config.memory_analysis.day_' + d)}</option>`;
    });
    html += `</select></label>`;
    html += `</div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}
