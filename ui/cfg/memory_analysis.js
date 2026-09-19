// Memory analysis uses the Helper LLM. Render only settings consumed by runtime.
let _memAnalysisSection = null;

async function renderMemoryAnalysisSection(section) {
    if (section) _memAnalysisSection = section;
    else section = _memAnalysisSection;
    const cfg = configData.memory_analysis || {};
    const threshold = cfg.auto_confirm_threshold ?? 0.92;
    document.getElementById('content').innerHTML = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>
        <div class="ma-intro-notice"><div class="ma-notice-body">
            <strong>${t('config.memory_analysis.auto_notice')}</strong>
            <p>${t('config.memory_analysis.auto_desc')}</p>
        </div></div>
        <div class="ma-block"><div class="ma-block-header"><div class="ma-block-text">
            <div class="ma-block-title">${t('config.memory_analysis.threshold_title')}</div>
            <div class="ma-block-desc">${t('config.memory_analysis.threshold_desc')}</div>
        </div></div><div class="ma-block-controls">
            <label class="ma-label-block">
                <span class="ma-label-text">${t('config.memory_analysis.threshold_label')}</span>
                <div class="ma-threshold-row">
                    <input type="range" class="ma-threshold-slider" data-path="memory_analysis.auto_confirm_threshold"
                        min="0" max="1" step="0.01" value="${escapeAttr(threshold)}"
                        oninput="this.nextElementSibling.textContent=this.value">
                    <span class="ma-threshold-value">${escapeHtml(String(threshold))}</span>
                </div>
            </label>
        </div></div>
        <div class="ma-block"><div class="ma-block-header"><div class="ma-block-text">
            <div class="ma-block-title">${t('config.memory_analysis.reflection_title')}</div>
            <div class="ma-block-desc">${t('config.memory_analysis.reflection_desc')}</div>
        </div></div><div class="ma-block-controls">
            <label class="ma-label-block">
                <span class="ma-label-text">${t('config.memory_analysis.reflection_day_label')}</span>
                <select class="field-select" data-path="memory_analysis.reflection_day">
                    ${['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'].map(day =>
                        `<option value="${day}" ${(cfg.reflection_day || 'sunday') === day ? 'selected' : ''}>${t('config.memory_analysis.day_' + day)}</option>`
                    ).join('')}
                </select>
            </label>
        </div></div>
    </div>`;
    attachChangeListeners();
}
