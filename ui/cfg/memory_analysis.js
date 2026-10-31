// Memory analysis uses the Helper LLM. Render only settings consumed by runtime.
let _memAnalysisSection = null;

async function renderMemoryAnalysisSection(section) {
    if (section) _memAnalysisSection = section;
    else section = _memAnalysisSection;
    const cfg = configData.memory_analysis || {};
    const threshold = cfg.auto_confirm_threshold ?? 0.92;
    const form = window.AuraConfigForm;
    const notice = `<div class="ma-intro-notice"><div class="ma-notice-body">
            <strong>${t('config.memory_analysis.auto_notice')}</strong>
            <p>${t('config.memory_analysis.auto_desc')}</p>
        </div></div>`;
    const slider = `<div class="ma-block"><div class="ma-block-header"><div class="ma-block-text">
            <div class="ma-block-title">${t('config.memory_analysis.threshold_title')}</div>
            <div class="ma-block-desc field-help" id="ma-threshold-help">${t('config.memory_analysis.threshold_desc')}</div>
        </div></div><div class="ma-block-controls">
            <label class="ma-label-block">
                <span class="ma-label-text">${t('config.memory_analysis.threshold_label')}</span>
                <div class="ma-threshold-row">
                    <input type="range" class="ma-threshold-slider" aria-describedby="ma-threshold-help" data-path="memory_analysis.auto_confirm_threshold"
                        min="0" max="1" step="0.01" value="${escapeAttr(threshold)}"
                        oninput="this.nextElementSibling.textContent=this.value">
                    <span class="ma-threshold-value">${escapeHtml(String(threshold))}</span>
                </div>
            </label>
        </div></div>`;
    document.getElementById('content').innerHTML = form.renderSpec({
        key: 'memory_analysis',
        label: section.label,
        desc: section.desc,
        fields: [
            form.toggle({ label: t('config.memory_analysis.learnings_enable'), help: t('config.memory_analysis.learnings_help'), path: 'learnings.enabled', value: configData.learnings?.enabled === true }),
            form.toggle({ label: t('config.memory_analysis.graph_enable'), help: t('config.memory_analysis.graph_help'), path: 'code_graph.enabled', value: configData.code_graph?.enabled === true }),
            notice,
            slider,
            form.select({
                label: t('config.memory_analysis.reflection_day_label'),
                help: t('config.memory_analysis.reflection_desc'),
                path: 'memory_analysis.reflection_day',
                value: cfg.reflection_day || 'sunday',
                options: ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'].map(day => ({ value: day, label: t('config.memory_analysis.day_' + day) }))
            })
        ]
    });
    attachChangeListeners();
}
