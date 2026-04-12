// cfg/memory_analysis.js — Memory Analysis config section module
// Refactored: clean information hierarchy, no redundant notices, clear controls

let _memAnalysisSection = null;

async function renderMemoryAnalysisSection(section) {
    if (section) _memAnalysisSection = section; else section = _memAnalysisSection;
    configData.memory_analysis = configData.memory_analysis || {};
    const cfg = configData.memory_analysis;

    // Sensible defaults for first-time users
    setNestedValue(configData, 'memory_analysis.enabled', cfg.enabled !== false);
    setNestedValue(configData, 'memory_analysis.real_time', cfg.real_time !== false);
    setNestedValue(configData, 'memory_analysis.query_expansion', cfg.query_expansion !== false);
    setNestedValue(configData, 'memory_analysis.llm_reranking', cfg.llm_reranking !== false);
    setNestedValue(configData, 'memory_analysis.unified_memory_block', cfg.unified_memory_block !== false);
    setNestedValue(configData, 'memory_analysis.effectiveness_tracking', cfg.effectiveness_tracking !== false);
    setNestedValue(configData, 'memory_analysis.weekly_reflection', cfg.weekly_reflection !== false);

    const html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>

        <div class="ma-intro-notice">
            <span class="ma-notice-icon">🧬</span>
            <div class="ma-notice-body">
                <strong>${t('config.memory_analysis.auto_notice')}</strong>
                <p>${t('config.memory_analysis.auto_desc')}</p>
            </div>
        </div>

        <div class="ma-block">
            <div class="ma-block-header">
                <span class="ma-block-icon">⚡</span>
                <div class="ma-block-text">
                    <div class="ma-block-title">${t('config.memory_analysis.provider_title')}</div>
                    <div class="ma-block-desc">${t('config.memory_analysis.provider_desc')}</div>
                </div>
            </div>
            <div class="ma-block-controls">
                <div class="ma-toggle-row">
                    <span class="ma-label-text">${t('config.memory_analysis.realtime_title')}</span>
                    <div class="toggle ${cfg.real_time ? 'on' : ''}" data-path="memory_analysis.real_time" onclick="toggleBool(this)"></div>
                </div>
                <div class="ma-toggle-row">
                    <span class="ma-label-text">${t('config.memory_analysis.query_expansion_label')}</span>
                    <div class="toggle ${cfg.query_expansion ? 'on' : ''}" data-path="memory_analysis.query_expansion" onclick="toggleBool(this)"></div>
                </div>
                <div class="ma-toggle-row">
                    <span class="ma-label-text">${t('config.memory_analysis.llm_reranking_label')}</span>
                    <div class="toggle ${cfg.llm_reranking ? 'on' : ''}" data-path="memory_analysis.llm_reranking" onclick="toggleBool(this)"></div>
                </div>
                <div class="ma-toggle-row">
                    <span class="ma-label-text">${t('config.memory_analysis.unified_block_label')}</span>
                    <div class="toggle ${cfg.unified_memory_block ? 'on' : ''}" data-path="memory_analysis.unified_memory_block" onclick="toggleBool(this)"></div>
                </div>
                <div class="ma-toggle-row">
                    <span class="ma-label-text">${t('config.memory_analysis.effectiveness_label')}</span>
                    <div class="toggle ${cfg.effectiveness_tracking ? 'on' : ''}" data-path="memory_analysis.effectiveness_tracking" onclick="toggleBool(this)"></div>
                </div>
            </div>
        </div>

        <div class="ma-block">
            <div class="ma-block-header">
                <span class="ma-block-icon">📊</span>
                <div class="ma-block-text">
                    <div class="ma-block-title">${t('config.memory_analysis.threshold_title')}</div>
                    <div class="ma-block-desc">${t('config.memory_analysis.threshold_desc')}</div>
                </div>
            </div>
            <div class="ma-block-controls">
                <div class="ma-threshold-wrap">
                    <label class="ma-label-block">
                        <span class="ma-label-text">${t('config.memory_analysis.threshold_label')}</span>
                        <div class="ma-threshold-row">
                            <input type="range" class="cfg-input ma-threshold-slider" data-path="memory_analysis.auto_confirm_threshold"
                                min="0" max="1" step="0.01" value="${cfg.auto_confirm_threshold != null ? cfg.auto_confirm_threshold : 0.92}"
                                oninput="this.nextElementSibling.textContent=this.value">
                            <span class="ma-threshold-value">${cfg.auto_confirm_threshold != null ? cfg.auto_confirm_threshold : 0.92}</span>
                        </div>
                    </label>
                </div>
            </div>
        </div>

        <div class="ma-block">
            <div class="ma-block-header">
                <span class="ma-block-icon">🗓️</span>
                <div class="ma-block-text">
                    <div class="ma-block-title">${t('config.memory_analysis.reflection_title')}</div>
                    <div class="ma-block-desc">${t('config.memory_analysis.reflection_desc')}</div>
                </div>
            </div>
            <div class="ma-block-controls">
                <div class="ma-toggle-row">
                    <span class="ma-label-text">${t('config.memory_analysis.reflection_label')}</span>
                    <div class="toggle ${cfg.weekly_reflection ? 'on' : ''}" data-path="memory_analysis.weekly_reflection" onclick="toggleBool(this)"></div>
                </div>
                <div class="ma-threshold-wrap">
                    <label class="ma-label-block">
                        <span class="ma-label-text">${t('config.memory_analysis.reflection_day_label')}</span>
                        <select class="cfg-input ma-input-spaced" data-path="memory_analysis.reflection_day"
                            onchange="setNestedValue(configData,'memory_analysis.reflection_day',this.value);setDirty(true)">
                            ${['monday','tuesday','wednesday','thursday','friday','saturday','sunday'].map(d =>
                                `<option value="${d}" ${(cfg.reflection_day || 'sunday') === d ? 'selected' : ''}>${t('config.memory_analysis.day_' + d)}</option>`
                            ).join('')}
                        </select>
                    </label>
                </div>
            </div>
        </div>

    </div>`;

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}
