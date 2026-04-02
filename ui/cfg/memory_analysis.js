// cfg/memory_analysis.js — Memory Analysis config section module

let _memAnalysisSection = null;

async function renderMemoryAnalysisSection(section) {
    if (section) _memAnalysisSection = section; else section = _memAnalysisSection;
    configData.memory_analysis = configData.memory_analysis || {};
    setNestedValue(configData, 'memory_analysis.enabled', true);
    setNestedValue(configData, 'memory_analysis.real_time', true);
    setNestedValue(configData, 'memory_analysis.query_expansion', true);
    setNestedValue(configData, 'memory_analysis.llm_reranking', true);
    setNestedValue(configData, 'memory_analysis.unified_memory_block', true);
    setNestedValue(configData, 'memory_analysis.effectiveness_tracking', true);
    setNestedValue(configData, 'memory_analysis.weekly_reflection', true);
    const cfg = configData.memory_analysis || {};

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="wh-notice">
        <span>🧬</span>
        <div>
            <strong>${t('config.memory_analysis.auto_notice')}</strong><br>
            <small>${t('config.memory_analysis.auto_desc')}</small>
        </div>
    </div>`;

    // ── Helper LLM ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.memory_analysis.provider_title')}</div>
        <div class="field-group-desc">${t('config.memory_analysis.provider_desc')}</div>`;
    html += `<div class="wh-notice">
        <span>⚡</span>
        <div>
            <strong>${t('config.memory_analysis.provider_title')}</strong><br>
            <small>${t('config.memory_analysis.provider_desc')}</small>
        </div>
    </div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.memory_analysis.realtime_title')}</div>
        <div class="field-group-desc">${t('config.memory_analysis.realtime_desc')}</div>`;
    html += `<div class="wh-notice">
        <span>⚙️</span>
        <div>
            <strong>${t('config.memory_analysis.enabled_label')}</strong><br>
            <small>${t('config.memory_analysis.auto_desc')}</small>
        </div>
    </div>`;
    html += `</div>`;

    // ── Auto-Confirm Threshold ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.memory_analysis.threshold_title')}</div>
        <div class="field-group-desc">${t('config.memory_analysis.threshold_desc')}</div>`;

    const curThreshold = cfg.auto_confirm_threshold != null ? cfg.auto_confirm_threshold : 0.92;
    html += `<label class="ma-label-block">
        <span class="ma-label-text">${t('config.memory_analysis.threshold_label')}</span>
        <div class="ma-threshold-row">
            <input type="range" class="cfg-input ma-threshold-slider" data-path="memory_analysis.auto_confirm_threshold"
                min="0" max="1" step="0.01" value="${curThreshold}"
                oninput="this.nextElementSibling.textContent=this.value">
            <span class="ma-threshold-value">${curThreshold}</span>
        </div>
    </label>`;
    html += `</div>`;

    // ── Weekly Reflection ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.memory_analysis.reflection_title')}</div>
        <div class="field-group-desc">${t('config.memory_analysis.reflection_desc')}</div>`;

    html += `<div class="ma-toggle-row">
        <span class="ma-label-text">${t('config.memory_analysis.reflection_label')}</span>
        <div class="toggle on disabled"></div>
    </div>`;

    // Reflection day
    const days = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'];
    const curDay = cfg.reflection_day || 'sunday';
    html += `<label class="ma-label-block">
        <span class="ma-label-text">${t('config.memory_analysis.reflection_day_label')}</span>
        <select class="cfg-input ma-input-spaced" data-path="memory_analysis.reflection_day"
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
