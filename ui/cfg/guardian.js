let _guardianSection = null;

async function renderGuardianSection(section) {
    if (section) _guardianSection = section; else section = _guardianSection;
    const cfg = configData.guardian || {};
    const ps = cfg.promptsec || {};

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += renderInjectionGuardCard(cfg.injection_guard || {});

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
            <div class="field-help">${guardianHelp('config.guardian.max_scan_bytes_help', 'config.guardian.scan_desc')}</div>
            <input type="number" class="field-input" data-path="guardian.max_scan_bytes" value="${cfg.max_scan_bytes != null ? cfg.max_scan_bytes : 16384}" min="1024" max="1048576" step="1024">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.guardian.scan_edge_bytes_label')}</div>
            <div class="field-help">${guardianHelp('config.guardian.scan_edge_bytes_help', 'config.guardian.scan_desc')}</div>
            <input type="number" class="field-input" data-path="guardian.scan_edge_bytes" value="${cfg.scan_edge_bytes != null ? cfg.scan_edge_bytes : 6144}" min="0" max="524288" step="1024">
        </div>
    </div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.guardian.sanitizer_title')}</div>
        <div class="field-group-desc">${t('config.guardian.sanitizer_desc')}</div>`;

    const sanitizer = ps.sanitizer || {};
    html += renderGuardianToggle('guardian.promptsec.sanitizer.normalize', sanitizer.normalize !== false, t('config.guardian.sanitizer_normalize_label'), guardianHelp('config.guardian.sanitizer_normalize_help', 'config.guardian.sanitizer_desc'));
    html += renderGuardianToggle('guardian.promptsec.sanitizer.dehomoglyph', sanitizer.dehomoglyph !== false, t('config.guardian.sanitizer_dehomoglyph_label'), guardianHelp('config.guardian.sanitizer_dehomoglyph_help', 'config.guardian.sanitizer_desc'));
    html += renderGuardianToggle('guardian.promptsec.sanitizer.decode', sanitizer.decode !== false, t('config.guardian.sanitizer_decode_label'), guardianHelp('config.guardian.sanitizer_decode_help', 'config.guardian.sanitizer_desc'));
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.guardian.embedding_title')}</div>
        <div class="field-group-desc">${t('config.guardian.embedding_desc')}</div>`;

    const embeddingEnabled = ps.embedding && ps.embedding.enabled === true;
    html += renderGuardianToggle('guardian.promptsec.embedding.enabled', embeddingEnabled, t('config.guardian.embedding_enabled_label'), guardianHelp('config.guardian.embedding_enabled_help', 'config.guardian.embedding_desc'));

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
        <div class="field-help">${guardianHelp('config.guardian.policy_help', 'config.guardian.policy_desc')}</div>
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
    html += renderGuardianToggle('guardian.promptsec.taint.enabled', taintEnabled, t('config.guardian.taint_enabled_label'), guardianHelp('config.guardian.taint_enabled_help', 'config.guardian.taint_desc'));

    const taintLevel = (ps.taint && ps.taint.default_level) || 'untrusted';
    const levels = ['untrusted', 'suspicious', 'trusted'];
    html += `<div class="field-group">
        <div class="field-label">${t('config.guardian.taint_level_label')}</div>
        <div class="field-help">${guardianHelp('config.guardian.taint_level_help', 'config.guardian.taint_desc')}</div>
        <select class="field-select" data-path="guardian.promptsec.taint.default_level">`;
    levels.forEach(l => {
        const sel = (taintLevel === l) ? ' selected' : '';
        html += `<option value="${l}"${sel}>${t('config.guardian.taint_level_' + l)}</option>`;
    });
    html += `</select></div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.guardian.llm_judge_title')}</div>
        <div class="field-group-desc">${t('config.guardian.llm_judge_desc')}</div>`;

    const judgeEnabled = ps.llm_judge && ps.llm_judge.enabled === true;
    html += renderGuardianToggle('guardian.promptsec.llm_judge.enabled', judgeEnabled, t('config.guardian.llm_judge_enabled_label'), guardianHelp('config.guardian.llm_judge_enabled_help', 'config.guardian.llm_judge_desc'));

    const judgeMode = (ps.llm_judge && ps.llm_judge.mode) || 'uncertain';
    const judgeModes = ['uncertain', 'always', 'threat_detected', 'no_threat'];
    html += `<div class="field-group">
        <div class="field-label">${t('config.guardian.llm_judge_mode_label')}</div>
        <div class="field-help">${guardianHelp('config.guardian.llm_judge_mode_help', 'config.guardian.llm_judge_desc')}</div>
        <select class="field-select" data-path="guardian.promptsec.llm_judge.mode">`;
    judgeModes.forEach(m => {
        const sel = (judgeMode === m) ? ' selected' : '';
        html += `<option value="${m}"${sel}>${t('config.guardian.llm_judge_mode_' + m)}</option>`;
    });
    html += `</select></div>`;

    const judgeTimeout = (ps.llm_judge && ps.llm_judge.timeout_secs != null) ? ps.llm_judge.timeout_secs : 2;
    html += `<div class="field-group">
        <div class="field-label">${t('config.guardian.llm_judge_timeout_label')}</div>
        <div class="field-help">${guardianHelp('config.guardian.llm_judge_timeout_help', 'config.guardian.llm_judge_desc')}</div>
        <input type="number" class="field-input" data-path="guardian.promptsec.llm_judge.timeout_secs" value="${judgeTimeout}" min="0" max="60" step="1">
    </div>`;

    const judgePolicy = (ps.llm_judge && ps.llm_judge.policy) || '';
    html += `<div class="field-group">
        <div class="field-label">${t('config.guardian.llm_judge_policy_label')}</div>
        <div class="field-help">${t('config.guardian.llm_judge_policy_help')}</div>
        <textarea class="field-input field-textarea" data-path="guardian.promptsec.llm_judge.policy" rows="3">${escapeHtml(judgePolicy)}</textarea>
    </div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.guardian.output_title')}</div>
        <div class="field-group-desc">${t('config.guardian.output_desc')}</div>`;

    const useSanitized = ps.use_sanitized_output === true;
    html += renderGuardianToggle('guardian.promptsec.use_sanitized_output', useSanitized, t('config.guardian.use_sanitized_output_label'), guardianHelp('config.guardian.use_sanitized_output_help', 'config.guardian.output_desc'));
    html += `</div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    bindInjectionGuardCard(document.getElementById('injection-guard-card'));
}

function renderInjectionGuardCard(cfg) {
    const tr = key => escapeHtml(t('config.guardian.local_' + key));
    return `<div class="field-group" id="injection-guard-card">
        <div class="field-group-title">${tr('title')}</div>
        <div class="field-group-desc">${tr('desc')}</div>
        <p>Horizon-Labs/prompt-injection-guard-small · Apache-2.0</p>
        <p data-guard-size></p>
        <p data-guard-status role="status" aria-live="polite">${tr('loading')}</p>
        <progress data-guard-progress max="100" value="0" hidden aria-label="${tr('download')}"></progress>
        <div class="cfg-actions-row">
            <button type="button" class="btn" data-guard-action="download" disabled>${tr('download')}</button>
            <button type="button" class="btn" data-guard-action="cancel" hidden>${tr('cancel')}</button>
            <button type="button" class="btn" data-guard-action="test" disabled>${tr('test')}</button>
        </div>
        <div class="field-group">
            <label class="field-label"><input type="checkbox" data-path="guardian.injection_guard.enabled" ${cfg.enabled ? 'checked' : ''} disabled> ${tr('enabled')}</label>
            <div class="field-help">${tr('enable_help')}</div>
        </div>
        <div class="field-group">
            <label class="field-label" for="injection-guard-action">${tr('action')}</label>
            <select id="injection-guard-action" class="field-select" data-path="guardian.injection_guard.action">
                <option value="quarantine" ${cfg.action !== 'datamark' ? 'selected' : ''}>${tr('quarantine')}</option>
                <option value="datamark" ${cfg.action === 'datamark' ? 'selected' : ''}>${tr('datamark')}</option>
            </select>
            <div class="field-help">${tr('datamark_help')}</div>
        </div>
        <p data-guard-result role="status" aria-live="polite"></p>
    </div>`;
}

function bindInjectionGuardCard(root) {
    if (!root) return;
    const lifetime = new AbortController();
    const tr = key => t('config.guardian.local_' + key);
    const enabled = root.querySelector('input[type="checkbox"]');
    const result = root.querySelector('[data-guard-result]');
    const buttons = [...root.querySelectorAll('[data-guard-action]')];
    let timer = null, request = null, status = null, busy = false, generation = 0;
    const live = () => root.isConnected && !lifetime.signal.aborted;
    const paint = () => {
        if (!live() || !status) return;
        const installing = ['downloading', 'verifying'].includes(status.state);
        enabled.disabled = busy || (!enabled.checked && (!status.available || !status.ready));
        const state = status.error === 'cancelled' ? 'cancelled' : status.state === 'ready' && status.enabled ? 'active' : status.state;
        root.querySelector('[data-guard-status]').textContent = tr('state_' + state);
        root.querySelector('[data-guard-size]').textContent = tr('size') + ': ' + Math.ceil(status.size_bytes / 1000000).toLocaleString() + ' MB + ' + Math.ceil(status.runtime_size_bytes / 1000000).toLocaleString() + ' MB ONNX Runtime';
        const progress = root.querySelector('[data-guard-progress]');
        progress.hidden = !installing;
        progress.value = Math.max(0, Math.min(100, Number(status.progress?.percent) || 0));
        buttons.forEach(button => {
            const action = button.dataset.guardAction;
            button.hidden = action === 'cancel' && !installing;
            button.classList.toggle('is-hidden', button.hidden);
            button.disabled = busy || !status.available || (action === 'download' && installing) || (action === 'test' && !status.ready);
        });
    };
    const refresh = async () => {
        clearTimeout(timer);
        if (!live() || busy || document.hidden) return;
        request?.abort(); request = new AbortController();
        const mine = ++generation;
        try {
            const response = await fetch('/api/guardian/injection-guard/status', {signal: request.signal});
            if (response.status === 401 || response.status === 403) { dispose(); return; }
            if (!response.ok) throw new Error('status');
            const next = await response.json();
            if (!live() || mine !== generation) return;
            status = next; paint();
        } catch (error) {
            if (live() && mine === generation && error.name !== 'AbortError') root.querySelector('[data-guard-status]').textContent = tr('failed');
        } finally {
            if (live() && mine === generation) timer = setTimeout(refresh, 1500);
        }
    };
    const run = async action => {
        if (!live() || busy) return;
        busy = true; ++generation; request?.abort(); clearTimeout(timer); paint(); result.textContent = '';
        try {
            const response = await fetch('/api/guardian/injection-guard/' + action, {method: 'POST', signal: lifetime.signal});
            if (!response.ok) throw new Error('action');
            const next = await response.json();
            if (!live()) return;
            status = next;
            if (action === 'test') result.textContent = tr('test_ok');
        } catch (error) { if (live() && error.name !== 'AbortError') result.textContent = tr('failed'); }
        finally { busy = false; if (live()) { paint(); refresh(); } }
    };
    const dispose = () => { lifetime.abort(); request?.abort(); clearTimeout(timer); observer.disconnect(); };
    const observer = new MutationObserver(() => { if (!root.isConnected) dispose(); });
    observer.observe(document.body, {childList: true, subtree: true});
    buttons.forEach(button => button.addEventListener('click', () => run(button.dataset.guardAction), {signal: lifetime.signal}));
    enabled.addEventListener('change', () => {
        setNestedValue(configData, 'guardian.injection_guard.enabled', enabled.checked);
        setDirty(true); paint();
    }, {signal: lifetime.signal});
    window.addEventListener('cfg:section-leave', dispose, {signal: lifetime.signal, once: true});
    window.addEventListener('pagehide', dispose, {signal: lifetime.signal, once: true});
    document.addEventListener('visibilitychange', () => { if (document.hidden) {request?.abort();clearTimeout(timer);} else refresh(); }, {signal: lifetime.signal});
    document.addEventListener('aurago:config-saved', refresh, {signal: lifetime.signal});
    refresh();
}

function renderGuardianToggle(path, on, label, help) {
    return `<div class="field-group">
        <div class="field-label">${label}</div>
        <div class="field-help">${help}</div>
        <div class="toggle-wrap">
            <div class="toggle ${on ? 'on' : ''}" data-path="${path}" onclick="toggleBool(this);setNestedValue(configData,'${path}',this.classList.contains('on'));setDirty(true)"></div>
            <span class="toggle-label">${t(on ? 'config.toggle.active' : 'config.toggle.inactive')}</span>
        </div>
    </div>`;
}

function guardianHelp(key, fallbackKey) {
    return escapeHtml(tOr(key, t(fallbackKey)));
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
