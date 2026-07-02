let _guardianSection = null;

async function renderGuardianSection(section) {
    if (section) _guardianSection = section; else section = _guardianSection;
    const cfg = configData.guardian || {};
    const ps = cfg.promptsec || {};

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.guardian.scan_title')}</div>
        <div class="field-group-desc">${t('config.guardian.scan_desc')}</div>`;

    const curPreset = ps.preset || 'strict';
    const presets = ['strict', 'moderate', 'lenient'];
    html += `<div class="field-group">
        <div class="field-label">${t('config.guardian.preset_label')}</div>
        <div class="field-help">${t('config.guardian.preset_help')}</div>
        <select class="field-select" data-path="guardian.promptsec.preset">`;
    presets.forEach(p => {
        const sel = (curPreset === p) ? ' selected' : '';
        html += `<option value="${p}"${sel}>${t('config.guardian.preset_' + p)}</option>`;
    });
    html += `</select></div>`;

    html += `<div class="field-grid two-cols">
        <div class="field-group">
            <div class="field-label">${t('config.guardian.max_scan_bytes_label')}</div>
            <input type="number" class="field-input" data-path="guardian.max_scan_bytes" value="${cfg.max_scan_bytes != null ? cfg.max_scan_bytes : 16384}" min="1024" max="1048576" step="1024">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.guardian.scan_edge_bytes_label')}</div>
            <input type="number" class="field-input" data-path="guardian.scan_edge_bytes" value="${cfg.scan_edge_bytes != null ? cfg.scan_edge_bytes : 6144}" min="0" max="524288" step="1024">
        </div>
    </div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.guardian.sanitizer_title')}</div>
        <div class="field-group-desc">${t('config.guardian.sanitizer_desc')}</div>`;

    const sanitizer = ps.sanitizer || {};
    html += renderGuardianToggle('guardian.promptsec.sanitizer.normalize', sanitizer.normalize !== false, t('config.guardian.sanitizer_normalize_label'));
    html += renderGuardianToggle('guardian.promptsec.sanitizer.dehomoglyph', sanitizer.dehomoglyph !== false, t('config.guardian.sanitizer_dehomoglyph_label'));
    html += renderGuardianToggle('guardian.promptsec.sanitizer.decode', sanitizer.decode !== false, t('config.guardian.sanitizer_decode_label'));
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.guardian.embedding_title')}</div>
        <div class="field-group-desc">${t('config.guardian.embedding_desc')}</div>`;

    const embeddingEnabled = ps.embedding && ps.embedding.enabled === true;
    html += renderGuardianToggle('guardian.promptsec.embedding.enabled', embeddingEnabled, t('config.guardian.embedding_enabled_label'));

    const threshold = (ps.embedding && ps.embedding.threshold != null) ? ps.embedding.threshold : 0.65;
    html += `<div class="field-group">
        <div class="field-label">${t('config.guardian.embedding_threshold_label')}</div>
        <div class="field-help">${t('config.guardian.embedding_threshold_help')}</div>
        <input type="number" class="field-input" data-path="guardian.promptsec.embedding.threshold" value="${threshold}" min="0" max="1" step="0.05">
    </div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.guardian.policy_title')}</div>
        <div class="field-group-desc">${t('config.guardian.policy_desc')}</div>`;

    const curPolicy = ps.policy || '';
    const policies = ['', 'rag', 'support', 'coding', 'translation', 'custom'];
    html += `<div class="field-group">
        <div class="field-label">${t('config.guardian.policy_label')}</div>
        <select class="field-select" data-path="guardian.promptsec.policy" onchange="guardianSetPolicy(this.value)">`;
    policies.forEach(p => {
        const sel = (curPolicy === p) ? ' selected' : '';
        html += `<option value="${p}"${sel}>${t('config.guardian.policy_' + (p || 'none'))}</option>`;
    });
    html += `</select></div>`;

    if (curPolicy === 'custom') {
        const disallowed = (ps.custom_policy && ps.custom_policy.disallowed_tasks) || [];
        const tasks = [
            { key: 'code_generation', label: t('config.guardian.task_code_generation') },
            { key: 'sql_access', label: t('config.guardian.task_sql_access') },
            { key: 'terminal_simulation', label: t('config.guardian.task_terminal_simulation') },
            { key: 'roleplay', label: t('config.guardian.task_roleplay') },
            { key: 'external_persona', label: t('config.guardian.task_external_persona') },
            { key: 'translation', label: t('config.guardian.task_translation') },
            { key: 'creative_writing', label: t('config.guardian.task_creative_writing') },
            { key: 'opinion_persuasion', label: t('config.guardian.task_opinion_persuasion') }
        ];
        html += `<div class="field-group">
            <div class="field-label">${t('config.guardian.custom_policy_label')}</div>
            <div class="field-help">${t('config.guardian.custom_policy_help')}</div>
            <div class="cfg-checkbox-list">`;
        tasks.forEach(task => {
            const checked = disallowed.includes(task.key) ? ' checked' : '';
            html += `<label class="cfg-checkbox-row">
                <input type="checkbox"${checked} onchange="guardianToggleCustomTask('${task.key}', this.checked)">
                <span>${escapeHtml(task.label)}</span>
            </label>`;
        });
        html += `</div></div>`;
    }
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.guardian.taint_title')}</div>
        <div class="field-group-desc">${t('config.guardian.taint_desc')}</div>`;

    const taintEnabled = ps.taint && ps.taint.enabled === true;
    html += renderGuardianToggle('guardian.promptsec.taint.enabled', taintEnabled, t('config.guardian.taint_enabled_label'));

    const taintLevel = (ps.taint && ps.taint.default_level) || 'untrusted';
    const levels = ['untrusted', 'suspicious', 'trusted'];
    html += `<div class="field-group">
        <div class="field-label">${t('config.guardian.taint_level_label')}</div>
        <select class="field-select" data-path="guardian.promptsec.taint.default_level">`;
    levels.forEach(l => {
        const sel = (taintLevel === l) ? ' selected' : '';
        html += `<option value="${l}"${sel}>${t('config.guardian.taint_level_' + l)}</option>`;
    });
    html += `</select></div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.guardian.structure_title')}</div>
        <div class="field-group-desc">${t('config.guardian.structure_desc')}</div>`;

    const structureEnabled = ps.structure && ps.structure.enabled === true;
    html += renderGuardianToggle('guardian.promptsec.structure.enabled', structureEnabled, t('config.guardian.structure_enabled_label'));

    const structureMode = (ps.structure && ps.structure.mode) || 'sandwich';
    const modes = ['sandwich', 'xml', 'random'];
    html += `<div class="field-group">
        <div class="field-label">${t('config.guardian.structure_mode_label')}</div>
        <select class="field-select" data-path="guardian.promptsec.structure.mode">`;
    modes.forEach(m => {
        const sel = (structureMode === m) ? ' selected' : '';
        html += `<option value="${m}"${sel}>${t('config.guardian.structure_mode_' + m)}</option>`;
    });
    html += `</select></div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.guardian.llm_judge_title')}</div>
        <div class="field-group-desc">${t('config.guardian.llm_judge_desc')}</div>`;

    const judgeEnabled = ps.llm_judge && ps.llm_judge.enabled === true;
    html += renderGuardianToggle('guardian.promptsec.llm_judge.enabled', judgeEnabled, t('config.guardian.llm_judge_enabled_label'));

    const judgeMode = (ps.llm_judge && ps.llm_judge.mode) || 'uncertain';
    const judgeModes = ['uncertain', 'always', 'threat_detected', 'no_threat'];
    html += `<div class="field-group">
        <div class="field-label">${t('config.guardian.llm_judge_mode_label')}</div>
        <select class="field-select" data-path="guardian.promptsec.llm_judge.mode">`;
    judgeModes.forEach(m => {
        const sel = (judgeMode === m) ? ' selected' : '';
        html += `<option value="${m}"${sel}>${t('config.guardian.llm_judge_mode_' + m)}</option>`;
    });
    html += `</select></div>`;

    const judgeTimeout = (ps.llm_judge && ps.llm_judge.timeout_secs != null) ? ps.llm_judge.timeout_secs : 2;
    html += `<div class="field-group">
        <div class="field-label">${t('config.guardian.llm_judge_timeout_label')}</div>
        <input type="number" class="field-input" data-path="guardian.promptsec.llm_judge.timeout_secs" value="${judgeTimeout}" min="0" max="60" step="1">
    </div>`;

    const judgePolicy = (ps.llm_judge && ps.llm_judge.policy) || '';
    html += `<div class="field-group">
        <div class="field-label">${t('config.guardian.llm_judge_policy_label')}</div>
        <div class="field-help">${t('config.guardian.llm_judge_policy_help')}</div>
        <textarea class="field-textarea" data-path="guardian.promptsec.llm_judge.policy" rows="3">${escapeHtml(judgePolicy)}</textarea>
    </div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.guardian.output_title')}</div>
        <div class="field-group-desc">${t('config.guardian.output_desc')}</div>`;

    const useSanitized = ps.use_sanitized_output === true;
    html += renderGuardianToggle('guardian.promptsec.use_sanitized_output', useSanitized, t('config.guardian.use_sanitized_output_label'));
    html += `</div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function renderGuardianToggle(path, on, label) {
    return `<div class="cfg-toggle-row-compact">
        <div class="toggle ${on ? 'on' : ''}" data-path="${path}" onclick="toggleBool(this);setNestedValue(configData,'${path}',this.classList.contains('on'));setDirty(true)"></div>
        <span class="cfg-toggle-label">${label}</span>
    </div>`;
}

function guardianSetPolicy(value) {
    if (!configData.guardian) configData.guardian = {};
    if (!configData.guardian.promptsec) configData.guardian.promptsec = {};
    setNestedValue(configData, 'guardian.promptsec.policy', value);
    setDirty(true);
    renderGuardianSection(null);
}

function guardianToggleCustomTask(task, checked) {
    if (!configData.guardian) configData.guardian = {};
    if (!configData.guardian.promptsec) configData.guardian.promptsec = {};
    if (!configData.guardian.promptsec.custom_policy) configData.guardian.promptsec.custom_policy = {};
    let tasks = configData.guardian.promptsec.custom_policy.disallowed_tasks || [];
    if (checked) {
        if (!tasks.includes(task)) tasks = tasks.concat(task);
    } else {
        tasks = tasks.filter(t => t !== task);
    }
    configData.guardian.promptsec.custom_policy.disallowed_tasks = tasks;
    setDirty(true);
}

function escapeHtml(text) {
    if (text == null) return '';
    return String(text)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}
